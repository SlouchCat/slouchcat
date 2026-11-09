-- Trickplay «Aus» first and default (1.23.1, Marco's decision of 9 November 2026; concept 18,
-- section 6; ADR 0027 addendum): the setting «Vorschau beim Spulen» of a new library is «Aus» for
-- films and series, and every library still at «Im Voraus» (the default of 00042, F-11) is switched
-- to «Aus». on_start, series_start and off stay as they are, and so do trickplay_movies_keep,
-- trickplay_series_keep and trickplay_series_scope.
--
-- The column DEFAULT 'ahead' of 00042 stays in the schema but is unused: changing a column default
-- in SQLite needs a rebuild of libraries, which half the schema references, for a value no code path
-- reads. library.Service.Create writes the trickplay columns of every new library explicitly
-- (model.DefaultLibraryTrickplay, «Aus»).
--
-- Effects: waiting trickplay.generate jobs of these libraries end without work (trickplayWanted),
-- and the next maintenance removes their stored preview sets («Aus» keeps none).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

UPDATE libraries SET trickplay_movies = 'off' WHERE trickplay_movies = 'ahead';
UPDATE libraries SET trickplay_series = 'off' WHERE trickplay_series = 'ahead';

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
