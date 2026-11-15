-- Concept 22 stage 3 «Freigabeländer» (ADR 0034): the certificates of every title per provider and
-- rating country, and the one-time hint of the administration.
--
--   item_certifications  what TMDB and TVDB delivered for the title level of a film or series, one
--                        row per provider and rating country of model.RatingSystems (FR "12",
--                        US "PG-13"), written by the match and the fill. With it a change of the
--                        rating country (F-16) computes the ages again without asking the
--                        providers, and «the strictest value over every provider» is recomputed
--                        locally (review 2, M2-2). A provider that delivered no certificate of a
--                        rating country leaves one row with country '' and certification '':
--                        "asked, nothing", so the job «Freigaben neu lesen» does not ask again. Only
--                        package metadata reads and writes the table (store/certq, depguard rule
--                        "certq"); the rows go with their title (ON DELETE CASCADE).
--   settings             metadata.minimum_age_notice = '1' when the database holds ratings of a
--                        provider (an update, not a fresh install): the administration shows once
--                        that the ages are minimum ages now, PG-13 «ab 13» instead of FSK 12
--                        (section 4.11), until the administrator closes it.
--
-- No table is rebuilt; new tables need no foreign_keys=OFF, the frame of the template stays.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE item_certifications (
    item_id       TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    provider      TEXT    NOT NULL CHECK (provider IN ('tmdb', 'tvdb')),
    country       TEXT    NOT NULL CHECK (country = '' OR (length(country) = 2 AND country = upper(country))),
    certification TEXT    NOT NULL CHECK (length(certification) <= 40 AND (country = '') = (certification = '')),
    fetched_at    INTEGER NOT NULL,
    PRIMARY KEY (item_id, provider, country)
) STRICT, WITHOUT ROWID;

INSERT INTO settings (key, value, updated_at)
SELECT 'metadata.minimum_age_notice', '1', CAST(unixepoch('subsec') * 1000 AS INTEGER)
WHERE EXISTS (SELECT 1 FROM items WHERE rating_source LIKE 'tmdb:%' OR rating_source LIKE 'tvdb:%');

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
