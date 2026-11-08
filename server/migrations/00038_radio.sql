-- Webradio, stage 1 (concept 16, sections 5, 7 and 12.2; ADR 0025). New tables only; no existing
-- table changes, so nothing is rebuilt. Times are Unix milliseconds. Package radio owns all of them;
-- none names a title, so the queries live in appq (rule 2 stays untouched).
--
--   radio_stations         one station as the app shows it: its own id (AUTOINCREMENT, never reused,
--                          because clients keep ids in URLs and favourites), source catalog or own,
--                          group_key (folded name | country | host of the homepage, fixed when the
--                          station is created, 5.3), name, sort and search keys, country (ISO 3166-1),
--                          region (ISO 3166-2), languages (JSON array of ISO 639-1), popularity, the
--                          derived quality, reachable (any catalog entry had lastcheckok), the own
--                          relay aborts in a row and broken_at (set at three, 7.4), the admin marks
--                          hidden and recommended, shows_titles (a relay saw stream titles),
--                          missing_since (every catalog entry gone, 5.2) and the state of the logo
--                          chain (section 6).
--   radio_station_aliases  the Radio Browser entries (stationuuid) that make up a station. An alias
--                          never moves to another station, so the station id stays stable (5.3).
--                          catalog_changed is the lastchangetime of the last import: the daily sync
--                          skips an entry that did not change.
--   radio_streams          the streams of a station, one per catalog entry (stationuuid NULL for an
--                          own station): address, codec, bitrate, HLS, lastcheckok and the admin's
--                          preferred stream (lock 'stream'). The best stream is the first row of
--                          ListRadioStreamsRanked.
--   radio_station_tags     raw catalog tags of a station (lower-cased, trimmed), input of the rules.
--   radio_station_facets   derived filter values (genre, kind, mood); manual = set by the admin.
--   radio_station_locks    fields the admin changed by hand (5.5); the sync never overwrites them.
--   radio_favorites        favourites of a profile in the profile's order; removed with the profile
--                          and with the station (F-18).
--   radio_profile_state    per profile: default country (F-12), last filter (JSON) and sort.
--   radio_listen_sessions  listen sessions (7.5): ULID id, the login session, account, profile and
--                          station, liveness (last_seen_at, expires_at), whether the client plays,
--                          and how it ended (the model.StoppedBy* values plus radio's own
--                          'expired', 'station' and 'restart').
--   radio_listen_time      listened time per profile and quarter hour of server time (quarter =
--                          server ms / 900000, like SumPlayedMsByQuarterHour); the K-5 daily limit
--                          adds it to the played time of titles (7.10, F-16).
--   radio_config           one row (id 1), like spotlight_config: buffer minutes, byte budget, maximum
--                          number of relays, grace after the last listener, hide broken stations,
--                          countries in the catalog (JSON array, NULL = all) and the state of the
--                          last catalog sync.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE radio_stations (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    source            TEXT    NOT NULL DEFAULT 'catalog' CHECK (source IN ('catalog', 'own')),
    group_key         TEXT    NOT NULL,
    name              TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    sort_key          TEXT    NOT NULL,
    search_text       TEXT    NOT NULL,
    country           TEXT    CHECK (country IS NULL OR length(country) = 2),
    region            TEXT,
    languages         TEXT    NOT NULL DEFAULT '[]',
    homepage          TEXT,
    favicon_url       TEXT,
    votes             INTEGER NOT NULL DEFAULT 0,
    clicks            INTEGER NOT NULL DEFAULT 0,
    quality           TEXT    CHECK (quality IN ('high', 'good', 'basic')),
    reachable         INTEGER NOT NULL DEFAULT 1 CHECK (reachable IN (0, 1)),
    failures          INTEGER NOT NULL DEFAULT 0,
    broken_at         INTEGER,
    hidden            INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)),
    recommended       INTEGER NOT NULL DEFAULT 0 CHECK (recommended IN (0, 1)),
    shows_titles      INTEGER NOT NULL DEFAULT 0 CHECK (shows_titles IN (0, 1)),
    missing_since     INTEGER,
    logo_state        TEXT    NOT NULL DEFAULT 'none'
                              CHECK (logo_state IN ('none', 'queued', 'catalog', 'website', 'own', 'failed')),
    logo_hash         TEXT,
    logo_tried_at     INTEGER,
    created_at        INTEGER NOT NULL,
    updated_at        INTEGER NOT NULL
) STRICT;
CREATE INDEX radio_stations_group ON radio_stations (group_key);
CREATE INDEX radio_stations_country ON radio_stations (country, hidden);

CREATE TABLE radio_station_aliases (
    stationuuid     TEXT    PRIMARY KEY,
    station_id      INTEGER NOT NULL REFERENCES radio_stations (id) ON DELETE CASCADE,
    seen_at         INTEGER NOT NULL,
    catalog_changed TEXT,
    missing_since   INTEGER
) STRICT;
CREATE INDEX radio_station_aliases_station ON radio_station_aliases (station_id);

