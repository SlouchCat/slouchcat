package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestThrottle(t *testing.T) {
	var buf bytes.Buffer
	log, _ := New(&buf, "text", slog.LevelInfo)
	now := time.Unix(1000, 0)
	th := NewThrottle(log, time.Minute)
	th.now = func() time.Time { return now }
	for range 5 {
		th.Warn("bad_address", "push: cannot open")
	}
	th.Warn("other", "other reason")
	now = now.Add(time.Minute)
	th.Warn("bad_address", "push: cannot open")
	out := buf.String()
	if n := strings.Count(out, "bad_address"); n != 2 {
		t.Fatalf("%d lines for one reason, want 2:\n%s", n, out)
	}
	if !strings.Contains(out, "suppressed=4") || !strings.Contains(out, "reason=other") {
		t.Fatalf("counter or second reason missing:\n%s", out)
	}
}
