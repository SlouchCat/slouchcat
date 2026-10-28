package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

var ipCounter atomic.Int64

// ctxDev is a request context of the device cookie with a client address of its own, so the
// per-client attempt limit does not interfere with tests that make many PIN attempts.
func ctxDev(device string) context.Context {
	n := ipCounter.Add(1)
	return model.WithClientIP(model.WithDeviceID(context.Background(), device), fmt.Sprintf("203.0.%d.%d", n/250, n%250))
}

func deviceN(n int) string { return fmt.Sprintf("%032x", n+1) }

func pinProfile(t *testing.T, s *Service, sess model.Session) model.Profile {
	t.Helper()
	kid, err := s.CreateProfile(ctxDev(deviceA), sess, model.ProfileInput{Name: str("Kind"), PIN: str("4711")})
	if err != nil {
		t.Fatal(err)
	}
	return kid
}

func isLocked(err error) bool { return errors.Is(err, model.ErrPinLocked) }

// A client cannot get fresh PIN attempts by sending other device cookies: the counter belongs to
// the device the session started on.
func TestPINLockIgnoresDeviceCookieOfRequest(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	kid := pinProfile(t, s, sess)

	for i, cookie := range []string{deviceA, deviceB, deviceN(7)} {
		_, err := s.SelectProfile(ctxDev(cookie), sess, kid.ID, "0000")
		if i < 2 && !errors.Is(err, model.ErrPinInvalid) {
			t.Fatalf("attempt %d with cookie %s: %v, want ErrPinInvalid", i+1, cookie, err)
		}
		if i == 2 && !isLocked(err) {
			t.Fatalf("third attempt with a third cookie: %v, want the lock", err)
		}
	}
	// A fourth cookie and no cookie at all are locked as well, and so is the right PIN.
	for _, ctx := range []context.Context{ctxDev(deviceN(99)), model.WithClientIP(context.Background(), "198.51.100.77")} {
		if _, err := s.SelectProfile(ctx, sess, kid.ID, "4711"); !isLocked(err) {
			t.Errorf("right PIN under another cookie: %v, want the lock", err)
		}
	}
	// The device of a session never changes, whatever the request carries.
	if got, _, err := s.Resolve(ctxDev(deviceN(5)), mustToken(t, s, sess)); err != nil || got.DeviceID == deviceN(5) {
		t.Errorf("resolve under another cookie: device %q, %v", got.DeviceID, err)
	}
}

// Sessions get a device when they start: the cookie's, or a fresh random one without cookie; a
// session from before the migration (no device) gets one on its next request, and keeps it.
func TestSessionDeviceFixedAtStart(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, token := owner(t, s)
	if sess.DeviceID != deviceA {
		t.Fatalf("device = %q, want the cookie's %q", sess.DeviceID, deviceA)
	}
	nocookie, _, err := s.StartSession(model.WithClientIP(context.Background(), "192.0.2.9"), sess.Account, sess.Profile, "ua")
	if err != nil {
		t.Fatal(err)
	}
	if !devicePattern.MatchString(nocookie.DeviceID) || nocookie.DeviceID == profileCeilingDevice {
		t.Errorf("session without cookie: device %q, want a random one", nocookie.DeviceID)
	}
	// A cookie claiming the reserved ceiling device is replaced.
	forged, _, err := s.StartSession(model.WithDeviceID(context.Background(), profileCeilingDevice), sess.Account, sess.Profile, "ua")
	if err != nil || forged.DeviceID == profileCeilingDevice {
		t.Errorf("reserved device accepted: %q, %v", forged.DeviceID, err)
	}

	if _, err := s.db.Exec(`UPDATE sessions SET device_id = NULL WHERE id = ?`, sess.ID); err != nil {
		t.Fatal(err)
	}
	first, _, err := s.Resolve(ctxDev(deviceB), token)
	if err != nil || !devicePattern.MatchString(first.DeviceID) {
		t.Fatalf("old session: device %q, %v", first.DeviceID, err)
	}
	again, _, err := s.Resolve(ctxDev(deviceN(3)), token)
	if err != nil || again.DeviceID != first.DeviceID {
		t.Errorf("device of the old session changed: %q then %q (%v)", first.DeviceID, again.DeviceID, err)
	}
}

