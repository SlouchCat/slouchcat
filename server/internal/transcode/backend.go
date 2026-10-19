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

// detectTimeout bounds every ffmpeg call of Detect.
const detectTimeout = 15 * time.Second

// Capabilities describes what the ffmpeg build and the hardware support.
type Capabilities struct {
	Backend   Backend
	Encoders  []string // H.264 encoders the build offers, for example libx264, h264_nvenc
	HWTonemap bool     // the backend tonemaps on the GPU (tonemap_cuda, vpp_qsv, tonemap_vaapi)
	Version   string   // first line of `ffmpeg -version`
}

// backendEncoder is the H.264 encoder of each backend.
var backendEncoder = map[Backend]string{
	BackendNVENC: "h264_nvenc", BackendQSV: "h264_qsv", BackendVAAPI: "h264_vaapi", BackendSoftware: "libx264",
}

// hwTonemapFilter is the GPU tonemapping filter of each hardware backend.
var hwTonemapFilter = map[Backend]string{
	BackendNVENC: "tonemap_cuda", BackendQSV: "vpp_qsv", BackendVAAPI: "tonemap_vaapi",
}

// Detect probes ffmpeg once (encoder and filter lists plus a one-frame test encode per hardware
// backend) and returns the best available backend: nvenc, qsv, vaapi, then software. override is
// "auto" (or empty) or one backend name; a forced hardware backend that fails its test encode is
// logged and replaced by software. Detect fails when ffmpeg does not run or has no libx264.
func Detect(ctx context.Context, r Runner, override string, log *slog.Logger) (Backend, Capabilities, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
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

	version, err := r.detectOutput(ctx, "-version")
	if err != nil {
		return "", Capabilities{}, err
	}
	encOut, err := r.detectOutput(ctx, "-hide_banner", "-encoders")
	if err != nil {
		return "", Capabilities{}, err
	}
	filterOut, err := r.detectOutput(ctx, "-hide_banner", "-filters")
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
		if _, err := r.detectOutput(ctx, testEncodeArgs(b)...); err != nil {
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
	if f, ok := hwTonemapFilter[caps.Backend]; ok {
		caps.HWTonemap = slices.Contains(filters, f)
	}
	log.Info("transcode backend detected", "backend", caps.Backend, "hw_tonemap", caps.HWTonemap, "version", caps.Version)
	return caps.Backend, caps, nil
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

// detectOutput runs ffmpeg with a fixed argument list and returns its standard output.
func (r Runner) detectOutput(ctx context.Context, args ...string) ([]byte, error) {
	if r.FFmpegPath == "" {
		return nil, errors.New("ffmpeg path is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, detectTimeout)
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
