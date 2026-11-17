package wishes

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/metadata"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// The wishes area «Entdecken», «Filme», «Serien» and «Suche» (concept 25 stage 5, architecture 21;
// N-50 to N-58). The server reads the discover and search routes of Seerr on behalf of the Seerr
// user the profile sends as (N-54); the client never talks to Seerr or TMDB.
//
//   - Who sees it: a profile with the request right while «Seiten Entdecken, Filme, Serien und
//     Suche» is on (F-9, N-56, N-57). Everyone else gets model.ErrNotFound (404).
//   - Cache: one hour per list, parameters, page, language and age step in wish_discover_pages
//     (N-54); the search 10 minutes in memory. While Seerr does not answer, an older row is
//     served with Stale set; without one the call fails with ErrDiscoverUnreachable.
//   - Age: for a profile below 18 Seerr is asked with certificationLte of the rating country (ADR
//     0034; DE for CH and AT, N-118) where the route knows it, a pre-filter only. Every hit is
//     checked again through visibility with the age in external_titles (unknown counts as 18).
//   - Details: every hit is looked up in metadata; hits without a stored row are fetched, for every
//     profile, at most 20 per page and within 3 s (architecture 16). Title, year and image kinds
//     come from Slouchcat's own data in the profile's language, Seerr's text is the fallback
//     (N-116: the film and series lists of Seerr answer in the user's locale, appendix I, I-61).
//   - Seerr's quirks (appendix I section 4a): no language on /discover/movies and /discover/tv
//     (it filters the original language there, N-115), sort orders with a date and vote floor
//     (N-117), at most 500 pages (I-67).
//   - Library: the match of Slouchcat through visibility (series TVDB first), never mediaInfo of
//     Seerr (N-55). A library title the profile sees carries its item id; one it may not see is
//     left out (404 rule). mediaInfo only says «in Seerr angefragt».

// SettingDiscoverPages is the switch «Seiten Entdecken, Filme, Serien und Suche» (N-57, data model
// 2.12): 0/1, default 1. The administration's wish settings write it; it is read here with its
// default like the other switches of stage 4.
const SettingDiscoverPages = "wishes.discover_pages"

// JobDiscover is the job type that warms the lists every hour.
const JobDiscover = "wishes.discover"

// The rows of «Entdecken», fixed (F-28, N-51), in their order (DiscoverRow.key of the contract).
const (
	RowTrending       = "trending"
	RowPopularMovies  = "popular_movies"
	RowMovieGenres    = "movie_genres"
	RowUpcomingMovies = "upcoming_movies"
	RowPopularShows   = "popular_shows"
	RowShowGenres     = "show_genres"
	RowUpcomingShows  = "upcoming_shows"
)

// DiscoverRows are the seven rows of «Entdecken» in their order.
var DiscoverRows = []string{RowTrending, RowPopularMovies, RowMovieGenres, RowUpcomingMovies, RowPopularShows, RowShowGenres, RowUpcomingShows}

// The sort orders of «Filme» and «Serien» (N-52 as amended by N-117: «Titel A-Z» is dropped,
// Seerr serves no usable title order, appendix I, I-63).
const (
	SortPopularity = "popularity"
	SortNewest     = "newest"
	SortRating     = "rating" // «Am besten bewertet»
)

// The search text of searchWishTitles: 2 to 100 characters (data model 4.5).
const (
	SearchMinLength = 2
	SearchMaxLength = 100
)

// ErrDiscoverUnreachable: Seerr did not answer and no older list is cached (503 seerr_unreachable).
var ErrDiscoverUnreachable = errors.New("wishes: seerr unreachable and nothing cached")

const (
	discoverTTL       = time.Hour        // a list is read again after an hour (N-54)
	discoverKeep      = 24 * time.Hour   // wishes.discover deletes older rows (data model 2.15)
	discoverWait      = 5 * time.Second  // a page waits so long for Seerr (architecture 21.2)
	detailWait        = 3 * time.Second  // and so long for the details of its hits (architecture 16)
	detailMax         = 20               // details fetched per page at most (architecture 16, AK-190)
	detailParallel    = 4                // details fetched at once (under the TMDB limiter)
	searchTTL         = 10 * time.Minute // the search in memory (architecture 21.2)
	searchMaxEntries  = 500
	discoverEvery     = time.Hour
	discoverPriority  = 4
	discoverRunKey    = JobDiscover
	discoverMaxPage   = 500 // TMDB serves no page above 500
	ratingCountryBase = model.DefaultRatingCountry
)

// DiscoverWish is the own open wish of the profile for a title (the mark «angefordert»).
type DiscoverWish struct {
	ID    int64
	State string
}

