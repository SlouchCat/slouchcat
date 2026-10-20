package playback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// W-8 guest cap: maxStreams of a guest account counts the streams of all its profiles.
func TestStartGuestStreamCap(t *testing.T) {
	f := vistest.New(t)
	f.AddGuest(t)
	if _, err := f.DB.Write.Exec(`UPDATE accounts SET max_streams = 1 WHERE id = ?`, f.Guest.ID); err != nil {
		t.Fatal(err)
	}
	f.Guest.Limits = model.PlaybackLimits{MaxStreams: 1}
	pg, pg18 := f.PG, f.PG18
	pg.Effective = pg.Limits.Cap(f.Guest.Limits)
	pg18.Effective = pg18.Limits.Cap(f.Guest.Limits)
	s := New(f.DB, vistest.Logger(), WithTokenKey(testKey))
	ctx := context.Background()
	m0, m6 := f.Rated(0), f.Rated(6)

	first, err := s.Start(ctx, browserSession(t, f, pg), m0.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	other := browserSession(t, f, pg18)
	_, err = s.Start(ctx, other, m6.ID, model.PlaybackRequest{})
	var sl *model.StreamLimitError
	if !errors.As(err, &sl) || *sl != (model.StreamLimitError{MaxStreams: 1, ActiveStreams: 1}) {
		t.Fatalf("second profile of the guest: %v", err)
	}
	// The owner account is not affected by the guest's cap.
	if _, err := s.Start(ctx, browserSession(t, f, f.P18), m6.ID, model.PlaybackRequest{}); err != nil {
		t.Errorf("owner: %v", err)
	}
	if err := s.StopStream(ctx, first.PlaybackSessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(ctx, other, m6.ID, model.PlaybackRequest{}); err != nil {
		t.Errorf("after the first stream ended: %v", err)
	}
}

// A start reserves its stream before its output waits for the budget: a second start of the
// profile meanwhile gets the stream-limit error at once instead of waiting for the first.
func TestStartReservesStreamDuringAdmission(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithTokenKey(testKey))
	ctx := context.Background()
	m6, m12 := f.Rated(6), f.Rated(12)
	f.SetMediaFile(t, m6.FileID, "mkv", "hevc", "aac", trackProbe(t, "hevc", "bt709", 1920, 1080, 8_000_000), nil)
	caps := capsChromeLinux
	one := f.AddLimitedProfile(t, "One", model.PlaybackLimits{MaxStreams: 1})
	a, b := browserSession(t, f, one), browserSession(t, f, one)

	var during error
	tc.onStart = func(HLSRequest) {
		tc.onStart = nil
		_, during = s.Start(ctx, b, m12.ID, model.PlaybackRequest{})
	}
	if _, err := s.Start(ctx, a, m6.ID, model.PlaybackRequest{Capabilities: &caps}); err != nil {
		t.Fatal(err)
	}
	var sl *model.StreamLimitError
	if !errors.As(during, &sl) || *sl != (model.StreamLimitError{MaxStreams: 1, ActiveStreams: 1}) {
		t.Errorf("start during the admission of another: %v", during)
	}
	if _, err := s.Start(ctx, b, m12.ID, model.PlaybackRequest{}); !errors.Is(err, model.ErrStreamLimit) {
		t.Errorf("start after the first was inserted: %v", err)
	}
	if r, a := s.starts.reserved(one.ID, one.AccountID); r != 0 || a != 0 {
		t.Errorf("reservations left: profile %d, account %d", r, a)
	}
}

// The start lock gives up with the context of the waiting start.
func TestStartLockCancellable(t *testing.T) {
	var l startLocks
	key := startKey{id: 7}
	unlock, err := l.lock(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := l.lock(ctx, key); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second lock: %v", err)
	}
	unlock()
	unlock, err = l.lock(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	if len(l.locks) != 0 {
		t.Errorf("locks left: %d", len(l.locks))
	}
}

// A switch whose request is invalid keeps the playback it would replace.
func TestStartInvalidSwitchKeepsPlayback(t *testing.T) {
	f := vistest.New(t)
	s := New(f.DB, vistest.Logger(), WithTokenKey(testKey))
	ctx := context.Background()
	m := f.Rated(6)
	session := browserSession(t, f, f.P18)
	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	nine := 9
	if _, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{AudioTrack: &nine}); !hasField(err, "audioTrack", "unknown_track") {
		t.Fatalf("invalid switch: %v", err)
	}
	if _, err := s.Authorize(ctx, tokenOf(t, info.StreamURL)); err != nil {
		t.Errorf("playback after an invalid switch: %v", err)
	}
}

// K-5 race: a progress event that uses up the limit while a start waits for its output revokes
// the profile's playback before the new row exists; the start then ends its own playback.
func TestStartRechecksWatchLimitAfterInsert(t *testing.T) {
	f := vistest.New(t)
	tc := newFakeTranscoder()
	clock := &manualClock{now: time.Date(2026, 10, 20, 18, 0, 0, 0, zurich)}
	s := New(f.DB, vistest.Logger(), WithClock(clock.Now), WithTranscoder(tc), WithTokenKey(testKey))
	ctx := context.Background()
	m6, m12 := f.Rated(6), f.Rated(12)
	f.SetMediaFile(t, m6.FileID, "mkv", "hevc", "aac", trackProbe(t, "hevc", "bt709", 1920, 1080, 8_000_000), nil)
	kid := f.AddLimitedProfile(t, "Kind", model.PlaybackLimits{WatchLimitMinutes: 1})
	caps := capsChromeLinux
	tc.onStart = func(HLSRequest) {
		tc.onStart = nil
		clock.Advance(time.Minute)
		if _, err := s.Report(ctx, kid, m12.ID, model.ProgressEvent{PositionMs: 60_000, DurationMs: vistest.DurationMs,
			PlayedMs: 60_000, Seq: 1, State: "playing", ClientTime: clock.Now()}); err != nil {
			t.Error(err)
		}
	}
	_, err := s.Start(ctx, browserSession(t, f, kid), m6.ID, model.PlaybackRequest{Capabilities: &caps})
	var wl *model.WatchLimitError
	if !errors.As(err, &wl) || *wl != (model.WatchLimitError{LimitMinutes: 1, UsedMinutes: 1}) {
		t.Fatalf("start racing the limit: %v", err)
	}
	var id, by string
	if err := f.DB.Read.QueryRow(`SELECT id, stopped_by FROM playback_sessions WHERE profile_id = ?`, kid.ID).Scan(&id, &by); err != nil {
		t.Fatal(err)
	}
	if by != model.StoppedByLimit {
		t.Errorf("stopped_by = %q", by)
	}
	if _, running := tc.Get(id); running {
		t.Error("output still running")
	}
}
