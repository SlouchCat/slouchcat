-- Podcast episodes without a cover of their own (decision of 11 November 2026): only the podcast
-- has a cover, every episode shows its podcast's. Since M6 (review fix of 20 October 2026) a feed
-- episode kept its itunes:image when it differed from the channel's, and image.fetch stored it in
-- seven files; on the test server that was 12 842 episodes with 7.7 GB of originals alone, most of
-- them the same picture under different addresses. The server no longer stores or fetches one.
--
--   items.cover_url, items.cover_path  emptied on every podcast episode
--   images, item_images,               the rows of every podcast episode go; the files below
--   item_local_images                  cache/images/<episode id>/ are removed by the server at
--                                      its first start after this migration (metadata
--                                      PurgeEpisodeImagesOnce), not here
--   items.locked_fields                the lock "poster" leaves the episodes (nothing to lock)
--   jobs                               queued image.fetch jobs of episodes go (they have nothing
--                                      to do any more); a running one ends without work
--
-- The number follows 00047 of concept 19 part A (PR #141) and 00048 of part B (PR #140), which
-- must be merged first.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

UPDATE items SET cover_url = NULL, cover_path = NULL
WHERE kind = 'podcast_episode' AND (cover_url IS NOT NULL OR cover_path IS NOT NULL);

UPDATE items
SET locked_fields = (SELECT json_group_array(j.value) FROM json_each(items.locked_fields) AS j WHERE j.value <> 'poster')
WHERE kind = 'podcast_episode' AND json_valid(locked_fields)
  AND EXISTS (SELECT 1 FROM json_each(items.locked_fields) AS j WHERE j.value = 'poster');

DELETE FROM images WHERE item_id IN (SELECT id FROM items WHERE kind = 'podcast_episode');
DELETE FROM item_images WHERE item_id IN (SELECT id FROM items WHERE kind = 'podcast_episode');
DELETE FROM item_local_images WHERE item_id IN (SELECT id FROM items WHERE kind = 'podcast_episode');

DELETE FROM jobs
WHERE type = 'image.fetch' AND state = 'queued'
  AND idempotency_key IN (SELECT 'image:' || id FROM items WHERE kind = 'podcast_episode');

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
