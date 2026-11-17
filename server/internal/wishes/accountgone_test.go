package wishes

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// Wishes of a deleted account or profile and the sender of a queued wish (architecture 8.1, users
// and profiles 8.1 and 10, AK-128; security review L-1, review M-3, Minor 9).

func (r *rfix) deleteAccount(id int64) {
	r.t.Helper()
	a := auth.New(r.db, r.log).WithAccountDeleting(r.svc.AccountDeleting)
	if err := a.DeleteAccount(r.ctx, id, nil); err != nil {
		r.t.Fatal(err)
	}
}

func (r *rfix) lastEvent(wishID int64) (string, string) {
	r.t.Helper()
	var kind, detail string
	if err := r.db.Read.QueryRowContext(r.ctx, `SELECT kind, coalesce(detail, '') FROM wish_events WHERE wish_id = ? ORDER BY at DESC, id DESC LIMIT 1`, wishID).Scan(&kind, &detail); err != nil {
		r.t.Fatal(err)
	}
	return kind, detail
}

// AK-128: an open wish of a deleted account without co-wishers fails with account_deleted, gets
// closed_at and gives its request id to the history; the request stays in Seerr and the title is
// free for a new wish of another account.
func TestDeleteAccountClosesItsWishes(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.gast, film(matrix))
	if res.Wish.State != model.WishPending {
		t.Fatalf("state = %s", res.Wish.State)
	}
	reqID := r.fake.Requests()[0].ID
	r.deleteAccount(2)
	w := r.wish(res.Wish.ID)
	if w.State != model.WishFailed || w.FailReason != model.WishFailAccountDeleted || w.SeerrRequestID != 0 || len(w.Requesters) != 0 {
		t.Fatalf("wish of the deleted account = %+v", w)
	}
	if n := r.count(`SELECT count(*) FROM wishes WHERE id = ? AND closed_at IS NOT NULL`, res.Wish.ID); n != 1 {
		t.Error("closed_at not set")
	}
	if kind, detail := r.lastEvent(res.Wish.ID); kind != eventAccountDeleted || !strings.Contains(detail, `"requestIds":[`) || !strings.Contains(detail, strconv.Itoa(reqID)) {
		t.Errorf("event = %s %s", kind, detail)
	}
	if n := len(r.fake.Requests()); n != 1 {
		t.Errorf("requests in seerr = %d, the request must stay", n)
	}
	// Another account wishes the same film: it attaches to the request that stayed.
	again := r.create(r.marco, film(matrix))
	if !again.Created || r.wish(again.Wish.ID).SeerrRequestID != reqID {
		t.Errorf("new wish = %+v", again)
	}
}

