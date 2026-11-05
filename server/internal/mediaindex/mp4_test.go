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

// The MP4 fixtures are built here box by box: 25 frames per second in the media timescale
// 12800 (512 ticks a frame, as ffmpeg writes it), keyframes at samples 1, 6 and 11 (1-based).

func box(typ string, body ...[]byte) []byte {
	b := bytes.Join(body, nil)
	return slices.Concat(binary.BigEndian.AppendUint32(nil, uint32(8+len(b))), []byte(typ), b)
}

func u32(v ...uint32) []byte {
	var b []byte
	for _, x := range v {
		b = binary.BigEndian.AppendUint32(b, x)
	}
	return b
}

func u64(v ...uint64) []byte {
	var b []byte
	for _, x := range v {
		b = binary.BigEndian.AppendUint64(b, x)
	}
	return b
}

// full prefixes a full box body with version and zero flags.
func full(version byte, fields ...[]byte) []byte {
	return slices.Concat([]byte{version, 0, 0, 0}, bytes.Join(fields, nil))
}

func mvhd(scale uint32) []byte { return box("mvhd", full(0, u32(0, 0, scale, 0), make([]byte, 80))) }

func mdhd1(scale uint32) []byte { return box("mdhd", full(1, u64(0, 0), u32(scale), u64(0), u32(0))) }

func hdlr(handler string) []byte {
	return box("hdlr", full(0, u32(0), []byte(handler), u32(0, 0, 0), []byte{0}))
}

// elst entries are (segment_duration, media_time) pairs, version 0.
func elst(entries ...[2]int64) []byte {
	b := u32(uint32(len(entries)))
	for _, e := range entries {
		b = append(b, u32(uint32(e[0]), uint32(int32(e[1])), 1<<16)...)
	}
	return box("edts", box("elst", full(0, b)))
}

// table builds stts or ctts from (count, value) pairs.
func table(typ string, version byte, entries ...[2]int64) []byte {
	b := u32(uint32(len(entries)))
	for _, e := range entries {
		b = append(b, u32(uint32(e[0]), uint32(int32(e[1])))...)
	}
	return box(typ, full(version, b))
}

func stss(samples ...uint32) []byte {
	return box("stss", full(0, u32(uint32(len(samples))), u32(samples...)))
}

// trakBox builds a track; edts (from elst) and the sample tables are passed as finished boxes.
func trakBox(id uint32, handler string, edts []byte, stbl ...[]byte) []byte {
	return box("trak",
		box("tkhd", full(0, u32(0, 0, id, 0, 0), make([]byte, 60))),
		edts,
		box("mdia", mdhd1(12800), hdlr(handler), box("minf", box("stbl", stbl...))),
	)
}

// mp4File puts ftyp, mdat and moov in order; faststart puts moov before mdat.
func mp4File(faststart bool, moov []byte) []byte {
	ftyp := box("ftyp", []byte("isom"), u32(512), []byte("isomavc1"))
	mdat := box("mdat", make([]byte, 3000))
	if faststart {
		return slices.Concat(ftyp, moov, mdat)
	}
	return slices.Concat(ftyp, mdat, moov)
}

var (
	frames = table("stts", 0, [2]int64{15, 512})
	// B-frames: every keyframe is shown two frames late, the edit list takes that back.
	bframes = table("ctts", 0, [2]int64{1, 1024}, [2]int64{4, 512}, [2]int64{1, 1024}, [2]int64{4, 512}, [2]int64{1, 1024}, [2]int64{4, 512})
	keys    = stss(1, 6, 11)
	audio   = trakBox(1, "soun", nil, table("stts", 0, [2]int64{100, 1024}))
)

