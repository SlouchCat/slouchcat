package radio

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
)

// The ICY reader (Icecast and Shoutcast, concept 16 section 7.1, build plan S4 and hard part 3.1):
// net/http rejects the status line "ICY 200 OK", so the reader writes an HTTP/1.0 request by hand
// on a guarded connection, rewrites that status line prefix and lets http.ReadResponse parse the
// header, which it first reads under a cap (plan m-6).

const (
	maxStatusLine  = 1 << 10
	maxHeaderBytes = 16 << 10
	maxHeaderLines = 100
	maxMetaInt     = 65536
	// minMetaInt is the smallest icy-metaint a stream may send: a tinier one would make every few
	// audio bytes a metadata block (real servers send 8192 or 16000).
	minMetaInt   = 512
	icyReadChunk = 16 << 10
	// maxCollectEvents bounds the events collect keeps for sniffing.
	maxCollectEvents = 256
)

// icyResponse is the answer of one ICY request: status and header parsed, the body still on the
// connection.
type icyResponse struct {
	status int
	header http.Header
	conn   net.Conn
	body   *bufio.Reader
	stop   func() bool // ends the context watch of the connection
}

func (r *icyResponse) close() {
	r.stop()
	_ = r.conn.Close()
}

// deadlineConn sets the read deadline before every read, so a stall breaks the connection.
type deadlineConn struct{ net.Conn }

func (c deadlineConn) Read(p []byte) (int, error) {
	_ = c.SetReadDeadline(time.Now().Add(readTimeout))
	return c.Conn.Read(p)
}

// icyGet sends one GET to u and reads the response header.
func (c *netClient) icyGet(ctx context.Context, u *url.URL) (*icyResponse, error) {
	site := netguard.Site(u.String())
	conn, err := c.dialURL(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("%w: connect to %s: %w", model.ErrRadioUnreachable, site, err)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	fail := func(err error) (*icyResponse, error) {
		stop()
		_ = conn.Close()
		return nil, err
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	_ = conn.SetWriteDeadline(time.Now().Add(readTimeout))
	req := "GET " + path + " HTTP/1.0\r\nHost: " + u.Host + "\r\nUser-Agent: " + c.userAgent +
		"\r\nIcy-MetaData: 1\r\nAccept: */*\r\nConnection: close\r\n\r\n"
	if _, err := io.WriteString(conn, req); err != nil {
		return fail(fmt.Errorf("%w: send request to %s: %w", model.ErrRadioUnreachable, site, err))
	}
	br := bufio.NewReaderSize(deadlineConn{conn}, 32<<10)
	head, err := readHeaderBlock(br)
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return fail(fmt.Errorf("%w: header of %s: %w", model.ErrRadioUnreachable, site, err))
	}
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(head)), nil)
	if err != nil {
		return fail(fmt.Errorf("%w: header of %s: %w", model.ErrRadioUnreachable, site, err))
	}
	_ = resp.Body.Close() // the header block holds no body; the body stays in br
	return &icyResponse{status: resp.StatusCode, header: resp.Header, conn: conn, body: br, stop: stop}, nil
}

// readHeaderBlock reads the status line and the header lines up to the blank line, at most
// maxStatusLine for the status line, maxHeaderBytes and maxHeaderLines in all. An "ICY " status
// line becomes "HTTP/1.0 ".
func readHeaderBlock(br *bufio.Reader) ([]byte, error) {
	var head []byte
	for lines := 0; ; lines++ {
		line, err := readCappedLine(br, maxHeaderBytes-len(head))
		if err != nil {
			return nil, err
		}
		if lines == 0 {
			if len(line) > maxStatusLine {
				return nil, errHeaderTooLarge
			}
			if rest, ok := bytes.CutPrefix(line, []byte("ICY ")); ok {
				line = append([]byte("HTTP/1.0 "), rest...)
			}
		}
		head = append(head, line...)
		if len(bytes.TrimRight(line, "\r\n")) == 0 {
			return head, nil
		}
		if lines >= maxHeaderLines {
			return nil, errHeaderTooLarge
		}
	}
}

