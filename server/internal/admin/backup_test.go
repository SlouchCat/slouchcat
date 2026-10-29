package admin

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"filippo.io/age"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store"
)

const (
	testPassphrase = "FAKE-backup-passphrase-1"
	testLogN       = 10 // scrypt work factor of the tests (production: 18)
	fakeTMDBSecret = "FAKE-tmdb-key-for-the-backup-test"
)

// stack is a config directory with its database, key file and admin service.
type stack struct {
	dir  string
	db   *store.DB
	ring *secrets.Keyring
	svc  *Service
	logs *bytes.Buffer
}

func newStack(t testing.TB, serverVersion string) *stack {
	t.Helper()
	var logs bytes.Buffer
	log, err := logging.New(&logs, logging.FormatJSON, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	st := &stack{dir: t.TempDir(), logs: &logs}
	st.db, err = store.Open(context.Background(), store.Options{ConfigDir: st.dir, Version: serverVersion, Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.db.Close() })
	if st.ring, err = secrets.Load(st.dir, log); err != nil {
		t.Fatal(err)
	}
	st.svc = New(st.db, auth.New(st.db, log), &fakeLibraries{}, st.ring, &fakeTMDB{}, func(string) string { return "" }, log).
		WithBackups(st.dir, serverVersion)
	st.svc.backups.scryptLogN = testLogN
	return st
}

// seed fills the stack with two accounts, three profiles, a title, progress and its events, a
// session with a playback session, a device with a PIN attempt, a push subscription, a Telegram
// link code and settings (one encrypted). It returns the ciphertext of the encrypted setting.
func (st *stack) seed(t *testing.T) string {
	t.Helper()
	enc, err := st.ring.Encrypt(fakeTMDBSecret)
	if err != nil {
		t.Fatal(err)
	}
	exec(t, st.db.Write,
		`INSERT INTO accounts (id, email, password_hash, role, created_at, updated_at) VALUES
		   (1, 'owner@example.test', '$argon2id$fake', 'admin', 1, 1), (2, 'guest@example.test', '$argon2id$fake', 'user', 2, 2)`,
		`INSERT INTO profiles (id, account_id, name, max_rating, is_default, created_at, updated_at) VALUES
		   (1, 1, 'Marco', 18, 1, 1, 1), (2, 1, 'Kind', 6, 0, 1, 1), (3, 2, 'Gast', 12, 1, 2, 2)`,
		`INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (1, 'Filme', 'video', 1, 1)`,
		`INSERT INTO items (id, kind, library_id, title, sort_title, year, added_at, created_at, updated_at) VALUES
		   ('01J9000000000000000000BK01', 'movie', 1, 'Film', 'film', 2001, 1, 1, 1)`,
		`INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES
		   (1, '01J9000000000000000000BK01', 60000, 600000, 0, 5), (3, '01J9000000000000000000BK01', 600000, 600000, 1, 6)`,
		`INSERT INTO progress_events (profile_id, item_id, device_id, position_ms, duration_ms, played_ms, seq, state,
		   client_time, server_time, playback_session_id) VALUES
		   (1, '01J9000000000000000000BK01', 'dddddddddddddddddddddddddddddddd', 60000, 600000, 60000, 1, 'paused', 4, 5, '01J9PLAYBACK00000000000001'),
		   (3, '01J9000000000000000000BK01', NULL, 600000, 600000, 540000, 1, 'ended', 5, 6, NULL)`,
		`INSERT INTO devices (id, user_agent, created_at, last_seen_at, type) VALUES ('dddddddddddddddddddddddddddddddd', 'test', 1, 1, 'tv')`,
		`INSERT INTO pin_attempts (profile_id, device_id, failures, locked_until, updated_at) VALUES (2, 'dddddddddddddddddddddddddddddddd', 2, 0, 3)`,
		`INSERT INTO sessions (id, token_hash, account_id, profile_id, device_id, created_at, last_seen_at, expires_at) VALUES
		   (1, 'FAKE-session-token-hash-0123456789abcdef', 1, 1, 'dddddddddddddddddddddddddddddddd', 1, 1, 9999999999999)`,
		`INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, mode, started_at,
		   last_activity_at, expires_at) VALUES ('01J9PLAYBACK00000000000001', 1, 1, 1, '01J9000000000000000000BK01', 'x', 'direct', 4, 5, 9999999999999)`,
		`INSERT INTO push_subscriptions (account_id, endpoint, p256dh, auth, created_at) VALUES
		   (1, 'https://push.example.test/FAKE-endpoint', 'FAKE-p256dh', 'FAKE-auth', 1)`,
		`INSERT INTO telegram_links (account_id, code, expires_at, created_at) VALUES (2, 'ABC123', 9999999999999, 1)`,
	)
	if _, err := st.db.Write.Exec(`INSERT INTO settings (key, value, updated_at) VALUES
		   ('tmdb_api_key', ?, 1), ('public_url', 'https://media.example.test', 1), ('vapid_public_key', 'FAKE-vapid-public', 1)`, enc); err != nil {
		t.Fatal(err)
	}
	return enc
}

func exec(t testing.TB, db *sql.DB, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

// entry is one file of an archive.
type entry struct {
	name     string
	typeflag byte
	data     []byte
}

// readArchive decrypts (with a passphrase) and lists an archive.
func readArchive(t *testing.T, path, passphrase string) []entry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var r io.Reader = f
	if passphrase != "" {
		id, err := age.NewScryptIdentity(passphrase)
		if err != nil {
			t.Fatal(err)
		}
		if r, err = age.Decrypt(f, id); err != nil {
			t.Fatalf("decrypt %s: %v", path, err)
		}
	}
	gz, err := gzip.NewReader(r)
	if err != nil {
		t.Fatalf("%s is no gzip stream: %v", path, err)
	}
	tr := tar.NewReader(gz)
	var out []entry
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, entry{name: h.Name, typeflag: h.Typeflag, data: data})
	}
}

