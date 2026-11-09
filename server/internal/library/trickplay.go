package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Trickplay (concept 18 part A, ADR 0027): the preview tiles of the seek bar of a film or an
// episode, one point every 10 s, 100 tiles per WebP sheet. A set is a folder per media file and
// file version, <mediaFileId>-<size>-<mtime>, below the cache directory: trickplay/ for a setting
// that stores, trickplay-session/ for one that keeps the set only while the file plays. The folder
// holds manifest.json (written first: grid, points, seek times, source of the points), one
// <sheet>.webp per finished sheet (written under a temporary name and renamed, so an existing sheet
// is finished) and the empty marker complete (written last). A run that stops keeps the manifest
// and the sheets; the next run makes only the missing ones. A complete set made again (a fallback
// set once the file has a keyframe index, a tile height that no longer fits) is made in the folder
// <name>.next beside it and replaces it when complete; the version of the set names the source and
// the tile height, so the new sheets get new URLs.
const (
	// TrickplayDir and TrickplaySessionDir are the folders of the stored and of the temporary
	// sets below the cache directory (concept 18, 6.3 and 6.5).
	TrickplayDir        = "trickplay"
	TrickplaySessionDir = "trickplay-session"
	// TrickplayInterval is the distance of the points (F-3).
	TrickplayInterval = 10 * time.Second
	// trickplayManifestName and trickplayCompleteName are the manifest and the marker of a set.
	trickplayManifestName = "manifest.json"
	trickplayCompleteName = "complete"
	// trickplayTempPrefix starts the name of a sheet or a manifest being written; the sweep
	// removes one older than chapterTempMaxAge (6 hours).
	trickplayTempPrefix = ".tmp-"
	// trickplayNextSuffix ends the folder of a set that is made again beside the complete one.
	trickplayNextSuffix = ".next"
)

// Job types of trickplay (concept 18 5.2, 6.3, 6.4).
const (
	JobTrickplay         = "trickplay.generate" // payload trickplayPayload, key trickplay:<mediaFileId>
	JobTrickplaySeries   = "trickplay.series"   // payload trickplaySeriesPayload, key trickplay_series:<seriesId>
	JobTrickplayBackfill = "trickplay.backfill" // key trickplay_backfill
)

// trickplayKey is the idempotency key of trickplay.generate of a media file.
func trickplayKey(mediaFileID string) string { return "trickplay:" + mediaFileID }

// Sources of the points of a set (manifest source, concept 18 5.4).
const (
	TrickplaySourceIndex    = "index"    // the next keyframe of each section from the keyframe index
	TrickplaySourceFallback = "fallback" // no index: the start of each section, the frame is the keyframe before it
)

// States of a set for the app (Trickplay.state).
const (
	TrickplayQueued   = "queued"
	TrickplayRunning  = "running"
	TrickplayComplete = "complete"
	TrickplayFailed   = "failed"
)

// TrickplayManifest is manifest.json of a set: the grid of the tiles and the points. SeekMs is where
// ffmpeg seeks for each point (a keyframe plus 1 ms, or the start of the section without an index);
// two points with the same seek time share the frame (a section without a keyframe of its own
// repeats the tile before it).
type TrickplayManifest struct {
	Version    string  `json:"version"` // TrickplaySetVersion
	IntervalMs int64   `json:"intervalMs"`
	TileWidth  int     `json:"tileWidth"`
	TileHeight int     `json:"tileHeight"`
	Columns    int     `json:"columns"`
	Rows       int     `json:"rows"`
	PointsMs   []int64 `json:"pointsMs"`
	SeekMs     []int64 `json:"seekMs"`
	Source     string  `json:"source"`
}

// Sheets returns the number of sheets of the set.
func (m TrickplayManifest) Sheets() int {
	per := m.Columns * m.Rows
	if per <= 0 {
		return 0
	}
	return (len(m.PointsMs) + per - 1) / per
}

// SheetPoints returns the range [from, to) of the points of sheet n.
func (m TrickplayManifest) SheetPoints(n int) (from, to int) {
	per := m.Columns * m.Rows
	from = min(n*per, len(m.PointsMs))
	return from, min(from+per, len(m.PointsMs))
}

