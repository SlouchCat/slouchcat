package admin

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Dashboard of M8 (A-3, design section 1): running streams, job counters with the newest failed
// jobs and their retry or cancellation, free space of the config and cache directories, the state of
// every integration, the newest errors of the server log and the update state, in one request that
// the overview polls.

// recentFailedLimit is how many failed jobs the dashboard lists.
const recentFailedLimit = 10

// Concept 11: the running jobs the overview lists, and how long the counts of the sidebar are
// kept. The sidebar polls the dashboard on every admin page; the counts read every title.
const (
	activeJobsLimit    = 3
	dashboardCountsTTL = 60 * time.Second
)

// countsCache keeps the counts of the sidebar for dashboardCountsTTL. gen grows with every
// ForgetDashboardCounts, so a computation that overlapped a change does not store its result.
// One computation runs per generation (flight): while it runs, the other callers get the expired
// values of the same generation, or wait for it when there are none.
type countsCache struct {
	mu     sync.Mutex
	gen    uint64
	at     time.Time
	ok     bool // vals belong to gen (possibly expired)
	vals   model.DashboardCounts
	flight *countsFlight
	// compute replaces computeDashboardCounts when set; a seam for tests.
	compute func(ctx context.Context, now time.Time) (model.DashboardCounts, error)
}

// countsFlight is the running computation of one generation; done closes when vals and err are set.
type countsFlight struct {
	gen  uint64
	done chan struct{}
	vals model.DashboardCounts
	err  error
}

// Jobs is implemented by *jobs.Queue: the counters, the failed jobs and their administration.
type Jobs interface {
	Stats(ctx context.Context) (queued, running, failed int, err error)
	// ListFailed returns the newest failed jobs, at most limit.
	ListFailed(ctx context.Context, limit int) ([]jobs.Job, error)
	// Retry queues a failed job again (model.ErrNotFound, model.ErrJobNotFailed).
	Retry(ctx context.Context, id int64) (jobs.Job, error)
	// Cancel cancels a queued or running job and reports whether there was one.
	Cancel(ctx context.Context, id int64) (bool, error)
}

// Errors is implemented by *logging.ErrorRing: the newest error records of the log, redacted.
type Errors interface {
	Entries() []logging.ErrorEntry
}

// Channels is implemented by package notify (lane S3): whether the server can send on each
// channel, keyed by model.ChannelEmail, model.ChannelPush and model.ChannelTelegram.
type Channels interface {
	ChannelAvailability(ctx context.Context) (map[string]model.ChannelAvail, error)
}

// opsState holds the collaborators and state of M8 lane S1: system state, jobs, errors,
// notification settings and the update check.
type opsState struct {
	system   SystemSource
	jobs     Jobs
	errors   Errors
	channels Channels
	telegram TelegramChecker
	email    EmailTester
	push     PushRotator
	env      NotifyEnv
	update   updateCheck
	updater  updaterLink // the updater companion (update_install.go, ADR 0024)
}

// WithJobs sets the job administration and returns the receiver.
func (s *Service) WithJobs(j Jobs) *Service {
	s.ops.jobs = j
	return s
}

// WithErrors sets the error ring of the log and returns the receiver.
func (s *Service) WithErrors(e Errors) *Service {
	s.ops.errors = e
	return s
}

// WithChannels sets the channel state of package notify and returns the receiver. Without it the
// dashboard derives e-mail, push and Telegram from the stored configuration alone.
func (s *Service) WithChannels(c Channels) *Service {
	s.ops.channels = c
	return s
}

