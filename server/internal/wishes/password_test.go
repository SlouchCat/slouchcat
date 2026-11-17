package wishes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// Build step 1.5b: the password in the three moments (architecture 18.3; N-91, N-94, N-95, N-103).

const (
	testPassword  = "Pw-for-seerr-7731-xq"
	testPassword2 = "Pw-second-seerr-8842-zr"
)

var clientN int

// client is a request context of a new client address (the attempt limit of auth counts per client).
func client() context.Context {
	clientN++
	return model.WithClientIP(model.WithDeviceID(context.Background(), "dddddddddddddddddddddddddddddddd"), "198.51.100."+string(rune('0'+clientN%10))+string(rune('0'+clientN/10%10)))
}

// withAuth returns the auth service of the fixture with the hook of the service.
func (f *fixture) withAuth() *auth.Service {
	return auth.New(f.db, f.log).WithPasswordSeen(f.svc.PasswordSeen)
}

// accept invites email and accepts it with password; it returns the session.
func (f *fixture) accept(a *auth.Service, email, password string) model.Session {
	f.t.Helper()
	inv, err := a.CreateInvitation(f.ctx, model.Account{ID: 1}, model.InvitationRequest{MaxRating: 12, AllLibraries: true})
	if err != nil {
		f.t.Fatal(err)
	}
	sess, _, err := a.AcceptInvitation(client(), model.AcceptInvitation{Token: inv.Token, Email: email, Password: password, ProfileName: "Neu"}, "ua")
	if err != nil {
		f.t.Fatal(err)
	}
	return sess
}

// noPasswordAnywhere searches the database files, the log and the queued jobs for the passwords.
func (f *fixture) noPasswordAnywhere(passwords ...string) {
	f.t.Helper()
	f.svc.waitBackground()
	if _, err := f.db.Write.ExecContext(f.ctx, `PRAGMA wal_checkpoint(FULL)`); err != nil {
		f.t.Fatal(err)
	}
	files := 0
	err := filepath.WalkDir(f.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files++
		for _, pw := range passwords {
			if bytes.Contains(data, []byte(pw)) {
				f.t.Errorf("%s carries a password", path)
			}
		}
		return nil
	})
	if err != nil || files == 0 {
		f.t.Fatalf("walk %s: %v, %d files", f.dir, err, files)
	}
	for _, pw := range passwords {
		if strings.Contains(f.logs.String(), pw) {
			f.t.Error("the log carries a password")
		}
	}
	f.queue.mu.Lock()
	defer f.queue.mu.Unlock()
	for _, j := range f.queue.jobs {
		for _, pw := range passwords {
			if bytes.Contains(j.Payload, []byte(pw)) {
				f.t.Errorf("job %s carries a password", j.Type)
			}
		}
	}
}

