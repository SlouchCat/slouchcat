package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify/email"
	"github.com/slouchcat/slouchcat/server/internal/notify/telegram"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Settings and integrations of M8 (A-2, design section 1): the public URL, the update check, the
// provider keys of M3 and M7, the e-mail server, the Telegram bot and the Web Push key pair. Secrets
// are stored encrypted through package secrets ("v1:" values) and never leave the server: the
// answers say where a value comes from and show the last four characters of a long stored key. A
// value of the environment wins over a stored one and cannot be changed or cleared here
// (model.EnvWinsError). Lane S3 sends with these settings through notify.Config (notify_config.go).

// Settings keys of M8 (migration 00016 adds no DDL for them).
const (
	SettingPublicURL           = "public_url"
	SettingUpdateCheckEnabled  = "update_check_enabled" // "0" = off; absent = on
	SettingUpdateState         = "update_state"         // JSON, see storedUpdate
	SettingSMTPHost            = "smtp_host"
	SettingSMTPPort            = "smtp_port"
	SettingSMTPUser            = "smtp_user"
	SettingSMTPPassword        = "smtp_password" // encrypted
	SettingSMTPFrom            = "smtp_from"
	SettingSMTPTLS             = "smtp_tls"
	SettingTelegramBotToken    = "telegram_bot_token" //nolint:gosec // G101: the name of a setting, encrypted at rest
	SettingTelegramBotUsername = "telegram_bot_username"
	SettingVAPIDPublicKey      = "vapid_public_key"
	SettingVAPIDPrivateKey     = "vapid_private_key" // encrypted

	// EnvTraktClientID overrides the stored Trakt client id (config.TraktClientID, M7).
	EnvTraktClientID = "SLOUCHCAT_TRAKT_CLIENT_ID"

	maxPublicURL     = 2048
	maxProviderKey   = 1024
	maxSMTPHost      = 253
	maxSMTPUser      = 254
	maxSMTPPassword  = 1024
	minSMTPFrom      = 3
	maxSMTPFrom      = 320
	maxBotToken      = 256
	minHintKeyLength = 12 // a shorter stored key (a TVDB PIN) shows no hint: four characters would give too much away
)

// NotifyEnv are the notification settings of the environment (config.Config); they win over the
// stored ones.
type NotifyEnv struct {
	SMTP             model.SMTPConfig // Host "" = none
	TelegramBotToken string
	PublicURL        string
}

// WithNotifyEnv sets the notification settings of the environment and returns the receiver.
func (s *Service) WithNotifyEnv(env NotifyEnv) *Service {
	s.ops.env = env
	return s
}

// TelegramChecker is implemented by package notify (lane S3): Telegram's getMe for a bot token.
type TelegramChecker interface {
	// GetMe returns the bot's user name; errors wrap model.ErrTelegramTokenInvalid or
	// model.ErrTelegramUnreachable and never contain the token.
	GetMe(ctx context.Context, token string) (username string, err error)
}

// EmailTester is implemented by package notify (lane S3): one test mail through the e-mail server
// of notify.Config.
type EmailTester interface {
	// SendTestEmail sends a test mail to to, or to the main account when empty;
	// model.ErrEmailNotConfigured, *model.SendError with model.ErrEmailSendFailed.
	SendTestEmail(ctx context.Context, to string) error
}

// PushRotator is implemented by package notify (lane S3).
type PushRotator interface {
	// RotateVAPID creates a new VAPID key pair (stored through SetVAPIDKeys) and deletes every push
	// subscription, which was bound to the old pair.
	RotateVAPID(ctx context.Context) error
}

// WithTelegram sets the bot token check and returns the receiver.
func (s *Service) WithTelegram(t TelegramChecker) *Service {
	s.ops.telegram = t
	return s
}

// WithEmailTester sets the test mail and returns the receiver.
func (s *Service) WithEmailTester(e EmailTester) *Service {
	s.ops.email = e
	return s
}

// WithPushRotator sets the VAPID key rotation and returns the receiver.
func (s *Service) WithPushRotator(p PushRotator) *Service {
	s.ops.push = p
	return s
}

