package transcode

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// The split layout (delayedInit, concept 26 stage 1, ADR 0041): with AC-3 or E-AC-3 the mp4 muxer
// runs with +delay_moov+frag_keyframe. It writes the moov at its first flush, after the first
// audio packet, and a fragment at every video keyframe besides the segment muxer's flushes. So the
// files of a run no longer map one to one onto segments:
//
//   - the first file is ftyp+moov alone when the run's first segment has one keyframe; that
//     segment's fragment then opens the second file, before the second segment's;
//   - a segment with several keyframes (remux) has one fragment per keyframe.
//
// The session therefore reads the files of such a run as one stream of boxes and assigns each
// fragment to the segment its first video sample lies in (presentation time on the shifted
// timeline, with the cut tolerance of the segment muxer). Segment k is complete
//
//   - when file k is closed and read: the segment muxer flushes the fragment of segment k into file
//     k before it closes it and opens file k+1 (file k+1 exists, or ffmpeg has exited);
//   - at once when its fragment lies in a later file: that is the fragment the delayed moov held
//     back, and the muxer writes everything it held in one flush;
//   - when a fragment of a later segment follows it.
//
// A file is read only up to its last complete box.

// fileSpan is a byte range of one file of a run.
type fileSpan struct {
	file      int
	off, size int64
}

// splitState is the reading position of a run in the split layout.
type splitState struct {
	file   int   // file being read (segment file number)
	off    int64 // offset of its next box
	low    int   // lowest file not removed yet
	scales map[uint32]uint32
	video  uint32     // track id of the video track, 0 none
	seg    int        // segment the pending fragments belong to, -1 none
	parts  []fileSpan // the pending fragments
}

func newSplitState(start int) *splitState {
	return &splitState{file: start, low: start, seg: -1}
}

// splitEvent is what one step of the reader found.
type splitEvent struct {
	init    []byte // ftyp+moov of the run
	segment int    // a complete segment (parts), -1 none
	parts   []fileSpan
}

// errSplitWait means the reader needs more data on disk.
var errSplitWait = errors.New("split layout: waiting for data")

// next reads boxes until it finds the init or a complete segment, and returns it; errSplitWait
// when it must wait for ffmpeg. final means ffmpeg has exited (exitOK without an error): every file
// is complete, and the pending segment is complete when exitOK holds.
func (st *splitState) next(dir string, p Plan, cut time.Duration, final, exitOK bool) (splitEvent, error) {
	for {
		path := filepath.Join(dir, fmt.Sprintf(segmentPattern, st.file))
		f, err := os.Open(path) //nolint:gosec // G304: path built by the session from its own run dir
		if errors.Is(err, fs.ErrNotExist) {
			if final && exitOK && st.seg >= 0 {
				return st.flush(), nil
			}
			return splitEvent{segment: -1}, errSplitWait
		}
		if err != nil {
			return splitEvent{segment: -1}, err
		}
		ev, err := st.step(f, dir, p, cut, final && exitOK)
		_ = f.Close()
		if err != nil || ev.init != nil || ev.segment >= 0 {
			return ev, err
		}
	}
}

// step reads the next box of the open file f (file st.file); a zero event with a nil error means
// the reader moved on and the caller loops. done means ffmpeg exited without an error: the last
// file is complete too. After an error exit the last file stays open, so its pending segment is
// never published half.
func (st *splitState) step(f *os.File, dir string, p Plan, cut time.Duration, done bool) (splitEvent, error) {
	none := splitEvent{segment: -1}
	// Whether the file is closed, before its size: the segment muxer closes file k before it
	// creates k+1, so a size read after k+1 exists is final.
	closed := done || exists(filepath.Join(dir, fmt.Sprintf(segmentPattern, st.file+1)))
	fi, err := f.Stat()
	if err != nil {
		return none, err
	}
	size := fi.Size()
	if st.off >= size {
		if !closed {
			return none, errSplitWait
		}
		ev := none
		if st.seg >= 0 && st.seg <= st.file {
			ev = st.flush() // file k is closed: segment k is complete
		}
		st.file, st.off = st.file+1, 0
		return ev, nil
	}
	typ, hdr, bsize, err := boxAt(f, st.off, size)
	if errors.Is(err, errSplitWait) && closed {
		return none, fmt.Errorf("segment file %d ends in a partial box at %d", st.file, st.off)
	}
	if err != nil {
		return none, err
	}
	switch typ {
	case "moov":
		moov, err := readAt(f, st.off+hdr, bsize-hdr)
		if err != nil {
			return none, err
		}
		if st.scales, st.video, err = parseMoov(moov); err != nil {
			return none, err
		}
		init, err := readAt(f, 0, st.off+bsize)
		if err != nil {
			return none, err
		}
		st.off += bsize
		return splitEvent{init: init, segment: -1}, nil
	case "moof":
		if st.scales == nil {
			return none, fmt.Errorf("segment file %d: moof before moov", st.file)
		}
		mtyp, _, msize, err := boxAt(f, st.off+bsize, size)
		if errors.Is(err, errSplitWait) && closed {
			return none, fmt.Errorf("segment file %d ends in a partial fragment at %d", st.file, st.off)
		}
		if err != nil {
			return none, err
		}
		if mtyp != "mdat" {
			return none, fmt.Errorf("segment file %d: moof followed by %q", st.file, mtyp)
		}
		moof, err := readAt(f, st.off+hdr, bsize-hdr)
		if err != nil {
			return none, err
		}
		pts, err := moofTime(moof, st.scales, st.video)
		if err != nil {
			return none, fmt.Errorf("segment file %d: %w", st.file, err)
		}
		j := segmentOf(p, pts-timelineOffset, cut)
		var ev splitEvent
		if st.seg >= 0 && j > st.seg {
			ev = st.flush()
		} else {
			ev = none
		}
		if st.seg < 0 || j > st.seg {
			st.seg = j
		}
		st.parts = append(st.parts, fileSpan{file: st.file, off: st.off, size: bsize + msize})
		st.off += bsize + msize
		if ev.segment < 0 && st.file > st.seg {
			ev = st.flush() // the held-back fragment: the whole segment
		}
		return ev, nil
	default: // ftyp, and boxes the muxer may add (free, sidx)
		st.off += bsize
		return none, nil
	}
}

