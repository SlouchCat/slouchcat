package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M8 wizard checks, system state, dashboard, job administration and update check (plan M8, A-1,
// A-3, A-5; design section 1). The admin service does the work; these handlers map between the
// contract and the model.

func (e serviceProblem) VisitCheckSetupPathResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitCheckSetupTmdbKeyResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitGetSystemInfoResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitGetDashboardResponse(w http.ResponseWriter) error   { return e.visit(w) }
func (e serviceProblem) VisitRetryJobResponse(w http.ResponseWriter) error       { return e.visit(w) }
func (e serviceProblem) VisitCancelJobResponse(w http.ResponseWriter) error      { return e.visit(w) }
func (e serviceProblem) VisitGetUpdateStateResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitCheckForUpdateResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) CheckSetupPath(ctx context.Context, req gen.CheckSetupPathRequestObject) (gen.CheckSetupPathResponseObject, error) {
	c, err := s.opts.Admin.CheckSetupPath(model.WithClientIP(ctx, s.clientAddr(ctx)), deref(req.Body).Path)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.CheckSetupPath200JSONResponse{Ok: c.OK, FsType: optString(c.FSType)}
	if c.WatchMode != "" {
		m := gen.PathCheckWatchMode(c.WatchMode)
		out.WatchMode = &m
	}
	if c.Warning != "" {
		w := gen.PathCheckWarning(c.Warning)
		out.Warning = &w
	}
	if c.Problem != "" {
		p := gen.PathCheckProblem(c.Problem)
		out.Problem = &p
	}
	return out, nil
}

func (s *server) CheckSetupTmdbKey(ctx context.Context, req gen.CheckSetupTmdbKeyRequestObject) (gen.CheckSetupTmdbKeyResponseObject, error) {
	c, err := s.opts.Admin.CheckSetupTMDBKey(model.WithClientIP(ctx, s.clientAddr(ctx)), deref(deref(req.Body).TmdbApiKey))
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.CheckSetupTmdbKey200JSONResponse{Ok: c.OK}
	if c.Problem != "" {
		p := gen.TMDBCheckProblem(c.Problem)
		out.Problem = &p
	}
	return out, nil
}

