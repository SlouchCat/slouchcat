-- The profile tag in Radarr and Sonarr (concept 25 step 4.6b, data model 2.14; package wishes
-- only, store/wishq). Times are Unix milliseconds. A tag is never removed: there is no query that
-- would forget a set row.

-- name: ListTagCandidates :many
-- The requesters (owner and co-wishers) of the wishes in procurement that have no tag row yet.
-- Wishes placed directly in Seerr (origin seerr) get none (N-27).
SELECT w.id AS wish_id, w.media, r.profile_id, r.account_id
FROM wishes AS w
JOIN wish_requesters AS r ON r.wish_id = w.id
WHERE w.state IN ('processing', 'partial') AND w.origin = 'slouchcat' AND r.closed_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM wish_tags AS t WHERE t.wish_id = w.id AND t.profile_id = r.profile_id)
ORDER BY w.id, r.profile_id;

-- name: GetProfileTag :one
SELECT * FROM wish_profile_tags WHERE profile_id = @profile_id;

-- name: GetTagProfile :one
-- The profile, its account and the name of the account's main profile (the account's name).
SELECT p.id, p.account_id, p.name,
       CAST(COALESCE((SELECT m.name FROM profiles AS m WHERE m.account_id = p.account_id AND m.is_default = 1
                      ORDER BY m.id LIMIT 1), '') AS TEXT) AS account_name
FROM profiles AS p WHERE p.id = @profile_id;

-- name: ListEarlierProfilesWithoutTag :many
-- The profiles created before the given one that have no stored tag name: they keep the plain
-- name if theirs is the same (K-18).
SELECT p.id, p.name FROM profiles AS p
WHERE p.id < @profile_id AND NOT EXISTS (SELECT 1 FROM wish_profile_tags AS t WHERE t.profile_id = p.id)
ORDER BY p.id;

-- name: TagNameOwner :one
-- The profile that holds a stored tag name; no row when the name is free.
SELECT profile_id FROM wish_profile_tags WHERE name = @name;

-- name: InsertProfileTag :execrows
-- The derived name of a profile, written once; a stored name stays (0 rows).
INSERT INTO wish_profile_tags (profile_id, name, custom, set_at) VALUES (@profile_id, @name, 0, @now)
ON CONFLICT (profile_id) DO NOTHING;

-- name: SetCustomProfileTag :exec
-- A name the administrator set (setProfileTagName).
INSERT INTO wish_profile_tags (profile_id, name, custom, set_at) VALUES (@profile_id, @name, 1, @now)
ON CONFLICT (profile_id) DO UPDATE SET name = excluded.name, custom = 1, set_at = excluded.set_at;

-- name: DeleteProfileTag :exec
-- Back to the derived name; the next tag of the profile stores it.
DELETE FROM wish_profile_tags WHERE profile_id = @profile_id;

-- name: CountProfiles :one
SELECT count(*) FROM profiles WHERE id = @profile_id;

-- name: InsertWishTag :execrows
-- A pending row for a requester; an existing row stays (0 rows).
INSERT OR IGNORE INTO wish_tags (wish_id, profile_id, service, tag_name, state)
VALUES (@wish_id, @profile_id, @service, @tag_name, 'pending');

-- name: ListOpenWishTags :many
-- The rows still to set, of the wishes in procurement.
SELECT t.wish_id, t.profile_id, t.service, t.tag_name, t.arr_title_id, t.state, w.media, w.tmdb_id, w.tvdb_id
FROM wish_tags AS t
JOIN wishes AS w ON w.id = t.wish_id
WHERE t.state <> 'set' AND w.state IN ('processing', 'partial')
ORDER BY t.wish_id, t.profile_id;

-- name: SetWishTagDone :exec
UPDATE wish_tags SET state = 'set', arr_tag_id = @arr_tag_id, arr_title_id = @arr_title_id, last_error = NULL,
                     attempts = attempts + 1, set_at = @now
WHERE wish_id = @wish_id AND profile_id = @profile_id AND state <> 'set';

-- name: SetOpenWishTagName :exec
-- The profile's tag name changed before the row was set: the row takes the current one.
UPDATE wish_tags SET tag_name = @tag_name WHERE wish_id = @wish_id AND profile_id = @profile_id AND state <> 'set';

-- name: SetWishTagOpen :exec
-- A try that did not set the tag: pending (title not yet there) or failed (service refused or did
-- not answer). Known ids are kept.
UPDATE wish_tags SET state = @state, last_error = sqlc.narg('last_error'), attempts = attempts + 1,
                     arr_tag_id = COALESCE(sqlc.narg('arr_tag_id'), arr_tag_id),
                     arr_title_id = COALESCE(sqlc.narg('arr_title_id'), arr_title_id)
WHERE wish_id = @wish_id AND profile_id = @profile_id AND state <> 'set';

-- name: CountWishTagsByState :many
-- The counts per service and state for the status of Radarr and Sonarr.
SELECT service, state, count(*) AS n FROM wish_tags GROUP BY service, state ORDER BY service, state;

-- name: ListWishTagsOf :many
-- The tag rows of some wishes (the requesters of the administration's list).
SELECT wish_id, profile_id, service, tag_name, state, last_error, set_at FROM wish_tags
WHERE wish_id IN (sqlc.slice('wish_ids'))
ORDER BY wish_id, profile_id;
