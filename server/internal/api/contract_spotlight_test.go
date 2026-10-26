package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Spotlight administration contract of M11 (plan 05-plan-spotlight.md 5.4 to 5.7, DoD of step 4),
// as the owner with the primary profile (p18).

// spotlightTrakt stands in for package metadata with a configured Trakt client id.
type spotlightTrakt struct{ refreshes int }

func (*spotlightTrakt) TraktStatus(context.Context) (model.TraktStatus, error) {
	return model.TraktStatus{Configured: true}, nil
}

func (s *spotlightTrakt) RefreshTrakt(context.Context) error {
	s.refreshes++
	return nil
}

// withSpotlightTrakt returns a handler of the fixture whose administration has a Trakt client id;
// every call builds a new admin.Service over the same database, like a restart of the server.
func (pf profileFixture) withSpotlightTrakt(t *testing.T, tr *spotlightTrakt) http.Handler {
	t.Helper()
	log := vistest.Logger()
	ring, err := secrets.Load(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	o := pf.opts
	o.Admin = admin.New(pf.f.DB, pf.a, &authTestLibraries{}, ring, &authTestTMDB{}, func(string) string { return "" }, log).WithTrakt(tr)
	return NewHandler(o)
}

func spotlightBody(listKey, media, listURL string, itemCount, minHits, intervalHours int) string {
	u := ""
	if listURL != "" {
		u = fmt.Sprintf(`,"listUrl":%q`, listURL)
	}
	return fmt.Sprintf(`{"enabled":true,"listKey":%q,"media":%q,"itemCount":%d,"minHits":%d,"intervalHours":%d%s}`,
		listKey, media, itemCount, minHits, intervalHours, u)
}

// getSpotlightConfig carries the seven list kinds in plan order; setSpotlightConfig refuses every
// value outside its range with 422 and the field name (DoD "Grenzen"), a medium the kind does not
// offer (5.6) and a watchlist address (5.7, rule 11).
func TestContractSpotlightConfig(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	put := func(body string) *gen.SpotlightConfig {
		t.Helper()
		rec := send(t, h, http.MethodPut, BasePath+"/admin/spotlight", "p18", body, nil)
		adminCurationAnswer(t, rec, http.StatusOK)
		var out gen.SpotlightConfig
		decode(t, rec, &out)
		return &out
	}

	rec := send(t, h, http.MethodGet, BasePath+"/admin/spotlight", "p18", "", nil)
	adminCurationAnswer(t, rec, http.StatusOK)
	var cfg gen.SpotlightConfig
	decode(t, rec, &cfg)
	if !cfg.Enabled || cfg.ListKey != "trending" || cfg.Media != "both" || cfg.ItemCount != 20 || cfg.MinHits != 5 || cfg.IntervalHours != 24 || cfg.ListUrl != nil {
		t.Errorf("seeded config %+v", cfg)
	}
	var keys []string
	for _, k := range cfg.ListKinds {
		keys = append(keys, string(k.Key))
		switch k.Key {
		case "boxoffice":
			if len(k.Media) != 1 || k.Media[0] != "movie" || k.NeedsUrl || k.MediaFilters {
				t.Errorf("boxoffice %+v", k)
			}
		case "url":
			if len(k.Media) != 3 || !k.NeedsUrl || !k.MediaFilters {
				t.Errorf("url %+v", k)
			}
		default:
			if len(k.Media) != 3 || k.NeedsUrl || k.MediaFilters {
				t.Errorf("%s %+v", k.Key, k)
			}
		}
	}
	if got := strings.Join(keys, ","); got != "trending,popular,watched_weekly,collected_monthly,anticipated,boxoffice,url" {
		t.Errorf("list kinds %s", got)
	}

	for _, c := range []struct {
		items, hits, hours int
		want               string
	}{
		{4, 5, 24, "itemCount invalid_value"}, {21, 5, 24, "itemCount invalid_value"},
		{20, -1, 24, "minHits invalid_value"}, {20, 11, 24, "minHits invalid_value"},
		{20, 5, 5, "intervalHours invalid_value"}, {20, 5, 49, "intervalHours invalid_value"},
	} {
		body := spotlightBody("trending", "both", "", c.items, c.hits, c.hours)
		if got := fieldProblem(t, send(t, h, http.MethodPut, BasePath+"/admin/spotlight", "p18", body, nil)); got != c.want {
			t.Errorf("PUT %s: %s, want %s", body, got, c.want)
		}
	}
	for _, c := range []struct{ items, hits, hours int }{{5, 0, 6}, {20, 10, 48}} {
		got := put(spotlightBody("trending", "movie", "", c.items, c.hits, c.hours))
		if got.ItemCount != c.items || got.MinHits != c.hits || got.IntervalHours != c.hours || got.Media != "movie" || len(got.ListKinds) != 7 {
			t.Errorf("bounds %+v stored as %+v", c, got)
		}
	}

	for body, want := range map[string]string{
		spotlightBody("boxoffice", "show", "", 20, 5, 24):                                         "media invalid_value",
		spotlightBody("url", "both", "", 20, 5, 24):                                               "listUrl required",
		spotlightBody("url", "both", "https://trakt.tv/users/secretwatcher/watchlist", 20, 5, 24): "listUrl invalid_value",
		spotlightBody("url", "both", "https://example.com/users/someone/lists/abc", 20, 5, 24):    "listUrl invalid_value",
	} {
		if got := fieldProblem(t, send(t, h, http.MethodPut, BasePath+"/admin/spotlight", "p18", body, nil)); got != want {
			t.Errorf("PUT %s: %s, want %s", body, got, want)
		}
	}
	if got := put(spotlightBody("boxoffice", "movie", "", 20, 5, 24)); got.ListKey != "boxoffice" || got.Media != "movie" {
		t.Errorf("boxoffice movie %+v", got)
	}
	const list = "https://trakt.tv/users/someone/lists/cat-favourites"
	if got := put(spotlightBody("url", "show", list, 12, 2, 8)); got.ListKey != "url" || got.ListUrl == nil || *got.ListUrl != list || got.Media != "show" {
		t.Errorf("user list %+v", got)
	}
	// Another kind drops the address.
	if got := put(spotlightBody("trending", "both", list, 20, 5, 24)); got.ListUrl != nil {
		t.Errorf("trending kept the address %q", *got.ListUrl)
	}
	var stored sql.NullString
	if err := pf.f.DB.Read.QueryRow(`SELECT list_url FROM spotlight_config WHERE id = 1`).Scan(&stored); err != nil || stored.Valid {
		t.Errorf("list_url %+v, %v", stored, err)
	}
}

// getSpotlightEntries lists entries without a library hit as well (S-4, DoD "Fehlende Titel sind
// sichtbar") and the orphaned overrides; setSpotlightEntryOverride hides, pins and annotates, null
// clears, an empty body deletes, and a taken rank is 409 rank_taken as problem+json (DoD step 2).
func TestContractSpotlightEntries(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	f.AddSpotlightFilms(t, 1)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := f.DB.Write.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	exec(`INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, title, year, fetched_at) VALUES
	      ('trending', 'movie', 1, 8001, 'Spotlight One', 2025, 1), ('trending', 'movie', 2, 9999, 'Not In The Library', NULL, 1)`)
	exec(`INSERT INTO spotlight_overrides (list_key, media, tmdb_id, hidden, note, updated_at) VALUES ('trending', 'show', 7777, 1, 'Weg', 1)`)
	entries := func() gen.SpotlightEntries {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/admin/spotlight/entries", "p18", "", nil)
		adminCurationAnswer(t, rec, http.StatusOK)
		var out gen.SpotlightEntries
		decode(t, rec, &out)
		return out
	}
	patch := func(path, body string, status int) gen.SpotlightEntry {
		t.Helper()
		rec := send(t, h, http.MethodPatch, BasePath+"/admin/spotlight/entries/"+path, "p18", body, nil)
		adminCurationAnswer(t, rec, status)
		var out gen.SpotlightEntry
		if status == http.StatusOK {
			decode(t, rec, &out)
		}
		return out
	}

	got := entries()
	if got.ListKey != "trending" || len(got.Entries) != 2 || len(got.Orphans) != 1 {
		t.Fatalf("entries %+v", got)
	}
	hit, miss, orphan := got.Entries[0], got.Entries[1], got.Orphans[0]
	if hit.TmdbId != 8001 || hit.Position != 1 || hit.ItemId == nil || *hit.ItemId != "01J9SPOTLIGHT0000000000001" ||
		hit.ItemTitle == nil || *hit.ItemTitle != "Spotlight 01" || hit.Year == nil || *hit.Year != 2025 || hit.Orphaned {
		t.Errorf("hit %+v", hit)
	}
	if miss.TmdbId != 9999 || miss.Position != 2 || miss.ItemId != nil || miss.ItemTitle != nil || miss.Title == nil || *miss.Title != "Not In The Library" || miss.Year != nil {
		t.Errorf("entry without a hit %+v", miss)
	}
	if orphan.Media != "show" || orphan.TmdbId != 7777 || orphan.Position != 0 || !orphan.Orphaned || !orphan.Hidden || orphan.Note == nil || *orphan.Note != "Weg" {
		t.Errorf("orphan %+v", orphan)
	}

	if e := patch("movie/8001", `{"hidden":true}`, http.StatusOK); !e.Hidden || e.PinnedRank != nil || e.Note != nil || e.ItemId == nil {
		t.Errorf("hidden %+v", e)
	}
	if e := patch("movie/8001", `{"pinnedRank":1,"note":"Cat mag ihn"}`, http.StatusOK); !e.Hidden || e.PinnedRank == nil || *e.PinnedRank != 1 || e.Note == nil || *e.Note != "Cat mag ihn" {
		t.Errorf("rank and note %+v", e)
	}

	// Another title on rank 1: 409 rank_taken in problem+json, not a 500.
	rec := send(t, h, http.MethodPatch, BasePath+"/admin/spotlight/entries/movie/9999", "p18", `{"pinnedRank":1}`, nil)
	if rec.Code != http.StatusConflict || problemCode(t, rec) != "rank_taken" || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/problem+json") {
		t.Errorf("taken rank: %d %s %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}

	for body, want := range map[string]string{
		`{"pinnedRank":0}`: "pinnedRank invalid_value",
		`{"note":"` + strings.Repeat("x", 281) + `"}`: "note too_long",
	} {
		if got := fieldProblem(t, send(t, h, http.MethodPatch, BasePath+"/admin/spotlight/entries/movie/8001", "p18", body, nil)); got != want {
			t.Errorf("PATCH %s: %s, want %s", body, got, want)
		}
	}

	// null clears one value and keeps the others; absent keeps.
	if e := patch("movie/8001", `{"note":null}`, http.StatusOK); !e.Hidden || e.PinnedRank == nil || e.Note != nil {
		t.Errorf("note null %+v", e)
	}
	if e := patch("movie/8001", `{"pinnedRank":null}`, http.StatusOK); !e.Hidden || e.PinnedRank != nil {
		t.Errorf("pinnedRank null %+v", e)
	}
	if e := patch("movie/9999", `{"pinnedRank":1}`, http.StatusOK); e.PinnedRank == nil || *e.PinnedRank != 1 || e.ItemId != nil {
		t.Errorf("free rank %+v", e)
	}

	// An empty body removes the override, of an entry and of an orphan.
	if e := patch("movie/8001", `{}`, http.StatusOK); e.Hidden || e.PinnedRank != nil || e.Note != nil || e.Position != 1 {
		t.Errorf("empty body %+v", e)
	}
	if e := patch("show/7777", `{}`, http.StatusOK); !e.Orphaned || e.Hidden || e.Note != nil {
		t.Errorf("orphan deleted %+v", e)
	}
	if got := entries(); len(got.Orphans) != 0 || got.Entries[0].Hidden || got.Entries[1].PinnedRank == nil {
		t.Errorf("after the deletes %+v", got)
	}

	// Neither entry nor override: 404.
	patch("movie/4242", `{"hidden":true}`, http.StatusNotFound)
	patch("show/7777", `{"hidden":true}`, http.StatusNotFound)
}

// refreshSpotlight: 422 without a client id and no stamp; with one 202, then 429 within a minute,
// also after a restart, since the stamp is in the database (DoD "Grenzen").
func TestContractSpotlightRefresh(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	refresh := func(h http.Handler, status int, code string) {
		t.Helper()
		rec := send(t, h, http.MethodPost, BasePath+"/admin/spotlight/refresh", "p18", "", nil)
		if rec.Code != status {
			t.Fatalf("status %d (%s), want %d", rec.Code, rec.Body.String(), status)
		}
		if code != "" && problemCode(t, rec) != code {
			t.Errorf("problem %s, want %s", rec.Body.String(), code)
		}
	}
	stamp := func() sql.NullInt64 {
		t.Helper()
		var v sql.NullInt64
		if err := pf.f.DB.Read.QueryRow(`SELECT last_manual_refresh FROM spotlight_config WHERE id = 1`).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}

	refresh(pf.h, http.StatusUnprocessableEntity, "provider_not_configured")
	if v := stamp(); v.Valid {
		t.Errorf("stamped without a client id: %d", v.Int64)
	}

	tr := &spotlightTrakt{}
	refresh(pf.withSpotlightTrakt(t, tr), http.StatusAccepted, "")
	if v := stamp(); !v.Valid || time.Since(time.UnixMilli(v.Int64)) > time.Minute {
		t.Errorf("stamp %+v", v)
	}
	refresh(pf.withSpotlightTrakt(t, tr), http.StatusTooManyRequests, "too_many_requests")
	refresh(pf.withSpotlightTrakt(t, tr), http.StatusTooManyRequests, "too_many_requests")
	if tr.refreshes != 1 {
		t.Errorf("refused refreshes reached Trakt: %d calls", tr.refreshes)
	}

	// A minute later the next refresh goes through.
	if _, err := pf.f.DB.Write.Exec(`UPDATE spotlight_config SET last_manual_refresh = ? WHERE id = 1`, time.Now().Add(-61*time.Second).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	refresh(pf.withSpotlightTrakt(t, tr), http.StatusAccepted, "")
	if tr.refreshes != 2 {
		t.Errorf("refresh after a minute: %d calls", tr.refreshes)
	}
}
