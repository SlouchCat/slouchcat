package admin

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

func ptr[T any](v T) *T { return &v }

type fakeTelegram struct {
	err    error
	tokens []string
}

func (f *fakeTelegram) GetMe(_ context.Context, token string) (string, error) {
	f.tokens = append(f.tokens, token)
	if f.err != nil {
		return "", f.err
	}
	return "slouchcat_bot", nil
}

type fakeEmailTester struct {
	err error
	to  []string
}

func (f *fakeEmailTester) SendTestEmail(_ context.Context, to string) error {
	f.to = append(f.to, to)
	return f.err
}

type fakeRotator struct {
	svc   *Service
	calls int
}

func (f *fakeRotator) RotateVAPID(ctx context.Context) error {
	f.calls++
	return f.svc.SetVAPIDKeys(ctx, "BNew-FAKE-pub", "new-FAKE-vapid-private")
}

const (
	fakeBotToken  = "123456:FAKE-bot-secret_xyz"
	fakeTVDBKey   = "FAKE-tvdb-0000-1111-2222-tvdbkey"
	fakeSMTPPass  = "FAKE-smtp-password"
	fakeTraktID   = "FAKE-trakt-client-id-0123456789abcdef"
	fakeVAPIDPriv = "FAKE-vapid-private-key"
)

