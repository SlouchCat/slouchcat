package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The page "Hintergrundjobs" (concept 09, ADR 0017, step 3): the job types of the registry with
// their runs, schedules and estimates, and the seven operations of the contract. Every estimate
// is computed here, from the median duration of a type and the reported progress of a run; the
// client counts seconds up and decides nothing (rule 12).

// Estimation rules of concept 09, 6.2.
const (
	jobRunsPerType        = 10 // running runs listed per type
	jobRunsLimitDefault   = 50
	jobRunsLimitMax       = 200
	measuredMinElapsed    = 30 * time.Second // a measured estimate needs this much elapsed time
	measuredMinFraction   = 20               // and done >= total / 20 (5 %)
	jobPodcastPoll        = "podcast.poll"
	jobTypeUnknownGroup   = string(jobs.GroupSystem)
	jobTypeUnknownPool    = string(jobs.PoolGeneral)
	jobTypeUnknownTrigger = string(jobs.TriggerEvent)
)

// jobControl are the registry and scheduler of package jobs and the worker count per pool.
type jobControl struct {
	reg     *jobs.Registry
	sched   *jobs.Scheduler
	workers map[jobs.Pool]int
}

// WithJobControl sets the registry, the scheduler and the workers per pool (main.go) and returns
// the receiver; the scheduler also serves as Schedules for the settings pages.
func (s *Service) WithJobControl(reg *jobs.Registry, sched *jobs.Scheduler, workers map[jobs.Pool]int) *Service {
	s.jobctl = &jobControl{reg: reg, sched: sched, workers: workers}
	s.schedules = sched
	return s
}

// jobType resolves a type name to its registered type (known) or its row in job_types (known
// false, after a downgrade); neither is model.ErrNotFound.
func (s *Service) jobType(ctx context.Context, typ string) (t jobs.Type, known bool, err error) {
	if s.jobctl == nil {
		return jobs.Type{}, false, ErrNotConfigured
	}
	if t, ok := s.jobctl.reg.Lookup(typ); ok {
		return t, true, nil
	}
	if _, err := s.sys.GetJobType(ctx, typ); errors.Is(err, sql.ErrNoRows) {
		return jobs.Type{}, false, model.ErrNotFound
	} else if err != nil {
		return jobs.Type{}, false, fmt.Errorf("get job type %s: %w", typ, err)
	}
	return jobs.Type{Name: typ}, false, nil
}

// ListJobTypes returns the overview of the page (listJobTypes).
func (s *Service) ListJobTypes(ctx context.Context) (model.JobOverview, error) {
	if s.jobctl == nil {
		return model.JobOverview{}, ErrNotConfigured
	}
	ov, err := s.jobOverview(ctx)
	if err != nil {
		return model.JobOverview{}, err
	}
	return ov, nil
}

// typeStats are the per-type numbers the estimates rest on.
type typeStats struct {
	counts    systemq.JobCountsByTypeRow
	median    time.Duration
	medianOK  bool
	pool      jobs.Pool
	known     bool
	minPrio   int
	queuedAny bool
}

