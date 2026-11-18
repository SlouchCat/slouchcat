package wishes

import (
	"errors"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// Security review H-1, L-2 to L-5: a Seerr user that administers the instance is never taken over
// by an invitation or an email change, never gets the account's password or email, and the
// hand-overs of one account run one after the other.

// managingRights are the rights that make a Seerr user privileged.
var managingRights = map[string]uint64{
	"ADMIN": seerrtest.PermAdmin, "MANAGE_USERS": seerrtest.PermManageUsers,
	"MANAGE_SETTINGS": seerrtest.PermManageSettings, "MANAGE_REQUESTS": seerrtest.PermManageRequests,
}

// A stored user says privileged by the rule of the automatic linking (user 1 or a managing
// right), wider than isAdmin, the limit exemption (MANAGE_USERS).
func TestSeerrUserPrivileged(t *testing.T) {
	for _, tc := range []struct {
		name       string
		id         int64
		perm       seerr.Permission
		privileged bool
	}{
		{"user 1 without rights", 1, 0, true},
		{"only MANAGE_REQUESTS", 7, seerr.PermManageRequests, true},
		{"only MANAGE_SETTINGS", 8, seerr.PermManageSettings, true},
		{"plain requester", 9, seerr.PermRequest, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := toSeerrUser(wishq.SeerrUser{SeerrID: tc.id, DisplayName: "x", UserType: 2, Permissions: int64(tc.perm)})
			if u.Privileged != tc.privileged || u.IsAdmin {
				t.Errorf("privileged = %v, isAdmin = %v; want %v, false", u.Privileged, u.IsAdmin, tc.privileged)
			}
		})
	}
}

// An invitation and an email change never link user 1 or a user with a managing right: the
// account waits with seerr_user_privileged and nothing goes to Seerr. «In Seerr eröffnen» of an
// administrator and the choice by hand still link it, without password.
func TestAutomaticLinkNeverTakesPrivilegedUser(t *testing.T) {
	for name, perm := range managingRights {
		for _, by := range []string{LinkedByInvite, LinkedByEmailChange} {
			t.Run(name+"/"+by, func(t *testing.T) {
				f := newFixture(t)
				f.fake.UpdateUser(4, func(u *seerrtest.User) { u.Permissions = seerrtest.PermRequest | perm; u.Password = "old-password-x" })
				if _, err := f.svc.provision(f.ctx, 3, by, testPassword, 0); err != nil {
					t.Fatal(err)
				}
				m := f.mapping(3)
				if m.State != StateWaiting || m.SeerrUserID != nil || m.ProvisionError != ProvisionUserPrivileged {
					t.Fatalf("mapping = %+v", m)
				}
				for _, route := range []string{"POST /user", "POST /user/{id}/settings/password", "POST /user/{id}/settings/main"} {
					if n := f.fake.Calls(route); n != 0 {
						t.Errorf("%s called %d times", route, n)
					}
				}
				if u, _ := f.fake.User(4); u.Password != "old-password-x" || u.Email != "grosseltern@example.test" {
					t.Errorf("privileged user changed: %+v", u)
				}
				// A retry of the waiting account (users job) keeps it waiting.
				if _, err := f.svc.provision(f.ctx, 3, by, "", 0); err != nil {
					t.Fatal(err)
				}
				if m := f.mapping(3); m.State != StateWaiting || m.ProvisionError != ProvisionUserPrivileged {
					t.Errorf("after a retry = %+v", m)
				}
				// «Alle eröffnen» leaves the waiting account out and counts it for a choice by hand
				// (security review M-1).
				f.refresh()
				f.runQueued(JobProvision)
				if _, _, manual, err := f.svc.ProvisionAll(f.ctx, 1); err != nil || manual != 1 {
					t.Fatalf("all: manual = %d, %v", manual, err)
				}
				if n := f.queued(JobProvision, `"accountId":3,`); n != 0 {
					t.Errorf("waiting privileged account queued %d times", n)
				}
				// «In Seerr eröffnen» without the confirmation keeps it waiting; with it, it links;
				// the password stays in Seerr.
				f.fake.ResetCalls() // the shared user the refresh queued got a password
				if m, err := f.svc.ProvisionAccount(f.ctx, 3, 1, false); err != nil || m.State != StateWaiting || m.ProvisionError != ProvisionUserPrivileged {
					t.Fatalf("unconfirmed «In Seerr eröffnen» = %+v, %v", m, err)
				}
				if _, err := f.svc.ProvisionAccount(f.ctx, 3, 1, true); err != nil {
					t.Fatal(err)
				}
				if m := f.mapping(3); m.State != OriginLinked || m.PasswordState != PasswordAdmin || m.LinkedBy != LinkedByAdmin {
					t.Errorf("after «In Seerr eröffnen» = %+v", m)
				}
				if hints, _ := f.svc.RecentLinks(f.ctx); len(hints) != 0 {
					t.Errorf("«Neu verknüpft» after an administrator's link: %+v", hints)
				}
				if n := f.fake.Calls("POST /user/{id}/settings/password"); n != 0 {
					t.Errorf("password handed to a privileged user: %d", n)
				}
			})
		}
	}
	// User 1 too, and the choice by hand stays possible.
	f := newFixture(t)
	f.exec(`UPDATE accounts SET email = 'admin@example.org' WHERE id = 2`)
	if _, err := f.svc.provision(f.ctx, 2, LinkedByInvite, testPassword, 0); err != nil {
		t.Fatal(err)
	}
	if m := f.mapping(2); m.State != StateWaiting || m.ProvisionError != ProvisionUserPrivileged {
		t.Fatalf("user 1 = %+v", m)
	}
	f.refresh()
	one := 1
	if m, err := f.svc.SetAccountMapping(f.ctx, 2, &one, 1); err != nil || m.PasswordState != PasswordAdmin {
		t.Errorf("by hand = %+v, %v", m, err)
	}
}

