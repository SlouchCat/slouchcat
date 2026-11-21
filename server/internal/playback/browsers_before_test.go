package playback

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Concept 26 stage 1 must not change what today's browsers get (concept 26 section 7). The rows
// below run the browser profiles of today over sources with Dolby and AAC tracks whose bit rate is
// probed (the field stage 1 added), with and without a bandwidth limit, and compare the whole
// outcome (decision, master playlist bandwidth and codecs, quality options) with the values
// origin/main (13ef4798) computed for the same input: the expectations were produced by running
// this file on that commit, not derived from the new code. The one deliberate change (MP3 is no
// longer copied into HLS) has its own row.

// browserProfiles are the capability reports of the web client of today.
var browserProfiles = []struct {
	name string
	caps model.PlaybackCapabilities
}{
	{"chrome", capsChrome},
	{"firefox", capsFirefox},
	{"safari-mac", capsSafari},
	{"safari-iphone", model.PlaybackCapabilities{Containers: []string{"mp4"}, Video: []string{"h264", "hevc"},
		Audio: []string{"aac", "ac3", "eac3", "mp3", "flac"}, HLS: "native", MaxWidth: 1179, MaxHeight: 2556, HDR: true, Type: "phone"}},
	{"edge", model.PlaybackCapabilities{Containers: []string{"mp4", "webm"}, Video: []string{"h264", "hevc", "vp9", "av1"},
		Audio: []string{"aac", "ac3", "eac3", "mp3", "opus", "flac"}, HLS: "mse", MaxWidth: 1920, MaxHeight: 1080, Type: "desktop"}},
}

// browserSources are JSON probes (the stored form, so the measured bitRate is read where the
// field exists): stream 1 the default track, stream 2 a second one.
var browserSources = []struct {
	name  string
	file  model.MediaFile
	probe string
	audio int
}{
	{"hevc-hdr-aac320", model.MediaFile{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac"},
		`{"format":{"duration":60000000000,"bitRate":30000000},"streams":[
		{"index":0,"type":"video","codec":"hevc","profile":"Main 10","width":3840,"height":2160,"pixFmt":"yuv420p10le","colorTransfer":"smpte2084"},
		{"index":1,"type":"audio","codec":"aac","channels":2,"default":true,"bitRate":320000},
		{"index":2,"type":"audio","codec":"ac3","channels":6,"bitRate":640000}]}`, -1},
	{"h264-mkv-aac320", model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "aac"},
		`{"format":{"duration":60000000000,"bitRate":9000000},"streams":[
		{"index":0,"type":"video","codec":"h264","profile":"High","width":1920,"height":1080,"pixFmt":"yuv420p"},
		{"index":1,"type":"audio","codec":"aac","channels":6,"default":true,"bitRate":320000}]}`, -1},
	{"h264-mkv-ac3", model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "ac3"},
		`{"format":{"duration":60000000000,"bitRate":9000000},"streams":[
		{"index":0,"type":"video","codec":"h264","profile":"High","width":1920,"height":1080,"pixFmt":"yuv420p"},
		{"index":1,"type":"audio","codec":"ac3","channels":6,"default":true,"bitRate":640000}]}`, -1},
	{"hevc-hdr-eac3", model.MediaFile{Container: "mkv", VideoCodec: "hevc", AudioCodec: "eac3"},
		`{"format":{"duration":60000000000,"bitRate":30000000},"streams":[
		{"index":0,"type":"video","codec":"hevc","profile":"Main 10","width":3840,"height":2160,"pixFmt":"yuv420p10le","colorTransfer":"smpte2084"},
		{"index":1,"type":"audio","codec":"eac3","channels":6,"default":true,"bitRate":768000}]}`, -1},
	{"h264-mp4-ac3-track", model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"},
		`{"format":{"duration":60000000000,"bitRate":9000000},"streams":[
		{"index":0,"type":"video","codec":"h264","profile":"High","width":1920,"height":1080,"pixFmt":"yuv420p"},
		{"index":1,"type":"audio","codec":"aac","channels":2,"default":true,"bitRate":192000},
		{"index":2,"type":"audio","codec":"ac3","channels":6,"bitRate":640000}]}`, 2},
}

// browserLimits are the bandwidth limits of the rows: none, a middle one and the smallest the
// contract allows.
var browserLimits = []int{0, 4200, 200}

