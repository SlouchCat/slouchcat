package wishes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// The outbox (concept 25, architecture 6.3 to 6.5, 14; build step 2.4): a wish waits as queued
// until Seerr took it. The first attempt runs right after createWish without pre-check (Seerr
// reports a duplicate itself with 409 or 202); from the second attempt on GET /movie|tv first
// looks for a request the lost answer of an earlier POST created, so a request is never created
// twice. The ladder of the attempts lives in wishes.next_attempt_at (1, 2, 5, 10, 15, 30 min, then
// hourly); after 24 h in the outbox the wish fails. The 24 h rest while Seerr is switched off,
// not configured or another instance waits (T-2).

// Job types of the outbox.
const (
	JobSubmit   = "wishes.submit"
	JobWithdraw = "wishes.withdraw"
)

// settingRest keeps the times the outbox rested (T-2): JSON restState.
const settingRest = "wishes.outbox_rest"

// restKeep is how long a finished rest span is kept: longer than any wish can wait.
const restKeep = 30 * 24 * time.Hour

var ladder = []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute, 30 * time.Minute}

// ladderDelay is the wait after the attempts-th failed attempt.
func ladderDelay(attempts int64) time.Duration {
	if attempts < 1 {
		return ladder[0]
	}
	if int(attempts) <= len(ladder) {
		return ladder[attempts-1]
	}
	return time.Hour
}

type wishPayload struct {
	WishID int64 `json:"wishId"`
}

// wishLocks holds one submission or withdrawal per wish (architecture 6.6, «Sperre je Wunsch»).
type wishLocks struct {
	mu   sync.Mutex
	held map[int64]bool
}

func (l *wishLocks) try(id int64) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held[id] {
		return nil, false
	}
	if l.held == nil {
		l.held = map[int64]bool{}
	}
	l.held[id] = true
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		delete(l.held, id)
	}, true
}

// queueWish queues a job of the wish (wishes.submit or wishes.withdraw) after delay; the key
// returns the active job for a second call.
func (s *Service) queueWish(ctx context.Context, typ string, wishID int64, delay time.Duration) error {
	if s.queue == nil {
		return nil
	}
	payload, err := json.Marshal(wishPayload{WishID: wishID})
	if err != nil {
		return err
	}
	_, err = s.queue.Enqueue(ctx, jobs.NewJob{Type: typ, Priority: submitPrio, IdempotencyKey: fmt.Sprintf("%s:%d", typ, wishID),
		Payload: payload, Delay: max(0, delay)})
	if err != nil {
		return fmt.Errorf("enqueue %s: %w", typ, err)
	}
	return nil
}

// attemptNow is the immediate attempt after createWish: at most 3 s, then the outbox.
func (s *Service) attemptNow(ctx context.Context, wishID int64) {
	ctx = context.WithoutCancel(ctx)
	at, err := s.submit(ctx, wishID, immediateWait)
	if err != nil {
		s.log.WarnContext(ctx, "immediate attempt of a wish failed", "wish_id", wishID, "err", err)
		at = s.now().Add(ladder[0])
	}
	if !at.IsZero() {
		if err := s.queueWish(ctx, JobSubmit, wishID, at.Sub(s.now())); err != nil {
			s.log.WarnContext(ctx, "wish not queued for the outbox", "wish_id", wishID, "err", err)
		}
	}
}

func (s *Service) handleSubmit(ctx context.Context, j *jobs.Job) error {
	var p wishPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil || p.WishID < 1 {
		return jobs.Final(fmt.Errorf("%s: bad payload", JobSubmit))
	}
	at, err := s.submit(ctx, p.WishID, 0)
	if err != nil {
		return err
	}
	if !at.IsZero() {
		return jobs.RetryLater(max(time.Second, at.Sub(s.now())), "seerr not reached")
	}
	return nil
}

// paused reports the errors of a session that make the outbox rest (T-2, architecture 7.4).
func paused(err error) bool {
	return errors.Is(err, errDisabled) || errors.Is(err, model.ErrSeerrNotConfigured) || errors.Is(err, model.ErrInstanceChanged)
}

// sent is what Seerr said to a submission.
type sent struct {
	fail       string // a fail reason: the wish fails
	retry      bool   // Seerr not reached, the key refused, a lookup failed: the next attempt
	keyInvalid error  // the refused key, for the state of the connection
	how        string // submitted or attached
	film       *seerr.Request
	mediaID    int
	mediaState seerr.MediaStatus
	seasons    map[int]seasonHit // series: the request per wished season
}

