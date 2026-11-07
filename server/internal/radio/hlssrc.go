package radio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
)

// HLS sources (build plan S4): a station that offers HLS is read segment by segment through the
// guarded HTTP client. Segments are judged by content (MPEG-TS, ADTS or MP3 behind ID3, fMP4),
// never by their content type (TS segments come as video/mp2t, plan R-11).

const (
	maxHLSPlaylist   = 1 << 20
	maxHLSSegment    = 16 << 20
	hlsStartSegments = 3 // the first load fetches the last three segments at once (a start burst)
	hlsMaxBandwidth  = 320_000
)

// hlsPlaylist is a parsed master or media playlist.
type hlsPlaylist struct {
	variants []hlsVariant
	target   time.Duration
	seq      int64 // media sequence of the first segment
	segments []hlsSegment
	initURI  string // EXT-X-MAP
	ended    bool
	key      string // METHOD of the last EXT-X-KEY ("" or NONE = clear)
}

type hlsVariant struct {
	uri       string
	bandwidth int
	codecs    string
}

type hlsSegment struct {
	seq           int64
	uri           string
	discontinuity bool
}

// parseHLS parses an HLS playlist.
func parseHLS(body []byte) hlsPlaylist {
	var pl hlsPlaylist
	var pendingVariant *hlsVariant
	disc := false
	n := int64(0)
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 0, 64<<10), maxHLSPlaylist)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		tag, val, _ := strings.Cut(line, ":")
		switch {
		case line == "":
		case tag == "#EXT-X-STREAM-INF":
			a := hlsAttrs(val)
			bw, _ := strconv.Atoi(a["BANDWIDTH"])
			pendingVariant = &hlsVariant{bandwidth: bw, codecs: a["CODECS"]}
		case tag == "#EXT-X-TARGETDURATION":
			if s, err := strconv.ParseFloat(val, 64); err == nil && s > 0 {
				pl.target = time.Duration(s * float64(time.Second))
			}
		case tag == "#EXT-X-MEDIA-SEQUENCE":
			pl.seq, _ = strconv.ParseInt(val, 10, 64)
		case tag == "#EXT-X-DISCONTINUITY":
			disc = true
		case tag == "#EXT-X-KEY":
			pl.key = strings.ToUpper(hlsAttrs(val)["METHOD"])
		case tag == "#EXT-X-MAP":
			pl.initURI = hlsAttrs(val)["URI"]
		case tag == "#EXT-X-ENDLIST":
			pl.ended = true
		case strings.HasPrefix(line, "#"):
		case pendingVariant != nil:
			pendingVariant.uri = line
			pl.variants = append(pl.variants, *pendingVariant)
			pendingVariant = nil
		default:
			pl.segments = append(pl.segments, hlsSegment{seq: pl.seq + n, uri: line, discontinuity: disc})
			n++
			disc = false
		}
	}
	return pl
}

// hlsAttrs parses an attribute list (KEY=value,KEY="quoted, value").
func hlsAttrs(s string) map[string]string {
	a := map[string]string{}
	for s != "" {
		k, rest, ok := strings.Cut(s, "=")
		if !ok {
			break
		}
		var v string
		if strings.HasPrefix(rest, `"`) {
			end := strings.IndexByte(rest[1:], '"')
			if end < 0 {
				v, rest = rest[1:], ""
			} else {
				v, rest = rest[1:1+end], rest[2+end:]
			}
			rest = strings.TrimPrefix(rest, ",")
		} else {
			v, rest, _ = strings.Cut(rest, ",")
		}
		a[strings.ToUpper(strings.TrimSpace(k))] = v
		s = rest
	}
	return a
}

// pickVariant chooses the variant with the highest bandwidth up to 320 kbit/s whose codecs are
// audio only (mp4a.*), else the one with the lowest bandwidth.
func pickVariant(vs []hlsVariant) hlsVariant {
	var best *hlsVariant
	lowest := vs[0]
	for i, v := range vs {
		if v.bandwidth < lowest.bandwidth {
			lowest = v
		}
		if v.bandwidth <= hlsMaxBandwidth && audioOnly(v.codecs) && (best == nil || v.bandwidth > best.bandwidth) {
			best = &vs[i]
		}
	}
	if best == nil {
		return lowest
	}
	return *best
}

func audioOnly(codecs string) bool {
	if codecs == "" {
		return false
	}
	for c := range strings.SplitSeq(codecs, ",") {
		if !strings.HasPrefix(strings.TrimSpace(strings.ToLower(c)), "mp4a.") {
			return false
		}
	}
	return true
}

