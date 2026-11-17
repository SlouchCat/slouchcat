package wishes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// The sync with Seerr (concept 25, architecture 7.1, 7.2, 8.1; build step 2.5). Seerr has no feed
// of changes and no filter by TMDB id, so the sync pages GET /request sorted by updatedAt
// descending and stops at the first request older than the pointer seerr.last_sync (minus a
// margin for the clocks). A pass covers «now» down to the pointer; it saves its page after every
// page (seerr.sync_pass), so an aborted run goes on where it stopped, and the pointer becomes the
// start of the pass once it is done: what changed during the pass is newer and comes in the next
// one. The very first sync runs as the initial sync without a page limit. Every write happens only
// when the request changed, so reading a request twice is harmless. Afterwards the sync checks
// open wishes one by one (GET /request/{id}, at most once a day per wish, which also finds deleted
// requests) and queues the due wishes of the outbox and the due withdrawals. An unknown request
// goes to the take-over (takeover.go, 7.3): a linked user's becomes a wish of its account, another
// user's a wish without a requester; nothing of the shared user. After the run the take-over of
// every newly set Seerr user is queued.

// JobSync is the job type of the sync.
const JobSync = "wishes.sync"

// Settings of the sync (data model 2.12).
const (
	settingLastSync = "seerr.last_sync" // Unix ms; empty: the initial sync is due
	settingSyncPass = "seerr.sync_pass" //nolint:gosec // G101: a settings key, not a credential
)

const (
	syncEvery      = 5 * time.Minute
	syncPriority   = 5
	syncPageSize   = 50
	syncMaxPages   = 40 // per regular run; the initial sync has no limit
	syncMargin     = 2 * time.Minute
	syncKeyInitial = JobSync + ":initial"
	checkEvery     = 24 * time.Hour
	checkMax       = 50
	outboxBatch    = 100
)

type syncPayload struct {
	Initial bool `json:"initial,omitempty"`
}

// syncPass is the state of a pass: from Top (when it started) down to Until; Page pages are done.
type syncPass struct {
	Top   int64 `json:"top"`
	Until int64 `json:"until"`
	Page  int   `json:"page"`
}

func (s *Service) runSync(ctx context.Context, _ *jobs.Queue, prio int) ([]int64, error) {
	if s.queue == nil {
		return nil, nil
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobSync, Priority: prio, IdempotencyKey: JobSync, Payload: []byte(`{}`)})
	if err != nil {
		return nil, fmt.Errorf("enqueue %s: %w", JobSync, err)
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, fmt.Errorf("raise %s: %w", JobSync, err)
	}
	return []int64{id}, nil
}

// SyncNow queues a sync with manual priority («Jetzt abgleichen», syncWishesNow; the Seerr
// webhook later); model.ErrSeerrNotConfigured without address or key.
func (s *Service) SyncNow(ctx context.Context) error {
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return err
	}
	if !cfg.Configured() {
		return model.ErrSeerrNotConfigured
	}
	_, err = s.runSync(ctx, nil, jobs.ManualPriority)
	return err
}

func (s *Service) handleSync(ctx context.Context, j *jobs.Job) error {
	var p syncPayload
	if len(j.Payload) > 0 {
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return jobs.Final(fmt.Errorf("%s: bad payload", JobSync))
		}
	}
	return s.sync(ctx, p.Initial)
}

// sync runs one regular or initial sync. Seerr switched off, not configured or another instance:
// nothing is read or sent, and the outbox rests (T-2). An unreachable Seerr or a refused key ends
// the run without an error; the state of the connection shows it.
func (s *Service) sync(ctx context.Context, initial bool) error {
	_, c, err := s.connection(ctx, false)
	if err == nil {
		if pending, perr := s.instancePending(ctx); perr != nil {
			return perr
		} else if pending != nil {
			err = model.ErrInstanceChanged
		}
	}
	if paused(err) {
		s.markRest(ctx, true)
		return nil
	}
	if err != nil {
		return err
	}
	admin := c.AsAdmin()
	if last, _ := s.state(ctx, settingLastError); last == "key_invalid" {
		if _, err := admin.Me(ctx); err != nil {
			s.recordOutcome(ctx, seerrErr(err))
			return nil
		}
	}
	s.markRest(ctx, false)
	seen := map[int]bool{}
	err = s.syncRequests(ctx, admin, initial, seen)
	if err == nil {
		err = s.checkWishes(ctx, admin)
	}
	if err == nil {
		err = s.syncQuotas(ctx, admin, seen)
	}
	if serr := seerrErr(err); serr != nil || err == nil {
		s.recordOutcome(ctx, serr)
	}
	var se *seerr.Error
	if errors.As(err, &se) {
		s.log.InfoContext(ctx, "wish sync ended early", "reason", errorClass(err))
		err = nil
	}
	if err != nil {
		return err
	}
	if err := s.queueDue(ctx); err != nil {
		return err
	}
	if err := s.queueImports(ctx); err != nil {
		s.log.WarnContext(ctx, "take-over not queued after the sync", "err", err)
	}
	s.tagsSoon(ctx) // the profile tags follow every sync (architecture 13)
	return nil
}

