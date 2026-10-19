package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// testParams keep the tests fast; TestPHCDefaultParams covers the production parameters.
var testParams = argonParams{memoryKiB: 64, time: 1, threads: 1, saltLen: 16, keyLen: 32}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newService(t *testing.T) (*Service, *fakeClock) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := store.Open(context.Background(), store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	clock := &fakeClock{t: time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)}
	s := New(db, log).WithClock(clock.now)
	s.params = testParams
	return s, clock
}

const (
	testEmail    = "Marco@Example.CH"
	testPassword = "correct horse battery"
)

func TestOwnerLoginResolveLogout(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t)

	required, err := s.SetupRequired(ctx)
	if err != nil || !required {
		t.Fatalf("SetupRequired = %v, %v; want true", required, err)
	}
	acc, prof, err := s.CreateOwner(ctx, "  "+testEmail+" ", testPassword, "en")
	if err != nil {
		t.Fatal(err)
	}
	if acc.Email != "marco@example.ch" || acc.Role != "owner" {
		t.Fatalf("account = %+v", acc)
	}
	want := model.Profile{ID: prof.ID, AccountID: acc.ID, Name: "marco", MaxRating: 18, Language: "en", AllLibraries: true}
	if prof != want {
		t.Fatalf("profile = %+v, want %+v", prof, want)
	}
	if required, _ := s.SetupRequired(ctx); required {
		t.Fatal("SetupRequired after CreateOwner")
	}
	if _, _, err := s.CreateOwner(ctx, "other@example.ch", testPassword, "de"); !errors.Is(err, model.ErrSetupDone) {
		t.Fatalf("second owner: %v, want ErrSetupDone", err)
	}

	sess, token, err := s.Login(ctx, "MARCO@example.ch", testPassword, "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 43 || strings.ContainsAny(token, "+/=") {
		t.Fatalf("token %q is not 32 bytes base64url without padding", token)
	}
	if sess.Account != acc || sess.Profile != prof {
		t.Fatalf("login session = %+v", sess)
	}
	got, _, err := s.Resolve(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != sess.ID || got.Account != acc || got.Profile != prof || !got.IsOwner() {
		t.Fatalf("resolved = %+v, want %+v", got, sess)
	}
	if err := s.Logout(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Resolve(ctx, token); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("resolve after logout: %v, want ErrNotFound", err)
	}
	if err := s.Logout(ctx, sess.ID); err != nil {
		t.Fatalf("second logout: %v", err)
	}
	for _, bad := range []string{"", "nope", strings.Repeat("a", 4096)} {
		if _, _, err := s.Resolve(ctx, bad); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("Resolve(%.10q) = %v, want ErrNotFound", bad, err)
		}
	}
}

func TestTokenStoredOnlyAsHash(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t)
	acc, prof, err := s.CreateOwner(ctx, testEmail, testPassword, "")
	if err != nil {
		t.Fatal(err)
	}
	if prof.Language != "de" {
		t.Fatalf("default language = %q", prof.Language)
	}
	_, token, err := s.StartSession(ctx, acc, prof, "ua")
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := s.db.QueryRowContext(ctx, "SELECT token_hash FROM sessions").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == token || stored != hashToken(token) || len(stored) != 64 {
		t.Fatalf("stored token_hash %q is not the SHA-256 hex of the token", stored)
	}
}

func TestWrongPasswordAndUnknownEmail(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t)
	if _, _, err := s.CreateOwner(ctx, testEmail, testPassword, "de"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ email, password string }{
		{testEmail, "wrong password!"},
		{"nobody@example.ch", testPassword},
	} {
		_, token, err := s.Login(ctx, tc.email, tc.password, "")
		if !errors.Is(err, model.ErrInvalidCredentials) || token != "" {
			t.Errorf("Login(%s) = %q, %v; want ErrInvalidCredentials", tc.email, token, err)
		}
	}
	var verr *model.ValidationError
	if _, _, err := s.Login(ctx, " ", "", ""); !errors.As(err, &verr) || len(verr.Fields) != 2 {
		t.Fatalf("empty login: %v, want two required fields", err)
	}
}

