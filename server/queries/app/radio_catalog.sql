-- Webradio catalog and settings (package store/appq, concept 16 sections 5 and 7.3; owner: radio,
-- lane B). The station rows name no title (rule 2). Lists, facets and search read the in-memory
-- index of package radio, which these queries fill at startup, after a sync and per changed
-- station. Times are Unix milliseconds.

-- name: GetRadioConfig :one
SELECT * FROM radio_config WHERE id = 1;

-- name: UpdateRadioConfig :exec
-- countries is a JSON array of ISO 3166-1 codes, NULL = every country.
UPDATE radio_config
SET buffer_minutes = @buffer_minutes, budget_mb = @budget_mb, max_relays = @max_relays,
    grace_seconds = @grace_seconds, hide_broken = @hide_broken, countries = sqlc.narg('countries'),
    updated_at = @now
WHERE id = 1;

-- name: UpdateRadioSyncState :exec
-- The outcome of a catalog sync; a failed sync keeps the counts and raises sync_failures.
UPDATE radio_config
SET last_sync_at = @last_sync_at, last_sync_error = sqlc.narg('last_sync_error'),
    sync_failures = @sync_failures, entries_total = @entries_total,
    entries_reachable = @entries_reachable, stations_total = @stations_total,
    last_added = @last_added, last_removed = @last_removed, updated_at = @now
WHERE id = 1;

-- name: ListRadioStationsForIndex :many
-- Every station with the codec, bitrate and HLS flag of its best stream (the first row of
-- ListRadioStreamsRanked, same order), for the in-memory index.
WITH ranked AS (
    SELECT st.station_id, st.codec, st.bitrate, st.hls,
           row_number() OVER (
               PARTITION BY st.station_id
               ORDER BY (st.preferred = 1 AND EXISTS (
                            SELECT 1 FROM radio_station_locks AS l
                            WHERE l.station_id = st.station_id AND l.field = 'stream')) DESC,
                        st.lastcheckok DESC, st.bitrate <= 320 DESC, st.bitrate DESC,
                        lower(st.codec) = 'aac' DESC, st.id
           ) AS rn
    FROM radio_streams AS st
)
SELECT s.id, s.source, s.name, s.sort_key, s.search_text, s.country, s.region, s.languages,
       s.homepage, s.votes, s.clicks, s.quality, s.reachable, s.failures, s.broken_at, s.hidden,
       s.recommended, s.shows_titles, s.missing_since, s.logo_state, s.logo_hash, s.created_at,
       s.updated_at, r.codec AS best_codec, r.bitrate AS best_bitrate,
       CAST(COALESCE(r.hls, 0) AS INTEGER) AS best_hls
FROM radio_stations AS s
LEFT JOIN ranked AS r ON r.station_id = s.id AND r.rn = 1
WHERE s.id = sqlc.narg('station_id') OR sqlc.narg('station_id') IS NULL
ORDER BY s.id;

-- name: ListRadioStationFacets :many
-- Facets of every station (station_id NULL) or of one station (the index reload of one row).
SELECT station_id, facet, value, manual FROM radio_station_facets
WHERE station_id = sqlc.narg('station_id') OR sqlc.narg('station_id') IS NULL
ORDER BY station_id, facet, value;

-- name: ListRadioStationTags :many
-- Raw tags of every station (station_id NULL) or of one station.
SELECT station_id, tag FROM radio_station_tags
WHERE station_id = sqlc.narg('station_id') OR sqlc.narg('station_id') IS NULL
ORDER BY station_id, tag;

-- name: GetRadioStation :one
SELECT * FROM radio_stations WHERE id = @id;

-- name: ListRadioStationLocks :many
SELECT field FROM radio_station_locks WHERE station_id = @station_id ORDER BY field;

-- name: ListRadioStreamsRanked :many
-- The streams of a station, best first (concept 16 section 5.3): the admin's preferred stream
-- while the field 'stream' is locked, then reachable, then up to 320 kbit/s, then higher bitrate,
-- then AAC, then the oldest. The single definition of "best stream": the sync derives quality and
-- codec from the first row, the relay plays it.
SELECT st.* FROM radio_streams AS st
WHERE st.station_id = @station_id
ORDER BY (st.preferred = 1 AND EXISTS (
             SELECT 1 FROM radio_station_locks AS l
             WHERE l.station_id = st.station_id AND l.field = 'stream')) DESC,
         st.lastcheckok DESC, st.bitrate <= 320 DESC, st.bitrate DESC,
         lower(st.codec) = 'aac' DESC, st.id;
