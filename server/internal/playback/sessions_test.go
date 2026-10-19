package playback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// fakeTranscoder records requests and serves fixed playlists and segments.
type fakeTranscoder struct {
	mu      sync.Mutex
	busy    bool
	started []HLSRequest
	running map[string]*fakeHLS
	stopped []string
	touched int
	backend string
}

type fakeHLS struct {
	t  *fakeTranscoder
	id string
}

func newFakeTranscoder() *fakeTranscoder {
	return &fakeTranscoder{running: map[string]*fakeHLS{}, backend: "software"}
}

func (f *fakeTranscoder) Backend() string { return f.backend }

func (f *fakeTranscoder) Start(_ context.Context, req HLSRequest) (HLSSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.busy {
		return nil, model.ErrTranscodeBusy
	}
	f.started = append(f.started, req)
	s := &fakeHLS{t: f, id: req.ID}
	f.running[req.ID] = s
	return s, nil
}

func (f *fakeTranscoder) Get(id string) (HLSSession, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.running[id]
	return s, ok
}

func (f *fakeTranscoder) Stop(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.running, id)
	f.stopped = append(f.stopped, id)
}

func (h *fakeHLS) MasterPlaylist(mediaURL string) []byte {
	return []byte("#EXTM3U\n" + mediaURL + "\n")
}

func (h *fakeHLS) MediaPlaylist(initURL string, segmentURL func(int) string) []byte {
	return []byte("#EXTM3U\n" + initURL + "\n" + segmentURL(0) + "\n")
}

type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }

func (h *fakeHLS) OpenInit(context.Context) (io.ReadSeekCloser, error) {
	return nopSeekCloser{bytes.NewReader([]byte("init:" + h.id))}, nil
}

func (h *fakeHLS) OpenSegment(_ context.Context, i int) (io.ReadSeekCloser, error) {
	return nopSeekCloser{bytes.NewReader([]byte{byte(i)})}, nil
}

func (h *fakeHLS) Touch() {
	h.t.mu.Lock()
	h.t.touched++
	h.t.mu.Unlock()
}

func browserSession(t *testing.T, f *vistest.Fixture, p model.Profile) model.Session {
	t.Helper()
	return f.Session(t, p)
}

// tokenOf extracts the st parameter of a playback URL.
func tokenOf(t *testing.T, streamURL string) string {
	t.Helper()
	u, err := url.Parse(streamURL)
	if err != nil {
		t.Fatal(err)
	}
	st := u.Query().Get("st")
	if st == "" {
		t.Fatalf("URL %q has no st", streamURL)
	}
	return st
}

