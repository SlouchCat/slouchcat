-- Intro and outro detection per season (job segments.detect, W-9; package store/systemq). Times are
-- Unix milliseconds. A file counts as probed when ffprobe succeeded and found a duration.

-- Episodes of a season with their primary media file (the order of ListMediaFilesForItem: largest
-- height, then the oldest), present and probed files only, with the path of the library root.
-- name: ListSegmentEpisodes :many
SELECT e.id AS item_id, m.id AS media_file_id, m.rel_path, m.duration_ms, m.probe, r.path AS root_path
FROM items AS e
JOIN media_files AS m ON m.id = (
    SELECT p.id FROM media_files AS p
    WHERE p.item_id = e.id AND p.missing_since IS NULL AND p.probed_at IS NOT NULL
      AND p.probe_error IS NULL AND p.duration_ms > 0
    ORDER BY COALESCE(p.height, 0) DESC, p.created_at, p.id
    LIMIT 1)
JOIN library_roots AS r ON r.id = m.root_id
WHERE e.parent_id = @season_id AND e.kind = 'episode' AND e.deleted_at IS NULL
ORDER BY e.episode_number, e.id;

-- Seasons of a library whose detection is pending, with at least two probed episodes and no active
-- segments.detect job.
-- name: ListPendingSegmentSeasons :many
SELECT s.id FROM items AS s
WHERE s.library_id = @library_id AND s.kind = 'season' AND s.deleted_at IS NULL
  AND s.segments_state = 'pending'
  AND (SELECT count(DISTINCT e.id) FROM items AS e
       JOIN media_files AS m ON m.item_id = e.id
       WHERE e.parent_id = s.id AND e.kind = 'episode' AND e.deleted_at IS NULL
         AND m.missing_since IS NULL AND m.probed_at IS NOT NULL AND m.probe_error IS NULL
         AND m.duration_ms > 0) >= 2
  AND NOT EXISTS (SELECT 1 FROM jobs AS j
                  WHERE j.idempotency_key = 'segments:' || s.id AND j.state IN ('queued', 'running'))
ORDER BY s.id;

-- Sets the season of the episode that owns a media file back to pending (a new or changed episode
-- file) and returns it with its number of probed episodes; no row for files of other titles.
-- name: MarkFileSeasonSegmentsPending :one
UPDATE items SET segments_state = 'pending', updated_at = @now
WHERE items.id = (SELECT e.parent_id FROM media_files AS f JOIN items AS e ON e.id = f.item_id
            WHERE f.id = @media_file_id AND e.kind = 'episode')
  AND items.kind = 'season'
RETURNING items.id, (SELECT count(DISTINCT e.id) FROM items AS e
               JOIN media_files AS m ON m.item_id = e.id
               WHERE e.parent_id = items.id AND e.kind = 'episode' AND e.deleted_at IS NULL
                 AND m.missing_since IS NULL AND m.probed_at IS NOT NULL AND m.probe_error IS NULL
                 AND m.duration_ms > 0) AS probed_episodes;

-- Removes the detected segments of the episodes of a season; manual ones stay.
-- name: DeleteDetectedSegments :exec
DELETE FROM media_segments
WHERE source IN ('fingerprint', 'chapter')
  AND item_id IN (SELECT id FROM items WHERE parent_id = @season_id AND kind = 'episode');

-- Inserts a detected segment unless a manual one of the kind exists.
-- name: InsertSegment :exec
INSERT INTO media_segments (item_id, kind, start_ms, end_ms, source, confidence, created_at)
VALUES (@item_id, @kind, @start_ms, @end_ms, @source, @confidence, @now)
ON CONFLICT (item_id, kind) DO NOTHING;

-- Records the result of a detection that started at @started, unless an episode file of the
-- season was probed since: then the season stays pending for the next run.
-- name: SetSeasonSegmentsState :execrows
UPDATE items SET segments_state = @state, segments_checked_at = @now, updated_at = @now
WHERE items.id = @season_id AND items.kind = 'season'
  AND NOT EXISTS (SELECT 1 FROM items AS e JOIN media_files AS m ON m.item_id = e.id
                  WHERE e.parent_id = @season_id AND m.probed_at > @started);