// rawSetting reads a stored value as it is in the database.
func (f *fixture) rawSetting(t *testing.T, key string) string {
	t.Helper()
	var v string
	_ = f.db.Read.QueryRowContext(context.Background(), "SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	return v
}

// A-2: keys, e-mail server and bot token are stored encrypted, shown only by source and hint, and
// read back through the notify configuration.
func TestUpdateSettingsStoresSecretsEncrypted(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	tg := &fakeTelegram{}
	tr := &fakeTrakt{}
	f.svc.WithTelegram(tg).WithTrakt(tr)

	st, err := f.svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.PublicURLSource != "none" || !st.UpdateCheckEnabled || st.Keys.TMDB.Source != "none" || st.Email.Source != "none" ||
		st.Telegram.Source != "none" || st.Push.Configured {
		t.Fatalf("defaults = %+v", st)
	}

	st, err = f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{
		PublicURL: ptr(" https://media.example.ch/ "),
		Keys:      model.AdminKeysUpdate{TMDB: ptr(" " + fakeKey + " "), TVDB: ptr(fakeTVDBKey), TVDBPin: ptr("1234"), Trakt: ptr(fakeTraktID)},
		Email: &model.EmailSettingsUpdate{Host: "smtp.example.ch", Port: 587, User: "media", Password: ptr(fakeSMTPPass),
			From: "Slouchcat <media@example.ch>", TLS: "starttls"},
		Telegram: &model.TelegramSettingsUpdate{BotToken: fakeBotToken},
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.PublicURL != "https://media.example.ch" || st.PublicURLSource != "stored" {
		t.Errorf("public URL = %q (%s)", st.PublicURL, st.PublicURLSource)
	}
	if st.Keys.TMDB != (model.KeyState{Source: "stored", Hint: fakeKey[len(fakeKey)-4:]}) || st.Keys.TVDBPin != (model.KeyState{Source: "stored"}) ||
		st.Keys.Trakt.Hint != "cdef" || st.Keys.YouTube.Source != "none" {
		t.Errorf("keys = %+v (a short PIN shows no hint)", st.Keys)
	}
	if st.Email != (model.EmailSettings{Source: "stored", Host: "smtp.example.ch", Port: 587, User: "media", From: "Slouchcat <media@example.ch>", TLS: "starttls"}) {
		t.Errorf("email = %+v", st.Email)
	}
	if st.Telegram != (model.TelegramSettings{Source: "stored", BotUsername: "slouchcat_bot"}) || len(tg.tokens) != 1 || tg.tokens[0] != fakeBotToken {
		t.Errorf("telegram = %+v, checked %v", st.Telegram, tg.tokens)
	}
	if len(f.tmdb.checks) != 1 || f.tmdb.checks[0] != fakeKey {
		t.Errorf("TMDB checks = %v, want the trimmed key once", f.tmdb.checks)
	}
	for _, key := range []string{SettingTMDBKey, SettingTVDBKey, SettingTVDBPin, SettingTraktClientID, SettingSMTPPassword, SettingTelegramBotToken} {
		if v := f.rawSetting(t, key); !strings.HasPrefix(v, "v1:") {
			t.Errorf("setting %s = %q, want encrypted", key, v)
		}
	}
	if got, _ := f.svc.TMDBKey(ctx); got != fakeKey {
		t.Errorf("TMDBKey = %q", got)
	}
	if key, pin, _ := f.svc.TVDBKey(ctx); key != fakeTVDBKey || pin != "1234" {
		t.Errorf("TVDBKey = %q, %q", key, pin)
	}
	if got, _ := f.svc.TraktClientID(ctx); got != fakeTraktID || tr.refreshes != 1 {
		t.Errorf("TraktClientID = %q, refreshes %d; want the new id and one refresh", got, tr.refreshes)
	}
	cfg, ok, err := f.svc.SMTPConfig(ctx)
	if err != nil || !ok || cfg != (model.SMTPConfig{Host: "smtp.example.ch", Port: 587, User: "media", Password: fakeSMTPPass,
		From: "Slouchcat <media@example.ch>", TLS: "starttls"}) {
		t.Errorf("SMTPConfig = %+v, %v, %v", cfg, ok, err)
	}
	if tok, _ := f.svc.TelegramToken(ctx); tok != fakeBotToken {
		t.Errorf("TelegramToken = %q", tok)
	}
	if u, _ := f.svc.PublicURL(ctx); u != "https://media.example.ch" {
		t.Errorf("PublicURL = %q", u)
	}

	// A new e-mail server without password keeps the stored one; clearing removes everything.
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{Email: &model.EmailSettingsUpdate{
		Host: "mail.example.ch", Port: 465, From: "media@example.ch", TLS: "tls"}}); err != nil {
		t.Fatal(err)
	}
	if cfg, _, _ := f.svc.SMTPConfig(ctx); cfg.Password != fakeSMTPPass || cfg.Host != "mail.example.ch" || cfg.User != "" {
		t.Errorf("SMTPConfig after the change = %+v, want the kept password", cfg)
	}
	st, err = f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{Clear: []string{"email", "telegram", "tvdb", "tvdbPin", "publicUrl"}})
	if err != nil {
		t.Fatal(err)
	}
	if st.Email.Source != "none" || st.Telegram.Source != "none" || st.Keys.TVDB.Source != "none" || st.PublicURLSource != "none" ||
		st.Keys.TMDB.Source != "stored" {
		t.Errorf("after clearing: %+v", st)
	}
	if _, ok, _ := f.svc.SMTPConfig(ctx); ok {
		t.Error("SMTPConfig still configured after clearing")
	}
	for _, key := range []string{SettingSMTPPassword, SettingTelegramBotToken, SettingTelegramBotUsername, SettingTVDBKey} {
		if v := f.rawSetting(t, key); v != "" {
			t.Errorf("setting %s left after clearing", key)
		}
	}
	for _, secret := range []string{fakeKey, fakeTVDBKey, fakeSMTPPass, fakeBotToken, fakeTraktID} {
		if strings.Contains(f.logs.String(), secret) {
			t.Errorf("log carries %q", secret)
		}
	}
}