// Dashboard returns the admin overview (A-3); ErrNotConfigured without WithJobs.
func (s *Service) Dashboard(ctx context.Context) (model.Dashboard, error) {
	if s.ops.jobs == nil {
		return model.Dashboard{}, ErrNotConfigured
	}
	d := model.Dashboard{Streams: []model.AdminStream{}, Errors: []model.ErrorEntry{}}
	var err error
	if s.streams != nil {
		if d.Streams, err = s.ListStreams(ctx); err != nil {
			return model.Dashboard{}, err
		}
	}
	if d.Jobs.Queued, d.Jobs.Running, d.Jobs.Failed, err = s.ops.jobs.Stats(ctx); err != nil {
		return model.Dashboard{}, err
	}
	if n, err := s.sys.CountPausedJobTypes(ctx); err != nil {
		return model.Dashboard{}, fmt.Errorf("count paused job types: %w", err)
	} else {
		d.Jobs.Paused = int(n)
	}
	failed, err := s.ops.jobs.ListFailed(ctx, recentFailedLimit)
	if err != nil {
		return model.Dashboard{}, err
	}
	d.Jobs.RecentFailed = make([]model.JobSummary, 0, len(failed))
	for _, j := range failed {
		d.Jobs.RecentFailed = append(d.Jobs.RecentFailed, jobSummary(j))
	}
	if err := s.dashboardJobs(ctx, &d.Jobs); err != nil {
		return model.Dashboard{}, err
	}
	libs, err := s.read.ListLibraries(ctx)
	if err != nil {
		return model.Dashboard{}, fmt.Errorf("list libraries: %w", err)
	}
	d.Attention = dashboardAttention(libs)
	if d.Counts, err = s.dashboardCounts(ctx); err != nil {
		return model.Dashboard{}, err
	}
	if d.Activity, err = s.recentActivity(ctx); err != nil {
		return model.Dashboard{}, err
	}
	d.Config = s.storage(ctx, s.ops.system.ConfigDir)
	d.Cache = s.storage(ctx, s.ops.system.CacheDir)
	if d.Update, err = s.UpdateState(ctx); err != nil {
		return model.Dashboard{}, err
	}
	if d.Integrations, err = s.integrations(ctx, d.Update); err != nil {
		return model.Dashboard{}, err
	}
	// The webhook tile (M9): without the integrations service the tile stays empty instead of
	// failing the whole dashboard.
	if s.webhooks != nil {
		if d.Webhooks, err = s.WebhookStatus(ctx); err != nil {
			return model.Dashboard{}, err
		}
	}
	d.Webhooks.Silent = append([]string{}, d.Webhooks.Silent...)
	d.Browse = s.browseHealth()
	if s.ops.errors != nil {
		for _, e := range s.ops.errors.Entries() {
			msg := e.Message
			if cause := e.Attrs["err"]; cause != "" {
				msg += ": " + cause
			}
			d.Errors = append(d.Errors, model.ErrorEntry{At: e.At, Source: e.Source, Message: msg, JobID: e.JobID, JobType: e.JobType})
		}
	}
	return d, nil
}

// dashboardJobs adds the running jobs, the next scheduled run and the failures by type of
// concept 11 to the job counters.
func (s *Service) dashboardJobs(ctx context.Context, jc *model.JobCounts) error {
	now := s.now()
	running, err := s.sys.ListRunningJobs(ctx, activeJobsLimit)
	if err != nil {
		return fmt.Errorf("list running jobs: %w", err)
	}
	jc.Active = make([]model.JobRun, 0, len(running))
	medians := map[string]*typeStats{} // the estimate of "Hintergrundjobs" needs the median of the type
	for _, r := range running {
		st, ok := medians[r.Type]
		if !ok && s.jobctl != nil {
			st = &typeStats{}
			if st.median, _, st.medianOK, err = s.jobctl.reg.Queue().MedianDuration(ctx, r.Type); err != nil {
				return fmt.Errorf("median duration of %s: %w", r.Type, err)
			}
			medians[r.Type] = st
		}
		jc.Active = append(jc.Active, s.jobRun(ctx, jobs.FromRow(r), st, now))
	}
	if jc.NextRun, err = s.nextScheduledRun(ctx); err != nil {
		return fmt.Errorf("next scheduled run: %w", err)
	}
	rows, err := s.sys.ListFailedJobsNewestPerType(ctx)
	if err != nil {
		return fmt.Errorf("list failed jobs by type: %w", err)
	}
	jc.FailedByType = make([]model.JobTypeFailures, 0, len(rows))
	for _, r := range rows {
		jc.FailedByType = append(jc.FailedByType, model.JobTypeFailures{
			Type: r.Type, Count: int(r.FailedCount), LastError: logging.RedactString(r.LastError.String),
			LastFailedAt: time.UnixMilli(r.UpdatedAt).UTC(),
		})
	}
	return nil
}

// nextScheduledRun returns the earliest next run over the switched-on schedules of registered,
// unpaused schedule types; nil without one or without WithJobControl.
func (s *Service) nextScheduledRun(ctx context.Context) (*model.JobNextRun, error) {
	if s.jobctl == nil {
		return nil, nil
	}
	schedules, err := s.jobctl.sched.Schedules(ctx)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	typeRows, err := s.sys.ListJobTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list job types: %w", err)
	}
	paused := map[string]bool{}
	for _, r := range typeRows {
		paused[r.Type] = r.PausedAt.Valid
	}
	var next *model.JobNextRun
	for _, sc := range schedules {
		if !sc.Enabled || paused[sc.Type] || sc.NextRunAt.IsZero() {
			continue
		}
		if t, ok := s.jobctl.reg.Lookup(sc.Type); !ok || !t.Scheduled() {
			continue
		}
		if next == nil || sc.NextRunAt.Before(next.At) {
			next = &model.JobNextRun{Type: sc.Type, At: sc.NextRunAt.UTC()}
		}
	}
	return next, nil
}

