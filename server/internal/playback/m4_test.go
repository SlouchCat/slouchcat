package playback

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Tests of M4 (design sections 3, 5, 6, 8): tracks and WebVTT subtitles (W-4), the W-8 limits and
// the quality ladder, the K-5 watch-time limit, device profiles (W-7) and the stream overview
// (W-6).

const (
	deviceA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	deviceB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	deviceC = "cccccccccccccccccccccccccccccccc"
)

var testKey = []byte("m4-test-key-m4-test-key-m4-test!!")

// trackProbe is a stored probe matching vistest.AddTracks: video, AAC (default) and AC3 5.1,
// SubRip and PGS, two chapters, two hours.
func trackProbe(t *testing.T, codec, transfer string, w, h int, bitrate int64) string {
	t.Helper()
	p := transcode.Probe{
		Format: transcode.Format{Name: "mov,mp4,m4a,3gp,3g2,mj2", Duration: 2 * time.Hour, BitRate: bitrate},
		Streams: []transcode.Stream{
			{Index: 0, Type: "video", Codec: codec, Width: w, Height: h, PixFmt: "yuv420p", ColorTransfer: transfer},
			{Index: 1, Type: "audio", Codec: "aac", Channels: 2, Default: true, Language: "de"},
			{Index: 2, Type: "audio", Codec: "ac3", Channels: 6, Language: "en"},
			{Index: 3, Type: "subtitle", Codec: "subrip", Language: "en"},
			{Index: 4, Type: "subtitle", Codec: "hdmv_pgs_subtitle", Language: "de"},
		},
		Chapters: []transcode.Chapter{{Start: 0, End: time.Hour, Title: "Teil 1"}, {Start: time.Hour, End: 2 * time.Hour, Title: "Teil 2"}},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var testKeyframes = library.EncodeKeyframes([]time.Duration{0, 4 * time.Second, 8 * time.Second})

// capsChromeLinux is a desktop Chrome capability profile with name and type.
var capsChromeLinux = model.PlaybackCapabilities{Containers: []string{"mp4", "webm"}, Video: []string{"h264", "vp9"},
	Audio: []string{"aac", "opus"}, HLS: "mse", MaxWidth: 2560, MaxHeight: 1440, Name: "Chrome 156 · Linux", Type: "desktop"}

func stoppedBy(t *testing.T, f *vistest.Fixture, id string) string {
	t.Helper()
	var by *string
	if err := f.DB.Read.QueryRow(`SELECT stopped_by FROM playback_sessions WHERE id = ?`, id).Scan(&by); err != nil {
		t.Fatal(err)
	}
	if by == nil {
		return ""
	}
	return *by
}

func readAll(t *testing.T, src model.StreamSource) string {
	t.Helper()
	defer src.File.Close()
	b, err := io.ReadAll(src.File)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type fakeConverter struct {
	mu    sync.Mutex
	calls []string
}

func (c *fakeConverter) embedded(_ context.Context, path string, idx int) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, filepath.Base(path))
	return []byte("WEBVTT\n\nembedded " + string(rune('0'+idx)) + "\n"), nil
}

func (c *fakeConverter) sidecar(_ context.Context, path string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, filepath.Base(path))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return append([]byte("WEBVTT\n\n"), data...), nil
}

func (c *fakeConverter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls)
}

