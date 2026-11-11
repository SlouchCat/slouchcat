// Package visibility is the only read path for titles on behalf of a viewing profile (plan 3.3,
// CLAUDE.md rule 2). Every query runs through the generated package store/scopedq, whose queries
// all join the view visible_items with the profile id of the Scope; the view applies deletion,
// availability, the FSK ceiling of the profile (unknown counts as 18) and library rights.
//
// Titles that do not exist and titles the profile may not see are indistinguishable: both return
// model.ErrNotFound, which the API answers with 404, never 403.
//
// Unfiltered reads (scanner, metadata jobs, administration) do not go through this package in M1a;
// they live in the packages library, metadata and admin on top of store/systemq. System creates the
// unfiltered scope for internal callers: the functions for viewers reject it, SystemItemRating
// (the administration view of a title's ratings, M2) and AdminSearch
// (the administration's title search, M7) require it.
package visibility

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// MaxRowItems caps every list (home rows) at 20 titles.
const MaxRowItems = 20

// ErrUnscoped is returned when a function that needs a profile receives the unfiltered system
// scope or the zero Scope.
var ErrUnscoped = errors.New("visibility: a profile scope is required")

// Scope is the viewer a read runs for. Build it with Profile or System; the zero value is
// rejected.
type Scope struct {
	profileID int64
	language  string // the interface language (de, en)
	metaLang  string // the metadata language (ADR 0018); the interface language when unset
	system    bool
	reason    string
}

// Profile returns the scope of a viewing profile: its id selects the rows of visible_items, its
// languages select the texts (metadata language first, then the interface language, then the
// rank of the server's list; concept 10 section 4.2).
func Profile(p model.Profile) Scope {
	lang := p.Language
	if lang != "de" && lang != "en" {
		lang = "de"
	}
	meta := p.MetadataLanguage
	if meta == "" {
		meta = lang
	}
	return Scope{profileID: p.ID, language: lang, metaLang: meta}
}

// langs returns the language preference of the scope for the text chain.
func (s Scope) langs() langPref { return langPref{meta: s.metaLang, ui: s.language} }

// System returns an unfiltered scope for internal callers; reason is written to the debug log when
// the scope is used. The analyzer itemsaccess allows calling it only in visibility, admin,
// library, metadata, jobs, notify and offline. Every viewer-facing function of this package
// rejects it with ErrUnscoped, because no viewer-facing read may run unfiltered.
func System(reason string) Scope {
	return Scope{system: true, reason: reason}
}

// Reader runs scoped reads against the reader pool of the database.
type Reader struct {
	q    *scopedq.Queries
	app  *appq.Queries    // spotlight_config (SpotlightRow)
	sys  *systemq.Queries // the system scope only (SystemItemRating)
	read *sql.DB          // the search builder (search.go); every statement it builds joins visible_items
	log  *slog.Logger
	now  func() time.Time
}

// Option configures a Reader.
type Option func(*Reader)

// WithClock replaces time.Now, the clock of the retention of the user picks (E-2).
func WithClock(now func() time.Time) Option {
	return func(r *Reader) { r.now = now }
}

// New returns a Reader on the read connections of db; log is the redacting logger.
func New(db *store.DB, log *slog.Logger, opts ...Option) *Reader {
	r := &Reader{q: scopedq.New(db.Read), app: appq.New(db.Read), sys: systemq.New(db.Read), read: db.Read, log: log, now: time.Now}
	for _, o := range opts {
		o(r)
	}
	return r
}

// profile returns the profile id of s or ErrUnscoped.
func (r *Reader) profile(ctx context.Context, s Scope) (int64, error) {
	if s.system {
		r.log.DebugContext(ctx, "system scope rejected by a profile read", "reason", s.reason)
		return 0, ErrUnscoped
	}
	if s.profileID == 0 {
		return 0, ErrUnscoped
	}
	return s.profileID, nil
}

