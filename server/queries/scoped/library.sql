-- Library views of concept 08 (ADR 0016): the side navigation and the facets of the library page.
-- A library exists for a profile only through a visible top-level title (films, series,
-- audiobooks, podcasts; never seasons, episodes or extras), derived from visible_items and not
-- from a second copy of the library rights (concept 08, 4.3). The page itself is built in Go
-- (visibility/library.go), its statements join visible_items like these.

-- Libraries with at least one visible top-level title, administration order, with the number of
-- those titles.
-- name: ListVisibleLibraries :many
SELECT l.id, l.name, l.kind, count(*) AS item_count
FROM libraries AS l
JOIN items AS i ON i.library_id = l.id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.kind IN ('movie', 'series', 'audiobook', 'podcast')
GROUP BY l.id
ORDER BY l.id;

-- One library with at least one visible top-level title; no row when it has none, is unknown or
-- not permitted (404, never 403).
-- name: GetVisibleLibrary :one
SELECT l.id, l.name, l.kind, count(*) AS item_count
FROM libraries AS l
JOIN items AS i ON i.library_id = l.id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE l.id = @library_id AND i.kind IN ('movie', 'series', 'audiobook', 'podcast')
GROUP BY l.id;

-- The folder contents (library_roots.content) of the libraries with a visible top-level title,
-- each once per library; the caller puts them in enum order.
-- name: ListVisibleLibraryContents :many
SELECT DISTINCT r.library_id, r.content
FROM library_roots AS r
WHERE EXISTS (SELECT 1 FROM items AS i
              JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
              WHERE i.library_id = r.library_id AND i.kind IN ('movie', 'series', 'audiobook', 'podcast'))
ORDER BY r.library_id, r.content;

-- Genres of the visible top-level titles of one library in one language with their counts, most
-- frequent first (facets.genres).
-- name: ListLibraryGenres :many
SELECT g.name, count(*) AS item_count
FROM item_genres AS g
JOIN items AS i ON i.id = g.item_id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE g.lang = @lang AND i.library_id = @library_id AND i.kind IN ('movie', 'series', 'audiobook', 'podcast')
GROUP BY g.name
ORDER BY item_count DESC, g.name;

-- The known effective ratings of the visible top-level titles of one library, ascending
-- (facets.ratings); visible_items already caps them at the profile's ceiling.
-- name: ListLibraryRatings :many
SELECT DISTINCT i.effective_rating
FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.library_id = @library_id AND i.kind IN ('movie', 'series', 'audiobook', 'podcast')
  AND i.effective_rating IS NOT NULL
ORDER BY i.effective_rating;
