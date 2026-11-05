package transcode

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRateControlNeverCQP pins the rate control of the HLS encodes: on every backend and for every
// ladder rung and some small profile limits the server never emits -global_quality together with
// -maxrate and no -b:v (ffmpeg turns that into CQP), and QSV and VAAPI always get a -b:v below
// -maxrate (equal values make ffmpeg pick CBR).
func TestRateControlNeverCQP(t *testing.T) {
	src := Source{Path: "/media/Film.mkv", Probe: Probe{Format: Format{Duration: 20 * time.Second}, Streams: []Stream{
		{Index: 0, Type: "video", Codec: "h264", Width: 3840, Height: 2160, FrameRate: 24},
		{Index: 1, Type: "audio", Codec: "aac", Channels: 2},
	}}}
	backends := []struct {
		name string
		caps Capabilities
		vbr  bool // capped VBR with -b:v below -maxrate
	}{
		{"software", Capabilities{Backend: BackendSoftware, Tonemapper: TonemapCPU}, false},
		{"nvenc", Capabilities{Backend: BackendNVENC, Tonemapper: TonemapCPU}, false},
		{"qsv", Capabilities{Backend: BackendQSV, Tonemapper: TonemapVPP, HWTonemap: true}, true},
		{"vaapi", Capabilities{Backend: BackendVAAPI, Tonemapper: TonemapCPU}, true},
	}
	// rate is the profile limit in bit/s (0 = the rung's own rate); wantB is the expected -b:v of
	// QSV and VAAPI, written out here independently of vbrTarget.
	cases := []struct {
		quality string
		height  int
		rate    int
		wantB   int
	}{
		{"2160p", 2160, 0, 18_750_000},
		{"1080p", 1080, 0, 6_000_000},
		{"720p", 720, 0, 3_000_000},
		{"480p", 480, 0, 1_500_000},
		{"360p", 360, 0, 999_000},
		{"720p", 720, 1_500_000, 1_125_000},
		{"720p", 720, 1_340_000, 1_005_000},
		{"720p", 720, 1_200_000, 1_000_000},
		{"480p", 480, 900_000, 899_000},
		{"360p", 360, 500_000, 499_000},
		{"360p", 360, 3, 2},
		{"360p", 360, 2, 1}, // the smallest cap with a target below it
	}
	for _, b := range backends {
		for _, c := range cases {
			t.Run(fmt.Sprintf("%s %s %d", b.name, c.quality, c.rate), func(t *testing.T) {
				tg := Target{Mode: ModeTranscode, SubtitleBurnIn: -1, AudioStream: -1, Quality: c.quality,
					MaxHeight: c.height, MaxBitrate: c.rate, Backend: b.caps.Backend, HWTonemap: b.caps.HWTonemap}
				p, err := NewPlan(src, tg)
				if err != nil {
					t.Fatal(err)
				}
				args, err := ffmpegArgs(b.caps, src, p, 0, "/w/s/run0001")
				if err != nil {
					t.Fatal(err)
				}
				line := strings.Join(args, " ")
				val := func(flag string) (n int, ok bool) {
					for i := 0; i+1 < len(args); i++ {
						if args[i] == flag {
							n, err := strconv.Atoi(args[i+1])
							if err != nil {
								t.Fatalf("%s %q is not a number", flag, args[i+1])
							}
							return n, true
						}
					}
					return 0, false
				}
				maxrate, hasMax := val("-maxrate")
				bv, hasB := val("-b:v")
				_, hasGQ := val("-global_quality")
				if !hasMax {
					t.Fatalf("no -maxrate: %s", line)
				}
				if hasGQ && !hasB {
					t.Errorf("-global_quality with -maxrate and no -b:v (CQP): %s", line)
				}
				if !b.vbr {
					return
				}
				if hasGQ || !hasB || bv >= maxrate || bv != c.wantB {
					t.Errorf("-b:v %d (present %v), -global_quality %v, -maxrate %d; want -b:v %d below -maxrate: %s",
						bv, hasB, hasGQ, maxrate, c.wantB, line)
				}
			})
		}
	}
}
