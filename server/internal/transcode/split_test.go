package transcode

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// Unit tests of the split-layout reader (split.go) on byte sequences built here, without ffmpeg:
// the layouts of +delay_moov+frag_keyframe, partial boxes while ffmpeg writes, 64-bit box sizes,
// tfdt in version 0 and 1, and the end of a run with and without an error.

// mp4Box builds a box with a 32-bit size, or with the 64-bit form when large.
func mp4Box(typ string, large bool, payload ...[]byte) []byte {
	body := slices.Concat(payload...)
	if large {
		b := make([]byte, 16, 16+len(body))
		binary.BigEndian.PutUint32(b, 1)
		copy(b[4:], typ)
		binary.BigEndian.PutUint64(b[8:], uint64(16+len(body)))
		return append(b, body...)
	}
	b := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(b, uint32(8+len(body)))
	copy(b[4:], typ)
	return append(b, body...)
}

func u32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }

// testMoov is a moov with a video track 1 and an audio track 2, both at timescale 1000.
func testMoov(large bool) []byte {
	trak := func(id uint32, handler string) []byte {
		tkhd := slices.Concat(u32(0), u32(0), u32(0), u32(id), make([]byte, 8)) // version 0: id at 12
		mdhd := slices.Concat(u32(0), u32(0), u32(0), u32(1000), make([]byte, 8))
		hdlr := slices.Concat(u32(0), u32(0), []byte(handler), make([]byte, 12))
		return mp4Box("trak", false, mp4Box("tkhd", false, tkhd),
			mp4Box("mdia", false, mp4Box("mdhd", false, mdhd), mp4Box("hdlr", false, hdlr)))
	}
	return mp4Box("moov", large, trak(1, "vide"), trak(2, "soun"))
}

// testFragment is a moof with a video and an audio traf whose first sample decodes at ms on the
// shifted timeline (tfdt version 1 when v1), followed by an mdat of n bytes (64-bit size when
// large).
func testFragment(ms int, v1, large bool, n int) []byte {
	traf := func(id uint32, t int) []byte {
		tfdt := slices.Concat(u32(0), u32(uint32(t)))
		if v1 {
			tfdt = slices.Concat(u32(1<<24), binary.BigEndian.AppendUint64(nil, uint64(t)))
		}
		return mp4Box("traf", false, mp4Box("tfhd", false, u32(0), u32(id)), mp4Box("tfdt", false, tfdt),
			mp4Box("trun", false, u32(0), u32(1)))
	}
	t := ms + int(timelineOffset/time.Millisecond)
	return slices.Concat(mp4Box("moof", false, traf(2, t-30), traf(1, t)), mp4Box("mdat", large, bytes.Repeat([]byte{byte(ms)}, n)))
}

var testFtyp = mp4Box("ftyp", false, []byte("isom"), u32(0x200))

// splitPlan is a 12-s plan with segments at 0, 4 and 8 s.
var splitPlan = Plan{Segments: []Segment{{Start: 0}, {Start: 4 * time.Second}, {Start: 8 * time.Second}}}

// splitRun writes files into a run directory and reads it with a splitState.
type splitRun struct {
	t   *testing.T
	dir string
	st  *splitState
}

func newSplitRun(t *testing.T) *splitRun {
	return &splitRun{t: t, dir: t.TempDir(), st: newSplitState(0)}
}

func (r *splitRun) write(file int, data ...[]byte) {
	r.t.Helper()
	path := filepath.Join(r.dir, fmt.Sprintf(segmentPattern, file))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		r.t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(slices.Concat(data...)); err != nil {
		r.t.Fatal(err)
	}
}

// events reads until the reader waits and returns what it published: "init" and "seg<k>:<bytes of
// the segment>" in order.
func (r *splitRun) events(final, exitOK bool) []string {
	r.t.Helper()
	var out []string
	for {
		ev, err := r.st.next(r.dir, splitPlan, remuxCutDelta, final, exitOK)
		if errors.Is(err, errSplitWait) {
			return out
		}
		if err != nil {
			r.t.Fatal(err)
		}
		if ev.init != nil {
			out = append(out, fmt.Sprintf("init:%d", len(ev.init)))
		}
		if ev.segment >= 0 {
			dst := filepath.Join(r.t.TempDir(), "seg")
			if err := writeParts(r.dir, dst, ev.parts); err != nil {
				r.t.Fatal(err)
			}
			data, err := os.ReadFile(dst)
			if err != nil {
				r.t.Fatal(err)
			}
			out = append(out, fmt.Sprintf("seg%d:%d", ev.segment, len(data)))
		}
	}
}

func expectEvents(t *testing.T, step string, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s: events %v, want %v", step, got, want)
	}
}

