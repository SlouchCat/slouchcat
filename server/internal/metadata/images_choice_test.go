package metadata

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The image choice of concept 10 section 6.2 (ADR 0018): the six steps over the fixed source
// order, the chain outside and the sources inside, the vote rule, the minimum width, and the
// cases the paper names as fixtures (TVDB-de beats TMDB-en; a local file stays after a second
// fill; a local file below the minimum width loses).

func cand(kind, lang string, w int, votes float64, count int, src string) ImageCandidate {
	return ImageCandidate{Kind: kind, Lang: lang, Width: w, Height: w * 3 / 2, Votes: votes, VoteCount: count, Source: src}
}

func TestChooseImageSteps(t *testing.T) {
	t.Parallel()
	st := DefaultSettings() // poster chain de, en, ""; min width 500; by vote
	tmdb := sourceCandidates{source: "tmdb", images: []ImageCandidate{
		cand("poster", "en", 2000, 7.0, 10, "/en-big.jpg"),
		cand("poster", "en", 2000, 8.5, 2, "/en-two-votes.jpg"), // two votes: the vote does not count
		cand("poster", "de", 300, 9.0, 50, "/de-small.jpg"),     // below the minimum width
		cand("poster", "", 1500, 6.0, 20, "/notext.jpg"),
		{Kind: "poster", Lang: "", Width: 1000, Source: "/default.jpg", Default: true},
	}}
	tvdb := sourceCandidates{source: "tvdb", images: []ImageCandidate{
		cand("poster", "de", 680, 100, 0, "https://art/de.jpg"), // the German TVDB poster
	}}

	// Step 3, chain outside: de comes first in the chain, TVDB stands behind TMDB in the list
	// and has the only wide German poster: it wins ("TVDB-de schlägt TMDB-en").
	got, ok := chooseImage("poster", st, []sourceCandidates{tmdb, tvdb})
	if !ok || got.source != "tvdb" || got.cand.Source != "https://art/de.jpg" {
		t.Fatalf("de chain: %+v %v", got, ok)
	}
	// Without TVDB: no German poster wide enough, so English; by vote the 10-vote one beats the
	// better-rated one with two votes.
	got, ok = chooseImage("poster", st, []sourceCandidates{tmdb})
	if !ok || got.cand.Source != "/en-big.jpg" {
		t.Fatalf("vote rule: %+v %v", got, ok)
	}
	// Without the vote rule the first English candidate in source order wins.
	noVote := st
	noVote.ImageByVote = false
	got, _ = chooseImage("poster", noVote, []sourceCandidates{tmdb})
	if got.cand.Source != "/en-big.jpg" {
		t.Fatalf("first in order: %+v", got)
	}
	// Step 4: a chain nobody meets with the width is run again without it; then the small
	// German TMDB poster with 50 votes beats TVDB's (a score, no votes).
	highBar := DefaultSettings()
	highBar.ImageMinWidth["poster"] = 5000
	got, ok = chooseImage("poster", highBar, []sourceCandidates{tmdb, tvdb})
	if !ok || got.cand.Source != "/de-small.jpg" {
		t.Fatalf("without width: %+v %v", got, ok)
	}
	// Step 5: nothing in the chain at all (a chain of a language nobody has): the default.
	onlyJa := DefaultSettings()
	onlyJa.ImageLangs["poster"] = []string{"ja"}
	got, ok = chooseImage("poster", onlyJa, []sourceCandidates{tmdb, tvdb})
	if !ok || !got.cand.Default || got.cand.Source != "/default.jpg" {
		t.Fatalf("default: %+v %v", got, ok)
	}
	// Step 6: no candidate of the kind.
	if _, ok := chooseImage("logo", st, []sourceCandidates{tmdb, tvdb}); ok {
		t.Fatal("a kind nobody offers should stay empty")
	}
	// Step 2: a local file wide enough wins before every provider; a small one does not (O-1).
	local := sourceCandidates{source: ImageSourceLocal, images: []ImageCandidate{cand("poster", "", 800, 0, 0, "file:Film/poster.jpg?1-2")}}
	got, _ = chooseImage("poster", st, []sourceCandidates{local, tmdb, tvdb})
	if got.source != ImageSourceLocal {
		t.Fatalf("local first: %+v", got)
	}
	small := sourceCandidates{source: ImageSourceLocal, images: []ImageCandidate{cand("poster", "", 300, 0, 0, "file:Film/poster.jpg?1-2")}}
	got, _ = chooseImage("poster", st, []sourceCandidates{small, tmdb, tvdb})
	if got.source != "tvdb" {
		t.Fatalf("small local file should lose to the chain: %+v", got)
	}
}