// seasonHit is the request that carries a season, or none (found false: the season is in Seerr
// already, «wird eingelesen»).
type seasonHit struct {
	found bool
	req   seerr.Request
	media seerr.MediaStatus
}

// submit runs one attempt of the outbox and returns when the next one is due (zero: none, the
// wish left the outbox or rests). wait > 0 limits the calls to Seerr (the immediate attempt).
func (s *Service) submit(ctx context.Context, wishID int64, wait time.Duration) (time.Time, error) {
	release, ok := s.locks.try(wishID)
	if !ok {
		return s.now().Add(busyRetry), nil
	}
	defer release()
	w, err := s.read.GetWish(ctx, wishID)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("wish: %w", err)
	}
	if w.State != model.WishQueued {
		return time.Time{}, nil
	}
	if !w.SeerrUserID.Valid {
		return time.Time{}, s.failWish(ctx, w.ID, model.WishFailSeerrUserGone, nil)
	}
	if failed, err := s.orphanSender(ctx, &w); err != nil || failed {
		return time.Time{}, err
	}
	if why, err := s.senderStale(ctx, w); err != nil {
		return time.Time{}, err
	} else if why != "" {
		s.log.WarnContext(ctx, "queued wish not sent: its seerr user may no longer send it", "wish_id", w.ID, "seerr_user_id", w.SeerrUserID.Int64, "detail", why)
		return time.Time{}, s.failWish(ctx, w.ID, model.WishFailSeerrUserGone, map[string]any{"detail": why})
	}
	if w.Media == model.WishMediaShow && !w.TvdbID.Valid {
		return time.Time{}, s.failWish(ctx, w.ID, model.WishFailTVDBIDMissing, nil)
	}
	sess, err := s.sessionFor(ctx, w)
	if paused(err) {
		s.markRest(ctx, true)
		return time.Time{}, nil // wishes.sync queues it again when Seerr is back
	}
	if err != nil {
		return time.Time{}, err
	}
	s.markRest(ctx, false)
	var seasons []wishq.WishSeason
	if w.Media == model.WishMediaShow {
		if seasons, err = s.read.ListWishSeasons(ctx, w.ID); err != nil {
			return time.Time{}, fmt.Errorf("seasons: %w", err)
		}
	}
	cctx := ctx
	if wait > 0 {
		var cancel context.CancelFunc
		cctx, cancel = context.WithTimeout(ctx, wait)
		defer cancel()
	}
	res := s.send(cctx, sess, w, seasons)
	switch {
	case res.retry:
		return s.attemptFailed(ctx, w, res)
	case res.fail != "":
		if res.fail == model.WishFailQuota {
			s.refreshQuotaOf(ctx, int(w.SeerrUserID.Int64))
		}
		return time.Time{}, s.failWish(ctx, w.ID, res.fail, map[string]any{"attempt": w.Attempts + 1})
	}
	if err := s.accepted(ctx, w, res); err != nil {
		// Seerr took the request but it is not stored: the attempt counts, so the next one
		// pre-checks with GET /movie|tv and attaches instead of sending a second POST. Only when
		// even this write fails does the next attempt POST again; Seerr answers 409 then, and the
		// wish attaches to the same request.
		if _, aerr := s.write.SetWishAttempt(context.WithoutCancel(ctx), wishq.SetWishAttemptParams{
			NextAttemptAt: num64(s.now().Add(ladder[0]).UnixMilli()), Now: s.now().UnixMilli(), ID: w.ID,
		}); aerr != nil {
			s.log.WarnContext(ctx, "attempt of a sent wish not counted", "wish_id", w.ID, "err", aerr)
		}
		return time.Time{}, err
	}
	return time.Time{}, nil
}

