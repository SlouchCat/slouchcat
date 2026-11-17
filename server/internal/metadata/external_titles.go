package metadata

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/externalq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Titles outside the library (concept 25 stage 2, 08-datenmodell-api.md section 2.9,
// 03-architektur.md sections 13, 15 and 16). A missing title is a TMDB movie or series (media
// "show") that the preview page, the search, the wishes and the administration show although no
// item stands for it. Package metadata alone fetches and stores it (store/externalq, depguard rule
// "externalq"); it never decides who may see it: the scoped callers ask visibility.MissingTitle and
// the wish rights first, the administration needs neither.
//
//   - EnsureExternalTitle returns the stored row, or fetches the title once from TMDB and stores
//     it when there is none (the preview page without a cache entry, the details of search hits
//     for profiles under 18). RefreshExternalTitle always fetches.
//   - A fetch reads the detail in every text language of the server's default (texts, genre
//     names), the certifications (movies: the main release per country; series: the content
//     ratings), the external ids (IMDb, TVDB), the trailers of the TMDB step of the trailer chain,
//     the status and, for a series, the seasons with their episode counts and the last and next
//     episode. min_age comes from ageFromCertifications in the country order of ADR 0034 (the
//     rating country first, then DE, AT, CH, GB, US); unknown is NULL (counts as 18). A change of
//     the rating country computes every age again from the stored certifications
//     (recomputeExternalAges, no TMDB request).
//   - The images are fetched on their first request (OpenExternalTitleImage): the TMDB original
//     of the kind is scaled into the sizes of imageWidths, never wider than the original, as JPEG
//     (a logo PNG) and WebP below <cache>/titles/<media>/<tmdbId>/, one external_title_images row
//     each. A refresh that finds another TMDB path for a kind drops its rows and files, so the next
//     request fetches the new image. The TMDB paths never leave the package: browsers get the
//     images through Slouchcat's own routes only.
//   - A title TMDB does not know (404) is not stored; EnsureExternalTitle answers model.ErrNotFound
//     and does not ask TMDB again for externalMissAge. A stored title that vanished at TMDB keeps
//     its row: the refresh moves fetched_at on and asks again only when it is due, so neither the
//     job nor a page view retries it at once. An image TMDB refuses is remembered the same way.
//   - titles.refresh (schedule, daily, priority 7: a short daily job above the bulk rungs, ladder
//     of jobs/queue.go) fetches every row older than ExternalTitleMaxAge again, the oldest first,
//     at most titlesRefreshBatch per run. Without a TMDB key it ends without work; a key TMDB
//     rejects or titlesRefreshFailures transport failures in a row end it with an error, so the
//     queue retries after its backoff.
const (
	JobTitlesRefresh      = "titles.refresh"
	titlesRefreshKey      = "titles.refresh"
	titlesRefreshPriority = 7
	titlesRefreshEvery    = 24 * time.Hour
	// ExternalTitleMaxAge is the age after which titles.refresh fetches a row again.
	ExternalTitleMaxAge = 7 * 24 * time.Hour
	titlesRefreshBatch  = 1000
	// ExternalTitleUnusedAge is how long a title may go unused before titles.refresh stops
	// fetching it and removes it (migration 00074); a wish closed within it keeps its title too.
	ExternalTitleUnusedAge = 30 * 24 * time.Hour
	// externalUsedStep is how often a read moves used_at on: at most once a day per title.
	externalUsedStep      = 24 * time.Hour
	titlesRefreshFailures = 3
	// externalMissAge is how long a title or image TMDB refused is not asked for again.
	externalMissAge   = time.Hour
	externalTitlesDir = "titles"

	// MediaMovie and MediaShow are the values of external_titles.media.
	MediaMovie = "movie"
	MediaShow  = "show"
)

// ExternalTitle is a stored title outside the library. It carries no TMDB image path: ImageKinds
// lists the kinds TMDB has an image of, which OpenExternalTitleImage serves.
type ExternalTitle struct {
	Media          string
	TMDBID         int64
	TVDBID         int64  // 0 = unknown
	IMDBID         string // "" = unknown
	OriginalTitle  string
	Year           int    // 0 = unknown
	ReleaseDate    string // YYYY-MM-DD, first air date of a series; "" = unknown
	RuntimeMinutes int    // 0 = unknown
	Genres         []ExternalGenre
	Certifications map[string]string // rating country -> TMDB certificate as delivered
	MinAge         *int              // nil = unknown (counts as 18)
	Texts          map[string]ExternalText
	Trailers       []ExternalTrailer
	Status         string // TMDB status ("Returning Series", "Released"); "" = unknown
	Seasons        []ExternalSeason
	NextEpisode    *ExternalEpisode
	LastEpisode    *ExternalEpisode
	ImageKinds     []string // poster, backdrop, logo: the kinds with a TMDB image
	Images         []ExternalImageRef
	FetchedAt      time.Time
}

// ExternalImageRef is one size of an image of a stored title as the pages reference it: the sizes
// of imageWidths. Before the first request scales the image (OpenExternalTitleImage) every size is
// offered with the nominal height of the kind (poster 2:3, backdrop 16:9, logo 4:1) and the time
// of the title's fetch as Version; afterwards only the stored sizes (never wider than the
// original) with their width, height and content hash, so a browser cache never keeps the guess.
type ExternalImageRef struct {
	Kind, Size    string
	Width, Height int
	Version       string
}

// nominalHeight is the height of width for a kind whose image is not scaled yet.
func nominalHeight(kind string, width int) int {
	switch kind {
	case "poster":
		return width * 3 / 2
	case "backdrop":
		return width * 9 / 16
	}
	return max(1, width/4)
}

// externalImageRefs are the references of the kinds with a TMDB image (see ExternalImageRef).
func externalImageRefs(kinds []string, stored []externalq.ListExternalTitleImageSizesRow, fetchedAt int64) []ExternalImageRef {
	var out []ExternalImageRef
	for _, kind := range kinds {
		n := 0
		for _, r := range stored {
			if r.Kind == kind {
				out = append(out, ExternalImageRef{Kind: kind, Size: r.Size, Width: int(r.Width), Height: int(r.Height), Version: r.ContentHash})
				n++
			}
		}
		if n > 0 {
			continue
		}
		for _, w := range imageWidths[kind] {
			out = append(out, ExternalImageRef{Kind: kind, Size: sizeName(w), Width: w, Height: nominalHeight(kind, w),
				Version: strconv.FormatInt(fetchedAt, 10)})
		}
	}
	return out
}

// ExternalGenre is a TMDB genre with its name per text language.
type ExternalGenre struct {
	ID    int64             `json:"id"`
	Names map[string]string `json:"names"`
}

// ExternalText is the title, overview and tagline of one text language.
type ExternalText struct {
	Title    string
	Overview string
	Tagline  string
}

// ExternalTrailer is a YouTube trailer of the TMDB videos; Lang "" is one without language.
type ExternalTrailer struct {
	Lang     string `json:"lang"`
	Key      string `json:"key"`
	Official bool   `json:"official"`
	Size     int    `json:"size,omitempty"`
}

// ExternalSeason is one season of a series as TMDB lists it (season 0 holds the specials).
type ExternalSeason struct {
	Number       int    `json:"number"`
	EpisodeCount int    `json:"episodeCount"`
	AirDate      string `json:"airDate,omitempty"`
}

// ExternalEpisode names one episode of a series (the last aired, the next one).
type ExternalEpisode struct {
	Season  int    `json:"season"`
	Episode int    `json:"episode"`
	AirDate string `json:"airDate,omitempty"`
}

// ExternalImageSize reports whether size is a size the images of kind are served in (the sizes of
// library images, imageWidths: poster w200, w400, w800; backdrop w780, w1280, w1920; logo w400,
// w800).
func ExternalImageSize(kind, size string) bool {
	for _, w := range imageWidths[kind] {
		if sizeName(w) == size {
			return true
		}
	}
	return false
}

func externalKey(media string, tmdbID int64) string {
	return "external:" + media + ":" + strconv.FormatInt(tmdbID, 10)
}

func validExternal(media string, tmdbID int64) bool {
	return (media == MediaMovie || media == MediaShow) && tmdbID > 0
}

// ExternalTitle returns the stored title without asking TMDB; model.ErrNotFound when there is none.
func (s *Service) ExternalTitle(ctx context.Context, media string, tmdbID int64) (ExternalTitle, error) {
	if !validExternal(media, tmdbID) {
		return ExternalTitle{}, model.ErrNotFound
	}
	q := externalq.New(s.db.Read)
	row, err := q.GetExternalTitle(ctx, externalq.GetExternalTitleParams{Media: media, TmdbID: tmdbID})
	if errors.Is(err, sql.ErrNoRows) {
		return ExternalTitle{}, model.ErrNotFound
	}
	if err != nil {
		return ExternalTitle{}, fmt.Errorf("get external title %s: %w", externalKey(media, tmdbID), err)
	}
	texts, err := q.ListExternalTitleTexts(ctx, externalq.ListExternalTitleTextsParams{Media: media, TmdbID: tmdbID})
	if err != nil {
		return ExternalTitle{}, fmt.Errorf("list texts of external title %s: %w", externalKey(media, tmdbID), err)
	}
	t, err := externalTitleOf(row, texts)
	if err != nil {
		return ExternalTitle{}, err
	}
	if now := s.now(); !row.UsedAt.Valid || row.UsedAt.Int64 < now.Add(-externalUsedStep).UnixMilli() {
		// In use (migration 00074): at most one write a day per title.
		if err := externalq.New(s.db.Write).TouchExternalTitleUsed(ctx, externalq.TouchExternalTitleUsedParams{
			Now: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}, Media: media, TmdbID: tmdbID,
			Before: sql.NullInt64{Int64: now.Add(-externalUsedStep).UnixMilli(), Valid: true},
		}); err != nil {
			return ExternalTitle{}, fmt.Errorf("touch external title %s: %w", externalKey(media, tmdbID), err)
		}
	}
	sizes, err := q.ListExternalTitleImageSizes(ctx, externalq.ListExternalTitleImageSizesParams{Media: media, TmdbID: tmdbID})
	if err != nil {
		return ExternalTitle{}, fmt.Errorf("list image sizes of external title %s: %w", externalKey(media, tmdbID), err)
	}
	t.Images = externalImageRefs(t.ImageKinds, sizes, row.FetchedAt)
	return t, nil
}

