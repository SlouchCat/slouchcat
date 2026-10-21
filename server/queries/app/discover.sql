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

-- Home rows (stream B1). Owner: admin package (writes), discover package (order of the start page).

-- name: ListHomeRows :many
SELECT key, position, hidden, updated_at FROM home_rows ORDER BY position, key;
