-- Radio details from Wikidata (package store/appq, concept 19 part B stage B2, B7.3; owner: radio).
-- One row per station of the offer with a unique match; radio.wikidata.sync replaces them in one
-- transaction. Times are Unix milliseconds.

-- name: ListRadioStationWikidataIDs :many
SELECT station_id FROM radio_station_wikidata ORDER BY station_id;

-- name: ListRadioStationWikidata :many
SELECT station_id, qid, owner_qid, owner, founded, fetched_at FROM radio_station_wikidata ORDER BY station_id;

-- name: GetRadioStationWikidata :one
SELECT station_id, qid, owner_qid, owner, founded, fetched_at FROM radio_station_wikidata
WHERE station_id = @station_id;

-- name: UpsertRadioStationWikidata :exec
INSERT INTO radio_station_wikidata (station_id, qid, owner_qid, owner, founded, fetched_at)
VALUES (@station_id, @qid, sqlc.narg('owner_qid'), sqlc.narg('owner'), sqlc.narg('founded'), @now)
ON CONFLICT (station_id) DO UPDATE
SET qid = excluded.qid, owner_qid = excluded.owner_qid, owner = excluded.owner,
    founded = excluded.founded, fetched_at = excluded.fetched_at;

-- name: DeleteRadioStationWikidata :exec
DELETE FROM radio_station_wikidata WHERE station_id = @station_id;

-- name: DeleteAllRadioStationWikidata :execrows
DELETE FROM radio_station_wikidata;

-- name: SetRadioWikidataSynced :exec
UPDATE radio_config SET wikidata_synced_at = @now, updated_at = @now WHERE id = 1;
