package wishes

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// Wishes that got stuck in the outbox (review M-1, M-2, M-4, M-5, Minor 4, 5).

// deleteInSeerr deletes a pending request in Seerr as its user, past Slouchcat.
func (r *rfix) deleteInSeerr(user, id int) {
	r.t.Helper()
	c, err := seerr.New(r.fake.URL, seerrtest.APIKey)
	if err != nil {
		r.t.Fatal(err)
	}
	if err := c.AsUser(user).DeleteRequest(r.ctx, id); err != nil {
		r.t.Fatalf("delete request %d: %v", id, err)
	}
}

// M-1: a season of a queued series wish arrives in the library another way; the wish stays queued
// and the outbox still sends the other season.
func TestQueuedSeasonStillSentWhenAnotherIsAvailable(t *testing.T) {
	r := newRequestFixture(t)
	r.fake.SetFailure("POST /request", seerrtest.Failure{Status: 503, Message: "down"})
	res := r.create(r.marco, show(got, 1, 2))
	if res.Wish.State != model.WishQueued {
		t.Fatalf("state = %s", res.Wish.State)
	}
	r.addTitle("show", got, itemGot, map[int]int{1: 10})
	if err := r.svc.reconcile(r.ctx, reconcilePayload{}); err != nil {
		t.Fatal(err)
	}
	w := r.wish(res.Wish.ID)
	if w.State != model.WishQueued || w.Seasons[0].State != model.WishAvailable || w.Seasons[1].State != model.WishQueued {
		t.Fatalf("after the reconcile = %+v", w)
	}
	due, err := r.svc.read.ListDueWishes(r.ctx, wishq.ListDueWishesParams{Now: num64(r.clock.now().Add(time.Hour).UnixMilli()), LimitCount: 10})
	if err != nil || len(due) != 1 {
		t.Fatalf("outbox = %d, %v", len(due), err)
	}
	r.fake.SetFailure("POST /request", seerrtest.Failure{})
	if err := r.submitJob(res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	reqs := r.fake.Requests()
	if len(reqs) != 1 || len(reqs[0].Seasons) != 1 || reqs[0].Seasons[0] != 2 {
		t.Fatalf("seerr requests = %+v", reqs)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishPartial || w.Seasons[1].SeerrRequestID != reqs[0].ID {
		t.Errorf("after the submission = %+v", w)
	}
}

// M-2: a retried series wish whose request was deleted in Seerr sends its season again instead of
// looping «attached»; the old request id goes into the event.
func TestRetriedSeriesWishSendsAgain(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, show(got, 1))
	old := r.wish(res.Wish.ID).Seasons[0].SeerrRequestID
	if old == 0 {
		t.Fatalf("no request: %+v", r.wish(res.Wish.ID))
	}
	r.deleteInSeerr(3, old)
	r.exec(`UPDATE wish_seasons SET state = 'failed' WHERE wish_id = ?`, res.Wish.ID)
	r.exec(`UPDATE wishes SET state = 'failed', fail_reason = 'seerr_request_gone', closed_at = 1 WHERE id = ?`, res.Wish.ID)
	if err := r.svc.RetryWish(r.ctx, res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishQueued || w.Seasons[0].State != model.WishQueued || w.Seasons[0].SeerrRequestID != 0 {
		t.Fatalf("retried = %+v", w)
	}
	if _, detail := r.lastEvent(res.Wish.ID); !strings.Contains(detail, `"requestIds":[`+strconv.Itoa(old)+`]`) {
		t.Errorf("retried event = %s", detail)
	}
	r.runJobs(JobSubmit)
	w := r.wish(res.Wish.ID)
	if w.State != model.WishPending || w.Seasons[0].SeerrRequestID == 0 || w.Seasons[0].SeerrRequestID == old {
		t.Errorf("after the submission = %+v", w)
	}
	if n := r.queue.count(JobSubmit); n != 0 {
		t.Errorf("still in the outbox: %d jobs", n)
	}
}

// M-4: a FAILED request locks the title in Seerr; a new wish attaches to it and fails with
// arr_failed at once, without 24 h of retries.
func TestConflictWithFailedRequestFailsAtOnce(t *testing.T) {
	r := newRequestFixture(t)
	failed := r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: matrix})
	r.fake.SetRequestStatus(failed.ID, seerrtest.RequestFailed)
	res := r.create(r.marco, film(matrix))
	w := r.wish(res.Wish.ID)
	if w.State != model.WishFailed || w.FailReason != model.WishFailArrFailed || w.SeerrRequestID != failed.ID {
		t.Fatalf("wish = %+v", w)
	}
	if n := r.queue.count(JobSubmit); n != 0 {
		t.Errorf("queued for retries: %d", n)
	}
	// A second profile of another account: attaches to the same FAILED request, no UNIQUE error.
	again := r.create(r.gast, film(matrix))
	if w := r.wish(again.Wish.ID); w.State != model.WishFailed || w.SeerrRequestID != failed.ID {
		t.Errorf("second wish = %+v", w)
	}
}

