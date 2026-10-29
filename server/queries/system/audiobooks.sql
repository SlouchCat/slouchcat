-- Unfiltered audiobook queries of the scanner and the probe job (package store/systemq, M6 design
-- 3.1). Owner: library package; the image job (metadata) reads the root of a book's cover. Times
-- are Unix milliseconds.

-- name: InsertAudiobook :one
-- A new book found by a scan: the metadata of metadata.json and the folder names (the tags follow
-- with the probe), the age rating of the library (rating_source 'library', or 18 from 'tags' for
-- an explicit book). Audiobooks need no provider match: metadata_state is 'matched' at once.
INSERT INTO items (id, kind, library_id, title, sort_title, year, genres, author, narrator, series_name,
                   series_index, explicit, cover_path, own_rating, rating_source, providers, metadata_state,
                   matched_at, added_at, created_at, updated_at)
VALUES (@id, 'audiobook', @library_id, @title, @sort_title, @year, @genres, @author, @narrator, @series_name,
        @series_index, @explicit, @cover_path, @own_rating, @rating_source, '["local"]', 'matched', @now,
        @now, @now, @now)
RETURNING *;

-- name: UpdateAudiobook :exec
-- The metadata of a book recomputed after its parts were probed (metadata.json > tags > folder
-- names). The caller keeps locked fields and manual ratings by passing the stored values.
UPDATE items
SET title = @title, sort_title = @sort_title, year = @year, genres = @genres, author = @author,
    narrator = @narrator, series_name = @series_name, series_index = @series_index, explicit = @explicit,
    cover_path = @cover_path, own_rating = @own_rating, rating_source = @rating_source,
    duration_ms = @duration_ms, updated_at = @now
WHERE id = @id;

-- name: ListAudiobookParts :many
-- The present media files of a book; the caller orders them (track tag, then file name).
SELECT * FROM media_files
WHERE item_id = @item_id AND missing_since IS NULL
ORDER BY COALESCE(part_index, 0), rel_path, id;

-- name: SetMediaFilePartIndex :exec
-- Position of a part in its book (1-based); NULL for a book of one file.
UPDATE media_files SET part_index = @part_index WHERE id = @id AND part_index IS NOT @part_index;

-- name: DeleteChapters :exec
DELETE FROM chapters WHERE item_id = @item_id;

-- name: InsertChapter :exec
INSERT INTO chapters (item_id, idx, title, start_ms, end_ms, media_file_id, file_start_ms)
VALUES (@item_id, @idx, @title, @start_ms, @end_ms, @media_file_id, @file_start_ms);

-- name: GetItemCoverRoot :one
-- The library root of a book's first present part: cover_path (a folder image) is relative to it.
SELECT r.path FROM media_files AS m
JOIN library_roots AS r ON r.id = m.root_id
WHERE m.item_id = @item_id
ORDER BY m.missing_since IS NOT NULL, COALESCE(m.part_index, 0), m.rel_path, m.id
LIMIT 1;
