-- Accounts, profiles and sessions (package store/appq). Owner: auth / admin packages.
-- Times are Unix milliseconds.

-- name: CountAccounts :one
SELECT count(*) FROM accounts;

-- name: InsertAccount :one
INSERT INTO accounts (email, password_hash, role, created_at, updated_at)
VALUES (@email, @password_hash, @role, @now, @now)
RETURNING *;

-- name: GetAccountByEmail :one
SELECT * FROM accounts WHERE email = @email;

-- name: GetAccount :one
SELECT * FROM accounts WHERE id = @id;

-- name: InsertProfile :one
INSERT INTO profiles (account_id, name, max_rating, language, accent, all_libraries, is_default, pin_hash,
                      max_bitrate_kbps, max_height, max_streams, watch_limit_minutes, created_at, updated_at)
VALUES (@account_id, @name, @max_rating, @language, sqlc.narg('accent'), @all_libraries, @is_default, sqlc.narg('pin_hash'),
        sqlc.narg('max_bitrate_kbps'), sqlc.narg('max_height'), sqlc.narg('max_streams'), sqlc.narg('watch_limit_minutes'), @now, @now)
RETURNING *;

-- name: GetProfile :one
SELECT * FROM profiles WHERE id = @id;

-- name: GetDefaultProfile :one
SELECT * FROM profiles WHERE account_id = @account_id ORDER BY is_default DESC, id LIMIT 1;

-- name: InsertSession :one
INSERT INTO sessions (token_hash, account_id, profile_id, device_id, user_agent, created_at, last_seen_at, expires_at)
VALUES (@token_hash, @account_id, sqlc.narg('profile_id'), sqlc.narg('device_id'), @user_agent, @now, @now, @expires_at)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions WHERE token_hash = @token_hash AND revoked_at IS NULL AND expires_at > @now;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = @now, expires_at = @expires_at WHERE id = @id;

-- name: RevokeSession :execrows
UPDATE sessions SET revoked_at = @now WHERE id = @id AND revoked_at IS NULL;

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
