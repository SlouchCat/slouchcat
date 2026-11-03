-- Relinking of soft-deleted titles (ADR 0023, idea 04). Owner: library package. A live film or
-- series and a soft-deleted one are twins when they share the library, the kind and at least one
-- provider id (TMDB, TVDB, IMDb), and no provider id contradicts. Only a fresh copy counts: the
-- live title was created after the twin was deleted, so two titles that lived side by side are
-- never merged. The deleted twin survives with its id; the live copy merges into it. Times are
-- Unix milliseconds.

-- name: FindDeletedTwin :one
-- The soft-deleted twin of a live film or series; among several the most recently deleted wins.
-- No time limit (ADR 0023, point 2). The unary + keeps the planner off items_library_kind, which
-- would walk every deleted title of the library, and on the provider id indexes (MULTI-INDEX OR).
SELECT d.id FROM items AS l
JOIN items AS d ON +d.library_id = l.library_id AND +d.kind = l.kind AND d.id <> l.id
WHERE l.id = @live_id AND l.deleted_at IS NULL AND l.kind IN ('movie', 'series')
  AND +d.deleted_at IS NOT NULL AND l.created_at > d.deleted_at
  AND (d.tmdb_id = l.tmdb_id OR d.tvdb_id = l.tvdb_id OR d.imdb_id = l.imdb_id)
  AND (d.tmdb_id IS NULL OR l.tmdb_id IS NULL OR d.tmdb_id = l.tmdb_id)
  AND (d.tvdb_id IS NULL OR l.tvdb_id IS NULL OR d.tvdb_id = l.tvdb_id)
  AND (d.imdb_id IS NULL OR l.imdb_id IS NULL OR d.imdb_id = l.imdb_id)
ORDER BY d.deleted_at DESC, d.id DESC
LIMIT 1;

-- name: ListRelinkPairs :many
-- Every live film or series of a library (all libraries with library_id 0) with its soft-deleted
-- twin, as FindDeletedTwin picks it. Two live copies may name the same deleted twin; the first
-- merge revives it, and the second copy then has no deleted twin any more. CROSS JOIN keeps the
-- live titles in the outer loop, so each looks up its twins by provider id (see FindDeletedTwin).
WITH pairs AS (
    SELECT l.id AS live_id, d.id AS deleted_id, l.library_id AS library_id, l.kind AS kind, l.title AS title,
           d.deleted_at AS deleted_at,
           row_number() OVER (PARTITION BY l.id ORDER BY d.deleted_at DESC, d.id DESC) AS rn
    FROM items AS l
    CROSS JOIN items AS d ON +d.library_id = l.library_id AND +d.kind = l.kind AND d.id <> l.id
    WHERE l.deleted_at IS NULL AND l.kind IN ('movie', 'series')
      AND (CAST(@library_id AS INTEGER) = 0 OR l.library_id = @library_id)
      AND +d.deleted_at IS NOT NULL AND l.created_at > d.deleted_at
      AND (d.tmdb_id = l.tmdb_id OR d.tvdb_id = l.tvdb_id OR d.imdb_id = l.imdb_id)
      AND (d.tmdb_id IS NULL OR l.tmdb_id IS NULL OR d.tmdb_id = l.tmdb_id)
      AND (d.tvdb_id IS NULL OR l.tvdb_id IS NULL OR d.tvdb_id = l.tvdb_id)
      AND (d.imdb_id IS NULL OR l.imdb_id IS NULL OR d.imdb_id = l.imdb_id)
)
SELECT live_id, deleted_id, library_id, kind, title, deleted_at FROM pairs
WHERE rn = 1
ORDER BY library_id, title, live_id;

