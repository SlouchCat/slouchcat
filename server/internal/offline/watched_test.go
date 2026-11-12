package offline

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 20 section 6.4: the check mark of a whole season staggers updated_at in episode order,
// the last episode the newest. Afterwards continue watching shows the series with S02E01 (the
// rule of concept 15 P-1), and the offline rule "next episode", which takes the newest row of a
// series (ListRecentEpisodeProgress), downloads S02E01 and not an episode just marked. Removing
// the check mark again keeps updated_at, so neither reader sees a new activity.
func TestSetWatchedStaggersEpisodes(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	f := e.f
	show := f.Show
	s2 := f.AddSecondSeason(t)
	for _, m := range []vistest.Movie{s2.E1, s2.E2} {
		f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probeJSON(t, testProbe()), keyframeBlob())
	}
	if _, err := e.svc.SetRules(ctx, e.session(f.P18, e.devA), model.DownloadRules{Enabled: true, NextEpisodes: true, WifiOnly: true,
		MaxCount: 3, Quality: model.DownloadLow}); err != nil {
		t.Fatal(err)
	}
	pb := playback.New(f.DB, vistest.Logger(), playback.WithClock(e.clock.Now))
	if err := pb.SetWatched(ctx, f.P18, show.Season.ID, true); err != nil {
		t.Fatal(err)
	}
	now := e.clock.Now().UnixMilli()
	for i, m := range []vistest.Movie{show.E1, show.E2, show.E3} {
		var updated, watched int64
		if err := f.DB.Read.QueryRow(`SELECT updated_at, watched_at FROM progress WHERE profile_id = ? AND item_id = ?`, f.P18.ID, m.ID).
			Scan(&updated, &watched); err != nil {
			t.Fatal(err)
		}
		if want := now - int64(2-i); updated != want || watched != want {
			t.Errorf("%s: updated_at %d, watched_at %d, want %d (staggered, the last episode now)", m.Label, updated, watched, want)
		}
	}
	vis := visibility.New(f.DB, vistest.Logger(), visibility.WithClock(e.clock.Now))
	cont, err := vis.ContinueWatching(ctx, visibility.Profile(f.P18), visibility.MaxRowItems)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range cont {
		got = append(got, it.ID)
	}
	if !slices.Contains(got, s2.E1.ID) || slices.ContainsFunc(got, func(id string) bool { return id == show.E1.ID || id == show.E2.ID || id == show.E3.ID }) {
		t.Errorf("continue watching after season 1 = %v, want S02E01 %s", got, s2.E1.ID)
	}
	auto := func(s model.OfflineSync) []string {
		var out []string
		for _, d := range s.Downloads {
			if d.Auto && d.Item != nil {
				out = append(out, d.Item.ID)
			}
		}
		return out
	}
	if got := auto(e.sync(f.P18, e.devA)); !slices.Equal(got, []string{s2.E1.ID}) {
		t.Errorf("auto-downloads after season 1 = %v, want S02E01 %s", got, s2.E1.ID)
	}

	// Removing the check mark of the season an hour later keeps updated_at: no row gets a newer
	// time, so neither continue watching nor the offline rule sees an activity.
	e.clock.Advance(time.Hour)
	if err := pb.SetWatched(ctx, f.P18, show.Season.ID, false); err != nil {
		t.Fatal(err)
	}
	var moved int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM progress WHERE profile_id = ? AND updated_at > ?`, f.P18.ID, now).Scan(&moved); err != nil || moved != 0 {
		t.Errorf("removing the check mark moved %d rows (%v), want none", moved, err)
	}
}
