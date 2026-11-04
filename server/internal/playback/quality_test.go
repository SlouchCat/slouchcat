package playback

import (
	"context"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Transcode hosts: QSV with tonemapping on the CPU (CT 190), NVENC with tonemap_cuda and software.
var (
	hostQSV      = transcode.Capabilities{Backend: transcode.BackendQSV, Tonemapper: transcode.TonemapCPU}
	hostNVENC    = transcode.Capabilities{Backend: transcode.BackendNVENC, Tonemapper: transcode.TonemapCUDA, HWTonemap: true}
	hostSoftware = transcode.Capabilities{Backend: transcode.BackendSoftware, Tonemapper: transcode.TonemapCPU}
	// caps4K is a desktop Chrome on a 4K screen; it decodes no HEVC, so HEVC sources transcode.
	caps4K = model.PlaybackCapabilities{Containers: []string{"mp4", "webm"}, Video: []string{"h264", "vp9", "av1"},
		Audio: []string{"aac", "opus"}, HLS: "mse", MaxWidth: 3840, MaxHeight: 2160, Type: "desktop"}
)

// option returns the option of quality q.
func option(t *testing.T, opts []model.QualityOption, q string) model.QualityOption {
	t.Helper()
	for _, o := range opts {
		if o.Quality == q {
			return o
		}
	}
	t.Fatalf("no option %s in %+v", q, opts)
	return model.QualityOption{}
}

// Concept 13 11.5: original never re-encodes the video. It plays directly or as a remux (the
// audio may be converted) and keeps original as its quality (d); a remembered cap does not apply
// to it; where the video would be re-encoded (burned-in subtitles (b), a codec the client cannot
// decode) it plays as auto (c).
func TestDecideOriginal(t *testing.T) {
	t.Parallel()
	mp4 := model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080}
	mkv := model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "aac"}
	dts := model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "dts"}
	hevc := model.MediaFile{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac"}
	p := sizedProbe("h264", "bt709", 1920, 1080, 6_000_000)
	pDTS := sizedProbe("h264", "bt709", 1920, 1080, 6_000_000)
	pDTS.Streams[1].Codec = "dts"
	pHEVC := sizedProbe("hevc", "bt709", 1920, 1080, 6_000_000)
	orig := func(caps model.PlaybackCapabilities) Request {
		return Request{Caps: caps, AudioStream: -1, Quality: model.QualityOriginal, Host: hostQSV}
	}
	withQuality := func(d Decision, q string) Decision { d.Quality = q; return d }

	directOrig := withQuality(direct(1920, 1080, 6_000_000, 1), model.QualityOriginal)
	if got := Decide(mp4, p, orig(capsChrome)); got != directOrig {
		t.Errorf("original of an MP4 = %+v, want %+v", got, directOrig)
	}
	if got, want := Decide(mkv, p, orig(capsChrome)), withQuality(remux(ReasonContainer, true, "", 1920, 1080, 6_000_000, 1), model.QualityOriginal); got != want {
		t.Errorf("original of an MKV = %+v, want %+v", got, want)
	}
	// The audio may be converted: DTS becomes AAC, the video stays.
	if got, want := Decide(dts, pDTS, orig(capsChrome)), withQuality(remux(ReasonContainer, false, "", 1920, 1080, 6_000_000, 1), model.QualityOriginal); got != want {
		t.Errorf("original with DTS = %+v, want %+v", got, want)
	}
	// quality wins over a remembered cap.
	capped := orig(capsChrome)
	capped.QualityCap = model.Quality720
	if got := Decide(mp4, p, capped); got != directOrig {
		t.Errorf("original with a cap of 720p = %+v, want %+v", got, directOrig)
	}
	// (c) A codec the client cannot decode: auto, a transcode on the rung of the source.
	if got, want := Decide(hevc, pHEVC, orig(capsChrome)), transcodeTo(ReasonVideoCodec, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1); got != want {
		t.Errorf("original of HEVC in Chrome = %+v, want %+v", got, want)
	}
	// (b) Burned-in subtitles re-encode the video: auto.
	pgs := &model.Track{ID: 7, Index: 3, Kind: "subtitle", Codec: "hdmv_pgs_subtitle", Source: "embedded"}
	burn := orig(capsChrome)
	burn.Subtitle = pgs
	if got := Decide(mp4, p, burn); got.Mode != ModeTranscode || got.Reason != ReasonBurnIn || got.Quality != model.Quality1080 || got.BurnStream != 3 {
		t.Errorf("original with burned-in subtitles = %+v", got)
	}

	// originalAvailable: independent of the quality that runs.
	auto := Request{Caps: capsChrome, AudioStream: -1, Host: hostQSV}
	manual := auto
	manual.Quality = model.Quality720
	autoBurn := auto
	autoBurn.Subtitle = pgs
	for _, c := range []struct {
		name string
		file model.MediaFile
		p    *transcode.Probe
		req  Request
		d    func(Decision) Decision
		want bool
	}{
		{"auto plays directly", mp4, p, auto, nil, true},
		{"original plays", mkv, p, orig(capsChrome), nil, true},
		{"manual 720p of a remuxable file", mkv, p, manual, nil, true},
		{"(b) burned-in subtitles", mp4, p, autoBurn, nil, false},
		{"HEVC in Chrome", hevc, pHEVC, auto, nil, false},
		{"(a) the remux found no keyframe index", mkv, p, orig(capsChrome), func(d Decision) Decision {
			return newSource(mkv, p, model.PlaybackLimits{}, capsChrome, hostQSV).transcode(d, ReasonKeyframesPending)
		}, false},
	} {
		d := Decide(c.file, c.p, c.req)
		if c.d != nil {
			d = c.d(d)
		}
		if got := originalAvailable(d, c.file, c.p, c.req); got != c.want {
			t.Errorf("%s: originalAvailable = %v, want %v (%+v)", c.name, got, c.want, d)
		}
	}
}