// DiscoverEntry is one tile (DiscoverEntry of the contract).
type DiscoverEntry struct {
	Media  string // model.WishMediaMovie or model.WishMediaShow
	TMDBID int64
	// Title and Year come from Slouchcat's own data in the profile's language when it knows the
	// title, else from Seerr (N-116: Seerr's display language of the film and series lists is the
	// user's locale, I-61).
	Title string
	Year  int // 0 = unknown
	// ImageKinds are the kinds Slouchcat has stored for the title (poster, backdrop, logo): its
	// own image route serves them. Empty while Slouchcat does not know the title (N-116).
	ImageKinds []string
	MinAge     *int   // the stored age, nil when unknown (only titles outside the library)
	ItemID     string // the library title the profile sees: mark «In der Bibliothek», ItemRef
	// SeerrRequested: Seerr knows a request for the title (mediaInfo pending, processing or
	// partially available); never «in der Bibliothek» (N-55).
	SeerrRequested bool
	Wish           *DiscoverWish // the profile's own open wish (owner or joined)
	CanRequest     bool          // missing, the profile may request the medium, no own open wish

	posterPath, backdropPath string // TMDB paths as Seerr gave them; internal only
}

// DiscoverGenre is one genre tile of the rows «Film-Genres» and «Serien-Genres», or one genre of
// the filter of «Filme» and «Serien».
type DiscoverGenre struct {
	ID       int
	Name     string
	Backdrop string // TMDB path of a backdrop of the genre; "" for the filter list
}

// DiscoverPage is one page of a list (DiscoverPage of the contract).
type DiscoverPage struct {
	Entries    []DiscoverEntry
	Page       int
	TotalPages int       // 0 = unknown
	Stale      bool      // older than an hour: Seerr did not answer
	FetchedAt  time.Time // the read from Seerr
}

// DiscoverRow is one row of «Entdecken» with its first page (DiscoverRow of the contract): the
// title rows carry Entries, the genre rows Genres.
type DiscoverRow struct {
	Key       string
	Entries   []DiscoverEntry
	Genres    []DiscoverGenre
	NextPage  int // 0 = none
	Stale     bool
	FetchedAt time.Time // zero when nothing could be read
}

// TitlesQuery is the query of «Filme» and «Serien» (listWishTitles).
type TitlesQuery struct {
	Media         string // model.WishMediaMovie or model.WishMediaShow
	Sort          string // SortPopularity (default), SortNewest, SortRating
	Genre         int    // TMDB genre id, 0 = all
	HideAvailable bool   // «Vorhandene ausblenden»: no library titles
	Page          int    // 1 based, 0 = 1
}

// DiscoverPagesOn reports whether the pages of the wishes area are switched on.
func (s *Service) DiscoverPagesOn(ctx context.Context) (bool, error) {
	return s.boolState(ctx, SettingDiscoverPages, true)
}

