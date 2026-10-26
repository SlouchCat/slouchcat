package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Images of the administration (M11 step 5b, plan 05-plan-spotlight.md 5b-a and 5b-b):
// getAdminItemImage is system-wide, and the posterUrl of picks and spotlight entries carries the
// content hash but no profile id.

// adminImageFile reads an image of the fixture's cache in the layout of vistest.addImage.
func adminImageFile(t *testing.T, f *vistest.Fixture, itemID, kind, size, ext string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.CacheDir, "images", itemID, kind+"-"+size+"."+ext))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// hiddenFromP18 returns a fixture title that the primary profile P18 may not see and that is not
// deleted (its only file is missing).
func hiddenFromP18(t *testing.T, f *vistest.Fixture) vistest.Movie {
	t.Helper()
	for _, m := range f.Movies {
		if !m.Deleted && !f.VisibleTo(f.P18, m) {
			return m
		}
	}
	t.Fatal("fixture has no title hidden from P18")
	return vistest.Movie{}
}

// DoD 5b "Route ist systemweit" and "Poster ohne Bild": the owner with the primary profile gets
// the image of a title that profile may not see (the scoped route answers 404 for it), as JPEG or
// as WebP by Accept, cacheable for good; without a session 401, with another profile 403; an
// unknown title, a title without poster, a deleted title and a size outside the contract 404.
func TestContractAdminItemImage(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	m := hiddenFromP18(t, f)
	path := BasePath + "/admin/items/" + m.ID + "/images/poster/w200"

	if rec := send(t, h, http.MethodGet, BasePath+"/items/"+m.ID+"/images/poster/w200", "p18", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("scoped image of a title hidden from P18: status %d, want 404", rec.Code)
	}
	for _, c := range []struct {
		accept, wantType string
		want             []byte
	}{
		{"", "image/jpeg", adminImageFile(t, f, m.ID, "poster", "w200", "jpg")},
		{"image/avif,image/webp,*/*", "image/webp", adminImageFile(t, f, m.ID, "poster", "w200", "webp")},
	} {
		var header map[string]string
		if c.accept != "" {
			header = map[string]string{"Accept": c.accept}
		}
		rec := send(t, h, http.MethodGet, path, "p18", "", header)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != c.wantType || !bytes.Equal(rec.Body.Bytes(), c.want) {
			t.Errorf("Accept %q: status %d, type %q", c.accept, rec.Code, rec.Header().Get("Content-Type"))
		}
		if v := rec.Header().Values("Vary"); !containsToken(v, "Accept") {
			t.Errorf("Accept %q: Vary = %q, want Accept", c.accept, v)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != "private, max-age=31536000, immutable" {
			t.Errorf("Accept %q: Cache-Control = %q", c.accept, cc)
		}
	}
	rec := send(t, h, http.MethodGet, BasePath+"/admin/items/"+m.ID+"/images/backdrop/w1280", "p18", "", nil)
	if want := adminImageFile(t, f, m.ID, "backdrop", "w1280", "jpg"); rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), want) {
		t.Errorf("backdrop w1280: status %d", rec.Code)
	}

	if rec := send(t, h, http.MethodGet, path, "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("without a session: status %d, want 401", rec.Code)
	}
	for _, token := range []string{"p12", "p6", "pg", "none"} {
		if rec := send(t, h, http.MethodGet, path, token, "", nil); rec.Code != http.StatusForbidden || problemCode(t, rec) != "forbidden" {
			t.Errorf("%s: status %d (%s), want 403 forbidden", token, rec.Code, rec.Body.String())
		}
	}

	f.AddSpotlightFilms(t, 1) // a title without any image
	var deleted string
	for _, d := range f.Movies {
		if d.Deleted {
			deleted = d.ID
		}
	}
	for name, p := range map[string]string{
		"unknown title":      "/admin/items/" + unknownItemID + "/images/poster/w200",
		"title w/o poster":   "/admin/items/01J9SPOTLIGHT0000000000001/images/poster/w200",
		"deleted title":      "/admin/items/" + deleted + "/images/poster/w200",
		"size of no kind":    "/admin/items/" + m.ID + "/images/poster/w1280",
		"kind not in enum":   "/admin/items/" + m.ID + "/images/logo/w200",
		"kept original":      "/admin/items/" + m.ID + "/images/poster/original",
		"legacy poster w500": "/admin/items/" + m.ID + "/images/poster/w500",
	} {
		if rec := send(t, h, http.MethodGet, BasePath+p, "p18", "", nil); rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
			t.Errorf("%s: status %d (%s), want 404 not_found", name, rec.Code, rec.Body.String())
		}
	}
}

