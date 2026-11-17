// Package seerrtest is a fake Seerr on an httptest.Server for the tests of package seerr and of the
// packages built on it (concept 25, build plan step 1.1). It models what appendix D and appendix H
// 3 describe of Seerr v3.5.2: authentication with X-Api-Key and X-API-User, users (local, Jellyfin,
// administrators, user 1), user settings that clear missing fields, the password and permission
// routes, the rolling quota, requests with auto-approval and status changes, duplicates (409) and
// series without new seasons (202), media status, titles, discover lists and the search, the
// settings routes, the request validator (unknown query parameters answer 400) and the failure
// pictures (403 texts, CSRF protection, 500, redirects, a hanging answer). Every call is counted per
// route, so a test can prove what was and was not called.
//
// Where appendix I (the check against a real Seerr 3.5.1) differs, the fake answers as appendix I
// says: POST /user 201 without the email and with defaultPermissions; 409 for an email in any
// spelling; POST …/settings/main without the quota fields and 400 INVALID_EMAIL for the email of
// another user; GET /settings/main with defaultPermissions and defaultQuotas; a wrong key and an
// unknown X-API-User 403 with the text in the field error, no key 401; users of the types Plex,
// local, Jellyfin and Emby.
//
// It is a test helper: only _test.go files import it (depguard rule test-helpers). It imports
// nothing of the server, so the tests of package seerr itself can use it.
package seerrtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// APIKey is the api key of every fake.
const APIKey = "seerr-test-key-6f1c2a9e4b7d" //nolint:gosec // G101: the key of the fake, not a credential

// Permission bits (appendix D 7.3); the fake needs only these.
const (
	PermAdmin            uint64 = 2
	PermManageSettings   uint64 = 4
	PermManageUsers      uint64 = 8
	PermManageRequests   uint64 = 16
	PermRequest          uint64 = 32
	PermAutoApprove      uint64 = 128
	PermAutoApproveMovie uint64 = 256
	PermAutoApproveTV    uint64 = 512
	PermRequestView      uint64 = 16384
	PermRequestMovie     uint64 = 262144
	PermRequestTV        uint64 = 524288
	PermManageIssues     uint64 = 1048576
)

// OwnerDefaultPermissions is the defaultPermissions of the owner's instance (appendix I, I-13):
// REQUEST, AUTO_APPROVE and MANAGE_ISSUES.
const OwnerDefaultPermissions = PermRequest | PermAutoApprove | PermManageIssues

// User types.
const (
	UserPlex     = 1
	UserLocal    = 2
	UserJellyfin = 3
	UserEmby     = 4
)

// Request statuses (appendix D 5).
const (
	RequestPending   = 1
	RequestApproved  = 2
	RequestDeclined  = 3
	RequestFailed    = 4
	RequestCompleted = 5
)

// Media statuses (appendix D 5).
const (
	MediaUnknown            = 1
	MediaPending            = 2
	MediaProcessing         = 3
	MediaPartiallyAvailable = 4
	MediaAvailable          = 5
	MediaBlocklisted        = 6
	MediaDeleted            = 7
)

// Texts of Seerr the fake answers with (appendix D 2.1, 3.4, 7.2, 8).
const (
	TextAccessEndpoint = "You do not have permission to access this endpoint"
	TextNoSession      = "cookie 'connect.sid' required"
	TextInvalidEmail   = "INVALID_EMAIL"
	TextCSRF           = "invalid csrf token"
	TextMovieQuota     = "Movie Quota exceeded."
	TextSeriesQuota    = "Series Quota exceeded."
	TextBlocklisted    = "This media is blocklisted."
	TextDuplicate      = "Request for this media already exists."
	TextNoSeasons      = "No seasons available to request"
	TextUserExists     = "User already exists with submitted email."
	TextNoMovieRight   = "You do not have permission to make movie requests."
	TextNoSeriesRight  = "You do not have permission to make series requests."
	TextModifyUser     = "You do not have permission to modify the request user."
	TextDeleteRequest  = "You do not have permission to delete this request."
	TextOnlyPending    = "Only pending requests can be approved or declined."
	TextOnlyFailed     = "Only failed requests can be retried."
)

// User is a Seerr user of the fake. Nil quota fields follow the defaults of Config.
type User struct {
	ID               int
	Email            string
	Username         string
	PlexUsername     string
	JellyfinUsername string // Emby users carry their name here too (appendix I, I-11)
	JellyfinUserID   string
	UserType         int // UserPlex, UserLocal (default), UserJellyfin, UserEmby
	Permissions      uint64
	Password         string // local users; the fake keeps it in clear text, it is a test helper
	MovieQuotaLimit  *int
	MovieQuotaDays   *int
	TVQuotaLimit     *int
	TVQuotaDays      *int
	// Further fields of settings/main.
	Locale, DiscoverRegion, StreamingRegion, OriginalLanguage, DiscordID string
	CreatedAt                                                            time.Time
}

func (u *User) has(p uint64) bool { return u.Permissions&PermAdmin != 0 || u.Permissions&p == p }

func (u *User) hasAny(ps ...uint64) bool {
	return slices.ContainsFunc(ps, u.has)
}

// Season is a season of a series title.
type Season struct {
	Number   int
	Episodes int
}

// Title is a TMDB title the fake knows; MediaType is "movie", "tv" or (search only) "person".
type Title struct {
	MediaType     string
	TMDBID        int
	TVDBID        int
	IMDBID        string
	Name          string
	Year          int
	GenreIDs      []int
	Popularity    float64
	Certification string // an age, compared by certificationLte
	// IgnoresCertification: the lists return the title whatever certificationLte says, like TMDB
	// does for a title rated in another country only (the server checks every hit again, N-56).
	IgnoresCertification bool
	Upcoming             bool
	// OriginalLanguage is the original language ("" = "en"): /discover/movies and /discover/tv
	// read their parameter language as a filter on it (appendix I, I-60).
	OriginalLanguage string
	// Names are the display names per language beside Name (the fallback).
	Names map[string]string
	// Released is the release or first air date YYYY-MM-DD ("" = 1 January of Year).
	Released    string
	VoteCount   int
	VoteAverage float64
	Seasons     []Season
}

// Request is a request of the fake.
type Request struct {
	ID         int
	MediaType  string
	TMDBID     int
	UserID     int
	Status     int
	Seasons    []int
	ModifiedBy int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// DVR is a Radarr or Sonarr server of /settings/radarr|sonarr.
type DVR struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Hostname    string `json:"hostname"`
	Port        int    `json:"port"`
	APIKey      string `json:"apiKey"`
	UseSSL      bool   `json:"useSsl"`
	BaseURL     string `json:"baseUrl"`
	Tags        []int  `json:"tags"`
	TagRequests bool   `json:"tagRequests"`
	Is4K        bool   `json:"is4k"`
	IsDefault   bool   `json:"isDefault"`
}

// Config holds the instance settings.
type Config struct {
	Version               string
	MediaServerType       int // 1 Plex, 2 Jellyfin (default), 3 Emby
	LocalLogin            bool
	EmailEnabled          bool
	EnableSpecialEpisodes bool
	PartialRequests       bool
	Movie4K, Series4K     bool
	MetadataTV            string // "tmdb" or "tvdb"
	MetadataAnime         string
	Radarr, Sonarr        []DVR
	DefaultPermissions    uint64
	// Locale is the instance's display language ("" = "en"): /discover/movies and /discover/tv
	// answer in the acting user's locale, else in this one, whatever the call asks (I-61).
	Locale string
	// MinTotalPages raises the totalPages the lists report (TMDB says 1001 while page 501 fails, I-67).
	MinTotalPages int
	// Default quotas for users whose fields are nil (Seerr: settings.main.defaultQuotas).
	DefaultMovieQuotaLimit, DefaultMovieQuotaDays int
	DefaultTVQuotaLimit, DefaultTVQuotaDays       int
}

