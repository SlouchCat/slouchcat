package metadata

import (
	"context"
	"testing"
)

// The hook WithMatched (concept 25, architecture 11): after the relink hook found no twin, with
// the written ids; for a series only after its seasons; never for a title with a twin.

type matchedCalls struct {
	got []MatchedTitle
	// seasonState is the state of season 1 at the time of the call (series).
	seasonState []string
}

func TestMatchedHookAfterMovieMatch(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	var calls matchedCalls
	e.svc.WithMatched(func(_ context.Context, m MatchedTitle) { calls.got = append(calls.got, m) })
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(context.Background(), idByTMDB); err != nil {
		t.Fatal(err)
	}
	if len(calls.got) != 1 || calls.got[0].ItemID != idByTMDB || calls.got[0].Kind != "movie" || calls.got[0].TMDBID != 550 {
		t.Errorf("hook calls %+v", calls.got)
	}
}

func TestMatchedHookAfterSeriesSeasons(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	var calls matchedCalls
	e.svc.WithMatched(func(_ context.Context, m MatchedTitle) {
		calls.got = append(calls.got, m)
		calls.seasonState = append(calls.seasonState, e.child(t, idSeason1).State)
	})
	e.addTree(t, node{title: "Test Show", year: 2010})
	if err := e.svc.match(context.Background(), idSeries); err != nil {
		t.Fatal(err)
	}
	if len(calls.got) != 1 || calls.got[0].Kind != "series" || calls.got[0].TMDBID != 900 {
		t.Fatalf("hook calls %+v", calls.got)
	}
	if calls.seasonState[0] != "matched" {
		t.Errorf("season 1 at the hook = %q, want matched (hook after the seasons)", calls.seasonState[0])
	}
}

// A later scan brings episodes the series job did not write: the job of each one reports the
// series again, after the episode is written and also when the provider does not know it. An
// episode the series job wrote reports nothing.
func TestMatchedHookAfterLaterEpisode(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	var calls matchedCalls
	watch := idE0102
	e.svc.WithMatched(func(_ context.Context, m MatchedTitle) {
		calls.got = append(calls.got, m)
		calls.seasonState = append(calls.seasonState, e.child(t, watch).State)
	})
	e.addTree(t, node{title: "Test Show", year: 2010})
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	if len(calls.got) != 1 {
		t.Fatalf("after the series job: hook calls %+v", calls.got)
	}
	series := calls.got[0]

	// An episode of a later scan: pending below the matched series.
	mustExec(t, e.db.Write, `UPDATE items SET metadata_state = 'pending' WHERE id = ?`, idE0102)
	if err := e.svc.match(ctx, idE0102); err != nil {
		t.Fatal(err)
	}
	if len(calls.got) != 2 || calls.got[1] != series {
		t.Fatalf("after a later episode: hook calls %+v, want the series %+v again", calls.got, series)
	}
	if calls.seasonState[1] != "matched" {
		t.Errorf("episode at the hook = %q, want matched (hook after the episode)", calls.seasonState[1])
	}

	// TMDB does not know season 2: the file plays all the same, so the series is reported.
	watch = idE0201
	if err := e.svc.match(ctx, idE0201); err != nil {
		t.Fatal(err)
	}
	if len(calls.got) != 3 || calls.got[2] != series || calls.seasonState[2] != "unmatched" {
		t.Fatalf("after an episode the provider does not know: hook calls %+v, episode %q", calls.got, calls.seasonState)
	}

	// The series job wrote this one: its own job does nothing.
	if err := e.svc.match(ctx, idE0101); err != nil {
		t.Fatal(err)
	}
	if len(calls.got) != 3 {
		t.Errorf("after an episode the series job wrote: hook calls %+v", calls.got)
	}
}

// An episode whose series is not matched yet reports nothing: the series job makes the call.
func TestMatchedHookNotForEpisodeOfUnmatchedSeries(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	var calls matchedCalls
	e.svc.WithMatched(func(_ context.Context, m MatchedTitle) { calls.got = append(calls.got, m) })
	e.addTree(t, node{title: "Test Show", tmdbID: 900})
	if err := e.svc.match(context.Background(), idE0101); err != nil {
		t.Fatal(err)
	}
	if r := e.child(t, idE0101); r.State != "skipped" || len(calls.got) != 0 {
		t.Errorf("episode %+v, hook calls %+v", r, calls.got)
	}
}

// A title about to merge into its deleted twin gets no call: the twin's match makes it.
func TestMatchedHookNotWithRelinkTwin(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	hook := &fakeRelink{twin: true}
	e.svc.relink = hook.relink
	var calls matchedCalls
	e.svc.WithMatched(func(_ context.Context, m MatchedTitle) { calls.got = append(calls.got, m) })
	e.addTree(t, node{title: "Test Show", year: 2010})
	if err := e.svc.match(context.Background(), idSeries); err != nil {
		t.Fatal(err)
	}
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(context.Background(), idByTMDB); err != nil {
		t.Fatal(err)
	}
	if len(hook.calls) != 2 || len(calls.got) != 0 {
		t.Errorf("relink calls %v, hook calls %+v", hook.calls, calls.got)
	}
}
