-- Unfiltered queries of the stored podcast episodes (package store/systemq, concept 19 part A,
-- ADR 0029). Owner: library package (planner podcast.sync, download podcast.download, folder
-- check); PodcastEffectiveSettings is shared with the metadata package, which lists the podcasts of
-- the administration from it. Every query name starts with Podcast. Times are Unix milliseconds.

-- name: PodcastEffectiveSettings :many
-- The one rule of the effective setting (5.1): a podcast's own value, else its library's. last_n
-- counts only with an own scope (the N belongs to the scope). Live subscriptions with a feed only:
-- a folder podcast without feed has nothing to load. Optionally for one podcast or one library.
SELECT p.id AS podcast_id, p.library_id, p.title, p.author, p.feed_url, p.feed_origin, p.cover_url, p.cover_path,
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