// providerKey is a provider key of AdminSettings.keys: its contract field (also its Clear name),
// its setting and the environment variable whose value wins.
type providerKey struct {
	field, setting, env string
}

// providerKeys in the order of the contract. The TVDB PIN pairs with the key of the same source
// (TVDBKey): with the key in the environment, the PIN comes from there too.
var providerKeys = []providerKey{
	{model.ClearTMDB, SettingTMDBKey, EnvTMDBKey},
	{model.ClearTVDB, SettingTVDBKey, EnvTVDBKey},
	{model.ClearTVDBPin, SettingTVDBPin, EnvTVDBKey},
	{model.ClearYouTube, SettingYouTubeKey, EnvYouTubeKey},
	{model.ClearTrakt, SettingTraktClientID, EnvTraktClientID},
}

// clearable are the valid entries of AdminSettingsUpdate.Clear.
var clearable = map[string]bool{
	model.ClearPublicURL: true, model.ClearTMDB: true, model.ClearTVDB: true, model.ClearTVDBPin: true,
	model.ClearYouTube: true, model.ClearTrakt: true, model.ClearEmail: true, model.ClearTelegram: true,
}

// Settings returns the server settings and integrations without any secret.
func (s *Service) Settings(ctx context.Context) (model.AdminSettings, error) {
	var st model.AdminSettings
	var err error
	st.PublicURL, st.PublicURLSource, err = s.publicURL(ctx)
	if err != nil {
		return model.AdminSettings{}, err
	}
	if st.UpdateCheckEnabled, err = s.updateCheckEnabled(ctx); err != nil {
		return model.AdminSettings{}, err
	}
	states := make(map[string]model.KeyState, len(providerKeys))
	for _, k := range providerKeys {
		if states[k.field], err = s.keyState(ctx, k); err != nil {
			return model.AdminSettings{}, err
		}
	}
	st.Keys = model.AdminKeys{TMDB: states[model.ClearTMDB], TVDB: states[model.ClearTVDB], TVDBPin: states[model.ClearTVDBPin],
		YouTube: states[model.ClearYouTube], Trakt: states[model.ClearTrakt]}
	if st.Email, err = s.emailSettings(ctx); err != nil {
		return model.AdminSettings{}, err
	}
	if st.Telegram, err = s.telegramSettings(ctx); err != nil {
		return model.AdminSettings{}, err
	}
	pub, _, err := s.setting(ctx, SettingVAPIDPublicKey)
	if err != nil {
		return model.AdminSettings{}, err
	}
	st.Push = model.PushSettings{Configured: pub != "", PublicKey: pub}
	return st, nil
}

// publicURL returns the public URL with its source.
func (s *Service) publicURL(ctx context.Context) (string, string, error) {
	if env := s.ops.env.PublicURL; env != "" {
		return env, model.SourceEnv, nil
	}
	v, ok, err := s.setting(ctx, SettingPublicURL)
	if err != nil || !ok {
		return "", model.SourceNone, err
	}
	return v, model.SourceStored, nil
}

// keyState tells where a provider key comes from; a stored key of at least minHintKeyLength
// characters shows its last four.
func (s *Service) keyState(ctx context.Context, k providerKey) (model.KeyState, error) {
	if s.envSet(k.env) {
		return model.KeyState{Source: model.SourceEnv}, nil
	}
	enc, ok, err := s.setting(ctx, k.setting)
	if err != nil || !ok {
		return model.KeyState{Source: model.SourceNone}, err
	}
	st := model.KeyState{Source: model.SourceStored}
	key, err := s.keyring.Decrypt(enc)
	if err != nil {
		// A key file that changed under the stored value (a restore without secrets restores no
		// such values, but a replaced key file can): the value counts as stored, without a hint.
		s.log.WarnContext(ctx, "stored key unreadable", "setting", k.setting, "err", err)
		return st, nil
	}
	if r := []rune(key); len(r) >= minHintKeyLength {
		st.Hint = string(r[len(r)-4:])
	}
	return st, nil
}

func (s *Service) envSet(name string) bool {
	return strings.TrimSpace(s.getenv(name)) != ""
}