// W-4: audio and subtitle tracks, text subtitles as WebVTT (cached per playback), bitmap and
// requested burn-in as transcode, validation of the wanted tracks.
func TestStartTracksAndSubtitles(t *testing.T) {
	f := vistest.New(t)
	tracks := f.AddTracks(t)
	m := f.Rated(6)
	ft := tracks[m.FileID]
	f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000), testKeyframes)
	tc := newFakeTranscoder()
	conv := &fakeConverter{}
	cache := t.TempDir()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithTokenKey(testKey), WithSubtitleConverter(conv.embedded, conv.sidecar), WithCacheDir(cache))
	ctx := context.Background()
	session := browserSession(t, f, f.P6)
	caps := capsChromeLinux

	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &caps, SubtitleTrack: ft.Text})
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode != ModeDirect || info.Selection != (model.TrackSelection{AudioTrack: 1, SubtitleTrack: ft.Text, SubtitleDelivery: model.SubtitleText}) {
		t.Fatalf("text subtitle = %s %+v", info.Mode, info.Selection)
	}
	if want := "/api/v1/playback/sessions/" + info.PlaybackSessionID + "/subtitles/"; !strings.HasPrefix(info.SubtitleURL, want) ||
		!strings.Contains(info.SubtitleURL, ".vtt?st=") {
		t.Errorf("subtitle URL = %q", info.SubtitleURL)
	}
	if len(info.Tracks.Audio) != 2 || info.Tracks.Audio[0].Channels != 2 || info.Tracks.Audio[1].Channels != 6 || !info.Tracks.Audio[0].Default ||
		len(info.Tracks.Subtitles) != 3 {
		t.Fatalf("tracks = %+v", info.Tracks)
	}
	for _, st := range info.Tracks.Subtitles {
		switch st.ID {
		case ft.Text:
			if !st.Text || st.Index != 3 || st.Source != "embedded" {
				t.Errorf("text track = %+v", st)
			}
		case ft.Bitmap:
			if st.Text || st.Index != 4 {
				t.Errorf("bitmap track = %+v", st)
			}
		case ft.Sidecar:
			if !st.Text || st.Index != -1 || st.Source != "sidecar" || st.Language != "de" || st.RelPath != ft.SidecarRel {
				t.Errorf("sidecar track = %+v", st)
			}
		}
	}
	if len(info.Chapters) != 2 || info.Chapters[1] != (model.Chapter{StartMs: 3_600_000, EndMs: 7_200_000, Title: "Teil 2"}) {
		t.Errorf("chapters = %+v", info.Chapters)
	}
	if info.MediaFile.Bitrate != 6_000_000 || info.MediaFile.HDR != "" {
		t.Errorf("media file = %+v", info.MediaFile)
	}
	g, err := s.Authorize(ctx, tokenOf(t, info.SubtitleURL))
	if err != nil || g.SubtitleTrack != ft.Text {
		t.Fatalf("grant = %+v, %v", g, err)
	}
	src, err := s.Subtitle(ctx, g, ft.Text)
	if err != nil {
		t.Fatal(err)
	}
	if src.MediaType != "text/vtt; charset=utf-8" || readAll(t, src) != "WEBVTT\n\nembedded 3\n" {
		t.Errorf("subtitle = %+v", src)
	}
	cached := filepath.Join(cache, "subtitles", info.PlaybackSessionID, strconv.FormatInt(ft.Text, 10)+".vtt")
	if _, err := os.Stat(cached); err != nil {
		t.Errorf("cache file: %v", err)
	}
	if src, err := s.Subtitle(ctx, g, ft.Text); err != nil || readAll(t, src) != "WEBVTT\n\nembedded 3\n" || conv.count() != 1 {
		t.Errorf("second request: %v, %d conversions", err, conv.count())
	}
	for _, other := range []int64{ft.Sidecar, ft.Bitmap, 0, 999999} {
		if _, err := s.Subtitle(ctx, g, other); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("track %d of the grant: %v", other, err)
		}
	}
	if err := s.Stop(ctx, session, info.PlaybackSessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache, "subtitles", info.PlaybackSessionID)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("cache after stop: %v", err)
	}
	if stoppedBy(t, f, info.PlaybackSessionID) != model.StoppedByClient {
		t.Errorf("stopped_by = %q", stoppedBy(t, f, info.PlaybackSessionID))
	}

	// The sidecar as text, converted from its file below the library root.
	info, err = s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &caps, SubtitleTrack: ft.Sidecar, SubtitleDelivery: model.SubtitleText})
	if err != nil {
		t.Fatal(err)
	}
	g, err = s.Authorize(ctx, tokenOf(t, info.SubtitleURL))
	if err != nil {
		t.Fatal(err)
	}
	if src, err := s.Subtitle(ctx, g, ft.Sidecar); err != nil || readAll(t, src) != "WEBVTT\n\n"+vistest.SidecarContent {
		t.Errorf("sidecar: %v", err)
	}

	// Bitmap subtitles are burned in: a transcode with the stream; text delivery is refused.
	if _, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &caps, SubtitleTrack: ft.Bitmap, SubtitleDelivery: model.SubtitleText}); !hasField(err, "subtitleDelivery", "not_text") {
		t.Errorf("text delivery of a bitmap track: %v", err)
	}
	info, err = s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &caps, SubtitleTrack: ft.Bitmap})
	if err != nil {
		t.Fatal(err)
	}
	last := tc.starts()[len(tc.starts())-1]
	if info.Transcoding == nil || info.Transcoding.Reason != ReasonBurnIn || info.SubtitleURL != "" ||
		info.Selection.SubtitleDelivery != model.SubtitleBurn || last.Decision.BurnStream != 4 || last.BurnInPath != "" {
		t.Errorf("bitmap = %+v %+v %+v", info.Transcoding, info.Selection, last.Decision)
	}
	if g, err := s.Authorize(ctx, tokenOf(t, info.StreamURL)); err != nil || g.SubtitleTrack != 0 {
		t.Errorf("burn grant = %+v, %v", g, err)
	}
	// A sidecar burned in on request: the transcoder gets its absolute path.
	info, err = s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &caps, SubtitleTrack: ft.Sidecar, SubtitleDelivery: model.SubtitleBurn})
	if err != nil {
		t.Fatal(err)
	}
	last = tc.starts()[len(tc.starts())-1]
	if info.Transcoding == nil || info.Transcoding.Reason != ReasonBurnIn || last.BurnInPath != filepath.Join(f.MediaDir, filepath.FromSlash(ft.SidecarRel)) {
		t.Errorf("sidecar burn = %+v, path %q", info.Transcoding, last.BurnInPath)
	}

	// Another audio track: remux with that track.
	alt := 2
	info, err = s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &caps, AudioTrack: &alt})
	if err != nil {
		t.Fatal(err)
	}
	last = tc.starts()[len(tc.starts())-1]
	if info.Transcoding == nil || info.Transcoding.Mode != ModeRemux || info.Transcoding.Reason != ReasonAudioTrack || info.Selection.AudioTrack != 2 ||
		last.Decision.AudioStream != 2 || last.Decision.AudioCopy {
		t.Errorf("audio track 2 = %+v %+v %+v", info.Transcoding, info.Selection, last.Decision)
	}

	// Validation: tracks of the file only, known qualities and deliveries.
	bad := 9
	for _, tc := range []struct {
		req         model.PlaybackRequest
		field, code string
	}{
		{model.PlaybackRequest{AudioTrack: &bad}, "audioTrack", "unknown_track"},
		{model.PlaybackRequest{SubtitleTrack: 999999}, "subtitleTrack", "unknown_track"},
		{model.PlaybackRequest{SubtitleTrack: tracks[f.Rated(0).FileID].Text}, "subtitleTrack", "unknown_track"},
		{model.PlaybackRequest{Quality: "4k"}, "quality", "invalid_value"},
		{model.PlaybackRequest{SubtitleDelivery: "smoke"}, "subtitleDelivery", "invalid_value"},
	} {
		if _, err := s.Start(ctx, session, m.ID, tc.req); !hasField(err, tc.field, tc.code) {
			t.Errorf("%+v: %v, want %s %s", tc.req, err, tc.field, tc.code)
		}
	}

	// Without a converter text subtitles are not available.
	plain := New(f.DB, vistest.Logger(), WithTokenKey(testKey))
	info, err = plain.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &caps, SubtitleTrack: ft.Text})
	if err != nil {
		t.Fatal(err)
	}
	g, err = plain.Authorize(ctx, tokenOf(t, info.SubtitleURL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plain.Subtitle(ctx, g, ft.Text); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("without converter: %v", err)
	}
}

