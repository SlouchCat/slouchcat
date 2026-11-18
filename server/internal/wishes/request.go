package wishes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Requesting (concept 25, architecture 6.1, 6.2, 6.5 and 19.2; build step 2.4): createWish with the
// co-wish (F-7, also per season), the pre-check of the limit, the season list of the dialog and
// the lists of «Meine Wünsche» and the administration. A new wish waits in the outbox (queued) and
// gets one immediate attempt (submit.go).

var (
	errNoLibrary = errors.New("wishes: no library")
	errNoTitles  = errors.New("wishes: no titles outside the library")
)

const (
	seasonsTTL    = time.Hour       // the season list of Seerr per series (architecture 6.2)
	seasonsWait   = 3 * time.Second // the dialog waits so long for Seerr
	listMax       = 10000           // a household's wishes, read at once and filtered here
	immediateWait = 3 * time.Second // the immediate attempt (architecture 6.1, 6.3)
	submitPrio    = 7               // wishes.submit and wishes.withdraw: a person waits
	submitDeadln  = 24 * time.Hour  // the outbox gives up after so long (rest excluded, T-2)
	eventDetail   = 500             // the longest detail of an event
	withdrawRetry = 2 * time.Second // a withdrawal waits for a running submission
	busyRetry     = 5 * time.Second // a second submission waits for the first
)

// CreateWish creates a wish of the profile, joins an open one, or answers what stops it
// (createWish, data model 4.4):
//
//   - *model.ValidationError: media, TMDB id, seasons with a film, season 0, a season Seerr does
//     not offer, no season left to request;
//   - *model.WishNotAllowedError: no right (WishRights reasons), films or series not allowed,
//     status_only (the setting «Nur Zustand zeigen» and the title is requested), tvdb_id_missing;
//   - model.ErrNotFound: TMDB does not know the title, or the profile may not see it;
//   - *model.WishAlreadyAvailableError: the title is in the library and the profile sees it (a
//     series only when no season is left to request);
//   - *model.WishQuotaExceededError: the free places do not reach the new seasons or the film;
//   - model.ErrTMDBUnreachable and model.ErrProviderNotConfigured from the title lookup.
//
// A co-wish never calls Seerr and never counts against the limit. A new wish gets one attempt of
// at most 3 s without pre-check; without an answer it stays queued for wishes.submit.
// createPerHour is the limit of createWish: wishes a profile makes or joins per hour (data model
// 4.4, Vorschlag 20/h); counted from wish_requesters, so it survives a restart. WithCreatePerHour
// replaces it for the e2e suite (development mode only).
const createPerHour = 20

func (s *Service) CreateWish(ctx context.Context, p model.Profile, in model.WishCreate) (model.WishCreated, error) {
	if s.lib == nil {
		return model.WishCreated{}, errNoLibrary
	}
	if s.titles == nil {
		return model.WishCreated{}, errNoTitles
	}
	if err := validateCreate(in); err != nil {
		return model.WishCreated{}, err
	}
	rights, sender, err := s.SenderFor(ctx, p.AccountID, p.ID)
	if err != nil {
		return model.WishCreated{}, err
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return model.WishCreated{}, err
	}
	if err := allowed(rights, ws, in); err != nil {
		return model.WishCreated{}, err
	}
	if n, err := s.read.CountProfileWishesSince(ctx, wishq.CountProfileWishesSinceParams{
		ProfileID: p.ID, Since: s.now().Add(-time.Hour).UnixMilli(),
	}); err != nil {
		return model.WishCreated{}, fmt.Errorf("wishes of the hour: %w", err)
	} else if n >= int64(s.createLimit) {
		return model.WishCreated{}, model.ErrTooManyAttempts
	}
	title, err := s.titles.EnsureExternalTitle(ctx, in.Media, in.TMDBID)
	if err != nil {
		return model.WishCreated{}, err
	}
	scope := visibility.Profile(p)
	place, err := s.placeOf(ctx, scope, rights, title)
	if err != nil {
		return model.WishCreated{}, err
	}
	var wanted []int
	tvdb := title.TVDBID
	if in.Media == model.WishMediaShow {
		offer, err := s.seasonOffer(ctx, title, place)
		if err != nil {
			return model.WishCreated{}, err
		}
		if wanted, err = chosenSeasons(in, offer.seasons); err != nil {
			if place.itemID != "" && len(offer.missing()) == 0 {
				return model.WishCreated{}, &model.WishAlreadyAvailableError{ItemID: place.itemID}
			}
			return model.WishCreated{}, err
		}
		if tvdb == 0 {
			tvdb = int64(offer.tvdbID)
		}
		if tvdb == 0 {
			return model.WishCreated{}, &model.WishNotAllowedError{Reason: model.WishReasonTVDBIDMissing}
		}
	}
	nw := newWish{profile: p, sender: sender, title: title, tvdb: tvdb, seasons: wanted, source: in.Source,
		statusOnly: ws.AlreadyRequested == model.WishesAlreadyStatus, name: titleName(title, p)}
	res, err := s.createTx(ctx, nw)
	if err != nil {
		return model.WishCreated{}, err
	}
	if res.created != 0 {
		s.log.InfoContext(ctx, "wish created", "wish_id", res.created, "media", in.Media, "tmdb_id", in.TMDBID, "seasons", len(wanted))
		s.attemptNow(ctx, res.created)
	}
	if len(res.joined) > 0 {
		s.tagsSoon(ctx) // a co-wisher gets its profile tag (architecture 13)
	}
	return s.created(ctx, p, res)
}

