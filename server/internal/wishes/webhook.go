package wishes

import (
	"context"
	"errors"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
)

// The Seerr webhook as a trigger (concept 25 stage 4, build step 4.5, F-18; architecture 10 and
// 13): a notification of Seerr queues a run of wishes.sync with the manual priority, or raises the
// waiting one. The job waits for the debounce window of the queue under the key of the regular
// sync, so a burst of notifications (an approval sends several) becomes one run. The content of
// the notification is never read: the sync reads the state from Seerr. The sync every 5 minutes
// stays the base.

// webhookPriority is the priority of a run the webhook queues (architecture 13: manual and webhook 7).
const webhookPriority = jobs.ManualPriority

// SyncFromWebhook queues the sync for a notification of the Seerr webhook (integrations.SeerrSync).
// queued is false while Seerr is not set up or switched off; then nothing is queued.
func (s *Service) SyncFromWebhook(ctx context.Context) (bool, error) {
	cfg, err := s.cfg.SeerrConfig(ctx)
	if err != nil {
		return false, err
	}
	if !cfg.Configured() || !cfg.Enabled {
		return false, nil
	}
	if s.queue == nil {
		return false, errors.New("wishes: no job queue")
	}
	id, err := s.queue.Enqueue(ctx, jobs.NewJob{Type: JobSync, Priority: webhookPriority, IdempotencyKey: JobSync,
		Payload: []byte(`{}`), Delay: jobs.DebounceWindow})
	if err != nil {
		return false, fmt.Errorf("enqueue %s: %w", JobSync, err)
	}
	if _, err := s.queue.RaiseQueuedPriority(ctx, id, webhookPriority); err != nil {
		return false, fmt.Errorf("raise %s: %w", JobSync, err)
	}
	return true, nil
}
