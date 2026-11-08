package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

// versionKey is the variable an install writes (ADR 0024 point 11); updaterVersionKey the one a
// self-update writes (ADR 0024 addendum point 2). The updater writes no other.
const (
	versionKey        = "SLOUCHCAT_VERSION"
	updaterVersionKey = "SLOUCHCAT_UPDATER_VERSION"
)

// keyLine matches an assignment of key at the start of a line, with the optional indent and
// "export " that compose's dotenv parser accepts. Group 1 is everything up to and including "=".
func keyLine(key string) *regexp.Regexp {
	return regexp.MustCompile(`^([ \t]*(?:export[ \t]+)?` + regexp.QuoteMeta(key) + `[ \t]*=)`)
}

// versionLine matches a SLOUCHCAT_VERSION assignment, updaterVersionLine one of
// SLOUCHCAT_UPDATER_VERSION.
var (
	versionLine        = keyLine(versionKey)
	updaterVersionLine = keyLine(updaterVersionKey)
)

// lineOf returns the pattern of key, one of the two keys the updater writes.
func lineOf(key string) *regexp.Regexp {
	if key == updaterVersionKey {
		return updaterVersionLine
	}
	return versionLine
}

// setVersion returns data with the value of every SLOUCHCAT_VERSION line set to v (setValue).
func setVersion(data []byte, v string) []byte { return setValue(data, versionKey, v) }

// setValue returns data with the value of every line assigning key set to v and all other
// bytes unchanged: line endings, the spaces around the value, its quotes ("…" or '…') and a
// trailing comment. Without such a line it appends one, with the line ending the file already
// uses (CRLF when it has one, else LF), after closing an unterminated last line.
func setValue(data []byte, key, v string) []byte {
	re := lineOf(key)
	var out bytes.Buffer
	found := false
	rest := data
	for len(rest) > 0 {
		line := rest
		end := []byte(nil)
		if i := bytes.IndexByte(rest, '\n'); i >= 0 {
			line, rest = rest[:i], rest[i+1:]
			end = []byte("\n")
			if bytes.HasSuffix(line, []byte("\r")) {
				line, end = line[:len(line)-1], []byte("\r\n")
			}
		} else {
			rest = nil
		}
		if m := re.FindSubmatch(line); m != nil {
			found = true
			value := line[len(m[1]):]
			start, stop := valueSpan(value)
			line = slices.Concat(m[1], value[:start], []byte(v), value[stop:])
		}
		out.Write(line)
		out.Write(end)
	}
	if found {
		return out.Bytes()
	}
	eol := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		eol = "\r\n"
	}
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		out.WriteString(eol)
	}
	out.WriteString(key + "=" + v + eol)
	return out.Bytes()
}

// valueSpan returns where the value lies in s, the part of a line after "=", as compose's dotenv
// parser reads it: leading spaces do not count; a value in double quotes (with \" inside) or
// single quotes is what lies between them; an unquoted value ends before " #" (a space, then the
// comment) and before trailing spaces. An unterminated quote (a value over several lines) spans
// the rest of the line, quote included.
func valueSpan(s []byte) (start, stop int) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	if i < len(s) && (s[i] == '"' || s[i] == '\'') {
		q := s[i]
		for j := i + 1; j < len(s); j++ {
			if q == '"' && s[j] == '\\' {
				j++
				continue
			}
			if s[j] == q {
				return i + 1, j
			}
		}
		return i, len(s)
	}
	stop = len(s)
	for j := i + 1; j < len(s); j++ {
		if s[j] == '#' && s[j-1] == ' ' {
			stop = j
			break
		}
	}
	for stop > i && (s[stop-1] == ' ' || s[stop-1] == '\t') {
		stop--
	}
	return i, stop
}

// envVersion returns the value of the last SLOUCHCAT_VERSION line in data.
func envVersion(data []byte) (string, bool) {
	v, found := "", false
	for line := range bytes.Lines(data) {
		line = bytes.TrimRight(line, "\r\n")
		if m := versionLine.FindSubmatch(line); m != nil {
			value := line[len(m[1]):]
			start, stop := valueSpan(value)
			v, found = string(value[start:stop]), true
		}
	}
	return v, found
}

