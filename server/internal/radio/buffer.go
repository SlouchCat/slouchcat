package radio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The buffer of a relay (concept 16 sections 7.2 and 7.3, build plan S6): the segments of the
// window, on the tmpfs below <dir>/<station>/<gen>/ (init.mp4 and <n>.m4s, written atomically), or
// in RAM when no tmpfs is mounted (memory mode: live only, the init and the last segments). The
// relay counts the bytes it holds and drops its oldest segments first when it would exceed its
// reservation, so all relays together never exceed the budget (hard part 3.4).

const (
	// memorySegments is how many segments a relay keeps in memory mode.
	memorySegments = 8
	// reserveFactor is the margin of a reservation over bitrate × window (container overhead).
	reserveFactor = 1.03
	// defaultReserveKbps is the bitrate a reservation assumes when a copied stream names none.
	defaultReserveKbps = 320
)

// segment is one segment of the window.
type segment struct {
	seq  int64
	gen  int
	dur  time.Duration
	pdt  time.Time // program date-time of its first sample
	disc bool      // the first segment of a generation after another one: EXT-X-DISCONTINUITY
	size int64
	data []byte // memory mode only
}

// buffer holds the window of one relay. Not safe for concurrent use (the relay's mutex).
type buffer struct {
	dir         string // <dir>/<station>; "" = memory mode
	window      time.Duration
	reservation int64

	inits       map[int][]byte
	segs        []*segment
	total       time.Duration
	bytes       int64
	discDropped int64 // discontinuities that left the window (EXT-X-DISCONTINUITY-SEQUENCE)
	nextSeq     int64
}

func newBuffer(dir string, window time.Duration, reservation int64) (*buffer, error) {
	if dir != "" {
		if err := os.RemoveAll(dir); err != nil {
			return nil, fmt.Errorf("clear radio buffer: %w", err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create radio buffer: %w", err)
		}
	}
	return &buffer{dir: dir, window: window, reservation: reservation, inits: map[int][]byte{}}, nil
}

// reservationBytes is the reservation of a relay: bitrate × window × reserveFactor.
func reservationBytes(kbps int, window time.Duration) int64 {
	return int64(float64(kbps) * 1000 / 8 * window.Seconds() * reserveFactor)
}

func (b *buffer) genDir(gen int) string { return filepath.Join(b.dir, strconv.Itoa(gen)) }

// addInit stores the init part of generation gen.
func (b *buffer) addInit(gen int, data []byte) error {
	b.inits[gen] = data
	b.bytes += int64(len(data))
	if b.dir == "" {
		return nil
	}
	if err := os.MkdirAll(b.genDir(gen), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(b.genDir(gen), "init.mp4"), data)
}

// add appends a segment and drops old ones beyond the window, the reservation or (memory mode)
// memorySegments; the newest segment always stays.
func (b *buffer) add(seg *segment, data []byte) error {
	seg.seq = b.nextSeq
	seg.size = int64(len(data))
	if b.dir == "" {
		seg.data = data
	} else if err := writeFileAtomic(filepath.Join(b.genDir(seg.gen), strconv.FormatInt(seg.seq, 10)+".m4s"), data); err != nil {
		return err
	}
	b.nextSeq++
	b.segs = append(b.segs, seg)
	b.total += seg.dur
	b.bytes += seg.size
	for len(b.segs) > 1 && (b.total-b.segs[0].dur >= b.window || b.bytes > b.reservation ||
		b.dir == "" && len(b.segs) > memorySegments) {
		b.drop()
	}
	return nil
}

// drop removes the oldest segment, and the init of its generation once no segment of it is left
// and a newer generation exists.
func (b *buffer) drop() {
	old := b.segs[0]
	b.segs = b.segs[1:]
	b.total -= old.dur
	b.bytes -= old.size
	if old.disc {
		b.discDropped++
	}
	if b.dir != "" {
		_ = os.Remove(filepath.Join(b.genDir(old.gen), strconv.FormatInt(old.seq, 10)+".m4s"))
	}
	if len(b.segs) > 0 && b.segs[0].gen != old.gen {
		b.bytes -= int64(len(b.inits[old.gen]))
		delete(b.inits, old.gen)
		if b.dir != "" {
			_ = os.RemoveAll(b.genDir(old.gen))
		}
	}
}

// find returns the segment seq of the window.
func (b *buffer) find(seq int64) (*segment, bool) {
	if len(b.segs) == 0 || seq < b.segs[0].seq || seq > b.segs[len(b.segs)-1].seq {
		return nil, false
	}
	return b.segs[seq-b.segs[0].seq], true
}

// path returns the file of a segment (tmpfs mode).
func (b *buffer) path(seg *segment) string {
	return filepath.Join(b.genDir(seg.gen), strconv.FormatInt(seg.seq, 10)+".m4s")
}

// readSegment returns the data of a segment: from RAM, or from its file; a segment dropped in the
// meantime is model.ErrNotFound.
func readSegment(seg *segment, path string) ([]byte, error) {
	if seg.data != nil {
		return seg.data, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: path built by the relay below its own buffer directory
	if errors.Is(err, os.ErrNotExist) {
		return nil, model.ErrNotFound
	}
	return data, err
}

// remove deletes the buffer directory of the relay.
func (b *buffer) remove() {
	if b.dir != "" {
		_ = os.RemoveAll(b.dir)
	}
}

// writeFileAtomic writes data to a temporary file and renames it over path, so a reader never
// sees a partial file.
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
