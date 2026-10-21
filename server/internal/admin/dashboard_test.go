package admin

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

type fakeChannels struct {
	avail map[string]model.ChannelAvail
}

func (f fakeChannels) ChannelAvailability(context.Context) (map[string]model.ChannelAvail, error) {
	return f.avail, nil
}

// A-3: a job that failed finally appears in recentFailed with its error redacted, its retry queues
// it again, an error of the log appears in errors, and the storage and integrations are filled.
func TestDashboardJobsErrorsAndStorage(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	queue := jobs.NewQueue(f.db.Write, nil)
	var logs bytes.Buffer
	log, ring, err := logging.NewWithErrors(&logs, logging.FormatJSON, slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	src := fakeSystem(t, false, false, "", "524288")
	src.ConfigDir, src.CacheDir = t.TempDir(), t.TempDir()
	f.svc.WithJobs(queue).WithErrors(ring).WithSystem(src)

	if _, err := f.svc.Dashboard(ctx); err != nil {
		t.Fatalf("empty dashboard: %v", err)
	}
	id, err := queue.Enqueue(ctx, jobs.NewJob{Type: "metadata.match", MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	j, err := queue.Claim(ctx, "w1", []string{"metadata.match"})
	if err != nil || j == nil {
		t.Fatalf("claim: %v, %v", j, err)
	}
	if err := queue.Fail(ctx, "w1", j, errors.New(`Get "https://api.themoviedb.org/3/movie/1?api_key=dash-FAKE-key": timeout`)); err != nil {
		t.Fatal(err)
	}
	log.With("component", "metadata").Error("tmdb unreachable", "err", errors.New("dial tcp: timeout"), "token", "dash-FAKE-token")

	d, err := f.svc.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.Jobs.Failed != 1 || d.Jobs.Queued != 0 || len(d.Jobs.RecentFailed) != 1 {
		t.Fatalf("jobs = %+v, want one failed", d.Jobs)
	}
	failed := d.Jobs.RecentFailed[0]
	if failed.ID != id || failed.Type != "metadata.match" || failed.State != "failed" || failed.Attempts != 1 ||
		!strings.Contains(failed.LastError, "api_key=[redacted]") || failed.UpdatedAt.IsZero() {
		t.Errorf("failed job = %+v", failed)
	}
	if len(d.Errors) != 1 || d.Errors[0].Source != "metadata" || d.Errors[0].Message != "tmdb unreachable: dial tcp: timeout" {
		t.Errorf("errors = %+v, want the logged error", d.Errors)
	}
	if d.Config.Path != src.ConfigDir || d.Config.TotalBytes <= 0 || d.Config.FreeBytes <= 0 || d.Cache.Path != src.CacheDir {
		t.Errorf("storage = %+v / %+v", d.Config, d.Cache)
	}
	if !d.Update.Enabled || d.Update.CurrentVersion != "dev" {
		t.Errorf("update = %+v", d.Update)
	}
	for _, secret := range []string{"dash-FAKE-key", "dash-FAKE-token"} {
		if strings.Contains(failed.LastError+d.Errors[0].Message, secret) {
			t.Errorf("dashboard carries %q", secret)
		}
	}

	retried, err := f.svc.RetryJob(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if retried.ID != id || retried.State != "queued" || retried.Attempts != 0 {
		t.Errorf("retried = %+v, want queued with attempts 0", retried)
	}
	if d, err = f.svc.Dashboard(ctx); err != nil || d.Jobs.Queued != 1 || d.Jobs.Failed != 0 || len(d.Jobs.RecentFailed) != 0 {
		t.Errorf("after the retry: %+v, %v", d.Jobs, err)
	}
	if _, err := f.svc.RetryJob(ctx, id); !errors.Is(err, model.ErrJobNotFailed) {
		t.Errorf("retry of a queued job: %v, want ErrJobNotFailed", err)
	}
	if err := f.svc.CancelJob(ctx, id); err != nil {
		t.Errorf("cancel: %v", err)
	}
	if err := f.svc.CancelJob(ctx, id); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("cancel of a cancelled job: %v, want ErrNotFound", err)
	}
	// "Abbrechen" on a failed job dismisses it (review I-5).
	failedID, err := queue.Enqueue(ctx, jobs.NewJob{Type: "metadata.match", MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}
	if j, err = queue.Claim(ctx, "w1", []string{"metadata.match"}); err != nil || j == nil || j.ID != failedID {
		t.Fatalf("claim: %v, %v", j, err)
	}
	if err := queue.Fail(ctx, "w1", j, errors.New("boom")); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.CancelJob(ctx, failedID); err != nil {
		t.Errorf("cancel of a failed job: %v", err)
	}
	if d, err = f.svc.Dashboard(ctx); err != nil || d.Jobs.Failed != 0 || len(d.Jobs.RecentFailed) != 0 {
		t.Errorf("after dismissing the failed job: %+v, %v", d.Jobs, err)
	}
	if _, err := f.svc.RetryJob(ctx, 9999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("retry of an unknown job: %v, want ErrNotFound", err)
	}
	if _, err := newFixture(t).svc.Dashboard(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("dashboard without jobs: %v, want ErrNotConfigured", err)
	}
}

// The integrations: keys by source, channels from notify or, without it, from the configuration,
// the update check by its switch.
func TestDashboardIntegrations(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.env[EnvYouTubeKey] = "yt-FAKE-env"
	f.svc.WithJobs(jobs.NewQueue(f.db.Write, nil))
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{Keys: model.AdminKeysUpdate{TMDB: ptr(fakeKey)}}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetVAPIDKeys(ctx, "BPub-FAKE", "priv-FAKE-vapid"); err != nil {
		t.Fatal(err)
	}
	states := func() map[string]model.IntegrationStatus {
		t.Helper()
		d, err := f.svc.Dashboard(ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]model.IntegrationStatus{}
		var keys []string
		for _, i := range d.Integrations {
			out[i.Key] = i
			keys = append(keys, i.Key)
		}
		if got := strings.Join(keys, ","); got != "tmdb,tvdb,youtube,trakt,email,push,telegram,updateCheck" {
			t.Fatalf("integration keys = %s", got)
		}
		return out
	}
	st := states()
	for key, want := range map[string]string{
		"tmdb": "configured", "tvdb": "missing", "youtube": "configured", "trakt": "missing",
		"email": "missing", "push": "configured", "telegram": "missing", "updateCheck": "configured",
	} {
		if st[key].State != want {
			t.Errorf("%s = %+v, want %s", key, st[key], want)
		}
	}

	attempt := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	f.svc.WithTrakt(&fakeTrakt{status: model.TraktStatus{Configured: true, LastAttempt: &attempt, LastError: "trending movie: status 503"}})
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{Keys: model.AdminKeysUpdate{Trakt: ptr("FAKE-trakt-dash-id")}}); err != nil {
		t.Fatal(err)
	}
	f.svc.WithChannels(fakeChannels{avail: map[string]model.ChannelAvail{
		model.ChannelEmail:    {Configured: true, Detail: "535 authentication failed"},
		model.ChannelTelegram: {Configured: true},
	}})
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{UpdateCheckEnabled: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	st = states()
	if e := st["email"]; e.State != "error" || e.Detail != "535 authentication failed" {
		t.Errorf("email = %+v, want error with the detail", e)
	}
	if st["telegram"].State != "configured" || st["push"].State != "missing" || st["updateCheck"].State != "disabled" {
		t.Errorf("telegram %+v, push %+v, update %+v", st["telegram"], st["push"], st["updateCheck"])
	}
	if tr := st["trakt"]; tr.State != "error" || tr.Detail != "trending movie: status 503" || tr.CheckedAt == nil || !tr.CheckedAt.Equal(attempt) {
		t.Errorf("trakt = %+v, want the error of the last attempt", tr)
	}
}
