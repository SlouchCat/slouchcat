// Package arrtest is a fake Radarr and a fake Sonarr on an httptest.Server for the tests of package
// arr and of the packages built on it (concept 25, build step 4.6a). It models what appendix I
// section 5 measured on Radarr 6.4.4 and Sonarr 4.0.20: tags (POST is idempotent and lower-cases
// the label; Radarr refuses labels outside ^[a-z0-9-]+$ with 400 and a validation array, Sonarr
// takes them), movies by tmdbId with hasFile, series by tvdbId with statistics and seasons, the
// editor with applyTags, the queue in pages (page, pageSize, totalRecords, records) and 401 for a
// wrong key. The Sonarr queue fields follow the documentation (not verified, I-57).
//
// Every call is counted by method and route, also calls to routes the fake does not know (they
// answer 404), so a test can prove that nothing but the allowed endpoints was called.
//
// It is a test helper: only _test.go files import it (depguard rule test-helpers). It imports
// nothing of the server, so the tests of package arr itself can use it.
package arrtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// APIKey is the api key of every fake.
const APIKey = "arr-test-key-3c8e1f0a9b2d" //nolint:gosec // G101: the key of the fake, not a credential

// Kind is the service a fake plays.
type Kind string

// The two services.
const (
	Radarr Kind = "radarr"
	Sonarr Kind = "sonarr"
)

// TextLabel is Radarr's validation message for a label outside ^[a-z0-9-]+$ (I-52).
const TextLabel = "Allowed characters a-z, 0-9 and -"

var radarrLabel = regexp.MustCompile(`^[a-z0-9-]+$`)

// Movie is a Radarr movie of the fake.
type Movie struct {
	ID      int
	TMDBID  int
	Title   string
	Tags    []int
	HasFile bool
}

// Season is a season of a Sonarr series. NoStatistics leaves the statistics out of the answer.
type Season struct {
	Number            int
	Monitored         bool
	EpisodeFileCount  int
	EpisodeCount      int
	TotalEpisodeCount int
	NoStatistics      bool
}

// Series is a Sonarr series of the fake; its statistics are the sums of its seasons.
type Series struct {
	ID      int
	TVDBID  int
	Title   string
	Tags    []int
	Seasons []Season
}

// Record is one queue record as JSON fields; a missing key is a missing field.
type Record map[string]any

// Failure is a failure picture of a route.
type Failure struct {
	Status   int    // answer with this status
	Body     string // raw body; with a 3xx status the body stays empty
	Location string // Location header of a 3xx answer
	Hang     bool   // never answer; the request ends when the client gives up or the fake closes
}

// Call is one call the fake received.
type Call struct {
	Route  string // method and route, "GET /api/v3/tag"; unknown routes keep their path
	Path   string
	Query  url.Values
	HasKey bool   // X-Api-Key was sent (right or wrong)
	Body   []byte // the request body
}

// Server is the fake. Its methods are safe for concurrent use.
type Server struct {
	*httptest.Server
	Kind Kind

	stop     chan struct{}
	stopOnce sync.Once

	mu          sync.Mutex
	tags        []tagJSON
	nextTag     int
	movies      map[int]*Movie
	series      map[int]*Series
	queue       []Record
	maxPageSize int
	failures    map[string]Failure
	calls       []Call
}

type tagJSON struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// NewRadarr starts a fake Radarr without tags, movies and queue. It closes with the test.
func NewRadarr(t testing.TB) *Server { return start(t, Radarr) }

// NewSonarr starts a fake Sonarr without tags, series and queue. It closes with the test.
func NewSonarr(t testing.TB) *Server { return start(t, Sonarr) }

func start(t testing.TB, kind Kind) *Server {
	t.Helper()
	s := &Server{
		Kind: kind, stop: make(chan struct{}), nextTag: 1,
		movies: map[int]*Movie{}, series: map[int]*Series{}, failures: map[string]Failure{},
	}
	s.Server = httptest.NewServer(s.routes())
	t.Cleanup(s.Close)
	return s
}

// Close ends hanging answers and stops the server.
func (s *Server) Close() {
	s.stopOnce.Do(func() { close(s.stop) })
	s.Server.Close()
}

// AddTag adds a tag (as an earlier run or Seerr's "Tag Requests" would) and returns its id.
func (s *Server) AddTag(label string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addTag(strings.ToLower(label))
}