// Failure is a failure picture of a route.
type Failure struct {
	Status   int    // answer with this status
	Message  string // as {"message": Message}; with a 3xx status the body stays empty
	Location string // Location header of a 3xx answer
	Hang     bool   // never answer; the request ends when the client gives up or the fake closes
	Times    int    // answer so this many times, then as usual (0: until removed)
}

// Call is one call the fake received.
type Call struct {
	Route      string // method and pattern below /api/v1, "GET /user/{id}"
	Path       string // path below /api/v1
	Query      url.Values
	HasKey     bool // X-Api-Key was sent (right or wrong)
	HasAPIUser bool
	APIUser    string
	BodyKeys   []string // top-level keys of a JSON body
}

type mediaKey struct {
	mediaType string
	tmdbID    int
}

type media struct {
	id        int
	key       mediaKey
	tvdbID    int
	status    int
	seasons   map[int]int // season number to status
	createdAt time.Time
	updatedAt time.Time
}

// Server is the fake. Its methods are safe for concurrent use.
type Server struct {
	*httptest.Server

	stop     chan struct{}
	stopOnce sync.Once

	mu          sync.Mutex
	now         time.Time
	cfg         Config
	csrf        bool
	users       map[int]*User
	nextUser    int
	titles      map[mediaKey]*Title
	media       map[mediaKey]*media
	nextMedia   int
	requests    map[int]*Request
	nextRequest int
	failures    map[string]Failure
	calls       []Call
}

// Start is the time of the fake clock of a new fake.
var Start = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

// New starts a fake with user 1 (administrator, Jellyfin user, email admin@example.org), local
// login on, default permissions REQUEST, no default quota, no titles. It closes with the test.
// Configure DefaultPermissions = OwnerDefaultPermissions for the picture of the owner's instance.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{
		stop: make(chan struct{}), now: Start,
		cfg: Config{
			Version: "3.5.2", MediaServerType: 2, LocalLogin: true, PartialRequests: true, MetadataTV: "tmdb", MetadataAnime: "tmdb",
			DefaultPermissions: PermRequest, DefaultMovieQuotaDays: 7, DefaultTVQuotaDays: 7,
		},
		users: map[int]*User{}, nextUser: 1, titles: map[mediaKey]*Title{}, media: map[mediaKey]*media{},
		nextMedia: 1, requests: map[int]*Request{}, nextRequest: 1, failures: map[string]Failure{},
	}
	s.AddUser(User{
		ID: 1, Email: "admin@example.org", JellyfinUsername: "admin", JellyfinUserID: "jf-admin",
		UserType: UserJellyfin, Permissions: PermAdmin,
	})
	s.Server = httptest.NewServer(s.routes())
	t.Cleanup(s.Close)
	return s
}

// Close ends hanging answers and stops the server.
func (s *Server) Close() {
	s.stopOnce.Do(func() { close(s.stop) })
	s.Server.Close()
}

// Configure changes the instance settings.
func (s *Server) Configure(f func(*Config)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.cfg)
}

// SetCSRF switches the CSRF protection: on, every POST, PUT and DELETE answers 403 "invalid csrf
// token" (csurf runs before every route, appendix D 2.4).
func (s *Server) SetCSRF(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.csrf = on
}

// SetFailure makes route ("GET /user/{id}", or "*" for every route) answer with f; a zero Failure
// removes it.
func (s *Server) SetFailure(route string, f Failure) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f == (Failure{}) {
		delete(s.failures, route)
		return
	}
	s.failures[route] = f
}

// Now returns the fake clock.
func (s *Server) Now() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now
}

// Advance moves the fake clock, for example past a quota window.
func (s *Server) Advance(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = s.now.Add(d)
}

// AddUser adds u and returns its id (u.ID, or the next free id when 0). CreatedAt defaults to the
// fake clock.
func (s *Server) AddUser(u User) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u.ID == 0 {
		u.ID = s.nextUser
	}
	s.nextUser = max(s.nextUser, u.ID+1)
	if u.CreatedAt.IsZero() {
		u.CreatedAt = s.now
	}
	if u.UserType == 0 {
		u.UserType = UserLocal
	}
	s.users[u.ID] = &u
	return u.ID
}

// User returns a copy of user id.
func (s *Server) User(id int) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, false
	}
	return *u, true
}

// UserByEmail returns a copy of the user with this email (any case).
func (s *Server) UserByEmail(email string) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u := s.userByEmail(email); u != nil {
		return *u, true
	}
	return User{}, false
}

// UpdateUser changes user id in place.
func (s *Server) UpdateUser(id int, f func(*User)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[id]; ok {
		f(u)
	}
}

// RemoveUser deletes a user and its requests as Seerr does (CASCADE), without a call.
func (s *Server) RemoveUser(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeUser(id)
}

// AddTitle adds a title to the TMDB catalogue of the fake.
func (s *Server) AddTitle(t Title) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.titles[mediaKey{t.MediaType, t.TMDBID}] = &t
}

// SetMediaStatus sets the media row of a title (created when missing) and the status of seasons.
func (s *Server) SetMediaStatus(mediaType string, tmdbID, status int, seasons map[int]int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.mediaFor(mediaKey{mediaType, tmdbID}, 0)
	m.status = status
	for n, st := range seasons {
		m.seasons[n] = st
	}
	m.updatedAt = s.now
}

// MediaStatus returns the status of the media row of a title and the status per season.
func (s *Server) MediaStatus(mediaType string, tmdbID int) (int, map[int]int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.media[mediaKey{mediaType, tmdbID}]
	if !ok {
		return 0, nil, false
	}
	return m.status, copyMap(m.seasons), true
}

func copyMap(in map[int]int) map[int]int {
	out := make(map[int]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Request returns a copy of request id.
func (s *Server) Request(id int) (Request, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.requests[id]
	if !ok {
		return Request{}, false
	}
	c := *r
	c.Seasons = slices.Clone(r.Seasons)
	return c, true
}

// Requests returns copies of all requests by id.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, 0, len(s.requests))
	for _, r := range s.sortedRequests() {
		c := *r
		c.Seasons = slices.Clone(r.Seasons)
		out = append(out, c)
	}
	return out
}

// SetRequestStatus changes the status of a request, as Seerr's jobs and Radarr do.
func (s *Server) SetRequestStatus(id, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.requests[id]; ok {
		r.Status = status
		r.UpdatedAt = s.now
	}
}

// Calls returns how often route ("POST /request") was called.
func (s *Server) Calls(route string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.calls {
		if c.Route == route {
			n++
		}
	}
	return n
}

// CallLog returns every call in order.
func (s *Server) CallLog() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

// ResetCalls forgets the calls so far.
func (s *Server) ResetCalls() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = nil
}

// route describes one route: public routes need no key, query lists the parameters the request
// validator accepts.
type route struct {
	pattern string
	public  bool
	query   []string
	handle  func(s *Server, w http.ResponseWriter, r *http.Request, actor *User, body map[string]json.RawMessage)
}

