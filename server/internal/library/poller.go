package library

import (
	"context"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"io/fs"
	"maps"
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
type Poller struct {
	root     paths.Root
	onChange func()

	mu   sync.Mutex
	snap map[string]uint64 // directory → listing hash; nil before the first pass
}

// NewPoller returns a poller for root. onChange runs after a pass that found changes (never
// after the first pass, which only records the baseline). It may be nil.
func NewPoller(root paths.Root, onChange func()) *Poller {
	return &Poller{root: root, onChange: onChange}
}

// Pass runs one stat pass.
func (p *Poller) Pass(ctx context.Context) (PassStats, error) {
	start := time.Now()
	r, err := paths.Open(p.root)
	if err != nil {
		return PassStats{}, err
	}
	defer r.Close()

	var stats PassStats
	next := map[string]uint64{}
	stack := []string{"."}
	var buf [8]byte
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		dir := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		f, err := r.Open(dir)
		if err != nil {
			if dir != "." && errors.Is(err, fs.ErrNotExist) {
				continue // removed during the pass; the next pass reports it
			}
			return stats, err
		}
		entries, err := f.ReadDir(-1)
		_ = f.Close()
		if err != nil {
			return stats, err
		}
		slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
		h := fnv.New64a()
		for _, e := range entries {
			if Ignored(e.Name()) {
				continue
			}
			_, _ = h.Write([]byte(e.Name()))
			if e.IsDir() {
				_, _ = h.Write([]byte{0, 'd', 0})
				stack = append(stack, path.Join(dir, e.Name()))
				continue
			}
			info, err := e.Info()
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return stats, err
			}
			binary.LittleEndian.PutUint64(buf[:], uint64(info.Size())) //nolint:gosec // G115: hashed bit pattern
			_, _ = h.Write([]byte{0, 'f'})
			_, _ = h.Write(buf[:])
			binary.LittleEndian.PutUint64(buf[:], uint64(info.ModTime().UnixNano()))
			_, _ = h.Write(buf[:])
			stats.Files++
		}
		next[dir] = h.Sum64()
		stats.Dirs++
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

// snapshot returns a copy of the directory hashes of the last pass, for tests.
func (p *Poller) snapshot() map[string]uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return maps.Clone(p.snap)
}
