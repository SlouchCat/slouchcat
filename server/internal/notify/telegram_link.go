package notify

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Telegram link (design 1, A-6): the account asks for a code, the user sends "/start <code>" to the
// bot, and the poller links the chat it came from. The code has six characters of linkAlphabet
// (no 0, 1: easy to read and type) and lives 15 minutes.
const (
	linkAlphabet     = "ABCDEFGHIJKLMNOPQRSTUVWXYZ23456789"
	linkCodeLen      = 6
	linkLifetime     = 15 * time.Minute
	linkPollInterval = 5 * time.Second
)

// Retention of the deliveries (the record of what went out on which channel).
const (
	deliveryKeep  = 30 * 24 * time.Hour
	pruneInterval = time.Hour
	pruneBatch    = 500 // the LIMIT of DeliveriesDeleteOld
)

// CreateTelegramLink issues a new link code for the session's account; an older open code stops
// working, a linked chat stays linked until the new code is used. model.ErrChannelUnavailable
// without a bot token.
func (s *Service) CreateTelegramLink(ctx context.Context, session model.Session) (model.ChannelSetting, error) {
	srv := s.serverState(ctx)
	if srv.token == "" {
		return model.ChannelSetting{}, model.ErrChannelUnavailable
	}
	now := s.now()
	var err error
	for range 3 { // a code that another account holds right now is drawn again
		err = s.write.TelegramLinkUpsert(ctx, systemq.TelegramLinkUpsertParams{
			AccountID: session.Account.ID, Code: newLinkCode(), ExpiresAt: now.Add(linkLifetime).UnixMilli(), Now: now.UnixMilli(),
		})
		if err == nil || !strings.Contains(err.Error(), "UNIQUE constraint failed: telegram_links.code") {
			break
		}
	}
	if err != nil {
		return model.ChannelSetting{}, fmt.Errorf("store telegram link: %w", err)
	}
	row, err := s.channelRow(ctx, session.Account.ID, model.ChannelTelegram)
	if err != nil {
		return model.ChannelSetting{}, err
	}
	return s.setting(ctx, session, row, srv)
}

// newLinkCode draws a code without modulo bias (bytes at or above the last multiple of the
// alphabet's size are drawn again).
func newLinkCode() string {
	limit := 256 - 256%len(linkAlphabet)
	out := make([]byte, 0, linkCodeLen)
	var buf [16]byte
	for len(out) < linkCodeLen {
		_, _ = rand.Read(buf[:]) // crypto/rand.Read never fails
		for _, c := range buf {
			if int(c) < limit && len(out) < linkCodeLen {
				out = append(out, linkAlphabet[int(c)%len(linkAlphabet)])
			}
		}
	}
	return string(out)
}

// DeleteTelegramLink unlinks the chat of the session's account, drops an open code and switches the
// channel off (idempotent).
func (s *Service) DeleteTelegramLink(ctx context.Context, session model.Session) error {
	if err := s.write.TelegramLinkDelete(ctx, session.Account.ID); err != nil {
		return fmt.Errorf("delete telegram link: %w", err)
	}
	row, err := s.channelRow(ctx, session.Account.ID, model.ChannelTelegram)
	if err != nil {
		return err
	}
	cfg := parseConfig(row.Config)
	cfg.ChatID = 0
	if err := s.write.ChannelUpsert(ctx, systemq.ChannelUpsertParams{
		AccountID: row.AccountID, Channel: row.Channel, Enabled: 0, Events: row.Events, Config: cfg.json(), Now: s.now().UnixMilli(),
	}); err != nil {
		return fmt.Errorf("unlink telegram: %w", err)
	}
	return nil
}

