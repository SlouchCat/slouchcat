-- YouTube trailers per title and language (package store/systemq). Owner: metadata package.
-- Manual rows (source 'manual') survive every trailer.find; times are Unix milliseconds.

-- name: DeleteAutoTrailers :exec
DELETE FROM trailers WHERE item_id = @item_id AND source <> 'manual';

-- name: InsertTrailer :exec
-- An automatic hit equal to a manual row (same language and video) keeps the manual row.
-- size is the TMDB resolution (concept 21 P-1), NULL when unknown.
INSERT INTO trailers (item_id, lang, youtube_id, title, source, official, size, created_at)
VALUES (@item_id, @lang, @youtube_id, @title, @source, @official, @size, @now)
ON CONFLICT (item_id, lang, youtube_id) DO NOTHING;

-- name: UpsertManualTrailer :one
-- A manual trailer replaces an automatic row with the same language and video.
INSERT INTO trailers (item_id, lang, youtube_id, title, source, official, created_at)
VALUES (@item_id, @lang, @youtube_id, NULL, 'manual', 0, @now)
ON CONFLICT (item_id, lang, youtube_id) DO UPDATE SET source = 'manual'
RETURNING *;

-- name: ListTrailersByItem :many
SELECT * FROM trailers WHERE item_id = @item_id ORDER BY lang, official DESC, id;

-- name: DeleteTrailer :execrows
DELETE FROM trailers WHERE id = @id AND item_id = @item_id;

-- name: CountTrailers :one
SELECT count(*) FROM trailers WHERE item_id = @item_id;

-- name: SetTrailerState :exec
UPDATE items
SET trailer_state = @trailer_state, trailer_checked_at = @checked_at, updated_at = @now
WHERE id = @id;

-- name: ListPendingTrailerItems :many
-- Matched movies and series of a library whose trailers are pending without an active trailer job
-- (QueuePendingTrailers).
SELECT i.id FROM items AS i
WHERE i.library_id = @library_id AND i.deleted_at IS NULL
  AND i.kind IN ('movie', 'series') AND i.tmdb_id IS NOT NULL
  AND i.metadata_state = 'matched' AND i.trailer_state = 'pending'
  AND NOT EXISTS (SELECT 1 FROM jobs AS j
                  WHERE j.idempotency_key = 'trailer:' || i.id AND j.state IN ('queued', 'running'))
ORDER BY i.id;

-- name: ListTrailerRecheckItems :many
-- Movies and series whose trailers were found and that carry an automatic trailer: the one-time
-- recheck after the update to concept 21 stage 2 picks their trailers by resolution again
-- (QueueTrailerRecheckOnce, O-3). A title with manual trailers only or a locked trailer stays out.
SELECT i.id FROM items AS i
WHERE i.deleted_at IS NULL AND i.kind IN ('movie', 'series') AND i.tmdb_id IS NOT NULL
  AND i.trailer_state = 'found' AND instr(i.locked_fields, '"trailer"') = 0
  AND EXISTS (SELECT 1 FROM trailers AS t WHERE t.item_id = i.id AND t.source <> 'manual')
ORDER BY i.id;