func validateCreate(in model.WishCreate) error {
	verr := &model.ValidationError{}
	switch in.Media {
	case model.WishMediaMovie:
		if len(in.Seasons) > 0 || in.AllSeasons {
			_ = verr.Add("seasons", "not_allowed")
		}
	case model.WishMediaShow:
		if len(in.Seasons) == 0 && !in.AllSeasons {
			_ = verr.Add("seasons", "required")
		}
		if len(in.Seasons) > 0 && in.AllSeasons {
			_ = verr.Add("seasons", "invalid_value")
		}
		for _, n := range in.Seasons {
			if n < 1 {
				_ = verr.Add("seasons", "out_of_range")
				break
			}
		}
	default:
		_ = verr.Add("media", "invalid_value")
	}
	if in.TMDBID < 1 {
		_ = verr.Add("tmdbId", "out_of_range")
	}
	if len(verr.Fields) > 0 {
		return verr
	}
	return nil
}

// allowed checks the right of the profile for the medium and the place it requests from.
func allowed(rights model.WishRights, ws model.WishSettings, in model.WishCreate) error {
	switch {
	case !rights.CanRequest:
		return &model.WishNotAllowedError{Reason: rights.Reason}
	case in.Media == model.WishMediaMovie && !rights.Movies:
		return &model.WishNotAllowedError{Reason: model.WishReasonMoviesNotAllowed}
	case in.Media == model.WishMediaShow && !rights.Shows:
		return &model.WishNotAllowedError{Reason: model.WishReasonShowsNotAllowed}
	case in.Source == model.WishSourceSearch && !ws.FromSearch, in.Source == model.WishSourceDetail && !ws.FromDetail:
		return &model.WishNotAllowedError{Reason: model.WishReasonSourceNotAllowed}
	}
	return nil
}

// requestRight is the right of visibility.RequestRight from the right of the profile.
func requestRight(r model.WishRights) visibility.RequestRight {
	return func(media string) bool {
		return r.CanRequest && ((media == visibility.MediaMovie && r.Movies) || (media == visibility.MediaShow && r.Shows))
	}
}

// place is where a title stands for the profile: missing, or a series of the library it sees with
// its playable seasons.
type place struct {
	itemID  string
	visible map[int]int // series of the library: playable episodes per season
	hidden  []int       // and the library seasons the profile can play none of (never offered)
}

// placeOf asks visibility (rule 2) where the title stands for the profile. A film in the library
// that the profile sees is WishAlreadyAvailableError; a title it may not see or that is rated too
// high is model.ErrNotFound.
func (s *Service) placeOf(ctx context.Context, scope visibility.Scope, rights model.WishRights, t metadata.ExternalTitle) (place, error) {
	ref := refOf(t)
	right := requestRight(rights)
	err := s.lib.MissingTitle(ctx, scope, right, ref)
	if err == nil {
		return place{}, nil
	}
	if !errors.Is(err, model.ErrNotFound) {
		return place{}, err
	}
	m, err := s.lib.MatchExternalTitles(ctx, scope, right, []visibility.ExternalRef{ref})
	if err != nil {
		return place{}, err
	}
	if len(m) == 0 || m[0].ItemID == "" {
		return place{}, model.ErrNotFound
	}
	if t.Media == model.WishMediaMovie {
		return place{}, &model.WishAlreadyAvailableError{ItemID: m[0].ItemID}
	}
	seasons, err := s.lib.VisibleSeasons(ctx, scope, m[0].ItemID)
	if err != nil {
		return place{}, err
	}
	hidden, err := s.lib.HiddenSeasons(ctx, scope, m[0].ItemID)
	if err != nil {
		return place{}, err
	}
	return place{itemID: m[0].ItemID, visible: seasons, hidden: hidden}, nil
}

func refOf(t metadata.ExternalTitle) visibility.ExternalRef {
	return visibility.ExternalRef{Media: t.Media, TMDBID: t.TMDBID, TVDBID: t.TVDBID, IMDBID: t.IMDBID}
}

// titleName is the title in the profile's metadata language, else its interface language, else
// any text, else the original title.
func titleName(t metadata.ExternalTitle, p model.Profile) string {
	for _, lang := range []string{p.MetadataLanguage, p.Language} {
		if lang == "" {
			continue
		}
		if tx, ok := t.Texts[lang]; ok && tx.Title != "" {
			return tx.Title
		}
		base, _, _ := strings.Cut(lang, "-")
		if tx, ok := t.Texts[base]; ok && tx.Title != "" {
			return tx.Title
		}
	}
	langs := make([]string, 0, len(t.Texts))
	for l, tx := range t.Texts {
		if tx.Title != "" {
			langs = append(langs, l)
		}
	}
	slices.Sort(langs)
	if len(langs) > 0 {
		return t.Texts[langs[0]].Title
	}
	if t.OriginalTitle != "" {
		return t.OriginalTitle
	}
	return fmt.Sprintf("TMDB %d", t.TMDBID)
}

// offer is the season list of a series for one profile.
type offer struct {
	source  string
	tvdbID  int
	seasons []model.WishSeasonOffer
}

func (o offer) missing() []int {
	var out []int
	for _, se := range o.seasons {
		if se.State != model.WishSeasonOfferAvailable {
			out = append(out, se.Number)
		}
	}
	return out
}

