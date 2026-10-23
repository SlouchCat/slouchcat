-- Usage statistics of the administration (M8, A-8, design 3.6; package admin). Unfiltered: the
-- administration sees every profile. The period is [from_ms, to_ms) in Unix milliseconds. Watch time
-- is the played time of the progress events whose server time lies in the period (offline events
-- count with their upload time); a play is one playback session, an event without one counts on
-- its own.
--
-- Since M10 events older than 90 days are condensed into progress_days (migration 00019), so every
-- watch-time query reads two sources and adds them up: the raw events of progress_events and the
-- condensed day rows of progress_days. Each source is aggregated on its own first (the distinct
-- playback sessions of the raw events cannot be counted after the union), then the two results are
-- summed per title, profile or device. A condensed day counts when it overlaps the period: its day
-- number lies between from_ms / 86400000 and (to_ms - 1) / 86400000. Only the older end of a period
-- can cut a day in two, because nothing newer than 90 days is ever condensed.

-- name: StatsTopTitles :many
-- The most watched titles by played time, with the plays and the title (German text, else English,
-- else the parsed title). The events are aggregated first, so only the listed titles are joined;
-- the plays are the distinct playback sessions plus the events without one.
WITH watched AS (
    SELECT item_id, SUM(watched_ms) AS watched_ms, SUM(plays) AS plays
    FROM (
        SELECT item_id, SUM(played_ms) AS watched_ms,
               count(DISTINCT playback_session_id) + SUM(playback_session_id IS NULL) AS plays
        FROM progress_events
        WHERE server_time >= @from_ms AND server_time < @to_ms
        GROUP BY item_id
        UNION ALL
        SELECT item_id, SUM(played_ms) AS watched_ms, SUM(plays) AS plays
        FROM progress_days
        WHERE day >= @from_ms / 86400000 AND day <= (@to_ms - 1) / 86400000
        GROUP BY item_id
    )
    GROUP BY item_id
    HAVING SUM(watched_ms) > 0
    ORDER BY watched_ms DESC, item_id
    LIMIT @limit_count
)
SELECT w.item_id, i.kind, COALESCE(td.title, te.title, i.title) AS title, i.year,
       CAST(w.watched_ms AS INTEGER) AS watched_ms, CAST(w.plays AS INTEGER) AS plays
FROM watched AS w
JOIN items AS i ON i.id = w.item_id
LEFT JOIN item_texts AS td ON td.item_id = i.id AND td.lang = 'de'
LEFT JOIN item_texts AS te ON te.item_id = i.id AND te.lang = 'en'
ORDER BY w.watched_ms DESC, w.item_id;

-- name: StatsWatchTimeByProfile :many
-- Played time and plays per profile with its account, most first.
WITH watched AS (
    SELECT profile_id, SUM(watched_ms) AS watched_ms, SUM(plays) AS plays
    FROM (
        SELECT profile_id, SUM(played_ms) AS watched_ms,
               count(DISTINCT playback_session_id) + SUM(playback_session_id IS NULL) AS plays
        FROM progress_events
        WHERE server_time >= @from_ms AND server_time < @to_ms
        GROUP BY profile_id
        UNION ALL
        SELECT profile_id, SUM(played_ms) AS watched_ms, SUM(plays) AS plays
        FROM progress_days
        WHERE day >= @from_ms / 86400000 AND day <= (@to_ms - 1) / 86400000
        GROUP BY profile_id
    )
    GROUP BY profile_id
)
SELECT p.id AS profile_id, p.name AS profile_name, a.email AS account_email,
       CAST(w.watched_ms AS INTEGER) AS watched_ms, CAST(w.plays AS INTEGER) AS plays
FROM watched AS w
JOIN profiles AS p ON p.id = w.profile_id
JOIN accounts AS a ON a.id = p.account_id
ORDER BY w.watched_ms DESC, p.id;

-- name: StatsPlaybackModes :one
-- Playback sessions started in the period by mode, and the transcodes encoded on the GPU. Package
-- playback keeps ended rows for 400 days, so every period of a year is complete. Playback sessions
-- are never condensed, so this query reads one table as before.
SELECT CAST(COALESCE(SUM(mode = 'direct'), 0) AS INTEGER)                      AS direct,
       CAST(COALESCE(SUM(mode = 'remux'), 0) AS INTEGER)                       AS remux,
       CAST(COALESCE(SUM(mode = 'transcode'), 0) AS INTEGER)                   AS transcode,
       CAST(COALESCE(SUM(mode = 'transcode' AND hardware = 1), 0) AS INTEGER) AS transcode_hardware
FROM playback_sessions
WHERE started_at >= @from_ms AND started_at < @to_ms;

-- name: StatsDevices :many
-- Played time and distinct devices per device type; events without a known device or type count as
-- unknown. The events are aggregated per device first; the condensed days store "no device" as the
-- empty id, which NULLIF maps back so both sources land in the same unknown bucket.
WITH watched AS (
    SELECT device_id, SUM(watched_ms) AS watched_ms
    FROM (
        SELECT device_id, SUM(played_ms) AS watched_ms
        FROM progress_events
        WHERE server_time >= @from_ms AND server_time < @to_ms
        GROUP BY device_id
        UNION ALL
        SELECT NULLIF(device_id, '') AS device_id, SUM(played_ms) AS watched_ms
        FROM progress_days
        WHERE day >= @from_ms / 86400000 AND day <= (@to_ms - 1) / 86400000
        GROUP BY NULLIF(device_id, '')
    )
    GROUP BY device_id
)
SELECT CAST(COALESCE(d.type, 'unknown') AS TEXT) AS device_type,
       count(w.device_id) AS devices,
       CAST(SUM(w.watched_ms) AS INTEGER) AS watched_ms
FROM watched AS w
LEFT JOIN devices AS d ON d.id = w.device_id
GROUP BY COALESCE(d.type, 'unknown')
ORDER BY watched_ms DESC, device_type;
