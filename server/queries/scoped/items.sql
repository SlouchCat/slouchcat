-- Scoped title queries (package store/scopedq), used only by the visibility package. Every query
-- joins visible_items for the given profile; the test in internal/store enforces it for this folder.
-- Times are Unix milliseconds.

-- name: GetVisibleItem :one
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.id = @item_id;

-- The row recent (recently added). It walks the partial index items_top_added (migration 00037)
-- in its order; the planner takes it only while the kind term is written exactly as in the index.
-- name: ListRecentlyAdded :many
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.kind IN ('movie', 'series', 'audiobook', 'podcast')
ORDER BY i.added_at DESC, i.id DESC
LIMIT @limit_count;

-- The row "Weiterschauen" (E-1, concept 15 P-1), newest activity first: one entry per series and
-- one per started, not completed film, audiobook or podcast episode.
--
-- watched holds the activity of the profile: progress rows of visible titles (extras never) that
-- are started or completed; a row at position 0 that is not completed is no activity. An episode of
-- a season of a series carries the series id, its season number and its last episode number (the
-- end of a file with several episodes), the same values as Reader.episodeChain passes to
-- GetNextVisibleEpisode. Per series only the newest row counts, completed or not, so an older
-- half-watched episode is ignored once another one was finished after it. Every other title is its
-- own group. newest keeps that row per group: max() over updated_at, then item_id (zero-padded, so
-- the text compares like the pair) is SQLite's single max() aggregate, whose bare columns come from
-- the row with the maximum (grouped). A series the profile may not see drops its whole group
-- (newest), the same as leaving out each of its episodes, but checked once per series. The newest
-- row then picks the entry:
--   - not completed (and started): the title itself;
--   - a completed episode at most @since old: the next episode by the rule of
--     GetNextVisibleEpisode (same season with a higher number, else a later season; specials only
--     after a special; visible only; none when the completed episode lacks its season or episode
--     number, like episodeChain). Edge decision of concept 15 step 2: an episode the profile
--     already completed is skipped (a rewatch of an old episode), so the entry is the first next
--     visible episode that is not completed; without one the series is absent;
--   - anything else (a completed film, a completed episode older than @since): no entry.
-- Cost (docs/perf/feinschliff.md): one pass over the progress rows of the profile (primary key
-- profile_id, item_id) with a join on items by id and a grouping; next_episodes starts from the
-- series whose newest row is a completed episode inside the window and walks items_parent (seasons
-- of the series) and items_episode (episodes of a season by number). The CROSS JOINs fix that
-- order: with plain JOINs SQLite scanned every title and filtered by series afterwards.
-- name: ListContinueWatching :many
WITH watched AS (
    SELECT p.item_id, p.updated_at, p.completed, p.updated_at >= @since AS in_window,
           se.parent_id AS series_id,
           coalesce(e.season_number, se.season_number) AS season_number,
           coalesce(e.episode_end, e.episode_number) AS last_episode
    FROM progress AS p
    JOIN items AS e ON e.id = p.item_id
    JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
    LEFT JOIN items AS se ON e.kind = 'episode' AND se.id = e.parent_id AND se.kind = 'season'
    WHERE p.profile_id = @profile_id AND e.kind <> 'extra' AND (p.completed = 1 OR p.position_ms > 0)
),
grouped AS (
    SELECT w.item_id, w.updated_at, w.completed, w.in_window, w.series_id, w.season_number, w.last_episode,
           max(printf('%020d', w.updated_at) || w.item_id) AS newest_key
    FROM watched AS w
    GROUP BY coalesce(w.series_id, w.item_id)
),
newest AS (
    SELECT g.* FROM grouped AS g
    WHERE g.series_id IS NULL OR EXISTS (
        SELECT 1 FROM visible_items AS vs WHERE vs.item_id = g.series_id AND vs.profile_id = @profile_id)
),
next_episodes AS (
    SELECT n.series_id, x.id AS next_id,
           row_number() OVER (PARTITION BY n.series_id
                              ORDER BY coalesce(x.season_number, xs.season_number), x.episode_number, x.id) AS rn
    FROM newest AS n
    CROSS JOIN items AS xs ON xs.parent_id = n.series_id AND xs.kind = 'season'
    CROSS JOIN items AS x ON x.parent_id = xs.id AND x.kind = 'episode'
    JOIN visible_items AS vx ON vx.item_id = x.id AND vx.profile_id = @profile_id
    LEFT JOIN progress AS xp ON xp.profile_id = @profile_id AND xp.item_id = x.id
    WHERE n.completed = 1 AND n.in_window = 1 AND x.id <> n.item_id
      AND n.season_number IS NOT NULL AND n.last_episode IS NOT NULL
      AND ((coalesce(x.season_number, xs.season_number) = n.season_number AND x.episode_number > n.last_episode)
           OR coalesce(x.season_number, xs.season_number) > n.season_number)
      AND (coalesce(x.season_number, xs.season_number) <> 0 OR n.season_number = 0)
      AND coalesce(xp.completed, 0) = 0
)
SELECT i.* FROM newest AS n
LEFT JOIN next_episodes AS nx ON nx.series_id = n.series_id AND nx.rn = 1
JOIN items AS i ON i.id = CASE WHEN n.completed = 0 THEN n.item_id ELSE nx.next_id END
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
ORDER BY n.updated_at DESC, i.id DESC
LIMIT @limit_count;

