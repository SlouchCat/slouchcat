package offline

import (
	"context"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// The cancel hook of download.prepare (concept 09, 6.3, F-2): a download whose preparation the
// administration cancelled ends failed with the code cancelled_by_admin, and the reconciliation
// does not queue it again (without the hook Reconcile queued every unfinished download without a
// job after at most 10 minutes). The viewer can start it anew.
func TestCancelledDownloadIsNotQueuedAgain(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	st := e.create(e.f.P18, e.devA, e.f.Rated(0).ID, model.DownloadLow)
	if e.pendingJobs()[primaryKey(st.ID)].IsZero() {
		t.Fatalf("no preparation job after create: %v", e.pendingJobs())
	}
	r := jobs.NewRegistry(e.queue, vistest.Logger())
	e.svc.RegisterTypes(r)
	n, err := r.CancelRuns(ctx, JobPrepare, false)
	if err != nil || n != 1 {
		t.Fatalf("cancel queued preparations: %d %v", n, err)
	}
	if row := e.row(st.ID); row.state != model.DownloadFailed || row.err != model.DownloadErrCancelled {
		t.Fatalf("download after the cancel = %+v, want failed/cancelled_by_admin", row)
	}
	if len(e.pendingJobs()) != 0 {
		t.Fatalf("jobs after the cancel = %v", e.pendingJobs())
	}
	if err := e.svc.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if row := e.row(st.ID); row.state != model.DownloadFailed || len(e.pendingJobs()) != 0 {
		t.Errorf("after the reconciliation: %+v, jobs %v; want the cancelled download left alone", row, e.pendingJobs())
	}
	// A ready download is not touched by the hook.
	ready := e.create(e.f.P18, e.devB, e.f.Rated(0).ID, model.DownloadLow)
	e.runJobs()
	if row := e.row(ready.ID); row.state != model.DownloadReady {
		t.Fatalf("second download not ready: %+v", row)
	}
	if err := e.svc.CancelPreparation(ctx, ready.ID); err != nil {
		t.Fatal(err)
	}
	if row := e.row(ready.ID); row.state != model.DownloadReady {
		t.Errorf("ready download after CancelPreparation = %+v, want untouched", row)
	}
}
