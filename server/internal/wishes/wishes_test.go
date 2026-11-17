package wishes

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/storetest"
)

// The fixture: a store with three accounts (1 the owner with the primary profile 1 and the child
// profile 2, 2 a guest with the primary profile 3, 3 a second guest without the right), a fake
// Seerr with the users 3 kinder, 4 grosseltern and 5 slouchcat-gaeste beside user 1, and a fake
// administration.

type fakeConfig struct {
	mu   sync.Mutex
	seer model.SeerrConfig
	wish model.WishSettings
}

func (c *fakeConfig) SeerrConfig(context.Context) (model.SeerrConfig, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seer, nil
}

func (c *fakeConfig) WishSettings(context.Context) (model.WishSettings, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.wish, nil
}

func (c *fakeConfig) set(f func(*fakeConfig)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f(c)
}

type fakeQueue struct {
	mu   sync.Mutex
	jobs []jobs.NewJob
}

func (q *fakeQueue) Enqueue(_ context.Context, j jobs.NewJob) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, j)
	return int64(len(q.jobs)), nil
}

func (q *fakeQueue) RaiseQueuedPriority(context.Context, int64, int) (bool, error) { return false, nil }

func (q *fakeQueue) count(typ string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, j := range q.jobs {
		if j.Type == typ {
			n++
		}
	}
	return n
}

