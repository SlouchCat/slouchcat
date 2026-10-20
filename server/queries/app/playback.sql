-- Playback sessions and stream tokens (package store/appq). Owner: playback package. The item of a
-- row is re-checked through the visibility package on every request; these queries never read
-- title tables. Times are Unix milliseconds.

-- name: InsertPlaybackSession :exec
-- The decision of a start (M4): quality rung, output size and bit rate, hardware encoder, the
-- chosen tracks and how subtitles are delivered, and the capabilities the client reported, so a
-- rebuild after a restart repeats the same decision.
INSERT INTO playback_sessions (id, session_id, account_id, profile_id, item_id, media_file_id, device_id,
                               mode, backend, reason, quality, width, height, bitrate, hardware, audio_track,
                               subtitle_track, subtitle_delivery, capabilities, position_ms,
                               started_at, last_activity_at, expires_at)
VALUES (@id, @session_id, @account_id, @profile_id, @item_id, @media_file_id, @device_id,
        @mode, @backend, @reason, sqlc.narg('quality'), sqlc.narg('width'), sqlc.narg('height'), sqlc.narg('bitrate'),
        @hardware, sqlc.narg('audio_track'), sqlc.narg('subtitle_track'), sqlc.narg('subtitle_delivery'),
        sqlc.narg('capabilities'), sqlc.narg('position_ms'), @now, @now, @expires_at);

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
UPDATE playback_sessions SET revoked_at = @now, stopped_by = 'client'
WHERE id = @id AND session_id = @session_id AND revoked_at IS NULL;

-- name: RevokePlaybackSessionsBySession :many
-- stopped_by is logout for a logout and a profile switch.
UPDATE playback_sessions SET revoked_at = @now, stopped_by = @stopped_by
WHERE session_id = @session_id AND revoked_at IS NULL
RETURNING id;

-- name: StopPlaybackSession :execrows
-- Ends a running playback session of any browser session (administration, W-6).
UPDATE playback_sessions SET revoked_at = @now, stopped_by = @stopped_by
WHERE id = @id AND revoked_at IS NULL AND expires_at > @now;

-- name: RevokePlaybackSessionsByDevice :many
-- Playback sessions of a device: recorded for it or started by one of its browser sessions.
UPDATE playback_sessions SET revoked_at = @now, stopped_by = @stopped_by
WHERE revoked_at IS NULL
  AND (playback_sessions.device_id = sqlc.arg('device_id')
       OR playback_sessions.session_id IN (SELECT s.id FROM sessions AS s WHERE s.device_id = sqlc.arg('device_id')))
RETURNING id;

-- name: CountActivePlaybackSessions :one
-- Running streams of a profile (W-8 maxStreams): not revoked, not expired, a request within the
-- activity window, browser session valid. A start revokes the playback it replaces (same browser
-- session and title, RevokeReplacedPlaybackSessions) before it counts.
SELECT count(*) FROM playback_sessions AS ps
JOIN sessions AS s ON s.id = ps.session_id
WHERE ps.profile_id = @profile_id AND ps.revoked_at IS NULL AND ps.expires_at > @now
  AND ps.last_activity_at >= @active_after
  AND s.revoked_at IS NULL AND s.expires_at > @now;

-- name: CountActiveAccountPlaybackSessions :one
-- Running streams of every profile of an account (W-8 guest cap maxStreams), as
-- CountActivePlaybackSessions.
SELECT count(*) FROM playback_sessions AS ps
JOIN sessions AS s ON s.id = ps.session_id
WHERE ps.account_id = @account_id AND ps.revoked_at IS NULL AND ps.expires_at > @now
  AND ps.last_activity_at >= @active_after
  AND s.revoked_at IS NULL AND s.expires_at > @now;

-- name: RevokeReplacedPlaybackSessions :many
-- A start replaces the playback of the same browser session and title (a track, quality or
-- version switch): the replaced one ends (stopped_by client) before the new one is counted.
UPDATE playback_sessions SET revoked_at = @now, stopped_by = 'client'
WHERE session_id = @session_id AND item_id = @item_id AND revoked_at IS NULL
RETURNING id;

-- name: RecordPlaybackPosition :execrows
-- A progress event of the playback (W-6 position, activity of the stream overview).
UPDATE playback_sessions SET position_ms = @position_ms, last_activity_at = @now, expires_at = @expires_at
WHERE id = @id AND profile_id = @profile_id AND revoked_at IS NULL;

-- name: ListActivePlaybackSessions :many
-- Running streams for the administration (W-6), newest first.
SELECT ps.id, ps.item_id, ps.media_file_id, ps.device_id, ps.mode, ps.backend, ps.reason, ps.quality,
       ps.width, ps.height, ps.bitrate, ps.hardware, ps.started_at, ps.last_activity_at, ps.position_ms,
       a.email AS account_email, p.name AS profile_name, d.name AS device_name
FROM playback_sessions AS ps
JOIN sessions AS s ON s.id = ps.session_id
JOIN accounts AS a ON a.id = ps.account_id
JOIN profiles AS p ON p.id = ps.profile_id
LEFT JOIN devices AS d ON d.id = ps.device_id
WHERE ps.revoked_at IS NULL AND ps.expires_at > @now AND ps.last_activity_at >= @active_after
  AND s.revoked_at IS NULL AND s.expires_at > @now
ORDER BY ps.started_at DESC, ps.id DESC;

-- name: GetDevice :one
SELECT * FROM devices WHERE id = @id;

-- name: RecordDeviceProfile :exec
-- The device of a playback start (W-7): owner (account and profile of the start) and, when the
-- client reported them, its capability profile, name and type. A device without a row (a
-- third-party client without the device cookie) gets one.
INSERT INTO devices (id, user_agent, created_at, last_seen_at, name, type, account_id, profile_id, capabilities, capabilities_at)
VALUES (@id, sqlc.narg('user_agent'), @now, @now, sqlc.narg('name'), sqlc.narg('type'), @account_id, @profile_id,
        sqlc.narg('capabilities'), sqlc.narg('capabilities_at'))
ON CONFLICT (id) DO UPDATE SET
    last_seen_at = excluded.last_seen_at,
    account_id = excluded.account_id,
    profile_id = excluded.profile_id,
    name = COALESCE(excluded.name, devices.name),
    type = COALESCE(excluded.type, devices.type),
    capabilities = COALESCE(excluded.capabilities, devices.capabilities),
    capabilities_at = COALESCE(excluded.capabilities_at, devices.capabilities_at),
    user_agent = COALESCE(devices.user_agent, excluded.user_agent);

-- name: ListEndedPlaybackSessions :many
-- Revoked or expired playback sessions, and those whose browser session ended, whose transcode
-- output or open stream may still be running.
SELECT ps.id FROM playback_sessions AS ps
LEFT JOIN sessions AS s ON s.id = ps.session_id
WHERE ps.revoked_at IS NOT NULL OR ps.expires_at <= @now
   OR s.id IS NULL OR s.revoked_at IS NOT NULL OR s.expires_at <= @now;

-- name: DeleteExpiredPlaybackSessions :execrows
DELETE FROM playback_sessions WHERE expires_at < @before OR revoked_at IS NOT NULL AND revoked_at < @before;