// emailSettings returns the e-mail server without its password.
func (s *Service) emailSettings(ctx context.Context) (model.EmailSettings, error) {
	if env := s.ops.env.SMTP; env.Host != "" {
		return model.EmailSettings{Source: model.SourceEnv, Host: env.Host, Port: env.Port, User: env.User, From: env.From, TLS: env.TLS}, nil
	}
	cfg, ok, err := s.storedSMTP(ctx, false)
	if err != nil || !ok {
		return model.EmailSettings{Source: model.SourceNone}, err
	}
	return model.EmailSettings{Source: model.SourceStored, Host: cfg.Host, Port: cfg.Port, User: cfg.User, From: cfg.From, TLS: cfg.TLS}, nil
}

// storedSMTP reads the stored e-mail server; the password only when withPassword. ok is false
// without a stored host.
func (s *Service) storedSMTP(ctx context.Context, withPassword bool) (model.SMTPConfig, bool, error) {
	v, err := s.settingValues(ctx, SettingSMTPHost, SettingSMTPPort, SettingSMTPUser, SettingSMTPFrom, SettingSMTPTLS)
	if err != nil || v[SettingSMTPHost] == "" {
		return model.SMTPConfig{}, false, err
	}
	port, _ := strconv.Atoi(v[SettingSMTPPort])
	cfg := model.SMTPConfig{Host: v[SettingSMTPHost], Port: port, User: v[SettingSMTPUser], From: v[SettingSMTPFrom], TLS: v[SettingSMTPTLS]}
	if cfg.TLS == "" {
		cfg.TLS = email.TLSStartTLS
	}
	if withPassword {
		if cfg.Password, err = s.secretKey(ctx, "", SettingSMTPPassword); err != nil {
			return model.SMTPConfig{}, false, err
		}
	}
	return cfg, true, nil
}

// telegramSettings returns the bot's source and user name without its token.
func (s *Service) telegramSettings(ctx context.Context) (model.TelegramSettings, error) {
	st := model.TelegramSettings{Source: model.SourceNone}
	if s.ops.env.TelegramBotToken != "" {
		st.Source = model.SourceEnv
	} else if _, ok, err := s.setting(ctx, SettingTelegramBotToken); err != nil {
		return model.TelegramSettings{}, err
	} else if ok {
		st.Source = model.SourceStored
	}
	if st.Source == model.SourceNone {
		return st, nil
	}
	name, _, err := s.setting(ctx, SettingTelegramBotUsername)
	if err != nil {
		return model.TelegramSettings{}, err
	}
	st.BotUsername = name
	return st, nil
}

// settingsChange is a validated AdminSettingsUpdate.
type settingsChange struct {
	upd       model.AdminSettingsUpdate
	cleared   map[string]bool
	publicURL string
	keys      map[string]string // field → new key
	email     *model.SMTPConfig // Password unused; upd.Email.Password decides
	botToken  string
}

// UpdateSettings checks the whole request, then the environment, then the new TMDB key and bot
// token against their services, and only then stores everything in one transaction; any error
// changes nothing. Errors: *model.ValidationError, *model.EnvWinsError, model.ErrTMDBKeyInvalid,
// model.ErrTMDBUnreachable, model.ErrTelegramTokenInvalid, model.ErrTelegramUnreachable.
func (s *Service) UpdateSettings(ctx context.Context, upd model.AdminSettingsUpdate) (model.AdminSettings, error) {
	c, verr := validateSettings(upd)
	if verr != nil {
		return model.AdminSettings{}, verr
	}
	if err := s.checkEnvWins(c); err != nil {
		return model.AdminSettings{}, err
	}
	if key, ok := c.keys[model.ClearTMDB]; ok {
		if err := s.tmdb.CheckKey(ctx, key); err != nil {
			return model.AdminSettings{}, err
		}
	}
	botName := ""
	if c.botToken != "" {
		if s.ops.telegram == nil {
			return model.AdminSettings{}, ErrNotConfigured
		}
		name, err := s.ops.telegram.GetMe(ctx, c.botToken)
		if err != nil {
			return model.AdminSettings{}, err
		}
		botName = name
	}
	changed, err := s.storeSettings(ctx, c, botName)
	if err != nil {
		return model.AdminSettings{}, err
	}
	if upd.UpdateCheckEnabled != nil {
		s.forgetUpdateNotice()
		s.syncSchedule(ctx, JobUpdateCheck, jobs.ScheduleUpdate{Enabled: upd.UpdateCheckEnabled})
	}
	s.log.InfoContext(ctx, "settings updated", "fields", changed)
	// A new Trakt client id fills the lists now instead of at the next daily run.
	if _, ok := c.keys[model.ClearTrakt]; ok && s.trakt != nil {
		if err := s.trakt.RefreshTrakt(ctx); err != nil {
			s.log.WarnContext(ctx, "queue trakt refresh for the new client id", "err", err)
		}
	}
	return s.Settings(ctx)
}

