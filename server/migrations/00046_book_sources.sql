-- Book sources (1.26.0, concept 19 stage D3, D6 and D7): audiobooks are matched against Audible,
-- the Deutsche Nationalbibliothek, Apple Books, Open Library and Google Books.
--
--   items.author_display   the unified author name (D6.4, «Vereinheitlichen»); NULL = like author.
--                          Display, sort by author and search take COALESCE(author_display, author);
--                          items.author stays the value of the files.
--   items.book_match_*     the match of a book (columns instead of the JSON items.book_match of D7,
--                          decided at the build): state matched, review (several candidates or a
--                          duration outside the tolerance), none (no hit) or error (a source failed);
--                          NULL = not tried yet (pending). provider, id and region of the matched
--                          entry, the time in Unix ms, the score (0..1), the error of the last run
--                          and whether the match was made in the editor (book_match_manual = 1; the
--                          automatic run never changes such a match).
--   book_match_candidates  the three best candidates of a book in review or error, rank 1 to 3;
--                          candidate is the JSON of the MetadataCandidate the list «Prüfen» shows.
--
-- The audiobook folders that still carry the default list ["local"] get the fillers of D6.2
-- (Marco's decision «alle samt Audible an»), and so do the titles with a file in them (the copy in
-- items.providers). The search triggers of 00015 are replaced so that the item document also holds
-- author_display when it differs from author, and the search index is rebuilt.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE items ADD COLUMN author_display TEXT;
ALTER TABLE items ADD COLUMN book_match_state TEXT
    CHECK (book_match_state IS NULL OR book_match_state IN ('matched', 'review', 'none', 'error'));
ALTER TABLE items ADD COLUMN book_match_provider TEXT;
ALTER TABLE items ADD COLUMN book_match_id TEXT;
ALTER TABLE items ADD COLUMN book_match_region TEXT;
ALTER TABLE items ADD COLUMN book_matched_at INTEGER;
ALTER TABLE items ADD COLUMN book_match_score REAL;
ALTER TABLE items ADD COLUMN book_match_error TEXT;
ALTER TABLE items ADD COLUMN book_match_manual INTEGER NOT NULL DEFAULT 0 CHECK (book_match_manual IN (0, 1));

-- The numbers of the page «Hörbücher: Quellen» and the list «Prüfen».
CREATE INDEX items_book_match_state ON items (book_match_state) WHERE kind = 'audiobook';

CREATE TABLE book_match_candidates (
    item_id     TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    rank        INTEGER NOT NULL CHECK (rank BETWEEN 1 AND 3),
    provider    TEXT    NOT NULL,
    provider_id TEXT    NOT NULL,
    score       REAL    NOT NULL,
    candidate   TEXT    NOT NULL CHECK (json_valid(candidate)),
    PRIMARY KEY (item_id, rank)
) STRICT;

-- The fillers of D6.2 for every audiobook folder with the default list, first on the titles with
-- a file in such a folder (while the folder still says ["local"]), then on the folders.
UPDATE items SET providers = '["local","audible","dnb","applebooks"]'
WHERE kind = 'audiobook' AND providers = '["local"]'
  AND id IN (SELECT m.item_id FROM media_files AS m JOIN library_roots AS r ON r.id = m.root_id
             WHERE r.content = 'audiobook' AND r.providers = '["local"]');
UPDATE library_roots SET providers = '["local","audible","dnb","applebooks"]'
WHERE content = 'audiobook' AND providers = '["local"]';

-- The item document of the search (00015) with author_display when it differs from author.
DROP TRIGGER search_items_insert;
DROP TRIGGER search_items_update;

-- +goose StatementBegin
CREATE TRIGGER search_items_insert AFTER INSERT ON items
BEGIN
    INSERT INTO search_docs (kind, item_id, ref_id, text)
    VALUES ('item', NEW.id, NEW.id,
            trim(coalesce(NEW.title, '') || ' / ' || coalesce(NEW.original_title, '') || ' / ' || coalesce(NEW.author, '')
                 || ' / ' || coalesce(NEW.narrator, '') || ' / ' || coalesce(NEW.series_name, '')
                 || CASE WHEN NEW.author_display IS NOT NULL AND NEW.author_display IS NOT NEW.author
                         THEN ' / ' || NEW.author_display ELSE '' END))
    ON CONFLICT (kind, ref_id, item_id) DO UPDATE SET text = excluded.text;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER search_items_update AFTER UPDATE OF title, original_title, author, author_display, narrator, series_name ON items
WHEN NEW.title IS NOT OLD.title OR NEW.original_title IS NOT OLD.original_title OR NEW.author IS NOT OLD.author
  OR NEW.author_display IS NOT OLD.author_display OR NEW.narrator IS NOT OLD.narrator OR NEW.series_name IS NOT OLD.series_name
BEGIN
    INSERT INTO search_docs (kind, item_id, ref_id, text)
    VALUES ('item', NEW.id, NEW.id,
            trim(coalesce(NEW.title, '') || ' / ' || coalesce(NEW.original_title, '') || ' / ' || coalesce(NEW.author, '')
                 || ' / ' || coalesce(NEW.narrator, '') || ' / ' || coalesce(NEW.series_name, '')
                 || CASE WHEN NEW.author_display IS NOT NULL AND NEW.author_display IS NOT NEW.author
                         THEN ' / ' || NEW.author_display ELSE '' END))
    ON CONFLICT (kind, ref_id, item_id) DO UPDATE SET text = excluded.text;
END;
-- +goose StatementEnd

-- Rebuild: every item document after the new rule, then the FTS table from search_docs.
INSERT INTO search_docs (kind, item_id, ref_id, text)
SELECT 'item', id, id,
       trim(coalesce(title, '') || ' / ' || coalesce(original_title, '') || ' / ' || coalesce(author, '')
            || ' / ' || coalesce(narrator, '') || ' / ' || coalesce(series_name, '')
            || CASE WHEN author_display IS NOT NULL AND author_display IS NOT author
                    THEN ' / ' || author_display ELSE '' END)
FROM items WHERE true
ON CONFLICT (kind, ref_id, item_id) DO UPDATE SET text = excluded.text;
INSERT INTO search_index (search_index) VALUES ('rebuild');

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
