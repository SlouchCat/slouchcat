// Package telegram is the client of the Telegram Bot API for the Telegram channel (M8, A-6):
// getMe, sendMessage and getUpdates (long polling) over net/http with JSON bodies. It has no
// database and logs nothing. The bot token travels only in the URL path (/bot<token>/<method>),
// which package logging redacts; no returned error carries the URL or the token. Errors carry
// Telegram's description and wrap ErrUnauthorized (token rejected) or ErrUnreachable (network,
// timeout, 429, 5xx).
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultBaseURL is the real Bot API.
const DefaultBaseURL = "https://api.telegram.org"

// PollTimeout is how long getUpdates waits for an update (long polling).
const PollTimeout = 20 * time.Second

// callTimeout bounds a request other than getUpdates.
const callTimeout = 15 * time.Second

// maxText is the longest message Telegram accepts (characters).
const maxText = 4096

// Errors the API calls wrap.
var (
	ErrUnauthorized = errors.New("telegram: bot token rejected")
	ErrUnreachable  = errors.New("telegram: unreachable")
)

// tokenPattern is the shape of a bot token, <bot id>:<secret>; the logging redaction matches it.
var tokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)

// APIError is an answer with ok false.
type APIError struct {
	Method      string
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram %s: %d %s", e.Method, e.Code, e.Description)
}

// Unwrap maps 401 and 404 (unknown token) to ErrUnauthorized, 429 and 5xx to ErrUnreachable.
func (e *APIError) Unwrap() error {
	switch {
	case e.Code == http.StatusUnauthorized || e.Code == http.StatusNotFound:
		return ErrUnauthorized
	case e.Code == http.StatusTooManyRequests || e.Code >= 500:
		return ErrUnreachable
	}
	return nil
}

// Update is an incoming update; only messages are requested.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// Message is a message to the bot.
type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
}

// Chat is the chat a message came from; its id addresses the replies.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// Client calls the Bot API at BaseURL (DefaultBaseURL when empty; a mock in development).
type Client struct {
	BaseURL string
	HTTP    *http.Client // nil = http.DefaultClient; the calls bound themselves by context
}

// GetMe checks the token and returns the bot's username.
func (c *Client) GetMe(ctx context.Context, token string) (string, error) {
	var me struct {
		Username string `json:"username"`
	}
	if err := c.call(ctx, token, "getMe", struct{}{}, &me, callTimeout); err != nil {
		return "", err
	}
	return me.Username, nil
}

// SendMessage sends text (cut to Telegram's limit) to a chat, without link previews.
func (c *Client) SendMessage(ctx context.Context, token string, chatID int64, text string) error {
	if utf8.RuneCountInString(text) > maxText {
		text = string([]rune(text)[:maxText-1]) + "…"
	}
	params := struct {
		ChatID    int64  `json:"chat_id"`
		Text      string `json:"text"`
		NoPreview bool   `json:"disable_web_page_preview"`
	}{chatID, text, true}
	return c.call(ctx, token, "sendMessage", params, nil, callTimeout)
}

// GetUpdates returns the messages from offset on, waiting up to PollTimeout for the first one.
// The next call passes the highest update_id + 1 so Telegram drops the confirmed ones.
func (c *Client) GetUpdates(ctx context.Context, token string, offset int64) ([]Update, error) {
	params := struct {
		Offset         int64    `json:"offset"`
		Timeout        int      `json:"timeout"`
		AllowedUpdates []string `json:"allowed_updates"`
	}{offset, int(PollTimeout / time.Second), []string{"message"}}
	var updates []Update
	if err := c.call(ctx, token, "getUpdates", params, &updates, PollTimeout+callTimeout); err != nil {
		return nil, err
	}
	return updates, nil
}

func (c *Client) call(ctx context.Context, token, method string, params, out any, timeout time.Duration) error {
	if !tokenPattern.MatchString(token) {
		return &APIError{Method: method, Code: http.StatusUnauthorized, Description: "malformed bot token"}
	}
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, err)
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/bot"+token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram %s: invalid base URL", method) // the error would quote the URL
	}
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err // drop the URL with the token
		}
		return fmt.Errorf("telegram %s: %w: %w", method, ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var answer struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&answer); err != nil {
		if resp.StatusCode != http.StatusOK {
			return &APIError{Method: method, Code: resp.StatusCode, Description: http.StatusText(resp.StatusCode)}
		}
		return fmt.Errorf("telegram %s: unreadable answer: %w", method, err)
	}
	if !answer.OK {
		code := answer.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		return &APIError{Method: method, Code: code, Description: answer.Description}
	}
	if out != nil {
		if err := json.Unmarshal(answer.Result, out); err != nil {
			return fmt.Errorf("telegram %s: unreadable result: %w", method, err)
		}
	}
	return nil
}
