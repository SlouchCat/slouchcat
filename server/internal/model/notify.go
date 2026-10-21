package model

import (
	"errors"
	"time"
)

// Notifications of M8 (plan 5.2, 5.5, A-6, A-7, E-8): the channels of an account (e-mail, Web Push,
// Telegram) with their events, the push subscriptions, the Telegram link and the announcements of
// the main user. The contract schemas of the same names in api/openapi.yaml are the source.

// Errors of the notification channels and announcements (M8).
var (
	ErrChannelUnavailable    = errors.New("channel unavailable")    // 422 channel_unavailable
	ErrChannelDisabled       = errors.New("channel disabled")       // 422 channel_disabled
	ErrEventNotAllowed       = errors.New("event not allowed")      // 422 event_not_allowed
	ErrSendFailed            = errors.New("send failed")            // 502 send_failed (see SendError)
	ErrPushUnavailable       = errors.New("push unavailable")       // 404 push_unavailable
	ErrAnnouncementPublished = errors.New("announcement published") // 409 announcement_published
)

// SendError is ErrSendFailed or ErrEmailSendFailed with a detail for the problem response; the
// detail never contains a secret.
type SendError struct {
	Err    error
	Detail string
}

func (e *SendError) Error() string { return e.Err.Error() + ": " + e.Detail }

// Unwrap makes errors.Is(err, e.Err) true.
func (e *SendError) Unwrap() error { return e.Err }

// Channels (Channel).
const (
	ChannelEmail    = "email"
	ChannelPush     = "push"
	ChannelTelegram = "telegram"
)

// Event types (EventType).
const (
	EventNewContent    = "new_content"
	EventAcquisition   = "acquisition"
	EventErrors        = "errors"
	EventInvitations   = "invitations"
	EventAnnouncements = "announcements"
)

// ChannelSummary is a channel in the account overview.
type ChannelSummary struct {
	Channel string
	Enabled bool
}

// ChannelSetting is one channel of an account with its events; exactly the part of its channel is set.
type ChannelSetting struct {
	Channel   string
	Enabled   bool
	Events    []string
	Available bool // the server has the channel configured
	Email     *EmailChannel
	Telegram  *TelegramChannel
	Push      *PushChannel
}

// EmailChannel is the recipient of the e-mail channel.
type EmailChannel struct {
	Address string
}

// TelegramChannel is the linked chat or the open link code.
type TelegramChannel struct {
	Linked        bool
	ChatID        string
	LinkCode      string
	LinkExpiresAt *time.Time
	BotUsername   string
}

// PushChannel lists the push subscriptions of the account.
type PushChannel struct {
	Subscriptions []PushSubscription
}

// ChannelSettingUpdate switches a channel and chooses its events; EmailAddress nil uses the account's e-mail.
type ChannelSettingUpdate struct {
	Enabled      bool
	Events       []string
	EmailAddress *string
}

// PushSubscription is a push subscription without its secrets.
type PushSubscription struct {
	ID         int64
	DeviceName string
	CreatedAt  time.Time
	ThisDevice bool
}

// PushSubscriptionInput is the browser's subscription; endpoint and keys are capability secrets.
type PushSubscriptionInput struct {
	Endpoint   string
	P256dh     string
	Auth       string
	DeviceName string
	UserAgent  string
}

// Announcement is an announcement as the administration sees it (A-7).
type Announcement struct {
	ID                int64
	Title             string
	Body              string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	PublishedAt       *time.Time
	CreatedBy         string // e-mail of the creator; empty when the account is gone
	AcknowledgedCount int
	ProfileCount      int
}

// AnnouncementInput writes an announcement.
type AnnouncementInput struct {
	Title   string
	Body    string
	Publish bool
}

// ProfileAnnouncement is a published announcement a profile has not acknowledged.
type ProfileAnnouncement struct {
	ID          int64
	Title       string
	Body        string
	PublishedAt time.Time
}

// Event is one notification for the channels of an account (design 3.2). ProfileID 0 addresses the
// account; Source groups the event errors for the hourly dedupe.
type Event struct {
	Type      string // Event* constants
	AccountID int64
	ProfileID int64
	Subject   string
	Body      string
	Link      string
	ItemID    string
	Source    string
}
