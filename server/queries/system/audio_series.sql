-- Audiobook series (concept 19 stage D2, D4.4): the writes of library.RelinkAudioSeries, the
-- sweep and "Reihe bearbeiten" (package admin). Reads for viewers live in queries/scoped/audio.sql.

-- The fields of the given titles that decide their series.
-- name: ListAudioSeriesLinkItems :many
SELECT id, library_id, kind, series_name, author, audio_series_id, deleted_at
FROM items
WHERE id IN (sqlc.slice('ids'));

-- name: GetAudioSeries :one
SELECT * FROM audio_series WHERE id = @id;

-- name: GetAudioSeriesByKey :one
SELECT * FROM audio_series WHERE library_id = @library_id AND series_key = @series_key;

-- name: InsertAudioSeries :exec
INSERT INTO audio_series (id, library_id, series_key, name, name_locked, created_at, updated_at)
VALUES (@id, @library_id, @series_key, @name, 0, @now, @now);

-- name: SetAudioSeriesKey :exec
UPDATE audio_series SET series_key = @series_key, updated_at = @now WHERE id = @id;

-- The name that follows the volumes; a locked name stays.
-- name: SetAudioSeriesName :exec
UPDATE audio_series SET name = @name, updated_at = @now WHERE id = @id AND name_locked = 0 AND name <> @name;

-- "Reihe bearbeiten" (D5.4): the name is set and no longer follows the volumes.
-- name: LockAudioSeriesName :exec
UPDATE audio_series SET name = @name, name_locked = 1, updated_at = @now WHERE id = @id;

-- name: DeleteAudioSeries :exec
DELETE FROM audio_series WHERE id = @id;

-- name: SetItemAudioSeries :exec
UPDATE items SET audio_series_id = @series_id WHERE id = @id;

-- The books of a series that are not deleted.
-- name: CountAudioSeriesLiveItems :one
SELECT count(*) FROM items WHERE audio_series_id = @series_id AND deleted_at IS NULL;

-- Every row of a series, soft-deleted ones included (the sweep keeps such a series).
-- name: CountAudioSeriesRows :one
SELECT count(*) FROM items WHERE audio_series_id = @series_id;

-- The series names of the books of a series that are not deleted, with their number.
-- name: ListAudioSeriesSpellings :many
SELECT CAST(series_name AS TEXT) AS series_name, count(*) AS n
FROM items
WHERE audio_series_id = @series_id AND deleted_at IS NULL AND series_name IS NOT NULL
GROUP BY series_name;

-- The sweep (D4.4): a series without any row in items and without a locked name.
-- name: DeleteOrphanAudioSeries :execrows
DELETE FROM audio_series
WHERE name_locked = 0
  AND NOT EXISTS (SELECT 1 FROM items AS i WHERE i.audio_series_id = audio_series.id);

-- Every audiobook, soft-deleted ones included (the first link after migration 00045).
-- name: ListAudiobookIDs :many
SELECT id FROM items WHERE kind = 'audiobook' ORDER BY id;

-- The books of a series the given profile may see ("In alle Baende schreiben", D5.4).
-- name: ListVisibleAudioSeriesVolumeIDs :many
SELECT i.id
FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.audio_series_id = @series_id
ORDER BY i.id;
