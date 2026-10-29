package offline

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Audio downloads (M6, design 3.5, O-1, O-3, H-4): an audiobook is one download (all its parts on
// one timeline), a podcast episode one download from its file or, without one, from its enclosure,
// which the preparation fetches into <dir>/<id>/source.<ext> first (Range resume, at most 2 GiB)
// and removes once the download is ready. The three qualities mean AAC 128, 96 and 64 kbit/s; the
// audio is copied when the source is AAC within 1.1 × the rate of the quality (a single part).

// audioKbps is the AAC rate of each download quality for audio titles.
var audioKbps = map[string]int{model.DownloadHigh: 128, model.DownloadMedium: 96, model.DownloadLow: 64}

// Limits of the enclosure fetch of a download.
const (
	maxSourceBytes      = 2 << 30
	sourceHeaderTimeout = 30 * time.Second
	maxSourceRedirects  = 5
	sourceName          = "source"
)

// downloadErrSourceUnavailable is the error code of a download whose enclosure could not be
// fetched in the last attempt.
const downloadErrSourceUnavailable = "source_unavailable"

// errSourceUnavailable marks a failed enclosure fetch; the last attempt fails the download with
// source_unavailable.
var errSourceUnavailable = errors.New("offline: enclosure not available")

// errSourceBroken marks a source file whose probe failed: it will not become usable by waiting, so
// the download fails with encode_failed at once.
var errSourceBroken = errors.New("offline: download source cannot be read as media")

// audioReader is the part of package visibility that audio downloads read (M6 stream A,
// visibility/audio.go): the parts of an audiobook in order and its book-wide chapters.
type audioReader interface {
	Parts(ctx context.Context, s visibility.Scope, itemID string) ([]visibility.MediaFileLocation, error)
	Chapters(ctx context.Context, s visibility.Scope, itemID string) ([]model.Chapter, error)
}

// sourceClients fetch enclosures: the response headers within 30 s, at most 5 redirects, http and
// https only, no overall limit (the body is capped at 2 GiB), no address in the local network
// unless the podcast's feed is there itself (package netguard).
var sourceClients = netguard.NewClients(http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > maxSourceRedirects || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
			return fmt.Errorf("%w: too many redirects or another scheme", errSourceUnavailable)
		}
		return nil
	},
}, sourceHeaderTimeout)

// isAudio reports whether a title kind downloads as audio.
func isAudio(kind string) bool { return kind == "audiobook" || kind == "podcast_episode" }

// downloadSource reads the source of a downloadable title: the media file of a movie or an
// episode (fileID, empty for the primary one), the parts of an audiobook, the file or the
// enclosure of a podcast episode.
func (s *Service) downloadSource(ctx context.Context, scope visibility.Scope, item model.ItemSummary, fileID string) (source, error) {
	if isAudio(item.Kind) {
		return s.audioSource(ctx, scope, item)
	}
	return s.sourceOf(ctx, scope, item, fileID)
}

// audioSource reads the source of an audiobook (its parts; one part like a film's file) or a
// podcast episode (its file, else its enclosure).
func (s *Service) audioSource(ctx context.Context, scope visibility.Scope, item model.ItemSummary) (source, error) {
	if item.Kind == "audiobook" && s.audio != nil {
		parts, err := s.audio.Parts(ctx, scope, item.ID)
		if err != nil {
			return source{}, err
		}
		if len(parts) > 1 {
			probes := make([]transcode.Probe, 0, len(parts))
			for _, p := range parts {
				pf, err := s.vis.MediaFileProbe(ctx, scope, item.ID, p.File.ID)
				if err != nil {
					return source{}, err
				}
				if probe := parseProbe(pf.Probe); probe != nil {
					probes = append(probes, *probe)
				}
			}
			src := source{item: item, loc: parts[0], parts: parts}
			if len(probes) == len(parts) {
				probe := transcode.ConcatProbe(probes)
				src.probe = &probe
			}
			return src, nil
		}
	}
	src, err := s.sourceOf(ctx, scope, item, "")
	if item.Kind != "podcast_episode" || !errors.Is(err, model.ErrNotFound) {
		return src, err
	}
	enc, err := s.vis.Enclosure(ctx, scope, item.ID)
	if err != nil {
		return source{}, err
	}
	return source{item: item, enclosure: &enc}, nil
}

