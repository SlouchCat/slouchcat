-- Concept 25 stage 3, step 3.4 (08-datenmodell-api.md section 2.11, 07-administration.md
-- section 10; Marco, E-10, N-97): the start page row «Erfüllte Wünsche».
--
--   home_rows        the new key wishes: the profile's wishes fulfilled within the last 14 days,
--                    library titles only (the content comes from package wishes, discover checks
--                    the visibility). SQLite cannot change a CHECK constraint, so the table is
--                    rebuilt; every row keeps its id, key, library, hidden flag and time, and the
--                    rows behind «Weiterschauen» move one place down. The row wishes comes right
--                    after «Weiterschauen» (as in the mockup), hidden when the start page already
--                    shows the most visible rows (15, MaxVisibleHomeRows). The trigger
--                    home_rows_library_added (00035, 00039) names home_rows, so it is dropped before
--                    the rebuild and created again after it, unchanged.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

DROP TRIGGER home_rows_library_added;

CREATE TABLE home_rows_new (
    id         TEXT    PRIMARY KEY CHECK (id = CASE WHEN key = 'library' THEN 'library:' || library_id ELSE key END),
    key        TEXT    NOT NULL CHECK (key IN ('spotlight', 'picks', 'continue', 'recent', 'released',
                                               'watchlist', 'collections', 'genres', 'libraries', 'library',
                                               'radio', 'wishes')),
    library_id INTEGER REFERENCES libraries (id) ON DELETE CASCADE,
    position   INTEGER NOT NULL,
    hidden     INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)),
    updated_at INTEGER NOT NULL,
    CHECK ((key = 'library') = (library_id IS NOT NULL))
) STRICT;

-- The place of «Weiterschauen»; without that row (never in practice) the new row goes last.
CREATE TEMP TABLE home_rows_anchor AS
SELECT COALESCE((SELECT position FROM home_rows WHERE key = 'continue'),
                (SELECT COALESCE(max(position), -1) FROM home_rows)) AS position;

INSERT INTO home_rows_new (id, key, library_id, position, hidden, updated_at)
SELECT id, key, library_id,
       position + CASE WHEN position > (SELECT position FROM home_rows_anchor) THEN 1 ELSE 0 END,
       hidden, updated_at
FROM home_rows;

-- The constant time keeps a fresh database byte for byte the same (storetest template).
INSERT INTO home_rows_new (id, key, library_id, position, hidden, updated_at)
SELECT 'wishes', 'wishes', NULL, (SELECT position FROM home_rows_anchor) + 1,
       CASE WHEN (SELECT count(*) FROM home_rows WHERE hidden = 0) >= 15 THEN 1 ELSE 0 END, 0;

DROP TABLE home_rows_anchor;
DROP TABLE home_rows;
ALTER TABLE home_rows_new RENAME TO home_rows;

-- A new library gets its row «Neu in …», hidden, at the end (00035, unchanged).
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
