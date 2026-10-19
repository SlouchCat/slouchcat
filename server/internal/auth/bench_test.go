package auth

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
)

// Benchmarks of the access path with the production argon2id parameters (docs/perf/M2.md):
//
//	go test ./internal/auth -run '^$' -bench . -benchtime 20x -count 5
//
// The clock advances by a minute per iteration, so the per-client attempt limit never interferes.
func benchService(b *testing.B) (*Service, model.Session, model.Profile) {
	b.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := store.Open(context.Background(), store.Options{ConfigDir: b.TempDir(), Version: "bench", Logger: log})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Close() })
	clock := &fakeClock{t: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)}
	s := New(db, log).WithClock(func() time.Time { clock.advance(time.Minute); return clock.now() })
	acc, prof, err := s.CreateOwner(context.Background(), testEmail, testPassword, "de")
	if err != nil {
		b.Fatal(err)
	}
	sess, _, err := s.StartSession(ctxOn(deviceA), acc, prof, "bench")
	if err != nil {
		b.Fatal(err)
	}
	kid, err := s.CreateProfile(ctxOn(deviceA), sess, model.ProfileInput{Name: str("Kind"), PIN: str("4711")})
	if err != nil {
		b.Fatal(err)
	}
	return s, sess, kid
}

// BenchmarkPINVerify is one argon2id check of a PIN (the cost of a selectProfile with PIN, without
// database access).
func BenchmarkPINVerify(b *testing.B) {
	hash, err := hashPassword("4711", defaultParams)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		if ok, err := verifyPassword("4711", hash); err != nil || !ok {
			b.Fatalf("verify: %v %v", ok, err)
		}
	}
}

func BenchmarkSelectProfileWithPIN(b *testing.B) {
	s, sess, kid := benchService(b)
	ctx := ctxOn(deviceA)
	b.ResetTimer()
	for range b.N {
		if _, err := s.SelectProfile(ctx, sess, kid.ID, "4711"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelectProfileWithoutPIN(b *testing.B) {
	s, sess, _ := benchService(b)
	ctx := ctxOn(deviceA)
	b.ResetTimer()
	for range b.N {
		if _, err := s.SelectProfile(ctx, sess, sess.Profile.ID, ""); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLogin(b *testing.B) {
	s, _, _ := benchService(b)
	ctx := ctxOn(deviceA)
	b.ResetTimer()
	for range b.N {
		if _, _, err := s.Login(ctx, testEmail, testPassword, "bench"); err != nil {
			b.Fatal(err)
		}
	}
}
