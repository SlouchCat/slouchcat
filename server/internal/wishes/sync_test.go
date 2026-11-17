package wishes

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
	"github.com/slouchcat/slouchcat/server/internal/store/storetest"
)

// Build step 2.5: the sync with Seerr and the reconcile with the library (architecture 7, 8, 11;
// DoD row 2.5 of the build plan).

// syncFixture is the request fixture whose clock runs an hour after the fake Seerr's, so every
// request of the fake is older than a pass that starts now.
func syncFixture(t *testing.T) *rfix {
	t.Helper()
	r := newRequestFixture(t)
	r.clock.t = r.fake.Now().Add(time.Hour)
	return r
}

// Item ids of the library titles (26 characters like every item id).
const (
	itemMatrix = "01J9WISH000000000000MATRIX"
	itemGot    = "01J9WISH0000000000000GOT01"
)

// addTitle puts a title into the library: a row in items (wishes.item_id refers to it) and the
// answer of the fake visibility.
func (r *rfix) addTitle(media string, tmdb int64, itemID string, seasons map[int]int) {
	r.t.Helper()
	kind := "movie"
	if media == "show" {
		kind = "series"
	}
	if err := storetest.AddBareTitle(r.ctx, r.db, itemID, kind); err != nil {
		r.t.Fatal(err)
	}
	r.lib.add(media, tmdb, itemID, seasons)
}

func (r *rfix) sync(initial bool) {
	r.t.Helper()
	if err := r.svc.handleSync(r.ctx, &jobs.Job{Type: JobSync, Payload: []byte(fmt.Sprintf(`{"initial":%t}`, initial))}); err != nil {
		r.t.Fatalf("sync: %v", err)
	}
}

func (r *rfix) setLastSync(t time.Time) {
	r.exec(`INSERT INTO settings (key, value, updated_at) VALUES ('seerr.last_sync', ?, 1)
	        ON CONFLICT (key) DO UPDATE SET value = excluded.value`, strconv.FormatInt(t.UnixMilli(), 10))
}

// manyRequests makes n approved requests of user 4 (it may approve itself) in the fake, each for
// its own film, stored directly: through POST /request the setup grows with the square of n.
func (r *rfix) manyRequests(n int) {
	r.t.Helper()
	for i := range n {
		id := 10000 + i
		r.fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: id, Name: fmt.Sprintf("Film %d", i)})
		r.fake.SeedRequest(seerrtest.Request{MediaType: "movie", TMDBID: id, UserID: 4, Status: seerrtest.RequestApproved, ModifiedBy: 4})
	}
}

func (r *rfix) skips() []string {
	var out []string
	for _, c := range r.fake.CallLog() {
		if c.Route == "GET /request" {
			out = append(out, c.Query.Get("skip"))
		}
	}
	return out
}

