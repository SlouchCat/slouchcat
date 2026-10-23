package integrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// RequestsPerSecond is the rate one integration may send at (plan 3.8 point 8). A burst of this
// size is allowed, because an import of a season fires several webhooks at once.
const RequestsPerSecond = 10

// actingEvents are the events that change files on disk and therefore trigger a scan (5.6).
// Radarr and Sonarr spell them exactly like this; the comparison ignores case so a later spelling
// still matches.
var actingEvents = map[string]bool{
	"download": true, // import and upgrade
	"rename":   true,
	// Deletions: the scan of the same directory notices the missing file.
	"moviefiledelete":   true,
	"episodefiledelete": true,
	"moviedelete":       true,
	"seriesdelete":      true,
}

// testEvent is the event the test button of Radarr and Sonarr sends.
const testEvent = "test"

// Receive handles one webhook: it authenticates the token, enforces the rate limit, records the
// call in the event log and queues a scan.path job when the event changed files. It returns
// model.ErrWebhookToken (401), model.ErrWebhookRateLimited (429), model.ErrWebhookPathOutside (422) or a validation error (422);
// every other event is accepted and ignored.
func (s *Service) Receive(ctx context.Context, token string, p model.WebhookPayload) (model.WebhookReceipt, error) {
	integ, err := s.Authenticate(ctx, token)
	if err != nil {
		return model.WebhookReceipt{}, err
	}
	if !s.limits.allow(integ.ID) {
		return model.WebhookReceipt{}, model.ErrWebhookRateLimited
	}
	if err := s.write.TouchWebhookIntegration(ctx, appq.TouchWebhookIntegrationParams{
		ID: integ.ID, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true},
	}); err != nil {
		return model.WebhookReceipt{}, fmt.Errorf("touch integration: %w", err)
	}

	event := strings.TrimSpace(p.Event)
	switch key := strings.ToLower(event); {
	case key == testEvent:
		s.record(ctx, integ.ID, event, "", "", model.ResultAcknowledged)
		return model.WebhookReceipt{Result: model.ResultAcknowledged}, nil
	case !actingEvents[key]:
		s.record(ctx, integ.ID, event, "", "", model.ResultIgnored)
		return model.WebhookReceipt{Result: model.ResultIgnored}, nil
	}

	reported := p.MoviePath
	field := "movie.folderPath"
	if integ.Kind == model.IntegrationSonarr {
		reported, field = p.SeriesPath, "series.path"
	}
	if strings.TrimSpace(reported) == "" {
		// An acting event of this kind always carries its directory; without one there is
		// nothing to scan, and a Sonarr body on a Radarr address lands here as well.
		s.record(ctx, integ.ID, event, "", "", model.ResultInvalidPayload)
		return model.WebhookReceipt{}, (&model.ValidationError{}).Add(field, "required")
	}

	target, err := s.Resolve(ctx, integ.PathMap, reported)
	if errors.Is(err, model.ErrWebhookPathOutside) {
		s.record(ctx, integ.ID, event, reported, target.Abs, model.ResultOutsideRoots)
		return model.WebhookReceipt{}, model.ErrWebhookPathOutside
	}
	if err != nil {
		return model.WebhookReceipt{}, err
	}
	// The log says what happened, so the entry is written only once the job is really queued.
	// A failure answers 500 and leaves no entry: Radarr and Sonarr retry, and the administration
	// is not told about a scan that never ran.
	if err := s.scan.RequestPathScan(ctx, target.LibraryID, target.RootID, []string{target.Dir}); err != nil {
		return model.WebhookReceipt{}, fmt.Errorf("queue path scan: %w", err)
	}
	s.record(ctx, integ.ID, event, reported, target.Abs, model.ResultScanQueued)
	return model.WebhookReceipt{Result: model.ResultScanQueued, Path: target.Abs}, nil
}

// record writes one entry of the event log. A failure must not fail the webhook: Radarr would
// retry an import that already queued its scan, so the error is logged and the call goes on.
func (s *Service) record(ctx context.Context, integrationID int64, event, reported, resolved, result string) {
	// A webhook that queued a scan must appear in the log even when the caller hung up, so the
	// insert outlives the request context.
	ctx = context.WithoutCancel(ctx)
	if _, err := s.write.InsertWebhookEvent(ctx, appq.InsertWebhookEventParams{
		IntegrationID: integrationID, Now: s.now().UnixMilli(), Event: truncateEvent(event),
		ReportedPath: nullString(reported), ResolvedPath: nullString(resolved), Result: result,
	}); err != nil {
		s.log.ErrorContext(ctx, "record webhook event", "integration_id", integrationID,
			"event", event, "result", result, "err", err)
	}
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// limiters holds one rate limiter per integration (3.8 point 8), created on first use. They run on
// the service's clock, so a test can hold the second still while it sends its requests.
type limiters struct {
	mu  sync.Mutex
	by  map[int64]*ratelimit.Limiter
	now func() time.Time
}

func newLimiters(now func() time.Time) *limiters {
	return &limiters{by: map[int64]*ratelimit.Limiter{}, now: now}
}

func (l *limiters) allow(id int64) bool {
	l.mu.Lock()
	lim := l.by[id]
	if lim == nil {
		lim = ratelimit.New(fmt.Sprintf("webhook:%d", id), RequestsPerSecond, RequestsPerSecond,
			ratelimit.WithClock(l.now))
		l.by[id] = lim
	}
	l.mu.Unlock()
	return lim.Allow()
}

// drop forgets the limiter of a deleted integration, so the map does not grow with every
// integration the administration ever had.
func (l *limiters) drop(id int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.by, id)
}

// maxEventName is the width of webhook_events.event (migration 00017). A service that sends a
// longer name keeps its log entry, shortened, instead of losing it to the CHECK constraint.
const maxEventName = 80

func truncateEvent(event string) string {
	if len(event) <= maxEventName {
		return event
	}
	runes := []rune(event)
	if len(runes) > maxEventName {
		runes = runes[:maxEventName]
	}
	return string(runes)
}
