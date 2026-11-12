package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// A podcast episode has no cover of its own, only its podcast has one (decision of 11 November
// 2026). Migration 00049 emptied cover_url and cover_path of every episode and deleted their image
// rows; the files below cache/images/<episode id>/ go once, at the first start after it.

// SettingEpisodeImagesPurged marks that the image folders of the podcast episodes were removed
// after migration 00049.
const SettingEpisodeImagesPurged = "metadata.episode_images_purged"

// PurgeEpisodeImagesOnce removes the image folder of every podcast episode (soft-deleted ones
// included) from the image cache, once: the setting SettingEpisodeImagesPurged remembers a run
// that removed every folder; a run that failed on a folder or was stopped starts again at the next
// start. It returns the number of folders removed. Nothing else writes these folders any more
// (image.fetch has no image kind for an episode), so the run takes no image lock.
func (s *Service) PurgeEpisodeImagesOnce(ctx context.Context) (int, error) {
	if _, err := s.read.GetSystemSetting(ctx, SettingEpisodeImagesPurged); err == nil {
		return 0, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("read %s: %w", SettingEpisodeImagesPurged, err)
	}
	if s.cacheE != nil {
		return 0, s.cacheE
	}
	ids, err := s.read.ListPodcastEpisodeIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list podcast episodes: %w", err)
	}
	root, err := paths.Open(s.cache)
	if err != nil {
		return 0, fmt.Errorf("episode images: %w", err)
	}
	defer root.Close()
	removed, failed := 0, 0
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return removed, err
		}
		dir := imagesDir + "/" + id
		if _, err := root.Lstat(dir); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := root.RemoveAll(dir); err != nil { //nolint:forbidigo // the cache root (images), never a library root (ADR 0032)
			s.log.Warn("remove the images of a podcast episode", "item_id", id, "err", err)
			failed++
			continue
		}
		removed++
	}
	if failed > 0 {
		return removed, fmt.Errorf("%d image folders of podcast episodes not removed", failed)
	}
	if err := s.write.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{
		Key: SettingEpisodeImagesPurged, Value: "1", Now: s.now().UnixMilli(),
	}); err != nil {
		return removed, fmt.Errorf("write %s: %w", SettingEpisodeImagesPurged, err)
	}
	if removed > 0 {
		s.log.Info("image folders of podcast episodes removed", "count", removed)
	}
	return removed, nil
}