func (s *Service) jobOverview(ctx context.Context) (model.JobOverview, error) {
	now := s.now()
	q := s.jobctl.reg.Queue()
	countRows, err := s.sys.JobCountsByType(ctx, now.UnixMilli())
	if err != nil {
		return model.JobOverview{}, fmt.Errorf("count jobs by type: %w", err)
	}
	counts := map[string]systemq.JobCountsByTypeRow{}
	for _, r := range countRows {
		counts[r.Type] = r
	}
	typeRows, err := s.sys.ListJobTypes(ctx)
	if err != nil {
		return model.JobOverview{}, fmt.Errorf("list job types: %w", err)
	}
	paused := map[string]time.Time{}
	for _, r := range typeRows {
		if r.PausedAt.Valid {
			paused[r.Type] = time.UnixMilli(r.PausedAt.Int64).UTC()
		}
	}
	// Every registered type, then every row of job_types that no type explains (known = false).
	types := s.jobctl.reg.Types()
	names := map[string]bool{}
	for _, t := range types {
		names[t.Name] = true
	}
	for _, r := range typeRows {
		if !names[r.Type] {
			types = append(types, jobs.Type{Name: r.Type})
		}
	}
	stats := map[string]*typeStats{}
	for _, t := range types {
		st := &typeStats{counts: counts[t.Name], pool: t.Pool, known: names[t.Name]}
		st.minPrio = int(st.counts.MinQueuedPriority)
		st.queuedAny = st.counts.Queued > 0
		if st.known {
			if m, _, ok, err := q.MedianDuration(ctx, t.Name); err != nil {
				return model.JobOverview{}, err
			} else {
				st.median, st.medianOK = m, ok
			}
		}
		stats[t.Name] = st
	}
	out := model.JobOverview{Paused: len(paused), Types: make([]model.JobTypeInfo, 0, len(types))}
	for _, pool := range []jobs.Pool{jobs.PoolGeneral, jobs.PoolDownload, jobs.PoolNotify} {
		p := model.JobPool{Pool: string(pool), Workers: s.jobctl.workers[pool]}
		if backlog, ok := s.poolBacklog(stats, pool, -1); ok {
			p.BacklogSeconds = &backlog
		}
		out.Pools = append(out.Pools, p)
	}
	for _, t := range types {
		info, err := s.jobTypeInfo(ctx, t, stats, paused, now)
		if err != nil {
			return model.JobOverview{}, err
		}
		out.Types = append(out.Types, info)
	}
	return out, nil
}

// poolBacklog sums queued × median over the types of pool whose waiting runs have at least
// priority minPrio (-1: all), divided by the pool's workers; ok is false without any median.
func (s *Service) poolBacklog(stats map[string]*typeStats, pool jobs.Pool, minPrio int) (int, bool) {
	workers := max(s.jobctl.workers[pool], 1)
	var total time.Duration
	any := false
	for _, st := range stats {
		if !st.known || st.pool != pool || !st.queuedAny || !st.medianOK {
			continue
		}
		if minPrio >= 0 && st.minPrio < minPrio {
			continue
		}
		total += time.Duration(st.counts.Queued) * st.median
		any = true
	}
	if !any {
		return 0, false
	}
	return int(total.Seconds()) / workers, true
}

func (s *Service) jobTypeInfo(ctx context.Context, t jobs.Type, stats map[string]*typeStats, paused map[string]time.Time, now time.Time) (model.JobTypeInfo, error) {
	st := stats[t.Name]
	info := model.JobTypeInfo{
		Type: t.Name, Group: jobTypeUnknownGroup, Trigger: jobTypeUnknownTrigger, Pool: jobTypeUnknownPool, Known: st.known,
		Queued: int(st.counts.Queued), Planned: int(st.counts.Planned), Running: int(st.counts.Running), Failed: int(st.counts.Failed),
		RunningRuns: []model.JobRun{},
	}
	if at, ok := paused[t.Name]; ok {
		info.Paused, info.PausedAt = true, at
	}
	if st.known {
		info.Group, info.Trigger, info.Pool = string(t.Group), string(t.Trigger), string(t.Pool)
		info.Runnable = t.Run != nil
		info.Stopping = info.Paused && s.jobctl.reg.Running(t.Name) > 0
		if st.medianOK {
			m := int(st.median.Seconds())
			info.MedianSeconds = &m
		}
		if st.queuedAny {
			if wait, ok := s.poolBacklog(stats, t.Pool, st.minPrio); ok {
				info.QueuedEstimateSeconds = &wait
			}
		}
		if t.Scheduled() {
			sched, err := s.jobctl.sched.Schedule(ctx, t.Name)
			if err != nil && !errors.Is(err, model.ErrNotFound) {
				return model.JobTypeInfo{}, err
			}
			if err == nil {
				info.Schedule = s.jobSchedule(t.Name, sched)
			}
		}
	}
	running, err := s.sys.ListRunningJobsByType(ctx, systemq.ListRunningJobsByTypeParams{Type: t.Name, LimitCount: jobRunsPerType})
	if err != nil {
		return model.JobTypeInfo{}, fmt.Errorf("list running jobs of %s: %w", t.Name, err)
	}
	for _, r := range running {
		info.RunningRuns = append(info.RunningRuns, s.jobRun(ctx, jobs.FromRow(r), st, now))
	}
	last, err := s.sys.GetLastFinishedJobByType(ctx, t.Name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.JobTypeInfo{}, fmt.Errorf("last run of %s: %w", t.Name, err)
	}
	if err == nil {
		run := s.jobRun(ctx, jobs.FromRow(last), st, now)
		info.LastRun = &run
	}
	return info, nil
}

