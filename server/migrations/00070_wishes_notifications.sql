-- Concept 25 stage 2 «Anfordern» (08-datenmodell-api.md section 2.11, 06-benachrichtigungen.md
-- sections 7 and 11): the inbox and the channel deliveries learn the wish kinds. Both tables carry
-- their kinds in a CHECK constraint, which SQLite cannot change, so both are rebuilt after the
-- template; every row is copied.
--
--   notifications  kind adds wish_available, wish_season_available, wish_approved, wish_declined
--                  and wish_failed (all kinds at once, also those whose producer comes in stage 4:
--                  one rebuild instead of two, data model 2.11 «Vorschlag»; the contract takes a
--                  value only with its producer, ADR 0009). item_id may be NULL now: a wish entry
--                  points at its wish (wish_id, gone with the wish) and at an item only once the
--                  title is in the library. dedupe_key replaces UNIQUE (profile_id, kind, item_id):
--                  'episode:<item_id>' for a new episode (every existing row gets it), for a wish
--                  'wish:<id>:available', 'wish:<id>:season:<n>', 'wish:<id>:approved',
--                  'wish:<id>:declined', 'wish:<id>:failed'. A new episode always has an item and
--                  no wish; every other kind has a wish.
--   deliveries     event adds wish_available, wish_status and wish_review (F-14).
--
-- The channel lists of existing accounts (account_channels.events) are not touched here: the event
-- wish_available has no producer yet, and an event without a producer would show in the channel
-- settings (ADR 0009). Its backfill comes with the producer (plan step 2.6).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE notifications_new (
    id         INTEGER PRIMARY KEY,
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL CHECK (kind IN ('new_episode', 'wish_available', 'wish_season_available',
                                                'wish_approved', 'wish_declined', 'wish_failed')),
    item_id    TEXT    REFERENCES items(id) ON DELETE CASCADE,
    parent_id  TEXT    REFERENCES items(id) ON DELETE SET NULL,
    wish_id    INTEGER REFERENCES wishes(id) ON DELETE CASCADE,
    dedupe_key TEXT    NOT NULL CHECK (dedupe_key <> ''),
    created_at INTEGER NOT NULL,
    read_at    INTEGER,
    UNIQUE (profile_id, dedupe_key),
    CHECK (kind = 'new_episode' OR wish_id IS NOT NULL),
    CHECK (kind <> 'new_episode' OR (item_id IS NOT NULL AND wish_id IS NULL)),
    CHECK (item_id IS NOT NULL OR wish_id IS NOT NULL)
) STRICT;

INSERT INTO notifications_new (id, profile_id, kind, item_id, parent_id, wish_id, dedupe_key, created_at, read_at)
SELECT id, profile_id, kind, item_id, parent_id, NULL, 'episode:' || item_id, created_at, read_at
FROM notifications;

DROP TABLE notifications;
ALTER TABLE notifications_new RENAME TO notifications;

CREATE INDEX notifications_inbox ON notifications (profile_id, created_at DESC, id DESC);
CREATE INDEX notifications_unread ON notifications (profile_id, read_at);
CREATE INDEX notifications_item ON notifications (item_id);
CREATE INDEX notifications_parent ON notifications (parent_id);
CREATE INDEX notifications_wish ON notifications (wish_id);

CREATE TABLE deliveries_new (
    id         INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id INTEGER REFERENCES profiles(id) ON DELETE SET NULL,
    channel    TEXT    NOT NULL CHECK (channel IN ('email', 'push', 'telegram')),
    event      TEXT    NOT NULL CHECK (event IN ('new_content', 'errors', 'invitations', 'announcements',
                                                 'wish_available', 'wish_status', 'wish_review')),
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
