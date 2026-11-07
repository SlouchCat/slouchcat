-- Webradio per profile (package store/appq, concept 16 sections 5.4 and 10.2; owner: radio, lane B).
-- Favourites and the remembered filter of a profile. The station rows name no title (rule 2). The
-- visibility of a station (hidden, countries setting) is decided in package radio before any of
-- these writes run; a hidden or excluded favourite stays in the table and is left out on read.
-- Times are Unix milliseconds.

-- name: GetRadioProfileState :one
SELECT * FROM radio_profile_state WHERE profile_id = @profile_id;

-- name: UpsertRadioProfileState :exec
-- last_filter is the JSON of the filter part of model.RadioQuery; NULL clears it.
INSERT INTO radio_profile_state (profile_id, default_country, last_filter, last_sort, updated_at)
VALUES (@profile_id, sqlc.narg('default_country'), sqlc.narg('last_filter'), sqlc.narg('last_sort'), @now)
ON CONFLICT (profile_id) DO UPDATE SET
    default_country = excluded.default_country, last_filter = excluded.last_filter,
    last_sort = excluded.last_sort, updated_at = excluded.updated_at;

-- name: ListRadioFavorites :many
-- The favourites of a profile in its order, with the station row (broken and missing stations
-- included, F-18 removes them with the station).
SELECT sqlc.embed(s), f.position, f.created_at AS favorite_created_at
FROM radio_favorites AS f
JOIN radio_stations AS s ON s.id = f.station_id
WHERE f.profile_id = @profile_id
ORDER BY f.position, f.station_id;

-- name: ListRadioFavoriteStationIDs :many
-- The station ids a profile marked, for the favorite flag of a list page.
SELECT station_id FROM radio_favorites WHERE profile_id = @profile_id ORDER BY position, station_id;

-- name: AddRadioFavorite :exec
-- Appends the station behind the last favourite; adding again keeps the position and created_at.
INSERT INTO radio_favorites (profile_id, station_id, position, created_at)
VALUES (@profile_id, @station_id,
        (SELECT COALESCE(MAX(position), -1) + 1 FROM radio_favorites WHERE profile_id = @profile_id), @now)
ON CONFLICT (profile_id, station_id) DO NOTHING;

-- name: RemoveRadioFavorite :exec
DELETE FROM radio_favorites WHERE profile_id = @profile_id AND station_id = @station_id;

-- name: SetRadioFavoritePosition :execrows
-- One row of a new order; the caller rewrites every position in one transaction.
UPDATE radio_favorites SET position = @position WHERE profile_id = @profile_id AND station_id = @station_id;

-- name: ListFavoriteStationIDsOfAllProfiles :many
-- Every station some profile marked, for the logo prefetch (section 6).
SELECT DISTINCT station_id FROM radio_favorites ORDER BY station_id;