// syncRequests runs the pass in progress or a new one; seen collects the Seerr users of the
// requests the pass read.
func (s *Service) syncRequests(ctx context.Context, admin seerr.Session, initial bool, seen map[int]bool) error {
	lastRaw, err := s.state(ctx, settingLastSync)
	if err != nil {
		return err
	}
	last, _ := strconv.ParseInt(lastRaw, 10, 64)
	if lastRaw == "" && !initial {
		if s.queue != nil {
			if _, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobSync, Priority: syncPriority, IdempotencyKey: syncKeyInitial,
				Payload: []byte(`{"initial":true}`)}); err != nil {
				return fmt.Errorf("enqueue initial sync: %w", err)
			}
		}
		return nil
	}
	var pass syncPass
	ok, err := s.stateJSON(ctx, settingSyncPass, &pass)
	if err != nil {
		return err
	}
	if !ok {
		pass = syncPass{Top: s.now().UnixMilli(), Until: last}
	}
	limit := syncMaxPages
	if initial {
		limit = 0
	}
	stop := pass.Until - syncMargin.Milliseconds()
	// Seerr pages by skip, not by a cursor (review Minor 7): a request deleted in Seerr during a
	// pass shifts the rest by one, and the request at the edge of the page is skipped in this pass.
	// It is not lost: a request that changes later comes first in a following pass, and an open
	// wish of it is read one by one after 24 h without a change (checkWishes).
	for pages := 0; limit == 0 || pages < limit; pages++ {
		page, err := admin.Requests(ctx, seerr.RequestQuery{Take: syncPageSize, Skip: pass.Page * syncPageSize, Sort: "modified", SortDirection: "desc"})
		if err != nil {
			return err
		}
		done := len(page.Results) < syncPageSize
		var todo []seerr.Request
		for _, r := range page.Results {
			if pass.Until > 0 && r.UpdatedAt.UnixMilli() < stop {
				done = true
				break
			}
			todo = append(todo, r)
			if r.RequestedBy != nil {
				seen[r.RequestedBy.ID] = true
			}
		}
		pass.Page++
		if initial && page.PageInfo.Pages > 0 {
			jobs.Progress(ctx, int64(pass.Page), int64(page.PageInfo.Pages), "")
		}
		if err := s.applyPage(ctx, todo, pass, done); err != nil {
			return err
		}
		if done {
			s.log.InfoContext(ctx, "wish sync pass done", "pages", pass.Page, "initial", initial)
			return nil
		}
	}
	return nil
}

// syncQuotas reads the limit state anew for the users with open wishes, with requests in this pass
// (also those made directly in Seerr) and for those never read (architecture 19.3). Without it the
// state of a user who used up the limit in Seerr waited for the daily users job, and the overview
// showed no «Limit erreicht». An unreachable Seerr or a refused key ends the run like the other
// parts of the sync; the failure of one user is logged.
func (s *Service) syncQuotas(ctx context.Context, admin seerr.Session, seen map[int]bool) error {
	rows, err := s.read.ListQuotaSyncUsers(ctx)
	if err != nil {
		return fmt.Errorf("quota users: %w", err)
	}
	for _, r := range rows {
		id := int(r.SeerrID)
		if r.Due == 0 && !seen[id] {
			continue
		}
		err := s.refreshQuota(ctx, admin, id)
		switch {
		case err == nil:
		case errors.Is(err, seerr.ErrUnreachable), errors.Is(err, seerr.ErrKeyInvalid), ctx.Err() != nil, seerrErr(err) == nil:
			return err
		default:
			s.log.WarnContext(ctx, "seerr quota not read", "seerr_user_id", id, "reason", errorClass(err))
		}
	}
	return nil
}