// TrickplayVersion is the file version of the set of a media file: its size and mtime (FileVersion,
// so a new probe alone keeps the set, concept 18 5.1). It names the folder of the set.
func TrickplayVersion(v FileVersion) string {
	return strconv.FormatInt(v.Size, 10) + "-" + strconv.FormatInt(v.Mtime, 10)
}

// TrickplaySetVersion is the version of a set (manifest, Trickplay.version, the v of the sheet URLs
// and their ETag): the file version, the source of the points and the tile height,
// "<size>-<mtime>-<source><tileHeight>". A set made again from an index or at another tile height
// gets a new version, so a browser never keeps an immutable sheet of the set before.
func TrickplaySetVersion(fileVersion, source string, tileHeight int) string {
	return fileVersion + "-" + source + strconv.Itoa(tileHeight)
}

// trickplayOfFile reports whether the set version version belongs to the file version fileVersion.
func trickplayOfFile(version, fileVersion string) bool {
	return strings.HasPrefix(version, fileVersion+"-")
}

// TrickplaySetName is the folder name of the set of a media file at a version.
func TrickplaySetName(mediaFileID string, v FileVersion) string {
	return mediaFileID + "-" + TrickplayVersion(v)
}

// TrickplaySheetName is the file name of sheet n in its set.
func TrickplaySheetName(n int) string { return strconv.Itoa(n) + ".webp" }

// TrickplayGrid computes the manifest of a media file (concept 18, 5.4 and 7): one point per 10-s
// section of durationMs, tiles of transcode.TrickplayTileWidth and the height of the display aspect
// ratio of the video stream v, at the file version fileVersion (TrickplayVersion; the manifest
// carries TrickplaySetVersion). With a keyframe index each section takes the first keyframe at or
// after its start that no earlier section took, seeked at the keyframe plus 1 ms (the index rounds
// to milliseconds; a rounded-down value would land a whole GOP early in MP4); a section without a
// keyframe of its own repeats the point before it. Without an index (kf empty) every section seeks
// its start (source fallback): the frame is then the keyframe before it. A file without a duration
// has no points.
func TrickplayGrid(durationMs int64, kf []time.Duration, v transcode.Stream, fileVersion string) TrickplayManifest {
	m := TrickplayManifest{
		IntervalMs: TrickplayInterval.Milliseconds(), TileWidth: transcode.TrickplayTileWidth,
		TileHeight: transcode.TrickplayTileHeight(v), Columns: transcode.TrickplayColumns, Rows: transcode.TrickplayRows,
		PointsMs: []int64{}, SeekMs: []int64{}, Source: TrickplaySourceIndex,
	}
	if len(kf) == 0 {
		m.Source = TrickplaySourceFallback
	}
	m.Version = TrickplaySetVersion(fileVersion, m.Source, m.TileHeight)
	if durationMs <= 0 {
		return m
	}
	step := m.IntervalMs
	n := int((durationMs + step - 1) / step)
	if len(kf) == 0 {
		for k := range n {
			at := int64(k) * step
			m.PointsMs = append(m.PointsMs, at)
			m.SeekMs = append(m.SeekMs, at)
		}
		return m
	}
	next := 0        // first keyframe no section took yet
	last := int64(0) // keyframe time of the last point
	for k := range n {
		start, end := int64(k)*step, int64(k+1)*step
		for next < len(kf) && kf[next].Milliseconds() < start {
			next++
		}
		switch {
		case next < len(kf) && kf[next].Milliseconds() < end:
			last = kf[next].Milliseconds()
			m.PointsMs = append(m.PointsMs, last)
			m.SeekMs = append(m.SeekMs, last+1)
			next++
		case k == 0:
			// No keyframe in the first section: its first frame.
			m.PointsMs = append(m.PointsMs, 0)
			m.SeekMs = append(m.SeekMs, 0)
		default:
			m.PointsMs = append(m.PointsMs, last)
			m.SeekMs = append(m.SeekMs, m.SeekMs[k-1])
		}
	}
	return m
}

