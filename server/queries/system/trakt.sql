-- Trakt lists (M7, AN-3, E-7, I-7), written by package metadata: every successful daily run
-- replaces the table as a whole in one transaction; a failed run empties it, so the two start page
-- rows disappear. Package visibility intersects the rows with the library at read time.

-- name: TraktDeleteLists :exec
DELETE FROM trakt_lists;

-- name: TraktInsertEntry :exec
INSERT INTO trakt_lists (list, media, position, tmdb_id, title, year, fetched_at)
VALUES (@list, @media, @position, @tmdb_id, @title, @year, @now);

-- name: TraktCountLists :many
-- Entries per list (films and shows together) fetched at min_fetched or later, before the library
-- intersection (admin status; the start page leaves older entries out as well).
SELECT list, count(*) AS entries FROM trakt_lists WHERE fetched_at >= @min_fetched GROUP BY list ORDER BY list;
