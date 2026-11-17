package seerr

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Connection and settings (architecture 15.1). The settings routes need ADMIN: call them on
// AsAdmin.

// Me returns GET /auth/me: the user the session acts as. On AsAdmin it is user 1 (key check and
// fingerprint of the instance); on AsUser(id) it proves that X-API-User works when the id comes
// back.
func (s Session) Me(ctx context.Context) (User, error) {
	var out User
	err := s.call(ctx, http.MethodGet, "/auth/me", nil, nil, &out)
	return out, err
}

// PublicSettings returns GET /settings/public (localLogin, 4K flags, partial requests, special
// episodes).
func (s Session) PublicSettings(ctx context.Context) (PublicSettings, error) {
	var out PublicSettings
	err := s.call(ctx, http.MethodGet, "/settings/public", nil, nil, &out)
	return out, err
}

// MetadataSettings returns GET /settings/metadatas (3.x only; 2.x answers ErrNotFound).
func (s Session) MetadataSettings(ctx context.Context) (MetadataSettings, error) {
	var out MetadataSettings
	err := s.call(ctx, http.MethodGet, "/settings/metadatas", nil, nil, &out)
	return out, err
}

// RadarrServers returns GET /settings/radarr. The answer carries the api keys of Radarr: they stay
// in DVRServer.APIKey, a Secret.
func (s Session) RadarrServers(ctx context.Context) ([]DVRServer, error) {
	var out []DVRServer
	err := s.call(ctx, http.MethodGet, "/settings/radarr", nil, nil, &out)
	return out, err
}

// SonarrServers returns GET /settings/sonarr, like RadarrServers.
func (s Session) SonarrServers(ctx context.Context) ([]DVRServer, error) {
	var out []DVRServer
	err := s.call(ctx, http.MethodGet, "/settings/sonarr", nil, nil, &out)
	return out, err
}

// MainSettings returns the part of GET /settings/main the integration reads: the permissions and
// the quota every new user gets (appendix I, I-7). Administrator only.
func (s Session) MainSettings(ctx context.Context) (MainSettings, error) {
	var out MainSettings
	err := s.call(ctx, http.MethodGet, "/settings/main", nil, nil, &out)
	return out, err
}

// ExplainRefusal tells apart why a call on behalf of a user was refused (N-114): Seerr answers an
// unknown X-API-User, a wrong key and a missing right with the same 403 (appendix I, I-2, I-3).
// For an ErrAccessDenied of a session on behalf of a user it asks as the administrator for the
// user (GET /user/{id}): not found is ErrUserGone, refused is ErrKeyInvalid, found keeps err (the
// user lacks the right). Every other error, and err on the administrator session, comes back as
// it is; so does err when the second call fails otherwise (Seerr unreachable).
func (s Session) ExplainRefusal(ctx context.Context, err error) error {
	id, ok := s.UserID()
	if !ok || !errors.Is(err, ErrAccessDenied) {
		return err
	}
	_, check := s.c.AsAdmin().User(ctx, id)
	switch {
	case errors.Is(check, ErrNotFound):
		return &Error{Op: "GET /user/" + pathID(id), Status: http.StatusNotFound, classes: []error{ErrUserGone, ErrNotFound}}
	case errors.Is(check, ErrKeyInvalid):
		return check
	}
	return err
}

// CSRFProtection reports whether Seerr's CSRF protection is on, which refuses every write with the
// api key (appendix D 2.4). It sends POST /request with an empty body, which the request
// validator of Seerr refuses (400) when the protection is off; it never creates a request. Derived
// from the source, not verified against an instance (architecture 15.1). A 2xx answer is
// ErrUnexpectedAnswer, never «protection off» (review, open question): Seerr then accepted the
// empty body.
func (s Session) CSRFProtection(ctx context.Context) (bool, error) {
	err := s.call(ctx, http.MethodPost, "/request", nil, struct{}{}, nil)
	switch {
	case errors.Is(err, ErrCSRF):
		return true, nil
	case errors.Is(err, ErrBadRequest), errors.Is(err, ErrServer):
		return false, nil
	case err == nil:
		return false, ErrUnexpectedAnswer
	}
	return false, err
}

