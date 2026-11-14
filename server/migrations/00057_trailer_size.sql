-- The resolution of a trailer (concept 21 P-1, O-2 answered 14 November 2026).
--
-- TMDB names the resolution of every video (size: 360, 480, 720, 1080, 2160); trailer.find picks
-- by it since concept 21 (section 2.2) and stores it here, so the administration shows it in the
-- trailer tab and bad trailers can be found later. NULL is unknown: the rows before this
-- migration (until the one-time recheck after the update has run), YouTube hits (the Data API only
-- says hd or sd) and manual trailers. The column takes the TMDB value as it comes, in pixels of
-- the short side.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE trailers ADD COLUMN size INTEGER CHECK (size IS NULL OR size > 0);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
