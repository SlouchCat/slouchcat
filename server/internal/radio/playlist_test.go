package radio

import (
	"strings"
	"testing"
	"time"
)

// bufferRelay returns a relay around a buffer in dir ("" = memory mode) with n segments of dur,
// generation gens(i) for segment i (nil: all 0).
func bufferRelay(t *testing.T, dir string, window time.Duration, reservation int64, n int, dur time.Duration, gens func(i int) int) *relay {
	t.Helper()
	buf, err := newBuffer(dir, window, reservation)
	if err != nil {
		t.Fatal(err)
	}
	r := &relay{buf: buf, lastGen: -1}
	at := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	for i := range n {
		gen := 0
		if gens != nil {
			gen = gens(i)
		}
		if _, ok := buf.inits[gen]; !ok {
			if err := buf.addInit(gen, []byte("init")); err != nil {
				t.Fatal(err)
			}
		}
		seg := &segment{gen: gen, dur: dur, pdt: at, disc: r.lastGen >= 0 && r.lastGen != gen}
		if err := buf.add(seg, make([]byte, 1000)); err != nil {
			t.Fatal(err)
		}
		r.lastGen = gen
		at = at.Add(dur)
	}
	return r
}

func TestPlaylistWindowLength(t *testing.T) {
	r := bufferRelay(t, t.TempDir(), 10*time.Minute, 1<<40, 200, 4*time.Second, nil)
	if n := len(r.buf.segs); n != 150 {
		t.Fatalf("%d segments in a 10-min window of 4-s segments, want 150", n)
	}
	pl := string(r.playlist("TOKEN", false))
	for _, want := range []string{"#EXT-X-VERSION:9\n", "#EXT-X-TARGETDURATION:4\n", "#EXT-X-SERVER-CONTROL:CAN-SKIP-UNTIL=36.0\n",
		"#EXT-X-START:TIME-OFFSET=-8\n", "#EXT-X-MEDIA-SEQUENCE:50\n", "#EXT-X-DISCONTINUITY-SEQUENCE:0\n",
		"#EXT-X-INDEPENDENT-SEGMENTS\n", "#EXT-X-MAP:URI=\"init/0?rt=TOKEN\"\n",
		"#EXT-X-PROGRAM-DATE-TIME:2026-11-07T10:03:20.000Z\n", "#EXTINF:4.000000,\nseg/50?rt=TOKEN\n", "seg/199?rt=TOKEN\n"} {
		if !strings.Contains(pl, want) {
			t.Errorf("playlist lacks %q", want)
		}
	}
	if strings.Count(pl, "#EXTINF:") != 150 || strings.Contains(pl, "CAN-BLOCK-RELOAD") {
		t.Errorf("playlist:\n%s", pl)
	}
	// Files outside the window are gone.
	if _, err := readSegment(&segment{}, r.buf.path(&segment{seq: 49})); err == nil {
		t.Error("segment 49 still on disk")
	}
}

func TestPlaylistDeltaSkip(t *testing.T) {
	r := bufferRelay(t, t.TempDir(), 30*time.Minute, 1<<40, 20, 4*time.Second, nil)
	full := string(r.playlist("T", false))
	delta := string(r.playlist("T", true))
	if strings.Contains(full, "EXT-X-SKIP") {
		t.Fatal("full playlist with EXT-X-SKIP")
	}
	// 80 s of segments: those ending at least 36 s before the end (0..10) are skipped.
	if !strings.Contains(delta, "#EXT-X-SKIP:SKIPPED-SEGMENTS=11\n") || strings.Count(delta, "#EXTINF:") != 9 {
		t.Fatalf("delta:\n%s", delta)
	}
	skip := strings.Index(delta, "#EXT-X-SKIP")
	mapAt := strings.Index(delta, "#EXT-X-MAP:")
	pdt := strings.Index(delta, "#EXT-X-PROGRAM-DATE-TIME:2026-11-07T10:00:44.000Z")
	first := strings.Index(delta, "seg/11?")
	if skip < 0 || mapAt < skip || pdt < mapAt || first < pdt {
		t.Fatalf("delta must repeat the map and a PDT at the first listed segment:\n%s", delta)
	}
	if !strings.Contains(delta, "#EXT-X-MEDIA-SEQUENCE:0\n") {
		t.Fatal("the media sequence of a delta stays the one of the full list")
	}
}

func TestPlaylistDiscontinuityKeepsMap(t *testing.T) {
	// Memory mode keeps 8 segments: generations 0 (5 segments), 1 (5), 2 (5).
	r := bufferRelay(t, "", 30*time.Minute, 1<<40, 15, 4*time.Second, func(i int) int { return i / 5 })
	pl := string(r.playlist("T", false))
	// Segments 7..14: the discontinuity of generation 1 (segment 5) left the window, the one of
	// generation 2 (segment 10) is listed.
	for _, want := range []string{"#EXT-X-MEDIA-SEQUENCE:7\n", "#EXT-X-DISCONTINUITY-SEQUENCE:1\n",
		"#EXT-X-MAP:URI=\"init/1?rt=T\"\n", "#EXT-X-DISCONTINUITY\n#EXT-X-MAP:URI=\"init/2?rt=T\"\n#EXT-X-PROGRAM-DATE-TIME:"} {
		if !strings.Contains(pl, want) {
			t.Errorf("playlist lacks %q:\n%s", want, pl)
		}
	}
	if strings.Count(pl, "#EXT-X-DISCONTINUITY\n") != 1 || strings.Count(pl, "#EXT-X-PROGRAM-DATE-TIME:") != 2 {
		t.Errorf("playlist:\n%s", pl)
	}
	if _, ok := r.buf.inits[0]; ok {
		t.Error("init of generation 0 kept after its last segment left")
	}
}

func TestBudgetEvictsOverReservation(t *testing.T) {
	// A reservation of 10 segments of 1000 bytes (plus the init): the window shrinks instead of
	// exceeding it.
	r := bufferRelay(t, t.TempDir(), 30*time.Minute, 10_004, 40, 4*time.Second, nil)
	if r.buf.bytes > 10_004 || len(r.buf.segs) != 10 {
		t.Fatalf("%d bytes in %d segments, want ≤ 10004 in 10", r.buf.bytes, len(r.buf.segs))
	}
	if r.buf.segs[0].seq != 30 {
		t.Fatalf("oldest segment %d, want 30 (the oldest go first)", r.buf.segs[0].seq)
	}
	if got := reservationBytes(128, 30*time.Minute); got != 29_664_000 {
		t.Fatalf("reservation of 128 kbit/s for 30 min: %d", got)
	}
	if got := maxRelaysByBudget(512, 30); got != 12 {
		t.Fatalf("relays by budget: %d", got)
	}
}
