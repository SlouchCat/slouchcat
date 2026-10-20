package transcode

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The segmenter contract (ADR 0003): ffmpeg keeps the source timeline (-copyts) shifted by
// timelineOffset, so every run of a session writes the same absolute tfdt for the same segment
// and restarts need no discontinuity. The offset keeps the negative decode times of B-frame
// sources and the AAC priming of segment 0 positive; the fMP4 muxer must not shift them.
const (
	timelineOffset = 10 * time.Second
	// remuxSeekMargin is how far before a copied keyframe the output seek starts, so the keyframe
	// survives ffmpeg's start check, which compares its decode time (before its presentation time).
	remuxSeekMargin = 250 * time.Millisecond
	// The segment muxer cuts at the first keyframe at or after a boundary minus this delta. Remux
	// boundaries are exact keyframe times (index in ms); transcode keyframes are forced on the
	// first frame at or after a grid point, and only there, so a generous delta is safe.
	remuxCutDelta     = 2 * time.Millisecond
	transcodeCutDelta = 250 * time.Millisecond
	// noAutoKeyframes is the GOP length: keyframes come only from -force_key_frames.
	noAutoKeyframes = "9999"

	segmentPattern = "seg%05d.m4s"
	segmentList    = "list.txt"
	movFlags       = "movflags=+empty_moov+frag_custom+default_base_moof+frag_discont+skip_trailer" +
		":avoid_negative_ts=disabled:use_editlist=0"

	// videoOut is the filter_complex output label of a bitmap subtitle burn-in.
	videoOut = "[vout]"
)

// ffmpegArgs builds the complete argument list of one ffmpeg run that writes the segments
// start, start+1, ... of plan into outDir. It is the only place that knows ffmpeg's options.
//
// Burned-in subtitles keep their timing on restarts: with -copyts the decoded frames keep the
// source timestamps after the input -ss, so the subtitles filter (which reads the subtitle track
// itself) and the overlay of a bitmap track (decoded from the same input) see source time.
// TestBurnInSurvivesRestart checks this on a real ffmpeg run.
func ffmpegArgs(caps Capabilities, src Source, p Plan, start int, outDir string) ([]string, error) {
	if start < 0 || start >= len(p.Segments) {
		return nil, fmt.Errorf("segment %d out of range (0..%d)", start, len(p.Segments)-1)
	}
	v, hasVideo := videoStream(src.Probe)
	a, hasAudio := selectedAudio(src.Probe, p.Target)
	encode := hasVideo && !p.Target.VideoCopy
	copyVideo := hasVideo && p.Target.VideoCopy
	b := p.Segments[start].Start

	args := []string{"-nostdin", "-hide_banner", "-loglevel", "warning", "-copyts"}
	var vf []string
	var enc []string
	var complexGraph string
	if encode {
		burn, err := burnInOf(src.Probe, p.Target)
		if err != nil {
			return nil, err
		}
		if burn.kind == burnText && burn.path == "" {
			burn.path = src.Path
		}
		c := videoPipeline(caps, v, p)
		args = append(args, c.in...)
		enc = c.enc
		if burn.kind == burnBitmap {
			complexGraph = c.overlayGraph(v.Index, burn.stream, p.Width, p.Height)
		} else {
			vf = c.filters(burn)
		}
	}
	if start > 0 {
		if encode {
			args = append(args, "-noaccurate_seek") // the output -ss trims exactly
		}
		args = append(args, "-ss", seconds6(b))
	}
	args = append(args, "-i", src.Path)
	if complexGraph != "" {
		args = append(args, "-filter_complex", complexGraph, "-map", videoOut)
	} else if hasVideo {
		args = append(args, "-map", "0:"+strconv.Itoa(v.Index))
	}
	if hasAudio {
		args = append(args, "-map", "0:"+strconv.Itoa(a.Index))
	}
	args = append(args, "-map_metadata", "-1", "-map_chapters", "-1")

	switch {
	case copyVideo:
		args = append(args, "-c:v", "copy")
		if p.Target.VideoTag != "" {
			args = append(args, "-tag:v", p.Target.VideoTag)
		}
	case encode:
		if len(vf) > 0 {
			args = append(args, "-vf", strings.Join(vf, ","))
		}
		args = append(args, enc...)
		// Encoder time is the source time minus the output seek.
		encStart := time.Duration(0)
		if start > 0 {
			encStart = b
		}
		if kf := boundaryList(p, start, encStart); kf != "" {
			args = append(args, "-force_key_frames", kf)
		}
	}
	if hasAudio {
		if p.Target.AudioCopy {
			args = append(args, "-c:a", "copy")
		} else {
			kbps := audioBitrate / 1000
			if p.Target.AudioBitrate > 0 {
				kbps = p.Target.AudioBitrate
			}
			args = append(args, "-c:a", "aac", "-b:a", strconv.Itoa(kbps)+"k", "-ac", "2")
		}
	}

	// Output seek and timeline: segment 0 keeps everything; a restart drops what lies before its
	// boundary, and -output_ts_offset undoes the shift of the output -ss.
	if start > 0 {
		x := b
		if copyVideo {
			x = b - seekMargin(src.Keyframes, b)
		}
		args = append(args, "-ss", seconds6(x), "-output_ts_offset", seconds6(x+timelineOffset))
	} else {
		args = append(args, "-output_ts_offset", seconds6(timelineOffset))
	}

	// The segment muxer measures -segment_times from the first video packet of the run: the
	// boundary itself on a restart, the first keyframe (remux) or about 0 (transcode) at the start.
	base := b
	if start == 0 {
		base = 0
		if copyVideo && len(src.Keyframes) > 0 {
			base = src.Keyframes[0]
		}
	}
	times := boundaryList(p, start, base)
	sentinel := seconds6(p.Duration - base + time.Hour) // never reached: no default 2-s cuts
	if times == "" {
		times = sentinel
	} else {
		times += "," + sentinel
	}
	delta := transcodeCutDelta
	if copyVideo {
		delta = remuxCutDelta
	}
	args = append(args,
		"-max_muxing_queue_size", "4096",
		"-avoid_negative_ts", "disabled",
		"-f", "segment",
		"-segment_format", "mp4",
		"-segment_format_options", movFlags,
		"-segment_times", times,
		"-segment_time_delta", seconds6(delta),
		"-segment_start_number", strconv.Itoa(start),
		"-individual_header_trailer", "0",
		"-reset_timestamps", "0",
		"-segment_list", filepath.Join(outDir, segmentList),
		"-segment_list_type", "flat",
		filepath.Join(outDir, segmentPattern),
	)
	return args, nil
}