// senderStale says why the frozen Seerr user of a queued wish may no longer send it, "" while it
// may (security review L-1). The rule of the papers (users and profiles 8.1, architecture 14): the
// wish stays with the user frozen at its creation, also after the account got another user or was
// unlinked, and Slouchcat never re-points it; it fails with seerr_user_gone when that user is gone
// from Seerr ("user_gone"). Beyond the papers it also fails ("mapping_changed") when the user now
// belongs to another account, or, no longer the user of any account and not the shared user, holds
// a managing right: sending would act as another account or as an administrator of Seerr.
func (s *Service) senderStale(ctx context.Context, w wishq.Wish) (string, error) {
	u, err := s.read.GetSeerrUser(ctx, w.SeerrUserID.Int64)
	if errors.Is(err, sql.ErrNoRows) {
		return "user_gone", nil
	}
	if err != nil {
		return "", fmt.Errorf("seerr user: %w", err)
	}
	if u.GoneAt.Valid {
		return "user_gone", nil
	}
	reqs, err := s.read.ListWishRequesters(ctx, w.ID)
	if err != nil {
		return "", fmt.Errorf("requesters: %w", err)
	}
	if len(reqs) == 0 || reqs[0].Role != model.WishRoleOwner {
		return "", nil
	}
	m, err := s.read.GetMappingBySeerrUser(ctx, num64(u.SeerrID))
	switch {
	case err == nil && m.AccountID != reqs[0].AccountID:
		return "mapping_changed", nil
	case err == nil:
		return "", nil
	case !errors.Is(err, sql.ErrNoRows):
		return "", fmt.Errorf("mapping of seerr user: %w", err)
	}
	if u.IsShared == 0 && privileged(u.SeerrID, seerr.Permission(uint64(max(0, u.Permissions)))) {
		return "mapping_changed", nil
	}
	return "", nil
}

// orphanSender re-resolves the sender of a queued wish that went to a co-wisher because its
// account was deleted (event owner_changed, reason account_deleted) while its frozen Seerr user
// belongs to no account any more: repoint gave up because a submission held the wish. The wish
// goes out as the current sender of its new owner, or fails with seerr_user_gone when the owner
// may not request (review Minor 1). A user only unlinked keeps sending (architecture 8.1). It
// reports whether the wish failed. The caller holds the wish's lock.
func (s *Service) orphanSender(ctx context.Context, w *wishq.Wish) (bool, error) {
	u, err := s.read.GetSeerrUser(ctx, w.SeerrUserID.Int64)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil // senderStale names it
	}
	if err != nil {
		return false, fmt.Errorf("seerr user: %w", err)
	}
	if u.IsShared == 1 {
		return false, nil
	}
	if _, err := s.read.GetMappingBySeerrUser(ctx, num64(u.SeerrID)); err == nil {
		return false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("mapping of seerr user: %w", err)
	}
	if handed, err := s.handedOverByDeletion(ctx, w.ID); err != nil || !handed {
		return false, err
	}
	reqs, err := s.read.ListWishRequesters(ctx, w.ID)
	if err != nil {
		return false, fmt.Errorf("requesters: %w", err)
	}
	if len(reqs) == 0 || reqs[0].Role != model.WishRoleOwner {
		return false, nil
	}
	rights, sender, err := s.SenderFor(ctx, reqs[0].AccountID, reqs[0].ProfileID)
	switch {
	case err != nil:
		return false, err
	case !rights.CanRequest && rights.Reason == model.WishReasonDisabled:
		return false, nil // the outbox rests
	case !rights.CanRequest:
		return true, s.failWish(ctx, w.ID, model.WishFailSeerrUserGone, map[string]any{"detail": "mapping_changed"})
	case int64(sender.SeerrUserID) == w.SeerrUserID.Int64:
		return false, nil
	}
	if _, err := s.write.SetWishSeerrUser(ctx, wishq.SetWishSeerrUserParams{SeerrUserID: num64(int64(sender.SeerrUserID)), Now: s.now().UnixMilli(), ID: w.ID}); err != nil {
		return false, fmt.Errorf("sender: %w", err)
	}
	s.log.InfoContext(ctx, "queued wish goes out as the current sender of its owner", "wish_id", w.ID, "from", w.SeerrUserID.Int64, "to", sender.SeerrUserID)
	w.SeerrUserID = num64(int64(sender.SeerrUserID))
	return false, nil
}

// handedOverByDeletion reports whether the newest change of owner of the wish came from the
// deletion of an account.
func (s *Service) handedOverByDeletion(ctx context.Context, wishID int64) (bool, error) {
	detail, err := s.read.LastWishEventDetail(ctx, wishq.LastWishEventDetailParams{WishID: wishID, Kind: "owner_changed"})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("events: %w", err)
	}
	var d struct {
		Reason string `json:"reason"`
	}
	return json.Unmarshal([]byte(detail.String), &d) == nil && d.Reason == eventAccountDeleted, nil
}