var (
	pageLang      = []string{"page", "language"}
	discoverMovie = []string{"page", "language", "sortBy", "genre", "studio", "keywords", "primaryReleaseDateGte",
		"primaryReleaseDateLte", "withRuntimeGte", "withRuntimeLte", "voteAverageGte", "voteAverageLte", "voteCountGte",
		"voteCountLte", "watchProviders", "watchRegion", "certification", "certificationGte", "certificationLte",
		"certificationCountry"}
	discoverTV = []string{"page", "language", "sortBy", "genre", "network", "keywords", "firstAirDateGte",
		"firstAirDateLte", "withRuntimeGte", "withRuntimeLte", "voteAverageGte", "voteAverageLte", "voteCountGte",
		"voteCountLte", "watchProviders", "watchRegion", "status", "certification", "certificationGte",
		"certificationLte", "certificationCountry"}
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	for _, rt := range []route{
		{pattern: "GET /status", public: true, handle: (*Server).status},
		{pattern: "GET /settings/public", public: true, handle: (*Server).settingsPublic},
		{pattern: "GET /auth/me", handle: (*Server).me},
		{pattern: "GET /settings/main", handle: (*Server).settingsMain},
		{pattern: "GET /settings/metadatas", handle: (*Server).settingsMetadatas},
		{pattern: "GET /settings/radarr", handle: (*Server).settingsRadarr},
		{pattern: "GET /settings/sonarr", handle: (*Server).settingsSonarr},
		{pattern: "GET /user", query: []string{"take", "skip", "q", "sort", "sortDirection", "includeIds"}, handle: (*Server).listUsers},
		{pattern: "POST /user", handle: (*Server).createUser},
		{pattern: "GET /user/{id}", handle: (*Server).getUser},
		{pattern: "DELETE /user/{id}", handle: (*Server).deleteUser},
		{pattern: "GET /user/{id}/settings/main", handle: (*Server).getUserSettings},
		{pattern: "POST /user/{id}/settings/main", handle: (*Server).setUserSettings},
		{pattern: "POST /user/{id}/settings/password", handle: (*Server).setPassword},
		{pattern: "POST /user/{id}/settings/permissions", handle: (*Server).setPermissions},
		{pattern: "GET /user/{id}/quota", handle: (*Server).getQuota},
		{pattern: "GET /user/{id}/requests", query: []string{"take", "skip"}, handle: (*Server).userRequests},
		{pattern: "POST /request", handle: (*Server).createRequest},
		{pattern: "GET /request", query: []string{"take", "skip", "filter", "sort", "sortDirection", "requestedBy", "mediaType"}, handle: (*Server).listRequests},
		{pattern: "GET /request/{id}", handle: (*Server).getRequest},
		{pattern: "DELETE /request/{id}", handle: (*Server).deleteRequest},
		{pattern: "POST /request/{id}/{status}", handle: (*Server).decideRequest},
		{pattern: "POST /request/{id}/retry", handle: (*Server).retryRequest},
		{pattern: "GET /movie/{id}", query: []string{"language"}, handle: (*Server).getMovie},
		{pattern: "GET /tv/{id}", query: []string{"language"}, handle: (*Server).getTV},
		{pattern: "GET /discover/trending", query: []string{"page", "language", "mediaType", "timeWindow"}, handle: (*Server).trending},
		{pattern: "GET /discover/movies", query: discoverMovie, handle: discoverList("movie", false)},
		{pattern: "GET /discover/movies/upcoming", query: pageLang, handle: discoverList("movie", true)},
		{pattern: "GET /discover/tv", query: discoverTV, handle: discoverList("tv", false)},
		{pattern: "GET /discover/tv/upcoming", query: pageLang, handle: discoverList("tv", true)},
		{pattern: "GET /discover/movies/genre/{id}", query: pageLang, handle: discoverList("movie", false)},
		{pattern: "GET /discover/tv/genre/{id}", query: pageLang, handle: discoverList("tv", false)},
		{pattern: "GET /discover/genreslider/movie", query: []string{"language"}, handle: genreSlider("movie")},
		{pattern: "GET /discover/genreslider/tv", query: []string{"language"}, handle: genreSlider("tv")},
		{pattern: "GET /genres/movie", query: []string{"language"}, handle: genreList("movie")},
		{pattern: "GET /genres/tv", query: []string{"language"}, handle: genreList("tv")},
		{pattern: "GET /search", query: []string{"query", "page", "language"}, handle: (*Server).search},
	} {
		method, path, _ := strings.Cut(rt.pattern, " ")
		mux.HandleFunc(method+" /api/v1"+path, s.wrap(rt))
	}
	return mux
}

// wrap records the call, applies a failure picture, the CSRF protection, the request validator
// and the authentication, then runs the route under the lock.
func (s *Server) wrap(rt route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var body map[string]json.RawMessage
		_ = json.Unmarshal(data, &body)
		call := Call{
			Route: rt.pattern, Path: strings.TrimPrefix(r.URL.Path, "/api/v1"), Query: r.URL.Query(),
			HasKey: r.Header.Get("X-Api-Key") != "", APIUser: r.Header.Get("X-API-User"),
		}
		_, call.HasAPIUser = r.Header["X-Api-User"]
		for k := range body {
			call.BodyKeys = append(call.BodyKeys, k)
		}
		sort.Strings(call.BodyKeys)

		s.mu.Lock()
		s.calls = append(s.calls, call)
		key := rt.pattern
		f, ok := s.failures[key]
		if !ok {
			key = "*"
			f, ok = s.failures[key]
		}
		if ok && f.Times > 0 {
			if f.Times--; f.Times == 0 {
				delete(s.failures, key)
			} else {
				s.failures[key] = f
			}
		}
		csrf := s.csrf
		s.mu.Unlock()

		if ok {
			switch {
			case f.Hang:
				select {
				case <-r.Context().Done():
				case <-s.stop:
				}
				return
			case f.Status >= 300 && f.Status < 400:
				if f.Location != "" {
					w.Header().Set("Location", f.Location)
				}
				w.WriteHeader(f.Status)
				return
			default:
				writeMessage(w, f.Status, f.Message)
				return
			}
		}
		if csrf && r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeMessage(w, http.StatusForbidden, TextCSRF)
			return
		}
		for k := range r.URL.Query() {
			if !slices.Contains(rt.query, k) {
				writeMessage(w, http.StatusBadRequest, "Unknown query parameter '"+k+"'")
				return
			}
		}

		s.mu.Lock()
		defer s.mu.Unlock()
		var actor *User
		if r.Header.Get("X-Api-Key") == APIKey {
			id := 1
			if v := r.Header.Get("X-API-User"); v != "" {
				id, _ = strconv.Atoi(v)
			}
			actor = s.users[id]
		}
		switch {
		case actor != nil || rt.public:
			rt.handle(s, w, r, actor, body)
		case r.Header.Get("X-Api-Key") == "":
			// Without a key Seerr asks for its session cookie (appendix I, I-2).
			writeJSON(w, http.StatusUnauthorized, map[string]any{"message": TextNoSession, "errors": []any{TextNoSession}})
		default:
			// A wrong key and an unknown X-API-User get the same answer (I-2, I-3).
			writeDenied(w)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeMessage(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"message": msg, "errors": []any{}})
}

// writeDenied is the answer of Seerr's authentication middleware: the text in the field error, not
// message (appendix I, I-2).
func writeDenied(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, map[string]any{"status": http.StatusForbidden, "error": TextAccessEndpoint})
}

func pathInt(r *http.Request, name string) int {
	n, _ := strconv.Atoi(r.PathValue(name))
	return n
}

func queryInt(r *http.Request, name string, def int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil {
		return n
	}
	return def
}

// decode reads key of body into dst; a missing key leaves dst unchanged and reports false.
func decode(body map[string]json.RawMessage, key string, dst any) bool {
	raw, ok := body[key]
	if !ok {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

func stamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// Connection and settings.

func (s *Server) status(w http.ResponseWriter, _ *http.Request, _ *User, _ map[string]json.RawMessage) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version": s.cfg.Version, "commitTag": "v" + s.cfg.Version, "updateAvailable": false,
		"commitsBehind": 0, "restartRequired": false,
	})
}

func (s *Server) settingsPublic(w http.ResponseWriter, _ *http.Request, _ *User, _ map[string]json.RawMessage) {
	writeJSON(w, http.StatusOK, map[string]any{
		"initialized": true, "applicationTitle": "Seerr", "applicationUrl": "", "localLogin": s.cfg.LocalLogin,
		"mediaServerType": s.cfg.MediaServerType, "partialRequestsEnabled": s.cfg.PartialRequests,
		"enableSpecialEpisodes": s.cfg.EnableSpecialEpisodes, "movie4kEnabled": s.cfg.Movie4K,
		"series4kEnabled": s.cfg.Series4K, "emailEnabled": s.cfg.EmailEnabled, "userEmailRequired": false,
		"locale": "en",
	})
}

func (s *Server) adminOnly(w http.ResponseWriter, actor *User) bool {
	if !actor.has(PermAdmin) {
		writeDenied(w)
		return false
	}
	return true
}

