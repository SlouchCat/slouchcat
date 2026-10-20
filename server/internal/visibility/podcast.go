package visibility

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Podcasts (M6, design 3.4): the newest visible episodes of a podcast with their total, the podcast
// of an episode, the enclosure of an episode and the cover of the podcast for episodes without an
// image of their own (queries/scoped/podcasts.sql). An explicit episode (FSK 18) is hidden from
// profiles under 18 like any other title above their ceiling.

// Episode lists of a podcast: the detail carries the newest DetailEpisodes, listPodcastEpisodes
// pages with at most MaxEpisodePage (DefaultEpisodePage without a limit).
const (
	DetailEpisodes     = 50
	DefaultEpisodePage = 50
	MaxEpisodePage     = 200
)

// podcastDetail completes the detail of a podcast container: the newest visible episodes and their
// total.
func (r *Reader) podcastDetail(ctx context.Context, s Scope, pid int64, it scopedq.Item, d *model.ItemDetail) error {
	eps, total, err := r.podcastEpisodes(ctx, s, pid, it.ID, 0, DetailEpisodes)
	if err != nil {
		return err
	}
	d.Episodes = eps
	d.EpisodeCount = &total
	return nil
}

// podcastEpisodeDetail completes the detail of a podcast episode with its podcast, when the
// profile sees it (it does whenever it sees the episode: the episode's rating is at least the
// podcast's).
func (r *Reader) podcastEpisodeDetail(ctx context.Context, s Scope, pid int64, it scopedq.Item, d *model.ItemDetail) error {
	if !it.ParentID.Valid {
		return nil
	}
	parents, err := r.list(ctx, s, pid, "podcast")(r.q.ListVisibleItems(ctx, scopedq.ListVisibleItemsParams{ProfileID: pid, ItemIds: []string{it.ParentID.String}}))
	if err != nil {
		return err
	}
	if len(parents) == 1 && parents[0].Kind == "podcast" {
		d.Podcast = &parents[0]
	}
	return nil
}

// PodcastEpisodes returns one page of the visible episodes of a visible podcast, newest first, and
// the number of visible episodes. limit is clamped to 1..MaxEpisodePage (0 = DefaultEpisodePage),
// offset to 0 and above. An unknown or invisible title and a title that is no podcast are
// model.ErrNotFound.
func (r *Reader) PodcastEpisodes(ctx context.Context, s Scope, podcastID string, offset, limit int) ([]model.ItemSummary, int, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, 0, err
	}
	it, err := r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: podcastID})
	if err != nil {
		return nil, 0, notFound(err)
	}
	if it.Kind != "podcast" {
		return nil, 0, model.ErrNotFound
	}
	if limit <= 0 {
		limit = DefaultEpisodePage
	}
	return r.podcastEpisodes(ctx, s, pid, it.ID, max(offset, 0), min(limit, MaxEpisodePage))
}

func (r *Reader) podcastEpisodes(ctx context.Context, s Scope, pid int64, podcastID string, offset, limit int) ([]model.ItemSummary, int, error) {
	parent := sql.NullString{String: podcastID, Valid: true}
	total, err := r.q.CountVisiblePodcastEpisodes(ctx, scopedq.CountVisiblePodcastEpisodesParams{ProfileID: pid, PodcastID: parent})
	if err != nil {
		return nil, 0, fmt.Errorf("count podcast episodes: %w", err)
	}
	eps, err := r.list(ctx, s, pid, "podcast episodes")(r.q.ListVisiblePodcastEpisodes(ctx, scopedq.ListVisiblePodcastEpisodesParams{
		ProfileID: pid, PodcastID: parent, LimitCount: int64(limit), OffsetCount: int64(offset),
	}))
	if err != nil {
		return nil, 0, err
	}
	return eps, int(total), nil
}

// NextPodcastEpisodes returns the visible episodes of a podcast published after the moment, newest
// first, at most limit (auto-downloads of new episodes, O-3). Episodes without a publication date
// never count as new.
func (r *Reader) NextPodcastEpisodes(ctx context.Context, s Scope, podcastID string, after time.Time, limit int) ([]model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	return r.list(ctx, s, pid, "new podcast episodes")(r.q.ListNextVisiblePodcastEpisodes(ctx, scopedq.ListNextVisiblePodcastEpisodesParams{
		ProfileID: pid, PodcastID: sql.NullString{String: podcastID, Valid: true},
		After: sql.NullInt64{Int64: after.UnixMilli(), Valid: true}, LimitCount: int64(max(limit, 1)),
	}))
}

// Enclosure is the remote media file of a visible podcast episode.
type Enclosure struct {
	ItemID     string
	URL        string
	Type       string // MIME type the feed names; empty when it names none
	Length     int64  // bytes the feed names; 0 when unknown
	DurationMs int64  // itunes:duration; 0 when unknown
}

// Enclosure returns the enclosure of a visible podcast episode; model.ErrNotFound for an unknown or
// invisible title, another kind and an episode without enclosure.
func (r *Reader) Enclosure(ctx context.Context, s Scope, itemID string) (Enclosure, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return Enclosure{}, err
	}
	row, err := r.q.GetVisibleEnclosure(ctx, scopedq.GetVisibleEnclosureParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return Enclosure{}, notFound(err)
	}
	return Enclosure{ItemID: row.ID, URL: row.EnclosureUrl.String, Type: row.EnclosureType.String,
		Length: row.EnclosureLength.Int64, DurationMs: row.DurationMs.Int64}, nil
}

// podcastImages gives the visible episodes among ids that have no image of their own the images of
// their visible podcast.
func (r *Reader) podcastImages(ctx context.Context, pid int64, ids []string, out *extras) error {
	rows, err := r.q.ListPodcastFallbackImages(ctx, scopedq.ListPodcastFallbackImagesParams{ProfileID: pid, ItemIds: ids})
	if err != nil {
		return fmt.Errorf("list podcast cover images: %w", err)
	}
	for _, row := range rows {
		out.images[row.EpisodeID] = append(out.images[row.EpisodeID], row.Image)
	}
	return nil
}

// podcastImage is the image lookup of Image for an episode without images of its own: the image
// of its podcast, or sql.ErrNoRows.
func (r *Reader) podcastImage(ctx context.Context, pid int64, itemID, kind, size, format string) (scopedq.Image, error) {
	im, err := r.q.GetPodcastFallbackImage(ctx, scopedq.GetPodcastFallbackImageParams{ProfileID: pid, ItemID: itemID, Kind: kind, Size: size, Format: format})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return scopedq.Image{}, fmt.Errorf("get podcast cover image: %w", err)
	}
	return im, err
}
