package logging

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"testing"
)

// fakeSecrets must never appear in log output.
var fakeSecrets = []string{
	"hunter2-FAKE", "tok_FAKE_123", "pin-FAKE-0815", "apikey-FAKE-xyz", "Bearer FAKE.jwt.value",
	"xapikey-FAKE", "s3cr3t-FAKE", "query-FAKE-key", "st-FAKE-token", "err-FAKE-key", "with-FAKE",
	"group-FAKE-pw", "url-FAKE-key", "msg-FAKE-key",
}

func logAll(t *testing.T, format Format) string {
	t.Helper()
	var buf bytes.Buffer
	log, err := New(&buf, format, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	log = log.With("token", "with-FAKE", "component", "test")

	log.Info("top-level keys",
		"password", "hunter2-FAKE",
		"Token", "tok_FAKE_123",
		"pin", "pin-FAKE-0815",
		slog.Group("profile", slog.Int("PIN", 4711)),
		"apiKey", "apikey-FAKE-xyz",
		"Authorization", "Bearer FAKE.jwt.value",
		"X-Api-Key", "xapikey-FAKE",
		"client_secret", "s3cr3t-FAKE",
		"user", "marco",
	)
	log.Info("nested group",
		slog.Group("request",
			slog.String("method", "GET"),
			slog.Group("auth", slog.String("Password", "group-FAKE-pw"), slog.Int("attempt", 2)),
		),
	)
	log.WithGroup("tmdb").Info("url strings",
		"url", "https://api.themoviedb.org/3/movie/1?language=de&api_key=query-FAKE-key",
		"stream", "/api/v1/items/1/hls/main.m3u8?st=st-FAKE-token&segment=3",
		"parsed", &url.URL{Scheme: "https", Host: "example.org", RawQuery: "api_key=url-FAKE-key"},
		"err", fmt.Errorf("fetch: %w", errors.New(`Get "https://x.test/?api_key=err-FAKE-key": timeout`)),
		"first", "first=kept",
	)
	log.Warn("message with https://x.test/?api_key=msg-FAKE-key inside")
	return buf.String()
}

func TestRedaction(t *testing.T) {
	for _, format := range []Format{FormatJSON, FormatText} {
		t.Run(string(format), func(t *testing.T) {
			out := logAll(t, format)
			t.Log("\n" + out)
			for _, secret := range fakeSecrets {
				if strings.Contains(out, secret) {
					t.Errorf("output contains secret %q", secret)
				}
			}
			for _, keep := range []string{
				"marco", "GET", "attempt", "language=de", "segment=3", "first=kept", "component",
				"api_key=" + Redacted, "st=" + Redacted, "api.themoviedb.org", "example.org",
			} {
				if !strings.Contains(out, keep) {
					t.Errorf("output lost non-secret %q", keep)
				}
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	for key, want := range map[string]bool{
		"password": true, "PIN": true, "offline_pin": true, "access_token": true, "api_key": true,
		"apiKey": true, "X-Api-Key": true, "authorization": true, "secret": true,
		"user": false, "path": false, "max_tokens": false, "mapping": false, "status": false,
	} {
		if got := isSensitiveKey(key); got != want {
			t.Errorf("isSensitiveKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestNewRejectsUnknownFormat(t *testing.T) {
	if _, err := New(&bytes.Buffer{}, "xml", slog.LevelInfo); err == nil {
		t.Fatal("expected an error for format xml")
	}
}
