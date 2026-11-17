package wishes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// The reconcile with the library (concept 25, architecture 8.1, 8.3 and 11; build step 2.5): a
// wish is available when Slouchcat can play it for a requester (F-13), not when Seerr says so. The
// match of a title in the library calls TitleMatched (the hook of package metadata after the
// provider ids are written, for a series after its seasons), which queues the reconcile of that
// title; the scan alone triggers nothing, because before the match the ids may be empty and the
// rating unknown. Every hour the reconcile checks every open wish. A film is available when a
// requester sees it; a season when a requester plays as many episodes as the season has (a
// running season: as many as have aired); a series when every wished season is, partial before.
// The Notifier decides per profile and writes notified_at; this package never does.

// JobReconcile is the job type of the reconcile.
const JobReconcile = "wishes.reconcile"

const (
	reconcileEvery    = time.Hour
	reconcilePriority = 5
	notifyAgainFor    = 30 * 24 * time.Hour // an available wish with waiting profiles is reported again so long
)

// reconcilePayload names the title of one match; empty: every open wish.
type reconcilePayload struct {
	ItemID string `json:"itemId,omitempty"`
	Media  string `json:"media,omitempty"`
	TMDBID int64  `json:"tmdbId,omitempty"`
	TVDBID int64  `json:"tvdbId,omitempty"`
	IMDBID string `json:"imdbId,omitempty"`
}

func (s *Service) runReconcile(ctx context.Context, _ *jobs.Queue, prio int) ([]int64, error) {
	if s.queue == nil {
		return nil, nil
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobReconcile, Priority: prio, IdempotencyKey: JobReconcile, Payload: []byte(`{}`)})
	if err != nil {
		return nil, fmt.Errorf("enqueue %s: %w", JobReconcile, err)
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, fmt.Errorf("raise %s: %w", JobReconcile, err)
	}
	return []int64{id}, nil
}

// queueReconcile queues the reconcile of one matched title, or of every open wish with nil.
func (s *Service) queueReconcile(ctx context.Context, p *reconcilePayload) error {
	if s.queue == nil {
		return nil
	}
	key, payload := JobReconcile, []byte(`{}`)
	if p != nil {
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}
		key, payload = JobReconcile+":item:"+p.ItemID, b
	}
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobReconcile, Priority: reconcilePriority, IdempotencyKey: key, Payload: payload}); err != nil {
		return fmt.Errorf("enqueue %s: %w", JobReconcile, err)
	}
	return nil
}

// TitleMatched is the hook of package metadata (metadata.Service.WithMatched): a film or series of
// the library got its provider ids. With an open wish for one of them it queues the reconcile of
// that title; a failure is logged, the hourly reconcile catches up.
func (s *Service) TitleMatched(ctx context.Context, m metadata.MatchedTitle) {
	media := model.WishMediaMovie
	switch m.Kind {
	case "movie":
	case "series":
		media = model.WishMediaShow
	default:
		return
	}
	p := reconcilePayload{ItemID: m.ItemID, Media: media, TMDBID: m.TMDBID, TVDBID: m.TVDBID, IMDBID: m.IMDBID}
	rows, err := s.wishesOfTitle(ctx, p)
	if err == nil && len(rows) > 0 {
		err = s.queueReconcile(ctx, &p)
	}
	if err != nil {
		s.log.WarnContext(ctx, "reconcile after a match not queued", "item_id", m.ItemID, "err", err)
	}
}

