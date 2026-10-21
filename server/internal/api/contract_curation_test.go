package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// Curation contract of M7 (design 3.2): Cat's picks (E-2, E-3), collections (E-6) and the order of
// the start page rows (E-9), all as the owner with the primary profile (p18).

// fieldProblem returns "field code" of a 422 validation_failed answer with exactly one field error.
func fieldProblem(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d (%s), want 422", rec.Code, rec.Body.String())
	}
	p := problemOf(t, rec)
	if p.Code != "validation_failed" || p.Errors == nil || len(*p.Errors) != 1 {
		t.Fatalf("problem %s, want validation_failed with one field", rec.Body.String())
	}
	return (*p.Errors)[0].Field + " " + (*p.Errors)[0].Code
}

// adminCurationAnswer checks what every curation answer must hold: no ItemRef and no image URL.
func adminCurationAnswer(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d (%s), want %d", rec.Code, rec.Body.String(), status)
	}
	if refs := itemRefsIn(t, rec); len(refs) != 0 {
		t.Errorf("answer contains ItemRef objects %v", refs)
	}
	if strings.Contains(rec.Body.String(), "/images/") {
		t.Errorf("answer contains image URLs: %s", rec.Body.String())
	}
}

// picksOf returns the ids of the picks row of a profile's start page.
func picksOf(t *testing.T, h http.Handler, token string) []string {
	t.Helper()
	rec := send(t, h, http.MethodGet, BasePath+"/home", token, "", nil)
	wantStatus(t, rec, http.StatusOK)
	_, rows := homeRows(t, rec)
	return rows["picks"]
}

