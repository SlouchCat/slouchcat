package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Titles outside the library for a profile (concept 25 stage 2, data model 4.5; architecture 15,
// 16): the preview page with its wish card, its images and the search section «Noch nicht in der
// Bibliothek». A profile without the request right sees no missing title, except the ones it
// wished for itself (F-9); with «Anfordern aus der Suche» and «Anfordern auf Detailseiten» both
// off neither does a profile with the right. visibility decides the age (unknown counts as 18) and
// whether the library has the title; this file reads no title table.

const (
	searchMinRunes    = 2
	searchMaxRunes    = 100
	searchDetails     = 20              // hits whose details the search fetches (architecture 16)
	searchDetailsWait = 3 * time.Second // and how long it waits for them
	searchWorkers     = 4
	searchPageMax     = 50
	searchPageDefault = 20
)

// titleWishes is what the profile has wished of one title: its open wish and its newest closed
// one.
type titleWishes struct {
	open, last *wishq.ListProfileWishesRow
}

func (w titleWishes) any() bool { return w.open != nil || w.last != nil }

// profileTitleWishes returns the wishes of the profile per title ("media:tmdb"), newest first.
func (s *Service) profileTitleWishes(ctx context.Context, p model.Profile) (map[string]titleWishes, error) {
	rows, err := s.read.ListProfileWishes(ctx, wishq.ListProfileWishesParams{ProfileID: p.ID, LimitCount: listMax})
	if err != nil {
		return nil, fmt.Errorf("profile wishes: %w", err)
	}
	out := map[string]titleWishes{}
	for i := range rows {
		r := &rows[i]
		k := titleKey(r.Media, r.TmdbID)
		tw := out[k]
		switch {
		case isOpen(r.State) && tw.open == nil:
			tw.open = r
		case !isOpen(r.State) && tw.last == nil:
			tw.last = r
		}
		out[k] = tw
	}
	return out, nil
}

func titleKey(media string, tmdbID int64) string { return fmt.Sprintf("%s:%d", media, tmdbID) }

// titleAccess is the right of a profile on one title outside the library.
type titleAccess struct {
	rights model.WishRights
	sender Sender
	ws     model.WishSettings
	wishes titleWishes
}

// right is the request right visibility asks: the right for the medium, or an own wish.
func (a titleAccess) right() visibility.RequestRight {
	base := requestRight(a.rights)
	return func(media string) bool { return a.wishes.any() || base(media) }
}

// access reads the right of the profile on the title and answers model.ErrNotFound when the
// profile may see no missing title of it at all: no right and no own wish, or both places off.
func (s *Service) access(ctx context.Context, p model.Profile, media string, tmdbID int64) (titleAccess, error) {
	if (media != model.WishMediaMovie && media != model.WishMediaShow) || tmdbID < 1 {
		return titleAccess{}, model.ErrNotFound
	}
	if s.lib == nil {
		return titleAccess{}, errNoLibrary
	}
	if s.titles == nil {
		return titleAccess{}, errNoTitles
	}
	var a titleAccess
	var err error
	if a.rights, a.sender, err = s.SenderFor(ctx, p.AccountID, p.ID); err != nil {
		return titleAccess{}, err
	}
	if a.ws, err = s.cfg.WishSettings(ctx); err != nil {
		return titleAccess{}, err
	}
	all, err := s.profileTitleWishes(ctx, p)
	if err != nil {
		return titleAccess{}, err
	}
	a.wishes = all[titleKey(media, tmdbID)]
	if !a.wishes.any() && (!requestRight(a.rights)(media) || (!a.ws.FromSearch && !a.ws.FromDetail)) {
		return titleAccess{}, model.ErrNotFound
	}
	return a, nil
}

// match asks visibility where the title stands: a library title the profile sees is
// WishAlreadyAvailableError, one it may not see and a title it may not see as missing (age, right)
// model.ErrNotFound.
func (s *Service) match(ctx context.Context, p model.Profile, a titleAccess, ref visibility.ExternalRef) error {
	m, err := s.lib.MatchExternalTitles(ctx, visibility.Profile(p), a.right(), []visibility.ExternalRef{ref})
	if err != nil {
		return err
	}
	switch {
	case len(m) == 0:
		return model.ErrNotFound
	case m[0].InLibrary && m[0].ItemID != "":
		return &model.WishAlreadyAvailableError{ItemID: m[0].ItemID}
	case !m[0].Missing:
		return model.ErrNotFound
	}
	return nil
}

// ExternalTitle is the preview page of a title outside the library with its wish card
// (getExternalTitle). model.ErrNotFound: TMDB does not know it, the profile may not see it (age
// over its ceiling or unknown below 18, no right and no own wish, both places off, a library title
// it may not see); *model.WishAlreadyAvailableError for a library title it sees;
// model.ErrTMDBUnreachable, model.ErrProviderNotConfigured from the title lookup.
func (s *Service) ExternalTitle(ctx context.Context, p model.Profile, media string, tmdbID int64) (model.ExternalTitlePreview, error) {
	a, err := s.access(ctx, p, media, tmdbID)
	if err != nil {
		return model.ExternalTitlePreview{}, err
	}
	t, err := s.titles.EnsureExternalTitle(ctx, media, tmdbID)
	if err != nil {
		return model.ExternalTitlePreview{}, err
	}
	if err := s.match(ctx, p, a, refOf(t)); err != nil {
		return model.ExternalTitlePreview{}, err
	}
	out := model.ExternalTitlePreview{ExternalTitleCard: card(t, p)}
	text := titleText(t, p)
	out.OriginalTitle, out.Overview, out.Tagline = t.OriginalTitle, text.Overview, text.Tagline
	out.ReleaseDate, out.RuntimeMinutes, out.Status = t.ReleaseDate, t.RuntimeMinutes, t.Status
	out.Genres = genreNames(t.Genres, p)
	for _, tr := range t.Trailers {
		out.Trailers = append(out.Trailers, model.ExternalTrailerRef{Key: tr.Key, Lang: tr.Lang})
	}
	if e := t.NextEpisode; e != nil {
		out.NextEpisode = &model.ExternalEpisodeDate{Season: e.Season, Episode: e.Episode, AirDate: e.AirDate}
	}
	if w := a.wishes.open; w != nil {
		out.Wish = &model.WishMark{ID: w.ID, State: w.State, StateDetail: w.StateDetail.String}
	} else if w := a.wishes.last; w != nil {
		at := w.UpdatedAt
		if w.ClosedAt.Valid {
			at = w.ClosedAt.Int64
		}
		out.LastWish = &model.WishLast{ID: w.ID, State: w.State, At: time.UnixMilli(at).UTC(), Reason: w.FailReason.String}
	}
	other := false // another open wish of the household holds the title (or a season of it)
	if media == model.WishMediaShow {
		if out.Seasons, other, err = s.seasonStates(ctx, t, a.wishes.open); err != nil {
			return model.ExternalTitlePreview{}, err
		}
	} else if a.wishes.open == nil {
		switch _, err := s.read.GetOpenMovieWish(ctx, tmdbID); {
		case err == nil:
			other = true
		case !errors.Is(err, sql.ErrNoRows):
			return model.ExternalTitlePreview{}, fmt.Errorf("open film wish: %w", err)
		}
	}
	out.CanRequest, out.RequestReason = requestable(a, media, out.Seasons, other)
	out.Request = model.WishRequestCard{CanRequest: out.CanRequest, Reason: out.RequestReason}
	if a.rights.CanRequest {
		if q, err := s.quotaOf(ctx, s.read, p.AccountID, a.sender); err != nil {
			return model.ExternalTitlePreview{}, err
		} else if q != nil {
			out.Request.Quota = q
		}
		if u, err := s.read.GetSeerrUser(ctx, int64(a.sender.SeerrUserID)); err == nil {
			out.Request.AutoApprove = toSeerrUser(u).AutoApprove
		} else if !errors.Is(err, sql.ErrNoRows) {
			return model.ExternalTitlePreview{}, fmt.Errorf("seerr user: %w", err)
		}
	}
	return out, nil
}

// seasonStates lists the seasons of a missing series (season 0 left out) from the stored title,
// each requested when an open wish holds it, else missing. other reports a season held by a wish
// the profile is not part of. The dialog asks Seerr for its own list (getWishSeasons).
func (s *Service) seasonStates(ctx context.Context, t metadata.ExternalTitle, own *wishq.ListProfileWishesRow) ([]model.ExternalSeasonState, bool, error) {
	out := []model.ExternalSeasonState{}
	other := false
	for _, se := range t.Seasons {
		if se.Number < 1 || se.EpisodeCount < 1 {
			continue
		}
		st := model.ExternalSeasonState{Number: se.Number, EpisodeCount: se.EpisodeCount, AirDate: se.AirDate, State: model.ExternalSeasonMissing}
		id, err := s.read.GetOpenSeasonWish(ctx, wishq.GetOpenSeasonWishParams{TmdbID: t.TMDBID, SeasonNumber: int64(se.Number)})
		switch {
		case err == nil:
			st.State = model.ExternalSeasonRequested
			if own == nil || own.ID != id {
				other = true
			}
		case !errors.Is(err, sql.ErrNoRows):
			return nil, false, fmt.Errorf("open season: %w", err)
		}
		out = append(out, st)
	}
	return out, other, nil
}

// requestable says whether the profile can request (or co-request) the title now and why not.
func requestable(a titleAccess, media string, seasons []model.ExternalSeasonState, other bool) (bool, string) {
	switch {
	case !a.rights.CanRequest:
		return false, a.rights.Reason
	case media == model.WishMediaMovie && !a.rights.Movies:
		return false, model.WishReasonMoviesNotAllowed
	case media == model.WishMediaShow && !a.rights.Shows:
		return false, model.WishReasonShowsNotAllowed
	}
	statusOnly := a.ws.AlreadyRequested == model.WishesAlreadyStatus
	if media == model.WishMediaMovie {
		switch {
		case a.wishes.open != nil:
			return false, model.WishReasonAlreadyRequested
		case other && statusOnly:
			return false, model.WishReasonStatusOnly
		}
		return true, ""
	}
	if slices.ContainsFunc(seasons, func(se model.ExternalSeasonState) bool { return se.State == model.ExternalSeasonMissing }) {
		return true, ""
	}
	switch {
	case other && !statusOnly:
		return true, ""
	case other:
		return false, model.WishReasonStatusOnly
	}
	return false, model.WishReasonAlreadyRequested
}

// card is the list form of a stored title for the profile, without wish and right.
func card(t metadata.ExternalTitle, p model.Profile) model.ExternalTitleCard {
	return model.ExternalTitleCard{
		Media: t.Media, TMDBID: t.TMDBID, Title: titleName(t, p), Year: t.Year, MinAge: t.MinAge,
		ImageKinds: slices.Clone(t.ImageKinds), HasTrailer: len(t.Trailers) > 0,
	}
}

// titleText is the text in the profile's metadata language, else its interface language, else
// English.
func titleText(t metadata.ExternalTitle, p model.Profile) metadata.ExternalText {
	for _, lang := range []string{p.MetadataLanguage, p.Language, "en"} {
		if lang == "" {
			continue
		}
		if tx, ok := t.Texts[lang]; ok && tx.Title != "" {
			return tx
		}
		base, _, _ := strings.Cut(lang, "-")
		if tx, ok := t.Texts[base]; ok && tx.Title != "" {
			return tx
		}
	}
	return metadata.ExternalText{}
}

// genreNames are the genres in the profile's language, else English.
func genreNames(gs []metadata.ExternalGenre, p model.Profile) []string {
	out := make([]string, 0, len(gs))
	for _, g := range gs {
		name := ""
		for _, lang := range []string{p.MetadataLanguage, p.Language, "en"} {
			base, _, _ := strings.Cut(lang, "-")
			if n := g.Names[lang]; lang != "" && n != "" {
				name = n
			} else if n := g.Names[base]; base != "" && n != "" {
				name = n
			}
			if name != "" {
				break
			}
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

// ExternalTitleImage opens an image of a title outside the library for the profile
// (getExternalTitleImage): the same rules as ExternalTitle, but without asking TMDB for the title
// (an image follows the page). model.ErrNotFound when the profile may not see it, for a library
// title and a title not stored.
func (s *Service) ExternalTitleImage(ctx context.Context, p model.Profile, media string, tmdbID int64, kind, size string, preferWebP bool) (model.StreamSource, error) {
	a, err := s.access(ctx, p, media, tmdbID)
	if err != nil {
		return model.StreamSource{}, err
	}
	if err := s.match(ctx, p, a, visibility.ExternalRef{Media: media, TMDBID: tmdbID}); err != nil {
		if errors.As(err, new(*model.WishAlreadyAvailableError)) {
			return model.StreamSource{}, model.ErrNotFound // the library title has its own images
		}
		return model.StreamSource{}, err
	}
	return s.titles.OpenExternalTitleImage(ctx, media, tmdbID, kind, size, preferWebP)
}

// AdminExternalTitleImage opens an image of a stored title outside the library for the
// administration (getAdminExternalTitleImage), without a profile.
func (s *Service) AdminExternalTitleImage(ctx context.Context, media string, tmdbID int64, kind, size string, preferWebP bool) (model.StreamSource, error) {
	if s.titles == nil {
		return model.StreamSource{}, errNoTitles
	}
	return s.titles.OpenExternalTitleImage(ctx, media, tmdbID, kind, size, preferWebP)
}

// ItemSummary is a library title as the profile sees it (the ItemRef of already_available and of
// «Meine Wünsche»); model.ErrNotFound when it may not see it.
func (s *Service) ItemSummary(ctx context.Context, p model.Profile, itemID string) (model.ItemSummary, error) {
	if s.lib == nil {
		return model.ItemSummary{}, errNoLibrary
	}
	return s.lib.Summary(ctx, visibility.Profile(p), itemID)
}

// SearchExternalTitles is the search section «Noch nicht in der Bibliothek» (searchExternalTitles):
// the TMDB hits of q (media movie, show or "" for both) that the profile sees as missing titles.
// The details of the first 20 hits that are missing or whose age is unknown are fetched within
// 3 s, then visibility is asked again: a hit whose age stays unknown is missing for a profile below
// 18 (it counts as 18). Empty without the right or with «Anfordern aus der Suche» off.
// *model.ValidationError for q outside 2 to 100 characters or an unknown medium;
// model.ErrTMDBUnreachable, model.ErrTMDBKeyInvalid, model.ErrProviderNotConfigured from TMDB.
// searchPerMin is the limit of searchExternalTitles per profile and minute, at once as well
// (Vorschlag; data model 4.5 names a 429 without a number). The pot is the token bucket of the
// login attempts (auth.Limiter), keyed by the profile.
const searchPerMin = 30

func (s *Service) SearchExternalTitles(ctx context.Context, p model.Profile, q, media string, offset, limit int) (model.ExternalTitlePage, error) {
	q = strings.TrimSpace(q)
	verr := &model.ValidationError{}
	switch n := utf8.RuneCountInString(q); {
	case n < searchMinRunes:
		_ = verr.Add("q", "too_short")
	case n > searchMaxRunes:
		_ = verr.Add("q", "too_long")
	}
	if media != "" && media != model.WishMediaMovie && media != model.WishMediaShow {
		_ = verr.Add("media", "invalid_value")
	}
	if len(verr.Fields) > 0 {
		return model.ExternalTitlePage{}, verr
	}
	out := model.ExternalTitlePage{Items: []model.ExternalTitleCard{}}
	if s.lib == nil || s.titles == nil {
		return out, nil
	}
	rights, err := s.ForProfile(ctx, p.AccountID, p.ID)
	if err != nil {
		return model.ExternalTitlePage{}, err
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return model.ExternalTitlePage{}, err
	}
	right := requestRight(rights)
	if !ws.FromSearch || !right(model.WishMediaMovie) && !right(model.WishMediaShow) {
		return out, nil
	}
	if err := s.searchLimit.Allow(strconv.FormatInt(p.ID, 10), s.now()); err != nil {
		return model.ExternalTitlePage{}, err // model.ErrTooManyAttempts: every search may ask TMDB
	}
	hits, err := s.titles.SearchExternalTitles(ctx, media, q)
	if err != nil {
		return model.ExternalTitlePage{}, err
	}
	refs := make([]visibility.ExternalRef, 0, len(hits))
	for _, h := range hits {
		refs = append(refs, visibility.ExternalRef{Media: h.Media, TMDBID: h.TMDBID})
	}
	scope := visibility.Profile(p)
	matches, err := s.lib.MatchExternalTitles(ctx, scope, right, refs)
	if err != nil {
		return model.ExternalTitlePage{}, err
	}
	var fetch []visibility.ExternalRef
	for _, m := range matches {
		if (m.Missing || m.NeedsDetails) && len(fetch) < searchDetails {
			fetch = append(fetch, m.Ref)
		}
	}
	stored := s.fetchSearchDetails(ctx, fetch)
	if len(fetch) > 0 {
		if matches, err = s.lib.MatchExternalTitles(ctx, scope, right, refs); err != nil {
			return model.ExternalTitlePage{}, err
		}
	}
	own, err := s.profileTitleWishes(ctx, p)
	if err != nil {
		return model.ExternalTitlePage{}, err
	}
	statusOnly := ws.AlreadyRequested == model.WishesAlreadyStatus
	for i, m := range matches {
		if !m.Missing {
			continue
		}
		h := hits[i]
		c := model.ExternalTitleCard{Media: h.Media, TMDBID: h.TMDBID, Title: h.Title, Year: h.Year}
		if t, ok := stored[titleKey(h.Media, h.TMDBID)]; ok {
			c = card(t, p)
		}
		c.CanRequest = true
		if w := own[titleKey(h.Media, h.TMDBID)].open; w != nil {
			c.Wish = &model.WishMark{ID: w.ID, State: w.State, StateDetail: w.StateDetail.String}
			c.CanRequest, c.RequestReason = false, model.WishReasonAlreadyRequested
		} else if h.Media == model.WishMediaMovie && statusOnly {
			if _, err := s.read.GetOpenMovieWish(ctx, h.TMDBID); err == nil {
				c.CanRequest, c.RequestReason = false, model.WishReasonStatusOnly
			} else if !errors.Is(err, sql.ErrNoRows) {
				return model.ExternalTitlePage{}, fmt.Errorf("open film wish: %w", err)
			}
		}
		out.Items = append(out.Items, c)
	}
	out.Total = len(out.Items)
	if limit <= 0 {
		limit = searchPageDefault
	}
	out.Items = page(out.Items, max(0, offset), min(limit, searchPageMax))
	return out, nil
}

// fetchSearchDetails makes sure of the stored titles of refs within searchDetailsWait, a few at a time,
// and returns those it has. A title TMDB does not answer for in time stays unknown.
func (s *Service) fetchSearchDetails(ctx context.Context, refs []visibility.ExternalRef) map[string]metadata.ExternalTitle {
	out := map[string]metadata.ExternalTitle{}
	if len(refs) == 0 {
		return out
	}
	ctx, cancel := context.WithTimeout(ctx, searchDetailsWait)
	defer cancel()
	var mu sync.Mutex
	var wg sync.WaitGroup
	next := make(chan visibility.ExternalRef)
	for range min(searchWorkers, len(refs)) {
		wg.Go(func() {
			for ref := range next {
				t, err := s.titles.EnsureExternalTitle(ctx, ref.Media, ref.TMDBID)
				if err != nil {
					if ctx.Err() == nil && !errors.Is(err, model.ErrNotFound) {
						s.log.InfoContext(ctx, "details of a search hit not fetched", "media", ref.Media, "tmdb_id", ref.TMDBID, "reason", errorClass(err))
					}
					continue
				}
				mu.Lock()
				out[titleKey(ref.Media, ref.TMDBID)] = t
				mu.Unlock()
			}
		})
	}
	for _, ref := range refs {
		select {
		case next <- ref:
		case <-ctx.Done():
		}
	}
	close(next)
	wg.Wait()
	return out
}

// withCards sets the card of the title outside the library on wishes of the profile (stored titles
// only; TMDB is not asked).
func (s *Service) withCards(ctx context.Context, p model.Profile, ws []model.Wish) error {
	if s.titles == nil || len(ws) == 0 {
		return nil
	}
	rights, err := s.ForProfile(ctx, p.AccountID, p.ID)
	if err != nil {
		return err
	}
	for i := range ws {
		w := &ws[i]
		t, err := s.titles.ExternalTitle(ctx, w.Media, w.TMDBID)
		if errors.Is(err, model.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		c := card(t, p)
		if isOpen(w.State) {
			c.Wish = &model.WishMark{ID: w.ID, State: w.State, StateDetail: w.StateDetail}
			c.RequestReason = model.WishReasonAlreadyRequested
		} else {
			c.CanRequest = w.CanRequestAgain && requestRight(rights)(w.Media)
		}
		w.External = &c
	}
	return nil
}
