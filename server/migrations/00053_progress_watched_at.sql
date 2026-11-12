-- «Gesehen» becomes lasting (concept 20 section 6.3, contract 1.31.0, 12 November 2026).
--
-- progress.completed follows the 90 % rule of every event (AN-10): starting a watched film again
-- makes it unwatched. watched_at (Unix milliseconds) is the moment a title was last finished, by
-- an accepted event that met the 90 % rule or by the check mark set by hand (setItemWatched); a
-- later event below 90 % leaves it, only removing the check mark clears it. NULL = never finished.
-- The check marks, watchedCount, the library status and the page «Gesehen» read it; continue
-- watching, next up and the offline rules keep reading completed.
--
-- Backfill: every row that is completed now was finished at its updated_at. A title finished
-- before this update and started again since cannot be recovered; the database never knew it.
--
-- progress_watched serves listWatched: the finished titles of a profile, newest first.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE progress ADD COLUMN watched_at INTEGER CHECK (watched_at IS NULL OR watched_at >= 0);
UPDATE progress SET watched_at = updated_at WHERE completed = 1;
CREATE INDEX progress_watched ON progress (profile_id, watched_at DESC) WHERE watched_at IS NOT NULL;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
