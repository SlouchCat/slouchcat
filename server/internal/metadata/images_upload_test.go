package metadata

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// webpBytes encodes a w×h WebP with the pipeline's own ffmpeg (the standard library has no WebP
// encoder).
func (e *env) webpBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in.png")
	if err := os.WriteFile(in, pngBytes(t, w, h), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.webp")
	if err := e.svc.runner.ScaleImage(context.Background(), in, []transcode.ImageOutput{{Path: out, Width: w, Format: transcode.ImageWebP}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// pngHeader is a PNG signature with an IHDR chunk of w×h pixels and nothing else: enough for
// image.DecodeConfig, nothing to decode.
func pngHeader(w, h uint32) []byte {
	var ihdr bytes.Buffer
	ihdr.WriteString("IHDR")
	_ = binary.Write(&ihdr, binary.BigEndian, w)
	_ = binary.Write(&ihdr, binary.BigEndian, h)
	ihdr.Write([]byte{8, 6, 0, 0, 0}) // 8 bit RGBA, no interlace
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	_ = binary.Write(&out, binary.BigEndian, uint32(ihdr.Len()-4))
	out.Write(ihdr.Bytes())
	_ = binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(ihdr.Bytes()))
	return out.Bytes()
}

// imageProvider returns the provider and source_url of one image row.
func (e *env) imageProvider(t *testing.T, id, kind, size, format string) (provider, source string) {
	t.Helper()
	if err := e.db.Read.QueryRow(`SELECT provider, source_url FROM images WHERE item_id = ? AND kind = ? AND size = ? AND format = ?`,
		id, kind, size, format).Scan(&provider, &source); err != nil {
		t.Fatalf("%s %s/%s/%s: %v", id, kind, size, format, err)
	}
	return provider, source
}

// fieldSource returns the provider of a field without language (item_field_sources).
func (e *env) fieldSource(t *testing.T, id, field string) string {
	t.Helper()
	var provider string
	if err := e.db.Read.QueryRow(`SELECT provider FROM item_field_sources WHERE item_id = ? AND field = ? AND lang = ''`, id, field).Scan(&provider); err != nil {
		t.Fatalf("source of %s: %v", field, err)
	}
	return provider
}

// uploadCode returns "<field> <code>" of the single field error of err.
func uploadCode(t *testing.T, err error) string {
	t.Helper()
	var verr *model.ValidationError
	if !errors.As(err, &verr) || len(verr.Fields) != 1 {
		t.Fatalf("upload error %v, want one field error", err)
	}
	return verr.Fields[0].Field + " " + verr.Fields[0].Code
}

// Concept 15 7.5: an uploaded image is stored "locked without choice" (original as
// <kind>-original.<ext> with provider manual and source upload:<sha256>, the kind locked, no
// item_images row), its sizes are derived at once, image.fetch and the match keep it, and lifting
// the lock brings the provider's image back. The format comes from the content and the pixel
// count from the header before anything is decoded.
func TestUploadItemImage(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	tmdbPoster := e.tmdb.srv.URL + "/t/p/original/poster550.jpg"
	dir := filepath.Join(e.cache, "images", idByTMDB)

	// JPEG poster.
	poster := jpegBytes(t, 600, 900)
	if err := e.svc.UploadItemImage(ctx, idByTMDB, "poster", poster); err != nil {
		t.Fatal(err)
	}
	checkUpload := func(kind, ext, mediaType string, data []byte, w, h int, formats []imageFormat) {
		t.Helper()
		imgs := e.images(t, idByTMDB)
		src := "upload:" + sha(data)
		orig := imgs[kind+"/original/jpeg"]
		if orig.RelPath != "images/"+idByTMDB+"/"+kind+"-original."+ext || orig.Source != src || orig.Hash != sha(data) ||
			orig.MediaType != mediaType || orig.Width != int64(w) || orig.Height != int64(h) || orig.Bytes != int64(len(data)) {
			t.Fatalf("%s original row %+v", kind, orig)
		}
		if p, _ := e.imageProvider(t, idByTMDB, kind, originalSize, originalFormat); p != ImageSourceManual {
			t.Fatalf("%s original provider %q, want manual", kind, p)
		}
		for _, sz := range imageWidths[kind] {
			for _, f := range formats {
				r, ok := imgs[kind+"/"+sizeName(sz)+"/"+string(f.format)]
				if !ok || r.Source != src || r.Width != int64(sz) || r.MediaType != f.mediaType {
					t.Fatalf("%s %s %s row %+v (found %v)", kind, sizeName(sz), f.format, r, ok)
				}
				if _, err := os.Stat(filepath.Join(e.cache, filepath.FromSlash(r.RelPath))); err != nil {
					t.Fatalf("%s file: %v", r.RelPath, err)
				}
			}
		}
		if got, ok := e.itemImages(t, idByTMDB)[kind]; ok {
			t.Fatalf("item_images row %+v after the upload, want none", got)
		}
		if r := e.item(t, idByTMDB); !strings.Contains(r.Locked, `"`+kind+`"`) {
			t.Fatalf("locked fields %s, want %s", r.Locked, kind)
		}
		if p := e.fieldSource(t, idByTMDB, kind); p != ImageSourceManual {
			t.Fatalf("field source of %s %q, want manual", kind, p)
		}
	}
	checkUpload("poster", "jpg", "image/jpeg", poster, 600, 900, imageFormats)
	if a := e.images(t, idByTMDB)["poster/w200/jpeg"].Accent; !a.Valid {
		t.Error("the uploaded poster carries no accent")
	}

	// image.fetch and a new match keep the upload; TMDB is not asked for the poster again.
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	checkUpload("poster", "jpg", "image/jpeg", poster, 600, 900, imageFormats)
	if n := e.tmdb.count("/t/p/original/poster550.jpg"); n != 1 {
		t.Fatalf("TMDB poster downloaded %d times, want 1", n)
	}

	// PNG backdrop: the original keeps its type, the former JPEG original goes.
	backdrop := pngBytes(t, 1280, 720)
	if err := e.svc.UploadItemImage(ctx, idByTMDB, "backdrop", backdrop); err != nil {
		t.Fatal(err)
	}
	checkUpload("backdrop", "png", "image/png", backdrop, 1280, 720, imageFormats)
	if _, err := os.Stat(filepath.Join(dir, "backdrop-original.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the former backdrop original is still there: %v", err)
	}

	// WebP logo: derived as PNG and WebP.
	logo := e.webpBytes(t, 800, 300)
	if err := e.svc.UploadItemImage(ctx, idByTMDB, "logo", logo); err != nil {
		t.Fatal(err)
	}
	checkUpload("logo", "webp", "image/webp", logo, 800, 300, logoFormats)

	// Refused by content, before anything is stored: text named like a JPEG, a damaged JPEG
	// header, more than 50 megapixels in the header, more than 20 MB, a kind the film has not,
	// another title.
	before := e.images(t, idByTMDB)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		kind string
		data []byte
		want string
	}{
		"text":        {"poster", []byte("this is no image, only a file called poster.jpg"), "file unsupported_format"},
		"broken jpeg": {"poster", append([]byte{0xff, 0xd8, 0xff, 0xe0}, bytes.Repeat([]byte{0}, 64)...), "file unsupported_format"},
		"gif":         {"poster", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), "file unsupported_format"},
		"60 MP":       {"poster", pngHeader(10_000, 6_000), "file too_many_pixels"},
		"too large":   {"poster", append(slicesOf(poster), make([]byte, model.MaxImageUploadBytes)...), "file too_large"},
		"kind":        {"season", poster, "kind invalid_value"},
		"path kind":   {"../poster", poster, "kind invalid_value"},
	} {
		if got := uploadCode(t, e.svc.UploadItemImage(ctx, idByTMDB, tc.kind, tc.data)); got != tc.want {
			t.Errorf("%s: %s, want %s", name, got, tc.want)
		}
	}
	if after := e.images(t, idByTMDB); len(after) != len(before) || after["poster/original/jpeg"] != before["poster/original/jpeg"] {
		t.Errorf("refused uploads changed the image rows")
	}
	if now, err := os.ReadDir(dir); err != nil || len(now) != len(entries) {
		t.Errorf("refused uploads changed the image dir: %d entries, want %d (%v)", len(now), len(entries), err)
	}
	if err := e.svc.UploadItemImage(ctx, "01ZZZZZZZZZZZZZZZZZZZZZZZZ", "poster", poster); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown title: %v", err)
	}
	if err := e.svc.UploadItemImage(ctx, "../../etc", "poster", poster); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("a path as title id: %v", err)
	}

	// Lifting the lock: the match decides the TMDB poster again and image.fetch replaces the upload.
	if err := e.svc.ClearItemImage(ctx, idByTMDB, "poster"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.itemImages(t, idByTMDB)["poster"]; got.Source != "/poster550.jpg" || got.Chosen != 0 {
		t.Fatalf("after lifting the lock %+v, want the automatic TMDB poster", got)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"original/jpeg", "w400/jpeg", "w800/webp"} {
		size, format, _ := strings.Cut(key, "/")
		if p, src := e.imageProvider(t, idByTMDB, "poster", size, format); src != tmdbPoster || p != model.ProviderTMDB {
			t.Errorf("poster %s after lifting the lock: %s %s, want tmdb %s", key, p, src, tmdbPoster)
		}
	}
	if p := e.fieldSource(t, idByTMDB, "poster"); p != model.ProviderTMDB {
		t.Errorf("field source of the poster %q after lifting the lock, want tmdb", p)
	}
}

