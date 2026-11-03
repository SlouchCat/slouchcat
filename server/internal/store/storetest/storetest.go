// Package storetest opens a migrated database for tests without running every migration again.
//
// store.Open on a fresh file applies all migrations. Under the race detector with modernc SQLite
// that costs about four seconds per call, and most test helpers open a fresh database per test.
// Open migrates one template per test process with store.Open, keeps its bytes in memory and
// writes them into the config directory of each test before it calls store.Open there. That
// store.Open finds no pending migration and therefore takes no backup, so the result is what a
// fresh store.Open returns: the same pragmas and connection pools, the same schema and schema
// version, foreign keys on.
//
// Tests of the migrations, the frozen databases in testdata/db, the backup before a migration or
// of store.Open itself keep calling store.Open.
package storetest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/slouchcat/slouchcat/server/internal/store"
)

var (
	templateOnce sync.Once
	templateData []byte
	templateMode fs.FileMode
	templateErr  error
)

// Open is store.Open on a copy of the migrated template database. It takes the same options;
// opts.ConfigDir must exist and must not hold a database yet.
func Open(ctx context.Context, opts store.Options) (*store.DB, error) {
	templateOnce.Do(func() { templateData, templateMode, templateErr = buildTemplate() })
	if templateErr != nil {
		return nil, fmt.Errorf("storetest: build template database: %w", templateErr)
	}
	path := filepath.Join(opts.ConfigDir, store.FileName)
	// O_EXCL: a database that is already there would be overwritten, and a test that reopens its
	// own database must call store.Open.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, templateMode) //nolint:gosec // path is the database file in the test's own config directory
	if err != nil {
		return nil, fmt.Errorf("storetest: %w", err)
	}
	_, err = f.Write(templateData)
	if err := errors.Join(err, f.Close()); err != nil {
		return nil, fmt.Errorf("storetest: write %s: %w", path, err)
	}
	return store.Open(ctx, opts)
}

// buildTemplate migrates a database in a temporary directory and returns the content and mode of
// its file. A TRUNCATE checkpoint moves everything from the WAL into the main file first, so the
// main file alone is the whole database.
func buildTemplate() ([]byte, fs.FileMode, error) {
	dir, err := os.MkdirTemp("", "slouchcat-storetest-")
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	ctx := context.Background()
	db, err := store.Open(ctx, store.Options{ConfigDir: dir, Version: "template", Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		return nil, 0, err
	}
	var busy, logFrames, checkpointed int
	err = db.Write.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed)
	if err := errors.Join(err, db.Close()); err != nil {
		return nil, 0, err
	}
	if busy != 0 {
		return nil, 0, errors.New("WAL checkpoint of the template did not complete")
	}
	path := filepath.Join(dir, store.FileName)
	if info, err := os.Stat(path + "-wal"); err == nil && info.Size() > 0 {
		return nil, 0, fmt.Errorf("template WAL still holds %d bytes after the checkpoint", info.Size())
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is inside a directory this function created
	if err != nil {
		return nil, 0, err
	}
	return data, info.Mode().Perm(), nil
}
