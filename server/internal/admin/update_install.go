package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/updater"
)

// Installing an update through the updater companion (concept 15 9.3, ADR 0024). The server never
// sees the Docker socket: it asks the companion over the internal Docker network to install the
// newest release (InstallUpdate), and the companion picks the version itself. The progress lives in
// state.json in the shared folder, which only the companion writes; the server reads it directly
// (InstallStatus), so the card shows it while the companion or the server restarts. Each request
// carries the token from the shared folder as a bearer token, and the token never reaches a log
// (CLAUDE.md rule 10). An install that ended rolled back or failed is reported once as the event
// errors (reportInstall); which one was reported is the setting update_install_reported, not a
// mark in state.json.

// SettingUpdateInstallReported is the finish time (RFC 3339) of the last failed or rolled back
// install that was sent as the event errors.
const SettingUpdateInstallReported = "update_install_reported"

const (
	// updaterProbeTimeout bounds GET /status for UpdateState.UpdaterAvailable; updaterProbeTTL is
	// how long its answer counts, so the admin pages stay fast while the companion is down.
	updaterProbeTimeout = 2 * time.Second
	updaterProbeTTL     = 10 * time.Second
	// updaterInstallTimeout bounds POST /install; the companion answers once it took the order.
	updaterInstallTimeout = 15 * time.Second
	// installWatchLimit is how long WatchUpdateInstall looks at a running install after the start.
	installWatchLimit = 15 * time.Minute
	maxTokenBytes     = 4 << 10
)

// refusalCodes are the reasons the companion gives before it changed anything; the caller of
// installUpdate saw them as 422, so they raise no event.
var refusalCodes = map[updater.Code]bool{
	updater.CodeBusy: true, updater.CodeNotNewer: true, updater.CodeManifestUnreachable: true,
	updater.CodeManifestInvalid: true, updater.CodeNoDigest: true, updater.CodeNoComposeLabels: true,
	updater.CodeOutsideDeployDir: true,
}

// problemCode is the form of a problem code (contract Problem.code).
var problemCode = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// updaterLink is the updater part of the service.
type updaterLink struct {
	url        string // "" = no companion
	dir        string // shared folder (updater.ServerDir)
	client     *http.Client
	failed     func(context.Context, model.UpdateInstall) error // event errors; nil = none
	watchEvery time.Duration

	probeMu   sync.Mutex // one probe at a time; guards probedAt and available
	probedAt  time.Time
	available bool

	stateMu  sync.Mutex
	stateBad bool // the last read of state.json failed; logged once until it reads again

	reportMu     sync.Mutex // guards reported and reportLoaded
	reported     time.Time
	reportLoaded bool
}

