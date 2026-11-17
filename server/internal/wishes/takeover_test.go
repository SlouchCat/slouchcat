package wishes

import (
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// Build step 4.6: the take-over of requests made in Seerr (F-21; architecture 7.3, users and
// profiles 8.2).

// runImports runs the queued take-overs and forgets them.
func (r *rfix) runImports() {
	r.t.Helper()
	r.queue.mu.Lock()
	var run []jobs.NewJob
	keep := r.queue.jobs[:0]
	for _, j := range r.queue.jobs {
		if j.Type == JobImport {
			run = append(run, j)
		} else {
			keep = append(keep, j)
		}
	}
	r.queue.jobs = keep
	r.queue.mu.Unlock()
	for _, j := range run {
		if err := r.svc.handleImport(r.ctx, &jobs.Job{Type: j.Type, Payload: j.Payload}); err != nil {
			r.t.Fatalf("import: %v", err)
		}
	}
}

// wishesOf lists the administration's wishes of a TMDB id.
func (r *rfix) wishesOf(tmdb int64) []model.AdminWish {
	r.t.Helper()
	all, _, _, err := r.svc.AdminWishes(r.ctx, model.WishFilterAll, false, 0, 0)
	if err != nil {
		r.t.Fatal(err)
	}
	var out []model.AdminWish
	for _, w := range all {
		if w.TMDBID == tmdb {
			out = append(out, w)
		}
	}
	return out
}

func owner(w model.AdminWish) int64 {
	for _, q := range w.Requesters {
		if q.Role == model.WishRoleOwner {
			return q.ProfileID
		}
	}
	return 0
}

// Old requests of a user that is assigned later: the regular sync does not see them (its pointer
// is past them); the take-over queued with the mapping gives them to the main profile of the
// account, as «In Seerr angelegt», without a message; a second run adds nothing.
func TestImportTakesOverOldRequestsOfNewlyAssignedUser(t *testing.T) {
	r := syncFixture(t)
	film := r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: alien})
	series := r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeTV, TMDBID: got, TVDBID: gotTVDB, Seasons: []int{1, 2}})
	r.setLastSync(r.clock.now())
	r.sync(false)
	if len(r.wishesOf(alien)) != 0 {
		t.Fatal("the regular sync saw an old request")
	}
	r.runImports() // account 1 (user 3), queued by the sync; nothing of user 4
	if len(r.wishesOf(alien)) != 0 {
		t.Fatal("an old request of user 4 taken over before the mapping")
	}
	if _, err := r.svc.SetAccountMapping(r.ctx, 3, ip(4), 1); err != nil {
		t.Fatal(err)
	}
	if n := r.queue.count(JobImport); n != 1 {
		t.Fatalf("imports queued = %d", n)
	}
	r.fake.ResetCalls()
	r.runImports()
	for _, c := range r.fake.CallLog() {
		if c.Route == "GET /request" && (c.Query.Get("requestedBy") != "4" || c.HasAPIUser) {
			t.Errorf("call = %+v", c)
		}
	}
	fw, sw := r.wishesOf(alien), r.wishesOf(got)
	if len(fw) != 1 || len(sw) != 1 {
		t.Fatalf("wishes = %+v / %+v", fw, sw)
	}
	if w := fw[0]; w.Origin != model.WishOriginSeerr || owner(w) != 4 || w.State != model.WishProcessing || w.SeerrRequestID != film.ID || w.SeerrUserID != 4 {
		t.Errorf("film wish = %+v", w)
	}
	if w := sw[0]; owner(w) != 4 || len(w.Seasons) != 2 || w.Seasons[0].SeerrRequestID != series.ID || w.State != model.WishProcessing {
		t.Errorf("series wish = %+v", w)
	}
	if got := r.note.got(); len(got) != 0 {
		t.Errorf("announced %v", got)
	}
	if n := r.count(`SELECT count(*) FROM wish_tags`); n != 0 {
		t.Errorf("tag rows = %d", n)
	}
	events := r.count(`SELECT count(*) FROM wish_events`)
	if n, err := r.svc.ImportSeerrRequests(r.ctx, ip(4)); err != nil || n != 1 {
		t.Fatalf("by hand: %d, %v", n, err)
	}
	r.runImports()
	if n := r.count(`SELECT count(*) FROM wishes`); n != 2 {
		t.Errorf("wishes after a second run = %d", n)
	}
	if n := r.count(`SELECT count(*) FROM wish_events`); n != events {
		t.Errorf("events %d -> %d", events, n)
	}
}

