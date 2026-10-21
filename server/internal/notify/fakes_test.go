package notify

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify/push"
	"github.com/slouchcat/slouchcat/server/internal/notify/telegram"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Fakes of the server settings and the senders for the channel tests.

const fakeSMTPPassword = "fake-smtp-Passw0rd"

type fakeConfig struct {
	mu        sync.Mutex
	smtp      model.SMTPConfig
	smtpOK    bool
	token     string
	pub, priv string
	publicURL string
	setVAPID  int
	err       error // returned by every read
}

func (c *fakeConfig) SMTPConfig(context.Context) (model.SMTPConfig, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.smtp, c.smtpOK, c.err
}

func (c *fakeConfig) TelegramToken(context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token, c.err
}

func (c *fakeConfig) VAPIDKeys(context.Context) (string, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pub, c.priv, c.err
}

func (c *fakeConfig) SetVAPIDKeys(_ context.Context, pub, priv string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pub, c.priv = pub, priv
	c.setVAPID++
	return nil
}

func (c *fakeConfig) PublicURL(context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.publicURL, c.err
}

func (c *fakeConfig) set(f func(c *fakeConfig)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f(c)
}

type sentMail struct {
	smtp                    model.SMTPConfig
	to, subject, text, html string
}

type fakeMailer struct {
	mu    sync.Mutex
	mails []sentMail
	err   error
}

func (m *fakeMailer) Mail(_ context.Context, cfg model.SMTPConfig, to, subject, text, html string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.mails = append(m.mails, sentMail{cfg, to, subject, text, html})
	return nil
}

func (m *fakeMailer) sent() []sentMail {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.mails)
}

func (m *fakeMailer) fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

type sentPush struct {
	vapid    push.VAPID
	endpoint string
	payload  string
	ttl      time.Duration
}

type fakePusher struct {
	mu     sync.Mutex
	pushes []sentPush
	errs   map[string]error // by endpoint
}

func (p *fakePusher) Push(_ context.Context, v push.VAPID, sub push.Subscription, payload []byte, ttl time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.errs[sub.Endpoint]; err != nil {
		return err
	}
	p.pushes = append(p.pushes, sentPush{v, sub.Endpoint, string(payload), ttl})
	return nil
}

func (p *fakePusher) sent() []sentPush {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.pushes)
}

type sentTelegram struct {
	token string
	chat  int64
	text  string
}

type fakeTelegram struct {
	mu       sync.Mutex
	messages []sentTelegram
	name     string
	meErr    error
	updates  []telegram.Update
	offsets  []int64
}

func (f *fakeTelegram) GetMe(context.Context, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.name, f.meErr
}

func (f *fakeTelegram) SendMessage(_ context.Context, token string, chatID int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, sentTelegram{token, chatID, text})
	return nil
}

func (f *fakeTelegram) GetUpdates(_ context.Context, _ string, offset int64) ([]telegram.Update, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.offsets = append(f.offsets, offset)
	var out []telegram.Update
	for _, u := range f.updates {
		if u.UpdateID >= offset {
			out = append(out, u)
		}
	}
	return out, nil
}

func (f *fakeTelegram) sent() []sentTelegram {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.messages)
}

// chanEnv is a service with the channels: queue, settings and fake senders on the vistest fixture
// with series and guest account.
type chanEnv struct {
	*env
	queue  *jobs.Queue
	cfg    *fakeConfig
	mailer *fakeMailer
	pusher *fakePusher
	tg     *fakeTelegram
}

const testBotToken = "7000000001:AAE-fake_bot_Token"

