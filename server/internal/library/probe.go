package library

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/trackpick"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// probeTarget is a media file with its root, ready for ffprobe.
type probeTarget struct {
	file  systemq.MediaFile
	root  paths.Root
	abs   string // absolute path for the ffprobe argument, resolved through paths
	found bool   // false when the file or its root is gone; the job then ends quietly
}

func (s *Service) loadProbeTarget(ctx context.Context, j *jobs.Job) (probeTarget, error) {
	var p mediaFilePayload
	if err := decodePayload(j, &p); err != nil {
		return probeTarget{}, err
	}
	mf, err := systemq.New(s.db.Read).GetMediaFileSystem(ctx, p.MediaFileID)
	if errors.Is(err, sql.ErrNoRows) {
		return probeTarget{}, nil
	}
	if err != nil {
		return probeTarget{}, fmt.Errorf("load media file: %w", err)
	}
	if mf.MissingSince.Valid {
		return probeTarget{}, nil
	}
	root, err := appq.New(s.db.Read).GetLibraryRoot(ctx, mf.RootID)
	if errors.Is(err, sql.ErrNoRows) {
		return probeTarget{}, nil
	}
	if err != nil {
		return probeTarget{}, fmt.Errorf("load library root: %w", err)
	}
	pr, err := paths.NewRoot(root.Path)
	if err != nil {
		return probeTarget{}, err
	}
	abs, err := paths.Resolve(pr, mf.RelPath)
	if err != nil {
		return probeTarget{}, err
	}
	return probeTarget{file: mf, root: pr, abs: abs, found: true}, nil
}

// errFileChanged: the media file row changed (size or mtime) while probe.file or probe.keyframes
// read the file; the job runs again after fileChangedRetry with the new version, without using an
// attempt (concept 14, Z-9).
var errFileChanged = errors.New("media file changed during the read")

// fileChangedRetry is the wait of a job whose file changed while it read it.
const fileChangedRetry = 10 * time.Second

// fileChanged returns the error of a job whose file changed while it read it: errFileChanged,
// retried by the queue without using an attempt.
func fileChanged(mediaFileID string) error {
	return fmt.Errorf("media file %s: %w: %w", mediaFileID, errFileChanged, jobs.RetryLater(fileChangedRetry, "the file changed during the read"))
}

