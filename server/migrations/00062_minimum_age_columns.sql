-- Concept 22 stage 3 «Freigabeländer» (ADR 0034): the rating columns of items and libraries lose
-- their CHECK on the FSK steps.
--
--   items.own_rating, items.effective_rating, items.min_child_rating and libraries.default_rating
--   were CHECK (... IN (0, 6, 12, 16, 18)). They hold the minimum age now (F-17: PG-13 = 13,
--   R = 17, TV-MA = 17, AT 14 = 14) and the steps of the rating country (F-12). The names stay
--   (review 2, B2-1): sqlc queries, Go code, the trigger logic, VerifyRatings and the lock key
--   "own_rating" are unchanged. The new CHECK only keeps the range 0 to 18 (the ceiling stays 18,
--   administrators have it, unknown counts as it, AN-1); the server checks the steps.
--
-- Neither table is rebuilt. modernc.org/sqlite ships an SQLite with DROP COLUMN and RENAME COLUMN
-- (concept section 4.11): per column a new column is added, the values copied, the old column
-- dropped (its column CHECK goes with it) and the new one renamed. The column moves to the end of
-- the table; every query names its columns or is expanded by sqlc, so the order does not matter.
--
-- DROP COLUMN refuses a column a trigger or a view names, so the four rating triggers and the
-- view visible_items go first and come back unchanged: the triggers word for word as SQLite holds
-- them (items_rating_insert from 00003, items_rating_update, items_container_update and
-- items_container_delete from 00010), the view from 00061. No index, foreign key or other trigger
-- names the columns. The copy runs while the triggers are gone, so it derives nothing: the derived
-- columns are copied as they are, and TestMigration00062 checks they still equal what the
-- triggers compute.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

DROP VIEW visible_items;
DROP TRIGGER items_rating_insert;
DROP TRIGGER items_rating_update;
DROP TRIGGER items_container_update;
DROP TRIGGER items_container_delete;

ALTER TABLE items ADD COLUMN own_rating_age INTEGER CHECK (own_rating_age IS NULL OR own_rating_age BETWEEN 0 AND 18);
ALTER TABLE items ADD COLUMN effective_rating_age INTEGER CHECK (effective_rating_age IS NULL OR effective_rating_age BETWEEN 0 AND 18);
ALTER TABLE items ADD COLUMN min_child_rating_age INTEGER CHECK (min_child_rating_age IS NULL OR min_child_rating_age BETWEEN 0 AND 18);
UPDATE items SET own_rating_age = own_rating, effective_rating_age = effective_rating, min_child_rating_age = min_child_rating;
ALTER TABLE items DROP COLUMN own_rating;
ALTER TABLE items DROP COLUMN effective_rating;
ALTER TABLE items DROP COLUMN min_child_rating;
ALTER TABLE items RENAME COLUMN own_rating_age TO own_rating;
ALTER TABLE items RENAME COLUMN effective_rating_age TO effective_rating;
ALTER TABLE items RENAME COLUMN min_child_rating_age TO min_child_rating;

ALTER TABLE libraries ADD COLUMN default_rating_age INTEGER CHECK (default_rating_age IS NULL OR default_rating_age BETWEEN 0 AND 18);
UPDATE libraries SET default_rating_age = default_rating;
ALTER TABLE libraries DROP COLUMN default_rating;
ALTER TABLE libraries RENAME COLUMN default_rating_age TO default_rating;

-- Word for word from 00003.
-- +goose StatementBegin
CREATE TRIGGER items_rating_insert AFTER INSERT ON items
BEGIN
    UPDATE items SET effective_rating = CASE
        WHEN NEW.parent_id IS NULL THEN NEW.own_rating
        WHEN (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id) IS NULL THEN NULL
        WHEN NEW.own_rating IS NULL THEN (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id)
        ELSE max(NEW.own_rating, (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id)) END,
        available = CASE WHEN NEW.kind = 'podcast_episode' THEN (NEW.enclosure_url IS NOT NULL) ELSE available END
    WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- Word for word from 00010.
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
        min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                            THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                            ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = OLD.parent_id AND OLD.parent_id IS NOT NEW.parent_id;
END;
-- +goose StatementEnd

-- Word for word from 00010.
-- +goose StatementBegin
CREATE TRIGGER items_container_update AFTER UPDATE OF available, effective_rating, deleted_at ON items
WHEN NEW.parent_id IS NOT NULL
BEGIN
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                            THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                            ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = NEW.parent_id;
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                            THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                            ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = (SELECT p.parent_id FROM items AS p WHERE p.id = NEW.parent_id);
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                            THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                            ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = (SELECT g.parent_id FROM items AS g
                WHERE g.id = (SELECT p.parent_id FROM items AS p WHERE p.id = NEW.parent_id));
END;
-- +goose StatementEnd

-- Word for word from 00010.
-- +goose StatementBegin
CREATE TRIGGER items_container_delete AFTER DELETE ON items
WHEN OLD.parent_id IS NOT NULL
BEGIN
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                            THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                            ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = OLD.parent_id;
END;
-- +goose StatementEnd

-- Visibility (plan 3.3): word for word from 00061.
CREATE VIEW visible_items AS
SELECT p.id AS profile_id, i.id AS item_id
FROM profiles AS p
JOIN accounts AS a ON a.id = p.account_id
JOIN items AS i
  ON i.deleted_at IS NULL
 AND i.available = 1
 AND COALESCE(i.effective_rating, 18) <= min(p.max_rating, a.max_rating)
 AND (i.kind NOT IN ('series', 'season', 'podcast') OR COALESCE(i.min_child_rating, 18) <= min(p.max_rating, a.max_rating))
 AND (i.kind <> 'extra' OR EXISTS (
        SELECT 1 FROM items AS pa
        WHERE pa.id = i.parent_id AND pa.deleted_at IS NULL AND pa.available = 1
          AND COALESCE(pa.effective_rating, 18) <= min(p.max_rating, a.max_rating)
          AND (pa.kind NOT IN ('series', 'season', 'podcast') OR COALESCE(pa.min_child_rating, 18) <= min(p.max_rating, a.max_rating))))
WHERE (p.all_libraries = 1
       OR EXISTS (SELECT 1 FROM profile_libraries AS pl
                  WHERE pl.profile_id = p.id AND pl.library_id = i.library_id))
  AND (a.all_libraries = 1
       OR EXISTS (SELECT 1 FROM account_libraries AS al
                  WHERE al.account_id = a.id AND al.library_id = i.library_id))
  AND NOT EXISTS (SELECT 1 FROM libraries AS dl WHERE dl.id = i.library_id AND dl.deleting_at IS NOT NULL);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
