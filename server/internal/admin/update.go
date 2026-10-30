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
	"github.com/slouchcat/slouchcat/server/internal/version"
)

// Update check of M8 (A-5, design 3.5). The job update.check reads the release manifest
// {"version", "url", "notes", "publishedAt"} (JSON, at most 64 KiB, 10 s), compares its version
// with the running one and stores the result in the setting update_state. The schedule update.check
// (concept 09) queues it 60 s after the start and then every UpdateInterval; its switch is the
// setting update_check_enabled (one value, two doors: this page and the jobs page). The check only
// reads the manifest; it never installs anything. Switched off (update_check_enabled "0") there is
// no job, no stored state and no notice.
const (
	JobUpdateCheck = "update.check"
	updateJobKey   = "update.check"
	// updatePriority puts the check before every scan, match, probe, image and segment job (the
	// priority ladder is in the package documentation of jobs). It reads one small file once a
	// day; at priority 1 it waited behind the keyframe indexes of a library import for days and
	// the administration showed a stale release.
	updatePriority = 8

	// UpdateInterval is the default interval of the schedule update.check; updateStartDelay its
	// start delay (the first row takes them, after that the table job_schedules is the source).
	UpdateInterval   = 24 * time.Hour
	updateStartDelay = 60 * time.Second
	updateTimeout    = 10 * time.Second
	maxManifestBytes = 64 << 10
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

// RegisterTypes registers the job types of the administration on r (concept 09, ADR 0017):
// update.check is a schedule type of the group system, every UpdateInterval, 60 s after the start,
// switched by update_check_enabled.
func (s *Service) RegisterTypes(r *jobs.Registry) {
	delay := updateStartDelay
	r.Register(jobs.Type{
		Name: JobUpdateCheck, Group: jobs.GroupSystem, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: updatePriority, Handler: s.handleUpdateCheck, Run: s.runUpdateCheck,
		Every: UpdateInterval, StartDelay: &delay, Seed: s.updateSeed,
	})
}

// runUpdateCheck is the Run of the schedule update.check (jobs.Type): one job under the key
// update.check with prio, raised when it already waits. Switched off it queues nothing (the
// schedule is disabled then as well; this is the guard for a manual run).
func (s *Service) runUpdateCheck(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
	on, err := s.updateCheckEnabled(ctx)
	if err != nil || !on {
		return nil, err
	}
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobUpdateCheck, Priority: prio, IdempotencyKey: updateJobKey, Payload: []byte(`{}`)})
	if err != nil {
		return nil, fmt.Errorf("enqueue %s: %w", JobUpdateCheck, err)
	}
	if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, fmt.Errorf("raise %s: %w", JobUpdateCheck, err)
	}
	return []int64{id}, nil
}

// updateSeed is the Seed of the schedule update.check: the switch of the setting and the time of
// the last check, so the first row after migration 00027 continues the daily rhythm.
func (s *Service) updateSeed(ctx context.Context) (jobs.ScheduleSeed, error) {
	on, err := s.updateCheckEnabled(ctx)
	if err != nil {
		return jobs.ScheduleSeed{}, err
	}
	seed := jobs.ScheduleSeed{Every: UpdateInterval, Enabled: on}
	if on {
		st, err := s.storedUpdate(ctx)
		if err != nil {
			return jobs.ScheduleSeed{}, err
		}
		if st.CheckedAt != 0 {
			seed.LastRun = time.UnixMilli(st.CheckedAt)
		}
	}
	return seed, nil
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
	out.Development = !isRelease(out.CurrentVersion)
	if !on {
		return out, nil
	}
	st, err := s.storedUpdate(ctx)
	if err != nil {
		return model.UpdateState{}, err
	}
	out.LatestVersion, out.URL, out.Notes, out.Error = st.LatestVersion, st.URL, st.Notes, st.Error
	out.Available = st.URL != "" && !out.Development && newerVersion(st.LatestVersion, out.CurrentVersion)
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
	if _, ok := version.Parse(m.Version); !ok {
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

// isRelease reports whether v is a semantic version, with or without the suffix `git describe`
// adds to a tag. A bare commit id, an image tag or "dev" is a development build: it is not older
// than any release, it is simply not comparable, and the state says so (UpdateState.Development)
// instead of asking for an update that may be older than what runs.
func isRelease(v string) bool {
	_, ok := version.Parse(v)
	return ok
}

// newerVersion reports whether latest is newer than current. Neither side is newer when it does not
// parse; a development build is isRelease's business, not this one's.
func newerVersion(latest, current string) bool {
	l, ok := version.Parse(latest)
	if !ok {
		return false
	}
	c, ok := version.Parse(current)
	return ok && l.Compare(c) > 0
}
