package playback

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// M6: an episode without a file plays from its enclosure through the server: a direct audio
// session without media file, Range passed through, visibility re-checked on every request, and
// failures of the podcast host answered as enclosure_unavailable.
func TestEnclosurePlayback(t *testing.T) {
	f := vistest.New(t)
	f.AddAudio(t)
	s := New(f.DB, vistest.Logger())
	ctx := context.Background()
	ep := f.Audio.EnclosureOnly
	session := browserSession(t, f, f.P12)

	info, err := s.Start(ctx, session, ep.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if info.MediaKind != model.MediaKindAudio || info.Mode != ModeDirect || info.MediaFile.ID != "" ||
		info.MediaFile.DurationMs != vistest.DurationMs || info.MediaFile.MimeType != vistest.EnclosureType || info.Chapters == nil {
		t.Fatalf("info = %+v", info)
	}
	g, err := s.Authorize(ctx, tokenOf(t, info.StreamURL))
	if err != nil || g.MediaFileID != "" || g.ItemID != ep.ID {
		t.Fatalf("grant = %+v %v", g, err)
	}
	// A Range request is passed through.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, info.StreamURL, nil)
	req.Header.Set("Range", "bytes=10-19")
	if err := s.ProxyEnclosure(ctx, g, rec, req); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusPartialContent || rec.Body.String() != string(ep.Content[10:20]) ||
		rec.Header().Get("Content-Range") != fmt.Sprintf("bytes 10-19/%d", vistest.FileSize) || rec.Header().Get("Accept-Ranges") != "bytes" ||
		rec.Header().Get("Content-Type") != vistest.EnclosureType || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Errorf("range: %d %v", rec.Code, rec.Header())
	}
	rec = httptest.NewRecorder()
	if err := s.ProxyEnclosure(ctx, g, rec, httptest.NewRequest(http.MethodGet, info.StreamURL, nil)); err != nil || rec.Code != http.StatusOK ||
		rec.Body.Len() != vistest.FileSize || rec.Header().Get("Content-Length") != fmt.Sprint(vistest.FileSize) {
		t.Errorf("whole file: %v %d %d", err, rec.Code, rec.Body.Len())
	}
	// Failures of the podcast host before the response started.
	for name, target := range map[string]string{
		"closed host": func() string {
			srv := httptest.NewServer(http.NotFoundHandler())
			srv.Close()
			return srv.URL + "/x.mp3"
		}(),
		"404":    f.Audio.EnclosureServer.URL + "/episodes/missing.mp3",
		"scheme": "file:///etc/passwd",
	} {
		f.SetEnclosure(t, ep.ID, target)
		rec := httptest.NewRecorder()
		if err := s.ProxyEnclosure(ctx, g, rec, httptest.NewRequest(http.MethodGet, info.StreamURL, nil)); !errors.Is(err, model.ErrEnclosureUnavailable) || rec.Body.Len() != 0 {
			t.Errorf("%s: %v", name, err)
		}
	}
	f.SetEnclosure(t, ep.ID, ep.Enclosure)
	// The title becomes invisible to the profile: 404 with the valid token.
	sixteen := 16
	f.SetRating(t, f.Audio.Podcast.ID, &sixteen)
	if err := s.ProxyEnclosure(ctx, g, httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, info.StreamURL, nil)); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("invisible episode: %v", err)
	}
	// A grant with a media file is no enclosure session.
	g.MediaFileID = "01J9000000000000000000F0Q0"
	if err := s.ProxyEnclosure(ctx, g, httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, info.StreamURL, nil)); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("grant with a file: %v", err)
	}
	// The explicit episode is invisible to P12: the start is 404.
	if _, err := s.Start(ctx, session, f.Audio.Explicit.ID, model.PlaybackRequest{}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("explicit episode for P12: %v", err)
	}
	if _, err := s.Start(ctx, browserSession(t, f, f.P18), f.Audio.Explicit.ID, model.PlaybackRequest{}); err != nil {
		t.Errorf("explicit episode for P18: %v", err)
	}
}