// settingsMain answers the part of GET /settings/main the integration reads, with further fields of
// the real answer (appendix I, I-7).
func (s *Server) settingsMain(w http.ResponseWriter, _ *http.Request, actor *User, _ map[string]json.RawMessage) {
	if s.adminOnly(w, actor) {
		writeJSON(w, http.StatusOK, map[string]any{
			"applicationTitle": "Seerr", "applicationUrl": "", "defaultPermissions": s.cfg.DefaultPermissions,
			"defaultQuotas": map[string]any{
				"movie": map[string]any{"quotaLimit": s.cfg.DefaultMovieQuotaLimit, "quotaDays": s.cfg.DefaultMovieQuotaDays},
				"tv":    map[string]any{"quotaLimit": s.cfg.DefaultTVQuotaLimit, "quotaDays": s.cfg.DefaultTVQuotaDays},
			},
			"hideAvailable": false, "localLogin": s.cfg.LocalLogin, "mediaServerType": s.cfg.MediaServerType,
			"partialRequestsEnabled": s.cfg.PartialRequests, "enableSpecialEpisodes": s.cfg.EnableSpecialEpisodes,
			"locale": "en",
		})
	}
}

func (s *Server) settingsMetadatas(w http.ResponseWriter, _ *http.Request, actor *User, _ map[string]json.RawMessage) {
	if s.adminOnly(w, actor) {
		writeJSON(w, http.StatusOK, map[string]any{"tv": s.cfg.MetadataTV, "anime": s.cfg.MetadataAnime})
	}
}

func (s *Server) settingsRadarr(w http.ResponseWriter, _ *http.Request, actor *User, _ map[string]json.RawMessage) {
	if s.adminOnly(w, actor) {
		writeJSON(w, http.StatusOK, append([]DVR{}, s.cfg.Radarr...))
	}
}

func (s *Server) settingsSonarr(w http.ResponseWriter, _ *http.Request, actor *User, _ map[string]json.RawMessage) {
	if s.adminOnly(w, actor) {
		writeJSON(w, http.StatusOK, append([]DVR{}, s.cfg.Sonarr...))
	}
}

// Users.

func (s *Server) userJSON(u *User, full bool) map[string]any {
	display := u.Username
	if display == "" {
		display = u.JellyfinUsername
	}
	if display == "" {
		display = u.Email
	}
	out := map[string]any{
		"id": u.ID, "username": u.Username, "jellyfinUsername": u.JellyfinUsername, "plexUsername": nullable(u.PlexUsername),
		"displayName": display, "userType": u.UserType, "permissions": u.Permissions,
		"avatar": "/avatar/" + strconv.Itoa(u.ID), "requestCount": s.requestCount(u.ID),
		"movieQuotaLimit": u.MovieQuotaLimit, "movieQuotaDays": u.MovieQuotaDays,
		"tvQuotaLimit": u.TVQuotaLimit, "tvQuotaDays": u.TVQuotaDays,
		"createdAt": stamp(u.CreatedAt), "updatedAt": stamp(u.CreatedAt),
	}
	if full {
		out["email"] = u.Email
		out["jellyfinUserId"] = u.JellyfinUserID
	}
	return out
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (s *Server) requestCount(userID int) int {
	n := 0
	for _, r := range s.requests {
		if r.UserID == userID {
			n++
		}
	}
	return n
}

func (s *Server) userByEmail(email string) *User {
	for _, u := range s.users {
		if u.Email != "" && strings.EqualFold(u.Email, email) {
			return u
		}
	}
	return nil
}

func (s *Server) sortedUsers() []*User {
	out := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Server) me(w http.ResponseWriter, _ *http.Request, actor *User, _ map[string]json.RawMessage) {
	writeJSON(w, http.StatusOK, s.userJSON(actor, true))
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
	if !actor.has(PermManageUsers) {
		writeDenied(w)
		return
	}
	q := strings.ToLower(r.URL.Query().Get("q"))
	var hits []map[string]any
	for _, u := range s.sortedUsers() {
		if q != "" && !strings.Contains(strings.ToLower(u.Username), q) && !strings.Contains(strings.ToLower(u.Email), q) &&
			!strings.Contains(strings.ToLower(u.JellyfinUsername), q) {
			continue
		}
		hits = append(hits, s.userJSON(u, true))
	}
	take, skip := queryInt(r, "take", 10), queryInt(r, "skip", 0)
	writeJSON(w, http.StatusOK, map[string]any{"pageInfo": pageInfo(len(hits), take, skip), "results": window(hits, take, skip)})
}

func pageInfo(total, take, skip int) map[string]any {
	take = max(take, 1)
	return map[string]any{"pages": (total + take - 1) / take, "pageSize": take, "results": total, "page": skip/take + 1}
}

func window[T any](all []T, take, skip int) []T {
	if skip >= len(all) {
		return []T{}
	}
	return all[skip:min(len(all), skip+max(take, 1))]
}

func (s *Server) createUser(w http.ResponseWriter, _ *http.Request, actor *User, body map[string]json.RawMessage) {
	if !actor.has(PermManageUsers) {
		writeDenied(w)
		return
	}
	var email, username, password string
	decode(body, "email", &email)
	decode(body, "username", &username)
	decode(body, "password", &password)
	if email == "" {
		email = username
	}
	switch {
	case email == "":
		writeMessage(w, http.StatusBadRequest, "request/body/email must be string")
		return
	case s.userByEmail(email) != nil:
		writeMessage(w, http.StatusConflict, TextUserExists)
		return
	case password == "" && !s.cfg.EmailEnabled:
		writeMessage(w, http.StatusInternalServerError, "Email notifications must be enabled")
		return
	}
	// permissions of the body are ignored: new users get the default permissions.
	u := &User{
		ID: s.nextUser, Email: strings.ToLower(email), Username: username, UserType: UserLocal,
		Permissions: s.cfg.DefaultPermissions, Password: password, CreatedAt: s.now,
	}
	s.nextUser++
	s.users[u.ID] = u
	// Seerr 3.5.1 answers without the email (appendix I, I-12).
	out := s.userJSON(u, true)
	delete(out, "email")
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) target(w http.ResponseWriter, r *http.Request) *User {
	u, ok := s.users[pathInt(r, "id")]
	if !ok {
		writeMessage(w, http.StatusNotFound, "User not found.")
		return nil
	}
	return u
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
	if u := s.target(w, r); u != nil {
		writeJSON(w, http.StatusOK, s.userJSON(u, u.ID == actor.ID || actor.has(PermManageUsers)))
	}
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
	if !actor.has(PermManageUsers) {
		writeDenied(w)
		return
	}
	u := s.target(w, r)
	switch {
	case u == nil:
		return
	case u.ID == 1:
		writeMessage(w, http.StatusMethodNotAllowed, "This account cannot be deleted.")
		return
	case u.has(PermAdmin) && actor.ID != 1:
		writeMessage(w, http.StatusMethodNotAllowed, "You cannot delete users with administrative privileges.")
		return
	}
	out := s.userJSON(u, true)
	s.removeUser(u.ID)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) removeUser(id int) {
	delete(s.users, id)
	for rid, r := range s.requests {
		if r.UserID == id {
			delete(s.requests, rid)
		}
	}
}

func (s *Server) ownOrManager(w http.ResponseWriter, actor, u *User) bool {
	if actor.ID != u.ID && !actor.has(PermManageUsers) {
		writeDenied(w)
		return false
	}
	return true
}

func (s *Server) getUserSettings(w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
	u := s.target(w, r)
	if u == nil || !s.ownOrManager(w, actor, u) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username": u.Username, "email": u.Email, "discordId": u.DiscordID, "locale": u.Locale,
		"discoverRegion": u.DiscoverRegion, "streamingRegion": u.StreamingRegion,
		"originalLanguage": u.OriginalLanguage, "movieQuotaLimit": u.MovieQuotaLimit,
		"movieQuotaDays": u.MovieQuotaDays, "tvQuotaLimit": u.TVQuotaLimit, "tvQuotaDays": u.TVQuotaDays,
		"watchlistSyncMovies": false, "watchlistSyncTv": false,
		"globalMovieQuotaLimit": s.cfg.DefaultMovieQuotaLimit, "globalMovieQuotaDays": s.cfg.DefaultMovieQuotaDays,
		"globalTvQuotaLimit": s.cfg.DefaultTVQuotaLimit, "globalTvQuotaDays": s.cfg.DefaultTVQuotaDays,
	})
}

