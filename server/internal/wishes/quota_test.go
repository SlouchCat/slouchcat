package wishes

import (
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// Build step 1.5c: the limit per account in Seerr (architecture 19; N-31, N-34, N-35, N-112).

func limitOf(t *testing.T, fake *seerrtest.Server, id int) [4]int {
	t.Helper()
	u, ok := fake.User(id)
	if !ok || u.MovieQuotaLimit == nil || u.MovieQuotaDays == nil || u.TVQuotaLimit == nil || u.TVQuotaDays == nil {
		t.Fatalf("user %d has empty limit fields: %+v", id, u)
	}
	return [4]int{*u.MovieQuotaLimit, *u.MovieQuotaDays, *u.TVQuotaLimit, *u.TVQuotaDays}
}

func ip(n int) *int { return &n }

// «Standard» writes the default numbers, «Eigenes» its own, «Unbegrenzt» 0; invalid values are
// refused.
func TestLimitModesWritten(t *testing.T) {
	f := newFixture(t)
	a, err := f.svc.ProvisionAccount(f.ctx, 2, 1, false) // opened with the default
	if err != nil {
		t.Fatal(err)
	}
	id := *a.SeerrUserID
	if got := limitOf(t, f.fake, id); got != [4]int{10, 7, 5, 7} {
		t.Errorf("default = %v", got)
	}
	set := func(mode string, movie, tv *model.WishLimit) model.AccountWishLimit {
		t.Helper()
		l, err := f.svc.SetAccountLimit(f.ctx, 2, mode, movie, tv, 1)
		if err != nil {
			t.Fatal(err)
		}
		if n := f.runQueued(JobProvision); n != 1 {
			t.Fatalf("%s: %d jobs", mode, n)
		}
		return l
	}
	l := set(model.WishLimitCustom, &model.WishLimit{Limit: 3, Days: 14}, &model.WishLimit{Limit: 0, Days: 30})
	if l.Mode != model.WishLimitCustom || l.Effective.Movie.Limit != 3 || l.Effective.TV.Days != 30 {
		t.Errorf("custom = %+v", l)
	}
	if got := limitOf(t, f.fake, id); got != [4]int{3, 14, 0, 30} {
		t.Errorf("custom in seerr = %v", got)
	}
	if set(model.WishLimitUnlimited, nil, nil).Mode != model.WishLimitUnlimited {
		t.Error("unlimited mode")
	}
	if got := limitOf(t, f.fake, id); got[0] != 0 || got[2] != 0 {
		t.Errorf("unlimited in seerr = %v", got)
	}
	set(model.WishLimitDefault, nil, nil)
	if got := limitOf(t, f.fake, id); got != [4]int{10, 7, 5, 7} {
		t.Errorf("back to default = %v", got)
	}
	for _, c := range []struct {
		mode      string
		movie, tv *model.WishLimit
	}{
		{model.WishLimitCustom, &model.WishLimit{Limit: 101, Days: 7}, &model.WishLimit{Limit: 1, Days: 7}},
		{model.WishLimitCustom, &model.WishLimit{Limit: 1, Days: 0}, &model.WishLimit{Limit: 1, Days: 7}},
		{model.WishLimitCustom, &model.WishLimit{Limit: 1, Days: 7}, nil},
		{model.WishLimitDefault, &model.WishLimit{Limit: 1, Days: 7}, nil},
		{"weekly", nil, nil},
	} {
		var verr *model.ValidationError
		if _, err := f.svc.SetAccountLimit(f.ctx, 2, c.mode, c.movie, c.tv, 1); !errors.As(err, &verr) {
			t.Errorf("%+v: %v", c, err)
		}
	}
	if _, err := f.svc.SetAccountLimit(f.ctx, 99, model.WishLimitUnlimited, nil, nil, 1); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown account: %v", err)
	}
	// A new default reaches the accounts that follow it.
	f.cfg.set(func(c *fakeConfig) { c.wish.DefaultLimit.Movie.Limit = 8 })
	f.svc.WishSettingsSaved(f.ctx, model.WishSettings{DefaultLimit: model.WishLimits{Movie: model.WishLimit{Limit: 10, Days: 7}, TV: model.WishLimit{Limit: 5, Days: 7}}},
		model.WishSettings{DefaultLimit: model.WishLimits{Movie: model.WishLimit{Limit: 8, Days: 7}, TV: model.WishLimit{Limit: 5, Days: 7}}})
	f.runQueued(JobProvision)
	if got := limitOf(t, f.fake, id); got[0] != 8 {
		t.Errorf("new default = %v", got)
	}
}

// A user with MANAGE_USERS gets nothing written («Kein Limit»).
func TestLimitNotForManageUsers(t *testing.T) {
	f := newFixture(t)
	f.fake.UpdateUser(4, func(u *seerrtest.User) { u.Permissions = seerrtest.PermManageUsers | seerrtest.PermRequest })
	a, err := f.svc.ProvisionAccount(f.ctx, 3, 1, true) // a manager is privileged: linked only after the confirmation
	if err != nil {
		t.Fatal(err)
	}
	if n := f.fake.Calls("POST /user/{id}/settings/main"); n != 0 {
		t.Errorf("limit written to a manager %d times", n)
	}
	if !a.Limit.SeerrAdmin || a.Quota == nil || !a.Quota.Unlimited || a.Quota.Reason != model.WishQuotaReasonAdmin {
		t.Errorf("manager = %+v, quota %+v", a.Limit, a.Quota)
	}
	if got := f.count(`SELECT count(*) FROM wish_account_limits`); got != 0 {
		t.Error("a manager's limit was taken over")
	}
}