// ErrUnexpectedAnswer: Seerr answered a probe in a way it should not (CSRFProtection).
var ErrUnexpectedAnswer = errors.New("seerr: unexpected answer")

// Users (appendix D 7, appendix H 3.1). The calls need MANAGE_USERS: call them on AsAdmin.

// UserQuery filters GET /user. Zero values are not sent.
type UserQuery struct {
	Take, Skip int
	Q          string // searches username, email, plexUsername and jellyfinUsername
	Sort       string // created, updated, requests, displayname, usertype, role
}

// Users returns one page of GET /user.
func (s Session) Users(ctx context.Context, q UserQuery) (UserPage, error) {
	v := url.Values{}
	setInt(v, "take", q.Take)
	setInt(v, "skip", q.Skip)
	setString(v, "q", q.Q)
	setString(v, "sort", q.Sort)
	var out UserPage
	err := s.call(ctx, http.MethodGet, "/user", v, nil, &out)
	return out, err
}

// User returns GET /user/{id}.
func (s Session) User(ctx context.Context, id int) (User, error) {
	var out User
	err := s.call(ctx, http.MethodGet, "/user/"+pathID(id), nil, nil, &out)
	return out, err
}

// CreateUser creates a local user with POST /user {email, username, password}. A user with the
// same email in any spelling answers ErrUserExists. Seerr 3.5.1 answers 201 without the email and
// with the instance's defaultPermissions, whatever the body says (appendix I, I-12): do not read
// the email from the answer. The password never appears in an error or the log.
func (s Session) CreateUser(ctx context.Context, u NewUser) (User, error) {
	if strings.TrimSpace(u.Email) == "" || u.Password == "" {
		return User{}, fmt.Errorf("%w: a new user needs email and password", ErrInvalid)
	}
	body := struct {
		Email    string `json:"email"`
		Username string `json:"username,omitempty"`
		Password string `json:"password"`
	}{u.Email, u.Username, u.Password}
	var out User
	err := s.call(ctx, http.MethodPost, "/user", nil, body, &out, u.Password)
	return out, err
}

// SetPassword sets the password of another user with POST /user/{id}/settings/password
// {newPassword}; Seerr wants at least 8 characters. Seerr refuses it for user 1 (it wants the
// current password) and for administrators other than the caller (appendix H 3.1). The password
// never appears in an error or the log.
func (s Session) SetPassword(ctx context.Context, id int, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("%w: empty password", ErrInvalid)
	}
	body := struct {
		NewPassword string `json:"newPassword"`
	}{newPassword}
	return s.call(ctx, http.MethodPost, "/user/"+pathID(id)+"/settings/password", nil, body, nil, newPassword)
}

// SetPermissions sets the permission mask with POST /user/{id}/settings/permissions.
func (s Session) SetPermissions(ctx context.Context, id int, p Permission) error {
	body := struct {
		Permissions Permission `json:"permissions"`
	}{p}
	return s.call(ctx, http.MethodPost, "/user/"+pathID(id)+"/settings/permissions", nil, body, nil)
}

// UserSettings is GET /user/{id}/settings/main. Seerr's POST of the same route writes every field
// from the body and clears the ones that are missing (appendix H 3.1), so the value keeps every
// field it read and SetUserSettings sends all of them back, with the typed fields laid over them.
// A quota field nil is sent as null (Seerr's default quota).
type UserSettings struct {
	Username        *string
	Email           *string
	MovieQuotaLimit *int
	MovieQuotaDays  *int
	TVQuotaLimit    *int
	TVQuotaDays     *int

	fields map[string]json.RawMessage // everything read, by key
}

// Keys of the typed fields of UserSettings.
const (
	keyUsername        = "username"
	keyEmail           = "email"
	keyMovieQuotaLimit = "movieQuotaLimit"
	keyMovieQuotaDays  = "movieQuotaDays"
	keyTVQuotaLimit    = "tvQuotaLimit"
	keyTVQuotaDays     = "tvQuotaDays"
)