// EnsureExternalTitle returns the stored title, or fetches and stores it when there is none. It
// answers model.ErrNotFound for a title TMDB does not know (and for externalMissAge does not ask
// again), model.ErrProviderNotConfigured without a TMDB key, model.ErrTMDBKeyInvalid and
// model.ErrTMDBUnreachable as the fetch met them.
func (s *Service) EnsureExternalTitle(ctx context.Context, media string, tmdbID int64) (ExternalTitle, error) {
	t, err := s.ExternalTitle(ctx, media, tmdbID)
	if !errors.Is(err, model.ErrNotFound) || !validExternal(media, tmdbID) {
		return t, err
	}
	key := externalKey(media, tmdbID)
	if s.externalMissed(key) {
		return ExternalTitle{}, model.ErrNotFound
	}
	release, err := s.lockItemImages(ctx, key)
	if err != nil {
		return ExternalTitle{}, err
	}
	defer release()
	// Another caller may have stored it while this one waited.
	if t, err := s.ExternalTitle(ctx, media, tmdbID); !errors.Is(err, model.ErrNotFound) {
		return t, err
	}
	return s.refreshExternal(ctx, media, tmdbID)
}

// RefreshExternalTitle fetches the title from TMDB and stores it, whatever its age. A title TMDB
// no longer knows keeps its stored row (fetched_at moves on) and the answer is model.ErrNotFound.
func (s *Service) RefreshExternalTitle(ctx context.Context, media string, tmdbID int64) (ExternalTitle, error) {
	if !validExternal(media, tmdbID) {
		return ExternalTitle{}, model.ErrNotFound
	}
	release, err := s.lockItemImages(ctx, externalKey(media, tmdbID))
	if err != nil {
		return ExternalTitle{}, err
	}
	defer release()
	return s.refreshExternal(ctx, media, tmdbID)
}

