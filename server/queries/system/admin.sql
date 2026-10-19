-- Unfiltered title queries of the administration (package store/systemq). Owner: admin package.
-- Every query name starts with Admin so the files of the other owners never collide. Times are
-- Unix milliseconds.

-- Administration view of a title (getAdminItem, design 4.2).

-- name: AdminListMediaFiles :many
-- Media files of the titles with their root and path, per title in the order of the versions
-- (available first, then the largest height, then the oldest).
SELECT m.id, m.item_id, m.root_id, m.rel_path, m.size, m.version_label, m.height, m.probed_at, m.probe_error,
       m.missing_since
FROM media_files AS m
WHERE m.item_id IN (sqlc.slice('item_ids'))
ORDER BY m.item_id, m.missing_since IS NOT NULL, COALESCE(m.height, 0) DESC, m.created_at, m.id;

-- name: AdminCountChildren :one
-- Children of a title that are not deleted: seasons, episodes (of the title and of its seasons)
-- and extras.
SELECT
    (SELECT count(*) FROM items AS s WHERE s.parent_id = @id AND s.kind = 'season' AND s.deleted_at IS NULL) AS seasons,
    (SELECT count(*) FROM items AS e WHERE e.kind = 'episode' AND e.deleted_at IS NULL
        AND (e.parent_id = @id OR e.parent_id IN (SELECT p.id FROM items AS p WHERE p.parent_id = @id AND p.kind = 'season'))) AS episodes,
    (SELECT count(*) FROM items AS x WHERE x.parent_id = @id AND x.kind = 'extra' AND x.deleted_at IS NULL) AS extras;

-- Maintenance issues (listMaintenanceIssues, B-9). Each kind has a page query and a count; a
-- NULL library_id lists every library.

-- name: AdminCountUnmatched :one
SELECT count(*) FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series', 'episode')
  AND (i.metadata_state IN ('unmatched', 'failed')
       OR (i.metadata_state = 'skipped' AND i.kind IN ('movie', 'series'))
       OR (i.metadata_state = 'pending' AND i.created_at < @pending_before))
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'));

-- name: AdminListUnmatched :many
SELECT i.id, i.kind, i.title, i.year, i.library_id, i.metadata_state, i.metadata_error FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series', 'episode')
  AND (i.metadata_state IN ('unmatched', 'failed')
       OR (i.metadata_state = 'skipped' AND i.kind IN ('movie', 'series'))
       OR (i.metadata_state = 'pending' AND i.created_at < @pending_before))
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'))
ORDER BY i.library_id, i.sort_title, i.id
LIMIT @limit_count OFFSET @offset_count;

-- name: AdminCountNoImages :one
SELECT count(*) FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series') AND i.metadata_state = 'matched'
  AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'poster' AND im.size = 'w400')
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'));

-- name: AdminListNoImages :many
SELECT i.id, i.kind, i.title, i.year, i.library_id FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series') AND i.metadata_state = 'matched'
  AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'poster' AND im.size = 'w400')
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'))
ORDER BY i.library_id, i.sort_title, i.id
LIMIT @limit_count OFFSET @offset_count;

-- name: AdminCountUnrated :one
SELECT count(*) FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series', 'episode') AND i.effective_rating IS NULL
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'));

-- name: AdminListUnrated :many
SELECT i.id, i.kind, i.title, i.year, i.library_id FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series', 'episode') AND i.effective_rating IS NULL
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'))
ORDER BY i.library_id, i.sort_title, i.id
LIMIT @limit_count OFFSET @offset_count;

-- name: AdminCountOrphaned :one
SELECT count(*) FROM items AS i
WHERE i.deleted_at IS NULL
  AND ((i.kind IN ('movie', 'episode', 'extra', 'audiobook')
        AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id AND m.missing_since IS NULL))
       OR (i.kind = 'season' AND NOT EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = i.id AND c.kind = 'episode' AND c.deleted_at IS NULL))
       OR (i.kind = 'series' AND NOT EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = i.id AND c.kind = 'season' AND c.deleted_at IS NULL)))
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'));