// Field returns the raw JSON of a field that was read and true, for fields without a typed one.
func (u UserSettings) Field(key string) (json.RawMessage, bool) {
	v, ok := u.fields[key]
	return v, ok
}

// UnmarshalJSON reads every field and the typed ones.
func (u *UserSettings) UnmarshalJSON(data []byte) error {
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	out := UserSettings{fields: fields}
	for key, dst := range map[string]any{
		keyUsername: &out.Username, keyEmail: &out.Email,
		keyMovieQuotaLimit: &out.MovieQuotaLimit, keyMovieQuotaDays: &out.MovieQuotaDays,
		keyTVQuotaLimit: &out.TVQuotaLimit, keyTVQuotaDays: &out.TVQuotaDays,
	} {
		if raw, ok := fields[key]; ok {
			if err := json.Unmarshal(raw, dst); err != nil {
				return fmt.Errorf("settings field %s: %w", key, err)
			}
		}
	}
	*u = out
	return nil
}

// MarshalJSON writes every field that was read, the typed fields laid over them.
func (u UserSettings) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(u.fields)+6)
	for k, v := range u.fields {
		out[k] = v
	}
	out[keyUsername] = u.Username
	out[keyEmail] = u.Email
	out[keyMovieQuotaLimit] = u.MovieQuotaLimit
	out[keyMovieQuotaDays] = u.MovieQuotaDays
	out[keyTVQuotaLimit] = u.TVQuotaLimit
	out[keyTVQuotaDays] = u.TVQuotaDays
	return json.Marshal(out)
}

// UserSettings returns GET /user/{id}/settings/main.
func (s Session) UserSettings(ctx context.Context, id int) (UserSettings, error) {
	var out UserSettings
	err := s.call(ctx, http.MethodGet, "/user/"+pathID(id)+"/settings/main", nil, nil, &out)
	return out, err
}

// SetUserSettings writes POST /user/{id}/settings/main with every field of set, which must come
// from UserSettings (ErrInvalid otherwise: a body without the other fields would clear them).
// Seerr takes the quota fields only for users without MANAGE_USERS. Its answer has no quota
// fields (appendix I, I-16): read them back with Quota or User. An email of another user is
// ErrInvalidEmail, and Seerr leaves the user unchanged (I-18).
func (s Session) SetUserSettings(ctx context.Context, id int, set UserSettings) error {
	if set.fields == nil {
		return fmt.Errorf("%w: read the user settings before writing them", ErrInvalid)
	}
	return s.call(ctx, http.MethodPost, "/user/"+pathID(id)+"/settings/main", nil, set, nil)
}

// Quota returns GET /user/{id}/quota (needs MANAGE_USERS and MANAGE_REQUESTS for another user).
func (s Session) Quota(ctx context.Context, id int) (Quota, error) {
	var out Quota
	err := s.call(ctx, http.MethodGet, "/user/"+pathID(id)+"/quota", nil, nil, &out)
	return out, err
}

// UserRequests returns one page of GET /user/{id}/requests (newest first); take and skip 0 are not
// sent.
func (s Session) UserRequests(ctx context.Context, id, take, skip int) (RequestPage, error) {
	v := url.Values{}
	setInt(v, "take", take)
	setInt(v, "skip", skip)
	var out RequestPage
	err := s.call(ctx, http.MethodGet, "/user/"+pathID(id)+"/requests", v, nil, &out)
	return out, err
}

// DeleteUser deletes a user with DELETE /user/{id}. Seerr deletes the user's requests with it
// (appendix D 7.2); the integration itself never deletes a user, only tests and the check against
// a real instance do.
func (s Session) DeleteUser(ctx context.Context, id int) error {
	return s.call(ctx, http.MethodDelete, "/user/"+pathID(id), nil, nil, nil)
}

// Requests (appendix D 3, 6, 8).

