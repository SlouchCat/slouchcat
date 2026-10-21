-- In-app notifications (M7, E-8, plan 5.5), package notify. Inbox reads join visible_items for the
-- episode, so an entry whose episode the profile may no longer see is neither listed nor counted.

-- name: NotifyEpisodeParent :one
-- The favourite-able parent of a new episode: the series of a series episode (episode -> season ->
-- series) or the podcast of a podcast episode; '' when the episode has none.
SELECT e.kind,
       CAST(COALESCE(CASE e.kind WHEN 'episode' THEN s.parent_id ELSE e.parent_id END, '') AS TEXT) AS parent_id
FROM items AS e
LEFT JOIN items AS s ON s.id = e.parent_id AND e.kind = 'episode'
WHERE e.id = @id AND e.kind IN ('episode', 'podcast_episode') AND e.deleted_at IS NULL;

-- name: ListFavoriteProfiles :many
-- The profiles with a favourite on a series or podcast. Visibility of the new episode is checked
-- by the caller per profile through package visibility.
SELECT p.id, p.account_id, p.name, p.max_rating, p.language
FROM favorites AS f
JOIN profiles AS p ON p.id = f.profile_id
WHERE f.item_id = @item_id
ORDER BY p.id;

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
UPDATE notifications SET read_at = @now WHERE profile_id = @profile_id AND read_at IS NULL;
