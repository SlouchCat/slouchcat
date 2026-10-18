package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store"
)

const (
	authTestPassword = "FAKE-password-4711"
	authTestKey      = "FAKE-tmdb-token-4711"
	authTestBadPath  = "/missing"
)

// authTestLibraries is an in-memory admin.Libraries.
type authTestLibraries struct{ libs []model.Library }

func (f *authTestLibraries) ValidatePath(_ context.Context, path string) error {
	if path == authTestBadPath {
		return model.ErrLibraryPathInvalid
	}
	return nil
}

func (f *authTestLibraries) Create(_ context.Context, req model.LibraryRequest) (model.Library, error) {
	started := time.Date(2026, 10, 18, 14, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	lib := model.Library{
		ID: int64(len(f.libs) + 1), Name: req.Name, Kind: req.Kind, CreatedAt: started,
		Roots: []model.LibraryRoot{{ID: int64(len(f.libs) + 1), Path: req.Path, FSType: "nfs", Watch: "poll", Warning: "network_share"}},
		Scan:  model.LibraryScan{State: "queued", StartedAt: &started},
	}
	f.libs = append(f.libs, lib)
	return lib, nil
}

func (f *authTestLibraries) List(context.Context) ([]model.Library, error) { return f.libs, nil }

func (f *authTestLibraries) Get(_ context.Context, id int64) (model.Library, error) {
	if id < 1 || id > int64(len(f.libs)) {
		return model.Library{}, model.ErrNotFound
	}
	return f.libs[id-1], nil
}

func (f *authTestLibraries) QueueScan(ctx context.Context, id int64) (model.Library, error) {
	return f.Get(ctx, id)
}

func (f *authTestLibraries) AcceptLoss(ctx context.Context, id int64) (model.Library, error) {
	return f.Get(ctx, id)
}

type authTestTMDB struct{ err error }

func (f *authTestTMDB) CheckKey(context.Context, string) error { return f.err }

type authEnv struct {
	h    http.Handler
	db   *store.DB
	auth *auth.Service
	tmdb *authTestTMDB
	logs *bytes.Buffer
}

func newAuthEnv(t *testing.T) *authEnv {
	t.Helper()
	var logs bytes.Buffer
	log, err := logging.New(&logs, logging.FormatJSON, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	db, err := store.Open(context.Background(), store.Options{ConfigDir: dir, Version: "test", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ring, err := secrets.Load(dir, log)
	if err != nil {
		t.Fatal(err)
	}
	a := auth.New(db, log)
	tmdb := &authTestTMDB{}
	adm := admin.New(db, a, &authTestLibraries{}, ring, tmdb, func(string) string { return "" }, log)
	h := NewHandler(Options{
		Version: "test", Started: time.Now(), Logger: log, Web: http.NotFoundHandler(), Auth: a, Admin: adm,
	})
	return &authEnv{h: h, db: db, auth: a, tmdb: tmdb, logs: &logs}
}

// call sends a request with an optional JSON body and cookie; POSTs carry the CSRF header.
func (e *authEnv) call(t *testing.T, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set(CSRFHeader, CSRFHeaderValue)
	}
	req.Header.Set("User-Agent", "handler-test")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func setupBody() map[string]any {
	return map[string]any{
		"email": "marco@example.ch", "password": authTestPassword, "language": "en", "tmdbApiKey": authTestKey,
		"library": map[string]any{"name": "Filme", "kind": "movies", "path": "/media/Filme"},
	}
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookie {
			return c
		}
	}
	t.Fatalf("no session cookie in %v", rec.Header())
	return nil
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body.String())
	}
}

func setupStatus(t *testing.T, e *authEnv) bool {
	t.Helper()
	rec := e.call(t, http.MethodGet, "/api/v1/setup", nil, nil)
	wantStatus(t, rec, http.StatusOK)
	var st gen.SetupStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	return st.SetupRequired
}

func TestSetupFlowEndToEnd(t *testing.T) {
	e := newAuthEnv(t)
	if !setupStatus(t, e) {
		t.Fatal("fresh server does not require setup")
	}

	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	cookie := sessionCookie(t, rec)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.MaxAge <= 0 {
		t.Fatalf("cookie attributes %+v", cookie)
	}
	var info gen.SessionInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Account.Email != "marco@example.ch" || info.Account.Role != gen.Owner || info.Profile.Name != "marco" ||
		info.Profile.Language != gen.En || info.Profile.MaxRating != 18 {
		t.Fatalf("session info = %+v", info)
	}
	if strings.Contains(rec.Body.String(), cookie.Value) {
		t.Fatal("token in the response body")
	}

	if setupStatus(t, e) {
		t.Fatal("setup still required after the wizard")
	}
	rec = e.call(t, http.MethodGet, "/api/v1/auth/session", nil, cookie)
	wantStatus(t, rec, http.StatusOK)
	var again gen.SessionInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &again); err != nil {
		t.Fatal(err)
	}
	if again != info {
		t.Fatalf("GET session = %+v, want %+v", again, info)
	}

	rec = e.call(t, http.MethodPost, "/api/v1/auth/logout", nil, cookie)
	wantStatus(t, rec, http.StatusNoContent)
	if c := sessionCookie(t, rec); c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("logout did not clear the cookie: %+v", c)
	}
	rec = e.call(t, http.MethodGet, "/api/v1/auth/session", nil, cookie)
	wantStatus(t, rec, http.StatusUnauthorized)
	if code := problemCode(t, rec); code != "unauthenticated" {
		t.Fatalf("code = %s", code)
	}

	// Login again with the e-mail in another case.
	rec = e.call(t, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "Marco@Example.ch", "password": authTestPassword}, nil)
	wantStatus(t, rec, http.StatusOK)
	rec = e.call(t, http.MethodGet, "/api/v1/auth/session", nil, sessionCookie(t, rec))
	wantStatus(t, rec, http.StatusOK)

	if out := e.logs.String(); strings.Contains(out, authTestPassword) || strings.Contains(out, authTestKey) || strings.Contains(out, cookie.Value) {
		t.Fatalf("secret in logs: %s", out)
	}
}

