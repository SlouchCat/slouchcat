package metadata

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Covers of audiobooks and podcasts (M6): a folder image, the picture embedded in an M4B, and a
// feed image from any host.

const (
	idBookFolder   = "01JTESTBOOK00000000000000A"
	idBookEmbedded = "01JTESTBOOK00000000000000B"
	idPodcast      = "01JTESTPODCAST000000000000"
	idEpisode      = "01JTESTEPISODE000000000000"
)

// audioEnv is an env with ffprobe (ExtractCover probes the file), an audiobook library on root and
// a podcast library.
func audioEnv(t *testing.T, root string) *env {
	t.Helper()
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	probe := filepath.Join(filepath.Dir(lookFFmpeg()), "ffprobe")
	if _, err := os.Stat(probe); err != nil {
		t.Skip("ffprobe not installed")
	}
	e.svc.runner = transcode.Runner{FFmpegPath: lookFFmpeg(), FFprobePath: probe}
	mustExec(t, e.db.Write, `INSERT INTO libraries (id, name, kind, default_rating, created_at, updated_at) VALUES (3, 'Hörbücher', 'audiobooks', 0, 0, 0)`)
	mustExec(t, e.db.Write, `INSERT INTO libraries (id, name, kind, default_rating, created_at, updated_at) VALUES (4, 'Podcasts', 'podcasts', 0, 0, 0)`)
	mustExec(t, e.db.Write, `INSERT INTO library_roots (id, library_id, path, fs_type, watch, created_at) VALUES (3, 3, ?, 'ext4', 'inotify', 0)`, root)
	return e
}

// addAudio inserts a matched audio item.
func (e *env) addAudio(t *testing.T, id, kind string, library int, parent, coverPath, coverURL any) {
	t.Helper()
	mustExec(t, e.db.Write, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, cover_path, cover_url, metadata_state,
	                         added_at, created_at, updated_at) VALUES (?, ?, ?, ?, 'T', 't', ?, ?, 'matched', 0, 0, 0)`,
		id, kind, parent, library, coverPath, coverURL)
}

func (e *env) addMediaFile(t *testing.T, id, itemID, rel string) {
	t.Helper()
	mustExec(t, e.db.Write, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, created_at, updated_at)
	                         VALUES (?, ?, 3, ?, 1, 1, 'mp4', 0, 0)`, id, itemID, rel)
}

// checkCover asserts the poster rows of a cover: the original and three widths in JPEG and WebP
// with an accent, all from source.
func (e *env) checkCover(t *testing.T, id, sourcePrefix string, w, h int) {
	t.Helper()
	imgs := e.images(t, id)
	orig, ok := imgs["poster/original/jpeg"]
	if !ok || !strings.HasPrefix(orig.Source, sourcePrefix) || orig.Width != int64(w) || orig.Height != int64(h) {
		t.Fatalf("original %+v ok %v, want %dx%d from %q", orig, ok, w, h, sourcePrefix)
	}
	for _, size := range []string{"w200", "w400", "w800"} {
		for _, f := range []string{"jpeg", "webp"} {
			r, ok := imgs["poster/"+size+"/"+f]
			if !ok || r.Source != orig.Source || !r.Accent.Valid {
				t.Errorf("%s %s: %+v ok %v", size, f, r, ok)
			}
			if _, err := os.Stat(filepath.Join(e.cache, filepath.FromSlash(r.RelPath))); err != nil {
				t.Errorf("%s %s file: %v", size, f, err)
			}
		}
	}
	if len(imgs) != 7 {
		t.Errorf("%d image rows, want 7", len(imgs))
	}
}

