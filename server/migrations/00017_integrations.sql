-- Incoming Radarr/Sonarr webhooks (plan M9, decision 5.6, 3.8 points 2 and 8; spec I-8).
-- Times are Unix milliseconds (UTC).
--
--   webhook_integrations  one row per address the administration handed to Radarr or Sonarr: the
--                         name, the kind, the SHA-256 of the 256-bit token (the token itself is
--                         shown once and never stored, 3.8 point 2), its last four characters for
--                         the list, the path mapping (JSON array of {"from","to"} prefix pairs,
--                         longest match wins, empty by default) and the last contact.
--   webhook_events        the event log of an integration: when, which event, the reported path,
--                         the path after the mapping and what came of it. A trigger keeps the
--                         newest 20 rows per integration (5.6), so the log never grows.
--
-- The token lives in the URL path of POST /webhooks/{token} and is redacted in every log line
-- (3.8 points 4 and 8, package logging).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE webhook_integrations (
    id           INTEGER PRIMARY KEY,
    name         TEXT    NOT NULL UNIQUE CHECK (length(name) BETWEEN 1 AND 80),
    kind         TEXT    NOT NULL CHECK (kind IN ('radarr', 'sonarr')),
    token_hash   TEXT    NOT NULL UNIQUE CHECK (length(token_hash) = 64),
    token_tail   TEXT    NOT NULL CHECK (length(token_tail) = 4),
    path_map     TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(path_map) AND json_type(path_map) = 'array'),
    last_seen_at INTEGER,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
) STRICT;

CREATE TABLE webhook_events (
    id             INTEGER PRIMARY KEY,
    integration_id INTEGER NOT NULL REFERENCES webhook_integrations(id) ON DELETE CASCADE,
    received_at    INTEGER NOT NULL,
    -- The event name as Radarr/Sonarr sent it (Download, Rename, Test, Grab …), kept verbatim so
    -- the log stays readable when a service adds one.
    event          TEXT    NOT NULL CHECK (length(event) <= 80),
    reported_path  TEXT,
    resolved_path  TEXT,
    result         TEXT    NOT NULL CHECK (result IN ('scan_queued', 'acknowledged', 'ignored',
                                                      'outside_roots', 'invalid_payload'))
) STRICT;
CREATE INDEX webhook_events_integration ON webhook_events (integration_id, id DESC);

-- Keep the newest 20 rows per integration (5.6): the log is a window, not a history.
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
