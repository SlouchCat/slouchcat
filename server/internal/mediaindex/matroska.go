package mediaindex

import (
	"bytes"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"time"
)

// Matroska element IDs (RFC 9559), with their length marker.
const (
	idEBML              = 0x1A45DFA3
	idSegment           = 0x18538067
	idSeekHead          = 0x114D9B74
	idSeek              = 0x4DBB
	idSeekID            = 0x53AB
	idSeekPosition      = 0x53AC
	idInfo              = 0x1549A966
	idTimestampScale    = 0x2AD7B1
	idTracks            = 0x1654AE6B
	idTrackEntry        = 0xAE
	idTrackNumber       = 0xD7
	idTrackType         = 0x83
	idCues              = 0x1C53BB6B
	idCuePoint          = 0xBB
	idCueTime           = 0xB3
	idCueTrackPositions = 0xB7
	idCueTrack          = 0xF7
	idCluster           = 0x1F43B675
)

// Read limits for Matroska (concept 14, sections 6.1 and 9.1).
const (
	mkvHead        = 64 << 10 // first read: EBML header, segment start, SeekHead, usually Info and Tracks
	mkvMasterLimit = 1 << 20  // Info, Tracks, one SeekHead
	mkvCuesLimit   = 8 << 20  // Cues in one read; the largest measured was 2.03 MB
	mkvTail        = 1 << 20  // searched for the Cues ID when no SeekHead names them
	mkvTopLevel    = 64       // top-level elements walked before the first Cluster
	mkvSeekHeads   = 4        // SeekHead positions tried (mkvmerge writes a second one at the end)
	mkvCandidates  = 5        // positions kept per element: one per SeekHead and one from the walk
	mkvTailTries   = 16       // Cues ID matches tried in the tail
	trackTypeVideo = 1
	mkvHeaderMax   = 12 // ID (up to 4 bytes) and size (up to 8 bytes)
)

var errCorrupt = fmt.Errorf("%w: corrupt element", ErrNoIndex)

// element is the header of a top-level element in the file.
type element struct {
	id      uint32
	data    int64 // file offset of the data
	size    int64
	unknown bool // size unknown (live streams); size is then 0
}

type matroska struct {
	rd        *reader
	head      []byte // file bytes [0, len(head))
	segStart  int64  // seek positions are relative to it (RFC 9559, section 16)
	segEnd    int64
	found     map[uint32][]int64 // file offsets of Info, Tracks, Cues, in the order found
	seekHeads map[int64]bool     // SeekHead positions already tried
}

func readMatroska(rd *reader) (Index, error) {
	head, err := rd.read(0, min(rd.size, mkvHead), mkvHead)
	if err != nil {
		return Index{}, err
	}
	m := &matroska{rd: rd, head: head, found: map[uint32][]int64{}, seekHeads: map[int64]bool{}}
	ebml, err := m.element(0)
	if err != nil || ebml.id != idEBML || ebml.unknown {
		return Index{}, fmt.Errorf("%w: no EBML header", ErrNoIndex)
	}
	seg, err := m.element(ebml.data + ebml.size)
	if err != nil || seg.id != idSegment {
		return Index{}, fmt.Errorf("%w: no segment", ErrNoIndex)
	}
	if seg.unknown {
		return Index{}, fmt.Errorf("%w: segment of unknown size", ErrNoIndex)
	}
	m.segStart, m.segEnd = seg.data, min(seg.data+seg.size, rd.size)
	m.locate()
	scale, err := m.timestampScale()
	if err != nil {
		return Index{}, err
	}
	track, err := m.videoTrack()
	if err != nil {
		return Index{}, err
	}
	ix := Index{Source: SourceCues, Track: track}
	ix.Keyframes, ix.Stats.CuePoints, err = m.cues(track, scale)
	return ix, err
}

