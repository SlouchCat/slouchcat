-- Concept 25 stage 4, build step 4.5 (F-18; architecture 10, data model 2.11): the Seerr webhook as
-- a trigger of the sync. Seerr calls the same receiver POST /webhooks/{token} as Radarr and Sonarr;
-- its notification is never read for a state, it only queues a run of wishes.sync.
--
--   webhook_integrations  the kind seerr joins radarr and sonarr (identifier of the service:
--                         seerr, not jellyseerr). Everything else stays as migration 00017 made it.
--   webhook_events        the result sync_queued joins the results: a Seerr notification that
--                         queued (or joined) a run of the sync.
--
-- Both tables are rebuilt because SQLite cannot change a CHECK; every row, its id and the trigger
-- that keeps 20 events per integration stay.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE webhook_integrations_new (
    id           INTEGER PRIMARY KEY,
    name         TEXT    NOT NULL UNIQUE CHECK (length(name) BETWEEN 1 AND 80),
    kind         TEXT    NOT NULL CHECK (kind IN ('radarr', 'sonarr', 'seerr')),
    token_hash   TEXT    NOT NULL UNIQUE CHECK (length(token_hash) = 64),
    token_tail   TEXT    NOT NULL CHECK (length(token_tail) = 4),
    path_map     TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(path_map) AND json_type(path_map) = 'array'),
    last_seen_at INTEGER,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
) STRICT;

INSERT INTO webhook_integrations_new (id, name, kind, token_hash, token_tail, path_map, last_seen_at, created_at, updated_at)
SELECT id, name, kind, token_hash, token_tail, path_map, last_seen_at, created_at, updated_at FROM webhook_integrations;

CREATE TABLE webhook_events_new (
    id             INTEGER PRIMARY KEY,
    integration_id INTEGER NOT NULL REFERENCES webhook_integrations(id) ON DELETE CASCADE,
    received_at    INTEGER NOT NULL,
    event          TEXT    NOT NULL CHECK (length(event) <= 80),
    reported_path  TEXT,
    resolved_path  TEXT,
    result         TEXT    NOT NULL CHECK (result IN ('scan_queued', 'acknowledged', 'ignored',
                                                      'outside_roots', 'invalid_payload', 'sync_queued'))
) STRICT;

INSERT INTO webhook_events_new (id, integration_id, received_at, event, reported_path, resolved_path, result)
SELECT id, integration_id, received_at, event, reported_path, resolved_path, result FROM webhook_events;

DROP TABLE webhook_events;
DROP TABLE webhook_integrations;
ALTER TABLE webhook_integrations_new RENAME TO webhook_integrations;
ALTER TABLE webhook_events_new RENAME TO webhook_events;

CREATE INDEX webhook_events_integration ON webhook_events (integration_id, id DESC);

-- Keep the newest 20 rows per integration (as migration 00017).
-- +goose StatementBegin
CREATE TRIGGER webhook_events_trim AFTER INSERT ON webhook_events
BEGIN
    DELETE FROM webhook_events
    WHERE integration_id = NEW.integration_id
      AND id <= (SELECT id FROM webhook_events
                 WHERE integration_id = NEW.integration_id
                 ORDER BY id DESC LIMIT 1 OFFSET 20);
END;
-- +goose StatementEnd

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
