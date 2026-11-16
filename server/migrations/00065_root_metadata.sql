-- Metadata settings per folder instead of per library (concept 24 section 3, ADR 0036, decision
-- 32): the libraries lose their deviation, the folders gain it, and every title learns its home
-- folder.
--
--   * library_roots.text_langs and .images: the folder's own text languages and images, NULL =
--     like the server's default. text_langs is the ordered JSON list of codes; images is
--     {"langs":{"poster":[…],"backdrop":[…],"logo":[…]},"kinds":[…]} as library_metadata had it
--     (a chain missing from langs follows the default's chain; kinds is complete, [] = none).
--   * items.root_id: the home folder of a title (ADR 0036). The folder of its oldest present file
--     (media_files.created_at, ties by path); a series and a podcast count the files of their
--     episodes; seasons, episodes and extras take their parent's. NULL = no home folder (a feed-only
--     podcast, a title whose files are all missing); such a title uses the default. ON DELETE SET
--     NULL: the soft-deleted titles of a removed folder lose the reference instead of pointing
--     nowhere. No trigger listens on root_id.
--   * Every deviation of library_metadata goes onto every video folder (content movie or series) of
--     its library, so what applied before applies unchanged; then the table goes.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE library_roots ADD COLUMN text_langs TEXT CHECK (text_langs IS NULL OR json_valid(text_langs));
ALTER TABLE library_roots ADD COLUMN images TEXT CHECK (images IS NULL OR json_valid(images));
ALTER TABLE items ADD COLUMN root_id INTEGER REFERENCES library_roots(id) ON DELETE SET NULL;
CREATE INDEX items_root ON items (root_id);

UPDATE library_roots SET
    text_langs = (SELECT m.text_langs FROM library_metadata AS m WHERE m.library_id = library_roots.library_id),
    images = (SELECT m.images FROM library_metadata AS m WHERE m.library_id = library_roots.library_id)
WHERE content IN ('movie', 'series')
  AND library_id IN (SELECT library_id FROM library_metadata);

-- Top-level titles with files of their own (films, audiobooks, and anything else without a parent
-- except series and podcasts).
UPDATE items SET root_id = (
    SELECT m.root_id FROM media_files AS m JOIN library_roots AS r ON r.id = m.root_id
    WHERE m.item_id = items.id AND m.missing_since IS NULL
    ORDER BY m.created_at, r.path || '/' || m.rel_path LIMIT 1)
WHERE parent_id IS NULL AND kind NOT IN ('series', 'podcast');

-- Series and podcasts: the oldest present file of their episodes (under a season or directly).
UPDATE items SET root_id = (
    SELECT m.root_id FROM media_files AS m JOIN library_roots AS r ON r.id = m.root_id
    JOIN items AS e ON e.id = m.item_id
    WHERE m.missing_since IS NULL AND e.kind IN ('episode', 'podcast_episode')
      AND (e.parent_id = items.id OR e.parent_id IN (SELECT c.id FROM items AS c WHERE c.parent_id = items.id))
    ORDER BY m.created_at, r.path || '/' || m.rel_path LIMIT 1)
WHERE parent_id IS NULL AND kind IN ('series', 'podcast');

-- Children inherit: first the direct children of top-level titles, then their children.
UPDATE items SET root_id = (SELECT p.root_id FROM items AS p WHERE p.id = items.parent_id)
WHERE parent_id IN (SELECT p.id FROM items AS p WHERE p.parent_id IS NULL);

UPDATE items SET root_id = (SELECT p.root_id FROM items AS p WHERE p.id = items.parent_id)
WHERE parent_id IN (SELECT p.id FROM items AS p WHERE p.parent_id IS NOT NULL);

DROP TABLE library_metadata;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