// dashboardAttention lists the libraries whose last scan the loss protection aborted and whose
// loss nobody accepted: state aborted, a reason other than cancelled (the administrator's own
// cancellation) and no loss_accepted_at. The server decides this, the client shows it (rule 12).
func dashboardAttention(libs []appq.Library) model.DashboardAttention {
	out := model.DashboardAttention{AbortedScans: []model.AbortedScan{}}
	for _, l := range libs {
		if l.LastScanState != "aborted" || !l.LastScanError.Valid || l.LastScanError.String == abortCancelled || l.LossAcceptedAt.Valid {
			continue
		}
		out.AbortedScans = append(out.AbortedScans, model.AbortedScan{LibraryID: l.ID, Name: l.Name, Reason: l.LastScanError.String})
	}
	return out
}

// abortCancelled is the abort reason of a scan the administrator cancelled (package library).
const abortCancelled = "cancelled"

// dashboardCounts returns the counts of the sidebar, recomputed at most every dashboardCountsTTL
// and at once after ForgetDashboardCounts.
func (s *Service) dashboardCounts(ctx context.Context) (model.DashboardCounts, error) {
	now := s.now()
	c := &s.counts
	c.mu.Lock()
	if c.ok && now.Sub(c.at) < dashboardCountsTTL {
		vals := c.vals
		c.mu.Unlock()
		return vals, nil
	}
	if f := c.flight; f != nil && f.gen == c.gen {
		if c.ok { // expired values of this generation while the computation runs
			vals := c.vals
			c.mu.Unlock()
			return vals, nil
		}
		c.mu.Unlock()
		select {
		case <-f.done:
			return f.vals, f.err
		case <-ctx.Done():
			return model.DashboardCounts{}, ctx.Err()
		}
	}
	f := &countsFlight{gen: c.gen, done: make(chan struct{})}
	c.flight = f

	compute := s.computeDashboardCounts
	if c.compute != nil {
		compute = c.compute
	}
	c.mu.Unlock()

	// The computation outlives a cancelled first caller, so the callers waiting on it get a result.
	f.vals, f.err = compute(context.WithoutCancel(ctx), now)
	c.mu.Lock()
	if f.err == nil && c.gen == f.gen {
		c.vals, c.at, c.ok = f.vals, now, true
	}
	if c.flight == f {
		c.flight = nil
	}
	c.mu.Unlock()
	close(f.done)
	return f.vals, f.err
}

// ForgetDashboardCounts drops the kept counts of the sidebar; the next dashboard recomputes them.
// The maintenance, libraries and accounts pages call it after every change (concept 11).
func (s *Service) ForgetDashboardCounts() {
	s.counts.mu.Lock()
	s.counts.gen++
	s.counts.ok = false
	s.counts.mu.Unlock()
}

func (s *Service) computeDashboardCounts(ctx context.Context, now time.Time) (model.DashboardCounts, error) {
	var out model.DashboardCounts
	titles, err := s.sys.CountAllLibraryItems(ctx)
	if err != nil {
		return out, fmt.Errorf("count items: %w", err)
	}
	out.Titles = int(titles)
	accounts, err := s.read.CountAccounts(ctx)
	if err != nil {
		return out, fmt.Errorf("count accounts: %w", err)
	}
	out.Accounts = int(accounts)
	// The five cheap maintenance counters of every library; rating_mismatch asks the provider for
	// every title and does not count.
	all := sql.NullInt64{}
	unmatched, err := s.sys.AdminCountUnmatched(ctx, systemq.AdminCountUnmatchedParams{PendingBefore: now.Add(-pendingAfter).UnixMilli(), LibraryID: all})
	if err != nil {
		return out, fmt.Errorf("count unmatched: %w", err)
	}
	counters := []struct {
		name  string
		count func(context.Context, interface{}) (int64, error)
	}{
		{IssueNoImages, s.sys.AdminCountNoImages},
		{IssueUnrated, s.sys.AdminCountUnrated},
		{IssueOrphaned, s.sys.AdminCountOrphaned},
		{IssueUnplayable, s.sys.AdminCountUnplayable},
	}
	open := unmatched
	for _, c := range counters {
		n, err := c.count(ctx, all)
		if err != nil {
			return out, fmt.Errorf("count %s: %w", c.name, err)
		}
		open += n
	}
	out.MaintenanceOpen = int(open)
	return out, nil
}

// storage returns the space of dir's file system; zero sizes when it cannot be read.
func (s *Service) storage(ctx context.Context, dir string) model.StorageInfo {
	info := model.StorageInfo{Path: dir}
	if dir == "" {
		return info
	}
	total, free, err := diskSpace(dir)
	if err != nil {
		s.log.WarnContext(ctx, "disk space", "dir", dir, "err", err)
		return info
	}
	info.TotalBytes, info.FreeBytes = total, free
	return info
}

