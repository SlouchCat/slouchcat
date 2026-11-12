-- The index of missing media files covers only the missing ones (12 November 2026).
--
-- media_files_missing (00003) held every file, also the present ones with missing_since NULL. Without
-- ANALYZE statistics SQLite takes an equality on an index as selective, so a correlated subquery
-- with "m.missing_since IS NULL" walked media_files_missing for the IS NULL and read the table row
-- of every present file of the server, once per outer row, instead of going from the episodes or
-- titles through media_files_item. On CT 190 (61 749 files, 713 of them missing, 106 podcasts)
-- PodcastList took 47 s, PodcastDownloadStats 47 s and PodcastNewestRoots 93 s: the list of the
-- page «Podcasts» of the administration stayed empty for minutes (docs/perf/podcasts-liste.md).
-- CountLibraryMediaFiles (INDEXED BY media_files_missing on the missing rows) and
-- ListKeyframesBackfillFiles (INDEXED BY media_files_keyframes) had worked around the same trap;
-- this migration removes the trap for every query.
--
-- Partial on missing_since IS NOT NULL, the index can no longer serve an IS NULL; every search for
-- missing files (IS NOT NULL, missing_since < ?) still finds it.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

DROP INDEX media_files_missing;
CREATE INDEX media_files_missing ON media_files (missing_since) WHERE missing_since IS NOT NULL;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