// locate notes the candidate positions of Info, Tracks and Cues: those the SeekHeads name (a
// SeekHead may name a second one) and those of a walk over the top-level elements up to the first
// Cluster. A SeekHead entry may be stale, so the walk goes on when the SeekHeads named all three;
// master and cues try the candidates in order, cues then searches the last MiB. An element whose
// header does not parse ends the walk, a SeekHead that does not parse is skipped: what was found
// so far still counts.
func (m *matroska) locate() {
	pos := m.segStart
	for range mkvTopLevel {
		if pos >= m.segEnd {
			return
		}
		el, err := m.element(pos)
		if err != nil {
			return
		}
		switch el.id {
		case idCluster:
			return
		case idSeekHead:
			if m.try(pos) {
				m.seekHead(el)
			}
		case idInfo, idTracks, idCues:
			m.note(el.id, pos)
		}
		if el.unknown {
			return
		}
		pos = el.data + el.size
	}
}

// note adds pos to the candidates of id, at most mkvCandidates distinct ones.
func (m *matroska) note(id uint32, pos int64) {
	if c := m.found[id]; len(c) < mkvCandidates && !slices.Contains(c, pos) {
		m.found[id] = append(c, pos)
	}
}

// try reports whether the SeekHead at pos may be read and counts it: every position tried, read
// or not, uses up one of mkvSeekHeads, so a SeekHead naming thousands of stale SeekHeads costs at
// most mkvSeekHeads reads.
func (m *matroska) try(pos int64) bool {
	if m.seekHeads[pos] || len(m.seekHeads) >= mkvSeekHeads {
		return false
	}
	m.seekHeads[pos] = true
	return true
}

// seekHead notes the positions a SeekHead names and follows the SeekHeads it names. A SeekHead
// that cannot be read ends here; the entries before a corrupt one still count (master and cues
// check what they find at a position).
func (m *matroska) seekHead(el element) {
	data, err := m.data(el, mkvMasterLimit)
	if err != nil {
		return
	}
	var next []int64
	_ = eachChild(data, func(id uint32, seek []byte) error {
		if id != idSeek {
			return nil
		}
		var target uint32
		var at uint64
		var haveAt bool
		err := eachChild(seek, func(id uint32, v []byte) error {
			switch id {
			case idSeekID:
				t, _, ok := readID(v)
				if !ok || len(v) > 4 {
					return errCorrupt
				}
				target = t
			case idSeekPosition:
				n, ok := readUint(v)
				if !ok {
					return errCorrupt
				}
				at, haveAt = n, true
			}
			return nil
		})
		if err != nil || !haveAt || at >= uint64(m.segEnd-m.segStart) { //nolint:gosec // G115: segEnd >= segStart
			return err // a position outside the segment is ignored
		}
		p := m.segStart + int64(at)
		switch target {
		case idInfo, idTracks, idCues:
			m.note(target, p)
		case idSeekHead:
			if len(next) < mkvSeekHeads { // more could never be tried
				next = append(next, p)
			}
		}
		return nil
	})
	for _, p := range next {
		if !m.try(p) {
			continue
		}
		sh, err := m.element(p)
		if err != nil || sh.id != idSeekHead {
			continue // a stale entry; the walk and the tail search remain
		}
		m.seekHead(sh)
	}
}

// timestampScale reads TimestampScale from Info (default 1 000 000 ns).
func (m *matroska) timestampScale() (uint64, error) {
	data, err := m.master(idInfo)
	if err != nil {
		return 0, err
	}
	scale := uint64(1_000_000)
	err = eachChild(data, func(id uint32, v []byte) error {
		if id == idTimestampScale {
			n, ok := readUint(v)
			if !ok || n == 0 {
				return errCorrupt
			}
			scale = n
		}
		return nil
	})
	return scale, err
}

// videoTrack returns the number of the first track of type video (ffmpeg's v:0).
func (m *matroska) videoTrack() (uint64, error) {
	data, err := m.master(idTracks)
	if err != nil {
		return 0, err
	}
	var track uint64
	err = eachChild(data, func(id uint32, entry []byte) error {
		if id != idTrackEntry || track != 0 {
			return nil
		}
		var number, typ uint64
		err := eachChild(entry, func(id uint32, v []byte) error {
			var ok bool
			switch id {
			case idTrackNumber:
				number, ok = readUint(v)
			case idTrackType:
				typ, ok = readUint(v)
			default:
				return nil
			}
			if !ok {
				return errCorrupt
			}
			return nil
		})
		if err == nil && typ == trackTypeVideo && number > 0 {
			track = number
		}
		return err
	})
	if err == nil && track == 0 {
		err = fmt.Errorf("%w: no video track", ErrNoIndex)
	}
	return track, err
}

