package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// serveAllImages answers /movie/{id}/images of the fake with the fixture's images block (every
// language, as TMDB answers without include_image_language).
func (f *fakeTMDB) serveAllImages(t *testing.T) {
	t.Helper()
	f.mux.HandleFunc("GET /3/movie/{id}/images", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("include_image_language") || r.URL.Query().Has("language") {
			t.Errorf("candidates asked with a language filter: %s", r.URL)
		}
		f.mu.Lock()
		f.hits[r.URL.Path]++
		fx := f.movies[r.PathValue("id")]
		f.mu.Unlock()
		if fx.images == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fx.images)
	})
}

// The image choice per title (concept 10 6.4, 7.3; DoD of step 5): candidates of every source,
// chosen and locked, kept through an apply with texts and images, lifted again.
func TestImageChoiceSurvivesApply(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	e.tmdb.serveAllImages(t)
	ctx := context.Background()
	e.tmdb.setImages("550",
		[]map[string]any{
			tmdbImage("/de-poster.jpg", "de", 1000, 1500, 5.5, 10),
			tmdbImage("/en-poster.jpg", "en", 2000, 3000, 6.0, 20),
			tmdbImage("/fr-poster.jpg", "fr", 1000, 1500, 9.0, 30),
		},
		[]map[string]any{tmdbImage("/clean-backdrop.jpg", nil, 1920, 1080, 5, 5)},
		[]map[string]any{tmdbImage("/de-logo.svg", "de", 800, 300, 5, 5)})
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.itemImages(t, idByTMDB)["poster"]; got.Source != "/de-poster.jpg" || got.Chosen != 0 {
		t.Fatalf("automatic poster = %+v, want the German one", got)
	}

	list, err := e.svc.ImageCandidates(ctx, idByTMDB, "poster", false)
	if err != nil {
		t.Fatal(err)
	}
	var srcs []string
	for _, c := range list.Candidates {
		srcs = append(srcs, c.Source)
		if c.Provider != model.ProviderTMDB || !strings.Contains(c.PreviewURL, "/w300/") {
			t.Errorf("candidate %+v: want TMDB with a w300 preview", c)
		}
	}
	// The poster chain is de, en, "" (F-10): fr has the most votes but stands behind the chain.
	if !slices.Equal(srcs, []string{"/de-poster.jpg", "/en-poster.jpg", "/fr-poster.jpg"}) {
		t.Fatalf("candidates %v, want the chain order", srcs)
	}
	if !list.Candidates[0].Chosen || list.Candidates[1].Chosen {
		t.Errorf("chosen marks %+v, want the decided German poster", list.Candidates[:2])
	}
	// A second call within ten minutes comes from the cache; refresh asks again.
	if _, err := e.svc.ImageCandidates(ctx, idByTMDB, "poster", false); err != nil {
		t.Fatal(err)
	}
	if n := e.tmdb.count("/3/movie/550/images"); n != 1 {
		t.Errorf("TMDB asked %d times, want 1 (cache)", n)
	}
	if _, err := e.svc.ImageCandidates(ctx, idByTMDB, "poster", true); err != nil {
		t.Fatal(err)
	}
	if n := e.tmdb.count("/3/movie/550/images"); n != 2 {
		t.Errorf("TMDB asked %d times after refresh, want 2", n)
	}
	logos, err := e.svc.ImageCandidates(ctx, idByTMDB, "logo", false)
	if err != nil || len(logos.Candidates) != 1 || !strings.HasSuffix(logos.Candidates[0].PreviewURL, "/w300/de-logo.png") {
		t.Fatalf("logo candidates %+v, %v (an SVG logo previews as its PNG twin)", logos, err)
	}

	var verr *model.ValidationError
	if err := e.svc.SetItemImage(ctx, idByTMDB, "poster", model.ProviderTMDB, "/somewhere-else.jpg"); !errors.As(err, &verr) || verr.Fields[0].Code != codeSourceUnknown {
		t.Fatalf("unknown source: %v, want 422 source_unknown", err)
	}
	if _, err := e.svc.ImageCandidates(ctx, idByTMDB, "episode_still", false); !errors.As(err, &verr) {
		t.Fatalf("episode still on a film: %v, want 422", err)
	}
	if _, err := e.svc.ImageCandidates(ctx, "01ZZZZZZZZZZZZZZZZZZZZZZZZ", "poster", false); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("unknown title: %v", err)
	}

	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if err := e.svc.SetItemImage(ctx, idByTMDB, "poster", model.ProviderTMDB, "/en-poster.jpg"); err != nil {
		t.Fatal(err)
	}
	if got := e.itemImages(t, idByTMDB)["poster"]; got.Source != "/en-poster.jpg" || got.Chosen != 1 || got.Lang != "en" {
		t.Fatalf("after the choice %+v", got)
	}
	if r := e.item(t, idByTMDB); !strings.Contains(r.Locked, `"poster"`) {
		t.Fatalf("locked fields %s, want poster", r.Locked)
	}
	if got := e.imageJobs(t); len(got) != 1 || !strings.HasPrefix(got[0], "image:"+idByTMDB+" p7") {
		t.Fatalf("image jobs %v, want one at the manual priority", got)
	}
	imgs, err := e.svc.ItemImages(ctx, idByTMDB)
	if err != nil {
		t.Fatal(err)
	}
	if imgs[0].Kind != "poster" || !imgs[0].Chosen || !imgs[0].Locked || imgs[0].Provider != model.ProviderTMDB || !strings.Contains(imgs[0].PreviewURL, "/w300/en-poster.jpg") {
		t.Fatalf("item images %+v", imgs)
	}

	// DoD: an apply with texts and images keeps the chosen, locked poster.
	id, _, err := e.svc.ApplyMetadata(ctx, ApplyRequest{What: []string{ApplyTexts, ApplyImages}})
	if err != nil {
		t.Fatal(err)
	}
	j, err := e.queue.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleApply(ctx, &j); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	// The queued match child ran by hand above; the fill waits for no job then.
	mustExec(t, e.db.Write, `DELETE FROM jobs WHERE type = ?`, JobMatch)
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.itemImages(t, idByTMDB)["poster"]; got.Source != "/en-poster.jpg" || got.Chosen != 1 {
		t.Fatalf("after apply the poster is %+v, want the chosen English one", got)
	}

	// Lifting the lock: the row and the lock go, a manual match decides again.
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if err := e.svc.ClearItemImage(ctx, idByTMDB, "poster"); err != nil {
		t.Fatal(err)
	}
	if r := e.item(t, idByTMDB); strings.Contains(r.Locked, `"poster"`) {
		t.Fatalf("locked fields %s after clearing", r.Locked)
	}
	if got := e.jobList(t, JobMatch); !slices.Equal(got, []string{"match:" + idByTMDB + " p7"}) {
		t.Fatalf("match jobs %v after clearing", got)
	}
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if got := e.itemImages(t, idByTMDB)["poster"]; got.Source != "/de-poster.jpg" || got.Chosen != 0 {
		t.Fatalf("after clearing %+v, want the automatic German poster again", got)
	}
}

