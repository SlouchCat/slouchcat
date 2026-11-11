-- Unfiltered queries of the stored podcast episodes (package store/systemq, concept 19 part A,
-- ADR 0029). Owner: library package (planner podcast.sync, download podcast.download, folder
-- check); PodcastEffectiveSettings is shared with the metadata package, which lists the podcasts of
-- the administration from it. Every query name starts with Podcast. Times are Unix milliseconds.

-- name: PodcastEffectiveSettings :many
-- The one rule of the effective setting (5.1): a podcast's own value, else its library's. last_n
-- counts only with an own scope (the N belongs to the scope). Live subscriptions with a feed only:
-- a folder podcast without feed has nothing to load. Optionally for one podcast or one library.
SELECT p.id AS podcast_id, p.library_id, p.title, p.author, p.explicit, p.feed_url, p.feed_origin, p.cover_url, p.cover_path,
       s.mode AS own_mode, s.scope AS own_scope, s.last_n AS own_last_n,
       CAST(COALESCE(s.mode, l.podcast_mode) AS TEXT) AS effective_mode,
       CAST(COALESCE(s.scope, l.podcast_scope) AS TEXT) AS effective_scope,
       CAST(CASE WHEN s.scope IS NOT NULL THEN COALESCE(s.last_n, l.podcast_last_n) ELSE l.podcast_last_n END AS INTEGER) AS effective_last_n,
       l.podcast_parallel, l.podcast_root_id, s.pending_root_id, s.pending_folder
FROM items AS p
JOIN libraries AS l ON l.id = p.library_id
LEFT JOIN podcast_settings AS s ON s.item_id = p.id
WHERE p.kind = 'podcast' AND p.deleted_at IS NULL AND p.feed_url IS NOT NULL
  AND (sqlc.narg('podcast_id') IS NULL OR p.id = sqlc.narg('podcast_id'))
  AND (sqlc.narg('library_id') IS NULL OR p.library_id = sqlc.narg('library_id'))
ORDER BY p.library_id, p.sort_title, p.id;

-- name: PodcastSetLibraryDownloads :execrows
-- The setting of a podcast library (LibraryPodcastDownloads); 0 rows for another kind.
UPDATE libraries
SET podcast_mode = @mode, podcast_scope = @scope, podcast_last_n = @last_n, podcast_parallel = @parallel,
    podcast_root_id = @root_id, updated_at = @now
WHERE id = @id AND kind = 'podcasts';

-- name: PodcastUpsertSettings :exec
-- The own setting of a podcast (NULL = like its library); a pending folder stays.
INSERT INTO podcast_settings (item_id, mode, scope, last_n, updated_at)
VALUES (@item_id, @mode, @scope, @last_n, @now)
ON CONFLICT (item_id) DO UPDATE SET mode = excluded.mode, scope = excluded.scope, last_n = excluded.last_n,
    updated_at = excluded.updated_at;

-- name: PodcastRecordDownloadFailure :exec
-- A download that failed its last attempt, with its code (never an address).
INSERT INTO podcast_download_failures (item_id, error, attempts, first_failed_at, last_failed_at)
VALUES (@item_id, @error, 1, @now, @now)
ON CONFLICT (item_id) DO UPDATE SET error = excluded.error, attempts = podcast_download_failures.attempts + 1,
    last_failed_at = excluded.last_failed_at;

-- name: PodcastClearDownloadFailure :exec
DELETE FROM podcast_download_failures WHERE item_id = @item_id;

-- name: PodcastKnownFolders :many
-- Folders of a root that the database knows (the first level of its media files): the folder
-- check (5.4) wants at least one of them on disk before the server writes there.
SELECT DISTINCT CAST(substr(rel_path, 1, instr(rel_path, '/') - 1) AS TEXT) AS folder
FROM media_files WHERE root_id = @root_id AND instr(rel_path, '/') > 1
LIMIT 50;

-- name: PodcastFoldersOf :many
-- The folders of a podcast: the first level of the media files of its episodes, live or deleted,
-- and of those of earlier subscriptions of the same feed in the library (section 8, riegel 1).
SELECT DISTINCT m.root_id, CAST(substr(m.rel_path, 1, instr(m.rel_path, '/') - 1) AS TEXT) AS folder
FROM media_files AS m
JOIN items AS e ON e.id = m.item_id
JOIN items AS p ON p.id = e.parent_id
JOIN library_roots AS r ON r.id = m.root_id
WHERE e.kind = 'podcast_episode' AND instr(m.rel_path, '/') > 1 AND r.content = 'podcast'
  AND (p.id = @podcast_id OR (p.kind = 'podcast' AND p.library_id = @library_id AND p.feed_url = @feed_url))
