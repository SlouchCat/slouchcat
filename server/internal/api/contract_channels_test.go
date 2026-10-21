package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify"
	"github.com/slouchcat/slouchcat/server/internal/notify/push"
	"github.com/slouchcat/slouchcat/server/internal/notify/telegram"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Contract tests of the M8 notification channels and announcements (plan 10.2, A-6, A-7): the
// fixture's notify service has an e-mail server and a VAPID pair (no Telegram bot) and fake senders.

// A valid browser subscription (P-256 point and 16-byte auth secret, base64url).
const (
	testP256dh   = "BEfrmZHP6SYUEsVBPxAbkpY8EhbW2KpNf6TIOIwN_ZTdaYwAb5LDmKu6Rg-pCiCCMM-YHekxm4JKercDc2g1Vco"
	testPushAuth = "dzBhFpaZ8b5MVIh_Y2-u2A"
)

func pushSubscriptionBody(endpoint string) string {
	return `{"endpoint":"` + endpoint + `","keys":{"p256dh":"` + testP256dh + `","auth":"` + testPushAuth + `"},"deviceName":"Registry"}`
}

// contractNotifyConfig is the server side of the channels: an e-mail server, a VAPID pair kept in
// memory, no Telegram bot.
type contractNotifyConfig struct {
	mu        sync.Mutex
	pub, priv string
}

func (*contractNotifyConfig) SMTPConfig(context.Context) (model.SMTPConfig, bool, error) {
	return model.SMTPConfig{Host: "mail.example.test", Port: 587, User: "mailer", Password: "contract-smtp-password",
		From: "slouchcat@example.test", TLS: "starttls"}, true, nil
}

func (*contractNotifyConfig) TelegramToken(context.Context) (string, error) { return "", nil }

func (c *contractNotifyConfig) VAPIDKeys(context.Context) (string, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pub, c.priv, nil
}

func (c *contractNotifyConfig) SetVAPIDKeys(_ context.Context, pub, priv string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pub, c.priv = pub, priv
	return nil
}

func (*contractNotifyConfig) PublicURL(context.Context) (string, error) { return "", nil }

// contractSenders records what the channels send.
type contractSenders struct {
	mu     sync.Mutex
	mails  []string // recipient: subject
	pushes []string // endpoints
}

func (s *contractSenders) Mail(_ context.Context, _ model.SMTPConfig, to, subject, _, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mails = append(s.mails, to+": "+subject)
	return nil
}

func (s *contractSenders) Push(_ context.Context, _ push.VAPID, sub push.Subscription, _ []byte, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pushes = append(s.pushes, sub.Endpoint)
	return nil
}

func (*contractSenders) GetMe(context.Context, string) (string, error) {
	return "", telegram.ErrUnauthorized
}
func (*contractSenders) SendMessage(context.Context, string, int64, string) error {
	return telegram.ErrUnauthorized
}

func (*contractSenders) GetUpdates(context.Context, string, int64) ([]telegram.Update, error) {
	return nil, nil
}

func (s *contractSenders) sentMails() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.mails)
}

// newContractNotify returns the notify service of the contract fixtures with its senders.
func newContractNotify(f *vistest.Fixture, log *slog.Logger, queue *jobs.Queue) (*notify.Service, *contractSenders) {
	senders := &contractSenders{}
	svc := notify.New(f.DB, visibility.New(f.DB, log), log, notify.WithQueue(queue),
		notify.WithSenders(notify.Senders{Email: senders, Push: senders, Telegram: senders}))
	svc.SetConfig(&contractNotifyConfig{})
	return svc, senders
}

