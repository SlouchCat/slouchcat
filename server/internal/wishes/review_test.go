package wishes

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// Build step 4.4: approving and declining in the administration pass the choice on to Seerr
// (F-16); the state follows Seerr's answer.

func (r *rfix) countAnnounced(s string) int {
	n := 0
	for _, c := range r.note.got() {
		if c == s {
			n++
		}
	}
	return n
}

// Approving a pending film approves its request in Seerr as user 1, moves the wish to processing,
// announces the approval once through the status channel and names the acting account.
func TestApproveWishMovesToProcessing(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	reqID := r.wish(res.Wish.ID).SeerrRequestID
	r.fake.ResetCalls()
	got, err := r.svc.ApproveWish(r.ctx, res.Wish.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != model.WishProcessing || got.ID != res.Wish.ID {
		t.Errorf("answer = %+v", got.Wish)
	}
	log := r.fake.CallLog()
	if len(log) != 1 || log[0].Route != "POST /request/{id}/{status}" || log[0].Path != fmt.Sprintf("/request/%d/approve", reqID) || log[0].HasAPIUser {
		t.Errorf("calls = %+v", log)
	}
	if req, _ := r.fake.Request(reqID); req.Status != seerrtest.RequestApproved {
		t.Errorf("seerr status = %d", req.Status)
	}
	if n := r.countAnnounced(fmt.Sprintf("approved %d", res.Wish.ID)); n != 1 {
		t.Errorf("approved announced %d times: %v", n, r.note.got())
	}
	if n := r.count(`SELECT count(*) FROM wish_events WHERE wish_id = ? AND kind = 'approved_by_admin'
	                 AND json_extract(detail, '$.accountId') = 1`, res.Wish.ID); n != 1 {
		t.Errorf("approved_by_admin events = %d", n)
	}
}

// Declining closes the wish as declined.
func TestDeclineWishCloses(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	got, err := r.svc.DeclineWish(r.ctx, res.Wish.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != model.WishDeclined {
		t.Errorf("state = %s", got.State)
	}
	if req, _ := r.fake.Request(got.SeerrRequestID); req.Status != seerrtest.RequestDeclined {
		t.Errorf("seerr status = %d", req.Status)
	}
	if !r.note.has(fmt.Sprintf("declined %d", res.Wish.ID)) {
		t.Errorf("decline not announced: %v", r.note.got())
	}
	if n := r.count(`SELECT count(*) FROM wish_events WHERE wish_id = ? AND kind = 'declined_by_admin'`, res.Wish.ID); n != 1 {
		t.Errorf("declined_by_admin events = %d", n)
	}
}

// A wish that is not pending is a conflict without a call; an unknown wish is not found; a request
// approved in Seerr meanwhile is a conflict too, and the wish takes its state.
func TestApproveWishNotPendingConflict(t *testing.T) {
	r := newRequestFixture(t)
	auto := r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: alien})
	res := r.create(r.marco, film(alien)) // attached to the approved request
	if w := r.wish(res.Wish.ID); w.State != model.WishProcessing || w.SeerrRequestID != auto.ID {
		t.Fatalf("wish = %+v", w.Wish)
	}
	r.fake.ResetCalls()
	if _, err := r.svc.ApproveWish(r.ctx, res.Wish.ID, 1); !errors.Is(err, model.ErrWishNotPending) {
		t.Errorf("processing: %v", err)
	}
	if _, err := r.svc.DeclineWish(r.ctx, 9999, 1); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
	if n := len(r.fake.CallLog()); n != 0 {
		t.Errorf("calls = %d", n)
	}
	pending := r.create(r.marco, film(matrix))
	r.fake.SetRequestStatus(r.wish(pending.Wish.ID).SeerrRequestID, seerrtest.RequestApproved)
	if _, err := r.svc.DeclineWish(r.ctx, pending.Wish.ID, 1); !errors.Is(err, model.ErrWishNotPending) {
		t.Errorf("approved in seerr: %v", err)
	}
	if w := r.wish(pending.Wish.ID); w.State != model.WishProcessing {
		t.Errorf("state after the conflict = %s", w.State)
	}
	if n := r.count(`SELECT count(*) FROM wish_events WHERE wish_id = ? AND kind LIKE '%_by_admin'`, pending.Wish.ID); n != 0 {
		t.Errorf("review events = %d", n)
	}
}

