-- Relinking of soft-deleted titles (ADR 0023, idea 04). Owner: library package. A live film or
-- series and a soft-deleted one are twins when they share the library, the kind and at least one
-- provider id (TMDB, TVDB, IMDb), and no provider id contradicts. The automatic relink after a
-- match only takes a fresh copy: the live title was created at most a grace period before the twin
-- was deleted (library.relinkGrace, decision 21), so two titles that lived side by side for longer
-- are never merged on their own. The maintenance action knows no such rule: the administrator sees
-- both titles and decides. The deleted twin survives with its id; the live copy merges into it.
-- A title taken out of Slouchcat (removed_at, concept 20 section 10.2) is no twin: it stays out
-- until "Wieder aufnehmen", and a live copy beside it stays a title of its own.
-- Times are Unix milliseconds.
--
-- The twin lookups are a UNION of one lookup per provider id, each through its own index
-- (INDEXED BY items_tmdb, items_tvdb, items_imdb), and CROSS JOIN fixes the order of the tables.
-- An OR over the three ids lets the planner choose, and with ANALYZE statistics (a film library
-- has no TVDB ids, so items_tvdb looks useless) it walks items_library_kind or the whole table for
-- every title: seconds instead of milliseconds inside the transaction of a folder removal
-- (TestRelinkLookupsUseProviderIndexes).

-- name: FindDeletedTwin :one
-- The soft-deleted twin of a live film or series; among several the most recently deleted wins.
-- With grace_ms the live title must have been created after the twin's deletion less grace_ms
-- (the automatic relink); without it any age counts (the maintenance action). No limit on how long
-- ago the twin was deleted (ADR 0023, point 2).
WITH live AS (
    SELECT i.id, i.library_id, i.kind, i.created_at, i.tmdb_id, i.tvdb_id, i.imdb_id FROM items AS i
    WHERE i.id = @live_id AND i.deleted_at IS NULL AND i.kind IN ('movie', 'series')
),
hits AS (
    SELECT d.id FROM live AS l CROSS JOIN items AS d INDEXED BY items_tmdb ON d.tmdb_id = l.tmdb_id WHERE d.deleted_at IS NOT NULL
    UNION
    SELECT d.id FROM live AS l CROSS JOIN items AS d INDEXED BY items_tvdb ON d.tvdb_id = l.tvdb_id WHERE d.deleted_at IS NOT NULL
    UNION
    SELECT d.id FROM live AS l CROSS JOIN items AS d INDEXED BY items_imdb ON d.imdb_id = l.imdb_id WHERE d.deleted_at IS NOT NULL
)
SELECT d.id FROM live AS l
CROSS JOIN hits AS h
CROSS JOIN items AS d ON d.id = h.id
WHERE d.library_id = l.library_id AND d.kind = l.kind AND d.id <> l.id AND d.removed_at IS NULL
  AND (CAST(sqlc.narg('grace_ms') AS INTEGER) IS NULL OR l.created_at > d.deleted_at - CAST(sqlc.narg('grace_ms') AS INTEGER))
  AND (d.tmdb_id IS NULL OR l.tmdb_id IS NULL OR d.tmdb_id = l.tmdb_id)
  AND (d.tvdb_id IS NULL OR l.tvdb_id IS NULL OR d.tvdb_id = l.tvdb_id)
  AND (d.imdb_id IS NULL OR l.imdb_id IS NULL OR d.imdb_id = l.imdb_id)
ORDER BY d.deleted_at DESC, d.id DESC
LIMIT 1;