// A request of a user of no account is kept without a requester and without a message; when the
// user is assigned later, the same wish gets the main profile as owner.
func TestTakeOverUnassignableGetsTargetProfileLater(t *testing.T) {
	r := syncFixture(t)
	r.fake.AddUser(seerrtest.User{ID: 6, Email: "wohnzimmer@example.test", Username: "wohnzimmer-tv", Permissions: seerrtest.PermRequest})
	r.refresh()
	req := r.directRequest(6, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: matrix})
	r.sync(true)
	ws := r.wishesOf(matrix)
	if len(ws) != 1 || len(ws[0].Requesters) != 0 || ws[0].State != model.WishPending || ws[0].SeerrUserID != 6 || ws[0].SeerrRequestID != req.ID {
		t.Fatalf("unassignable = %+v", ws)
	}
	if got := r.note.got(); len(got) != 0 {
		t.Errorf("announced %v", got)
	}
	unmapped, _, _, err := r.svc.AdminWishes(r.ctx, model.WishFilterAll, true, 0, 0)
	if err != nil || len(unmapped) != 1 || unmapped[0].ID != ws[0].ID {
		t.Errorf("filter unmapped = %+v, %v", unmapped, err)
	}
	// A profile asking for the same film joins it (Seerr is not called).
	if _, err := r.svc.SetAccountMapping(r.ctx, 2, ip(6), 1); err != nil {
		t.Fatal(err)
	}
	r.runImports()
	ws = r.wishesOf(matrix)
	if len(ws) != 1 || owner(ws[0]) != 3 || ws[0].State != model.WishPending {
		t.Fatalf("after the mapping = %+v", ws)
	}
	if n := r.count(`SELECT count(*) FROM wish_events WHERE wish_id = ? AND kind = 'owner_changed'`, ws[0].ID); n != 1 {
		t.Errorf("owner events = %d", n)
	}
}

// The shared user's requests are not taken over, neither by the sync nor by hand, and count 0.
func TestTakeOverSkipsSharedUser(t *testing.T) {
	r := syncFixture(t)
	r.directRequest(5, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: alien})
	r.sync(true)
	if ws := r.wishesOf(alien); len(ws) != 0 {
		t.Errorf("wishes = %+v", ws)
	}
	if _, err := r.svc.ImportSeerrRequests(r.ctx, ip(5)); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("by hand: %v", err)
	}
	r.fake.ResetCalls()
	if n, err := r.svc.CountSeerrRequests(r.ctx, 5); err != nil || n != 0 {
		t.Errorf("count = %d, %v", n, err)
	}
	if n := len(r.fake.CallLog()); n != 0 {
		t.Errorf("calls = %d", n)
	}
}

// The message before a mapping counts with take=1 as user 1.
func TestImportCountForTheMessage(t *testing.T) {
	r := newRequestFixture(t)
	for _, id := range []int{matrix, alien} {
		r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: id})
	}
	r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeTV, TMDBID: got, TVDBID: gotTVDB, Seasons: []int{1}})
	r.directRequest(3, seerr.NewRequest{MediaType: seerr.MediaTypeTV, TMDBID: got, TVDBID: gotTVDB, Seasons: []int{2}})
	r.fake.ResetCalls()
	n, err := r.svc.CountSeerrRequests(r.ctx, 4)
	if err != nil || n != 3 {
		t.Fatalf("count = %d, %v", n, err)
	}
	log := r.fake.CallLog()
	if len(log) != 1 || log[0].Query.Get("take") != "1" || log[0].Query.Get("requestedBy") != "4" || log[0].HasAPIUser {
		t.Errorf("calls = %+v", log)
	}
}

