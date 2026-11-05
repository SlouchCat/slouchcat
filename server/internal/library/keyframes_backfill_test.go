package library

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/mediaindex"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Tests of the keyframe backfill (concept 14, 6.6).

// fileName is the folder and the name of the file an indexReader reads: the rel_path of the
// files of videoFiles.
func fileName(r io.ReaderAt) string {
	f, ok := r.(interface{ Name() string })
	if !ok {
		return ""
	}
	return path.Join(path.Base(path.Dir(f.Name())), path.Base(f.Name()))
}

// backfillCues returns an indexReader that hands every file the cues of the long test MKV, except
// the files named in without (files without cues); reads counts the calls by path.
func backfillCues(without ...string) (read func(io.ReaderAt, int64, string) (mediaindex.Index, error), reads func() map[string]int) {
	cues := []time.Duration{0, 2, 7, 8, 15, 21, 30, 31, 45, 60, 61, 75}
	for i := range cues {
		cues[i] *= time.Second
	}
	var mu sync.Mutex
	calls := map[string]int{}
	read = func(r io.ReaderAt, _ int64, _ string) (mediaindex.Index, error) {
		mu.Lock()
		calls[fileName(r)]++
		mu.Unlock()
		for _, w := range without {
			if fileName(r) == w {
				return mediaindex.Index{}, mediaindex.ErrNoIndex
			}
		}
		return mediaindex.Index{Keyframes: cues, Source: mediaindex.SourceCues}, nil
	}
	reads = func() map[string]int {
		mu.Lock()
		defer mu.Unlock()
		out := map[string]int{}
		for k, v := range calls {
			out[k] = v
		}
		return out
	}
	return read, reads
}

func (e *env) rel(id string) string {
	e.t.Helper()
	return e.str(`SELECT rel_path FROM media_files WHERE id = ?`, id)
}

func (e *env) backfill(ctx context.Context) error {
	e.t.Helper()
	return e.svc.handleKeyframesBackfill(ctx, &jobs.Job{Type: JobKeyframesBackfill})
}

// The backfill stores the cues where they pass the checks and queues the full read only for the
// rest, at priority 1; files with an index, files that cannot be remuxed, missing files and the
// files of audiobooks and podcast episodes are not read. The progress ends at done = total.
func TestKeyframesBackfillStoresCuesAndQueuesOnlyTheRest(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, ids := e.videoFiles("A (2001)/A (2001).mkv", "B (2002)/B (2002).mkv", "C (2003)/C (2003).mkv",
		"D (2004)/D (2004).mkv", "E (2005)/E (2005).mkv", "F (2006)/F (2006).mkv", "G (2007)/G (2007).mkv",
		"H (2008)/H (2008).mkv")
	e.setIndex(ids[3])
	e.exec(`UPDATE media_files SET video_codec = 'vc1' WHERE id = ?`, ids[4])
	e.exec(`UPDATE media_files SET missing_since = 1 WHERE id = ?`, ids[5])
	e.exec(`UPDATE items SET kind = 'audiobook' WHERE id = (SELECT item_id FROM media_files WHERE id = ?)`, ids[6])
	e.exec(`UPDATE items SET kind = 'podcast_episode' WHERE id = (SELECT item_id FROM media_files WHERE id = ?)`, ids[7])
	read, reads := backfillCues(e.rel(ids[2]))
	e.svc.indexReader = read
	e.svc.keyframeVerifier = func(context.Context, string, []time.Duration, uint64) error { return nil }
	rec := &jobs.Recorder{}
	if err := e.backfill(jobs.WithRecorder(e.ctx, rec)); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"cues/blob jobs=0", "cues/blob jobs=0", "- jobs=1", "cues/blob jobs=0", "- jobs=0", "- jobs=0",
		"- jobs=0", "- jobs=0"} {
		if v := e.indexState(ids[i]); v != want {
			t.Errorf("file %d: index = %q, want %q", i, v, want)
		}
	}
	if v := e.str(`SELECT priority || ' ' || state FROM jobs WHERE type = ?`, JobProbeKeyframes); v != "1 queued" {
		t.Errorf("full read = %q, want priority 1, queued", v)
	}
	got := reads()
	for i, want := range []int{1, 1, 1, 0, 0, 0, 0, 0} {
		if got[e.rel(ids[i])] != want {
			t.Errorf("file %d read %d times, want %d", i, got[e.rel(ids[i])], want)
		}
	}
	if p, ok := rec.Last(); !ok || p.Done != 3 || p.Total != 3 || rec.Skipped {
		t.Errorf("progress = %+v (skipped %v), want 3 of 3", p, rec.Skipped)
	}
	if len(rec.Reports) != 4 || !slices.Contains([]string{e.rel(ids[0]), e.rel(ids[1]), e.rel(ids[2])}, rec.Reports[0].Note) {
		t.Errorf("reports = %+v, want one per file relative to its root and the end", rec.Reports)
	}

	// A second run finds nothing to do: the file queued for the full read is not queued twice.
	rec = &jobs.Recorder{}
	if err := e.backfill(jobs.WithRecorder(e.ctx, rec)); err != nil {
		t.Fatal(err)
	}
	if n := e.jobCount(JobProbeKeyframes); n != 1 {
		t.Errorf("full reads after the second run = %d, want 1", n)
	}

}

