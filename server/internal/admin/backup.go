package admin

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"filippo.io/age"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/version"
)

// Backup and restore (A-4; plan 3.8 point 3; design 3.4). A manual backup is a tar.gz archive in
// <config>/backups with exactly three entries: manifest.json, slouchcat.db (a consistent copy) and,
// with secrets, secret.key – never a media file. Without secrets the copy loses the encrypted
// settings with the VAPID public key, the push subscriptions (bound to the VAPID pair), the
// Telegram link codes, the sessions and the PIN attempts. With secrets the whole archive is
// encrypted with a passphrase (age, scrypt). A restore unpacks and checks an archive in
// <config>/restore, stages it next to the live database (store.StageRestore) and asks main to
// restart; store.Open swaps it in before the migrations.

const (
	manualBackupPrefix = "slouchcat-backup-"
	archiveSuffix      = ".tar.gz"
	encryptedSuffix    = archiveSuffix + ".age"
	manualTimeLayout   = "20060102T150405Z"
	tempDirName        = "tmp"

	manifestEntry = "manifest.json"
	databaseEntry = store.FileName
	keyEntry      = secrets.KeyFile

	backupFormat           = 1
	minPassphraseLen       = 12
	keyFileSize            = 32
	maxBackupManifestBytes = 1 << 20

	// restartDelay lets the 202 of a restore reach the client before main restarts the server;
	// restartAnnounced is the restart time the client is told (it polls the health check from then).
	restartDelay     = time.Second
	restartAnnounced = 2 * time.Second

	defaultScryptLogN = 18 // age's default work factor: about one second per encryption
	// maxScryptLogN caps the work factor an uploaded archive may ask for: scrypt needs 128·8·2^logN
	// bytes, 1 GiB at 20 (age accepts 22, 4 GiB, which a crafted file could use to exhaust memory).
	maxScryptLogN = 20
)

// ageMagic starts every age file.
var ageMagic = []byte("age-encryption.org/")

// unsafeNameChars are replaced in the version part of a backup name (the contract allows
// [A-Za-z0-9._-]).
var unsafeNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// backupState is the backup configuration of the service.
type backupState struct {
	mu         sync.Mutex // one backup or restore at a time
	configDir  string     // empty until WithBackups
	version    string
	restart    func() // nil until WithRestart
	scryptLogN int
}

// backupManifest is manifest.json of an archive. IncludesFeedCredentials is always true: the feed
// and enclosure URLs of private podcasts keep their tokens also in a backup without secrets, so a
// restore keeps the subscriptions (review M-6); the manifest says so.
type backupManifest struct {
	Format                  int              `json:"format"`
	Version                 string           `json:"version"`
	CreatedAt               time.Time        `json:"createdAt"`
	IncludesSecrets         bool             `json:"includesSecrets"`
	IncludesFeedCredentials bool             `json:"includesFeedCredentials"`
	Tables                  map[string]int64 `json:"tables"` // rows per table of the copy
}

// WithBackups sets the config directory (database, key file and backups/) and the server version,
// and returns the receiver. Without it the backup operations answer ErrNotConfigured.
func (s *Service) WithBackups(configDir, serverVersion string) *Service {
	s.backups.configDir, s.backups.version = configDir, serverVersion
	if s.backups.scryptLogN == 0 {
		s.backups.scryptLogN = defaultScryptLogN
	}
	return s
}

// WithRestart sets the function that restarts the server after a staged restore (main: graceful
// shutdown, then the same binary again), and returns the receiver. Without it a staged restore is
// applied at the next start.
func (s *Service) WithRestart(restart func()) *Service {
	s.backups.restart = restart
	return s
}

