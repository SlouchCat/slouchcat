package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Trickplay estimate (concept 18, 9): the library page shows what the previews of a video library
// take per film, per episode and for the whole library, next to what the stored sets take today and
// the free space of the cache drive.

// trickplayBytesPerHour is the size of the previews of one hour of video (measurement 3.6: 1.3 MB
// at 16:9, 1.8 MB at 2.40:1; decimal megabytes).
const trickplayBytesPerHour = 1_500_000

// TrickplayUsage is implemented by package library: the bytes of the stored trickplay sets of a
// library per kind, counted by the hourly library.sweep (0 before the first count), and the free
// space of the cache drive (-1: unknown).
type TrickplayUsage interface {
	TrickplayUsage(libraryID int64) (movies, series, cacheFree int64)
}

// WithTrickplay sets the trickplay usage of package library and returns the receiver.
func (s *Service) WithTrickplay(t TrickplayUsage) *Service {
	s.trickplay = t
	return s
}

// trickplayEstimate computes the estimate of a video library from the running time of its present
// media files.
func (s *Service) trickplayEstimate(ctx context.Context, libraryID int64) (*model.LibraryTrickplayEstimate, error) {
	rows, err := s.sys.AdminTrickplayDurations(ctx, libraryID)
	if err != nil {
		return nil, fmt.Errorf("trickplay durations of library %d: %w", libraryID, err)
	}
	est := &model.LibraryTrickplayEstimate{CacheFreeBytes: -1}
	for _, r := range rows {
		k := model.TrickplayKindEstimate{Titles: int(r.Titles), Files: int(r.Files), DurationMs: r.DurationMs}
		k.LibraryBytes = trickplayBytes(r.DurationMs)
		if r.Files > 0 {
			k.PerTitleBytes = trickplayBytes(r.DurationMs / r.Files)
		}
		switch r.Kind {
		case "movie":
			est.Movies = k
		case "episode":
			est.Series = k
		}
	}
	if s.trickplay != nil {
		est.Movies.StoredBytes, est.Series.StoredBytes, est.CacheFreeBytes = s.trickplay.TrickplayUsage(libraryID)
	}
	return est, nil
}

// trickplayBytes is the estimated size of the previews of durationMs of video.
func trickplayBytes(durationMs int64) int64 {
	return durationMs * trickplayBytesPerHour / time.Hour.Milliseconds()
}
