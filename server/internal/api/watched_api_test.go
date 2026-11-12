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
	put := func(token, id, body string) *http.Response {
		t.Helper()
		return send(t, h, http.MethodPut, BasePath+"/items/"+id+"/watched", token, body, nil).Result()
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
	if res := put("p12", film.ID, `{"watched":true}`); res.StatusCode != http.StatusNoContent {
		t.Fatalf("mark film: %d", res.StatusCode)
	}
	if p := detail("p12", film.ID).Progress; p == nil || !p.Completed || p.PositionMs != p.DurationMs {
		t.Errorf("film after the check mark: %+v, want finished at the end", p)
	}
	show := f.Show
	if res := put("p12", show.Series.ID, `{"watched":true}`); res.StatusCode != http.StatusNoContent {
		t.Fatalf("mark series: %d", res.StatusCode)
	}
	if d := detail("p12", show.Series.ID); d.Watched == nil || !*d.Watched {
		t.Errorf("series for P12 after the check mark: watched %v, want true", d.Watched)
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
	if res := put("p12", show.Series.ID, `{"watched":false}`); res.StatusCode != http.StatusNoContent {
		t.Fatalf("unmark series: %d", res.StatusCode)
	}
	if p := detail("p12", show.E1.ID).Progress; p != nil && p.Completed {
		t.Errorf("E1 for P12 after removing: %+v, want open", p)
	}
	if res := put("p12", film.ID, ""); res.StatusCode != http.StatusBadRequest {
		t.Errorf("no body: %d, want 400", res.StatusCode)
	}
	if res := put("p6", film.ID, `{"watched":true}`); res.StatusCode != http.StatusNotFound {
		t.Errorf("invisible film: %d, want 404", res.StatusCode)
	}
	if res := put("none", film.ID, `{"watched":true}`); res.StatusCode != http.StatusForbidden {
		t.Errorf("session without profile: %d, want 403", res.StatusCode)
	}
}