func newChanEnv(t *testing.T) *chanEnv {
	t.Helper()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddAudio(t)
	f.AddGuest(t)
	e := &chanEnv{
		env: &env{f: f, now: time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)},
		cfg: &fakeConfig{
			smtp:   model.SMTPConfig{Host: "127.0.0.1", Port: 2525, User: "mailer", Password: fakeSMTPPassword, From: "slouchcat@example.test", TLS: "none"},
			smtpOK: true, publicURL: "https://slouchcat.example.test",
		},
		mailer: &fakeMailer{}, pusher: &fakePusher{errs: map[string]error{}}, tg: &fakeTelegram{name: "slouchcat_bot"},
	}
	clock := func() time.Time { return e.now }
	e.queue = jobs.NewQueue(f.DB.Write, clock)
	e.svc = New(f.DB, visibility.New(f.DB, vistest.Logger()), vistest.Logger(), WithClock(clock), WithQueue(e.queue),
		WithSenders(Senders{Email: e.mailer, Push: e.pusher, Telegram: e.tg}))
	e.svc.SetConfig(e.cfg)
	for _, acc := range []model.Account{f.Account, f.Guest} {
		if err := e.svc.EnsureDefaults(context.Background(), acc.ID); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func session(acc model.Account, p model.Profile) model.Session {
	return model.Session{Account: acc, Profile: p}
}

func (e *chanEnv) owner() model.Session { return session(e.f.Account, e.f.P18) }
func (e *chanEnv) guest() model.Session { return session(e.f.Guest, e.f.PG) }

// enable switches a channel of the session's account on with the events.
func (e *chanEnv) enable(t *testing.T, s model.Session, channel string, events ...string) {
	t.Helper()
	if _, err := e.svc.UpdateChannel(context.Background(), s, channel, model.ChannelSettingUpdate{Enabled: true, Events: events}); err != nil {
		t.Fatalf("enable %s: %v", channel, err)
	}
}

// linkChat links a Telegram chat to an account directly.
func (e *chanEnv) linkChat(t *testing.T, accountID, chatID int64) {
	t.Helper()
	e.exec(t, `UPDATE account_channels SET config = json_object('chat_id', ?) WHERE account_id = ? AND channel = 'telegram'`, chatID, accountID)
}

// drain runs every runnable notify.deliver job like a worker and returns how many ran.
func (e *chanEnv) drain(t *testing.T) int {
	t.Helper()
	ctx := context.Background()
	n := 0
	for {
		j, err := e.queue.Claim(ctx, "test", []string{JobDeliver})
		if err != nil {
			t.Fatal(err)
		}
		if j == nil {
			return n
		}
		n++
		if err := e.svc.deliver(ctx, j); err != nil {
			if ferr := e.queue.Fail(ctx, "test", j, err); ferr != nil {
				t.Fatal(ferr)
			}
			continue
		}
		if err := e.queue.Complete(ctx, "test", j.ID); err != nil {
			t.Fatal(err)
		}
	}
}

type deliveryRow struct {
	account        int64
	profile        sql.NullInt64
	channel, event string
	subject, state string
	attempts       int
	lastError      sql.NullString
}

func (e *chanEnv) deliveries(t *testing.T) []deliveryRow {
	t.Helper()
	rows, err := e.f.DB.Read.Query(`SELECT account_id, profile_id, channel, event, subject, state, attempts, last_error FROM deliveries ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []deliveryRow
	for rows.Next() {
		var d deliveryRow
		if err := rows.Scan(&d.account, &d.profile, &d.channel, &d.event, &d.subject, &d.state, &d.attempts, &d.lastError); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (e *chanEnv) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.f.DB.Read.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

var errFake = errors.New("fake failure")

// subscriptionInput is a browser subscription with valid keys.
func subscriptionInput(t *testing.T, endpoint string) model.PushSubscriptionInput {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	return model.PushSubscriptionInput{Endpoint: endpoint, P256dh: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		Auth: base64.RawURLEncoding.EncodeToString(auth)}
}

// subscribe stores a push subscription for the session's account and returns its id.
func (e *chanEnv) subscribe(t *testing.T, s model.Session, endpoint string) int64 {
	t.Helper()
	sub, err := e.svc.CreatePushSubscription(context.Background(), s, subscriptionInput(t, endpoint))
	if err != nil {
		t.Fatal(err)
	}
	return sub.ID
}
