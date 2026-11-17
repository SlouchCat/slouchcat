package seerr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
// from the source, not verified against an instance (architecture 15.1).
func (s Session) CSRFProtection(ctx context.Context) (bool, error) {
	err := s.call(ctx, http.MethodPost, "/request", nil, struct{}{}, nil)
	switch {
	case errors.Is(err, ErrCSRF):
		return true, nil
	case errors.Is(err, ErrBadRequest), errors.Is(err, ErrServer):
		return false, nil
	case err == nil:
		return false, nil
	}
	return false, err
}

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

// Discover and search (appendix H 3.3). Every list is a TMDB call inside Seerr. Each query type
// carries only the parameters its route knows: Seerr's request validator refuses others with 400.

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
	MediaType  string // all, movie, tv
	TimeWindow string // day, week
}

// DiscoverQuery is the query of GET /discover/movies and /discover/tv.
type DiscoverQuery struct {
	PageQuery
	SortBy               string // for example popularity.desc
	Genre                string // TMDB genre ids, comma separated
	CertificationLte     string
	CertificationCountry string
}

// Trending returns GET /discover/trending.
func (s Session) Trending(ctx context.Context, q TrendingQuery) (DiscoverPage, error) {
	v := q.values()
	setString(v, "mediaType", q.MediaType)
	setString(v, "timeWindow", q.TimeWindow)
	return s.discover(ctx, "/discover/trending", v)
}

// DiscoverMovies returns GET /discover/movies.
func (s Session) DiscoverMovies(ctx context.Context, q DiscoverQuery) (DiscoverPage, error) {
	return s.discover(ctx, "/discover/movies", q.discoverValues())
}

// DiscoverTV returns GET /discover/tv.
func (s Session) DiscoverTV(ctx context.Context, q DiscoverQuery) (DiscoverPage, error) {
	return s.discover(ctx, "/discover/tv", q.discoverValues())
}

// UpcomingMovies returns GET /discover/movies/upcoming.
func (s Session) UpcomingMovies(ctx context.Context, q PageQuery) (DiscoverPage, error) {
	return s.discover(ctx, "/discover/movies/upcoming", q.values())
}

// UpcomingTV returns GET /discover/tv/upcoming.
func (s Session) UpcomingTV(ctx context.Context, q PageQuery) (DiscoverPage, error) {
	return s.discover(ctx, "/discover/tv/upcoming", q.values())
}

// MoviesByGenre returns GET /discover/movies/genre/{id}.
func (s Session) MoviesByGenre(ctx context.Context, genreID int, q PageQuery) (DiscoverPage, error) {
	return s.discover(ctx, "/discover/movies/genre/"+pathID(genreID), q.values())
}

// TVByGenre returns GET /discover/tv/genre/{id}.
func (s Session) TVByGenre(ctx context.Context, genreID int, q PageQuery) (DiscoverPage, error) {
	return s.discover(ctx, "/discover/tv/genre/"+pathID(genreID), q.values())
}

// MovieGenres returns GET /discover/genreslider/movie.
func (s Session) MovieGenres(ctx context.Context, language string) ([]GenreSlide, error) {
	return s.genres(ctx, "/discover/genreslider/movie", language)
}

// TVGenres returns GET /discover/genreslider/tv.
func (s Session) TVGenres(ctx context.Context, language string) ([]GenreSlide, error) {
	return s.genres(ctx, "/discover/genreslider/tv", language)
}

// SearchQuery is the query of GET /search. Query goes to Seerr as typed, so the prefixes tmdb:,
// tvdb:, imdb: and year: work (GH:server/lib/search.ts).
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
	return s.discover(ctx, "/search", v)
}

func (q DiscoverQuery) discoverValues() url.Values {
	v := q.values()
	setString(v, "sortBy", q.SortBy)
	setString(v, "genre", q.Genre)
	setString(v, "certificationLte", q.CertificationLte)
	setString(v, "certificationCountry", q.CertificationCountry)
	return v
}

func (s Session) discover(ctx context.Context, path string, v url.Values) (DiscoverPage, error) {
	var out DiscoverPage
	err := s.call(ctx, http.MethodGet, path, v, nil, &out)
	return out, err
}

func (s Session) genres(ctx context.Context, path, language string) ([]GenreSlide, error) {
	v := url.Values{}
	setString(v, "language", language)
	var out []GenreSlide
	err := s.call(ctx, http.MethodGet, path, v, nil, &out)
	return out, err
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
