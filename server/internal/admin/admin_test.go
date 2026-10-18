package admin

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/store"
)

const (
	fakeKey      = "FAKE-tmdb-read-token-1234567890"
	fakePassword = "FAKE-password-0815"
	badPath      = "/does/not/exist"
)

type fakeLibraries struct {
	created []model.LibraryRequest
	scanned []int64
	lost    []int64
}

func (f *fakeLibraries) ValidatePath(_ context.Context, path string) error {
	if path == badPath {
		return model.ErrLibraryPathInvalid
	}
	return nil
}

func (f *fakeLibraries) Create(_ context.Context, req model.LibraryRequest) (model.Library, error) {
	f.created = append(f.created, req)
	return f.lib(int64(len(f.created))), nil
}

func (f *fakeLibraries) lib(id int64) model.Library {
	req := f.created[id-1]
	return model.Library{ID: id, Name: req.Name, Kind: req.Kind,
		Roots: []model.LibraryRoot{{ID: id, Path: req.Path, FSType: "ext4", Watch: "inotify"}},
		Scan:  model.LibraryScan{State: "queued"}}
}

func (f *fakeLibraries) List(context.Context) ([]model.Library, error) {
	out := []model.Library{}
	for i := range f.created {
		out = append(out, f.lib(int64(i+1)))
	}
	return out, nil
}

func (f *fakeLibraries) Get(_ context.Context, id int64) (model.Library, error) {
	if id < 1 || id > int64(len(f.created)) {
		return model.Library{}, model.ErrNotFound
	}
	return f.lib(id), nil
}

func (f *fakeLibraries) QueueScan(ctx context.Context, id int64) (model.Library, error) {
	f.scanned = append(f.scanned, id)
	return f.Get(ctx, id)
}

func (f *fakeLibraries) AcceptLoss(ctx context.Context, id int64) (model.Library, error) {
	f.lost = append(f.lost, id)
	return f.Get(ctx, id)
}

type fakeTMDB struct {
	err    error
	checks []string
}

func (f *fakeTMDB) CheckKey(_ context.Context, key string) error {
	f.checks = append(f.checks, key)
	return f.err
}

type fixture struct {
	svc  *Service
	auth *auth.Service
	libs *fakeLibraries
	tmdb *fakeTMDB
	ring *secrets.Keyring
	db   *store.DB
	env  map[string]string
	logs *bytes.Buffer
}

func newFixture(t *testing.T) *fixture {
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
	f := &fixture{auth: auth.New(db, log), libs: &fakeLibraries{}, tmdb: &fakeTMDB{}, ring: ring, db: db,
		env: map[string]string{}, logs: &logs}
	f.svc = New(db, f.auth, f.libs, ring, f.tmdb, func(k string) string { return f.env[k] }, log)
	return f
}

func setupRequest() model.SetupRequest {
	return model.SetupRequest{
		Email: "marco@example.ch", Password: fakePassword, Language: "de", TMDBKey: " " + fakeKey + " ",
		Library: model.LibraryRequest{Name: "Filme", Kind: "movies", Path: "/media/Filme"},
	}
}

