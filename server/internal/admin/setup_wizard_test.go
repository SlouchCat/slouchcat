package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// guidedSetup runs the public half of the guided wizard (no library) and returns the owner's
// account id.
func guidedSetup(t *testing.T, f *fixture) int64 {
	t.Helper()
	req := setupRequest()
	req.Library = nil
	session, _, err := f.svc.RunSetup(model.WithClientIP(context.Background(), "192.0.2.40"), req, "test")
	if err != nil {
		t.Fatal(err)
	}
	return session.Account.ID
}

func stepStates(p model.SetupProgress) map[string]string {
	out := map[string]string{}
	for _, s := range p.Steps {
		out[s.Step] = s.State
	}
	return out
}

// Concept 22, 3.4: runSetup without a library records an open wizard with the public steps done;
// with a library it records none, and no wizard counts as finished (no data migration, the e2e
// setup is not led into the wizard).
func TestRunSetupRecordsTheGuidedWizard(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFixture(t)
	guidedSetup(t, f)
	if len(f.libs.created) != 0 {
		t.Errorf("guided setup created libraries: %+v", f.libs.created)
	}
	pending, err := f.svc.SetupPending(ctx)
	if err != nil || !pending {
		t.Fatalf("pending after the guided setup = %v, %v; want true", pending, err)
	}
	p, err := f.svc.SetupProgress(ctx)
	if err != nil {
		t.Fatal(err)
	}
	states := stepStates(p)
	if p.Finished || len(p.Steps) != len(model.SetupSteps) || states["language"] != model.SetupDone ||
		states["account"] != model.SetupDone || states["region"] != model.SetupOpen || states["summary"] != model.SetupOpen {
		t.Errorf("progress = %+v", p)
	}

	old := newFixture(t)
	if _, _, err := old.svc.RunSetup(model.WithClientIP(ctx, "192.0.2.41"), setupRequest(), "test"); err != nil {
		t.Fatal(err)
	}
	if pending, err := old.svc.SetupPending(ctx); err != nil || pending {
		t.Errorf("pending after a setup with a library = %v, %v; want false", pending, err)
	}
	p, err = old.svc.SetupProgress(ctx)
	if err != nil || !p.Finished || stepStates(p)["region"] != model.SetupDone {
		t.Errorf("progress without a recorded wizard = %+v, %v; want every step done", p, err)
	}
	if _, err := old.svc.UpdateSetupProgress(ctx, model.SetupProgressUpdate{TrickplayDefault: new(true)}); !errors.Is(err, model.ErrSetupDone) {
		t.Errorf("update without a recorded wizard = %v, want ErrSetupDone (never opened again)", err)
	}
}

// Step 2 names the first profile and its colour (concept 22, 3.3).
func TestRunSetupNamesTheFirstProfile(t *testing.T) {
	t.Parallel()
	ctx := model.WithClientIP(context.Background(), "192.0.2.42")
	f := newFixture(t)
	req := setupRequest()
	req.Library, req.ProfileName, req.Accent, req.Language = nil, " Marco ", "teal", "de-DE"
	session, _, err := f.svc.RunSetup(ctx, req, "test")
	if err != nil {
		t.Fatal(err)
	}
	if session.Profile.Name != "Marco" || session.Profile.Accent != "teal" || session.Profile.Language != "de-DE" {
		t.Errorf("first profile = %+v", session.Profile)
	}

	bad := newFixture(t)
	req.Accent = "purple"
	var verr *model.ValidationError
	if _, _, err := bad.svc.RunSetup(ctx, req, "test"); !errors.As(err, &verr) || verr.Fields[0].Field != "accent" {
		t.Errorf("unknown accent = %v, want a validation error on accent", err)
	}
}

