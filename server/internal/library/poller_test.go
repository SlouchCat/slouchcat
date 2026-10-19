package library

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// writeTree creates 30 show folders with 9 season folders each (300 directories below the
// root) and 10 files in every one of them.
func writeTree(t *testing.T, dir string) {
	t.Helper()
	for s := range 30 {
		show := fmt.Sprintf("Show %02d", s)
		for f := range 10 {
			writeFile(t, dir, fmt.Sprintf("%s/extra-%02d.nfo", show, f), "x")
		}
		for season := range 9 {
			for e := range 10 {
				writeFile(t, dir, fmt.Sprintf("%s/Season %02d/%s - S%02dE%02d.mkv", show, season+1, show, season+1, e+1), show)
			}
		}
	}
}

func newPollers(t *testing.T, dir string) (serial, parallel *Poller) {
	t.Helper()
	serial = NewPoller(mustRoot(t, dir), nil)
	serial.Workers = 1
	parallel = NewPoller(mustRoot(t, dir), nil)
	parallel.Workers = 8
	return serial, parallel
}

// passBoth runs one pass with each poller and checks that both see the same tree.
func passBoth(t *testing.T, serial, parallel *Poller) (PassStats, PassStats) {
	t.Helper()
	ctx := context.Background()
	s1, err := serial.Pass(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s8, err := parallel.Pass(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if s1.Dirs != s8.Dirs || s1.Files != s8.Files || s1.Changed != s8.Changed {
		t.Fatalf("1 worker: %+v; 8 workers: %+v", s1, s8)
	}
	if a, b := serial.snapshot(), parallel.snapshot(); !maps.Equal(a, b) {
		t.Fatalf("snapshots differ: %d vs %d directories", len(a), len(b))
	}
	return s1, s8
}

func TestPollerParallelMatchesSerial(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir)
	serial, parallel := newPollers(t, dir)

	st, _ := passBoth(t, serial, parallel)
	if st.Dirs != 301 || st.Files != 3000 || st.Changed != 0 {
		t.Fatalf("first pass = %+v; want 301 dirs, 3000 files, baseline only", st)
	}
	// Repeated parallel passes over an unchanged tree give the same snapshot.
	base := parallel.snapshot()
	for range 3 {
		if st, err := parallel.Pass(context.Background()); err != nil || st.Changed != 0 {
			t.Fatalf("pass over an unchanged tree = %+v, %v", st, err)
		}
		if !maps.Equal(base, parallel.snapshot()) {
			t.Fatal("snapshot changed between identical passes")
		}
	}

	writeFile(t, dir, "Show 07/Season 03/Show 07 - S03E11.mkv", "new")
	if _, st := passBoth(t, serial, parallel); st.Changed != 1 {
		t.Fatalf("pass after a new file = %+v; want 1 changed dir", st)
	}
	remove(t, dir, "Show 21/Season 09/Show 21 - S09E04.mkv")
	if _, st := passBoth(t, serial, parallel); st.Changed != 1 || st.Files != 3000 {
		t.Fatalf("pass after a deleted file = %+v; want 1 changed dir", st)
	}
	writeFile(t, dir, "Show 12/Season 10/Show 12 - S10E01.mkv", "new")
	if _, st := passBoth(t, serial, parallel); st.Changed != 2 || st.Dirs != 302 {
		t.Fatalf("pass after a new folder = %+v; want 2 changed of 302 dirs", st)
	}
	if err := os.RemoveAll(filepath.Join(dir, "Show 12", "Season 10")); err != nil {
		t.Fatal(err)
	}
	if _, st := passBoth(t, serial, parallel); st.Changed != 2 || st.Dirs != 301 {
		t.Fatalf("pass after a removed folder = %+v; want 2 changed of 301 dirs", st)
	}
}

func TestPollerPassCancel(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir)
	p := NewPoller(mustRoot(t, dir), func() { t.Error("onChange after a cancelled pass") })
	if _, err := p.Pass(context.Background()); err != nil {
		t.Fatal(err)
	}
	base := p.snapshot()
	writeFile(t, dir, "Show 00/new.mkv", "x")
	goroutines := runtime.NumGoroutine()

	// Cancelled before the start: no listing completes and the baseline stays.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if _, err := p.Pass(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("pass with a cancelled context = %v, want context.Canceled", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("cancelled pass took %v", d)
	}
	if !maps.Equal(base, p.snapshot()) {
		t.Error("a cancelled pass replaced the snapshot")
	}

	// Cancelled while the workers run: the pass stops promptly and leaves no worker behind.
	p.onChange = nil
	cancelled := 0
	for range 20 {
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(100*time.Microsecond, cancel)
		start := time.Now()
		_, err := p.Pass(ctx)
		cancel()
		switch {
		case errors.Is(err, context.Canceled):
			cancelled++
		case err != nil:
			t.Fatalf("pass cancelled while running = %v", err)
		}
		if d := time.Since(start); d > time.Second {
			t.Errorf("pass cancelled after 100 µs took %v", d)
		}
	}
	if cancelled == 0 {
		t.Error("no pass was cancelled while running")
	}
	waitFor(t, 5*time.Second, "the workers to stop", func() bool { return runtime.NumGoroutine() <= goroutines })
}