func (s *Server) addTag(label string) int {
	for _, t := range s.tags {
		if t.Label == label {
			return t.ID
		}
	}
	id := s.nextTag
	s.nextTag++
	s.tags = append(s.tags, tagJSON{ID: id, Label: label})
	return id
}

// Tags returns the tags by id.
func (s *Server) Tags() map[int]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[int]string{}
	for _, t := range s.tags {
		out[t.ID] = t.Label
	}
	return out
}

// AddMovie adds a Radarr movie.
func (s *Server) AddMovie(m Movie) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m.Tags = slices.Clone(m.Tags)
	s.movies[m.ID] = &m
}

// Movie returns a copy of the movie with the Radarr id.
func (s *Server) Movie(id int) (Movie, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.movies[id]
	if !ok {
		return Movie{}, false
	}
	out := *m
	out.Tags = slices.Clone(m.Tags)
	return out, true
}

// AddSeries adds a Sonarr series.
func (s *Server) AddSeries(se Series) {
	s.mu.Lock()
	defer s.mu.Unlock()
	se.Tags = slices.Clone(se.Tags)
	se.Seasons = slices.Clone(se.Seasons)
	s.series[se.ID] = &se
}

// Series returns a copy of the series with the Sonarr id.
func (s *Server) Series(id int) (Series, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	se, ok := s.series[id]
	if !ok {
		return Series{}, false
	}
	out := *se
	out.Tags = slices.Clone(se.Tags)
	out.Seasons = slices.Clone(se.Seasons)
	return out, true
}

// SetQueue replaces the queue.
func (s *Server) SetQueue(records ...Record) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = slices.Clone(records)
}

// SetMaxPageSize caps the pageSize the queue honours (0: no cap), so a test can force several
// pages.
func (s *Server) SetMaxPageSize(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxPageSize = n
}

// SetFailure makes route ("GET /api/v3/tag", or "*" for every route) answer with f; a zero Failure
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

// Calls returns how often route ("POST /api/v3/tag") was called.
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

// Routes returns the routes the fake answers for its kind.
func (s *Server) Routes() []string {
	out := []string{"GET /api/v3/tag", "POST /api/v3/tag", "GET /api/v3/queue"}
	if s.Kind == Radarr {
		return append(out, "GET /api/v3/movie", "PUT /api/v3/movie/editor")
	}
	return append(out, "GET /api/v3/series", "PUT /api/v3/series/editor")
}

func (s *Server) routes() http.Handler {
	handlers := map[string]func(*Server, http.ResponseWriter, *http.Request, []byte){
		"GET /api/v3/tag":   (*Server).listTags,
		"POST /api/v3/tag":  (*Server).createTag,
		"GET /api/v3/queue": (*Server).getQueue,
	}
	if s.Kind == Radarr {
		handlers["GET /api/v3/movie"] = (*Server).getMovies
		handlers["PUT /api/v3/movie/editor"] = (*Server).movieEditor
	} else {
		handlers["GET /api/v3/series"] = (*Server).getSeries
		handlers["PUT /api/v3/series/editor"] = (*Server).seriesEditor
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := r.Method + " " + r.URL.Path
		h, known := handlers[route]
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		s.mu.Lock()
		s.calls = append(s.calls, Call{Route: route, Path: r.URL.Path, Query: r.URL.Query(), HasKey: r.Header.Get("X-Api-Key") != "", Body: body})
		f, failing := s.failures[route]
		if !failing {
			f, failing = s.failures["*"]
		}
		s.mu.Unlock()

		if failing {
			switch {
			case f.Hang:
				select {
				case <-r.Context().Done():
				case <-s.stop:
				}
			case f.Status >= 300 && f.Status < 400:
				if f.Location != "" {
					w.Header().Set("Location", f.Location)
				}
				w.WriteHeader(f.Status)
			default:
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(f.Status)
				_, _ = w.Write([]byte(f.Body))
			}
			return
		}
		if r.Header.Get("X-Api-Key") != APIKey {
			w.WriteHeader(http.StatusUnauthorized) // both services answer 401 with an empty body
			return
		}
		if !known {
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "NotFound"})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		h(s, w, r, body)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) listTags(w http.ResponseWriter, _ *http.Request, _ []byte) {
	writeJSON(w, http.StatusOK, append([]tagJSON{}, s.tags...))
}

