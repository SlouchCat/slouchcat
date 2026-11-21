package notify

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify/push"
)

// Concept 27 stage 1: the message «Neues Gerät verbunden» (K-7) and the push of the Android app
// (section 7.3, ADR 0044).

// device_linked goes to every switched-on channel of the account, whatever events the channel
// chose, and to no other account; it names time and kind and marks the name as the device's claim.
func TestDeviceLinkedGoesToEveryChannel(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	e.enable(t, e.owner(), model.ChannelEmail, model.EventAnnouncements)
	e.enable(t, e.owner(), model.ChannelPush)
	e.subscribe(t, e.owner(), "https://push.example.test/owner")
	e.enable(t, e.guest(), model.ChannelEmail, model.EventAnnouncements)
	e.exec(t, `UPDATE profiles SET language = 'de' WHERE account_id = ?`, e.f.Account.ID)
	at := time.Date(2026, 11, 21, 9, 30, 0, 0, time.UTC)
	if err := e.svc.DeviceLinked(ctx, e.f.Account.ID, "tv", "Wohnzimmer", "Android TV 14", at); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	d := e.deliveries(t)
	if len(d) != 2 {
		t.Fatalf("deliveries = %+v, want e-mail and push of the owner", d)
	}
	for _, row := range d {
		if row.account != e.f.Account.ID || row.event != model.EventDeviceLinked || row.state != "sent" || row.profile.Valid {
			t.Errorf("delivery %+v", row)
		}
	}
	subject := localeText(t, "de", "device_linked_subject", nil)
	body := localeText(t, "de", "device_linked_body", vars{"time": "2026-11-21 09:30 UTC", "kind": localeText(t, "de", "device_kind_tv", nil),
		"name": "Wohnzimmer", "system": "Android TV 14"})
	mails := e.mailer.sent()
	if len(mails) != 1 || mails[0].subject != subject || !strings.Contains(mails[0].text, body) {
		t.Fatalf("mails = %+v, want %q / %q", mails, subject, body)
	}
	if !strings.Contains(body, "«Wohnzimmer»") || !strings.Contains(body, "Fernseher") {
		t.Errorf("body %q names neither the claimed name nor the kind", body)
	}
	pushes := e.pusher.sent()
	if len(pushes) != 1 || !strings.Contains(pushes[0].payload, subject) {
		t.Errorf("pushes = %+v", pushes)
	}
	if !strings.Contains(pushes[0].payload, "/settings/devices") {
		t.Errorf("push without the link to «Meine Geräte»: %s", pushes[0].payload)
	}
}