// The pass stops at the first request older than the pointer: one page although 60 older
// requests follow; the approval moves the wish to processing and is announced once; a second run
// writes no further event.
func TestSyncStopsAtPointerWithoutDuplicates(t *testing.T) {
	r := syncFixture(t)
	r.manyRequests(60)
	r.fake.Advance(time.Hour)
	res := r.create(r.marco, film(matrix))
	r.setLastSync(r.fake.Now().Add(-10 * time.Minute))
	r.fake.Advance(30 * time.Minute)
	r.fake.SetRequestStatus(r.wish(res.Wish.ID).SeerrRequestID, seerrtest.RequestApproved)
	r.clock.t = r.fake.Now().Add(time.Minute)
	r.fake.ResetCalls()
	r.sync(false)
	if got := r.skips(); len(got) != 1 {
		t.Errorf("pages read = %v", got)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishProcessing {
		t.Errorf("state = %s", w.State)
	}
	events := r.count(`SELECT count(*) FROM wish_events WHERE wish_id = ?`, res.Wish.ID)
	r.clock.advance(time.Minute)
	r.setLastSync(r.fake.Now().Add(-10 * time.Minute)) // the same request is read again
	r.sync(false)
	if n := r.count(`SELECT count(*) FROM wish_events WHERE wish_id = ?`, res.Wish.ID); n != events {
		t.Errorf("events %d -> %d", events, n)
	}
	approved := 0
	for _, c := range r.note.got() {
		if c == fmt.Sprintf("approved %d", res.Wish.ID) {
			approved++
		}
	}
	if approved != 1 {
		t.Errorf("approved announced %d times: %v", approved, r.note.got())
	}
}

// An aborted pass goes on at its saved page; when it is done the pointer is its start.
func TestSyncContinuesAtSavedPage(t *testing.T) {
	r := syncFixture(t)
	r.manyRequests(120)
	top := r.clock.now().Add(-time.Minute).UnixMilli()
	r.setLastSync(time.UnixMilli(1))
	r.exec(`INSERT INTO settings (key, value, updated_at) VALUES ('seerr.sync_pass', ?, 1)`, fmt.Sprintf(`{"top":%d,"until":1,"page":1}`, top))
	r.fake.ResetCalls()
	r.sync(false)
	if got := r.skips(); len(got) != 2 || got[0] != "50" || got[1] != "100" {
		t.Errorf("pages = %v", got)
	}
	if n := r.count(`SELECT count(*) FROM settings WHERE key = 'seerr.last_sync' AND value = ?`, strconv.FormatInt(top, 10)); n != 1 {
		t.Error("pointer is not the start of the pass")
	}
	if n := r.count(`SELECT count(*) FROM settings WHERE key = 'seerr.sync_pass'`); n != 0 {
		t.Error("pass not dropped")
	}
}

// 3000 requests: without a pointer the regular run only queues the initial sync; the initial
// sync reads every page without a limit and sets the pointer; afterwards a run reads one page.
func TestInitialSyncWithoutPageLimit(t *testing.T) {
	r := syncFixture(t)
	r.manyRequests(3000)
	r.fake.ResetCalls()
	r.sync(false)
	if len(r.skips()) != 0 || r.queue.count(JobSync) != 1 || r.queue.jobs[0].IdempotencyKey != syncKeyInitial {
		t.Fatalf("regular run without a pointer: pages %d, jobs %+v", len(r.skips()), r.queue.jobs)
	}
	r.queue.jobs = nil
	top := r.clock.now().UnixMilli()
	// One run reads every page. On a machine too slow for the client's time-out a page call can
	// fail; the run then ends early with that error, keeps its page and the next run goes on. A
	// page limit would end a run without such an error, so every early end must have one.
	for runs := 1; ; runs++ {
		r.sync(true)
		if r.count(`SELECT count(*) FROM settings WHERE key = ?`, settingSyncPass) == 0 {
			break
		}
		if n := strings.Count(r.logs.String(), "wish sync ended early"); n != runs {
			t.Fatalf("run %d ended before the last page without an error of Seerr (%d early ends)", runs, n)
		}
		if runs == 5 {
			t.Fatal("initial sync not done after 5 runs")
		}
	}
	pages := map[string]bool{}
	for _, skip := range r.skips() {
		pages[skip] = true
	}
	if n := len(pages); n < 60 || n > 61 {
		t.Errorf("initial pages = %d", n)
	}
	if n := r.count(`SELECT count(*) FROM settings WHERE key = 'seerr.last_sync' AND value = ?`, strconv.FormatInt(top, 10)); n != 1 {
		t.Error("initial sync did not set the pointer")
	}
	r.fake.ResetCalls()
	r.clock.advance(5 * time.Minute)
	r.sync(false)
	if n := len(r.skips()); n != 1 {
		t.Errorf("pages after the initial sync = %d", n)
	}
}

// A wish Seerr did not change for a day is read one by one at most once a day; a request Seerr
// deleted fails the wish (seerr_request_gone).
func TestSyncSingleCheckAndRequestGone(t *testing.T) {
	r := syncFixture(t)
	res := r.create(r.marco, film(matrix))
	id := r.wish(res.Wish.ID).SeerrRequestID
	r.exec(`UPDATE wishes SET updated_at = 0 WHERE id = ?`, res.Wish.ID)
	r.setLastSync(r.clock.now())
	r.fake.ResetCalls()
	r.sync(false)
	r.sync(false)
	if n := r.fake.Calls("GET /request/{id}"); n != 1 {
		t.Errorf("checks within a day = %d", n)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishPending {
		t.Errorf("state = %s", w.State)
	}
	c, err := seerr.New(r.fake.URL, seerrtest.APIKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AsAdmin().DeleteRequest(r.ctx, id); err != nil {
		t.Fatal(err)
	}
	r.clock.advance(25 * time.Hour)
	r.sync(false)
	if n := r.fake.Calls("GET /request/{id}"); n != 2 {
		t.Errorf("checks after a day = %d", n)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishFailed || w.FailReason != model.WishFailRequestGone {
		t.Errorf("gone: %+v", w.Wish)
	}
	if !r.note.has(fmt.Sprintf("failed %d seerr_request_gone", res.Wish.ID)) {
		t.Errorf("notifier = %v", r.note.got())
	}
}

// The sync queues the due wishes of the outbox again; Seerr switched off makes it rest.
func TestSyncQueuesTheOutbox(t *testing.T) {
	r := syncFixture(t)
	r.fake.SetFailure("POST /request", seerrtest.Failure{Status: 503})
	r.create(r.marco, film(matrix))
	r.queue.jobs = nil
	r.setLastSync(r.clock.now())
	r.clock.advance(2 * time.Minute)
	r.sync(false)
	if n := r.queue.count(JobSubmit); n != 1 {
		t.Errorf("submit jobs = %d", n)
	}
	r.cfg.set(func(c *fakeConfig) { c.seer.Enabled = false })
	r.queue.jobs = nil
	r.fake.ResetCalls()
	r.sync(false)
	if len(r.fake.CallLog()) != 0 || r.queue.count(JobSubmit) != 0 {
		t.Errorf("seerr off: calls %d, jobs %d", len(r.fake.CallLog()), r.queue.count(JobSubmit))
	}
	if n := r.count(`SELECT count(*) FROM settings WHERE key = 'wishes.outbox_rest' AND value LIKE '%since%'`); n != 1 {
		t.Error("rest not opened")
	}
}

// The match, not the scan, makes a film available: before the hook nothing is queued; the hook
// queues the reconcile of the title, which reports the wish available.
func TestReconcileAfterTheMatch(t *testing.T) {
	r := syncFixture(t)
	res := r.create(r.marco, film(matrix))
	r.queue.jobs = nil
	r.addTitle("movie", matrix, itemMatrix, nil) // scanned: in the library, ids not matched yet for the hook
	if r.queue.count(JobReconcile) != 0 {
		t.Fatal("reconcile queued before the match")
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishPending {
		t.Fatalf("state before the match = %s", w.State)
	}
	r.svc.TitleMatched(r.ctx, metadata.MatchedTitle{ItemID: "item-other", Kind: "movie", TMDBID: 1})
	if r.queue.count(JobReconcile) != 0 {
		t.Error("reconcile queued for a title without a wish")
	}
	r.svc.TitleMatched(r.ctx, metadata.MatchedTitle{ItemID: itemMatrix, Kind: "movie", TMDBID: matrix})
	if r.queue.count(JobReconcile) != 1 || r.queue.jobs[0].IdempotencyKey != JobReconcile+":item:"+itemMatrix {
		t.Fatalf("jobs = %+v", r.queue.jobs)
	}
	r.runJobs(JobReconcile)
	w := r.wish(res.Wish.ID)
	if w.State != model.WishAvailable || w.ItemID != itemMatrix || w.AvailableAt == nil {
		t.Errorf("after the match: %+v", w.Wish)
	}
	if !r.note.has(fmt.Sprintf("available %d %s", res.Wish.ID, itemMatrix)) {
		t.Errorf("notifier = %v", r.note.got())
	}
	if n := r.count(`SELECT count(*) FROM wish_requesters WHERE notified_at IS NOT NULL`); n != 0 {
		t.Error("wishes set notified_at itself")
	}
}

// A season with 1 of 10 episodes stays processing; a complete season makes the wish partial and
// is announced as a season; the last one makes it available.
func TestReconcileSeasonsByEpisodes(t *testing.T) {
	r := syncFixture(t)
	res := r.create(r.marco, show(got, 1, 2))
	r.fake.SetRequestStatus(r.wish(res.Wish.ID).Seasons[0].SeerrRequestID, seerrtest.RequestApproved)
	r.exec(`UPDATE wishes SET state = 'processing' WHERE id = ?`, res.Wish.ID)
	r.exec(`UPDATE wish_seasons SET state = 'processing' WHERE wish_id = ?`, res.Wish.ID)
	reconcile := func() {
		t.Helper()
		if err := r.svc.reconcile(r.ctx, reconcilePayload{}); err != nil {
			t.Fatal(err)
		}
	}
	r.addTitle("show", got, itemGot, map[int]int{1: 1})
	reconcile()
	if w := r.wish(res.Wish.ID); w.State != model.WishProcessing || w.Seasons[0].State != model.WishProcessing {
		t.Errorf("1 of 10: %+v", w.Wish)
	}
	r.addTitle("show", got, itemGot, map[int]int{1: 10, 2: 3})
	reconcile()
	if w := r.wish(res.Wish.ID); w.State != model.WishPartial || w.Seasons[0].State != model.WishAvailable || w.Seasons[1].State != model.WishProcessing {
		t.Errorf("season 1 complete: %+v", w.Wish)
	}
	if !r.note.has(fmt.Sprintf("season %d 1/10", res.Wish.ID)) {
		t.Errorf("notifier = %v", r.note.got())
	}
	r.addTitle("show", got, itemGot, map[int]int{1: 10, 2: 10})
	reconcile()
	if w := r.wish(res.Wish.ID); w.State != model.WishAvailable {
		t.Errorf("both seasons: %+v", w.Wish)
	}
	if !r.note.has(fmt.Sprintf("available %d %s", res.Wish.ID, itemGot)) {
		t.Errorf("notifier = %v", r.note.got())
	}
}

// A film in the library that no requester may see yet stays processing «wird eingelesen»; profiles
// that wait for an available wish are reported again by the hourly run.
func TestReconcileHiddenAndWaiting(t *testing.T) {
	r := syncFixture(t)
	res := r.create(r.marco, film(matrix))
	r.exec(`UPDATE wishes SET state = 'processing' WHERE id = ?`, res.Wish.ID)
	r.addTitle("movie", matrix, itemMatrix, nil)
	r.lib.hidden[itemMatrix] = true
	if err := r.svc.reconcile(r.ctx, reconcilePayload{}); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishProcessing || w.StateDetail != model.WishDetailImporting {
		t.Errorf("hidden: %+v", w.Wish)
	}
	r.lib.hidden[itemMatrix] = false
	r.note.waiting = []int64{1}
	for range 2 {
		if err := r.svc.reconcile(r.ctx, reconcilePayload{}); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	for _, c := range r.note.got() {
		if c == fmt.Sprintf("available %d %s", res.Wish.ID, itemMatrix) {
			n++
		}
	}
	if n != 2 { // made available, then reported again for the waiting profile
		t.Errorf("available announced %d times", n)
	}
}

// The sync reads the limit state anew (architecture 19.3): a user never read, and a user whose
// requests the pass saw, also those made directly in Seerr. Before, the state waited for the daily
// users job and the overview showed no «Limit erreicht» although the limit was used up in Seerr.
func TestSyncReadsQuotaOfSeenUsers(t *testing.T) {
	r := syncFixture(t)
	r.fake.UpdateUser(3, func(u *seerrtest.User) { u.MovieQuotaLimit, u.MovieQuotaDays = ip(2), ip(7) })
	r.exec(`INSERT INTO wish_account_limits (account_id, mode, movie_limit, movie_days, tv_limit, tv_days, set_at)
	        VALUES (1, 'custom', 2, 7, 0, 7, 1)`)
	reached := func() []model.WishQuotaHint {
		t.Helper()
		d, err := r.svc.DashboardWishes(r.ctx)
		if err != nil || d == nil {
			t.Fatalf("dashboard = %+v, %v", d, err)
		}
		return d.QuotaReached
	}
	r.setLastSync(r.fake.Now().Add(-10 * time.Minute))
	r.sync(false) // nothing in the pass; the state of user 3 was never read
	if q := r.mapping(1).Quota; q == nil || q.Movie == nil || q.Movie.Used != 0 {
		t.Fatalf("never read: quota = %+v", q)
	}
	first := r.directRequest(3, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: matrix})
	second := r.directRequest(3, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: alien})
	if got := reached(); len(got) != 0 {
		t.Fatalf("reached before the sync = %+v", got)
	}
	r.fake.ResetCalls()
	r.setLastSync(r.fake.Now().Add(-10 * time.Minute))
	r.sync(false)
	got := reached()
	if len(got) != 1 || got[0].AccountID != 1 || got[0].Movie == nil || got[0].Movie.Used != 2 || got[0].Movie.Remaining != 0 {
		t.Errorf("limit reached = %+v", got)
	}
	if n := r.fake.Calls("GET /user/{id}/quota"); n != 1 {
		t.Errorf("quota read %d times, want once (user 3 only)", n)
	}
	// The sync took the two requests over as wishes of the account (step 4.6), so the user has open
	// wishes and is read in every pass until they close: Seerr declines both, the next pass sees
	// that and closes them.
	r.fake.SetRequestStatus(first.ID, seerrtest.RequestDeclined)
	r.fake.SetRequestStatus(second.ID, seerrtest.RequestDeclined)
	r.clock.advance(time.Minute)
	r.setLastSync(r.fake.Now().Add(-10 * time.Minute))
	r.sync(false)
	// A pass without requests of the user and without open wishes reads nothing.
	r.fake.ResetCalls()
	r.clock.advance(time.Minute)
	r.setLastSync(r.clock.now())
	r.sync(false)
	if n := r.fake.Calls("GET /user/{id}/quota"); n != 0 {
		t.Errorf("quota read %d times without a reason", n)
	}
}
