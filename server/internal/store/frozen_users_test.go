package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// The frozen user-management state carries all three levels of ADR 0014 (testdata/db/README.md):
// the owner with two profiles, an account that administers nothing with its own ceiling, library
// switch and W-8 caps, and a second administrator that was appointed and therefore sits at FSK 18
// with all libraries. A later migration that touches accounts has to carry every one of them.
//
// TestFrozenStatesMigrate picks the file up through its glob and runs the migrations over it;
// this test is about what the state contains, not about migrating it.
func TestFrozenUserManagementStateCoversTheThreeLevels(t *testing.T) {
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "db", "benutzerverwaltung.sql"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := t.TempDir()
	replay(t, filepath.Join(cfg, FileName), string(script))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"owner":                                `SELECT count(*) FROM accounts WHERE is_owner = 1 AND role = 'admin'`,
		"second administrator":                 `SELECT count(*) FROM accounts WHERE is_owner = 0 AND role = 'admin' AND max_rating = 18 AND all_libraries = 1`,
		"account that administers nothing":     `SELECT count(*) FROM accounts WHERE role = 'user' AND max_rating = 12`,
		"account with W-8 caps":                `SELECT count(*) FROM accounts WHERE role = 'user' AND max_height IS NOT NULL AND max_streams IS NOT NULL`,
		"account that came from an invitation": `SELECT count(*) FROM accounts WHERE invited_by IS NOT NULL`,
		"used invitation":                      `SELECT count(*) FROM invitations WHERE used_by IS NOT NULL`,
		"child profile of the owner":           `SELECT count(*) FROM profiles p JOIN accounts a ON a.id = p.account_id WHERE a.is_owner = 1 AND p.is_default = 0 AND p.max_rating = 6`,
		"primary profile per account":          `SELECT min(n) FROM (SELECT count(*) AS n FROM profiles WHERE is_default = 1 GROUP BY account_id)`,
		"channel rows of every account":        `SELECT count(*) FROM account_channels`,
		"scanned film library":                 `SELECT count(*) FROM items WHERE kind = 'movie'`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if n == 0 {
			t.Errorf("the frozen user-management state has no %s", what)
		}
	}

	// Exactly one owner, and the three accounts are the three levels and nothing else.
	var owners, accounts int
	if err := db.Read.QueryRow(`SELECT count(*) FROM accounts WHERE is_owner = 1`).Scan(&owners); err != nil {
		t.Fatal(err)
	}
	if err := db.Read.QueryRow(`SELECT count(*) FROM accounts`).Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if owners != 1 || accounts != 3 {
		t.Errorf("owners = %d, accounts = %d; want 1 and 3", owners, accounts)
	}
}