// send asks Seerr: the pre-check from the second attempt on, then POST /request.
func (s *Service) send(ctx context.Context, sess seerr.Session, w wishq.Wish, seasons []wishq.WishSeason) sent {
	res := sent{how: "submitted", seasons: map[int]seasonHit{}}
	var wanted []int
	for _, se := range seasons {
		if se.State == model.WishQueued && !se.SeerrRequestID.Valid {
			wanted = append(wanted, int(se.SeasonNumber))
		}
	}
	movie := w.Media == model.WishMediaMovie
	if !movie && len(wanted) == 0 {
		// A queued series wish without a queued season that carries no request: never sent as an
		// empty season set and never «attached» without a hit (review M-2). The outbox tries again
		// and gives up after its 24 h.
		s.log.ErrorContext(ctx, "queued series wish without a season to send", "wish_id", w.ID)
		return sent{retry: true}
	}
	if w.Attempts > 0 {
		var mi *seerr.Media
		if movie {
			d, err := sess.Movie(ctx, int(w.TmdbID))
			if err != nil {
				return s.refused(ctx, sess, err)
			}
			mi = d.MediaInfo
		} else {
			d, err := sess.TV(ctx, int(w.TmdbID))
			if err != nil {
				return s.refused(ctx, sess, err)
			}
			mi = d.MediaInfo
		}
		if mi != nil {
			if mi.Status == seerr.MediaBlocklisted {
				res.fail = model.WishFailBlocklisted
				return res
			}
			res.mediaID, res.mediaState = mi.ID, mi.Status
			if movie {
				if r := blockingRequest(mi.Requests); r != nil {
					res.film, res.how = r, "attached"
					return res
				}
			} else {
				wanted = res.hitSeasons(mi, wanted)
				if len(wanted) == 0 {
					res.how = "attached"
					return res
				}
			}
		}
	}
	req := seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: int(w.TmdbID)}
	if !movie {
		req = seerr.NewRequest{MediaType: seerr.MediaTypeTV, TMDBID: int(w.TmdbID), TVDBID: int(w.TvdbID.Int64), Seasons: wanted}
	}
	r, err := sess.CreateRequest(ctx, req)
	switch {
	case err == nil:
		if r.Media != nil {
			res.mediaID, res.mediaState = r.Media.ID, r.Media.Status
		}
		if movie {
			res.film = &r
			return res
		}
		var missing []int
		for _, n := range wanted {
			if slices.ContainsFunc(r.Seasons, func(sr seerr.SeasonRequest) bool { return sr.SeasonNumber == n }) {
				res.seasons[n] = seasonHit{found: true, req: r, media: res.mediaState}
			} else {
				missing = append(missing, n) // filtered out by Seerr: requested by someone else
			}
		}
		if len(missing) > 0 {
			return s.lookup(ctx, w, missing, res)
		}
		return res
	case errors.Is(err, seerr.ErrDuplicate), errors.Is(err, seerr.ErrNoSeasons):
		res.how = "attached"
		return s.lookup(ctx, w, wanted, res)
	}
	return s.refused(ctx, sess, err)
}

// lookup finds the requests of a title through GET /movie|tv without X-API-User (architecture
// 6.4: 409, 202 and seasons Seerr filtered out). A season without an open request is in Seerr
// already.
func (s *Service) lookup(ctx context.Context, w wishq.Wish, seasons []int, res sent) sent {
	admin, err := s.AdminSession(ctx)
	if err != nil {
		return sent{retry: true}
	}
	var mi *seerr.Media
	if w.Media == model.WishMediaMovie {
		d, err := admin.Movie(ctx, int(w.TmdbID))
		if err != nil {
			return sent{retry: true}
		}
		mi = d.MediaInfo
	} else {
		d, err := admin.TV(ctx, int(w.TmdbID))
		if err != nil {
			return sent{retry: true}
		}
		mi = d.MediaInfo
	}
	if mi == nil {
		return sent{retry: true}
	}
	res.mediaID, res.mediaState = mi.ID, mi.Status
	if w.Media == model.WishMediaMovie {
		r := blockingRequest(mi.Requests)
		if r == nil {
			return sent{retry: true}
		}
		res.film = r
		return res
	}
	for _, n := range res.hitSeasons(mi, seasons) {
		res.seasons[n] = seasonHit{media: seasonMedia(mi, n)}
	}
	return res
}

