package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify/email"
	"github.com/slouchcat/slouchcat/server/internal/notify/push"
	"github.com/slouchcat/slouchcat/server/internal/notify/telegram"
)

// Config is the server side of the channels (package admin implements it: the environment wins
// over the stored settings). Every method is read on each use, so a change in the settings applies
// to the next message.
type Config interface {
	// SMTPConfig returns the e-mail server and whether one is configured.
	SMTPConfig(ctx context.Context) (model.SMTPConfig, bool, error)
	// TelegramToken returns the bot token; empty = no Telegram channel.
	TelegramToken(ctx context.Context) (string, error)
	// VAPIDKeys returns the Web Push key pair (base64url); empty = none yet, notify creates one and
	// stores it with SetVAPIDKeys.
	VAPIDKeys(ctx context.Context) (pub, priv string, err error)
	SetVAPIDKeys(ctx context.Context, pub, priv string) error
	// PublicURL is the server's address for links in messages; empty = no links.
	PublicURL(ctx context.Context) (string, error)
}

// Mailer sends one e-mail through the SMTP server of cfg (notify/email in production).
type Mailer interface {
	Mail(ctx context.Context, cfg model.SMTPConfig, to, subject, text, html string) error
}

// Pusher hands one Web Push message to the push service of a subscription (notify/push in
// production); push.ErrSubscriptionGone when the service answers 404 or 410.
type Pusher interface {
	Push(ctx context.Context, vapid push.VAPID, sub push.Subscription, payload []byte, ttl time.Duration) error
}

// TelegramAPI is the Bot API (notify/telegram in production).
type TelegramAPI interface {
	GetMe(ctx context.Context, token string) (string, error)
	SendMessage(ctx context.Context, token string, chatID int64, text string) error
	GetUpdates(ctx context.Context, token string, offset int64) ([]telegram.Update, error)
}

// Senders replaces the senders of the channels; nil fields keep the production sender.
type Senders struct {
	Email    Mailer
	Push     Pusher
	Telegram TelegramAPI
}

// WithSenders sets the senders (main passes the Telegram client with the development base URL;
// tests pass fakes).
func WithSenders(senders Senders) Option {
	return func(s *Service) {
		if senders.Email != nil {
			s.mailer = senders.Email
		}
		if senders.Push != nil {
			s.pusher = senders.Push
		}
		if senders.Telegram != nil {
			s.telegram = senders.Telegram
		}
	}
}

// WithQueue sets the job queue of the deliveries (notify.deliver); without it events reach no
// channel.
func WithQueue(q *jobs.Queue) Option {
	return func(s *Service) { s.queue = q }
}

// WithDevPush accepts http push endpoints and lets the push sender reach private and loopback
// addresses (development mode: the push mock of the end-to-end tests).
func WithDevPush(dev bool) Option {
	return func(s *Service) { s.devPush = dev }
}

// SetConfig sets the server side of the channels. main calls it once admin exists (admin depends
// on library and metadata, which depend on notify), before the workers, the Telegram poller and the
// HTTP server start.
func (s *Service) SetConfig(c Config) {
	s.cfg = c
}

// smtpMailer sends through notify/email.
type smtpMailer struct{}

func (smtpMailer) Mail(ctx context.Context, cfg model.SMTPConfig, to, subject, text, html string) error {
	return email.Sender{Host: cfg.Host, Port: cfg.Port, User: cfg.User, Password: cfg.Password, From: cfg.From, TLS: cfg.TLS}.
		Send(ctx, to, subject, text, html)
}

// webPusher sends through notify/push.
type webPusher struct{ allowPrivate bool }

func (p webPusher) Push(ctx context.Context, vapid push.VAPID, sub push.Subscription, payload []byte, ttl time.Duration) error {
	return push.Sender{VAPID: vapid, AllowPrivate: p.allowPrivate}.Send(ctx, sub, payload, ttl)
}

