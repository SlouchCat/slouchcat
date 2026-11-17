package integrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The Seerr webhook (concept 25 stage 4, build step 4.5, F-18; architecture 10). Seerr calls the
// same receiver as Radarr and Sonarr, with the token of an integration of the kind seerr in the
// path. A notification is only a trigger: it queues a run of the sync wishes.sync and nothing of
// its body is read for a state (it carries no user id, no request status, no signature, and Seerr
// never repeats it, appendix D 9.2, 9.3). The sync every 5 minutes stays the base.

// SeerrSync queues a run of the sync with Seerr. Package wishes implements it; queued is false
// while Seerr is not set up or switched off, and an error means nothing was queued.
type SeerrSync interface {
	SyncFromWebhook(ctx context.Context) (queued bool, err error)
}

// WithSeerrSync sets the sync a Seerr notification triggers.
func WithSeerrSync(s SeerrSync) Option {
	return func(svc *Service) { svc.seerr = s }
}

// seerrTestEvent is notification_type of the test button of Seerr.
const seerrTestEvent = "test_notification"

// seerrEventName is the log entry of a notification without a type.
const seerrEventName = "seerr"

// receiveSeerr handles a notification of Seerr: the test is acknowledged, every other
// notification queues the sync (sync_queued), or is ignored while Seerr is not set up. A failure to
// queue answers an error and leaves no entry, like a failed scan of Radarr and Sonarr.
func (s *Service) receiveSeerr(ctx context.Context, integ model.Integration, p model.WebhookPayload) (model.WebhookReceipt, error) {
	event := strings.TrimSpace(p.Notification)
	if event == "" {
		event = strings.TrimSpace(p.Event)
	}
	if event == "" {
		event = seerrEventName
	}
	if strings.EqualFold(event, seerrTestEvent) {
		s.record(ctx, integ.ID, event, "", "", model.ResultAcknowledged)
		return model.WebhookReceipt{Result: model.ResultAcknowledged}, nil
	}
	if s.seerr == nil {
		s.record(ctx, integ.ID, event, "", "", model.ResultIgnored)
		return model.WebhookReceipt{Result: model.ResultIgnored}, nil
	}
	queued, err := s.seerr.SyncFromWebhook(ctx)
	if err != nil {
		return model.WebhookReceipt{}, fmt.Errorf("queue wish sync: %w", err)
	}
	if !queued {
		s.record(ctx, integ.ID, event, "", "", model.ResultIgnored)
		return model.WebhookReceipt{Result: model.ResultIgnored}, nil
	}
	s.record(ctx, integ.ID, event, "", "", model.ResultSyncQueued)
	s.log.InfoContext(ctx, "seerr webhook queued the wish sync", "integration_id", integ.ID, "event", truncateEvent(event))
	return model.WebhookReceipt{Result: model.ResultSyncQueued}, nil
}

// Seerr's notification types (appendix D 9.1) that change a request: pending, approved, available,
// failed, declined and auto-approved. Issues and comments do not concern the wishes.
const (
	seerrMediaPending      = 2
	seerrMediaApproved     = 4
	seerrMediaAvailable    = 8
	seerrMediaFailed       = 16
	seerrMediaDeclined     = 64
	seerrMediaAutoApproved = 128
)

// seerrPayload is the JSON payload for Seerr. Slouchcat reads only notification_type (for the
// log); the template sends no name or address of a person.
const seerrPayload = `{
  "notification_type": "{{notification_type}}",
  "event": "{{event}}",
  "media_type": "{{media_type}}",
  "media_tmdbid": "{{media_tmdbid}}",
  "media_tvdbid": "{{media_tvdbid}}",
  "request_id": "{{request_id}}"
}`

// SeerrWebhookSetup returns the settings the administrator enters in Seerr (band «Sofortmeldung»).
// The address is the one shown once when the integration of kind seerr is created or renewed.
func SeerrWebhookSetup() model.SeerrWebhookSetup {
	return model.SeerrWebhookSetup{
		JSONPayload: seerrPayload,
		Types:       seerrMediaPending | seerrMediaApproved | seerrMediaAvailable | seerrMediaFailed | seerrMediaDeclined | seerrMediaAutoApproved,
		TypeNames:   []string{"MEDIA_PENDING", "MEDIA_APPROVED", "MEDIA_AVAILABLE", "MEDIA_FAILED", "MEDIA_DECLINED", "MEDIA_AUTO_APPROVED"},
	}
}
