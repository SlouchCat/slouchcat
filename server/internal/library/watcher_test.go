package library

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// queuedPathScans returns the number of queued scan.path jobs and the directories they name.
func (e *env) queuedPathScans() (int, []string) {
	e.t.Helper()
	rows, err := e.db.Read.QueryContext(e.ctx, `SELECT payload FROM jobs WHERE type = ? AND state = 'queued'`, JobScanPath)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	n, dirs := 0, []string{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			e.t.Fatal(err)
		}
		var p pathScanPayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			e.t.Fatalf("payload %s: %v", raw, err)
		}
		n++
		for _, r := range p.Roots {
			dirs = append(dirs, r.Dirs...)
		}
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return n, dirs
}

// waitPathScan waits for exactly one queued scan.path job naming want.
func (e *env) waitPathScan(want ...string) {
	e.t.Helper()
	waitFor(e.t, 5*time.Second, fmt.Sprintf("a queued scan.path of %q", want), func() bool {
		n, dirs := e.queuedPathScans()
		return n == 1 && slices.Equal(dirs, want)
	})
}

func TestInotifyWatcherQueuesDebouncedPathScan(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	lib := e.create(dir)
	e.cancelScans() // the scan queued by Create

	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	var calls atomic.Int32
	changed := e.svc.rootChanged(ctx, lib.ID, lib.Roots[0].ID)
	w := NewWatcher(mustRoot(t, dir), WatchInotify, func(dirs []string) {
		calls.Add(1)
		changed(dirs)
	}, discard)
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	waitFor(t, 5*time.Second, "inotify watches", func() bool { return w.Stats().Dirs == 1 })

	// Two quick changes: one report after the quiet window, one job with both new folders and the
	// root whose listing changed.
	start := time.Now()
	writeFile(t, dir, "Alien (1979)/Alien (1979).mkv", "a")
	time.Sleep(100 * time.Millisecond)
	writeFile(t, dir, "Aliens (1986)/Aliens (1986).mkv", "b")
	e.waitPathScan(".", "Alien (1979)", "Aliens (1986)")
	t.Logf("path scan queued %v after the first change", time.Since(start).Round(time.Millisecond))
	time.Sleep(w.quiet + 500*time.Millisecond)
	if n := calls.Load(); n != 1 {
		t.Errorf("two quick changes reported %d times, want 1", n)
	}
	if n, _ := e.queuedPathScans(); n != 1 || e.queuedScans() != 0 {
		t.Errorf("queued path scans = %d, full scans = %d; want 1 and 0", n, e.queuedScans())
	}
	if d := w.Stats().Dirs; d != 3 {
		t.Errorf("watched dirs = %d, want 3 (root and two new folders)", d)
	}
	if lib, err := e.svc.Get(e.ctx, lib.ID); err != nil || lib.Scan.State != "queued" {
		t.Errorf("library state = %q, %v; want queued", lib.Scan.State, err)
	}

	// A new file in a folder created after the start: the job names that folder only.
	e.cancelScans()
	writeFile(t, dir, "Alien (1979)/Alien (1979) - 2160p.mkv", "c")
	e.waitPathScan("Alien (1979)")

	// A removed folder: the folder (its files are gone) and its parent.
	e.cancelScans()
	if err := os.RemoveAll(filepath.Join(dir, "Aliens (1986)")); err != nil {
		t.Fatal(err)
	}
	e.waitPathScan(".", "Aliens (1986)")

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

// A poll watcher queues scan.path with the directories its stat pass found changed.
func TestPollWatcherQueuesPathScan(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	writeFile(t, dir, "Alien (1979)/Alien (1979).mkv", "a")
	lib := e.create(dir)
	e.cancelScans()
	ctx, cancel := context.WithCancel(e.ctx)
	defer cancel()
	w := NewWatcher(mustRoot(t, dir), WatchPoll, e.svc.rootChanged(ctx, lib.ID, lib.Roots[0].ID), discard)
	w.pause = func(time.Duration) time.Duration { return 50 * time.Millisecond }
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	waitFor(t, 5*time.Second, "the baseline pass", func() bool { return w.Stats().LastPass.Dirs == 2 })

	writeFile(t, dir, "Up (2009)/Up (2009).mkv", "u")
	e.waitPathScan(".", "Up (2009)")
	if st := w.Stats(); st.LastPass.Dirs != 3 {
		t.Errorf("last pass = %+v", st.LastPass)
	}
	if e.queuedScans() != 0 {
		t.Error("a change in one folder queued a full scan")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// Changes reported while a scan.path job waits merge into it; while it runs they wait in memory;
// more than MaxChangedDirs fall back to a full scan.
func TestTriggerPathScanCoalesces(t *testing.T) {
	e := newEnv(t)
	dir := t.TempDir()
	lib := e.create(dir)
	e.cancelScans()
	ctx, cancel := context.WithCancel(e.ctx)
	rootID := lib.Roots[0].ID

	e.svc.triggerPathScan(ctx, lib.ID, rootID, []string{"B", "."})
	e.svc.triggerPathScan(ctx, lib.ID, rootID, []string{"A", "B"})
	e.waitPathScan(".", "A", "B")
	if n := e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ? AND priority = 6`, JobScanPath, pathScanKey(lib.ID)); n != 1 {
		t.Errorf("scan.path jobs with key and priority 6 = %d, want 1", n)
	}

	// The job runs: the new directory is not merged into it but kept for the next job.
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE jobs SET state = 'running' WHERE type = ?`, JobScanPath); err != nil {
		t.Fatal(err)
	}
	e.svc.triggerPathScan(ctx, lib.ID, rootID, []string{"C"})
	if v := e.str(`SELECT payload FROM jobs WHERE type = ?`, JobScanPath); strings.Contains(v, `"C"`) {
		t.Errorf("running job changed: %s", v)
	}
	e.svc.pathMu.Lock()
	pending, retrying := e.svc.pathPending[lib.ID][rootID]["C"], e.svc.pathRetrying[lib.ID]
	e.svc.pathMu.Unlock()
	if !pending || !retrying {
		t.Errorf("directory C pending %v, retry scheduled %v; want both", pending, retrying)
	}
	cancel()
	e.svc.wg.Wait()

	// Too many directories: a full scan instead.
	e.cancelScans()
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE jobs SET state = 'done' WHERE state = 'running'`); err != nil {
		t.Fatal(err)
	}
	ctx2, cancel2 := context.WithCancel(e.ctx)
	defer cancel2()
	e.svc.pathPending = map[int64]map[int64]map[string]bool{}
	e.svc.pathRetrying = map[int64]bool{}
	e.svc.triggerPathScan(ctx2, lib.ID, rootID, []string{"x"})
	many := make([]string, MaxChangedDirs)
	for i := range many {
		many[i] = fmt.Sprintf("d%03d", i)
	}
	e.svc.triggerPathScan(ctx2, lib.ID, rootID, many)
	if e.queuedScans() != 1 {
		t.Errorf("queued full scans = %d, want 1", e.queuedScans())
	}
}

func TestPollerDetectsChanges(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Alien (1979)/Alien (1979).mkv", "a")
	writeFile(t, dir, "Alien (1979)/movie.nfo", "n")
	writeFile(t, dir, "Heat (1995)/Heat (1995).mkv", "h")
	var calls atomic.Int32
	var reported []string
	p := NewPoller(mustRoot(t, dir), func(dirs []string) {
		calls.Add(1)
		reported = dirs
	})
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
	if want := []string{"Heat (1995)"}; !slices.Equal(st.ChangedDirs, want) || !slices.Equal(reported, want) {
		t.Errorf("changed dirs = %q, reported %q; want %q", st.ChangedDirs, reported, want)
	}
	// Deleted file.
	remove(t, dir, "Alien (1979)/movie.nfo")
	if st, err = p.Pass(ctx); err != nil || st.Changed != 1 || calls.Load() != 2 {
		t.Fatalf("pass after a deleted file = %+v, %v, calls %d", st, err, calls.Load())
	}
	if want := []string{"Alien (1979)"}; !slices.Equal(reported, want) {
		t.Errorf("reported %q, want %q", reported, want)
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
	if want := []string{".", "Up (2009)"}; !slices.Equal(reported, want) {
		t.Errorf("reported %q, want %q", reported, want)
	}
	// Removed folder.
	if err := os.RemoveAll(filepath.Join(dir, "Up (2009)")); err != nil {
		t.Fatal(err)
	}
	if st, err = p.Pass(ctx); err != nil || st.Changed != 2 || st.Dirs != 3 {
		t.Fatalf("pass after a removed folder = %+v, %v", st, err)
	}
	if want := []string{".", "Up (2009)"}; !slices.Equal(reported, want) {
		t.Errorf("reported %q after removal, want %q", reported, want)
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
	w := NewWatcher(mustRoot(t, dir), WatchPoll, e.svc.rootChanged(ctx, lib.ID, lib.Roots[0].ID), discard)
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
	// Start queues the scan and the sweep after the watchers are up; on a slow runner the watcher
	// reports its first pass before those rows exist, so wait for them like for the watcher.
	waitFor(t, 5*time.Second, "the scan queued by Start", func() bool { return e.queuedScans() == 1 })
	waitFor(t, 5*time.Second, "the sweep queued by Start", func() bool {
		return e.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = 'sweep'`, JobSweep) == 1
	})
	// The dashboard's inotify use is the sum of the watched directories of every root (M8).
	if st := e.svc.WatcherStats()[lib.Roots[0].ID]; e.svc.WatchedDirs() != int64(st.Dirs) {
		t.Errorf("WatchedDirs = %d, want %d of the only root", e.svc.WatchedDirs(), st.Dirs)
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
