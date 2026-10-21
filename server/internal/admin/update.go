package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Update check of M8 (A-5, design 3.5). The job update.check reads the release manifest
// {"version", "url", "notes", "publishedAt"} (JSON, at most 64 KiB, 10 s), compares its version
// with the running one and stores the result in the setting update_state. RunUpdateCheck queues it
// 60 s after the start and then whenever the last check is a day old, the way metadata.RunTrakt
// schedules the Trakt lists. The check only reads the manifest; it never installs anything. Switched
// off (update_check_enabled "0") there is no job, no stored state and no notice.
const (
	JobUpdateCheck = "update.check"
	updateJobKey   = "update.check"
	updatePriority = 1

	// UpdateInterval is the age of the last check after which RunUpdateCheck queues the next.
	UpdateInterval     = 24 * time.Hour
	updatePollInterval = 15 * time.Minute
	updateStartDelay   = 60 * time.Second
	updateTimeout      = 10 * time.Second
	maxManifestBytes   = 64 << 10
)

// storedUpdate is the setting update_state (times in Unix ms).
type storedUpdate struct {
	LatestVersion string `json:"latestVersion,omitempty"`
	URL           string `json:"url,omitempty"`
	Notes         string `json:"notes,omitempty"`
	CheckedAt     int64  `json:"checkedAt,omitempty"`
	Error         string `json:"error,omitempty"`
}

// manifest is the release manifest.
type manifest struct {
	Version     string `json:"version"`
	URL         string `json:"url"`
	Notes       string `json:"notes"`
	PublishedAt string `json:"publishedAt"`
}

// updateCheck is the update part of the service.
type updateCheck struct {
	manifest string
	client   *http.Client
	running  sync.Mutex // one check at a time

	mu     sync.Mutex // guards loaded and notice, the cache of UpdateAvailable
	loaded bool
	notice *model.UpdateNotice
}

// WithUpdateCheck sets the release manifest (config.ManifestURL) and the HTTP client of the check
// (nil: a client with the 10 s timeout) and returns the receiver.
func (s *Service) WithUpdateCheck(manifestURL string, client *http.Client) *Service {
	if client == nil {
		client = &http.Client{Timeout: updateTimeout}
	}
	s.ops.update.manifest, s.ops.update.client = manifestURL, client
	return s
}

// RegisterJobs registers the handlers of the admin jobs (update.check) on w.
func (s *Service) RegisterJobs(w *jobs.Worker) {
	w.Handle(JobUpdateCheck, s.handleUpdateCheck)
}

