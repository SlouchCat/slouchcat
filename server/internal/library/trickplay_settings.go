package library

import (
	"context"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// LibraryTrickplay returns the trickplay setting of a library row (migration 00042).
func LibraryTrickplay(l appq.Library) model.LibraryTrickplay {
	return model.LibraryTrickplay{
		Movies: model.TrickplaySetting{Mode: l.TrickplayMovies, Keep: l.TrickplayMoviesKeep == 1},
		Series: model.TrickplaySetting{Mode: l.TrickplaySeries, Keep: l.TrickplaySeriesKeep == 1, Scope: l.TrickplaySeriesScope},
	}
}

// SetLibraryTrickplay stores the trickplay setting t of library id on q (the caller's transaction).
func SetLibraryTrickplay(ctx context.Context, q *appq.Queries, id int64, t model.LibraryTrickplay, now int64) error {
	if err := q.SetLibraryTrickplay(ctx, appq.SetLibraryTrickplayParams{
		Movies: t.Movies.Mode, MoviesKeep: boolInt(t.Movies.Keep),
		Series: t.Series.Mode, SeriesKeep: boolInt(t.Series.Keep), SeriesScope: t.Series.Scope,
		Now: now, ID: id,
	}); err != nil {
		return fmt.Errorf("store trickplay setting of library %d: %w", id, err)
	}
	return nil
}
