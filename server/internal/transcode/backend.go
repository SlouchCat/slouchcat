package transcode

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// Backend is the video encoder family of a transcode.
type Backend string

// Backends in the order Detect prefers them.
const (
	BackendNVENC    Backend = "nvenc"
	BackendQSV      Backend = "qsv"
	BackendVAAPI    Backend = "vaapi"
	BackendSoftware Backend = "software"
)

// vaapiDevice is the DRM render node used by the QSV and VAAPI backends.
const vaapiDevice = "/dev/dri/renderD128"

// Detect runs before the server listens, so it is bounded: detectDeadline for all of it,
// testEncodeTimeout for each hardware test encode (a missing or hanging device must not block).
// They are variables for the tests.
var (
	detectDeadline    = 10 * time.Second
	testEncodeTimeout = 5 * time.Second
)

// Capabilities describes what the ffmpeg build and the hardware support.
type Capabilities struct {
	Backend        Backend
	Encoders       []string // H.264 encoders the build offers, for example libx264, h264_nvenc
	HardwareEncode bool     // the backend encodes on the GPU (every backend but software)
	// Tonemapper is the HDR → SDR filter the backend's pipeline uses (Tonemap* constants: cuda,
	// placebo, vpp_qsv, vaapi, cpu); Detect test-encodes it on a PQ source.
	Tonemapper string
	HWTonemap  bool   // the tonemapper runs on the GPU (every tonemapper but cpu)
	Version    string // first line of `ffmpeg -version`
}

// backendEncoder is the H.264 encoder of each backend.
var backendEncoder = map[Backend]string{
	BackendNVENC: "h264_nvenc", BackendQSV: "h264_qsv", BackendVAAPI: "h264_vaapi", BackendSoftware: "libx264",
}

// gpuTonemappers lists the GPU tonemappers of each hardware backend in the order Detect tries
// them, with the filter the build must offer.
var gpuTonemappers = map[Backend][]struct{ name, filter string }{
	BackendNVENC: {{TonemapCUDA, "tonemap_cuda"}, {TonemapPlacebo, "libplacebo"}},
	BackendQSV:   {{TonemapVPP, "vpp_qsv"}},
	BackendVAAPI: {{TonemapVAAPI, "tonemap_vaapi"}},
}

// Detect probes ffmpeg once (encoder and filter lists plus a one-frame test encode per hardware
// backend) and returns the best available backend: nvenc, qsv, vaapi, then software. override is
// "auto" (or empty) or one backend name; a forced hardware backend that fails its test encode is
// logged and replaced by software. Detect fails when ffmpeg does not run or has no libx264.
//
// The tonemapper is the first GPU filter of the chosen backend that the build offers and that
// passes a one-frame test encode of its HDR chain on a PQ source (each bounded by
// testEncodeTimeout); otherwise the CPU chain.
func Detect(ctx context.Context, r Runner, override string, log *slog.Logger) (Backend, Capabilities, error) {
	return DetectTonemap(ctx, r, override, TonemapPrefAuto, log)
}

// Tonemap preferences of DetectTonemap (SLOUCHCAT_HDR_TONEMAP): auto picks the GPU tonemapper
// where it passed P-6 (NVENC: tonemap_cuda or libplacebo; VAAPI best effort) and the CPU chain
// on QSV, whose vpp_qsv tonemapping flattens colours (docs/perf/M4.md: mean dE2000 10 on skin
// against the reference, the CPU chain 0); gpu forces the GPU tonemapper of the backend when it
// passes its test encode; cpu forces the CPU chain everywhere.
const (
	TonemapPrefAuto = "auto"
	TonemapPrefGPU  = "gpu"
	TonemapPrefCPU  = "cpu"
)