-- Cat's recommendations (E-1, E-2) for the profile: global rows (profile_id NULL) and rows of the
-- profile, inside their optional time window, with the main user's note. An item listed twice is
-- deduplicated by the caller (the row of the profile wins over the global one), so the query has
-- no limit: the caller cuts after the deduplication (the table is small, curated by hand).
-- name: ListTopPicks :many
SELECT sqlc.embed(i), t.note, t.profile_id AS pick_profile_id
FROM top_picks AS t
JOIN items AS i ON i.id = t.item_id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE (t.profile_id IS NULL OR t.profile_id = @profile_id)
  AND (t.starts_at IS NULL OR t.starts_at <= @now)
  AND (t.ends_at IS NULL OR t.ends_at > @now)
ORDER BY t.position, t.id;

-- name: ListItemTexts :many
SELECT t.* FROM item_texts AS t
JOIN visible_items AS v ON v.item_id = t.item_id AND v.profile_id = @profile_id
WHERE t.item_id IN (sqlc.slice('item_ids'));

-- Delivered image rows (all sizes and formats, without the kept TMDB originals).
-- name: ListItemImages :many
SELECT im.* FROM images AS im
JOIN visible_items AS v ON v.item_id = im.item_id AND v.profile_id = @profile_id
WHERE im.item_id IN (sqlc.slice('item_ids')) AND im.size <> 'original'
ORDER BY im.item_id, im.kind, im.size, im.format;

-- name: ListItemProgress :many
SELECT p.* FROM progress AS p
JOIN visible_items AS v ON v.item_id = p.item_id AND v.profile_id = p.profile_id
WHERE p.profile_id = @profile_id AND p.item_id IN (sqlc.slice('item_ids'));

-- Media files (versions, B-5) of a visible title with the path of their library root: available
-- files first, then the largest height, then the oldest, so the first row is the primary file.
-- name: ListMediaFilesForItem :many
SELECT m.id, m.item_id, m.rel_path, m.size, m.mtime, m.container, m.video_codec, m.audio_codec,
       m.width, m.height, m.duration_ms, m.version_label, m.missing_since, m.created_at, m.probed_at,
       r.path AS root_path
FROM media_files AS m
JOIN visible_items AS v ON v.item_id = m.item_id AND v.profile_id = @profile_id
JOIN library_roots AS r ON r.id = m.root_id
WHERE m.item_id = @item_id
ORDER BY m.missing_since IS NOT NULL, COALESCE(m.height, 0) DESC, m.created_at, m.id;

