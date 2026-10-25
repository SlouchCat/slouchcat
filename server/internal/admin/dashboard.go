package admin

import (
	"context"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Dashboard of M8 (A-3, design section 1): running streams, job counters with the newest failed
// jobs and their retry or cancellation, free space of the config and cache directories, the state of
// every integration, the newest errors of the server log and the update state, in one request that
// the overview polls.

// recentFailedLimit is how many failed jobs the dashboard lists.
const recentFailedLimit = 10

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
	failed, err := s.ops.jobs.ListFailed(ctx, recentFailedLimit)
	if err != nil {
		return model.Dashboard{}, err
	}
	d.Jobs.RecentFailed = make([]model.JobSummary, 0, len(failed))
	for _, j := range failed {
		d.Jobs.RecentFailed = append(d.Jobs.RecentFailed, jobSummary(j))
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
	ok, err := s.ops.jobs.Cancel(ctx, id)
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
