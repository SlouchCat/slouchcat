package api

import (
	"fmt"
	"net/http"
	"slices"
	"testing"
)

// Contract 1.44.0 (question 36, ADR 0045): the request right follows the rating. A profile below 18
// has no right and cannot get it (422 canRequest age_restricted); a rating that crosses 18 sets the
// right and wins over a canRequest sent with it, as the phone app 1.0.3 always sends one.
func TestContractRequestRightFollowsRating(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	path := BasePath + fmt.Sprintf("/profiles/%d", f.P6.ID)
	update := func(body string) (int, bool) {
		t.Helper()
		rec := send(t, h, http.MethodPatch, path, "p18", body, nil)
		var p struct {
			CanRequest bool `json:"canRequest"`
		}
		if rec.Code == http.StatusOK {
			decode(t, rec, &p)
		}
		return rec.Code, p.CanRequest
	}
	rec := send(t, h, http.MethodPatch, path, "p18", `{"canRequest":true}`, nil)
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "validation_failed" ||
		!slices.Contains(fieldErrors(t, rec), "canRequest age_restricted") {
		t.Fatalf("right for a profile aged 6 = %d %s", rec.Code, rec.Body.String())
	}
	if code, on := update(`{"maxRating":18,"canRequest":false}`); code != http.StatusOK || !on {
		t.Errorf("lifted to 18 with the old «off» = %d, canRequest %v; want 200 and on", code, on)
	}
	if code, on := update(`{"maxRating":12,"canRequest":true}`); code != http.StatusOK || on {
		t.Errorf("lowered to 12 with the old «on» = %d, canRequest %v; want 200 and off", code, on)
	}
	if code, on := update(`{"name":"P6","canRequest":false}`); code != http.StatusOK || on {
		t.Errorf("switching off at 12 = %d, canRequest %v", code, on)
	}
	rec = send(t, h, http.MethodPost, BasePath+"/profiles", "p18", `{"name":"Neu","maxRating":12,"canRequest":true}`, nil)
	if rec.Code != http.StatusUnprocessableEntity || !slices.Contains(fieldErrors(t, rec), "canRequest age_restricted") {
		t.Errorf("new profile aged 12 with the right = %d %s", rec.Code, rec.Body.String())
	}
}
