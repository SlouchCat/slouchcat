package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// filmPath returns the absolute path of the placeholder file of a film of the tpEnv.
func (e *tpEnv) filmPath(id string) string {
	e.t.Helper()
	return filepath.Join(e.dir, filepath.FromSlash(e.str(`SELECT rel_path FROM media_files WHERE id = ?`, id)))
}

// payload returns the payload of the active trickplay.generate job of a file.
func (e *tpEnv) payload(id string) trickplayPayload {
	e.t.Helper()
	var p trickplayPayload
	if err := json.Unmarshal([]byte(e.str(`SELECT payload FROM jobs WHERE idempotency_key = ? AND state IN ('queued', 'running')`, trickplayKey(id))), &p); err != nil {
		e.t.Fatal(err)
	}
	return p
}

// A job that ended without a complete set leaves the state failed, not queued (the player stops
// asking): a start of an unreadable file, a cancel, a setting that no longer wants the set.
func TestTrickplayStateAfterEndedJob(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	gone := e.film("Weg (2020)", 30_000)
	cancelled := e.film("Abgebrochen (2021)", 30_000)
	if err := os.Remove(e.filmPath(gone)); err != nil {
		t.Fatal(err)
	}
	e.start(gone, 0, trickplayAheadSetting.Movies)
	if _, _, state := e.status(gone); state != TrickplayQueued {
		t.Errorf("waiting: %q", state)
	}
	if _, err := e.run(); err != nil {
		t.Fatalf("unreadable start: %v", err)
	}
	if n := e.int(`SELECT count(*) FROM jobs WHERE idempotency_key = ? AND state = 'done'`, trickplayKey(gone)); n != 1 {
		t.Errorf("%d done jobs", n)
	}
	if _, _, state := e.status(gone); state != TrickplayFailed {
		t.Errorf("after a start of an unreadable file: %q, want failed", state)
	}
	// A cancelled job.
	e.start(cancelled, 0, trickplayAheadSetting.Movies)
	if _, err := e.queue.Cancel(e.ctx, e.int(`SELECT id FROM jobs WHERE idempotency_key = ?`, trickplayKey(cancelled))); err != nil {
		t.Fatal(err)
	}
	if _, _, state := e.status(cancelled); state != TrickplayFailed {
		t.Errorf("after a cancel: %q, want failed", state)
	}
	// A new start asks again: queued.
	e.start(cancelled, 0, trickplayAheadSetting.Movies)
	if _, _, state := e.status(cancelled); state != TrickplayQueued {
		t.Errorf("after a new start: %q, want queued", state)
	}
}

// Points that hang (each runs into its timeout, no frame failure) end the job after five in a row
// without grabbing the rest of the sheet (5.5), and as failed at its first attempt.
func TestTrickplayHangingPointsStopEarly(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Haenger (2020)", 1_000_000) // 100 points, one sheet
	var calls atomic.Int64
	e.svc.trickplayFrameFn = func(context.Context, string, time.Duration, int, transcode.Stream) ([]byte, error) {
		calls.Add(1)
		return nil, fmt.Errorf("ffmpeg: %w", context.DeadlineExceeded)
	}
	e.start(id, 0, trickplayAheadSetting.Movies)
	j, err := e.run()
	if !errors.Is(err, errTrickplayFailures) {
		t.Fatalf("hanging points: %v", err)
	}
	// The point of the fifth failure ends the sheet; each other worker may have one point running.
	if n, most := calls.Load(), int64(trickplayMaxFailures+trickplayProcs-1); n < trickplayMaxFailures || n > most {
		t.Errorf("%d points tried, want %d to %d", n, trickplayMaxFailures, most)
	}
	if got, _ := e.queue.Get(e.ctx, j.ID); got.State != jobs.StateFailed || got.Attempts != 1 {
		t.Errorf("job %s after %d attempts, want failed after 1", got.State, got.Attempts)
	}
	if _, _, state := e.status(id); state != TrickplayFailed {
		t.Errorf("state %q", state)
	}

	// The share goes away: the first error that is no frame failure finds the file unreadable and
	// ends the job at once.
	other := e.film("Freigabe (2021)", 1_000_000)
	path := e.filmPath(other)
	calls.Store(0)
	e.svc.trickplayFrameFn = func(context.Context, string, time.Duration, int, transcode.Stream) ([]byte, error) {
		calls.Add(1)
		_ = os.Remove(path)
		return nil, errors.New("Input/output error")
	}
	e.start(other, 0, trickplayAheadSetting.Movies)
	if _, err := e.run(); err == nil || !strings.Contains(err.Error(), "unreadable during trickplay") {
		t.Fatalf("share gone: %v", err)
	}
	if n := calls.Load(); n > trickplayProcs {
		t.Errorf("%d points tried after the share went", n)
	}
}