// refreshExternal fetches and stores one title; the caller holds its lock.
func (s *Service) refreshExternal(ctx context.Context, media string, tmdbID int64) (ExternalTitle, error) {
	key := externalKey(media, tmdbID)
	rec, err := s.fetchExternal(ctx, media, tmdbID)
	switch {
	case errors.Is(err, ErrNotFound):
		s.externalMiss(key)
		now := s.now().UnixMilli()
		if terr := externalq.New(s.db.Write).TouchExternalTitle(ctx, externalq.TouchExternalTitleParams{FetchedAt: now, Media: media, TmdbID: tmdbID}); terr != nil {
			return ExternalTitle{}, fmt.Errorf("touch external title %s: %w", key, terr)
		}
		return ExternalTitle{}, model.ErrNotFound
	case errors.Is(err, errNotConfigured):
		return ExternalTitle{}, model.ErrProviderNotConfigured
	case err != nil:
		return ExternalTitle{}, err
	}
	if err := s.storeExternal(ctx, rec); err != nil {
		return ExternalTitle{}, err
	}
	return s.ExternalTitle(ctx, media, tmdbID)
}

// externalRecord is one fetched title, ready to store.
type externalRecord struct {
	title externalq.UpsertExternalTitleParams
	texts []externalq.InsertExternalTitleTextParams
}

// fetchExternal reads a title from TMDB in the text languages of the server's default.
func (s *Service) fetchExternal(ctx context.Context, media string, tmdbID int64) (externalRecord, error) {
	c, err := s.tmdb.keyed(ctx)
	if err != nil {
		return externalRecord{}, err
	}
	st, err := s.Settings(ctx)
	if err != nil {
		return externalRecord{}, err
	}
	langs := st.TextLangs
	if len(langs) == 0 {
		langs = []string{"en"}
	}
	imageLangs := primaryLanguages(langs)
	rec := externalRecord{title: externalq.UpsertExternalTitleParams{Media: media, TmdbID: tmdbID, FetchedAt: s.now().UnixMilli()}}
	genres := map[int64]*ExternalGenre{}
	var order []int64
	addGenres := func(lang string, list []Genre) {
		for _, g := range list {
			if g.ID <= 0 || g.Name == "" {
				continue
			}
			if genres[g.ID] == nil {
				genres[g.ID] = &ExternalGenre{ID: g.ID, Names: map[string]string{}}
				order = append(order, g.ID)
			}
			genres[g.ID].Names[lang] = g.Name
		}
	}
	addText := func(lang, title, overview, tagline string) {
		if strings.TrimSpace(title) == "" {
			return
		}
		rec.texts = append(rec.texts, externalq.InsertExternalTitleTextParams{Media: media, TmdbID: tmdbID, Language: lang,
			Title: title, Overview: trimmedNull(overview), Tagline: trimmedNull(tagline)})
	}
	var certs map[string]string
	var imgs Images
	var poster, backdrop, date string
	if media == MediaMovie {
		var main Movie
		for i, l := range langs {
			var il []string
			if i == 0 {
				il = imageLangs
			}
			m, err := c.Movie(ctx, tmdbID, tmdbLang(l), il)
			if err != nil {
				return externalRecord{}, err
			}
			if i == 0 {
				main = m
			}
			addText(l, m.Title, m.Overview, m.Tagline)
			addGenres(l, m.Genres)
		}
		certs = releaseCertifications(main.ReleaseDates)
		imgs, poster, backdrop, date = main.Images, main.PosterPath, main.BackdropPath, main.ReleaseDate
		rec.title.ImdbID = trimmedNull(main.IMDBID)
		rec.title.OriginalTitle = trimmedNull(main.OriginalTitle)
		rec.title.RuntimeMinutes = nullPositive(main.Runtime)
		rec.title.Status = trimmedNull(main.Status)
	} else {
		var main TV
		for i, l := range langs {
			var il []string
			if i == 0 {
				il = imageLangs
			}
			tv, err := c.TV(ctx, tmdbID, tmdbLang(l), il)
			if err != nil {
				return externalRecord{}, err
			}
			if i == 0 {
				main = tv
			}
			addText(l, tv.Name, tv.Overview, tv.Tagline)
			addGenres(l, tv.Genres)
		}
		certs = tvCertifications(main)
		imgs, poster, backdrop, date = main.Images, main.PosterPath, main.BackdropPath, main.FirstAirDate
		rec.title.ImdbID = trimmedNull(main.ExternalIDs.IMDBID)
		rec.title.TvdbID = nullPositive64(main.ExternalIDs.TVDBID)
		rec.title.OriginalTitle = trimmedNull(main.OriginalName)
		rec.title.Status = trimmedNull(main.Status)
		for _, rt := range main.EpisodeRunTime {
			if rt > 0 {
				rec.title.RuntimeMinutes = nullPositive(rt)
				break
			}
		}
		seasons := make([]ExternalSeason, 0, len(main.Seasons))
		for _, se := range main.Seasons {
			seasons = append(seasons, ExternalSeason{Number: se.SeasonNumber, EpisodeCount: se.EpisodeCount, AirDate: validDate(se.AirDate)})
		}
		if rec.title.Seasons, err = marshalJSON(seasons); err != nil {
			return externalRecord{}, err
		}
		if rec.title.LastEpisode, err = episodeJSON(main.LastEpisodeToAir); err != nil {
			return externalRecord{}, err
		}
		if rec.title.NextEpisode, err = episodeJSON(main.NextEpisodeToAir); err != nil {
			return externalRecord{}, err
		}
	}
	if date = validDate(date); date != "" {
		rec.title.ReleaseDate = trimmedNull(date)
		if y, ok := yearOf(date); ok {
			rec.title.Year = sql.NullInt64{Int64: int64(y), Valid: true}
		}
	}
	if rec.title.Seasons == "" {
		rec.title.Seasons = "[]"
	}
	if certs == nil {
		certs = map[string]string{}
	}
	if age, _ := ageFromCertifications(certs, st.RatingCountry); age != nil {
		rec.title.MinAge = sql.NullInt64{Int64: int64(*age), Valid: true}
	}
	if rec.title.Certifications, err = marshalJSON(certs); err != nil {
		return externalRecord{}, err
	}
	list := make([]ExternalGenre, 0, len(order))
	for _, id := range order {
		list = append(list, *genres[id])
	}
	if rec.title.Genres, err = marshalJSON(list); err != nil {
		return externalRecord{}, err
	}
	rec.title.PosterPath = trimmedNull(pickExternalImage(imgs.Posters, imageLangs, poster, false))
	rec.title.BackdropPath = trimmedNull(pickExternalImage(imgs.Backdrops, imageLangs, backdrop, true))
	rec.title.LogoPath = trimmedNull(pickExternalImage(imgs.Logos, imageLangs, "", false))
	trailers, err := s.externalTrailers(ctx, media, tmdbID, langs)
	if err != nil {
		return externalRecord{}, err
	}
	if rec.title.Trailers, err = marshalJSON(trailers); err != nil {
		return externalRecord{}, err
	}
	return rec, nil
}

