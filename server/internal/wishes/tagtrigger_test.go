package wishes

import (
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/seerr"
)

// The triggers of wishes.tag (architecture 13): after every sync, when a wish becomes processing
// and when a co-wisher joins.

func (r *rfix) clearQueue() {
	r.queue.mu.Lock()
	r.queue.jobs = nil
	r.queue.mu.Unlock()
}

func TestTagQueuedAfterEverySync(t *testing.T) {
	r := syncFixture(t)
	r.setLastSync(r.clock.now())
	r.clearQueue()
	r.sync(false)
	if n := r.queue.count(JobTag); n != 1 {
		t.Errorf("tag runs after the sync = %d", n)
	}
	r.clearQueue()
	r.cfg.set(func(c *fakeConfig) { c.seer.Enabled = false })
	r.sync(false)
	if n := r.queue.count(JobTag); n != 0 {
		t.Errorf("tag runs while Seerr rests = %d", n)
	}
}

func TestTagQueuedWhenWishBecomesProcessing(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix)) // pending
	if n := r.queue.count(JobTag); n != 0 {
		t.Fatalf("tag runs for a pending wish = %d", n)
	}
	if _, err := r.svc.ApproveWish(r.ctx, res.Wish.ID, 1); err != nil {
		t.Fatal(err)
	}
	if n := r.queue.count(JobTag); n != 1 {
		t.Errorf("tag runs after the approval = %d", n)
	}
	// Created processing at once (attached to an approved request).
	r.clearQueue()
	r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: alien})
	r.create(r.marco, film(alien))
	if n := r.queue.count(JobTag); n != 1 {
		t.Errorf("tag runs after an attached wish = %d", n)
	}
}

func TestTagQueuedWhenCoWisherJoins(t *testing.T) {
	r := newRequestFixture(t)
	r.create(r.marco, film(matrix))
	r.clock.advance(time.Minute)
	r.clearQueue()
	res := r.create(r.gast, film(matrix))
	if len(res.Joined) != 1 {
		t.Fatalf("joined = %+v", res)
	}
	if n := r.queue.count(JobTag); n != 1 {
		t.Errorf("tag runs after a co-wish = %d", n)
	}
}