// chosenSeasons resolves the seasons of the input against the offer: «all» is every season the
// profile cannot play; a named season must be offered and not playable.
func chosenSeasons(in model.WishCreate, seasons []model.WishSeasonOffer) ([]int, error) {
	state := make(map[int]string, len(seasons))
	var missing []int
	for _, se := range seasons {
		state[se.Number] = se.State
		if se.State != model.WishSeasonOfferAvailable {
			missing = append(missing, se.Number)
		}
	}
	if in.AllSeasons {
		if len(missing) == 0 {
			return nil, (&model.ValidationError{}).Add("seasons", model.WishCodeNoSeason)
		}
		return missing, nil
	}
	var out []int
	for _, n := range in.Seasons {
		st, ok := state[n]
		if !ok || st == model.WishSeasonOfferAvailable {
			return nil, (&model.ValidationError{}).Add("seasons", model.WishCodeSeasonNotOffered)
		}
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out, nil
}

// seasonsCache keeps the season list of Seerr per series for an hour (architecture 6.2).
type seasonsCache struct {
	mu      sync.Mutex
	entries map[int64]seasonsEntry
}

type seasonsEntry struct {
	at      time.Time
	tvdbID  int
	seasons []model.WishSeasonOffer // number and episode count only
}

func (c *seasonsCache) get(tmdbID int64, now time.Time) (seasonsEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[tmdbID]
	if !ok || now.Sub(e.at) >= seasonsTTL {
		return seasonsEntry{}, false
	}
	return e, true
}

func (c *seasonsCache) put(tmdbID int64, e seasonsEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[int64]seasonsEntry{}
	}
	c.entries[tmdbID] = e
}

// seerrSeasons reads the seasons Seerr can request with GET /tv/{tmdbId} (no X-API-User, 3 s),
// from the cache within an hour. Season 0 and seasons without episodes are dropped, as Seerr does.
func (s *Service) seerrSeasons(ctx context.Context, tmdbID int64) (seasonsEntry, error) {
	if e, ok := s.seasons.get(tmdbID, s.now()); ok {
		return e, nil
	}
	admin, err := s.AdminSession(ctx)
	if err != nil {
		return seasonsEntry{}, err
	}
	cctx, cancel := context.WithTimeout(ctx, seasonsWait)
	defer cancel()
	tv, err := admin.TV(cctx, int(tmdbID))
	if err != nil {
		return seasonsEntry{}, err
	}
	e := seasonsEntry{at: s.now()}
	if tv.ExternalIDs != nil && tv.ExternalIDs.TVDBID != nil {
		e.tvdbID = *tv.ExternalIDs.TVDBID
	}
	for _, se := range tv.Seasons {
		if se.SeasonNumber >= 1 && se.EpisodeCount > 0 {
			e.seasons = append(e.seasons, model.WishSeasonOffer{Number: se.SeasonNumber, EpisodeCount: se.EpisodeCount})
		}
	}
	s.seasons.put(tmdbID, e)
	return e, nil
}

// seasonOffer is the season list of a series for a profile: Seerr's, else TMDB's
// (tmdb_fallback), each season available (the profile plays every episode), requested (another
// open wish holds it) or missing.
func (s *Service) seasonOffer(ctx context.Context, t metadata.ExternalTitle, pl place) (offer, error) {
	o := offer{source: model.WishSeasonsFromSeerr}
	var list []model.WishSeasonOffer
	if e, err := s.seerrSeasons(ctx, t.TMDBID); err == nil {
		list, o.tvdbID = e.seasons, e.tvdbID
	} else {
		s.log.InfoContext(ctx, "seasons of seerr not read, taken from tmdb", "tmdb_id", t.TMDBID, "reason", errorClass(err))
		o.source = model.WishSeasonsTMDBFallback
		for _, se := range t.Seasons {
			if se.Number >= 1 && se.EpisodeCount > 0 {
				list = append(list, model.WishSeasonOffer{Number: se.Number, EpisodeCount: se.EpisodeCount})
			}
		}
	}
	o.seasons = make([]model.WishSeasonOffer, 0, len(list))
	for _, se := range list {
		if slices.Contains(pl.hidden, se.Number) {
			continue // not offered: a request would tell the profile the library has it
		}
		se.State = model.WishSeasonOfferMissing
		switch _, err := s.read.GetOpenSeasonWish(ctx, wishq.GetOpenSeasonWishParams{TmdbID: t.TMDBID, SeasonNumber: int64(se.Number)}); {
		case pl.visible[se.Number] >= se.EpisodeCount:
			se.State = model.WishSeasonOfferAvailable
		case err == nil:
			se.State = model.WishSeasonOfferRequested
		case !errors.Is(err, sql.ErrNoRows):
			return offer{}, fmt.Errorf("open season: %w", err)
		}
		o.seasons = append(o.seasons, se)
	}
	return o, nil
}

