package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/secrets"
)

// Restore of a backup (A-4, design 3.4). Package admin checks an archive in <config>/restore/ and
// stages the database as <config>/slouchcat.db.restore, with the key file of a backup with secrets
// as <config>/secret.key.restore; then the server restarts. Open applies the staged files before
// it migrates: the current database is kept as backups/pre-restore-<version>-<UTC>.sqlite (with
// its key file next to it when the restore replaces the key), the staged files take the place of
// the live ones and the migrations bring an older backup up to date.

const (
	// RestoreDir is the scratch directory inside the config directory where an archive is unpacked
	// and checked; Open removes it.
	RestoreDir = "restore"
	// RestoreDBFile and RestoreKeyFile are the staged files next to the live ones.
	RestoreDBFile  = FileName + ".restore"
	RestoreKeyFile = secrets.KeyFile + ".restore"

	preRestorePrefix = "pre-restore-"
	preRestoreSuffix = ".sqlite"
	backupKeySuffix  = ".secret.key"
)

// Errors of CheckRestorable.
var (
	ErrRestoreInvalid = errors.New("not an intact Slouchcat database")
	ErrSchemaNewer    = errors.New("database schema is newer than this server")
)

// BackupKeyFile is the name of the key file kept next to the automatic backup name (only
// pre-restore backups have one).
func BackupKeyFile(name string) string {
	return strings.TrimSuffix(name, preRestoreSuffix) + backupKeySuffix
}

// Snapshot writes a consistent copy of the database behind conn (any pool of it; conn only names
// the file) to dst with VACUUM INTO. The copy is read through a read-only connection of its own,
// because the reader pool is query-only (which VACUUM INTO refuses) and the writer must stay free
// for the server: in WAL mode the copy neither waits for nor blocks writers. The copy is in
// rollback-journal mode.
func Snapshot(ctx context.Context, conn *sql.DB, dst string) error {
	var path string
	if err := conn.QueryRowContext(ctx, "SELECT file FROM pragma_database_list WHERE name = 'main'").Scan(&path); err != nil {
		return fmt.Errorf("snapshot: locate database: %w", err)
	}
	src, err := openFile(ctx, path, "ro")
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	if _, err := src.ExecContext(ctx, "VACUUM INTO ?", dst); err != nil {
		return errors.Join(fmt.Errorf("snapshot to %s: %w", dst, err), src.Close())
	}
	return src.Close()
}

// OpenFile opens an existing database file outside the server's pools (a backup copy or a restore
// candidate) with one connection and foreign keys on. The caller closes it.
func OpenFile(ctx context.Context, path string) (*sql.DB, error) {
	return openFile(ctx, path, "rw")
}

func openFile(ctx context.Context, path, mode string) (*sql.DB, error) {
	q := url.Values{"mode": {mode}, "_pragma": {"busy_timeout(5000)", "foreign_keys(1)"}}
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: q.Encode()}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("open database %s: %w", path, err), db.Close())
	}
	return db, nil
}

// CheckRestorable opens a candidate database and checks that it is an intact Slouchcat database
// this server can migrate: PRAGMA integrity_check and foreign_key_check pass, and its schema
// version (goose) is not newer than the newest embedded migration. Errors: ErrRestoreInvalid,
// ErrSchemaNewer.
func CheckRestorable(ctx context.Context, path string) error {
	db, err := OpenFile(ctx, path)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRestoreInvalid, err)
	}
	err = checkRestorable(ctx, db)
	return errors.Join(err, db.Close())
}

func checkRestorable(ctx context.Context, db *sql.DB) error {
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("%w: integrity check: %w", ErrRestoreInvalid, err)
	}
	if result != "ok" {
		return fmt.Errorf("%w: integrity check: %s", ErrRestoreInvalid, result)
	}
	violated, err := foreignKeysViolated(ctx, db)
	if err != nil {
		return fmt.Errorf("%w: foreign key check: %w", ErrRestoreInvalid, err)
	}
	if violated {
		return fmt.Errorf("%w: foreign keys violated", ErrRestoreInvalid)
	}
	var schema sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT max(version_id) FROM goose_db_version WHERE is_applied").Scan(&schema); err != nil {
		return fmt.Errorf("%w: schema version: %w", ErrRestoreInvalid, err)
	}
	if !schema.Valid || schema.Int64 < 1 {
		return fmt.Errorf("%w: no migration applied", ErrRestoreInvalid)
	}
	latest, err := latestMigration(migrationsFS(Options{}))
	if err != nil {
		return err
	}
	if schema.Int64 > latest {
		return fmt.Errorf("%w: schema %d, server %d", ErrSchemaNewer, schema.Int64, latest)
	}
	return nil
}

func foreignKeysViolated(ctx context.Context, db *sql.DB) (bool, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	violated := rows.Next()
	return violated, rows.Err()
}

