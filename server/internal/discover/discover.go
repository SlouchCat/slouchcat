// Package discover answers the start page and the detail page (E-1, E-9) for a viewing profile.
// It reads titles only through the visibility package and opens stored images only through the
// paths package, relative to the cache directory.
//
// Start page (M7, E-1, E-9): the eight rows picks (Cat's recommendations), continue (continue
// watching), recent (recently added), watchlist, collections, genres and the Trakt rows trending
// and popular, in the order the administration stored (table home_rows); hidden and empty rows are
// omitted, a Trakt row also below TraktMinHits titles (AN-3). The package also answers search,
// genres, collections, the watchlist and the favourites of a profile (m7.go). It holds no SQL on
// titles: reads go through visibility, the writes of watchlist and favourites through store/appq,
// whose tables name titles only by id.
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

// Home row keys (model.HomeRow.Key), in the default order of E-1.
const (
	RowPicks         = model.RowPicks
	RowContinue      = model.RowContinue
	RowRecent        = model.RowRecent
	RowWatchlist     = model.RowWatchlist
	RowCollections   = model.RowCollections
	RowGenres        = model.RowGenres
	RowTraktTrending = model.RowTraktTrending
	RowTraktPopular  = model.RowTraktPopular
)

// TraktMinHits is the smallest number of visible library titles a Trakt row needs; with fewer the
// row is omitted (AN-3).
const TraktMinHits = 5

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
// rows and without a Trakt row of fewer than TraktMinHits titles; never more than MaxRows.
func (s *Service) Home(ctx context.Context, profile model.Profile) ([]model.HomeRow, error) {
	scope := visibility.Profile(profile)
	keys, err := s.rowOrder(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.HomeRow, 0, len(keys))
	for _, key := range keys {
		row, err := s.row(ctx, scope, key)
		if err != nil {
			return nil, fmt.Errorf("home row %s: %w", key, err)
		}
		if len(row.Items) > 0 || len(row.Collections) > 0 || len(row.Genres) > 0 {
			out = append(out, row)
		}
		if len(out) == MaxRows {
			break
		}
	}
	return out, nil
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
	case RowTraktTrending, RowTraktPopular:
		list := visibility.TraktTrending
		if key == RowTraktPopular {
			list = visibility.TraktPopular
		}
		row.Items, err = s.vis.TraktRow(ctx, scope, list, visibility.MaxRowItems)
		if len(row.Items) < TraktMinHits {
			row.Items = nil
		}
	}
	return row, err
}

// FirstPick returns the first title of the picks row, the slide the start page's hero carousel
// shows first, or model.ErrNotFound when the row is empty or hidden. The app shell of the start
// page uses it to preload the hero image (P-10); the order is the one of Home.
func (s *Service) FirstPick(ctx context.Context, profile model.Profile) (model.ItemSummary, error) {
	keys, err := s.rowOrder(ctx)
	if err != nil {
		return model.ItemSummary{}, err
	}
	if !slices.Contains(keys, RowPicks) {
		return model.ItemSummary{}, model.ErrNotFound
	}
	items, err := s.vis.Picks(ctx, visibility.Profile(profile), 1)
	if err != nil {
		return model.ItemSummary{}, fmt.Errorf("first pick: %w", err)
	}
	if len(items) == 0 {
		return model.ItemSummary{}, model.ErrNotFound
	}
	return items[0], nil
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
