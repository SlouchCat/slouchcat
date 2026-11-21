package playback

import (
	"context"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// A start of the TV app (concept 26 stage 1): with selectsTracks the English AC-3 track plays
// directly, and the German PGS track plays without burn-in and without a WebVTT address; the same
// report without the new fields remuxes and burns in, as before.
func TestStartTVTracks(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	tracks := f.AddTracks(t)
	m := f.Rated(6)
	ft := tracks[m.FileID]
	f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000), testKeyframes)
	s := New(f.DB, vistest.Logger(), WithTranscoder(newFakeTranscoder()), WithTokenKey(testKey))
	ctx := context.Background()
	start := func(caps model.PlaybackCapabilities, req model.PlaybackRequest) model.PlaybackInfo {
		t.Helper()
		req.Capabilities = &caps
		info, err := s.Start(ctx, browserSession(t, f, f.P6), m.ID, req)
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	english := 2
	info := start(tvCaps(), model.PlaybackRequest{AudioTrack: &english, SubtitleTrack: ft.Bitmap})
	if info.Mode != ModeDirect || info.Transcoding != nil || info.Selection.AudioTrack != 2 ||
		info.Selection.SubtitleTrack != ft.Bitmap || info.Selection.SubtitleDelivery != "" || info.SubtitleURL != "" {
		t.Errorf("tv: mode %s, selection %+v, subtitle url %q; want direct, audio 2, pgs %d without delivery and url",
			info.Mode, info.Selection, info.SubtitleURL, ft.Bitmap)
	}
	info = start(tvOld(), model.PlaybackRequest{AudioTrack: &english, SubtitleTrack: ft.Bitmap})
	if info.Transcoding == nil || info.Transcoding.Mode != ModeTranscode || info.Transcoding.Reason != ReasonBurnIn ||
		info.Selection.SubtitleDelivery != model.SubtitleBurn {
		t.Errorf("without the new fields: %+v, selection %+v; want a transcode that burns in", info.Transcoding, info.Selection)
	}
}
