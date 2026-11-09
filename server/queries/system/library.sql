-- Unfiltered queries of the library package for series, extras, ignored files and the scan
-- benchmark (package store/systemq). Owner: library package. Times are Unix milliseconds.

-- name: ListIgnoredFilesByRoot :many
-- Files the administrator removed from the library (B-9); the scanner skips them.
SELECT rel_path FROM ignored_files WHERE root_id = @root_id ORDER BY rel_path;

-- name: InsertIgnoredFile :exec
INSERT INTO ignored_files (root_id, rel_path, reason, created_at)
VALUES (@root_id, @rel_path, @reason, @now)
ON CONFLICT (root_id, rel_path) DO UPDATE SET reason = excluded.reason;

-- name: DeleteMediaFile :exec
DELETE FROM media_files WHERE id = @id;

-- name: SoftDeleteItemWithoutFiles :execrows
-- A movie, episode or extra whose last media file was removed leaves the library.
UPDATE items SET deleted_at = @now, updated_at = @now
WHERE items.id = @id AND items.deleted_at IS NULL AND items.kind IN ('movie', 'episode', 'extra')
  AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id);

-- name: UpdateMediaFileProbeError :execrows
-- ffprobe failed on the file (B-9 "unplayable"): the attempt is recorded, the stream columns stay.
-- Only for the version (size, mtime) the probe read, like UpdateMediaFileProbe: 0 rows means the
-- file changed meanwhile, and probe.file runs again on the new version (concept 14, Z-9).
UPDATE media_files SET probe_error = @probe_error, probed_at = @now, updated_at = @now
WHERE id = @id AND size = @size AND mtime = @mtime;

-- name: SoftDeleteEmptySeasons :execrows
-- Seasons whose children (episodes, extras) are all soft-deleted leave with them (sweep).
UPDATE items SET deleted_at = @now, updated_at = @now
WHERE kind = 'season' AND deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL);

-- name: SoftDeleteEmptySeries :execrows
-- Series whose children (seasons, extras) are all soft-deleted leave with them (sweep).
UPDATE items SET deleted_at = @now, updated_at = @now
WHERE kind = 'series' AND deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL);

-- name: CountLibraryItemsByKind :many
SELECT kind, count(*) AS n FROM items
WHERE library_id = @library_id AND deleted_at IS NULL
GROUP BY kind ORDER BY kind;

-- name: JobStatsSince :one
-- Job counts of the scan benchmark: all jobs created since a time, the done ones, those that ran
-- more than once (attempts > 1: retried or claimed again after a lost lease) and those that
-- failed because their lease expired.
SELECT count(*) AS total,
       CAST(coalesce(sum(state = 'done'), 0) AS INTEGER) AS done,
       CAST(coalesce(sum(state IN ('queued', 'running')), 0) AS INTEGER) AS active,
       CAST(coalesce(sum(attempts > 1), 0) AS INTEGER) AS repeated,
       CAST(coalesce(sum(state = 'failed' AND last_error LIKE 'lease expired%'), 0) AS INTEGER) AS lease_expired
FROM jobs WHERE created_at >= @since;

-- name: CountDuplicateJobKeys :one
-- Idempotency keys with a prefix (for example 'probe:%') held by more than one job created since
-- a time: work queued twice.
SELECT count(*) FROM (
    SELECT idempotency_key FROM jobs
    WHERE created_at >= @since AND idempotency_key LIKE @prefix
    GROUP BY idempotency_key HAVING count(*) > 1
);

-- name: CountLibraryProbeErrors :one
SELECT count(*) FROM media_files AS m
JOIN library_roots AS r ON r.id = m.root_id
WHERE r.library_id = @library_id AND m.probe_error IS NOT NULL;

-- Removing a folder from a library (idea 02, E12). The order of the three queries is not taste:
-- media_files.root_id cascades on delete, so a title left without any file would never be seen
-- by SoftDeleteItemsMissingSince again (it requires EXISTS (SELECT 1 FROM media_files ...)), and
-- downloads.media_file_id cascades too, so a surviving title would lose its download.

-- name: SoftDeleteItemsOfRootWithoutOtherFiles :execrows
-- Titles that live only in this folder leave the library at once. Unlike
-- SoftDeleteItemWithoutFiles this carries 'audiobook': an audiobook library would keep empty
-- rows behind otherwise. "Another folder" means any other registered folder, not only one of
-- the same library: folders may not overlap, so the two coincide, except for a title that was
-- given a file of another library by AdminMoveMediaFiles while merging. Keeping that title is
-- the safe reading -- it still has a file to play.
UPDATE items SET deleted_at = @now, updated_at = @now
WHERE items.deleted_at IS NULL
  AND (items.kind IN ('movie', 'episode', 'extra', 'audiobook') OR (items.kind = 'podcast_episode' AND items.enclosure_url IS NULL))
  AND EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.root_id = @root_id)
  AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.root_id <> @root_id);