// CreateRequest sends POST /request on behalf of the session's user; on AsAdmin it refuses with
// ErrInvalid, because Seerr would approve every request of user 1 at once (architecture 4.1). The
// body never carries userId. A series needs TVDBID and at least one season. ErrDuplicate and
// ErrNoSeasons (202) mean Seerr created nothing.
func (s Session) CreateRequest(ctx context.Context, r NewRequest) (Request, error) {
	if _, ok := s.UserID(); !ok {
		return Request{}, fmt.Errorf("%w: requests are made on behalf of a user (AsUser)", ErrInvalid)
	}
	switch {
	case r.TMDBID < 1:
		return Request{}, fmt.Errorf("%w: request without tmdb id", ErrInvalid)
	case r.MediaType == MediaTypeMovie && (r.TVDBID != 0 || len(r.Seasons) > 0):
		return Request{}, fmt.Errorf("%w: a movie request has no tvdb id and no seasons", ErrInvalid)
	case r.MediaType == MediaTypeTV && (r.TVDBID < 1 || len(r.Seasons) == 0):
		return Request{}, fmt.Errorf("%w: a series request needs the tvdb id and seasons", ErrInvalid)
	case r.MediaType != MediaTypeMovie && r.MediaType != MediaTypeTV:
		return Request{}, fmt.Errorf("%w: unknown media type", ErrInvalid)
	}
	body := struct {
		MediaType string `json:"mediaType"`
		MediaID   int    `json:"mediaId"`
		TVDBID    int    `json:"tvdbId,omitempty"`
		Seasons   []int  `json:"seasons,omitempty"`
		Is4K      bool   `json:"is4k"`
	}{r.MediaType, r.TMDBID, r.TVDBID, r.Seasons, r.Is4K}
	var out Request
	status, err := s.send(ctx, http.MethodPost, "/request", nil, body, &out)
	if err != nil {
		return Request{}, err
	}
	if status == http.StatusAccepted {
		return Request{}, &Error{Op: "POST /request", Status: status, Message: "No seasons available to request", classes: []error{ErrNoSeasons}}
	}
	return out, nil
}

// Request returns GET /request/{id}; a deleted request is ErrNotFound.
func (s Session) Request(ctx context.Context, id int) (Request, error) {
	var out Request
	err := s.call(ctx, http.MethodGet, "/request/"+pathID(id), nil, nil, &out)
	return out, err
}

// RequestQuery filters GET /request. Zero values are not sent; Seerr has no "since" parameter, so
// a sync sorts by modified and stops at the last known updatedAt.
type RequestQuery struct {
	Take, Skip    int
	Filter        string // approved, pending, unavailable, failed, completed, …; "" all
	Sort          string // modified, added; "" id
	SortDirection string // asc, desc
	RequestedBy   int
	MediaType     string // movie, tv, all
}

// Requests returns one page of GET /request.
func (s Session) Requests(ctx context.Context, q RequestQuery) (RequestPage, error) {
	v := url.Values{}
	setInt(v, "take", q.Take)
	setInt(v, "skip", q.Skip)
	setString(v, "filter", q.Filter)
	setString(v, "sort", q.Sort)
	setString(v, "sortDirection", q.SortDirection)
	setInt(v, "requestedBy", q.RequestedBy)
	setString(v, "mediaType", q.MediaType)
	var out RequestPage
	err := s.call(ctx, http.MethodGet, "/request", v, nil, &out)
	return out, err
}

// DeleteRequest deletes a request with DELETE /request/{id}. Seerr allows it for an own PENDING
// request or with MANAGE_REQUESTS; otherwise it answers 401, which is ErrNoPermission.
func (s Session) DeleteRequest(ctx context.Context, id int) error {
	return s.call(ctx, http.MethodDelete, "/request/"+pathID(id), nil, nil, nil)
}

// ApproveRequest approves a PENDING request (MANAGE_REQUESTS); another status is ErrConflict.
func (s Session) ApproveRequest(ctx context.Context, id int) (Request, error) {
	var out Request
	err := s.call(ctx, http.MethodPost, "/request/"+pathID(id)+"/approve", nil, nil, &out)
	return out, err
}

// DeclineRequest declines a PENDING request (MANAGE_REQUESTS); Seerr keeps no reason.
func (s Session) DeclineRequest(ctx context.Context, id int) (Request, error) {
	var out Request
	err := s.call(ctx, http.MethodPost, "/request/"+pathID(id)+"/decline", nil, nil, &out)
	return out, err
}

