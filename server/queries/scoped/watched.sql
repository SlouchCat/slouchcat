-- The check mark "gesehen" (concept 20, P-5 and P-14, package store/scopedq, read by the visibility
-- package only). Every query joins visible_items: a title the profile may not see is neither
-- marked nor listed, and an episode it may not see is never counted.

-- The titles setItemWatched writes for a visible title (section 6.4), in episode order: the title
-- itself for a film, episode, extra, audiobook or podcast episode, the visible episodes of a
-- season, the visible episodes of every season of a series (specials, season 0, first) or the
-- visible episodes of a podcast, oldest first. duration_ms is the duration a missing progress row
-- starts with: for an audiobook the book-wide duration, for every other kind the probed duration
-- of the primary file (available files first, then the largest height, the oldest, as
-- ListMediaFilesForItem orders them), then the duration from the feed and the runtime of the
-- metadata, 0 when none is known.
-- name: ListVisibleWatchTargets :many
WITH title AS (
    SELECT i.id, i.kind FROM items AS i
    JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
    WHERE i.id = @item_id
),
wanted AS (
    SELECT t.id FROM title AS t WHERE t.kind NOT IN ('series', 'season', 'podcast')
    UNION ALL
    SELECT e.id FROM title AS t JOIN items AS e ON e.parent_id = t.id AND e.kind IN ('episode', 'podcast_episode')
    WHERE t.kind IN ('season', 'podcast')
    UNION ALL
    SELECT e.id FROM title AS t
    JOIN items AS s ON s.parent_id = t.id AND s.kind = 'season'
    JOIN items AS e ON e.parent_id = s.id AND e.kind = 'episode'
    WHERE t.kind = 'series'
)
SELECT e.id,
       CAST(CASE WHEN e.kind = 'audiobook' THEN COALESCE(e.duration_ms, f.duration_ms, e.runtime_minutes * 60000, 0)
                 ELSE COALESCE(f.duration_ms, e.duration_ms, e.runtime_minutes * 60000, 0) END AS INTEGER) AS duration_ms
FROM wanted AS x
JOIN items AS e ON e.id = x.id
JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
LEFT JOIN media_files AS f ON f.id = (
    SELECT m.id FROM media_files AS m
    WHERE m.item_id = e.id AND m.duration_ms > 0
    ORDER BY (m.missing_since IS NULL AND m.probe_error IS NULL) DESC, m.height DESC, m.created_at, m.id
    LIMIT 1)
ORDER BY CASE WHEN e.kind = 'podcast_episode' THEN COALESCE(e.published_at, e.added_at) ELSE 0 END,
         COALESCE(e.season_number, 0), COALESCE(e.episode_number, 0), e.id;

