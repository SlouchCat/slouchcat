-- Accounts, profiles and sessions (package store/appq). Owner: auth / admin packages.
-- Times are Unix milliseconds.

-- name: CountAccounts :one
SELECT count(*) FROM accounts;

-- name: InsertAccount :one
-- The setup wizard's account is the owner; every other account comes from an invitation and is
-- inserted by InsertInvitedAccount, which writes its role as a literal (ADR 0014).
-- The account starts at 18 and may request titles (question 36, ADR 0045: every account without an
-- age limit).
INSERT INTO accounts (email, password_hash, role, is_owner, can_request, created_at, updated_at)
VALUES (@email, @password_hash, @role, @is_owner, 1, @now, @now)
RETURNING *;

-- name: GetAccountByEmail :one
SELECT * FROM accounts WHERE email = @email;

-- name: GetAccount :one
SELECT * FROM accounts WHERE id = @id;

-- name: InsertProfile :one
-- A profile without an age limit may request titles, one with an age limit may not (question 36,
-- ADR 0045): can_request follows max_rating; the account's right still caps it.
INSERT INTO profiles (account_id, name, max_rating, language, accent, all_libraries, is_default, can_request, pin_hash,
                      max_bitrate_kbps, max_height, max_streams, watch_limit_minutes,
                      audio_languages, subtitle_mode, subtitle_languages, created_at, updated_at)
VALUES (@account_id, @name, @max_rating, @language, sqlc.narg('accent'), @all_libraries, @is_default, CAST(@max_rating >= 18 AS INTEGER), sqlc.narg('pin_hash'),
        sqlc.narg('max_bitrate_kbps'), sqlc.narg('max_height'), sqlc.narg('max_streams'), sqlc.narg('watch_limit_minutes'),
        @audio_languages, @subtitle_mode, @subtitle_languages, @now, @now)
RETURNING *;

-- name: GetProfile :one
SELECT * FROM profiles WHERE id = @id;

-- name: GetDefaultProfile :one
SELECT * FROM profiles WHERE account_id = @account_id ORDER BY is_default DESC, id LIMIT 1;

-- name: InsertSession :one
-- kind: browser (cookie, sliding expiry), app or device (bearer, an expiry far in the future that
-- TouchSession never moves; ADR 0040).
INSERT INTO sessions (token_hash, account_id, profile_id, device_id, user_agent, created_at, last_seen_at, expires_at, kind)
VALUES (@token_hash, @account_id, sqlc.narg('profile_id'), sqlc.narg('device_id'), @user_agent, @now, @now, @expires_at, @kind)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions WHERE token_hash = @token_hash AND revoked_at IS NULL AND expires_at > @now;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = @now, expires_at = @expires_at WHERE id = @id;

-- name: RevokeSession :execrows
UPDATE sessions SET revoked_at = @now WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeOlderBearerSessionsOnDevice :many
-- A new app or device session of an account on a device ends the older app and device sessions of
-- the same account on the same device (concept 27, code review m-5): a lost token dies with the
-- next sign-in.
UPDATE sessions SET revoked_at = @now
WHERE account_id = @account_id AND device_id = @device_id AND kind <> 'browser' AND id <> @keep_id
  AND revoked_at IS NULL
RETURNING id;

-- name: RevokeAccountSessions :execrows
UPDATE sessions SET revoked_at = @now WHERE account_id = @account_id AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :execrows
-- A session that a playback session still refers to stays (deleting it would delete the playback
-- sessions, which package playback keeps for the statistics); it goes with the last of them. A
-- revoked or expired session authorises nothing.
DELETE FROM sessions
WHERE (sessions.expires_at < @now OR sessions.revoked_at IS NOT NULL AND sessions.revoked_at < @before)
  AND NOT EXISTS (SELECT 1 FROM playback_sessions AS ps WHERE ps.session_id = sessions.id);

-- name: GetSetting :one
SELECT value FROM settings WHERE key = @key;

-- name: UpsertSetting :exec
INSERT INTO settings (key, value, updated_at) VALUES (@key, @value, @now)
ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at;

-- name: DeleteSetting :exec
DELETE FROM settings WHERE key = @key;