// A paused or cancelled run ends through its context; the next run starts over and reads only the
// files still without an index.
func TestKeyframesBackfillAfterPauseSkipsWhatIsDone(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, ids := e.videoFiles("A (2001)/A (2001).mkv", "B (2002)/B (2002).mkv", "C (2003)/C (2003).mkv", "D (2004)/D (2004).mkv")
	slices.Sort(ids) // the order of the cursor
	e.svc.keyframeVerifier = func(context.Context, string, []time.Duration, uint64) error { return nil }
	read, reads := backfillCues()
	ctx, cancel := context.WithCancel(e.ctx)
	calls := 0
	e.svc.indexReader = func(r io.ReaderAt, size int64, container string) (mediaindex.Index, error) {
		if calls++; calls == 2 {
			cancel() // the pause arrives while the second file is read
		}
		return read(r, size, container)
	}
	if err := e.backfill(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("paused run = %v, want context.Canceled", err)
	}
	if v := e.indexState(ids[0]) + ", " + e.indexState(ids[1]); v != "cues/blob jobs=0, - jobs=0" {
		t.Errorf("after the pause: %q, want the first file done only", v)
	}
	e.svc.indexReader = read
	if err := e.backfill(e.ctx); err != nil {
		t.Fatal(err)
	}
	got := reads()
	for i, want := range []int{1, 2, 1, 1} {
		if got[e.rel(ids[i])] != want {
			t.Errorf("file %d read %d times, want %d", i, got[e.rel(ids[i])], want)
		}
		if v := e.indexState(ids[i]); v != "cues/blob jobs=0" {
			t.Errorf("file %d: index = %q", i, v)
		}
	}
}

// Three container reads in a row that time out (a hanging network share, 6.1) stop the run with an
// error for the backoff; the files behind stay untouched. A file whose read timed out gets no full
// read: it would hang on the same share and hold the one worker of the pool index (review of wave
// C, m-3); the next run tries it again.
func TestKeyframesBackfillStopsAfterThreeTimeouts(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	_, ids := e.videoFiles("A (2001)/A (2001).mkv", "B (2002)/B (2002).mkv", "C (2003)/C (2003).mkv",
		"D (2004)/D (2004).mkv", "E (2005)/E (2005).mkv", "F (2006)/F (2006).mkv")
	slices.Sort(ids) // the order of the cursor
	e.svc.keyframeVerifier = func(context.Context, string, []time.Duration, uint64) error { return nil }
	cues, _ := backfillCues()
	// The second file reads in time, which breaks the first series; files 3 to 5 time out.
	inTime := e.rel(ids[1])
	e.svc.indexReader = func(r io.ReaderAt, size int64, container string) (mediaindex.Index, error) {
		if fileName(r) == inTime {
			return cues(r, size, container)
		}
		return mediaindex.Index{}, errIndexReadTimeout
	}
	err := e.backfill(e.ctx)
	if !errors.Is(err, errBackfillTimeouts) {
		t.Fatalf("run = %v, want errBackfillTimeouts", err)
	}
	for i, want := range []string{"- jobs=0", "cues/blob jobs=0", "- jobs=0", "- jobs=0", "- jobs=0", "- jobs=0"} {
		if v := e.indexState(ids[i]); v != want {
			t.Errorf("file %d: index = %q, want %q", i, v, want)
		}
	}
}

