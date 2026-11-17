package wishes

import (
	"errors"
	"strings"
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
	if err := r.svc.RetryWish(r.ctx, res.Wish.ID, 1); err != nil {
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
	if err := r.svc.RetryWish(r.ctx, res.Wish.ID, 1); !errors.Is(err, model.ErrWishNotRetryable) {
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
	if err := r.svc.RetryWish(r.ctx, res.Wish.ID, 1); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishProcessing || r.fake.Calls("POST /request/{id}/retry") != 1 {
		t.Errorf("arr_failed retried: %+v, %d calls", w.Wish, r.fake.Calls("POST /request/{id}/retry"))
	}
	// Seerr no longer reports it as failed: refused, nothing changes.
	r.exec(`UPDATE wishes SET state = 'failed', fail_reason = 'arr_failed', closed_at = 1 WHERE id = ?`, res.Wish.ID)
	if err := r.svc.RetryWish(r.ctx, res.Wish.ID, 1); !errors.Is(err, model.ErrWishNotRetryable) {
		t.Errorf("request not failed in seerr: %v", err)
	}
	if err := r.svc.RetryWish(r.ctx, 99999, 1); !errors.Is(err, model.ErrNotFound) {
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

// SpotlightWishes (concept 25 step 3.3): the open wish of a title before a closed one, none for a
// title without wish; requesting needs Seerr set up and switched on and «Wünsche aktiv».
func TestSpotlightWishes(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	r.exec(`INSERT INTO wishes (id, media, tmdb_id, title, origin, state, created_at, updated_at, closed_at)
	        VALUES (801, 'movie', ?, 'Matrix', 'slouchcat', 'declined', 1, ?, 1), (802, 'movie', ?, 'Alien', 'slouchcat', 'declined', 1, 1, 1)`,
		matrix, r.clock.now().Add(time.Hour).UnixMilli(), alien)
	r.cfg.set(func(c *fakeConfig) { c.wish.Enabled = true })
	keys := []model.TitleKey{{Media: "movie", TMDBID: matrix}, {Media: "movie", TMDBID: alien}, {Media: "show", TMDBID: 1}}
	marks, can, err := r.svc.SpotlightWishes(r.ctx, keys)
	if err != nil {
		t.Fatal(err)
	}
	if m := marks[keys[0]]; m.ID != res.Wish.ID || len(marks) != 2 || marks[keys[1]].State != model.WishDeclined || !can {
		t.Errorf("marks %+v, can %v", marks, can)
	}
	r.cfg.set(func(c *fakeConfig) { c.wish.Enabled = false })
	if _, can, _ := r.svc.SpotlightWishes(r.ctx, nil); can {
		t.Error("requestable with the wishes off")
	}
}

// Two wishes failed on one FAILED request: retrying one takes the requesters of the other along as
// co-wishers, the other stays closed (review Minor 5); the event names the acting account
// (security review N-2).
func TestRetryTakesOtherFailedWishesAlong(t *testing.T) {
	r := newRequestFixture(t)
	a := r.create(r.marco, film(matrix))
	reqID := r.wish(a.Wish.ID).SeerrRequestID
	r.exec(`UPDATE wishes SET state = 'failed', fail_reason = 'arr_failed', closed_at = 1 WHERE id = ?`, a.Wish.ID)
	b := r.create(r.gast, film(matrix))
	if !b.Created || r.wish(b.Wish.ID).SeerrRequestID != reqID {
		t.Fatalf("second wish = %+v", r.wish(b.Wish.ID))
	}
	r.exec(`UPDATE wishes SET state = 'failed', fail_reason = 'arr_failed', closed_at = 1 WHERE id = ?`, b.Wish.ID)
	r.fake.SetRequestStatus(reqID, seerrtest.RequestFailed)
	if err := r.svc.RetryWish(r.ctx, a.Wish.ID, 7); err != nil {
		t.Fatal(err)
	}
	w := r.wish(a.Wish.ID)
	joined := false
	for _, q := range w.Requesters {
		joined = joined || (q.ProfileID == r.gast.ID && q.Role == model.WishRoleJoined)
	}
	if w.State != model.WishProcessing || !joined {
		t.Errorf("retried wish = %+v", w)
	}
	if other := r.wish(b.Wish.ID); other.State != model.WishFailed {
		t.Errorf("other wish = %+v", other)
	}
	if kind, _ := r.lastEvent(b.Wish.ID); kind != eventRetriedWith {
		t.Errorf("event of the other wish = %s", kind)
	}
	var detail string
	if err := r.db.Read.QueryRowContext(r.ctx, `SELECT detail FROM wish_events WHERE wish_id = ? AND kind = 'retried'`, a.Wish.ID).Scan(&detail); err != nil || !strings.Contains(detail, `"accountId":7`) {
		t.Errorf("retried event = %s, %v", detail, err)
	}
}
