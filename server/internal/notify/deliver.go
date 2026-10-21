package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify/push"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

const (
	// pushTTL is how long a push service keeps a message for an offline browser; a test message
	// is only worth a minute.
	pushTTL     = 24 * time.Hour
	pushTestTTL = time.Minute
	maxError    = 500
)

// Reasons a delivery is skipped instead of sent: the channel has nowhere to send. Not an error of
// the job; the delivery ends skipped with the reason.
var (
	errChannelOff    = errors.New("channel switched off")
	errNotConfigured = errors.New("channel not configured on the server")
	errNoRecipient   = errors.New("no recipient (not linked or no subscription)")
)

// RegisterJobs registers the handler of notify.deliver on w; main gives the deliveries a worker of
// their own, so a slow mail server never holds up scans.
func (s *Service) RegisterJobs(w *jobs.Worker) {
	w.Handle(JobDeliver, s.deliver)
}

// deliver sends one delivery. A delivery that is no longer queued is done (the job may run twice);
// a channel without recipient ends skipped; an error is recorded and returned, so the queue retries
// it (5 attempts) and the delivery ends failed with the last attempt. A failed delivery raises no
// event errors (no loop).
func (s *Service) deliver(ctx context.Context, j *jobs.Job) error {
	var p deliverPayload
	if err := json.Unmarshal(j.Payload, &p); err != nil || p.DeliveryID == 0 {
		return fmt.Errorf("notify.deliver payload: %w", errors.Join(err, errors.New("no delivery id")))
	}
	d, err := s.write.DeliveryGet(ctx, p.DeliveryID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // the account was deleted
	}
	if err != nil {
		return fmt.Errorf("load delivery %d: %w", p.DeliveryID, err)
	}
	if d.State != "queued" {
		return nil
	}
	if n, err := s.write.DeliveryClaim(ctx, d.ID); err != nil || n == 0 {
		return err
	}
	row, err := s.channelRow(ctx, d.AccountID, d.Channel)
	if err != nil {
		return err
	}
	m := message{Subject: d.Subject, Body: d.Body, Link: d.Link.String}
	if row.Enabled != 1 {
		err = errChannelOff
	} else {
		err = s.sendOn(ctx, d.AccountID, row, m, pushTTL)
	}
	switch {
	case err == nil:
		return s.write.DeliverySent(ctx, systemq.DeliverySentParams{Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, ID: d.ID})
	case errors.Is(err, errChannelOff), errors.Is(err, errNotConfigured), errors.Is(err, errNoRecipient):
		return s.write.DeliverySkipped(ctx, systemq.DeliverySkippedParams{Reason: sql.NullString{String: err.Error(), Valid: true}, ID: d.ID})
	default:
		state := "queued"
		if j.Attempts >= j.MaxAttempts {
			state = "failed"
		}
		if ferr := s.write.DeliveryFailed(context.WithoutCancel(ctx), systemq.DeliveryFailedParams{
			State: state, LastError: sql.NullString{String: errorText(err), Valid: true}, ID: d.ID,
		}); ferr != nil {
			return errors.Join(err, ferr)
		}
		return err
	}
}

// errorText is an error for deliveries.last_error and problem details: redacted like a log line
// and cut.
func errorText(err error) string {
	return truncate(logging.RedactString(err.Error()), maxError)
}

// sendOn sends m on one channel of an account: errNotConfigured when the server lacks the channel,
// errNoRecipient without a linked chat or a subscription, else the sender's error. Push goes to
// every subscription of the account; a subscription the push service no longer knows is deleted.
// It succeeds when at least one subscription took the message (a retry after a partial failure may
// reach a browser twice).
func (s *Service) sendOn(ctx context.Context, accountID int64, row systemq.AccountChannel, m message, ttl time.Duration) error {
	srv := s.serverState(ctx)
	if !srv.available(row.Channel) {
		return errNotConfigured
	}
	cfg := parseConfig(row.Config)
	switch row.Channel {
	case model.ChannelEmail:
		to := cfg.Address
		if to == "" {
			acc, err := s.write.NotifyAccount(ctx, accountID)
			if err != nil {
				return fmt.Errorf("account of the delivery: %w", err)
			}
			to = acc.Email
		}
		plain, html := emailBodies(m, srv.publicURL)
		return s.mailer.Mail(ctx, srv.smtp, to, m.Subject, plain, html)
	case model.ChannelTelegram:
		if cfg.ChatID == 0 {
			return errNoRecipient
		}
		return s.telegram.SendMessage(ctx, srv.token, cfg.ChatID, telegramText(m, srv.publicURL))
	case model.ChannelPush:
		return s.sendPush(ctx, accountID, m, srv.publicURL, ttl)
	}
	return errNotConfigured
}

