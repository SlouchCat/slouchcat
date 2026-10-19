package library

import (
	"context"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/paths"
)

// minPollInterval is the shortest pause between two stat passes (plan 3.4).
const minPollInterval = 45 * time.Second

// PollInterval returns the pause after a stat pass that took last: max(45 s, 3 × last), so a
// slow share is never busy more than a quarter of the time.
func PollInterval(last time.Duration) time.Duration {
	return max(minPollInterval, 3*last)
}

// DefaultPollWorkers is the number of concurrent directory listings of a stat pass. Over SMB a
// listing costs a network round trip of several milliseconds, so a pass is latency-bound and
// overlapping the listings shortens it nearly linearly up to the server's limit.
const DefaultPollWorkers = 16

// PassStats describes one stat pass of a Poller.
type PassStats struct {
	Dirs     int           // directories listed
	Files    int           // other entries seen (files, symlinks)
	Changed  int           // directories new, gone or with a different listing than in the previous pass
	Duration time.Duration // wall time of the pass
}

// Poller detects changes below a root without inotify (NFS, SMB): each pass lists every
// directory and compares a hash of its sorted entries (name, size, mtime) with the previous pass.
// It reads directory listings and the attributes that come with them; it never opens files.
//
// Every directory is listed in every pass, even when its own mtime and size are unchanged: a
// file that grows or is rewritten in place changes its own size and mtime but not those of the
// directory, so only the listing of the directory itself reveals it.
type Poller struct {
	root     paths.Root
	onChange func()

	// Workers is the number of concurrent directory listings; 0 or less means DefaultPollWorkers.
	// Set it before the first pass.
	Workers int

	mu   sync.Mutex
	snap map[string]uint64 // directory → listing hash; nil before the first pass
}

// NewPoller returns a poller for root. onChange runs after a pass that found changes (never
// after the first pass, which only records the baseline). It may be nil.
func NewPoller(root paths.Root, onChange func()) *Poller {
	return &Poller{root: root, onChange: onChange}
}

// listing is the result of listing one directory.
type listing struct {
	dir     string
	sum     uint64   // hash of the sorted entries
	files   int      // non-directory entries
	subdirs []string // relative paths of the subdirectories to list next
	gone    bool     // the directory vanished during the pass
	err     error
}

// Pass runs one stat pass. Directories are listed by a bounded pool of workers that share one
// *os.Root; the calling goroutine owns the queue of directories still to list and adds the
// subdirectories of each finished listing, so the walk itself runs in parallel. The result does
// not depend on scheduling: each directory's hash covers only its own sorted entries.
func (p *Poller) Pass(ctx context.Context) (PassStats, error) {
	start := time.Now()
	r, err := paths.Open(p.root)
	if err != nil {
		return PassStats{}, err
	}
	defer r.Close()

	workers := p.Workers
	if workers <= 0 {
		workers = DefaultPollWorkers
	}
	ctx, cancel := context.WithCancel(ctx)
	jobs := make(chan string)
	results := make(chan listing)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for dir := range jobs {
				res := listDir(r, dir)
				select {
				case results <- res:
				case <-ctx.Done():
					return
				}
			}
		})
	}
	// Stop the workers before the root closes; a listing in flight finishes first.
	defer func() {
		cancel()
		close(jobs)
		wg.Wait()
	}()

	var stats PassStats
	next := map[string]uint64{}
	queue := []string{"."}
	inflight := 0
	for len(queue) > 0 || inflight > 0 {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		var send chan string
		var dir string
		if len(queue) > 0 {
			send, dir = jobs, queue[len(queue)-1]
		}
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		case send <- dir:
			queue = queue[:len(queue)-1]
			inflight++
		case res := <-results:
			inflight--
			if res.err != nil {
				return stats, res.err
			}
			if res.gone {
				continue // removed during the pass; the next pass reports it
			}
			next[res.dir] = res.sum
			stats.Dirs++
			stats.Files += res.files
			queue = append(queue, res.subdirs...)
		}
	}

	p.mu.Lock()
	if p.snap != nil {
		for d, sum := range next {
			if old, ok := p.snap[d]; !ok || old != sum {
				stats.Changed++
			}
		}
		for d := range p.snap {
			if _, ok := next[d]; !ok {
				stats.Changed++
			}
		}
	}
	p.snap = next
	p.mu.Unlock()
	stats.Duration = time.Since(start)
	if stats.Changed > 0 && p.onChange != nil {
		p.onChange()
	}
	return stats, nil
}

// listDir lists dir below r and hashes its sorted entries (name, and size and mtime for
// non-directories), skipping ignored names.
func listDir(r *os.Root, dir string) listing {
	res := listing{dir: dir}
	f, err := r.Open(dir)
	if err != nil {
		if dir != "." && errors.Is(err, fs.ErrNotExist) {
			res.gone = true
			return res
		}
		res.err = err
		return res
	}
	entries, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil {
		res.err = err
		return res
	}
	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	h := fnv.New64a()
	var buf [8]byte
	for _, e := range entries {
		if Ignored(e.Name()) {
			continue
		}
		_, _ = h.Write([]byte(e.Name()))
		if e.IsDir() {
			_, _ = h.Write([]byte{0, 'd', 0})
			res.subdirs = append(res.subdirs, path.Join(dir, e.Name()))
			continue
		}
		info, err := e.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			res.err = err
			return res
		}
		binary.LittleEndian.PutUint64(buf[:], uint64(info.Size())) //nolint:gosec // G115: hashed bit pattern
		_, _ = h.Write([]byte{0, 'f'})
		_, _ = h.Write(buf[:])
		binary.LittleEndian.PutUint64(buf[:], uint64(info.ModTime().UnixNano()))
		_, _ = h.Write(buf[:])
		res.files++
	}
	res.sum = h.Sum64()
	return res
}

// snapshot returns a copy of the directory hashes of the last pass, for tests.
func (p *Poller) snapshot() map[string]uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return maps.Clone(p.snap)
}
