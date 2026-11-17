-- Notifications about wishes (concept 25, 06-benachrichtigungen.md), package notify. notify reads
-- the wish and its requesters to address the messages, and writes the inbox entry together with
-- wish_requesters.notified_at, the guard that keeps a second message about the same wish out also
-- after the inbox entry was cleared away (section 7). Package wishes owns every other column.

-- name: NotifyWish :one
-- The wish a message is about.
SELECT id, media, title, state FROM wishes WHERE id = @id;

-- name: NotifyWishRequesters :many
-- The open requesters of a wish (owner and joined profiles, closed_at unset) with what the
-- messages need of their profile: name, languages, PIN and the notified_at guard.
SELECT r.profile_id, r.account_id, r.role, r.notified_at,
       p.name, p.max_rating, p.language, p.metadata_language, p.pin_hash IS NOT NULL AS has_pin
FROM wish_requesters AS r
JOIN profiles AS p ON p.id = r.profile_id
WHERE r.wish_id = @wish_id AND r.closed_at IS NULL
ORDER BY r.role = 'joined', r.created_at, r.profile_id;

-- name: NotifyWishRequesterClaim :execrows
-- Sets notified_at of one requester when it is unset; 0 rows when the profile was notified before.
UPDATE wish_requesters SET notified_at = @now
WHERE wish_id = @wish_id AND profile_id = @profile_id AND notified_at IS NULL;

-- name: NotifyWishInsert :execresult
-- One inbox entry of a wish per profile and dedupe key; a second one inserts nothing.
INSERT OR IGNORE INTO notifications (profile_id, kind, item_id, parent_id, wish_id, dedupe_key, created_at)
VALUES (@profile_id, @kind, sqlc.narg('item_id'), sqlc.narg('parent_id'), @wish_id, @dedupe_key, @now);

-- name: NotifyWishSeason :one
-- One wished season with the guard of its message (wish_seasons.notified_at).
SELECT state, notified_at FROM wish_seasons WHERE wish_id = @wish_id AND season_number = @season_number;

-- name: NotifyWishSeasonNotified :exec
-- Sets notified_at of a season once every open requester got its message.
UPDATE wish_seasons SET notified_at = @now
WHERE wish_id = @wish_id AND season_number = @season_number AND notified_at IS NULL;

-- name: NotifyWishSeasonsNotifiedAll :exec
-- Sets notified_at of every season of a wish that is available as a whole: no season message
-- follows the message about the series.
UPDATE wish_seasons SET notified_at = @now WHERE wish_id = @wish_id AND notified_at IS NULL;

-- name: NotifySeasonItem :one
-- The season of a library series by its number (the link and the inbox entry of a season message).
SELECT id FROM items
WHERE parent_id = @series_id AND kind = 'season' AND season_number = @season_number AND deleted_at IS NULL
ORDER BY id
LIMIT 1;

-- name: NotifyWishOwner :one
-- The account e-mail and profile name of the owner of a wish (the text of wish_review).
SELECT a.email, p.name
FROM wish_requesters AS r
JOIN accounts AS a ON a.id = r.account_id
JOIN profiles AS p ON p.id = r.profile_id
WHERE r.wish_id = @wish_id AND r.role = 'owner';
