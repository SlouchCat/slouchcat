// Package discover answers the start page and the detail page (E-1, E-9) for a viewing profile.
// It reads titles only through the visibility package and opens stored images only through the
// paths package, relative to the cache directory.
//
// M1a start page: the rows picks (Cat's recommendations, placeholder content until curation in
// M7), continue (continue watching) and recent (recently added), in this order; empty rows are
// omitted. Search, watchlist, favourites and collections follow in later milestones.
package discover

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Home row keys in display order.
const (
	RowPicks    = "picks"
	RowContinue = "continue"
	RowRecent   = "recent"
)

// Service implements the catalog service of the API.
type Service struct {
	vis      *visibility.Reader
	cacheDir string
	log      *slog.Logger
}

// New returns the catalog service. cacheDir is the absolute cache root that image rel_paths are
// relative to; log is the redacting logger.
func New(db *store.DB, cacheDir string, log *slog.Logger) *Service {
	return &Service{vis: visibility.New(db, log), cacheDir: cacheDir, log: log}
}

// Home returns the rows of the start page in display order, without empty rows.
func (s *Service) Home(ctx context.Context, profile model.Profile) ([]model.HomeRow, error) {
	scope := visibility.Profile(profile)
	rows := []struct {
		key  string
		load func(context.Context, visibility.Scope, int) ([]model.ItemSummary, error)
	}{
		{RowPicks, s.vis.Picks},
		{RowContinue, s.vis.ContinueWatching},
		{RowRecent, s.vis.Recent},
	}
	out := make([]model.HomeRow, 0, len(rows))
	for _, r := range rows {
		items, err := r.load(ctx, scope, visibility.MaxRowItems)
		if err != nil {
			return nil, fmt.Errorf("home row %s: %w", r.key, err)
		}
		if len(items) > 0 {
			out = append(out, model.HomeRow{Key: r.key, Items: items})
		}
	}
	return out, nil
}

// FirstPick returns the first title of the picks row, the slide the start page's hero carousel
// shows first, or model.ErrNotFound when the row is empty. The app shell of the start page uses it
// to preload the hero image (P-10); the order is the one of Home.
func (s *Service) FirstPick(ctx context.Context, profile model.Profile) (model.ItemSummary, error) {
	items, err := s.vis.Picks(ctx, visibility.Profile(profile), 1)
	if err != nil {
		return model.ItemSummary{}, fmt.Errorf("first pick: %w", err)
	}
	if len(items) == 0 {
		return model.ItemSummary{}, model.ErrNotFound
	}
	return items[0], nil
}

// Item returns the detail of a visible title; media files carry their MIME type for the client's
// canPlayType check.
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

// Image opens a stored image of a visible title. The caller closes the file.
func (s *Service) Image(ctx context.Context, profile model.Profile, itemID, kind, size string) (model.StreamSource, error) {
	loc, err := s.vis.Image(ctx, visibility.Profile(profile), itemID, kind, size)
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