-- name: CountRelinkUserData :one
-- The user data on a title and its descendants (seasons, episodes, extras), per table.
WITH tree AS (
    SELECT t.id FROM items AS t
    WHERE t.id = @id OR t.parent_id = @id
       OR t.parent_id IN (SELECT s.id FROM items AS s WHERE s.parent_id = @id)
)
SELECT
    (SELECT count(*) FROM progress WHERE item_id IN (SELECT id FROM tree))          AS progress,
    (SELECT count(*) FROM progress_events WHERE item_id IN (SELECT id FROM tree))   AS progress_events,
    (SELECT count(*) FROM progress_days WHERE item_id IN (SELECT id FROM tree))     AS progress_days,
    (SELECT count(*) FROM playback_sessions WHERE item_id IN (SELECT id FROM tree)) AS playback_sessions,
    (SELECT count(*) FROM watchlist WHERE item_id IN (SELECT id FROM tree))         AS watchlist,
    (SELECT count(*) FROM favorites WHERE item_id IN (SELECT id FROM tree))         AS favorites,
    (SELECT count(*) FROM collection_items WHERE item_id IN (SELECT id FROM tree))  AS collection_items,
    (SELECT count(*) FROM notifications WHERE item_id IN (SELECT id FROM tree))     AS notifications,
    (SELECT count(*) FROM top_picks WHERE item_id IN (SELECT id FROM tree))         AS top_picks,
    (SELECT count(*) FROM downloads WHERE item_id IN (SELECT id FROM tree))         AS downloads,
    (SELECT count(*) FROM activity_events WHERE item_id IN (SELECT id FROM tree))   AS activity_events;

-- name: ListRelinkChildren :many
-- All children of an item, soft-deleted ones too: live ones first, then the most recently deleted.
SELECT id, kind, season_number, episode_number, deleted_at FROM items
WHERE parent_id = @parent_id
ORDER BY deleted_at IS NOT NULL, deleted_at DESC, id;

-- name: RelinkSetParent :exec
-- A child of the copy without a counterpart in the twin moves below the twin; the triggers
-- recompute its rating and the availability of both parents.
UPDATE items SET parent_id = @parent_id, updated_at = @now WHERE id = @id;

-- name: RelinkMovePlaybackSessions :exec
-- The statistics read the sessions (admin_stats.sql). A running stream of the copy ends: its
-- stream token names the copy's id.
UPDATE playback_sessions SET item_id = @target_id WHERE item_id = @source_id;

-- name: RelinkMoveActivityEvents :exec
-- activity_events has no foreign key on items; the rows follow the title all the same.
UPDATE activity_events SET item_id = @target_id WHERE item_id = @source_id;

