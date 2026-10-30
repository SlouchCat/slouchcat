package library

import (
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The Run of scan.library queues a scan of every library with the priority the scheduler passes
// and raises a waiting scan (same key) to a higher priority instead of being swallowed by it; a
// library without a folder queues none (idea 02, step 14).
func TestRunScansRaisesWaitingScan(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.create(t.TempDir())
	e.cancelScans() // the scan queued by Create
	podcasts, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Podcasts", Kind: model.LibraryPodcasts})
	if err != nil {
		t.Fatal(err)
	}
	ids, err := e.svc.runScans(e.ctx, e.queue, prioScan)
	if err != nil || len(ids) != 1 {
		t.Fatalf("scheduled run: %v, %v; want one scan (the podcast library %d has no folder)", ids, err, podcasts.ID)
	}
	if j, err := e.queue.Get(e.ctx, ids[0]); err != nil || j.Priority != prioScan || j.IdempotencyKey != scanKey(lib.ID) {
		t.Fatalf("scheduled scan = %+v, %v", j, err)
	}
	manual, err := e.svc.runScans(e.ctx, e.queue, jobs.ManualPriority)
	if err != nil || len(manual) != 1 || manual[0] != ids[0] {
		t.Fatalf("run by hand: %v, %v; want the waiting scan %d", manual, err, ids[0])
	}
	if j, _ := e.queue.Get(e.ctx, ids[0]); j.Priority != jobs.ManualPriority || j.State != jobs.StateQueued {
		t.Errorf("waiting scan after the run by hand = %+v, want queued at %d", j, jobs.ManualPriority)
	}
	if _, err := e.svc.runScans(e.ctx, e.queue, prioScan); err != nil {
		t.Fatal(err)
	}
	if j, _ := e.queue.Get(e.ctx, ids[0]); j.Priority != jobs.ManualPriority {
		t.Errorf("waiting scan after the next scheduled run = %+v, want still %d (never lowered)", j, jobs.ManualPriority)
	}
	if e.queuedScans() != 1 {
		t.Errorf("queued scans = %d, want the one job under the key", e.queuedScans())
	}
}

// The Run of library.sweep queues one sweep under the key "sweep" and raises a waiting one.
func TestRunSweepRaisesWaitingSweep(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ids, err := e.svc.runSweep(e.ctx, e.queue, prioSweep)
	if err != nil || len(ids) != 1 {
		t.Fatalf("scheduled run: %v, %v", ids, err)
	}
	again, err := e.svc.runSweep(e.ctx, e.queue, jobs.ManualPriority)
	if err != nil || len(again) != 1 || again[0] != ids[0] {
		t.Fatalf("run by hand: %v, %v; want the waiting sweep %d", again, err, ids[0])
	}
	if j, _ := e.queue.Get(e.ctx, ids[0]); j.Priority != jobs.ManualPriority || j.IdempotencyKey != "sweep" {
		t.Errorf("waiting sweep after the run by hand = %+v", j)
	}
	r := jobs.NewRegistry()
	e.svc.RegisterTypes(r)
	for _, name := range []string{JobScanLibrary, JobSweep} {
		ty, ok := r.Lookup(name)
		if !ok || !ty.Scheduled() || ty.StartDelay == nil || *ty.StartDelay != 0 || ty.Run == nil {
			t.Errorf("%s = %+v, want a schedule type that fires at the start", name, ty)
		}
	}
	if len(r.Types()) != 6 {
		t.Errorf("registered types = %d, want the six job types of the library", len(r.Types()))
	}
}
