package metadata

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
)

// fakeRelink is a relink hook (WithRelink) that records its calls.
type fakeRelink struct {
	twin  bool
	err   error
	calls []string
}

func (f *fakeRelink) relink(_ context.Context, itemID string) (bool, error) {
	f.calls = append(f.calls, itemID)
	return f.twin, f.err
}

// A series with a deleted twin is about to merge into it: the match stops after the title, without
// seasons, fill or trailer for the copy (ADR 0023).
func TestMatchSeriesStopsAfterRelink(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	hook := &fakeRelink{twin: true}
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

// Without a twin the match goes on as before.
func TestMatchGoesOnWithoutRelink(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	hook := &fakeRelink{}
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
}

// A failed hook fails the match, so the queue retries it and the hook runs again; the title is
// matched already.
func TestMatchFailsWhenRelinkHookFails(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	hookErr := errors.New("database is locked")
	hook := &fakeRelink{err: hookErr}
	e.svc.relink = hook.relink
	e.addTree(t, node{title: "Test Show", year: 2010})
	if err := e.svc.match(context.Background(), idSeries); !errors.Is(err, hookErr) {
		t.Fatalf("match = %v, want the hook's error", err)
	}
	if s := e.child(t, idSeries); s.State != "matched" {
		t.Errorf("series state %q, want matched", s.State)
	}
	if r := e.child(t, idSeason1); r.State != "pending" {
		t.Errorf("season 1 state %q, want pending", r.State)
	}
}

// A film with a deleted twin gets no image fetch and no trailer search.
func TestMatchMovieStopsAfterRelink(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	hook := &fakeRelink{twin: true}
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

// A match queued with noRelink (library.relink failed for good or was cancelled) skips the hook
// and completes the series: seasons and fill.
func TestMatchWithNoRelinkSkipsTheHook(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	hook := &fakeRelink{twin: true}
	e.svc.relink = hook.relink
	e.addTree(t, node{title: "Test Show", year: 2010})
	if err := e.svc.handleMatch(context.Background(), &jobs.Job{Type: JobMatch, Payload: []byte(`{"itemId":"` + idSeries + `","noRelink":true}`)}); err != nil {
		t.Fatal(err)
	}
	if len(hook.calls) != 0 {
		t.Errorf("hook calls %v, want none", hook.calls)
	}
	if r := e.child(t, idSeason1); r.State != "matched" {
		t.Errorf("season 1 state %q, want matched", r.State)
	}
	if fills := e.jobList(t, JobFill); len(fills) != 1 {
		t.Errorf("fill jobs %v, want one", fills)
	}
}
