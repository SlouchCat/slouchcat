package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// The trickplay jobs (concept 18, 5 and 6; ADR 0027).
//
//   - trickplay.generate (pool trickplay, one worker, windowable): makes the set of one media file,
//     sheet by sheet from the sheet of the start position, at most two ffmpeg processes. The
//     payload names the file, the start position and the origin of the job; the setting of the
//     library is read when the job starts and again after every sheet. A job that is no start job
//     yields to a waiting start job after each sheet. A failure ends the set as failed at once
//     (jobs.Final), the sheets stay, the next trigger tries again; only an attempt that an expired
//     lease cut short (an unclean stop) gets the second of its two attempts.
//   - trickplay.series (pool general, event): the look-ahead of «Beim Start alle Folgen»; queues
//     trickplay.generate for the other episodes in the scope of the library and ends.
//   - trickplay.backfill (schedule every 7 days, off by default): queues trickplay.generate for
//     every file of a library at «Im Voraus» without a complete set (or with a fallback set once
//     the file has a keyframe index).
//
// Jobs are queued through Enqueue and EnqueueOverridingWindow only, never with the pause override
// (5.3): a paused trickplay.generate stays paused, and jobs.MaxPauseOverrides does not count it.

// Origins of trickplay.generate, in their rank: a later trigger of a higher rank takes the job
// over (start before download before series before import before backfill).
const (
	TrickplayOriginStart    = "start"
	TrickplayOriginDownload = "download"
	TrickplayOriginSeries   = "series"
	TrickplayOriginImport   = "import"
	TrickplayOriginBackfill = "backfill"
)

// trickplayRank orders the origins.
var trickplayRank = map[string]int{
	TrickplayOriginBackfill: 1, TrickplayOriginImport: 2, TrickplayOriginSeries: 3,
	TrickplayOriginDownload: 4, TrickplayOriginStart: 5,
}

// Priorities of trickplay.generate per origin (5.3).
const (
	prioTrickplayStart      = 9
	prioTrickplayDownload   = 5
	prioTrickplaySeriesNext = 3 // the next two episodes of a look-ahead
	prioTrickplaySeries     = 2 // the rest of a look-ahead, throttled
	prioTrickplayImport     = 2
	prioTrickplayBackfill   = 1
	prioTrickplaySeriesJob  = 3 // trickplay.series itself
)

const (
	trickplaySeriesKeyPrefix = "trickplay_series:"
	trickplayBackfillKey     = "trickplay_backfill"
	trickplayBackfillEvery   = 7 * 24 * time.Hour
	// trickplayProcs is the number of ffmpeg processes of a set (F-7); one while a transcode keeps
	// the CPU busy, and for a throttled look-ahead job while any playback is open.
	trickplayProcs = 2
	// trickplayStartWait bounds how long a sheet waits while a transcode on the CPU has not written
	// the first segment of a start or a jump yet (the viewer waits for it; W-3); trickplayStartPoll
	// is how often it looks again.
	trickplayStartWait = 10 * time.Second
	trickplayStartPoll = 100 * time.Millisecond
	// trickplayPointTimeout bounds one point (a hanging share must not hold the one worker).
	trickplayPointTimeout = 15 * time.Second
	// trickplayMaxFailures consecutive broken points end the job as failed.
	trickplayMaxFailures = 5
	// trickplaySeriesFull is how many episodes of a look-ahead run at full speed (F-16).
	trickplaySeriesFull = 2
	// TrickplaySessionKeep is how long a set that does not store outlives the last open playback
	// session of its file (6.5).
	TrickplaySessionKeep = 10 * time.Minute
	// Free-space floor of the cache drive (6.3): below 10 GB, or below 10 % on a drive under
	// 100 GB, import, backfill and look-ahead stop and a start writes a temporary set.
	trickplayFloorBytes   = 10 << 30
	trickplayFloorPercent = 10
	// trickplayAttempts are the attempts of trickplay.generate: a failure ends it at once
	// (jobs.Final), the second is for an attempt that an expired lease cut short.
	trickplayAttempts = 2
)

// Notes of a run below the free-space floor on «Hintergrundjobs» (6.3; the progress note of the
// run, German like the other notes of the page).
const (
	trickplayFloorStopNote = "Zu wenig freier Speicher im Cache: Vorschau gestoppt"
	trickplayFloorTempNote = "Zu wenig freier Speicher im Cache: Vorschau nur vorübergehend"
)

// errTrickplayFailures ends a set after trickplayMaxFailures broken points in a row.
var errTrickplayFailures = errors.New("too many broken trickplay points in a row")

// trickplayPayload is the payload of trickplay.generate.
type trickplayPayload struct {
	MediaFileID string `json:"mediaFileId"`
	StartMs     int64  `json:"startMs,omitempty"`
	Origin      string `json:"origin"`
	// Throttled marks a look-ahead job beyond the next two episodes: one ffmpeg process while
	// any playback is open (F-16).
	Throttled bool `json:"throttled,omitempty"`
	// UnreadableSince is when an import or backfill job first found its file unreadable (Unix
	// ms; 0 while readable): it gives up a week after it.
	UnreadableSince int64 `json:"unreadableSince,omitempty"`
}

