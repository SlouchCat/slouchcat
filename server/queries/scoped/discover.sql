-- Discover reads of M7 (design 3.1): watchlist, collections and genres (the spotlight row: spotlight.sql).
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

-- The titles of a list that are on the watchlist of the profile (concept 17 D-9, every ItemRef
-- carries the mark since contract 1.20.0): one lookup per id on the primary key (profile_id,
-- item_id); an invisible title is never marked.
-- name: ListWatchlistMarks :many
SELECT w.item_id FROM watchlist AS w
JOIN visible_items AS v ON v.item_id = w.item_id AND v.profile_id = w.profile_id
WHERE w.profile_id = @profile_id AND w.item_id IN (sqlc.slice('item_ids'));

-- Collections with at least one visible member (E-6), by name: the number of visible members and
-- the cover, the visible member first in collection order (position, then release date). Since
-- 1.25.0 without the audiobook collections, whose visible members are all audiobooks (concept 19
-- D4.1): they stand in the row "Reihen und Sammlungen" of their library (ListVisibleAudioCollections).
-- Since 1.33.0 (concept 21 P-10) every collection list also carries the facts of its visible
-- members (the same columns in ListVisibleCollections, ListVisibleCollectionsByID and
-- ListCollectionsOfItem, keep them equal): the first and last year (0 without one), the summed
-- runtime (a series or podcast counts 0), seen_count by the rule of the check mark (a single title
-- by progress.watched_at, a series or podcast when it has visible episodes and every one is
-- watched, the episodes of ListVisibleEpisodeCounts) and started (1 when a member, or an episode
-- of one, is watched or has a running position).
-- name: ListVisibleCollections :many
SELECT c.id, c.name, c.name_de, count(*) AS member_count,
       CAST((SELECT ci2.item_id FROM collection_items AS ci2
             JOIN items AS i2 ON i2.id = ci2.item_id
             JOIN visible_items AS v2 ON v2.item_id = ci2.item_id AND v2.profile_id = @profile_id
             WHERE ci2.collection_id = c.id
             ORDER BY ci2.position, i2.release_date IS NULL, i2.release_date, i2.id
             LIMIT 1) AS TEXT) AS cover_id,
       CAST(COALESCE(min(i.year), 0) AS INTEGER) AS first_year,
       CAST(COALESCE(max(i.year), 0) AS INTEGER) AS last_year,
       CAST(COALESCE(sum(CASE WHEN i.kind IN ('series', 'podcast') THEN 0 ELSE i.runtime_minutes END), 0) AS INTEGER) AS runtime_minutes,
       CAST(COALESCE(sum(CASE WHEN i.kind IN ('series', 'podcast') THEN
                EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id)
                AND NOT EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id
                          AND NOT EXISTS (SELECT 1 FROM progress AS pe WHERE pe.profile_id = @profile_id AND pe.item_id = e.id AND pe.watched_at IS NOT NULL))
            ELSE EXISTS (SELECT 1 FROM progress AS p WHERE p.profile_id = @profile_id AND p.item_id = i.id AND p.watched_at IS NOT NULL)
            END), 0) AS INTEGER) AS seen_count,
       CAST(COALESCE(max(CASE WHEN i.kind IN ('series', 'podcast') THEN
                EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        CROSS JOIN progress AS pe ON pe.profile_id = @profile_id AND pe.item_id = e.id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id
                          AND (pe.watched_at IS NOT NULL OR pe.position_ms > 0))
            ELSE EXISTS (SELECT 1 FROM progress AS p WHERE p.profile_id = @profile_id AND p.item_id = i.id
                         AND (p.watched_at IS NOT NULL OR p.position_ms > 0))
            END), 0) AS INTEGER) AS started
FROM collections AS c
JOIN collection_items AS ci ON ci.collection_id = c.id
JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
JOIN items AS i ON i.id = ci.item_id
GROUP BY c.id
HAVING sum(i.kind <> 'audiobook') > 0
ORDER BY c.name, c.id
LIMIT @limit_count;

