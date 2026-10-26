-- Spotlight (M11, plan 05-plan-spotlight.md section 5.3, ADR 0011). The two fixed Trakt rows
-- become one configurable row and a new row by release date: home_rows swaps trakt_trending and
-- trakt_popular for spotlight and released, so there are still exactly eight keys (E-9).
--
-- home_rows has to be rebuilt because SQLite cannot change a CHECK constraint. The three new
-- tables are created before trakt_lists is dropped: the frozen trending entries move over, so the
-- M7 evidence in testdata/db survives and the row is not empty between the migration and the first
-- run.
--
-- Two named losses: a hidden flag of the two dropped rows is gone (the new rows start visible),
-- and the entries of the list "popular" are not carried over (a cache, refilled by the next run).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

-- ---------------------------------------------------------------------------------------------
-- Start page rows: rebuild with the new key set.
-- ---------------------------------------------------------------------------------------------
CREATE TABLE home_rows_new (
    key        TEXT    PRIMARY KEY CHECK (key IN ('spotlight', 'picks', 'continue', 'recent', 'released',
                                                  'watchlist', 'collections', 'genres')),
    position   INTEGER NOT NULL,
    hidden     INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)),
    updated_at INTEGER NOT NULL
) STRICT;

-- The six rows that stay keep their position and their hidden flag for now; the positions are set
-- below.
INSERT INTO home_rows_new (key, position, hidden, updated_at)
SELECT key, position, hidden, updated_at FROM home_rows
WHERE key NOT IN ('trakt_trending', 'trakt_popular');

-- The two new rows, both visible. Without this insert the table would hold six rows, the start
-- page no stage, and the target "exactly 0 to 7" would be wrong. The positions are placeholders.
INSERT INTO home_rows_new (key, position, hidden, updated_at) VALUES
    ('spotlight', -2, 0, 0),
    ('released', -1, 0, 0);

-- Target positions, given explicitly instead of by a ranking over the new table: home_rows has no
-- UNIQUE on position, and an alphabetical tiebreak would put picks before spotlight. The rank of
-- the six remaining keys is counted over the OLD table (one plus the rows ordered before by
-- position, then key), whose rows carry only their own positions; home_rows_new already holds the
-- two placeholders, which would mix into the ranking. (A window function would say the same, but
-- sqlc's SQLite grammar does not parse one.) The six rows still carry their old positions here.
UPDATE home_rows_new SET position = 0 WHERE key = 'spotlight';
UPDATE home_rows_new SET position = 1 + (
    SELECT count(*) FROM home_rows AS o
    WHERE o.key NOT IN ('trakt_trending', 'trakt_popular')
      AND (o.position < home_rows_new.position
           OR (o.position = home_rows_new.position AND o.key < home_rows_new.key))
) WHERE key NOT IN ('spotlight', 'released');

-- released goes directly behind recent; everything behind recent moves up one. recent itself is
-- never updated here, so the subquery stays stable while the statement runs.
UPDATE home_rows_new SET position = position + 1
WHERE key <> 'released' AND position > (SELECT position FROM home_rows_new WHERE key = 'recent');
UPDATE home_rows_new SET position = (SELECT position FROM home_rows_new WHERE key = 'recent') + 1
WHERE key = 'released';

DROP TABLE home_rows;
ALTER TABLE home_rows_new RENAME TO home_rows;

-- ---------------------------------------------------------------------------------------------
-- Spotlight: configuration, fetched entries and the curation of single entries.
-- ---------------------------------------------------------------------------------------------

-- One row, always id 1. The values that E-7 and AN-3 held as constants live here from now on:
-- how many titles the row shows (5..20), below how many hits it disappears (0..10) and how often
-- the list is fetched (6..48 hours; the staleness limit is twice that). list_url belongs to
-- list_key 'url' only and may name a user; it never reaches a log (rule 10).
CREATE TABLE spotlight_config (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    enabled             INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    list_key            TEXT    NOT NULL CHECK (list_key IN ('trending', 'popular', 'watched_weekly',
                                                             'collected_monthly', 'anticipated',
                                                             'boxoffice', 'url')),
    list_url            TEXT,
    media               TEXT    NOT NULL CHECK (media IN ('movie', 'show', 'both')),
    item_count          INTEGER NOT NULL CHECK (item_count BETWEEN 5 AND 20),
    min_hits            INTEGER NOT NULL CHECK (min_hits BETWEEN 0 AND 10),
    interval_hours      INTEGER NOT NULL CHECK (interval_hours BETWEEN 6 AND 48),
    last_manual_refresh INTEGER,
    updated_at          INTEGER NOT NULL
) STRICT;

-- Without this row the server has no list, no count and no interval, and twice interval_hours
-- would be undefined. The values are today's constants, only writable now.
INSERT INTO spotlight_config (id, enabled, list_key, media, item_count, min_hits, interval_hours, updated_at)
VALUES (1, 1, 'trending', 'both', 20, 5, 24, 0);

-- The fetched list, a pure cache: every successful run replaces it as a whole in one transaction,
-- a failed run empties it and the row disappears (I-7).
CREATE TABLE spotlight_entries (
    list_key   TEXT    NOT NULL,
    media      TEXT    NOT NULL CHECK (media IN ('movie', 'show')),
    position   INTEGER NOT NULL,
    tmdb_id    INTEGER NOT NULL,
    title      TEXT,
    year       INTEGER,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (list_key, media, position)
) STRICT, WITHOUT ROWID;
CREATE INDEX spotlight_entries_tmdb ON spotlight_entries (media, tmdb_id);

-- The curation of single entries, kept apart from the cache: the key names the title and the list,
-- never a position, so a note survives a run that reorders the list and a list change does not tip
-- notes onto foreign titles. No foreign key on items: an entry without a library hit must stay
-- hideable. note is as long as a pick's note (280).
CREATE TABLE spotlight_overrides (
    list_key    TEXT    NOT NULL,
    media       TEXT    NOT NULL CHECK (media IN ('movie', 'show')),
    tmdb_id     INTEGER NOT NULL,
    hidden      INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)),
    pinned_rank INTEGER CHECK (pinned_rank IS NULL OR pinned_rank >= 1),
    note        TEXT    CHECK (note IS NULL OR length(note) <= 280),
    updated_at  INTEGER NOT NULL,
    PRIMARY KEY (list_key, media, tmdb_id)
) STRICT, WITHOUT ROWID;

-- Without this index two entries could sit on rank 1, and the server would resolve the tie by
-- position: silent and inexplicable. The rank counts per list, not per medium, because the row is
-- one single row for media 'both'.
CREATE UNIQUE INDEX spotlight_overrides_rank ON spotlight_overrides (list_key, pinned_rank)
WHERE pinned_rank IS NOT NULL;

-- Carry the frozen state over, so the M7 evidence in testdata/db does not vanish. Only the
-- trending rows, because one singleton configuration holds exactly one list_key.
INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, title, year, fetched_at)
SELECT 'trending', media, position, tmdb_id, title, year, fetched_at
FROM trakt_lists WHERE list = 'trending';

DROP TABLE trakt_lists;

-- ---------------------------------------------------------------------------------------------
-- The new row "recently released" reads items.release_date, which had no index (items has one on
-- added_at, tmdb_id and parent_id).
-- ---------------------------------------------------------------------------------------------
CREATE INDEX items_released ON items (release_date DESC)
WHERE release_date IS NOT NULL AND deleted_at IS NULL;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
