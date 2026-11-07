package radio

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

func TestTokenBoundToStation(t *testing.T) {
	e := newRelayEnv(t)
	e.st.Set("/a", mp3Mount(t, true))
	e.st.Set("/b", mp3Mount(t, true))
	a := e.addStation("A FM", "CH", "MP3", 128, e.st.URL("/a"))
	b := e.addStation("B FM", "CH", "MP3", 128, e.st.URL("/b"))
	_, la := e.listen(e.f.P18, a)
	_, lb := e.listen(e.f.P18, b)
	ctx := context.Background()
	for name, tc := range map[string][2]string{
		"token of A on B":     {lb.ListenID, la.Token},
		"B's id, A's station": {lb.ListenID, SignToken(testTokenKey, lb.ListenID, a)},
		"forged":              {la.ListenID, SignToken([]byte("other key"), la.ListenID, a)},
		"garbage":             {la.ListenID, "x.y"},
		"empty":               {la.ListenID, ""},
	} {
		if _, err := e.s.AuthorizeLive(ctx, tc[0], tc[1]); !errors.Is(err, model.ErrUnauthenticated) {
			t.Errorf("%s: %v, want ErrUnauthenticated", name, err)
		}
	}
	g := e.grant(la)
	if g.StationID != a || g.Profile.ID != e.f.P18.ID {
		t.Fatalf("grant %+v", g)
	}
	// A grant serves only the relay of its own station.
	if _, err := e.s.Playlist(ctx, model.RadioGrant{ListenID: la.ListenID, StationID: b}, ""); !errors.Is(err, model.ErrUnauthenticated) {
		t.Fatalf("grant of A for B's relay: %v", err)
	}
	pl, err := e.s.Playlist(ctx, g, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Init(ctx, g, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Init(ctx, g, 7); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown generation: %v", err)
	}
	first := segments(e.relay(a))[0].seq
	if _, done, err := e.s.Segment(ctx, g, first); err != nil || done == nil {
		t.Fatalf("segment %d: %v", first, err)
	}
	if _, _, err := e.s.Segment(ctx, g, first+100_000); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("segment outside the window: %v", err)
	}
	if want := "seg/" + strconv.FormatInt(first, 10) + "?rt=" + la.Token; !strings.Contains(string(pl), want) {
		t.Fatalf("playlist lacks %q", want)
	}
}

func TestListenRefusedAtWatchLimit(t *testing.T) {
	e := newRelayEnv(t)
	e.watch.limit, e.watch.remaining = 60, 0
	e.st.Set("/live", mp3Mount(t, true))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	_, err := e.s.Listen(context.Background(), e.f.Session(t, e.f.P18), id)
	var wl *model.WatchLimitError
	if !errors.As(err, &wl) || wl.LimitMinutes != 60 || wl.UsedMinutes != 60 {
		t.Fatalf("listen at the limit: %v", err)
	}
	if e.st.Connections("/live") != 0 {
		t.Fatal("a refused listen started a relay")
	}
}

func TestListenVisibility(t *testing.T) {
	e := newRelayEnv(t)
	e.st.Set("/live", mp3Mount(t, true))
	hidden := e.addStation("Hidden FM", "CH", "MP3", 128, e.st.URL("/live"))
	e.exec(`UPDATE radio_stations SET hidden = 1 WHERE id = ?`, hidden)
	de := e.addStation("DE FM", "DE", "MP3", 128, e.st.URL("/live"))
	e.exec(`UPDATE radio_config SET countries = '["CH"]'`)
	sess := e.f.Session(t, e.f.P18)
	for _, id := range []int64{hidden, de, 9999} {
		if _, err := e.s.Listen(context.Background(), sess, id); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("station %d: %v, want ErrNotFound", id, err)
		}
		if _, err := e.s.Now(context.Background(), sess, id, time.Time{}); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("now of station %d: %v, want ErrNotFound", id, err)
		}
	}
	if e.st.Connections("/live") != 0 {
		t.Fatal("a hidden or excluded station started a relay")
	}
}

func TestAuthorizeLiveSlidesExpiry(t *testing.T) {
	e := newRelayEnv(t, withFakeClock(time.Date(2026, 11, 7, 20, 0, 0, 0, time.UTC)))
	e.st.Set("/live", mp3Mount(t, true))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	_, l := e.listen(e.f.P18, id)
	ctx := context.Background()
	seen := map[int64]bool{e.listenRow(l.ListenID).LastSeenAt: true}
	for range 80 { // 40 min of live requests every 30 s, no heartbeat
		e.clock.Add(30 * time.Second)
		if _, err := e.s.AuthorizeLive(ctx, l.ListenID, l.Token); err != nil {
			t.Fatal(err)
		}
		e.s.sweep(ctx)
		row := e.listenRow(l.ListenID)
		if row.EndedAt.Valid {
			t.Fatalf("session ended (%s) although segments are fetched", row.EndedBy.String)
		}
		seen[row.LastSeenAt] = true
	}
	if n := len(seen); n < 39 || n > 41 {
		t.Fatalf("last_seen_at written %d times in 40 min, want once a minute", n)
	}
	// Without requests the session ends after the buffer length (30 min).
	e.clock.Add(30 * time.Minute)
	e.s.sweep(ctx)
	if row := e.listenRow(l.ListenID); row.EndedBy.String != model.RadioEndedExpired {
		t.Fatalf("ended by %q, want expired", row.EndedBy.String)
	}
	if _, err := e.s.AuthorizeLive(ctx, l.ListenID, l.Token); !errors.Is(err, model.ErrUnauthenticated) {
		t.Fatalf("live request after the expiry: %v", err)
	}
}

