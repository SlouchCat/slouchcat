-- The hints of the wishes in the administration overview (concept 25 stage 4, administration 2:
-- "n Wuensche ohne Profil-Tag", "n Downloads verzoegert"; package wishes only, store/wishq). Times
-- are Unix milliseconds.

-- name: ListFailedTagHints :many
-- Tags that could not be set, once per wish and service; a tag that waits for the title does not
-- count. The newest wishes first, at most max_rows.
SELECT w.id AS wish_id, w.title, t.service, CAST(MAX(COALESCE(t.last_error, '')) AS TEXT) AS last_error
FROM wish_tags t JOIN wishes w ON w.id = t.wish_id
WHERE t.state = 'failed'
GROUP BY w.id, t.service
ORDER BY w.id DESC
LIMIT @max_rows;

-- name: ListStalledProgressHints :many
-- Wishes in procurement whose download is stalled by a read not older than fresh_since.
SELECT w.id AS wish_id, w.title, p.service
FROM wish_progress p JOIN wishes w ON w.id = p.wish_id
WHERE p.stage = 'stalled' AND w.state IN ('processing', 'partial') AND p.read_at >= @fresh_since
ORDER BY w.id DESC
LIMIT @max_rows;
