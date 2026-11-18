-- In-app notifications (M7, E-8, plan 5.5), package notify. An entry with an item is read through
-- visible_items, so an entry whose title the profile may no longer see is neither listed nor
-- counted. An entry without an item belongs to a wish (concept 25, 06-benachrichtigungen.md
-- section 8) and is read when the profile is a requester of that wish (wish_requesters).

-- name: NotifyHasFollowed :one
-- Whether any profile has a series or podcast on its watchlist (concept 20 P-2: the watchlist took
-- over from the favourite); without one a batch of new episodes needs no further read.
SELECT EXISTS (
    SELECT 1 FROM watchlist AS w JOIN items AS i ON i.id = w.item_id AND i.kind IN ('series', 'podcast')
) AS has_followed;

-- name: NotifyFollowerTargets :many
-- For a batch of new episodes: the followed parent of each (the series of a series episode,
-- episode -> season -> series, or the podcast of a podcast episode) and every profile with that
-- parent on its watchlist. Visibility of the episode is checked by the caller per profile through
-- package visibility.
SELECT e.id AS episode_id, w.item_id AS parent_id, p.id, p.account_id, p.name, p.max_rating, p.language
FROM items AS e
LEFT JOIN items AS s ON s.id = e.parent_id AND e.kind = 'episode'
JOIN watchlist AS w ON w.item_id = CASE e.kind WHEN 'episode' THEN s.parent_id ELSE e.parent_id END
JOIN items AS f ON f.id = w.item_id AND f.kind IN ('series', 'podcast')
JOIN profiles AS p ON p.id = w.profile_id
WHERE e.id IN (sqlc.slice('episode_ids')) AND e.kind IN ('episode', 'podcast_episode') AND e.deleted_at IS NULL
ORDER BY e.id, p.id;

-- name: NotifyInsert :execresult
-- One entry per profile and episode; a second event for the same episode inserts nothing (the
-- dedupe key of an episode is 'episode:<item_id>', migration 00070).
INSERT OR IGNORE INTO notifications (profile_id, kind, item_id, parent_id, dedupe_key, created_at)
VALUES (@profile_id, @kind, CAST(@item_id AS TEXT), @parent_id, 'episode:' || CAST(@item_id AS TEXT), @now);

-- name: NotifyList :many
-- One page of the profile's inbox, newest first: entries whose title the profile sees and entries
-- without a title of a wish the profile requested or joined.
SELECT n.id, n.kind, n.item_id, n.parent_id, n.wish_id, n.created_at, n.read_at
FROM notifications AS n
WHERE n.profile_id = @profile_id
  AND (EXISTS (SELECT 1 FROM visible_items AS v WHERE v.profile_id = n.profile_id AND v.item_id = n.item_id)
       OR (n.item_id IS NULL
           AND EXISTS (SELECT 1 FROM wish_requesters AS r WHERE r.wish_id = n.wish_id AND r.profile_id = n.profile_id)))
ORDER BY n.created_at DESC, n.id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: NotifyCount :one
-- Total and unread entries of the profile's inbox that NotifyList reads.
SELECT count(*) AS total, CAST(COALESCE(SUM(n.read_at IS NULL), 0) AS INTEGER) AS unread
FROM notifications AS n
WHERE n.profile_id = @profile_id
  AND (EXISTS (SELECT 1 FROM visible_items AS v WHERE v.profile_id = n.profile_id AND v.item_id = n.item_id)
       OR (n.item_id IS NULL
           AND EXISTS (SELECT 1 FROM wish_requesters AS r WHERE r.wish_id = n.wish_id AND r.profile_id = n.profile_id)));

-- name: NotifyMarkRead :execrows
-- Marks one entry of the profile read that NotifyList reads (an entry read before keeps its time);
-- 0 rows for an unknown entry, another profile's entry or one the profile no longer sees.
UPDATE notifications SET read_at = COALESCE(read_at, @now)
WHERE notifications.id = @id AND notifications.profile_id = @profile_id
  AND (EXISTS (SELECT 1 FROM visible_items AS v WHERE v.profile_id = notifications.profile_id
                                                  AND v.item_id = notifications.item_id)
       OR (notifications.item_id IS NULL
           AND EXISTS (SELECT 1 FROM wish_requesters AS r WHERE r.wish_id = notifications.wish_id
                                                            AND r.profile_id = notifications.profile_id)));

-- name: NotifyMarkAllRead :exec
-- Marks every unread entry of the profile read that NotifyList reads; entries of hidden titles keep
-- their state, so they come back unread when the profile may see them again.
UPDATE notifications SET read_at = @now
WHERE notifications.profile_id = @profile_id AND notifications.read_at IS NULL
  AND (EXISTS (SELECT 1 FROM visible_items AS v WHERE v.profile_id = notifications.profile_id
                                                  AND v.item_id = notifications.item_id)
       OR (notifications.item_id IS NULL
           AND EXISTS (SELECT 1 FROM wish_requesters AS r WHERE r.wish_id = notifications.wish_id
                                                            AND r.profile_id = notifications.profile_id)));

-- name: NotifyDeleteOldRead :execrows
-- Read entries older than before go (offline cleanup, at most 500 per batch); unread ones stay.
DELETE FROM notifications
WHERE id IN (SELECT n.id FROM notifications AS n WHERE n.read_at IS NOT NULL AND n.created_at < @before LIMIT 500);