-- The start page row "Sammlungen" (concept 21 O-16): the collections of ListVisibleCollections with
-- at least two visible members, the started ones first, each group by name; the same columns.
-- name: ListHomeCollections :many
SELECT c.id, c.name, c.name_de, count(*) AS member_count,
       CAST((SELECT ci2.item_id FROM collection_items AS ci2
             JOIN items AS i2 ON i2.id = ci2.item_id
             JOIN visible_items AS v2 ON v2.item_id = ci2.item_id AND v2.profile_id = @profile_id
             WHERE ci2.collection_id = c.id
             ORDER BY ci2.position, i2.release_date IS NULL, i2.release_date, i2.id
             LIMIT 1) AS TEXT) AS cover_id,
       CAST(COALESCE(min(i.year), 0) AS INTEGER) AS first_year,
       CAST(COALESCE(max(i.year), 0) AS INTEGER) AS last_year,
       CAST(COALESCE(sum(CASE WHEN i.kind IN ('series', 'podcast') THEN 0 ELSE i.runtime_minutes END), 0) AS INTEGER) AS runtime_minutes,
       CAST(COALESCE(sum(CASE WHEN i.kind IN ('series', 'podcast') THEN
                EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id)
                AND NOT EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id
                          AND NOT EXISTS (SELECT 1 FROM progress AS pe WHERE pe.profile_id = @profile_id AND pe.item_id = e.id AND pe.watched_at IS NOT NULL))
            ELSE EXISTS (SELECT 1 FROM progress AS p WHERE p.profile_id = @profile_id AND p.item_id = i.id AND p.watched_at IS NOT NULL)
            END), 0) AS INTEGER) AS seen_count,
       CAST(COALESCE(max(CASE WHEN i.kind IN ('series', 'podcast') THEN
                EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        CROSS JOIN progress AS pe ON pe.profile_id = @profile_id AND pe.item_id = e.id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id
                          AND (pe.watched_at IS NOT NULL OR pe.position_ms > 0))
            ELSE EXISTS (SELECT 1 FROM progress AS p WHERE p.profile_id = @profile_id AND p.item_id = i.id
                         AND (p.watched_at IS NOT NULL OR p.position_ms > 0))
            END), 0) AS INTEGER) AS started
FROM collections AS c
JOIN collection_items AS ci ON ci.collection_id = c.id
JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
JOIN items AS i ON i.id = ci.item_id
GROUP BY c.id
HAVING sum(i.kind <> 'audiobook') > 0 AND count(*) >= 2
ORDER BY started DESC, c.name, c.id
LIMIT @limit_count;

-- The number of collections ListVisibleCollections lists without its limit: the count of the entry
-- "Sammlungen" of the side navigation (concept 21 P-9).
-- name: CountVisibleCollections :one
SELECT count(*) FROM (
    SELECT c.id
    FROM collections AS c
    JOIN collection_items AS ci ON ci.collection_id = c.id
    JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
    JOIN items AS i ON i.id = ci.item_id
    GROUP BY c.id
    HAVING sum(i.kind <> 'audiobook') > 0
);

-- The collections of the given ids that have a visible member (search results), with count and
-- cover like ListVisibleCollections; the caller keeps its own order.
-- name: ListVisibleCollectionsByID :many
SELECT c.id, c.name, c.name_de, count(*) AS member_count,
       CAST((SELECT ci2.item_id FROM collection_items AS ci2
             JOIN items AS i2 ON i2.id = ci2.item_id
             JOIN visible_items AS v2 ON v2.item_id = ci2.item_id AND v2.profile_id = @profile_id
             WHERE ci2.collection_id = c.id
             ORDER BY ci2.position, i2.release_date IS NULL, i2.release_date, i2.id
             LIMIT 1) AS TEXT) AS cover_id,
       CAST(COALESCE(min(i.year), 0) AS INTEGER) AS first_year,
       CAST(COALESCE(max(i.year), 0) AS INTEGER) AS last_year,
       CAST(COALESCE(sum(CASE WHEN i.kind IN ('series', 'podcast') THEN 0 ELSE i.runtime_minutes END), 0) AS INTEGER) AS runtime_minutes,
       CAST(COALESCE(sum(CASE WHEN i.kind IN ('series', 'podcast') THEN
                EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id)
                AND NOT EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id
                          AND NOT EXISTS (SELECT 1 FROM progress AS pe WHERE pe.profile_id = @profile_id AND pe.item_id = e.id AND pe.watched_at IS NOT NULL))
            ELSE EXISTS (SELECT 1 FROM progress AS p WHERE p.profile_id = @profile_id AND p.item_id = i.id AND p.watched_at IS NOT NULL)
            END), 0) AS INTEGER) AS seen_count,
       CAST(COALESCE(max(CASE WHEN i.kind IN ('series', 'podcast') THEN
                EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        CROSS JOIN progress AS pe ON pe.profile_id = @profile_id AND pe.item_id = e.id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id
                          AND (pe.watched_at IS NOT NULL OR pe.position_ms > 0))
            ELSE EXISTS (SELECT 1 FROM progress AS p WHERE p.profile_id = @profile_id AND p.item_id = i.id
                         AND (p.watched_at IS NOT NULL OR p.position_ms > 0))
            END), 0) AS INTEGER) AS started