// hevcProbe is a stored probe of a 90-s HEVC SDR file.
func hevcProbe(t *testing.T) string {
	t.Helper()
	p := transcode.Probe{Format: transcode.Format{Name: "matroska,webm", Duration: 90 * time.Second}, Streams: []transcode.Stream{
		{Index: 0, Type: "video", Codec: "hevc", Width: 1280, Height: 720, PixFmt: "yuv420p10le", ColorTransfer: "bt709", ColorPrimaries: "bt709"},
		{Index: 1, Type: "audio", Codec: "aac", Channels: 2},
	}}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStreamTokenFormat(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	st := SignToken(key, "01JAAAAAAAAAAAAAAAAAAAAAAA", "item")
	id, mac, ok := parseToken(st)
	if !ok || id != "01JAAAAAAAAAAAAAAAAAAAAAAA" || len(mac) != macLength {
		t.Fatalf("parse %q = %q %x %v", st, id, mac, ok)
	}
	if strings.ContainsAny(st, "+/=") {
		t.Errorf("token %q is not URL-safe", st)
	}
	if SignToken(key, id, "other") == st || SignToken([]byte("another key"), id, "item") == st {
		t.Error("the MAC does not cover the item or the key")
	}
	for _, bad := range []string{"", "nodot", "short.AAAA", "01JAAAAAAAAAAAAAAAAAAAAAAA.!!", "01JAAAAAAAAAAAAAAAAAAAAAAA.AAAA"} {
		if _, _, ok := parseToken(bad); ok {
			t.Errorf("parseToken(%q) accepted", bad)
		}
	}
}

func TestStreamTokenLifecycle(t *testing.T) {
	f := vistest.New(t)
	now := time.Now()
	clock := func() time.Time { return now }
	s := New(f.DB, vistest.Logger(), WithClock(clock), WithTokenKey([]byte("test-key-test-key-test-key-12345")))
	ctx := context.Background()
	m := f.Rated(6)
	session := browserSession(t, f, f.P6)

	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{DeviceID: "tv"})
	if err != nil {
		t.Fatal(err)
	}
	st := tokenOf(t, info.StreamURL)
	g, err := s.Authorize(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	if g.ID != info.PlaybackSessionID || g.ItemID != m.ID || g.MediaFileID != m.FileID || g.Mode != ModeDirect ||
		g.Session.ID != session.ID || g.Session.Profile.ID != f.P6.ID || g.Session.Account.ID != f.Account.ID || g.Session.Profile.MaxRating != 6 {
		t.Errorf("grant = %+v", g)
	}
	src, err := s.OpenGrant(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	_ = src.File.Close()
	if _, err := s.HLS(ctx, g); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("HLS of a direct grant: %v", err)
	}

	// Forged and foreign tokens.
	id, _, _ := parseToken(st)
	for _, bad := range []string{"", "garbage", SignToken([]byte("wrong key"), id, m.ID), SignToken(s.key, id, f.Rated(0).ID), SignToken(s.key, "01JAAAAAAAAAAAAAAAAAAAAAAA", m.ID)} {
		if _, err := s.Authorize(ctx, bad); !errors.Is(err, model.ErrUnauthenticated) {
			t.Errorf("Authorize(%q) = %v, want ErrUnauthenticated", bad, err)
		}
	}

	// Activity slides the expiry: 11 h later still valid, touched; 11 h after that still valid.
	now = now.Add(11 * time.Hour)
	if _, err := s.Authorize(ctx, st); err != nil {
		t.Fatalf("after 11 h: %v", err)
	}
	now = now.Add(11 * time.Hour)
	if _, err := s.Authorize(ctx, st); err != nil {
		t.Fatalf("after 22 h with activity: %v", err)
	}
	// 12 h without a request: expired.
	now = now.Add(SessionLifetime + time.Second)
	if _, err := s.Authorize(ctx, st); !errors.Is(err, model.ErrUnauthenticated) {
		t.Errorf("expired: %v", err)
	}
}

func TestStopAndLogoutRevoke(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc))
	ctx := context.Background()
	m := f.Rated(12)
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", hevcProbe(t), nil)
	mine := browserSession(t, f, f.P18)
	other := browserSession(t, f, f.P18)

	a, err := s.Start(ctx, mine, m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Start(ctx, mine, m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Mode != "hls" || a.Transcoding == nil || a.Transcoding.Mode != ModeTranscode || a.Transcoding.Backend != "software" || a.Transcoding.Reason != ReasonVideoCodec {
		t.Fatalf("HEVC on an H.264-only client = %+v %+v", a, a.Transcoding)
	}
	if want := "/api/v1/playback/sessions/" + a.PlaybackSessionID + "/master.m3u8?st="; !strings.HasPrefix(a.StreamURL, want) {
		t.Errorf("stream URL = %q", a.StreamURL)
	}

	// Another browser session cannot stop it.
	if err := s.Stop(ctx, other, a.PlaybackSessionID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("stop by another session: %v", err)
	}
	if _, err := s.Authorize(ctx, tokenOf(t, a.StreamURL)); err != nil {
		t.Fatalf("still valid after a foreign stop: %v", err)
	}
	if err := s.Stop(ctx, mine, a.PlaybackSessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authorize(ctx, tokenOf(t, a.StreamURL)); !errors.Is(err, model.ErrUnauthenticated) {
		t.Errorf("after stop: %v", err)
	}
	if _, running := tc.Get(a.PlaybackSessionID); running {
		t.Error("transcode still running after stop")
	}
	if err := s.Stop(ctx, mine, a.PlaybackSessionID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("second stop: %v", err)
	}

	// Logout revokes the rest and stops their output.
	if err := s.EndSession(ctx, mine.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authorize(ctx, tokenOf(t, b.StreamURL)); !errors.Is(err, model.ErrUnauthenticated) {
		t.Errorf("after logout: %v", err)
	}
	if _, running := tc.Get(b.PlaybackSessionID); running {
		t.Error("transcode still running after logout")
	}
}

func TestRevokedBrowserSessionInvalidatesTokens(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, vistest.Logger())
	ctx := context.Background()
	session := browserSession(t, f, f.P18)
	info, err := s.Start(ctx, session, f.Rated(18).ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Write.ExecContext(ctx, `UPDATE sessions SET revoked_at = 1 WHERE id = ?`, session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authorize(ctx, tokenOf(t, info.StreamURL)); !errors.Is(err, model.ErrUnauthenticated) {
		t.Errorf("token of a revoked browser session: %v", err)
	}
}

func TestStartHLSRequestAndVisibilityRecheck(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc))
	ctx := context.Background()
	m := f.Rated(6)
	kf := []time.Duration{0, 2 * time.Second, 7 * time.Second, 8 * time.Second}
	f.SetMediaFile(t, m.FileID, "mkv", "h264", "aac", hevcProbe(t), library.EncodeKeyframes(kf))
	session := browserSession(t, f, f.P6)

	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &model.PlaybackCapabilities{Video: []string{"h264"}, Audio: []string{"aac"}, HLS: "mse"}})
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode != "hls" || info.Transcoding.Mode != ModeRemux || info.Transcoding.Backend != "" || info.Transcoding.Reason != ReasonContainer {
		t.Fatalf("H.264 MKV = %+v %+v", info, info.Transcoding)
	}
	if len(tc.started) != 1 {
		t.Fatalf("started %d outputs", len(tc.started))
	}
	req := tc.started[0]
	if req.ID != info.PlaybackSessionID || !filepath.IsAbs(req.Path) || filepath.Base(req.Path) != filepath.Base(m.RelPath) ||
		len(req.Keyframes) != len(kf) || req.Keyframes[2] != 7*time.Second || req.Probe.Format.Duration != 90*time.Second ||
		req.Decision != (Decision{Mode: ModeRemux, VideoCopy: true, AudioCopy: true, Reason: ReasonContainer}) {
		t.Errorf("request = %+v", req)
	}

	g, err := s.Authorize(ctx, tokenOf(t, info.StreamURL))
	if err != nil {
		t.Fatal(err)
	}
	hs, err := s.HLS(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(hs.MasterPlaylist("media.m3u8")); !strings.Contains(got, "media.m3u8") {
		t.Errorf("master = %q", got)
	}
	if _, err := s.OpenGrant(ctx, g); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("Range stream with an HLS grant: %v", err)
	}

	// The title is raised above the profile: the next request answers 404 although the token is valid.
	sixteen := 16
	f.SetRating(t, m.ID, &sixteen)
	if _, err := s.Authorize(ctx, tokenOf(t, info.StreamURL)); err != nil {
		t.Fatalf("token itself stays valid: %v", err)
	}
	if _, err := s.HLS(ctx, g); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("HLS of a title that became invisible: %v", err)
	}
}