// checkAdminPosterURL checks DoD 5b "Keine Profil-Id in der Adresse": the poster URL of an
// administration row names getAdminItemImage with the content hash of the w200 JPEG and no p, and
// the owner with the primary profile can load it.
func checkAdminPosterURL(t *testing.T, pf profileFixture, label, itemID, posterURL string) {
	t.Helper()
	sum := sha256.Sum256(adminImageFile(t, pf.f, itemID, "poster", "w200", "jpg"))
	if want := BasePath + "/admin/items/" + itemID + "/images/poster/w200?v=" + hex.EncodeToString(sum[:]); posterURL != want {
		t.Errorf("%s: posterUrl %q, want %q", label, posterURL, want)
	}
	u, err := url.Parse(posterURL)
	if err != nil {
		t.Fatal(err)
	}
	if q := u.Query(); q.Has("p") || strings.Contains(posterURL, "&p=") {
		t.Errorf("%s: posterUrl %q carries a profile id", label, posterURL)
	}
	if rec := send(t, pf.h, http.MethodGet, posterURL, "p18", "", nil); rec.Code != http.StatusOK {
		t.Errorf("%s: posterUrl %q answers %d", label, posterURL, rec.Code)
	}
}

// Counterpart of TestImageURLsCarryProfileID for the administration: every pick of a title with a
// poster carries a posterUrl without profile id, also when the active profile may not see the
// title; a deleted title has none. Spotlight entries carry one for a library hit with a poster
// only.
func TestAdminImageURLsCarryNoProfileID(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	hidden := hiddenFromP18(t, f)
	deleted := map[string]bool{}
	for _, m := range f.Movies {
		deleted[m.ID] = m.Deleted
	}

	rec := send(t, h, http.MethodGet, BasePath+"/admin/picks", "p18", "", nil)
	adminCurationAnswer(t, rec, http.StatusOK)
	var picks []gen.AdminPick
	decode(t, rec, &picks)
	sawHidden, sawDeleted := false, false
	for _, p := range picks {
		if deleted[p.ItemId] {
			sawDeleted = true
			if p.PosterUrl != nil {
				t.Errorf("pick %d of a deleted title has posterUrl %q", p.Id, *p.PosterUrl)
			}
			continue
		}
		if p.PosterUrl == nil {
			t.Errorf("pick %d (%s) has no posterUrl", p.Id, p.ItemId)
			continue
		}
		sawHidden = sawHidden || p.ItemId == hidden.ID
		checkAdminPosterURL(t, pf, "pick "+p.ItemId, p.ItemId, *p.PosterUrl)
	}
	if !sawHidden || !sawDeleted {
		t.Errorf("picks lack the hidden title %s or a deleted title: hidden %v, deleted %v", hidden.ID, sawHidden, sawDeleted)
	}

	// Spotlight: the film rated 12 (TMDB id 1002) is a hit with poster, spotlight film 1 (8001) a
	// hit without image, 9999 no hit.
	f.AddSpotlightFilms(t, 1)
	film := f.Rated(12)
	if _, err := f.DB.Write.Exec(`INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, title, fetched_at) VALUES
	      ('trending', 'movie', 1, 1002, 'Film', 1), ('trending', 'movie', 2, 8001, 'Spotlight', 1), ('trending', 'movie', 3, 9999, 'Elsewhere', 1)`); err != nil {
		t.Fatal(err)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/admin/spotlight/entries", "p18", "", nil)
	adminCurationAnswer(t, rec, http.StatusOK)
	var entries gen.SpotlightEntries
	decode(t, rec, &entries)
	if len(entries.Entries) != 3 {
		t.Fatalf("spotlight entries %s", rec.Body.String())
	}
	for _, e := range entries.Entries {
		switch e.TmdbId {
		case 1002:
			if e.ItemId == nil || *e.ItemId != film.ID || e.PosterUrl == nil {
				t.Errorf("hit with poster: %+v", e)
				continue
			}
			checkAdminPosterURL(t, pf, "spotlight 1002", film.ID, *e.PosterUrl)
		default:
			if e.PosterUrl != nil {
				t.Errorf("entry %d without poster has posterUrl %q", e.TmdbId, *e.PosterUrl)
			}
		}
	}
}
