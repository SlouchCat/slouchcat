package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// catalogTMDB serves the TMDB details of the series 1399 and 1400 (a rejected token gets TMDB's
// 401) and counts the requests per series; the fake answers every other series but 900 with 404.
type catalogTMDB struct {
	mu   sync.Mutex
	hits map[string]int
	fake *fakeTMDB
}

// count is the number of detail requests for series id: the served ones and the unknown ones the
// fake answers itself.
func (c *catalogTMDB) count(id string) int {
	c.mu.Lock()
	n := c.hits[id]
	c.mu.Unlock()
	return n + c.fake.count("/3/tv/"+id)
}

func serveCatalog(t *testing.T, e *env) *catalogTMDB {
	t.Helper()
	c := &catalogTMDB{hits: map[string]int{}, fake: e.tmdb}
	details := map[string]map[string]any{
		"1399": {
			"id": 1399, "name": "Catalog Show", "status": "Returning Series", "in_production": true,
			"seasons": []any{
				map[string]any{"season_number": 0, "episode_count": 3, "air_date": "2010-12-05"},
				map[string]any{"season_number": 1, "episode_count": 10, "air_date": "2011-04-17"},
				map[string]any{"season_number": 2, "episode_count": 8, "air_date": "2026-11-01"},
				map[string]any{"season_number": 3, "episode_count": 0, "air_date": "not a date"},
			},
			"last_episode_to_air": map[string]any{"season_number": 2, "episode_number": 3, "air_date": "2026-11-15"},
			"next_episode_to_air": map[string]any{"season_number": 2, "episode_number": 4, "air_date": "2026-11-22"},
		},
		"1400": {
			"id": 1400, "name": "Ended Show", "status": "Ended", "in_production": false,
			"seasons": []any{map[string]any{"season_number": 1, "episode_count": 6, "air_date": "2001-01-01"}},
		},
	}
	for id, d := range details {
		e.tmdb.mux.HandleFunc("GET /3/tv/"+id, func(w http.ResponseWriter, r *http.Request) {
			c.mu.Lock()
			c.hits[id]++
			c.mu.Unlock()
			if r.Header.Get("Authorization") != "Bearer "+goodToken {
				http.Error(w, `{"status_code":7}`, http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(d)
		})
	}
	return c
}

// addSeries adds a series to library 2; tmdbID nil leaves it without TMDB id.
func (e *env) addSeries(t *testing.T, id string, tmdbID any) {
	t.Helper()
	mustExec(t, e.db.Write, `INSERT INTO items (id, kind, library_id, title, sort_title, tmdb_id, metadata_state, added_at, created_at, updated_at)
		VALUES (?, 'series', 2, ?, ?, ?, 'matched', 0, 0, 0)`, id, id, id, tmdbID)
}

type catalogRow struct {
	tmdbID       int64
	status       sql.NullString
	inProduction int64
	seasons      string
	next, last   sql.NullString
	fetchedAt    int64
}

func (e *env) catalogRow(t *testing.T, itemID string) (catalogRow, bool) {
	t.Helper()
	var r catalogRow
	err := e.db.Read.QueryRow(`SELECT tmdb_id, status, in_production, seasons, next_episode, last_episode, fetched_at
		FROM series_catalog WHERE item_id = ?`, itemID).Scan(&r.tmdbID, &r.status, &r.inProduction, &r.seasons, &r.next, &r.last, &r.fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return catalogRow{}, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return r, true
}

const (
	catalogSeriesA = "01J9000000000000000000CATA" // TMDB 1399
	catalogSeriesB = "01J9000000000000000000CATB" // TMDB 1400
	catalogSeriesC = "01J9000000000000000000CATC" // without TMDB id
	catalogSeriesD = "01J9000000000000000000CATD" // TMDB 4040, unknown to TMDB
)

func runCatalogSweep(t *testing.T, e *env) {
	t.Helper()
	if err := e.svc.handleSeriesCatalog(context.Background(), &jobs.Job{Type: JobSeriesCatalog, Payload: json.RawMessage(`{}`)}); err != nil {
		t.Fatalf("series.catalog: %v", err)
	}
}

// The sweep stores the seasons of every series with a TMDB id as TMDB lists them (season 0
// included, a bad date dropped), the status, in production and the last and next episode; a series
// without TMDB id gets no row and no request.
func TestSeriesCatalogStoresSeasons(t *testing.T) {
	e := newEnv(t, goodToken)
	c := serveCatalog(t, e)
	e.addSeries(t, catalogSeriesA, 1399)
	e.addSeries(t, catalogSeriesB, 1400)
	e.addSeries(t, catalogSeriesC, nil)
	runCatalogSweep(t, e)
	a, ok := e.catalogRow(t, catalogSeriesA)
	if !ok {
		t.Fatal("no catalog row for the series with TMDB id")
	}
	var seasons []ExternalSeason
	if err := json.Unmarshal([]byte(a.seasons), &seasons); err != nil {
		t.Fatal(err)
	}
	want := []ExternalSeason{{0, 3, "2010-12-05"}, {1, 10, "2011-04-17"}, {2, 8, "2026-11-01"}, {3, 0, ""}}
	if !slices.Equal(seasons, want) {
		t.Errorf("seasons = %+v, want %+v", seasons, want)
	}
	if a.tmdbID != 1399 || a.status.String != "Returning Series" || a.inProduction != 1 || a.fetchedAt != testNow.UnixMilli() {
		t.Errorf("row = %+v", a)
	}
	var next, last ExternalEpisode
	if err := json.Unmarshal([]byte(a.next.String), &next); err != nil || next != (ExternalEpisode{2, 4, "2026-11-22"}) {
		t.Errorf("next episode = %q, %v", a.next.String, err)
	}
	if err := json.Unmarshal([]byte(a.last.String), &last); err != nil || last != (ExternalEpisode{2, 3, "2026-11-15"}) {
		t.Errorf("last episode = %q, %v", a.last.String, err)
	}
	b, ok := e.catalogRow(t, catalogSeriesB)
	if !ok || b.status.String != "Ended" || b.inProduction != 0 || b.next.Valid || b.last.Valid || b.seasons != `[{"number":1,"episodeCount":6,"airDate":"2001-01-01"}]` {
		t.Errorf("ended series = %+v, %v", b, ok)
	}
	if _, ok := e.catalogRow(t, catalogSeriesC); ok {
		t.Error("a series without TMDB id got a catalog row")
	}
	if c.count("1399") != 1 || c.count("1400") != 1 {
		t.Errorf("requests = %v, want one per series", c.hits)
	}
}

// The sweep fetches only the due series: missing rows, rows of another TMDB id and rows older than
// SeriesCatalogMaxAge; fresh rows are left alone.
func TestSeriesCatalogRefreshRespectsStaleness(t *testing.T) {
	e := newEnv(t, goodToken)
	c := serveCatalog(t, e)
	e.addSeries(t, catalogSeriesA, 1399)
	e.addSeries(t, catalogSeriesB, 1400)
	runCatalogSweep(t, e)
	runCatalogSweep(t, e)
	if c.count("1399") != 1 || c.count("1400") != 1 {
		t.Fatalf("second sweep with fresh rows asked TMDB: %v", c.hits)
	}
	stale := testNow.Add(-SeriesCatalogMaxAge - time.Hour).UnixMilli()
	fresh := testNow.Add(-SeriesCatalogMaxAge + time.Hour).UnixMilli()
	mustExec(t, e.db.Write, `UPDATE series_catalog SET fetched_at = ? WHERE item_id = ?`, stale, catalogSeriesA)
	mustExec(t, e.db.Write, `UPDATE series_catalog SET fetched_at = ? WHERE item_id = ?`, fresh, catalogSeriesB)
	runCatalogSweep(t, e)
	if c.count("1399") != 2 || c.count("1400") != 1 {
		t.Errorf("after one stale row: %v, want 1399 twice and 1400 once", c.hits)
	}
	if a, _ := e.catalogRow(t, catalogSeriesA); a.fetchedAt != testNow.UnixMilli() {
		t.Errorf("fetched_at of the refreshed row = %d", a.fetchedAt)
	}
	// Another TMDB id makes a fresh row due at once.
	mustExec(t, e.db.Write, `UPDATE series_catalog SET tmdb_id = 1399 WHERE item_id = ?`, catalogSeriesB)
	runCatalogSweep(t, e)
	if b, _ := e.catalogRow(t, catalogSeriesB); c.count("1400") != 2 || b.tmdbID != 1400 {
		t.Errorf("row of another TMDB id: %d requests, tmdb %d; want 2 and 1400", c.count("1400"), b.tmdbID)
	}
	// The schedule queues one sweep under its key at priority 7.
	ids, err := e.svc.runSeriesCatalog(context.Background(), e.queue, seriesCatalogPriority)
	if err != nil || len(ids) != 1 {
		t.Fatalf("run = %v, %v", ids, err)
	}
	var prio int
	var key string
	if err := e.db.Read.QueryRow(`SELECT priority, idempotency_key FROM jobs WHERE id = ?`, ids[0]).Scan(&prio, &key); err != nil {
		t.Fatal(err)
	}
	if prio != 7 || key != seriesCatalogKey {
		t.Errorf("sweep priority %d key %q, want 7 and %q", prio, key, seriesCatalogKey)
	}
}

// A series TMDB does not know gets an empty row, a stored one keeps its seasons; the sweep succeeds
// and does not ask again before the row is due (no retry storm). A row of another TMDB id is
// emptied when the new id is unknown.
func TestSeriesCatalogUnknownAtTMDB(t *testing.T) {
	e := newEnv(t, goodToken)
	c := serveCatalog(t, e)
	e.addSeries(t, catalogSeriesD, 4040)
	e.addSeries(t, catalogSeriesA, 1399)
	stale := testNow.Add(-SeriesCatalogMaxAge - time.Hour).UnixMilli()
	mustExec(t, e.db.Write, `INSERT INTO series_catalog (item_id, tmdb_id, status, seasons, fetched_at) VALUES (?, 1399, 'Ended', '[{"number":1,"episodeCount":10}]', ?)`, catalogSeriesA, stale)
	mustExec(t, e.db.Write, `UPDATE items SET tmdb_id = 4041 WHERE id = ?`, catalogSeriesA) // changed to an unknown id
	runCatalogSweep(t, e)
	d, ok := e.catalogRow(t, catalogSeriesD)
	if !ok || d.tmdbID != 4040 || d.seasons != "[]" || d.status.Valid || d.fetchedAt != testNow.UnixMilli() {
		t.Errorf("unknown series = %+v, %v; want an empty row fetched now", d, ok)
	}
	a, _ := e.catalogRow(t, catalogSeriesA)
	if a.tmdbID != 4041 || a.seasons != "[]" || a.status.Valid {
		t.Errorf("row of the old id after the unknown new id = %+v, want emptied", a)
	}
	runCatalogSweep(t, e)
	if c.count("4040") != 1 || c.count("4041") != 1 {
		t.Errorf("requests for unknown series = %v, want one each", c.hits)
	}
	// A known series that vanished keeps its seasons; its fetched_at moves on.
	mustExec(t, e.db.Write, `UPDATE items SET tmdb_id = 4040 WHERE id = ?`, catalogSeriesA)
	mustExec(t, e.db.Write, `UPDATE series_catalog SET tmdb_id = 4040, seasons = '[{"number":1,"episodeCount":10}]', fetched_at = ? WHERE item_id = ?`, stale, catalogSeriesA)
	runCatalogSweep(t, e)
	if a, _ := e.catalogRow(t, catalogSeriesA); a.seasons != `[{"number":1,"episodeCount":10}]` || a.fetchedAt != testNow.UnixMilli() {
		t.Errorf("vanished series = %+v, want its seasons kept and fetched_at now", a)
	}
	// The run of one unknown series succeeds too.
	if err := e.svc.handleSeriesCatalog(context.Background(), &jobs.Job{Type: JobSeriesCatalog, Payload: json.RawMessage(`{"itemId":"` + catalogSeriesD + `"}`)}); err != nil {
		t.Errorf("run of one unknown series: %v", err)
	}
}

// Without a TMDB key both runs end without work and without error; a key TMDB rejects fails them,
// so the queue retries after its backoff.
func TestSeriesCatalogWithoutKey(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, "")
	c := serveCatalog(t, e)
	e.addSeries(t, catalogSeriesA, 1399)
	runCatalogSweep(t, e)
	if err := e.svc.handleSeriesCatalog(ctx, &jobs.Job{Type: JobSeriesCatalog, Payload: json.RawMessage(`{"itemId":"` + catalogSeriesA + `"}`)}); err != nil {
		t.Errorf("run of one series without key: %v", err)
	}
	if c.count("1399") != 0 {
		t.Errorf("requests without key = %d", c.count("1399"))
	}
	if _, ok := e.catalogRow(t, catalogSeriesA); ok {
		t.Error("a row without key")
	}
	bad := newEnv(t, "rejected-token")
	serveCatalog(t, bad)
	bad.addSeries(t, catalogSeriesA, 1399)
	if err := bad.svc.handleSeriesCatalog(ctx, &jobs.Job{Type: JobSeriesCatalog, Payload: json.RawMessage(`{}`)}); !errors.Is(err, model.ErrTMDBKeyInvalid) {
		t.Errorf("sweep with a rejected key = %v, want ErrTMDBKeyInvalid", err)
	}
	if err := bad.svc.handleSeriesCatalog(ctx, &jobs.Job{Type: JobSeriesCatalog, Payload: json.RawMessage(`{"itemId":"` + catalogSeriesA + `"}`)}); !errors.Is(err, model.ErrTMDBKeyInvalid) {
		t.Errorf("run of one series with a rejected key = %v, want ErrTMDBKeyInvalid", err)
	}
}

// Repeated transport failures end the sweep with an error (the queue retries); a single failure
// between successes does not.
func TestSeriesCatalogTransportFailures(t *testing.T) {
	e := newEnv(t, goodToken)
	for _, id := range []string{"1399", "1400", "4040"} {
		e.tmdb.mux.HandleFunc("GET /3/tv/"+id, func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusBadGateway)
		})
	}
	for _, id := range []string{catalogSeriesA, catalogSeriesB, catalogSeriesD} {
		e.addSeries(t, id, map[string]int{catalogSeriesA: 1399, catalogSeriesB: 1400, catalogSeriesD: 4040}[id])
	}
	err := e.svc.handleSeriesCatalog(context.Background(), &jobs.Job{Type: JobSeriesCatalog, Payload: json.RawMessage(`{}`)})
	if err == nil || errors.Is(err, model.ErrTMDBKeyInvalid) {
		t.Errorf("sweep with three failures = %v, want a transport error", err)
	}
}