-- One delivered image in the wanted format with JPEG as the fallback: format 'webp' returns the
-- WebP row when it exists, else the JPEG row; format 'jpeg' only the JPEG row.
-- name: GetItemImage :one
-- The wanted format (webp) when stored, else the fallback of the kind (jpeg; png for a logo).
SELECT im.* FROM images AS im
JOIN visible_items AS v ON v.item_id = im.item_id AND v.profile_id = @profile_id
WHERE im.item_id = @item_id AND im.kind = @kind AND im.size = @size AND im.size <> 'original'
  AND im.format IN (@format, @fallback)
ORDER BY im.format = 'webp' DESC
LIMIT 1;

-- Series tree, extras, cast and trailers (M3, design 4.1).

-- Visible titles by id, in no particular order (parent chain of seasons and episodes).
-- name: ListVisibleItems :many
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.id IN (sqlc.slice('item_ids'));

-- Number of visible content children of visible series and seasons: seasons of a series, episodes
-- of a season (extras never count). One query with a correlated subquery over visible_items.
-- watched_count counts the visible episodes of a season the profile has completed (concept 17
-- section 2.3, like the "done" of the library's episode statistics); 0 for a series.
-- name: ListVisibleChildCounts :many
SELECT i.id,
       (SELECT count(*) FROM items AS c
        JOIN visible_items AS vc ON vc.item_id = c.id AND vc.profile_id = @profile_id
        WHERE c.parent_id = i.id AND c.kind IN ('season', 'episode')) AS child_count,
       (SELECT count(*) FROM items AS c
        JOIN visible_items AS vc ON vc.item_id = c.id AND vc.profile_id = @profile_id
        JOIN progress AS pc ON pc.profile_id = @profile_id AND pc.item_id = c.id AND pc.completed = 1
        WHERE c.parent_id = i.id AND c.kind = 'episode') AS watched_count
FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.id IN (sqlc.slice('item_ids')) AND i.kind IN ('series', 'season');

-- The series of visible seasons (parent) and episodes (grandparent), when the series is visible.
-- name: ListVisibleSeriesOf :many
SELECT i.id AS item_id, i.kind AS item_kind, s.id AS series_id, s.title AS series_title
FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
LEFT JOIN items AS p ON p.id = i.parent_id
JOIN items AS s ON s.kind = 'series'
               AND ((i.kind = 'season' AND s.id = i.parent_id) OR (i.kind = 'episode' AND s.id = p.parent_id))
JOIN visible_items AS vs ON vs.item_id = s.id AND vs.profile_id = @profile_id
WHERE i.id IN (sqlc.slice('item_ids')) AND i.kind IN ('season', 'episode');

-- Visible seasons of a series by number, specials (season 0) last.
-- name: ListVisibleSeasons :many
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.parent_id = @series_id AND i.kind = 'season'
ORDER BY COALESCE(i.season_number, 0) = 0, i.season_number, i.sort_title, i.id;

-- Visible episodes of a season by number.
-- name: ListVisibleEpisodes :many
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.parent_id = @season_id AND i.kind = 'episode'
ORDER BY i.episode_number IS NULL, i.episode_number, i.sort_title, i.id;

-- Visible extras of a film, series or season by kind and title.
-- name: ListVisibleExtras :many
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.parent_id = @parent_id AND i.kind = 'extra'
ORDER BY i.extra_kind, i.sort_title, i.id;

-- The next visible episode of a series after (season_number, last_episode): a higher number in the
-- same season, else the first episode of a later season. Specials (season 0) only follow a special.
-- name: GetNextVisibleEpisode :one
SELECT i.* FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
JOIN items AS s ON s.id = i.parent_id AND s.kind = 'season'
WHERE s.parent_id = @series_id AND i.kind = 'episode' AND i.id <> @item_id
  AND ((COALESCE(i.season_number, s.season_number) = @season_number AND i.episode_number > @last_episode)
       OR COALESCE(i.season_number, s.season_number) > @season_number)
  AND (COALESCE(i.season_number, s.season_number) <> 0 OR @season_number = 0)
ORDER BY COALESCE(i.season_number, s.season_number), i.episode_number, i.id
LIMIT 1;

-- Credits of a visible title with the people's names, in the provider's order.
-- name: ListVisibleCredits :many
SELECT c.person_id, p.name, c.role, c.character, c.ord
FROM credits AS c
JOIN visible_items AS v ON v.item_id = c.item_id AND v.profile_id = @profile_id
JOIN people AS p ON p.id = c.person_id
WHERE c.item_id = @item_id
ORDER BY c.ord, c.role, p.name, c.person_id;

-- name: CountVisibleTrailers :one
SELECT count(*) FROM trailers AS t
JOIN visible_items AS v ON v.item_id = t.item_id AND v.profile_id = @profile_id
WHERE t.item_id = @item_id;

-- Trailers of a visible title by language, official first (the reader puts the profile language
-- and English in front).
-- name: ListVisibleTrailers :many
SELECT t.* FROM trailers AS t
JOIN visible_items AS v ON v.item_id = t.item_id AND v.profile_id = @profile_id
WHERE t.item_id = @item_id
ORDER BY t.lang, t.official DESC, t.id;

-- Credits of a person on visible titles only (a person without one does not exist for the profile).
-- name: ListVisiblePersonCredits :many
SELECT sqlc.embed(i), p.name AS person_name, c.role, c.character
FROM credits AS c
JOIN people AS p ON p.id = c.person_id
JOIN items AS i ON i.id = c.item_id
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE c.person_id = @person_id
ORDER BY i.year IS NULL, i.year DESC, i.sort_title, i.id, c.role;

-- Parents of visible titles that the profile may see too. A summary names its parent only then:
-- an extra of a series without available episodes (an extra alone keeps the series unavailable)
-- must not reveal the series id.
-- name: ListVisibleParents :many
SELECT i.id, i.parent_id
FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
JOIN visible_items AS vp ON vp.item_id = i.parent_id AND vp.profile_id = @profile_id
WHERE i.id IN (sqlc.slice('item_ids'));

-- Recently released (M11, S-5, S-6, ADR 0011): films and audiobooks by their own release_date,
-- series by the release_date of their newest VISIBLE episode, within [since, today] (twelve months,
-- fixed), newest first. The window sits in the episode join, so the partial index items_released
-- (release_date DESC WHERE release_date IS NOT NULL AND deleted_at IS NULL) carries both branches
-- and the aggregation runs over the episodes of one year only. A series whose newest episode the
-- profile may not see is dated by the newest one it may see; a series without a visible dated
-- episode is absent. A title dated in the future is not released yet and is left out.
-- name: ListRecentlyReleased :many
SELECT sqlc.embed(i), r.released FROM (
    SELECT t.id AS item_id, t.release_date AS released
    FROM items AS t
    JOIN visible_items AS v ON v.item_id = t.id AND v.profile_id = @profile_id
    WHERE t.kind IN ('movie', 'audiobook') AND t.deleted_at IS NULL
      AND t.release_date IS NOT NULL AND t.release_date >= @since AND t.release_date <= @today
    UNION ALL
    SELECT s.id, max(e.release_date)
    FROM items AS s
    JOIN visible_items AS vs ON vs.item_id = s.id AND vs.profile_id = @profile_id
    JOIN items AS se ON se.parent_id = s.id AND se.kind = 'season' AND se.deleted_at IS NULL
    JOIN items AS e ON e.parent_id = se.id AND e.kind = 'episode' AND e.deleted_at IS NULL
                   AND e.release_date IS NOT NULL AND e.release_date >= @since AND e.release_date <= @today
    JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
    WHERE s.kind = 'series' AND s.deleted_at IS NULL
    GROUP BY s.id
) AS r
JOIN items AS i ON i.id = r.item_id
ORDER BY r.released DESC, i.id DESC
LIMIT @limit_count;

-- Which of the given visible titles have a stored trailer (ItemRef.hasTrailer, M11 D3: filled only
-- for the rows spotlight and picks of the start page, so one query over at most 40 titles).
-- name: ListVisibleTrailerItems :many
SELECT DISTINCT t.item_id FROM trailers AS t
JOIN visible_items AS v ON v.item_id = t.item_id AND v.profile_id = @profile_id
WHERE t.item_id IN (sqlc.slice('item_ids'));
