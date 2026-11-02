-- Activity log of the admin overview (concept 11 section 11.4, ADR 0019, migration 00031). Written
-- by admin.RecordActivity inside one write transaction (flood rule, owner check, insert), read by
-- getDashboard, cleaned up by the hook admin registers in jobs.prune.

-- name: InsertActivityEvent :one
INSERT INTO activity_events (at, kind, account_id, profile_id, item_id, library_id, subject, detail)
VALUES (@at, @kind, sqlc.narg('account_id'), sqlc.narg('profile_id'), sqlc.narg('item_id'), sqlc.narg('library_id'),
        sqlc.narg('subject'), @detail)
RETURNING id;

-- name: CountActivityProfile :one
-- Whether the profile of an event still exists: an event that arrives after its profile or
-- account was deleted (a start written in the background) is dropped, not stored.
SELECT count(*) FROM profiles WHERE id = @id;

-- name: CountActivityAccount :one
SELECT count(*) FROM accounts WHERE id = @id;

-- name: CountRecentPlaybackStarts :one
-- Flood rule of playback_started: the same profile started the same title since @since.
SELECT count(*) FROM activity_events
WHERE kind = 'playback_started' AND at >= @since AND profile_id = @profile_id AND item_id = @item_id;

-- name: CountRecentLibraryActivity :one
-- Flood rule of scan_failed and scan_aborted: an event of the kind and the reason for the library
-- since @since (a manual cancellation does not hide a real abort within the hour).
SELECT count(*) FROM activity_events
WHERE kind = @kind AND at >= @since AND library_id = @library_id
  AND json_extract(detail, '$.reason') = CAST(@reason AS TEXT);

-- name: BumpJobFailedActivity :execrows
-- Flood rule of job_failed: the row of the job type in the current hour (at >= @since) counts one
-- failure more and moves to the time of the newest one (never back, should two arrive out of order).
UPDATE activity_events
SET at = MAX(at, @at), detail = json_set(detail, '$.count', COALESCE(json_extract(detail, '$.count'), 1) + 1)
WHERE id = (
    SELECT a.id FROM activity_events AS a
    WHERE a.kind = 'job_failed' AND a.at >= @since AND json_extract(a.detail, '$.jobType') = CAST(@job_type AS TEXT)
    ORDER BY a.at DESC, a.id DESC
    LIMIT 1);

-- name: ActivityItemTitle :one
-- The title of an event's item at the time of the event (subject); the item may be gone already.
SELECT title FROM items WHERE id = @id;

-- name: ListActivity :many
-- The newest events with the e-mail address of the account and the name of the profile, read now
-- (they are not stored in the row).
SELECT a.id, a.at, a.kind, a.item_id, a.library_id, a.subject, a.detail,
       acc.email AS account_email, p.name AS profile_name
FROM activity_events AS a
LEFT JOIN accounts AS acc ON acc.id = a.account_id
LEFT JOIN profiles AS p ON p.id = a.profile_id
ORDER BY a.at DESC, a.id DESC
LIMIT @limit_count;

-- name: PruneActivity :execrows
-- One batch of the cleanup in jobs.prune: events older than @before and every event beyond the
-- newest @keep.
DELETE FROM activity_events WHERE id IN (
    SELECT a.id FROM activity_events AS a
    WHERE a.at < @before
       OR a.id NOT IN (SELECT k.id FROM activity_events AS k ORDER BY k.at DESC, k.id DESC LIMIT @keep)
    ORDER BY a.id
    LIMIT @batch
);
