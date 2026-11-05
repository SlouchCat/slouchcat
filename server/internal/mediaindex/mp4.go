package mediaindex

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"time"
)

// Read limits for MP4 (concept 14, sections 6.1 and 9.1).
const (
	mp4MoovLimit = 64 << 20 // moov in one read; the largest measured ctts alone was 1.8 MB
	mp4TopLevel  = 64       // top-level boxes walked to find moov
	mp4Header    = 16       // size, type and 64-bit size
)

var errBox = fmt.Errorf("%w: corrupt box", ErrNoIndex)

// readMP4 walks the top-level boxes to moov (at the start with faststart, else behind mdat) and
// computes the presentation times of the sync samples of the first video track.
func readMP4(rd *reader) (Index, error) {
	var pos int64
	for range mp4TopLevel {
		if pos > rd.size-8 {
			break
		}
		h, err := rd.read(pos, min(mp4Header, rd.size-pos), mp4Header)
		if err != nil {
			return Index{}, err
		}
		typ, hl, size, ok := boxHeader(h, rd.size-pos)
		if !ok {
			return Index{}, errBox
		}
		switch typ {
		case "moov":
			moov, err := rd.read(pos+hl, size-hl, mp4MoovLimit)
			if err != nil {
				return Index{}, err
			}
			return parseMoov(moov)
		case "moof":
			return Index{}, fmt.Errorf("%w: fragmented MP4", ErrNoIndex)
		}
		pos += size // size <= rd.size-pos
	}
	return Index{}, fmt.Errorf("%w: no moov", ErrNoIndex)
}

// trak holds the boxes of a video track that the times need; elst, ctts and stss may be nil.
type trak struct {
	mdhd, elst, stts, ctts, stss []byte
	mediaScale, movieScale       uint64
}

func parseMoov(moov []byte) (Index, error) {
	var movieScale uint64
	var video []byte
	err := eachBox(moov, func(typ string, body []byte) error {
		switch typ {
		case "mvhd":
			movieScale = fullBoxScale(body)
		case "mvex":
			return fmt.Errorf("%w: fragmented MP4", ErrNoIndex)
		case "trak":
			if video == nil {
				hdlr, err := find(body, "mdia", "hdlr")
				if err != nil {
					return err
				}
				if len(hdlr) >= 12 && string(hdlr[8:12]) == "vide" {
					video = body
				}
			}
		}
		return nil
	})
	if err != nil {
		return Index{}, err
	}
	if video == nil {
		return Index{}, fmt.Errorf("%w: no video track", ErrNoIndex)
	}
	tkhd, err := find(video, "tkhd")
	if err != nil {
		return Index{}, err
	}
	c := cursor{b: tkhd}
	if c.u8() == 1 {
		c.skip(3 + 16)
	} else {
		c.skip(3 + 8)
	}
	id := c.u32()
	if c.bad {
		return Index{}, errBox
	}
	get := func(b []byte, path ...string) []byte {
		v, e := find(b, path...)
		if err == nil {
			err = e
		}
		return v
	}
	stbl := get(video, "mdia", "minf", "stbl")
	t := trak{
		mdhd: get(video, "mdia", "mdhd"), elst: get(video, "edts", "elst"),
		stts: get(stbl, "stts"), ctts: get(stbl, "ctts"), stss: get(stbl, "stss"),
		movieScale: movieScale,
	}
	if err != nil {
		return Index{}, err
	}
	if t.mdhd == nil || t.stts == nil {
		return Index{}, fmt.Errorf("%w: video track without mdhd or stts", ErrNoIndex)
	}
	if t.mediaScale = fullBoxScale(t.mdhd); t.mediaScale == 0 {
		return Index{}, fmt.Errorf("%w: media timescale 0", ErrNoIndex)
	}
	kf, err := t.keyframes()
	return Index{Keyframes: kf, Source: SourceMP4, Track: uint64(id)}, err
}

// run is one entry of stts or ctts: count samples with the same value. Both fields come from
// 32-bit fields, so they fit an int64 with room.
type run struct {
	count int64
	value int64
}

