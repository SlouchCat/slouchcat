package wishes

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/seerr/seerrtest"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Stale reads and the requester's own view (review Minor 1, 2, 3; security review M-1,
// architecture 8.3).

// scopedLib is the fake library in which some profiles may not see some items.
type scopedLib struct {
	*fakeLib
	hidden map[visibility.Scope]map[string]bool
}

// hideFrom hides itemID from profile p (both scopes the service builds for it).
func (l *scopedLib) hideFrom(p model.Profile, itemID string) {
	for _, sc := range []visibility.Scope{visibility.Profile(p), visibility.Profile(model.Profile{ID: p.ID, AccountID: p.AccountID})} {
		if l.hidden[sc] == nil {
			l.hidden[sc] = map[string]bool{}
		}
		l.hidden[sc][itemID] = true
	}
}

func (l *scopedLib) Visible(ctx context.Context, sc visibility.Scope, itemID string) error {
	if l.hidden[sc][itemID] {
		return model.ErrNotFound
	}
	return l.fakeLib.Visible(ctx, sc, itemID)
}

func (l *scopedLib) MatchExternalTitles(ctx context.Context, sc visibility.Scope, right visibility.RequestRight, refs []visibility.ExternalRef) ([]visibility.ExternalMatch, error) {
	out, err := l.fakeLib.MatchExternalTitles(ctx, sc, right, refs)
	for i := range out {
		if l.hidden[sc][out[i].ItemID] {
			out[i].ItemID = ""
		}
	}
	return out, err
}

// Security review M-1: a co-wisher that may not see the library title reads its available wish as
// «In Beschaffung · wird eingelesen», without time and item, in every list and count; the title
// stays 404 for it. The profile that sees it reads available.
func TestAvailableReadsProcessingForProfileThatCannotSee(t *testing.T) {
	r := newRequestFixture(t)
	lib := &scopedLib{fakeLib: r.lib, hidden: map[visibility.Scope]map[string]bool{}}
	r.fixture.rebuild(WithLibrary(lib), WithTitles(r.titles), WithNotifier(r.note), WithClock(r.clock.now))
	res := r.create(r.marco, film(matrix))
	r.create(r.gast, film(matrix))
	r.addTitle("movie", matrix, itemMatrix, nil)
	lib.hideFrom(r.gast, itemMatrix)
	if err := r.svc.reconcile(r.ctx, reconcilePayload{}); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishAvailable {
		t.Fatalf("wish = %+v", w)
	}
	mine, err := r.svc.ProfileWishes(r.ctx, r.marco, model.WishFilterAll, 0, 10)
	if err != nil || len(mine.Items) != 1 || mine.Items[0].State != model.WishAvailable || mine.Counts.Available != 1 {
		t.Fatalf("marco = %+v, %v", mine, err)
	}
	for name, get := range map[string]func() (model.WishPage, error){
		"profile": func() (model.WishPage, error) { return r.svc.ProfileWishes(r.ctx, r.gast, model.WishFilterAll, 0, 10) },
		"account": func() (model.WishPage, error) { return r.svc.AccountWishes(r.ctx, r.gast, model.WishFilterAll, 0, 10) },
	} {
		page, err := get()
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("%s = %+v, %v", name, page, err)
		}
		w := page.Items[0]
		if w.State != model.WishProcessing || w.StateDetail != model.WishDetailImporting || w.AvailableAt != nil || w.ItemID != "" {
			t.Errorf("%s: wish = %+v", name, w)
		}
		if page.Counts.Available != 0 || page.Counts.Processing != 1 {
			t.Errorf("%s: counts = %+v", name, page.Counts)
		}
		if slices.ContainsFunc(w.Events, func(e model.WishEvent) bool { return strings.Contains(e.Detail, `"to":"available"`) }) {
			t.Errorf("%s: an event tells the title is available: %+v", name, w.Events)
		}
	}
	if avail, _ := r.svc.ProfileWishes(r.ctx, r.gast, model.WishFilterAvailable, 0, 10); avail.Total != 0 {
		t.Errorf("filter available = %+v", avail)
	}
	if _, err := r.svc.ExternalTitle(r.ctx, r.gast, model.WishMediaMovie, matrix); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("title for the profile that may not see it: %v, want 404", err)
	}
}

