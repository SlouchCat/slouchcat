-- Devices, PIN locks, profile and account management, invitations (package store/appq). Owner:
-- auth (all of it) and admin (accounts, invitations). Never touches title tables. Times are Unix
-- milliseconds.

-- name: UpsertDevice :exec
INSERT INTO devices (id, user_agent, created_at, last_seen_at) VALUES (@id, @user_agent, @now, @now)
ON CONFLICT (id) DO UPDATE SET last_seen_at = excluded.last_seen_at;

-- name: GetPinAttempt :one
SELECT * FROM pin_attempts WHERE profile_id = @profile_id AND device_id = @device_id;

-- name: UpsertPinAttempt :exec
INSERT INTO pin_attempts (profile_id, device_id, failures, locked_until, updated_at)
VALUES (@profile_id, @device_id, @failures, @locked_until, @now)
ON CONFLICT (profile_id, device_id) DO UPDATE
SET failures = excluded.failures, locked_until = excluded.locked_until, updated_at = excluded.updated_at;

-- name: DeletePinAttempt :exec
DELETE FROM pin_attempts WHERE profile_id = @profile_id AND device_id = @device_id;

-- name: DeletePinAttemptsByProfile :exec
DELETE FROM pin_attempts WHERE profile_id = @profile_id;

-- name: ListProfilesByAccount :many
SELECT * FROM profiles WHERE account_id = @account_id ORDER BY is_default DESC, id;

-- name: GetProfileOfAccount :one
SELECT * FROM profiles WHERE id = @id AND account_id = @account_id;

-- name: CountProfilesByAccount :one
SELECT count(*) FROM profiles WHERE account_id = @account_id;

-- name: UpdateProfile :exec
UPDATE profiles
SET name = @name, max_rating = @max_rating, language = @language, accent = @accent,
    all_libraries = @all_libraries, max_bitrate_kbps = sqlc.narg('max_bitrate_kbps'), max_height = sqlc.narg('max_height'),
    max_streams = sqlc.narg('max_streams'), watch_limit_minutes = sqlc.narg('watch_limit_minutes'), updated_at = @now
WHERE id = @id;

-- name: SetProfilePin :exec
UPDATE profiles SET pin_hash = @pin_hash, updated_at = @now WHERE id = @id;

-- name: DeleteProfile :exec
DELETE FROM profiles WHERE id = @id;

-- name: ListProfileLibrariesByAccount :many
SELECT pl.profile_id, pl.library_id FROM profile_libraries AS pl
JOIN profiles AS p ON p.id = pl.profile_id
WHERE p.account_id = @account_id
ORDER BY pl.profile_id, pl.library_id;

-- name: ListProfileLibraries :many
SELECT library_id FROM profile_libraries WHERE profile_id = @profile_id ORDER BY library_id;

-- name: DeleteProfileLibraries :exec
DELETE FROM profile_libraries WHERE profile_id = @profile_id;

-- name: InsertProfileLibrary :exec
INSERT INTO profile_libraries (profile_id, library_id) VALUES (@profile_id, @library_id);

-- name: SetSessionProfile :execrows
-- The device of a session is fixed at its start (PIN locks must not depend on a cookie the client
-- can drop); the profile must still exist in the session's account.
UPDATE sessions SET profile_id = @profile_id, last_seen_at = @now
WHERE sessions.id = @id AND sessions.revoked_at IS NULL
  AND EXISTS (SELECT 1 FROM profiles AS p WHERE p.id = @profile_id AND p.account_id = sessions.account_id);

-- name: ClearSessionsProfile :execrows
-- Sessions that have the profile active fall back to the picker (before the profile is deleted).
UPDATE sessions SET profile_id = NULL WHERE profile_id = @profile_id;

-- name: AssignSessionDevice :exec
UPDATE sessions SET device_id = @device_id WHERE id = @id AND device_id IS NULL;

-- name: ClampProfileRatings :execrows
UPDATE profiles SET max_rating = @max_rating, updated_at = @now WHERE account_id = @account_id AND max_rating > @max_rating;

-- name: DeleteStaleDevices :execrows
DELETE FROM devices WHERE last_seen_at < @before;

-- name: DeleteStalePinAttempts :execrows
DELETE FROM pin_attempts WHERE updated_at < @before;

-- name: TouchAccount :exec
UPDATE accounts SET last_seen_at = @now WHERE id = @id;

-- name: InsertInvitedAccount :one
-- An invitation never creates an administrator: the role is a literal in this file, not a
-- parameter, so no request body and no slip in a handler can raise it (ADR 0014, lock 4).
INSERT INTO accounts (email, password_hash, role, max_rating, all_libraries, invited_by, created_at, updated_at)
VALUES (@email, @password_hash, 'user', @max_rating, @all_libraries, sqlc.narg('invited_by'), @now, @now)
RETURNING *;

-- name: GetGuestAccount :one
-- An account that administers nothing; the administration changes its limits and may revoke it.
SELECT * FROM accounts WHERE id = @id AND role = 'user';

-- name: ListGuestAccounts :many
SELECT a.id, a.email, a.max_rating, a.all_libraries, a.created_at, a.last_seen_at,
       a.max_bitrate_kbps, a.max_height, a.max_streams,
       (SELECT count(*) FROM profiles AS p WHERE p.account_id = a.id) AS profile_count
FROM accounts AS a
WHERE a.role = 'user'
ORDER BY a.created_at DESC, a.id DESC;