// applyPage writes what one page changed and the state of the pass in one transaction; the last
// page moves the pointer and drops the pass.
func (s *Service) applyPage(ctx context.Context, reqs []seerr.Request, pass syncPass, done bool) error {
	cands, err := s.syncCandidates(ctx, reqs)
	if err != nil {
		return err
	}
	var changes []change
	importing := false
	err = s.inTx(ctx, func(q *wishq.Queries) error {
		changes = changes[:0]
		taken := 0
		importing = false
		for _, r := range reqs {
			ch, known, err := s.applyRequest(ctx, q, r)
			if err != nil {
				return err
			}
			if c, ok := cands[r.ID]; ok && !known {
				wrote, err := s.takeOver(ctx, q, c)
				if err != nil {
					return err
				}
				if wrote {
					taken++
					importing = importing || c.detail == model.WishDetailImporting
				}
			}
			changes = append(changes, ch...)
		}
		if taken > 0 {
			s.log.InfoContext(ctx, "seerr requests taken over by the sync", "count", taken)
		}
		if done {
			if err := s.setState(ctx, q, settingLastSync, strconv.FormatInt(pass.Top, 10)); err != nil {
				return err
			}
			return s.delState(ctx, q, settingSyncPass)
		}
		return s.setStateJSON(ctx, q, settingSyncPass, pass)
	})
	if err != nil {
		return err
	}
	s.afterChanges(ctx, changes)
	if importing {
		if err := s.queueReconcile(ctx, nil); err != nil {
			s.log.WarnContext(ctx, "reconcile not queued after a take-over", "err", err)
		}
	}
	return nil
}

// syncCandidates prepares the unknown requests of a page for the take-over, outside the write
// transaction (the title may come from TMDB): the shared user's and those without a user are left
// out.
func (s *Service) syncCandidates(ctx context.Context, reqs []seerr.Request) (map[int]takeOverCandidate, error) {
	out := map[int]takeOverCandidate{}
	for _, r := range reqs {
		known, err := s.requestKnown(ctx, s.read, r.ID)
		if err != nil {
			return nil, err
		}
		if known {
			continue
		}
		owner, skip, err := s.syncOwner(ctx, r)
		if err != nil {
			return nil, err
		}
		if !skip {
			out[r.ID] = s.candidateOf(ctx, r, owner)
		}
	}
	return out, nil
}

// change is a change of state of a wish that the sync announces after its transaction.
type change struct {
	wishID         int64
	from, to, fail string
	importing      bool
}

// applyRequest writes the state of a request into the wish or the seasons that carry it (known
// false when none does). Closed and withdrawn wishes and seasons are left as they are; available
// comes from the library only.
func (s *Service) applyRequest(ctx context.Context, q *wishq.Queries, r seerr.Request) ([]change, bool, error) {
	id := sql.NullInt64{Int64: int64(r.ID), Valid: true}
	ms := seerr.MediaUnknown
	if r.Media != nil {
		ms = r.Media.Status
	}
	w, err := q.GetWishBySeerrRequest(ctx, id)
	switch {
	case err == nil:
		ch, err := s.applyFilm(ctx, q, w, r, ms)
		return ch, true, err
	case !errors.Is(err, sql.ErrNoRows):
		return nil, false, fmt.Errorf("wish of request: %w", err)
	}
	seasons, err := q.ListWishSeasonsByRequest(ctx, id)
	if err != nil {
		return nil, false, fmt.Errorf("seasons of request: %w", err)
	}
	if len(seasons) == 0 {
		return nil, false, nil
	}
	var out []change
	for _, wishID := range distinctWishes(seasons) {
		ch, err := s.applySeasons(ctx, q, wishID, func(n int) (seasonUpdate, bool) {
			for _, se := range seasons {
				if se.WishID == wishID && int(se.SeasonNumber) == n {
					return seasonUpdateOf(r, n), true
				}
			}
			return seasonUpdate{}, false
		})
		if err != nil {
			return nil, true, err
		}
		out = append(out, ch...)
	}
	return out, true, nil
}

func distinctWishes(seasons []wishq.WishSeason) []int64 {
	var out []int64
	for _, se := range seasons {
		if len(out) == 0 || out[len(out)-1] != se.WishID {
			out = append(out, se.WishID)
		}
	}
	return out
}

// applyFilm moves an open film wish to the state of its request.
func (s *Service) applyFilm(ctx context.Context, q *wishq.Queries, w wishq.Wish, r seerr.Request, ms seerr.MediaStatus) ([]change, error) {
	if !isOpen(w.State) || w.State == model.WishQueued {
		return nil, nil
	}
	state, detail, fail := requestState(r.Status, ms)
	return s.setWish(ctx, q, w, state, detail, fail, updatedOf(r))
}