// The match of a series queues the catalog of that series at priority 4.
func TestSeriesCatalogQueuedByMatch(t *testing.T) {
	e := newEnv(t, goodToken)
	e.addTree(t, node{title: "Test Show", year: 2010})
	if err := e.svc.match(context.Background(), idSeries); err != nil {
		t.Fatal(err)
	}
	if got, want := e.jobList(t, JobSeriesCatalog), []string{seriesCatalogKey + ":" + idSeries + " p4"}; !slices.Equal(got, want) {
		t.Errorf("series.catalog jobs after the match = %v, want %v", got, want)
	}
}

// The event run: QueueSeriesCatalog queues one job per series at priority 4 under its own key; it
// fetches that series only; a title without TMDB id or of another kind ends it without work. The
// match of a series queues it.
func TestSeriesCatalogEventRun(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, goodToken)
	c := serveCatalog(t, e)
	e.addSeries(t, catalogSeriesA, 1399)
	e.addSeries(t, catalogSeriesB, 1400)
	e.addSeries(t, catalogSeriesC, nil)
	if err := e.svc.QueueSeriesCatalog(ctx, catalogSeriesA); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.QueueSeriesCatalog(ctx, catalogSeriesA); err != nil {
		t.Fatal(err)
	}
	var n, prio int
	if err := e.db.Read.QueryRow(`SELECT count(*), max(priority) FROM jobs WHERE type = ? AND idempotency_key = ?`, JobSeriesCatalog, seriesCatalogKey+":"+catalogSeriesA).Scan(&n, &prio); err != nil {
		t.Fatal(err)
	}
	if n != 1 || prio != 4 {
		t.Errorf("queued event jobs = %d at priority %d, want 1 at 4", n, prio)
	}
	for _, id := range []string{catalogSeriesA, catalogSeriesC, "01J9000000000000000000NONE"} {
		if err := e.svc.handleSeriesCatalog(ctx, &jobs.Job{Type: JobSeriesCatalog, Payload: json.RawMessage(`{"itemId":"` + id + `"}`)}); err != nil {
			t.Errorf("run of %s: %v", id, err)
		}
	}
	if c.count("1399") != 1 || c.count("1400") != 0 {
		t.Errorf("requests = %v, want only the queued series", c.hits)
	}
	// The event run fetches even a fresh row (a new match).
	if err := e.svc.handleSeriesCatalog(ctx, &jobs.Job{Type: JobSeriesCatalog, Payload: json.RawMessage(`{"itemId":"` + catalogSeriesA + `"}`)}); err != nil || c.count("1399") != 2 {
		t.Errorf("second event run = %v, %d requests; want 2", err, c.count("1399"))
	}
	if _, ok := e.catalogRow(t, catalogSeriesC); ok {
		t.Error("a series without TMDB id got a row")
	}
	r := jobs.NewRegistry(e.queue, slog.New(slog.DiscardHandler))
	e.svc.RegisterTypes(r)
	if typ, ok := r.Lookup(JobSeriesCatalog); !ok || !typ.Scheduled() || typ.Run == nil || typ.Handler == nil || typ.Priority != 7 {
		t.Errorf("registered type = %+v, %v", typ, ok)
	}
}
