package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Collaborators of the admin service in the contract fixture (M3): package metadata and the scan
// cancellation and file removal of package library are faked; they record nothing and change no
// data, so the admin registry can run against the shared fixture.

type contractMetadata struct{}

func (contractMetadata) Search(context.Context, string, string, string, *int) ([]model.MetadataCandidate, error) {
	return nil, model.ErrProviderNotConfigured
}
func (contractMetadata) AssignMatch(context.Context, string, string, string) error { return nil }
func (contractMetadata) Refresh(context.Context, string) error                     { return nil }
func (contractMetadata) AddManualTrailer(_ context.Context, _, lang, youtubeID string) (model.Trailer, error) {
	return model.Trailer{ID: 1, Lang: lang, YouTubeID: youtubeID, Source: "manual"}, nil
}

func (contractMetadata) ProviderLanguages(context.Context) (map[string][]string, error) {
	return map[string][]string{"tmdb": {"de", "en"}, "tvdb": {"de", "en"}, "anilist": {"en"}}, nil
}
func (contractMetadata) DeleteTrailer(context.Context, string, int64) error { return model.ErrNotFound }
func (contractMetadata) ListTrailersSystem(context.Context, string) ([]model.Trailer, error) {
	return []model.Trailer{{ID: 7, Lang: "de", YouTubeID: "DeTrailer06", Source: "youtube"}}, nil
}

type contractLibraryMaintenance struct{}

func (contractLibraryMaintenance) CancelScan(context.Context, int64) (model.Library, error) {
	return model.Library{}, model.ErrNotFound
}
func (contractLibraryMaintenance) IgnoreFiles(_ context.Context, ids []string, _ string) (files, itemsDeleted int, err error) {
	return len(ids), 0, nil
}

// The contract fixture has no soft-deleted title with a live copy: the list is empty, and a
// requested pair is no pair, so it is skipped (the Go tests of admin and library relink real
// pairs).
func (contractLibraryMaintenance) ListRelinkPairs(context.Context, int64) ([]library.RelinkPair, error) {
	return []library.RelinkPair{}, nil
}
func (contractLibraryMaintenance) CountRelinkUserData(context.Context, []string) (map[string]library.RelinkUserData, error) {
	return map[string]library.RelinkUserData{}, nil
}
func (contractLibraryMaintenance) CheckRelinkPair(context.Context, string, string) (int64, error) {
	return 0, library.ErrNotTwins
}
func (contractLibraryMaintenance) Relink(context.Context, string, string) (library.RelinkResult, error) {
	return library.RelinkResult{}, library.ErrNotTwins
}

// itemRefsIn returns the ids of the JSON objects in the response that have the shape of an
// ItemRef (an id with images); administration answers must not contain any.
func itemRefsIn(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	var refs []string
	if rec.Body.Len() == 0 || rec.Code >= 300 {
		return refs
	}
	var doc any
	decode(t, rec, &doc)
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			if _, ok := v["images"]; ok {
				refs = append(refs, fmt.Sprint(v["id"]))
			}
			for _, c := range v {
				walk(c)
			}
		case []any:
			for _, c := range v {
				walk(c)
			}
		}
	}
	walk(doc)
	return refs
}

