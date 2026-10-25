package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

func TestLibraryEndpoints(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)

	rec = e.call(t, http.MethodGet, "/api/v1/admin/libraries", nil, owner)
	wantStatus(t, rec, http.StatusOK)
	var list []gen.Library
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Filme" || list[0].Kind != gen.LibraryKindMovies || len(list[0].Roots) != 1 {
		t.Fatalf("libraries = %+v", list)
	}
	root := list[0].Roots[0]
	if root.Path != "/media/Filme" || root.FsType != "nfs" || root.Watch != gen.Poll || root.Warning == nil || *root.Warning != "network_share" {
		t.Fatalf("root = %+v", root)
	}
	scan := list[0].Scan
	if scan.State != gen.ScanStateQueued || scan.StartedAt == nil || scan.StartedAt.Location() != time.UTC || scan.FilesSeen == nil {
		t.Fatalf("scan = %+v", scan)
	}
	if !scan.StartedAt.Equal(time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("startedAt = %v", scan.StartedAt)
	}

	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries", map[string]any{"name": "Serien", "kind": "series", "path": "/media/Serien"}, owner)
	wantStatus(t, rec, http.StatusCreated)
	var created gen.Library
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Id != 2 || created.Kind != gen.LibraryKindSeries {
		t.Fatalf("created = %+v", created)
	}

	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries", map[string]any{"name": " ", "kind": "series", "path": "/x"}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if code := problemCode(t, rec); code != "validation_failed" {
		t.Fatalf("code = %s", code)
	}

	wantStatus(t, e.call(t, http.MethodGet, "/api/v1/admin/libraries/2", nil, owner), http.StatusOK)
	wantStatus(t, e.call(t, http.MethodPost, "/api/v1/admin/libraries/2/scan", nil, owner), http.StatusAccepted)
	wantStatus(t, e.call(t, http.MethodPost, "/api/v1/admin/libraries/1/accept-loss", nil, owner), http.StatusAccepted)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/libraries/99"},
		{http.MethodPost, "/api/v1/admin/libraries/99/scan"},
		{http.MethodPost, "/api/v1/admin/libraries/99/accept-loss"},
	} {
		rec := e.call(t, tc.method, tc.path, nil, owner)
		wantStatus(t, rec, http.StatusNotFound)
		if code := problemCode(t, rec); code != "not_found" {
			t.Fatalf("%s %s: code = %s", tc.method, tc.path, code)
		}
	}

	rec = e.call(t, http.MethodGet, "/api/v1/admin/libraries", nil, nil)
	wantStatus(t, rec, http.StatusUnauthorized)
}

// A non-owner session is refused on admin routes (M2 brings guests; the role exists in the schema).
func TestLibraryEndpointsRefuseGuests(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	wantStatus(t, e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil), http.StatusCreated)
	ctx := context.Background()
	res, err := e.db.Write.ExecContext(ctx,
		`INSERT INTO accounts (email, password_hash, role, created_at, updated_at) VALUES ('guest@example.ch', 'x', 'guest', 0, 0)`)
	if err != nil {
		t.Fatal(err)
	}
	accountID, _ := res.LastInsertId()
	res, err = e.db.Write.ExecContext(ctx,
		`INSERT INTO profiles (account_id, name, created_at, updated_at) VALUES (?, 'Gast', 0, 0)`, accountID)
	if err != nil {
		t.Fatal(err)
	}
	profileID, _ := res.LastInsertId()
	_, token, err := e.auth.StartSession(ctx, model.Account{ID: accountID, Role: "guest"}, model.Profile{ID: profileID}, "")
	if err != nil {
		t.Fatal(err)
	}
	guest := &http.Cookie{Name: SessionCookie, Value: token}
	wantStatus(t, e.call(t, http.MethodGet, "/api/v1/auth/session", nil, guest), http.StatusOK)
	rec := e.call(t, http.MethodGet, "/api/v1/admin/libraries", nil, guest)
	wantStatus(t, rec, http.StatusForbidden)
	if code := problemCode(t, rec); code != "forbidden" {
		t.Fatalf("code = %s", code)
	}
}

// TestLibraryRootEndpoints covers the two operations of idea 02, E8 and E12: a library takes a
// further folder, and a folder leaves only when the administrator repeats its path. The statuses
// are the point here; what the removal does to titles and downloads is tested in package library.
func TestLibraryRootEndpoints(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)

	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries/1/roots", map[string]any{"path": "/media/Filme-2"}, owner)
	wantStatus(t, rec, http.StatusCreated)
	var lib gen.Library
	if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
		t.Fatal(err)
	}
	if len(lib.Roots) != 2 || lib.Roots[1].Path != "/media/Filme-2" {
		t.Fatalf("roots = %+v", lib.Roots)
	}
	second := lib.Roots[1].Id

	// A missing path is a validation error, a path the rules refuse is library_path_invalid.
	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries/1/roots", map[string]any{"path": "  "}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if code := problemCode(t, rec); code != "validation_failed" {
		t.Fatalf("code = %s", code)
	}
	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries/1/roots", map[string]any{"path": authTestBadPath}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if code := problemCode(t, rec); code != "library_path_invalid" {
		t.Fatalf("code = %s", code)
	}
	wantStatus(t, e.call(t, http.MethodPost, "/api/v1/admin/libraries/99/roots", map[string]any{"path": "/media/x"}, owner), http.StatusNotFound)

	// The confirmation is compared against the stored folder, so a near miss changes nothing.
	path := func(id int64) string {
		return "/api/v1/admin/libraries/1/roots/" + strconv.FormatInt(id, 10) + "/removal"
	}
	rec = e.call(t, http.MethodPost, path(second), map[string]any{"confirmPath": "/media/Filme-3"}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if code := problemCode(t, rec); code != "confirm_mismatch" {
		t.Fatalf("code = %s", code)
	}
	rec = e.call(t, http.MethodPost, path(second), map[string]any{"confirmPath": ""}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if code := problemCode(t, rec); code != "validation_failed" {
		t.Fatalf("code = %s", code)
	}
	wantStatus(t, e.call(t, http.MethodPost, path(99), map[string]any{"confirmPath": "/media/Filme-2"}, owner), http.StatusNotFound)

	// A running scan holds the folders it started with.
	e.libs.scanRunning = true
	rec = e.call(t, http.MethodPost, path(second), map[string]any{"confirmPath": "/media/Filme-2"}, owner)
	wantStatus(t, rec, http.StatusConflict)
	if code := problemCode(t, rec); code != "scan_running" {
		t.Fatalf("code = %s", code)
	}
	e.libs.scanRunning = false

	rec = e.call(t, http.MethodPost, path(second), map[string]any{"confirmPath": "/media/Filme-2"}, owner)
	wantStatus(t, rec, http.StatusOK)
	if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
		t.Fatal(err)
	}
	if len(lib.Roots) != 1 || lib.Roots[0].Path != "/media/Filme" {
		t.Fatalf("roots after the removal = %+v", lib.Roots)
	}

	// The last folder may go too (E13); the library stays without one.
	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries/1/roots/1/removal", map[string]any{"confirmPath": "/media/Filme"}, owner)
	wantStatus(t, rec, http.StatusOK)
	if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
		t.Fatal(err)
	}
	if len(lib.Roots) != 0 || lib.Name == "" {
		t.Fatalf("library without a folder = %+v", lib)
	}
}
