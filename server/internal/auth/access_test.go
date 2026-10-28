package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

const (
	deviceA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	deviceB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

var clientCounter atomic.Int64

// fresh is a request context from a new client address, so tests with many public requests stay
// below the per-client attempt limit.
func fresh() context.Context {
	n := clientCounter.Add(1)
	return model.WithClientIP(model.WithDeviceID(context.Background(), deviceA), fmt.Sprintf("198.51.%d.%d", n/250, n%250))
}

func ctxOn(device string) context.Context {
	return model.WithClientIP(model.WithDeviceID(context.Background(), device), "192.0.2.1")
}

// owner creates the owner and returns it with the primary profile and a session on it.
func owner(t *testing.T, s *Service) (model.Session, string) {
	t.Helper()
	acc, prof, err := s.CreateOwner(context.Background(), testEmail, testPassword, "de")
	if err != nil {
		t.Fatal(err)
	}
	sess, token, err := s.StartSession(ctxOn(deviceA), acc, prof, "ua")
	if err != nil {
		t.Fatal(err)
	}
	return sess, token
}

func addLibrary(t *testing.T, s *Service, name string) int64 {
	t.Helper()
	lib, err := s.write.InsertLibrary(context.Background(), appq.InsertLibraryParams{Name: name, Kind: "movies", Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	return lib.ID
}

func str(v string) *string { return &v }
func num(v int) *int       { return &v }

func validationCodes(err error) []string {
	var verr *model.ValidationError
	if !errors.As(err, &verr) {
		return nil
	}
	out := make([]string, 0, len(verr.Fields))
	for _, f := range verr.Fields {
		out = append(out, f.Field+":"+f.Code)
	}
	return out
}

func TestLockDurationCurve(t *testing.T) {
	t.Parallel()
	for failures, want := range map[int64]time.Duration{
		0: 0, 1: 0, 2: 0, 3: 30 * time.Second, 4: time.Minute, 5: 2 * time.Minute, 6: 4 * time.Minute,
		7: 8 * time.Minute, 8: 15 * time.Minute, 9: 15 * time.Minute, 200: 15 * time.Minute,
	} {
		if got := lockDuration(failures); got != want {
			t.Errorf("lockDuration(%d) = %v, want %v", failures, got, want)
		}
	}
}

func TestPINValidation(t *testing.T) {
	t.Parallel()
	for pin, want := range map[string]bool{"1234": true, "12345678": true, "123": false, "123456789": false, "12a4": false, "": false, " 1234": false, "１２３４": false} {
		if ValidPIN(pin) != want {
			t.Errorf("ValidPIN(%q) = %v, want %v", pin, !want, want)
		}
	}
}

// AN-11 with a clock mock: from the third wrong PIN on the profile is locked for 30 s, 60 s, 120 s
// … up to 15 min per profile and device; a right PIN resets; another device is unaffected; the
// lock survives a new service on the same database (restart).
func TestPINLockoutCurve(t *testing.T) {
	t.Parallel()
	s, clock := newService(t)
	sess, _ := owner(t, s)
	ctx := context.Background()
	kid, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Kind"), MaxRating: num(6), PIN: str("4711")})
	if err != nil {
		t.Fatal(err)
	}
	if !kid.HasPin {
		t.Fatal("profile created with a PIN reports hasPin false")
	}
	var hash string
	if err := s.db.QueryRow(`SELECT pin_hash FROM profiles WHERE id = ?`, kid.ID).Scan(&hash); err != nil || !strings.HasPrefix(hash, "$argon2id$v=19$") || strings.Contains(hash, "4711") {
		t.Fatalf("stored PIN hash = %q (%v)", hash, err)
	}

	// The same account on a second device: its own session, started with the cookie of that device.
	sessB, _, err := s.StartSession(ctxOn(deviceB), sess.Account, sess.Profile, "ua")
	if err != nil {
		t.Fatal(err)
	}
	if sess.DeviceID != deviceA || sessB.DeviceID != deviceB {
		t.Fatalf("session devices = %q, %q", sess.DeviceID, sessB.DeviceID)
	}
	wrong := func(device string) error {
		session := sess
		if device == deviceB {
			session = sessB
		}
		_, err := s.SelectProfile(ctxOn(device), session, kid.ID, "0000")
		return err
	}
	lockedFor := func(err error) time.Duration {
		t.Helper()
		var l *model.PinLockedError
		if !errors.Is(err, model.ErrPinLocked) || !errors.As(err, &l) {
			t.Fatalf("err = %v, want a PinLockedError", err)
		}
		return l.RetryAfter
	}

	// Missing PIN and a malformed one do not count.
	if _, err := s.SelectProfile(ctxOn(deviceA), sess, kid.ID, ""); !errors.Is(err, model.ErrPinRequired) {
		t.Fatalf("no PIN: %v", err)
	}
	if _, err := s.SelectProfile(ctxOn(deviceA), sess, kid.ID, "12"); !slices.Equal(validationCodes(err), []string{"pin:invalid_pin"}) {
		t.Fatalf("malformed PIN: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if err := wrong(deviceA); !errors.Is(err, model.ErrPinInvalid) {
			t.Fatalf("failure %d: %v, want ErrPinInvalid", i, err)
		}
	}
	for i, want := range []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 15 * time.Minute, 15 * time.Minute} {
		if got := lockedFor(wrong(deviceA)); got != want {
			t.Fatalf("failure %d locks for %v, want %v", i+3, got, want)
		}
		// Locked: the right PIN is refused, with the remaining time shrinking.
		clock.advance(want - 10*time.Second)
		_, err := s.SelectProfile(ctxOn(deviceA), sess, kid.ID, "4711")
		if got := lockedFor(err); got != 10*time.Second {
			t.Fatalf("remaining lock = %v, want 10s", got)
		}
		// Another device is not locked.
		if i == 0 {
			if got, err := s.SelectProfile(ctxOn(deviceB), sessB, kid.ID, "4711"); err != nil || got.Profile.ID != kid.ID {
				t.Fatalf("other device: %v", err)
			}
			if err := wrong(deviceB); !errors.Is(err, model.ErrPinInvalid) {
				t.Fatalf("other device counts separately: %v", err)
			}
		}
		clock.advance(10 * time.Second)
	}

	// The lock is in the database: a new service (restart) sees it.
	if err := wrong(deviceA); err == nil {
		t.Fatal("expected another failure")
	}
	restarted := &Service{db: s.db, write: s.write, read: s.read, log: s.log, now: clock.now, params: testParams}
	if _, err := restarted.SelectProfile(ctxOn(deviceA), sess, kid.ID, "4711"); !errors.Is(err, model.ErrPinLocked) {
		t.Errorf("after a restart: %v, want the lock to persist", err)
	}

	// A right PIN after the lock resets the counter: the next wrong PIN starts again at 1.
	clock.advance(16 * time.Minute)
	if got, err := s.SelectProfile(ctxOn(deviceA), sess, kid.ID, "4711"); err != nil || got.Profile.ID != kid.ID {
		t.Fatalf("right PIN after the lock: %v", err)
	}
	if err := wrong(deviceA); !errors.Is(err, model.ErrPinInvalid) {
		t.Errorf("first failure after the reset: %v, want ErrPinInvalid", err)
	}

	// Setting or removing the PIN clears the counters.
	for i := 0; i < 2; i++ {
		_ = wrong(deviceA) // the 2nd failure since the reset is the 3rd in a row → locked
	}
	if err := s.SetProfilePIN(ctx, sess, kid.ID, "9876"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SelectProfile(ctxOn(deviceA), sess, kid.ID, "9876"); err != nil {
		t.Errorf("new PIN right after the change: %v", err)
	}
	if err := s.RemoveProfilePIN(ctx, sess, kid.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := s.SelectProfile(ctxOn(deviceA), sess, kid.ID, ""); err != nil || got.Profile.HasPin {
		t.Errorf("select without PIN after removing it: %v", err)
	}
}

func TestSelectProfileWithoutPINAndOtherAccounts(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, token := owner(t, s)
	ctx := ctxOn(deviceA)
	kid, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Kind"), MaxRating: num(6)})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.SelectProfile(ctx, sess, kid.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != sess.ID || got.Profile.ID != kid.ID || got.Profile.IsPrimary || got.Profile.MaxRating != 6 {
		t.Fatalf("selected = %+v", got.Profile)
	}
	// The same session now resolves with the new profile (in place, no new token).
	resolved, _, err := s.Resolve(ctx, token)
	if err != nil || resolved.Profile.ID != kid.ID {
		t.Fatalf("resolve after the switch: %+v, %v", resolved.Profile, err)
	}
	// The device of the request is remembered with the session.
	var device *string
	if err := s.db.QueryRow(`SELECT device_id FROM sessions WHERE id = ?`, sess.ID).Scan(&device); err != nil || device == nil || *device != deviceA {
		t.Errorf("session device = %v (%v)", device, err)
	}

	// A profile of another account is not found, as is an unknown id.
	other := addGuestAccount(t, s, "other@example.ch", 18, true, nil)
	foreign, err := s.write.InsertProfile(context.Background(), appq.InsertProfileParams{AccountID: other.ID, Name: "Fremd", MaxRating: 18, Language: "de", AllLibraries: 1, IsDefault: 1, Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{foreign.ID, 9999} {
		if _, err := s.SelectProfile(ctx, sess, id, ""); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("select %d: %v, want ErrNotFound", id, err)
		}
	}
	// A revoked session cannot select.
	if err := s.Logout(ctx, sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SelectProfile(ctx, sess, kid.ID, ""); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("select on a revoked session: %v", err)
	}
}

func addGuestAccount(t *testing.T, s *Service, email string, maxRating int64, all bool, libs []int64) appq.Account {
	t.Helper()
	acc, err := s.write.InsertInvitedAccount(context.Background(), appq.InsertInvitedAccountParams{Email: email, PasswordHash: "x", MaxRating: maxRating, AllLibraries: boolInt(all), Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range libs {
		if err := s.write.InsertAccountLibrary(context.Background(), appq.InsertAccountLibraryParams{AccountID: acc.ID, LibraryID: id}); err != nil {
			t.Fatal(err)
		}
	}
	return acc
}

// Login starts the session with the profile only when the account has exactly one without PIN.
func TestLoginProfileChoice(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := ctxOn(deviceA)

	login := func() model.Session {
		t.Helper()
		got, _, err := s.Login(fresh(), testEmail, testPassword, "ua")
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := login(); !got.HasProfile() || got.Profile.ID != sess.Profile.ID || !got.Profile.IsPrimary {
		t.Fatalf("one profile without PIN: %+v", got.Profile)
	}
	if err := s.SetProfilePIN(ctx, sess, sess.Profile.ID, "1234"); err != nil {
		t.Fatal(err)
	}
	if got := login(); got.HasProfile() || got.Account.Role != model.RoleAdmin || !got.Account.IsOwner || got.Account.MaxRating != 18 || !got.Account.AllLibraries {
		t.Fatalf("one profile with PIN: %+v", got)
	}
	if err := s.RemoveProfilePIN(ctx, sess, sess.Profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Zweit")}); err != nil {
		t.Fatal(err)
	}
	got := login()
	if got.HasProfile() {
		t.Fatalf("two profiles: login chose %+v", got.Profile)
	}
	// A session without profile resolves without one and may list the profiles for the picker.
	resolved, _, err := s.Resolve(ctx, mustToken(t, s, got))
	if err != nil || resolved.HasProfile() {
		t.Fatalf("resolve without profile: %+v, %v", resolved.Profile, err)
	}
	list, err := s.ListMyProfiles(ctx, resolved.Account.ID)
	if err != nil || len(list) != 2 || !list[0].IsPrimary || list[1].Name != "Zweit" {
		t.Fatalf("picker list = %+v, %v", list, err)
	}
	// Profile management needs the primary profile active.
	if _, err := s.ListProfiles(ctx, resolved); !errors.Is(err, model.ErrPrimaryProfileRequired) {
		t.Errorf("ListProfiles without profile: %v", err)
	}
}

func mustToken(t *testing.T, s *Service, session model.Session) string {
	t.Helper()
	// A session of this test was started by Login; fetch its token by starting the same account again
	// is not possible (tokens are stored hashed), so make a fresh one on the same account and profile.
	_, token, err := s.StartSession(ctxOn(deviceA), session.Account, session.Profile, "ua")
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestProfileRules(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, token := owner(t, s)
	ctx := ctxOn(deviceA)
	libA, libB := addLibrary(t, s, "A"), addLibrary(t, s, "B")

	// Defaults: maxRating 18 and the language of the primary profile; all libraries.
	p, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("  Kind  ")})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Kind" || p.MaxRating != 18 || p.Language != "de" || !p.AllLibraries || p.IsPrimary || p.HasPin || p.Accent != "" {
		t.Fatalf("defaults = %+v", p)
	}
	// Validation.
	for name, tc := range map[string]struct {
		in   model.ProfileInput
		want []string
	}{
		"no name":        {model.ProfileInput{}, []string{"name:required"}},
		"blank":          {model.ProfileInput{Name: str("  ")}, []string{"name:required"}},
		"long":           {model.ProfileInput{Name: str(strings.Repeat("ä", 61))}, []string{"name:too_long"}},
		"rating":         {model.ProfileInput{Name: str("x"), MaxRating: num(7)}, []string{"maxRating:invalid_value"}},
		"language":       {model.ProfileInput{Name: str("x"), Language: str("fr")}, []string{"language:invalid_value"}},
		"accent":         {model.ProfileInput{Name: str("x"), Accent: str("pink")}, []string{"accent:invalid_value"}},
		"pin":            {model.ProfileInput{Name: str("x"), PIN: str("12")}, []string{"pin:invalid_pin"}},
		"unknown lib":    {model.ProfileInput{Name: str("x"), AllLibraries: boolPtr(false), LibraryIDs: &[]int64{libA, 999}}, []string{"libraryIds:unknown_library"}},
		"several":        {model.ProfileInput{Name: str(""), Accent: str("nope")}, []string{"name:required", "accent:invalid_value"}},
		"ignored if all": {model.ProfileInput{Name: str("ok"), LibraryIDs: &[]int64{999}}, nil},
	} {
		_, err := s.CreateProfile(ctx, sess, tc.in)
		if got := validationCodes(err); !slices.Equal(got, tc.want) {
			t.Errorf("%s: %v (%v), want %v", name, got, err, tc.want)
		}
	}
	for _, accent := range Accents {
		if _, err := s.UpdateProfile(ctx, sess, p.ID, model.ProfileInput{Accent: &accent}); err != nil {
			t.Errorf("accent %s: %v", accent, err)
		}
	}

	// Libraries: a restricted profile keeps its rights across updates that do not mention them;
	// switching to all libraries drops them.
	r, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("Eingeschränkt"), AllLibraries: boolPtr(false), LibraryIDs: &[]int64{libB, libA, libB}})
	if err != nil {
		t.Fatal(err)
	}
	if r.AllLibraries || !slices.Equal(r.LibraryIDs, []int64{libA, libB}) {
		t.Fatalf("restricted profile = %+v", r)
	}
	if r, err = s.UpdateProfile(ctx, sess, r.ID, model.ProfileInput{Name: str("Umbenannt")}); err != nil || !slices.Equal(r.LibraryIDs, []int64{libA, libB}) || r.Name != "Umbenannt" {
		t.Fatalf("rename kept libraries: %+v, %v", r, err)
	}
	if r, err = s.UpdateProfile(ctx, sess, r.ID, model.ProfileInput{LibraryIDs: &[]int64{libB}}); err != nil || !slices.Equal(r.LibraryIDs, []int64{libB}) {
		t.Fatalf("library replace: %+v, %v", r, err)
	}
	listed, _ := s.ListProfiles(ctx, sess)
	if i := slices.IndexFunc(listed, func(x model.Profile) bool { return x.ID == r.ID }); i < 0 || !slices.Equal(listed[i].LibraryIDs, []int64{libB}) {
		t.Errorf("listed libraries = %+v", listed)
	}
	if r, err = s.UpdateProfile(ctx, sess, r.ID, model.ProfileInput{AllLibraries: boolPtr(true)}); err != nil || !r.AllLibraries || len(r.LibraryIDs) != 0 {
		t.Fatalf("all libraries: %+v, %v", r, err)
	}

	// Primary profile: required for management, not deletable, found only inside the account.
	kidSession, err := s.SelectProfile(ctx, sess, p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile(ctx, kidSession, model.ProfileInput{Name: str("x")}); !errors.Is(err, model.ErrPrimaryProfileRequired) {
		t.Errorf("create as a non-primary profile: %v", err)
	}
	if err := s.DeleteProfile(ctx, kidSession, r.ID, nil); !errors.Is(err, model.ErrPrimaryProfileRequired) {
		t.Errorf("delete as a non-primary profile: %v", err)
	}
	if err := s.SetProfilePIN(ctx, kidSession, r.ID, "1234"); !errors.Is(err, model.ErrPrimaryProfileRequired) {
		t.Errorf("set PIN as a non-primary profile: %v", err)
	}
	if err := s.DeleteProfile(ctx, sess, sess.Profile.ID, func(context.Context, int64) error { t.Error("end must not run for a refused delete"); return nil }); !errors.Is(err, model.ErrPrimaryProfile) {
		t.Errorf("delete primary: %v", err)
	}
	if err := s.DeleteProfile(ctx, sess, 9999, nil); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("delete unknown: %v", err)
	}
	var ended []int64
	// Order: the sessions drop the profile first, then end runs, then the profile is deleted, then
	// end runs once more for a stream that slipped in between.
	var states []string
	end := func(_ context.Context, id int64) error {
		ended = append(ended, id)
		var attached, profiles int
		_ = s.db.QueryRow(`SELECT (SELECT count(*) FROM sessions WHERE profile_id = ?), (SELECT count(*) FROM profiles WHERE id = ?)`, id, id).Scan(&attached, &profiles)
		states = append(states, fmt.Sprintf("sessions %d, profile %d", attached, profiles))
		return nil
	}
	if err := s.DeleteProfile(ctx, sess, p.ID, end); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ended, []int64{p.ID, p.ID}) || !slices.Equal(states, []string{"sessions 0, profile 1", "sessions 0, profile 0"}) {
		t.Errorf("end called for %v with states %v, want twice: detached before the delete, deleted after", ended, states)
	}
	// kidSession lost its profile (the session itself lives on).
	if got, _, err := s.Resolve(ctx, token); err != nil || got.HasProfile() {
		t.Fatalf("resolve after the profile was deleted: %+v, %v", got.Profile, err)
	}
	var left int
	if err := s.db.QueryRow(`SELECT count(*) FROM sessions WHERE id = ? AND profile_id IS NULL`, kidSession.ID).Scan(&left); err != nil || left != 1 {
		t.Errorf("session of the deleted profile: %d rows without profile (%v)", left, err)
	}
	// Setting and removing the PIN of a foreign or unknown profile.
	if err := s.SetProfilePIN(ctx, sess, 9999, "1234"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("PIN of an unknown profile: %v", err)
	}
	if err := s.RemoveProfilePIN(ctx, sess, r.ID); err != nil {
		t.Errorf("removing a missing PIN: %v", err)
	}
}

func boolPtr(b bool) *bool { return &b }

func TestMaxTenProfiles(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	for i := 2; i <= MaxProfiles; i++ {
		if _, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("P" + string(rune('a'+i)))}); err != nil {
			t.Fatalf("profile %d: %v", i, err)
		}
	}
	_, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("elf")})
	if !slices.Equal(validationCodes(err), []string{"name:too_many_profiles"}) {
		t.Fatalf("11th profile: %v", err)
	}
	// Deleting one makes room again.
	list, _ := s.ListProfiles(ctx, sess)
	if len(list) != MaxProfiles {
		t.Fatalf("%d profiles", len(list))
	}
	if err := s.DeleteProfile(ctx, sess, list[1].ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("elf")}); err != nil {
		t.Errorf("after a delete: %v", err)
	}
}

