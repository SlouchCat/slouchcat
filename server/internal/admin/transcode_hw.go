package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// The transcode backend chosen in the wizard or the administration (concept 22, F-10, O-19). It is
// read once at start: --hw and a non-empty SLOUCHCAT_HW win (source env), then the stored choice,
// then auto. A stored hardware backend that fails its probe at start takes the automatic choice
// instead (transcode.DetectOptions.FallbackAuto), and the dashboard says so (GPUInfo.FallbackFrom);
// --hw and the environment keep the old way (software). Restarting never loops: the stored choice
// stays as it is.

// SettingTranscodeHW holds the chosen backend (TranscodeChoices).
const SettingTranscodeHW = "transcode_hw"

// TranscodeChoices are the values of SettingTranscodeHW and AdminSettings.TranscodeHW.
var TranscodeChoices = []string{"auto", string(transcode.BackendNVENC), string(transcode.BackendQSV), string(transcode.BackendVAAPI),
	string(transcode.BackendSoftware)}

// StoredTranscodeHW reads the stored choice before the admin service exists (main, at start);
// "" when none or an unknown value is stored.
func StoredTranscodeHW(ctx context.Context, db *store.DB) (string, error) {
	v, err := appq.New(db.Read).GetSetting(ctx, SettingTranscodeHW)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("read setting %s: %w", SettingTranscodeHW, err)
	case !slices.Contains(TranscodeChoices, v):
		return "", nil
	}
	return v, nil
}

// transcodeChoice returns the choice with its source (env, stored, none) and whether it differs
// from what the running server uses: a choice of auto against a forced backend at start, or a
// backend other than the running one. With the environment winning nothing the setting says
// applies, so no restart is due.
func (s *Service) transcodeChoice(ctx context.Context) (choice, source string, restart bool, err error) {
	sys := s.ops.system
	if sys.HWExplicit {
		return sys.HWChoice, model.SourceEnv, false, nil
	}
	v, ok, err := s.setting(ctx, SettingTranscodeHW)
	if err != nil {
		return "", "", false, err
	}
	choice, source = "auto", model.SourceNone
	if ok && slices.Contains(TranscodeChoices, v) {
		choice, source = v, model.SourceStored
	}
	start := sys.HWChoice
	if start == "" {
		start = "auto"
	}
	running := string(sys.Caps.Backend)
	if running == "" {
		running = string(transcode.BackendSoftware)
	}
	return choice, source, choice != start && choice != running, nil
}

// transcodeRestartNeeded is the restart of finishSetup.
func (s *Service) transcodeRestartNeeded(ctx context.Context) (bool, error) {
	_, _, restart, err := s.transcodeChoice(ctx)
	return restart, err
}
