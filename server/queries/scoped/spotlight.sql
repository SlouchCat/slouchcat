-- The spotlight list (M11, ADR 0011) intersected with the visible library titles, in list order,
-- with the curation of the main user: hidden entries are filtered here (filtering in Go would eat
-- into a window), pinned_rank and note travel along. There is no LIMIT on purpose (plan 5.3): a
-- pinned entry far down the list must reach the caller, which deduplicates, places the pinned
-- entries by rank and cuts to item_count. The cache holds at most 100 rows per medium. A title held
-- twice (as film and as show) appears twice. Entries fetched before min_fetched are stale (no
-- successful run for two intervals) and left out.
--
-- Matching (concept 25 N-61, like the library): a film by TMDB id, then IMDb id; a show by TVDB
-- id, then TMDB id, then IMDb id. Each branch uses its index on items (items_tvdb, items_tmdb,
-- items_imdb) and joins visible_items, so the ladder runs over the titles this profile sees only:
-- an invisible title never decides the match. An entry keeps the hits of its best rung only, so a
-- title that shares a weaker id with the entry does not ride along. An entry without any visible
-- hit, matched or not (unmatched: tmdb_id NULL, migration 00071), yields no row.
-- name: ListSpotlightRow :many
WITH entries AS (
    SELECT e.list_key, e.media, e.position, e.tmdb_id, e.tvdb_id, e.imdb_id,
           CASE e.media WHEN 'movie' THEN 'movie' ELSE 'series' END AS kind
    FROM spotlight_entries AS e
    WHERE e.list_key = @list_key AND e.fetched_at >= @min_fetched
), hits AS (
    SELECT e.list_key, e.media, e.position, e.tmdb_id, 1 AS rung, i.id AS item_id
    FROM entries AS e
    JOIN items AS i ON i.tvdb_id = e.tvdb_id AND i.kind = e.kind AND i.deleted_at IS NULL
    JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
    WHERE e.media = 'show'
    UNION ALL
    SELECT e.list_key, e.media, e.position, e.tmdb_id, 2 AS rung, i.id AS item_id
    FROM entries AS e
    JOIN items AS i ON i.tmdb_id = e.tmdb_id AND i.kind = e.kind AND i.deleted_at IS NULL
    JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
    UNION ALL
    SELECT e.list_key, e.media, e.position, e.tmdb_id, 3 AS rung, i.id AS item_id
    FROM entries AS e
    JOIN items AS i ON i.imdb_id = e.imdb_id AND i.kind = e.kind AND i.deleted_at IS NULL
    JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
), best AS (
    SELECT h.list_key, h.media, h.position, h.tmdb_id, h.item_id
    FROM hits AS h
    WHERE h.rung = (SELECT min(b.rung) FROM hits AS b WHERE b.media = h.media AND b.position = h.position)
)
SELECT sqlc.embed(i), o.pinned_rank, o.note FROM best AS h
JOIN items AS i ON i.id = h.item_id
LEFT JOIN spotlight_overrides AS o ON o.list_key = h.list_key AND o.media = h.media AND o.tmdb_id = h.tmdb_id
WHERE coalesce(o.hidden, 0) = 0
ORDER BY h.position, h.media, i.id;
