package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/admin"
	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// relinkHandlerMaint answers the relink of package library from a script: a live id ending in 9
// is no pair any more, every other pair lies in library 1; the relinks fail with errs in their
// order (nil = merged).
type relinkHandlerMaint struct {
	contractLibraryMaintenance
	errs []error
	n    int
}

func (m *relinkHandlerMaint) CheckRelinkPair(_ context.Context, _, liveID string) (int64, error) {
	if strings.HasSuffix(liveID, "9") {
		return 0, library.ErrNotTwins
	}
	return 1, nil
}

func (m *relinkHandlerMaint) Relink(_ context.Context, deletedID, liveID string) (library.RelinkResult, error) {
	m.n++
	if m.n <= len(m.errs) && m.errs[m.n-1] != nil {
		return library.RelinkResult{}, m.errs[m.n-1]
	}
	return library.RelinkResult{KeptID: deletedID, RemovedID: liveID, Merged: 2, Moved: 1}, nil
}

// relinkItems maps the outcome: the merged pairs, the skipped ones and stoppedBy when a running
// scan stopped the run after a merge; with nothing merged a scan answers 409 scan_running naming
// the library.
func TestRelinkItemsHandler(t *testing.T) {
	t.Parallel()
	maint := &relinkHandlerMaint{}
	e := newAuthEnvAt(t, &stepClock{now: time.Now()}, func(a *admin.Service) { a.WithLibraryMaintenance(maint) })
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)
	ref := func(deleted, live string) map[string]string {
		return map[string]string{"deletedId": "01JB8Z6Q4M7N9P2R3S5T7V9X" + deleted, "liveId": "01JB8Z6Q4M7N9P2R3S5T7V9X" + live}
	}
	post := func(pairs ...map[string]string) *httptest.ResponseRecorder {
		return e.call(t, http.MethodPost, "/api/v1/admin/maintenance/relink", map[string]any{"pairs": pairs}, owner)
	}

	maint.errs = []error{nil, model.ErrScanRunning}
	rec = post(ref("21", "31"), ref("22", "39"), ref("23", "33"), ref("24", "34"))
	wantStatus(t, rec, http.StatusOK)
	var out gen.RelinkOutcome
	decode(t, rec, &out)
	if len(out.Relinked) != 1 || out.Relinked[0] != (gen.RelinkedPair{KeptId: "01JB8Z6Q4M7N9P2R3S5T7V9X21", RemovedId: "01JB8Z6Q4M7N9P2R3S5T7V9X31", Merged: 2, Moved: 1}) ||
		out.Skipped != 1 || out.StoppedBy == nil || *out.StoppedBy != gen.RelinkOutcomeStoppedBy("scan_running") {
		t.Errorf("outcome = %+v", out)
	}

	maint.n, maint.errs = 0, nil
	rec = post(ref("21", "31"))
	wantStatus(t, rec, http.StatusOK)
	var raw map[string]json.RawMessage
	decode(t, rec, &raw)
	if _, ok := raw["stoppedBy"]; ok || string(raw["skipped"]) != "0" {
		t.Errorf("outcome without a stop = %s", rec.Body.String())
	}

	maint.n, maint.errs = 0, []error{model.ErrScanRunning}
	rec = post(ref("21", "31"), ref("22", "39"))
	wantStatus(t, rec, http.StatusConflict)
	var p gen.Problem
	decode(t, rec, &p)
	if p.Code != "scan_running" || p.Detail == nil || !strings.Contains(*p.Detail, `"Filme"`) {
		t.Errorf("problem = %+v, want scan_running naming the library", p)
	}
}
