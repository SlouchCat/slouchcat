-- Concept 25 stage 4 «Serien und Komfort» (08-datenmodell-api.md section 2.10): the seasons a
-- series of the library has at TMDB, so the series page can show the missing ones («2 von 4
-- Staffeln vorhanden») and the wishes know how many episodes make a season complete. Package
-- metadata writes it (job series.catalog, store/systemq), package visibility reads it.
--
--   series_catalog  one row per series item with a TMDB id. tmdb_id is the id the row was fetched
--                   for (a changed id of the item makes the row due again). status is TMDB's state
--                   ("Returning Series", "Ended", ...), NULL = unknown; in_production 0/1. seasons
--                   is the JSON array [{number, episodeCount, airDate}] as TMDB lists it, season 0
--                   (specials) included: the readers leave it out. next_episode and last_episode
--                   are {season, episode, airDate} like external_titles. A series TMDB does not
--                   know keeps (or gets) a row without seasons, so the job does not ask again
--                   before the row is due. fetched_at is the last fetch in Unix milliseconds.
--                   The row goes with its item.
--
-- A new table only, nothing is rebuilt; the frame of the template stays.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE series_catalog (
    item_id       TEXT    NOT NULL PRIMARY KEY REFERENCES items (id) ON DELETE CASCADE,
    tmdb_id       INTEGER NOT NULL CHECK (tmdb_id > 0),
    status        TEXT,
    in_production INTEGER NOT NULL DEFAULT 0 CHECK (in_production IN (0, 1)),
    seasons       TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(seasons) AND json_type(seasons) = 'array'),
    next_episode  TEXT    CHECK (next_episode IS NULL OR json_valid(next_episode)),
    last_episode  TEXT    CHECK (last_episode IS NULL OR json_valid(last_episode)),
    fetched_at    INTEGER NOT NULL
) STRICT;

-- The due rows of the scheduled run, the oldest first.
CREATE INDEX series_catalog_fetched ON series_catalog (fetched_at);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
