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

-- Catalog sync (concept 16 section 5.3, lane B): the merge of the Radio Browser entries into
-- stations. An alias never moves to another station and two stations never merge.

-- name: ListRadioAliasesForSync :many
-- Every alias with its station, the lastchangetime of its last import and the check flag of its
-- stream, loaded once per sync.
SELECT a.stationuuid, a.station_id, a.catalog_changed,
       CAST(COALESCE(st.lastcheckok, 1) AS INTEGER) AS lastcheckok
FROM radio_station_aliases AS a
LEFT JOIN radio_streams AS st ON st.stationuuid = a.stationuuid;

-- name: ListRadioStationsForSync :many
-- Group key and counters of every station, oldest first (the group map keeps the oldest catalog
-- station of a key).
SELECT id, source, group_key, votes, clicks, reachable FROM radio_stations ORDER BY id;

-- name: ListRadioStationLockFields :many
-- Locks of every station (station_id NULL) or of one station.
SELECT station_id, field FROM radio_station_locks
WHERE station_id = sqlc.narg('station_id') OR sqlc.narg('station_id') IS NULL
ORDER BY station_id, field;

-- name: InsertRadioCatalogStation :one
-- A new catalog station; the recompute after the entries fills the derived columns.
INSERT INTO radio_stations (source, group_key, name, sort_key, search_text, country, created_at, updated_at)
VALUES ('catalog', @group_key, @name, @sort_key, @search_text, sqlc.narg('country'), @now, @now)
RETURNING id;

-- name: UpsertRadioAlias :exec
-- A new or changed alias; the station of a known alias never changes.
INSERT INTO radio_station_aliases (stationuuid, station_id, seen_at, catalog_changed)
VALUES (@stationuuid, @station_id, @now, sqlc.narg('catalog_changed'))
ON CONFLICT (stationuuid) DO UPDATE SET
    seen_at = excluded.seen_at, catalog_changed = excluded.catalog_changed, missing_since = NULL;

-- name: TouchRadioAlias :exec
-- An unchanged alias was seen again.
UPDATE radio_station_aliases SET seen_at = @now, missing_since = NULL WHERE stationuuid = @stationuuid;

-- name: UpsertRadioCatalogStream :exec
-- The stream of a new or changed alias; the admin's preferred flag stays.
INSERT INTO radio_streams (station_id, stationuuid, url, url_resolved, codec, bitrate, hls, lastcheckok, updated_at)
VALUES (@station_id, @stationuuid, @url, sqlc.narg('url_resolved'), sqlc.narg('codec'), sqlc.narg('bitrate'),
        @hls, @lastcheckok, @now)
ON CONFLICT (stationuuid) DO UPDATE SET
    url = excluded.url, url_resolved = excluded.url_resolved, codec = excluded.codec,
    bitrate = excluded.bitrate, hls = excluded.hls, lastcheckok = excluded.lastcheckok,
    updated_at = excluded.updated_at;

-- name: SetRadioStreamCheck :exec
-- The check flag of an unchanged alias's stream changed.
UPDATE radio_streams SET lastcheckok = @lastcheckok, updated_at = @now WHERE stationuuid = @stationuuid;

-- name: UpdateRadioStationFromSync :exec
-- The derived columns of a station after its entries changed (locked fields are passed through).
UPDATE radio_stations
SET name = @name, sort_key = @sort_key, search_text = @search_text, country = sqlc.narg('country'),
    region = sqlc.narg('region'), languages = @languages, homepage = sqlc.narg('homepage'),
    favicon_url = sqlc.narg('favicon_url'), votes = @votes, clicks = @clicks,
    quality = sqlc.narg('quality'), reachable = @reachable, missing_since = NULL, updated_at = @now
WHERE id = @id;

-- name: UpdateRadioStationCounters :exec
-- Votes, clicks and reachability of a station whose entries are otherwise unchanged.
UPDATE radio_stations SET votes = @votes, clicks = @clicks, reachable = @reachable, updated_at = @now
WHERE id = @id;

