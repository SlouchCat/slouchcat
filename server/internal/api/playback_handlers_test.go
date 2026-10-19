package api

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
)

// fakeTranscoder serves fixed playlists and segments for every playback session id, so tokens
// minted directly in the store reach the handlers; it records stops.
type fakeTranscoder struct {
	mu      sync.Mutex
	stopped []string
	busy    bool
}

type fakeHLS struct{ id string }

func newFakeTranscoder() *fakeTranscoder { return &fakeTranscoder{} }

func (*fakeTranscoder) Backend() string { return "software" }

func (f *fakeTranscoder) Start(_ context.Context, req playback.HLSRequest) (playback.HLSSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.busy {
		return nil, model.ErrTranscodeBusy
	}
	return fakeHLS{id: req.ID}, nil
}

func (*fakeTranscoder) Get(id string) (playback.HLSSession, bool) { return fakeHLS{id: id}, true }

func (f *fakeTranscoder) Stop(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, id)
}

func (f *fakeTranscoder) stops() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.stopped...)
}

func (fakeHLS) MasterPlaylist(mediaURL string) []byte {
	return []byte("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1000000\n" + mediaURL + "\n")
}

func (fakeHLS) MediaPlaylist(initURL string, segmentURL func(int) string) []byte {
	return []byte("#EXTM3U\n#EXT-X-MAP:URI=\"" + initURL + "\"\n#EXTINF:4,\n" + segmentURL(0) + "\n#EXT-X-ENDLIST\n")
}

type readSeekNopCloser struct{ *bytes.Reader }

func (readSeekNopCloser) Close() error { return nil }

func (h fakeHLS) OpenInit(context.Context) (io.ReadSeekCloser, error) {
	return readSeekNopCloser{bytes.NewReader([]byte("init " + h.id))}, nil
}

// restartedSegment is a segment whose wait a newer seek superseded.
const restartedSegment = 99

func (fakeHLS) OpenSegment(_ context.Context, i int) (io.ReadSeekCloser, error) {
	if i == restartedSegment {
		return nil, model.ErrSegmentRestarted
	}
	return readSeekNopCloser{bytes.NewReader([]byte{byte(i)})}, nil
}

func (fakeHLS) Touch() {}