// WithUpdater sets the updater companion (config.UpdaterURL, "" = none), its shared folder
// (config.UpdaterDir) and the function that sends a failed or rolled back install as the event
// errors (notify.UpdateInstallFailed; nil = none), and returns the receiver.
func (s *Service) WithUpdater(url, dir string, failed func(context.Context, model.UpdateInstall) error) *Service {
	u := &s.ops.updater
	u.url, u.dir, u.failed = strings.TrimRight(url, "/"), dir, failed
	u.client = &http.Client{
		Timeout: updaterInstallTimeout,
		// A redirect would carry the token elsewhere; the companion never sends one.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	u.watchEvery = 5 * time.Second
	return s
}

// InstallUpdate asks the companion to install the newest release and returns the state it
// reported: model.ErrUpdaterUnavailable without a companion, when it does not answer or refuses
// the token; model.ErrUpdateRunning while an install runs; *model.UpdateRefusedError when it
// refused before it changed anything.
func (s *Service) InstallUpdate(ctx context.Context) (model.UpdateInstall, error) {
	u := &s.ops.updater
	if u.url == "" {
		return model.UpdateInstall{}, model.ErrUpdaterUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, updaterInstallTimeout)
	defer cancel()
	resp, err := s.callUpdater(ctx, http.MethodPost, updater.PathInstall)
	if err != nil {
		s.log.WarnContext(ctx, "updater not reached for the install", "err", err)
		s.forgetUpdaterProbe()
		return model.UpdateInstall{}, model.ErrUpdaterUnavailable
	}
	defer func() { _ = resp.Body.Close() }()
	var st updater.State
	decodeErr := decodeState(resp.Body, &st)
	switch resp.StatusCode {
	case http.StatusAccepted:
		if decodeErr != nil || !knownPhase(st.Phase) {
			s.log.WarnContext(ctx, "updater accepted the install with an unreadable state", "err", decodeErr)
			return s.InstallStatus(ctx)
		}
		s.log.InfoContext(ctx, "update install started", "from", st.From, "to", st.To, "phase", string(st.Phase))
		return installOf(st), nil
	case http.StatusConflict:
		return model.UpdateInstall{}, model.ErrUpdateRunning
	case http.StatusUnprocessableEntity:
		code := string(st.Code)
		if !problemCode.MatchString(code) {
			code = "update_refused"
		}
		s.log.InfoContext(ctx, "updater refused the install", "code", code)
		return model.UpdateInstall{}, &model.UpdateRefusedError{Code: code, Message: logging.RedactString(truncateRunes(st.Message, 500))}
	case http.StatusUnauthorized:
		s.log.WarnContext(ctx, "updater refused the server's token")
	default:
		s.log.WarnContext(ctx, "updater answered the install unexpectedly", "status", resp.StatusCode)
	}
	s.forgetUpdaterProbe()
	return model.UpdateInstall{}, model.ErrUpdaterUnavailable
}

// InstallStatus reads the state of the last install from state.json; idle when the companion
// never wrote one (or without a companion). It also reports a failed install once (reportInstall).
func (s *Service) InstallStatus(ctx context.Context) (model.UpdateInstall, error) {
	st, ok := s.installState(ctx)
	if !ok {
		return model.UpdateInstall{Phase: string(updater.PhaseIdle)}, nil
	}
	s.reportInstall(ctx, st)
	return installOf(st), nil
}

// WatchUpdateInstall looks at state.json at the start and reports an install that ended rolled
// back or failed once. While an install still runs (the companion waits for this server to become
// healthy, or brings the old one back), it looks again every few seconds, at most
// installWatchLimit long. It returns when ctx ends.
func (s *Service) WatchUpdateInstall(ctx context.Context) {
	u := &s.ops.updater
	deadline := s.now().Add(installWatchLimit)
	for {
		st, ok := s.installState(ctx)
		if !ok {
			return
		}
		s.reportInstall(ctx, st)
		if !st.Phase.Running() || !s.now().Before(deadline) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(u.watchEvery):
		}
	}
}

// withUpdaterState adds the companion to the state of the update check: whether it is reachable
// and the last install it reported.
func (s *Service) withUpdaterState(ctx context.Context, out model.UpdateState) model.UpdateState {
	out.UpdaterAvailable = s.updaterAvailable(ctx)
	if st, ok := s.installState(ctx); ok {
		s.reportInstall(ctx, st)
		in := installOf(st)
		out.Install = &in
	}
	return out
}

// updaterAvailable reports whether GET /status answers 200 with the token, from a cache of
// updaterProbeTTL.
func (s *Service) updaterAvailable(ctx context.Context) bool {
	u := &s.ops.updater
	if u.url == "" {
		return false
	}
	u.probeMu.Lock()
	defer u.probeMu.Unlock()
	if !u.probedAt.IsZero() && s.now().Sub(u.probedAt) < updaterProbeTTL {
		return u.available
	}
	ctx, cancel := context.WithTimeout(ctx, updaterProbeTimeout)
	defer cancel()
	ok := false
	resp, err := s.callUpdater(ctx, http.MethodGet, updater.PathStatus)
	switch {
	case err != nil:
		s.log.DebugContext(ctx, "updater not reached", "err", err)
	default:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, updater.MaxFileBytes))
		_ = resp.Body.Close()
		ok = resp.StatusCode == http.StatusOK
		if !ok {
			s.log.DebugContext(ctx, "updater status refused", "status", resp.StatusCode)
		}
	}
	u.probedAt, u.available = s.now(), ok
	return ok
}

// forgetUpdaterProbe makes the next updaterAvailable ask again.
func (s *Service) forgetUpdaterProbe() {
	u := &s.ops.updater
	u.probeMu.Lock()
	u.probedAt = time.Time{}
	u.probeMu.Unlock()
}

// callUpdater sends one request with the token to the companion. The token goes only into the
// header; neither it nor the header is ever logged.
func (s *Service) callUpdater(ctx context.Context, method, path string) (*http.Response, error) {
	u := &s.ops.updater
	token, err := readToken(filepath.Join(u.dir, updater.TokenFile))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.url+path, nil)
	if err != nil {
		return nil, fmt.Errorf("updater request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "Slouchcat/"+s.version())
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("updater %s %s: %w", method, path, err)
	}
	return resp, nil
}

