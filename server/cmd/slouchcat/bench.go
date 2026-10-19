package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/paths"
)

// watchbench measures the change detection of one library root (plan 3.4, decision log row 3):
// the duration of the stat pass of the poller over all directories (the strategy for NFS/SMB) and,
// when requested, the time and watch count to register the whole tree with inotify plus the
// latency until a new file is reported. It writes the numbers for docs/perf/<Mx>.md.
//
//	slouchcat watchbench --root DIR [--passes 3] [--workers 8] [--inotify]
func watchbench(args []string) int {
	fs := flag.NewFlagSet("watchbench", flag.ContinueOnError)
	root := fs.String("root", "", "absolute directory to measure")
	passes := fs.Int("passes", 3, "number of stat passes")
	workers := fs.Int("workers", library.DefaultPollWorkers, "concurrent directory listings of the stat pass")
	inotify := fs.Bool("inotify", false, "also register the tree with inotify and measure the event latency")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *root == "" {
		fmt.Fprintln(os.Stderr, "watchbench: --root is required")
		return 2
	}
	if *workers < 1 {
		fmt.Fprintln(os.Stderr, "watchbench: --workers must be at least 1")
		return 2
	}
	if err := runWatchbench(*root, *passes, *workers, *inotify); err != nil {
		fmt.Fprintln(os.Stderr, "watchbench:", err)
		return 1
	}
	return 0
}

func runWatchbench(dir string, passes, workers int, inotify bool) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	root, err := paths.NewRoot(abs)
	if err != nil {
		return err
	}
	fsType, err := paths.DetectFS(abs)
	if err != nil {
		return err
	}
	fmt.Printf("root: %s (fs %s)\n", abs, fsType)

	changed := 0
	poller := library.NewPoller(root, func([]string) { changed++ })
	poller.Workers = workers
	fmt.Printf("poller: %d workers\n", workers)
	ctx := context.Background()
	for i := range passes {
		st, err := poller.Pass(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("pass %d: %d dirs, %d files, %d changed, %s (next interval %s)\n",
			i+1, st.Dirs, st.Files, st.Changed, st.Duration.Round(time.Millisecond), library.PollInterval(st.Duration))
	}
	if !inotify {
		return nil
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	seen := make(chan struct{}, 1)
	w := library.NewWatcher(root, library.WatchInotify, func([]string) {
		select {
		case seen <- struct{}{}:
		default:
		}
	}, log)
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	start := time.Now()
	runErr := make(chan error, 1)
	go func() { runErr <- w.Run(wctx) }()
	// The watcher registers directories while walking; the count is stable once it stops growing.
	last := -1
	for {
		time.Sleep(200 * time.Millisecond)
		n := w.Stats().Dirs
		if n == last {
			break
		}
		last = n
		select {
		case err := <-runErr:
			return fmt.Errorf("watcher stopped during setup: %w", err)
		default:
		}
	}
	fmt.Printf("inotify: %d directories watched after %s\n", last, time.Since(start).Round(time.Millisecond))
	if max, err := os.ReadFile("/proc/sys/fs/inotify/max_user_watches"); err == nil {
		fmt.Printf("inotify: fs.inotify.max_user_watches = %s", max)
	}

	probe := filepath.Join(abs, "watchbench-probe.mkv")
	created := time.Now()
	if err := os.WriteFile(probe, []byte("probe"), 0o600); err != nil {
		return err
	}
	defer os.Remove(probe)
	select {
	case <-seen:
		fmt.Printf("inotify: new file reported after %s (includes the %s debounce)\n",
			time.Since(created).Round(time.Millisecond), "2 s")
	case <-time.After(30 * time.Second):
		return errors.New("inotify: new file not reported within 30 s")
	}
	cancel()
	return nil
}