// drainDeliveries runs the queued notify.deliver jobs of the fixture like the worker.
func drainDeliveries(t *testing.T, pf profileFixture) {
	t.Helper()
	ctx := context.Background()
	w := jobs.NewWorker(pf.queue, "contract-notify", vistest.Logger())
	pf.notifySvc.RegisterJobs(w)
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var n int
		if err := pf.f.DB.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ? AND state IN ('queued', 'running')`, notify.JobDeliver).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
}

// A-7, plan 10.2: published announcements reach every profile until that profile acknowledges
// them; drafts reach nobody; a guest account with e-mail on (default) gets the mail, the main
// account (e-mail off) none. Answers carry no title id.
func TestContractAnnouncements(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	titles := f.All()
	post := func(token, path, body string) int {
		t.Helper()
		return send(t, h, http.MethodPost, BasePath+path, token, body, nil).Code
	}
	var draft, first gen.Announcement
	rec := send(t, h, http.MethodPost, BasePath+"/admin/announcements", "p18", `{"title":"Entwurf","body":"Noch nicht"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create draft: %d %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &draft)
	rec = send(t, h, http.MethodPost, BasePath+"/admin/announcements", "p18", `{"title":"Wartung","body":"Samstag offline","publish":true}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create published: %d %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &first)
	if first.PublishedAt == nil || draft.PublishedAt != nil || first.CreatedBy == nil || *first.CreatedBy != f.Account.Email {
		t.Errorf("announcements = %+v / %+v", draft, first)
	}
	open := func(token string) []int64 {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/announcements", token, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: GET /announcements = %d %s", token, rec.Code, rec.Body.String())
		}
		if found := itemIDsIn(t, rec, titles); len(found) != 0 {
			t.Errorf("%s: title ids %v", token, found)
		}
		var list []gen.ProfileAnnouncement
		decode(t, rec, &list)
		var ids []int64
		for _, a := range list {
			ids = append(ids, a.Id)
		}
		return ids
	}
	for _, token := range []string{"p18", "p6", "p0", "pg", "pg18"} {
		if got := open(token); !slices.Equal(got, []int64{first.Id}) {
			t.Errorf("%s open = %v", token, got)
		}
	}
	if code := post("p6", fmt.Sprintf("/announcements/%d/ack", first.Id), ""); code != http.StatusNoContent {
		t.Errorf("p6 ack = %d", code)
	}
	if code := post("p6", fmt.Sprintf("/announcements/%d/ack", first.Id), ""); code != http.StatusNoContent {
		t.Errorf("p6 second ack = %d", code)
	}
	if code := post("p6", fmt.Sprintf("/announcements/%d/ack", draft.Id), ""); code != http.StatusNotFound {
		t.Errorf("ack of a draft = %d", code)
	}
	if got := open("p6"); len(got) != 0 {
		t.Errorf("p6 after ack = %v", got)
	}
	if got := open("p12"); !slices.Equal(got, []int64{first.Id}) {
		t.Errorf("p12 after p6's ack = %v", got)
	}
	// Publishing the draft adds it; a published one cannot change (409).
	if code := post("p18", fmt.Sprintf("/admin/announcements/%d/publish", draft.Id), ""); code != http.StatusOK {
		t.Fatalf("publish = %d", code)
	}
	if got := open("pg"); !slices.Equal(got, []int64{draft.Id, first.Id}) {
		t.Errorf("pg open = %v", got)
	}
	rec = send(t, h, http.MethodPut, fmt.Sprintf("%s/admin/announcements/%d", BasePath, draft.Id), "p18", `{"title":"x","body":"y"}`, nil)
	if rec.Code != http.StatusConflict || problemCode(t, rec) != "announcement_published" {
		t.Errorf("update of a published one: %d %s", rec.Code, rec.Body.String())
	}
	rec = send(t, h, http.MethodGet, BasePath+"/admin/announcements", "p18", "", nil)
	var all []gen.Announcement
	decode(t, rec, &all)
	if len(all) != 2 || all[0].Id != first.Id || all[0].AcknowledgedCount != 1 || all[1].AcknowledgedCount != 0 { // newest first
		t.Errorf("admin list = %+v", all)
	}
	drainDeliveries(t, pf)
	mails := pf.senders.sentMails()
	slices.Sort(mails)
	if want := []string{f.Guest.Email + ": Entwurf", f.Guest.Email + ": Wartung"}; !slices.Equal(mails, want) {
		t.Errorf("mails = %v, want %v", mails, want)
	}
}

// A-6: the channels are account operations (no profile needed); the rules of the contract hold and
// no answer carries the endpoint or the keys of a push subscription.
func TestContractChannels(t *testing.T) {
	pf := newProfileFixture(t)
	h := pf.h
	rec := send(t, h, http.MethodGet, BasePath+"/me/channels", "none", "", nil)
	var settings gen.ChannelSettings
	decode(t, rec, &settings)
	if rec.Code != http.StatusOK || len(settings.Channels) != 3 || settings.Channels[0].Email == nil ||
		string(settings.Channels[0].Email.Address) != pf.f.Account.Email || !settings.Channels[0].Available || settings.Channels[2].Available {
		t.Fatalf("channels = %d %s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct {
		token, path, body string
		status            int
		code              string
	}{
		{"pg", "/me/channels/email", `{"enabled":true,"events":["errors"]}`, http.StatusUnprocessableEntity, "event_not_allowed"},
		{"p18", "/me/channels/telegram", `{"enabled":true,"events":["errors"]}`, http.StatusUnprocessableEntity, "channel_unavailable"},
		{"p18", "/me/channels/email", `{"enabled":true,"events":["errors"],"email":{"address":"owner-inbox@example.test"}}`, http.StatusOK, ""},
	} {
		rec := send(t, h, http.MethodPut, BasePath+tc.path, tc.token, tc.body, nil)
		if rec.Code != tc.status || (tc.code != "" && problemCode(t, rec) != tc.code) {
			t.Errorf("%s PUT %s: %d %s", tc.token, tc.path, rec.Code, rec.Body.String())
		}
	}
	// Test messages: e-mail to the chosen address, push without subscription is disabled.
	if rec := send(t, h, http.MethodPost, BasePath+"/me/channels/email/test", "none", "", nil); rec.Code != http.StatusNoContent {
		t.Errorf("test e-mail: %d %s", rec.Code, rec.Body.String())
	}
	if mails := pf.senders.sentMails(); len(mails) != 1 || !strings.HasPrefix(mails[0], "owner-inbox@example.test: ") {
		t.Errorf("mails = %v", mails)
	}
	if rec := send(t, h, http.MethodPut, BasePath+"/me/channels/push", "p18", `{"enabled":true,"events":["errors"]}`, nil); rec.Code != http.StatusOK {
		t.Fatalf("push on: %d %s", rec.Code, rec.Body.String())
	}
	rec = send(t, h, http.MethodPost, BasePath+"/me/channels/push/test", "p18", "", nil)
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "channel_disabled" {
		t.Errorf("push test without subscription: %d %s", rec.Code, rec.Body.String())
	}
	// Push subscriptions: https only outside development, secrets never answered.
	rec = send(t, h, http.MethodPost, BasePath+"/me/push-subscriptions", "p18", pushSubscriptionBody("http://push.example.test/x"), nil)
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "validation_failed" {
		t.Errorf("http endpoint: %d %s", rec.Code, rec.Body.String())
	}
	endpoint := "https://push.example.test/send/capability-secret-123"
	rec = send(t, h, http.MethodPost, BasePath+"/me/push-subscriptions", "p18", pushSubscriptionBody(endpoint), nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("subscribe: %d %s", rec.Code, rec.Body.String())
	}
	var sub gen.PushSubscriptionSummary
	decode(t, rec, &sub)
	rec = send(t, h, http.MethodGet, BasePath+"/me/channels", "p18", "", nil)
	for _, secret := range []string{"capability-secret-123", testP256dh, testPushAuth} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Errorf("GET /me/channels carries %q", secret)
		}
	}
	if rec := send(t, h, http.MethodPost, BasePath+"/me/channels/push/test", "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Errorf("push test: %d %s", rec.Code, rec.Body.String())
	}
	path := fmt.Sprintf("%s/me/push-subscriptions/%d", BasePath, sub.Id)
	if rec := send(t, h, http.MethodDelete, path, "pg", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("guest deletes the owner's subscription: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodDelete, path, "p12", "", nil); rec.Code != http.StatusNoContent {
		t.Errorf("another profile of the account deletes it: %d %s", rec.Code, rec.Body.String())
	}
	var key gen.VapidKey
	rec = send(t, h, http.MethodGet, BasePath+"/push/vapid-key", "pg", "", nil)
	decode(t, rec, &key)
	if rec.Code != http.StatusOK || len(key.PublicKey) != 87 {
		t.Errorf("vapid key: %d %s", rec.Code, rec.Body.String())
	}
}
