package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// JobDeliver is the job type that sends one delivery; its payload is {"deliveryId": <id>}.
const JobDeliver = "notify.deliver"

const (
	deliverPriority = 3
	deliverAttempts = 5
	// errorsWindow is how long one source of the event errors stays quiet after a message.
	errorsWindow = time.Hour
	maxSubject   = 200
	maxBody      = 4000
)

// deliverPayload is the payload of a notify.deliver job.
type deliverPayload struct {
	DeliveryID int64 `json:"deliveryId"`
}

// Dispatch is the default Dispatcher of the inbox: a new episode of a favourite becomes the event
// new_content of the profile's account, in the profile's language, prefixed with its name (5.5).
// Its visibility for the profile is checked before (NewEpisodes).
func (s *Service) Dispatch(ctx context.Context, n model.Notification, p model.Profile) error {
	parent := n.Item.SeriesTitle
	key := "new_episode"
	if n.Parent != nil {
		parent = n.Parent.Title
		if n.Parent.Kind == "podcast" {
			key = "new_podcast_episode"
		}
	}
	if parent == "" {
		parent = n.Item.Title
	}
	body := n.Item.Title
	if n.Item.SeasonNumber != nil && n.Item.EpisodeNumber != nil {
		body = fmt.Sprintf("S%02dE%02d · %s", *n.Item.SeasonNumber, *n.Item.EpisodeNumber, n.Item.Title)
	}
	return s.Emit(ctx, model.Event{
		Type: model.EventNewContent, AccountID: p.AccountID, ProfileID: p.ID,
		Subject: text(p.Language, key, parent), Body: body, Link: "/items/" + n.Item.ID, ItemID: n.Item.ID,
	})
}

// target is one switched-on channel of an account that chose an event.
type target struct {
	accountID int64
	channel   string
	config    string
}

// Emit records an event for the channels (design 3.2): one row in deliveries and one job
// notify.deliver per switched-on channel that chose the event and that the server can send on.
// Recipients: errors the main account (once per Source and hour), announcements every account, the
// other events the event's account. ProfileID prefixes the subject with "[<profile name>] ".
// Without a queue (WithQueue) it does nothing. A failure to queue one delivery marks it failed and
// is returned after the others.
func (s *Service) Emit(ctx context.Context, ev model.Event) error {
	if s.queue == nil {
		return nil
	}
	if !slices.Contains(eventNames, ev.Type) {
		return fmt.Errorf("unknown event %q", ev.Type)
	}
	if ev.Type == model.EventErrors {
		// The dedupe reads and writes deliveries; serialised so two failures of one source in the
		// same moment send once.
		s.errorsMu.Lock()
		defer s.errorsMu.Unlock()
		if ev.Source != "" {
			recent, err := s.write.DeliveryRecentExists(ctx, systemq.DeliveryRecentExistsParams{
				Event: ev.Type, Source: sql.NullString{String: ev.Source, Valid: true}, Since: s.now().Add(-errorsWindow).UnixMilli(),
			})
			if err != nil {
				return fmt.Errorf("dedupe errors: %w", err)
			}
			if recent {
				return nil
			}
		}
	}
	targets, err := s.targets(ctx, ev)
	if err != nil || len(targets) == 0 {
		return err
	}
	subject := ev.Subject
	if ev.ProfileID != 0 {
		name, err := s.write.NotifyProfileName(ctx, ev.ProfileID)
		if err != nil {
			return fmt.Errorf("profile of the event: %w", err)
		}
		subject = "[" + name + "] " + subject
	}
	srv := s.serverState(ctx)
	var errs []error
	for _, t := range targets {
		if !srv.available(t.channel) || (t.channel == model.ChannelTelegram && parseConfig(t.config).ChatID == 0) {
			continue
		}
		if err := s.queueDelivery(ctx, t, ev, subject); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// targets returns the switched-on channels that chose the event.
func (s *Service) targets(ctx context.Context, ev model.Event) ([]target, error) {
	var out []target
	switch ev.Type {
	case model.EventErrors:
		rows, err := s.write.ChannelTargetsOwners(ctx, ev.Type)
		if err != nil {
			return nil, fmt.Errorf("recipients of %s: %w", ev.Type, err)
		}
		for _, r := range rows {
			out = append(out, target{r.AccountID, r.Channel, r.Config})
		}
	case model.EventAnnouncements:
		rows, err := s.write.ChannelTargetsByEvent(ctx, ev.Type)
		if err != nil {
			return nil, fmt.Errorf("recipients of %s: %w", ev.Type, err)
		}
		for _, r := range rows {
			out = append(out, target{r.AccountID, r.Channel, r.Config})
		}
	default:
		if ev.AccountID == 0 {
			return nil, fmt.Errorf("event %s without account", ev.Type)
		}
		rows, err := s.write.ChannelTargetsByAccountEvent(ctx, systemq.ChannelTargetsByAccountEventParams{AccountID: ev.AccountID, Event: ev.Type})
		if err != nil {
			return nil, fmt.Errorf("recipients of %s: %w", ev.Type, err)
		}
		for _, r := range rows {
			out = append(out, target{r.AccountID, r.Channel, r.Config})
		}
	}
	return out, nil
}

// queueDelivery stores one delivery and queues its job.
func (s *Service) queueDelivery(ctx context.Context, t target, ev model.Event, subject string) error {
	id, err := s.write.DeliveryInsert(ctx, systemq.DeliveryInsertParams{
		AccountID: t.accountID, ProfileID: sql.NullInt64{Int64: ev.ProfileID, Valid: ev.ProfileID != 0},
		Channel: t.channel, Event: ev.Type, Source: sql.NullString{String: ev.Source, Valid: ev.Source != ""},
		Subject: truncate(subject, maxSubject), Body: truncate(ev.Body, maxBody),
		Link: sql.NullString{String: ev.Link, Valid: ev.Link != ""}, Now: s.now().UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("store delivery: %w", err)
	}
	payload, _ := json.Marshal(deliverPayload{DeliveryID: id})
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type: JobDeliver, Priority: deliverPriority, IdempotencyKey: JobDeliver + ":" + strconv.FormatInt(id, 10),
		Payload: payload, MaxAttempts: deliverAttempts,
	}); err != nil {
		_ = s.write.DeliveryFailed(context.WithoutCancel(ctx), systemq.DeliveryFailedParams{
			State: "failed", LastError: sql.NullString{String: "not queued", Valid: true}, ID: id,
		})
		return fmt.Errorf("queue delivery %d: %w", id, err)
	}
	return nil
}
