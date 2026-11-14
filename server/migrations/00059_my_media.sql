-- «Meine Medien» (concept 21 P-9 and P-11, O-20 and O-21 answered 14 November 2026): one order of
-- the group for the side navigation and for the tiles of the start page row «Meine Medien», the
-- same for every profile, with hiding per place. Hiding only takes the entry out of that place; the
-- titles stay in the search and the rows, and what a profile may see is still the accounts' rule.
--
--   id            the entry's identity (CHECK ties it to key and library_id): 'library:<id>' for a
--                 library, the key for radio, collections and picks
--   key           library, radio («Radio»), collections («Sammlungen») or picks
--                 («User-Empfehlungen»)
--   library_id    the library of an entry library, NULL for every other key (CHECK); deleting the
--                 library takes its entry along
--   position      the place in the group, 0..n-1 (a deleted library leaves a gap until the next save)
--   hide_sidebar  1: not in the side navigation's group «Meine Medien»
--   hide_home     1: no tile in the start page row «Meine Medien»
--
-- Seeded so that both places look as today or as concept 21 P-9 wants them: the libraries in id
-- order (the order of the side navigation until now), then Radio (moving from the top of the side
-- navigation into the group, with a tile on the start page), Sammlungen and User-Empfehlungen
-- (shown in the side navigation; on the start page hidden, since both have rows of their own). The
-- trigger my_media_library_added gives every library created later its entry directly behind the last
-- library (Radio, Sammlungen and User-Empfehlungen move one place down), visible in both places (O-20,
-- Marco 14 November 2026: «hinter Bibliotheken»).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE my_media (
    id           TEXT    PRIMARY KEY CHECK (id = CASE WHEN key = 'library' THEN 'library:' || library_id ELSE key END),
    key          TEXT    NOT NULL CHECK (key IN ('library', 'radio', 'collections', 'picks')),
    library_id   INTEGER REFERENCES libraries (id) ON DELETE CASCADE,
    position     INTEGER NOT NULL CHECK (position >= 0),
    hide_sidebar INTEGER NOT NULL DEFAULT 0 CHECK (hide_sidebar IN (0, 1)),
    hide_home    INTEGER NOT NULL DEFAULT 0 CHECK (hide_home IN (0, 1)),
    updated_at   INTEGER NOT NULL,
    CHECK ((key = 'library') = (library_id IS NOT NULL))
) STRICT;

CREATE INDEX my_media_library ON my_media (library_id) WHERE library_id IS NOT NULL;

INSERT INTO my_media (id, key, library_id, position, hide_sidebar, hide_home, updated_at)
SELECT 'library:' || l.id, 'library', l.id, (SELECT count(*) FROM libraries AS o WHERE o.id < l.id), 0, 0, 0
FROM libraries AS l;

INSERT INTO my_media (id, key, library_id, position, hide_sidebar, hide_home, updated_at)
SELECT 'radio', 'radio', NULL, count(*), 0, 0, 0 FROM libraries
UNION ALL
SELECT 'collections', 'collections', NULL, count(*) + 1, 0, 1, 0 FROM libraries
UNION ALL
SELECT 'picks', 'picks', NULL, count(*) + 2, 0, 1, 0 FROM libraries;

-- A new library directly behind the last library (first place without one), visible in the side
-- navigation and on the start page (O-20). The entries behind it move one place down.
-- +goose StatementBegin
CREATE TRIGGER my_media_library_added AFTER INSERT ON libraries
BEGIN
    UPDATE my_media SET position = position + 1
    WHERE position >= (SELECT COALESCE(max(position) + 1, 0) FROM my_media WHERE key = 'library');
    INSERT INTO my_media (id, key, library_id, position, hide_sidebar, hide_home, updated_at)
    VALUES ('library:' || NEW.id, 'library', NEW.id,
            (SELECT COALESCE(max(position) + 1, 0) FROM my_media WHERE key = 'library'), 0, 0, NEW.created_at);
END;
-- +goose StatementEnd

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