// device_linked is a security message (security review M-1 of stage 1): it reaches every channel
// the account set up although all are switched off, and the e-mail goes to the account's own
// address, not to one set for the channel (which a stolen session could have changed).
func TestDeviceLinkedIgnoresSwitchedOffChannels(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	e.cfg.set(func(c *fakeConfig) { c.token = testBotToken })
	e.subscribe(t, e.owner(), "https://push.example.test/owner")
	e.linkChat(t, e.f.Account.ID, 4242)
	other := "elsewhere@example.test"
	for _, channel := range []string{model.ChannelEmail, model.ChannelPush, model.ChannelTelegram} {
		upd := model.ChannelSettingUpdate{Enabled: false}
		if channel == model.ChannelEmail {
			upd.EmailAddress = &other
		}
		if _, err := e.svc.UpdateChannel(ctx, e.owner(), channel, upd); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.svc.DeviceLinked(ctx, e.f.Account.ID, "tv", "Stube", "Android TV 14", time.Now()); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	got := map[string]string{}
	for _, d := range e.deliveries(t) {
		got[d.channel] = d.state
	}
	want := map[string]string{model.ChannelEmail: "sent", model.ChannelPush: "sent", model.ChannelTelegram: "sent"}
	if !maps.Equal(got, want) {
		t.Fatalf("deliveries %v, want %v", got, want)
	}
	if mails := e.mailer.sent(); len(mails) != 1 || !strings.EqualFold(mails[0].to, e.f.Account.Email) {
		t.Errorf("mails %+v, want one to the account's own address", mails)
	}
	if len(e.pusher.sent()) != 1 || len(e.tg.sent()) != 1 {
		t.Errorf("pushes %d, Telegram %d; want 1 each", len(e.pusher.sent()), len(e.tg.sent()))
	}
	// An ordinary event stays off on the switched-off channels.
	if err := e.svc.InvitationAccepted(ctx, e.f.Account.ID, "gast@example.test"); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	if n := len(e.deliveries(t)); n != 3 {
		t.Errorf("%d deliveries after an ordinary event on switched-off channels, want still 3", n)
	}
}

// Without any channel the server can carry (no e-mail server, no Telegram bot, no subscription),
// the pairing still counts and only «Meine Geräte» shows it (ADR 0040, known limit); the server
// logs the gap as an error.
func TestDeviceLinkedWithoutAnyChannel(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	e.cfg.set(func(c *fakeConfig) { c.smtpOK = false })
	if err := e.svc.DeviceLinked(ctx, e.f.Account.ID, "tv", "Stube", "Android TV 14", time.Now()); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	for _, d := range e.deliveries(t) {
		if d.state == "sent" {
			t.Errorf("delivery %+v sent without a channel", d)
		}
	}
	if len(e.mailer.sent()) != 0 || len(e.tg.sent()) != 0 {
		t.Error("a message went out without a channel")
	}
}

// Push is retried per subscription (code review M-2 of stage 1): the push relay answers 503 once,
// the browser takes the message at once; the retry goes to the relay only, so the browser gets the
// message once and the phone gets it too.
func TestPushRetriesOnlyTheFailedSubscription(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	e.enable(t, e.owner(), model.ChannelPush, model.EventInvitations)
	browser := "https://push.example.test/browser"
	phone := push.DefaultRelayOrigin + "/v1/push/1.phone"
	e.subscribe(t, e.owner(), browser)
	e.subscribe(t, e.owner(), phone)
	e.pusher.errs[phone] = errors.New("push service answered 503 Service Unavailable")
	if err := e.svc.InvitationAccepted(ctx, e.f.Account.ID, "gast@example.test"); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	if d := e.deliveries(t); len(d) != 1 || d[0].state != "queued" {
		t.Fatalf("after the 503: %+v, want the delivery queued for a retry", d)
	}
	delete(e.pusher.errs, phone)
	e.exec(t, `UPDATE jobs SET run_after = 0`) // the retry is due now
	e.drain(t)
	count := map[string]int{}
	for _, p := range e.pusher.sent() {
		count[p.endpoint]++
	}
	if count[browser] != 1 || count[phone] != 1 {
		t.Errorf("pushes %v, want one each", count)
	}
	if d := e.deliveries(t); len(d) != 1 || d[0].state != "sent" {
		t.Errorf("after the retry: %+v", d)
	}
}

// A language without the texts yet (the translation round comes later) falls back to English.
func TestDeviceLinkedFallsBackToEnglish(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	// Until the translation round, fr lacks the texts and takes English; afterwards its own.
	want := func(lang, key string) string {
		if raw, ok := texts.Raw(lang, key); ok {
			return raw
		}
		return localeText(t, "en", key, nil)
	}
	for _, key := range []string{"device_linked_subject", "device_linked_body", "device_kind_tv"} {
		if got, en := text("ar", key), localeText(t, "en", key, nil); got != en {
			t.Errorf("text(ar, %s) = %q, want English %q", key, got, en)
		}
	}
	e.enable(t, e.guest(), model.ChannelEmail)
	e.exec(t, `UPDATE profiles SET language = 'fr' WHERE account_id = ?`, e.f.Guest.ID)
	if err := e.svc.DeviceLinked(ctx, e.f.Guest.ID, "tv", "Salon", "Android TV 14", time.Now()); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	mails := e.mailer.sent()
	if subject := want("fr", "device_linked_subject"); len(mails) != 1 || mails[0].subject != subject {
		t.Fatalf("mails = %+v, want %q", mails, subject)
	}
	if !strings.Contains(mails[0].text, "Salon") || !strings.Contains(mails[0].text, want("fr", "device_kind_tv")) {
		t.Errorf("text %q", mails[0].text)
	}
}

// A subscription of the push relay gets the payload cut to 350 characters of body and at most 2597
// bytes; a browser subscription of the same account gets the full one.
func TestPushToRelayIsCut(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	e.enable(t, e.owner(), model.ChannelPush, model.EventAnnouncements)
	e.subscribe(t, e.owner(), "https://push.example.test/browser")
	e.subscribe(t, e.owner(), push.DefaultRelayOrigin+"/v1/push/1.blob")
	long := strings.Repeat("Länger Text ", 100)
	if err := e.svc.Emit(ctx, model.Event{Type: model.EventAnnouncements, Subject: "Hinweis", Body: long, Link: "/announcements"}); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	pushes := e.pusher.sent()
	if len(pushes) != 2 {
		t.Fatalf("pushes = %+v", pushes)
	}
	for _, p := range pushes {
		var m struct{ Title, Body, URL string }
		if err := json.Unmarshal([]byte(p.payload), &m); err != nil {
			t.Fatal(err)
		}
		relay := push.IsRelay(p.endpoint, push.DefaultRelayOrigin)
		switch {
		case relay && (utf8.RuneCountInString(m.Body) != relayMaxBody || len(p.payload) > push.RelayMaxPayload):
			t.Errorf("relay payload: body %d characters, %d bytes", utf8.RuneCountInString(m.Body), len(p.payload))
		case !relay && m.Body != strings.TrimSpace(long) && m.Body != long:
			t.Errorf("browser body cut to %d characters", utf8.RuneCountInString(m.Body))
		}
	}
}

// relayPayload counts bytes, not characters: escaped and multi-byte text is cut until the JSON
// fits; a long link becomes its path, or "/".
func TestRelayPayload(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		title, body, link string
		wantURL           string
	}{
		"short":           {"Titel", "Text", "/items/1", "/items/1"},
		"escapes":         {strings.Repeat("&", maxSubject), strings.Repeat("<", 400), "/items/1", "/items/1"},
		"four-byte runes": {strings.Repeat("😀", maxSubject), strings.Repeat("😀", 400), "/", "/"},
		"long link":       {"T", "B", "https://media.example.test/items/1?" + strings.Repeat("x", 300), "/items/1"},
		"long path":       {"T", "B", "https://media.example.test/" + strings.Repeat("p", 300), "/"},
		"control escapes": {"T", strings.Repeat(" ", 400), "/", "/"},
	} {
		got := relayPayload(c.title, c.body, c.link)
		var m struct{ Title, Body, URL string }
		if err := json.Unmarshal(got, &m); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(got) > push.RelayMaxPayload || utf8.RuneCountInString(m.Body) > relayMaxBody || m.URL != c.wantURL {
			t.Errorf("%s: %d bytes, body %d characters, url %q", name, len(got), utf8.RuneCountInString(m.Body), m.URL)
		}
		if name == "short" && (m.Title != c.title || m.Body != c.body) {
			t.Errorf("short payload changed: %s", got)
		}
	}
}

