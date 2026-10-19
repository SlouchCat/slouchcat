-- Devices, PIN locks, profile and guest management, invitations (package store/appq). Owner:
-- auth (all of it) and admin (guests, invitations). Never touches title tables. Times are Unix
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
    all_libraries = @all_libraries, updated_at = @now
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
UPDATE sessions SET profile_id = @profile_id, device_id = COALESCE(sqlc.narg('device_id'), device_id), last_seen_at = @now
WHERE id = @id AND revoked_at IS NULL;

-- name: TouchAccount :exec
UPDATE accounts SET last_seen_at = @now WHERE id = @id;

-- name: InsertGuestAccount :one
INSERT INTO accounts (email, password_hash, role, max_rating, all_libraries, invited_by, created_at, updated_at)
VALUES (@email, @password_hash, 'guest', @max_rating, @all_libraries, sqlc.narg('invited_by'), @now, @now)
RETURNING *;

-- name: GetGuestAccount :one
SELECT * FROM accounts WHERE id = @id AND role = 'guest';

-- name: ListGuestAccounts :many
SELECT a.id, a.email, a.max_rating, a.all_libraries, a.created_at, a.last_seen_at,
       (SELECT count(*) FROM profiles AS p WHERE p.account_id = a.id) AS profile_count
FROM accounts AS a
WHERE a.role = 'guest'
ORDER BY a.created_at DESC, a.id DESC;

-- name: UpdateAccountLimits :exec
UPDATE accounts SET max_rating = @max_rating, all_libraries = @all_libraries, updated_at = @now WHERE id = @id;

-- name: DeleteAccount :execrows
DELETE FROM accounts WHERE id = @id;

-- name: ListAccountLibraries :many
SELECT library_id FROM account_libraries WHERE account_id = @account_id ORDER BY library_id;

-- name: ListGuestLibraries :many
SELECT al.account_id, al.library_id FROM account_libraries AS al
JOIN accounts AS a ON a.id = al.account_id
WHERE a.role = 'guest'
ORDER BY al.account_id, al.library_id;

-- name: DeleteAccountLibraries :exec
DELETE FROM account_libraries WHERE account_id = @account_id;

-- name: InsertAccountLibrary :exec
INSERT INTO account_libraries (account_id, library_id) VALUES (@account_id, @library_id);

-- name: RevokePlaybackSessionsByAccount :many
UPDATE playback_sessions SET revoked_at = @now
WHERE account_id = @account_id AND revoked_at IS NULL
RETURNING id;

-- name: RevokePlaybackSessionsByProfile :many
UPDATE playback_sessions SET revoked_at = @now
WHERE profile_id = @profile_id AND revoked_at IS NULL
RETURNING id;

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
