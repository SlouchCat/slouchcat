package visibility

import (
	"context"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Concept 13 11.3: the track choice learns the series of an episode (through its season) and the
// original language (the series' for an episode) through visible_items, never from playback.
func TestReaderTrackContext(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	tracks := f.AddTracks(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	m := f.Rated(6)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := f.DB.Write.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE items SET original_language = 'en' WHERE id = ?`, m.ID)
	exec(`UPDATE items SET original_language = 'ja' WHERE id = ?`, f.Show.Series.ID)
	exec(`UPDATE items SET original_language = 'de' WHERE id = ?`, f.Show.E1.ID)

	for _, tc := range []struct {
		p    model.Profile
		item string
		want TrackContext
	}{
		{f.P6, m.ID, TrackContext{OriginalLanguage: "en"}},
		{f.P18, f.Show.E1.ID, TrackContext{SeriesID: f.Show.Series.ID, OriginalLanguage: "ja"}},
		{f.P18, f.Show.Special.ID, TrackContext{SeriesID: f.Show.Series.ID, OriginalLanguage: "ja"}},
		{f.P18, f.Show.Series.ID, TrackContext{OriginalLanguage: "ja"}},
		{f.P18, f.Rated(12).ID, TrackContext{}}, // not looked up yet
	} {
		got, err := r.TrackContext(ctx, Profile(tc.p), tc.item)
		if err != nil || got != tc.want {
			t.Errorf("TrackContext(%s, %s) = %+v, %v; want %+v", tc.p.Name, tc.item, got, err, tc.want)
		}
	}
	// The series keeps its unknown language ("" looked up) rather than the episode's.
	exec(`UPDATE items SET original_language = '' WHERE id = ?`, f.Show.Series.ID)
	if got, err := r.TrackContext(ctx, Profile(f.P18), f.Show.E1.ID); err != nil || got.OriginalLanguage != "" {
		t.Errorf("unknown series language = %+v, %v", got, err)
	}
	// Invisible: the episode is rated 12 through its series, P6 sees nothing of it.
	if _, err := r.TrackContext(ctx, Profile(f.P6), f.Show.E1.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible episode: %v", err)
	}
	if _, err := r.TrackContext(ctx, System("test"), m.ID); !errors.Is(err, ErrUnscoped) {
		t.Errorf("system scope: %v", err)
	}

	// tracks.is_hearing_impaired reaches the track list.
	exec(`UPDATE tracks SET is_hearing_impaired = 1 WHERE id = ?`, tracks[m.FileID].Text)
	list, err := r.Tracks(ctx, Profile(f.P6), m.ID, m.FileID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list.Subtitles {
		if s.HearingImpaired != (s.ID == tracks[m.FileID].Text) {
			t.Errorf("subtitle %d hearing impaired = %v", s.ID, s.HearingImpaired)
		}
	}
}
