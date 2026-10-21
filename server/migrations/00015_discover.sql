-- Discover and curation (plan M7, 3.1 discover/notify, 3.3 query builder, 3.4 step 5, 5.5, AN-3 decision 9;
-- M7 design section 1). Times are Unix milliseconds (UTC).
--
--   watchlist        one list per profile (E-5); every kind except season and extra (checked in code).
--   favorites        series and podcasts a profile follows (E-8); the kind is checked in code.
--   collections      TMDB collections (source 'tmdb', one per tmdb_id) and manual collections of the
--                    admin (source 'manual'); name is the English or admin name, name_de the German
--                    one from TMDB. collection_items holds the members with a position.
--   home_rows        the order of the start page rows (E-9): exactly the eight keys, with a hidden flag.
--   item_genres      one row per item, language and genre name, kept by triggers from items.genres
--                    (JSON object {"de":[...],"en":[...]} or a plain list that counts for both).
--   search_index     FTS5 index for the search (E-4): one row per items row ('item': title, original
--                    title, author, narrator, series name), per item_texts row ('text'), per credit
--                    ('person': the person's name, ref_id = people.id) and per collection member
--                    ('collection': both names, ref_id = collection id). Kept by triggers in the same
--                    transaction as the source rows; readers always join visible_items.
--   trakt_lists      the public Trakt lists (trending, popular; movies and shows) of the last
--                    successful daily run, replaced as a whole; emptied when Trakt fails (I-7).
--   notifications    the in-app inbox of a profile (E-8): a new episode of a favourite series or
--                    podcast; one row per profile and episode.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

-- ---------------------------------------------------------------------------------------------
-- Watchlist and favourites (E-5, E-8).
-- ---------------------------------------------------------------------------------------------
CREATE TABLE watchlist (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (profile_id, item_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX watchlist_item ON watchlist (item_id);

CREATE TABLE favorites (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (profile_id, item_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX favorites_item ON favorites (item_id);

-- ---------------------------------------------------------------------------------------------
-- Collections (E-6).
-- ---------------------------------------------------------------------------------------------
CREATE TABLE collections (
    id          TEXT    PRIMARY KEY CHECK (length(id) = 26),
    source      TEXT    NOT NULL CHECK (source IN ('tmdb', 'manual')),
    tmdb_id     INTEGER UNIQUE,
    name        TEXT    NOT NULL,
    name_de     TEXT,
    overview    TEXT,
    overview_de TEXT,
    poster_path TEXT,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
) STRICT;

CREATE TABLE collection_items (
    collection_id TEXT    NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    item_id       TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    position      INTEGER NOT NULL DEFAULT 0,
    created_at    INTEGER NOT NULL,
    PRIMARY KEY (collection_id, item_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX collection_items_item ON collection_items (item_id);

-- ---------------------------------------------------------------------------------------------
-- Start page rows (E-1, E-9): the eight keys in the default order of E-1, then the Trakt rows.
-- ---------------------------------------------------------------------------------------------
CREATE TABLE home_rows (
    key        TEXT    PRIMARY KEY CHECK (key IN ('picks', 'continue', 'recent', 'watchlist', 'collections',
                                                 'genres', 'trakt_trending', 'trakt_popular')),
    position   INTEGER NOT NULL,
    hidden     INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)),
    updated_at INTEGER NOT NULL
) STRICT;

INSERT INTO home_rows (key, position, hidden, updated_at) VALUES
    ('picks', 0, 0, 0), ('continue', 1, 0, 0), ('recent', 2, 0, 0), ('watchlist', 3, 0, 0),
    ('collections', 4, 0, 0), ('genres', 5, 0, 0), ('trakt_trending', 6, 0, 0), ('trakt_popular', 7, 0, 0);

CREATE INDEX top_picks_profile ON top_picks (profile_id);

-- ---------------------------------------------------------------------------------------------
-- Genres per item and language (search filter, genres row).
-- ---------------------------------------------------------------------------------------------
CREATE TABLE item_genres (
    item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    lang    TEXT NOT NULL CHECK (lang IN ('de', 'en')),
    name    TEXT NOT NULL,
    PRIMARY KEY (item_id, lang, name)
) STRICT, WITHOUT ROWID;
CREATE INDEX item_genres_name ON item_genres (lang, name);

INSERT OR IGNORE INTO item_genres (item_id, lang, name)
SELECT i.id, l.lang, g.value
FROM items AS i
JOIN (SELECT 'de' AS lang UNION ALL SELECT 'en') AS l
JOIN json_each(CASE WHEN json_type(i.genres) = 'object' THEN json_extract(i.genres, '$.' || l.lang)
                    ELSE i.genres END) AS g
WHERE json_valid(i.genres) AND g.type = 'text' AND g.value <> '';

-- +goose StatementBegin
CREATE TRIGGER items_genres_insert AFTER INSERT ON items
BEGIN
    INSERT OR IGNORE INTO item_genres (item_id, lang, name)
    SELECT NEW.id, l.lang, g.value
    FROM (SELECT 'de' AS lang UNION ALL SELECT 'en') AS l
    JOIN json_each(CASE WHEN json_type(NEW.genres) = 'object' THEN json_extract(NEW.genres, '$.' || l.lang)
                        ELSE NEW.genres END) AS g
    WHERE json_valid(NEW.genres) AND g.type = 'text' AND g.value <> '';
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER items_genres_update AFTER UPDATE OF genres ON items
BEGIN
    DELETE FROM item_genres WHERE item_id = NEW.id;
    INSERT OR IGNORE INTO item_genres (item_id, lang, name)
    SELECT NEW.id, l.lang, g.value
    FROM (SELECT 'de' AS lang UNION ALL SELECT 'en') AS l
    JOIN json_each(CASE WHEN json_type(NEW.genres) = 'object' THEN json_extract(NEW.genres, '$.' || l.lang)
                        ELSE NEW.genres END) AS g
    WHERE json_valid(NEW.genres) AND g.type = 'text' AND g.value <> '';
END;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------------------------
-- Search index (E-4). Prefix queries from the second character ("ab"*); diacritics folded.
-- ---------------------------------------------------------------------------------------------
CREATE VIRTUAL TABLE search_index USING fts5(
    text,
    kind UNINDEXED,
    item_id UNINDEXED,
    ref_id UNINDEXED,
    tokenize = 'unicode61 remove_diacritics 2',
    prefix = '2 3'
);

INSERT INTO search_index (text, kind, item_id, ref_id)
SELECT trim(coalesce(title, '') || ' / ' || coalesce(original_title, '') || ' / ' || coalesce(author, '')
            || ' / ' || coalesce(narrator, '') || ' / ' || coalesce(series_name, '')),
       'item', id, id
FROM items;

INSERT INTO search_index (text, kind, item_id, ref_id)
SELECT title, 'text', item_id, item_id || ':' || lang FROM item_texts;

INSERT INTO search_index (text, kind, item_id, ref_id)
SELECT p.name, 'person', c.item_id, CAST(c.person_id AS TEXT)
FROM credits AS c JOIN people AS p ON p.id = c.person_id;

-- +goose StatementBegin
CREATE TRIGGER search_items_insert AFTER INSERT ON items
BEGIN
    INSERT INTO search_index (text, kind, item_id, ref_id)
    VALUES (trim(coalesce(NEW.title, '') || ' / ' || coalesce(NEW.original_title, '') || ' / ' || coalesce(NEW.author, '')
                 || ' / ' || coalesce(NEW.narrator, '') || ' / ' || coalesce(NEW.series_name, '')),
            'item', NEW.id, NEW.id);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_items_update AFTER UPDATE OF title, original_title, author, narrator, series_name ON items
BEGIN
    DELETE FROM search_index WHERE kind = 'item' AND ref_id = OLD.id;
    INSERT INTO search_index (text, kind, item_id, ref_id)
    VALUES (trim(coalesce(NEW.title, '') || ' / ' || coalesce(NEW.original_title, '') || ' / ' || coalesce(NEW.author, '')
                 || ' / ' || coalesce(NEW.narrator, '') || ' / ' || coalesce(NEW.series_name, '')),
            'item', NEW.id, NEW.id);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_items_delete AFTER DELETE ON items
BEGIN
    DELETE FROM search_index WHERE item_id = OLD.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_texts_insert AFTER INSERT ON item_texts
BEGIN
    INSERT INTO search_index (text, kind, item_id, ref_id)
    VALUES (NEW.title, 'text', NEW.item_id, NEW.item_id || ':' || NEW.lang);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_texts_update AFTER UPDATE OF title ON item_texts
BEGIN
    DELETE FROM search_index WHERE kind = 'text' AND ref_id = OLD.item_id || ':' || OLD.lang;
    INSERT INTO search_index (text, kind, item_id, ref_id)
    VALUES (NEW.title, 'text', NEW.item_id, NEW.item_id || ':' || NEW.lang);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_texts_delete AFTER DELETE ON item_texts
BEGIN
    DELETE FROM search_index WHERE kind = 'text' AND ref_id = OLD.item_id || ':' || OLD.lang;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_credits_insert AFTER INSERT ON credits
BEGIN
    INSERT INTO search_index (text, kind, item_id, ref_id)
    SELECT p.name, 'person', NEW.item_id, CAST(NEW.person_id AS TEXT) FROM people AS p WHERE p.id = NEW.person_id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_credits_delete AFTER DELETE ON credits
BEGIN
    DELETE FROM search_index WHERE kind = 'person' AND item_id = OLD.item_id AND ref_id = CAST(OLD.person_id AS TEXT)
      AND NOT EXISTS (SELECT 1 FROM credits AS c WHERE c.item_id = OLD.item_id AND c.person_id = OLD.person_id);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_people_update AFTER UPDATE OF name ON people
BEGIN
    UPDATE search_index SET text = NEW.name WHERE kind = 'person' AND ref_id = CAST(NEW.id AS TEXT);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_collections_update AFTER UPDATE OF name, name_de ON collections
BEGIN
    UPDATE search_index SET text = trim(NEW.name || ' / ' || coalesce(NEW.name_de, ''))
    WHERE kind = 'collection' AND ref_id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_collection_items_insert AFTER INSERT ON collection_items
BEGIN
    INSERT INTO search_index (text, kind, item_id, ref_id)
    SELECT trim(c.name || ' / ' || coalesce(c.name_de, '')), 'collection', NEW.item_id, NEW.collection_id
    FROM collections AS c WHERE c.id = NEW.collection_id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_collection_items_delete AFTER DELETE ON collection_items
BEGIN
    DELETE FROM search_index WHERE kind = 'collection' AND item_id = OLD.item_id AND ref_id = OLD.collection_id;
END;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------------------------
-- Trakt lists (E-7, I-7, AN-3).
-- ---------------------------------------------------------------------------------------------
CREATE TABLE trakt_lists (
    list       TEXT    NOT NULL CHECK (list IN ('trending', 'popular')),
    media      TEXT    NOT NULL CHECK (media IN ('movie', 'show')),
    position   INTEGER NOT NULL,
    tmdb_id    INTEGER NOT NULL,
    title      TEXT,
    year       INTEGER,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (list, media, position)
) STRICT, WITHOUT ROWID;
CREATE INDEX trakt_lists_tmdb ON trakt_lists (media, tmdb_id);

-- ---------------------------------------------------------------------------------------------
-- In-app notifications (E-8, 5.5).
-- ---------------------------------------------------------------------------------------------
CREATE TABLE notifications (
    id         INTEGER PRIMARY KEY,
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL CHECK (kind IN ('new_episode')),
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    parent_id  TEXT    REFERENCES items(id) ON DELETE SET NULL,
    created_at INTEGER NOT NULL,
    read_at    INTEGER,
    UNIQUE (profile_id, kind, item_id)
) STRICT;
CREATE INDEX notifications_inbox ON notifications (profile_id, read_at, created_at DESC);
CREATE INDEX notifications_item ON notifications (item_id);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
