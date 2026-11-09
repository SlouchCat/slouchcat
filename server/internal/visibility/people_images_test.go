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
// sees every live member: P18, and P12 once the film rated 18 is deleted; never for the manual
// collection without a poster (ADR 0026).
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
	if _, err := f.DB.Write.Exec(`UPDATE items SET deleted_at = 1 WHERE id = ?`, f.Rated(18).ID); err != nil {
		t.Fatal(err)
	}
	if got, err := r.CollectionImage(ctx, Profile(f.P12), vistest.MixedCollectionID, false); err != nil || got.RelPath != f.Discover.MixedImageRel {
		t.Errorf("P12 after the hidden member was deleted = %+v, %v", got, err)
	}
	if _, err := r.CollectionImage(ctx, Profile(f.P6), vistest.MixedCollectionID, false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("P6 still misses the film rated 12: err = %v, want ErrNotFound", err)
	}
}