// scheduleLimits returns the bounds of the interval of a type: Trakt 6 to 48 hours (the spotlight
// page, 4.3), every other type the scheduler's.
func scheduleLimits(typ string) (minEvery, maxEvery time.Duration) {
	if typ == jobTraktRefresh {
		return minSpotlightInterval * time.Hour, maxSpotlightInterval * time.Hour
	}
	return jobs.MinEvery, jobs.MaxEvery
}

func (s *Service) jobSchedule(typ string, sc jobs.Schedule) *model.JobSchedule {
	lo, hi := scheduleLimits(typ)
	out := &model.JobSchedule{
		EverySeconds: int(sc.Every.Seconds()), NextRunAt: sc.NextRunAt.UTC(), Enabled: sc.Enabled,
		TimeZone: s.jobctl.sched.Location().String(), MinEverySeconds: int(lo.Seconds()), MaxEverySeconds: int(hi.Seconds()),
	}
	if !sc.LastFiredAt.IsZero() {
		out.LastFiredAt = sc.LastFiredAt.UTC()
	}
	if sc.Window != nil {
		out.Window = &model.JobWindow{StartMinute: sc.Window.Start, EndMinute: sc.Window.End}
	}
	return out
}

// jobRun maps a run with its subject, duration and estimate (concept 09, 6.2).
func (s *Service) jobRun(ctx context.Context, j jobs.Job, st *typeStats, now time.Time) model.JobRun {
	run := model.JobRun{JobSummary: jobSummary(j), Priority: j.Priority, Skipped: j.Skipped, Estimate: model.JobEstimateNone,
		StartedAt: utcOrZero(j.StartedAt), FinishedAt: utcOrZero(j.FinishedAt), Subject: s.jobSubject(ctx, j)}
	if j.State == jobs.StateQueued && j.RunAfter.After(now) {
		run.RunAfter = j.RunAfter.UTC()
	}
	if d := j.Duration(now); !j.StartedAt.IsZero() {
		run.Duration, run.HasDuration = d, true
	}
	if p := j.Progress; p.Done > 0 || p.Total > 0 || p.Note != "" {
		run.Progress = &model.JobProgress{Done: p.Done, Total: p.Total, Note: p.Note}
	}
	if j.State != jobs.StateRunning || j.StartedAt.IsZero() {
		return run
	}
	elapsed := now.Sub(j.StartedAt)
	p := j.Progress
	switch {
	case p.Total > 0 && p.Done > 0 && p.Done*measuredMinFraction >= p.Total && elapsed >= measuredMinElapsed:
		left := int((elapsed * time.Duration(p.Total-p.Done) / time.Duration(p.Done)).Seconds())
		run.EstimatedSecondsLeft, run.Estimate = &left, model.JobEstimateMeasured
	case st != nil && st.medianOK:
		run.Estimate = model.JobEstimateMedian
		if elapsed <= 2*st.median {
			left := int(max(st.median-elapsed, 0).Seconds())
			run.EstimatedSecondsLeft = &left
		}
	}
	return run
}

