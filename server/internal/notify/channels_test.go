package notify

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify/push"
	"github.com/slouchcat/slouchcat/server/internal/notify/telegram"
)

// Plan 5.2: the main account starts with every channel off (new content, errors, invitations and
// announcements chosen), a guest with e-mail on for announcements. available follows the server.
func TestChannelDefaults(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	owner, err := e.svc.Channels(ctx, e.owner())
	if err != nil {
		t.Fatal(err)
	}
	if len(owner) != 3 || owner[0].Channel != model.ChannelEmail || owner[1].Channel != model.ChannelPush || owner[2].Channel != model.ChannelTelegram {
		t.Fatalf("owner channels = %+v", owner)
	}
	for _, c := range owner {
		if c.Enabled || !slices.Equal(c.Events, []string{"new_content", "errors", "invitations", "announcements"}) {
			t.Errorf("owner %s = enabled %v events %v", c.Channel, c.Enabled, c.Events)
		}
	}
	if owner[0].Email == nil || owner[0].Email.Address != e.f.Account.Email || !owner[0].Available {
		t.Errorf("owner e-mail = %+v", owner[0])
	}
	if !owner[1].Available || owner[1].Push == nil || len(owner[1].Push.Subscriptions) != 0 {
		t.Errorf("owner push = %+v", owner[1])
	}
	if owner[2].Available || owner[2].Telegram == nil || owner[2].Telegram.Linked {
		t.Errorf("owner telegram without a bot = %+v", owner[2])
	}
	guest, err := e.svc.Channels(ctx, e.guest())
	if err != nil {
		t.Fatal(err)
	}
	if !guest[0].Enabled || !slices.Equal(guest[0].Events, []string{"announcements"}) || guest[0].Email.Address != e.f.Guest.Email {
		t.Errorf("guest e-mail = %+v", guest[0])
	}
	if guest[1].Enabled || guest[2].Enabled {
		t.Errorf("guest push/telegram switched on: %+v", guest)
	}

	// An account without rows (created before the hook) gets them on first read; without settings
	// no channel is available.
	e.exec(t, `DELETE FROM account_channels WHERE account_id = ?`, e.f.Guest.ID)
	bare := New(e.f.DB, e.svc.vis, e.svc.log)
	list, err := bare.Channels(ctx, e.guest())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range list {
		if c.Available {
			t.Errorf("%s available without settings", c.Channel)
		}
	}
	if !list[0].Enabled {
		t.Error("guest e-mail default not recreated")
	}
}

func TestUpdateChannel(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	addr := func(s string) *string { return &s }
	var verr *model.ValidationError
	cases := []struct {
		name    string
		session model.Session
		channel string
		upd     model.ChannelSettingUpdate
		check   func(error) bool
	}{
		{"unknown channel", e.owner(), "sms", model.ChannelSettingUpdate{}, func(err error) bool { return errors.Is(err, model.ErrNotFound) }},
		{"unknown event", e.owner(), "email", model.ChannelSettingUpdate{Events: []string{"spam"}},
			func(err error) bool { return errors.As(err, &verr) && verr.Fields[0].Field == "events" }},
		{"invalid address", e.owner(), "email", model.ChannelSettingUpdate{EmailAddress: addr("not-an-address")},
			func(err error) bool { return errors.As(err, &verr) && verr.Fields[0].Field == "email.address" }},
		{"errors on a guest", e.guest(), "email", model.ChannelSettingUpdate{Enabled: true, Events: []string{"errors"}},
			func(err error) bool { return errors.Is(err, model.ErrEventNotAllowed) }},
		{"telegram without bot", e.owner(), "telegram", model.ChannelSettingUpdate{Enabled: true},
			func(err error) bool { return errors.Is(err, model.ErrChannelUnavailable) }},
		{"switching off an unavailable channel", e.owner(), "telegram", model.ChannelSettingUpdate{Enabled: false, Events: []string{"errors"}},
			func(err error) bool { return err == nil }},
		{"owner errors", e.owner(), "email", model.ChannelSettingUpdate{Enabled: true, Events: []string{"errors", "new_content", "errors"}},
			func(err error) bool { return err == nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := e.svc.UpdateChannel(ctx, tc.session, tc.channel, tc.upd); !tc.check(err) {
				t.Errorf("err = %v", err)
			}
		})
	}
	// The account's own address means "the account's e-mail": nothing is stored, so the channel
	// follows a later change of it (review M-4).
	if _, err := e.svc.UpdateChannel(ctx, e.owner(), "email", model.ChannelSettingUpdate{Enabled: true,
		EmailAddress: addr(strings.ToUpper(e.f.Account.Email))}); err != nil {
		t.Fatal(err)
	}
	if n := e.count(t, `SELECT count(*) FROM account_channels WHERE account_id = ? AND channel = 'email' AND config = '{}'`, e.f.Account.ID); n != 1 {
		t.Error("the account's own address was stored")
	}
	moved := e.owner()
	moved.Account.Email = "new-owner@example.test"
	if c, err := e.svc.Channels(ctx, moved); err != nil || c[0].Email.Address != "new-owner@example.test" {
		t.Errorf("after an e-mail change the channel shows %+v, %v; want the new address", c[0].Email, err)
	}
	c, err := e.svc.UpdateChannel(ctx, e.owner(), "email", model.ChannelSettingUpdate{Enabled: true,
		Events: []string{"announcements", "new_content", "new_content"}, EmailAddress: addr(" Inbox@Example.test ")})
	if err != nil {
		t.Fatal(err)
	}
	if !c.Enabled || !slices.Equal(c.Events, []string{"new_content", "announcements"}) || c.Email.Address != "inbox@example.test" {
		t.Errorf("stored = %+v %+v", c, c.Email)
	}
	// Without an address the account's e-mail is the recipient again.
	c, err = e.svc.UpdateChannel(ctx, e.owner(), "email", model.ChannelSettingUpdate{Enabled: false, Events: []string{}})
	if err != nil || c.Enabled || len(c.Events) != 0 || c.Email.Address != e.f.Account.Email {
		t.Errorf("reset = %+v %v", c, err)
	}
}

