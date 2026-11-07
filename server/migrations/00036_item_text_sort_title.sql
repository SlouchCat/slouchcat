-- The sort title as the administrator typed it (concept 15 7.5, P-6). Schema only.
--
--   item_texts.sort_title  the text of the field «Sortiertitel» of the metadata editor in that
--                          language, as typed (case and accents kept), while the lock
--                          sort_title:<lang> holds it; NULL otherwise. The library keeps sorting
--                          by sort_key, which the editor derives from it (folded, lower-cased).
--                          items.sort_title stays the scanner's key of the file name.
--
-- ADD COLUMN works on the STRICT table item_texts, so no table is rebuilt; every existing row gets
-- NULL.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE item_texts ADD COLUMN sort_title TEXT;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