// ErrNotSystem is returned by SystemItemRating for a viewer scope.
var ErrNotSystem = errors.New("visibility: the system scope is required")

// SystemItemRating returns the ratings of a title for the administration (rating override). It
// reads without any profile filter and therefore accepts only the System scope; an unknown or
// deleted title is model.ErrNotFound.
func (r *Reader) SystemItemRating(ctx context.Context, s Scope, itemID string) (model.ItemRating, error) {
	if !s.system {
		return model.ItemRating{}, ErrNotSystem
	}
	r.log.DebugContext(ctx, "system scope read", "reason", s.reason, "item_id", itemID)
	it, err := r.sys.GetItemSystem(ctx, itemID)
	if err != nil {
		return model.ItemRating{}, notFound(err)
	}
	if it.DeletedAt.Valid {
		return model.ItemRating{}, model.ErrNotFound
	}
	out := model.ItemRating{ID: it.ID, Kind: it.Kind, Title: it.Title, RatingSource: it.RatingSource.String}
	if it.OwnRating.Valid {
		v := int(it.OwnRating.Int64)
		out.OwnRating = &v
	}
	if it.EffectiveRating.Valid {
		v := int(it.EffectiveRating.Int64)
		out.EffectiveRating = &v
	}
	return out, nil
}

// Summary returns a visible title as it appears in lists.
func (r *Reader) Summary(ctx context.Context, s Scope, itemID string) (model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.ItemSummary{}, err
	}
	it, err := r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return model.ItemSummary{}, notFound(err)
	}
	extras, err := r.extras(ctx, pid, s.langs(), []string{it.ID})
	if err != nil {
		return model.ItemSummary{}, err
	}
	return summarize(it, extras, s.langs()), nil
}

// Item returns the detail of a visible title with its media files (versions, primary first, with
// their tracks, bit rate and HDR format) and, by kind, its seasons, episodes, extras, parent chain, next episode, cast and trailer count.
func (r *Reader) Item(ctx context.Context, s Scope, itemID string) (model.ItemDetail, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.ItemDetail{}, err
	}
	it, err := r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return model.ItemDetail{}, notFound(err)
	}
	extras, err := r.extras(ctx, pid, s.langs(), []string{it.ID})
	if err != nil {
		return model.ItemDetail{}, err
	}
	files, err := r.q.ListMediaFilesForItem(ctx, scopedq.ListMediaFilesForItemParams{ProfileID: pid, ItemID: it.ID})
	if err != nil {
		return model.ItemDetail{}, fmt.Errorf("list media files: %w", err)
	}
	d := detail(it, extras, s.langs())
	d.MediaFiles = make([]model.MediaFile, 0, len(files))
	for _, f := range files {
		d.MediaFiles = append(d.MediaFiles, mediaFile(f))
	}
	if err := r.fileDetails(ctx, pid, it.ID, d.MediaFiles); err != nil {
		return model.ItemDetail{}, err
	}
	if err := r.completeDetail(ctx, s, pid, it, &d); err != nil {
		return model.ItemDetail{}, err
	}
	return d, nil
}

// Recent returns the most recently added visible titles.
func (r *Reader) Recent(ctx context.Context, s Scope, limit int) ([]model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	items, err := r.q.ListRecentlyAdded(ctx, scopedq.ListRecentlyAddedParams{ProfileID: pid, LimitCount: clamp(limit)})
	if err != nil {
		return nil, fmt.Errorf("list recently added: %w", err)
	}
	return r.summaries(ctx, s, pid, items)
}

// ReleasedMonths is the window of the row "recently released" (M11, S-5: twelve months, fixed).
const ReleasedMonths = 12

