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

// KeepBackups is the number of pre-migration backups (and of pre-restore backups) that are kept.
const KeepBackups = 5

// Automatic backups in <config>/backups (A-4): the copy before a migration is
// pre-migration-<version>-<UTC>.db; releases before M8 named it slouchcat-<version>-<UTC>.db, which
// is still listed and pruned. The manual archives of package admin (slouchcat-backup-…) end in
// .tar.gz and never match these names.
const (
	BackupDir          = "backups"
	preMigrationPrefix = "pre-migration-"
	legacyBackupPrefix = "slouchcat-"
	backupSuffix       = ".db"
	backupTimeLayout   = "20060102T150405.000Z"
)

// unsafeFileChars are replaced in the version part of backup names, which the contract restricts
// to [A-Za-z0-9._-].
var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

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
		dir := filepath.Join(opts.ConfigDir, BackupDir)
		path, err := backup(ctx, db, dir, opts.Version, now())
		if err != nil {
			return err
		}
		opts.Logger.Info("database backed up before migration", "path", path)
		if err := pruneBackups(dir, KeepBackups, isPreMigrationBackup); err != nil {
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
	name := autoBackupName(preMigrationPrefix, version, now, backupSuffix)
	path := filepath.Join(dir, name)
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return "", fmt.Errorf("back up database to %s: %w", path, err)
	}
	return path, nil
}

// pruneBackups deletes all but the newest keep automatic backups in dir that match (a kind of
// backup), with the key file kept next to a pre-restore backup and the WAL and index of one that
// was moved instead of copied. Age comes from the UTC timestamp at the end of the file name, which
// sorts lexically.
func pruneBackups(dir string, keep int, match func(name string) bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("list backups: %w", err)
	}
	type file struct{ name, stamp string }
	var files []file
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !match(n) {
			continue
		}
		_, at, ok := ParseAutoBackupName(n)
		if !ok {
			continue
		}
		files = append(files, file{name: n, stamp: at.Format(backupTimeLayout)})
	}
	slices.SortFunc(files, func(a, b file) int { return strings.Compare(b.stamp, a.stamp) })
	var errs []error
	for _, f := range files[min(keep, len(files)):] {
		errs = append(errs, os.Remove(filepath.Join(dir, f.name)))
		for _, extra := range append(databaseFiles(filepath.Join(dir, f.name))[1:], filepath.Join(dir, BackupKeyFile(f.name))) {
			if fileExists(extra) {
				errs = append(errs, os.Remove(extra))
			}
		}
	}
	return errors.Join(errs...)
}

// autoBackupName is prefix + version (unsafe characters replaced) + "-" + UTC time + suffix.
func autoBackupName(prefix, version string, now time.Time, suffix string) string {
	return prefix + unsafeFileChars.ReplaceAllString(version, "_") + "-" + now.UTC().Format(backupTimeLayout) + suffix
}

// ParseAutoBackupName reads the server version and the time of an automatic backup's file name
// (pre-migration, legacy pre-migration or pre-restore); ok is false for any other name.
func ParseAutoBackupName(name string) (version string, at time.Time, ok bool) {
	var base string
	switch {
	case isPreMigrationBackup(name):
		base = strings.TrimSuffix(name, backupSuffix)
		if b, found := strings.CutPrefix(base, preMigrationPrefix); found {
			base = b
		} else {
			base = strings.TrimPrefix(base, legacyBackupPrefix)
		}
	case strings.HasPrefix(name, preRestorePrefix) && strings.HasSuffix(name, preRestoreSuffix):
		base = strings.TrimSuffix(strings.TrimPrefix(name, preRestorePrefix), preRestoreSuffix)
	default:
		return "", time.Time{}, false
	}
	if len(base) < len(backupTimeLayout)+2 || base[len(base)-len(backupTimeLayout)-1] != '-' {
		return "", time.Time{}, false
	}
	at, err := time.Parse(backupTimeLayout, base[len(base)-len(backupTimeLayout):])
	if err != nil {
		return "", time.Time{}, false
	}
	return base[:len(base)-len(backupTimeLayout)-1], at, true
}

// isPreMigrationBackup matches pre-migration-*.db and the names of releases before M8,
// slouchcat-*.db (the manual archives slouchcat-backup-* end in .tar.gz or .tar.gz.age).
func isPreMigrationBackup(name string) bool {
	return strings.HasSuffix(name, backupSuffix) &&
		(strings.HasPrefix(name, preMigrationPrefix) || strings.HasPrefix(name, legacyBackupPrefix))
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