-- name: MoveDownloadsOffRoot :execrows
-- Downloads of the titles that survive move to a file of theirs under another folder; a present
-- file wins over a missing one. Without this the cascade of the next delete would take them, and
-- the tombstone trigger would tell the device to drop a package whose title is still visible.
UPDATE downloads SET media_file_id = (
        SELECT m.id FROM media_files AS m
        WHERE m.item_id = downloads.item_id AND m.root_id <> @root_id
        ORDER BY m.missing_since IS NOT NULL, m.id LIMIT 1
    ), updated_at = @now
WHERE downloads.media_file_id IN (SELECT m.id FROM media_files AS m WHERE m.root_id = @root_id)
  AND EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = downloads.item_id AND m.root_id <> @root_id);

-- name: CountMediaFilesOfRoot :one
-- For the Warn line of the removal: how many files go with the folder.
SELECT count(*) FROM media_files WHERE root_id = @root_id;

-- name: ListMediaFileRelPathsOfRoot :many
-- The known files of a folder that gets a new path (updateLibraryRoot, ADR 0023): at least one of
-- them must lie under the new path.
SELECT rel_path FROM media_files WHERE root_id = @root_id ORDER BY rel_path;

-- name: DeleteLibraryRoot :execrows
-- ON DELETE CASCADE clears media_files of the folder; the triggers set items.available.
DELETE FROM library_roots WHERE id = @id AND library_id = @library_id;

-- name: CountLibraryRootContent :one
-- What a folder holds, for the confirmation of its removal (idea 02, E12): its media files, and
-- the titles that have at least one file in it. The second number is the upper bound of what the
-- removal takes out -- a title with a file in another folder of the library stays -- and one pass
-- over the root_id index answers both.
SELECT count(*) AS files, count(DISTINCT item_id) AS items FROM media_files WHERE root_id = @root_id;

-- Content and provider list of a folder (updateLibraryRoot, ADR 0015, concept 6.3).

-- name: ListTitleIDsOfRoot :many
-- The titles with a file in the folder, as the fill and match jobs need them: a movie, audiobook
-- or podcast is its own title, an episode stands for its series (the parent of its season, or its
-- direct parent for an episode without season folder), an extra for the title above it.
SELECT DISTINCT COALESCE(
        CASE WHEN g.kind = 'series' THEN g.id END,
        CASE WHEN p.kind IN ('series', 'movie', 'podcast') THEN p.id END,
        i.id) AS id
FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
LEFT JOIN items AS p ON p.id = i.parent_id
LEFT JOIN items AS g ON g.id = p.parent_id
WHERE m.root_id = @root_id AND i.deleted_at IS NULL
ORDER BY id;

-- name: UpdateItemsProvidersByRoot :execrows
-- The new list of a folder on every title with a file in it, its ancestors and the descendants of
-- those ancestors: a series with all its seasons and episodes, a movie with its extras. The tree
-- is three levels deep at most (series, season, episode), so the ancestors are the parent and the
-- grandparent, the descendants the children and the grandchildren of what those name.
UPDATE items SET providers = @providers, updated_at = @now
WHERE deleted_at IS NULL AND (
    id IN (SELECT m.item_id FROM media_files AS m WHERE m.root_id = @root_id)
    OR id IN (SELECT i.parent_id FROM media_files AS m JOIN items AS i ON i.id = m.item_id WHERE m.root_id = @root_id)
    OR id IN (SELECT p.parent_id FROM media_files AS m JOIN items AS i ON i.id = m.item_id
              JOIN items AS p ON p.id = i.parent_id WHERE m.root_id = @root_id)
    OR parent_id IN (SELECT i.parent_id FROM media_files AS m JOIN items AS i ON i.id = m.item_id WHERE m.root_id = @root_id)
    OR parent_id IN (SELECT p.parent_id FROM media_files AS m JOIN items AS i ON i.id = m.item_id
                     JOIN items AS p ON p.id = i.parent_id WHERE m.root_id = @root_id)
    OR parent_id IN (SELECT c.id FROM items AS c WHERE c.parent_id IN (
                         SELECT p.parent_id FROM media_files AS m JOIN items AS i ON i.id = m.item_id
                         JOIN items AS p ON p.id = i.parent_id WHERE m.root_id = @root_id)));

-- name: DeleteMediaFilesOfRoot :execrows
-- A content change reads the folder afresh (concept 6.3): the cascade takes the downloads of the
-- soft-deleted titles with it and the triggers set items.available.
DELETE FROM media_files WHERE root_id = @root_id;

-- name: DeleteIgnoredFilesOfRoot :execrows
-- The ignored files of a folder refer to its old reading; with another content they start over.
DELETE FROM ignored_files WHERE root_id = @root_id;

-- Chapter images (concept 13 stage 3, chapters.images).

-- name: ListChapterImageFiles :many
-- The probed media files of films and episodes with their probe: the backfill queues
-- chapters.images for those without images, the sweep keeps the image folders of exactly these
-- files at their probe and chapter list.
SELECT m.id, m.probed_at, m.probe, m.duration_ms, i.kind, m.missing_since IS NOT NULL AS missing
FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
WHERE i.kind IN ('movie', 'episode') AND i.deleted_at IS NULL AND m.probed_at IS NOT NULL AND m.probe IS NOT NULL
  AND m.probe_error IS NULL
ORDER BY m.id;

