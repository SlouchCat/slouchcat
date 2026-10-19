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
// unfiltered scope that later milestones will accept for internal callers; the functions of M1a
// reject it.
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
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// MaxRowItems caps every list (home rows) at 20 titles.
const MaxRowItems = 20

// placeholderPicks is the size of the carousel placeholder while no top pick is visible.
const placeholderPicks = 6

// ErrUnscoped is returned when a function that needs a profile receives the unfiltered system
// scope or the zero Scope.
var ErrUnscoped = errors.New("visibility: a profile scope is required")

// Scope is the viewer a read runs for. Build it with Profile or System; the zero value is
// rejected.
type Scope struct {
	profileID int64
	language  string
	system    bool
	reason    string
}

// Profile returns the scope of a viewing profile: its id selects the rows of visible_items, its
// language selects the texts.
func Profile(p model.Profile) Scope {
	lang := p.Language
	if lang != "de" && lang != "en" {
		lang = "de"
	}
	return Scope{profileID: p.ID, language: lang}
}

// System returns an unfiltered scope for internal callers; reason is written to the debug log when
// the scope is used. The analyzer itemsaccess allows calling it only in visibility, admin,
// library, metadata, jobs, notify and offline. In M1a every function of this package rejects it
// with ErrUnscoped, because no viewer-facing read may run unfiltered.
func System(reason string) Scope {
	return Scope{system: true, reason: reason}
}

// Reader runs scoped reads against the reader pool of the database.
type Reader struct {
	q   *scopedq.Queries
	log *slog.Logger
	now func() time.Time
}

// New returns a Reader on the read connections of db; log is the redacting logger.
func New(db *store.DB, log *slog.Logger) *Reader {
	return &Reader{q: scopedq.New(db.Read), log: log, now: time.Now}
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
	extras, err := r.extras(ctx, pid, []string{it.ID})
	if err != nil {
		return model.ItemSummary{}, err
	}
	return summarize(it, extras, s.language), nil
}

// Item returns the detail of a visible title with its media files.
func (r *Reader) Item(ctx context.Context, s Scope, itemID string) (model.ItemDetail, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.ItemDetail{}, err
	}
	it, err := r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return model.ItemDetail{}, notFound(err)
	}
	extras, err := r.extras(ctx, pid, []string{it.ID})
	if err != nil {
		return model.ItemDetail{}, err
	}
	files, err := r.q.ListMediaFilesForItem(ctx, scopedq.ListMediaFilesForItemParams{ProfileID: pid, ItemID: it.ID})
	if err != nil {
		return model.ItemDetail{}, fmt.Errorf("list media files: %w", err)
	}
	d := detail(it, extras, s.language)
	d.MediaFiles = make([]model.MediaFile, 0, len(files))
	for _, f := range files {
		d.MediaFiles = append(d.MediaFiles, mediaFile(f))
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

// ContinueWatching returns started, not completed titles of the profile, most recent first.
func (r *Reader) ContinueWatching(ctx context.Context, s Scope, limit int) ([]model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	items, err := r.q.ListContinueWatching(ctx, scopedq.ListContinueWatchingParams{ProfileID: pid, LimitCount: clamp(limit)})
	if err != nil {
		return nil, fmt.Errorf("list continue watching: %w", err)
	}
	return r.summaries(ctx, s, pid, items)
}

// Picks returns Cat's recommendations (E-1) that the profile may see, in their curated order.
//
// M1a placeholder: curation arrives in M7. While no top pick is visible for the profile, Picks
// returns the newest visible titles that have a backdrop (at most 6), so the carousel is never
// empty for a library with images.
func (r *Reader) Picks(ctx context.Context, s Scope, limit int) ([]model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	items, err := r.q.ListTopPicks(ctx, scopedq.ListTopPicksParams{ProfileID: pid, Now: sql.NullInt64{Int64: r.now().UnixMilli(), Valid: true}, LimitCount: clamp(limit)})
	if err != nil {
		return nil, fmt.Errorf("list top picks: %w", err)
	}
	items = dedupe(items)
	if len(items) == 0 {
		items, err = r.q.ListNewestWithBackdrop(ctx, scopedq.ListNewestWithBackdropParams{ProfileID: pid, LimitCount: min(clamp(limit), placeholderPicks)})
		if err != nil {
			return nil, fmt.Errorf("list placeholder picks: %w", err)
		}
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
}

// MediaFile returns a media file of a visible title. An empty fileID selects the primary file: the
// oldest file that is not missing. A file of another title, an unknown file and a title without an
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
			return MediaFileLocation{ItemID: f.ItemID, File: mediaFile(f), RootPath: f.RootPath, RelPath: f.RelPath}, nil
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
// w1920). With preferWebP it returns the WebP row when one exists, else (and without preferWebP)
// the JPEG row. A missing w400 poster falls back to M1a's w500 JPEG (legacyPosterSize).
func (r *Reader) Image(ctx context.Context, s Scope, itemID, kind, size string, preferWebP bool) (ImageLocation, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return ImageLocation{}, err
	}
	format := "jpeg"
	if preferWebP {
		format = "webp"
	}
	im, err := r.q.GetItemImage(ctx, scopedq.GetItemImageParams{ProfileID: pid, ItemID: itemID, Kind: kind, Size: size, Format: format})
	if errors.Is(err, sql.ErrNoRows) && kind == "poster" && size == posterSize {
		im, err = r.q.GetItemImage(ctx, scopedq.GetItemImageParams{ProfileID: pid, ItemID: itemID, Kind: kind, Size: legacyPosterSize, Format: format})
	}
	if err != nil {
		return ImageLocation{}, notFound(err)
	}
	return ImageLocation{RelPath: im.RelPath, MediaType: im.MediaType, ContentHash: im.ContentHash, Bytes: im.Bytes, ModTime: im.CreatedAt}, nil
}

// extras holds the per-item rows that complete a summary.
type extras struct {
	texts    map[string][]scopedq.ItemText
	images   map[string][]scopedq.Image
	progress map[string]scopedq.Progress
}

// extras loads texts, images and progress for a set of visible items in three queries.
func (r *Reader) extras(ctx context.Context, pid int64, ids []string) (extras, error) {
	out := extras{texts: map[string][]scopedq.ItemText{}, images: map[string][]scopedq.Image{}, progress: map[string]scopedq.Progress{}}
	if len(ids) == 0 {
		return out, nil
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
	progress, err := r.q.ListItemProgress(ctx, scopedq.ListItemProgressParams{ProfileID: pid, ItemIds: ids})
	if err != nil {
		return out, fmt.Errorf("list item progress: %w", err)
	}
	for _, p := range progress {
		out.progress[p.ItemID] = p
	}
	return out, nil
}

func (r *Reader) summaries(ctx context.Context, s Scope, pid int64, items []scopedq.Item) ([]model.ItemSummary, error) {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	ex, err := r.extras(ctx, pid, ids)
	if err != nil {
		return nil, err
	}
	out := make([]model.ItemSummary, 0, len(items))
	for _, it := range items {
		out = append(out, summarize(it, ex, s.language))
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

func dedupe(items []scopedq.Item) []scopedq.Item {
	seen := make(map[string]bool, len(items))
	out := items[:0]
	for _, it := range items {
		if !seen[it.ID] {
			seen[it.ID] = true
			out = append(out, it)
		}
	}
	return out
}