// boundaryList returns the starts of the segments after start, relative to base, comma-separated.
func boundaryList(p Plan, start int, base time.Duration) string {
	var parts []string
	for _, s := range p.Segments[start+1:] {
		parts = append(parts, seconds6(s.Start-base))
	}
	return strings.Join(parts, ",")
}

// seekMargin is remuxSeekMargin, but at most half the distance to the previous keyframe, so the
// output seek never lets that keyframe through.
func seekMargin(kf []time.Duration, b time.Duration) time.Duration {
	m := remuxSeekMargin
	i, _ := slices.BinarySearch(kf, b)
	if earlier := kf[:i]; len(earlier) > 0 {
		m = min(m, (b-earlier[len(earlier)-1])/2)
	}
	return m
}

// chain is the video pipeline of a backend: body decodes, scales and tonemaps down to SDR at the
// output size and ends in software frames (hwOut false) or in hardware frames of the backend
// (hwOut true); upload turns software frames into the encoder's hardware frames (QSV, VAAPI).
type chain struct {
	in     []string
	body   []string
	hwOut  bool
	upload []string
	enc    []string
}

// filters returns the -vf chain: body, an optional text subtitle (libass needs software frames,
// so hardware frames are downloaded first) and the upload to the encoder.
func (c chain) filters(burn burnIn) []string {
	vf := slices.Clone(c.body)
	if burn.kind == burnText {
		if c.hwOut {
			vf = append(vf, "hwdownload", "format=nv12")
		}
		vf = append(vf, burn.filter())
	} else if c.hwOut {
		return vf
	}
	return append(vf, c.upload...)
}

// overlayGraph returns the -filter_complex graph of a bitmap subtitle (PGS, DVD): the video
// pipeline in software frames, the subtitle stream (rendered by ffmpeg's sub2video) scaled to the
// output size and overlaid, then the upload to the encoder; the result is labelled videoOut.
func (c chain) overlayGraph(video, sub, w, h int) string {
	body := slices.Clone(c.body)
	if c.hwOut {
		body = append(body, "hwdownload", "format=nv12")
	}
	g := "[0:" + strconv.Itoa(video) + "]" + strings.Join(body, ",") + "[v];" +
		"[0:" + strconv.Itoa(sub) + "]scale=w=" + strconv.Itoa(w) + ":h=" + strconv.Itoa(h) + "[s];" +
		"[v][s]overlay=eof_action=pass"
	if len(c.upload) > 0 {
		g += "," + strings.Join(c.upload, ",")
	}
	return g + videoOut
}

