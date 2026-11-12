-- A library that is being deleted (concept 20 section 9.3, stage 5 after the acceptance on CT 190,
-- 12 November 2026).
--
-- deleteLibrary ran inside the request; on CT 190 it took 9.5 s for 304 titles and 27.4 s for
-- 31 641 (the backup of the 1.5 GB database alone about 9 s). It is a background job now
-- (library.delete): the request checks the name and running playbacks, sets
-- libraries.deleting_at (Unix milliseconds) and queues the job. From that moment the library is
-- gone for every view: ListLibraries and GetLibrary leave it out, and visible_items leaves out its
-- titles (the NOT EXISTS below, one lookup of the library by its key per title), so lists, search,
-- home rows, the offline rules and the playback start no longer see it. The job deletes the row; a
-- job that fails before its transaction clears deleting_at again.
--
-- visible_items is dropped and created again, word for word as in 00055 plus the last condition.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE libraries ADD COLUMN deleting_at INTEGER CHECK (deleting_at IS NULL OR deleting_at >= 0);

DROP VIEW visible_items;

-- Visibility (plan 3.3): the profile's own ceiling and library rights, capped by the account's; an
-- extra only below a visible parent; nothing of a library that is being deleted.
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
