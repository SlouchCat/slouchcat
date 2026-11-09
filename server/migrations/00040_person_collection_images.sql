-- Person and collection images (concept 17 stage 3, sections 5.1 and 5.2; ADR 0026). Two tables
-- after the pattern of images (00029_item_images.sql): one row per delivered format, written by the
-- job people.images. The files live below the cache directory (people/<id>/w185.{jpg,webp},
-- collections/<id>/w400.{jpg,webp}), regenerable and outside the backup.
--
--   person_images      the portrait of a person: TMDB w185 as fetched (jpeg) and derived WebP.
--   collection_images  the TMDB poster of a collection: w400 as JPEG and WebP.
--
-- source is the provider path the row derives from (people.profile_path, collections.poster_path),
-- so a changed path is seen. status 'ok' carries the file (rel_path relative to the cache root,
-- content_hash for cache busting, media_type, width, height, bytes, all set and positive); status
-- 'missing' records a failed fetch (404, empty or undecodable answer) as a single jpeg row with
-- none of these, so no run fetches the same 404 again before the row ages out. fetched_at is Unix
-- milliseconds. A CHECK passes on NULL, so the 'ok' rule names IS NOT NULL before each comparison.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE person_images (
    person_id    INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    format       TEXT    NOT NULL CHECK (format IN ('jpeg', 'webp')),
    status       TEXT    NOT NULL CHECK (status IN ('ok', 'missing')),
    source       TEXT    NOT NULL,                 -- people.profile_path the row derives from
    size         TEXT    NOT NULL,                 -- w185
    rel_path     TEXT,                             -- relative to the cache root
    content_hash TEXT,                             -- SHA-256 hex, cache busting only
    media_type   TEXT,                             -- image/jpeg, image/webp
    width        INTEGER,
    height       INTEGER,
    bytes        INTEGER,
    fetched_at   INTEGER NOT NULL,
    PRIMARY KEY (person_id, format),
    CHECK (status <> 'ok' OR (rel_path IS NOT NULL AND content_hash IS NOT NULL AND media_type IS NOT NULL
                              AND width IS NOT NULL AND height IS NOT NULL AND bytes IS NOT NULL
                              AND width > 0 AND height > 0 AND bytes > 0)),
    CHECK (status <> 'missing' OR (format = 'jpeg' AND rel_path IS NULL AND content_hash IS NULL
                                   AND media_type IS NULL AND width IS NULL AND height IS NULL AND bytes IS NULL))
) STRICT, WITHOUT ROWID;

CREATE TABLE collection_images (
    collection_id TEXT    NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    format        TEXT    NOT NULL CHECK (format IN ('jpeg', 'webp')),
    status        TEXT    NOT NULL CHECK (status IN ('ok', 'missing')),
    source        TEXT    NOT NULL,                -- collections.poster_path the row derives from
    size          TEXT    NOT NULL,                -- w400
    rel_path      TEXT,                            -- relative to the cache root
    content_hash  TEXT,                            -- SHA-256 hex, cache busting only
    media_type    TEXT,                            -- image/jpeg, image/webp
    width         INTEGER,
    height        INTEGER,
    bytes         INTEGER,
    fetched_at    INTEGER NOT NULL,
    PRIMARY KEY (collection_id, format),
    CHECK (status <> 'ok' OR (rel_path IS NOT NULL AND content_hash IS NOT NULL AND media_type IS NOT NULL
                              AND width IS NOT NULL AND height IS NOT NULL AND bytes IS NOT NULL
                              AND width > 0 AND height > 0 AND bytes > 0)),
    CHECK (status <> 'missing' OR (format = 'jpeg' AND rel_path IS NULL AND content_hash IS NULL
                                   AND media_type IS NULL AND width IS NULL AND height IS NULL AND bytes IS NULL))
) STRICT, WITHOUT ROWID;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