// keyframes computes t = (DTS(n) + ctts(n) − media_time) / mdhd.timescale + empty /
// mvhd.timescale for every sync sample n, rounded to ms (concept 14, section 6.1).
func (t *trak) keyframes() ([]time.Duration, error) {
	offset, err := t.editOffset()
	if err != nil {
		return nil, err
	}
	stts, err := runs(t.stts, false)
	if err != nil {
		return nil, err
	}
	var ctts []run
	if t.ctts != nil {
		// ffmpeg reads the offsets as signed in both versions; so do we.
		if ctts, err = runs(t.ctts, true); err != nil {
			return nil, err
		}
	}
	var total int64
	for _, r := range stts {
		total += r.count // at most 2^29 entries (moov limit) of less than 2^32 each: no overflow
	}
	sync, err := t.syncSamples(total)
	if err != nil {
		return nil, err
	}
	out := make([]time.Duration, 0, len(sync))
	dts := &runCursor{runs: stts, sum: true}
	cts := &runCursor{runs: ctts}
	for _, n := range sync {
		x, ok := dts.at(n)
		if ok && t.ctts != nil {
			var c int64
			if c, ok = cts.at(n); ok {
				x, ok = add(x, c)
			}
		}
		var ts time.Duration
		if ok {
			ts, ok = toDuration(x, t.mediaScale)
		}
		if ok {
			ts, ok = add(ts, offset)
		}
		if !ok {
			return nil, fmt.Errorf("%w: time of sample %d outside the tables or overflowing", ErrNoIndex, n)
		}
		out = append(out, ts.Round(time.Millisecond))
	}
	return out, nil
}

// editOffset returns the shift of the edit list: the empty edits before the first non-empty edit
// (media_time −1, movie timescale) minus its media_time (media timescale). More than one
// non-empty edit is ErrNoIndex.
func (t *trak) editOffset() (time.Duration, error) {
	if t.elst == nil {
		return 0, nil
	}
	c := cursor{b: t.elst}
	v := c.u8()
	c.skip(3)
	n := uint64(c.u32())
	size := uint64(12)
	if v == 1 {
		size = 20
	}
	if c.bad || n > uint64(len(c.b))/size {
		return 0, errBox
	}
	var empty, mediaTime int64
	nonEmpty := 0
	for range n {
		var dur uint64
		var mt int64
		if v == 1 {
			dur, mt = c.u64(), int64(c.u64()) //nolint:gosec // G115: media_time is signed
		} else {
			dur, mt = uint64(c.u32()), int64(int32(c.u32())) //nolint:gosec // G115: media_time is signed
		}
		c.skip(4) // media_rate
		switch {
		case mt == -1 && nonEmpty == 0:
			if dur > math.MaxInt64 {
				return 0, errBox
			}
			var ok bool
			if empty, ok = add(empty, int64(dur)); !ok {
				return 0, errBox
			}
		case mt >= 0:
			if nonEmpty++; nonEmpty > 1 {
				return 0, fmt.Errorf("%w: edit list with more than one edit", ErrNoIndex)
			}
			mediaTime = mt
		}
	}
	shift, ok := toDuration(-mediaTime, t.mediaScale)
	if !ok {
		return 0, errBox
	}
	if empty > 0 {
		e, ok := toDuration(empty, t.movieScale)
		if !ok {
			return 0, fmt.Errorf("%w: empty edit without movie timescale", ErrNoIndex)
		}
		if shift, ok = add(shift, e); !ok {
			return 0, errBox
		}
	}
	return shift, nil
}

// syncSamples returns the sync samples (0-based, sorted): from stss, or all samples without it
// (Apple QTFF).
func (t *trak) syncSamples(total int64) ([]int64, error) {
	if t.stss == nil {
		if total > maxEntries {
			return nil, fmt.Errorf("%w: %d samples without stss", ErrNoIndex, total)
		}
		out := make([]int64, total)
		for i := range out {
			out[i] = int64(i)
		}
		return out, nil
	}
	c := cursor{b: t.stss}
	c.skip(4)
	n := uint64(c.u32())
	if c.bad || n > uint64(len(c.b))/4 {
		return nil, errBox
	}
	if n > maxEntries {
		return nil, fmt.Errorf("%w: %d sync samples", ErrNoIndex, n)
	}
	out := make([]int64, n)
	for i := range out {
		s := int64(c.u32())
		if s == 0 || s > total {
			return nil, fmt.Errorf("%w: sync sample %d of %d", ErrNoIndex, s, total)
		}
		out[i] = s - 1
	}
	slices.Sort(out)
	return out, nil
}

