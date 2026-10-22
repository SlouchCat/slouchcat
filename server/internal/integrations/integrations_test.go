package integrations

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// fakeScanner records the path scans the service asks for.
type fakeScanner struct {
	calls []scanCall
}

type scanCall struct {
	libraryID, rootID int64
	dirs              []string
}

func (f *fakeScanner) RequestPathScan(_ context.Context, libraryID, rootID int64, dirs []string) {
	f.calls = append(f.calls, scanCall{libraryID: libraryID, rootID: rootID, dirs: dirs})
}

type env struct {
	svc   *Service
	scan  *fakeScanner
	db    *store.DB
	app   *appq.Queries
	now   time.Time
	roots map[string]int64 // path → root id
}

func newEnv(t *testing.T) *env {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := store.Open(context.Background(), store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	e := &env{
		scan: &fakeScanner{}, db: db, app: appq.New(db.Write),
		now: time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC), roots: map[string]int64{},
	}
	e.svc = New(db, e.scan, log, WithClock(func() time.Time { return e.now }),
		WithBaseURL(func(context.Context) string { return "https://slouchcat.example.ch/api/v1" }))
	return e
}

// addLibrary creates a library with one root and remembers the root id.
func (e *env) addLibrary(t *testing.T, name, rootPath string) (libraryID, rootID int64) {
	t.Helper()
	ctx := context.Background()
	lib, err := e.app.InsertLibrary(ctx, appq.InsertLibraryParams{
		Name: name, Kind: "movies", DefaultRating: sql.NullInt64{Int64: 0, Valid: true}, Now: e.now.UnixMilli(),
	})
	if err != nil {
		t.Fatal(err)
	}
	root, err := e.app.InsertLibraryRoot(ctx, appq.InsertLibraryRootParams{
		LibraryID: lib.ID, Path: rootPath, FsType: "ext4", Watch: "inotify", Now: e.now.UnixMilli(),
	})
	if err != nil {
		t.Fatal(err)
	}
	e.roots[rootPath] = root.ID
	return lib.ID, root.ID
}

func TestCreateRevealsTheAddressOnceAndStoresOnlyTheHash(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	created, err := e.svc.Create(ctx, "Radarr", model.IntegrationRadarr)
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Token) != TokenChars {
		t.Fatalf("token %q has %d characters, want %d", created.Token, len(created.Token), TokenChars)
	}
	if want := "https://slouchcat.example.ch/api/v1/webhooks/" + created.Token; created.URL != want {
		t.Errorf("address %q, want %q", created.URL, want)
	}
	if created.TokenTail != created.Token[len(created.Token)-4:] {
		t.Errorf("tail %q does not match token %q", created.TokenTail, created.Token)
	}

	// Reading it again never carries the token or the address.
	read, err := e.svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Token != "" || read.URL != "" {
		t.Errorf("reading an integration revealed token %q and address %q", read.Token, read.URL)
	}
	if read.TokenTail != created.TokenTail {
		t.Errorf("tail %q, want %q", read.TokenTail, created.TokenTail)
	}

	// The database holds the hash, never the token.
	row, err := e.app.GetWebhookIntegration(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.TokenHash != HashToken(created.Token) {
		t.Errorf("stored hash %q, want the SHA-256 of the token", row.TokenHash)
	}
	if row.TokenHash == created.Token {
		t.Error("the token itself is stored")
	}
}

func TestCreateRejectsAnEmptyNameAnUnknownKindAndADuplicateName(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if _, err := e.svc.Create(ctx, "  ", model.IntegrationRadarr); !isValidation(t, err, "name") {
		t.Errorf("empty name: %v", err)
	}
	if _, err := e.svc.Create(ctx, "Sabnzbd", "sabnzbd"); !isValidation(t, err, "kind") {
		t.Errorf("unknown kind: %v", err)
	}
	if _, err := e.svc.Create(ctx, "Sonarr", model.IntegrationSonarr); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Create(ctx, "Sonarr", model.IntegrationSonarr); !isValidation(t, err, "name") {
		t.Errorf("duplicate name: %v", err)
	}
}

func TestAuthenticateFindsTheIntegrationAndRejectsEverythingElse(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	created, err := e.svc.Create(ctx, "Radarr", model.IntegrationRadarr)
	if err != nil {
		t.Fatal(err)
	}
	found, err := e.svc.Authenticate(ctx, created.Token)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != created.ID {
		t.Errorf("found integration %d, want %d", found.ID, created.ID)
	}
	other := created.Token[:len(created.Token)-1] + "x"
	if _, err := e.svc.Authenticate(ctx, other); err == nil {
		t.Error("a wrong token authenticated")
	}
	if _, err := e.svc.Authenticate(ctx, "short"); err == nil {
		t.Error("a token of the wrong length authenticated")
	}
}

func TestRenewTokenReplacesTheAddress(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	created, err := e.svc.Create(ctx, "Radarr", model.IntegrationRadarr)
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := e.svc.RenewToken(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if renewed.Token == created.Token {
		t.Fatal("the token did not change")
	}
	if _, err := e.svc.Authenticate(ctx, created.Token); err == nil {
		t.Error("the old token still works")
	}
	if _, err := e.svc.Authenticate(ctx, renewed.Token); err != nil {
		t.Errorf("the new token does not work: %v", err)
	}
	if _, err := e.svc.RenewToken(ctx, 999); err == nil {
		t.Error("renewing an unknown integration succeeded")
	}
}

func TestDeleteRemovesTheIntegrationAndItsLog(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	created, err := e.svc.Create(ctx, "Radarr", model.IntegrationRadarr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.app.InsertWebhookEvent(ctx, appq.InsertWebhookEventParams{
		IntegrationID: created.ID, Now: e.now.UnixMilli(), Event: "Test", Result: model.ResultAcknowledged,
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Get(ctx, created.ID); err == nil {
		t.Error("the integration is still there")
	}
	rows, err := e.app.ListWebhookEvents(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("%d log entries survived the delete", len(rows))
	}
	if err := e.svc.Delete(ctx, created.ID); err == nil {
		t.Error("deleting an unknown integration succeeded")
	}
}

func TestEventLogKeepsTheNewestTwentyEntries(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	created, err := e.svc.Create(ctx, "Radarr", model.IntegrationRadarr)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 25 {
		if _, err := e.app.InsertWebhookEvent(ctx, appq.InsertWebhookEventParams{
			IntegrationID: created.ID, Now: e.now.Add(time.Duration(i) * time.Second).UnixMilli(),
			Event: "Download", Result: model.ResultIgnored,
		}); err != nil {
			t.Fatal(err)
		}
	}
	events, err := e.svc.Events(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != MaxEvents {
		t.Fatalf("%d entries, want %d", len(events), MaxEvents)
	}
	// Newest first, and the newest is the 25th insert.
	if want := e.now.Add(24 * time.Second); !events[0].ReceivedAt.Equal(want) {
		t.Errorf("newest entry at %s, want %s", events[0].ReceivedAt, want)
	}
	if !events[len(events)-1].ReceivedAt.After(e.now.Add(3 * time.Second)) {
		t.Errorf("oldest kept entry at %s is too old", events[len(events)-1].ReceivedAt)
	}
}

func isValidation(t *testing.T, err error, field string) bool {
	t.Helper()
	var verr *model.ValidationError
	if !errors.As(err, &verr) {
		return false
	}
	for _, f := range verr.Fields {
		if f.Field == field {
			return true
		}
	}
	return false
}
