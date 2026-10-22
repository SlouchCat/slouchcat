package admin

import (
	"context"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Integrations administers the incoming Radarr/Sonarr webhooks (M9, I-8; package integrations).
// Errors: *model.ValidationError and model.ErrNotFound.
type Integrations interface {
	List(ctx context.Context) ([]model.Integration, error)
	Get(ctx context.Context, id int64) (model.Integration, error)
	Create(ctx context.Context, name, kind string) (model.Integration, error)
	RenewToken(ctx context.Context, id int64) (model.Integration, error)
	SetPathMap(ctx context.Context, id int64, pairs []model.PathMapping) (model.Integration, error)
	Delete(ctx context.Context, id int64) error
	Events(ctx context.Context, id int64) ([]model.IntegrationEvent, error)
	// EventsSince counts the webhooks of every integration since a point in time (dashboard tile).
	EventsSince(ctx context.Context, since time.Time) (int, error)
}

// WithIntegrations sets the webhook integrations and returns the receiver.
func (s *Service) WithIntegrations(i Integrations) *Service {
	s.webhooks = i
	return s
}

// SilentAfter is the time without a report after which the dashboard warns about an integration
// (decision 5.6).
const SilentAfter = 7 * 24 * time.Hour

// ListIntegrations returns every webhook integration, oldest first, without tokens.
func (s *Service) ListIntegrations(ctx context.Context) ([]model.Integration, error) {
	if s.webhooks == nil {
		return nil, ErrNotConfigured
	}
	return s.webhooks.List(ctx)
}

// CreateIntegration adds an integration; the answer carries the address once (3.8 point 2).
func (s *Service) CreateIntegration(ctx context.Context, name, kind string) (model.Integration, error) {
	if s.webhooks == nil {
		return model.Integration{}, ErrNotConfigured
	}
	return s.webhooks.Create(ctx, name, kind)
}

// RenewIntegrationToken replaces the token and reveals the new address once; the old one stops
// working immediately.
func (s *Service) RenewIntegrationToken(ctx context.Context, id int64) (model.Integration, error) {
	if s.webhooks == nil {
		return model.Integration{}, ErrNotConfigured
	}
	return s.webhooks.RenewToken(ctx, id)
}

// SetIntegrationPathMap replaces the prefix pairs of an integration.
func (s *Service) SetIntegrationPathMap(ctx context.Context, id int64, pairs []model.PathMapping) (model.Integration, error) {
	if s.webhooks == nil {
		return model.Integration{}, ErrNotConfigured
	}
	return s.webhooks.SetPathMap(ctx, id, pairs)
}

// DeleteIntegration removes an integration and its event log.
func (s *Service) DeleteIntegration(ctx context.Context, id int64) error {
	if s.webhooks == nil {
		return ErrNotConfigured
	}
	return s.webhooks.Delete(ctx, id)
}

// IntegrationEvents returns the log of an integration, newest first (the newest 20).
func (s *Service) IntegrationEvents(ctx context.Context, id int64) ([]model.IntegrationEvent, error) {
	if s.webhooks == nil {
		return nil, ErrNotConfigured
	}
	return s.webhooks.Events(ctx, id)
}

// WebhookStatus is the dashboard tile "Webhooks" (A-3, decision 5.6): how many webhooks arrived
// in the last 24 hours and which integrations have reported nothing for SilentAfter.
func (s *Service) WebhookStatus(ctx context.Context) (model.WebhookStatus, error) {
	if s.webhooks == nil {
		return model.WebhookStatus{}, ErrNotConfigured
	}
	list, err := s.webhooks.List(ctx)
	if err != nil {
		return model.WebhookStatus{}, err
	}
	now := s.now()
	count, err := s.webhooks.EventsSince(ctx, now.Add(-24*time.Hour))
	if err != nil {
		return model.WebhookStatus{}, err
	}
	out := model.WebhookStatus{Total: len(list), EventsLast24h: count}
	for _, i := range list {
		// An integration that never reported is silent once it is older than the window: a fresh
		// one is simply not configured in Radarr yet.
		last := i.LastSeen
		if last.IsZero() {
			last = i.CreatedAt
		}
		if now.Sub(last) >= SilentAfter {
			out.Silent = append(out.Silent, i.Name)
		}
	}
	return out, nil
}
