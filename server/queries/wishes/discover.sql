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

-- name: GetProfileAgeCeiling :one
-- The age ceiling of a profile: the lower of the profile's and the account's (visibility applies
-- the same, architecture section 3). It names the age step of the cache key and the pre-filter
-- certificationLte; the decision per title stays with visibility.
SELECT CAST(min(p.max_rating, a.max_rating) AS INTEGER) AS max_age
FROM profiles AS p
JOIN accounts AS a ON a.id = p.account_id
WHERE p.id = @profile_id AND p.account_id = @account_id;

-- name: ListDiscoverProfiles :many
-- The profiles whose account and profile switch allow requests, with their languages and age
-- ceiling (wishes.discover warms the lists for these). The service resolves the full right.
SELECT p.id AS profile_id, p.account_id, p.language, p.metadata_language,
       CAST(min(p.max_rating, a.max_rating) AS INTEGER) AS max_age
FROM profiles AS p
JOIN accounts AS a ON a.id = p.account_id
WHERE p.can_request = 1 AND a.can_request = 1
ORDER BY p.id;

-- name: ListOpenWishesOfProfile :many
-- The open wishes the profile owns or joined, for the mark "angefordert" on the tiles of a page.
SELECT w.id, w.media, w.tmdb_id, w.state
FROM wish_requesters AS r
JOIN wishes AS w ON w.id = r.wish_id
WHERE r.profile_id = @profile_id AND w.state IN ('queued', 'pending', 'processing', 'partial');
