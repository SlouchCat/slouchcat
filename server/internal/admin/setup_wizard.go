package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The guided wizard of concept 22 part A. runSetup without a library records an open wizard in the
// setting setup_wizard; the owner goes on with steps 3 to 9 behind the login, through the
// endpoints of the administration, and finishSetup closes it. A missing setting counts as
// finished: every installation set up before contract 1.35.0, and runSetup with a library (the
// e2e setup, scripts), never sees the wizard, so no data migration is needed (3.4).

// SettingSetupWizard holds the wizard as JSON (storedWizard).
const SettingSetupWizard = "setup_wizard"

// storedWizard is the JSON of SettingSetupWizard.
type storedWizard struct {
	Steps            map[string]string `json:"steps"`
	TrickplayDefault bool              `json:"trickplayDefault,omitempty"`
	FinishedAt       int64             `json:"finishedAt,omitempty"` // Unix ms; 0 while open
}

// publicSteps are done by runSetup itself; they never go back to open.
var publicSteps = []string{"language", "account"}

// readWizard returns the stored wizard; ok is false when none is stored (finished by the rule).
func (s *Service) readWizard(ctx context.Context) (storedWizard, bool, error) {
	raw, ok, err := s.setting(ctx, SettingSetupWizard)
	if err != nil || !ok {
		return storedWizard{}, false, err
	}
	var w storedWizard
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		// A broken row must not trap the owner in the wizard: it counts as finished.
		s.log.WarnContext(ctx, "unreadable setup wizard; treated as finished", "err", err)
		return storedWizard{}, false, nil
	}
	return w, true, nil
}

func (s *Service) storeWizard(ctx context.Context, w storedWizard) error {
	raw, err := json.Marshal(w)
	if err != nil {
		return fmt.Errorf("encode setup wizard: %w", err)
	}
	if err := s.write.UpsertSetting(ctx, appq.UpsertSettingParams{Key: SettingSetupWizard, Value: string(raw), Now: s.now().UnixMilli()}); err != nil {
		return fmt.Errorf("store setup wizard: %w", err)
	}
	return nil
}

// openWizard is the wizard runSetup records: the public steps done, the rest open.
func openWizard() storedWizard {
	w := storedWizard{Steps: map[string]string{}}
	for _, step := range model.SetupSteps {
		w.Steps[step] = model.SetupOpen
	}
	for _, step := range publicSteps {
		w.Steps[step] = model.SetupDone
	}
	return w
}

// progress maps the stored wizard; without one every step is done and the wizard finished.
func (s *Service) progress(w storedWizard, stored bool) model.SetupProgress {
	out := model.SetupProgress{Steps: make([]model.SetupStepState, 0, len(model.SetupSteps)),
		TrickplayDefault: w.TrickplayDefault, Finished: !stored || w.FinishedAt != 0}
	for _, step := range model.SetupSteps {
		state := model.SetupDone
		if stored {
			state = w.Steps[step]
			if state == "" {
				state = model.SetupOpen
			}
		}
		out.Steps = append(out.Steps, model.SetupStepState{Step: step, State: state})
	}
	if w.FinishedAt != 0 {
		out.FinishedAt = time.UnixMilli(w.FinishedAt)
	}
	return out
}

// SetupPending reports whether the guided wizard is open (SessionInfo.setupPending, F-11).
func (s *Service) SetupPending(ctx context.Context) (bool, error) {
	w, stored, err := s.readWizard(ctx)
	if err != nil {
		return false, err
	}
	return stored && w.FinishedAt == 0, nil
}

// SetupProgress returns the wizard (getSetupProgress).
func (s *Service) SetupProgress(ctx context.Context) (model.SetupProgress, error) {
	w, stored, err := s.readWizard(ctx)
	if err != nil {
		return model.SetupProgress{}, err
	}
	return s.progress(w, stored), nil
}

