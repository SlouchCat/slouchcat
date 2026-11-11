-- The radio offer (package store/appq, concept 19 part B, B4 and B10; owner: radio): the chosen
-- countries with their mode and the exceptions per station. The service keeps the offer as one
-- snapshot in memory and reads these tables only at the first use and when it stores a draft.

-- name: GetRadioOfferState :one
SELECT offer_version, offer_seeded, hide_broken FROM radio_config WHERE id = 1;

-- name: ListRadioOfferCountries :many
SELECT country, mode FROM radio_offer_countries ORDER BY country;

-- name: ListRadioOfferExceptions :many
SELECT station_id, offered FROM radio_offer_exceptions ORDER BY station_id;

-- name: UpsertRadioOfferCountry :exec
INSERT INTO radio_offer_countries (country, mode, updated_at) VALUES (@country, @mode, @now)
ON CONFLICT (country) DO UPDATE SET mode = excluded.mode, updated_at = excluded.updated_at;

-- name: DeleteRadioOfferCountry :exec
DELETE FROM radio_offer_countries WHERE country = @country;

-- name: UpsertRadioOfferException :exec
-- A station gone since the draft was computed has no row to refer to: the insert skips it.
INSERT INTO radio_offer_exceptions (station_id, offered, updated_at)
SELECT @station_id, @offered, @now WHERE EXISTS (SELECT 1 FROM radio_stations WHERE id = @station_id)
ON CONFLICT (station_id) DO UPDATE SET offered = excluded.offered, updated_at = excluded.updated_at;

-- name: DeleteRadioOfferException :exec
DELETE FROM radio_offer_exceptions WHERE station_id = @station_id;

-- name: StoreRadioOfferVersion :execrows
-- The next version of the offer, only while the stored one is still expected (B4.3).
UPDATE radio_config SET offer_version = @version, hide_broken = @hide_broken, updated_at = @now
WHERE id = 1 AND offer_version = @expected;

-- name: ListHiddenRadioStationIDs :many
-- The stations the administration hid before 00048: the first start demarks them (B4.4).
SELECT id FROM radio_stations WHERE hidden = 1 ORDER BY id;

-- name: SetRadioOfferSeeded :exec
UPDATE radio_config SET offer_seeded = 1, updated_at = @now WHERE id = 1;

-- name: ListRadioStationsForSeed :many
-- Name and the address of the best stream (the first row of ListRadioStreamsRanked) of every
-- station, for the first start: the filter values and the provider of all of them (B4.4).
WITH ranked AS (
    SELECT st.station_id, st.url, st.url_resolved,
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
SELECT s.id, s.name, r.url AS best_url, r.url_resolved AS best_url_resolved
FROM radio_stations AS s
LEFT JOIN ranked AS r ON r.station_id = s.id AND r.rn = 1
ORDER BY s.id;

-- name: ListAllRadioStationTags :many
-- Every raw tag of every station, by station (the first start).
SELECT station_id, tag FROM radio_station_tags ORDER BY station_id, tag;

-- name: ListRadioStationTagsOf :many
-- The raw tags of the stations in the JSON array ids (one page of the preview list).
SELECT station_id, tag FROM radio_station_tags
WHERE station_id IN (SELECT j.value FROM json_each(@ids) AS j)
ORDER BY station_id, tag;
