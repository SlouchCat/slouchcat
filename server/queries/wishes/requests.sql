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
