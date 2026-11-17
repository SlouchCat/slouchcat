package auth

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M8: a new account (owner or guest) runs the account hook after its commit, an accepted
// invitation tells the inviting account; a failing hook leaves the account in place.
func TestAccountHooks(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	var mu sync.Mutex
	var created []int64
	type accepted struct {
		inviter int64
		email   string
	}
	var invites []accepted
	s.WithAccountCreated(func(_ context.Context, id int64) error {
		mu.Lock()
		defer mu.Unlock()
		created = append(created, id)
		return errors.New("hook failure is only logged")
	}).WithInvitationAccepted(func(_ context.Context, inviter int64, email string) error {
		mu.Lock()
		defer mu.Unlock()
		invites = append(invites, accepted{inviter, email})
		return nil
	})
	ownerSess, _ := owner(t, s)
	inv, err := s.CreateInvitation(ctxOn(deviceA), ownerSess.Account, model.InvitationRequest{MaxRating: 12, AllLibraries: true})
	if err != nil {
		t.Fatal(err)
	}
	guest, _, err := s.AcceptInvitation(fresh(), model.AcceptInvitation{Token: inv.Token, Email: "Gast@Example.CH",
		Password: "guest-password-1", ProfileName: "Gast"}, "ua")
	if err != nil {
		t.Fatalf("accept with a failing hook: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(created, []int64{ownerSess.Account.ID, guest.Account.ID}) {
		t.Errorf("created = %v", created)
	}
	if len(invites) != 1 || invites[0].inviter != ownerSess.Account.ID || invites[0].email != "gast@example.ch" {
		t.Errorf("accepted = %+v", invites)
	}
}

// Concept 25 (architecture 18.3): the password hook runs after the commit of AcceptInvitation and
// UpdateAccount and after the check of Login, with the moment; never for a refused attempt, and
// the change is visible to the hook (committed).
func TestPasswordSeenHook(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	type seen struct {
		account        int64
		email, pw, mo  string
		committedEmail string
	}
	var mu sync.Mutex
	var got []seen
	s.WithPasswordSeen(func(ctx context.Context, id int64, email, pw, moment string) {
		acc, err := s.read.GetAccount(ctx, id)
		if err != nil {
			t.Errorf("hook before the commit: %v", err)
		}
		mu.Lock()
		defer mu.Unlock()
		got = append(got, seen{id, email, pw, moment, acc.Email})
	})
	ownerSess, _ := owner(t, s)
	inv, err := s.CreateInvitation(ctxOn(deviceA), ownerSess.Account, model.InvitationRequest{MaxRating: 12, AllLibraries: true})
	if err != nil {
		t.Fatal(err)
	}
	guest, _, err := s.AcceptInvitation(fresh(), model.AcceptInvitation{Token: inv.Token, Email: "Gast@Example.CH",
		Password: "guest-password-1", ProfileName: "Gast"}, "ua")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Login(fresh(), "gast@example.ch", "wrong-password-1", "ua"); err == nil {
		t.Fatal("wrong password accepted")
	}
	if _, _, err := s.Login(fresh(), "gast@example.ch", "guest-password-1", "ua"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateAccount(fresh(), guest, model.AccountUpdate{CurrentPassword: "wrong-password-1", NewPassword: new("guest-password-2")}); err == nil {
		t.Fatal("wrong current password accepted")
	}
	if _, err := s.UpdateAccount(fresh(), guest, model.AccountUpdate{CurrentPassword: "guest-password-1", NewPassword: new("guest-password-2")}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateAccount(fresh(), guest, model.AccountUpdate{CurrentPassword: "guest-password-2", Email: new("neu@example.ch")}); err != nil {
		t.Fatal(err)
	}
	id := guest.Account.ID
	want := []seen{
		{id, "gast@example.ch", "guest-password-1", PasswordMomentInvitation, "gast@example.ch"},
		{id, "gast@example.ch", "guest-password-1", PasswordMomentLogin, "gast@example.ch"},
		{id, "gast@example.ch", "guest-password-2", PasswordMomentPasswordChange, "gast@example.ch"},
		{id, "neu@example.ch", "guest-password-2", PasswordMomentEmailChange, "neu@example.ch"},
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(got, want) {
		t.Errorf("seen = %+v\nwant %+v", got, want)
	}
}
