-- Unfiltered title queries for the scanner (package store/systemq). Owner: library package.
-- Times are Unix milliseconds.

-- name: InsertItem :one
INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, original_title, year, tmdb_id, imdb_id,
                   own_rating, rating_source, added_at, created_at, updated_at)
VALUES (@id, @kind, @parent_id, @library_id, @title, @sort_title, @original_title, @year, @tmdb_id, @imdb_id,
        @own_rating, @rating_source, @now, @now, @now)
RETURNING *;

-- name: GetItemSystem :one
SELECT * FROM items WHERE id = @id;

-- name: ListItemsByLibrary :many
SELECT * FROM items WHERE library_id = @library_id AND deleted_at IS NULL ORDER BY id;

-- name: CountLibraryItems :one
SELECT count(*) FROM items WHERE library_id = @library_id AND deleted_at IS NULL;

-- name: InsertMediaFile :one
INSERT INTO media_files (id, item_id, root_id, rel_path, dev, inode, size, mtime, partial_hash, version_label,
                         container, created_at, updated_at)
VALUES (@id, @item_id, @root_id, @rel_path, @dev, @inode, @size, @mtime, @partial_hash, @version_label,
        @container, @now, @now)
RETURNING *;

-- name: ListMediaFilesByRoot :many
SELECT * FROM media_files WHERE root_id = @root_id ORDER BY rel_path;

-- name: CountLibraryMediaFiles :one
SELECT count(*) FROM media_files AS m
JOIN library_roots AS r ON r.id = m.root_id
WHERE r.library_id = @library_id AND m.missing_since IS NULL;

-- name: SetMediaFilePath :exec
UPDATE media_files SET rel_path = @rel_path, mtime = @mtime, missing_since = NULL, updated_at = @now WHERE id = @id;

-- name: SetMediaFileMissing :exec
UPDATE media_files SET missing_since = COALESCE(missing_since, @now), updated_at = @now WHERE id = @id;

-- name: SoftDeleteItemsMissingSince :execrows
UPDATE items SET deleted_at = @now, updated_at = @now
WHERE deleted_at IS NULL
  AND kind IN ('movie', 'episode', 'extra', 'audiobook')
  AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since IS NULL)
  AND EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since < @before);

-- name: UpdateMediaFileLocation :exec
-- Same file at a new path (rename, move), changed in place, or back after missing_since.
UPDATE media_files
SET rel_path = @rel_path, dev = @dev, inode = @inode, size = @size, mtime = @mtime,
    partial_hash = @partial_hash, version_label = @version_label, missing_since = NULL, updated_at = @now
WHERE id = @id;

-- name: RestoreItem :exec
-- A file of a soft-deleted item reappeared: the item comes back with its user data.
UPDATE items SET deleted_at = NULL, updated_at = @now WHERE id = @id AND deleted_at IS NOT NULL;

-- name: GetMediaFileSystem :one
SELECT * FROM media_files WHERE id = @id;

-- name: UpdateMediaFileProbe :exec
UPDATE media_files
SET container = @container, video_codec = @video_codec, audio_codec = @audio_codec, width = @width,
    height = @height, duration_ms = @duration_ms, probe = @probe, probed_at = @now, updated_at = @now
WHERE id = @id;

-- name: UpdateMediaFileKeyframes :exec
UPDATE media_files SET keyframes = @keyframes, keyframes_at = @now, updated_at = @now WHERE id = @id;

-- name: DeleteMediaFileTracks :exec
DELETE FROM tracks WHERE media_file_id = @media_file_id;

-- name: InsertTrack :exec
INSERT INTO tracks (media_file_id, stream_index, kind, codec, language, title, source, rel_path, is_default, is_forced)
VALUES (@media_file_id, @stream_index, @kind, @codec, @language, @title, @source, @rel_path, @is_default, @is_forced);

-- name: ListMediaFileTracks :many
SELECT * FROM tracks WHERE media_file_id = @media_file_id ORDER BY source, stream_index;