// Released returns the visible titles released in the last ReleasedMonths, newest first: films and
// audiobooks by their release_date, a series by the release_date of its newest visible episode
// (S-6), so a series whose newest episode the profile may not see is dated by the newest one it
// may see and a series without a visible dated episode is absent. Titles without a date and titles
// dated after today are left out. The dates are compared as YYYY-MM-DD in UTC.
func (r *Reader) Released(ctx context.Context, s Scope, limit int) ([]model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	today := r.now().UTC()
	since := today.AddDate(0, -ReleasedMonths, 0)
	rows, err := r.q.ListRecentlyReleased(ctx, scopedq.ListRecentlyReleasedParams{
		ProfileID: pid, LimitCount: clamp(limit),
		Since: sql.NullString{String: since.Format(time.DateOnly), Valid: true},
		Today: sql.NullString{String: today.Format(time.DateOnly), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list recently released: %w", err)
	}
	items := make([]scopedq.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.Item)
	}
	return r.summaries(ctx, s, pid, items)
}

// NextEpisodeWindow is how long a completed episode keeps its series in "Weiterschauen" with the
// next episode (concept 15 P-1, F-2) and gets the next episodes downloaded automatically (O-3).
const NextEpisodeWindow = 60 * 24 * time.Hour

// ContinueWatching returns the row "Weiterschauen" of the profile, newest activity first: per
// visible series the episode with the newest progress while it is not completed, else the next
// visible episode not yet completed while that progress is at most NextEpisodeWindow old (concept
// 15 P-1); started, not completed films, audiobooks and podcast episodes as before. The rules are
// those of ListContinueWatching.
func (r *Reader) ContinueWatching(ctx context.Context, s Scope, limit int) ([]model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	items, err := r.q.ListContinueWatching(ctx, scopedq.ListContinueWatchingParams{
		ProfileID: pid, LimitCount: clamp(limit), Since: r.now().Add(-NextEpisodeWindow).UnixMilli(),
	})
	if err != nil {
		return nil, fmt.Errorf("list continue watching: %w", err)
	}
	return r.summaries(ctx, s, pid, items)
}

// Picks returns the user picks (E-1, E-2, concept 18 part B, ADR 0028) that the profile may see:
// every title one of the profiles of the owner's account recommended, once, newest recommendation
// first, while that recommendation is younger than the retention at the Reader's clock; at most
// limit (clamped to 1..MaxRowItems). Without a visible pick the list is empty and the start page has
// no carousel (E-9: no empty rows). Anonymous: the summaries carry no note and no profile.
func (r *Reader) Picks(ctx context.Context, s Scope, limit int) ([]model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	return r.picksPage(ctx, s, pid, 0, int(clamp(limit)))
}

// PicksPage returns one page of the same list as Picks without its cut to MaxRowItems (the page
// «User-Empfehlungen», listPicks) and the number of visible picks; paged in SQL. offset is clamped to
// 0 and above, limit to 1..MaxLibraryPage (0 or less = DefaultLibraryPage).
func (r *Reader) PicksPage(ctx context.Context, s Scope, offset, limit int) ([]model.ItemSummary, int, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, 0, err
	}
	offset, limit = libraryPage(offset, limit)
	total, err := r.q.CountUserPicks(ctx, scopedq.CountUserPicksParams{ProfileID: pid, Now: r.now().UnixMilli()})
	if err != nil {
		return nil, 0, fmt.Errorf("count user picks: %w", err)
	}
	items, err := r.picksPage(ctx, s, pid, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	return items, int(total), nil
}

func (r *Reader) picksPage(ctx context.Context, s Scope, pid int64, offset, limit int) ([]model.ItemSummary, error) {
	rows, err := r.q.ListUserPicks(ctx, scopedq.ListUserPicksParams{
		ProfileID: pid, Now: r.now().UnixMilli(), LimitCount: int64(limit), OffsetCount: int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list user picks: %w", err)
	}
	items := make([]scopedq.Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.Item)
	}
	return r.summaries(ctx, s, pid, items)
}

// MediaFileLocation is a media file of a visible title with the data needed to open it through the
// paths package. RootPath is the absolute library root, RelPath is relative to it.
type MediaFileLocation struct {
	ItemID   string
	File     model.MediaFile
	RootPath string
	RelPath  string
	// ProbedAt is the Unix millisecond time of the last probe (0: never probed); the chapter
	// images of the file are stored per probe (concept 13 stage 3).
	ProbedAt int64
	// Mtime is the modification time of the file row (Unix ms); with File.SizeBytes the version
	// a keyframe index computed for playback is stored for (library.StoreKeyframes, concept 14).
	Mtime int64
	// LibraryID is the library of the file's root; Trickplay its trickplay setting (concept 18,
	// 8.1; LibraryTrickplay.For picks the one of the title's kind).
	LibraryID int64
	Trickplay model.LibraryTrickplay
}

// MediaFile returns a media file of a visible title. An empty fileID selects the primary file: the
// file that is not missing with the largest height, then the oldest (the first of Item's list). A file of another title, an unknown file and a title without an
// available file return model.ErrNotFound.
func (r *Reader) MediaFile(ctx context.Context, s Scope, itemID, fileID string) (MediaFileLocation, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return MediaFileLocation{}, err
	}
	files, err := r.q.ListMediaFilesForItem(ctx, scopedq.ListMediaFilesForItemParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return MediaFileLocation{}, fmt.Errorf("list media files: %w", err)
	}
	for _, f := range files {
		if (fileID == "" && !f.MissingSince.Valid) || (fileID != "" && f.ID == fileID) {
			return MediaFileLocation{ItemID: f.ItemID, File: mediaFile(f), RootPath: f.RootPath, RelPath: f.RelPath, ProbedAt: f.ProbedAt.Int64, Mtime: f.Mtime,
				LibraryID: f.LibraryID, Trickplay: model.LibraryTrickplay{
					Movies: model.TrickplaySetting{Mode: f.TrickplayMovies, Keep: f.TrickplayMoviesKeep == 1},
					Series: model.TrickplaySetting{Mode: f.TrickplaySeries, Keep: f.TrickplaySeriesKeep == 1, Scope: f.TrickplaySeriesScope},
				}}, nil
		}
	}
	return MediaFileLocation{}, model.ErrNotFound
}

