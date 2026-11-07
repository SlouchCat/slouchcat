// Package radiotest is the test station of the webradio (concept 16, build plan S4): a server on
// 127.0.0.1 with raw connections, so it can answer like Icecast ("HTTP/1.0 200 OK" with
// icy-metaint) and like Shoutcast v1 ("ICY 200 OK"), and with redirect chains, .pls and .m3u
// wrappers, wrong content types, HTML and HLS. Tests change a mount at any time, drop its
// connections and let the next connection send other audio. It is a test helper: only _test.go
// files (and the e2e build) import it.
package radiotest

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Kind is how a mount answers.
type Kind int

const (
	Icecast   Kind = iota // HTTP/1.0 200 OK with icy-* headers and, with MetaInt, ICY metadata
	Shoutcast             // ICY 200 OK
	Redirect              // 302 to Target
	PLS                   // a .pls playlist naming Target
	M3U                   // a plain .m3u playlist naming Target
	HTML                  // an HTML page (Body)
	HLSMaster             // an HLS master playlist of Variants
	HLSMedia              // an HLS media playlist of the segments set with SetHLS/AppendHLS
)

// Title is a StreamTitle in force from audio byte At of a connection on.
type Title struct {
	At   int
	Text string
}

// Variant is one entry of an HLS master playlist.
type Variant struct {
	Path      string
	Bandwidth int
	Codecs    string
}

// Segment is one HLS media segment.
type Segment struct {
	Data          []byte
	Discontinuity bool
}

// Mount describes what one path answers.
type Mount struct {
	Kind Kind
	// ContentType overrides the kind's content type; "-" sends none.
	ContentType string
	// Audio is the audio of connection i: Audio[min(i, len-1)], looped unless Once.
	Audio [][]byte
	Once  bool
	// MetaInt > 0 inserts ICY metadata every MetaInt audio bytes (with icy-metaint).
	MetaInt int
	Titles  []Title
	// CutAfter[i] > 0 closes connection i after that many audio bytes (mid-frame on purpose).
	CutAfter []int
	// Burst is sent at once at the start of a connection (default 64 KiB); after it the station
	// sends BytesPerSecond (0 = as fast as the client reads).
	Burst          int
	BytesPerSecond int
	// Target is the path or URL of Redirect, PLS and M3U.
	Target string
	// Variants of HLSMaster.
	Variants []Variant
	// Header holds extra raw header lines ("Name: value\r\n…").
	Header string
	// Body of HTML.
	Body []byte
	// Down answers 503 Service Unavailable.
	Down bool
	// Name is sent as icy-name, Bitrate as icy-br.
	Name    string
	Bitrate int
}

type mount struct {
	Mount
	path   string
	conns  int
	open   map[net.Conn]struct{}
	hls    []Segment
	hlsSeq int64 // media sequence of hls[0]
	window int
	target int
}

// Station is the running test station.
type Station struct {
	ln     net.Listener
	mu     sync.Mutex
	mounts map[string]*mount
	wg     sync.WaitGroup
}

// Start listens on 127.0.0.1 on a free port.
func Start() (*Station, error) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Station{ln: ln, mounts: map[string]*mount{}}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

// URL returns the URL of path on the station.
func (s *Station) URL(path string) string { return "http://" + s.ln.Addr().String() + path }

// Close stops the station and its connections.
func (s *Station) Close() {
	_ = s.ln.Close()
	s.mu.Lock()
	for _, m := range s.mounts {
		for c := range m.open {
			_ = c.Close()
		}
	}
	s.mu.Unlock()
	s.wg.Wait()
}

// Set replaces the mount of path; open connections keep what they were sending.
func (s *Station) Set(path string, m Mount) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.mounts[path]
	nm := &mount{Mount: m, path: path, open: map[net.Conn]struct{}{}, window: 6, target: 4}
	if old != nil {
		nm.conns, nm.open, nm.hls, nm.hlsSeq, nm.window, nm.target = old.conns, old.open, old.hls, old.hlsSeq, old.window, old.target
	}
	s.mounts[path] = nm
}

// SetHLS sets the segments of the HLS media mount at path (media sequence seq0 for the first)
// with target duration target seconds; the playlist lists the last window segments.
func (s *Station) SetHLS(path string, seq0 int64, target, window int, segs ...Segment) {
	s.mu.Lock()
	m := s.mounts[path]
	s.mu.Unlock()
	if m == nil {
		s.Set(path, Mount{Kind: HLSMedia})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m = s.mounts[path]
	m.hls, m.hlsSeq, m.target, m.window = segs, seq0, target, window
}

// AppendHLS appends segments to the HLS media mount at path.
func (s *Station) AppendHLS(path string, segs ...Segment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.mounts[path]
	m.hls = append(m.hls, segs...)
}

// Connections returns how many connections path has answered so far.
func (s *Station) Connections(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.mounts[path]; m != nil {
		return m.conns
	}
	return 0
}

// Drop closes the open connections of path now.
func (s *Station) Drop(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.mounts[path]; m != nil {
		for c := range m.open {
			_ = c.Close()
		}
	}
}

func (s *Station) serve() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer c.Close()
			s.handle(c)
		}()
	}
}

