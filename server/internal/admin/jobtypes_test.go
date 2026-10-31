package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The page "Hintergrundjobs" (concept 09, step 3) on the admin fixture: a registry with the
// update check of the service, a fake schedule type, a fake event type and a fake trakt.refresh,
// a scheduler in UTC and a manual clock.

type jobFixture struct {
	*fixture
	ctx   context.Context
	now   time.Time
	queue *jobs.Queue
	reg   *jobs.Registry
	sched *jobs.Scheduler
	runs  int // runs of the fake schedule type
}

const (
	fakeSchedule = "fake.schedule"
	fakeEvent    = "fake.event"
)

func newJobFixture(t *testing.T) *jobFixture {
	t.Helper()
	jf := &jobFixture{fixture: newFixture(t), ctx: context.Background(), now: time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)}
	jf.svc.now = func() time.Time { return jf.now }
	jf.queue = jobs.NewQueue(jf.db.Write, func() time.Time { return jf.now })
	jf.reg = jobs.NewRegistry(jf.queue, jf.svc.log)
	jf.svc.RegisterTypes(jf.reg)
	noop := func(context.Context, *jobs.Job) error { return nil }
	run := func(name string) func(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
		return func(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
			jf.runs++
			id, err := q.Enqueue(ctx, jobs.NewJob{Type: name, Priority: prio, IdempotencyKey: name})
			if err != nil {
				return nil, err
			}
			_, err = q.RaiseQueuedPriority(ctx, id, prio)
			return []int64{id}, err
		}
	}
	atStart := time.Duration(0)
	jf.reg.Register(jobs.Type{Name: fakeSchedule, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: 5, Handler: noop, Run: run(fakeSchedule), Every: 6 * time.Hour, StartDelay: &atStart})
	jf.reg.Register(jobs.Type{Name: fakeEvent, Group: jobs.GroupMetadata, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: 3, Handler: noop})
	jf.reg.Register(jobs.Type{Name: jobTraktRefresh, Group: jobs.GroupMetadata, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerSchedule,
		Priority: 2, Handler: noop, Run: run(jobTraktRefresh), Every: 24 * time.Hour})
	jf.sched = jobs.NewScheduler(jf.queue, jf.reg, jf.svc.log, jobs.WithLocation(time.UTC))
	if err := jf.sched.Start(jf.ctx); err != nil {
		t.Fatal(err)
	}
	jf.svc.WithJobs(jf.queue).WithJobControl(jf.reg, jf.sched, map[jobs.Pool]int{jobs.PoolGeneral: 2, jobs.PoolDownload: 1, jobs.PoolNotify: 1})
	return jf
}

