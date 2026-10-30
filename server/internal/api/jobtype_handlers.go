package api

import (
	"context"
	"net/http"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The page "Hintergrundjobs" (concept 09, ADR 0017, step 3): seven admin operations on job types
// and runs. The admin service computes everything; these handlers map between the contract and
// the model.

func (e serviceProblem) VisitListJobTypesResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitListJobRunsResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitRunJobTypeResponse(w http.ResponseWriter) error    { return e.visit(w) }
func (e serviceProblem) VisitPauseJobTypeResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitResumeJobTypeResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitUpdateJobScheduleResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitCancelJobRunsResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) ListJobTypes(ctx context.Context, _ gen.ListJobTypesRequestObject) (gen.ListJobTypesResponseObject, error) {
	ov, err := s.opts.Admin.ListJobTypes(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.JobOverview{Paused: ov.Paused, Pools: make([]gen.JobPool, 0, len(ov.Pools)), Types: make([]gen.JobTypeInfo, 0, len(ov.Types))}
	for _, p := range ov.Pools {
		out.Pools = append(out.Pools, gen.JobPool{Pool: gen.JobPoolPool(p.Pool), Workers: p.Workers, BacklogSeconds: p.BacklogSeconds})
	}
	for _, t := range ov.Types {
		out.Types = append(out.Types, toJobTypeInfo(t))
	}
	return gen.ListJobTypes200JSONResponse(out), nil
}

func (s *server) ListJobRuns(ctx context.Context, req gen.ListJobRunsRequestObject) (gen.ListJobRunsResponseObject, error) {
	limit := 0
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	page, err := s.opts.Admin.ListJobRuns(ctx, req.Params.Type, string(req.Params.State), limit)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.JobRunPage{Total: page.Total, Runs: make([]gen.JobRun, 0, len(page.Runs))}
	for _, r := range page.Runs {
		out.Runs = append(out.Runs, toJobRun(r))
	}
	return gen.ListJobRuns200JSONResponse(out), nil
}

func (s *server) RunJobType(ctx context.Context, req gen.RunJobTypeRequestObject) (gen.RunJobTypeResponseObject, error) {
	runs, err := s.opts.Admin.RunJobType(ctx, req.Type)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.RunJobType202JSONResponse, 0, len(runs))
	for _, r := range runs {
		out = append(out, toJobRun(r))
	}
	return out, nil
}

func (s *server) PauseJobType(ctx context.Context, req gen.PauseJobTypeRequestObject) (gen.PauseJobTypeResponseObject, error) {
	info, err := s.opts.Admin.PauseJobType(ctx, req.Type)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.PauseJobType200JSONResponse(toJobTypeInfo(info)), nil
}

func (s *server) ResumeJobType(ctx context.Context, req gen.ResumeJobTypeRequestObject) (gen.ResumeJobTypeResponseObject, error) {
	info, err := s.opts.Admin.ResumeJobType(ctx, req.Type)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ResumeJobType200JSONResponse(toJobTypeInfo(info)), nil
}

func (s *server) UpdateJobSchedule(ctx context.Context, req gen.UpdateJobScheduleRequestObject) (gen.UpdateJobScheduleResponseObject, error) {
	body := deref(req.Body)
	upd := model.JobScheduleUpdate{EverySeconds: body.EverySeconds, Enabled: body.Enabled}
	if body.ClearWindow != nil && *body.ClearWindow {
		upd.ClearWindow = true
	} else if body.Window != nil {
		upd.Window = &model.JobWindow{StartMinute: body.Window.StartMinute, EndMinute: body.Window.EndMinute}
	}
	info, err := s.opts.Admin.UpdateJobSchedule(ctx, req.Type, upd)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.UpdateJobSchedule200JSONResponse(toJobTypeInfo(info)), nil
}

func (s *server) CancelJobRuns(ctx context.Context, req gen.CancelJobRunsRequestObject) (gen.CancelJobRunsResponseObject, error) {
	all := req.Params.Scope != nil && *req.Params.Scope == gen.CancelJobRunsParamsScopeAll
	n, err := s.opts.Admin.CancelJobRuns(ctx, req.Type, all)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CancelJobRuns200JSONResponse(gen.JobCancelResult{Cancelled: n}), nil
}

func toJobTypeInfo(t model.JobTypeInfo) gen.JobTypeInfo {
	out := gen.JobTypeInfo{
		Type: t.Type, Group: gen.JobTypeInfoGroup(t.Group), Trigger: gen.JobTypeInfoTrigger(t.Trigger),
		Pool: gen.JobTypeInfoPool(t.Pool), Paused: t.Paused, Stopping: t.Stopping, Runnable: t.Runnable, Known: t.Known,
		MedianDurationSeconds: t.MedianSeconds, QueuedEstimateSeconds: t.QueuedEstimateSeconds,
		Running: make([]gen.JobRun, 0, len(t.RunningRuns)),
	}
	out.Counts.Queued, out.Counts.Planned, out.Counts.Running, out.Counts.Failed = t.Queued, t.Planned, t.Running, t.Failed
	if !t.PausedAt.IsZero() {
		at := t.PausedAt.UTC()
		out.PausedAt = &at
	}
	if sc := t.Schedule; sc != nil {
		g := gen.JobSchedule{EverySeconds: sc.EverySeconds, NextRunAt: sc.NextRunAt.UTC(), Enabled: sc.Enabled, TimeZone: sc.TimeZone}
		g.Limits.MinEverySeconds, g.Limits.MaxEverySeconds = sc.MinEverySeconds, sc.MaxEverySeconds
		if !sc.LastFiredAt.IsZero() {
			at := sc.LastFiredAt.UTC()
			g.LastFiredAt = &at
		}
		if sc.Window != nil {
			g.Window = &gen.JobWindow{StartMinute: sc.Window.StartMinute, EndMinute: sc.Window.EndMinute}
		}
		out.Schedule = &g
	}
	for _, r := range t.RunningRuns {
		out.Running = append(out.Running, toJobRun(r))
	}
	if t.LastRun != nil {
		last := toJobRun(*t.LastRun)
		out.LastRun = &last
	}
	return out
}

func toJobRun(r model.JobRun) gen.JobRun {
	out := gen.JobRun{
		Id: r.ID, Type: r.Type, State: gen.JobRunState(r.State), Priority: r.Priority,
		Attempts: r.Attempts, MaxAttempts: r.MaxAttempts, LastError: optString(r.LastError), UpdatedAt: r.UpdatedAt.UTC(),
		Subject: optString(r.Subject), Skipped: r.Skipped, EstimatedSecondsLeft: r.EstimatedSecondsLeft,
		Estimate: gen.JobRunEstimate(r.Estimate),
	}
	out.CreatedAt, out.RunAfter, out.StartedAt, out.FinishedAt = optTime(r.CreatedAt), optTime(r.RunAfter), optTime(r.StartedAt), optTime(r.FinishedAt)
	if r.HasDuration {
		d := int(r.Duration.Seconds())
		out.DurationSeconds = &d
	}
	if r.Progress != nil {
		out.Progress = &gen.JobProgress{Done: int(r.Progress.Done), Total: int(r.Progress.Total), Note: optString(r.Progress.Note)}
	}
	return out
}

// optTime returns t in UTC, or nil for the zero time.
func optTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}