func (s *Station) handle(c net.Conn) {
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	req, err := http.ReadRequest(bufio.NewReader(c))
	if err != nil {
		return
	}
	_ = c.SetReadDeadline(time.Time{})
	path := req.URL.Path
	s.mu.Lock()
	m, seg := s.lookup(path)
	if m == nil {
		s.mu.Unlock()
		_, _ = io.WriteString(c, "HTTP/1.0 404 Not Found\r\nContent-Length: 0\r\n\r\n")
		return
	}
	idx := m.conns
	if seg == nil {
		m.conns++
	}
	m.open[c] = struct{}{}
	mc := m.Mount
	playlist := s.mediaPlaylist(m)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(m.open, c)
		s.mu.Unlock()
	}()
	if seg != nil {
		writeHead(c, "HTTP/1.1 200 OK", "video/mp2t", "Content-Length: "+strconv.Itoa(len(seg.Data))+"\r\n")
		_, _ = c.Write(seg.Data)
		return
	}
	if mc.Down {
		_, _ = io.WriteString(c, "HTTP/1.0 503 Service Unavailable\r\nContent-Length: 0\r\n\r\n")
		return
	}
	switch mc.Kind {
	case Redirect:
		_, _ = io.WriteString(c, "HTTP/1.0 302 Found\r\nLocation: "+mc.Target+"\r\n"+mc.Header+"\r\n")
	case PLS:
		writeBody(c, mc, "audio/x-scpls", []byte("[playlist]\nNumberOfEntries=1\nFile1="+s.abs(mc.Target)+"\nTitle1=Test\nVersion=2\n"))
	case M3U:
		writeBody(c, mc, "audio/x-mpegurl", []byte("# a plain m3u\n"+s.abs(mc.Target)+"\n"))
	case HTML:
		writeBody(c, mc, "text/html; charset=utf-8", mc.Body)
	case HLSMaster:
		var b strings.Builder
		b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
		for _, v := range mc.Variants {
			fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d", v.Bandwidth)
			if v.Codecs != "" {
				fmt.Fprintf(&b, ",CODECS=%q", v.Codecs)
			}
			b.WriteString("\n" + v.Path + "\n")
		}
		writeBody(c, mc, "application/vnd.apple.mpegurl", []byte(b.String()))
	case HLSMedia:
		writeBody(c, mc, "application/vnd.apple.mpegurl", playlist)
	default:
		s.stream(c, mc, idx)
	}
}

// lookup finds the mount of path, or the HLS mount and segment of "<mount>/seg/<seq>".
func (s *Station) lookup(path string) (*mount, *Segment) {
	if m := s.mounts[path]; m != nil {
		return m, nil
	}
	base, n, ok := strings.Cut(path, "/seg/")
	if !ok {
		return nil, nil
	}
	m := s.mounts[base]
	seq, err := strconv.ParseInt(n, 10, 64)
	if m == nil || err != nil || seq < m.hlsSeq || seq >= m.hlsSeq+int64(len(m.hls)) {
		return nil, nil
	}
	seg := m.hls[seq-m.hlsSeq]
	return m, &seg
}

