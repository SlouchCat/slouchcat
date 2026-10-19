-- Scoped title queries (package store/scopedq), used only by the visibility package. Every query
-- joins visible_items for the given profile; the test in internal/store enforces it for this folder.
-- Times are Unix milliseconds.

-- name: GetVisibleItem :one
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.id = @item_id;

-- name: ListRecentlyAdded :many
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.kind IN ('movie', 'series')
ORDER BY i.added_at DESC, i.id DESC
LIMIT @limit_count;

-- Started but not completed titles of the profile, most recently watched first (E-1 "Weiterschauen").
-- name: ListContinueWatching :many
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
JOIN progress AS p ON p.item_id = i.id AND p.profile_id = @profile_id
WHERE p.completed = 0 AND p.position_ms > 0
ORDER BY p.updated_at DESC, i.id DESC
LIMIT @limit_count;

-- Cat's recommendations (E-1) for the profile: global rows (profile_id NULL) and rows of the profile,
-- inside their optional time window. An item listed twice is deduplicated by the caller.
-- name: ListTopPicks :many
SELECT i.* FROM top_picks AS t
JOIN items AS i ON i.id = t.item_id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE (t.profile_id IS NULL OR t.profile_id = @profile_id)
  AND (t.starts_at IS NULL OR t.starts_at <= @now)
  AND (t.ends_at IS NULL OR t.ends_at > @now)
ORDER BY t.position, t.id
LIMIT @limit_count;

-- Placeholder for the carousel while top_picks has nothing for the profile (M1a): newest titles
-- that have a backdrop.
-- name: ListNewestWithBackdrop :many
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.kind IN ('movie', 'series')
  AND EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'backdrop' AND im.size <> 'original')
ORDER BY i.added_at DESC, i.id DESC
LIMIT @limit_count;

-- name: ListItemTexts :many
SELECT t.* FROM item_texts AS t
JOIN visible_items AS v ON v.item_id = t.item_id AND v.profile_id = @profile_id
WHERE t.item_id IN (sqlc.slice('item_ids'));

-- Delivered image rows (all sizes and formats, without the kept TMDB originals).
-- name: ListItemImages :many
SELECT im.* FROM images AS im
JOIN visible_items AS v ON v.item_id = im.item_id AND v.profile_id = @profile_id
WHERE im.item_id IN (sqlc.slice('item_ids')) AND im.size <> 'original'
ORDER BY im.item_id, im.kind, im.size, im.format;

-- name: ListItemProgress :many
SELECT p.* FROM progress AS p
JOIN visible_items AS v ON v.item_id = p.item_id AND v.profile_id = p.profile_id
WHERE p.profile_id = @profile_id AND p.item_id IN (sqlc.slice('item_ids'));

-- Media files of a visible title with the path of their library root; available files first, then
-- by age, so the first row is the primary file.
-- name: ListMediaFilesForItem :many
SELECT m.id, m.item_id, m.rel_path, m.size, m.mtime, m.container, m.video_codec, m.audio_codec,
       m.width, m.height, m.duration_ms, m.version_label, m.missing_since, m.created_at,
       r.path AS root_path
FROM media_files AS m
JOIN visible_items AS v ON v.item_id = m.item_id AND v.profile_id = @profile_id
JOIN library_roots AS r ON r.id = m.root_id
WHERE m.item_id = @item_id
ORDER BY m.missing_since IS NOT NULL, m.created_at, m.id;

-- One delivered image in the wanted format with JPEG as the fallback: format 'webp' returns the
-- WebP row when it exists, else the JPEG row; format 'jpeg' only the JPEG row.
-- name: GetItemImage :one
SELECT im.* FROM images AS im
JOIN visible_items AS v ON v.item_id = im.item_id AND v.profile_id = @profile_id
WHERE im.item_id = @item_id AND im.kind = @kind AND im.size = @size AND im.size <> 'original'
  AND im.format IN (@format, 'jpeg')
ORDER BY im.format = 'jpeg'
LIMIT 1;