// DetectTonemap is Detect with a tonemap preference (see the TonemapPref* constants).
func DetectTonemap(ctx context.Context, r Runner, override, tonemapPref string, log *slog.Logger) (Backend, Capabilities, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	tonemapPref = strings.ToLower(strings.TrimSpace(tonemapPref))
	switch tonemapPref {
	case "", TonemapPrefAuto:
		tonemapPref = TonemapPrefAuto
	case TonemapPrefGPU, TonemapPrefCPU:
	default:
		return "", Capabilities{}, fmt.Errorf("unknown tonemap preference %q (want auto, gpu or cpu)", tonemapPref)
	}
	override = strings.ToLower(strings.TrimSpace(override))
	candidates := []Backend{BackendNVENC, BackendQSV, BackendVAAPI}
	switch override {
	case "", "auto":
	case string(BackendSoftware):
		candidates = nil
	case string(BackendNVENC), string(BackendQSV), string(BackendVAAPI):
		candidates = []Backend{Backend(override)}
	default:
		return "", Capabilities{}, fmt.Errorf("unknown hardware backend %q (want auto, nvenc, qsv, vaapi or software)", override)
	}

	version, err := r.detectOutput(ctx, detectDeadline, "-version")
	if err != nil {
		return "", Capabilities{}, err
	}
	encOut, err := r.detectOutput(ctx, detectDeadline, "-hide_banner", "-encoders")
	if err != nil {
		return "", Capabilities{}, err
	}
	filterOut, err := r.detectOutput(ctx, detectDeadline, "-hide_banner", "-filters")
	if err != nil {
		return "", Capabilities{}, err
	}
	encoders := parseCodecList(encOut)
	filters := parseCodecList(filterOut)
	caps := Capabilities{Version: firstLine(version)}
	for _, b := range []Backend{BackendSoftware, BackendNVENC, BackendQSV, BackendVAAPI} {
		if slices.Contains(encoders, backendEncoder[b]) {
			caps.Encoders = append(caps.Encoders, backendEncoder[b])
		}
	}
	if !slices.Contains(encoders, "libx264") {
		return "", caps, errors.New("ffmpeg has no libx264 encoder")
	}

	caps.Backend = BackendSoftware
	for _, b := range candidates {
		if !slices.Contains(encoders, backendEncoder[b]) {
			log.Info("transcode backend not in this ffmpeg build", "backend", b)
			continue
		}
		if _, err := r.detectOutput(ctx, testEncodeTimeout, testEncodeArgs(b)...); err != nil {
			level := slog.LevelInfo
			if override != "" && override != "auto" {
				level = slog.LevelWarn
			}
			log.Log(ctx, level, "transcode backend failed its test encode", "backend", b, "err", err)
			continue
		}
		caps.Backend = b
		break
	}
	caps.HardwareEncode = caps.Backend != BackendSoftware
	caps.Tonemapper = TonemapCPU
	gpuCandidates := gpuTonemappers[caps.Backend]
	switch {
	case tonemapPref == TonemapPrefCPU:
		gpuCandidates = nil
	case tonemapPref == TonemapPrefAuto && caps.Backend == BackendQSV:
		// vpp_qsv tonemaps with a flat curve (P-6 fails on the reference NAS); the hybrid chain
		// decodes and scales on the GPU and tonemaps on the CPU (SLOUCHCAT_HDR_TONEMAP=gpu opts in).
		gpuCandidates = nil
		log.Info("qsv tonemaps on the CPU (vpp_qsv flattens colours); set SLOUCHCAT_HDR_TONEMAP=gpu to use vpp_qsv")
	}
	for _, tm := range gpuCandidates {
		if !slices.Contains(filters, tm.filter) {
			continue
		}
		if _, err := r.detectOutput(ctx, testEncodeTimeout, hdrTestArgs(tm.name)...); err != nil {
			log.Info("tonemapper failed its test encode", "backend", caps.Backend, "tonemapper", tm.name, "err", err)
			continue
		}
		caps.Tonemapper = tm.name
		break
	}
	caps.HWTonemap = caps.Tonemapper != TonemapCPU
	log.Info("transcode backend detected", "backend", caps.Backend, "tonemapper", caps.Tonemapper, "version", caps.Version)
	return caps.Backend, caps, nil
}

