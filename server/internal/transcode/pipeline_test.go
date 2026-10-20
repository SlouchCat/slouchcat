package transcode

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Golden argument lists per backend and source (reviewed against ffmpeg 6.1, see ADR 0003).
// {mux} and {list} stand for the fixed segment muxer options.
func TestFFmpegArgsGolden(t *testing.T) {
	hevc := Source{
		Path: "/media/Film (2024)/Film.mkv",
		Probe: Probe{Format: Format{Duration: 13 * time.Second, BitRate: 8_800_000}, Streams: []Stream{
			{Index: 0, Type: "video", Codec: "hevc", Profile: "Main 10", Level: 120, Width: 1920, Height: 1080, PixFmt: "yuv420p10le", ColorTransfer: "bt709"},
			{Index: 1, Type: "audio", Codec: "eac3", Channels: 6},
			{Index: 2, Type: "audio", Codec: "aac", Channels: 2, Default: true},
		}},
		Keyframes: []time.Duration{0, 2 * time.Second, 5 * time.Second, 7 * time.Second, 9100 * time.Millisecond, 9200 * time.Millisecond, 12 * time.Second},
	}
	hdr := Source{
		Path: "/media/HDR.mkv",
		Probe: Probe{Format: Format{Duration: 10 * time.Second}, Streams: []Stream{
			{Index: 0, Type: "video", Codec: "hevc", Profile: "Main 10", Width: 3840, Height: 2160, PixFmt: "yuv420p10le", ColorTransfer: "smpte2084", ColorPrimaries: "bt2020"},
			{Index: 1, Type: "audio", Codec: "aac", Channels: 2},
		}},
	}
	const mux = "-max_muxing_queue_size 4096 -avoid_negative_ts disabled -f segment -segment_format mp4 " +
		"-segment_format_options movflags=+empty_moov+frag_custom+default_base_moof+frag_discont+skip_trailer:avoid_negative_ts=disabled:use_editlist=0"
	const list = "-individual_header_trailer 0 -reset_timestamps 0 -segment_list /w/s/run0001/list.txt -segment_list_type flat /w/s/run0001/seg%05d.m4s"
	cases := []struct {
		name  string
		caps  Capabilities
		src   Source
		t     Target
		start int
		want  string
	}{
		{"remux", Capabilities{Backend: BackendNVENC}, hevc, Target{Mode: ModeRemux, VideoCopy: true, AudioCopy: true, VideoTag: "hvc1"}, 0,
			`-nostdin -hide_banner -loglevel warning -copyts -i /media/Film (2024)/Film.mkv -map 0:0 -map 0:2 -map_metadata -1 -map_chapters -1 -c:v copy -tag:v hvc1 -c:a copy -output_ts_offset 10.000000 {mux} -segment_times 5.000000,9.100000,3613.000000 -segment_time_delta 0.002000 -segment_start_number 0 {list}`},
		{"remux-restart", Capabilities{Backend: BackendNVENC}, hevc, Target{Mode: ModeRemux, VideoCopy: true, VideoTag: "hvc1"}, 2,
			`-nostdin -hide_banner -loglevel warning -copyts -ss 9.100000 -i /media/Film (2024)/Film.mkv -map 0:0 -map 0:2 -map_metadata -1 -map_chapters -1 -c:v copy -tag:v hvc1 -c:a aac -b:a 160k -ac 2 -ss 8.850000 -output_ts_offset 18.850000 {mux} -segment_times 3603.900000 -segment_time_delta 0.002000 -segment_start_number 2 {list}`},
		{"sw", Capabilities{Backend: BackendSoftware}, hevc, Target{Mode: ModeTranscode, MaxHeight: 720}, 0,
			`-nostdin -hide_banner -loglevel warning -copyts -i /media/Film (2024)/Film.mkv -map 0:0 -map 0:2 -map_metadata -1 -map_chapters -1 -vf scale=w=1280:h=720:flags=bicubic,format=yuv420p -c:v libx264 -preset veryfast -crf 23 -maxrate 4000000 -bufsize 8000000 -profile:v high -level:v 4.1 -g 9999 -sc_threshold 0 -force_key_frames 4.000000,8.000000,12.000000 -c:a aac -b:a 160k -ac 2 -output_ts_offset 10.000000 {mux} -segment_times 4.000000,8.000000,12.000000,3613.000000 -segment_time_delta 0.250000 -segment_start_number 0 {list}`},
		{"sw-restart", Capabilities{Backend: BackendSoftware}, hevc, Target{Mode: ModeTranscode}, 1,
			`-nostdin -hide_banner -loglevel warning -copyts -noaccurate_seek -ss 4.000000 -i /media/Film (2024)/Film.mkv -map 0:0 -map 0:2 -map_metadata -1 -map_chapters -1 -vf scale=w=1920:h=1080:flags=bicubic,format=yuv420p -c:v libx264 -preset veryfast -crf 23 -maxrate 8000000 -bufsize 16000000 -profile:v high -level:v 4.1 -g 9999 -sc_threshold 0 -force_key_frames 4.000000,8.000000 -c:a aac -b:a 160k -ac 2 -ss 4.000000 -output_ts_offset 14.000000 {mux} -segment_times 4.000000,8.000000,3609.000000 -segment_time_delta 0.250000 -segment_start_number 1 {list}`},
		{"sw-hdr", Capabilities{Backend: BackendSoftware}, hdr, Target{Mode: ModeTranscode, Tonemap: true}, 1,
			`-nostdin -hide_banner -loglevel warning -copyts -noaccurate_seek -ss 4.000000 -i /media/HDR.mkv -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf zscale=w=1920:h=1080,zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p -c:v libx264 -preset veryfast -crf 23 -maxrate 8000000 -bufsize 16000000 -profile:v high -level:v 4.1 -g 9999 -sc_threshold 0 -force_key_frames 4.000000 -c:a aac -b:a 160k -ac 2 -ss 4.000000 -output_ts_offset 14.000000 {mux} -segment_times 4.000000,3606.000000 -segment_time_delta 0.250000 -segment_start_number 1 {list}`},
		{"nvenc", Capabilities{Backend: BackendNVENC}, hevc, Target{Mode: ModeTranscode}, 1,
			`-nostdin -hide_banner -loglevel warning -copyts -hwaccel cuda -hwaccel_output_format cuda -noaccurate_seek -ss 4.000000 -i /media/Film (2024)/Film.mkv -map 0:0 -map 0:2 -map_metadata -1 -map_chapters -1 -vf scale_cuda=w=1920:h=1080:format=nv12 -c:v h264_nvenc -preset p4 -tune hq -rc vbr -cq 23 -b:v 0 -maxrate 8000000 -bufsize 16000000 -profile:v high -level:v 4.1 -g 9999 -no-scenecut 1 -forced-idr 1 -force_key_frames 4.000000,8.000000 -c:a aac -b:a 160k -ac 2 -ss 4.000000 -output_ts_offset 14.000000 {mux} -segment_times 4.000000,8.000000,3609.000000 -segment_time_delta 0.250000 -segment_start_number 1 {list}`},
		{"nvenc-hdr", Capabilities{Backend: BackendNVENC}, hdr, Target{Mode: ModeTranscode, Tonemap: true}, 0,
			`-nostdin -hide_banner -loglevel warning -copyts -hwaccel cuda -hwaccel_output_format cuda -i /media/HDR.mkv -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale_cuda=w=1920:h=1080,hwdownload,format=p010le,zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p -c:v h264_nvenc -preset p4 -tune hq -rc vbr -cq 23 -b:v 0 -maxrate 8000000 -bufsize 16000000 -profile:v high -level:v 4.1 -g 9999 -no-scenecut 1 -forced-idr 1 -force_key_frames 4.000000,8.000000 -c:a aac -b:a 160k -ac 2 -output_ts_offset 10.000000 {mux} -segment_times 4.000000,8.000000,3610.000000 -segment_time_delta 0.250000 -segment_start_number 0 {list}`},
		{"nvenc-hdr-cuda", Capabilities{Backend: BackendNVENC, HWTonemap: true}, hdr, Target{Mode: ModeTranscode, Tonemap: true}, 0,
			`-nostdin -hide_banner -loglevel warning -copyts -hwaccel cuda -hwaccel_output_format cuda -i /media/HDR.mkv -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale_cuda=w=1920:h=1080,tonemap_cuda=format=nv12:p=bt709:t=bt709:m=bt709:tonemap=bt2390:peak=100:desat=0 -c:v h264_nvenc -preset p4 -tune hq -rc vbr -cq 23 -b:v 0 -maxrate 8000000 -bufsize 16000000 -profile:v high -level:v 4.1 -g 9999 -no-scenecut 1 -forced-idr 1 -force_key_frames 4.000000,8.000000 -c:a aac -b:a 160k -ac 2 -output_ts_offset 10.000000 {mux} -segment_times 4.000000,8.000000,3610.000000 -segment_time_delta 0.250000 -segment_start_number 0 {list}`},
		{"qsv-hdr", Capabilities{Backend: BackendQSV, HWTonemap: true}, hdr, Target{Mode: ModeTranscode, Tonemap: true}, 0,
			`-nostdin -hide_banner -loglevel warning -copyts -init_hw_device vaapi=va:/dev/dri/renderD128 -init_hw_device qsv=qs@va -filter_hw_device qs -hwaccel qsv -hwaccel_output_format qsv -c:v hevc_qsv -i /media/HDR.mkv -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf vpp_qsv=w=1920:h=1080:format=nv12:tonemap=1 -c:v h264_qsv -preset veryfast -global_quality 23 -look_ahead 0 -maxrate 8000000 -bufsize 16000000 -profile:v high -level:v 41 -g 9999 -forced_idr 1 -force_key_frames 4.000000,8.000000 -c:a aac -b:a 160k -ac 2 -output_ts_offset 10.000000 {mux} -segment_times 4.000000,8.000000,3610.000000 -segment_time_delta 0.250000 -segment_start_number 0 {list}`},
		{"vaapi", Capabilities{Backend: BackendVAAPI}, hevc, Target{Mode: ModeTranscode}, 0,
			`-nostdin -hide_banner -loglevel warning -copyts -init_hw_device vaapi=va:/dev/dri/renderD128 -filter_hw_device va -hwaccel vaapi -hwaccel_output_format vaapi -hwaccel_device va -i /media/Film (2024)/Film.mkv -map 0:0 -map 0:2 -map_metadata -1 -map_chapters -1 -vf scale_vaapi=w=1920:h=1080:format=nv12 -c:v h264_vaapi -rc_mode VBR -b:v 8000000 -maxrate 8000000 -bufsize 16000000 -profile:v high -level:v 4.1 -g 9999 -force_key_frames 4.000000,8.000000,12.000000 -c:a aac -b:a 160k -ac 2 -output_ts_offset 10.000000 {mux} -segment_times 4.000000,8.000000,12.000000,3613.000000 -segment_time_delta 0.250000 -segment_start_number 0 {list}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := NewPlan(c.src, c.t)
			if err != nil {
				t.Fatal(err)
			}
			args, err := ffmpegArgs(c.caps, c.src, p, c.start, "/w/s/run0001")
			if err != nil {
				t.Fatal(err)
			}
			want := strings.NewReplacer("{mux}", mux, "{list}", list).Replace(c.want)
			if got := strings.Join(args, " "); got != want {
				t.Errorf("args differ\ngot:  %s\nwant: %s", got, want)
			}
		})
	}
}

// Golden argument lists of the M4 variants: quality rungs, audio mapping, subtitle burn-in per
// backend, the libplacebo chain and Dolby Vision. Only the part up to the output seek is compared;
// the segment muxer options are covered above.
func TestFFmpegArgsGoldenM4(t *testing.T) {
	streams := []Stream{
		{Index: 0, Type: "video", Codec: "hevc", Profile: "Main 10", Width: 1920, Height: 1080, PixFmt: "yuv420p10le", ColorTransfer: "bt709"},
		{Index: 1, Type: "audio", Codec: "aac", Channels: 2, Default: true, Language: "ger"},
		{Index: 2, Type: "audio", Codec: "ac3", Channels: 6, Language: "eng"},
		{Index: 3, Type: "subtitle", Codec: "subrip", Language: "ger"},
		{Index: 4, Type: "subtitle", Codec: "ass", Language: "eng"},
		{Index: 5, Type: "subtitle", Codec: "hdmv_pgs_subtitle", Language: "eng"},
		{Index: 6, Type: "subtitle", Codec: "dvb_teletext"},
	}
	sdr := Source{Path: "/media/Film, Teil 2: Rückkehr (2024)/Film.mkv", Probe: Probe{Format: Format{Duration: 10 * time.Second}, Streams: streams}}
	hdr := Source{Path: "/media/HDR.mkv", Probe: Probe{Format: Format{Duration: 10 * time.Second}, Streams: []Stream{
		{Index: 0, Type: "video", Codec: "hevc", Profile: "Main 10", Width: 3840, Height: 2160, PixFmt: "yuv420p10le", ColorTransfer: "smpte2084", ColorPrimaries: "bt2020"},
		{Index: 1, Type: "audio", Codec: "aac", Channels: 2},
		{Index: 2, Type: "subtitle", Codec: "hdmv_pgs_subtitle"},
	}}}
	dv5 := Source{Path: "/media/DV5.mp4", Probe: Probe{Format: Format{Duration: 10 * time.Second}, Streams: []Stream{
		{Index: 0, Type: "video", Codec: "hevc", Profile: "Main 10", Width: 3840, Height: 2160, PixFmt: "yuv420p10le", DolbyVisionProfile: 5},
		{Index: 1, Type: "audio", Codec: "eac3", Channels: 6},
	}}}
	sw := Capabilities{Backend: BackendSoftware, Tonemapper: TonemapCPU}
	nvenc := Capabilities{Backend: BackendNVENC, Tonemapper: TonemapCPU}
	cuda := Capabilities{Backend: BackendNVENC, Tonemapper: TonemapCUDA, HWTonemap: true}
	placebo := Capabilities{Backend: BackendNVENC, Tonemapper: TonemapPlacebo, HWTonemap: true}
	qsv := Capabilities{Backend: BackendQSV, Tonemapper: TonemapVPP, HWTonemap: true}
	qsvCPU := Capabilities{Backend: BackendQSV, Tonemapper: TonemapCPU}
	vaapi := Capabilities{Backend: BackendVAAPI, Tonemapper: TonemapCPU}
	tc := func(mods ...func(*Target)) Target {
		t := Target{Mode: ModeTranscode, SubtitleBurnIn: -1, AudioStream: -1, Tonemap: true}
		for _, m := range mods {
			m(&t)
		}
		return t
	}
	quality := func(q string) func(*Target) { return func(t *Target) { t.Quality = q } }
	burn := func(i int) func(*Target) { return func(t *Target) { t.SubtitleBurnIn = i } }
	audio := func(i int, copyAudio bool) func(*Target) {
		return func(t *Target) { t.AudioStream, t.AudioCopy = i, copyAudio }
	}
	const (
		x264  = "-c:v libx264 -preset veryfast -crf 23 -maxrate %[1]d -bufsize %[2]d -profile:v high -level:v 4.1 -g 9999 -sc_threshold 0"
		nvEnc = "-c:v h264_nvenc -preset p4 -tune hq -rc vbr -cq 23 -b:v 0 -maxrate %[1]d -bufsize %[2]d -profile:v high -level:v 4.1 -g 9999 -no-scenecut 1 -forced-idr 1"
		qsEnc = "-c:v h264_qsv -preset veryfast -global_quality 23 -look_ahead 0 -maxrate %[1]d -bufsize %[2]d -profile:v high -level:v 41 -g 9999 -forced_idr 1"
		vaEnc = "-c:v h264_vaapi -rc_mode VBR -b:v %[1]d -maxrate %[1]d -bufsize %[2]d -profile:v high -level:v 4.1 -g 9999"
		aac   = "-c:a aac -b:a 160k -ac 2"
		head  = "-nostdin -hide_banner -loglevel warning -copyts"
		cudaI = "-hwaccel cuda -hwaccel_output_format cuda"
		qsvI  = "-init_hw_device vaapi=va:/dev/dri/renderD128 -init_hw_device qsv=qs@va -filter_hw_device qs -hwaccel qsv -hwaccel_output_format qsv -c:v hevc_qsv"
		vaI   = "-init_hw_device vaapi=va:/dev/dri/renderD128 -filter_hw_device va -hwaccel vaapi -hwaccel_output_format vaapi -hwaccel_device va"
		plI   = "-init_hw_device cuda=cu -init_hw_device vulkan=vk@cu -filter_hw_device vk -hwaccel cuda -hwaccel_output_format cuda -hwaccel_device cu"
		srt   = `subtitles=filename=/media/Film\, Teil 2\\: Rückkehr (2024)/Film.mkv:si=0`
		cpuTM = "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p"
		pl    = "libplacebo=w=1920:h=1080:tonemapping=bt.2390:colorspace=bt709:color_primaries=bt709:color_trc=bt709:range=tv:format=yuv420p"
		dvTag = "setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc"
	)
	enc := func(f string, rate int) string { return fmt.Sprintf(f, rate, 2*rate) }
	in := "-i " + sdr.Path
	cases := []struct {
		name  string
		caps  Capabilities
		src   Source
		t     Target
		start int
		want  string
	}{
		{"rung 480p", sw, sdr, tc(quality("480p")), 0,
			head + " " + in + " -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale=w=852:h=480:flags=bicubic,format=yuv420p " + enc(x264, 2_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"rung 720p capped at 1.5 Mbit/s", sw, sdr, tc(quality("720p"), func(t *Target) { t.MaxBitrate = 1_500_000 }), 0,
			head + " " + in + " -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale=w=1280:h=720:flags=bicubic,format=yuv420p " + enc(x264, 1_500_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"audio track 2 copied", sw, sdr, tc(audio(2, true)), 0,
			head + " " + in + " -map 0:0 -map 0:2 -map_metadata -1 -map_chapters -1 -vf scale=w=1920:h=1080:flags=bicubic,format=yuv420p " + enc(x264, 8_000_000) + " -force_key_frames 4.000000,8.000000 -c:a copy"},
		{"audio index of a subtitle keeps the default", sw, sdr, tc(audio(3, false)), 0,
			head + " " + in + " -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale=w=1920:h=1080:flags=bicubic,format=yuv420p " + enc(x264, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"burn srt software, restart", sw, sdr, tc(burn(3), quality("720p")), 1,
			head + " -noaccurate_seek -ss 4.000000 " + in + " -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale=w=1280:h=720:flags=bicubic,format=yuv420p," + srt + " " + enc(x264, 4_000_000) + " -force_key_frames 4.000000 " + aac},
		{"burn ass second subtitle stream", sw, sdr, tc(burn(4)), 0,
			head + " " + in + " -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale=w=1920:h=1080:flags=bicubic,format=yuv420p," + strings.Replace(srt, "si=0", "si=1", 1) + " " + enc(x264, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"burn sidecar", sw, sdr, tc(func(t *Target) { t.BurnInPath = `/media/it's [x];a.de.ass` }), 0,
			head + " " + in + ` -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale=w=1920:h=1080:flags=bicubic,format=yuv420p,subtitles=filename=/media/it\\\'s \[x\]\;a.de.ass ` + enc(x264, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"burn srt nvenc", nvenc, sdr, tc(burn(3)), 0,
			head + " " + cudaI + " " + in + " -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale_cuda=w=1920:h=1080:format=nv12,hwdownload,format=nv12," + srt + " " + enc(nvEnc, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"burn srt qsv", qsv, sdr, tc(burn(3)), 0,
			head + " " + qsvI + " " + in + " -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf vpp_qsv=w=1920:h=1080:format=nv12,hwdownload,format=nv12," + srt + ",format=nv12,hwupload=extra_hw_frames=64 " + enc(qsEnc, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"burn srt vaapi", vaapi, sdr, tc(burn(3)), 0,
			head + " " + vaI + " " + in + " -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale_vaapi=w=1920:h=1080:format=nv12,hwdownload,format=nv12," + srt + ",format=nv12,hwupload " + enc(vaEnc, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"burn pgs software, restart", sw, sdr, tc(burn(5), quality("720p")), 1,
			head + " -noaccurate_seek -ss 4.000000 " + in + " -filter_complex [0:0]scale=w=1280:h=720:flags=bicubic,format=yuv420p[v];[0:5]scale=w=1280:h=720[s];[v][s]overlay=eof_action=pass[vout] -map [vout] -map 0:1 -map_metadata -1 -map_chapters -1 " + enc(x264, 4_000_000) + " -force_key_frames 4.000000 " + aac},
		{"burn pgs nvenc", nvenc, sdr, tc(burn(5)), 0,
			head + " " + cudaI + " " + in + " -filter_complex [0:0]scale_cuda=w=1920:h=1080:format=nv12,hwdownload,format=nv12[v];[0:5]scale=w=1920:h=1080[s];[v][s]overlay=eof_action=pass[vout] -map [vout] -map 0:1 -map_metadata -1 -map_chapters -1 " + enc(nvEnc, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"burn pgs qsv", qsv, sdr, tc(burn(5)), 0,
			head + " " + qsvI + " " + in + " -filter_complex [0:0]vpp_qsv=w=1920:h=1080:format=nv12,hwdownload,format=nv12[v];[0:5]scale=w=1920:h=1080[s];[v][s]overlay=eof_action=pass,format=nv12,hwupload=extra_hw_frames=64[vout] -map [vout] -map 0:1 -map_metadata -1 -map_chapters -1 " + enc(qsEnc, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"burn pgs hdr cuda tonemap 720p", cuda, hdr, tc(burn(2), quality("720p")), 0,
			"-nostdin -hide_banner -loglevel warning -copyts " + cudaI + " -i /media/HDR.mkv -filter_complex [0:0]scale_cuda=w=1280:h=720," + cudaTonemapFilter + ",hwdownload,format=nv12[v];[0:2]scale=w=1280:h=720[s];[v][s]overlay=eof_action=pass[vout] -map [vout] -map 0:1 -map_metadata -1 -map_chapters -1 " + enc(nvEnc, 4_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"burn pgs hdr placebo", placebo, hdr, tc(burn(2)), 0,
			"-nostdin -hide_banner -loglevel warning -copyts " + plI + " -i /media/HDR.mkv -filter_complex [0:0]scale_cuda=w=1920:h=1080,hwdownload,format=p010le,hwupload," + pl + ",hwdownload,format=yuv420p[v];[0:2]scale=w=1920:h=1080[s];[v][s]overlay=eof_action=pass[vout] -map [vout] -map 0:1 -map_metadata -1 -map_chapters -1 " + enc(nvEnc, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"placebo hdr", placebo, hdr, tc(), 0,
			"-nostdin -hide_banner -loglevel warning -copyts " + plI + " -i /media/HDR.mkv -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale_cuda=w=1920:h=1080,hwdownload,format=p010le,hwupload," + pl + ",hwdownload,format=yuv420p " + enc(nvEnc, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"qsv hdr cpu tonemap fallback", qsvCPU, hdr, tc(), 0,
			"-nostdin -hide_banner -loglevel warning -copyts " + qsvI + " -i /media/HDR.mkv -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf vpp_qsv=w=1920:h=1080:format=p010,hwdownload,format=p010le," + cpuTM + ",format=nv12,hwupload=extra_hw_frames=64 " + enc(qsEnc, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"dolby vision 5 placebo", placebo, dv5, tc(), 0,
			"-nostdin -hide_banner -loglevel warning -copyts " + plI + " -i /media/DV5.mp4 -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf scale_cuda=w=1920:h=1080,hwdownload,format=p010le,hwupload," + pl + ":apply_dolbyvision=1,hwdownload,format=yuv420p " + enc(nvEnc, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"dolby vision 5 software as hdr10", sw, dv5, tc(), 0,
			"-nostdin -hide_banner -loglevel warning -copyts -i /media/DV5.mp4 -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf " + dvTag + ",zscale=w=1920:h=1080," + cpuTM + " " + enc(x264, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
		{"dolby vision 5 cuda as hdr10", cuda, dv5, tc(), 0,
			"-nostdin -hide_banner -loglevel warning -copyts " + cudaI + " -i /media/DV5.mp4 -map 0:0 -map 0:1 -map_metadata -1 -map_chapters -1 -vf " + dvTag + ",scale_cuda=w=1920:h=1080," + cudaTonemapFilter + " " + enc(nvEnc, 8_000_000) + " -force_key_frames 4.000000,8.000000 " + aac},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := NewPlan(c.src, c.t)
			if err != nil {
				t.Fatal(err)
			}
			args, err := ffmpegArgs(c.caps, c.src, p, c.start, "/w/s/run0001")
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Join(args, " ")
			got = got[:strings.Index(got, " -output_ts_offset")]
			if c.start > 0 {
				got = strings.TrimSuffix(got, " -ss "+seconds6(p.Segments[c.start].Start))
			}
			if got != c.want {
				t.Errorf("args differ\ngot:  %s\nwant: %s", got, c.want)
			}
		})
	}
}

func TestBurnInValidation(t *testing.T) {
	src := Source{Path: "/m.mkv", Probe: Probe{Format: Format{Duration: 10 * time.Second}, Streams: []Stream{
		{Index: 0, Type: "video", Codec: "h264", Width: 1920, Height: 1080},
		{Index: 1, Type: "subtitle", Codec: "dvb_teletext"},
		{Index: 2, Type: "subtitle", Codec: "hdmv_pgs_subtitle"},
	}}, Keyframes: []time.Duration{0}}
	if _, err := NewPlan(src, Target{Mode: ModeTranscode, SubtitleBurnIn: 1}); err == nil {
		t.Error("want error for burning in teletext")
	}
	if _, err := NewPlan(src, Target{Mode: ModeRemux, VideoCopy: true, SubtitleBurnIn: 2}); err == nil {
		t.Error("want error for burning in during a remux")
	}
	if _, err := NewPlan(src, Target{Mode: ModeRemux, VideoCopy: true, SubtitleBurnIn: -1, BurnInPath: "/m.de.srt"}); err == nil {
		t.Error("want error for burning in a sidecar during a remux")
	}
	if _, err := NewPlan(src, Target{Mode: ModeRemux, VideoCopy: true, SubtitleBurnIn: 0}); err != nil {
		t.Errorf("an index that is no subtitle burns nothing: %v", err)
	}
}

func TestFilterValue(t *testing.T) {
	cases := map[string]string{
		"/m/a b (2024)/f.mkv": "/m/a b (2024)/f.mkv",
		"/m/a:b":              `/m/a\\:b`,
		"/m/it's":             `/m/it\\\'s`,
		`/m/back\slash`:       `/m/back\\\\slash`,
		"/m/x,y;z[1]":         `/m/x\,y\;z\[1\]`,
		"/m/k=v":              `/m/k\\=v`,
	}
	for in, want := range cases {
		if got := filterValue(in); got != want {
			t.Errorf("filterValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHDRChain(t *testing.T) {
	hdr := videoSource("hevc", 3840, 2160, "smpte2084", sec(10))
	p, err := NewPlan(hdr, Target{Mode: ModeTranscode, Tonemap: true, SubtitleBurnIn: -1, AudioStream: -1})
	if err != nil {
		t.Fatal(err)
	}
	v := hdr.Probe.Streams[0]
	cases := []struct {
		caps    Capabilities
		in, out string
	}{
		{Capabilities{Backend: BackendSoftware, Tonemapper: TonemapCPU}, "", "zscale=w=1920:h=1080,zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p"},
		{Capabilities{Backend: BackendNVENC, Tonemapper: TonemapCUDA}, "-hwaccel cuda -hwaccel_output_format cuda", "scale_cuda=w=1920:h=1080," + cudaTonemapFilter + ",hwdownload,format=nv12"},
		{Capabilities{Backend: BackendQSV, Tonemapper: TonemapVPP}, "-init_hw_device vaapi=va:/dev/dri/renderD128 -init_hw_device qsv=qs@va -filter_hw_device qs -hwaccel qsv -hwaccel_output_format qsv -c:v hevc_qsv", "vpp_qsv=w=1920:h=1080:format=nv12:tonemap=1,hwdownload,format=nv12"},
	}
	for _, c := range cases {
		in, filters := HDRChain(c.caps, v, p)
		if strings.Join(in, " ") != c.in || strings.Join(filters, ",") != c.out {
			t.Errorf("%s/%s: in %q filters %q", c.caps.Backend, c.caps.Tonemapper, in, strings.Join(filters, ","))
		}
	}
}