// validateSettings checks the form of every field; a field may not be set and cleared at once.
func validateSettings(upd model.AdminSettingsUpdate) (settingsChange, *model.ValidationError) {
	c := settingsChange{upd: upd, cleared: map[string]bool{}, keys: map[string]string{}}
	var verr *model.ValidationError
	for _, f := range upd.Clear {
		if !clearable[f] {
			verr = verr.Add("clear", "invalid_value")
			continue
		}
		c.cleared[f] = true
	}
	if upd.PublicURL != nil {
		u, ok := normalizePublicURL(*upd.PublicURL)
		switch {
		case !ok:
			verr = verr.Add("publicUrl", "invalid_value")
		case c.cleared[model.ClearPublicURL]:
			verr = verr.Add("publicUrl", "not_allowed")
		}
		c.publicURL = u
	}
	values := map[string]*string{
		model.ClearTMDB: upd.Keys.TMDB, model.ClearTVDB: upd.Keys.TVDB, model.ClearTVDBPin: upd.Keys.TVDBPin,
		model.ClearYouTube: upd.Keys.YouTube, model.ClearTrakt: upd.Keys.Trakt,
	}
	for _, k := range providerKeys {
		v := values[k.field]
		if v == nil {
			continue
		}
		key := strings.TrimSpace(*v)
		switch {
		case key == "":
			verr = verr.Add("keys."+k.field, "required")
		case len(key) > maxProviderKey:
			verr = verr.Add("keys."+k.field, "too_long")
		case c.cleared[k.field]:
			verr = verr.Add("keys."+k.field, "not_allowed")
		}
		c.keys[k.field] = key
	}
	if e := upd.Email; e != nil {
		cfg := model.SMTPConfig{Host: strings.TrimSpace(e.Host), Port: e.Port, User: strings.TrimSpace(e.User),
			From: strings.TrimSpace(e.From), TLS: e.TLS}
		verr = validateEmail(verr, cfg, e.Password)
		if c.cleared[model.ClearEmail] {
			verr = verr.Add("email", "not_allowed")
		}
		c.email = &cfg
	}
	if t := upd.Telegram; t != nil {
		token := strings.TrimSpace(t.BotToken)
		switch {
		case token == "":
			verr = verr.Add("telegram.botToken", "required")
		case len(token) > maxBotToken:
			verr = verr.Add("telegram.botToken", "too_long")
		case !telegram.ValidToken(token): // the form the log redaction masks
			verr = verr.Add("telegram.botToken", "invalid_value")
		case c.cleared[model.ClearTelegram]:
			verr = verr.Add("telegram", "not_allowed")
		}
		c.botToken = token
	}
	return c, verr
}

