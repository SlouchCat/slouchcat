package api

import (
	"context"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// System state, dashboard, job administration and update check (plan M8, A-1, A-3, A-5; design
// section 1). The admin service does the work; these handlers map between the
// contract and the model.

func (e serviceProblem) VisitGetSystemInfoResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitGetDashboardResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitDismissMinimumAgeNoticeResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitRetryJobResponse(w http.ResponseWriter) error       { return e.visit(w) }
func (e serviceProblem) VisitCancelJobResponse(w http.ResponseWriter) error      { return e.visit(w) }
func (e serviceProblem) VisitGetUpdateStateResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitCheckForUpdateResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitInstallUpdateResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitGetUpdateInstallStatusResponse(w http.ResponseWriter) error {
	return e.visit(w)
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
	paused := d.Jobs.Paused
	out := gen.Dashboard{
		Streams:      make([]gen.AdminStream, 0, len(d.Streams)),
		Jobs:         gen.JobCounts{Queued: d.Jobs.Queued, Running: d.Jobs.Running, Failed: d.Jobs.Failed, Paused: &paused},
		Integrations: make([]gen.IntegrationStatus, 0, len(d.Integrations)),
		Errors:       make([]gen.ErrorEntry, 0, len(d.Errors)),
		Update:       toUpdateState(d.Update),
		Webhooks: gen.WebhookTileStatus{
			Total: d.Webhooks.Total, EventsLast24h: d.Webhooks.EventsLast24h,
			Silent: append([]string{}, d.Webhooks.Silent...),
		},
		Browse: &gen.BrowseHealth{LostSlots: d.Browse.LostSlots, Slots: d.Browse.Slots},
		Attention: gen.DashboardAttention{
			AbortedScans:     make([]gen.AbortedScan, 0, len(d.Attention.AbortedScans)),
			RejectedKeys:     append([]string{}, d.Attention.RejectedKeys...),
			MinimumAgeNotice: trueOrNil(d.Attention.MinimumAgeNotice),
			RatingsRereading: trueOrNil(d.Attention.RatingsRereading),
			Wishes:           toDashboardWishes(d.Attention.Wishes),
		},
		Counts: gen.DashboardCounts{
			Titles: d.Counts.Titles, Accounts: d.Counts.Accounts, MaintenanceOpen: d.Counts.MaintenanceOpen,
		},
	}
	for _, a := range d.Attention.AbortedScans {
		out.Attention.AbortedScans = append(out.Attention.AbortedScans, gen.AbortedScan{LibraryId: a.LibraryID, Name: a.Name, Reason: a.Reason})
	}
	for _, st := range d.Streams {
		out.Streams = append(out.Streams, toAdminStream(st))
	}
	out.Jobs.RecentFailed = make([]gen.JobSummary, 0, len(d.Jobs.RecentFailed))
	for _, j := range d.Jobs.RecentFailed {
		out.Jobs.RecentFailed = append(out.Jobs.RecentFailed, toJobSummary(j))
	}
	out.Jobs.Active = make([]gen.JobRun, 0, len(d.Jobs.Active))
	for _, r := range d.Jobs.Active {
		out.Jobs.Active = append(out.Jobs.Active, toJobRun(r))
	}
	if n := d.Jobs.NextRun; n != nil {
		out.Jobs.NextRun = &gen.JobNextRun{Type: n.Type, At: n.At.UTC()}
	}
	out.Jobs.FailedByType = make([]gen.JobTypeFailures, 0, len(d.Jobs.FailedByType))
	for _, f := range d.Jobs.FailedByType {
		out.Jobs.FailedByType = append(out.Jobs.FailedByType, gen.JobTypeFailures{
			Type: f.Type, Count: f.Count, LastError: optString(f.LastError), LastFailedAt: f.LastFailedAt.UTC(),
		})
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
	out.Activity = make([]gen.ActivityEntry, 0, len(d.Activity))
	for _, a := range d.Activity {
		out.Activity = append(out.Activity, toActivityEntry(a))
	}
	return gen.GetDashboard200JSONResponse(out), nil
}

// toActivityEntry maps an entry of the activity log (concept 11); empty fields stay absent.
func toActivityEntry(a model.ActivityEntry) gen.ActivityEntry {
	d := a.Detail
	out := gen.ActivityEntry{
		Id: a.ID, At: a.At.UTC(), Kind: a.Kind, ProfileName: optString(a.ProfileName), AccountEmail: optString(a.AccountEmail),
		ItemTitle: optString(a.ItemTitle), LibraryName: optString(a.LibraryName),
		Detail: gen.ActivityDetail{
			NewTitles: d.NewTitles, Version: optString(d.Version), Trigger: optString(d.Trigger), Reason: optString(d.Reason),
			JobType: optString(d.JobType), By: optString(d.By), Mode: optString(d.Mode),
		},
	}
	if d.SizeBytes > 0 {
		size := d.SizeBytes
		out.Detail.SizeBytes = &size
	}
	if d.Count > 0 {
		count := d.Count
		out.Detail.Count = &count
	}
	return out
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

// InstallUpdate asks the updater companion to install the newest release (concept 15 P-8,
// ADR 0024): 202 with its state, 409 update_running, 422 with its code, 503 updater_unavailable.
func (s *server) InstallUpdate(ctx context.Context, _ gen.InstallUpdateRequestObject) (gen.InstallUpdateResponseObject, error) {
	in, err := s.opts.Admin.InstallUpdate(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.InstallUpdate202JSONResponse(toUpdateInstall(in)), nil
}

// GetUpdateInstallStatus reads the last install from the companion's state.json.
func (s *server) GetUpdateInstallStatus(ctx context.Context, _ gen.GetUpdateInstallStatusRequestObject) (gen.GetUpdateInstallStatusResponseObject, error) {
	in, err := s.opts.Admin.InstallStatus(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.GetUpdateInstallStatus200JSONResponse(toUpdateInstall(in)), nil
}

// toSystemInfo maps the system state for getSystemInfo and the wizard (getSetupStatus.system).
func toSystemInfo(info model.SystemInfo) gen.SystemInfo {
	g := info.GPU
	out := gen.SystemInfo{
		Version: info.Version, StartedAt: info.StartedAt.UTC(), ConfigFs: info.ConfigFS, CacheFs: info.CacheFS, TimeZone: info.TimeZone,
		Gpu: gen.GPUInfo{
			Backend: gen.TranscodeBackend(g.Backend), HardwareEncode: g.HardwareEncode, Tonemapper: gen.GPUInfoTonemapper(g.Tonemapper),
			HwTonemap: g.HWTonemap, FfmpegVersion: optString(g.FFmpegVersion), Candidates: make([]gen.TranscodeCandidate, 0, len(g.Candidates)),
		},
		Inotify: gen.InotifyInfo{
			MaxUserWatches: info.Inotify.MaxUserWatches, WatchedDirs: info.Inotify.WatchedDirs,
			UsagePercent: info.Inotify.UsagePercent, Warn: info.Inotify.Warn,
		},
	}
	out.Gpu.Devices.Dri, out.Gpu.Devices.Nvidia = g.DeviceDRI, g.DeviceNvidia
	for _, c := range g.Candidates {
		cand := gen.TranscodeCandidate{Backend: gen.TranscodeCandidateBackend(c.Backend), Ok: c.OK}
		if c.Reason != "" {
			r := gen.TranscodeCandidateReason(c.Reason)
			cand.Reason = &r
		}
		out.Gpu.Candidates = append(out.Gpu.Candidates, cand)
	}
	if g.FallbackFrom != "" {
		f := gen.TranscodeBackend(g.FallbackFrom)
		out.Gpu.FallbackFrom = &f
	}
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
	out := gen.UpdateState{
		Enabled: u.Enabled, CurrentVersion: u.CurrentVersion, LatestVersion: optString(u.LatestVersion),
		Development: u.Development, Available: u.Available,
		Url: optString(u.URL), Notes: optString(u.Notes), CheckedAt: utc(u.CheckedAt), Error: optString(u.Error),
		LastSuccessAt: utc(u.LastSuccessAt), UpdaterConfigured: u.UpdaterConfigured, UpdaterAvailable: u.UpdaterAvailable,
		UpdaterVersion: optString(u.UpdaterVersion),
	}
	if u.Install != nil {
		in := toUpdateInstall(*u.Install)
		out.Install = &in
	}
	return out
}

func toUpdateInstall(in model.UpdateInstall) gen.UpdateInstallStatus {
	return gen.UpdateInstallStatus{
		Phase: gen.UpdateInstallStatusPhase(in.Phase), From: optString(in.From), To: optString(in.To),
		StartedAt: utc(in.StartedAt), FinishedAt: utc(in.FinishedAt), Migrated: in.Migrated,
		Backup: optString(in.Backup), Code: optString(in.Code), Message: optString(in.Message),
	}
}

func (s *server) DismissMinimumAgeNotice(ctx context.Context, _ gen.DismissMinimumAgeNoticeRequestObject) (gen.DismissMinimumAgeNoticeResponseObject, error) {
	if err := s.opts.Admin.DismissMinimumAgeNotice(ctx); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DismissMinimumAgeNotice204Response{}, nil
}

// trueOrNil is a flag that the contract leaves out while it is false.
func trueOrNil(v bool) *bool {
	if !v {
		return nil
	}
	return &v
}