type fixture struct {
	t     *testing.T
	ctx   context.Context
	db    *store.DB
	fake  *seerrtest.Server
	cfg   *fakeConfig
	queue *fakeQueue
	svc   *Service
	logs  *bytes.Buffer
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	var logs bytes.Buffer
	log, err := logging.New(&logs, logging.FormatJSON, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	db, err := storetest.Open(ctx, store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fake := seerrtest.New(t)
	fake.AddUser(seerrtest.User{ID: 3, Email: "kinder@example.test", Username: "kinder", Permissions: seerrtest.PermRequest})
	fake.AddUser(seerrtest.User{ID: 4, Email: "grosseltern@example.test", Username: "grosseltern", Permissions: seerrtest.PermRequest | seerrtest.PermAutoApprove})
	fake.AddUser(seerrtest.User{ID: 5, Email: "slouchcat-gaeste@slouchcat.invalid", Username: "slouchcat-gaeste", Permissions: seerrtest.PermRequestMovie})
	f := &fixture{t: t, ctx: ctx, db: db, fake: fake, queue: &fakeQueue{}, logs: &logs,
		cfg: &fakeConfig{seer: model.SeerrConfig{Enabled: true, URL: fake.URL, APIKey: seerrtest.APIKey},
			wish: model.WishSettings{Enabled: true, AllowMovies: true, AllowShows: true, Unmapped: model.WishesUnmappedFallback,
				DefaultLimit: model.WishLimits{Movie: model.WishLimit{Limit: 10, Days: 7}, TV: model.WishLimit{Limit: 5, Days: 7}}}}}
	f.svc = New(db, f.cfg, f.queue, log, WithClientOptions(seerr.WithTimeout(2*time.Second), seerr.WithLimiter(ratelimit.New("wishes-test", 1000, 100))))
	f.exec(`INSERT INTO accounts (id, email, password_hash, role, is_owner, created_at, updated_at, can_request)
	        VALUES (1, 'marco@example.test', 'x', 'admin', 1, 1, 1, 1), (2, 'gast@example.test', 'x', 'user', 0, 1, 1, 1),
	               (3, 'grosseltern@example.test', 'x', 'user', 0, 1, 1, 0)`)
	f.exec(`INSERT INTO profiles (id, account_id, name, is_default, created_at, updated_at, can_request)
	        VALUES (1, 1, 'Marco', 1, 1, 1, 1), (2, 1, 'Noah', 0, 1, 1, 0), (3, 2, 'Gast', 1, 1, 1, 1), (4, 3, 'Oma', 1, 1, 1, 1)`)
	return f
}

func (f *fixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.db.Write.ExecContext(f.ctx, query, args...); err != nil {
		f.t.Fatalf("%s: %v", query, err)
	}
}

func (f *fixture) count(query string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.db.Read.QueryRowContext(f.ctx, query, args...).Scan(&n); err != nil {
		f.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// refresh runs one seerr.users job.
func (f *fixture) refresh() {
	f.t.Helper()
	if err := f.svc.handleUsers(f.ctx, &jobs.Job{Type: JobUsers}); err != nil {
		f.t.Fatalf("seerr.users: %v", err)
	}
}

func check(res model.SeerrTestResult, key string) (model.SeerrCheck, bool) {
	for _, c := range res.Checks {
		if c.Key == key {
			return c, true
		}
	}
	return model.SeerrCheck{}, false
}

func apiUserCalls(fake *seerrtest.Server) int {
	n := 0
	for _, c := range fake.CallLog() {
		if c.HasAPIUser {
			n++
		}
	}
	return n
}

// The refresh mirrors every user with type, rights and the limit fields as Seerr holds them, marks
// administrators, keeps the fingerprint and logs each call without key.
func TestUsersRefreshMirrorsSeerr(t *testing.T) {
	f := newFixture(t)
	limit, days := 3, 14
	f.fake.UpdateUser(4, func(u *seerrtest.User) { u.MovieQuotaLimit, u.MovieQuotaDays = &limit, &days })
	f.fake.AddUser(seerrtest.User{ID: 6, Email: "emby@example.test", JellyfinUsername: "emby", UserType: seerrtest.UserEmby, Permissions: seerrtest.PermRequest})
	f.refresh()
	for query, want := range map[string]int{
		`SELECT count(*) FROM seerr_users WHERE gone_at IS NULL`:                                                                             5,
		`SELECT count(*) FROM seerr_users WHERE seerr_id = 1 AND is_admin = 1 AND user_type = 3`:                                             1,
		`SELECT count(*) FROM seerr_users WHERE seerr_id = 3 AND is_admin = 0 AND user_type = 2 AND permissions = 32`:                        1,
		`SELECT count(*) FROM seerr_users WHERE seerr_id = 4 AND movie_quota_limit = 3 AND movie_quota_days = 14 AND tv_quota_limit IS NULL`: 1,
		`SELECT count(*) FROM seerr_users WHERE seerr_id = 6 AND user_type = 4 AND display_name = 'emby'`:                                    1,
		`SELECT count(*) FROM seerr_users WHERE created_at IS NULL`:                                                                          0,
		`SELECT count(*) FROM settings WHERE key = 'seerr.instance' AND value LIKE '%admin@example.org%'`:                                    1,
		`SELECT count(*) FROM seerr_log WHERE method = 'GET' AND path = '/user' AND status = 200`:                                            1,
		`SELECT count(*) FROM settings WHERE key = 'seerr.last_success'`:                                                                     1,
	} {
		if got := f.count(query); got != want {
			t.Errorf("%s = %d, want %d", query, got, want)
		}
	}
	if strings.Contains(f.logs.String(), seerrtest.APIKey) || f.count(`SELECT count(*) FROM seerr_log WHERE error LIKE ?`, "%"+seerrtest.APIKey+"%") != 0 {
		t.Error("the api key reached a log")
	}
	if n := apiUserCalls(f.fake); n != 0 {
		t.Errorf("the refresh sent %d calls on behalf of a user", n)
	}
	users, err := f.svc.SeerrUsers(f.ctx)
	if err != nil || len(users) != 5 {
		t.Fatalf("users = %v, %v", users, err)
	}
	if u := users[2]; u.ID != 4 || !u.CanRequestMovies || !u.AutoApprove || u.Quota.MovieLimit == nil || *u.Quota.MovieLimit != 3 || u.Quota.TVLimit != nil {
		t.Errorf("user 4 = %+v", u)
	}
	// Switched off: no call at all.
	f.cfg.set(func(c *fakeConfig) { c.seer.Enabled = false })
	f.fake.ResetCalls()
	f.refresh()
	if n := len(f.fake.CallLog()); n != 0 {
		t.Errorf("switched off: %d calls", n)
	}
}

// A user deleted in Seerr is marked gone; its account keeps the mapping but loses the right
// (seerr_user_gone). Coming back clears the mark.
func TestUsersGoneAndRights(t *testing.T) {
	f := newFixture(t)
	f.refresh()
	three := 3
	if _, err := f.svc.SetAccountMapping(f.ctx, 2, &three, 1); err != nil {
		t.Fatal(err)
	}
	rights, sender, err := f.svc.SenderFor(f.ctx, 2, 3)
	if err != nil || !rights.CanRequest || sender.SeerrUserID != 3 || sender.Shared || !rights.Movies || !rights.Shows {
		t.Fatalf("before: %+v %+v %v", rights, sender, err)
	}
	f.fake.RemoveUser(3)
	f.fake.Advance(time.Minute)
	f.svc.now = func() time.Time { return time.Now().Add(time.Minute) }
	f.refresh()
	if got := f.count(`SELECT count(*) FROM seerr_users WHERE seerr_id = 3 AND gone_at IS NOT NULL`); got != 1 {
		t.Errorf("gone_at of user 3 not set")
	}
	if got := f.count(`SELECT count(*) FROM wish_account_map WHERE account_id = 2 AND seerr_user_id = 3`); got != 1 {
		t.Errorf("the mapping of a gone user went away")
	}
	if rights, _ := f.svc.ForProfile(f.ctx, 2, 3); rights.CanRequest || rights.Reason != model.WishReasonSeerrUserGone || !rights.Enabled {
		t.Errorf("gone user: %+v, want seerr_user_gone", rights)
	}
	users, _ := f.svc.SeerrUsers(f.ctx)
	for _, u := range users {
		if u.ID == 3 && !u.Gone {
			t.Error("user 3 not shown as gone")
		}
	}
	// A gone user cannot be chosen.
	one := 3
	if _, err := f.svc.SetAccountMapping(f.ctx, 3, &one, 1); !errors.Is(err, model.ErrSeerrUserTaken) {
		t.Errorf("choosing a gone user: %v", err)
	}
}

// Choosing by hand: one account per user (409 seerr_user_taken), never the shared user, the
// password of a chosen user stays in Seerr ('off', 'admin' for an administrator), unlinking keeps
// the user.
func TestMappingByHand(t *testing.T) {
	f := newFixture(t)
	f.refresh()
	if err := f.svc.MarkShared(f.ctx, 5); err != nil {
		t.Fatal(err)
	}
	three, five, one, nine := 3, 5, 1, 99
	m, err := f.svc.SetAccountMapping(f.ctx, 2, &three, 1)
	if err != nil || m.State != OriginManual || m.SeerrUserID == nil || *m.SeerrUserID != 3 || m.PasswordState != PasswordOff || m.LinkedBy != "" {
		t.Fatalf("by hand: %+v, %v", m, err)
	}
	if _, err := f.svc.SetAccountMapping(f.ctx, 3, &three, 1); !errors.Is(err, model.ErrSeerrUserTaken) {
		t.Errorf("a user of another account: %v, want seerr_user_taken", err)
	}
	if _, err := f.svc.SetAccountMapping(f.ctx, 3, &five, 1); !errors.Is(err, model.ErrSeerrUserTaken) {
		t.Errorf("the shared user: %v, want seerr_user_taken", err)
	}
	if err := f.svc.MarkShared(f.ctx, 3); !errors.Is(err, model.ErrSeerrUserTaken) {
		t.Errorf("marking a mapped user shared: %v", err)
	}
	if shared, _ := f.svc.SharedUser(f.ctx); shared == nil || shared.ID != 5 {
		t.Errorf("a refused MarkShared changed the shared user: %+v", shared)
	}
	if _, err := f.svc.SetAccountMapping(f.ctx, 3, &nine, 1); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown user: %v", err)
	}
	if _, err := f.svc.SetAccountMapping(f.ctx, 42, &three, 1); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown account: %v", err)
	}
	if m, err := f.svc.SetAccountMapping(f.ctx, 1, &one, 1); err != nil || m.PasswordState != PasswordAdmin {
		t.Errorf("user 1: %+v, %v", m, err)
	}
	// The same user again for the same account is fine.
	if _, err := f.svc.SetAccountMapping(f.ctx, 2, &three, 1); err != nil {
		t.Errorf("same user again: %v", err)
	}
	all, err := f.svc.WishMappings(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 3 || all.Opened != 2 || all.SharedUser == nil || all.SharedUser.ID != 5 || len(all.UnlinkedUsers) != 1 || all.UnlinkedUsers[0].ID != 4 {
		t.Errorf("mappings = %+v", all)
	}
	if a := all.Accounts[2]; a.State != model.WishMappingNone || a.Limit.Mode != model.WishLimitDefault || a.Limit.Effective.Movie.Limit != 10 {
		t.Errorf("account 3 = %+v", a)
	}
	if a := all.Accounts[0]; !a.Limit.SeerrAdmin {
		t.Errorf("account 1 on user 1 = %+v, want SeerrAdmin", a.Limit)
	}
	if m, err := f.svc.SetAccountMapping(f.ctx, 2, nil, 1); err != nil || m.State != model.WishMappingNone {
		t.Errorf("unlink: %+v, %v", m, err)
	}
	if got := f.count(`SELECT count(*) FROM seerr_users WHERE seerr_id = 3`); got != 1 {
		t.Error("unlinking deleted the user")
	}
	if n := apiUserCalls(f.fake); n != 0 {
		t.Errorf("the mapping sent %d calls to Seerr on behalf of a user", n)
	}
}

// The right of a profile: wishes off, the cap of the account, the switch of the profile, the user
// of the account, the shared user or none.
func TestRightsOfProfiles(t *testing.T) {
	f := newFixture(t)
	f.refresh()
	for _, c := range []struct {
		name               string
		account, profile   int64
		wantRequest        bool
		wantReason         string
		wantShared         bool
		wantMovies, wantTV bool
	}{
		{"no user, no shared user", 1, 1, false, model.WishReasonNoMapping, false, false, false},
		{"child profile", 1, 2, false, model.WishReasonProfileNotAllowed, false, false, false},
		{"account without right", 3, 4, false, model.WishReasonAccountNotAllowed, false, false, false},
	} {
		r, s, err := f.svc.SenderFor(f.ctx, c.account, c.profile)
		if err != nil || r.CanRequest != c.wantRequest || r.Reason != c.wantReason || s.Shared != c.wantShared {
			t.Errorf("%s: %+v %+v %v", c.name, r, s, err)
		}
	}
	if _, err := f.svc.ForProfile(f.ctx, 2, 1); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("a profile of another account: %v", err)
	}
	// The shared user (F-4, N-101): only films for this user; shows follow the rights in Seerr.
	if err := f.svc.MarkShared(f.ctx, 5); err != nil {
		t.Fatal(err)
	}
	r, s, err := f.svc.SenderFor(f.ctx, 1, 1)
	if err != nil || !r.CanRequest || !s.Shared || s.SeerrUserID != 5 || !r.Movies || r.Shows {
		t.Errorf("shared: %+v %+v %v", r, s, err)
	}
	f.cfg.set(func(c *fakeConfig) { c.wish.Unmapped = model.WishesUnmappedDeny })
	if r, _ := f.svc.ForProfile(f.ctx, 1, 1); r.CanRequest || r.Reason != model.WishReasonNoMapping {
		t.Errorf("deny: %+v", r)
	}
	// The user of the account wins over the shared user; a waiting account uses the shared user.
	f.cfg.set(func(c *fakeConfig) { c.wish.Unmapped = model.WishesUnmappedFallback; c.wish.AllowShows = false })
	four := 4
	if _, err := f.svc.SetAccountMapping(f.ctx, 1, &four, 1); err != nil {
		t.Fatal(err)
	}
	if r, s, _ := f.svc.SenderFor(f.ctx, 1, 1); !r.CanRequest || s.SeerrUserID != 4 || s.Shared || !r.Movies || r.Shows {
		t.Errorf("own user, shows off: %+v %+v", r, s)
	}
	f.exec(`INSERT INTO wish_account_map (account_id, state, set_at) VALUES (2, 'waiting', 1)`)
	if _, s, _ := f.svc.SenderFor(f.ctx, 2, 3); !s.Shared {
		t.Errorf("waiting account: %+v, want the shared user", s)
	}
	// Wishes off or Seerr off: disabled.
	f.cfg.set(func(c *fakeConfig) { c.wish.Enabled = false })
	if r, _ := f.svc.ForProfile(f.ctx, 1, 1); r.Enabled || r.CanRequest || r.Reason != model.WishReasonDisabled {
		t.Errorf("wishes off: %+v", r)
	}
	f.cfg.set(func(c *fakeConfig) { c.wish.Enabled = true; c.seer.Enabled = false })
	if r, _ := f.svc.ForProfile(f.ctx, 1, 1); r.CanRequest || r.Reason != model.WishReasonDisabled {
		t.Errorf("seerr off: %+v", r)
	}
	// Seerr unreachable keeps the right (the wish waits).
	f.cfg.set(func(c *fakeConfig) { c.seer.Enabled = true })
	f.fake.SetFailure("*", seerrtest.Failure{Status: 503})
	if r, _ := f.svc.ForProfile(f.ctx, 1, 1); !r.CanRequest {
		t.Errorf("unreachable: %+v, want the right", r)
	}
}

// The check list against a healthy fake: every check ok, acting as user 3 proven, the record of
// the test opens «Wünsche aktiv», and the test creates nothing in Seerr.
func TestInstanceConnectionCheckList(t *testing.T) {
	f := newFixture(t)
	f.fake.Configure(func(c *seerrtest.Config) {
		c.MetadataTV = "tvdb"
		c.Radarr = []seerrtest.DVR{{ID: 0, Name: "Radarr", Hostname: "radarr", Port: 7878, APIKey: "radarr-FAKE"}}
		c.Sonarr = []seerrtest.DVR{{ID: 0, Name: "Sonarr", Hostname: "sonarr", Port: 8989, APIKey: "sonarr-FAKE"}}
	})
	if err := f.svc.CanEnableWishes(f.ctx); !errors.Is(err, model.ErrSeerrNotTested) {
		t.Errorf("before a test: %v", err)
	}
	res, err := f.svc.TestConnection(f.ctx, model.SeerrTestInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.Version != "3.5.2" {
		t.Errorf("result = %+v", res)
	}
	for _, key := range []string{model.SeerrCheckReachable, model.SeerrCheckVersion, model.SeerrCheckKeyValid, model.SeerrCheckManageUsers,
		model.SeerrCheckInstance, model.SeerrCheckActAsUser, model.SeerrCheckCSRFOff, model.SeerrCheckRadarr, model.SeerrCheckSonarr,
		model.SeerrCheckLocalLogin, model.SeerrCheckTVDBSeasons} {
		if c, ok := check(res, key); !ok || c.Level != model.SeerrLevelOK {
			t.Errorf("%s = %+v (%v), want ok", key, c, ok)
		}
	}
	// The fake runs on 127.0.0.1: the password stays in the household's network, no line.
	if c, ok := check(res, model.SeerrCheckPasswordTransport); ok {
		t.Errorf("password_transport on a local address: %+v", c)
	}
	for _, c := range f.fake.CallLog() {
		if c.HasAPIUser && c.APIUser != "3" {
			t.Errorf("acted as user %s, want 3", c.APIUser)
		}
	}
	if f.fake.Calls("POST /user") != 0 || len(f.fake.Requests()) != 0 {
		t.Error("the test created something in Seerr")
	}
	if err := f.svc.CanEnableWishes(f.ctx); err != nil {
		t.Errorf("after a passed test: %v", err)
	}
	st, err := f.svc.Status(f.ctx)
	if err != nil || st.State != model.SeerrStateConnected || st.Version != "3.5.2" || !st.Tested || st.InstanceChanged || len(st.Log) == 0 {
		t.Errorf("status = %+v, %v", st, err)
	}
	// An old version warns but does not fail; the key refused fails and closes «Wünsche aktiv».
	f.fake.Configure(func(c *seerrtest.Config) { c.Version = "2.5.0" })
	res, _ = f.svc.TestConnection(f.ctx, model.SeerrTestInput{})
	if c, _ := check(res, model.SeerrCheckVersion); !res.OK || c.Level != model.SeerrLevelWarn || c.Detail != model.SeerrDetailNotVerified {
		t.Errorf("old version: %+v", res)
	}
	f.cfg.set(func(c *fakeConfig) { c.seer.APIKey = "wrong-key" })
	res, _ = f.svc.TestConnection(f.ctx, model.SeerrTestInput{})
	if c, _ := check(res, model.SeerrCheckKeyValid); res.OK || c.Level != model.SeerrLevelFail {
		t.Errorf("wrong key: %+v", res)
	}
	if err := f.svc.CanEnableWishes(f.ctx); !errors.Is(err, model.ErrSeerrNotTested) {
		t.Errorf("after a failed test: %v", err)
	}
	if st, _ := f.svc.Status(f.ctx); st.State != model.SeerrStateKeyInvalid {
		t.Errorf("state = %s, want key_invalid", st.State)
	}
}

// Warnings never fail the test (administration 4.1): local login off, seasons from TMDB, no Sonarr,
// only user 1 to act as.
func TestInstanceWarningsDoNotFail(t *testing.T) {
	f := newFixture(t)
	f.fake.Configure(func(c *seerrtest.Config) { c.LocalLogin = false; c.MetadataTV = "tmdb" })
	for _, id := range []int{3, 4, 5} {
		f.fake.RemoveUser(id)
	}
	res, err := f.svc.TestConnection(f.ctx, model.SeerrTestInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("warnings failed the test: %+v", res)
	}
	for key, detail := range map[string]string{
		model.SeerrCheckLocalLogin: model.SeerrDetailOff, model.SeerrCheckTVDBSeasons: "tmdb",
		model.SeerrCheckSonarr: "0", model.SeerrCheckActAsUser: model.SeerrDetailNotTestable,
	} {
		if c, _ := check(res, key); c.Level != model.SeerrLevelWarn || c.Detail != detail {
			t.Errorf("%s = %+v, want warn %s", key, c, detail)
		}
	}
	if err := f.svc.CanEnableWishes(f.ctx); err != nil {
		t.Errorf("warnings closed «Wünsche aktiv»: %v", err)
	}
	// CSRF protection on is a failure; acting as a user does not count for T-1 then, «Wünsche aktiv»
	// stays possible (the checks of T-1 passed).
	f.fake.SetCSRF(true)
	res, _ = f.svc.TestConnection(f.ctx, model.SeerrTestInput{})
	if c, _ := check(res, model.SeerrCheckCSRFOff); res.OK || c.Level != model.SeerrLevelFail {
		t.Errorf("csrf on: %+v", res)
	}
	// Unreachable: only the first check, failed.
	f.fake.SetFailure("*", seerrtest.Failure{Status: 302, Location: "http://elsewhere.invalid/"})
	res, _ = f.svc.TestConnection(f.ctx, model.SeerrTestInput{})
	if len(res.Checks) != 1 || res.Checks[0] != (model.SeerrCheck{Key: model.SeerrCheckReachable, Level: model.SeerrLevelFail, Detail: model.SeerrDetailRedirect}) {
		t.Errorf("redirect: %+v", res)
	}
	if st, _ := f.svc.Status(f.ctx); st.State != model.SeerrStateUnreachable || st.LastError != "redirect" {
		t.Errorf("status = %s %s", st.State, st.LastError)
	}
}

// A test of values not saved yet: another host needs the key (422 seerr_key_required); the saved
// key never travels there; such a test neither counts for T-1 nor touches the fingerprint.
func TestInstanceNewHostNeedsKey(t *testing.T) {
	f := newFixture(t)
	other := seerrtest.New(t)
	if _, err := f.svc.TestConnection(f.ctx, model.SeerrTestInput{URL: other.URL}); !errors.Is(err, model.ErrSeerrKeyRequired) {
		t.Errorf("new host without key: %v, want ErrSeerrKeyRequired", err)
	}
	if n := len(other.CallLog()); n != 0 {
		t.Errorf("the other host got %d calls", n)
	}
	var verr *model.ValidationError
	if _, err := f.svc.TestConnection(f.ctx, model.SeerrTestInput{URL: "http://user:pw@seerr.lan"}); !errors.As(err, &verr) {
		t.Errorf("bad address: %v", err)
	}
	f.cfg.set(func(c *fakeConfig) { c.seer = model.SeerrConfig{} })
	if _, err := f.svc.TestConnection(f.ctx, model.SeerrTestInput{}); !errors.Is(err, model.ErrSeerrNotConfigured) {
		t.Errorf("nothing saved: %v", err)
	}
	f.cfg.set(func(c *fakeConfig) {
		c.seer = model.SeerrConfig{Enabled: true, URL: f.fake.URL, APIKey: seerrtest.APIKey}
	})
	res, err := f.svc.TestConnection(f.ctx, model.SeerrTestInput{URL: other.URL, APIKey: seerrtest.APIKey})
	if err != nil || !res.OK {
		t.Fatalf("other host with key: %+v, %v", res, err)
	}
	if got := f.count(`SELECT count(*) FROM settings WHERE key IN ('seerr.test', 'seerr.instance')`); got != 0 {
		t.Errorf("a test of unsaved values stored %d state rows", got)
	}
	// The same host with the path given explicitly is the saved connection.
	if _, err := f.svc.TestConnection(f.ctx, model.SeerrTestInput{URL: f.fake.URL + "/api/v1/"}); err != nil {
		t.Errorf("same host without key: %v", err)
	}
	if err := f.svc.CanEnableWishes(f.ctx); err != nil {
		t.Errorf("a test of the saved values: %v", err)
	}
}

// Another instance behind the address (architecture 7.4): the refresh stops before writing, the
// test says «other instance detected», no call runs on behalf of a user until the administrator
// confirms; reset drops every mapping.
func TestInstanceChangeStopsUserCalls(t *testing.T) {
	f := newFixture(t)
	f.refresh()
	three := 3
	if _, err := f.svc.SetAccountMapping(f.ctx, 2, &three, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.TestConnection(f.ctx, model.SeerrTestInput{}); err != nil {
		t.Fatal(err)
	}
	// A new instance under the same address: user 1 created later, user 3 is someone else.
	f.fake.UpdateUser(1, func(u *seerrtest.User) { u.CreatedAt = u.CreatedAt.Add(72 * time.Hour); u.Email = "root@other.example" })
	f.fake.RemoveUser(3)
	f.fake.AddUser(seerrtest.User{ID: 3, Email: "fremd@other.example", Username: "fremd", Permissions: seerrtest.PermRequest,
		CreatedAt: seerrtest.Start.Add(time.Hour)})
	f.fake.ResetCalls()
	f.refresh()
	if got := f.count(`SELECT count(*) FROM seerr_users WHERE seerr_id = 3 AND email = 'kinder@example.test' AND gone_at IS NULL`); got != 1 {
		t.Error("the refresh of another instance wrote the mirror")
	}
	if n := apiUserCalls(f.fake); n != 0 {
		t.Errorf("%d calls on behalf of a user", n)
	}
	st, err := f.svc.Status(f.ctx)
	if err != nil || !st.InstanceChanged || st.State != model.SeerrStateInstanceChanged || st.Tested {
		t.Errorf("status = %+v, %v", st, err)
	}
	res, err := f.svc.TestConnection(f.ctx, model.SeerrTestInput{})
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := check(res, model.SeerrCheckInstance); c.Level != model.SeerrLevelFail || c.Detail != model.SeerrDetailInstanceChanged {
		t.Errorf("instance check = %+v", c)
	}
	if c, _ := check(res, model.SeerrCheckActAsUser); c.Level != model.SeerrLevelWarn || c.Detail != model.SeerrDetailInstanceChanged {
		t.Errorf("act_as_user = %+v", c)
	}
	if n := apiUserCalls(f.fake); n != 0 {
		t.Errorf("the test sent %d calls on behalf of a user", n)
	}
	if _, err := f.svc.UserSession(f.ctx, 3); !errors.Is(err, model.ErrInstanceChanged) {
		t.Errorf("UserSession = %v, want ErrInstanceChanged", err)
	}
	if err := f.svc.CanEnableWishes(f.ctx); !errors.Is(err, model.ErrSeerrNotTested) {
		t.Errorf("CanEnableWishes = %v", err)
	}
	if r, _ := f.svc.ForProfile(f.ctx, 2, 3); !r.CanRequest {
		t.Errorf("another instance took the right: %+v", r)
	}
	if _, err := f.svc.ConfirmInstance(f.ctx, "maybe"); err == nil {
		t.Error("an unknown action passed")
	}
	// Reset: every mapping and the old mirror go, the new fingerprint holds, the refresh is queued.
	st, err = f.svc.ConfirmInstance(f.ctx, InstanceReset)
	if err != nil || st.InstanceChanged {
		t.Fatalf("reset: %+v, %v", st, err)
	}
	if got := f.count(`SELECT count(*) FROM wish_account_map`) + f.count(`SELECT count(*) FROM seerr_users`); got != 0 {
		t.Errorf("reset kept %d rows", got)
	}
	if f.queue.count(JobUsers) != 1 {
		t.Errorf("reset queued %d refreshes, want 1", f.queue.count(JobUsers))
	}
	if _, err := f.svc.ConfirmInstance(f.ctx, InstanceConfirm); !errors.Is(err, model.ErrInstanceUnchanged) {
		t.Errorf("nothing pending: %v", err)
	}
	if _, err := f.svc.UserSession(f.ctx, 3); err != nil {
		t.Errorf("after the reset: %v", err)
	}
	f.refresh()
	if got := f.count(`SELECT count(*) FROM seerr_users WHERE seerr_id = 3 AND email = 'fremd@other.example'`); got != 1 {
		t.Error("the refresh after the reset did not load the new instance")
	}
}

// Confirming keeps the mappings and takes over the new fingerprint.
func TestInstanceConfirmKeepsMappings(t *testing.T) {
	f := newFixture(t)
	f.refresh()
	four := 4
	if _, err := f.svc.SetAccountMapping(f.ctx, 1, &four, 1); err != nil {
		t.Fatal(err)
	}
	f.fake.UpdateUser(1, func(u *seerrtest.User) { u.Email = "owner@new.example" })
	f.refresh()
	if st, _ := f.svc.Status(f.ctx); !st.InstanceChanged {
		t.Fatal("the change was not detected")
	}
	if _, err := f.svc.ConfirmInstance(f.ctx, InstanceConfirm); err != nil {
		t.Fatal(err)
	}
	if got := f.count(`SELECT count(*) FROM wish_account_map WHERE account_id = 1 AND seerr_user_id = 4`); got != 1 {
		t.Error("confirm dropped the mapping")
	}
	if got := f.count(`SELECT count(*) FROM settings WHERE key = 'seerr.instance' AND value LIKE '%owner@new.example%'`); got != 1 {
		t.Error("confirm did not take over the fingerprint")
	}
	f.fake.ResetCalls()
	f.refresh()
	if st, _ := f.svc.Status(f.ctx); st.InstanceChanged {
		t.Error("the confirmed instance counts as changed again")
	}
}

// The hooks of package admin and the manual refresh.
func TestUsersQueuedOnSaveAndByHand(t *testing.T) {
	f := newFixture(t)
	f.svc.SeerrSaved(f.ctx)
	if err := f.svc.RefreshUsers(f.ctx); err != nil {
		t.Fatal(err)
	}
	if n := f.queue.count(JobUsers); n != 2 {
		t.Errorf("queued %d, want 2", n)
	}
	f.cfg.set(func(c *fakeConfig) { c.seer = model.SeerrConfig{} })
	if err := f.svc.RefreshUsers(f.ctx); !errors.Is(err, model.ErrSeerrNotConfigured) {
		t.Errorf("without connection: %v", err)
	}
}
