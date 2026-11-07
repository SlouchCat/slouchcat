package updater

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Recorder writes the server's ServerRecord of this start to server.json in the shared folder
// (ADR 0024, point 12): Before right before the migrations, After once they are through. Its two
// methods are the schema hooks of store.Open (store.SchemaHooks). The record is the updater's
// only way to tell whether a new server migrated the database when that server never becomes
// healthy.
//
// Without a companion the folder is missing: the recorder logs that once at debug level and
// writes nothing. A write that fails is logged once as a warning. Neither stops the server.
type Recorder struct {
	path    string
	version string
	log     *slog.Logger
	now     func() time.Time

	mu     sync.Mutex
	rec    ServerRecord
	silent bool // a failure was logged; later ones are not
}

// NewRecorder returns the recorder for the folder dir (normally ServerDir) and the running
// version.
func NewRecorder(dir, version string, log *slog.Logger) *Recorder {
	return &Recorder{path: filepath.Join(dir, ServerFile), version: version, log: log, now: time.Now}
}

// Before records the start: version, schema version found and the pre-migration backup ("" when
// none was taken).
func (r *Recorder) Before(schema int64, backup string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rec = ServerRecord{Version: r.version, SchemaBefore: schema, Backup: backup, StartedAt: r.now().UTC()}
	r.write()
}

// After records the schema version the migrations left and when they were through.
func (r *Recorder) After(schema int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rec.SchemaAfter, r.rec.MigratedAt = schema, r.now().UTC()
	r.write()
}

func (r *Recorder) write() {
	if r.silent {
		return
	}
	if _, err := os.Stat(filepath.Dir(r.path)); errors.Is(err, fs.ErrNotExist) {
		r.silent = true
		r.log.Debug("no updater folder, server.json not written", "dir", filepath.Dir(r.path))
		return
	}
	if err := WriteJSON(r.path, r.rec, 0o644); err != nil {
		r.silent = true
		r.log.Warn("server.json for the updater not written", "err", err)
	}
}