// Review of wave C, m-5: a file of a later batch that probe.file indexes while the backfill runs
// leaves the count; the progress still ends at done of done.
func TestKeyframesBackfillProgressEndsComplete(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := make([]string, keyframesBackfillBatch+1)
	for i := range files {
		files[i] = fmt.Sprintf("Film %03d (2001)/Film %03d (2001).mkv", i, i)
	}
	_, ids := e.videoFiles(files...)
	slices.Sort(ids) // the order of the cursor
	last := ids[len(ids)-1]
	e.svc.keyframeVerifier = func(context.Context, string, []time.Duration, uint64) error { return nil }
	cues, _ := backfillCues()
	var once sync.Once
	e.svc.indexReader = func(r io.ReaderAt, size int64, container string) (mediaindex.Index, error) {
		once.Do(func() { // probe.file indexes the file of the second batch meanwhile
			_, _ = e.db.Write.ExecContext(e.ctx, `UPDATE media_files SET keyframes = x'01', keyframes_source = 'cues' WHERE id = ?`, last)
		})
		return cues(r, size, container)
	}
	rec := &jobs.Recorder{}
	if err := e.backfill(jobs.WithRecorder(e.ctx, rec)); err != nil {
		t.Fatal(err)
	}
	if p, ok := rec.Last(); !ok || p.Done != keyframesBackfillBatch || p.Total != keyframesBackfillBatch {
		t.Errorf("last progress = %+v, want %d of %d", p, keyframesBackfillBatch, keyframesBackfillBatch)
	}
}

// Review of wave C, m-4: a file that is gone from the disk before the scan marks it missing gets no
// full read from probe.file, from the backfill or from probe.keyframes (ENOENT on open), and the
// log says so.
func TestMissingFileGetsNoFullRead(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	files := []string{"A (2001)/A (2001).mkv", "B (2002)/B (2002).mkv", "C (2003)/C (2003).mkv"}
	dir, ids := e.videoFiles(files...)
	for _, f := range files {
		remove(t, dir, f)
	}
	var logs bytes.Buffer
	e.svc.log = slog.New(slog.NewTextHandler(&logs, nil))
	e.svc.probe = transcode.Runner{FFprobePath: fakeFFprobe(t, "h264")}
	if err := e.probeFile(ids[0]); err != nil {
		t.Fatal(err)
	}
	e.svc.probe = transcode.Runner{} // a full read would fail: ffprobe not configured
	if err := e.backfill(e.ctx); err != nil {
		t.Fatal(err)
	}
	if err := e.fullRead(ids[2]); err != nil {
		t.Errorf("probe.keyframes of a missing file = %v, want done", err)
	}
	for i, id := range ids {
		if v := e.indexState(id); v != "- jobs=0" {
			t.Errorf("file %d: index = %q, want none and no full read", i, v)
		}
	}
	if n := strings.Count(logs.String(), "media file missing, keyframe index not read"); n != 5 {
		t.Errorf("log %q, want five missing files (probe.file, backfill three times, probe.keyframes)", logs.String())
	}
	if strings.Contains(logs.String(), "full read queued") {
		t.Errorf("log %q, want no full read", logs.String())
	}
}

// Review of wave C, m-6: a library root the backfill cannot open is logged by its id, without
// its absolute path.
func TestKeyframesBackfillLogsRootWithoutPath(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	dir, _ := e.videoFiles("A (2001)/A (2001).mkv")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	e.svc.log = slog.New(slog.NewTextHandler(&logs, nil))
	if err := e.backfill(e.ctx); err != nil {
		t.Fatal(err)
	}
	if out := logs.String(); !strings.Contains(out, "library root not readable") || !strings.Contains(out, "root_id=") ||
		!strings.Contains(out, "no such file or directory") || strings.Contains(out, dir) {
		t.Errorf("log %q, want the root by its id and the cause without %s", out, dir)
	}
}

// keyframes.backfill: a schedule type in the pool general at priority 2, every 7 days, with the
// schedule off by default (O-6). Run queues one job under its key and raises a waiting one (a
// manual run at 7).
func TestKeyframesBackfillType(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	r := jobs.NewRegistry(e.queue, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e.svc.RegisterTypes(r)
	typ, ok := r.Lookup(JobKeyframesBackfill)
	if !ok || typ.Group != jobs.GroupLibrary || typ.Pool != jobs.PoolGeneral || typ.Trigger != jobs.TriggerSchedule ||
		typ.Priority != 2 || typ.Every != 7*24*time.Hour || typ.StartDelay != nil || typ.Seed == nil {
		t.Fatalf("%s = %+v", JobKeyframesBackfill, typ)
	}
	seed, err := typ.Seed(e.ctx)
	if err != nil || seed.Enabled {
		t.Errorf("seed = %+v, %v, want the schedule off", seed, err)
	}
	first, err := typ.Run(e.ctx, e.queue, typ.Priority)
	if err != nil {
		t.Fatal(err)
	}
	again, err := typ.Run(e.ctx, e.queue, jobs.ManualPriority)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(again) != 1 || first[0] != again[0] {
		t.Errorf("runs = %v, %v, want one job", first, again)
	}
	if v := e.str(`SELECT priority || ' ' || idempotency_key FROM jobs WHERE id = ?`, first[0]); v != "7 keyframes.backfill" {
		t.Errorf("backfill job = %q, want raised to 7 under its key", v)
	}
}
