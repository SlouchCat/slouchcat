-- The call log of the Seerr view (concept 25, data model 2.8) and the state settings package wishes
-- keeps (seerr.version, seerr.instance, seerr.test, seerr.last_*; data model 2.12). The settings an
-- administrator sets are written by package admin. Package wishes only, store/wishq.

-- name: InsertSeerrLog :exec
-- A trigger keeps the newest 20 rows.
INSERT INTO seerr_log (at, method, path, status, duration_ms, error)
VALUES (@at, @method, @path, @status, @duration_ms, @error);

-- name: ListSeerrLog :many
SELECT * FROM seerr_log ORDER BY id DESC LIMIT 20;

-- name: GetWishSetting :one
SELECT value FROM settings WHERE key = @key;

-- name: UpsertWishSetting :exec
INSERT INTO settings (key, value, updated_at) VALUES (@key, @value, @now)
ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at;

-- name: DeleteWishSetting :exec
DELETE FROM settings WHERE key = @key;
