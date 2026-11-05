-- Unfiltered title queries for the scanner (package store/systemq). Owner: library package.
-- Times are Unix milliseconds.

-- name: InsertItem :one
-- providers is the ordered metadata source list of the root the file lies in (ADR 0015); a child
-- (season, episode, extra) carries the list of its parent.
INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, original_title, year, release_date,
                   tmdb_id, tvdb_id, imdb_id, season_number, episode_number, episode_end, extra_kind,
                   own_rating, rating_source, providers, added_at, created_at, updated_at)
VALUES (@id, @kind, @parent_id, @library_id, @title, @sort_title, @original_title, @year, @release_date,
        @tmdb_id, @tvdb_id, @imdb_id, @season_number, @episode_number, @episode_end, @extra_kind,
        @own_rating, @rating_source, @providers, @now, @now, @now)
RETURNING *;

-- name: GetItemSystem :one
SELECT * FROM items WHERE id = @id;

-- name: ListItemsByLibrary :many
SELECT * FROM items WHERE library_id = @library_id AND deleted_at IS NULL ORDER BY id;

-- name: ListDeletedItemsByLibrary :many
-- Soft-deleted items that a re-downloaded file may bring back (plan 3.2, ADR 0023): titles by
-- provider id without a time limit, by (sort_title, year) within titleRelinkWindow, and their
-- seasons and episodes; extras and podcast episodes only for the tree (kind, parent), so a
-- returning extra restores its season and series in the index as well. Only the columns of the
-- scanner's item index (loadItemIndex); the most recently deleted first, the later id among equal
-- times (as FindDeletedTwin). The list names every kind, so the planner seeks items_library_kind
-- (library, kind, deleted_at) instead of walking the live titles of the library.
SELECT id, kind, parent_id, season_number, episode_number, sort_title, year, tmdb_id, tvdb_id, imdb_id, deleted_at
FROM items
WHERE library_id = @library_id AND deleted_at IS NOT NULL
  AND kind IN ('movie', 'series', 'season', 'episode', 'extra', 'audiobook', 'podcast', 'podcast_episode')
ORDER BY deleted_at DESC, id DESC;

-- name: CountLibraryItems :one
SELECT count(*) FROM items WHERE library_id = @library_id AND deleted_at IS NULL;

-- name: CountAllLibraryItems :one
-- The sum of CountLibraryItems over every library (concept 11, Dashboard.counts.titles).
SELECT count(*) FROM items WHERE deleted_at IS NULL AND library_id IN (SELECT id FROM libraries);

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
-- A podcast episode counts only without enclosure: one with an enclosure stays as a feed episode
-- when its file goes (ADR 0015).
UPDATE items SET deleted_at = @now, updated_at = @now
WHERE deleted_at IS NULL
  AND (kind IN ('movie', 'episode', 'extra', 'audiobook') OR (kind = 'podcast_episode' AND enclosure_url IS NULL))
  AND EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id)
  AND NOT EXISTS (SELECT 1 FROM media_files AS m
                  WHERE m.item_id = items.id AND (m.missing_since IS NULL OR m.missing_since >= @before));

-- name: UpdateMediaFileLocation :exec
-- Same file at a new path (rename, move), changed in place, or back after missing_since.
-- reset_index (1 when size or mtime changed, decided by the writer whichever way the scanner
-- matched the file) clears the keyframe index of the old version; a rename or move without a
-- change keeps it (concept 14, 4.2, Z-1).
UPDATE media_files
SET rel_path = @rel_path, dev = @dev, inode = @inode, size = @size, mtime = @mtime,
    partial_hash = @partial_hash, version_label = @version_label, missing_since = NULL,
    keyframes = CASE WHEN CAST(@reset_index AS INTEGER) = 1 THEN NULL ELSE keyframes END,
    keyframes_at = CASE WHEN CAST(@reset_index AS INTEGER) = 1 THEN NULL ELSE keyframes_at END,
    keyframes_source = CASE WHEN CAST(@reset_index AS INTEGER) = 1 THEN NULL ELSE keyframes_source END,
    updated_at = @now
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

-- name: UpdateMediaFileProbe :execrows
-- Only for the version (size, mtime) the probe read: 0 rows means the file changed meanwhile, and
-- probe.file retries with the new version (concept 14, 4.2, Z-9).
UPDATE media_files
SET container = @container, video_codec = @video_codec, audio_codec = @audio_codec, width = @width,
    height = @height, duration_ms = @duration_ms, probe = @probe, probe_error = NULL, probed_at = @now,
    updated_at = @now
WHERE id = @id AND size = @size AND mtime = @mtime;