// hitSeasons records the requests of mi that carry a wanted season and lock it (blocks) and
// returns the rest.
func (res *sent) hitSeasons(mi *seerr.Media, wanted []int) []int {
	var rest []int
	for _, n := range wanted {
		found := false
		for i := range mi.Requests {
			r := mi.Requests[i]
			if !blocks(r) || !slices.ContainsFunc(r.Seasons, func(sr seerr.SeasonRequest) bool { return sr.SeasonNumber == n }) {
				continue
			}
			res.seasons[n] = seasonHit{found: true, req: r, media: seasonMedia(mi, n)}
			found = true
			break
		}
		if !found {
			rest = append(rest, n)
		}
	}
	return rest
}

func seasonMedia(mi *seerr.Media, n int) seerr.MediaStatus {
	for _, se := range mi.Seasons {
		if se.SeasonNumber == n {
			return se.Status
		}
	}
	return seerr.MediaUnknown
}

func isOpenRequest(r seerr.Request) bool {
	return !r.Is4K && (r.Status == seerr.RequestPending || r.Status == seerr.RequestApproved)
}

// blocks reports whether a request locks its title in Seerr, so a new POST gets 409: pending,
// approved, and FAILED too (only DECLINED and COMPLETED free it, architecture 8.1 «Ausnahme
// arr_failed»). A wish attaches to it; a FAILED one makes the wish failed with arr_failed at once
// instead of 24 h of retries (review M-4).
func blocks(r seerr.Request) bool {
	return isOpenRequest(r) || (!r.Is4K && r.Status == seerr.RequestFailed)
}

func blockingRequest(reqs []seerr.Request) *seerr.Request {
	for i := range reqs {
		if blocks(reqs[i]) {
			return &reqs[i]
		}
	}
	return nil
}

// refused maps a refusal of Seerr (architecture 6.4, N-114).
func (s *Service) refused(ctx context.Context, sess seerr.Session, err error) sent {
	switch {
	case errors.Is(err, seerr.ErrQuota):
		return sent{fail: model.WishFailQuota}
	case errors.Is(err, seerr.ErrBlocklisted):
		return sent{fail: model.WishFailBlocklisted}
	case errors.Is(err, seerr.ErrNoPermission):
		return sent{fail: model.WishFailNoPermission}
	case errors.Is(err, seerr.ErrKeyInvalid):
		return sent{retry: true, keyInvalid: err}
	case errors.Is(err, seerr.ErrAccessDenied):
		why := sess.ExplainRefusal(ctx, err)
		switch {
		case errors.Is(why, seerr.ErrUserGone):
			return sent{fail: model.WishFailSeerrUserGone}
		case errors.Is(why, seerr.ErrKeyInvalid):
			return sent{retry: true, keyInvalid: why}
		case errors.Is(why, seerr.ErrAccessDenied):
			return sent{fail: model.WishFailNoPermission} // the user exists and lacks the right
		}
	}
	return sent{retry: true}
}

// attemptFailed counts the attempt, sets the next one and fails the wish after 24 h in the outbox
// (rest excluded).
func (s *Service) attemptFailed(ctx context.Context, w wishq.Wish, res sent) (time.Time, error) {
	if res.keyInvalid != nil {
		s.recordOutcome(ctx, res.keyInvalid)
	}
	now := s.now()
	since := w.CreatedAt
	if at, err := s.read.LastWishEventAt(ctx, wishq.LastWishEventAtParams{WishID: w.ID, Kind: eventRetried}); err != nil {
		return time.Time{}, fmt.Errorf("retried at: %w", err)
	} else if at > since {
		since = at // a retry of the administration starts the 24 h anew
	}
	age := now.UnixMilli() - since - s.rested(ctx, since, now.UnixMilli())
	if time.Duration(age)*time.Millisecond >= submitDeadln {
		s.log.ErrorContext(ctx, "wish not submitted to seerr within 24 hours", "wish_id", w.ID, "attempts", w.Attempts+1)
		return time.Time{}, s.failWish(ctx, w.ID, model.WishFailUnreachable24h, map[string]any{"attempts": w.Attempts + 1})
	}
	next := now.Add(ladderDelay(w.Attempts + 1))
	if _, err := s.write.SetWishAttempt(ctx, wishq.SetWishAttemptParams{NextAttemptAt: num64(next.UnixMilli()), Now: now.UnixMilli(), ID: w.ID}); err != nil {
		return time.Time{}, fmt.Errorf("attempt: %w", err)
	}
	return next, nil
}