// merge returns the payload of a job that holds the key after a trigger asked for next: the origin
// of the higher rank wins and brings its start position; a start of the same rank moves the start
// position. A job stays throttled only while every trigger was a throttled look-ahead; the week of
// an unreadable file counts only for import and backfill.
func (p trickplayPayload) merge(next trickplayPayload) trickplayPayload {
	out := p
	if out.Origin == "" || trickplayRank[next.Origin] > trickplayRank[out.Origin] ||
		(next.Origin == TrickplayOriginStart && out.Origin == TrickplayOriginStart) {
		out.Origin, out.StartMs = next.Origin, next.StartMs
	}
	out.Throttled = out.Origin == TrickplayOriginSeries && p.Throttled && next.Throttled
	if out.Origin != TrickplayOriginImport && out.Origin != TrickplayOriginBackfill {
		out.UnreadableSince = 0
	}
	return out
}

// trickplaySeriesPayload is the payload of trickplay.series.
type trickplaySeriesPayload struct {
	SeriesID  string `json:"seriesId"`
	EpisodeID string `json:"episodeId"`
}

// TrickplayLoad tells whether a transcode keeps the CPU busy (transcode.Manager.CPUHeavy) and
// whether a transcode on the CPU waits for the first segment of a start or a jump
// (transcode.Manager.CPUStarting).
type TrickplayLoad interface {
	CPUHeavy() bool
	CPUStarting() bool
}

// WithTrickplayLoad sets the CPU load of the transcoder; without it a set always runs two ffmpeg
// processes (except a throttled look-ahead while a playback is open).
func WithTrickplayLoad(l TrickplayLoad) Option {
	return func(s *Service) { s.trickplayLoad = l }
}

// trickplayWanted reports whether a job of origin may (still) work under setting t (5.2): never at
// off; import and backfill only at ahead, the look-ahead only at series_start; a setting that does
// not store only while a playback of the file is open (a download makes its set all the same).
func trickplayWanted(t model.TrickplaySetting, origin string, playing bool) bool {
	switch {
	case t.Mode == "" || t.Mode == model.TrickplayOff:
		return false
	case origin == TrickplayOriginImport || origin == TrickplayOriginBackfill:
		return t.Mode == model.TrickplayAhead
	case origin == TrickplayOriginSeries:
		return t.Mode == model.TrickplaySeriesStart
	case origin == TrickplayOriginDownload:
		return true
	}
	return t.Stores() || playing
}

// trickplayFileSetting returns the setting of the kind of the file of row.
func trickplayFileSetting(r systemq.GetTrickplayFileRow) model.TrickplaySetting {
	return model.LibraryTrickplay{
		Movies: model.TrickplaySetting{Mode: r.TrickplayMovies, Keep: r.TrickplayMoviesKeep == 1},
		Series: model.TrickplaySetting{Mode: r.TrickplaySeries, Keep: r.TrickplaySeriesKeep == 1, Scope: r.TrickplaySeriesScope},
	}.For(r.Kind)
}

// enqueueTrickplay queues trickplay.generate for a file with the payload p at prio
// (trickplayAttempts). overrideWindow lets it start outside the time window of the type (start,
// look-ahead, download). A job that holds the key already takes p over by trickplayPayload.merge:
// a waiting one through SetQueuedPayload, a running one through SetRunningPayload (it reads the
// payload after every sheet); a waiting one is raised to prio, and for a start or a download it
// waits no longer for a later start (an import that waits an hour for an unreadable file), never
// with the pause override. It returns the id of the job.
func enqueueTrickplay(ctx context.Context, q *jobs.Queue, p trickplayPayload, prio int, overrideWindow bool) (int64, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return 0, fmt.Errorf("encode %s payload: %w", JobTrickplay, err)
	}
	nj := jobs.NewJob{Type: JobTrickplay, Priority: prio, IdempotencyKey: trickplayKey(p.MediaFileID), Payload: raw, MaxAttempts: trickplayAttempts}
	var id int64
	if overrideWindow {
		id, err = q.EnqueueOverridingWindow(ctx, nj)
	} else if id, err = q.Enqueue(ctx, nj); err == nil {
		_, err = q.RaiseQueuedPriority(ctx, id, prio)
	}
	if err == nil && (p.Origin == TrickplayOriginStart || p.Origin == TrickplayOriginDownload) {
		_, err = q.PullQueuedForward(ctx, id)
	}
	if err != nil {
		return 0, err
	}
	j, err := q.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	var cur trickplayPayload
	if json.Unmarshal(j.Payload, &cur) != nil {
		return id, nil
	}
	merged := cur.merge(p)
	if merged == cur {
		return id, nil
	}
	if raw, err = json.Marshal(merged); err != nil {
		return 0, fmt.Errorf("encode %s payload: %w", JobTrickplay, err)
	}
	switch j.State {
	case jobs.StateQueued:
		if ok, err := q.SetQueuedPayload(ctx, id, raw); err != nil || ok {
			return id, err
		}
		// Claimed meanwhile: the running job reads the payload after its sheet.
		_, err = q.SetRunningPayload(ctx, id, raw)
	case jobs.StateRunning:
		_, err = q.SetRunningPayload(ctx, id, raw)
	}
	return id, err
}