// choose decides a download quality of a source: the video profile of a film or an episode, the
// AAC rate of an audio title.
func (src source) choose(prof model.DownloadProfile, audio int) choice {
	if !isAudio(src.item.Kind) {
		return decide(src.loc.File, src.probe, prof, audio)
	}
	multi := len(src.parts) > 1
	d := durationOf(src.loc.File, src.probe)
	if multi && src.probe == nil {
		d = 0 // the first part's duration is not the book's: the summed one of the item applies
	}
	if d <= 0 && src.enclosure != nil {
		d = time.Duration(src.enclosure.DurationMs) * time.Millisecond
	}
	if d <= 0 && src.item.DurationMs != nil {
		d = time.Duration(*src.item.DurationMs) * time.Millisecond
	}
	p, size := src.probe, src.loc.File.SizeBytes
	if p == nil && src.enclosure != nil {
		p, size = enclosureProbe(*src.enclosure, d), src.enclosure.Length
	}
	return decideAudio(p, prof, d, multi, size)
}

// enclosureProbe stands for the probe of an enclosure that is not fetched yet: one stereo audio
// stream of the codec its MIME type implies, the size and duration the feed names, so the estimate
// takes the copy decision the preparation will take. nil when the type names no known codec.
func enclosureProbe(enc visibility.Enclosure, d time.Duration) *transcode.Probe {
	mt, _, _ := mime.ParseMediaType(enc.Type)
	codec := map[string]string{"audio/mp4": "aac", "audio/x-m4a": "aac", "audio/aac": "aac", "audio/mpeg": "mp3", "audio/mp3": "mp3"}[mt]
	if codec == "" {
		return nil
	}
	return &transcode.Probe{Format: transcode.Format{Duration: d, Size: enc.Length},
		Streams: []transcode.Stream{{Index: 0, Type: "audio", Codec: codec, Channels: 2, Default: true}}}
}

// decideAudio picks the AAC target of an audio download in profile prof and estimates its size
// ((kbit/s) × duration / 8 × 1.02, from the source rate when copied). The audio is copied when the
// source is a single AAC file with at most two channels and at most 1.1 × the profile's rate.
func decideAudio(p *transcode.Probe, prof model.DownloadProfile, d time.Duration, multiPart bool, size int64) choice {
	kbps := audioKbps[prof.Quality]
	c := choice{Profile: prof, AudioKbps: kbps, DurationMs: d.Milliseconds()}
	audio := -1
	if a := selectedAudio(p, -1); a != nil {
		audio = a.Index
		src := sourceAudioKbps(p, d, size)
		if !multiPart && a.Codec == "aac" && a.Channels >= 1 && a.Channels <= 2 && src > 0 && float64(src) <= copyHeadroom*float64(kbps) {
			c.AudioCopy, c.AudioKbps = true, src
		}
	}
	c.Target = transcode.Target{Mode: transcode.ModeTranscode, AudioCopy: c.AudioCopy, SubtitleBurnIn: -1, AudioStream: audio,
		AudioBitrate: kbps, ConstantRate: true}
	c.EstimatedBytes = estimate(c.AudioKbps, d)
	return c
}

// sourceAudioKbps is the bit rate of an audio-only source: the container's, else size over
// duration; 0 when unknown.
func sourceAudioKbps(p *transcode.Probe, d time.Duration, size int64) int {
	if p != nil && p.Format.BitRate > 0 {
		return int(p.Format.BitRate / 1000)
	}
	if p != nil && p.Format.Size > 0 {
		size = p.Format.Size
	}
	if size <= 0 || d <= 0 {
		return 0
	}
	return int(float64(size) * 8 / d.Seconds() / 1000)
}

