package radio

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/rules"
	"github.com/slouchcat/slouchcat/server/internal/store"
)

// frozenRadioDB replays a frozen state of testdata/db with extra SQL and opens it through
// store.Open, which runs the migrations up to the current one.
func frozenRadioDB(t *testing.T, state, extra string) *store.DB {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", state))
	if err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	dsn := (&url.URL{Scheme: "file", Path: filepath.Join(cfg, store.FileName),
		RawQuery: url.Values{"_pragma": {"foreign_keys(0)"}}.Encode()}).String()
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(string(script) + extra); err != nil {
		t.Fatalf("replay %s: %v", state, err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), store.Options{ConfigDir: cfg, Version: "test", Logger: catDiscard})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// storedFacets reads the stored facets of a station by facet, in the order of the rules.
func storedFacets(t *testing.T, db *store.DB, id int64) map[string][]string {
	t.Helper()
	rows, err := db.Read.Query(`SELECT facet, value FROM radio_station_facets WHERE station_id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var f, v string
		if err := rows.Scan(&f, &v); err != nil {
			t.Fatal(err)
		}
		out[f] = append(out[f], v)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	sortFacetValues(out)
	return out
}

// PB-6 and PB-7: the first start after migration 00048 on the newest frozen state with radio data
// (hoerbuch-quellen.sql: countries ["AT","CH","DE"], tag rules, manual facets and facet locks of
// webradio2.sql), with a hidden Swiss station (3), the hidden own station (59) and a catalog station
// without tags (7) added. The offer is empty, the countries setting is not taken over; every
// hidden station is demarked, the favourites stay in the database. Choosing CH brings the Swiss
// favourites back, not the hidden ones. The rules of the administration, the manual values and the
// facet locks are gone, and the facets of a catalog station, one without tags and the own station
// equal what the rules file derives; every station with a stream has its provider. A second start
// changes nothing.
func TestFirstStartPreparesTheOffer(t *testing.T) {
	t.Parallel() // the migration of the frozen state takes most of a minute
	db := frozenRadioDB(t, "hoerbuch-quellen.sql", `
UPDATE radio_stations SET hidden = 1 WHERE id IN (3, 59);
DELETE FROM radio_station_tags WHERE station_id = 7;`)
	ctx := context.Background()
	count := func(query string, args ...any) int {
		t.Helper()
		var n int
		if err := db.Read.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	s := New(db, catDiscard, WithClock(func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) }))
	s.cat.zone = func() string { return "Europe/Zurich" }
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	o, err := s.Offer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if o.Totals.Offered != 0 || o.Totals.Countries != 0 || o.Totals.Demarked != 3 || o.Version != 0 {
		t.Errorf("offer after the first start = %+v, want empty with the three hidden stations demarked", o.Totals)
	}
	if got := count(`SELECT count(*) FROM radio_offer_exceptions WHERE offered = 0 AND station_id IN (3, 25, 59)`); got != 3 {
		t.Errorf("%d hidden stations demarked, want 3 (CH, DE and the own one)", got)
	}
	if count(`SELECT offer_seeded FROM radio_config`) != 1 || count(`SELECT count(*) FROM radio_favorites`) != 6 {
		t.Error("the flag is not set or a favourite went")
	}
	if f, err := s.Facets(ctx, catSession(1), model.RadioQuery{}); err != nil || !f.OfferEmpty {
		t.Errorf("facets after the first start: offerEmpty %v, %v", f.OfferEmpty, err)
	}
	for what, query := range map[string]string{
		"tag rules":       `SELECT count(*) FROM radio_tag_rules`,
		"manual facets":   `SELECT count(*) FROM radio_station_facets WHERE manual = 1`,
		"facet locks":     `SELECT count(*) FROM radio_station_locks WHERE field IN ('genre', 'kind', 'mood')`,
		"no provider":     `SELECT count(*) FROM radio_stations s WHERE provider IS NULL AND source = 'catalog' AND EXISTS (SELECT 1 FROM radio_streams WHERE station_id = s.id)`,
		"own provider":    `SELECT count(*) FROM radio_stations WHERE id = 59 AND provider IS NOT NULL`,
		"radio.test host": `SELECT count(*) FROM radio_stations WHERE id = 1 AND provider <> 'radio.test'`,
	} {
		if n := count(query); n != 0 {
			t.Errorf("%s: %d rows", what, n)
		}
	}
	def := rules.Default()
	for id, tags := range map[int64][]string{1: {"pop", "test"}, 7: nil, 59: nil} {
		var name string
		if err := db.Read.QueryRowContext(ctx, `SELECT name FROM radio_stations WHERE id = ?`, id).Scan(&name); err != nil {
			t.Fatal(err)
		}
		want, _ := deriveFacets(def, name, tags)
		if got := storedFacets(t, db, id); !sameFacets(got, want) {
			t.Errorf("station %d facets %v, want %v from the rules file", id, got, want)
		}
	}
	// Choosing CH brings the Swiss favourites back, not the hidden own one.
	if _, err := s.ApplyOffer(ctx, 0, []model.RadioOfferStep{{Op: model.RadioOfferChooseCountry, Country: "CH"}}); err != nil {
		t.Fatal(err)
	}
	favs, err := s.Favorites(ctx, catSession(1))
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, st := range favs {
		ids = append(ids, st.ID)
	}
	if !slices.Equal(ids, []int64{5, 4, 1}) {
		t.Errorf("favourites with CH chosen = %v, want 5, 4, 1 (CH, in their order)", ids)
	}
	if _, err := s.Station(ctx, catSession(1), 3); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("the hidden Swiss station with CH chosen: %v, want not found", err)
	}
	// A second start finds the flag and changes nothing.
	before := count(`SELECT count(*) FROM radio_offer_exceptions`)
	if err := New(db, catDiscard).Start(ctx); err != nil {
		t.Fatal(err)
	}
	if after := count(`SELECT count(*) FROM radio_offer_exceptions`); after != before || count(`SELECT count(*) FROM radio_offer_countries`) != 1 {
		t.Errorf("a second start changed the offer: %d exceptions, were %d", after, before)
	}
}

// sameFacets reports whether two facet maps hold the same values per facet.
func sameFacets(a, b map[string][]string) bool {
	for _, f := range facetNames {
		if !slices.Equal(a[f], b[f]) {
			return false
		}
	}
	return true
}