// setWish writes a state that differs, with its event; nothing when state and detail stay.
func (s *Service) setWish(ctx context.Context, q *wishq.Queries, w wishq.Wish, state, detail, fail string, updated sql.NullInt64) ([]change, error) {
	now := s.now().UnixMilli()
	if updated.Valid && updated != w.SeerrUpdatedAt {
		if _, err := q.SetWishSeerr(ctx, wishq.SetWishSeerrParams{SeerrUpdatedAt: updated, Now: now, ID: w.ID}); err != nil {
			return nil, fmt.Errorf("seerr updated: %w", err)
		}
	}
	if state == w.State && detail == w.StateDetail.String && fail == w.FailReason.String {
		return nil, nil
	}
	p := wishq.SetWishStateParams{State: state, StateDetail: nullString(detail), FailReason: nullString(fail), Now: now, ID: w.ID}
	if state == model.WishProcessing && !w.ApprovedAt.Valid {
		p.ApprovedAt = num64(now)
	}
	if !isOpen(state) {
		p.ClosedAt = num64(now)
	}
	if _, err := q.SetWishState(ctx, p); err != nil {
		return nil, fmt.Errorf("wish state: %w", err)
	}
	if !isOpen(state) {
		if err := s.closeSeasons(ctx, q, w.ID, state); err != nil {
			return nil, err
		}
	}
	if state != w.State {
		if err := s.event(ctx, q, w.ID, "state_changed", map[string]any{"from": w.State, "to": state, "reason": fail}); err != nil {
			return nil, err
		}
	}
	return []change{{wishID: w.ID, from: w.State, to: state, fail: fail, importing: detail == model.WishDetailImporting}}, nil
}

// seasonUpdate is the state a request gives one season.
type seasonUpdate struct {
	state, detail, fail string
}

func seasonUpdateOf(r seerr.Request, n int) seasonUpdate {
	ms := seerr.MediaUnknown
	if r.Media != nil {
		ms = seasonMedia(r.Media, n)
	}
	st, detail, fail := requestState(r.Status, ms)
	return seasonUpdate{st, detail, fail}
}

// applySeasons moves the open seasons of a series wish that seasonState knows and derives the
// state of the wish from all its seasons (architecture 8.1: the weakest open state).
func (s *Service) applySeasons(ctx context.Context, q *wishq.Queries, wishID int64, seasonState func(int) (seasonUpdate, bool)) ([]change, error) {
	w, err := q.GetWish(ctx, wishID)
	if err != nil {
		return nil, fmt.Errorf("wish: %w", err)
	}
	if !isOpen(w.State) || w.State == model.WishQueued {
		return nil, nil
	}
	seasons, err := q.ListWishSeasons(ctx, wishID)
	if err != nil {
		return nil, fmt.Errorf("seasons: %w", err)
	}
	var states []string
	fail, importing := "", false
	for _, se := range seasons {
		st := se.State
		if u, ok := seasonState(int(se.SeasonNumber)); ok {
			if _, open := stateRank[st]; open && u.state != st {
				if _, err := q.SetWishSeasonState(ctx, wishq.SetWishSeasonStateParams{State: u.state, WishID: wishID, SeasonNumber: se.SeasonNumber}); err != nil {
					return nil, fmt.Errorf("season state: %w", err)
				}
				st = u.state
			}
			if u.fail != "" {
				fail = u.fail
			}
			importing = importing || (st == model.WishProcessing && u.detail == model.WishDetailImporting)
		}
		states = append(states, st)
	}
	state := showState(states)
	detail := ""
	if state == model.WishProcessing && importing {
		detail = model.WishDetailImporting
	}
	if state != model.WishFailed {
		fail = ""
	} else if fail == "" {
		fail = model.WishFailArrFailed
	}
	return s.setWish(ctx, q, w, state, detail, fail, sql.NullInt64{})
}

// afterChanges announces the changes and queues the reconcile for titles Seerr reports available.
func (s *Service) afterChanges(ctx context.Context, changes []change) {
	reconcile, procured := false, false
	for _, c := range changes {
		s.announce(ctx, c.wishID, c.from, c.to, c.fail)
		reconcile = reconcile || c.importing
		procured = procured || (c.to == model.WishProcessing && c.from != model.WishProcessing)
	}
	if procured {
		s.tagsSoon(ctx)
	}
	if reconcile {
		if err := s.queueReconcile(ctx, nil); err != nil {
			s.log.WarnContext(ctx, "reconcile not queued after the sync", "err", err)
		}
	}
}