func TestSessionExpiryAndSlidingTouch(t *testing.T) {
	ctx := context.Background()
	s, clock := newService(t)
	acc, prof, err := s.CreateOwner(ctx, testEmail, testPassword, "de")
	if err != nil {
		t.Fatal(err)
	}
	sess, token, err := s.StartSession(ctx, acc, prof, "")
	if err != nil {
		t.Fatal(err)
	}
	start := clock.now()
	if !sess.ExpiresAt.Equal(start.Add(SessionLifetime)) {
		t.Fatalf("expires = %v", sess.ExpiresAt)
	}

	// Within the touch interval nothing is written.
	clock.advance(time.Minute)
	got, extended, err := s.Resolve(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if extended || !got.ExpiresAt.Equal(start.Add(SessionLifetime)) {
		t.Fatalf("expiry moved within the touch interval: %v", got.ExpiresAt)
	}

	// After the interval the expiry slides.
	clock.advance(10 * time.Minute)
	got, extended, err = s.Resolve(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	slid := clock.now().Add(SessionLifetime)
	if !extended || !got.ExpiresAt.Equal(slid) {
		t.Fatalf("expires = %v, want %v", got.ExpiresAt, slid)
	}

	// Unused for longer than the lifetime: gone.
	clock.advance(SessionLifetime + time.Second)
	if _, _, err := s.Resolve(ctx, token); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("expired session resolved: %v", err)
	}
	n, err := s.DeleteExpiredSessions(ctx)
	if err != nil || n != 1 {
		t.Fatalf("DeleteExpiredSessions = %d, %v; want 1", n, err)
	}
}

func TestValidation(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t)
	cases := []struct {
		email, password, language string
		want                      []model.FieldError
	}{
		{"", "", "de", []model.FieldError{{Field: "email", Code: "required"}, {Field: "password", Code: "required"}}},
		{"no-at-sign", "abc12", "de", []model.FieldError{{Field: "email", Code: "invalid_email"}, {Field: "password", Code: "too_short"}}},
		{"a@b@c.ch", testPassword, "de", []model.FieldError{{Field: "email", Code: "invalid_email"}}},
		{"a@localhost", testPassword, "de", []model.FieldError{{Field: "email", Code: "invalid_email"}}},
		{"a@.ch", testPassword, "de", []model.FieldError{{Field: "email", Code: "invalid_email"}}},
		{strings.Repeat("a", 250) + "@b.ch", testPassword, "de", []model.FieldError{{Field: "email", Code: "invalid_email"}}},
		{"a@b.ch", strings.Repeat("x", 257), "de", []model.FieldError{{Field: "password", Code: "too_long"}}},
		{"a@b.ch", "ääääääääää", "fr", []model.FieldError{{Field: "language", Code: "invalid_value"}}},
	}
	for _, tc := range cases {
		_, _, err := s.CreateOwner(ctx, tc.email, tc.password, tc.language)
		var verr *model.ValidationError
		if !errors.As(err, &verr) {
			t.Errorf("CreateOwner(%.20q): %v, want ValidationError", tc.email, err)
			continue
		}
		if !slices.Equal(verr.Fields, tc.want) {
			t.Errorf("CreateOwner(%.20q) fields = %v, want %v", tc.email, verr.Fields, tc.want)
		}
		if strings.Contains(err.Error(), tc.password) && tc.password != "" {
			t.Errorf("validation error leaks the password: %v", err)
		}
	}
	if required, _ := s.SetupRequired(ctx); !required {
		t.Fatal("invalid input created an account")
	}
}

func TestPHCDefaultParams(t *testing.T) {
	encoded, err := hashPassword(testPassword, defaultParams)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("PHC string %q", encoded)
	}
	parts := strings.Split(encoded, "$")
	if len(parts[4]) != 22 || len(parts[5]) != 43 {
		t.Fatalf("salt/hash length in %q, want 16/32 bytes", encoded)
	}
	ok, err := verifyPassword(testPassword, encoded)
	if err != nil || !ok {
		t.Fatalf("verify = %v, %v", ok, err)
	}
	if ok, _ := verifyPassword(testPassword+"x", encoded); ok {
		t.Fatal("wrong password verified")
	}
	other, _ := hashPassword(testPassword, defaultParams)
	if other == encoded {
		t.Fatal("same password hashed twice gives the same string (salt not random)")
	}
}

func TestPHCParamsReadFromString(t *testing.T) {
	encoded, err := hashPassword(testPassword, testParams)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Fatalf("PHC string %q", encoded)
	}
	if ok, err := verifyPassword(testPassword, encoded); err != nil || !ok {
		t.Fatalf("verify with parameters from the string = %v, %v", ok, err)
	}
	for _, bad := range []string{
		"", "plain", "$argon2i$v=19$m=64,t=1,p=1$AAAA$AAAA", "$argon2id$v=16$m=64,t=1,p=1$AAAA$AAAA",
		"$argon2id$v=19$m=99999999,t=1,p=1$AAAA$AAAA", "$argon2id$v=19$m=64,t=1,p=1$!!$AAAA",
		"$argon2id$v=19$m=64,t=0,p=1$AAAA$AAAA",
	} {
		if _, err := verifyPassword(testPassword, bad); !errors.Is(err, errBadHash) {
			t.Errorf("verify(%q) = %v, want errBadHash", bad, err)
		}
	}
}

// Logout revokes the playback sessions of the browser session with it (plan 5.4), and only those.
func TestLogoutRevokesPlaybackSessions(t *testing.T) {
	ctx := context.Background()
	f := vistest.New(t)
	s := New(f.DB, vistest.Logger())
	mine, other := f.Session(t, f.P18), f.Session(t, f.P18)
	q := appq.New(f.DB.Write)
	for id, session := range map[string]int64{"01JAAAAAAAAAAAAAAAAAAAAAA1": mine.ID, "01JAAAAAAAAAAAAAAAAAAAAAA2": mine.ID, "01JAAAAAAAAAAAAAAAAAAAAAA3": other.ID} {
		err := q.InsertPlaybackSession(ctx, appq.InsertPlaybackSessionParams{
			ID: id, SessionID: session, AccountID: f.Account.ID, ProfileID: f.P18.ID, ItemID: f.Rated(18).ID,
			MediaFileID: f.Rated(18).FileID, Mode: "direct", Now: 1, ExpiresAt: time.Now().Add(time.Hour).UnixMilli(),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Logout(ctx, mine.ID); err != nil {
		t.Fatal(err)
	}
	var revoked, open int
	if err := f.DB.Read.QueryRowContext(ctx, `SELECT count(revoked_at), count(*) - count(revoked_at) FROM playback_sessions`).Scan(&revoked, &open); err != nil {
		t.Fatal(err)
	}
	if revoked != 2 || open != 1 {
		t.Errorf("after logout: %d revoked, %d open; want 2 and 1", revoked, open)
	}
}