// Tonemappers of Capabilities.Tonemapper: the filter that converts HDR to SDR.
const (
	TonemapCUDA    = "cuda"    // tonemap_cuda (jellyfin-ffmpeg), NVENC
	TonemapPlacebo = "placebo" // libplacebo over Vulkan, NVENC with a build without tonemap_cuda
	TonemapVPP     = "vpp_qsv" // vpp_qsv tonemap=1, QSV
	TonemapVAAPI   = "vaapi"   // tonemap_vaapi, VAAPI (best effort)
	TonemapCPU     = "cpu"     // zscale + tonemap (hable) on the CPU
)

// tonemapperOf returns the tonemapper of caps; without a detected value (zero Capabilities in
// tests) HWTonemap picks the GPU filter of the backend, QSV always uses vpp_qsv, else the CPU.
func tonemapperOf(caps Capabilities) string {
	switch {
	case caps.Tonemapper != "":
		return caps.Tonemapper
	case caps.Backend == BackendQSV:
		return TonemapVPP
	case caps.HWTonemap && caps.Backend == BackendNVENC:
		return TonemapCUDA
	case caps.HWTonemap && caps.Backend == BackendVAAPI:
		return TonemapVAAPI
	default:
		return TonemapCPU
	}
}

// videoPipeline returns the input options (hardware decoding), the filter chain and the encoder
// options of the transcode backend. It always scales before tonemapping (plan 3.5).
//
// Rate control: HLS encodes at constant quality capped at the plan's VideoBitrate (maxrate,
// bufsize 2 s). Target.ConstantRate (downloads, O-1) encodes at a constant bit rate of
// VideoBitrate instead, so the size is predictable: libx264 with b = minrate = maxrate and
// nal-hrd=cbr (filler data), NVENC -rc cbr, QSV b = maxrate (the encoder then picks CBR), VAAPI
// -rc_mode CBR.
//
// HDR per backend: NVENC uses tonemap_cuda when the build has it (jellyfin-ffmpeg), else
// libplacebo over Vulkan (bt.2390; the Vulkan device is derived from the CUDA device, frames go
// through system memory), else the CPU chain; QSV tonemaps in vpp_qsv, VAAPI in tonemap_vaapi
// (best effort), software with zscale and the hable curve.
//
// Dolby Vision: profile 8 (HDR10 or HLG base layer) and profile 7 (the decoders ignore the
// enhancement layer, the base layer is HDR10) tonemap like HDR10. Profile 5 (IPT-PQ-c2) is only
// correct with libplacebo (apply_dolbyvision=1 reshapes it with the RPU); every other tonemapper
// treats it as HDR10 (setparams tags it BT.2020/PQ), so its colours are off (documented in
// docs/perf/M4.md).
func videoPipeline(caps Capabilities, v Stream, p Plan) chain {
	w, h := strconv.Itoa(p.Width), strconv.Itoa(p.Height)
	br := p.VideoBitrate
	if br <= 0 {
		br = rungFor(p.Height).Bitrate
	}
	maxrate, bufsize := strconv.Itoa(br), strconv.Itoa(2*br)
	tonemap, cbr := p.Target.Tonemap, p.Target.ConstantRate
	tm := tonemapperOf(caps)
	var c chain
	placebo := caps.Backend == BackendNVENC && tm == TonemapPlacebo && hwDecodable(v)
	if tonemap && v.DolbyVisionProfile > 0 && v.ColorTransfer == "" && !placebo {
		c.body = []string{dvAsHDR10}
	}
	switch caps.Backend {
	case BackendNVENC:
		hw := hwDecodable(v)
		cudaIn := []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
		switch {
		case hw && !tonemap:
			c.in = cudaIn
			c.body = append(c.body, "scale_cuda=w="+w+":h="+h+":format=nv12")
			c.hwOut = true
		case hw && tm == TonemapCUDA:
			c.in = cudaIn
			c.body = append(c.body, "scale_cuda=w="+w+":h="+h, cudaTonemapFilter)
			c.hwOut = true
		case placebo:
			// The Vulkan device is derived from the decoder's CUDA device, so both use the same GPU.
			c.in = []string{"-init_hw_device", "cuda=cu", "-init_hw_device", "vulkan=vk@cu", "-filter_hw_device", "vk",
				"-hwaccel", "cuda", "-hwaccel_output_format", "cuda", "-hwaccel_device", "cu"}
			c.body = append(c.body, "scale_cuda=w="+w+":h="+h, "hwdownload", "format=p010le", "hwupload",
				placeboFilter(w, h, v.DolbyVisionProfile > 0), "hwdownload", "format=yuv420p")
		case hw:
			// ffmpeg without tonemap_cuda and libplacebo: scale on the GPU, tonemap on the CPU.
			c.in = cudaIn
			c.body = append(c.body, "scale_cuda=w="+w+":h="+h, "hwdownload", "format=p010le")
			c.body = append(c.body, cpuTonemap()...)
		default:
			c.body = append(c.body, softwareChain(w, h, tonemap)...)
		}
		rc := []string{"-rc", "vbr", "-cq", "23", "-b:v", "0"}
		if cbr {
			rc = []string{"-rc", "cbr", "-b:v", maxrate}
		}
		c.enc = slices.Concat([]string{"-c:v", "h264_nvenc", "-preset", "p4", "-tune", "hq"}, rc, []string{
			"-maxrate", maxrate, "-bufsize", bufsize, "-profile:v", "high", "-level:v", "4.1",
			"-g", noAutoKeyframes, "-no-scenecut", "1", "-forced-idr", "1"})
	case BackendQSV:
		c.in = []string{"-init_hw_device", "vaapi=va:" + vaapiDevice, "-init_hw_device", "qsv=qs@va", "-filter_hw_device", "qs"}
		c.upload = []string{"format=nv12", "hwupload=extra_hw_frames=64"}
		cpuTM := tonemap && tm == TonemapCPU
		vpp := "vpp_qsv=w=" + w + ":h=" + h + ":format=nv12"
		switch {
		case cpuTM:
			vpp = "vpp_qsv=w=" + w + ":h=" + h + ":format=p010"
		case tonemap:
			vpp += ":tonemap=1"
		}
		if dec, ok := qsvDecoder[v.Codec]; ok && hwDecodable(v) {
			c.in = append(c.in, "-hwaccel", "qsv", "-hwaccel_output_format", "qsv", "-c:v", dec)
			c.body = append(c.body, vpp)
		} else {
			c.body = append(c.body, "format="+uploadFormat(v), "hwupload=extra_hw_frames=64", vpp)
		}
		c.hwOut = true
		if cpuTM {
			// The GPU failed its HDR test encode: scale in vpp_qsv, tonemap on the CPU.
			c.body = append(c.body, "hwdownload", "format=p010le")
			c.body = append(c.body, cpuTonemap()...)
			c.hwOut = false
		}
		rc := []string{"-global_quality", "23"}
		if cbr {
			rc = []string{"-b:v", maxrate}
		}
		c.enc = slices.Concat([]string{"-c:v", "h264_qsv", "-preset", "veryfast"}, rc, []string{"-look_ahead", "0",
			"-maxrate", maxrate, "-bufsize", bufsize, "-profile:v", "high", "-level:v", "41",
			"-g", noAutoKeyframes, "-forced_idr", "1"})
	case BackendVAAPI:
		c.in = []string{"-init_hw_device", "vaapi=va:" + vaapiDevice, "-filter_hw_device", "va"}
		c.upload = []string{"format=nv12", "hwupload"}
		hw := hwDecodable(v)
		if hw {
			c.in = append(c.in, "-hwaccel", "vaapi", "-hwaccel_output_format", "vaapi", "-hwaccel_device", "va")
		}
		switch {
		case hw && !tonemap:
			c.body = append(c.body, "scale_vaapi=w="+w+":h="+h+":format=nv12")
			c.hwOut = true
		case hw && tm == TonemapVAAPI:
			c.body = append(c.body, "tonemap_vaapi=format=nv12:t=bt709:m=bt709:p=bt709", "scale_vaapi=w="+w+":h="+h)
			c.hwOut = true
		case hw:
			c.body = append(c.body, "scale_vaapi=w="+w+":h="+h, "hwdownload", "format=p010le")
			c.body = append(c.body, cpuTonemap()...)
		default:
			c.body = append(c.body, softwareChain(w, h, tonemap)...)
		}
		mode := "VBR"
		if cbr {
			mode = "CBR"
		}
		c.enc = []string{"-c:v", "h264_vaapi", "-rc_mode", mode, "-b:v", maxrate, "-maxrate", maxrate,
			"-bufsize", bufsize, "-profile:v", "high", "-level:v", "4.1", "-g", noAutoKeyframes}
	default:
		c.body = append(c.body, softwareChain(w, h, tonemap)...)
		rc := []string{"-crf", "23", "-maxrate", maxrate, "-bufsize", bufsize}
		if cbr {
			// nal-hrd=cbr pads with filler data up to the rate (the only x264 parameter of the chain).
			rc = []string{"-b:v", maxrate, "-minrate", maxrate, "-maxrate", maxrate, "-bufsize", bufsize,
				"-x264-params", "nal-hrd=cbr"}
		}
		c.enc = slices.Concat([]string{"-c:v", "libx264", "-preset", "veryfast"}, rc, []string{
			"-profile:v", "high", "-level:v", "4.1", "-g", noAutoKeyframes, "-sc_threshold", "0"})
	}
	return c
}