// RetryRequest sends a FAILED request to Radarr or Sonarr again (MANAGE_REQUESTS, admin session;
// concept 25, architecture 8.1 «arr_failed»); another status is ErrConflict. Not verified against a
// real instance.
func (s Session) RetryRequest(ctx context.Context, id int) (Request, error) {
	var out Request
	err := s.call(ctx, http.MethodPost, "/request/"+pathID(id)+"/retry", nil, nil, &out)
	return out, err
}

// Titles. Every call makes Seerr ask TMDB (appendix D 6.2).

// Movie returns GET /movie/{tmdbId} with mediaInfo and its requests.
func (s Session) Movie(ctx context.Context, tmdbID int) (MovieDetails, error) {
	var out MovieDetails
	err := s.call(ctx, http.MethodGet, "/movie/"+pathID(tmdbID), nil, nil, &out)
	return out, err
}

// TV returns GET /tv/{tmdbId} with the seasons Seerr can request and mediaInfo.
func (s Session) TV(ctx context.Context, tmdbID int) (TVDetails, error) {
	var out TVDetails
	err := s.call(ctx, http.MethodGet, "/tv/"+pathID(tmdbID), nil, nil, &out)
	return out, err
}

// Discover and search (appendix H 3.3, appendix I section 4). Every list is a TMDB call inside
// Seerr. The calls run only on behalf of a user (AsUser: X-API-User), so Seerr answers with that
// user's language and request state; the administrator session refuses them with ErrInvalid. Each
// query type carries only the parameters its route knows, and a guard (checkQuery) refuses every
// other parameter before the request leaves: Seerr's request validator answers an unknown one
// with 400 «Unknown query parameter» (appendix D 2.5, I-44: certification.lte is refused).

// PageQuery is the query of the routes that know only page and language (upcoming, by genre).
type PageQuery struct {
	Page     int
	Language string
}

func (q PageQuery) values() url.Values {
	v := url.Values{}
	setInt(v, "page", q.Page)
	setString(v, "language", q.Language)
	return v
}

// TrendingQuery is the query of GET /discover/trending.
type TrendingQuery struct {
	PageQuery
	MediaType  string // all, movie, tv; "" = Seerr's default (all)
	TimeWindow string // day, week; "" = Seerr's default (day)
}

// DiscoverQuery is the query of GET /discover/movies and /discover/tv. It has no language: on these
// two routes Seerr reads language as a filter on the original language (appendix I, I-60, N-115),
// and their display language is the acting user's locale, not selectable per call (I-61).
type DiscoverQuery struct {
	Page                 int
	SortBy               string // for example popularity.desc; Seerr answers an unknown value with the default list (I-68)
	Genre                string // TMDB genre ids, comma separated
	CertificationLte     string // a certificate of CertificationCountry, for example "12" (DE)
	CertificationCountry string // ISO 3166-1 alpha-2
	ReleasedBefore       string // YYYY-MM-DD: primaryReleaseDateLte (films) resp. firstAirDateLte (series)
	VoteCountGte         int    // the least number of votes (I-64, I-65)
}

// MaxPage is the highest page TMDB serves: page 501 answers 500 although totalPages may say 1001
// (I-67). The client refuses a higher page and reports at most this many pages.
const MaxPage = 500

// The discover and search routes and the query parameters Seerr's validator accepts on each
// (GH:server/routes/discover.ts, search.ts; appendix H 3.3). The client sends a subset.
const (
	routeTrending       = "/discover/trending"
	routeMovies         = "/discover/movies"
	routeMoviesUpcoming = "/discover/movies/upcoming"
	routeTV             = "/discover/tv"
	routeTVUpcoming     = "/discover/tv/upcoming"
	routeMoviesGenre    = "/discover/movies/genre/{id}"
	routeTVGenre        = "/discover/tv/genre/{id}"
	routeMovieSlider    = "/discover/genreslider/movie"
	routeTVSlider       = "/discover/genreslider/tv"
	routeMovieGenres    = "/genres/movie"
	routeTVGenres       = "/genres/tv"
	routeSearch         = "/search"
)