// Run does the background work of the channels until ctx ends (main starts it): it polls the bot
// while link codes are open and a bot token is configured (getUpdates waits up to 20 s for a
// message, so the loop does not spin; a failed poll is logged and retried after the interval), and
// once an hour it deletes finished deliveries older than deliveryKeep.
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(s.linkPoll)
	defer ticker.Stop()
	var pruned time.Time
	for {
		if now := s.now(); now.Sub(pruned) >= pruneInterval {
			if err := s.pruneDeliveries(ctx, now); err != nil && ctx.Err() == nil {
				s.log.WarnContext(ctx, "delete old deliveries", "err", err)
			}
			pruned = now
		}
		if err := s.pollTelegramLinks(ctx); err != nil && ctx.Err() == nil {
			s.log.WarnContext(ctx, "telegram link poll failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// pruneDeliveries deletes sent, failed and skipped deliveries older than deliveryKeep in batches.
func (s *Service) pruneDeliveries(ctx context.Context, now time.Time) error {
	for {
		n, err := s.write.DeliveriesDeleteOld(ctx, now.Add(-deliveryKeep).UnixMilli())
		if err != nil {
			return err
		}
		if n < pruneBatch {
			return nil
		}
	}
}

// pollTelegramLinks runs one round: drop expired codes, and with open ones read the bot's messages
// and link the chats that sent a valid "/start <code>".
func (s *Service) pollTelegramLinks(ctx context.Context) error {
	now := s.now().UnixMilli()
	if _, err := s.write.TelegramLinksDeleteExpired(ctx, now); err != nil {
		return fmt.Errorf("delete expired links: %w", err)
	}
	open, err := s.write.TelegramLinksOpen(ctx, now)
	if err != nil || open == 0 || s.cfg == nil {
		return err
	}
	token, err := s.cfg.TelegramToken(ctx)
	if err != nil || token == "" {
		return err
	}
	updates, err := s.telegram.GetUpdates(ctx, token, s.tgOffset)
	if err != nil {
		return err
	}
	for _, u := range updates {
		s.tgOffset = max(s.tgOffset, u.UpdateID+1)
		if u.Message == nil {
			continue
		}
		code, ok := linkCodeIn(u.Message.Text)
		if !ok {
			continue
		}
		if err := s.linkChat(ctx, token, code, u.Message.Chat.ID); err != nil {
			s.log.WarnContext(ctx, "telegram link failed", "chat_id", u.Message.Chat.ID, "err", err)
		}
	}
	return nil
}

// linkCodeIn reads the code of "/start CODE" (also "/start@bot CODE", any case).
func linkCodeIn(text string) (string, bool) {
	f := strings.Fields(text)
	if len(f) != 2 || (f[0] != "/start" && !strings.HasPrefix(f[0], "/start@")) {
		return "", false
	}
	code := strings.ToUpper(f[1])
	if len(code) != linkCodeLen || strings.Trim(code, linkAlphabet) != "" {
		return "", false
	}
	return code, true
}

// linkChat links the chat to the account of an open code, switches the channel on, drops the code
// and confirms in the chat. An unknown or expired code is ignored.
func (s *Service) linkChat(ctx context.Context, token, code string, chatID int64) error {
	accountID, err := s.write.TelegramLinkByCode(ctx, systemq.TelegramLinkByCodeParams{Code: code, Now: s.now().UnixMilli()})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find link code: %w", err)
	}
	row, err := s.channelRow(ctx, accountID, model.ChannelTelegram)
	if err != nil {
		return err
	}
	cfg := parseConfig(row.Config)
	cfg.ChatID = chatID
	if err := s.write.ChannelUpsert(ctx, systemq.ChannelUpsertParams{
		AccountID: accountID, Channel: model.ChannelTelegram, Enabled: 1, Events: row.Events, Config: cfg.json(), Now: s.now().UnixMilli(),
	}); err != nil {
		return fmt.Errorf("link chat: %w", err)
	}
	if err := s.write.TelegramLinkDelete(ctx, accountID); err != nil {
		return fmt.Errorf("delete link code: %w", err)
	}
	s.log.InfoContext(ctx, "telegram chat linked", "account_id", accountID, "chat_id", chatID)
	acc, err := s.write.NotifyAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("account of the link: %w", err)
	}
	return s.telegram.SendMessage(ctx, token, chatID, text(acc.Language, "telegram_linked", acc.Email))
}