func TestImageFetchFolderCover(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// A "?" in a folder name stays part of the path (the version follows the last "?").
	if err := os.MkdirAll(filepath.Join(root, "Verfasser", "Buch?"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Verfasser", "Buch?", "cover.jpg"), jpegBytes(t, 500, 500), 0o600); err != nil {
		t.Fatal(err)
	}
	e := audioEnv(t, root)
	e.addAudio(t, idBookFolder, "audiobook", 3, nil, "Verfasser/Buch?/cover.jpg", nil)
	e.addMediaFile(t, "01JTESTFILE00000000000000A", idBookFolder, "Verfasser/Buch?/01.mp3")
	ctx := context.Background()
	if err := e.svc.fetchImages(ctx, idBookFolder); err != nil {
		t.Fatal(err)
	}
	e.checkCover(t, idBookFolder, "file:Verfasser/Buch?/cover.jpg?", 500, 500)
	// Unchanged: nothing is read again; a replaced cover file is.
	before := e.images(t, idBookFolder)["poster/w200/jpeg"].Hash
	if err := e.svc.fetchImages(ctx, idBookFolder); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Verfasser", "Buch?", "cover.jpg"), jpegBytes(t, 300, 300), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(root, "Verfasser", "Buch?", "cover.jpg"), testNow, testNow); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.fetchImages(ctx, idBookFolder); err != nil {
		t.Fatal(err)
	}
	if after := e.images(t, idBookFolder)["poster/w200/jpeg"].Hash; after == before {
		t.Error("replaced cover not fetched again")
	}
	e.checkCover(t, idBookFolder, "file:", 300, 300)
}

func TestImageFetchEmbeddedCover(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", "media", "Audiobooks"))
	if err != nil {
		t.Fatal(err)
	}
	rel := "Slouchcat Test Author/Slouchcat Test Audio Series/01 - Slouchcat Test Audiobook M4B/Slouchcat Test Audiobook M4B.m4b"
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("test media missing (run `task testdata:media` in the repository root): %v", err)
	}
	e := audioEnv(t, root)
	const fileID = "01JTESTFILE00000000000000B"
	e.addAudio(t, idBookEmbedded, "audiobook", 3, nil, "embedded:"+fileID, nil)
	e.addMediaFile(t, fileID, idBookEmbedded, rel)
	if err := e.svc.fetchImages(context.Background(), idBookEmbedded); err != nil {
		t.Fatal(err)
	}
	e.checkCover(t, idBookEmbedded, "embedded:"+fileID+"?", 600, 600)
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(jpegBytes(t, w, h)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestImageFetchRemoteCover(t *testing.T) {
	t.Parallel()
	cover := pngBytes(t, 400, 400)
	mux := http.NewServeMux()
	mux.HandleFunc("/feed/cover.png", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("cover request carries credentials")
		}
		w.Header().Set("Content-Type", "image/jpg") // a common wrong type: the content decides
		_, _ = w.Write(cover)
	})
	mux.HandleFunc("/page.html", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>no image</body></html>"))
	})
	mux.HandleFunc("/huge.png", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(cover[:64])
		_, _ = w.Write(make([]byte, maxCoverBytes))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	e := audioEnv(t, t.TempDir())
	ctx := context.Background()
	e.addAudio(t, idPodcast, "podcast", 4, nil, nil, srv.URL+"/feed/cover.png")
	// The feed itself lives on the loopback address, so its images may too (package netguard).
	mustExec(t, e.db.Write, `UPDATE items SET feed_url = ? WHERE id = ?`, srv.URL+"/feed.xml", idPodcast)
	if err := e.svc.fetchImages(ctx, idPodcast); err != nil {
		t.Fatal(err)
	}
	e.checkCover(t, idPodcast, srv.URL+"/feed/cover.png", 400, 400)
	if got := e.images(t, idPodcast)["poster/original/jpeg"].MediaType; got != "image/png" {
		t.Errorf("original media type %q", got)
	}

	// An episode with its own image; a broken one fails (and is retried by the job), a page is no
	// image, a body over 10 MiB is refused.
	for _, c := range []string{"/page.html", "/huge.png", "/missing.png"} {
		mustExec(t, e.db.Write, `DELETE FROM items WHERE id = ?`, idEpisode)
		e.addAudio(t, idEpisode, "podcast_episode", 4, idPodcast, nil, srv.URL+c)
		if err := e.svc.fetchImages(ctx, idEpisode); err == nil {
			t.Errorf("%s: no error", c)
		}
		if n := len(e.images(t, idEpisode)); n != 0 {
			t.Errorf("%s: %d image rows", c, n)
		}
	}
	// M6 review OQ-1: the image of a public feed that points into the local network is refused,
	// for the episode (through its podcast) and the podcast.
	mustExec(t, e.db.Write, `UPDATE items SET feed_url = 'https://203.0.113.5/feed.xml?token=x' WHERE id = ?`, idPodcast)
	mustExec(t, e.db.Write, `UPDATE items SET cover_url = ? WHERE id = ?`, srv.URL+"/feed/cover.png", idEpisode)
	if err := e.svc.fetchImages(ctx, idEpisode); !errors.Is(err, netguard.ErrRefused) || len(e.images(t, idEpisode)) != 0 {
		t.Errorf("local image of a public feed: %v", err)
	}
	mustExec(t, e.db.Write, `UPDATE items SET cover_url = ? WHERE id = ?`, srv.URL+"/feed/other.png", idPodcast)
	if err := e.svc.fetchImages(ctx, idPodcast); !errors.Is(err, netguard.ErrRefused) {
		t.Errorf("local image of a public podcast: %v", err)
	}
	// Not http(s): nothing to fetch.
	mustExec(t, e.db.Write, `UPDATE items SET cover_url = 'ftp://example.test/c.jpg' WHERE id = ?`, idEpisode)
	if err := e.svc.fetchImages(ctx, idEpisode); err != nil || len(e.images(t, idEpisode)) != 0 {
		t.Errorf("ftp cover: %v", err)
	}
}

