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
)

// ffmpegArgs builds the complete argument list of one ffmpeg run that writes the segments
// start, start+1, ... of plan into outDir. It is the only place that knows ffmpeg's options.
func ffmpegArgs(caps Capabilities, src Source, p Plan, start int, outDir string) ([]string, error) {
	if start < 0 || start >= len(p.Segments) {
		return nil, fmt.Errorf("segment %d out of range (0..%d)", start, len(p.Segments)-1)
	}
	v, hasVideo := videoStream(src.Probe)
	a, hasAudio := audioStream(src.Probe)
	encode := hasVideo && !p.Target.VideoCopy
	copyVideo := hasVideo && p.Target.VideoCopy
	b := p.Segments[start].Start

	args := []string{"-nostdin", "-hide_banner", "-loglevel", "warning", "-copyts"}
	var vf []string
	var enc []string
	if encode {
		var in []string
		in, vf, enc = videoPipeline(caps, v, p)
		args = append(args, in...)
	}
	if start > 0 {
		if encode {
			args = append(args, "-noaccurate_seek") // the output -ss trims exactly
		}
		args = append(args, "-ss", seconds6(b))
	}
	args = append(args, "-i", src.Path)
	if hasVideo {
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
			args = append(args, "-c:a", "aac", "-b:a", strconv.Itoa(audioBitrate/1000)+"k", "-ac", "2")
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

// videoPipeline returns the input options (hardware decoding), the filter chain and the encoder
// options of the transcode backend.
func videoPipeline(caps Capabilities, v Stream, p Plan) (in, vf, enc []string) {
	w, h := strconv.Itoa(p.Width), strconv.Itoa(p.Height)
	br := videoBitrate(p.Height)
	maxrate, bufsize := strconv.Itoa(br), strconv.Itoa(2*br)
	tonemap := p.Target.Tonemap
	switch caps.Backend {
	case BackendNVENC:
		hw := hwDecodable(v)
		switch {
		case hw && !tonemap:
			in = []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
			vf = []string{"scale_cuda=w=" + w + ":h=" + h + ":format=nv12"}
		case hw && caps.HWTonemap:
			in = []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
			vf = []string{"scale_cuda=w=" + w + ":h=" + h,
				"tonemap_cuda=format=nv12:p=bt709:t=bt709:m=bt709:tonemap=bt2390:peak=100:desat=0"}
		case hw:
			// ffmpeg without tonemap_cuda (Ubuntu 6.1): scale on the GPU, tonemap on the CPU.
			in = []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
			vf = append([]string{"scale_cuda=w=" + w + ":h=" + h, "hwdownload", "format=p010le"}, cpuTonemap()...)
		default:
			vf = softwareChain(w, h, tonemap)
		}
		enc = []string{"-c:v", "h264_nvenc", "-preset", "p4", "-tune", "hq", "-rc", "vbr", "-cq", "23", "-b:v", "0",
			"-maxrate", maxrate, "-bufsize", bufsize, "-profile:v", "high", "-level:v", "4.1",
			"-g", noAutoKeyframes, "-no-scenecut", "1", "-forced-idr", "1"}
	case BackendQSV:
		in = []string{"-init_hw_device", "vaapi=va:" + vaapiDevice, "-init_hw_device", "qsv=qs@va", "-filter_hw_device", "qs"}
		vpp := "vpp_qsv=w=" + w + ":h=" + h + ":format=nv12"
		if tonemap {
			vpp += ":tonemap=1"
		}
		if dec, ok := qsvDecoder[v.Codec]; ok && hwDecodable(v) {
			in = append(in, "-hwaccel", "qsv", "-hwaccel_output_format", "qsv", "-c:v", dec)
			vf = []string{vpp}
		} else {
			vf = []string{"format=" + uploadFormat(v), "hwupload=extra_hw_frames=64", vpp}
		}
		enc = []string{"-c:v", "h264_qsv", "-preset", "veryfast", "-global_quality", "23", "-look_ahead", "0",
			"-maxrate", maxrate, "-bufsize", bufsize, "-profile:v", "high", "-level:v", "41",
			"-g", noAutoKeyframes, "-forced_idr", "1"}
	case BackendVAAPI:
		in = []string{"-init_hw_device", "vaapi=va:" + vaapiDevice, "-filter_hw_device", "va"}
		hw := hwDecodable(v)
		if hw {
			in = append(in, "-hwaccel", "vaapi", "-hwaccel_output_format", "vaapi", "-hwaccel_device", "va")
		}
		switch {
		case hw && !tonemap:
			vf = []string{"scale_vaapi=w=" + w + ":h=" + h + ":format=nv12"}
		case hw && caps.HWTonemap:
			vf = []string{"tonemap_vaapi=format=nv12:t=bt709:m=bt709:p=bt709", "scale_vaapi=w=" + w + ":h=" + h}
		case hw:
			vf = slices.Concat([]string{"scale_vaapi=w=" + w + ":h=" + h, "hwdownload", "format=p010le"},
				cpuTonemap(), []string{"format=nv12", "hwupload"})
		default:
			vf = append(softwareChain(w, h, tonemap), "format=nv12", "hwupload")
		}
		enc = []string{"-c:v", "h264_vaapi", "-rc_mode", "VBR", "-b:v", maxrate, "-maxrate", maxrate,
			"-bufsize", bufsize, "-profile:v", "high", "-level:v", "4.1", "-g", noAutoKeyframes}
	default:
		vf = softwareChain(w, h, tonemap)
		enc = []string{"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
			"-maxrate", maxrate, "-bufsize", bufsize, "-profile:v", "high", "-level:v", "4.1",
			"-g", noAutoKeyframes, "-sc_threshold", "0"}
	}
	return in, vf, enc
}

// softwareChain scales (and tonemaps) on the CPU and ends in 8-bit 4:2:0.
func softwareChain(w, h string, tonemap bool) []string {
	if tonemap {
		return append([]string{"zscale=w=" + w + ":h=" + h}, cpuTonemap()...)
	}
	return []string{"scale=w=" + w + ":h=" + h + ":flags=bicubic", "format=yuv420p"}
}

// cpuTonemap converts HDR10/HLG to BT.709 SDR with zscale and the hable curve (the tonemap filter
// of ffmpeg has no bt.2390).
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
