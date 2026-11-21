-- Concept 27 stage 1 (ADR 0040): three kinds of session. A browser session (cookie, 30 days
-- sliding) is what every session was so far; an app session (the Android phone app) and a device
-- session (the Android TV app) travel as Authorization: Bearer and never expire. "Never" is an
-- expiry far in the future, not NULL, so every query that checks expires_at > @now keeps working;
-- TouchSession does not move it for these kinds. Every existing session is a browser session.
--
-- A column only, nothing is rebuilt; the frame of the template stays.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE sessions ADD COLUMN kind TEXT NOT NULL DEFAULT 'browser' CHECK (kind IN ('browser', 'app', 'device'));
CREATE INDEX sessions_device ON sessions (device_id);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
