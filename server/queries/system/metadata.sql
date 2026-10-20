-- Unfiltered title queries for metadata enrichment and image storage (package store/systemq).
-- Owner: metadata package. Times are Unix milliseconds.

-- name: UpdateItemMatch :exec
-- Writes the matched provider data of a movie or series. The caller has already kept the values
-- of locked fields.
UPDATE items
SET original_title     = @original_title,
    year               = @year,
    release_date       = @release_date,
    runtime_minutes    = @runtime_minutes,
    genres             = @genres,
    tmdb_id            = @tmdb_id,
    tvdb_id            = @tvdb_id,
    imdb_id            = @imdb_id,
    own_rating         = @own_rating,
    rating_source      = @rating_source,
    tmdb_poster_path   = @tmdb_poster_path,
    tmdb_backdrop_path = @tmdb_backdrop_path,
    metadata_state     = 'matched',
    metadata_error     = NULL,
    matched_at         = @now,
    updated_at         = @now
WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateChildMatch :exec
-- Writes the provider data of a season or episode (series job or episode match). The caller has
-- already kept the values of locked fields.
UPDATE items
SET release_date       = @release_date,
    runtime_minutes    = @runtime_minutes,
    tmdb_poster_path   = @tmdb_poster_path,
    tmdb_backdrop_path = @tmdb_backdrop_path,
    metadata_state     = 'matched',
    metadata_error     = NULL,
    matched_at         = @now,
    updated_at         = @now
WHERE id = @id AND deleted_at IS NULL;

-- name: ListChildItems :many
-- The children of a series (seasons, episodes without season folder, extras) or of a season.
SELECT * FROM items
WHERE parent_id = @parent_id AND deleted_at IS NULL
ORDER BY season_number, episode_number, id;

-- name: ListEpisodesToMatch :many
-- Episodes of a series (below a season or directly below the series) that still wait for their
-- metadata: pending, or skipped because the series was not matched yet.
SELECT e.id FROM items AS e
WHERE e.kind = 'episode' AND e.deleted_at IS NULL
  AND (e.metadata_state = 'pending' OR (e.metadata_state = 'skipped' AND e.metadata_error = 'series_unmatched'))
  AND (e.parent_id = @series_id
       OR e.parent_id IN (SELECT s.id FROM items AS s WHERE s.parent_id = @series_id AND s.kind = 'season'))
ORDER BY e.season_number, e.episode_number, e.id;

-- name: ListPendingMatchItems :many
-- Titles of a library whose metadata is pending without an active match job (QueuePendingMetadata):
-- movies and series, and episodes whose series is matched (the series job fills the others).
SELECT i.id, i.kind FROM items AS i
LEFT JOIN items AS p ON p.id = i.parent_id
LEFT JOIN items AS g ON g.id = p.parent_id
WHERE i.library_id = @library_id AND i.deleted_at IS NULL AND i.metadata_state = 'pending'
  AND (i.kind IN ('movie', 'series')
       OR (i.kind = 'episode' AND ((p.kind = 'series' AND p.metadata_state = 'matched')
                                   OR (g.kind = 'series' AND g.metadata_state = 'matched'))))
  AND NOT EXISTS (SELECT 1 FROM jobs AS j
                  WHERE j.idempotency_key = 'match:' || i.id AND j.state IN ('queued', 'running'))
ORDER BY i.id;

-- name: ListUpgradeMatchItems :many
-- Matched TMDB movies and series of a library without image paths and without a finished trailer
-- check: titles matched before M3, which lack the image paths, credits and trailers only a match
-- writes (QueuePendingMetadata queues one match each, at most @limit_count per run). The M3 match
-- stores the paths or queues trailer.find, whose check sets trailer_checked_at, so a title is
-- queued once.
SELECT i.id FROM items AS i
WHERE i.library_id = @library_id AND i.deleted_at IS NULL
  AND i.kind IN ('movie', 'series') AND i.metadata_state = 'matched' AND i.tmdb_id IS NOT NULL
  AND i.tmdb_poster_path IS NULL AND i.tmdb_backdrop_path IS NULL AND i.trailer_checked_at IS NULL
  AND instr(i.locked_fields, '"trailer"') = 0
  AND NOT EXISTS (SELECT 1 FROM jobs AS j
                  WHERE j.idempotency_key IN ('match:' || i.id, 'trailer:' || i.id) AND j.state IN ('queued', 'running'))
