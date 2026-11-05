package transcode

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/mediaindex"
)

// The cross test of concept 14, step 2 (b): package mediaindex must read from the container the
// keyframes the full read (Runner.Keyframes) finds, within 1 ms. mediaindex itself may not start
// ffmpeg (depguard, CLAUDE.md rule 3), so the files are generated here, like the other ffmpeg
// tests of this package (hlsRunner skips without ffmpeg).

// crossKeyframes are irregular keyframes as in «H264 MKV Long» (testdata/generate-media.sh): 90 s,
// the last keyframe at 75 s.
const crossKeyframes = "0,2,7,8,15,21,30,31,45,60,61,75"

func TestMediaindexCross(t *testing.T) {
	r := hlsRunner(t)
	video := []string{"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=90"}
	audio := []string{"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=90"}
	x264 := []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "35", "-pix_fmt", "yuv420p",
		"-x264-params", "keyint=infinite:scenecut=0", "-force_key_frames", crossKeyframes}
	aac := []string{"-c:a", "aac", "-b:a", "64k"}
	cat := func(parts ...[]string) []string {
		var out []string
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	for _, tc := range []struct {
		name, file, container string
		args                  []string
		track                 uint64
		late                  []string // remux with the video 0.5 s late and these extra arguments
	}{
		// ffmpeg's Matroska muxer: cues for the video track only, one SeekHead.
		{"mkv", "clip.mkv", "mkv", cat(video, audio, x264, aac), 1, nil},
		// Audio first: the cues must be those of track 2.
		{"mkv audio first", "clip.mkv", "mkv", cat(video, audio, []string{"-map", "1:a", "-map", "0:v"}, x264, aac), 2, nil},
		// B-frames: ctts and an edit list that takes the composition delay back; moov behind mdat.
		{"mp4 b-frames", "clip.mp4", "mp4", cat(video, audio, x264, []string{"-bf", "3"}, aac), 1, nil},
		{"mp4 b-frames faststart", "clip.mp4", "mp4", cat(video, audio, x264, []string{"-bf", "3", "-movflags", "+faststart"}, aac), 1, nil},
		// ctts version 1 with negative offsets instead of the edit-list shift.
		{"mp4 negative cts", "clip.mp4", "mp4", cat(video, audio, x264, []string{"-bf", "3", "-movflags", "+negative_cts_offsets"}, aac), 1, nil},
		// The video starts 0.5 s late: an empty edit in front of the shifting one.
		{"mov empty edit", "clip.mov", "mov", cat(video, audio, x264, []string{"-bf", "3"}, aac), 1, []string{}},
		{"mp4 empty edit negative cts", "clip.mp4", "mp4", cat(video, audio, x264, []string{"-bf", "3", "-movflags", "+negative_cts_offsets"}, aac), 1,
			[]string{"-movflags", "+negative_cts_offsets"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.file)
			if tc.late == nil {
				runFFmpeg(t, r, append(tc.args, path)...)
			} else {
				src := filepath.Join(t.TempDir(), "src.mp4")
				runFFmpeg(t, r, append(tc.args, src)...)
				runFFmpeg(t, r, cat([]string{"-itsoffset", "0.5", "-i", src, "-i", src, "-map", "0:v", "-map", "1:a", "-c", "copy"}, tc.late, []string{path})...)
			}
			ctx := context.Background()
			want, err := r.Keyframes(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			probe, err := r.Probe(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			st, err := f.Stat()
			if err != nil {
				t.Fatal(err)
			}
			ix, err := mediaindex.Read(f, st.Size(), tc.container)
			if err != nil {
				t.Fatal(err)
			}
			if len(ix.Keyframes) != len(want) {
				t.Fatalf("mediaindex %v, full read %v", ix.Keyframes, want)
			}
			for i := range want {
				if d := ix.Keyframes[i] - want[i]; d < -time.Millisecond || d > time.Millisecond {
					t.Fatalf("keyframe %d: mediaindex %v, full read %v", i, ix.Keyframes[i], want[i])
				}
			}
			if ix.Track != tc.track {
				t.Fatalf("track %d, want %d", ix.Track, tc.track)
			}
			if first := ix.Keyframes[0]; tc.late != nil && first != 500*time.Millisecond {
				t.Fatalf("first keyframe %v, want 500ms behind the empty edit", first)
			}
			if err := mediaindex.Check(ix, probe.Format.Duration); err != nil {
				t.Fatalf("Check: %v (duration %v, keyframes %v)", err, probe.Format.Duration, ix.Keyframes)
			}
			t.Logf("%d keyframes, last %v, duration %v, %+v", len(ix.Keyframes), ix.Keyframes[len(ix.Keyframes)-1], probe.Format.Duration, ix.Stats)
		})
	}
}
