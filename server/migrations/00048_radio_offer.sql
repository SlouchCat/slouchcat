-- Radio offer (1.28.0, concept 19 part B stage B1, B4 and B10): the app shows only the stations of
-- the offer instead of the whole catalog, and nothing is assigned by hand any more (B5). Times are
-- Unix milliseconds. Package radio owns the radio tables; none names a title (rule 2).
--
--   radio_offer_countries   the chosen countries with their mode: 'all' (every station of the
--                           country, new ones included) or 'marked' (only the stations marked by an
--                           exception).
--   radio_offer_exceptions  one exception per station, independent of the country (B4.1): offered 1
--                           (marked) or 0 (demarked). A station that goes takes its exception along.
--   radio_config            offer_version counts the stored offers (preview and apply, B4.3);
--                           offer_seeded is 0 until the server prepared the offer at the first start
--                           after this migration (B4.4: every hidden station demarked, the filter
--                           values of every station derived again, the provider filled), which SQL
--                           alone cannot do. hide_broken stays and belongs to the offer now.
--   radio_stations.provider the registrable domain of the station's best stream (B7.4).
--
-- The rules of the administration, the manual filter values and the locks genre, kind and mood go
-- (B5, OB-4); the locks name, logo, region and stream stay. radio_config.countries and
-- radio_stations.hidden stay in the tables and are no longer read (like 'ahead' of 00042): no
-- rebuild of the tables. The offer does not take over countries: it starts empty (OB-2).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE radio_offer_countries (
    country    TEXT    PRIMARY KEY CHECK (country GLOB '[A-Z][A-Z]'),
    mode       TEXT    NOT NULL CHECK (mode IN ('all', 'marked')),
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE radio_offer_exceptions (
    station_id INTEGER PRIMARY KEY REFERENCES radio_stations (id) ON DELETE CASCADE,
    offered    INTEGER NOT NULL CHECK (offered IN (0, 1)),
    updated_at INTEGER NOT NULL
) STRICT;

ALTER TABLE radio_config ADD COLUMN offer_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE radio_config ADD COLUMN offer_seeded INTEGER NOT NULL DEFAULT 0 CHECK (offer_seeded IN (0, 1));
ALTER TABLE radio_stations ADD COLUMN provider TEXT;

DELETE FROM radio_tag_rules;
DELETE FROM radio_station_facets WHERE manual = 1;
DELETE FROM radio_station_locks WHERE field IN ('genre', 'kind', 'mood');

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