-- name: DeleteRadioStationTags :exec
DELETE FROM radio_station_tags WHERE station_id = @station_id;

-- name: InsertRadioStationTags :exec
-- tags is a JSON array of strings.
INSERT INTO radio_station_tags (station_id, tag)
SELECT @station_id, j.value FROM json_each(@tags) AS j WHERE true
ON CONFLICT DO NOTHING;

-- name: InsertRadioStationFacets :exec
-- pairs is a JSON array of [facet, value] pairs.
INSERT INTO radio_station_facets (station_id, facet, value, manual)
SELECT @station_id, json_extract(j.value, '$[0]'), json_extract(j.value, '$[1]'), @manual
FROM json_each(@pairs) AS j WHERE true
ON CONFLICT (station_id, facet, value) DO UPDATE SET manual = excluded.manual;

-- name: DeleteRadioStationFacets :exec
-- Every value of the facets named in the JSON array facets.
DELETE FROM radio_station_facets
WHERE station_id = @station_id AND facet IN (SELECT j.value FROM json_each(@facets) AS j);

-- name: MarkRadioAliasesMissing :execrows
-- Aliases the sync that started at run_start did not see.
UPDATE radio_station_aliases SET missing_since = @now WHERE seen_at < @run_start AND missing_since IS NULL;

-- name: MarkRadioStationsMissing :execrows
-- Catalog stations whose aliases are all missing.
UPDATE radio_stations SET missing_since = @now, updated_at = @now
WHERE source = 'catalog' AND missing_since IS NULL
  AND NOT EXISTS (SELECT 1 FROM radio_station_aliases AS a
                  WHERE a.station_id = radio_stations.id AND a.missing_since IS NULL);

-- name: ClearRadioStationsMissing :execrows
-- Stations of which an alias came back.
UPDATE radio_stations SET missing_since = NULL, updated_at = @now
WHERE missing_since IS NOT NULL
  AND EXISTS (SELECT 1 FROM radio_station_aliases AS a
              WHERE a.station_id = radio_stations.id AND a.missing_since IS NULL);

-- name: DeleteMissingRadioStations :execrows
-- Catalog stations missing since before; favourites, locks and the rest go by cascade (F-18).
-- Own stations are never removed by the sync.
DELETE FROM radio_stations WHERE source = 'catalog' AND missing_since IS NOT NULL AND missing_since < @before;

-- name: DeleteMissingRadioAliases :many
-- Aliases missing since before on surviving stations (their streams go by cascade).
DELETE FROM radio_station_aliases WHERE missing_since IS NOT NULL AND missing_since < @before
RETURNING station_id;

-- name: CountRadioStations :one
SELECT count(*) FROM radio_stations;

-- name: ListRecommendedRadioStationIDs :many
-- Stations the admin recommends, for the logo prefetch.
SELECT id FROM radio_stations WHERE recommended = 1 ORDER BY id;

-- Station administration (concept 16 section 5.6, lane B).

-- name: CountRadioStationAliases :one
SELECT count(*) FROM radio_station_aliases WHERE station_id = @station_id;

-- name: UpdateRadioStationAdmin :exec
UPDATE radio_stations
SET name = @name, sort_key = @sort_key, search_text = @search_text, region = sqlc.narg('region'),
    hidden = @hidden, recommended = @recommended, updated_at = @now
WHERE id = @id;

-- name: DeleteRadioStationLocks :exec
DELETE FROM radio_station_locks WHERE station_id = @station_id;

-- name: InsertRadioStationLock :exec
INSERT INTO radio_station_locks (station_id, field) VALUES (@station_id, @field) ON CONFLICT DO NOTHING;

-- name: SetRadioStreamPreferred :exec
-- Marks one stream of a station as the admin's choice (stream_id NULL: none).
UPDATE radio_streams
SET preferred = CASE WHEN id = sqlc.narg('stream_id') THEN 1 ELSE 0 END
WHERE station_id = @station_id;
