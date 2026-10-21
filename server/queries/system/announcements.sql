-- Announcements of the main user (M8, A-7, plan 5.2; package notify). Times are Unix milliseconds.
-- published_at NULL is a draft; a profile acknowledges a published announcement once.

-- name: AnnouncementInsert :one
INSERT INTO announcements (title, body, created_by, created_at, updated_at, published_at)
VALUES (@title, @body, sqlc.narg('created_by'), @now, @now, sqlc.narg('published_at'))
RETURNING id;

-- name: AnnouncementGet :one
-- One announcement with the creator's e-mail, the acknowledgements and the number of profiles.
SELECT n.id, n.title, n.body, n.created_at, n.updated_at, n.published_at,
       CAST(COALESCE(a.email, '') AS TEXT) AS created_by_email,
       (SELECT count(*) FROM announcement_acks AS k WHERE k.announcement_id = n.id) AS acknowledged_count,
       (SELECT count(*) FROM profiles) AS profile_count
FROM announcements AS n
LEFT JOIN accounts AS a ON a.id = n.created_by
WHERE n.id = @id;

-- name: AnnouncementList :many
-- Every announcement, newest first, with the same counts as AnnouncementGet.
SELECT n.id, n.title, n.body, n.created_at, n.updated_at, n.published_at,
       CAST(COALESCE(a.email, '') AS TEXT) AS created_by_email,
       (SELECT count(*) FROM announcement_acks AS k WHERE k.announcement_id = n.id) AS acknowledged_count,
       (SELECT count(*) FROM profiles) AS profile_count
FROM announcements AS n
LEFT JOIN accounts AS a ON a.id = n.created_by
ORDER BY n.created_at DESC, n.id DESC;

-- name: AnnouncementUpdate :execrows
-- Changes a draft; 0 rows for an unknown or a published announcement.
UPDATE announcements SET title = @title, body = @body, updated_at = @now
WHERE id = @id AND published_at IS NULL;

-- name: AnnouncementPublish :execrows
-- Publishes a draft; 0 rows for an unknown or an already published announcement.
UPDATE announcements SET published_at = @now, updated_at = @now
WHERE id = @id AND published_at IS NULL;

-- name: AnnouncementDelete :execrows
DELETE FROM announcements WHERE id = @id;

-- name: AnnouncementsOpenForProfile :many
-- Published announcements the profile has not acknowledged, newest first.
SELECT n.id, n.title, n.body, n.published_at
FROM announcements AS n
WHERE n.published_at IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM announcement_acks AS k WHERE k.announcement_id = n.id AND k.profile_id = @profile_id)
ORDER BY n.published_at DESC, n.id DESC
LIMIT @limit_count;

-- name: AnnouncementAck :execrows
-- Acknowledges a published announcement for the profile (idempotent); 0 rows for an unknown or
-- unpublished announcement and for a second acknowledgement.
INSERT OR IGNORE INTO announcement_acks (announcement_id, profile_id, acked_at)
SELECT n.id, @profile_id, @now FROM announcements AS n WHERE n.id = @id AND n.published_at IS NOT NULL;

-- name: AnnouncementIsPublished :one
SELECT EXISTS (SELECT 1 FROM announcements WHERE id = @id AND published_at IS NOT NULL) AS published;