// TrickplayStart describes a playback start for QueueTrickplayForPlayback.
type TrickplayStart struct {
	MediaFileID string
	Version     FileVersion
	Kind        string // movie, episode
	StartMs     int64
	Setting     model.TrickplaySetting // of the title's kind in its library (visibility.MediaFileLocation)
	SeriesID    string                 // an episode's series; empty otherwise
	EpisodeID   string
}

// QueueTrickplayForPlayback is the start trigger (concept 18, 5.2 (1), 6.4): unless the setting is
// off or the title is no film or episode, it queues trickplay.generate at priority 9 with origin
// start through EnqueueOverridingWindow (never the pause override), when the file has no complete
// set; a job that holds the key takes the start position and the origin over. A complete
// temporary set of a setting that stores now is renamed into the stored folder (6.6). For an
// episode at series_start it queues the look-ahead trickplay.series of its series as well; a
// waiting one takes the new episode, a running one runs again for it.
func QueueTrickplayForPlayback(ctx context.Context, q *jobs.Queue, cacheDir string, st TrickplayStart) error {
	if cacheDir == "" || q == nil || !chapterKind(st.Kind) || st.Setting.Mode == "" || st.Setting.Mode == model.TrickplayOff {
		return nil
	}
	switch {
	case !TrickplaySetComplete(cacheDir, st.MediaFileID, st.Version):
		if _, err := enqueueTrickplay(ctx, q, trickplayPayload{MediaFileID: st.MediaFileID, StartMs: max(st.StartMs, 0), Origin: TrickplayOriginStart},
			prioTrickplayStart, true); err != nil {
			return err
		}
	case st.Setting.Stores():
		if err := keepTrickplaySession(cacheDir, TrickplaySetName(st.MediaFileID, st.Version)); err != nil {
			return err
		}
	}
	if st.Kind != "episode" || st.Setting.Mode != model.TrickplaySeriesStart || st.SeriesID == "" {
		return nil
	}
	raw, err := json.Marshal(trickplaySeriesPayload{SeriesID: st.SeriesID, EpisodeID: st.EpisodeID})
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", JobTrickplaySeries, err)
	}
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobTrickplaySeries, Priority: prioTrickplaySeriesJob,
		IdempotencyKey: trickplaySeriesKeyPrefix + st.SeriesID, Payload: raw})
	if err != nil {
		return err
	}
	// The newest episode wins while the look-ahead waits; a running one runs again for it
	// (handleTrickplaySeries reads its payload when it is through).
	if ok, err := q.SetQueuedPayload(ctx, id, raw); err != nil || ok {
		return err
	}
	_, err = q.SetRunningPayload(ctx, id, raw)
	return err
}

// keepTrickplaySession renames the complete temporary set name into the stored folder (6.6) when
// no stored set of that name exists; a set the maintenance removed meanwhile is no error.
func keepTrickplaySession(cacheDir, name string) error {
	cache, err := paths.NewRoot(cacheDir)
	if err != nil {
		return err
	}
	root, err := paths.Open(cache)
	if err != nil {
		return err
	}
	defer root.Close()
	stored, session := TrickplayDir+"/"+name, TrickplaySessionDir+"/"+name
	if _, err := root.Stat(stored); err == nil {
		return nil
	}
	switch _, err := root.Stat(session + "/" + trickplayCompleteName); {
	case errors.Is(err, fs.ErrNotExist):
		return nil // no complete temporary set: nothing to keep
	case err != nil:
		return fmt.Errorf("temporary trickplay set: %w", err)
	}
	if err := root.MkdirAll(TrickplayDir, 0o750); err != nil {
		return fmt.Errorf("create trickplay dir: %w", err)
	}
	if err := root.Rename(session, stored); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("keep temporary trickplay set: %w", err)
	}
	return syncTrickplayDir(root, TrickplayDir)
}

// QueueTrickplayForDownload asks for the complete set of a media file an offline download waits
// for (concept 18, 6.8): trickplay.generate at priority 5 with origin download through
// EnqueueOverridingWindow. queued reports whether a job is queued or running now; false when the
// set is complete, the file gets none (no film or episode, the setting off, gone) or there is no
// cache directory. db is a read connection.
func QueueTrickplayForDownload(ctx context.Context, db *sql.DB, q *jobs.Queue, cacheDir, mediaFileID string) (queued bool, err error) {
	if cacheDir == "" || q == nil {
		return false, nil
	}
	row, err := systemq.New(db).GetTrickplayFile(ctx, mediaFileID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load media file: %w", err)
	}
	setting := trickplayFileSetting(row)
	if !chapterKind(row.Kind) || setting.Mode == model.TrickplayOff || !row.ProbedAt.Valid {
		return false, nil
	}
	if TrickplaySetComplete(cacheDir, mediaFileID, FileVersion{Size: row.Size, Mtime: row.Mtime}) {
		return false, nil
	}
	if _, err := enqueueTrickplay(ctx, q, trickplayPayload{MediaFileID: mediaFileID, Origin: TrickplayOriginDownload}, prioTrickplayDownload, true); err != nil {
		return false, err
	}
	return true, nil
}

