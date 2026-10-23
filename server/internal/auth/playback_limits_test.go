package auth

import (
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// W-8 and K-5 limits of profiles and guest accounts (M4 design 2.1): a profile request replaces
// all four limits at once, a guest update the three caps; the effective limits of a guest profile
// are capped by its account, everywhere a profile is read.
func TestPlaybackLimits(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	ownerSess, _ := owner(t, s)
	ctx := ctxOn(deviceA)

	kid, err := s.CreateProfile(ctx, ownerSess, model.ProfileInput{Name: str("Kind"),
		Limits: &model.PlaybackLimits{MaxBitrateKbps: 4000, MaxHeight: 720, MaxStreams: 1, WatchLimitMinutes: 30}})
	if err != nil {
		t.Fatal(err)
	}
	want := model.PlaybackLimits{MaxBitrateKbps: 4000, MaxHeight: 720, MaxStreams: 1, WatchLimitMinutes: 30}
	if kid.Limits != want || kid.Effective != want {
		t.Fatalf("created = %+v / %+v", kid.Limits, kid.Effective)
	}
	// An update without limits keeps them; limits replace all four (a field left out clears it).
	kid, err = s.UpdateProfile(ctx, ownerSess, kid.ID, model.ProfileInput{Name: str("Kind 2")})
	if err != nil || kid.Limits != want {
		t.Fatalf("update without limits = %+v, %v", kid.Limits, err)
	}
	kid, err = s.UpdateProfile(ctx, ownerSess, kid.ID, model.ProfileInput{Limits: &model.PlaybackLimits{MaxHeight: 480}})
	if err != nil || kid.Limits != (model.PlaybackLimits{MaxHeight: 480}) {
		t.Fatalf("replace limits = %+v, %v", kid.Limits, err)
	}
	list, err := s.ListProfiles(ctx, ownerSess)
	if err != nil || len(list) != 2 || list[1].Limits != (model.PlaybackLimits{MaxHeight: 480}) || list[0].Limits != (model.PlaybackLimits{}) {
		t.Fatalf("list = %+v, %v", list, err)
	}

	for _, tc := range []struct {
		limits model.PlaybackLimits
		codes  []string
	}{
		{model.PlaybackLimits{MaxBitrateKbps: 199}, []string{"limits.maxBitrateKbps:too_small"}},
		{model.PlaybackLimits{MaxBitrateKbps: 200_001}, []string{"limits.maxBitrateKbps:too_large"}},
		{model.PlaybackLimits{MaxHeight: 1000}, []string{"limits.maxHeight:invalid_value"}},
		{model.PlaybackLimits{MaxStreams: 17}, []string{"limits.maxStreams:too_large"}},
		{model.PlaybackLimits{WatchLimitMinutes: 1441}, []string{"limits.watchLimitMinutes:too_large"}},
		{model.PlaybackLimits{MaxStreams: -1, WatchLimitMinutes: -1}, []string{"limits.maxStreams:too_small", "limits.watchLimitMinutes:too_small"}},
	} {
		_, err := s.UpdateProfile(ctx, ownerSess, kid.ID, model.ProfileInput{Limits: &tc.limits})
		if got := validationCodes(err); !slices.Equal(got, tc.codes) {
			t.Errorf("limits %+v: codes %v, want %v", tc.limits, got, tc.codes)
		}
	}

	// A guest account caps every profile; the profile keeps its own values.
	inv, err := s.CreateInvitation(ctx, ownerSess.Account, model.InvitationRequest{MaxRating: 12, AllLibraries: true})
	if err != nil {
		t.Fatal(err)
	}
	guest, token, err := s.AcceptInvitation(fresh(), model.AcceptInvitation{Token: inv.Token, Email: "gast@example.ch", Password: "gast-password-1", ProfileName: "Gast"}, "ua")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := s.UpdateGuest(ctx, guest.Account.ID, model.GuestUpdate{Limits: &model.PlaybackLimits{MaxBitrateKbps: 8000, MaxHeight: 1080, MaxStreams: 2, WatchLimitMinutes: 60}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Limits != (model.PlaybackLimits{MaxBitrateKbps: 8000, MaxHeight: 1080, MaxStreams: 2}) {
		t.Fatalf("guest caps = %+v (the watch-time limit is a profile setting)", updated.Limits)
	}
	if _, err := s.UpdateGuest(ctx, guest.Account.ID, model.GuestUpdate{Limits: &model.PlaybackLimits{MaxHeight: 999}}); !slices.Equal(validationCodes(err), []string{"limits.maxHeight:invalid_value"}) {
		t.Errorf("invalid guest cap: %v", err)
	}
	guests, err := s.ListGuests(ctx)
	if err != nil || len(guests) != 1 || guests[0].Limits != updated.Limits {
		t.Fatalf("list guests = %+v, %v", guests, err)
	}
	resolved, _, err := s.Resolve(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Account.Limits != updated.Limits {
		t.Errorf("resolved account caps = %+v", resolved.Account.Limits)
	}
	// The guest's primary profile sets limits above and below the caps; the effective limits take
	// the stricter value per field.
	g2, err := s.UpdateProfile(ctx, resolved, resolved.Profile.ID, model.ProfileInput{
		Limits: &model.PlaybackLimits{MaxBitrateKbps: 20000, MaxHeight: 720, WatchLimitMinutes: 45},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := (model.PlaybackLimits{MaxBitrateKbps: 8000, MaxHeight: 720, MaxStreams: 2, WatchLimitMinutes: 45}); g2.Effective != want {
		t.Errorf("effective = %+v, want %+v", g2.Effective, want)
	}
	resolved, _, err = s.Resolve(ctx, token)
	if err != nil || resolved.Profile.Effective != g2.Effective || resolved.Profile.Limits.MaxBitrateKbps != 20000 {
		t.Errorf("resolved profile = %+v / %+v, %v", resolved.Profile.Limits, resolved.Profile.Effective, err)
	}
	mine, err := s.ListMyProfiles(ctx, guest.Account.ID)
	if err != nil || len(mine) != 1 || mine[0].Effective != g2.Effective {
		t.Errorf("picker profiles = %+v, %v", mine, err)
	}
}
