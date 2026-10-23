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
