package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// toChapters sends auto and imageUrl only when set (contract 1.12.0) and never a nil list.
func TestToChapters(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(toChapters([]model.Chapter{
		{StartMs: 0, EndMs: 600_000, Auto: true},
		{StartMs: 600_000, EndMs: 900_000, Title: "Credits", ImageURL: "/api/v1/items/x/chapters/1/image?mediaFileId=y"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"auto":true,"endMs":600000,"startMs":0},` +
		`{"endMs":900000,"imageUrl":"/api/v1/items/x/chapters/1/image?mediaFileId=y","startMs":600000,"title":"Credits"}]`
	if string(b) != want {
		t.Errorf("chapters = %s, want %s", b, want)
	}
	if b, _ := json.Marshal(toChapters(nil)); string(b) != "[]" {
		t.Errorf("nil chapters = %s", b)
	}
}

// chapterTestProbe is a stored probe of 45 minutes without chapter marks: five replacement
// chapters.
func chapterTestProbe(t *testing.T) (string, transcode.Probe) {
	t.Helper()
	p := transcode.Probe{Format: transcode.Format{Name: "mov,mp4", Duration: 45 * time.Minute},
		Streams: []transcode.Stream{{Index: 0, Type: "video", Codec: "h264", Width: 1920, Height: 1080}}}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b), p
}

// writeChapterImage stores a chapter image of a media file at a version below dir, like the job
// chapters.images does.
func writeChapterImage(t *testing.T, dir, fileID, version string, idx int, body []byte) {
	t.Helper()
	folder := filepath.Join(dir, filepath.FromSlash(library.ChapterImagesRel(fileID, version)))
	if err := os.MkdirAll(folder, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, library.ChapterImageName(idx)), body, 0o600); err != nil {
		t.Fatal(err)
	}
}

// getChapterImage (concept 13 stage 3): the image of a visible title's file, 404 for everything
// else: another profile's age rule, a file of another (invisible) title, an index without image,
// malformed parameters and a new probe. The version in v decides the caching.
func TestChapterImage(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f := pf.f
	dir := t.TempDir()
	pb := playback.New(f.DB, vistest.Logger(), playback.WithTokenKey(testTokenKey), playback.WithCacheDir(dir))
	h := pf.handlerWithPlayback(pb)
	m12, m18 := f.Rated(12), f.Rated(18)
	raw, probe := chapterTestProbe(t)
	for _, m := range []vistest.Movie{m12, m18} {
		f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", raw, nil)
	}
	f.SetProbedAt(t, m12.FileID, 1234)
	f.SetProbedAt(t, m18.FileID, 99)
	version := library.ChapterImagesVersion(1234, transcode.Chapters(&probe, "movie", 0))
	image := []byte("RIFF\x10\x00\x00\x00WEBPVP8 chapter")
	writeChapterImage(t, dir, m12.FileID, version, 0, image)
	writeChapterImage(t, dir, m18.FileID, library.ChapterImagesVersion(99, transcode.Chapters(&probe, "movie", 0)), 0,
		[]byte("RIFF\x10\x00\x00\x00WEBPVP8 other"))
	path := func(itemID, fileID, idx string) string {
		return BasePath + "/items/" + itemID + "/chapters/" + idx + "/image?mediaFileId=" + fileID + "&v=" + version
	}

	for _, token := range []string{"p18", "p12"} {
		rec := send(t, h, http.MethodGet, path(m12.ID, m12.FileID, "0"), token, "", nil)
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), image) {
			t.Fatalf("%s: status %d (%s)", token, rec.Code, rec.Body.String())
		}
		if ct, cc := rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"); ct != "image/webp" || cc != "private, max-age=31536000, immutable" {
			t.Errorf("%s: content type %q, cache control %q", token, ct, cc)
		}
		if rec.Header().Get("Last-Modified") == "" || rec.Header().Get("ETag") != `"`+version+`"` {
			t.Errorf("%s: Last-Modified %q, ETag %q", token, rec.Header().Get("Last-Modified"), rec.Header().Get("ETag"))
		}
	}
	// No v, a malformed one or one of another probe: the image of the current version, but
	// revalidated (no-cache); a matching If-None-Match answers 304.
	for _, q := range []string{"", "&v=1", "&v=99-0123abcd"} {
		rec := send(t, h, http.MethodGet, BasePath+"/items/"+m12.ID+"/chapters/0/image?mediaFileId="+m12.FileID+q, "p18", "", nil)
		if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "private, no-cache" || !bytes.Equal(rec.Body.Bytes(), image) ||
			rec.Header().Get("ETag") != `"`+version+`"` {
			t.Errorf("v %q: status %d, cache control %q, ETag %q", q, rec.Code, rec.Header().Get("Cache-Control"), rec.Header().Get("ETag"))
		}
	}
	if rec := send(t, h, http.MethodGet, path(m12.ID, m12.FileID, "0"), "p18", "", map[string]string{"If-None-Match": `"` + version + `"`}); rec.Code != http.StatusNotModified {
		t.Errorf("If-None-Match: status %d", rec.Code)
	}
	// Another chapter list of the same probe (older chapter rules) that the sweep has not removed
	// yet: its own URL serves its own image, immutable.
	older := []byte("RIFF\x10\x00\x00\x00WEBPVP8 older")
	writeChapterImage(t, dir, m12.FileID, "1234-0123abcd", 0, older)
	rec := send(t, h, http.MethodGet, BasePath+"/items/"+m12.ID+"/chapters/0/image?mediaFileId="+m12.FileID+"&v=1234-0123abcd", "p18", "", nil)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), older) || rec.Header().Get("ETag") != `"1234-0123abcd"` {
		t.Errorf("older chapter list: status %d, ETag %q", rec.Code, rec.Header().Get("ETag"))
	}
	for _, c := range []struct{ name, token, path string }{
		{"age rule", "p6", path(m12.ID, m12.FileID, "0")},
		{"invisible title", "p12", path(m18.ID, m18.FileID, "0")},
		{"file of another, invisible title", "p12", path(m12.ID, m18.FileID, "0")},
		{"file of another title", "p18", path(m12.ID, m18.FileID, "0")},
		{"index without image", "p18", path(m12.ID, m12.FileID, "1")},
		{"index with a leading zero", "p18", path(m12.ID, m12.FileID, "00")},
		{"index no number", "p18", path(m12.ID, m12.FileID, "x")},
		{"negative index", "p18", path(m12.ID, m12.FileID, "-1")},
		{"file id no ULID", "p18", path(m12.ID, "not-a-ulid", "0")},
		{"no file id", "p18", BasePath + "/items/" + m12.ID + "/chapters/0/image"},
	} {
		rec := send(t, h, http.MethodGet, c.path, c.token, "", nil)
		if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
			t.Errorf("%s: status %d (%s), want 404 not_found", c.name, rec.Code, rec.Body.String())
		}
	}
	if rec := send(t, h, http.MethodGet, path(m12.ID, m12.FileID, "0"), "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("without session: status %d", rec.Code)
	}
	// A new probe of the file: its version has no folder yet, the old URL finds nothing.
	f.SetProbedAt(t, m12.FileID, 5678)
	if rec := send(t, h, http.MethodGet, path(m12.ID, m12.FileID, "0"), "p18", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("new probe: status %d", rec.Code)
	}
}
