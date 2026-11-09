package visibility

import (
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// The portrait of the fixture person (roles in the films rated 6 and 18) answers for a profile that
// sees one of them, in the wanted format; a profile that sees neither, an unknown person and a
// 'missing' row answer ErrNotFound (ADR 0026).
func TestPersonImage(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	r := New(f.DB, vistest.Logger())
	ctx := t.Context()
	for _, p := range []model.Profile{f.P6, f.P18} {
		jpg, err := r.PersonImage(ctx, Profile(p), f.Person.ID, false)
		if err != nil || jpg.RelPath != f.Person.ImageRel || jpg.MediaType != "image/jpeg" || jpg.ContentHash == "" || jpg.ModTime != 1 {
			t.Errorf("profile %d: jpeg = %+v, %v", p.MaxRating, jpg, err)
		}
		webp, err := r.PersonImage(ctx, Profile(p), f.Person.ID, true)
		if err != nil || webp.RelPath != f.Person.ImageWebPRel || webp.MediaType != "image/webp" {
			t.Errorf("profile %d: webp = %+v, %v", p.MaxRating, webp, err)
		}
	}
	if _, err := r.PersonImage(ctx, Profile(f.P0), f.Person.ID, false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("person only in hidden titles: err = %v, want ErrNotFound", err)
	}
	if _, err := r.PersonImage(ctx, Profile(f.P18), 999, false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown person: err = %v, want ErrNotFound", err)
	}
	// Without the WebP row the JPEG answers a WebP request; a 'missing' row answers nothing.
	if _, err := f.DB.Write.Exec(`DELETE FROM person_images WHERE format = 'webp'`); err != nil {
		t.Fatal(err)
	}
	if fb, err := r.PersonImage(ctx, Profile(f.P6), f.Person.ID, true); err != nil || fb.MediaType != "image/jpeg" {
		t.Errorf("webp fallback = %+v, %v", fb, err)
	}
	if _, err := f.DB.Write.Exec(`UPDATE person_images SET status = 'missing', rel_path = NULL, content_hash = NULL, media_type = NULL,
	                                width = NULL, height = NULL, bytes = NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.PersonImage(ctx, Profile(f.P6), f.Person.ID, false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("missing row: err = %v, want ErrNotFound", err)
	}
}

// The TMDB poster of the mixed collection (films rated 6, 12, 18) answers only for a profile that
// sees every member: P18; never for the manual collection without a poster, and for nobody once a
// member is deleted, since a deleted member may be one a child profile must not see (ADR 0026).
func TestCollectionImage(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddDiscover(t)
	r := New(f.DB, vistest.Logger())
	ctx := t.Context()
	jpg, err := r.CollectionImage(ctx, Profile(f.P18), vistest.MixedCollectionID, false)
	if err != nil || jpg.RelPath != f.Discover.MixedImageRel || jpg.MediaType != "image/jpeg" || jpg.ContentHash == "" {
		t.Errorf("P18 jpeg = %+v, %v", jpg, err)
	}
	webp, err := r.CollectionImage(ctx, Profile(f.P18), vistest.MixedCollectionID, true)
	if err != nil || webp.RelPath != f.Discover.MixedImageWebPRel || webp.MediaType != "image/webp" {
		t.Errorf("P18 webp = %+v, %v", webp, err)
	}
	for _, p := range []model.Profile{f.P16, f.P12, f.P6, f.P0} {
		if _, err := r.CollectionImage(ctx, Profile(p), vistest.MixedCollectionID, true); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("profile %d with a hidden member: err = %v, want ErrNotFound", p.MaxRating, err)
		}
	}
	if _, err := r.CollectionImage(ctx, Profile(f.P18), vistest.ManualCollectionID, false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("manual collection: err = %v, want ErrNotFound", err)
	}
	// The fields take the same rule, for several collections in one query (E-7).
	for _, p := range []model.Profile{f.P18, f.P12} {
		refs, err := r.Collections(ctx, Profile(p), MaxRowItems)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range refs {
			if want := c.ID == vistest.MixedCollectionID && p.ID == f.P18.ID; (c.Image != nil) != want {
				t.Errorf("profile %d collection %s image %+v, want set %v", p.MaxRating, c.ID, c.Image, want)
			}
		}
		col, err := r.Collection(ctx, Profile(p), vistest.MixedCollectionID, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if (col.Image != nil) != (p.ID == f.P18.ID) || (col.Image != nil && (col.Image.ContentHash == "" || col.Image.Width != 8)) {
			t.Errorf("profile %d collection page image %+v", p.MaxRating, col.Image)
		}
	}
	if _, err := f.DB.Write.Exec(`UPDATE items SET deleted_at = 1 WHERE id = ?`, f.Rated(18).ID); err != nil {
		t.Fatal(err)
	}
	// The child profile P12 sees both live members now, yet the poster stays hidden: the endpoint
	// and the fields alike.
	for _, p := range []model.Profile{f.P12, f.P18, f.P6} {
		if _, err := r.CollectionImage(ctx, Profile(p), vistest.MixedCollectionID, false); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("profile %d after the member rated 18 was deleted: err = %v, want ErrNotFound", p.MaxRating, err)
		}
	}
	for _, p := range []model.Profile{f.P12, f.P18} {
		col, err := r.Collection(ctx, Profile(p), vistest.MixedCollectionID, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if col.Image != nil {
			t.Errorf("profile %d collection page image %+v after a member was deleted, want none", p.MaxRating, col.Image)
		}
		refs, err := r.Collections(ctx, Profile(p), MaxRowItems)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range refs {
			if c.Image != nil {
				t.Errorf("profile %d collection %s image %+v after a member was deleted, want none", p.MaxRating, c.ID, c.Image)
			}
		}
	}
}

// The portrait comes with the cast of a title, the person and the people of a search, from the
// LEFT JOIN on the 'ok' JPEG row; with or without image rows the lists keep their length (m-3).
func TestPersonPortraitFields(t *testing.T) {
	t.Parallel()
	f := vistest.New(t)
	f.AddSeries(t)
	f.AddDiscover(t)
	r := New(f.DB, vistest.Logger())
	ctx := t.Context()
	m6 := f.Rated(6).ID
	type counts struct{ cast, credits, people int }
	read := func(wantImage bool) counts {
		t.Helper()
		d, err := r.Item(ctx, Profile(f.P18), m6)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range d.Cast {
			if c.PersonID == f.Person.ID && (c.Image != nil) != wantImage {
				t.Errorf("cast image %+v, want set %v", c.Image, wantImage)
			}
		}
		p, err := r.Person(ctx, Profile(f.P18), f.Person.ID)
		if err != nil {
			t.Fatal(err)
		}
		if (p.Image != nil) != wantImage || (wantImage && (p.Image.Width != 6 || p.Image.Height != 9 || p.Image.ContentHash == "")) {
			t.Errorf("person image %+v, want set %v", p.Image, wantImage)
		}
		res, err := r.Search(ctx, Profile(f.P18), model.SearchQuery{Text: "Pat", MaxRating: -1})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.People) == 0 {
			t.Fatal("search found no person")
		}
		for _, sp := range res.People {
			if sp.ID == f.Person.ID && (sp.Image != nil) != wantImage {
				t.Errorf("search person image %+v, want set %v", sp.Image, wantImage)
			}
		}
		return counts{len(d.Cast), len(p.Credits), len(res.People)}
	}
	with := read(true)
	// A 'missing' row, like no row, gives no image.
	if _, err := f.DB.Write.Exec(`DELETE FROM person_images WHERE format = 'webp'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.Write.Exec(`UPDATE person_images SET status = 'missing', rel_path = NULL, content_hash = NULL, media_type = NULL,
	                                width = NULL, height = NULL, bytes = NULL`); err != nil {
		t.Fatal(err)
	}
	missing := read(false)
	if _, err := f.DB.Write.Exec(`DELETE FROM person_images`); err != nil {
		t.Fatal(err)
	}
	without := read(false)
	if with != without || missing != without {
		t.Errorf("rows with images %+v, missing %+v, without %+v; want equal", with, missing, without)
	}
}
