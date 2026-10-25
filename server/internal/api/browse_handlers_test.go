package api

import (
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/browse"
	"github.com/slouchcat/slouchcat/server/internal/paths"
)

// browseEnv is an authEnv whose folder browser may list one temporary directory.
func browseEnv(t *testing.T) (*authEnv, string) {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"Schatzkammer", "Serien", "@eaDir"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	wire := func(a *admin.Service) {
		a.WithBrowse([]string{dir}, browse.ClassifyRoots([]string{dir}, quiet),
			browse.New(quiet, browse.WithConfinedTo(browse.ResolveRoots([]string{dir}, quiet))))
	}
	return newAuthEnvAt(t, &stepClock{now: time.Now()}, wire), dir
}

func TestBrowseEndpoints(t *testing.T) {
	t.Parallel()
	e, dir := browseEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)

	rec = e.call(t, http.MethodGet, "/api/v1/admin/fs/roots", nil, owner)
	wantStatus(t, rec, http.StatusOK)
	wantNoStore(t, rec)
	var roots []gen.BrowseRoot
	if err := json.Unmarshal(rec.Body.Bytes(), &roots); err != nil {
		t.Fatal(err)
	}
	// The configured root and the root of the library the wizard created (/media/Filme, which
	// lies below no configured root and is therefore shown but not browsable).
	var configured *gen.BrowseRoot
	for i, r := range roots {
		if r.Path == dir {
			configured = &roots[i]
		}
		if r.Path == "/media/Filme" && (r.Browsable == nil || *r.Browsable) {
			t.Errorf("a library root below no configured root is browsable: %+v", r)
		}
	}
	if configured == nil || configured.Source != gen.BrowseRootConfigured || configured.FsType == nil {
		t.Fatalf("roots = %+v, want the configured one with its file system", roots)
	}

	rec = e.call(t, http.MethodPost, "/api/v1/admin/fs/browse", map[string]any{"rootId": configured.Id}, owner)
	wantStatus(t, rec, http.StatusOK)
	wantNoStore(t, rec)
	var listing gen.DirectoryListing
	if err := json.Unmarshal(rec.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if !listing.AtRoot || listing.Rel != "" || listing.ParentRel != nil || listing.Watch == nil {
		t.Fatalf("listing = %+v, want the root itself with its watch mode", listing)
	}
	names := make([]string, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		names = append(names, entry.Name)
	}
	if strings.Join(names, "|") != "Schatzkammer|Serien" {
		t.Errorf("entries = %v, want the two directories without the NAS metadata folder", names)
	}

	// One level down, and the path of the directory never reaches the request log: that is why
	// this endpoint is a POST (the log keeps the query string of a GET).
	e.logs.Reset()
	rec = e.call(t, http.MethodPost, "/api/v1/admin/fs/browse",
		map[string]any{"rootId": configured.Id, "rel": "Schatzkammer"}, owner)
	wantStatus(t, rec, http.StatusOK)
	if strings.Contains(e.logs.String(), "Schatzkammer") || strings.Contains(e.logs.String(), dir) {
		t.Errorf("the request log names the browsed path: %s", e.logs.String())
	}
}

// E6: an unknown root and a rel that names nothing below a known one answer with the same body,
// so the endpoint tells nothing about the file system outside the roots. And no case of this
// endpoint answers 403 for a path: that status belongs to the session and the CSRF header.
func TestBrowseHidesWhatIsOutside(t *testing.T) {
	t.Parallel()
	e, dir := browseEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)
	rootID := firstRootID(t, e, owner, dir)

	bodies := map[string]string{}
	for name, body := range map[string]map[string]any{
		"unknown root":    {"rootId": "c999"},
		"missing rel":     {"rootId": rootID, "rel": "gibtsnicht"},
		"deep missing":    {"rootId": rootID, "rel": "Schatzkammer/tiefer"},
		"library root":    {"rootId": "l1"}, // /media/Filme: grandfathered, listed, not browsable
		"empty root name": {"rootId": "c"},
	} {
		rec := e.call(t, http.MethodPost, "/api/v1/admin/fs/browse", body, owner)
		if rec.Code != http.StatusNotFound || problemCode(t, rec) != "path_not_browsable" {
			t.Errorf("%s: status %d (%s), want 404 path_not_browsable", name, rec.Code, rec.Body.String())
		}
		bodies[name] = rec.Body.String()
	}
	first := ""
	for name, body := range bodies {
		if first == "" {
			first = body
			continue
		}
		if body != first {
			t.Errorf("%s answers a different body than the others:\n%s\n%s", name, body, first)
		}
	}

	// A bad request is a validation error, never a 404 that would hide a typo, and never a 403.
	for name, body := range map[string]map[string]any{
		"dot dot":  {"rootId": rootID, "rel": ".."},
		"inside":   {"rootId": rootID, "rel": "a/../b"},
		"absolute": {"rootId": rootID, "rel": "/etc/passwd"},
		"nul byte": {"rootId": rootID, "rel": "a\x00b"},
		"limit":    {"rootId": rootID, "limit": 5000},
	} {
		rec := e.call(t, http.MethodPost, "/api/v1/admin/fs/browse", body, owner)
		if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "validation_failed" {
			t.Errorf("%s: status %d (%s), want 422 validation_failed", name, rec.Code, rec.Body.String())
		}
	}

	// 403 on this endpoint means the session or the CSRF header, never a path.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/fs/browse", strings.NewReader(`{"rootId":"`+rootID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(owner)
	rec = httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || problemCode(t, rec) != "csrf_header_missing" {
		t.Errorf("without the CSRF header: status %d (%s), want 403 csrf_header_missing", rec.Code, rec.Body.String())
	}
}

