package api

import (
	"net/http"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// setItemWatched (concept 20 section 6.4, contract 1.31.0): P12 marks a film and the detail shows
// it finished; the check mark of the series reaches the episodes P12 may see and not E2 (rated
// 16), which P18 still finds open; false removes the check mark again. A missing body is 400
// (the generated decoder), an invisible title 404 and a session without profile 403.
func TestHTTPSetItemWatched(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	put := func(token, id, body string) int {
		t.Helper()
		return send(t, h, http.MethodPut, BasePath+"/items/"+id+"/watched", token, body, nil).Code
	}
	detail := func(token, id string) gen.ItemDetail {
		t.Helper()
		var d gen.ItemDetail
		rec := send(t, h, http.MethodGet, BasePath+"/items/"+id, token, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("detail %s for %s: %d", id, token, rec.Code)
		}
		decode(t, rec, &d)
		return d
	}
	film := f.Rated(12)
	if code := put("p12", film.ID, `{"watched":true}`); code != http.StatusNoContent {
		t.Fatalf("mark film: %d", code)
	}
	if p := detail("p12", film.ID).Progress; p == nil || !p.Completed || p.PositionMs != p.DurationMs || p.WatchedAt == nil || !p.WatchedAt.Equal(p.UpdatedAt) {
		t.Errorf("film after the check mark: %+v, want finished at the end with watchedAt", p)
	}
	show := f.Show
	if code := put("p12", show.Series.ID, `{"watched":true}`); code != http.StatusNoContent {
		t.Fatalf("mark series: %d", code)
	}
	// P12 sees E1, E3 and the special: all three watched, the series full (contract 1.31.0).
	if d := detail("p12", show.Series.ID); d.Watched == nil || !*d.Watched || d.WatchedCount == nil || *d.WatchedCount != 3 {
		t.Errorf("series for P12 after the check mark: watched %v, watchedCount %v, want true and 3", d.Watched, d.WatchedCount)
	}
	if p := detail("p12", show.E1.ID).Progress; p == nil || !p.Completed {
		t.Errorf("E1 for P12: %+v, want finished", p)
	}
	if p := detail("p18", show.E2.ID).Progress; p != nil && p.Completed {
		t.Errorf("E2 hidden from P12 was marked: %+v", p)
	}
	if d := detail("p18", show.Series.ID); d.Watched == nil || *d.Watched {
		t.Errorf("series for P18 (nothing marked): watched %v, want false", d.Watched)
	}
	if code := put("p12", show.Series.ID, `{"watched":false}`); code != http.StatusNoContent {
		t.Fatalf("unmark series: %d", code)
	}
	if p := detail("p12", show.E1.ID).Progress; p != nil && (p.Completed || p.WatchedAt != nil) {
		t.Errorf("E1 for P12 after removing: %+v, want open without watchedAt", p)
	}
	// One episode marked: the half check mark, watched false with watchedCount 1 (F-12).
	if code := put("p12", show.E1.ID, `{"watched":true}`); code != http.StatusNoContent {
		t.Fatalf("mark E1: %d", code)
	}
	if d := detail("p12", show.Series.ID); d.Watched == nil || *d.Watched || d.WatchedCount == nil || *d.WatchedCount != 1 {
		t.Errorf("series for P12 with E1 marked: watched %v, watchedCount %v, want false and 1", d.Watched, d.WatchedCount)
	}
	// listWatched: the film and the series (one entry, its visible episodes counted).
	var page gen.WatchedPage
	rec := send(t, h, http.MethodGet, BasePath+"/watched?limit=500", "p12", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("listWatched: %d %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &page)
	if len(page.Items) != 2 || page.Total != 2 || page.Limit != 100 || page.Items[0].Item.Id != show.Series.ID || page.Items[1].Item.Id != film.ID {
		t.Fatalf("listWatched for P12 = %s, want the series, then the film, limit 100", rec.Body.String())
	}
	series := page.Items[0]
	if series.EpisodeCount == nil || *series.EpisodeCount != 3 || series.Item.WatchedCount == nil || *series.Item.WatchedCount != 1 ||
		page.Items[1].EpisodeCount != nil || page.Items[1].LastWatchedAt.IsZero() {
		t.Errorf("listWatched entries = %s, want the series with 1 of 3 episodes", rec.Body.String())
	}
	decode(t, send(t, h, http.MethodGet, BasePath+"/watched?content=movie&sort=title", "p12", "", nil), &page)
	if len(page.Items) != 1 || page.Items[0].Item.Id != film.ID || page.Total != 1 {
		t.Errorf("listWatched films by title = %+v", page)
	}
	for _, q := range []string{"content=radio", "sort=added"} {
		if rec := send(t, h, http.MethodGet, BasePath+"/watched?"+q, "p12", "", nil); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("listWatched?%s: %d, want 422", q, rec.Code)
		}
	}
	if code := put("p12", film.ID, ""); code != http.StatusBadRequest {
		t.Errorf("no body: %d, want 400", code)
	}
	if code := put("p6", film.ID, `{"watched":true}`); code != http.StatusNotFound {
		t.Errorf("invisible film: %d, want 404", code)
	}
	if code := put("none", film.ID, `{"watched":true}`); code != http.StatusForbidden {
		t.Errorf("session without profile: %d, want 403", code)
	}
}