FROM collections AS c
JOIN collection_items AS ci ON ci.collection_id = c.id
JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
JOIN items AS i ON i.id = ci.item_id
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
             LIMIT 1) AS TEXT) AS cover_id,
       CAST(COALESCE(min(i.year), 0) AS INTEGER) AS first_year,
       CAST(COALESCE(max(i.year), 0) AS INTEGER) AS last_year,
       CAST(COALESCE(sum(CASE WHEN i.kind IN ('series', 'podcast') THEN 0 ELSE i.runtime_minutes END), 0) AS INTEGER) AS runtime_minutes,
       CAST(COALESCE(sum(CASE WHEN i.kind IN ('series', 'podcast') THEN
                EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id)
                AND NOT EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id
                          AND NOT EXISTS (SELECT 1 FROM progress AS pe WHERE pe.profile_id = @profile_id AND pe.item_id = e.id AND pe.watched_at IS NOT NULL))
            ELSE EXISTS (SELECT 1 FROM progress AS p WHERE p.profile_id = @profile_id AND p.item_id = i.id AND p.watched_at IS NOT NULL)
            END), 0) AS INTEGER) AS seen_count,
       CAST(COALESCE(max(CASE WHEN i.kind IN ('series', 'podcast') THEN
                EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        CROSS JOIN progress AS pe ON pe.profile_id = @profile_id AND pe.item_id = e.id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id
                          AND (pe.watched_at IS NOT NULL OR pe.position_ms > 0))
            ELSE EXISTS (SELECT 1 FROM progress AS p WHERE p.profile_id = @profile_id AND p.item_id = i.id
                         AND (p.watched_at IS NOT NULL OR p.position_ms > 0))
            END), 0) AS INTEGER) AS started
FROM collections AS c
JOIN collection_items AS ci ON ci.collection_id = c.id
JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
JOIN items AS i ON i.id = ci.item_id
WHERE c.id IN (SELECT m.collection_id FROM collection_items AS m
               JOIN visible_items AS vm ON vm.item_id = m.item_id AND vm.profile_id = @profile_id
               WHERE m.item_id = @item_id)
GROUP BY c.id
ORDER BY c.name, c.id;

-- A collection with at least one visible member: names, overviews, the description of TMDB with
-- its language and the main language it was asked in (concept 21 O-18), the number of visible
-- members and their facts by the rule of the collection lists.
-- name: GetVisibleCollection :one
SELECT c.id, c.name, c.name_de, c.overview, c.overview_de, c.description, c.description_lang, c.details_lang,
       count(*) AS member_count,
       CAST(COALESCE(min(i.year), 0) AS INTEGER) AS first_year,
       CAST(COALESCE(max(i.year), 0) AS INTEGER) AS last_year,
       CAST(COALESCE(sum(CASE WHEN i.kind IN ('series', 'podcast') THEN 0 ELSE i.runtime_minutes END), 0) AS INTEGER) AS runtime_minutes,
       CAST(COALESCE(sum(CASE WHEN i.kind IN ('series', 'podcast') THEN
                EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id)
                AND NOT EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id
                          AND NOT EXISTS (SELECT 1 FROM progress AS pe WHERE pe.profile_id = @profile_id AND pe.item_id = e.id AND pe.watched_at IS NOT NULL))
            ELSE EXISTS (SELECT 1 FROM progress AS p WHERE p.profile_id = @profile_id AND p.item_id = i.id AND p.watched_at IS NOT NULL)
            END), 0) AS INTEGER) AS seen_count,
       CAST(COALESCE(max(CASE WHEN i.kind IN ('series', 'podcast') THEN
                EXISTS (SELECT 1 FROM items AS se
                        CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                        CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                        CROSS JOIN progress AS pe ON pe.profile_id = @profile_id AND pe.item_id = e.id
                        WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season')) AND +e.library_id = i.library_id
                          AND (pe.watched_at IS NOT NULL OR pe.position_ms > 0))
            ELSE EXISTS (SELECT 1 FROM progress AS p WHERE p.profile_id = @profile_id AND p.item_id = i.id
                         AND (p.watched_at IS NOT NULL OR p.position_ms > 0))
            END), 0) AS INTEGER) AS started
