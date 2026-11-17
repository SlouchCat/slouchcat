package wishes

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// Build step 1.5a: opening and linking the Seerr user of an account, the shared user and the
// deletion of an account (architecture 18.2, 18.4, 18.5; N-90, N-93, N-101, N-103, N-110, N-111).

// runQueued runs the queued jobs of type typ that the fake queue collected and forgets them.
func (f *fixture) runQueued(typ string) int {
	f.t.Helper()
	f.queue.mu.Lock()
	var run []jobs.NewJob
	keep := f.queue.jobs[:0]
	for _, j := range f.queue.jobs {
		if j.Type == typ {
			run = append(run, j)
		} else {
			keep = append(keep, j)
		}
	}
	f.queue.jobs = keep
	f.queue.mu.Unlock()
	for _, j := range run {
		job := &jobs.Job{Type: j.Type, Payload: j.Payload}
		var err error
		if typ == JobProvision {
			err = f.svc.handleProvision(f.ctx, job)
		}
		if err != nil {
			f.t.Fatalf("%s %s: %v", typ, j.Payload, err)
		}
	}
	return len(run)
}

func (f *fixture) mapping(accountID int64) model.WishAccountMapping {
	f.t.Helper()
	a, err := f.svc.accountMapping(f.ctx, accountID)
	if err != nil {
		f.t.Fatal(err)
	}
	return a
}

func bodyKeys(fake *seerrtest.Server, route string) []string {
	for _, c := range fake.CallLog() {
		if c.Route == route {
			return c.BodyKeys
		}
	}
	return nil
}

