-- The download progress from the queue of Radarr and Sonarr (concept 25 step 4.6d, data model
-- 2.16; package wishes only, store/wishq). Times are Unix milliseconds. In procurement means the
-- states processing and partial.

-- name: CountProcurementWishes :one
SELECT count(*) FROM wishes WHERE state IN ('processing', 'partial');

-- name: ListProcurementWishes :many
-- The wishes in procurement with their last progress. The title id may also come from the tag rows
-- (the same id, data model 2.16).
SELECT w.id, w.media, w.tmdb_id, w.tvdb_id,
       CAST(COALESCE(p.arr_title_id,
                     (SELECT t.arr_title_id FROM wish_tags AS t WHERE t.wish_id = w.id AND t.arr_title_id IS NOT NULL
                      ORDER BY t.profile_id LIMIT 1), 0) AS INTEGER) AS arr_title_id,
       p.has_file, p.episodes_done, p.episodes_total, p.title_read_at
FROM wishes AS w
LEFT JOIN wish_progress AS p ON p.wish_id = w.id
WHERE w.state IN ('processing', 'partial')
ORDER BY w.id;

-- name: UpsertWishProgress :exec
INSERT INTO wish_progress (wish_id, service, arr_title_id, stage, percent, eta_seconds, episodes_done, episodes_total,
                           admin_message, error, has_file, title_read_at, read_at)
VALUES (@wish_id, @service, sqlc.narg('arr_title_id'), sqlc.narg('stage'), sqlc.narg('percent'), sqlc.narg('eta_seconds'),
        sqlc.narg('episodes_done'), sqlc.narg('episodes_total'), sqlc.narg('admin_message'), sqlc.narg('error'),
        sqlc.narg('has_file'), sqlc.narg('title_read_at'), @read_at)
ON CONFLICT (wish_id) DO UPDATE SET
    service = excluded.service, arr_title_id = excluded.arr_title_id, stage = excluded.stage, percent = excluded.percent,
    eta_seconds = excluded.eta_seconds, episodes_done = excluded.episodes_done, episodes_total = excluded.episodes_total,
    admin_message = excluded.admin_message, error = excluded.error, has_file = excluded.has_file,
    title_read_at = excluded.title_read_at, read_at = excluded.read_at;

-- name: SetWishProgressUnreadable :exec
-- The service could not be read: the last values and read_at stay, so the administration shows
-- the time of the last read; a new row has nothing to show.
INSERT INTO wish_progress (wish_id, service, error, read_at) VALUES (@wish_id, @service, 'unreachable', @now)
ON CONFLICT (wish_id) DO UPDATE SET error = 'unreachable';

-- name: DeleteAllWishProgress :exec
-- The switch is off.
DELETE FROM wish_progress;

-- name: DeleteClosedWishProgress :exec
-- Rows of wishes that left procurement (the trigger covers state changes; this covers the rest).
DELETE FROM wish_progress
WHERE wish_id NOT IN (SELECT id FROM wishes WHERE state IN ('processing', 'partial'));

-- name: ListWishProgressOf :many
-- The progress of some wishes in procurement, for the pages.
SELECT p.* FROM wish_progress AS p
JOIN wishes AS w ON w.id = p.wish_id
WHERE p.wish_id IN (sqlc.slice('wish_ids')) AND w.state IN ('processing', 'partial')
ORDER BY p.wish_id;

-- name: ProgressByService :many
-- The status per service: rows, rows with something to show, rows the service could not read and
-- the last read.
SELECT service, count(*) AS n,
       CAST(COALESCE(sum(CASE WHEN stage IS NOT NULL AND COALESCE(error, '') <> 'unreachable' THEN 1 ELSE 0 END), 0) AS INTEGER) AS tracked,
       CAST(COALESCE(sum(CASE WHEN error = 'unreachable' THEN 1 ELSE 0 END), 0) AS INTEGER) AS unreachable,
       CAST(COALESCE(max(read_at), 0) AS INTEGER) AS last_read_at
FROM wish_progress GROUP BY service ORDER BY service;
