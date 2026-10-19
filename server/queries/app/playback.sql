-- Playback sessions and stream tokens (package store/appq). Owner: playback package. The item of a
-- row is re-checked through the visibility package on every request; these queries never read
-- title tables. Times are Unix milliseconds.

-- name: InsertPlaybackSession :exec
INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, device_id,
                               mode, backend, reason, started_at, last_activity_at, expires_at)
VALUES (@id, @session_id, @account_id, @profile_id, @item_id, @media_file_id, @device_id,
        @mode, @backend, @reason, @now, @now, @expires_at);

-- name: GetPlaybackSession :one
-- A playback session that is neither revoked nor expired and whose browser session is still valid.
SELECT ps.* FROM playback_sessions AS ps
JOIN sessions AS s ON s.id = ps.session_id
WHERE ps.id = @id AND ps.revoked_at IS NULL AND ps.expires_at > @now
  AND s.revoked_at IS NULL AND s.expires_at > @now;

-- name: TouchPlaybackSession :exec
UPDATE playback_sessions SET last_activity_at = @now, expires_at = @expires_at
WHERE id = @id AND revoked_at IS NULL;

-- name: RevokePlaybackSession :execrows
-- Revokes a playback session of one browser session (only the owner may stop it).
UPDATE playback_sessions SET revoked_at = @now
WHERE id = @id AND session_id = @session_id AND revoked_at IS NULL;

-- name: RevokePlaybackSessionsBySession :many
UPDATE playback_sessions SET revoked_at = @now
WHERE session_id = @session_id AND revoked_at IS NULL
RETURNING id;

-- name: ListEndedPlaybackSessions :many
-- Revoked or expired playback sessions, and those whose browser session ended, whose transcode
-- output or open stream may still be running.
SELECT ps.id FROM playback_sessions AS ps
LEFT JOIN sessions AS s ON s.id = ps.session_id
WHERE ps.revoked_at IS NOT NULL OR ps.expires_at <= @now
   OR s.id IS NULL OR s.revoked_at IS NOT NULL OR s.expires_at <= @now;

-- name: DeleteExpiredPlaybackSessions :execrows
DELETE FROM playback_sessions WHERE expires_at < @before OR revoked_at IS NOT NULL AND revoked_at < @before;
