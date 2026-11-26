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

// Concept 25 (build step 1.6) with question 36 (ADR 0045): the request right on accounts, profiles
// and invitations. Every account and profile without an age limit starts with it, every one with an
// age limit has it never and cannot get it (canRequest age_restricted); a rating that crosses 18
// sets it and wins over a canRequest sent with it; a profile never gets it above its account (the
// cap); an administrator account takes only the right; an invitation hands right and limit to its
// account, and the hook runs whenever the right of an account changes.
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
	ageRestricted := func(what string, err error) {
		t.Helper()
		var verr *model.ValidationError
		if !errors.As(err, &verr) || !slices.ContainsFunc(verr.Fields, func(e model.FieldError) bool {
			return e.Field == "canRequest" && e.Code == model.WishReasonAgeRestricted
		}) {
			t.Errorf("%s: %v, want canRequest age_restricted", what, err)
		}
	}
	ownerSess, _ := owner(t, s)
	if !ownerSess.Account.CanRequest || !ownerSess.Profile.CanRequest {
		t.Errorf("owner = %v, primary profile = %v", ownerSess.Account.CanRequest, ownerSess.Profile.CanRequest)
	}
	// Without an age limit a further profile starts with the right (F-5 started it without).
	adult, err := s.CreateProfile(ctxOn(deviceA), ownerSess, model.ProfileInput{Name: new("Erwachsen")})
	if err != nil || !adult.CanRequest {
		t.Fatalf("further profile = %+v, %v", adult, err)
	}
	kid, err := s.CreateProfile(ctxOn(deviceA), ownerSess, model.ProfileInput{Name: new("Kind"), MaxRating: new(12)})
	if err != nil || kid.CanRequest {
		t.Fatalf("profile with an age limit = %+v, %v", kid, err)
	}
	_, err = s.CreateProfile(ctxOn(deviceA), ownerSess, model.ProfileInput{Name: new("Kind 2"), MaxRating: new(6), CanRequest: new(true)})
	ageRestricted("new profile with an age limit and the right", err)
	_, err = s.UpdateProfile(ctxOn(deviceA), ownerSess, kid.ID, model.ProfileInput{CanRequest: new(true)})
	ageRestricted("right for a profile with an age limit", err)
	_, err = s.UpdateProfile(ctxOn(deviceA), ownerSess, kid.ID, model.ProfileInput{MaxRating: new(16), CanRequest: new(true)})
	ageRestricted("right with a step below 18", err)
	if kid, err = s.UpdateProfile(ctxOn(deviceA), ownerSess, kid.ID, model.ProfileInput{CanRequest: new(false)}); err != nil || kid.CanRequest {
		t.Errorf("taking it away = %+v, %v", kid, err)
	}
	// The rating wins: lowering it with the old «on» of the phone app 1.0.3 takes the right, lifting
	// it with the old «off» gives it back.
	if adult, err = s.UpdateProfile(ctxOn(deviceA), ownerSess, adult.ID, model.ProfileInput{MaxRating: new(12), CanRequest: new(true)}); err != nil || adult.CanRequest {
		t.Errorf("lowered to 12 = %+v, %v", adult, err)
	}
	if adult, err = s.UpdateProfile(ctxOn(deviceA), ownerSess, adult.ID, model.ProfileInput{MaxRating: new(18), CanRequest: new(false)}); err != nil || !adult.CanRequest {
		t.Errorf("lifted to 18 = %+v, %v", adult, err)
	}
	// A right switched off by hand comes back after an age limit (ADR 0045, point 5).
	if adult, err = s.UpdateProfile(ctxOn(deviceA), ownerSess, adult.ID, model.ProfileInput{CanRequest: new(false)}); err != nil || adult.CanRequest {
		t.Errorf("switched off = %+v, %v", adult, err)
	}
	if adult, err = s.UpdateProfile(ctxOn(deviceA), ownerSess, adult.ID, model.ProfileInput{MaxRating: new(16)}); err != nil || adult.CanRequest {
		t.Errorf("limit after switching off = %+v, %v", adult, err)
	}
	if adult, err = s.UpdateProfile(ctxOn(deviceA), ownerSess, adult.ID, model.ProfileInput{MaxRating: new(18)}); err != nil || !adult.CanRequest {
		t.Errorf("limit lifted = %+v, %v", adult, err)
	}
	// The owner is an administrator: only the right, no access limits.
	if err := s.UpdateAccountAccess(ctxOn(deviceA), ownerSess.Account.ID, model.AccountAccessUpdate{CanRequest: new(false)}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateAccountAccess(ctxOn(deviceA), ownerSess.Account.ID, model.AccountAccessUpdate{CanRequest: new(true), MaxRating: new(12)}); !errors.Is(err, model.ErrAdminLimitsFixed) {
		t.Errorf("administrator with a limit: %v", err)
	}
	// An invitation with an age limit never carries the right.
	_, err = s.CreateInvitation(ctxOn(deviceA), ownerSess.Account, model.InvitationRequest{MaxRating: 12, AllLibraries: true, CanRequest: new(true)})
	ageRestricted("invitation with an age limit and the right", err)
	// An invitation without right and with an own limit.
	inv, err := s.CreateInvitation(ctxOn(deviceA), ownerSess.Account, model.InvitationRequest{MaxRating: 18, AllLibraries: true,
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
	// The account's rating: an age limit takes the right of the account and, through the lowered
	// profiles, of its profiles; lifting it gives the account's back, the profiles keep their step.
	rights := func() (int64, []int64) {
		t.Helper()
		acc, err := s.read.GetAccount(context.Background(), guest.Account.ID)
		if err != nil {
			t.Fatal(err)
		}
		profiles, err := s.read.ListProfilesByAccount(context.Background(), guest.Account.ID)
		if err != nil {
			t.Fatal(err)
		}
		var out []int64
		for _, p := range profiles {
			out = append(out, p.CanRequest)
		}
		return acc.CanRequest, out
	}
	if err := s.UpdateAccountAccess(ctxOn(deviceA), guest.Account.ID, model.AccountAccessUpdate{MaxRating: new(12), CanRequest: new(true)}); err != nil {
		t.Fatal(err)
	}
	if acc, profiles := rights(); acc != 0 || slices.Contains(profiles, 1) {
		t.Errorf("account at 12: account %d, profiles %v", acc, profiles)
	}
	ageRestricted("account with an age limit and the right",
		s.UpdateAccountAccess(ctxOn(deviceA), guest.Account.ID, model.AccountAccessUpdate{CanRequest: new(true)}))
	if err := s.UpdateAccountAccess(ctxOn(deviceA), guest.Account.ID, model.AccountAccessUpdate{MaxRating: new(18), CanRequest: new(false)}); err != nil {
		t.Fatal(err)
	}
	if acc, profiles := rights(); acc != 1 || slices.Contains(profiles, 1) {
		t.Errorf("account back at 18: account %d, profiles %v", acc, profiles)
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
	// Without canRequest the setting «Neue Konten dürfen anfordern» decides, on by default, and only
	// without an age limit.
	invite := func(rating int) bool {
		t.Helper()
		inv, err := s.CreateInvitation(ctxOn(deviceA), ownerSess.Account, model.InvitationRequest{MaxRating: rating, AllLibraries: true})
		if err != nil || inv.WishLimit.Mode != model.WishLimitDefault {
			t.Fatalf("invitation by the setting = %+v, %v", inv, err)
		}
		return inv.CanRequest
	}
	if !invite(18) || invite(12) {
		t.Error("default of «Neue Konten dürfen anfordern»: want on at 18, off at 12")
	}
	if err := s.write.UpsertSetting(context.Background(), appq.UpsertSettingParams{Key: settingNewAccountsCanRequest, Value: "0", Now: 1}); err != nil {
		t.Fatal(err)
	}
	if invite(18) {
		t.Error("«Neue Konten dürfen anfordern» off: the invitation still gives the right")
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(changed, []int64{ownerSess.Account.ID, guest.Account.ID, guest.Account.ID, guest.Account.ID}) {
		t.Errorf("hook = %v", changed)
	}
}
