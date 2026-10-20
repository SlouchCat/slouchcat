-- Library complete (plan M3, sections 3.2, 3.4, 5.3; design .omc/plans/m3-design.md). Times are
-- Unix milliseconds (UTC).
--
--   items          series tree numbers (season_number, episode_number, episode_end for a file
--                  that holds several episodes), tvdb_id, extra_kind (kind 'extra'), the TMDB
--                  image paths the match stores for image.fetch, and the trailer state.
--   media_files    probe_error: the last ffprobe failure (B-9 "unplayable").
--   ignored_files  files the administrator removed from the library (B-9 cleanup); the scanner
--                  skips them.
--   trailers       YouTube trailers per title and language (B-8, W-10); manual rows survive refreshes.
--   people/credits cast and crew from the metadata provider (B-3); no images in M3.
--   triggers       container availability and min_child_rating count only seasons, episodes and
--                  podcast episodes: an extra alone makes no series available.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE items ADD COLUMN season_number INTEGER CHECK (season_number IS NULL OR season_number >= 0);
ALTER TABLE items ADD COLUMN episode_number INTEGER CHECK (episode_number IS NULL OR episode_number >= 0);
ALTER TABLE items ADD COLUMN episode_end INTEGER CHECK (episode_end IS NULL OR episode_end >= 0);
ALTER TABLE items ADD COLUMN tvdb_id INTEGER;
ALTER TABLE items ADD COLUMN extra_kind TEXT CHECK (extra_kind IS NULL OR extra_kind IN
    ('trailer', 'featurette', 'deleted_scene', 'behind_the_scenes', 'interview', 'scene', 'short', 'other'));
ALTER TABLE items ADD COLUMN tmdb_poster_path TEXT;
ALTER TABLE items ADD COLUMN tmdb_backdrop_path TEXT;
ALTER TABLE items ADD COLUMN trailer_state TEXT NOT NULL DEFAULT 'pending'
    CHECK (trailer_state IN ('pending', 'found', 'none', 'failed'));
ALTER TABLE items ADD COLUMN trailer_checked_at INTEGER;

CREATE INDEX items_tvdb ON items (tvdb_id);
CREATE INDEX items_episode ON items (parent_id, season_number, episode_number);

ALTER TABLE media_files ADD COLUMN probe_error TEXT;

CREATE TABLE ignored_files (
    root_id    INTEGER NOT NULL REFERENCES library_roots(id) ON DELETE CASCADE,
    rel_path   TEXT    NOT NULL,
    reason     TEXT    NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (root_id, rel_path)
) STRICT, WITHOUT ROWID;

CREATE TABLE trailers (
    id         INTEGER PRIMARY KEY,
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    lang       TEXT    NOT NULL CHECK (length(lang) BETWEEN 2 AND 3),
    youtube_id TEXT    NOT NULL CHECK (length(youtube_id) BETWEEN 8 AND 16),
    title      TEXT,
    source     TEXT    NOT NULL CHECK (source IN ('tmdb', 'youtube', 'ai', 'manual')),
    official   INTEGER NOT NULL DEFAULT 0 CHECK (official IN (0, 1)),
    created_at INTEGER NOT NULL,
    UNIQUE (item_id, lang, youtube_id)
) STRICT;

CREATE INDEX trailers_item ON trailers (item_id, lang);

CREATE TABLE people (
    id           INTEGER PRIMARY KEY,
    tmdb_id      INTEGER UNIQUE,
    name         TEXT    NOT NULL,
    profile_path TEXT,
    created_at   INTEGER NOT NULL
) STRICT;

CREATE TABLE credits (
    item_id   TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role      TEXT    NOT NULL CHECK (role IN ('cast', 'director', 'writer', 'creator')),
    character TEXT,
    ord       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (item_id, person_id, role)
) STRICT, WITHOUT ROWID;

CREATE INDEX credits_person ON credits (person_id);

-- ---------------------------------------------------------------------------------------------
-- Container state counts only the children that carry content (not extras).
-- ---------------------------------------------------------------------------------------------
DROP TRIGGER items_rating_update;
DROP TRIGGER items_container_update;
DROP TRIGGER items_container_delete;

-- +goose StatementBegin
CREATE TRIGGER items_rating_update AFTER UPDATE OF own_rating, parent_id ON items
BEGIN
    -- level 0: the row itself
    UPDATE items SET effective_rating = CASE
        WHEN NEW.parent_id IS NULL THEN NEW.own_rating
        WHEN (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id) IS NULL THEN NULL
        WHEN NEW.own_rating IS NULL THEN (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id)
        ELSE max(NEW.own_rating, (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id)) END
    WHERE id = NEW.id;
    -- level 1: children
    UPDATE items SET effective_rating = CASE
        WHEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id) IS NULL THEN NULL
        WHEN own_rating IS NULL THEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)
        ELSE max(own_rating, (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)) END
    WHERE parent_id = NEW.id;
    -- level 2: grandchildren
    UPDATE items SET effective_rating = CASE
        WHEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id) IS NULL THEN NULL
        WHEN own_rating IS NULL THEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)
        ELSE max(own_rating, (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)) END
    WHERE parent_id IN (SELECT c.id FROM items AS c WHERE c.parent_id = NEW.id);
    -- level 3: great-grandchildren
    UPDATE items SET effective_rating = CASE
        WHEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id) IS NULL THEN NULL
        WHEN own_rating IS NULL THEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)
        ELSE max(own_rating, (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)) END
    WHERE parent_id IN (SELECT g.id FROM items AS g
                        WHERE g.parent_id IN (SELECT c.id FROM items AS c WHERE c.parent_id = NEW.id));
    -- the previous parent (re-parenting) recomputes its container state
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = OLD.parent_id AND OLD.parent_id IS NOT NEW.parent_id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER items_container_update AFTER UPDATE OF available, effective_rating, deleted_at ON items
WHEN NEW.parent_id IS NOT NULL
BEGIN
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = NEW.parent_id;
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = (SELECT p.parent_id FROM items AS p WHERE p.id = NEW.parent_id);
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = (SELECT g.parent_id FROM items AS g
                WHERE g.id = (SELECT p.parent_id FROM items AS p WHERE p.id = NEW.parent_id));
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER items_container_delete AFTER DELETE ON items
WHEN OLD.parent_id IS NOT NULL
BEGIN
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = OLD.parent_id;
END;
-- +goose StatementEnd

-- Existing containers: recompute once with the new rule.
UPDATE items SET
    available = EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                        AND c.kind IN ('season', 'episode', 'podcast_episode')),
    min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                        WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                          AND c.kind IN ('season', 'episode', 'podcast_episode'))
WHERE kind IN ('series', 'season', 'podcast');

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
