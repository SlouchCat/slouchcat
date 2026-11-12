package vistest

import "testing"

// SecondSeason is the season 2 of AddSecondSeason with its two episodes.
type SecondSeason struct {
	Season Movie // season 2 of the series of AddSeries, inherits 12
	E1, E2 Movie // S02E01 and S02E02, no own rating, effective 12
}

// AddSecondSeason adds season 2 with the episodes S02E01 and S02E02 (each with a media file) to
// the series of AddSeries, which it needs: the check mark of a whole season (concept 20 section
// 6.4) must lead continue watching and the offline rule "next episode" to S02E01. The titles are
// not part of All, so the contract tests do not see them.
func (f *Fixture) AddSecondSeason(t testing.TB) SecondSeason {
	t.Helper()
	if f.Show.Series.ID == "" {
		t.Fatal("vistest: AddSecondSeason needs AddSeries")
	}
	twelve := ptr(12)
	season := f.addMovie(t, Movie{Kind: "season", Label: "S2", ParentID: f.Show.Series.ID, Container: true, Effective: twelve, MinChild: twelve},
		f.libraryID, f.rootID, f.MediaDir, 42)
	e1 := f.addMovie(t, Movie{Kind: "episode", Label: "G1", ParentID: season.ID, Effective: twelve, RelPath: "Show/Season 02/Show - S02E01.mp4"},
		f.libraryID, f.rootID, f.MediaDir, 43)
	e2 := f.addMovie(t, Movie{Kind: "episode", Label: "G2", ParentID: season.ID, Effective: twelve, RelPath: "Show/Season 02/Show - S02E02.mp4"},
		f.libraryID, f.rootID, f.MediaDir, 44)
	exec(t, f.DB, `UPDATE items SET season_number = 2 WHERE id IN (?, ?, ?)`, season.ID, e1.ID, e2.ID)
	exec(t, f.DB, `UPDATE items SET episode_number = 1 WHERE id = ?`, e1.ID)
	exec(t, f.DB, `UPDATE items SET episode_number = 2 WHERE id = ?`, e2.ID)
	return SecondSeason{Season: season, E1: e1, E2: e2}
}
