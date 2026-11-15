package auth

import (
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Profiles, accounts and invitations pick their ceiling from the steps of the rating country
// (concept 22 F-12, section 4.11 «Prüfung»): DE without the setting, the French steps with FR;
// a value a profile or account already has stays possible after the country changed, a new
// value outside the steps does not.
func TestCeilingsFollowTheRatingCountry(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	if rs, err := s.RatingSystem(ctx); err != nil || rs.Country != "DE" {
		t.Fatalf("rating system without the setting = %+v, %v; want DE", rs, err)
	}
	kid, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Kind"), MaxRating: num(12)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("x"), MaxRating: num(10)}); !slices.Equal(validationCodes(err), []string{"maxRating:invalid_value"}) {
		t.Errorf("10 in Germany: %v, want invalid_value", err)
	}
	guest := addGuestAccount(t, s, "oma@example.ch", 16, true, nil)

	if err := s.write.UpsertSetting(ctx, appq.UpsertSettingParams{Key: model.SettingRatingCountry, Value: "FR", Now: 1}); err != nil {
		t.Fatal(err)
	}
	if rs, err := s.RatingSystem(ctx); err != nil || rs.Country != "FR" || !rs.HasAge(10) || rs.HasAge(6) {
		t.Fatalf("rating system FR = %+v, %v", rs, err)
	}
	if _, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Ado"), MaxRating: num(10)}); err != nil {
		t.Errorf("10 in France: %v", err)
	}
	if _, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("y"), MaxRating: num(6)}); !slices.Equal(validationCodes(err), []string{"maxRating:invalid_value"}) {
		t.Errorf("6 in France: %v, want invalid_value", err)
	}
	// «Kind» keeps its 12 (a French step too) and may store it again; the account its 16.
	if _, err := s.UpdateProfile(ctx, sess, kid.ID, model.ProfileInput{MaxRating: num(12)}); err != nil {
		t.Errorf("keeping 12: %v", err)
	}
	if err := s.UpdateAccountAccess(ctx, guest.ID, model.AccountAccessUpdate{MaxRating: num(16)}); err != nil {
		t.Errorf("keeping the account's 16: %v", err)
	}
	// The US: 13 is a step; «Kind» keeps 12 although it is none, 6 is a US step again.
	if err := s.write.UpsertSetting(ctx, appq.UpsertSettingParams{Key: model.SettingRatingCountry, Value: "US", Now: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateProfile(ctx, sess, kid.ID, model.ProfileInput{MaxRating: num(12)}); err != nil {
		t.Errorf("keeping 12 in the US: %v", err)
	}
	if _, err := s.UpdateProfile(ctx, sess, kid.ID, model.ProfileInput{MaxRating: num(11)}); !slices.Equal(validationCodes(err), []string{"maxRating:invalid_value"}) {
		t.Errorf("11 in the US: %v, want invalid_value", err)
	}
	if p, err := s.UpdateProfile(ctx, sess, kid.ID, model.ProfileInput{MaxRating: num(13)}); err != nil || p.MaxRating != 13 {
		t.Errorf("13 in the US: %+v, %v", p, err)
	}
	if err := s.UpdateAccountAccess(ctx, guest.ID, model.AccountAccessUpdate{MaxRating: num(17)}); err != nil {
		t.Errorf("17 for an account in the US: %v", err)
	}
	if _, err := s.CreateInvitation(ctx, sess.Account, model.InvitationRequest{MaxRating: 13, AllLibraries: true}); err != nil {
		t.Errorf("an invitation with 13 in the US: %v", err)
	}
	if _, err := s.CreateInvitation(ctx, sess.Account, model.InvitationRequest{MaxRating: 12, AllLibraries: true}); !slices.Equal(validationCodes(err), []string{"maxRating:invalid_value"}) {
		t.Errorf("an invitation with 12 in the US: %v, want invalid_value", err)
	}
	// A setting nobody knows falls back to DE.
	if err := s.write.UpsertSetting(ctx, appq.UpsertSettingParams{Key: model.SettingRatingCountry, Value: "XX", Now: 3}); err != nil {
		t.Fatal(err)
	}
	if rs, err := s.RatingSystem(ctx); err != nil || rs.Country != "DE" {
		t.Errorf("unknown setting = %+v, %v; want DE", rs, err)
	}
}