// A start takes over an import job that waits an hour for an unreadable file: it runs now, as a
// start, with the week of the import cleared, never with the pause override.
func TestTrickplayStartPullsWaitingJobForward(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Spaeter (2020)", 30_000)
	path := e.filmPath(id)
	if err := os.Rename(path, path+".off"); err != nil {
		t.Fatal(err)
	}
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: id, Origin: TrickplayOriginImport}, prioTrickplayImport, false); err != nil {
		t.Fatal(err)
	}
	_, err := e.run()
	var later *jobs.RetryLaterError
	if !errors.As(err, &later) || later.After != chapterUnreadableRetry {
		t.Fatalf("unreadable import: %v", err)
	}
	if p := e.payload(id); p.UnreadableSince == 0 {
		t.Fatalf("payload %+v", p)
	}
	if err := os.Rename(path+".off", path); err != nil {
		t.Fatal(err)
	}
	e.start(id, 0, trickplayAheadSetting.Movies)
	now := e.clock.Now().UnixMilli()
	if at := e.int(`SELECT run_after FROM jobs WHERE idempotency_key = ? AND state = 'queued'`, trickplayKey(id)); at > now {
		t.Errorf("run_after %d, %d ms after now", at, at-now)
	}
	if p := e.payload(id); p.Origin != TrickplayOriginStart || p.UnreadableSince != 0 {
		t.Errorf("taken over: %+v", p)
	}
	if n := e.int(`SELECT count(*) FROM jobs WHERE override_pause = 1`); n != 0 {
		t.Errorf("%d pause overrides", n)
	}
	if _, err := e.run(); err != nil || !slices.Contains(e.setFiles(TrickplayDir, id), "complete") {
		t.Errorf("start after the takeover: %v, %v", e.setFiles(TrickplayDir, id), err)
	}
}

// An unreadable file at the start of a job (5.5): a start ends without retry; an import waits an
// hour, notes since when, and gives up a week later.
func TestTrickplayUnreadableFile(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Offline (2020)", 30_000)
	if err := os.Remove(e.filmPath(id)); err != nil {
		t.Fatal(err)
	}
	e.start(id, 0, trickplayAheadSetting.Movies)
	if _, err := e.run(); err != nil || e.jobCount(JobTrickplay) != 1 {
		t.Fatalf("unreadable start: %v, %d jobs", err, e.jobCount(JobTrickplay))
	}
	if n := e.int(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued'`, JobTrickplay); n != 0 {
		t.Errorf("a start of an unreadable file waits: %d", n)
	}
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: id, Origin: TrickplayOriginImport}, prioTrickplayImport, false); err != nil {
		t.Fatal(err)
	}
	var later *jobs.RetryLaterError
	if _, err := e.run(); !errors.As(err, &later) || later.After != time.Hour {
		t.Fatalf("unreadable import: %v, want RetryLater(1h)", err)
	}
	if got, want := e.payload(id).UnreadableSince, e.clock.Now().UnixMilli(); got != want {
		t.Errorf("unreadableSince %d, want %d", got, want)
	}
	e.clock.Advance(6 * 24 * time.Hour)
	if _, err := e.run(); !errors.As(err, &later) {
		t.Fatalf("six days unreadable: %v, want RetryLater", err)
	}
	e.clock.Advance(2 * 24 * time.Hour)
	if _, err := e.run(); err != nil {
		t.Fatalf("eight days unreadable: %v, want the end", err)
	}
	if n := e.int(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued'`, JobTrickplay); n != 0 || e.setFiles(TrickplayDir, id) != nil {
		t.Errorf("after giving up: %d waiting, files %v", n, e.setFiles(TrickplayDir, id))
	}
}

