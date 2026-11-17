package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/storetest"
)

// Build step 4.5: the Seerr webhook as a trigger of the sync (F-18).

// fakeSync counts the runs of the sync a notification asks for.
type fakeSync struct {
	calls  int
	off    bool
	failed error
}

func (f *fakeSync) SyncFromWebhook(context.Context) (bool, error) {
	if f.failed != nil {
		return false, f.failed
	}
	f.calls++
	return !f.off, nil
}

type seerrEnv struct {
	svc   *Service
	sync  *fakeSync
	logs  *bytes.Buffer
	db    *store.DB
	integ model.Integration
}

func newSeerrEnv(t *testing.T) *seerrEnv {
	t.Helper()
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	db, err := storetest.Open(context.Background(), store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	e := &seerrEnv{sync: &fakeSync{}, logs: &logs, db: db}
	e.svc = New(db, &fakeScanner{}, log, WithClock(func() time.Time { return now }), WithSeerrSync(e.sync))
	e.integ, err = e.svc.Create(context.Background(), "Seerr", model.IntegrationSeerr)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// The default template of Seerr (appendix D 9.2) sends notification_type, subject and the special
// keys; the receiver reads only the type for its log and queues the sync.
func TestSeerrWebhookQueuesTheSync(t *testing.T) {
	e := newSeerrEnv(t)
	ctx := context.Background()
	if e.integ.Kind != model.IntegrationSeerr || e.integ.Token == "" {
		t.Fatalf("integration = %+v", e.integ)
	}
	res, err := e.svc.Receive(ctx, e.integ.Token, model.WebhookPayload{Notification: "MEDIA_APPROVED"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Result != model.ResultSyncQueued || res.Path != "" {
		t.Errorf("receipt = %+v", res)
	}
	if e.sync.calls != 1 {
		t.Errorf("sync calls = %d", e.sync.calls)
	}
	events, err := e.svc.Events(ctx, e.integ.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Event != "MEDIA_APPROVED" || events[0].Result != model.ResultSyncQueued {
		t.Errorf("events = %+v", events)
	}
	// A body without a type (another template) still triggers; the test button is acknowledged.
	if res, err := e.svc.Receive(ctx, e.integ.Token, model.WebhookPayload{}); err != nil || res.Result != model.ResultSyncQueued {
		t.Errorf("without a type: %+v, %v", res, err)
	}
	if res, err := e.svc.Receive(ctx, e.integ.Token, model.WebhookPayload{Notification: "TEST_NOTIFICATION"}); err != nil || res.Result != model.ResultAcknowledged {
		t.Errorf("test: %+v, %v", res, err)
	}
	if e.sync.calls != 2 {
		t.Errorf("sync calls = %d", e.sync.calls)
	}
	// Seerr switched off in Slouchcat: accepted and ignored.
	e.sync.off = true
	if res, err := e.svc.Receive(ctx, e.integ.Token, model.WebhookPayload{Notification: "MEDIA_PENDING"}); err != nil || res.Result != model.ResultIgnored {
		t.Errorf("switched off: %+v, %v", res, err)
	}
	// The queue fails: an error and no entry.
	e.sync.off, e.sync.failed = false, errors.New("queue down")
	before, _ := e.svc.Events(ctx, e.integ.ID)
	if _, err := e.svc.Receive(ctx, e.integ.Token, model.WebhookPayload{Notification: "MEDIA_PENDING"}); err == nil {
		t.Error("a failed queue answered no error")
	}
	if after, _ := e.svc.Events(ctx, e.integ.ID); len(after) != len(before) {
		t.Errorf("entries %d -> %d", len(before), len(after))
	}
}

// A wrong token is refused like for the other kinds, without a sync; neither the log nor the event
// log holds the token.
func TestSeerrWebhookWrongTokenAndNoSecretInLogs(t *testing.T) {
	e := newSeerrEnv(t)
	ctx := context.Background()
	wrong := strings.Repeat("A", TokenChars)
	if _, err := e.svc.Receive(ctx, wrong, model.WebhookPayload{Notification: "MEDIA_PENDING"}); !errors.Is(err, model.ErrWebhookToken) {
		t.Errorf("wrong token: %v", err)
	}
	if _, err := e.svc.Receive(ctx, "short", model.WebhookPayload{}); !errors.Is(err, model.ErrWebhookToken) {
		t.Errorf("short token: %v", err)
	}
	if e.sync.calls != 0 {
		t.Errorf("sync calls = %d", e.sync.calls)
	}
	if _, err := e.svc.Receive(ctx, e.integ.Token, model.WebhookPayload{Notification: "MEDIA_DECLINED"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(e.logs.String(), e.integ.Token) {
		t.Error("the token is in the log")
	}
	events, err := e.svc.Events(ctx, e.integ.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(events)
	if strings.Contains(string(raw), e.integ.Token) {
		t.Error("the token is in the event log")
	}
	list, err := e.svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Token != "" || list[0].Kind != model.IntegrationSeerr {
		t.Errorf("list = %+v", list)
	}
}

// The rate limit of 10/s holds for Seerr too.
func TestSeerrWebhookRateLimited(t *testing.T) {
	e := newSeerrEnv(t)
	ctx := context.Background()
	for range RequestsPerSecond {
		if _, err := e.svc.Receive(ctx, e.integ.Token, model.WebhookPayload{Notification: "MEDIA_PENDING"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.svc.Receive(ctx, e.integ.Token, model.WebhookPayload{Notification: "MEDIA_PENDING"}); !errors.Is(err, model.ErrWebhookRateLimited) {
		t.Errorf("err = %v", err)
	}
}

// The setup the administration shows: a JSON template without personal data and the request types.
func TestSeerrWebhookSetup(t *testing.T) {
	setup := SeerrWebhookSetup()
	var tpl map[string]string
	if err := json.Unmarshal([]byte(setup.JSONPayload), &tpl); err != nil {
		t.Fatalf("template is no JSON object: %v", err)
	}
	if tpl["notification_type"] != "{{notification_type}}" {
		t.Errorf("template = %v", tpl)
	}
	for _, private := range []string{"email", "username", "avatar"} {
		if strings.Contains(setup.JSONPayload, private) {
			t.Errorf("template carries %s", private)
		}
	}
	if setup.Types != 2+4+8+16+64+128 || len(setup.TypeNames) != 6 {
		t.Errorf("types = %d %v", setup.Types, setup.TypeNames)
	}
}