// N-112: linking takes over the user's limit when the account has none.
func TestLimitTakenOverWhenLinked(t *testing.T) {
	for _, c := range []struct {
		name           string
		ml, md, tl, td *int
		own            bool
		wantMode       string
		wantSeerr      [4]int
	}{
		{"all empty: default", nil, nil, nil, nil, false, model.WishLimitDefault, [4]int{10, 7, 5, 7}},
		{"both 0: unlimited", ip(0), ip(30), ip(0), nil, false, model.WishLimitUnlimited, [4]int{0, 7, 0, 7}},
		{"own values, empty from the default", ip(3), ip(14), nil, nil, false, model.WishLimitCustom, [4]int{3, 14, 5, 7}},
		{"out of bounds: clamped and written back", ip(250), ip(0), ip(2), ip(365), false, model.WishLimitCustom, [4]int{100, 1, 2, 90}},
		{"own limit in slouchcat wins", ip(3), ip(14), ip(1), ip(7), true, model.WishLimitCustom, [4]int{4, 21, 4, 21}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.fake.UpdateUser(4, func(u *seerrtest.User) {
				u.MovieQuotaLimit, u.MovieQuotaDays, u.TVQuotaLimit, u.TVQuotaDays = c.ml, c.md, c.tl, c.td
			})
			if c.own {
				f.exec(`INSERT INTO wish_account_limits (account_id, mode, movie_limit, movie_days, tv_limit, tv_days, set_at)
				        VALUES (3, 'custom', 4, 21, 4, 21, 1)`)
			}
			a, err := f.svc.ProvisionAccount(f.ctx, 3, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if a.State != OriginLinked || a.Limit.Mode != c.wantMode {
				t.Errorf("mapping = %+v", a)
			}
			if got := limitOf(t, f.fake, 4); got != c.wantSeerr {
				t.Errorf("in seerr = %v, want %v", got, c.wantSeerr)
			}
		})
	}
	// The same for a user chosen by hand: the job writes it.
	f := newFixture(t)
	f.fake.UpdateUser(3, func(u *seerrtest.User) { u.MovieQuotaLimit, u.MovieQuotaDays = ip(2), ip(10) })
	f.refresh()
	three := 3
	if _, err := f.svc.SetAccountMapping(f.ctx, 2, &three, 1); err != nil {
		t.Fatal(err)
	}
	f.runQueued(JobProvision)
	if a := f.mapping(2); a.Limit.Mode != model.WishLimitCustom || a.Limit.Effective.Movie != (model.WishLimit{Limit: 2, Days: 10}) {
		t.Errorf("chosen = %+v", a.Limit)
	}
	if got := limitOf(t, f.fake, 3); got != [4]int{2, 10, 5, 7} {
		t.Errorf("chosen in seerr = %v", got)
	}
}

// The state from GET /user/{id}/quota and «nächster Platz frei».
func TestQuotaStateAndNextFree(t *testing.T) {
	f := newFixture(t)
	f.rebuild(WithClock(f.fake.Now))
	f.exec(`INSERT INTO wish_account_limits (account_id, mode, movie_limit, movie_days, tv_limit, tv_days, set_at)
	        VALUES (2, 'custom', 2, 7, 0, 7, 1)`)
	a, err := f.svc.ProvisionAccount(f.ctx, 2, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	id := *a.SeerrUserID
	if a.Quota == nil || a.Quota.Movie == nil || a.Quota.Movie.Used != 0 || a.Quota.Movie.Remaining != 2 || a.Quota.TV != nil {
		t.Fatalf("fresh quota = %+v", a.Quota)
	}
	f.fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 603, Name: "The Matrix"})
	f.fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: 604, Name: "Reloaded"})
	c, err := seerr.New(f.fake.URL, seerrtest.APIKey)
	if err != nil {
		t.Fatal(err)
	}
	first := f.fake.Now()
	if _, err := c.AsUser(id).CreateRequest(f.ctx, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: 603}); err != nil {
		t.Fatal(err)
	}
	f.fake.Advance(2 * 24 * time.Hour)
	if _, err := c.AsUser(id).CreateRequest(f.ctx, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: 604}); err != nil {
		t.Fatal(err)
	}
	f.refresh() // the users job reads the state of every mapped user
	q := f.mapping(2).Quota
	if q == nil || q.Movie == nil || q.Movie.Used != 2 || q.Movie.Remaining != 0 || q.Movie.NextFreeAt == nil ||
		!q.Movie.NextFreeAt.Equal(first.Add(7*24*time.Hour)) || q.At == nil {
		t.Errorf("quota = %+v %+v", q, q.Movie)
	}
	// Unlimited: no halves.
	if _, err := f.svc.SetAccountLimit(f.ctx, 2, model.WishLimitUnlimited, nil, nil, 1); err != nil {
		t.Fatal(err)
	}
	f.runQueued(JobProvision)
	if q := f.mapping(2).Quota; q == nil || !q.Unlimited || q.Reason != model.WishQuotaReasonNoLimit || q.Movie != nil {
		t.Errorf("unlimited quota = %+v", q)
	}
	// The shared user shows the default.
	if err := f.svc.EnsureSharedUser(f.ctx); err != nil {
		t.Fatal(err)
	}
	if all, _ := f.svc.WishMappings(f.ctx); all.SharedQuota == nil || all.SharedQuota.Movie == nil || all.SharedQuota.Movie.Limit != 10 {
		t.Errorf("shared quota = %+v", all.SharedQuota)
	}
}
