package playback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// The stop watch (concept 13, getPlaybackSessionState): every stop path marks the playback
// stopped with its stopped_by value; only the browser session that started it gets an answer;
// unknown, foreign and forgotten ids are model.ErrNotFound; State answers with the database closed.
func TestPlaybackStateFromMemory(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddGuest(t)
	now := time.Now()
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithClock(func() time.Time { return now }))
	ctx := context.Background()
	m := f.Rated(6)
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", hevcProbe(t), nil)
	direct := f.Rated(0) // no probe: direct play; visible to every profile

	start := func(session model.Session, itemID string) string {
		t.Helper()
		info, err := s.Start(ctx, session, itemID, model.PlaybackRequest{})
		if err != nil {
			t.Fatal(err)
		}
		return info.PlaybackSessionID
	}
	state := func(session model.Session, id string) model.PlaybackState {
		t.Helper()
		st, err := s.State(session, id)
		if err != nil {
			t.Fatalf("state of %s: %v", id, err)
		}
		return st
	}
	notFound := func(session model.Session, id, what string) {
		t.Helper()
		if _, err := s.State(session, id); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("%s: %v, want not found", what, err)
		}
	}

	mine, other := browserSession(t, f, f.P18), browserSession(t, f, f.P18)
	a := start(mine, m.ID)
	if st := state(mine, a); st.Stopped || st.StoppedBy != "" {
		t.Errorf("running playback = %+v", st)
	}
	notFound(other, a, "another browser session of the same profile")
	notFound(mine, "01JUNKNOWN0000000000000000", "an unknown id")

	// Each stop path with its reason.
	if err := s.StopStream(ctx, a); err != nil {
		t.Fatal(err)
	}
	if st := state(mine, a); !st.Stopped || st.StoppedBy != model.StoppedByAdmin {
		t.Errorf("after the administration's stop = %+v", st)
	}
	notFound(other, a, "a stopped playback of another browser session")

	b := start(mine, direct.ID)
	if err := s.Stop(ctx, mine, b); err != nil {
		t.Fatal(err)
	}
	if st := state(mine, b); !st.Stopped || st.StoppedBy != model.StoppedByClient {
		t.Errorf("after the client's stop = %+v", st)
	}

	c := start(mine, direct.ID)
	replacing := start(mine, direct.ID) // a switch replaces the playback of the same title
	if st := state(mine, c); !st.Stopped || st.StoppedBy != model.StoppedByClient {
		t.Errorf("replaced playback = %+v", st)
	}
	if err := s.EndSession(ctx, mine.ID); err != nil {
		t.Fatal(err)
	}
	if st := state(mine, replacing); !st.Stopped || st.StoppedBy != model.StoppedByLogout {
		t.Errorf("after logout = %+v", st)
	}

	p6 := browserSession(t, f, f.P6)
	d := start(p6, direct.ID)
	if err := s.EndProfile(ctx, f.P6.ID); err != nil {
		t.Fatal(err)
	}
	if st := state(p6, d); !st.Stopped || st.StoppedBy != model.StoppedByRevoked {
		t.Errorf("after the profile was deleted = %+v", st)
	}

	guest := browserSession(t, f, f.PG)
	g := start(guest, direct.ID)
	if err := s.EndAccount(ctx, f.Guest.ID); err != nil {
		t.Fatal(err)
	}
	if st := state(guest, g); !st.Stopped || st.StoppedBy != model.StoppedByRevoked {
		t.Errorf("after the account lost its access = %+v", st)
	}

	// The sweep marks an expired playback stopped without a reason and forgets stopped entries
	// 10 minutes after their stop.
	e := start(other, direct.ID)
	now = now.Add(9 * time.Minute)
	s.sweep(ctx)
	if st := state(other, e); st.Stopped {
		t.Errorf("running playback after a sweep = %+v", st)
	}
	if st := state(mine, a); !st.Stopped {
		t.Errorf("stop forgotten after 9 minutes: %+v", st)
	}
	now = now.Add(SessionLifetime)
	s.sweep(ctx)
	if st := state(other, e); !st.Stopped || st.StoppedBy != "" {
		t.Errorf("expired playback = %+v", st)
	}
	notFound(mine, a, "a playback stopped more than 10 minutes ago")
	now = now.Add(stateRetention + time.Second)
	s.sweep(ctx)
	notFound(other, e, "an expired playback 10 minutes after the sweep saw it")

	// A restart starts with an empty registry: unknown, not stopped.
	before := browserSession(t, f, f.P18)
	r := start(before, direct.ID)
	if st := state(before, r); st.Stopped {
		t.Fatalf("running playback = %+v", st)
	}
	restarted := New(f.DB, vistest.Logger(), WithClock(func() time.Time { return now }))
	if _, err := restarted.State(before, r); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("after a restart: %v, want not found", err)
	}
	// The administration's stop names the browser session of the row: known again, stopped.
	if err := restarted.StopStream(ctx, r); err != nil {
		t.Fatal(err)
	}
	if st, err := restarted.State(before, r); err != nil || !st.Stopped || st.StoppedBy != model.StoppedByAdmin {
		t.Errorf("stopped by the administration after a restart = %+v, %v", st, err)
	}
	if _, err := restarted.State(other, r); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("another browser session after a restart: %v, want not found", err)
	}

	// A row deleted with its browser session (CASCADE) is never listed by the sweep again: the
	// running entry goes after activeRetention.
	orphan := start(before, f.Rated(6).ID)
	if _, err := f.DB.Write.ExecContext(ctx, `DELETE FROM playback_sessions WHERE id = ?`, orphan); err != nil {
		t.Fatal(err)
	}
	now = now.Add(activeRetention - time.Minute)
	s.sweep(ctx)
	if st := state(before, orphan); st.Stopped {
		t.Errorf("orphan before activeRetention = %+v", st)
	}
	now = now.Add(2 * time.Minute)
	s.sweep(ctx)
	notFound(before, orphan, "a running entry older than activeRetention")

	// No query on this path: with both database handles closed State still answers.
	live := browserSession(t, f, f.P18)
	h := start(live, direct.ID)
	if err := f.DB.Read.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.Write.Close(); err != nil {
		t.Fatal(err)
	}
	if st := state(live, h); st.Stopped {
		t.Errorf("running playback with the database closed = %+v", st)
	}
	notFound(other, h, "another browser session with the database closed")
}
