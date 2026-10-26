// Package discover answers the start page and the detail page (E-1, E-9) for a viewing profile.
// It reads titles only through the visibility package and opens stored images only through the
// paths package, relative to the cache directory.
//
// Start page (M7, M11, E-1, E-9, ADR 0011): the eight rows spotlight (the configured Trakt list
// intersected with the library, with the main user's curation), picks (Cat's recommendations),
// continue (continue watching), recent (recently added), released (by release date within twelve
// months, a series by its newest visible episode, S-5, S-6), watchlist, collections and genres, in
// the order the administration stored (table home_rows); hidden and empty rows are omitted, the
// spotlight row also below the configured minimum of hits, spotlight_config.min_hits (AN-3). The
// page names the row the client draws as the stage (HomePage.HeroRow: spotlight when present, else
// none; picks is never the stage) and carries hasTrailer for the rows spotlight and picks only (D3).
// The package also answers search, genres, collections, the watchlist and the favourites of a
// profile (m7.go).
// It holds no SQL on titles: reads go through visibility, the writes of watchlist and favourites
// through store/appq, whose tables name titles only by id.
package discover

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Home row keys (model.HomeRow.Key), in the default order of ADR 0011.
const (
	RowSpotlight   = model.RowSpotlight
	RowPicks       = model.RowPicks
	RowContinue    = model.RowContinue
	RowRecent      = model.RowRecent
	RowReleased    = model.RowReleased
	RowWatchlist   = model.RowWatchlist
	RowCollections = model.RowCollections
	RowGenres      = model.RowGenres
)

// MaxRows caps the start page (E-9; the contract's HomePage.rows maxItems).
const MaxRows = 8

// Service implements the catalog service of the API.
type Service struct {
	vis      *visibility.Reader
	read     *appq.Queries // home_rows
	write    *appq.Queries // watchlist, favorites
	cacheDir string
	log      *slog.Logger
	now      func() time.Time
}

// Option configures a Service.
type Option func(*Service)

// WithClock replaces time.Now: the time windows of the picks (E-2) and the time of watchlist and
// favourite entries.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// New returns the catalog service. cacheDir is the absolute cache root that image rel_paths are
// relative to; log is the redacting logger.
func New(db *store.DB, cacheDir string, log *slog.Logger, opts ...Option) *Service {
	s := &Service{read: appq.New(db.Read), write: appq.New(db.Write), cacheDir: cacheDir, log: log, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	s.vis = visibility.New(db, log, visibility.WithClock(s.now))
	return s
}

// Home returns the rows of the start page in the stored order (E-9), without hidden and empty
// rows and without a spotlight row below its configured minimum of hits; never more than MaxRows.
// HeroRow names the stage (plan 3.5): spotlight when that row is in the answer, at its stored
// place, else nothing; picks is never the stage. The titles of the rows spotlight and picks carry
// HasTrailer (D3); every other row leaves it nil.
func (s *Service) Home(ctx context.Context, profile model.Profile) (model.HomePage, error) {
	scope := visibility.Profile(profile)
	page, err := s.page(ctx, scope, false)
	if err != nil {
		return model.HomePage{}, err
	}
	for i := range page.Rows {
		if row := &page.Rows[i]; row.Key == RowSpotlight || row.Key == RowPicks {
			if err := s.markTrailers(ctx, scope, row.Items); err != nil {
				return model.HomePage{}, fmt.Errorf("home row %s: %w", row.Key, err)
			}
		}
	}
	return page, nil
}

// page builds the rows of the start page and names the stage; it is the one computation behind
// Home, FirstHero and FirstPoster. With untilHero it stops once the stage row is placed: the rows
// after it change neither the stage nor its place.
func (s *Service) page(ctx context.Context, scope visibility.Scope, untilHero bool) (model.HomePage, error) {
	keys, err := s.rowOrder(ctx)
	if err != nil {
		return model.HomePage{}, err
	}
	page := model.HomePage{Rows: make([]model.HomeRow, 0, len(keys))}
	for _, key := range keys {
		row, err := s.row(ctx, scope, key)
		if err != nil {
			return model.HomePage{}, fmt.Errorf("home row %s: %w", key, err)
		}
		if len(row.Items) > 0 || len(row.Collections) > 0 || len(row.Genres) > 0 {
			page.Rows = append(page.Rows, row)
			if key == RowSpotlight {
				page.HeroRow = RowSpotlight
				if untilHero {
					break
				}
			}
		}
		if len(page.Rows) == MaxRows {
			break
		}
	}
	return page, nil
}

// markTrailers sets HasTrailer on every title of a row from one visibility query (D3).
func (s *Service) markTrailers(ctx context.Context, scope visibility.Scope, items []model.ItemSummary) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	flags, err := s.vis.TrailerFlags(ctx, scope, ids)
	if err != nil {
		return err
	}
	for i := range items {
		has := flags[items[i].ID]
		items[i].HasTrailer = &has
	}
	return nil
}