func streamParam(t *testing.T, streamURL string) string {
	t.Helper()
	u, err := url.Parse(streamURL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("st")
}

// Plan 5.4: token-only access, 401 for missing, forged and revoked tokens, revocation by stop and
// logout, only the owning browser session may stop, Referrer-Policy everywhere.
func TestStreamTokenEndpoints(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	m := f.Rated(12)
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", `{"format":{"name":"matroska,webm","duration":90000000000},"streams":[{"index":0,"type":"video","codec":"hevc","width":1280,"height":720}]}`, nil)

	rec := send(t, h, http.MethodPost, BasePath+"/items/"+m.ID+"/play", "p12", `{"capabilities":{"video":["h264"],"audio":["aac"],"hls":"mse"}}`, nil)
	var info struct {
		Mode              string `json:"mode"`
		StreamURL         string `json:"streamUrl"`
		PlaybackSessionID string `json:"playbackSessionId"`
		Transcoding       struct {
			Mode    string `json:"mode"`
			Backend string `json:"backend"`
			Reason  string `json:"reason"`
		} `json:"transcoding"`
	}
	decode(t, rec, &info)
	if info.Mode != "hls" || info.Transcoding.Mode != "transcode" || info.Transcoding.Backend != "software" || info.Transcoding.Reason != "video_codec" ||
		info.StreamURL != BasePath+"/playback/sessions/"+info.PlaybackSessionID+"/master.m3u8?st="+url.QueryEscape(streamParam(t, info.StreamURL)) {
		t.Fatalf("startPlayback = %s", rec.Body.String())
	}
	if rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("JSON response without Referrer-Policy: %v", rec.Header())
	}
	st := streamParam(t, info.StreamURL)
	base := BasePath + "/playback/sessions/" + info.PlaybackSessionID

	// Playlists and segments with the token, no cookie.
	master := send(t, h, http.MethodGet, info.StreamURL, "", "", nil)
	if master.Code != http.StatusOK || !strings.Contains(master.Body.String(), "media.m3u8?st="+url.QueryEscape(st)) ||
		master.Header().Get("Referrer-Policy") != "no-referrer" || master.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("master: %d %v %q", master.Code, master.Header(), master.Body.String())
	}
	for _, rel := range []string{"media.m3u8", "init.mp4", "seg/0", "seg/7"} {
		if got := send(t, h, http.MethodGet, base+"/"+rel+"?st="+url.QueryEscape(st), "", "", nil); got.Code != http.StatusOK {
			t.Errorf("%s: %d %s", rel, got.Code, got.Body.String())
		}
	}

	// Without, with a forged and with another session's token: 401; the cookie alone does not help.
	_, foreign := pf.mint(t, "p12", f.Rated(6), "remux")
	for name, u := range map[string]string{
		"no token":         base + "/media.m3u8",
		"cookie only":      base + "/seg/0",
		"forged":           base + "/media.m3u8?st=" + url.QueryEscape(playback.SignToken([]byte("wrong key"), info.PlaybackSessionID, m.ID)),
		"garbage":          base + "/media.m3u8?st=nonsense",
		"direct no token":  BasePath + "/items/" + m.ID + "/stream",
		"direct forged st": BasePath + "/items/" + m.ID + "/stream?st=" + url.QueryEscape(playback.SignToken([]byte("wrong key"), info.PlaybackSessionID, m.ID)),
	} {
		cookie := ""
		if name == "cookie only" || name == "direct no token" {
			cookie = "p12"
		}
		got := send(t, h, http.MethodGet, u, cookie, "", nil)
		if got.Code != http.StatusUnauthorized || problemCode(t, got) != "unauthenticated" {
			t.Errorf("%s: %d %s", name, got.Code, got.Body.String())
		}
	}
	// A valid token of another playback session does not open this one.
	if got := send(t, h, http.MethodGet, base+"/media.m3u8?st="+url.QueryEscape(foreign), "", "", nil); got.Code != http.StatusNotFound {
		t.Errorf("token of another playback session: %d", got.Code)
	}
	// An HLS token does not open the Range stream.
	if got := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/stream?st="+url.QueryEscape(st), "", "", nil); got.Code != http.StatusNotFound {
		t.Errorf("Range stream with an HLS token: %d", got.Code)
	}
	if got := send(t, h, http.MethodGet, base+"/seg/x?st="+url.QueryEscape(st), "", "", nil); got.Code != http.StatusNotFound {
		t.Errorf("segment x: %d", got.Code)
	}
	// A wait that a newer seek superseded: 503, retry in a second.
	if got := send(t, h, http.MethodGet, base+"/seg/99?st="+url.QueryEscape(st), "", "", nil); got.Code != http.StatusServiceUnavailable ||
		problemCode(t, got) != "segment_restarted" || got.Header().Get("Retry-After") != "1" {
		t.Errorf("superseded segment: %d %v %s", got.Code, got.Header(), got.Body.String())
	}

	// Only the owning browser session may stop it; then 401 within the next request.
	if got := send(t, h, http.MethodDelete, base, "p18", "", nil); got.Code != http.StatusNotFound {
		t.Errorf("stop by another browser session: %d", got.Code)
	}
	if got := send(t, h, http.MethodGet, base+"/seg/1?st="+url.QueryEscape(st), "", "", nil); got.Code != http.StatusOK {
		t.Errorf("still valid after a foreign stop: %d", got.Code)
	}
	if got := send(t, h, http.MethodDelete, base, "p12", "", nil); got.Code != http.StatusNoContent {
		t.Fatalf("stop: %d %s", got.Code, got.Body.String())
	}
	start := time.Now()
	got := send(t, h, http.MethodGet, base+"/seg/1?st="+url.QueryEscape(st), "", "", nil)
	if got.Code != http.StatusUnauthorized || time.Since(start) > time.Second {
		t.Errorf("after stop: %d in %v", got.Code, time.Since(start))
	}
	if stops := pf.tc.stops(); len(stops) != 1 || stops[0] != info.PlaybackSessionID {
		t.Errorf("transcoder stops = %v", stops)
	}

	// Logout ends every playback session of the browser session.
	ps, st2 := pf.mint(t, "p12", f.Rated(6), "direct")
	stream := BasePath + "/items/" + f.Rated(6).ID + "/stream?st=" + url.QueryEscape(st2)
	if got := send(t, h, http.MethodGet, stream, "", "", nil); got.Code != http.StatusOK {
		t.Fatalf("direct stream before logout: %d", got.Code)
	}
	if got := send(t, h, http.MethodPost, BasePath+"/auth/logout", "p12", "", nil); got.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", got.Code)
	}
	if got := send(t, h, http.MethodGet, stream, "", "", nil); got.Code != http.StatusUnauthorized {
		t.Errorf("direct stream after logout: %d", got.Code)
	}
	if stops := pf.tc.stops(); !strings.Contains(strings.Join(stops, ","), ps) {
		t.Errorf("logout did not stop %s: %v", ps, stops)
	}

	// The web app and problem responses carry the header too.
	if got := send(t, h, http.MethodGet, "/", "", "", nil); got.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("web app response without Referrer-Policy")
	}
}

