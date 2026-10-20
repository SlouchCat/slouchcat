-- Playback progress (package store/appq). Owner: playback package. Visibility of the item is
-- checked before any of these run. Times are Unix milliseconds.

-- name: GetProgress :one
SELECT * FROM progress WHERE profile_id = @profile_id AND item_id = @item_id;

-- name: UpsertProgress :exec
INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at)
VALUES (@profile_id, @item_id, @position_ms, @duration_ms, @completed, @now)
ON CONFLICT (profile_id, item_id) DO UPDATE SET
    position_ms = excluded.position_ms, duration_ms = excluded.duration_ms,
    completed = excluded.completed, updated_at = excluded.updated_at;

-- name: InsertProgressEvent :exec
INSERT INTO progress_events (profile_id, item_id, media_file_id, device_id, playback_session_id, position_ms,
                             duration_ms, played_ms, seq, state, client_time, server_time)
VALUES (@profile_id, @item_id, @media_file_id, @device_id, sqlc.narg('playback_session_id'), @position_ms,
        @duration_ms, @played_ms, @seq, @state, @client_time, @server_time);

-- name: CountDuplicateProgressEvents :one
-- An event the client sent again (a retry after a lost answer): the same playback session, device
-- and media file with the same seq, client time, position and state (K-5: it must not count
-- twice).
SELECT count(*) FROM progress_events
WHERE playback_session_id IS sqlc.narg('playback_session_id') AND seq = @seq
  AND profile_id = @profile_id AND item_id = @item_id
  AND device_id IS sqlc.narg('device_id') AND media_file_id IS sqlc.narg('media_file_id')
  AND client_time = @client_time AND position_ms = @position_ms AND state = @state;

-- name: GetLastPlaybackEventTime :one
-- Server time of the newest event of the same playback (playback session, device and media file),
-- 0 without one; the played time of the next event is bounded by the time since (K-5).
SELECT CAST(COALESCE(MAX(server_time), 0) AS INTEGER) AS server_time FROM progress_events
WHERE playback_session_id IS sqlc.narg('playback_session_id')
  AND profile_id = @profile_id AND item_id = @item_id
  AND device_id IS sqlc.narg('device_id') AND media_file_id IS sqlc.narg('media_file_id');

-- name: GetLatestDeviceProgressEvent :one
-- The newest event (by client clock, then seq) of one device and media file, to drop late events
-- of the same playback.
SELECT seq, client_time FROM progress_events
WHERE profile_id = @profile_id AND item_id = @item_id AND device_id = @device_id
  AND media_file_id IS @media_file_id
ORDER BY client_time DESC, seq DESC
LIMIT 1;

-- name: SumPlayedMs :one
-- Played time of a profile in [from, to) by server time (K-5 watch time, one calendar day).
SELECT CAST(COALESCE(SUM(played_ms), 0) AS INTEGER) AS played_ms FROM progress_events
WHERE profile_id = @profile_id AND server_time >= @from_ms AND server_time < @to_ms;

-- name: SumPlayedMsByQuarterHour :many
-- Played time of a profile in [from, to) per quarter hour of server time (K-5 report): every UTC
-- offset is a multiple of 15 minutes, so each quarter hour lies within one local calendar day.
SELECT CAST(server_time / 900000 AS INTEGER) AS quarter, CAST(SUM(played_ms) AS INTEGER) AS played_ms
FROM progress_events
WHERE profile_id = @profile_id AND server_time >= @from_ms AND server_time < @to_ms
GROUP BY quarter
ORDER BY quarter;
