package offline

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// subtitlesClip is the 10-s 1080p H.264/AAC clip with SubRip and ASS tracks of
// testdata/generate-media.sh (task testdata:media).
var subtitlesClip = filepath.Join("..", "..", "..", "testdata", "media", "Movies", "Slouchcat Test Subtitles (2024)",
	"Slouchcat Test Subtitles (2024).mkv")

// The preparation end to end with the real encoder (transcode.Runner.Download of M5 stream A):
// a remux in high and a constant-rate encode in low, each a playable fragmented MP4 whose index
// matches the file, with the WebVTT sidecars; the encode lands within ±10 % of its estimate (O-1).
func TestPrepareWithRealEncoder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	runner := transcode.Runner{FFmpegPath: "ffmpeg", FFprobePath: "ffprobe"}
	if _, err := runner.Version(ctx); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}
	clip, err := os.ReadFile(subtitlesClip)
	if err != nil {
		t.Skipf("generated test media missing (task testdata:media): %v", err)
	}
	e := newEnv(t, WithEncoder(runner), WithAdmission(nil))
	m := e.f.Rated(6)
	path := filepath.Join(e.f.MediaDir, filepath.FromSlash(m.RelPath))
	if err := os.WriteFile(path, clip, 0o600); err != nil {
		t.Fatal(err)
	}
	probe, err := runner.Probe(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	e.f.SetMediaFile(t, m.FileID, "mkv", "h264", "aac", probeJSON(t, probe), nil)

	for _, tc := range []struct {
		quality string
		device  string
		mode    string
	}{{model.DownloadHigh, e.devA, "remux"}, {model.DownloadLow, e.devB, "transcode"}} {
		t.Run(tc.quality, func(t *testing.T) {
			st := e.create(e.f.P12, tc.device, m.ID, tc.quality)
			e.runJobs()
			r := e.row(st.ID)
			if r.state != model.DownloadReady {
				var lastErr string
				_ = e.f.DB.Read.QueryRow(`SELECT COALESCE(last_error, '') FROM jobs WHERE idempotency_key = ?`, primaryKey(st.ID)).Scan(&lastErr)
				t.Fatalf("row %+v, last job error %q", r, lastErr)
			}
			man, err := e.svc.Manifest(ctx, e.session(e.f.P12, tc.device), st.ID)
			if err != nil {
				t.Fatal(err)
			}
			var mode string
			if err := e.f.DB.Read.QueryRow(`SELECT mode FROM downloads WHERE id = ?`, st.ID).Scan(&mode); err != nil || mode != tc.mode {
				t.Errorf("mode = %q, want %s", mode, tc.mode)
			}
			end := man.InitLength
			var total int64
			for i, s := range man.Segments {
				if s.Offset != end || s.Length <= 0 {
					t.Fatalf("segment %d = %+v, want offset %d", i, s, end)
				}
				end += s.Length
				total += s.DurationMs
			}
			info, err := os.Stat(filepath.Join(e.dir, st.ID, mediaFile))
			if err != nil || info.Size() != end || man.SizeBytes != end || r.size != end {
				t.Fatalf("file %v (%v), index end %d, manifest %d, row %d", info.Size(), err, end, man.SizeBytes, r.size)
			}
			if total < 9_900 || total > 10_100 {
				t.Errorf("segments cover %d ms, want about 10 s", total)
			}
			out, err := runner.Probe(ctx, filepath.Join(e.dir, st.ID, mediaFile))
			if err != nil {
				t.Fatalf("the prepared file does not probe: %v", err)
			}
			var video, audio string
			for _, s := range out.Streams {
				switch s.Type {
				case "video":
					video = s.Codec
				case "audio":
					audio += s.Codec
				}
			}
			if video != "h264" || audio != "aac" || !strings.HasPrefix(man.Codecs, "avc1.") {
				t.Errorf("prepared streams video %q audio %q, codecs %q", video, audio, man.Codecs)
			}
			if tc.mode == "transcode" {
				if man.Height != 480 || float64(end) < 0.9*float64(st.EstimatedBytes) || float64(end) > 1.1*float64(st.EstimatedBytes) {
					t.Errorf("encode: %dx%d, %d bytes against the estimate %d", man.Width, man.Height, end, st.EstimatedBytes)
				}
				t.Logf("low: %d bytes, estimate %d (%+.1f %%)", end, st.EstimatedBytes, 100*(float64(end)/float64(st.EstimatedBytes)-1))
			}
			// The embedded SubRip track and the sidecar file as WebVTT.
			ft := e.tracks[m.FileID]
			for _, id := range []int64{ft.Text, ft.Sidecar} {
				vtt := string(e.file(st.ID, "subs/"+strconv.FormatInt(id, 10)+".vtt"))
				if !strings.HasPrefix(vtt, "WEBVTT") {
					t.Errorf("subtitle %d = %q", id, vtt)
				}
			}
			if len(man.Subtitles) != 2 {
				t.Errorf("manifest subtitles = %+v", man.Subtitles)
			}
			if _, err := os.Stat(filepath.Join(e.dir, st.ID, "encode")); err == nil {
				t.Error("the encoder's working directory remains")
			}
		})
	}
}