func (jf *jobFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := jf.db.Write.ExecContext(jf.ctx, query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func (jf *jobFixture) typeInfo(t *testing.T, typ string) model.JobTypeInfo {
	t.Helper()
	ov, err := jf.svc.ListJobTypes(jf.ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, ti := range ov.Types {
		if ti.Type == typ {
			return ti
		}
	}
	t.Fatalf("type %s not in the overview: %+v", typ, ov.Types)
	return model.JobTypeInfo{}
}

// finish runs a job of typ to done after d on the clock (a worker's claim and complete).
func (jf *jobFixture) finish(t *testing.T, typ string, d time.Duration) {
	t.Helper()
	id, err := jf.queue.Enqueue(jf.ctx, jobs.NewJob{Type: typ})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jf.queue.Claim(jf.ctx, "w", []string{typ}); err != nil {
		t.Fatal(err)
	}
	jf.now = jf.now.Add(d)
	if err := jf.queue.Complete(jf.ctx, "w", id); err != nil {
		t.Fatal(err)
	}
}

// The overview: every type with group, trigger, pool, schedule and counts; the median after five
// runs; the waiting time cumulated by priority over the pool; a running run with its estimate; a
// row without a registered type as unknown; the paused count.
func TestJobTypesOverview(t *testing.T) {
	jf := newJobFixture(t)
	ctx := jf.ctx
	// Five runs of the event type of 10, 20, 30, 40, 50 s: median 30 s.
	for i := 1; i <= 5; i++ {
		jf.finish(t, fakeEvent, time.Duration(i)*10*time.Second)
	}
	// Three waiting runs at priority 3, one planned (in the future), one running since 40 s.
	for i := range 3 {
		if _, err := jf.queue.Enqueue(ctx, jobs.NewJob{Type: fakeEvent, Priority: 3, IdempotencyKey: "e" + string(rune('a'+i))}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := jf.queue.Enqueue(ctx, jobs.NewJob{Type: fakeEvent, Priority: 3, Delay: time.Hour, IdempotencyKey: "planned"}); err != nil {
		t.Fatal(err)
	}
	running, err := jf.queue.Claim(ctx, "w", []string{fakeEvent})
	if err != nil || running == nil {
		t.Fatal(err)
	}
	jf.now = jf.now.Add(40 * time.Second)
	// A waiting run of the schedule type at priority 5 (above 3): it counts for the event type's wait.
	if _, err := jf.svc.RunJobType(ctx, fakeSchedule); err != nil {
		t.Fatal(err)
	}
	jf.exec(t, `INSERT INTO job_types (type, updated_at) VALUES ('ghost.job', 1)`)
	jf.exec(t, `UPDATE job_types SET paused_at = 1 WHERE type = 'ghost.job'`)

	ov, err := jf.svc.ListJobTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ov.Paused != 1 {
		t.Errorf("paused = %d, want 1 (ghost.job)", ov.Paused)
	}
	ev := jf.typeInfo(t, fakeEvent)
	if ev.Group != "metadata" || ev.Trigger != "event" || ev.Pool != "general" || ev.Runnable || ev.Schedule != nil || !ev.Known {
		t.Errorf("event type = %+v", ev)
	}
	if ev.Queued != 2 || ev.Planned != 1 || ev.Running != 1 || ev.Failed != 0 {
		t.Errorf("counts of the event type = %d queued, %d planned, %d running, %d failed; want 2, 1, 1, 0", ev.Queued, ev.Planned, ev.Running, ev.Failed)
	}
	if ev.MedianSeconds == nil || *ev.MedianSeconds != 30 {
		t.Errorf("median = %v, want 30 s", ev.MedianSeconds)
	}
	// Wait: the event type's own 2 × 30 s plus nothing of the schedule type (no median), over 2 workers.
	if ev.QueuedEstimateSeconds == nil || *ev.QueuedEstimateSeconds != 30 {
		t.Errorf("queued estimate = %v, want 30 s (2 × 30 s / 2 workers)", ev.QueuedEstimateSeconds)
	}
	if len(ev.RunningRuns) != 1 || ev.RunningRuns[0].ID != running.ID {
		t.Fatalf("running runs = %+v", ev.RunningRuns)
	}
	run := ev.RunningRuns[0]
	if run.Estimate != model.JobEstimateMedian || run.EstimatedSecondsLeft == nil || *run.EstimatedSecondsLeft != 0 || !run.HasDuration || run.Duration != 40*time.Second {
		t.Errorf("running run = estimate %s left %v duration %v; want median, 0 s left (40 s > 30 s median), 40 s", run.Estimate, run.EstimatedSecondsLeft, run.Duration)
	}
	if ev.LastRun == nil || ev.LastRun.State != "done" || !ev.LastRun.HasDuration || ev.LastRun.Duration != 50*time.Second {
		t.Errorf("last run = %+v, want the done run of 50 s", ev.LastRun)
	}
	sc := jf.typeInfo(t, fakeSchedule)
	if sc.Schedule == nil || sc.Schedule.EverySeconds != 6*3600 || !sc.Schedule.Enabled || sc.Schedule.TimeZone != "UTC" || sc.Schedule.MinEverySeconds != 300 || sc.Schedule.MaxEverySeconds != 7*24*3600 || !sc.Runnable {
		t.Errorf("schedule type = %+v", sc)
	}
	if sc.Queued != 1 || sc.QueuedEstimateSeconds != nil {
		t.Errorf("schedule type counts = %d queued, estimate %v; want 1 and none (no median yet)", sc.Queued, sc.QueuedEstimateSeconds)
	}
	tr := jf.typeInfo(t, jobTraktRefresh)
	if tr.Schedule == nil || tr.Schedule.MinEverySeconds != 6*3600 || tr.Schedule.MaxEverySeconds != 48*3600 {
		t.Errorf("trakt limits = %+v, want 6 to 48 hours", tr.Schedule)
	}
	ghost := jf.typeInfo(t, "ghost.job")
	if ghost.Known || !ghost.Paused || ghost.Runnable {
		t.Errorf("unknown row = %+v, want known false, paused", ghost)
	}
	if up := jf.typeInfo(t, JobUpdateCheck); up.Group != "system" || up.Schedule == nil {
		t.Errorf("update.check = %+v", up)
	}
	var general *model.JobPool
	for i := range ov.Pools {
		if ov.Pools[i].Pool == "general" {
			general = &ov.Pools[i]
		}
	}
	if general == nil || general.Workers != 2 || general.BacklogSeconds == nil || *general.BacklogSeconds != 30 {
		t.Errorf("general pool = %+v, want 2 workers and a backlog of 30 s", general)
	}
	// The dashboard carries the paused count.
	d, err := jf.svc.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.Jobs.Paused != 1 {
		t.Errorf("dashboard paused = %d", d.Jobs.Paused)
	}
}

// A measured estimate from the reported progress once 5 % and 30 s are reached.
func TestJobRunMeasuredEstimate(t *testing.T) {
	jf := newJobFixture(t)
	if _, err := jf.queue.Enqueue(jf.ctx, jobs.NewJob{Type: fakeEvent}); err != nil {
		t.Fatal(err)
	}
	j, err := jf.queue.Claim(jf.ctx, "w", []string{fakeEvent})
	if err != nil || j == nil {
		t.Fatal(err)
	}
	if err := jf.queue.UpdateProgress(jf.ctx, "w", j.ID, jobs.RunProgress{Done: 25, Total: 100, Note: "Filme · Movies"}); err != nil {
		t.Fatal(err)
	}
	jf.now = jf.now.Add(60 * time.Second)
	ev := jf.typeInfo(t, fakeEvent)
	if len(ev.RunningRuns) != 1 {
		t.Fatal("no running run")
	}
	run := ev.RunningRuns[0]
	if run.Estimate != model.JobEstimateMeasured || run.EstimatedSecondsLeft == nil || *run.EstimatedSecondsLeft != 180 {
		t.Errorf("estimate = %s %v, want measured 180 s (60 s for 25 of 100)", run.Estimate, run.EstimatedSecondsLeft)
	}
	if run.Progress == nil || run.Progress.Done != 25 || run.Progress.Note != "Filme · Movies" {
		t.Errorf("progress = %+v", run.Progress)
	}

	// With a progress series on the worker the rate counts only the work since the series
	// began (B-3): 20 of 100 in 20 s leaves 80 s for the remaining 75, not 180 s. A series
	// shorter than measuredMinSeries gives no measured estimate (and no median: event type).
	jf.svc.jobctl.series = func(id int64) (time.Time, int64, bool) {
		return jf.now.Add(-20 * time.Second), 5, id == j.ID
	}
	run = jf.typeInfo(t, fakeEvent).RunningRuns[0]
	if run.Estimate != model.JobEstimateMeasured || run.EstimatedSecondsLeft == nil || *run.EstimatedSecondsLeft != 75 {
		t.Errorf("estimate with a series = %s %v, want measured 75 s (20 s for 20 of 100, 75 left)", run.Estimate, run.EstimatedSecondsLeft)
	}
	jf.svc.jobctl.series = func(int64) (time.Time, int64, bool) { return jf.now.Add(-5 * time.Second), 20, true }
	run = jf.typeInfo(t, fakeEvent).RunningRuns[0]
	if run.Estimate != model.JobEstimateNone || run.EstimatedSecondsLeft != nil {
		t.Errorf("estimate with a 5 s series = %s %v, want none", run.Estimate, run.EstimatedSecondsLeft)
	}
}

// runJobType: event types and unknown rows are not runnable, a paused type refuses, a schedule
// type queues at the manual priority and the next scheduled run does not move (F-5).
func TestRunJobType(t *testing.T) {
	jf := newJobFixture(t)
	ctx := jf.ctx
	if _, err := jf.svc.RunJobType(ctx, fakeEvent); !errors.Is(err, model.ErrJobTypeNotRunnable) {
		t.Errorf("run of an event type: %v", err)
	}
	if _, err := jf.svc.RunJobType(ctx, "nobody.home"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("run of an unknown type: %v", err)
	}
	jf.exec(t, `INSERT INTO job_types (type, updated_at) VALUES ('ghost.job', 1)`)
	if _, err := jf.svc.RunJobType(ctx, "ghost.job"); !errors.Is(err, model.ErrJobTypeNotRunnable) {
		t.Errorf("run of an unknown row: %v", err)
	}
	before, err := jf.sched.Schedule(ctx, fakeSchedule)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := jf.svc.RunJobType(ctx, fakeSchedule)
	if err != nil || len(runs) != 1 || runs[0].Priority != jobs.ManualPriority || runs[0].State != "queued" {
		t.Fatalf("run = %+v, %v; want one queued run at priority 7", runs, err)
	}
	if after, _ := jf.sched.Schedule(ctx, fakeSchedule); !after.NextRunAt.Equal(before.NextRunAt) {
		t.Errorf("next run moved from %v to %v by a manual run", before.NextRunAt, after.NextRunAt)
	}
	if _, err := jf.svc.PauseJobType(ctx, fakeSchedule); err != nil {
		t.Fatal(err)
	}
	if _, err := jf.svc.RunJobType(ctx, fakeSchedule); !errors.Is(err, model.ErrJobTypePaused) {
		t.Errorf("run of a paused type: %v", err)
	}
	// update.check switched off refuses like the button on its page.
	if _, err := jf.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{UpdateCheckEnabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	if _, err := jf.svc.RunJobType(ctx, JobUpdateCheck); !errors.Is(err, model.ErrUpdateCheckDisabled) {
		t.Errorf("run of the switched-off update check: %v", err)
	}
}

// Pause and resume return the type with its state; unknown types are 404.
func TestPauseResumeJobType(t *testing.T) {
	jf := newJobFixture(t)
	ctx := jf.ctx
	info, err := jf.svc.PauseJobType(ctx, fakeEvent)
	if err != nil || !info.Paused || info.PausedAt.IsZero() || info.Stopping {
		t.Fatalf("pause = %+v, %v", info, err)
	}
	if paused, _ := jf.queue.TypePaused(ctx, fakeEvent); !paused {
		t.Error("type not paused in the database")
	}
	info, err = jf.svc.ResumeJobType(ctx, fakeEvent)
	if err != nil || info.Paused || !info.PausedAt.IsZero() {
		t.Fatalf("resume = %+v, %v", info, err)
	}
	if _, err := jf.svc.PauseJobType(ctx, "nobody.home"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("pause unknown: %v", err)
	}
	if _, err := jf.svc.ResumeJobType(ctx, "nobody.home"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("resume unknown: %v", err)
	}
}

// updateJobSchedule: limits per type, the window, and one value, two doors for the interval of
// trakt.refresh (spotlight_config.interval_hours, whole hours 6 to 48) and the switch of
// update.check (the setting).
func TestUpdateJobSchedule(t *testing.T) {
	jf := newJobFixture(t)
	ctx := jf.ctx
	if _, err := jf.svc.UpdateJobSchedule(ctx, fakeEvent, model.JobScheduleUpdate{Enabled: ptr(false)}); !errors.Is(err, model.ErrJobTypeNotScheduled) {
		t.Errorf("schedule of an event type: %v", err)
	}
	if _, err := jf.svc.UpdateJobSchedule(ctx, "nobody.home", model.JobScheduleUpdate{}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("schedule of an unknown type: %v", err)
	}
	var verr *model.ValidationError
	for _, tc := range []struct {
		typ string
		upd model.JobScheduleUpdate
	}{
		{fakeSchedule, model.JobScheduleUpdate{EverySeconds: ptr(60)}},
		{fakeSchedule, model.JobScheduleUpdate{Window: &model.JobWindow{StartMinute: 60, EndMinute: 60}}},
		{jobTraktRefresh, model.JobScheduleUpdate{EverySeconds: ptr(5 * 3600)}},
		{jobTraktRefresh, model.JobScheduleUpdate{EverySeconds: ptr(90 * 60 * 5)}}, // 7.5 h: not whole hours
	} {
		if _, err := jf.svc.UpdateJobSchedule(ctx, tc.typ, tc.upd); !errors.As(err, &verr) {
			t.Errorf("%s %+v: %v, want a validation error", tc.typ, tc.upd, err)
		}
	}
	info, err := jf.svc.UpdateJobSchedule(ctx, fakeSchedule, model.JobScheduleUpdate{EverySeconds: ptr(2 * 3600), Window: &model.JobWindow{StartMinute: 180, EndMinute: 300}})
	if err != nil || info.Schedule == nil || info.Schedule.EverySeconds != 7200 || info.Schedule.Window == nil || info.Schedule.Window.StartMinute != 180 {
		t.Fatalf("update = %+v, %v", info.Schedule, err)
	}
	if next := info.Schedule.NextRunAt.In(time.UTC); next.Hour() != 3 || next.Minute() != 0 {
		t.Errorf("next run with the window 03:00-05:00 = %v", next)
	}
	info, err = jf.svc.UpdateJobSchedule(ctx, fakeSchedule, model.JobScheduleUpdate{ClearWindow: true})
	if err != nil || info.Schedule.Window != nil {
		t.Errorf("clear window = %+v, %v", info.Schedule, err)
	}
	// Trakt: whole hours within 6 to 48, written into the spotlight config too.
	info, err = jf.svc.UpdateJobSchedule(ctx, jobTraktRefresh, model.JobScheduleUpdate{EverySeconds: ptr(7 * 3600)})
	if err != nil || info.Schedule.EverySeconds != 7*3600 {
		t.Fatalf("trakt update = %+v, %v", info.Schedule, err)
	}
	if cfg, err := appq.New(jf.db.Read).GetSpotlightConfig(ctx); err != nil || cfg.IntervalHours != 7 {
		t.Errorf("spotlight interval = %d %v, want 7", cfg.IntervalHours, err)
	}
	// Update check: the switch is the setting.
	info, err = jf.svc.UpdateJobSchedule(ctx, JobUpdateCheck, model.JobScheduleUpdate{Enabled: ptr(false)})
	if err != nil || info.Schedule.Enabled {
		t.Fatalf("update.check off = %+v, %v", info.Schedule, err)
	}
	if st, err := jf.svc.UpdateState(ctx); err != nil || st.Enabled {
		t.Errorf("update state after switching off from the jobs page = %+v, %v", st, err)
	}
	info, err = jf.svc.UpdateJobSchedule(ctx, JobUpdateCheck, model.JobScheduleUpdate{Enabled: ptr(true)})
	if err != nil || !info.Schedule.Enabled {
		t.Fatalf("update.check on = %+v, %v", info.Schedule, err)
	}
	if st, err := jf.svc.UpdateState(ctx); err != nil || !st.Enabled {
		t.Errorf("update state after switching on = %+v, %v", st, err)
	}
}

// listJobRuns and cancelJobRuns: queued runs in claim order, planned apart, totals; the queued
// scope cancels the waiting ones (planned included), all also the running one.
func TestListAndCancelJobRuns(t *testing.T) {
	jf := newJobFixture(t)
	ctx := jf.ctx
	for i, prio := range []int{2, 5, 3} {
		if _, err := jf.queue.Enqueue(ctx, jobs.NewJob{Type: fakeEvent, Priority: prio, IdempotencyKey: "q" + string(rune('a'+i))}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := jf.queue.Enqueue(ctx, jobs.NewJob{Type: fakeEvent, Priority: 9, Delay: time.Hour, IdempotencyKey: "planned"}); err != nil {
		t.Fatal(err)
	}
	if _, err := jf.queue.Enqueue(ctx, jobs.NewJob{Type: fakeEvent, Priority: 9, IdempotencyKey: "running"}); err != nil {
		t.Fatal(err)
	}
	running, err := jf.queue.Claim(ctx, "w", []string{fakeEvent})
	if err != nil || running == nil || running.IdempotencyKey != "running" {
		t.Fatalf("claim: %+v %v", running, err)
	}
	page, err := jf.svc.ListJobRuns(ctx, fakeEvent, "queued", 2)
	if err != nil || page.Total != 3 || len(page.Runs) != 2 || page.Runs[0].Priority != 5 || page.Runs[1].Priority != 3 {
		t.Errorf("queued page = %+v, %v; want total 3, the two highest priorities first", page, err)
	}
	if page, err := jf.svc.ListJobRuns(ctx, fakeEvent, "planned", 0); err != nil || page.Total != 1 || len(page.Runs) != 1 || page.Runs[0].RunAfter.IsZero() {
		t.Errorf("planned page = %+v, %v", page, err)
	}
	if page, err := jf.svc.ListJobRuns(ctx, fakeEvent, "running", 0); err != nil || page.Total != 1 || page.Runs[0].ID != running.ID {
		t.Errorf("running page = %+v, %v", page, err)
	}
	var verr *model.ValidationError
	if _, err := jf.svc.ListJobRuns(ctx, fakeEvent, "held", 0); !errors.As(err, &verr) {
		t.Errorf("unknown state: %v", err)
	}
	if _, err := jf.svc.ListJobRuns(ctx, "nobody.home", "queued", 0); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown type: %v", err)
	}
	n, err := jf.svc.CancelJobRuns(ctx, fakeEvent, false)
	if err != nil || n != 4 {
		t.Fatalf("cancel queued = %d, %v; want 4 (three waiting and the planned one)", n, err)
	}
	if j, _ := jf.queue.Get(ctx, running.ID); j.State != jobs.StateRunning {
		t.Errorf("running run cancelled by the queued scope: %+v", j)
	}
	if n, err := jf.svc.CancelJobRuns(ctx, fakeEvent, true); err != nil || n != 1 {
		t.Errorf("cancel all = %d, %v; want the running one", n, err)
	}
	if page, err := jf.svc.ListJobRuns(ctx, fakeEvent, "cancelled", 0); err != nil || page.Total != 5 {
		t.Errorf("cancelled page = %+v, %v", page, err)
	}
	if _, err := jf.svc.CancelJobRuns(ctx, "nobody.home", false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("cancel unknown: %v", err)
	}
}