ORDER BY m.root_id, folder;

-- name: PodcastFilesInFolder :many
-- Every media file below a folder of a root, whichever item it hangs on (riegel 1): prefix is the
-- folder with a trailing slash, upper the same folder followed by '0' (the byte after '/').
SELECT m.id, m.item_id, m.rel_path, m.missing_since, i.kind, i.parent_id, i.guid, i.deleted_at
FROM media_files AS m JOIN items AS i ON i.id = m.item_id
WHERE m.root_id = @root_id AND m.rel_path >= @prefix AND m.rel_path < @upper
ORDER BY m.rel_path;

-- name: PodcastFeedEpisodes :many
-- The live feed episodes of a podcast, newest first (undated ones last), with how many media file
-- rows each has and how many of them are missing.
SELECT i.id, i.title, i.sort_title, i.published_at, i.duration_ms, i.enclosure_url, i.enclosure_type, i.enclosure_length,
       CAST((SELECT count(*) FROM media_files AS m WHERE m.item_id = i.id) AS INTEGER) AS files,
       CAST((SELECT count(*) FROM media_files AS m WHERE m.item_id = i.id AND m.missing_since IS NOT NULL) AS INTEGER) AS missing_files
FROM items AS i
WHERE i.parent_id = @podcast_id AND i.kind = 'podcast_episode' AND i.deleted_at IS NULL AND i.enclosure_url IS NOT NULL
ORDER BY i.published_at IS NULL, i.published_at DESC, i.id;

-- name: PodcastPairable :one
-- Whether a feed episode without a file and a live file episode without guid of the same podcast
-- may still be paired (riegel 1, checked in the pairing transaction).
SELECT CAST(EXISTS (SELECT 1 FROM items AS f
                    WHERE f.id = @feed_episode_id AND f.kind = 'podcast_episode' AND f.deleted_at IS NULL
                      AND f.enclosure_url IS NOT NULL AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = f.id))
        AND EXISTS (SELECT 1 FROM items AS e
                    WHERE e.id = @file_episode_id AND e.kind = 'podcast_episode' AND e.deleted_at IS NULL AND e.guid IS NULL
                      AND e.parent_id = (SELECT parent_id FROM items WHERE id = @feed_episode_id)) AS INTEGER) AS pairable;

-- name: PodcastNewestFolder :many
-- The folder of a podcast for a download (6.2, step 2): the one of its newest file that is there
-- (by mtime), on a podcast folder; none for a podcast without such a file.
SELECT m.root_id, CAST(substr(m.rel_path, 1, instr(m.rel_path, '/') - 1) AS TEXT) AS folder
FROM media_files AS m
JOIN items AS e ON e.id = m.item_id
JOIN library_roots AS r ON r.id = m.root_id
WHERE e.parent_id = @podcast_id AND e.kind = 'podcast_episode' AND m.missing_since IS NULL
  AND instr(m.rel_path, '/') > 1 AND r.content = 'podcast'
ORDER BY m.mtime DESC, m.id DESC
LIMIT 1;

-- name: PodcastEpisodeFiles :many
-- The media file rows of an episode, missing ones included (a removed episode keeps its rows).
SELECT id, root_id, rel_path, missing_since FROM media_files WHERE item_id = @item_id ORDER BY id;

-- name: PodcastRelPathTaken :one
-- Whether a name in a folder is held by a media file row, missing or not (UNIQUE (root_id,
-- rel_path)); the row given as own_id (a removed row of the episode itself) does not count.
SELECT CAST(EXISTS (SELECT 1 FROM media_files WHERE root_id = @root_id AND rel_path = @rel_path AND id <> @own_id) AS INTEGER) AS taken;

-- name: PodcastReviveMediaFile :execrows
-- A removed row of an episode gets the file a download entered (6.2): new place and identity,
-- missing_since empty, no probe and no keyframe index of the old file.
UPDATE media_files
SET root_id = @root_id, rel_path = @rel_path, dev = @dev, inode = @inode, size = @size, mtime = @mtime,
    partial_hash = @partial_hash, container = @container, missing_since = NULL, probe = NULL, probed_at = NULL,
    probe_error = NULL, keyframes = NULL, keyframes_at = NULL, keyframes_source = NULL, updated_at = @now