// hasKey reports whether data assigns key.
func hasKey(data []byte, key string) bool {
	re := lineOf(key)
	for line := range bytes.Lines(data) {
		if re.Match(line) {
			return true
		}
	}
	return false
}

// envTarget picks the env file whose SLOUCHCAT_VERSION line is edited (envTargetFor).
func envTarget(files []string) (string, error) { return envTargetFor(files, versionKey) }

// envTargetFor picks the env file whose line of key is edited: the last listed file that
// contains it (compose lets a later --env-file win), otherwise the last listed file, where the
// line is appended. A file that does not exist counts as empty. A symbolic link is read through
// its target (checked by inside), and the file must be a regular one (readRegular).
func envTargetFor(files []string, key string) (string, error) {
	for i := len(files) - 1; i >= 0; i-- {
		path := files[i]
		if target, err := filepath.EvalSymlinks(path); err == nil {
			path = target
		}
		data, _, err := readRegular(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("read env file: %w", err)
		}
		if hasKey(data, key) {
			return files[i], nil
		}
	}
	return files[len(files)-1], nil
}

// envFile is the env file as it was before the install, so that it can be put back.
type envFile struct {
	path     string
	exists   bool
	data     []byte
	mode     fs.FileMode
	uid, gid int
}

// readEnvFile reads path; a missing file is remembered as missing and gets uid:gid when written.
// A symbolic link is followed: its target (checked to lie inside the deploy folder, see inside)
// is what the install backs up and replaces, in the target's own folder, and the link stays. The
// resolved file itself is read with readRegular.
func readEnvFile(path string, uid, gid int) (envFile, error) {
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	}
	f := envFile{path: path, mode: 0o600, uid: uid, gid: gid}
	data, info, err := readRegular(path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	f.exists, f.data, f.mode = true, data, info.Mode().Perm()
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		f.uid, f.gid = int(st.Uid), int(st.Gid)
	}
	return f, nil
}

// errNotRegular is returned by readRegular for a symbolic link, a FIFO or anything else but a
// regular file.
var errNotRegular = errors.New("not a regular file")

// readRegular reads the regular file at path without following a symbolic link at its last
// element and without blocking (a FIFO would hang the install); it returns the content and the
// file's information from the open descriptor. The updater runs as root: an env file or a backup
// that is not a regular file is refused, not read through.
func readRegular(path string) ([]byte, fs.FileInfo, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0) //nolint:gosec // G304: env file or its backup from the compose labels, checked to lie inside the deploy folder
	if errors.Is(err, syscall.ELOOP) {
		return nil, nil, fmt.Errorf("read %s: %w", path, errNotRegular)
	}
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("read %s: %w", path, errNotRegular)
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}
	return data, info, nil
}

// maxBackupTries bounds the search for a free backup name.
const maxBackupTries = 1000

// backupName is the name of the n-th backup taken before installing v: <name>.bak-vor-<v> for the
// first, <name>.bak-vor-<v>-2, -3, … for the next.
func (f envFile) backupName(v string, n int) string {
	name := f.path + ".bak-vor-" + v
	if n > 1 {
		name += "-" + strconv.Itoa(n)
	}
	return name
}

