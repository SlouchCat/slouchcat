package visibility

import (
	"database/sql"
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

// legacyPosterSize is the only poster size of M1a. Until image.fetch has derived the new sizes
// (after the upgrade, or for a poster whose original cannot be fetched), the w500 JPEG stands in
// for the w400 poster, so a title keeps its poster: the summary names it as w400 and the image
// endpoint serves it under that size (see Reader.Image).
const legacyPosterSize = "w500"

// pickImage returns the image of kind in the given size, or nil; the w400 poster falls back to
// M1a's w500 row. Of the two formats it takes the WebP row's hash, else the JPEG row's; width and
// height are the same for both.
func pickImage(rows []scopedq.Image, kind, size string) *model.Image {
	img := pickExactImage(rows, kind, size)
	if img == nil && kind == "poster" && size == posterSize {
		if img = pickExactImage(rows, kind, legacyPosterSize); img != nil {
			img.Size = posterSize
		}
	}
	return img
}

// pickExactImage returns the image of kind in exactly the given size, or nil.
func pickExactImage(rows []scopedq.Image, kind, size string) *model.Image {
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
		if p.PlaybackRate.Valid {
			rate := p.PlaybackRate.Float64
			s.Progress.PlaybackRate = &rate
		}
	}
	// Audio (M6).
	s.Author = it.Author.String
	s.Explicit = it.Explicit == 1
	if it.PublishedAt.Valid {
		t := time.UnixMilli(it.PublishedAt.Int64).UTC()
		s.PublishedAt = &t
	}
	if it.DurationMs.Valid {
		d := it.DurationMs.Int64
		s.DurationMs = &d
	}
	s.ParentID = ex.parents[it.ID] // only a parent the profile may see
	s.SeasonNumber = optInt(it.SeasonNumber)
	s.EpisodeNumber = optInt(it.EpisodeNumber)
	s.EpisodeEnd = optInt(it.EpisodeEnd)
	s.ExtraKind = it.ExtraKind.String
	s.SeriesTitle = ex.seriesTitles[it.ID]
	if it.Kind == "series" || it.Kind == "season" {
		n := ex.childCounts[it.ID]
		s.ChildCount = &n
	}
	return s
}

func optInt(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
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
	if it.TvdbID.Valid {
		id := it.TvdbID.Int64
		d.TVDBID = &id
	}
	d.Narrator = it.Narrator.String
	d.SeriesName = it.SeriesName.String
	if it.SeriesIndex.Valid {
		idx := it.SeriesIndex.Float64
		d.SeriesIndex = &idx
	}
	return d
}

// versionLabel is the label of a version (B-5): the label parsed from the file name, else one
// derived from the probed height, so the version choice is always labelled. Only a file without a
// probed height and without a parsed label has none.
func versionLabel(parsed string, height int64) string {
	switch {
	case parsed != "":
		return parsed
	case height >= 2000:
		return "2160p"
	case height >= 1000:
		return "1080p"
	case height >= 700:
		return "720p"
	case height > 0:
		return "SD"
	default:
		return ""
	}
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
		VersionLabel: versionLabel(f.VersionLabel.String, f.Height.Int64),
		Available:    !f.MissingSince.Valid,
	}
}
