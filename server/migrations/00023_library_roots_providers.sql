-- Libraries as families, content and ordered metadata sources per root (ADR 0015, question 13 in
-- docs/planning/04-entscheidungen.md, concept docs/planning/07-bibliotheken.md section 4).
--
--   library_roots.content    what a folder holds: movie, series, audiobook or podcast. It picks the
--                            name parser and the identifier that read the folder; until now that was
--                            libraries.kind, one value for every folder of a library.
--   library_roots.providers  the ordered metadata sources of the folder as a JSON array, for example
--                            ["tvdb","tmdb","anilist"]: the first entry identifies the title (ids,
--                            season numbering), every further one fills what is still empty.
--   items.providers          the list copied from the root when the title was created (a series has
--                            no media file of its own, so it cannot be derived); children inherit it.
--   items.anilist_id         the AniList id next to tmdb_id, imdb_id and tvdb_id.
--   items.feed_origin        who chose the feed of a podcast: 'admin' for a subscription made by hand,
--                            'file' for a feedURL read from a metadata.json. The network guard never
--                            grants the LAN exception to a 'file' feed.
--   item_field_sources       which provider won each field (and language) of a title, so the fill job
--                            is idempotent: a provider further back in the list never overwrites the
--                            value of one further in front. lang '' for fields without a language
--                            (NULL in a primary key is never equal to itself in SQLite).
--   libraries.kind           only the family remains: 'video' (movie and series folders), 'audiobooks'
--                            and 'podcasts'. 'movies' and 'series' are burnt and never reused; a
--                            foreign client that tests kind === 'movies' is meant to break, not to be
--                            silently wrong. SQLite has no ALTER COLUMN for a CHECK, so the table is
--                            rebuilt with its id kept; items, library_roots, profile_libraries,
--                            account_libraries and invitation_libraries keep pointing at the right
--                            row. The view visible_items reads items, profiles, accounts and the two
--                            library rights tables, but never libraries, and no trigger does: the
--                            rebuild follows 00014, which left the view alone as well.
--
-- Existing data: every root takes content and providers from its library ('movies' → movie/["tmdb"],
-- 'series' → series/["tmdb","tvdb"], the order the code used until now, 'audiobooks' →
-- audiobook/["local"]); every title takes its list from its kind; item_field_sources stays empty
-- (the provenance of existing titles is unknown, the first fill job writes it). Two guards in the
-- same run abort before COMMIT when a root does not fit its family or its first provider does not
-- fit its content; a plain SELECT could not do that.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

-- ---------------------------------------------------------------------------------------------
-- library_roots: content and the ordered provider list.
-- ---------------------------------------------------------------------------------------------
ALTER TABLE library_roots ADD COLUMN content TEXT NOT NULL DEFAULT 'movie'
    CHECK (content IN ('movie', 'series', 'audiobook', 'podcast'));
ALTER TABLE library_roots ADD COLUMN providers TEXT NOT NULL DEFAULT '["tmdb"]'
    CHECK (json_valid(providers) AND json_array_length(providers) >= 1);

-- No ELSE on purpose: a root of a podcast library cannot exist before this migration (AddRoot
-- refused it), and if one does, NULL fails the NOT NULL constraint and stops the run.
UPDATE library_roots
SET content = (SELECT CASE l.kind WHEN 'movies' THEN 'movie' WHEN 'series' THEN 'series'
                                  WHEN 'audiobooks' THEN 'audiobook' END
               FROM libraries AS l WHERE l.id = library_roots.library_id),
    providers = (SELECT CASE l.kind WHEN 'movies' THEN '["tmdb"]' WHEN 'series' THEN '["tmdb","tvdb"]'
                                    WHEN 'audiobooks' THEN '["local"]' END
                 FROM libraries AS l WHERE l.id = library_roots.library_id);

-- ---------------------------------------------------------------------------------------------
-- items: the copied list, the AniList id, the origin of a podcast feed; the provenance table.
-- ---------------------------------------------------------------------------------------------
ALTER TABLE items ADD COLUMN providers TEXT NOT NULL DEFAULT '["tmdb"]' CHECK (json_valid(providers));
ALTER TABLE items ADD COLUMN anilist_id INTEGER;
ALTER TABLE items ADD COLUMN feed_origin TEXT CHECK (feed_origin IS NULL OR feed_origin IN ('admin', 'file'));

