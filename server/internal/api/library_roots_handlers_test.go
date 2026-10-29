package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
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
	if len(infos) != 4 {
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
}
