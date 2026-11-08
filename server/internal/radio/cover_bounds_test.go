package radio

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Bounds of the cover chain against a hostile station (security review M-1, L-2, L-3) and the
// worker's lifecycle.

// titleRelay is a relay of station id with one published segment whose titles go through title
// (an unframed source: the stage-1 stamp).
func titleRelay(t *testing.T, s *Service, id int64) *relay {
	t.Helper()
	r := bufferRelay(t, "", 30*time.Minute, 1<<40, 1, 4*time.Second, nil)
	r.s, r.stationID, r.showsTitles, r.pdtGen = s, id, true, -1
	return r
}

func announce(r *relay, text string) {
	r.title(context.Background(), streamEvent{Title: text, TitleSet: true}, nil, &feedState{})
}

// One relay queues at most one lookup every 20 s; the entries in between keep their key and so
// still get the cover of a later lookup of it. Another relay has its own gap.
func TestCoverMinGapPerRelay(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)}
	s := New(nil, testLogger(), WithClock(clock.Now))
	if s.cov.minGap != 20*time.Second {
		t.Fatalf("gap %v", s.cov.minGap)
	}
	r, other := titleRelay(t, s, 1), titleRelay(t, s, 2)
	announce(r, "A - One")
	clock.Add(5 * time.Second)
	announce(r, "A - Two")
	announce(other, "B - One")
	if n := len(s.cov.queue); n != 2 || s.cov.queued[trackKey("A", "Two")] || r.titles[1].coverKey != trackKey("A", "Two") {
		t.Fatalf("%d queued after a title within 20 s, keys %v", n, s.cov.queued)
	}
	clock.Add(15 * time.Second)
	announce(r, "A - Three")
	if n := len(s.cov.queue); n != 3 {
		t.Fatalf("%d queued after 20 s, want 3", n)
	}
}

// The images stations name pass a limiter of their own: one every 5 s, two at once.
func TestCoverStreamImageLimiter(t *testing.T) {
	c := newCoverState()
	var allowed []bool
	for range 3 {
		allowed = append(allowed, c.stream.Allow())
	}
	if !allowed[0] || !allowed[1] || allowed[2] {
		t.Fatalf("stream image limiter %v, want two at once and then a wait", allowed)
	}
	e := newCoverEnv(t)
	e.s.cov.stream = ratelimit.New("test-stream-slow", 0.001, 1)
	e.fake.body("/stream/c.png", "image/png", pngImage(t, 200, 200))
	if _, row, ok := e.lookup("", "", e.fake.srv.URL+"/stream/c.png"); !ok || row.State != trackFound {
		t.Fatalf("row %+v %v", row, ok)
	}
	if e.s.cov.stream.Allow() || !e.s.cov.mb.Allow() {
		t.Fatal("the stream image did not take a token of its own limiter")
	}
}

// The sweep keeps the found covers within the budget: the oldest go first, with their files.
func TestCoverBudget(t *testing.T) {
	e := newCoverEnv(t)
	e.s.cov.maxFound = 2
	var dirs []string
	var keys []string
	for _, name := range []string{"a", "b", "c"} {
		e.fake.body("/stream/"+name+".png", "image/png", pngImage(t, 200, 200))
		j, row, ok := e.lookup("", "", e.fake.srv.URL+"/stream/"+name+".png")
		if !ok || row.State != trackFound {
			t.Fatalf("%s: %+v %v", name, row, ok)
		}
		keys = append(keys, j.key)
		dirs = append(dirs, filepath.Join(e.cache, coverDir(row.ID)))
		e.clock.Add(time.Minute)
	}
	e.s.sweepCovers(e.ctx, e.clock.Now())
	if _, ok := e.row(keys[0]); ok {
		t.Fatal("the oldest cover over the budget was kept")
	}
	if _, err := os.Stat(dirs[0]); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("files of a cover over the budget: %v", err)
	}
	for i := 1; i < 3; i++ {
		if _, ok := e.row(keys[i]); !ok {
			t.Fatalf("cover %d within the budget removed", i)
		}
		if _, err := os.Stat(dirs[i]); err != nil {
			t.Fatal(err)
		}
	}
}

