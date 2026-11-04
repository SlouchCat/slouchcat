package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/library"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
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

// writeChapterImage stores a chapter image of a media file at a probe below dir and sets the file's
// probed_at, like the job chapters.images does.
func writeChapterImage(t *testing.T, f *vistest.Fixture, dir, fileID string, probedAt int64, idx int, body []byte) {
	t.Helper()
	f.SetProbedAt(t, fileID, probedAt)
	folder := filepath.Join(dir, filepath.FromSlash(library.ChapterImagesRel(fileID, probedAt)))
	if err := os.MkdirAll(folder, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, library.ChapterImageName(idx)), body, 0o600); err != nil {
		t.Fatal(err)
	}
}

// getChapterImage (concept 13 stage 3): the image of a visible title's file, 404 for everything
// else: another profile's age rule, a file of another (invisible) title, an index without image,
// malformed parameters and an older probe.
func TestChapterImage(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f := pf.f
	dir := t.TempDir()
	pb := playback.New(f.DB, vistest.Logger(), playback.WithTokenKey(testTokenKey), playback.WithCacheDir(dir))
	h := pf.handlerWithPlayback(pb)
	m12, m18 := f.Rated(12), f.Rated(18)
	image := []byte("RIFF\x10\x00\x00\x00WEBPVP8 chapter")
	writeChapterImage(t, f, dir, m12.FileID, 1234, 0, image)
	writeChapterImage(t, f, dir, m18.FileID, 99, 0, []byte("RIFF\x10\x00\x00\x00WEBPVP8 other"))
	path := func(itemID, fileID, idx string) string {
		return BasePath + "/items/" + itemID + "/chapters/" + idx + "/image?mediaFileId=" + fileID + "&v=1234"
	}

	for _, token := range []string{"p18", "p12"} {
		rec := send(t, h, http.MethodGet, path(m12.ID, m12.FileID, "0"), token, "", nil)
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), image) {
			t.Fatalf("%s: status %d (%s)", token, rec.Code, rec.Body.String())
		}
		if ct, cc := rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"); ct != "image/webp" || cc != "private, max-age=31536000, immutable" {
			t.Errorf("%s: content type %q, cache control %q", token, ct, cc)
		}
		if rec.Header().Get("Last-Modified") == "" || rec.Header().Get("ETag") != `"1234"` {
			t.Errorf("%s: Last-Modified %q, ETag %q", token, rec.Header().Get("Last-Modified"), rec.Header().Get("ETag"))
		}
	}
	// Another or no v: the image of the current probe, but revalidated (no-cache); a matching
	// If-None-Match answers 304.
	for _, q := range []string{"&v=1", ""} {
		rec := send(t, h, http.MethodGet, BasePath+"/items/"+m12.ID+"/chapters/0/image?mediaFileId="+m12.FileID+q, "p18", "", nil)
		if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "private, no-cache" || !bytes.Equal(rec.Body.Bytes(), image) {
			t.Errorf("v %q: status %d, cache control %q", q, rec.Code, rec.Header().Get("Cache-Control"))
		}
	}
	if rec := send(t, h, http.MethodGet, path(m12.ID, m12.FileID, "0"), "p18", "", map[string]string{"If-None-Match": `"1234"`}); rec.Code != http.StatusNotModified {
		t.Errorf("If-None-Match: status %d", rec.Code)
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
	// A new probe of the file: the folder of the old probe is no longer served.
	f.SetProbedAt(t, m12.FileID, 5678)
	if rec := send(t, h, http.MethodGet, path(m12.ID, m12.FileID, "0"), "p18", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("older probe: status %d", rec.Code)
	}
}
