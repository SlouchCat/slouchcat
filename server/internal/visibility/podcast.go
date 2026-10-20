package visibility

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Podcasts (M6, design 3.4). Stream B fills the details: the newest visible episodes of a podcast
// with their total, and the podcast of an episode (queries/scoped/podcasts.sql).

// podcastDetail completes the detail of a podcast container.
func (r *Reader) podcastDetail(_ context.Context, _ Scope, _ int64, _ scopedq.Item, d *model.ItemDetail) error {
	d.Episodes = []model.ItemSummary{}
	n := 0
	d.EpisodeCount = &n
	return nil
}

// podcastEpisodeDetail completes the detail of a podcast episode.
func (r *Reader) podcastEpisodeDetail(_ context.Context, _ Scope, _ int64, _ scopedq.Item, _ *model.ItemDetail) error {
	return nil
}
