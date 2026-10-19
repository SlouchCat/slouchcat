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

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// fakeTranscoder records requests and serves fixed playlists and segments. Like the manager it
// rejects a remux without keyframe index (ErrNoKeyframes); rejectRemux rejects every remux.
type fakeTranscoder struct {
	mu          sync.Mutex
	busy        bool
	rejectRemux bool
	onStart     func(HLSRequest) // called before Start returns, outside the lock
	started     []HLSRequest
	running     map[string]*fakeHLS
	stopped     []string
	touched     int
	backend     string
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
	if f.busy {
		f.mu.Unlock()
		return nil, model.ErrTranscodeBusy
	}
	if req.Decision.Mode == ModeRemux && (f.rejectRemux || len(req.Keyframes) == 0) {
		f.mu.Unlock()
		return nil, ErrNoKeyframes
	}
	f.started = append(f.started, req)
	s := &fakeHLS{t: f, id: req.ID}
	f.running[req.ID] = s
	hook := f.onStart
	f.mu.Unlock()
	if hook != nil {
		hook(req)
	}
	return s, nil
}

// lose drops an output without a stop, like a server restart.
func (f *fakeTranscoder) lose(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.running, id)
}

func (f *fakeTranscoder) starts() []HLSRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]HLSRequest(nil), f.started...)
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

// hevcProbe is a stored probe of a 90-s HEVC 10-bit SDR file.
func hevcProbe(t *testing.T) string {
	t.Helper()
	return probeJSON(t, "hevc", "yuv420p10le")
}

// h264Probe is a stored probe of a 90-s H.264 8-bit file.
func h264Probe(t *testing.T) string {
	t.Helper()
	return probeJSON(t, "h264", "yuv420p")
}

