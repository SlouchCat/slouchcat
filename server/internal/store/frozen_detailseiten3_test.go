package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 17 stage 3 (migration 00040; testdata/db/detailseiten3.sql) carries
// the image rows of the stage: a person with an ok portrait in both formats, a person with a missing
// row, a TMDB collection with an ok poster in both formats and one with a missing row, both with
// members. TestFrozenStatesMigrate replays it like every state; this test checks that each of those
// is really in it.
func TestFrozenDetailseiten3StateCoversTheConcept(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "detailseiten3.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"a person portrait as jpeg and webp": `SELECT count(*) = 2 FROM person_images pi JOIN people p ON p.id = pi.person_id
			WHERE pi.status = 'ok' AND pi.source = p.profile_path AND pi.size = 'w185' AND pi.person_id = 1`,
		"a missing person portrait": `SELECT count(*) FROM person_images WHERE status = 'missing' AND format = 'jpeg' AND rel_path IS NULL`,
		"a collection poster as jpeg and webp": `SELECT count(*) = 2 FROM collection_images ci JOIN collections c ON c.id = ci.collection_id
			WHERE ci.status = 'ok' AND c.source = 'tmdb' AND ci.source = c.poster_path AND ci.size = 'w400'`,
		"a missing collection poster":           `SELECT count(*) FROM collection_images WHERE status = 'missing' AND format = 'jpeg' AND rel_path IS NULL`,
		"members of both collections":           `SELECT count(DISTINCT collection_id) = 2 FROM collection_items WHERE collection_id IN (SELECT collection_id FROM collection_images)`,
		"a person without a portrait path":      `SELECT count(*) FROM people WHERE profile_path IS NULL`,
		"the stage 2 radio data kept":           `SELECT count(*) FROM radio_stations WHERE source = 'own'`,
		"migration 00040 recorded as applied":   `SELECT count(*) FROM goose_db_version WHERE version_id = 40 AND is_applied = 1`,
		"foreign keys of the frozen state hold": `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen detailseiten3 state has no %s", what)
		}
	}
}
