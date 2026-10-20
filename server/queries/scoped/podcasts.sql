-- Scoped podcast queries (package store/scopedq, M6 design 3.4), used only by the visibility package.
-- Every query joins visible_items for the given profile. Episodes are ordered newest first: by
-- publication date, episodes without a date after the dated ones, then by creation.

-- name: ListVisiblePodcastEpisodes :many
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.parent_id = @podcast_id AND i.kind = 'podcast_episode'
ORDER BY i.published_at IS NULL, i.published_at DESC, i.created_at DESC, i.id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountVisiblePodcastEpisodes :one
SELECT count(*) FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.parent_id = @podcast_id AND i.kind = 'podcast_episode';

-- Visible episodes published after a moment, newest first (auto-downloads of new episodes, O-3).
-- name: ListNextVisiblePodcastEpisodes :many
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.parent_id = @podcast_id AND i.kind = 'podcast_episode' AND i.published_at > @after
ORDER BY i.published_at DESC, i.id DESC
LIMIT @limit_count;

-- The enclosure of a visible episode (stream proxy and download source).
-- name: GetVisibleEnclosure :one
SELECT i.id, i.enclosure_url, i.enclosure_type, i.enclosure_length, i.duration_ms FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.id = @item_id AND i.kind = 'podcast_episode' AND i.enclosure_url IS NOT NULL;

-- Images of the visible podcast of visible episodes that have no image of their own (the episode
-- shows the cover of its podcast).
-- name: ListPodcastFallbackImages :many
SELECT i.id AS episode_id, sqlc.embed(im) FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
JOIN visible_items AS vp ON vp.item_id = i.parent_id AND vp.profile_id = @profile_id
JOIN images AS im ON im.item_id = i.parent_id AND im.size <> 'original'
WHERE i.id IN (sqlc.slice('item_ids')) AND i.kind = 'podcast_episode'
  AND NOT EXISTS (SELECT 1 FROM images AS own WHERE own.item_id = i.id AND own.size <> 'original')
ORDER BY i.id, im.kind, im.size, im.format;

-- One image of the visible podcast of a visible episode without images of its own, like
-- GetItemImage (WebP when asked and stored, else JPEG).
-- name: GetPodcastFallbackImage :one
SELECT im.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
JOIN visible_items AS vp ON vp.item_id = i.parent_id AND vp.profile_id = @profile_id
JOIN images AS im ON im.item_id = i.parent_id
WHERE i.id = @item_id AND i.kind = 'podcast_episode'
  AND NOT EXISTS (SELECT 1 FROM images AS own WHERE own.item_id = i.id AND own.size <> 'original')
  AND im.kind = @kind AND im.size = @size AND im.size <> 'original' AND im.format IN (@format, 'jpeg')
ORDER BY im.format = 'jpeg'
LIMIT 1;