// An ffmpeg run that hangs ends after the render timeout; the title has no cover then.
func TestCoverRenderTimeout(t *testing.T) {
	e := newCoverEnv(t)
	hang := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(hang, []byte("#!/bin/sh\nexec sleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	e.s.o.runner = transcode.Runner{FFmpegPath: hang}
	e.s.cov.render = 200 * time.Millisecond
	if s := newCoverState(); s.render != 30*time.Second {
		t.Fatalf("render timeout %v", s.render)
	}
	e.fake.body("/stream/c.png", "image/png", pngImage(t, 200, 200))
	start := time.Now()
	_, row, ok := e.lookup("", "", e.fake.srv.URL+"/stream/c.png")
	if d := time.Since(start); d > 10*time.Second || !ok || row.State != trackNone {
		t.Fatalf("after %v: %+v %v", d, row, ok)
	}
}

// A title that folds to more than 512 bytes («ß» becomes «ss») keeps a key the table accepts.
func TestCoverLongFoldedKey(t *testing.T) {
	e := newCoverEnv(t)
	e.fake.body("/ws/2/recording", "application/json", []byte(`{"recordings":[]}`))
	e.fake.body("/search", "application/json", []byte(`{"results":[]}`))
	entry, ok := cleanTitle(strings.Repeat("ß", 100) + " - " + strings.Repeat("ß", 400))
	if !ok {
		t.Fatal("no title")
	}
	j, row, found := e.lookup(entry.Artist, entry.Title, "")
	if !regexp.MustCompile(`^h:[0-9a-f]{64}$`).MatchString(j.key) || !found || row.State != trackNone {
		t.Fatalf("key %q, row %+v %v", j.key, row, found)
	}
	if k := trackKey("Band", "Song"); k != "band|song" {
		t.Fatalf("short key %q", k)
	}
}

// An entry a second relay adds while the lookup of its key runs gets the cover; after the lookup
// the key can be queued again, and that lookup is a cache hit.
func TestCoverDedupeAcrossRelays(t *testing.T) {
	e := newCoverEnv(t)
	e.s.cov.minGap = 0
	r1, r2, r3 := titleRelay(t, e.s, 1), titleRelay(t, e.s, 2), titleRelay(t, e.s, 3)
	e.s.rel.relays[1], e.s.rel.relays[2], e.s.rel.relays[3] = r1, r2, r3
	e.fake.body("/release/r/front-500", "image/png", pngImage(t, 500, 500))
	answer := mbAnswer(100, [4]string{"r", "Official", "Album", "g"})
	e.fake.route("/ws/2/recording", func(w http.ResponseWriter, _ *http.Request) {
		announce(r2, "Band - Song") // while the lookup runs: the key is queued already
		_, _ = w.Write(answer)
	})
	announce(r1, "Band - Song")
	j := <-e.s.cov.queue
	e.s.lookupCover(e.ctx, j)
	if len(e.s.cov.queue) != 0 || e.s.cov.queued[j.key] {
		t.Fatalf("queue %d, key still queued %v", len(e.s.cov.queue), e.s.cov.queued[j.key])
	}
	announce(r3, "Band - Song")
	if len(e.s.cov.queue) != 1 {
		t.Fatal("the key could not be queued again after its lookup")
	}
	n := e.fake.total()
	e.s.lookupCover(e.ctx, <-e.s.cov.queue)
	if e.fake.total() != n {
		t.Fatal("the second lookup was no cache hit")
	}
	for i, r := range []*relay{r1, r2, r3} {
		if r.titles[0].CoverTrackID == 0 || r.titles[0].CoverHash == "" {
			t.Fatalf("relay %d: entry without cover %+v", i+1, r.titles[0])
		}
	}
}

// Run returns only after the cover worker ended: no lookup runs on after it.
func TestRunWaitsForTheCoverWorker(t *testing.T) {
	e := newCoverEnv(t)
	started := make(chan struct{})
	e.fake.route("/stream/slow.png", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(e.ctx)
	ran := make(chan struct{})
	go func() {
		defer close(ran)
		e.s.Run(ctx)
	}()
	j, _ := coverJobFor(titleEntry{streamURL: e.fake.srv.URL + "/stream/slow.png"})
	e.s.queueCover(j)
	<-started
	cancel()
	<-ran
	e.s.cov.queue <- coverJob{key: "after"}
	time.Sleep(100 * time.Millisecond)
	if len(e.s.cov.queue) != 1 {
		t.Fatal("a cover worker still runs after Run returned")
	}
}
