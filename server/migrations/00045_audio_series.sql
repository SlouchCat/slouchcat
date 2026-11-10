-- Audiobook series (1.25.0, concept 19 stage D2, D4.4): the row «Reihen und Sammlungen» and the
-- page of a series group the books of a library by a series key instead of a collection.
--
--   audio_series        one series per library and key. series_key is the normalised series name
--                       plus the normalised first author (D4.1, library/audioseries); the id does
--                       not follow the key (D4.4): a new list of suffixes changes no id, so the
--                       address of a series page stays valid. name is the displayed name, the most
--                       common spelling of the books without suffix, or the name «Reihe
--                       bearbeiten» set and locked (name_locked = 1, D5.4).
--   items.audio_series_id
--                       the series of a book, set only by library.RelinkAudioSeries in the
--                       transaction of the writer (scan, editor, «Reihe bearbeiten»). NULL for a
--                       book without series name and for every other kind; a deleted series sets
--                       it NULL.
--
-- The library sweep deletes a series without any row in items (soft-deleted ones count) and without
-- a locked name. The books that exist already are linked once at the first start after the update
-- (library.Service.RelinkAudioSeriesOnce); the migration itself links nothing, because the key
-- needs the suffix list of the server.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE audio_series (
    id          TEXT    PRIMARY KEY CHECK (length(id) = 26),
    library_id  INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    series_key  TEXT    NOT NULL CHECK (series_key <> ''),
    name        TEXT    NOT NULL,
    name_locked INTEGER NOT NULL DEFAULT 0 CHECK (name_locked IN (0, 1)),
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    UNIQUE (library_id, series_key)
) STRICT;

ALTER TABLE items ADD COLUMN audio_series_id TEXT REFERENCES audio_series(id) ON DELETE SET NULL;

-- The volumes of a series (page, count, sweep); only books carry one.
CREATE INDEX items_audio_series ON items (audio_series_id) WHERE audio_series_id IS NOT NULL;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