// DiscoverVisible reports whether the profile sees the pages of the wishes area
// (SessionInfo.wishes.discover): the switch is on and the profile may request.
func (s *Service) DiscoverVisible(ctx context.Context, p model.Profile) (bool, error) {
	_, err := s.discoverAccess(ctx, p)
	if errors.Is(err, model.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// Discover returns the seven rows of «Entdecken» with their first page (getWishDiscover).
// model.ErrNotFound without the right or with the pages off; ErrDiscoverUnreachable when no row
// could be read at all.
func (s *Service) Discover(ctx context.Context, p model.Profile) ([]DiscoverRow, error) {
	a, err := s.discoverAccess(ctx, p)
	if err != nil {
		return nil, err
	}
	budget := &detailBudget{left: detailMax}
	rows := make([]DiscoverRow, 0, len(DiscoverRows))
	read := 0
	for _, key := range DiscoverRows {
		row, err := s.discoverRow(ctx, a, key, budget)
		switch {
		case errors.Is(err, ErrDiscoverUnreachable):
			rows = append(rows, DiscoverRow{Key: key, Entries: []DiscoverEntry{}, Genres: []DiscoverGenre{}, Stale: true})
			continue
		case err != nil:
			return nil, err
		}
		read++
		rows = append(rows, row)
	}
	if read == 0 {
		return nil, ErrDiscoverUnreachable
	}
	return rows, nil
}

func (s *Service) discoverRow(ctx context.Context, a discoverAccess, key string, budget *detailBudget) (DiscoverRow, error) {
	row := DiscoverRow{Key: key, Entries: []DiscoverEntry{}, Genres: []DiscoverGenre{}}
	if key == RowMovieGenres || key == RowShowGenres {
		g, err := s.genreTiles(ctx, a, key == RowShowGenres)
		if err != nil {
			return row, err
		}
		row.Genres, row.Stale, row.FetchedAt = g.genres, g.stale, g.at
		return row, nil
	}
	page, err := s.listPage(ctx, a, listSpec{row: key}, 1, false, budget)
	if err != nil {
		return row, err
	}
	row.Entries, row.Stale, row.FetchedAt = page.Entries, page.Stale, page.FetchedAt
	if page.TotalPages > 1 {
		row.NextPage = 2
	}
	return row, nil
}

// DiscoverRowPage returns one page of a row of «Entdecken» (listWishDiscoverRow). genre (a TMDB
// genre id) is required on the genre rows and allowed on the rows of popular films and series; it
// narrows them. model.ErrNotFound for an unknown row, without the right or with the pages off;
// *model.ValidationError for a genre where the row takes none, a missing genre and a page out of
// range; ErrDiscoverUnreachable.
func (s *Service) DiscoverRowPage(ctx context.Context, p model.Profile, row string, page, genre int) (DiscoverPage, error) {
	if !knownRow(row) {
		if _, err := s.discoverAccess(ctx, p); err != nil {
			return DiscoverPage{}, err
		}
		return DiscoverPage{}, model.ErrNotFound
	}
	a, err := s.discoverAccess(ctx, p)
	if err != nil {
		return DiscoverPage{}, err
	}
	verr := checkPage(page)
	genreRow := row == RowMovieGenres || row == RowShowGenres
	switch {
	case genre < 0:
		verr = verr.Add("genre", "out_of_range")
	case genreRow && genre == 0:
		verr = verr.Add("genre", "required")
	case genre != 0 && !genreRow && row != RowPopularMovies && row != RowPopularShows:
		verr = verr.Add("genre", "not_allowed")
	}
	if verr != nil {
		return DiscoverPage{}, verr
	}
	spec := listSpec{row: row, genre: genre}
	return s.listPage(ctx, a, spec, max(page, 1), false, &detailBudget{left: detailMax})
}

// DiscoverTitles returns a page of «Filme» or «Serien» (listWishTitles). model.ErrNotFound for an
// unknown medium, without the right or with the pages off; *model.ValidationError for an unknown
// sort, a negative genre and a page out of range; ErrDiscoverUnreachable.
func (s *Service) DiscoverTitles(ctx context.Context, p model.Profile, q TitlesQuery) (DiscoverPage, error) {
	a, err := s.discoverAccess(ctx, p)
	if err != nil {
		return DiscoverPage{}, err
	}
	if q.Media != model.WishMediaMovie && q.Media != model.WishMediaShow {
		return DiscoverPage{}, model.ErrNotFound
	}
	if q.Sort == "" {
		q.Sort = SortPopularity
	}
	verr := checkPage(q.Page)
	if q.Sort != SortPopularity && q.Sort != SortNewest && q.Sort != SortRating {
		verr = verr.Add("sort", "invalid_value")
	}
	if q.Genre < 0 {
		verr = verr.Add("genre", "out_of_range")
	}
	if verr != nil {
		return DiscoverPage{}, verr
	}
	spec := listSpec{titles: q.Media, sort: q.Sort, genre: q.Genre}
	return s.listPage(ctx, a, spec, max(q.Page, 1), q.HideAvailable, &detailBudget{left: detailMax})
}

// DiscoverGenres returns the genres of films or series for the filter of «Filme» and «Serien»
// (GET /genres/movie|tv, cached like a list). Errors as DiscoverTitles.
func (s *Service) DiscoverGenres(ctx context.Context, p model.Profile, media string) ([]DiscoverGenre, error) {
	a, err := s.discoverAccess(ctx, p)
	if err != nil {
		return nil, err
	}
	if media != model.WishMediaMovie && media != model.WishMediaShow {
		return nil, model.ErrNotFound
	}
	key := cacheKey("genre_list", "media="+media, a.language)
	hits, err := s.cached(ctx, a, key, func(sess seerr.Session, ctx context.Context) (cachedList, error) {
		var list []seerr.Genre
		var err error
		if media == model.WishMediaShow {
			list, err = sess.TVGenreList(ctx, a.language)
		} else {
			list, err = sess.MovieGenreList(ctx, a.language)
		}
		out := cachedList{}
		for _, g := range list {
			out.genres = append(out.genres, cachedGenre{ID: g.ID, Name: g.Name})
		}
		return out, err
	})
	if err != nil {
		return nil, err
	}
	return toGenres(hits.genres), nil
}

// SearchWishTitles searches over Seerr (searchWishTitles; F-11, N-53): any text TMDB knows, or a
// title by id with tmdb:, tvdb:, imdb: (case-insensitive, a blank after the colon allowed).
// Persons never appear. *model.ValidationError for a text outside 2 to 100 characters and a page
// out of range; model.ErrNotFound without the right or with the pages off; ErrDiscoverUnreachable.
// The raw hits stay 10 minutes in memory per text, page, language and age step; the stale answer
// of the lists does not exist here.
func (s *Service) SearchWishTitles(ctx context.Context, p model.Profile, q string, page int) (DiscoverPage, error) {
	a, err := s.discoverAccess(ctx, p)
	if err != nil {
		return DiscoverPage{}, err
	}
	q = strings.TrimSpace(q)
	verr := checkPage(page)
	if n := utf8.RuneCountInString(q); n < SearchMinLength || n > SearchMaxLength {
		verr = verr.Add("q", "invalid_length")
	}
	if verr != nil {
		return DiscoverPage{}, verr
	}
	page = max(page, 1)
	text := strings.ToLower(q)
	prefix, id, byID := seerr.ParseIDSearch(q)
	if byID {
		text = string(prefix) + ":" + id
	}
	key := cacheKey("search", "q="+text, "p="+strconv.Itoa(page), a.language, strconv.Itoa(a.maxAge))
	list, at, ok := s.discover.get(key, s.now())
	if !ok {
		sess, err := s.UserSession(ctx, a.sender.SeerrUserID)
		if err != nil {
			return DiscoverPage{}, discoverFailed(err)
		}
		cctx, cancel := context.WithTimeout(ctx, discoverWait)
		defer cancel()
		var res seerr.DiscoverPage
		if byID {
			res, err = sess.SearchByID(cctx, prefix, id, "", seerr.PageQuery{Page: page, Language: a.language})
		} else {
			res, err = sess.Search(cctx, seerr.SearchQuery{PageQuery: seerr.PageQuery{Page: page, Language: a.language}, Query: q})
		}
		if err != nil {
			return DiscoverPage{}, discoverFailed(err)
		}
		list, at = hitsOf(res), time.UnixMilli(s.now().UnixMilli())
		s.discover.put(key, list, at)
	}
	return s.entries(ctx, a, list, page, false, at, false, &detailBudget{left: detailMax})
}

// discoverAccess is what a page needs of the profile.
type discoverAccess struct {
	profile  model.Profile
	rights   model.WishRights
	sender   Sender
	maxAge   int    // min(profile, account): the age step of the cache key
	language string // the metadata language Seerr answers in
}

// discoverAccess checks the switch and the right; model.ErrNotFound for every refusal.
func (s *Service) discoverAccess(ctx context.Context, p model.Profile) (discoverAccess, error) {
	on, err := s.DiscoverPagesOn(ctx)
	if err != nil {
		return discoverAccess{}, err
	}
	if !on {
		return discoverAccess{}, model.ErrNotFound
	}
	rights, sender, err := s.SenderFor(ctx, p.AccountID, p.ID)
	if err != nil {
		return discoverAccess{}, err
	}
	if !rights.CanRequest || sender.SeerrUserID < 1 {
		return discoverAccess{}, model.ErrNotFound
	}
	age, err := s.read.GetProfileAgeCeiling(ctx, wishq.GetProfileAgeCeilingParams{ProfileID: p.ID, AccountID: p.AccountID})
	if errors.Is(err, sql.ErrNoRows) {
		return discoverAccess{}, model.ErrNotFound
	}
	if err != nil {
		return discoverAccess{}, fmt.Errorf("age ceiling: %w", err)
	}
	lang := p.MetadataLanguage
	if lang == "" {
		lang = model.MetadataBase(p.Language)
	}
	return discoverAccess{profile: p, rights: rights, sender: sender, maxAge: int(min(max(age, 0), model.MaxRatingAge)), language: lang}, nil
}

func knownRow(row string) bool {
	for _, r := range DiscoverRows {
		if r == row {
			return true
		}
	}
	return false
}

func checkPage(page int) *model.ValidationError {
	if page < 0 || page > discoverMaxPage {
		return (*model.ValidationError)(nil).Add("page", "out_of_range")
	}
	return nil
}

// listSpec names a title list: a row of «Entdecken» or the grid of a medium.
type listSpec struct {
	row    string // a row key, or "" for the grid
	titles string // the grid: model.WishMediaMovie or model.WishMediaShow
	sort   string
	genre  int
}

func (l listSpec) key(page int, a discoverAccess) string {
	g := ""
	if l.genre > 0 {
		g = strconv.Itoa(l.genre)
	}
	if l.row != "" {
		return cacheKey(l.row, "genre="+g, "p="+strconv.Itoa(page), a.language, strconv.Itoa(a.maxAge))
	}
	name := "movies"
	if l.titles == model.WishMediaShow {
		name = "shows"
	}
	return cacheKey(name, "sort="+l.sort, "genre="+g, "p="+strconv.Itoa(page), a.language, strconv.Itoa(a.maxAge))
}

func cacheKey(parts ...string) string { return strings.Join(parts, "|") }

// fetch reads the list from Seerr.
// /discover/movies and /discover/tv get no language (N-115: there it filters the original
// language); the other routes get it as the display language (I-62).
func (l listSpec) fetch(ctx context.Context, sess seerr.Session, page int, a discoverAccess, country, today string) (seerr.DiscoverPage, error) {
	pq := seerr.PageQuery{Page: page, Language: a.language}
	discoverQuery := func(tv bool, sortBy string) seerr.DiscoverQuery {
		q := seerr.DiscoverQuery{Page: page, SortBy: sortBy}
		if l.genre > 0 {
			q.Genre = strconv.Itoa(l.genre)
		}
		if a.maxAge < model.MaxRatingAge {
			q.CertificationLte, q.CertificationCountry = certificationFor(country, a.maxAge, tv)
		}
		return q
	}
	switch l.row {
	case RowTrending:
		return sess.Trending(ctx, seerr.TrendingQuery{PageQuery: pq, MediaType: "all"})
	case RowPopularMovies, RowMovieGenres:
		return sess.DiscoverMovies(ctx, discoverQuery(false, "popularity.desc"))
	case RowPopularShows, RowShowGenres:
		return sess.DiscoverTV(ctx, discoverQuery(true, "popularity.desc"))
	case RowUpcomingMovies:
		return sess.UpcomingMovies(ctx, pq)
	case RowUpcomingShows:
		return sess.UpcomingTV(ctx, pq)
	}
	tv := l.titles == model.WishMediaShow
	o := gridSorts[l.sort][0]
	if tv {
		o = gridSorts[l.sort][1]
	}
	q := discoverQuery(tv, o.sortBy)
	q.VoteCountGte = o.votes
	if o.released {
		q.ReleasedBefore = today
	}
	if tv {
		return sess.DiscoverTV(ctx, q)
	}
	return sess.DiscoverMovies(ctx, q)
}

// gridSort is how a sort order of the grids goes to Seerr, for films [0] and series [1] (N-117).
type gridSort struct {
	sortBy   string
	votes    int  // voteCountGte, 0 = none
	released bool // only released until today (primaryReleaseDateLte resp. firstAirDateLte)
}

var gridSorts = map[string][2]gridSort{
	SortPopularity: {{sortBy: "popularity.desc"}, {sortBy: "popularity.desc"}},
	// Without the date the list starts with titles of the next years, without the vote floor with
	// unknown ones (I-64).
	SortNewest: {{"primary_release_date.desc", 50, true}, {"first_air_date.desc", 20, true}},
	// Without the floor titles with a handful of votes lead (I-65).
	SortRating: {{sortBy: "vote_average.desc", votes: 500}, {sortBy: "vote_average.desc", votes: 200}},
}

// certificationFor is the pre-filter of an age ceiling: the certificate of the rating country,
// else of Germany (ADR 0034, F-12). For CH and AT it is Germany's (N-118): TMDB knows too few
// certificates of those two, the pre-filter would empty the lists (I-66).
func certificationFor(country string, age int, tv bool) (lte, cc string) {
	if country == "CH" || country == "AT" {
		country = ratingCountryBase
	}
	for _, c := range []string{country, ratingCountryBase} {
		if cert, ok := metadata.CertificationAtMost(c, age, tv); ok {
			return cert, c
		}
	}
	return "", ""
}

// listPage reads one page of a title list through the cache and builds its entries.
func (s *Service) listPage(ctx context.Context, a discoverAccess, l listSpec, page int, hideAvailable bool, budget *detailBudget) (DiscoverPage, error) {
	country, err := s.state(ctx, model.SettingRatingCountry)
	if err != nil {
		return DiscoverPage{}, err
	}
	if !model.IsRatingCountry(country) {
		country = ratingCountryBase
	}
	list, err := s.cached(ctx, a, l.key(page, a), func(sess seerr.Session, ctx context.Context) (cachedList, error) {
		res, err := l.fetch(ctx, sess, page, a, country, s.now().Format(time.DateOnly))
		return cachedList{hits: hitsOf(res), totalPages: res.TotalPages}, err
	})
	if err != nil {
		return DiscoverPage{}, err
	}
	out, err := s.entries(ctx, a, list.hits, page, hideAvailable, list.at, list.stale, budget)
	out.TotalPages = min(list.totalPages, discoverMaxPage)
	return out, err
}

// cachedHit is one hit as wish_discover_pages keeps it (data model 2.15).
type cachedHit struct {
	Media        string `json:"media"` // movie, show
	TMDBID       int64  `json:"tmdbId"`
	Title        string `json:"title"`
	Year         int    `json:"year,omitempty"`
	PosterPath   string `json:"posterPath,omitempty"`
	BackdropPath string `json:"backdropPath,omitempty"`
	SeerrStatus  int    `json:"seerrStatus,omitempty"` // mediaInfo.status, only for «in Seerr angefragt»
}

// cachedGenre is one genre of a genre list in wish_discover_pages.
type cachedGenre struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Backdrop string `json:"backdrop,omitempty"`
}

// cachedList is a row of wish_discover_pages read back.
type cachedList struct {
	hits       []cachedHit
	genres     []cachedGenre
	totalPages int
	at         time.Time
	stale      bool
}

// hitsOf keeps the films and series of a Seerr page (never a person) in Seerr's order.
func hitsOf(p seerr.DiscoverPage) []cachedHit {
	out := make([]cachedHit, 0, len(p.Results))
	for _, t := range p.Results {
		media := ""
		switch t.MediaType {
		case seerr.MediaTypeMovie:
			media = model.WishMediaMovie
		case seerr.MediaTypeTV:
			media = model.WishMediaShow
		}
		if media == "" || t.ID < 1 {
			continue
		}
		h := cachedHit{Media: media, TMDBID: int64(t.ID), Title: t.DisplayName(), Year: t.Year(), PosterPath: t.PosterPath, BackdropPath: t.BackdropPath}
		if t.MediaInfo != nil {
			h.SeerrStatus = int(t.MediaInfo.Status)
		}
		out = append(out, h)
	}
	return out
}

// cached returns the row of key when it is younger than an hour, else reads it from Seerr with
// fetch and stores it; while Seerr fails an older row is returned stale.
func (s *Service) cached(ctx context.Context, a discoverAccess, key string, fetch func(seerr.Session, context.Context) (cachedList, error)) (cachedList, error) {
	row, err := s.read.GetWishDiscoverPage(ctx, key)
	have := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return cachedList{}, fmt.Errorf("discover cache: %w", err)
	}
	now := time.UnixMilli(s.now().UnixMilli()) // the precision of fetched_at
	if have && now.Sub(time.UnixMilli(row.FetchedAt)) < discoverTTL {
		return decodeCached(row, false)
	}
	fresh, err := func() (cachedList, error) {
		sess, err := s.UserSession(ctx, a.sender.SeerrUserID)
		if err != nil {
			return cachedList{}, err
		}
		cctx, cancel := context.WithTimeout(ctx, discoverWait)
		defer cancel()
		return fetch(sess, cctx)
	}()
	if err != nil {
		if ctx.Err() != nil {
			return cachedList{}, ctx.Err()
		}
		if have {
			s.log.InfoContext(ctx, "discover list stale, seerr failed", "list", strings.SplitN(key, "|", 2)[0], "err", errorClass(err))
			return decodeCached(row, true)
		}
		return cachedList{}, discoverFailed(err)
	}
	var items any = fresh.hits
	if fresh.hits == nil {
		items = fresh.genres
	}
	if fresh.hits == nil && fresh.genres == nil {
		items = []cachedHit{}
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return cachedList{}, fmt.Errorf("discover cache: %w", err)
	}
	var total sql.NullInt64
	if fresh.totalPages > 0 {
		total = sql.NullInt64{Int64: int64(fresh.totalPages), Valid: true}
	}
	if err := s.write.UpsertWishDiscoverPage(ctx, wishq.UpsertWishDiscoverPageParams{
		CacheKey: key, Items: string(raw), TotalPages: total, FetchedAt: now.UnixMilli(),
	}); err != nil {
		return cachedList{}, fmt.Errorf("discover cache: %w", err)
	}
	fresh.at = now
	return fresh, nil
}