// rowOrder returns the keys of the visible rows in their stored order (migration 00015 stores the
// default order; without rows the default order applies).
func (s *Service) rowOrder(ctx context.Context) ([]string, error) {
	rows, err := s.read.ListHomeRows(ctx)
	if err != nil {
		return nil, fmt.Errorf("list home rows: %w", err)
	}
	if len(rows) == 0 {
		return model.HomeRowKeys, nil
	}
	keys := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Hidden == 0 {
			keys = append(keys, r.Key)
		}
	}
	return keys, nil
}

// row loads one row of the start page; an unknown key gives an empty row.
func (s *Service) row(ctx context.Context, scope visibility.Scope, key string) (model.HomeRow, error) {
	row := model.HomeRow{Key: key}
	var err error
	switch key {
	case RowPicks:
		row.Items, err = s.vis.Picks(ctx, scope, visibility.MaxRowItems)
	case RowContinue:
		row.Items, err = s.vis.ContinueWatching(ctx, scope, visibility.MaxRowItems)
	case RowRecent:
		row.Items, err = s.vis.Recent(ctx, scope, visibility.MaxRowItems)
	case RowWatchlist:
		row.Items, err = s.vis.WatchlistRow(ctx, scope, visibility.MaxRowItems)
	case RowCollections:
		row.Collections, err = s.vis.Collections(ctx, scope, visibility.MaxRowItems)
	case RowGenres:
		row.Genres, err = s.vis.Genres(ctx, scope, visibility.MaxRowItems)
	case RowReleased:
		row.Items, err = s.vis.Released(ctx, scope, visibility.MaxRowItems)
	case RowSpotlight:
		row.Items, err = s.spotlightRow(ctx, scope)
	}
	return row, err
}

// spotlightRow reads the spotlight row with the configured size and omits it below the configured
// minimum of hits (AN-3, S-3). min_hits counts the hits before the cut to item_count (plan 5.3):
// "show 5 if there are at least 10" is a valid setting, not a row that never appears.
func (s *Service) spotlightRow(ctx context.Context, scope visibility.Scope) ([]model.ItemSummary, error) {
	cfg, err := s.read.GetSpotlightConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get spotlight config: %w", err)
	}
	items, hits, err := s.vis.SpotlightRow(ctx, scope, int(cfg.ItemCount))
	if err != nil {
		return nil, err
	}
	if hits < int(cfg.MinHits) {
		return nil, nil
	}
	return items, nil
}

// FirstHero returns the first title of the stage row (HomePage.HeroRow) and whether that row is
// the first row of the page, or model.ErrNotFound when the page has no stage. The app shell of the
// start page uses it to preload the stage image and, only when the stage comes first, to paint it
// ahead (P-10, plan 3.5). It derives from the same computation as Home, so it follows its rule.
func (s *Service) FirstHero(ctx context.Context, profile model.Profile) (model.ItemSummary, bool, error) {
	page, err := s.page(ctx, visibility.Profile(profile), true)
	if err != nil {
		return model.ItemSummary{}, false, err
	}
	if page.HeroRow == "" {
		return model.ItemSummary{}, false, model.ErrNotFound
	}
	i := slices.IndexFunc(page.Rows, func(r model.HomeRow) bool { return r.Key == page.HeroRow })
	return page.Rows[i].Items[0], i == 0, nil
}

