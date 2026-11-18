package wishes

import (
	"fmt"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify"
	"github.com/slouchcat/slouchcat/server/internal/store/storetest"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// The real notify service behind the adapter: a wish that becomes available through the reconcile
// gets one inbox entry per requester and notified_at, both written by notify; a second pass and a
// second call send nothing.
func TestNotifierAdapterWithNotify(t *testing.T) {
	r := syncFixture(t)
	n := notify.New(r.db, visibility.New(r.db, r.log), r.log)
	r.fixture.rebuild(WithLibrary(r.lib), WithTitles(r.titles), WithNotifier(NotifyAdapter(n)), WithClock(r.clock.now))
	res := r.create(r.marco, film(matrix))
	r.exec(`UPDATE wishes SET state = 'processing' WHERE id = ?`, res.Wish.ID)
	r.addTitle("movie", matrix, itemMatrix, nil)
	if err := storetest.MarkTitleAvailable(r.ctx, r.db, itemMatrix); err != nil { // playable for the real visibility
		t.Fatal(err)
	}
	entries := func() int {
		return r.count(`SELECT count(*) FROM notifications WHERE wish_id = ? AND kind = ? AND profile_id = ?`, res.Wish.ID, notify.KindWishAvailable, r.marco.ID)
	}
	for pass := range 2 {
		if err := r.svc.reconcile(r.ctx, reconcilePayload{}); err != nil {
			t.Fatal(err)
		}
		if w := r.wish(res.Wish.ID); w.State != model.WishAvailable {
			t.Fatalf("pass %d: state %s", pass, w.State)
		}
		if got := entries(); got != 1 {
			t.Errorf("pass %d: %d inbox entries, want 1", pass, got)
		}
	}
	if got := r.count(`SELECT count(*) FROM wish_requesters WHERE wish_id = ? AND profile_id = ? AND notified_at IS NOT NULL`, res.Wish.ID, r.marco.ID); got != 1 {
		t.Error("notified_at not set by notify")
	}
	out, err := NotifyAdapter(n).WishAvailable(r.ctx, res.Wish.ID, itemMatrix)
	if err != nil || len(out.Notified) != 0 || entries() != 1 {
		t.Errorf("second call: %+v, %v, %d entries", out, err, entries())
	}
	// A season without a known target is not handed on.
	if out, err := NotifyAdapter(n).WishSeasonAvailable(r.ctx, SeasonNotice{WishID: res.Wish.ID, SeriesID: itemMatrix, Season: 1}); err != nil || len(out.Notified)+len(out.Waiting) != 0 {
		t.Errorf("season without target: %+v, %v", out, err)
	}
}

// The episode target of a season comes from series_catalog when it has the season, else from the
// title outside the library; a season the catalog lists with no aired episode is not reported.
func TestNotifierSeasonTargetFromCatalog(t *testing.T) {
	r := syncFixture(t)
	res := r.create(r.marco, show(got, 1, 2))
	r.exec(`UPDATE wishes SET state = 'processing' WHERE id = ?`, res.Wish.ID)
	r.exec(`UPDATE wish_seasons SET state = 'processing' WHERE wish_id = ?`, res.Wish.ID)
	// The catalog: season 1 is on air with 4 aired episodes, season 2 has none aired yet.
	r.lib.catalog[itemGot] = map[int]visibility.SeasonTarget{1: {Episodes: 4, Airing: true}, 2: {Episodes: 0, Airing: true}}
	r.addTitle("show", got, itemGot, map[int]int{1: 4, 2: 0})
	if err := r.svc.reconcile(r.ctx, reconcilePayload{}); err != nil {
		t.Fatal(err)
	}
	w := r.wish(res.Wish.ID)
	if w.State != model.WishPartial || w.Seasons[0].State != model.WishAvailable || w.Seasons[1].State != model.WishProcessing {
		t.Errorf("catalog target: %+v", w.Wish)
	}
	if !r.note.has(fmt.Sprintf("season %d 1/4", res.Wish.ID)) {
		t.Errorf("notifier = %v", r.note.got())
	}
	for _, c := range r.note.got() {
		if c == fmt.Sprintf("season %d 2/0", res.Wish.ID) {
			t.Error("a season without an aired episode was reported")
		}
	}
}