// master reads the data of the Info or Tracks element at the first candidate that holds it.
func (m *matroska) master(id uint32) ([]byte, error) {
	for _, pos := range m.found[id] {
		if el, err := m.element(pos); err == nil && el.id == id {
			return m.data(el, mkvMasterLimit)
		}
	}
	return nil, fmt.Errorf("%w: element %#x not found", ErrNoIndex, id)
}

// cues returns the video cue times (videoCues) of the Cues element at the first candidate that
// holds it, else of the first match of its ID in the last MiB of the file that parses as Cues
// with video cues.
func (m *matroska) cues(track, scale uint64) ([]time.Duration, int, error) {
	for _, pos := range m.found[idCues] {
		el, err := m.element(pos)
		if err == nil && el.id == idCues {
			data, err := m.data(el, mkvCuesLimit)
			if err != nil {
				return nil, 0, err
			}
			return videoCues(data, track, scale)
		}
	}
	n := min(m.rd.size-m.segStart, mkvTail)
	tail, err := m.bytesAt(m.rd.size-n, n)
	if err != nil {
		return nil, 0, err
	}
	id := []byte{idCues >> 24, idCues >> 16 & 0xFF, idCues >> 8 & 0xFF, idCues & 0xFF}
	for try, i := 0, 0; try < mkvTailTries; try++ {
		j := bytes.Index(tail[i:], id)
		if j < 0 {
			break
		}
		i += j
		if _, hl, size, unknown, ok := header(tail[i:]); ok && !unknown && size <= uint64(len(tail)-i-hl) { //nolint:gosec // G115: header fits, so hl <= len(tail)-i
			kf, points, err := videoCues(tail[i+hl:i+hl+int(size)], track, scale) //nolint:gosec // G115: size <= len(tail) checked above
			if err == nil {
				return kf, points, nil
			}
		}
		i++
	}
	return nil, 0, fmt.Errorf("%w: no cues", ErrNoIndex)
}

// videoCues returns the times of the cue points that name the video track in any of their
// CueTrackPositions, rounded to ms, and the number of all cue points. A video cue time before the
// one of the previous video cue point is corruption (ErrNoIndex).
func videoCues(data []byte, track, scale uint64) ([]time.Duration, int, error) {
	var out []time.Duration
	var last uint64
	points := 0
	err := eachChild(data, func(id uint32, point []byte) error {
		if id != idCuePoint {
			return nil
		}
		if points++; points > maxEntries {
			return fmt.Errorf("%w: more than %d cue points", ErrNoIndex, maxEntries)
		}
		var at uint64
		var haveTime, video bool
		err := eachChild(point, func(id uint32, v []byte) error {
			switch id {
			case idCueTime:
				n, ok := readUint(v)
				if !ok {
					return errCorrupt
				}
				at, haveTime = n, true
			case idCueTrackPositions:
				return eachChild(v, func(id uint32, v []byte) error {
					if id == idCueTrack {
						n, ok := readUint(v)
						if !ok {
							return errCorrupt
						}
						video = video || n == track
					}
					return nil
				})
			}
			return nil
		})
		if err != nil || !video || !haveTime {
			return err
		}
		if at > math.MaxInt64/scale {
			return fmt.Errorf("%w: cue time overflows", ErrNoIndex)
		}
		if len(out) > 0 && at < last { // cues are written in time order; equal times are merged by Read
			return fmt.Errorf("%w: cue time %d before %d", ErrNoIndex, at, last)
		}
		last = at
		out = append(out, time.Duration(at*scale).Round(time.Millisecond)) //nolint:gosec // G115: at*scale <= MaxInt64 checked above
		return nil
	})
	if err != nil {
		return nil, points, err
	}
	if len(out) == 0 {
		return nil, points, fmt.Errorf("%w: no cues for the video track", ErrNoIndex)
	}
	return out, points, nil
}

