package api

import (
	"net/http"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// Concept 22 part A over HTTP: runSetup without a library starts the guided wizard with a named
// first profile in German (Germany); the owner's session says setupPending at the sign-in and
// on every read until finishSetup ran, which needs a library first.
func TestGuidedSetupOverHTTP(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	body := setupBody()
	delete(body, "library")
	body["language"], body["profileName"], body["accent"] = "de-DE", "Marco", "teal"
	rec := e.call(t, http.MethodPost, "/api/v1/setup", body, nil)
	wantStatus(t, rec, http.StatusCreated)
	owner := sessionCookie(t, rec)
	var started gen.SessionInfo
	decode(t, rec, &started)
	if started.SetupPending == nil || !*started.SetupPending || started.Profile == nil || started.Profile.Name != "Marco" ||
		started.Profile.Accent == nil || string(*started.Profile.Accent) != "teal" || started.Profile.Language != gen.DeDE {
		t.Fatalf("session after the guided setup = %s", rec.Body.String())
	}

	rec = e.call(t, http.MethodGet, "/api/v1/admin/setup", nil, owner)
	wantStatus(t, rec, http.StatusOK)
	var p gen.SetupProgress
	decode(t, rec, &p)
	if p.Finished || len(p.Steps) != 9 || p.Steps[0].State != gen.SetupStepDone || p.Steps[2].Step != gen.SetupStepRegion || p.Steps[2].State != gen.SetupStepOpen {
		t.Fatalf("progress = %s", rec.Body.String())
	}
	rec = e.call(t, http.MethodPut, "/api/v1/admin/setup", map[string]any{"step": "region", "state": "skipped", "trickplayDefault": true}, owner)
	wantStatus(t, rec, http.StatusOK)
	decode(t, rec, &p)
	if p.Steps[2].State != gen.SetupStepSkipped || !p.TrickplayDefault {
		t.Errorf("progress after the update = %s", rec.Body.String())
	}
	rec = e.call(t, http.MethodPut, "/api/v1/admin/setup", map[string]any{"step": "region"}, owner)
	wantStatus(t, rec, http.StatusUnprocessableEntity)

	rec = e.call(t, http.MethodPost, "/api/v1/admin/setup/finish", nil, owner)
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "library_required" {
		t.Fatalf("finish without a library: %d %s", rec.Code, rec.Body.String())
	}
	rec = e.call(t, http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "marco@example.ch", "password": authTestPassword}, nil)
	wantStatus(t, rec, http.StatusOK)
	var again gen.SessionInfo
	decode(t, rec, &again)
	if again.SetupPending == nil || !*again.SetupPending {
		t.Errorf("a new sign-in before the finish: setupPending %v, want true (F-11)", again.SetupPending)
	}

	rec = e.call(t, http.MethodPost, "/api/v1/admin/libraries", map[string]any{"name": "Filme", "kind": "video",
		"roots": []any{movieRoot("/media/Filme")}}, owner)
	wantStatus(t, rec, http.StatusCreated)
	rec = e.call(t, http.MethodPost, "/api/v1/admin/setup/finish", nil, owner)
	wantStatus(t, rec, http.StatusOK)
	var done gen.SetupFinished
	decode(t, rec, &done)
	if done.Restart != gen.SetupFinishedRestartNone {
		t.Errorf("finish = %s, want no restart", rec.Body.String())
	}
	rec = e.call(t, http.MethodGet, "/api/v1/auth/session", nil, owner)
	var after gen.SessionInfo
	decode(t, rec, &after)
	if after.SetupPending == nil || *after.SetupPending {
		t.Errorf("session after the finish: setupPending %v, want false", after.SetupPending)
	}
}

// The classic setup with a library records no wizard: the session never says setupPending, so
// the e2e setup and older callers are not led into the wizard (concept 22, 3.4).
func TestSetupWithLibraryIsFinished(t *testing.T) {
	t.Parallel()
	e := newAuthEnv(t)
	rec := e.call(t, http.MethodPost, "/api/v1/setup", setupBody(), nil)
	wantStatus(t, rec, http.StatusCreated)
	var info gen.SessionInfo
	decode(t, rec, &info)
	if info.SetupPending == nil || *info.SetupPending {
		t.Errorf("setupPending after a setup with a library = %v, want false", info.SetupPending)
	}
}
