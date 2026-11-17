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
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// The take-over of requests made in Seerr (concept 25 stage 4, build step 4.6, F-21; architecture
// 7.3, users and profiles 8.2). The regular sync stops at the first request older than its
// pointer and never sees the old requests of a user that is linked later, so the job
// wishes.import:<seerrUserId> reads GET /request?requestedBy=<id> page by page as user 1. It runs
// when the Seerr user of an account is set or changed (by hand, and after every sync for each
// ready mapping whose user was set since its last take-over, which covers the links of the
// provisioning), and by hand («Anfragen übernehmen»). The rules:
//
//   - A request of a linked user without a wish becomes a wish of origin seerr («In Seerr
//     angelegt») owned by the main profile of the account; it gets no profile tag (N-27).
//   - Only open requests are taken over, and those Seerr reports available only when they changed
//     in the last 30 days (Vorschlag of 03 7.3); declined, failed and 4K requests are not.
//   - A wish that already holds the title open (a wish of Slouchcat not yet answered by Seerr) wins:
//     nothing is created, the outbox attaches it.
//   - A wish Slouchcat already keeps without a requester («nicht zuordenbar») gets the main profile
//     as owner; its state and history stay.
//   - Nothing of the shared user «slouchcat-gaeste» is taken over (N-102).
//   - No message goes out for a taken-over request; a film the library already has is marked as
//     reported for its owner, so the reconcile sends no «verfügbar» either.
//   - A second run creates nothing: a request is known by its id.
//
// The regular sync hands every unknown request to the same path (architecture 7.1): a linked
// user's becomes a wish of its account, another user's a wish without a requester, shown only in
// the administration.

// JobImport is the job type of the take-over.
const JobImport = "wishes.import"

const (
	importPriority = 5
	importPageSize = 50
	importWindow   = 30 * 24 * time.Hour // requests Seerr reports available are taken over this long
)

type importPayload struct {
	SeerrUserID int  `json:"seerrUserId"`
	Again       bool `json:"again,omitempty"` // by hand: from the first page, even when done
}

// importState is the state of the take-over of one user (setting wishes.import.<id>): the mapping
// it belongs to (set_at), the next page and whether it is done.
type importState struct {
	SetAt int64 `json:"setAt"`
	Page  int   `json:"page"`
	Done  bool  `json:"done"`
}

func importKey(id int) string      { return JobImport + ":" + strconv.Itoa(id) }
func importStateKey(id int) string { return JobImport + "." + strconv.Itoa(id) }

func (s *Service) registerImportType(r *jobs.Registry) {
	r.Register(jobs.Type{
		Name: JobImport, Group: jobs.GroupSystem, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent,
		Priority: importPriority, Handler: s.handleImport,
	})
}

func (s *Service) queueImport(ctx context.Context, seerrUserID, prio int, again bool) error {
	if s.queue == nil {
		return errors.New("wishes: no job queue")
	}
	payload, err := json.Marshal(importPayload{SeerrUserID: seerrUserID, Again: again})
	if err != nil {
		return err
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobImport, Priority: prio, IdempotencyKey: importKey(seerrUserID), Payload: payload})
	if err != nil {
		return fmt.Errorf("enqueue %s: %w", JobImport, err)
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return fmt.Errorf("raise %s: %w", JobImport, err)
	}
	return nil
}