// validateEmail checks the e-mail server fields.
func validateEmail(verr *model.ValidationError, cfg model.SMTPConfig, password *string) *model.ValidationError {
	switch {
	case cfg.Host == "":
		verr = verr.Add("email.host", "required")
	case len(cfg.Host) > maxSMTPHost:
		verr = verr.Add("email.host", "too_long")
	case strings.ContainsAny(cfg.Host, " \t\r\n/:"):
		verr = verr.Add("email.host", "invalid_value")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		verr = verr.Add("email.port", "invalid_value")
	}
	if utf8.RuneCountInString(cfg.User) > maxSMTPUser {
		verr = verr.Add("email.user", "too_long")
	}
	if password != nil && utf8.RuneCountInString(*password) > maxSMTPPassword {
		verr = verr.Add("email.password", "too_long")
	}
	switch n := utf8.RuneCountInString(cfg.From); {
	case n < minSMTPFrom:
		verr = verr.Add("email.from", "required")
	case n > maxSMTPFrom:
		verr = verr.Add("email.from", "too_long")
	default:
		if _, err := mail.ParseAddress(cfg.From); err != nil {
			verr = verr.Add("email.from", "invalid_email")
		}
	}
	switch {
	case cfg.TLS == "":
		verr = verr.Add("email.tls", "required")
	case !validTLS(cfg.TLS):
		verr = verr.Add("email.tls", "invalid_value")
	}
	return verr
}

// validTLS reports whether mode is an encryption mode of the e-mail server (email.DefaultPort).
func validTLS(mode string) bool {
	_, ok := email.DefaultPort(mode)
	return ok
}