func (s *server) GetSystemInfo(ctx context.Context, _ gen.GetSystemInfoRequestObject) (gen.GetSystemInfoResponseObject, error) {
	info, err := s.opts.Admin.SystemInfo(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetSystemInfo200JSONResponse(toSystemInfo(info)), nil
}

func (s *server) GetDashboard(ctx context.Context, _ gen.GetDashboardRequestObject) (gen.GetDashboardResponseObject, error) {
	d, err := s.opts.Admin.Dashboard(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := gen.Dashboard{
		Streams:      make([]gen.AdminStream, 0, len(d.Streams)),
		Jobs:         gen.JobCounts{Queued: d.Jobs.Queued, Running: d.Jobs.Running, Failed: d.Jobs.Failed},
		Integrations: make([]gen.IntegrationStatus, 0, len(d.Integrations)),
		Errors:       make([]gen.ErrorEntry, 0, len(d.Errors)),
		Update:       toUpdateState(d.Update),
	}
	for _, st := range d.Streams {
		out.Streams = append(out.Streams, toAdminStream(st))
	}
	out.Jobs.RecentFailed = make([]gen.JobSummary, 0, len(d.Jobs.RecentFailed))
	for _, j := range d.Jobs.RecentFailed {
		out.Jobs.RecentFailed = append(out.Jobs.RecentFailed, toJobSummary(j))
	}
	out.Storage.Config = gen.StorageInfo{Path: d.Config.Path, TotalBytes: d.Config.TotalBytes, FreeBytes: d.Config.FreeBytes}
	out.Storage.Cache = gen.StorageInfo{Path: d.Cache.Path, TotalBytes: d.Cache.TotalBytes, FreeBytes: d.Cache.FreeBytes}
	for _, i := range d.Integrations {
		out.Integrations = append(out.Integrations, gen.IntegrationStatus{
			Key: gen.IntegrationStatusKey(i.Key), State: gen.IntegrationStatusState(i.State), Detail: optString(i.Detail),
			CheckedAt: utc(i.CheckedAt),
		})
	}
	for _, e := range d.Errors {
		entry := gen.ErrorEntry{At: e.At.UTC(), Source: e.Source, Message: e.Message, JobType: optString(e.JobType)}
		if e.JobID != 0 {
			id := e.JobID
			entry.JobId = &id
		}
		out.Errors = append(out.Errors, entry)
	}
	return gen.GetDashboard200JSONResponse(out), nil
}

func (s *server) RetryJob(ctx context.Context, req gen.RetryJobRequestObject) (gen.RetryJobResponseObject, error) {
	j, err := s.opts.Admin.RetryJob(ctx, req.Id)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RetryJob200JSONResponse(toJobSummary(j)), nil
}

func (s *server) CancelJob(ctx context.Context, req gen.CancelJobRequestObject) (gen.CancelJobResponseObject, error) {
	if err := s.opts.Admin.CancelJob(ctx, req.Id); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CancelJob204Response{}, nil
}

func (s *server) GetUpdateState(ctx context.Context, _ gen.GetUpdateStateRequestObject) (gen.GetUpdateStateResponseObject, error) {
	u, err := s.opts.Admin.UpdateState(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetUpdateState200JSONResponse(toUpdateState(u)), nil
}

func (s *server) CheckForUpdate(ctx context.Context, _ gen.CheckForUpdateRequestObject) (gen.CheckForUpdateResponseObject, error) {
	u, err := s.opts.Admin.CheckForUpdate(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CheckForUpdate200JSONResponse(toUpdateState(u)), nil
}

// toSystemInfo maps the system state for getSystemInfo and the wizard (getSetupStatus.system).
func toSystemInfo(info model.SystemInfo) gen.SystemInfo {
	g := info.GPU
	out := gen.SystemInfo{
		Version: info.Version, StartedAt: info.StartedAt.UTC(), ConfigFs: info.ConfigFS, CacheFs: info.CacheFS,
		Gpu: gen.GPUInfo{
			Backend: gen.GPUInfoBackend(g.Backend), HardwareEncode: g.HardwareEncode, Tonemapper: gen.GPUInfoTonemapper(g.Tonemapper),
			HwTonemap: g.HWTonemap, FfmpegVersion: optString(g.FFmpegVersion),
		},
		Inotify: gen.InotifyInfo{
			MaxUserWatches: info.Inotify.MaxUserWatches, WatchedDirs: info.Inotify.WatchedDirs,
			UsagePercent: info.Inotify.UsagePercent, Warn: info.Inotify.Warn,
		},
	}
	out.Gpu.Devices.Dri, out.Gpu.Devices.Nvidia = g.DeviceDRI, g.DeviceNvidia
	if g.ComposeOverride != "" {
		o := gen.GPUInfoComposeOverride(g.ComposeOverride)
		out.Gpu.ComposeOverride = &o
	}
	return out
}

func toJobSummary(j model.JobSummary) gen.JobSummary {
	out := gen.JobSummary{
		Id: j.ID, Type: j.Type, State: gen.JobSummaryState(j.State), Attempts: j.Attempts, MaxAttempts: j.MaxAttempts,
		LastError: optString(j.LastError), UpdatedAt: j.UpdatedAt.UTC(),
	}
	if !j.CreatedAt.IsZero() {
		c := j.CreatedAt.UTC()
		out.CreatedAt = &c
	}
	return out
}

func toUpdateState(u model.UpdateState) gen.UpdateState {
	return gen.UpdateState{
		Enabled: u.Enabled, CurrentVersion: u.CurrentVersion, LatestVersion: optString(u.LatestVersion), Available: u.Available,
		Url: optString(u.URL), Notes: optString(u.Notes), CheckedAt: utc(u.CheckedAt), Error: optString(u.Error),
	}
}
