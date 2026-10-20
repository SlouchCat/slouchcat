package playback

import (
	"context"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Plan 3.5: lowering a 4K SDR source saves no budget (cost 3 on every rung), so an exhausted
// budget answers transcode_busy after one attempt instead of waiting again on 720p.
func TestStartBudgetNotLoweredFor4K(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	tc.busyAbove = 360
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithTokenKey(testKey))
	m := f.Rated(6)
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", trackProbe(t, "hevc", "bt709", 3840, 2160, 40_000_000), nil)
	caps := capsChromeLinux
	_, err := s.Start(context.Background(), browserSession(t, f, f.P18), m.ID, model.PlaybackRequest{Capabilities: &caps})
	if !errors.Is(err, model.ErrTranscodeBusy) {
		t.Fatalf("4k sdr on an exhausted budget: %v", err)
	}
	if len(tc.attempts) != 1 {
		t.Errorf("attempts = %d, want 1", len(tc.attempts))
	}
}

// A VobSub sidecar (.sub) is neither text nor a bitmap a transcode can burn in: 422.
func TestTrackRequestNotBurnable(t *testing.T) {
	tracks := model.TrackList{Subtitles: []model.Track{
		{ID: 5, Index: -1, Kind: "subtitle", Codec: "sub", Source: "sidecar", RelPath: "Film/Film.de.sub"},
		{ID: 6, Index: 4, Kind: "subtitle", Codec: "hdmv_pgs_subtitle", Source: "embedded"},
		{ID: 7, Index: 5, Kind: "subtitle", Codec: "dvb_subtitle", Source: "embedded"},
		{ID: 8, Index: -1, Kind: "subtitle", Codec: "subrip", Source: "sidecar", Text: true},
	}}
	for _, c := range []struct {
		track    int64
		delivery string
		code     string
	}{
		{5, "", "not_burnable"},
		{5, model.SubtitleBurn, "not_burnable"},
		{5, model.SubtitleText, "not_text"},
		{7, "", "not_burnable"},
		{6, "", ""},
		{8, model.SubtitleBurn, ""},
	} {
		_, err := trackRequest(model.PlaybackRequest{SubtitleTrack: c.track, SubtitleDelivery: c.delivery}, tracks)
		switch {
		case c.code == "" && err != nil:
			t.Errorf("track %d %q: %v", c.track, c.delivery, err)
		case c.code != "" && !hasField(err, "subtitleDelivery", c.code):
			t.Errorf("track %d %q: %v, want %s", c.track, c.delivery, err, c.code)
		}
	}
}

// A rebuild repeats the recorded decision, not the profile's current limits: the client holds
// the master playlist of the start. A bandwidth limit that capped the bit rate stays applied.
func TestRebuildKeepsRecordedLimits(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithTokenKey(testKey))
	ctx := context.Background()
	m := f.Rated(6)
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", trackProbe(t, "hevc", "bt709", 1920, 1080, 8_000_000), nil)
	caps := capsChromeLinux
	for _, c := range []struct {
		name   string
		limits model.PlaybackLimits
	}{
		{"no limit", model.PlaybackLimits{}},
		{"bandwidth", model.PlaybackLimits{MaxBitrateKbps: 500}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := f.AddLimitedProfile(t, c.name, c.limits)
			info, err := s.Start(ctx, browserSession(t, f, p), m.ID, model.PlaybackRequest{Capabilities: &caps})
			if err != nil {
				t.Fatal(err)
			}
			started := tc.starts()[len(tc.starts())-1].Decision
			if _, err := f.DB.Write.Exec(`UPDATE profiles SET max_height = 480, max_bitrate_kbps = 300 WHERE id = ?`, p.ID); err != nil {
				t.Fatal(err)
			}
			tc.lose(info.PlaybackSessionID)
			g, err := s.Authorize(ctx, tokenOf(t, info.StreamURL))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.HLS(ctx, g, 3); err != nil {
				t.Fatal(err)
			}
			if rebuilt := tc.starts()[len(tc.starts())-1].Decision; rebuilt != started {
				t.Errorf("rebuilt %+v, started %+v", rebuilt, started)
			}
		})
	}
}