func decodeCached(row wishq.WishDiscoverPage, stale bool) (cachedList, error) {
	out := cachedList{totalPages: int(row.TotalPages.Int64), at: time.UnixMilli(row.FetchedAt), stale: stale}
	// A genre list and a title list share the table; the key says which, the fields tell it too.
	var probe []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(row.Items), &probe); err != nil {
		return cachedList{}, fmt.Errorf("discover cache %s: %w", row.CacheKey, err)
	}
	if len(probe) > 0 {
		if _, isTitle := probe[0]["tmdbId"]; !isTitle {
			if err := json.Unmarshal([]byte(row.Items), &out.genres); err != nil {
				return cachedList{}, fmt.Errorf("discover cache %s: %w", row.CacheKey, err)
			}
			return out, nil
		}
	}
	if err := json.Unmarshal([]byte(row.Items), &out.hits); err != nil {
		return cachedList{}, fmt.Errorf("discover cache %s: %w", row.CacheKey, err)
	}
	return out, nil
}

// discoverFailed turns a failed read from Seerr without a cached row into ErrDiscoverUnreachable.
func discoverFailed(err error) error {
	return fmt.Errorf("%w: %s", ErrDiscoverUnreachable, errorClass(err))
}

// genreTiles reads the genre slider of films or series.
type genreTiles struct {
	genres []DiscoverGenre
	stale  bool
	at     time.Time
}

