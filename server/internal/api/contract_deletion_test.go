package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Concept 20 stage 5 (O-13): a delete refused because a profile plays a title answers 409
// playing and names the profiles; a typed confirmation that does not repeat the name answers 422
// confirm_mismatch with a text that fits a library as well as a folder.
func TestDeletionProblems(t *testing.T) {
	t.Parallel()
	p := problems{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rec := httptest.NewRecorder()
	p.mapError(rec, httptest.NewRequest(http.MethodPost, BasePath+"/admin/libraries/1/deletion", nil),
		&model.PlayingError{Profiles: []string{"Anna", "Ben"}})
	var body struct {
		Code            string   `json:"code"`
		Status          int      `json:"status"`
		PlayingProfiles []string `json:"playingProfiles"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusConflict || body.Code != "playing" || body.Status != http.StatusConflict ||
		!slices.Equal(body.PlayingProfiles, []string{"Anna", "Ben"}) {
		t.Errorf("playing: status %d, body %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	p.mapError(rec, httptest.NewRequest(http.MethodPost, BasePath+"/admin/libraries/1/deletion", nil), model.ErrConfirmMismatch)
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "confirm_mismatch" {
		t.Errorf("mismatch: status %d, body %s", rec.Code, rec.Body.String())
	}
}

// Concept 20 P-9 through the contract on the fixture: the preview of a film names its file with
// the folder of its root, «Nur aus Slouchcat entfernen» takes it out (the administration and a
// second preview answer 404), «Entfernte Titel» lists it and «Wieder aufnehmen» brings it back.
func TestContractItemDeletion(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	path := BasePath + "/admin/items/" + fixtureFilm6 + "/deletion"
	rec := send(t, pf.h, http.MethodGet, path, "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var preview gen.ItemDeletionPreview
	decode(t, rec, &preview)
	if preview.FileCount != len(preview.Files) || preview.FileCount == 0 || preview.Files[0].RootPath == "" ||
		preview.ConfirmTitle == "" || preview.Kind != gen.ItemKindMovie || len(preview.PlayingProfiles) != 0 {
		t.Fatalf("preview = %+v", preview)
	}
	// The fixture's files lie in a temporary folder of the test: deletable (ADR 0032).
	if !preview.FilesDeletable || preview.FilesReason != nil || !preview.Files[0].Deletable {
		t.Errorf("files verdict = %v %v, file %+v", preview.FilesDeletable, preview.FilesReason, preview.Files[0])
	}
	stamp := fmt.Sprintf(`"expectedFiles":{"count":%d,"hash":%q}`, preview.ExpectedFiles.Count, preview.ExpectedFiles.Hash)
	confirm := `{"mode":"library",` + stamp + `,"confirmTitle":` + strconv.Quote(strings.ToUpper(preview.ConfirmTitle)) + `}`
	rec = send(t, pf.h, http.MethodPost, path, "p18", confirm, nil)
	wantStatus(t, rec, http.StatusOK)
	var result gen.ItemDeletionResult
	decode(t, rec, &result)
	if result.RemovedItems == 0 || result.DeletedFiles != 0 {
		t.Errorf("result = %+v", result)
	}
	wantStatus(t, send(t, pf.h, http.MethodGet, path, "p18", "", nil), http.StatusNotFound)
	wantStatus(t, send(t, pf.h, http.MethodGet, BasePath+"/admin/items/"+fixtureFilm6, "p18", "", nil), http.StatusNotFound)
	rec = send(t, pf.h, http.MethodGet, BasePath+"/admin/maintenance/removed", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var page gen.RemovedItemPage
	decode(t, rec, &page)
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Id != fixtureFilm6 || page.Items[0].LibraryName == "" {
		t.Fatalf("removed = %+v", page)
	}
	rec = send(t, pf.h, http.MethodPost, BasePath+"/admin/maintenance/removed/"+fixtureFilm6+"/restore", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	wantStatus(t, send(t, pf.h, http.MethodGet, BasePath+"/admin/items/"+fixtureFilm6, "p18", "", nil), http.StatusOK)
	// A stamp that is not the preview's: 409 changed, nothing goes.
	rec = send(t, pf.h, http.MethodPost, path, "p18", `{"mode":"files","expectedFiles":{"count":1,"hash":"00000000000000000000000000000000"}}`, nil)
	wantStatus(t, rec, http.StatusConflict)
	if code := problemCode(t, rec); code != "changed" {
		t.Errorf("a foreign stamp = %s, want changed", code)
	}
	// «Auch die Dateien löschen»: the files go, and the title for good (O-14); it is not listed
	// among the removed titles.
	rec = send(t, pf.h, http.MethodPost, path, "p18", confirm[:len(`{"mode":"`)]+`files"`+confirm[len(`{"mode":"library"`):], nil)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &result)
	if result.DeletedFiles != preview.FileCount || result.TitleDeleted == nil || !*result.TitleDeleted || len(result.Leftover) != 0 {
		t.Errorf("files result = %+v", result)
	}
	wantStatus(t, send(t, pf.h, http.MethodGet, BasePath+"/admin/items/"+fixtureFilm6, "p18", "", nil), http.StatusNotFound)
	rec = send(t, pf.h, http.MethodGet, BasePath+"/admin/maintenance/removed", "p18", "", nil)
	decode(t, rec, &page)
	if page.Total != 0 {
		t.Errorf("removed after the delete with files = %+v", page)
	}
	// An unknown mode is refused (validation_failed).
	wantStatus(t, send(t, pf.h, http.MethodPost, path, "p18", `{"mode":"trash",`+stamp+`}`, nil), http.StatusUnprocessableEntity)
}