// An app session manages its own subscription with any profile (concept 27 7.3) and keeps one per
// device; a browser session still needs the primary profile.
func TestAppSessionOwnsItsSubscription(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	const phone, other = "0123456789abcdef0123456789abcdef", "fedcba9876543210fedcba9876543210"
	for _, id := range []string{phone, other} {
		e.exec(t, `INSERT INTO devices (id, created_at, last_seen_at) VALUES (?, 1, 1)`, id)
	}
	app := model.Session{Account: e.f.Account, Profile: e.f.P12, Kind: model.SessionApp, DeviceID: phone}
	browserChild := model.Session{Account: e.f.Account, Profile: e.f.P12, DeviceID: other}
	if _, err := e.svc.CreatePushSubscription(ctx, browserChild, subscriptionInput(t, "https://push.example.test/b")); !errors.Is(err, model.ErrPrimaryProfileRequired) {
		t.Fatalf("browser with a child profile: %v", err)
	}
	first := e.subscribe(t, app, push.DefaultRelayOrigin+"/v1/push/1.first")
	foreign := e.subscribe(t, model.Session{Account: e.f.Account, Profile: e.f.P18, DeviceID: other}, "https://push.example.test/other")
	second := e.subscribe(t, app, push.DefaultRelayOrigin+"/v1/push/1.second")
	if n := e.count(t, `SELECT count(*) FROM push_subscriptions WHERE id = ?`, first); n != 0 {
		t.Error("the older subscription of the device stayed")
	}
	if n := e.count(t, `SELECT count(*) FROM push_subscriptions WHERE account_id = ?`, e.f.Account.ID); n != 2 {
		t.Errorf("%d subscriptions, want the new one of the phone and the other device's", n)
	}
	// Renewing the same endpoint keeps the row.
	if again := e.subscribe(t, app, push.DefaultRelayOrigin+"/v1/push/1.second"); again != second {
		t.Errorf("renewal gave row %d, want %d", again, second)
	}
	if err := e.svc.DeletePushSubscription(ctx, app, foreign); !errors.Is(err, model.ErrPrimaryProfileRequired) {
		t.Errorf("app deletes another device's subscription with a child profile: %v", err)
	}
	if err := e.svc.DeletePushSubscription(ctx, app, second); err != nil {
		t.Errorf("app deletes its own subscription: %v", err)
	}
	if n := e.count(t, `SELECT count(*) FROM push_subscriptions WHERE account_id = ?`, e.f.Account.ID); n != 1 {
		t.Errorf("%d subscriptions left, want 1", n)
	}
}
