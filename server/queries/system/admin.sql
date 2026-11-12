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

-- Keyframe index coverage of the libraries (Library.keyframeIndex, concept 14, 4.1, F-7).

-- name: AdminCountKeyframeIndex :many
-- Files worth indexing per library and keyframes_source, one query for every library. "Worth
-- indexing" (concept 14, 4.1; the same condition as in package library): the file is present
-- (missing_since IS NULL), its video can be remuxed (h264, hevc, av1, vp9) and its title is no
-- audiobook and no podcast episode. The media_files part is the condition of the partial index
-- media_files_keyframes (00033), which carries the count (INDEXED BY fails the query when the
-- condition drifts from the index); CROSS JOIN keeps media_files outside, the kind comes through
-- the primary key of items.
SELECT i.library_id, m.keyframes_source, count(*) AS files
FROM media_files AS m INDEXED BY media_files_keyframes
CROSS JOIN items AS i ON i.id = m.item_id
WHERE m.missing_since IS NULL
  AND m.video_codec IN ('h264', 'hevc', 'av1', 'vp9')
  AND i.kind NOT IN ('audiobook', 'podcast_episode')
GROUP BY i.library_id, m.keyframes_source;

-- Maintenance issues (listMaintenanceIssues, B-9). Each kind has a page query and a count; a
-- NULL library_id lists every library. unmatched and unrated list an episode only when its series
-- is matched (rated); otherwise the series stands for all its episodes, so a large series fills
-- the list with one entry instead of one per episode.

-- name: AdminCountUnmatched :one
SELECT count(*) FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series', 'episode')
  AND (i.metadata_state IN ('unmatched', 'failed')
       OR (i.metadata_state = 'skipped' AND i.kind IN ('movie', 'series'))
       OR (i.metadata_state = 'pending' AND i.created_at < @pending_before))
  AND (i.kind <> 'episode' OR EXISTS (
        SELECT 1 FROM items AS sr
        WHERE sr.kind = 'series' AND sr.metadata_state = 'matched'
          AND (sr.id = i.parent_id OR sr.id = (SELECT p.parent_id FROM items AS p WHERE p.id = i.parent_id))))
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'));

-- name: AdminListUnmatched :many
SELECT i.id, i.kind, i.title, i.year, i.library_id, i.metadata_state, i.metadata_error FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series', 'episode')
  AND (i.metadata_state IN ('unmatched', 'failed')
       OR (i.metadata_state = 'skipped' AND i.kind IN ('movie', 'series'))
       OR (i.metadata_state = 'pending' AND i.created_at < @pending_before))
  AND (i.kind <> 'episode' OR EXISTS (
        SELECT 1 FROM items AS sr
        WHERE sr.kind = 'series' AND sr.metadata_state = 'matched'
          AND (sr.id = i.parent_id OR sr.id = (SELECT p.parent_id FROM items AS p WHERE p.id = i.parent_id))))
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
  AND (i.kind <> 'episode' OR EXISTS (
        SELECT 1 FROM items AS sr
        WHERE sr.kind = 'series' AND sr.effective_rating IS NOT NULL
          AND (sr.id = i.parent_id OR sr.id = (SELECT p.parent_id FROM items AS p WHERE p.id = i.parent_id))))
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'));

-- name: AdminListUnrated :many
SELECT i.id, i.kind, i.title, i.year, i.library_id FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series', 'episode') AND i.effective_rating IS NULL
  AND (i.kind <> 'episode' OR EXISTS (
        SELECT 1 FROM items AS sr
        WHERE sr.kind = 'series' AND sr.effective_rating IS NOT NULL
          AND (sr.id = i.parent_id OR sr.id = (SELECT p.parent_id FROM items AS p WHERE p.id = i.parent_id))))
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
-- Media files whose content hash a file of another title shares. Files below 1 MiB (empty files,
-- placeholder trees, samples) are left out: identical small files say nothing about duplicates.
SELECT m.partial_hash, m.id AS media_file_id, m.root_id, m.rel_path, m.size, m.version_label, m.missing_since,
       i.id, i.kind, i.title, i.year, i.library_id
FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
WHERE m.partial_hash IS NOT NULL AND m.size >= 1048576 AND i.deleted_at IS NULL
  AND (sqlc.narg('library_id') IS NULL OR i.library_id = sqlc.narg('library_id'))
  AND EXISTS (SELECT 1 FROM media_files AS o JOIN items AS oi ON oi.id = o.item_id
              WHERE o.partial_hash = m.partial_hash AND o.size >= 1048576 AND o.item_id <> m.item_id AND oi.deleted_at IS NULL)
