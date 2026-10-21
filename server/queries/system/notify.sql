-- In-app notifications (M7, E-8, plan 5.5), package notify. Inbox reads join visible_items for the
-- episode, so an entry whose episode the profile may no longer see is neither listed nor counted.

-- name: NotifyHasFavorites :one
-- Whether any profile has a favourite; without one a batch of new episodes needs no further read.
SELECT EXISTS (SELECT 1 FROM favorites) AS has_favorites;

-- name: NotifyFavoriteTargets :many
-- For a batch of new episodes: the favourite-able parent of each (the series of a series episode,
-- episode -> season -> series, or the podcast of a podcast episode) and every profile with a
-- favourite on that parent. Visibility of the episode is checked by the caller per profile through
-- package visibility.
SELECT e.id AS episode_id, f.item_id AS parent_id, p.id, p.account_id, p.name, p.max_rating, p.language
FROM items AS e
LEFT JOIN items AS s ON s.id = e.parent_id AND e.kind = 'episode'
JOIN favorites AS f ON f.item_id = CASE e.kind WHEN 'episode' THEN s.parent_id ELSE e.parent_id END
JOIN profiles AS p ON p.id = f.profile_id
WHERE e.id IN (sqlc.slice('episode_ids')) AND e.kind IN ('episode', 'podcast_episode') AND e.deleted_at IS NULL
ORDER BY e.id, p.id;

-- name: NotifyInsert :execresult
-- One entry per profile and episode; a second event for the same episode inserts nothing.
INSERT OR IGNORE INTO notifications (profile_id, kind, item_id, parent_id, created_at)
VALUES (@profile_id, @kind, @item_id, @parent_id, @now);

-- name: NotifyList :many
-- One page of the profile's inbox, newest first, only entries whose episode the profile sees.
SELECT n.id, n.kind, n.item_id, n.parent_id, n.created_at, n.read_at
FROM notifications AS n
JOIN visible_items AS v ON v.profile_id = n.profile_id AND v.item_id = n.item_id
WHERE n.profile_id = @profile_id
ORDER BY n.created_at DESC, n.id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: NotifyCount :one
-- Total and unread entries of the profile's inbox whose episode the profile sees.
SELECT count(*) AS total, CAST(COALESCE(SUM(n.read_at IS NULL), 0) AS INTEGER) AS unread
FROM notifications AS n
JOIN visible_items AS v ON v.profile_id = n.profile_id AND v.item_id = n.item_id
WHERE n.profile_id = @profile_id;

-- name: NotifyMarkRead :execrows
-- Marks one visible entry of the profile read (an entry read before keeps its time); 0 rows for an
-- unknown entry, another profile's entry or one whose episode the profile no longer sees.
UPDATE notifications SET read_at = COALESCE(read_at, @now)
WHERE notifications.id = @id AND notifications.profile_id = @profile_id
  AND EXISTS (SELECT 1 FROM visible_items AS v WHERE v.profile_id = notifications.profile_id
                                                 AND v.item_id = notifications.item_id);

-- name: NotifyMarkAllRead :exec
-- Marks every unread entry of the profile read whose episode the profile sees; entries of hidden
-- episodes keep their state, so they come back unread when the profile may see them again.
UPDATE notifications SET read_at = @now
WHERE notifications.profile_id = @profile_id AND notifications.read_at IS NULL
  AND EXISTS (SELECT 1 FROM visible_items AS v WHERE v.profile_id = notifications.profile_id
                                                 AND v.item_id = notifications.item_id);

-- name: NotifyDeleteOldRead :execrows
-- Read entries older than before go (offline cleanup, at most 500 per batch); unread ones stay.
DELETE FROM notifications
WHERE id IN (SELECT n.id FROM notifications AS n WHERE n.read_at IS NOT NULL AND n.created_at < @before LIMIT 500);
