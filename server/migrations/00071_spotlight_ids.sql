-- Concept 25 stage 3 «Spotlight und Startseite» (08-datenmodell-api.md sections 2.11 and 3.2, row
-- spotlight_ids; 05-spotlight-integration.md section 3.3, N-60 to N-62): every spotlight entry
-- keeps the three ids Trakt names, so package visibility recognises library titles by TVDB and IMDb
-- id as well (films TMDB, then IMDb; series TVDB, then TMDB, then IMDb).
--
--   tvdb_id   the TVDB id from Trakt ids.tvdb; NULL when Trakt names none.
--   imdb_id   the IMDb id from Trakt ids.imdb (tt...); NULL when Trakt names none.
--   tmdb_id   now NULL for a show Trakt names by its TVDB id only and TMDB «find» does not know:
--             the entry is «nicht zuordenbar» (unmatched, N-62). It is not requestable (Seerr needs
--             the TMDB id), has no override (spotlight_overrides is keyed by the TMDB id) and still
--             shows a library series with that TVDB id. A film always has a TMDB id.
--
-- The papers name ADD COLUMN, but tmdb_id was NOT NULL and SQLite has no ALTER COLUMN, so the table
-- is rebuilt. It is a pure cache (every Trakt run replaces it as a whole), no foreign key, view or
-- trigger names it; the rows are copied with NULL for the new ids, the next run fills them.
-- spotlight_entries_tmdb comes back unchanged next to the two new indexes. The rebuild of
-- home_rows (row wishes, the same row of section 3.2) is not part of this migration: it changes
-- the contract enum HomeRowKey and comes with its own migration.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE spotlight_entries_new (
    list_key   TEXT    NOT NULL,
    media      TEXT    NOT NULL CHECK (media IN ('movie', 'show')),
    position   INTEGER NOT NULL,
    tmdb_id    INTEGER CHECK (tmdb_id IS NULL OR tmdb_id > 0),
    title      TEXT,
    year       INTEGER,
    fetched_at INTEGER NOT NULL,
    tvdb_id    INTEGER CHECK (tvdb_id IS NULL OR tvdb_id > 0),
    imdb_id    TEXT    CHECK (imdb_id IS NULL OR imdb_id <> ''),
    CHECK (tmdb_id IS NOT NULL OR (media = 'show' AND tvdb_id IS NOT NULL)),
    PRIMARY KEY (list_key, media, position)
) STRICT, WITHOUT ROWID;

INSERT INTO spotlight_entries_new (list_key, media, position, tmdb_id, title, year, fetched_at)
SELECT list_key, media, position, tmdb_id, title, year, fetched_at FROM spotlight_entries;

DROP TABLE spotlight_entries;
ALTER TABLE spotlight_entries_new RENAME TO spotlight_entries;

CREATE INDEX spotlight_entries_tmdb ON spotlight_entries (media, tmdb_id);
CREATE INDEX spotlight_entries_tvdb ON spotlight_entries (media, tvdb_id);
CREATE INDEX spotlight_entries_imdb ON spotlight_entries (media, imdb_id);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