ORDER BY m.partial_hash, i.id, m.id;

-- Merge (mergeItems, B-9): one transaction per source moves everything of the source to the
-- target, then deletes the source. The triggers recompute availability and ratings on the way.

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
-- The playback rate goes with the winning row; a winner without one keeps the target's.
INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, playback_rate, updated_at)
SELECT p.profile_id, @target_id, p.position_ms, p.duration_ms, p.completed, p.playback_rate, p.updated_at
FROM progress AS p WHERE p.item_id = @source_id AND true
ON CONFLICT (profile_id, item_id) DO UPDATE
SET position_ms = excluded.position_ms, duration_ms = excluded.duration_ms, completed = excluded.completed,
    playback_rate = coalesce(excluded.playback_rate, progress.playback_rate),
    updated_at = max(progress.updated_at, excluded.updated_at)
WHERE excluded.completed > progress.completed
   OR (excluded.completed = progress.completed AND excluded.position_ms > progress.position_ms);

-- name: AdminMoveProgressEvents :exec
UPDATE progress_events SET item_id = @target_id WHERE item_id = @source_id;

-- name: AdminMergeProgressDays :exec
-- The condensed days of the source (M10, progress_days) are added to the target's days; a day the
-- target already has keeps its row with the played time and the plays of both. The source's rows
-- go with the source (ON DELETE CASCADE).
INSERT INTO progress_days (profile_id, item_id, device_id, day, played_ms, plays)
SELECT d.profile_id, @target_id, d.device_id, d.day, d.played_ms, d.plays
FROM progress_days AS d WHERE d.item_id = @source_id AND true
ON CONFLICT (profile_id, item_id, device_id, day) DO UPDATE
SET played_ms = progress_days.played_ms + excluded.played_ms,
    plays = progress_days.plays + excluded.plays;

-- name: AdminMoveUserPicks :exec
-- A profile that recommended both titles keeps the newer time on the target; a plain UPDATE would
-- fail on the primary key. The caller deletes the source's rows afterwards (AdminDeleteUserPicksOfItem).
INSERT INTO user_picks (item_id, profile_id, created_at)
SELECT @target_id, s.profile_id, s.created_at FROM user_picks AS s WHERE s.item_id = @source_id
ON CONFLICT (item_id, profile_id) DO UPDATE SET created_at = max(user_picks.created_at, excluded.created_at);

-- name: AdminMoveWatchlist :exec
-- A profile that has both titles on its watchlist keeps the target's entry; the source's one goes
-- with the source (ON DELETE CASCADE). The same holds for notifications.
UPDATE OR IGNORE watchlist SET item_id = @target_id WHERE item_id = @source_id;

-- name: AdminMoveCollectionItems :exec
-- Inserted (not updated) so the search index triggers of collection_items run for the target; the
-- source's rows go with the source.
INSERT OR IGNORE INTO collection_items (collection_id, item_id, position, created_at)
SELECT ci.collection_id, @target_id, ci.position, ci.created_at FROM collection_items AS ci WHERE ci.item_id = @source_id;

-- name: AdminMoveNotifications :exec
UPDATE OR IGNORE notifications SET item_id = @target_id WHERE item_id = @source_id;

-- name: AdminMoveNotificationParents :exec
UPDATE notifications SET parent_id = @target_id WHERE parent_id = @source_id;

-- name: AdminMoveDownloads :exec
-- Downloads follow their file to the target. A device that holds the target already keeps that
-- download; the source's one goes with the source (its tombstone tells the device to delete it).
UPDATE OR IGNORE downloads SET item_id = @target_id, updated_at = @now WHERE item_id = @source_id;

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

-- Stream overview (W-6, M4): titles and durations of running playback sessions. The overview is
-- unfiltered (owner only) and names titles in plain text, never as ItemRef.

