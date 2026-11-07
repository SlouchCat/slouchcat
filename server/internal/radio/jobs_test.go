package radio

import (
	"net/http"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
)

// catJobs adds a queue, a registry with the radio types and a started scheduler to e.
func catJobs(t *testing.T) (*catEnv, *jobs.Queue, *jobs.Scheduler) {
	t.Helper()
	var q *jobs.Queue
	var sched *jobs.Scheduler
	e := newCatEnv(t)
	q = jobs.NewQueue(e.db.Write, e.clock.Now)
	reg := jobs.NewRegistry(q, catDiscard)
	sched = jobs.NewScheduler(q, reg, catDiscard, jobs.WithLocation(time.UTC))
	e.s.o.queue, e.s.o.scheduler = q, sched
	e.s.RegisterTypes(reg)
	if err := sched.Start(e.ctx); err != nil {
		t.Fatal(err)
	}
	return e, q, sched
}

// The schedule row is daily in the window 04:30 to 06:30; a start with an empty catalog sets the
// one-off date ten minutes ahead once, and a synced catalog sets none.
func TestFirstRunOnceDate(t *testing.T) {
	e, _, sched := catJobs(t)
	row, err := sched.Schedule(e.ctx, JobCatalogSync)
	if err != nil {
		t.Fatal(err)
	}
	if row.Every != 24*time.Hour || !row.Enabled || row.Window == nil || *row.Window != catalogSyncWindow || row.StartDelay != nil {
		t.Errorf("schedule = %+v", row)
	}
	if want := time.Date(2026, 11, 8, 4, 30, 0, 0, time.UTC); !row.NextRunAt.Equal(want) {
		t.Errorf("first run at %v, want %v", row.NextRunAt, want)
	}
	if err := e.s.Start(e.ctx); err != nil {
		t.Fatal(err)
	}
	row, _ = sched.Schedule(e.ctx, JobCatalogSync)
	if want := e.clock.Now().Add(10 * time.Minute); !row.OnceAt.Equal(want) {
		t.Fatalf("one-off date = %v, want %v", row.OnceAt, want)
	}
	// A second start keeps the date.
	e.clock.Advance(time.Minute)
	if err := e.s.Start(e.ctx); err != nil {
		t.Fatal(err)
	}
	if again, _ := sched.Schedule(e.ctx, JobCatalogSync); !again.OnceAt.Equal(row.OnceAt) {
		t.Errorf("second start moved the date to %v", again.OnceAt)
	}
	// The date fires one sync.
	e.clock.Advance(10 * time.Minute)
	if err := sched.Tick(e.ctx); err != nil {
		t.Fatal(err)
	}
	cs, err := e.s.CatalogState(e.ctx)
	if err != nil || cs.RunningJobID == 0 {
		t.Fatalf("no sync queued by the date: %+v %v", cs, err)
	}
	e.sync()
	if err := e.s.Start(e.ctx); err != nil {
		t.Fatal(err)
	}
	if row, _ := sched.Schedule(e.ctx, JobCatalogSync); !row.OnceAt.IsZero() {
		t.Errorf("start after a sync set the date %v", row.OnceAt)
	}
}

// SyncCatalog queues one job with manual priority and names it again while it waits.
func TestSyncCatalogQueuesOnce(t *testing.T) {
	e, q, _ := catJobs(t)
	id, err := e.s.SyncCatalog(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.s.SyncCatalog(e.ctx)
	if err != nil || again != id {
		t.Errorf("second request = %d %v, want %d", again, err, id)
	}
	j, err := q.Get(e.ctx, id)
	if err != nil || j.Priority != jobs.ManualPriority || j.MaxAttempts != 1 {
		t.Errorf("job = %+v %v", j, err)
	}
	if cs, _ := e.s.CatalogState(e.ctx); cs.RunningJobID != id {
		t.Errorf("running job id = %d, want %d", cs.RunningJobID, id)
	}
}

// A failed scheduled sync sets the retry as the one-off date: 15 min after the first failure.
func TestSyncFailureSetsRetry(t *testing.T) {
	e, _, sched := catJobs(t)
	e.answer(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "down", http.StatusServiceUnavailable) })
	if err := e.s.handleCatalogSync(e.ctx, nil); err == nil {
		t.Fatal("sync against a 503 succeeded")
	}
	row, err := sched.Schedule(e.ctx, JobCatalogSync)
	if err != nil {
		t.Fatal(err)
	}
	if want := e.clock.Now().Add(15 * time.Minute); !row.OnceAt.Equal(want) {
		t.Errorf("retry date = %v, want %v", row.OnceAt, want)
	}
}
