-- The wishes, their seasons, requesters and history (concept 25, data model 2.4 to 2.7; package
-- wishes only, store/wishq). These tables are not items: a wish never joins items, item_id is the
-- value package visibility returned (CLAUDE.md rule 2). Times are Unix milliseconds. The open
-- states are queued, pending, processing and partial.

-- name: InsertWish :one
-- A new wish. A second open film wish for the same TMDB id fails on wishes_open_movie; the
-- service then joins the existing one.
INSERT INTO wishes (media, tmdb_id, tvdb_id, imdb_id, title, year, origin, seerr_request_id,
                    seerr_media_id, seerr_user_id, state, attempts, next_attempt_at, created_at,
                    updated_at, submitted_at)
VALUES (@media, @tmdb_id, sqlc.narg('tvdb_id'), sqlc.narg('imdb_id'), @title, sqlc.narg('year'), @origin,
        sqlc.narg('seerr_request_id'), sqlc.narg('seerr_media_id'), sqlc.narg('seerr_user_id'), @state, 0,
        sqlc.narg('next_attempt_at'), @now, @now, sqlc.narg('submitted_at'))
RETURNING id;

-- name: GetWish :one
SELECT * FROM wishes WHERE id = @id;

-- name: GetOpenMovieWish :one
-- The open film wish of a TMDB id (at most one, wishes_open_movie).
SELECT * FROM wishes
WHERE media = 'movie' AND tmdb_id = @tmdb_id AND state IN ('queued', 'pending', 'processing');

-- name: ListOpenWishesByTitle :many
-- The open wishes of a title: a film by TMDB id; a show by TVDB id first, then TMDB id (the order
-- of the library, architecture section 5). A show without TVDB id passes NULL for it.
SELECT * FROM wishes
WHERE state IN ('queued', 'pending', 'processing', 'partial') AND media = @media
  AND (tmdb_id = @tmdb_id OR (media = 'show' AND tvdb_id IS NOT NULL AND tvdb_id = sqlc.narg('tvdb_id')))
ORDER BY id;

-- name: ListOpenWishes :many
-- Every open wish, oldest first (wishes.reconcile).
SELECT * FROM wishes WHERE state IN ('queued', 'pending', 'processing', 'partial') ORDER BY id;

-- name: GetWishBySeerrRequest :one
SELECT * FROM wishes WHERE seerr_request_id = @seerr_request_id;

-- name: ListDueWishes :many
-- The outbox: queued wishes whose next attempt is due, oldest attempt first.
SELECT * FROM wishes
WHERE state = 'queued' AND (next_attempt_at IS NULL OR next_attempt_at <= @now)
ORDER BY next_attempt_at, id
LIMIT @limit_count;

-- name: ListDueWithdrawals :many
-- Withdrawn wishes whose request still has to be deleted in Seerr.
SELECT * FROM wishes
WHERE withdraw_pending = 1 AND (next_attempt_at IS NULL OR next_attempt_at <= @now)
ORDER BY next_attempt_at, id
LIMIT @limit_count;

-- name: SetWishState :execrows
-- Moves a wish to a state. fail_reason must be set exactly for failed (CHECK); closed_at is set by
-- the caller for the closing states.
UPDATE wishes SET state = @state, state_detail = sqlc.narg('state_detail'), fail_reason = sqlc.narg('fail_reason'),
                  updated_at = @now, approved_at = COALESCE(sqlc.narg('approved_at'), approved_at),
                  available_at = COALESCE(sqlc.narg('available_at'), available_at),
                  closed_at = sqlc.narg('closed_at')
WHERE id = @id;

-- name: SetWishSubmitted :execrows
-- The request reached Seerr: request and media id (the request id only for a film), the state
-- Seerr answered with and the time.
UPDATE wishes SET seerr_request_id = sqlc.narg('seerr_request_id'), seerr_media_id = sqlc.narg('seerr_media_id'),
                  state = @state, fail_reason = NULL, attempts = attempts + 1, next_attempt_at = NULL,
                  submitted_at = @now, updated_at = @now, seerr_updated_at = sqlc.narg('seerr_updated_at')
WHERE id = @id;

-- name: SetWishAttempt :execrows
-- A failed attempt of the outbox: count it and set the next one.
UPDATE wishes SET attempts = attempts + 1, next_attempt_at = @next_attempt_at, updated_at = @now
WHERE id = @id;

-- name: SetWishWithdrawPending :execrows
UPDATE wishes SET withdraw_pending = @withdraw_pending, next_attempt_at = sqlc.narg('next_attempt_at'), updated_at = @now
WHERE id = @id;

-- name: SetWishItem :execrows
-- The library item package visibility found for the wish (NULL clears it).
UPDATE wishes SET item_id = sqlc.narg('item_id'), updated_at = @now WHERE id = @id;

-- name: SetWishChecked :exec
UPDATE wishes SET checked_at = @now WHERE id = @id;

-- name: SetWishExternalIDs :execrows
-- The TVDB and IMDb ids found later (a show needs its TVDB id before it is sent).
UPDATE wishes SET tvdb_id = COALESCE(sqlc.narg('tvdb_id'), tvdb_id), imdb_id = COALESCE(sqlc.narg('imdb_id'), imdb_id),
                  updated_at = @now
