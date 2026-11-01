package library

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"sync"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// activityRecorder collects what the scanner reports to the activity log (concept 11).
type activityRecorder struct {
	mu     sync.Mutex
	events []model.ActivityEvent
}

func (r *activityRecorder) record(_ context.Context, e model.ActivityEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *activityRecorder) take() []model.ActivityEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.events
	r.events = nil
	return out
}

func kinds(events []model.ActivityEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Kind)
	}
	return out
}

// Concept 11 (11.4): a full scan reports its start with the trigger and its end with the new
// titles; a path scan reports nothing while it runs through, but its abort is reported like the
// abort of a full scan, with the fixed code only.
func TestScanActivity(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	rec := &activityRecorder{}
	e.svc.WithActivity(rec.record)
	dir := tree(t, twentyMovies[:5])
	lib := e.create(dir)

	e.scan(lib.ID)
	got := rec.take()
	if !slices.Equal(kinds(got), []string{model.ActivityScanStarted, model.ActivityScanDone}) {
		t.Fatalf("full scan reported %v", kinds(got))
	}
	if s := got[0]; s.LibraryID != lib.ID || s.Subject != "Filme" || s.Detail.Trigger != model.ScanTriggerManual {
		t.Errorf("scan_started = %+v", s)
	}
	if d := got[1]; d.LibraryID != lib.ID || d.Detail.NewTitles == nil || *d.Detail.NewTitles != 5 {
		t.Errorf("scan_done = %+v, want 5 new titles", d)
	}

	writeFile(t, dir, "Neu (2020)/Neu (2020).mkv", "neu")
	if s := e.pathScan(lib, ".", "Neu (2020)"); s.Scan.State != "done" {
		t.Fatalf("path scan state %q", s.Scan.State)
	}
	if got := rec.take(); len(got) != 0 {
		t.Errorf("a path scan that ran through reported %v", kinds(got))
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if s := e.pathScan(lib, "."); s.Scan.State != "aborted" {
		t.Fatalf("path scan state %q", s.Scan.State)
	}
	got = rec.take()
	if len(got) != 1 || got[0].Kind != model.ActivityScanAborted || got[0].Detail.Reason != abortRootMissing || got[0].LibraryID != lib.ID {
		t.Errorf("aborted path scan reported %+v", got)
	}
}

// The trigger of a full scan travels in the job payload: the schedule queues "schedule", a
// change seen by a watcher "watcher", the administration "manual".
func TestScanTriggerInPayload(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.create(tree(t, twentyMovies[:1]))
	payload := func() libraryPayload {
		t.Helper()
		var raw string
		if err := e.db.Read.QueryRowContext(e.ctx, `SELECT payload FROM jobs WHERE type = ? AND state = 'queued'`, JobScanLibrary).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var p libraryPayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p := payload(); p.LibraryID != lib.ID || p.Trigger != model.ScanTriggerManual {
		t.Errorf("payload of the first scan = %+v", p)
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `DELETE FROM jobs`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.runScans(e.ctx, e.queue, 1); err != nil {
		t.Fatal(err)
	}
	if p := payload(); p.Trigger != model.ScanTriggerSchedule {
		t.Errorf("payload of the scheduled scan = %+v", p)
	}
	if _, err := e.db.Write.ExecContext(e.ctx, `DELETE FROM jobs`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.enqueueScan(e.ctx, lib.ID, jobs.DebounceWindow, prioScan, model.ScanTriggerWatcher); err != nil {
		t.Fatal(err)
	}
	if p := payload(); p.Trigger != model.ScanTriggerWatcher {
		t.Errorf("payload of the watcher scan = %+v", p)
	}
}