// hdrTestArgs tonemaps one 10-bit PQ frame with the HDR chain of a GPU tonemapper (the same
// filters videoPipeline uses) and encodes it with the backend's encoder into the null muxer.
func hdrTestArgs(tonemapper string) []string {
	pq := "format=p010le,setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc,"
	var in []string
	var vf, enc string
	switch tonemapper {
	case TonemapCUDA:
		in = []string{"-init_hw_device", "cuda=cu", "-filter_hw_device", "cu"}
		vf = pq + "hwupload,scale_cuda=w=256:h=256," + cudaTonemapFilter
		enc = backendEncoder[BackendNVENC]
	case TonemapPlacebo:
		in = []string{"-init_hw_device", "cuda=cu", "-init_hw_device", "vulkan=vk@cu", "-filter_hw_device", "vk"}
		vf = pq + "hwupload," + placeboFilter("256", "256", false) + ",hwdownload,format=yuv420p"
		enc = backendEncoder[BackendNVENC]
	case TonemapVPP:
		in = []string{"-init_hw_device", "vaapi=va:" + vaapiDevice, "-init_hw_device", "qsv=qs@va", "-filter_hw_device", "qs"}
		vf = pq + "hwupload=extra_hw_frames=16,vpp_qsv=w=256:h=256:format=nv12:tonemap=1"
		enc = backendEncoder[BackendQSV]
	default: // TonemapVAAPI
		in = []string{"-init_hw_device", "vaapi=va:" + vaapiDevice, "-filter_hw_device", "va"}
		vf = pq + "hwupload,tonemap_vaapi=format=nv12:t=bt709:m=bt709:p=bt709,scale_vaapi=w=256:h=256"
		enc = backendEncoder[BackendVAAPI]
	}
	return slices.Concat([]string{"-hide_banner", "-loglevel", "error", "-nostdin"}, in,
		[]string{"-f", "lavfi", "-i", "color=c=gray:s=256x256:r=25:d=0.2", "-vf", vf,
			"-frames:v", "1", "-c:v", enc, "-f", "null", "-"})
}

// testEncodeArgs encodes one black frame with the backend's encoder into the null muxer.
func testEncodeArgs(b Backend) []string {
	in := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	src := []string{"-f", "lavfi", "-i", "color=c=black:s=256x256:r=25:d=0.2"}
	out := []string{"-frames:v", "1", "-c:v", backendEncoder[b], "-f", "null", "-"}
	switch b {
	case BackendQSV:
		in = append(in, "-init_hw_device", "vaapi=va:"+vaapiDevice, "-init_hw_device", "qsv=qs@va", "-filter_hw_device", "qs")
		src = append(src, "-vf", "format=nv12,hwupload=extra_hw_frames=16")
	case BackendVAAPI:
		in = append(in, "-init_hw_device", "vaapi=va:"+vaapiDevice, "-filter_hw_device", "va")
		src = append(src, "-vf", "format=nv12,hwupload")
	}
	return slices.Concat(in, src, out)
}

// detectOutput runs ffmpeg with a fixed argument list for at most timeout and returns its
// standard output.
func (r Runner) detectOutput(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	if r.FFmpegPath == "" {
		return nil, errors.New("ffmpeg path is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, r.FFmpegPath, args...) //nolint:gosec // G204: configured binary, fixed argument list
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg %s: %w: %s", args[len(args)-1], err, tail(stderr.Bytes(), 512))
	}
	return out, nil
}

// parseCodecList extracts the names of `ffmpeg -encoders` or `ffmpeg -filters`: data lines have
// the form " V....D libx264  description" or " ... scale_cuda  V->V  description"; the legend
// lines ("V..... = Video") and the title and separator lines are skipped.
func parseCodecList(out []byte) []string {
	var names []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[1] != "=" {
			names = append(names, fields[1])
		}
	}
	return names
}

func firstLine(b []byte) string {
	line, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(line)
}

// tail returns the last n bytes of b as trimmed text.
func tail(b []byte, n int) string {
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return strings.TrimSpace(string(b))
}
