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
	"strings"
	"syscall"
)

// versionKey is the one variable the updater writes (ADR 0024 point 11).
const versionKey = "SLOUCHCAT_VERSION"

// versionLine matches a SLOUCHCAT_VERSION assignment at the start of a line, with the optional
// indent and "export " that compose's dotenv parser accepts. Group 1 is everything up to and
// including "=".
var versionLine = regexp.MustCompile(`^([ \t]*(?:export[ \t]+)?` + versionKey + `[ \t]*=)`)

// setVersion returns data with the value of every SLOUCHCAT_VERSION line set to v and all other
// bytes unchanged: line endings, the spaces around the value, its quotes ("…" or '…') and a
// trailing comment. Without such a line it appends one, with the line ending the file already
// uses (CRLF when it has one, else LF), after closing an unterminated last line.
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
	out.WriteString(versionKey + "=" + v + eol)
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
// line is appended. A file that does not exist counts as empty. A symbolic link is read through
// its target (checked by inside), and the file must be a regular one (readRegular).
func envTarget(files []string) (string, error) {
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

// backupName is the name of the backup taken before installing v.
func (f envFile) backupName(v string) string {
	return f.path + ".bak-vor-" + v
}

// backup copies the file to <name>.bak-vor-<v> (0600, owned like the original: it holds secrets)
// and never deletes another backup: hand-made ones share the name pattern. A backup of that name
// already there is replaced. A missing file has nothing to back up.
func (f envFile) backup(v string) error {
	if !f.exists {
		return nil
	}
	return writeAtomic(f.backupName(v), f.data, 0o600, f.uid, f.gid)
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