// 6.6: a complete temporary set of a setting that stores now is renamed at the next start, without
// a job, so the maintenance keeps it.
func TestTrickplayStartKeepsTemporarySet(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Behalten (2020)", 30_000)
	e.setting(model.TrickplayOnStart, false)
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: id, Origin: TrickplayOriginDownload}, prioTrickplayDownload, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.run(); err != nil || !slices.Contains(e.setFiles(TrickplaySessionDir, id), "complete") {
		t.Fatalf("temporary set: %v, %v", e.setFiles(TrickplaySessionDir, id), err)
	}
	e.setting(model.TrickplayOnStart, true)
	e.start(id, 0, model.TrickplaySetting{Mode: model.TrickplayOnStart, Keep: true})
	if got := e.setFiles(TrickplayDir, id); !slices.Contains(got, "complete") || e.setFiles(TrickplaySessionDir, id) != nil {
		t.Errorf("after the start: stored %v, temporary %v", got, e.setFiles(TrickplaySessionDir, id))
	}
	if n := e.int(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued'`, JobTrickplay); n != 0 {
		t.Errorf("the start queued %d jobs", n)
	}
	if err := e.svc.sweepTrickplay(e.ctx); err != nil {
		t.Fatal(err)
	}
	if !TrickplaySetComplete(e.cache, id, e.version(id)) {
		t.Error("the maintenance removed the kept set")
	}
}

type fakeLoad struct{ heavy, starting atomic.Bool }

func (f *fakeLoad) CPUHeavy() bool    { return f.heavy.Load() }
func (f *fakeLoad) CPUStarting() bool { return f.starting.Load() }

// The ffmpeg processes of a sheet (5.3): two; one while a transcode keeps the CPU busy, and for a
// throttled look-ahead while any playback is open.
func TestTrickplayProcs(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	load := &fakeLoad{}
	e.svc.trickplayLoad = load
	run := func(p trickplayPayload) int {
		r := &trickplayRun{s: e.svc, p: p, read: systemq.New(e.db.Read)}
		return r.procs(e.ctx)
	}
	start := trickplayPayload{Origin: TrickplayOriginStart}
	throttled := trickplayPayload{Origin: TrickplayOriginSeries, Throttled: true}
	if a, b := run(start), run(throttled); a != 2 || b != 2 {
		t.Errorf("idle: %d %d, want 2 2", a, b)
	}
	load.heavy.Store(true)
	if a, b := run(start), run(throttled); a != 1 || b != 1 {
		t.Errorf("CPU busy: %d %d, want 1 1", a, b)
	}
	load.heavy.Store(false)
	e.profiles(1)
	item := e.film("Laeuft (2020)", 30_000)
	now := e.clock.Now().UnixMilli()
	e.exec(`INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode, started_at, last_activity_at, expires_at)
	        VALUES ('01JPLAYBACKSESSION00000001', 1, 1, 1, (SELECT item_id FROM media_files WHERE id = ?), ?, 'direct', ?, ?, ?)`, item, item, now, now, now+3_600_000)
	if a, b, c := run(start), run(throttled), run(trickplayPayload{Origin: TrickplayOriginSeries}); a != 2 || b != 1 || c != 2 {
		t.Errorf("a playback open: start %d, throttled %d, next episodes %d; want 2 1 2", a, b, c)
	}
}

// W-3: a sheet waits while a transcode on the CPU has not written the first segment of a start or
// a jump, and goes on as soon as that segment is out; a stopped job stops waiting.
func TestTrickplayYieldsToAStartingTranscode(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	load := &fakeLoad{}
	e.svc.trickplayLoad = load
	r := &trickplayRun{s: e.svc, p: trickplayPayload{Origin: TrickplayOriginStart}}
	begin := time.Now()
	if err := r.yieldToStart(e.ctx); err != nil || time.Since(begin) > trickplayStartPoll {
		t.Fatalf("no starting transcode: %v after %v", err, time.Since(begin))
	}
	load.starting.Store(true)
	const segment = 400 * time.Millisecond
	time.AfterFunc(segment, func() { load.starting.Store(false) })
	begin = time.Now()
	if err := r.yieldToStart(e.ctx); err != nil {
		t.Fatal(err)
	}
	if waited := time.Since(begin); waited < segment || waited > segment+5*trickplayStartPoll {
		t.Errorf("waited %v for a first segment out after %v", waited, segment)
	}
	load.starting.Store(true)
	ctx, cancel := context.WithTimeout(e.ctx, segment)
	defer cancel()
	if err := r.yieldToStart(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("stopped job: %v, want the context's error", err)
	}
}

