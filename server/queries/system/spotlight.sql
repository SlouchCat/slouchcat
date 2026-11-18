-- Spotlight entries (M11, E-7, I-7, ADR 0011), written by package metadata: every successful run
-- replaces the table as a whole in one transaction; a failed run empties it, so the start page row
-- disappears. Package visibility intersects the entries with the library at read time.

-- name: SpotlightDeleteEntries :exec
DELETE FROM spotlight_entries;

-- tmdb_id NULL marks a show known by its TVDB id only that TMDB find did not resolve
-- (unmatched, concept 25 N-62, migration 00071).
-- name: SpotlightInsertEntry :exec
INSERT INTO spotlight_entries (list_key, media, position, tmdb_id, tvdb_id, imdb_id, title, year, fetched_at)
VALUES (@list_key, @media, @position, @tmdb_id, @tvdb_id, @imdb_id, @title, @year, @now);

-- name: SpotlightCountEntries :one
-- Entries (films and shows together) fetched at min_fetched or later, before the library
-- intersection (admin status; the start page leaves older entries out as well).
SELECT count(*) FROM spotlight_entries WHERE fetched_at >= @min_fetched;

-- The administration's view of the fetched list (M11 step 4, plan 5.5; concept 25 step 3.3): every
-- entry of the list in fetch order with its three ids, its library title and its override. The
-- library title is matched like the start page (concept 25 N-61) but without a profile filter (the
-- administration is system-wide, S-4 shows entries without a hit as well): a film by TMDB id, then
-- IMDb id; a show by TVDB id, then TMDB id, then IMDb id; deleted titles excluded; match_rung names the
-- rung (tvdb, tmdb, imdb). The title text is German, else English, else the parsed title, like the
-- other administration views. Unmatched entries (no TMDB id, migration 00071) are listed too; they
-- have no override, since overrides are keyed by the TMDB id.
-- name: ListSpotlightEntriesAdmin :many
WITH entries AS (
    SELECT e.list_key, e.media, e.position, e.tmdb_id, e.tvdb_id, e.imdb_id, e.title, e.year,
           CASE e.media WHEN 'movie' THEN 'movie' ELSE 'series' END AS kind
    FROM spotlight_entries AS e
    WHERE e.list_key = @list_key
), matched AS (
    SELECT e.*,
           CASE WHEN e.media = 'show' THEN (SELECT i.id FROM items AS i WHERE i.tvdb_id = e.tvdb_id AND i.kind = e.kind
                                            AND i.deleted_at IS NULL ORDER BY i.id LIMIT 1) END AS by_tvdb,
           (SELECT i.id FROM items AS i WHERE i.tmdb_id = e.tmdb_id AND i.kind = e.kind AND i.deleted_at IS NULL
            ORDER BY i.id LIMIT 1) AS by_tmdb,
           (SELECT i.id FROM items AS i WHERE i.imdb_id = e.imdb_id AND i.kind = e.kind AND i.deleted_at IS NULL
            ORDER BY i.id LIMIT 1) AS by_imdb
    FROM entries AS e
)
SELECT m.media, m.position, m.tmdb_id, m.tvdb_id, m.imdb_id, m.title, m.year,
       CAST(COALESCE(m.by_tvdb, m.by_tmdb, m.by_imdb, '') AS TEXT) AS item_id,
       CAST(CASE WHEN m.by_tvdb IS NOT NULL THEN 'tvdb' WHEN m.by_tmdb IS NOT NULL THEN 'tmdb'
                 WHEN m.by_imdb IS NOT NULL THEN 'imdb' ELSE '' END AS TEXT) AS match_rung,
       CAST(COALESCE(td.title, te.title, (SELECT tr.title FROM item_texts AS tr WHERE tr.item_id = i.id ORDER BY tr.rank, tr.lang LIMIT 1), i.title, '') AS TEXT) AS item_title,
       o.hidden AS override_hidden, o.pinned_rank, o.note
FROM matched AS m
LEFT JOIN items AS i ON i.id = COALESCE(m.by_tvdb, m.by_tmdb, m.by_imdb)
LEFT JOIN item_texts AS td ON td.item_id = i.id AND td.lang = 'de'
LEFT JOIN item_texts AS te ON te.item_id = i.id AND te.lang = 'en'
LEFT JOIN spotlight_overrides AS o ON o.list_key = m.list_key AND o.media = m.media AND o.tmdb_id = m.tmdb_id
ORDER BY m.media, m.position;

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

-- name: ListSpotlightTitlesToStore :many
-- The missing entries of the spotlight lists whose title outside the library is not stored yet
-- (concept 25, 05-spotlight-integration.md section 5): titles.refresh fetches them, so the
-- administration shows their posters and wishes. An entry the library holds by any of its ids is
-- not missing.
SELECT DISTINCT e.media, e.tmdb_id FROM spotlight_entries AS e
WHERE e.tmdb_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM external_titles AS t WHERE t.media = e.media AND t.tmdb_id = e.tmdb_id)
  AND NOT EXISTS (SELECT 1 FROM items AS i
                  WHERE i.kind = CASE e.media WHEN 'movie' THEN 'movie' ELSE 'series' END AND i.deleted_at IS NULL
                    AND (i.tmdb_id = e.tmdb_id OR (e.media = 'show' AND i.tvdb_id = e.tvdb_id) OR i.imdb_id = e.imdb_id))
ORDER BY e.media, e.tmdb_id;