// ListBackups lists the manual archives and the automatic copies of the backup directory, newest
// first.
func (s *Service) ListBackups(context.Context) ([]model.Backup, error) {
	if s.backups.configDir == "" {
		return nil, ErrNotConfigured
	}
	dir := s.backupDir()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []model.Backup{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list backups: %w", err)
	}
	out := []model.Backup{}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // removed meanwhile
		}
		if b, ok := describeBackup(dir, e.Name(), info); ok {
			out = append(out, b)
		}
	}
	slices.SortFunc(out, func(a, b model.Backup) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

// describeBackup reads a file name of the backup directory; ok is false for other files.
func describeBackup(dir, name string, info fs.FileInfo) (model.Backup, bool) {
	b := model.Backup{Name: name, SizeBytes: info.Size(), CreatedAt: info.ModTime().UTC()}
	if v, at, ok := store.ParseAutoBackupName(name); ok {
		b.Kind, b.Version, b.CreatedAt = model.BackupAuto, v, at
		b.IncludesSecrets = fileExists(filepath.Join(dir, store.BackupKeyFile(name)))
		return b, true
	}
	encrypted := strings.HasSuffix(name, encryptedSuffix)
	if !strings.HasPrefix(name, manualBackupPrefix) || !encrypted && !strings.HasSuffix(name, archiveSuffix) {
		return model.Backup{}, false
	}
	b.Kind, b.Encrypted, b.IncludesSecrets = model.BackupManual, encrypted, encrypted
	if v, at, ok := parseManualName(name); ok {
		b.Version, b.CreatedAt = v, at
	}
	if !encrypted {
		if m, err := readArchiveManifest(filepath.Join(dir, name)); err == nil {
			b.Version, b.IncludesSecrets = m.Version, m.IncludesSecrets
		}
	}
	return b, true
}

// parseManualName reads the version and the time of slouchcat-backup-<version>-<UTC>.tar.gz[.age].
func parseManualName(name string) (string, time.Time, bool) {
	base := strings.TrimSuffix(strings.TrimSuffix(name, ".age"), archiveSuffix)
	base = strings.TrimPrefix(base, manualBackupPrefix)
	if len(base) < len(manualTimeLayout)+2 || base[len(base)-len(manualTimeLayout)-1] != '-' {
		return "", time.Time{}, false
	}
	at, err := time.Parse(manualTimeLayout, base[len(base)-len(manualTimeLayout):])
	if err != nil {
		return "", time.Time{}, false
	}
	return base[:len(base)-len(manualTimeLayout)-1], at, true
}

// readArchiveManifest reads the manifest of an unencrypted archive: its first entry.
func readArchiveManifest(path string) (m backupManifest, err error) {
	f, err := os.Open(path) //nolint:gosec // G304: a directory entry of the backup directory
	if err != nil {
		return backupManifest{}, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	gz, err := gzip.NewReader(bufio.NewReader(f))
	if err != nil {
		return backupManifest{}, err
	}
	tr := tar.NewReader(gz)
	h, err := tr.Next()
	if err != nil {
		return backupManifest{}, err
	}
	if h.Name != manifestEntry {
		return backupManifest{}, errors.New("first entry is not the manifest")
	}
	return m, json.NewDecoder(io.LimitReader(tr, maxBackupManifestBytes)).Decode(&m)
}

// CreateBackup writes a backup archive. The copy is taken without holding up writers
// (store.Snapshot), stripped of secrets unless requested, checked with PRAGMA integrity_check and
// archived; temporary files live in <config>/tmp and are removed on every path.
func (s *Service) CreateBackup(ctx context.Context, req model.BackupRequest) (model.Backup, error) {
	if req.IncludeSecrets {
		switch {
		case req.Passphrase == "":
			return model.Backup{}, model.ErrPassphraseRequired
		case utf8.RuneCountInString(req.Passphrase) < minPassphraseLen:
			return model.Backup{}, model.ErrPassphraseTooShort
		}
	}
	if s.backups.configDir == "" {
		return model.Backup{}, ErrNotConfigured
	}
	s.backups.mu.Lock()
	defer s.backups.mu.Unlock()
	start := time.Now()
	tmp, err := s.tempDir("backup-")
	if err != nil {
		return model.Backup{}, err
	}
	defer s.removeAll(ctx, tmp)

	copyPath := filepath.Join(tmp, databaseEntry)
	if err := store.Snapshot(ctx, s.db, copyPath); err != nil { // s.db only names the file; the copy has a connection of its own
		return model.Backup{}, err
	}
	tables, err := prepareCopy(ctx, copyPath, req.IncludeSecrets)
	if err != nil {
		return model.Backup{}, err
	}
	var key []byte
	passphrase := ""
	if req.IncludeSecrets {
		key, err = os.ReadFile(filepath.Join(s.backups.configDir, secrets.KeyFile))
		if err != nil {
			return model.Backup{}, fmt.Errorf("read key file: %w", err)
		}
		passphrase = req.Passphrase
	}
	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return model.Backup{}, fmt.Errorf("create backup directory: %w", err)
	}
	name, at, err := newBackupName(dir, s.backups.version, s.now().UTC().Truncate(time.Second), req.IncludeSecrets)
	if err != nil {
		return model.Backup{}, err
	}
	m := backupManifest{Format: backupFormat, Version: s.backups.version, CreatedAt: at, IncludesSecrets: req.IncludeSecrets,
		IncludesFeedCredentials: true, Tables: tables}
	archive := filepath.Join(tmp, "archive")
	if err := writeArchive(archive, m, copyPath, key, passphrase, s.backups.scryptLogN); err != nil {
		return model.Backup{}, err
	}
	path := filepath.Join(dir, name)
	if err := os.Rename(archive, path); err != nil {
		return model.Backup{}, fmt.Errorf("store backup: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return model.Backup{}, fmt.Errorf("store backup: %w", err)
	}
	s.log.InfoContext(ctx, "backup created", "name", name, "size_bytes", info.Size(),
		"includes_secrets", req.IncludeSecrets, "duration_ms", time.Since(start).Milliseconds())
	return model.Backup{Name: name, CreatedAt: at, SizeBytes: info.Size(), IncludesSecrets: req.IncludeSecrets,
		Encrypted: req.IncludeSecrets, Version: s.backups.version, Kind: model.BackupManual}, nil
}

// maxNameTries bounds the search for a free archive name (one per second after the backup's time).
const maxNameTries = 60

// newBackupName returns a free archive name at the time at or, when taken, a later second. It
// gives up after maxNameTries names and on any error but a missing file (the directory cannot be
// read), so a backup never spins while it holds the backup lock.
func newBackupName(dir, serverVersion string, at time.Time, encrypted bool) (string, time.Time, error) {
	suffix := archiveSuffix
	if encrypted {
		suffix = encryptedSuffix
	}
	v := unsafeNameChars.ReplaceAllString(serverVersion, "_")
	if v == "" {
		v = "unknown"
	}
	start := at
	for range maxNameTries {
		name := manualBackupPrefix + v + "-" + at.Format(manualTimeLayout) + suffix
		_, err := os.Lstat(filepath.Join(dir, name))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return name, at, nil
		case err != nil:
			return "", time.Time{}, fmt.Errorf("backup name: %w", err)
		}
		at = at.Add(time.Second)
	}
	return "", time.Time{}, fmt.Errorf("backup name: no free name within %d seconds of %s", maxNameTries, start.Format(time.RFC3339))
}

// prepareCopy removes the secrets from the copy unless includeSecrets, checks its integrity and
// counts the rows of every table.
func prepareCopy(ctx context.Context, path string, includeSecrets bool) (map[string]int64, error) {
	db, err := store.OpenFile(ctx, path)
	if err != nil {
		return nil, err
	}
	tables, err := stripAndCount(ctx, db, includeSecrets)
	return tables, errors.Join(err, db.Close())
}

func stripAndCount(ctx context.Context, db *sql.DB, includeSecrets bool) (map[string]int64, error) {
	conn, err := db.Conn(ctx) // secure_delete is a setting of the connection
	if err != nil {
		return nil, fmt.Errorf("open backup copy: %w", err)
	}
	defer func() { _ = conn.Close() }()
	if !includeSecrets {
		for _, stmt := range []string{
			// The deleted rows are overwritten with zeros, so no ciphertext or token hash survives
			// in a free page of the file.
			"PRAGMA secure_delete = ON",
			"DELETE FROM settings WHERE value LIKE 'v1:%' OR key = 'vapid_public_key'",
			"DELETE FROM push_subscriptions",
			"DELETE FROM telegram_links",
			// With foreign keys on, this also deletes the playback sessions (stream tokens).
			"DELETE FROM sessions",
			"DELETE FROM pin_attempts",
		} {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return nil, fmt.Errorf("strip secrets from the backup copy: %w", err)
			}
		}
	}
	var integrity string
	if err := conn.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return nil, fmt.Errorf("check backup copy: %w", err)
	}
	if integrity != "ok" {
		return nil, fmt.Errorf("backup copy fails the integrity check: %s", integrity)
	}
	names, err := tableNames(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("list tables of the backup copy: %w", err)
	}
	tables := make(map[string]int64, len(names))
	for _, n := range names {
		var count int64
		quoted := `"` + strings.ReplaceAll(n, `"`, `""`) + `"`
		if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM "+quoted).Scan(&count); err != nil {
			return nil, fmt.Errorf("count rows of %s: %w", n, err)
		}
		tables[n] = count
	}
	return tables, nil
}

