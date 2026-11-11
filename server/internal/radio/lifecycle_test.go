package radio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Listen sessions and relays across their lifecycle (review S-L1, S-L2, CR-2, CR-3, CR-7, CR-8).

// forceTouch makes the next live request of a listen session write its touch.
func (e *relayEnv) forceTouch(listenID string) {
	e.s.rel.mu.Lock()
	e.s.rel.sessions[listenID].touched = time.Time{}
	e.s.rel.mu.Unlock()
}

// A listen token outlives no login session: the touch of a live request checks it.
func TestLiveEndsWithRevokedLoginSession(t *testing.T) {
	e := newRelayEnv(t)
	e.st.Set("/live", mp3Mount(t, true))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	sess, l := e.listen(e.f.P18, id)
	e.forceTouch(l.ListenID)
	e.grant(l) // a valid login session passes the check
	e.exec(`UPDATE sessions SET revoked_at = ? WHERE id = ?`, time.Now().UnixMilli(), sess.ID)
	if _, err := e.s.AuthorizeLive(context.Background(), l.ListenID, l.Token); err != nil {
		t.Fatalf("live request before the next touch: %v", err) // checked at most once a minute
	}
	e.forceTouch(l.ListenID)
	if _, err := e.s.AuthorizeLive(context.Background(), l.ListenID, l.Token); !errors.Is(err, model.ErrUnauthenticated) {
		t.Fatalf("live request of a revoked login session: %v, want ErrUnauthenticated", err)
	}
	if row := e.listenRow(l.ListenID); row.EndedBy.String != model.StoppedByRevoked {
		t.Fatalf("listen session ended by %q, want revoked", row.EndedBy.String)
	}
	// An expired login session ends its listen session the same way.
	sess2, l2 := e.listen(e.f.P18, id)
	e.exec(`UPDATE sessions SET expires_at = ? WHERE id = ?`, time.Now().Add(-time.Minute).UnixMilli(), sess2.ID)
	e.forceTouch(l2.ListenID)
	if _, err := e.s.AuthorizeLive(context.Background(), l2.ListenID, l2.Token); !errors.Is(err, model.ErrUnauthenticated) {
		t.Fatalf("live request of an expired login session: %v", err)
	}
}

