package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The backup before a library is deleted (concept 20 section 9.2): pre-library-delete-<version>-
// <UTC>.db in <config>/backups, with a retention of its own, so a few deletions in a row never
// push out one of the KeepBackups backups before a migration (pruneBackups of migrate matches only
// pre-migration names). ParseAutoBackupName reads the name, so the backup page lists it and can
// restore it like every automatic backup; restoring it puts everything back to the state before
// the delete, the progress of every profile since included.
const (
	preLibraryDeletePrefix = "pre-library-delete-"
	// KeepLibraryDeleteBackups is the number of backups before a library delete that are kept.
	KeepLibraryDeleteBackups = 2
)

// BackupBeforeLibraryDelete writes the backup before a library delete with VACUUM INTO (Snapshot:
// a read-only connection of its own, so writers neither wait nor block) and prunes the older ones
// of its kind. It returns the file name.
func BackupBeforeLibraryDelete(ctx context.Context, conn *sql.DB, configDir, version string, now time.Time) (string, error) {
	dir := filepath.Join(configDir, BackupDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	name := autoBackupName(preLibraryDeletePrefix, version, now, backupSuffix)
	if err := Snapshot(ctx, conn, filepath.Join(dir, name)); err != nil {
		return "", fmt.Errorf("back up database before the library delete: %w", err)
	}
	if err := pruneBackups(dir, KeepLibraryDeleteBackups, isLibraryDeleteBackup); err != nil {
		return name, err
	}
	return name, nil
}

func isLibraryDeleteBackup(name string) bool {
	return strings.HasPrefix(name, preLibraryDeletePrefix) && strings.HasSuffix(name, backupSuffix)
}
