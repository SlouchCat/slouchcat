-- Removes the notification event "acquisition" (plan M9 step 4, decision of 22 October 2026).
--
-- Its producer was I-1, the Radarr/Sonarr client with the acquisition state of a title. I-1 fell
-- away when M9 was cut down to the incoming webhook, so the event has no producer any more and no
-- account can choose it. Two places carry it:
--
--   deliveries.event        a CHECK constraint listing the five event names (migration 00016).
--                           SQLite cannot drop a CHECK, so the table is rebuilt after the
--                           template; the rows are copied, which cannot lose anything because
--                           nothing ever wrote an acquisition delivery.
--   account_channels.events a JSON array of chosen events. The name is removed from every array,
--                           so the settings page shows four event types instead of five.
--
-- Forward only, like every migration: an older server would reject an unknown event anyway.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE deliveries_new (
    id         INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id INTEGER REFERENCES profiles(id) ON DELETE SET NULL,
    channel    TEXT    NOT NULL CHECK (channel IN ('email', 'push', 'telegram')),
    event      TEXT    NOT NULL CHECK (event IN ('new_content', 'errors', 'invitations', 'announcements')),
    source     TEXT,
    subject    TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    link       TEXT,
    state      TEXT    NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'sent', 'failed', 'skipped')),
    attempts   INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT,
    created_at INTEGER NOT NULL,
    sent_at    INTEGER
) STRICT;

INSERT INTO deliveries_new (id, account_id, profile_id, channel, event, source, subject, body, link,
                            state, attempts, last_error, created_at, sent_at)
SELECT id, account_id, profile_id, channel, event, source, subject, body, link,
       state, attempts, last_error, created_at, sent_at
FROM deliveries
WHERE event <> 'acquisition';

DROP TABLE deliveries;
ALTER TABLE deliveries_new RENAME TO deliveries;

CREATE INDEX deliveries_state ON deliveries (state, created_at);
CREATE INDEX deliveries_account ON deliveries (account_id, created_at DESC);
CREATE INDEX deliveries_dedupe ON deliveries (event, source, created_at) WHERE source IS NOT NULL;

-- The chosen events of every channel, without "acquisition" and in their original order.
UPDATE account_channels
SET events = (SELECT json_group_array(value)
              FROM json_each(account_channels.events)
              WHERE value <> 'acquisition')
WHERE EXISTS (SELECT 1 FROM json_each(account_channels.events) WHERE value = 'acquisition');

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
