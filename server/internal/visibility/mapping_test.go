package visibility

import (
	"database/sql"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

func TestSummaryImagesAndAccent(t *testing.T) {
	row := func(kind, size, format, hash string, w int, accent string) scopedq.Image {
		return scopedq.Image{Kind: kind, Size: size, Format: format, ContentHash: hash, Width: int64(w), Height: int64(w * 3 / 2),
			Accent: sql.NullString{String: accent, Valid: accent != ""}}
	}
	rows := []scopedq.Image{
		row("backdrop", "w1280", "jpeg", "b1280j", 1280, ""),
		row("backdrop", "w1280", "webp", "b1280w", 1280, ""),
		row("backdrop", "w780", "jpeg", "b780j", 780, ""), // no WebP: the JPEG hash
		row("poster", "w200", "webp", "p200w", 200, "#123abc"),
		row("poster", "w200", "jpeg", "p200j", 200, "#123abc"),
		row("poster", "w400", "jpeg", "p400j", 400, "#123abc"),
		row("poster", "w400", "webp", "p400w", 400, "#123abc"),
		row("poster", "w500", "jpeg", "m1a", 500, ""), // M1a leftover: never exposed
	}
	s := summarize(scopedq.Item{ID: "i", Kind: "movie", Title: "t"}, extras{images: map[string][]scopedq.Image{"i": rows}}, "de")
	for name, c := range map[string]struct {
		got        string
		size, hash string
		width      int
	}{
		"poster":        {imgKey(s.Poster), "w400", "p400w", 400},
		"posterSmall":   {imgKey(s.PosterSmall), "w200", "p200w", 200},
		"backdrop":      {imgKey(s.Backdrop), "w1280", "b1280w", 1280},
		"backdropSmall": {imgKey(s.BackdropSmall), "w780", "b780j", 780},
	} {
		if want := c.size + "/" + c.hash; c.got != want {
			t.Errorf("%s = %s, want %s", name, c.got, want)
		}
	}
	if s.Poster.Width != 400 || s.Poster.Height != 600 || s.Poster.Kind != "poster" {
		t.Errorf("poster = %+v", s.Poster)
	}
	if s.PosterLarge != nil || s.BackdropLarge != nil {
		t.Errorf("sizes without rows: posterLarge %+v, backdropLarge %+v", s.PosterLarge, s.BackdropLarge)
	}
	if s.Accent != "#123abc" {
		t.Errorf("accent = %q", s.Accent)
	}

	// M1a rows only (before the pipeline ran): the w500 poster stands in for w400, the two backdrop
	// sizes, no accent.
	m1a := []scopedq.Image{row("poster", "w500", "jpeg", "m1a", 500, ""), row("backdrop", "w1280", "jpeg", "b", 1280, "")}
	s = summarize(scopedq.Item{ID: "i", Kind: "movie"}, extras{images: map[string][]scopedq.Image{"i": m1a}}, "de")
	if imgKey(s.Poster) != "w400/m1a" || s.Poster.Width != 500 || s.PosterSmall != nil || s.PosterLarge != nil || s.Backdrop == nil || s.Accent != "" {
		t.Errorf("M1a summary = %+v, poster %+v", s, s.Poster)
	}
}

// B-5: every version is labelled, from the file name or else from the probed height.
func TestVersionLabel(t *testing.T) {
	for _, c := range []struct {
		parsed string
		height int64
		want   string
	}{
		{"Director's Cut", 1080, "Director's Cut"}, {"", 2160, "2160p"}, {"", 2000, "2160p"}, {"", 1080, "1080p"},
		{"", 1000, "1080p"}, {"", 800, "720p"}, {"", 700, "720p"}, {"", 576, "SD"}, {"", 0, ""},
	} {
		if got := versionLabel(c.parsed, c.height); got != c.want {
			t.Errorf("versionLabel(%q, %d) = %q, want %q", c.parsed, c.height, got, c.want)
		}
	}
}

func imgKey(img *model.Image) string {
	if img == nil {
		return "<nil>"
	}
	return img.Size + "/" + img.ContentHash
}
