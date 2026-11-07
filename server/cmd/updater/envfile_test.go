package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
		{"export and indent kept", "  export SLOUCHCAT_VERSION = 1.15.0 # pinned\n", "  export SLOUCHCAT_VERSION =1.16.0\n"},
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
	} {
		if got := inside(root, tc.path); got != tc.want {
			t.Errorf("inside(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