func tableNames(ctx context.Context, conn *sql.Conn) ([]string, error) {
	rows, err := conn.QueryContext(ctx, "SELECT name FROM sqlite_schema WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// writeArchive writes the archive to a new file at path (mode 0600); a passphrase encrypts it.
func writeArchive(path string, m backupManifest, dbPath string, key []byte, passphrase string, logN int) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // G304: temporary file in <config>/tmp
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer func() {
		if err != nil {
			_ = f.Close()
		}
	}()
	bw := bufio.NewWriterSize(f, 1<<20)
	var sink io.Writer = bw
	var enc io.WriteCloser
	if passphrase != "" {
		r, err := age.NewScryptRecipient(passphrase)
		if err != nil {
			return fmt.Errorf("encrypt archive: %w", err)
		}
		r.SetWorkFactor(logN)
		if enc, err = age.Encrypt(bw, r); err != nil {
			return fmt.Errorf("encrypt archive: %w", err)
		}
		sink = enc
	}
	gz := gzip.NewWriter(sink)
	tw := tar.NewWriter(gz)
	meta, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	if err := addEntry(tw, manifestEntry, int64(len(meta)), m.CreatedAt, bytes.NewReader(meta)); err != nil {
		return err
	}
	if err := addFileEntry(tw, databaseEntry, dbPath, m.CreatedAt); err != nil {
		return err
	}
	if key != nil {
		if err := addEntry(tw, keyEntry, int64(len(key)), m.CreatedAt, bytes.NewReader(key)); err != nil {
			return err
		}
	}
	if err := errors.Join(tw.Close(), gz.Close()); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}
	if enc != nil {
		if err := enc.Close(); err != nil {
			return fmt.Errorf("encrypt archive: %w", err)
		}
	}
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}
	return f.Close()
}

