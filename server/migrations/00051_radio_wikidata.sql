-- Radio details from Wikidata (1.30.0, concept 19 part B stage B2, B7.3, B10): owner and founding
-- date of the stations in the offer, matched locally through their homepage. Times are Unix
-- milliseconds. Package radio owns the radio tables; none names a title (rule 2).
--
--   radio_station_wikidata  one row per station of the offer with a unique Wikidata match: the
--                           item (qid), its owner (P127, item and label in the server's language,
--                           else English) when the item names exactly one, and its founding date
--                           (P571) as written with its precision: YYYY, YYYY-MM or YYYY-MM-DD. A
--                           station that goes takes its row along; a station that leaves the
--                           offer loses it (the server deletes it, B7.1).
--   radio_config            wikidata_enabled is the setting «Wikidata verwenden» (on by default,
--                           like the cover switches); wikidata_synced_at the end of the last
--                           successful run of radio.wikidata.sync (an apply of the offer queues
--                           a run only when it is more than 24 hours ago).
--
-- The number 00050 belongs to the image-saving patch 1.28.1, which merges first or leaves a gap.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE radio_station_wikidata (
    station_id INTEGER PRIMARY KEY REFERENCES radio_stations (id) ON DELETE CASCADE,
    qid        TEXT    NOT NULL CHECK (qid GLOB 'Q[1-9]*' AND length(qid) <= 20),
    owner_qid  TEXT    CHECK (owner_qid GLOB 'Q[1-9]*' AND length(owner_qid) <= 20),
    owner      TEXT    CHECK (length(owner) BETWEEN 1 AND 200),
    founded    TEXT    CHECK (founded GLOB '[0-9][0-9][0-9][0-9]'
                           OR founded GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]'
                           OR founded GLOB '[0-9][0-9][0-9][0-9]-[0-1][0-9]-[0-3][0-9]'),
    fetched_at INTEGER NOT NULL,
    CHECK (owner IS NULL OR owner_qid IS NOT NULL)
) STRICT;

ALTER TABLE radio_config ADD COLUMN wikidata_enabled INTEGER NOT NULL DEFAULT 1 CHECK (wikidata_enabled IN (0, 1));
ALTER TABLE radio_config ADD COLUMN wikidata_synced_at INTEGER;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