// flush hands out the pending segment.
func (st *splitState) flush() splitEvent {
	ev := splitEvent{segment: st.seg, parts: st.parts}
	st.seg, st.parts = -1, nil
	return ev
}

// cleanup removes the files of the run that are read and published completely.
func (st *splitState) cleanup(dir string) {
	keep := st.file
	if len(st.parts) > 0 {
		keep = min(keep, st.parts[0].file)
	}
	for ; st.low < keep; st.low++ {
		_ = os.Remove(filepath.Join(dir, fmt.Sprintf(segmentPattern, st.low)))
	}
}

// writeParts writes the byte ranges of a segment from the run directory to path atomically.
func writeParts(dir, path string, parts []fileSpan) error {
	tmp := path + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600) //nolint:gosec // G304: session work dir
	if err != nil {
		return err
	}
	for _, sp := range parts {
		in, err := os.Open(filepath.Join(dir, fmt.Sprintf(segmentPattern, sp.file))) //nolint:gosec // G304: run dir
		if err != nil {
			_ = out.Close()
			return err
		}
		_, err = io.Copy(out, io.NewSectionReader(in, sp.off, sp.size))
		_ = in.Close()
		if err != nil {
			_ = out.Close()
			return err
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// segmentOf is the segment of the plan whose start, less the cut tolerance of the segment muxer,
// is the last at or before t (source time); 0 before the first.
func segmentOf(p Plan, t, cut time.Duration) int {
	j := 0
	for i, s := range p.Segments {
		if s.Start-cut <= t {
			j = i
		}
	}
	return j
}

// boxAt reads the header of the top-level box at off of a file of size bytes: its type, the length
// of its header (8, or 16 with a 64-bit size) and its size; errSplitWait when the header or the
// box is not completely on disk yet.
func boxAt(f *os.File, off, size int64) (string, int64, int64, error) {
	if size-off < 8 {
		return "", 0, 0, errSplitWait
	}
	buf := make([]byte, 16)
	n := min(size-off, 16)
	if _, err := f.ReadAt(buf[:n], off); err != nil {
		return "", 0, 0, err
	}
	bsize, hdr := int64(binary.BigEndian.Uint32(buf)), int64(8)
	typ := string(buf[4:8])
	switch bsize {
	case 0: // to the end of the file: the size is not patched yet
		return "", 0, 0, errSplitWait
	case 1:
		if n < 16 {
			return "", 0, 0, errSplitWait
		}
		bsize, hdr = int64(binary.BigEndian.Uint64(buf[8:])), 16 //nolint:gosec // G115: compared with the file size below
	}
	if bsize < hdr {
		return "", 0, 0, fmt.Errorf("box %q at %d has invalid size %d", typ, off, bsize)
	}
	if bsize > size-off {
		return "", 0, 0, errSplitWait
	}
	return typ, hdr, bsize, nil
}

func readAt(f *os.File, off, n int64) ([]byte, error) {
	b := make([]byte, n)
	if _, err := f.ReadAt(b, off); err != nil {
		return nil, err
	}
	return b, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// childBoxes returns the payloads of the child boxes of b by type (the first of each type), and
// all of them in order.
func childBoxes(b []byte) (map[string][]byte, []box, error) {
	boxes, err := walkBoxes(b)
	if err != nil {
		return nil, nil, err
	}
	m := map[string][]byte{}
	for _, bx := range boxes {
		if _, ok := m[bx.Type]; !ok {
			m[bx.Type] = payloadOf(b, bx)
		}
	}
	return m, boxes, nil
}

// payloadOf is the payload of the box bx of b, after its 8- or 16-byte header (walkBoxes
// checked both against the size).
func payloadOf(b []byte, bx box) []byte {
	if binary.BigEndian.Uint32(b[bx.Offset:]) == 1 {
		if bx.Size < 16 {
			return nil
		}
		return b[bx.Offset+16 : bx.Offset+bx.Size]
	}
	return b[bx.Offset+8 : bx.Offset+bx.Size]
}

// parseMoov reads the timescale of every track of a moov payload (tkhd track id, mdhd timescale)
// and the id of the video track (hdlr vide).
func parseMoov(moov []byte) (map[uint32]uint32, uint32, error) {
	_, boxes, err := childBoxes(moov)
	if err != nil {
		return nil, 0, err
	}
	scales := map[uint32]uint32{}
	var video uint32
	for _, bx := range boxes {
		if bx.Type != "trak" {
			continue
		}
		trak, _, err := childBoxes(payloadOf(moov, bx))
		if err != nil {
			return nil, 0, err
		}
		tkhd := trak["tkhd"]
		mdia, _, err := childBoxes(trak["mdia"])
		if err != nil {
			return nil, 0, err
		}
		mdhd, hdlr := mdia["mdhd"], mdia["hdlr"]
		id, idOK := fullBoxField(tkhd, 12, 20)
		ts, tsOK := fullBoxField(mdhd, 12, 20)
		if !idOK || !tsOK || ts == 0 || len(hdlr) < 12 {
			return nil, 0, errors.New("moov: track without tkhd, mdhd or hdlr")
		}
		scales[id] = ts
		if string(hdlr[8:12]) == "vide" && video == 0 {
			video = id
		}
	}
	if len(scales) == 0 {
		return nil, 0, errors.New("moov without tracks")
	}
	return scales, video, nil
}

// fullBoxField reads the 32-bit field of a full box (tkhd, mdhd) at off0 in version 0 and off1 in
// version 1.
func fullBoxField(b []byte, off0, off1 int) (uint32, bool) {
	off := off0
	if len(b) > 0 && b[0] == 1 {
		off = off1
	}
	if len(b) < off+4 {
		return 0, false
	}
	return binary.BigEndian.Uint32(b[off:]), true
}

// moofTime is the presentation time of the first sample of the video track of a moof payload
// (tfdt plus the first sample's composition offset), else of its first track, on the output
// timeline.
func moofTime(moof []byte, scales map[uint32]uint32, video uint32) (time.Duration, error) {
	_, boxes, err := childBoxes(moof)
	if err != nil {
		return 0, err
	}
	var first *time.Duration
	for _, bx := range boxes {
		if bx.Type != "traf" {
			continue
		}
		id, t, err := trafTime(payloadOf(moof, bx), scales)
		if err != nil {
			return 0, err
		}
		if id == video {
			return t, nil
		}
		if first == nil {
			first = &t
		}
	}
	if first == nil {
		return 0, errors.New("moof without traf")
	}
	return *first, nil
}

// trafTime returns the track id and the presentation time of the first sample of a traf payload.
func trafTime(traf []byte, scales map[uint32]uint32) (uint32, time.Duration, error) {
	c, _, err := childBoxes(traf)
	if err != nil {
		return 0, 0, err
	}
	tfhd, tfdt, trun := c["tfhd"], c["tfdt"], c["trun"]
	if len(tfhd) < 8 || len(tfdt) < 8 {
		return 0, 0, errors.New("traf without tfhd or tfdt")
	}
	id := binary.BigEndian.Uint32(tfhd[4:])
	ts := scales[id]
	if ts == 0 {
		return 0, 0, fmt.Errorf("traf of unknown track %d", id)
	}
	var base int64
	if tfdt[0] == 1 {
		if len(tfdt) < 12 {
			return 0, 0, errors.New("short tfdt")
		}
		base = int64(binary.BigEndian.Uint64(tfdt[4:])) //nolint:gosec // G115: decode times stay far below 2^63
	} else {
		base = int64(binary.BigEndian.Uint32(tfdt[4:]))
	}
	base += firstCompositionOffset(trun)
	return id, time.Duration(float64(base) / float64(ts) * float64(time.Second)), nil
}

// firstCompositionOffset is the composition time offset of the first sample of a trun payload,
// 0 when it has none.
func firstCompositionOffset(trun []byte) int64 {
	if len(trun) < 8 {
		return 0
	}
	flags := binary.BigEndian.Uint32(trun) & 0xffffff
	p := 8 // version and flags, sample count
	if flags&0x1 != 0 {
		p += 4 // data offset
	}
	if flags&0x4 != 0 {
		p += 4 // first sample flags
	}
	for _, bit := range []uint32{0x100, 0x200, 0x400} { // duration, size, flags of sample 1
		if flags&bit != 0 {
			p += 4
		}
	}
	if flags&0x800 == 0 || len(trun) < p+4 {
		return 0
	}
	v := binary.BigEndian.Uint32(trun[p:])
	if trun[0] == 1 {
		return int64(int32(v)) //nolint:gosec // G115: version 1 offsets are signed
	}
	return int64(v)
}