-- name: AdminListOrphaned :many
-- detail: no_file (no media file), files_missing (every file has missing_since), no_episodes
-- (season), no_seasons (series).
SELECT i.id, i.kind, i.title, i.year, i.library_id,
       CAST(CASE
           WHEN i.kind = 'season' THEN 'no_episodes'
           WHEN i.kind = 'series' THEN 'no_seasons'
           WHEN EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id) THEN 'files_missing'
           ELSE 'no_file' END AS TEXT) AS detail
FROM items AS i
WHERE i.deleted_at IS NULL
  AND ((i.kind IN ('movie', 'episode', 'extra', 'audiobook')
        AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id AND m.missing_since IS NULL))
       OR (i.kind = 'season' AND NOT EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = i.id AND c.kind = 'episode' AND c.deleted_at IS NULL))
       OR (i.kind = 'series' AND NOT EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = i.id AND c.kind = 'season' AND c.deleted_at IS NULL)))
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'))
ORDER BY i.library_id, i.sort_title, i.id
LIMIT @limit_count OFFSET @offset_count;

-- name: AdminCountUnplayable :one
SELECT count(*) FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
WHERE m.probe_error IS NOT NULL AND i.deleted_at IS NULL
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'));

-- name: AdminListUnplayable :many
SELECT i.id, i.kind, i.title, i.year, i.library_id, m.id AS media_file_id, m.rel_path, m.probe_error
FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
WHERE m.probe_error IS NOT NULL AND i.deleted_at IS NULL
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'))
ORDER BY i.library_id, i.sort_title, i.id, m.rel_path
LIMIT @limit_count OFFSET @offset_count;

-- name: AdminListRatingState :many
-- Every title with the inputs and the stored results of the rating and container triggers
-- (VerifyRatings); has_file: a media file without missing_since.
SELECT i.id, i.kind, i.parent_id, i.library_id, i.title, i.year, i.own_rating, i.effective_rating,
       i.min_child_rating, i.available, i.deleted_at,
       EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id AND m.missing_since IS NULL) AS has_file
FROM items AS i
ORDER BY i.library_id, i.sort_title, i.id;

-- Duplicates (listDuplicates, B-9).

-- name: AdminListProviderDuplicates :many
-- Films and series of one library that share a provider id with another title of the same kind.
SELECT CAST('tmdb:' || i.tmdb_id AS TEXT) AS group_key, i.id, i.kind, i.title, i.year, i.library_id
FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series') AND i.tmdb_id IS NOT NULL
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'))
  AND EXISTS (SELECT 1 FROM items AS o WHERE o.id <> i.id AND o.deleted_at IS NULL AND o.library_id = i.library_id
              AND o.kind = i.kind AND o.tmdb_id = i.tmdb_id)
UNION ALL
SELECT CAST('tvdb:' || i.tvdb_id AS TEXT) AS group_key, i.id, i.kind, i.title, i.year, i.library_id
FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series') AND i.tvdb_id IS NOT NULL
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'))
  AND EXISTS (SELECT 1 FROM items AS o WHERE o.id <> i.id AND o.deleted_at IS NULL AND o.library_id = i.library_id
              AND o.kind = i.kind AND o.tvdb_id = i.tvdb_id)
UNION ALL
SELECT CAST('imdb:' || i.imdb_id AS TEXT) AS group_key, i.id, i.kind, i.title, i.year, i.library_id
FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series') AND i.imdb_id IS NOT NULL
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'))
  AND EXISTS (SELECT 1 FROM items AS o WHERE o.id <> i.id AND o.deleted_at IS NULL AND o.library_id = i.library_id
              AND o.kind = i.kind AND o.imdb_id = i.imdb_id)
ORDER BY 1, 6, 2;

