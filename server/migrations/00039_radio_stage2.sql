-- Webradio, stage 2 (concept 16 sections 5.4, 8, 10.2 and 12.2; ADR 0025): the admin's tag rules,
-- the cover cache, the cover switches of the settings and the start page row «Meine Sender». Times
-- are Unix milliseconds. Package radio owns the radio tables; none names a title (rule 2).
--
--   radio_tag_rules  the admin's rules «tag → facet value» (5.4). They override the embedded
--                    default rules (server/internal/radio/rules/default.json) per tag and facet:
--                    (tag, facet, value) maps the tag in that facet to value, (tag, facet, NULL)
--                    takes the default mapping of the tag in that facet away. (tag, 'ignore', NULL)
--                    takes a meaningless tag («fm», «online») off the worklist and changes no
--                    facet. tag is folded like the stored tags (rules.Fold); the values are checked
--                    against rules.Values by the server.
--   radio_tracks     the cover cache (8.4): one row per normalised «artist|title» (or 'url:' and the
--                    SHA-256 of an image address a stream announced in StreamUrl), found with the
--                    source of the image and its content hash, or none. A hit is kept 30 days, a
--                    miss 24 hours (expires_at); the images live below <cache>/radio/covers/.
--   radio_config     two new columns: cover_musicbrainz and cover_itunes switch the two steps of the
--                    cover chain (paper 11 «Einstellungen»), both on.
--   indexes          radio_favorites and radio_listen_sessions by station: deleting a station
--                    (an own one, or a catalog one 7 days after it left the catalog) cascades into
--                    both, and without them each delete scanned the two tables.
--   home_rows        the new key radio («Meine Sender», 10.2). SQLite cannot change a CHECK
--                    constraint, so the table is rebuilt; every row keeps its id, key, library,
--                    position, hidden flag and time. The row radio is added hidden at the end (F-17).
--                    The trigger home_rows_library_added (00035) names home_rows, so it is dropped
--                    before the rebuild and created again after it, unchanged.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE radio_tag_rules (
    tag        TEXT    NOT NULL CHECK (length(tag) BETWEEN 1 AND 100),
    facet      TEXT    NOT NULL CHECK (facet IN ('genre', 'kind', 'mood', 'ignore')),
    value      TEXT    CHECK (value IS NULL OR length(value) BETWEEN 1 AND 40),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (tag, facet),
    CHECK (facet <> 'ignore' OR value IS NULL)
) STRICT, WITHOUT ROWID;

CREATE TABLE radio_tracks (
    id         INTEGER PRIMARY KEY,
    key        TEXT    NOT NULL UNIQUE CHECK (length(key) BETWEEN 1 AND 600),
    artist     TEXT,
    title      TEXT,
    state      TEXT    NOT NULL CHECK (state IN ('found', 'none')),
    source     TEXT    CHECK (source IN ('stream', 'musicbrainz', 'itunes')),
    image_hash TEXT,
    checked_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    CHECK ((state = 'found') = (source IS NOT NULL AND image_hash IS NOT NULL))
) STRICT;
CREATE INDEX radio_tracks_expires ON radio_tracks (expires_at);

ALTER TABLE radio_config ADD COLUMN cover_musicbrainz INTEGER NOT NULL DEFAULT 1 CHECK (cover_musicbrainz IN (0, 1));
ALTER TABLE radio_config ADD COLUMN cover_itunes INTEGER NOT NULL DEFAULT 1 CHECK (cover_itunes IN (0, 1));

CREATE INDEX radio_favorites_station ON radio_favorites (station_id);
CREATE INDEX radio_listen_sessions_station ON radio_listen_sessions (station_id);

DROP TRIGGER home_rows_library_added;

CREATE TABLE home_rows_new (
    id         TEXT    PRIMARY KEY CHECK (id = CASE WHEN key = 'library' THEN 'library:' || library_id ELSE key END),
    key        TEXT    NOT NULL CHECK (key IN ('spotlight', 'picks', 'continue', 'recent', 'released',
                                               'watchlist', 'collections', 'genres', 'libraries', 'library',
                                               'radio')),
    library_id INTEGER REFERENCES libraries (id) ON DELETE CASCADE,
    position   INTEGER NOT NULL,
    hidden     INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)),
    updated_at INTEGER NOT NULL,
    CHECK ((key = 'library') = (library_id IS NOT NULL))
) STRICT;

INSERT INTO home_rows_new (id, key, library_id, position, hidden, updated_at)
SELECT id, key, library_id, position, hidden, updated_at FROM home_rows;

-- «Meine Sender» behind the last row, hidden (F-17). The constant time keeps a fresh database byte
-- for byte the same (storetest template).
INSERT INTO home_rows_new (id, key, library_id, position, hidden, updated_at)
SELECT 'radio', 'radio', NULL, COALESCE(max(position), -1) + 1, 1, 0 FROM home_rows;

DROP TABLE home_rows;
ALTER TABLE home_rows_new RENAME TO home_rows;

-- A new library gets its row «Neu in …», hidden, at the end (00035, unchanged).
-- +goose StatementBegin
CREATE TRIGGER home_rows_library_added AFTER INSERT ON libraries
BEGIN
    INSERT INTO home_rows (id, key, library_id, position, hidden, updated_at)
    VALUES ('library:' || NEW.id, 'library', NEW.id, (SELECT COALESCE(max(position), -1) + 1 FROM home_rows), 1, NEW.created_at);
END;
-- +goose StatementEnd

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
