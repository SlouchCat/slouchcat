package admin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// A-1: the wizard's path check describes a path instead of failing on it.
func TestCheckSetupPath(t *testing.T) {
	ctx := model.WithClientIP(context.Background(), "192.0.2.20")
	f := newFixture(t)
	f.libs.validateDir = true
	dir := t.TempDir()
	file := filepath.Join(dir, "film.mkv")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	type pathCase struct {
		path        string
		wantOK      bool
		wantProblem string
	}
	cases := map[string]pathCase{
		"directory":    {path: dir, wantOK: true},
		"relative":     {path: "media/movies", wantProblem: "not_absolute"},
		"missing":      {path: filepath.Join(dir, "missing"), wantProblem: "missing"},
		"file":         {path: file, wantProblem: "not_directory"},
		"below a file": {path: filepath.Join(file, "x"), wantProblem: "not_directory"},
	}
	if os.Geteuid() != 0 { // root reads every directory; the case cannot occur for it
		locked := filepath.Join(dir, "locked")
		if err := os.Mkdir(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
		cases["unreadable"] = pathCase{path: locked, wantProblem: "not_readable"}
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := f.svc.CheckSetupPath(ctx, tc.path)
			if err != nil {
				t.Fatal(err)
			}
			if got.OK != tc.wantOK || got.Problem != tc.wantProblem {
				t.Fatalf("check = %+v, want ok %v, problem %q", got, tc.wantOK, tc.wantProblem)
			}
			if tc.wantOK && (got.FSType == "" || got.WatchMode == "") {
				t.Errorf("a usable path lacks its file system or watch mode: %+v", got)
			}
			if !tc.wantOK && (got.FSType != "" || got.WatchMode != "" || got.Warning != "") {
				t.Errorf("a bad path carries details: %+v", got)
			}
		})
	}
	var verr *model.ValidationError
	if _, err := f.svc.CheckSetupPath(ctx, "  "); !errors.As(err, &verr) {
		t.Errorf("empty path: %v, want a validation error", err)
	}
}

// E14: a path below no browsable root answers not_browsable and is not described any further.
// The evidence that nothing stat'ed it is the file: library.PathProblem would have answered
// not_directory for it, which is exactly the leak the own branch avoids.
func TestCheckSetupPathNotBrowsable(t *testing.T) {
	ctx := model.WithClientIP(context.Background(), "192.0.2.24")
	f := newFixture(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "film.mkv")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.libs.notBrowsable = map[string]bool{dir: true, file: true, "/config": true}
	for _, path := range []string{dir, file, "/config"} {
		got, err := f.svc.CheckSetupPath(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		if got.OK || got.Problem != library.PathNotBrowsable {
			t.Errorf("CheckSetupPath(%q) = %+v, want problem not_browsable", path, got)
		}
		if got.FSType != "" || got.WatchMode != "" || got.Warning != "" {
			t.Errorf("CheckSetupPath(%q) describes the path: %+v", path, got)
		}
	}
	if len(f.libs.validated) != 3 {
		t.Errorf("the check ran %d times, want one per path", len(f.libs.validated))
	}
}

// A failure that is not about the path -- the database, say -- must not be described as a path
// problem: the operator would go and fix a folder that is fine.
func TestCheckSetupPathPassesOnOtherFailures(t *testing.T) {
	ctx := model.WithClientIP(context.Background(), "192.0.2.25")
	f := newFixture(t)
	boom := errors.New("database is locked")
	f.libs.validateErr = boom
	if _, err := f.svc.CheckSetupPath(ctx, t.TempDir()); !errors.Is(err, boom) {
		t.Errorf("CheckSetupPath with a failing check = %v, want the error itself", err)
	}
}

// A-1: the key check names an invalid key and an unreachable TMDB; the key is not stored.
func TestCheckSetupTMDBKey(t *testing.T) {
	ctx := model.WithClientIP(context.Background(), "192.0.2.21")
	f := newFixture(t)
	for name, tc := range map[string]struct {
		err         error
		wantOK      bool
		wantProblem string
	}{
		"valid":       {wantOK: true},
		"invalid":     {err: model.ErrTMDBKeyInvalid, wantProblem: "invalid"},
		"unreachable": {err: model.ErrTMDBUnreachable, wantProblem: "unreachable"},
	} {
		t.Run(name, func(t *testing.T) {
			f.tmdb.err = tc.err
			got, err := f.svc.CheckSetupTMDBKey(ctx, " "+fakeKey+" ")
			if err != nil {
				t.Fatal(err)
			}
			if got.OK != tc.wantOK || got.Problem != tc.wantProblem {
				t.Fatalf("check = %+v, want ok %v, problem %q", got, tc.wantOK, tc.wantProblem)
			}
			if last := f.tmdb.checks[len(f.tmdb.checks)-1]; last != fakeKey {
				t.Errorf("checked key %q, want the trimmed key", last)
			}
		})
	}
	if n := f.settings(t); n != 0 {
		t.Errorf("settings after key checks = %d, want 0", n)
	}
	var verr *model.ValidationError
	if _, err := f.svc.CheckSetupTMDBKey(ctx, ""); !errors.As(err, &verr) {
		t.Errorf("empty key: %v, want a validation error", err)
	}
}

// Both checks count against the attempt limit and end with the wizard.
func TestSetupChecksLimitedAndOnlyBeforeSetup(t *testing.T) {
	ctx := model.WithClientIP(context.Background(), "192.0.2.22")
	f := newFixture(t)
	dir := t.TempDir()
	var err error
	for range 11 {
		if _, err = f.svc.CheckSetupPath(ctx, dir); err != nil {
			break
		}
	}
	if !errors.Is(err, model.ErrTooManyAttempts) {
		t.Errorf("11th check: %v, want ErrTooManyAttempts", err)
	}

	other := model.WithClientIP(context.Background(), "192.0.2.23")
	if _, _, err := f.auth.CreateOwner(other, "marco@example.ch", fakePassword, "de"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CheckSetupPath(other, dir); !errors.Is(err, model.ErrSetupDone) {
		t.Errorf("path check after the wizard: %v, want ErrSetupDone", err)
	}
	if _, err := f.svc.CheckSetupTMDBKey(other, fakeKey); !errors.Is(err, model.ErrSetupDone) {
		t.Errorf("key check after the wizard: %v, want ErrSetupDone", err)
	}
}