// wishesOfTitle are the open wishes of a matched title: by TMDB id (a series by TVDB id too), a
// film also by IMDb id.
func (s *Service) wishesOfTitle(ctx context.Context, p reconcilePayload) ([]wishq.Wish, error) {
	var out []wishq.Wish
	seen := map[int64]bool{}
	add := func(rows []wishq.Wish, err error) error {
		if err != nil {
			return fmt.Errorf("wishes of title: %w", err)
		}
		for _, w := range rows {
			if !seen[w.ID] {
				seen[w.ID] = true
				out = append(out, w)
			}
		}
		return nil
	}
	if p.TMDBID > 0 || p.TVDBID > 0 {
		if err := add(s.read.ListOpenWishesByTitle(ctx, wishq.ListOpenWishesByTitleParams{Media: p.Media, TmdbID: p.TMDBID,
			TvdbID: sql.NullInt64{Int64: p.TVDBID, Valid: p.TVDBID > 0}})); err != nil {
			return nil, err
		}
	}
	if p.Media == model.WishMediaShow && p.TVDBID > 0 {
		if err := add(s.read.ListOpenShowWishesByTVDB(ctx, sql.NullInt64{Int64: p.TVDBID, Valid: true})); err != nil {
			return nil, err
		}
	}
	if p.Media == model.WishMediaMovie && p.IMDBID != "" {
		if err := add(s.read.ListOpenMovieWishesByIMDb(ctx, sql.NullString{String: p.IMDBID, Valid: true})); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Service) handleReconcile(ctx context.Context, j *jobs.Job) error {
	var p reconcilePayload
	if len(j.Payload) > 0 {
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			return jobs.Final(fmt.Errorf("%s: bad payload", JobReconcile))
		}
	}
	return s.reconcile(ctx, p)
}

// reconcile checks the open wishes of one title, or every open wish and the available wishes whose
// requesters still wait for their message.
func (s *Service) reconcile(ctx context.Context, p reconcilePayload) error {
	if s.lib == nil {
		return errNoLibrary
	}
	var rows []wishq.Wish
	var err error
	if p.ItemID != "" {
		rows, err = s.wishesOfTitle(ctx, p)
	} else {
		rows, err = s.read.ListOpenWishes(ctx)
	}
	if err != nil {
		return fmt.Errorf("open wishes: %w", err)
	}
	done := make(map[int64]bool, len(rows))
	for _, w := range rows {
		if err := s.reconcileWish(ctx, w); err != nil {
			return err
		}
		done[w.ID] = true
	}
	if p.ItemID != "" {
		return nil
	}
	waiting, err := s.read.ListUnnotifiedAvailableWishes(ctx, num64(s.now().Add(-notifyAgainFor).UnixMilli()))
	if err != nil {
		return fmt.Errorf("waiting wishes: %w", err)
	}
	for _, w := range waiting {
		if w.ItemID.Valid && !done[w.ID] {
			s.available(ctx, w.ID, w.ItemID.String)
		}
	}
	return nil
}

// reconcileWish moves one open wish to available, partial or «wird eingelesen» after the library.
func (s *Service) reconcileWish(ctx context.Context, w wishq.Wish) error {
	ref := visibility.ExternalRef{Media: w.Media, TMDBID: w.TmdbID, TVDBID: w.TvdbID.Int64, IMDBID: w.ImdbID.String}
	itemID, found, err := s.lib.TitleByExternalIDs(ctx, ref)
	if err != nil || !found {
		return err
	}
	if w.ItemID.String != itemID {
		if _, err := s.write.SetWishItem(ctx, wishq.SetWishItemParams{ItemID: nullString(itemID), Now: s.now().UnixMilli(), ID: w.ID}); err != nil {
			return fmt.Errorf("wish item: %w", err)
		}
	}
	reqs, err := s.read.ListOpenWishRequesters(ctx, w.ID)
	if err != nil {
		return fmt.Errorf("requesters: %w", err)
	}
	scopes := make([]visibility.Scope, 0, len(reqs))
	for _, r := range reqs {
		scopes = append(scopes, visibility.Profile(model.Profile{ID: r.ProfileID, AccountID: r.AccountID}))
	}
	if w.Media == model.WishMediaMovie {
		return s.reconcileFilm(ctx, w, itemID, scopes)
	}
	return s.reconcileSeries(ctx, w, itemID, scopes)
}

