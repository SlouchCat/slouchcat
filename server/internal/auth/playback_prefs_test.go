package auth

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The language and subtitle preferences (concept 13 11.4, F-13): every profile reads and replaces
// its own, the primary profile sets them for every profile through createProfile and
// updateProfile, and every value is checked (ISO 639-1 codes without duplicates, at most ten, one of
// the four subtitle rules).
func TestPlaybackPrefs(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	equal := func(a, b model.PlaybackPrefs) bool {
		return slices.Equal(a.AudioLanguages, b.AudioLanguages) && a.SubtitleMode == b.SubtitleMode &&
			slices.Equal(a.SubtitleLanguages, b.SubtitleLanguages)
	}
	none := model.PlaybackPrefs{AudioLanguages: []string{}, SubtitleMode: model.SubtitleModeForced, SubtitleLanguages: []string{}}

	// A profile created without preferences has the column defaults of migration 00032.
	child, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Kind")})
	if err != nil {
		t.Fatal(err)
	}
	if !equal(child.Prefs, none) {
		t.Errorf("defaults = %+v, want %+v", child.Prefs, none)
	}
	childSession := model.Session{ID: sess.ID, Account: sess.Account, Profile: child}
	if got, err := s.GetPlaybackPrefs(ctx, childSession); err != nil || !equal(got, none) {
		t.Errorf("get defaults = %+v, %v", got, err)
	}

	// The child profile replaces its own preferences; the primary profile sees them.
	want := model.PlaybackPrefs{AudioLanguages: []string{"en", "de"}, SubtitleMode: model.SubtitleModeForeign, SubtitleLanguages: []string{"de"}}
	if got, err := s.SetPlaybackPrefs(ctx, childSession, want); err != nil || !equal(got, want) {
		t.Fatalf("set = %+v, %v", got, err)
	}
	if got, err := s.GetPlaybackPrefs(ctx, childSession); err != nil || !equal(got, want) {
		t.Errorf("get after set = %+v, %v, want %+v", got, err, want)
	}
	list, err := s.ListProfiles(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	if i := slices.IndexFunc(list, func(p model.Profile) bool { return p.ID == child.ID }); i < 0 || !equal(list[i].Prefs, want) {
		t.Errorf("listProfiles = %+v", list)
	}
	// The primary profile's own preferences are untouched.
	if got, err := s.GetPlaybackPrefs(ctx, sess); err != nil || !equal(got, none) {
		t.Errorf("primary after the child's change = %+v, %v", got, err)
	}

	eleven := strings.Split("de en fr it es pt nl ja ko zh ru", " ")
	for name, tc := range map[string]struct {
		prefs model.PlaybackPrefs
		want  []string
	}{
		"all missing":   {model.PlaybackPrefs{}, []string{"audioLanguages:required", "subtitleMode:required", "subtitleLanguages:required"}},
		"unknown code":  {model.PlaybackPrefs{AudioLanguages: []string{"xx"}, SubtitleMode: "off", SubtitleLanguages: []string{}}, []string{"audioLanguages:invalid_value"}},
		"capitals":      {model.PlaybackPrefs{AudioLanguages: []string{"DE"}, SubtitleMode: "off", SubtitleLanguages: []string{}}, []string{"audioLanguages:invalid_value"}},
		"three letters": {model.PlaybackPrefs{AudioLanguages: []string{"ger"}, SubtitleMode: "off", SubtitleLanguages: []string{"gsw"}}, []string{"audioLanguages:invalid_value", "subtitleLanguages:invalid_value"}},
		"region":        {model.PlaybackPrefs{AudioLanguages: []string{}, SubtitleMode: "off", SubtitleLanguages: []string{"pt-br"}}, []string{"subtitleLanguages:invalid_value"}},
		"duplicate":     {model.PlaybackPrefs{AudioLanguages: []string{"de", "en", "de"}, SubtitleMode: "off", SubtitleLanguages: []string{}}, []string{"audioLanguages:invalid_value"}},
		"eleven":        {model.PlaybackPrefs{AudioLanguages: []string{}, SubtitleMode: "off", SubtitleLanguages: eleven}, []string{"subtitleLanguages:too_long"}},
		"mode":          {model.PlaybackPrefs{AudioLanguages: []string{}, SubtitleMode: "sometimes", SubtitleLanguages: []string{}}, []string{"subtitleMode:invalid_value"}},
	} {
		_, err := s.SetPlaybackPrefs(ctx, childSession, tc.prefs)
		if got := validationCodes(err); !slices.Equal(got, tc.want) {
			t.Errorf("%s: %v (%v), want %v", name, got, err, tc.want)
		}
	}
	if got, err := s.GetPlaybackPrefs(ctx, childSession); err != nil || !equal(got, want) {
		t.Errorf("a refused change changed the preferences: %+v, %v", got, err)
	}
	// Ten languages and every rule are fine; an empty list means none.
	for _, mode := range subtitleModes {
		p := model.PlaybackPrefs{AudioLanguages: eleven[:10], SubtitleMode: mode, SubtitleLanguages: []string{}}
		if got, err := s.SetPlaybackPrefs(ctx, childSession, p); err != nil || !equal(got, p) {
			t.Errorf("mode %s: %+v, %v", mode, got, err)
		}
	}

	// The primary profile sets the fields of any profile; fields left out keep their value.
	upd, err := s.UpdateProfile(ctx, sess, child.ID, model.ProfileInput{SubtitleMode: str(model.SubtitleModeAlways), AudioLanguages: &[]string{"ja"}})
	if err != nil {
		t.Fatal(err)
	}
	wantUpd := model.PlaybackPrefs{AudioLanguages: []string{"ja"}, SubtitleMode: model.SubtitleModeAlways, SubtitleLanguages: []string{}}
	if !equal(upd.Prefs, wantUpd) {
		t.Errorf("update = %+v, want %+v", upd.Prefs, wantUpd)
	}
	if upd, err = s.UpdateProfile(ctx, sess, child.ID, model.ProfileInput{Name: str("Kind 2")}); err != nil || !equal(upd.Prefs, wantUpd) {
		t.Errorf("an update without the fields = %+v, %v", upd.Prefs, err)
	}
	if got, err := s.GetPlaybackPrefs(ctx, childSession); err != nil || !equal(got, wantUpd) {
		t.Errorf("stored after update = %+v, %v", got, err)
	}
	_, err = s.UpdateProfile(ctx, sess, child.ID, model.ProfileInput{SubtitleLanguages: &[]string{"klingon"}, SubtitleMode: str("never")})
	if got := validationCodes(err); !slices.Equal(got, []string{"subtitleMode:invalid_value", "subtitleLanguages:invalid_value"}) {
		t.Errorf("update with bad values: %v (%v)", got, err)
	}
	// Only the primary profile changes other profiles.
	if _, err := s.UpdateProfile(ctx, childSession, child.ID, model.ProfileInput{SubtitleMode: str("off")}); !errors.Is(err, model.ErrPrimaryProfileRequired) {
		t.Errorf("update by the child: %v, want primary_profile_required", err)
	}

	// createProfile takes the fields right away.
	teen, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Teen"), SubtitleMode: str(model.SubtitleModeOff), SubtitleLanguages: &[]string{"fr", "en"}})
	if err != nil {
		t.Fatal(err)
	}
	wantTeen := model.PlaybackPrefs{AudioLanguages: []string{}, SubtitleMode: model.SubtitleModeOff, SubtitleLanguages: []string{"fr", "en"}}
	if !equal(teen.Prefs, wantTeen) {
		t.Errorf("create = %+v, want %+v", teen.Prefs, wantTeen)
	}
	if got, err := s.GetPlaybackPrefs(ctx, model.Session{Account: sess.Account, Profile: teen}); err != nil || !equal(got, wantTeen) {
		t.Errorf("stored after create = %+v, %v", got, err)
	}
	if _, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Bad"), AudioLanguages: &[]string{"de", "de"}}); !slices.Equal(validationCodes(err), []string{"audioLanguages:invalid_value"}) {
		t.Errorf("create with a duplicate: %v", err)
	}

	// A session without a profile, or with a profile of another account, has no preferences.
	if _, err := s.GetPlaybackPrefs(ctx, model.Session{Account: sess.Account}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("without a profile: %v", err)
	}
	other := sess.Account
	other.ID++
	if _, err := s.SetPlaybackPrefs(ctx, model.Session{Account: other, Profile: child}, want); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("profile of another account: %v", err)
	}
	if err := s.ResetSeriesTracks(ctx, model.Session{Account: sess.Account}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("reset without a profile: %v", err)
	}
	if err := s.ResetSeriesTracks(ctx, childSession); err != nil {
		t.Errorf("reset without entries: %v", err)
	}
}