// A stored probe without a sample aspect ratio (before contract 1.23.0) is completed from the file
// once: an anamorphic PAL file gets 4:3 tiles. A set whose tile height no longer fits the stored
// probe is made again under a new version.
func TestTrickplaySampleAspect(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Anamorph (2020)", 30_000)
	const pal = `{"format":{"duration":30000000000},"streams":[{"index":0,"type":"video","codec":"mpeg2video","width":720,"height":576}]}`
	e.exec(`UPDATE media_files SET probe = ? WHERE id = ?`, pal, id)
	probes := 0
	e.svc.trickplayProbeFn = func(context.Context, string) (transcode.Probe, error) {
		probes++
		return transcode.Probe{Streams: []transcode.Stream{{Index: 0, Type: "video", Width: 720, Height: 576, SAR: "16:15"}}}, nil
	}
	e.start(id, 0, trickplayAheadSetting.Movies)
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	m, ok := ReadTrickplayManifest(e.cache, id, e.version(id))
	if !ok || m.TileHeight != 240 || probes != 1 {
		t.Fatalf("manifest %+v (%v), %d probes", m, ok, probes)
	}
	// The stored probe learns another ratio (a new probe): the set is made again.
	e.exec(`UPDATE media_files SET probe = ? WHERE id = ?`, strings.Replace(pal, `"height":576`, `"height":576,"sar":"64:45"`, 1), id)
	e.start(id, 0, trickplayAheadSetting.Movies)
	if n := e.jobCount(JobTrickplay); n != 1 {
		t.Fatalf("a complete set queued %d jobs", n)
	}
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: id, Origin: TrickplayOriginStart}, prioTrickplayStart, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	again, ok := ReadTrickplayManifest(e.cache, id, e.version(id))
	if !ok || again.TileHeight != 180 || again.Version == m.Version || probes != 1 {
		t.Errorf("made again: %+v (%v), %d probes", again, ok, probes)
	}
}

// Below the free-space floor a stopping job says so on «Hintergrundjobs» (its progress note, 6.3).
func TestTrickplayFloorNote(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Voll (2020)", 30_000)
	e.svc.diskSpace = func(string) (int64, int64, error) { return 100 << 30, 5 << 30, nil }
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: id, Origin: TrickplayOriginImport}, prioTrickplayImport, false); err != nil {
		t.Fatal(err)
	}
	j, err := e.queue.Claim(e.ctx, "trickplay-1", []string{JobTrickplay})
	if err != nil || j == nil {
		t.Fatal(err)
	}
	rec := &jobs.Recorder{}
	if err := e.svc.handleTrickplay(jobs.WithRecorder(e.ctx, rec), j); err != nil {
		t.Fatal(err)
	}
	if err := e.queue.Complete(e.ctx, "trickplay-1", j.ID); err != nil {
		t.Fatal(err)
	}
	if p, ok := rec.Last(); !ok || p.Note != trickplayFloorStopNote {
		t.Errorf("import below the floor: %+v %v", p, ok)
	}
	rec = &jobs.Recorder{}
	if err := e.svc.handleTrickplayBackfill(jobs.WithRecorder(e.ctx, rec), &jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	if p, ok := rec.Last(); !ok || p.Note != trickplayFloorStopNote {
		t.Errorf("backfill below the floor: %+v %v", p, ok)
	}
	// A start writes a temporary set and says so.
	rec = &jobs.Recorder{}
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: id, Origin: TrickplayOriginStart}, prioTrickplayStart, true); err != nil {
		t.Fatal(err)
	}
	if j, err = e.queue.Claim(e.ctx, "trickplay-1", []string{JobTrickplay}); err != nil || j == nil {
		t.Fatal(err)
	}
	e.profiles(1)
	now := e.clock.Now().UnixMilli()
	e.exec(`INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode, started_at, last_activity_at, expires_at)
	        VALUES ('01JPLAYBACKSESSION00000001', 1, 1, 1, (SELECT item_id FROM media_files WHERE id = ?), ?, 'direct', ?, ?, ?)`, id, id, now, now, now+3_600_000)
	if err := e.svc.handleTrickplay(jobs.WithRecorder(e.ctx, rec), j); err != nil {
		t.Fatal(err)
	}
	if p, ok := rec.Last(); !ok || p.Note != trickplayFloorTempNote || !slices.Contains(e.setFiles(TrickplaySessionDir, id), "complete") {
		t.Errorf("start below the floor: %+v %v, %v", p, ok, e.setFiles(TrickplaySessionDir, id))
	}
}

