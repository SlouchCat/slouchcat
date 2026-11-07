// Package updater is the protocol between the server and the updater companion
// server/cmd/updater (concept 15 section 9.3, ADR 0024): addresses, file names, the install state
// and the server's start record, and the version rule of an install. It starts no process and
// knows nothing of Docker; both programs import it, so the two sides cannot drift apart.
//
// The two share the folder deploy/updater/ on the host. The updater owns token and state.json,
// the server owns server.json; each file has exactly one writer, which replaces it atomically
// (WriteJSON), so a reader never sees half a file. The server mounts the folder at ServerDir and
// talks to the updater only over the internal Docker network (DefaultURL, Authorization: Bearer
// with the token). The token never appears in a log (CLAUDE.md rule 10).
package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/version"
)

const (
	// ListenAddr is where the updater listens; it publishes no port.
	ListenAddr = ":8099"
	// DefaultURL is the updater as the server reaches it on the internal Docker network.
	DefaultURL = "http://slouchcat-updater:8099"
	// PathStatus (GET) returns the State; PathInstall (POST) starts an install of the newest
	// release. Both need the token.
	PathStatus  = "/status"
	PathInstall = "/install"

	// TokenFile is the shared secret, written once by the updater (0600, owned by PUID:PGID).
	TokenFile = "token"
	// StateFile holds the State; only the updater writes it.
	StateFile = "state.json"
	// ServerFile holds the ServerRecord; only the server writes it.
	ServerFile = "server.json"
	// ServerDir is the shared folder inside the server container (read-write: server.json).
	ServerDir = "/run/slouchcat-updater"

	// MaxFileBytes caps what ReadJSON reads; the files hold a few hundred bytes.
	MaxFileBytes = 64 << 10
)

// The bounds of an install in the updater. The server derives from them how long a running
// install may take before it counts as interrupted.
const (
	// CheckTimeout bounds the checks of POST /install, which run before it answers.
	CheckTimeout = time.Minute
	// PullTimeout bounds docker compose pull.
	PullTimeout = 30 * time.Minute
	// CommandTimeout bounds every other docker call.
	CommandTimeout = 5 * time.Minute
	// WaitTimeout gives a new server time for long migrations (concept 15 9.3); the old server
	// after a rollback gets as long.
	WaitTimeout = 10 * time.Minute
	// InstallWorstCase is the longest an install runs without an interruption: the checks, the
	// pull, the digest check, compose up and the wait for the new server, then the rollback with
	// compose up and the wait for the old one.
	InstallWorstCase = CheckTimeout + PullTimeout + CommandTimeout + 2*(CommandTimeout+WaitTimeout)
)

// Phase is the step an install is in.
type Phase string

const (
	PhaseIdle       Phase = "idle"        // no install yet
	PhaseChecking   Phase = "checking"    // reading the release manifest
	PhaseBackingUp  Phase = "backing_up"  // saving .env to .env.bak-vor-<version>
	PhasePulling    Phase = "pulling"     // docker compose pull
	PhaseVerifying  Phase = "verifying"   // pulled digest against the manifest
	PhaseRestarting Phase = "restarting"  // docker compose up
	PhaseWaiting    Phase = "waiting"     // waiting for the new server to become healthy
	PhaseDone       Phase = "done"        // the new version runs
	PhaseRolledBack Phase = "rolled_back" // the new version failed, the old one runs again
	PhaseFailed     Phase = "failed"      // stopped; Code says why
)

// Running reports whether an install is under way; a second one is refused (CodeBusy).
func (p Phase) Running() bool {
	switch p {
	case PhaseChecking, PhaseBackingUp, PhasePulling, PhaseVerifying, PhaseRestarting, PhaseWaiting:
		return true
	}
	return false
}

// Final reports whether p ends an install.
func (p Phase) Final() bool {
	return p == PhaseDone || p == PhaseRolledBack || p == PhaseFailed
}

// Code is the machine reason of a refused, failed or rolled back install.
type Code string

const (
	CodeBusy                Code = "busy"                 // an install is running
	CodeNotNewer            Code = "not_newer"            // the manifest offers no newer release
	CodeManifestUnreachable Code = "manifest_unreachable" // the manifest could not be fetched
	CodeManifestInvalid     Code = "manifest_invalid"     // the manifest does not parse or its version is not a release
	CodeNoDigest            Code = "no_digest"            // the manifest has no image digest
	CodeNoComposeLabels     Code = "no_compose_labels"    // the server container was not started by Compose
	CodeOutsideDeployDir    Code = "outside_deploy_dir"   // a compose file lies outside SLOUCHCAT_DEPLOY_DIR
	CodePullFailed          Code = "pull_failed"          // docker compose pull failed
	CodeDigestMismatch      Code = "digest_mismatch"      // the pulled image is not the one of the manifest
	CodeUnhealthy           Code = "unhealthy"            // the new server did not become healthy in time
	CodeRollbackFailed      Code = "rollback_failed"      // the old version did not come back either
	CodeMigratedNoRollback  Code = "migrated_no_rollback" // a migration ran; back only with the backup
)

