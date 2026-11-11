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
