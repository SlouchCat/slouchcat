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

-- name: UpdateMediaFileProbeError :exec
-- ffprobe failed on the file (B-9 "unplayable"): the attempt is recorded, the stream columns stay.
UPDATE media_files SET probe_error = @probe_error, probed_at = @now, updated_at = @now WHERE id = @id;

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

-- name: CountDuplicateActiveJobs :one
-- Idempotency keys held by more than one job since a time (a probe queued twice).
SELECT count(*) FROM (
    SELECT idempotency_key FROM jobs
    WHERE created_at >= @since AND idempotency_key IS NOT NULL
    GROUP BY idempotency_key HAVING count(*) > 1
);