// WishSeasons is the season list of the request dialog (getWishSeasons) for a series the profile
// may request: a missing one, or one of the library with seasons it cannot play. Errors:
// *model.WishNotAllowedError without the right for series, model.ErrNotFound for a series it may
// not see or TMDB does not know, model.ErrTMDBUnreachable.
func (s *Service) WishSeasons(ctx context.Context, p model.Profile, tmdbID int64) (model.WishSeasons, error) {
	if s.lib == nil {
		return model.WishSeasons{}, errNoLibrary
	}
	if s.titles == nil {
		return model.WishSeasons{}, errNoTitles
	}
	rights, err := s.ForProfile(ctx, p.AccountID, p.ID)
	if err != nil {
		return model.WishSeasons{}, err
	}
	if err := allowed(rights, model.WishSettings{}, model.WishCreate{Media: model.WishMediaShow}); err != nil {
		return model.WishSeasons{}, err
	}
	t, err := s.titles.EnsureExternalTitle(ctx, model.WishMediaShow, tmdbID)
	if err != nil {
		return model.WishSeasons{}, err
	}
	pl, err := s.placeOf(ctx, visibility.Profile(p), rights, t)
	if err != nil {
		return model.WishSeasons{}, err
	}
	o, err := s.seasonOffer(ctx, t, pl)
	if err != nil {
		return model.WishSeasons{}, err
	}
	return model.WishSeasons{Source: o.source, Seasons: o.seasons}, nil
}

// newWish is what createTx stores.
type newWish struct {
	profile    model.Profile
	sender     Sender
	title      metadata.ExternalTitle
	tvdb       int64
	seasons    []int
	source     string
	statusOnly bool
	name       string
}

// createResult names the new wish (0 without one) and the wishes the profile joined or already had.
type createResult struct {
	created int64
	joined  []int64
	same    int64 // the profile already had this film wish (idempotent)
}

// createTx joins open wishes and creates the rest in one transaction. Two profiles that create
// the same title at the same moment never both insert: the write pool has one connection and
// every transaction begins IMMEDIATE (store.Open), so the second runs after the first, finds its
// wish and joins it (architecture 6.1 step 4 needs no repeat here).
func (s *Service) createTx(ctx context.Context, nw newWish) (createResult, error) {
	var res createResult
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		res = createResult{}
		now := s.now().UnixMilli()
		t := nw.title
		var rest []int
		if t.Media == model.WishMediaMovie {
			open, err := q.GetOpenMovieWish(ctx, t.TMDBID)
			switch {
			case err == nil:
				same, err := s.join(ctx, q, open.ID, nw, now)
				if err != nil {
					return err
				}
				if same {
					res.same = open.ID
				} else {
					res.joined = append(res.joined, open.ID)
				}
				return nil
			case !errors.Is(err, sql.ErrNoRows):
				return fmt.Errorf("open wish: %w", err)
			}
		} else {
			for _, n := range nw.seasons {
				id, err := q.GetOpenSeasonWish(ctx, wishq.GetOpenSeasonWishParams{TmdbID: t.TMDBID, SeasonNumber: int64(n)})
				switch {
				case errors.Is(err, sql.ErrNoRows):
					rest = append(rest, n)
					continue
				case err != nil:
					return fmt.Errorf("open season: %w", err)
				}
				if slices.Contains(res.joined, id) {
					continue
				}
				if _, err := s.join(ctx, q, id, nw, now); err != nil {
					return err
				}
				res.joined = append(res.joined, id)
			}
			if len(rest) == 0 {
				return nil
			}
		}
		needed := max(1, len(rest))
		if quota, err := s.quotaOf(ctx, q, nw.profile.AccountID, nw.sender); err != nil {
			return err
		} else if quota != nil && exceeded(quota, t.Media, needed) {
			return &model.WishQuotaExceededError{Quota: *quota, Needed: needed}
		}
		id, err := q.InsertWish(ctx, wishq.InsertWishParams{
			Media: t.Media, TmdbID: t.TMDBID, TvdbID: sql.NullInt64{Int64: nw.tvdb, Valid: nw.tvdb > 0},
			ImdbID: nullString(t.IMDBID), Title: nw.name, Year: sql.NullInt64{Int64: int64(t.Year), Valid: t.Year > 0},
			Origin: model.WishOriginSlouchcat, SeerrUserID: sql.NullInt64{Int64: int64(nw.sender.SeerrUserID), Valid: true},
			State: model.WishQueued, Now: now,
		})
		if err != nil {
			return err
		}
		for _, n := range rest {
			if err := q.UpsertWishSeason(ctx, wishq.UpsertWishSeasonParams{WishID: id, SeasonNumber: int64(n), TmdbID: t.TMDBID, State: model.WishQueued}); err != nil {
				return err
			}
		}
		if _, err := q.InsertWishRequester(ctx, wishq.InsertWishRequesterParams{
			WishID: id, ProfileID: nw.profile.ID, AccountID: nw.profile.AccountID, Role: model.WishRoleOwner, Now: now,
		}); err != nil {
			return err
		}
		res.created = id
		return s.event(ctx, q, id, "created", map[string]any{"profileId": nw.profile.ID, "source": nw.source, "seasons": rest, "shared": nw.sender.Shared})
	})
	return res, err
}

// join makes the profile a requester of an open wish without a call to Seerr (F-7). It reports
// whether the profile was a requester already. The setting «Nur Zustand zeigen» refuses a new
// co-wish.
func (s *Service) join(ctx context.Context, q *wishq.Queries, wishID int64, nw newWish, now int64) (bool, error) {
	if _, err := q.GetWishRequester(ctx, wishq.GetWishRequesterParams{WishID: wishID, ProfileID: nw.profile.ID}); err == nil {
		return true, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("requester: %w", err)
	}
	if nw.statusOnly {
		return false, &model.WishNotAllowedError{Reason: model.WishReasonStatusOnly}
	}
	n, err := q.InsertWishRequester(ctx, wishq.InsertWishRequesterParams{
		WishID: wishID, ProfileID: nw.profile.ID, AccountID: nw.profile.AccountID, Role: model.WishRoleJoined, Now: now,
	})
	if err != nil {
		return false, fmt.Errorf("join wish: %w", err)
	}
	if n == 0 {
		return true, nil
	}
	return false, s.event(ctx, q, wishID, "joined", map[string]any{"profileId": nw.profile.ID})
}