// mediaPlaylist renders the HLS media playlist of m (the last window segments).
func (s *Station) mediaPlaylist(m *mount) []byte {
	if m.Kind != HLSMedia {
		return nil
	}
	first := max(0, len(m.hls)-m.window)
	var b strings.Builder
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:%d\n", m.target, m.hlsSeq+int64(first))
	b.WriteString(m.Header)
	for i := first; i < len(m.hls); i++ {
		if m.hls[i].Discontinuity {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		fmt.Fprintf(&b, "#EXTINF:%d.000,\n%s/seg/%d\n", m.target, m.path, m.hlsSeq+int64(i))
	}
	return []byte(b.String())
}

func (s *Station) abs(target string) string {
	if strings.Contains(target, "://") {
		return target
	}
	return s.URL(target)
}

func writeHead(w io.Writer, status, contentType, extra string) {
	h := status + "\r\n"
	if contentType != "-" {
		h += "Content-Type: " + contentType + "\r\n"
	}
	_, _ = io.WriteString(w, h+extra+"\r\n")
}

func writeBody(c net.Conn, m Mount, contentType string, body []byte) {
	if m.ContentType != "" {
		contentType = m.ContentType
	}
	writeHead(c, "HTTP/1.0 200 OK", contentType, m.Header+"Content-Length: "+strconv.Itoa(len(body))+"\r\n")
	_, _ = c.Write(body)
}

// stream sends the audio of connection idx with ICY metadata.
func (s *Station) stream(c net.Conn, m Mount, idx int) {
	status := "HTTP/1.0 200 OK"
	if m.Kind == Shoutcast {
		status = "ICY 200 OK"
	}
	ct := "audio/mpeg"
	if m.ContentType != "" {
		ct = m.ContentType
	}
	extra := m.Header
	if m.Name != "" {
		extra += "icy-name: " + m.Name + "\r\n"
	}
	if m.Bitrate > 0 {
		extra += "icy-br: " + strconv.Itoa(m.Bitrate) + "\r\n"
	}
	if m.MetaInt > 0 {
		extra += "icy-metaint: " + strconv.Itoa(m.MetaInt) + "\r\n"
	}
	writeHead(c, status, ct, extra)
	if len(m.Audio) == 0 {
		return
	}
	audio := m.Audio[min(idx, len(m.Audio)-1)]
	cut := 0
	if idx < len(m.CutAfter) {
		cut = m.CutAfter[idx]
	}
	burst := m.Burst
	if burst == 0 {
		burst = 64 << 10
	}
	w := &icyWriter{w: c, metaint: m.MetaInt, titles: m.Titles}
	start := time.Now()
	for sent, pos := 0, 0; ; {
		if pos == len(audio) {
			if m.Once {
				return
			}
			pos = 0
		}
		n := min(4<<10, len(audio)-pos)
		if cut > 0 {
			if sent >= cut {
				return
			}
			n = min(n, cut-sent)
		}
		if err := w.write(audio[pos : pos+n]); err != nil {
			return
		}
		sent += n
		pos += n
		if m.BytesPerSecond > 0 && sent > burst {
			due := start.Add(time.Duration(float64(sent-burst) / float64(m.BytesPerSecond) * float64(time.Second)))
			time.Sleep(time.Until(due))
		}
	}
}

// icyWriter interleaves audio with ICY metadata blocks.
type icyWriter struct {
	w       io.Writer
	metaint int
	titles  []Title
	pos     int // audio bytes written
	left    int
	sent    string
	started bool
}

func (w *icyWriter) write(b []byte) error {
	if w.metaint == 0 {
		_, err := w.w.Write(b)
		w.pos += len(b)
		return err
	}
	if !w.started {
		w.left, w.started = w.metaint, true
	}
	for len(b) > 0 {
		n := min(len(b), w.left)
		if _, err := w.w.Write(b[:n]); err != nil {
			return err
		}
		b, w.pos, w.left = b[n:], w.pos+n, w.left-n
		if w.left > 0 {
			continue
		}
		w.left = w.metaint
		if _, err := w.w.Write(w.block()); err != nil {
			return err
		}
	}
	return nil
}

// block returns the metadata block at the current position: the title in force when it changed,
// else an empty block.
func (w *icyWriter) block() []byte {
	title, ok := "", false
	for _, t := range w.titles {
		if t.At <= w.pos {
			title, ok = t.Text, true
		}
	}
	if !ok || title == w.sent {
		return []byte{0}
	}
	w.sent = title
	return MetaBlock(title)
}

// MetaBlock returns an ICY metadata block (length byte and padding) announcing title.
func MetaBlock(title string) []byte {
	text := []byte("StreamTitle='" + title + "';StreamUrl='';")
	n := min((len(text)+15)/16, 255)
	text = text[:min(len(text), n*16)]
	out := make([]byte, 1+n*16)
	out[0] = byte(n)
	copy(out[1:], text)
	return out
}

// ID3 returns an ID3v2.4 tag with TIT2 title and TPE1 artist (UTF-8), the leading tag of an HLS
// packed-audio segment.
func ID3(artist, title string) []byte {
	var frames bytes.Buffer
	for _, f := range []struct{ id, v string }{{"TPE1", artist}, {"TIT2", title}} {
		if f.v == "" {
			continue
		}
		frames.WriteString(f.id)
		frames.Write(syncsafe(1 + len(f.v)))
		frames.Write([]byte{0, 0, 3})
		frames.WriteString(f.v)
	}
	tag := append([]byte{'I', 'D', '3', 4, 0, 0}, syncsafe(frames.Len())...)
	return append(tag, frames.Bytes()...)
}

func syncsafe(n int) []byte {
	return []byte{byte(n >> 21 & 0x7F), byte(n >> 14 & 0x7F), byte(n >> 7 & 0x7F), byte(n & 0x7F)}
}

// SplitADTS cuts ADTS audio into n parts at frame boundaries (HLS packed-audio segments).
func SplitADTS(b []byte, n int) ([][]byte, error) {
	var frames []int // offsets
	for off := 0; off+7 <= len(b); {
		if b[off] != 0xFF || b[off+1]&0xF6 != 0xF0 {
			return nil, errors.New("radiotest: no ADTS frame at " + strconv.Itoa(off))
		}
		frames = append(frames, off)
		off += int(b[off+3]&3)<<11 | int(b[off+4])<<3 | int(b[off+5]>>5)
	}
	per := (len(frames) + n - 1) / n
	var parts [][]byte
	for i := 0; i < len(frames); i += per {
		end := len(b)
		if i+per < len(frames) {
			end = frames[i+per]
		}
		parts = append(parts, b[frames[i]:end])
	}
	return parts, nil
}
