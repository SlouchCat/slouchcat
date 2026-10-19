package library

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/paths"
)

// Watch modes of a library root (column library_roots.watch, plan 3.4).
const (
	WatchInotify     = "inotify"      // local file systems: ext4, xfs, btrfs, zfs
	WatchPoll        = "poll"         // network shares: NFS, SMB/CIFS
	WatchInotifyPoll = "inotify_poll" // FUSE (Unraid shfs, mergerfs) and unknown file systems
)

// maxUserWatchesFile holds the per-user inotify watch limit of the host.
const maxUserWatchesFile = "/proc/sys/fs/inotify/max_user_watches"

// WatchStats is the state of a Watcher.
type WatchStats struct {
	Mode     string
	Dirs     int           // directories watched by inotify
	LastPass PassStats     // last stat pass (poll modes)
	Interval time.Duration // pause before the next stat pass
}

// Watcher reports changes below one library root by calling onChange, at most once per quiet
// period of jobs.DebounceWindow for inotify events and once per changed stat pass.
type Watcher struct {
	root     paths.Root
	mode     string
	onChange func()
	log      *slog.Logger
	quiet    time.Duration

	events atomic.Int64 // inotify events since the last stat pass (inotify_poll)

	mu       sync.Mutex
	fw       *fsnotify.Watcher
	last     PassStats
	interval time.Duration
}

// NewWatcher returns a watcher of root in mode (WatchInotify, WatchPoll or WatchInotifyPoll).
func NewWatcher(root paths.Root, mode string, onChange func(), log *slog.Logger) *Watcher {
	return &Watcher{root: root, mode: mode, onChange: onChange, log: log, quiet: jobs.DebounceWindow}
}

// Stats returns the watched directory count, the last stat pass and the current poll interval.
func (w *Watcher) Stats() WatchStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	st := WatchStats{Mode: w.mode, LastPass: w.last, Interval: w.interval}
	if w.fw != nil {
		st.Dirs = len(w.fw.WatchList())
	}
	return st
}

// Run watches until ctx ends.
func (w *Watcher) Run(ctx context.Context) error {
	switch w.mode {
	case WatchInotify:
		return w.runInotify(ctx)
	case WatchPoll:
		return w.runPoll(ctx, false)
	default:
		errc := make(chan error, 1)
		go func() { errc <- w.runInotify(ctx) }()
		perr := w.runPoll(ctx, true)
		return errors.Join(perr, <-errc)
	}
}

func (w *Watcher) runPoll(ctx context.Context, withInotify bool) error {
	p := NewPoller(w.root, func() {
		if withInotify && w.events.Load() == 0 {
			w.log.Warn("stat pass found changes that inotify did not report; this file system needs polling")
		}
		w.onChange()
	})
	failing := false
	for {
		st, err := p.Pass(ctx)
		// Count inotify events from here to the next pass, which compares against them.
		w.events.Store(0)
		if ctx.Err() != nil {
			return nil
		}
		switch {
		case err != nil && !failing:
			w.log.Warn("stat pass failed", "err", err)
			failing = true
		case err == nil && failing:
			w.log.Info("stat pass works again")
			failing = false
		}
		interval := PollInterval(st.Duration)
		w.mu.Lock()
		if err == nil {
			w.last = st
		}
		w.interval = interval
		w.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}
}

func (w *Watcher) runInotify(ctx context.Context) error {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fw.Close()
	r, err := paths.Open(w.root)
	if err != nil {
		return err
	}
	defer r.Close()
	w.mu.Lock()
	w.fw = fw
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.fw = nil
		w.mu.Unlock()
	}()

	w.addTree(fw, r, ".")
	if limit := maxUserWatches(); limit > 0 {
		if dirs := len(fw.WatchList()); dirs*10 > limit*8 {
			w.log.Warn("inotify watches above 80 % of fs.inotify.max_user_watches; raise the limit on the host",
				"watched_dirs", dirs, "max_user_watches", limit)
		}
	}

	var debounce <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-fw.Events:
			if !ok {
				return nil
			}
			rel, ok := w.relative(ev.Name)
			if !ok || Ignored(path.Base(rel)) || ev.Op == fsnotify.Chmod {
				continue
			}
			if ev.Has(fsnotify.Create) {
				if fi, err := r.Lstat(rel); err == nil && fi.IsDir() {
					w.addTree(fw, r, rel)
				}
			}
			w.events.Add(1)
			debounce = time.After(w.quiet)
		case err, ok := <-fw.Errors:
			if !ok {
				return nil
			}
			w.log.Warn("inotify error", "err", err)
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				w.onChange()
			}
		case <-debounce:
			debounce = nil
			w.onChange()
		}
	}
}

// relative turns an absolute event path into a path relative to the root.
func (w *Watcher) relative(name string) (string, bool) {
	dir := w.root.Dir()
	if name == dir {
		return ".", true
	}
	rel, ok := strings.CutPrefix(name, strings.TrimSuffix(dir, "/")+"/")
	return rel, ok && rel != ""
}

// addTree watches rel and every directory below it, skipping ignored ones.
func (w *Watcher) addTree(fw *fsnotify.Watcher, r *os.Root, rel string) {
	_ = fs.WalkDir(r.FS(), rel, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			// Vanished or unreadable entries are skipped; the next scan sees them.
			return nil
		}
		if p != "." && Ignored(d.Name()) {
			return fs.SkipDir
		}
		abs, err := paths.Resolve(w.root, p)
		if err != nil {
			return fs.SkipDir
		}
		if err := fw.Add(abs); err != nil {
			w.log.Warn("inotify watch failed; changes below this directory need the periodic scan", "dir", p, "err", err)
			return fs.SkipAll
		}
		return nil
	})
}

// maxUserWatches reads the inotify watch limit; 0 when unknown.
func maxUserWatches() int {
	b, err := os.ReadFile(maxUserWatchesFile)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	return n
}
