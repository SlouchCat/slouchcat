package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
)

// KeepBackups is the number of pre-migration backups that are kept.
const KeepBackups = 5

const (
	backupDir        = "backups"
	backupPrefix     = "slouchcat-"
	backupSuffix     = ".db"
	backupTimeLayout = "20060102T150405.000Z"
)

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._+-]`)

// migrate runs pending migrations on db. When the database file existed before and at least one
// migration is pending, it first writes a backup with VACUUM INTO and prunes old backups.
func migrate(ctx context.Context, db *sql.DB, existed bool, opts Options) error {
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationsFS(opts),
		goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	pending, err := provider.HasPending(ctx)
	if err != nil {
		return fmt.Errorf("check pending migrations: %w", err)
	}
	if !pending {
		return nil
	}
	if existed {
		now := time.Now
		if opts.now != nil {
			now = opts.now
		}
		dir := filepath.Join(opts.ConfigDir, backupDir)
		path, err := backup(ctx, db, dir, opts.Version, now())
		if err != nil {
			return err
		}
		opts.Logger.Info("database backed up before migration", "path", path)
		if err := pruneBackups(dir, KeepBackups); err != nil {
			return err
		}
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	for _, r := range results {
		opts.Logger.Info("migration applied", "version", r.Source.Version,
			"file", filepath.Base(r.Source.Path), "duration_ms", r.Duration.Milliseconds())
	}
	return checkForeignKeys(ctx, db)
}

// backup writes a consistent copy of db to dir and returns its path.
func backup(ctx context.Context, db *sql.DB, dir, version string, now time.Time) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	name := backupPrefix + unsafeFileChars.ReplaceAllString(version, "_") + "-" +
		now.UTC().Format(backupTimeLayout) + backupSuffix
	path := filepath.Join(dir, name)
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return "", fmt.Errorf("back up database to %s: %w", path, err)
	}
	return path, nil
}

// pruneBackups deletes all but the newest keep backups in dir. Age comes from the UTC timestamp
// at the end of the file name, which sorts lexically.
func pruneBackups(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}
	type file struct{ name, stamp string }
	var files []file
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, backupPrefix) || !strings.HasSuffix(n, backupSuffix) {
			continue
		}
		base := strings.TrimSuffix(n, backupSuffix)
		if len(base) < len(backupTimeLayout) {
			continue
		}
		stamp := base[len(base)-len(backupTimeLayout):]
		if _, err := time.Parse(backupTimeLayout, stamp); err != nil {
			continue
		}
		files = append(files, file{name: n, stamp: stamp})
	}
	slices.SortFunc(files, func(a, b file) int { return strings.Compare(b.stamp, a.stamp) })
	var errs []error
	for _, f := range files[min(keep, len(files)):] {
		errs = append(errs, os.Remove(filepath.Join(dir, f.name)))
	}
	return errors.Join(errs...)
}

// checkForeignKeys fails when any foreign key is violated. Migrations run with foreign keys off,
// and goose discards the rows of their PRAGMA foreign_key_check, so the server checks here.
func checkForeignKeys(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("foreign key check: %w", err)
	}
	defer rows.Close()
	var violations []string
	for rows.Next() {
		var table, parent string
		var rowid sql.NullInt64
		var fkid int64
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return fmt.Errorf("foreign key check: %w", err)
		}
		violations = append(violations, fmt.Sprintf("%s(rowid %d) -> %s", table, rowid.Int64, parent))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("foreign key check: %w", err)
	}
	if len(violations) > 0 {
		return fmt.Errorf("foreign key violations after migration: %s", strings.Join(violations, ", "))
	}
	return nil
}
