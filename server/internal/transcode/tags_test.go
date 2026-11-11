package transcode

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// tagRunner returns a Runner with ffmpeg and ffprobe from PATH; the tag tests need the real
// binaries and fail instead of skipping.
func tagRunner(t *testing.T) Runner {
	t.Helper()
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatalf("ffmpeg is required for the tag tests: %v", err)
	}
	return Runner{FFmpegPath: ff, FFprobePath: realRunner(t).FFprobePath}
}

// Concept 19 P-19 (6.5, O-7): an mp3 and an m4a with a cover and a tag of the provider get title,
// album, artist and date without re-encoding; the cover, the audio codec and the provider's other
// tags stay. The copy goes to the writer the caller gives, the temporary directory goes. An ogg is
// not tagged and nothing is written.
func TestTagAudio(t *testing.T) {
	t.Parallel()
	r := tagRunner(t)
	ctx := context.Background()
	dir := t.TempDir()
	tags := AudioTags{Title: "Folge 12: Der Anfang", Album: "Testpodcast", Artist: "Redaktion", Date: "2026-11-09T18:00:00Z"}
	for _, tc := range []struct{ ext, codec string }{{"mp3", "libmp3lame"}, {"m4a", "aac"}} {
		in := filepath.Join(dir, "in."+tc.ext)
		if _, err := r.runFFmpeg(ctx, nil, "-nostdin", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=d=2",
			"-f", "lavfi", "-i", "color=red:s=64x64:d=1", "-map", "0", "-map", "1", "-c:a", tc.codec, "-c:v", "mjpeg", "-frames:v", "1",
			"-disposition:v", "attached_pic", "-metadata", "title=Anbietertitel", "-metadata", "genre=Talk", in); err != nil {
			t.Fatal(err)
		}
		before, err := r.Probe(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, ".slouchcat-x."+tc.ext+".tagged")
		f, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		tmp := t.TempDir()
		if err := errors.Join(r.TagAudio(ctx, in, f, tc.ext, tags, tmp), f.Close()); err != nil {
			t.Fatalf("%s: %v", tc.ext, err)
		}
		if left, err := os.ReadDir(tmp); err != nil || len(left) != 0 {
			t.Errorf("%s: temporary files left: %v %v", tc.ext, left, err)
		}
		p, err := r.Probe(ctx, out)
		if err != nil {
			t.Fatalf("%s: probe tagged file: %v", tc.ext, err)
		}
		for k, want := range map[string]string{"title": tags.Title, "album": tags.Album, "artist": tags.Artist, "date": tags.Date, "genre": "Talk"} {
			if got := p.Format.Tags[k]; got != want {
				t.Errorf("%s: tag %s = %q, want %q (tags %v)", tc.ext, k, got, want, p.Format.Tags)
			}
		}
		if len(p.Streams) != len(before.Streams) || p.Streams[0].Codec != before.Streams[0].Codec || !p.Streams[1].AttachedPic {
			t.Errorf("%s: streams %+v, want those of the source %+v", tc.ext, p.Streams, before.Streams)
		}
		if d := p.Format.Duration - before.Format.Duration; d > 50e6 || d < -50e6 {
			t.Errorf("%s: duration %v, source %v", tc.ext, p.Format.Duration, before.Format.Duration)
		}
	}
	in := filepath.Join(dir, "in.ogg")
	if _, err := r.runFFmpeg(ctx, nil, "-nostdin", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=d=1", "-c:a", "libvorbis", in); err != nil {
		t.Fatal(err)
	}
	orig, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := r.TagAudio(ctx, in, &out, "ogg", tags, t.TempDir()); !errors.Is(err, ErrNotTaggable) {
		t.Errorf("ogg: %v, want ErrNotTaggable", err)
	}
	if out.Len() != 0 {
		t.Errorf("ogg: %d bytes written", out.Len())
	}
	if now, err := os.ReadFile(in); err != nil || !bytes.Equal(now, orig) {
		t.Errorf("ogg changed: %v", err)
	}
}