// The three moments: an invitation opens the user with the real password, a changed password goes
// to a synced user, a sign-in hands it over only with next_login; off gets nothing.
func TestPasswordSeenThreeMoments(t *testing.T) {
	f := newFixture(t)
	a := f.withAuth()
	sess := f.accept(a, "neu@example.test", testPassword)
	f.svc.waitBackground()
	m := f.mapping(sess.Account.ID)
	if m.State != OriginCreated || m.PasswordState != PasswordSynced || m.PasswordSyncedAt == nil {
		t.Fatalf("after the invitation = %+v", m)
	}
	u, _ := f.fake.User(*m.SeerrUserID)
	if u.Password != testPassword || u.Email != "neu@example.test" {
		t.Errorf("seerr user = %+v", u)
	}
	// A new password goes to the synced user at once.
	if _, err := a.UpdateAccount(client(), sess, model.AccountUpdate{CurrentPassword: testPassword, NewPassword: new(testPassword2)}); err != nil {
		t.Fatal(err)
	}
	f.svc.waitBackground()
	if u, _ := f.fake.User(*m.SeerrUserID); u.Password != testPassword2 {
		t.Errorf("password after the change = %q", u.Password)
	}
	// Sign-in: nothing while synced; with next_login it is set.
	f.fake.ResetCalls()
	if _, _, err := a.Login(client(), "neu@example.test", testPassword2, "ua"); err != nil {
		t.Fatal(err)
	}
	f.svc.waitBackground()
	if n := f.fake.Calls("POST /user/{id}/settings/password"); n != 0 {
		t.Errorf("sign-in while synced set the password %d times", n)
	}
	if _, err := f.svc.SetPasswordMode(f.ctx, sess.Account.ID, PasswordModeNextLogin); err != nil {
		t.Fatal(err)
	}
	f.fake.UpdateUser(*m.SeerrUserID, func(u *seerrtest.User) { u.Password = "changed-in-seerr" })
	if _, _, err := a.Login(client(), "neu@example.test", testPassword2, "ua"); err != nil {
		t.Fatal(err)
	}
	f.svc.waitBackground()
	if u, _ := f.fake.User(*m.SeerrUserID); u.Password != testPassword2 || f.mapping(sess.Account.ID).PasswordState != PasswordSynced {
		t.Errorf("next_login not handed over: %q", u.Password)
	}
	// Off: neither a change nor a sign-in reaches Seerr.
	if _, err := f.svc.SetPasswordMode(f.ctx, sess.Account.ID, PasswordModeOff); err != nil {
		t.Fatal(err)
	}
	f.fake.ResetCalls()
	if _, err := a.UpdateAccount(client(), sess, model.AccountUpdate{CurrentPassword: testPassword2, NewPassword: new(testPassword)}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Login(client(), "neu@example.test", testPassword, "ua"); err != nil {
		t.Fatal(err)
	}
	f.svc.waitBackground()
	if n := len(f.fake.CallLog()); n != 0 {
		t.Errorf("off: %d calls to seerr", n)
	}
	f.noPasswordAnywhere(testPassword, testPassword2)
}

// An administrator's link keeps the password (off); a change of the account's password gives Seerr
// nothing. Administrators of Seerr and Emby users refuse the mode.
func TestPasswordModeOfLinkedUsers(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.ProvisionAccount(f.ctx, 3, 1, false); err != nil { // grosseltern, linked to 4
		t.Fatal(err)
	}
	f.svc.PasswordSeen(f.ctx, 3, "grosseltern@example.test", testPassword, MomentPasswordChange)
	f.svc.PasswordSeen(f.ctx, 3, "grosseltern@example.test", testPassword, MomentLogin)
	f.svc.waitBackground()
	if n := f.fake.Calls("POST /user/{id}/settings/password"); n != 0 {
		t.Errorf("off: password set %d times", n)
	}
	f.exec(`UPDATE wish_account_map SET password_state = 'admin' WHERE account_id = 3`)
	if _, err := f.svc.SetPasswordMode(f.ctx, 3, PasswordModeNextLogin); !errors.Is(err, model.ErrPasswordNotSyncable) {
		t.Errorf("admin: %v", err)
	}
	if _, err := f.svc.SetPasswordMode(f.ctx, 2, PasswordModeNextLogin); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("no user: %v", err)
	}
	if _, err := f.svc.SetPasswordMode(f.ctx, 3, "always"); err == nil {
		t.Error("unknown mode accepted")
	}
	f.exec(`UPDATE wish_account_map SET password_state = 'off' WHERE account_id = 3`)
	if m, err := f.svc.SetPasswordMode(f.ctx, 3, PasswordModeNextLogin); err != nil || m.PasswordState != PasswordNextLogin {
		t.Errorf("switched on = %+v, %v", m, err)
	}
	f.svc.PasswordSeen(f.ctx, 3, "grosseltern@example.test", testPassword, MomentLogin)
	f.svc.waitBackground()
	if u, _ := f.fake.User(4); u.Password != testPassword {
		t.Error("next_login after switching on not handed over")
	}
	f.noPasswordAnywhere(testPassword)
}