// ImageLocation is a stored image of a visible title. RelPath is relative to the cache root.
type ImageLocation struct {
	RelPath     string
	MediaType   string
	ContentHash string
	Bytes       int64
	ModTime     int64 // Unix milliseconds of the download
}

// Image returns the stored image of a visible title by kind (poster, backdrop) and size (w200 …
// w3840). With preferWebP it returns the WebP row when one exists, else (and without preferWebP)
// the JPEG row. A missing w400 poster falls back to M1a's w500 JPEG (legacyPosterSize), any other
// missing size to the widest stored size of the kind.
func (r *Reader) Image(ctx context.Context, s Scope, itemID, kind, size string, preferWebP bool) (ImageLocation, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return ImageLocation{}, err
	}
	// WebP when the client names it, else the fallback of the kind: JPEG, PNG for a logo (ADR 0018).
	fallback := "jpeg"
	if kind == "logo" {
		fallback = "png"
	}
	format := fallback
	if preferWebP {
		format = "webp"
	}
	im, err := r.q.GetItemImage(ctx, scopedq.GetItemImageParams{ProfileID: pid, ItemID: itemID, Kind: kind, Size: size, Format: format, Fallback: fallback})
	if errors.Is(err, sql.ErrNoRows) && kind == "poster" && size == posterSize {
		im, err = r.q.GetItemImage(ctx, scopedq.GetItemImageParams{ProfileID: pid, ItemID: itemID, Kind: kind, Size: legacyPosterSize, Format: format, Fallback: fallback})
	}
	if errors.Is(err, sql.ErrNoRows) && derivedSizes[kind][size] {
		// A size the pipeline did not derive (no size is wider than the original, an episode still
		// has no w1920; decision of 11 November 2026): the widest stored one of the kind.
		im, err = r.q.GetWidestItemImage(ctx, scopedq.GetWidestItemImageParams{ProfileID: pid, ItemID: itemID, Kind: kind, Format: format, Fallback: fallback})
	}
	if errors.Is(err, sql.ErrNoRows) {
		im, err = r.podcastImage(ctx, pid, itemID, kind, size, format) // an episode shows its podcast's cover (M6)
	}
	if err != nil {
		return ImageLocation{}, notFound(err)
	}
	return ImageLocation{RelPath: im.RelPath, MediaType: im.MediaType, ContentHash: im.ContentHash, Bytes: im.Bytes, ModTime: im.CreatedAt}, nil
}

