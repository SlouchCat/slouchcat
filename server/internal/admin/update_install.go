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
	"sync/atomic"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/updater"
	"github.com/slouchcat/slouchcat/server/internal/version"
)

// Installing an update through the updater companion (concept 15 9.3, ADR 0024). The server never
// sees the Docker socket: it asks the companion over the internal Docker network to install the
// newest release (InstallUpdate), and the companion picks the version itself. The progress lives in
// state.json in the shared folder, which only the companion writes; the server reads it directly
// (InstallStatus), so the card shows it while the companion or the server restarts. Each request
// carries the token from the shared folder as a bearer token, and the token never reaches a log
// (CLAUDE.md rule 10). An install that ended rolled back or failed is reported once as the event
// errors (reportInstall); which one was reported is the setting update_install_reported, not a
// mark in state.json. While an install runs, the server neither restarts nor takes or restores a
// backup, and the other way round (installRunning, backups.mu).

// SettingUpdateInstallReported is the finish time (RFC 3339) of the last failed or rolled back
// install that was sent as the event errors.
const SettingUpdateInstallReported = "update_install_reported"

const (
	// updaterProbeTimeout bounds GET /status for UpdateState.UpdaterAvailable; updaterProbeTTL is
	// how long its answer counts, so the admin pages stay fast while the companion is down.
	updaterProbeTimeout = 2 * time.Second
	updaterProbeTTL     = 10 * time.Second
	// updaterProbeWait is how long a read of the admin pages waits for a probe that runs; a slower
	// probe answers the next read, so the dashboard poll never waits for a companion that is down.
	updaterProbeWait = 500 * time.Millisecond
	// updaterInstallTimeout bounds POST /install. The companion answers only after its checks: the
	// release manifest (up to 20 s) and docker inspect of the server container, about 25 s at
	// worst.
	updaterInstallTimeout = 40 * time.Second
	// installStaleAfter: a running phase in state.json this long after the start while the companion
	// does not answer was interrupted (the companion is gone), and the server reports it as failed.
	// It lies above the longest install of the companion (updater.InstallWorstCase, 66 minutes) with
	// a margin for the docker inspect calls of its waits: 70 minutes. It also bounds how long
	// followInstall tries again to send a final result.
	installStaleAfter = updater.InstallWorstCase + 4*time.Minute
	// installRetryMax caps the pause between two tries to send a failed install as the event errors.
	installRetryMax = time.Minute
	maxTokenBytes   = 4 << 10
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
	kick       chan struct{} // InstallUpdate started an install: WatchUpdateInstall follows it

	// starting is set while InstallUpdate waits for the companion's answer (it holds backups.mu).
	starting atomic.Bool

	probeMu   sync.Mutex // guards probedAt, available, version and probing
	probedAt  time.Time
	available bool
	version   string        // the companion's version from its status answer; "" = unknown
	probing   chan struct{} // closed when the running probe ends; nil = none

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
	u.kick = make(chan struct{}, 1)
	s.forgetUpdaterProbe() // an answer of another companion counts no more
	return s
}