// hlsStream reads an HLS media playlist as a stream.
type hlsStream struct {
	c       *netClient
	media   *url.URL
	inf     streamInfo
	codecs  string
	target  time.Duration
	lastSeq int64 // media sequence of the last queued segment; -1 before the first load
	queue   []hlsSegment
	reload  time.Time
	ended   bool
	init    []byte // fMP4 initialisation (EXT-X-MAP), sent before the first segment and after a discontinuity
	sendIni bool
	pending *streamEvent
}

// openHLS opens the HLS source at rawURL (a master or media playlist) and reads its first
// segment to know the audio.
func (c *netClient) openHLS(ctx context.Context, rawURL string) (stream, error) {
	body, _, err := c.get(ctx, rawURL, maxHLSPlaylist)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: unparsable playlist URL", model.ErrRadioUnreachable)
	}
	pl := parseHLS(body)
	codecs := ""
	if len(pl.variants) > 0 {
		v := pickVariant(pl.variants)
		codecs = v.codecs
		if u, err = u.Parse(v.uri); err != nil {
			return nil, fmt.Errorf("%w: unparsable variant URL", model.ErrRadioUnreachable)
		}
		if body, _, err = c.get(ctx, u.String(), maxHLSPlaylist); err != nil {
			return nil, err
		}
		pl = parseHLS(body)
	}
	s := &hlsStream{c: c, media: u, codecs: codecs, lastSeq: -1,
		inf: streamInfo{URL: u.String(), ContentType: "application/vnd.apple.mpegurl", HLS: true}}
	if err := s.apply(ctx, pl); err != nil {
		return nil, err
	}
	ev, err := s.fetchNext(ctx)
	if err != nil {
		return nil, err
	}
	in, kind, err := sniff(ev.Audio)
	if err != nil || kind != kindAudio {
		return nil, fmt.Errorf("%w: HLS segment of %s is no audio", model.ErrRadioNotAudio, netguard.Site(u.String()))
	}
	if in.Codec == "aac" && (strings.Contains(codecs, "mp4a.40.5") || strings.Contains(codecs, "mp4a.40.29")) {
		in.Codec = "he-aac"
	}
	s.inf.Input = in
	s.pending = &ev
	return s, nil
}

func (s *hlsStream) Info() streamInfo { return s.inf }
func (s *hlsStream) Close() error     { return nil }

// apply takes a loaded media playlist: new segments are queued (on the first load the last
// three), a media sequence gap marks a discontinuity.
func (s *hlsStream) apply(ctx context.Context, pl hlsPlaylist) error {
	if pl.key != "" && pl.key != "NONE" {
		return fmt.Errorf("%w: encrypted HLS (%s)", model.ErrRadioNotAudio, pl.key)
	}
	s.target, s.ended = pl.target, pl.ended
	if s.target <= 0 {
		s.target = 6 * time.Second
	}
	s.reload = time.Now().Add(max(s.target/2, time.Second))
	if pl.initURI != "" && s.init == nil {
		u, err := s.media.Parse(pl.initURI)
		if err != nil {
			return fmt.Errorf("%w: unparsable EXT-X-MAP", model.ErrRadioNotAudio)
		}
		if s.init, _, err = s.c.get(ctx, u.String(), maxHLSSegment); err != nil {
			return err
		}
		s.sendIni = true
	}
	segs := pl.segments
	if n := len(segs); s.lastSeq >= 0 && n > 0 && segs[n-1].seq < s.lastSeq {
		s.lastSeq = -1 // the source restarted its media sequence: start again at its end
		s.c.log.Info("radio hls media sequence restarted", "site", netguard.Site(s.media.String()))
	}
	first := s.lastSeq < 0
	if first && len(segs) > hlsStartSegments {
		segs = segs[len(segs)-hlsStartSegments:]
	}
	for _, seg := range segs {
		if !first && seg.seq <= s.lastSeq {
			continue
		}
		if !first && seg.seq > s.lastSeq+1 {
			s.c.log.Info("radio hls media sequence gap", "site", netguard.Site(s.media.String()),
				"after", s.lastSeq, "next", seg.seq)
			seg.discontinuity = true
		}
		s.queue = append(s.queue, seg)
		s.lastSeq = seg.seq
		first = false
	}
	return nil
}

func (s *hlsStream) Next(ctx context.Context) (streamEvent, error) {
	if s.pending != nil {
		ev := *s.pending
		s.pending = nil
		return ev, nil
	}
	return s.fetchNext(ctx)
}

