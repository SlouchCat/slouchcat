-- The top-level titles by the time they were added (start page, docs/perf/feinschliff.md section 3).
-- Schema only.
--
--   items_top_added  films, series, audiobooks and podcasts (the kinds of a library's top level),
--                    newest first, then by id: the order of the row «Neu hinzugefügt»
--                    (ListRecentlyAdded). It is partial on exactly the term the queries over the
--                    top level carry (i.kind IN ('movie', 'series', 'audiobook', 'podcast')), so
--                    the planner walks these few thousand rows instead of every item.
--
-- Without it the row «Neu hinzugefügt» and the libraries of «Meine Medien» (ListVisibleLibraries)
-- scanned the whole table and evaluated visible_items for every row, episodes and podcast
-- episodes included, and items_added (00003) walks every podcast episode added after the newest
-- titles. On the database of CT 190 (70 000 items, 38 000 of them podcast episodes) each of the
-- two queries took about 30 ms on the development machine.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE INDEX items_top_added ON items (added_at DESC, id DESC)
WHERE kind IN ('movie', 'series', 'audiobook', 'podcast');

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