func TestStartFallsBackAndReportsBusy(t *testing.T) {
	f := vistest.New(t)
	ctx := context.Background()
	m := f.Rated(0)
	session := browserSession(t, f, f.P0)

	// Not probed yet: direct play, the client decides.
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc))
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", "", nil)
	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{})
	if err != nil || info.Mode != ModeDirect || len(tc.started) != 0 {
		t.Fatalf("unprobed HEVC = %+v, %v", info, err)
	}
	// No transcoder: direct play as well.
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", hevcProbe(t), nil)
	if info, err := New(f.DB, vistest.Logger()).Start(ctx, session, m.ID, model.PlaybackRequest{}); err != nil || info.Mode != ModeDirect {
		t.Fatalf("without transcoder = %+v, %v", info, err)
	}
	// Budget exhausted: 503 and the playback session is revoked.
	tc.busy = true
	if _, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{}); !errors.Is(err, model.ErrTranscodeBusy) {
		t.Fatalf("busy: %v", err)
	}
	var open int
	if err := f.DB.Read.QueryRowContext(ctx, `SELECT count(*) FROM playback_sessions WHERE mode = 'transcode' AND revoked_at IS NULL`).Scan(&open); err != nil {
		t.Fatal(err)
	}
	if open != 0 {
		t.Errorf("%d transcode sessions left open after ErrTranscodeBusy", open)
	}
	// Invisible title.
	if _, err := s.Start(ctx, session, f.Rated(6).ID, model.PlaybackRequest{}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible: %v", err)
	}
	var verr *model.ValidationError
	if _, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{DeviceID: strings.Repeat("x", 65)}); !errors.As(err, &verr) {
		t.Errorf("long device id: %v", err)
	}
}

func TestSweepStopsEndedOutputAndDeletesOldRows(t *testing.T) {
	f := vistest.New(t)
	now := time.Now()
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithClock(func() time.Time { return now }))
	ctx := context.Background()
	m := f.Rated(18)
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", hevcProbe(t), nil)
	session := browserSession(t, f, f.P18)
	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	// The client vanished: 13 h later the session is expired; the next Start sweeps its output.
	now = now.Add(13 * time.Hour)
	if _, err := s.Start(ctx, browserSession(t, f, f.P18), f.Rated(0).ID, model.PlaybackRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, running := tc.Get(info.PlaybackSessionID); running {
		t.Error("output of an expired session still runs after the sweep")
	}
	// A day later the row is gone.
	now = now.Add(2 * 24 * time.Hour)
	if _, err := s.Start(ctx, browserSession(t, f, f.P18), f.Rated(0).ID, model.PlaybackRequest{}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := f.DB.Read.QueryRowContext(ctx, `SELECT count(*) FROM playback_sessions WHERE id = ?`, info.PlaybackSessionID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("expired row not deleted")
	}
}