// A linked user that became privileged or external in Seerr after the link never gets the
// password or the email again: both are read fresh before every write, and the state follows.
func TestHandOverChecksRightsFresh(t *testing.T) {
	for name, change := range map[string]func(*seerrtest.User){
		"MANAGE_SETTINGS": func(u *seerrtest.User) { u.Permissions |= seerrtest.PermManageSettings },
		"MANAGE_REQUESTS": func(u *seerrtest.User) { u.Permissions |= seerrtest.PermManageRequests },
		"jellyfin":        func(u *seerrtest.User) { u.UserType = seerrtest.UserJellyfin; u.JellyfinUsername = "oma" },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			if _, err := f.svc.provision(f.ctx, 3, LinkedByInvite, testPassword, 0); err != nil {
				t.Fatal(err)
			}
			if m := f.mapping(3); m.State != OriginLinked || m.PasswordState != PasswordSynced {
				t.Fatalf("linked = %+v", m)
			}
			f.fake.UpdateUser(4, change)
			f.fake.ResetCalls()
			if err := f.svc.handOver(f.ctx, 3, "grosseltern@example.test", testPassword2, MomentPasswordChange); err != nil {
				t.Fatal(err)
			}
			want := PasswordAdmin
			if name == "jellyfin" {
				want = PasswordExternal
			}
			if m := f.mapping(3); m.PasswordState != want {
				t.Errorf("after the password change = %+v, want %s", m, want)
			}
			f.exec(`UPDATE wish_account_map SET password_state = 'synced' WHERE account_id = 3`) // as if read before
			if err := f.svc.handOver(f.ctx, 3, "attacker@example.test", testPassword2, MomentEmailChange); err != nil {
				t.Fatal(err)
			}
			for _, route := range []string{"POST /user/{id}/settings/password", "POST /user/{id}/settings/main"} {
				if n := f.fake.Calls(route); n != 0 {
					t.Errorf("%s called %d times", route, n)
				}
			}
			if u, _ := f.fake.User(4); u.Password != testPassword || u.Email != "grosseltern@example.test" {
				t.Errorf("user in seerr = %+v", u)
			}
			if m := f.mapping(3); m.PasswordState != want {
				t.Errorf("after the email change = %+v, want %s", m, want)
			}
		})
	}
}