// TrickplaySheetOrder returns the order the sheets of m are made in from startMs (concept 18, 4):
// the sheet of the start position, then forwards to the end, then backwards to the first.
func TrickplaySheetOrder(m TrickplayManifest, startMs int64) []int {
	total := m.Sheets()
	if total == 0 {
		return nil
	}
	first := 0
	if startMs > 0 && m.IntervalMs > 0 {
		first = min(int(startMs/m.IntervalMs)/(m.Columns*m.Rows), total-1)
	}
	out := make([]int, 0, total)
	for n := first; n < total; n++ {
		out = append(out, n)
	}
	for n := first - 1; n >= 0; n-- {
		out = append(out, n)
	}
	return out
}

// TrickplayFile is what the state of a set needs to know of its media file.
type TrickplayFile struct {
	ID         string
	Version    FileVersion
	DurationMs int64
	Video      *transcode.Stream // main video stream of the stored probe; nil: no trickplay
	Keyframes  []time.Duration   // keyframe index; nil: none (yet)
}

// TrickplayStatus is the state of the set of a media file for the app (getTrickplay,
// PlaybackInfo.trickplay): the manifest of the set (or the grid the job will write, before it
// exists), the sheets that exist and the state from the marker and the job under the key
// trickplay:<id>: complete with the marker, running or queued while a job holds the key, failed
// when the last job ended without a complete set (failed, or done or cancelled without one: an
// unreadable file, a cancel, a setting that no longer wants it), queued before any job. ok is false
// for a file without video or duration. db is a read connection.
func TrickplayStatus(ctx context.Context, db *sql.DB, cacheDir string, f TrickplayFile) (m TrickplayManifest, sheets []int, state string, ok bool, err error) {
	if f.Video == nil || f.DurationMs <= 0 {
		return TrickplayManifest{}, nil, "", false, nil
	}
	version := TrickplayVersion(f.Version)
	m = TrickplayGrid(f.DurationMs, f.Keyframes, *f.Video, version)
	state = TrickplayQueued
	if cacheDir != "" {
		cache, err := paths.NewRoot(cacheDir)
		if err != nil {
			return TrickplayManifest{}, nil, "", false, err
		}
		if rel, found := findTrickplaySet(cache, TrickplaySetName(f.ID, f.Version)); found {
			if stored, rerr := readTrickplayManifest(cache, rel); rerr == nil && trickplayOfFile(stored.Version, version) {
				m = stored
			}
			names, rerr := paths.ReadDirNames(cache, rel, 0)
			if rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
				return TrickplayManifest{}, nil, "", false, fmt.Errorf("read trickplay set: %w", rerr)
			}
			complete := false
			for _, name := range names {
				if name == trickplayCompleteName {
					complete = true
				}
				if n, isSheet := trickplaySheetIndex(name); isSheet && n < m.Sheets() {
					sheets = append(sheets, n)
				}
			}
			slices.Sort(sheets)
			if complete {
				return m, sheets, TrickplayComplete, true, nil
			}
		}
	}
	read := systemq.New(db)
	key := sql.NullString{String: trickplayKey(f.ID), Valid: true}
	id, err := read.GetActiveJobIDByKey(ctx, key)
	switch {
	case err == nil:
		j, err := read.GetJob(ctx, id)
		if err != nil {
			return TrickplayManifest{}, nil, "", false, fmt.Errorf("trickplay job of %s: %w", f.ID, err)
		}
		if j.State == "running" {
			state = TrickplayRunning
		}
	case errors.Is(err, sql.ErrNoRows):
		ended, err := read.TrickplayJobEnded(ctx, key)
		if err != nil {
			return TrickplayManifest{}, nil, "", false, fmt.Errorf("last trickplay job of %s: %w", f.ID, err)
		}
		if ended {
			state = TrickplayFailed
		}
	default:
		return TrickplayManifest{}, nil, "", false, fmt.Errorf("trickplay job of %s: %w", f.ID, err)
	}
	return m, sheets, state, true, nil
}

// trickplaySheetIndex parses the file name of a finished sheet.
func trickplaySheetIndex(name string) (int, bool) {
	base, ok := strings.CutSuffix(name, ".webp")
	if !ok || base == "" || strings.HasPrefix(name, trickplayTempPrefix) {
		return 0, false
	}
	n, err := strconv.Atoi(base)
	if err != nil || n < 0 || strconv.Itoa(n) != base {
		return 0, false
	}
	return n, true
}

