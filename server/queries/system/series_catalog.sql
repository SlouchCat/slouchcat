-- The season catalog of the series of the library at TMDB (concept 25 step 4.1, data model
-- section 2.10). Package metadata writes it (job series.catalog), package visibility reads it.

-- name: UpsertSeriesCatalog :exec
INSERT INTO series_catalog (item_id, tmdb_id, status, in_production, seasons, next_episode, last_episode, fetched_at)
VALUES (@item_id, @tmdb_id, @status, @in_production, @seasons, @next_episode, @last_episode, @fetched_at)
ON CONFLICT (item_id) DO UPDATE SET
    tmdb_id = excluded.tmdb_id, status = excluded.status, in_production = excluded.in_production,
    seasons = excluded.seasons, next_episode = excluded.next_episode, last_episode = excluded.last_episode,
    fetched_at = excluded.fetched_at;

-- name: TouchSeriesCatalog :exec
-- A series TMDB does not know: a row fetched for the same id keeps its values and only waits for
-- its next due date; a row of another id (or none) becomes an empty row of this id, so the job
-- does not ask again before it is due.
INSERT INTO series_catalog (item_id, tmdb_id, fetched_at)
VALUES (@item_id, @tmdb_id, @fetched_at)
ON CONFLICT (item_id) DO UPDATE SET
    fetched_at = excluded.fetched_at,
    status = CASE WHEN series_catalog.tmdb_id = excluded.tmdb_id THEN series_catalog.status END,
    in_production = CASE WHEN series_catalog.tmdb_id = excluded.tmdb_id THEN series_catalog.in_production ELSE 0 END,
    seasons = CASE WHEN series_catalog.tmdb_id = excluded.tmdb_id THEN series_catalog.seasons ELSE '[]' END,
    next_episode = CASE WHEN series_catalog.tmdb_id = excluded.tmdb_id THEN series_catalog.next_episode END,
    last_episode = CASE WHEN series_catalog.tmdb_id = excluded.tmdb_id THEN series_catalog.last_episode END,
    tmdb_id = excluded.tmdb_id;

-- name: GetSeriesCatalog :one
SELECT * FROM series_catalog WHERE item_id = @item_id;

-- name: GetSeriesCatalogTarget :one
-- The TMDB id of a live series of the library; no row for another kind, a deleted series or one
-- without TMDB id.
SELECT id, CAST(tmdb_id AS INTEGER) AS tmdb_id FROM items
WHERE id = @item_id AND kind = 'series' AND deleted_at IS NULL AND tmdb_id IS NOT NULL AND tmdb_id > 0;

-- name: ListDueSeriesCatalog :many
-- The live series with a TMDB id whose catalog is missing, was fetched for another TMDB id or
-- before cutoff: the missing ones first, then the oldest.
SELECT i.id, CAST(i.tmdb_id AS INTEGER) AS tmdb_id FROM items AS i
LEFT JOIN series_catalog AS c ON c.item_id = i.id
WHERE i.kind = 'series' AND i.deleted_at IS NULL AND i.tmdb_id IS NOT NULL AND i.tmdb_id > 0
  AND (c.item_id IS NULL OR c.tmdb_id <> i.tmdb_id OR c.fetched_at < @cutoff)
ORDER BY coalesce(c.fetched_at, 0), i.id
LIMIT @limit_count;

-- name: ListLibrarySeasonNumbers :many
-- The season numbers (1 or more) of a series with at least one live, available episode in the
-- library, for every profile alike: visibility leaves the seasons a profile cannot play out of
-- its catalog and its request dialog instead of reading them as missing (review of 18 Nov 2026).
-- The season number is the episode's, else its season's, as in ListVisibleSeasonEpisodes.
SELECT DISTINCT CAST(coalesce(e.season_number, se.season_number) AS INTEGER) AS season_number
FROM items AS se
JOIN items AS e ON e.parent_id = se.id AND e.kind = 'episode'
WHERE se.parent_id = @series_id AND se.kind = 'season'
  AND e.deleted_at IS NULL AND e.available = 1
  AND coalesce(e.season_number, se.season_number) >= 1
ORDER BY 1;