func probeJSON(t *testing.T, codec, pixFmt string) string {
	t.Helper()
	p := transcode.Probe{Format: transcode.Format{Name: "matroska,webm", Duration: 90 * time.Second}, Streams: []transcode.Stream{
		{Index: 0, Type: "video", Codec: codec, Width: 1280, Height: 720, PixFmt: pixFmt, ColorTransfer: "bt709", ColorPrimaries: "bt709"},
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
	if _, err := s.HLS(ctx, g, 0); !errors.Is(err, model.ErrNotFound) {
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
	f.SetMediaFile(t, m.FileID, "mkv", "h264", "aac", h264Probe(t), library.EncodeKeyframes(kf))
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
	hs, err := s.HLS(ctx, g, 0)
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
	if _, err := s.HLS(ctx, g, 0); !errors.Is(err, model.ErrNotFound) {
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
	info, err := s.Start(ctx, browserSession(t, f, f.P18), m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	loggedOut := browserSession(t, f, f.P18)
	other, err := s.Start(ctx, loggedOut, m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	// Start does not sweep; the background sweep does.
	s.sweep(ctx)
	if _, running := tc.Get(info.PlaybackSessionID); !running {
		t.Fatal("sweep stopped a valid session")
	}
	// The login of the second playback is revoked elsewhere (not through EndSession): the next
	// sweep stops its output.
	if _, err := f.DB.Write.ExecContext(ctx, `UPDATE sessions SET revoked_at = 1 WHERE id = ?`, loggedOut.ID); err != nil {
		t.Fatal(err)
	}
	s.sweep(ctx)
	if _, running := tc.Get(other.PlaybackSessionID); running {
		t.Error("output of a revoked browser session still runs after the sweep")
	}
	if _, running := tc.Get(info.PlaybackSessionID); !running {
		t.Fatal("sweep stopped a valid session")
	}
	// The client vanished: 13 h later the session is expired; the sweep stops its output.
	now = now.Add(13 * time.Hour)
	s.sweep(ctx)
	if _, running := tc.Get(info.PlaybackSessionID); running {
		t.Error("output of an expired session still runs after the sweep")
	}
	// A day later the row is gone.
	now = now.Add(2 * 24 * time.Hour)
	s.sweep(ctx)
	var n int
	if err := f.DB.Read.QueryRowContext(ctx, `SELECT count(*) FROM playback_sessions WHERE id = ?`, info.PlaybackSessionID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("expired row not deleted")
	}
}

func TestRunSweepsUntilCancelled(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc))
	ctx, cancel := context.WithCancel(context.Background())
	m := f.Rated(18)
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", hevcProbe(t), nil)
	session := browserSession(t, f, f.P18)
	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Write.ExecContext(ctx, `UPDATE sessions SET revoked_at = 1 WHERE id = ?`, session.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, running := tc.Get(info.PlaybackSessionID); !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Run did not sweep at start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// fakeIndex is a keyframe indexer for Start: it returns kf after delay, or the context error.
type fakeIndex struct {
	mu    sync.Mutex
	calls int
	kf    []time.Duration
	delay time.Duration
}

func (x *fakeIndex) keyframes(ctx context.Context, _ string) ([]time.Duration, error) {
	x.mu.Lock()
	x.calls++
	x.mu.Unlock()
	select {
	case <-time.After(x.delay):
		return x.kf, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func keyframeJob(t *testing.T, f *vistest.Fixture, fileID string) (priority int, state string) {
	t.Helper()
	err := f.DB.Read.QueryRowContext(context.Background(),
		`SELECT priority, state FROM jobs WHERE idempotency_key = ? AND state IN ('queued', 'running')`, "keyframes:"+fileID).Scan(&priority, &state)
	if err != nil {
		t.Fatalf("keyframe job of %s: %v", fileID, err)
	}
	return priority, state
}

// I1: a remux without keyframe index computes it on demand and stores it for the next start.
func TestRemuxComputesMissingKeyframes(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	idx := &fakeIndex{kf: []time.Duration{0, 5 * time.Second, 10 * time.Second}}
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithKeyframeIndex(idx.keyframes))
	ctx := context.Background()
	m := f.Rated(6)
	f.SetMediaFile(t, m.FileID, "mkv", "h264", "aac", h264Probe(t), nil)
	session := browserSession(t, f, f.P6)

	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if info.Transcoding == nil || info.Transcoding.Mode != ModeRemux || info.Transcoding.Reason != ReasonContainer {
		t.Fatalf("H.264 MKV without index = %+v", info.Transcoding)
	}
	if got := tc.starts(); len(got) != 1 || len(got[0].Keyframes) != 3 || got[0].Keyframes[1] != 5*time.Second {
		t.Fatalf("started = %+v", got)
	}
	pf, err := visibility.New(f.DB, vistest.Logger()).MediaFileProbe(ctx, visibility.Profile(f.P6), m.ID, m.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if kf, err := library.DecodeKeyframes(pf.Keyframes); err != nil || len(kf) != 3 {
		t.Errorf("stored index = %v, %v", kf, err)
	}
	// The next start uses the stored index.
	if _, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{}); err != nil {
		t.Fatal(err)
	}
	if idx.calls != 1 {
		t.Errorf("index computed %d times", idx.calls)
	}
}

// I1: an index that takes longer than the timeout falls back to transcoding and queues the index
// job at top priority, replacing the scan's low-priority job.
func TestRemuxFallsBackWhileKeyframesPending(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	idx := &fakeIndex{kf: []time.Duration{0}, delay: time.Hour}
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithKeyframeIndex(idx.keyframes))
	s.keyframeTimeout = 50 * time.Millisecond
	ctx := context.Background()
	m := f.Rated(6)
	f.SetMediaFile(t, m.FileID, "mkv", "h264", "ac3", h264Probe(t), nil)
	if _, err := jobs.NewQueue(f.DB.Write, nil).Enqueue(ctx, jobs.NewJob{Type: library.JobProbeKeyframes, Priority: 1,
		IdempotencyKey: "keyframes:" + m.FileID, Payload: []byte(`{"mediaFileId":"` + m.FileID + `"}`)}); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	info, err := s.Start(ctx, browserSession(t, f, f.P6), m.ID, model.PlaybackRequest{Capabilities: &model.PlaybackCapabilities{Video: []string{"h264"}, Audio: []string{"aac", "ac3"}, HLS: "native"}})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("start took %v", time.Since(start))
	}
	if info.Transcoding == nil || info.Transcoding.Mode != ModeTranscode || info.Transcoding.Reason != ReasonKeyframesPending || info.Transcoding.Backend != "software" {
		t.Fatalf("pending index = %+v", info.Transcoding)
	}
	got := tc.starts()
	if len(got) != 1 || got[0].Decision != (Decision{Mode: ModeTranscode, MaxHeight: 1080, Reason: ReasonKeyframesPending}) || got[0].Keyframes != nil {
		t.Errorf("started = %+v", got)
	}
	if prio, state := keyframeJob(t, f, m.FileID); prio != 9 || state != "queued" {
		t.Errorf("keyframe job = %d %s, want 9 queued", prio, state)
	}
	var row struct{ mode, reason string }
	if err := f.DB.Read.QueryRowContext(ctx, `SELECT mode, reason FROM playback_sessions WHERE id = ?`, info.PlaybackSessionID).Scan(&row.mode, &row.reason); err != nil {
		t.Fatal(err)
	}
	if row.mode != ModeTranscode || row.reason != ReasonKeyframesPending {
		t.Errorf("row = %+v", row)
	}
}

// I1 safety net: the transcoder rejects a remux (transcode.ErrNoKeyframes); Start transcodes.
func TestRemuxRejectedByTranscoderFallsBack(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	tc.rejectRemux = true
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc))
	ctx := context.Background()
	m := f.Rated(6)
	f.SetMediaFile(t, m.FileID, "mkv", "h264", "aac", h264Probe(t), library.EncodeKeyframes([]time.Duration{0, 4 * time.Second}))
	info, err := s.Start(ctx, browserSession(t, f, f.P6), m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if info.Transcoding == nil || info.Transcoding.Mode != ModeTranscode || info.Transcoding.Reason != ReasonKeyframesPending {
		t.Fatalf("rejected remux = %+v", info.Transcoding)
	}
	if got := tc.starts(); len(got) != 1 || got[0].Decision.Mode != ModeTranscode || !got[0].Decision.AudioCopy {
		t.Errorf("started = %+v", got)
	}
	if prio, _ := keyframeJob(t, f, m.FileID); prio != 9 {
		t.Errorf("keyframe job priority %d", prio)
	}
}

// I6: the output starts at the profile's resume position; without progress at 0.
func TestStartPassesResumePosition(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc))
	ctx := context.Background()
	m := f.Rated(12) // P18 is at 1/6 of the runtime
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", hevcProbe(t), nil)
	info, err := s.Start(ctx, browserSession(t, f, f.P18), m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	want := time.Duration(vistest.DurationMs/6) * time.Millisecond
	if got := tc.starts(); len(got) != 1 || got[0].Resume != want || info.ResumePositionMs != vistest.DurationMs/6 {
		t.Errorf("resume = %+v, info %d", got, info.ResumePositionMs)
	}
	if _, err := s.Start(ctx, browserSession(t, f, f.P12), m.ID, model.PlaybackRequest{}); err != nil {
		t.Fatal(err)
	}
	if got := tc.starts(); len(got) != 2 || got[1].Resume != 0 || got[1].StartSegment != 0 {
		t.Errorf("without progress = %+v", got[1])
	}
}

// M6: a logout while the output waited for admission stops it again.
func TestStartStopsOutputAfterLogoutDuringAdmission(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc))
	ctx := context.Background()
	m := f.Rated(18)
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", hevcProbe(t), nil)
	session := browserSession(t, f, f.P18)
	tc.onStart = func(HLSRequest) {
		if _, err := f.DB.Write.ExecContext(ctx, `UPDATE sessions SET revoked_at = 1 WHERE id = ?`, session.ID); err != nil {
			t.Error(err)
		}
	}
	if _, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{}); !errors.Is(err, model.ErrUnauthenticated) {
		t.Fatalf("start across a logout: %v", err)
	}
	if got := tc.starts(); len(got) != 1 {
		t.Fatalf("started %d", len(got))
	}
	if _, running := tc.Get(tc.starts()[0].ID); running {
		t.Error("output still runs after the logout")
	}
}