WHERE id = @id AND item_id = @item_id AND missing_since IS NOT NULL;

-- name: PodcastSetPendingFolder :exec
-- The folder a download created for a podcast without any file (6.2, step 2).
INSERT INTO podcast_settings (item_id, pending_root_id, pending_folder, updated_at)
VALUES (@item_id, @root_id, @folder, @now)
ON CONFLICT (item_id) DO UPDATE SET pending_root_id = excluded.pending_root_id, pending_folder = excluded.pending_folder,
    updated_at = excluded.updated_at;

-- name: PodcastClearPendingFolder :exec
-- The first file of a podcast is entered: its folder follows from its files from now on.
UPDATE podcast_settings SET pending_root_id = NULL, pending_folder = NULL, updated_at = @now
WHERE item_id = @item_id AND (pending_root_id IS NOT NULL OR pending_folder IS NOT NULL);

-- name: PodcastActiveDownloads :many
-- The queued and running downloads of the episodes of a podcast (the planner cancels those that are
-- no longer wanted, 5.3).
SELECT j.id, j.state, j.payload, e.id AS episode_id
FROM jobs AS j JOIN items AS e ON j.idempotency_key = 'podcast-dl:' || e.id
WHERE j.type = 'podcast.download' AND j.state IN ('queued', 'running') AND e.parent_id = @podcast_id
ORDER BY j.id;

-- name: PodcastDownloadFailuresOf :many
-- The failed downloads of the episodes of a podcast.
SELECT f.item_id, f.error, f.attempts, f.first_failed_at, f.last_failed_at
FROM podcast_download_failures AS f JOIN items AS e ON e.id = f.item_id
WHERE e.parent_id = @podcast_id;

-- name: PodcastsWithRetryableFailures :many
-- The podcasts with a failed download that podcast.sync_all tries again (6.2): the last failure
-- older than six hours, the first one within seven days.
SELECT DISTINCT e.parent_id AS podcast_id
FROM podcast_download_failures AS f JOIN items AS e ON e.id = f.item_id
WHERE e.deleted_at IS NULL AND f.last_failed_at <= @retry_before AND f.first_failed_at >= @give_up_before;

-- name: PodcastMeanFileSize :one
-- The mean size of the files of a podcast's episodes that are there (the estimate of an episode
-- without enclosure_length); 0 without one.
SELECT CAST(COALESCE(avg(m.size), 0) AS INTEGER) AS mean
FROM media_files AS m JOIN items AS e ON e.id = m.item_id
WHERE e.parent_id = @podcast_id AND e.kind = 'podcast_episode' AND m.missing_since IS NULL;

-- name: PodcastDownloadStats :many
-- Per live podcast: the bytes of its files that are there, its queued, running and failed downloads
-- and the publication of its newest episode (0 without one; AdminPodcast, section 11).
SELECT p.id AS podcast_id,
       CAST(COALESCE((SELECT sum(m.size) FROM media_files AS m JOIN items AS e ON e.id = m.item_id
                      WHERE e.parent_id = p.id AND e.kind = 'podcast_episode' AND e.deleted_at IS NULL AND m.missing_since IS NULL), 0) AS INTEGER) AS stored_bytes,
       CAST((SELECT count(*) FROM jobs AS j JOIN items AS e ON j.idempotency_key = 'podcast-dl:' || e.id
             WHERE e.parent_id = p.id AND j.type = 'podcast.download' AND j.state = 'queued') AS INTEGER) AS queued,
       CAST((SELECT count(*) FROM jobs AS j JOIN items AS e ON j.idempotency_key = 'podcast-dl:' || e.id
             WHERE e.parent_id = p.id AND j.type = 'podcast.download' AND j.state = 'running') AS INTEGER) AS running,
       CAST((SELECT count(*) FROM podcast_download_failures AS f JOIN items AS e ON e.id = f.item_id
             WHERE e.parent_id = p.id AND e.deleted_at IS NULL) AS INTEGER) AS failed,
       CAST(COALESCE((SELECT max(e.published_at) FROM items AS e
                      WHERE e.parent_id = p.id AND e.kind = 'podcast_episode' AND e.deleted_at IS NULL), 0) AS INTEGER) AS latest_episode_at
