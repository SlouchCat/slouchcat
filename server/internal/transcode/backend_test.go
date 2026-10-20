package transcode

import (
	"context"
	"fmt"
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
	if b != BackendQSV || caps.Backend != BackendQSV || caps.HWTonemap || caps.Tonemapper != TonemapCPU || !caps.HardwareEncode {
		t.Errorf("auto: backend %s, caps %+v; want qsv with the cpu tonemapper (nvenc fails its test encode, no vpp_qsv filter)", b, caps)
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
	if b, caps, err := Detect(ctx, r, "software", nil); err != nil || b != BackendSoftware || caps.HardwareEncode || caps.Tonemapper != TonemapCPU {
		t.Errorf("software: %s, %+v, %v", b, caps, err)
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

// The tonemapper is the first GPU filter of the backend that the build has and that passes its
// HDR test encode: tonemap_cuda, then libplacebo, else the CPU.
func TestDetectTonemapper(t *testing.T) {
	const script = `case "$*" in
"-version") echo "ffmpeg version 6.1.1";;
"-hide_banner -encoders") printf ' ------\n V....D libx264  x264\n V....D h264_nvenc  NVENC\n V....D h264_qsv  QSV\n';;
"-hide_banner -filters") printf ' ... tonemap_cuda  V->V  t\n ... libplacebo  N->V  p\n ... vpp_qsv  V->V  q\n';;
%s
*) exit 1;;
esac
`
	cases := []struct {
		name, rules, override string
		want                  string
	}{
		{"cuda", `*tonemap_cuda*h264_nvenc*) exit 0;; *libplacebo*) exit 1;; *h264_nvenc*) exit 0;;`, "nvenc", TonemapCUDA},
		{"placebo when tonemap_cuda fails", `*tonemap_cuda*) exit 1;; *vulkan=vk@cu*libplacebo*h264_nvenc*) exit 0;; *h264_nvenc*) exit 0;;`, "nvenc", TonemapPlacebo},
		{"cpu when both fail", `*tonemap_cuda*) exit 1;; *libplacebo*) exit 1;; *h264_nvenc*) exit 0;;`, "nvenc", TonemapCPU},
		{"vpp_qsv", `*vpp_qsv*tonemap=1*h264_qsv*) exit 0;; *h264_qsv*) exit 0;;`, "qsv", TonemapVPP},
		{"qsv without hdr", `*tonemap=1*) exit 1;; *h264_qsv*) exit 0;;`, "qsv", TonemapCPU},
	}
	for _, c := range cases {
		bin := fakeBinary(t, fmt.Sprintf(script, c.rules))
		_, caps, err := Detect(context.Background(), Runner{FFmpegPath: bin}, c.override, nil)
		if err != nil || caps.Tonemapper != c.want || caps.HWTonemap != (c.want != TonemapCPU) || string(caps.Backend) != c.override {
			t.Errorf("%s: %+v, %v; want tonemapper %s", c.name, caps, err, c.want)
		}
	}
}

func TestHDRTestArgs(t *testing.T) {
	for tm, want := range map[string]string{
		TonemapCUDA:    "-init_hw_device cuda=cu -filter_hw_device cu -f lavfi -i color=c=gray:s=256x256:r=25:d=0.2 -vf format=p010le,setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc,hwupload,scale_cuda=w=256:h=256," + cudaTonemapFilter + " -frames:v 1 -c:v h264_nvenc",
		TonemapPlacebo: "-init_hw_device cuda=cu -init_hw_device vulkan=vk@cu -filter_hw_device vk -f lavfi -i color=c=gray:s=256x256:r=25:d=0.2 -vf format=p010le,setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc,hwupload,libplacebo=w=256:h=256:tonemapping=bt.2390:colorspace=bt709:color_primaries=bt709:color_trc=bt709:range=tv:format=yuv420p,hwdownload,format=yuv420p -frames:v 1 -c:v h264_nvenc",
		TonemapVPP:     "-init_hw_device vaapi=va:/dev/dri/renderD128 -init_hw_device qsv=qs@va -filter_hw_device qs -f lavfi -i color=c=gray:s=256x256:r=25:d=0.2 -vf format=p010le,setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc,hwupload=extra_hw_frames=16,vpp_qsv=w=256:h=256:format=nv12:tonemap=1 -frames:v 1 -c:v h264_qsv",
		TonemapVAAPI:   "-init_hw_device vaapi=va:/dev/dri/renderD128 -filter_hw_device va -f lavfi -i color=c=gray:s=256x256:r=25:d=0.2 -vf format=p010le,setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc,hwupload,tonemap_vaapi=format=nv12:t=bt709:m=bt709:p=bt709,scale_vaapi=w=256:h=256 -frames:v 1 -c:v h264_vaapi",
	} {
		got := strings.Join(hdrTestArgs(tm), " ")
		if want = "-hide_banner -loglevel error -nostdin " + want + " -f null -"; got != want {
			t.Errorf("%s:\ngot:  %s\nwant: %s", tm, got, want)
		}
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
	if caps.Tonemapper == "" || caps.HardwareEncode != (b != BackendSoftware) {
		t.Errorf("tonemapper %q, hardware encode %v", caps.Tonemapper, caps.HardwareEncode)
	}
	t.Logf("backend %s, encoders %v, tonemapper %s, %s", b, caps.Encoders, caps.Tonemapper, caps.Version)
}
