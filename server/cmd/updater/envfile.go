package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
)

// versionKey is the one variable the updater writes (ADR 0024 point 11).
const versionKey = "SLOUCHCAT_VERSION"

// keepBackups is how many .env backups stay next to the env file.
const keepBackups = 5

// versionLine matches a SLOUCHCAT_VERSION assignment at the start of a line, with the optional
// indent and "export " that compose's dotenv parser accepts. Group 1 is everything up to and
// including "=".
var versionLine = regexp.MustCompile(`^([ \t]*(?:export[ \t]+)?` + versionKey + `[ \t]*=)`)

// setVersion returns data with every SLOUCHCAT_VERSION line set to v and all other bytes
// unchanged, line endings included. Without such a line it appends one, with the line ending the
// file already uses (CRLF when it has one, else LF), after closing an unterminated last line.
func setVersion(data []byte, v string) []byte {
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
		if m := versionLine.FindSubmatch(line); m != nil {
			found = true
			line = append(append([]byte{}, m[1]...), v...)
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
	out.WriteString(versionKey + "=" + v + eol)
	return out.Bytes()
}

// hasVersion reports whether data assigns SLOUCHCAT_VERSION.
func hasVersion(data []byte) bool {
	for line := range bytes.Lines(data) {
		if versionLine.Match(line) {
			return true
		}
	}
	return false
}

// envTarget picks the env file whose SLOUCHCAT_VERSION line is edited: the last listed file that
// contains it (compose lets a later --env-file win), otherwise the last listed file, where the
// line is appended. A file that does not exist counts as empty.
func envTarget(files []string) (string, error) {
	for i := len(files) - 1; i >= 0; i-- {
		data, err := os.ReadFile(files[i])
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("read env file: %w", err)
		}
		if hasVersion(data) {
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
func readEnvFile(path string, uid, gid int) (envFile, error) {
	f := envFile{path: path, mode: 0o600, uid: uid, gid: gid}
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: env file from the compose labels, checked to lie inside the deploy folder
	if err != nil {
		return f, err
	}
	f.exists, f.data, f.mode = true, data, info.Mode().Perm()
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		f.uid, f.gid = int(st.Uid), int(st.Gid)
	}
	return f, nil
}

// backupName is the name of the backup taken before installing v.
func (f envFile) backupName(v string) string {
	return f.path + ".bak-vor-" + v
}

// backup copies the file to <name>.bak-vor-<v> (0600, owned like the original: it holds secrets)
// and keeps the newest keepBackups backups. A missing file has nothing to back up.
func (f envFile) backup(v string) error {
	if !f.exists {
		return nil
	}
	if err := writeAtomic(f.backupName(v), f.data, 0o600, f.uid, f.gid); err != nil {
		return err
	}
	return rotateBackups(f.path, keepBackups)
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

// rotateBackups removes all but the newest keep backups <path>.bak-vor-* (by modification time).
func rotateBackups(path string, keep int) error {
	matches, err := filepath.Glob(globEscape(path) + ".bak-vor-*")
	if err != nil {
		return err
	}
	type backup struct {
		path string
		mod  int64
	}
	var list []backup
	for _, m := range matches {
		info, err := os.Lstat(m)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		list = append(list, backup{m, info.ModTime().UnixNano()})
	}
	slices.SortFunc(list, func(a, b backup) int {
		if c := cmp.Compare(b.mod, a.mod); c != 0 { // newest first
			return c
		}
		return strings.Compare(b.path, a.path)
	})
	var errs []error
	for _, b := range list[min(keep, len(list)):] {
		errs = append(errs, os.Remove(b.path))
	}
	return errors.Join(errs...)
}

// globEscape quotes the glob metacharacters of a literal path.
func globEscape(p string) string {
	return strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`).Replace(p)
}

// writeAtomic replaces path with data: temporary file in the same folder with mode and owner,
// sync, rename. A reader (compose) never sees half a file.
func writeAtomic(path string, data []byte, mode fs.FileMode, uid, gid int) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
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
			return errors.Join(err, tmp.Close())
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// inside reports whether path lies inside root (both cleaned, symbolic links resolved). For a
// path that does not exist (an env file the install will create) the folder is resolved instead.
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
	rel, err := filepath.Rel(root, resolved)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
