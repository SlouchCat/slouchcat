package auth

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
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

// Concept 25 (build step 1.6): the request right on accounts, profiles and invitations. The owner
// and every primary profile start with it, further profiles without; a profile never gets it above
// its account (the cap); an administrator account takes only the right; an invitation hands right
// and limit to its account.
func TestCanRequest(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	var mu sync.Mutex
	var changed []int64
	s.WithRequestRightChanged(func(_ context.Context, id int64) {
		mu.Lock()
		defer mu.Unlock()
		changed = append(changed, id)
	})
	ownerSess, _ := owner(t, s)
	if !ownerSess.Account.CanRequest || !ownerSess.Profile.CanRequest {
		t.Errorf("owner = %v, primary profile = %v", ownerSess.Account.CanRequest, ownerSess.Profile.CanRequest)
	}
	kid, err := s.CreateProfile(ctxOn(deviceA), ownerSess, model.ProfileInput{Name: new("Kind")})
	if err != nil || kid.CanRequest {
		t.Fatalf("further profile = %+v, %v", kid, err)
	}
	if kid, err = s.UpdateProfile(ctxOn(deviceA), ownerSess, kid.ID, model.ProfileInput{CanRequest: new(true)}); err != nil || !kid.CanRequest {
		t.Errorf("right for the further profile = %+v, %v", kid, err)
	}
	// The owner is an administrator: only the right, no access limits.
	if err := s.UpdateAccountAccess(ctxOn(deviceA), ownerSess.Account.ID, model.AccountAccessUpdate{CanRequest: new(false)}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateAccountAccess(ctxOn(deviceA), ownerSess.Account.ID, model.AccountAccessUpdate{CanRequest: new(true), MaxRating: new(12)}); !errors.Is(err, model.ErrAdminLimitsFixed) {
		t.Errorf("administrator with a limit: %v", err)
	}
	// An invitation without right and with an own limit.
	inv, err := s.CreateInvitation(ctxOn(deviceA), ownerSess.Account, model.InvitationRequest{MaxRating: 12, AllLibraries: true,
		CanRequest: new(false), WishLimit: &model.InvitationWishLimit{Mode: model.WishLimitCustom,
			Movie: &model.WishLimit{Limit: 3, Days: 14}, TV: &model.WishLimit{Limit: 1, Days: 7}}})
	if err != nil || inv.CanRequest || inv.WishLimit.Mode != model.WishLimitCustom || inv.WishLimit.Movie.Limit != 3 {
		t.Fatalf("invitation = %+v, %v", inv, err)
	}
	guest, _, err := s.AcceptInvitation(fresh(), model.AcceptInvitation{Token: inv.Token, Email: "gast@example.ch", Password: "guest-password-1", ProfileName: "Gast"}, "ua")
	if err != nil {
		t.Fatal(err)
	}
	if guest.Account.CanRequest || !guest.Profile.CanRequest {
		t.Errorf("guest = %v, primary profile = %v", guest.Account.CanRequest, guest.Profile.CanRequest)
	}
	// The cap: no profile of the guest gets the right while the account has none.
	var verr *model.ValidationError
	if _, err := s.UpdateProfile(ctxOn(deviceA), guest, guest.Profile.ID, model.ProfileInput{CanRequest: new(true)}); !errors.As(err, &verr) || !hasField(verr, "canRequest") {
		t.Errorf("profile above its account: %v", err)
	}
	if _, err := s.CreateProfile(ctxOn(deviceA), guest, model.ProfileInput{Name: new("Zwei"), CanRequest: new(true)}); !errors.As(err, &verr) {
		t.Errorf("new profile above its account: %v", err)
	}
	if err := s.UpdateAccountAccess(ctxOn(deviceA), guest.Account.ID, model.AccountAccessUpdate{CanRequest: new(true)}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateAccountAccess(ctxOn(deviceA), guest.Account.ID, model.AccountAccessUpdate{CanRequest: new(true)}); err != nil {
		t.Fatal(err) // unchanged: no second hook call
	}
	if p, err := s.UpdateProfile(ctxOn(deviceA), guest, guest.Profile.ID, model.ProfileInput{CanRequest: new(true)}); err != nil || !p.CanRequest {
		t.Errorf("profile after the account got the right = %+v, %v", p, err)
	}
	// Invalid limits of an invitation.
	for _, wl := range []*model.InvitationWishLimit{
		{Mode: model.WishLimitCustom, Movie: &model.WishLimit{Limit: 101, Days: 7}, TV: &model.WishLimit{Limit: 1, Days: 7}},
		{Mode: model.WishLimitCustom, Movie: &model.WishLimit{Limit: 1, Days: 7}},
		{Mode: model.WishLimitUnlimited, Movie: &model.WishLimit{Limit: 1, Days: 7}},
		{Mode: "weekly"},
	} {
		if _, err := s.CreateInvitation(ctxOn(deviceA), ownerSess.Account, model.InvitationRequest{MaxRating: 12, AllLibraries: true, WishLimit: wl}); !errors.As(err, &verr) {
			t.Errorf("limit %+v: %v", wl, err)
		}
	}
	// Without canRequest the setting «Neue Konten dürfen anfordern» decides.
	if err := s.write.UpsertSetting(context.Background(), appq.UpsertSettingParams{Key: settingNewAccountsCanRequest, Value: "1", Now: 1}); err != nil {
		t.Fatal(err)
	}
	if inv, err := s.CreateInvitation(ctxOn(deviceA), ownerSess.Account, model.InvitationRequest{MaxRating: 12, AllLibraries: true}); err != nil || !inv.CanRequest || inv.WishLimit.Mode != model.WishLimitDefault {
		t.Errorf("invitation by the setting = %+v, %v", inv, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(changed, []int64{ownerSess.Account.ID, guest.Account.ID}) {
		t.Errorf("hook = %v", changed)
	}
}
