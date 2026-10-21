package notify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify/telegram"
)

// botServer is an httptest Bot API for testBotToken: getMe, getUpdates (answers at once with the
// queued updates from offset on) and sendMessage.
type botServer struct {
	srv      *httptest.Server
	mu       sync.Mutex
	updates  []telegram.Update
	offsets  []int64
	messages []map[string]any
}

func newBotServer(t *testing.T) *botServer {
	t.Helper()
	b := &botServer{}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		prefix := "/bot" + testBotToken + "/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		var result any
		switch strings.TrimPrefix(r.URL.Path, prefix) {
		case "getMe":
			result = map[string]any{"username": "slouchcat_bot"}
		case "getUpdates":
			offset := int64(body["offset"].(float64))
			b.offsets = append(b.offsets, offset)
			out := []telegram.Update{}
			for _, u := range b.updates {
				if u.UpdateID >= offset {
					out = append(out, u)
				}
			}
			result = out
		case "sendMessage":
			b.messages = append(b.messages, body)
			result = map[string]any{"message_id": len(b.messages)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
	}))
	t.Cleanup(b.srv.Close)
	return b
}

func (b *botServer) state() ([]int64, []map[string]any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]int64(nil), b.offsets...), append([]map[string]any(nil), b.messages...)
}

func (b *botServer) send(id int64, chat int64, text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.updates = append(b.updates, telegram.Update{UpdateID: id, Message: &telegram.Message{Text: text, Chat: telegram.Chat{ID: chat, Type: "private"}}})
}

var codePattern = regexp.MustCompile(`^[A-Z2-9]{6}$`)

// A-6: the account asks for a code, the user sends "/start <code>" to the bot, the poller links
// the chat, switches the channel on and confirms in the chat.
func TestTelegramLinkFlow(t *testing.T) {
	e := newChanEnv(t)
	bot := newBotServer(t)
	e.svc.telegram = &telegram.Client{BaseURL: bot.srv.URL}
	ctx := context.Background()
	if _, err := e.svc.CreateTelegramLink(ctx, e.owner()); !errors.Is(err, model.ErrChannelUnavailable) {
		t.Fatalf("without bot: %v", err)
	}
	// No open code: the poller does not ask Telegram.
	if err := e.svc.pollTelegramLinks(ctx); err != nil {
		t.Fatal(err)
	}
	e.cfg.set(func(c *fakeConfig) { c.token = testBotToken })
	first, err := e.svc.CreateTelegramLink(ctx, e.owner())
	if err != nil {
		t.Fatal(err)
	}
	c, err := e.svc.CreateTelegramLink(ctx, e.owner())
	if err != nil {
		t.Fatal(err)
	}
	tg := c.Telegram
	if !codePattern.MatchString(tg.LinkCode) || tg.Linked || tg.BotUsername != "slouchcat_bot" || !c.Available ||
		tg.LinkExpiresAt == nil || !tg.LinkExpiresAt.Equal(e.now.Add(15*time.Minute)) {
		t.Fatalf("link = %+v", tg)
	}
	if first.Telegram.LinkCode == tg.LinkCode {
		t.Error("a new code repeats the old one")
	}
	bot.send(1, 1111, "/start "+first.Telegram.LinkCode) // the replaced code
	bot.send(2, 2222, "hallo")
	bot.send(3, 4242, "/start@slouchcat_bot "+strings.ToLower(tg.LinkCode))
	if err := e.svc.pollTelegramLinks(ctx); err != nil {
		t.Fatal(err)
	}
	list, err := e.svc.Channels(ctx, e.owner())
	if err != nil {
		t.Fatal(err)
	}
	linked := list[2]
	if !linked.Enabled || !linked.Telegram.Linked || linked.Telegram.ChatID != "4242" || linked.Telegram.LinkCode != "" {
		t.Errorf("after the link = %+v %+v", linked, linked.Telegram)
	}
	offsets, messages := bot.state()
	if len(messages) != 1 || messages[0]["chat_id"].(float64) != 4242 ||
		messages[0]["text"] != "Slouchcat: Dieser Chat ist jetzt mit dem Konto owner@example.test verknüpft." {
		t.Errorf("confirmation = %+v", messages)
	}
	// The code is used up: no open code, no further poll.
	if err := e.svc.pollTelegramLinks(ctx); err != nil {
		t.Fatal(err)
	}
	if offsets2, _ := bot.state(); len(offsets) != 1 || offsets[0] != 0 || len(offsets2) != 1 {
		t.Errorf("getUpdates offsets = %v then %v", offsets, offsets2)
	}
	// A new code: the next poll continues after the confirmed updates.
	if _, err := e.svc.CreateTelegramLink(ctx, e.owner()); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.pollTelegramLinks(ctx); err != nil {
		t.Fatal(err)
	}
	if offsets, _ := bot.state(); len(offsets) != 2 || offsets[1] != 4 {
		t.Errorf("offsets = %v", offsets)
	}
	// Expired codes are dropped without asking Telegram.
	e.now = e.now.Add(16 * time.Minute)
	if err := e.svc.pollTelegramLinks(ctx); err != nil {
		t.Fatal(err)
	}
	if offsets, _ := bot.state(); len(offsets) != 2 {
		t.Errorf("polled with an expired code: %v", offsets)
	}
	if n := e.count(t, `SELECT count(*) FROM telegram_links`); n != 0 {
		t.Errorf("links left = %d", n)
	}
	// Unlink: chat gone, channel off; idempotent.
	for range 2 {
		if err := e.svc.DeleteTelegramLink(ctx, e.owner()); err != nil {
			t.Fatal(err)
		}
	}
	list, _ = e.svc.Channels(ctx, e.owner())
	if list[2].Enabled || list[2].Telegram.Linked {
		t.Errorf("after unlink = %+v", list[2].Telegram)
	}
}