// Invalid input, values of the environment and a rejected key or token change nothing.
func TestUpdateSettingsRefusals(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	tg := &fakeTelegram{}
	f.svc.WithTelegram(tg)
	f.env[EnvTVDBKey] = "tvdb-FAKE-env"
	f.svc.WithNotifyEnv(NotifyEnv{SMTP: model.SMTPConfig{Host: "env.example.ch", Port: 587, TLS: "starttls", Password: "env-FAKE-pass"}})
	email := &model.EmailSettingsUpdate{Host: "smtp.example.ch", Port: 587, From: "media@example.ch", TLS: "starttls"}

	for name, tc := range map[string]struct {
		upd   model.AdminSettingsUpdate
		field string // validation field:code, or env_wins field
		want  error
	}{
		"empty public URL":      {upd: model.AdminSettingsUpdate{PublicURL: ptr("")}, field: "publicUrl:invalid_value"},
		"public URL with query": {upd: model.AdminSettingsUpdate{PublicURL: ptr("https://x.example/?a=1")}, field: "publicUrl:invalid_value"},
		"ftp public URL":        {upd: model.AdminSettingsUpdate{PublicURL: ptr("ftp://x.example")}, field: "publicUrl:invalid_value"},
		"set and clear":         {upd: model.AdminSettingsUpdate{PublicURL: ptr("https://x.example"), Clear: []string{"publicUrl"}}, field: "publicUrl:not_allowed"},
		"unknown clear":         {upd: model.AdminSettingsUpdate{Clear: []string{"everything"}}, field: "clear:invalid_value"},
		"blank key":             {upd: model.AdminSettingsUpdate{Keys: model.AdminKeysUpdate{YouTube: ptr("  ")}}, field: "keys.youtube:required"},
		"bad port": {upd: model.AdminSettingsUpdate{Email: &model.EmailSettingsUpdate{Host: "smtp.example.ch", Port: 0, From: "a@b.ch", TLS: "tls"}},
			field: "email.port:invalid_value"},
		"bad sender": {upd: model.AdminSettingsUpdate{Email: &model.EmailSettingsUpdate{Host: "smtp.example.ch", Port: 25, From: "nobody", TLS: "none"}},
			field: "email.from:invalid_email"},
		"bad tls": {upd: model.AdminSettingsUpdate{Email: &model.EmailSettingsUpdate{Host: "smtp.example.ch", Port: 25, From: "a@b.ch", TLS: "ssl"}},
			field: "email.tls:invalid_value"},
		"bad token":         {upd: model.AdminSettingsUpdate{Telegram: &model.TelegramSettingsUpdate{BotToken: "not a token"}}, field: "telegram.botToken:invalid_value"},
		"tvdb from env":     {upd: model.AdminSettingsUpdate{Keys: model.AdminKeysUpdate{TVDB: ptr(fakeTVDBKey)}}, field: "env:keys.tvdb"},
		"tvdb pin from env": {upd: model.AdminSettingsUpdate{Clear: []string{"tvdbPin"}}, field: "env:keys.tvdbPin"},
		"email from env":    {upd: model.AdminSettingsUpdate{Email: email}, field: "env:email"},
		"email clear env":   {upd: model.AdminSettingsUpdate{Clear: []string{"email"}}, field: "env:email"},
		"tmdb rejected": {upd: model.AdminSettingsUpdate{Keys: model.AdminKeysUpdate{TMDB: ptr(fakeKey)}, UpdateCheckEnabled: ptr(false)},
			want: model.ErrTMDBKeyInvalid},
		"telegram rejected": {upd: model.AdminSettingsUpdate{Telegram: &model.TelegramSettingsUpdate{BotToken: fakeBotToken},
			Keys: model.AdminKeysUpdate{YouTube: ptr("yt-FAKE-key-0000000000")}}, want: model.ErrTelegramTokenInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			f.tmdb.err, tg.err = model.ErrTMDBKeyInvalid, model.ErrTelegramTokenInvalid
			_, err := f.svc.UpdateSettings(ctx, tc.upd)
			var verr *model.ValidationError
			var env *model.EnvWinsError
			switch {
			case strings.HasPrefix(tc.field, "env:"):
				if !errors.As(err, &env) || "env:"+env.Field != tc.field {
					t.Fatalf("err = %v, want env_wins %s", err, tc.field)
				}
			case tc.field != "":
				if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0].Field+":"+verr.Fields[0].Code != tc.field {
					t.Fatalf("err = %v, want %s", err, tc.field)
				}
			case !errors.Is(err, tc.want):
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if n := f.settings(t); n != 0 {
				t.Errorf("settings stored after a refusal: %d", n)
			}
		})
	}
	st, err := f.svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Keys.TVDB.Source != "env" || st.Keys.TVDBPin.Source != "env" || st.Keys.TVDB.Hint != "" ||
		st.Email != (model.EmailSettings{Source: "env", Host: "env.example.ch", Port: 587, TLS: "starttls"}) {
		t.Errorf("settings with environment = %+v", st)
	}
	if cfg, ok, _ := f.svc.SMTPConfig(ctx); !ok || cfg.Password != "env-FAKE-pass" {
		t.Errorf("SMTPConfig from the environment = %+v", cfg)
	}
}