// requestState maps a request and its media to the state of a wish (architecture 8.1, I-46:
// status 5 «completed» counts as approved).
func requestState(rs seerr.MediaRequestStatus, ms seerr.MediaStatus) (state, detail, fail string) {
	switch {
	case ms == seerr.MediaBlocklisted:
		return model.WishFailed, "", model.WishFailBlocklisted
	case rs == seerr.RequestPending:
		return model.WishPending, "", ""
	case rs == seerr.RequestDeclined:
		return model.WishDeclined, "", ""
	case rs == seerr.RequestFailed:
		return model.WishFailed, "", model.WishFailArrFailed
	case rs == seerr.RequestCompleted && ms == seerr.MediaDeleted:
		return model.WishFailed, "", model.WishFailMediaDeleted
	case rs == seerr.RequestCompleted:
		return model.WishProcessing, model.WishDetailImporting, ""
	case ms == seerr.MediaAvailable || ms == seerr.MediaPartiallyAvailable:
		return model.WishProcessing, model.WishDetailImporting, ""
	}
	return model.WishProcessing, "", ""
}

// stateRank orders the open states, the weakest first (architecture 8.1).
var stateRank = map[string]int{model.WishQueued: 0, model.WishPending: 1, model.WishProcessing: 2}

// showState derives the state of a series wish from its seasons: the weakest open state; queued
// while a season waits in the outbox; partial once a season is available while others are open;
// available when every season is; otherwise failed before declined before withdrawn.
func showState(states []string) string {
	open, avail := "", 0
	has := map[string]bool{}
	for _, st := range states {
		has[st] = true
		if r, ok := stateRank[st]; ok && (open == "" || r < stateRank[open]) {
			open = st
		}
		if st == model.WishAvailable {
			avail++
		}
	}
	switch {
	case open == model.WishQueued:
		// A season still waits in the outbox: the wish stays queued, so wishes.submit sends it,
		// also while another season is available already (review M-1).
		return open
	case open != "" && avail > 0:
		return model.WishPartial
	case open != "":
		return open
	case avail > 0 && avail == len(states):
		return model.WishAvailable
	case has[model.WishFailed]:
		return model.WishFailed
	case has[model.WishDeclined]:
		return model.WishDeclined
	case avail > 0:
		return model.WishAvailable
	}
	return model.WishWithdrawn
}

func updatedOf(r seerr.Request) sql.NullInt64 {
	if r.UpdatedAt.IsZero() {
		return sql.NullInt64{}
	}
	return num64(r.UpdatedAt.UnixMilli())
}