// V-17: a remembered cap (qualityCap) acts like a device limit: a file within it keeps playing
// directly or as a remux, a file above it is transcoded to the cap (reason quality); quality wins
// over it; the panel keeps auto current.
func TestDecideQualityCap(t *testing.T) {
	t.Parallel()
	mp4 := model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac", Width: 1920, Height: 1080}
	mkv := model.MediaFile{Container: "mkv", VideoCodec: "h264", AudioCodec: "aac"}
	hevc := model.MediaFile{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac"}
	p := sizedProbe("h264", "bt709", 1920, 1080, 6_000_000)
	small := sizedProbe("h264", "bt709", 1280, 720, 3_000_000)
	uhd := sizedProbe("hevc", "bt709", 3840, 2160, 40_000_000)
	capped := func(capQ, quality string) Request {
		return Request{Caps: capsChrome, AudioStream: -1, Quality: quality, QualityCap: capQ, Host: hostQSV}
	}
	for _, c := range []struct {
		name string
		file model.MediaFile
		p    *transcode.Probe
		req  Request
		want Decision
	}{
		{"1080p file within a cap of 1080p stays direct", mp4, p, capped(model.Quality1080, ""), direct(1920, 1080, 6_000_000, 1)},
		{"720p file within a cap of 720p stays direct", mp4, small, capped(model.Quality720, model.QualityAuto), direct(1280, 720, 3_000_000, 1)},
		{"MKV within the cap stays a remux", mkv, p, capped(model.Quality1080, ""), remux(ReasonContainer, true, "", 1920, 1080, 6_000_000, 1)},
		{"1080p file above a cap of 720p", mp4, p, capped(model.Quality720, ""), transcodeTo(ReasonQuality, model.Quality720, 1280, 720, 4_000_000, false, true, 1)},
		{"4K transcode under a cap of 480p", hevc, uhd, capped(model.Quality480, ""), transcodeTo(ReasonQuality, model.Quality480, 852, 480, 2_000_000, false, true, 1)},
		{"quality 1080p wins over a cap of 720p", mp4, p, capped(model.Quality720, model.Quality1080), direct(1920, 1080, 6_000_000, 1)},
	} {
		if got := Decide(c.file, c.p, c.req); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
	req := capped(model.Quality720, "")
	d := Decide(mp4, p, req)
	opts := QualityOptions(d, newSource(mp4, p, model.PlaybackLimits{}, capsChrome, hostQSV), originalAvailable(d, mp4, p, req), "")
	if a, r := option(t, opts, model.QualityAuto), option(t, opts, model.Quality720); !a.Current || r.Current || a.Direct || a.Codec != "h264" || a.Height != 720 {
		t.Errorf("capped options: auto %+v, 720p %+v", a, r)
	}
	if o := option(t, opts, model.QualityOriginal); !o.Available || o.Current || !o.Direct || o.Height != 1080 || o.Bitrate != 6_000_000 {
		t.Errorf("capped options: original %+v", o)
	}
	for _, q := range []string{"", model.Quality1080, model.Quality720, model.Quality480, model.Quality360} {
		if !validQualityCap(q) {
			t.Errorf("validQualityCap(%q) = false", q)
		}
	}
	for _, q := range []string{model.QualityAuto, model.QualityOriginal, model.Quality2160, "4k"} {
		if validQualityCap(q) {
			t.Errorf("validQualityCap(%q) = true", q)
		}
	}
}

// ADR 0021 end to end in Decide: 2160p transcodes on the 2160p rung only on a hardware backend,
// from a 4K source (2160 lines or 3840 pixels wide), on a device of at least 1440 lines, within the
// profile's limits and from SDR (or with tonemapping on the GPU); else it runs on 1080p. auto never
// picks 2160p.
func TestDecide2160(t *testing.T) {
	t.Parallel()
	hevc := model.MediaFile{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac"}
	uhd := sizedProbe("hevc", "bt709", 3840, 2160, 40_000_000)
	hdr := sizedProbe("hevc", "smpte2084", 3840, 2160, 40_000_000)
	scope := sizedProbe("hevc", "bt709", 3840, 1600, 30_000_000)
	qhd := sizedProbe("hevc", "bt709", 2560, 1440, 20_000_000)
	pick := func(host transcode.Capabilities, caps model.PlaybackCapabilities, limits model.PlaybackLimits, quality string) Request {
		return Request{Caps: caps, Limits: limits, AudioStream: -1, Quality: quality, Host: host}
	}
	hd := model.PlaybackCapabilities{Video: []string{"h264"}, Audio: []string{"aac"}, HLS: "mse", MaxWidth: 1920, MaxHeight: 1080}
	phone := caps4K
	phone.Type = "phone"
	uhd2160 := transcodeTo(ReasonVideoCodec, model.Quality2160, 3840, 2160, 25_000_000, false, true, 1)
	uhd1080 := transcodeTo(ReasonVideoCodec, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1)
	for _, c := range []struct {
		name string
		p    *transcode.Probe
		req  Request
		want Decision
	}{
		{"qsv 2160p", uhd, pick(hostQSV, caps4K, model.PlaybackLimits{}, model.Quality2160), uhd2160},
		{"qsv auto never 2160p", uhd, pick(hostQSV, caps4K, model.PlaybackLimits{}, model.QualityAuto), uhd1080},
		{"nvenc 2160p", uhd, pick(hostNVENC, caps4K, model.PlaybackLimits{}, model.Quality2160), uhd2160},
		{"device of 1440 lines", uhd, pick(hostQSV, capsChrome, model.PlaybackLimits{}, model.Quality2160), uhd2160},
		{"profile allows 2160", uhd, pick(hostQSV, caps4K, model.PlaybackLimits{MaxHeight: 2160}, model.Quality2160), uhd2160},
		{"software backend", uhd, pick(hostSoftware, caps4K, model.PlaybackLimits{}, model.Quality2160), uhd1080},
		{"no transcode host", uhd, pick(transcode.Capabilities{}, caps4K, model.PlaybackLimits{}, model.Quality2160), uhd1080},
		{"device of 1080 lines", uhd, pick(hostQSV, hd, model.PlaybackLimits{}, model.Quality2160), uhd1080},
		{"phone", uhd, pick(hostQSV, phone, model.PlaybackLimits{}, model.Quality2160),
			transcodeTo(ReasonVideoCodec, model.Quality720, 1280, 720, 4_000_000, false, true, 1)},
		{"profile limit 1080", uhd, pick(hostQSV, caps4K, model.PlaybackLimits{MaxHeight: 1080}, model.Quality2160),
			transcodeTo(ReasonMaxHeight, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1)},
		{"bandwidth below the rung", uhd, pick(hostQSV, caps4K, model.PlaybackLimits{MaxBitrateKbps: 20_000}, model.Quality2160),
			transcodeTo(ReasonBandwidth, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1)},
		{"HDR with cpu tonemap", hdr, pick(hostQSV, caps4K, model.PlaybackLimits{}, model.Quality2160),
			transcodeTo(ReasonHDR, model.Quality1080, 1920, 1080, 8_000_000, true, true, 1)},
		{"HDR with gpu tonemap", hdr, pick(hostNVENC, caps4K, model.PlaybackLimits{}, model.Quality2160),
			transcodeTo(ReasonHDR, model.Quality2160, 3840, 2160, 25_000_000, true, true, 1)},
		{"scope 4K keeps 3840x1600", scope, pick(hostQSV, caps4K, model.PlaybackLimits{}, model.Quality2160),
			transcodeTo(ReasonVideoCodec, model.Quality2160, 3840, 1600, 25_000_000, false, true, 1)},
		{"2560x1440 is no 4K source", qhd, pick(hostQSV, caps4K, model.PlaybackLimits{}, model.Quality2160),
			transcodeTo(ReasonVideoCodec, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1)},
	} {
		if got := Decide(hevc, c.p, c.req); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}

	// A 4K file the client plays directly: 2160p is the rung of the source and stays direct, a
	// lower rung transcodes.
	mp4 := model.MediaFile{Container: "mp4", VideoCodec: "hevc", AudioCodec: "aac"}
	hevcCaps := capsChromeHEVC
	hevcCaps.MaxWidth, hevcCaps.MaxHeight = 3840, 2160
	if got, want := Decide(mp4, uhd, pick(hostQSV, hevcCaps, model.PlaybackLimits{}, model.Quality2160)), direct(3840, 2160, 40_000_000, 1); got != want {
		t.Errorf("2160p of a direct 4K file = %+v, want %+v", got, want)
	}
	if got, want := Decide(mp4, uhd, pick(hostQSV, hevcCaps, model.PlaybackLimits{}, model.Quality1080)),
		transcodeTo(ReasonQuality, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1); got != want {
		t.Errorf("1080p of a direct 4K file = %+v, want %+v", got, want)
	}

	// The panel offers 2160p where the host allows it.
	for _, c := range []struct {
		name  string
		host  transcode.Capabilities
		p     *transcode.Probe
		avail bool
	}{
		{"qsv", hostQSV, uhd, true}, {"qsv scope", hostQSV, scope, true}, {"software", hostSoftware, uhd, false},
		{"qsv HDR", hostQSV, hdr, false}, {"nvenc HDR", hostNVENC, hdr, true}, {"qsv 2560x1440", hostQSV, qhd, false},
	} {
		req := pick(c.host, caps4K, model.PlaybackLimits{}, model.QualityAuto)
		d := Decide(hevc, c.p, req)
		opts := QualityOptions(d, newSource(hevc, c.p, model.PlaybackLimits{}, caps4K, c.host), originalAvailable(d, hevc, c.p, req), "")
		o := option(t, opts, model.Quality2160)
		if o.Available != c.avail || o.Current || o.Bitrate != 25_000_000 || o.Codec != "h264" || o.Direct {
			t.Errorf("%s: 2160p option %+v, want available %v", c.name, o, c.avail)
		}
		if a := option(t, opts, model.QualityAuto); !a.Current || a.Height != 1080 {
			t.Errorf("%s: auto option %+v", c.name, a)
		}
	}
	// The rung of a source the client plays directly is direct with the source's codec and rate.
	req := pick(hostQSV, hevcCaps, model.PlaybackLimits{}, model.QualityAuto)
	d := Decide(mp4, uhd, req)
	opts := QualityOptions(d, newSource(mp4, uhd, model.PlaybackLimits{}, hevcCaps, hostQSV), originalAvailable(d, mp4, uhd, req), "")
	if o := option(t, opts, model.Quality2160); !o.Available || !o.Direct || o.Codec != "hevc" || o.Bitrate != 40_000_000 || o.Width != 3840 {
		t.Errorf("2160p option of a direct 4K file %+v", o)
	}
	if o := option(t, opts, model.Quality1080); o.Direct || o.Codec != "h264" || o.Bitrate != 8_000_000 {
		t.Errorf("1080p option of a direct 4K file %+v", o)
	}
	if a := option(t, opts, model.QualityAuto); !a.Current || !a.Direct || a.Codec != "hevc" {
		t.Errorf("auto option of a direct 4K file %+v", a)
	}
	// 2160p played directly on the QSV host is the source's rung and current.
	req = pick(hostQSV, hevcCaps, model.PlaybackLimits{}, model.Quality2160)
	d = Decide(mp4, uhd, req)
	opts = QualityOptions(d, newSource(mp4, uhd, model.PlaybackLimits{}, hevcCaps, hostQSV), originalAvailable(d, mp4, uhd, req), model.Quality2160)
	if o, a := option(t, opts, model.Quality2160), option(t, opts, model.QualityAuto); !o.Current || a.Current {
		t.Errorf("direct 2160p on qsv: 2160p %+v, auto %+v", o, a)
	}
}

// A remembered 2160p that the title cannot offer (final review stage 2, G2-M1): the file plays
// directly, the 2160p row is not available, so auto is current and no rung is.
func TestQualityOptionsUnavailable2160(t *testing.T) {
	t.Parallel()
	mp4 := model.MediaFile{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"}
	hd := sizedProbe("h264", "bt709", 1920, 1080, 6_000_000)
	hevc := model.MediaFile{Container: "mp4", VideoCodec: "hevc", AudioCodec: "aac"}
	uhd := sizedProbe("hevc", "bt709", 3840, 2160, 40_000_000)
	hevcCaps := capsChromeHEVC
	hevcCaps.MaxWidth, hevcCaps.MaxHeight = 3840, 2160
	for _, c := range []struct {
		name string
		file model.MediaFile
		p    *transcode.Probe
		caps model.PlaybackCapabilities
		host transcode.Capabilities
	}{
		{"1080p direct", mp4, hd, caps4K, hostQSV},
		{"4K direct on a software host", hevc, uhd, hevcCaps, hostSoftware},
	} {
		req := Request{Caps: c.caps, AudioStream: -1, Quality: model.Quality2160, Host: c.host}
		d := Decide(c.file, c.p, req)
		if d.Mode != ModeDirect {
			t.Fatalf("%s: decision %+v, want direct play", c.name, d)
		}
		opts := QualityOptions(d, newSource(c.file, c.p, model.PlaybackLimits{}, c.caps, c.host), originalAvailable(d, c.file, c.p, req), model.Quality2160)
		if o := option(t, opts, model.Quality2160); o.Available || o.Current {
			t.Errorf("%s: 2160p option %+v, want unavailable and not current", c.name, o)
		}
		if a := option(t, opts, model.QualityAuto); !a.Current {
			t.Errorf("%s: auto option %+v, want current", c.name, a)
		}
		for _, o := range opts {
			if o.Current && o.Quality != model.QualityAuto {
				t.Errorf("%s: %s is current too", c.name, o.Quality)
			}
		}
	}
}

// The budget (plan 3.5, ADR 0021) lowers 2160p to 1080p, also a source tonemapped on the GPU;
// with the host in the target the 2160p transcode costs 5 and the 1080p one 3, without the host
// both would be planned at 1080 lines.
func TestLowered2160AndCost(t *testing.T) {
	t.Parallel()
	hevc := model.MediaFile{Container: "mkv", VideoCodec: "hevc", AudioCodec: "aac"}
	uhd := sizedProbe("hevc", "bt709", 3840, 2160, 40_000_000)
	uhd.Format.Duration = 3_600_000_000_000
	d := Decide(hevc, uhd, Request{Caps: caps4K, AudioStream: -1, Quality: model.Quality2160, Host: hostQSV})
	low, ok := lowered(d, 3840, 2160, model.PlaybackLimits{})
	if want := transcodeTo(ReasonBudgetLowered, model.Quality1080, 1920, 1080, 8_000_000, false, true, 1); !ok || low != want {
		t.Errorf("lowered 2160p = %+v %v, want %+v", low, ok, want)
	}
	hdr := transcodeTo(ReasonHDR, model.Quality2160, 3840, 2160, 25_000_000, true, true, 1)
	if low, ok := lowered(hdr, 3840, 2160, model.PlaybackLimits{}); !ok || low.Quality != model.Quality1080 || !low.Tonemap || low.Reason != ReasonBudgetLowered {
		t.Errorf("lowered 2160p HDR = %+v %v", low, ok)
	}

	req := HLSRequest{Probe: *uhd, Decision: d}
	target := hlsTarget(req, hostQSV)
	if target.Backend != transcode.BackendQSV || target.HWTonemap || target.MaxHeight != 2160 || target.Quality != model.Quality2160 {
		t.Errorf("2160p target = %+v", target)
	}
	if got := transcode.Cost(hlsSource(req), target); got != 5 {
		t.Errorf("cost of 2160p on qsv = %v, want 5", got)
	}
	if got := transcode.Cost(hlsSource(req), hlsTarget(req, transcode.Capabilities{})); got != 3 {
		t.Errorf("cost of 2160p without a host = %v, want 3 (planned at 1080 lines)", got)
	}
	if !cheaper(req, low, hostQSV) {
		t.Error("1080p is not cheaper than 2160p on qsv")
	}
	if cheaper(req, low, transcode.Capabilities{}) {
		t.Error("without a host 2160p and 1080p would cost the same")
	}
	if nv := hlsTarget(HLSRequest{Probe: *uhd, Decision: hdr}, hostNVENC); nv.Backend != transcode.BackendNVENC || !nv.HWTonemap {
		t.Errorf("nvenc target = %+v", nv)
	}
	plan, err := transcode.NewPlan(hlsSource(req), target)
	if err != nil || plan.Height != 2160 || plan.Width != 3840 || plan.Cost != 5 {
		t.Errorf("2160p plan = %+v, %v", plan, err)
	}
}

// The service end to end with a QSV host: 2160p plays on the 2160p rung and is current; an
// exhausted budget lowers it to 1080p; a remembered cap keeps a file within it direct and caps one
// above it, with auto current.
func TestStart2160AndCap(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	tc := newFakeTranscoder()
	tc.backend = string(transcode.BackendQSV)
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithTokenKey(testKey))
	ctx := context.Background()
	m, hd := f.Rated(6), f.Rated(0)
	f.SetMediaFile(t, m.FileID, "mkv", "hevc", "aac", trackProbe(t, "hevc", "bt709", 3840, 2160, 40_000_000), testKeyframes)
	f.SetMediaFile(t, hd.FileID, "mp4", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000), nil)
	caps := capsChromeLinux
	session := browserSession(t, f, f.P18)

	info, err := s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &caps, Quality: model.Quality2160})
	if err != nil {
		t.Fatal(err)
	}
	last := tc.starts()[len(tc.starts())-1]
	if tr := info.Transcoding; tr == nil || tr.Quality != model.Quality2160 || tr.Height != 2160 || tr.Backend != "qsv" || last.Decision.MaxHeight != 2160 {
		t.Errorf("2160p = %+v, decision %+v", info.Transcoding, last.Decision)
	}
	if o := option(t, info.Qualities, model.Quality2160); !o.Available || !o.Current {
		t.Errorf("2160p option %+v", o)
	}
	if o := option(t, info.Qualities, model.QualityOriginal); o.Available || o.Current {
		t.Errorf("original of HEVC in Chrome %+v", o)
	}
	var quality string
	if err := f.DB.Read.QueryRow(`SELECT quality FROM playback_sessions WHERE id = ?`, info.PlaybackSessionID).Scan(&quality); err != nil || quality != model.Quality2160 {
		t.Errorf("stored quality %q, %v", quality, err)
	}

	tc.mu.Lock()
	tc.busyAbove = 1080
	tc.mu.Unlock()
	info, err = s.Start(ctx, session, m.ID, model.PlaybackRequest{Capabilities: &caps, Quality: model.Quality2160})
	if err != nil {
		t.Fatal(err)
	}
	if tr := info.Transcoding; tr == nil || tr.Quality != model.Quality1080 || tr.Reason != ReasonBudgetLowered {
		t.Errorf("lowered 2160p = %+v", info.Transcoding)
	}
	if o := option(t, info.Qualities, model.Quality1080); !o.Current {
		t.Errorf("1080p option after lowering %+v", o)
	}

	// V-17: within the cap the MP4 plays directly, above it the cap applies.
	info, err = s.Start(ctx, session, hd.ID, model.PlaybackRequest{Capabilities: &caps, QualityCap: model.Quality1080})
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode != ModeDirect || !option(t, info.Qualities, model.QualityAuto).Current {
		t.Errorf("cap 1080p = %s %+v", info.Mode, info.Transcoding)
	}
	info, err = s.Start(ctx, session, hd.ID, model.PlaybackRequest{Capabilities: &caps, QualityCap: model.Quality720})
	if err != nil {
		t.Fatal(err)
	}
	if tr := info.Transcoding; tr == nil || tr.Quality != model.Quality720 || tr.Reason != ReasonQuality ||
		!option(t, info.Qualities, model.QualityAuto).Current || option(t, info.Qualities, model.Quality720).Current {
		t.Errorf("cap 720p = %+v %+v", info.Transcoding, info.Qualities)
	}
}