// handleProbe runs ffprobe on a media file, stores codecs, resolution, duration and the probe
// JSON, replaces its tracks (embedded streams and sidecar subtitles) and stores the keyframe
// index from the container or queues the full read (concept 14, 2.2). Everything that reads the
// file or starts a process runs before the write transaction; the transaction only writes, and
// only for the file version (size, mtime) remembered from the row before the file was read.
func (s *Service) handleProbe(ctx context.Context, j *jobs.Job) error {
	t, err := s.loadProbeTarget(ctx, j)
	if err != nil || !t.found {
		return err
	}
	version := VersionOf(t.file)
	if t.file.Size == 0 {
		// Never playable; spares an ffprobe process (placeholder trees of the scan benchmark).
		return s.storeProbeError(ctx, t, version, errEmptyFile)
	}
	pr, err := s.probe.Probe(ctx, t.abs)
	if err != nil {
		var fileErr *transcode.FileError
		if ctx.Err() != nil || !errors.As(err, &fileErr) {
			// Interrupted, or the environment failed (ffprobe missing or not configured, no
			// process, an I/O error): not a property of the file, so the job retries with backoff.
			return fmt.Errorf("probe %s: %w", t.file.ID, err)
		}
		return s.storeProbeError(ctx, t, version, err)
	}
	r, err := paths.Open(t.root)
	if err != nil {
		return err
	}
	defer r.Close()
	sidecars, err := findSidecars(r, t.file.RelPath)
	if err != nil {
		return fmt.Errorf("list sidecar subtitles: %w", err)
	}
	raw, err := json.Marshal(pr)
	if err != nil {
		return fmt.Errorf("encode probe: %w", err)
	}

	params := systemq.UpdateMediaFileProbeParams{
		Container:  canonicalContainer(pr.Format.Name, t.file.Container),
		DurationMs: sql.NullInt64{Int64: pr.Format.Duration.Milliseconds(), Valid: pr.Format.Duration > 0},
		Probe:      sql.NullString{String: string(raw), Valid: true},
		ID:         t.file.ID,
		Size:       version.Size,
		Mtime:      version.Mtime,
	}
	if v := mainVideo(pr.Streams); v != nil {
		params.VideoCodec = sql.NullString{String: v.Codec, Valid: v.Codec != ""}
		params.Width = sql.NullInt64{Int64: int64(v.Width), Valid: v.Width > 0}
		params.Height = sql.NullInt64{Int64: int64(v.Height), Valid: v.Height > 0}
	}
	if a := mainAudio(pr.Streams); a != nil {
		params.AudioCodec = sql.NullString{String: a.Codec, Valid: a.Codec != ""}
	}

	item, err := systemq.New(s.db.Read).GetItemSystem(ctx, t.file.ItemID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("load item of the probed file: %w", err)
	}
	// The keyframe index of this version: from the container when it passes the checks, else the
	// full read (probe.keyframes). A file that has its index already (the scanner clears it when
	// the file changes) is not read again.
	var index containerIndex
	if !t.file.KeyframesSource.Valid {
		index, err = s.indexFromContainer(ctx, indexTarget{
			id: t.file.ID, root: t.root, dir: r, rel: t.file.RelPath, abs: t.abs, container: params.Container,
			duration: pr.Format.Duration, codec: params.VideoCodec.String, kind: item.Kind, muxer: pr.Format.Tags["encoder"],
		})
		if err != nil {
			return err
		}
	}
	var payload mediaFilePayload
	if err := decodePayload(j, &payload); err != nil {
		return err
	}
	// Whether chapters.images follows reads the cache directory, so it is decided before the
	// write transaction too.
	chapterImages := s.cacheDir != "" && chapterKind(item.Kind) && (payload.ChapterImages || s.hadChapterImages(t.file, item.Kind))

	now := s.now()
	params.Now = sql.NullInt64{Int64: now.UnixMilli(), Valid: true}
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	n, err := q.UpdateMediaFileProbe(ctx, params)
	if err != nil {
		return fmt.Errorf("store probe: %w", err)
	}
	if n == 0 {
		return fileChanged(t.file.ID)
	}
	if err := q.DeleteMediaFileTracks(ctx, t.file.ID); err != nil {
		return fmt.Errorf("delete tracks: %w", err)
	}
	for _, st := range pr.Streams {
		if st.AttachedPic || (st.Type != "video" && st.Type != "audio" && st.Type != "subtitle") {
			continue
		}
		lang := strings.ToLower(st.Language)
		if lang == "und" {
			lang = ""
		}
		if err := q.InsertTrack(ctx, systemq.InsertTrackParams{
			MediaFileID: t.file.ID, StreamIndex: int64(st.Index), Kind: st.Type,
			Codec:     sql.NullString{String: st.Codec, Valid: st.Codec != ""},
			Language:  sql.NullString{String: lang, Valid: lang != ""},
			Title:     sql.NullString{String: st.Title, Valid: st.Title != ""},
			Source:    "embedded",
			IsDefault: boolInt(st.Default), IsForced: boolInt(st.Forced),
			IsHearingImpaired: boolInt(hearingImpaired(st)),
		}); err != nil {
			return fmt.Errorf("insert track: %w", err)
		}
	}
	for i, sc := range sidecars {
		title := ""
		if sc.hearingImpaired {
			title = "SDH"
		}
		if err := q.InsertTrack(ctx, systemq.InsertTrackParams{
			MediaFileID: t.file.ID, StreamIndex: int64(i), Kind: "subtitle",
			Codec:    sql.NullString{String: sc.codec, Valid: true},
			Language: sql.NullString{String: sc.lang, Valid: sc.lang != ""},
			Title:    sql.NullString{String: title, Valid: title != ""},
			Source:   "sidecar", RelPath: sql.NullString{String: sc.rel, Valid: true},
			IsForced: boolInt(sc.forced), IsHearingImpaired: boolInt(sc.hearingImpaired),
		}); err != nil {
			return fmt.Errorf("insert sidecar track: %w", err)
		}
	}
	switch {
	case len(index.keyframes) > 0:
		if _, err := StoreKeyframes(ctx, tx, t.file.ID, index.keyframes, index.source, version, now); err != nil {
			return err
		}
	case index.worthy:
		if err := enqueueTx(ctx, q, now, keyframesJob(t.file.ID)); err != nil {
			return err
		}
	}
	if chapterImages {
		if err := enqueueChapterImagesTx(ctx, q, now, t.file.ID); err != nil {
			return err
		}
	}
	if err := markSeasonPending(ctx, q, t.file.ID, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if item.Kind == "podcast_episode" {
		return s.finishPodcastEpisode(ctx, item, t.file, &pr)
	}
	return s.finishAudiobook(ctx, t.file.ItemID)
}

// hadChapterImages reports whether the file had chapter images (or «done, no images») for the
// probe before this one: a re-probe (a refresh of the title, a changed file) then queues the
// images of the new probe, which the sweep would otherwise leave the file without (concept 13).
func (s *Service) hadChapterImages(f systemq.MediaFile, kind string) bool {
	if !f.ProbedAt.Valid || f.ProbeError.Valid {
		return false
	}
	_, _, version, ok := storedChapterImages(f.Probe, kind, f.DurationMs.Int64, f.ProbedAt.Int64)
	if !ok {
		return false
	}
	cache, err := paths.NewRoot(s.cacheDir)
	return err == nil && hasChapterImages(cache, ChapterImagesRel(f.ID, version))
}

// maxProbeError bounds the stored ffprobe message.
const maxProbeError = 500

var errEmptyFile = errors.New("empty file")

// storeProbeError records a failed ffprobe run (B-9 "unplayable"): probed_at and probe_error
// with the message, in which the absolute path is replaced by the file name, and no tracks. The
// job ends successfully, so a broken file is not retried; a changed file or a refresh probes again.
// Like the probe, the error is written only for the version v the run read: when the file changed
// meanwhile, the job runs again on the new version (concept 14, Z-9).
func (s *Service) storeProbeError(ctx context.Context, t probeTarget, v FileVersion, cause error) error {
	msg := strings.ReplaceAll(cause.Error(), t.abs, path.Base(t.file.RelPath))
	msg = strings.ReplaceAll(msg, t.root.Dir(), "")
	if len(msg) > maxProbeError {
		msg = strings.ToValidUTF8(msg[:maxProbeError], "")
	}
	s.log.WarnContext(ctx, "probe failed, file marked unplayable", "media_file_id", t.file.ID, "err", msg)
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	n, err := q.UpdateMediaFileProbeError(ctx, systemq.UpdateMediaFileProbeErrorParams{
		ProbeError: sql.NullString{String: msg, Valid: true}, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, ID: t.file.ID,
		Size: v.Size, Mtime: v.Mtime,
	})
	if err != nil {
		return fmt.Errorf("store probe error: %w", err)
	}
	if n == 0 {
		return fileChanged(t.file.ID)
	}
	if err := q.DeleteMediaFileTracks(ctx, t.file.ID); err != nil {
		return fmt.Errorf("delete tracks: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.finishAudiobook(ctx, t.file.ItemID) // a book completes without its broken part
}

// fullReadTimeout bounds the full keyframe read of probe.keyframes (concept 14, 6.3): 27.5 GB at
// 15 MB/s take about 31 minutes, the bound leaves a factor of four.
const fullReadTimeout = 2 * time.Hour

// handleKeyframes is probe.keyframes, the full keyframe read in the pool index (concept 14, 6.3).
// A file with an index, and a file whose video cannot be remuxed, has nothing to do (jobs.Skip).
// Otherwise the index from the container is tried first (cheap; it covers jobs queued before the
// container was read during probe.file), then ffprobe reads every packet of the video stream
// within fullReadTimeout. Keyframes are stored as scan, a read without one and a broken file
// (transcode.FileError) as none, both for the version remembered from the row; a missing file ends
// the job without an index, and every other failure is returned for the backoff, so the source
// stays NULL and the backfill tries again. Audiobooks and podcast episodes are not left out here:
// the automatic paths do not queue them, a playback does (2.3).
func (s *Service) handleKeyframes(ctx context.Context, j *jobs.Job) error {
	t, err := s.loadProbeTarget(ctx, j)
	if err != nil || !t.found {
		return err // a vanished row or a file marked missing ends quietly
	}
	version := VersionOf(t.file)
	if t.file.KeyframesSource.Valid || !transcode.CopyableVideo(t.file.VideoCodec.String) {
		jobs.Skip(ctx)
		return nil
	}
	// The index of a long file takes minutes; the note names the file relative to its root
	// (concept 09, 6.2), never the absolute path.
	jobs.Progress(ctx, 0, 0, t.file.RelPath)
	dir, err := paths.Open(t.root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.log.InfoContext(ctx, "library root missing, keyframe index not read", "media_file_id", t.file.ID, "path", t.file.RelPath)
			return nil
		}
		return err
	}
	defer dir.Close()
	index, err := s.indexFromContainer(ctx, indexTarget{
		id: t.file.ID, root: t.root, dir: dir, rel: t.file.RelPath, abs: t.abs, container: t.file.Container,
		duration: time.Duration(t.file.DurationMs.Int64) * time.Millisecond, codec: t.file.VideoCodec.String,
	})
	if err != nil {
		return err
	}
	if len(index.keyframes) > 0 {
		return s.storeReadIndex(ctx, t.file.ID, index.keyframes, index.source, version)
	}

	readCtx, cancel := context.WithTimeout(ctx, fullReadTimeout)
	defer cancel()
	kf, err := s.probe.Keyframes(readCtx, t.abs)
	var fileErr *transcode.FileError
	switch {
	case err == nil:
		// No keyframe in the whole read stores none (StoreKeyframes).
		return s.storeReadIndex(ctx, t.file.ID, kf, KeyframesScan, version)
	case ctx.Err() != nil:
		return fmt.Errorf("keyframes of media file %s: %w", t.file.ID, ctx.Err())
	case fileMissing(dir, t.file.RelPath, err):
		// A file gone for good is marked by the next scan; one gone for a moment (a network share
		// that dropped) is read by the next backfill. Five retries would not help either.
		s.log.InfoContext(ctx, "media file missing, keyframe index not read", "media_file_id", t.file.ID, "path", t.file.RelPath)
		return nil
	case readCtx.Err() == nil && errors.As(err, &fileErr):
		s.log.WarnContext(ctx, "full keyframe read failed on the file, index marked impossible", "media_file_id", t.file.ID,
			"path", t.file.RelPath, "err", strings.ReplaceAll(err.Error(), t.abs, t.file.RelPath))
		return s.storeReadIndex(ctx, t.file.ID, nil, KeyframesNone, version)
	}
	return fmt.Errorf("keyframes of media file %s: %w", t.file.ID, err)
}

// storeReadIndex stores the index of probe.keyframes for version v. When nothing is written and
// the row has no index but another version, the file changed during the read: the probe.file of
// the new version found this job under its key and queued nothing, so the job runs again on the
// new version instead of ending without an index (review of wave B, m-2).
func (s *Service) storeReadIndex(ctx context.Context, mediaFileID string, kf []time.Duration, source string, v FileVersion) error {
	stored, err := StoreKeyframes(ctx, s.db.Write, mediaFileID, kf, source, v, s.now())
	if err != nil || stored {
		return err
	}
	cur, err := systemq.New(s.db.Write).GetMediaFileSystem(ctx, mediaFileID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load media file: %w", err)
	}
	if !cur.KeyframesSource.Valid && !cur.MissingSince.Valid && VersionOf(cur) != v {
		return fileChanged(mediaFileID)
	}
	return nil
}

// fileMissing reports whether a failed read of rel means the file is not there: the error says
// so (ffprobe prints the operating system's message), or the file is gone now.
func fileMissing(dir *os.Root, rel string, err error) bool {
	if errors.Is(err, fs.ErrNotExist) || strings.Contains(strings.ToLower(err.Error()), "no such file or directory") {
		return true
	}
	_, serr := dir.Stat(rel)
	return errors.Is(serr, fs.ErrNotExist)
}

// keyframesFormat is the version byte of the keyframe blob.
const keyframesFormat = 1

// EncodeKeyframes encodes a keyframe index for media_files.keyframes:
//
//	byte 0      format version (1)
//	then        one unsigned varint (encoding/binary.AppendUvarint) per keyframe: its distance in
//	            milliseconds to the previous keyframe, the first one relative to 0
//
// Times are rounded to milliseconds; negative times count as 0, and the input is sorted first.
// About two bytes per keyframe for a GOP of two seconds.
func EncodeKeyframes(kf []time.Duration) []byte {
	sorted := slices.Clone(kf)
	slices.Sort(sorted)
	out := make([]byte, 0, 1+2*len(sorted))
	out = append(out, keyframesFormat)
	var prev int64
	for _, d := range sorted {
		ms := max(d.Round(time.Millisecond).Milliseconds(), prev)
		out = binary.AppendUvarint(out, uint64(ms-prev)) // ms >= prev by construction
		prev = ms
	}
	return out
}

// DecodeKeyframes reverses EncodeKeyframes.
func DecodeKeyframes(b []byte) ([]time.Duration, error) {
	if len(b) == 0 {
		return nil, nil
	}
	if b[0] != keyframesFormat {
		return nil, fmt.Errorf("keyframe index format %d not supported", b[0])
	}
	var out []time.Duration
	var ms uint64
	for i := 1; i < len(b); {
		d, n := binary.Uvarint(b[i:])
		if n <= 0 {
			return nil, errors.New("keyframe index corrupt")
		}
		ms += d
		out = append(out, time.Duration(ms)*time.Millisecond) //nolint:gosec // G115: decoded from our own encoding
		i += n
	}
	return out, nil
}

func mainVideo(streams []transcode.Stream) *transcode.Stream {
	for i := range streams {
		if streams[i].Type == "video" && !streams[i].AttachedPic {
			return &streams[i]
		}
	}
	return nil
}

func mainAudio(streams []transcode.Stream) *transcode.Stream {
	var first *transcode.Stream
	for i := range streams {
		if streams[i].Type != "audio" {
			continue
		}
		if streams[i].Default {
			return &streams[i]
		}
		if first == nil {
			first = &streams[i]
		}
	}
	return first
}

// canonicalContainer maps the ffprobe format name to the container column, keeping the file
// extension where it names the same format (mov/m4v/mp4, mkv/webm).
func canonicalContainer(formatName, ext string) string {
	f := strings.ToLower(formatName)
	switch {
	case strings.Contains(f, "matroska"):
		if ext == "webm" {
			return "webm"
		}
		return "mkv"
	case strings.Contains(f, "mp4") || strings.Contains(f, "mov"):
		if ext == "mov" || ext == "m4v" || ext == "m4a" || ext == "m4b" {
			return ext
		}
		return "mp4"
	case f == "avi":
		return "avi"
	case f == "mpegts":
		if ext == "m2ts" {
			return ext
		}
		return "ts"
	case f == "asf":
		return "wmv"
	case strings.HasPrefix(f, "mpeg"):
		return "mpg"
	}
	return ext
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// sidecar is an external subtitle next to a media file:
// <stem>[.<lang>][.forced|.hi|.sdh|.cc].{srt,ass,ssa,sub,vtt} (I-5).
type sidecar struct {
	rel             string
	lang            string
	codec           string
	forced          bool
	hearingImpaired bool
}

var (
	subtitleCodecs = map[string]string{"srt": "subrip", "ass": "ass", "ssa": "ssa", "sub": "sub", "vtt": "webvtt"}
	langRe         = regexp.MustCompile(`^([a-z]{2,3}(-[a-z]{2,4})?|[a-z]{4,15})$`) // "de", "pt-br", "german"
)

// findSidecars lists the subtitle files that belong to the media file rel, sorted by name.
func findSidecars(r *os.Root, rel string) ([]sidecar, error) {
	dir := path.Dir(rel)
	f, err := r.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && !Ignored(e.Name()) {
			names = append(names, e.Name())
		}
	}
	return sidecarsOf(rel, names), nil
}

// sidecarsOf returns the sidecars of the media file rel among the file names of its directory,
// sorted by path.
func sidecarsOf(rel string, names []string) []sidecar {
	dir := path.Dir(rel)
	stem := strings.TrimSuffix(path.Base(rel), path.Ext(rel))
	var out []sidecar
	for _, name := range names {
		if sc, ok := parseSidecar(stem, name); ok {
			sc.rel = path.Join(dir, name)
			out = append(out, sc)
		}
	}
	slices.SortFunc(out, func(a, b sidecar) int { return strings.Compare(a.rel, b.rel) })
	return out
}

// isSubtitle reports whether name has a subtitle extension (a sidecar candidate).
func isSubtitle(name string) bool {
	ext := strings.TrimPrefix(path.Ext(name), ".")
	_, ok := subtitleCodecs[strings.ToLower(ext)]
	return ok && !Ignored(name)
}

func parseSidecar(stem, name string) (sidecar, bool) {
	rest, ok := strings.CutPrefix(name, stem+".")
	if !ok {
		return sidecar{}, false
	}
	parts := strings.Split(rest, ".")
	codec, ok := subtitleCodecs[strings.ToLower(parts[len(parts)-1])]
	if !ok {
		return sidecar{}, false
	}
	sc := sidecar{codec: codec}
	for _, p := range parts[:len(parts)-1] {
		lp := strings.ToLower(p)
		switch {
		case lp == "forced":
			sc.forced = true
		case lp == "hi" || lp == "sdh" || lp == "cc":
			sc.hearingImpaired = true
		case lp == "default":
		case sc.lang == "" && langRe.MatchString(lp):
			sc.lang = lp
		default:
			// Any other part (a title, a second movie stem) means the file is not ours.
			return sidecar{}, false
		}
	}
	return sc, true
}

// finishPodcastEpisode brings a file episode of a podcast folder up to date after its probe
// (concept 4.5): the tags give the title (unless it only repeats the podcast's title, as
// Audiobookshelf writes it for some episodes), the publication date (RFC 2822 in date or
// releasedate) and the description (comment, else subtitle); the probe gives the duration. Then
// the file is paired with a feed episode of the same podcast when exactly one fits (the same
// publication minute, else the same title, duration within two percent): the feed episode takes
// the file and keeps its guid, enclosure and texts, the file episode leaves. No pair, or two
// candidates: nothing happens, both stay, the maintenance page lists them as file_and_feed.
func (s *Service) finishPodcastEpisode(ctx context.Context, item systemq.Item, file systemq.MediaFile, pr *transcode.Probe) error {
	if item.DeletedAt.Valid || !item.ParentID.Valid || item.EnclosureUrl.Valid {
		return nil // a feed episode that already holds its file needs no tags
	}
	podcast, err := systemq.New(s.db.Read).GetItemSystem(ctx, item.ParentID.String)
	if err != nil {
		return fmt.Errorf("load podcast of the episode: %w", err)
	}
	tags := map[string]string{}
	for k, v := range pr.Format.Tags {
		tags[strings.ToLower(k)] = strings.TrimSpace(v)
	}
	title := item.Title
	if t := tags["title"]; t != "" && !strings.EqualFold(t, podcast.Title) {
		title = t
	}
	published := item.PublishedAt
	for _, key := range []string{"date", "releasedate", "tdrl", "tdat"} {
		if ts, ok := parseTagDate(tags[key]); ok {
			published = sql.NullInt64{Int64: ts.UnixMilli(), Valid: true}
			break
		}
	}
	duration := sql.NullInt64{Int64: pr.Format.Duration.Milliseconds(), Valid: pr.Format.Duration > 0}
	description := cmp.Or(tags["comment"], tags["subtitle"])

	candidates, err := systemq.New(s.db.Read).PodcastListFeedEpisodesWithoutFile(ctx, item.ParentID)
	if err != nil {
		return fmt.Errorf("list feed episodes: %w", err)
	}
	var hits []systemq.PodcastListFeedEpisodesWithoutFileRow
	for _, c := range candidates {
		if metadataPairs(c.PublishedAt, c.SortTitle, c.DurationMs, published, SortTitle(title), duration) {
			hits = append(hits, c)
		}
	}

	now := s.now().UnixMilli()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	if len(hits) == 1 {
		// The feed episode takes the file and keeps everything the feed said; the file episode
		// leaves. Its texts stay with it -- the feed's description is the better one.
		feedEpisode := hits[0]
		if err := q.PodcastMoveMediaFiles(ctx, systemq.PodcastMoveMediaFilesParams{TargetID: feedEpisode.ID, Now: now, SourceID: item.ID}); err != nil {
			return fmt.Errorf("move file to feed episode: %w", err)
		}
		if err := q.PodcastSoftDeleteEpisode(ctx, systemq.PodcastSoftDeleteEpisodeParams{Now: sql.NullInt64{Int64: now, Valid: true}, ID: item.ID}); err != nil {
			return fmt.Errorf("retire file episode: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		s.log.InfoContext(ctx, "podcast file paired with its feed episode", "item_id", feedEpisode.ID, "media_file_id", file.ID)
		return nil
	}
	if err := q.PodcastUpdateEpisodeFromTags(ctx, systemq.PodcastUpdateEpisodeFromTagsParams{
		Title: title, SortTitle: SortTitle(title), PublishedAt: published, DurationMs: duration, Now: now, ID: item.ID,
	}); err != nil {
		return fmt.Errorf("update episode from tags: %w", err)
	}
	if description != "" {
		for _, lang := range []string{"de", "en"} {
			if err := q.UpsertItemText(ctx, systemq.UpsertItemTextParams{
				ItemID: item.ID, Lang: lang, Title: title, Overview: nullString(description), SortKey: sortKey(lang, title),
			}); err != nil {
				return fmt.Errorf("store episode text: %w", err)
			}
		}
	}
	return tx.Commit()
}

// metadataPairs applies the pairing rule of concept 4.5 between a feed episode and a file
// episode: the same publication minute, else the same sort title; a duration known on both sides
// must agree within two percent of the feed's.
func metadataPairs(feedPublished sql.NullInt64, feedSortTitle string, feedDuration sql.NullInt64,
	filePublished sql.NullInt64, fileSortTitle string, fileDuration sql.NullInt64,
) bool {
	sameMinute := feedPublished.Valid && filePublished.Valid && feedPublished.Int64/60000 == filePublished.Int64/60000
	sameTitle := feedSortTitle != "" && feedSortTitle == fileSortTitle
	if !sameMinute && !sameTitle {
		return false
	}
	if feedDuration.Valid && feedDuration.Int64 > 0 && fileDuration.Valid && fileDuration.Int64 > 0 {
		diff := feedDuration.Int64 - fileDuration.Int64
		if diff < 0 {
			diff = -diff
		}
		if diff*50 > feedDuration.Int64 {
			return false
		}
	}
	return true
}

// parseTagDate reads the date of an episode tag: RFC 2822 as Audiobookshelf writes it ("Fri, 13
// Feb 2026 06:54:00 +0100"), else RFC 3339, else a plain date.
func parseTagDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, "Mon, 2 Jan 2006 15:04:05 -0700", "2 Jan 2006 15:04:05 -0700",
		time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// hearingImpaired reports whether an embedded stream is a subtitle for the deaf and hard of
// hearing: ffprobe's disposition flag or its title (review step 9, m-7).
func hearingImpaired(st transcode.Stream) bool {
	return st.Type == "subtitle" && (st.HearingImpaired || trackpick.HearingImpairedTitle(st.Title))
}