// externalTrailers asks the TMDB step of the trailer chain (no YouTube search: its daily quota
// belongs to the library) for one trailer per primary text language and one without language.
func (s *Service) externalTrailers(ctx context.Context, media string, tmdbID int64, langs []string) ([]ExternalTrailer, error) {
	kind := "movie"
	if media == MediaShow {
		kind = "series"
	}
	hits, err := s.trailersTMDB.FindTrailers(ctx, TrailerQuery{Kind: kind, TMDBID: tmdbID, Langs: primaryLanguages(langs), Regions: trailerRegions(langs)})
	if err != nil {
		return nil, fmt.Errorf("trailers of external title %s: %w", externalKey(media, tmdbID), err)
	}
	out := make([]ExternalTrailer, 0, len(hits))
	for _, h := range hits {
		out = append(out, ExternalTrailer{Lang: h.Lang, Key: h.YouTubeID, Official: h.Official, Size: h.Size})
	}
	return out, nil
}

// pickExternalImage chooses the TMDB path of one kind from the images block: an image in the first
// text language that has one, else one without text (for a backdrop the image without text comes
// first), else def, the path of the detail answer. The lists come in TMDB's order (best voted
// first). A logo must be a PNG: an SVG cannot be scaled.
func pickExternalImage(list []Image, langs []string, def string, textless bool) string {
	usable := func(im Image) bool {
		return tmdbImagePath(im.FilePath) && !strings.HasSuffix(strings.ToLower(im.FilePath), ".svg")
	}
	byLang := func(lang string) string {
		for _, im := range list {
			l := ""
			if im.Lang != nil {
				l = *im.Lang
			}
			if l == lang && usable(im) {
				return im.FilePath
			}
		}
		return ""
	}
	order := slices.Clone(langs)
	if textless {
		order = append([]string{""}, order...)
	} else {
		order = append(order, "")
	}
	for _, l := range order {
		if p := byLang(l); p != "" {
			return p
		}
	}
	if tmdbImagePath(def) {
		return def
	}
	return ""
}

// storeExternal writes a fetched title, its texts and drops the images of a kind whose TMDB path
// changed; their files go after the commit.
func (s *Service) storeExternal(ctx context.Context, rec externalRecord) error {
	key := externalKey(rec.title.Media, rec.title.TmdbID)
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store external title %s: %w", key, err)
	}
	defer func() { _ = tx.Rollback() }()
	q := externalq.New(tx)
	old, err := q.GetExternalTitle(ctx, externalq.GetExternalTitleParams{Media: rec.title.Media, TmdbID: rec.title.TmdbID})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("store external title %s: %w", key, err)
	}
	if err := q.UpsertExternalTitle(ctx, rec.title); err != nil {
		return fmt.Errorf("store external title %s: %w", key, err)
	}
	if err := q.DeleteExternalTitleTexts(ctx, externalq.DeleteExternalTitleTextsParams{Media: rec.title.Media, TmdbID: rec.title.TmdbID}); err != nil {
		return fmt.Errorf("store texts of external title %s: %w", key, err)
	}
	for _, t := range rec.texts {
		if err := q.InsertExternalTitleText(ctx, t); err != nil {
			return fmt.Errorf("store texts of external title %s: %w", key, err)
		}
	}
	var gone []string
	for kind, paths := range map[string][2]sql.NullString{
		"poster":   {old.PosterPath, rec.title.PosterPath},
		"backdrop": {old.BackdropPath, rec.title.BackdropPath},
		"logo":     {old.LogoPath, rec.title.LogoPath},
	} {
		if paths[0] == paths[1] {
			continue
		}
		rels, err := q.DeleteExternalTitleImages(ctx, externalq.DeleteExternalTitleImagesParams{Media: rec.title.Media, TmdbID: rec.title.TmdbID, Kind: kind})
		if err != nil {
			return fmt.Errorf("drop images of external title %s: %w", key, err)
		}
		gone = append(gone, rels...)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store external title %s: %w", key, err)
	}
	s.removeExternalFiles(gone)
	return nil
}

// removeExternalFiles deletes image files below the cache root; a missing file is fine.
func (s *Service) removeExternalFiles(rels []string) {
	if len(rels) == 0 || s.cacheE != nil {
		return
	}
	root, err := paths.Open(s.cache)
	if err != nil {
		s.log.Warn("open cache for external title images", "err", err)
		return
	}
	defer root.Close()
	for _, rel := range rels {
		if err := root.Remove(rel); err != nil && !errors.Is(err, os.ErrNotExist) { //nolint:forbidigo // the cache root (external title images), never a library root (ADR 0032)
			s.log.Warn("remove external title image", "err", err)
		}
	}
}

