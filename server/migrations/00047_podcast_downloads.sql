-- Stored podcast episodes (1.27.0, concept 19 part A, sections 5, 6 and 10; ADR 0029): Slouchcat
-- loads the enclosure of a feed episode into the folder of its podcast and enters it as the
-- episode's media file. What is stored here is only the setting and the failures; whether an
-- episode is stored, removed, queued or loading, and which folder a podcast has, is derived from
-- media_files and jobs (ADR 0029, decision 7).
--
--   libraries.podcast_mode      save | link (default): «Neue Folgen» of a podcast library
--   libraries.podcast_scope     new (default) | last_n | all: what save loads (5.2)
--   libraries.podcast_last_n    N of last_n, 1 to 100 (default 10)
--   libraries.podcast_parallel  «Gleichzeitig laden», 1 to 5 (default 2)
--   libraries.podcast_root_id   «Ordner für neue Abos»; NULL = the first podcast folder of the
--                               library. Emptied when the folder goes or stops holding podcasts.
--   podcast_settings            the setting of one podcast; NULL = like its library. last_n counts
--                               only with an own scope. pending_root_id and pending_folder name
--                               the folder a download created for a podcast without any file,
--                               until its first file is entered (6.2, step 2).
--   podcast_download_failures   the last failure of an episode's download: a code, never an
--                               address (CLAUDE.md rule 10); a success removes the row.
--
-- Every library starts at «Nur Link» (O-2): after the update Slouchcat behaves as before. SQLite
-- adds a column with a CHECK constraint or a REFERENCES clause with a NULL default to a STRICT
-- table without a rebuild (00032, 00042).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE libraries ADD COLUMN podcast_mode TEXT NOT NULL DEFAULT 'link'
    CHECK (podcast_mode IN ('save', 'link'));
ALTER TABLE libraries ADD COLUMN podcast_scope TEXT NOT NULL DEFAULT 'new'
    CHECK (podcast_scope IN ('new', 'last_n', 'all'));
ALTER TABLE libraries ADD COLUMN podcast_last_n INTEGER NOT NULL DEFAULT 10
    CHECK (podcast_last_n BETWEEN 1 AND 100);
ALTER TABLE libraries ADD COLUMN podcast_parallel INTEGER NOT NULL DEFAULT 2
    CHECK (podcast_parallel BETWEEN 1 AND 5);
ALTER TABLE libraries ADD COLUMN podcast_root_id INTEGER REFERENCES library_roots(id) ON DELETE SET NULL;

CREATE TABLE podcast_settings (
    item_id         TEXT    PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
    mode            TEXT    CHECK (mode IS NULL OR mode IN ('save', 'link')),
    scope           TEXT    CHECK (scope IS NULL OR scope IN ('new', 'last_n', 'all')),
    last_n          INTEGER CHECK (last_n IS NULL OR last_n BETWEEN 1 AND 100),
    pending_root_id INTEGER REFERENCES library_roots(id) ON DELETE SET NULL,
    pending_folder  TEXT,
    updated_at      INTEGER NOT NULL
) STRICT;

CREATE TABLE podcast_download_failures (
    item_id         TEXT    PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
    error           TEXT    NOT NULL,
    attempts        INTEGER NOT NULL DEFAULT 1 CHECK (attempts >= 1),
    first_failed_at INTEGER NOT NULL,
    last_failed_at  INTEGER NOT NULL
) STRICT;

-- A folder whose content changes away from podcast is no target for new subscriptions any more.
-- +goose StatementBegin
CREATE TRIGGER library_roots_podcast_target AFTER UPDATE OF content ON library_roots
WHEN OLD.content = 'podcast' AND NEW.content <> 'podcast'
BEGIN
    UPDATE libraries SET podcast_root_id = NULL WHERE podcast_root_id = NEW.id;
    UPDATE podcast_settings SET pending_root_id = NULL, pending_folder = NULL WHERE pending_root_id = NEW.id;
END;
-- +goose StatementEnd

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
