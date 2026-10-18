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
INSERT INTO progress_events (profile_id, item_id, media_file_id, device_id, position_ms, duration_ms,
                             played_ms, seq, state, client_time, server_time)
VALUES (@profile_id, @item_id, @media_file_id, @device_id, @position_ms, @duration_ms,
        @played_ms, @seq, @state, @client_time, @server_time);
