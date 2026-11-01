-- Activity log of the admin overview (concept 11 section 11.4, ADR 0019): one row per event
-- ("Letzte Aktivität"), written by admin.RecordActivity with the flood rules per kind and read by
-- getDashboard (the six newest). kind is a string without CHECK: the known kinds are checked in Go,
-- and a new kind breaks no client. at is Unix milliseconds (UTC) like everywhere. subject holds the
-- name of the title or the library at the time of the event, so an entry outlives the deletion of
-- either; there are deliberately no foreign keys on items and libraries. Account and profile are
-- kept as ids only: the e-mail address and the profile name are read when the log is read, and
-- deleting an account or a profile deletes its rows (auth.DeleteAccount, auth.DeleteProfile).
-- detail is a JSON object with the few numbers and fixed codes of ActivityEntry.detail; no secret,
-- no path, no IP address and no free error text (rule 10). Rows older than 30 days, and beyond the
-- newest 2'000, are removed by the cleanup hook of jobs.prune.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE activity_events (
    id         INTEGER PRIMARY KEY,
    at         INTEGER NOT NULL,
    kind       TEXT    NOT NULL,
    account_id INTEGER,
    profile_id INTEGER,
    item_id    TEXT,
    library_id INTEGER,
    subject    TEXT,
    detail     TEXT    NOT NULL DEFAULT '{}' CHECK (json_valid(detail))
) STRICT;

CREATE INDEX activity_events_at ON activity_events (at DESC);
CREATE INDEX activity_events_account ON activity_events (account_id);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
