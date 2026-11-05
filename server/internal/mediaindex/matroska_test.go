package mediaindex

import (
	"bytes"
	"encoding/binary"
	"errors"
	"maps"
	"slices"
	"testing"
	"time"
)

// The Matroska fixtures are built here byte by byte (no real media in the repository, concept 14
// step 2): every element with an 8-byte size and every unsigned integer with 8 bytes, so the
// layout and the seek positions do not depend on the values.

func ebmlID(id uint32) []byte {
	b := binary.BigEndian.AppendUint32(nil, id)
	for len(b) > 1 && b[0] == 0 {
		b = b[1:]
	}
	return b
}

func el(id uint32, data ...[]byte) []byte {
	body := bytes.Join(data, nil)
	out := append(ebmlID(id), 0x01)
	out = append(out, binary.BigEndian.AppendUint64(nil, uint64(len(body)))[1:]...)
	return append(out, body...)
}

func uintEl(id uint32, v uint64) []byte { return el(id, binary.BigEndian.AppendUint64(nil, v)) }

// unknownSize is an element header with all size bits set.
func unknownSize(id uint32, data ...[]byte) []byte {
	out := append(ebmlID(id), 0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
	return append(out, bytes.Join(data, nil)...)
}

func seek(id uint32, pos int64) []byte {
	return el(idSeek, el(idSeekID, ebmlID(id)), uintEl(idSeekPosition, uint64(pos)))
}

func info(scale uint64) []byte { return el(idInfo, uintEl(idTimestampScale, scale)) }

// tracks lists track entries as (number, type) pairs.
func tracks(entries ...[2]uint64) []byte {
	var es [][]byte
	for _, e := range entries {
		es = append(es, el(idTrackEntry, uintEl(idTrackNumber, e[0]), uintEl(idTrackType, e[1])))
	}
	return el(idTracks, es...)
}

func cuePoint(t uint64, tracks ...uint64) []byte {
	parts := [][]byte{uintEl(idCueTime, t)}
	for _, tr := range tracks {
		parts = append(parts, el(idCueTrackPositions, uintEl(idCueTrack, tr), uintEl(0xF1, 0)))
	}
	return el(idCuePoint, parts...)
}

func cluster(pad int) []byte { return el(idCluster, uintEl(0xE7, 0), el(0xA3, make([]byte, pad))) }

// child is a top-level element of a segment; it may refer to the positions of the others.
type child struct {
	name  string
	build func(pos map[string]int64) []byte
}

func fixed(name string, b []byte) child {
	return child{name, func(map[string]int64) []byte { return b }}
}

// mkvFile lays out the EBML header and a segment of known size with the children. The first
// pass measures them, the second fills in the positions (relative to the segment data).
func mkvFile(children ...child) []byte {
	pos := map[string]int64{}
	var parts [][]byte
	for range 2 {
		parts = parts[:0]
		var at int64
		for _, c := range children {
			b := c.build(pos)
			pos[c.name] = at
			at += int64(len(b))
			parts = append(parts, b)
		}
	}
	return append(el(idEBML, el(0x4282, []byte("matroska"))), el(idSegment, parts...)...)
}

// longCues are the keyframes of «H264 MKV Long» (testdata/generate-media.sh), in ms.
var longCues = []uint64{0, 2000, 7000, 8000, 15000, 21000, 30000, 31000, 45000, 60000, 61000, 75000}

func ms(v ...uint64) []time.Duration {
	out := make([]time.Duration, len(v))
	for i, x := range v {
		out[i] = time.Duration(x) * time.Millisecond
	}
	return out
}

// videoAudioCues has a cue point for audio before every video cue point and lists the audio
// track first in the shared cue points, as mkvmerge does.
func videoAudioCues() []byte {
	var points [][]byte
	for _, t := range longCues {
		points = append(points, cuePoint(t+500, 2), cuePoint(t, 2, 1))
	}
	return el(idCues, points...)
}

// matroskaFixtures are the cases of concept 14, step 2 (a), and the seeds of FuzzReadMatroska.
func matroskaFixtures() map[string][]byte {
	av := tracks([2]uint64{1, 1}, [2]uint64{2, 2})
	return map[string][]byte{
		// mkvmerge: the first SeekHead names Info, Tracks and a second SeekHead behind the
		// clusters (outside the first 64 KiB), which names the Cues.
		"second-seekhead": mkvFile(
			child{"sh1", func(p map[string]int64) []byte {
				return el(idSeekHead, seek(idInfo, p["info"]), seek(idTracks, p["tracks"]), seek(idSeekHead, p["sh2"]))
			}},
			fixed("info", info(1_000_000)), fixed("tracks", av), fixed("cluster", cluster(80<<10)),
			fixed("cues", videoAudioCues()),
			child{"sh2", func(p map[string]int64) []byte { return el(idSeekHead, seek(idCues, p["cues"])) }},
		),
		// ffmpeg: one SeekHead with everything, the Cues at the end.
		"one-seekhead": mkvFile(
			child{"sh", func(p map[string]int64) []byte {
				return el(idSeekHead, seek(idInfo, p["info"]), seek(idTracks, p["tracks"]), seek(idCues, p["cues"]))
			}},
			fixed("void", el(0xEC, make([]byte, 40))),
			fixed("info", info(1_000_000)), fixed("tracks", av), fixed("cluster", cluster(1000)),
			fixed("cues", videoAudioCues()),
		),
		// No SeekHead: Info and Tracks by the walk, the Cues by the search of the tail.
		"no-seekhead": mkvFile(
			fixed("info", info(1_000_000)), fixed("tracks", av), fixed("cluster", cluster(1000)),
			fixed("cues", videoAudioCues()),
		),
		// The SeekHead names only the Cues; Info and Tracks come from the walk.
		"seekhead-cues-only": mkvFile(
			child{"sh", func(p map[string]int64) []byte { return el(idSeekHead, seek(idCues, p["cues"])) }},
			fixed("info", info(1_000_000)), fixed("tracks", av), fixed("cluster", cluster(1000)),
			fixed("cues", videoAudioCues()),
		),
		// Audio is track 1, video track 2; TimestampScale 100 µs.
		"video-second-track": mkvFile(
			fixed("info", info(100_000)), fixed("tracks", tracks([2]uint64{1, 2}, [2]uint64{2, 1})),
			fixed("cluster", cluster(100)),
			fixed("cues", el(idCues, cuePoint(0, 1, 2), cuePoint(5, 1), cuePoint(15, 2), cuePoint(20_000, 2))),
		),
		"cues-without-video": mkvFile(
			fixed("info", info(1_000_000)), fixed("tracks", av), fixed("cluster", cluster(100)),
			fixed("cues", el(idCues, cuePoint(0, 2), cuePoint(1000, 2))),
		),
		"no-cues": mkvFile(
			fixed("info", info(1_000_000)), fixed("tracks", av), fixed("cluster", cluster(100)),
		),
		"unknown-segment-size": append(el(idEBML, el(0x4282, []byte("matroska"))),
			unknownSize(idSegment, info(1_000_000), av, cluster(100), videoAudioCues())...),
	}
}

func TestReadMatroska(t *testing.T) {
	fx := matroskaFixtures()
	for _, tc := range []struct {
		name      string
		container string
		want      []time.Duration
		track     uint64
		points    int
	}{
		{"second-seekhead", "mkv", ms(longCues...), 1, 24},
		{"one-seekhead", "mkv", ms(longCues...), 1, 24},
		{"no-seekhead", "webm", ms(longCues...), 1, 24},
		{"seekhead-cues-only", "MKV", ms(longCues...), 1, 24},
		{"video-second-track", "mkv", ms(0, 2, 2000), 2, 4}, // 1.5 ms rounds to 2 ms
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := fx[tc.name]
			ix, err := Read(bytes.NewReader(b), int64(len(b)), tc.container)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ix.Keyframes, tc.want) || ix.Track != tc.track || ix.Source != SourceCues || ix.Stats.CuePoints != tc.points {
				t.Fatalf("got %v track %d source %q points %d, want %v track %d points %d",
					ix.Keyframes, ix.Track, ix.Source, ix.Stats.CuePoints, tc.want, tc.track, tc.points)
			}
			if ix.Stats.Reads == 0 || ix.Stats.BytesRead == 0 || ix.Stats.BytesRead > int64(len(b)) {
				t.Fatalf("stats %+v for %d bytes", ix.Stats, len(b))
			}
		})
	}
}