// accepted stores what Seerr took. A wish withdrawn meanwhile keeps its state; its request ids
// are stored and the withdrawal is queued (architecture 6.3 step 5). A wish that left the outbox
// otherwise meanwhile (closed with its deleted account, failed, available) is not written: the
// event keeps the request ids (review Minor 9).
func (s *Service) accepted(ctx context.Context, w wishq.Wish, res sent) error {
	now := s.now().UnixMilli()
	var state, detail, fail string
	withdrawn, late := false, false
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		cur, err := q.GetWish(ctx, w.ID)
		if err != nil {
			return fmt.Errorf("wish: %w", err)
		}
		withdrawn = cur.State == model.WishWithdrawn
		if late = cur.State != model.WishQueued && !withdrawn; late {
			ids := []int{}
			if res.film != nil {
				ids = append(ids, res.film.ID)
			}
			for _, hit := range res.seasons {
				if hit.found && !slices.Contains(ids, hit.req.ID) {
					ids = append(ids, hit.req.ID)
				}
			}
			return s.event(ctx, q, w.ID, res.how, map[string]any{"requestIds": ids, "late": cur.State})
		}
		ids := []int{}
		var reqID, updated sql.NullInt64
		if res.film != nil {
			state, detail, fail = requestState(res.film.Status, res.mediaState)
			reqID, updated = num64(int64(res.film.ID)), updatedOf(*res.film)
			ids = append(ids, res.film.ID)
		} else {
			seasons, err := q.ListWishSeasons(ctx, w.ID)
			if err != nil {
				return fmt.Errorf("seasons: %w", err)
			}
			var states []string
			for _, se := range seasons {
				st := se.State
				if hit, ok := res.seasons[int(se.SeasonNumber)]; ok {
					var rid, up sql.NullInt64
					if hit.found {
						st, _, _ = requestState(hit.req.Status, hit.media)
						rid, up = num64(int64(hit.req.ID)), updatedOf(hit.req)
						if !slices.Contains(ids, hit.req.ID) {
							ids = append(ids, hit.req.ID)
						}
					} else {
						st = model.WishProcessing // in Seerr already, «wird eingelesen»
					}
					if withdrawn {
						st = model.WishWithdrawn
					}
					if _, err := q.SetWishSeasonRequest(ctx, wishq.SetWishSeasonRequestParams{SeerrRequestID: rid, SeerrUpdatedAt: up,
						State: st, WishID: w.ID, SeasonNumber: se.SeasonNumber}); err != nil {
						return fmt.Errorf("season request: %w", err)
					}
				}
				states = append(states, st)
			}
			state = showState(states)
			if state == model.WishProcessing && (res.mediaState == seerr.MediaAvailable || res.mediaState == seerr.MediaPartiallyAvailable || len(ids) == 0) {
				detail = model.WishDetailImporting
			}
			if state == model.WishFailed {
				fail = model.WishFailArrFailed
			}
		}
		media := sql.NullInt64{Int64: int64(res.mediaID), Valid: res.mediaID > 0}
		keep := state
		if withdrawn {
			keep = model.WishWithdrawn
		}
		keepFail := sql.NullString{}
		if keep == model.WishFailed {
			keepFail = nullString(fail)
		}
		if _, err := q.SetWishSubmitted(ctx, wishq.SetWishSubmittedParams{SeerrRequestID: reqID, SeerrMediaID: media, State: keep, FailReason: keepFail,
			Now: num64(now), SeerrUpdatedAt: updated, ID: w.ID}); err != nil {
			return fmt.Errorf("submitted: %w", err)
		}
		if withdrawn {
			return s.event(ctx, q, w.ID, res.how, map[string]any{"requestIds": ids, "withdrawn": true})
		}
		p := wishq.SetWishStateParams{State: state, StateDetail: nullString(detail), FailReason: nullString(fail), Now: now, ID: w.ID}
		if state == model.WishProcessing {
			p.ApprovedAt = num64(now)
		}
		if !isOpen(state) {
			p.ClosedAt = num64(now)
		}
		if _, err := q.SetWishState(ctx, p); err != nil {
			return fmt.Errorf("state: %w", err)
		}
		return s.event(ctx, q, w.ID, res.how, map[string]any{"requestIds": ids, "state": state})
	})
	if err != nil {
		return err
	}
	if late {
		s.log.InfoContext(ctx, "wish reached seerr after it left the outbox", "wish_id", w.ID, "how", res.how)
		return nil
	}
	s.log.InfoContext(ctx, "wish reached seerr", "wish_id", w.ID, "how", res.how, "state", state, "withdrawn", withdrawn)
	if withdrawn {
		return s.queueWish(ctx, JobWithdraw, w.ID, 0)
	}
	s.announce(ctx, w.ID, "", state, fail)
	if state == model.WishProcessing {
		s.tagsSoon(ctx)
	}
	s.refreshQuotaOf(ctx, int(w.SeerrUserID.Int64))
	return nil
}

// announce tells the Notifier about a change of state: pending asks for a review, pending to
// processing is an approval, declined and failed close the wish. An approval at creation (from
// queued) is no message.
//
// Known limit (review Minor 8): it runs after the commit of the change, so a crash in between
// loses «zur Freigabe», «freigegeben», «abgelehnt» and «fehlgeschlagen» for good. «Verfügbar» is
// caught up because wish_requesters.notified_at marks what was sent (ListUnnotifiedAvailableWishes);
// the status messages have no such mark, and a catch-up would need one in the schema (a column or
// an outbox of messages), not a change of this package alone.
func (s *Service) announce(ctx context.Context, wishID int64, from, to, fail string) {
	if s.notifier == nil || from == to {
		return
	}
	var err error
	switch {
	case to == model.WishPending:
		err = s.notifier.WishReview(ctx, wishID)
	case to == model.WishProcessing && from == model.WishPending:
		err = s.notifier.WishApproved(ctx, wishID)
	case to == model.WishDeclined:
		err = s.notifier.WishDeclined(ctx, wishID)
	case to == model.WishFailed:
		err = s.notifier.WishFailed(ctx, wishID, fail)
	}
	if err != nil {
		s.log.WarnContext(ctx, "wish change not announced", "wish_id", wishID, "state", to, "err", err)
	}
}