// E9 over HTTP: a file system that does not answer gives 503 path_unreachable with Retry-After,
// and once every slot is held by a call that never came back, browse_busy.
func TestBrowseUnreachableAndBusy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	blocked := make(chan struct{})
	t.Cleanup(func() { close(blocked) })
	wire := func(a *admin.Service) {
		a.WithBrowse([]string{dir}, browse.ClassifyRoots([]string{dir}, quiet),
			browse.New(quiet, browse.WithFS(blockingFS{blocked}), browse.WithSlots(1),
				browse.WithWatchdog(20*time.Millisecond), browse.WithGrace(time.Minute)))
	}
	e := newAuthEnvAt(t, &stepClock{now: time.Now()}, wire)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)
	rootID := firstRootID(t, e, owner, dir)

	rec = e.call(t, http.MethodPost, "/api/v1/admin/fs/browse", map[string]any{"rootId": rootID}, owner)
	wantStatus(t, rec, http.StatusServiceUnavailable)
	if code := problemCode(t, rec); code != "path_unreachable" {
		t.Errorf("code = %s, want path_unreachable", code)
	}
	if got := rec.Header().Get("Retry-After"); got != "30" {
		t.Errorf("Retry-After = %q, want 30", got)
	}
	// The slot is held by the call that never came back; the next one is turned away at once.
	rec = e.call(t, http.MethodPost, "/api/v1/admin/fs/browse", map[string]any{"rootId": rootID}, owner)
	wantStatus(t, rec, http.StatusServiceUnavailable)
	if code := problemCode(t, rec); code != "browse_busy" {
		t.Errorf("code = %s, want browse_busy", code)
	}
}

// blockingFS never answers, like a dead network mount.
type blockingFS struct{ block chan struct{} }

func (f blockingFS) ReadDirNames(paths.Root, string, int) ([]string, error) {
	<-f.block
	return nil, nil
}

func (f blockingFS) LstatAll(paths.Root, string, []string) ([]fs.DirEntry, error) {
	<-f.block
	return nil, nil
}

func (f blockingFS) FSTypeAt(paths.Root, string) (paths.FSType, error) {
	<-f.block
	return paths.FSUnknown, nil
}

// E5: the wizard's root list does not count against the attempt limit, which it shares with the
// setup itself -- ten loads of the setup page must not lock the operator out.
func TestSetupRootsDoesNotSpendSetupAttempts(t *testing.T) {
	t.Parallel()
	e, dir := browseEnv(t)
	for i := range 10 {
		rec := e.call(t, http.MethodGet, "/api/v1/setup/roots", nil, nil)
		wantStatus(t, rec, http.StatusOK)
		if i == 0 {
			wantNoStore(t, rec)
			var roots []gen.BrowseRoot
			if err := json.Unmarshal(rec.Body.Bytes(), &roots); err != nil {
				t.Fatal(err)
			}
			if len(roots) != 1 || roots[0].Path != dir {
				t.Fatalf("roots = %+v, want the configured one", roots)
			}
			// Before an account exists nobody learns which mount is a network share.
			if roots[0].FsType != nil || roots[0].Warning != nil {
				t.Errorf("the wizard's list describes the file system: %+v", roots[0])
			}
		}
	}
	wantStatus(t, e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil), http.StatusCreated)
	// Once the wizard ran the list ends with it.
	rec := e.call(t, http.MethodGet, "/api/v1/setup/roots", nil, nil)
	wantStatus(t, rec, http.StatusConflict)
	if code := problemCode(t, rec); code != "setup_done" {
		t.Errorf("after the wizard: code %s, want setup_done", code)
	}
}

// firstRootID returns the id of the configured root with this path.
func firstRootID(t *testing.T, e *authEnv, owner *http.Cookie, dir string) string {
	t.Helper()
	rec := e.call(t, http.MethodGet, "/api/v1/admin/fs/roots", nil, owner)
	wantStatus(t, rec, http.StatusOK)
	var roots []gen.BrowseRoot
	if err := json.Unmarshal(rec.Body.Bytes(), &roots); err != nil {
		t.Fatal(err)
	}
	for _, r := range roots {
		if r.Path == dir {
			return r.Id
		}
	}
	t.Fatalf("no root for %s in %+v", dir, roots)
	return ""
}

// wantNoStore: a directory listing names folders of the media tree, so no cache keeps one.
func wantNoStore(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}