// testEpisode is an 8-s MP3 episode of testdata/generate-media.sh (task testdata:media).
var testEpisode = filepath.Join("..", "..", "..", "testdata", "media", "Podcasts", "episode-1.mp3")

// M6, O-1: a podcast episode without file goes from an enclosure on an HTTP server through the real
// encoder into a fragmented MP4 with AAC in each quality, within ±10 % of the estimate for its
// duration; the fetched source is gone afterwards.
func TestEpisodeDownloadWithRealEncoder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	runner := transcode.Runner{FFmpegPath: "ffmpeg", FFprobePath: "ffprobe"}
	if _, err := runner.Version(ctx); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}
	mp3, err := os.ReadFile(testEpisode)
	if err != nil {
		t.Skipf("generated test media missing (task testdata:media): %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		http.ServeContent(w, r, "", time.Unix(1, 0), bytes.NewReader(mp3))
	}))
	defer srv.Close()
	e := newEnv(t, WithEncoder(runner), WithAdmission(nil))
	e.f.AddAudio(t)
	ep := e.f.Audio.EnclosureOnly
	e.f.SetEnclosure(t, ep.ID, srv.URL+"/episode-1.mp3")
	src := filepath.Join(t.TempDir(), "episode.mp3")
	if err := os.WriteFile(src, mp3, 0o600); err != nil {
		t.Fatal(err)
	}
	probe, err := runner.Probe(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	for i, q := range []string{model.DownloadHigh, model.DownloadMedium, model.DownloadLow} {
		t.Run(q, func(t *testing.T) {
			device := e.device(strconv.Itoa(i + 2))
			st := e.create(e.f.P12, device, ep.ID, q)
			e.runJobs()
			r := e.row(st.ID)
			if r.state != model.DownloadReady {
				t.Fatalf("row %+v", r)
			}
			man, err := e.svc.Manifest(ctx, e.session(e.f.P12, device), st.ID)
			if err != nil {
				t.Fatal(err)
			}
			out, err := runner.Probe(ctx, filepath.Join(e.dir, st.ID, mediaFile))
			if err != nil {
				t.Fatalf("the prepared file does not probe: %v", err)
			}
			if len(out.Streams) != 1 || out.Streams[0].Codec != "aac" || man.MimeType != "audio/mp4" || man.Codecs != "mp4a.40.2" {
				t.Errorf("prepared %+v, manifest %s %s", out.Streams, man.MimeType, man.Codecs)
			}
			want := estimate(audioKbps[q], probe.Format.Duration)
			if float64(r.size) < 0.9*float64(want) || float64(r.size) > 1.1*float64(want) {
				t.Errorf("%s: %d bytes against the estimate %d", q, r.size, want)
			}
			t.Logf("%s: %d bytes, estimate %d (%+.1f %%)", q, r.size, want, 100*(float64(r.size)/float64(want)-1))
			if _, err := os.Stat(filepath.Join(e.dir, st.ID, "source.mp3")); err == nil {
				t.Error("the fetched source remains")
			}
		})
	}
}