func hasField(err error, field, code string) bool {
	var verr *model.ValidationError
	if !errors.As(err, &verr) {
		return false
	}
	for _, f := range verr.Fields {
		if f.Field == field && f.Code == code {
			return true
		}
	}
	return false
}

// manualClock is a test clock in a fixed time zone (the server's time zone of the watch time).
type manualClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var zurich = time.FixedZone("CET", 3600)

// K-5 with a clock mock: a 30-minute limit, progress events of 10 s each; the event that reaches
// the limit revokes the playback (stopped_by limit, 401 for its token) and answers 0 minutes; a
// start after the limit answers the watch-limit error; the next day starts fresh; the report
// shows the minutes per day.
func TestWatchLimit(t *testing.T) {
	f := vistest.New(t)
	f.AddGuest(t)
	clock := &manualClock{now: time.Date(2026, 10, 20, 18, 0, 0, 0, zurich)}
	s := New(f.DB, vistest.Logger(), WithClock(clock.Now), WithTokenKey(testKey))
	ctx := context.Background()
	kid := f.AddLimitedProfile(t, "Kind", model.PlaybackLimits{WatchLimitMinutes: 30})
	session := browserSession(t, f, kid)
	m := f.Rated(6)

	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if info.WatchTime == nil || *info.WatchTime != (model.WatchTime{LimitMinutes: 30, RemainingMinutes: 30}) {
		t.Fatalf("watch time at the start = %+v", info.WatchTime)
	}
	st := tokenOf(t, info.StreamURL)
	revokedAt := 0
	for i := 1; i <= 200 && revokedAt == 0; i++ {
		clock.Advance(10 * time.Second)
		p, err := s.Report(ctx, kid, m.ID, model.ProgressEvent{
			PlaybackSessionID: info.PlaybackSessionID, PositionMs: int64(i) * 10_000, DurationMs: vistest.DurationMs, PlayedMs: 10_000,
			Seq: int64(i), State: "playing", ClientTime: clock.Now(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if p.WatchTime == nil {
			t.Fatal("no watch time in the progress answer")
		}
		_, authErr := s.Authorize(ctx, st)
		switch {
		case p.WatchTime.RemainingMinutes == 0:
			revokedAt = i
			if !errors.Is(authErr, model.ErrUnauthenticated) {
				t.Errorf("event %d reached the limit, the token still works: %v", i, authErr)
			}
			if p.WatchTime.UsedMinutes != 30 {
				t.Errorf("used at the limit = %+v", p.WatchTime)
			}
		case authErr != nil:
			t.Fatalf("event %d (remaining %d): token refused %v", i, p.WatchTime.RemainingMinutes, authErr)
		}
	}
	if revokedAt != 180 {
		t.Fatalf("revoked after event %d, want 180 (30 min of 10-s events)", revokedAt)
	}
	if by := stoppedBy(t, f, info.PlaybackSessionID); by != model.StoppedByLimit {
		t.Errorf("stopped_by = %q", by)
	}
	_, err = s.Start(ctx, session, m.ID, model.PlaybackRequest{})
	var wl *model.WatchLimitError
	if !errors.Is(err, model.ErrWatchLimit) || !errors.As(err, &wl) || *wl != (model.WatchLimitError{LimitMinutes: 30, UsedMinutes: 30}) {
		t.Fatalf("start after the limit: %v", err)
	}
	// Midnight in the server's time zone starts a new day.
	clock.Advance(6*time.Hour - 30*time.Minute)
	if info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{}); err != nil || info.WatchTime.RemainingMinutes != 30 {
		t.Fatalf("start the next day: %+v, %v", info.WatchTime, err)
	}

	// The report: only the primary profile, only profiles of the account.
	owner := browserSession(t, f, f.P18)
	rep, err := s.WatchReport(ctx, owner, kid.ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []model.WatchDay{{Date: "2026-10-19", Minutes: 0}, {Date: "2026-10-20", Minutes: 30}, {Date: "2026-10-21", Minutes: 0}}
	if rep.ProfileID != kid.ID || rep.LimitMinutes != 30 || len(rep.Days) != 3 {
		t.Fatalf("report = %+v", rep)
	}
	for i := range want {
		if rep.Days[i] != want[i] {
			t.Errorf("day %d = %+v, want %+v", i, rep.Days[i], want[i])
		}
	}
	if rep, err := s.WatchReport(ctx, owner, kid.ID, 0); err != nil || len(rep.Days) != 30 {
		t.Errorf("default days: %d, %v", len(rep.Days), err)
	}
	if _, err := s.WatchReport(ctx, session, kid.ID, 3); !errors.Is(err, model.ErrPrimaryProfileRequired) {
		t.Errorf("non-primary profile: %v", err)
	}
	if _, err := s.WatchReport(ctx, owner, f.PG.ID, 3); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("profile of another account: %v", err)
	}
	if _, err := s.WatchReport(ctx, owner, kid.ID, 91); !hasField(err, "days", "invalid_value") {
		t.Errorf("91 days: %v", err)
	}
}

// W-8: the profile's limits shape the output (height, bit rate), the manual rung is honoured, and
// a profile with one stream gets the stream-limit error for a second one.
func TestStartLimits(t *testing.T) {
	f := vistest.New(t)
	clock := &manualClock{now: time.Date(2026, 10, 20, 18, 0, 0, 0, zurich)}
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithClock(clock.Now), WithTranscoder(tc), WithTokenKey(testKey))
	ctx := context.Background()
	m6, m12 := f.Rated(6), f.Rated(12)
	for _, m := range []vistest.Movie{m6, m12} {
		f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 10_000_000), testKeyframes)
	}
	caps := capsChromeLinux
	start := func(p model.Profile, m vistest.Movie, quality string) (model.PlaybackInfo, error) {
		return s.Start(ctx, browserSession(t, f, p), m.ID, model.PlaybackRequest{Capabilities: &caps, Quality: quality})
	}

	info, err := start(f.AddLimitedProfile(t, "Max720", model.PlaybackLimits{MaxHeight: 720}), m6, "")
	if err != nil {
		t.Fatal(err)
	}
	if tr := info.Transcoding; tr == nil || tr.Mode != ModeTranscode || tr.Height != 720 || tr.Width != 1280 || tr.Quality != model.Quality720 || tr.Reason != ReasonMaxHeight {
		t.Errorf("max 720p = %+v", info.Transcoding)
	}
	if last := tc.starts()[len(tc.starts())-1]; last.Decision.MaxHeight != 720 {
		t.Errorf("transcoder got %+v", last.Decision)
	}
	info, err = start(f.AddLimitedProfile(t, "Mbit4", model.PlaybackLimits{MaxBitrateKbps: 4000}), m6, "")
	if err != nil {
		t.Fatal(err)
	}
	if tr := info.Transcoding; tr == nil || tr.Bitrate > 4_000_000 || tr.Reason != ReasonBandwidth {
		t.Errorf("4 Mbit/s = %+v", info.Transcoding)
	}
	info, err = start(f.P18, m6, model.Quality480)
	if err != nil {
		t.Fatal(err)
	}
	if tr := info.Transcoding; tr == nil || tr.Quality != model.Quality480 || tr.Height != 480 || tr.Reason != ReasonQuality {
		t.Errorf("manual 480p = %+v", info.Transcoding)
	}
	for _, q := range info.Qualities {
		if q.Current != (q.Quality == model.Quality480) {
			t.Errorf("quality option %+v", q)
		}
	}

	one := f.AddLimitedProfile(t, "One", model.PlaybackLimits{MaxStreams: 1})
	a, b := browserSession(t, f, one), browserSession(t, f, one)
	first, err := s.Start(ctx, a, m6.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Start(ctx, b, m12.ID, model.PlaybackRequest{})
	var sl *model.StreamLimitError
	if !errors.Is(err, model.ErrStreamLimit) || !errors.As(err, &sl) || *sl != (model.StreamLimitError{MaxStreams: 1, ActiveStreams: 1}) {
		t.Fatalf("second stream: %v", err)
	}
	// A track or quality switch (same browser session and title) replaces the playback: the old
	// one ends (stopped_by client, 401 for its token, 404 for the client's own stop).
	switched, err := s.Start(ctx, a, m6.ID, model.PlaybackRequest{Quality: model.Quality720})
	if err != nil {
		t.Fatalf("switch: %v", err)
	}
	if _, err := s.Authorize(ctx, tokenOf(t, first.StreamURL)); !errors.Is(err, model.ErrUnauthenticated) {
		t.Errorf("token of the replaced playback: %v", err)
	}
	if by := stoppedBy(t, f, first.PlaybackSessionID); by != model.StoppedByClient {
		t.Errorf("replaced playback stopped_by = %q", by)
	}
	if err := s.Stop(ctx, a, first.PlaybackSessionID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("stop of the replaced playback: %v", err)
	}
	// Repeated starts of the same title never run more than one stream.
	tokens := []string{tokenOf(t, switched.StreamURL)}
	for range 5 {
		again, err := s.Start(ctx, a, m6.ID, model.PlaybackRequest{})
		if err != nil {
			t.Fatalf("repeated start: %v", err)
		}
		tokens = append(tokens, tokenOf(t, again.StreamURL))
	}
	valid := 0
	for _, st := range tokens {
		if _, err := s.Authorize(ctx, st); err == nil {
			valid++
		}
	}
	if valid != 1 {
		t.Errorf("%d of %d tokens valid after repeated starts, want 1", valid, len(tokens))
	}
	// Streams without a request for two minutes do not count.
	clock.Advance(ActiveWindow + time.Second)
	second, err := s.Start(ctx, b, m12.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatalf("after the activity window: %v", err)
	}
	if _, err := s.Start(ctx, a, m12.ID, model.PlaybackRequest{}); !errors.Is(err, model.ErrStreamLimit) {
		t.Errorf("third stream: %v", err)
	}
	// Stopping frees the slot.
	if err := s.Stop(ctx, b, second.PlaybackSessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(ctx, a, m12.ID, model.PlaybackRequest{}); err != nil {
		t.Errorf("after stop: %v", err)
	}
	_ = first
}

// Plan 3.5: an exhausted budget runs an SDR source one rung lower (budget_lowered); an HDR source
// answers transcode_busy.
func TestStartBudgetLowered(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	tc.busyAbove = 720
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithTokenKey(testKey))
	ctx := context.Background()
	sdr, hdr := f.Rated(6), f.Rated(12)
	f.SetMediaFile(t, sdr.FileID, "mkv", "hevc", "aac", trackProbe(t, "hevc", "bt709", 1920, 1080, 8_000_000), nil)
	f.SetMediaFile(t, hdr.FileID, "mkv", "hevc", "aac", trackProbe(t, "hevc", "smpte2084", 3840, 2160, 40_000_000), nil)
	caps := capsChromeLinux
	session := browserSession(t, f, f.P18)

	info, err := s.Start(ctx, session, sdr.ID, model.PlaybackRequest{Capabilities: &caps})
	if err != nil {
		t.Fatal(err)
	}
	if tr := info.Transcoding; tr == nil || tr.Quality != model.Quality720 || tr.Reason != ReasonBudgetLowered || tr.Height != 720 {
		t.Errorf("lowered = %+v", info.Transcoding)
	}
	if len(tc.attempts) != 1 || tc.attempts[0].Decision.MaxHeight != 1080 {
		t.Errorf("refused attempts = %+v", tc.attempts)
	}
	var quality string
	if err := f.DB.Read.QueryRow(`SELECT quality FROM playback_sessions WHERE id = ?`, info.PlaybackSessionID).Scan(&quality); err != nil || quality != model.Quality720 {
		t.Errorf("stored quality %q, %v", quality, err)
	}
	if _, err := s.Start(ctx, session, hdr.ID, model.PlaybackRequest{Capabilities: &caps}); !errors.Is(err, model.ErrTranscodeBusy) {
		t.Errorf("hdr source: %v", err)
	}
}