-- name: ListRelinkPairs :many
-- Every live film or series of a library (all libraries with library_id 0) with its soft-deleted
-- twin, as FindDeletedTwin picks it without grace_ms: the maintenance action knows no freshness
-- rule. Two live copies may name the same deleted twin; the first merge revives it, and the second
-- copy then has no deleted twin any more. CROSS JOIN keeps the live titles in the outer loop, so
-- each looks up its twins by provider id.
WITH live AS (
    SELECT i.id, i.tmdb_id, i.tvdb_id, i.imdb_id FROM items AS i
    WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series')
      AND (CAST(@library_id AS INTEGER) = 0 OR i.library_id = @library_id)
),
hits AS (
    SELECT l.id AS live_id, d.id AS deleted_id FROM live AS l
    CROSS JOIN items AS d INDEXED BY items_tmdb ON d.tmdb_id = l.tmdb_id WHERE d.deleted_at IS NOT NULL
    UNION
    SELECT l.id, d.id FROM live AS l
    CROSS JOIN items AS d INDEXED BY items_tvdb ON d.tvdb_id = l.tvdb_id WHERE d.deleted_at IS NOT NULL
    UNION
    SELECT l.id, d.id FROM live AS l
    CROSS JOIN items AS d INDEXED BY items_imdb ON d.imdb_id = l.imdb_id WHERE d.deleted_at IS NOT NULL
),
pairs AS (
    SELECT l.id AS live_id, d.id AS deleted_id, l.library_id AS library_id, l.kind AS kind, l.title AS title,
           l.year AS year, d.title AS deleted_title, d.year AS deleted_year, d.deleted_at AS deleted_at,
           row_number() OVER (PARTITION BY l.id ORDER BY d.deleted_at DESC, d.id DESC) AS rn
    FROM hits AS h
    CROSS JOIN items AS l ON l.id = h.live_id
    CROSS JOIN items AS d ON d.id = h.deleted_id
    WHERE d.library_id = l.library_id AND d.kind = l.kind AND d.id <> l.id AND d.removed_at IS NULL
      AND (d.tmdb_id IS NULL OR l.tmdb_id IS NULL OR d.tmdb_id = l.tmdb_id)
      AND (d.tvdb_id IS NULL OR l.tvdb_id IS NULL OR d.tvdb_id = l.tvdb_id)
      AND (d.imdb_id IS NULL OR l.imdb_id IS NULL OR d.imdb_id = l.imdb_id)
)
SELECT live_id, deleted_id, library_id, kind, title, year, deleted_title, deleted_year, deleted_at FROM pairs
WHERE rn = 1
ORDER BY library_id, title, live_id;

-- name: CountRelinkUserData :many
-- The user data on titles and their descendants (seasons, episodes, extras), per title and table,
-- in one statement for the listed pairs of the maintenance action. A title without any row is
-- missing from the result.
WITH roots AS (
    SELECT r.id FROM items AS r WHERE r.id IN (sqlc.slice('ids'))
),
tree AS (
    SELECT roots.id AS root, roots.id AS id FROM roots
    UNION ALL
    SELECT c.parent_id AS root, c.id AS id FROM items AS c WHERE c.parent_id IN (SELECT roots.id FROM roots)
    UNION ALL
    SELECT s.parent_id AS root, g.id AS id FROM items AS s JOIN items AS g ON g.parent_id = s.id
    WHERE s.parent_id IN (SELECT roots.id FROM roots)
),
counts AS (
    SELECT t.root AS root, 'progress' AS tbl, count(*) AS n FROM tree AS t JOIN progress AS x ON x.item_id = t.id GROUP BY t.root
    UNION ALL
    SELECT t.root, 'progress_events', count(*) FROM tree AS t JOIN progress_events AS x ON x.item_id = t.id GROUP BY t.root
    UNION ALL
    SELECT t.root, 'progress_days', count(*) FROM tree AS t JOIN progress_days AS x ON x.item_id = t.id GROUP BY t.root
    UNION ALL
    SELECT t.root, 'playback_sessions', count(*) FROM tree AS t JOIN playback_sessions AS x ON x.item_id = t.id GROUP BY t.root
    UNION ALL
    SELECT t.root, 'watchlist', count(*) FROM tree AS t JOIN watchlist AS x ON x.item_id = t.id GROUP BY t.root
    UNION ALL
    SELECT t.root, 'favorites', count(*) FROM tree AS t JOIN favorites AS x ON x.item_id = t.id GROUP BY t.root
    UNION ALL
    SELECT t.root, 'collection_items', count(*) FROM tree AS t JOIN collection_items AS x ON x.item_id = t.id GROUP BY t.root
    UNION ALL
    SELECT t.root, 'notifications', count(*) FROM tree AS t JOIN notifications AS x ON x.item_id = t.id GROUP BY t.root
    UNION ALL
    SELECT t.root, 'user_picks', count(*) FROM tree AS t JOIN user_picks AS x ON x.item_id = t.id GROUP BY t.root
    UNION ALL
    SELECT t.root, 'downloads', count(*) FROM tree AS t JOIN downloads AS x ON x.item_id = t.id GROUP BY t.root
    UNION ALL
    SELECT t.root, 'activity_events', count(*) FROM tree AS t JOIN activity_events AS x ON x.item_id = t.id GROUP BY t.root
)
SELECT CAST(counts.root AS TEXT) AS id,
    CAST(sum(CASE counts.tbl WHEN 'progress' THEN counts.n ELSE 0 END) AS INTEGER)          AS progress,
    CAST(sum(CASE counts.tbl WHEN 'progress_events' THEN counts.n ELSE 0 END) AS INTEGER)   AS progress_events,
    CAST(sum(CASE counts.tbl WHEN 'progress_days' THEN counts.n ELSE 0 END) AS INTEGER)     AS progress_days,
    CAST(sum(CASE counts.tbl WHEN 'playback_sessions' THEN counts.n ELSE 0 END) AS INTEGER) AS playback_sessions,
    CAST(sum(CASE counts.tbl WHEN 'watchlist' THEN counts.n ELSE 0 END) AS INTEGER)         AS watchlist,
    CAST(sum(CASE counts.tbl WHEN 'favorites' THEN counts.n ELSE 0 END) AS INTEGER)         AS favorites,
    CAST(sum(CASE counts.tbl WHEN 'collection_items' THEN counts.n ELSE 0 END) AS INTEGER)  AS collection_items,
    CAST(sum(CASE counts.tbl WHEN 'notifications' THEN counts.n ELSE 0 END) AS INTEGER)     AS notifications,
    CAST(sum(CASE counts.tbl WHEN 'user_picks' THEN counts.n ELSE 0 END) AS INTEGER)        AS user_picks,
    CAST(sum(CASE counts.tbl WHEN 'downloads' THEN counts.n ELSE 0 END) AS INTEGER)         AS downloads,
    CAST(sum(CASE counts.tbl WHEN 'activity_events' THEN counts.n ELSE 0 END) AS INTEGER)   AS activity_events