// setUserSettings writes every field from the body and clears the missing ones, as versions before
// 3.5.1 do (appendix H 3.1; 3.5.1 keeps a missing username and email, appendix I, I-17: the
// stricter picture proves that the client sends everything back); the quota fields only when the
// target has no MANAGE_USERS and is not the caller. The email of another user (any spelling) is
// 400 INVALID_EMAIL and changes nothing (I-18); the answer has no quota fields (I-16).
func (s *Server) setUserSettings(w http.ResponseWriter, r *http.Request, actor *User, body map[string]json.RawMessage) {
	u := s.target(w, r)
	if u == nil || !s.ownOrManager(w, actor, u) {
		return
	}
	str := func(key string) string {
		var v string
		decode(body, key, &v)
		return v
	}
	if other := s.userByEmail(str("email")); other != nil && other.ID != u.ID {
		writeMessage(w, http.StatusBadRequest, TextInvalidEmail)
		return
	}
	num := func(key string) *int {
		var v *int
		decode(body, key, &v)
		return v
	}
	u.Username, u.Email, u.DiscordID, u.Locale = str("username"), str("email"), str("discordId"), str("locale")
	u.DiscoverRegion, u.StreamingRegion, u.OriginalLanguage = str("discoverRegion"), str("streamingRegion"), str("originalLanguage")
	if !u.has(PermManageUsers) && actor.ID != u.ID {
		u.MovieQuotaLimit, u.MovieQuotaDays = num("movieQuotaLimit"), num("movieQuotaDays")
		u.TVQuotaLimit, u.TVQuotaDays = num("tvQuotaLimit"), num("tvQuotaDays")
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username": u.Username, "email": u.Email, "locale": u.Locale, "discoverRegion": u.DiscoverRegion,
		"streamingRegion": u.StreamingRegion, "originalLanguage": u.OriginalLanguage,
		"watchlistSyncMovies": false, "watchlistSyncTv": false,
	})
}