// Concept 13 11.5 (a), (c), (d) in the service: an original remux without a keyframe index plays
// as auto (a transcode, keyframes_pending) with original unavailable and auto current; with the
// index it is an original remux, stored as original, current in the panel and rebuilt as an
// original remux.
func TestStartOriginal(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	tc := newFakeTranscoder()
	s := New(f.DB, vistest.Logger(), WithTranscoder(tc), WithTokenKey(testKey))
	ctx := context.Background()
	pending, ready := f.Rated(0), f.Rated(6)
	f.SetMediaFile(t, pending.FileID, "mkv", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000), nil)
	f.SetMediaFile(t, ready.FileID, "mkv", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000), testKeyframes)
	caps := capsChromeLinux
	session := browserSession(t, f, f.P18)
	stored := func(id string) string {
		t.Helper()
		var q string
		if err := f.DB.Read.QueryRow(`SELECT quality FROM playback_sessions WHERE id = ?`, id).Scan(&q); err != nil {
			t.Fatal(err)
		}
		return q
	}

	info, err := s.Start(ctx, session, pending.ID, model.PlaybackRequest{Capabilities: &caps, Quality: model.QualityOriginal})
	if err != nil {
		t.Fatal(err)
	}
	if tr := info.Transcoding; tr == nil || tr.Mode != ModeTranscode || tr.Reason != ReasonKeyframesPending || tr.Quality != model.Quality1080 {
		t.Errorf("(a) original without index = %+v", info.Transcoding)
	}
	if o, a := option(t, info.Qualities, model.QualityOriginal), option(t, info.Qualities, model.QualityAuto); o.Available || o.Current || !a.Current {
		t.Errorf("(a)/(c) options: original %+v, auto %+v", o, a)
	}
	if q := stored(info.PlaybackSessionID); q != model.Quality1080 {
		t.Errorf("(a) stored quality %q", q)
	}

	info, err = s.Start(ctx, session, ready.ID, model.PlaybackRequest{Capabilities: &caps, Quality: model.QualityOriginal})
	if err != nil {
		t.Fatal(err)
	}
	if tr := info.Transcoding; tr == nil || tr.Mode != ModeRemux || tr.Quality != model.QualityOriginal {
		t.Errorf("(d) original remux = %+v", info.Transcoding)
	}
	if o, a := option(t, info.Qualities, model.QualityOriginal), option(t, info.Qualities, model.QualityAuto); !o.Available || !o.Current || !o.Direct || a.Current {
		t.Errorf("(d) options: original %+v, auto %+v", o, a)
	}
	if q := stored(info.PlaybackSessionID); q != model.QualityOriginal {
		t.Errorf("(d) stored quality %q", q)
	}
	g, err := s.Authorize(ctx, tokenOf(t, info.StreamURL))
	if err != nil {
		t.Fatal(err)
	}
	tc.lose(info.PlaybackSessionID)
	if _, err := s.HLS(ctx, g, 2); err != nil {
		t.Fatal(err)
	}
	if last := tc.starts()[len(tc.starts())-1]; last.ID != info.PlaybackSessionID || last.Decision.Mode != ModeRemux ||
		last.Decision.Quality != model.QualityOriginal || last.StartSegment != 2 {
		t.Errorf("(d) rebuilt %+v", last)
	}
}

