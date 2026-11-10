-- The pencil for audiobooks (1.24.0, concept 19 stage D1, D5.3): three new fields of a book and two
-- repairs, so the editor finds every book in a state it can work with.
--
--   items.publisher, items.isbn, items.asin   read from metadata.json and the tags of the first part
--                                             (absMeta, tagMeta) and written by the editor; locks
--                                             publisher, isbn, asin in items.locked_fields like the
--                                             other columns. NULL = unknown.
--
-- Repairs:
--
--   * A refresh of a book queued metadata.match, which ends every audiobook with metadata_state
--     'skipped' and metadata_error 'unsupported_kind'; image.fetch then left its cover alone. From
--     1.24.0 a book needs no match (the files are its source), so such books are 'matched' again.
--   * The language of a book is its original language (D5.3; setItemOriginalLanguage). The scan
--     writes it from now on; a book the scan does not touch again takes the language of its text
--     rows: de when there is a German row (the scan writes the book's language and en), else en.
--     A locked value and a book without any text row stay as they are.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE items ADD COLUMN publisher TEXT;
ALTER TABLE items ADD COLUMN isbn TEXT;
ALTER TABLE items ADD COLUMN asin TEXT;

UPDATE items SET metadata_state = 'matched', metadata_error = NULL
WHERE kind = 'audiobook' AND metadata_state = 'skipped' AND metadata_error = 'unsupported_kind';

UPDATE items SET original_language = CASE
        WHEN EXISTS (SELECT 1 FROM item_texts AS t WHERE t.item_id = items.id AND t.lang = 'de') THEN 'de'
        ELSE 'en' END
WHERE kind = 'audiobook' AND original_language IS NULL
  AND instr(locked_fields, '"original_language"') = 0
  AND EXISTS (SELECT 1 FROM item_texts AS t WHERE t.item_id = items.id AND t.lang IN ('de', 'en'));

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