func addFileEntry(tw *tar.Writer, name, path string, at time.Time) (err error) {
	f, err := os.Open(path) //nolint:gosec // G304: the copy in <config>/tmp
	if err != nil {
		return fmt.Errorf("archive %s: %w", name, err)
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("archive %s: %w", name, err)
	}
	return addEntry(tw, name, info.Size(), at, f)
}

func addEntry(tw *tar.Writer, name string, size int64, at time.Time, r io.Reader) error {
	h := &tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: 0o600, Size: size, ModTime: at, Format: tar.FormatPAX}
	if err := tw.WriteHeader(h); err != nil {
		return fmt.Errorf("archive %s: %w", name, err)
	}
	if _, err := io.Copy(tw, r); err != nil {
		return fmt.Errorf("archive %s: %w", name, err)
	}
	return nil
}

// OpenBackup opens a backup of the list for the download.
func (s *Service) OpenBackup(ctx context.Context, name string) (model.StreamSource, error) {
	b, path, err := s.findBackup(ctx, name)
	if err != nil {
		return model.StreamSource{}, err
	}
	f, err := os.Open(path) //nolint:gosec // G304: the name is a directory entry of the backup list
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("open backup: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		return model.StreamSource{}, errors.Join(fmt.Errorf("open backup: %w", err), f.Close())
	}
	return model.StreamSource{File: f, MediaType: "application/octet-stream", Size: info.Size(), ModTime: info.ModTime(), Name: b.Name}, nil
}