// A start of another episode while the look-ahead runs is not lost: the running job takes the
// payload and runs again for that episode.
func TestTrickplaySeriesRunningTakesNewEpisode(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	dir := t.TempDir()
	show := "Serie (2020)"
	for _, rel := range []string{"Season 01/Serie - S01E01.mkv", "Season 01/Serie - S01E02.mkv", "Season 01/Serie - S01E03.mkv",
		"Season 01/Serie - S01E04.mkv"} {
		writeFile(t, dir, show+"/"+rel, rel)
	}
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	e.exec(`UPDATE media_files SET probe = ?, probed_at = 1, duration_ms = 60000`, trickplayProbe)
	e.exec(`UPDATE libraries SET trickplay_series = 'series_start', trickplay_series_scope = 'next_episode' WHERE id = ?`, lib.ID)
	series := e.str(`SELECT id FROM items WHERE kind = 'series'`)
	ep := func(n int) (item, file string) {
		item = e.str(`SELECT ep.id FROM items AS ep WHERE ep.kind = 'episode' AND ep.episode_number = ?`, n)
		return item, e.str(`SELECT id FROM media_files WHERE item_id = ?`, item)
	}
	e1, f1 := ep(1)
	e3, f3 := ep(3)
	_, f2 := ep(2)
	_, f4 := ep(4)
	setting := model.TrickplaySetting{Mode: model.TrickplaySeriesStart, Scope: model.TrickplayScopeNextEpisode}
	startEp := func(item, file string) {
		t.Helper()
		if err := QueueTrickplayForPlayback(e.ctx, e.queue, e.cache, TrickplayStart{MediaFileID: file, Version: e.version(file), Kind: "episode",
			Setting: setting, SeriesID: series, EpisodeID: item}); err != nil {
			t.Fatal(err)
		}
	}
	startEp(e1, f1)
	j, err := e.queue.Claim(e.ctx, "general-1", []string{JobTrickplaySeries})
	if err != nil || j == nil {
		t.Fatalf("claim look-ahead: %v %v", j, err)
	}
	startEp(e3, f3) // while the look-ahead of episode 1 runs
	if payload := e.str(`SELECT payload FROM jobs WHERE id = ?`, j.ID); !strings.Contains(payload, e3) {
		t.Fatalf("running look-ahead payload %s", payload)
	}
	if err := e.svc.handleTrickplaySeries(e.ctx, j); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{f2, f4} {
		if n := e.int(`SELECT count(*) FROM jobs WHERE idempotency_key = ? AND json_extract(payload, '$.origin') = 'series'`, trickplayKey(f)); n != 1 {
			t.Errorf("look-ahead of %s: %d jobs", f, n)
		}
	}
}

// The maintenance treats a waiting job as a lock too: the folders of its file stay until it ran.
func TestTrickplaySweepWaitingJobLocks(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Warten (2020)", 30_000)
	e.start(id, 0, trickplayAheadSetting.Movies)
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	set := filepath.Join(e.cache, TrickplayDir, TrickplaySetName(id, e.version(id)))
	e.setting(model.TrickplayOff, true)
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: id, Origin: TrickplayOriginImport}, prioTrickplayImport, false); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.sweepTrickplay(e.ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(set); err != nil {
		t.Fatalf("the set of a file with a waiting job went: %v", err)
	}
	e.exec(`DELETE FROM jobs WHERE state = 'queued'`)
	if err := e.svc.sweepTrickplay(e.ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(set); err == nil {
		t.Error("the set stays after the job")
	}
}

// A stop between removing a set made again and renaming its successor: the next run restores it.
func TestTrickplayRestoresSetMadeAgain(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Neu (2020)", 30_000)
	e.start(id, 0, trickplayAheadSetting.Movies)
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	set := filepath.Join(e.cache, TrickplayDir, TrickplaySetName(id, e.version(id)))
	if err := os.Rename(set, set+trickplayNextSuffix); err != nil {
		t.Fatal(err)
	}
	e.sheets = nil
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: id, Origin: TrickplayOriginStart}, prioTrickplayStart, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	if !TrickplaySetComplete(e.cache, id, e.version(id)) || len(e.sheets) != 0 {
		t.Errorf("restored %v, sheets made %v", TrickplaySetComplete(e.cache, id, e.version(id)), e.sheets)
	}
	if _, err := os.Stat(set + trickplayNextSuffix); err == nil {
		t.Error("the folder of the set made again stays")
	}
}