// trickplayFrame runs transcode.Runner.TrickplayFrame, or the test seam.
func (s *Service) trickplayFrame(ctx context.Context, in string, at time.Duration, height int, v transcode.Stream) ([]byte, error) {
	if s.trickplayFrameFn != nil {
		return s.trickplayFrameFn(ctx, in, at, height, v)
	}
	return s.probe.TrickplayFrame(ctx, in, at, height, v)
}

// trickplayProbe runs transcode.Runner.Probe, or the test seam.
func (s *Service) trickplayProbe(ctx context.Context, in string) (transcode.Probe, error) {
	if s.trickplayProbeFn != nil {
		return s.trickplayProbeFn(ctx, in)
	}
	return s.probe.Probe(ctx, in)
}

// trickplaySheet runs transcode.Runner.TrickplaySheet, or the test seam.
func (s *Service) trickplaySheet(ctx context.Context, tiles [][]byte, height int, out string) error {
	if s.trickplaySheetFn != nil {
		return s.trickplaySheetFn(ctx, tiles, height, out)
	}
	return s.probe.TrickplaySheet(ctx, tiles, height, out)
}

// trickplaySpaceOK reports whether the cache drive is above the free-space floor (6.3); an unknown
// free space counts as above it.
func (s *Service) trickplaySpaceOK() bool {
	total, free, err := s.diskSpace(s.cacheDir)
	if err != nil || total <= 0 {
		return true
	}
	floor := int64(trickplayFloorBytes)
	if pct := total * trickplayFloorPercent / 100; pct < floor {
		floor = pct
	}
	return free >= floor
}

// trickplayRun is one run of trickplay.generate.
type trickplayRun struct {
	s       *Service
	j       *jobs.Job
	p       trickplayPayload
	read    *systemq.Queries
	cache   paths.Root
	root    *os.Root
	rel     string // folder of the set below the cache root
	rootDir string // library root of the media file (the key of Service.readable)
	in      string // absolute path of the media file
	video   transcode.Stream
	m       TrickplayManifest
	version FileVersion
	// next is set while a complete set is made again in rel (<name>.next); the last sheet
	// replaces the complete one with it.
	next bool
	// failures counts the broken points in a row across sheets and workers.
	failures atomic.Int64
	// note is the progress note of the run (below the free-space floor).
	note string
}

// handleTrickplay makes the set of one media file (concept 18, 5): see the comment at the top of
// this file and the steps below.
//
//   - A file that is gone, missing, of a deleted title, not probed, without video or duration, or
//     whose set is complete ends the job without work; so does a setting that no longer wants the
//     origin of the job (trickplayWanted).
//   - The set goes to trickplay/ when the setting stores and the cache drive is above the floor,
//     else to trickplay-session/; a start below the floor writes a temporary set, import,
//     backfill and look-ahead end. A temporary set of a file whose setting stores now is renamed.
//   - A file that cannot be read: start, look-ahead, download and a setting that does not store end
//     without retry; import and backfill wait an hour, for a week at most.
//   - After every sheet: the payload (start position, origin), the setting, the file version and a
//     waiting start job are read again.
//   - Every error but a RetryLater ends the job at once (jobs.Final).
func (s *Service) handleTrickplay(ctx context.Context, j *jobs.Job) error {
	err := s.runTrickplay(ctx, j)
	var later *jobs.RetryLaterError
	if err != nil && !errors.As(err, &later) {
		return jobs.Final(err)
	}
	return err
}

