package radio

import "time"

// framer cuts MP3 or ADTS audio into whole frames (build plan M-1): the packager only ever gets
// complete frames, in copy and in transcode mode. After the start and after every connection
// break (reset) it drops what is left of the old connection's last frame, strips a leading ID3v2
// tag of the new connection and resyncs on two consecutive valid headers before it emits again.
type framer struct {
	codec    string // "mp3" or "aac"
	carry    []byte // bytes not yet emitted
	synced   bool
	stripID3 bool // at the start of a connection
	last     frameHeader
	hasLast  bool
	pushed   time.Duration // audio of the frames the last push emitted
}

// framerMaxCarry bounds the bytes the framer keeps while it searches for a sync (an ID3 tag
// larger than this is skipped byte by byte, like garbage).
const framerMaxCarry = 1 << 20

func newFramer(codec string) *framer { return &framer{codec: codec, stripID3: true} }

// reset marks a connection break: the partial frame of the old connection is dropped and the new
// one must prove its sync again.
func (f *framer) reset() {
	f.carry = f.carry[:0]
	f.synced = false
	f.stripID3 = true
}

// params returns the header of the last emitted frame (ok false before the first one).
func (f *framer) params() (frameHeader, bool) { return f.last, f.hasLast }

// push adds bytes of the current connection and returns the whole frames now complete; their
// audio duration is f.pushed until the next push.
func (f *framer) push(b []byte) []byte {
	f.carry = append(f.carry, b...)
	f.pushed = 0
	var out []byte
	i := 0
loop:
	for i < len(f.carry) {
		rest := f.carry[i:]
		if f.stripID3 {
			switch n := id3Length(rest); {
			case n < 0: // too short to tell
				break loop
			case n > len(rest) && n <= framerMaxCarry: // wait for the whole tag
				break loop
			case n > 0 && n <= len(rest):
				i += n
				continue
			}
			f.stripID3 = false // no tag, or one too large to keep: the sync search skips it
		}
		if !f.synced {
			_, ok, need := syncAt(f.codec, f.carry, i)
			switch {
			case ok:
				f.synced = true
			case need:
				break loop
			default:
				i++
			}
			continue
		}
		h, ok := parseFrameHeader(f.codec, rest)
		switch {
		case !ok && len(rest) < 9: // header not complete yet
			break loop
		case !ok: // lost sync: resync with two headers
			f.synced = false
			continue
		case len(rest) < h.length:
			break loop
		}
		f.last, f.hasLast = h, true
		f.pushed += h.duration()
		out = append(out, rest[:h.length]...)
		i += h.length
	}
	f.carry = append(f.carry[:0], f.carry[i:]...)
	if len(f.carry) > framerMaxCarry {
		f.carry = append(f.carry[:0], f.carry[len(f.carry)-framerMaxCarry:]...)
	}
	return out
}
