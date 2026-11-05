package offline

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Concept 13 11.3 and 11.11: a download without audioTrack, a season download and an auto-download
// get the audio track the player would choose (package trackpick): the profile's languages, the
// original language, the language remembered for the series, else the file's default. The fixture
// files have German AAC (default, stream 1) and English AC3 (stream 2).
func TestDownloadsUseProfileLanguage(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	show := f.Show
	withPrefs := func(p model.Profile, audio ...string) model.Profile {
		p.Prefs = model.PlaybackPrefs{AudioLanguages: audio, SubtitleMode: model.SubtitleModeForced}
		return p
	}
	audioOf := func(itemID, device string) int64 {
		t.Helper()
		var a int64
		if err := f.DB.Read.QueryRow(`SELECT audio_stream FROM downloads WHERE item_id = ? AND device_id = ?`, itemID, device).Scan(&a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	create := func(p model.Profile, device, itemID string, req model.DownloadRequest) {
		t.Helper()
		req.Quality = model.DownloadLow
		if _, err := e.svc.Create(ctx, e.session(p, device), itemID, req); err != nil {
			t.Fatalf("create %s: %v", itemID, err)
		}
	}

	// A film: the profile's English, the original language, the file's default, the request.
	create(withPrefs(f.P18, "fr", "en"), e.devA, f.Rated(0).ID, model.DownloadRequest{})
	e.exec(`UPDATE items SET original_language = 'en' WHERE id = ?`, f.Rated(6).ID)
	create(withPrefs(f.P18, "fr"), e.devA, f.Rated(6).ID, model.DownloadRequest{})
	create(withPrefs(f.P18, "fr"), e.devA, f.Rated(12).ID, model.DownloadRequest{})
	german := 1
	create(withPrefs(f.P18, "en"), e.devA, f.Rated(16).ID, model.DownloadRequest{AudioTrack: &german})
	for _, tc := range []struct {
		item string
		want int64
	}{{f.Rated(0).ID, 2}, {f.Rated(6).ID, 2}, {f.Rated(12).ID, 1}, {f.Rated(16).ID, 1}} {
		if got := audioOf(tc.item, e.devA); got != tc.want {
			t.Errorf("audio of %s = %d, want %d", tc.item, got, tc.want)
		}
	}

	// A season: the language the request names (any form of the code), else the profile's rule.
	create(withPrefs(f.P18, "en"), e.devB, show.Season.ID, model.DownloadRequest{})
	for _, ep := range []string{show.E1.ID, show.E2.ID, show.E3.ID} {
		if got := audioOf(ep, e.devB); got != 2 {
			t.Errorf("season episode %s audio = %d, want 2", ep, got)
		}
	}
	devC := e.device("c")
	create(withPrefs(f.P18, "en"), devC, show.Season.ID, model.DownloadRequest{AudioLanguage: "ger"})
	if got := audioOf(show.E1.ID, devC); got != 1 {
		t.Errorf("season in ger audio = %d, want 1", got)
	}

	// The language remembered for the series comes before the profile's.
	e.exec(`INSERT INTO profile_series_tracks (profile_id, series_id, audio_language, subtitle_language, subtitle_off, updated_at)
	        VALUES (?, ?, 'en', '', 0, 1)`, f.P16.ID, show.Series.ID)
	devD := e.device("d")
	create(withPrefs(f.P16, "de"), devD, show.Special.ID, model.DownloadRequest{})
	if got := audioOf(show.Special.ID, devD); got != 2 {
		t.Errorf("remembered series audio = %d, want 2", got)
	}

	// Auto-download of the next episode: the profile's language as the profile row stores it.
	e.exec(`UPDATE profiles SET audio_languages = '["en"]' WHERE id = ?`, f.P12.ID)
	devE := e.device("e")
	if _, err := e.svc.SetRules(ctx, e.session(f.P12, devE), model.DownloadRules{Enabled: true, NextEpisodes: true, WifiOnly: true,
		MaxCount: 1, Quality: model.DownloadLow}); err != nil {
		t.Fatal(err)
	}
	e.exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES (?, ?, 1000, 2000, 1, ?)`,
		f.P12.ID, show.E1.ID, e.clock.Now().Add(-time.Hour).UnixMilli())
	s := e.sync(f.P12, devE)
	var auto []string
	for _, d := range s.Downloads {
		if d.Auto && d.Item != nil {
			auto = append(auto, d.Item.ID)
		}
	}
	if !slices.Equal(auto, []string{show.E3.ID}) {
		t.Fatalf("auto-downloads = %v, want E3", auto)
	}
	if got := audioOf(show.E3.ID, devE); got != 2 {
		t.Errorf("auto-download audio = %d, want 2", got)
	}
}