// mp4Fixtures are the cases of concept 14, step 2 (a), and the seeds of FuzzReadMP4.
func mp4Fixtures() map[string][]byte {
	bvideo := trakBox(1, "vide", elst([2]int64{600, 1024}), frames, bframes, keys)
	return map[string][]byte{
		"bframes-moov-last": mp4File(false, box("moov", mvhd(1000), bvideo)),
		"bframes-faststart": mp4File(true, box("moov", mvhd(1000), bvideo)),
		"audio-first":       mp4File(false, box("moov", mvhd(1000), audio, trakBox(2, "vide", nil, frames, keys))),
		"empty-edit":        mp4File(false, box("moov", mvhd(1000), trakBox(1, "vide", elst([2]int64{5, -1}, [2]int64{600, 0}), frames, keys))),
		"ctts-v1-negative":  mp4File(false, box("moov", mvhd(1000), trakBox(1, "vide", nil, frames, table("ctts", 1, [2]int64{5, 0}, [2]int64{1, -512}, [2]int64{9, 0}), keys))),
		"all-sync":          mp4File(true, box("moov", mvhd(1000), trakBox(1, "vide", nil, table("stts", 0, [2]int64{3, 512})))),
		"two-edits":         mp4File(false, box("moov", mvhd(1000), trakBox(1, "vide", elst([2]int64{200, 0}, [2]int64{200, 2560}), frames, keys))),
		"fragmented":        mp4File(true, box("moov", mvhd(1000), trakBox(1, "vide", nil, frames, keys), box("mvex", box("trex", full(0, u32(1, 1, 0, 0, 0)))))),
		"moof-before-moov":  slices.Concat(box("ftyp", []byte("iso6")), box("moof", box("mfhd", full(0, u32(1)))), box("moov", mvhd(1000), trakBox(1, "vide", nil, frames, keys))),
		"no-video":          mp4File(false, box("moov", mvhd(1000), audio)),
		"no-moov":           slices.Concat(box("ftyp", []byte("isom")), box("mdat", make([]byte, 100))),
		"large-mdat-moov-to-eof": slices.Concat(box("ftyp", []byte("isom")),
			u32(1), []byte("mdat"), u64(16+100), make([]byte, 100),
			u32(0), []byte("moov"), mvhd(1000)[0:], trakBox(1, "vide", nil, frames, keys)),
	}
}

func TestReadMP4(t *testing.T) {
	fx := mp4Fixtures()
	for _, tc := range []struct {
		name  string
		want  []time.Duration
		track uint64
	}{
		{"bframes-moov-last", ms(0, 200, 400), 1},
		{"bframes-faststart", ms(0, 200, 400), 1},
		{"audio-first", ms(0, 200, 400), 2},
		{"empty-edit", ms(5, 205, 405), 1},
		{"ctts-v1-negative", ms(0, 160, 400), 1},
		{"all-sync", ms(0, 40, 80), 1},
		{"large-mdat-moov-to-eof", ms(0, 200, 400), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := fx[tc.name]
			ix, err := Read(bytes.NewReader(b), int64(len(b)), "mp4")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ix.Keyframes, tc.want) || ix.Track != tc.track || ix.Source != SourceMP4 {
				t.Fatalf("got %v track %d source %q, want %v track %d", ix.Keyframes, ix.Track, ix.Source, tc.want, tc.track)
			}
		})
	}
}

func TestReadMP4NoIndex(t *testing.T) {
	fx := mp4Fixtures()
	b := fx["bframes-moov-last"]
	cases := map[string][]byte{
		"two-edits": fx["two-edits"], "fragmented": fx["fragmented"], "moof-before-moov": fx["moof-before-moov"],
		"no-video": fx["no-video"], "no-moov": fx["no-moov"], "truncated": b[:len(b)-20],
	}
	for _, name := range slices.Sorted(maps.Keys(cases)) {
		t.Run(name, func(t *testing.T) {
			b := cases[name]
			if _, err := Read(bytes.NewReader(b), int64(len(b)), "mov"); !errors.Is(err, ErrNoIndex) {
				t.Fatalf("err %v, want ErrNoIndex", err)
			}
		})
	}
}

// A moov that announces more than 64 MiB is refused before it is read.
func TestReadMP4MoovTooLarge(t *testing.T) {
	const announced = 65 << 20
	head := slices.Concat(box("ftyp", []byte("isom")), u32(announced), []byte("moov"))
	size := int64(len(head)) - 8 + announced
	ix, err := Read(sparse{head, size}, size, "m4v")
	if !errors.Is(err, ErrNoIndex) {
		t.Fatalf("err %v, want ErrNoIndex", err)
	}
	if ix.Stats.BytesRead > 64 {
		t.Fatalf("read %d bytes, want only the box headers", ix.Stats.BytesRead)
	}
}

func FuzzReadMP4(f *testing.F) {
	fx := mp4Fixtures()
	for _, name := range slices.Sorted(maps.Keys(fx)) {
		f.Add(fx[name])
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		fuzzRead(t, b, "mp4", SourceMP4)
	})
}