func TestLinkCodeIn(t *testing.T) {
	for text, want := range map[string]string{
		"/start ABC234":       "ABC234",
		"/start abc234":       "ABC234",
		"/start@bot ABC234":   "ABC234",
		"/start ABC23":        "",
		"/start ABC10O":       "",
		"/start":              "",
		"start ABC234":        "",
		"/start ABC234 extra": "",
		"/startABC234 ABC234": "",
		"/start ÄBC234":       "",
	} {
		got, ok := linkCodeIn(text)
		if got != want || ok != (want != "") {
			t.Errorf("linkCodeIn(%q) = %q %v", text, got, ok)
		}
	}
	seen := map[string]bool{}
	for range 200 {
		c := newLinkCode()
		if !codePattern.MatchString(c) || strings.ContainsAny(c, "01") {
			t.Fatalf("code %q", c)
		}
		seen[c] = true
	}
	if len(seen) < 195 {
		t.Errorf("only %d distinct codes of 200", len(seen))
	}
}

// Run deletes finished deliveries after 30 days (queued ones stay) and returns when its context
// ends.
func TestRunPrunesAndStops(t *testing.T) {
	e := newChanEnv(t)
	e.svc.linkPoll = 10 * time.Millisecond
	old := e.now.Add(-31 * 24 * time.Hour).UnixMilli()
	for _, state := range []string{"sent", "failed", "skipped", "queued"} {
		e.exec(t, `INSERT INTO deliveries (account_id, channel, event, subject, body, state, created_at) VALUES (?, 'email', 'errors', 's', 'b', ?, ?)`,
			e.f.Account.ID, state, old)
	}
	e.exec(t, `INSERT INTO deliveries (account_id, channel, event, subject, body, state, created_at) VALUES (?, 'email', 'errors', 's', 'b', 'sent', ?)`,
		e.f.Account.ID, e.now.UnixMilli())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.svc.Run(ctx)
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for e.count(t, `SELECT count(*) FROM deliveries`) != 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop")
	}
	if n := e.count(t, `SELECT count(*) FROM deliveries WHERE state = 'queued' OR created_at = ?`, e.now.UnixMilli()); n != 2 {
		t.Errorf("kept deliveries = %d, want the queued and the recent one", n)
	}
	if n := e.count(t, `SELECT count(*) FROM deliveries`); n != 2 {
		t.Errorf("deliveries = %d", n)
	}
}