// event writes one entry of the history; detail is JSON without secrets.
func (s *Service) event(ctx context.Context, q *wishq.Queries, wishID int64, kind string, detail map[string]any) error {
	var d sql.NullString
	if len(detail) > 0 {
		b, err := json.Marshal(detail)
		if err != nil {
			return fmt.Errorf("event detail: %w", err)
		}
		if len(b) <= eventDetail {
			d = sql.NullString{String: string(b), Valid: true}
		}
	}
	if err := q.InsertWishEvent(ctx, wishq.InsertWishEventParams{WishID: wishID, Now: s.now().UnixMilli(), Kind: kind, Detail: d}); err != nil {
		return fmt.Errorf("wish event %s: %w", kind, err)
	}
	return nil
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// quotaOf is the limit state of the user the profile sends as, with the wishes waiting in the
// outbox (N-35): nil while Seerr's state was never read (no pre-check then); unlimited for a user
// with MANAGE_USERS or without a limit.
func (s *Service) quotaOf(ctx context.Context, q *wishq.Queries, accountID int64, sender Sender) (*model.WishQuota, error) {
	row, err := q.GetSeerrUser(ctx, int64(sender.SeerrUserID))
	if err != nil {
		return nil, fmt.Errorf("seerr user: %w", err)
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return nil, err
	}
	limit := ws.DefaultLimit
	if !sender.Shared {
		l, err := q.GetAccountWishLimit(ctx, accountID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("limit: %w", err)
		}
		limit = accountLimit(l, err == nil, ws.DefaultLimit, row.IsAdmin == 1).Effective
	}
	quota := wishQuota(row, limit)
	if quota == nil || quota.Unlimited {
		return quota, nil
	}
	uid := sql.NullInt64{Int64: int64(sender.SeerrUserID), Valid: true}
	if quota.Movie != nil {
		n, err := q.CountQueuedMovieWishes(ctx, uid)
		if err != nil {
			return nil, fmt.Errorf("queued films: %w", err)
		}
		quota.Movie.Queued, quota.Movie.Remaining = int(n), max(0, quota.Movie.Remaining-int(n))
	}
	if quota.TV != nil {
		n, err := q.CountQueuedSeasons(ctx, uid)
		if err != nil {
			return nil, fmt.Errorf("queued seasons: %w", err)
		}
		quota.TV.Queued, quota.TV.Remaining = int(n), max(0, quota.TV.Remaining-int(n))
	}
	return quota, nil
}

func exceeded(q *model.WishQuota, media string, needed int) bool {
	if q.Unlimited {
		return false
	}
	part := q.Movie
	if media == model.WishMediaShow {
		part = q.TV
	}
	return part != nil && part.Remaining < needed
}

// WishQuota is the limit state of the user the profile sends as (getWishQuota), from the stored
// state without a call to Seerr; Queued counts the wishes waiting in the outbox and Remaining is
// what is left after them. *model.WishNotAllowedError without the right; a zero WishQuota (At nil)
// while Seerr's state was never read.
func (s *Service) WishQuota(ctx context.Context, p model.Profile) (model.WishQuota, error) {
	rights, sender, err := s.SenderFor(ctx, p.AccountID, p.ID)
	if err != nil {
		return model.WishQuota{}, err
	}
	if !rights.CanRequest {
		return model.WishQuota{}, &model.WishNotAllowedError{Reason: rights.Reason}
	}
	q, err := s.quotaOf(ctx, s.read, p.AccountID, sender)
	if err != nil || q == nil {
		return model.WishQuota{}, err
	}
	return *q, nil
}

// created builds the answer of createWish.
func (s *Service) created(ctx context.Context, p model.Profile, res createResult) (model.WishCreated, error) {
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return model.WishCreated{}, err
	}
	out := model.WishCreated{Created: res.created != 0}
	main := res.created
	if main == 0 {
		main = res.same
	}
	if main != 0 {
		w, err := s.wishFor(ctx, p.ID, main, ws)
		if err != nil {
			return model.WishCreated{}, err
		}
		out.Wish = &w
	}
	for _, id := range res.joined {
		w, err := s.wishFor(ctx, p.ID, id, ws)
		if err != nil {
			return model.WishCreated{}, err
		}
		out.Joined = append(out.Joined, w)
	}
	if err := s.withCards(ctx, p, out.Joined); err != nil {
		return model.WishCreated{}, err
	}
	if out.Wish != nil {
		cards := []model.Wish{*out.Wish}
		if err := s.withCards(ctx, p, cards); err != nil {
			return model.WishCreated{}, err
		}
		out.Wish = &cards[0]
	} else if len(out.Joined) > 0 && out.Joined[0].Media == model.WishMediaMovie {
		out.Wish = &out.Joined[0]
	}
	return out, nil
}