func TestContractAdminPicks(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	create := func(body string) []gen.AdminPick {
		t.Helper()
		rec := send(t, h, http.MethodPost, BasePath+"/admin/picks", "p18", body, nil)
		adminCurationAnswer(t, rec, http.StatusCreated)
		var out []gen.AdminPick
		decode(t, rec, &out)
		return out
	}
	list := func() []gen.AdminPick {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/admin/picks", "p18", "", nil)
		adminCurationAnswer(t, rec, http.StatusOK)
		var out []gen.AdminPick
		decode(t, rec, &out)
		return out
	}

	global := create(`{"itemId":"` + f.Rated(0).ID + `","note":"Für alle"}`)
	if len(global) != 1 || global[0].ProfileId != nil || global[0].Note == nil || *global[0].Note != "Für alle" ||
		global[0].Title != "Film 00" || global[0].Kind != "movie" {
		t.Fatalf("global pick %+v", global)
	}
	forP12 := create(fmt.Sprintf(`{"itemId":"%s","profileIds":[%d]}`, f.Rated(12).ID, f.P12.ID))
	if len(forP12) != 1 || forP12[0].ProfileId == nil || *forP12[0].ProfileId != f.P12.ID || forP12[0].ProfileName == nil || *forP12[0].ProfileName != "P12" {
		t.Fatalf("pick for P12 %+v", forP12)
	}
	// E-3: a pick for P12 of a title P12 may not see never reaches P12.
	create(fmt.Sprintf(`{"itemId":"%s","profileIds":[%d]}`, f.Rated(18).ID, f.P12.ID))
	// E-2: outside its window a pick is invisible.
	create(`{"itemId":"` + f.Rated(16).ID + `","startsAt":"1999-01-01T00:00:00Z","endsAt":"2000-01-01T00:00:00Z"}`)
	create(`{"itemId":"` + f.Show.Series.ID + `","startsAt":"2100-01-01T00:00:00Z"}`)

	for body, want := range map[string]string{
		`{"itemId":"` + unknownItemID + `"}`:                                                                   "itemId unknown_item",
		`{"itemId":"` + f.Show.Season.ID + `"}`:                                                                "itemId invalid_value",
		`{"itemId":"` + f.Show.Extra.ID + `"}`:                                                                 "itemId invalid_value",
		fmt.Sprintf(`{"itemId":"%s","profileIds":[%d]}`, f.Rated(0).ID, f.PG.ID):                               "profileIds invalid_value",
		`{"itemId":"` + f.Rated(0).ID + `","note":"` + strings.Repeat("x", 281) + `"}`:                         "note too_long",
		`{"itemId":"` + f.Rated(0).ID + `","startsAt":"2026-02-01T00:00:00Z","endsAt":"2026-01-01T00:00:00Z"}`: "endsAt invalid_value",
	} {
		if got := fieldProblem(t, send(t, h, http.MethodPost, BasePath+"/admin/picks", "p18", body, nil)); got != want {
			t.Errorf("POST %s: %s, want %s", body, got, want)
		}
	}

	p12 := picksOf(t, h, "p12")
	for _, m := range []string{f.Rated(0).ID, f.Rated(6).ID, f.Rated(12).ID} {
		if !slices.Contains(p12, m) {
			t.Errorf("P12 picks %v miss %s", p12, m)
		}
	}
	for _, m := range []string{f.Rated(18).ID, f.Rated(16).ID, f.Show.Series.ID} {
		if slices.Contains(p12, m) {
			t.Errorf("P12 picks %v contain %s", p12, m)
		}
	}
	if p16 := picksOf(t, h, "p16"); slices.Contains(p16, f.Rated(12).ID) || !slices.Contains(p16, f.Rated(0).ID) {
		t.Errorf("P16 picks %v: the pick for P12 leaked or the global one is missing", p16)
	}

	// Reorder: the global pick of the film rated 0 first.
	all := list()
	ids := []int64{global[0].Id}
	for _, p := range all {
		if p.Id != global[0].Id {
			ids = append(ids, p.Id)
		}
	}
	order := func(ids []int64) string {
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			parts = append(parts, fmt.Sprint(id))
		}
		return `{"ids":[` + strings.Join(parts, ",") + `]}`
	}
	rec := send(t, h, http.MethodPut, BasePath+"/admin/picks/order", "p18", order(ids), nil)
	wantStatus(t, rec, http.StatusNoContent)
	if got := list(); got[0].Id != global[0].Id || got[0].Position != 0 {
		t.Errorf("first pick after reorder %+v", got[0])
	}
	if p16 := picksOf(t, h, "p16"); len(p16) == 0 || p16[0] != f.Rated(0).ID {
		t.Errorf("P16 picks after reorder %v, want the film rated 0 first", p16)
	}
	if got := fieldProblem(t, send(t, h, http.MethodPut, BasePath+"/admin/picks/order", "p18", order(ids[1:]), nil)); got != "ids invalid_value" {
		t.Errorf("reorder without one pick: %s", got)
	}

	// PATCH: absent keeps, null clears.
	path := fmt.Sprintf("%s/admin/picks/%d", BasePath, global[0].Id)
	patch := func(body string) gen.AdminPick {
		t.Helper()
		rec := send(t, h, http.MethodPatch, path, "p18", body, nil)
		adminCurationAnswer(t, rec, http.StatusOK)
		var p gen.AdminPick
		decode(t, rec, &p)
		return p
	}
	if p := patch(`{"endsAt":"2000-01-01T00:00:00Z"}`); p.Note == nil || *p.Note != "Für alle" || p.EndsAt == nil {
		t.Errorf("after setting endsAt %+v, want the note kept", p)
	}
	if slices.Contains(picksOf(t, h, "p16"), f.Rated(0).ID) {
		t.Error("an expired pick is still on the start page")
	}
	if p := patch(`{"endsAt":null,"note":null}`); p.Note != nil || p.EndsAt != nil {
		t.Errorf("after clearing %+v", p)
	}
	if !slices.Contains(picksOf(t, h, "p16"), f.Rated(0).ID) {
		t.Error("the pick did not come back after clearing endsAt")
	}
	if got := fieldProblem(t, send(t, h, http.MethodPatch, path, "p18", `{"startsAt":"2026-02-01T00:00:00Z","endsAt":"2026-01-01T00:00:00Z"}`, nil)); got != "endsAt invalid_value" {
		t.Errorf("PATCH with an empty window: %s", got)
	}
	if rec := send(t, h, http.MethodPatch, BasePath+"/admin/picks/9999", "p18", `{"note":"x"}`, nil); rec.Code != http.StatusNotFound {
		t.Errorf("PATCH unknown pick: %d", rec.Code)
	}

	wantStatus(t, send(t, h, http.MethodDelete, path, "p18", "", nil), http.StatusNoContent)
	wantStatus(t, send(t, h, http.MethodDelete, path, "p18", "", nil), http.StatusNotFound)
	if got := list(); len(got) != len(all)-1 {
		t.Errorf("%d picks after the delete, want %d", len(got), len(all)-1)
	}
}