// OpenExternalTitleImage opens the image of a stored title by kind (poster, backdrop, logo) and
// size (ExternalImageSize): the WebP with preferWebP, else the JPEG (a logo: PNG). A size the
// original was too small for falls back to the widest stored size of the kind. The image is
// fetched from TMDB and scaled on its first request. It answers model.ErrNotFound for an unknown
// size, a title without stored row or without an image of the kind and an image TMDB refuses,
// model.ErrTMDBUnreachable when the download failed for now. It checks no profile: the scoped
// caller asks visibility first (age and wish rights, 404 otherwise), the administration does not.
func (s *Service) OpenExternalTitleImage(ctx context.Context, media string, tmdbID int64, kind, size string, preferWebP bool) (model.StreamSource, error) {
	if !validExternal(media, tmdbID) || !ExternalImageSize(kind, size) {
		return model.StreamSource{}, model.ErrNotFound
	}
	if s.cacheE != nil {
		return model.StreamSource{}, s.cacheE
	}
	q := externalq.New(s.db.Read)
	t, err := q.GetExternalTitle(ctx, externalq.GetExternalTitleParams{Media: media, TmdbID: tmdbID})
	if errors.Is(err, sql.ErrNoRows) {
		return model.StreamSource{}, model.ErrNotFound
	}
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("get external title %s: %w", externalKey(media, tmdbID), err)
	}
	src := map[string]sql.NullString{"poster": t.PosterPath, "backdrop": t.BackdropPath, "logo": t.LogoPath}[kind].String
	if !tmdbImagePath(src) {
		return model.StreamSource{}, model.ErrNotFound
	}
	root, err := paths.Open(s.cache)
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("open cache: %w", err)
	}
	defer root.Close()
	params := externalq.ListExternalTitleImagesParams{Media: media, TmdbID: tmdbID, Kind: kind}
	rows, err := q.ListExternalTitleImages(ctx, params)
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("list images of external title %s: %w", externalKey(media, tmdbID), err)
	}
	sourceURL := s.client.imageURL(originalSize, src)
	if !externalImagesReady(rows, sourceURL, rootExists(root)) {
		if err := s.fetchExternalImages(ctx, root, media, tmdbID, kind, src); err != nil {
			return model.StreamSource{}, err
		}
		if rows, err = externalq.New(s.db.Read).ListExternalTitleImages(ctx, params); err != nil {
			return model.StreamSource{}, fmt.Errorf("list images of external title %s: %w", externalKey(media, tmdbID), err)
		}
	}
	format := FallbackFormat(kind)
	if preferWebP {
		format = string(transcode.ImageWebP)
	}
	pick := pickExternalRow(rows, size, format)
	if pick == nil {
		return model.StreamSource{}, model.ErrNotFound
	}
	f, err := root.Open(pick.RelPath)
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("open external title image: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return model.StreamSource{}, fmt.Errorf("stat external title image: %w", err)
	}
	return model.StreamSource{File: f, MediaType: pick.MediaType, Size: info.Size(), ModTime: time.UnixMilli(pick.FetchedAt),
		Name: pick.RelPath, ETag: `"` + pick.ContentHash + `"`}, nil
}

// externalImagesReady reports whether the rows of a kind stem from sourceURL and their files exist.
func externalImagesReady(rows []externalq.ExternalTitleImage, sourceURL string, exists func(rel string) bool) bool {
	if len(rows) == 0 {
		return false
	}
	for _, r := range rows {
		if r.SourceUrl != sourceURL || !exists(r.RelPath) {
			return false
		}
	}
	return true
}

// pickExternalRow returns the row of size and format, else the widest row of the format.
func pickExternalRow(rows []externalq.ExternalTitleImage, size, format string) *externalq.ExternalTitleImage {
	var widest *externalq.ExternalTitleImage
	for i := range rows {
		r := &rows[i]
		if r.Format != format {
			continue
		}
		if r.Size == size {
			return r
		}
		if widest == nil || r.Width > widest.Width {
			widest = r
		}
	}
	return widest
}

// fetchExternalImages downloads the TMDB original of a kind and stores its sizes; one caller per
// title and kind at a time.
func (s *Service) fetchExternalImages(ctx context.Context, root *os.Root, media string, tmdbID int64, kind, src string) error {
	key := externalKey(media, tmdbID) + ":" + kind
	release, err := s.lockItemImages(ctx, key)
	if err != nil {
		return err
	}
	defer release()
	sourceURL := s.client.imageURL(originalSize, src)
	q := externalq.New(s.db.Write)
	params := externalq.ListExternalTitleImagesParams{Media: media, TmdbID: tmdbID, Kind: kind}
	rows, err := q.ListExternalTitleImages(ctx, params)
	if err != nil {
		return fmt.Errorf("list images of %s: %w", key, err)
	}
	if externalImagesReady(rows, sourceURL, rootExists(root)) {
		return nil // another request stored them meanwhile
	}
	missKey := key + ":" + src
	if s.externalMissed(missKey) {
		return model.ErrNotFound
	}
	data, _, err := s.client.DownloadCDNImage(ctx, originalSize, src)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if errors.Is(err, model.ErrTMDBUnreachable) {
			return err
		}
		s.externalMiss(missKey) // a 4xx, too large: the same answer next time
		return model.ErrNotFound
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		s.externalMiss(missKey)
		return model.ErrNotFound
	}
	dir := externalTitlesDir + "/" + media + "/" + strconv.FormatInt(tmdbID, 10)
	files, err := s.scaleExternal(ctx, root, dir, kind, data, cfg.Width, format)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if errors.Is(err, transcode.ErrImageExit) {
			s.externalMiss(missKey) // ffmpeg cannot decode what the header promised
			return model.ErrNotFound
		}
		return err
	}
	now := s.now().UnixMilli()
	keep := map[string]bool{}
	for _, f := range files {
		keep[f.rel] = true
		if err := q.UpsertExternalTitleImage(ctx, externalq.UpsertExternalTitleImageParams{Media: media, TmdbID: tmdbID, Kind: kind,
			Size: f.size, Format: f.format, SourceUrl: sourceURL, RelPath: f.rel, ContentHash: f.hash, MediaType: f.mediaType,
			Width: int64(f.width), Height: int64(f.height), Bytes: int64(f.bytes), FetchedAt: now}); err != nil {
			return fmt.Errorf("store image of %s: %w", key, err)
		}
	}
	// Rows of an earlier original that this one did not make again (a size it is too small for).
	var stale []string
	for _, r := range rows {
		if keep[r.RelPath] {
			continue
		}
		stale = append(stale, r.RelPath)
	}
	if len(stale) > 0 {
		if err := s.dropExternalRows(ctx, media, tmdbID, kind, keep); err != nil {
			return err
		}
		s.removeExternalFiles(stale)
	}
	return s.boundExternalImages(ctx, media, tmdbID)
}