// FirstPoster returns the first title of the first row of the start page when the page has no
// stage, or model.ErrNotFound when it has one (the stage image is preloaded instead, FirstHero),
// when it has no rows, or when its first row holds collections or genres rather than titles. That
// title's poster is the first image of a page without a stage, the card the client loads first
// (P-10); the app shell of the start page preloads it. It derives from the same computation as Home.
func (s *Service) FirstPoster(ctx context.Context, profile model.Profile) (model.ItemSummary, error) {
	page, err := s.page(ctx, visibility.Profile(profile), true)
	if err != nil {
		return model.ItemSummary{}, err
	}
	if page.HeroRow != "" || len(page.Rows) == 0 || len(page.Rows[0].Items) == 0 {
		return model.ItemSummary{}, model.ErrNotFound
	}
	return page.Rows[0].Items[0], nil
}

// Item returns the detail of a visible title (with its series tree, extras, cast and trailer
// count); media files carry their MIME type for the client's canPlayType check.
func (s *Service) Item(ctx context.Context, profile model.Profile, itemID string) (model.ItemDetail, error) {
	d, err := s.vis.Item(ctx, visibility.Profile(profile), itemID)
	if err != nil {
		return model.ItemDetail{}, err
	}
	for i := range d.MediaFiles {
		f := &d.MediaFiles[i]
		f.MimeType = playback.MimeType(f.Container, f.VideoCodec, f.AudioCodec)
	}
	return d, nil
}

// Trailers returns the trailers of a visible title, the profile language first, or
// model.ErrNotFound.
func (s *Service) Trailers(ctx context.Context, profile model.Profile, itemID string) ([]model.Trailer, error) {
	return s.vis.Trailers(ctx, visibility.Profile(profile), itemID)
}

// Person returns a person with the credits on titles the profile may see, or model.ErrNotFound
// when none is visible.
func (s *Service) Person(ctx context.Context, profile model.Profile, personID int64) (model.Person, error) {
	return s.vis.Person(ctx, visibility.Profile(profile), personID)
}

// PodcastEpisodes returns one page of the visible episodes of a visible podcast, newest first, and
// their total (M6); model.ErrNotFound for an invisible title or one that is no podcast.
func (s *Service) PodcastEpisodes(ctx context.Context, profile model.Profile, podcastID string, offset, limit int) ([]model.ItemSummary, int, error) {
	return s.vis.PodcastEpisodes(ctx, visibility.Profile(profile), podcastID, offset, limit)
}

// Image opens a stored image of a visible title: WebP when preferWebP is set and the WebP row
// exists, JPEG otherwise. The caller closes the file.
func (s *Service) Image(ctx context.Context, profile model.Profile, itemID, kind, size string, preferWebP bool) (model.StreamSource, error) {
	loc, err := s.vis.Image(ctx, visibility.Profile(profile), itemID, kind, size, preferWebP)
	if err != nil {
		return model.StreamSource{}, err
	}
	root, err := paths.NewRoot(s.cacheDir)
	if err != nil {
		return model.StreamSource{}, err
	}
	dir, err := paths.Open(root)
	if err != nil {
		return model.StreamSource{}, err
	}
	defer dir.Close() // files opened through the root stay usable after it is closed
	f, err := dir.Open(loc.RelPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.log.WarnContext(ctx, "image row without file", "item_id", itemID, "kind", kind, "size", size)
			return model.StreamSource{}, model.ErrNotFound
		}
		return model.StreamSource{}, err
	}
	info, err := f.Stat()
	if err != nil {
		return model.StreamSource{}, errors.Join(fmt.Errorf("stat image: %w", err), f.Close())
	}
	return model.StreamSource{
		File:      f,
		MediaType: loc.MediaType,
		Size:      info.Size(),
		ModTime:   time.UnixMilli(loc.ModTime).UTC(),
		Name:      itemID + "/" + kind + "/" + size,
	}, nil
}