// DeleteBackup deletes a backup of the list (with the key file kept next to a pre-restore copy).
func (s *Service) DeleteBackup(ctx context.Context, name string) error {
	b, path, err := s.findBackup(ctx, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete backup: %w", err)
	}
	if b.Kind == model.BackupAuto {
		if err := os.Remove(filepath.Join(s.backupDir(), store.BackupKeyFile(name))); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("delete backup key: %w", err)
		}
	}
	s.log.InfoContext(ctx, "backup deleted", "name", name)
	return nil
}

// findBackup returns the backup of the list with the name and its path; model.ErrNotFound for any
// other name, so no path from a request reaches the file system.
func (s *Service) findBackup(ctx context.Context, name string) (model.Backup, string, error) {
	list, err := s.ListBackups(ctx)
	if err != nil {
		return model.Backup{}, "", err
	}
	for _, b := range list {
		if b.Name == name {
			return b, filepath.Join(s.backupDir(), b.Name), nil
		}
	}
	return model.Backup{}, "", model.ErrNotFound
}

// RestoreBackupUpload checks an uploaded archive, stages it and schedules the restart.
func (s *Service) RestoreBackupUpload(ctx context.Context, archive io.Reader, passphrase string) (model.RestoreResult, error) {
	if s.backups.configDir == "" {
		return model.RestoreResult{}, ErrNotConfigured
	}
	return s.restore(ctx, func(dir string) (backupManifest, error) { return unpackArchive(archive, passphrase, dir) })
}

// RestoreBackup restores a backup of the list: a manual archive like an upload, an automatic copy
// as it is (with the key file kept next to a pre-restore copy).
func (s *Service) RestoreBackup(ctx context.Context, name, passphrase string) (model.RestoreResult, error) {
	b, path, err := s.findBackup(ctx, name)
	if err != nil {
		return model.RestoreResult{}, err
	}
	f, err := os.Open(path) //nolint:gosec // G304: the name is a directory entry of the backup list
	if err != nil {
		return model.RestoreResult{}, fmt.Errorf("open backup: %w", err)
	}
	defer func() { _ = f.Close() }()
	if b.Kind == model.BackupAuto {
		key := ""
		if b.IncludesSecrets {
			key = filepath.Join(s.backupDir(), store.BackupKeyFile(name))
		}
		return s.restore(ctx, func(dir string) (backupManifest, error) { return unpackDatabase(f, key, b.Version, dir) })
	}
	return s.restore(ctx, func(dir string) (backupManifest, error) { return unpackArchive(f, passphrase, dir) })
}