// server is the state of the channels on the server for one request or delivery.
type server struct {
	smtp       model.SMTPConfig
	email      bool
	token      string // Telegram bot token; empty = none
	push       bool   // a Config exists, so a VAPID pair exists or can be created
	publicURL  string
	emailError string // why the e-mail settings could not be read
	tgError    string
}

func (s server) available(channel string) bool {
	switch channel {
	case model.ChannelEmail:
		return s.email
	case model.ChannelPush:
		return s.push
	case model.ChannelTelegram:
		return s.token != ""
	}
	return false
}

// serverState reads the settings of the channels. A setting that cannot be read leaves its channel
// unavailable (logged); the other channels keep working.
func (s *Service) serverState(ctx context.Context) server {
	var out server
	if s.cfg == nil {
		return out
	}
	out.push = true
	smtp, ok, err := s.cfg.SMTPConfig(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "read e-mail settings", "err", err)
		out.emailError = "settings unreadable"
	}
	out.smtp, out.email = smtp, ok && err == nil
	token, err := s.cfg.TelegramToken(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "read Telegram settings", "err", err)
		out.tgError = "settings unreadable"
	}
	out.token = token
	if err != nil {
		out.token = ""
	}
	publicURL, err := s.cfg.PublicURL(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "read public URL", "err", err)
	}
	out.publicURL = strings.TrimRight(publicURL, "/")
	return out
}

// ChannelAvailability reports for the dashboard whether the server can send on each channel
// (admin.Channels, A-3), keyed by model.ChannelEmail, ChannelPush and ChannelTelegram; the detail
// says why a channel is in error.
func (s *Service) ChannelAvailability(ctx context.Context) (map[string]model.ChannelAvail, error) {
	srv := s.serverState(ctx)
	pushAvail := model.ChannelAvail{Configured: srv.push}
	if srv.push {
		if _, err := s.vapid(ctx); err != nil {
			pushAvail = model.ChannelAvail{Detail: "key pair unavailable"}
		}
	}
	return map[string]model.ChannelAvail{
		model.ChannelEmail:    {Configured: srv.email, Detail: srv.emailError},
		model.ChannelPush:     pushAvail,
		model.ChannelTelegram: {Configured: srv.token != "", Detail: srv.tgError},
	}, nil
}

// botName caches the username of a bot token (shown with the link code); a failed lookup is
// retried after botRetry.
type botName struct {
	token string
	name  string
	at    time.Time
}

const botRetry = time.Minute

// botUsername returns the bot's username for token, asking Telegram at most once per token (and
// once a minute after a failure); "" when unknown.
func (s *Service) botUsername(ctx context.Context, token string) string {
	s.botMu.Lock()
	defer s.botMu.Unlock()
	if s.bot.token == token && (s.bot.name != "" || s.now().Sub(s.bot.at) < botRetry) {
		return s.bot.name
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	name, err := s.telegram.GetMe(ctx, token)
	if err != nil {
		s.log.WarnContext(ctx, "telegram bot name", "err", err)
	}
	s.bot = botName{token: token, name: name, at: s.now()}
	return name
}

// GetMe checks a bot token with Telegram and returns the bot's username (admin.TelegramChecker):
// model.ErrTelegramTokenInvalid when Telegram rejects the token, model.ErrTelegramUnreachable
// otherwise.
func (s *Service) GetMe(ctx context.Context, token string) (string, error) {
	name, err := s.telegram.GetMe(ctx, token)
	switch {
	case err == nil:
		s.botMu.Lock()
		s.bot = botName{token: token, name: name, at: s.now()}
		s.botMu.Unlock()
		return name, nil
	case errors.Is(err, telegram.ErrUnauthorized):
		return "", fmt.Errorf("%w: %w", model.ErrTelegramTokenInvalid, err)
	default:
		return "", fmt.Errorf("%w: %w", model.ErrTelegramUnreachable, err)
	}
}