// PersonImage returns the stored portrait of a person with a role on a title the profile may see
// (concept 17 5.1, ADR 0026): the WebP row with preferWebP, else the JPEG row; model.ErrNotFound
// without such a role or portrait.
func (r *Reader) PersonImage(ctx context.Context, s Scope, personID int64, preferWebP bool) (ImageLocation, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return ImageLocation{}, err
	}
	format := "jpeg"
	if preferWebP {
		format = "webp"
	}
	im, err := r.q.GetVisiblePersonImage(ctx, scopedq.GetVisiblePersonImageParams{PersonID: personID, Format: format, ProfileID: pid})
	if err != nil {
		return ImageLocation{}, notFound(err)
	}
	return ImageLocation{RelPath: im.RelPath.String, MediaType: im.MediaType.String, ContentHash: im.ContentHash.String, Bytes: im.Bytes.Int64, ModTime: im.FetchedAt}, nil
}

// CollectionImage returns the stored TMDB poster of a collection that is fully visible to the
// profile (ListFullyVisibleCollectionImages, concept 17 5.2): the WebP row with preferWebP, else
// the JPEG row; model.ErrNotFound otherwise. It never returns a member's poster.
func (r *Reader) CollectionImage(ctx context.Context, s Scope, collectionID string, preferWebP bool) (ImageLocation, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return ImageLocation{}, err
	}
	rows, err := r.q.ListFullyVisibleCollectionImages(ctx, scopedq.ListFullyVisibleCollectionImagesParams{CollectionIds: []string{collectionID}, ProfileID: pid})
	if err != nil {
		return ImageLocation{}, fmt.Errorf("collection image: %w", err)
	}
	var pick *scopedq.CollectionImage
	for i := range rows {
		if rows[i].Format == "jpeg" || (preferWebP && rows[i].Format == "webp") {
			if pick == nil || rows[i].Format == "webp" {
				pick = &rows[i]
			}
		}
	}
	if pick == nil {
		return ImageLocation{}, model.ErrNotFound
	}
	return ImageLocation{RelPath: pick.RelPath.String, MediaType: pick.MediaType.String, ContentHash: pick.ContentHash.String, Bytes: pick.Bytes.Int64, ModTime: pick.FetchedAt}, nil
}

// portrait returns the portrait of a person from its 'ok' JPEG row (the columns of a LEFT JOIN on
// person_images), nil without one. The person has a role on a visible title wherever it is read,
// so getPersonImage opens it.
func portrait(hash sql.NullString, width, height sql.NullInt64) *model.Image {
	if !hash.Valid {
		return nil
	}
	return &model.Image{Kind: "portrait", Size: portraitSize, ContentHash: hash.String, Width: int(width.Int64), Height: int(height.Int64)}
}

