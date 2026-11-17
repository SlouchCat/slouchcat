-- The cache of the discover lists of the wishes area (concept 25 step 5.1, data model 2.15;
-- package wishes only, store/wishq). Times are Unix milliseconds.

-- name: GetWishDiscoverPage :one
SELECT * FROM wish_discover_pages WHERE cache_key = @cache_key;

-- name: UpsertWishDiscoverPage :exec
INSERT INTO wish_discover_pages (cache_key, items, total_pages, fetched_at)
VALUES (@cache_key, @items, sqlc.narg('total_pages'), @fetched_at)
ON CONFLICT (cache_key) DO UPDATE SET
    items = excluded.items, total_pages = excluded.total_pages, fetched_at = excluded.fetched_at;

-- name: DeleteOldWishDiscoverPages :execrows
-- Rows older than the cut-off (wishes.discover keeps 24 hours).
DELETE FROM wish_discover_pages WHERE fetched_at < @before;
