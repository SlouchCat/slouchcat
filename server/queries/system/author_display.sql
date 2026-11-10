-- Unified author and series names (concept 19 stage D3, D6.4, "Vereinheitlichen"): the reads of
-- library.RecomputeAuthorDisplay. The write of items.author_display is UpdateAuthorDisplay
-- (book_match.sql), the one of audio_series.name SetAudioSeriesName (audio_series.sql).

-- Every audiobook that is not deleted with its author, its unified name, its locks and the
-- provider that wrote its author (NULL: no provenance, as after an edit or before stage D1).
-- name: ListAudiobookAuthors :many
SELECT i.id, i.author, i.author_display, i.locked_fields, s.provider AS author_provider
FROM items AS i
LEFT JOIN item_field_sources AS s ON s.item_id = i.id AND s.field = 'author' AND s.lang = ''
WHERE i.kind = 'audiobook' AND i.deleted_at IS NULL
ORDER BY i.id;

-- The series whose name follows the volumes (not locked).
-- name: ListUnlockedAudioSeriesIDs :many
SELECT id FROM audio_series WHERE name_locked = 0 ORDER BY id;