// State is the content of state.json and the answer of PathStatus.
type State struct {
	Phase      Phase     `json:"phase"`
	From       string    `json:"from,omitempty"`
	To         string    `json:"to,omitempty"`
	StartedAt  time.Time `json:"startedAt,omitzero"`
	FinishedAt time.Time `json:"finishedAt,omitzero"`
	// Migrated is true when the new server ran a migration (from server.json).
	Migrated bool `json:"migrated,omitempty"`
	// Backup is the name of the pre-migration backup when known.
	Backup  string `json:"backup,omitempty"`
	Code    Code   `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	// RollingBack is set in a running phase once the updater takes the install back (the env file
	// is put back and the old version started), so that a start after an interruption finishes the
	// rollback instead of waiting for the new version. Only the updater reads it; a final state
	// never carries it.
	RollingBack bool `json:"rollingBack,omitempty"`
}

// ServerRecord is the content of server.json: the server writes it at every start before and
// after the migrations, so the updater sees a migration even when the new server never becomes
// healthy. SchemaAfter and MigratedAt stay zero until the migrations are through.
type ServerRecord struct {
	Version      string    `json:"version"`
	SchemaBefore int64     `json:"schemaBefore"`
	SchemaAfter  int64     `json:"schemaAfter,omitempty"`
	Backup       string    `json:"backup,omitempty"`
	StartedAt    time.Time `json:"startedAt,omitzero"`
	MigratedAt   time.Time `json:"migratedAt,omitzero"`
}

// Migrated reports whether the start the record describes ran a migration: its schema changed,
// or it started its migrations and never finished them (MigratedAt zero). The updater reads it
// to decide whether a failed install may be taken back.
func (r ServerRecord) Migrated() bool {
	return r.MigratedAt.IsZero() || r.SchemaAfter != r.SchemaBefore
}

// ErrTooLarge is returned by ReadJSON for a file over MaxFileBytes.
var ErrTooLarge = errors.New("updater: file too large")

// ErrNotRegular is returned by ReadJSON for a path that is a symbolic link, a FIFO or anything
// else but a regular file.
var ErrNotRegular = errors.New("updater: not a regular file")

// ReadJSON decodes the JSON file at path into v, reading at most MaxFileBytes. A missing file
// returns an error that matches fs.ErrNotExist. The other side can write the shared folder, so
// the file is opened without following a symbolic link and without blocking (a FIFO would hang
// the reader), and it must be a regular file.
func ReadJSON(path string, v any) error {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0) //nolint:gosec // G304: a fixed file name inside the shared updater folder
	if errors.Is(err, syscall.ELOOP) {
		return fmt.Errorf("read %s: %w", filepath.Base(path), ErrNotRegular)
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("read %s: %w", filepath.Base(path), ErrNotRegular)
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if len(data) > MaxFileBytes {
		return fmt.Errorf("read %s: %w", filepath.Base(path), ErrTooLarge)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return nil
}

// WriteJSON replaces the file at path with v as JSON: temporary file with mode perm in the same
// directory, sync, rename. The file keeps the owner of the writing process.
func WriteJSON(path string, v any, perm fs.FileMode) error {
	return WriteJSONOwned(path, v, perm, -1, -1)
}

// WriteJSONOwned is WriteJSON with the owner uid:gid (-1 keeps the process's). Mode and owner are
// set on the open temporary file, never by path: the folder is shared with a process that could
// put a symbolic link where the path points, and the rename replaces such a link without
// touching its target.
func WriteJSONOwned(path string, v any, perm fs.FileMode, uid, gid int) (err error) {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	steps := []func() error{
		func() error { return tmp.Chmod(perm) },
		func() error {
			if uid < 0 && gid < 0 {
				return nil
			}
			return tmp.Chown(uid, gid)
		},
		func() error { _, err := tmp.Write(data); return err },
		tmp.Sync,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return errors.Join(fmt.Errorf("write %s: %w", filepath.Base(path), err), tmp.Close())
		}
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

// releasePattern is the only form a version may have before the updater writes it into .env:
// no "v", no pre-release, no build metadata, nothing that could carry a line break.
var releasePattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// ValidRelease reports whether v is a plain release version "X.Y.Z".
func ValidRelease(v string) bool {
	if !releasePattern.MatchString(v) {
		return false
	}
	_, ok := version.Parse(v) // also refuses leading zeros and numbers beyond uint64
	return ok
}

// Newer reports whether candidate is a plain release newer than the running version. running may
// be a self-built image ("1.15.0-12-gabc1234", "1.15.0-dirty"), which counts as its release
// (package version); a running version that does not parse ("dev") is never updated.
func Newer(candidate, running string) bool {
	if !ValidRelease(candidate) {
		return false
	}
	c, ok := version.Compare(candidate, running)
	return ok && c > 0
}