func (s *Service) genreTiles(ctx context.Context, a discoverAccess, tv bool) (genreTiles, error) {
	key := cacheKey(RowMovieGenres, a.language, strconv.Itoa(a.maxAge))
	if tv {
		key = cacheKey(RowShowGenres, a.language, strconv.Itoa(a.maxAge))
	}
	list, err := s.cached(ctx, a, key, func(sess seerr.Session, ctx context.Context) (cachedList, error) {
		var slides []seerr.GenreSlide
		var err error
		if tv {
			slides, err = sess.TVGenres(ctx, a.language)
		} else {
			slides, err = sess.MovieGenres(ctx, a.language)
		}
		out := cachedList{genres: []cachedGenre{}}
		for _, g := range slides {
			c := cachedGenre{ID: g.ID, Name: g.Name}
			if len(g.Backdrops) > 0 {
				c.Backdrop = g.Backdrops[0]
			}
			out.genres = append(out.genres, c)
		}
		return out, err
	})
	if err != nil {
		return genreTiles{}, err
	}
	return genreTiles{genres: toGenres(list.genres), stale: list.stale, at: list.at}, nil
}

func toGenres(in []cachedGenre) []DiscoverGenre {
	out := make([]DiscoverGenre, 0, len(in))
	for _, g := range in {
		out = append(out, DiscoverGenre(g))
	}
	return out
}

