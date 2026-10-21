package logging

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
)

// The ring keeps Error records only, redacted like the log itself, with source and job taken from
// the attributes, newest first and at most its size.
func TestErrorRing(t *testing.T) {
	var buf bytes.Buffer
	log, ring, err := NewWithErrors(&buf, FormatJSON, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("not kept", "component", "x")
	log.Warn("not kept either")
	log.With("component", "metadata").Error("tmdb request failed",
		"api_key", "ring-FAKE-key",
		"url", "https://api.themoviedb.org/3/movie/1?api_key=ringq-FAKE-key",
		"err", fmt.Errorf("fetch: %w", errors.New(`Get "https://x.test/?api_key=ringerr-FAKE-key": timeout`)),
		slog.Group("smtp", slog.String("password", "ring-FAKE-pass")),
	)
	log.With("worker", "worker-1", "job_id", int64(42), "job_type", "metadata.match").Error("job failed finally", "err", "boom")
	log.WithGroup("library").Error("watch failed", "bot", "/bot123:ringbot-FAKE-token/getMe")
	log.Error("plain")

	entries := ring.Entries()
	if len(entries) != 4 {
		t.Fatalf("entries = %d, want 4: %+v", len(entries), entries)
	}
	if entries[0].Message != "plain" || entries[0].Source != "server" {
		t.Errorf("newest = %+v, want the plain record from source server", entries[0])
	}
	if e := entries[1]; e.Source != "library" || e.Message != "watch failed" {
		t.Errorf("group record = %+v, want source library", e)
	}
	if e := entries[2]; e.JobID != 42 || e.JobType != "metadata.match" || e.Source != "metadata.match" || e.Attrs["err"] != "boom" {
		t.Errorf("job record = %+v", e)
	}
	if e := entries[3]; e.Source != "metadata" || e.Attrs["api_key"] != Redacted || e.Attrs["smtp.password"] != Redacted {
		t.Errorf("first record = %+v", e)
	}
	for _, e := range entries {
		text := e.Message + fmt.Sprint(e.Attrs)
		for _, secret := range []string{"ring-FAKE-key", "ringq-FAKE-key", "ringerr-FAKE-key", "ring-FAKE-pass", "ringbot-FAKE-token"} {
			if strings.Contains(text, secret) {
				t.Errorf("ring entry carries %q: %+v", secret, e)
			}
		}
	}
	// The output handler still gets every record.
	if n := strings.Count(buf.String(), "\n"); n != 6 {
		t.Errorf("log lines = %d, want 6", n)
	}
	// Entries are copies.
	entries[3].Attrs["api_key"] = "changed"
	if ring.Entries()[3].Attrs["api_key"] != Redacted {
		t.Error("Entries shares its maps with the ring")
	}
}

func TestErrorRingKeepsTheNewest(t *testing.T) {
	ring := NewErrorRing(3)
	log := slog.New(NewRedactingHandler(ring.Wrap(slog.NewTextHandler(io.Discard, nil))))
	for i := range 5 {
		log.Error(fmt.Sprintf("error %d", i))
	}
	var got []string
	for _, e := range ring.Entries() {
		got = append(got, e.Message)
	}
	if want := []string{"error 4", "error 3", "error 2"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("entries = %v, want %v", got, want)
	}
	if len(NewErrorRing(3).Entries()) != 0 {
		t.Error("an empty ring has entries")
	}
}