// fetchNext waits for the next segment (reloading the playlist as needed) and fetches it.
func (s *hlsStream) fetchNext(ctx context.Context) (streamEvent, error) {
	for len(s.queue) == 0 {
		if s.ended {
			return streamEvent{}, io.EOF
		}
		t := time.NewTimer(time.Until(s.reload))
		select {
		case <-ctx.Done():
			t.Stop()
			return streamEvent{}, ctx.Err()
		case <-t.C:
		}
		body, _, err := s.c.get(ctx, s.media.String(), maxHLSPlaylist)
		if err != nil {
			return streamEvent{}, err
		}
		if err := s.apply(ctx, parseHLS(body)); err != nil {
			return streamEvent{}, err
		}
	}
	seg := s.queue[0]
	s.queue = s.queue[1:]
	u, err := s.media.Parse(seg.uri)
	if err != nil {
		return streamEvent{}, fmt.Errorf("%w: unparsable segment URL", model.ErrRadioUnreachable)
	}
	data, _, err := s.c.get(ctx, u.String(), maxHLSSegment)
	if err != nil {
		return streamEvent{}, err
	}
	ev := streamEvent{Discontinuity: seg.discontinuity}
	switch {
	case len(data) >= 8 && isBoxType(string(data[4:8])):
		if s.sendIni || seg.discontinuity {
			data = append(append([]byte(nil), s.init...), data...)
			s.sendIni = false
		}
	case len(data) > 0 && data[0] == 0x47:
	default:
		if n := id3Length(data); n > 0 && n <= len(data) {
			ev.Title, ev.TitleSet = id3Title(data[:n])
			data = data[n:]
		}
	}
	ev.Audio = data
	return ev, nil
}

func isBoxType(t string) bool {
	switch t {
	case "ftyp", "styp", "moof", "moov", "sidx", "prft", "emsg":
		return true
	}
	return false
}

// id3Title reads TIT2, TPE1 and a TXXX titled "title" or "StreamTitle" of an ID3v2.3/2.4 tag.
func id3Title(tag []byte) (string, bool) {
	if len(tag) < 10 {
		return "", false
	}
	version := tag[3]
	body := tag[10:]
	if tag[5]&0x40 != 0 && len(body) >= 4 { // extended header
		n := int(binary.BigEndian.Uint32(body))
		if version == 4 {
			n = syncsafe(body)
		} else {
			n += 4
		}
		if n > len(body) {
			return "", false
		}
		body = body[n:]
	}
	var artist, title, txxx string
	for len(body) >= 10 && body[0] != 0 {
		id := string(body[:4])
		n := int(binary.BigEndian.Uint32(body[4:]))
		if version == 4 {
			n = syncsafe(body[4:])
		}
		if n < 0 || 10+n > len(body) {
			break
		}
		data := body[10 : 10+n]
		body = body[10+n:]
		switch id {
		case "TIT2":
			title = id3Text(data)
		case "TPE1":
			artist = id3Text(data)
		case "TXXX":
			desc, val, _ := strings.Cut(id3Text(data), "\x00")
			if d := strings.ToLower(desc); d == "title" || d == "streamtitle" {
				txxx = val
			}
		}
	}
	if t := joinArtistTitle(artist, title); t != "" {
		return t, true
	}
	return txxx, txxx != ""
}

func syncsafe(b []byte) int {
	return int(b[0]&0x7F)<<21 | int(b[1]&0x7F)<<14 | int(b[2]&0x7F)<<7 | int(b[3]&0x7F)
}

// id3Text decodes an ID3 text frame (encoding byte, then ISO-8859-1, UTF-16 with BOM,
// UTF-16BE or UTF-8); a TXXX keeps the NUL between description and value.
func id3Text(d []byte) string {
	if len(d) == 0 {
		return ""
	}
	enc, d := d[0], d[1:]
	switch enc {
	case 1, 2:
		be := enc == 2
		u := make([]uint16, 0, len(d)/2)
		for i := 0; i+1 < len(d); i += 2 {
			switch {
			case d[i] == 0xFF && d[i+1] == 0xFE:
				be = false
				continue
			case d[i] == 0xFE && d[i+1] == 0xFF:
				be = true
				continue
			}
			if be {
				u = append(u, uint16(d[i])<<8|uint16(d[i+1]))
			} else {
				u = append(u, uint16(d[i+1])<<8|uint16(d[i]))
			}
		}
		return strings.TrimRight(string(utf16.Decode(u)), "\x00")
	case 3:
		return strings.TrimRight(string(d), "\x00")
	}
	r := make([]rune, len(d)) // ISO-8859-1
	for i, c := range d {
		r[i] = rune(c)
	}
	return strings.TrimRight(string(r), "\x00")
}