FROM collections AS c
JOIN collection_items AS ci ON ci.collection_id = c.id
JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
JOIN items AS i ON i.id = ci.item_id
WHERE c.id = @collection_id
GROUP BY c.id;

-- The part of a collection the big button plays (concept 21 P-10): the first visible member in
-- collection order that is not finished (a single title without progress.completed, a series or
-- podcast that is not watched by the rule of seen_count), with its place counted from 1. No row when
-- every member is finished (the page then plays the first part again).
-- name: GetVisibleCollectionNext :one
WITH members AS (
    SELECT i.id, i.kind, i.library_id,
           row_number() OVER (ORDER BY ci.position, i.release_date IS NULL, i.release_date, i.id) AS part
    FROM collection_items AS ci
    JOIN items AS i ON i.id = ci.item_id
    JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
    WHERE ci.collection_id = @collection_id
)
SELECT m.id, CAST(m.part AS INTEGER) AS part
FROM members AS m
WHERE CASE WHEN m.kind IN ('series', 'podcast') THEN
          NOT EXISTS (SELECT 1 FROM items AS se
                      CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                      CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                      WHERE (se.id = m.id OR (se.parent_id = m.id AND se.kind = 'season')) AND +e.library_id = m.library_id)
          OR EXISTS (SELECT 1 FROM items AS se
                     CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
                     CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
                     WHERE (se.id = m.id OR (se.parent_id = m.id AND se.kind = 'season')) AND +e.library_id = m.library_id
                       AND NOT EXISTS (SELECT 1 FROM progress AS pe WHERE pe.profile_id = @profile_id AND pe.item_id = e.id AND pe.watched_at IS NOT NULL))
      ELSE NOT EXISTS (SELECT 1 FROM progress AS p WHERE p.profile_id = @profile_id AND p.item_id = m.id AND p.completed = 1)
      END
ORDER BY m.part
LIMIT 1;

-- The visible members of a collection in collection order, ids only: the titles setCollectionWatched
-- changes (concept 21 O-19).
-- name: ListVisibleCollectionMemberIDs :many
SELECT i.id FROM collection_items AS ci
JOIN items AS i ON i.id = ci.item_id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE ci.collection_id = @collection_id
ORDER BY ci.position, i.release_date IS NULL, i.release_date, i.id;

-- One page of the visible members of a collection in collection order (position, then release date).
-- name: ListVisibleCollectionItems :many
SELECT i.* FROM collection_items AS ci
JOIN items AS i ON i.id = ci.item_id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE ci.collection_id = @collection_id
ORDER BY ci.position, i.release_date IS NULL, i.release_date, i.id
LIMIT @limit_count OFFSET @offset_count;

-- Genres of the visible titles (films, series, audiobooks, podcasts) in one language, most
-- frequent first. The titles come first, from the partial index items_top_added (migration
-- 00037), so visible_items is evaluated once per title; CROSS JOIN keeps the genres after them
-- (from item_genres first it was evaluated once per genre of a title).
-- name: ListVisibleGenres :many
SELECT g.name, count(*) AS item_count
FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
CROSS JOIN item_genres AS g ON g.item_id = i.id AND g.lang = @lang
WHERE i.kind IN ('movie', 'series', 'audiobook', 'podcast')
GROUP BY g.name
ORDER BY item_count DESC, g.name
LIMIT @limit_count;