// readCappedLine reads one line (with its newline) of at most limit bytes.
func readCappedLine(br *bufio.Reader, limit int) ([]byte, error) {
	var line []byte
	for {
		if limit <= 0 {
			return nil, errHeaderTooLarge
		}
		chunk, err := br.ReadSlice('\n')
		if len(chunk) > limit {
			return nil, errHeaderTooLarge
		}
		limit -= len(chunk)
		line = append(line, chunk...)
		switch {
		case err == nil:
			return line, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		default:
			return nil, err
		}
	}
}

// icyStream is an Icecast or Shoutcast stream after its header: audio with the ICY metadata cut
// out; titles of StreamTitle and of Ogg comments.
type icyStream struct {
	inf     streamInfo
	resp    *icyResponse
	left    int           // audio bytes until the next metadata block
	pending []streamEvent // read for sniffing, not yet handed out
	ogg     *oggTitles
}

func newICYStream(rawURL, contentType string, resp *icyResponse) *icyStream {
	h := resp.header
	inf := streamInfo{URL: rawURL, ContentType: contentType,
		ICYName: strings.TrimSpace(h.Get("icy-name")), ICYGenre: strings.TrimSpace(h.Get("icy-genre")),
		ICYURL: strings.TrimSpace(h.Get("icy-url"))}
	if br, _, _ := strings.Cut(h.Get("icy-br"), ","); br != "" {
		inf.ICYBitrate, _ = strconv.Atoi(strings.TrimSpace(br))
	}
	if mi, err := strconv.Atoi(strings.TrimSpace(h.Get("icy-metaint"))); err == nil && mi >= minMetaInt && mi <= maxMetaInt {
		inf.MetaInt = mi
	}
	return &icyStream{inf: inf, resp: resp, left: inf.MetaInt}
}

func (s *icyStream) Info() streamInfo { return s.inf }

func (s *icyStream) Close() error {
	s.resp.close()
	return nil
}

// collect reads at least n audio bytes (less at the end of the stream, or after maxCollectEvents
// events) for sniffing and returns them; Next hands the events out again.
func (s *icyStream) collect(n int) ([]byte, error) {
	var audio []byte
	for len(audio) < n && len(s.pending) < maxCollectEvents {
		b, title, ok, err := s.read()
		if len(b) > 0 || ok {
			s.pending = append(s.pending, streamEvent{Audio: b, Title: title, TitleSet: ok})
			audio = append(audio, b...)
		}
		if err != nil {
			return audio, err
		}
	}
	return audio, nil
}

func (s *icyStream) Next(ctx context.Context) (streamEvent, error) {
	if s.inf.Input.Format == "ogg" && s.ogg == nil {
		s.ogg = &oggTitles{}
	}
	var ev streamEvent
	if len(s.pending) > 0 {
		ev = s.pending[0]
		s.pending = s.pending[1:]
	} else {
		b, title, ok, err := s.read()
		if err != nil && len(b) == 0 && !ok {
			if ctx.Err() != nil {
				return streamEvent{}, ctx.Err()
			}
			return streamEvent{}, err
		}
		ev = streamEvent{Audio: b, Title: title, TitleSet: ok}
	}
	if s.ogg != nil && len(ev.Audio) > 0 {
		if t, ok := s.ogg.push(ev.Audio); ok && !ev.TitleSet {
			ev.Title, ev.TitleSet = t, true
		}
	}
	return ev, nil
}

// read returns the next audio bytes up to the next metadata block, or the title of a metadata
// block (ok) with no audio.
func (s *icyStream) read() (audio []byte, title string, ok bool, err error) {
	if s.inf.MetaInt > 0 && s.left == 0 {
		s.left = s.inf.MetaInt
		n, err := s.resp.body.ReadByte()
		if err != nil {
			return nil, "", false, err
		}
		if n == 0 {
			return nil, "", false, nil
		}
		block := make([]byte, int(n)*16)
		if _, err := io.ReadFull(s.resp.body, block); err != nil {
			return nil, "", false, err
		}
		t, has, _ := parseICYMeta(block)
		return nil, t, has, nil
	}
	size := icyReadChunk
	if s.inf.MetaInt > 0 {
		size = min(size, s.left)
	}
	buf := make([]byte, size)
	n, err := s.resp.body.Read(buf)
	if s.inf.MetaInt > 0 {
		s.left -= n
	}
	return buf[:n], "", false, err
}