-- name: CountOtherAdmins :one
-- How many administrators would be left without this account. Read inside the write transaction
-- of SetAccountRole and DeleteAccount, so two concurrent demotions cannot both win (ADR 0014).
SELECT count(*) FROM accounts WHERE role = 'admin' AND id <> @id;

-- name: SetAccountRole :execrows
-- The appointment lifts the account to 18 and all libraries (the table CHECK demands it, and the
-- administration views read system-wide anyway); taking the role away leaves both as they are
-- (decision 12 f). The owner is never touched here: the service answers owner_protected first,
-- and `is_owner = 0` is the second lock in case it ever does not.
UPDATE accounts
SET role = @role,
    max_rating = CASE WHEN @role = 'admin' THEN 18 ELSE max_rating END,
    all_libraries = CASE WHEN @role = 'admin' THEN 1 ELSE all_libraries END,
    updated_at = @now
WHERE id = @id AND is_owner = 0;

-- name: UpdateAccountLimits :exec
UPDATE accounts
SET max_rating = @max_rating, all_libraries = @all_libraries, max_bitrate_kbps = sqlc.narg('max_bitrate_kbps'),
    max_height = sqlc.narg('max_height'), max_streams = sqlc.narg('max_streams'), updated_at = @now
WHERE id = @id;

-- name: DeleteAccount :execrows
DELETE FROM accounts WHERE id = @id;

-- name: ListAccountLibraries :many
SELECT library_id FROM account_libraries WHERE account_id = @account_id ORDER BY library_id;

-- name: ListGuestLibraries :many
SELECT al.account_id, al.library_id FROM account_libraries AS al
JOIN accounts AS a ON a.id = al.account_id
WHERE a.role = 'user'
ORDER BY al.account_id, al.library_id;

-- name: DeleteAccountLibraries :exec
DELETE FROM account_libraries WHERE account_id = @account_id;

-- name: InsertAccountLibrary :exec
INSERT INTO account_libraries (account_id, library_id) VALUES (@account_id, @library_id);

-- name: RevokePlaybackSessionsByAccount :many
UPDATE playback_sessions SET revoked_at = @now, stopped_by = @stopped_by
WHERE account_id = @account_id AND revoked_at IS NULL
RETURNING id;

-- name: RevokePlaybackSessionsByProfile :many
-- stopped_by: revoked when the profile is deleted, limit when its daily watch time is used up.
UPDATE playback_sessions SET revoked_at = @now, stopped_by = @stopped_by
WHERE profile_id = @profile_id AND revoked_at IS NULL
RETURNING id;

-- Device overview of the administration (W-7, M4).

-- name: ListDevicesAdmin :many
-- Devices, last contact first, with the owner of the last playback start and the number of running
-- streams (the activity window of CountActivePlaybackSessions).
SELECT d.id, d.user_agent, d.created_at, d.last_seen_at, d.name, d.type, d.capabilities, d.capabilities_at,
       a.email AS owner_email, p.name AS owner_profile,
       (SELECT count(*) FROM playback_sessions AS ps
        JOIN sessions AS s ON s.id = ps.session_id
        WHERE ps.device_id = d.id AND ps.revoked_at IS NULL AND ps.expires_at > @now
          AND ps.last_activity_at >= @active_after AND s.revoked_at IS NULL AND s.expires_at > @now) AS active_streams
FROM devices AS d
LEFT JOIN accounts AS a ON a.id = d.account_id
LEFT JOIN profiles AS p ON p.id = d.profile_id
ORDER BY d.last_seen_at DESC, d.id;

-- name: RevokeDeviceSessions :many
UPDATE sessions SET revoked_at = @now WHERE device_id = @device_id AND revoked_at IS NULL
RETURNING id;

-- name: DeletePinAttemptsByDevice :exec
DELETE FROM pin_attempts WHERE device_id = @device_id;

-- name: DeleteDevice :execrows
DELETE FROM devices WHERE id = @id;

-- name: InsertInvitation :one
INSERT INTO invitations (token_hash, note, max_rating, all_libraries, created_by, created_at, expires_at)
VALUES (@token_hash, @note, @max_rating, @all_libraries, sqlc.narg('created_by'), @now, @expires_at)
RETURNING *;

-- name: InsertInvitationLibrary :exec
INSERT INTO invitation_libraries (invitation_id, library_id) VALUES (@invitation_id, @library_id);

-- name: GetInvitationByTokenHash :one
SELECT * FROM invitations WHERE token_hash = @token_hash;

-- name: GetInvitation :one
SELECT * FROM invitations WHERE id = @id;

-- name: ListInvitations :many
SELECT i.*, a.email AS used_by_email FROM invitations AS i
LEFT JOIN accounts AS a ON a.id = i.used_by
ORDER BY i.created_at DESC, i.id DESC;

-- name: ListInvitationLibraries :many
SELECT invitation_id, library_id FROM invitation_libraries ORDER BY invitation_id, library_id;

-- name: ListInvitationLibraryIDs :many
SELECT library_id FROM invitation_libraries WHERE invitation_id = @invitation_id ORDER BY library_id;

-- name: MarkInvitationUsed :execrows
-- One-time: only an open (unused, unrevoked, unexpired) invitation can be taken.
UPDATE invitations SET used_at = @now, used_by = @used_by
WHERE id = @id AND used_at IS NULL AND revoked_at IS NULL AND expires_at > @now;

-- name: RevokeInvitation :execrows
UPDATE invitations SET revoked_at = @now WHERE id = @id AND used_at IS NULL AND revoked_at IS NULL;
