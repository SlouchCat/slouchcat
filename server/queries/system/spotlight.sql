-- Spotlight entries (M11, E-7, I-7, ADR 0011), written by package metadata: every successful run
-- replaces the table as a whole in one transaction; a failed run empties it, so the start page row
-- disappears. Package visibility intersects the entries with the library at read time.

-- name: SpotlightDeleteEntries :exec
DELETE FROM spotlight_entries;

-- name: SpotlightInsertEntry :exec
INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, title, year, fetched_at)
VALUES (@list_key, @media, @position, @tmdb_id, @title, @year, @now);

-- name: SpotlightCountEntries :one
-- Entries (films and shows together) fetched at min_fetched or later, before the library
-- intersection (admin status; the start page leaves older entries out as well).
SELECT count(*) FROM spotlight_entries WHERE fetched_at >= @min_fetched;