// HDRChain returns the input options and the video filters a transcode of v with plan p runs on
// this backend, ending in software frames (hardware frames are downloaded as NV12). It exists for
// the P-6 measurement (testdata/hdr-quality.sh through `slouchcat hlsbench --print-chain`), which
// renders frames through exactly the server's chain.
func HDRChain(caps Capabilities, v Stream, p Plan) (in, filters []string) {
	c := videoPipeline(caps, v, p)
	filters = slices.Clone(c.body)
	if c.hwOut {
		filters = append(filters, "hwdownload", "format=nv12")
	}
	return c.in, filters
}

// cudaTonemapFilter is the tonemap_cuda call of jellyfin-ffmpeg (BT.2390, SDR at 100 nits).
const cudaTonemapFilter = "tonemap_cuda=format=nv12:p=bt709:t=bt709:m=bt709:tonemap=bt2390:peak=100:desat=0"

// dvAsHDR10 tags Dolby Vision without transfer characteristics (profile 5) as BT.2020/PQ, so a
// tonemapper without Dolby Vision support handles it like HDR10.
const dvAsHDR10 = "setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc"

// placeboFilter tonemaps on Vulkan with BT.2390 to BT.709 SDR in 8-bit 4:2:0 at w x h;
// dolbyVision applies the RPU of a Dolby Vision stream (needed for profile 5).
func placeboFilter(w, h string, dolbyVision bool) string {
	f := "libplacebo=w=" + w + ":h=" + h + ":tonemapping=bt.2390:colorspace=bt709:color_primaries=bt709:color_trc=bt709:range=tv:format=yuv420p"
	if dolbyVision {
		f += ":apply_dolbyvision=1"
	}
	return f
}