func (s *Server) createTag(w http.ResponseWriter, _ *http.Request, body []byte) {
	var in struct {
		Label string `json:"label"`
	}
	if json.Unmarshal(body, &in) != nil || strings.TrimSpace(in.Label) == "" {
		writeJSON(w, http.StatusBadRequest, []map[string]any{{"propertyName": "Label", "errorMessage": "'Label' must not be empty.", "severity": "error"}})
		return
	}
	label := strings.ToLower(in.Label) // I-51
	if s.Kind == Radarr && !radarrLabel.MatchString(label) {
		writeJSON(w, http.StatusBadRequest, []map[string]any{{
			"propertyName": "Label", "errorMessage": TextLabel, "attemptedValue": in.Label, "severity": "error", "errorCode": "RegularExpressionValidator",
		}})
		return
	}
	id := s.addTag(label) // I-50: the same label again gives the same id
	writeJSON(w, http.StatusCreated, tagJSON{ID: id, Label: label})
}

func (s *Server) movieJSON(m *Movie) map[string]any {
	out := map[string]any{
		"id": m.ID, "tmdbId": m.TMDBID, "title": m.Title, "tags": nonNil(m.Tags), "hasFile": m.HasFile,
		"path": "/movies/" + m.Title, "monitored": true,
	}
	if m.HasFile {
		out["movieFile"] = map[string]any{"id": m.ID * 10, "path": "/movies/" + m.Title + "/" + m.Title + ".mkv"}
	}
	return out
}