// entryDB writes the database entry of an archive to a file and opens it.
func entryDB(t *testing.T, entries []entry) (*sql.DB, []byte) {
	t.Helper()
	for _, e := range entries {
		if e.name == store.FileName {
			path := filepath.Join(t.TempDir(), "copy.db")
			if err := os.WriteFile(path, e.data, 0o600); err != nil {
				t.Fatal(err)
			}
			db, err := store.OpenFile(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			return db, e.data
		}
	}
	t.Fatal("archive has no database")
	return nil, nil
}

func count(t *testing.T, db *sql.DB, query string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// The archive holds the manifest, the database and – with secrets – the key file, nothing else:
// no media file of a library root, no directory, no link.
func TestBackupArchiveHasOnlyDatabaseAndManifest(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	st.seed(t)
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film (2001).mkv"), []byte("FAKE-media-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	exec(t, st.db.Write, fmt.Sprintf(`INSERT INTO library_roots (library_id, path, created_at) VALUES (1, '%s', 1)`, media))
	st.svc.now = func() time.Time { return time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC) }

	for _, tc := range []struct {
		req        model.BackupRequest
		passphrase string
		want       []string
	}{
		{model.BackupRequest{}, "", []string{"manifest.json", "slouchcat.db"}},
		{model.BackupRequest{IncludeSecrets: true, Passphrase: testPassphrase}, testPassphrase, []string{"manifest.json", "slouchcat.db", "secret.key"}},
	} {
		b, err := st.svc.CreateBackup(ctx, tc.req)
		if err != nil {
			t.Fatal(err)
		}
		wantName := "slouchcat-backup-0.9.0-20261021T120000Z.tar.gz"
		if tc.req.IncludeSecrets {
			wantName = "slouchcat-backup-0.9.0-20261021T120000Z.tar.gz.age"
		}
		if b.Name != wantName || b.Kind != model.BackupManual || b.IncludesSecrets != tc.req.IncludeSecrets ||
			b.Encrypted != tc.req.IncludeSecrets || b.Version != "0.9.0" || b.SizeBytes <= 0 {
			t.Errorf("backup = %+v, want %s", b, wantName)
		}
		path := filepath.Join(st.dir, "backups", b.Name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 || info.Size() != b.SizeBytes {
			t.Errorf("%s: mode %v, size %d; want 0600 and %d", b.Name, info.Mode().Perm(), info.Size(), b.SizeBytes)
		}
		entries := readArchive(t, path, tc.passphrase)
		var names []string
		for _, e := range entries {
			names = append(names, e.name)
			if e.typeflag != tar.TypeReg {
				t.Errorf("%s: entry %s has type %c, want a regular file", b.Name, e.name, e.typeflag)
			}
			if bytes.Contains(e.data, []byte("FAKE-media-bytes")) {
				t.Errorf("%s: entry %s carries the media file", b.Name, e.name)
			}
		}
		if !slices.Equal(names, tc.want) {
			t.Errorf("%s: entries %v, want %v", b.Name, names, tc.want)
		}
		var m backupManifest
		if err := json.Unmarshal(entries[0].data, &m); err != nil {
			t.Fatal(err)
		}
		// Private feed URLs keep their tokens in every backup, and the manifest says so (review M-6).
		if m.Format != 1 || m.Version != "0.9.0" || !m.CreatedAt.Equal(b.CreatedAt) || m.IncludesSecrets != tc.req.IncludeSecrets ||
			!m.IncludesFeedCredentials || m.Tables["accounts"] != 2 || m.Tables["profiles"] != 3 || m.Tables["progress_events"] != 2 {
			t.Errorf("%s: manifest %+v", b.Name, m)
		}
		t.Logf("%s: %d bytes, entries %v", b.Name, b.SizeBytes, names)
	}
	if left, _ := os.ReadDir(filepath.Join(st.dir, "tmp")); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

// Without secrets the copy loses the encrypted settings (with the VAPID public key), the push
// subscriptions, the Telegram link codes, the sessions with their playback sessions and the PIN
// attempts; the deleted values are not left in the file either.
func TestBackupWithoutSecretsStripsEncryptedSettings(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	enc := st.seed(t)
	b, err := st.svc.CreateBackup(ctx, model.BackupRequest{Passphrase: "ignored without secrets"})
	if err != nil {
		t.Fatal(err)
	}
	if b.IncludesSecrets || b.Encrypted || !strings.HasSuffix(b.Name, ".tar.gz") {
		t.Fatalf("backup = %+v", b)
	}
	entries := readArchive(t, filepath.Join(st.dir, "backups", b.Name), "")
	db, raw := entryDB(t, entries)
	for query, want := range map[string]int{
		"SELECT count(*) FROM settings WHERE value LIKE 'v1:%'":        0,
		"SELECT count(*) FROM settings WHERE key = 'vapid_public_key'": 0,
		"SELECT count(*) FROM settings WHERE key = 'public_url'":       1,
		"SELECT count(*) FROM push_subscriptions":                      0,
		"SELECT count(*) FROM telegram_links":                          0,
		"SELECT count(*) FROM sessions":                                0,
		"SELECT count(*) FROM playback_sessions":                       0,
		"SELECT count(*) FROM pin_attempts":                            0,
		"SELECT count(*) FROM accounts":                                2,
		"SELECT count(*) FROM progress_events":                         2,
		"SELECT count(*) FROM devices":                                 1,
	} {
		if got := count(t, db, query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	for _, secret := range []string{enc, "FAKE-session-token-hash", "FAKE-endpoint", "FAKE-p256dh", "ABC123", "FAKE-vapid-public"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Errorf("the database file still contains %q", secret)
		}
	}
	if count(t, st.db.Read, "SELECT count(*) FROM settings WHERE value LIKE 'v1:%'") != 1 {
		t.Error("the live database lost its encrypted setting")
	}
}

// A backup with secrets needs a passphrase of at least 12 characters and is encrypted with it.
func TestBackupWithSecretsNeedsPassphrase(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	enc := st.seed(t)
	for _, tc := range []struct {
		passphrase string
		want       error
	}{
		{"", model.ErrPassphraseRequired},
		{"elf-zeichen", model.ErrPassphraseTooShort}, // 11 characters
		{"äöüäöüäöüäö", model.ErrPassphraseTooShort}, // 11 characters, 22 bytes
		{"zwölf-zeiche", nil},                        // 12 characters, 13 bytes
		{testPassphrase, nil},
	} {
		_, err := st.svc.CreateBackup(ctx, model.BackupRequest{IncludeSecrets: true, Passphrase: tc.passphrase})
		if !errors.Is(err, tc.want) {
			t.Errorf("passphrase %q: err = %v, want %v", tc.passphrase, err, tc.want)
		}
	}
	list, err := st.svc.ListBackups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("backups %+v, want the two with a valid passphrase", list)
	}
	b := list[0]
	if !b.Encrypted || !b.IncludesSecrets || !strings.HasSuffix(b.Name, ".tar.gz.age") {
		t.Fatalf("backup = %+v", b)
	}
	path := filepath.Join(st.dir, "backups", b.Name)
	head := make([]byte, len(ageMagic))
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadFull(f, head)
	_ = f.Close()
	if !bytes.Equal(head, ageMagic) {
		t.Fatalf("archive starts with %q, want the age header", head)
	}
	entries := readArchive(t, path, testPassphrase)
	key, err := os.ReadFile(filepath.Join(st.dir, secrets.KeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(entries[2].data, key) {
		t.Error("secret.key of the archive differs from the key file")
	}
	db, _ := entryDB(t, entries)
	var got string
	if err := db.QueryRow("SELECT value FROM settings WHERE key = 'tmdb_api_key'").Scan(&got); err != nil || got != enc {
		t.Errorf("encrypted setting = %q, %v; want it kept", got, err)
	}
	if count(t, db, "SELECT count(*) FROM sessions") != 1 || count(t, db, "SELECT count(*) FROM push_subscriptions") != 1 {
		t.Error("a backup with secrets keeps sessions and push subscriptions")
	}
	if strings.Contains(st.logs.String(), testPassphrase) {
		t.Error("the passphrase reached the log")
	}
}

// tableHash is a hash of every row of a table (in a fixed order, so rowid and WITHOUT ROWID tables
// compare alike), restricted by where.
func tableHash(t *testing.T, db *sql.DB, table, where string) (string, int) {
	t.Helper()
	rows, err := db.Query("SELECT * FROM " + table + " " + where)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, fmt.Sprintf("%#v", vals))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	slices.Sort(lines)
	sum := sha256.Sum256([]byte(strings.Join(cols, ",") + "\n" + strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:8]), len(lines)
}

// reopen closes the stack's database and opens it again, which applies a staged restore.
func (st *stack) reopen(t *testing.T, serverVersion string) {
	t.Helper()
	if err := st.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), store.Options{ConfigDir: st.dir, Version: serverVersion, Logger: st.svc.log})
	if err != nil {
		t.Fatal(err)
	}
	st.db = db
	t.Cleanup(func() { _ = db.Close() })
}

// A-4: a backup of stack A restored on a fresh stack B yields the same accounts, profiles,
// progress, progress events and settings (table hashes); without secrets B keeps its own key and
// gets no encrypted value.
func TestRestoreOnFreshStack(t *testing.T) {
	ctx := context.Background()
	a := newStack(t, "0.9.0")
	a.seed(t)
	keyA, err := os.ReadFile(filepath.Join(a.dir, secrets.KeyFile))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name    string
		secrets bool
	}{{"with secrets", true}, {"without secrets", false}} {
		t.Run(tc.name, func(t *testing.T) {
			req := model.BackupRequest{IncludeSecrets: tc.secrets}
			passphrase := ""
			if tc.secrets {
				req.Passphrase, passphrase = testPassphrase, testPassphrase
			}
			backup, err := a.svc.CreateBackup(ctx, req)
			if err != nil {
				t.Fatal(err)
			}

			b := newStack(t, "0.9.0")
			keyB, err := os.ReadFile(filepath.Join(b.dir, secrets.KeyFile))
			if err != nil {
				t.Fatal(err)
			}
			restarted := make(chan struct{})
			b.svc.WithRestart(func() { close(restarted) })
			archive, err := os.Open(filepath.Join(a.dir, "backups", backup.Name))
			if err != nil {
				t.Fatal(err)
			}
			res, err := b.svc.RestoreBackupUpload(ctx, archive, passphrase)
			_ = archive.Close()
			if err != nil {
				t.Fatal(err)
			}
			if res.Version != "0.9.0" || res.IncludesSecrets != tc.secrets || time.Until(res.RestartAt) < time.Second {
				t.Errorf("restore result %+v", res)
			}
			select {
			case <-restarted:
			case <-time.After(5 * time.Second):
				t.Fatal("restart not requested")
			}
			if _, err := os.Stat(filepath.Join(b.dir, store.RestoreDir)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("restore directory left: %v", err)
			}
			b.reopen(t, "0.9.0")

			settingsWhere := ""
			if !tc.secrets {
				settingsWhere = "WHERE value NOT LIKE 'v1:%' AND key != 'vapid_public_key'"
			}
			for _, tbl := range []struct{ name, where string }{
				{"accounts", ""}, {"profiles", ""}, {"progress", ""}, {"progress_events", ""}, {"settings", settingsWhere},
			} {
				want, n := tableHash(t, a.db.Read, tbl.name, tbl.where)
				got, m := tableHash(t, b.db.Read, tbl.name, "")
				if got != want || n != m || n == 0 {
					t.Errorf("%s: hash %s (%d rows), want %s (%d rows)", tbl.name, got, m, want, n)
				}
				t.Logf("%s: %d rows, hash %s", tbl.name, m, got)
			}
			keyAfter, err := os.ReadFile(filepath.Join(b.dir, secrets.KeyFile))
			if err != nil {
				t.Fatal(err)
			}
			if tc.secrets && !bytes.Equal(keyAfter, keyA) {
				t.Error("the key file of A was not restored")
			}
			if !tc.secrets {
				if bytes.Equal(keyAfter, keyA) || !bytes.Equal(keyAfter, keyB) {
					t.Error("without secrets B keeps its own key")
				}
				if n := count(t, b.db.Read, "SELECT count(*) FROM settings WHERE value LIKE 'v1:%'"); n != 0 {
					t.Errorf("%d encrypted settings restored without secrets", n)
				}
			}
			list, err := b.svc.ListBackups(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(list) != 1 || list[0].Kind != model.BackupAuto || !strings.HasPrefix(list[0].Name, "pre-restore-0.9.0-") ||
				list[0].IncludesSecrets != tc.secrets {
				t.Errorf("backups of B after the restore: %+v, want the pre-restore copy", list)
			}
		})
	}
}

// A backup of a newer server is refused, by its version or by its schema (development builds).
func TestRestoreRejectsNewerVersion(t *testing.T) {
	ctx := context.Background()
	newer := newStack(t, "99.0.0")
	newer.seed(t)
	backup, err := newer.svc.CreateBackup(ctx, model.BackupRequest{})
	if err != nil {
		t.Fatal(err)
	}
	server := newStack(t, "0.9.0")
	if err := os.MkdirAll(filepath.Join(server.dir, "backups"), 0o750); err != nil {
		t.Fatal(err)
	}
	copyTo(t, filepath.Join(newer.dir, "backups", backup.Name), filepath.Join(server.dir, "backups", backup.Name))
	if _, err := server.svc.RestoreBackup(ctx, backup.Name, ""); !errors.Is(err, model.ErrBackupNewerThanServer) {
		t.Errorf("version 99.0.0 on 0.9.0: err = %v, want ErrBackupNewerThanServer", err)
	}

	// Same unparsable version, but the schema has a migration this server does not know.
	dev := newStack(t, "dev")
	archive := craftArchive(t, dev, func(db *sql.DB) {
		exec(t, db, "INSERT INTO goose_db_version (version_id, is_applied) VALUES (99999, 1)")
	}, backupManifest{Format: 1, Version: "dev"})
	if _, err := dev.svc.RestoreBackupUpload(ctx, bytes.NewReader(archive), ""); !errors.Is(err, model.ErrBackupNewerThanServer) {
		t.Errorf("schema 99999: err = %v, want ErrBackupNewerThanServer", err)
	}
	for _, st := range []*stack{server, dev} {
		assertNothingStaged(t, st)
	}
}

// An encrypted backup needs its passphrase.
func TestRestoreWrongPassphrase(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	st.seed(t)
	backup, err := st.svc.CreateBackup(ctx, model.BackupRequest{IncludeSecrets: true, Passphrase: testPassphrase})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		passphrase string
		want       error
	}{
		{"", model.ErrPassphraseRequired},
		{"FAKE-the-wrong-passphrase", model.ErrPassphraseWrong},
	} {
		if _, err := st.svc.RestoreBackup(ctx, backup.Name, tc.passphrase); !errors.Is(err, tc.want) {
			t.Errorf("passphrase %q: err = %v, want %v", tc.passphrase, err, tc.want)
		}
	}
	assertNothingStaged(t, st)
	if _, err := st.svc.RestoreBackup(ctx, backup.Name, testPassphrase); err != nil {
		t.Fatalf("right passphrase: %v", err)
	}
	if _, err := os.Stat(filepath.Join(st.dir, store.RestoreDBFile)); err != nil {
		t.Errorf("right passphrase: nothing staged: %v", err)
	}
	if _, err := os.Stat(filepath.Join(st.dir, store.RestoreKeyFile)); err != nil {
		t.Errorf("right passphrase: key not staged: %v", err)
	}
}

// newBackupName takes the next free second, but gives up after a minute of taken names and on a
// directory it cannot read instead of looping while it holds the backup lock (review M-13).
func TestNewBackupNameIsBounded(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	name, got, err := newBackupName(dir, "0.9.0", at, false)
	if err != nil || name != "slouchcat-backup-0.9.0-20261021T120000Z.tar.gz" || !got.Equal(at) {
		t.Fatalf("free name: %q %v %v", name, got, err)
	}
	for i := range maxNameTries {
		n := "slouchcat-backup-0.9.0-" + at.Add(time.Duration(i)*time.Second).Format(manualTimeLayout) + ".tar.gz"
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if name, _, err := newBackupName(dir, "0.9.0", at, false); err == nil {
		t.Errorf("a minute of taken names: %q, want an error", name)
	}
	if name, got, err := newBackupName(dir, "0.9.0", at, true); err != nil || !got.Equal(at) || !strings.HasSuffix(name, ".age") {
		t.Errorf("encrypted name: %q %v %v", name, got, err)
	}
	notADir := filepath.Join(dir, "file")
	if err := os.WriteFile(notADir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if name, _, err := newBackupName(notADir, "0.9.0", at, false); err == nil {
		t.Errorf("unreadable directory: %q, want an error", name)
	}
}

// An uploaded archive that asks for a scrypt work factor above 2^20 is refused before scrypt runs
// (review M-2: age accepts 2^22, which needs 4 GiB). The archive is a real one whose header names
// 2^21; without the cap the restore would spend 1 GiB and then report a wrong passphrase.
func TestRestoreRefusesHugeScryptWorkFactor(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	st.seed(t)
	backup, err := st.svc.CreateBackup(ctx, model.BackupRequest{IncludeSecrets: true, Passphrase: testPassphrase})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(st.dir, "backups", backup.Name))
	if err != nil {
		t.Fatal(err)
	}
	stanza := regexp.MustCompile(`(?m)^(-> scrypt \S+) 10$`)
	if !stanza.Match(data) {
		t.Fatalf("no scrypt stanza with work factor 10 in the header: %q", data[:min(len(data), 200)])
	}
	data = stanza.ReplaceAll(data, []byte("$1 21"))
	_, err = st.svc.RestoreBackupUpload(ctx, bytes.NewReader(data), testPassphrase)
	if !errors.Is(err, model.ErrBackupInvalid) || !strings.Contains(err.Error(), "work factor too large") {
		t.Errorf("work factor 2^21: err = %v, want ErrBackupInvalid (work factor too large)", err)
	}
	assertNothingStaged(t, st)
}

// Anything but the three regular entries, a wrong manifest or a damaged database is refused
// without staging anything.
func TestRestoreRejectsInvalidArchives(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	st.seed(t)
	good := craftArchive(t, st, nil, backupManifest{Format: 1, Version: "0.9.0"})
	goodEntries := readArchiveBytes(t, good)
	meta := func(m backupManifest) entry {
		raw, _ := json.Marshal(m)
		return entry{name: "manifest.json", typeflag: tar.TypeReg, data: raw}
	}
	db := goodEntries[1]
	key := entry{name: "secret.key", typeflag: tar.TypeReg, data: bytes.Repeat([]byte{7}, 32)}
	for name, archive := range map[string][]byte{
		"no gzip":         []byte("not a backup"),
		"raw database":    db.data,
		"media entry":     tarGz(t, meta(backupManifest{Format: 1}), db, entry{name: "Film.mkv", typeflag: tar.TypeReg, data: []byte("x")}),
		"path entry":      tarGz(t, meta(backupManifest{Format: 1}), entry{name: "../slouchcat.db", typeflag: tar.TypeReg, data: db.data}),
		"symlink":         tarGz(t, meta(backupManifest{Format: 1}), entry{name: "slouchcat.db", typeflag: tar.TypeSymlink}),
		"directory":       tarGz(t, meta(backupManifest{Format: 1}), entry{name: "slouchcat.db", typeflag: tar.TypeDir}),
		"duplicate":       tarGz(t, meta(backupManifest{Format: 1}), db, db),
		"no database":     tarGz(t, meta(backupManifest{Format: 1})),
		"no manifest":     tarGz(t, db),
		"format 2":        tarGz(t, meta(backupManifest{Format: 2}), db),
		"key undeclared":  tarGz(t, meta(backupManifest{Format: 1}), db, key),
		"key missing":     tarGz(t, meta(backupManifest{Format: 1, IncludesSecrets: true}), db),
		"short key":       tarGz(t, meta(backupManifest{Format: 1, IncludesSecrets: true}), db, entry{name: "secret.key", typeflag: tar.TypeReg, data: []byte("short")}),
		"damaged db":      tarGz(t, meta(backupManifest{Format: 1}), entry{name: "slouchcat.db", typeflag: tar.TypeReg, data: append([]byte("SQLite format 3\x00"), bytes.Repeat([]byte{0xff}, 4096)...)}),
		"foreign db":      tarGz(t, meta(backupManifest{Format: 1}), entry{name: "slouchcat.db", typeflag: tar.TypeReg, data: emptySQLite(t)}),
		"truncated":       good[:len(good)/2],
		"corrupt payload": corrupt(t, good),
	} {
		_, err := st.svc.RestoreBackupUpload(ctx, bytes.NewReader(archive), "")
		if !errors.Is(err, model.ErrBackupInvalid) {
			t.Errorf("%s: err = %v, want ErrBackupInvalid", name, err)
		}
	}
	assertNothingStaged(t, st)
	if _, err := st.svc.RestoreBackupUpload(ctx, bytes.NewReader(good), ""); err != nil {
		t.Fatalf("the intact archive: %v", err)
	}
}

// The list holds manual archives and automatic copies, newest first; other files are left out
// and only listed names can be opened, restored or deleted.
func TestListOpenDeleteBackups(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	st.svc.now = func() time.Time { return time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC) }
	manual, err := st.svc.CreateBackup(ctx, model.BackupRequest{})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(st.dir, "backups")
	for name, data := range map[string]string{
		"pre-migration-0.8.0-20261020T080000.000Z.db":       "db",
		"slouchcat-0.7.0-20261019T080000.000Z.db":           "db",
		"pre-restore-0.9.0-20261021T130000.000Z.sqlite":     "db",
		"pre-restore-0.9.0-20261021T130000.000Z.secret.key": "key",
		"notes.txt": "x",
		"slouchcat-backup-0.9.0-20261018T080000Z.tar.gz.age": "age",
		"slouchcat-backup-broken.tar.gz":                     "x",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	list, err := st.svc.ListBackups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, b := range list {
		got = append(got, fmt.Sprintf("%s %s %s secrets=%v encrypted=%v", b.Name, b.Kind, b.Version, b.IncludesSecrets, b.Encrypted))
	}
	want := []string{
		"pre-restore-0.9.0-20261021T130000.000Z.sqlite auto 0.9.0 secrets=true encrypted=false",
		manual.Name + " manual 0.9.0 secrets=false encrypted=false",
		"pre-migration-0.8.0-20261020T080000.000Z.db auto 0.8.0 secrets=false encrypted=false",
		"slouchcat-0.7.0-20261019T080000.000Z.db auto 0.7.0 secrets=false encrypted=false",
		"slouchcat-backup-0.9.0-20261018T080000Z.tar.gz.age manual 0.9.0 secrets=true encrypted=true",
	}
	// The broken archive has neither a time in its name nor a manifest: it sorts by its mtime (now).
	if len(got) != 6 || !slices.Equal(slices.DeleteFunc(slices.Clone(got), func(s string) bool { return strings.HasPrefix(s, "slouchcat-backup-broken") }), want) {
		t.Errorf("list:\n%s\nwant (plus the broken archive):\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	for _, name := range []string{"notes.txt", "../slouchcat.db", "pre-restore-0.9.0-20261021T130000.000Z.secret.key", "missing.tar.gz"} {
		if _, err := st.svc.OpenBackup(ctx, name); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("open %q: err = %v, want ErrNotFound", name, err)
		}
		if err := st.svc.DeleteBackup(ctx, name); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("delete %q: err = %v, want ErrNotFound", name, err)
		}
		if _, err := st.svc.RestoreBackup(ctx, name, ""); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("restore %q: err = %v, want ErrNotFound", name, err)
		}
	}
	src, err := st.svc.OpenBackup(ctx, manual.Name)
	if err != nil {
		t.Fatal(err)
	}
	if src.Size != manual.SizeBytes || src.MediaType != "application/octet-stream" {
		t.Errorf("open: %+v", src)
	}
	_ = src.File.Close()
	if err := st.svc.DeleteBackup(ctx, "pre-restore-0.9.0-20261021T130000.000Z.sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pre-restore-0.9.0-20261021T130000.000Z.secret.key")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the key file of the deleted pre-restore copy is left: %v", err)
	}
}