func (s *Service) reconcileFilm(ctx context.Context, w wishq.Wish, itemID string, scopes []visibility.Scope) error {
	seen := len(scopes) == 0 // a wish without requesters follows the library alone
	for _, sc := range scopes {
		if err := s.lib.Visible(ctx, sc, itemID); err == nil {
			seen = true
			break
		} else if !errors.Is(err, model.ErrNotFound) {
			return err
		}
	}
	if !seen {
		// In the library, but no requester may see it yet (rating, architecture 8.3).
		return s.markImporting(ctx, w)
	}
	return s.markAvailable(ctx, w, itemID, nil)
}

// target is the number of episodes a season needs: its episodes, or for a running season the
// ones aired so far (architecture 8.1; series_catalog follows in stage 4, until then the title
// outside the library).
func target(t metadata.ExternalTitle, n int) (episodes int, airing bool) {
	for _, se := range t.Seasons {
		if se.Number == n {
			episodes = se.EpisodeCount
		}
	}
	if t.LastEpisode != nil && t.LastEpisode.Season == n && t.NextEpisode != nil && t.NextEpisode.Season == n && t.LastEpisode.Episode < episodes {
		return t.LastEpisode.Episode, true
	}
	return episodes, false
}

func (s *Service) reconcileSeries(ctx context.Context, w wishq.Wish, itemID string, scopes []visibility.Scope) error {
	if s.titles == nil {
		return errNoTitles
	}
	t, err := s.titles.EnsureExternalTitle(ctx, model.WishMediaShow, w.TmdbID)
	if errors.Is(err, model.ErrNotFound) || errors.Is(err, model.ErrTMDBUnreachable) || errors.Is(err, model.ErrProviderNotConfigured) {
		s.log.InfoContext(ctx, "season counts of a wished series unknown", "wish_id", w.ID, "err", err)
		return nil
	}
	if err != nil {
		return err
	}
	best := map[int]int{} // the most playable episodes per season among the requesters
	for _, sc := range scopes {
		got, err := s.lib.VisibleSeasons(ctx, sc, itemID)
		if errors.Is(err, model.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		for n, c := range got {
			best[n] = max(best[n], c)
		}
	}
	seasons, err := s.read.ListWishSeasons(ctx, w.ID)
	if err != nil {
		return fmt.Errorf("seasons: %w", err)
	}
	var complete []SeasonNotice
	for _, se := range seasons {
		n := int(se.SeasonNumber)
		eps, airing := target(t, n)
		if se.State == model.WishAvailable || se.State == model.WishWithdrawn || eps == 0 || best[n] < eps {
			continue
		}
		complete = append(complete, SeasonNotice{WishID: w.ID, SeriesID: itemID, Season: n, Episodes: eps, Airing: airing})
	}
	if len(complete) == 0 {
		return s.seasonsAgain(ctx, w, itemID, t, seasons)
	}
	return s.markAvailable(ctx, w, itemID, complete)
}

// seasonsAgain reports the complete seasons of a partial wish again while profiles wait.
func (s *Service) seasonsAgain(ctx context.Context, w wishq.Wish, itemID string, t metadata.ExternalTitle, seasons []wishq.WishSeason) error {
	if w.State != model.WishPartial || s.notifier == nil {
		return nil
	}
	for _, se := range seasons {
		if se.State != model.WishAvailable || se.NotifiedAt.Valid {
			continue
		}
		eps, airing := target(t, int(se.SeasonNumber))
		if _, err := s.notifier.WishSeasonAvailable(ctx, SeasonNotice{WishID: w.ID, SeriesID: itemID, Season: int(se.SeasonNumber), Episodes: eps, Airing: airing}); err != nil {
			s.log.WarnContext(ctx, "season of a wish not announced", "wish_id", w.ID, "season", se.SeasonNumber, "err", err)
		}
	}
	return nil
}

// markImporting keeps an open wish in processing with «wird eingelesen».
func (s *Service) markImporting(ctx context.Context, w wishq.Wish) error {
	if w.State != model.WishProcessing || w.StateDetail.String == model.WishDetailImporting {
		return nil
	}
	_, err := s.write.SetWishState(ctx, wishq.SetWishStateParams{State: w.State, StateDetail: nullString(model.WishDetailImporting), Now: s.now().UnixMilli(), ID: w.ID})
	return err
}

// markAvailable writes a film, or the complete seasons of a series, as available and announces it:
// the whole wish when every season is, else each new season.
func (s *Service) markAvailable(ctx context.Context, w wishq.Wish, itemID string, complete []SeasonNotice) error {
	state := model.WishAvailable
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		now := s.now().UnixMilli()
		if w.Media == model.WishMediaShow {
			for _, c := range complete {
				if _, err := q.SetWishSeasonState(ctx, wishq.SetWishSeasonStateParams{State: model.WishAvailable, AvailableAt: num64(now),
					WishID: w.ID, SeasonNumber: int64(c.Season)}); err != nil {
					return fmt.Errorf("season available: %w", err)
				}
			}
			seasons, err := q.ListWishSeasons(ctx, w.ID)
			if err != nil {
				return fmt.Errorf("seasons: %w", err)
			}
			var states []string
			for _, se := range seasons {
				if se.State != model.WishWithdrawn {
					states = append(states, se.State)
				}
			}
			state = showState(states)
		}
		if state == w.State {
			return s.event(ctx, q, w.ID, "season_available", map[string]any{"seasons": seasonNumbers(complete)})
		}
		p := wishq.SetWishStateParams{State: state, Now: now, ID: w.ID}
		switch state {
		case model.WishAvailable:
			p.AvailableAt, p.ClosedAt = num64(now), num64(now)
		case model.WishProcessing, model.WishPartial:
			p.StateDetail = w.StateDetail
		case model.WishFailed: // the other seasons failed before
			p.FailReason = w.FailReason
			if !p.FailReason.Valid {
				p.FailReason = nullString(model.WishFailArrFailed)
			}
			p.ClosedAt = num64(now)
		}
		if _, err := q.SetWishState(ctx, p); err != nil {
			return fmt.Errorf("wish state: %w", err)
		}
		return s.event(ctx, q, w.ID, "state_changed", map[string]any{"from": w.State, "to": state, "seasons": seasonNumbers(complete)})
	})
	if err != nil {
		return err
	}
	s.log.InfoContext(ctx, "wish found in the library", "wish_id", w.ID, "state", state, "seasons", len(complete))
	if state == model.WishAvailable {
		s.available(ctx, w.ID, itemID)
		return nil
	}
	if s.notifier != nil {
		for _, c := range complete {
			if _, err := s.notifier.WishSeasonAvailable(ctx, c); err != nil {
				s.log.WarnContext(ctx, "season of a wish not announced", "wish_id", w.ID, "season", c.Season, "err", err)
			}
		}
	}
	return nil
}

// available tells the Notifier that the wish is playable; profiles that cannot see it yet are
// reported again by the next reconcile.
func (s *Service) available(ctx context.Context, wishID int64, itemID string) {
	if s.notifier == nil {
		return
	}
	out, err := s.notifier.WishAvailable(ctx, wishID, itemID)
	if err != nil {
		s.log.WarnContext(ctx, "available wish not announced", "wish_id", wishID, "err", err)
		return
	}
	if len(out.Waiting) > 0 {
		s.log.InfoContext(ctx, "profiles wait for an available wish", "wish_id", wishID, "waiting", len(out.Waiting))
	}
}

func seasonNumbers(c []SeasonNotice) string {
	parts := make([]string, 0, len(c))
	for _, x := range c {
		parts = append(parts, fmt.Sprint(x.Season))
	}
	return strings.Join(parts, ",")
}
