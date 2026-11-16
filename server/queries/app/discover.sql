-- Discover (package store/appq, M7 design 3.1). The tables name items only by id; the visibility of
-- a title is checked through the visibility package before any of these writes run. Times are Unix
-- milliseconds.

-- Watchlist (stream A). Owner: discover package. Both writes are idempotent; adding again keeps the
-- first created_at. A new entry follows the title from the moment it was added (follow_since, concept
-- 20 P-2: new podcast episodes after it are fetched by the offline rule).

-- name: AddWatchlist :exec
INSERT INTO watchlist (profile_id, item_id, created_at, follow_since) VALUES (@profile_id, @item_id, @now, @now)
ON CONFLICT (profile_id, item_id) DO NOTHING;

-- name: RemoveWatchlist :exec
DELETE FROM watchlist WHERE profile_id = @profile_id AND item_id = @item_id;

-- User picks (concept 18 part B, ADR 0028). Owner: discover package; it checks the visibility of the
-- title and that the profile belongs to the owner's account first. Recommending again renews the
-- time, taking back deletes the own row only.

-- name: UpsertUserPick :exec
INSERT INTO user_picks (item_id, profile_id, created_at) VALUES (@item_id, @profile_id, @now)
ON CONFLICT (item_id, profile_id) DO UPDATE SET created_at = excluded.created_at;

-- name: DeleteUserPick :exec
DELETE FROM user_picks WHERE item_id = @item_id AND profile_id = @profile_id;

-- name: GetPicksRetention :one
SELECT retention_days FROM picks_config WHERE id = 1;

-- Home rows (stream B1): the order and visibility of the start page rows (E-9, concept 15 P-7).
-- A row is named by its id: the key for the fixed rows, 'library:<id>' for a library row
-- (migration 00035). The administration rewrites the positions in one transaction. A library
-- row carries the name of its library (the row title "Neu in ...") and its deleting mark
-- (library.delete): the administration's read leaves such a row out but keeps it in the stored
-- order.

-- name: ListHomeRows :many
SELECT h.id, h.key, h.library_id, COALESCE(l.name, '') AS library_name, h.position, h.hidden, h.updated_at,
       l.deleting_at AS library_deleting_at
FROM home_rows AS h
LEFT JOIN libraries AS l ON l.id = h.library_id
ORDER BY h.position, h.id;

-- name: SetHomeRow :execrows
UPDATE home_rows SET position = @position, hidden = @hidden, updated_at = @now WHERE id = @id;

-- "Meine Medien" (concept 21 P-11, migration 00059): one order of the group for the side navigation
-- and the start page tiles, for every profile, hidden per place. An entry is named by its id:
-- 'library:<id>' for a library, the key for radio, collections and picks. A library entry carries
-- the name and kind of its library and its deleting mark (library.delete): the administration's read
-- leaves such an entry out but keeps it in the stored order. The administration rewrites the
-- positions in one transaction.

-- name: ListMyMedia :many
SELECT m.id, m.key, m.library_id, COALESCE(l.name, '') AS library_name, COALESCE(l.kind, '') AS library_kind,
       m.position, m.hide_sidebar, m.hide_home, l.deleting_at AS library_deleting_at
FROM my_media AS m
LEFT JOIN libraries AS l ON l.id = m.library_id
ORDER BY m.position, m.id;

-- name: SetMyMediaEntry :execrows
UPDATE my_media SET position = @position, hide_sidebar = @hide_sidebar, hide_home = @hide_home, updated_at = @now
WHERE id = @id;