// browserOutcome is the whole outcome of one row as one line.
func browserOutcome(t *testing.T, caps model.PlaybackCapabilities, file model.MediaFile, probeJSON string, audio, limit int) string {
	t.Helper()
	var p transcode.Probe
	if err := json.Unmarshal([]byte(probeJSON), &p); err != nil {
		t.Fatal(err)
	}
	limits := model.PlaybackLimits{MaxBitrateKbps: limit}
	c := normalizeCapabilities(&caps)
	req := Request{Caps: c, Limits: limits, AudioStream: audio}
	d := Decide(file, &p, req)
	out := fmt.Sprintf("%s/%s copy=%t q=%s %dx%d br=%d tag=%s tm=%t", d.Mode, d.Reason, d.AudioCopy, d.Quality, d.Width, d.Height,
		d.Bitrate, d.VideoTag, d.Tonemap)
	if d.Mode != ModeDirect {
		src := transcode.Source{Probe: p, Keyframes: []time.Duration{0, 4 * time.Second, 8 * time.Second}}
		plan, err := transcode.NewPlan(src, hlsTarget(HLSRequest{Decision: d}, transcode.Capabilities{}))
		if err != nil {
			t.Fatal(err)
		}
		out += fmt.Sprintf(" bw=%d vbr=%d codecs=%s", plan.Bandwidth, plan.VideoBitrate, transcode.CodecsString(plan, src))
	}
	var opts []string
	for _, o := range QualityOptions(d, newSource(file, &p, limits, c, transcode.Capabilities{}), originalAvailable(d, file, &p, req), "") {
		if o.Available {
			opts = append(opts, fmt.Sprintf("%s:%d", o.Quality, o.Bitrate))
		}
	}
	return out + " opts=" + strings.Join(opts, ",")
}

func TestBrowsersAsBefore(t *testing.T) {
	t.Parallel()
	var got []string
	for _, b := range browserProfiles {
		for _, s := range browserSources {
			for _, l := range browserLimits {
				got = append(got, fmt.Sprintf("%s %s %d: %s", b.name, s.name, l, browserOutcome(t, b.caps, s.file, s.probe, s.audio, l)))
			}
		}
	}
	if len(browsersBefore) == 0 {
		t.Fatalf("no expectations; this run computes:\n%s", strings.Join(got, "\n"))
	}
	for i, line := range got {
		if i >= len(browsersBefore) || line != browsersBefore[i] {
			want := ""
			if i < len(browsersBefore) {
				want = browsersBefore[i]
			}
			t.Errorf("row %d\n got: %s\nwant: %s", i, line, want)
		}
	}
	if len(got) != len(browsersBefore) {
		t.Errorf("%d rows, want %d", len(got), len(browsersBefore))
	}
}

// The deliberate change for browsers (ADR 0041): an MKV with MP3 audio remuxes with the audio
// encoded to AAC. origin/main copied the MP3 and named it AAC (mp4a.40.2).
func TestBrowserMP3Encoded(t *testing.T) {
	t.Parallel()
	mkv := model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "mp3"}
	probe := `{"format":{"duration":60000000000,"bitRate":9000000},"streams":[
		{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p"},
		{"index":1,"type":"audio","codec":"mp3","channels":2,"default":true,"bitRate":192000}]}`
	got := browserOutcome(t, capsChrome, mkv, probe, -1, 0)
	if want := "remux/container copy=false q=auto"; !strings.HasPrefix(got, want) || !strings.Contains(got, ",mp4a.40.2 ") {
		t.Errorf("mp3 mkv on chrome: %s\nwant %s… with mp4a.40.2", got, want)
	}
}

