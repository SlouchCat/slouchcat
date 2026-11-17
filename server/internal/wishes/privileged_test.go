package wishes

import (
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// Security review H-1, L-2 to L-5: a Seerr user that administers the instance is never taken over
// by an invitation or an email change, never gets the account's password or email, and the
// hand-overs of one account run one after the other.

// managingRights are the rights that make a Seerr user privileged.
var managingRights = map[string]uint64{
	"ADMIN": seerrtest.PermAdmin, "MANAGE_USERS": seerrtest.PermManageUsers,
	"MANAGE_SETTINGS": seerrtest.PermManageSettings, "MANAGE_REQUESTS": seerrtest.PermManageRequests,
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
				// The administrator's «In Seerr eröffnen» links it; the password stays in Seerr.
				if _, err := f.svc.ProvisionAccount(f.ctx, 3, 1); err != nil {
					t.Fatal(err)
				}
				if m := f.mapping(3); m.State != OriginLinked || m.PasswordState != PasswordAdmin {
					t.Errorf("after «In Seerr eröffnen» = %+v", m)
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
	unlock, err := f.svc.accountLocks.lock(f.ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- f.svc.handOver(f.ctx, 3, "grosseltern@example.test", testPassword, MomentLogin) }()
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