// Switching the update check off drops its state.
func TestUpdateSettingsUpdateCheckSwitch(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	if err := f.svc.write.UpsertSetting(ctx, upsert(SettingUpdateState, `{"latestVersion":"9.9.9","url":"https://x.example","checkedAt":1}`)); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{UpdateCheckEnabled: ptr(false)})
	if err != nil {
		t.Fatal(err)
	}
	if st.UpdateCheckEnabled || f.rawSetting(t, SettingUpdateState) != "" {
		t.Errorf("enabled %v, state %q after switching off", st.UpdateCheckEnabled, f.rawSetting(t, SettingUpdateState))
	}
	if st, err = f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{UpdateCheckEnabled: ptr(true)}); err != nil || !st.UpdateCheckEnabled {
		t.Errorf("switched on: %+v, %v", st, err)
	}
}

// The test mail needs an e-mail server, validates the recipient and passes it to notify.
func TestSendTestEmail(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	tester := &fakeEmailTester{}
	f.svc.WithEmailTester(tester)
	if err := f.svc.SendTestEmail(ctx, ""); !errors.Is(err, model.ErrEmailNotConfigured) {
		t.Errorf("without a server: %v, want ErrEmailNotConfigured", err)
	}
	if _, err := f.svc.UpdateSettings(ctx, model.AdminSettingsUpdate{Email: &model.EmailSettingsUpdate{
		Host: "smtp.example.ch", Port: 587, From: "media@example.ch", TLS: "starttls"}}); err != nil {
		t.Fatal(err)
	}
	var verr *model.ValidationError
	if err := f.svc.SendTestEmail(ctx, "Marco <marco@example.ch>"); !errors.As(err, &verr) {
		t.Errorf("recipient with a name: %v, want invalid_email", err)
	}
	if err := f.svc.SendTestEmail(ctx, ""); err != nil {
		t.Fatal(err)
	}
	tester.err = &model.SendError{Err: model.ErrEmailSendFailed, Detail: "550 mailbox unavailable"}
	if err := f.svc.SendTestEmail(ctx, " marco@example.ch "); !errors.Is(err, model.ErrEmailSendFailed) {
		t.Errorf("refused mail: %v, want ErrEmailSendFailed", err)
	}
	if strings.Join(tester.to, ",") != ",marco@example.ch" {
		t.Errorf("test mails to %q", tester.to)
	}
	if err := newFixtureWithSMTP(t).svc.SendTestEmail(ctx, ""); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("without the tester: %v, want ErrNotConfigured", err)
	}
}

func newFixtureWithSMTP(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.svc.WithNotifyEnv(NotifyEnv{SMTP: model.SMTPConfig{Host: "env.example.ch", Port: 25, TLS: "none"}})
	return f
}

// The VAPID pair is stored with the private key encrypted; the rotation goes through notify.
func TestVAPIDKeysAndRotation(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	if pub, priv, err := f.svc.VAPIDKeys(ctx); err != nil || pub != "" || priv != "" {
		t.Fatalf("no pair yet: %q, %q, %v", pub, priv, err)
	}
	if err := f.svc.SetVAPIDKeys(ctx, "BPub-FAKE", fakeVAPIDPriv); err != nil {
		t.Fatal(err)
	}
	if pub, priv, _ := f.svc.VAPIDKeys(ctx); pub != "BPub-FAKE" || priv != fakeVAPIDPriv {
		t.Errorf("pair = %q, %q", pub, priv)
	}
	if v := f.rawSetting(t, SettingVAPIDPrivateKey); !strings.HasPrefix(v, "v1:") {
		t.Errorf("private key stored as %q, want encrypted", v)
	}
	if _, err := f.svc.RotatePushKeys(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("rotation without notify: %v, want ErrNotConfigured", err)
	}
	rot := &fakeRotator{svc: f.svc}
	st, err := f.svc.WithPushRotator(rot).RotatePushKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rot.calls != 1 || !st.Push.Configured || st.Push.PublicKey != "BNew-FAKE-pub" {
		t.Errorf("after the rotation: %+v (%d calls)", st.Push, rot.calls)
	}
}