func TestDecideImagesHonoursLocksChoicesAndSwitches(t *testing.T) {
	t.Parallel()
	st := DefaultSettings()
	cands := []providerFields{{provider: "tmdb", images: []ImageCandidate{
		cand("poster", "de", 1000, 7, 10, "/de.jpg"), cand("backdrop", "", 1920, 7, 10, "/bd.jpg"), cand("logo", "de", 800, 7, 10, "/logo.png"),
	}}}
	stored := []systemq.ItemImage{{ItemID: "x", Kind: "poster", Provider: "tmdb", Source: "/old.jpg"}, {ItemID: "x", Kind: "logo", Provider: ImageSourceLocal, Source: "file:l.png?1-1", Chosen: 1}}
	decisions, empty := decideImages("movie", st, []string{"tmdb"}, cands, nil, stored, map[string]bool{"backdrop": true, "logo": true})
	if len(empty) != 0 {
		t.Errorf("empty = %v", empty)
	}
	if len(decisions) != 1 || decisions[0].kind != "poster" || !decisions[0].changed || decisions[0].cand.Source != "/de.jpg" {
		t.Fatalf("decisions = %+v, want the poster only (backdrop locked, logo chosen)", decisions)
	}
	// The same decision again is not a change.
	stored[0].Source = "/de.jpg"
	decisions, _ = decideImages("movie", st, []string{"tmdb"}, cands, nil, stored, map[string]bool{"backdrop": true, "logo": true})
	if len(decisions) != 1 || decisions[0].changed {
		t.Fatalf("second run: %+v", decisions)
	}
	// The logo switched off: not decided; a season poster only when its switch is on.
	noLogo := DefaultSettings()
	noLogo.ImageKinds = []string{ImageKindSeasonPoster}
	decisions, _ = decideImages("movie", noLogo, []string{"tmdb"}, cands, nil, nil, nil)
	for _, d := range decisions {
		if d.kind == "logo" {
			t.Error("logo decided although switched off")
		}
	}
	if kinds := imageKindsOf("episode", noLogo); kinds != nil {
		t.Errorf("episode still switched off: %v", kinds)
	}
	if kinds := imageKindsOf("season", noLogo); len(kinds) != 1 {
		t.Errorf("season poster switched on: %v", kinds)
	}
	// Local rows become candidates of the source "local" with the file: address.
	local := []systemq.ItemLocalImage{{ItemID: "x", Kind: "poster", RelPath: "Film/poster.jpg", Width: 1000, Height: 1500, Size: 10, Mtime: 20}}
	decisions, _ = decideImages("movie", st, []string{"tmdb"}, cands, local, nil, nil)
	for _, d := range decisions {
		if d.kind == "poster" && (d.source != ImageSourceLocal || d.cand.Source != "file:Film/poster.jpg?10-20") {
			t.Errorf("local poster: %+v", d)
		}
	}
	// "local bleibt nach dem zweiten fill": the stored local row is the same decision again.
	stored = []systemq.ItemImage{{ItemID: "x", Kind: "poster", Provider: ImageSourceLocal, Source: "file:Film/poster.jpg?10-20"}}
	decisions, _ = decideImages("movie", st, []string{"tmdb"}, cands, local, stored, nil)
	for _, d := range decisions {
		if d.kind == "poster" && d.changed {
			t.Errorf("local poster replaced on the second run: %+v", d)
		}
	}
	// Local files switched off: the provider decides.
	noLocal := DefaultSettings()
	noLocal.LocalImages = false
	decisions, _ = decideImages("movie", noLocal, []string{"tmdb"}, cands, local, nil, nil)
	for _, d := range decisions {
		if d.kind == "poster" && d.source != "tmdb" {
			t.Errorf("local switched off: %+v", d)
		}
	}
}