// dropExternalRows deletes the rows of a kind whose file is not in keep.
func (s *Service) dropExternalRows(ctx context.Context, media string, tmdbID int64, kind string, keep map[string]bool) error {
	q := externalq.New(s.db.Write)
	rows, err := q.ListExternalTitleImages(ctx, externalq.ListExternalTitleImagesParams{Media: media, TmdbID: tmdbID, Kind: kind})
	if err != nil {
		return fmt.Errorf("list images of %s: %w", externalKey(media, tmdbID), err)
	}
	for _, r := range rows {
		if keep[r.RelPath] {
			continue
		}
		if err := q.DeleteExternalTitleImage(ctx, r.ID); err != nil {
			return fmt.Errorf("drop image of %s: %w", externalKey(media, tmdbID), err)
		}
	}
	return nil
}

// scaledExternal is one stored file of an external title image.
type scaledExternal struct {
	storedFile
	size, format string
}

// scaleExternal scales the original data (its width and image.DecodeConfig format) into every
// width of kind up to the original's (at least the smallest, at the original's width), in the
// formats of the kind, and renames the files into dir only when all of them exist.
func (s *Service) scaleExternal(ctx context.Context, root *os.Root, dir, kind string, data []byte, origWidth int, format string) ([]scaledExternal, error) {
	if err := root.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create image dir: %w", err)
	}
	src, err := tempName(dir)
	if err != nil {
		return nil, err
	}
	if err := writeFile(root, dir, src, data); err != nil {
		return nil, err
	}
	tmps := []string{src}
	defer func() {
		for _, t := range tmps {
			if rmErr := root.Remove(t); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) { //nolint:forbidigo // the cache root (external title images), never a library root (ADR 0032)
				s.log.Warn("remove temporary image", "err", rmErr)
			}
		}
	}()
	var widths []int
	var names []string
	for _, w := range imageWidths[kind] {
		if w <= origWidth {
			widths, names = append(widths, w), append(names, sizeName(w))
		}
	}
	if len(widths) == 0 {
		widths, names = []int{origWidth}, []string{sizeName(imageWidths[kind][0])}
	}
	in, err := paths.Resolve(s.cache, src)
	if err != nil {
		return nil, fmt.Errorf("resolve image: %w", err)
	}
	formats := formatsOf(kind)
	var outs []transcode.ImageOutput
	var plan []scaledExternal
	var tmpRels []string
	for i, w := range widths {
		for _, f := range formats {
			tmp, err := tempName(dir)
			if err != nil {
				return nil, err
			}
			tmps = append(tmps, tmp)
			abs, err := paths.Resolve(s.cache, tmp)
			if err != nil {
				return nil, fmt.Errorf("resolve temporary image: %w", err)
			}
			outs = append(outs, transcode.ImageOutput{Path: abs, Width: w, Format: f.format})
			tmpRels = append(tmpRels, tmp)
			plan = append(plan, scaledExternal{storedFile: storedFile{rel: dir + "/" + kind + "-" + names[i] + "." + f.ext, mediaType: f.mediaType},
				size: names[i], format: string(f.format)})
		}
	}
	if err := s.runner.ScaleImage(ctx, in, transcode.ImageDemuxer(format), outs); err != nil {
		return nil, fmt.Errorf("scale external title image: %w", err)
	}
	for i := range plan {
		out, err := root.ReadFile(tmpRels[i])
		if err != nil {
			return nil, fmt.Errorf("read scaled image: %w", err)
		}
		f, err := describeFile(out, plan[i].mediaType, plan[i].rel)
		if err != nil {
			return nil, err
		}
		plan[i].storedFile = *f
	}
	for i := range plan {
		if err := root.Rename(tmpRels[i], plan[i].rel); err != nil {
			return nil, fmt.Errorf("rename image: %w", err)
		}
	}
	return plan, nil
}

// externalMissed reports whether key was refused less than externalMissAge ago.
func (s *Service) externalMissed(key string) bool {
	s.externalMu.Lock()
	defer s.externalMu.Unlock()
	at, ok := s.externalMisses[key]
	if ok && s.now().Sub(at) >= externalMissAge {
		delete(s.externalMisses, key)
		return false
	}
	return ok
}

// externalMiss remembers that TMDB refused key.
func (s *Service) externalMiss(key string) {
	s.externalMu.Lock()
	defer s.externalMu.Unlock()
	if s.externalMisses == nil {
		s.externalMisses = map[string]time.Time{}
	}
	s.externalMisses[key] = s.now()
}

// recomputeExternalAges computes min_age of every stored title again from its certifications
// with country first (a change of the rating country, F-16, ADR 0034); no TMDB request.
func (s *Service) recomputeExternalAges(ctx context.Context, country string) error {
	q := externalq.New(s.db.Write)
	after := externalq.ListExternalTitleAgesParams{LimitCount: 500}
	for {
		rows, err := q.ListExternalTitleAges(ctx, after)
		if err != nil {
			return fmt.Errorf("list ages of external titles: %w", err)
		}
		for _, r := range rows {
			after.AfterMedia, after.AfterTmdbID = r.Media, r.TmdbID
			var certs map[string]string
			if err := json.Unmarshal([]byte(r.Certifications), &certs); err != nil {
				return fmt.Errorf("certifications of %s: %w", externalKey(r.Media, r.TmdbID), err)
			}
			age := sql.NullInt64{}
			if a, _ := ageFromCertifications(certs, country); a != nil {
				age = sql.NullInt64{Int64: int64(*a), Valid: true}
			}
			if age == r.MinAge {
				continue
			}
			if err := q.SetExternalTitleMinAge(ctx, externalq.SetExternalTitleMinAgeParams{MinAge: age, Media: r.Media, TmdbID: r.TmdbID}); err != nil {
				return fmt.Errorf("set age of %s: %w", externalKey(r.Media, r.TmdbID), err)
			}
		}
		if len(rows) < int(after.LimitCount) {
			return nil
		}
	}
}