func (s *Server) getMovies(w http.ResponseWriter, r *http.Request, _ []byte) {
	want, _ := strconv.Atoi(r.URL.Query().Get("tmdbId"))
	out := []map[string]any{}
	for _, id := range sortedKeys(s.movies) {
		m := s.movies[id]
		if want == 0 || m.TMDBID == want {
			out = append(out, s.movieJSON(m))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) seriesJSON(se *Series) map[string]any {
	var total Season
	seasons := []map[string]any{}
	for _, sn := range se.Seasons {
		total.EpisodeFileCount += sn.EpisodeFileCount
		total.EpisodeCount += sn.EpisodeCount
		total.TotalEpisodeCount += sn.TotalEpisodeCount
		js := map[string]any{"seasonNumber": sn.Number, "monitored": sn.Monitored}
		if !sn.NoStatistics {
			js["statistics"] = map[string]any{
				"episodeFileCount": sn.EpisodeFileCount, "episodeCount": sn.EpisodeCount, "totalEpisodeCount": sn.TotalEpisodeCount,
			}
		}
		seasons = append(seasons, js)
	}
	return map[string]any{
		"id": se.ID, "tvdbId": se.TVDBID, "title": se.Title, "tags": nonNil(se.Tags), "path": "/tv/" + se.Title,
		"seasons": seasons,
		"statistics": map[string]any{
			"seasonCount": len(se.Seasons), "episodeFileCount": total.EpisodeFileCount, "episodeCount": total.EpisodeCount,
			"totalEpisodeCount": total.TotalEpisodeCount,
		},
	}
}

func (s *Server) getSeries(w http.ResponseWriter, r *http.Request, _ []byte) {
	want, _ := strconv.Atoi(r.URL.Query().Get("tvdbId"))
	out := []map[string]any{}
	for _, id := range sortedKeys(s.series) {
		se := s.series[id]
		if want == 0 || se.TVDBID == want {
			out = append(out, s.seriesJSON(se))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// applyTags changes tags as the editor does: add, remove or replace.
func applyTags(tags, change []int, mode string) []int {
	switch mode {
	case "add":
		for _, t := range change {
			if !slices.Contains(tags, t) {
				tags = append(tags, t)
			}
		}
	case "remove":
		tags = slices.DeleteFunc(tags, func(t int) bool { return slices.Contains(change, t) })
	case "replace":
		tags = slices.Clone(change)
	}
	return tags
}

type editorBody struct {
	MovieIDs  []int  `json:"movieIds"`
	SeriesIDs []int  `json:"seriesIds"`
	Tags      []int  `json:"tags"`
	ApplyTags string `json:"applyTags"`
}

func (s *Server) movieEditor(w http.ResponseWriter, _ *http.Request, body []byte) {
	var in editorBody
	if json.Unmarshal(body, &in) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid body"})
		return
	}
	out := []map[string]any{}
	for _, id := range in.MovieIDs {
		if m, ok := s.movies[id]; ok {
			m.Tags = applyTags(m.Tags, in.Tags, in.ApplyTags)
			out = append(out, s.movieJSON(m))
		}
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (s *Server) seriesEditor(w http.ResponseWriter, _ *http.Request, body []byte) {
	var in editorBody
	if json.Unmarshal(body, &in) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid body"})
		return
	}
	out := []map[string]any{}
	for _, id := range in.SeriesIDs {
		if se, ok := s.series[id]; ok {
			se.Tags = applyTags(se.Tags, in.Tags, in.ApplyTags)
			out = append(out, s.seriesJSON(se))
		}
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (s *Server) getQueue(w http.ResponseWriter, r *http.Request, _ []byte) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(q.Get("pageSize"))
	if size < 1 {
		size = 10 // the default of both services
	}
	if s.maxPageSize > 0 && size > s.maxPageSize {
		size = s.maxPageSize
	}
	from := min((page-1)*size, len(s.queue))
	to := min(from+size, len(s.queue))
	writeJSON(w, http.StatusOK, map[string]any{
		"page": page, "pageSize": size, "sortKey": "timeleft", "sortDirection": "ascending",
		"totalRecords": len(s.queue), "records": s.queue[from:to],
	})
}

func nonNil(v []int) []int {
	if v == nil {
		return []int{}
	}
	return v
}

func sortedKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// RadarrQueue returns the queue pictures of appendix I, I-56: movie 1 downloading (size 1000,
// sizeleft 550, 12 min left), movie 2 completed but stuck (importBlocked, warning, with messages),
// movie 3 with only movieId and status, and an entry without movieId (an unknown item).
func RadarrQueue() []Record {
	return []Record{
		{
			"id": 101, "movieId": 1, "title": "Film.One.2024.1080p", "size": 1000.0, "sizeleft": 550.0,
			"timeleft": "00:12:00", "estimatedCompletionTime": "2026-11-17T12:12:00Z", "status": "downloading",
			"trackedDownloadState": "downloading", "trackedDownloadStatus": "ok", "statusMessages": []any{},
			"downloadClient": "SABnzbd", "downloadId": "SABnzbd_nzo_1", "protocol": "usenet", "outputPath": "/downloads/incomplete/Film.One",
		},
		{
			"id": 102, "movieId": 2, "title": "Film.Two.2023.2160p", "size": 5000.0, "sizeleft": 0.0,
			"timeleft": "00:00:00", "status": "completed", "trackedDownloadState": "importBlocked",
			"trackedDownloadStatus": "warning", "errorMessage": "",
			"statusMessages": []any{map[string]any{"title": "Film.Two.2023.2160p", "messages": []any{"No files found are eligible for import"}}},
			"downloadClient": "SABnzbd", "protocol": "usenet", "outputPath": "/downloads/complete/Film.Two",
		},
		{"id": 103, "movieId": 3, "status": "queued"},
		{"id": 104, "title": "Unknown.Release", "size": 10.0, "sizeleft": 5.0, "status": "downloading"},
	}
}

// SonarrQueue returns Sonarr queue pictures after the documentation (not verified, I-57): two
// episodes of series 7 season 1 downloading, one with 1 day and 2 hours left, and an entry
// without seriesId.
func SonarrQueue() []Record {
	return []Record{
		{
			"id": 201, "seriesId": 7, "episodeId": 71, "seasonNumber": 1, "size": 2000.0, "sizeleft": 500.0,
			"timeleft": "1.02:03:04", "status": "downloading", "trackedDownloadState": "downloading",
			"trackedDownloadStatus": "ok", "downloadClient": "SABnzbd", "protocol": "usenet",
		},
		{
			"id": 202, "seriesId": 7, "episodeId": 72, "seasonNumber": 1, "size": 2000.0, "sizeleft": 2000.0,
			"timeleft": "", "status": "queued", "trackedDownloadState": "downloading", "trackedDownloadStatus": "ok",
		},
		{"id": 203, "episodeId": 99, "seasonNumber": 2, "size": 100.0, "sizeleft": 50.0, "status": "downloading"},
	}
}
