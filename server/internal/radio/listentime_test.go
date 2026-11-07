package radio

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// listenTime returns the radio_listen_time rows of a profile by quarter.
func (e *relayEnv) listenTime(profileID int64) map[int64]int64 {
	e.t.Helper()
	rows, err := e.f.DB.Read.Query(`SELECT quarter, played_ms FROM radio_listen_time WHERE profile_id = ?`, profileID)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var q, ms int64
		if err := rows.Scan(&q, &ms); err != nil {
			e.t.Fatal(err)
		}
		out[q] = ms
	}
	if err := rows.Err(); err != nil {
		e.t.Fatal(err)
	}
	return out
}

// fetch fetches segments from the first of the window until want of audio was delivered,
// advancing the clock by each segment's duration; it returns the delivered duration in whole ms
// per segment, as credited.
func (e *relayEnv) fetch(g model.RadioGrant, r *relay, want time.Duration) time.Duration {
	e.t.Helper()
	ctx := context.Background()
	var got time.Duration
	for n := segments(r)[0].seq; got < want; n++ {
		waitSegments(e.t, r, n+1)
		if _, err := e.s.AuthorizeLive(ctx, g.ListenID, e.tokens[g.ListenID]); err != nil {
			e.t.Fatal(err)
		}
		_, dur, err := r.segmentData(n)
		if err != nil {
			e.t.Fatal(err)
		}
		if _, _, err := e.s.Segment(ctx, g, n); err != nil {
			e.t.Fatal(err)
		}
		e.clock.Add(dur)
		got += dur.Truncate(time.Millisecond)
	}
	return got
}

func TestListenTimeFromDeliveredSegments(t *testing.T) {
	start := time.Date(2026, 11, 7, 10, 14, 50, 0, time.UTC)
	e := newRelayEnv(t, withFakeClock(start))
	e.st.Set("/live", mp3Mount(t, false))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	_, l := e.listen(e.f.P18, id)
	e.tokens = map[string]string{l.ListenID: l.Token}
	r := e.relay(id)
	g := e.grant(l)
	got := e.fetch(g, r, 2*time.Minute) // no heartbeat at all
	first := segments(r)[0].seq
	ctx := context.Background()
	if _, _, err := e.s.Segment(ctx, g, first); err != nil { // a re-fetch counts once
		t.Fatal(err)
	}
	e.s.sweep(ctx)
	rows := e.listenTime(e.f.P18.ID)
	q := start.UnixMilli() / quarterMs
	if len(rows) != 2 || rows[q] != 10_000 || rows[q+1] != got.Milliseconds()-10_000 {
		t.Fatalf("listen time %v, want 10 s in quarter %d and %d ms in the next", rows, q, got.Milliseconds()-10_000)
	}
}

func TestPausedSegmentsNotCounted(t *testing.T) {
	e := newRelayEnv(t, withFakeClock(time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)))
	e.st.Set("/live", mp3Mount(t, false))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	sess, l := e.listen(e.f.P18, id)
	e.tokens = map[string]string{l.ListenID: l.Token}
	ctx := context.Background()
	if _, err := e.s.Heartbeat(ctx, sess, l.ListenID, false); err != nil {
		t.Fatal(err)
	}
	r := e.relay(id)
	e.fetch(e.grant(l), r, 20*time.Second) // hls.js fills its buffer ahead after the pause
	e.s.sweep(ctx)
	if rows := e.listenTime(e.f.P18.ID); len(rows) != 0 {
		t.Fatalf("listen time while paused: %v", rows)
	}
}

func TestWatchLimitEndsListenSessions(t *testing.T) {
	e := newRelayEnv(t, withFakeClock(time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)))
	e.watch.limit, e.watch.remaining = 60, 1
	e.st.Set("/live", mp3Mount(t, false))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	sess, l := e.listen(e.f.P18, id)
	_, other := e.listen(e.f.P18, id)
	e.tokens = map[string]string{l.ListenID: l.Token}
	ctx := context.Background()
	hb, err := e.s.Heartbeat(ctx, sess, l.ListenID, true)
	if err != nil || hb.WatchTime == nil || hb.WatchTime.RemainingMinutes != 1 {
		t.Fatalf("heartbeat %+v %v", hb, err)
	}
	r := e.relay(id)
	e.fetch(e.grant(l), r, 10*time.Second)
	e.watch.set(0) // the flushed listen time used up the day
	e.s.sweep(ctx)
	for _, id := range []string{l.ListenID, other.ListenID} {
		if row := e.listenRow(id); row.EndedBy.String != model.StoppedByLimit {
			t.Fatalf("listen %s ended by %q, want limit", id, row.EndedBy.String)
		}
	}
	var wl *model.WatchLimitError
	if _, err := e.s.Heartbeat(ctx, sess, l.ListenID, true); !errors.As(err, &wl) || wl.LimitMinutes != 60 {
		t.Fatalf("heartbeat after the limit: %v", err)
	}
	if _, err := e.s.AuthorizeLive(ctx, l.ListenID, l.Token); !errors.Is(err, model.ErrUnauthenticated) {
		t.Fatalf("live request after the limit: %v", err)
	}
}