// The preview of a local candidate serves only files the scanner listed for the title (9.2).
func TestLocalImagePreviewServesOnlyListedFiles(t *testing.T) {
	t.Parallel()
	e := newEnv(t, goodToken)
	ctx := context.Background()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "First Film (2001)"), 0o750); err != nil {
		t.Fatal(err)
	}
	poster := jpegBytes(t, 60, 90)
	if err := os.WriteFile(filepath.Join(root, "First Film (2001)", "poster.jpg"), poster, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "First Film (2001)", "secret.jpg"), poster, 0o600); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `INSERT INTO library_roots (id, library_id, path, fs_type, watch, created_at) VALUES (7, 1, ?, 'ext4', 'inotify', 0)`, root)
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	mustExec(t, e.db.Write, `INSERT INTO item_local_images (item_id, kind, rel_path, root_id, width, height, size, mtime) VALUES (?, 'poster', 'First Film (2001)/poster.jpg', 7, 60, 90, ?, 1)`, idByTMDB, len(poster))
	source := "file:First Film (2001)/poster.jpg?" + itoa(len(poster)) + "-1"

	list, err := e.svc.ImageCandidates(ctx, idByTMDB, "poster", false)
	if err != nil || len(list.Candidates) == 0 || list.Candidates[0].Provider != ImageSourceLocal || list.Candidates[0].Source != source {
		t.Fatalf("candidates %+v, %v: the local file first", list, err)
	}
	src, err := e.svc.LocalImagePreview(ctx, idByTMDB, source)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(src.File)
	_ = src.File.Close()
	if err != nil || string(data) != string(poster) || src.MediaType != "image/jpeg" {
		t.Fatalf("preview %d bytes %s, %v", len(data), src.MediaType, err)
	}
	for _, bad := range []string{
		"file:First Film (2001)/secret.jpg?" + itoa(len(poster)) + "-1",
		"file:../../etc/passwd?1-1",
		"file:First Film (2001)/poster.jpg?1-1",
		"/de-poster.jpg",
	} {
		if _, err := e.svc.LocalImagePreview(ctx, idByTMDB, bad); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("preview of %q: %v, want not found", bad, err)
		}
	}
}