WHERE id = @id;

-- name: UpsertWishSeason :exec
-- One season of a series wish. tmdb_id must be the wish's (trigger wish_seasons_copy_insert); a
-- second open row for the same TMDB id and season fails on wish_seasons_open.
INSERT INTO wish_seasons (wish_id, season_number, tmdb_id, seerr_request_id, seerr_updated_at, state)
VALUES (@wish_id, @season_number, @tmdb_id, sqlc.narg('seerr_request_id'), sqlc.narg('seerr_updated_at'), @state)
ON CONFLICT (wish_id, season_number) DO UPDATE SET
    seerr_request_id = COALESCE(excluded.seerr_request_id, wish_seasons.seerr_request_id),
    seerr_updated_at = COALESCE(excluded.seerr_updated_at, wish_seasons.seerr_updated_at),
    state = excluded.state;

-- name: SetWishSeasonState :execrows
UPDATE wish_seasons SET state = @state, available_at = COALESCE(sqlc.narg('available_at'), available_at)
WHERE wish_id = @wish_id AND season_number = @season_number;

-- name: SetWishSeasonNotified :execrows
-- Marks the season reported once; a second call for a reported season changes nothing.
UPDATE wish_seasons SET notified_at = @now
WHERE wish_id = @wish_id AND season_number = @season_number AND notified_at IS NULL;

-- name: ListWishSeasons :many
SELECT * FROM wish_seasons WHERE wish_id = @wish_id ORDER BY season_number;

-- name: GetOpenSeasonWish :one
-- The wish that holds a season of a TMDB id open (at most one, wish_seasons_open).
SELECT wish_id FROM wish_seasons
WHERE tmdb_id = @tmdb_id AND season_number = @season_number AND state IN ('queued', 'pending', 'processing');

-- name: ListWishSeasonsByRequest :many
-- The seasons a Seerr request carries (the sync).
SELECT * FROM wish_seasons WHERE seerr_request_id = @seerr_request_id ORDER BY wish_id, season_number;

-- name: InsertWishRequester :execrows
-- The owner or a profile that joins; joining twice changes nothing (0 rows).
INSERT OR IGNORE INTO wish_requesters (wish_id, profile_id, account_id, role, created_at)
VALUES (@wish_id, @profile_id, @account_id, @role, @now);

-- name: ListWishRequesters :many
-- The requesters of a wish, the owner first.
SELECT * FROM wish_requesters WHERE wish_id = @wish_id ORDER BY role = 'joined', created_at, profile_id;

-- name: GetWishRequester :one
SELECT * FROM wish_requesters WHERE wish_id = @wish_id AND profile_id = @profile_id;

-- name: SetWishRequesterNotified :execrows
-- Marks "verfuegbar" reported for the profile; a second call changes nothing.
UPDATE wish_requesters SET notified_at = @now
WHERE wish_id = @wish_id AND profile_id = @profile_id AND notified_at IS NULL;

-- name: ListProfileWishes :many
-- "Meine Wuensche": the wishes of a profile (owner or joined), newest first.
SELECT w.*, r.role, r.created_at AS requested_at, r.notified_at
FROM wish_requesters AS r
JOIN wishes AS w ON w.id = r.wish_id
WHERE r.profile_id = @profile_id
ORDER BY r.created_at DESC, w.id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: ListAccountWishes :many
-- listAccountWishes: the wishes of every profile of an account, one row per profile and wish,
-- newest first.
SELECT w.*, r.role, r.created_at AS requested_at, r.notified_at, r.profile_id, p.name AS profile_name
FROM wish_requesters AS r
JOIN wishes AS w ON w.id = r.wish_id
JOIN profiles AS p ON p.id = r.profile_id
WHERE r.account_id = @account_id
ORDER BY r.created_at DESC, w.id DESC, r.profile_id
LIMIT @limit_count;

-- name: ProfileHasWish :one
-- Whether the profile requested this title, in any state (the exception of F-9: a profile sees a
-- title it wished for without the request right).
SELECT EXISTS (
    SELECT 1 FROM wish_requesters AS r JOIN wishes AS w ON w.id = r.wish_id
    WHERE r.profile_id = @profile_id AND w.media = @media AND w.tmdb_id = @tmdb_id
) AS has_wish;

-- name: InsertWishEvent :exec
INSERT INTO wish_events (wish_id, at, kind, detail) VALUES (@wish_id, @now, @kind, sqlc.narg('detail'));

-- name: ListWishEvents :many
-- The newest events of a wish, newest first.
SELECT * FROM wish_events WHERE wish_id = @wish_id ORDER BY at DESC, id DESC LIMIT @limit_count;

-- name: LastWishEventAt :one
-- When the wish last had an event of this kind; 0 = never (the 24 h of the outbox count from a
-- retry of the administration).
SELECT CAST(COALESCE(MAX(at), 0) AS INTEGER) AS at FROM wish_events WHERE wish_id = @wish_id AND kind = @kind;

-- name: CountProfileWishesSince :one
-- The wishes a profile made or joined since a time (the limit of createWish per profile and hour).
SELECT count(*) FROM wish_requesters WHERE profile_id = @profile_id AND created_at >= @since;
