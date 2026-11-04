package playback

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/trackpick"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 13 11.3: the server chooses the tracks a start leaves open by the profile's languages,
// the original language and the subtitle rule, and says why (TrackSelection reasons). The fixture
// file has German AAC (default, stream 1) and English AC3 5.1 (stream 2), English SubRip
// (stream 3), German PGS (stream 4) and a German SubRip sidecar.
func TestStartTrackChoice(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	tracks := f.AddTracks(t)
	m := f.Rated(6)
	ft := tracks[m.FileID]
	f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000), testKeyframes)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithTokenKey(testKey))
	plain := New(f.DB, vistest.Logger(), WithTokenKey(testKey))
	ctx := context.Background()
	caps := capsChromeLinux
	start := func(svc *Service, prefs model.PlaybackPrefs, req model.PlaybackRequest) model.PlaybackInfo {
		t.Helper()
		session := browserSession(t, f, f.P6)
		session.Profile.Prefs = prefs
		req.Capabilities = &caps
		info, err := svc.Start(ctx, session, m.ID, req)
		if err != nil {
			t.Fatal(err)
		}
		return info
	}
	prefs := func(mode string, audio, subs []string) model.PlaybackPrefs {
		return model.PlaybackPrefs{AudioLanguages: audio, SubtitleMode: mode, SubtitleLanguages: subs}
	}
	type want struct {
		mode           string // ModeDirect, ModeRemux, ModeTranscode
		audio          int
		audioReason    string
		subtitle       int64
		delivery       string
		subtitleReason string
	}
	check := func(name string, info model.PlaybackInfo, w want) {
		t.Helper()
		mode := ModeDirect
		if info.Transcoding != nil {
			mode = info.Transcoding.Mode
		}
		sel := info.Selection
		if mode != w.mode || sel.AudioTrack != w.audio || sel.AudioReason != w.audioReason || sel.SubtitleTrack != w.subtitle ||
			sel.SubtitleDelivery != w.delivery || sel.SubtitleReason != w.subtitleReason {
			t.Errorf("%s: %s %+v, want %+v", name, mode, sel, w)
		}
	}

	// Audio rule (F-4): profile languages, then the original language, then the file's default.
	check("profile english", start(s, prefs("forced", []string{"fr", "en"}, nil), model.PlaybackRequest{}),
		want{ModeRemux, 2, trackpick.AudioProfile, 0, "", trackpick.SubtitleOff})
	check("profile german is the default", start(s, prefs("forced", []string{"de"}, nil), model.PlaybackRequest{}),
		want{ModeDirect, 1, trackpick.AudioProfile, 0, "", trackpick.SubtitleOff})
	check("file default", start(s, prefs("forced", []string{"fr"}, nil), model.PlaybackRequest{}),
		want{ModeDirect, 1, trackpick.AudioFile, 0, "", trackpick.SubtitleOff})
	f.SetOriginalLanguage(t, m.ID, "en")
	check("original", start(s, prefs("forced", []string{"fr"}, nil), model.PlaybackRequest{}),
		want{ModeRemux, 2, trackpick.AudioOriginal, 0, "", trackpick.SubtitleOff})
	alt := 1
	check("request", start(s, prefs("forced", []string{"en"}, nil), model.PlaybackRequest{AudioTrack: &alt}),
		want{ModeDirect, 1, trackpick.AudioRequest, 0, "", trackpick.SubtitleOff})
	// Without HLS the fallback plays the default track and says so.
	alt = 2
	check("fallback plays the default", start(plain, prefs("forced", nil, nil), model.PlaybackRequest{AudioTrack: &alt}),
		want{ModeDirect, 1, trackpick.AudioFile, 0, "", trackpick.SubtitleOff})

	// Subtitle rule.
	check("mode off", start(s, prefs("off", []string{"en"}, []string{"en"}), model.PlaybackRequest{}),
		want{ModeRemux, 2, trackpick.AudioProfile, 0, "", trackpick.SubtitleOff})
	check("always full text track", start(s, prefs("always", []string{"de"}, []string{"en"}), model.PlaybackRequest{}),
		want{ModeDirect, 1, trackpick.AudioProfile, ft.Text, model.SubtitleText, trackpick.SubtitleMode})
	// German: the text sidecar comes before the PGS bitmap of the same language.
	check("always text before bitmap", start(s, prefs("always", []string{"de"}, []string{"de"}), model.PlaybackRequest{}),
		want{ModeDirect, 1, trackpick.AudioProfile, ft.Sidecar, model.SubtitleText, trackpick.SubtitleMode})
	check("foreign audio", start(s, prefs("foreign", []string{"fr"}, []string{"de"}), model.PlaybackRequest{}),
		want{ModeRemux, 2, trackpick.AudioOriginal, ft.Sidecar, model.SubtitleText, trackpick.SubtitleMode})
	check("foreign audio in a profile language", start(s, prefs("foreign", []string{"en"}, []string{"de"}), model.PlaybackRequest{}),
		want{ModeRemux, 2, trackpick.AudioProfile, 0, "", trackpick.SubtitleOff})
	check("subtitleOff", start(s, prefs("always", []string{"de"}, []string{"en"}), model.PlaybackRequest{SubtitleOff: true}),
		want{ModeDirect, 1, trackpick.AudioProfile, 0, "", trackpick.SubtitleRequest})
	check("subtitle request", start(s, prefs("off", []string{"de"}, nil), model.PlaybackRequest{SubtitleTrack: ft.Text}),
		want{ModeDirect, 1, trackpick.AudioProfile, ft.Text, model.SubtitleText, trackpick.SubtitleRequest})
	// A text delivery leaves the bitmap out of the choice; "always" falls back to the forced rule.
	f.DeleteTrack(t, ft.Sidecar)
	check("text delivery skips bitmaps", start(s, prefs("always", []string{"de"}, []string{"de"}), model.PlaybackRequest{SubtitleDelivery: model.SubtitleText}),
		want{ModeDirect, 1, trackpick.AudioProfile, 0, "", trackpick.SubtitleOff})

	// forced: the forced text track in the language of the audio.
	f.SetTrackForced(t, ft.Text)
	check("forced text", start(s, prefs("forced", []string{"en"}, nil), model.PlaybackRequest{}),
		want{ModeRemux, 2, trackpick.AudioProfile, ft.Text, model.SubtitleText, trackpick.SubtitleMode})

	// F-11: a forced bitmap only when the playback is transcoded anyway.
	f.SetTrackForced(t, ft.Bitmap)
	check("forced bitmap on direct play", start(s, prefs("forced", []string{"de"}, nil), model.PlaybackRequest{}),
		want{ModeDirect, 1, trackpick.AudioProfile, 0, "", trackpick.SubtitleForcedBitmap})
	// English audio by request: the forced English text track follows the audio.
	check("forced follows the requested audio", start(s, prefs("forced", []string{"de"}, nil), model.PlaybackRequest{AudioTrack: &alt}),
		want{ModeRemux, 2, trackpick.AudioRequest, ft.Text, model.SubtitleText, trackpick.SubtitleMode})
	info := start(s, prefs("forced", []string{"de"}, nil), model.PlaybackRequest{Quality: model.Quality720})
	check("forced bitmap on a transcode", info, want{ModeTranscode, 1, trackpick.AudioProfile, ft.Bitmap, model.SubtitleBurn, trackpick.SubtitleMode})
	if last := tc.starts()[len(tc.starts())-1]; last.Decision.BurnStream != 4 || info.Transcoding.Reason != ReasonBurnIn {
		t.Errorf("burn = %+v, reason %s", last.Decision, info.Transcoding.Reason)
	}
	// Without a transcoder the burn-in is dropped again, and the reason says what is missing.
	check("forced bitmap without transcoder", start(plain, prefs("forced", []string{"de"}, nil), model.PlaybackRequest{Quality: model.Quality720}),
		want{ModeDirect, 1, trackpick.AudioProfile, 0, "", trackpick.SubtitleForcedBitmap})

	// A film has no series: rememberForSeries writes nothing.
	start(s, prefs("forced", nil, nil), model.PlaybackRequest{AudioTrack: &alt, RememberForSeries: true})
	var n int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM profile_series_tracks`).Scan(&n); err != nil || n != 0 {
		t.Errorf("series rows after a film = %d, %v", n, err)
	}
}

// Concept 13 11.3 "Je Serie merken": a choice by hand during an episode with rememberForSeries
// keeps its languages for the series, and the next episode starts with them.
func TestStartRemembersSeries(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	tracks := f.AddTracks(t)
	for _, e := range []vistest.Movie{f.Show.E1, f.Show.E2, f.Show.E3, f.Show.Special} {
		f.SetMediaFile(t, e.FileID, "mp4", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000), testKeyframes)
	}
	s := New(f.DB, vistest.Logger(), WithTranscoder(newFakeTranscoder()), WithTokenKey(testKey))
	ctx := context.Background()
	caps := capsChromeLinux
	session := browserSession(t, f, f.P18)
	session.Profile.Prefs = model.PlaybackPrefs{AudioLanguages: []string{"de"}, SubtitleMode: model.SubtitleModeForced, SubtitleLanguages: []string{"de"}}
	start := func(e vistest.Movie, req model.PlaybackRequest) model.TrackSelection {
		t.Helper()
		req.Capabilities = &caps
		info, err := s.Start(ctx, session, e.ID, req)
		if err != nil {
			t.Fatal(err)
		}
		return info.Selection
	}
	remembered := func() (appq.ProfileSeriesTrack, bool) {
		t.Helper()
		row, err := appq.New(f.DB.Read).GetSeriesTracks(ctx, appq.GetSeriesTracksParams{ProfileID: f.P18.ID, SeriesID: f.Show.Series.ID})
		if errors.Is(err, sql.ErrNoRows) {
			return row, false
		}
		if err != nil {
			t.Fatal(err)
		}
		return row, true
	}

	// Nothing remembered: the profile's German, no subtitles; a choice without the switch keeps nothing.
	english := 2
	if sel := start(f.Show.E1, model.PlaybackRequest{}); sel.AudioTrack != 1 || sel.AudioReason != trackpick.AudioProfile {
		t.Errorf("first episode = %+v", sel)
	}
	start(f.Show.E1, model.PlaybackRequest{AudioTrack: &english, SubtitleTrack: tracks[f.Show.E1.FileID].Text})
	if _, ok := remembered(); ok {
		t.Error("remembered without rememberForSeries")
	}

	// English audio with English subtitles, remembered for the series.
	start(f.Show.E1, model.PlaybackRequest{AudioTrack: &english, SubtitleTrack: tracks[f.Show.E1.FileID].Text, RememberForSeries: true})
	if row, ok := remembered(); !ok || row.AudioLanguage != "en" || row.SubtitleLanguage != "en" || row.SubtitleOff != 0 {
		t.Errorf("remembered = %+v, %v", row, ok)
	}
	sel := start(f.Show.E2, model.PlaybackRequest{})
	if sel.AudioTrack != 2 || sel.AudioReason != trackpick.AudioSeries || sel.SubtitleTrack != tracks[f.Show.E2.FileID].Text ||
		sel.SubtitleReason != trackpick.SubtitleSeries {
		t.Errorf("next episode = %+v", sel)
	}

	// Subtitles off by hand: the audio language stays remembered.
	start(f.Show.E2, model.PlaybackRequest{SubtitleOff: true, RememberForSeries: true})
	if row, ok := remembered(); !ok || row.AudioLanguage != "en" || row.SubtitleLanguage != "" || row.SubtitleOff != 1 {
		t.Errorf("remembered off = %+v, %v", row, ok)
	}
	if sel := start(f.Show.Special, model.PlaybackRequest{}); sel.AudioTrack != 2 || sel.AudioReason != trackpick.AudioSeries ||
		sel.SubtitleTrack != 0 || sel.SubtitleReason != trackpick.SubtitleSeries {
		t.Errorf("special after off = %+v", sel)
	}

	// A track without language stores an empty language, which does not count: the profile's German again.
	f.ClearTrackLanguage(t, tracks[f.Show.E3.FileID].AudioAlt)
	start(f.Show.E3, model.PlaybackRequest{AudioTrack: &english, SubtitleOff: true, RememberForSeries: true})
	if row, ok := remembered(); !ok || row.AudioLanguage != "" || row.SubtitleOff != 1 {
		t.Errorf("remembered without language = %+v, %v", row, ok)
	}
	if sel := start(f.Show.E1, model.PlaybackRequest{}); sel.AudioTrack != 1 || sel.AudioReason != trackpick.AudioProfile {
		t.Errorf("after a track without language = %+v", sel)
	}
}
