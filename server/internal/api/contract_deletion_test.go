package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

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