// Review Minor 2: the single check works on the wish as it is in the transaction; a wish withdrawn
// during the calls (the withdrawal deleted the request, GET answers 404) stays withdrawn.
func TestCheckDoesNotFailWithdrawnWish(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	stale, err := r.svc.read.GetWish(r.ctx, res.Wish.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.svc.WithdrawWish(r.ctx, r.marco, res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	r.deleteInSeerr(3, int(stale.SeerrRequestID.Int64))
	admin, err := r.svc.AdminSession(r.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.svc.checkWish(r.ctx, admin, stale, r.clock.now()); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishWithdrawn {
		t.Errorf("wish = %+v", w)
	}
	if slices.ContainsFunc(r.note.got(), func(s string) bool { return strings.HasPrefix(s, "failed") }) {
		t.Errorf("announced = %v", r.note.got())
	}
}

// Review Minor 3: the reconcile writes over the wish as it is now; a wish withdrawn while the
// library was asked stays withdrawn and nothing is announced.
func TestReconcileDoesNotReviveWithdrawnWish(t *testing.T) {
	r := newRequestFixture(t)
	res := r.create(r.marco, film(matrix))
	stale, err := r.svc.read.GetWish(r.ctx, res.Wish.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.svc.WithdrawWish(r.ctx, r.marco, res.Wish.ID); err != nil {
		t.Fatal(err)
	}
	r.addTitle("movie", matrix, itemMatrix, nil)
	if err := r.svc.markAvailable(r.ctx, stale, itemMatrix, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.markImporting(r.ctx, stale); err != nil {
		t.Fatal(err)
	}
	if w := r.wish(res.Wish.ID); w.State != model.WishWithdrawn {
		t.Errorf("wish = %+v", w)
	}
	if slices.ContainsFunc(r.note.got(), func(s string) bool { return strings.HasPrefix(s, "available") }) {
		t.Errorf("announced = %v", r.note.got())
	}
}

// Review Minor 1: «nächster Platz frei» comes from the requests inside the window, also for a user
// with more than one page of older requests.
func TestNextFreeFromRequestsInsideTheWindow(t *testing.T) {
	f := newFixture(t)
	f.rebuild(WithClock(f.fake.Now))
	a, err := f.svc.ProvisionAccount(f.ctx, 2, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	id := *a.SeerrUserID
	if _, err := f.svc.SetAccountLimit(f.ctx, 2, model.WishLimitUnlimited, nil, nil, 1); err != nil {
		t.Fatal(err)
	}
	f.runQueued(JobProvision)
	c, err := seerr.New(f.fake.URL, seerrtest.APIKey)
	if err != nil {
		t.Fatal(err)
	}
	request := func(tmdb int) {
		f.fake.AddTitle(seerrtest.Title{MediaType: "movie", TMDBID: tmdb, Name: "Film"})
		if _, err := c.AsUser(id).CreateRequest(f.ctx, seerr.NewRequest{MediaType: seerr.MediaTypeMovie, TMDBID: tmdb}); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 55 { // more than one page, all before the window
		request(1000 + i)
	}
	f.fake.Advance(8 * 24 * time.Hour)
	first := f.fake.Now()
	for i := range 3 {
		request(2000 + i)
		f.fake.Advance(time.Hour)
	}
	if _, err := f.svc.SetAccountLimit(f.ctx, 2, model.WishLimitCustom, &model.WishLimit{Limit: 3, Days: 7}, &model.WishLimit{Limit: 0, Days: 7}, 1); err != nil {
		t.Fatal(err)
	}
	f.runQueued(JobProvision)
	f.refresh()
	q := f.mapping(2).Quota
	if q == nil || q.Movie == nil || q.Movie.Remaining != 0 || q.Movie.NextFreeAt == nil || !q.Movie.NextFreeAt.Equal(first.Add(7*24*time.Hour)) {
		t.Errorf("quota = %+v %+v", q, q.Movie)
	}
}