// refreshQuotaOf reads the limit state of the user anew after a sent wish (N-35); a failure is
// logged.
func (s *Service) refreshQuotaOf(ctx context.Context, seerrUserID int) {
	admin, err := s.AdminSession(ctx)
	if err == nil {
		err = s.refreshQuota(ctx, admin, seerrUserID)
	}
	if err != nil {
		s.log.InfoContext(ctx, "seerr quota not read after a wish", "seerr_user_id", seerrUserID, "reason", errorClass(err))
	}
}

// failWish closes a wish as failed with its open seasons and tells the Notifier.
func (s *Service) failWish(ctx context.Context, wishID int64, reason string, detail map[string]any) error {
	from := ""
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		w, err := q.GetWish(ctx, wishID)
		if err != nil {
			return fmt.Errorf("wish: %w", err)
		}
		from = w.State
		now := s.now().UnixMilli()
		if _, err := q.SetWishState(ctx, wishq.SetWishStateParams{State: model.WishFailed, FailReason: nullString(reason), Now: now,
			ClosedAt: num64(now), ID: wishID}); err != nil {
			return fmt.Errorf("fail wish: %w", err)
		}
		if err := s.closeSeasons(ctx, q, wishID, model.WishFailed); err != nil {
			return err
		}
		d := map[string]any{"from": from, "to": model.WishFailed, "reason": reason}
		for k, v := range detail {
			d[k] = v
		}
		return s.event(ctx, q, wishID, "state_changed", d)
	})
	if err != nil {
		return err
	}
	s.log.InfoContext(ctx, "wish failed", "wish_id", wishID, "reason", reason)
	s.announce(ctx, wishID, from, model.WishFailed, reason)
	return nil
}

// closeSeasons moves the open seasons of a wish to state.
func (s *Service) closeSeasons(ctx context.Context, q *wishq.Queries, wishID int64, state string) error {
	seasons, err := q.ListWishSeasons(ctx, wishID)
	if err != nil {
		return fmt.Errorf("seasons: %w", err)
	}
	for _, se := range seasons {
		if _, open := stateRank[se.State]; !open {
			continue
		}
		if _, err := q.SetWishSeasonState(ctx, wishq.SetWishSeasonStateParams{State: state, WishID: wishID, SeasonNumber: se.SeasonNumber}); err != nil {
			return fmt.Errorf("season state: %w", err)
		}
	}
	return nil
}

// restState is the rest of the outbox (T-2): the open rest since Since and the finished spans.
type restState struct {
	Since int64      `json:"since,omitempty"`
	Spans [][2]int64 `json:"spans,omitempty"`
}

// markRest opens a rest when the outbox meets Seerr switched off and closes it when Seerr answers
// a session again.
func (s *Service) markRest(ctx context.Context, resting bool) {
	var st restState
	if _, err := s.stateJSON(ctx, settingRest, &st); err != nil {
		s.log.WarnContext(ctx, "outbox rest not read", "err", err)
		return
	}
	now := s.now().UnixMilli()
	switch {
	case resting && st.Since == 0:
		st.Since = now
	case !resting && st.Since != 0:
		st.Spans = append(st.Spans, [2]int64{st.Since, now})
		st.Since = 0
		st.Spans = slices.DeleteFunc(st.Spans, func(sp [2]int64) bool { return sp[1] < now-restKeep.Milliseconds() })
	default:
		return
	}
	if err := s.setStateJSON(ctx, s.write, settingRest, st); err != nil {
		s.log.WarnContext(ctx, "outbox rest not stored", "err", err)
	}
}

// rested is how long the outbox rested between from and to (ms).
func (s *Service) rested(ctx context.Context, from, to int64) int64 {
	var st restState
	if _, err := s.stateJSON(ctx, settingRest, &st); err != nil {
		return 0
	}
	spans := st.Spans
	if st.Since != 0 {
		spans = append(spans, [2]int64{st.Since, to})
	}
	var sum int64
	for _, sp := range spans {
		if a, b := max(sp[0], from), min(sp[1], to); b > a {
			sum += b - a
		}
	}
	return sum
}
