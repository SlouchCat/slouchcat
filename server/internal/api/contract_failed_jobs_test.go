package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Contract 1.31.0 (concept 20, P-6): the failed runs of every type in pages, and the two bulk
// actions retryFailedJobs and dismissFailedJobs. The admin service gets a registry with two event
// types whose runs this test fails by hand.
func TestFailedJobsOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	ctx := context.Background()
	log := vistest.Logger()
	const typeA, typeB = "fake.alpha", "fake.beta"
	noop := func(context.Context, *jobs.Job) error { return nil }
	reg := jobs.NewRegistry(pf.queue, log)
	pf.off.RegisterTypes(reg)
	for _, name := range []string{typeA, typeB} {
		reg.Register(jobs.Type{Name: name, Group: jobs.GroupSystem, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: 1, Handler: noop})
	}
	sched := jobs.NewScheduler(pf.queue, reg, log)
	if err := sched.Start(ctx); err != nil {
		t.Fatal(err)
	}
	pf.admin.WithJobControl(reg, sched, map[jobs.Pool]int{jobs.PoolGeneral: 1, jobs.PoolDownload: 1, jobs.PoolNotify: 1, jobs.PoolIndex: 1})
	fail := func(typ string) {
		t.Helper()
		if _, err := pf.queue.Enqueue(ctx, jobs.NewJob{Type: typ, MaxAttempts: 1}); err != nil {
			t.Fatal(err)
		}
		j, err := pf.queue.Claim(ctx, "test", []string{typ})
		if err != nil || j == nil {
			t.Fatalf("claim %s: %v, %v", typ, j, err)
		}
		if err := pf.queue.Fail(ctx, "test", j, errors.New("broke")); err != nil {
			t.Fatal(err)
		}
	}
	fail(typeA)
	fail(typeA)
	fail(typeB)
	// A failure of a type the registry no longer knows: listed and dismissed, never queued again.
	gone, err := pf.queue.Enqueue(ctx, jobs.NewJob{Type: "old.gone", MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pf.f.DB.Write.ExecContext(ctx, `UPDATE jobs SET state = 'failed' WHERE id = ?`, gone); err != nil {
		t.Fatal(err)
	}

	// Without a type only state=failed; the rest is 422 from the handler.
	rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/jobs?state=queued", "p18", "", nil)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	var p gen.Problem
	decode(t, rec, &p)
	if p.Errors == nil || len(*p.Errors) != 1 || (*p.Errors)[0] != (gen.FieldError{Field: "type", Code: "required"}) {
		t.Errorf("listJobRuns without type: %s, want type required", rec.Body.String())
	}
	page := func(query string) gen.JobRunPage {
		t.Helper()
		rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/jobs?"+query, "p18", "", nil)
		wantStatus(t, rec, http.StatusOK)
		var out gen.JobRunPage
		decode(t, rec, &out)
		return out
	}
	if all := page("state=failed"); all.Total != 4 || len(all.Runs) != 4 {
		t.Fatalf("failed of every type = %d of %d, want 4 of 4", len(all.Runs), all.Total)
	}
	first, second := page("state=failed&limit=3"), page("state=failed&limit=3&offset=3")
	if first.Total != 4 || len(first.Runs) != 3 || len(second.Runs) != 1 || second.Runs[0].Id == first.Runs[0].Id ||
		second.Runs[0].Id == first.Runs[1].Id || second.Runs[0].Id == first.Runs[2].Id {
		t.Fatalf("pages = %+v / %+v, want 3 then the fourth", first.Runs, second.Runs)
	}
	if a := page("type=" + typeA + "&state=failed&offset=1"); a.Total != 2 || len(a.Runs) != 1 {
		t.Fatalf("failed of %s from 1 = %d of %d, want 1 of 2", typeA, len(a.Runs), a.Total)
	}

	wantStatus(t, send(t, pf.h, http.MethodPost, BasePath+"/admin/jobs/failed/retry?type=nobody.home", "p18", "", nil), http.StatusNotFound)
	wantStatus(t, send(t, pf.h, http.MethodDelete, BasePath+"/admin/jobs/failed?type=nobody.home", "p18", "", nil), http.StatusNotFound)
	rec = send(t, pf.h, http.MethodPost, BasePath+"/admin/jobs/failed/retry?type="+typeA, "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var retried gen.JobRetryFailedResult
	decode(t, rec, &retried)
	if retried != (gen.JobRetryFailedResult{Retried: 2}) {
		t.Errorf("retry of %s = %+v, want 2 retried", typeA, retried)
	}
	// Every type: typeB is queued, the unregistered type stays failed and is no skip.
	rec = send(t, pf.h, http.MethodPost, BasePath+"/admin/jobs/failed/retry", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &retried)
	if retried != (gen.JobRetryFailedResult{Retried: 1}) {
		t.Errorf("retry of every type = %+v, want 1 retried (typeB only)", retried)
	}
	if left := page("state=failed"); left.Total != 1 || left.Runs[0].Id != gone {
		t.Fatalf("failed after the retry = %+v, want the unregistered one", left.Runs)
	}
	rec = send(t, pf.h, http.MethodDelete, BasePath+"/admin/jobs/failed", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var dismissed gen.JobDismissResult
	decode(t, rec, &dismissed)
	if dismissed.Dismissed != 1 {
		t.Errorf("dismiss = %+v, want 1", dismissed)
	}
	if left := page("state=failed"); left.Total != 0 {
		t.Errorf("failed after the dismiss = %d, want 0", left.Total)
	}
}