func TestStartPlaybackBusy(t *testing.T) {
	pf := newProfileFixture(t)
	m := pf.f.Rated(0)
	pf.f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", `{"format":{"duration":90000000000},"streams":[{"type":"video","codec":"hevc"}]}`, nil)
	pf.tc.busy = true
	rec := send(t, pf.h, http.MethodPost, BasePath+"/items/"+m.ID+"/play", "p0", `{}`, nil)
	if rec.Code != http.StatusServiceUnavailable || problemCode(t, rec) != "transcode_busy" || rec.Header().Get("Retry-After") == "" {
		t.Errorf("busy: %d %v %s", rec.Code, rec.Header(), rec.Body.String())
	}
}

// I2, plan 5.4: stopping a direct-play session ends a Range response "bytes=0-" that is already
// streaming to a client that stopped reading (browser buffer full), within a second.
func TestStopEndsRunningRangeResponse(t *testing.T) {
	pf := newProfileFixture(t)
	m := pf.f.Rated(12)
	// A long film: a sparse file far larger than the socket buffers.
	if err := os.Truncate(filepath.Join(pf.f.MediaDir, filepath.FromSlash(m.RelPath)), 1<<30); err != nil {
		t.Fatal(err)
	}
	id, st := pf.mint(t, "p12", m, "direct")
	closed := make(chan struct{})
	var once sync.Once
	srv := httptest.NewUnstartedServer(pf.h)
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			once.Do(func() { close(closed) })
		}
	}
	srv.Start()
	defer srv.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+BasePath+"/items/"+m.ID+"/stream?st="+url.QueryEscape(st), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=0-")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("range: %d", resp.StatusCode)
	}
	if _, err := io.CopyN(io.Discard, resp.Body, 1<<20); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // the server fills the socket buffers and blocks in a write

	if got := send(t, pf.h, http.MethodDelete, BasePath+"/playback/sessions/"+id, "p12", "", nil); got.Code != http.StatusNoContent {
		t.Fatalf("stop: %d %s", got.Code, got.Body.String())
	}
	// The client does not read on: the server must still close the connection.
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("connection still open a second after the stop")
	}
	n, err := io.Copy(io.Discard, resp.Body) // what was buffered, then the closed connection
	if err == nil {
		t.Errorf("response completed after the stop (%d more bytes)", n)
	}
}
