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

// progressTimes returns updated_at and watched_at (0 when NULL) of a progress row.
func progressTimes(t *testing.T, f *vistest.Fixture, profileID int64, itemID string) (updated, watched int64) {
	t.Helper()
	if err := f.DB.Read.QueryRow(`SELECT updated_at, COALESCE(watched_at, 0) FROM progress WHERE profile_id = ? AND item_id = ?`, profileID, itemID).
		Scan(&updated, &watched); err != nil {
		t.Fatal(err)
	}
	return updated, watched
}

// checkReaders asserts what continue watching shows of the series and what the offline rule
// "next episode" downloads.
func checkReaders(t *testing.T, e *env, wantContinue, wantAuto string, notContinue ...string) {
	t.Helper()
	f := e.f
	vis := visibility.New(f.DB, vistest.Logger(), visibility.WithClock(e.clock.Now))
	cont, err := vis.ContinueWatching(context.Background(), visibility.Profile(f.P18), visibility.MaxRowItems)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range cont {
		got = append(got, it.ID)
	}
	if !slices.Contains(got, wantContinue) || slices.ContainsFunc(got, func(id string) bool { return slices.Contains(notContinue, id) }) {
		t.Errorf("continue watching = %v, want %s and none of %v", got, wantContinue, notContinue)
	}
	var auto []string
	for _, d := range e.sync(f.P18, e.devA).Downloads {
		if d.Auto && d.Item != nil {
			auto = append(auto, d.Item.ID)
		}
	}
	if !slices.Equal(auto, []string{wantAuto}) {
		t.Errorf("auto-downloads = %v, want %s", auto, wantAuto)
	}
}

// Decision of 12 November 2026 (concept 20 section 6.4): the check mark of a season keeps the
// dates of an episode watched before, watched_at and updated_at alike; only the newly marked
// episodes get the staggered times, the last one the newest, so both readers still move on to
// S02E01.
func TestSetWatchedSeasonKeepsWatchedEpisodes(t *testing.T) {
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
	if err := pb.SetWatched(ctx, f.P18, show.E1.ID, true); err != nil {
		t.Fatal(err)
	}
	first := e.clock.Now().UnixMilli()
	e.clock.Advance(time.Hour)
	if err := pb.SetWatched(ctx, f.P18, show.Season.ID, true); err != nil {
		t.Fatal(err)
	}
	now := e.clock.Now().UnixMilli()
	if u, w := progressTimes(t, f, f.P18.ID, show.E1.ID); u != first || w != first {
		t.Errorf("E1 watched before: updated_at %d, watched_at %d, want both kept at %d", u, w, first)
	}
	for i, m := range []vistest.Movie{show.E2, show.E3} {
		if u, w := progressTimes(t, f, f.P18.ID, m.ID); u != now-int64(1-i) || w != now-int64(1-i) {
			t.Errorf("%s newly marked: updated_at %d, watched_at %d, want %d", m.Label, u, w, now-int64(1-i))
		}
	}
	checkReaders(t, e, s2.E1.ID, s2.E1.ID, show.E1.ID, show.E2.ID, show.E3.ID)
}

// Decision of 12 November 2026 (concept 20 section 6.4): a check mark on an episode before the
// newest started episode of the series (S01E03 while S02E01 runs) gets an updated_at below that
// episode's, so continue watching keeps S02E01 and the offline rule "next episode" keeps loading
// S02E02. The same holds for the whole of season 1 afterwards: a stored row older than S02E01
// keeps its updated_at, a new one gets a time below S02E01, and S01E03, watched already, keeps
// its dates.
func TestSetWatchedBehindKeepsContinueWatching(t *testing.T) {
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
	running := e.clock.Now().Add(-time.Minute).UnixMilli()
	for _, row := range []struct {
		id      string
		pos     int64
		updated int64
	}{{s2.E1.ID, 600_000, running}, {show.E1.ID, 0, running - 5000}} {
		if _, err := f.DB.Write.Exec(`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at)
			VALUES (?, ?, ?, ?, 0, ?)`, f.P18.ID, row.id, row.pos, vistest.DurationMs, row.updated); err != nil {
			t.Fatal(err)
		}
	}
	checkReaders(t, e, s2.E1.ID, s2.E2.ID)

	pb := playback.New(f.DB, vistest.Logger(), playback.WithClock(e.clock.Now))
	if err := pb.SetWatched(ctx, f.P18, show.E3.ID, true); err != nil {
		t.Fatal(err)
	}
	checked := e.clock.Now().UnixMilli()
	if u, w := progressTimes(t, f, f.P18.ID, show.E3.ID); u != running-1 || w != checked {
		t.Errorf("S01E03 checked: updated_at %d, watched_at %d, want %d (below S02E01) and %d", u, w, running-1, checked)
	}
	checkReaders(t, e, s2.E1.ID, s2.E2.ID, show.E3.ID)

	e.clock.Advance(time.Hour)
	if err := pb.SetWatched(ctx, f.P18, show.Season.ID, true); err != nil {
		t.Fatal(err)
	}
	now := e.clock.Now().UnixMilli()
	for _, tc := range []struct {
		m                vistest.Movie
		updated, watched int64
	}{
		{show.E1, running - 5000, now - 2}, // stored, older than S02E01: keeps its updated_at
		{show.E2, running - 1, now - 1},    // new row: below S02E01
		{show.E3, running - 1, checked},    // watched before: untouched
	} {
		if u, w := progressTimes(t, f, f.P18.ID, tc.m.ID); u != tc.updated || w != tc.watched {
			t.Errorf("%s after season 1: updated_at %d, watched_at %d, want %d and %d", tc.m.Label, u, w, tc.updated, tc.watched)
		}
	}
	if u, _ := progressTimes(t, f, f.P18.ID, s2.E1.ID); u != running {
		t.Errorf("S02E01 updated_at %d, want %d", u, running)
	}
	checkReaders(t, e, s2.E1.ID, s2.E2.ID, show.E1.ID, show.E2.ID, show.E3.ID)
}