// ImportSeerrRequests queues the take-over by hand («Anfragen übernehmen», importSeerrRequests):
// for one Seerr user, or with nil for every user linked to an account. It returns the number of
// users queued. model.ErrSeerrNotConfigured without address or key; model.ErrNotFound for a user
// that belongs to no account (the shared user included).
func (s *Service) ImportSeerrRequests(ctx context.Context, seerrUserID *int) (int, error) {
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return 0, err
	}
	if !cfg.Configured() {
		return 0, model.ErrSeerrNotConfigured
	}
	if seerrUserID != nil {
		if _, ok, err := s.importOwner(ctx, *seerrUserID); err != nil {
			return 0, err
		} else if !ok {
			return 0, model.ErrNotFound
		}
		return 1, s.queueImport(ctx, *seerrUserID, jobs.ManualPriority, true)
	}
	rows, err := s.read.ListReadyMappingsForImport(ctx)
	if err != nil {
		return 0, fmt.Errorf("ready mappings: %w", err)
	}
	n := 0
	for _, m := range rows {
		if !m.SeerrUserID.Valid {
			continue
		}
		if err := s.queueImport(ctx, int(m.SeerrUserID.Int64), jobs.ManualPriority, true); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// CountSeerrRequests is the number of requests of a Seerr user for the message before a mapping
// is saved («3 Anfragen von grosseltern werden übernommen», countSeerrRequests): Seerr's
// pageInfo.results of GET /request?requestedBy=<id>&take=1 as user 1. It counts every request of
// the user, also those the take-over leaves out (declined, failed, older available ones). The
// shared user counts 0 (N-102). Errors: model.ErrSeerrNotConfigured, model.ErrInstanceChanged,
// model.ErrSeerrUnreachable, model.ErrSeerrFailed.
func (s *Service) CountSeerrRequests(ctx context.Context, seerrUserID int) (int, error) {
	if u, err := s.read.GetSeerrUser(ctx, int64(seerrUserID)); err == nil && u.IsShared == 1 {
		return 0, nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("seerr user: %w", err)
	}
	admin, err := s.reviewSession(ctx)
	if err != nil {
		return 0, err
	}
	page, err := admin.Requests(ctx, seerr.RequestQuery{Take: 1, RequestedBy: seerrUserID})
	if err != nil {
		return 0, reviewErr(err)
	}
	return page.PageInfo.Results, nil
}

func (s *Service) handleImport(ctx context.Context, j *jobs.Job) error {
	var p importPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil || p.SeerrUserID < 1 {
		return jobs.Final(fmt.Errorf("%s: bad payload", JobImport))
	}
	return s.importRequests(ctx, p.SeerrUserID, p.Again)
}

// importRequests takes over the requests of one user, page by page; the state is saved with every
// page, so an aborted run goes on where it stopped. Seerr switched off, not set up or another
// instance: nothing is read. A user that belongs to no account any more: nothing to do.
func (s *Service) importRequests(ctx context.Context, seerrUserID int, again bool) error {
	_, c, err := s.connection(ctx, false)
	if err == nil {
		if pending, perr := s.instancePending(ctx); perr != nil {
			return perr
		} else if pending != nil {
			err = model.ErrInstanceChanged
		}
	}
	if paused(err) {
		return nil
	}
	if err != nil {
		return err
	}
	own, ok, err := s.importOwner(ctx, seerrUserID)
	if err != nil || !ok {
		return err
	}
	var st importState
	if _, err := s.stateJSON(ctx, importStateKey(seerrUserID), &st); err != nil {
		return err
	}
	if again || st.SetAt != own.setAt {
		st = importState{SetAt: own.setAt}
	} else if st.Done {
		return nil
	}
	admin := c.AsAdmin()
	taken := 0
	for {
		page, err := admin.Requests(ctx, seerr.RequestQuery{Take: importPageSize, Skip: st.Page * importPageSize, RequestedBy: seerrUserID})
		if err != nil {
			s.recordOutcome(ctx, seerrErr(err))
			return err
		}
		cands := make([]takeOverCandidate, 0, len(page.Results))
		for _, r := range page.Results {
			if r.RequestedBy != nil && r.RequestedBy.ID != seerrUserID {
				continue // Seerr filtered by requestedBy; anything else is not this user's
			}
			cands = append(cands, s.candidateOf(ctx, r, &own.owner))
		}
		st.Page++
		st.Done = len(page.Results) < importPageSize
		n, err := s.applyTakeOvers(ctx, cands, func(q *wishq.Queries) error {
			return s.setStateJSON(ctx, q, importStateKey(seerrUserID), st)
		})
		if err != nil {
			return err
		}
		taken += n
		if st.Done {
			break
		}
	}
	s.recordOutcome(ctx, nil)
	s.log.InfoContext(ctx, "seerr requests taken over", "seerr_user_id", seerrUserID, "wishes", taken, "pages", st.Page)
	return nil
}

// takeOverOwner is the profile a taken-over request belongs to.
type takeOverOwner struct {
	profile model.Profile
}

// importTarget is the owner of a linked user's requests and the time the user was set.
type importTarget struct {
	owner takeOverOwner
	setAt int64
}

// importOwner finds the main profile of the account the Seerr user belongs to; false for a user
// without a ready mapping, for the shared user and for an account without a main profile.
func (s *Service) importOwner(ctx context.Context, seerrUserID int) (importTarget, bool, error) {
	m, err := s.read.GetMappingBySeerrUser(ctx, sql.NullInt64{Int64: int64(seerrUserID), Valid: true})
	if errors.Is(err, sql.ErrNoRows) {
		return importTarget{}, false, nil
	}
	if err != nil {
		return importTarget{}, false, fmt.Errorf("mapping of seerr user: %w", err)
	}
	if m.State != StateReady {
		return importTarget{}, false, nil
	}
	if u, err := s.read.GetSeerrUser(ctx, int64(seerrUserID)); err == nil && u.IsShared == 1 {
		return importTarget{}, false, nil
	}
	p, err := s.read.GetPrimaryProfileOfAccount(ctx, m.AccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return importTarget{}, false, nil
	}
	if err != nil {
		return importTarget{}, false, fmt.Errorf("main profile: %w", err)
	}
	return importTarget{
		owner: takeOverOwner{profile: model.Profile{ID: p.ID, AccountID: p.AccountID, Language: p.Language, MetadataLanguage: p.MetadataLanguage.String}},
		setAt: m.SetAt,
	}, true, nil
}

// syncOwner is the owner of an unknown request the sync met: the main profile of the account of a
// linked user, nil for a user of no account; skip for the shared user (N-102) and a request
// without a user.
func (s *Service) syncOwner(ctx context.Context, r seerr.Request) (owner *takeOverOwner, skip bool, err error) {
	if r.RequestedBy == nil || r.RequestedBy.ID < 1 {
		return nil, true, nil
	}
	id := r.RequestedBy.ID
	u, err := s.read.GetSeerrUser(ctx, int64(id))
	switch {
	case err == nil && u.IsShared == 1:
		return nil, true, nil
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return nil, false, fmt.Errorf("seerr user: %w", err)
	}
	t, ok, err := s.importOwner(ctx, id)
	if err != nil || !ok {
		return nil, false, err
	}
	return &t.owner, false, nil
}

// takeOverCandidate is a request prepared outside the transaction: the wish it would become, with
// the title read from TMDB (no network call inside a write transaction).
type takeOverCandidate struct {
	req      seerr.Request
	owner    *takeOverOwner // nil: no requester («nicht zuordenbar»)
	skip     bool           // not taken over (declined, failed, 4K, too old, without media)
	media    string
	tmdbID   int64
	tvdbID   int64
	imdbID   string
	year     int
	title    string
	state    string
	detail   string
	seasons  map[int]string // season number to its state (series)
	itemID   string         // the library title when Seerr reports it available and the library has it
	userInDB bool           // the requesting user is in the mirror (seerr_user_id is a reference)
}

// candidateOf prepares a request for the take-over.
func (s *Service) candidateOf(ctx context.Context, r seerr.Request, owner *takeOverOwner) takeOverCandidate {
	c := takeOverCandidate{req: r, owner: owner, skip: true}
	if r.Media == nil || r.Media.TMDBID < 1 || r.Is4K {
		return c
	}
	if known, err := s.requestKnown(ctx, s.read, r.ID); err != nil || known {
		c.skip = err != nil
		return c // a known request is never created again; it may still get its owner
	}
	switch {
	case r.Type == seerr.MediaTypeMovie || (r.Type == "" && r.Media.MediaType == seerr.MediaTypeMovie):
		c.media = model.WishMediaMovie
		c.state, c.detail, _ = requestState(r.Status, r.Media.Status)
	case r.Type == seerr.MediaTypeTV || (r.Type == "" && r.Media.MediaType == seerr.MediaTypeTV):
		c.media = model.WishMediaShow
		c.seasons = map[int]string{}
		var states []string
		importing := false
		for _, sr := range r.Seasons {
			if sr.SeasonNumber < 1 {
				continue
			}
			st, detail, _ := requestState(r.Status, seasonMedia(r.Media, sr.SeasonNumber))
			if _, open := stateRank[st]; !open {
				continue
			}
			c.seasons[sr.SeasonNumber] = st
			states = append(states, st)
			importing = importing || detail == model.WishDetailImporting
		}
		if len(states) == 0 {
			return c
		}
		c.state = showState(states)
		if importing && c.state == model.WishProcessing {
			c.detail = model.WishDetailImporting
		}
	default:
		return c
	}
	if _, open := stateRank[c.state]; !open {
		return c
	}
	if c.detail == model.WishDetailImporting && r.UpdatedAt.Before(s.now().Add(-importWindow)) {
		return c
	}
	c.tmdbID = int64(r.Media.TMDBID)
	if r.Media.TVDBID != nil && *r.Media.TVDBID > 0 {
		c.tvdbID = int64(*r.Media.TVDBID)
	}
	t := metadata.ExternalTitle{Media: c.media, TMDBID: c.tmdbID}
	if s.titles != nil {
		if got, err := s.titles.EnsureExternalTitle(ctx, c.media, c.tmdbID); err == nil {
			t = got
		} else {
			s.log.InfoContext(ctx, "title of a taken-over request not read", "tmdb_id", c.tmdbID, "err", err)
		}
	}
	p := model.Profile{}
	if owner != nil {
		p = owner.profile
	}
	c.title = titleName(t, p)
	c.imdbID, c.year = t.IMDBID, t.Year
	if c.tvdbID == 0 {
		c.tvdbID = t.TVDBID
	}
	if c.detail == model.WishDetailImporting && s.lib != nil && owner != nil {
		ref := visibility.ExternalRef{Media: c.media, TMDBID: c.tmdbID, TVDBID: c.tvdbID, IMDBID: c.imdbID}
		if itemID, found, err := s.lib.TitleByExternalIDs(ctx, ref); err == nil && found {
			c.itemID = itemID
		}
	}
	if r.RequestedBy != nil {
		_, err := s.read.GetSeerrUser(ctx, int64(r.RequestedBy.ID))
		c.userInDB = err == nil
	}
	c.skip = false
	return c
}

// requestKnown reports whether a wish or a season carries the request.
func (s *Service) requestKnown(ctx context.Context, q *wishq.Queries, requestID int) (bool, error) {
	id := sql.NullInt64{Int64: int64(requestID), Valid: true}
	if _, err := q.GetWishBySeerrRequest(ctx, id); err == nil {
		return true, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("wish of request: %w", err)
	}
	seasons, err := q.ListWishSeasonsByRequest(ctx, id)
	if err != nil {
		return false, fmt.Errorf("seasons of request: %w", err)
	}
	return len(seasons) > 0, nil
}

// applyTakeOvers writes the candidates and then, in the same transaction, what fin writes (the
// state of a page). It returns the number of wishes created or given an owner.
func (s *Service) applyTakeOvers(ctx context.Context, cands []takeOverCandidate, fin func(q *wishq.Queries) error) (int, error) {
	n, importing := 0, false
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		n, importing = 0, false
		for _, c := range cands {
			done, err := s.takeOver(ctx, q, c)
			if err != nil {
				return err
			}
			if done {
				n++
				importing = importing || c.detail == model.WishDetailImporting
			}
		}
		if fin != nil {
			return fin(q)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if importing {
		if err := s.queueReconcile(ctx, nil); err != nil {
			s.log.WarnContext(ctx, "reconcile not queued after a take-over", "err", err)
		}
	}
	return n, nil
}

// takeOver writes one candidate: a known wish without a requester gets the owner, an unknown
// request becomes a wish unless an open wish holds the title. It reports whether it wrote.
func (s *Service) takeOver(ctx context.Context, q *wishq.Queries, c takeOverCandidate) (bool, error) {
	if known, err := s.requestKnown(ctx, q, c.req.ID); err != nil {
		return false, err
	} else if known {
		return s.adoptKnown(ctx, q, c)
	}
	if c.skip {
		return false, nil
	}
	seasons := map[int]string{}
	if c.media == model.WishMediaMovie {
		if _, err := q.GetOpenMovieWish(ctx, c.tmdbID); err == nil {
			return false, nil // a wish of Slouchcat holds the film open; the outbox attaches it
		} else if !errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("open wish: %w", err)
		}
	} else {
		var states []string
		for n, st := range c.seasons {
			if _, err := q.GetOpenSeasonWish(ctx, wishq.GetOpenSeasonWishParams{TmdbID: c.tmdbID, SeasonNumber: int64(n)}); err == nil {
				continue
			} else if !errors.Is(err, sql.ErrNoRows) {
				return false, fmt.Errorf("open season: %w", err)
			}
			seasons[n] = st
			states = append(states, st)
		}
		if len(states) == 0 {
			return false, nil
		}
		c.state = showState(states)
		if c.state != model.WishProcessing {
			c.detail = ""
		}
	}
	now := s.now().UnixMilli()
	p := wishq.InsertWishParams{
		Media: c.media, TmdbID: c.tmdbID, TvdbID: sql.NullInt64{Int64: c.tvdbID, Valid: c.tvdbID > 0},
		ImdbID: nullString(c.imdbID), Title: c.title, Year: sql.NullInt64{Int64: int64(c.year), Valid: c.year > 0},
		Origin: model.WishOriginSeerr, SeerrMediaID: sql.NullInt64{Int64: int64(c.req.Media.ID), Valid: c.req.Media.ID > 0},
		State: c.state, Now: now, SubmittedAt: num64(c.req.CreatedAt.UnixMilli()),
	}
	if c.userInDB {
		p.SeerrUserID = num64(int64(c.req.RequestedBy.ID))
	}
	if c.media == model.WishMediaMovie {
		p.SeerrRequestID = num64(int64(c.req.ID))
	}
	id, err := q.InsertWish(ctx, p)
	if err != nil {
		return false, fmt.Errorf("insert taken-over wish: %w", err)
	}
	sp := wishq.SetWishStateParams{State: c.state, StateDetail: nullString(c.detail), Now: now, ID: id}
	if c.state == model.WishProcessing {
		sp.ApprovedAt = num64(now)
	}
	if _, err := q.SetWishState(ctx, sp); err != nil {
		return false, fmt.Errorf("taken-over state: %w", err)
	}
	if _, err := q.SetWishSeerr(ctx, wishq.SetWishSeerrParams{SeerrUpdatedAt: updatedOf(c.req), Now: now, ID: id}); err != nil {
		return false, fmt.Errorf("taken-over seerr: %w", err)
	}
	for n, st := range seasons {
		if err := q.UpsertWishSeason(ctx, wishq.UpsertWishSeasonParams{WishID: id, SeasonNumber: int64(n), TmdbID: c.tmdbID,
			SeerrRequestID: num64(int64(c.req.ID)), SeerrUpdatedAt: updatedOf(c.req), State: st}); err != nil {
			return false, fmt.Errorf("taken-over season: %w", err)
		}
	}
	profileID := any(nil)
	if c.owner != nil {
		profileID = c.owner.profile.ID
		if err := s.addTakeOverOwner(ctx, q, id, c.owner, c.itemID != ""); err != nil {
			return false, err
		}
	}
	return true, s.event(ctx, q, id, "imported", map[string]any{"requestId": c.req.ID, "seerrUserId": seerrUserOf(c.req), "profileId": profileID})
}

// adoptKnown gives a known wish without a requester the owner (users and profiles 8.2 point 7);
// its state and history stay. A wish that is already available needs no message.
func (s *Service) adoptKnown(ctx context.Context, q *wishq.Queries, c takeOverCandidate) (bool, error) {
	if c.owner == nil {
		return false, nil
	}
	id := sql.NullInt64{Int64: int64(c.req.ID), Valid: true}
	var wishIDs []int64
	if w, err := q.GetWishBySeerrRequest(ctx, id); err == nil {
		wishIDs = append(wishIDs, w.ID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("wish of request: %w", err)
	}
	seasons, err := q.ListWishSeasonsByRequest(ctx, id)
	if err != nil {
		return false, fmt.Errorf("seasons of request: %w", err)
	}
	wishIDs = append(wishIDs, distinctWishes(seasons)...)
	done := false
	for _, wishID := range wishIDs {
		reqs, err := q.ListWishRequesters(ctx, wishID)
		if err != nil {
			return false, fmt.Errorf("requesters: %w", err)
		}
		if len(reqs) > 0 {
			continue
		}
		w, err := q.GetWish(ctx, wishID)
		if err != nil {
			return false, fmt.Errorf("wish: %w", err)
		}
		if err := s.addTakeOverOwner(ctx, q, wishID, c.owner, w.State == model.WishAvailable); err != nil {
			return false, err
		}
		if err := s.event(ctx, q, wishID, "owner_changed", map[string]any{"to": c.owner.profile.ID, "takenOver": true}); err != nil {
			return false, err
		}
		done = true
	}
	return done, nil
}

// addTakeOverOwner makes the profile the owner of a taken-over wish. reported marks the wish as
// told to the profile: a title the library had before the take-over is no news (03 7.3).
func (s *Service) addTakeOverOwner(ctx context.Context, q *wishq.Queries, wishID int64, o *takeOverOwner, reported bool) error {
	now := s.now().UnixMilli()
	if _, err := q.InsertWishRequester(ctx, wishq.InsertWishRequesterParams{
		WishID: wishID, ProfileID: o.profile.ID, AccountID: o.profile.AccountID, Role: model.WishRoleOwner, Now: now,
	}); err != nil {
		return fmt.Errorf("owner of a taken-over wish: %w", err)
	}
	if !reported {
		return nil
	}
	if _, err := q.SetWishRequesterNotified(ctx, wishq.SetWishRequesterNotifiedParams{Now: num64(now), WishID: wishID, ProfileID: o.profile.ID}); err != nil {
		return fmt.Errorf("reported: %w", err)
	}
	seasons, err := q.ListWishSeasons(ctx, wishID)
	if err != nil {
		return fmt.Errorf("seasons: %w", err)
	}
	for _, se := range seasons {
		if _, err := q.SetWishSeasonNotified(ctx, wishq.SetWishSeasonNotifiedParams{Now: num64(now), WishID: wishID, SeasonNumber: se.SeasonNumber}); err != nil {
			return fmt.Errorf("season reported: %w", err)
		}
	}
	return nil
}

func seerrUserOf(r seerr.Request) any {
	if r.RequestedBy == nil {
		return nil
	}
	return r.RequestedBy.ID
}

// queueImports queues the take-over of every ready mapping whose user was set since its last
// take-over, or whose take-over did not finish (after each sync). It covers the links of the
// provisioning (by the email, after an invitation or an email change) as well as a choice by hand.
func (s *Service) queueImports(ctx context.Context) error {
	rows, err := s.read.ListReadyMappingsForImport(ctx)
	if err != nil {
		return fmt.Errorf("ready mappings: %w", err)
	}
	for _, m := range rows {
		if !m.SeerrUserID.Valid {
			continue
		}
		id := int(m.SeerrUserID.Int64)
		var st importState
		ok, err := s.stateJSON(ctx, importStateKey(id), &st)
		if err != nil {
			return err
		}
		if ok && st.SetAt == m.SetAt && st.Done {
			continue
		}
		if err := s.queueImport(ctx, id, importPriority, false); err != nil {
			return err
		}
	}
	return nil
}