// backup copies the file to <name>.bak-vor-<v> (0600, owned like the original: it holds secrets)
// and returns the base name it wrote. It never deletes or replaces another backup: hand-made ones
// share the name pattern, and so does an earlier install of the same version. When the name is
// taken it uses <name>.bak-vor-<v>-2, -3, … (the first free one). The content goes to a temporary
// file first and is linked to its name (link fails when the name exists, even as a dangling
// symbolic link), so a backup never shows half written. A missing file has nothing to back up
// and returns "".
func (f envFile) backup(v string) (string, error) {
	if !f.exists {
		return "", nil
	}
	tmp, err := writeTemp(f.path+".bak-vor-"+v, f.data, 0o600, f.uid, f.gid)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp) }()
	for n := 1; n <= maxBackupTries; n++ {
		name := f.backupName(v, n)
		err := os.Link(tmp, name)
		if err == nil {
			return filepath.Base(name), nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("no free backup name %s.bak-vor-%s-N", filepath.Base(f.path), v)
}

// saveAs writes the content to the new file name (0600, owned like the original) through a
// temporary file linked to it, as backup does: it fails when name exists, even as a dangling
// symbolic link, and a backup never shows half written.
func (f envFile) saveAs(name string) error {
	tmp, err := writeTemp(name, f.data, 0o600, f.uid, f.gid)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()
	return os.Link(tmp, name)
}

// findBackup returns the path of the backup taken before installing v, next to the env file.
// persisted is the base name the install stored in state.json ("" when it stored none). A name
// that is not a base name of this file's backups of v is refused (errBackupName): the shared
// folder holding state.json is writable by the server. Without a stored name it is the highest
// numbered match (the newest one the updater wrote), the plain name counting as the first.
func (f envFile) findBackup(v, persisted string) (string, error) {
	re, err := regexp.Compile(`^` + regexp.QuoteMeta(filepath.Base(f.path)+".bak-vor-"+v) + `(?:-([0-9]+))?$`)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(f.path)
	if persisted != "" {
		if filepath.Base(persisted) != persisted || !re.MatchString(persisted) {
			return "", fmt.Errorf("%w: %q", errBackupName, truncate(persisted, 80))
		}
		return filepath.Join(dir, persisted), nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	best, bestN := "", 0
	for _, e := range entries {
		m := re.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n := 1
		if m[1] != "" {
			if n, err = strconv.Atoi(m[1]); err != nil {
				continue
			}
		}
		if n > bestN {
			best, bestN = e.Name(), n
		}
	}
	if best == "" {
		return f.backupName(v, 1), nil // not there: the caller's read reports it
	}
	return filepath.Join(dir, best), nil
}

// errBackupName is returned by findBackup for a stored backup name that does not fit.
var errBackupName = errors.New("backup name does not fit the env file and version")

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// write replaces the file with data, keeping its mode and owner.
func (f envFile) write(data []byte) error {
	return writeAtomic(f.path, data, f.mode, f.uid, f.gid)
}

// restore puts the file back as it was before the install.
func (f envFile) restore() error {
	if !f.exists {
		err := os.Remove(f.path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return f.write(f.data)
}

// writeAtomic replaces path with data: temporary file in the same folder with mode and owner,
// sync, rename. A reader (compose) never sees half a file.
func writeAtomic(path string, data []byte, mode fs.FileMode, uid, gid int) error {
	tmp, err := writeTemp(path, data, mode, uid, gid)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// writeTemp writes data to a new temporary file next to path, with mode and owner, synced, and
// returns its name. The caller renames or links it, or removes it.
func writeTemp(path string, data []byte, mode fs.FileMode, uid, gid int) (name string, err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	steps := []func() error{
		func() error { return tmp.Chmod(mode) },
		func() error { return tmp.Chown(uid, gid) },
		func() error { _, err := tmp.Write(data); return err },
		tmp.Sync,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return "", errors.Join(err, tmp.Close())
		}
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

// inside reports whether path lies inside root (both cleaned, symbolic links resolved) and not in
// the shared folder <root>/updater (sharedDir) or at it: the server can write there, so no compose
// file, env file or working folder of the updater may lie there. For a path that does not exist
// (an env file the install will create) the folder is resolved instead.
func inside(root, path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if errors.Is(err, fs.ErrNotExist) {
		var dir string
		dir, err = filepath.EvalSymlinks(filepath.Dir(path))
		resolved = filepath.Join(dir, filepath.Base(path))
	}
	if err != nil {
		return false
	}
	shared := filepath.Join(root, sharedDir)
	if s, err := filepath.EvalSymlinks(shared); err == nil {
		shared = s
	}
	return within(root, resolved) && !within(shared, resolved)
}

// within reports whether path is base or lies below it (both cleaned).
func within(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