// runTrickplay is one run of handleTrickplay.
func (s *Service) runTrickplay(ctx context.Context, j *jobs.Job) error {
	var p trickplayPayload
	if err := decodePayload(j, &p); err != nil {
		return err
	}
	if s.cacheDir == "" {
		return nil
	}
	r := &trickplayRun{s: s, j: j, p: p, read: systemq.New(s.db.Read)}
	row, ok, err := r.load(ctx)
	if err != nil || !ok {
		return err
	}
	setting := trickplayFileSetting(row)
	playing, err := r.filePlaying(ctx)
	if err != nil {
		return err
	}
	if !trickplayWanted(setting, p.Origin, playing) {
		s.log.InfoContext(ctx, "trickplay not wanted by the library setting", "media_file_id", row.ID, "origin", p.Origin, "mode", setting.Mode)
		return nil
	}
	stores := setting.Stores() && s.trickplaySpaceOK()
	if setting.Stores() && !stores {
		if p.Origin != TrickplayOriginStart && p.Origin != TrickplayOriginDownload {
			s.log.WarnContext(ctx, "cache drive below the free-space floor, trickplay stops", "media_file_id", row.ID, "origin", p.Origin)
			jobs.Progress(ctx, 0, 0, trickplayFloorStopNote)
			return nil
		}
		s.log.WarnContext(ctx, "cache drive below the free-space floor, temporary trickplay set", "media_file_id", row.ID)
		r.note = trickplayFloorTempNote
		jobs.Progress(ctx, 0, 0, r.note)
	}
	if r.cache, err = paths.NewRoot(s.cacheDir); err != nil {
		return err
	}
	if r.root, err = paths.Open(r.cache); err != nil {
		return err
	}
	defer r.root.Close()
	if err := r.place(stores); err != nil {
		return err
	}
	want := r.grid(row)
	_, completeErr := r.root.Stat(r.rel + "/" + trickplayCompleteName)
	stored, storedErr := readTrickplayManifest(r.cache, r.rel)
	usable := storedErr == nil && r.fits(stored, want)
	switch {
	case usable && completeErr == nil:
		return nil
	case usable:
		r.m = stored
	case storedErr == nil && completeErr == nil:
		// A complete set made again (5.4: a fallback set once the file has an index; a tile
		// height that no longer fits): beside it, the complete one stays until the new one is.
		r.next = true
		r.rel += trickplayNextSuffix
	}
	r.rootDir = row.RootPath
	media, err := paths.NewRoot(row.RootPath)
	if err != nil {
		return err
	}
	if r.in, err = paths.Resolve(media, row.RelPath); err != nil {
		return err
	}
	if err := s.readable(row.RootPath, r.in, chapterReadTimeout); err != nil {
		return r.unreadable(ctx, stores, err)
	}
	if p.UnreadableSince != 0 {
		r.p.UnreadableSince = 0
		r.storePayload(ctx)
	}
	if r.m.Version == "" {
		if err := r.manifest(ctx, row, want); err != nil {
			return err
		}
	}
	return r.sheets(ctx)
}

// grid computes the manifest of the file of the run from its stored probe and keyframe index.
func (r *trickplayRun) grid(row systemq.GetTrickplayFileRow) TrickplayManifest {
	var kf []time.Duration
	switch row.KeyframesSource.String {
	case KeyframesCues, KeyframesMP4, KeyframesScan:
		if decoded, err := DecodeKeyframes(row.Keyframes); err == nil {
			kf = decoded
		}
	}
	return TrickplayGrid(row.DurationMs.Int64, kf, r.video, TrickplayVersion(r.version))
}

// fits reports whether the set of manifest m serves the file as want would: the same file
// version, not a fallback set while the file has an index now, and the tile height of the sample
// aspect ratio when the stored probe has one (a set of a probe without one took it from the file,
// see manifest).
func (r *trickplayRun) fits(m, want TrickplayManifest) bool {
	switch {
	case !trickplayOfFile(m.Version, TrickplayVersion(r.version)):
		return false
	case m.Source == TrickplaySourceFallback && want.Source == TrickplaySourceIndex:
		return false
	case r.video.SAR != "" && m.TileHeight != want.TileHeight:
		return false
	}
	return true
}

// load reads the file of the job; ok is false when there is nothing to do.
func (r *trickplayRun) load(ctx context.Context) (systemq.GetTrickplayFileRow, bool, error) {
	row, err := r.read.GetTrickplayFile(ctx, r.p.MediaFileID)
	if errors.Is(err, sql.ErrNoRows) {
		return row, false, nil
	}
	if err != nil {
		return row, false, fmt.Errorf("load media file: %w", err)
	}
	if !chapterKind(row.Kind) || !row.ProbedAt.Valid || row.MissingSince.Valid || !row.DurationMs.Valid || row.DurationMs.Int64 <= 0 {
		return row, false, nil
	}
	var pr transcode.Probe
	if !row.Probe.Valid || json.Unmarshal([]byte(row.Probe.String), &pr) != nil {
		r.s.log.WarnContext(ctx, "stored probe unreadable, no trickplay", "media_file_id", row.ID)
		return row, false, nil
	}
	v := mainVideo(pr.Streams)
	if v == nil {
		return row, false, nil
	}
	r.video, r.version = *v, FileVersion{Size: row.Size, Mtime: row.Mtime}
	return row, true, nil
}

// filePlaying reports whether a playback session of the file is open (neither revoked nor expired).
func (r *trickplayRun) filePlaying(ctx context.Context) (bool, error) {
	now := r.s.now()
	n, err := r.read.CountActivePlaybacksOfFile(ctx, systemq.CountActivePlaybacksOfFileParams{
		MediaFileID: r.p.MediaFileID, Now: now.UnixMilli(), ActiveAfter: 0,
	})
	if err != nil {
		return false, fmt.Errorf("count playbacks of the file: %w", err)
	}
	return n > 0, nil
}

