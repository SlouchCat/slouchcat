package visibility

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

func text(lang, title, overview string) scopedq.ItemText {
	return scopedq.ItemText{Lang: lang, Title: title, Overview: sql.NullString{String: overview, Valid: overview != ""}}
}

func TestPickTextsFallback(t *testing.T) {
	both := []scopedq.ItemText{text("en", "Alien", "In space"), text("de", "Alien – Das unheimliche Wesen", "")}
	got := pickTexts(both, "de", "alien")
	if got.title != "Alien – Das unheimliche Wesen" || got.overview != "In space" {
		t.Errorf("de with en overview fallback = %+v", got)
	}
	if got := pickTexts(both, "en", "alien"); got.title != "Alien" {
		t.Errorf("en = %+v", got)
	}
	if got := pickTexts([]scopedq.ItemText{text("en", "Alien", "")}, "de", "alien"); got.title != "Alien" {
		t.Errorf("de without de row = %+v, want the en title", got)
	}
	if got := pickTexts(nil, "de", "alien (1979)"); got.title != "alien (1979)" || got.overview != "" {
		t.Errorf("no rows = %+v, want the parsed title", got)
	}
}

func TestPickGenres(t *testing.T) {
	cases := []struct {
		raw, lang string
		want      []string
	}{
		{`{"de":["Drama"],"en":["Drama","War"]}`, "de", []string{"Drama"}},
		{`{"de":["Drama"],"en":["Drama","War"]}`, "en", []string{"Drama", "War"}},
		{`{"en":["War"]}`, "de", []string{"War"}},
		{`{}`, "de", []string{}},
		{`[]`, "de", []string{}},
		{`["Action"]`, "en", []string{"Action"}},
		{`not json`, "de", []string{}},
	}
	for _, c := range cases {
		got := pickGenres(c.raw, c.lang)
		if got == nil || !slices.Equal(got, c.want) {
			t.Errorf("pickGenres(%s, %s) = %#v, want %#v", c.raw, c.lang, got, c.want)
		}
	}
}

func ids(items []model.ItemSummary) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func TestReaderFiltersByProfile(t *testing.T) {
	f := vistest.New(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	p6, p18, p0 := Profile(f.P6), Profile(f.P18), Profile(f.P0)

	recent, err := r.Recent(ctx, p6, 50)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(6).ID, f.Rated(0).ID}; !slices.Equal(ids(recent), want) {
		t.Errorf("P6 recent = %v, want %v (newest first)", ids(recent), want)
	}
	all, err := r.Recent(ctx, p18, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(f.Movies) {
		t.Errorf("P18 recent has %d titles, want %d (unrated included)", len(all), len(f.Movies))
	}

	for _, m := range f.Movies {
		_, err := r.Item(ctx, p6, m.ID)
		if vistest.VisibleTo(f.P6, m) != (err == nil) {
			t.Errorf("P6 item %s: err = %v", m.Label, err)
		}
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			t.Errorf("P6 item %s: err = %v, want ErrNotFound", m.Label, err)
		}
	}
	if _, err := r.Item(ctx, p18, "01J9000000000000000000ZZZZ"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown item: err = %v", err)
	}

	cont, err := r.ContinueWatching(ctx, p6, 20)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(6).ID}; !slices.Equal(ids(cont), want) {
		t.Errorf("P6 continue = %v, want %v", ids(cont), want)
	}
	if p := cont[0].Progress; p == nil || p.PositionMs != vistest.DurationMs/6 || p.Completed {
		t.Errorf("P6 continue progress = %+v", p)
	}

	picks, err := r.Picks(ctx, p6, 20)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(6).ID}; !slices.Equal(ids(picks), want) {
		t.Errorf("P6 picks = %v, want %v", ids(picks), want)
	}
	picks, err = r.Picks(ctx, p18, 20)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(18).ID, f.Rated(6).ID}; !slices.Equal(ids(picks), want) {
		t.Errorf("P18 picks = %v, want curated order %v", ids(picks), want)
	}
	// No top pick is visible for P0: the placeholder shows the newest titles with a backdrop.
	picks, err = r.Picks(ctx, p0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.Rated(0).ID}; !slices.Equal(ids(picks), want) {
		t.Errorf("P0 placeholder picks = %v, want %v", ids(picks), want)
	}
}