func TestSessionExpiresAfterBuffer(t *testing.T) {
	e := newRelayEnv(t, withFakeClock(time.Date(2026, 11, 7, 20, 0, 0, 0, time.UTC)))
	e.exec(`UPDATE radio_config SET buffer_minutes = 10`)
	e.st.Set("/live", mp3Mount(t, true))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	sess, l := e.listen(e.f.P18, id)
	ctx := context.Background()
	e.clock.Add(9 * time.Minute)
	hb, err := e.s.Heartbeat(ctx, sess, l.ListenID, false)
	if err != nil || !hb.ExpiresAt.Equal(e.clock.Now().Add(10*time.Minute)) || hb.WatchTime != nil {
		t.Fatalf("heartbeat %+v %v", hb, err)
	}
	if row := e.listenRow(l.ListenID); row.Playing != 0 {
		t.Fatal("pause not stored")
	}
	e.clock.Add(9 * time.Minute)
	e.s.sweep(ctx)
	if e.listenRow(l.ListenID).EndedAt.Valid {
		t.Fatal("session ended before the buffer length since the heartbeat")
	}
	e.clock.Add(time.Minute)
	e.s.sweep(ctx)
	if row := e.listenRow(l.ListenID); row.EndedBy.String != model.RadioEndedExpired {
		t.Fatalf("ended by %q, want expired", row.EndedBy.String)
	}
	if _, err := e.s.Heartbeat(ctx, sess, l.ListenID, true); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("heartbeat of an expired session: %v", err)
	}
}

func TestListenEndPaths(t *testing.T) {
	e := newRelayEnv(t)
	e.st.Set("/live", mp3Mount(t, true))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	ctx := context.Background()
	s1, l1 := e.listen(e.f.P18, id)
	_, l2 := e.listen(e.f.P12, id)
	_, l3 := e.listen(e.f.P6, id)
	s4, l4 := e.listen(e.f.P16, id)
	if err := e.s.EndListen(ctx, e.f.Session(t, e.f.P12), l1.ListenID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("another profile ends a listen: %v", err)
	}
	if err := e.s.EndSession(ctx, s1.ID, model.StoppedByLogout); err != nil {
		t.Fatal(err)
	}
	if err := e.s.EndProfile(ctx, e.f.P12.ID, model.StoppedByRevoked); err != nil {
		t.Fatal(err)
	}
	if err := e.s.StopListener(ctx, l3.ListenID); err != nil {
		t.Fatal(err)
	}
	if err := e.s.StopListener(ctx, l3.ListenID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("stopping an ended listener: %v", err)
	}
	if err := e.s.EndAccount(ctx, s4.Account.ID); err != nil {
		t.Fatal(err)
	}
	for l, by := range map[string]string{l1.ListenID: model.StoppedByLogout, l2.ListenID: model.StoppedByRevoked,
		l3.ListenID: model.StoppedByAdmin, l4.ListenID: model.StoppedByRevoked} {
		if row := e.listenRow(l); row.EndedBy.String != by {
			t.Errorf("listen %s ended by %q, want %q", l, row.EndedBy.String, by)
		}
	}
	if err := e.s.EndListen(ctx, s1, l1.ListenID); err != nil {
		t.Fatalf("ending an ended listen is idempotent: %v", err)
	}
	rel, err := e.s.Relays(ctx)
	if err != nil || len(rel.Relays) != 1 || len(rel.Relays[0].Listeners) != 0 {
		t.Fatalf("relays %+v %v", rel, err)
	}
}

func TestStartEndsSessionsOfTheLastRun(t *testing.T) {
	e := newRelayEnv(t)
	id := e.addStation("Test FM", "CH", "MP3", 128, "http://example.invalid/")
	sess := e.f.Session(t, e.f.P18)
	if err := appq.New(e.f.DB.Write).InsertRadioListenSession(context.Background(), appq.InsertRadioListenSessionParams{
		ID: "01J9RADIOLASTRUN0000000001", SessionID: sess.ID, AccountID: sess.Account.ID, ProfileID: e.f.P18.ID,
		StationID: id, Now: 1, ExpiresAt: 1 << 50}); err != nil {
		t.Fatal(err)
	}
	e.s.relayStart(context.Background())
	if row := e.listenRow("01J9RADIOLASTRUN0000000001"); row.EndedBy.String != model.RadioEndedRestart {
		t.Fatalf("ended by %q, want restart", row.EndedBy.String)
	}
}
