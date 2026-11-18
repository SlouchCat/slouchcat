-- Titles outside the library (concept 25, architecture sections 3 and 5), package visibility. A
-- title is in the library when a title of its kind that is not deleted carries one of its ids,
-- visible to the profile or not: a library title rated too high for a profile is not missing, it
-- does not exist for that profile (404). Matching follows the library: a film by TMDB id, then
-- IMDb id; a show by TVDB id, then TMDB id, then IMDb id. The ids a caller does not know come from
-- external_titles. Each branch uses its index on items (items_tvdb, items_tmdb, items_imdb).

-- name: TitleByExternalIDs :one
-- The library title of the ids, without a profile (package wishes asks through visibility): the
-- best rung wins, an available title before a missing one, then the smallest id.
WITH ref AS (
    SELECT CAST(@media AS TEXT) AS media,
           CASE CAST(@media AS TEXT) WHEN 'movie' THEN 'movie' ELSE 'series' END AS kind,
           CAST(@tmdb_id AS INTEGER) AS tmdb_id,
           COALESCE(sqlc.narg('tvdb_id'), (SELECT x.tvdb_id FROM external_titles AS x
                                          WHERE x.media = CAST(@media AS TEXT) AND x.tmdb_id = CAST(@tmdb_id AS INTEGER))) AS tvdb_id,
           COALESCE(sqlc.narg('imdb_id'), (SELECT x.imdb_id FROM external_titles AS x
                                          WHERE x.media = CAST(@media AS TEXT) AND x.tmdb_id = CAST(@tmdb_id AS INTEGER))) AS imdb_id
), hits AS (
    SELECT 1 AS rung, i.id, i.available FROM ref AS f
    JOIN items AS i ON i.tvdb_id = f.tvdb_id AND i.kind = f.kind AND i.deleted_at IS NULL
    WHERE f.media = 'show'
    UNION ALL
    SELECT 2 AS rung, i.id, i.available FROM ref AS f
    JOIN items AS i ON i.tmdb_id = f.tmdb_id AND i.kind = f.kind AND i.deleted_at IS NULL
    UNION ALL
    SELECT 3 AS rung, i.id, i.available FROM ref AS f
    JOIN items AS i ON i.imdb_id = f.imdb_id AND i.kind = f.kind AND i.deleted_at IS NULL
)
SELECT h.id FROM hits AS h ORDER BY h.rung, h.available DESC, h.id LIMIT 1;

-- name: MatchExternalTitles :many
-- For a profile and the titles in the JSON array refs ([{"media","tmdb","tvdb","imdb"}], position
-- = index): whether the title is in the library at all, the library title the profile sees (the
-- best rung among the visible titles only, like the spotlight), whether external_titles holds the
-- title and its age (NULL = unknown), and the age ceiling of the profile: min(profile, account).
-- A profile that does not exist yields no row.
WITH refs AS (
    SELECT CAST(r."key" AS INTEGER) AS pos,
           CAST(json_extract(r.value, '$.media') AS TEXT) AS media,
           CASE json_extract(r.value, '$.media') WHEN 'movie' THEN 'movie' ELSE 'series' END AS kind,
           CAST(json_extract(r.value, '$.tmdb') AS INTEGER) AS tmdb_id,
           json_extract(r.value, '$.tvdb') AS tvdb_id,
           json_extract(r.value, '$.imdb') AS imdb_id
    FROM json_each(@refs) AS r
), ids AS (
    SELECT f.pos, f.media, f.kind, f.tmdb_id,
           COALESCE(f.tvdb_id, x.tvdb_id) AS tvdb_id, COALESCE(f.imdb_id, x.imdb_id) AS imdb_id,
           x.tmdb_id IS NOT NULL AS has_row, x.min_age
    FROM refs AS f
    LEFT JOIN external_titles AS x ON x.media = f.media AND x.tmdb_id = f.tmdb_id
), hits AS (
    SELECT d.pos, 1 AS rung, i.id FROM ids AS d
    JOIN items AS i ON i.tvdb_id = d.tvdb_id AND i.kind = d.kind AND i.deleted_at IS NULL
    WHERE d.media = 'show'
    UNION ALL
    SELECT d.pos, 2 AS rung, i.id FROM ids AS d
    JOIN items AS i ON i.tmdb_id = d.tmdb_id AND i.kind = d.kind AND i.deleted_at IS NULL
    UNION ALL
    SELECT d.pos, 3 AS rung, i.id FROM ids AS d
    JOIN items AS i ON i.imdb_id = d.imdb_id AND i.kind = d.kind AND i.deleted_at IS NULL
), seen AS (
    SELECT h.pos, h.rung, h.id FROM hits AS h
    JOIN visible_items AS v ON v.item_id = h.id AND v.profile_id = @profile_id
), ceiling AS (
    SELECT min(p.max_rating, a.max_rating) AS max_age
    FROM profiles AS p JOIN accounts AS a ON a.id = p.account_id
    WHERE p.id = @profile_id
)
SELECT d.pos,
       CAST(EXISTS (SELECT 1 FROM hits AS h WHERE h.pos = d.pos) AS INTEGER) AS in_library,
       CAST(COALESCE((SELECT s.id FROM seen AS s WHERE s.pos = d.pos ORDER BY s.rung, s.id LIMIT 1), '') AS TEXT) AS item_id,
       CAST(d.has_row AS INTEGER) AS has_row,
       d.min_age,
       CAST(c.max_age AS INTEGER) AS max_age
FROM ids AS d
CROSS JOIN ceiling AS c
ORDER BY d.pos;