func TestContractAdminCollections(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	call := func(method, path, body string, status int) gen.AdminCollection {
		t.Helper()
		rec := send(t, h, method, BasePath+path, "p18", body, nil)
		adminCurationAnswer(t, rec, status)
		var c gen.AdminCollection
		if status != http.StatusNoContent {
			decode(t, rec, &c)
		}
		return c
	}

	if got := fieldProblem(t, send(t, h, http.MethodPost, BasePath+"/admin/collections", "p18", `{"name":" "}`, nil)); got != "name required" {
		t.Errorf("empty name: %s", got)
	}
	c := call(http.MethodPost, "/admin/collections", `{"name":"Meine Sammlung"}`, http.StatusCreated)
	if c.Source != "manual" || c.Name != "Meine Sammlung" || c.ItemCount != 0 || c.Items == nil {
		t.Fatalf("created %+v", c)
	}
	items := "/admin/collections/" + c.Id + "/items"
	members := []string{f.Rated(18).ID, f.Show.Series.ID, f.Rated(6).ID}
	c = call(http.MethodPut, items, `{"itemIds":["`+strings.Join(members, `","`)+`"]}`, http.StatusOK)
	var got []string
	for _, it := range c.Items {
		got = append(got, it.ItemId)
	}
	if !slices.Equal(got, members) || c.ItemCount != 3 || c.Items[2].Position != 2 || c.Items[1].Kind != "series" {
		t.Errorf("members %+v", c)
	}
	for body, want := range map[string]string{
		`{"itemIds":["` + unknownItemID + `"]}`:    "itemIds unknown_item",
		`{"itemIds":["` + f.Show.E1.ID + `"]}`:     "itemIds invalid_value",
		`{"itemIds":["` + f.Show.Season.ID + `"]}`: "itemIds invalid_value",
	} {
		if got := fieldProblem(t, send(t, h, http.MethodPut, BasePath+items, "p18", body, nil)); got != want {
			t.Errorf("PUT %s: %s, want %s", body, got, want)
		}
	}
	if c = call(http.MethodPatch, "/admin/collections/"+c.Id, `{"name":"Umbenannt"}`, http.StatusOK); c.Name != "Umbenannt" || len(c.Items) != 3 {
		t.Errorf("renamed %+v", c)
	}
	if c = call(http.MethodGet, "/admin/collections/"+c.Id, "", http.StatusOK); len(c.Items) != 3 || c.Items[0].ItemId != f.Rated(18).ID {
		t.Errorf("get %+v", c)
	}

	// A TMDB collection (written by the metadata match) is read-only.
	tmdbID := "01J9000000000000000000TMDB"
	if _, err := f.DB.Write.Exec(`INSERT INTO collections (id, source, tmdb_id, name, name_de, created_at, updated_at) VALUES (?, 'tmdb', 10, 'Saga', 'Saga DE', 1, 1)`, tmdbID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Write.Exec(`INSERT INTO collection_items (collection_id, item_id, position, created_at) VALUES (?, ?, 0, 1)`, tmdbID, f.Rated(12).ID); err != nil {
		t.Fatal(err)
	}
	if tc := call(http.MethodGet, "/admin/collections/"+tmdbID, "", http.StatusOK); tc.Source != "tmdb" || tc.TmdbId == nil || *tc.TmdbId != 10 || len(tc.Items) != 1 {
		t.Errorf("tmdb collection %+v", tc)
	}
	for _, r := range []struct{ method, path, body string }{
		{http.MethodPatch, "/admin/collections/" + tmdbID, `{"name":"Neu"}`},
		{http.MethodDelete, "/admin/collections/" + tmdbID, ""},
		{http.MethodPut, "/admin/collections/" + tmdbID + "/items", `{"itemIds":[]}`},
	} {
		rec := send(t, h, r.method, BasePath+r.path, "p18", r.body, nil)
		if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "not_editable" {
			t.Errorf("%s %s: %d %s, want 422 not_editable", r.method, r.path, rec.Code, rec.Body.String())
		}
	}

	rec := send(t, h, http.MethodGet, BasePath+"/admin/collections", "p18", "", nil)
	adminCurationAnswer(t, rec, http.StatusOK)
	var all []gen.AdminCollection
	decode(t, rec, &all)
	// The fixture (vistest.AddDiscover) carries collections of its own; the list is sorted by name,
	// carries no members and includes the TMDB one and the renamed manual one.
	byName := map[string]gen.AdminCollection{}
	for i, c := range all {
		byName[c.Name] = c
		if len(c.Items) != 0 {
			t.Errorf("list entry %s carries members", c.Name)
		}
		if i > 0 && all[i-1].Name > c.Name {
			t.Errorf("list not sorted by name: %s before %s", all[i-1].Name, c.Name)
		}
	}
	if _, ok := byName["Saga"]; !ok {
		t.Errorf("list lacks the TMDB collection: %+v", all)
	}
	if ren, ok := byName["Umbenannt"]; !ok || ren.ItemCount != 3 {
		t.Errorf("list lacks the renamed manual collection with 3 members: %+v", all)
	}

	call(http.MethodDelete, "/admin/collections/"+c.Id, "", http.StatusNoContent)
	wantStatus(t, send(t, h, http.MethodGet, BasePath+"/admin/collections/"+c.Id, "p18", "", nil), http.StatusNotFound)
}

// E-9: the stored order and the hidden flag reach the start page.
func TestContractHomeRows(t *testing.T) {
	pf := newProfileFixture(t)
	h := pf.h
	rec := send(t, h, http.MethodGet, BasePath+"/admin/home-rows", "p18", "", nil)
	adminCurationAnswer(t, rec, http.StatusOK)
	var rows []gen.HomeRowSetting
	decode(t, rec, &rows)
	if len(rows) != 8 || rows[0].Key != "picks" || rows[7].Key != "trakt_popular" {
		t.Fatalf("default rows %+v", rows)
	}

	set := func(order []string, hidden string) {
		t.Helper()
		parts := make([]string, 0, len(order))
		for i, k := range order {
			parts = append(parts, fmt.Sprintf(`{"key":%q,"position":%d,"hidden":%t}`, k, i, k == hidden))
		}
		rec := send(t, h, http.MethodPut, BasePath+"/admin/home-rows", "p18", "["+strings.Join(parts, ",")+"]", nil)
		adminCurationAnswer(t, rec, http.StatusOK)
		var got []gen.HomeRowSetting
		decode(t, rec, &got)
		for i, r := range got {
			if string(r.Key) != order[i] || r.Position != i || r.Hidden != (order[i] == hidden) {
				t.Fatalf("stored rows %+v, want order %v", got, order)
			}
		}
	}
	// homeKeys returns the row keys of P18's start page that are in order, checking that every
	// key appears in the stored order.
	homeKeys := func(order []string) []string {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/home", "p18", "", nil)
		wantStatus(t, rec, http.StatusOK)
		keys, _ := homeRows(t, rec)
		last := -1
		for _, k := range keys {
			i := slices.Index(order, k)
			if i <= last {
				t.Fatalf("start page rows %v do not follow the stored order %v", keys, order)
			}
			last = i
		}
		return keys
	}

	order := []string{"recent", "continue", "picks", "watchlist", "collections", "genres", "trakt_trending", "trakt_popular"}
	set(order, "")
	keys := homeKeys(order)
	if !slices.Contains(keys, "recent") || !slices.Contains(keys, "picks") || keys[0] != "recent" {
		t.Errorf("P18 rows %v, want recent first and picks present", keys)
	}
	set(order, "recent")
	if keys := homeKeys(order); slices.Contains(keys, "recent") || keys[0] != "continue" {
		t.Errorf("P18 rows %v with recent hidden", keys)
	}

	for name, body := range map[string]string{
		"seven rows":  `[{"key":"picks","position":0,"hidden":false}]`,
		"twice picks": `[{"key":"picks","position":0,"hidden":false},{"key":"picks","position":1,"hidden":false},{"key":"recent","position":2,"hidden":false},{"key":"watchlist","position":3,"hidden":false},{"key":"collections","position":4,"hidden":false},{"key":"genres","position":5,"hidden":false},{"key":"trakt_trending","position":6,"hidden":false},{"key":"trakt_popular","position":7,"hidden":false}]`,
		"empty":       `[]`,
	} {
		if got := fieldProblem(t, send(t, h, http.MethodPut, BasePath+"/admin/home-rows", "p18", body, nil)); got != "rows invalid_value" {
			t.Errorf("%s: %s", name, got)
		}
	}
}
