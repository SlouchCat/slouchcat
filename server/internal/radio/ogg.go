package radio

import (
	"bytes"
	"encoding/binary"
	"strings"
)

// Titles of Ogg Vorbis and Ogg Opus streams (concept 16 section 7.8, build plan m-9): the second
// header packet of every logical stream is a Vorbis comment ("\x03vorbis") or OpusTags, with
// TITLE and ARTIST. Icecast Ogg stations change the title by starting a new logical stream
// (a chained stream with a new BOS page), and usually send no icy-metaint, so this is their only
// title source. The parser only reads; the bytes go to ffmpeg unchanged.

const (
	oggMaxPacket   = 64 << 10
	oggMaxComments = 256
	oggMaxPage     = 27 + 255 + 255*255
)

// oggTitles parses the Ogg pages of a stream as they arrive.
type oggTitles struct {
	carry   []byte
	streams map[uint32]*oggLogical
}

// oggLogical is the header state of one logical stream (one serial number).
type oggLogical struct {
	packets int    // complete packets so far
	pkt     []byte // the packet being assembled
	over    bool   // the packet is beyond oggMaxPacket and is skipped
	done    bool   // the comment packet was read
}

// push adds stream bytes and returns the title of the last comment packet completed in them.
func (o *oggTitles) push(b []byte) (string, bool) {
	if o.streams == nil {
		o.streams = map[uint32]*oggLogical{}
	}
	o.carry = append(o.carry, b...)
	var title string
	var found bool
	for {
		i := bytes.Index(o.carry, []byte("OggS"))
		if i < 0 {
			o.carry = o.carry[max(0, len(o.carry)-3):]
			break
		}
		page := o.carry[i:]
		if len(page) < 27 || len(page) < 27+int(page[26]) {
			o.carry = page
			break
		}
		segs := page[27 : 27+int(page[26])]
		size := 27 + len(segs)
		for _, s := range segs {
			size += int(s)
		}
		if len(page) < size {
			o.carry = page
			break
		}
		if t, ok := o.page(page[:size], segs); ok {
			title, found = t, true
		}
		o.carry = page[size:]
	}
	if len(o.carry) > oggMaxPage {
		o.carry = o.carry[len(o.carry)-oggMaxPage:]
	}
	o.carry = append([]byte(nil), o.carry...)
	return title, found
}

// page reads the packets of one page.
func (o *oggTitles) page(page, segs []byte) (string, bool) {
	serial := binary.LittleEndian.Uint32(page[14:])
	st := o.streams[serial]
	if page[5]&0x02 != 0 || st == nil { // BOS: a new logical stream (chained streams reuse no state)
		st = &oggLogical{}
		o.streams[serial] = st
	}
	if page[5]&0x04 != 0 { // EOS
		defer delete(o.streams, serial)
	}
	if st.done {
		return "", false
	}
	body := page[27+len(segs):]
	var title string
	var found bool
	for _, s := range segs {
		n := int(s)
		if !st.over {
			if len(st.pkt)+n > oggMaxPacket {
				st.over, st.pkt = true, nil
			} else {
				st.pkt = append(st.pkt, body[:n]...)
			}
		}
		body = body[n:]
		if s == 255 {
			continue // the packet goes on
		}
		st.packets++
		if st.packets == 2 && !st.over {
			title, found = oggComment(st.pkt)
			st.done = true
			return title, found
		}
		st.pkt, st.over = st.pkt[:0], false
	}
	return title, found
}

// oggComment reads TITLE and ARTIST of a Vorbis comment or OpusTags packet.
func oggComment(p []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(p, []byte("\x03vorbis")):
		p = p[7:]
	case bytes.HasPrefix(p, []byte("OpusTags")):
		p = p[8:]
	default:
		return "", false
	}
	next := func() ([]byte, bool) {
		if len(p) < 4 {
			return nil, false
		}
		n := binary.LittleEndian.Uint32(p)
		if int(n) > len(p)-4 {
			return nil, false
		}
		v := p[4 : 4+n]
		p = p[4+n:]
		return v, true
	}
	if _, ok := next(); !ok { // vendor
		return "", false
	}
	if len(p) < 4 {
		return "", false
	}
	count := min(binary.LittleEndian.Uint32(p), oggMaxComments)
	p = p[4:]
	var artist, title string
	for range count {
		c, ok := next()
		if !ok {
			break
		}
		k, v, ok := strings.Cut(decodeMetaText(c), "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(k) {
		case "TITLE":
			title = v
		case "ARTIST":
			artist = v
		}
	}
	if artist == "" && title == "" {
		return "", false
	}
	return joinArtistTitle(artist, title), true
}
