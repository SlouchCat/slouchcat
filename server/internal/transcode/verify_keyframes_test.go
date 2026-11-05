package transcode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/mediaindex"
)

// Golden argument list of the sample of concept 14, 6.2: one ffprobe call, the head and one
// packet at each pick, the pick a millisecond late for the backward seek.
func TestVerifyKeyframesArgsGolden(t *testing.T) {
	t.Parallel()
	got := strings.Join(verifyKeyframesArgs("/media/Film (2024)/Film.mkv", []time.Duration{2*time.Second + 2*time.Millisecond, 75 * time.Second}), " ")
	want := "-v error -select_streams v:0 -show_entries packet=pts_time,flags -read_intervals %+#30,2.003%+#1,75.001%+#1 -of csv=p=0 /media/Film (2024)/Film.mkv"
	if got != want {
		t.Errorf("args\n got %s\nwant %s", got, want)
	}
	if got := intervalSeconds(3*time.Hour + 4*time.Millisecond + 600*time.Microsecond); got != "10800.004" {
		t.Errorf("intervalSeconds = %s", got)
	}
}

func TestVerifyPicks(t *testing.T) {
	t.Parallel()
	at := []time.Duration{0, 2 * time.Second, 7 * time.Second, 8 * time.Second, 15 * time.Second, 21 * time.Second}
	a, b := verifyPicks(at, 42), verifyPicks(at, 42)
	if !slices.Equal(a, b) || len(a) != 2 || a[0] >= a[1] || a[0] == 0 {
		t.Fatalf("picks %v, %v: want two sorted entries behind entry 0, equal for one seed", a, b)
	}
	seen := map[time.Duration]bool{}
	for seed := range uint64(50) {
		for _, p := range verifyPicks(at, seed) {
			seen[p] = true
		}
	}
	if len(seen) != len(at)-1 || seen[0] {
		t.Errorf("picks over 50 seeds %v, want every entry behind entry 0", seen)
	}
	if got := verifyPicks(at[:2], 7); !slices.Equal(got, at[1:2]) {
		t.Errorf("picks of two entries = %v", got)
	}
	if got := verifyPicks(at[:1], 7); len(got) != 0 {
		t.Errorf("picks of one entry = %v", got)
	}
}

func TestCheckVerifyPackets(t *testing.T) {
	t.Parallel()
	picks := []time.Duration{2 * time.Second, 8 * time.Second}
	for _, tc := range []struct {
		name, out string
		ok        bool
	}{
		{"match", "0.000000,K__\n0.041667,___\n0.083333,___\n2.000000,K__\n8.001000,K__\n", true},
		{"no time on a b-frame", "N/A,___\n0.000000,K__\n2.000000,K__\n8.000000,K__\n", true},
		{"entry 0 is not the first keyframe", "0.500000,K__\n2.000000,K__\n8.000000,K__\n", false},
		{"no keyframe in the head", "0.000000,___\n2.000000,K__\n8.000000,K__\n", false},
		{"pick lands on an earlier keyframe", "0.000000,K__\n2.000000,K__\n7.000000,K__\n", false},
		{"pick is no keyframe", "0.000000,K__\n2.000000,K__\n8.000000,___\n", false},
		{"pick 3 ms off", "0.000000,K__\n2.000000,K__\n8.003000,K__\n", false},
		{"too few packets", "0.000000,K__\n2.000000,K__\n", false},
	} {
		err := checkVerifyPackets([]byte(tc.out), 0, picks)
		if (err == nil) != tc.ok || (err != nil && !errors.Is(err, ErrKeyframeMismatch)) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// VerifyKeyframes against files ffmpeg wrote: the index the container carries passes for every
// seed, an index with an entry that is not a keyframe or a wrong entry 0 fails.
func TestVerifyKeyframesFFmpeg(t *testing.T) {
	r := hlsRunner(t)
	video := []string{"-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=30"}
	audio := []string{"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=30"}
	x264 := []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "35", "-pix_fmt", "yuv420p",
		"-x264-params", "keyint=infinite:scenecut=0", "-force_key_frames", "0,2,7,8,15,21,27", "-c:a", "aac", "-b:a", "64k"}
	for _, tc := range []struct {
		name, file, container string
		extra                 []string
	}{
		{"mkv", "clip.mkv", "mkv", nil},
		{"mp4 b-frames", "clip.mp4", "mp4", []string{"-bf", "3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.file)
			args := slices.Concat(video, audio, x264, tc.extra, []string{path})
			runFFmpeg(t, r, args...)
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
			// The index holds the forced keyframes, shifted by the start of the first one (the edit
			// list of the MP4 with B-frames), and entry for entry what the full read finds.
			forced := []time.Duration{0, 2, 7, 8, 15, 21, 27}
			if len(ix.Keyframes) != len(forced) {
				t.Fatalf("index %v, want the keyframes %v s", ix.Keyframes, forced)
			}
			ctx := context.Background()
			full, err := r.Keyframes(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			if len(full) != len(forced) {
				t.Fatalf("full read %v, want the keyframes %v s", full, forced)
			}
			offset := ix.Keyframes[0]
			if offset < 0 || offset > 500*time.Millisecond {
				t.Errorf("first keyframe at %v, want at most 0.5 s", offset)
			}
			for i, s := range forced {
				if d := ix.Keyframes[i] - offset - s*time.Second; d < -2*time.Millisecond || d > 2*time.Millisecond {
					t.Errorf("keyframe %d at %v, want %d s after the first", i, ix.Keyframes[i], s)
				}
				if d := ix.Keyframes[i] - full[i]; d < -2*time.Millisecond || d > 2*time.Millisecond {
					t.Errorf("keyframe %d at %v, the full read finds it at %v", i, ix.Keyframes[i], full[i])
				}
			}
			started := time.Now()
			for seed := range uint64(10) {
				if err := r.VerifyKeyframes(ctx, path, ix.Keyframes, seed); err != nil {
					t.Fatalf("seed %d: %v (index %v)", seed, err, ix.Keyframes)
				}
			}
			t.Logf("10 samples in %v", time.Since(started))
			k := ix.Keyframes
			notKey := []time.Duration{k[0], k[2] + 500*time.Millisecond, k[4]}
			if err := r.VerifyKeyframes(ctx, path, notKey, 1); !errors.Is(err, ErrKeyframeMismatch) {
				t.Errorf("entry between keyframes: %v, want a mismatch", err)
			}
			wrongFirst := []time.Duration{k[1], k[2], k[4]}
			if err := r.VerifyKeyframes(ctx, path, wrongFirst, 1); !errors.Is(err, ErrKeyframeMismatch) {
				t.Errorf("entry 0 behind the first keyframe: %v, want a mismatch", err)
			}
		})
	}
}
