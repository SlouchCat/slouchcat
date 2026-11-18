-- Concept 25 stage 2 «Anfordern» (08-datenmodell-api.md section 2.9): titles outside the library,
-- read from TMDB for the preview page, the search, the wishes and the administration. These tables
-- are not items: a missing title has no item, no file and no profile scope of its own. Only package
-- metadata reads and writes them (store/externalq, depguard rule "externalq"); visibility reads
-- min_age next to items in its own queries (MissingTitle).
--
--   external_titles        one row per TMDB movie or series (media 'show'), keyed by its TMDB id.
--                          min_age NULL = unknown (counts as 18); metadata computes it from the raw
--                          certifications (country -> TMDB certificate) with ageFromCertifications
--                          in the country order of ADR 0034, again after a change of the rating
--                          country. genres is a JSON array [{id, names: {lang: name}}] (the names per
--                          text language); trailers [{lang, key, official, size}] (YouTube keys,
--                          lang "" = without language); seasons [{number, episodeCount, airDate}]
--                          only for a show; next_episode and last_episode {season, episode, airDate}.
--                          The TMDB image paths never reach a browser: the images go through
--                          Slouchcat's own route. fetched_at is the last fetch in Unix milliseconds.
--   external_title_texts   title, overview and tagline per text language.
--   external_title_images  the scaled images in the cache directory (pattern images and 00040):
--                          one row per kind, size and format, source_url the TMDB original it was
--                          made from, rel_path relative to the cache root.
--
-- New tables only, nothing is rebuilt; the frame of the template stays.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE external_titles (
    media           TEXT    NOT NULL CHECK (media IN ('movie', 'show')),
    tmdb_id         INTEGER NOT NULL CHECK (tmdb_id > 0),
    tvdb_id         INTEGER CHECK (tvdb_id IS NULL OR tvdb_id > 0),
    imdb_id         TEXT    CHECK (imdb_id IS NULL OR imdb_id <> ''),
    original_title  TEXT,
    year            INTEGER,
    release_date    TEXT    CHECK (release_date IS NULL OR release_date GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),
    runtime_minutes INTEGER CHECK (runtime_minutes IS NULL OR runtime_minutes > 0),
    genres          TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(genres) AND json_type(genres) = 'array'),
    certifications  TEXT    NOT NULL DEFAULT '{}' CHECK (json_valid(certifications) AND json_type(certifications) = 'object'),
    min_age         INTEGER CHECK (min_age IS NULL OR min_age BETWEEN 0 AND 18),
    poster_path     TEXT,
    backdrop_path   TEXT,
    logo_path       TEXT,
    trailers        TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(trailers) AND json_type(trailers) = 'array'),
    status          TEXT,
    seasons         TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(seasons) AND json_type(seasons) = 'array'
                                                         AND (media = 'show' OR seasons = '[]')),
    next_episode    TEXT    CHECK (next_episode IS NULL OR (media = 'show' AND json_valid(next_episode))),
    last_episode    TEXT    CHECK (last_episode IS NULL OR (media = 'show' AND json_valid(last_episode))),
    fetched_at      INTEGER NOT NULL,
    PRIMARY KEY (media, tmdb_id)
) STRICT, WITHOUT ROWID;

CREATE INDEX external_titles_tvdb ON external_titles (tvdb_id);
CREATE INDEX external_titles_imdb ON external_titles (imdb_id);

CREATE TABLE external_title_texts (
    media    TEXT    NOT NULL,
    tmdb_id  INTEGER NOT NULL,
    language TEXT    NOT NULL CHECK (language <> ''),
    title    TEXT    NOT NULL,
    overview TEXT,
    tagline  TEXT,
    PRIMARY KEY (media, tmdb_id, language),
    FOREIGN KEY (media, tmdb_id) REFERENCES external_titles (media, tmdb_id) ON DELETE CASCADE
) STRICT, WITHOUT ROWID;

CREATE TABLE external_title_images (
    id           INTEGER PRIMARY KEY,
    media        TEXT    NOT NULL,
    tmdb_id      INTEGER NOT NULL,
    kind         TEXT    NOT NULL CHECK (kind IN ('poster', 'backdrop', 'logo')),
    size         TEXT    NOT NULL,                 -- w400
    format       TEXT    NOT NULL CHECK (format IN ('jpeg', 'png', 'webp')),
    lang         TEXT,                             -- language of the TMDB image, NULL = without text
    source_url   TEXT    NOT NULL,                 -- the TMDB original the row was scaled from
    rel_path     TEXT    NOT NULL,                 -- relative to the cache root
    content_hash TEXT    NOT NULL,                 -- SHA-256 hex, cache busting only
    media_type   TEXT    NOT NULL,                 -- image/jpeg, image/png, image/webp
    width        INTEGER NOT NULL CHECK (width > 0),
    height       INTEGER NOT NULL CHECK (height > 0),
    bytes        INTEGER NOT NULL CHECK (bytes > 0),
    fetched_at   INTEGER NOT NULL,
    UNIQUE (media, tmdb_id, kind, size, format),
    FOREIGN KEY (media, tmdb_id) REFERENCES external_titles (media, tmdb_id) ON DELETE CASCADE
) STRICT;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