// An automatic copy restores as it is: a pre-migration database with the current key, a
// pre-restore database with the key kept next to it.
func TestRestoreAutomaticBackup(t *testing.T) {
	ctx := context.Background()
	st := newStack(t, "0.9.0")
	st.seed(t)
	dir := filepath.Join(st.dir, "backups")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	name := "pre-restore-0.8.0-20261020T080000.000Z.sqlite"
	if err := store.Snapshot(ctx, st.db.Write, filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, store.BackupKeyFile(name)), bytes.Repeat([]byte{9}, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := st.svc.RestoreBackup(ctx, name, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != "0.8.0" || !res.IncludesSecrets {
		t.Errorf("result %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Errorf("the automatic copy must stay in the list: %v", err)
	}
	st.reopen(t, "0.9.0")
	key, err := os.ReadFile(filepath.Join(st.dir, secrets.KeyFile))
	if err != nil || !bytes.Equal(key, bytes.Repeat([]byte{9}, 32)) {
		t.Errorf("key after the restore = %x, %v; want the kept key", key, err)
	}
	if count(t, st.db.Read, "SELECT count(*) FROM accounts") != 2 {
		t.Error("restored database lacks the accounts")
	}
}

func assertNothingStaged(t *testing.T, st *stack) {
	t.Helper()
	for _, name := range []string{store.RestoreDBFile, store.RestoreKeyFile, store.RestoreDir} {
		if _, err := os.Stat(filepath.Join(st.dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s exists after a refused restore (%v)", name, err)
		}
	}
}

// craftArchive builds an unencrypted archive of the stack's database, changed by alter.
func craftArchive(t *testing.T, st *stack, alter func(*sql.DB), m backupManifest) []byte {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "slouchcat.db")
	if err := store.Snapshot(ctx, st.db.Write, dbPath); err != nil {
		t.Fatal(err)
	}
	if alter != nil {
		db, err := store.OpenFile(ctx, dbPath)
		if err != nil {
			t.Fatal(err)
		}
		alter(db)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "archive")
	if err := writeArchive(path, m, dbPath, nil, "", testLogN); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readArchiveBytes(t *testing.T, data []byte) []entry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return readArchive(t, path, "")
}

func tarGz(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Typeflag: e.typeflag, Mode: 0o600, Size: int64(len(e.data))}
		switch e.typeflag {
		case tar.TypeSymlink:
			h.Linkname, h.Size = "/etc/passwd", 0
		case tar.TypeDir:
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			if _, err := tw.Write(e.data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := errors.Join(tw.Close(), gz.Close()); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// emptySQLite is an intact SQLite database without Slouchcat's schema.
func emptySQLite(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "other.db")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	db, err := store.OpenFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	exec(t, db, "CREATE TABLE notes (id INTEGER PRIMARY KEY)")
	_ = db.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// corrupt damages the compressed payload of an archive: it flips a byte, starting in the middle,
// and walks on until the damage really shows.
//
// Not every flipped byte is damage. About 2.6 % of the positions of a gzip stream are slack bits of
// the deflate encoding; a flip there decompresses to exactly the same bytes, and the gzip checksum,
// which covers the *uncompressed* data, rightly still matches. Such an archive is intact, and a
// restore must accept it. Flipping the middle byte blindly therefore asked for an error that must
// not come whenever the middle happened to land on a slack bit -- and the middle moves, because the
// keyring of a fresh stack is random and its encrypted setting changes the length of the archive.
// See docs/dev/CURRENT_STATUS.md, "Flackernder Backup-Test".
func corrupt(t *testing.T, archive []byte) []byte {
	t.Helper()
	for pos := len(archive) / 2; pos < len(archive); pos++ {
		out := slices.Clone(archive)
		out[pos] ^= 0xff
		if _, err := io.ReadAll(mustGzipReader(out)); err != nil {
			return out
		}
	}
	t.Fatal("no byte in the second half of the archive damages the payload")
	return nil
}

// mustGzipReader returns a reader over data; a header that no longer parses is damage in itself,
// so it is reported as a reader that fails on the first read.
func mustGzipReader(data []byte) io.Reader {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return errReader{err}
	}
	return zr
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

func copyTo(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