var (
	pageLanguage = []string{"page", "language"}
	// Seerr knows language on /discover/movies and /discover/tv too, as a filter on the original
	// language (I-60); the client never sends it there (N-115), so the guard does not allow it.
	discoverFilter = []string{"page", "sortBy", "genre", "studio", "keywords", "primaryReleaseDateGte",
		"primaryReleaseDateLte", "withRuntimeGte", "withRuntimeLte", "voteAverageGte", "voteAverageLte", "voteCountGte",
		"voteCountLte", "watchProviders", "watchRegion", "certification", "certificationGte", "certificationLte",
		"certificationCountry"}
	discoverTVFilter = []string{"page", "sortBy", "genre", "network", "keywords", "firstAirDateGte", "firstAirDateLte",
		"withRuntimeGte", "withRuntimeLte", "voteAverageGte", "voteAverageLte", "voteCountGte", "voteCountLte",
		"watchProviders", "watchRegion", "status", "certification", "certificationGte", "certificationLte",
		"certificationCountry"}

	routeParams = map[string][]string{
		routeTrending:       {"page", "language", "mediaType", "timeWindow"},
		routeMovies:         discoverFilter,
		routeMoviesUpcoming: pageLanguage,
		routeTV:             discoverTVFilter,
		routeTVUpcoming:     pageLanguage,
		routeMoviesGenre:    pageLanguage,
		routeTVGenre:        pageLanguage,
		routeMovieSlider:    {"language"},
		routeTVSlider:       {"language"},
		routeMovieGenres:    {"language"},
		routeTVGenres:       {"language"},
		routeSearch:         {"query", "page", "language"},
	}

	// routeValues are the values a parameter may take where Seerr knows a fixed set.
	routeValues = map[string][]string{
		"mediaType":  {"all", MediaTypeMovie, MediaTypeTV},
		"timeWindow": {"day", "week"},
	}
)

// checkQuery is the guard of the discover and search routes: a session on behalf of a user, only
// parameters Seerr's validator knows for route, only known values of the fixed sets.
func (s Session) checkQuery(route string, v url.Values) error {
	op := http.MethodGet + " " + route
	if p := v.Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err != nil || n < 1 || n > MaxPage {
			return &Error{Op: op, classes: []error{ErrInvalid}, cause: errors.New("page out of range")}
		}
	}
	if _, ok := s.UserID(); !ok {
		return &Error{Op: op, classes: []error{ErrInvalid}, cause: errors.New("discover and search need a user session")}
	}
	known, ok := routeParams[route]
	if !ok {
		return &Error{Op: op, classes: []error{ErrInvalid}, cause: errors.New("unknown discover route")}
	}
	for k, vals := range v {
		if !slices.Contains(known, k) {
			return &Error{Op: op, classes: []error{ErrInvalid}, cause: fmt.Errorf("query parameter %q is not known to Seerr", k)}
		}
		if set, fixed := routeValues[k]; fixed {
			for _, val := range vals {
				if !slices.Contains(set, val) {
					return &Error{Op: op, classes: []error{ErrInvalid}, cause: fmt.Errorf("value of %q is not known to Seerr", k)}
				}
			}
		}
	}
	return nil
}

// Trending returns GET /discover/trending. The route has no certification filter.
func (s Session) Trending(ctx context.Context, q TrendingQuery) (DiscoverPage, error) {
	v := q.values()
	setString(v, "mediaType", q.MediaType)
	setString(v, "timeWindow", q.TimeWindow)
	return s.discover(ctx, routeTrending, routeTrending, v)
}

// DiscoverMovies returns GET /discover/movies.
func (s Session) DiscoverMovies(ctx context.Context, q DiscoverQuery) (DiscoverPage, error) {
	return s.discover(ctx, routeMovies, routeMovies, q.discoverValues(false))
}

// DiscoverTV returns GET /discover/tv.
func (s Session) DiscoverTV(ctx context.Context, q DiscoverQuery) (DiscoverPage, error) {
	return s.discover(ctx, routeTV, routeTV, q.discoverValues(true))
}

