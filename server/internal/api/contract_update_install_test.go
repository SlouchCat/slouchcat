package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/updater"
)

// Concept 15 P-8 (contract 1.16.0, ADR 0024) over HTTP: installUpdate against a fake companion
// (202 with its state, 409 update_running, 422 with its code, 503 when it refuses the token),
// getUpdateInstallStatus from state.json, and getUpdateState with updaterAvailable and install.
func TestUpdateInstallOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h
	dir := t.TempDir()
	const token = "updater-FAKE-token-0123456789abcdef"
	if err := os.WriteFile(filepath.Join(dir, updater.TokenFile), []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	installStatus, installBody := http.StatusAccepted, `{"phase":"checking","from":"0.0.0-test","to":"0.0.1","startedAt":"2026-11-07T10:00:00Z"}`
	companion := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET " + updater.PathStatus:
			_, _ = w.Write([]byte(`{"phase":"idle"}`))
		case "POST " + updater.PathInstall:
			w.WriteHeader(installStatus)
			_, _ = w.Write([]byte(installBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(companion.Close)
	pf.admin.WithUpdater(companion.URL, dir, nil)

	rec := send(t, h, http.MethodGet, BasePath+"/admin/update/status", "p18", "", nil)
	var st gen.UpdateInstallStatus
	decode(t, rec, &st)
	if rec.Code != http.StatusOK || st.Phase != gen.UpdateInstallStatusPhaseIdle {
		t.Errorf("status without state.json = %d %s", rec.Code, rec.Body.String())
	}

	rec = send(t, h, http.MethodPost, BasePath+"/admin/update/install", "p18", "", nil)
	decode(t, rec, &st)
	if rec.Code != http.StatusAccepted || st.Phase != gen.UpdateInstallStatusPhaseChecking || deref(st.To) != "0.0.1" ||
		st.StartedAt == nil || !st.StartedAt.Equal(time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("install = %d %s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct {
		status    int
		body      string
		want      int
		code      string
		inProblem string
	}{
		{http.StatusConflict, `{"phase":"pulling","code":"busy"}`, http.StatusConflict, "update_running", ""},
		{http.StatusUnprocessableEntity, `{"phase":"failed","code":"not_newer","message":"0.0.1 is not newer than 0.0.1"}`,
			http.StatusUnprocessableEntity, "not_newer", "0.0.1 is not newer than 0.0.1"},
		{http.StatusUnprocessableEntity, `{"phase":"failed","code":"Bad Code"}`, http.StatusUnprocessableEntity, "update_refused", ""},
		{http.StatusInternalServerError, `oops`, http.StatusServiceUnavailable, "updater_unavailable", ""},
	} {
		mu.Lock()
		installStatus, installBody = tc.status, tc.body
		mu.Unlock()
		rec = send(t, h, http.MethodPost, BasePath+"/admin/update/install", "p18", "", nil)
		if rec.Code != tc.want || problemCode(t, rec) != tc.code || !strings.Contains(rec.Body.String(), tc.inProblem) {
			t.Errorf("companion %d %s: got %d %s, want %d %s", tc.status, tc.body, rec.Code, rec.Body.String(), tc.want, tc.code)
		}
	}

	state := updater.State{Phase: updater.PhaseRolledBack, From: "0.0.0-test", To: "0.0.1", Code: updater.CodeUnhealthy,
		StartedAt: time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC), FinishedAt: time.Date(2026, 11, 7, 10, 4, 0, 0, time.UTC)}
	if err := updater.WriteJSON(filepath.Join(dir, updater.StateFile), state, 0o644); err != nil {
		t.Fatal(err)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/admin/update", "p18", "", nil)
	var u gen.UpdateState
	decode(t, rec, &u)
	if rec.Code != http.StatusOK || u.UpdaterAvailable == nil || !*u.UpdaterAvailable || u.Install == nil ||
		u.Install.Phase != gen.UpdateInstallStatusPhaseRolledBack || deref(u.Install.Code) != "unhealthy" || u.Install.Migrated {
		t.Errorf("update state = %d %s", rec.Code, rec.Body.String())
	}
	rec = send(t, h, http.MethodGet, BasePath+"/admin/update/status", "p18", "", nil)
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil || rec.Code != http.StatusOK ||
		raw["phase"] != "rolled_back" || raw["finishedAt"] != "2026-11-07T10:04:00Z" || raw["migrated"] != false {
		t.Errorf("status from state.json = %d %s", rec.Code, rec.Body.String())
	}

	// The companion refuses a token it does not know: no companion for the server.
	if err := os.WriteFile(filepath.Join(dir, updater.TokenFile), []byte("updater-FAKE-other-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec = send(t, h, http.MethodPost, BasePath+"/admin/update/install", "p18", "", nil)
	if rec.Code != http.StatusServiceUnavailable || problemCode(t, rec) != "updater_unavailable" || strings.Contains(rec.Body.String(), "FAKE") {
		t.Errorf("install with a refused token = %d %s", rec.Code, rec.Body.String())
	}
	// A guest administers nothing.
	if rec = send(t, h, http.MethodPost, BasePath+"/admin/update/install", "pg", "", nil); rec.Code != http.StatusForbidden {
		t.Errorf("guest install = %d", rec.Code)
	}
}