-- name: GetChapterImageFile :one
-- A media file for chapters.images: the stored probe at its probed_at, its duration, the kind of
-- its title and the folder it lies in. A file of a deleted title or whose last probe failed has
-- none.
SELECT m.id, m.rel_path, m.duration_ms, m.probe, m.probed_at, m.missing_since, i.kind, r.path AS root_path
FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
JOIN library_roots AS r ON r.id = m.root_id
WHERE m.id = @id AND i.deleted_at IS NULL AND m.probe_error IS NULL;

-- name: CountActivePlaybacksOfFile :one
-- Playback sessions of a media file that still play: neither revoked nor expired, with a request
-- since @active_after. chapters.images waits while one exists (concept 13 11.6, not during
-- playback).
SELECT count(*) FROM playback_sessions
WHERE media_file_id = @media_file_id AND revoked_at IS NULL AND expires_at > @now AND last_activity_at >= @active_after;

-- Trickplay (concept 18 part A, ADR 0027). The setting of the library comes with every file, so the
-- job reads it before each sheet.

-- name: GetTrickplayFile :one
-- A film or episode file for trickplay.generate: version, duration, stored probe and keyframe
-- index, the folder it lies in and the trickplay setting of its library. A file of a deleted title
-- or whose last probe failed has none.
SELECT m.id, m.item_id, m.size, m.mtime, m.duration_ms, m.probe, m.probed_at, m.keyframes, m.keyframes_source,
       m.missing_since, m.rel_path, i.kind, i.library_id, r.path AS root_path,
       l.trickplay_movies, l.trickplay_movies_keep, l.trickplay_series, l.trickplay_series_keep, l.trickplay_series_scope
FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
JOIN library_roots AS r ON r.id = m.root_id
JOIN libraries AS l ON l.id = i.library_id
WHERE m.id = @id AND i.deleted_at IS NULL AND m.probe_error IS NULL;

-- name: ListTrickplayFiles :many
-- Every film and episode file of a title that is not deleted, with its version, whether it is
-- missing or probed, its keyframe source and the setting of its library: the backfill queues the
-- files without a complete set, the sweep keeps the sets of exactly these files at their version.
SELECT m.id, m.size, m.mtime, m.missing_since IS NOT NULL AS missing, m.probed_at IS NOT NULL AND m.probe_error IS NULL AS probed,
       m.keyframes_source, i.kind, i.library_id,
       l.trickplay_movies, l.trickplay_movies_keep, l.trickplay_series, l.trickplay_series_keep, l.trickplay_series_scope
FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
JOIN libraries AS l ON l.id = i.library_id
WHERE i.kind IN ('movie', 'episode') AND i.deleted_at IS NULL
ORDER BY m.id;

-- name: ListTrickplaySeriesFiles :many
-- The present, probed files of the episodes of a series for its look-ahead (concept 18, 6.4), in
-- season and episode order; per episode the primary file first (largest height, then the oldest),
-- the rule of visibility.MediaFile without a choice.
SELECT e.id AS item_id, CAST(coalesce(e.season_number, se.season_number, 0) AS INTEGER) AS season,
       CAST(coalesce(e.episode_number, 0) AS INTEGER) AS episode, m.id AS media_file_id
FROM items AS e
JOIN items AS se ON se.id = e.parent_id AND se.kind = 'season'
JOIN media_files AS m ON m.item_id = e.id
WHERE se.parent_id = @series_id AND e.kind = 'episode' AND e.deleted_at IS NULL
  AND m.missing_since IS NULL AND m.probed_at IS NOT NULL AND m.probe_error IS NULL
ORDER BY season, episode, e.id, COALESCE(m.height, 0) DESC, m.created_at, m.id;

-- name: CountWaitingTrickplayStarts :one
-- Start jobs of trickplay.generate that wait and may run now (priority @priority and higher): a
-- running import, backfill, look-ahead or download job yields to them after its sheet.
SELECT count(*) FROM jobs
WHERE type = 'trickplay.generate' AND state = 'queued' AND priority >= @priority AND run_after <= @now;

-- name: TrickplayJobEnded :one
-- Whether a run of trickplay.generate under @idempotency_key ended (done, failed or cancelled): with
-- no active job and no complete set, the set of the file ended without being made (state failed).
SELECT EXISTS (
    SELECT 1 FROM jobs
    WHERE type = 'trickplay.generate' AND state IN ('done', 'failed', 'cancelled') AND idempotency_key = @idempotency_key
);

-- name: CountOpenPlaybacks :one
-- Playback sessions that are open (neither revoked nor expired): the series look-ahead beyond the
-- next two episodes runs one ffmpeg process while one exists.
SELECT count(*) FROM playback_sessions WHERE revoked_at IS NULL AND expires_at > @now;

-- name: CountPlaybacksOfFileOpenSince :one
-- Playback sessions of a media file that were open at some point since @since (still open, or
-- revoked or expired after it): a set that does not store goes 10 minutes after the last one.
SELECT count(*) FROM playback_sessions
WHERE media_file_id = @media_file_id AND expires_at > @since AND (revoked_at IS NULL OR revoked_at > @since);
