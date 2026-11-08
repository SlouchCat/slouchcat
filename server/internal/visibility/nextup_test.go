package visibility

import "testing"

// nep is an episode of the nextUp table: id, season and number (-1 = none), the progress as
// "" (none), "started" or "done", and the time of that progress.
func nep(id string, season, number int, state string, at int64) nextUpEpisode {
	e := nextUpEpisode{ID: id, UpdatedAt: at}
	if season >= 0 {
		e.Season = &season
	}
	if number >= 0 {
		e.Number = &number
	}
	switch state {
	case "started":
		e.PositionMs = 1000
	case "done":
		e.Completed, e.PositionMs = true, 5000
	case "zero": // a row at position 0, not completed: no activity
	default:
		e.UpdatedAt = 0
	}
	return e
}

// withEnd sets the last episode of a file with several.
func withEnd(e nextUpEpisode, end int) nextUpEpisode {
	e.End = &end
	return e
}

// TestNextUp covers every row of concept 17 table 3.3 and the edges of the contract's NextUp:
// specials, multi-episode files, episodes without a number, hidden episodes (absent from the list)
// and the rewatch. The lists are in the order of ListSeriesNextUpEpisodes (season 0 first, those
// without a season or number last).
func TestNextUp(t *testing.T) {
	t.Parallel()
	// series builds S00E01, S01E01, S01E02, S01E03-E04, S02E01 with the given states and times.
	series := func(p0, e1, e2, e3, s2 string, at0, at1, at2, at3, ats2 int64) []nextUpEpisode {
		return []nextUpEpisode{
			nep("P1", 0, 1, p0, at0),
			nep("E1", 1, 1, e1, at1),
			nep("E2", 1, 2, e2, at2),
			withEnd(nep("E3", 1, 3, e3, at3), 4),
			nep("S2", 2, 1, s2, ats2),
		}
	}
	for _, tc := range []struct {
		name   string
		eps    []nextUpEpisode
		reason string
		want   string
	}{
		// Table 3.3.
		{"no progress: start with the first regular episode, not the special",
			series("", "", "", "", "", 0, 0, 0, 0, 0), NextUpStart, "E1"},
		{"the newest episode started: resume it",
			series("", "done", "started", "", "", 0, 1, 2, 0, 0), NextUpResume, "E2"},
		{"the newest episode completed, a next one exists: next",
			series("", "done", "", "", "", 0, 1, 0, 0, 0), NextUpNext, "E2"},
		{"the next one is taken even when watched",
			series("", "done", "done", "done", "", 0, 1, 5, 3, 0), NextUpNext, "E3"},
		{"the newest completed has no next, an earlier one is unwatched: the first unwatched",
			series("", "done", "", "done", "done", 0, 1, 0, 2, 3), NextUpNext, "E2"},
		{"everything watched: rewatch from the first regular episode",
			series("", "done", "done", "done", "done", 0, 1, 2, 3, 4), NextUpRewatch, "E1"},
		// Edges.
		{"a row at position 0 is no activity",
			series("", "zero", "", "", "", 0, 9, 0, 0, 0), NextUpStart, "E1"},
		{"an older started episode is ignored once a newer one was completed",
			series("", "started", "done", "", "", 0, 1, 2, 0, 0), NextUpNext, "E3"},
		{"equal times: the greater id is the newest",
			series("", "done", "started", "", "", 0, 7, 7, 0, 0), NextUpResume, "E2"},
		{"multi-episode file: the next one follows its last episode (S01E03-E04, then S02E01)",
			series("", "done", "done", "done", "", 0, 1, 2, 3, 0), NextUpNext, "S2"},
		{"the newest completed is a special: the first regular episode not completed",
			series("done", "done", "", "", "", 5, 1, 0, 0, 0), NextUpNext, "E2"},
		{"the newest completed is a special and every regular one is watched: rewatch",
			series("done", "done", "done", "done", "done", 9, 1, 2, 3, 4), NextUpRewatch, "E1"},
		{"a started special is resumed",
			series("started", "done", "", "", "", 5, 1, 0, 0, 0), NextUpResume, "P1"},
		{"rewatch: the first episode started again is resumed",
			series("", "started", "done", "done", "done", 0, 9, 2, 3, 4), NextUpResume, "E1"},
		{"rewatch: after the first episode completed again, the watched second is next",
			series("", "done", "done", "done", "done", 0, 9, 2, 3, 4), NextUpNext, "E2"},
		{"hidden episode: E2 absent, so E1 completed leads to E3",
			[]nextUpEpisode{nep("E1", 1, 1, "done", 1), withEnd(nep("E3", 1, 3, "", 0), 4)}, NextUpNext, "E3"},
		{"hidden episode: its progress is not in the list, so nothing started",
			[]nextUpEpisode{nep("E1", 1, 1, "", 0), withEnd(nep("E3", 1, 3, "", 0), 4)}, NextUpStart, "E1"},
		{"multi-episode file: an episode numbered inside the file is skipped",
			[]nextUpEpisode{withEnd(nep("E3", 1, 3, "done", 1), 4), nep("E4", 1, 4, "", 0), nep("E5", 1, 5, "", 0)}, NextUpNext, "E5"},
		{"specials only: start with the first special",
			[]nextUpEpisode{nep("P1", 0, 1, "", 0), nep("P2", 0, 2, "", 0)}, NextUpStart, "P1"},
		{"specials only: a completed special leads to the next special, watched or not",
			[]nextUpEpisode{nep("P1", 0, 1, "done", 2), nep("P2", 0, 2, "done", 1), nep("P3", 0, 3, "", 0)}, NextUpNext, "P2"},
		{"specials only, all watched: rewatch",
			[]nextUpEpisode{nep("P1", 0, 1, "done", 1), nep("P2", 0, 2, "done", 2)}, NextUpRewatch, "P1"},
		{"an episode without a number sorts last and is never the start",
			[]nextUpEpisode{nep("E1", 1, 1, "", 0), nep("E2", 1, 2, "", 0), nep("EX", 1, -1, "", 0)}, NextUpStart, "E1"},
		{"the newest completed has no number: the first regular episode not completed",
			[]nextUpEpisode{nep("E1", 1, 1, "done", 1), nep("E2", 1, 2, "", 0), nep("EX", 1, -1, "done", 5)}, NextUpNext, "E2"},
		{"the newest completed has no season: the first regular episode not completed",
			[]nextUpEpisode{nep("E1", 1, 1, "", 0), nep("E2", 1, 2, "done", 1), nep("EX", -1, 7, "done", 5)}, NextUpNext, "E1"},
		{"an episode without a number never follows by number, only as the first unwatched",
			[]nextUpEpisode{nep("E1", 1, 1, "done", 2), nep("EX", 1, -1, "", 0)}, NextUpNext, "EX"},
	} {
		reason, ep, ok := nextUp(tc.eps)
		if !ok || reason != tc.reason || ep.ID != tc.want {
			t.Errorf("%s: %q %q (ok %v), want %q %q", tc.name, reason, ep.ID, ok, tc.reason, tc.want)
		}
	}
	if _, _, ok := nextUp(nil); ok {
		t.Error("no visible episode: want no next up")
	}
}