// detailBudget counts the details a page may still fetch (detailMax per page).
type detailBudget struct {
	mu   sync.Mutex
	left int
}

func (b *detailBudget) take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.left <= 0 {
		return false
	}
	b.left--
	return true
}

// entries checks every hit for the profile (rule 12: the server decides): the library match and
// the age through visibility, the own open wishes. Every hit is looked up in metadata (N-116): a
// stored title gives its texts, age and image kinds, a title not stored is fetched within the
// budget of the page; a fetched one is matched again, its TVDB id may name a library series and
// its age decides below 18.
func (s *Service) entries(ctx context.Context, a discoverAccess, hits []cachedHit, page int, hideAvailable bool, at time.Time, stale bool, budget *detailBudget) (DiscoverPage, error) {
	out := DiscoverPage{Entries: []DiscoverEntry{}, Page: page, Stale: stale, FetchedAt: at}
	if len(hits) == 0 {
		return out, nil
	}
	if s.lib == nil {
		return out, errNoLibrary
	}
	scope := visibility.Profile(a.profile)
	right := requestRight(a.rights)
	refs := make([]visibility.ExternalRef, len(hits))
	for i, h := range hits {
		refs[i] = visibility.ExternalRef{Media: h.Media, TMDBID: h.TMDBID}
	}
	matches, err := s.lib.MatchExternalTitles(ctx, scope, right, refs)
	if err != nil {
		return out, err
	}
	details, fetched := s.fetchDetails(ctx, matches, refs, budget)
	if len(fetched) > 0 {
		re := make([]visibility.ExternalRef, len(fetched))
		for i, idx := range fetched {
			re[i] = refs[idx]
		}
		m2, err := s.lib.MatchExternalTitles(ctx, scope, right, re)
		if err != nil {
			return out, err
		}
		for i, idx := range fetched {
			matches[idx] = m2[i]
		}
	}
	wishes, err := s.openWishes(ctx, a.profile.ID)
	if err != nil {
		return out, err
	}
	for i, h := range hits {
		m := matches[i]
		var e DiscoverEntry
		switch {
		case m.InLibrary && m.ItemID != "" && !hideAvailable:
			e = DiscoverEntry{ItemID: m.ItemID}
		case !m.InLibrary && m.Missing:
			if d := details[i]; d != nil {
				e.MinAge = d.MinAge
			}
		default:
			// A library title the profile may not see (404 rule), a hidden one, one rated above the
			// profile or with an age still unknown below 18, a medium it may not request.
			continue
		}
		e.Media, e.TMDBID, e.Title, e.Year = h.Media, h.TMDBID, h.Title, h.Year
		e.posterPath, e.backdropPath = h.PosterPath, h.BackdropPath
		if d := details[i]; d != nil {
			if name, ok := ownTitle(*d, a.profile); ok {
				e.Title = name
			}
			if d.Year > 0 {
				e.Year = d.Year
			}
			e.ImageKinds = d.ImageKinds
		}
		st := seerr.MediaStatus(h.SeerrStatus)
		e.SeerrRequested = st == seerr.MediaPending || st == seerr.MediaProcessing || st == seerr.MediaPartiallyAvailable
		if w, ok := wishes[refKeyOf(h.Media, h.TMDBID)]; ok {
			e.Wish = &w
		}
		e.CanRequest = e.ItemID == "" && e.Wish == nil && right(h.Media)
		out.Entries = append(out.Entries, e)
	}
	return out, nil
}

