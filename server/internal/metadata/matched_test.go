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
