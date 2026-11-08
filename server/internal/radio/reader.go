package radio

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// The stream reader (concept 16 sections 7.1 and 7.8, build plan S4): every connection is dialled
// through the guarded dialer (controlFunc), every redirect and playlist hop dials again, so the
// guard sees each resolved address. The reader hands out clean audio (ICY metadata removed, ID3
// tags of HLS segments removed) and the titles the stream announces.

const (
	// maxHops bounds redirects and playlist hops of one open, together.
	maxHops = 5
	// readTimeout is the stall limit of a stream read; a stall is a connection break.
	readTimeout = 20 * time.Second
	// maxPlaylistBytes bounds a .pls or .m3u answer.
	maxPlaylistBytes = 64 << 10
)

// dialControl is the type of controlFunc and of the test replacement.
type dialControl func(network, address string, c syscall.RawConn) error

// netClient is the one way the radio part reaches the network: a dialer with the guard and an
// HTTP client on the same dialer for HLS, the catalog and logos.
type netClient struct {
	log       *slog.Logger
	dialer    *net.Dialer
	userAgent string
	http      *http.Client
}

func newNetClient(control dialControl, userAgent string, log *slog.Logger) *netClient {
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second, Control: control}
	if userAgent == "" {
		userAgent = "Slouchcat"
	}
	c := &netClient{log: log, dialer: d, userAgent: userAgent}
	c.http = &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil, // the guard checks the address it dials
			DialContext:           d.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          16,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxHops {
				return fmt.Errorf("more than %d redirects", maxHops)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to scheme %q refused", req.URL.Scheme)
			}
			return nil
		},
	}
	return c
}

// get fetches rawURL with the guarded HTTP client and returns at most limit bytes of the body
// (more is an error).
func (c *netClient) get(ctx context.Context, rawURL string, limit int64) ([]byte, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s", model.ErrRadioUnreachable, netguard.Site(rawURL))
	}
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", model.ErrRadioUnreachable, netguard.Clean(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, nil, fmt.Errorf("%w: %s answered %d", model.ErrRadioUnreachable, netguard.Site(rawURL), resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: read %s: %w", model.ErrRadioUnreachable, netguard.Site(rawURL), netguard.Clean(err))
	}
	if int64(len(b)) > limit {
		return nil, nil, fmt.Errorf("%w: %s sent more than %d bytes", model.ErrRadioNotAudio, netguard.Site(rawURL), limit)
	}
	return b, resp.Header, nil
}

// dialURL dials the host of u through the guard; https gets TLS 1.2 or later.
func (c *netClient) dialURL(ctx context.Context, u *url.URL) (net.Conn, error) {
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	addr := net.JoinHostPort(u.Hostname(), port)
	if u.Scheme == "https" {
		td := &tls.Dialer{NetDialer: c.dialer, Config: &tls.Config{ServerName: u.Hostname(), MinVersion: tls.VersionTLS12}}
		return td.DialContext(ctx, "tcp", addr)
	}
	return c.dialer.DialContext(ctx, "tcp", addr)
}

// streamInfo describes an open stream.
type streamInfo struct {
	URL         string // the URL that answered with audio (log it only through netguard.Site)
	ContentType string
	Input       transcode.RadioInput
	HLS         bool
	MetaInt     int    // ICY metadata interval, 0 = none
	ICYName     string // icy-name
	ICYGenre    string
	ICYURL      string
	ICYBitrate  int // first number of icy-br
}

// streamEvent is one step of a stream: audio bytes, a title the stream announced (TitleSet with
// Title, which may be empty: the stream cleared it), or both. StreamURL is the StreamUrl of the
// ICY block that announced the title (an image address for some stations, concept 16 section 8).
// Discontinuity marks a source discontinuity before this audio (HLS EXT-X-DISCONTINUITY or a
// media sequence gap).
type streamEvent struct {
	Audio         []byte
	Title         string
	TitleSet      bool
	StreamURL     string
	Discontinuity bool
}

// stream is one open connection to a station's stream.
type stream interface {
	Info() streamInfo
	// Next blocks for the next event; an error is a connection break (io.EOF included).
	Next(ctx context.Context) (streamEvent, error)
	Close() error
}

// playlistTypes are the content types read as a playlist. Every other answer, the audio types
// (audio/mpeg, audio/aac, audio/aacp, audio/mp4, audio/ogg, application/ogg, audio/flac,
// audio/opus) included, is sniffed: the packager needs the format from the bytes anyway, and a
// wrong or missing type then decides nothing (concept 16 section 7.8).
var playlistTypes = []string{"audio/x-scpls", "audio/x-mpegurl", "audio/mpegurl", "application/vnd.apple.mpegurl", "application/x-mpegurl"}

