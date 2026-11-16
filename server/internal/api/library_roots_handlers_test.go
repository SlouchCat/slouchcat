package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Handler tests of ADR 0015: folders with content and provider list through the contract,
// several folders in one request (F-13), updateLibraryRoot and listMetadataProviders. The rules
// themselves are tested in package library; here the statuses and codes of the contract are.

func TestCreateLibraryWithSeveralFolders(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)

	// A film and a series folder in one go (the "Dokus" case of the concept).
	body := map[string]any{"name": "Dokus", "kind": "video", "roots": []any{
		movieRoot("/media/Doku/Filme"),
		map[string]any{"path": "/media/Doku/Serien", "content": "series", "providers": []string{"tvdb", "tmdb", "anilist"}},
	}}
	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries", body, owner)
	wantStatus(t, rec, http.StatusCreated)
	var lib gen.Library
	if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
		t.Fatal(err)
	}
	if lib.Kind != gen.LibraryKindVideo || len(lib.Roots) != 2 || lib.Roots[0].Content != gen.RootContentMovie ||
		lib.Roots[1].Content != gen.RootContentSeries || len(lib.Roots[1].Providers) != 3 || lib.Roots[1].Providers[0] != gen.MetadataProviderTvdb {
		t.Fatalf("library = %+v", lib)
	}
	// A podcast library may start without a folder; a video library may not.
	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries", map[string]any{"name": "Podcasts", "kind": "podcasts", "roots": []any{}}, owner)
	wantStatus(t, rec, http.StatusCreated)
	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries", map[string]any{"name": "Leer", "kind": "video", "roots": []any{}}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if f := fieldProblem(t, rec); f != "roots roots_required" {
		t.Fatalf("empty video library: %s", f)
	}
	// The old kinds are gone, the shape of a folder is checked.
	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries", map[string]any{"name": "Alt", "kind": "movies", "roots": []any{movieRoot("/media/x")}}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries", map[string]any{"name": "Form", "kind": "video",
		"roots": []any{map[string]any{"path": "/media/x", "content": "movie", "providers": []string{}}}}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if f := fieldProblem(t, rec); f != "roots[0].providers required" {
		t.Fatalf("empty provider list: %s", f)
	}
}