func TestReaderMapsDetailInProfileLanguage(t *testing.T) {
	f := vistest.New(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	m := f.Rated(6)

	de, err := r.Item(ctx, Profile(f.P18), m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if de.Title != "Film 06" || de.Overview != "Handlung 06" || de.Tagline != "Leitsatz 06" || !slices.Equal(de.Genres, []string{"Abenteuer"}) {
		t.Errorf("de detail = %+v", de)
	}
	if de.Rating == nil || *de.Rating != 6 || de.Year == nil || de.RuntimeMinutes == nil || de.TMDBID == nil || de.ReleaseDate == "" || de.OriginalTitle != "Original 06" {
		t.Errorf("de detail fields = %+v", de)
	}
	if de.Poster == nil || de.Poster.Size != "w400" || de.Poster.Width != 10 || de.Backdrop == nil || de.Backdrop.Size != "w1280" || de.Backdrop.ContentHash == "" {
		t.Errorf("images = %+v / %+v", de.Poster, de.Backdrop)
	}
	for name, img := range map[string]struct {
		got  *model.Image
		size string
		w    int
	}{
		"posterSmall": {de.PosterSmall, "w200", 4}, "posterLarge": {de.PosterLarge, "w800", 20}, "backdropLarge": {de.BackdropLarge, "w1920", 24},
	} {
		if img.got == nil || img.got.Size != img.size || img.got.Width != img.w || img.got.ContentHash == "" {
			t.Errorf("%s = %+v", name, img.got)
		}
	}
	if de.Accent != vistest.ItemAccent {
		t.Errorf("accent = %q, want %q", de.Accent, vistest.ItemAccent)
	}
	if de.BackdropSmall == nil || de.BackdropSmall.Size != "w780" || de.BackdropSmall.Width != 8 || de.BackdropSmall.ContentHash == de.Backdrop.ContentHash {
		t.Errorf("small backdrop = %+v", de.BackdropSmall)
	}
	if len(de.MediaFiles) != 1 || de.MediaFiles[0].ID != m.FileID || !de.MediaFiles[0].Available || de.MediaFiles[0].SizeBytes != vistest.FileSize {
		t.Errorf("media files = %+v", de.MediaFiles)
	}
	if de.Progress != nil {
		t.Errorf("P18 has no progress on the 6-rated movie, got %+v", de.Progress)
	}

	en, err := r.Item(ctx, Profile(f.P6), m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if en.Title != "Movie 06" || !slices.Equal(en.Genres, []string{"Adventure"}) || en.Progress == nil {
		t.Errorf("en detail = %+v", en)
	}

	unrated, err := r.Summary(ctx, Profile(f.P18), f.Unrated().ID)
	if err != nil {
		t.Fatal(err)
	}
	if unrated.Rating != nil {
		t.Errorf("unrated movie rating = %v, want nil", *unrated.Rating)
	}
}

func TestReaderMediaFileAndImage(t *testing.T) {
	f := vistest.New(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	m6, m12 := f.Rated(6), f.Rated(12)

	loc, err := r.MediaFile(ctx, Profile(f.P6), m6.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if loc.File.ID != m6.FileID || loc.RootPath != f.MediaDir || loc.RelPath != m6.RelPath {
		t.Errorf("primary file = %+v", loc)
	}
	if _, err := r.MediaFile(ctx, Profile(f.P6), m6.ID, m6.FileID); err != nil {
		t.Errorf("explicit file: %v", err)
	}
	if _, err := r.MediaFile(ctx, Profile(f.P18), m6.ID, m12.FileID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("file of another title: err = %v, want ErrNotFound", err)
	}
	if _, err := r.MediaFile(ctx, Profile(f.P6), m12.ID, ""); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("file of an invisible title: err = %v, want ErrNotFound", err)
	}

	img, err := r.Image(ctx, Profile(f.P6), m6.ID, "poster", "w400", false)
	if err != nil {
		t.Fatal(err)
	}
	if img.RelPath != m6.PosterRel || img.MediaType != "image/jpeg" || img.ContentHash == "" {
		t.Errorf("image = %+v", img)
	}
	webp, err := r.Image(ctx, Profile(f.P6), m6.ID, "poster", "w400", true)
	if err != nil {
		t.Fatal(err)
	}
	if webp.RelPath != m6.PosterWebPRel || webp.MediaType != "image/webp" || webp.ContentHash == img.ContentHash {
		t.Errorf("webp image = %+v", webp)
	}
	// Without a WebP row the JPEG answers a WebP request.
	if _, err := f.DB.Write.Exec(`DELETE FROM images WHERE item_id = ? AND kind = 'poster' AND size = 'w400' AND format = 'webp'`, m6.ID); err != nil {
		t.Fatal(err)
	}
	if fb, err := r.Image(ctx, Profile(f.P6), m6.ID, "poster", "w400", true); err != nil || fb.RelPath != m6.PosterRel || fb.MediaType != "image/jpeg" {
		t.Errorf("webp fallback = %+v, %v", fb, err)
	}
	for _, size := range []string{"w1280", "original", "w500"} {
		if _, err := r.Image(ctx, Profile(f.P6), m6.ID, "poster", size, true); !errors.Is(err, model.ErrNotFound) {
			t.Errorf("poster %s: err = %v, want ErrNotFound", size, err)
		}
	}
	if _, err := r.Image(ctx, Profile(f.P6), m12.ID, "poster", "w400", false); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("image of an invisible title: err = %v", err)
	}
}

func TestReaderRejectsUnscopedReads(t *testing.T) {
	f := vistest.New(t)
	r := New(f.DB, vistest.Logger())
	ctx := context.Background()
	for name, s := range map[string]Scope{"system": System("test"), "zero": {}} {
		if _, err := r.Recent(ctx, s, 10); !errors.Is(err, ErrUnscoped) {
			t.Errorf("%s scope: Recent err = %v, want ErrUnscoped", name, err)
		}
		if _, err := r.Item(ctx, s, f.Rated(0).ID); !errors.Is(err, ErrUnscoped) {
			t.Errorf("%s scope: Item err = %v, want ErrUnscoped", name, err)
		}
	}
}

func TestFixtureIDsMatchContract(t *testing.T) {
	f := vistest.New(t)
	for _, m := range f.Movies {
		if len(m.ID) != 26 || len(m.FileID) != 26 {
			t.Errorf("id %q / %q is not 26 characters long", m.ID, m.FileID)
		}
	}
}