-- name: SetMediaFileKeyframes :execrows
-- The keyframe index is final per file version (concept 14, I-1): written only while there is
-- none and the row still has the size and mtime the writer remembered before it read the file.
-- 0 rows (an index is there, or the file changed) is not an error. keyframes_source is cues,
-- mp4, scan (with a blob) or none (blob NULL).
UPDATE media_files SET keyframes = @keyframes, keyframes_at = @now, keyframes_source = @source, updated_at = @now
WHERE id = @id AND keyframes_source IS NULL AND size = @size AND mtime = @mtime;

-- name: ResetUnavailableKeyframes :exec
-- The refresh of a title in the maintenance retries a file marked "index impossible"; a stored
-- index (cues, mp4, scan) stays (concept 14, 4.2, O-16).
UPDATE media_files SET keyframes_source = NULL, updated_at = @now WHERE id = @id AND keyframes_source = 'none';

-- name: ListKeyframesBackfillFiles :many
-- The files keyframes.backfill reads (concept 14, 6.6): worth an index and without one, by id
-- after the cursor @after_id, @limit_count at a time. "Worth an index" is the condition of
-- library.indexWorthy (4.1): present, a video codec that can be remuxed
-- (transcode.CopyableVideo), neither an audiobook nor a podcast episode. It implies the WHERE of
-- the partial index media_files_keyframes, which carries the search (INDEXED BY: without
-- statistics the planner takes media_files_missing, which lists nearly every file); the id in
-- the index gives the order of the cursor without a sort. CountKeyframesBackfillFiles states the
-- same condition.
SELECT mf.id, mf.root_id, mf.rel_path, mf.container, mf.duration_ms, mf.video_codec, mf.size, mf.mtime, i.kind
FROM media_files AS mf INDEXED BY media_files_keyframes
JOIN items AS i ON i.id = mf.item_id
WHERE mf.keyframes_source IS NULL AND mf.missing_since IS NULL AND mf.video_codec IN ('h264', 'hevc', 'av1', 'vp9')
  AND i.kind NOT IN ('audiobook', 'podcast_episode') AND mf.id > @after_id
ORDER BY mf.id
LIMIT @limit_count;

-- name: CountKeyframesBackfillFiles :one
-- The count of ListKeyframesBackfillFiles without a cursor: the total of the backfill's progress.
SELECT count(*)
FROM media_files AS mf INDEXED BY media_files_keyframes
JOIN items AS i ON i.id = mf.item_id
WHERE mf.keyframes_source IS NULL AND mf.missing_since IS NULL AND mf.video_codec IN ('h264', 'hevc', 'av1', 'vp9')
  AND i.kind NOT IN ('audiobook', 'podcast_episode');

-- name: LastJobFailedSince :one
-- Whether the newest finished run of a type under an idempotency key failed at or after @since
-- (concept 14, 6.4, O-17: a playback queues no new full keyframe read for a file whose last read
-- failed within 24 h). A run newer than one at @since finished at @since or later too, so only
-- the runs since @since are looked at: a range of the index jobs_history, the newest of them by
-- finished_at decides.
SELECT EXISTS (
    SELECT 1 FROM (
        SELECT l.state FROM jobs AS l
        WHERE l.type = @type AND l.state IN ('done', 'failed', 'cancelled') AND l.finished_at >= @since
          AND l.idempotency_key = @idempotency_key
        ORDER BY l.finished_at DESC, l.id DESC
        LIMIT 1
    ) AS newest
    WHERE newest.state = 'failed'
) AS failed;

-- name: DeleteMediaFileTracks :exec
DELETE FROM tracks WHERE media_file_id = @media_file_id;

-- name: InsertTrack :exec
INSERT INTO tracks (media_file_id, stream_index, kind, codec, language, title, source, rel_path, is_default, is_forced, is_hearing_impaired)
VALUES (@media_file_id, @stream_index, @kind, @codec, @language, @title, @source, @rel_path, @is_default, @is_forced, @is_hearing_impaired);

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
INSERT INTO tracks (media_file_id, stream_index, kind, codec, language, title, source, rel_path, is_default, is_forced, is_hearing_impaired)
VALUES (@media_file_id, -1, 'subtitle', @codec, @language, @title, 'sidecar', @rel_path, 0, @is_forced, @is_hearing_impaired)
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

-- name: ListRootFilesWithItems :many
-- The live media files of a root with the kind and parent of their title, for the local image
-- scan (concept 10 4.6, ADR 0018): a movie's folder, an episode's season and series.
SELECT m.item_id, m.rel_path, i.kind, i.parent_id
FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
WHERE m.root_id = @root_id AND m.missing_since IS NULL AND i.deleted_at IS NULL
ORDER BY m.rel_path;

-- name: ListItemParents :many
-- id and parent of titles, for the series of the seasons of a root (local image scan).
SELECT id, parent_id FROM items WHERE id IN (sqlc.slice('ids'));
