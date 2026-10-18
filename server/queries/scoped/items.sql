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