// UpdateSetupProgress records the state of a step and/or the trickplay default
// (updateSetupProgress). A finished wizard is never opened again: model.ErrSetupDone.
// *model.ValidationError: step without state or the other way round, an unknown step or state, a
// public step set back.
func (s *Service) UpdateSetupProgress(ctx context.Context, upd model.SetupProgressUpdate) (model.SetupProgress, error) {
	var verr *model.ValidationError
	if (upd.Step == nil) != (upd.State == nil) {
		verr = verr.Add("step", "required")
	}
	if upd.Step != nil && !slices.Contains(model.SetupSteps, *upd.Step) {
		verr = verr.Add("step", "invalid_value")
	}
	if upd.State != nil && !slices.Contains([]string{model.SetupOpen, model.SetupDone, model.SetupSkipped}, *upd.State) {
		verr = verr.Add("state", "invalid_value")
	}
	if upd.Step != nil && upd.State != nil && slices.Contains(publicSteps, *upd.Step) && *upd.State != model.SetupDone {
		verr = verr.Add("state", "invalid_value")
	}
	if verr != nil {
		return model.SetupProgress{}, verr
	}
	s.setupMu.Lock()
	defer s.setupMu.Unlock()
	w, stored, err := s.readWizard(ctx)
	if err != nil {
		return model.SetupProgress{}, err
	}
	if !stored || w.FinishedAt != 0 {
		return model.SetupProgress{}, model.ErrSetupDone
	}
	if w.Steps == nil {
		w.Steps = map[string]string{}
	}
	if upd.Step != nil {
		w.Steps[*upd.Step] = *upd.State
	}
	if upd.TrickplayDefault != nil {
		w.TrickplayDefault = *upd.TrickplayDefault
	}
	if err := s.storeWizard(ctx, w); err != nil {
		return model.SetupProgress{}, err
	}
	return s.progress(w, true), nil
}

// FinishSetup closes the wizard (finishSetup): it needs a library (model.ErrLibraryRequired),
// marks the summary done and the wizard finished, and restarts the server through RestartServer
// when the stored transcode choice differs from what runs (F-10, O-18). A restart that a backup,
// restore, update or another restart keeps off answers SetupRestartBusy; the choice then applies
// after the next restart. Finishing a finished wizard changes nothing.
func (s *Service) FinishSetup(ctx context.Context, accountID int64) (model.SetupFinished, error) {
	s.setupMu.Lock()
	defer s.setupMu.Unlock()
	w, stored, err := s.readWizard(ctx)
	if err != nil {
		return model.SetupFinished{}, err
	}
	if !stored || w.FinishedAt != 0 {
		return model.SetupFinished{Restart: model.SetupRestartNone}, nil
	}
	libs, err := s.libraries.ListRoots(ctx)
	if err != nil {
		return model.SetupFinished{}, fmt.Errorf("list libraries: %w", err)
	}
	if len(libs) == 0 {
		return model.SetupFinished{}, model.ErrLibraryRequired
	}
	restart, err := s.transcodeRestartNeeded(ctx)
	if err != nil {
		return model.SetupFinished{}, err
	}
	// The restart first: a wizard is finished only together with its restart (scheduled, or left to
	// the administration while something keeps it off). Any other failure leaves the wizard open,
	// so «Einrichtung abschliessen» can be pressed again and the choice is not lost.
	done := model.SetupFinished{Restart: model.SetupRestartNone}
	if restart {
		accepted, err := s.RestartServer(ctx, accountID)
		switch {
		case err == nil:
			done = model.SetupFinished{Restart: model.SetupRestartScheduled, RestartAt: accepted.RestartAt}
		case errors.Is(err, model.ErrRestartPending), errors.Is(err, model.ErrBackupBusy), errors.Is(err, model.ErrUpdateRunning),
			errors.Is(err, ErrNotConfigured):
			s.log.InfoContext(ctx, "setup wizard: restart for the transcode choice left to the administration", "err", err)
			done = model.SetupFinished{Restart: model.SetupRestartBusy}
		default:
			return model.SetupFinished{}, err
		}
	}
	if w.Steps == nil {
		w.Steps = map[string]string{}
	}
	w.Steps["summary"] = model.SetupDone
	// RestartServer shuts down only after restartDelay: the wizard is stored well before.
	w.FinishedAt = s.now().UnixMilli()
	if err := s.storeWizard(ctx, w); err != nil {
		return model.SetupFinished{}, err
	}
	s.log.InfoContext(ctx, "setup wizard finished", "account_id", accountID, "restart", done.Restart)
	return done, nil
}
