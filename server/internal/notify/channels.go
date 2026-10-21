package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The channels in the order of the settings and the event types (contract enums Channel and
// EventType).
var (
	channelNames = []string{model.ChannelEmail, model.ChannelPush, model.ChannelTelegram}
	eventNames   = []string{model.EventNewContent, model.EventAcquisition, model.EventErrors, model.EventInvitations, model.EventAnnouncements}
)

const maxEmailLen = 254

// channelConfig is account_channels.config: the recipient of the e-mail channel (empty = the
// account's e-mail) and the linked Telegram chat. Neither is a secret.
type channelConfig struct {
	Address string `json:"address,omitempty"`
	ChatID  int64  `json:"chat_id,omitempty"`
}

func parseConfig(raw string) channelConfig {
	var c channelConfig
	_ = json.Unmarshal([]byte(raw), &c) // the column is checked JSON; unknown fields are ignored
	return c
}

func (c channelConfig) json() string {
	b, _ := json.Marshal(c)
	return string(b)
}

func parseEvents(raw string) []string {
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// EnsureDefaults creates the three channel rows of a new account with the defaults of plan 5.2:
// e-mail on with announcements for a guest, everything off for the main account (with new content,
// errors, invitations and announcements chosen). Existing rows stay. auth calls it after CreateOwner
// and AcceptInvitation.
func (s *Service) EnsureDefaults(ctx context.Context, accountID int64) error {
	if err := s.write.ChannelEnsureDefaults(ctx, systemq.ChannelEnsureDefaultsParams{Now: s.now().UnixMilli(), AccountID: accountID}); err != nil {
		return fmt.Errorf("default channels of account %d: %w", accountID, err)
	}
	return nil
}

// channelRows returns the three channel rows of an account, creating missing ones with the
// defaults first (an account of an older fixture or whose creation hook failed).
func (s *Service) channelRows(ctx context.Context, accountID int64) ([]systemq.AccountChannel, error) {
	rows, err := s.read.ChannelsByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("channels: %w", err)
	}
	if len(rows) == len(channelNames) {
		return rows, nil
	}
	if err := s.EnsureDefaults(ctx, accountID); err != nil {
		return nil, err
	}
	rows, err = s.read.ChannelsByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("channels: %w", err)
	}
	return rows, nil
}

// channelRow returns one channel row of an account (model.ErrNotFound for an unknown channel).
func (s *Service) channelRow(ctx context.Context, accountID int64, channel string) (systemq.AccountChannel, error) {
	if !slices.Contains(channelNames, channel) {
		return systemq.AccountChannel{}, model.ErrNotFound
	}
	rows, err := s.channelRows(ctx, accountID)
	if err != nil {
		return systemq.AccountChannel{}, err
	}
	for _, r := range rows {
		if r.Channel == channel {
			return r, nil
		}
	}
	return systemq.AccountChannel{}, model.ErrNotFound
}