// place picks the folder of the set (stored or temporary) and creates it; a temporary set of a
// file whose setting stores now is renamed into the stored folder (6.6).
func (r *trickplayRun) place(stores bool) error {
	name := TrickplaySetName(r.p.MediaFileID, r.version)
	stored, session := TrickplayDir+"/"+name, TrickplaySessionDir+"/"+name
	if err := r.root.MkdirAll(TrickplayDir, 0o750); err != nil {
		return fmt.Errorf("create trickplay dir: %w", err)
	}
	if err := r.root.MkdirAll(TrickplaySessionDir, 0o750); err != nil {
		return fmt.Errorf("create trickplay dir: %w", err)
	}
	for _, dir := range []string{stored, session} {
		// A stop between removing a set made again and renaming its successor (sheets).
		_, mainErr := r.root.Stat(dir)
		_, nextErr := r.root.Stat(dir + trickplayNextSuffix + "/" + trickplayCompleteName)
		if errors.Is(mainErr, fs.ErrNotExist) && nextErr == nil {
			if err := r.root.Rename(dir+trickplayNextSuffix, dir); err != nil {
				return fmt.Errorf("restore trickplay set: %w", err)
			}
		}
	}
	_, storedErr := r.root.Stat(stored)
	_, sessionErr := r.root.Stat(session)
	switch {
	case storedErr == nil:
		r.rel = stored // a stored set is used until the maintenance removes it
	case stores && sessionErr == nil:
		if err := r.root.Rename(session, stored); err != nil {
			return fmt.Errorf("keep temporary trickplay set: %w", err)
		}
		r.rel = stored
	case stores:
		r.rel = stored
	default:
		r.rel = session
	}
	if err := r.root.MkdirAll(r.rel, 0o750); err != nil {
		return fmt.Errorf("create trickplay set: %w", err)
	}
	return nil
}

// unreadable ends or defers a job whose media file cannot be read at its start (5.5).
func (r *trickplayRun) unreadable(ctx context.Context, stores bool, cause error) error {
	if !stores || (r.p.Origin != TrickplayOriginImport && r.p.Origin != TrickplayOriginBackfill) {
		r.s.log.WarnContext(ctx, "media file unreadable, no trickplay", "media_file_id", r.p.MediaFileID, "origin", r.p.Origin, "err", cause)
		return nil
	}
	now := r.s.now()
	if r.p.UnreadableSince == 0 {
		r.p.UnreadableSince = now.UnixMilli()
		r.storePayload(ctx)
	} else if now.Sub(time.UnixMilli(r.p.UnreadableSince)) > chapterUnreadableGiveUp {
		r.s.log.WarnContext(ctx, "media file unreadable for a week, trickplay given up", "media_file_id", r.p.MediaFileID, "err", cause)
		return nil
	}
	r.s.log.WarnContext(ctx, "media file unreadable, trickplay waits", "media_file_id", r.p.MediaFileID, "err", cause)
	return jobs.RetryLater(chapterUnreadableRetry, "the media file is unreadable")
}

// storePayload stores the payload of the run in its job; a failure only loses the note (logged).
func (r *trickplayRun) storePayload(ctx context.Context) {
	raw, err := json.Marshal(r.p)
	if err == nil {
		_, err = r.s.queue.SetRunningPayload(ctx, r.j.ID, raw)
	}
	if err != nil {
		r.s.log.WarnContext(ctx, "store trickplay job payload", "job_id", r.j.ID, "err", err)
	}
}

// manifest starts the set in rel (5.1): the manifest of a set made again before (rel is
// <name>.next) when it fits, else an empty folder with the manifest of want, written first. A stored
// probe without a sample aspect ratio (probes before contract 1.23.0 carry none) is completed from
// the file once, so an anamorphic source gets undistorted tiles.
func (r *trickplayRun) manifest(ctx context.Context, row systemq.GetTrickplayFileRow, want TrickplayManifest) error {
	if m, err := readTrickplayManifest(r.cache, r.rel); err == nil && r.fits(m, want) {
		r.m = m
		return nil
	}
	if r.video.SAR == "" {
		if pr, err := r.s.trickplayProbe(ctx, r.in); err != nil {
			r.s.log.WarnContext(ctx, "probe for the sample aspect ratio failed, square pixels", "media_file_id", r.p.MediaFileID, "err", err)
		} else {
			for _, v := range pr.Streams {
				if v.Index == r.video.Index && v.SAR != "" {
					r.video.SAR = v.SAR
					want = r.grid(row)
				}
			}
		}
	}
	if err := r.root.RemoveAll(r.rel); err != nil { //nolint:forbidigo // the cache root (trickplay), never a library root (ADR 0032)
		return fmt.Errorf("clear trickplay set: %w", err)
	}
	if err := r.root.MkdirAll(r.rel, 0o750); err != nil {
		return fmt.Errorf("create trickplay set: %w", err)
	}
	r.m = want
	raw, err := json.Marshal(r.m)
	if err != nil {
		return fmt.Errorf("encode trickplay manifest: %w", err)
	}
	return r.writeFile(trickplayManifestName, raw)
}

// writeFile writes name into the set under a temporary name, flushes it to the drive and renames
// it, so a power loss leaves no empty manifest or marker.
func (r *trickplayRun) writeFile(name string, data []byte) error {
	tmp := r.rel + "/" + trickplayTempPrefix + name + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	f, err := r.root.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return fmt.Errorf("write trickplay %s: %w", name, err)
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = r.root.Remove(tmp) //nolint:forbidigo // the cache root (trickplay), never a library root (ADR 0032)
		return fmt.Errorf("write trickplay %s: %w", name, err)
	}
	return r.store(tmp, name)
}

