package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// fakeIntegrations answers the dashboard tile from a fixed list.
type fakeIntegrations struct {
	list   []model.Integration
	since  time.Time
	events int
}

func (f *fakeIntegrations) List(context.Context) ([]model.Integration, error) { return f.list, nil }
func (f *fakeIntegrations) Get(context.Context, int64) (model.Integration, error) {
	return model.Integration{}, model.ErrNotFound
}

func (f *fakeIntegrations) Create(context.Context, string, string) (model.Integration, error) {
	return model.Integration{}, nil
}

func (f *fakeIntegrations) RenewToken(context.Context, int64) (model.Integration, error) {
	return model.Integration{}, nil
}

func (f *fakeIntegrations) SetPathMap(context.Context, int64, []model.PathMapping) (model.Integration, error) {
	return model.Integration{}, nil
}
func (f *fakeIntegrations) Delete(context.Context, int64) error { return nil }
func (f *fakeIntegrations) Events(context.Context, int64) ([]model.IntegrationEvent, error) {
	return nil, nil
}

func (f *fakeIntegrations) EventsSince(_ context.Context, since time.Time) (int, error) {
	f.since = since
	return f.events, nil
}

func TestWebhookStatusCountsAndWarnsAboutSilentIntegrations(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	fake := &fakeIntegrations{events: 7, list: []model.Integration{
		// Reported an hour ago: fine.
		{Name: "Radarr", LastSeen: now.Add(-time.Hour), CreatedAt: now.Add(-30 * 24 * time.Hour)},
		// Last report eight days ago: silent.
		{Name: "Sonarr", LastSeen: now.Add(-8 * 24 * time.Hour), CreatedAt: now.Add(-30 * 24 * time.Hour)},
		// Never reported, created yesterday: not yet configured in Radarr, no warning.
		{Name: "Neu", CreatedAt: now.Add(-24 * time.Hour)},
		// Never reported, created a month ago: silent.
		{Name: "Vergessen", CreatedAt: now.Add(-30 * 24 * time.Hour)},
	}}
	s := &Service{webhooks: fake, now: func() time.Time { return now }}

	st, err := s.WebhookStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 4 {
		t.Errorf("total %d, want 4", st.Total)
	}
	if st.EventsLast24h != 7 {
		t.Errorf("events %d, want 7", st.EventsLast24h)
	}
	if want := now.Add(-24 * time.Hour); !fake.since.Equal(want) {
		t.Errorf("counted since %s, want %s", fake.since, want)
	}
	if len(st.Silent) != 2 || st.Silent[0] != "Sonarr" || st.Silent[1] != "Vergessen" {
		t.Errorf("silent %v, want [Sonarr Vergessen]", st.Silent)
	}
}

func TestIntegrationOperationsNeedTheService(t *testing.T) {
	s := &Service{now: time.Now}
	ctx := context.Background()
	if _, err := s.ListIntegrations(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("list: %v", err)
	}
	if _, err := s.CreateIntegration(ctx, "Radarr", model.IntegrationRadarr); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("create: %v", err)
	}
	if err := s.DeleteIntegration(ctx, 1); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("delete: %v", err)
	}
	if _, err := s.WebhookStatus(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("status: %v", err)
	}
}