// unseen tells, per library item, whether a profile may not see it: an available or partial wish
// of such an item reads «In Beschaffung · wird eingelesen» for that profile (architecture 8.3,
// security review M-1). The state of the wish stays as it is; only what the profile reads changes,
// and the title stays 404 for it. One unseen serves one list (it caches per profile and item).
type unseen struct {
	s     *Service
	cache map[[2]int64]map[string]bool // profile and account -> item -> hidden
}

func (s *Service) unseen() *unseen { return &unseen{s: s, cache: map[[2]int64]map[string]bool{}} }

// hides reports whether the wish row (state, item) reads as processing for the profile.
func (u *unseen) hides(ctx context.Context, profileID, accountID int64, state string, itemID sql.NullString) (bool, error) {
	if (state != model.WishAvailable && state != model.WishPartial) || !itemID.Valid || u.s.lib == nil {
		return false, nil
	}
	k := [2]int64{profileID, accountID}
	if h, ok := u.cache[k][itemID.String]; ok {
		return h, nil
	}
	err := u.s.lib.Visible(ctx, visibility.Profile(model.Profile{ID: profileID, AccountID: accountID}), itemID.String)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return false, err
	}
	if u.cache[k] == nil {
		u.cache[k] = map[string]bool{}
	}
	u.cache[k][itemID.String] = err != nil
	return err != nil, nil
}

// asProcessing is what a hidden wish row reads: processing with «wird eingelesen», no time of the
// availability, no item.
func asProcessing(state, detail *string, availableAt *sql.NullInt64, itemID *sql.NullString) {
	*state, *detail = model.WishProcessing, model.WishDetailImporting
	*availableAt, *itemID = sql.NullInt64{}, sql.NullString{}
}

// hideAvailability removes what tells a hidden wish is in the library: available seasons read
// processing, and the events of the availability go. A hidden wish is never withdrawn (review
// m-3): its real state is available or partial, and WithdrawWish answers both alike.
func hideAvailability(w *model.Wish) {
	w.Hidden = true
	w.CanWithdraw = false
	for i := range w.Seasons {
		if w.Seasons[i].State == model.WishAvailable {
			w.Seasons[i].State = model.WishProcessing
		}
	}
	w.Events = slices.DeleteFunc(w.Events, func(e model.WishEvent) bool {
		if e.Kind == "season_available" {
			return true
		}
		var d struct {
			To string `json:"to"`
		}
		return e.Kind == "state_changed" && json.Unmarshal([]byte(e.Detail), &d) == nil && (d.To == model.WishAvailable || d.To == model.WishPartial)
	})
}

// wishFor reads one wish as the profile sees it; model.ErrNotFound when it is not the profile's.
func (s *Service) wishFor(ctx context.Context, profileID, wishID int64, ws model.WishSettings) (model.Wish, error) {
	r, err := s.read.GetWishRequester(ctx, wishq.GetWishRequesterParams{WishID: wishID, ProfileID: profileID})
	if errors.Is(err, sql.ErrNoRows) {
		return model.Wish{}, model.ErrNotFound
	}
	if err != nil {
		return model.Wish{}, fmt.Errorf("requester: %w", err)
	}
	w, err := s.read.GetWish(ctx, wishID)
	if err != nil {
		return model.Wish{}, fmt.Errorf("wish: %w", err)
	}
	hidden, err := s.unseen().hides(ctx, profileID, r.AccountID, w.State, w.ItemID)
	if err != nil {
		return model.Wish{}, err
	}
	if hidden {
		asProcessing(&w.State, &w.StateDetail.String, &w.AvailableAt, &w.ItemID)
		w.StateDetail.Valid = true
	}
	out, err := s.toWish(ctx, w, r.Role, ws)
	if hidden {
		hideAvailability(&out)
	}
	return out, err
}

// toWish maps a row with its seasons and newest events; role "" is the administration's view.
func (s *Service) toWish(ctx context.Context, w wishq.Wish, role string, ws model.WishSettings) (model.Wish, error) {
	out := model.Wish{
		ID: w.ID, Media: w.Media, TMDBID: w.TmdbID, TVDBID: w.TvdbID.Int64, IMDBID: w.ImdbID.String, Title: w.Title,
		Year: int(w.Year.Int64), State: w.State, StateDetail: w.StateDetail.String, FailReason: w.FailReason.String,
		Origin: w.Origin, Role: role, ItemID: w.ItemID.String,
		CreatedAt: time.UnixMilli(w.CreatedAt).UTC(), UpdatedAt: time.UnixMilli(w.UpdatedAt).UTC(), AvailableAt: timeOf(w.AvailableAt),
		Seasons: []model.WishSeason{}, Events: []model.WishEvent{},
	}
	seasons, err := s.read.ListWishSeasons(ctx, w.ID)
	if err != nil {
		return model.Wish{}, fmt.Errorf("seasons: %w", err)
	}
	for _, se := range seasons {
		out.Seasons = append(out.Seasons, model.WishSeason{Number: int(se.SeasonNumber), State: se.State, SeerrRequestID: int(se.SeerrRequestID.Int64)})
	}
	events, err := s.read.ListWishEvents(ctx, wishq.ListWishEventsParams{WishID: w.ID, LimitCount: model.WishEventsShown})
	if err != nil {
		return model.Wish{}, fmt.Errorf("events: %w", err)
	}
	for i, e := range events {
		out.Events = append(out.Events, model.WishEvent{At: time.UnixMilli(e.At).UTC(), Kind: e.Kind, Detail: e.Detail.String})
		if i == 0 && e.Kind == "reopened" {
			out.Hint = model.WishHintAlreadyApproved
		}
	}
	open := isOpen(w.State)
	switch role {
	case model.WishRoleOwner:
		out.CanWithdraw = ws.AllowWithdraw && (w.State == model.WishQueued || w.State == model.WishPending)
	case model.WishRoleJoined:
		out.CanWithdraw = ws.AllowWithdraw && open
	}
	out.CanRequestAgain = role != "" && !open && w.State != model.WishAvailable && w.FailReason.String != model.WishFailArrFailed
	return out, nil
}

