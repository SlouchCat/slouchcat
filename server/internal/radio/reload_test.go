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

// The entry of one station equals its entry in the full load, facets included, and both
// carry the best stream of ListRadioStreamsRanked: a worse preferred stream wins only under the lock
// "stream", and a station without streams has no codec.
func TestLoadEntryMatchesFullLoad(t *testing.T) {
	e := newCatEnv(t)
	e.sync()
	name := "Changed Name"
	if _, err := e.s.UpdateStation(e.ctx, e.stationID("Radio 24"), model.AdminRadioStationPatch{Name: &name}); err != nil {
		t.Fatal(err)
	}
	lockedPref, freePref, noStreams := e.stationID("Testradio Eins"), e.stationID("Testradio Zwei"), e.stationID("Testradio Drei")
	for _, id := range []int64{lockedPref, freePref} {
		e.exec(`INSERT INTO radio_streams (station_id, url, codec, bitrate, lastcheckok, preferred, updated_at)
			VALUES (?, 'http://example.invalid/worse', 'OGG', 32, 0, 1, 0)`, id)
	}
	e.exec(`INSERT INTO radio_station_locks (station_id, field) VALUES (?, 'stream')`, lockedPref)
	e.exec(`DELETE FROM radio_streams WHERE station_id = ?`, noStreams)
	q := appq.New(e.db.Read)
	all, err := loadEntries(e.ctx, q, sql.NullInt64{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range all {
		got, err := loadEntries(e.ctx, q, sql.NullInt64{Int64: want.st.ID, Valid: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
			t.Errorf("station %d: entry %+v, want %+v", want.st.ID, got, want)
		}
		streams, err := q.ListRadioStreamsRanked(e.ctx, want.st.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(streams) == 0 {
			if want.st.Codec != "" || want.st.BitrateKbps != 0 {
				t.Errorf("station %d without streams: codec %q, bitrate %d", want.st.ID, want.st.Codec, want.st.BitrateKbps)
			}
			continue
		}
		if want.st.Codec != streams[0].Codec.String || want.st.BitrateKbps != int(streams[0].Bitrate.Int64) {
			t.Errorf("station %d: best stream %s/%d, ranked first %s/%d", want.st.ID, want.st.Codec, want.st.BitrateKbps,
				streams[0].Codec.String, streams[0].Bitrate.Int64)
		}
	}
	if len(all) == 0 {
		t.Error("no station loaded")
	}
	if got := codecOf(t, all, lockedPref); got != "OGG" {
		t.Errorf("preferred stream under the lock: best codec %q, want OGG", got)
	}
	if got := codecOf(t, all, freePref); got == "OGG" {
		t.Error("a preferred stream without the lock won")
	}
	gone, err := loadEntries(e.ctx, q, sql.NullInt64{Int64: 1 << 40, Valid: true})
	if err != nil || len(gone) != 0 {
		t.Errorf("a missing station loaded %v, %v", gone, err)
	}
}

// codecOf returns the codec of the station id in entries.
func codecOf(t *testing.T, entries []*indexEntry, id int64) string {
	t.Helper()
	for _, e := range entries {
		if e.st.ID == id {
			return e.st.Codec
		}
	}
	t.Fatalf("station %d not loaded", id)
	return ""
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
		"ListRadioStationTags", "ListRadioStationLocks", "ListRadioStreamsRanked", "GetRadioStation",
		// Own stations (stage 2); the delete also through its cascades (migration 00039 indexes
		// radio_favorites and radio_listen_sessions by station).
		"UpdateRadioOwnStation", "UpdateRadioOwnStreamURL", "SetRadioOwnStreamProbe", "SetRadioStationQuality",
		"DeleteRadioOwnStation"} {
		query := oneStatement(t, src, name)
		plan := explain(t, e, query)
		// Every SCAN except the materialised CTE (subquery-N, the alias r of its one row) reads a
		// whole table; a plan without SEARCH means the statement was not explained at all.
		for _, line := range strings.Split(plan, "\n") {
			if strings.HasPrefix(line, "SCAN ") && !strings.HasPrefix(line, "SCAN (subquery-") && !strings.HasPrefix(line, "SCAN r ") {
				t.Errorf("%s scans a table (%s):\n%s", name, line, plan)
			}
		}
		if !strings.Contains(plan, "SEARCH ") {
			t.Errorf("%s has no keyed read:\n%s", name, plan)
		}
	}
	// The daily delete of the missing catalog stations reads radio_stations as a whole by design,
	// but its cascades go through the keys of the station.
	plan := explain(t, e, oneStatement(t, src, "DeleteMissingRadioStations"))
	for _, line := range strings.Split(plan, "\n") {
		if strings.HasPrefix(line, "SCAN ") && !strings.HasPrefix(line, "SCAN radio_stations") {
			t.Errorf("DeleteMissingRadioStations scans a table in its cascade (%s):\n%s", line, plan)
		}
	}
}

// explain returns the detail lines of EXPLAIN QUERY PLAN for query, its parameters bound to 1.
// namedParam finds them.
func explain(t *testing.T, e *catEnv, query string) string {
	t.Helper()
	var args []any
	seen := map[string]bool{}
	for _, m := range namedParam.FindAllStringSubmatch(query, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			args = append(args, sql.Named(m[1], 1))
		}
	}
	rows, err := e.db.Read.QueryContext(e.ctx, "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v\n%s", err, query)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return plan.String()
}

// oneStatement returns the statement of the named query in src without its comment lines (a
// comment may hold a semicolon), cut at its semicolon, with the sqlc macros as parameters.
func oneStatement(t *testing.T, src []byte, name string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)-- name: ` + name + ` :\w+\n(.*?)(?:\n-- name: |\z)`).FindSubmatch(src)
	if m == nil {
		t.Fatalf("query %s not found", name)
	}
	var b strings.Builder
	for _, line := range strings.Split(string(m[1]), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			b.WriteString(line + "\n")
		}
	}
	stmt, _, ok := strings.Cut(b.String(), ";")
	if !ok {
		t.Fatalf("query %s has no semicolon", name)
	}
	return sqlcArg.ReplaceAllString(stmt, "@$1")
}

var namedParam = regexp.MustCompile(`@(\w+)`)

// sqlcArg is a sqlc.arg or sqlc.narg macro, which SQLite reads as the named parameter.
var sqlcArg = regexp.MustCompile(`sqlc\.n?arg\('(\w+)'\)`)

// BenchmarkAdminStationsLarge measures one page of the admin list (the whole catalog) over the
// synthetic catalog of 50,000 stations.
// go test ./internal/radio -run '^$' -bench BenchmarkAdminStationsLarge -benchtime 5x
func BenchmarkAdminStationsLarge(b *testing.B) {
	e := newCatEnv(b)
	raw := synthCatalog(50000)
	e.body.Store(&raw)
	e.sync()
	for b.Loop() {
		page, err := e.s.AdminStations(e.ctx, model.AdminRadioQuery{Limit: 50})
		if err != nil || len(page.Items) != 50 {
			b.Fatalf("page %d items, %v", len(page.Items), err)
		}
	}
}
