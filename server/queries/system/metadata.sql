-- Unfiltered title queries for metadata enrichment and image storage (package store/systemq).
-- Owner: metadata package. Times are Unix milliseconds.

-- name: UpdateItemFromTMDB :exec
-- Writes the matched TMDB data. The caller has already kept the values of locked fields.
UPDATE items
SET original_title  = @original_title,
    year            = @year,
    release_date    = @release_date,
    runtime_minutes = @runtime_minutes,
    genres          = @genres,
    tmdb_id         = @tmdb_id,
    imdb_id         = @imdb_id,
    own_rating      = @own_rating,
    rating_source   = @rating_source,
    metadata_state  = 'matched',
    metadata_error  = NULL,
    matched_at      = @now,
    updated_at      = @now
WHERE id = @id AND deleted_at IS NULL;

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
-- (QueueIncompleteImages). Titles without any image row are not listed: TMDB has none for them,
-- or their first image.fetch is still pending.
SELECT i.id AS item_id, i.locked_fields, im.kind, im.size, im.format, im.source_url, im.rel_path
FROM items AS i
JOIN images AS im ON im.item_id = i.id
WHERE i.library_id = @library_id
  AND i.deleted_at IS NULL
  AND i.metadata_state = 'matched'
  AND i.tmdb_id IS NOT NULL
ORDER BY i.id, im.kind, im.size, im.format;