ORDER BY i.id
LIMIT @limit_count;

-- name: AssignItemMatch :exec
-- Manual match (B-6): the chosen provider ID, the other provider ID and imdb_id cleared, the IDs
-- locked, metadata and trailers pending until the queued match has run.
UPDATE items
SET tmdb_id        = @tmdb_id,
    tvdb_id        = @tvdb_id,
    imdb_id        = NULL,
    locked_fields  = @locked_fields,
    metadata_state = 'pending',
    metadata_error = NULL,
    trailer_state  = 'pending',
    updated_at     = @now
WHERE id = @id AND deleted_at IS NULL;

-- name: ListMediaFileIDsByItem :many
-- The files of a title (refresh probes them again).
SELECT id FROM media_files WHERE item_id = @item_id ORDER BY id;

-- name: SetItemMetadataState :exec
UPDATE items
SET metadata_state = @metadata_state,
    metadata_error = @metadata_error,
    matched_at     = @matched_at,
    updated_at     = @now
WHERE id = @id;

-- name: ListItemTextsByItem :many
SELECT * FROM item_texts WHERE item_id = @item_id ORDER BY lang;

-- name: UpsertItemText :exec
INSERT INTO item_texts (item_id, lang, title, overview, tagline)
VALUES (@item_id, @lang, @title, @overview, @tagline)
ON CONFLICT (item_id, lang) DO UPDATE
SET title = excluded.title, overview = excluded.overview, tagline = excluded.tagline;

-- name: ListImagesByItem :many
-- Every stored image row of a title, originals included (image.fetch compares them with TMDB).
SELECT * FROM images WHERE item_id = @item_id ORDER BY kind, size, format;

-- name: UpsertImage :exec
INSERT INTO images (item_id, kind, size, format, lang, source_url, rel_path, content_hash, media_type, width,
                    height, bytes, accent, created_at)
VALUES (@item_id, @kind, @size, @format, @lang, @source_url, @rel_path, @content_hash, @media_type, @width,
        @height, @bytes, @accent, @now)
ON CONFLICT (item_id, kind, size, format) DO UPDATE
SET lang         = excluded.lang,
    source_url   = excluded.source_url,
    rel_path     = excluded.rel_path,
    content_hash = excluded.content_hash,
    media_type   = excluded.media_type,
    width        = excluded.width,
    height       = excluded.height,
    bytes        = excluded.bytes,
    accent       = excluded.accent,
    created_at   = excluded.created_at;

-- name: DeleteImage :exec
-- Removes a row the pipeline no longer produces (for example the M1a poster in w500).
DELETE FROM images WHERE id = @id;

-- name: ListMatchedItemImages :many
-- Stored images of the matched titles of a library, for the image check after a scan
-- (QueueIncompleteImages). Titles without any image row are not listed here (ListItemsMissingImages
-- covers those with a stored path).
SELECT i.id AS item_id, i.locked_fields, im.kind, im.size, im.format, im.source_url, im.rel_path, im.accent
FROM items AS i
JOIN images AS im ON im.item_id = i.id
WHERE i.library_id = @library_id
  AND i.deleted_at IS NULL
  AND i.metadata_state = 'matched'
ORDER BY i.id, im.kind, im.size, im.format;

-- name: ListItemsMissingImages :many
-- Matched titles of a library with a stored provider image path but no row of that kind yet
-- (their image.fetch failed or never ran): poster for movies, series and seasons, backdrop (still)
-- for movies, series and episodes.
SELECT i.id, i.kind, i.locked_fields,
       CAST(i.tmdb_poster_path IS NOT NULL AND i.kind IN ('movie', 'series', 'season')
            AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'poster') AS INTEGER) AS poster_missing,
       CAST(i.tmdb_backdrop_path IS NOT NULL AND i.kind IN ('movie', 'series', 'episode')
            AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'backdrop') AS INTEGER) AS backdrop_missing
FROM items AS i
WHERE i.library_id = @library_id
  AND i.deleted_at IS NULL
  AND i.metadata_state = 'matched'
  AND ((i.tmdb_poster_path IS NOT NULL AND i.kind IN ('movie', 'series', 'season')
        AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'poster'))
    OR (i.tmdb_backdrop_path IS NOT NULL AND i.kind IN ('movie', 'series', 'episode')
        AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'backdrop')))
ORDER BY i.id;
