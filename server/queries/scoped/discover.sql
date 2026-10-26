-- Discover reads of M7 (design 3.1): watchlist, favourites, collections and genres (the spotlight row: spotlight.sql).
-- Every query joins visible_items for the given profile, so an entry whose title became invisible
-- is absent and a collection exists for the profile only through a visible member. Times are Unix
-- milliseconds.

-- Watchlist of the profile (E-5), newest entry first.
-- name: ListWatchlist :many
SELECT i.* FROM watchlist AS w
JOIN items AS i ON i.id = w.item_id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE w.profile_id = @profile_id
ORDER BY w.created_at DESC, i.id
LIMIT @limit_count OFFSET @offset_count;

-- name: CountWatchlist :one
SELECT count(*) FROM watchlist AS w
JOIN visible_items AS v ON v.item_id = w.item_id AND v.profile_id = w.profile_id
WHERE w.profile_id = @profile_id;

-- 1 when the visible title is on the watchlist of the profile, else 0.
-- name: GetWatchlistEntry :one
SELECT count(*) FROM watchlist AS w
JOIN visible_items AS v ON v.item_id = w.item_id AND v.profile_id = w.profile_id
WHERE w.profile_id = @profile_id AND w.item_id = @item_id;

-- Favourite series and podcasts of the profile (E-8), newest first.
-- name: ListFavorites :many
SELECT i.* FROM favorites AS f
JOIN items AS i ON i.id = f.item_id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE f.profile_id = @profile_id
ORDER BY f.created_at DESC, i.id
LIMIT @limit_count;

-- 1 when the visible title is a favourite of the profile, else 0.
-- name: GetFavorite :one
SELECT count(*) FROM favorites AS f
JOIN visible_items AS v ON v.item_id = f.item_id AND v.profile_id = f.profile_id
WHERE f.profile_id = @profile_id AND f.item_id = @item_id;

-- Collections with at least one visible member (E-6), by name: the number of visible members and
-- the cover, the visible member first in collection order (position, then release date).
-- name: ListVisibleCollections :many
SELECT c.id, c.name, c.name_de, count(*) AS member_count,
       CAST((SELECT ci2.item_id FROM collection_items AS ci2
             JOIN items AS i2 ON i2.id = ci2.item_id
             JOIN visible_items AS v2 ON v2.item_id = ci2.item_id AND v2.profile_id = @profile_id
             WHERE ci2.collection_id = c.id
             ORDER BY ci2.position, i2.release_date IS NULL, i2.release_date, i2.id
             LIMIT 1) AS TEXT) AS cover_id
FROM collections AS c
JOIN collection_items AS ci ON ci.collection_id = c.id
JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
GROUP BY c.id
ORDER BY c.name, c.id
LIMIT @limit_count;

-- The collections of the given ids that have a visible member (search results), with count and
-- cover like ListVisibleCollections; the caller keeps its own order.
-- name: ListVisibleCollectionsByID :many
SELECT c.id, c.name, c.name_de, count(*) AS member_count,
       CAST((SELECT ci2.item_id FROM collection_items AS ci2
             JOIN items AS i2 ON i2.id = ci2.item_id
             JOIN visible_items AS v2 ON v2.item_id = ci2.item_id AND v2.profile_id = @profile_id
             WHERE ci2.collection_id = c.id
             ORDER BY ci2.position, i2.release_date IS NULL, i2.release_date, i2.id
             LIMIT 1) AS TEXT) AS cover_id
FROM collections AS c
JOIN collection_items AS ci ON ci.collection_id = c.id
JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
WHERE c.id IN (sqlc.slice('collection_ids'))
GROUP BY c.id;

-- The collections a visible title belongs to (detail page), by name, with count and cover.
-- name: ListCollectionsOfItem :many
SELECT c.id, c.name, c.name_de, count(*) AS member_count,
       CAST((SELECT ci2.item_id FROM collection_items AS ci2
             JOIN items AS i2 ON i2.id = ci2.item_id
             JOIN visible_items AS v2 ON v2.item_id = ci2.item_id AND v2.profile_id = @profile_id
             WHERE ci2.collection_id = c.id
             ORDER BY ci2.position, i2.release_date IS NULL, i2.release_date, i2.id
             LIMIT 1) AS TEXT) AS cover_id
FROM collections AS c
JOIN collection_items AS ci ON ci.collection_id = c.id
JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
WHERE c.id IN (SELECT m.collection_id FROM collection_items AS m
               JOIN visible_items AS vm ON vm.item_id = m.item_id AND vm.profile_id = @profile_id
               WHERE m.item_id = @item_id)
GROUP BY c.id
ORDER BY c.name, c.id;

-- A collection with at least one visible member: names, overviews and the number of visible members.
-- name: GetVisibleCollection :one
SELECT c.id, c.name, c.name_de, c.overview, c.overview_de, count(*) AS member_count
FROM collections AS c
JOIN collection_items AS ci ON ci.collection_id = c.id
JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
WHERE c.id = @collection_id
GROUP BY c.id;

-- One page of the visible members of a collection in collection order (position, then release date).
-- name: ListVisibleCollectionItems :many
SELECT i.* FROM collection_items AS ci
JOIN items AS i ON i.id = ci.item_id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE ci.collection_id = @collection_id
ORDER BY ci.position, i.release_date IS NULL, i.release_date, i.id
LIMIT @limit_count OFFSET @offset_count;

-- Genres of the visible titles (films, series, audiobooks, podcasts) in one language, most
-- frequent first.
-- name: ListVisibleGenres :many
SELECT g.name, count(*) AS item_count
FROM item_genres AS g
JOIN items AS i ON i.id = g.item_id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE g.lang = @lang AND i.kind IN ('movie', 'series', 'audiobook', 'podcast')
GROUP BY g.name
ORDER BY item_count DESC, g.name
LIMIT @limit_count;