// A wish of a deleted account with a co-wisher of another account goes to the oldest of them; a
// queued wish then goes out as the new owner's Seerr user.
func TestDeleteAccountHandsWishesOver(t *testing.T) {
	r := newRequestFixture(t)
	r.gast2() // account 2 sends as user 4
	r.fake.SetFailure("POST /request", seerrtest.Failure{Status: 503, Message: "down"})
	res := r.create(r.gast, film(matrix))
	if res.Wish.State != model.WishQueued || r.wish(res.Wish.ID).SeerrUserID != 4 {
		t.Fatalf("queued = %+v", r.wish(res.Wish.ID))
	}
	r.clock.advance(time.Second)
	r.create(r.noah, film(matrix))
	r.clock.advance(time.Second)
	r.create(r.marco, film(matrix))
	r.deleteAccount(2)
	w := r.wish(res.Wish.ID)
	if w.State != model.WishQueued || w.SeerrUserID != 3 || len(w.Requesters) != 2 || w.Requesters[0].ProfileID != r.noah.ID || w.Requesters[0].Role != model.WishRoleOwner {
		t.Fatalf("handed over = %+v", w)
	}
	r.fake.SetFailure("POST /request", seerrtest.Failure{})
	r.fake.ResetCalls()
	if err := r.submitJob(res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if r.fake.Calls("POST /request") != 1 {
		t.Errorf("posts = %d", r.fake.Calls("POST /request"))
	}
	if reqs := r.fake.Requests(); len(reqs) != 1 || reqs[0].UserID != 3 {
		t.Errorf("seerr requests = %+v", reqs)
	}
}

// A deleted profile's wishes go to the primary profile of its account; a wish the primary
// profile joined already keeps one row, now the owner.
func TestDeleteProfileHandsWishesToPrimary(t *testing.T) {
	r := newRequestFixture(t)
	a := r.create(r.noah, film(matrix))
	b := r.create(r.noah, film(alien))
	r.create(r.marco, film(alien))
	if err := r.svc.ProfileDeleting(r.ctx, 1, r.noah.ID); err != nil {
		t.Fatal(err)
	}
	r.exec(`DELETE FROM profiles WHERE id = 2`)
	for _, id := range []int64{a.Wish.ID, b.Wish.ID} {
		w := r.wish(id)
		if len(w.Requesters) != 1 || w.Requesters[0].ProfileID != r.marco.ID || w.Requesters[0].Role != model.WishRoleOwner || !isOpen(w.State) {
			t.Errorf("wish %d = %+v", id, w)
		}
		if kind, detail := r.lastEvent(id); kind != "owner_changed" || !strings.Contains(detail, "profile_deleted") {
			t.Errorf("event of %d = %s %s", id, kind, detail)
		}
	}
}

// The frozen user of a queued wish keeps sending after the account was unlinked (papers 8.1); it
// does not once it belongs to another account: seerr_user_gone with mapping_changed, no POST.
func TestQueuedWishSenderResolvedAgain(t *testing.T) {
	r := newRequestFixture(t)
	r.fake.SetFailure("POST /request", seerrtest.Failure{Status: 503, Message: "down"})
	kept := r.create(r.marco, film(alien))
	moved := r.create(r.marco, film(matrix))
	if _, err := r.svc.SetAccountMapping(r.ctx, 1, nil, 1); err != nil {
		t.Fatal(err)
	}
	r.fake.SetFailure("POST /request", seerrtest.Failure{})
	if err := r.submitJob(kept.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(kept.Wish.ID); w.State != model.WishPending || w.SeerrUserID != 3 {
		t.Errorf("unlinked account, frozen user = %+v", w)
	}
	if _, err := r.svc.SetAccountMapping(r.ctx, 2, ip(3), 1); err != nil { // user 3 now belongs to gast
		t.Fatal(err)
	}
	r.fake.ResetCalls()
	if err := r.submitJob(moved.Wish.ID); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(moved.Wish.ID); w.State != model.WishFailed || w.FailReason != model.WishFailSeerrUserGone {
		t.Errorf("user of another account = %+v", w)
	}
	if _, detail := r.lastEvent(moved.Wish.ID); !strings.Contains(detail, "mapping_changed") {
		t.Errorf("event detail = %s", detail)
	}
	if n := r.fake.Calls("POST /request"); n != 0 {
		t.Errorf("sent as another account's user: %d", n)
	}
}

// A withdrawal of a wish whose frozen user is gone from Seerr has nothing left to delete.
func TestWithdrawOfGoneUserCallsNothing(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	if err := r.svc.WithdrawWish(r.ctx, r.marco, res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	r.exec(`UPDATE seerr_users SET gone_at = 1 WHERE seerr_id = 3`)
	r.fake.ResetCalls()
	r.runJobs(JobWithdraw)
	if n := len(r.fake.CallLog()); n != 0 {
		t.Errorf("calls = %+v", r.fake.CallLog())
	}
	if n := r.count(`SELECT count(*) FROM wishes WHERE withdraw_pending = 1`); n != 0 {
		t.Error("mark not cleared")
	}
}

// What Seerr took for a wish that left the outbox meanwhile (closed with its account) is not
// written over the closed state; the event keeps the request id (review Minor 9).
func TestAcceptedOnlyFromQueued(t *testing.T) {
	r := newRequestFixture(t)
	r.fake.SetFailure("POST /request", seerrtest.Failure{Status: 503, Message: "down"})
	res := r.create(r.marco, film(matrix))
	w, err := r.svc.read.GetWish(r.ctx, res.Wish.ID)
	if err != nil {
		t.Fatal(err)
	}
	r.exec(`UPDATE wishes SET state = 'failed', fail_reason = 'account_deleted', closed_at = 1 WHERE id = ?`, w.ID)
	if err := r.svc.accepted(r.ctx, w, sent{how: "submitted", film: &seerr.Request{ID: 77, Status: seerr.RequestPending}}); err != nil {
		t.Fatal(err)
	}
	got := r.wish(w.ID)
	if got.State != model.WishFailed || got.FailReason != model.WishFailAccountDeleted || got.SeerrRequestID != 0 {
		t.Errorf("wish = %+v", got)
	}
	if kind, detail := r.lastEvent(w.ID); kind != "submitted" || !strings.Contains(detail, "77") {
		t.Errorf("event = %s %s", kind, detail)
	}
}
