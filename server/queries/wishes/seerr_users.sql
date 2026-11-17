-- The mirror of the Seerr users (concept 25, data model 2.1; package wishes only, store/wishq).
-- Times are Unix milliseconds. A refresh upserts every user it reads with the same fetched_at and
-- then marks every other row gone; is_shared and the limit state are never touched by the upsert.

-- name: UpsertSeerrUser :exec
INSERT INTO seerr_users (seerr_id, display_name, email, user_type, permissions,
                         movie_quota_limit, movie_quota_days, tv_quota_limit, tv_quota_days,
                         is_admin, created_at, fetched_at, gone_at)
VALUES (@seerr_id, @display_name, @email, @user_type, @permissions,
        @movie_quota_limit, @movie_quota_days, @tv_quota_limit, @tv_quota_days,
        @is_admin, @created_at, @fetched_at, NULL)
ON CONFLICT (seerr_id) DO UPDATE SET
    display_name = excluded.display_name, email = excluded.email, user_type = excluded.user_type,
    permissions = excluded.permissions, movie_quota_limit = excluded.movie_quota_limit,
    movie_quota_days = excluded.movie_quota_days, tv_quota_limit = excluded.tv_quota_limit,
    tv_quota_days = excluded.tv_quota_days, is_admin = excluded.is_admin,
    created_at = excluded.created_at, fetched_at = excluded.fetched_at, gone_at = NULL;

-- name: MarkSeerrUsersGone :execrows
-- Every user the refresh at fetched_at did not read is gone from Seerr (or the row was never read).
UPDATE seerr_users SET gone_at = @now WHERE gone_at IS NULL AND fetched_at < @fetched_at;

-- name: GetSeerrUser :one
SELECT * FROM seerr_users WHERE seerr_id = @seerr_id;

-- name: ListSeerrUsers :many
-- Every known user with the account it belongs to (NULL = none), by id.
SELECT sqlc.embed(u), m.account_id AS mapped_account_id, a.email AS mapped_account_email
FROM seerr_users AS u
LEFT JOIN wish_account_map AS m ON m.seerr_user_id = u.seerr_id
LEFT JOIN accounts AS a ON a.id = m.account_id
ORDER BY u.seerr_id;

-- name: ListSeerrUserCreatedAt :many
-- The creation times of the known users, part of the fingerprint of the instance.
SELECT seerr_id, created_at FROM seerr_users WHERE created_at IS NOT NULL;

-- name: FindFreeSeerrUserByEmail :one
-- A present user with this email (any case) that belongs to no account and is not the shared user
-- (linking by the same email, N-2).
SELECT u.* FROM seerr_users AS u
WHERE u.email = @email AND u.gone_at IS NULL AND u.is_shared = 0
  AND NOT EXISTS (SELECT 1 FROM wish_account_map AS m WHERE m.seerr_user_id = u.seerr_id)
ORDER BY u.seerr_id
LIMIT 1;

-- name: GetSharedSeerrUser :one
SELECT * FROM seerr_users WHERE is_shared = 1;

-- name: ClearSharedSeerrUser :exec
UPDATE seerr_users SET is_shared = 0 WHERE is_shared = 1;

-- name: SetSharedSeerrUser :execrows
-- Marks the user as the shared user "slouchcat-gaeste" (N-101); clear the old one first. The trigger
-- seerr_users_shared_mapped refuses a user that belongs to an account.
UPDATE seerr_users SET is_shared = 1 WHERE seerr_id = @seerr_id;

-- name: SetSeerrUserQuota :exec
-- The limit state of GET /user/{id}/quota (concept 25, architecture 19.3); NULL = unknown.
UPDATE seerr_users SET
    movie_quota_limit = @movie_quota_limit, movie_quota_days = @movie_quota_days,
    tv_quota_limit = @tv_quota_limit, tv_quota_days = @tv_quota_days,
    movie_quota_used = @movie_quota_used, tv_quota_used = @tv_quota_used,
    movie_next_free_at = @movie_next_free_at, tv_next_free_at = @tv_next_free_at, quota_at = @quota_at
WHERE seerr_id = @seerr_id;

-- name: DeleteAllSeerrUsers :exec
-- Resetting the mapping after an instance change: the mirror belongs to the old instance. Run after
-- DeleteAllAccountMappings (foreign key).
DELETE FROM seerr_users;

-- name: SetSeerrUserPermissions :exec
-- The rights Slouchcat just wrote to a user it opened (N-110).
UPDATE seerr_users SET permissions = @permissions,
    is_admin = CASE WHEN (@permissions & 10) <> 0 THEN 1 ELSE 0 END
WHERE seerr_id = @seerr_id;

-- name: SetSeerrUserEmail :exec
-- The email Slouchcat just wrote to a user after an e-mail change of its account (N-95).
UPDATE seerr_users SET email = @email WHERE seerr_id = @seerr_id;

-- name: SetSeerrUserQuotaState :exec
-- What GET /user/{id}/quota says is used, when the next place is free (N-35) and when it was read;
-- the limit fields stay as GET /user holds them.
UPDATE seerr_users SET
    movie_quota_used = @movie_quota_used, tv_quota_used = @tv_quota_used,
    movie_next_free_at = @movie_next_free_at, tv_next_free_at = @tv_next_free_at, quota_at = @quota_at
WHERE seerr_id = @seerr_id;

-- name: ListQuotaSyncUsers :many
-- The users whose limit state a wish sync reads anew (architecture 19.3): a ready user of an
-- account or the shared user, without MANAGE_USERS, with an open wish or never read. The sync adds
-- the users of the requests it saw in its pass, also those made directly in Seerr.
SELECT u.seerr_id,
       CAST(u.quota_at IS NULL OR EXISTS (
           SELECT 1 FROM wishes AS w
           WHERE w.seerr_user_id = u.seerr_id AND w.state IN ('queued', 'pending', 'processing', 'partial')
       ) AS INTEGER) AS due
FROM seerr_users AS u
WHERE u.is_admin = 0 AND u.gone_at IS NULL
  AND (u.is_shared = 1 OR EXISTS (
      SELECT 1 FROM wish_account_map AS m WHERE m.seerr_user_id = u.seerr_id AND m.state = 'ready'
  ))
ORDER BY u.seerr_id;
