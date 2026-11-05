package library

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/mediaindex"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Tests of concept 14, step 4b: the full keyframe read probe.keyframes in the pool index (6.3) and
// the playback request (6.4).

// videoFiles scans a tree of the movies files and makes every file a probed 90 s H.264 Matroska
// file without an index, without jobs; it returns the media file ids in the order of files.
func (e *env) videoFiles(files ...string) (dir string, ids []string) {
	e.t.Helper()
	dir = tree(e.t, files)
	e.scan(e.create(dir).ID)
	e.exec(`UPDATE media_files SET video_codec = 'h264', container = 'mkv', duration_ms = 90000, probed_at = 1`)
	e.exec(`DELETE FROM jobs`)
	for _, f := range files {
		ids = append(ids, e.str(`SELECT id FROM media_files WHERE rel_path = ?`, f))
	}
	return dir, ids
}

// scriptFFprobe writes an ffprobe that runs the shell commands body.
func scriptFFprobe(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffprobe")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// noContainerIndex makes the container read fail, so probe.keyframes goes on to the full read.
func noContainerIndex(io.ReaderAt, int64, string) (mediaindex.Index, error) {
	return mediaindex.Index{}, mediaindex.ErrNoIndex
}

func (e *env) fullRead(id string) error {
	e.t.Helper()
	return e.svc.handleKeyframes(e.ctx, mediaJob(e.t, JobProbeKeyframes, id))
}

// The result classes of the full read (6.3, Z-3): keyframes are stored as scan, a read without one
// and a broken file as none (done, no retry); a missing file ends done without an index (O-9: the
// 263 failures on CT 190 were ENOENT of a network share that dropped); everything else is an error
// for the backoff and leaves the source NULL.
func TestFullReadResultClasses(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := []string{"A (2001)/A (2001).mkv", "B (2002)/B (2002).mkv", "C (2003)/C (2003).mkv", "D (2004)/D (2004).mkv",
		"E (2005)/E (2005).mkv", "F (2006)/F (2006).mkv"}
	dir, ids := e.videoFiles(files...)
	e.svc.indexReader = noContainerIndex
	if err := os.Remove(filepath.Join(dir, files[5])); err != nil {
		t.Fatal(err)
	}
	for i, tc := range []struct {
		name, script, want string
		fails              bool
	}{
		{"keyframes", `echo '0.000000,K__'; echo '0.041667,___'; echo '2.000000,K__'`, "scan/blob jobs=0", false},
		{"no keyframe", `echo '0.000000,___'`, "none jobs=0", false},
		{"file error", `echo 'Invalid data found when processing input' >&2; exit 1`, "none jobs=0", false},
		{"no such file", `echo "$9: No such file or directory" >&2; exit 1`, "- jobs=0", false},
		{"environment", `echo 'Input/output error' >&2; exit 1`, "- jobs=0", true},
		{"file deleted", `echo 'Input/output error' >&2; exit 1`, "- jobs=0", false},
	} {
		e.svc.probe = transcode.Runner{FFprobePath: scriptFFprobe(t, tc.script)}
		err := e.fullRead(ids[i])
		if (err != nil) != tc.fails {
			t.Errorf("%s: err = %v, want failure %v", tc.name, err, tc.fails)
		}
		var later *jobs.RetryLaterError
		if errors.As(err, &later) {
			t.Errorf("%s: %v, want an error that uses an attempt", tc.name, err)
		}
		if v := e.indexState(ids[i]); v != tc.want {
			t.Errorf("%s: index = %q, want %q", tc.name, v, tc.want)
		}
	}
	if got := e.storedKeyframes(ids[0]); len(got) != 2 || got[1] != 2*time.Second {
		t.Errorf("keyframes of the full read = %v", got)
	}
}

// A file with an index, a file that cannot be remuxed and a missing file need no read: the job
// ends without starting ffprobe (a runner without ffprobe would fail).
func TestFullReadSkipsFilesWithoutWork(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, ids := e.videoFiles("A (2001)/A (2001).mkv", "B (2002)/B (2002).mkv", "C (2003)/C (2003).mkv")
	e.svc.indexReader = noContainerIndex
	e.svc.probe = transcode.Runner{}
	e.setIndex(ids[0])
	e.exec(`UPDATE media_files SET video_codec = 'vc1' WHERE id = ?`, ids[1])
	e.exec(`UPDATE media_files SET missing_since = 1 WHERE id = ?`, ids[2])
	for i, want := range []bool{true, true, false} {
		rec := &jobs.Recorder{}
		if err := e.svc.handleKeyframes(jobs.WithRecorder(e.ctx, rec), mediaJob(t, JobProbeKeyframes, ids[i])); err != nil {
			t.Errorf("file %d: %v", i, err)
		}
		if rec.Skipped != want {
			t.Errorf("file %d: skipped = %v, want %v", i, rec.Skipped, want)
		}
	}
	if v := e.indexState(ids[1]) + " " + e.indexState(ids[2]); v != "- jobs=0 - jobs=0" {
		t.Errorf("index of the unworthy and the missing file = %q", v)
	}
}

// probe.keyframes tries the container first (6.3 step 2): a job queued before probe.file read the
// container ends with its cues and no full read.
func TestFullReadTriesTheContainerFirst(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, ids := e.videoFiles("A (2001)/A (2001).mkv")
	want := []time.Duration{0, 2, 7, 8, 15, 21, 30, 31, 45, 60, 61, 75}
	for i := range want {
		want[i] *= time.Second
	}
	e.svc.indexReader = func(io.ReaderAt, int64, string) (mediaindex.Index, error) {
		return mediaindex.Index{Keyframes: want, Source: mediaindex.SourceCues}, nil
	}
	e.svc.keyframeVerifier = func(context.Context, string, []time.Duration, uint64) error { return nil }
	e.svc.probe = transcode.Runner{}
	rec := &jobs.Recorder{}
	if err := e.svc.handleKeyframes(jobs.WithRecorder(e.ctx, rec), mediaJob(t, JobProbeKeyframes, ids[0])); err != nil {
		t.Fatal(err)
	}
	if v := e.indexState(ids[0]); v != "cues/blob jobs=0" {
		t.Errorf("index = %q, want the cues", v)
	}
	if p, ok := rec.Last(); !ok || p.Note != "A (2001)/A (2001).mkv" {
		t.Errorf("progress = %+v, want the path relative to the root", p)
	}
}

// Review of wave B, m-2: the file changes during the full read. The probe.file of the new version
// found the running job under its key and queued nothing, so the job runs again on the new
// version (without using an attempt) instead of ending without an index.
func TestFullReadOfChangedFileRunsAgain(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, ids := e.videoFiles("A (2001)/A (2001).mkv")
	e.svc.indexReader = noContainerIndex
	marks := t.TempDir()
	entered, release := filepath.Join(marks, "entered"), filepath.Join(marks, "release")
	e.svc.probe = transcode.Runner{FFprobePath: scriptFFprobe(t, "touch '"+entered+"'\nwhile [ ! -f '"+release+"' ]; do sleep 0.01; done\n"+
		"echo '0.000000,K__'; echo '2.000000,K__'")}
	done := make(chan error, 1)
	go func() { done <- e.fullRead(ids[0]) }()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		if _, err := os.Stat(entered); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("full read not started")
		}
	}
	e.exec(`UPDATE media_files SET mtime = mtime + 1000 WHERE id = ?`, ids[0])
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := <-done
	var later *jobs.RetryLaterError
	if !errors.Is(err, errFileChanged) || !errors.As(err, &later) {
		t.Fatalf("full read of the old version = %v, want a retry on the new version", err)
	}
	if v := e.indexState(ids[0]); v != "- jobs=0" {
		t.Errorf("index of the old version = %q", v)
	}
	if err := e.fullRead(ids[0]); err != nil {
		t.Fatal(err)
	}
	if v := e.indexState(ids[0]); v != "scan/blob jobs=0" {
		t.Errorf("index of the new version = %q", v)
	}
}

