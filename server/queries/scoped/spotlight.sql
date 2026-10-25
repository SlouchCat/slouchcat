-- The spotlight list (M11, ADR 0011) intersected with the visible library titles by TMDB id, in
-- list order, with the curation of the main user: hidden entries are filtered here (filtering in
-- Go would eat into a window), pinned_rank and note travel along. There is no LIMIT on purpose
-- (plan 5.3): a pinned entry far down the list must reach the caller, which deduplicates, places
-- the pinned entries by rank and cuts to item_count. The cache holds at most 100 rows per medium.
-- A title held twice (as film and as show) appears twice. Entries fetched before min_fetched are
-- stale (no successful run for two intervals) and left out.
-- name: ListSpotlightRow :many
SELECT sqlc.embed(i), o.pinned_rank, o.note FROM spotlight_entries AS e
JOIN items AS i ON i.tmdb_id = e.tmdb_id
               AND i.kind = CASE e.media WHEN 'movie' THEN 'movie' ELSE 'series' END
               AND i.deleted_at IS NULL
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
LEFT JOIN spotlight_overrides AS o ON o.list_key = e.list_key AND o.media = e.media AND o.tmdb_id = e.tmdb_id
WHERE e.list_key = @list_key AND e.fetched_at >= @min_fetched AND coalesce(o.hidden, 0) = 0
ORDER BY e.position, e.media, i.id;
