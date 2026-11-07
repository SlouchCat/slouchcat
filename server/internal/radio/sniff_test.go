package radio

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/radio/radiotest"
)

func TestSniffFrames(t *testing.T) {
	flac := append([]byte("fLaC\x00\x00\x00\x22"), make([]byte, 34)...)
	flac[8+10], flac[8+11], flac[8+12] = 0x0A, 0xC4, 0x42 // 44100 Hz, 2 channels
	for _, tc := range []struct {
		name          string
		data          []byte
		kind          sniffKind
		format, codec string
		rate, ch      int
	}{
		{"ID3+MP3", testAudio(t, "tone-mp3-128.mp3"), kindAudio, "mp3", "mp3", 44100, 2},
		{"ADTS", testAudio(t, "tone-aac-64.aac"), kindAudio, "aac", "aac", 44100, 2},
		{"OggS/Vorbis", testAudio(t, "tone-ogg.ogg"), kindAudio, "ogg", "vorbis", 44100, 2},
		{"OggS/Opus", testAudio(t, "tone-opus.ogg"), kindAudio, "ogg", "opus", 48000, 2},
		{"fLaC", flac, kindAudio, "flac", "flac", 44100, 2},
		{"TS", testAudio(t, "hls/seg0.ts"), kindAudio, "mpegts", "aac", 0, 0},
		{"HLS", []byte("#EXTM3U\n#EXT-X-TARGETDURATION:4\nseg/1\n"), kindHLS, "", "", 0, 0},
		{"m3u", []byte("#EXTM3U\nhttp://example.org/s\n"), kindPlaylist, "", "", 0, 0},
		{"pls", []byte("[Playlist]\nFile1=http://example.org/s\n"), kindPlaylist, "", "", 0, 0},
	} {
		in, kind, err := sniff(tc.data)
		if err != nil || kind != tc.kind || in.Format != tc.format || in.Codec != tc.codec ||
			in.SampleRate != tc.rate || in.Channels != tc.ch {
			t.Errorf("%s: %+v kind %d err %v", tc.name, in, kind, err)
		}
	}
	garbage := bytes.Repeat([]byte{0xFF, 0xFB, 0x12, 0x34, 0x00, 0x00}, 4000) // MP3-like syncs with no frame behind
	for name, data := range map[string][]byte{"garbage": garbage, "html": []byte("<!doctype html><title>x</title>")} {
		if _, _, err := sniff(data); !errors.Is(err, model.ErrRadioNotAudio) {
			t.Errorf("%s: %v, want ErrRadioNotAudio", name, err)
		}
	}
}

// checkFrames fails unless b is a run of whole frames of codec from its first to its last byte.
func checkFrames(t *testing.T, codec string, b []byte) int {
	t.Helper()
	n := 0
	for off := 0; off < len(b); n++ {
		h, ok := parseFrameHeader(codec, b[off:])
		if !ok || off+h.length > len(b) {
			t.Fatalf("no whole %s frame at %d of %d", codec, off, len(b))
		}
		off += h.length
	}
	return n
}

// framesOf strips the ID3 tag of a test file and returns its whole frames.
func framesOf(t *testing.T, codec string, file []byte) []byte {
	t.Helper()
	if n := id3Length(file); n > 0 {
		file = file[n:]
	}
	checkFrames(t, codec, file)
	return file
}

func TestFramerWholeFramesOnly(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for codec, name := range map[string]string{"mp3": "tone-mp3-128.mp3", "aac": "tone-aac-64.aac"} {
		file := testAudio(t, name)
		f := newFramer(codec)
		var out []byte
		for b := file; len(b) > 0; {
			n := min(len(b), 1+rng.IntN(3000))
			got := f.push(b[:n])
			if len(got) > 0 {
				checkFrames(t, codec, got)
			}
			out = append(out, got...)
			b = b[n:]
		}
		frames := framesOf(t, codec, file)
		// The last frame is emitted once the next header could be checked; at the end of the
		// file at most one frame is still held back.
		if !bytes.HasPrefix(frames, out) || len(frames)-len(out) > 2000 {
			t.Errorf("%s: %d of %d frame bytes, not a prefix", codec, len(out), len(frames))
		}
		if h, ok := f.params(); !ok || h.sampleRate != 44100 || h.channels != 2 {
			t.Errorf("%s: params %+v", codec, h)
		}
	}
}

func TestFramerDropsPartialAndID3OnReconnect(t *testing.T) {
	for codec, name := range map[string]string{"mp3": "tone-mp3-128.mp3", "aac": "tone-aac-64.aac"} {
		file := testAudio(t, name)
		frames := framesOf(t, codec, file)
		f := newFramer(codec)
		out := f.push(file[:len(file)/3+7]) // the connection breaks in the middle of a frame
		f.reset()
		tag := radiotest.ID3("Artist", "Title \xff\xfb\x90\x00 with a sync")
		out = append(out, f.push(append(tag, frames...))...)
		checkFrames(t, codec, out)
		if bytes.Contains(out, []byte("ID3")) {
			t.Errorf("%s: ID3 tag reached the output", codec)
		}
	}
}

func TestFramerResyncNeedsTwoHeaders(t *testing.T) {
	file := testAudio(t, "tone-mp3-128.mp3")
	frames := framesOf(t, "mp3", file)
	// A valid-looking MP3 header with nothing valid at its frame length, then the real frames.
	fake := []byte{0xFF, 0xFB, 0x90, 0x64}
	garbage := append(append([]byte("junk"), fake...), bytes.Repeat([]byte{0x11}, 600)...)
	f := newFramer("mp3")
	out := f.push(append(garbage, frames...))
	if !bytes.HasPrefix(frames, out) || len(out) < len(frames)-2000 {
		t.Fatalf("output does not start at the real frames (%d of %d bytes)", len(out), len(frames))
	}
}
