-- Trickplay (concept 18 part A, sections 6 and 8.3; ADR 0027): the setting «Vorschau beim Spulen»
-- of every library, one for films (titles of kind movie) and one for series (kind episode), also
-- in a mixed library. Only video libraries use them; in audiobook and podcast libraries the columns
-- keep their defaults and have no effect. The state of the preview sets lives in the cache folder
-- and in jobs, not here. Existing video libraries start at «Im Voraus» (F-11).
--
--   trickplay_movies        ahead (default) | on_start | off
--   trickplay_movies_keep   1 (default): on_start stores the set for good; 0: only while it plays
--   trickplay_series        ahead (default) | on_start | series_start | off
--   trickplay_series_keep   as trickplay_movies_keep, for episodes
--   trickplay_series_scope  series (default) | season_rest | next_episode: the look-ahead of
--                           series_start (6.4)
--
-- keep is stored for every mode but read only with on_start, scope only with series_start.
-- SQLite adds a column with a CHECK constraint to a STRICT table without a rebuild (00032, 00039).
-- The number 00042 follows 00041 of part B (user picks), built in parallel.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE libraries ADD COLUMN trickplay_movies TEXT NOT NULL DEFAULT 'ahead'
    CHECK (trickplay_movies IN ('ahead', 'on_start', 'off'));
ALTER TABLE libraries ADD COLUMN trickplay_movies_keep INTEGER NOT NULL DEFAULT 1
    CHECK (trickplay_movies_keep IN (0, 1));
ALTER TABLE libraries ADD COLUMN trickplay_series TEXT NOT NULL DEFAULT 'ahead'
    CHECK (trickplay_series IN ('ahead', 'on_start', 'series_start', 'off'));
ALTER TABLE libraries ADD COLUMN trickplay_series_keep INTEGER NOT NULL DEFAULT 1
    CHECK (trickplay_series_keep IN (0, 1));
ALTER TABLE libraries ADD COLUMN trickplay_series_scope TEXT NOT NULL DEFAULT 'series'
    CHECK (trickplay_series_scope IN ('series', 'season_rest', 'next_episode'));

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
