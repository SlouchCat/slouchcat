package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/updater"
)

// The updater runs as root and shares deploy/updater/ with the server, which can put a symbolic
// link at any name there. A chmod or chown by path would follow it; the updater sets mode and
// owner only through open descriptors.
func TestNoChownOrChmodByPath(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && (pkg.Name == "os" || pkg.Name == "syscall") {
				switch sel.Sel.Name {
				case "Chown", "Chmod", "Lchown", "Chtimes":
					t.Errorf("%s: %s.%s by path", fset.Position(sel.Pos()), pkg.Name, sel.Sel.Name)
				}
			}
			return true
		})
	}
}

// A symbolic link the server planted at state.json is replaced by the file; its target keeps its
// content and mode.
func TestPersistReplacesPlantedLink(t *testing.T) {
	h := newHarness(t)
	h.start()
	victim := filepath.Join(t.TempDir(), "docker.sock")
	if err := os.WriteFile(victim, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(h.deploy, "updater", updater.StateFile)
	if err := os.Symlink(victim, path); err != nil {
		t.Fatal(err)
	}
	h.u.persist(updater.State{Phase: updater.PhaseIdle})
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
		t.Fatalf("state.json = %v, %v, want a regular file 0644", info, err)
	}
	if data, err := os.ReadFile(victim); err != nil || string(data) != "keep" {
		t.Errorf("link target = %q, %v", data, err)
	}
	if info, err := os.Stat(victim); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("link target mode = %v, %v, want 0600", info, err)
	}
}

// A token that is not a regular file of its own is removed (not its target) and replaced; a
// FIFO does not block the start.
func TestStartReplacesTokenThatIsNoRegularFile(t *testing.T) {
	for _, tc := range []struct {
		name  string
		plant func(t *testing.T, path, victim string)
	}{
		{"symbolic link", func(t *testing.T, path, victim string) {
			if err := os.Symlink(victim, path); err != nil {
				t.Fatal(err)
			}
		}},
		{"hard link", func(t *testing.T, path, victim string) {
			if err := os.Link(victim, path); err != nil {
				t.Fatal(err)
			}
		}},
		{"fifo", func(t *testing.T, path, _ string) {
			if err := syscall.Mkfifo(path, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			dir := filepath.Join(h.deploy, "updater")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			// The victim lies in the deploy folder so that a hard link to it is possible.
			victim := filepath.Join(h.deploy, "victim")
			secret := strings.Repeat("v", 64)
			if err := os.WriteFile(victim, []byte(secret), 0o640); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, updater.TokenFile)
			tc.plant(t, path, victim)
			h.start()
			if string(h.token) == secret || len(h.token) != 64 {
				t.Errorf("token = %d characters, taken from the planted file: %v", len(h.token), string(h.token) == secret)
			}
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
				t.Errorf("token file = %v, %v, want a regular file 0600", info, err)
			}
			if data, err := os.ReadFile(victim); err != nil || string(data) != secret {
				t.Errorf("victim = %q, %v, want it untouched", data, err)
			}
			if info, err := os.Stat(victim); err != nil || info.Mode().Perm() != 0o640 {
				t.Errorf("victim mode = %v, %v, want 0640 as before", info, err)
			}
			if !strings.Contains(h.logs.String(), "replacing it") {
				t.Errorf("the replacement is not logged: %s", h.logs)
			}
		})
	}
}
