package wishes

import (
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Build step 4.5: the Seerr webhook only triggers the sync (F-18).

// A burst of notifications queues one run of the sync, delayed by the debounce window, with the
// priority of a manual run; it calls Seerr for nothing and changes no wish.
func TestWebhookBurstQueuesOneSync(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	now := r.clock.now()
	queue := jobs.NewQueue(r.db.Write, func() time.Time { return now })
	svc := New(r.db, r.cfg, queue, r.log, WithClock(r.clock.now))
	r.fake.ResetCalls()
	for range 5 {
		queued, err := svc.SyncFromWebhook(r.ctx)
		if err != nil || !queued {
			t.Fatalf("queued = %v, %v", queued, err)
		}
	}
	if n := r.count(`SELECT count(*) FROM jobs WHERE type = ? AND idempotency_key = ? AND priority = 7 AND run_after = ?`,
		JobSync, JobSync, now.Add(jobs.DebounceWindow).UnixMilli()); n != 1 {
		t.Errorf("sync jobs = %d", n)
	}
	if n := r.count(`SELECT count(*) FROM jobs WHERE type = ?`, JobSync); n != 1 {
		t.Errorf("all sync jobs = %d", n)
	}
	if n := len(r.fake.CallLog()); n != 0 {
		t.Errorf("seerr calls = %d", n)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishPending {
		t.Errorf("state = %s", w.State)
	}
}

// Seerr switched off or not set up: nothing is queued.
func TestWebhookWithoutSeerrQueuesNothing(t *testing.T) {
	r := newRequestFixture(t)
	queue := jobs.NewQueue(r.db.Write, nil)
	svc := New(r.db, r.cfg, queue, r.log, WithClock(r.clock.now))
	r.cfg.set(func(c *fakeConfig) { c.seer.Enabled = false })
	if queued, err := svc.SyncFromWebhook(r.ctx); err != nil || queued {
		t.Errorf("switched off: %v, %v", queued, err)
	}
	r.cfg.set(func(c *fakeConfig) { c.seer = model.SeerrConfig{Enabled: true} })
	if queued, err := svc.SyncFromWebhook(r.ctx); err != nil || queued {
		t.Errorf("not set up: %v, %v", queued, err)
	}
	if n := r.count(`SELECT count(*) FROM jobs WHERE type = ?`, JobSync); n != 0 {
		t.Errorf("sync jobs = %d", n)
	}
}
