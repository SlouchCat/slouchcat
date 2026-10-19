package transcode

import (
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