// restore unpacks a backup into <config>/restore with unpack, checks it, stages it and schedules
// the restart. The restore directory is removed on every path.
func (s *Service) restore(ctx context.Context, unpack func(dir string) (backupManifest, error)) (model.RestoreResult, error) {
	s.backups.mu.Lock()
	defer s.backups.mu.Unlock()
	start := time.Now()
	dir := filepath.Join(s.backups.configDir, store.RestoreDir)
	if err := os.RemoveAll(dir); err != nil {
		return model.RestoreResult{}, fmt.Errorf("clean restore directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return model.RestoreResult{}, fmt.Errorf("create restore directory: %w", err)
	}
	defer s.removeAll(ctx, dir)

	m, err := unpack(dir)
	if err != nil {
		return model.RestoreResult{}, err
	}
	// Versions that are no semantic version (development builds) leave the decision to the schema
	// version below.
	if c, ok := version.Compare(m.Version, s.backups.version); ok && c > 0 {
		return model.RestoreResult{}, fmt.Errorf("%w: backup %s, server %s", model.ErrBackupNewerThanServer, m.Version, s.backups.version)
	}
	dbPath := filepath.Join(dir, databaseEntry)
	switch err := store.CheckRestorable(ctx, dbPath); {
	case errors.Is(err, store.ErrSchemaNewer):
		return model.RestoreResult{}, fmt.Errorf("%w: %w", model.ErrBackupNewerThanServer, err)
	case errors.Is(err, store.ErrRestoreInvalid):
		return model.RestoreResult{}, fmt.Errorf("%w: %w", model.ErrBackupInvalid, err)
	case err != nil:
		return model.RestoreResult{}, err
	}
	keyPath := ""
	if m.IncludesSecrets {
		keyPath = filepath.Join(dir, keyEntry)
	}
	if err := store.StageRestore(s.backups.configDir, dbPath, keyPath); err != nil {
		return model.RestoreResult{}, err
	}
	s.log.InfoContext(ctx, "backup staged for restore", "version", m.Version, "includes_secrets", m.IncludesSecrets,
		"duration_ms", time.Since(start).Milliseconds())
	if s.backups.restart != nil {
		time.AfterFunc(restartDelay, s.backups.restart)
	} else {
		s.log.WarnContext(ctx, "no restart configured: the staged restore is applied at the next start")
	}
	return model.RestoreResult{RestartAt: s.now().Add(restartAnnounced), Version: m.Version, IncludesSecrets: m.IncludesSecrets}, nil
}

// unpackArchive decrypts (age) and unpacks an archive into dir: only the three known entries as
// regular files, at most model.MaxBackupBytes in all; the manifest must match the entries.
func unpackArchive(src io.Reader, passphrase, dir string) (backupManifest, error) {
	br := bufio.NewReaderSize(src, 1<<20)
	var plain io.Reader = br
	if head, _ := br.Peek(len(ageMagic)); bytes.Equal(head, ageMagic) {
		if passphrase == "" {
			return backupManifest{}, model.ErrPassphraseRequired
		}
		id, err := age.NewScryptIdentity(passphrase)
		if err != nil {
			return backupManifest{}, model.ErrPassphraseRequired
		}
		id.SetMaxWorkFactor(maxScryptLogN)
		d, err := age.Decrypt(br, id)
		switch {
		case errors.Is(err, age.ErrIncorrectIdentity):
			return backupManifest{}, model.ErrPassphraseWrong
		case err != nil:
			return backupManifest{}, invalid(err)
		}
		plain = d
	}
	gz, err := gzip.NewReader(plain)
	if err != nil {
		return backupManifest{}, invalid(err)
	}
	tr := tar.NewReader(gz)
	remaining := int64(model.MaxBackupBytes)
	seen := map[string]bool{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return backupManifest{}, invalid(err)
		}
		name := "" // one of the known entries; no name from the archive reaches the file system
		for _, n := range []string{manifestEntry, databaseEntry, keyEntry} {
			if h.Name == n {
				name = n
			}
		}
		switch {
		case h.Typeflag != tar.TypeReg, name == "", seen[name]:
			return backupManifest{}, invalid(fmt.Errorf("unexpected entry %q", h.Name))
		case h.Size > remaining, name == manifestEntry && h.Size > maxBackupManifestBytes, name == keyEntry && h.Size != keyFileSize:
			return backupManifest{}, invalid(fmt.Errorf("entry %q has %d bytes", name, h.Size))
		}
		remaining -= h.Size
		seen[name] = true
		if err := writeNewFile(filepath.Join(dir, name), tr); err != nil {
			return backupManifest{}, err
		}
	}
	// The gzip checksum and the authentication of age's last chunk are checked at the end.
	switch n, err := io.Copy(io.Discard, io.LimitReader(gz, 1<<20+1)); {
	case err != nil:
		return backupManifest{}, invalid(fmt.Errorf("end of the archive: %w", err))
	case n > 1<<20:
		return backupManifest{}, invalid(errors.New("data after the end of the archive"))
	}
	if !seen[manifestEntry] || !seen[databaseEntry] {
		return backupManifest{}, invalid(errors.New("manifest or database missing"))
	}
	raw, err := os.ReadFile(filepath.Join(dir, manifestEntry)) //nolint:gosec // G304: fixed name in <config>/restore
	if err != nil {
		return backupManifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m backupManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return backupManifest{}, invalid(err)
	}
	if m.Format != backupFormat || m.IncludesSecrets != seen[keyEntry] {
		return backupManifest{}, invalid(fmt.Errorf("manifest format %d, includesSecrets %v, key file %v", m.Format, m.IncludesSecrets, seen[keyEntry]))
	}
	return m, nil
}

