package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// HTTP tests of M4 (plan 7 rows K-5, W-6, W-7, W-8; design sections 2, 6, 8).

type stepClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *stepClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type m4Problem struct {
	Code       string         `json:"code"`
	Extensions map[string]int `json:"extensions"`
	Errors     []struct {
		Field string `json:"field"`
		Code  string `json:"code"`
	} `json:"errors"`
}

type m4Info struct {
	Mode              string `json:"mode"`
	StreamURL         string `json:"streamUrl"`
	SubtitleURL       string `json:"subtitleUrl"`
	PlaybackSessionID string `json:"playbackSessionId"`
	Transcoding       *struct {
		Mode     string `json:"mode"`
		Quality  string `json:"quality"`
		Height   int    `json:"height"`
		Bitrate  int    `json:"bitrate"`
		Reason   string `json:"reason"`
		Hardware bool   `json:"hardware"`
	} `json:"transcoding"`
	WatchTime *struct {
		LimitMinutes     int `json:"limitMinutes"`
		UsedMinutes      int `json:"usedMinutes"`
		RemainingMinutes int `json:"remainingMinutes"`
	} `json:"watchTime"`
	Device *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"device"`
	Selection struct {
		AudioTrack       *int   `json:"audioTrack"`
		SubtitleTrack    *int64 `json:"subtitleTrack"`
		SubtitleDelivery string `json:"subtitleDelivery"`
	} `json:"selection"`
	Qualities []struct {
		Quality   string `json:"quality"`
		Available bool   `json:"available"`
		Current   bool   `json:"current"`
	} `json:"qualities"`
	Segments struct {
		Intro *struct {
			StartMs int64 `json:"startMs"`
		} `json:"intro"`
	} `json:"segments"`
	NextEpisode *struct {
		ID string `json:"id"`
	} `json:"nextEpisode"`
}

func play(t *testing.T, h http.Handler, token, itemID, body string) (m4Info, int, m4Problem) {
	t.Helper()
	rec := send(t, h, http.MethodPost, BasePath+"/items/"+itemID+"/play", token, body, nil)
	var info m4Info
	var p m4Problem
	if rec.Code == http.StatusOK {
		decode(t, rec, &info)
	} else {
		decode(t, rec, &p)
	}
	return info, rec.Code, p
}

// streamStatus requests a stream URL without the cookie (a Range of the first bytes).
func streamStatus(t *testing.T, h http.Handler, streamURL string) int {
	t.Helper()
	return send(t, h, http.MethodGet, streamURL, "", "", map[string]string{"Range": "bytes=0-9"}).Code
}

const chromeCaps = `{"containers":["mp4","webm"],"video":["h264","vp9"],"audio":["aac","opus"],"hls":"mse",` +
	`"maxWidth":2560,"maxHeight":1440,"hdr":false,"name":"Chrome 156 · Linux","type":"desktop"}`

// K-5 with a clock mock: 30-minute limit, progress events of 10 s each; the event that reaches the
// limit answers 0 minutes and the stream answers 401 at once; the next start answers 403
// watch_limit_reached with the numbers; the report shows the minutes of today; only the primary
// profile reads it.
func TestHTTPWatchLimit(t *testing.T) {
	pf := newProfileFixture(t)
	f := pf.f
	clock := &stepClock{now: time.Date(2026, 10, 20, 17, 0, 0, 0, time.Local)}
	pb := playback.New(f.DB, vistest.Logger(), playback.WithTokenKey(testTokenKey), playback.WithTranscoder(pf.tc), playback.WithClock(clock.Now))
	h := pf.handlerWithPlayback(pb)
	kid := f.AddLimitedProfile(t, "Kind", model.PlaybackLimits{WatchLimitMinutes: 30})
	pf.newSession(t, "kid", kid)
	m := f.Rated(6)

	info, code, p := play(t, h, "kid", m.ID, `{}`)
	if code != http.StatusOK || info.WatchTime == nil || info.WatchTime.LimitMinutes != 30 || info.WatchTime.RemainingMinutes != 30 {
		t.Fatalf("start = %d %+v %+v", code, info.WatchTime, p)
	}
	stopped := 0
	for i := 1; i <= 200 && stopped == 0; i++ {
		clock.advance(10 * time.Second)
		body := fmt.Sprintf(`{"playbackSessionId":%q,"positionMs":%d,"durationMs":%d,"playedMs":10000,"seq":%d,"state":"playing","clientTime":%q}`,
			info.PlaybackSessionID, i*10_000, vistest.DurationMs, i, clock.Now().UTC().Format(time.RFC3339))
		rec := send(t, h, http.MethodPost, BasePath+"/items/"+m.ID+"/progress", "kid", body, nil)
		var prog struct {
			WatchTime *struct {
				RemainingMinutes int `json:"remainingMinutes"`
				UsedMinutes      int `json:"usedMinutes"`
			} `json:"watchTime"`
		}
		decode(t, rec, &prog)
		if rec.Code != http.StatusOK || prog.WatchTime == nil {
			t.Fatalf("event %d: %d %s", i, rec.Code, rec.Body.String())
		}
		st := streamStatus(t, h, info.StreamURL)
		if prog.WatchTime.RemainingMinutes == 0 {
			stopped = i
			if st != http.StatusUnauthorized || prog.WatchTime.UsedMinutes != 30 {
				t.Errorf("at the limit: stream %d, watch time %+v", st, prog.WatchTime)
			}
		} else if st != http.StatusPartialContent {
			t.Fatalf("event %d before the limit: stream %d", i, st)
		}
	}
	if stopped != 180 {
		t.Fatalf("stopped after event %d, want 180", stopped)
	}
	_, code, p = play(t, h, "kid", m.ID, `{}`)
	if code != http.StatusForbidden || p.Code != "watch_limit_reached" || p.Extensions["limitMinutes"] != 30 || p.Extensions["usedMinutes"] != 30 {
		t.Errorf("start after the limit = %d %+v", code, p)
	}

	rec := send(t, h, http.MethodGet, BasePath+fmt.Sprintf("/profiles/%d/watch-time?days=2", kid.ID), "p18", "", nil)
	var rep struct {
		ProfileID    int64 `json:"profileId"`
		LimitMinutes int   `json:"limitMinutes"`
		Days         []struct {
			Date    string `json:"date"`
			Minutes int    `json:"minutes"`
		} `json:"days"`
	}
	decode(t, rec, &rep)
	if rec.Code != http.StatusOK || rep.ProfileID != kid.ID || rep.LimitMinutes != 30 || len(rep.Days) != 2 ||
		rep.Days[1].Date != "2026-10-20" || rep.Days[1].Minutes != 30 || rep.Days[0].Minutes != 0 {
		t.Errorf("report = %d %s", rec.Code, rec.Body.String())
	}
	// The raw events and the report agree (K-5: deviation <= 1 min).
	var playedMs int64
	if err := f.DB.Read.QueryRow(`SELECT sum(played_ms) FROM progress_events WHERE profile_id = ?`, kid.ID).Scan(&playedMs); err != nil {
		t.Fatal(err)
	}
	if diff := playedMs/60_000 - int64(rep.Days[1].Minutes); diff < -1 || diff > 1 {
		t.Errorf("raw events %d ms, report %d min", playedMs, rep.Days[1].Minutes)
	}
	rec = send(t, h, http.MethodGet, BasePath+fmt.Sprintf("/profiles/%d/watch-time", kid.ID), "p6", "", nil)
	if rec.Code != http.StatusForbidden || problemCode(t, rec) != "primary_profile_required" {
		t.Errorf("non-primary profile: %d %s", rec.Code, rec.Body.String())
	}
	rec = send(t, h, http.MethodGet, BasePath+fmt.Sprintf("/profiles/%d/watch-time", f.PG.ID), "p18", "", nil)
	if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
		t.Errorf("profile of another account: %d %s", rec.Code, rec.Body.String())
	}
	rec = send(t, h, http.MethodGet, BasePath+fmt.Sprintf("/profiles/%d/watch-time?days=91", kid.ID), "p18", "", nil)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("91 days: %d %s", rec.Code, rec.Body.String())
	}
}

// W-8: max 720p → output 720p; one stream → the second start 409 with the numbers; 4 Mbit/s →
// bit rate <= 4 Mbit/s; a manual 480p → transcode on 480p.
func TestHTTPPlaybackLimits(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	m6, m0 := f.Rated(6), f.Rated(0)
	kf := library.EncodeKeyframes([]time.Duration{0, 4 * time.Second})
	probe := `{"format":{"name":"mov,mp4","duration":7200000000000,"bitRate":10000000},"streams":[` +
		`{"index":0,"type":"video","codec":"h264","width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709"},` +
		`{"index":1,"type":"audio","codec":"aac","channels":2,"default":true}]}`
	for _, m := range []vistest.Movie{m6, m0} {
		f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", probe, kf)
	}
	for name, limits := range map[string]model.PlaybackLimits{
		"max720": {MaxHeight: 720}, "mbit4": {MaxBitrateKbps: 4000}, "one": {MaxStreams: 1},
	} {
		p := f.AddLimitedProfile(t, name, limits)
		pf.newSession(t, name, p)
		if name == "one" {
			pf.newSession(t, "one-b", p)
		}
	}
	body := `{"capabilities":` + chromeCaps + `}`
	if info, code, p := play(t, h, "max720", m6.ID, body); code != http.StatusOK || info.Transcoding == nil || info.Transcoding.Height != 720 ||
		info.Transcoding.Quality != "720p" || info.Transcoding.Reason != "max_height" {
		t.Errorf("max 720p = %d %+v %+v", code, info.Transcoding, p)
	}
	if info, code, _ := play(t, h, "mbit4", m6.ID, body); code != http.StatusOK || info.Transcoding == nil || info.Transcoding.Bitrate > 4_000_000 ||
		info.Transcoding.Reason != "bandwidth" {
		t.Errorf("4 Mbit/s = %d %+v", code, info.Transcoding)
	}
	info, code, _ := play(t, h, "p18", m6.ID, `{"quality":"480p","capabilities":`+chromeCaps+`}`)
	if code != http.StatusOK || info.Transcoding == nil || info.Transcoding.Quality != "480p" || info.Transcoding.Height != 480 || info.Transcoding.Reason != "quality" {
		t.Errorf("manual 480p = %d %+v", code, info.Transcoding)
	}
	for _, q := range info.Qualities {
		if q.Current != (q.Quality == "480p") || !q.Available {
			t.Errorf("quality option %+v", q)
		}
	}
	if _, code, _ := play(t, h, "one", m6.ID, `{}`); code != http.StatusOK {
		t.Fatalf("first stream: %d", code)
	}
	_, code, p := play(t, h, "one-b", m0.ID, `{}`)
	if code != http.StatusConflict || p.Code != "stream_limit_reached" || p.Extensions["maxStreams"] != 1 || p.Extensions["activeStreams"] != 1 {
		t.Errorf("second stream = %d %+v", code, p)
	}
	// Validation of the wanted tracks (422 field codes).
	_, code, p = play(t, h, "p18", m6.ID, `{"audioTrack":9}`)
	if code != http.StatusUnprocessableEntity || len(p.Errors) != 1 || p.Errors[0].Field != "audioTrack" || p.Errors[0].Code != "unknown_track" {
		t.Errorf("unknown audio track = %d %+v", code, p)
	}
	_, code, p = play(t, h, "p18", m6.ID, fmt.Sprintf(`{"subtitleTrack":%d,"subtitleDelivery":"text"}`, pf.tracks[m6.FileID].Bitmap))
	if code != http.StatusUnprocessableEntity || len(p.Errors) != 1 || p.Errors[0].Code != "not_text" {
		t.Errorf("text delivery of a bitmap track = %d %+v", code, p)
	}
}

// W-8 guest cap over HTTP: a guest with maxStreams 1 gets 409 for a stream of a second profile; a
// switch of the same browser session and title replaces the playback (its token answers 401, its
// own stop 404) instead of counting as a second stream.
func TestHTTPGuestStreamCapAndSwitch(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	if _, err := f.DB.Write.Exec(`UPDATE accounts SET max_streams = 1 WHERE id = ?`, f.Guest.ID); err != nil {
		t.Fatal(err)
	}
	m0, m6 := f.Rated(0), f.Rated(6)
	first, code, p := play(t, h, "pg", m0.ID, `{}`)
	if code != http.StatusOK {
		t.Fatalf("first stream = %d %+v", code, p)
	}
	_, code, p = play(t, h, "pg18", m6.ID, `{}`)
	if code != http.StatusConflict || p.Code != "stream_limit_reached" || p.Extensions["maxStreams"] != 1 || p.Extensions["activeStreams"] != 1 {
		t.Errorf("second profile of the guest = %d %+v", code, p)
	}
	switched, code, p := play(t, h, "pg", m0.ID, `{"quality":"480p"}`)
	if code != http.StatusOK {
		t.Fatalf("switch = %d %+v", code, p)
	}
	if st := streamStatus(t, h, first.StreamURL); st != http.StatusUnauthorized {
		t.Errorf("replaced stream = %d", st)
	}
	if st := streamStatus(t, h, switched.StreamURL); st != http.StatusPartialContent {
		t.Errorf("new stream = %d", st)
	}
	rec := send(t, h, http.MethodDelete, BasePath+"/playback/sessions/"+first.PlaybackSessionID, "pg", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("stop of the replaced playback = %d %s", rec.Code, rec.Body.String())
	}
}

// W-4 over HTTP: the subtitle URL of a text track works without the cookie; another track of the
// playback answers 404, a forged token 401; the selection names the track.
func TestHTTPSubtitles(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	m := f.Rated(6)
	ft := pf.tracks[m.FileID]
	info, code, p := play(t, h, "p6", m.ID, fmt.Sprintf(`{"subtitleTrack":%d}`, ft.Sidecar))
	if code != http.StatusOK || info.SubtitleURL == "" || info.Selection.SubtitleTrack == nil || *info.Selection.SubtitleTrack != ft.Sidecar ||
		info.Selection.SubtitleDelivery != "text" {
		t.Fatalf("start = %d %+v %+v", code, info, p)
	}
	rec := send(t, h, http.MethodGet, info.SubtitleURL, "", "", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "WEBVTT\n\n"+vistest.SidecarContent || rec.Header().Get("Content-Type") != "text/vtt; charset=utf-8" {
		t.Errorf("subtitle = %d %q %v", rec.Code, rec.Body.String(), rec.Header())
	}
	other := strings.Replace(info.SubtitleURL, fmt.Sprintf("/subtitles/%d.vtt", ft.Sidecar), fmt.Sprintf("/subtitles/%d.vtt", ft.Text), 1)
	if rec := send(t, h, http.MethodGet, other, "", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("another track: %d", rec.Code)
	}
	forged := BasePath + "/playback/sessions/" + info.PlaybackSessionID + fmt.Sprintf("/subtitles/%d.vtt?st=", ft.Sidecar) +
		url.QueryEscape(playback.SignToken([]byte("another key, another key, another"), info.PlaybackSessionID, m.ID))
	if rec := send(t, h, http.MethodGet, forged, "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("forged token: %d", rec.Code)
	}
	noExt := strings.Replace(info.SubtitleURL, ".vtt?", "?", 1)
	if rec := send(t, h, http.MethodGet, noExt, "", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("without .vtt: %d", rec.Code)
	}
	// An episode with an intro and a next episode.
	info, code, _ = play(t, h, "p18", f.Show.E1.ID, `{}`)
	if code != http.StatusOK || info.Segments.Intro == nil || info.Segments.Intro.StartMs != 5_000 || info.NextEpisode == nil || info.NextEpisode.ID != f.Show.E2.ID {
		t.Errorf("episode = %d %+v %+v", code, info.Segments, info.NextEpisode)
	}
}

// W-6 and W-7: two streams in the overview with profile, device, title, quality and mode; the
// administrator's stop answers 401 for the stream and stops ffmpeg; the device carries the
// reported capabilities, its owner and the last contact; forgetting it removes it.
func TestHTTPAdminStreamsAndDevices(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	hevc := f.Show.E3
	f.SetMediaFile(t, hevc.FileID, "mkv", "hevc", "aac",
		`{"format":{"name":"matroska,webm","duration":7200000000000},"streams":[{"index":0,"type":"video","codec":"hevc","width":1920,"height":1080,"pixFmt":"yuv420p10le"},{"index":1,"type":"audio","codec":"aac","default":true}]}`, nil)
	a, code, _ := play(t, h, "p18", hevc.ID, `{"capabilities":`+chromeCaps+`}`)
	if code != http.StatusOK || a.Transcoding == nil || a.Device == nil || a.Device.Name != "Chrome 156 · Linux" {
		t.Fatalf("stream A = %d %+v %+v", code, a.Transcoding, a.Device)
	}
	b, code, _ := play(t, h, "p6", f.Rated(6).ID, `{}`)
	if code != http.StatusOK || b.Mode != "direct" {
		t.Fatalf("stream B = %d %+v", code, b)
	}

	rec := send(t, h, http.MethodGet, BasePath+"/admin/streams", "p18", "", nil)
	var streams []struct {
		ID           string `json:"id"`
		AccountEmail string `json:"accountEmail"`
		ProfileName  string `json:"profileName"`
		Device       *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"device"`
		ItemID     string `json:"itemId"`
		ItemTitle  string `json:"itemTitle"`
		Mode       string `json:"mode"`
		Quality    string `json:"quality"`
		Hardware   bool   `json:"hardware"`
		Backend    string `json:"backend"`
		Height     int    `json:"height"`
		DurationMs int64  `json:"durationMs"`
	}
	decode(t, rec, &streams)
	if rec.Code != http.StatusOK || len(streams) != 2 || streams[0].ID != b.PlaybackSessionID || streams[1].ID != a.PlaybackSessionID {
		t.Fatalf("streams = %d %s", rec.Code, rec.Body.String())
	}
	sa, sb := streams[1], streams[0]
	if sa.ProfileName != "P18" || sa.AccountEmail != f.Account.Email || sa.Device == nil || sa.Device.Name != "Chrome 156 · Linux" ||
		sa.ItemTitle != "Film SS · S01E03-E04 · Film E3" || sa.Mode != "transcode" || sa.Quality != "1080p" || sa.Hardware || sa.Backend != "software" ||
		sa.Height != 1080 || sa.DurationMs != vistest.DurationMs {
		t.Errorf("stream A = %+v", sa)
	}
	if sb.ProfileName != "P6" || sb.ItemTitle != "Film 06" || sb.Mode != "direct" || sb.Device == nil {
		t.Errorf("stream B = %+v", sb)
	}

	rec = send(t, h, http.MethodDelete, BasePath+"/admin/streams/"+b.PlaybackSessionID, "p18", "", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("stop: %d %s", rec.Code, rec.Body.String())
	}
	if st := streamStatus(t, h, b.StreamURL); st != http.StatusUnauthorized {
		t.Errorf("stream after the administrator's stop: %d", st)
	}
	if rec := send(t, h, http.MethodDelete, BasePath+"/admin/streams/"+a.PlaybackSessionID, "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("stop A: %d", rec.Code)
	}
	if stops := pf.tc.stops(); len(stops) == 0 || stops[len(stops)-1] != a.PlaybackSessionID {
		t.Errorf("ffmpeg of A not stopped: %v", stops)
	}
	if rec := send(t, h, http.MethodDelete, BasePath+"/admin/streams/"+a.PlaybackSessionID, "p18", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("second stop: %d", rec.Code)
	}

	rec = send(t, h, http.MethodGet, BasePath+"/admin/devices", "p18", "", nil)
	var devices []struct {
		ID           string    `json:"id"`
		Name         string    `json:"name"`
		Type         string    `json:"type"`
		OwnerEmail   string    `json:"ownerEmail"`
		OwnerProfile string    `json:"ownerProfile"`
		LastSeenAt   time.Time `json:"lastSeenAt"`
		Capabilities *struct {
			Containers []string `json:"containers"`
			Video      []string `json:"video"`
			Audio      []string `json:"audio"`
			MaxHeight  int      `json:"maxHeight"`
			HDR        bool     `json:"hdr"`
		} `json:"capabilities"`
		ActiveStreams int `json:"activeStreams"`
	}
	decode(t, rec, &devices)
	var found bool
	for _, d := range devices {
		if d.ID != a.Device.ID {
			continue
		}
		found = true
		if d.Name != "Chrome 156 · Linux" || d.Type != "desktop" || d.OwnerEmail != f.Account.Email || d.OwnerProfile != "P18" ||
			d.Capabilities == nil || d.Capabilities.MaxHeight != 1440 || len(d.Capabilities.Containers) != 2 || d.Capabilities.HDR ||
			time.Since(d.LastSeenAt) > time.Minute || d.ActiveStreams != 0 {
			t.Errorf("device = %+v", d)
		}
	}
	if !found {
		t.Fatalf("device %s missing: %s", a.Device.ID, rec.Body.String())
	}
	if rec := send(t, h, http.MethodDelete, BasePath+"/admin/devices/"+a.Device.ID, "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("forget the device: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/admin/devices", "p18", "", nil); strings.Contains(rec.Body.String(), a.Device.ID) {
		t.Errorf("forgotten device still listed")
	}
}
