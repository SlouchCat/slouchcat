package api

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// The radio offer through the API (contract 1.28.0, concept 19 part B, PB-1, PB-11): after the
// administration keeps only Switzerland, the German station answers 404 on station, listen, image
// and favourite for every profile, P6's German favourite stays stored but is not listed, the Swiss
// stations stay. The preview list shows the draft's marks, and a second apply on the old version
// is 409 offer_conflict.
func TestRadioOfferKeepsOnlyTheChosenCountry(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	de := fmt.Sprint(vistest.RadioDE)
	rec := send(t, h, http.MethodPost, BasePath+"/admin/radio/offer/preview", "p18",
		`{"steps":[{"op":"removeCountry","country":"DE"},{"op":"mark","stationIds":[`+radioHidden+`]}],"list":{"country":"CH"}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	var preview struct {
		Version int64 `json:"version"`
		Diff    struct {
			Added, Removed int
		} `json:"diff"`
		Countries []struct {
			Code, Continent string
			Mode            *string
			Offered, Total  int
		} `json:"countries"`
		List struct {
			Total int `json:"total"`
			Items []struct {
				ID      int64 `json:"id"`
				Offered bool  `json:"offered"`
				Broken  bool  `json:"broken"`
			} `json:"items"`
		} `json:"list"`
	}
	decode(t, rec, &preview)
	if preview.Version != 0 || preview.Diff.Added != 1 || preview.Diff.Removed != 1 || preview.List.Total != 3 {
		t.Errorf("preview = %+v, want version 0, one station in (the demarked one), one out (DE), three CH stations", preview)
	}
	for _, it := range preview.List.Items {
		if !it.Offered || it.Broken != (it.ID == vistest.RadioBroken) {
			t.Errorf("preview list item %+v", it)
		}
	}
	for _, c := range preview.Countries {
		if c.Code == "DE" && (c.Mode != nil || c.Offered != 0 || c.Total != 1 || c.Continent != "europe") {
			t.Errorf("DE in the draft = %+v", c)
		}
	}
	// The preview stored nothing: DE is still reachable.
	if rec := send(t, h, http.MethodGet, BasePath+"/radio/stations/"+de, "p6", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("DE after the preview: %d", rec.Code)
	}
	rec = send(t, h, http.MethodPost, BasePath+"/admin/radio/offer/apply", "p18",
		`{"version":0,"steps":[{"op":"removeCountry","country":"DE"}]}`, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":1`) || !strings.Contains(rec.Body.String(), `"demarkedStationIds":[]`) {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body.String())
	}
	for _, token := range []string{"p0", "p6", "p12", "p18"} {
		for _, c := range []struct{ method, path string }{
			{http.MethodGet, "/radio/stations/" + de},
			{http.MethodPost, "/radio/stations/" + de + "/listen"},
			{http.MethodGet, "/radio/images/" + de + "/logo/w256"},
			{http.MethodPut, "/radio/favorites/" + de},
		} {
			rec := send(t, h, c.method, BasePath+c.path, token, "", nil)
			if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
				t.Errorf("%s %s %s: %d %s, want 404 not_found", token, c.method, c.path, rec.Code, rec.Body.String())
			}
		}
		if rec := send(t, h, http.MethodGet, BasePath+"/radio/stations/"+radioCH, token, "", nil); rec.Code != http.StatusOK {
			t.Errorf("%s CH station: %d", token, rec.Code)
		}
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/radio/favorites", "p6", "", nil); strings.Contains(rec.Body.String(), `"id":`+de) {
		t.Errorf("P6 favourites list the German station: %s", rec.Body.String())
	}
	var n int
	if err := pf.f.DB.Read.QueryRow(`SELECT count(*) FROM radio_favorites WHERE station_id = ?`, vistest.RadioDE).Scan(&n); err != nil || n != 1 {
		t.Errorf("German favourite rows = %d (%v), want 1 kept", n, err)
	}
	rec = send(t, h, http.MethodPost, BasePath+"/admin/radio/offer/apply", "p18",
		`{"version":0,"steps":[{"op":"chooseCountry","country":"FR"}]}`, nil)
	if rec.Code != http.StatusConflict || problemCode(t, rec) != "offer_conflict" {
		t.Errorf("apply on the old version: %d %s, want 409 offer_conflict", rec.Code, rec.Body.String())
	}
	rec = send(t, h, http.MethodPost, BasePath+"/admin/radio/offer/apply", "p18",
		`{"version":1,"steps":[{"op":"chooseCountry","country":"ch"}]}`, nil)
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "validation_failed" {
		t.Errorf("apply with a bad country: %d %s, want 422", rec.Code, rec.Body.String())
	}
}

