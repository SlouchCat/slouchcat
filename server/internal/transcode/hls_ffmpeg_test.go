package transcode

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// These tests run the real ffmpeg (skipped when it is missing) on a 20-s H.264/AAC MKV generated
// with lavfi: B-frames, irregular keyframes at 0, 2, 5, 7, 9, 13, 14, 18 s and the usual AAC
// priming (audio starts at -21 ms), the cases that break naive segmenters.

func hlsRunner(t *testing.T) Runner {
	t.Helper()
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	fp, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	return Runner{FFmpegPath: ff, FFprobePath: fp}
}

func runFFmpeg(t *testing.T, r Runner, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, r.FFmpegPath, append([]string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y"}, args...)...).CombinedOutput()
	if err != nil || len(out) > 0 {
		t.Fatalf("ffmpeg %v: %v\n%s", args, err, out)
	}
}

// testClip generates the clip once per test and returns it probed, with its keyframe index.
func testClip(t *testing.T, r Runner) Source {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clip.mkv")
	runFFmpeg(t, r, "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=24", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-t", "20", "-c:v", "libx264", "-preset", "veryfast", "-bf", "3", "-force_key_frames", "0,2,5,7,9,13,14,18",
		"-x264-params", "keyint=1000:scenecut=0", "-c:a", "aac", "-b:a", "128k", path)
	ctx := context.Background()
	probe, err := r.Probe(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	kf, err := r.Keyframes(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []time.Duration{0, sec(2), sec(5), sec(7), sec(9), sec(13), sec(14), sec(18)}; !slices.Equal(kf, want) {
		t.Fatalf("keyframes %v, want %v", kf, want)
	}
	return Source{Path: path, Probe: probe, Keyframes: kf}
}

func newFFmpegManager(t *testing.T, r Runner, b Backend) *Manager {
	t.Helper()
	m, err := NewManager(ManagerOptions{Runner: r, WorkDir: filepath.Join(t.TempDir(), "transcode"), Backend: b})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	return m
}

// reader returns a function that reads an opened init or media segment completely.
func reader(t *testing.T) func(io.ReadSeekCloser, error) []byte {
	return func(rc io.ReadSeekCloser, err error) []byte {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
}

// track is what the test needs from the init segment.
type track struct {
	timescale uint32
	video     bool
}

// fragment is the first sample of one track in a media segment.
type fragment struct {
	pts  float64 // presentation time of the first sample in seconds (tfdt + composition offset)
	sync bool    // first sample is a sync sample (keyframe)
}

func children(t *testing.T, b []byte) map[string][]byte {
	t.Helper()
	boxes, err := walkBoxes(b)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string][]byte{}
	for _, bx := range boxes {
		if _, ok := m[bx.Type]; !ok {
			m[bx.Type] = b[bx.Offset+8 : bx.Offset+bx.Size]
		}
	}
	return m
}

func childList(t *testing.T, b []byte, typ string) [][]byte {
	t.Helper()
	boxes, err := walkBoxes(b)
	if err != nil {
		t.Fatal(err)
	}
	var res [][]byte
	for _, bx := range boxes {
		if bx.Type == typ {
			res = append(res, b[bx.Offset+8:bx.Offset+bx.Size])
		}
	}
	return res
}

func parseInit(t *testing.T, init []byte) map[uint32]track {
	t.Helper()
	top := children(t, init)
	if _, ok := top["ftyp"]; !ok || top["moov"] == nil || top["moof"] != nil || top["mdat"] != nil {
		t.Fatalf("init is not ftyp+moov only")
	}
	tracks := map[uint32]track{}
	for _, trak := range childList(t, top["moov"], "trak") {
		c := children(t, trak)
		tkhd := c["tkhd"]
		var id uint32
		if tkhd[0] == 1 {
			id = binary.BigEndian.Uint32(tkhd[20:])
		} else {
			id = binary.BigEndian.Uint32(tkhd[12:])
		}
		mdia := children(t, c["mdia"])
		mdhd := mdia["mdhd"]
		var ts uint32
		if mdhd[0] == 1 {
			ts = binary.BigEndian.Uint32(mdhd[20:])
		} else {
			ts = binary.BigEndian.Uint32(mdhd[12:])
		}
		tracks[id] = track{timescale: ts, video: string(mdia["hdlr"][8:12]) == "vide"}
	}
	return tracks
}

func parseFragment(t *testing.T, seg []byte, tracks map[uint32]track) map[bool]fragment {
	t.Helper()
	top, err := walkBoxes(seg)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) == 0 || top[0].Type != "moof" {
		t.Fatalf("segment does not start with moof: %+v", top)
	}
	res := map[bool]fragment{}
	for _, traf := range childList(t, seg[top[0].Offset+8:top[0].Offset+top[0].Size], "traf") {
		c := children(t, traf)
		tfhd, tfdt, trun := c["tfhd"], c["tfdt"], c["trun"]
		id := binary.BigEndian.Uint32(tfhd[4:])
		tfhdFlags := binary.BigEndian.Uint32(tfhd[0:]) & 0xffffff
		off := 8
		var defaultFlags uint32
		for _, f := range []struct {
			bit  uint32
			size int
		}{{0x1, 8}, {0x2, 4}, {0x8, 4}, {0x10, 4}} {
			if tfhdFlags&f.bit != 0 {
				off += f.size
			}
		}
		if tfhdFlags&0x20 != 0 {
			defaultFlags = binary.BigEndian.Uint32(tfhd[off:])
		}
		var base uint64
		if tfdt[0] == 1 {
			base = binary.BigEndian.Uint64(tfdt[4:])
		} else {
			base = uint64(binary.BigEndian.Uint32(tfdt[4:]))
		}
		trunFlags := binary.BigEndian.Uint32(trun[0:]) & 0xffffff
		p := 8
		if trunFlags&0x1 != 0 {
			p += 4
		}
		flags := defaultFlags
		if trunFlags&0x4 != 0 {
			flags = binary.BigEndian.Uint32(trun[p:])
			p += 4
		}
		if trunFlags&0x100 != 0 {
			p += 4
		}
		if trunFlags&0x200 != 0 {
			p += 4
		}
		if trunFlags&0x400 != 0 {
			if trunFlags&0x4 == 0 {
				flags = binary.BigEndian.Uint32(trun[p:])
			}
			p += 4
		}
		var cto int64
		if trunFlags&0x800 != 0 {
			cto = int64(int32(binary.BigEndian.Uint32(trun[p:])))
		}
		tr := tracks[id]
		res[tr.video] = fragment{pts: float64(int64(base)+cto) / float64(tr.timescale), sync: flags&0x10000 == 0}
	}
	return res
}

// checkSegment asserts that segment i starts with moof and a keyframe whose presentation time is
// the planned start on the shifted timeline (tolerance: one 1/1000 MKV tick plus rounding).
func checkSegment(t *testing.T, p Plan, tracks map[uint32]track, i int, seg []byte) {
	t.Helper()
	f := parseFragment(t, seg, tracks)
	v, ok := f[true]
	if !ok {
		t.Fatalf("segment %d has no video fragment", i)
	}
	want := (p.Segments[i].Start + timelineOffset).Seconds()
	if d := v.pts - want; d < -0.002 || d > 0.002 || !v.sync {
		t.Errorf("segment %d: first video sample at %.4f s (sync %v), want %.4f s and a keyframe", i, v.pts, v.sync, want)
	}
	if a, ok := f[false]; !ok || a.pts < want-0.3 || a.pts > want+0.05 {
		t.Errorf("segment %d: audio fragment %+v, want it to start near %.4f s", i, a, want)
	}
}

// decodes asserts that ffmpeg decodes init followed by the segments without errors and returns
// the number of decoded video frames. framemd5 instead of the null muxer: the null muxer
// complains about the millisecond timestamps of B-frame MKV sources even for the source itself.
func decodes(t *testing.T, r Runner, init []byte, segs ...[]byte) int {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cat.mp4")
	if err := os.WriteFile(path, slices.Concat(append([][]byte{init}, segs...)...), 0o600); err != nil {
		t.Fatal(err)
	}
	md5 := filepath.Join(dir, "frames.md5")
	runFFmpeg(t, r, "-xerror", "-i", path, "-map", "0:v", "-f", "framemd5", md5)
	data, err := os.ReadFile(md5)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Count(data, []byte("\n0,"))
}

// stopAndForget stops ffmpeg and deletes the segments from index from on, so the next requests
// must restart ffmpeg however fast the first run was.
func stopAndForget(t *testing.T, s *Session, from int) {
	t.Helper()
	s.mu.Lock()
	var r *run
	if s.run != nil {
		r = s.run
		s.killLocked(r, false)
	}
	s.mu.Unlock()
	if r != nil {
		<-r.exited
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := from; k < len(s.done); k++ {
		s.done[k] = false
		if err := os.Remove(s.segmentPath(k)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

func runCount(s *Session) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runs
}

func TestHLSRemuxEndToEnd(t *testing.T) {
	r := hlsRunner(t)
	src := testClip(t, r)
	p, err := NewPlan(src, Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := starts(p), []time.Duration{0, sec(5), sec(9), sec(13), sec(18)}; !slices.Equal(got, want) {
		t.Fatalf("plan %v, want %v", got, want)
	}
	m := newFFmpegManager(t, r, BackendSoftware)
	read := reader(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := m.Start(ctx, "remux", src, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	init := read(s.OpenInit(ctx))
	tracks := parseInit(t, init)
	segs := make([][]byte, len(p.Segments))
	for i := range segs {
		segs[i] = read(s.OpenSegment(ctx, i))
		checkSegment(t, p, tracks, i, segs[i])
	}
	// Seeking back is served from disk.
	for _, i := range []int{3, 4} {
		if again := read(s.OpenSegment(ctx, i)); !slices.Equal(again, segs[i]) {
			t.Errorf("segment %d changed on the second read", i)
		}
	}
	if n := decodes(t, r, init, segs...); n != 480 {
		t.Errorf("init + all segments decode to %d frames, want 480 (20 s at 24 fps)", n)
	}
	decodes(t, r, init, segs[3])

	// A second session: stop ffmpeg after segment 0, seek to the last segment (restart with -ss),
	// then back to 3 (another restart that stops when it reaches the existing segment 4).
	s2, err := m.Start(ctx, "remux2", src, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	read(s2.OpenSegment(ctx, 0))
	stopAndForget(t, s2, 1)
	last := read(s2.OpenSegment(ctx, 4))
	checkSegment(t, p, tracks, 4, last)
	seg3 := read(s2.OpenSegment(ctx, 3))
	checkSegment(t, p, tracks, 3, seg3)
	if runs := runCount(s2); runs != 3 {
		t.Errorf("%d ffmpeg runs, want 3 (start, seek to 4, seek to 3)", runs)
	}
	decodes(t, r, init, last)
	decodes(t, r, init, seg3, last)
	if want := "avc1.64001E,mp4a.40.2"; CodecsString(p, src) != want {
		t.Errorf("codecs %q, want %q", CodecsString(p, src), want)
	}
	m.Stop("remux2")
	if s2.CPUTime() <= 0 {
		t.Error("no CPU time recorded")
	}
}

func TestHLSTranscodeEndToEnd(t *testing.T) {
	r := hlsRunner(t)
	src := testClip(t, r)
	p, err := NewPlan(src, Target{Mode: ModeTranscode, MaxHeight: 720})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := starts(p), []time.Duration{0, sec(4), sec(8), sec(12), sec(16)}; !slices.Equal(got, want) {
		t.Fatalf("plan %v, want %v", got, want)
	}
	m := newFFmpegManager(t, r, BackendSoftware)
	read := reader(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := m.Start(ctx, "transcode", src, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	init := read(s.OpenInit(ctx))
	tracks := parseInit(t, init)
	seg0 := read(s.OpenSegment(ctx, 0))
	checkSegment(t, p, tracks, 0, seg0)
	stopAndForget(t, s, 1)
	seg4 := read(s.OpenSegment(ctx, 4)) // restart at 16 s
	checkSegment(t, p, tracks, 4, seg4)
	seg2 := read(s.OpenSegment(ctx, 2)) // restart at 8 s
	checkSegment(t, p, tracks, 2, seg2)
	seg3 := read(s.OpenSegment(ctx, 3)) // same run, or waits for it
	checkSegment(t, p, tracks, 3, seg3)
	seg1 := read(s.OpenSegment(ctx, 1))
	checkSegment(t, p, tracks, 1, seg1)
	if runs := runCount(s); runs != 4 {
		t.Errorf("%d ffmpeg runs, want 4 (start, seek to 4, seek to 2, seek to 1)", runs)
	}
	if n := decodes(t, r, init, seg0, seg1, seg2, seg3, seg4); n != 480 {
		t.Errorf("init + segments of four runs decode to %d frames, want 480", n)
	}
	decodes(t, r, init, seg4)
}

// A 10-s GOP (x264 -g 240 at 24 fps): with a grid plan the segment muxer cut once per keyframe and
// the numbering drifted from the playlist; NewPlan refuses remux without the index, and with the
// index ffmpeg's cuts match the plan, from the start, from a resume segment and after a seek.
func TestHLSRemuxLongGOP(t *testing.T) {
	r := hlsRunner(t)
	path := filepath.Join(t.TempDir(), "longgop.mkv")
	runFFmpeg(t, r, "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-t", "35", "-c:v", "libx264", "-preset", "ultrafast", "-bf", "3", "-g", "240",
		"-x264-params", "keyint=240:min-keyint=240:scenecut=0", "-c:a", "aac", "-b:a", "96k", path)
	ctx := context.Background()
	probe, err := r.Probe(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	target := Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true}
	if _, err := NewPlan(Source{Path: path, Probe: probe}, target); !errors.Is(err, ErrNoKeyframes) {
		t.Fatalf("remux without index: %v, want ErrNoKeyframes", err)
	}
	kf, err := r.Keyframes(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []time.Duration{0, sec(10), sec(20), sec(30)}; !slices.Equal(kf, want) {
		t.Fatalf("keyframes %v, want %v", kf, want)
	}
	src := Source{Path: path, Probe: probe, Keyframes: kf}
	p, err := NewPlan(src, target)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := starts(p), []time.Duration{0, sec(10), sec(20), sec(30)}; !slices.Equal(got, want) {
		t.Fatalf("plan %v, want %v", got, want)
	}
	m := newFFmpegManager(t, r, BackendSoftware)
	read := reader(t)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	s, err := m.Start(ctx, "longgop", src, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	init := read(s.OpenInit(ctx))
	tracks := parseInit(t, init)
	segs := make([][]byte, len(p.Segments))
	for i := range segs {
		segs[i] = read(s.OpenSegment(ctx, i))
		checkSegment(t, p, tracks, i, segs[i])
	}
	if n := decodes(t, r, init, segs...); n != 840 {
		t.Errorf("init + all segments decode to %d frames, want 840 (35 s at 24 fps)", n)
	}

	// Resume at segment 2, then seek back to 1 (a restart that stops at the existing segment 2).
	s2, err := m.Start(ctx, "longgop2", src, p, 2)
	if err != nil {
		t.Fatal(err)
	}
	init2 := read(s2.OpenInit(ctx))
	seg2 := read(s2.OpenSegment(ctx, 2))
	checkSegment(t, p, tracks, 2, seg2)
	seg1 := read(s2.OpenSegment(ctx, 1))
	checkSegment(t, p, tracks, 1, seg1)
	decodes(t, r, init2, seg1, seg2)
	if runs := runCount(s2); runs != 2 {
		t.Errorf("%d ffmpeg runs, want 2 (resume at 2, seek to 1)", runs)
	}
}