func TestUpdateLibraryRootEndpoint(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)
	const root = "/api/v1/admin/libraries/1/roots/1"

	// A list change needs no confirmation.
	rec = e.call(t, http.MethodPatch, root, map[string]any{"providers": []string{"tmdb", "anilist"}}, owner)
	wantStatus(t, rec, http.StatusOK)
	var lib gen.Library
	if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
		t.Fatal(err)
	}
	if len(lib.Roots) != 1 || len(lib.Roots[0].Providers) != 2 || lib.Roots[0].Providers[1] != gen.MetadataProviderAnilist {
		t.Fatalf("root after the list change = %+v", lib.Roots)
	}
	// A content change without the typed path, with a wrong one, under a running scan, and done.
	rec = e.call(t, http.MethodPatch, root, map[string]any{"content": "series"}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if code := problemCode(t, rec); code != "confirm_mismatch" {
		t.Fatalf("content change without confirmation: %s", code)
	}
	rec = e.call(t, http.MethodPatch, root, map[string]any{"content": "series", "confirmPath": "/media/Serien"}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	e.libs.scanRunning = true
	rec = e.call(t, http.MethodPatch, root, map[string]any{"content": "series", "confirmPath": "/media/Filme"}, owner)
	wantStatus(t, rec, http.StatusConflict)
	if code := problemCode(t, rec); code != "scan_running" {
		t.Fatalf("content change under a scan: %s", code)
	}
	e.libs.scanRunning = false
	rec = e.call(t, http.MethodPatch, root, map[string]any{"content": "series", "confirmPath": "/media/Filme", "providers": []string{"tmdb", "tvdb"}}, owner)
	wantStatus(t, rec, http.StatusOK)
	if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
		t.Fatal(err)
	}
	if lib.Roots[0].Content != gen.RootContentSeries {
		t.Fatalf("root after the content change = %+v", lib.Roots[0])
	}
	// A new path (ADR 0023): not with a content change, not under a running scan, with a list.
	rec = e.call(t, http.MethodPatch, root, map[string]any{"path": "/media/Serien-neu", "content": "movie", "confirmPath": "/media/Filme"}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if f := fieldProblem(t, rec); f != "path path_with_content" {
		t.Fatalf("path with content: %s", f)
	}
	e.libs.scanRunning = true
	rec = e.call(t, http.MethodPatch, root, map[string]any{"path": "/media/Serien-neu"}, owner)
	wantStatus(t, rec, http.StatusConflict)
	e.libs.scanRunning = false
	rec = e.call(t, http.MethodPatch, root, map[string]any{"path": "/media/Serien-neu", "providers": []string{"tvdb", "tmdb"}}, owner)
	wantStatus(t, rec, http.StatusOK)
	if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
		t.Fatal(err)
	}
	if r := lib.Roots[0]; r.Path != "/media/Serien-neu" || r.Content != gen.RootContentSeries || r.Providers[0] != gen.MetadataProviderTvdb {
		t.Fatalf("root after the path change = %+v", r)
	}
	rec = e.call(t, http.MethodPatch, root, map[string]any{"path": "/" + strings.Repeat("x", 4096)}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if f := fieldProblem(t, rec); f != "path too_long" {
		t.Fatalf("long path: %s", f)
	}
	// Nothing to change, an unknown folder, an unknown value.
	rec = e.call(t, http.MethodPatch, root, map[string]any{}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	rec = e.call(t, http.MethodPatch, "/api/v1/admin/libraries/1/roots/99", map[string]any{"providers": []string{"tmdb"}}, owner)
	wantStatus(t, rec, http.StatusNotFound)
	rec = e.call(t, http.MethodPatch, root, map[string]any{"content": "music", "confirmPath": "/media/Filme"}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if f := fieldProblem(t, rec); f != "content invalid_value" {
		t.Fatalf("unknown content: %s", f)
	}
}

func TestListMetadataProvidersEndpoint(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)
	rec = e.call(t, http.MethodGet, "/api/v1/admin/metadata/providers", nil, owner)
	wantStatus(t, rec, http.StatusOK)
	var infos []gen.MetadataProviderInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &infos); err != nil {
		t.Fatal(err)
	}
	byID := map[gen.MetadataProvider]gen.MetadataProviderInfo{}
	for _, p := range infos {
		byID[p.Id] = p
	}
	if len(infos) != 9 {
		t.Fatalf("providers = %+v", infos)
	}
	// The wizard stored a TMDB key, no TVDB key exists: TVDB is listed but not configured.
	if !byID[gen.MetadataProviderTmdb].Configured || byID[gen.MetadataProviderTvdb].Configured {
		t.Fatalf("configured: tmdb %v, tvdb %v", byID[gen.MetadataProviderTmdb].Configured, byID[gen.MetadataProviderTvdb].Configured)
	}
	if tvdb := byID[gen.MetadataProviderTvdb]; len(tvdb.Contents) != 1 || tvdb.Contents[0] != gen.RootContentSeries || len(tvdb.Roles) != 2 {
		t.Fatalf("tvdb = %+v", tvdb)
	}
	if ani := byID[gen.MetadataProviderAnilist]; !ani.Configured || len(ani.Roles) != 1 || ani.Roles[0] != gen.Fill ||
		len(ani.Languages) != 1 || ani.Languages[0] != "en" || len(ani.Contents) != 2 {
		t.Fatalf("anilist = %+v", ani)
	}
	if local := byID[gen.MetadataProviderLocal]; !local.Configured || len(local.Contents) != 2 || local.Contents[0] != gen.RootContentAudiobook {
		t.Fatalf("local = %+v", local)
	}
	// The book sources of concept 19 D6 fill audiobook folders; only Google Books needs a key,
	// and the test server has none.
	for _, id := range []gen.MetadataProvider{gen.MetadataProviderAudible, gen.MetadataProviderDnb, gen.MetadataProviderApplebooks,
		gen.MetadataProviderOpenlibrary, gen.MetadataProviderGooglebooks} {
		book, ok := byID[id]
		if !ok || len(book.Roles) != 1 || book.Roles[0] != gen.Fill || len(book.Contents) != 1 || book.Contents[0] != gen.RootContentAudiobook ||
			len(book.Languages) != 0 || book.Configured != (id != gen.MetadataProviderGooglebooks) {
			t.Fatalf("%s = %+v", id, book)
		}
	}
}

// Concept 24 section 3.6 (contract 1.40.0): a folder's own text languages and images through the
// contract. A new folder takes them or follows the default (null); on a change each has three
// states: left out = unchanged, null = like the default, a value = the folder's own, and an own
// image setting with no kind on is kept as such.
func TestFolderMetadataThroughTheContract(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)
	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries", map[string]any{"name": "Anime", "kind": "video", "roots": []any{
		map[string]any{"path": "/media/Anime", "content": "series", "providers": []string{"tvdb", "tmdb"}, "textLangs": []string{"ja", "en"}},
	}}, owner)
	wantStatus(t, rec, http.StatusCreated)
	root := func(rec *httptest.ResponseRecorder) map[string]json.RawMessage {
		t.Helper()
		var lib struct {
			ID    int64                        `json:"id"`
			Roots []map[string]json.RawMessage `json:"roots"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil || len(lib.Roots) == 0 {
			t.Fatalf("library %s: %v", rec.Body.String(), err)
		}
		return lib.Roots[len(lib.Roots)-1]
	}
	got := root(rec)
	if string(got["textLangs"]) != `["ja","en"]` || string(got["images"]) != "null" || string(got["homeTitleCount"]) != "0" {
		t.Fatalf("new folder: textLangs %s images %s homeTitleCount %s", got["textLangs"], got["images"], got["homeTitleCount"])
	}
	var lib gen.Library
	if err := json.Unmarshal(rec.Body.Bytes(), &lib); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/v1/admin/libraries/%d/roots/%d", lib.Id, lib.Roots[0].Id)

	// Own images with no kind on; textLangs left out stays.
	rec = e.call(t, http.MethodPatch, path, map[string]any{"images": map[string]any{"langs": map[string]any{"poster": []string{"", "ja"}}, "kinds": []string{}}}, owner)
	wantStatus(t, rec, http.StatusOK)
	got = root(rec)
	if string(got["textLangs"]) != `["ja","en"]` || !strings.Contains(string(got["images"]), `"kinds":[]`) || !strings.Contains(string(got["images"]), `"poster":["","ja"]`) {
		t.Fatalf("after the images: textLangs %s images %s", got["textLangs"], got["images"])
	}
	// null = like the default, for each field on its own.
	rec = e.call(t, http.MethodPatch, path, map[string]any{"textLangs": nil}, owner)
	wantStatus(t, rec, http.StatusOK)
	got = root(rec)
	if string(got["textLangs"]) != "null" || string(got["images"]) == "null" {
		t.Fatalf("after textLangs null: textLangs %s images %s", got["textLangs"], got["images"])
	}
	// Not a list: a field error, nothing written.
	rec = e.call(t, http.MethodPatch, path, map[string]any{"textLangs": "ja"}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if f := fieldProblem(t, rec); f != "textLangs invalid_value" {
		t.Fatalf("textLangs as a string: %s", f)
	}
}

// TestSaveLibraryEndpoint: saveLibrary through the contract (concept 24 4.3). The rules live in
// package library and are tested there; here the mapping of the body and the statuses and codes
// of the contract are.
func TestSaveLibraryEndpoint(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)
	const lib = "/api/v1/admin/libraries/1"
	// The fake library service holds library 1; package admin reads its row for the settings.
	if _, err := e.db.Write.Exec(`INSERT INTO libraries (id, name, kind, created_at, updated_at) VALUES (1, 'Filme', 'video', 1, 1)`); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"revision": "r1", "name": "Kino",
		"roots": []any{
			map[string]any{"id": 1, "path": "/media/Filme", "content": "series", "providers": []string{"tmdb"}, "confirmPath": "/media/Filme",
				"textLangs": []string{"ja", "en"}, "images": map[string]any{"langs": map[string]any{"poster": []string{""}}, "kinds": []string{}}},
			movieRoot("/media/Neu"),
		},
		"removeRoots": []any{map[string]any{"id": 7, "confirmPath": "/media/Alt"}},
	}

	rec = e.call(t, http.MethodPut, lib, body, owner)
	wantStatus(t, rec, http.StatusOK)
	var saved gen.LibrarySaved
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Library.Id != 1 || len(saved.MetadataChanged) != 1 || saved.MetadataChanged[0].RootId != 1 || saved.MetadataChanged[0].HomeTitleCount != 3 {
		t.Fatalf("answer = %s", rec.Body.String())
	}
	got := e.libs.saved
	if got.Revision != "r1" || len(got.Roots) != 2 || len(got.Remove) != 1 || got.Remove[0].ID != 7 || got.Remove[0].ConfirmPath != "/media/Alt" {
		t.Fatalf("request = %+v", got)
	}
	if r := got.Roots[0]; r.ID != 1 || r.Content != "series" || r.ConfirmPath != "/media/Filme" || r.Metadata.TextLangs == nil ||
		len(*r.Metadata.TextLangs) != 2 || r.Metadata.Images == nil || len(r.Metadata.Images.Langs["poster"]) != 1 {
		t.Fatalf("kept folder = %+v", r)
	}
	if r := got.Roots[1]; r.ID != 0 || r.Path != "/media/Neu" || r.Metadata.TextLangs != nil || r.Metadata.Images != nil {
		t.Fatalf("new folder = %+v", r)
	}
	var name string
	if err := e.db.Read.QueryRow(`SELECT name FROM libraries WHERE id = 1`).Scan(&name); err != nil || name != "Kino" {
		t.Fatalf("name = %q %v", name, err)
	}

	// The three conflicts and a field error; the name written by the settings step goes back.
	body["name"] = "Anders"
	for _, c := range []struct {
		err  error
		code string
	}{{model.ErrStale, "stale"}, {model.ErrRootUnlisted, "root_unlisted"}, {model.ErrScanRunning, "scan_running"}} {
		e.libs.saveErr = c.err
		rec = e.call(t, http.MethodPut, lib, body, owner)
		wantStatus(t, rec, http.StatusConflict)
		if code := problemCode(t, rec); code != c.code {
			t.Errorf("%v: code %s, want %s", c.err, code, c.code)
		}
	}
	e.libs.saveErr = (&model.ValidationError{}).Add("roots[1].path", "root_overlap")
	rec = e.call(t, http.MethodPut, lib, body, owner)
	if f := fieldProblem(t, rec); f != "roots[1].path root_overlap" {
		t.Fatalf("field error: %s", f)
	}
	e.libs.saveErr = nil
	if err := e.db.Read.QueryRow(`SELECT name FROM libraries WHERE id = 1`).Scan(&name); err != nil || name != "Kino" {
		t.Fatalf("name after the refused saves = %q %v", name, err)
	}
	// Library fields are checked before the folders, with the names of updateLibrary.
	body["name"] = " "
	rec = e.call(t, http.MethodPut, lib, body, owner)
	if f := fieldProblem(t, rec); f != "name required" {
		t.Fatalf("empty name: %s", f)
	}
	rec = e.call(t, http.MethodPut, "/api/v1/admin/libraries/99", body, owner)
	wantStatus(t, rec, http.StatusNotFound)
}