// «In Seerr eröffnen» with an email Seerr knows in another spelling links at once: no POST /user,
// no password, no rights written, password state off, linked_by admin.
func TestProvisionLinksByEmail(t *testing.T) {
	f := newFixture(t)
	f.fake.UpdateUser(4, func(u *seerrtest.User) { u.Email = "GrossEltern@Example.TEST" })
	a, err := f.svc.ProvisionAccount(f.ctx, 3, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.State != OriginLinked || a.SeerrUserID == nil || *a.SeerrUserID != 4 || a.LinkedBy != LinkedByAdmin || a.LinkedAt == nil || a.PasswordState != PasswordOff {
		t.Errorf("mapping = %+v", a)
	}
	for _, route := range []string{"POST /user", "POST /user/{id}/settings/password", "POST /user/{id}/settings/permissions"} {
		if n := f.fake.Calls(route); n != 0 {
			t.Errorf("%s called %d times", route, n)
		}
	}
	if u, _ := f.fake.User(4); u.Permissions != seerrtest.PermRequest|seerrtest.PermAutoApprove {
		t.Errorf("linked user lost its rights: %d", u.Permissions)
	}
	if got := f.count(`SELECT count(*) FROM wish_account_map WHERE account_id = 3 AND set_by = 1`); got != 1 {
		t.Error("set_by not kept")
	}
	// A second run keeps the link and the rights of a linked user, even without the request right.
	f.fake.ResetCalls()
	if _, err := f.svc.ProvisionAccount(f.ctx, 3, 1, false); err != nil {
		t.Fatal(err)
	}
	if n := f.fake.Calls("POST /user/{id}/settings/permissions"); n != 0 {
		t.Errorf("rights of a linked user written %d times", n)
	}
	// «Alle eröffnen» with a hit in the mirror counts a link and keeps the password state off.
	f.refresh()
	f.fake.UpdateUser(3, func(u *seerrtest.User) { u.Email = "gast@example.test" })
	f.refresh()
	link, create, _, err := f.svc.ProvisionAll(f.ctx, 1)
	if err != nil || link != 1 || create != 1 {
		t.Fatalf("all = %d, %d, %v", link, create, err)
	}
	if n := f.runQueued(JobProvision); n < 2 { // the two accounts, and the shared user the refreshes queued
		t.Errorf("jobs = %d", n)
	}
	if a := f.mapping(2); a.State != OriginLinked || a.PasswordState != PasswordOff || a.LinkedBy != LinkedByAdmin {
		t.Errorf("gast after «Alle eröffnen» = %+v", a)
	}
	if a := f.mapping(1); a.State != OriginCreated || a.PasswordState != PasswordNextLogin {
		t.Errorf("owner after «Alle eröffnen» = %+v", a)
	}
}

// A new user: POST /user, the rights Seerr gives it stay, settings/main goes back with every field
// and the default limit; without the request right the rights are 0, with it again the instance's
// defaultPermissions. Seerr unreachable: waiting, later opened with next_login.
func TestProvisionCreates(t *testing.T) {
	f := newFixture(t)
	f.fake.Configure(func(c *seerrtest.Config) { c.DefaultPermissions = seerrtest.OwnerDefaultPermissions })
	a, err := f.svc.ProvisionAccount(f.ctx, 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.State != OriginCreated || a.SeerrUserID == nil || a.PasswordState != PasswordNextLogin || a.LinkedBy != "" {
		t.Fatalf("mapping = %+v", a)
	}
	u, _ := f.fake.User(*a.SeerrUserID)
	if u.Email != "marco@example.test" || u.Username != "marco" || u.Permissions != seerrtest.OwnerDefaultPermissions || u.Password == "" {
		t.Errorf("seerr user = %+v", u)
	}
	if n := f.fake.Calls("POST /user/{id}/settings/permissions"); n != 0 {
		t.Errorf("rights written %d times", n)
	}
	if n := f.fake.Calls("POST /user/{id}/settings/password"); n != 0 {
		t.Errorf("password set %d times without the account's password", n)
	}
	if u.MovieQuotaLimit == nil || *u.MovieQuotaLimit != 10 || *u.MovieQuotaDays != 7 || *u.TVQuotaLimit != 5 || *u.TVQuotaDays != 7 {
		t.Errorf("limit = %v %v %v %v", u.MovieQuotaLimit, u.MovieQuotaDays, u.TVQuotaLimit, u.TVQuotaDays)
	}
	keys := bodyKeys(f.fake, "POST /user/{id}/settings/main")
	for _, k := range []string{"username", "email", "discordId", "locale", "discoverRegion", "streamingRegion", "originalLanguage",
		"watchlistSyncMovies", "watchlistSyncTv", "globalMovieQuotaLimit", "movieQuotaLimit", "tvQuotaDays"} {
		if !slices.Contains(keys, k) {
			t.Errorf("settings/main sent back without %s: %v", k, keys)
		}
	}
	if got := f.count(`SELECT count(*) FROM wish_account_map WHERE account_id = 1 AND limit_written_at IS NOT NULL`); got != 1 {
		t.Error("limit_written_at not set")
	}

	// Without the request right: permissions 0.
	f.exec(`UPDATE accounts SET can_request = 0 WHERE id = 2`)
	f.fake.ResetCalls()
	b, err := f.svc.ProvisionAccount(f.ctx, 2, 1, false)
	if err != nil || b.State != OriginCreated {
		t.Fatalf("gast = %+v, %v", b, err)
	}
	if gu, _ := f.fake.User(*b.SeerrUserID); gu.Permissions != 0 || f.fake.Calls("POST /user/{id}/settings/permissions") != 1 {
		t.Errorf("rights without the request right = %d", gu.Permissions)
	}
	// The right comes back: the current defaultPermissions from GET /settings/main.
	f.fake.Configure(func(c *seerrtest.Config) { c.DefaultPermissions = seerrtest.PermRequest | seerrtest.PermManageIssues })
	f.exec(`UPDATE accounts SET can_request = 1 WHERE id = 2`)
	f.fake.ResetCalls()
	if _, err := f.svc.ProvisionAccount(f.ctx, 2, 1, false); err != nil {
		t.Fatal(err)
	}
	if gu, _ := f.fake.User(*b.SeerrUserID); gu.Permissions != seerrtest.PermRequest|seerrtest.PermManageIssues || f.fake.Calls("GET /settings/main") != 1 {
		t.Errorf("rights after the right came back = %d", gu.Permissions)
	}
	if got := f.count(`SELECT permissions FROM seerr_users WHERE seerr_id = ?`, *b.SeerrUserID); got != int(seerrtest.PermRequest|seerrtest.PermManageIssues) {
		t.Errorf("mirror permissions = %d", got)
	}
	// The log names neither the email nor a password.
	if strings.Contains(f.logs.String(), "gast@example.test") || strings.Contains(f.logs.String(), "marco@example.test") {
		t.Error("an email reached the log")
	}
}

// 409 on POST /user: read again and link. Seerr unreachable: waiting with the reason; the job and
// the users job try again, the account is then opened with next_login.
func TestProvisionConflictAndWait(t *testing.T) {
	f := newFixture(t)
	f.fake.SetFailure("GET /user", seerrtest.Failure{Status: 200, Times: 1}) // the first search finds nothing
	f.fake.UpdateUser(3, func(u *seerrtest.User) { u.Email = "gast@example.test" })
	a, err := f.svc.ProvisionAccount(f.ctx, 2, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if a.State != OriginLinked || *a.SeerrUserID != 3 || f.fake.Calls("POST /user") != 1 || f.fake.Calls("GET /user") != 2 {
		t.Errorf("after 409 = %+v, POST %d, GET %d", a, f.fake.Calls("POST /user"), f.fake.Calls("GET /user"))
	}

	f.fake.SetFailure("*", seerrtest.Failure{Status: 503, Message: "Something went wrong."})
	w, err := f.svc.ProvisionAccount(f.ctx, 1, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	// Only the class of the error, never Seerr's message (contract WishAccountMapping).
	if w.State != StateWaiting || w.SeerrUserID != nil || w.ProvisionError != "server" {
		t.Errorf("waiting = %+v", w)
	}
	// The job fails, so the queue tries again.
	if err := f.svc.handleProvision(f.ctx, &jobs.Job{Type: JobProvision, Payload: []byte(`{"accountId":1,"intent":"retry","linkedBy":"admin"}`)}); err == nil {
		t.Error("the job did not fail while Seerr is unreachable")
	}
	f.fake.SetFailure("*", seerrtest.Failure{})
	f.refresh() // queues the waiting account
	if n := f.runQueued(JobProvision); n < 1 {
		t.Fatalf("waiting account not queued by the users job")
	}
	if m := f.mapping(1); m.State != OriginCreated || m.PasswordState != PasswordNextLogin || m.ProvisionError != "" {
		t.Errorf("opened later = %+v", m)
	}
}

// Opening is switched off or not set up.
func TestProvisionRefused(t *testing.T) {
	f := newFixture(t)
	f.cfg.set(func(c *fakeConfig) { c.wish.ProvisionAccounts = false })
	if _, err := f.svc.ProvisionAccount(f.ctx, 1, 1, false); !errors.Is(err, model.ErrProvisionDisabled) {
		t.Errorf("switched off: %v", err)
	}
	f.cfg.set(func(c *fakeConfig) { c.wish.ProvisionAccounts = true })
	if _, err := f.svc.ProvisionAccount(f.ctx, 99, 1, false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown account: %v", err)
	}
	f.cfg.set(func(c *fakeConfig) { c.seer.APIKey = "" })
	if _, _, _, err := f.svc.ProvisionAll(f.ctx, 1); !errors.Is(err, model.ErrSeerrNotConfigured) {
		t.Errorf("not configured: %v", err)
	}
}

// An invitation with an email Seerr knows links at once (linked_by invite) and hands over the
// password, except to a sign-in through Emby; user 1 is never linked by an invitation (security
// review H-1); the hint «Neu verknüpft» stays seven days, never after a link by an administrator.
func TestProvisionInviteAndLinkedHint(t *testing.T) {
	f := newFixture(t)
	now := time.Now()
	f.rebuild(WithClock(func() time.Time { return now }))
	f.fake.UpdateUser(3, func(u *seerrtest.User) { u.Email = "gast@example.test"; u.Password = "old-password-1" })
	const pw = "the-real-password-1"
	if _, err := f.svc.provision(f.ctx, 2, LinkedByInvite, pw, 0); err != nil {
		t.Fatal(err)
	}
	m := f.mapping(2)
	if m.State != OriginLinked || m.LinkedBy != LinkedByInvite || m.PasswordState != PasswordSynced || m.PasswordSyncedAt == nil {
		t.Errorf("invite = %+v", m)
	}
	if u, _ := f.fake.User(3); u.Password != pw || f.fake.Calls("POST /user/{id}/settings/password") != 1 {
		t.Errorf("password not handed over: %q", u.Password)
	}
	if r, err := f.svc.ForProfile(f.ctx, 2, 3); err != nil || !r.CanRequest {
		t.Errorf("right after the invitation = %+v, %v", r, err)
	}
	// User 1 (an administrator) waits for the administrator; an Emby user is linked without password.
	f.exec(`UPDATE accounts SET email = 'admin@example.org' WHERE id = 1`)
	f.fake.AddUser(seerrtest.User{ID: 9, Email: "grosseltern@example.test", JellyfinUsername: "oma", UserType: seerrtest.UserEmby, Permissions: seerrtest.PermRequest})
	f.fake.UpdateUser(4, func(u *seerrtest.User) { u.Email = "other@example.test" })
	f.fake.ResetCalls()
	for _, id := range []int64{1, 3} {
		if _, err := f.svc.provision(f.ctx, id, LinkedByInvite, pw, 0); err != nil {
			t.Fatal(err)
		}
	}
	if a := f.mapping(1); a.State != StateWaiting || a.SeerrUserID != nil || a.ProvisionError != ProvisionUserPrivileged {
		t.Errorf("user 1 = %+v", a)
	}
	if a := f.mapping(3); a.State != OriginLinked || *a.SeerrUserID != 9 || a.PasswordState != PasswordExternal {
		t.Errorf("emby = %+v", a)
	}
	if n := f.fake.Calls("POST /user/{id}/settings/password"); n != 0 {
		t.Errorf("password handed to an administrator or an emby user: %d", n)
	}
	hints, err := f.svc.RecentLinks(f.ctx)
	if err != nil || len(hints) != 2 {
		t.Fatalf("hints = %+v, %v", hints, err)
	}
	for _, h := range hints {
		if h.LinkedBy != LinkedByInvite || h.SeerrDisplayName == "" {
			t.Errorf("hint = %+v", h)
		}
	}
	// A link by an administrator never shows.
	f.exec(`DELETE FROM wish_account_map WHERE account_id = 3`)
	f.cfg.set(func(c *fakeConfig) { c.wish.ProvisionAccounts = true })
	if _, err := f.svc.ProvisionAccount(f.ctx, 3, 1, false); err != nil {
		t.Fatal(err)
	}
	if hints, _ := f.svc.RecentLinks(f.ctx); len(hints) != 1 {
		t.Errorf("hints after an administrator's link = %+v", hints)
	}
	now = now.Add(8 * 24 * time.Hour)
	if hints, _ := f.svc.RecentLinks(f.ctx); len(hints) != 0 {
		t.Errorf("hints after eight days = %+v", hints)
	}
	if strings.Contains(f.logs.String(), pw) || strings.Contains(f.logs.String(), "gast@example.test") {
		t.Error("the log carries the password or the email")
	}
}

// The shared user «slouchcat-gaeste»: taken over when Seerr has the email, else opened once with
// the rights Seerr gives it; the default limit is written, no rights. Accounts without a user send
// through it (fallback), not with deny.
func TestSharedUser(t *testing.T) {
	f := newFixture(t)
	f.fake.Configure(func(c *seerrtest.Config) { c.DefaultPermissions = seerrtest.OwnerDefaultPermissions })
	f.fake.RemoveUser(5)
	if err := f.svc.EnsureSharedUser(f.ctx); err != nil {
		t.Fatal(err)
	}
	sh, err := f.svc.SharedUser(f.ctx)
	if err != nil || sh == nil {
		t.Fatalf("shared = %+v, %v", sh, err)
	}
	u, _ := f.fake.User(sh.ID)
	if u.Email != SharedEmail || u.Username != SharedUsername || u.Permissions != seerrtest.OwnerDefaultPermissions || u.Password == "" {
		t.Errorf("shared user in seerr = %+v", u)
	}
	if u.MovieQuotaLimit == nil || *u.MovieQuotaLimit != 10 || *u.TVQuotaLimit != 5 {
		t.Errorf("shared limit = %v %v", u.MovieQuotaLimit, u.TVQuotaLimit)
	}
	if f.fake.Calls("POST /user") != 1 || f.fake.Calls("POST /user/{id}/settings/permissions") != 0 {
		t.Errorf("POST /user %d, permissions %d", f.fake.Calls("POST /user"), f.fake.Calls("POST /user/{id}/settings/permissions"))
	}
	if strings.Contains(f.logs.String(), u.Password) || f.count(`SELECT count(*) FROM settings WHERE value LIKE ?`, "%"+u.Password+"%") != 0 {
		t.Error("the random password was kept")
	}
	// A second run calls nothing.
	f.fake.ResetCalls()
	if err := f.svc.EnsureSharedUser(f.ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(f.fake.CallLog()); n != 0 {
		t.Errorf("second run made %d calls", n)
	}
	// Waiting account with fallback sends through it; deny refuses.
	f.exec(`INSERT INTO wish_account_map (account_id, state, set_at) VALUES (2, 'waiting', 1)`)
	if r, s, _ := f.svc.SenderFor(f.ctx, 2, 3); !r.CanRequest || !s.Shared || s.SeerrUserID != sh.ID {
		t.Errorf("waiting with fallback = %+v %+v", r, s)
	}
	f.cfg.set(func(c *fakeConfig) { c.wish.Unmapped = model.WishesUnmappedDeny })
	if r, _ := f.svc.ForProfile(f.ctx, 2, 3); r.CanRequest || r.Reason != model.WishReasonNoMapping {
		t.Errorf("waiting with deny = %+v", r)
	}
	// Switching «Wünsche aktiv» on queues it; with wishes off it does nothing.
	f.svc.WishSettingsSaved(f.ctx, model.WishSettings{}, model.WishSettings{Enabled: true})
	if n := f.queue.count(JobProvision); n != 1 {
		t.Errorf("queued after switching on = %d", n)
	}
}

// The shared user is taken over when Seerr already has its email.
func TestSharedUserTakenOver(t *testing.T) {
	f := newFixture(t)
	if err := f.svc.EnsureSharedUser(f.ctx); err != nil {
		t.Fatal(err)
	}
	if sh, _ := f.svc.SharedUser(f.ctx); sh == nil || sh.ID != 5 || f.fake.Calls("POST /user") != 0 {
		t.Errorf("shared = %+v, POST /user %d", sh, f.fake.Calls("POST /user"))
	}
	if u, _ := f.fake.User(5); u.Permissions != seerrtest.PermRequestMovie || u.MovieQuotaLimit == nil || *u.MovieQuotaLimit != 10 {
		t.Errorf("taken over = %+v", u)
	}
	f.cfg.set(func(c *fakeConfig) { c.wish.Enabled = false })
	f.exec(`UPDATE seerr_users SET is_shared = 0`)
	f.fake.ResetCalls()
	if err := f.svc.EnsureSharedUser(f.ctx); err != nil || len(f.fake.CallLog()) != 0 {
		t.Errorf("wishes off: %v, %d calls", err, len(f.fake.CallLog()))
	}
}

// Deleting an account: a user Slouchcat opened loses its rights and gets a random password, a
// linked user is not touched; both stay in Seerr (K-2, F-31).
func TestDeleteAccountProvision(t *testing.T) {
	f := newFixture(t)
	f.cfg.set(func(c *fakeConfig) { c.wish.ProvisionAccounts = true })
	a, err := f.svc.ProvisionAccount(f.ctx, 2, 1, false) // opened
	if err != nil || a.State != OriginCreated {
		t.Fatalf("opened = %+v, %v", a, err)
	}
	if _, err := f.svc.ProvisionAccount(f.ctx, 3, 1, false); err != nil { // linked to 4
		t.Fatal(err)
	}
	opened, _ := f.fake.User(*a.SeerrUserID)
	authSvc := auth.New(f.db, f.log).WithAccountDeleting(f.svc.AccountDeleting)
	f.fake.ResetCalls()
	if err := authSvc.DeleteAccount(f.ctx, 3, nil); err != nil {
		t.Fatal(err)
	}
	if n := f.queue.count(JobProvision); n != 0 {
		t.Errorf("linked user: %d jobs", n)
	}
	if err := authSvc.DeleteAccount(f.ctx, 2, nil); err != nil {
		t.Fatal(err)
	}
	if n := f.runQueued(JobProvision); n != 1 {
		t.Fatalf("opened user: %d jobs", n)
	}
	u, ok := f.fake.User(opened.ID)
	if !ok || u.Permissions != 0 || u.Password == opened.Password || u.Password == "" {
		t.Errorf("opened user after deletion = %+v", u)
	}
	if l, ok := f.fake.User(4); !ok || l.Permissions != seerrtest.PermRequest|seerrtest.PermAutoApprove {
		t.Errorf("linked user after deletion = %+v", l)
	}
	for _, c := range f.fake.CallLog() {
		if strings.Contains(c.Path, "/user/4") {
			t.Errorf("call for the linked user: %s", c.Route)
		}
	}
	if n := f.fake.Calls("DELETE /user/{id}"); n != 0 {
		t.Errorf("a user was deleted in seerr")
	}
}

// A provisioning job never carries a password.
func TestProvisionPayloadWithoutPassword(t *testing.T) {
	f := newFixture(t)
	f.cfg.set(func(c *fakeConfig) { c.wish.ProvisionAccounts = true })
	if _, _, _, err := f.svc.ProvisionAll(f.ctx, 1); err != nil {
		t.Fatal(err)
	}
	f.queue.mu.Lock()
	defer f.queue.mu.Unlock()
	for _, j := range f.queue.jobs {
		var p map[string]any
		if err := json.Unmarshal(j.Payload, &p); err != nil {
			t.Fatal(err)
		}
		for k := range p {
			if !slices.Contains([]string{"accountId", "intent", "seerrUser", "linkedBy", "setBy", "shared", "closeSeerr"}, k) {
				t.Errorf("payload key %s", k)
			}
		}
	}
}

// The job that follows a hand choice only writes the limit to the chosen user: undone or changed
// before it runs, it does nothing; it never searches by the email and never opens a user. A retry
// does nothing once the mapping no longer waits, and an open job nothing once «Neue Konten in Seerr
// eröffnen» is off.
func TestProvisionJobOfHandChoiceNeverOpens(t *testing.T) {
	f := newFixture(t)
	f.cfg.set(func(c *fakeConfig) { c.wish.ProvisionAccounts = true })
	f.refresh()
	f.runQueued(JobProvision) // the shared user the refresh queued
	three, four := 3, 4
	if _, err := f.svc.SetAccountMapping(f.ctx, 2, &three, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SetAccountMapping(f.ctx, 2, nil, 1); err != nil { // undone before the job runs
		t.Fatal(err)
	}
	f.fake.ResetCalls()
	if n := f.runQueued(JobProvision); n != 1 {
		t.Fatalf("jobs = %d", n)
	}
	if n := len(f.fake.CallLog()); n != 0 {
		t.Errorf("undone hand choice: %d calls to seerr, first %+v", n, f.fake.CallLog()[0])
	}
	if got := f.count(`SELECT count(*) FROM wish_account_map WHERE account_id = 2`); got != 0 {
		t.Errorf("undone hand choice left a mapping")
	}

	// Changed to another user: the job of the first choice does nothing, the second one writes the
	// limit to user 4 only.
	if _, err := f.svc.SetAccountMapping(f.ctx, 2, &three, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SetAccountMapping(f.ctx, 2, &four, 1); err != nil {
		t.Fatal(err)
	}
	f.fake.ResetCalls()
	if n := f.runQueued(JobProvision); n != 2 {
		t.Fatalf("jobs = %d", n)
	}
	for _, c := range f.fake.CallLog() {
		if strings.Contains(c.Path, "/user/3") || c.Route == "POST /user" || c.Route == "GET /user" {
			t.Errorf("changed hand choice: call %s %s", c.Route, c.Path)
		}
	}
	if f.fake.Calls("POST /user/{id}/settings/main") != 1 {
		t.Errorf("limit of user 4 written %d times", f.fake.Calls("POST /user/{id}/settings/main"))
	}
	if got := f.count(`SELECT count(*) FROM wish_account_map WHERE account_id = 2 AND seerr_user_id = 4 AND limit_written_at IS NOT NULL`); got != 1 {
		t.Errorf("limit of the chosen user not recorded")
	}

	// A retry of a waiting account that was undone meanwhile does nothing.
	f.fake.SetFailure("*", seerrtest.Failure{Status: 503})
	if w, err := f.svc.ProvisionAccount(f.ctx, 1, 1, false); err != nil || w.State != StateWaiting {
		t.Fatalf("waiting = %+v, %v", w, err)
	}
	f.fake.SetFailure("*", seerrtest.Failure{})
	if err := f.svc.queueProvision(f.ctx, provisionPayload{AccountID: 1, Intent: intentRetry, LinkedBy: LinkedByAdmin}, provisionPriority); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SetAccountMapping(f.ctx, 1, nil, 1); err != nil {
		t.Fatal(err)
	}
	f.fake.ResetCalls()
	f.runQueued(JobProvision)
	if n := f.fake.Calls("POST /user") + f.fake.Calls("GET /user"); n != 0 {
		t.Errorf("undone waiting account: %d searches or creations", n)
	}

	// «Alle eröffnen», then the switch goes off before the jobs run: nobody is opened.
	if _, _, _, err := f.svc.ProvisionAll(f.ctx, 1); err != nil {
		t.Fatal(err)
	}
	f.cfg.set(func(c *fakeConfig) { c.wish.ProvisionAccounts = false })
	f.fake.ResetCalls()
	if n := f.runQueued(JobProvision); n == 0 {
		t.Fatal("«Alle eröffnen» queued nothing")
	}
	if n := f.fake.Calls("POST /user") + f.fake.Calls("GET /user"); n != 0 {
		t.Errorf("switched off: %d searches or creations", n)
	}
}