// A match with the images block: the German poster wins over the English one, the backdrop
// without text, the logo is written as a kind of its own; the pipeline derives PNG and WebP for
// the logo (transparency kept) and the SVG twin is asked for as PNG (O-3).
func TestMatchAndFetchWithLogoAndLanguageChain(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	ctx := context.Background()
	e.tmdb.setImages("550",
		[]map[string]any{tmdbImage("/poster550.jpg", "en", 2000, 3000, 7.0, 20), tmdbImage("/poster550-de.jpg", "de", 1000, 1500, 6.0, 5)},
		[]map[string]any{tmdbImage("/backdrop550.jpg", nil, 1920, 1080, 7.0, 20), tmdbImage("/backdrop550-en.jpg", "en", 1920, 1080, 9.0, 50)},
		[]map[string]any{tmdbImage("/logo550.svg", "de", 800, 300, 5.0, 4), tmdbImage("/logo550-en.png", "en", 800, 300, 5.0, 4)})
	e.tmdb.serveImage("/t/p/original/poster550-de.jpg", e.tmdb.poster, "image/jpeg")
	e.tmdb.serveImage("/t/p/original/logo550.png", alphaPNG(t, 800, 300), "image/png")
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	chosen := e.itemImages(t, idByTMDB)
	if chosen["poster"].Source != "/poster550-de.jpg" || chosen["poster"].Lang != "de" || chosen["poster"].Provider != "tmdb" {
		t.Errorf("poster = %+v, want the German one", chosen["poster"])
	}
	if chosen["backdrop"].Source != "/backdrop550.jpg" || chosen["backdrop"].Lang != "" {
		t.Errorf("backdrop = %+v, want the one without text (chain '', de, en)", chosen["backdrop"])
	}
	if chosen["logo"].Source != "/logo550.svg" || chosen["logo"].Lang != "de" {
		t.Errorf("logo = %+v, want the German SVG", chosen["logo"])
	}
	if n := e.tmdb.count("/3/movie/550"); n != 2 {
		t.Errorf("movie requests = %d, want one per language (the images ride on the record language)", n)
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	imgs := e.images(t, idByTMDB)
	if e.tmdb.count("/t/p/original/logo550.png") != 1 || e.tmdb.count("/t/p/original/logo550.svg") != 0 {
		t.Errorf("the SVG logo should be fetched as its PNG twin (png %d, svg %d)", e.tmdb.count("/t/p/original/logo550.png"), e.tmdb.count("/t/p/original/logo550.svg"))
	}
	for _, key := range []string{"logo/original/jpeg", "logo/w400/png", "logo/w400/webp", "logo/w800/png", "logo/w800/webp"} {
		if _, ok := imgs[key]; !ok {
			t.Errorf("logo row %s missing (have %d rows)", key, len(imgs))
		}
	}
	if _, ok := imgs["logo/w400/jpeg"]; ok {
		t.Error("a logo must not be derived as JPEG")
	}
	if imgs["logo/original/jpeg"].MediaType != "image/png" || imgs["logo/w400/png"].MediaType != "image/png" {
		t.Errorf("logo media types %q %q", imgs["logo/original/jpeg"].MediaType, imgs["logo/w400/png"].MediaType)
	}
	// Transparency survives: the derived PNG has an alpha channel (colour type 6).
	data, err := os.ReadFile(filepath.Join(e.cache, imgs["logo/w400/png"].RelPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 26 || data[25] != 6 {
		t.Errorf("derived logo PNG colour type %d, want 6 (RGBA)", data[25])
	}
	if imgs["poster/original/jpeg"].Source != e.tmdb.srv.URL+"/t/p/original/poster550-de.jpg" {
		t.Errorf("poster original source %q", imgs["poster/original/jpeg"].Source)
	}
	var lang, provider string
	if err := e.db.Read.QueryRow(`SELECT lang, provider FROM images WHERE item_id = ? AND kind = 'poster' AND size = 'w400' AND format = 'webp'`, idByTMDB).Scan(&lang, &provider); err != nil || lang != "de" || provider != "tmdb" {
		t.Errorf("poster rows carry lang %q provider %q (%v), want de tmdb", lang, provider, err)
	}

	// The logo switched off: the next fetch removes its rows and files.
	mustExec(t, e.db.Write, `INSERT INTO settings (key, value, updated_at) VALUES (?, '["season_poster","episode_still"]', 0)`, SettingImageKinds)
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if imgs := e.images(t, idByTMDB); imgs["logo/w400/png"].RelPath != "" {
		t.Error("logo rows should be gone after the switch")
	}
	if _, err := os.Stat(filepath.Join(e.cache, "images", idByTMDB, "logo-w400.png")); !os.IsNotExist(err) {
		t.Errorf("logo file should be gone: %v", err)
	}
}

// A local poster.jpg next to the film (item_local_images, written by the scanner) beats every
// provider and is read through the root of the library folder; a replaced file (new size/mtime)
// is a new source for image.fetch.
func TestLocalImageBeatsTheProviders(t *testing.T) {
	t.Parallel()
	requireFFmpeg(t)
	e := newEnv(t, goodToken)
	ctx := context.Background()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "First Film (2001)"), 0o750); err != nil {
		t.Fatal(err)
	}
	poster := jpegBytes(t, 600, 900)
	if err := os.WriteFile(filepath.Join(root, "First Film (2001)", "poster.jpg"), poster, 0o600); err != nil {
		t.Fatal(err)
	}
	mustExec(t, e.db.Write, `INSERT INTO library_roots (id, library_id, path, fs_type, watch, created_at) VALUES (7, 1, ?, 'ext4', 'inotify', 0)`, root)
	e.addItem(t, itemSeed{id: idByTMDB, title: "First Film", tmdbID: 550})
	mustExec(t, e.db.Write, `INSERT INTO item_local_images (item_id, kind, rel_path, root_id, width, height, size, mtime) VALUES (?, 'poster', 'First Film (2001)/poster.jpg', 7, 600, 900, ?, 1)`, idByTMDB, len(poster))
	if err := e.svc.match(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	chosen := e.itemImages(t, idByTMDB)
	if chosen["poster"].Provider != ImageSourceLocal || chosen["poster"].Source != "file:First Film (2001)/poster.jpg?"+itoa(len(poster))+"-1" {
		t.Fatalf("poster = %+v, want the local file", chosen["poster"])
	}
	if chosen["backdrop"].Provider != "tmdb" {
		t.Fatalf("backdrop = %+v, want TMDB (no local fanart)", chosen["backdrop"])
	}
	if err := e.svc.fetchImages(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	imgs := e.images(t, idByTMDB)
	if imgs["poster/original/jpeg"].Hash != sha(poster) || e.tmdb.count("/t/p/original/poster550.jpg") != 0 {
		t.Fatalf("the poster original should be the local file (hash %s, tmdb downloads %d)", imgs["poster/original/jpeg"].Hash, e.tmdb.count("/t/p/original/poster550.jpg"))
	}
	e.checkPipelineKind(t, idByTMDB, "poster")

	// A second fill keeps the local choice (the fixture case of 6.2).
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if again := e.itemImages(t, idByTMDB); again["poster"].Source != chosen["poster"].Source {
		t.Fatalf("local poster replaced by the fill: %+v", again["poster"])
	}

	// Local files switched off: the fill goes back to the provider.
	mustExec(t, e.db.Write, `INSERT INTO settings (key, value, updated_at) VALUES (?, '0', 0)`, SettingLocalImages)
	mustExec(t, e.db.Write, `UPDATE items SET providers = '["tmdb","tvdb"]' WHERE id = ?`, idByTMDB)
	if err := e.svc.fill(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if after := e.itemImages(t, idByTMDB); after["poster"].Provider != "tmdb" {
		t.Fatalf("poster with local files off = %+v, want tmdb", after["poster"])
	}
}

// itemImages returns the decided image rows of a title by kind.
func (e *env) itemImages(t *testing.T, id string) map[string]systemq.ItemImage {
	t.Helper()
	rows, err := e.db.Read.Query(`SELECT kind, provider, lang, source, chosen FROM item_images WHERE item_id = ?`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]systemq.ItemImage{}
	for rows.Next() {
		r := systemq.ItemImage{ItemID: id}
		if err := rows.Scan(&r.Kind, &r.Provider, &r.Lang, &r.Source, &r.Chosen); err != nil {
			t.Fatal(err)
		}
		out[r.Kind] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// checkPipelineKind checks that every derived size and format of a kind has a row with a file.
func (e *env) checkPipelineKind(t *testing.T, id, kind string) {
	t.Helper()
	imgs := e.images(t, id)
	for _, w := range imageWidths[kind] {
		for _, f := range formatsOf(kind) {
			key := kind + "/" + sizeName(w) + "/" + string(f.format)
			r, ok := imgs[key]
			if !ok {
				t.Errorf("%s: no row", key)
				continue
			}
			if _, err := os.Stat(filepath.Join(e.cache, r.RelPath)); err != nil {
				t.Errorf("%s: file %s: %v", key, r.RelPath, err)
			}
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// alphaPNG is a w×h PNG with a transparency gradient (a logo).
func alphaPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.NRGBA{R: 240, G: 200, B: 40, A: uint8(255 * x / w)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
