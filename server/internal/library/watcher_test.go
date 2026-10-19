package library

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/paths"
)

func mustRoot(t *testing.T, dir string) paths.Root {
	t.Helper()
	r, err := paths.NewRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// waitFor polls cond until it holds or the timeout passes.
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout after %v waiting for %s", timeout, what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (e *env) queuedScans() int {
	return e.count(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued'`, JobScanLibrary)
}

func (e *env) cancelScans() {
	e.t.Helper()
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE jobs SET state = 'cancelled' WHERE state = 'queued'`); err != nil {
		e.t.Fatal(err)
	}
}

func TestInotifyWatcherQueuesDebouncedScan(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	lib := e.create(dir)
	e.cancelScans() // the scan queued by Create

	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	var calls atomic.Int32
	w := NewWatcher(mustRoot(t, dir), WatchInotify, func() {
		calls.Add(1)
		e.svc.triggerScan(ctx, lib.ID)
	}, discard)
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	waitFor(t, 5*time.Second, "inotify watches", func() bool { return w.Stats().Dirs == 1 })

	start := time.Now()
	writeFile(t, dir, "Alien (1979)/Alien (1979).mkv", "a")
	time.Sleep(100 * time.Millisecond)
	writeFile(t, dir, "Aliens (1986)/Aliens (1986).mkv", "b")
	waitFor(t, 5*time.Second, "a queued scan.library job", func() bool { return e.queuedScans() == 1 })
	t.Logf("scan queued %v after the first change", time.Since(start).Round(time.Millisecond))
	time.Sleep(w.quiet + 500*time.Millisecond)
	if n := calls.Load(); n != 1 {
		t.Errorf("two quick changes triggered %d times, want 1", n)
	}
	if n := e.queuedScans(); n != 1 {
		t.Errorf("queued scans = %d, want 1", n)
	}
	if d := w.Stats().Dirs; d != 3 {
		t.Errorf("watched dirs = %d, want 3 (root and two new folders)", d)
	}
	if lib, err := e.svc.Get(e.ctx, lib.ID); err != nil || lib.Scan.State != "queued" {
		t.Errorf("library state = %q, %v; want queued", lib.Scan.State, err)
	}

	// A change inside a directory created after the start is seen as well.
	e.cancelScans()
	writeFile(t, dir, "Alien (1979)/movie.nfo", "<movie/>")
	waitFor(t, 5*time.Second, "a scan after a change in a new folder", func() bool { return e.queuedScans() == 1 })

	// Hidden and ignored entries do not trigger.
	e.cancelScans()
	before := calls.Load()
	writeFile(t, dir, ".partial~", "x")
	if err := os.Mkdir(filepath.Join(dir, "@eaDir"), 0o750); err != nil {
		t.Fatal(err)
	}
	time.Sleep(w.quiet + 500*time.Millisecond)
	if calls.Load() != before {
		t.Error("ignored entries triggered a scan")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not stop")
	}
}

func TestPollerDetectsChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Alien (1979)/Alien (1979).mkv", "a")
	writeFile(t, dir, "Alien (1979)/movie.nfo", "n")
	writeFile(t, dir, "Heat (1995)/Heat (1995).mkv", "h")
	var calls atomic.Int32
	p := NewPoller(mustRoot(t, dir), func() { calls.Add(1) })
	ctx := context.Background()

	st, err := p.Pass(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Dirs != 3 || st.Files != 3 || st.Changed != 0 || calls.Load() != 0 {
		t.Fatalf("first pass = %+v, calls %d; want 3 dirs, 3 files, baseline only", st, calls.Load())
	}
	snap := p.snapshot()

	// Identical tree: identical hashes, no change.
	st, err = p.Pass(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Changed != 0 || calls.Load() != 0 {
		t.Fatalf("second pass = %+v, want no change", st)
	}
	again := p.snapshot()
	if len(again) != len(snap) {
		t.Fatalf("snapshot sizes differ: %d vs %d", len(again), len(snap))
	}
	for d, h := range snap {
		if again[d] != h {
			t.Errorf("hash of %q not stable across identical passes", d)
		}
	}

	// New file.
	writeFile(t, dir, "Heat (1995)/Heat (1995) - 2160p.mkv", "h4k")
	if st, err = p.Pass(ctx); err != nil || st.Changed != 1 || calls.Load() != 1 {
		t.Fatalf("pass after a new file = %+v, %v, calls %d; want 1 changed dir", st, err, calls.Load())
	}
	// Deleted file.
	remove(t, dir, "Alien (1979)/movie.nfo")
	if st, err = p.Pass(ctx); err != nil || st.Changed != 1 || calls.Load() != 2 {
		t.Fatalf("pass after a deleted file = %+v, %v, calls %d", st, err, calls.Load())
	}
	// Changed size of a file.
	writeFile(t, dir, "Heat (1995)/Heat (1995).mkv", "longer content")
	if st, err = p.Pass(ctx); err != nil || st.Changed != 1 {
		t.Fatalf("pass after a changed file = %+v, %v", st, err)
	}
	// New folder: the parent listing and the new folder change.
	writeFile(t, dir, "Up (2009)/Up (2009).mkv", "u")
	if st, err = p.Pass(ctx); err != nil || st.Changed != 2 || st.Dirs != 4 {
		t.Fatalf("pass after a new folder = %+v, %v; want 2 changed of 4 dirs", st, err)
	}
	// Removed folder.
	if err := os.RemoveAll(filepath.Join(dir, "Up (2009)")); err != nil {
		t.Fatal(err)
	}
	if st, err = p.Pass(ctx); err != nil || st.Changed != 2 || st.Dirs != 3 {
		t.Fatalf("pass after a removed folder = %+v, %v", st, err)
	}
	// Ignored entries do not count.
	calls.Store(0)
	writeFile(t, dir, ".hidden", "x")
	writeFile(t, dir, "@eaDir/thumb.jpg", "x")
	if st, err = p.Pass(ctx); err != nil || st.Changed != 0 || calls.Load() != 0 {
		t.Fatalf("pass after ignored entries = %+v, %v", st, err)
	}
	// A missing root is an error, not a change.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Pass(ctx); err == nil {
		t.Fatal("pass over a missing root succeeded")
	}
}

func TestPollInterval(t *testing.T) {
	cases := map[time.Duration]time.Duration{
		0:                45 * time.Second,
		time.Second:      45 * time.Second,
		15 * time.Second: 45 * time.Second,
		16 * time.Second: 48 * time.Second,
		2 * time.Minute:  6 * time.Minute,
	}
	for last, want := range cases {
		if got := PollInterval(last); got != want {
			t.Errorf("PollInterval(%v) = %v, want %v", last, got, want)
		}
	}
}

func TestPollWatcherQueuesScan(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	lib := e.create(dir)
	e.cancelScans()
	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	w := NewWatcher(mustRoot(t, dir), WatchPoll, func() { e.svc.triggerScan(ctx, lib.ID) }, discard)
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	waitFor(t, 5*time.Second, "the first stat pass", func() bool { return w.Stats().Interval == minPollInterval })
	if st := w.Stats(); st.Mode != WatchPoll || st.LastPass.Dirs != 1 || st.Dirs != 0 {
		t.Errorf("stats = %+v", st)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestStartRunsWatchersAndSchedules(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, "Alien (1979)/Alien (1979).mkv", "a")
	lib := e.create(dir)
	e.cancelScans()

	ctx, cancel := context.WithCancel(e.ctx)
	done := make(chan error, 1)
	go func() { done <- e.svc.Start(ctx) }()
	waitFor(t, 5*time.Second, "the watcher of the root", func() bool {
		st, ok := e.svc.WatcherStats()[lib.Roots[0].ID]
		return ok && (st.Dirs > 0 || st.LastPass.Dirs > 0)
	})
	if n := e.queuedScans(); n != 1 {
		t.Errorf("scans queued by Start = %d, want 1", n)
	}
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = 'sweep'`, JobSweep); n != 1 {
		t.Errorf("sweep jobs = %d, want 1", n)
	}
	// A library created while Start runs gets its watcher.
	dir2 := t.TempDir()
	lib2, err := e.svc.Create(e.ctx, testRequest("Zweite", dir2))
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, "the watcher of the new library", func() bool {
		_, ok := e.svc.WatcherStats()[lib2.Roots[0].ID]
		return ok
	})
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after cancel")
	}
}

// TestPollerPassOverTestMedia measures a stat pass over the generated test media.
func TestPollerPassOverTestMedia(t *testing.T) {
	p := NewPoller(mustRoot(t, filepath.Dir(testMedia(t))), nil)
	if _, err := p.Pass(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, err := p.Pass(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Dirs < 10 || st.Files < 10 || st.Changed != 0 {
		t.Fatalf("pass = %+v", st)
	}
	t.Logf("stat pass over testdata/media: %d dirs, %d files in %v (next pass in %v)", st.Dirs, st.Files, st.Duration, PollInterval(st.Duration))
}