// findTrickplaySet returns the folder of the set name below the cache root: the stored one, else
// the temporary one.
func findTrickplaySet(cache paths.Root, name string) (rel string, found bool) {
	for _, dir := range []string{TrickplayDir, TrickplaySessionDir} {
		rel := dir + "/" + name
		abs, err := paths.Resolve(cache, rel)
		if err != nil {
			continue
		}
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			return rel, true
		}
	}
	return "", false
}

// TrickplaySetComplete reports whether the set of a media file at version v is complete, stored or
// temporary.
func TrickplaySetComplete(cacheDir, mediaFileID string, v FileVersion) bool {
	if cacheDir == "" {
		return false
	}
	cache, err := paths.NewRoot(cacheDir)
	if err != nil {
		return false
	}
	rel, found := findTrickplaySet(cache, TrickplaySetName(mediaFileID, v))
	if !found {
		return false
	}
	abs, err := paths.Resolve(cache, rel+"/"+trickplayCompleteName)
	if err != nil {
		return false
	}
	_, err = os.Stat(abs)
	return err == nil
}

// OpenTrickplaySheet opens sheet n of the set of a media file at version v below the cache
// directory (stored, else temporary); fs.ErrNotExist when it is not there. The file is opened
// through package paths (rule 4).
func OpenTrickplaySheet(cacheDir, mediaFileID string, v FileVersion, n int) (*os.File, error) {
	if cacheDir == "" || n < 0 {
		return nil, fs.ErrNotExist
	}
	cache, err := paths.NewRoot(cacheDir)
	if err != nil {
		return nil, err
	}
	rel, found := findTrickplaySet(cache, TrickplaySetName(mediaFileID, v))
	if !found {
		return nil, fs.ErrNotExist
	}
	root, err := paths.Open(cache)
	if err != nil {
		return nil, err
	}
	defer root.Close() // a file opened through the root stays usable after it is closed
	return root.Open(rel + "/" + TrickplaySheetName(n))
}

// TrickplaySetVersionOf returns the version of the set of a media file at version v (stored, else
// temporary) from its manifest; ok is false without a readable manifest of that file version.
func TrickplaySetVersionOf(cacheDir, mediaFileID string, v FileVersion) (version string, ok bool) {
	if cacheDir == "" {
		return "", false
	}
	cache, err := paths.NewRoot(cacheDir)
	if err != nil {
		return "", false
	}
	rel, found := findTrickplaySet(cache, TrickplaySetName(mediaFileID, v))
	if !found {
		return "", false
	}
	m, err := readTrickplayManifest(cache, rel)
	if err != nil || !trickplayOfFile(m.Version, TrickplayVersion(v)) {
		return "", false
	}
	return m.Version, true
}

// readTrickplayManifest reads manifest.json of the set at rel below the cache root.
func readTrickplayManifest(cache paths.Root, rel string) (TrickplayManifest, error) {
	abs, err := paths.Resolve(cache, rel+"/"+trickplayManifestName)
	if err != nil {
		return TrickplayManifest{}, err
	}
	b, err := os.ReadFile(abs) //nolint:gosec // G304: resolved through package paths below the cache root
	if err != nil {
		return TrickplayManifest{}, err
	}
	var m TrickplayManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return TrickplayManifest{}, fmt.Errorf("parse trickplay manifest: %w", err)
	}
	if m.Columns <= 0 || m.Rows <= 0 || m.TileHeight <= 0 || len(m.SeekMs) != len(m.PointsMs) {
		return TrickplayManifest{}, errors.New("trickplay manifest is malformed")
	}
	return m, nil
}

// ReadTrickplayManifest reads the manifest of the complete set of a media file at version v
// (stored, else temporary); ok is false when there is no complete set.
func ReadTrickplayManifest(cacheDir, mediaFileID string, v FileVersion) (m TrickplayManifest, ok bool) {
	if !TrickplaySetComplete(cacheDir, mediaFileID, v) {
		return TrickplayManifest{}, false
	}
	cache, err := paths.NewRoot(cacheDir)
	if err != nil {
		return TrickplayManifest{}, false
	}
	rel, found := findTrickplaySet(cache, TrickplaySetName(mediaFileID, v))
	if !found {
		return TrickplayManifest{}, false
	}
	m, err = readTrickplayManifest(cache, rel)
	return m, err == nil
}