// element reads the header of the element at pos.
func (m *matroska) element(pos int64) (element, error) {
	if pos < 0 || pos >= m.rd.size {
		return element{}, fmt.Errorf("%w: element at %d outside the file", ErrNoIndex, pos)
	}
	b, err := m.bytesAt(pos, min(mkvHeaderMax, m.rd.size-pos))
	if err != nil {
		return element{}, err
	}
	id, hl, size, unknown, ok := header(b)
	if !ok || size > math.MaxInt64-uint64(pos)-uint64(hl) { //nolint:gosec // G115: pos >= 0, hl <= 12
		return element{}, errCorrupt
	}
	return element{id: id, data: pos + int64(hl), size: int64(size), unknown: unknown}, nil //nolint:gosec // G115: size checked above
}

// data reads the data of a top-level element of at most limit bytes.
func (m *matroska) data(el element, limit int64) ([]byte, error) {
	if el.unknown {
		return nil, fmt.Errorf("%w: element %#x of unknown size", ErrNoIndex, el.id)
	}
	if el.size > limit {
		return nil, fmt.Errorf("%w: element %#x of %d bytes exceeds %d", ErrNoIndex, el.id, el.size, limit)
	}
	return m.bytesAt(el.data, el.size)
}

// bytesAt returns n bytes at pos, from the head when it holds them.
func (m *matroska) bytesAt(pos, n int64) ([]byte, error) {
	if pos >= 0 && n >= 0 && pos <= int64(len(m.head))-n {
		return m.head[pos : pos+n], nil
	}
	return m.rd.read(pos, n, max(n, 0)) // callers bound n
}

// eachChild calls fn for every child element of the data of a master element. A child that does
// not fit, or one of unknown size, makes the element corrupt.
func eachChild(b []byte, fn func(id uint32, data []byte) error) error {
	for len(b) > 0 {
		id, hl, size, unknown, ok := header(b)
		if !ok || unknown || size > uint64(len(b)-hl) { //nolint:gosec // G115: header fits, so hl <= len(b)
			return errCorrupt
		}
		end := hl + int(size) //nolint:gosec // G115: size <= len(b) checked above
		if err := fn(id, b[hl:end]); err != nil {
			return err
		}
		b = b[end:]
	}
	return nil
}

// header parses an element header: the ID with its marker, the header length and the data size.
func header(b []byte) (id uint32, hl int, size uint64, unknown, ok bool) {
	id, n, ok := readID(b)
	if !ok {
		return 0, 0, 0, false, false
	}
	size, m, unknown, ok := readSize(b[n:])
	return id, n + m, size, unknown, ok
}

// readID reads an EBML ID (1 to 4 bytes), keeping the length marker.
func readID(b []byte) (uint32, int, bool) {
	if len(b) == 0 || b[0] == 0 {
		return 0, 0, false
	}
	n := bits.LeadingZeros8(b[0]) + 1
	if n > 4 || len(b) < n {
		return 0, 0, false
	}
	var id uint32
	for _, c := range b[:n] {
		id = id<<8 | uint32(c)
	}
	return id, n, true
}

// readSize reads an EBML data size (1 to 8 bytes) without the length marker; all value bits set
// means unknown.
func readSize(b []byte) (size uint64, n int, unknown, ok bool) {
	if len(b) == 0 || b[0] == 0 {
		return 0, 0, false, false
	}
	n = bits.LeadingZeros8(b[0]) + 1
	if len(b) < n {
		return 0, 0, false, false
	}
	mask := byte(0xFF) >> n
	size = uint64(b[0] & mask)
	unknown = b[0]&mask == mask
	for _, c := range b[1:n] {
		size = size<<8 | uint64(c)
		unknown = unknown && c == 0xFF
	}
	if unknown {
		size = 0
	}
	return size, n, unknown, true
}

// readUint reads an EBML unsigned integer (0 to 8 bytes, big-endian).
func readUint(b []byte) (uint64, bool) {
	if len(b) > 8 {
		return 0, false
	}
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v, true
}