func TestUpdateSetupProgress(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFixture(t)
	guidedSetup(t, f)
	p, err := f.svc.UpdateSetupProgress(ctx, model.SetupProgressUpdate{Step: new("services"), State: new(model.SetupSkipped),
		TrickplayDefault: new(true)})
	if err != nil {
		t.Fatal(err)
	}
	if stepStates(p)["services"] != model.SetupSkipped || !p.TrickplayDefault {
		t.Errorf("progress = %+v", p)
	}
	for name, upd := range map[string]model.SetupProgressUpdate{
		"step without state": {Step: new("region")},
		"unknown step":       {Step: new("welcome"), State: new(model.SetupDone)},
		"unknown state":      {Step: new("region"), State: new("later")},
		"account reopened":   {Step: new("account"), State: new(model.SetupOpen)},
	} {
		var verr *model.ValidationError
		if _, err := f.svc.UpdateSetupProgress(ctx, upd); !errors.As(err, &verr) {
			t.Errorf("%s = %v, want a validation error", name, err)
		}
	}
	if p, _ := f.svc.SetupProgress(ctx); stepStates(p)["services"] != model.SetupSkipped || !p.TrickplayDefault {
		t.Errorf("the refused updates changed the wizard: %+v", p)
	}
}

// finishSetup needs a library (422 library_required), closes the wizard for good and restarts
// only when the transcode choice differs from what runs.
func TestFinishSetup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFixture(t)
	owner := guidedSetup(t, f)
	if _, err := f.svc.FinishSetup(ctx, owner); !errors.Is(err, model.ErrLibraryRequired) {
		t.Fatalf("finish without a library = %v, want ErrLibraryRequired", err)
	}
	if pending, _ := f.svc.SetupPending(ctx); !pending {
		t.Error("a refused finish closed the wizard")
	}
	if _, err := f.svc.CreateLibrary(ctx, *setupRequest().Library); err != nil {
		t.Fatal(err)
	}
	done, err := f.svc.FinishSetup(ctx, owner)
	if err != nil || done.Restart != model.SetupRestartNone {
		t.Fatalf("finish = %+v, %v; want no restart", done, err)
	}
	p, err := f.svc.SetupProgress(ctx)
	if err != nil || !p.Finished || p.FinishedAt.IsZero() || stepStates(p)["summary"] != model.SetupDone {
		t.Errorf("progress after finish = %+v, %v", p, err)
	}
	if pending, _ := f.svc.SetupPending(ctx); pending {
		t.Error("still pending after finish")
	}
	if _, err := f.svc.UpdateSetupProgress(ctx, model.SetupProgressUpdate{TrickplayDefault: new(false)}); !errors.Is(err, model.ErrSetupDone) {
		t.Errorf("update after finish = %v, want ErrSetupDone", err)
	}
	if again, err := f.svc.FinishSetup(ctx, owner); err != nil || again.Restart != model.SetupRestartNone {
		t.Errorf("second finish = %+v, %v", again, err)
	}
}

func TestFinishSetupRestartsForTheTranscodeChoice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFixture(t)
	f.svc.WithSystem(SystemSource{HWChoice: "auto", Caps: transcode.Capabilities{Backend: transcode.BackendSoftware}})
	owner := guidedSetup(t, f)
	if _, err := f.svc.CreateLibrary(ctx, *setupRequest().Library); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{TranscodeHW: new("qsv")}); err != nil {
		t.Fatal(err)
	}
	// Without the restart wired (or while a backup runs) the choice waits for the next restart.
	done, err := f.svc.FinishSetup(ctx, owner)
	if err != nil || done.Restart != model.SetupRestartBusy {
		t.Fatalf("finish without a restart function = %+v, %v; want busy", done, err)
	}

	g := newFixture(t)
	g.svc.WithSystem(SystemSource{HWChoice: "auto", Caps: transcode.Capabilities{Backend: transcode.BackendSoftware}})
	restarted := make(chan struct{}, 1)
	g.svc.WithRestart(func() { restarted <- struct{}{} })
	owner = guidedSetup(t, g)
	if _, err := g.svc.CreateLibrary(ctx, *setupRequest().Library); err != nil {
		t.Fatal(err)
	}
	if _, err := g.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{TranscodeHW: new("qsv")}); err != nil {
		t.Fatal(err)
	}
	done, err = g.svc.FinishSetup(ctx, owner)
	if err != nil || done.Restart != model.SetupRestartScheduled || done.RestartAt.IsZero() {
		t.Fatalf("finish with a new choice = %+v, %v; want a scheduled restart", done, err)
	}
	<-restarted
}