// The users job sets the password state from the rights it reads: privileged → admin, external →
// external; a user that lost its right falls back to off, never back to a synced password.
func TestUsersJobGuardsPasswordState(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.provision(f.ctx, 3, LinkedByInvite, testPassword, 0); err != nil {
		t.Fatal(err)
	}
	f.fake.UpdateUser(4, func(u *seerrtest.User) { u.Permissions |= seerrtest.PermManageRequests })
	f.refresh()
	if m := f.mapping(3); m.PasswordState != PasswordAdmin {
		t.Errorf("promoted = %+v", m)
	}
	if got := f.count(`SELECT is_admin FROM seerr_users WHERE seerr_id = 4`); got != 0 {
		t.Error("MANAGE_REQUESTS counted for «Kein Limit»; Seerr exempts only MANAGE_USERS")
	}
	f.fake.UpdateUser(4, func(u *seerrtest.User) { u.Permissions = seerrtest.PermRequest })
	f.refresh()
	if m := f.mapping(3); m.PasswordState != PasswordOff {
		t.Errorf("demoted = %+v", m)
	}
}

// Hand-overs of one account run one after the other and the login reads the mapping under the
// lock: a login that waited behind a password change does not write the old password.
func TestHandOverOneAtATimePerAccount(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.provision(f.ctx, 3, LinkedByInvite, "", 0); err != nil {
		t.Fatal(err)
	}
	if m := f.mapping(3); m.PasswordState != PasswordNextLogin {
		t.Fatalf("linked = %+v", m)
	}
	blocked := make(chan struct{})
	f.svc.accountLocks.waiting = func(int64) { close(blocked) }
	unlock, err := f.svc.accountLocks.lock(f.ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- f.svc.handOver(f.ctx, 3, "grosseltern@example.test", testPassword, MomentLogin) }()
	// The login waits in the lock (review Minor 8: proven, not hoped for).
	select {
	case <-blocked:
	case err := <-done:
		t.Fatalf("the login did not wait for the lock: %v", err)
	}
	// Meanwhile a password change synced the new password.
	f.fake.UpdateUser(4, func(u *seerrtest.User) { u.Password = testPassword2 })
	f.exec(`UPDATE wish_account_map SET password_state = 'synced' WHERE account_id = 3`)
	unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if u, _ := f.fake.User(4); u.Password != testPassword2 || f.fake.Calls("POST /user/{id}/settings/password") != 0 {
		t.Errorf("the login wrote the old password: %q", u.Password)
	}
}

// A user that held the shared address already gets a fresh random password when Slouchcat takes it
// over (security review L-4).
func TestSharedUserTakeOverResetsPassword(t *testing.T) {
	f := newFixture(t)
	f.fake.UpdateUser(5, func(u *seerrtest.User) { u.Password = "known-to-someone" })
	if err := f.svc.EnsureSharedUser(f.ctx); err != nil {
		t.Fatal(err)
	}
	if u, _ := f.fake.User(5); u.Password == "known-to-someone" || u.Password == "" {
		t.Errorf("password of the taken-over shared user = %q", u.Password)
	}
}