FROM counts
GROUP BY counts.root;

-- name: ListRelinkChildren :many
-- All children of an item, soft-deleted ones too: live ones first, then the most recently deleted.
SELECT id, kind, season_number, episode_number, deleted_at FROM items
WHERE parent_id = @parent_id
ORDER BY deleted_at IS NOT NULL, deleted_at DESC, id DESC;

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
WITH hits AS (
    SELECT d.id FROM items AS d INDEXED BY items_tmdb WHERE d.tmdb_id = sqlc.narg('tmdb_id') AND d.deleted_at IS NOT NULL
    UNION
    SELECT d.id FROM items AS d INDEXED BY items_tvdb WHERE d.tvdb_id = sqlc.narg('tvdb_id') AND d.deleted_at IS NOT NULL
    UNION
    SELECT d.id FROM items AS d INDEXED BY items_imdb WHERE d.imdb_id = sqlc.narg('imdb_id') AND d.deleted_at IS NOT NULL
)
SELECT d.id FROM hits AS h
CROSS JOIN items AS d ON d.id = h.id
WHERE d.library_id = @library_id AND d.kind = @kind AND d.removed_at IS NULL
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

-- name: ListFreshCopiesOfDeleted :many
-- The live films and series that became twins of a title soft-deleted at now in the caller's
-- transaction (a folder removed or changed, the sweep, the cleanup): fresh copies created at most
-- grace_ms before the deletion, sharing a provider id and contradicting none (FindDeletedTwin with
-- grace_ms). Each gets library.relink, so the order "add the new folder, scan, remove the old one"
-- relinks without another match. CROSS JOIN fixes the order: the libraries, then their titles
-- deleted at now through items_library_kind (library, kind, deleted_at), then each one's copies by
-- provider id, so no statement walks every title.
WITH deleted AS (
    SELECT d.id, d.tmdb_id, d.tvdb_id, d.imdb_id FROM libraries AS lib
    CROSS JOIN items AS d ON d.library_id = lib.id AND d.kind IN ('movie', 'series') AND d.deleted_at = @now
    WHERE d.removed_at IS NULL
),
hits AS (
    SELECT d.id AS deleted_id, l.id AS live_id FROM deleted AS d
    CROSS JOIN items AS l INDEXED BY items_tmdb ON l.tmdb_id = d.tmdb_id WHERE l.deleted_at IS NULL
    UNION
    SELECT d.id, l.id FROM deleted AS d
    CROSS JOIN items AS l INDEXED BY items_tvdb ON l.tvdb_id = d.tvdb_id WHERE l.deleted_at IS NULL
    UNION
    SELECT d.id, l.id FROM deleted AS d
    CROSS JOIN items AS l INDEXED BY items_imdb ON l.imdb_id = d.imdb_id WHERE l.deleted_at IS NULL
)
SELECT DISTINCT l.id FROM hits AS h
CROSS JOIN items AS d ON d.id = h.deleted_id
CROSS JOIN items AS l ON l.id = h.live_id
WHERE l.library_id = d.library_id AND l.kind = d.kind AND l.id <> d.id
  AND l.created_at > d.deleted_at - CAST(@grace_ms AS INTEGER)
  AND (d.tmdb_id IS NULL OR l.tmdb_id IS NULL OR d.tmdb_id = l.tmdb_id)
  AND (d.tvdb_id IS NULL OR l.tvdb_id IS NULL OR d.tvdb_id = l.tvdb_id)
  AND (d.imdb_id IS NULL OR l.imdb_id IS NULL OR d.imdb_id = l.imdb_id)
ORDER BY l.id;