// M7: a valid HLS grant whose output is gone (server restart) rebuilds it at the requested segment.
func TestHLSRebuildsLostOutput(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc))
	ctx := context.Background()
	hevc := f.Rated(18)
	f.SetMediaFile(t, hevc.FileID, "mkv", "hevc", "aac", hevcProbe(t), nil)
	h264 := f.Rated(6)
	f.SetMediaFile(t, h264.FileID, "mkv", "h264", "ac3", h264Probe(t), library.EncodeKeyframes([]time.Duration{0, 4 * time.Second, 9 * time.Second}))
	safari := &model.PlaybackCapabilities{Video: []string{"h264", "hevc"}, Audio: []string{"aac", "ac3"}, HLS: "native"}

	a, err := s.Start(ctx, browserSession(t, f, f.P18), hevc.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Start(ctx, browserSession(t, f, f.P18), h264.ID, model.PlaybackRequest{Capabilities: safari})
	if err != nil {
		t.Fatal(err)
	}
	if b.Transcoding.Mode != ModeRemux || !tc.starts()[1].Decision.AudioCopy {
		t.Fatalf("safari remux = %+v %+v", b.Transcoding, tc.starts()[1])
	}
	for _, info := range []model.PlaybackInfo{a, b} {
		tc.lose(info.PlaybackSessionID)
	}

	ga, err := s.Authorize(ctx, tokenOf(t, a.StreamURL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.HLS(ctx, ga, 7); err != nil {
		t.Fatalf("rebuild transcode: %v", err)
	}
	gb, err := s.Authorize(ctx, tokenOf(t, b.StreamURL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.HLS(ctx, gb, 0); err != nil {
		t.Fatalf("rebuild remux: %v", err)
	}
	got := tc.starts()
	if len(got) != 4 {
		t.Fatalf("started %d outputs", len(got))
	}
	ra, rb := got[2], got[3]
	if ra.ID != a.PlaybackSessionID || ra.StartSegment != 7 || ra.Resume != 0 ||
		ra.Decision != (Decision{Mode: ModeTranscode, MaxHeight: 1080, AudioCopy: true}) {
		t.Errorf("rebuilt transcode = %+v", ra)
	}
	if rb.ID != b.PlaybackSessionID || len(rb.Keyframes) != 3 ||
		rb.Decision != (Decision{Mode: ModeRemux, VideoCopy: true, AudioCopy: false}) {
		t.Errorf("rebuilt remux = %+v", rb)
	}
	// A running output is not started again.
	if _, err := s.HLS(ctx, ga, 3); err != nil || len(tc.starts()) != 4 {
		t.Errorf("second request: %v, %d starts", err, len(tc.starts()))
	}
	// A stopped playback is not rebuilt: its token is refused before HLS is asked.
	if err := s.Stop(ctx, ga.Session, ga.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authorize(ctx, tokenOf(t, a.StreamURL)); !errors.Is(err, model.ErrUnauthenticated) {
		t.Errorf("after stop: %v", err)
	}
	// Concurrent requests of a lost output rebuild it once.
	tc.lose(gb.ID)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := s.HLS(ctx, gb, 2); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := len(tc.starts()); n != 5 {
		t.Errorf("concurrent rebuild started %d outputs, want 1 more", n-4)
	}
}

// I2: revoking a direct-play session ends a stream that is already being read within a second,
// whether by Stop or by a revocation the stream notices itself (re-read of the row).
func TestRevokeEndsOpenStream(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, vistest.Logger())
	s.grantRecheck = 100 * time.Millisecond
	ctx := context.Background()
	m := f.Rated(18)

	open := func(session model.Session) (model.PlaybackGrant, chan error) {
		t.Helper()
		info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{})
		if err != nil {
			t.Fatal(err)
		}
		g, err := s.Authorize(ctx, tokenOf(t, info.StreamURL))
		if err != nil {
			t.Fatal(err)
		}
		src, err := s.OpenGrant(ctx, g)
		if err != nil {
			t.Fatal(err)
		}
		ended := make(chan error, 1)
		go func() {
			defer src.File.Close()
			buf := make([]byte, 512)
			for {
				_, err := src.File.Read(buf)
				if errors.Is(err, io.EOF) { // a long film: start over
					_, err = src.File.Seek(0, io.SeekStart)
				}
				if err != nil {
					ended <- err
					return
				}
				time.Sleep(time.Millisecond)
			}
		}()
		return g, ended
	}
	expectEnd := func(name string, ended chan error) {
		t.Helper()
		select {
		case err := <-ended:
			if !errors.Is(err, model.ErrUnauthenticated) {
				t.Errorf("%s: read ended with %v", name, err)
			}
		case <-time.After(time.Second):
			t.Errorf("%s: stream still open a second after the revocation", name)
		}
	}

	g, ended := open(browserSession(t, f, f.P18))
	time.Sleep(50 * time.Millisecond)
	select {
	case err := <-ended:
		t.Fatalf("stream ended early: %v", err)
	default:
	}
	if err := s.Stop(ctx, g.Session, g.ID); err != nil {
		t.Fatal(err)
	}
	expectEnd("stop", ended)

	session := browserSession(t, f, f.P18)
	_, ended = open(session)
	if err := s.EndSession(ctx, session.ID); err != nil {
		t.Fatal(err)
	}
	expectEnd("logout", ended)

	// Revoked elsewhere (the login ended): the stream re-reads its row and stops.
	session = browserSession(t, f, f.P18)
	_, ended = open(session)
	if _, err := f.DB.Write.ExecContext(ctx, `UPDATE sessions SET revoked_at = 1 WHERE id = ?`, session.ID); err != nil {
		t.Fatal(err)
	}
	expectEnd("ended login", ended)

	// The reader goroutine reports the end first and closes its file right after (deferred), so the
	// registry empties a moment later.
	var left int
	for deadline := time.Now().Add(time.Second); ; {
		s.streamsMu.Lock()
		left = len(s.streams)
		s.streamsMu.Unlock()
		if left == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if left != 0 {
		t.Errorf("%d playback ids still registered after close", left)
	}
}

// EndAccount (guest revocation) and EndProfile (profile deleted) revoke the playback sessions of
// the account or profile only, stop their output and answer 401 for the tokens at once; a deleted
// account takes its playback sessions with it (the join with sessions finds nothing).
func TestEndAccountAndEndProfile(t *testing.T) {
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddGuest(t)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc))
	ctx := context.Background()
	m := f.Rated(6)
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", hevcProbe(t), nil)

	start := func(session model.Session) model.PlaybackInfo {
		t.Helper()
		info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{})
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	alive := func(info model.PlaybackInfo) bool {
		t.Helper()
		_, err := s.Authorize(ctx, tokenOf(t, info.StreamURL))
		if err != nil && !errors.Is(err, model.ErrUnauthenticated) {
			t.Fatal(err)
		}
		return err == nil
	}

	guestSession := browserSession(t, f, f.PG)
	g1, g2 := start(guestSession), start(browserSession(t, f, f.PG18))
	p18, p6 := start(browserSession(t, f, f.P18)), start(browserSession(t, f, f.P6))
	for name, info := range map[string]model.PlaybackInfo{"g1": g1, "g2": g2, "p18": p18, "p6": p6} {
		if !alive(info) {
			t.Fatalf("%s not valid before", name)
		}
	}

	// A profile: only its playback ends.
	if err := s.EndProfile(ctx, f.P6.ID); err != nil {
		t.Fatal(err)
	}
	if alive(p6) || !alive(p18) || !alive(g1) {
		t.Errorf("EndProfile: p6 alive %v, p18 alive %v, g1 alive %v", alive(p6), alive(p18), alive(g1))
	}
	if _, running := tc.Get(p6.PlaybackSessionID); running {
		t.Error("output of the profile still running")
	}

	// An account: every profile of it, nothing of the owner.
	if err := s.EndAccount(ctx, f.Guest.ID); err != nil {
		t.Fatal(err)
	}
	if alive(g1) || alive(g2) || !alive(p18) {
		t.Errorf("EndAccount: g1 alive %v, g2 alive %v, p18 alive %v", alive(g1), alive(g2), alive(p18))
	}
	for _, info := range []model.PlaybackInfo{g1, g2} {
		if _, running := tc.Get(info.PlaybackSessionID); running {
			t.Errorf("output %s of the account still running", info.PlaybackSessionID)
		}
	}
	// Nothing to end is not an error.
	if err := s.EndAccount(ctx, 9999); err != nil {
		t.Errorf("EndAccount of an unknown account: %v", err)
	}

	// Deleting the account removes the browser sessions: tokens that were not even revoked answer 401.
	again := start(guestSession)
	if !alive(again) {
		t.Fatal("new guest playback not valid")
	}
	if _, err := f.DB.Write.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, f.Guest.ID); err != nil {
		t.Fatal(err)
	}
	if alive(again) {
		t.Error("a token of a deleted account is still valid")
	}
}