// W-7 and W-6: the capability profile is stored with the device and its owner; running streams
// are listed with profile, device, quality and mode; the administration ends one (401 for its
// token, ffmpeg stopped, stopped_by admin) or every stream of a device.
func TestDevicesAndStreamOverview(t *testing.T) {
	f := vistest.New(t)
	clock := &manualClock{now: time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC)}
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithClock(clock.Now), WithTranscoder(tc), WithTokenKey(testKey))
	ctx := context.Background()
	m6, m12 := f.Rated(6), f.Rated(12)
	f.SetMediaFile(t, m6.FileID, "mkv", "hevc", "aac", trackProbe(t, "hevc", "bt709", 1920, 1080, 8_000_000), nil)
	caps := capsChromeLinux
	onA := browserSession(t, f, f.P18)
	onA.DeviceID = deviceA
	onB := browserSession(t, f, f.P6)
	onB.DeviceID = deviceB

	a, err := s.Start(ctx, onA, m6.ID, model.PlaybackRequest{Capabilities: &caps})
	if err != nil {
		t.Fatal(err)
	}
	if a.Device == nil || *a.Device != (model.DeviceRef{ID: deviceA, Name: "Chrome 156 · Linux"}) {
		t.Errorf("device = %+v", a.Device)
	}
	var capsJSON, name, typ string
	var accountID, profileID, capsAt int64
	if err := f.DB.Read.QueryRow(`SELECT capabilities, name, type, account_id, profile_id, capabilities_at FROM devices WHERE id = ?`, deviceA).
		Scan(&capsJSON, &name, &typ, &accountID, &profileID, &capsAt); err != nil {
		t.Fatal(err)
	}
	var stored model.PlaybackCapabilities
	if err := json.Unmarshal([]byte(capsJSON), &stored); err != nil || stored.MaxHeight != 1440 || stored.Video[1] != "vp9" || stored.Containers[1] != "webm" ||
		name != caps.Name || typ != "desktop" || accountID != f.Account.ID || profileID != f.P18.ID || capsAt != clock.Now().UnixMilli() {
		t.Errorf("device row = %s %s %s %d %d %d", capsJSON, name, typ, accountID, profileID, capsAt)
	}
	clock.Advance(time.Second)
	b, err := s.Start(ctx, onB, f.Rated(0).ID, model.PlaybackRequest{}) // P6 sees the film rated 0
	if err != nil {
		t.Fatal(err)
	}
	// A start without a report keeps the stored profile of the device.
	if b.Device == nil || b.Device.ID != deviceB || b.Device.Name != "" {
		t.Errorf("device B = %+v", b.Device)
	}

	if _, err := s.Report(ctx, f.P18, m6.ID, model.ProgressEvent{PlaybackSessionID: a.PlaybackSessionID, PositionMs: 42_000, DurationMs: vistest.DurationMs,
		PlayedMs: 10_000, Seq: 1, State: "playing", ClientTime: clock.Now()}); err != nil {
		t.Fatal(err)
	}
	streams, err := s.ListStreams(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 2 || streams[0].ID != b.PlaybackSessionID || streams[1].ID != a.PlaybackSessionID {
		t.Fatalf("streams = %+v", streams)
	}
	sa, sb := streams[1], streams[0]
	if sa.ProfileName != "P18" || sa.AccountEmail != f.Account.Email || sa.Device == nil || sa.Device.Name != caps.Name || sa.Mode != ModeTranscode ||
		sa.Backend != "software" || sa.Hardware || sa.Quality != model.Quality1080 || sa.Height != 1080 || sa.Bitrate != 8_000_000 ||
		sa.Reason != ReasonVideoCodec || sa.PositionMs == nil || *sa.PositionMs != 42_000 || sa.ItemID != m6.ID {
		t.Errorf("stream A = %+v", sa)
	}
	if sb.ProfileName != "P6" || sb.Mode != ModeDirect || sb.Quality != "" || sb.Device == nil || sb.Device.ID != deviceB {
		t.Errorf("stream B = %+v", sb)
	}

	if err := s.StopStream(ctx, a.PlaybackSessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authorize(ctx, tokenOf(t, a.StreamURL)); !errors.Is(err, model.ErrUnauthenticated) {
		t.Errorf("token after the administrator's stop: %v", err)
	}
	if _, running := tc.Get(a.PlaybackSessionID); running || len(tc.stopped) == 0 || tc.stopped[len(tc.stopped)-1] != a.PlaybackSessionID {
		t.Errorf("ffmpeg not stopped: %v", tc.stopped)
	}
	if by := stoppedBy(t, f, a.PlaybackSessionID); by != model.StoppedByAdmin {
		t.Errorf("stopped_by = %q", by)
	}
	for _, id := range []string{a.PlaybackSessionID, "01JAAAAAAAAAAAAAAAAAAAAAAA"} {
		if err := s.StopStream(ctx, id); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("stop %s again: %v", id, err)
		}
	}
	if err := s.EndDevice(ctx, deviceB); err != nil {
		t.Fatal(err)
	}
	if by := stoppedBy(t, f, b.PlaybackSessionID); by != model.StoppedByAdmin {
		t.Errorf("stopped_by of the device's stream = %q", by)
	}
	if streams, err := s.ListStreams(ctx); err != nil || len(streams) != 0 {
		t.Errorf("streams after the stops = %+v, %v", streams, err)
	}

	// A browser session without a device: the client's deviceId counts in the form of a device id.
	bare := browserSession(t, f, f.P16)
	c, err := s.Start(ctx, bare, m6.ID, model.PlaybackRequest{DeviceID: deviceC, Capabilities: &caps})
	if err != nil {
		t.Fatal(err)
	}
	if c.Device == nil || c.Device.ID != deviceC {
		t.Errorf("third-party device = %+v", c.Device)
	}
	tv, err := s.Start(ctx, bare, m12.ID, model.PlaybackRequest{DeviceID: "tv"})
	if err != nil {
		t.Fatal(err)
	}
	var rowDevice string
	var devices int
	if err := f.DB.Read.QueryRow(`SELECT device_id FROM playback_sessions WHERE id = ?`, tv.PlaybackSessionID).Scan(&rowDevice); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM devices WHERE id = 'tv'`).Scan(&devices); err != nil {
		t.Fatal(err)
	}
	if tv.Device != nil || rowDevice != "tv" || devices != 0 {
		t.Errorf("free-form device id: %+v, row %q, %d devices", tv.Device, rowDevice, devices)
	}
	// Streams leave the overview after two minutes without a request.
	clock.Advance(ActiveWindow + time.Second)
	if streams, err := s.ListStreams(ctx); err != nil || len(streams) != 0 {
		t.Errorf("idle streams listed: %+v, %v", streams, err)
	}
}

// Every end of a playback records why (stopped_by).
func TestStoppedBy(t *testing.T) {
	f := vistest.New(t)
	f.AddGuest(t)
	s := New(f.DB, vistest.Logger(), WithTokenKey(testKey))
	ctx := context.Background()
	m := f.Rated(0)
	start := func(session model.Session) string {
		t.Helper()
		info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{})
		if err != nil {
			t.Fatal(err)
		}
		return info.PlaybackSessionID
	}
	logout := browserSession(t, f, f.P18)
	id := start(logout)
	if err := s.EndSession(ctx, logout.ID); err != nil {
		t.Fatal(err)
	}
	if by := stoppedBy(t, f, id); by != model.StoppedByLogout {
		t.Errorf("logout: %q", by)
	}
	id = start(browserSession(t, f, f.PG))
	if err := s.EndAccount(ctx, f.Guest.ID); err != nil {
		t.Fatal(err)
	}
	if by := stoppedBy(t, f, id); by != model.StoppedByRevoked {
		t.Errorf("guest revoked: %q", by)
	}
	id = start(browserSession(t, f, f.P0))
	if err := s.EndProfile(ctx, f.P0.ID); err != nil {
		t.Fatal(err)
	}
	if by := stoppedBy(t, f, id); by != model.StoppedByRevoked {
		t.Errorf("profile deleted: %q", by)
	}
}

// W-9: chapters, intro and outro and the next episode come with the playback.
func TestStartSegmentsAndNextEpisode(t *testing.T) {
	f := vistest.New(t)
	f.AddSeries(t)
	segs := f.AddSegments(t, f.Show.E1.ID)
	s := New(f.DB, vistest.Logger(), WithTokenKey(testKey))
	ctx := context.Background()
	session := browserSession(t, f, f.P18)
	info, err := s.Start(ctx, session, f.Show.E1.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if info.Segments.Intro == nil || *info.Segments.Intro != *segs.Intro || info.Segments.Outro == nil || *info.Segments.Outro != *segs.Outro {
		t.Errorf("segments = %+v", info.Segments)
	}
	if info.NextEpisode == nil || info.NextEpisode.ID != f.Show.E2.ID {
		t.Errorf("next episode = %+v", info.NextEpisode)
	}
	// P12 does not see E2 (rated 16): its next episode is E3.
	info, err = s.Start(ctx, browserSession(t, f, f.P12), f.Show.E1.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if info.NextEpisode == nil || info.NextEpisode.ID != f.Show.E3.ID {
		t.Errorf("next episode for P12 = %+v", info.NextEpisode)
	}
	if info, err := s.Start(ctx, session, f.Rated(6).ID, model.PlaybackRequest{}); err != nil || info.NextEpisode != nil ||
		info.Segments.Intro != nil || len(info.Chapters) != 0 {
		t.Errorf("film = %+v %+v %v", info.NextEpisode, info.Segments, err)
	}
}