// integrations returns the state of every integration in the order of the contract.
func (s *Service) integrations(ctx context.Context, update model.UpdateState) ([]model.IntegrationStatus, error) {
	out := make([]model.IntegrationStatus, 0, 8)
	for _, k := range providerKeys {
		if k.field == model.ClearTVDBPin {
			continue // part of the TVDB key
		}
		if k.field == model.ClearFanart || k.field == model.ClearFanartClient {
			continue // an image source switched on the page "Metadaten", not an integration of the overview
		}
		ks, err := s.keyState(ctx, k)
		if err != nil {
			return nil, err
		}
		st := model.IntegrationStatus{Key: k.field, State: model.IntegrationConfigured}
		if ks.Source == model.SourceNone {
			st.State = model.IntegrationMissing
		}
		if k.field == model.ClearTrakt && st.State == model.IntegrationConfigured && s.trakt != nil {
			ts, err := s.trakt.TraktStatus(ctx)
			if err != nil {
				return nil, err
			}
			st.CheckedAt = ts.LastSuccess
			if ts.LastError != "" {
				st.State, st.Detail, st.CheckedAt = model.IntegrationError, logging.RedactString(ts.LastError), ts.LastAttempt
			}
		}
		out = append(out, st)
	}
	avail, err := s.channelAvailability(ctx)
	if err != nil {
		return nil, err
	}
	for _, ch := range []string{model.ChannelEmail, model.ChannelPush, model.ChannelTelegram} {
		a := avail[ch]
		st := model.IntegrationStatus{Key: ch, State: model.IntegrationConfigured}
		switch {
		case !a.Configured:
			st.State = model.IntegrationMissing
		case a.Detail != "":
			st.State, st.Detail = model.IntegrationError, logging.RedactString(a.Detail)
		}
		out = append(out, st)
	}
	st := model.IntegrationStatus{Key: "updateCheck", State: model.IntegrationConfigured, CheckedAt: update.CheckedAt}
	switch {
	case !update.Enabled:
		st.State = model.IntegrationDisabled
	case update.Error != "":
		st.State, st.Detail = model.IntegrationError, update.Error
	}
	return append(out, st), nil
}

// channelAvailability asks package notify; without it the configuration alone decides.
func (s *Service) channelAvailability(ctx context.Context) (map[string]model.ChannelAvail, error) {
	if s.ops.channels != nil {
		return s.ops.channels.ChannelAvailability(ctx)
	}
	_, email, err := s.SMTPConfig(ctx)
	if err != nil {
		return nil, err
	}
	pub, _, err := s.VAPIDKeys(ctx)
	if err != nil {
		return nil, err
	}
	token, err := s.TelegramToken(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]model.ChannelAvail{
		model.ChannelEmail: {Configured: email}, model.ChannelPush: {Configured: pub != ""}, model.ChannelTelegram: {Configured: token != ""},
	}, nil
}

// RetryJob queues a failed job again (model.ErrNotFound, model.ErrJobNotFailed).
func (s *Service) RetryJob(ctx context.Context, id int64) (model.JobSummary, error) {
	if s.ops.jobs == nil {
		return model.JobSummary{}, ErrNotConfigured
	}
	j, err := s.ops.jobs.Retry(ctx, id)
	if err != nil {
		return model.JobSummary{}, err
	}
	s.log.InfoContext(ctx, "job queued again by the admin", "job_id", id, "queued_job_id", j.ID, "job_type", j.Type)
	return jobSummary(j), nil
}

// CancelJob cancels a queued or running job or dismisses a failed one (model.ErrNotFound for an
// unknown, done or cancelled one).
func (s *Service) CancelJob(ctx context.Context, id int64) error {
	if s.ops.jobs == nil {
		return ErrNotConfigured
	}
	var ok bool
	var err error
	if s.jobctl != nil {
		ok, err = s.jobctl.reg.CancelJob(ctx, id) // through the cancel hook of the type (concept 09, 6.3)
	} else {
		ok, err = s.ops.jobs.Cancel(ctx, id)
	}
	if err != nil {
		return err
	}
	if !ok {
		return model.ErrNotFound
	}
	s.log.InfoContext(ctx, "job cancelled by the admin", "job_id", id)
	return nil
}

// jobSummary maps a job; its last error is redacted like the log.
func jobSummary(j jobs.Job) model.JobSummary {
	return model.JobSummary{
		ID: j.ID, Type: j.Type, State: string(j.State), Attempts: j.Attempts, MaxAttempts: max(j.MaxAttempts, 1),
		LastError: logging.RedactString(j.LastError), CreatedAt: utcOrZero(j.CreatedAt), UpdatedAt: utcOrZero(j.UpdatedAt),
	}
}

func utcOrZero(t time.Time) time.Time {
	if t.IsZero() || t.UnixMilli() == 0 {
		return time.Time{}
	}
	return t.UTC()
}
