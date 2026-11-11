package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Migration 00051 (concept 19 part B stage B2, 1.30.0) adds the Wikidata matches and the two
// settings of radio_config: from the frozen state of stage B1 (radio-angebot.sql) the table is
// empty, «Wikidata verwenden» is on and no run is recorded; the CHECKs refuse a bad item, owner or
// date, and a station that goes takes its match along.
func TestMigration00051RadioWikidata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "radio-angebot.sql"))
	db, err := Open(ctx, Options{ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	text := func(query string) string {
		t.Helper()
		var s string
		if err := db.Read.QueryRowContext(ctx, query).Scan(&s); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return s
	}
	for query, want := range map[string]string{
		`SELECT count(*) FROM radio_station_wikidata`:                                      "0",
		`SELECT wikidata_enabled || '/' || (wikidata_synced_at IS NULL) FROM radio_config`: "1/1",
		`SELECT offer_version || '/' || offer_seeded FROM radio_config`:                    "2/1",
		`SELECT count(*) FROM pragma_foreign_key_check`:                                    "0",
		`SELECT count(*) FROM goose_db_version WHERE version_id = 51 AND is_applied = 1`:   "1",
	} {
		if got := text(query); got != want {
			t.Errorf("%s = %s, want %s", query, got, want)
		}
	}
	for _, stmt := range []string{
		`INSERT INTO radio_station_wikidata (station_id, qid, fetched_at) VALUES (1, 'X1', 1)`,
		`INSERT INTO radio_station_wikidata (station_id, qid, fetched_at) VALUES (1, 'Q0', 1)`,
		`INSERT INTO radio_station_wikidata (station_id, qid, owner, fetched_at) VALUES (1, 'Q1', 'Owner', 1)`,
		`INSERT INTO radio_station_wikidata (station_id, qid, owner_qid, owner, fetched_at) VALUES (1, 'Q1', 'Q2', '', 1)`,
		`INSERT INTO radio_station_wikidata (station_id, qid, founded, fetched_at) VALUES (1, 'Q1', '83', 1)`,
		`INSERT INTO radio_station_wikidata (station_id, qid, founded, fetched_at) VALUES (1, 'Q1', '1983-1-1', 1)`,
		`INSERT INTO radio_station_wikidata (station_id, qid, fetched_at) VALUES (9999, 'Q1', 1)`,
		`UPDATE radio_config SET wikidata_enabled = 2`,
	} {
		if _, err := db.Write.ExecContext(ctx, stmt); err == nil {
			t.Errorf("accepted: %s", stmt)
		}
	}
	for _, stmt := range []string{
		`INSERT INTO radio_station_wikidata (station_id, qid, owner_qid, owner, founded, fetched_at) VALUES (1, 'Q1157782', 'Q1441462', 'SRF', '1983-11-01', 1)`,
		`INSERT INTO radio_station_wikidata (station_id, qid, founded, fetched_at) VALUES (2, 'Q9', '1983', 1), (3, 'Q10', '1983-08', 1)`,
		`DELETE FROM radio_stations WHERE id = 2`,
	} {
		if _, err := db.Write.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if got := text(`SELECT group_concat(station_id) FROM radio_station_wikidata`); got != "1,3" {
		t.Errorf("matches after the delete of station 2 = %s, want 1,3", got)
	}
}
