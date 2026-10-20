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