// unpackDatabase copies an automatic backup (a bare database) and its key file, when given, into dir.
func unpackDatabase(src io.Reader, keyPath, backupVersion, dir string) (backupManifest, error) {
	if err := writeNewFile(filepath.Join(dir, databaseEntry), io.LimitReader(src, model.MaxBackupBytes)); err != nil {
		return backupManifest{}, err
	}
	m := backupManifest{Format: backupFormat, Version: backupVersion}
	if keyPath != "" {
		key, err := os.ReadFile(keyPath) //nolint:gosec // G304: key file next to a backup of the list
		if err != nil {
			return backupManifest{}, fmt.Errorf("read backup key: %w", err)
		}
		if len(key) != keyFileSize {
			return backupManifest{}, invalid(fmt.Errorf("key file has %d bytes", len(key)))
		}
		if err := writeNewFile(filepath.Join(dir, keyEntry), bytes.NewReader(key)); err != nil {
			return backupManifest{}, err
		}
		m.IncludesSecrets = true
	}
	return m, nil
}

// writeNewFile writes r to a new file (mode 0600). A failing read means a damaged archive
// (model.ErrBackupInvalid); a failing write is returned as it is.
func writeNewFile(path string, r io.Reader) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // G304: fixed names in <config>/restore
	if err != nil {
		return fmt.Errorf("unpack %s: %w", filepath.Base(path), err)
	}
	_, err = io.Copy(f, r)
	var pathErr *fs.PathError
	switch {
	case errors.As(err, &pathErr):
		return errors.Join(fmt.Errorf("unpack %s: %w", filepath.Base(path), err), f.Close())
	case err != nil:
		return errors.Join(invalid(err), f.Close())
	}
	return f.Close()
}

func invalid(err error) error { return fmt.Errorf("%w: %w", model.ErrBackupInvalid, err) }

func (s *Service) backupDir() string { return filepath.Join(s.backups.configDir, store.BackupDir) }

// tempDir creates a fresh directory below <config>/tmp (same file system as the backups, so the
// finished archive is renamed into place).
func (s *Service) tempDir(prefix string) (string, error) {
	base := filepath.Join(s.backups.configDir, tempDirName)
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", fmt.Errorf("create temporary directory: %w", err)
	}
	dir, err := os.MkdirTemp(base, prefix)
	if err != nil {
		return "", fmt.Errorf("create temporary directory: %w", err)
	}
	return dir, nil
}

func (s *Service) removeAll(ctx context.Context, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		s.log.WarnContext(ctx, "remove temporary directory", "dir", dir, "err", err)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