-- name: ListRelinkChainTwins :many
-- The other soft-deleted twins of a relinked title (an older generation of the same title), the
-- most recently deleted first: they share at least one of the given provider ids (the survivor's,
-- completed by its copy's) and contradict none. The relink folds them into the survivor, so a
-- later match of the survivor finds no older twin.
SELECT d.id FROM items AS d
WHERE +d.library_id = @library_id AND +d.kind = @kind AND +d.deleted_at IS NOT NULL
  AND (d.tmdb_id = sqlc.narg('tmdb_id') OR d.tvdb_id = sqlc.narg('tvdb_id') OR d.imdb_id = sqlc.narg('imdb_id'))
  AND (d.tmdb_id IS NULL OR sqlc.narg('tmdb_id') IS NULL OR d.tmdb_id = sqlc.narg('tmdb_id'))
  AND (d.tvdb_id IS NULL OR sqlc.narg('tvdb_id') IS NULL OR d.tvdb_id = sqlc.narg('tvdb_id'))
  AND (d.imdb_id IS NULL OR sqlc.narg('imdb_id') IS NULL OR d.imdb_id = sqlc.narg('imdb_id'))
ORDER BY d.deleted_at DESC, d.id DESC;

-- name: RelinkSetCurated :exec
-- The curated state the survivor takes over from its copy (relinker.carry): provider ids and age
-- rating with the locks the copy adds. The rating triggers recompute the effective ratings.
UPDATE items
SET tmdb_id = @tmdb_id, tvdb_id = @tvdb_id, imdb_id = @imdb_id, own_rating = @own_rating,
    rating_source = @rating_source, locked_fields = @locked_fields, updated_at = @now
WHERE id = @id;

-- name: RelinkDropUnchosenImages :exec
-- An image the administrator chose for the copy (chosen = 1) replaces the survivor's automatic
-- choice of the kind.
DELETE FROM item_images
WHERE item_images.item_id = @target_id AND item_images.chosen = 0
  AND item_images.kind IN (SELECT s.kind FROM item_images AS s WHERE s.item_id = @source_id AND s.chosen = 1);

-- name: RelinkDropCachedImages :exec
-- The cached files of the survivor for the kinds whose choice comes from the copy.
DELETE FROM images
WHERE images.item_id = @target_id
  AND images.kind IN (SELECT s.kind FROM item_images AS s WHERE s.item_id = @source_id)
  AND images.kind NOT IN (SELECT t.kind FROM item_images AS t WHERE t.item_id = @target_id);

-- name: RelinkMoveCachedImages :exec
-- The copy's cached files of the kinds whose choice moves with them (run before RelinkMoveImages).
UPDATE OR IGNORE images SET item_id = @target_id
WHERE images.item_id = @source_id
  AND images.kind NOT IN (SELECT t.kind FROM item_images AS t WHERE t.item_id = @target_id);

-- name: RelinkMoveImages :exec
-- The copy's image choice of a kind the survivor has none of.
UPDATE OR IGNORE item_images SET item_id = @target_id WHERE item_id = @source_id;

-- name: RelinkMoveLocalImages :exec
-- The copy's local images (poster.jpg next to its files) of a kind the survivor has none of.
UPDATE item_local_images SET item_id = @target_id
WHERE item_local_images.item_id = @source_id
  AND item_local_images.kind NOT IN (SELECT t.kind FROM item_local_images AS t WHERE t.item_id = @target_id);

-- name: RelinkReplaceChapters :exec
-- The chapters belong to the copy's media file, which moved to the survivor: they replace the
-- survivor's chapters, if the copy has any.
DELETE FROM chapters
WHERE chapters.item_id = @target_id AND EXISTS (SELECT 1 FROM chapters AS s WHERE s.item_id = @source_id);

-- name: RelinkMoveChapters :exec
UPDATE chapters SET item_id = @target_id WHERE item_id = @source_id;

-- name: RelinkCountSegments :one
SELECT count(*) FROM media_segments WHERE item_id = @item_id;

-- name: RelinkDropDetectedSegments :exec
-- The copy's segments were detected on the file that moved: they replace the survivor's detected
-- ones; a manual segment of the survivor stays.
DELETE FROM media_segments WHERE item_id = @target_id AND source <> 'manual';

-- name: RelinkMoveSegments :exec
UPDATE OR IGNORE media_segments SET item_id = @target_id WHERE item_id = @source_id;

-- name: RelinkSeasonSegmentsPending :exec
-- An episode whose file moved in without segments sets its season back to pending, so the next
-- scan queues the detection.
UPDATE items SET segments_state = 'pending', updated_at = @now
WHERE items.id = (SELECT e.parent_id FROM items AS e WHERE e.id = @episode_id AND e.kind = 'episode')
  AND items.kind = 'season';

-- name: RelinkDropDeadDownloads :exec
-- A download of the survivor that cannot be served any more (failed, evicted, or its file is
-- missing) gives way to the copy's download for the same device, which then moves in. The delete
-- trigger leaves a tombstone for the dead package only; the device keeps the copy's package.
DELETE FROM downloads
WHERE downloads.item_id = @target_id
  AND downloads.device_id IN (SELECT s.device_id FROM downloads AS s WHERE s.item_id = @source_id)
  AND (downloads.state IN ('failed', 'evicted')
       OR (downloads.media_file_id IS NOT NULL
           AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.id = downloads.media_file_id AND m.missing_since IS NULL)));
