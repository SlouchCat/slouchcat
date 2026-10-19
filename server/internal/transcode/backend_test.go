package transcode

import (
	"context"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

const fakeDetect = `case "$*" in
"-version") echo "ffmpeg version 7.1.2-Jellyfin Copyright (c) 2000-2025"; echo "built with gcc";;
"-hide_banner -encoders") printf 'Encoders:\n V..... = Video\n A..... = Audio\n ------\n V....D libx264              libx264 H.264\n V....D h264_nvenc           NVIDIA NVENC\n V....D h264_qsv             H.264 QSV\n A....D aac                  AAC\n';;
"-hide_banner -filters") printf 'Filters:\n  T.. = Timeline support\n  | = Source or sink filter\n ... tonemap_cuda      V->V       tonemap\n ... scale_cuda        V->V       scale\n';;
*h264_nvenc*) echo "no CUDA device" >&2; exit 1;;
*h264_qsv*) exit 0;;
*) exit 1;;
esac
`

func TestDetectFake(t *testing.T) {
	bin := fakeBinary(t, fakeDetect)
	r := Runner{FFmpegPath: bin}
	ctx := context.Background()

	b, caps, err := Detect(ctx, r, "auto", nil)
	if err != nil {
		t.Fatal(err)
	}
	if b != BackendQSV || caps.Backend != BackendQSV || caps.HWTonemap {
		t.Errorf("auto: backend %s, caps %+v; want qsv without hw tonemap (nvenc fails its test encode)", b, caps)
	}
	if !slices.Equal(caps.Encoders, []string{"libx264", "h264_nvenc", "h264_qsv"}) {
		t.Errorf("encoders %v", caps.Encoders)
	}
	if caps.Version != "ffmpeg version 7.1.2-Jellyfin Copyright (c) 2000-2025" {
		t.Errorf("version %q", caps.Version)
	}

	if b, _, err := Detect(ctx, r, "NVENC", nil); err != nil || b != BackendSoftware {
		t.Errorf("forced nvenc that fails: %s, %v; want software", b, err)
	}
	if b, _, err := Detect(ctx, r, "vaapi", nil); err != nil || b != BackendSoftware {
		t.Errorf("forced vaapi without encoder: %s, %v; want software", b, err)
	}
	if b, _, err := Detect(ctx, r, "software", nil); err != nil || b != BackendSoftware {
		t.Errorf("software: %s, %v", b, err)
	}
	if _, _, err := Detect(ctx, r, "cuda", nil); err == nil {
		t.Error("want error for an unknown override")
	}
	if _, _, err := Detect(ctx, Runner{}, "auto", nil); err == nil {
		t.Error("want error without ffmpeg path")
	}
	noX264 := fakeBinary(t, `case "$*" in "-version") echo v;; *) printf ' ------\n V....D h264_nvenc NVENC\n';; esac`)
	if _, _, err := Detect(ctx, Runner{FFmpegPath: noX264}, "auto", nil); err == nil {
		t.Error("want error without libx264")
	}
}

func TestDetectBoundsHangingTestEncode(t *testing.T) {
	old := testEncodeTimeout
	testEncodeTimeout = 200 * time.Millisecond
	t.Cleanup(func() { testEncodeTimeout = old })
	// nvenc and qsv hang (a stuck device); Detect gives each its timeout and falls back.
	bin := fakeBinary(t, strings.Replace(strings.Replace(fakeDetect, `*h264_nvenc*) echo "no CUDA device" >&2; exit 1;;`,
		`*h264_nvenc*) exec sleep 30;;`, 1), `*h264_qsv*) exit 0;;`, `*h264_qsv*) exec sleep 30;;`, 1))
	start := time.Now()
	b, _, err := Detect(context.Background(), Runner{FFmpegPath: bin}, "auto", nil)
	if err != nil || b != BackendSoftware {
		t.Fatalf("hanging hardware: %s, %v; want software", b, err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("Detect took %v with two hanging test encodes of 200 ms", d)
	}
}

func TestParseCodecList(t *testing.T) {
	out := []byte("Filters:\n  T.. = Timeline support\n  A = Audio input/output\n  | = Source or sink filter\n" +
		" ... abench            A->A       Benchmark\n .SC zscale            V->V       Apply resizing\n")
	if got := parseCodecList(out); !slices.Equal(got, []string{"abench", "zscale"}) {
		t.Errorf("parseCodecList = %v", got)
	}
}

// TestDetectReal logs what this machine offers; it needs the real ffmpeg.
func TestDetectReal(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	b, caps, err := Detect(context.Background(), Runner{FFmpegPath: bin}, "auto", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(caps.Encoders, "libx264") || b == "" {
		t.Errorf("backend %s, caps %+v", b, caps)
	}
	t.Logf("backend %s, encoders %v, hw tonemap %v, %s", b, caps.Encoders, caps.HWTonemap, caps.Version)
}