// checkWishes reads open wishes one by one that Seerr did not change for a day, at most once a day
// each (architecture 7.1): a request Seerr deleted answers 404, and the wish fails.
func (s *Service) checkWishes(ctx context.Context, admin seerr.Session) error {
	now := s.now()
	rows, err := s.read.ListWishesToCheck(ctx, wishq.ListWishesToCheckParams{Cutoff: now.Add(-checkEvery).UnixMilli(), LimitCount: checkMax})
	if err != nil {
		return fmt.Errorf("wishes to check: %w", err)
	}
	for _, w := range rows {
		if err := s.checkWish(ctx, admin, w, now); err != nil {
			return err
		}
	}
	return nil
}

// checkWish reads the requests of one wish and writes what changed. The wish is read again in the
// write transaction: a wish withdrawn or closed during the calls stays as it is (review Minor 2,
// a GET 404 after the withdrawal deleted the request would turn withdrawn into failed).
func (s *Service) checkWish(ctx context.Context, admin seerr.Session, w wishq.Wish, now time.Time) error {
	ids, _, err := s.knownRequests(ctx, w)
	if err != nil {
		return err
	}
	reqs := map[int]*seerr.Request{}
	for _, id := range ids {
		r, err := admin.Request(ctx, id)
		switch {
		case errors.Is(err, seerr.ErrNotFound):
			reqs[id] = nil
		case err != nil:
			return err
		default:
			reqs[id] = &r
		}
	}
	var changes []change
	err = s.inTx(ctx, func(q *wishq.Queries) error {
		changes = nil
		if err := q.SetWishChecked(ctx, wishq.SetWishCheckedParams{Now: num64(now.UnixMilli()), ID: w.ID}); err != nil {
			return fmt.Errorf("checked: %w", err)
		}
		cur, err := q.GetWish(ctx, w.ID)
		if err != nil {
			return fmt.Errorf("wish: %w", err)
		}
		if !isOpen(cur.State) || cur.State == model.WishQueued {
			return nil
		}
		w = cur
		if w.Media == model.WishMediaMovie {
			r, ok := reqs[int(w.SeerrRequestID.Int64)]
			if !ok {
				return nil
			}
			if r == nil {
				ch, err := s.setWish(ctx, q, w, model.WishFailed, "", model.WishFailRequestGone, sql.NullInt64{})
				changes = ch
				return err
			}
			ms := seerr.MediaUnknown
			if r.Media != nil {
				ms = r.Media.Status
			}
			ch, err := s.applyFilm(ctx, q, w, *r, ms)
			changes = ch
			return err
		}
		seasons, err := q.ListWishSeasons(ctx, w.ID)
		if err != nil {
			return fmt.Errorf("seasons: %w", err)
		}
		ch, err := s.applySeasons(ctx, q, w.ID, func(n int) (seasonUpdate, bool) {
			for _, se := range seasons {
				if int(se.SeasonNumber) != n || !se.SeerrRequestID.Valid {
					continue
				}
				r, ok := reqs[int(se.SeerrRequestID.Int64)]
				switch {
				case !ok:
					return seasonUpdate{}, false
				case r == nil:
					return seasonUpdate{state: model.WishFailed, fail: model.WishFailRequestGone}, true
				}
				return seasonUpdateOf(*r, n), true
			}
			return seasonUpdate{}, false
		})
		changes = ch
		return err
	})
	if err != nil {
		return err
	}
	s.afterChanges(ctx, changes)
	return nil
}

// queueDue queues the wishes of the outbox whose attempt is due and the due withdrawals.
func (s *Service) queueDue(ctx context.Context) error {
	now := num64(s.now().UnixMilli())
	due, err := s.read.ListDueWishes(ctx, wishq.ListDueWishesParams{Now: now, LimitCount: outboxBatch})
	if err != nil {
		return fmt.Errorf("due wishes: %w", err)
	}
	for _, w := range due {
		if err := s.queueWish(ctx, JobSubmit, w.ID, 0); err != nil {
			return err
		}
	}
	gone, err := s.read.ListDueWithdrawals(ctx, wishq.ListDueWithdrawalsParams{Now: now, LimitCount: outboxBatch})
	if err != nil {
		return fmt.Errorf("due withdrawals: %w", err)
	}
	for _, w := range gone {
		if err := s.queueWish(ctx, JobWithdraw, w.ID, 0); err != nil {
			return err
		}
	}
	return nil
}