// normalizePublicURL trims the URL and its trailing slash; ok is false unless it is an absolute
// http or https URL without query, fragment or credentials.
func normalizePublicURL(raw string) (string, bool) {
	v := strings.TrimRight(strings.TrimSpace(raw), "/")
	if v == "" || len(v) > maxPublicURL {
		return "", false
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" ||
		u.User != nil || strings.Contains(v, "?") || strings.Contains(v, "#") {
		return "", false
	}
	return v, true
}

// checkEnvWins refuses to set or clear a value that comes from the environment.
func (s *Service) checkEnvWins(c settingsChange) error {
	if (c.upd.PublicURL != nil || c.cleared[model.ClearPublicURL]) && s.ops.env.PublicURL != "" {
		return &model.EnvWinsError{Field: "publicUrl"}
	}
	for _, k := range providerKeys {
		if _, set := c.keys[k.field]; (set || c.cleared[k.field]) && s.envSet(k.env) {
			return &model.EnvWinsError{Field: "keys." + k.field}
		}
	}
	if (c.email != nil || c.cleared[model.ClearEmail]) && s.ops.env.SMTP.Host != "" {
		return &model.EnvWinsError{Field: "email"}
	}
	if (c.botToken != "" || c.cleared[model.ClearTelegram]) && s.ops.env.TelegramBotToken != "" {
		return &model.EnvWinsError{Field: "telegram"}
	}
	return nil
}

// storeSettings writes the change in one transaction and returns the changed field names.
func (s *Service) storeSettings(ctx context.Context, c settingsChange, botName string) ([]string, error) {
	now := s.now().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	w := settingsWriter{s: s, ctx: ctx, q: appq.New(tx), now: now}
	var changed []string
	switch {
	case c.upd.PublicURL != nil:
		w.set(SettingPublicURL, c.publicURL)
		changed = append(changed, "publicUrl")
	case c.cleared[model.ClearPublicURL]:
		w.del(SettingPublicURL)
		changed = append(changed, "publicUrl")
	}
	if on := c.upd.UpdateCheckEnabled; on != nil {
		w.set(SettingUpdateCheckEnabled, map[bool]string{true: "1", false: "0"}[*on])
		if !*on {
			w.del(SettingUpdateState) // switched off: no job, no notice, no old state
		}
		changed = append(changed, "updateCheckEnabled")
	}
	for _, k := range providerKeys {
		if key, ok := c.keys[k.field]; ok {
			w.setSecret(k.setting, key)
			changed = append(changed, "keys."+k.field)
		} else if c.cleared[k.field] {
			w.del(k.setting)
			changed = append(changed, "keys."+k.field)
		}
	}
	switch {
	case c.email != nil:
		e := c.email
		w.set(SettingSMTPHost, e.Host)
		w.set(SettingSMTPPort, strconv.Itoa(e.Port))
		w.setOrDel(SettingSMTPUser, e.User)
		w.set(SettingSMTPFrom, e.From)
		w.set(SettingSMTPTLS, e.TLS)
		if p := c.upd.Email.Password; p != nil && *p != "" {
			w.setSecret(SettingSMTPPassword, *p)
		} else if p != nil {
			w.del(SettingSMTPPassword)
		}
		changed = append(changed, "email")
	case c.cleared[model.ClearEmail]:
		for _, key := range []string{SettingSMTPHost, SettingSMTPPort, SettingSMTPUser, SettingSMTPPassword, SettingSMTPFrom, SettingSMTPTLS} {
			w.del(key)
		}
		changed = append(changed, "email")
	}
	switch {
	case c.botToken != "":
		w.setSecret(SettingTelegramBotToken, c.botToken)
		w.setOrDel(SettingTelegramBotUsername, botName)
		changed = append(changed, "telegram")
	case c.cleared[model.ClearTelegram]:
		w.del(SettingTelegramBotToken)
		w.del(SettingTelegramBotUsername)
		changed = append(changed, "telegram")
	}
	if w.err != nil {
		return nil, w.err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit settings: %w", err)
	}
	return changed, nil
}

// settingsWriter writes settings inside one transaction and keeps the first error.
type settingsWriter struct {
	s   *Service
	ctx context.Context
	q   *appq.Queries
	now int64
	err error
}

func (w *settingsWriter) set(key, value string) {
	if w.err == nil {
		if err := w.q.UpsertSetting(w.ctx, appq.UpsertSettingParams{Key: key, Value: value, Now: w.now}); err != nil {
			w.err = fmt.Errorf("store setting %s: %w", key, err)
		}
	}
}

func (w *settingsWriter) setSecret(key, value string) {
	if w.err != nil {
		return
	}
	enc, err := w.s.keyring.Encrypt(value)
	if err != nil {
		w.err = fmt.Errorf("encrypt setting %s: %w", key, err)
		return
	}
	w.set(key, enc)
}

func (w *settingsWriter) setOrDel(key, value string) {
	if value == "" {
		w.del(key)
		return
	}
	w.set(key, value)
}

func (w *settingsWriter) del(key string) {
	if w.err == nil {
		if err := w.q.DeleteSetting(w.ctx, key); err != nil {
			w.err = fmt.Errorf("delete setting %s: %w", key, err)
		}
	}
}

// setting reads a plain setting; ok is false when it is not set.
func (s *Service) setting(ctx context.Context, key string) (string, bool, error) {
	v, err := s.read.GetSetting(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read setting %s: %w", key, err)
	}
	return v, true, nil
}

// settingValues reads plain settings; absent ones are "".
func (s *Service) settingValues(ctx context.Context, keys ...string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		v, _, err := s.setting(ctx, k)
		if err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

// SendTestEmail sends a test mail through the configured e-mail server to to, or to the main
// account when to is empty (A-2, A-6); every call counts against the client's attempt limit.
// Errors: model.ErrTooManyAttempts, *model.ValidationError, model.ErrEmailNotConfigured,
// *model.SendError with model.ErrEmailSendFailed.
func (s *Service) SendTestEmail(ctx context.Context, to string) error {
	if err := s.auth.AllowAttempt(ctx); err != nil {
		return err
	}
	to = strings.TrimSpace(to)
	if to != "" {
		if a, err := mail.ParseAddress(to); err != nil || a.Address != to || a.Name != "" {
			return (&model.ValidationError{}).Add("to", "invalid_email")
		}
	}
	if _, ok, err := s.SMTPConfig(ctx); err != nil {
		return err
	} else if !ok {
		return model.ErrEmailNotConfigured
	}
	if s.ops.email == nil {
		return ErrNotConfigured
	}
	return s.ops.email.SendTestEmail(ctx, to)
}

// RotatePushKeys creates a new VAPID pair, deletes every push subscription and returns the
// settings with the new public key.
func (s *Service) RotatePushKeys(ctx context.Context) (model.AdminSettings, error) {
	if s.ops.push == nil {
		return model.AdminSettings{}, ErrNotConfigured
	}
	if err := s.ops.push.RotateVAPID(ctx); err != nil {
		return model.AdminSettings{}, err
	}
	s.log.InfoContext(ctx, "web push keys rotated")
	return s.Settings(ctx)
}
