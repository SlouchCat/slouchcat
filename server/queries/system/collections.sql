-- Collections (E-6, M7; package store/systemq). TMDB collections are written by package metadata
-- while it matches a film, manual collections by package admin. Times are Unix milliseconds.

-- TMDB collections (metadata).

-- name: UpsertCollection :one
-- A TMDB collection by its tmdb_id; the names and texts follow TMDB on every match.
INSERT INTO collections (id, source, tmdb_id, name, name_de, overview, overview_de, poster_path, created_at, updated_at)
VALUES (@id, 'tmdb', @tmdb_id, @name, @name_de, @overview, @overview_de, @poster_path, @now, @now)
ON CONFLICT (tmdb_id) DO UPDATE
SET name = excluded.name, name_de = excluded.name_de,
    overview = COALESCE(excluded.overview, collections.overview),
    overview_de = COALESCE(excluded.overview_de, collections.overview_de),
    poster_path = COALESCE(excluded.poster_path, collections.poster_path),
    updated_at = excluded.updated_at
RETURNING id;

-- name: UpsertCollectionItem :exec
INSERT INTO collection_items (collection_id, item_id, position, created_at)
VALUES (@collection_id, @item_id, @position, @now)
ON CONFLICT (collection_id, item_id) DO UPDATE SET position = excluded.position;

-- name: DeleteCollectionItemBySource :exec
-- Removes a title from every TMDB collection except keep_id ('' = from all): a film that left
-- its collection. Manual collections keep it.
DELETE FROM collection_items
WHERE item_id = @item_id AND collection_id <> @keep_id
  AND collection_id IN (SELECT c.id FROM collections AS c WHERE c.source = 'tmdb');

-- Administration (admin): unfiltered, titles in plain text.

-- name: AdminListCollections :many
-- Every collection by name with the number of its live members.
SELECT c.id, c.source, c.tmdb_id, c.name, c.name_de,
       (SELECT count(*) FROM collection_items AS ci JOIN items AS i ON i.id = ci.item_id
        WHERE ci.collection_id = c.id AND i.deleted_at IS NULL) AS item_count
FROM collections AS c
ORDER BY c.name COLLATE NOCASE, c.id;

-- name: AdminGetCollection :one
SELECT c.id, c.source, c.tmdb_id, c.name, c.name_de,
       (SELECT count(*) FROM collection_items AS ci JOIN items AS i ON i.id = ci.item_id
        WHERE ci.collection_id = c.id AND i.deleted_at IS NULL) AS item_count
FROM collections AS c
WHERE c.id = @id;

-- name: AdminListCollectionItems :many
-- Live members in order (position, then release date) with kind, title (German text, else
-- English, else the parsed title) and year.
SELECT ci.item_id, ci.position, i.kind, COALESCE(td.title, te.title, i.title) AS title, i.year
FROM collection_items AS ci
JOIN items AS i ON i.id = ci.item_id
LEFT JOIN item_texts AS td ON td.item_id = i.id AND td.lang = 'de'
LEFT JOIN item_texts AS te ON te.item_id = i.id AND te.lang = 'en'
WHERE ci.collection_id = @collection_id AND i.deleted_at IS NULL
ORDER BY ci.position, i.release_date, i.sort_title, i.id;

-- name: AdminInsertCollection :exec
INSERT INTO collections (id, source, name, created_at, updated_at) VALUES (@id, 'manual', @name, @now, @now);

-- name: AdminRenameCollection :exec
UPDATE collections SET name = @name, updated_at = @now WHERE id = @id;

-- name: AdminTouchCollection :exec
UPDATE collections SET updated_at = @now WHERE id = @id;

-- name: AdminDeleteCollection :exec
DELETE FROM collections WHERE id = @id;

-- name: AdminClearCollectionItems :exec
DELETE FROM collection_items WHERE collection_id = @collection_id;

-- name: AdminInsertCollectionItem :exec
INSERT INTO collection_items (collection_id, item_id, position, created_at)
VALUES (@collection_id, @item_id, @position, @now);

-- name: AdminCollectionCandidates :many
-- The kinds of the titles that may become members of a manual collection (one read for the whole
-- list); unknown and deleted ids are absent.
SELECT id, kind FROM items WHERE id IN (sqlc.slice('item_ids')) AND deleted_at IS NULL;