// Auth calls EnsureDefaults for a new account; the rows exist before the first read.
func TestEnsureDefaultsIdempotent(t *testing.T) {
	e := newChanEnv(t)
	e.exec(t, `DELETE FROM account_channels`)
	ctx := context.Background()
	for range 2 {
		if err := e.svc.EnsureDefaults(ctx, e.f.Guest.ID); err != nil {
			t.Fatal(err)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM account_channels WHERE account_id = ?`, e.f.Guest.ID); n != 3 {
		t.Errorf("rows = %d", n)
	}
	if n := e.count(t, `SELECT count(*) FROM account_channels WHERE account_id = ? AND channel = 'email' AND enabled = 1`, e.f.Guest.ID); n != 1 {
		t.Errorf("guest e-mail on = %d", n)
	}
}

// E-8 (M8 part), plan 5.5: a new episode of a favourite becomes an e-mail with the profile's name
// in front of the subject, only when the account switched e-mail on for new content, and never for
// a profile that may not see the episode.
func TestNewContentByEmail(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	e.favorite(t, e.f.Show.Series.ID, e.f.P12, e.f.P6)
	// E-mail off (default): the inbox gets the entry, no channel.
	ep1 := e.addEpisode(t, "A1", nil)
	if err := e.svc.NewEpisodes(ctx, []string{ep1}); err != nil {
		t.Fatal(err)
	}
	if ids, _, _ := e.inbox(t, e.f.P12); !slices.Equal(ids, []string{ep1}) {
		t.Fatalf("P12 inbox = %v", ids)
	}
	if d := e.deliveries(t); len(d) != 0 {
		t.Fatalf("deliveries with e-mail off: %+v", d)
	}
	// E-mail on for new content: one mail for P12; P6 may not see the episode (series rated 12).
	e.enable(t, e.owner(), model.ChannelEmail, model.EventNewContent)
	ep2 := e.addEpisode(t, "A2", nil)
	if err := e.svc.NewEpisodes(ctx, []string{ep2}); err != nil {
		t.Fatal(err)
	}
	if n := e.drain(t); n != 1 {
		t.Fatalf("jobs = %d", n)
	}
	mails := e.mailer.sent()
	if len(mails) != 1 {
		t.Fatalf("mails = %+v", mails)
	}
	m := mails[0]
	if m.to != e.f.Account.Email || m.subject != "[P12] Neue Folge: Film SS" || m.smtp.Password != fakeSMTPPassword {
		t.Errorf("mail = %+v", m)
	}
	if !strings.Contains(m.text, "S01E09 · Episode A2") || !strings.Contains(m.text, "https://slouchcat.example.test/items/"+ep2) ||
		!strings.Contains(m.html, `href="https://slouchcat.example.test/items/`+ep2+`"`) {
		t.Errorf("mail body = %q / %q", m.text, m.html)
	}
	d := e.deliveries(t)
	if len(d) != 1 || d[0].profile.Int64 != e.f.P12.ID || d[0].state != "sent" || d[0].event != model.EventNewContent || d[0].attempts != 1 {
		t.Errorf("deliveries = %+v", d)
	}
	if ids, _, _ := e.inbox(t, e.f.P6); len(ids) != 0 {
		t.Errorf("P6 inbox = %v", ids)
	}
	// An English profile gets the English subject.
	e.favorite(t, e.f.Show.Series.ID, e.f.P16)
	e.exec(t, `UPDATE profiles SET language = 'en' WHERE id = ?`, e.f.P16.ID)
	e.exec(t, `DELETE FROM favorites WHERE profile_id = ?`, e.f.P12.ID)
	ep3 := e.addEpisode(t, "A3", nil)
	if err := e.svc.NewEpisodes(ctx, []string{ep3}); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	if mails := e.mailer.sent(); len(mails) != 2 || mails[1].subject != "[P16] New episode: Movie SS" {
		t.Errorf("mails = %+v", mails)
	}
}

// A-7: publishing sends the announcement to every account that chose it: the guest (e-mail on by
// default) gets a mail, the main account with e-mail off none.
func TestAnnouncementsSendAndAcknowledge(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	draft, err := e.svc.CreateAnnouncement(ctx, e.f.Account, model.AnnouncementInput{Title: "Wartung", Body: "Am Samstag\nkurz offline."})
	if err != nil {
		t.Fatal(err)
	}
	if draft.PublishedAt != nil || draft.CreatedBy != e.f.Account.Email {
		t.Errorf("draft = %+v", draft)
	}
	if n := e.drain(t); n != 0 || len(e.mailer.sent()) != 0 {
		t.Fatalf("a draft was sent")
	}
	if list, _ := e.svc.Announcements(ctx, e.f.PG); len(list) != 0 {
		t.Errorf("draft visible to PG: %+v", list)
	}
	if err := e.svc.AckAnnouncement(ctx, e.f.PG, draft.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("ack of a draft: %v", err)
	}
	if _, err := e.svc.UpdateAnnouncement(ctx, draft.ID, "Wartung am Samstag", "Am Samstag kurz offline."); err != nil {
		t.Fatal(err)
	}
	pub, err := e.svc.PublishAnnouncement(ctx, draft.ID)
	if err != nil || pub.PublishedAt == nil || pub.Title != "Wartung am Samstag" {
		t.Fatalf("publish = %+v %v", pub, err)
	}
	e.drain(t)
	mails := e.mailer.sent()
	if len(mails) != 1 || mails[0].to != e.f.Guest.Email || mails[0].subject != "Wartung am Samstag" {
		t.Fatalf("mails = %+v", mails)
	}
	// Published twice: nothing is sent again; a published one cannot change.
	if _, err := e.svc.PublishAnnouncement(ctx, draft.ID); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	if n := len(e.mailer.sent()); n != 1 {
		t.Errorf("mails after the second publish = %d", n)
	}
	if _, err := e.svc.UpdateAnnouncement(ctx, draft.ID, "x", "y"); !errors.Is(err, model.ErrAnnouncementPublished) {
		t.Errorf("update of a published one: %v", err)
	}
	// Profiles: open until acknowledged, per profile; a second ack is fine.
	for _, p := range []model.Profile{e.f.PG, e.f.PG18, e.f.P18, e.f.P6} {
		list, err := e.svc.Announcements(ctx, p)
		if err != nil || len(list) != 1 || list[0].ID != draft.ID {
			t.Errorf("%s open = %+v %v", p.Name, list, err)
		}
	}
	for range 2 {
		if err := e.svc.AckAnnouncement(ctx, e.f.PG, draft.ID); err != nil {
			t.Fatal(err)
		}
	}
	if list, _ := e.svc.Announcements(ctx, e.f.PG); len(list) != 0 {
		t.Errorf("PG after ack = %+v", list)
	}
	if list, _ := e.svc.Announcements(ctx, e.f.PG18); len(list) != 1 {
		t.Errorf("PG18 after PG's ack = %+v", list)
	}
	all, err := e.svc.ListAnnouncements(ctx)
	if err != nil || len(all) != 1 || all[0].AcknowledgedCount != 1 || all[0].ProfileCount != 7 {
		t.Errorf("admin list = %+v %v", all, err)
	}
	// Publish on create; unknown ids.
	if _, err := e.svc.CreateAnnouncement(ctx, e.f.Account, model.AnnouncementInput{Title: "Neu", Body: "Sofort", Publish: true}); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	if n := len(e.mailer.sent()); n != 2 {
		t.Errorf("mails after create with publish = %d", n)
	}
	if err := e.svc.AckAnnouncement(ctx, e.f.PG, 9999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("ack unknown: %v", err)
	}
	if _, err := e.svc.PublishAnnouncement(ctx, 9999); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("publish unknown: %v", err)
	}
	if err := e.svc.DeleteAnnouncement(ctx, draft.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.DeleteAnnouncement(ctx, draft.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
	var verr *model.ValidationError
	if _, err := e.svc.CreateAnnouncement(ctx, e.f.Account, model.AnnouncementInput{Title: strings.Repeat("x", 121), Body: " "}); !errors.As(err, &verr) || len(verr.Fields) != 2 {
		t.Errorf("validation = %v", err)
	}
}

// The event errors goes to the main account only, once per source and hour; a failed delivery
// job raises none.
func TestErrorsOwnerOnlyWithDedupe(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	e.cfg.set(func(c *fakeConfig) { c.token = testBotToken })
	e.enable(t, e.owner(), model.ChannelEmail, model.EventErrors)
	e.enable(t, e.owner(), model.ChannelTelegram, model.EventErrors)
	e.linkChat(t, e.f.Account.ID, 4242)
	// The guest switches everything on that it may; errors is not among it.
	e.enable(t, e.guest(), model.ChannelEmail, model.EventAnnouncements, model.EventNewContent)

	job := func(typ string) jobs.Job { return jobs.Job{ID: 7, Type: typ, Attempts: 5, MaxAttempts: 5} }
	e.svc.JobFailed(ctx, job("metadata.match"), errFake)
	e.svc.JobFailed(ctx, job("metadata.match"), errFake) // same source within the hour
	e.svc.JobFailed(ctx, job(JobDeliver), errFake)       // no loop
	if err := e.svc.ScanAborted(ctx, "Filme", "loss_over_threshold"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ScanAborted(ctx, "Filme", "root_missing"); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	mails, msgs := e.mailer.sent(), e.tg.sent()
	if len(mails) != 2 || len(msgs) != 2 {
		t.Fatalf("mails %+v, telegram %+v", mails, msgs)
	}
	if mails[0].to != e.f.Account.Email || mails[0].subject != "Fehler: Job metadata.match" || !strings.Contains(mails[0].text, "fake failure") {
		t.Errorf("job mail = %+v", mails[0])
	}
	if mails[1].subject != "Scan abgebrochen: Filme" || !strings.Contains(mails[1].text, "20 %") {
		t.Errorf("scan mail = %+v", mails[1])
	}
	if msgs[0].chat != 4242 || msgs[0].token != testBotToken || !strings.HasPrefix(msgs[0].text, "Fehler: Job metadata.match\n\n") {
		t.Errorf("telegram = %+v", msgs[0])
	}
	for _, d := range e.deliveries(t) {
		if d.account != e.f.Account.ID || d.event != model.EventErrors {
			t.Errorf("delivery %+v", d)
		}
	}
	// Another hour: the same source goes out again.
	e.now = e.now.Add(61 * time.Minute)
	e.svc.JobFailed(ctx, job("metadata.match"), errFake)
	e.drain(t)
	if n := len(e.mailer.sent()); n != 3 {
		t.Errorf("mails after an hour = %d", n)
	}
}

// Push: a subscription the push service no longer knows is deleted; with none left the delivery
// is skipped. The payload carries title, body and link.
func TestPushGoneAndSkipped(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	e.enable(t, e.owner(), model.ChannelPush, model.EventInvitations)
	live, gone := e.subscribe(t, e.owner(), "https://push.example.test/live"), e.subscribe(t, e.owner(), "https://push.example.test/gone")
	e.pusher.errs["https://push.example.test/gone"] = fmt.Errorf("push: %w", push.ErrSubscriptionGone)
	if err := e.svc.InvitationAccepted(ctx, e.f.Account.ID, "gast@example.test"); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	pushes := e.pusher.sent()
	if len(pushes) != 1 || pushes[0].endpoint != "https://push.example.test/live" || pushes[0].ttl != pushTTL {
		t.Fatalf("pushes = %+v", pushes)
	}
	want := `{"title":"Einladung angenommen","body":"gast@example.test hat deine Einladung angenommen und ein Konto erstellt.","url":"https://slouchcat.example.test/admin/access"}`
	if pushes[0].payload != want {
		t.Errorf("payload = %s", pushes[0].payload)
	}
	if pushes[0].vapid.PublicKey == "" || pushes[0].vapid.Subscriber != "https://slouchcat.example.test" {
		t.Errorf("vapid = %+v", pushes[0].vapid)
	}
	if n := e.count(t, `SELECT count(*) FROM push_subscriptions WHERE id = ?`, gone); n != 0 {
		t.Error("gone subscription kept")
	}
	if d := e.deliveries(t); len(d) != 1 || d[0].state != "sent" {
		t.Errorf("deliveries = %+v", d)
	}
	// Only the gone one left: skipped, and it is removed.
	e.exec(t, `DELETE FROM push_subscriptions WHERE id = ?`, live)
	e.subscribe(t, e.owner(), "https://push.example.test/gone")
	if err := e.svc.InvitationAccepted(ctx, e.f.Account.ID, "zweit@example.test"); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	d := e.deliveries(t)
	if len(d) != 2 || d[1].state != "skipped" || !strings.Contains(d[1].lastError.String, "no recipient") {
		t.Errorf("deliveries = %+v", d)
	}
	if n := e.count(t, `SELECT count(*) FROM push_subscriptions`); n != 0 {
		t.Errorf("subscriptions left = %d", n)
	}
}

// A failing sender is retried by the queue; after the last attempt the delivery ends failed with
// the error, which carries no secret. A retry of its job by the admin sends it again.
func TestDeliveryRetriesThenFails(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	e.enable(t, e.owner(), model.ChannelEmail, model.EventInvitations)
	e.mailer.fail(errors.New("smtp authentication: 535 5.7.8 invalid https://x.test/?token=" + fakeSMTPPassword))
	if err := e.svc.InvitationAccepted(ctx, e.f.Account.ID, "gast@example.test"); err != nil {
		t.Fatal(err)
	}
	for range 10 {
		e.drain(t)
		e.now = e.now.Add(10 * time.Minute) // past every backoff
	}
	d := e.deliveries(t)
	if len(d) != 1 || d[0].state != "failed" || d[0].attempts != deliverAttempts {
		t.Fatalf("deliveries = %+v", d)
	}
	if !strings.Contains(d[0].lastError.String, "535") || strings.Contains(d[0].lastError.String, fakeSMTPPassword) {
		t.Errorf("last error = %q", d[0].lastError.String)
	}
	if n := e.count(t, `SELECT count(*) FROM jobs WHERE type = ? AND state = 'failed'`, JobDeliver); n != 1 {
		t.Errorf("failed jobs = %d", n)
	}
	// "Erneut" on the dashboard (review M-1): the retried job sends the failed delivery again.
	e.mailer.fail(nil)
	var jobID int64
	if err := e.f.DB.Read.QueryRow(`SELECT id FROM jobs WHERE type = ? AND state = 'failed'`, JobDeliver).Scan(&jobID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.queue.Retry(ctx, jobID); err != nil {
		t.Fatal(err)
	}
	if n := e.drain(t); n != 1 {
		t.Errorf("drained %d jobs after the retry, want 1", n)
	}
	if d := e.deliveries(t); len(d) != 1 || d[0].state != "sent" || len(e.mailer.sent()) != 1 {
		t.Errorf("after the retry: deliveries %+v, mails %d", d, len(e.mailer.sent()))
	}
}

// A channel switched off after the event skips its delivery.
func TestDeliverySkippedWhenSwitchedOff(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	e.enable(t, e.owner(), model.ChannelEmail, model.EventInvitations)
	if err := e.svc.InvitationAccepted(ctx, e.f.Account.ID, "gast@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UpdateChannel(ctx, e.owner(), model.ChannelEmail, model.ChannelSettingUpdate{Events: []string{"invitations"}}); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	if d := e.deliveries(t); len(d) != 1 || d[0].state != "skipped" || len(e.mailer.sent()) != 0 {
		t.Errorf("deliveries = %+v", d)
	}
}

func TestTestChannel(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	if err := e.svc.TestChannel(ctx, e.owner(), model.ChannelEmail); !errors.Is(err, model.ErrChannelDisabled) {
		t.Errorf("e-mail off: %v", err)
	}
	if err := e.svc.TestChannel(ctx, e.owner(), model.ChannelTelegram); !errors.Is(err, model.ErrChannelUnavailable) {
		t.Errorf("telegram without bot: %v", err)
	}
	if err := e.svc.TestChannel(ctx, e.owner(), "sms"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
	if err := e.svc.TestChannel(ctx, e.guest(), model.ChannelEmail); err != nil {
		t.Fatal(err)
	}
	if mails := e.mailer.sent(); len(mails) != 1 || mails[0].to != e.f.Guest.Email || mails[0].subject != "Slouchcat: test message" {
		t.Errorf("mails = %+v", mails)
	}
	// The cause stays in the log: a guest learns nothing about the mail server (review M-11).
	e.mailer.fail(errors.New("smtp connect mail.lan:587: dial tcp 192.168.1.5:587: connect: connection refused"))
	err := e.svc.TestChannel(ctx, e.guest(), model.ChannelEmail)
	var send *model.SendError
	if !errors.As(err, &send) || !errors.Is(err, model.ErrSendFailed) || send.Detail != testFailedDetail {
		t.Errorf("failing send: %v", err)
	}
	// Push without subscription and Telegram without chat count as disabled.
	e.enable(t, e.owner(), model.ChannelPush)
	if err := e.svc.TestChannel(ctx, e.owner(), model.ChannelPush); !errors.Is(err, model.ErrChannelDisabled) {
		t.Errorf("push without subscription: %v", err)
	}
	e.cfg.set(func(c *fakeConfig) { c.token = testBotToken })
	e.enable(t, e.owner(), model.ChannelTelegram)
	if err := e.svc.TestChannel(ctx, e.owner(), model.ChannelTelegram); !errors.Is(err, model.ErrChannelDisabled) {
		t.Errorf("telegram without chat: %v", err)
	}
	e.linkChat(t, e.f.Account.ID, 99)
	if err := e.svc.TestChannel(ctx, e.owner(), model.ChannelTelegram); err != nil {
		t.Fatal(err)
	}
	if msgs := e.tg.sent(); len(msgs) != 1 || msgs[0].chat != 99 || !strings.Contains(msgs[0].text, "Kanal Telegram funktioniert") {
		t.Errorf("telegram = %+v", msgs)
	}
	e.subscribe(t, e.owner(), "https://push.example.test/a")
	if err := e.svc.TestChannel(ctx, e.owner(), model.ChannelPush); err != nil {
		t.Fatal(err)
	}
	if p := e.pusher.sent(); len(p) != 1 || p[0].ttl != pushTestTTL {
		t.Errorf("pushes = %+v", p)
	}
}

// ADR 0005 (review I-2): a profile other than the primary one reads the channels of its account but
// changes and tests none; a session without a profile changes them.
func TestChannelChangesNeedThePrimaryProfile(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	e.cfg.set(func(c *fakeConfig) { c.token = testBotToken })
	child := session(e.f.Account, e.f.P6)
	if c, err := e.svc.Channels(ctx, child); err != nil || len(c) != 3 {
		t.Fatalf("read: %v, %v", c, err)
	}
	sub := e.subscribe(t, e.owner(), "https://push.example.test/owner")
	for name, call := range map[string]func() error{
		"update": func() error {
			_, err := e.svc.UpdateChannel(ctx, child, model.ChannelEmail, model.ChannelSettingUpdate{Enabled: true, Events: []string{"announcements"}})
			return err
		},
		"test": func() error { return e.svc.TestChannel(ctx, child, model.ChannelEmail) },
		"link": func() error {
			_, err := e.svc.CreateTelegramLink(ctx, child)
			return err
		},
		"unlink": func() error { return e.svc.DeleteTelegramLink(ctx, child) },
		"subscribe": func() error {
			_, err := e.svc.CreatePushSubscription(ctx, child, subscriptionInput(t, "https://push.example.test/child"))
			return err
		},
		"unsubscribe": func() error { return e.svc.DeletePushSubscription(ctx, child, sub) },
	} {
		if err := call(); !errors.Is(err, model.ErrPrimaryProfileRequired) {
			t.Errorf("%s by a child profile: %v, want ErrPrimaryProfileRequired", name, err)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM push_subscriptions`); n != 1 {
		t.Errorf("subscriptions = %d, want the owner's only", n)
	}
	if n := e.count(t, `SELECT count(*) FROM telegram_links`); n != 0 || len(e.mailer.sent()) != 0 {
		t.Errorf("link codes %d, mails %v; want none", n, e.mailer.sent())
	}
	account := model.Session{Account: e.f.Account}
	if _, err := e.svc.UpdateChannel(ctx, account, model.ChannelEmail, model.ChannelSettingUpdate{Enabled: true}); err != nil {
		t.Errorf("update without a profile: %v", err)
	}
	if err := e.svc.DeletePushSubscription(ctx, account, sub); err != nil {
		t.Errorf("unsubscribe without a profile: %v", err)
	}
}

// Test messages count against the client's attempt limit before anything is sent (review M-11).
func TestTestChannelAttemptLimit(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	calls := 0
	WithAttemptLimit(func(context.Context) error {
		calls++
		if calls > 1 {
			return model.ErrTooManyAttempts
		}
		return nil
	})(e.svc)
	if err := e.svc.TestChannel(ctx, e.guest(), model.ChannelEmail); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.TestChannel(ctx, e.guest(), model.ChannelEmail); !errors.Is(err, model.ErrTooManyAttempts) {
		t.Errorf("second test: %v, want ErrTooManyAttempts", err)
	}
	if mails := e.mailer.sent(); len(mails) != 1 {
		t.Errorf("mails = %d, want 1", len(mails))
	}
}

func TestSendTestEmail(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	if err := e.svc.SendTestEmail(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.SendTestEmail(ctx, "other@example.test"); err != nil {
		t.Fatal(err)
	}
	mails := e.mailer.sent()
	if len(mails) != 2 || mails[0].to != e.f.Account.Email || mails[1].to != "other@example.test" || mails[0].subject != "Slouchcat: Testnachricht" {
		t.Errorf("mails = %+v", mails)
	}
	e.mailer.fail(errors.New("smtp authentication: 535 rejected"))
	err := e.svc.SendTestEmail(ctx, "")
	var send *model.SendError
	if !errors.As(err, &send) || !errors.Is(err, model.ErrEmailSendFailed) || strings.Contains(send.Detail, fakeSMTPPassword) {
		t.Errorf("failing send: %v", err)
	}
	e.cfg.set(func(c *fakeConfig) { c.smtpOK = false })
	if err := e.svc.SendTestEmail(ctx, ""); !errors.Is(err, model.ErrEmailNotConfigured) {
		t.Errorf("without server: %v", err)
	}
}

// The VAPID pair is created once, also under concurrent first use; a rotation replaces it and
// deletes every subscription.
func TestVAPIDKeys(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	keys := make([]string, 8)
	for i := range keys {
		wg.Go(func() {
			k, err := e.svc.PushPublicKey(ctx)
			if err != nil {
				t.Error(err)
			}
			keys[i] = k
		})
	}
	wg.Wait()
	for _, k := range keys {
		if k != keys[0] || k == "" {
			t.Fatalf("keys = %v", keys)
		}
	}
	if e.cfg.setVAPID != 1 {
		t.Errorf("SetVAPIDKeys calls = %d", e.cfg.setVAPID)
	}
	e.subscribe(t, e.owner(), "https://push.example.test/a")
	if err := e.svc.RotateVAPID(ctx); err != nil {
		t.Fatal(err)
	}
	k, _ := e.svc.PushPublicKey(ctx)
	if k == keys[0] || e.cfg.setVAPID != 2 {
		t.Errorf("rotation kept the key (%d sets)", e.cfg.setVAPID)
	}
	if n := e.count(t, `SELECT count(*) FROM push_subscriptions`); n != 0 {
		t.Errorf("subscriptions after rotation = %d", n)
	}
	bare := New(e.f.DB, e.svc.vis, e.svc.log)
	if _, err := bare.PushPublicKey(ctx); !errors.Is(err, model.ErrPushUnavailable) {
		t.Errorf("without settings: %v", err)
	}
	if err := bare.RotateVAPID(ctx); !errors.Is(err, model.ErrPushUnavailable) {
		t.Errorf("rotate without settings: %v", err)
	}
}

func TestPushSubscriptions(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	e.exec(t, `INSERT INTO devices (id, created_at, last_seen_at) VALUES (?, 1, 1)`, strings.Repeat("a", 32))
	devCtx := model.WithDeviceID(ctx, strings.Repeat("a", 32))
	in := subscriptionInput(t, "https://push.example.test/one")
	in.DeviceName = "Laptop"
	first, err := e.svc.CreatePushSubscription(devCtx, e.owner(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !first.ThisDevice || first.DeviceName != "Laptop" {
		t.Errorf("first = %+v", first)
	}
	// Same endpoint again: same row, new name.
	in.DeviceName = "Laptop 2"
	again, err := e.svc.CreatePushSubscription(devCtx, e.owner(), in)
	if err != nil || again.ID != first.ID || again.DeviceName != "Laptop 2" {
		t.Errorf("again = %+v %v", again, err)
	}
	// An unknown device id is not stored (no row in devices).
	other, err := e.svc.CreatePushSubscription(model.WithDeviceID(ctx, strings.Repeat("b", 32)), e.owner(), subscriptionInput(t, "https://push.example.test/two"))
	if err != nil || other.ThisDevice {
		t.Errorf("other = %+v %v", other, err)
	}
	list, err := e.svc.Channels(devCtx, e.owner())
	if err != nil {
		t.Fatal(err)
	}
	subs := list[1].Push.Subscriptions
	if len(subs) != 2 || !subs[0].ThisDevice || subs[1].ThisDevice {
		t.Errorf("subscriptions = %+v", subs)
	}
	var verr *model.ValidationError
	bad := model.PushSubscriptionInput{Endpoint: "http://push.example.test/x", P256dh: "BNcR", Auth: "tBHI", DeviceName: strings.Repeat("x", 81)}
	if _, err := e.svc.CreatePushSubscription(ctx, e.owner(), bad); !errors.As(err, &verr) || len(verr.Fields) != 4 {
		t.Errorf("validation = %v", err)
	}
	// Development mode accepts http endpoints (the push mock).
	dev := New(e.f.DB, e.svc.vis, e.svc.log, WithDevPush(true))
	dev.SetConfig(e.cfg)
	if _, err := dev.CreatePushSubscription(ctx, e.owner(), subscriptionInput(t, "http://127.0.0.1:9000/push/1")); err != nil {
		t.Errorf("http endpoint in development: %v", err)
	}
	if err := e.svc.DeletePushSubscription(ctx, e.guest(), first.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("guest deletes the owner's subscription: %v", err)
	}
	if err := e.svc.DeletePushSubscription(ctx, e.owner(), first.ID); err != nil {
		t.Fatal(err)
	}
	bare := New(e.f.DB, e.svc.vis, e.svc.log)
	if _, err := bare.CreatePushSubscription(ctx, e.owner(), in); !errors.Is(err, model.ErrPushUnavailable) {
		t.Errorf("without settings: %v", err)
	}
}

func TestChannelAvailabilityAndGetMe(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	got, err := e.svc.ChannelAvailability(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got[model.ChannelEmail].Configured || !got[model.ChannelPush].Configured || got[model.ChannelTelegram].Configured {
		t.Errorf("availability = %+v", got)
	}
	e.cfg.set(func(c *fakeConfig) { c.err = errFake })
	got, _ = e.svc.ChannelAvailability(ctx)
	if got[model.ChannelEmail].Configured || got[model.ChannelEmail].Detail == "" {
		t.Errorf("unreadable settings = %+v", got)
	}
	e.tg.name = "slouchcat_bot"
	if name, err := e.svc.GetMe(ctx, testBotToken); err != nil || name != "slouchcat_bot" {
		t.Errorf("GetMe = %q %v", name, err)
	}
	e.tg.meErr = fmt.Errorf("telegram getMe: %w", telegram.ErrUnauthorized)
	if _, err := e.svc.GetMe(ctx, testBotToken); !errors.Is(err, model.ErrTelegramTokenInvalid) {
		t.Errorf("rejected token: %v", err)
	}
	e.tg.meErr = errFake
	if _, err := e.svc.GetMe(ctx, testBotToken); !errors.Is(err, model.ErrTelegramUnreachable) {
		t.Errorf("unreachable: %v", err)
	}
}

// M9 produces it; the hook already reaches the account's channels.
func TestAcquisitionChanged(t *testing.T) {
	e := newChanEnv(t)
	ctx := context.Background()
	e.enable(t, e.guest(), model.ChannelEmail, model.EventAcquisition)
	if err := e.svc.AcquisitionChanged(ctx, e.f.Guest.ID, "Dune", "available"); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	if mails := e.mailer.sent(); len(mails) != 1 || mails[0].to != e.f.Guest.Email || mails[0].subject != "Acquisition: Dune" {
		t.Errorf("mails = %+v", mails)
	}
	if err := e.svc.AcquisitionChanged(ctx, 99999, "x", "y"); err != nil {
		t.Errorf("unknown account: %v", err)
	}
}

// Without a queue (the M7 setup) events reach no channel and nothing fails.
func TestEmitWithoutQueue(t *testing.T) {
	e := newChanEnv(t)
	bare := New(e.f.DB, e.svc.vis, e.svc.log)
	bare.SetConfig(e.cfg)
	if err := bare.Emit(context.Background(), model.Event{Type: model.EventAnnouncements, Subject: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Emit(context.Background(), model.Event{Type: "spam"}); err == nil {
		t.Error("unknown event accepted")
	}
	if d := e.deliveries(t); len(d) != 0 {
		t.Errorf("deliveries = %+v", d)
	}
}
