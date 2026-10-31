package library

import (
	"context"
	"fmt"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
)

// The cancel hook of scan.library (concept 09, 6.3): the administration's cancel of a queued scan
// job ends the library aborted/cancelled like the button on the library page, instead of leaving
// it "queued" until the next scheduled run. A deleted library is no error.
func TestCancelHookEndsLibraryAborted(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.create(t.TempDir()) // Create queues a scan
	if got, _ := e.svc.Get(e.ctx, lib.ID); got.Scan.State != "queued" {
		t.Fatalf("scan state after create = %q, want queued", got.Scan.State)
	}
	r := jobs.NewRegistry(e.queue, discard)
	e.svc.RegisterTypes(r)
	n, err := r.CancelRuns(e.ctx, JobScanLibrary, false)
	if err != nil || n != 1 {
		t.Fatalf("cancel queued scans: %d %v", n, err)
	}
	got, err := e.svc.Get(e.ctx, lib.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Scan.State != "aborted" || got.Scan.LastError != "cancelled" {
		t.Errorf("library after the cancel = %+v, want aborted/cancelled", got.Scan)
	}
	if e.queuedScans() != 0 {
		t.Errorf("queued scans after the cancel = %d", e.queuedScans())
	}
	// The hook of a job of a library that no longer exists is no error.
	ty, _ := r.Lookup(JobScanLibrary)
	if err := ty.Cancel(context.Background(), jobs.Job{Type: JobScanLibrary, Payload: []byte(fmt.Sprintf(`{"libraryId":%d}`, 4711))}); err != nil {
		t.Errorf("hook for a deleted library: %v", err)
	}
	if ty, _ := r.Lookup(JobScanPath); ty.Cancel == nil {
		t.Error("scan.path has no cancel hook")
	}
}