// The body an administration of 1.26 sends to updateRadioConfig ({"countries":null}, also with a
// country list and hideBroken) is answered 200: the strict server ignores the fields the schema no
// longer names, so the offer stays as it is (contract 1.28.0, description of updateRadioConfig).
func TestRadioConfigIgnoresTheOldCountries(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	before := send(t, h, http.MethodGet, BasePath+"/admin/radio/offer", "p18", "", nil)
	if before.Code != http.StatusOK {
		t.Fatalf("offer: %d %s", before.Code, before.Body.String())
	}
	for _, body := range []string{`{"countries":null}`, `{"countries":["FR"],"hideBroken":false,"graceSeconds":60}`} {
		rec := send(t, h, http.MethodPatch, BasePath+"/admin/radio/config", "p18", body, nil)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "countries") || strings.Contains(rec.Body.String(), "hideBroken") {
			t.Errorf("%s: %d %s, want 200 without the old fields", body, rec.Code, rec.Body.String())
		}
	}
	after := send(t, h, http.MethodGet, BasePath+"/admin/radio/offer", "p18", "", nil)
	if after.Body.String() != before.Body.String() || !strings.Contains(after.Body.String(), `"hideBroken":true`) {
		t.Errorf("offer changed by the old body:\nbefore %s\nafter  %s", before.Body.String(), after.Body.String())
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/radio/stations/"+radioExcluded, "p6", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("the French station after countries [FR]: %d, want 404", rec.Code)
	}
}

// Contract 1.28.0 (PB-13): the tag rules, the hand-set facets, the hidden flag and the settings
// countries and hideBroken are gone from the contract; the offer operations are there.
func TestRadioContractDropsTheTagRules(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	paths, _ := doc["paths"].(map[string]any)
	if _, ok := paths["/admin/radio/tag-rules"]; ok {
		t.Error("the path /admin/radio/tag-rules is still in the contract")
	}
	for _, p := range []string{"/admin/radio/offer", "/admin/radio/offer/preview", "/admin/radio/offer/apply"} {
		if _, ok := paths[p]; !ok {
			t.Errorf("the path %s is missing", p)
		}
	}
	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	for name := range schemas {
		if strings.HasPrefix(name, "RadioTagRule") || name == "RadioTagCount" || name == "RadioCoverage" || name == "RadioStationFacet" {
			t.Errorf("schema %s is still in the contract", name)
		}
	}
	props := func(schema string) map[string]any {
		s, _ := schemas[schema].(map[string]any)
		p, _ := s["properties"].(map[string]any)
		if p == nil {
			t.Fatalf("schema %s has no properties", schema)
		}
		return p
	}
	for schema, gone := range map[string][]string{
		"AdminRadioStation":       {"hidden", "facets"},
		"AdminRadioStationPatch":  {"genres", "kinds", "moods", "hidden"},
		"AdminRadioStationCreate": {"genres", "kinds", "moods"},
		"RadioConfig":             {"countries", "hideBroken"},
		"RadioConfigUpdate":       {"countries", "hideBroken"},
	} {
		p := props(schema)
		for _, f := range gone {
			if _, ok := p[f]; ok {
				t.Errorf("%s.%s is still in the contract", schema, f)
			}
		}
	}
	for schema, field := range map[string]string{"AdminRadioStation": "offered", "RadioStation": "provider",
		"RadioPreferences": "effectiveCountry", "RadioFacets": "offerEmpty"} {
		if _, ok := props(schema)[field]; !ok {
			t.Errorf("%s.%s is missing", schema, field)
		}
	}
	lock, _ := schemas["RadioLockField"].(map[string]any)
	enum, _ := lock["enum"].([]any)
	for _, v := range enum {
		if s, _ := v.(string); slices.Contains([]string{"genre", "kind", "mood"}, s) {
			t.Errorf("RadioLockField still has %s", s)
		}
	}
	list, _ := paths["/admin/radio/stations"].(map[string]any)
	get, _ := list["get"].(map[string]any)
	params, _ := get["parameters"].([]any)
	for _, p := range params {
		m, _ := p.(map[string]any)
		if n, _ := m["name"].(string); n == "genre" || n == "exceptions" {
			t.Errorf("listAdminRadioStations still has the parameter %s", n)
		}
	}
}
