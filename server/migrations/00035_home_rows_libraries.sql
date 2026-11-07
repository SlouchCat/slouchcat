-- Libraries on the start page (concept 15 P-7, F-11, F-12). Two new row kinds: «Meine Medien»
-- (key libraries, one tile per library) and «Neu in <library>» (key library, one row per library).
-- Several rows of the key library would collide wherever the key names a row, so every row gets an
-- id: the key for the fixed rows, 'library:<library id>' for a library row.
--
--   id          the row's identity (CHECK ties it to key and library_id, so a fixed row exists once
--               and a library has one row at most)
--   key         the kind of the row: the eight rows of 00021 plus libraries and library
--   library_id  the library of a row library, NULL for every other key (CHECK); deleting a library
--               takes its row along (no deleteLibrary exists today, the cascade is a precaution)
--
-- home_rows has to be rebuilt because SQLite cannot change a CHECK constraint or a primary key. The
-- eight rows keep their position, hidden flag and time (id = key). «Meine Medien» and one row per
-- existing library are added hidden behind them, libraries in id order (the order of the side
-- navigation), so the start page looks as before. The trigger home_rows_library_added gives every
-- library created later its row the same way: hidden, at the end.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE home_rows_new (
    id         TEXT    PRIMARY KEY CHECK (id = CASE WHEN key = 'library' THEN 'library:' || library_id ELSE key END),
    key        TEXT    NOT NULL CHECK (key IN ('spotlight', 'picks', 'continue', 'recent', 'released',
                                               'watchlist', 'collections', 'genres', 'libraries', 'library')),
    library_id INTEGER REFERENCES libraries (id) ON DELETE CASCADE,
    position   INTEGER NOT NULL,
    hidden     INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)),
    updated_at INTEGER NOT NULL,
    CHECK ((key = 'library') = (library_id IS NOT NULL))
) STRICT;

INSERT INTO home_rows_new (id, key, library_id, position, hidden, updated_at)
SELECT key, key, NULL, position, hidden, updated_at FROM home_rows;

-- Behind the last stored row: «Meine Medien», then the libraries by id. The positions are counted
-- over the old table, which holds only the eight rows.
INSERT INTO home_rows_new (id, key, library_id, position, hidden, updated_at)
SELECT 'libraries', 'libraries', NULL, COALESCE(max(position), -1) + 1, 1, 0 FROM home_rows;

INSERT INTO home_rows_new (id, key, library_id, position, hidden, updated_at)
SELECT 'library:' || l.id, 'library', l.id,
       (SELECT COALESCE(max(position), -1) + 2 FROM home_rows) + (SELECT count(*) FROM libraries AS o WHERE o.id < l.id),
       1, 0
FROM libraries AS l;

DROP TABLE home_rows;
ALTER TABLE home_rows_new RENAME TO home_rows;

-- A new library gets its row «Neu in …», hidden, at the end (8.3).
-- +goose StatementBegin
CREATE TRIGGER home_rows_library_added AFTER INSERT ON libraries
BEGIN
    INSERT INTO home_rows (id, key, library_id, position, hidden, updated_at)
    VALUES ('library:' || NEW.id, 'library', NEW.id, (SELECT COALESCE(max(position), -1) + 1 FROM home_rows), 1, NEW.created_at);
END;
-- +goose StatementEnd

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
