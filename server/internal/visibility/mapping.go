package visibility

import (
	"encoding/json"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Image sizes of the M1b pipeline (plan 3.1) as summaries expose them. Every size exists as WebP
// and JPEG; the reference carries the WebP row's hash when there is one (the hash only busts
// caches, the format is negotiated per request).
const (
	posterSmallSize   = "w200"
	posterSize        = "w400"
	posterLargeSize   = "w800"
	backdropSmallSize = "w780"
	backdropSize      = "w1280"
	backdropLargeSize = "w1920"
)

// fallbackLang is the second choice for texts after the profile language (B-3).
const fallbackLang = "en"

// texts picks the profile language, then English, field by field.
type texts struct {
	title, overview, tagline string
}

// pickTexts returns the title, overview and tagline in lang with a fallback to English per field.
// The title falls back to the parsed title of the item when no text row has one.
func pickTexts(rows []scopedq.ItemText, lang, parsedTitle string) texts {
	var primary, fallback *scopedq.ItemText
	for i := range rows {
		switch rows[i].Lang {
		case lang:
			primary = &rows[i]
		case fallbackLang:
			fallback = &rows[i]
		}
	}
	first := func(get func(*scopedq.ItemText) string) string {
		for _, r := range []*scopedq.ItemText{primary, fallback} {
			if r != nil {
				if v := get(r); v != "" {
					return v
				}
			}
		}
		return ""
	}
	out := texts{
		title:    first(func(t *scopedq.ItemText) string { return t.Title }),
		overview: first(func(t *scopedq.ItemText) string { return t.Overview.String }),
		tagline:  first(func(t *scopedq.ItemText) string { return t.Tagline.String }),
	}
	if out.title == "" {
		out.title = parsedTitle
	}
	return out
}

// pickGenres reads the genres column: a JSON object {"de": [...], "en": [...]} written by the
// metadata package, or a plain array before matching. It returns the profile language, then
// English, and never nil.
func pickGenres(raw, lang string) []string {
	var byLang map[string][]string
	if err := json.Unmarshal([]byte(raw), &byLang); err == nil {
		for _, l := range []string{lang, fallbackLang} {
			if g := byLang[l]; len(g) > 0 {
				return g
			}
		}
		return []string{}
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err == nil && list != nil {
		return list
	}
	return []string{}
}

// pickImage returns the image of kind in exactly the given size, or nil. Of the two formats it
// takes the WebP row's hash, else the JPEG row's; width and height are the same for both.
func pickImage(rows []scopedq.Image, kind, size string) *model.Image {
	var chosen *scopedq.Image
	for i := range rows {
		r := &rows[i]
		if r.Kind != kind || r.Size != size {
			continue
		}
		if chosen == nil || r.Format == "webp" {
			chosen = r
		}
	}
	if chosen == nil {
		return nil
	}
	return &model.Image{Kind: kind, Size: size, ContentHash: chosen.ContentHash, Width: int(chosen.Width), Height: int(chosen.Height)}
}

// pickAccent returns the accent colour stored on the poster rows (image.fetch writes the same
// value to every derived poster row), or "".
func pickAccent(rows []scopedq.Image) string {
	for i := range rows {
		if rows[i].Kind == "poster" && rows[i].Accent.Valid {
			return rows[i].Accent.String
		}
	}
	return ""
}

func summarize(it scopedq.Item, ex extras, lang string) model.ItemSummary {
	s := model.ItemSummary{
		ID:            it.ID,
		Kind:          it.Kind,
		Title:         pickTexts(ex.texts[it.ID], lang, it.Title).title,
		Poster:        pickImage(ex.images[it.ID], "poster", posterSize),
		PosterSmall:   pickImage(ex.images[it.ID], "poster", posterSmallSize),
		PosterLarge:   pickImage(ex.images[it.ID], "poster", posterLargeSize),
		Backdrop:      pickImage(ex.images[it.ID], "backdrop", backdropSize),
		BackdropSmall: pickImage(ex.images[it.ID], "backdrop", backdropSmallSize),
		BackdropLarge: pickImage(ex.images[it.ID], "backdrop", backdropLargeSize),
		Accent:        pickAccent(ex.images[it.ID]),
	}
	if it.Year.Valid {
		y := int(it.Year.Int64)
		s.Year = &y
	}
	if it.RuntimeMinutes.Valid {
		m := int(it.RuntimeMinutes.Int64)
		s.RuntimeMinutes = &m
	}
	if it.EffectiveRating.Valid {
		r := int(it.EffectiveRating.Int64)
		s.Rating = &r
	}
	if p, ok := ex.progress[it.ID]; ok {
		s.Progress = &model.Progress{
			PositionMs: p.PositionMs,
			DurationMs: p.DurationMs,
			Completed:  p.Completed == 1,
			UpdatedAt:  time.UnixMilli(p.UpdatedAt).UTC(),
		}
	}
	return s
}

func detail(it scopedq.Item, ex extras, lang string) model.ItemDetail {
	t := pickTexts(ex.texts[it.ID], lang, it.Title)
	d := model.ItemDetail{
		ItemSummary:   summarize(it, ex, lang),
		OriginalTitle: it.OriginalTitle.String,
		Tagline:       t.tagline,
		Overview:      t.overview,
		ReleaseDate:   it.ReleaseDate.String,
		Genres:        pickGenres(it.Genres, lang),
	}
	if it.TmdbID.Valid {
		id := it.TmdbID.Int64
		d.TMDBID = &id
	}
	return d
}

func mediaFile(f scopedq.ListMediaFilesForItemRow) model.MediaFile {
	return model.MediaFile{
		ID:           f.ID,
		Container:    f.Container,
		VideoCodec:   f.VideoCodec.String,
		AudioCodec:   f.AudioCodec.String,
		Width:        int(f.Width.Int64),
		Height:       int(f.Height.Int64),
		DurationMs:   f.DurationMs.Int64,
		SizeBytes:    f.Size,
		VersionLabel: f.VersionLabel.String,
		Available:    !f.MissingSince.Valid,
	}
}