// planAudio resolves the source of an audio download and plans it: the parts of a multi-part
// audiobook (concatenated; parts whose probe failed are left out), its single file, or the
// enclosure of a podcast episode, fetched first from the URL the feed names now (the one stored
// with the download when the episode has none). A file that is not probed yet or not readable is
// errSourceWait, one whose probe failed errSourceBroken; a failed fetch errSourceUnavailable.
func (s *Service) planAudio(ctx context.Context, row systemq.Download, item systemq.GetDownloadItemRow) (planned, error) {
	prof, ok := model.DownloadProfileFor(row.Quality)
	if !ok {
		return planned{}, fmt.Errorf("download %s has unknown quality %q", row.ID, row.Quality)
	}
	var src transcode.Source
	var size int64
	multi, cleanup := false, ""
	switch {
	case row.SourceUrl.Valid && !row.MediaFileID.Valid:
		name, err := s.fetchSource(ctx, row.ID, item.FeedUrl.String, item.FeedOrigin.String, cmp.Or(item.EnclosureUrl.String, row.SourceUrl.String))
		if err != nil {
			return planned{}, err
		}
		dir, err := s.copyDir(row.ID)
		if err != nil {
			return planned{}, err
		}
		abs := filepath.Join(dir, name)
		probe, err := s.enc.Probe(ctx, abs)
		if err != nil {
			return planned{}, fmt.Errorf("%w: probe enclosure: %w", errSourceUnavailable, err)
		}
		src, cleanup = transcode.Source{Path: abs, Probe: probe}, row.ID+"/"+name
	default:
		parts, err := s.write.ListDownloadParts(ctx, row.ItemID)
		if err != nil {
			return planned{}, fmt.Errorf("list parts: %w", err)
		}
		if len(parts) == 0 {
			m, err := s.write.GetDownloadSource(ctx, row.MediaFileID.String)
			if err != nil {
				return planned{}, fmt.Errorf("get download source: %w", err)
			}
			if m.ProbeError.Valid {
				return planned{}, fmt.Errorf("%w: media file %s", errSourceBroken, m.ID)
			}
			parts = []systemq.ListDownloadPartsRow{{ID: m.ID, RelPath: m.RelPath, Size: m.Size, DurationMs: m.DurationMs, Probe: m.Probe, RootPath: m.RootPath}}
		}
		abs := make([]string, 0, len(parts))
		probes := make([]transcode.Probe, 0, len(parts))
		for _, p := range parts {
			probe := parseProbe(p.Probe.String)
			if probe == nil {
				return planned{}, fmt.Errorf("%w: media file %s is not probed yet", errSourceWait, p.ID)
			}
			root, err := paths.NewRoot(p.RootPath)
			if err != nil {
				return planned{}, err
			}
			file, err := paths.Resolve(root, p.RelPath)
			if err != nil {
				return planned{}, fmt.Errorf("resolve media file %s: %w", p.ID, err)
			}
			if err := readable(root, p.RelPath); err != nil {
				return planned{}, fmt.Errorf("%w: media file %s: %w", errSourceWait, p.ID, err)
			}
			abs, probes = append(abs, file), append(probes, *probe)
			size += p.Size
		}
		multi = len(parts) > 1
		if multi {
			src = transcode.Source{Parts: abs, Probe: transcode.ConcatProbe(probes)}
		} else {
			src = transcode.Source{Path: abs[0], Probe: probes[0]}
		}
	}
	c := decideAudio(&src.Probe, prof, src.Probe.Format.Duration, multi, size)
	plan, err := transcode.NewPlan(src, c.Target)
	if err != nil {
		return planned{}, fmt.Errorf("plan download %s: %w", row.ID, err)
	}
	return planned{src: src, plan: plan, choice: c, cleanup: cleanup}, nil
}