// The key is needed again when the scheme changes, not only the host (security review L-5).
func TestSameHostIncludesScheme(t *testing.T) {
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{"https://seerr.example.test", "https://SEERR.example.test", true},
		{"https://seerr.example.test:5055", "https://seerr.example.test:5055/x", true},
		{"https://seerr.example.test", "http://seerr.example.test", false},
		{"https://seerr.example.test", "https://other.example.test", false},
		{"http://seerr:5055", "http://seerr:5056", false},
	} {
		if got := sameHost(c.a, c.b); got != c.same {
			t.Errorf("sameHost(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

// queued counts the waiting jobs of type typ whose payload contains part.
func (f *fixture) queued(typ, part string) int {
	f.queue.mu.Lock()
	defer f.queue.mu.Unlock()
	n := 0
	for _, j := range f.queue.jobs {
		if j.Type == typ && strings.Contains(string(j.Payload), part) {
			n++
		}
	}
	return n
}

// «Alle eröffnen» is automatic for a privileged user (security review M-1): a privileged user of
// the same email is counted for a choice by hand, its job leaves the account waiting with
// seerr_user_privileged and Seerr is not touched. Open and retry jobs of one account have keys of
// their own (review Minor 4).
func TestProvisionAllNeverTakesPrivilegedUser(t *testing.T) {
	f := newFixture(t)
	f.fake.UpdateUser(4, func(u *seerrtest.User) { u.Permissions = seerrtest.PermRequest | seerrtest.PermManageRequests })
	f.refresh()
	f.runQueued(JobProvision)
	link, create, manual, err := f.svc.ProvisionAll(f.ctx, 1)
	if err != nil || manual != 1 || link+create+manual != 3 {
		t.Fatalf("all = %d, %d, %d, %v", link, create, manual, err)
	}
	f.fake.ResetCalls()
	f.runQueued(JobProvision)
	if m := f.mapping(3); m.State != StateWaiting || m.ProvisionError != ProvisionUserPrivileged || m.LinkedBy != LinkedByAdmin {
		t.Errorf("privileged after «Alle eröffnen» = %+v", m)
	}
	for _, c := range f.fake.CallLog() {
		if strings.Contains(c.Path, "/user/4") {
			t.Errorf("privileged user touched: %s %s", c.Route, c.Path)
		}
	}
	keys := map[string]bool{}
	for _, intent := range []string{intentOpen, intentRetry} {
		if err := f.svc.queueProvision(f.ctx, provisionPayload{AccountID: 3, Intent: intent}, provisionPriority); err != nil {
			t.Fatal(err)
		}
	}
	f.queue.mu.Lock()
	for _, j := range f.queue.jobs {
		keys[j.IdempotencyKey] = true
	}
	f.queue.mu.Unlock()
	if len(keys) != 2 {
		t.Errorf("open and retry share a key: %v", keys)
	}
}

// A user of the shared address that belongs to an account keeps its password: the take-over
// stops before the password (security review N-5).
func TestSharedUserOfAnAccountKeepsPassword(t *testing.T) {
	f := newFixture(t)
	f.refresh()
	f.fake.UpdateUser(5, func(u *seerrtest.User) { u.Password = "known-to-the-account" })
	f.exec(`INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, password_state, set_at) VALUES (2, 5, 'manual', 'ready', 'off', 0)`)
	if err := f.svc.EnsureSharedUser(f.ctx); !errors.Is(err, model.ErrSeerrUserTaken) {
		t.Fatalf("ensure = %v", err)
	}
	if u, _ := f.fake.User(5); u.Password != "known-to-the-account" {
		t.Errorf("password of the account's user rewritten: %q", u.Password)
	}
}

// A hand choice waits for the provisioning of the account in flight (review m-2): the job that
// holds the lock links its user, the choice runs after it and stays.
func TestSetAccountMappingWaitsForTheProvisioning(t *testing.T) {
	f := newFixture(t)
	f.refresh()
	blocked := make(chan struct{})
	f.svc.accountLocks.waiting = func(int64) { close(blocked) }
	unlock, err := f.svc.accountLocks.lock(f.ctx, 3) // the job of «Alle eröffnen»
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.SetAccountMapping(f.ctx, 3, ip(4), 1)
		done <- err
	}()
	select {
	case <-blocked:
	case err := <-done:
		t.Fatalf("the hand choice did not wait for the lock: %v", err)
	}
	if _, err := f.svc.provision(f.ctx, 3, LinkedByAdmin, "", 0); err != nil {
		t.Fatal(err)
	}
	if m := f.mapping(3); m.State != OriginLinked {
		t.Fatalf("job = %+v", m)
	}
	unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if m := f.mapping(3); m.State != OriginManual || m.SeerrUserID == nil || *m.SeerrUserID != 4 {
		t.Errorf("hand choice = %+v", m)
	}
}
