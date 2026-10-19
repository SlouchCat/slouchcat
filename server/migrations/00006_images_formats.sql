-- Image pipeline (plan 3.1, M1b): image.fetch keeps the TMDB original once per kind and derives
-- every delivered size as WebP and JPEG with ffmpeg. The images table gets the delivery format
-- and the accent colour; one row per (item, kind, size, format).
--
--   size    w200, w400, w800 (poster), w780, w1280, w1920 (backdrop), or 'original' for the kept
--           TMDB download (never served; its source_url detects a changed TMDB image). The
--           original row always uses format 'jpeg' as its single slot; media_type holds the type
--           as downloaded (image/jpeg or image/png).
--   format  'webp' or 'jpeg' of a derived row.
--   accent  lower-case #rrggbb from palettegen on the w200 poster; stored on every derived poster
--           row of the title, NULL on backdrops and originals.
--
-- Rows of M1a (poster w500, backdrop w780 and w1280, stored unchanged) are kept as format 'jpeg'.
-- They count as incomplete for QueueIncompleteImages (originals, WebP and the new sizes are
-- missing), so the scan that the server schedules at every start queues image.fetch for each such
-- title; that run replaces them and removes the w500 poster.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;

CREATE TABLE images_new (
    id           INTEGER PRIMARY KEY,
    item_id      TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    kind         TEXT    NOT NULL CHECK (kind IN ('poster', 'backdrop')),
    size         TEXT    NOT NULL,                 -- w200 … w1920 or 'original'
    format       TEXT    NOT NULL DEFAULT 'jpeg' CHECK (format IN ('jpeg', 'webp')),
    lang         TEXT,
    source_url   TEXT    NOT NULL,                 -- URL of the TMDB original the row derives from
    rel_path     TEXT    NOT NULL,                 -- relative to the cache root
    content_hash TEXT    NOT NULL,                 -- SHA-256 hex, cache busting only
    media_type   TEXT    NOT NULL,                 -- image/jpeg, image/png, image/webp
    width        INTEGER NOT NULL,
    height       INTEGER NOT NULL,
    bytes        INTEGER NOT NULL,
    accent       TEXT    CHECK (accent IS NULL OR (length(accent) = 7 AND accent GLOB '#[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]')),
    created_at   INTEGER NOT NULL,
    UNIQUE (item_id, kind, size, format)
) STRICT;

INSERT INTO images_new (id, item_id, kind, size, format, lang, source_url, rel_path, content_hash, media_type,
                        width, height, bytes, accent, created_at)
SELECT id, item_id, kind, size, 'jpeg', lang, source_url, rel_path, content_hash, media_type,
       width, height, bytes, NULL, created_at
FROM images;

DROP TABLE images;
ALTER TABLE images_new RENAME TO images;

PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
