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

-- name: GetImageByItemKindSize :one
SELECT * FROM images WHERE item_id = @item_id AND kind = @kind AND size = @size;

-- name: ListImagesByItem :many
SELECT * FROM images WHERE item_id = @item_id ORDER BY kind, size;

-- name: UpsertImage :exec
INSERT INTO images (item_id, kind, size, lang, source_url, rel_path, content_hash, media_type, width, height,
                    bytes, created_at)
VALUES (@item_id, @kind, @size, @lang, @source_url, @rel_path, @content_hash, @media_type, @width, @height,
        @bytes, @now)
ON CONFLICT (item_id, kind, size) DO UPDATE
SET lang         = excluded.lang,
    source_url   = excluded.source_url,
    rel_path     = excluded.rel_path,
    content_hash = excluded.content_hash,
    media_type   = excluded.media_type,
    width        = excluded.width,
    height       = excluded.height,
    bytes        = excluded.bytes,
    created_at   = excluded.created_at;