// UpcomingMovies returns GET /discover/movies/upcoming.
func (s Session) UpcomingMovies(ctx context.Context, q PageQuery) (DiscoverPage, error) {
	return s.discover(ctx, routeMoviesUpcoming, routeMoviesUpcoming, q.values())
}

// UpcomingTV returns GET /discover/tv/upcoming.
func (s Session) UpcomingTV(ctx context.Context, q PageQuery) (DiscoverPage, error) {
	return s.discover(ctx, routeTVUpcoming, routeTVUpcoming, q.values())
}

// MoviesByGenre returns GET /discover/movies/genre/{id}.
func (s Session) MoviesByGenre(ctx context.Context, genreID int, q PageQuery) (DiscoverPage, error) {
	return s.discover(ctx, routeMoviesGenre, "/discover/movies/genre/"+pathID(genreID), q.values())
}

// TVByGenre returns GET /discover/tv/genre/{id}.
func (s Session) TVByGenre(ctx context.Context, genreID int, q PageQuery) (DiscoverPage, error) {
	return s.discover(ctx, routeTVGenre, "/discover/tv/genre/"+pathID(genreID), q.values())
}

// MovieGenres returns GET /discover/genreslider/movie: the genre tiles with a few backdrops.
func (s Session) MovieGenres(ctx context.Context, language string) ([]GenreSlide, error) {
	var out []GenreSlide
	err := s.genres(ctx, routeMovieSlider, language, &out)
	return out, err
}

// TVGenres returns GET /discover/genreslider/tv.
func (s Session) TVGenres(ctx context.Context, language string) ([]GenreSlide, error) {
	var out []GenreSlide
	err := s.genres(ctx, routeTVSlider, language, &out)
	return out, err
}

// MovieGenreList returns GET /genres/movie: every film genre with its name (I-45), for the genre
// filter of «Filme».
func (s Session) MovieGenreList(ctx context.Context, language string) ([]Genre, error) {
	var out []Genre
	err := s.genres(ctx, routeMovieGenres, language, &out)
	return out, err
}

// TVGenreList returns GET /genres/tv.
func (s Session) TVGenreList(ctx context.Context, language string) ([]Genre, error) {
	var out []Genre
	err := s.genres(ctx, routeTVGenres, language, &out)
	return out, err
}

// SearchQuery is the query of GET /search. Query goes to Seerr as typed (space as %20), so the
// prefixes tmdb:, tvdb:, imdb: and year: work (GH:server/lib/search.ts); SearchByID builds them.
type SearchQuery struct {
	PageQuery
	Query string
}

// Search returns GET /search; the results may contain persons (MediaType "person").
func (s Session) Search(ctx context.Context, q SearchQuery) (DiscoverPage, error) {
	if strings.TrimSpace(q.Query) == "" {
		return DiscoverPage{}, fmt.Errorf("%w: empty search", ErrInvalid)
	}
	v := q.values()
	v.Set("query", q.Query)
	return s.discover(ctx, routeSearch, routeSearch, v)
}

// IDPrefix is a prefix of the search by id (N-53).
type IDPrefix string

// The prefixes of the search by id; year: is not offered (data model 4.5).
const (
	PrefixTMDB IDPrefix = "tmdb"
	PrefixTVDB IDPrefix = "tvdb"
	PrefixIMDB IDPrefix = "imdb"
)

var (
	idDigits = regexp.MustCompile(`^[0-9]{1,10}$`)
	idIMDb   = regexp.MustCompile(`^tt[0-9]{1,10}$`)
)