// slicesOf copies b, so appending to the copy leaves b alone.
func slicesOf(b []byte) []byte { return append([]byte(nil), b...) }

// The episode still («Folgenbild») takes an upload while the setting "episode stills" is on.
func TestUploadEpisodeStill(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.addTree(t, node{title: "Test Show", tmdbID: 900})
	if err := e.svc.match(ctx, idSeries); err != nil {
		t.Fatal(err)
	}
	still := jpegBytes(t, 1280, 720)
	if err := e.svc.UploadItemImage(ctx, idE0101, "backdrop", still); err != nil {
		t.Fatal(err)
	}
	imgs := e.images(t, idE0101)
	if r := imgs["backdrop/w780/webp"]; r.Source != "upload:"+sha(still) {
		t.Fatalf("still rows %v", imgs)
	}
	if _, ok := e.itemImages(t, idE0101)["backdrop"]; ok {
		t.Fatal("item_images row after the upload")
	}
	if got := uploadCode(t, e.svc.UploadItemImage(ctx, idE0101, "poster", still)); got != "kind invalid_value" {
		t.Fatalf("poster of an episode: %s", got)
	}
	if got := uploadCode(t, e.svc.UploadItemImage(ctx, idSeason1, "backdrop", still)); got != "kind invalid_value" {
		t.Fatalf("backdrop of a season: %s", got)
	}
	mustExec(t, e.db.Write, `INSERT INTO settings (key, value, updated_at) VALUES (?, '["logo","season_poster"]', 0)`, SettingImageKinds)
	if got := uploadCode(t, e.svc.UploadItemImage(ctx, idE0101, "backdrop", still)); got != "kind invalid_value" {
		t.Fatalf("episode still with the setting off: %s", got)
	}
}