-- The page "Gesehen" (listWatched, section 15.4): every film and audiobook with watched_at, and
-- every series and podcast with at least one visible watched episode (of the title's library, the
-- rule of ItemRef.watched), once each. Radio, extras, seasons and episodes are no entries.
-- last_watched_at is the title's watched_at or the newest one of its visible episodes,
-- watched_count the number of its visible watched episodes (0 for a single title). Both branches
-- walk the progress rows of the profile with watched_at (the partial index progress_watched)
-- before any visibility check. kind narrows to one content ('' = all), by_title sorts by the
-- sort key of the text chain (metadata language, interface language, file name) instead of newest
-- first, ties by the other order and the id. total is the size of the whole list.
-- name: ListWatched :many
WITH watched AS (
    SELECT i.id, i.kind, i.library_id, i.parent_id, p.watched_at
    FROM progress AS p INDEXED BY progress_watched
    CROSS JOIN items AS i ON i.id = p.item_id
    WHERE p.profile_id = @profile_id AND p.watched_at IS NOT NULL
      AND i.kind IN ('movie', 'audiobook', 'episode', 'podcast_episode')
),
singles AS (
    SELECT w.id, w.watched_at AS last_watched_at, 0 AS watched_count
    FROM watched AS w
    CROSS JOIN visible_items AS v ON v.item_id = w.id AND v.profile_id = @profile_id
    WHERE w.kind IN ('movie', 'audiobook')
),
episodes AS (
    SELECT COALESCE(se.parent_id, w.parent_id) AS top_id, w.library_id, w.watched_at
    FROM watched AS w
    CROSS JOIN visible_items AS ve ON ve.item_id = w.id AND ve.profile_id = @profile_id
    LEFT JOIN items AS se ON se.id = w.parent_id AND se.kind = 'season'
    WHERE w.kind IN ('episode', 'podcast_episode')
),
tops AS (
    SELECT t.id, max(x.watched_at) AS last_watched_at, count(*) AS watched_count
    FROM episodes AS x
    CROSS JOIN items AS t ON t.id = x.top_id AND t.kind IN ('series', 'podcast') AND t.library_id = x.library_id
    CROSS JOIN visible_items AS vt ON vt.item_id = t.id AND vt.profile_id = @profile_id
    GROUP BY t.id
),
entries AS (
    SELECT id, last_watched_at, watched_count FROM singles
    UNION ALL
    SELECT id, last_watched_at, watched_count FROM tops
),
keyed AS (
    SELECT e.id, i.kind, e.last_watched_at, e.watched_count,
           CASE WHEN CAST(@by_title AS INTEGER) = 1 THEN 0 ELSE -e.last_watched_at END AS first_key,
           COALESCE(NULLIF(tm.sort_key, ''), NULLIF(tu.sort_key, ''), i.sort_title, '') AS sort_key,
           CASE WHEN CAST(@by_title AS INTEGER) = 1 THEN -e.last_watched_at ELSE 0 END AS last_key
    FROM entries AS e
    CROSS JOIN items AS i ON i.id = e.id
    LEFT JOIN item_texts AS tm ON tm.item_id = i.id AND tm.lang = @meta_lang
    LEFT JOIN item_texts AS tu ON tu.item_id = i.id AND tu.lang = @ui_lang
    WHERE i.kind = @kind OR @kind = ''
)
SELECT k.id, k.kind, CAST(k.last_watched_at AS INTEGER) AS last_watched_at, CAST(k.watched_count AS INTEGER) AS watched_count,
       CAST(count(*) OVER () AS INTEGER) AS total
FROM keyed AS k
ORDER BY k.first_key, k.sort_key, k.last_key, k.id
LIMIT @limit_count OFFSET @offset_count;

-- The size of ListWatched, for a page beyond its end (that page has no row to carry total).
-- name: CountWatched :one
WITH watched AS (
    SELECT i.id, i.kind, i.library_id, i.parent_id
    FROM progress AS p INDEXED BY progress_watched
    CROSS JOIN items AS i ON i.id = p.item_id
    WHERE p.profile_id = @profile_id AND p.watched_at IS NOT NULL
      AND i.kind IN ('movie', 'audiobook', 'episode', 'podcast_episode')
),
entries AS (
    SELECT w.id FROM watched AS w
    CROSS JOIN visible_items AS v ON v.item_id = w.id AND v.profile_id = @profile_id
    WHERE w.kind IN ('movie', 'audiobook')
    UNION
    SELECT t.id FROM watched AS w
    CROSS JOIN visible_items AS ve ON ve.item_id = w.id AND ve.profile_id = @profile_id
    LEFT JOIN items AS se ON se.id = w.parent_id AND se.kind = 'season'
    CROSS JOIN items AS t ON t.id = COALESCE(se.parent_id, w.parent_id) AND t.kind IN ('series', 'podcast') AND t.library_id = w.library_id
    CROSS JOIN visible_items AS vt ON vt.item_id = t.id AND vt.profile_id = @profile_id
    WHERE w.kind IN ('episode', 'podcast_episode')
)
SELECT count(*) FROM entries AS e
JOIN items AS i ON i.id = e.id
WHERE i.kind = @kind OR @kind = '';

-- The number of visible episodes of the given series and podcasts, over the episodes of the rule
-- of ItemRef.watched (an episode or podcast episode of the title's library in one of its seasons,
-- specials included, or directly below it): episodeCount of listWatched, the "96" of "19 of 96".
-- name: ListVisibleEpisodeCounts :many
SELECT i.id,
       CAST((SELECT count(*) FROM items AS se
             CROSS JOIN items AS e ON e.parent_id = se.id AND e.kind IN ('episode', 'podcast_episode')
             CROSS JOIN visible_items AS ve ON ve.item_id = e.id AND ve.profile_id = @profile_id
             WHERE (se.id = i.id OR (se.parent_id = i.id AND se.kind = 'season'))
               AND +e.library_id = i.library_id) AS INTEGER) AS episode_count
FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
WHERE i.id IN (sqlc.slice('item_ids')) AND i.kind IN ('series', 'podcast');