// RunUpdateCheck queues update.check 60 s after the start when it is due and then checks every
// 15 minutes, until ctx ends.
func (s *Service) RunUpdateCheck(ctx context.Context) {
	start := time.NewTimer(updateStartDelay)
	defer start.Stop()
	select {
	case <-ctx.Done():
		return
	case <-start.C:
	}
	t := time.NewTicker(updatePollInterval)
	defer t.Stop()
	for {
		if _, err := s.QueueDueUpdateCheck(ctx); err != nil && ctx.Err() == nil {
			s.log.WarnContext(ctx, "queue update check", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// QueueDueUpdateCheck queues update.check when the check is on and the last one is UpdateInterval
// old or there was none (one tick of RunUpdateCheck). It reports whether it queued.
func (s *Service) QueueDueUpdateCheck(ctx context.Context) (bool, error) {
	if s.queue == nil {
		return false, nil
	}
	on, err := s.updateCheckEnabled(ctx)
	if err != nil || !on {
		return false, err
	}
	st, err := s.storedUpdate(ctx)
	if err != nil {
		return false, err
	}
	if st.CheckedAt != 0 && s.now().Sub(time.UnixMilli(st.CheckedAt)) < UpdateInterval {
		return false, nil
	}
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobUpdateCheck, Priority: updatePriority, IdempotencyKey: updateJobKey,
		Payload: []byte(`{}`)}); err != nil {
		return false, fmt.Errorf("enqueue %s: %w", JobUpdateCheck, err)
	}
	return true, nil
}

// handleUpdateCheck runs one check. A failed read of the manifest is stored in the state and not
// retried (the next daily run or the admin tries again); only a failure to store it is an error.
func (s *Service) handleUpdateCheck(ctx context.Context, _ *jobs.Job) error {
	on, err := s.updateCheckEnabled(ctx)
	if err != nil || !on {
		return err
	}
	_, err = s.checkUpdate(ctx)
	return err
}

// UpdateState returns the state of the update check (A-5).
func (s *Service) UpdateState(ctx context.Context) (model.UpdateState, error) {
	on, err := s.updateCheckEnabled(ctx)
	if err != nil {
		return model.UpdateState{}, err
	}
	out := model.UpdateState{Enabled: on, CurrentVersion: s.version()}
	if !on {
		return out, nil
	}
	st, err := s.storedUpdate(ctx)
	if err != nil {
		return model.UpdateState{}, err
	}
	out.LatestVersion, out.URL, out.Notes, out.Error = st.LatestVersion, st.URL, st.Notes, st.Error
	out.Available = st.URL != "" && newerVersion(st.LatestVersion, out.CurrentVersion)
	if st.CheckedAt != 0 {
		t := time.UnixMilli(st.CheckedAt).UTC()
		out.CheckedAt = &t
	}
	return out, nil
}

// CheckForUpdate reads the release manifest now; a failure is reported in the state's Error
// (model.ErrUpdateCheckDisabled when the check is off).
func (s *Service) CheckForUpdate(ctx context.Context) (model.UpdateState, error) {
	on, err := s.updateCheckEnabled(ctx)
	if err != nil {
		return model.UpdateState{}, err
	}
	if !on {
		return model.UpdateState{}, model.ErrUpdateCheckDisabled
	}
	return s.checkUpdate(ctx)
}

// UpdateAvailable returns the header notice of the main account (SessionInfo.updateAvailable):
// the newer published version, or nil. It is answered from memory; a check or a change of the
// setting refreshes it.
func (s *Service) UpdateAvailable(ctx context.Context) *model.UpdateNotice {
	u := &s.ops.update
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.loaded {
		st, err := s.UpdateState(ctx)
		if err != nil {
			s.log.WarnContext(ctx, "read update state", "err", err)
			return nil
		}
		u.notice, u.loaded = noticeOf(st), true
	}
	if u.notice == nil {
		return nil
	}
	n := *u.notice
	return &n
}

func noticeOf(st model.UpdateState) *model.UpdateNotice {
	if !st.Available {
		return nil
	}
	return &model.UpdateNotice{Version: st.LatestVersion, URL: st.URL}
}

// forgetUpdateNotice makes UpdateAvailable read the state again.
func (s *Service) forgetUpdateNotice() {
	s.ops.update.mu.Lock()
	s.ops.update.loaded = false
	s.ops.update.mu.Unlock()
}

// checkUpdate reads the manifest, stores the result and returns the new state. A failed read keeps
// the last known release and records the error.
func (s *Service) checkUpdate(ctx context.Context) (model.UpdateState, error) {
	s.ops.update.running.Lock()
	defer s.ops.update.running.Unlock()
	st, err := s.storedUpdate(ctx)
	if err != nil {
		return model.UpdateState{}, err
	}
	st.CheckedAt, st.Error = s.now().UnixMilli(), ""
	m, err := s.fetchManifest(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return model.UpdateState{}, ctx.Err()
		}
		st.Error = logging.RedactString(err.Error())
		s.log.WarnContext(ctx, "update check failed", "err", err)
	} else {
		st.LatestVersion, st.URL, st.Notes = m.Version, m.URL, m.Notes
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return model.UpdateState{}, fmt.Errorf("encode update state: %w", err)
	}
	if err := s.write.UpsertSetting(ctx, appq.UpsertSettingParams{Key: SettingUpdateState, Value: string(raw), Now: st.CheckedAt}); err != nil {
		return model.UpdateState{}, fmt.Errorf("store update state: %w", err)
	}
	s.forgetUpdateNotice()
	out, err := s.UpdateState(ctx)
	if err == nil && st.Error == "" {
		s.log.InfoContext(ctx, "update check done", "latest", st.LatestVersion, "available", out.Available)
	}
	return out, err
}

// errManifest marks a manifest that arrived but is not usable.
var errManifest = errors.New("release manifest invalid")

// fetchManifest reads and checks the release manifest.
func (s *Service) fetchManifest(ctx context.Context) (manifest, error) {
	u := &s.ops.update
	if u.manifest == "" || u.client == nil {
		return manifest{}, errors.New("no release manifest configured")
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.manifest, nil)
	if err != nil {
		return manifest{}, fmt.Errorf("manifest request: %w", err)
	}
	req.Header.Set("User-Agent", "Slouchcat/"+s.version())
	req.Header.Set("Accept", "application/json")
	resp, err := u.client.Do(req)
	if err != nil {
		return manifest{}, fmt.Errorf("release manifest unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return manifest{}, fmt.Errorf("release manifest: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes+1))
	if err != nil {
		return manifest{}, fmt.Errorf("read release manifest: %w", err)
	}
	if len(body) > maxManifestBytes {
		return manifest{}, fmt.Errorf("%w: larger than 64 KiB", errManifest)
	}
	var m manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return manifest{}, fmt.Errorf("%w: not JSON", errManifest)
	}
	if _, ok := parseSemver(m.Version); !ok {
		return manifest{}, fmt.Errorf("%w: version %q", errManifest, truncateRunes(m.Version, 40))
	}
	if p, err := url.Parse(m.URL); err != nil || (p.Scheme != "https" && p.Scheme != "http") || p.Host == "" {
		return manifest{}, fmt.Errorf("%w: url", errManifest)
	}
	return m, nil
}

// storedUpdate reads update_state; zero when absent or unreadable.
func (s *Service) storedUpdate(ctx context.Context) (storedUpdate, error) {
	raw, ok, err := s.setting(ctx, SettingUpdateState)
	if err != nil || !ok {
		return storedUpdate{}, err
	}
	var st storedUpdate
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		s.log.WarnContext(ctx, "stored update state unreadable", "err", err)
		return storedUpdate{}, nil
	}
	return st, nil
}

// updateCheckEnabled reads update_check_enabled; on unless "0".
func (s *Service) updateCheckEnabled(ctx context.Context) (bool, error) {
	v, _, err := s.setting(ctx, SettingUpdateCheckEnabled)
	return v != "0", err
}

// version is the running version; "dev" without one.
func (s *Service) version() string {
	if v := s.ops.system.Version; v != "" {
		return v
	}
	return "dev"
}

func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