// The second SeekHead lies outside the head; reading it and the Cues costs three reads more,
// the cluster in between is never read.
func TestReadMatroskaSecondSeekHeadReads(t *testing.T) {
	b := matroskaFixtures()["second-seekhead"]
	ix, err := Read(bytes.NewReader(b), int64(len(b)), "mkv")
	if err != nil {
		t.Fatal(err)
	}
	if ix.Stats.Reads != 5 || ix.Stats.BytesRead > mkvHead+4<<10 {
		t.Fatalf("stats %+v, want 5 reads of about 64 KiB", ix.Stats)
	}
}

// «H264 MKV Long»: 90 s, last keyframe at 75 s; the end gap of 15 s must pass Check.
func TestReadMatroskaLongEndGap(t *testing.T) {
	b := matroskaFixtures()["one-seekhead"]
	ix, err := Read(bytes.NewReader(b), int64(len(b)), "mkv")
	if err != nil {
		t.Fatal(err)
	}
	if err := Check(ix, 90*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestReadMatroskaNoIndex(t *testing.T) {
	fx := matroskaFixtures()
	for _, name := range []string{"cues-without-video", "no-cues", "unknown-segment-size"} {
		t.Run(name, func(t *testing.T) {
			b := fx[name]
			if _, err := Read(bytes.NewReader(b), int64(len(b)), "mkv"); !errors.Is(err, ErrNoIndex) {
				t.Fatalf("err %v, want ErrNoIndex", err)
			}
		})
	}
	b := fx["one-seekhead"]
	for _, tc := range []struct {
		name string
		b    []byte
		ct   string
	}{
		{"other container", b, "avi"},
		{"truncated", b[:len(b)-10], "mkv"},
		{"not ebml", append([]byte{0, 0, 0, 0}, b...), "mkv"},
		{"empty", nil, "mkv"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Read(bytes.NewReader(tc.b), int64(len(tc.b)), tc.ct); !errors.Is(err, ErrNoIndex) {
				t.Fatalf("err %v, want ErrNoIndex", err)
			}
		})
	}
}

// sparse is a file of size bytes that starts with b and continues with zeros.
type sparse struct {
	b    []byte
	size int64
}

func (s sparse) ReadAt(p []byte, off int64) (int, error) {
	for i := range p {
		p[i] = 0
		if j := off + int64(i); j < int64(len(s.b)) {
			p[i] = s.b[j]
		}
	}
	return len(p), nil
}

// A Cues element that announces more than 8 MiB is refused before it is read.
func TestReadMatroskaCuesTooLarge(t *testing.T) {
	const announced = 9 << 20
	cuesHeader := append(ebmlID(idCues), 0x01)
	cuesHeader = append(cuesHeader, binary.BigEndian.AppendUint64(nil, announced)[1:]...)
	// The SeekHead has the same size whatever the positions; measure it first.
	infoAt := int64(len(el(idSeekHead, seek(idInfo, 0), seek(idTracks, 0), seek(idCues, 0))))
	tr := tracks([2]uint64{1, 1})
	tracksAt := infoAt + int64(len(info(1_000_000)))
	cuesAt := tracksAt + int64(len(tr))
	sh := el(idSeekHead, seek(idInfo, infoAt), seek(idTracks, tracksAt), seek(idCues, cuesAt))
	seg := slices.Concat(sh, info(1_000_000), tr, cuesHeader)
	segHeader := append(ebmlID(idSegment), 0x01)
	segHeader = append(segHeader, binary.BigEndian.AppendUint64(nil, uint64(len(seg))+announced)[1:]...)
	file := slices.Concat(el(idEBML), segHeader, seg)
	size := int64(len(file)) + announced
	ix, err := Read(sparse{file, size}, size, "mkv")
	if !errors.Is(err, ErrNoIndex) {
		t.Fatalf("err %v, want ErrNoIndex", err)
	}
	if ix.Stats.BytesRead > mkvHead {
		t.Fatalf("read %d bytes, want at most the head", ix.Stats.BytesRead)
	}
}

func FuzzReadMatroska(f *testing.F) {
	fx := matroskaFixtures()
	for _, name := range slices.Sorted(maps.Keys(fx)) {
		f.Add(fx[name])
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		fuzzRead(t, b, "mkv", SourceCues)
	})
}

// fuzzRead checks what Read promises for any input: ErrNoIndex or a sorted, unique index.
func fuzzRead(t *testing.T, b []byte, container string, src Source) {
	ix, err := Read(bytes.NewReader(b), int64(len(b)), container)
	if ix.Stats.BytesRead > int64(len(b))+mkvHead {
		t.Fatalf("read %d bytes of %d", ix.Stats.BytesRead, len(b))
	}
	if err != nil {
		if !errors.Is(err, ErrNoIndex) {
			t.Fatalf("err %v does not wrap ErrNoIndex", err)
		}
		return
	}
	if ix.Source != src || len(ix.Keyframes) == 0 || len(ix.Keyframes) > maxEntries {
		t.Fatalf("index %+v", ix)
	}
	for i := 1; i < len(ix.Keyframes); i++ {
		if ix.Keyframes[i] <= ix.Keyframes[i-1] {
			t.Fatalf("keyframes not sorted and unique: %v", ix.Keyframes)
		}
	}
	_ = Check(ix, time.Hour) // must not panic
}