func isOpen(state string) bool {
	switch state {
	case model.WishQueued, model.WishPending, model.WishProcessing, model.WishPartial:
		return true
	}
	return false
}

// inFilter says whether a state belongs to a filter of the lists.
func inFilter(state, filter string) bool {
	switch filter {
	case model.WishFilterOpen:
		return isOpen(state)
	case model.WishFilterAvailable:
		return state == model.WishAvailable
	case model.WishFilterClosed:
		return !isOpen(state) && state != model.WishAvailable
	}
	return true
}

func count(c *model.WishCounts, state string) {
	switch {
	case isOpen(state):
		c.Open++
		switch state {
		case model.WishPending:
			c.Pending++
		case model.WishProcessing:
			c.Processing++
		case model.WishPartial:
			c.Partial++
		}
	case state == model.WishAvailable:
		c.Available++
	default:
		c.Closed++
	}
}

func validFilter(filter string) error {
	switch filter {
	case "", model.WishFilterOpen, model.WishFilterAvailable, model.WishFilterClosed, model.WishFilterAll:
		return nil
	}
	return (&model.ValidationError{}).Add("state", "invalid_value")
}

func page[T any](all []T, offset, limit int) []T {
	if offset >= len(all) {
		return []T{}
	}
	end := len(all)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return all[offset:end]
}

// ProfileWishes is «Meine Wünsche» (listWishes): the wishes of the profile, its own and those it
// joined, newest first, filtered by open, available, closed or all; Counts count every wish of
// the profile. A *model.ValidationError for an unknown filter.
func (s *Service) ProfileWishes(ctx context.Context, p model.Profile, filter string, offset, limit int) (model.WishPage, error) {
	if err := validFilter(filter); err != nil {
		return model.WishPage{}, err
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return model.WishPage{}, err
	}
	rows, err := s.read.ListProfileWishes(ctx, wishq.ListProfileWishesParams{ProfileID: p.ID, LimitCount: listMax})
	if err != nil {
		return model.WishPage{}, fmt.Errorf("profile wishes: %w", err)
	}
	var hits []wishq.ListProfileWishesRow
	hidden := map[int64]bool{}
	seen := s.unseen()
	out := model.WishPage{Items: []model.Wish{}}
	for _, r := range rows {
		h, err := seen.hides(ctx, p.ID, p.AccountID, r.State, r.ItemID)
		if err != nil {
			return model.WishPage{}, err
		}
		if h {
			hidden[r.ID] = true
			asProcessing(&r.State, &r.StateDetail.String, &r.AvailableAt, &r.ItemID)
			r.StateDetail.Valid = true
		}
		count(&out.Counts, r.State)
		if inFilter(r.State, filter) {
			hits = append(hits, r)
		}
	}
	out.Total = len(hits)
	for _, r := range page(hits, max(0, offset), limit) {
		w, err := s.toWish(ctx, profileRow(r), r.Role, ws)
		if err != nil {
			return model.WishPage{}, err
		}
		if hidden[r.ID] {
			hideAvailability(&w)
		}
		out.Items = append(out.Items, w)
	}
	if err := s.withCards(ctx, p, out.Items); err != nil {
		return model.WishPage{}, err
	}
	return out, nil
}

// AccountWishes is listAccountWishes: the wishes of every profile of the account, one entry per
// profile and wish with the profile's name, newest first; the cards as profile p (the primary
// profile asking) sees them. Filter, total and counts as in ProfileWishes.
func (s *Service) AccountWishes(ctx context.Context, p model.Profile, filter string, offset, limit int) (model.WishPage, error) {
	if err := validFilter(filter); err != nil {
		return model.WishPage{}, err
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return model.WishPage{}, err
	}
	rows, err := s.read.ListAccountWishes(ctx, wishq.ListAccountWishesParams{AccountID: p.AccountID, LimitCount: listMax})
	if err != nil {
		return model.WishPage{}, fmt.Errorf("account wishes: %w", err)
	}
	var hits []wishq.ListAccountWishesRow
	type entry struct{ wish, profile int64 }
	hidden := map[entry]bool{}
	seen := s.unseen()
	out := model.WishPage{Items: []model.Wish{}}
	for _, r := range rows {
		// Each entry as the profile of the entry sees it (architecture 8.3).
		h, err := seen.hides(ctx, r.ProfileID, p.AccountID, r.State, r.ItemID)
		if err != nil {
			return model.WishPage{}, err
		}
		if h {
			hidden[entry{r.ID, r.ProfileID}] = true
			asProcessing(&r.State, &r.StateDetail.String, &r.AvailableAt, &r.ItemID)
			r.StateDetail.Valid = true
		}
		count(&out.Counts, r.State)
		if inFilter(r.State, filter) {
			hits = append(hits, r)
		}
	}
	out.Total = len(hits)
	for _, r := range page(hits, max(0, offset), limit) {
		w, err := s.toWish(ctx, profileRow(wishq.ListProfileWishesRow{
			ID: r.ID, Media: r.Media, TmdbID: r.TmdbID, TvdbID: r.TvdbID, ImdbID: r.ImdbID, Title: r.Title, Year: r.Year,
			Origin: r.Origin, SeerrRequestID: r.SeerrRequestID, SeerrMediaID: r.SeerrMediaID, SeerrUserID: r.SeerrUserID,
			State: r.State, StateDetail: r.StateDetail, FailReason: r.FailReason, Attempts: r.Attempts,
			NextAttemptAt: r.NextAttemptAt, WithdrawPending: r.WithdrawPending, CheckedAt: r.CheckedAt, ItemID: r.ItemID,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, SubmittedAt: r.SubmittedAt, ApprovedAt: r.ApprovedAt,
			AvailableAt: r.AvailableAt, ClosedAt: r.ClosedAt, SeerrUpdatedAt: r.SeerrUpdatedAt,
		}), r.Role, ws)
		if err != nil {
			return model.WishPage{}, err
		}
		w.ProfileName = r.ProfileName
		if hidden[entry{r.ID, r.ProfileID}] {
			hideAvailability(&w)
		}
		if r.ProfileID != p.ID {
			// Withdrawing and requesting again belong to the profile of the entry («Meine Wünsche»).
			w.CanWithdraw, w.CanRequestAgain = false, false
		}
		out.Items = append(out.Items, w)
	}
	if err := s.withCards(ctx, p, out.Items); err != nil {
		return model.WishPage{}, err
	}
	return out, nil
}