// The layout of a run whose first segment has one keyframe: file 0 is ftyp+moov alone, file 1
// holds the held-back fragment of segment 0 and the fragment of segment 1.
func TestSplitHeldBackFragment(t *testing.T) {
	r := newSplitRun(t)
	initLen := len(testFtyp) + len(testMoov(false))
	f0, f4, f8 := testFragment(0, false, false, 100), testFragment(4000, false, false, 200), testFragment(8000, false, false, 300)
	r.write(0, testFtyp, testMoov(false))
	expectEvents(t, "file 0", r.events(false, false), fmt.Sprintf("init:%d", initLen))
	r.write(1, f0)
	expectEvents(t, "held-back fragment", r.events(false, false), fmt.Sprintf("seg0:%d", len(f0)))
	r.write(1, f4)
	expectEvents(t, "segment 1 still open", r.events(false, false))
	r.write(2, f8)
	expectEvents(t, "file 1 closed", r.events(false, false), fmt.Sprintf("seg1:%d", len(f4)))
	expectEvents(t, "ffmpeg done", r.events(true, true), fmt.Sprintf("seg2:%d", len(f8)))
	expectEvents(t, "nothing more", r.events(true, true))
}

// A remux segment with several keyframes has one fragment per keyframe, in one file; 64-bit box
// sizes and tfdt version 1 read the same.
func TestSplitSeveralFragments(t *testing.T) {
	for _, tc := range []struct {
		name      string
		v1, large bool
	}{{"32-bit, tfdt v0", false, false}, {"64-bit, tfdt v1", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			r := newSplitRun(t)
			a, b := testFragment(0, tc.v1, tc.large, 50), testFragment(2000, tc.v1, tc.large, 60)
			c, d := testFragment(4000, tc.v1, tc.large, 70), testFragment(6500, tc.v1, tc.large, 80)
			r.write(0, testFtyp, testMoov(tc.large), a, b)
			r.write(1, c, d)
			r.write(2, testFragment(8000, tc.v1, tc.large, 90))
			expectEvents(t, "run", r.events(false, false), fmt.Sprintf("init:%d", len(testFtyp)+len(testMoov(tc.large))),
				fmt.Sprintf("seg0:%d", len(a)+len(b)), fmt.Sprintf("seg1:%d", len(c)+len(d)))
		})
	}
}

// While ffmpeg writes, a box is on disk in pieces: the reader waits at every cut and never
// publishes a part; once the file has grown it goes on.
func TestSplitGrowingFile(t *testing.T) {
	r := newSplitRun(t)
	head := slices.Concat(testFtyp, testMoov(false))
	f0, f2 := testFragment(0, false, false, 400), testFragment(2000, false, false, 400)
	stream := slices.Concat(head, f0, f2)
	var got []string
	for cut := 0; cut < len(stream); cut += 37 {
		end := min(cut+37, len(stream))
		r.write(0, stream[cut:end])
		got = append(got, r.events(false, false)...)
	}
	expectEvents(t, "piece by piece", got, fmt.Sprintf("init:%d", len(head)))
	r.write(1)
	expectEvents(t, "file 0 closed", r.events(false, false), fmt.Sprintf("seg0:%d", len(f0)+len(f2)))
}

// A size field of 0 (not patched yet) is a box still being written, not an error.
func TestSplitUnpatchedSize(t *testing.T) {
	r := newSplitRun(t)
	r.write(0, testFtyp, testMoov(false))
	frag := testFragment(0, false, false, 10)
	unpatched := slices.Clone(frag)
	binary.BigEndian.PutUint32(unpatched, 0)
	r.write(0, unpatched[:20])
	expectEvents(t, "unpatched moof", r.events(false, false), fmt.Sprintf("init:%d", len(testFtyp)+len(testMoov(false))))
}

// ffmpeg ended with an error: the open segment of its last file is not published; without an
// error it is.
func TestSplitExitStatus(t *testing.T) {
	for _, ok := range []bool{false, true} {
		r := newSplitRun(t)
		f0 := testFragment(0, false, false, 10)
		r.write(0, testFtyp, testMoov(false), f0)
		got := r.events(true, ok)
		want := []string{fmt.Sprintf("init:%d", len(testFtyp)+len(testMoov(false)))}
		if ok {
			want = append(want, fmt.Sprintf("seg0:%d", len(f0)))
		}
		expectEvents(t, fmt.Sprintf("exit ok %v", ok), got, want...)
	}
}

// Broken layouts are errors: a moof before the moov, a closed file that ends in a partial box.
func TestSplitBrokenLayout(t *testing.T) {
	r := newSplitRun(t)
	r.write(0, testFtyp, testFragment(0, false, false, 10))
	if _, err := r.st.next(r.dir, splitPlan, remuxCutDelta, false, false); err == nil || errors.Is(err, errSplitWait) {
		t.Errorf("moof before moov: %v, want an error", err)
	}
	r = newSplitRun(t)
	frag := testFragment(0, false, false, 10)
	r.write(0, testFtyp, testMoov(false), frag[:len(frag)-3])
	r.write(1)
	var err error
	for err == nil {
		_, err = r.st.next(r.dir, splitPlan, remuxCutDelta, false, false)
	}
	if errors.Is(err, errSplitWait) {
		t.Error("closed file with a partial fragment: the reader waits, want an error")
	}
}
