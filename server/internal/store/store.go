// Package store opens the SQLite database of Slouchcat and keeps its schema current.
//
// The database is a single file in the config directory, opened with modernc.org/sqlite (no CGO)
// in WAL mode. Write holds exactly one connection, so writers never contend for the SQLite write
// lock inside the process; Read is a pool of query-only connections. Generated sqlc queries live in
// the subpackages systemq (unfiltered, restricted import) and scopedq (joined with visible_items).
//
// Open refuses a config directory on a network share, because SQLite WAL relies on shared memory
// and POSIX locks that NFS and SMB do not provide reliably.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/migrations"
)

// FileName is the database file inside the config directory.
const FileName = "slouchcat.db"

// ErrNetworkShare is returned when the config directory lives on NFS or SMB/CIFS.
var ErrNetworkShare = errors.New("/config must not live on a network share: SQLite WAL on NFS/SMB corrupts the database")

// DB bundles the writer connection and the reader pool of one database file.
type DB struct {
	Write *sql.DB
	Read  *sql.DB
}

// Options configure Open.
type Options struct {
	ConfigDir string       // Existing directory that holds the database and backups/.
	Version   string       // Server version, part of backup file names.
	Logger    *slog.Logger // Redacting logger.

	migrations fs.FS            // Overrides the embedded migrations in tests.
	now        func() time.Time // Overrides the clock in tests.
}

// Open checks the file system of the config directory, opens the database, backs it up when
// migrations are pending and runs them.
func Open(ctx context.Context, opts Options) (*DB, error) {
	fsType, err := paths.DetectFS(opts.ConfigDir)
	if err != nil {
		return nil, err
	}
	if fsType.IsNetwork() {
		return nil, fmt.Errorf("%w (detected %s at %s)", ErrNetworkShare, fsType, opts.ConfigDir)
	}
	opts.Logger.Info("config directory checked", "dir", opts.ConfigDir, "fs_type", string(fsType))

	path := filepath.Join(opts.ConfigDir, FileName)
	_, statErr := os.Stat(path)
	existed := statErr == nil

	write, err := openPool(ctx, path, url.Values{"_txlock": {"immediate"}}, 1)
	if err != nil {
		return nil, err
	}
	if err := migrate(ctx, write, existed, opts); err != nil {
		return nil, errors.Join(err, write.Close())
	}
	read, err := openPool(ctx, path, url.Values{"_pragma": {"query_only(1)"}}, max(4, runtime.NumCPU()))
	if err != nil {
		return nil, errors.Join(err, write.Close())
	}
	return &DB{Write: write, Read: read}, nil
}

// Close closes the reader pool and the writer.
func (db *DB) Close() error {
	return errors.Join(db.Read.Close(), db.Write.Close())
}

func openPool(ctx context.Context, path string, extra url.Values, maxConns int) (*sql.DB, error) {
	q := url.Values{"_pragma": {"busy_timeout(5000)", "journal_mode(WAL)", "foreign_keys(1)", "synchronous(NORMAL)"}}
	for k, vs := range extra {
		q[k] = append(q[k], vs...)
	}
	// SQLite parses the DSN as a URI, so the path is percent-encoded (spaces, '#', '?', '%').
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: q.Encode()}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns)
	db.SetConnMaxLifetime(0)
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("open database %s: %w", path, err), db.Close())
	}
	return db, nil
}

func migrationsFS(opts Options) fs.FS {
	if opts.migrations != nil {
		return opts.migrations
	}
	return migrations.FS
}