// A missing library root ends the job without an index, like a missing file.
func TestFullReadOfMissingRootEndsDone(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir, ids := e.videoFiles("A (2001)/A (2001).mkv")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	e.svc.probe = transcode.Runner{}
	if err := e.fullRead(ids[0]); err != nil {
		t.Errorf("full read with the root gone = %v, want done", err)
	}
}

// probe.keyframes runs in the pool index; its one worker index-1 reports a final failure like the
// general workers (cmd/slouchcat TestIndexWorker).
func TestKeyframeTypesRegistered(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	r := jobs.NewRegistry(e.queue, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e.svc.RegisterTypes(r)
	kf, ok := r.Lookup(JobProbeKeyframes)
	if !ok || kf.Pool != jobs.PoolIndex || kf.Trigger != jobs.TriggerEvent || kf.Priority != prioKeyframes {
		t.Errorf("%s = %+v, want an event type in the pool index", JobProbeKeyframes, kf)
	}
}

// keyframeJob returns priority, state and override_pause of the active keyframe job of a file, and
// how many keyframe jobs of the file were cancelled.
func (e *env) keyframeJob(id string) string {
	e.t.Helper()
	return e.str(`SELECT coalesce((SELECT priority || ' ' || state || ' override=' || override_pause FROM jobs
	                               WHERE idempotency_key = 'keyframes:' || ? AND state IN ('queued', 'running')), '-')
	              || ' cancelled=' || (SELECT count(*) FROM jobs WHERE idempotency_key = 'keyframes:' || ? AND state = 'cancelled')`, id, id)
}

// The playback request (6.4): a new job at priority 9 through EnqueueOverridingPause (it carries
// override_pause, which nothing else sets); a waiting job of a scan is raised instead of cancelled
// (Z-4); nothing for a file whose index is impossible; nothing within 24 h after a failed read
// (O-17), again after that, and at once when a later run of the file is done (review of wave C,
// m-8: the newest finished run decides).
func TestQueueKeyframesForPlayback(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, ids := e.videoFiles("A (2001)/A (2001).mkv", "B (2002)/B (2002).mkv", "C (2003)/C (2003).mkv", "D (2004)/D (2004).mkv",
		"E (2005)/E (2005).mkv")
	queue := func(id string) jobs.PauseOverride {
		t.Helper()
		res, err := QueueKeyframesForPlayback(e.ctx, e.db.Read, e.queue, id, e.clock.Now())
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	if res := queue(ids[0]); !res.Inserted || !res.OverridesPause {
		t.Errorf("new request = %+v, want inserted with the pause override", res)
	}
	if v := e.keyframeJob(ids[0]); v != "9 queued override=1 cancelled=0" {
		t.Errorf("new request: %q", v)
	}
	first := e.count(`SELECT id FROM jobs WHERE idempotency_key = 'keyframes:' || ?`, ids[0])
	if res := queue(ids[0]); res.Inserted || int(res.ID) != first || e.jobCount(JobProbeKeyframes) != 1 {
		t.Errorf("second request = %+v, want the waiting job %d", res, first)
	}

	scan, err := e.queue.Enqueue(e.ctx, jobs.NewJob{Type: JobProbeKeyframes, Priority: prioKeyframes, IdempotencyKey: "keyframes:" + ids[1],
		Payload: []byte(`{"mediaFileId":"` + ids[1] + `"}`), Delay: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if res := queue(ids[1]); res.ID != scan || res.Inserted || !res.Raised || !res.OverridesPause {
		t.Errorf("request for a waiting scan job = %+v, want job %d raised", res, scan)
	}
	if v := e.keyframeJob(ids[1]); v != "9 queued override=1 cancelled=0" {
		t.Errorf("raised scan job: %q", v)
	}
	if v := e.str(`SELECT run_after <= ? FROM jobs WHERE id = ?`, e.clock.Now().UnixMilli(), scan); v != "1" {
		t.Errorf("raised scan job still delayed")
	}

	e.exec(`UPDATE media_files SET keyframes_source = 'none' WHERE id = ?`, ids[2])
	if res := queue(ids[2]); res != (jobs.PauseOverride{}) {
		t.Errorf("request for an impossible index = %+v, want nothing", res)
	}
	if v := e.keyframeJob(ids[2]); v != "- cancelled=0" {
		t.Errorf("impossible index: %q", v)
	}

	failed, err := e.queue.Enqueue(e.ctx, jobs.NewJob{Type: JobProbeKeyframes, Priority: prioKeyframes, IdempotencyKey: "keyframes:" + ids[3]})
	if err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE jobs SET state = 'failed', finished_at = ? WHERE id = ?`, e.clock.Now().UnixMilli(), failed)
	e.clock.Advance(23 * time.Hour)
	if res := queue(ids[3]); res != (jobs.PauseOverride{}) {
		t.Errorf("request 23 h after a failed read = %+v, want nothing", res)
	}
	e.clock.Advance(2 * time.Hour)
	if res := queue(ids[3]); !res.Inserted {
		t.Errorf("request 25 h after a failed read = %+v, want a new job", res)
	}

	for i, state := range []string{"failed", "done"} {
		run, err := e.queue.Enqueue(e.ctx, jobs.NewJob{Type: JobProbeKeyframes, Priority: prioKeyframes, IdempotencyKey: "keyframes:" + ids[4]})
		if err != nil {
			t.Fatal(err)
		}
		e.exec(`UPDATE jobs SET state = ?, finished_at = ? WHERE id = ?`, state, e.clock.Now().Add(time.Duration(i)*time.Hour).UnixMilli(), run)
	}
	e.clock.Advance(2 * time.Hour)
	if res := queue(ids[4]); !res.Inserted {
		t.Errorf("request after a failed and a later done read = %+v, want a new job", res)
	}
}

// O-7: the fourth playback request at a time waits for the end of the pause like any job.
func TestQueueKeyframesForPlaybackCap(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, ids := e.videoFiles("A (2001)/A (2001).mkv", "B (2002)/B (2002).mkv", "C (2003)/C (2003).mkv", "D (2004)/D (2004).mkv")
	for i, id := range ids {
		res, err := QueueKeyframesForPlayback(e.ctx, e.db.Read, e.queue, id, e.clock.Now())
		if err != nil {
			t.Fatal(err)
		}
		if want := i < jobs.MaxPauseOverrides; !res.Inserted || res.OverridesPause != want {
			t.Errorf("request %d = %+v, want inserted with override %v", i+1, res, want)
		}
	}
	if v := e.keyframeJob(ids[3]); v != "9 queued override=0 cancelled=0" {
		t.Errorf("fourth request: %q", v)
	}
}