-- name: AdminListTitleYearDuplicates :many
-- Films and series without any provider id that share sort title and year with another one of
-- the same kind and library.
SELECT i.id, i.kind, i.title, i.sort_title, i.year, i.library_id
FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series')
  AND i.tmdb_id IS NULL AND i.tvdb_id IS NULL AND i.imdb_id IS NULL
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'))
  AND EXISTS (SELECT 1 FROM items AS o WHERE o.id <> i.id AND o.deleted_at IS NULL AND o.library_id = i.library_id
              AND o.kind = i.kind AND o.tmdb_id IS NULL AND o.tvdb_id IS NULL AND o.imdb_id IS NULL
              AND o.sort_title = i.sort_title AND o.year IS i.year)
ORDER BY i.library_id, i.kind, i.sort_title, i.year, i.id;

-- name: AdminListHashDuplicates :many
-- Media files whose content hash a file of another title shares.
SELECT m.partial_hash, m.id AS media_file_id, m.root_id, m.rel_path, m.size, m.version_label, m.missing_since,
       i.id, i.kind, i.title, i.year, i.library_id
FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
WHERE m.partial_hash IS NOT NULL AND i.deleted_at IS NULL
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'))
  AND EXISTS (SELECT 1 FROM media_files AS o JOIN items AS oi ON oi.id = o.item_id
              WHERE o.partial_hash = m.partial_hash AND o.item_id <> m.item_id AND oi.deleted_at IS NULL)
ORDER BY m.partial_hash, i.id, m.id;

-- Merge (mergeItems, B-9): one transaction moves everything of a source to the target, then
-- deletes the source. The triggers recompute availability and ratings on the way.

-- name: AdminMoveMediaFiles :exec
UPDATE media_files SET item_id = @target_id, updated_at = @now WHERE item_id = @source_id;

-- name: AdminMoveChildren :exec
UPDATE items SET parent_id = @target_id, updated_at = @now WHERE parent_id = @source_id;

-- name: AdminMoveTrailers :exec
-- A trailer the target already has (same language and video) stays with the source and goes with it.
UPDATE OR IGNORE trailers SET item_id = @target_id WHERE item_id = @source_id;

-- name: AdminMoveCredits :exec
UPDATE OR IGNORE credits SET item_id = @target_id WHERE item_id = @source_id;

-- name: AdminMergeProgress :exec
-- Per profile the further progress wins: completed over not completed, then the larger position.
INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at)
SELECT p.profile_id, @target_id, p.position_ms, p.duration_ms, p.completed, p.updated_at
FROM progress AS p WHERE p.item_id = @source_id AND true
ON CONFLICT (profile_id, item_id) DO UPDATE
SET position_ms = excluded.position_ms, duration_ms = excluded.duration_ms, completed = excluded.completed,
    updated_at = max(progress.updated_at, excluded.updated_at)
WHERE excluded.completed > progress.completed
   OR (excluded.completed = progress.completed AND excluded.position_ms > progress.position_ms);

-- name: AdminMoveProgressEvents :exec
UPDATE progress_events SET item_id = @target_id WHERE item_id = @source_id;

-- name: AdminMoveTopPicks :exec
UPDATE top_picks SET item_id = @target_id WHERE item_id = @source_id;

-- name: AdminDeleteItem :exec
DELETE FROM items WHERE id = @id;

-- Cleanup (cleanupItems, B-9).

-- name: AdminCleanupState :one
-- What decides whether a title may be soft-deleted: its files (all, present) and its children
-- that carry content.
SELECT i.kind, i.deleted_at,
       (SELECT count(*) FROM media_files AS m WHERE m.item_id = i.id AND m.missing_since IS NULL) AS present_files,
       (SELECT count(*) FROM items AS c WHERE c.parent_id = i.id AND c.deleted_at IS NULL
          AND c.kind IN ('season', 'episode', 'podcast_episode')) AS content_children
FROM items AS i WHERE i.id = @id;

-- name: AdminSoftDeleteItem :execrows
UPDATE items SET deleted_at = @now, updated_at = @now WHERE id = @id AND deleted_at IS NULL;
