package main

import (
	"bytes"
	"log/slog"
	"reflect"
	"strings"
	"testing"
)

func TestParseOrigins(t *testing.T) {
	got, err := parseOrigins("https://Push.Example.Test, https://push-probe.example.test:8443/")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://push.example.test", "https://push-probe.example.test:8443"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseOrigins = %v, want %v", got, want)
	}
	for _, bad := range []string{"", "push.example.test", "ftp://push.example.test", "https://push.example.test/v1"} {
		if _, err := parseOrigins(bad); err == nil {
			t.Errorf("parseOrigins(%q) accepted", bad)
		}
	}
}

// Security review I-3: both replaced Google endpoints and an http origin are warned about.
func TestWarnStartup(t *testing.T) {
	run := func() string {
		var buf bytes.Buffer
		warnStartup(slog.New(slog.NewTextHandler(&buf, nil)), []string{"https://push.example.test"})
		return buf.String()
	}
	t.Setenv("RELAY_FCM_BASE_URL", "")
	t.Setenv("RELAY_FCM_TOKEN_URL", "")
	if out := run(); out != "" {
		t.Fatalf("warnings without overrides:\n%s", out)
	}
	t.Setenv("RELAY_FCM_TOKEN_URL", "http://fake.example.test/token")
	if out := run(); !strings.Contains(out, "RELAY_FCM_TOKEN_URL is set") || strings.Contains(out, "RELAY_FCM_BASE_URL") {
		t.Fatalf("token URL alone:\n%s", out)
	}
	t.Setenv("RELAY_FCM_BASE_URL", "http://fake.example.test")
	if out := run(); !strings.Contains(out, "RELAY_FCM_BASE_URL is set") || !strings.Contains(out, "RELAY_FCM_TOKEN_URL is set") {
		t.Fatalf("both overrides:\n%s", out)
	}
	var buf bytes.Buffer
	t.Setenv("RELAY_FCM_BASE_URL", "")
	t.Setenv("RELAY_FCM_TOKEN_URL", "")
	warnStartup(slog.New(slog.NewTextHandler(&buf, nil)), []string{"http://127.0.0.1:8181"})
	if !strings.Contains(buf.String(), "not https") {
		t.Fatalf("http origin:\n%s", buf.String())
	}
}
