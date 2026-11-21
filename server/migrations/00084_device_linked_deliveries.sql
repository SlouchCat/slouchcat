-- Concept 27 stage 1 (ADR 0040, «Kopplung»; K26 4.6 K-7): every pairing of a TV goes out as the
-- message «Neues Gerät verbunden» on the channels of the account (e-mail, Telegram, push). The
-- deliveries learn the event device_linked. The table carries its events in a CHECK constraint,
-- which SQLite cannot change, so it is rebuilt after the template; every row is copied.
--
-- The inbox (notifications) is not touched: it belongs to a profile and every kind there needs a
-- title or a wish (migration 00070), while the message belongs to the account. The channel lists
-- (account_channels.events) are not touched either: device_linked is no event an account chooses;
-- it goes to every switched-on channel of the account (notify.DeviceLinked).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE deliveries_new (
    id         INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id INTEGER REFERENCES profiles(id) ON DELETE SET NULL,
    channel    TEXT    NOT NULL CHECK (channel IN ('email', 'push', 'telegram')),
    event      TEXT    NOT NULL CHECK (event IN ('new_content', 'errors', 'invitations', 'announcements',
                                                 'wish_available', 'wish_status', 'wish_review',
                                                 'device_linked')),
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
FROM deliveries;

DROP TABLE deliveries;
ALTER TABLE deliveries_new RENAME TO deliveries;

CREATE INDEX deliveries_state ON deliveries (state, created_at);
CREATE INDEX deliveries_account ON deliveries (account_id, created_at DESC);
CREATE INDEX deliveries_dedupe ON deliveries (event, source, created_at) WHERE source IS NOT NULL;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