// M6 review M-1: whatever type the podcast host sends, the proxied answer is audio (or
// application/octet-stream) with nosniff and a sandboxing CSP, never a page on the server's origin.
func TestEnclosureContentType(t *testing.T) {
	f := vistest.New(t)
	f.AddAudio(t)
	s := New(f.DB, vistest.Logger())
	ctx := context.Background()
	ep := f.Audio.EnclosureOnly
	upstream := ""
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if upstream != "" {
			w.Header().Set("Content-Type", upstream)
		}
		w.Header().Set("Set-Cookie", "host=podcast; Path=/")
		w.Header().Set("Keep-Alive", "timeout=5")
		w.Header().Set("X-Host-Header", "1")
		_, _ = w.Write([]byte("<html><script>alert(1)</script></html>"))
	}))
	defer host.Close()
	f.SetEnclosure(t, ep.ID, host.URL+"/episode.mp3")
	info, err := s.Start(ctx, browserSession(t, f, f.P12), ep.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	g, err := s.Authorize(ctx, tokenOf(t, info.StreamURL))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ upstream, feed, want string }{
		{"audio/mp4; charset=binary", vistest.EnclosureType, "audio/mp4"},
		{"video/mp4", "", "video/mp4"},
		{"text/html; charset=utf-8", vistest.EnclosureType, vistest.EnclosureType},
		{"image/svg+xml", "", "application/octet-stream"},
		{"", "text/html", "application/octet-stream"},
	} {
		upstream = tc.upstream
		f.SetEnclosureType(t, ep.ID, tc.feed)
		rec := httptest.NewRecorder()
		if err := s.ProxyEnclosure(ctx, g, rec, httptest.NewRequest(http.MethodGet, info.StreamURL, nil)); err != nil {
			t.Fatal(err)
		}
		h := rec.Header()
		if h.Get("Content-Type") != tc.want || h.Get("X-Content-Type-Options") != "nosniff" ||
			h.Get("Content-Security-Policy") != "sandbox; default-src 'none'" || h.Get("Content-Disposition") != "inline" {
			t.Errorf("upstream %q, feed %q: headers %v", tc.upstream, tc.feed, h)
		}
		// M6 review m-12: cookies, hop-by-hop and other headers of the podcast host stay behind.
		for _, k := range []string{"Set-Cookie", "Keep-Alive", "X-Host-Header"} {
			if h.Get(k) != "" {
				t.Errorf("header %s passed through", k)
			}
		}
	}
	// M6 review m-12: the episode turns explicit (18) while P12 holds a stream token: 404.
	eighteen := 18
	f.SetRating(t, ep.ID, &eighteen)
	if err := s.ProxyEnclosure(ctx, g, httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, info.StreamURL, nil)); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("explicit episode on an earlier token: %v", err)
	}
}

// M6 review OQ-1: an enclosure in the local network plays only when the podcast's feed is there
// too (the fixture feed lives on the loopback address); the enclosure of a public feed that points
// into the local network is refused after DNS resolution.
func TestEnclosureLocalNetwork(t *testing.T) {
	f := vistest.New(t)
	f.AddAudio(t)
	s := New(f.DB, vistest.Logger())
	ctx := context.Background()
	ep := f.Audio.EnclosureOnly
	info, err := s.Start(ctx, browserSession(t, f, f.P12), ep.ID, model.PlaybackRequest{})
	if err != nil {
		t.Fatal(err)
	}
	g, err := s.Authorize(ctx, tokenOf(t, info.StreamURL))
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := s.ProxyEnclosure(ctx, g, rec, httptest.NewRequest(http.MethodGet, info.StreamURL, nil)); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("loopback feed: %v %d", err, rec.Code)
	}
	f.SetFeedURL(t, f.Audio.Podcast.ID, "https://203.0.113.5/feed.xml")
	rec = httptest.NewRecorder()
	if err := s.ProxyEnclosure(ctx, g, rec, httptest.NewRequest(http.MethodGet, info.StreamURL, nil)); !errors.Is(err, model.ErrEnclosureUnavailable) || rec.Body.Len() != 0 {
		t.Errorf("public feed, local enclosure: %v %d", err, rec.Body.Len())
	}
}
