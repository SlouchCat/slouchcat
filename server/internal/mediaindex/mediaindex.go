// Package mediaindex reads the keyframe times a container already carries (Matroska cues, the
// MP4 sample tables) from an io.ReaderAt. It opens no file and starts no process: the caller
// hands it a file opened through paths (rule 4); the check against ffprobe lives in transcode
// (rule 3).
//
// The bytes come from outside (downloads, release groups), so the parsers trust no size (concept
// 14, section 9.1): every read has a fixed limit and nothing is allocated from a size above it,
// positions stay inside the file, sums and products are checked for overflow, an index holds at
// most maxEntries keyframes, and the element nesting is fixed by the code (at most six levels;
// the one recursion, a SeekHead naming another, stops after four SeekHeads). Anything unexpected
// ends in ErrNoIndex, never in a panic.
package mediaindex

import (
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"time"
)

// Source names where an index came from; the values are those of media_files.keyframes_source.
type Source string

// Sources of Read.
const (
	SourceCues Source = "cues" // Matroska and WebM cues
	SourceMP4  Source = "mp4"  // MP4 sample tables (stss, stts, ctts, elst)
)

// Index is the keyframe index of the first video stream of a file.
type Index struct {
	Keyframes []time.Duration // sorted, unique, rounded to ms as in the stored blob
	Source    Source
	Track     uint64 // track number (Matroska) or track ID (MP4) of the video stream used
	Stats     Stats  // bytes read, reads, cue points of all tracks; for logs and docs/perf
}

// Stats describes the work of one Read.
type Stats struct {
	BytesRead int64 // bytes returned by the reader
	Reads     int   // ReadAt calls
	CuePoints int   // cue points of all tracks (Matroska only)
}

// ErrNoIndex: the container carries no usable index (no cues, no moov, fragmented MP4, other
// container); the caller falls back to the full read. Read wraps it with the reason.
var ErrNoIndex = errors.New("mediaindex: no index in container")

// maxEntries bounds the keyframes of one index and the cue points of one Cues element.
const maxEntries = 1_000_000

// Read returns the keyframe index the container of the file carries. size is the file size,
// container the value of media_files.container (mkv, webm, mp4, m4v, mov); any other container
// returns ErrNoIndex. Every error wraps ErrNoIndex.
func Read(r io.ReaderAt, size int64, container string) (Index, error) {
	if r == nil || size <= 0 {
		return Index{}, fmt.Errorf("%w: empty file", ErrNoIndex)
	}
	rd := &reader{r: r, size: size}
	var ix Index
	var err error
	switch strings.ToLower(container) {
	case "mkv", "webm":
		ix, err = readMatroska(rd)
	case "mp4", "m4v", "mov":
		ix, err = readMP4(rd)
	default:
		return Index{}, fmt.Errorf("%w: container %q", ErrNoIndex, container)
	}
	ix.Stats.BytesRead, ix.Stats.Reads = rd.bytes, rd.reads
	if err != nil {
		return Index{Stats: ix.Stats}, err
	}
	if len(ix.Keyframes) == 0 {
		return Index{Stats: ix.Stats}, fmt.Errorf("%w: no keyframe of the video track", ErrNoIndex)
	}
	if len(ix.Keyframes) > maxEntries {
		return Index{Stats: ix.Stats}, fmt.Errorf("%w: more than %d keyframes", ErrNoIndex, maxEntries)
	}
	slices.Sort(ix.Keyframes)
	ix.Keyframes = slices.Compact(ix.Keyframes)
	return ix, nil
}

// reader reads bounded ranges of the file and counts the work.
type reader struct {
	r     io.ReaderAt
	size  int64
	bytes int64
	reads int
}

// read returns n bytes at off. n above limit, or a range outside the file, is ErrNoIndex before
// anything is allocated.
func (rd *reader) read(off, n, limit int64) ([]byte, error) {
	if n > limit {
		return nil, fmt.Errorf("%w: %d bytes at %d exceed the limit of %d", ErrNoIndex, n, off, limit)
	}
	if off < 0 || n < 0 || off > rd.size || n > rd.size-off {
		return nil, fmt.Errorf("%w: range %d+%d outside the file of %d bytes", ErrNoIndex, off, n, rd.size)
	}
	b := make([]byte, n)
	rd.reads++
	m, err := rd.r.ReadAt(b, off)
	rd.bytes += int64(m)
	if m < len(b) {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return nil, fmt.Errorf("%w: read at %d: %w", ErrNoIndex, off, err)
	}
	return b, nil
}

// Thresholds of Check (concept 14, section 6.2), with distance to the measured maxima.
const (
	maxInnerGap  = 30 * time.Second // measured maximum 10.68 s
	maxFirst     = 5 * time.Second  // measured maximum 0.083 s
	minEndGapCap = 60 * time.Second // measured maximum end gap 42.3 s
	perEntry     = time.Minute      // at least one entry per minute of duration
	lastSlack    = time.Second      // the last entry may lie this far behind the duration
)

// Check applies the plausibility rules an index from the container must pass before it is
// stored (concept 14, section 6.2). duration is the probed duration of the file.
func Check(ix Index, duration time.Duration) error {
	kf := ix.Keyframes
	if duration <= 0 {
		return errors.New("mediaindex: duration unknown")
	}
	if len(kf) < 2 {
		return fmt.Errorf("mediaindex: %d keyframes, want at least 2", len(kf))
	}
	if int64(len(kf))*int64(perEntry) < int64(duration) { // len <= maxEntries: no overflow
		return fmt.Errorf("mediaindex: %d keyframes for %v, want at least one per minute", len(kf), duration)
	}
	if kf[0] < 0 {
		return fmt.Errorf("mediaindex: negative keyframe %v", kf[0])
	}
	if kf[0] > maxFirst {
		return fmt.Errorf("mediaindex: first keyframe at %v, want at most %v", kf[0], maxFirst)
	}
	var gap time.Duration
	for i := 1; i < len(kf); i++ {
		if kf[i] <= kf[i-1] {
			return fmt.Errorf("mediaindex: keyframe %d at %v not after %v", i, kf[i], kf[i-1])
		}
		gap = max(gap, kf[i]-kf[i-1])
	}
	if gap > maxInnerGap {
		return fmt.Errorf("mediaindex: gap of %v between keyframes, want at most %v", gap, maxInnerGap)
	}
	last := kf[len(kf)-1]
	if last > duration+lastSlack {
		return fmt.Errorf("mediaindex: last keyframe at %v behind the duration %v", last, duration)
	}
	if end, limit := duration-last, max(minEndGapCap, 2*gap); end > limit {
		return fmt.Errorf("mediaindex: end gap of %v, want at most %v", end, limit)
	}
	return nil
}

// toDuration converts x ticks of a timescale to a time.Duration without overflow: ok is false
// when the result does not fit.
func toDuration(x int64, scale uint64) (time.Duration, bool) {
	if scale == 0 || scale > math.MaxUint32 {
		return 0, false
	}
	s := int64(scale)
	q, r := x/s, x%s
	if q > math.MaxInt64/int64(time.Second) || q < math.MinInt64/int64(time.Second) {
		return 0, false
	}
	// |r| < s <= 2^32, so r*1e9 fits; q*1e9 fits by the check above.
	return add(time.Duration(q*int64(time.Second)), time.Duration(r*int64(time.Second)/s))
}

// add returns a+b; ok is false on overflow.
func add[T ~int64](a, b T) (T, bool) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, false
	}
	return a + b, true
}
