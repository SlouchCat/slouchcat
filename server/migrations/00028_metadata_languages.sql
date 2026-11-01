-- Metadata languages as an ordered list (concept 10 step 1, ADR 0018, decision 16):
--   * item_texts loses CHECK (lang IN ('de','en')): the texts of a title come in the languages of
--     the effective metadata language list (settings metadata.text_langs, per library from step 3),
--     BCP-47 with an optional region (de, en, pt-BR). The new column rank is the position of the
--     row's language in that list at the time of the fill (0 = first); the viewer-facing chain
--     reads metadata language of the profile -> interface language -> lowest rank -> items.title
--     without reading a setting (rule 2 and 12). Existing rows get de 0, en 1 (today's order).
--     SQLite knows no ALTER COLUMN: the table is rebuilt after the template, sort_key and the
--     index item_texts_sort_key of 00026 stay, and the triggers search_texts_* of 00015 (which
--     the DROP TABLE removes) are created again, unchanged.
--   * item_genres loses the same CHECK; its triggers of 00015 write one row per key of the
--     items.genres object instead of one per 'de' and 'en'. A plain array (a title before its
--     match) keeps today's rule and stands for de and en.
--   * profiles.metadata_language: the language of the metadata a profile reads, NULL = like its
--     interface language (profiles.language, which stays de or en and steers the interface).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE item_texts_new (
    item_id  TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    lang     TEXT    NOT NULL CHECK (length(lang) BETWEEN 2 AND 12),
    title    TEXT    NOT NULL,
    overview TEXT,
    tagline  TEXT,
    sort_key TEXT,
    rank     INTEGER NOT NULL DEFAULT 0 CHECK (rank >= 0),
    PRIMARY KEY (item_id, lang)
) STRICT, WITHOUT ROWID;

INSERT INTO item_texts_new (item_id, lang, title, overview, tagline, sort_key, rank)
SELECT item_id, lang, title, overview, tagline, sort_key, CASE lang WHEN 'de' THEN 0 WHEN 'en' THEN 1 ELSE 2 END
FROM item_texts;

DROP TABLE item_texts;
ALTER TABLE item_texts_new RENAME TO item_texts;

CREATE INDEX item_texts_sort_key ON item_texts (item_id, lang, sort_key);
CREATE INDEX item_texts_rank     ON item_texts (item_id, rank);

-- The search triggers of 00015, unchanged (the DROP TABLE above removed them).
-- +goose StatementBegin
CREATE TRIGGER search_texts_insert AFTER INSERT ON item_texts
BEGIN
    INSERT INTO search_docs (kind, item_id, ref_id, text)
    VALUES ('text', NEW.item_id, NEW.item_id || ':' || NEW.lang, NEW.title)
    ON CONFLICT (kind, ref_id, item_id) DO UPDATE SET text = excluded.text;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_texts_update AFTER UPDATE OF title ON item_texts
WHEN NEW.title IS NOT OLD.title
BEGIN
    INSERT INTO search_docs (kind, item_id, ref_id, text)
    VALUES ('text', NEW.item_id, NEW.item_id || ':' || NEW.lang, NEW.title)
    ON CONFLICT (kind, ref_id, item_id) DO UPDATE SET text = excluded.text;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_texts_delete AFTER DELETE ON item_texts
BEGIN
    DELETE FROM search_docs WHERE kind = 'text' AND ref_id = OLD.item_id || ':' || OLD.lang AND item_id = OLD.item_id;
END;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------------------------
-- item_genres without the language CHECK; the triggers read the keys of the genres object.
-- ---------------------------------------------------------------------------------------------
CREATE TABLE item_genres_new (
    item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    lang    TEXT NOT NULL CHECK (length(lang) BETWEEN 2 AND 12),
    name    TEXT NOT NULL,
    PRIMARY KEY (item_id, lang, name)
) STRICT, WITHOUT ROWID;

INSERT INTO item_genres_new (item_id, lang, name) SELECT item_id, lang, name FROM item_genres;

-- The triggers on items refer to item_genres; the RENAME below re-parses the schema and would
-- fail while the table is gone, so they go first and are created again below.
DROP TRIGGER items_genres_insert;
DROP TRIGGER items_genres_update;

DROP TABLE item_genres;
ALTER TABLE item_genres_new RENAME TO item_genres;
CREATE INDEX item_genres_name ON item_genres (lang, name);

-- +goose StatementBegin
CREATE TRIGGER items_genres_insert AFTER INSERT ON items
BEGIN
    INSERT OR IGNORE INTO item_genres (item_id, lang, name)
    SELECT NEW.id, l.key, g.value
    FROM json_each(NEW.genres) AS l
    JOIN json_each(l.value) AS g
    WHERE json_valid(NEW.genres) AND json_type(NEW.genres) = 'object' AND l.type = 'array'
      AND g.type = 'text' AND g.value <> '';
    INSERT OR IGNORE INTO item_genres (item_id, lang, name)
    SELECT NEW.id, l.lang, g.value
    FROM (SELECT 'de' AS lang UNION ALL SELECT 'en') AS l
    JOIN json_each(NEW.genres) AS g
    WHERE json_valid(NEW.genres) AND json_type(NEW.genres) = 'array' AND g.type = 'text' AND g.value <> '';
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER items_genres_update AFTER UPDATE OF genres ON items
BEGIN
    DELETE FROM item_genres WHERE item_id = NEW.id;
    INSERT OR IGNORE INTO item_genres (item_id, lang, name)
    SELECT NEW.id, l.key, g.value
    FROM json_each(NEW.genres) AS l
    JOIN json_each(l.value) AS g
    WHERE json_valid(NEW.genres) AND json_type(NEW.genres) = 'object' AND l.type = 'array'
      AND g.type = 'text' AND g.value <> '';
    INSERT OR IGNORE INTO item_genres (item_id, lang, name)
    SELECT NEW.id, l.lang, g.value
    FROM (SELECT 'de' AS lang UNION ALL SELECT 'en') AS l
    JOIN json_each(NEW.genres) AS g
    WHERE json_valid(NEW.genres) AND json_type(NEW.genres) = 'array' AND g.type = 'text' AND g.value <> '';
END;
-- +goose StatementEnd

ALTER TABLE profiles ADD COLUMN metadata_language TEXT CHECK (metadata_language IS NULL OR length(metadata_language) BETWEEN 2 AND 12);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
