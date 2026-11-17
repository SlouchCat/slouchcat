-- The limit per account when it deviates from the default (concept 25, data model 2.13; package
-- wishes only, store/wishq). No row = "Standard".

-- name: GetAccountWishLimit :one
SELECT * FROM wish_account_limits WHERE account_id = @account_id;

-- name: ListAccountWishLimits :many
SELECT * FROM wish_account_limits ORDER BY account_id;

-- name: UpsertAccountWishLimit :exec
INSERT INTO wish_account_limits (account_id, mode, movie_limit, movie_days, tv_limit, tv_days, set_at, set_by)
VALUES (@account_id, @mode, @movie_limit, @movie_days, @tv_limit, @tv_days, @now, @set_by)
ON CONFLICT (account_id) DO UPDATE SET
    mode = excluded.mode, movie_limit = excluded.movie_limit, movie_days = excluded.movie_days,
    tv_limit = excluded.tv_limit, tv_days = excluded.tv_days, set_at = excluded.set_at,
    set_by = excluded.set_by;

-- name: DeleteAccountWishLimit :exec
DELETE FROM wish_account_limits WHERE account_id = @account_id;