// M-5: a withdrawn wish still carries the id of its request while the withdrawal waits; another
// account attaches to that request after a 409 without a UNIQUE error, and the withdrawal leaves
// the request to it.
func TestAttachToRequestOfWithdrawnWish(t *testing.T) {
	r := newRequestFixture(t)
	a := r.create(r.marco, film(matrix))
	reqID := r.wish(a.Wish.ID).SeerrRequestID
	if err := r.svc.WithdrawWish(r.ctx, r.marco, a.Wish.ID); err != nil {
		t.Fatal(err)
	}
	b := r.create(r.gast, film(matrix))
	if w := r.wish(b.Wish.ID); w.State != model.WishPending || w.SeerrRequestID != reqID {
		t.Fatalf("second wish = %+v", w)
	}
	r.runJobs(JobWithdraw)
	if _, ok := r.fake.Request(reqID); !ok {
		t.Error("the withdrawal deleted the request another wish carries")
	}
	if w := r.wish(a.Wish.ID); w.State != model.WishWithdrawn {
		t.Errorf("withdrawn wish = %+v", w)
	}
	if n := r.count(`SELECT count(*) FROM wishes WHERE withdraw_pending = 1`); n != 0 {
		t.Error("mark not cleared")
	}
	// The sync finds the open wish for the request.
	got, err := r.svc.read.GetWishBySeerrRequest(r.ctx, num64(int64(reqID)))
	if err != nil || got.ID != b.Wish.ID {
		t.Errorf("wish of the request = %d, %v", got.ID, err)
	}
}

// Minor 4: a withdrawn wish whose request completed with its medium deleted meanwhile is reopened
// as failed with its reason and closing time, not refused by the CHECK.
func TestReopenKeepsFailReason(t *testing.T) {
	r := newRequestFixture(t)
	a := r.create(r.marco, film(matrix))
	if err := r.svc.WithdrawWish(r.ctx, r.marco, a.Wish.ID); err != nil {
		t.Fatal(err)
	}
	id := r.wish(a.Wish.ID).SeerrRequestID
	r.fake.SetRequestStatus(id, seerrtest.RequestCompleted)
	r.fake.SetMediaStatus("movie", matrix, seerrtest.MediaDeleted, nil)
	r.runJobs(JobWithdraw)
	w := r.wish(a.Wish.ID)
	if w.State != model.WishFailed || w.FailReason != model.WishFailMediaDeleted {
		t.Errorf("reopened = %+v", w)
	}
	if n := r.count(`SELECT count(*) FROM wishes WHERE id = ? AND closed_at IS NOT NULL AND withdraw_pending = 0`, a.Wish.ID); n != 1 {
		t.Error("closed_at or the mark wrong")
	}
}

// Minor 5: a series wish whose last open season arrives while the others were declined is closed
// as declined with its closing time.
func TestDeclinedByReconcileGetsClosedAt(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, show(got, 1, 2))
	r.exec(`UPDATE wish_seasons SET state = 'declined' WHERE wish_id = ? AND season_number = 1`, res.Wish.ID)
	r.exec(`UPDATE wish_seasons SET state = 'processing' WHERE wish_id = ? AND season_number = 2`, res.Wish.ID)
	r.exec(`UPDATE wishes SET state = 'processing' WHERE id = ?`, res.Wish.ID)
	r.addTitle("show", got, itemGot, map[int]int{2: 10})
	if err := r.svc.reconcile(r.ctx, reconcilePayload{}); err != nil {
		t.Fatal(err)
	}
	if n := r.count(`SELECT count(*) FROM wishes WHERE id = ? AND state = 'declined' AND closed_at IS NOT NULL`, res.Wish.ID); n != 1 {
		t.Errorf("wish = %+v", r.wish(res.Wish.ID))
	}
}
