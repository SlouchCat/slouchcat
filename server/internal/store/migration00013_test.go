package store

import (
	"context"
	"database/sql"
	"io/fs"
	"regexp"
	"testing"
	"testing/fstest"

	"github.com/slouchcat/slouchcat/server/migrations"
)

// Migration 00013 gives every profile that already has a PIN a salt of its own (32 lower-case hex
// characters, AN-19) and none to profiles without a PIN (M5 review K12). The state before 00013 is
// built here: the migrations up to 00012, then an account with two PIN profiles and one without.
func TestMigration00013SaltsExistingPins(t *testing.T) {
	ctx := context.Background()
	before := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() >= "00013" {
			continue
		}
		data, err := fs.ReadFile(migrations.FS, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		before[e.Name()] = &fstest.MapFile{Data: data}
	}
	dir := t.TempDir()
	opts := testOptions(t, dir)
	opts.migrations = before
	db, err := Open(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := queryInt(t, db.Read, `SELECT count(*) FROM pragma_table_info('profiles') WHERE name = 'pin_salt'`); got != val(0) {
		t.Fatalf("pin_salt exists before 00013 (%v)", got)
	}
	mustExec(t, db.Write, `INSERT INTO accounts (id, email, password_hash, role, created_at, updated_at) VALUES (1, 'k12@example.test', 'x', 'owner', 1, 1)`)
	mustExec(t, db.Write, `INSERT INTO profiles (id, account_id, name, pin_hash, created_at, updated_at) VALUES
	         (1, 1, 'Eins', '$argon2id$one', 1, 1), (2, 1, 'Zwei', '$argon2id$two', 1, 1), (3, 1, 'Ohne', NULL, 1, 1)`)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(ctx, testOptions(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	salts := map[int64]sql.NullString{}
	rows, err := db.Read.Query(`SELECT id, pin_salt FROM profiles ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var salt sql.NullString
		if err := rows.Scan(&id, &salt); err != nil {
			t.Fatal(err)
		}
		salts[id] = salt
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	hex32 := regexp.MustCompile(`^[0-9a-f]{32}$`)
	for _, id := range []int64{1, 2} {
		if !salts[id].Valid || !hex32.MatchString(salts[id].String) {
			t.Errorf("profile %d with PIN: salt %+v, want 32 hex characters", id, salts[id])
		}
	}
	if salts[1].String == salts[2].String {
		t.Errorf("both PIN profiles got the salt %q", salts[1].String)
	}
	if salts[3].Valid {
		t.Errorf("profile without PIN got the salt %q", salts[3].String)
	}
}