// softwareChain scales (and tonemaps) on the CPU and ends in 8-bit 4:2:0.
func softwareChain(w, h string, tonemap bool) []string {
	if tonemap {
		return append([]string{"zscale=w=" + w + ":h=" + h}, cpuTonemap()...)
	}
	return []string{"scale=w=" + w + ":h=" + h + ":flags=bicubic", "format=yuv420p"}
}

// cpuTonemap converts HDR10/HLG to BT.709 SDR with zscale and the hable curve. ffmpeg's tonemap
// filter has no BT.2390 (only none, linear, gamma, clip, reinhard, hable, mobius); hable is the
// closest and the software reference of P-6 (docs/perf/M1b.md, M4.md).
func cpuTonemap() []string {
	return []string{"zscale=t=linear:npl=100", "format=gbrpf32le", "zscale=p=bt709",
		"tonemap=tonemap=hable:desat=0", "zscale=t=bt709:m=bt709:r=tv", "format=yuv420p"}
}

var qsvDecoder = map[string]string{
	"h264": "h264_qsv", "hevc": "hevc_qsv", "vp9": "vp9_qsv", "av1": "av1_qsv", "mpeg2video": "mpeg2_qsv",
}

// hwDecodable reports whether the GPU decodes the source: H.264 only in 8 bit, HEVC, VP9 and AV1
// in 8 and 10 bit, MPEG-2.
func hwDecodable(v Stream) bool {
	switch v.Codec {
	case "h264":
		return !highBitDepth(v.PixFmt) && !strings.Contains(v.PixFmt, "422") && !strings.Contains(v.PixFmt, "444")
	case "hevc", "vp9", "av1":
		return !strings.Contains(v.PixFmt, "422") && !strings.Contains(v.PixFmt, "444") && !strings.Contains(v.PixFmt, "12")
	case "mpeg2video":
		return true
	default:
		return false
	}
}

func highBitDepth(pixFmt string) bool {
	return strings.Contains(pixFmt, "10") || strings.Contains(pixFmt, "12")
}

// uploadFormat is the software frame format handed to hwupload for QSV.
func uploadFormat(v Stream) string {
	if highBitDepth(v.PixFmt) {
		return "p010le"
	}
	return "nv12"
}

// seconds6 formats a duration as seconds with microsecond precision, the form ffmpeg parses.
func seconds6(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', 6, 64)
}