// latestMigration is the highest version among the migration files (NNNNN_name.sql).
func latestMigration(fsys fs.FS) (int64, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return 0, fmt.Errorf("list migrations: %w", err)
	}
	var latest int64
	for _, e := range entries {
		prefix, _, ok := strings.Cut(e.Name(), "_")
		if !ok || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		if v, err := strconv.ParseInt(prefix, 10, 64); err == nil {
			latest = max(latest, v)
		}
	}
	return latest, nil
}

// StageRestore moves a checked database (and the key file of a backup with secrets; keyPath empty
// otherwise) into place for the next Open. Both must lie inside configDir (the restore directory),
// so the renames stay on one file system. The key goes first and the database last: the staged
// database marks a complete staging, a staged key alone is left over from an interrupted one.
func StageRestore(configDir, dbPath, keyPath string) error {
	stagedDB, stagedKey := filepath.Join(configDir, RestoreDBFile), filepath.Join(configDir, RestoreKeyFile)
	for _, p := range []string{stagedDB, stagedKey} {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("stage restore: %w", err)
		}
	}
	if keyPath != "" {
		if err := os.Rename(keyPath, stagedKey); err != nil {
			return fmt.Errorf("stage restore key: %w", err)
		}
	}
	if err := os.Rename(dbPath, stagedDB); err != nil {
		return fmt.Errorf("stage restore database: %w", err)
	}
	return nil
}

// applyRestore swaps in a staged restore (see the package comment of this file). It runs before the
// database is opened; every step can run again after a crash: the staged database is renamed last,
// so a start that finds it repeats the swap.
func applyRestore(ctx context.Context, opts Options) error {
	dir := opts.ConfigDir
	stagedDB, stagedKey := filepath.Join(dir, RestoreDBFile), filepath.Join(dir, RestoreKeyFile)
	if !fileExists(stagedDB) {
		if fileExists(stagedKey) {
			opts.Logger.Warn("staged key file without a staged database removed", "file", stagedKey)
			if err := os.Remove(stagedKey); err != nil {
				return fmt.Errorf("remove staged key: %w", err)
			}
		}
		return nil
	}
	start := time.Now()
	live, liveKey := filepath.Join(dir, FileName), filepath.Join(dir, secrets.KeyFile)
	backups := filepath.Join(dir, BackupDir)
	var kept string
	if fileExists(live) {
		now := time.Now
		if opts.now != nil {
			now = opts.now
		}
		if err := os.MkdirAll(backups, 0o750); err != nil {
			return fmt.Errorf("create backup directory: %w", err)
		}
		kept = filepath.Join(backups, autoBackupName(preRestorePrefix, opts.Version, now(), preRestoreSuffix))
		if err := vacuumInto(ctx, live, kept); err != nil {
			return err
		}
		// The restored key replaces the current one: keep it with the backup it decrypts.
		if fileExists(stagedKey) && fileExists(liveKey) {
			if err := copyFile(liveKey, filepath.Join(backups, BackupKeyFile(filepath.Base(kept)))); err != nil {
				return err
			}
		}
	}
	if fileExists(stagedKey) {
		if err := os.Rename(stagedKey, liveKey); err != nil {
			return fmt.Errorf("apply restored key: %w", err)
		}
	}
	for _, p := range []string{live, live + "-wal", live + "-shm"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove replaced database: %w", err)
		}
	}
	if err := os.Rename(stagedDB, live); err != nil {
		return fmt.Errorf("apply restored database: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(dir, RestoreDir)); err != nil {
		return fmt.Errorf("remove restore directory: %w", err)
	}
	opts.Logger.Info("backup restored", "kept_as", kept, "duration_ms", time.Since(start).Milliseconds())
	if kept != "" {
		return pruneBackups(backups, KeepBackups, func(n string) bool {
			return strings.HasPrefix(n, preRestorePrefix) && strings.HasSuffix(n, preRestoreSuffix)
		})
	}
	return nil
}

// vacuumInto copies the database file src (with its WAL) to dst.
func vacuumInto(ctx context.Context, src, dst string) error {
	db, err := OpenFile(ctx, src)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", dst); err != nil {
		return errors.Join(fmt.Errorf("back up database to %s: %w", dst, err), db.Close())
	}
	return db.Close()
}

// copyFile copies src to a new file dst with mode 0600.
func copyFile(src, dst string) (err error) {
	in, err := os.Open(src) //nolint:gosec // G304: the key file inside the operator-configured config directory
	if err != nil {
		return fmt.Errorf("copy %s: %w", src, err)
	}
	defer func() { err = errors.Join(err, in.Close()) }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // G304: fixed name inside the backup directory
	if err != nil {
		return fmt.Errorf("copy to %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		return errors.Join(fmt.Errorf("copy to %s: %w", dst, err), out.Close())
	}
	return out.Close()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
