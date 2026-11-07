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
// A record the same version left at an earlier start (a restart, or a crash loop under restart:
// unless-stopped) keeps its schemaBefore and its first backup, so the updater still sees that this
// version migrated the database and names the backup taken before the first migration. It counts
// only while it fits the database found now: its migrations never finished, or the schema found
// is the one they left. Otherwise the database was changed in between (a manual downgrade with a
// restored backup, then the same version again), and the start records itself afresh.
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
	prev   *ServerRecord // the record an earlier start of this version left; nil = none
	loaded bool          // prev was read
	silent bool          // a failure was logged; later ones are not
}

// NewRecorder returns the recorder for the folder dir (normally ServerDir) and the running
// version.
func NewRecorder(dir, version string, log *slog.Logger) *Recorder {
	return &Recorder{path: filepath.Join(dir, ServerFile), version: version, log: log, now: time.Now}
}

// KeptBackup is the pre-migration backup an earlier start of this version recorded ("" when
// none): the store never prunes it (store.SchemaHooks.KeepBackup), because the updater names it
// as the way back. It is asked before the schema is known, so a record that Before then drops
// still spares its backup from the pruning of this one start; the next start no longer names it.
func (r *Recorder) KeptBackup() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p := r.previous(); p != nil {
		return p.Backup
	}
	return ""
}

// Before records the start: version, schema version found and the pre-migration backup ("" when
// none was taken). After an earlier start of the same version it keeps that start's schema
// version and its backup when it named one, as long as that record fits the schema found (see
// Recorder); a record that does not fit is dropped, and KeptBackup no longer names its backup.
// The fields of After are cleared until the migrations of this start are through.
func (r *Recorder) Before(schema int64, backup string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := ServerRecord{Version: r.version, SchemaBefore: schema, Backup: backup, StartedAt: r.now().UTC()}
	if p := r.previous(); p != nil && !p.MigratedAt.IsZero() && p.SchemaAfter != schema {
		r.prev = nil // the database changed since that start: its record and backup count no more
	}
	if p := r.previous(); p != nil {
		rec.SchemaBefore = p.SchemaBefore
		if p.Backup != "" {
			rec.Backup = p.Backup
		}
	}
	r.rec = rec
	r.write()
}

// previous reads server.json once and returns it when an earlier start of this version wrote it.
// A missing or unreadable file counts as none. The caller holds mu.
func (r *Recorder) previous() *ServerRecord {
	if !r.loaded {
		r.loaded = true
		var p ServerRecord
		if err := ReadJSON(r.path, &p); err == nil && p.Version == r.version {
			r.prev = &p
		}
	}
	return r.prev
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
