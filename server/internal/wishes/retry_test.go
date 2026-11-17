package wishes

import (
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// retryWish (architecture 8.1, 8.2): a submission failure goes back to the outbox with 24 h anew; a
// request Seerr reports as failed is retried in Seerr; other states are refused.
func TestRetryWish(t *testing.T) {
	r := newRequestFixture(t)
	r.fake.SetFailure("*", seerrtest.Failure{Status: 503})
	res := r.create(r.marco, film(matrix))
	r.clock.advance(25 * time.Hour)
	if err := r.submitJob(res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishFailed || w.FailReason != model.WishFailUnreachable24h {
		t.Fatalf("not failed: %+v", w.Wish)
	}
	if err := r.svc.RetryWish(r.ctx, res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishQueued || w.FailReason != "" || w.Events[0].Kind != eventRetried {
		t.Fatalf("after the retry: %+v", w.Wish)
	}
	// Seerr still down: the outbox waits instead of failing at once, because the 24 h count anew.
	var later *jobs.RetryLaterError
	if err := r.submitJob(res.Wish.ID); !errors.As(err, &later) {
		t.Fatalf("attempt after the retry: %v", err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishQueued {
		t.Errorf("failed again at once: %+v", w.Wish)
	}
	if err := r.svc.RetryWish(r.ctx, res.Wish.ID); !errors.Is(err, model.ErrWishNotRetryable) {
		t.Errorf("retry of a queued wish: %v", err)
	}
	// Seerr back: sent; Seerr reports the request failed (arr_failed); the retry asks Seerr.
	r.fake.SetFailure("*", seerrtest.Failure{})
	if err := r.submitJob(res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	w := r.wish(res.Wish.ID)
	r.exec(`UPDATE wishes SET state = 'failed', fail_reason = 'arr_failed', closed_at = 1 WHERE id = ?`, res.Wish.ID)
	r.fake.SetRequestStatus(w.SeerrRequestID, seerrtest.RequestFailed)
	if err := r.svc.RetryWish(r.ctx, res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishProcessing || r.fake.Calls("POST /request/{id}/retry") != 1 {
		t.Errorf("arr_failed retried: %+v, %d calls", w.Wish, r.fake.Calls("POST /request/{id}/retry"))
	}
	// Seerr no longer reports it as failed: refused, nothing changes.
	r.exec(`UPDATE wishes SET state = 'failed', fail_reason = 'arr_failed', closed_at = 1 WHERE id = ?`, res.Wish.ID)
	if err := r.svc.RetryWish(r.ctx, res.Wish.ID); !errors.Is(err, model.ErrWishNotRetryable) {
		t.Errorf("request not failed in seerr: %v", err)
	}
	if err := r.svc.RetryWish(r.ctx, 99999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown wish: %v", err)
	}
}

// TitlesInUse names the titles of open wishes and of wishes closed since a time (titles.refresh,
// migration 00074); an older closed wish names none.
func TestTitlesInUse(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	r.exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, fail_reason, created_at, updated_at, closed_at)
	        VALUES (801, 'movie', ?, 'Alien', 'slouchcat', 'failed', 'quota', 1, 1, ?), (802, 'show', 77, 'Alt', 'slouchcat', 'declined', NULL, 1, 1, 5)`,
		alien, r.clock.now().UnixMilli())
	open, recent, err := r.svc.TitlesInUse(r.ctx, r.clock.now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].TMDBID != matrix || res.Wish == nil || len(recent) != 1 || recent[0].TMDBID != alien {
		t.Errorf("open %v, recent %v", open, recent)
	}
}