func TestSetupConflictsAndValidation(t *testing.T) {
	e := newAuthEnv(t)

	bad := setupBody()
	bad["password"] = "abc"
	bad["library"] = map[string]any{"name": "", "kind": "movies", "path": ""}
	rec := e.call(t, http.MethodPost, "/api/v1/setup", bad, nil)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	var p gen.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Code != "validation_failed" || p.Errors == nil {
		t.Fatalf("problem = %+v", p)
	}
	want := []gen.FieldError{{Field: "password", Code: "too_short"}, {Field: "library.name", Code: "required"}, {Field: "library.path", Code: "required"}}
	if !slices.Equal(*p.Errors, want) {
		t.Fatalf("field errors = %v, want %v", *p.Errors, want)
	}

	bad = setupBody()
	bad["library"] = map[string]any{"name": "Filme", "kind": "movies", "path": authTestBadPath}
	rec = e.call(t, http.MethodPost, "/api/v1/setup", bad, nil)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if code := problemCode(t, rec); code != "library_path_invalid" {
		t.Fatalf("code = %s", code)
	}

	e.tmdb.err = model.ErrTMDBKeyInvalid
	rec = e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if code := problemCode(t, rec); code != "tmdb_key_invalid" {
		t.Fatalf("code = %s", code)
	}
	if strings.Contains(rec.Body.String(), authTestKey) {
		t.Fatal("key echoed in the problem")
	}
	if !setupStatus(t, e) {
		t.Fatal("a failed setup created the account")
	}

	e.tmdb.err = nil
	wantStatus(t, e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil), http.StatusCreated)
	rec = e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusConflict)
	if code := problemCode(t, rec); code != "setup_done" {
		t.Fatalf("code = %s", code)
	}
}

func TestLoginFailures(t *testing.T) {
	e := newAuthEnv(t)
	wantStatus(t, e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil), http.StatusCreated)
	for _, body := range []map[string]any{
		{"email": "marco@example.ch", "password": "wrong-password"},
		{"email": "nobody@example.ch", "password": authTestPassword},
	} {
		rec := e.call(t, http.MethodPost, "/api/v1/auth/login", body, nil)
		wantStatus(t, rec, http.StatusUnauthorized)
		if code := problemCode(t, rec); code != "invalid_credentials" {
			t.Fatalf("code = %s", code)
		}
		if len(rec.Result().Cookies()) != 0 {
			t.Fatal("failed login set a cookie")
		}
	}
	rec := e.call(t, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "marco@example.ch"}, nil)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if code := problemCode(t, rec); code != "validation_failed" {
		t.Fatalf("code = %s", code)
	}
}
