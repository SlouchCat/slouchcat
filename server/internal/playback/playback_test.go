package playback

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

func TestMimeType(t *testing.T) {
	cases := []struct{ container, video, audio, want string }{
		{"mp4", "h264", "aac", `video/mp4; codecs="avc1.64001F, mp4a.40.2"`},
		{"M4V", "H264", "AAC", `video/mp4; codecs="avc1.64001F, mp4a.40.2"`},
		{"mov", "hevc", "aac", "video/mp4"},
		{"mp4", "h264", "", "video/mp4"},
		{"mkv", "h264", "aac", "video/x-matroska"},
		{"webm", "vp9", "opus", "video/webm"},
		{"avi", "mpeg4", "mp3", "application/octet-stream"},
		{"", "", "", "application/octet-stream"},
	}
	for _, c := range cases {
		if got := MimeType(c.container, c.video, c.audio); got != c.want {
			t.Errorf("MimeType(%q, %q, %q) = %q, want %q", c.container, c.video, c.audio, got, c.want)
		}
	}
}

func TestCompleted(t *testing.T) {
	cases := []struct {
		pos, dur int64
		want     bool
	}{
		{899, 1000, false},
		{900, 1000, true},
		{1000, 1000, true},
		{0, 1000, false},
		{5, 0, false},
		{6_479_999, 7_200_000, false},
		{6_480_000, 7_200_000, true},
	}
	for _, c := range cases {
		if got := Completed(c.pos, c.dur); got != c.want {
			t.Errorf("Completed(%d, %d) = %v, want %v", c.pos, c.dur, got, c.want)
		}
	}
}

func TestValidate(t *testing.T) {
	ok := model.ProgressEvent{PositionMs: 1, DurationMs: 2, State: "paused"}
	if err := validate(ok); err != nil {
		t.Fatalf("valid event: %v", err)
	}
	bad := model.ProgressEvent{PositionMs: -1, DurationMs: 0, PlayedMs: -1, Seq: -1, State: "stopped", DeviceID: string(make([]byte, 65))}
	var verr *model.ValidationError
	if err := validate(bad); !errors.As(err, &verr) {
		t.Fatalf("invalid event: err = %v", err)
	}
	got := map[string]string{}
	for _, f := range verr.Fields {
		got[f.Field] = f.Code
	}
	want := map[string]string{"state": "invalid_value", "positionMs": "too_small", "durationMs": "too_small", "playedMs": "too_small", "seq": "too_small", "deviceId": "too_long"}
	for field, code := range want {
		if got[field] != code {
			t.Errorf("field %s: code %q, want %q (all %v)", field, got[field], code, got)
		}
	}
}

func countEvents(t *testing.T, f *vistest.Fixture, profileID int64, itemID string) int {
	t.Helper()
	var n int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM progress_events WHERE profile_id = ? AND item_id = ?`, profileID, itemID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestReportCompletesAndReopens(t *testing.T) {
	f := vistest.New(t)
	now := time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
	s := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return now }))
	ctx := context.Background()
	m := f.Rated(12)
	ev := func(pos int64, state string) model.ProgressEvent {
		return model.ProgressEvent{PositionMs: pos, DurationMs: vistest.DurationMs, PlayedMs: 10_000, Seq: 1, State: state, ClientTime: now}
	}

	p, err := s.Report(ctx, f.P18, m.ID, ev(vistest.DurationMs*95/100, "playing"))
	if err != nil {
		t.Fatal(err)
	}
	if !p.Completed || p.PositionMs != vistest.DurationMs*95/100 || !p.UpdatedAt.Equal(now) {
		t.Errorf("95 %% event = %+v, want completed", p)
	}
	info, err := s.Start(ctx, f.P18, m.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.ResumePositionMs != 0 {
		t.Errorf("completed title resumes at %d, want 0", info.ResumePositionMs)
	}

	// A later event before the 90 % mark reopens the title (AN-10).
	p, err = s.Report(ctx, f.P18, m.ID, ev(vistest.DurationMs/2, "paused"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Completed {
		t.Errorf("50 %% after completion = %+v, want reopened", p)
	}
	info, err = s.Start(ctx, f.P18, m.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if info.ResumePositionMs != vistest.DurationMs/2 {
		t.Errorf("resume = %d, want %d", info.ResumePositionMs, vistest.DurationMs/2)
	}
	if p, err = s.Report(ctx, f.P18, m.ID, ev(vistest.DurationMs, "ended")); err != nil || !p.Completed {
		t.Errorf("ended event = %+v, %v", p, err)
	}
	if n := countEvents(t, f, f.P18.ID, m.ID); n != 3 {
		t.Errorf("stored %d events, want 3", n)
	}

	// Invisible title: 404 and nothing stored.
	if _, err := s.Report(ctx, f.P6, m.ID, ev(1000, "playing")); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible title: err = %v, want ErrNotFound", err)
	}
	if n := countEvents(t, f, f.P6.ID, m.ID); n != 0 {
		t.Errorf("invisible title stored %d events", n)
	}
	var verr *model.ValidationError
	if _, err := s.Report(ctx, f.P18, m.ID, ev(-1, "playing")); !errors.As(err, &verr) {
		t.Errorf("negative position: err = %v, want validation error", err)
	}
}

func TestStartDescribesDirectPlay(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, vistest.Logger())
	m := f.Rated(6)
	info, err := s.Start(context.Background(), f.P6, m.ID, "", "device-1")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode != "direct" || info.MediaFile.ID != m.FileID || info.Item.ID != m.ID {
		t.Errorf("info = %+v", info)
	}
	if want := "/api/v1/items/" + m.ID + "/stream?file=" + m.FileID; info.StreamURL != want {
		t.Errorf("stream URL = %q, want %q", info.StreamURL, want)
	}
	if info.MediaFile.MimeType != `video/mp4; codecs="avc1.64001F, mp4a.40.2"` {
		t.Errorf("mime = %q", info.MediaFile.MimeType)
	}
	if info.ResumePositionMs != vistest.DurationMs/6 {
		t.Errorf("resume = %d, want the stored position", info.ResumePositionMs)
	}
	if _, err := s.Start(context.Background(), f.P6, f.Rated(16).ID, "", ""); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible title: err = %v", err)
	}
}

func TestOpenReadsThroughRoot(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, vistest.Logger())
	ctx := context.Background()
	m := f.Rated(0)
	src, err := s.Open(ctx, f.P6, m.ID, m.FileID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(src.File)
	_ = src.File.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, m.Content) || src.Size != vistest.FileSize || src.MediaType != "video/mp4" {
		t.Errorf("source = %d bytes, size %d, type %s", len(data), src.Size, src.MediaType)
	}
	if _, err := s.Open(ctx, f.P6, f.Rated(18).ID, ""); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible title: err = %v", err)
	}
	// A file that vanished from disk before the scanner noticed answers 404.
	if err := os.Remove(filepath.Join(f.MediaDir, filepath.FromSlash(m.RelPath))); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(ctx, f.P6, m.ID, ""); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("vanished file: err = %v, want ErrNotFound", err)
	}
}