// Guest accounts: created from an invitation with its ceiling and libraries; their profiles stay
// within both; the ceiling can be changed and the libraries replaced.
func TestGuestLimits(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	ownerSess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	libA, libB := addLibrary(t, s, "A"), addLibrary(t, s, "B")

	inv, err := s.CreateInvitation(ctx, ownerSess.Account, model.InvitationRequest{Note: "Oma", MaxRating: 12, LibraryIDs: []int64{libA}})
	if err != nil {
		t.Fatal(err)
	}
	guest, token, err := s.AcceptInvitation(fresh(), model.AcceptInvitation{Token: inv.Token, Email: "Oma@Example.ch", Password: "oma-password-1", ProfileName: "Oma"}, "ua")
	if err != nil {
		t.Fatal(err)
	}
	if guest.Account.Role != model.RoleUser || guest.Account.IsOwner || guest.Account.MaxRating != 12 || guest.Account.AllLibraries || !slices.Equal(guest.Account.LibraryIDs, []int64{libA}) ||
		!guest.Profile.IsPrimary || guest.Profile.MaxRating != 12 || guest.Profile.Language != "de" {
		t.Fatalf("guest session = %+v / %+v", guest.Account, guest.Profile)
	}
	// Resolve carries the limits.
	resolved, _, err := s.Resolve(ctx, token)
	if err != nil || resolved.Account.MaxRating != 12 || !slices.Equal(resolved.Account.LibraryIDs, []int64{libA}) || resolved.Account.Email != "oma@example.ch" {
		t.Fatalf("resolved = %+v, %v", resolved.Account, err)
	}

	// The guest's profiles: ceiling and libraries of the account apply.
	if _, err := s.CreateProfile(ctx, guest, model.ProfileInput{Name: str("Opa"), MaxRating: num(16)}); !slices.Equal(validationCodes(err), []string{"maxRating:above_account_limit"}) {
		t.Errorf("above the ceiling: %v", err)
	}
	if _, err := s.CreateProfile(ctx, guest, model.ProfileInput{Name: str("Opa"), AllLibraries: boolPtr(false), LibraryIDs: &[]int64{libB}}); !slices.Equal(validationCodes(err), []string{"libraryIds:unknown_library"}) {
		t.Errorf("library outside the account: %v", err)
	}
	def, err := s.CreateProfile(ctx, guest, model.ProfileInput{Name: str("Default")})
	if err != nil || def.MaxRating != 12 {
		t.Errorf("default rating of a guest profile = %d (%v), want the ceiling 12", def.MaxRating, err)
	}
	ok, err := s.CreateProfile(ctx, guest, model.ProfileInput{Name: str("Enkel"), MaxRating: num(6), AllLibraries: boolPtr(false), LibraryIDs: &[]int64{libA}})
	if err != nil || !slices.Equal(ok.LibraryIDs, []int64{libA}) {
		t.Errorf("guest profile within limits: %+v, %v", ok, err)
	}

	// The owner changes the limits.
	updated, err := s.UpdateGuest(ctx, guest.Account.ID, model.GuestUpdate{MaxRating: num(6), LibraryIDs: &[]int64{libA, libB}})
	if err != nil {
		t.Fatal(err)
	}
	if updated.MaxRating != 6 || updated.AllLibraries || !slices.Equal(updated.LibraryIDs, []int64{libA, libB}) || updated.ProfileCount != 3 || updated.Email != "oma@example.ch" {
		t.Fatalf("updated guest = %+v", updated)
	}
	if _, err := s.UpdateGuest(ctx, guest.Account.ID, model.GuestUpdate{MaxRating: num(5)}); !slices.Equal(validationCodes(err), []string{"maxRating:invalid_value"}) {
		t.Errorf("invalid ceiling: %v", err)
	}
	if _, err := s.UpdateGuest(ctx, guest.Account.ID, model.GuestUpdate{LibraryIDs: &[]int64{999}}); !slices.Equal(validationCodes(err), []string{"libraryIds:unknown_library"}) {
		t.Errorf("unknown library: %v", err)
	}
	if g, err := s.UpdateGuest(ctx, guest.Account.ID, model.GuestUpdate{AllLibraries: boolPtr(true)}); err != nil || !g.AllLibraries || len(g.LibraryIDs) != 0 {
		t.Errorf("all libraries: %+v, %v", g, err)
	}
	// The owner is no guest.
	if _, err := s.UpdateGuest(ctx, ownerSess.Account.ID, model.GuestUpdate{MaxRating: num(0)}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("update of the owner: %v", err)
	}
	if err := s.RevokeAccount(ctx, ownerSess.Account.ID, nil); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("revoke of the owner: %v", err)
	}
	list, err := s.ListGuests(ctx)
	if err != nil || len(list) != 1 || list[0].ProfileCount != 3 || list[0].LastSeenAt != nil {
		t.Fatalf("guest list = %+v, %v", list, err)
	}

	// Revocation: end runs first, then sessions and account are gone in one step.
	var endedFor int64
	if err := s.RevokeAccount(ctx, guest.Account.ID, func(_ context.Context, id int64) error { endedFor = id; return nil }); err != nil {
		t.Fatal(err)
	}
	if endedFor != guest.Account.ID {
		t.Errorf("end called for %d", endedFor)
	}
	if _, _, err := s.Resolve(ctx, token); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("resolve after revocation: %v", err)
	}
	if _, _, err := s.Login(fresh(), "oma@example.ch", "oma-password-1", "ua"); !errors.Is(err, model.ErrInvalidCredentials) {
		t.Errorf("login after revocation: %v", err)
	}
	if err := s.RevokeAccount(ctx, guest.Account.ID, nil); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("second revocation: %v", err)
	}
	// A failing end aborts the revocation.
	inv2, _ := s.CreateInvitation(ctx, ownerSess.Account, model.InvitationRequest{MaxRating: 6, AllLibraries: true})
	g2, _, err := s.AcceptInvitation(fresh(), model.AcceptInvitation{Token: inv2.Token, Email: "g2@example.ch", Password: "oma-password-2", ProfileName: "G2"}, "ua")
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	if err := s.RevokeAccount(ctx, g2.Account.ID, func(context.Context, int64) error { return boom }); !errors.Is(err, boom) {
		t.Errorf("failing end: %v", err)
	}
	if _, _, err := s.Login(fresh(), "g2@example.ch", "oma-password-2", "ua"); err != nil {
		t.Errorf("the account must survive a failed revocation: %v", err)
	}
}

