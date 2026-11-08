-- Tag rules of the administration (package store/appq, concept 16 section 5.4, stage 2; owner:
-- radio). The admin's rules override the embedded default rules per tag and facet (migration
-- 00039). The counts and the derived facets of every station are read once per change of the rules
-- or per view of the tab "Einordnung", never per station.

-- name: ListRadioTagRules :many
SELECT tag, facet, value, updated_at FROM radio_tag_rules ORDER BY tag, facet;

-- name: DeleteRadioTagRules :exec
DELETE FROM radio_tag_rules;

-- name: InsertRadioTagRule :exec
INSERT INTO radio_tag_rules (tag, facet, value, updated_at) VALUES (@tag, @facet, sqlc.narg('value'), @now);

-- name: ListRadioTagCounts :many
-- Stations per raw tag, most first (the worklist and the station numbers of the rules).
SELECT tag, count(*) AS stations FROM radio_station_tags GROUP BY tag ORDER BY stations DESC, tag;

-- name: ListAllRadioStationTags :many
-- Every raw tag of every station, by station (applying changed rules).
SELECT station_id, tag FROM radio_station_tags ORDER BY station_id, tag;

-- name: ListDerivedRadioStationFacets :many
-- The facet values the rules derived (manual = 0), by station (applying changed rules).
SELECT station_id, facet, value FROM radio_station_facets WHERE manual = 0 ORDER BY station_id, facet, value;
