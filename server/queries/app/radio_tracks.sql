-- Cover cache of the webradio (package store/appq, concept 16 section 8.4, stage 2; owner: radio).
-- One row per normalised "artist|title" or announced image address (migration 00039); a hit is kept
-- 30 days, a miss 24 hours. Every query reads one row through a key, except the sweep, which walks
-- the index on expires_at.

-- name: GetRadioTrackByKey :one
SELECT * FROM radio_tracks WHERE key = @key;

-- name: GetRadioTrack :one
SELECT * FROM radio_tracks WHERE id = @id;

-- name: UpsertRadioTrack :one
INSERT INTO radio_tracks (key, artist, title, state, source, image_hash, checked_at, expires_at)
VALUES (@key, sqlc.narg('artist'), sqlc.narg('title'), @state, sqlc.narg('source'), sqlc.narg('image_hash'), @now, @expires_at)
ON CONFLICT (key) DO UPDATE SET artist = excluded.artist, title = excluded.title, state = excluded.state,
    source = excluded.source, image_hash = excluded.image_hash, checked_at = excluded.checked_at,
    expires_at = excluded.expires_at
RETURNING id;

-- name: DeleteExpiredRadioTracks :many
DELETE FROM radio_tracks WHERE expires_at < @now RETURNING id, image_hash;

-- The cover budget of the daily sweep (security review M-1): the found rows over the budget, the
-- oldest checks first, go with their files. Both walk the table once a day, never per request.

-- name: CountFoundRadioTracks :one
SELECT count(*) FROM radio_tracks WHERE state = 'found';

-- name: DeleteOldestFoundRadioTracks :many
DELETE FROM radio_tracks WHERE id IN (
    SELECT id FROM radio_tracks WHERE state = 'found' ORDER BY checked_at, id LIMIT @n
) RETURNING id;