// fetchSource fetches the enclosure of a download into <dir>/<id>/source.<ext> and returns the file
// name. A complete file is reused; a partial one (source.<ext>.part) continues with a Range
// request guarded by If-Range with the validator (strong ETag, else Last-Modified) stored next to
// it, and only when the answer resumes at the byte the partial file ends with; anything else
// starts over. Failures are errSourceUnavailable; they name the enclosure's host only. feedURL is
// the podcast's feed: an enclosure in the local network is fetched only when the feed is there too
// and was subscribed by hand (feedOrigin, package netguard).
func (s *Service) fetchSource(ctx context.Context, id, feedURL, feedOrigin, rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("%w: invalid enclosure URL", errSourceUnavailable)
	}
	root, err := s.root()
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := root.MkdirAll(id, 0o750); err != nil {
		return "", err
	}
	name := sourceName + "." + sourceExt(u.Path)
	if info, err := root.Stat(id + "/" + name); err == nil && !info.IsDir() {
		return name, nil
	}
	part := id + "/" + name + ".part"
	validator := part + ".validator"
	f, err := root.OpenFile(part, os.O_RDWR|os.O_CREATE, 0o640)
	if err != nil {
		return "", err
	}
	defer f.Close()
	have, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errSourceUnavailable, netguard.Clean(err))
	}
	req.Header.Set("User-Agent", "Slouchcat")
	// The bytes of a resume must count the same representation: no transparent gzip.
	req.Header.Set("Accept-Encoding", "identity")
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
		if v, err := root.ReadFile(validator); err == nil && len(v) > 0 {
			req.Header.Set("If-Range", string(v))
		}
	}
	resp, err := sourceClients.For(ctx, feedURL, feedOrigin).Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errSourceUnavailable, netguard.Clean(err))
	}
	defer resp.Body.Close()
	first, total, rangeOK := contentRange(resp.Header.Get("Content-Range"))
	switch {
	case resp.StatusCode == http.StatusPartialContent && have > 0 && rangeOK && first == have:
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && have > 0 && rangeOK && total == have:
		// The partial file already holds everything.
		return name, s.completeSource(root, part, validator, id+"/"+name)
	case resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		// The host resumed elsewhere or the file changed size: start over with the next attempt.
		if err := f.Truncate(0); err != nil {
			return "", err
		}
		return "", fmt.Errorf("%w: resume at byte %d answered HTTP %d with range %q", errSourceUnavailable, have,
			resp.StatusCode, resp.Header.Get("Content-Range"))
	case resp.StatusCode == http.StatusOK:
		if err := f.Truncate(0); err != nil {
			return "", err
		}
		if have, err = f.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		v := resp.Header.Get("ETag")
		if v == "" || strings.HasPrefix(v, "W/") {
			v = resp.Header.Get("Last-Modified")
		}
		if err := root.WriteFile(validator, []byte(v), 0o640); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("%w: enclosure host answered HTTP %d", errSourceUnavailable, resp.StatusCode)
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxSourceBytes-have+1))
	switch {
	case err != nil:
		return "", fmt.Errorf("%w: %w", errSourceUnavailable, netguard.Clean(err))
	case have+n > maxSourceBytes:
		return "", fmt.Errorf("%w: enclosure larger than 2 GiB", errSourceUnavailable)
	}
	if err := f.Sync(); err != nil {
		return "", err
	}
	if err := s.completeSource(root, part, validator, id+"/"+name); err != nil {
		return "", err
	}
	s.log.InfoContext(ctx, "download source fetched", "download_id", id, "bytes", have+n)
	return name, nil
}

// completeSource renames the partial enclosure to its final name and drops its validator.
func (s *Service) completeSource(root *os.Root, part, validator, final string) error {
	if err := root.Rename(part, final); err != nil {
		return err
	}
	if err := root.Remove(validator); err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.log.Warn("remove download source validator", "path", validator, "err", err)
	}
	return nil
}

// contentRange parses a Content-Range header, "bytes first-last/total" or "bytes */total"; first
// is -1 in the second form, total -1 when it is "*".
func contentRange(v string) (first, total int64, ok bool) {
	spec, found := strings.CutPrefix(strings.TrimSpace(v), "bytes ")
	rng, size, found2 := strings.Cut(spec, "/")
	if !found || !found2 {
		return 0, 0, false
	}
	total = -1
	if size != "*" {
		if total, ok = parseBytes(size); !ok {
			return 0, 0, false
		}
	}
	if rng == "*" {
		return -1, total, true
	}
	lo, hi, found := strings.Cut(rng, "-")
	if !found {
		return 0, 0, false
	}
	first, ok = parseBytes(lo)
	last, ok2 := parseBytes(hi)
	if !ok || !ok2 || last < first {
		return 0, 0, false
	}
	return first, total, true
}

func parseBytes(v string) (int64, bool) {
	n, err := strconv.ParseInt(v, 10, 64)
	return n, err == nil && n >= 0
}

// sourceExt is the file extension of an enclosure from its URL path ("audio" when it has none of
// the known audio extensions).
func sourceExt(p string) string {
	switch ext := strings.ToLower(strings.TrimPrefix(path.Ext(p), ".")); ext {
	case "mp3", "m4a", "m4b", "mp4", "aac", "ogg", "opus", "flac", "wav":
		return ext
	}
	return "audio"
}

// removeSource deletes the fetched enclosure of a ready download (rel below the downloads root).
func (s *Service) removeSource(ctx context.Context, rel string) {
	if rel == "" {
		return
	}
	root, err := s.root()
	if err != nil {
		s.log.WarnContext(ctx, "open downloads directory", "err", err)
		return
	}
	defer root.Close()
	if err := root.Remove(rel); err != nil && !errors.Is(err, fs.ErrNotExist) {
		s.log.WarnContext(ctx, "remove download source", "path", rel, "err", err)
	}
}

// mediaTypeOf is the MIME type of a prepared download: audio/mp4 for audio titles.
func mediaTypeOf(kind string) string {
	if isAudio(kind) {
		return "audio/mp4"
	}
	return "video/mp4"
}