// Declined requests and requests Seerr reports available for more than 30 days are not taken
// over; a film the library already has is marked as reported for its owner (no «verfügbar»).
func TestImportRulesClosedOldAndAvailable(t *testing.T) {
	r := syncFixture(t)
	declined := r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: alien})
	r.fake.SetRequestStatus(declined.ID, seerrtest.RequestDeclined)
	old := r.directRequest(4, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: matrix})
	r.fake.SetRequestStatus(old.ID, seerrtest.RequestCompleted)
	r.fake.SetMediaStatus("movie", matrix, seerrtest.MediaAvailable, nil)
	r.clock.t = r.fake.Now().Add(31 * 24 * time.Hour)
	if _, err := r.svc.SetAccountMapping(r.ctx, 3, ip(4), 1); err != nil {
		t.Fatal(err)
	}
	r.runImports()
	if n := r.count(`SELECT count(*) FROM wishes`); n != 0 {
		t.Fatalf("wishes = %d", n)
	}
	// The same film, available within the window and in the library.
	r.clock.t = r.fake.Now().Add(time.Hour)
	r.addTitle("movie", matrix, itemMatrix, nil)
	if _, err := r.svc.ImportSeerrRequests(r.ctx, ip(4)); err != nil {
		t.Fatal(err)
	}
	r.runImports()
	ws := r.wishesOf(matrix)
	if len(ws) != 1 || ws[0].StateDetail != model.WishDetailImporting || owner(ws[0]) != 4 {
		t.Fatalf("wishes = %+v", ws)
	}
	if n := r.count(`SELECT count(*) FROM wish_requesters WHERE wish_id = ? AND notified_at IS NOT NULL`, ws[0].ID); n != 1 {
		t.Errorf("owner not marked as reported")
	}
}

// The regular sync hands an unknown request of a linked user to the take-over: a request the
// account's user made in Seerr itself becomes a wish of the main profile.
func TestSyncTakeOverUnknownRequestOfLinkedUser(t *testing.T) {
	r := syncFixture(t)
	r.setLastSync(r.fake.Now().Add(-10 * time.Minute))
	req := r.directRequest(3, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: alien})
	r.sync(false)
	ws := r.wishesOf(alien)
	if len(ws) != 1 || owner(ws[0]) != 1 || ws[0].Origin != model.WishOriginSeerr || ws[0].SeerrRequestID != req.ID || ws[0].State != model.WishPending {
		t.Fatalf("wishes = %+v", ws)
	}
	if got := r.note.got(); len(got) != 0 {
		t.Errorf("announced %v", got)
	}
	// A wish of Slouchcat waiting in the outbox holds its film: the sync creates nothing for it.
	r.fake.SetFailure("POST /request", seerrtest.Failure{Status: 503, Message: "down"})
	res := r.create(r.marco, film(matrix))
	r.fake.SetFailure("POST /request", seerrtest.Failure{})
	if res.Wish.State != model.WishQueued {
		t.Fatalf("state = %s", res.Wish.State)
	}
	r.directRequest(3, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: matrix}) // the lost answer
	r.sync(false)
	if ws := r.wishesOf(matrix); len(ws) != 1 || ws[0].ID != res.Wish.ID {
		t.Errorf("matrix wishes = %+v", ws)
	}
}

// After each sync every ready mapping whose user was set since its last take-over gets one
// (the links of the provisioning); once done, the next sync queues none.
func TestSyncQueuesTakeOverOfNewMappings(t *testing.T) {
	r := syncFixture(t)
	r.setLastSync(r.clock.now())
	r.sync(false)
	if n := r.queue.count(JobImport); n != 1 { // account 1, user 3, mapped by the fixture
		t.Fatalf("imports queued = %d", n)
	}
	r.runImports()
	r.sync(false)
	if n := r.queue.count(JobImport); n != 0 {
		t.Errorf("imports queued after the take-over = %d", n)
	}
	r.clock.advance(time.Minute)
	if _, err := r.svc.SetAccountMapping(r.ctx, 1, ip(4), 1); err != nil { // another user
		t.Fatal(err)
	}
	r.queue.mu.Lock()
	r.queue.jobs = nil
	r.queue.mu.Unlock()
	r.sync(false)
	if n := r.queue.count(JobImport); n != 1 {
		t.Errorf("imports queued after a new user = %d", n)
	}
}
