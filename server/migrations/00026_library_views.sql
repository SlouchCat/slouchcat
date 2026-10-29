-- Library views, side navigation and spotlight rhythm (concept 08, ADR 0016):
--   * spotlight_config.rotation_seconds: how long the stage shows one title (F-10, 3 to 15 s,
--     default 5), set on the administration page "Spotlight".
--   * item_texts.sort_key: the sort key of the shown title per language. items.sort_title comes
--     from the file name and does not match the title in the profile language ("Spirited Away" in
--     the file name, "Chihiros Reise" in item_texts), so the library page sorts by this key: the
--     shown title with the article of its language stripped (der, die, das, ein, eine; the, a,
--     an), lower-cased, diacritics folded (Ä -> a, É -> e, ß -> ss). The letter bar takes the
--     first character (A-Z, else #). The backfill below is a SQL approximation of model.SortKey
--     (lower() folds ASCII only, so the common Latin-1 letters are replaced by hand); the scanner
--     and the metadata jobs write the exact key with every text from now on.
--   * items.vote_average: the TMDB vote average (0 to 10, NULL = unknown), a field of the fill
--     order of ADR 0015 (item_field_sources gets 'vote_average'); the job metadata.vote_backfill
--     fills the existing titles. Never the age rating, which stays own_rating / effective_rating.
--   * Indexes for the sort orders of the library page per library and kind, and for the sort key.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE spotlight_config ADD COLUMN rotation_seconds INTEGER NOT NULL DEFAULT 5
    CHECK (rotation_seconds BETWEEN 3 AND 15);

ALTER TABLE items ADD COLUMN vote_average REAL
    CHECK (vote_average IS NULL OR (vote_average >= 0 AND vote_average <= 10));

ALTER TABLE item_texts ADD COLUMN sort_key TEXT;

-- Backfill in two steps (sqlc's parser knows no UPDATE ... FROM): fold the common Latin-1 letters
-- by hand (lower() is ASCII only), lower-case and trim; then strip the article of the row's
-- language. A title that is only an article keeps it.
UPDATE item_texts
SET sort_key = trim(lower(replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(
                          replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(
                          replace(replace(replace(replace(replace(replace(replace(replace(replace(replace(
                          title,
                          'Ä', 'a'), 'ä', 'a'), 'Ö', 'o'), 'ö', 'o'), 'Ü', 'u'), 'ü', 'u'), 'ß', 'ss'),
                          'É', 'e'), 'é', 'e'), 'È', 'e'), 'è', 'e'), 'Ê', 'e'), 'ê', 'e'), 'Ë', 'e'), 'ë', 'e'),
                          'À', 'a'), 'à', 'a'), 'Â', 'a'), 'â', 'a'), 'Á', 'a'), 'á', 'a'), 'Ç', 'c'), 'ç', 'c'),
                          'Ô', 'o'), 'ô', 'o'), 'Ó', 'o'), 'ó', 'o'), 'Î', 'i'), 'î', 'i'), 'Ñ', 'n')));

UPDATE item_texts
SET sort_key = CASE
    WHEN lang = 'de' AND sort_key LIKE 'der %'  AND length(sort_key) > 4 THEN trim(substr(sort_key, 5))
    WHEN lang = 'de' AND sort_key LIKE 'die %'  AND length(sort_key) > 4 THEN trim(substr(sort_key, 5))
    WHEN lang = 'de' AND sort_key LIKE 'das %'  AND length(sort_key) > 4 THEN trim(substr(sort_key, 5))
    WHEN lang = 'de' AND sort_key LIKE 'eine %' AND length(sort_key) > 5 THEN trim(substr(sort_key, 6))
    WHEN lang = 'de' AND sort_key LIKE 'ein %'  AND length(sort_key) > 4 THEN trim(substr(sort_key, 5))
    WHEN lang = 'en' AND sort_key LIKE 'the %'  AND length(sort_key) > 4 THEN trim(substr(sort_key, 5))
    WHEN lang = 'en' AND sort_key LIKE 'an %'   AND length(sort_key) > 3 THEN trim(substr(sort_key, 4))
    WHEN lang = 'en' AND sort_key LIKE 'a %'    AND length(sort_key) > 2 THEN trim(substr(sort_key, 3))
    ELSE sort_key END;

CREATE INDEX items_library_kind_added    ON items (library_id, kind, added_at DESC);
CREATE INDEX items_library_kind_released ON items (library_id, kind, release_date DESC);
CREATE INDEX items_library_kind_vote     ON items (library_id, kind, vote_average DESC);
CREATE INDEX item_texts_sort_key         ON item_texts (item_id, lang, sort_key);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