// jobSubject derives what a run is about from its payload: a library, a title with its episode
// marker, a channel, a file relative to its root. Best effort, redacted like the last error;
// "" when nothing can be derived (the client shows the id then).
func (s *Service) jobSubject(ctx context.Context, j jobs.Job) string {
	var p struct {
		LibraryID   int64  `json:"libraryId"`
		ItemID      string `json:"itemId"`
		PodcastID   string `json:"podcastId"`
		SeasonID    string `json:"seasonId"`
		MediaFileID string `json:"mediaFileId"`
		DeliveryID  int64  `json:"deliveryId"`
		DownloadID  string `json:"downloadId"`
	}
	if err := json.Unmarshal(j.Payload, &p); err != nil {
		return ""
	}
	var subject string
	switch {
	case p.LibraryID != 0:
		if lib, err := s.read.GetLibrary(ctx, p.LibraryID); err == nil {
			subject = lib.Name
		}
	case p.ItemID != "":
		subject = s.itemLabel(ctx, p.ItemID)
	case p.PodcastID != "":
		subject = s.itemLabel(ctx, p.PodcastID)
	case p.SeasonID != "":
		subject = s.itemLabel(ctx, p.SeasonID)
	case p.MediaFileID != "":
		if mf, err := s.sys.GetMediaFileSystem(ctx, p.MediaFileID); err == nil {
			subject = s.itemLabel(ctx, mf.ItemID)
			if base := path.Base(mf.RelPath); subject == "" {
				subject = base
			}
		}
	case p.DeliveryID != 0:
		if d, err := s.sys.DeliveryGet(ctx, p.DeliveryID); err == nil {
			subject = d.Channel
		}
	case p.DownloadID != "":
		if d, err := s.sys.GetDownload(ctx, p.DownloadID); err == nil {
			subject = s.itemLabel(ctx, d.ItemID)
		}
	}
	return logging.RedactString(strings.TrimSpace(subject))
}

// itemLabel names a title: a film or series by its title, a season as "Series · S01", an episode
// as "Series · S01E02"; "" for an unknown id.
func (s *Service) itemLabel(ctx context.Context, id string) string {
	it, err := s.sys.GetItemSystem(ctx, id)
	if err != nil {
		return ""
	}
	marker := func(it systemq.Item) string {
		switch {
		case it.SeasonNumber.Valid && it.EpisodeNumber.Valid:
			return fmt.Sprintf("S%02dE%02d", it.SeasonNumber.Int64, it.EpisodeNumber.Int64)
		case it.SeasonNumber.Valid:
			return fmt.Sprintf("S%02d", it.SeasonNumber.Int64)
		}
		return ""
	}
	m := marker(it)
	if m == "" || !it.ParentID.Valid {
		return it.Title
	}
	// Up to the series: an episode's parent is its season, the season's parent the series.
	top := it
	for range 2 {
		if !top.ParentID.Valid {
			break
		}
		parent, err := s.sys.GetItemSystem(ctx, top.ParentID.String)
		if err != nil {
			break
		}
		top = parent
	}
	return top.Title + " · " + m
}

// ListJobRuns returns the runs of a type in one state (listJobRuns); model.ErrNotFound for an
// unknown type, a validation error for an unknown state.
func (s *Service) ListJobRuns(ctx context.Context, typ, state string, limit int) (model.JobRunPage, error) {
	if _, _, err := s.jobType(ctx, typ); err != nil {
		return model.JobRunPage{}, err
	}
	if limit <= 0 {
		limit = jobRunsLimitDefault
	}
	limit = min(limit, jobRunsLimitMax)
	now := s.now()
	st := &typeStats{}
	if m, _, ok, err := s.jobctl.reg.Queue().MedianDuration(ctx, typ); err != nil {
		return model.JobRunPage{}, err
	} else {
		st.median, st.medianOK = m, ok
	}
	var rows []systemq.Job
	var total int64
	var err error
	switch state {
	case string(jobs.StateQueued), model.JobRunPlanned:
		planned := int64(0)
		if state == model.JobRunPlanned {
			planned = 1
		}
		rows, err = s.sys.ListQueuedJobRuns(ctx, systemq.ListQueuedJobRunsParams{Type: typ, Now: now.UnixMilli(), Planned: planned, LimitCount: int64(limit)})
		if err == nil {
			total, err = s.sys.CountQueuedJobRuns(ctx, systemq.CountQueuedJobRunsParams{Type: typ, Now: now.UnixMilli(), Planned: planned})
		}
	case string(jobs.StateRunning), string(jobs.StateDone), string(jobs.StateFailed), string(jobs.StateCancelled):
		rows, err = s.sys.ListJobRunsByState(ctx, systemq.ListJobRunsByStateParams{Type: typ, State: state, LimitCount: int64(limit)})
		if err == nil {
			total, err = s.sys.CountJobRunsByState(ctx, systemq.CountJobRunsByStateParams{Type: typ, State: state})
		}
	default:
		var verr *model.ValidationError
		return model.JobRunPage{}, verr.Add("state", "invalid_value")
	}
	if err != nil {
		return model.JobRunPage{}, fmt.Errorf("list %s runs of %s: %w", state, typ, err)
	}
	page := model.JobRunPage{Total: int(total), Runs: make([]model.JobRun, 0, len(rows))}
	for _, r := range rows {
		page.Runs = append(page.Runs, s.jobRun(ctx, jobs.FromRow(r), st, now))
	}
	return page, nil
}

