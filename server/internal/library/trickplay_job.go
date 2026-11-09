package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
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
//     yields to a waiting start job after each sheet. Every job runs one attempt: a failure ends the
//     set as failed, the sheets stay, the next trigger tries again.
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
// position. A job stays throttled only while every trigger was a throttled look-ahead.
func (p trickplayPayload) merge(next trickplayPayload) trickplayPayload {
	out := p
	if out.Origin == "" || trickplayRank[next.Origin] > trickplayRank[out.Origin] ||
		(next.Origin == TrickplayOriginStart && out.Origin == TrickplayOriginStart) {
		out.Origin, out.StartMs = next.Origin, next.StartMs
	}
	out.Throttled = out.Origin == TrickplayOriginSeries && p.Throttled && next.Throttled
	return out
}

// trickplaySeriesPayload is the payload of trickplay.series.
type trickplaySeriesPayload struct {
	SeriesID  string `json:"seriesId"`
	EpisodeID string `json:"episodeId"`
}

// TrickplayLoad tells whether a transcode keeps the CPU busy (transcode.Manager.CPUHeavy).
type TrickplayLoad interface{ CPUHeavy() bool }

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

// enqueueTrickplay queues trickplay.generate for a file with the payload p at prio, one attempt.
// overrideWindow lets it start outside the time window of the type (start, look-ahead, download).
// A job that holds the key already takes p over by trickplayPayload.merge: a waiting one through
// SetQueuedPayload, a running one through SetRunningPayload (it reads the payload after every
// sheet); a waiting one is raised to prio. It returns the id of the job.
func enqueueTrickplay(ctx context.Context, q *jobs.Queue, p trickplayPayload, prio int, overrideWindow bool) (int64, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return 0, fmt.Errorf("encode %s payload: %w", JobTrickplay, err)
	}
	nj := jobs.NewJob{Type: JobTrickplay, Priority: prio, IdempotencyKey: trickplayKey(p.MediaFileID), Payload: raw, MaxAttempts: 1}
	var id int64
	if overrideWindow {
		id, err = q.EnqueueOverridingWindow(ctx, nj)
	} else if id, err = q.Enqueue(ctx, nj); err == nil {
		_, err = q.RaiseQueuedPriority(ctx, id, prio)
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
// set; a job that holds the key takes the start position and the origin over. For an episode at
// series_start it queues the look-ahead trickplay.series of its series as well; a waiting one
// takes the new episode.
func QueueTrickplayForPlayback(ctx context.Context, q *jobs.Queue, cacheDir string, st TrickplayStart) error {
	if cacheDir == "" || q == nil || !chapterKind(st.Kind) || st.Setting.Mode == "" || st.Setting.Mode == model.TrickplayOff {
		return nil
	}
	if !TrickplaySetComplete(cacheDir, st.MediaFileID, st.Version) {
		if _, err := enqueueTrickplay(ctx, q, trickplayPayload{MediaFileID: st.MediaFileID, StartMs: max(st.StartMs, 0), Origin: TrickplayOriginStart},
			prioTrickplayStart, true); err != nil {
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
	// The newest episode wins while the look-ahead waits.
	_, err = q.SetQueuedPayload(ctx, id, raw)
	return err
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
func (s *Service) handleTrickplay(ctx context.Context, j *jobs.Job) error {
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
			return nil
		}
		s.log.WarnContext(ctx, "cache drive below the free-space floor, temporary trickplay set", "media_file_id", row.ID)
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
	if _, err := r.root.Stat(r.rel + "/" + trickplayCompleteName); err == nil {
		return nil
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
	if err := r.manifest(row); err != nil {
		return err
	}
	return r.sheets(ctx)
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

// manifest reads the manifest of the set, or computes the grid and writes it first (5.1).
func (r *trickplayRun) manifest(row systemq.GetTrickplayFileRow) error {
	version := TrickplayVersion(r.version)
	if m, err := readTrickplayManifest(r.cache, r.rel); err == nil && m.Version == version {
		r.m = m
		return nil
	}
	var kf []time.Duration
	switch row.KeyframesSource.String {
	case KeyframesCues, KeyframesMP4, KeyframesScan:
		if decoded, err := DecodeKeyframes(row.Keyframes); err == nil {
			kf = decoded
		}
	}
	r.m = TrickplayGrid(row.DurationMs.Int64, kf, r.video, version)
	raw, err := json.Marshal(r.m)
	if err != nil {
		return fmt.Errorf("encode trickplay manifest: %w", err)
	}
	return r.writeFile(trickplayManifestName, raw)
}

// writeFile writes name into the set under a temporary name and renames it.
func (r *trickplayRun) writeFile(name string, data []byte) error {
	tmp := r.rel + "/" + trickplayTempPrefix + name + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := r.root.WriteFile(tmp, data, 0o640); err != nil {
		return fmt.Errorf("write trickplay %s: %w", name, err)
	}
	if err := r.root.Rename(tmp, r.rel+"/"+name); err != nil {
		_ = r.root.Remove(tmp)
		return fmt.Errorf("store trickplay %s: %w", name, err)
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
	failures := 0
	for len(done) < total {
		jobs.Progress(ctx, int64(len(done)), int64(total), "")
		n := -1
		for _, k := range TrickplaySheetOrder(r.m, r.p.StartMs) {
			if !done[k] {
				n = k
				break
			}
		}
		if err := r.sheet(ctx, n, &failures); err != nil {
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
	jobs.Progress(ctx, int64(total), int64(total), "")
	if err := r.writeFile(trickplayCompleteName, nil); err != nil {
		return err
	}
	r.s.log.InfoContext(ctx, "trickplay set written", "media_file_id", r.p.MediaFileID, "sheets", total, "points", len(r.m.PointsMs),
		"source", r.m.Source, "stored", strings.HasPrefix(r.rel, TrickplayDir+"/"))
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
// black and logged; failures counts broken points in a row across sheets, trickplayMaxFailures end
// the job. An error that is no frame failure (the share went away) ends the job when the file
// cannot be read any more, else it counts like a broken point.
func (r *trickplayRun) sheet(ctx context.Context, n int, failures *int) error {
	from, to := r.m.SheetPoints(n)
	tiles := make([][]byte, to-from)
	errs := make([]error, to-from)
	var wg sync.WaitGroup
	next := make(chan int)
	for range min(r.procs(ctx), to-from) {
		wg.Go(func() {
			for k := range next {
				if k > from && r.m.SeekMs[k] == r.m.SeekMs[k-1] {
					continue // the point repeats the one before it; filled below
				}
				pctx, cancel := context.WithTimeout(ctx, trickplayPointTimeout)
				tiles[k-from], errs[k-from] = r.s.trickplayFrame(pctx, r.in, time.Duration(r.m.SeekMs[k])*time.Millisecond, r.m.TileHeight, r.video)
				cancel()
			}
		})
	}
	for k := from; k < to; k++ {
		next <- k
	}
	close(next)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	black := transcode.BlackTrickplayTile(r.m.TileHeight)
	for i := range tiles {
		k := from + i
		if k > from && r.m.SeekMs[k] == r.m.SeekMs[k-1] {
			tiles[i] = tiles[i-1]
			continue
		}
		if errs[i] == nil {
			*failures = 0
			continue
		}
		if !transcode.FrameFailed(errs[i]) {
			if rerr := r.s.readable(r.rootDir, r.in, chapterReadTimeout); rerr != nil {
				return fmt.Errorf("media file of %s unreadable during trickplay: %w", r.p.MediaFileID, rerr)
			}
		}
		*failures++
		r.s.log.WarnContext(ctx, "trickplay point failed", "media_file_id", r.p.MediaFileID, "point", k, "err", errs[i])
		if *failures >= trickplayMaxFailures {
			return fmt.Errorf("%w: %s at point %d: %w", errTrickplayFailures, r.p.MediaFileID, k, errs[i])
		}
		tiles[i] = black
	}
	tmp := r.rel + "/" + trickplayTempPrefix + strconv.Itoa(n) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".webp"
	out, err := paths.Resolve(r.cache, tmp)
	if err != nil {
		return err
	}
	if err := r.s.trickplaySheet(ctx, tiles, r.m.TileHeight, out); err != nil {
		_ = r.root.Remove(tmp)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("trickplay sheet %d of %s: %w", n, r.p.MediaFileID, err)
	}
	if err := r.root.Rename(tmp, r.rel+"/"+TrickplaySheetName(n)); err != nil {
		_ = r.root.Remove(tmp)
		return fmt.Errorf("store trickplay sheet: %w", err)
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
