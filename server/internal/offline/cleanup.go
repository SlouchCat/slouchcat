package offline

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Cleanup of the download directory (AN-20, M5 design 3.3): server copies 24 h after the device
// fetched them and 7 days after they were ready without a fetch (evicted; the next sync of the
// device prepares them again while it lacks the file), the partial files of failed preparations,
// directories without a download row and tombstones older than 400 days. The copy on the device
// has no expiry (O-6).

// Run cleans up at once and then every 10 minutes until ctx ends.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(cleanupInterval)
	defer t.Stop()
	for {
		if err := s.Cleanup(ctx); err != nil && ctx.Err() == nil {
			s.log.WarnContext(ctx, "download cleanup", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Cleanup runs one pass of the cleanup.
func (s *Service) Cleanup(ctx context.Context) error {
	now := s.now()
	if err := s.evict(ctx, now); err != nil {
		return err
	}
	if err := s.removeFailed(ctx, now); err != nil {
		return err
	}
	if err := s.removeOrphans(ctx); err != nil {
		return err
	}
	for {
		n, err := s.write.DeleteOldTombstones(ctx, ms(now.Add(-tombstoneKeep)))
		if err != nil {
			return fmt.Errorf("delete old tombstones: %w", err)
		}
		if n < writeBatch {
			return nil
		}
	}
}

// evict removes the expired server copies in batches.
func (s *Service) evict(ctx context.Context, now time.Time) error {
	for {
		ids, err := s.write.ListDownloadsToEvict(ctx, systemq.ListDownloadsToEvictParams{
			FetchedBefore: nullMs(now.Add(-fetchedKeep)), ReadyBefore: nullMs(now.Add(-unfetchedKeep)),
		})
		if err != nil {
			return fmt.Errorf("list downloads to evict: %w", err)
		}
		for _, id := range ids {
			if _, err := s.write.EvictDownload(ctx, systemq.EvictDownloadParams{Now: ms(now), ID: id}); err != nil {
				return fmt.Errorf("evict download: %w", err)
			}
			s.removeFiles(ctx, id)
			s.log.InfoContext(ctx, "download copy evicted", "download_id", id)
		}
		if len(ids) < writeBatch {
			return nil
		}
	}
}

// removeFailed removes the partial files of failed preparations.
func (s *Service) removeFailed(ctx context.Context, now time.Time) error {
	for {
		ids, err := s.write.ListFailedDownloadFiles(ctx)
		if err != nil {
			return fmt.Errorf("list failed downloads: %w", err)
		}
		for _, id := range ids {
			s.removeFiles(ctx, id)
			if err := s.write.SetDownloadFilesRemoved(ctx, systemq.SetDownloadFilesRemovedParams{Now: nullMs(now), ID: id}); err != nil {
				return fmt.Errorf("record removed files: %w", err)
			}
		}
		if len(ids) < writeBatch {
			return nil
		}
	}
}

// removeOrphans deletes the directories whose download row is gone (deleted while the job wrote,
// or a database restored from a backup).
func (s *Service) removeOrphans(ctx context.Context) error {
	if s.dir == "" {
		return nil
	}
	root, err := s.root()
	if err != nil {
		return err
	}
	defer root.Close()
	d, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("open downloads directory: %w", err)
	}
	entries, err := d.ReadDir(-1)
	_ = d.Close()
	if err != nil {
		return fmt.Errorf("read downloads directory: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && downloadID.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	for chunk := range slices.Chunk(names, writeBatch) {
		existing, err := s.write.ListExistingDownloadIDs(ctx, chunk)
		if err != nil {
			return fmt.Errorf("list downloads: %w", err)
		}
		for _, name := range chunk {
			if !slices.Contains(existing, name) {
				s.removeFiles(ctx, name)
				s.log.InfoContext(ctx, "orphaned download directory removed", "download_id", name)
			}
		}
	}
	return nil
}

func nullMs(t time.Time) sql.NullInt64 { return sql.NullInt64{Int64: ms(t), Valid: true} }
