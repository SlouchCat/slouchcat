-- Concept 25 stage 5, step 5.1 (08-datenmodell-api.md section 2.15): the cache of the discover
-- lists of the wishes area («Entdecken», «Filme», «Serien»). Only package wishes reads and writes it
-- (store/wishq); the job wishes.discover warms it every hour and deletes rows older than 24 hours.
-- Times are Unix milliseconds.
--
--   wish_discover_pages  one row per list, its parameters, page, language and age step, for
--                        example movies|sort=popularity|genre=|p=2|de|12. items is the JSON array
--                        of the hits in Seerr's order with their basic data only (media, tmdbId,
--                        title, year, posterPath, seerrStatus); a genre list keeps its tiles there.
--                        Ratings and details stay in external_titles. total_pages is Seerr's page
--                        count, NULL when Seerr gave none. fetched_at is the read from Seerr: older
--                        than an hour is read again, and kept as the stale answer while Seerr is
--                        unreachable.
--
-- A new table only, nothing is rebuilt; the frame of the template stays.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE wish_discover_pages (
    cache_key   TEXT    NOT NULL PRIMARY KEY CHECK (cache_key <> '' AND length(cache_key) <= 500),
    items       TEXT    NOT NULL CHECK (json_valid(items) AND json_type(items) = 'array'),
    total_pages INTEGER CHECK (total_pages IS NULL OR total_pages >= 0),
    fetched_at  INTEGER NOT NULL
) STRICT;

CREATE INDEX wish_discover_pages_fetched ON wish_discover_pages (fetched_at);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