// RunJobType runs a type now with the manual priority (runJobType, F-3, F-5): the queued runs are
// returned, the schedule does not move. model.ErrJobTypeNotRunnable for an event type or an
// unknown row, model.ErrJobTypePaused for a paused type, and the services' own refusals
// (model.ErrProviderNotConfigured, model.ErrRefreshTooSoon, model.ErrUpdateCheckDisabled).
func (s *Service) RunJobType(ctx context.Context, typ string) ([]model.JobRun, error) {
	t, known, err := s.jobType(ctx, typ)
	if err != nil {
		return nil, err
	}
	if !known || t.Run == nil {
		return nil, model.ErrJobTypeNotRunnable
	}
	q := s.jobctl.reg.Queue()
	if paused, err := q.TypePaused(ctx, typ); err != nil {
		return nil, err
	} else if paused {
		return nil, model.ErrJobTypePaused
	}
	var ids []int64
	switch {
	case typ == jobTraktRefresh && s.trakt != nil:
		// The same door as "Jetzt abrufen": client id, one fetch a minute, manual priority.
		if err := s.RefreshSpotlight(ctx); err != nil {
			return nil, err
		}
		active, err := q.ListActive(ctx, typ)
		if err != nil {
			return nil, err
		}
		for _, j := range active {
			ids = append(ids, j.ID)
		}
	case typ == JobUpdateCheck:
		on, err := s.updateCheckEnabled(ctx)
		if err != nil {
			return nil, err
		}
		if !on {
			return nil, model.ErrUpdateCheckDisabled
		}
		if ids, err = t.Run(ctx, q, jobs.ManualPriority); err != nil {
			return nil, err
		}
	case typ == jobPodcastPoll && s.podcasts != nil:
		// By hand every subscription is fetched, not only the due ones (4.3).
		if ids, err = s.podcasts.QueueAllPodcasts(ctx, jobs.ManualPriority); err != nil {
			return nil, err
		}
	default:
		if ids, err = t.Run(ctx, q, jobs.ManualPriority); err != nil {
			return nil, err
		}
	}
	s.log.InfoContext(ctx, "job type run by the admin", "job_type", typ, "jobs", len(ids))
	now := s.now()
	runs := make([]model.JobRun, 0, len(ids))
	for _, id := range ids {
		j, err := q.Get(ctx, id)
		if err != nil {
			continue
		}
		runs = append(runs, s.jobRun(ctx, j, nil, now))
	}
	return runs, nil
}

// PauseJobType pauses a type (pauseJobType) and returns it; model.ErrNotFound for an unknown one.
func (s *Service) PauseJobType(ctx context.Context, typ string) (model.JobTypeInfo, error) {
	if _, _, err := s.jobType(ctx, typ); err != nil {
		return model.JobTypeInfo{}, err
	}
	if err := s.jobctl.reg.Pause(ctx, typ); err != nil {
		return model.JobTypeInfo{}, err
	}
	s.log.InfoContext(ctx, "job type paused by the admin", "job_type", typ)
	return s.jobTypeInfoOf(ctx, typ)
}

// ResumeJobType ends the pause of a type (resumeJobType).
func (s *Service) ResumeJobType(ctx context.Context, typ string) (model.JobTypeInfo, error) {
	if _, _, err := s.jobType(ctx, typ); err != nil {
		return model.JobTypeInfo{}, err
	}
	if err := s.jobctl.reg.Resume(ctx, typ); err != nil {
		return model.JobTypeInfo{}, err
	}
	s.log.InfoContext(ctx, "job type resumed by the admin", "job_type", typ)
	return s.jobTypeInfoOf(ctx, typ)
}

