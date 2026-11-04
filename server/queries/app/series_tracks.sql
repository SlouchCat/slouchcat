-- Languages a profile last chose by hand for a series (package store/appq; concept 13 section 11.3,
-- migration 00032). Owners: playback (reads and writes the choice of a start) and auth (resets it
-- on "Meine Wiedergabe"); offline reads it for the audio of downloads. The series id always comes from the visibility package; these queries
-- never read title tables. Times are Unix milliseconds.

-- name: GetSeriesTracks :one
SELECT * FROM profile_series_tracks WHERE profile_id = @profile_id AND series_id = @series_id;

-- name: UpsertSeriesTracks :exec
-- An empty language counts as none (a track without language); subtitle_off records that the
-- profile switched subtitles off for the series.
INSERT INTO profile_series_tracks (profile_id, series_id, audio_language, subtitle_language, subtitle_off, updated_at)
VALUES (@profile_id, @series_id, @audio_language, @subtitle_language, @subtitle_off, @now)
ON CONFLICT (profile_id, series_id) DO UPDATE SET
    audio_language = excluded.audio_language, subtitle_language = excluded.subtitle_language,
    subtitle_off = excluded.subtitle_off, updated_at = excluded.updated_at;

-- name: DeleteSeriesTracksByProfile :execrows
DELETE FROM profile_series_tracks WHERE profile_id = @profile_id;
