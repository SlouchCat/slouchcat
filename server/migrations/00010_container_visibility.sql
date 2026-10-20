-- Container visibility (M3 review, findings 1 and 2; plan 3.3, 10.2).
--
--   triggers       min_child_rating of a container takes, for a child that is itself a container
--                  (a season of a series), the larger of the child's effective_rating and its
--                  min_child_rating, so a series whose episodes are all above a level is above that
--                  level too (before, the series only saw the seasons' inherited rating). Leaf
--                  children (episodes, podcast episodes) count with their effective_rating as before.
--                  Containers are recomputed once: seasons first, then series and podcasts.
--   visible_items  an extra is visible only while its parent (movie, series or season) is visible:
--                  live, available and within the profile's and the account's ceiling, containers
--                  also by min_child_rating. Everything else is the view of migration 00007.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

DROP VIEW visible_items;
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
        min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                            THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                            ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
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

-- Existing containers: recompute once with the new rule, the seasons before the series they belong to.
UPDATE items SET
    available = EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                        AND c.kind IN ('season', 'episode', 'podcast_episode')),
    min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                        THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                        ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
                        WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                          AND c.kind IN ('season', 'episode', 'podcast_episode'))
WHERE kind = 'season';
UPDATE items SET
    available = EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                        AND c.kind IN ('season', 'episode', 'podcast_episode')),
    min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                        THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                        ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
                        WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                          AND c.kind IN ('season', 'episode', 'podcast_episode'))
WHERE kind IN ('series', 'podcast');

-- Visibility (plan 3.3): the profile's own ceiling and library rights, capped by the account's; an
-- extra only below a visible parent.
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
                  WHERE al.account_id = a.id AND al.library_id = i.library_id));

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