// UpdateJobSchedule changes interval, window or switch of a schedule type (updateJobSchedule,
// F-4). The interval must lie within the type's limits (Trakt whole hours 6 to 48), the window be
// valid; the interval of trakt.refresh is written into spotlight_config and the switch of
// update.check into the setting as well (one value, two doors). model.ErrJobTypeNotScheduled for
// a type without schedule, *model.ValidationError for refused values.
func (s *Service) UpdateJobSchedule(ctx context.Context, typ string, upd model.JobScheduleUpdate) (model.JobTypeInfo, error) {
	t, known, err := s.jobType(ctx, typ)
	if err != nil {
		return model.JobTypeInfo{}, err
	}
	if !known || !t.Scheduled() {
		return model.JobTypeInfo{}, model.ErrJobTypeNotScheduled
	}
	var verr *model.ValidationError
	var change jobs.ScheduleUpdate
	if upd.EverySeconds != nil {
		every := time.Duration(*upd.EverySeconds) * time.Second
		lo, hi := scheduleLimits(typ)
		if every < lo || every > hi || (typ == jobTraktRefresh && every%time.Hour != 0) {
			verr = verr.Add("everySeconds", "invalid_value")
		}
		change.Every = &every
	}
	switch {
	case upd.ClearWindow:
		change.ClearWindow = true
	case upd.Window != nil:
		w := jobs.Window{Start: upd.Window.StartMinute, End: upd.Window.EndMinute}
		if w.Start < 0 || w.Start > 1439 || w.End < 0 || w.End > 1439 || w.Start == w.End {
			verr = verr.Add("window", "invalid_value")
		}
		change.Window = &w
	}
	change.Enabled = upd.Enabled
	if verr != nil {
		return model.JobTypeInfo{}, verr
	}
	if typ == jobTraktRefresh && change.Every != nil {
		cfg, err := s.write.GetSpotlightConfig(ctx)
		if err != nil {
			return model.JobTypeInfo{}, fmt.Errorf("get spotlight config: %w", err)
		}
		if err := s.write.SetSpotlightInterval(ctx, appq.SetSpotlightIntervalParams{IntervalHours: int64(*change.Every / time.Hour), Now: s.now().UnixMilli()}); err != nil {
			return model.JobTypeInfo{}, fmt.Errorf("set spotlight interval: %w", err)
		}
		s.log.InfoContext(ctx, "spotlight interval set from the jobs page", "hours", int64(*change.Every/time.Hour), "before", cfg.IntervalHours)
	}
	if typ == JobUpdateCheck && change.Enabled != nil {
		if err := s.setUpdateCheckEnabled(ctx, *change.Enabled); err != nil {
			return model.JobTypeInfo{}, err
		}
	}
	if _, err := s.jobctl.sched.UpdateSchedule(ctx, typ, change); err != nil {
		return model.JobTypeInfo{}, err
	}
	s.log.InfoContext(ctx, "job schedule updated by the admin", "job_type", typ)
	return s.jobTypeInfoOf(ctx, typ)
}

// CancelJobRuns cancels the waiting runs of a type, with all also the running ones
// (cancelJobRuns, F-2), through the cancel hook of the type; it returns how many.
func (s *Service) CancelJobRuns(ctx context.Context, typ string, all bool) (int, error) {
	if _, _, err := s.jobType(ctx, typ); err != nil {
		return 0, err
	}
	n, err := s.jobctl.reg.CancelRuns(ctx, typ, all)
	if err != nil {
		return 0, err
	}
	s.log.InfoContext(ctx, "job runs cancelled by the admin", "job_type", typ, "all", all, "cancelled", n)
	return n, nil
}

// jobTypeInfoOf returns the info of one type out of the overview.
func (s *Service) jobTypeInfoOf(ctx context.Context, typ string) (model.JobTypeInfo, error) {
	ov, err := s.jobOverview(ctx)
	if err != nil {
		return model.JobTypeInfo{}, err
	}
	i := slices.IndexFunc(ov.Types, func(t model.JobTypeInfo) bool { return t.Type == typ })
	if i < 0 {
		return model.JobTypeInfo{}, model.ErrNotFound
	}
	return ov.Types[i], nil
}
