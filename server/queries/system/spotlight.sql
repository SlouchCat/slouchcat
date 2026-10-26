-- Spotlight entries (M11, E-7, I-7, ADR 0011), written by package metadata: every successful run
-- replaces the table as a whole in one transaction; a failed run empties it, so the start page row
-- disappears. Package visibility intersects the entries with the library at read time.

-- name: SpotlightDeleteEntries :exec
DELETE FROM spotlight_entries;

-- name: SpotlightInsertEntry :exec
INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, title, year, fetched_at)
VALUES (@list_key, @media, @position, @tmdb_id, @title, @year, @now);

-- name: SpotlightCountEntries :one
-- Entries (films and shows together) fetched at min_fetched or later, before the library
-- intersection (admin status; the start page leaves older entries out as well).
SELECT count(*) FROM spotlight_entries WHERE fetched_at >= @min_fetched;

-- The administration's view of the fetched list (M11 step 4, plan 5.5): every entry of the list
-- in fetch order with its library title (matched by TMDB id and kind, deleted titles excluded, no
-- profile filter: the administration is system-wide, S-4 shows entries without a hit as well) and
-- its override. The title text is German, else English, else the parsed title, like the other
-- administration views.
-- name: ListSpotlightEntriesAdmin :many
SELECT e.media, e.position, e.tmdb_id, e.title, e.year,
       i.id AS item_id, COALESCE(td.title, te.title, i.title, '') AS item_title,
       o.hidden AS override_hidden, o.pinned_rank, o.note
FROM spotlight_entries AS e
LEFT JOIN items AS i ON i.tmdb_id = e.tmdb_id
                    AND i.kind = CASE e.media WHEN 'movie' THEN 'movie' ELSE 'series' END
                    AND i.deleted_at IS NULL
LEFT JOIN item_texts AS td ON td.item_id = i.id AND td.lang = 'de'
LEFT JOIN item_texts AS te ON te.item_id = i.id AND te.lang = 'en'
LEFT JOIN spotlight_overrides AS o ON o.list_key = e.list_key AND o.media = e.media AND o.tmdb_id = e.tmdb_id
WHERE e.list_key = @list_key
ORDER BY e.media, e.position;

-- Overrides of a list whose title is not among the fetched entries (orphans, plan 7.2); the
-- administration shows and deletes them.
-- name: ListSpotlightOrphans :many
SELECT o.media, o.tmdb_id, o.hidden, o.pinned_rank, o.note
FROM spotlight_overrides AS o
WHERE o.list_key = @list_key
  AND NOT EXISTS (SELECT 1 FROM spotlight_entries AS e
                  WHERE e.list_key = o.list_key AND e.media = o.media AND e.tmdb_id = o.tmdb_id)
ORDER BY o.media, o.tmdb_id;

-- name: GetSpotlightOverride :one
SELECT * FROM spotlight_overrides WHERE list_key = @list_key AND media = @media AND tmdb_id = @tmdb_id;

-- name: CountSpotlightEntry :one
SELECT count(*) FROM spotlight_entries WHERE list_key = @list_key AND media = @media AND tmdb_id = @tmdb_id;

-- The partial unique index spotlight_overrides_rank refuses a second entry on the same rank of a
-- list; the caller maps that constraint error to 409 rank_taken.
-- name: UpsertSpotlightOverride :exec
INSERT INTO spotlight_overrides (list_key, media, tmdb_id, hidden, pinned_rank, note, updated_at)
VALUES (@list_key, @media, @tmdb_id, @hidden, @pinned_rank, @note, @now)
ON CONFLICT (list_key, media, tmdb_id) DO UPDATE
SET hidden = excluded.hidden, pinned_rank = excluded.pinned_rank, note = excluded.note, updated_at = excluded.updated_at;

-- name: DeleteSpotlightOverride :execrows
DELETE FROM spotlight_overrides WHERE list_key = @list_key AND media = @media AND tmdb_id = @tmdb_id;