// ownTitle is the title of Slouchcat's own data in the profile's metadata language, else its
// interface language (each also by its base), with false when neither has a text: Seerr's text
// stays then (N-116).
func ownTitle(t metadata.ExternalTitle, p model.Profile) (string, bool) {
	for _, lang := range []string{p.MetadataLanguage, model.MetadataBase(p.Language), p.Language} {
		if lang == "" {
			continue
		}
		base, _, _ := strings.Cut(lang, "-")
		for _, l := range []string{lang, base} {
			if tx, ok := t.Texts[l]; ok && tx.Title != "" {
				return tx.Title, true
			}
		}
	}
	return "", false
}

// fetchDetails looks every hit up in metadata within detailWait: a stored title is read without
// cost, a title not stored is fetched from TMDB while the budget lasts (detailMax per page). It
// returns the titles by index (nil: unknown) and the indexes fetched now, whose refs got their
// TVDB and IMDb ids to be matched again.
func (s *Service) fetchDetails(ctx context.Context, matches []visibility.ExternalMatch, refs []visibility.ExternalRef, budget *detailBudget) ([]*metadata.ExternalTitle, []int) {
	details := make([]*metadata.ExternalTitle, len(refs))
	if s.titles == nil {
		return details, nil
	}
	dctx, cancel := context.WithTimeout(ctx, detailWait)
	defer cancel()
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		fetched []int
		sem     = make(chan struct{}, detailParallel)
	)
	for i, m := range matches {
		if !m.Stored && !budget.take() {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, stored bool) {
			defer wg.Done()
			defer func() { <-sem }()
			t, err := s.titles.EnsureExternalTitle(dctx, refs[i].Media, refs[i].TMDBID)
			if err != nil {
				if !errors.Is(err, model.ErrNotFound) {
					s.log.DebugContext(ctx, "discover details not fetched", "media", refs[i].Media, "tmdb_id", refs[i].TMDBID, "err", err)
				}
				return
			}
			mu.Lock()
			defer mu.Unlock()
			details[i] = &t
			if !stored {
				refs[i].TVDBID, refs[i].IMDBID = t.TVDBID, t.IMDBID
				fetched = append(fetched, i)
			}
		}(i, m.Stored)
	}
	wg.Wait()
	return details, fetched
}