// ParseIDSearch reads a search text that names a title by id: tmdb:603, TMDB: 603, tvdb:121361,
// imdb:tt0133093 (prefix case-insensitive, blanks around the id allowed; Seerr itself finds
// nothing for "tmdb: 603", I-42). It returns the prefix and the normalised id, or false when the
// text is no search by id or the id has the wrong form.
func ParseIDSearch(q string) (IDPrefix, string, bool) {
	head, id, ok := strings.Cut(strings.TrimSpace(q), ":")
	if !ok {
		return "", "", false
	}
	prefix := IDPrefix(strings.ToLower(strings.TrimSpace(head)))
	id = strings.ToLower(strings.TrimSpace(id))
	switch {
	case (prefix == PrefixTMDB || prefix == PrefixTVDB) && idDigits.MatchString(id):
		id = strings.TrimLeft(id, "0")
		return prefix, id, id != ""
	case prefix == PrefixIMDB && idIMDb.MatchString(id):
		return prefix, id, true
	}
	return "", "", false
}

// SearchByID searches Seerr for the title with the id and keeps only the hits that are it:
// tmdb:603 also finds the series and the person with that TMDB id (I-42), so the hits are
// filtered by mediaType (MediaTypeMovie, MediaTypeTV, "" for both) and, for tmdb:, by the id;
// tvdb: finds series only; persons never pass. ErrInvalid for an id of the wrong form.
func (s Session) SearchByID(ctx context.Context, prefix IDPrefix, id, mediaType string, q PageQuery) (DiscoverPage, error) {
	p, norm, ok := ParseIDSearch(string(prefix) + ":" + id)
	if !ok || p != prefix || (mediaType != "" && mediaType != MediaTypeMovie && mediaType != MediaTypeTV) {
		return DiscoverPage{}, fmt.Errorf("%w: search by id", ErrInvalid)
	}
	page, err := s.Search(ctx, SearchQuery{PageQuery: q, Query: string(p) + ":" + norm})
	if err != nil {
		return page, err
	}
	tmdbID, _ := strconv.Atoi(norm)
	kept := page.Results[:0]
	for _, t := range page.Results {
		switch {
		case t.MediaType != MediaTypeMovie && t.MediaType != MediaTypeTV:
		case mediaType != "" && t.MediaType != mediaType:
		case p == PrefixTMDB && t.ID != tmdbID:
		case p == PrefixTVDB && t.MediaType != MediaTypeTV:
		default:
			kept = append(kept, t)
		}
	}
	// The film before the series of the same id, as the real instance answered (appendix I,
	// I-42), whatever order a page has.
	slices.SortStableFunc(kept, func(a, b Title) int {
		return cmp.Compare(mediaOrder(a.MediaType), mediaOrder(b.MediaType))
	})
	page.Results = kept
	page.TotalResults = len(kept)
	page.TotalPages = min(page.TotalPages, 1)
	return page, nil
}

func mediaOrder(t string) int {
	if t == MediaTypeMovie {
		return 0
	}
	return 1
}

func (q DiscoverQuery) discoverValues(tv bool) url.Values {
	v := url.Values{}
	setInt(v, "page", q.Page)
	setString(v, "sortBy", q.SortBy)
	setString(v, "genre", q.Genre)
	setString(v, "certificationLte", q.CertificationLte)
	setString(v, "certificationCountry", q.CertificationCountry)
	if tv {
		setString(v, "firstAirDateLte", q.ReleasedBefore)
	} else {
		setString(v, "primaryReleaseDateLte", q.ReleasedBefore)
	}
	setInt(v, "voteCountGte", q.VoteCountGte)
	return v
}

func (s Session) discover(ctx context.Context, route, path string, v url.Values) (DiscoverPage, error) {
	if err := s.checkQuery(route, v); err != nil {
		return DiscoverPage{}, err
	}
	var out DiscoverPage
	err := s.call(ctx, http.MethodGet, path, v, nil, &out)
	out.TotalPages = min(out.TotalPages, MaxPage)
	return out, err
}

func (s Session) genres(ctx context.Context, route, language string, out any) error {
	v := url.Values{}
	setString(v, "language", language)
	if err := s.checkQuery(route, v); err != nil {
		return err
	}
	return s.call(ctx, http.MethodGet, route, v, nil, out)
}

func setInt(v url.Values, key string, n int) {
	if n > 0 {
		v.Set(key, strconv.Itoa(n))
	}
}

func setString(v url.Values, key, s string) {
	if s != "" {
		v.Set(key, s)
	}
}