// Channels returns the e-mail, push and Telegram channels of the session's account, in this order.
func (s *Service) Channels(ctx context.Context, session model.Session) ([]model.ChannelSetting, error) {
	rows, err := s.channelRows(ctx, session.Account.ID)
	if err != nil {
		return nil, err
	}
	srv := s.serverState(ctx)
	out := make([]model.ChannelSetting, 0, len(rows))
	for _, r := range rows {
		c, err := s.setting(ctx, session, r, srv)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// setting builds one channel of the account with its part: the recipient, the Telegram link or
// the push subscriptions.
func (s *Service) setting(ctx context.Context, session model.Session, r systemq.AccountChannel, srv server) (model.ChannelSetting, error) {
	cfg := parseConfig(r.Config)
	c := model.ChannelSetting{Channel: r.Channel, Enabled: r.Enabled == 1, Events: parseEvents(r.Events), Available: srv.available(r.Channel)}
	if c.Events == nil {
		c.Events = []string{}
	}
	switch r.Channel {
	case model.ChannelEmail:
		address := cfg.Address
		if address == "" {
			address = session.Account.Email
		}
		c.Email = &model.EmailChannel{Address: address}
	case model.ChannelTelegram:
		t := &model.TelegramChannel{Linked: cfg.ChatID != 0}
		if cfg.ChatID != 0 {
			t.ChatID = strconv.FormatInt(cfg.ChatID, 10)
		}
		link, err := s.read.TelegramLinkGet(ctx, session.Account.ID)
		switch {
		case err == nil && link.ExpiresAt > s.now().UnixMilli():
			t.LinkCode = link.Code
			at := time.UnixMilli(link.ExpiresAt).UTC()
			t.LinkExpiresAt = &at
		case err != nil && !errors.Is(err, sql.ErrNoRows):
			return model.ChannelSetting{}, fmt.Errorf("telegram link: %w", err)
		}
		if srv.token != "" {
			t.BotUsername = s.botUsername(ctx, srv.token)
		}
		c.Telegram = t
	case model.ChannelPush:
		subs, err := s.pushSubscriptions(ctx, session)
		if err != nil {
			return model.ChannelSetting{}, err
		}
		c.Push = &model.PushChannel{Subscriptions: subs}
	}
	return c, nil
}

// UpdateChannel switches a channel of the session's account and stores its events and, for
// e-mail, the recipient (nil = the account's e-mail). Errors: model.ErrNotFound (unknown channel),
// *model.ValidationError (unknown event, invalid address), model.ErrEventNotAllowed (errors on a
// guest account), model.ErrChannelUnavailable (switching on a channel the server lacks; switching
// off is always allowed).
func (s *Service) UpdateChannel(ctx context.Context, session model.Session, channel string, upd model.ChannelSettingUpdate) (model.ChannelSetting, error) {
	if !slices.Contains(channelNames, channel) {
		return model.ChannelSetting{}, model.ErrNotFound
	}
	var verr *model.ValidationError
	events := make([]string, 0, len(upd.Events))
	for _, e := range upd.Events {
		switch {
		case !slices.Contains(eventNames, e):
			verr = verr.Add("events", "invalid_value")
		case !slices.Contains(events, e):
			events = append(events, e)
		}
	}
	var address string
	if channel == model.ChannelEmail && upd.EmailAddress != nil {
		address = strings.ToLower(strings.TrimSpace(*upd.EmailAddress))
		if !validAddress(address) {
			verr = verr.Add("email.address", "invalid_email")
		}
	}
	if verr != nil {
		return model.ChannelSetting{}, verr
	}
	if slices.Contains(events, model.EventErrors) && !session.IsOwner() {
		return model.ChannelSetting{}, model.ErrEventNotAllowed
	}
	srv := s.serverState(ctx)
	if upd.Enabled && !srv.available(channel) {
		return model.ChannelSetting{}, model.ErrChannelUnavailable
	}
	row, err := s.channelRow(ctx, session.Account.ID, channel)
	if err != nil {
		return model.ChannelSetting{}, err
	}
	cfg := parseConfig(row.Config)
	if channel == model.ChannelEmail {
		cfg.Address = address
	}
	slices.SortFunc(events, func(a, b string) int { return slices.Index(eventNames, a) - slices.Index(eventNames, b) })
	raw, _ := json.Marshal(events)
	row.Enabled, row.Events, row.Config = boolInt(upd.Enabled), string(raw), cfg.json()
	if err := s.write.ChannelUpsert(ctx, systemq.ChannelUpsertParams{
		AccountID: row.AccountID, Channel: row.Channel, Enabled: row.Enabled, Events: row.Events, Config: row.Config,
		Now: s.now().UnixMilli(),
	}); err != nil {
		return model.ChannelSetting{}, fmt.Errorf("store channel: %w", err)
	}
	return s.setting(ctx, session, row, srv)
}

// validAddress accepts a bare e-mail address (no display name) of at most 254 characters.
func validAddress(address string) bool {
	if address == "" || len(address) > maxEmailLen {
		return false
	}
	a, err := mail.ParseAddress(address)
	return err == nil && a.Address == address && a.Name == "" && strings.Contains(address[strings.LastIndex(address, "@"):], ".")
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