// runs reads the entries of stts or ctts; signed reads the values as int32.
func runs(b []byte, signed bool) ([]run, error) {
	c := cursor{b: b}
	c.skip(4)
	n := uint64(c.u32())
	if c.bad || n > uint64(len(c.b))/8 {
		return nil, errBox
	}
	out := make([]run, n)
	for i := range out {
		out[i].count = int64(c.u32())
		if v := c.u32(); signed {
			out[i].value = int64(int32(v)) //nolint:gosec // G115: the offset is signed
		} else {
			out[i].value = int64(v)
		}
	}
	return out, nil
}

// runCursor finds the value (ctts) or the running sum (stts: the DTS) of ascending samples.
type runCursor struct {
	runs  []run
	sum   bool
	i     int   // current run
	first int64 // first sample of run i
	base  int64 // DTS at the first sample of run i
}

func (c *runCursor) at(n int64) (int64, bool) {
	for c.i < len(c.runs) && n >= c.first+c.runs[c.i].count {
		r := c.runs[c.i]
		if c.sum {
			step, ok := mulNonNeg(r.count, r.value)
			if ok {
				c.base, ok = add(c.base, step)
			}
			if !ok {
				return 0, false
			}
		}
		c.first += r.count
		c.i++
	}
	if c.i == len(c.runs) {
		return 0, false
	}
	if !c.sum {
		return c.runs[c.i].value, true
	}
	d, ok := mulNonNeg(n-c.first, c.runs[c.i].value)
	if !ok {
		return 0, false
	}
	return add(c.base, d)
}

// mulNonNeg returns a*b for a, b >= 0; ok is false on overflow.
func mulNonNeg(a, b int64) (int64, bool) {
	hi, lo := bits.Mul64(uint64(a), uint64(b)) //nolint:gosec // G115: a, b >= 0
	if hi != 0 || lo > math.MaxInt64 {
		return 0, false
	}
	return int64(lo), true
}

// fullBoxScale reads the timescale of mvhd or mdhd (version 0 or 1); 0 when the box is short.
func fullBoxScale(b []byte) uint64 {
	c := cursor{b: b}
	if c.u8() == 1 {
		c.skip(3 + 16)
	} else {
		c.skip(3 + 8)
	}
	s := uint64(c.u32())
	if c.bad {
		return 0
	}
	return s
}

// find returns the body of the first box along path below b; nil when a box is missing.
func find(b []byte, path ...string) ([]byte, error) {
	for _, name := range path {
		var next []byte
		err := eachBox(b, func(typ string, body []byte) error {
			if next == nil && typ == name {
				next = body
			}
			return nil
		})
		if err != nil || next == nil {
			return nil, err
		}
		b = next
	}
	return b, nil
}

// eachBox calls fn for every box in b. A box that does not fit makes the parent corrupt.
func eachBox(b []byte, fn func(typ string, body []byte) error) error {
	for len(b) > 0 {
		typ, hl, size, ok := boxHeader(b, int64(len(b)))
		if !ok {
			return errBox
		}
		if err := fn(typ, b[hl:size]); err != nil {
			return err
		}
		b = b[size:]
	}
	return nil
}

// boxHeader parses a box header: size 1 means a 64-bit size follows, size 0 means up to the end
// (rest bytes). ok is false when the box is shorter than its header or longer than rest.
func boxHeader(b []byte, rest int64) (typ string, hl, size int64, ok bool) {
	if len(b) < 8 {
		return "", 0, 0, false
	}
	size, hl, typ = int64(binary.BigEndian.Uint32(b)), 8, string(b[4:8])
	switch size {
	case 0:
		size = rest
	case 1:
		if len(b) < 16 {
			return "", 0, 0, false
		}
		big := binary.BigEndian.Uint64(b[8:])
		if big > math.MaxInt64 {
			return "", 0, 0, false
		}
		size, hl = int64(big), 16
	}
	return typ, hl, size, size >= hl && size <= rest
}

// cursor reads big-endian fields; a read past the end sets bad and returns 0.
type cursor struct {
	b   []byte
	bad bool
}

func (c *cursor) take(n int) []byte {
	if c.bad || len(c.b) < n {
		c.bad = true
		return nil
	}
	v := c.b[:n]
	c.b = c.b[n:]
	return v
}

func (c *cursor) skip(n int) { c.take(n) }

func (c *cursor) u8() byte {
	if v := c.take(1); v != nil {
		return v[0]
	}
	return 0
}

func (c *cursor) u32() uint32 {
	if v := c.take(4); v != nil {
		return binary.BigEndian.Uint32(v)
	}
	return 0
}

func (c *cursor) u64() uint64 {
	if v := c.take(8); v != nil {
		return binary.BigEndian.Uint64(v)
	}
	return 0
}
