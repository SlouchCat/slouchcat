package metadata

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// fakeRelink is a relink hook (WithRelink) that records its calls.
type fakeRelink struct {
	merged bool
	err    error
	calls  []string
}

func (f *fakeRelink) relink(_ context.Context, itemID string) (string, bool, error) {
	f.calls = append(f.calls, itemID)
	if f.merged {
		return "01JDELETEDTWIN000000000000", true, f.err
	}
	return itemID, false, f.err
}

// A series that merged into its deleted twin is gone: the match stops after the title, without
// seasons, fill or trailer for the copy (ADR 0023).
func TestMatchSeriesStopsAfterRelink(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	hook := &fakeRelink{merged: true}
	e.svc.relink = hook.relink
	e.addTree(t, node{title: "Test Show", year: 2010})
	if err := e.svc.match(context.Background(), idSeries); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(hook.calls, []string{idSeries}) {
		t.Fatalf("hook calls %v", hook.calls)
	}
	if s := e.child(t, idSeries); s.State != "matched" || s.TMDB.Int64 != 900 {
		t.Fatalf("series row %+v", s)
	}
	if r := e.child(t, idSeason1); r.State != "pending" {
		t.Errorf("season 1 state %q, want pending (no season match for the copy)", r.State)
	}
	for _, typ := range []string{JobFill, JobTrailerFind} {
		if jobs := e.jobList(t, typ); len(jobs) != 0 {
			t.Errorf("%s jobs %v, want none", typ, jobs)
		}
	}
}

// Without a merge, and after a failed relink, the match goes on as before.
func TestMatchGoesOnWithoutRelink(t *testing.T) {
	t.Parallel()
	for name, hook := range map[string]*fakeRelink{
		"no twin": {},
		"error":   {err: errors.New("relink failed")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, goodToken)
			e.svc.relink = hook.relink
			e.addTree(t, node{title: "Test Show", year: 2010})
			if err := e.svc.match(context.Background(), idSeries); err != nil {
				t.Fatal(err)
			}
			if r := e.child(t, idSeason1); r.State != "matched" {
				t.Errorf("season 1 state %q, want matched", r.State)
			}
			if jobs := e.jobList(t, JobFill); len(jobs) != 1 {
				t.Errorf("fill jobs %v, want one", jobs)
			}
		})
	}
}

// A film that merged gets no image fetch and no trailer search.
func TestMatchMovieStopsAfterRelink(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	hook := &fakeRelink{merged: true}
	e.svc.relink = hook.relink
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(context.Background(), idByTMDB); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(hook.calls, []string{idByTMDB}) {
		t.Fatalf("hook calls %v", hook.calls)
	}
	for _, typ := range []string{JobImageFetch, JobTrailerFind} {
		if jobs := e.jobList(t, typ); len(jobs) != 0 {
			t.Errorf("%s jobs %v, want none", typ, jobs)
		}
	}
}