// ManagerTranscoder takes the host from its manager: Backend, Capabilities and the targets agree
// with the manager's capabilities.
func TestManagerTranscoderHost(t *testing.T) {
	t.Parallel()
	m, err := transcode.NewManager(transcode.ManagerOptions{
		Runner: transcode.Runner{FFmpegPath: "ffmpeg", FFprobePath: "ffprobe"}, WorkDir: t.TempDir() + "/transcode",
		Backend: transcode.BackendQSV, Caps: transcode.Capabilities{Tonemapper: transcode.TonemapVPP, HWTonemap: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	tc := NewManagerTranscoder(m)
	if host, mc := tc.Capabilities(), m.Capabilities(); tc.Backend() != "qsv" || host.Backend != mc.Backend || host.HWTonemap != mc.HWTonemap || !host.HWTonemap {
		t.Errorf("host = %q %+v, manager %+v", tc.Backend(), tc.Capabilities(), m.Capabilities())
	}
	target := hlsTarget(HLSRequest{Decision: transcodeTo(ReasonHDR, model.Quality2160, 3840, 2160, 25_000_000, true, true, 1)}, tc.Capabilities())
	if target.Backend != transcode.BackendQSV || !target.HWTonemap {
		t.Errorf("target = %+v", target)
	}
}