FROM items AS p
WHERE p.kind = 'podcast' AND p.deleted_at IS NULL;

-- name: PodcastLibraryMode :one
-- The mode of stored episodes of a podcast library (the effective mode of a podcast that follows it).
SELECT podcast_mode FROM libraries WHERE id = @id;

-- name: PodcastNewestRoots :many
-- Per live podcast with a feed the root of its newest file that is there on a podcast folder, as
-- PodcastNewestFolder picks it for a download (6.2, step 2); 0 without one. For the list of the
-- administration (AdminPodcast.savable), which checks each root once.
SELECT p.id AS podcast_id,
       CAST(COALESCE((SELECT m.root_id FROM media_files AS m
                      JOIN items AS e ON e.id = m.item_id
                      JOIN library_roots AS r ON r.id = m.root_id
                      WHERE e.parent_id = p.id AND e.kind = 'podcast_episode' AND m.missing_since IS NULL
                        AND instr(m.rel_path, '/') > 1 AND r.content = 'podcast'
                      ORDER BY m.mtime DESC, m.id DESC
                      LIMIT 1), 0) AS INTEGER) AS root_id
FROM items AS p
WHERE p.kind = 'podcast' AND p.deleted_at IS NULL AND p.feed_url IS NOT NULL;

-- name: PodcastAllEpisodesWithoutFile :many
-- The live feed episodes of every live podcast with a feed that have no media file row at all (a
-- removed episode keeps its rows): the episodes the riegel judge for AdminPodcast.unclearEpisodeCount.
SELECT CAST(i.parent_id AS TEXT) AS podcast_id, i.id, i.title, i.sort_title, i.published_at, i.duration_ms, i.enclosure_url
FROM items AS i JOIN items AS p ON p.id = i.parent_id
WHERE i.kind = 'podcast_episode' AND i.deleted_at IS NULL AND i.enclosure_url IS NOT NULL
  AND p.kind = 'podcast' AND p.deleted_at IS NULL AND p.feed_url IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id)
ORDER BY i.parent_id, i.published_at IS NULL, i.published_at DESC, i.id;

-- name: PodcastAllFolders :many
-- The folders of every podcast, live or deleted (the first level of the media files of its
-- episodes on a podcast folder), with its library and feed: PodcastFoldersOf for all podcasts at
-- once (riegel 1 counts the folders of earlier subscriptions of the same feed).
SELECT DISTINCT p.id AS podcast_id, p.library_id, p.feed_url, m.root_id,
       CAST(substr(m.rel_path, 1, instr(m.rel_path, '/') - 1) AS TEXT) AS folder
FROM media_files AS m
JOIN items AS e ON e.id = m.item_id
JOIN items AS p ON p.id = e.parent_id
JOIN library_roots AS r ON r.id = m.root_id
WHERE e.kind = 'podcast_episode' AND p.kind = 'podcast' AND instr(m.rel_path, '/') > 1 AND r.content = 'podcast'
ORDER BY p.id, m.root_id, folder;

-- name: PodcastAllFolderFiles :many
-- Every media file in a folder of a podcast folder, whichever item it hangs on (riegel 1):
-- PodcastFilesInFolder for all folders at once, with the folder (the first level of rel_path).
SELECT m.id, m.item_id, m.root_id, m.rel_path, m.missing_since, i.kind, i.parent_id, i.guid, i.deleted_at,
       CAST(substr(m.rel_path, 1, instr(m.rel_path, '/') - 1) AS TEXT) AS folder
FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
JOIN library_roots AS r ON r.id = m.root_id
WHERE r.content = 'podcast' AND instr(m.rel_path, '/') > 1
ORDER BY m.root_id, m.rel_path;

-- name: PodcastAllFileEpisodesWithoutGUID :many
-- The live file episodes (no guid) of every podcast: PodcastListFileEpisodesWithoutGUID for all.
SELECT CAST(i.parent_id AS TEXT) AS podcast_id, i.id, i.title, i.sort_title, i.published_at, i.duration_ms
FROM items AS i
WHERE i.kind = 'podcast_episode' AND i.deleted_at IS NULL AND i.guid IS NULL AND i.parent_id IS NOT NULL;