// Concept 22, F-10: --hw and SLOUCHCAT_HW win over the stored choice, which wins over auto; a
// restart is due only when the choice differs from what runs.
func TestTranscodeChoicePrecedence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cases := []struct {
		name        string
		src         SystemSource
		stored      string
		choice, via string
		restart     bool
	}{
		{"nothing set", SystemSource{HWChoice: "auto", Caps: transcode.Capabilities{Backend: transcode.BackendQSV}}, "", "auto", model.SourceNone, false},
		{"stored like the running backend", SystemSource{HWChoice: "auto", Caps: transcode.Capabilities{Backend: transcode.BackendQSV}}, "qsv", "qsv", model.SourceStored, false},
		{"stored other backend", SystemSource{HWChoice: "auto", Caps: transcode.Capabilities{Backend: transcode.BackendQSV}}, "software", "software", model.SourceStored, true},
		{"auto after a forced start", SystemSource{HWChoice: "qsv", Caps: transcode.Capabilities{Backend: transcode.BackendQSV}}, "auto", "auto", model.SourceStored, true},
		{"stored backend that fell back", SystemSource{HWChoice: "nvenc", Caps: transcode.Capabilities{Backend: transcode.BackendQSV, FallbackFrom: transcode.BackendNVENC}}, "nvenc", "nvenc", model.SourceStored, false},
		{"environment wins", SystemSource{HWChoice: "vaapi", HWExplicit: true, Caps: transcode.Capabilities{Backend: transcode.BackendSoftware}}, "qsv", "vaapi", model.SourceEnv, false},
	}
	for _, c := range cases {
		f := newFixture(t)
		f.svc.WithSystem(c.src)
		if c.stored != "" {
			if err := f.svc.write.UpsertSetting(ctx, upsert(SettingTranscodeHW, c.stored)); err != nil {
				t.Fatal(err)
			}
		}
		st, err := f.svc.Settings(ctx)
		if err != nil || st.TranscodeHW != c.choice || st.TranscodeHWSource != c.via || st.TranscodeHWRestart != c.restart {
			t.Errorf("%s: %s/%s restart %v, %v; want %s/%s restart %v", c.name, st.TranscodeHW, st.TranscodeHWSource,
				st.TranscodeHWRestart, err, c.choice, c.via, c.restart)
		}
	}

	f := newFixture(t)
	f.svc.WithSystem(SystemSource{HWChoice: "vaapi", HWExplicit: true})
	var env *model.EnvWinsError
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{TranscodeHW: new("qsv")}); !errors.As(err, &env) || env.Field != "transcodeHw" {
		t.Errorf("choice while SLOUCHCAT_HW is set = %v, want env_wins on transcodeHw", err)
	}
	var verr *model.ValidationError
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{TranscodeHW: new("cuda")}); !errors.As(err, &verr) {
		t.Errorf("unknown choice = %v, want a validation error", err)
	}
}

// SystemInfo carries the probes of every hardware backend, the stored choice that fell back and
// the server's time zone (concept 22, 3.5).
func TestSystemInfoCandidatesAndZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.svc.WithSystem(SystemSource{TimeZone: "Europe/Zurich", HWChoice: "nvenc", DevDir: t.TempDir(), Caps: transcode.Capabilities{
		Backend: transcode.BackendQSV, FallbackFrom: transcode.BackendNVENC, Candidates: []transcode.Candidate{
			{Backend: transcode.BackendNVENC, Reason: transcode.CandidateNoDevice}, {Backend: transcode.BackendQSV, OK: true},
			{Backend: transcode.BackendVAAPI, Reason: transcode.CandidateTestFailed},
		},
	}})
	info, err := f.svc.SystemInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	g := info.GPU
	if info.TimeZone != "Europe/Zurich" || g.FallbackFrom != "nvenc" || len(g.Candidates) != 3 ||
		g.Candidates[0] != (model.TranscodeCandidate{Backend: "nvenc", Reason: "no_device"}) || !g.Candidates[1].OK {
		t.Errorf("system info = %+v", info)
	}
	if info, _ := newFixture(t).svc.SystemInfo(context.Background()); info.TimeZone != "UTC" {
		t.Errorf("time zone without one = %q, want UTC", info.TimeZone)
	}
}
