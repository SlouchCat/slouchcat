-- The description of a TMDB collection for the collection page (concept 21 P-10, O-18 answered 14
-- November 2026).
--
-- The job people.details fills it from TMDB /collection/{id} after the people: the overview in the
-- main metadata language (the first text language), or the English one when TMDB has none in it
-- (description_lang then says en), the rule of the person biography (O-13). details_lang is the
-- main language the description was asked in and details_fetched_at the time of the fetch (Unix
-- milliseconds); a TMDB collection with details_fetched_at NULL, an older fetch or another main
-- language is fetched again. A collection TMDB does not know keeps a NULL description with its
-- fetch time. Manual collections are never fetched. The older columns overview and overview_de
-- stay as they are (the match fills them from belongs_to_collection, which carries no overview).
-- Every new column is NULL for the rows before this migration.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE collections ADD COLUMN description TEXT CHECK (description IS NULL OR description <> '');
ALTER TABLE collections ADD COLUMN description_lang TEXT
    CHECK ((description IS NULL) = (description_lang IS NULL) AND (description_lang IS NULL OR description_lang <> ''));
ALTER TABLE collections ADD COLUMN details_lang TEXT;
ALTER TABLE collections ADD COLUMN details_fetched_at INTEGER;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
