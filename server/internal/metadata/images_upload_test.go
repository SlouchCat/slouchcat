package metadata

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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
	if err := e.svc.runner.ScaleImage(context.Background(), in, "png_pipe", []transcode.ImageOutput{{Path: out, Width: w, Format: transcode.ImageWebP}}); err != nil {
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
	// The editor previews the upload from the stored w400 poster (a lock without choice).
	adminImgs, err := e.svc.ItemImages(ctx, idByTMDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(adminImgs) == 0 || adminImgs[0].Kind != "poster" || adminImgs[0].Provider != "" || adminImgs[0].StoredSize != "w400" ||
		(adminImgs[0].StoredHash != e.images(t, idByTMDB)["poster/w400/jpeg"].Hash && adminImgs[0].StoredHash != e.images(t, idByTMDB)["poster/w400/webp"].Hash) {
		t.Fatalf("item images after the upload %+v, want the stored w400 poster", adminImgs)
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

// manualRows returns the image rows of a kind with provider manual (an upload).
func (e *env) manualRows(t *testing.T, id, kind string) []string {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT rel_path FROM images WHERE item_id = ? AND kind = ? AND provider = 'manual'`, id, kind)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var rel string
		if err := rows.Scan(&rel); err != nil {
			t.Fatal(err)
		}
		out = append(out, rel)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// «Zurück auf automatisch» on an upload (concept 15 7.4): the uploaded rows, their files and the
// field source manual go with the lock, before the match runs. With a provider image (the TMDB
// poster) image.fetch brings it back; without one (the film has no TMDB logo) the kind is empty
// for viewers and the editor alike, and stays so after the match and image.fetch.
func TestClearUploadedImage(t *testing.T) {
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
	for _, kind := range []string{"poster", "logo"} {
		var data []byte
		if kind == "poster" {
			data = jpegBytes(t, 600, 900)
		} else {
			data = pngBytes(t, 800, 300)
		}
		if err := e.svc.UploadItemImage(ctx, idByTMDB, kind, data); err != nil {
			t.Fatal(err)
		}
		files := e.manualRows(t, idByTMDB, kind)
		if len(files) == 0 {
			t.Fatalf("no uploaded %s rows", kind)
		}
		if err := e.svc.ClearItemImage(ctx, idByTMDB, kind); err != nil {
			t.Fatal(err)
		}
		if got := e.manualRows(t, idByTMDB, kind); len(got) != 0 {
			t.Errorf("%s rows of the upload after lifting the lock: %v", kind, got)
		}
		for _, rel := range files {
			if _, err := os.Stat(filepath.Join(e.cache, filepath.FromSlash(rel))); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("%s file %s after lifting the lock: %v", kind, rel, err)
			}
		}
		var n int
		if err := e.db.Read.QueryRow(`SELECT count(*) FROM item_field_sources WHERE item_id = ? AND field = ?`, idByTMDB, kind).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s field sources after lifting the lock: %d (%v), want none", kind, n, err)
		}
		if r := e.item(t, idByTMDB); strings.Contains(r.Locked, `"`+kind+`"`) {
			t.Errorf("locked fields %s still hold %s", r.Locked, kind)
		}
	}
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	imgs := e.images(t, idByTMDB)
	if p, src := e.imageProvider(t, idByTMDB, "poster", "w400", "jpeg"); p != model.ProviderTMDB || src != e.tmdb.srv.URL+"/t/p/original/poster550.jpg" {
		t.Errorf("poster after lifting the lock: %s %s, want the TMDB poster", p, src)
	}
	for key := range imgs {
		if strings.HasPrefix(key, "logo/") {
			t.Errorf("logo row %s without a provider logo", key)
		}
	}
	adminImgs, err := e.svc.ItemImages(ctx, idByTMDB)
	if err != nil {
		t.Fatal(err)
	}
	for _, img := range adminImgs {
		if img.Kind == "logo" {
			t.Errorf("editor logo %+v, want none", img)
		}
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

// dirFiles returns the files of dir by name with the SHA-256 of their content.
func dirFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, en := range entries {
		data, err := os.ReadFile(filepath.Join(dir, en.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[en.Name()] = sha(data)
	}
	return out
}

// A file whose header passes (format by content, pixel count) but whose body ffmpeg cannot decode
// is refused as unsupported_format, and nothing of the kind changes: the rows, the choice, the
// lock, the field source and the files stay, no temporary file is left behind.
func TestUploadItemImageUndecodable(t *testing.T) {
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
	dir := filepath.Join(e.cache, "images", idByTMDB)
	full := pngBytes(t, 600, 900)
	for name, data := range map[string][]byte{
		"header only":    pngHeader(600, 900),
		"truncated png":  full[:len(full)/2],
		"truncated jpeg": jpegBytes(t, 600, 900)[:400],
	} {
		images, chosen, locked, files := e.images(t, idByTMDB), e.itemImages(t, idByTMDB), e.item(t, idByTMDB).Locked, dirFiles(t, dir)
		if got := uploadCode(t, e.svc.UploadItemImage(ctx, idByTMDB, "poster", data)); got != "file unsupported_format" {
			t.Errorf("%s: %s, want file unsupported_format", name, got)
		}
		if after := e.images(t, idByTMDB); !maps.Equal(after, images) {
			t.Errorf("%s: image rows changed", name)
		}
		if after := e.itemImages(t, idByTMDB); !maps.Equal(after, chosen) {
			t.Errorf("%s: item_images %v, want %v", name, after, chosen)
		}
		if after := e.item(t, idByTMDB).Locked; after != locked {
			t.Errorf("%s: locked fields %s, want %s", name, after, locked)
		}
		if after := dirFiles(t, dir); !maps.Equal(after, files) {
			t.Errorf("%s: image dir %v, want %v", name, after, files)
		}
	}
	if _, ok := e.itemImages(t, idByTMDB)["poster"]; !ok {
		t.Error("the poster lost its item_images row")
	}
	if p, src := e.imageProvider(t, idByTMDB, "poster", originalSize, originalFormat); p != "tmdb" || strings.HasPrefix(src, "upload:") {
		t.Errorf("poster original %s %s, want the TMDB one", p, src)
	}
}

// vp8x wraps the simple lossy WebP simple (VP8 chunk right after the header) in an extended file
// with a w×h canvas, flags and the chunks extra before the frame.
func vp8x(simple []byte, w, h int, flags byte, extra ...[]byte) []byte {
	var body bytes.Buffer
	body.WriteString("WEBPVP8X")
	_ = binary.Write(&body, binary.LittleEndian, uint32(10))
	body.Write([]byte{flags, 0, 0, 0})
	body.Write([]byte{byte(w - 1), byte((w - 1) >> 8), byte((w - 1) >> 16), byte(h - 1), byte((h - 1) >> 8), byte((h - 1) >> 16)})
	for _, c := range extra {
		body.Write(c)
	}
	body.Write(simple[12:]) // the VP8 chunk
	var out bytes.Buffer
	out.WriteString("RIFF")
	_ = binary.Write(&out, binary.LittleEndian, uint32(body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

// An extended WebP counts by its frame, not by the canvas it declares: a 100×100 canvas around an
// 800×600 frame (ffmpeg decodes the frame) is refused, an animated WebP as well; a canvas that
// matches its frame is taken with the frame's size.
func TestUploadItemImageWebPFrame(t *testing.T) {
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
	simple := e.webpBytes(t, 800, 600)
	if string(simple[12:16]) != "VP8 " {
		t.Fatalf("the encoder wrote %q, want a simple lossy WebP", simple[12:16])
	}
	anim := []byte("ANIM\x06\x00\x00\x00\x00\x00\x00\x00\x00\x00")
	for name, data := range map[string][]byte{
		"small canvas":   vp8x(simple, 100, 100, 0),
		"large canvas":   vp8x(simple, 1600, 1200, 0),
		"animation flag": vp8x(simple, 800, 600, 0x02),
		"anim chunk":     vp8x(simple, 800, 600, 0, anim),
	} {
		if got := uploadCode(t, e.svc.UploadItemImage(ctx, idByTMDB, "backdrop", data)); got != "file unsupported_format" {
			t.Errorf("%s: %s, want file unsupported_format", name, got)
		}
	}
	exif := []byte("EXIF\x03\x00\x00\x00abc\x00") // odd size, padded
	ok := vp8x(simple, 800, 600, 0x08, exif)
	if err := e.svc.UploadItemImage(ctx, idByTMDB, "backdrop", ok); err != nil {
		t.Fatalf("matching canvas: %v", err)
	}
	if r := e.images(t, idByTMDB)["backdrop/original/jpeg"]; r.Width != 800 || r.Height != 600 || r.MediaType != "image/webp" {
		t.Fatalf("original row %+v, want an 800×600 WebP", r)
	}
}

// An image.fetch that read the locks of a title before an upload cannot write its download over
// the uploaded original: the upload waits for the fetch (lockItemImages), locks the kind and
// stores its image last, and the next fetch keeps it.
func TestUploadWaitsForRunningFetch(t *testing.T) {
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
	// The decided poster changes to one whose download hangs until release.
	arrived, release := make(chan struct{}), make(chan struct{})
	var once, arrive sync.Once
	letGo := func() { once.Do(func() { close(release) }) }
	t.Cleanup(letGo)
	slow := jpegBytes(t, 500, 750)
	e.tmdb.mux.HandleFunc("GET /t/p/original/slow.jpg", func(w http.ResponseWriter, _ *http.Request) {
		arrive.Do(func() { close(arrived) })
		<-release
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(slow)
	})
	mustExec(t, e.db.Write, `UPDATE item_images SET source = '/slow.jpg' WHERE item_id = ? AND kind = 'poster'`, idByTMDB)

	fetched := make(chan error, 1)
	go func() { fetched <- e.svc.fetchImages(ctx, idByTMDB) }()
	<-arrived
	upload := jpegBytes(t, 600, 900)
	uploaded := make(chan error, 1)
	go func() { uploaded <- e.svc.UploadItemImage(ctx, idByTMDB, "poster", upload) }()
	select {
	case err := <-uploaded:
		letGo()
		t.Fatalf("the upload finished while the fetch was downloading (%v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	letGo()
	if err := <-fetched; err != nil {
		t.Fatal(err)
	}
	if err := <-uploaded; err != nil {
		t.Fatal(err)
	}
	check := func(when string) {
		t.Helper()
		imgs := e.images(t, idByTMDB)
		for _, key := range []string{"poster/original/jpeg", "poster/w400/jpeg", "poster/w800/webp"} {
			if r := imgs[key]; r.Source != "upload:"+sha(upload) {
				t.Errorf("%s: %s from %s, want the upload", when, key, r.Source)
			}
		}
		if got, err := os.ReadFile(filepath.Join(e.cache, "images", idByTMDB, "poster-original.jpg")); err != nil || sha(got) != sha(upload) {
			t.Errorf("%s: the original file is not the upload (%v)", when, err)
		}
	}
	check("after the upload")
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	check("after the next fetch")
}