// Email change: a ready user gets the new email; a taken one leaves it unchanged with a hint; an
// account without a user and a hit is linked at once with the password (email_change).
func TestPasswordEmailChange(t *testing.T) {
	f := newFixture(t)
	a := f.withAuth()
	sess := f.accept(a, "neu@example.test", testPassword)
	f.svc.waitBackground()
	m := f.mapping(sess.Account.ID)
	if _, err := a.UpdateAccount(client(), sess, model.AccountUpdate{CurrentPassword: testPassword, Email: new("Neu2@Example.test")}); err != nil {
		t.Fatal(err)
	}
	f.svc.waitBackground()
	if u, _ := f.fake.User(*m.SeerrUserID); u.Email != "neu2@example.test" {
		t.Errorf("email in seerr = %q", u.Email)
	}
	sess.Account.Email = "neu2@example.test"
	// Taken by user 3 in seerr: unchanged, hint email_taken.
	if _, err := a.UpdateAccount(client(), sess, model.AccountUpdate{CurrentPassword: testPassword, Email: new("kinder@example.test")}); err != nil {
		t.Fatal(err)
	}
	f.svc.waitBackground()
	if u, _ := f.fake.User(*m.SeerrUserID); u.Email != "neu2@example.test" {
		t.Errorf("email after INVALID_EMAIL = %q", u.Email)
	}
	if got := f.mapping(sess.Account.ID); got.ProvisionError != ProvisionEmailTaken || got.State != OriginCreated {
		t.Errorf("hint = %+v", got)
	}
	// Account 2 without a user changes to an email Seerr has: linked at once with the password.
	f.fake.UpdateUser(4, func(u *seerrtest.User) { u.Email = "oma@example.test"; u.Password = "old-password-x" })
	f.exec(`UPDATE accounts SET email = 'oma@example.test' WHERE id = 2`) // auth commits before the hook
	f.svc.PasswordSeen(f.ctx, 2, "oma@example.test", testPassword2, MomentEmailChange)
	f.svc.waitBackground()
	got := f.mapping(2)
	if got.State != OriginLinked || got.LinkedBy != LinkedByEmailChange || got.PasswordState != PasswordSynced {
		t.Errorf("email change link = %+v", got)
	}
	if u, _ := f.fake.User(4); u.Password != testPassword2 {
		t.Error("password not handed to the user linked after the email change")
	}
	// Without a hit nothing is opened for an account that never had a user.
	f.fake.ResetCalls()
	f.exec(`UPDATE accounts SET email = 'nobody@example.test' WHERE id = 1`)
	f.svc.PasswordSeen(f.ctx, 1, "nobody@example.test", testPassword2, MomentEmailChange)
	f.svc.waitBackground()
	if f.mapping(1).State != "none" || f.fake.Calls("POST /user") != 0 {
		t.Errorf("email change without a hit opened a user")
	}
	f.noPasswordAnywhere(testPassword, testPassword2)
}

// The answer to the browser is not delayed by a hanging Seerr and is the same with and without a
// Seerr user of the same email (N-94); a hanging hand-over leaves the account waiting with a job
// without the password.
func TestPasswordSeenDoesNotDelayOrTell(t *testing.T) {
	answers := map[bool]string{}
	for _, known := range []bool{false, true} {
		f := newFixture(t)
		if known {
			f.fake.AddUser(seerrtest.User{ID: 30, Email: "same@example.test", Username: "same", Password: "x"})
		}
		a := f.withAuth()
		inv, err := a.CreateInvitation(f.ctx, model.Account{ID: 1}, model.InvitationRequest{MaxRating: 12, AllLibraries: true})
		if err != nil {
			t.Fatal(err)
		}
		if !known {
			f.fake.SetFailure("*", seerrtest.Failure{Hang: true})
		}
		start := time.Now()
		sess, _, err := a.AcceptInvitation(client(), model.AcceptInvitation{Token: inv.Token, Email: "same@example.test", Password: testPassword, ProfileName: "Neu"}, "ua")
		if err != nil {
			t.Fatal(err)
		}
		// The hanging Seerr holds the hand-over for handOverTimeout; an answer that waited for it
		// would take at least that long. Half of it is the bound, far above any login on a busy
		// machine, so only a waiting answer fails.
		if d := time.Since(start); d >= handOverTimeout/2 {
			t.Errorf("known %v: the answer took %v", known, d)
		}
		sess.ID, sess.Account.ID, sess.Profile.ID, sess.Profile.AccountID = 0, 0, 0, 0
		sess.ExpiresAt = time.Time{}
		b, err := json.Marshal(sess)
		if err != nil {
			t.Fatal(err)
		}
		answers[known] = string(b)
		f.svc.waitBackground()
		if !known {
			if m := f.mapping(4); m.State != StateWaiting && m.State != "none" {
				t.Errorf("hanging seerr = %+v", m)
			}
			if f.queue.count(JobProvision) != 1 {
				t.Errorf("no job for the waiting account: %d", f.queue.count(JobProvision))
			}
		} else if m := f.mapping(4); m.State != OriginLinked || m.LinkedBy != LinkedByInvite {
			t.Errorf("known email = %+v", m)
		}
		f.noPasswordAnywhere(testPassword)
	}
	if answers[false] != answers[true] {
		t.Errorf("the answers differ:\n%s\n%s", answers[false], answers[true])
	}
}
