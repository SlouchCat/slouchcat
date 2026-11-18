-- Concept 25 (08-datenmodell-api.md section 2.9, «Aufräumen»): titles outside the library in use.
-- Only package metadata reads and writes the tables (store/externalq).
--
--   external_titles.used_at  when a page last read the title (the preview page, a card of the
--                            search, of the wishes or of the administration), Unix milliseconds,
--                            written at most once a day per title. titles.refresh fetches again
--                            only the titles of open wishes and those used within 30 days, and
--                            deletes the rows (with texts, images and image files) that were not
--                            used for 30 days, that no open or recently closed wish refers to and
--                            that are not an entry of the spotlight list. Existing rows start with
--                            their fetch time.
--
-- A new column with an index; nothing is rebuilt.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE external_titles ADD COLUMN used_at INTEGER;
UPDATE external_titles SET used_at = fetched_at;
CREATE INDEX external_titles_used ON external_titles (used_at);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