-- name: AdminStreamItems :many
-- Title (German text, else English, else the first language by rank, else the parsed title; ADR 0018)
-- with, for an episode, the series id and title and the season and episode numbers; the owner of a
-- podcast episode (its podcast) and of an extra (its movie, else its series) with the owner's title;
-- the title's own duration (the book sum of an audiobook, the feed's of a podcast episode), the MIME
-- type of an enclosure and the number of present media files (an audiobook's parts; concept 18, 17.4).
SELECT i.id, i.kind, COALESCE(td.title, te.title, (SELECT tr.title FROM item_texts AS tr WHERE tr.item_id = i.id ORDER BY tr.rank, tr.lang LIMIT 1), i.title) AS title, i.season_number, i.episode_number, i.episode_end,
       p.season_number AS parent_season_number, COALESCE(sd.title, se.title, (SELECT tr.title FROM item_texts AS tr WHERE tr.item_id = s.id ORDER BY tr.rank, tr.lang LIMIT 1), s.title, '') AS series_title,
       s.id AS series_id,
       o.id AS owner_id, COALESCE(od.title, oe.title, (SELECT tr.title FROM item_texts AS tr WHERE tr.item_id = o.id ORDER BY tr.rank, tr.lang LIMIT 1), o.title, '') AS owner_title,
       i.duration_ms, i.enclosure_type,
       (SELECT COUNT(*) FROM media_files AS m WHERE m.item_id = i.id AND m.missing_since IS NULL AND m.probe_error IS NULL) AS part_count
FROM items AS i
LEFT JOIN item_texts AS td ON td.item_id = i.id AND td.lang = 'de'
LEFT JOIN item_texts AS te ON te.item_id = i.id AND te.lang = 'en'
LEFT JOIN items AS p ON p.id = i.parent_id
LEFT JOIN items AS s ON s.kind = 'series' AND (s.id = p.parent_id OR s.id = p.id)
LEFT JOIN item_texts AS sd ON sd.item_id = s.id AND sd.lang = 'de'
LEFT JOIN item_texts AS se ON se.item_id = s.id AND se.lang = 'en'
LEFT JOIN items AS o ON o.id = CASE i.kind WHEN 'podcast_episode' THEN i.parent_id WHEN 'extra' THEN COALESCE(s.id, p.id) END
LEFT JOIN item_texts AS od ON od.item_id = o.id AND od.lang = 'de'
LEFT JOIN item_texts AS oe ON oe.item_id = o.id AND oe.lang = 'en'
WHERE i.id IN (sqlc.slice('item_ids'));

-- name: AdminStreamChapters :many
-- The chapters of the titles of running streams (an audiobook's chapter at the position, 17.4).
SELECT item_id, title, start_ms, end_ms FROM chapters WHERE item_id IN (sqlc.slice('item_ids')) ORDER BY item_id, start_ms, idx;

-- name: AdminMediaFileDurations :many
-- Duration, audio codec and the probed overall bit rate (bit/s, the format's; 0 unknown) of media files.
SELECT id, duration_ms, audio_codec, CAST(COALESCE(json_extract(probe, '$.format.bitRate'), 0) AS INTEGER) AS bit_rate
FROM media_files WHERE id IN (sqlc.slice('ids'));

-- Library settings (updateLibrary, M6 AN-2).

-- name: AdminApplyLibraryRating :execrows
-- A new default rating of an audio library reaches every live title whose rating still comes from
-- the library, and raises a manual rating below it to it (AN-2: a manual rating is only stricter);
-- feed and tag ratings (18) stay. The triggers pass it on to the descendants.
UPDATE items SET own_rating = @own_rating, updated_at = @now
WHERE library_id = @library_id AND deleted_at IS NULL
  AND (rating_source = 'library' OR (rating_source = 'manual' AND own_rating < @own_rating));

-- User picks (concept 18 part B, 14.7, ADR 0028). Administration view: unfiltered, titles in plain
-- text, never a profile name. One row per title with a recommendation younger than the retention
-- at @now: the number of those recommendations and the newest of them, newest first.

-- name: AdminListUserPicks :many
SELECT u.item_id, i.kind,
       COALESCE(td.title, te.title, (SELECT tr.title FROM item_texts AS tr WHERE tr.item_id = i.id ORDER BY tr.rank, tr.lang LIMIT 1), i.title) AS title,
       i.year, count(*) AS pick_count, CAST(max(u.created_at) AS INTEGER) AS picked_at
FROM user_picks AS u
JOIN picks_config AS c ON c.id = 1
JOIN items AS i ON i.id = u.item_id
LEFT JOIN item_texts AS td ON td.item_id = i.id AND td.lang = 'de'
LEFT JOIN item_texts AS te ON te.item_id = i.id AND te.lang = 'en'
WHERE u.created_at > @now - c.retention_days * 86400000 AND i.deleted_at IS NULL
GROUP BY u.item_id
ORDER BY picked_at DESC, u.item_id;

-- name: AdminDeleteUserPicksOfItem :execrows
-- Every recommendation of a title (removeAdminUserPick), and the rows a merge or a relink left on
-- its source after AdminMoveUserPicks.
DELETE FROM user_picks WHERE item_id = @item_id;

-- name: AdminGetPicksConfig :one
SELECT retention_days, updated_at FROM picks_config WHERE id = 1;

-- name: AdminSetPicksConfig :exec
UPDATE picks_config SET retention_days = @retention_days, updated_at = @now WHERE id = 1;

-- name: AdminOwnerProfileIDs :many
-- Profiles of the owner's account: the only targets of a pick (E-2). The owner badge, not the
-- role: with a second administrator, role = 'admin' would put picks in front of their children
-- as well (ADR 0014).
SELECT p.id FROM profiles AS p JOIN accounts AS a ON a.id = p.account_id WHERE a.is_owner = 1 ORDER BY p.id;

-- Images of the administration (getAdminItemImage, M11 step 5b): system-wide, without a profile.
-- Like the scoped GetItemImage, a deleted title has no image and the kept original is never served.

-- name: AdminGetItemImage :one
-- One stored image of a title: WebP when asked and stored, else the fallback of the kind (JPEG;
-- PNG for a logo).
SELECT im.rel_path, im.media_type, im.content_hash, im.created_at
FROM images AS im
JOIN items AS i ON i.id = im.item_id AND i.deleted_at IS NULL
WHERE im.item_id = @item_id AND im.kind = @kind AND im.size = @size AND im.size <> 'original'
  AND im.format IN (@format, @fallback)
ORDER BY im.format = 'webp' DESC
LIMIT 1;

-- name: AdminGetWidestItemImage :one
-- The widest stored size of a kind, for a size the pipeline did not derive because the original is
-- narrower (decision of 11 November 2026), with the format rule of AdminGetItemImage.
SELECT im.rel_path, im.media_type, im.content_hash, im.created_at
FROM images AS im
JOIN items AS i ON i.id = im.item_id AND i.deleted_at IS NULL
WHERE im.item_id = @item_id AND im.kind = @kind AND im.size <> 'original'
  AND im.format IN (@format, @fallback)
ORDER BY im.width DESC, im.format = 'webp' DESC
LIMIT 1;

-- name: AdminListImages :many
-- The image of one kind (poster, backdrop) in one size of each title (one row per title, the JPEG
-- row: the URL only needs the content hash, the handler picks the format by Accept); titles without
-- that image are absent. @size and @kind come before the slice: sqlc numbers them after it
-- otherwise, which the expanded slice takes.
SELECT im.item_id, im.content_hash
FROM images AS im
JOIN items AS i ON i.id = im.item_id AND i.deleted_at IS NULL
WHERE im.size = @size AND im.kind = @kind AND im.format = 'jpeg' AND im.item_id IN (sqlc.slice('item_ids'));

-- name: AdminListFileAndFeedEpisodes :many
-- Podcast episodes of one podcast that may be the same episode twice (ADR 0015, concept 4.5): a
-- file episode (no enclosure) and a feed episode (no file) with the same publication minute or
-- the same sort title. The scanner pairs only unambiguous cases; the rest is listed here.
SELECT fe.id AS file_id, fe.title AS file_title, fe.year AS file_year,
       ne.id AS feed_id, ne.title AS feed_title, ne.year AS feed_year, fe.library_id, fe.parent_id
FROM items AS fe
JOIN items AS ne ON ne.parent_id = fe.parent_id AND ne.kind = 'podcast_episode' AND ne.deleted_at IS NULL
     AND ne.enclosure_url IS NOT NULL AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = ne.id)
WHERE fe.kind = 'podcast_episode' AND fe.deleted_at IS NULL AND fe.enclosure_url IS NULL
  AND (sqlc.narg('library_id') IS NULL OR fe.library_id = sqlc.narg('library_id'))
  AND ((fe.published_at IS NOT NULL AND ne.published_at IS NOT NULL AND fe.published_at / 60000 = ne.published_at / 60000)
       OR fe.sort_title = ne.sort_title)
ORDER BY fe.parent_id, fe.id, ne.id;

-- name: AdminTrickplayDurations :many
-- The running time of the films and episodes of a library for the storage estimate of trickplay
-- (concept 18, 9): per kind the titles with a present media file, the present files with a
-- duration (every version counts) and their running time together.
SELECT i.kind, count(DISTINCT i.id) AS titles, count(m.duration_ms) AS files,
       CAST(coalesce(sum(m.duration_ms), 0) AS INTEGER) AS duration_ms
FROM media_files AS m
JOIN items AS i ON i.id = m.item_id
WHERE i.library_id = @library_id AND i.kind IN ('movie', 'episode') AND i.deleted_at IS NULL
  AND m.missing_since IS NULL
GROUP BY i.kind;
