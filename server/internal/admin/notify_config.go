package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// The server configuration of the notification channels for package notify (lane S3 defines the
// interface notify.Config with exactly these methods): the e-mail server, the Telegram bot token,
// the VAPID key pair of Web Push and the public URL for links. The environment wins over the stored
// settings for all but the VAPID pair, which only the server creates. Secrets are decrypted here
// and never logged.

// SMTPConfig returns the e-mail server with its password; configured is false when neither the
// environment nor the settings name a host.
func (s *Service) SMTPConfig(ctx context.Context) (model.SMTPConfig, bool, error) {
	if env := s.ops.env.SMTP; env.Host != "" {
		return env, true, nil
	}
	return s.storedSMTP(ctx, true)
}

// TelegramToken returns the bot token of the environment or else the stored one; "" = none.
func (s *Service) TelegramToken(ctx context.Context) (string, error) {
	if env := s.ops.env.TelegramBotToken; env != "" {
		return env, nil
	}
	return s.secretKey(ctx, "", SettingTelegramBotToken)
}

// VAPIDKeys returns the VAPID key pair (base64url); both "" while none was created.
func (s *Service) VAPIDKeys(ctx context.Context) (pub, priv string, err error) {
	pub, ok, err := s.setting(ctx, SettingVAPIDPublicKey)
	if err != nil || !ok {
		return "", "", err
	}
	if priv, err = s.secretKey(ctx, "", SettingVAPIDPrivateKey); err != nil || priv == "" {
		return "", "", err
	}
	return pub, priv, nil
}

// SetVAPIDKeys stores a VAPID key pair, the private key encrypted, in one transaction.
func (s *Service) SetVAPIDKeys(ctx context.Context, pub, priv string) error {
	pub, priv = strings.TrimSpace(pub), strings.TrimSpace(priv)
	if pub == "" || priv == "" {
		return errors.New("vapid keys must not be empty")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	w := settingsWriter{s: s, ctx: ctx, q: appq.New(tx), now: s.now().UnixMilli()}
	w.set(SettingVAPIDPublicKey, pub)
	w.setSecret(SettingVAPIDPrivateKey, priv)
	if w.err != nil {
		return w.err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit vapid keys: %w", err)
	}
	return nil
}

// PublicURL returns the address of the server for links, without a trailing slash: the
// environment wins, else the stored setting; "" = no links.
func (s *Service) PublicURL(ctx context.Context) (string, error) {
	u, _, err := s.publicURL(ctx)
	return u, err
}
