package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/logging"
)

const testToken = "7000000001:AAE-secret_Token"

// fakeBot is an httptest Bot API: it answers getMe, sendMessage and getUpdates for testToken and
// records the bodies.
type fakeBot struct {
	srv     *httptest.Server
	mu      sync.Mutex
	calls   []string // method names
	bodies  []map[string]any
	updates []Update
	status  int // non-zero: every call answers this status with a JSON error
}

func newFakeBot(t *testing.T) *fakeBot {
	t.Helper()
	b := &fakeBot{}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		defer b.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if b.status != 0 {
			w.WriteHeader(b.status)
			_, _ = io.WriteString(w, `{"ok":false,"error_code":`+jsonInt(b.status)+`,"description":"Too Many Requests: retry after 5"}`)
			return
		}
		token, method, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, "/bot"), "/")
		if !ok || token != testToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"ok":false,"error_code":401,"description":"Unauthorized"}`)
			return
		}
		var body map[string]any
		if r.Header.Get("Content-Type") != "application/json" || json.NewDecoder(r.Body).Decode(&body) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		b.calls = append(b.calls, method)
		b.bodies = append(b.bodies, body)
		var result any
		switch method {
		case "getMe":
			result = map[string]any{"id": 7000000001, "is_bot": true, "username": "slouchcat_bot"}
		case "sendMessage":
			result = map[string]any{"message_id": 1}
		case "getUpdates":
			offset := int64(body["offset"].(float64))
			var out []Update
			for _, u := range b.updates {
				if u.UpdateID >= offset {
					out = append(out, u)
				}
			}
			result = out
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"ok":false,"error_code":404,"description":"Not Found"}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result})
	}))
	t.Cleanup(b.srv.Close)
	return b
}

func jsonInt(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestClientCalls(t *testing.T) {
	bot := newFakeBot(t)
	bot.updates = []Update{
		{UpdateID: 10, Message: &Message{Text: "/start ABC234", Chat: Chat{ID: 4242, Type: "private"}}},
		{UpdateID: 11, Message: &Message{Text: "hello", Chat: Chat{ID: 4243}}},
	}
	c := &Client{BaseURL: bot.srv.URL + "/"}
	ctx := context.Background()
	name, err := c.GetMe(ctx, testToken)
	if err != nil || name != "slouchcat_bot" {
		t.Fatalf("GetMe = %q, %v", name, err)
	}
	if err := c.SendMessage(ctx, testToken, 4242, "Hallo"); err != nil {
		t.Fatal(err)
	}
	updates, err := c.GetUpdates(ctx, testToken, 11)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].UpdateID != 11 || updates[0].Message.Chat.ID != 4243 {
		t.Errorf("updates = %+v", updates)
	}
	bot.mu.Lock()
	defer bot.mu.Unlock()
	if strings.Join(bot.calls, ",") != "getMe,sendMessage,getUpdates" {
		t.Errorf("calls = %v", bot.calls)
	}
	if b := bot.bodies[1]; b["chat_id"].(float64) != 4242 || b["text"] != "Hallo" || b["disable_web_page_preview"] != true {
		t.Errorf("sendMessage body = %v", b)
	}
	if b := bot.bodies[2]; b["offset"].(float64) != 11 || b["timeout"].(float64) != 20 {
		t.Errorf("getUpdates body = %v", b)
	}
}

// A long text is cut to Telegram's limit.
func TestSendMessageCutsLongText(t *testing.T) {
	bot := newFakeBot(t)
	c := &Client{BaseURL: bot.srv.URL}
	if err := c.SendMessage(context.Background(), testToken, 1, strings.Repeat("ä", 5000)); err != nil {
		t.Fatal(err)
	}
	bot.mu.Lock()
	defer bot.mu.Unlock()
	if n := utf8.RuneCountInString(bot.bodies[0]["text"].(string)); n != maxText {
		t.Errorf("text has %d characters", n)
	}
}

// Errors carry Telegram's description and the right sentinel, never the token.
func TestClientErrors(t *testing.T) {
	bot := newFakeBot(t)
	c := &Client{BaseURL: bot.srv.URL}
	ctx := context.Background()
	check := func(name string, err error, sentinel error, want string) {
		t.Helper()
		if !errors.Is(err, sentinel) {
			t.Errorf("%s: err = %v, want %v", name, err, sentinel)
		}
		if err != nil && (strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), want)) {
			t.Errorf("%s: error %q (want %q, no token)", name, err, want)
		}
	}
	_, err := c.GetMe(ctx, "7000000001:wrong-secret")
	check("wrong token", err, ErrUnauthorized, "Unauthorized")
	_, err = c.GetMe(ctx, "not a token/../secret")
	check("malformed token", err, ErrUnauthorized, "malformed")

	bot.mu.Lock()
	bot.status = http.StatusTooManyRequests
	bot.mu.Unlock()
	check("rate limited", c.SendMessage(ctx, testToken, 1, "x"), ErrUnreachable, "retry after")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	down := &Client{BaseURL: "http://" + addr}
	_, err = down.GetMe(ctx, testToken)
	check("unreachable", err, ErrUnreachable, "getMe")
	if err != nil && strings.Contains(err.Error(), "/bot") {
		t.Errorf("transport error carries the URL: %v", err)
	}
}

// One token shape (review N-2): every token ValidToken accepts, and so the settings store, is
// masked by the log redaction in a bot API path; other shapes are refused.
func TestValidTokenIsWhatTheLogMasks(t *testing.T) {
	for _, token := range []string{testToken, "1:Zq_-9", "123456789:AAE_x-Y_z0123456789abcdefghijklmnopq"} {
		if !ValidToken(token) {
			t.Errorf("ValidToken(%q) = false", token)
			continue
		}
		_, secret, _ := strings.Cut(token, ":")
		if got := logging.RedactString(DefaultBaseURL + "/bot" + token + "/getMe"); strings.Contains(got, secret) {
			t.Errorf("redacted %q still carries the secret", got)
		}
	}
	for _, token := range []string{"", "abc:def", "123:", ":abc", "123:abc def", "123:abc/../x", "123:abc:def"} {
		if ValidToken(token) {
			t.Errorf("ValidToken(%q) = true", token)
		}
	}
}