// store renames the flushed file tmp to name in the set and flushes the folder.
func (r *trickplayRun) store(tmp, name string) error {
	if err := r.root.Rename(tmp, r.rel+"/"+name); err != nil {
		_ = r.root.Remove(tmp) //nolint:forbidigo // the cache root (trickplay), never a library root (ADR 0032)
		return fmt.Errorf("store trickplay %s: %w", name, err)
	}
	return syncTrickplayDir(r.root, r.rel)
}

// syncTrickplayDir flushes the folder rel below root (the names of a rename).
func syncTrickplayDir(root *os.Root, rel string) error {
	d, err := root.Open(rel)
	if err != nil {
		return fmt.Errorf("open trickplay folder: %w", err)
	}
	err = d.Sync()
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("flush trickplay folder: %w", err)
	}
	return nil
}

// sheets makes the missing sheets in the order of the start position and writes the marker.
func (r *trickplayRun) sheets(ctx context.Context) error {
	total := r.m.Sheets()
	done := map[int]bool{}
	for n := range total {
		if _, err := r.root.Stat(r.rel + "/" + TrickplaySheetName(n)); err == nil {
			done[n] = true
		}
	}
	for len(done) < total {
		jobs.Progress(ctx, int64(len(done)), int64(total), r.note)
		n := -1
		for _, k := range TrickplaySheetOrder(r.m, r.p.StartMs) {
			if !done[k] {
				n = k
				break
			}
		}
		if err := r.sheet(ctx, n); err != nil {
			return err
		}
		done[n] = true
		if len(done) == total {
			break
		}
		if stop, err := r.recheck(ctx); stop || err != nil {
			return err
		}
	}
	jobs.Progress(ctx, int64(total), int64(total), r.note)
	if err := r.writeFile(trickplayCompleteName, nil); err != nil {
		return err
	}
	if r.next {
		// The set made again replaces the complete one; a stop between the two steps is
		// repaired by place.
		main := strings.TrimSuffix(r.rel, trickplayNextSuffix)
		if err := r.root.RemoveAll(main); err != nil { //nolint:forbidigo // the cache root (trickplay), never a library root (ADR 0032)
			return fmt.Errorf("remove the trickplay set made again: %w", err)
		}
		if err := r.root.Rename(r.rel, main); err != nil {
			return fmt.Errorf("store the trickplay set made again: %w", err)
		}
		r.rel = main
		if err := syncTrickplayDir(r.root, main[:strings.LastIndex(main, "/")]); err != nil {
			return err
		}
	}
	r.s.log.InfoContext(ctx, "trickplay set written", "media_file_id", r.p.MediaFileID, "sheets", total, "points", len(r.m.PointsMs),
		"source", r.m.Source, "stored", strings.HasPrefix(r.rel, TrickplayDir+"/"))
	return nil
}

// yieldToStart waits, at most trickplayStartWait, while a transcode on the CPU has not written the
// first segment of a start or a jump: the frames of a sheet decode the same file on the same CPU,
// and the viewer waits for that segment. On the playback start of a file without set
// (QueueTrickplayForPlayback) the set would otherwise be made while the first segment is encoded.
func (r *trickplayRun) yieldToStart(ctx context.Context) error {
	if r.s.trickplayLoad == nil {
		return nil
	}
	deadline := time.Now().Add(trickplayStartWait)
	for r.s.trickplayLoad.CPUStarting() && time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(trickplayStartPoll):
		}
	}
	return nil
}

// procs returns the ffmpeg processes of the next sheet (5.3).
func (r *trickplayRun) procs(ctx context.Context) int {
	if r.s.trickplayLoad != nil && r.s.trickplayLoad.CPUHeavy() {
		return 1
	}
	if r.p.Origin == TrickplayOriginSeries && r.p.Throttled {
		n, err := r.read.CountOpenPlaybacks(ctx, r.s.now().UnixMilli())
		if err != nil || n > 0 {
			return 1
		}
	}
	return trickplayProcs
}

