package wishes

import (
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
)

// Build step 1.6: the request right reaches Seerr, an invitation hands its limit to the account,
// the overview shows the integration and the hints.

func TestCanRequestFollowsToSeerr(t *testing.T) {
	f := newFixture(t)
	a, err := f.svc.ProvisionAccount(f.ctx, 2, 1, false) // opened, with the right
	if err != nil {
		t.Fatal(err)
	}
	authSvc := auth.New(f.db, f.log).WithRequestRightChanged(f.svc.RequestRightChanged)
	if err := authSvc.UpdateAccountAccess(f.ctx, 2, model.AccountAccessUpdate{CanRequest: new(false)}); err != nil {
		t.Fatal(err)
	}
	if n := f.runQueued(JobProvision); n != 1 {
		t.Fatalf("jobs = %d", n)
	}
	if u, _ := f.fake.User(*a.SeerrUserID); u.Permissions != 0 {
		t.Errorf("rights after the right was taken = %d", u.Permissions)
	}
	if r, _ := f.svc.ForProfile(f.ctx, 2, 3); r.CanRequest || r.Reason != model.WishReasonAccountNotAllowed {
		t.Errorf("session right = %+v", r)
	}
	// A linked user: no job.
	if _, err := f.svc.ProvisionAccount(f.ctx, 3, 1, false); err != nil {
		t.Fatal(err)
	}
	if err := authSvc.UpdateAccountAccess(f.ctx, 3, model.AccountAccessUpdate{CanRequest: new(true)}); err != nil {
		t.Fatal(err)
	}
	if n := f.queue.count(JobProvision); n != 0 {
		t.Errorf("linked user queued %d jobs", n)
	}
}

func TestInvitationLimitAndDashboard(t *testing.T) {
	f := newFixture(t)
	now := time.Now()
	f.rebuild(WithClock(func() time.Time { return now }))
	a := auth.New(f.db, f.log).WithAccountCreated(f.svc.AccountCreated).WithPasswordSeen(f.svc.PasswordSeen)
	inv, err := a.CreateInvitation(f.ctx, model.Account{ID: 1}, model.InvitationRequest{MaxRating: 12, AllLibraries: true, CanRequest: new(true),
		WishLimit: &model.InvitationWishLimit{Mode: model.WishLimitCustom, Movie: &model.WishLimit{Limit: 3, Days: 14}, TV: &model.WishLimit{Limit: 1, Days: 7}}})
	if err != nil {
		t.Fatal(err)
	}
	f.fake.UpdateUser(3, func(u *seerrtest.User) { u.Email = "neu@example.test" })
	sess, _, err := a.AcceptInvitation(client(), model.AcceptInvitation{Token: inv.Token, Email: "neu@example.test", Password: testPassword, ProfileName: "Neu"}, "ua")
	if err != nil {
		t.Fatal(err)
	}
	f.svc.waitBackground()
	m := f.mapping(sess.Account.ID)
	if m.State != OriginLinked || m.Limit.Mode != model.WishLimitCustom || m.Limit.Effective.Movie.Limit != 3 || !m.CanRequest {
		t.Fatalf("invited account = %+v", m)
	}
	if got := limitOf(t, f.fake, 3); got != [4]int{3, 14, 1, 7} {
		t.Errorf("invitation limit in seerr = %v", got)
	}
	// The overview: «Neu verknüpft», the integration, accounts without a user, the limit reached.
	f.exec(`UPDATE seerr_users SET movie_quota_used = 3, quota_at = 1 WHERE seerr_id = 3`)
	d, err := f.svc.DashboardWishes(f.ctx)
	if err != nil || d == nil {
		t.Fatalf("dashboard = %+v, %v", d, err)
	}
	if len(d.Linked) != 1 || d.Linked[0].AccountID != sess.Account.ID || d.Unmapped != 2 || !d.UnmappedFallback || d.SeerrState != "" {
		t.Errorf("dashboard = %+v", d)
	}
	if len(d.QuotaReached) != 1 || d.QuotaReached[0].Movie == nil || d.QuotaReached[0].TV != nil {
		t.Errorf("limit reached = %+v", d.QuotaReached)
	}
	st, err := f.svc.IntegrationStatus(f.ctx)
	if err != nil || st.Key != "seerr" || st.State != model.IntegrationConfigured {
		t.Errorf("integration = %+v, %v", st, err)
	}
	f.fake.SetFailure("*", seerrtest.Failure{Status: 503})
	if err := f.svc.handleUsers(f.ctx, &jobs.Job{Type: JobUsers}); err == nil {
		t.Error("the users job did not fail while Seerr is unreachable")
	}
	if st, _ := f.svc.IntegrationStatus(f.ctx); st.State != model.IntegrationError || st.Detail != model.SeerrStateUnreachable {
		t.Errorf("unreachable = %+v", st)
	}
	if d, _ := f.svc.DashboardWishes(f.ctx); d == nil || d.SeerrState != model.SeerrStateUnreachable {
		t.Errorf("dashboard unreachable = %+v", d)
	}
	f.cfg.set(func(c *fakeConfig) { c.seer.Enabled = false })
	if st, _ := f.svc.IntegrationStatus(f.ctx); st.State != model.IntegrationDisabled {
		t.Errorf("disabled = %+v", st)
	}
	f.cfg.set(func(c *fakeConfig) { c.seer.URL = "" })
	if st, _ := f.svc.IntegrationStatus(f.ctx); st.State != model.IntegrationMissing {
		t.Errorf("missing = %+v", st)
	}
	if d, _ := f.svc.DashboardWishes(f.ctx); d != nil {
		t.Errorf("dashboard without seerr = %+v", d)
	}
	f.noPasswordAnywhere(testPassword)
}