// browsersBefore are the rows as origin/main (13ef4798) computes them.
var browsersBefore = []string{
	"chrome hevc-hdr-aac320 0: transcode/hdr copy=true q=1080p 1920x1080 br=8000000 tag= tm=true bw=8160000 vbr=8000000 codecs=avc1.640029,mp4a.40.2 opts=auto:8000000,1080p:8000000,720p:4000000,480p:2000000,360p:1000000",
	"chrome hevc-hdr-aac320 4200: transcode/hdr copy=true q=720p 1280x720 br=4000000 tag= tm=true bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"chrome hevc-hdr-aac320 200: transcode/hdr copy=true q=360p 640x360 br=40000 tag= tm=true bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"chrome h264-mkv-aac320 0: remux/container copy=true q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,mp4a.40.2 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"chrome h264-mkv-aac320 4200: transcode/bandwidth copy=true q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"chrome h264-mkv-aac320 200: transcode/bandwidth copy=true q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"chrome h264-mkv-ac3 0: remux/container copy=false q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,mp4a.40.2 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"chrome h264-mkv-ac3 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"chrome h264-mkv-ac3 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"chrome hevc-hdr-eac3 0: transcode/hdr copy=false q=1080p 1920x1080 br=8000000 tag= tm=true bw=8160000 vbr=8000000 codecs=avc1.640029,mp4a.40.2 opts=auto:8000000,1080p:8000000,720p:4000000,480p:2000000,360p:1000000",
	"chrome hevc-hdr-eac3 4200: transcode/hdr copy=false q=720p 1280x720 br=4000000 tag= tm=true bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"chrome hevc-hdr-eac3 200: transcode/hdr copy=false q=360p 640x360 br=40000 tag= tm=true bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"chrome h264-mp4-ac3-track 0: remux/audio_track copy=false q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,mp4a.40.2 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"chrome h264-mp4-ac3-track 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"chrome h264-mp4-ac3-track 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"firefox hevc-hdr-aac320 0: transcode/hdr copy=true q=1080p 1920x1080 br=8000000 tag= tm=true bw=8160000 vbr=8000000 codecs=avc1.640029,mp4a.40.2 opts=auto:8000000,1080p:8000000,720p:4000000,480p:2000000,360p:1000000",
	"firefox hevc-hdr-aac320 4200: transcode/hdr copy=true q=720p 1280x720 br=4000000 tag= tm=true bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"firefox hevc-hdr-aac320 200: transcode/hdr copy=true q=360p 640x360 br=40000 tag= tm=true bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"firefox h264-mkv-aac320 0: remux/container copy=true q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,mp4a.40.2 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"firefox h264-mkv-aac320 4200: transcode/bandwidth copy=true q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"firefox h264-mkv-aac320 200: transcode/bandwidth copy=true q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"firefox h264-mkv-ac3 0: remux/container copy=false q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,mp4a.40.2 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"firefox h264-mkv-ac3 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"firefox h264-mkv-ac3 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"firefox hevc-hdr-eac3 0: transcode/hdr copy=false q=1080p 1920x1080 br=8000000 tag= tm=true bw=8160000 vbr=8000000 codecs=avc1.640029,mp4a.40.2 opts=auto:8000000,1080p:8000000,720p:4000000,480p:2000000,360p:1000000",
	"firefox hevc-hdr-eac3 4200: transcode/hdr copy=false q=720p 1280x720 br=4000000 tag= tm=true bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"firefox hevc-hdr-eac3 200: transcode/hdr copy=false q=360p 640x360 br=40000 tag= tm=true bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"firefox h264-mp4-ac3-track 0: remux/audio_track copy=false q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,mp4a.40.2 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"firefox h264-mp4-ac3-track 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"firefox h264-mp4-ac3-track 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"safari-mac hevc-hdr-aac320 0: remux/container copy=true q=auto 3840x2160 br=30000000 tag=hvc1 tm=false bw=36000000 vbr=0 codecs=hvc1.2.4.L120.B0,mp4a.40.2 opts=auto:30000000,original:30000000,1080p:8000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-mac hevc-hdr-aac320 4200: transcode/bandwidth copy=true q=720p 1280x720 br=4000000 tag= tm=true bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-mac hevc-hdr-aac320 200: transcode/bandwidth copy=true q=360p 640x360 br=40000 tag= tm=true bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"safari-mac h264-mkv-aac320 0: remux/container copy=true q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,mp4a.40.2 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-mac h264-mkv-aac320 4200: transcode/bandwidth copy=true q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-mac h264-mkv-aac320 200: transcode/bandwidth copy=true q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"safari-mac h264-mkv-ac3 0: remux/container copy=true q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,ac-3 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-mac h264-mkv-ac3 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-mac h264-mkv-ac3 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"safari-mac hevc-hdr-eac3 0: remux/container copy=true q=auto 3840x2160 br=30000000 tag=hvc1 tm=false bw=36000000 vbr=0 codecs=hvc1.2.4.L120.B0,ec-3 opts=auto:30000000,original:30000000,1080p:8000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-mac hevc-hdr-eac3 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=true bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-mac hevc-hdr-eac3 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=true bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"safari-mac h264-mp4-ac3-track 0: remux/audio_track copy=true q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,ac-3 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-mac h264-mp4-ac3-track 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-mac h264-mp4-ac3-track 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"safari-iphone hevc-hdr-aac320 0: remux/container copy=true q=auto 3840x2160 br=30000000 tag=hvc1 tm=false bw=36000000 vbr=0 codecs=hvc1.2.4.L120.B0,mp4a.40.2 opts=auto:30000000,original:30000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-iphone hevc-hdr-aac320 4200: transcode/bandwidth copy=true q=720p 1280x720 br=4000000 tag= tm=true bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-iphone hevc-hdr-aac320 200: transcode/bandwidth copy=true q=360p 640x360 br=40000 tag= tm=true bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"safari-iphone h264-mkv-aac320 0: remux/container copy=true q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,mp4a.40.2 opts=auto:9000000,original:9000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-iphone h264-mkv-aac320 4200: transcode/bandwidth copy=true q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-iphone h264-mkv-aac320 200: transcode/bandwidth copy=true q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"safari-iphone h264-mkv-ac3 0: remux/container copy=true q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,ac-3 opts=auto:9000000,original:9000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-iphone h264-mkv-ac3 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-iphone h264-mkv-ac3 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"safari-iphone hevc-hdr-eac3 0: remux/container copy=true q=auto 3840x2160 br=30000000 tag=hvc1 tm=false bw=36000000 vbr=0 codecs=hvc1.2.4.L120.B0,ec-3 opts=auto:30000000,original:30000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-iphone hevc-hdr-eac3 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=true bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-iphone hevc-hdr-eac3 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=true bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"safari-iphone h264-mp4-ac3-track 0: remux/audio_track copy=true q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,ac-3 opts=auto:9000000,original:9000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-iphone h264-mp4-ac3-track 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"safari-iphone h264-mp4-ac3-track 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"edge hevc-hdr-aac320 0: transcode/hdr copy=true q=1080p 1920x1080 br=8000000 tag= tm=true bw=8160000 vbr=8000000 codecs=avc1.640029,mp4a.40.2 opts=auto:8000000,1080p:8000000,720p:4000000,480p:2000000,360p:1000000",
	"edge hevc-hdr-aac320 4200: transcode/hdr copy=true q=720p 1280x720 br=4000000 tag= tm=true bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"edge hevc-hdr-aac320 200: transcode/hdr copy=true q=360p 640x360 br=40000 tag= tm=true bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"edge h264-mkv-aac320 0: remux/container copy=true q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,mp4a.40.2 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"edge h264-mkv-aac320 4200: transcode/bandwidth copy=true q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"edge h264-mkv-aac320 200: transcode/bandwidth copy=true q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"edge h264-mkv-ac3 0: remux/container copy=true q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,ac-3 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"edge h264-mkv-ac3 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"edge h264-mkv-ac3 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"edge hevc-hdr-eac3 0: transcode/hdr copy=false q=1080p 1920x1080 br=8000000 tag= tm=true bw=8160000 vbr=8000000 codecs=avc1.640029,mp4a.40.2 opts=auto:8000000,1080p:8000000,720p:4000000,480p:2000000,360p:1000000",
	"edge hevc-hdr-eac3 4200: transcode/hdr copy=false q=720p 1280x720 br=4000000 tag= tm=true bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"edge hevc-hdr-eac3 200: transcode/hdr copy=false q=360p 640x360 br=40000 tag= tm=true bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
	"edge h264-mp4-ac3-track 0: remux/audio_track copy=true q=auto 1920x1080 br=9000000 tag= tm=false bw=10800000 vbr=0 codecs=avc1.640029,ac-3 opts=auto:9000000,original:9000000,1080p:9000000,720p:4000000,480p:2000000,360p:1000000",
	"edge h264-mp4-ac3-track 4200: transcode/bandwidth copy=false q=720p 1280x720 br=4000000 tag= tm=false bw=4160000 vbr=4000000 codecs=avc1.640029,mp4a.40.2 opts=auto:4000000,720p:4000000,480p:2000000,360p:1000000",
	"edge h264-mp4-ac3-track 200: transcode/bandwidth copy=false q=360p 640x360 br=40000 tag= tm=false bw=200000 vbr=40000 codecs=avc1.640029,mp4a.40.2 opts=auto:40000,360p:40000",
}