func TestQueueIncompleteImagesCoversAudio(t *testing.T) {
	t.Parallel()
	e := audioEnv(t, t.TempDir())
	e.addAudio(t, idBookFolder, "audiobook", 3, nil, "Verfasser/Buch/cover.jpg", nil)
	e.addAudio(t, idBookEmbedded, "audiobook", 3, nil, nil, nil) // no cover: nothing to queue
	if n, err := e.svc.QueueIncompleteImages(context.Background(), 3); err != nil || n != 1 {
		t.Errorf("queued %d (%v), want 1", n, err)
	}
	if jobs := e.imageJobs(t); len(jobs) != 1 || !strings.HasPrefix(jobs[0], "image:"+idBookFolder) {
		t.Errorf("image jobs %v", jobs)
	}
}

func TestImageConfigWebP(t *testing.T) {
	t.Parallel()
	header := func(chunk string, payload []byte) []byte {
		b := append([]byte("RIFF\x00\x00\x00\x00WEBP"+chunk), make([]byte, 4)...)
		return append(b, payload...)
	}
	vp8 := make([]byte, 14)
	copy(vp8[3:], []byte{0x9d, 0x01, 0x2a})
	binary.LittleEndian.PutUint16(vp8[6:], 640)
	binary.LittleEndian.PutUint16(vp8[8:], 480)
	vp8l := make([]byte, 14)
	vp8l[0] = 0x2f
	binary.LittleEndian.PutUint32(vp8l[1:], uint32(300-1)|uint32(200-1)<<14)
	vp8x := make([]byte, 14)
	vp8x[4], vp8x[5], vp8x[6] = 0xff, 0x03, 0 // width 1024
	vp8x[7], vp8x[8], vp8x[9] = 0xff, 0x01, 0 // height 512
	for _, c := range []struct {
		data []byte
		w, h int
	}{{header("VP8 ", vp8), 640, 480}, {header("VP8L", vp8l), 300, 200}, {header("VP8X", vp8x), 1024, 512}} {
		cfg, err := imageConfig(c.data, "image/webp")
		if err != nil || cfg.Width != c.w || cfg.Height != c.h {
			t.Errorf("%s: %dx%d %v, want %dx%d", c.data[12:16], cfg.Width, cfg.Height, err, c.w, c.h)
		}
	}
	if _, err := imageConfig([]byte("RIFF"), "image/webp"); err == nil {
		t.Error("short webp accepted")
	}
}
