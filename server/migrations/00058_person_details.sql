-- The facts of a person for the person page (concept 21 P-8, O-13 and O-14 answered 14 November
-- 2026).
--
-- The job people.details fills them from TMDB /person/{id} with combined_credits: the biography in
-- the main metadata language (the first text language), or the English one when TMDB has none in
-- it (biography_lang then says en), birthday and deathday as YYYY-MM-DD, the place of birth and the
-- known-for department as TMDB names them, and total_credits, the number of films and series TMDB
-- lists for the person (every title once, for «n von m Titeln»). details_lang is the main language
-- the facts were asked in and details_fetched_at the time of the fetch (Unix milliseconds); a
-- person with details_fetched_at NULL, an older fetch or another main language is fetched again.
-- A person TMDB does not know keeps NULL facts with its fetch time. Every column is NULL for the
-- rows before this migration.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE people ADD COLUMN biography TEXT CHECK (biography IS NULL OR biography <> '');
ALTER TABLE people ADD COLUMN biography_lang TEXT
    CHECK ((biography IS NULL) = (biography_lang IS NULL) AND (biography_lang IS NULL OR biography_lang <> ''));
ALTER TABLE people ADD COLUMN birthday TEXT
    CHECK (birthday IS NULL OR birthday GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]');
ALTER TABLE people ADD COLUMN deathday TEXT
    CHECK (deathday IS NULL OR deathday GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]');
ALTER TABLE people ADD COLUMN place_of_birth TEXT CHECK (place_of_birth IS NULL OR place_of_birth <> '');
ALTER TABLE people ADD COLUMN known_for_department TEXT
    CHECK (known_for_department IS NULL OR known_for_department <> '');
ALTER TABLE people ADD COLUMN total_credits INTEGER CHECK (total_credits IS NULL OR total_credits >= 0);
ALTER TABLE people ADD COLUMN details_lang TEXT;
ALTER TABLE people ADD COLUMN details_fetched_at INTEGER;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