CREATE TABLE radio_streams (
    id           INTEGER PRIMARY KEY,
    station_id   INTEGER NOT NULL REFERENCES radio_stations (id) ON DELETE CASCADE,
    stationuuid  TEXT    UNIQUE REFERENCES radio_station_aliases (stationuuid) ON DELETE CASCADE,
    url          TEXT    NOT NULL,
    url_resolved TEXT,
    codec        TEXT,
    bitrate      INTEGER,
    hls          INTEGER NOT NULL DEFAULT 0 CHECK (hls IN (0, 1)),
    lastcheckok  INTEGER NOT NULL DEFAULT 1 CHECK (lastcheckok IN (0, 1)),
    preferred    INTEGER NOT NULL DEFAULT 0 CHECK (preferred IN (0, 1)),
    updated_at   INTEGER NOT NULL
) STRICT;
CREATE INDEX radio_streams_station ON radio_streams (station_id);

CREATE TABLE radio_station_tags (
    station_id INTEGER NOT NULL REFERENCES radio_stations (id) ON DELETE CASCADE,
    tag        TEXT    NOT NULL,
    PRIMARY KEY (station_id, tag)
) STRICT, WITHOUT ROWID;
CREATE INDEX radio_station_tags_tag ON radio_station_tags (tag);

CREATE TABLE radio_station_facets (
    station_id INTEGER NOT NULL REFERENCES radio_stations (id) ON DELETE CASCADE,
    facet      TEXT    NOT NULL CHECK (facet IN ('genre', 'kind', 'mood')),
    value      TEXT    NOT NULL,
    manual     INTEGER NOT NULL DEFAULT 0 CHECK (manual IN (0, 1)),
    PRIMARY KEY (station_id, facet, value)
) STRICT, WITHOUT ROWID;

CREATE TABLE radio_station_locks (
    station_id INTEGER NOT NULL REFERENCES radio_stations (id) ON DELETE CASCADE,
    field      TEXT    NOT NULL CHECK (field IN ('name', 'logo', 'genre', 'kind', 'mood', 'region', 'stream')),
    PRIMARY KEY (station_id, field)
) STRICT, WITHOUT ROWID;

CREATE TABLE radio_favorites (
    profile_id INTEGER NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    station_id INTEGER NOT NULL REFERENCES radio_stations (id) ON DELETE CASCADE,
    position   INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (profile_id, station_id)
) STRICT, WITHOUT ROWID;

CREATE TABLE radio_profile_state (
    profile_id      INTEGER PRIMARY KEY REFERENCES profiles (id) ON DELETE CASCADE,
    default_country TEXT    CHECK (default_country IS NULL OR length(default_country) = 2),
    last_filter     TEXT,
    last_sort       TEXT    CHECK (last_sort IS NULL OR last_sort IN ('popular', 'name', 'quality', 'new')),
    updated_at      INTEGER NOT NULL
) STRICT;

CREATE TABLE radio_listen_sessions (
    id           TEXT    PRIMARY KEY CHECK (length(id) = 26),
    session_id   INTEGER NOT NULL REFERENCES sessions (id) ON DELETE CASCADE,
    account_id   INTEGER NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    profile_id   INTEGER NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    station_id   INTEGER NOT NULL REFERENCES radio_stations (id) ON DELETE CASCADE,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,
    playing      INTEGER NOT NULL DEFAULT 1 CHECK (playing IN (0, 1)),
    ended_at     INTEGER,
    ended_by     TEXT    CHECK (ended_by IN ('client', 'logout', 'revoked', 'limit', 'admin', 'expired',
                                             'station', 'restart'))
) STRICT;
CREATE INDEX radio_listen_sessions_open ON radio_listen_sessions (station_id) WHERE ended_at IS NULL;
CREATE INDEX radio_listen_sessions_profile ON radio_listen_sessions (profile_id) WHERE ended_at IS NULL;

CREATE TABLE radio_listen_time (
    profile_id INTEGER NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    quarter    INTEGER NOT NULL,
    played_ms  INTEGER NOT NULL CHECK (played_ms BETWEEN 0 AND 900000),
    PRIMARY KEY (profile_id, quarter)
) STRICT, WITHOUT ROWID;

CREATE TABLE radio_config (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    buffer_minutes    INTEGER NOT NULL DEFAULT 30 CHECK (buffer_minutes IN (10, 30, 60)),
    budget_mb         INTEGER NOT NULL DEFAULT 512 CHECK (budget_mb BETWEEN 64 AND 4096),
    max_relays        INTEGER NOT NULL DEFAULT 10 CHECK (max_relays BETWEEN 1 AND 50),
    grace_seconds     INTEGER NOT NULL DEFAULT 120 CHECK (grace_seconds BETWEEN 0 AND 1800),
    hide_broken       INTEGER NOT NULL DEFAULT 1 CHECK (hide_broken IN (0, 1)),
    countries         TEXT,
    last_sync_at      INTEGER,
    last_sync_error   TEXT,
    sync_failures     INTEGER NOT NULL DEFAULT 0,
    entries_total     INTEGER NOT NULL DEFAULT 0,
    entries_reachable INTEGER NOT NULL DEFAULT 0,
    stations_total    INTEGER NOT NULL DEFAULT 0,
    last_added        INTEGER NOT NULL DEFAULT 0,
    last_removed      INTEGER NOT NULL DEFAULT 0,
    updated_at        INTEGER NOT NULL
) STRICT;

-- Without this row the server has no buffer, budget or relay limit; the values are F-13's. The
-- constant updated_at keeps a fresh database byte for byte the same (storetest template).
INSERT INTO radio_config (id, updated_at) VALUES (1, 0);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