// fullyVisibleCollectionImages returns the TMDB posters (JPEG rows) of the collections among ids
// that the profile sees completely (ListFullyVisibleCollectionImages, the rule of
// getCollectionImage), by collection id.
func (r *Reader) fullyVisibleCollectionImages(ctx context.Context, pid int64, ids []string) (map[string]*model.Image, error) {
	out := map[string]*model.Image{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.q.ListFullyVisibleCollectionImages(ctx, scopedq.ListFullyVisibleCollectionImagesParams{CollectionIds: ids, ProfileID: pid})
	if err != nil {
		return nil, fmt.Errorf("collection images: %w", err)
	}
	for _, im := range rows {
		if im.Format == "jpeg" {
			out[im.CollectionID] = &model.Image{Kind: "poster", Size: im.Size, ContentHash: im.ContentHash.String,
				Width: int(im.Width.Int64), Height: int(im.Height.Int64)}
		}
	}
	return out, nil
}

// extras holds the per-item rows that complete a summary.
type extras struct {
	texts        map[string][]scopedq.ItemText
	images       map[string][]scopedq.Image
	progress     map[string]scopedq.Progress
	childCounts  map[string]int              // visible seasons of a series, visible episodes of a season
	watched      map[string]int              // completed visible episodes of a season
	allWatched   map[string]bool             // a series or podcast with visible episodes, all completed (1.22.0)
	seriesTitles map[string]string           // title of the series of a season or episode, profile language
	series       map[string]*model.SeriesRef // the series of an episode with its images
	parents      map[string]string           // parent id of an item whose parent is visible too
	watchlist    map[string]bool             // on the watchlist of the profile (concept 17 D-9)
}

// extras loads texts, images, progress, watchlist marks, child counts and series titles for a set
// of visible items; lang picks the series titles.
func (r *Reader) extras(ctx context.Context, pid int64, lang langPref, ids []string) (extras, error) {
	out := extras{texts: map[string][]scopedq.ItemText{}, images: map[string][]scopedq.Image{}, progress: map[string]scopedq.Progress{},
		childCounts: map[string]int{}, watched: map[string]int{}, allWatched: map[string]bool{}, seriesTitles: map[string]string{}, series: map[string]*model.SeriesRef{}, parents: map[string]string{},
		watchlist: map[string]bool{}}
	if len(ids) == 0 {
		return out, nil
	}
	if err := r.treeExtras(ctx, pid, lang, ids, &out); err != nil {
		return out, err
	}
	texts, err := r.q.ListItemTexts(ctx, scopedq.ListItemTextsParams{ProfileID: pid, ItemIds: ids})
	if err != nil {
		return out, fmt.Errorf("list item texts: %w", err)
	}
	for _, t := range texts {
		out.texts[t.ItemID] = append(out.texts[t.ItemID], t)
	}
	images, err := r.q.ListItemImages(ctx, scopedq.ListItemImagesParams{ProfileID: pid, ItemIds: ids})
	if err != nil {
		return out, fmt.Errorf("list item images: %w", err)
	}
	for _, im := range images {
		out.images[im.ItemID] = append(out.images[im.ItemID], im)
	}
	if err := r.podcastImages(ctx, pid, ids, &out); err != nil {
		return out, err
	}
	progress, err := r.q.ListItemProgress(ctx, scopedq.ListItemProgressParams{ProfileID: pid, ItemIds: ids})
	if err != nil {
		return out, fmt.Errorf("list item progress: %w", err)
	}
	for _, p := range progress {
		out.progress[p.ItemID] = p
	}
	marks, err := r.q.ListWatchlistMarks(ctx, scopedq.ListWatchlistMarksParams{ProfileID: pid, ItemIds: ids})
	if err != nil {
		return out, fmt.Errorf("list watchlist marks: %w", err)
	}
	for _, id := range marks {
		out.watchlist[id] = true
	}
	return out, nil
}

func (r *Reader) summaries(ctx context.Context, s Scope, pid int64, items []scopedq.Item) ([]model.ItemSummary, error) {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	ex, err := r.extras(ctx, pid, s.langs(), ids)
	if err != nil {
		return nil, err
	}
	out := make([]model.ItemSummary, 0, len(items))
	for _, it := range items {
		out = append(out, summarize(it, ex, s.langs()))
	}
	return out, nil
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	return err
}

func clamp(limit int) int64 {
	return int64(max(1, min(limit, MaxRowItems)))
}
