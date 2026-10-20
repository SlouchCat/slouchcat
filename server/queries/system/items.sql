-- Unfiltered title queries for the scanner (package store/systemq). Owner: library package.
-- Times are Unix milliseconds.

-- name: InsertItem :one
INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, original_title, year, release_date,
                   tmdb_id, tvdb_id, imdb_id, season_number, episode_number, episode_end, extra_kind,
                   own_rating, rating_source, added_at, created_at, updated_at)
VALUES (@id, @kind, @parent_id, @library_id, @title, @sort_title, @original_title, @year, @release_date,
        @tmdb_id, @tvdb_id, @imdb_id, @season_number, @episode_number, @episode_end, @extra_kind,
        @own_rating, @rating_source, @now, @now, @now)
RETURNING *;

-- name: GetItemSystem :one
SELECT * FROM items WHERE id = @id;

-- name: ListItemsByLibrary :many
SELECT * FROM items WHERE library_id = @library_id AND deleted_at IS NULL ORDER BY id;

-- name: ListRecentlyDeletedItemsByLibrary :many
-- Soft-deleted titles that a re-downloaded file may bring back by provider id (plan 3.2).
SELECT * FROM items
WHERE library_id = @library_id AND deleted_at IS NOT NULL AND deleted_at >= @since
ORDER BY deleted_at DESC, id;

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
  AND EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id)
  AND NOT EXISTS (SELECT 1 FROM media_files AS m
                  WHERE m.item_id = items.id AND (m.missing_since IS NULL OR m.missing_since >= @before));

-- name: UpdateMediaFileLocation :exec
-- Same file at a new path (rename, move), changed in place, or back after missing_since.
UPDATE media_files
SET rel_path = @rel_path, dev = @dev, inode = @inode, size = @size, mtime = @mtime,
    partial_hash = @partial_hash, version_label = @version_label, missing_since = NULL, updated_at = @now
WHERE id = @id;

-- name: RestoreItem :exec
-- A file of a soft-deleted item reappeared: the item comes back with its user data, and with it
-- its soft-deleted parent and grandparent (the season and series of an episode or a season
-- extra), so the tree stays whole. An ancestor that is not a container (the film of an extra)
-- comes back only with a present file of its own; otherwise the next sweep would delete it again.
UPDATE items SET deleted_at = NULL, updated_at = @now
WHERE items.deleted_at IS NOT NULL
  AND (items.id = @id
       OR (items.id IN ((SELECT p.parent_id FROM items AS p WHERE p.id = @id),
                        (SELECT g.parent_id FROM items AS g WHERE g.id = (SELECT p.parent_id FROM items AS p WHERE p.id = @id)))
           AND (items.kind IN ('series', 'season', 'podcast')
                OR EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since IS NULL))));

-- name: GetMediaFileSystem :one
SELECT * FROM media_files WHERE id = @id;

-- name: UpdateMediaFileProbe :exec
UPDATE media_files
SET container = @container, video_codec = @video_codec, audio_codec = @audio_codec, width = @width,
    height = @height, duration_ms = @duration_ms, probe = @probe, probe_error = NULL, probed_at = @now,
    updated_at = @now
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

-- Sidecar subtitle sync of the scans (I-5, M4): sidecar rows are compared with the files on disk
-- without probing again.

-- name: ListSidecarTracksByRoot :many
SELECT t.id, t.media_file_id, t.rel_path FROM tracks AS t
JOIN media_files AS m ON m.id = t.media_file_id
WHERE m.root_id = @root_id AND t.source = 'sidecar'
ORDER BY t.media_file_id, t.rel_path;

-- name: ListMediaFileSidecarTracks :many
SELECT id, rel_path FROM tracks WHERE media_file_id = @media_file_id AND source = 'sidecar' ORDER BY rel_path;

-- name: InsertSidecarTrack :exec
-- A sidecar found by a scan; stream_index -1 like the API reports sidecars.
INSERT INTO tracks (media_file_id, stream_index, kind, codec, language, title, source, rel_path, is_default, is_forced)
VALUES (@media_file_id, -1, 'subtitle', @codec, @language, @title, 'sidecar', @rel_path, 0, @is_forced)
ON CONFLICT DO NOTHING;

-- name: DeleteTrack :exec
DELETE FROM tracks WHERE id = @id;

-- name: UpdateItemRatingOverride :exec
-- Administrator override of the age rating (M2): own_rating, rating_source ('manual', or the value
-- the caller restores when the override is cleared) and locked_fields in one statement, so the
-- triggers recompute the effective rating of the title and its descendants.
UPDATE items
SET own_rating = @own_rating, rating_source = @rating_source, locked_fields = @locked_fields, updated_at = @now
WHERE id = @id AND deleted_at IS NULL;