// readToken reads the shared token; an error never contains it.
func readToken(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // G304: the fixed token file inside the shared updater folder
	if err != nil {
		return "", fmt.Errorf("updater token: %w", err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxTokenBytes+1))
	if err != nil {
		return "", fmt.Errorf("updater token: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if len(data) > maxTokenBytes || token == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", errors.New("updater token: empty or malformed")
	}
	return token, nil
}

// installState reads state.json; false when it is missing, unreadable, too large or names an
// unknown phase. A failure is logged once until the file reads again.
func (s *Service) installState(ctx context.Context) (updater.State, bool) {
	u := &s.ops.updater
	if u.dir == "" {
		return updater.State{}, false
	}
	var st updater.State
	err := updater.ReadJSON(filepath.Join(u.dir, updater.StateFile), &st)
	if errors.Is(err, fs.ErrNotExist) {
		return updater.State{}, false
	}
	if err == nil && !knownPhase(st.Phase) {
		err = fmt.Errorf("read %s: unknown phase %q", updater.StateFile, truncateRunes(string(st.Phase), 40))
	}
	u.stateMu.Lock()
	defer u.stateMu.Unlock()
	if err != nil {
		if !u.stateBad {
			s.log.WarnContext(ctx, "updater state unreadable", "err", err)
		}
		u.stateBad = true
		return updater.State{}, false
	}
	u.stateBad = false
	return st, true
}

// reportInstall sends an install that ended rolled back or failed (not one refused before it
// changed anything) once as the event errors: once means its finish time is newer than the
// setting update_install_reported, which it then becomes. A failed send is tried again at the
// next read.
func (s *Service) reportInstall(ctx context.Context, st updater.State) {
	u := &s.ops.updater
	if u.failed == nil || st.FinishedAt.IsZero() || refusalCodes[st.Code] ||
		(st.Phase != updater.PhaseRolledBack && st.Phase != updater.PhaseFailed) {
		return
	}
	u.reportMu.Lock()
	defer u.reportMu.Unlock()
	if !u.reportLoaded {
		raw, ok, err := s.setting(ctx, SettingUpdateInstallReported)
		if err != nil {
			s.log.WarnContext(ctx, "read the reported update install", "err", err)
			return
		}
		if ok {
			if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
				u.reported = t
			}
		}
		u.reportLoaded = true
	}
	if !st.FinishedAt.After(u.reported) {
		return
	}
	if err := u.failed(ctx, installOf(st)); err != nil {
		s.log.WarnContext(ctx, "report the failed update install", "err", err)
		return
	}
	at := st.FinishedAt.UTC().Format(time.RFC3339Nano)
	if err := s.write.UpsertSetting(ctx, appq.UpsertSettingParams{Key: SettingUpdateInstallReported, Value: at, Now: s.now().UnixMilli()}); err != nil {
		s.log.WarnContext(ctx, "store the reported update install", "err", err)
		return
	}
	u.reported = st.FinishedAt
	s.log.InfoContext(ctx, "failed update install reported", "phase", string(st.Phase), "code", string(st.Code), "to", st.To)
}

// decodeState decodes a State from the companion's answer, at most updater.MaxFileBytes.
func decodeState(r io.Reader, st *updater.State) error {
	data, err := io.ReadAll(io.LimitReader(r, updater.MaxFileBytes+1))
	if err != nil {
		return err
	}
	if len(data) > updater.MaxFileBytes {
		return updater.ErrTooLarge
	}
	return json.Unmarshal(data, st)
}

func knownPhase(p updater.Phase) bool {
	return p == updater.PhaseIdle || p.Running() || p.Final()
}

func installOf(st updater.State) model.UpdateInstall {
	out := model.UpdateInstall{
		Phase: string(st.Phase), From: st.From, To: st.To, Migrated: st.Migrated, Backup: st.Backup,
		Message: logging.RedactString(truncateRunes(st.Message, 500)),
	}
	if problemCode.MatchString(string(st.Code)) { // the contract's form of a code
		out.Code = string(st.Code)
	}
	if !st.StartedAt.IsZero() {
		t := st.StartedAt.UTC()
		out.StartedAt = &t
	}
	if !st.FinishedAt.IsZero() {
		t := st.FinishedAt.UTC()
		out.FinishedAt = &t
	}
	return out
}
