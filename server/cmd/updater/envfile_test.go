package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"
)

func TestSetVersionByteExact(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"lf", "A=1\nSLOUCHCAT_VERSION=1.15.0\nB=2\n", "A=1\nSLOUCHCAT_VERSION=1.16.0\nB=2\n"},
		{"crlf", "A=1\r\nSLOUCHCAT_VERSION=1.15.0\r\nB=2\r\n", "A=1\r\nSLOUCHCAT_VERSION=1.16.0\r\nB=2\r\n"},
		{"no trailing newline", "A=1\nSLOUCHCAT_VERSION=1.15.0", "A=1\nSLOUCHCAT_VERSION=1.16.0"},
		{"crlf without trailing newline", "A=1\r\nSLOUCHCAT_VERSION=1.15.0", "A=1\r\nSLOUCHCAT_VERSION=1.16.0"},
		{"mixed endings kept", "A=1\r\nSLOUCHCAT_VERSION=1.15.0\nB=2\r\n", "A=1\r\nSLOUCHCAT_VERSION=1.16.0\nB=2\r\n"},
		{"empty value", "SLOUCHCAT_VERSION=\n", "SLOUCHCAT_VERSION=1.16.0\n"},
		{"export, indent and comment kept", "  export SLOUCHCAT_VERSION = 1.15.0 # pinned\n", "  export SLOUCHCAT_VERSION = 1.16.0 # pinned\n"},
		{"comment without space is the value", "SLOUCHCAT_VERSION=1.15.0#x\n", "SLOUCHCAT_VERSION=1.16.0\n"},
		{"tab before # is the value", "SLOUCHCAT_VERSION=1.15.0\t# x\n", "SLOUCHCAT_VERSION=1.16.0\n"},
		{"hash at the start is the value", "SLOUCHCAT_VERSION= #x\n", "SLOUCHCAT_VERSION= 1.16.0\n"},
		{"trailing spaces kept", "SLOUCHCAT_VERSION=1.15.0  \r\n", "SLOUCHCAT_VERSION=1.16.0  \r\n"},
		{"double quotes", "SLOUCHCAT_VERSION=\"1.15.0\"\n", "SLOUCHCAT_VERSION=\"1.16.0\"\n"},
		{"double quotes and comment", "SLOUCHCAT_VERSION=\"1.15.0\"  # pinned by hand\r\n", "SLOUCHCAT_VERSION=\"1.16.0\"  # pinned by hand\r\n"},
		{"single quotes and comment", "SLOUCHCAT_VERSION='1.15.0' # x\n", "SLOUCHCAT_VERSION='1.16.0' # x\n"},
		{"escaped quote inside", "SLOUCHCAT_VERSION=\"1.1\\\"5\" # x\n", "SLOUCHCAT_VERSION=\"1.16.0\" # x\n"},
		{"empty quotes", "SLOUCHCAT_VERSION=''\n", "SLOUCHCAT_VERSION='1.16.0'\n"},
		{"unterminated quote", "SLOUCHCAT_VERSION=\"1.15.0\n", "SLOUCHCAT_VERSION=1.16.0\n"},
		{"every assignment", "SLOUCHCAT_VERSION=1.14.0\nSLOUCHCAT_VERSION=1.15.0\n", "SLOUCHCAT_VERSION=1.16.0\nSLOUCHCAT_VERSION=1.16.0\n"},
		{"comment and look-alikes untouched",
			"# SLOUCHCAT_VERSION=1.0.0\nSLOUCHCAT_VERSIONS=x\nMY_SLOUCHCAT_VERSION=1\nSLOUCHCAT_VERSION=1.15.0\n",
			"# SLOUCHCAT_VERSION=1.0.0\nSLOUCHCAT_VERSIONS=x\nMY_SLOUCHCAT_VERSION=1\nSLOUCHCAT_VERSION=1.16.0\n"},
		{"other bytes untouched", "PW='a=b\"c'\t\n\nSLOUCHCAT_VERSION=1.15.0\n\xff\xfe\n", "PW='a=b\"c'\t\n\nSLOUCHCAT_VERSION=1.16.0\n\xff\xfe\n"},
		{"append lf", "A=1\n", "A=1\nSLOUCHCAT_VERSION=1.16.0\n"},
		{"append crlf", "A=1\r\nB=2\r\n", "A=1\r\nB=2\r\nSLOUCHCAT_VERSION=1.16.0\r\n"},
		{"append after unterminated line", "A=1", "A=1\nSLOUCHCAT_VERSION=1.16.0\n"},
		{"append after unterminated crlf file", "A=1\r\nB=2", "A=1\r\nB=2\r\nSLOUCHCAT_VERSION=1.16.0\r\n"},
		{"append to empty", "", "SLOUCHCAT_VERSION=1.16.0\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(setVersion([]byte(tc.in), "1.16.0")); got != tc.want {
				t.Errorf("setVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestEnvTarget(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := write("a.env", "SLOUCHCAT_VERSION=1.15.0\n")
	b := write("b.env", "OTHER=1\n")
	c := write("c.env", "SLOUCHCAT_VERSION=1.14.0\n")
	missing := filepath.Join(dir, "missing.env")
	for _, tc := range []struct {
		files []string
		want  string
	}{
		{[]string{a, b}, a},       // the last file that contains the line
		{[]string{a, c, b}, c},    // a later file wins in compose
		{[]string{b}, b},          // none contains it: append to the last
		{[]string{a, missing}, a}, // a missing file counts as empty
		{[]string{b, missing}, missing},
	} {
		got, err := envTarget(tc.files)
		if err != nil || got != tc.want {
			t.Errorf("envTarget(%v) = %q, %v, want %q", tc.files, got, err, tc.want)
		}
	}
}

func TestEnvFileBackupAndRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	orig := "SLOUCHCAT_VERSION=1.15.0\r\nSECRET=x"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := readEnvFile(path, os.Getuid(), os.Getgid())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.backup("1.16.0"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path + ".bak-vor-1.16.0")
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup = %v, %v", info, err)
	}
	if err := f.write(setVersion(f.data, "1.16.0")); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o644 {
		t.Errorf("mode after write = %v, want 0644 as before", info.Mode())
	}
	if err := f.restore(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != orig {
		t.Errorf("restored = %q, want %q", got, orig)
	}

	// A missing env file is created by the install and removed again by the restore.
	gone := filepath.Join(dir, "new.env")
	f, err = readEnvFile(gone, os.Getuid(), os.Getgid())
	if err != nil || f.exists {
		t.Fatalf("readEnvFile(missing) = %+v, %v", f, err)
	}
	if err := f.backup("1.16.0"); err != nil {
		t.Fatal(err)
	}
	if err := f.write(setVersion(nil, "1.16.0")); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(gone); info.Mode().Perm() != 0o600 {
		t.Errorf("new env file mode = %v, want 0600", info.Mode())
	}
	if err := f.restore(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gone); !os.IsNotExist(err) {
		t.Errorf("restore left the created env file: %v", err)
	}
}

func TestBackupRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("SLOUCHCAT_VERSION=1.15.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	for i := range 6 {
		p := fmt.Sprintf("%s.bak-vor-1.%d.0", path, i)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		mod := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	// Another file's backups are not this file's.
	other := filepath.Join(dir, "other.env.bak-vor-1.0.0")
	if err := os.WriteFile(other, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := readEnvFile(path, os.Getuid(), os.Getgid())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.backup("1.16.0"); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(path + ".bak-vor-*")
	if err != nil {
		t.Fatal(err)
	}
	for i := range matches {
		matches[i] = filepath.Base(matches[i])
	}
	slices.Sort(matches)
	want := []string{".env.bak-vor-1.16.0", ".env.bak-vor-1.2.0", ".env.bak-vor-1.3.0", ".env.bak-vor-1.4.0", ".env.bak-vor-1.5.0"}
	if !slices.Equal(matches, want) {
		t.Errorf("backups = %v, want %v", matches, want)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("rotation removed another file's backup: %v", err)
	}
}

func TestInside(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "out")); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(root, "updater")
	if err := os.Mkdir(shared, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, filepath.Join(root, "toshared")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "updaterx"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{root, true},
		{sub, true},
		{filepath.Join(sub, "missing.env"), true},
		{filepath.Join(root, "..", filepath.Base(root)+"x"), false},
		{filepath.Join(root, "out", "compose.yaml"), false},
		{outside, false},
		{"compose.yaml", false},
		{filepath.Join(root, "missing", "deeper.yaml"), false},
		// The shared folder, the server's to write, counts as outside, also through a link.
		{shared, false},
		{filepath.Join(shared, "compose.yaml"), false},
		{filepath.Join(shared, "..", "updater", ".env"), false},
		{filepath.Join(root, "toshared", ".env"), false},
		{filepath.Join(root, "updaterx", ".env"), true},
	} {
		if got := inside(root, tc.path); got != tc.want {
			t.Errorf("inside(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestEnvVersion(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		found    bool
	}{
		{"A=1\r\nSLOUCHCAT_VERSION=1.16.0\r\n", "1.16.0", true},
		{"SLOUCHCAT_VERSION=\"1.16.0\" # pinned\n", "1.16.0", true},
		{"SLOUCHCAT_VERSION=1.15.0\nSLOUCHCAT_VERSION='1.16.0'", "1.16.0", true}, // the last one wins
		{"# SLOUCHCAT_VERSION=1.16.0\n", "", false},
		{"", "", false},
	} {
		if got, found := envVersion([]byte(tc.in)); got != tc.want || found != tc.found {
			t.Errorf("envVersion(%q) = %q, %v, want %q, %v", tc.in, got, found, tc.want, tc.found)
		}
	}
}

// A symbolic link as env file: the target is backed up and replaced in its own folder, the link
// stays a link.
func TestEnvFileSymlinkWritesTarget(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "env"), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "env", "slouchcat.env")
	orig := "SLOUCHCAT_VERSION=1.15.0\nSECRET=x\n"
	if err := os.WriteFile(target, []byte(orig), 0o640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".env")
	if err := os.Symlink(filepath.Join("env", "slouchcat.env"), link); err != nil {
		t.Fatal(err)
	}
	f, err := readEnvFile(link, os.Getuid(), os.Getgid())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.backup("1.16.0"); err != nil {
		t.Fatal(err)
	}
	if err := f.write(setVersion(f.data, "1.16.0")); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf(".env = %v, %v, want the link kept", info, err)
	}
	if got, _ := os.ReadFile(target); string(got) != "SLOUCHCAT_VERSION=1.16.0\nSECRET=x\n" {
		t.Errorf("target = %q", got)
	}
	if info, _ := os.Stat(target); info.Mode().Perm() != 0o640 {
		t.Errorf("target mode = %v, want 0640 as before", info.Mode())
	}
	if got, err := os.ReadFile(target + ".bak-vor-1.16.0"); err != nil || string(got) != orig {
		t.Errorf("backup next to the target = %q, %v", got, err)
	}
	if err := f.restore(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(link); string(got) != orig {
		t.Errorf("restored through the link = %q", got)
	}
}

