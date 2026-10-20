-- Offline PIN material and uploaded offline progress events (M5, AN-10, AN-18, AN-19; package
-- store/appq). Owner: auth (profiles) and playback (progress_events, and the download an event
-- names). Never touches title tables; the visibility of an event's title is checked before it is
-- stored. Times are Unix milliseconds.

-- name: SetProfilePinSalt :exec
-- The salt of the normal PIN: new with every PIN, NULL without one (the client derives its offline
-- verifier from it).
UPDATE profiles SET pin_salt = sqlc.narg('pin_salt') WHERE id = @id;

-- name: SetProfileOfflinePin :exec
-- The PBKDF2 verifier of the separate offline PIN as JSON, NULL to remove it.
UPDATE profiles SET offline_pin = sqlc.narg('offline_pin'), updated_at = @now WHERE id = @id;

-- name: CountOfflineDuplicateEvents :one
-- An offline event uploaded before: the same playback id and seq for the same profile and title.
SELECT count(*) FROM progress_events
WHERE playback_session_id = @playback_id AND seq = @seq AND profile_id = @profile_id AND item_id = @item_id;

-- name: GetPreviousPlaybackEventTime :one
-- Server time of the newest stored event of a playback at or before a time, 0 without one; the
-- played time of an offline event is bounded by the time since (K-5).
SELECT CAST(COALESCE(MAX(server_time), 0) AS INTEGER) AS server_time FROM progress_events
WHERE playback_session_id = @playback_id AND profile_id = @profile_id AND item_id = @item_id
  AND server_time <= @at;

-- name: InsertOfflineProgressEvent :exec
-- server_time is the corrected event time, uploaded_at the server time of the upload.
INSERT INTO progress_events (profile_id, item_id, media_file_id, device_id, playback_session_id, position_ms,
                             duration_ms, played_ms, seq, state, client_time, server_time, offline, uploaded_at,
                             download_id)
VALUES (@profile_id, @item_id, sqlc.narg('media_file_id'), sqlc.narg('device_id'), @playback_id, @position_ms,
        @duration_ms, @played_ms, @seq, @state, @client_time, @server_time, 1, @uploaded_at,
        sqlc.narg('download_id'));

-- name: GetOfflineEventDownload :one
-- The download an offline event names: its account, device, title, allowed profiles and creation
-- (an event must fit it and cannot be older than the download).
SELECT account_id, device_id, item_id, allowed_profile_ids, created_at FROM downloads WHERE id = @id;
