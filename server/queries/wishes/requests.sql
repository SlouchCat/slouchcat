-- Requesting, the outbox, withdrawing, the sync and the reconcile of the wishes (concept 25,
-- architecture 6 to 8 and 11; build steps 2.4 and 2.5). Package wishes only (store/wishq). Times
-- are Unix milliseconds.

-- name: CountQueuedMovieWishes :one
-- Film wishes of a Seerr user still waiting in the outbox: Seerr does not count them yet (N-35).
SELECT count(*) FROM wishes
WHERE state = 'queued' AND media = 'movie' AND seerr_user_id = @seerr_user_id;

-- name: CountQueuedSeasons :one
-- Seasons of series wishes of a Seerr user still waiting in the outbox (N-35).
SELECT count(*) FROM wish_seasons AS s JOIN wishes AS w ON w.id = s.wish_id
WHERE w.state = 'queued' AND w.media = 'show' AND w.seerr_user_id = @seerr_user_id AND s.state = 'queued';

-- name: DeleteWishRequester :execrows
-- A profile that joined withdraws only itself (architecture 6.6).
DELETE FROM wish_requesters WHERE wish_id = @wish_id AND profile_id = @profile_id;

-- name: SetWishRequesterRole :execrows
UPDATE wish_requesters SET role = @role WHERE wish_id = @wish_id AND profile_id = @profile_id;

-- name: ListOpenWishRequesters :many
-- The requesters that still follow the wish, the owner first.
SELECT * FROM wish_requesters WHERE wish_id = @wish_id AND closed_at IS NULL
ORDER BY role = 'joined', created_at, profile_id;

-- name: ListWishRequesterNames :many
-- The requesters of a wish with account email and profile name (administration).
SELECT r.profile_id, r.account_id, r.role, r.created_at, r.closed_at, a.email AS account_email, p.name AS profile_name
FROM wish_requesters AS r
JOIN accounts AS a ON a.id = r.account_id
JOIN profiles AS p ON p.id = r.profile_id
WHERE r.wish_id = @wish_id
ORDER BY r.role = 'joined', r.created_at, r.profile_id;

-- name: SetWishSeasonRequest :execrows
-- The Seerr request that carries a season and the state the season follows.
UPDATE wish_seasons SET seerr_request_id = sqlc.narg('seerr_request_id'), seerr_updated_at = sqlc.narg('seerr_updated_at'),
                        state = @state
WHERE wish_id = @wish_id AND season_number = @season_number;

-- name: SetWishSeerr :execrows
-- The request and media id of a film wish and the time Seerr last changed the request (sync).
UPDATE wishes SET seerr_request_id = COALESCE(sqlc.narg('seerr_request_id'), seerr_request_id),
                  seerr_media_id = COALESCE(sqlc.narg('seerr_media_id'), seerr_media_id),
                  seerr_updated_at = COALESCE(sqlc.narg('seerr_updated_at'), seerr_updated_at), updated_at = @now
WHERE id = @id;

-- name: ListAllWishes :many
-- Every wish for the administration, the latest change first.
SELECT * FROM wishes ORDER BY updated_at DESC, id DESC LIMIT @limit_count;

-- name: ListWishesToCheck :many
-- Open wishes with a request that Seerr did not change for a day and that were not checked one by
-- one for a day (architecture 7.1: GET /request/{id} at most once per 24 h per wish).
SELECT * FROM wishes
WHERE state IN ('pending', 'processing', 'partial') AND updated_at <= @cutoff
  AND (checked_at IS NULL OR checked_at <= @cutoff)
  AND (seerr_request_id IS NOT NULL
       OR EXISTS (SELECT 1 FROM wish_seasons AS s WHERE s.wish_id = wishes.id AND s.seerr_request_id IS NOT NULL))
ORDER BY checked_at IS NOT NULL, checked_at, id
LIMIT @limit_count;

-- name: ListUnnotifiedAvailableWishes :many
-- Available wishes with a requester that still waits for its message (a profile that cannot see
-- the title yet); the reconcile reports them again.
SELECT * FROM wishes AS w
WHERE w.state = 'available' AND w.available_at >= @since
  AND EXISTS (SELECT 1 FROM wish_requesters AS r WHERE r.wish_id = w.id AND r.closed_at IS NULL AND r.notified_at IS NULL)
ORDER BY w.id;

-- name: ListOpenShowWishesByTVDB :many
-- Open series wishes of a TVDB id (the hook after the match of a TVDB-first series).
SELECT * FROM wishes
WHERE media = 'show' AND tvdb_id = @tvdb_id AND state IN ('queued', 'pending', 'processing', 'partial')
ORDER BY id;

-- name: ListOpenMovieWishesByIMDb :many
-- Open film wishes of an IMDb id (the hook after the match of a film known by IMDb id only).
SELECT * FROM wishes
WHERE media = 'movie' AND imdb_id = @imdb_id AND state IN ('queued', 'pending', 'processing')
ORDER BY id;

-- name: SetWishSeerrUser :execrows
-- A queued wish that nothing of has reached Seerr goes out as another user (the hand-over of a
-- deleted account's wish to a co-wisher of another account).
UPDATE wishes SET seerr_user_id = @seerr_user_id, updated_at = @now
WHERE id = @id AND state = 'queued' AND seerr_request_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM wish_seasons AS s WHERE s.wish_id = wishes.id AND s.seerr_request_id IS NOT NULL);

-- name: ListAccountWishIDs :many
-- The wishes an account requested or joined (before the account is deleted).
SELECT DISTINCT wish_id FROM wish_requesters WHERE account_id = @account_id ORDER BY wish_id;

-- name: ListProfileOwnedWishIDs :many
-- The wishes a profile owns (before the profile is deleted).
SELECT wish_id FROM wish_requesters WHERE profile_id = @profile_id AND role = 'owner' ORDER BY wish_id;

-- name: DeleteAccountWishRequesters :execrows
-- The requesters of a wish that belong to an account (its deletion hands the wish over first).
DELETE FROM wish_requesters WHERE wish_id = @wish_id AND account_id = @account_id;

-- name: MoveWishRequester :execrows
-- A requester row goes to another profile with its role, times and message guard.
UPDATE wish_requesters SET profile_id = @to_profile_id WHERE wish_id = @wish_id AND profile_id = @from_profile_id;

-- name: ClearWishRequest :execrows
-- The request id of a film wish goes (into the history); a closed wish no longer holds it.
UPDATE wishes SET seerr_request_id = NULL, seerr_updated_at = NULL, updated_at = @now WHERE id = @id;

-- name: ClearWishSeasonRequests :execrows
-- The request ids of the seasons of a wish go (into the history).
UPDATE wish_seasons SET seerr_request_id = NULL, seerr_updated_at = NULL WHERE wish_id = @wish_id;

-- name: SetWishClosedAt :execrows
-- A closed wish gets its closing time once.
UPDATE wishes SET closed_at = COALESCE(closed_at, @now), updated_at = @now WHERE id = @id;

-- name: GetPrimaryProfile :one
-- The primary profile of an account (it takes the wishes of a deleted profile).
SELECT id FROM profiles WHERE account_id = @account_id AND is_default = 1;
