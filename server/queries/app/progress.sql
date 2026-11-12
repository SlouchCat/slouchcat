-- Playback progress (package store/appq). Owner: playback package. Visibility of the item is
-- checked before any of these run. Times are Unix milliseconds.

-- name: GetProgress :one
SELECT * FROM progress WHERE profile_id = @profile_id AND item_id = @item_id;

-- name: UpsertProgress :exec
-- playback_rate (H-3) is replaced only by an event that carries one; NULL keeps the stored speed.
-- watched_at (concept 20 section 6.3) is the time of an event that meets the 90 % rule, NULL for
-- one that does not: the later of it and the stored one stays, so an event below 90 % reopens the
-- title for continue watching (completed) but never clears its check mark.
INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at, playback_rate, watched_at)
VALUES (@profile_id, @item_id, @position_ms, @duration_ms, @completed, @now, sqlc.narg('playback_rate'), sqlc.narg('watched_at'))
ON CONFLICT (profile_id, item_id) DO UPDATE SET
    position_ms = excluded.position_ms, duration_ms = excluded.duration_ms,
    completed = excluded.completed, updated_at = excluded.updated_at,
    playback_rate = COALESCE(excluded.playback_rate, progress.playback_rate),
    watched_at = CASE WHEN progress.watched_at IS NULL OR excluded.watched_at > progress.watched_at
                      THEN COALESCE(excluded.watched_at, progress.watched_at) ELSE progress.watched_at END;

-- name: MarkProgressWatched :exec
-- An accepted event that meets the 90 % rule but does not replace the stored progress (a late
-- online event, an offline event older than the stored progress) still finishes the title:
-- watched_at becomes the later of the stored one and the event's time, nothing else changes.
UPDATE progress SET watched_at = max(COALESCE(watched_at, 0), @at)
WHERE profile_id = @profile_id AND item_id = @item_id;

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
-- Played time of a profile in [from, to) by server time (K-5 watch time, one calendar day): titles
-- plus listened radio (radio_listen_time, concept 16 section 7.10). Radio counts per quarter hour;
-- a day starts on a quarter hour (see below), so a quarter lies wholly inside or outside the day.
SELECT CAST(
    COALESCE((SELECT SUM(pe.played_ms) FROM progress_events AS pe
              WHERE pe.profile_id = @profile_id AND pe.server_time >= @from_ms AND pe.server_time < @to_ms), 0)
  + COALESCE((SELECT SUM(rl.played_ms) FROM radio_listen_time AS rl
              WHERE rl.profile_id = @profile_id AND rl.quarter * 900000 >= @from_ms
                AND rl.quarter * 900000 < @to_ms), 0)
  AS INTEGER) AS played_ms;

-- name: SumPlayedMsByQuarterHour :many
-- Played time of a profile in [from, to) per quarter hour of server time (K-5 report): every UTC
-- offset is a multiple of 15 minutes, so each quarter hour lies within one local calendar day.
-- Listened radio (radio_listen_time) adds to the quarter hours of titles (concept 16 section 7.10).
SELECT CAST(quarter AS INTEGER) AS quarter, CAST(SUM(played_ms) AS INTEGER) AS played_ms
FROM (
    SELECT pe.server_time / 900000 AS quarter, pe.played_ms FROM progress_events AS pe
    WHERE pe.profile_id = @profile_id AND pe.server_time >= @from_ms AND pe.server_time < @to_ms
    UNION ALL
    SELECT rl.quarter, rl.played_ms FROM radio_listen_time AS rl
    WHERE rl.profile_id = @profile_id AND rl.quarter * 900000 >= @from_ms AND rl.quarter * 900000 < @to_ms
) AS played
GROUP BY quarter
ORDER BY quarter;

-- Condensation of old events into progress_days (M10, plan 3.2; package playback, condense.go).
-- Only whole UTC days older than playback.ProgressRetention are condensed, one day per transaction,
-- so the single writer is never held for long and a run that stops in the middle leaves whole days
-- behind. The K-5 watch time above reads the running day only and is never affected.

-- name: OldestCondensableDay :one
-- The UTC day number (server_time / 86400000) of the oldest raw event before the day-aligned
-- boundary, -1 when there is none left to condense.
SELECT CAST(COALESCE(MIN(server_time) / 86400000, -1) AS INTEGER) AS day FROM progress_events
WHERE server_time < @before;

-- name: CondenseProgressDay :exec
-- Adds one whole UTC day of raw events to progress_days: the played time summed, plays = the
-- distinct playback sessions of the day plus the events without one, an event without a device
-- under the empty device id. Run again for the same day (a late offline upload) it adds to the
-- stored row instead of replacing it, so the condensation is idempotent over the rows it already
-- deleted and never loses a late upload.
INSERT INTO progress_days (profile_id, item_id, device_id, day, played_ms, plays)
SELECT profile_id, item_id, COALESCE(device_id, ''), @day, SUM(played_ms),
       count(DISTINCT playback_session_id) + SUM(playback_session_id IS NULL)
FROM progress_events
WHERE server_time >= @day * 86400000 AND server_time < (@day + 1) * 86400000
GROUP BY profile_id, item_id, COALESCE(device_id, '')
ON CONFLICT (profile_id, item_id, device_id, day) DO UPDATE SET
    played_ms = progress_days.played_ms + excluded.played_ms,
    plays = progress_days.plays + excluded.plays;

-- name: DeleteCondensedProgressEvents :execrows
-- Deletes the raw events of a condensed UTC day; runs in the same transaction as
-- CondenseProgressDay, so no event is ever counted twice or lost.
DELETE FROM progress_events
WHERE server_time >= CAST(@day AS INTEGER) * 86400000
  AND server_time < (CAST(@day AS INTEGER) + 1) * 86400000;
