-- Images with source, language and the administrator's choice (concept 10 step 2, ADR 0018):
--   * images is rebuilt after the template: kind learns 'logo', format learns 'png' (the second
--     delivered format of a logo, which keeps its transparency; posters and backdrops stay JPEG
--     and WebP), and provider names the source the row derives from (tmdb, tvdb, anilist, local,
--     fanart, cover, manual). The backfill reads the host of source_url: image.tmdb.org -> tmdb,
--     artworks.thetvdb.com -> tvdb, anilist.co -> anilist, file:/embedded: prefixes -> cover,
--     anything else -> tmdb (the only provider before M3). lang stays as stored (NULL = unknown,
--     the state of every row before this step; '' = without text from now on).
--   * item_images holds the decided source per title and kind, the single input of image.fetch:
--     provider, lang ('' = without text or unknown), source (the provider's address, or
--     file:<rel>?<size>-<mtime> for a local file) and chosen (1 = picked by the administrator in
--     the image choice, which also locks the kind in items.locked_fields). The backfill carries
--     items.tmdb_poster_path and tmdb_backdrop_path over (provider by host, lang ''); those two
--     columns stay on items unused from now on (their removal is a later rebuild of items).
--   * item_local_images lists the image files the scanner found next to the media (poster.jpg,
--     fanart.jpg, logo.png, ...): relative to the root, readable through package paths only,
--     with the dimensions from the file header and size/mtime for change detection. Empty here;
--     the next scan fills it.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE images_new (
    id           INTEGER PRIMARY KEY,
    item_id      TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    kind         TEXT    NOT NULL CHECK (kind IN ('poster', 'backdrop', 'logo')),
    size         TEXT    NOT NULL,                 -- w200 … w1920 or 'original'
    format       TEXT    NOT NULL DEFAULT 'jpeg' CHECK (format IN ('jpeg', 'webp', 'png')),
    lang         TEXT,                             -- '' = without text, NULL = unknown (rows before step 2)
    provider     TEXT    NOT NULL DEFAULT 'tmdb',  -- tmdb, tvdb, anilist, local, fanart, cover, manual
    source_url   TEXT    NOT NULL,                 -- address of the original the row derives from
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

INSERT INTO images_new (id, item_id, kind, size, format, lang, provider, source_url, rel_path, content_hash, media_type,
                        width, height, bytes, accent, created_at)
SELECT id, item_id, kind, size, format, lang,
       CASE
         WHEN source_url LIKE 'https://image.tmdb.org/%' THEN 'tmdb'
         WHEN source_url LIKE 'https://artworks.thetvdb.com/%' THEN 'tvdb'
         WHEN source_url LIKE 'https://s4.anilist.co/%' OR source_url LIKE 'https://%anilist.co/%' THEN 'anilist'
         WHEN source_url LIKE 'file:%' OR source_url LIKE 'embedded:%' THEN 'cover'
         WHEN source_url LIKE 'http://%' OR source_url LIKE 'https://%' THEN 'cover'
         ELSE 'tmdb'
       END,
       source_url, rel_path, content_hash, media_type, width, height, bytes, accent, created_at
FROM images;

DROP TABLE images;
ALTER TABLE images_new RENAME TO images;

CREATE TABLE item_images (
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL CHECK (kind IN ('poster', 'backdrop', 'logo')),
    provider   TEXT    NOT NULL,
    lang       TEXT    NOT NULL DEFAULT '',
    source     TEXT    NOT NULL,
    chosen     INTEGER NOT NULL DEFAULT 0 CHECK (chosen IN (0, 1)),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (item_id, kind)
) STRICT, WITHOUT ROWID;

INSERT INTO item_images (item_id, kind, provider, lang, source, chosen, updated_at)
SELECT id, 'poster',
       CASE WHEN tmdb_poster_path LIKE '/%' THEN 'tmdb'
            WHEN tmdb_poster_path LIKE 'https://artworks.thetvdb.com/%' THEN 'tvdb'
            WHEN tmdb_poster_path LIKE 'https://%anilist.co/%' THEN 'anilist'
            ELSE 'tmdb' END,
       '', tmdb_poster_path, 0, updated_at
FROM items WHERE tmdb_poster_path IS NOT NULL AND tmdb_poster_path <> '' AND deleted_at IS NULL;

INSERT INTO item_images (item_id, kind, provider, lang, source, chosen, updated_at)
SELECT id, 'backdrop',
       CASE WHEN tmdb_backdrop_path LIKE '/%' THEN 'tmdb'
            WHEN tmdb_backdrop_path LIKE 'https://artworks.thetvdb.com/%' THEN 'tvdb'
            WHEN tmdb_backdrop_path LIKE 'https://%anilist.co/%' THEN 'anilist'
            ELSE 'tmdb' END,
       '', tmdb_backdrop_path, 0, updated_at
FROM items WHERE tmdb_backdrop_path IS NOT NULL AND tmdb_backdrop_path <> '' AND deleted_at IS NULL;

CREATE TABLE item_local_images (
    item_id  TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    kind     TEXT    NOT NULL CHECK (kind IN ('poster', 'backdrop', 'logo')),
    rel_path TEXT    NOT NULL,                     -- relative to the root of the title's files
    root_id  INTEGER NOT NULL REFERENCES library_roots(id) ON DELETE CASCADE,
    width    INTEGER NOT NULL,
    height   INTEGER NOT NULL,
    size     INTEGER NOT NULL,
    mtime    INTEGER NOT NULL,
    PRIMARY KEY (item_id, kind, rel_path)
) STRICT, WITHOUT ROWID;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