// openStream connects to rawURL and follows redirects and playlists (at most maxHops together)
// until a response with audio answers. Errors: model.ErrRadioUnreachable (wrapping
// netguard.ErrRefused for a local target), model.ErrRadioNotAudio.
func (c *netClient) openStream(ctx context.Context, rawURL string) (stream, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: unparsable stream URL", model.ErrRadioUnreachable)
	}
	for hops := 0; ; {
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("%w: scheme %q refused", model.ErrRadioUnreachable, u.Scheme)
		}
		resp, err := c.icyGet(ctx, u)
		if err != nil {
			return nil, err
		}
		next, st, err := c.answer(ctx, u, resp)
		if err != nil || st != nil {
			return st, err
		}
		if hops++; hops > maxHops {
			return nil, fmt.Errorf("%w: more than %d redirects or playlist hops", model.ErrRadioUnreachable, maxHops)
		}
		u = next
	}
}

// answer handles one response of openStream: a redirect or playlist returns the next URL, audio
// returns the stream. resp is closed unless it becomes the stream.
func (c *netClient) answer(ctx context.Context, u *url.URL, resp *icyResponse) (*url.URL, stream, error) {
	site := netguard.Site(u.String())
	switch code := resp.status; {
	case code == 301 || code == 302 || code == 303 || code == 307 || code == 308:
		resp.close()
		loc, err := u.Parse(resp.header.Get("Location"))
		if err != nil || resp.header.Get("Location") == "" {
			return nil, nil, fmt.Errorf("%w: %s redirects without a valid Location", model.ErrRadioUnreachable, site)
		}
		return loc, nil, nil
	case code/100 != 2:
		resp.close()
		return nil, nil, fmt.Errorf("%w: %s answered %d", model.ErrRadioUnreachable, site, code)
	}
	ct, _, _ := mime.ParseMediaType(resp.header.Get("Content-Type"))
	ct = strings.ToLower(ct)
	if slices.Contains(playlistTypes, ct) {
		body, err := io.ReadAll(io.LimitReader(resp.body, maxPlaylistBytes))
		resp.close()
		if err != nil && len(body) == 0 {
			return nil, nil, fmt.Errorf("%w: read playlist of %s: %w", model.ErrRadioUnreachable, site, netguard.Clean(err))
		}
		return c.playlistNext(ctx, u, body)
	}
	if v := strings.TrimSpace(resp.header.Get("icy-metaint")); v != "" {
		if mi, err := strconv.Atoi(v); err != nil || mi < minMetaInt || mi > maxMetaInt {
			resp.close()
			return nil, nil, fmt.Errorf("%w: %s sends icy-metaint %q", model.ErrRadioNotAudio, site, v)
		}
	}
	st := newICYStream(u.String(), ct, resp)
	first, err := st.collect(sniffMinBytes)
	if err != nil && len(first) == 0 {
		_ = st.Close()
		return nil, nil, fmt.Errorf("%w: read %s: %w", model.ErrRadioUnreachable, site, err)
	}
	in, kind, err := sniff(first)
	switch {
	case err != nil:
		_ = st.Close()
		return nil, nil, fmt.Errorf("%w: %s sends %q", model.ErrRadioNotAudio, site, ct)
	case kind == kindAudio:
		if in.Codec == "aac" && ct == "audio/aacp" {
			in.Codec = "he-aac"
		}
		if in.BitrateKbps == 0 {
			in.BitrateKbps = st.inf.ICYBitrate
		}
		st.inf.Input = in
		return nil, st, nil
	}
	_ = st.Close()
	return c.playlistNext(ctx, u, first)
}

// playlistNext resolves a playlist body: HLS opens the HLS source, a .pls or .m3u names the next
// URL (its first http or https entry).
func (c *netClient) playlistNext(ctx context.Context, u *url.URL, body []byte) (*url.URL, stream, error) {
	if _, kind, _ := sniff(body); kind == kindHLS {
		st, err := c.openHLS(ctx, u.String())
		return nil, st, err
	}
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if k, v, ok := strings.Cut(line, "="); ok && strings.HasPrefix(strings.ToLower(k), "file") {
			line = strings.TrimSpace(v) // .pls: FileN=<url>
		}
		if l := strings.ToLower(line); !strings.HasPrefix(l, "http://") && !strings.HasPrefix(l, "https://") {
			continue
		}
		if next, err := url.Parse(line); err == nil {
			return next, nil, nil
		}
	}
	return nil, nil, fmt.Errorf("%w: playlist of %s names no stream", model.ErrRadioNotAudio, netguard.Site(u.String()))
}

// errHeaderTooLarge is the reader error of a response whose header exceeds the caps.
var errHeaderTooLarge = errors.New("response header too large")