// setPassword: user 1 changes only its own password and needs the current one; an administrator's
// password changes only the administrator or user 1; a caller with MANAGE_USERS sets another
// user's password without the current one (appendix H 3.1).
func (s *Server) setPassword(w http.ResponseWriter, r *http.Request, actor *User, body map[string]json.RawMessage) {
	u := s.target(w, r)
	if u == nil {
		return
	}
	var current, next string
	decode(body, "currentPassword", &current)
	decode(body, "newPassword", &next)
	switch {
	case len(next) < 8:
		writeMessage(w, http.StatusBadRequest, "Password must be at least 8 characters.")
	case u.ID == 1 && actor.ID != 1, u.has(PermAdmin) && actor.ID != u.ID && actor.ID != 1:
		writeMessage(w, http.StatusForbidden, "You do not have permission to modify this user's password.")
	case actor.ID == u.ID:
		if current == "" || current != u.Password {
			writeMessage(w, http.StatusForbidden, "Incorrect current password.")
			return
		}
		u.Password = next
		w.WriteHeader(http.StatusNoContent)
	case !actor.has(PermManageUsers):
		writeDenied(w)
	default:
		u.Password = next
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) setPermissions(w http.ResponseWriter, r *http.Request, actor *User, body map[string]json.RawMessage) {
	if !actor.has(PermManageUsers) {
		writeDenied(w)
		return
	}
	u := s.target(w, r)
	if u == nil {
		return
	}
	var p uint64
	if !decode(body, "permissions", &p) {
		writeMessage(w, http.StatusBadRequest, "request/body must have required property 'permissions'")
		return
	}
	if u.ID == 1 || (p&PermAdmin != 0 && actor.ID != 1) {
		writeMessage(w, http.StatusForbidden, "You do not have permission to modify this user's permissions.")
		return
	}
	u.Permissions = p
	writeJSON(w, http.StatusOK, map[string]any{"permissions": p})
}

// Quota (appendix H 3.2).

type quotaPart struct {
	days, limit, used int
}

func (q quotaPart) json() map[string]any {
	out := map[string]any{"days": q.days, "limit": q.limit, "used": q.used, "restricted": q.limit > 0 && q.limit-q.used <= 0}
	if q.limit > 0 {
		out["remaining"] = max(0, q.limit-q.used)
	}
	return out
}

func (q quotaPart) remaining() int { return max(0, q.limit-q.used) }

func orDefault(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

// quota counts the requests of u created within the last days: movies per request, series per
// season, declined requests not; users with MANAGE_USERS have no limit.
func (s *Server) quota(u *User) (movie, tv quotaPart) {
	movie = quotaPart{days: orDefault(u.MovieQuotaDays, s.cfg.DefaultMovieQuotaDays), limit: orDefault(u.MovieQuotaLimit, s.cfg.DefaultMovieQuotaLimit)}
	tv = quotaPart{days: orDefault(u.TVQuotaDays, s.cfg.DefaultTVQuotaDays), limit: orDefault(u.TVQuotaLimit, s.cfg.DefaultTVQuotaLimit)}
	if u.has(PermManageUsers) {
		movie.limit, tv.limit = 0, 0
	}
	for _, r := range s.requests {
		if r.UserID != u.ID || r.Status == RequestDeclined {
			continue
		}
		switch {
		case r.MediaType == "movie" && movie.limit > 0 && r.CreatedAt.After(s.now.AddDate(0, 0, -movie.days)):
			movie.used++
		case r.MediaType == "tv" && tv.limit > 0 && r.CreatedAt.After(s.now.AddDate(0, 0, -tv.days)):
			tv.used += len(r.Seasons)
		}
	}
	return movie, tv
}

func (s *Server) getQuota(w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
	u := s.target(w, r)
	if u == nil {
		return
	}
	if actor.ID != u.ID && !actor.has(PermManageUsers|PermManageRequests) {
		writeMessage(w, http.StatusForbidden, "You do not have permission to view this user's quota.")
		return
	}
	movie, tv := s.quota(u)
	writeJSON(w, http.StatusOK, map[string]any{"movie": movie.json(), "tv": tv.json()})
}

// Requests (appendix D 3, 4, 6, 8).

func (s *Server) mediaFor(k mediaKey, tvdbID int) *media {
	m, ok := s.media[k]
	if !ok {
		m = &media{id: s.nextMedia, key: k, tvdbID: tvdbID, status: MediaUnknown, seasons: map[int]int{}, createdAt: s.now, updatedAt: s.now}
		s.nextMedia++
		s.media[k] = m
	}
	if tvdbID != 0 {
		m.tvdbID = tvdbID
	}
	return m
}

func (s *Server) mediaJSON(m *media, withRequests bool) map[string]any {
	var seasons []map[string]any
	nums := make([]int, 0, len(m.seasons))
	for n := range m.seasons {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	for _, n := range nums {
		seasons = append(seasons, map[string]any{"id": m.id*1000 + n, "seasonNumber": n, "status": m.seasons[n], "status4k": MediaUnknown})
	}
	out := map[string]any{
		"id": m.id, "tmdbId": m.key.tmdbID, "mediaType": m.key.mediaType, "status": m.status, "status4k": MediaUnknown,
		"seasons": seasons, "createdAt": stamp(m.createdAt), "updatedAt": stamp(m.updatedAt),
	}
	if m.tvdbID != 0 {
		out["tvdbId"] = m.tvdbID
	} else {
		out["tvdbId"] = nil
	}
	if withRequests {
		reqs := []map[string]any{}
		for _, r := range s.sortedRequests() {
			if (mediaKey{r.MediaType, r.TMDBID}) == m.key {
				reqs = append(reqs, s.requestJSON(r, false))
			}
		}
		out["requests"] = reqs
	}
	return out
}

func (s *Server) requestJSON(r *Request, withMedia bool) map[string]any {
	seasons := []map[string]any{}
	for _, n := range r.Seasons {
		seasons = append(seasons, map[string]any{"id": r.ID*1000 + n, "seasonNumber": n, "status": r.Status})
	}
	out := map[string]any{
		"id": r.ID, "status": r.Status, "type": r.MediaType, "is4k": false, "seasons": seasons,
		"createdAt": stamp(r.CreatedAt), "updatedAt": stamp(r.UpdatedAt),
	}
	if u, ok := s.users[r.UserID]; ok {
		out["requestedBy"] = s.userJSON(u, false)
	}
	if u, ok := s.users[r.ModifiedBy]; ok {
		out["modifiedBy"] = s.userJSON(u, false)
	}
	if m, ok := s.media[mediaKey{r.MediaType, r.TMDBID}]; ok && withMedia {
		out["media"] = s.mediaJSON(m, false)
	}
	return out
}

func (s *Server) sortedRequests() []*Request {
	out := make([]*Request, 0, len(s.requests))
	for _, r := range s.requests {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Server) openRequests(k mediaKey) []*Request {
	var out []*Request
	for _, r := range s.sortedRequests() {
		if (mediaKey{r.MediaType, r.TMDBID}) == k && r.Status != RequestDeclined && r.Status != RequestCompleted {
			out = append(out, r)
		}
	}
	return out
}

// createRequest follows MediaRequest.request of Seerr v3.5.2: the rights and the quota of the
// request user, auto-approval by the caller (X-API-User), duplicates, the season filter and 202.
func (s *Server) createRequest(w http.ResponseWriter, _ *http.Request, actor *User, body map[string]json.RawMessage) {
	var mediaType string
	var tmdbID, tvdbID, userID int
	if !decode(body, "mediaType", &mediaType) || !decode(body, "mediaId", &tmdbID) {
		writeMessage(w, http.StatusBadRequest, "request/body must have required property 'mediaType'")
		return
	}
	decode(body, "tvdbId", &tvdbID)
	user := actor
	if decode(body, "userId", &userID) && userID != 0 {
		if !actor.has(PermManageUsers | PermManageRequests) {
			writeMessage(w, http.StatusForbidden, TextModifyUser)
			return
		}
		if user = s.users[userID]; user == nil {
			writeMessage(w, http.StatusInternalServerError, "Could not find any entity of type \"User\"")
			return
		}
	}
	movie := mediaType == "movie"
	switch {
	case movie && !user.hasAny(PermRequest, PermRequestMovie):
		writeMessage(w, http.StatusForbidden, TextNoMovieRight)
		return
	case !movie && !user.hasAny(PermRequest, PermRequestTV):
		writeMessage(w, http.StatusForbidden, TextNoSeriesRight)
		return
	}
	mq, tq := s.quota(user)
	if movie && mq.limit > 0 && mq.remaining() == 0 {
		writeMessage(w, http.StatusForbidden, TextMovieQuota)
		return
	}
	if !movie && tq.limit > 0 && tq.remaining() == 0 {
		writeMessage(w, http.StatusForbidden, TextSeriesQuota)
		return
	}
	k := mediaKey{mediaType, tmdbID}
	title, ok := s.titles[k]
	if !ok {
		writeMessage(w, http.StatusInternalServerError, "Unable to retrieve media from TMDB.")
		return
	}
	if m, ok := s.media[k]; ok && m.status == MediaBlocklisted {
		writeMessage(w, http.StatusForbidden, TextBlocklisted)
		return
	}
	var seasons []int
	if movie {
		if len(s.openRequests(k)) > 0 {
			writeMessage(w, http.StatusConflict, TextDuplicate)
			return
		}
	} else {
		seasons = s.wantedSeasons(title, body)
		if len(seasons) == 0 {
			writeMessage(w, http.StatusAccepted, TextNoSeasons)
			return
		}
		if tq.limit > 0 && tq.remaining() < len(seasons) {
			writeMessage(w, http.StatusForbidden, TextSeriesQuota)
			return
		}
	}
	m := s.mediaFor(k, tvdbID)
	if m.status == MediaUnknown || m.status == MediaDeleted {
		m.status = MediaPending
	}
	req := &Request{ID: s.nextRequest, MediaType: mediaType, TMDBID: tmdbID, UserID: user.ID, Status: RequestPending, Seasons: seasons, CreatedAt: s.now, UpdatedAt: s.now}
	s.nextRequest++
	auto := PermAutoApproveMovie
	if !movie {
		auto = PermAutoApproveTV
	}
	if actor.hasAny(PermAutoApprove, auto, PermManageRequests) {
		req.Status, req.ModifiedBy = RequestApproved, actor.ID
		s.approved(m)
	}
	m.updatedAt = s.now
	s.requests[req.ID] = req
	writeJSON(w, http.StatusCreated, s.requestJSON(req, true))
}

// wantedSeasons resolves seasons ([]int or "all") and drops season 0 without special episodes,
// seasons in an open request and seasons the media row has (status other than unknown or deleted).
func (s *Server) wantedSeasons(t *Title, body map[string]json.RawMessage) []int {
	var nums []int
	var all string
	if decode(body, "seasons", &all) && all == "all" {
		for _, se := range t.Seasons {
			if se.Number > 0 && se.Episodes > 0 {
				nums = append(nums, se.Number)
			}
		}
	} else {
		decode(body, "seasons", &nums)
	}
	k := mediaKey{t.MediaType, t.TMDBID}
	taken := map[int]bool{}
	for _, r := range s.openRequests(k) {
		for _, n := range r.Seasons {
			taken[n] = true
		}
	}
	if m, ok := s.media[k]; ok {
		for n, st := range m.seasons {
			if st != MediaUnknown && st != MediaDeleted {
				taken[n] = true
			}
		}
	}
	var out []int
	for _, n := range nums {
		if (n == 0 && !s.cfg.EnableSpecialEpisodes) || taken[n] || slices.Contains(out, n) {
			continue
		}
		out = append(out, n)
	}
	sort.Ints(out)
	return out
}

func (s *Server) approved(m *media) {
	switch m.status {
	case MediaAvailable, MediaPartiallyAvailable, MediaProcessing:
	default:
		m.status = MediaProcessing
	}
}

func (s *Server) canView(actor *User, r *Request) bool {
	return r.UserID == actor.ID || actor.hasAny(PermManageRequests, PermRequestView)
}

func (s *Server) listRequests(w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
	q := r.URL.Query()
	by := queryInt(r, "requestedBy", 0)
	if by != 0 && by != actor.ID && !actor.hasAny(PermManageRequests, PermRequestView) {
		writeMessage(w, http.StatusForbidden, "You do not have permission to view this user's requests.")
		return
	}
	statuses := map[string][]int{
		"pending": {RequestPending}, "approved": {RequestApproved}, "processing": {RequestApproved},
		"failed": {RequestFailed}, "completed": {RequestCompleted}, "available": {RequestCompleted},
		"unavailable": {RequestPending, RequestApproved}, "declined": {RequestDeclined},
	}[q.Get("filter")]
	var hits []*Request
	for _, req := range s.sortedRequests() {
		m := s.media[mediaKey{req.MediaType, req.TMDBID}]
		switch {
		case m != nil && m.status == MediaBlocklisted:
		case by != 0 && req.UserID != by:
		case by == 0 && !actor.hasAny(PermManageRequests, PermRequestView) && req.UserID != actor.ID:
		case statuses != nil && !slices.Contains(statuses, req.Status):
		case q.Get("mediaType") != "" && q.Get("mediaType") != "all" && q.Get("mediaType") != req.MediaType:
		default:
			hits = append(hits, req)
		}
	}
	key := func(x *Request) (time.Time, int) { return time.Time{}, x.ID }
	switch q.Get("sort") {
	case "modified":
		key = func(x *Request) (time.Time, int) { return x.UpdatedAt, x.ID }
	case "added":
		key = func(x *Request) (time.Time, int) { return x.CreatedAt, x.ID }
	}
	asc := q.Get("sortDirection") == "asc"
	sort.SliceStable(hits, func(i, j int) bool {
		ti, ii := key(hits[i])
		tj, ij := key(hits[j])
		if asc {
			return ti.Before(tj) || (ti.Equal(tj) && ii < ij)
		}
		return tj.Before(ti) || (ti.Equal(tj) && ij < ii)
	})
	take, skip := queryInt(r, "take", 10), queryInt(r, "skip", 0)
	out := []map[string]any{}
	for _, req := range window(hits, take, skip) {
		out = append(out, s.requestJSON(req, true))
	}
	writeJSON(w, http.StatusOK, map[string]any{"pageInfo": pageInfo(len(hits), take, skip), "results": out, "serviceErrors": map[string]any{}})
}

func (s *Server) userRequests(w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
	u := s.target(w, r)
	if u == nil {
		return
	}
	if u.ID != actor.ID && !actor.hasAny(PermManageRequests, PermRequestView) {
		writeMessage(w, http.StatusForbidden, "You do not have permission to view this user's requests.")
		return
	}
	var hits []*Request
	all := s.sortedRequests()
	for i := len(all) - 1; i >= 0; i-- { // id DESC
		if all[i].UserID == u.ID {
			hits = append(hits, all[i])
		}
	}
	take, skip := queryInt(r, "take", 20), queryInt(r, "skip", 0)
	out := []map[string]any{}
	for _, req := range window(hits, take, skip) {
		out = append(out, s.requestJSON(req, true))
	}
	writeJSON(w, http.StatusOK, map[string]any{"pageInfo": pageInfo(len(hits), take, skip), "results": out})
}

func (s *Server) findRequest(w http.ResponseWriter, r *http.Request) *Request {
	req, ok := s.requests[pathInt(r, "id")]
	if !ok {
		writeMessage(w, http.StatusNotFound, "Request not found.")
		return nil
	}
	return req
}

func (s *Server) getRequest(w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
	req := s.findRequest(w, r)
	switch {
	case req == nil:
	case !s.canView(actor, req):
		writeMessage(w, http.StatusForbidden, "You do not have permission to view this request.")
	default:
		writeJSON(w, http.StatusOK, s.requestJSON(req, true))
	}
}

func (s *Server) deleteRequest(w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
	req := s.findRequest(w, r)
	switch {
	case req == nil:
	case !actor.has(PermManageRequests) && (req.UserID != actor.ID || req.Status != RequestPending):
		writeMessage(w, http.StatusUnauthorized, TextDeleteRequest)
	default:
		delete(s.requests, req.ID)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) decideRequest(w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
	status := r.PathValue("status")
	if status != "approve" && status != "decline" {
		writeMessage(w, http.StatusBadRequest, "Status must be approve or decline.")
		return
	}
	if !actor.has(PermManageRequests) {
		writeDenied(w)
		return
	}
	req := s.findRequest(w, r)
	switch {
	case req == nil:
		return
	case req.Status != RequestPending:
		writeMessage(w, http.StatusConflict, TextOnlyPending)
		return
	}
	m := s.mediaFor(mediaKey{req.MediaType, req.TMDBID}, 0)
	if status == "approve" {
		req.Status = RequestApproved
		s.approved(m)
	} else {
		req.Status = RequestDeclined
		if m.status != MediaDeleted {
			m.status = MediaUnknown
		}
	}
	req.ModifiedBy, req.UpdatedAt, m.updatedAt = actor.ID, s.now, s.now
	writeJSON(w, http.StatusOK, s.requestJSON(req, true))
}

// retryRequest sends a FAILED request again: approved, the media processing; another status is 409.
func (s *Server) retryRequest(w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
	if !actor.has(PermManageRequests) {
		writeDenied(w)
		return
	}
	req := s.findRequest(w, r)
	switch {
	case req == nil:
		return
	case req.Status != RequestFailed:
		writeMessage(w, http.StatusConflict, TextOnlyFailed)
		return
	}
	m := s.mediaFor(mediaKey{req.MediaType, req.TMDBID}, 0)
	req.Status = RequestApproved
	s.approved(m)
	req.ModifiedBy, req.UpdatedAt, m.updatedAt = actor.ID, s.now, s.now
	writeJSON(w, http.StatusOK, s.requestJSON(req, true))
}

// Titles, discover and search.

func (s *Server) titleJSON(t *Title) map[string]any { return s.titleJSONIn(t, "") }

func (t *Title) originalLanguage() string {
	if t.OriginalLanguage == "" {
		return "en"
	}
	return t.OriginalLanguage
}

func (t *Title) date() string {
	if t.Released != "" {
		return t.Released
	}
	if t.Year > 0 {
		return strconv.Itoa(t.Year) + "-01-01"
	}
	return ""
}

// titleJSONIn is the title with its name in lang (Names), else Name.
func (s *Server) titleJSONIn(t *Title, lang string) map[string]any {
	out := map[string]any{
		"id": t.TMDBID, "mediaType": t.MediaType, "overview": "Overview of " + t.Name,
		"posterPath": "/poster" + strconv.Itoa(t.TMDBID) + ".jpg", "backdropPath": "/backdrop" + strconv.Itoa(t.TMDBID) + ".jpg",
		"genreIds": slices.Clone(t.GenreIDs), "popularity": t.Popularity, "voteAverage": t.VoteAverage, "voteCount": t.VoteCount,
		"originalLanguage": t.originalLanguage(),
	}
	name := t.Name
	if n, ok := t.Names[lang]; ok && lang != "" {
		name = n
	}
	date := t.date()
	switch t.MediaType {
	case "movie":
		out["title"], out["originalTitle"], out["releaseDate"] = name, t.Name, date
	case "tv":
		out["name"], out["originalName"], out["firstAirDate"] = name, t.Name, date
	default:
		out["name"] = name
	}
	if m, ok := s.media[mediaKey{t.MediaType, t.TMDBID}]; ok {
		out["mediaInfo"] = s.mediaJSON(m, false)
	}
	return out
}

func (s *Server) details(w http.ResponseWriter, r *http.Request, mediaType string) (*Title, map[string]any) {
	t, ok := s.titles[mediaKey{mediaType, pathInt(r, "id")}]
	if !ok {
		writeMessage(w, http.StatusNotFound, "Unable to retrieve "+mediaType+".")
		return nil, nil
	}
	out := s.titleJSON(t)
	ext := map[string]any{"imdbId": nil, "tvdbId": nil}
	if t.IMDBID != "" {
		ext["imdbId"] = t.IMDBID
	}
	if t.TVDBID != 0 {
		ext["tvdbId"] = t.TVDBID
	}
	out["externalIds"] = ext
	if m, ok := s.media[mediaKey{mediaType, t.TMDBID}]; ok {
		out["mediaInfo"] = s.mediaJSON(m, true)
	}
	return t, out
}

func (s *Server) getMovie(w http.ResponseWriter, r *http.Request, _ *User, _ map[string]json.RawMessage) {
	if _, out := s.details(w, r, "movie"); out != nil {
		writeJSON(w, http.StatusOK, out)
	}
}

func (s *Server) getTV(w http.ResponseWriter, r *http.Request, _ *User, _ map[string]json.RawMessage) {
	t, out := s.details(w, r, "tv")
	if out == nil {
		return
	}
	seasons := []map[string]any{}
	for _, se := range t.Seasons {
		seasons = append(seasons, map[string]any{
			"id": t.TMDBID*100 + se.Number, "seasonNumber": se.Number, "episodeCount": se.Episodes,
			"name": "Season " + strconv.Itoa(se.Number), "airDate": nil,
		})
	}
	out["seasons"] = seasons
	writeJSON(w, http.StatusOK, out)
}

// pageSize is TMDB's page size.
const pageSize = 20

// page answers one page of titles in the display language lang, sorted by sortBy: the release
// date or the vote average descending, else (popularity.desc, and every value Seerr does not
// know, title.asc and name.asc among them: I-63, I-68) by popularity. Page 501 and above answer
// 500 like TMDB (I-67).
func (s *Server) page(w http.ResponseWriter, r *http.Request, titles []*Title, lang, sortBy string) {
	less := func(a, b *Title) bool { return a.Popularity > b.Popularity }
	switch sortBy {
	case "primary_release_date.desc", "first_air_date.desc":
		less = func(a, b *Title) bool { return a.date() > b.date() }
	case "vote_average.desc":
		less = func(a, b *Title) bool { return a.VoteAverage > b.VoteAverage }
	}
	sort.SliceStable(titles, func(i, j int) bool {
		if less(titles[i], titles[j]) != less(titles[j], titles[i]) {
			return less(titles[i], titles[j])
		}
		return titles[i].TMDBID < titles[j].TMDBID
	})
	page := max(queryInt(r, "page", 1), 1)
	if page > 500 {
		writeMessage(w, http.StatusInternalServerError, "Unable to retrieve popular movies.")
		return
	}
	results := []map[string]any{}
	for _, t := range window(titles, pageSize, (page-1)*pageSize) {
		results = append(results, s.titleJSONIn(t, lang))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"page": page, "totalPages": max((len(titles)+pageSize-1)/pageSize, s.cfg.MinTotalPages), "totalResults": len(titles), "results": results,
	})
}

func (s *Server) catalogue(keep func(*Title) bool) []*Title {
	var out []*Title
	for _, t := range s.titles {
		if keep(t) {
			out = append(out, t)
		}
	}
	return out
}

func (s *Server) trending(w http.ResponseWriter, r *http.Request, _ *User, _ map[string]json.RawMessage) {
	mt := r.URL.Query().Get("mediaType")
	s.page(w, r, s.catalogue(func(t *Title) bool {
		return t.MediaType != "person" && (mt == "" || mt == "all" || mt == t.MediaType)
	}), r.URL.Query().Get("language"), "")
}

// discoverList answers /discover/movies|tv (filtered: genre, certificationLte, the release date
// and vote count, sortBy, and language as the original language with the display language of the
// acting user or the instance, I-60, I-61), their upcoming lists and the lists by genre (genre id
// in the path); those two take language as the display language.
func discoverList(mediaType string, upcoming bool) func(*Server, http.ResponseWriter, *http.Request, *User, map[string]json.RawMessage) {
	return func(s *Server, w http.ResponseWriter, r *http.Request, actor *User, _ map[string]json.RawMessage) {
		q := r.URL.Query()
		filtered := !upcoming && pathInt(r, "id") == 0
		lang, sortBy, original := q.Get("language"), "", ""
		before, votes := q.Get("primaryReleaseDateLte")+q.Get("firstAirDateLte"), queryInt(r, "voteCountGte", 0)
		if filtered {
			original, sortBy = lang, q.Get("sortBy")
			lang = s.cfg.Locale
			if actor != nil && actor.Locale != "" {
				lang = actor.Locale
			}
			if lang == "" {
				lang = "en"
			}
		}
		var genres []int
		for g := range strings.SplitSeq(r.URL.Query().Get("genre"), ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(g)); err == nil {
				genres = append(genres, n)
			}
		}
		if id := pathInt(r, "id"); id != 0 {
			genres = []int{id}
		}
		lte, hasLte := -1, false
		if v := r.URL.Query().Get("certificationLte"); v != "" {
			lte, _ = strconv.Atoi(v)
			hasLte = true
		}
		s.page(w, r, s.catalogue(func(t *Title) bool {
			if t.MediaType != mediaType || (upcoming && !t.Upcoming) {
				return false
			}
			if len(genres) > 0 && !slices.ContainsFunc(genres, func(g int) bool { return slices.Contains(t.GenreIDs, g) }) {
				return false
			}
			if original != "" && t.originalLanguage() != original {
				return false
			}
			if before != "" && t.date() > before || t.VoteCount < votes {
				return false
			}
			if hasLte && !t.IgnoresCertification {
				c, err := strconv.Atoi(t.Certification)
				return err == nil && c <= lte
			}
			return true
		}), lang, sortBy)
	}
}

// genreNames are TMDB genre names the slider shows.
var genreNames = map[int]string{12: "Adventure", 16: "Animation", 18: "Drama", 28: "Action", 35: "Comedy", 99: "Documentary", 10765: "Sci-Fi & Fantasy"}

func genreSlider(mediaType string) func(*Server, http.ResponseWriter, *http.Request, *User, map[string]json.RawMessage) {
	return func(s *Server, w http.ResponseWriter, _ *http.Request, _ *User, _ map[string]json.RawMessage) {
		backdrops := map[int][]string{}
		for _, t := range s.catalogue(func(t *Title) bool { return t.MediaType == mediaType }) {
			for _, g := range t.GenreIDs {
				backdrops[g] = append(backdrops[g], "/backdrop"+strconv.Itoa(t.TMDBID)+".jpg")
			}
		}
		ids := make([]int, 0, len(backdrops))
		for g := range backdrops {
			ids = append(ids, g)
		}
		sort.Ints(ids)
		out := []map[string]any{}
		for _, g := range ids {
			name, ok := genreNames[g]
			if !ok {
				name = "Genre " + strconv.Itoa(g)
			}
			sort.Strings(backdrops[g])
			out = append(out, map[string]any{"id": g, "name": name, "backdrops": backdrops[g][:min(4, len(backdrops[g]))]})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// genreList answers GET /genres/movie|tv: every genre of the titles of the medium, by id.
func genreList(mediaType string) func(*Server, http.ResponseWriter, *http.Request, *User, map[string]json.RawMessage) {
	return func(s *Server, w http.ResponseWriter, _ *http.Request, _ *User, _ map[string]json.RawMessage) {
		seen := map[int]bool{}
		for _, t := range s.catalogue(func(t *Title) bool { return t.MediaType == mediaType }) {
			for _, g := range t.GenreIDs {
				seen[g] = true
			}
		}
		ids := make([]int, 0, len(seen))
		for g := range seen {
			ids = append(ids, g)
		}
		sort.Ints(ids)
		out := []map[string]any{}
		for _, g := range ids {
			name, ok := genreNames[g]
			if !ok {
				name = "Genre " + strconv.Itoa(g)
			}
			out = append(out, map[string]any{"id": g, "name": name})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// search knows the prefixes tmdb:, tvdb:, imdb: and year: of lib/search.ts; otherwise it matches
// the name case-insensitively. Like the real one (appendix I, I-42), tmdb: returns every title and
// person with that TMDB id, whatever the media type, and a blank after the colon finds nothing.
func (s *Server) search(w http.ResponseWriter, r *http.Request, _ *User, _ map[string]json.RawMessage) {
	q := strings.TrimSpace(r.URL.Query().Get("query"))
	lower := strings.ToLower(q)
	var keep func(*Title) bool
	switch {
	case strings.HasPrefix(lower, "tmdb:"):
		id, _ := strconv.Atoi(q[5:])
		keep = func(t *Title) bool { return t.TMDBID == id }
	case strings.HasPrefix(lower, "tvdb:"):
		id, _ := strconv.Atoi(q[5:])
		keep = func(t *Title) bool { return t.MediaType == "tv" && t.TVDBID == id }
	case strings.HasPrefix(lower, "imdb:"):
		id := q[5:]
		keep = func(t *Title) bool { return t.IMDBID != "" && strings.EqualFold(t.IMDBID, id) }
	case strings.HasPrefix(lower, "year:"):
		rest := strings.TrimSpace(q[5:])
		yearText, name, _ := strings.Cut(rest, " ")
		year, _ := strconv.Atoi(yearText)
		name = strings.ToLower(strings.TrimSpace(name))
		keep = func(t *Title) bool {
			return t.MediaType != "person" && t.Year == year && strings.Contains(strings.ToLower(t.Name), name)
		}
	default:
		keep = func(t *Title) bool { return lower != "" && strings.Contains(strings.ToLower(t.Name), lower) }
	}
	s.page(w, r, s.catalogue(keep), r.URL.Query().Get("language"), "")
}
