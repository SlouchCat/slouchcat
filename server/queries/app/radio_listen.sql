-- Webradio listen sessions and listen time (package store/appq, concept 16 sections 7.2, 7.5 and
-- 7.10; owner: radio, lane A). A listen session belongs to one login session, account, profile and
-- station; ended_by takes the model.StoppedBy* values plus radio's own 'expired', 'station' and
-- 'restart'. Times are Unix milliseconds.

-- name: InsertRadioListenSession :exec
INSERT INTO radio_listen_sessions (id, session_id, account_id, profile_id, station_id, created_at,
                                   last_seen_at, expires_at, playing)
VALUES (@id, @session_id, @account_id, @profile_id, @station_id, @now, @now, @expires_at, 1);

-- name: GetRadioListenSession :one
SELECT * FROM radio_listen_sessions WHERE id = @id;

-- name: TouchRadioListenSession :execrows
-- A heartbeat or a live request (at most once a minute, M-2) slides the expiry of an open session.
UPDATE radio_listen_sessions SET last_seen_at = @now, expires_at = @expires_at, playing = @playing
WHERE id = @id AND ended_at IS NULL;

-- name: EndRadioListenSession :execrows
UPDATE radio_listen_sessions SET ended_at = @now, ended_by = @ended_by
WHERE id = @id AND ended_at IS NULL;

-- name: EndRadioListenSessionsBySession :many
-- Logout or revocation of a login session; returns the ended ids so the relays close them.
UPDATE radio_listen_sessions SET ended_at = @now, ended_by = @ended_by
WHERE session_id = @session_id AND ended_at IS NULL
RETURNING id;

-- name: EndRadioListenSessionsByProfile :many
UPDATE radio_listen_sessions SET ended_at = @now, ended_by = @ended_by
WHERE profile_id = @profile_id AND ended_at IS NULL
RETURNING id;

-- name: EndRadioListenSessionsByAccount :many
UPDATE radio_listen_sessions SET ended_at = @now, ended_by = @ended_by
WHERE account_id = @account_id AND ended_at IS NULL
RETURNING id;

-- name: EndRadioListenSessionsOfStation :many
-- The relay of a station ended (ended_by 'station').
UPDATE radio_listen_sessions SET ended_at = @now, ended_by = @ended_by
WHERE station_id = @station_id AND ended_at IS NULL
RETURNING id;

-- name: EndOpenRadioListenSessions :execrows
-- At startup no relay runs, so every open session ended with the last process ('restart').
UPDATE radio_listen_sessions SET ended_at = @now, ended_by = 'restart' WHERE ended_at IS NULL;

-- name: ExpireRadioListenSessions :many
-- Sessions whose client stopped asking (expires_at passed) end as 'expired'.
UPDATE radio_listen_sessions SET ended_at = @now, ended_by = 'expired'
WHERE ended_at IS NULL AND expires_at <= @now
RETURNING id;

-- name: ListOpenRadioListenSessions :many
SELECT * FROM radio_listen_sessions WHERE ended_at IS NULL ORDER BY station_id, created_at, id;

-- name: AddRadioListenTime :exec
-- Adds listened time to one quarter hour of a profile; a quarter never holds more than 15 minutes.
INSERT INTO radio_listen_time (profile_id, quarter, played_ms)
VALUES (@profile_id, @quarter, min(900000, @played_ms))
ON CONFLICT (profile_id, quarter) DO UPDATE SET
    played_ms = min(900000, radio_listen_time.played_ms + excluded.played_ms);

-- name: DeleteOldRadioListenTime :execrows
-- Listen time older than the K-5 report needs (quarter < @before_quarter).
DELETE FROM radio_listen_time WHERE quarter < @before_quarter;

-- name: SetRadioStationShowsTitles :execrows
-- The relay saw the first real title of a station (concept 16 section 8, facet "titles").
UPDATE radio_stations SET shows_titles = 1 WHERE id = @id AND shows_titles = 0;

-- name: RecordRadioRelayAbort :one
-- A relay of the station ended because no stream answered (7.4): one abort more; at three in a
-- row the station is marked broken.
UPDATE radio_stations SET
    failures = failures + 1,
    broken_at = CASE WHEN failures + 1 >= 3 THEN coalesce(broken_at, @now) ELSE broken_at END
WHERE id = @id
RETURNING failures, broken_at;

-- name: ClearRadioStationBroken :execrows
-- A relay of a broken station started to play again (S2 visibility table: success clears broken_at).
UPDATE radio_stations SET broken_at = NULL WHERE id = @id AND broken_at IS NOT NULL;

-- name: ResetRadioRelayFailures :execrows
-- A relay ran for a minute: the aborts before no longer count.
UPDATE radio_stations SET failures = 0 WHERE id = @id AND failures > 0;

-- Station logos (concept 16 section 6, build plan S7; owner: radio, lane A). logo_state: none,
-- queued, catalog, website, own, failed; logo_hash is the sha256 of the source image.

-- name: MarkRadioLogoQueued :execrows
-- A station without a logo, or whose chain failed before retry_before, gets the logo job.
UPDATE radio_stations SET logo_state = 'queued'
WHERE id = @id AND (logo_state = 'none'
    OR (logo_state = 'failed' AND (logo_tried_at IS NULL OR logo_tried_at < @retry_before)));

-- name: SetRadioLogoFetched :execrows
-- The logo job's result (catalog, website or failed); an own upload is never replaced.
UPDATE radio_stations SET logo_state = @logo_state, logo_hash = @logo_hash, logo_tried_at = @now
WHERE id = @id AND logo_state <> 'own';

-- name: SetRadioLogoOwn :exec
UPDATE radio_stations SET logo_state = @logo_state, logo_hash = @logo_hash WHERE id = @id;

-- name: LockRadioStationLogo :exec
INSERT INTO radio_station_locks (station_id, field) VALUES (@station_id, 'logo') ON CONFLICT DO NOTHING;

-- name: UnlockRadioStationLogo :exec
DELETE FROM radio_station_locks WHERE station_id = @station_id AND field = 'logo';

-- name: ListRecommendedRadioStationIDs :many
-- The recommended stations, whose logos the catalog sync prefetches.
SELECT id FROM radio_stations WHERE recommended = 1 ORDER BY id;