// Over all devices, more than 20 wrong PINs within an hour lock the profile for 15 min for
// everyone; the window then starts over.
func TestPINProfileCeiling(t *testing.T) {
	t.Parallel()
	s, clock := newService(t)
	sess, _ := owner(t, s)
	kid := pinProfile(t, s, sess)

	// Ten devices with two failures each stay below every per-device lock: 20 failures.
	sessions := make([]model.Session, 12)
	for i := range sessions {
		var err error
		if sessions[i], _, err = s.StartSession(ctxDev(deviceN(i)), sess.Account, sess.Profile, "ua"); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 10 {
		for range 2 {
			if _, err := s.SelectProfile(ctxDev(deviceN(i)), sessions[i], kid.ID, "0000"); !errors.Is(err, model.ErrPinInvalid) {
				t.Fatalf("device %d: %v, want ErrPinInvalid", i, err)
			}
			clock.advance(time.Second)
		}
	}
	// The 21st failure, from a device that never failed, locks the profile for everyone.
	_, err := s.SelectProfile(ctxDev(deviceN(10)), sessions[10], kid.ID, "0000")
	var l *model.PinLockedError
	if !errors.As(err, &l) || l.RetryAfter != 15*time.Minute {
		t.Fatalf("21st failure: %v, want a 15 min lock", err)
	}
	// Even the right PIN, on a device that never failed, is refused while the lock lasts.
	if _, err := s.SelectProfile(ctxDev(deviceN(11)), sessions[11], kid.ID, "4711"); !isLocked(err) {
		t.Errorf("right PIN during the ceiling lock: %v", err)
	}
	clock.advance(15*time.Minute + time.Second)
	if got, err := s.SelectProfile(ctxDev(deviceN(11)), sessions[11], kid.ID, "4711"); err != nil || got.Profile.ID != kid.ID {
		t.Errorf("right PIN after the lock: %v", err)
	}
	// Still inside the hour (21 failures counted): the next failure locks again.
	if _, err := s.SelectProfile(ctxDev(deviceN(10)), sessions[10], kid.ID, "0000"); !isLocked(err) {
		t.Errorf("failure inside the same window: %v, want the lock again", err)
	}
	// An hour after the window began, the count starts over.
	clock.advance(time.Hour + 16*time.Minute)
	if _, err := s.SelectProfile(ctxDev(deviceN(11)), sessions[11], kid.ID, "0000"); !errors.Is(err, model.ErrPinInvalid) {
		t.Errorf("first failure of a new window: %v, want ErrPinInvalid", err)
	}
}

// A device counter whose last failure is older than 24 h starts over.
func TestPINCounterDecays(t *testing.T) {
	t.Parallel()
	s, clock := newService(t)
	sess, _ := owner(t, s)
	kid := pinProfile(t, s, sess)
	wrong := func() error {
		_, err := s.SelectProfile(ctxDev(deviceA), sess, kid.ID, "0000")
		return err
	}
	for range 2 {
		if err := wrong(); !errors.Is(err, model.ErrPinInvalid) {
			t.Fatal(err)
		}
	}
	clock.advance(23 * time.Hour)
	if err := wrong(); !isLocked(err) {
		t.Fatalf("third failure within 24 h: %v, want the lock", err)
	}
	clock.advance(24*time.Hour + time.Hour)
	if err := wrong(); !errors.Is(err, model.ErrPinInvalid) {
		t.Errorf("failure after 25 h without failure: %v, want a fresh count (ErrPinInvalid)", err)
	}
}

// SetSessionProfile only takes a profile that exists in the session's account; a vanished one is
// reported as zero rows, which SelectProfile maps to ErrNotFound.
func TestSetSessionProfileGuard(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	other := addGuestAccount(t, s, "other@example.ch", 18, true, nil)
	foreign, err := s.write.InsertProfile(context.Background(), appq.InsertProfileParams{AccountID: other.ID, Name: "Fremd", MaxRating: 18, Language: "de", AllLibraries: 1, IsDefault: 1, Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	for name, id := range map[string]int64{"deleted or unknown": 9999, "foreign account": foreign.ID} {
		n, err := s.write.SetSessionProfile(context.Background(), appq.SetSessionProfileParams{ID: sess.ID, ProfileID: sql.NullInt64{Int64: id, Valid: true}, Now: 1})
		if err != nil || n != 0 {
			t.Errorf("%s: %d rows, %v; want 0 rows and no error", name, n, err)
		}
	}
	if got, _, _ := s.Resolve(ctxOn(deviceA), mustToken(t, s, sess)); got.Profile.ID != sess.Profile.ID {
		t.Errorf("the session profile changed")
	}
}

// Devices and PIN counters unused for 400 days go with the sweep of expired sessions.
func TestStaleDevicesPruned(t *testing.T) {
	t.Parallel()
	s, clock := newService(t)
	sess, _ := owner(t, s)
	kid := pinProfile(t, s, sess)
	old, fresh := deviceN(1), deviceN(2)
	for _, d := range []string{old, fresh} {
		if _, _, err := s.StartSession(ctxDev(d), sess.Account, sess.Profile, "ua"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SelectProfile(ctxDev(d), mustSession(t, s, d, sess), kid.ID, "0000"); err == nil {
			t.Fatal("wrong PIN accepted")
		}
		clock.advance(time.Second)
	}
	clock.advance(401 * 24 * time.Hour)
	if err := s.touchDeviceRow(context.Background(), fresh); err != nil { // used again today
		t.Fatal(err)
	}
	if err := s.write.UpsertPinAttempt(context.Background(), appq.UpsertPinAttemptParams{ProfileID: kid.ID, DeviceID: fresh, Failures: 1, Now: clock.now().UnixMilli()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteExpiredSessions(context.Background()); err != nil {
		t.Fatal(err)
	}
	count := func(query string, args ...any) int {
		var n int
		if err := s.db.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count(`SELECT count(*) FROM devices WHERE id = ?`, old) != 0 || count(`SELECT count(*) FROM pin_attempts WHERE device_id = ?`, old) != 0 {
		t.Error("the stale device and its PIN counter must be gone")
	}
	if count(`SELECT count(*) FROM devices WHERE id = ?`, fresh) != 1 || count(`SELECT count(*) FROM pin_attempts WHERE device_id = ?`, fresh) != 1 {
		t.Error("the device used today must stay")
	}
}

// mustSession starts a session on the device and returns it.
func mustSession(t *testing.T, s *Service, device string, like model.Session) model.Session {
	t.Helper()
	sess, _, err := s.StartSession(ctxDev(device), like.Account, like.Profile, "ua")
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

// A profile of another account (real, existing) is not found for every management call, and the
// foreign profile stays untouched.
func TestForeignProfilesAreNotFound(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	other := addGuestAccount(t, s, "other@example.ch", 18, true, nil)
	foreign, err := s.write.InsertProfile(context.Background(), appq.InsertProfileParams{AccountID: other.ID, Name: "Fremd", MaxRating: 18, Language: "de", AllLibraries: 1, Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateProfile(ctx, sess, foreign.ID, model.ProfileInput{Name: str("Gehackt")}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("UpdateProfile: %v", err)
	}
	if err := s.DeleteProfile(ctx, sess, foreign.ID, func(context.Context, int64) error { t.Error("end ran for a foreign profile"); return nil }); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("DeleteProfile: %v", err)
	}
	if err := s.SetProfilePIN(ctx, sess, foreign.ID, "1234"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("SetProfilePIN: %v", err)
	}
	if err := s.RemoveProfilePIN(ctx, sess, foreign.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("RemoveProfilePIN: %v", err)
	}
	row, err := s.read.GetProfile(context.Background(), foreign.ID)
	if err != nil || row.Name != "Fremd" || row.PinHash.Valid {
		t.Errorf("foreign profile changed: %+v, %v", row, err)
	}
}

// Revoking a guest: sessions are revoked before end runs, the account is deleted after the first
// call, and end runs again afterwards.
func TestDeleteAccountOrder(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	ownerSess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	inv, _ := s.CreateInvitation(ctx, ownerSess.Account, model.InvitationRequest{MaxRating: 6, AllLibraries: true})
	guest, token, err := s.AcceptInvitation(fresh(), model.AcceptInvitation{Token: inv.Token, Email: "g@example.ch", Password: "guest-password-1", ProfileName: "G"}, "ua")
	if err != nil {
		t.Fatal(err)
	}
	var states []string
	end := func(ctx context.Context, id int64) error {
		_, _, resolveErr := s.Resolve(ctx, token)
		var accounts int
		_ = s.db.QueryRow(`SELECT count(*) FROM accounts WHERE id = ?`, id).Scan(&accounts)
		states = append(states, fmt.Sprintf("session usable %v, accounts %d", resolveErr == nil, accounts))
		return nil
	}
	if err := s.DeleteAccount(ctx, guest.Account.ID, end); err != nil {
		t.Fatal(err)
	}
	want := []string{"session usable false, accounts 1", "session usable false, accounts 0"}
	if !slices.Equal(states, want) {
		t.Errorf("end saw %v, want %v", states, want)
	}
}

// The profile limit is enforced inside the write transaction: concurrent creations cannot exceed it.
func TestConcurrentProfileCreationRespectsLimit(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	var wg sync.WaitGroup
	errs := make([]error, 14)
	for i := range errs {
		wg.Go(func() {
			_, errs[i] = s.CreateProfile(ctxOn(deviceA), sess, model.ProfileInput{Name: str(fmt.Sprintf("P%d", i))})
		})
	}
	wg.Wait()
	var created int
	for _, err := range errs {
		switch {
		case err == nil:
			created++
		case slices.Equal(validationCodes(err), []string{"name:too_many_profiles"}):
		default:
			t.Errorf("unexpected: %v", err)
		}
	}
	list, _ := s.ListProfiles(ctxOn(deviceA), sess)
	if created != MaxProfiles-1 || len(list) != MaxProfiles {
		t.Errorf("%d created, %d profiles, want %d and %d", created, len(list), MaxProfiles-1, MaxProfiles)
	}
}

// allLibraries false needs libraries: invitations, guests and profiles answer libraryIds required.
func TestRestrictedWithoutLibrariesIsRefused(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	lib := addLibrary(t, s, "A")
	required := []string{"libraryIds:required"}

	if _, err := s.CreateInvitation(ctx, sess.Account, model.InvitationRequest{MaxRating: 6}); !slices.Equal(validationCodes(err), required) {
		t.Errorf("invitation without libraries: %v", err)
	}
	if _, err := s.CreateInvitation(ctx, sess.Account, model.InvitationRequest{MaxRating: 6, LibraryIDs: []int64{}}); !slices.Equal(validationCodes(err), required) {
		t.Errorf("invitation with an empty list: %v", err)
	}
	if _, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("x"), AllLibraries: boolPtr(false)}); !slices.Equal(validationCodes(err), required) {
		t.Errorf("profile without libraries: %v", err)
	}
	if _, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("x"), AllLibraries: boolPtr(false), LibraryIDs: &[]int64{}}); !slices.Equal(validationCodes(err), required) {
		t.Errorf("profile with an empty list: %v", err)
	}
	p, err := s.CreateProfile(ctx, sess, model.ProfileInput{Name: str("ok")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateProfile(ctx, sess, p.ID, model.ProfileInput{AllLibraries: boolPtr(false)}); !slices.Equal(validationCodes(err), required) {
		t.Errorf("update to restricted without libraries: %v", err)
	}
	if _, err := s.UpdateProfile(ctx, sess, p.ID, model.ProfileInput{AllLibraries: boolPtr(false), LibraryIDs: &[]int64{lib}}); err != nil {
		t.Errorf("update to restricted with a library: %v", err)
	}
	if _, err := s.UpdateProfile(ctx, sess, p.ID, model.ProfileInput{LibraryIDs: &[]int64{}}); !slices.Equal(validationCodes(err), required) {
		t.Errorf("update with an empty list: %v", err)
	}

	inv, _ := s.CreateInvitation(ctx, sess.Account, model.InvitationRequest{MaxRating: 6, AllLibraries: true})
	guest, _, err := s.AcceptInvitation(fresh(), model.AcceptInvitation{Token: inv.Token, Email: "g@example.ch", Password: "guest-password-1", ProfileName: "G"}, "ua")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateAccountAccess(ctx, guest.Account.ID, model.AccountAccessUpdate{AllLibraries: boolPtr(false)}); !slices.Equal(validationCodes(err), required) {
		t.Errorf("guest to restricted without libraries: %v", err)
	}
	if err := s.UpdateAccountAccess(ctx, guest.Account.ID, model.AccountAccessUpdate{AllLibraries: boolPtr(false), LibraryIDs: &[]int64{}}); !slices.Equal(validationCodes(err), required) {
		t.Errorf("guest with an empty list: %v", err)
	}
}

// Lowering a guest's ceiling lowers its profiles in the same step; raising it leaves them.
func TestGuestCeilingClampsProfiles(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	inv, _ := s.CreateInvitation(ctx, sess.Account, model.InvitationRequest{MaxRating: 16, AllLibraries: true})
	guest, _, err := s.AcceptInvitation(fresh(), model.AcceptInvitation{Token: inv.Token, Email: "g@example.ch", Password: "guest-password-1", ProfileName: "G"}, "ua")
	if err != nil {
		t.Fatal(err)
	}
	for name, rating := range map[string]int{"Zwölf": 12, "Sechs": 6, "Sechzehn": 16} {
		if _, err := s.CreateProfile(ctx, guest, model.ProfileInput{Name: str(name), MaxRating: num(rating)}); err != nil {
			t.Fatal(err)
		}
	}
	ratings := func() map[string]int {
		list, err := s.ListProfiles(ctx, guest)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]int{}
		for _, p := range list {
			out[p.Name] = p.MaxRating
		}
		return out
	}
	if err := s.UpdateAccountAccess(ctx, guest.Account.ID, model.AccountAccessUpdate{MaxRating: num(12)}); err != nil {
		t.Fatal(err)
	}
	if got, want := ratings(), map[string]int{"G": 12, "Zwölf": 12, "Sechs": 6, "Sechzehn": 12}; !mapsEqual(got, want) {
		t.Errorf("after lowering to 12: %v, want %v", got, want)
	}
	if err := s.UpdateAccountAccess(ctx, guest.Account.ID, model.AccountAccessUpdate{MaxRating: num(16)}); err != nil {
		t.Fatal(err)
	}
	if got := ratings(); got["Sechzehn"] != 12 || got["Sechs"] != 6 {
		t.Errorf("raising the ceiling must not raise profiles: %v", got)
	}
}

func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// listAccountLibraries: the main account sees every library, a guest its own, by name.
func TestListAccountLibraries(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	sess, _ := owner(t, s)
	ctx := ctxOn(deviceA)
	b, a, c := addLibrary(t, s, "Zeta"), addLibrary(t, s, "alpha"), addLibrary(t, s, "Mitte")
	names := func(libs []model.LibraryRef) []string {
		out := []string{}
		for _, l := range libs {
			out = append(out, l.Name)
		}
		return out
	}
	libs, err := s.ListAccountLibraries(ctx, sess.Account)
	if err != nil || !slices.Equal(names(libs), []string{"alpha", "Mitte", "Zeta"}) || libs[0].Kind != "movies" {
		t.Fatalf("owner libraries = %+v, %v", libs, err)
	}
	guest := addGuestAccount(t, s, "g@example.ch", 12, false, []int64{b, a})
	libs, err = s.ListAccountLibraries(ctx, model.Account{ID: guest.ID, Role: model.RoleUser, LibraryIDs: []int64{b, a}})
	if err != nil || !slices.Equal(names(libs), []string{"alpha", "Zeta"}) {
		t.Errorf("guest libraries = %+v, %v", libs, err)
	}
	_ = c
	if libs, err := s.ListAccountLibraries(ctx, model.Account{Role: model.RoleUser}); err != nil || len(libs) != 0 {
		t.Errorf("guest without rights: %+v, %v", libs, err)
	}
}