// InstallUpdate asks the companion to install the newest release and returns the state it
// reported: model.ErrUpdaterUnavailable without a companion, when it does not answer or refuses
// the token; model.ErrUpdateRunning while an install runs; *model.UpdateRefusedError when it
// refused before it changed anything. Like RestartServer it answers model.ErrRestartPending while a
// restart or restore is scheduled and model.ErrBackupBusy while a backup or restore runs, and it
// holds that lock until the companion answered. A started install is followed by
// WatchUpdateInstall, so a failure while this server keeps running is reported at once.
func (s *Service) InstallUpdate(ctx context.Context) (model.UpdateInstall, error) {
	u := &s.ops.updater
	if u.url == "" {
		return model.UpdateInstall{}, model.ErrUpdaterUnavailable
	}
	if s.backups.restartPending.Load() {
		return model.UpdateInstall{}, model.ErrRestartPending
	}
	if !s.backups.mu.TryLock() {
		return model.UpdateInstall{}, model.ErrBackupBusy
	}
	defer s.backups.mu.Unlock()
	if s.backups.restartPending.Load() { // scheduled by a restore that held the lock until now
		return model.UpdateInstall{}, model.ErrRestartPending
	}
	u.starting.Store(true)
	defer u.starting.Store(false)
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
		select { // WatchUpdateInstall follows the install; one waiting kick is enough
		case u.kick <- struct{}{}:
		default:
		}
		if decodeErr != nil || !knownPhase(st.Phase) {
			// state.json may still hold the previous install (a done one would reload the page):
			// the card shows a running install and follows state.json from there.
			s.log.WarnContext(ctx, "updater accepted the install with an unreadable state", "err", decodeErr)
			return model.UpdateInstall{Phase: string(updater.PhaseChecking)}, nil
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

// InstallStatus reads the state of the last install from state.json as the admin pages show it
// (shownInstall); idle when the companion never wrote one (or without a companion). It also
// reports a failed install once (reportInstall).
func (s *Service) InstallStatus(ctx context.Context) (model.UpdateInstall, error) {
	st, ok := s.installState(ctx)
	if !ok {
		return model.UpdateInstall{Phase: string(updater.PhaseIdle)}, nil
	}
	s.reportInstall(ctx, st)
	if st, ok = s.shownInstall(ctx, st); !ok {
		return model.UpdateInstall{Phase: string(updater.PhaseIdle)}, nil
	}
	return installOf(st), nil
}

// WatchUpdateInstall runs while ctx lives; main starts it once. It follows the install found in
// state.json at the start (the companion may still wait for this server to become healthy, or
// bring the old one back) and every install this server started (InstallUpdate), and reports one
// that ended rolled back or failed once (followInstall).
func (s *Service) WatchUpdateInstall(ctx context.Context) {
	u := &s.ops.updater
	for {
		s.followInstall(ctx)
		select {
		case <-ctx.Done():
			return
		case <-u.kick:
		}
	}
}

// followInstall looks at state.json every few seconds while an install runs, until it ended or
// counts as interrupted (shownInstall: older than installStaleAfter while the companion does not
// answer), and sends a final result that failed or rolled back as the event errors. A send that
// failed is tried again with a growing pause, at most installStaleAfter after the first try.
func (s *Service) followInstall(ctx context.Context) {
	u := &s.ops.updater
	var deadline time.Time // of the tries to send a final result; zero while the install runs
	pause := u.watchEvery
	for {
		st, ok := s.installState(ctx)
		if !ok {
			return
		}
		delivered := s.reportInstall(ctx, st)
		running := st.Phase.Running()
		if running {
			if shown, _ := s.shownInstall(ctx, st); !shown.Phase.Running() {
				return // interrupted: the companion is gone
			}
		} else {
			if deadline.IsZero() {
				deadline = s.now().Add(installStaleAfter)
			}
			if delivered || !s.now().Before(deadline) {
				return
			}
		}
		wait := u.watchEvery
		if !running {
			wait, pause = pause, min(2*pause, installRetryMax)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// installRunning reports whether an install is under way as the admin pages show it: the server
// waits for the companion's answer, or state.json names a running phase that is not stale. A
// restart, a restore or a new backup would cut it off (model.ErrUpdateRunning).
func (s *Service) installRunning(ctx context.Context) bool {
	if s.ops.updater.starting.Load() {
		return true
	}
	st, ok := s.installState(ctx)
	if !ok {
		return false
	}
	st, ok = s.shownInstall(ctx, st)
	return ok && st.Phase.Running()
}

// shownInstall is the install as the admin pages show it; the server decides (CLAUDE.md rule 12).
// A running phase older than installStaleAfter (or without a start) while the companion does not
// answer was interrupted: it is shown as failed without a code. A final result that names neither
// from nor to as the running version belongs to an install before a later manual update: ok is
// false, nothing is shown.
func (s *Service) shownInstall(ctx context.Context, st updater.State) (updater.State, bool) {
	switch {
	case st.Phase.Running():
		if (st.StartedAt.IsZero() || s.now().Sub(st.StartedAt) >= installStaleAfter) && !s.updaterAvailable(ctx) {
			st.Message = fmt.Sprintf("the install stopped during the phase %s and the updater companion does not answer; "+
				"check SLOUCHCAT_VERSION in the env file", st.Phase)
			st.Phase, st.Code = updater.PhaseFailed, ""
		}
	case st.Phase.Final():
		if !s.isRunningVersion(st.From) && !s.isRunningVersion(st.To) {
			return updater.State{}, false
		}
	}
	return st, true
}

// isRunningVersion reports whether v is the running version; a self-built image of a release
// counts as that release (package version).
func (s *Service) isRunningVersion(v string) bool {
	if v == "" {
		return false
	}
	running := s.version()
	c, ok := version.Compare(v, running)
	return v == running || (ok && c == 0)
}

// withUpdaterState adds the companion to the state of the update check: whether it is configured
// and reachable, and the last install it reported as the admin pages show it.
func (s *Service) withUpdaterState(ctx context.Context, out model.UpdateState) model.UpdateState {
	out.UpdaterConfigured = s.ops.updater.url != ""
	out.UpdaterAvailable = s.updaterAvailable(ctx)
	if out.UpdaterAvailable {
		u := &s.ops.updater
		u.probeMu.Lock()
		out.UpdaterVersion = u.version
		u.probeMu.Unlock()
	}
	if st, ok := s.installState(ctx); ok {
		s.reportInstall(ctx, st)
		if st, ok = s.shownInstall(ctx, st); ok {
			in := installOf(st)
			out.Install = &in
		}
	}
	return out
}

// updaterAvailable reports whether GET /status answers 200 with the token, from a cache of
// updaterProbeTTL. One probe runs at a time, in the background: a caller waits for it at most
// updaterProbeWait and otherwise gets the last answer (false before the first).
func (s *Service) updaterAvailable(ctx context.Context) bool {
	u := &s.ops.updater
	if u.url == "" {
		return false
	}
	u.probeMu.Lock()
	if !u.probedAt.IsZero() && s.now().Sub(u.probedAt) < updaterProbeTTL {
		defer u.probeMu.Unlock()
		return u.available
	}
	done := u.probing
	if done == nil {
		done = make(chan struct{})
		u.probing = done
		go s.probeUpdater(context.WithoutCancel(ctx), done)
	}
	u.probeMu.Unlock()
	timer := time.NewTimer(updaterProbeWait)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	case <-ctx.Done():
	}
	u.probeMu.Lock()
	defer u.probeMu.Unlock()
	return u.available
}

// probeUpdater asks GET /status once, stores the answer and closes done.
func (s *Service) probeUpdater(ctx context.Context, done chan struct{}) {
	u := &s.ops.updater
	ctx, cancel := context.WithTimeout(ctx, updaterProbeTimeout)
	defer cancel()
	ok, ver := false, ""
	resp, err := s.callUpdater(ctx, http.MethodGet, updater.PathStatus)
	switch {
	case err != nil:
		s.log.DebugContext(ctx, "updater not reached", "err", err)
	default:
		ok = resp.StatusCode == http.StatusOK
		if ok {
			// The version is shown on the card; one that is no semantic version (or an answer that
			// does not read) counts as unknown, like that of a companion before 1.18.0.
			var st updater.State
			if err := decodeState(resp.Body, &st); err == nil && isRelease(st.Updater) {
				ver = truncateRunes(st.Updater, 40)
			}
		} else {
			s.log.DebugContext(ctx, "updater status refused", "status", resp.StatusCode)
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, updater.MaxFileBytes))
		_ = resp.Body.Close()
	}
	u.probeMu.Lock()
	u.probedAt, u.available, u.version, u.probing = s.now(), ok, ver, nil
	u.probeMu.Unlock()
	close(done)
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
// setting update_install_reported, which it then becomes. It returns false when the send failed
// (or the setting could not be read); followInstall and the next read try again. The finish
// time counts as reported in memory once sent, also when storing the setting fails.
func (s *Service) reportInstall(ctx context.Context, st updater.State) bool {
	u := &s.ops.updater
	if u.failed == nil || st.FinishedAt.IsZero() || refusalCodes[st.Code] ||
		(st.Phase != updater.PhaseRolledBack && st.Phase != updater.PhaseFailed) {
		return true
	}
	u.reportMu.Lock()
	defer u.reportMu.Unlock()
	if !u.reportLoaded {
		raw, ok, err := s.setting(ctx, SettingUpdateInstallReported)
		if err != nil {
			s.log.WarnContext(ctx, "read the reported update install", "err", err)
			return false
		}
		if ok {
			if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
				u.reported = t
			}
		}
		u.reportLoaded = true
	}
	if !st.FinishedAt.After(u.reported) {
		return true
	}
	if err := u.failed(ctx, installOf(st)); err != nil {
		s.log.WarnContext(ctx, "report the failed update install", "err", err)
		return false
	}
	u.reported = st.FinishedAt // sent: not again in this process, even if the setting is not stored
	at := st.FinishedAt.UTC().Format(time.RFC3339Nano)
	if err := s.write.UpsertSetting(ctx, appq.UpsertSettingParams{Key: SettingUpdateInstallReported, Value: at, Now: s.now().UnixMilli()}); err != nil {
		s.log.WarnContext(ctx, "store the reported update install", "err", err)
	}
	s.log.InfoContext(ctx, "failed update install reported", "phase", string(st.Phase), "code", string(st.Code), "to", st.To)
	return true
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
