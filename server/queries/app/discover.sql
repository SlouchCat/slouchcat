-- Discover (package store/appq, M7 design 3.1). The tables name items only by id; the visibility of
-- a title is checked through the visibility package before any of these writes run. Times are Unix
-- milliseconds.

-- Watchlist and favourites (stream A). Owner: discover package. Both writes are idempotent; adding
-- again keeps the first created_at.

-- name: AddWatchlist :exec
INSERT INTO watchlist (profile_id, item_id, created_at) VALUES (@profile_id, @item_id, @now)
ON CONFLICT (profile_id, item_id) DO NOTHING;

-- name: RemoveWatchlist :exec
DELETE FROM watchlist WHERE profile_id = @profile_id AND item_id = @item_id;

-- name: AddFavorite :exec
INSERT INTO favorites (profile_id, item_id, created_at) VALUES (@profile_id, @item_id, @now)
ON CONFLICT (profile_id, item_id) DO NOTHING;

-- name: RemoveFavorite :exec
DELETE FROM favorites WHERE profile_id = @profile_id AND item_id = @item_id;

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
-- row carries the name of its library (the row title "Neu in ...").

-- name: ListHomeRows :many
SELECT h.id, h.key, h.library_id, COALESCE(l.name, '') AS library_name, h.position, h.hidden, h.updated_at
FROM home_rows AS h
LEFT JOIN libraries AS l ON l.id = h.library_id
ORDER BY h.position, h.id;

-- name: SetHomeRow :execrows
UPDATE home_rows SET position = @position, hidden = @hidden, updated_at = @now WHERE id = @id;