UPDATE items SET providers = CASE
    WHEN kind IN ('series', 'season', 'episode') THEN '["tmdb","tvdb"]'
    WHEN kind IN ('audiobook', 'podcast', 'podcast_episode') THEN '["local"]'
    ELSE '["tmdb"]' END;
UPDATE items SET feed_origin = 'admin' WHERE kind = 'podcast' AND feed_url IS NOT NULL;

CREATE TABLE item_field_sources (
    item_id  TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    field    TEXT NOT NULL,              -- title, overview, tagline, original_title, genres,
                                         -- runtime_minutes, release_date, own_rating, poster, backdrop
    lang     TEXT NOT NULL DEFAULT '',   -- '' for fields without a language
    provider TEXT NOT NULL,
    PRIMARY KEY (item_id, field, lang)
) STRICT, WITHOUT ROWID;

-- ---------------------------------------------------------------------------------------------
-- libraries: the kind becomes the family. Columns as 00014 left them (PRAGMA table_info on the
-- frozen M11 state: id, name, kind, default_rating, created_at, updated_at, last_scan_state,
-- last_scan_started_at, last_scan_finished_at, last_scan_error, scan_files_seen,
-- scan_files_total, loss_accepted_at); no later migration touched the table.
-- ---------------------------------------------------------------------------------------------
CREATE TABLE libraries_new (
    id                     INTEGER PRIMARY KEY,
    name                   TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    kind                   TEXT    NOT NULL CHECK (kind IN ('video', 'audiobooks', 'podcasts')),
    default_rating         INTEGER CHECK (default_rating IS NULL OR default_rating IN (0, 6, 12, 16, 18)),
    created_at             INTEGER NOT NULL,
    updated_at             INTEGER NOT NULL,
    last_scan_state        TEXT    NOT NULL DEFAULT 'never'
                           CHECK (last_scan_state IN ('never', 'queued', 'running', 'done', 'failed', 'aborted')),
    last_scan_started_at   INTEGER,
    last_scan_finished_at  INTEGER,
    last_scan_error        TEXT,
    scan_files_seen        INTEGER NOT NULL DEFAULT 0,
    scan_files_total       INTEGER NOT NULL DEFAULT 0,
    loss_accepted_at       INTEGER
) STRICT;

INSERT INTO libraries_new (id, name, kind, default_rating, created_at, updated_at, last_scan_state,
                           last_scan_started_at, last_scan_finished_at, last_scan_error, scan_files_seen,
                           scan_files_total, loss_accepted_at)
SELECT id, name, CASE kind WHEN 'movies' THEN 'video' WHEN 'series' THEN 'video' ELSE kind END,
       default_rating, created_at, updated_at, last_scan_state,
       last_scan_started_at, last_scan_finished_at, last_scan_error, scan_files_seen,
       scan_files_total, loss_accepted_at
FROM libraries;

DROP TABLE libraries;
ALTER TABLE libraries_new RENAME TO libraries;

-- ---------------------------------------------------------------------------------------------
-- Guards in the same run, before COMMIT. A row with n > 0 violates the CHECK and ends the run.
-- ---------------------------------------------------------------------------------------------
CREATE TEMP TABLE migration_00023_guard (n INTEGER NOT NULL CHECK (n = 0));

-- Every root fits the family of its library.
INSERT INTO migration_00023_guard (n)
SELECT count(*) FROM library_roots AS r JOIN libraries AS l ON l.id = r.library_id
WHERE NOT ((l.kind = 'video' AND r.content IN ('movie', 'series'))
        OR (l.kind = 'audiobooks' AND r.content = 'audiobook')
        OR (l.kind = 'podcasts' AND r.content = 'podcast'));

-- The first provider of every root identifies its content: tmdb for movies, tmdb or tvdb for
-- series, local for audiobooks and podcasts.
INSERT INTO migration_00023_guard (n)
SELECT count(*) FROM library_roots
WHERE NOT ((content = 'movie' AND json_extract(providers, '$[0]') = 'tmdb')
        OR (content = 'series' AND json_extract(providers, '$[0]') IN ('tmdb', 'tvdb'))
        OR (content IN ('audiobook', 'podcast') AND json_extract(providers, '$[0]') = 'local'));

DROP TABLE migration_00023_guard;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
