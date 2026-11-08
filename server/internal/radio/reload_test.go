package radio

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The entry of one station equals its entry in the full load, locks and facets included.
func TestLoadEntryMatchesFullLoad(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	name := "Changed Name"
	if _, err := e.s.UpdateStation(e.ctx, e.stationID("Radio 24"), model.AdminRadioStationPatch{Name: &name}); err != nil {
		t.Fatal(err)
	}
	q := appq.New(e.db.Read)
	all, err := loadEntries(e.ctx, q, sql.NullInt64{})
	if err != nil {
		t.Fatal(err)
	}
	locked := 0
	for _, want := range all {
		got, err := loadEntries(e.ctx, q, sql.NullInt64{Int64: want.st.ID, Valid: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
			t.Errorf("station %d: entry %+v, want %+v", want.st.ID, got, want)
		}
		if want.locked {
			locked++
		}
	}
	if len(all) == 0 || locked != 1 {
		t.Errorf("%d stations, %d locked; want some and one", len(all), locked)
	}
	gone, err := loadEntries(e.ctx, q, sql.NullInt64{Int64: 1 << 40, Valid: true})
	if err != nil || len(gone) != 0 {
		t.Errorf("a missing station loaded %v, %v", gone, err)
	}
}

// The queries of one station read through its keys and never scan a whole table: the reload
// after each logo and each row of the admin list run them (55,000 stations on CT 190).
func TestOneStationQueriesUseKeys(t *testing.T) {
	e := newCatEnv(t)
	src, err := os.ReadFile(filepath.Join("..", "..", "queries", "app", "radio_catalog.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"GetRadioStationForIndex", "ListRadioStationFacetsByStation",
		"ListRadioStationTags", "ListRadioStationLocks", "ListRadioStreamsRanked", "GetRadioStation"} {
		m := regexp.MustCompile(`(?s)-- name: ` + name + ` :\w+\n(.*?);`).FindSubmatch(src)
		if m == nil {
			t.Fatalf("query %s not found", name)
		}
		rows, err := e.db.Read.QueryContext(e.ctx, "EXPLAIN QUERY PLAN "+string(m[1]), sql.Named("station_id", 1), sql.Named("id", 1))
		if err != nil {
			t.Fatalf("explain %s: %v", name, err)
		}
		var plan strings.Builder
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan.WriteString(detail + "\n")
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
		if regexp.MustCompile(`(?m)^SCAN (radio_|st\b|s\b|l\b)`).MatchString(plan.String()) {
			t.Errorf("%s scans a table:\n%s", name, plan.String())
		}
	}
}

// BenchmarkAdminStationsLarge measures one page of the admin list (exceptions) over the synthetic
// catalog of 40,000 stations, every station recommended so the page is full.
// go test ./internal/radio -run '^$' -bench BenchmarkAdminStationsLarge -benchtime 5x
func BenchmarkAdminStationsLarge(b *testing.B) {
	e := newCatEnv(b)
	raw := synthCatalog(50000)
	e.body.Store(&raw)
	e.sync()
	if _, err := e.db.Write.ExecContext(e.ctx, `UPDATE radio_stations SET recommended = 1`); err != nil {
		b.Fatal(err)
	}
	if _, err := e.s.rebuildIndex(e.ctx); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		page, err := e.s.AdminStations(e.ctx, model.AdminRadioQuery{Exceptions: true, Limit: 50})
		if err != nil || len(page.Items) != 50 {
			b.Fatalf("page %d items, %v", len(page.Items), err)
		}
	}
}
