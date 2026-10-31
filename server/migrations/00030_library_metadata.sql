-- Metadata settings per library (concept 10 step 3, ADR 0018, decision 16): the deviation of a
-- library from the server's metadata settings, two switches in the interface ("Sprachen
-- abweichend", "Bilder abweichend"). NULL in a column means "like the server"; no row means
-- everything like the server. text_langs is the ordered JSON list of the library's text
-- languages (must hold de or en, E-9/F-9); images is the JSON object
-- {"langs":{"poster":[…],"backdrop":[…],"logo":[…]},"kinds":[…]} of the image chains and the
-- switchable kinds. Minimum width, vote rule, local files, rating country, adult search and
-- Fanart.tv are server settings only (F-6) and live in settings (metadata.*, no DDL needed).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE library_metadata (
    library_id INTEGER NOT NULL PRIMARY KEY REFERENCES libraries(id) ON DELETE CASCADE,
    text_langs TEXT    CHECK (text_langs IS NULL OR json_valid(text_langs)),
    images     TEXT    CHECK (images IS NULL OR json_valid(images)),
    updated_at INTEGER NOT NULL
) STRICT, WITHOUT ROWID;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