// The administration view and the provider errors through the HTTP layer.
func TestContractAdminItemAndProviderErrors(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	rec := send(t, h, http.MethodGet, BasePath+"/admin/items/"+fixtureFilm6, "p18", "", nil)
	var it struct {
		ID         string `json:"id"`
		Kind       string `json:"kind"`
		LibraryID  int64  `json:"libraryId"`
		MediaFiles []struct {
			ID     string `json:"id"`
			Path   string `json:"path"`
			RootID int64  `json:"rootId"`
		} `json:"mediaFiles"`
		Trailers []struct {
			EmbedURL string `json:"embedUrl"`
		} `json:"trailers"`
		Children struct {
			Extras int `json:"extras"`
		} `json:"children"`
		LockedFields []string `json:"lockedFields"`
	}
	decode(t, rec, &it)
	m6 := pf.f.Rated(6)
	if it.ID != m6.ID || it.Kind != "movie" || it.LibraryID != pf.f.LibraryID() || len(it.MediaFiles) != 1 || it.MediaFiles[0].Path != m6.RelPath ||
		it.MediaFiles[0].RootID != pf.f.RootID() || it.Children.Extras != 2 || len(it.Trailers) != 1 || it.LockedFields == nil {
		t.Errorf("getAdminItem = %s", rec.Body.String())
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/admin/items/"+unknownItemID, "p18", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown title: %d", rec.Code)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/admin/metadata/search?provider=tvdb&kind=series&query=Show", "p18", "", nil)
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "provider_not_configured" {
		t.Errorf("unconfigured provider: %d %s", rec.Code, rec.Body.String())
	}
	rec = send(t, h, http.MethodPut, BasePath+"/admin/items/"+fixtureSeries+"/match", "p18", `{"provider":"tmdb","providerId":"x"}`, nil)
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "validation_failed" {
		t.Errorf("invalid provider id: %d %s", rec.Code, rec.Body.String())
	}
	rec = send(t, h, http.MethodPost, BasePath+"/admin/maintenance/cleanup", "p18", `{"mediaFileIds":["x"]}`, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"files\":1,\"items\":0}\n" {
		t.Errorf("cleanup of a file: %d %s", rec.Code, rec.Body.String())
	}
	rec = send(t, h, http.MethodGet, BasePath+"/admin/maintenance/issues?kind=orphaned&limit=1", "p18", "", nil)
	var page struct {
		Total int `json:"total"`
		Items []struct {
			Detail string `json:"detail"`
		} `json:"items"`
	}
	decode(t, rec, &page)
	if rec.Code != http.StatusOK || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Detail != "files_missing" {
		t.Errorf("orphaned issues: %d %s", rec.Code, rec.Body.String())
	}
}

// A-3 (review I-5): DELETE /admin/jobs/{id} dismisses a failed job, which the dashboard lists with
// "Abbrechen"; a cancelled one answers 404.
func TestContractCancelFailedJob(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	ctx := context.Background()
	id, err := pf.queue.Enqueue(ctx, jobs.NewJob{Type: "contract.fail", MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	j, err := pf.queue.Claim(ctx, "contract", []string{"contract.fail"})
	if err != nil || j == nil || j.ID != id {
		t.Fatalf("claim: %v, %v", j, err)
	}
	if err := pf.queue.Fail(ctx, "contract", j, errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("%s/admin/jobs/%d", BasePath, id)
	if rec := send(t, pf.h, http.MethodDelete, path, "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Errorf("cancel a failed job: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(t, pf.h, http.MethodDelete, path, "p18", "", nil); rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
		t.Errorf("cancel it again: %d %s", rec.Code, rec.Body.String())
	}
}

// Metadata settings of concept 10 (ADR 0018): the fake answers with the defaults and records nothing.
func (contractMetadata) MetadataSettings(context.Context) (model.MetadataSettings, error) {
	return model.MetadataSettings{TextLangs: []string{"de", "en"}, ImageLangs: map[string][]string{}, ImageMinWidth: map[string]int{}, RatingCountry: "DE"}, nil
}

func (m contractMetadata) UpdateMetadataSettings(ctx context.Context, _ model.MetadataSettings) (model.MetadataSettings, error) {
	return m.MetadataSettings(ctx)
}

func (m contractMetadata) LibraryMetadata(ctx context.Context, libraryID int64) (model.LibraryMetadata, error) {
	if libraryID > 100 {
		return model.LibraryMetadata{}, model.ErrNotFound
	}
	eff, _ := m.MetadataSettings(ctx)
	return model.LibraryMetadata{LibraryID: libraryID, Effective: eff}, nil
}

func (m contractMetadata) UpdateLibraryMetadata(ctx context.Context, libraryID int64, _ *[]string, _ *model.LibraryImages) (model.LibraryMetadata, error) {
	return m.LibraryMetadata(ctx, libraryID)
}

func (contractMetadata) Apply(context.Context, model.ApplyMetadataRequest) (model.ApplyMetadataResult, error) {
	return model.ApplyMetadataResult{JobID: 1, Titles: 0}, nil
}

// The image choice per title (concept 10 step 5).
func (m contractMetadata) ImageCandidates(context.Context, string, string, bool) (model.ImageCandidateList, error) {
	return model.ImageCandidateList{Candidates: []model.ImageCandidate{{Provider: "tmdb", Lang: "de", Width: 1000, Height: 1500, Source: "/p.jpg", PreviewURL: "https://image.tmdb.org/t/p/w300/p.jpg"}, {Provider: "local", Width: 600, Height: 900, Source: "file:Film/poster.jpg?1-1"}}, Unavailable: []string{}}, nil
}
func (m contractMetadata) SetItemImage(context.Context, string, string, string, string) error {
	return nil
}
func (m contractMetadata) ClearItemImage(context.Context, string, string) error { return nil }
func (m contractMetadata) SetItemOriginalLanguage(context.Context, string, *string) error {
	return nil
}
func (m contractMetadata) ItemImages(context.Context, string) ([]model.AdminItemImage, error) {
	return nil, nil
}
func (m contractMetadata) LocalImagePreview(context.Context, string, string) (model.StreamSource, error) {
	return model.StreamSource{}, model.ErrNotFound
}