func (f *fixture) accounts(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.db.Read.QueryRowContext(context.Background(), "SELECT count(*) FROM accounts").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (f *fixture) settings(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.db.Read.QueryRowContext(context.Background(), "SELECT count(*) FROM settings").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRunSetupHappyPath(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	sess, token, err := f.svc.RunSetup(ctx, setupRequest(), "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || sess.Account.Email != "marco@example.ch" || !sess.IsOwner() {
		t.Fatalf("session = %+v", sess)
	}
	if p := sess.Profile; p.Name != "marco" || p.MaxRating != 18 || p.Language != "de" || !p.AllLibraries {
		t.Fatalf("profile = %+v", p)
	}
	resolved, err := f.auth.Resolve(ctx, token)
	if err != nil || resolved.ID != sess.ID {
		t.Fatalf("resolve setup token: %+v, %v", resolved, err)
	}
	if len(f.libs.created) != 1 || f.libs.created[0] != setupRequest().Library {
		t.Fatalf("libraries created = %+v", f.libs.created)
	}
	if !slices.Equal(f.tmdb.checks, []string{fakeKey}) {
		t.Fatalf("tmdb checks = %v (want the trimmed key once)", f.tmdb.checks)
	}

	var stored string
	if err := f.db.Read.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", SettingTMDBKey).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, "v1:") || strings.Contains(stored, fakeKey) {
		t.Fatalf("stored setting %q is not encrypted", stored)
	}
	if plain, err := f.ring.Decrypt(stored); err != nil || plain != fakeKey {
		t.Fatalf("decrypt stored key: %v", err)
	}
	if key, err := f.svc.TMDBKey(ctx); err != nil || key != fakeKey {
		t.Fatalf("TMDBKey = %v", err)
	}
	if out := f.logs.String(); strings.Contains(out, fakeKey) || strings.Contains(out, fakePassword) {
		t.Fatalf("secret in logs: %s", out)
	}

	if _, _, err := f.svc.RunSetup(ctx, setupRequest(), ""); !errors.Is(err, model.ErrSetupDone) {
		t.Fatalf("second setup: %v, want ErrSetupDone", err)
	}
	if len(f.libs.created) != 1 {
		t.Fatal("second setup created a library")
	}
}

func TestRunSetupWithoutKey(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	req := setupRequest()
	req.TMDBKey = "   "
	if _, _, err := f.svc.RunSetup(ctx, req, ""); err != nil {
		t.Fatal(err)
	}
	if len(f.tmdb.checks) != 0 || f.settings(t) != 0 {
		t.Fatalf("empty key was checked or stored: %v", f.tmdb.checks)
	}
	if key, err := f.svc.TMDBKey(ctx); err != nil || key != "" {
		t.Fatalf("TMDBKey without key = %q, %v", key, err)
	}
}

func TestRunSetupFailuresCreateNothing(t *testing.T) {
	cases := []struct {
		name    string
		tmdbErr error
		mutate  func(*model.SetupRequest)
		want    error
	}{
		{name: "invalid key", tmdbErr: model.ErrTMDBKeyInvalid, want: model.ErrTMDBKeyInvalid},
		{name: "tmdb unreachable", tmdbErr: model.ErrTMDBUnreachable, want: model.ErrTMDBUnreachable},
		{name: "bad path", mutate: func(r *model.SetupRequest) { r.Library.Path = badPath }, want: model.ErrLibraryPathInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.tmdb.err = tc.tmdbErr
			req := setupRequest()
			if tc.mutate != nil {
				tc.mutate(&req)
			}
			_, token, err := f.svc.RunSetup(context.Background(), req, "")
			if !errors.Is(err, tc.want) || token != "" {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if strings.Contains(err.Error(), fakeKey) || strings.Contains(err.Error(), fakePassword) {
				t.Fatalf("error leaks a secret: %v", err)
			}
			if f.accounts(t) != 0 || f.settings(t) != 0 || len(f.libs.created) != 0 {
				t.Fatal("failed setup left data behind")
			}
		})
	}
}

func TestRunSetupValidationListsEveryField(t *testing.T) {
	f := newFixture(t)
	req := model.SetupRequest{
		Email: "not-an-email", Password: "short", Language: "fr", TMDBKey: strings.Repeat("k", 2000),
		Library: model.LibraryRequest{Name: " ", Kind: "music", Path: ""},
	}
	_, _, err := f.svc.RunSetup(context.Background(), req, "")
	var verr *model.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	want := []model.FieldError{
		{Field: "email", Code: "invalid_email"}, {Field: "password", Code: "too_short"},
		{Field: "language", Code: "invalid_value"}, {Field: "library.name", Code: "required"},
		{Field: "library.kind", Code: "invalid_value"}, {Field: "library.path", Code: "required"},
		{Field: "tmdbApiKey", Code: "too_long"},
	}
	if !slices.Equal(verr.Fields, want) {
		t.Fatalf("fields = %v\nwant     %v", verr.Fields, want)
	}
	if len(f.tmdb.checks) != 0 || f.accounts(t) != 0 {
		t.Fatal("validation failure checked the key or created an account")
	}
}

func TestTMDBKeyEnvWins(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	if _, _, err := f.svc.RunSetup(ctx, setupRequest(), ""); err != nil {
		t.Fatal(err)
	}
	f.env[EnvTMDBKey] = " FAKE-env-token "
	if key, err := f.svc.TMDBKey(ctx); err != nil || key != "FAKE-env-token" {
		t.Fatalf("TMDBKey with env = %q, %v", key, err)
	}
	f.env[EnvTMDBKey] = ""
	if key, err := f.svc.TMDBKey(ctx); err != nil || key != fakeKey {
		t.Fatalf("TMDBKey without env = %v", err)
	}
}

func TestLibraryDelegation(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	var verr *model.ValidationError
	if _, err := f.svc.CreateLibrary(ctx, model.LibraryRequest{Kind: "series"}); !errors.As(err, &verr) ||
		!slices.Equal(verr.Fields, []model.FieldError{{Field: "name", Code: "required"}, {Field: "path", Code: "required"}}) {
		t.Fatalf("CreateLibrary validation: %v", err)
	}
	lib, err := f.svc.CreateLibrary(ctx, model.LibraryRequest{Name: "Serien", Kind: "series", Path: "/media/Serien"})
	if err != nil || lib.ID != 1 || lib.Kind != "series" {
		t.Fatalf("CreateLibrary = %+v, %v", lib, err)
	}
	if libs, err := f.svc.ListLibraries(ctx); err != nil || len(libs) != 1 {
		t.Fatalf("ListLibraries = %v, %v", libs, err)
	}
	if _, err := f.svc.GetLibrary(ctx, 7); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("GetLibrary unknown: %v", err)
	}
	if _, err := f.svc.ScanLibrary(ctx, 1); err != nil || !slices.Equal(f.libs.scanned, []int64{1}) {
		t.Fatalf("ScanLibrary: %v %v", err, f.libs.scanned)
	}
	if _, err := f.svc.AcceptLibraryLoss(ctx, 1); err != nil || !slices.Equal(f.libs.lost, []int64{1}) {
		t.Fatalf("AcceptLibraryLoss: %v %v", err, f.libs.lost)
	}
}