// Invitation lifecycle: preview, accept, a second accept (410), expiry after 7 days, revocation,
// unknown tokens, validation, and the stored hash.
func TestInvitationLifecycle(t *testing.T) {
	t.Parallel()
	s, clock := newService(t)
	ownerSess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	_ = addLibrary(t, s, "Filme")
	libB := addLibrary(t, s, "Serien")

	if _, err := s.CreateInvitation(ctx, ownerSess.Account, model.InvitationRequest{MaxRating: 7, AllLibraries: true}); !slices.Equal(validationCodes(err), []string{"maxRating:invalid_value"}) {
		t.Errorf("invalid ceiling: %v", err)
	}
	if _, err := s.CreateInvitation(ctx, ownerSess.Account, model.InvitationRequest{MaxRating: 6, LibraryIDs: []int64{42}}); !slices.Equal(validationCodes(err), []string{"libraryIds:unknown_library"}) {
		t.Errorf("unknown library: %v", err)
	}
	if _, err := s.CreateInvitation(ctx, ownerSess.Account, model.InvitationRequest{MaxRating: 6, AllLibraries: true, Note: strings.Repeat("n", 121)}); !slices.Equal(validationCodes(err), []string{"note:too_long"}) {
		t.Errorf("long note: %v", err)
	}

	inv, err := s.CreateInvitation(ctx, ownerSess.Account, model.InvitationRequest{Note: " Oma ", MaxRating: 12, LibraryIDs: []int64{libB}})
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Token) != 43 || strings.ContainsAny(inv.Token, "+/=") || inv.State != InvitationOpen || inv.Note != "Oma" || !inv.ExpiresAt.Equal(clock.now().Add(7*24*time.Hour)) {
		t.Fatalf("created = %+v", inv)
	}
	var stored string
	if err := s.db.QueryRow(`SELECT token_hash FROM invitations WHERE id = ?`, inv.ID).Scan(&stored); err != nil || stored == inv.Token || stored != hashToken(inv.Token) {
		t.Errorf("stored token_hash = %q (%v): only the SHA-256 may be stored", stored, err)
	}

	prev, err := s.GetInvitation(fresh(), inv.Token)
	if err != nil {
		t.Fatal(err)
	}
	if prev.Note != "Oma" || prev.MaxRating != 12 || prev.AllLibraries || len(prev.Libraries) != 1 || prev.Libraries[0] != (model.LibraryRef{ID: libB, Name: "Serien"}) {
		t.Fatalf("preview = %+v", prev)
	}
	all, _ := s.CreateInvitation(ctx, ownerSess.Account, model.InvitationRequest{MaxRating: 18, AllLibraries: true})
	if p, err := s.GetInvitation(fresh(), all.Token); err != nil || len(p.Libraries) != 2 || !p.AllLibraries {
		t.Errorf("preview of an all-libraries link = %+v, %v", p, err)
	}

	// Unknown tokens: wrong length, unknown value.
	for _, tok := range []string{"", "short", strings.Repeat("A", 43), inv.Token + "x"} {
		if _, err := s.GetInvitation(fresh(), tok); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("token %q: %v, want ErrNotFound", tok, err)
		}
	}

	// Validation errors leave the link open.
	bad := model.AcceptInvitation{Token: inv.Token, Email: "nope", Password: "short", ProfileName: "", Language: "fr"}
	if _, _, err := s.AcceptInvitation(fresh(), bad, "ua"); !slices.Equal(validationCodes(err), []string{"email:invalid_email", "password:too_short", "profileName:required", "language:invalid_value"}) {
		t.Errorf("validation: %v", err)
	}
	// The owner's e-mail is taken (any case) and the link stays open.
	taken := model.AcceptInvitation{Token: inv.Token, Email: strings.ToUpper(testEmail), Password: "oma-password-1", ProfileName: "Oma"}
	if _, _, err := s.AcceptInvitation(fresh(), taken, "ua"); !slices.Equal(validationCodes(err), []string{"email:already_exists"}) {
		t.Errorf("taken e-mail: %v", err)
	}
	if _, err := s.GetInvitation(fresh(), inv.Token); err != nil {
		t.Fatalf("the link must stay open after failed attempts: %v", err)
	}

	good := model.AcceptInvitation{Token: inv.Token, Email: "oma@example.ch", Password: "oma-password-1", ProfileName: "Oma", Language: "en"}
	sess, _, err := s.AcceptInvitation(fresh(), good, "ua")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Profile.Language != "en" || sess.Profile.Name != "Oma" || !slices.Equal(sess.Account.LibraryIDs, []int64{libB}) {
		t.Fatalf("guest = %+v / %+v", sess.Account, sess.Profile)
	}
	if _, err := s.GetInvitation(fresh(), inv.Token); !errors.Is(err, model.ErrInvitationGone) {
		t.Errorf("preview of a used link: %v", err)
	}
	good.Email = "second@example.ch"
	if _, _, err := s.AcceptInvitation(fresh(), good, "ua"); !errors.Is(err, model.ErrInvitationGone) {
		t.Errorf("second accept: %v", err)
	}
	// The guest can log in and starts in its only profile; the account has no PIN.
	if got, _, err := s.Login(fresh(), "oma@example.ch", "oma-password-1", "ua"); err != nil || got.Profile.ID != sess.Profile.ID {
		t.Errorf("guest login: %+v, %v", got.Profile, err)
	}

	// List: states and the e-mail of the guest.
	list, err := s.ListInvitations(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	byID := map[int64]model.Invitation{}
	for _, i := range list {
		byID[i.ID] = i
	}
	if u := byID[inv.ID]; u.State != InvitationUsed || u.UsedByEmail != "oma@example.ch" || u.UsedAt == nil || !slices.Equal(u.LibraryIDs, []int64{libB}) {
		t.Errorf("used invitation = %+v", u)
	}
	if byID[all.ID].Token != "" {
		t.Error("the list must not carry the token")
	}

	// Revocation; revoking a used or revoked link is fine, an unknown id is not found.
	if err := s.RevokeInvitation(ctx, all.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeInvitation(ctx, all.ID); err != nil {
		t.Errorf("second revoke: %v", err)
	}
	if err := s.RevokeInvitation(ctx, inv.ID); err != nil {
		t.Errorf("revoke of a used link: %v", err)
	}
	if err := s.RevokeInvitation(ctx, 9999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("revoke unknown: %v", err)
	}
	if _, err := s.GetInvitation(fresh(), all.Token); !errors.Is(err, model.ErrInvitationGone) {
		t.Errorf("preview of a revoked link: %v", err)
	}
	if got, _ := s.ListInvitations(ctx); byIDOf(got, inv.ID).State != InvitationUsed || byIDOf(got, all.ID).State != InvitationRevoked {
		t.Errorf("states after revoking: %+v", got)
	}

	// Expiry: 7 days after creation the link answers 410, on the last second it still works.
	exp, _ := s.CreateInvitation(ctx, ownerSess.Account, model.InvitationRequest{MaxRating: 6, AllLibraries: true})
	clock.advance(7*24*time.Hour - time.Second)
	if _, err := s.GetInvitation(fresh(), exp.Token); err != nil {
		t.Errorf("one second before the expiry: %v", err)
	}
	clock.advance(time.Second)
	if _, err := s.GetInvitation(fresh(), exp.Token); !errors.Is(err, model.ErrInvitationGone) {
		t.Errorf("at the expiry: %v", err)
	}
	exp2 := model.AcceptInvitation{Token: exp.Token, Email: "late@example.ch", Password: "late-password-1", ProfileName: "Spät"}
	if _, _, err := s.AcceptInvitation(fresh(), exp2, "ua"); !errors.Is(err, model.ErrInvitationGone) {
		t.Errorf("accept of an expired link: %v", err)
	}
	if got, _ := s.ListInvitations(ctx); byIDOf(got, exp.ID).State != InvitationExpired {
		t.Errorf("expired state: %+v", byIDOf(got, exp.ID))
	}
}

func byIDOf(list []model.Invitation, id int64) model.Invitation {
	for _, i := range list {
		if i.ID == id {
			return i
		}
	}
	return model.Invitation{}
}

// Two concurrent acceptances of one link: exactly one account is created.
func TestInvitationAcceptedOnce(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	ownerSess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	inv, err := s.CreateInvitation(ctx, ownerSess.Account, model.InvitationRequest{MaxRating: 6, AllLibraries: true})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make([]error, 4)
	for i := range results {
		wg.Go(func() {
			in := model.AcceptInvitation{Token: inv.Token, Email: "g" + string(rune('a'+i)) + "@example.ch", Password: "guest-password-1", ProfileName: "G"}
			_, _, results[i] = s.AcceptInvitation(fresh(), in, "ua")
		})
	}
	wg.Wait()
	var ok, gone int
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, model.ErrInvitationGone):
			gone++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 || gone != 3 {
		t.Errorf("%d accepted, %d gone; want 1 and 3", ok, gone)
	}
	guests, _ := s.ListGuests(ctx)
	if len(guests) != 1 {
		t.Errorf("%d guest accounts", len(guests))
	}
}