func profileRow(r wishq.ListProfileWishesRow) wishq.Wish {
	return wishq.Wish{
		ID: r.ID, Media: r.Media, TmdbID: r.TmdbID, TvdbID: r.TvdbID, ImdbID: r.ImdbID, Title: r.Title, Year: r.Year,
		Origin: r.Origin, SeerrRequestID: r.SeerrRequestID, SeerrMediaID: r.SeerrMediaID, SeerrUserID: r.SeerrUserID,
		State: r.State, StateDetail: r.StateDetail, FailReason: r.FailReason, Attempts: r.Attempts,
		NextAttemptAt: r.NextAttemptAt, WithdrawPending: r.WithdrawPending, CheckedAt: r.CheckedAt, ItemID: r.ItemID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, SubmittedAt: r.SubmittedAt, ApprovedAt: r.ApprovedAt,
		AvailableAt: r.AvailableAt, ClosedAt: r.ClosedAt, SeerrUpdatedAt: r.SeerrUpdatedAt,
	}
}

// AdminWishes is the list of the administration (listAdminWishes): every wish, the latest change
// first, with its requesters; unmapped true keeps the wishes without a requester. The total and
// the counts are those of the filter.
func (s *Service) AdminWishes(ctx context.Context, filter string, unmapped bool, offset, limit int) ([]model.AdminWish, int, model.WishCounts, error) {
	if err := validFilter(filter); err != nil {
		return nil, 0, model.WishCounts{}, err
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return nil, 0, model.WishCounts{}, err
	}
	rows, err := s.read.ListAllWishes(ctx, listMax)
	if err != nil {
		return nil, 0, model.WishCounts{}, fmt.Errorf("all wishes: %w", err)
	}
	var with map[int64]bool
	if unmapped {
		ids, err := s.read.ListWishIDsWithRequesters(ctx)
		if err != nil {
			return nil, 0, model.WishCounts{}, fmt.Errorf("wishes with requesters: %w", err)
		}
		with = make(map[int64]bool, len(ids))
		for _, id := range ids {
			with[id] = true
		}
	}
	var counts model.WishCounts
	var hits []wishq.Wish
	for _, r := range rows {
		if unmapped && with[r.ID] {
			continue
		}
		count(&counts, r.State)
		if inFilter(r.State, filter) {
			hits = append(hits, r)
		}
	}
	// Only the page is mapped (review Minor 10: two queries per wish for up to 10'000 wishes).
	out := []model.AdminWish{}
	for _, r := range page(hits, max(0, offset), limit) {
		names, err := s.read.ListWishRequesterNames(ctx, r.ID)
		if err != nil {
			return nil, 0, model.WishCounts{}, fmt.Errorf("requesters: %w", err)
		}
		a := model.AdminWish{SeerrUserID: int(r.SeerrUserID.Int64), SeerrRequestID: int(r.SeerrRequestID.Int64), Attempts: int(r.Attempts),
			Requesters: []model.AdminWishRequester{}}
		for _, n := range names {
			a.Requesters = append(a.Requesters, model.AdminWishRequester{AccountID: n.AccountID, AccountEmail: n.AccountEmail,
				ProfileID: n.ProfileID, ProfileName: n.ProfileName, Role: n.Role, Closed: n.ClosedAt.Valid})
		}
		if a.Wish, err = s.toWish(ctx, r, "", ws); err != nil {
			return nil, 0, model.WishCounts{}, err
		}
		out = append(out, a)
	}
	return out, len(hits), counts, nil
}

// sessionFor returns the user session of a wish's frozen Seerr user (N-102: a later mapping change
// moves nothing).
func (s *Service) sessionFor(ctx context.Context, w wishq.Wish) (seerr.Session, error) {
	return s.UserSession(ctx, int(w.SeerrUserID.Int64))
}