func refKeyOf(media string, tmdbID int64) string { return media + ":" + strconv.FormatInt(tmdbID, 10) }

// openWishes are the profile's open wishes by media and TMDB id.
func (s *Service) openWishes(ctx context.Context, profileID int64) (map[string]DiscoverWish, error) {
	rows, err := s.read.ListOpenWishesOfProfile(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("open wishes of the profile: %w", err)
	}
	out := make(map[string]DiscoverWish, len(rows))
	for _, r := range rows {
		out[refKeyOf(r.Media, r.TmdbID)] = DiscoverWish{ID: r.ID, State: r.State}
	}
	return out, nil
}

// searchCache keeps the raw search hits in memory (architecture 21.2: 10 minutes).
type searchCache struct {
	mu      sync.Mutex
	entries map[string]searchEntry
}

type searchEntry struct {
	hits []cachedHit
	at   time.Time
}

func (c *searchCache) get(key string, now time.Time) ([]cachedHit, time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || now.Sub(e.at) >= searchTTL {
		return nil, time.Time{}, false
	}
	return e.hits, e.at, true
}

func (c *searchCache) put(key string, hits []cachedHit, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]searchEntry{}
	}
	if len(c.entries) >= searchMaxEntries {
		for k, e := range c.entries {
			if now.Sub(e.at) >= searchTTL {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= searchMaxEntries {
			c.entries = map[string]searchEntry{}
		}
	}
	c.entries[key] = searchEntry{hits: hits, at: now}
}

// The job wishes.discover (architecture 21.2): every hour it warms the first page of every row for
// each language and age step a profile with the request right has, through one such profile (its
// details too, within the budget of each row), and deletes rows older than a day. It rests while
// the pages are off or the wishes are not active (AK-187).

func (s *Service) registerDiscoverType(r *jobs.Registry) {
	r.Register(jobs.Type{
		Name: JobDiscover, Group: jobs.GroupSystem, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: discoverPriority, Handler: s.handleDiscover, Run: s.runDiscover, Every: discoverEvery,
	})
}

func (s *Service) runDiscover(ctx context.Context, _ *jobs.Queue, prio int) ([]int64, error) {
	if ok, err := s.discoverActive(ctx); err != nil || !ok || s.queue == nil {
		return nil, err
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobDiscover, Priority: prio, IdempotencyKey: discoverRunKey, Payload: []byte(`{}`), MaxAttempts: 1})
	if err != nil {
		return nil, fmt.Errorf("enqueue %s: %w", JobDiscover, err)
	}
	return []int64{id}, nil
}

// discoverActive: the pages are on and the wishes active with a configured Seerr.
func (s *Service) discoverActive(ctx context.Context) (bool, error) {
	on, err := s.DiscoverPagesOn(ctx)
	if err != nil || !on {
		return false, err
	}
	ws, err := s.cfg.WishSettings(ctx)
	if err != nil {
		return false, err
	}
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return false, err
	}
	return ws.Enabled && cfg.Enabled && cfg.Configured(), nil
}

func (s *Service) handleDiscover(ctx context.Context, _ *jobs.Job) error {
	if _, err := s.write.DeleteOldWishDiscoverPages(ctx, s.now().Add(-discoverKeep).UnixMilli()); err != nil {
		return fmt.Errorf("delete old discover pages: %w", err)
	}
	if ok, err := s.discoverActive(ctx); err != nil || !ok {
		return err
	}
	profiles, err := s.read.ListDiscoverProfiles(ctx)
	if err != nil {
		return fmt.Errorf("discover profiles: %w", err)
	}
	done := map[string]bool{}
	warmed, failed := 0, 0
	for _, row := range profiles {
		p := model.Profile{ID: row.ProfileID, AccountID: row.AccountID, Language: row.Language, MetadataLanguage: row.MetadataLanguage.String, MaxRating: int(row.MaxAge)}
		a, err := s.discoverAccess(ctx, p)
		if errors.Is(err, model.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		step := a.language + "|" + strconv.Itoa(a.maxAge)
		if done[step] {
			continue
		}
		done[step] = true
		for _, key := range DiscoverRows {
			if _, err := s.discoverRow(ctx, a, key, &detailBudget{left: detailMax}); err != nil {
				if errors.Is(err, ErrDiscoverUnreachable) {
					failed++
					continue
				}
				return err
			}
			warmed++
		}
	}
	s.log.InfoContext(ctx, "discover lists warmed", "steps", len(done), "rows", warmed, "failed", failed)
	return nil
}
