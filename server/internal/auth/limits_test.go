package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// More concurrent logins than argon2id slots all finish; the slots are free afterwards.
func TestConcurrentLoginsBeyondArgonSlots(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t)
	if _, _, err := s.CreateOwner(ctx, testEmail, testPassword, "de"); err != nil {
		t.Fatal(err)
	}
	const n = 4 * maxArgonRuns
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			// Distinct clients, so the attempt limit stays out of the way.
			ctx := model.WithClientIP(ctx, fmt.Sprintf("192.0.2.%d", i))
			_, _, errs[i] = s.Login(ctx, testEmail, testPassword, "")
		})
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("login %d: %v", i, err)
		}
	}
	if len(argonSlots) != 0 {
		t.Fatalf("%d argon2id slots still taken", len(argonSlots))
	}
}

// A login waiting for a slot gives up with its request context.
func TestLoginWaitsForArgonSlotWithContext(t *testing.T) {
	s, _ := newService(t)
	for range maxArgonRuns {
		argonSlots <- struct{}{}
	}
	t.Cleanup(func() {
		for range maxArgonRuns {
			<-argonSlots
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, _, err := s.Login(ctx, testEmail, testPassword, ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("login with all slots taken = %v, want DeadlineExceeded", err)
	}
	if _, _, err := s.CreateOwner(ctx, testEmail, testPassword, "de"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CreateOwner with all slots taken = %v, want DeadlineExceeded", err)
	}
}

// The 11th attempt of one client within a minute is refused; another client still gets through,
// and the bucket refills over time.
func TestLoginAttemptLimitPerClient(t *testing.T) {
	ctx := context.Background()
	s, clock := newService(t)
	if _, _, err := s.CreateOwner(ctx, testEmail, testPassword, "de"); err != nil {
		t.Fatal(err)
	}
	a := model.WithClientIP(ctx, "198.51.100.7")
	b := model.WithClientIP(ctx, "198.51.100.8")
	for i := range attemptBurst {
		if _, _, err := s.Login(a, testEmail, "wrong password!", ""); !errors.Is(err, model.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v, want ErrInvalidCredentials", i+1, err)
		}
	}
	runs := argonRuns.Load()
	if _, _, err := s.Login(a, testEmail, testPassword, ""); !errors.Is(err, model.ErrTooManyAttempts) {
		t.Fatalf("11th attempt: %v, want ErrTooManyAttempts", err)
	}
	if argonRuns.Load() != runs {
		t.Error("a refused attempt ran argon2id")
	}
	if _, _, err := s.Login(b, testEmail, testPassword, ""); err != nil {
		t.Fatalf("other client: %v", err)
	}
	clock.advance(6 * time.Second) // one token
	if _, _, err := s.Login(a, testEmail, testPassword, ""); err != nil {
		t.Fatalf("after refill: %v", err)
	}
	if _, _, err := s.Login(a, testEmail, testPassword, ""); !errors.Is(err, model.ErrTooManyAttempts) {
		t.Fatalf("second attempt after one refill: %v, want ErrTooManyAttempts", err)
	}
}

// Buckets idle for longer than bucketIdleAfter are dropped when a new client arrives.
func TestAttemptBucketsExpire(t *testing.T) {
	var l attemptLimiter
	start := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	for i := range 3 {
		if err := l.allow(fmt.Sprintf("203.0.113.%d", i), start); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.allow("203.0.113.9", start.Add(bucketIdleAfter+time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(l.buckets) != 1 {
		t.Fatalf("buckets = %d, want 1 after the sweep", len(l.buckets))
	}
}

// Over-long e-mail and password are refused before any argon2id run.
func TestLoginRejectsOverlongInputWithoutHashing(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t)
	runs := argonRuns.Load()
	start := time.Now()
	_, _, err := s.Login(ctx, strings.Repeat("a", 250)+"@b.ch", strings.Repeat("x", maxPasswordLen+1), "")
	var verr *model.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("login = %v, want ValidationError", err)
	}
	want := []model.FieldError{{Field: "email", Code: "too_long"}, {Field: "password", Code: "too_long"}}
	if fmt.Sprint(verr.Fields) != fmt.Sprint(want) {
		t.Fatalf("fields = %v, want %v", verr.Fields, want)
	}
	if argonRuns.Load() != runs {
		t.Fatal("over-long input ran argon2id")
	}
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Errorf("rejection took %v", d)
	}
	// Exactly at the limit is accepted (and verified).
	if _, _, err := s.Login(ctx, testEmail, strings.Repeat("x", maxPasswordLen), ""); !errors.Is(err, model.ErrInvalidCredentials) {
		t.Fatalf("password at the limit: %v, want ErrInvalidCredentials", err)
	}
}

// The bucket table is capped: addresses beyond maxBuckets push out the least recently used one,
// which then starts with a full bucket again; the cap holds however many addresses appear.
func TestAttemptLimiterCapsBuckets(t *testing.T) {
	var l attemptLimiter
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	for range attemptBurst {
		if err := l.allow("first", now); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.allow("first", now); !errors.Is(err, model.ErrTooManyAttempts) {
		t.Fatalf("first client not limited: %v", err)
	}
	for i := range maxBuckets + 500 {
		now = now.Add(time.Millisecond) // all within the idle window
		if err := l.allow(fmt.Sprintf("10.%d.%d.%d", i>>16, (i>>8)&255, i&255), now); err != nil {
			t.Fatal(err)
		}
		if len(l.buckets) > maxBuckets {
			t.Fatalf("%d buckets after %d clients, want at most %d", len(l.buckets), i+1, maxBuckets)
		}
	}
	if len(l.buckets) != maxBuckets {
		t.Errorf("%d buckets, want the cap %d", len(l.buckets), maxBuckets)
	}
	if _, ok := l.buckets["first"]; ok {
		t.Error("the least recently used bucket was not evicted")
	}
	if err := l.allow("first", now); err != nil {
		t.Errorf("evicted client starts over: %v", err)
	}
}