// sheet grabs the points of sheet n and writes it. A broken point (transcode.FrameFailed) is filled
// black and logged; failures counts broken points in a row across sheets, and the
// trickplayMaxFailures-th ends the job at once (the workers stop the sheet, 5.5). The first error of
// a sheet that is no frame failure (a timeout, the share went away) checks the file: when it cannot
// be read any more, the job ends at once; else it counts like a broken point.
func (r *trickplayRun) sheet(ctx context.Context, n int) error {
	if err := r.yieldToStart(ctx); err != nil {
		return err
	}
	from, to := r.m.SheetPoints(n)
	tiles := make([][]byte, to-from)
	sctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	var checked sync.Once
	var wg sync.WaitGroup
	next := make(chan int)
	for range min(r.procs(ctx), to-from) {
		wg.Go(func() {
			for k := range next {
				if sctx.Err() != nil {
					// The feed may still hand out points after a stop (select picks either ready
					// case); none of them is grabbed.
					continue
				}
				if k > from && r.m.SeekMs[k] == r.m.SeekMs[k-1] {
					continue // the point repeats the one before it; filled below
				}
				pctx, cancel := context.WithTimeout(sctx, trickplayPointTimeout)
				tile, err := r.s.trickplayFrame(pctx, r.in, time.Duration(r.m.SeekMs[k])*time.Millisecond, r.m.TileHeight, r.video)
				cancel()
				if err == nil {
					tiles[k-from] = tile
					r.failures.Store(0)
					continue
				}
				if sctx.Err() != nil {
					continue // the sheet ends
				}
				if !transcode.FrameFailed(err) {
					checked.Do(func() {
						if rerr := r.s.readable(r.rootDir, r.in, chapterReadTimeout); rerr != nil {
							stop(fmt.Errorf("media file of %s unreadable during trickplay: %w", r.p.MediaFileID, rerr))
						}
					})
					if sctx.Err() != nil {
						continue
					}
				}
				r.s.log.WarnContext(ctx, "trickplay point failed", "media_file_id", r.p.MediaFileID, "point", k, "err", err)
				if r.failures.Add(1) >= trickplayMaxFailures {
					stop(fmt.Errorf("%w: %s at point %d: %w", errTrickplayFailures, r.p.MediaFileID, k, err))
				}
			}
		})
	}
feed:
	for k := from; k < to; k++ {
		select {
		case next <- k:
		case <-sctx.Done():
			break feed
		}
	}
	close(next)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := context.Cause(sctx); err != nil {
		return err
	}
	black := transcode.BlackTrickplayTile(r.m.TileHeight)
	for i := range tiles {
		k := from + i
		switch {
		case k > from && r.m.SeekMs[k] == r.m.SeekMs[k-1]:
			tiles[i] = tiles[i-1]
		case tiles[i] == nil:
			tiles[i] = black
		}
	}
	tmp := r.rel + "/" + trickplayTempPrefix + strconv.Itoa(n) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".webp"
	out, err := paths.Resolve(r.cache, tmp)
	if err != nil {
		return err
	}
	if err := r.s.trickplaySheet(ctx, tiles, r.m.TileHeight, out); err != nil {
		_ = r.root.Remove(tmp) //nolint:forbidigo // the cache root (trickplay), never a library root (ADR 0032)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("trickplay sheet %d of %s: %w", n, r.p.MediaFileID, err)
	}
	if err := r.syncFile(tmp); err != nil {
		_ = r.root.Remove(tmp) //nolint:forbidigo // the cache root (trickplay), never a library root (ADR 0032)
		return err
	}
	return r.store(tmp, TrickplaySheetName(n))
}

// syncFile flushes the file rel below the cache root (a sheet ffmpeg wrote) to the drive.
func (r *trickplayRun) syncFile(rel string) error {
	f, err := r.root.Open(rel)
	if err != nil {
		return fmt.Errorf("open trickplay sheet: %w", err)
	}
	err = f.Sync()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("flush trickplay sheet: %w", err)
	}
	return nil
}

// recheck reads, after a sheet, the payload of the job (a start moves the position or takes the
// job over), the setting, the version of the file and the queue: stop is true when the job ends
// now (without error), err a RetryLater when it yields or the file changed.
func (r *trickplayRun) recheck(ctx context.Context) (stop bool, err error) {
	if j, err := r.s.queue.Get(ctx, r.j.ID); err == nil {
		var cur trickplayPayload
		if json.Unmarshal(j.Payload, &cur) == nil && cur.MediaFileID == r.p.MediaFileID {
			r.p.Origin, r.p.StartMs, r.p.Throttled = cur.Origin, cur.StartMs, cur.Throttled
		}
	}
	row, err := r.read.GetTrickplayFile(ctx, r.p.MediaFileID)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return true, fmt.Errorf("load media file: %w", err)
	}
	if (FileVersion{Size: row.Size, Mtime: row.Mtime}) != r.version {
		return true, jobs.RetryLater(chapterChangedRetry, "the media file changed")
	}
	playing, err := r.filePlaying(ctx)
	if err != nil {
		return true, err
	}
	setting := trickplayFileSetting(row)
	if !trickplayWanted(setting, r.p.Origin, playing) {
		r.s.log.InfoContext(ctx, "trickplay ends: the library setting changed", "media_file_id", r.p.MediaFileID, "origin", r.p.Origin, "mode", setting.Mode)
		return true, nil
	}
	if r.p.Origin != TrickplayOriginStart && r.p.Origin != TrickplayOriginDownload &&
		strings.HasPrefix(r.rel, TrickplayDir+"/") && !r.s.trickplaySpaceOK() {
		r.s.log.WarnContext(ctx, "cache drive below the free-space floor, trickplay stops", "media_file_id", r.p.MediaFileID, "origin", r.p.Origin)
		jobs.Progress(ctx, 0, 0, trickplayFloorStopNote)
		return true, nil
	}
	if r.p.Origin != TrickplayOriginStart {
		waiting, err := r.read.CountWaitingTrickplayStarts(ctx, systemq.CountWaitingTrickplayStartsParams{
			Priority: prioTrickplayStart, Now: r.s.now().UnixMilli(),
		})
		if err != nil {
			return true, fmt.Errorf("count waiting trickplay starts: %w", err)
		}
		if waiting > 0 {
			return true, jobs.RetryLater(0, "a trickplay start job waits")
		}
	}
	return false, nil
}
