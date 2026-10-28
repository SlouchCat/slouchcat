package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Migration 00022 (ADR 0014) rebuilds accounts: role keeps only 'admin' and 'user', the owner moves
// into the new column is_owner, and two CHECKs hold the model together. TestFrozenStatesMigrate
// already runs it over every frozen state; this test proves what the rebuild must not lose and
// what the new constraints must refuse.
//
// The frozen M11 state holds one owner. The test adds a guest with its own ceiling, a library
// right and the three W-8 caps, so the columns that came after 00002 are carried over as well —
// a forgotten column in the INSERT ... SELECT would vanish without a sound.
func TestMigration00022RenamesTheRolesAndMarksTheOwner(t *testing.T) {
	cfg := migratedM11WithGuest(t)
	db, err := Open(context.Background(), Options{
		ConfigDir: cfg, Version: "migration-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// The owner became an administrator and carries the badge; the guest became a plain account.
	type row struct {
		email    string
		role     string
		isOwner  int
		rating   int
		allLibs  int
		bitrate  *int
		height   *int
		streams  *int
		invitedB *int64
	}
	rows, err := db.Read.Query(`SELECT email, role, is_owner, max_rating, all_libraries,
	                                   max_bitrate_kbps, max_height, max_streams, invited_by
	                            FROM accounts ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.email, &r.role, &r.isOwner, &r.rating, &r.allLibs, &r.bitrate, &r.height, &r.streams, &r.invitedB); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("accounts = %+v, want the frozen owner and the added guest", got)
	}
	owner, guest := got[0], got[1]
	if owner.role != "admin" || owner.isOwner != 1 || owner.rating != 18 || owner.allLibs != 1 {
		t.Errorf("owner = %+v, want role admin, is_owner 1, 18, all libraries", owner)
	}
	if guest.role != "user" || guest.isOwner != 0 || guest.rating != 12 || guest.allLibs != 0 {
		t.Errorf("guest = %+v, want role user, is_owner 0, 12, not all libraries", guest)
	}
	// The columns that 00007 and 00011 added survive the rebuild character for character.
	if guest.bitrate == nil || *guest.bitrate != 4000 || guest.height == nil || *guest.height != 720 ||
		guest.streams == nil || *guest.streams != 2 || guest.invitedB == nil || *guest.invitedB == 0 {
		t.Errorf("guest limits and invited_by = %+v, want 4000/720/2 and the owner's id", guest)
	}
	// Its library right and its profile still hang on the same id.
	var libs, profiles int
	if err := db.Read.QueryRow(`SELECT count(*) FROM account_libraries`).Scan(&libs); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRow(`SELECT count(*) FROM profiles WHERE account_id = (SELECT id FROM accounts WHERE role = 'user')`).Scan(&profiles); err != nil {
		t.Fatal(err)
	}
	if libs != 1 || profiles != 1 {
		t.Errorf("account_libraries = %d, guest profiles = %d; want 1 and 1", libs, profiles)
	}

	// Exactly one owner, and the owner is an administrator.
	var owners, wrong int
	if err := db.Read.QueryRow(`SELECT count(*) FROM accounts WHERE is_owner = 1`).Scan(&owners); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRow(`SELECT count(*) FROM accounts WHERE is_owner = 1 AND role <> 'admin'`).Scan(&wrong); err != nil {
		t.Fatal(err)
	}
	if owners != 1 || wrong != 0 {
		t.Errorf("owners = %d, owners that are no administrators = %d; want 1 and 0", owners, wrong)
	}

	// A forgotten role creates a harmless account, not a silent administrator.
	if _, err := db.Write.Exec(`INSERT INTO accounts (email, password_hash, created_at, updated_at)
	                            VALUES ('default@example.test', 'x', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	var role string
	var isOwner int
	if err := db.Read.QueryRow(`SELECT role, is_owner FROM accounts WHERE email = 'default@example.test'`).Scan(&role, &isOwner); err != nil {
		t.Fatal(err)
	}
	if role != "user" || isOwner != 0 {
		t.Errorf("default role = %q, is_owner = %d; want user and 0", role, isOwner)
	}

	// What the two CHECKs and the partial index refuse.
	refusals := map[string]string{
		"an account below FSK 18 cannot become an administrator": `UPDATE accounts SET role = 'admin' WHERE role = 'user' AND max_rating = 12`,
		"a second owner": `UPDATE accounts SET is_owner = 1 WHERE email = 'default@example.test'`,
		"an owner that is not an administrator": `INSERT INTO accounts (email, password_hash, role, is_owner, created_at, updated_at)
		                                          VALUES ('x@example.test', 'x', 'user', 1, 1, 1)`,
		"a third role value": `INSERT INTO accounts (email, password_hash, role, created_at, updated_at)
		                       VALUES ('y@example.test', 'x', 'owner', 1, 1)`,
	}
	names := make([]string, 0, len(refusals))
	for name := range refusals {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := db.Write.Exec(refusals[name]); err == nil {
			t.Errorf("%s: the database accepted it", name)
		}
	}

	// The view came back and still reads the account ceiling (hard rule 2): the guest's profile
	// sees nothing above FSK 12 even though the profile itself allows 18.
	var visible int
	if err := db.Read.QueryRow(`SELECT count(*) FROM visible_items AS v
	                            JOIN items AS i ON i.id = v.item_id
	                            JOIN profiles AS p ON p.id = v.profile_id
	                            WHERE p.account_id = (SELECT id FROM accounts WHERE max_rating = 12)
	                              AND COALESCE(i.effective_rating, 18) > 12`).Scan(&visible); err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Errorf("the account ceiling let %d titles above FSK 12 through", visible)
	}
}

// migratedM11WithGuest replays the frozen M11 state plus a guest account with everything that
// accounts gained after 00002, and returns the config directory to open.
func migratedM11WithGuest(t *testing.T) string {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "M11.sql"))
	if err != nil {
		t.Fatal(err)
	}
	const extra = `
INSERT INTO accounts (email, password_hash, role, max_rating, all_libraries, invited_by, last_seen_at,
                      max_bitrate_kbps, max_height, max_streams, created_at, updated_at)
SELECT 'gast@example.test', 'x', 'guest', 12, 0, a.id, 1793097400000, 4000, 720, 2, 1793097380000, 1793097380000
FROM accounts AS a WHERE a.role = 'owner';
INSERT INTO account_libraries (account_id, library_id)
SELECT a.id, l.id FROM accounts AS a JOIN libraries AS l WHERE a.role = 'guest' LIMIT 1;
INSERT INTO profiles (account_id, name, max_rating, language, all_libraries, is_default, created_at, updated_at)
SELECT a.id, 'Gast', 18, 'de', 1, 1, 1, 1 FROM accounts AS a WHERE a.role = 'guest';
`
	cfg := t.TempDir()
	replay(t, filepath.Join(cfg, FileName), strings.Join([]string{string(script), extra}, "\n"))
	return cfg
}