// Seerr gone: the error says so and nothing changes.
func TestApproveWishSeerrUnreachable(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	events := r.count(`SELECT count(*) FROM wish_events WHERE wish_id = ?`, res.Wish.ID)
	r.fake.SetFailure("POST /request/{id}/{status}", seerrtest.Failure{Status: 500, Message: "boom"})
	if _, err := r.svc.ApproveWish(r.ctx, res.Wish.ID, 1); !errors.Is(err, model.ErrSeerrFailed) {
		t.Errorf("500: %v", err)
	}
	r.fake.Close()
	_, err := r.svc.ApproveWish(r.ctx, res.Wish.ID, 1)
	if !errors.Is(err, model.ErrSeerrUnreachable) {
		t.Errorf("closed: %v", err)
	}
	if err != nil && strings.Contains(err.Error(), seerrtest.APIKey) {
		t.Error("the key is in the error")
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishPending {
		t.Errorf("state = %s", w.State)
	}
	if n := r.count(`SELECT count(*) FROM wish_events WHERE wish_id = ?`, res.Wish.ID); n != events {
		t.Errorf("events %d -> %d", events, n)
	}
	if r.countAnnounced(fmt.Sprintf("approved %d", res.Wish.ID)) != 0 {
		t.Error("approval announced")
	}
}

// A series whose pending seasons went out in two requests: both are approved, the wish becomes
// processing and the approval is announced once.
func TestApproveWishSeriesTwoRequests(t *testing.T) {
	r := newRequestFixture(t)
	s1 := r.directRequest(3, seerr.NewRequest{MediaType: seerr.MediaTypeTV, TMDBID: got, TVDBID: gotTVDB, Seasons: []int{1}})
	res := r.create(r.marco, show(got, 1, 2))
	w := r.wish(res.Wish.ID)
	if w.State != model.WishPending || len(w.Seasons) != 2 || w.Seasons[0].SeerrRequestID != s1.ID || w.Seasons[1].SeerrRequestID == s1.ID {
		t.Fatalf("wish = %+v", w.Wish)
	}
	s2 := w.Seasons[1].SeerrRequestID
	r.fake.ResetCalls()
	out, err := r.svc.ApproveWish(r.ctx, res.Wish.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if out.State != model.WishProcessing {
		t.Errorf("state = %s, seasons %+v", out.State, out.Seasons)
	}
	if n := r.fake.Calls("POST /request/{id}/{status}"); n != 2 {
		t.Errorf("decisions = %d", n)
	}
	for _, id := range []int{s1.ID, s2} {
		if req, _ := r.fake.Request(id); req.Status != seerrtest.RequestApproved {
			t.Errorf("request %d status = %d", id, req.Status)
		}
	}
	if n := r.countAnnounced(fmt.Sprintf("approved %d", res.Wish.ID)); n != 1 {
		t.Errorf("approved announced %d times", n)
	}
}

// Another instance waits for the administrator: no call.
func TestApproveWishInstanceChanged(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	r.exec(`INSERT INTO settings (key, value, updated_at) VALUES ('seerr.instance_pending', '{"adminEmail":"x@example.test"}', 1)`)
	r.fake.ResetCalls()
	if _, err := r.svc.ApproveWish(r.ctx, res.Wish.ID, 1); !errors.Is(err, model.ErrInstanceChanged) {
		t.Errorf("err = %v", err)
	}
	if n := len(r.fake.CallLog()); n != 0 {
		t.Errorf("calls = %d", n)
	}
}

// A series wish that is processing as a whole with one season still pending in Seerr: approving
// acts on that season's request only (review, open question).
func TestApproveWishPendingSeasonOfProcessingSeries(t *testing.T) {
	r := newRequestFixture(t)
	s1 := r.directRequest(3, seerr.NewRequest{MediaType: seerr.MediaTypeTV, TMDBID: got, TVDBID: gotTVDB, Seasons: []int{1}})
	res := r.create(r.marco, show(got, 1, 2))
	w := r.wish(res.Wish.ID)
	s2 := w.Seasons[1].SeerrRequestID
	r.fake.SetRequestStatus(s1.ID, seerrtest.RequestApproved)
	r.exec(`UPDATE wishes SET state = 'processing' WHERE id = ?`, res.Wish.ID)
	r.exec(`UPDATE wish_seasons SET state = 'processing' WHERE wish_id = ? AND season_number = 1`, res.Wish.ID)
	r.fake.ResetCalls()
	out, err := r.svc.ApproveWish(r.ctx, res.Wish.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if n := r.fake.Calls("POST /request/{id}/{status}"); n != 1 {
		t.Errorf("decisions = %d, want 1", n)
	}
	if req, _ := r.fake.Request(s2); req.Status != seerrtest.RequestApproved {
		t.Errorf("season 2 request status = %d", req.Status)
	}
	if out.State != model.WishProcessing || out.Seasons[1].State != model.WishProcessing {
		t.Errorf("after the approval = %s %+v", out.State, out.Seasons)
	}
	// Nothing pending any more: the conflict as before, no call.
	r.fake.ResetCalls()
	if _, err := r.svc.DeclineWish(r.ctx, res.Wish.ID, 1); !errors.Is(err, model.ErrWishNotPending) {
		t.Errorf("nothing pending: %v", err)
	}
	if n := len(r.fake.CallLog()); n != 0 {
		t.Errorf("calls = %d", n)
	}
}
