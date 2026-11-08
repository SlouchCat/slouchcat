package radio

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/radio/radiotest"
)

// Acceptance test of the title timeline (concept 16 section 7.6, build plan A1): the test station
// announces titles at known byte offsets; every entry's time minus the start of its generation
// must be the audio time of that offset, at most one segment (4 s) off.

// frameWalk returns the byte ends of the frames of a test file and the duration of one frame. It
// reads the files of testdata/radio/generate.sh (MP3: MPEG-1 Layer III at 44.1 kHz after an ID3v2
// tag; ADTS: one raw data block at 44.1 kHz) by itself, not with the framer under test.
func frameWalk(t *testing.T, codec string, file []byte) ([]int, float64) {
	t.Helper()
	i, per := 0, 1024.0/44100
	if codec == "mp3" {
		per = 1152.0 / 44100
		if string(file[:3]) == "ID3" {
			i = 10 + (int(file[6])<<21 | int(file[7])<<14 | int(file[8])<<7 | int(file[9]))
		}
	}
	kbps := [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	var ends []int
	for i+7 <= len(file) {
		if file[i] != 0xFF {
			t.Fatalf("%s: no frame at byte %d", codec, i)
		}
		n := 0
		switch codec {
		case "mp3":
			if (file[i+2]>>2)&3 != 0 { // 44.1 kHz
				t.Fatalf("mp3 frame at %d is not 44.1 kHz", i)
			}
			n = 144*kbps[file[i+2]>>4]*1000/44100 + int(file[i+2]>>1)&1
		default:
			if file[i+6]&3 != 0 {
				t.Fatalf("adts frame at %d has more than one raw data block", i)
			}
			n = int(file[i+3]&3)<<11 | int(file[i+4])<<3 | int(file[i+5]>>5)
		}
		if n <= 0 {
			t.Fatalf("%s: frame of %d bytes at %d", codec, n, i)
		}
		i += n
		ends = append(ends, i)
	}
	return ends, per
}

// audioAt returns the audio time of the whole frames before byte off of a connection that loops
// file (the test station repeats it).
func audioAt(t *testing.T, codec string, file []byte, off int) time.Duration {
	t.Helper()
	ends, per := frameWalk(t, codec, file)
	whole := 0
	for _, e := range ends {
		if e <= off%len(file) {
			whole++
		}
	}
	secs := float64(off/len(file)*len(ends)+whole) * per
	return time.Duration(secs * float64(time.Second))
}

// genStart returns the program date-time of the first segment of generation gen in the window.
func genStart(t *testing.T, r *relay, gen int) time.Time {
	t.Helper()
	for _, s := range segments(r) {
		if s.gen == gen {
			return s.pdt
		}
	}
	t.Fatalf("no segment of generation %d in the window", gen)
	return time.Time{}
}

// waitTitles waits until the relay's timeline holds n resolved entries and returns the timeline.
func waitTitles(t *testing.T, r *relay, n int) []titleEntry {
	t.Helper()
	deadline := time.After(60 * time.Second)
	for {
		r.mu.Lock()
		resolved := 0
		for _, e := range r.titles {
			if !e.pending {
				resolved++
			}
		}
		titles, ch := append([]titleEntry(nil), r.titles...), r.changed
		r.mu.Unlock()
		if resolved >= n {
			return titles
		}
		select {
		case <-ch:
		case <-r.done:
			t.Fatalf("relay ended with %d of %d titles: %v", resolved, n, relayError(r))
		case <-deadline:
			t.Fatalf("%d of %d titles after 60 s: %+v", resolved, n, titles)
		}
	}
}

// wantTitle is an announced title with the audio of its connection and the byte offset it starts
// at; base is the audio the generation had before that connection.
type wantTitle struct {
	text  string
	gen   int
	codec string
	file  []byte
	base  time.Duration
	off   int
}

// checkTimeline compares the timeline with want and returns the largest deviation.
func checkTimeline(t *testing.T, r *relay, titles []titleEntry, want []wantTitle) time.Duration {
	t.Helper()
	if len(titles) < len(want) {
		t.Fatalf("%d titles, want %d: %+v", len(titles), len(want), titles)
	}
	var worst time.Duration
	for i, w := range want {
		e := titles[i]
		if e.Text != w.text || e.gen != w.gen || e.pending {
			t.Fatalf("title %d is %q of generation %d (pending %v), want %q of %d", i, e.Text, e.gen, e.pending, w.text, w.gen)
		}
		got := e.At.Sub(genStart(t, r, w.gen))
		exp := w.base + audioAt(t, w.codec, w.file, w.off)
		dev := time.Duration(math.Abs(float64(got - exp)))
		t.Logf("%-14s generation %d: at %v, audio position %v, deviation %v", w.text, w.gen, got, exp, dev)
		if dev > targetDuration*time.Second {
			t.Errorf("%q stamped %v after its generation start, its audio starts at %v: more than one segment off", w.text, got, exp)
		}
		worst = max(worst, dev)
	}
	return worst
}

// The relay reads faster than real time (no pacing in the tests), so a stamp by the wall clock
// would drift; the stamp by audio position does not. The connection breaks after the MP3 part
// and comes back with AAC: a new generation, whose titles count from its own start.
func TestTimelineByAudioPosition(t *testing.T) {
	e := newRelayEnv(t)
	mp3, aac := testAudio(t, "tone-mp3-128.mp3"), testAudio(t, "tone-aac-64.aac")
	const mi = 8192
	e.st.Set("/live", radiotest.Mount{Kind: radiotest.Icecast, Audio: [][]byte{mp3, aac}, CutAfter: []int{70 * mi}, MetaInt: mi,
		Name: "Test FM", Titles: []radiotest.Title{
			{At: 0, Text: "Test FM"}, {At: 1 * mi, Text: "Early - Start"}, {At: 20 * mi, Text: "A - One"},
			{At: 50 * mi, Text: "B - Two"}, {At: 90 * mi, Text: "C - Three"}}})
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	e.listen(e.f.P18, id)
	r := e.relay(id)
	titles := waitTitles(t, r, 7)
	worst := checkTimeline(t, r, titles, []wantTitle{
		{"Early - Start", 0, "mp3", mp3, 0, 1 * mi},
		{"A - One", 0, "mp3", mp3, 0, 20 * mi},
		{"B - Two", 0, "mp3", mp3, 0, 50 * mi},
		{"Early - Start", 1, "aac", aac, 0, 1 * mi},
		{"A - One", 1, "aac", aac, 0, 20 * mi},
		{"B - Two", 1, "aac", aac, 0, 50 * mi},
		{"C - Three", 1, "aac", aac, 0, 90 * mi},
	})
	t.Logf("largest deviation %v", worst)
	// The packager keeps the audio time: generation 0 (MP3 transcoded to AAC) lasts what was fed.
	var out time.Duration
	for _, s := range segments(r) {
		if s.gen == 0 {
			out += s.dur
		}
	}
	fed := audioAt(t, "mp3", mp3, 70*mi)
	dev := time.Duration(math.Abs(float64(out - fed)))
	t.Logf("generation 0: %v of segments for %v of audio fed, deviation %v", out, fed, dev)
	if dev > targetDuration*time.Second {
		t.Errorf("generation 0 published %v for %v of audio", out, fed)
	}
	if e, ok := r.liveEntry(); !ok || e.Text == "" {
		t.Fatalf("live entry %+v %v", e, ok)
	}
}

// A reconnect with the same codec keeps the generation: the titles of the second connection count
// on from the audio of the first.
func TestTimelineAcrossReconnect(t *testing.T) {
	e := newRelayEnv(t)
	mp3 := testAudio(t, "tone-mp3-128.mp3")
	const mi = 8192
	cut := 40*mi + 1000 // mid-frame: the framer drops the partial frame
	e.st.Set("/live", radiotest.Mount{Kind: radiotest.Icecast, Audio: [][]byte{mp3}, CutAfter: []int{cut}, MetaInt: mi,
		Titles: []radiotest.Title{{At: 10 * mi, Text: "A - One"}, {At: 30 * mi, Text: "B - Two"}}})
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	e.listen(e.f.P18, id)
	r := e.relay(id)
	titles := waitTitles(t, r, 4)
	first := audioAt(t, "mp3", mp3, cut) // the whole frames of the first connection
	worst := checkTimeline(t, r, titles, []wantTitle{
		{"A - One", 0, "mp3", mp3, 0, 10 * mi},
		{"B - Two", 0, "mp3", mp3, 0, 30 * mi},
		{"A - One", 0, "mp3", mp3, first, 10 * mi},
		{"B - Two", 0, "mp3", mp3, first, 30 * mi},
	})
	t.Logf("largest deviation %v", worst)
}

// A paced station sends its start burst (64 KiB, about 4 s) at once and real time after it; the
// titles in and after the burst keep their audio position.
func TestTimelineAfterStartBurst(t *testing.T) {
	e := newRelayEnv(t)
	mp3 := testAudio(t, "tone-mp3-128.mp3")
	const mi = 8192
	e.st.Set("/live", radiotest.Mount{Kind: radiotest.Icecast, Audio: [][]byte{mp3}, MetaInt: mi, BytesPerSecond: 16000,
		Titles: []radiotest.Title{{At: 4 * mi, Text: "In - Burst"}, {At: 10 * mi, Text: "After - Burst"}}})
	id := e.addStation("Test FM", "CH", "MP3", 128, e.st.URL("/live"))
	sess, _ := e.listen(e.f.P18, id)
	r := e.relay(id)
	titles := waitTitles(t, r, 2)
	worst := checkTimeline(t, r, titles, []wantTitle{
		{"In - Burst", 0, "mp3", mp3, 0, 4 * mi},
		{"After - Burst", 0, "mp3", mp3, 0, 10 * mi},
	})
	t.Logf("largest deviation %v", worst)
	now, err := e.s.Now(context.Background(), sess, id, time.Time{})
	if err != nil || len(now.Entries) != 2 || now.Entries[1].At.Before(now.Entries[0].At) {
		t.Fatalf("now %+v %v", now, err)
	}
}

// HLS packed audio carries its title in the ID3 tag of a segment, with the audio. A title in the
// segment after a discontinuity arrives before the relay knows that the audio starts a new
// generation (copy mode): it is stamped at the end of the old one, at most a segment off the new
// one's start.
func TestTimelineHLSDiscontinuity(t *testing.T) {
	e := newRelayEnv(t)
	parts, err := radiotest.SplitADTS(testAudio(t, "tone-aac-64.aac"), 4)
	if err != nil {
		t.Fatal(err)
	}
	e.st.Set("/hls", radiotest.Mount{Kind: radiotest.HLSMedia})
	// The relay starts three segments before the end of the source's playlist (hlsStartSegments).
	e.st.SetHLS("/hls", 0, 4, 6, radiotest.Segment{Data: parts[0]}, radiotest.Segment{Data: append(radiotest.ID3("A", "One"), parts[1]...)},
		radiotest.Segment{Data: parts[2]}, radiotest.Segment{Data: append(radiotest.ID3("B", "Two"), parts[3]...)})
	id := e.addStation("HLS FM", "CH", "AAC", 64, e.st.URL("/hls"))
	e.listen(e.f.P18, id)
	r := e.relay(id)
	e.st.AppendHLS("/hls", radiotest.Segment{Data: append(radiotest.ID3("C", "Three"), parts[0]...), Discontinuity: true},
		radiotest.Segment{Data: parts[1]}, radiotest.Segment{Data: parts[2]})
	waitGenSegment(t, r, 1)
	titles := waitTitles(t, r, 3)
	var first time.Duration // audio of parts[1] and parts[2]
	for _, p := range parts[1:3] {
		ends, per := frameWalk(t, "aac", p)
		first += time.Duration(float64(len(ends)) * per * float64(time.Second))
	}
	for i, w := range []struct {
		text string
		at   time.Duration // after the start of generation 0
	}{{"A - One", 0}, {"B - Two", first}} {
		got := titles[i].At.Sub(genStart(t, r, 0))
		dev := time.Duration(math.Abs(float64(got - w.at)))
		t.Logf("%-8s at %v, audio position %v, deviation %v", w.text, got, w.at, dev)
		if titles[i].Text != w.text || dev > targetDuration*time.Second {
			t.Fatalf("title %d %+v at %v, want %q at %v", i, titles[i], got, w.text, w.at)
		}
	}
	c := titles[2]
	dev := time.Duration(math.Abs(float64(c.At.Sub(genStart(t, r, 1)))))
	t.Logf("C - Three at the generation change, deviation %v", dev)
	if c.Text != "C - Three" || dev > targetDuration*time.Second {
		t.Fatalf("title after the discontinuity %+v, %v from the start of generation 1", c, dev)
	}
}
