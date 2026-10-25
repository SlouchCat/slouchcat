-- The spotlight list (M11, ADR 0011) intersected with the visible library titles by TMDB id, in
-- list order. A title held twice (as film and as show) appears twice; the caller deduplicates.
-- Entries fetched before min_fetched are stale (no successful run for two intervals) and left out.
-- name: ListSpotlightRow :many
SELECT i.* FROM spotlight_entries AS e
JOIN items AS i ON i.tmdb_id = e.tmdb_id
               AND i.kind = CASE e.media WHEN 'movie' THEN 'movie' ELSE 'series' END
               AND i.deleted_at IS NULL
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE e.list_key = @list_key AND e.fetched_at >= @min_fetched
ORDER BY e.position, e.media, i.id
LIMIT @limit_count;