// A station that leaves the offer while a profile listens (PB-12): a demarked one and one whose
// country goes; the apply ends their listen sessions and stops their relays, the next heartbeat
// answers 404 and a live request 401: invisible means 404, also for a listener already playing.
func TestStationLeavingTheOfferEndsListening(t *testing.T) {
	e := newRelayEnv(t)
	ctx := context.Background()
	e.st.Set("/a", mp3Mount(t, true))
	e.st.Set("/b", mp3Mount(t, true))
	a := e.addStation("A FM", "CH", "MP3", 128, e.st.URL("/a"))
	b := e.addStation("B FM", "DE", "MP3", 128, e.st.URL("/b"))
	sess, la := e.listen(e.f.P18, a)
	_, lb := e.listen(e.f.P18, b)
	o, err := e.s.offer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// b is in the offer through its mark; removing its country takes the exceptions of its
	// stations with it.
	if _, err := e.s.ApplyOffer(ctx, o.version, []model.RadioOfferStep{{Op: model.RadioOfferDemark, StationIDs: []int64{a}},
		{Op: model.RadioOfferRemoveCountry, Country: "DE"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Heartbeat(ctx, sess, la.ListenID, true); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("heartbeat after the station left the offer: %v, want ErrNotFound", err)
	}
	for name, tc := range map[string]struct {
		id int64
		l  model.RadioListen
	}{"demarked": {a, la}, "country removed": {b, lb}} {
		e.s.rel.mu.Lock()
		r := e.s.rel.relays[tc.id]
		e.s.rel.mu.Unlock()
		if r != nil {
			t.Errorf("%s: relay still runs", name)
		}
		if row := e.listenRow(tc.l.ListenID); row.EndedBy.String != model.StoppedByAdmin {
			t.Errorf("%s: listen session ended by %q, want admin", name, row.EndedBy.String)
		}
		if _, err := e.s.AuthorizeLive(ctx, tc.l.ListenID, tc.l.Token); !errors.Is(err, model.ErrUnauthenticated) {
			t.Errorf("%s: live request: %v", name, err)
		}
	}
}

// A relay that a Listen is starting or joining is busy for the sweep, even before its first
// session (the relay of a slow station takes up to readyTimeout).
func TestSweepKeepsJoiningRelay(t *testing.T) {
	e := newRelayEnv(t)
	ctx := context.Background()
	e.st.Set("/live", mp3Mount(t, true))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	cfg, err := appq.New(e.f.DB.Read).GetRadioConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	r, err := e.s.joinRelay(ctx, e.stationRow(id), cfg)
	if err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	running := func() bool {
		e.s.rel.mu.Lock()
		defer e.s.rel.mu.Unlock()
		return e.s.rel.relays[id] == r
	}
	e.s.stopIdleRelays(ctx, later)
	if !running() {
		t.Fatal("the sweep stopped a relay with a pending join")
	}
	e.s.leaveRelay(r)
	e.s.stopIdleRelays(ctx, later) // idle from now on
	if !running() {
		t.Fatal("the sweep stopped the relay before its grace")
	}
	e.s.stopIdleRelays(ctx, later.Add(time.Duration(cfg.GraceSeconds+1)*time.Second))
	if running() {
		t.Fatal("the sweep kept an idle relay after its grace")
	}
}

// A relay that is ending is not joined: the Listen gets its error.
func TestJoinRefusesEndingRelay(t *testing.T) {
	e := newRelayEnv(t)
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	ending := &relay{stationID: id, state: relayEnding, err: fmt.Errorf("%w: test", model.ErrRadioNotAudio),
		done: make(chan struct{})}
	e.s.rel.mu.Lock()
	e.s.rel.relays[id] = ending
	e.s.rel.mu.Unlock()
	t.Cleanup(func() {
		e.s.rel.mu.Lock()
		delete(e.s.rel.relays, id)
		e.s.rel.mu.Unlock()
	})
	cfg, err := appq.New(e.f.DB.Read).GetRadioConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r, err := e.s.joinRelay(context.Background(), e.stationRow(id), cfg); !errors.Is(err, model.ErrRadioNotAudio) || r != nil {
		t.Fatalf("join of an ending relay: %v %v", r, err)
	}
}

// Once Run ended (shutdown), no relay starts any more.
func TestNoRelayAfterRun(t *testing.T) {
	e := newRelayEnv(t)
	e.st.Set("/live", mp3Mount(t, true))
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.s.Run(ctx)
		close(done)
	}()
	cancel()
	<-done
	if _, err := e.s.Listen(context.Background(), e.f.Session(t, e.f.P18), id); !errors.Is(err, model.ErrRadioUnreachable) {
		t.Fatalf("listen after Run: %v, want ErrRadioUnreachable", err)
	}
	e.s.rel.mu.Lock()
	defer e.s.rel.mu.Unlock()
	if len(e.s.rel.relays) != 0 {
		t.Fatalf("%d relays after Run", len(e.s.rel.relays))
	}
}

// Every relay has its own buffer directory: the old relay of a station that ends after the next
// one started removes only its own files. Directories of the last process go at the start.
func TestRelayBufferDirPerRelay(t *testing.T) {
	e := newRelayEnv(t)
	st := e.stationRow(e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live")))
	old, err := e.s.newRelay(st, nil, time.Minute, 1<<20, e.dir)
	if err != nil {
		t.Fatal(err)
	}
	next, err := e.s.newRelay(st, nil, time.Minute, 1<<20, e.dir)
	if err != nil {
		t.Fatal(err)
	}
	if old.buf.dir == next.buf.dir {
		t.Fatalf("both relays use %s", old.buf.dir)
	}
	old.buf.remove()
	if _, err := os.Stat(next.buf.dir); err != nil {
		t.Fatalf("the old relay removed the buffer of the next one: %v", err)
	}
	keep := filepath.Join(e.dir, "keep")
	for _, d := range []string{"12", "12-" + ulid.Make().String(), "keep"} {
		if err := os.MkdirAll(filepath.Join(e.dir, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	e.s.removeStaleBuffers()
	entries, err := os.ReadDir(e.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(keep) {
		t.Fatalf("after the start: %v, want only %s", entries, keep)
	}
}
