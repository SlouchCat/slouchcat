package store

import "testing"

// The frozen state before migration 00065 (testdata/db/ordner-metadaten.sql) is
// freigabelaender.sql after 00064 with a second series folder in library 2, a season 2 of «Slouchcat
// Test Show» in it whose file is older than every file of season 1, a missing file of «Slouchcat
// Test Intro Show» there, older than its present ones, and a second library deviation (library 1,
// text languages only). TestFrozenStatesMigrate replays it like every state; this test checks that
// each of those is really in it before the migration.
func TestFrozenOrdnerMetadatenStateCoversTheStage(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	path := cfg + "/" + FileName
	replay(t, path, frozenState(t, "ordner-metadaten.sql"))
	raw := openRaw(t, path)
	for what, query := range map[string]string{
		"migration 00064 as the last one":            `SELECT count(*) FROM goose_db_version WHERE version_id = 64 AND is_applied = 1 AND NOT EXISTS (SELECT 1 FROM goose_db_version WHERE version_id > 64)`,
		"library with two series folders":            `SELECT count(*) FROM library_roots WHERE library_id = 2 AND content = 'series' GROUP BY library_id HAVING count(*) = 2`,
		"deviation with texts and images":            `SELECT count(*) FROM library_metadata WHERE library_id = 2 AND text_langs IS NOT NULL AND images IS NOT NULL`,
		"deviation with texts only":                  `SELECT count(*) FROM library_metadata WHERE library_id = 1 AND text_langs IS NOT NULL AND images IS NULL`,
		"series with files in both folders":          `SELECT count(*) FROM items AS s WHERE s.kind = 'series' AND (SELECT count(DISTINCT m.root_id) FROM media_files AS m JOIN items AS e ON e.id = m.item_id JOIN items AS p ON p.id = e.parent_id WHERE p.parent_id = s.id) = 2`,
		"its oldest file in the second folder":       `SELECT count(*) FROM media_files WHERE root_id = 5 AND missing_since IS NULL AND created_at < (SELECT min(created_at) FROM media_files WHERE root_id = 2)`,
		"an older missing file in the second folder": `SELECT count(*) FROM media_files WHERE root_id = 5 AND missing_since IS NOT NULL`,
		"a feed-only podcast":                        `SELECT count(*) FROM items AS p WHERE p.kind = 'podcast' AND NOT EXISTS (SELECT 1 FROM media_files AS m JOIN items AS e ON e.id = m.item_id WHERE e.parent_id = p.id)`,
	} {
		var n int
		if err := raw.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("frozen ordner-metadaten state has no %s", what)
		}
	}
}