func (s *Service) sendPush(ctx context.Context, accountID int64, m message, publicURL string, ttl time.Duration) error {
	subs, err := s.write.PushSubscriptionsByAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("push subscriptions: %w", err)
	}
	if len(subs) == 0 {
		return errNoRecipient
	}
	vapid, err := s.vapid(ctx)
	if err != nil {
		return err
	}
	link := m.Link
	if abs := absolute(m.Link, publicURL); abs != "" {
		link = abs
	}
	if link == "" {
		link = "/"
	}
	payload, _ := json.Marshal(struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		URL   string `json:"url"`
	}{m.Subject, m.Body, link})
	sent := 0
	var errs []error
	for _, sub := range subs {
		err := s.pusher.Push(ctx, vapid, push.Subscription{Endpoint: sub.Endpoint, P256dh: sub.P256dh, Auth: sub.Auth}, payload, ttl)
		switch {
		case err == nil:
			sent++
			if err := s.write.PushSubscriptionUsed(ctx, systemq.PushSubscriptionUsedParams{Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, ID: sub.ID}); err != nil {
				s.log.WarnContext(ctx, "mark push subscription used", "subscription_id", sub.ID, "err", err)
			}
		case errors.Is(err, push.ErrSubscriptionGone):
			s.log.InfoContext(ctx, "push subscription gone, deleted", "subscription_id", sub.ID, "account_id", accountID)
			if err := s.write.PushSubscriptionDeleteByID(ctx, sub.ID); err != nil {
				errs = append(errs, fmt.Errorf("delete push subscription: %w", err))
			}
		default:
			if ferr := s.write.PushSubscriptionFailed(ctx, sub.ID); ferr != nil {
				s.log.WarnContext(ctx, "count push failure", "subscription_id", sub.ID, "err", ferr)
			}
			errs = append(errs, err)
		}
	}
	switch {
	case sent > 0:
		if len(errs) > 0 {
			s.log.WarnContext(ctx, "push reached only some subscriptions", "account_id", accountID, "sent", sent, "err", errors.Join(errs...))
		}
		return nil
	case len(errs) > 0:
		return errors.Join(errs...)
	default:
		return errNoRecipient // every subscription was gone
	}
}

// TestChannel sends a test message on a channel of the session's account, right away and not
// through the queue. Errors: model.ErrNotFound (unknown channel), model.ErrChannelUnavailable,
// model.ErrChannelDisabled (switched off, not linked or without subscription), *model.SendError
// with model.ErrSendFailed and a detail without secrets.
func (s *Service) TestChannel(ctx context.Context, session model.Session, channel string) error {
	row, err := s.channelRow(ctx, session.Account.ID, channel)
	if err != nil {
		return err
	}
	if !s.serverState(ctx).available(channel) {
		return model.ErrChannelUnavailable
	}
	if row.Enabled != 1 {
		return model.ErrChannelDisabled
	}
	lang := session.Profile.Language
	if lang == "" {
		acc, err := s.write.NotifyAccount(ctx, session.Account.ID)
		if err != nil {
			return fmt.Errorf("account: %w", err)
		}
		lang = acc.Language
	}
	m := message{Subject: text(lang, "test_subject"), Body: text(lang, "test_body", text(lang, "channel_"+channel)), Link: "/"}
	err = s.sendOn(ctx, session.Account.ID, row, m, pushTestTTL)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errNotConfigured):
		return model.ErrChannelUnavailable
	case errors.Is(err, errNoRecipient):
		return model.ErrChannelDisabled
	case ctx.Err() != nil:
		return ctx.Err()
	default:
		s.log.WarnContext(ctx, "test message failed", "channel", channel, "err", err)
		return &model.SendError{Err: model.ErrSendFailed, Detail: errorText(err)}
	}
}

// SendTestEmail sends a test mail through the configured server to to, or to the main account when
// empty (admin.EmailTester). model.ErrEmailNotConfigured without a server; *model.SendError with
// model.ErrEmailSendFailed and a detail without secrets.
func (s *Service) SendTestEmail(ctx context.Context, to string) error {
	srv := s.serverState(ctx)
	if !srv.email {
		return model.ErrEmailNotConfigured
	}
	owner, err := s.write.NotifyOwner(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("main account: %w", err)
	}
	lang := owner.Language
	if to = strings.TrimSpace(to); to == "" {
		to = owner.Email
	}
	if to == "" {
		return &model.SendError{Err: model.ErrEmailSendFailed, Detail: "no recipient"}
	}
	m := message{Subject: text(lang, "test_subject"), Body: text(lang, "test_body", text(lang, "channel_email")), Link: "/"}
	plain, html := emailBodies(m, srv.publicURL)
	if err := s.mailer.Mail(ctx, srv.smtp, to, m.Subject, plain, html); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.log.WarnContext(ctx, "test mail failed", "err", err)
		return &model.SendError{Err: model.ErrEmailSendFailed, Detail: errorText(err)}
	}
	return nil
}
