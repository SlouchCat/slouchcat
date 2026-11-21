package logging

import (
	"bytes"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"testing"
)

func TestRedactsSecretKeys(t *testing.T) {
	var buf bytes.Buffer
	log, err := New(&buf, "json", slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	secrets := map[string]string{
		"fcmToken":          "fcm-secret-1",
		"credentials":       "cred-secret-2",
		"private_key":       "pk-secret-3",
		"sealingKey":        "seal-secret-4",
		"authorization":     "Bearer secret-5",
		"pickup":            "pickup-secret-6",
		"pickupHash":        "hash-secret-7",
		"vapidKey":          "vapid-secret-8",
		"endpoint":          "https://push.example.test/v1/push/1.secret-9",
		"packet":            "packet-secret-10",
		"access_token":      "access-secret-11",
		"X-Forwarded-For":   "addr-secret-12",
		"remote_addr":       "addr-secret-13",
		"client_ip":         "addr-secret-14",
		"assertion":         "jwt-secret-15",
		"meeting_point_id":  "id-secret-16",
		"firebase.key":      "key-secret-17",
		"caller":            "caller-secret-18",
		"host":              "host-secret-19",
		"blob":              "blob-secret-20",
		"service_account.x": "",
	}
	var attrs []any
	for k, v := range secrets {
		if v != "" {
			attrs = append(attrs, k, v)
		}
	}
	log.Info("message", attrs...)
	log.With("token", "with-secret-21").WithGroup("g").Info("grouped", "secret", "group-secret-22")
	out := buf.String()
	for _, s := range []string{"secret-1", "secret-2", "secret-3", "secret-4", "secret-5", "secret-6", "secret-7", "secret-8",
		"secret-9", "secret-10", "secret-11", "secret-12", "secret-13", "secret-14", "secret-15", "secret-16", "secret-17",
		"secret-18", "secret-19", "secret-20", "secret-21", "secret-22"} {
		if strings.Contains(out, s) {
			t.Errorf("log contains %q:\n%s", s, out)
		}
	}
}

func TestRedactsAddressesAndPaths(t *testing.T) {
	cases := map[string]string{
		"dial tcp 192.0.2.10:443: connection refused":     "dial tcp [redacted]:443: connection refused",
		"from [2001:db8::1]:51234":                        "from [redacted]:51234",
		"from 2001:db8:0:1::42 now":                       "from [redacted] now",
		"mapped ::ffff:198.51.100.7":                      "mapped [redacted]",
		"PUT /v1/pair/d4IH25H6bSo7RyM_N_8fTg":             "PUT /v1/pair/[redacted]",
		"POST https://push.example.test/v1/push/1.AbC_-x": "POST https://push.example.test/v1/push/[redacted]",
		"at 10:00:00 the build 1.2.3 ran":                 "at 10:00:00 the build 1.2.3 ran",
		"2026-11-21T10:00:00Z":                            "2026-11-21T10:00:00Z",
	}
	for in, want := range cases {
		if got := RedactString(in); got != want {
			t.Errorf("RedactString(%q) = %q, want %q", in, got, want)
		}
	}
}

type addrStringer struct{}

func (addrStringer) String() string { return "198.51.100.9" }

func TestRedactsErrorsURLsAndAny(t *testing.T) {
	var buf bytes.Buffer
	log, _ := New(&buf, "text", slog.LevelInfo)
	u, _ := url.Parse("https://push.example.test/v1/push/1.blobsecret")
	log.Error("failed", "err", errors.New("read tcp 192.0.2.44:8080->198.51.100.3:443: reset"), "url", u,
		"stringer", addrStringer{}, "list", []string{"203.0.113.5"})
	out := buf.String()
	for _, s := range []string{"192.0.2.44", "198.51.100.3", "blobsecret", "198.51.100.9", "203.0.113.5"} {
		if strings.Contains(out, s) {
			t.Errorf("log contains %q:\n%s", s, out)
		}
	}
	if _, err := New(&buf, "xml", slog.LevelInfo); err == nil {
		t.Error("unknown format accepted")
	}
}