// The env file and its backup are read only as regular files: a FIFO does not hang the updater
// and is refused, also behind a symbolic link; a link at the last element is refused where it is
// not resolved first (the backup).
func TestReadRegularRefusesOtherFiles(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo.env")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	linkToFifo := filepath.Join(dir, ".env")
	if err := os.Symlink(fifo, linkToFifo); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "plain.env")
	if err := os.WriteFile(plain, []byte("SLOUCHCAT_VERSION=1.15.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkToPlain := filepath.Join(dir, "plain.env.bak-vor-1.16.0")
	if err := os.Symlink(plain, linkToPlain); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, path := range []string{fifo, linkToFifo} {
			if _, err := readEnvFile(path, os.Getuid(), os.Getgid()); !errors.Is(err, errNotRegular) {
				t.Errorf("readEnvFile(%s) = %v, want not a regular file", filepath.Base(path), err)
			}
			if _, err := envTarget([]string{path}); !errors.Is(err, errNotRegular) {
				t.Errorf("envTarget(%s) = %v, want not a regular file", filepath.Base(path), err)
			}
		}
		if _, _, err := readRegular(linkToPlain); !errors.Is(err, errNotRegular) {
			t.Errorf("readRegular(link) = %v, want not a regular file", err)
		}
		if data, info, err := readRegular(plain); err != nil || string(data) != "SLOUCHCAT_VERSION=1.15.0\n" || info.Mode().Perm() != 0o600 {
			t.Errorf("readRegular(plain) = %q, %v, %v", data, info, err)
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a FIFO blocked the read")
	}
}