// refreshOrder lists the titles titles.refresh fetches again, at most titlesRefreshBatch: the
// stored titles of open wishes older than ExternalTitleMaxAge first, then the other stale titles
// in use (used within ExternalTitleUnusedAge or on the spotlight list), the oldest fetch first.
func (s *Service) refreshOrder(ctx context.Context, now time.Time, open []TitleRef) ([]TitleRef, error) {
	cutoff := now.Add(-ExternalTitleMaxAge).UnixMilli()
	q := externalq.New(s.db.Read)
	var rows []TitleRef
	seen := map[TitleRef]bool{}
	for _, r := range open {
		at, err := q.GetExternalTitleFetchedAt(ctx, externalq.GetExternalTitleFetchedAtParams{Media: r.Media, TmdbID: r.TMDBID})
		switch {
		case errors.Is(err, sql.ErrNoRows):
			continue // never stored: the wish's pages fetch it
		case err != nil:
			return nil, fmt.Errorf("%s: fetch time: %w", JobTitlesRefresh, err)
		}
		if at < cutoff && !seen[r] && len(rows) < titlesRefreshBatch {
			seen[r] = true
			rows = append(rows, r)
		}
	}
	stale, err := q.ListStaleExternalTitles(ctx, externalq.ListStaleExternalTitlesParams{
		Cutoff: cutoff, UsedSince: sql.NullInt64{Int64: now.Add(-ExternalTitleUnusedAge).UnixMilli(), Valid: true}, LimitCount: titlesRefreshBatch,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: list stale titles: %w", JobTitlesRefresh, err)
	}
	for _, r := range stale {
		if ref := (TitleRef{Media: r.Media, TMDBID: r.TmdbID}); !seen[ref] && len(rows) < titlesRefreshBatch {
			seen[ref] = true
			rows = append(rows, ref)
		}
	}
	// The missing spotlight entries not stored yet (05-spotlight-integration.md section 5).
	missing, err := systemq.New(s.db.Read).ListSpotlightTitlesToStore(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: spotlight titles: %w", JobTitlesRefresh, err)
	}
	for _, r := range missing {
		if ref := (TitleRef{Media: r.Media, TMDBID: r.TmdbID.Int64}); !seen[ref] && len(rows) < titlesRefreshBatch {
			seen[ref] = true
			rows = append(rows, ref)
		}
	}
	return rows, nil
}

// ExternalPosterVersion is the version of the stored poster of a title outside the library for the
// administration's image route (getAdminExternalTitleImage): the content hash of its w200 JPEG,
// else the time of the title's fetch while the poster is not scaled yet; "" when the title is not
// stored or has no poster. Reading it counts as a use (migration 00074).
func (s *Service) ExternalPosterVersion(ctx context.Context, media string, tmdbID int64) (string, error) {
	t, err := s.ExternalTitle(ctx, media, tmdbID)
	if errors.Is(err, model.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for _, im := range t.Images {
		if im.Kind == "poster" && im.Size == model.AdminPosterSize {
			return im.Version, nil
		}
	}
	return "", nil
}

// removeUnusedExternal deletes the stored titles not used since ExternalTitleUnusedAge before now
// that are not on the spotlight list and that no wish of open or recent holds, with their texts,
// images and image files.
func (s *Service) removeUnusedExternal(ctx context.Context, now time.Time, open, recent []TitleRef) error {
	keep := map[TitleRef]bool{}
	for _, r := range append(slices.Clone(open), recent...) {
		keep[r] = true
	}
	rows, err := externalq.New(s.db.Read).ListUnusedExternalTitles(ctx, externalq.ListUnusedExternalTitlesParams{
		UsedBefore: sql.NullInt64{Int64: now.Add(-ExternalTitleUnusedAge).UnixMilli(), Valid: true}, LimitCount: titlesRefreshBatch,
	})
	if err != nil {
		return fmt.Errorf("%s: list unused titles: %w", JobTitlesRefresh, err)
	}
	removed := 0
	for _, r := range rows {
		if keep[TitleRef{Media: r.Media, TMDBID: r.TmdbID}] {
			continue
		}
		rels, err := s.deleteExternal(ctx, r.Media, r.TmdbID)
		if err != nil {
			return fmt.Errorf("%s: %w", JobTitlesRefresh, err)
		}
		s.removeExternalFiles(rels)
		removed++
	}
	if removed > 0 {
		s.log.InfoContext(ctx, "unused external titles removed", "count", removed)
	}
	return nil
}

// deleteExternal removes one stored title with its texts and image rows in one transaction and
// returns the image files to remove.
func (s *Service) deleteExternal(ctx context.Context, media string, tmdbID int64) ([]string, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := externalq.New(tx)
	rels, err := q.DeleteExternalTitleImageRows(ctx, externalq.DeleteExternalTitleImageRowsParams{Media: media, TmdbID: tmdbID})
	if err != nil {
		return nil, fmt.Errorf("delete images of external title %s: %w", externalKey(media, tmdbID), err)
	}
	if err := q.DeleteExternalTitleRow(ctx, externalq.DeleteExternalTitleRowParams{Media: media, TmdbID: tmdbID}); err != nil {
		return nil, fmt.Errorf("delete external title %s: %w", externalKey(media, tmdbID), err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return rels, nil
}

// runTitlesRefresh is the Run of the schedule titles.refresh: one job under its key.
func (s *Service) runTitlesRefresh(ctx context.Context, q *jobs.Queue, prio int) ([]int64, error) {
	id, err := q.Enqueue(ctx, jobs.NewJob{Type: JobTitlesRefresh, Priority: prio, IdempotencyKey: titlesRefreshKey})
	if err != nil {
		return nil, fmt.Errorf("enqueue %s: %w", JobTitlesRefresh, err)
	}
	if _, err := q.RaiseQueuedPriority(ctx, id, prio); err != nil {
		return nil, fmt.Errorf("raise %s: %w", JobTitlesRefresh, err)
	}
	return []int64{id}, nil
}

// TitleRef names a title outside the library.
type TitleRef struct {
	Media  string
	TMDBID int64
}

// WishTitles names the titles the wishes hold (package wishes, wired in main.go; metadata does not
// import it): open are those of open wishes, recent those of wishes closed since closedSince.
type WishTitles func(ctx context.Context, closedSince time.Time) (open, recent []TitleRef, err error)

// WithWishTitles sets the source of the titles the wishes hold (titles.refresh, migration 00074);
// without it no title counts as wished.
func WithWishTitles(fn WishTitles) Option {
	return func(o *options) { o.wishTitles = fn }
}

// handleTitlesRefresh keeps the stored titles in use (migration 00074): it removes the titles not
// used for ExternalTitleUnusedAge that no open or recently closed wish holds and that are not on
// the spotlight list, then fetches again the titles of open wishes and the titles used within
// ExternalTitleUnusedAge (or on the spotlight list) that are older than ExternalTitleMaxAge, the
// open wishes first. Without a TMDB key it only removes.
func (s *Service) handleTitlesRefresh(ctx context.Context, _ *jobs.Job) error {
	now := s.now()
	var open, recent []TitleRef
	if s.wishTitles != nil {
		var err error
		if open, recent, err = s.wishTitles(ctx, now.Add(-ExternalTitleUnusedAge)); err != nil {
			return fmt.Errorf("%s: titles of the wishes: %w", JobTitlesRefresh, err)
		}
	}
	if err := s.removeUnusedExternal(ctx, now, open, recent); err != nil {
		return err
	}
	if !s.tmdb.Configured(ctx) {
		jobs.Skip(ctx) // without a TMDB key there is nothing to fetch
		return nil
	}
	rows, err := s.refreshOrder(ctx, now, open)
	if err != nil {
		return err
	}
	failures := 0
	for i, r := range rows {
		jobs.Progress(ctx, int64(i), int64(len(rows)), "")
		_, err := s.RefreshExternalTitle(ctx, r.Media, r.TMDBID)
		switch {
		case err == nil, errors.Is(err, model.ErrNotFound):
			failures = 0 // a title TMDB no longer has keeps its row until the next due date
		case ctx.Err() != nil:
			return ctx.Err()
		case errors.Is(err, model.ErrTMDBKeyInvalid), errors.Is(err, model.ErrProviderNotConfigured):
			return fmt.Errorf("%s: %w", JobTitlesRefresh, err)
		default:
			failures++
			s.log.Warn("external title refresh failed", "media", r.Media, "tmdb_id", r.TMDBID, "err", err)
			if failures >= titlesRefreshFailures {
				return fmt.Errorf("%s: %d failures in a row: %w", JobTitlesRefresh, failures, err)
			}
		}
	}
	jobs.Progress(ctx, int64(len(rows)), int64(len(rows)), "")
	return nil
}

// externalTitleOf converts a stored row and its texts.
func externalTitleOf(row externalq.ExternalTitle, texts []externalq.ListExternalTitleTextsRow) (ExternalTitle, error) {
	t := ExternalTitle{Media: row.Media, TMDBID: row.TmdbID, TVDBID: row.TvdbID.Int64, IMDBID: row.ImdbID.String,
		OriginalTitle: row.OriginalTitle.String, Year: int(row.Year.Int64), ReleaseDate: row.ReleaseDate.String,
		RuntimeMinutes: int(row.RuntimeMinutes.Int64), Status: row.Status.String, Texts: map[string]ExternalText{},
		FetchedAt: time.UnixMilli(row.FetchedAt)}
	if row.MinAge.Valid {
		age := int(row.MinAge.Int64)
		t.MinAge = &age
	}
	for _, f := range []struct {
		raw string
		out any
	}{{row.Genres, &t.Genres}, {row.Certifications, &t.Certifications}, {row.Trailers, &t.Trailers}, {row.Seasons, &t.Seasons}} {
		if err := json.Unmarshal([]byte(f.raw), f.out); err != nil {
			return ExternalTitle{}, fmt.Errorf("external title %s: %w", externalKey(row.Media, row.TmdbID), err)
		}
	}
	for _, f := range []struct {
		raw sql.NullString
		out **ExternalEpisode
	}{{row.NextEpisode, &t.NextEpisode}, {row.LastEpisode, &t.LastEpisode}} {
		if !f.raw.Valid {
			continue
		}
		if err := json.Unmarshal([]byte(f.raw.String), f.out); err != nil {
			return ExternalTitle{}, fmt.Errorf("external title %s: %w", externalKey(row.Media, row.TmdbID), err)
		}
	}
	for kind, p := range map[string]sql.NullString{"poster": row.PosterPath, "backdrop": row.BackdropPath, "logo": row.LogoPath} {
		if p.Valid && p.String != "" {
			t.ImageKinds = append(t.ImageKinds, kind)
		}
	}
	slices.Sort(t.ImageKinds)
	for _, x := range texts {
		t.Texts[x.Language] = ExternalText{Title: x.Title, Overview: x.Overview.String, Tagline: x.Tagline.String}
	}
	return t, nil
}

// tvCertifications returns the content rating of a series per country.
func tvCertifications(tv TV) map[string]string {
	certs := map[string]string{}
	for _, r := range tv.ContentRatings.Results {
		if code, rating := strings.ToUpper(strings.TrimSpace(r.Country)), strings.TrimSpace(r.Rating); rating != "" {
			certs[code] = rating
		}
	}
	return certs
}

func episodeJSON(e *TVEpisode) (sql.NullString, error) {
	if e == nil || e.SeasonNumber < 0 || e.EpisodeNumber <= 0 {
		return sql.NullString{}, nil
	}
	raw, err := marshalJSON(ExternalEpisode{Season: e.SeasonNumber, Episode: e.EpisodeNumber, AirDate: validDate(e.AirDate)})
	return trimmedNull(raw), err
}

func marshalJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode external title: %w", err)
	}
	return string(raw), nil
}

// validDate keeps a YYYY-MM-DD date and drops anything else.
func validDate(d string) string {
	if _, err := time.Parse(time.DateOnly, d); err != nil {
		return ""
	}
	return d
}

// trimmedNull is the trimmed text, NULL when empty.
func trimmedNull(v string) sql.NullString { return nullString(strings.TrimSpace(v)) }

func nullPositive(v int) sql.NullInt64 { return sql.NullInt64{Int64: int64(v), Valid: v > 0} }

func nullPositive64(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: v > 0} }