// Resolve moves accounts.last_seen_at at most every five minutes, with the session touch.
func TestAccountLastSeen(t *testing.T) {
	t.Parallel()
	s, clock := newService(t)
	_, token := owner(t, s)
	ctx := ctxOn(deviceA)
	lastSeen := func() int64 {
		t.Helper()
		var v *int64
		if err := s.db.QueryRow(`SELECT last_seen_at FROM accounts LIMIT 1`).Scan(&v); err != nil {
			t.Fatal(err)
		}
		if v == nil {
			return 0
		}
		return *v
	}
	if lastSeen() != 0 {
		t.Fatal("last_seen_at set before any request")
	}
	if _, _, err := s.Resolve(ctx, token); err != nil || lastSeen() != 0 {
		t.Fatalf("a fresh session is not a touch: %v, %d", err, lastSeen())
	}
	clock.advance(6 * time.Minute)
	if _, extended, err := s.Resolve(ctx, token); err != nil || !extended {
		t.Fatalf("resolve after 6 min: %v, extended %v", err, extended)
	}
	first := lastSeen()
	if first != clock.now().UnixMilli() {
		t.Errorf("last_seen_at = %d, want %d", first, clock.now().UnixMilli())
	}
	clock.advance(2 * time.Minute)
	_, _, _ = s.Resolve(ctx, token)
	if lastSeen() != first {
		t.Error("last_seen_at moved within five minutes")
	}
}
