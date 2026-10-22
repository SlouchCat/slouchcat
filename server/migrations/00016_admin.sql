-- Administration and notifications (plan M8, 3.1 admin/notify, 5.2, 5.5; M8 design section 2).
-- Times are Unix milliseconds (UTC).
--
--   account_channels   one row per account and channel (email, push, telegram): switched on or off,
--                      the chosen events (JSON array of new_content, acquisition, errors, invitations,
--                      announcements) and the channel's settings (JSON: email {"address"}, telegram
--                      {"chat_id"}; neither is a secret). The in-app inbox is always on and no channel.
--   telegram_links     the open link code of an account (six characters, valid for 15 minutes); the
--                      bot links the chat when it receives "/start <code>".
--   push_subscriptions the Web Push subscriptions of an account (endpoint and keys are capability
--                      secrets bound to the server's VAPID pair; a new pair deletes them all).
--   deliveries         one row per account, channel and event; the job notify.deliver carries
--                      {"deliveryId"} and sends it (queued → sent, failed or skipped).
--   announcements      announcements of the main user (A-7); published_at NULL = draft.
--   announcement_acks  which profile acknowledged which announcement.
--
-- New settings keys need no DDL (public_url, update_check_enabled, update_state, smtp_*, telegram_*,
-- vapid_*). Default channels (5.2): every existing account gets the three rows; e-mail is switched on
-- for guests only (event announcements), the main account's e-mail and every push and Telegram row
-- start switched off. New accounts get the same rows from package notify.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE account_channels (
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    channel    TEXT    NOT NULL CHECK (channel IN ('email', 'push', 'telegram')),
    enabled    INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    events     TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(events) AND json_type(events) = 'array'),
    config     TEXT    NOT NULL DEFAULT '{}' CHECK (json_valid(config) AND json_type(config) = 'object'),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (account_id, channel)
) STRICT, WITHOUT ROWID;

CREATE TABLE telegram_links (
    account_id INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    code       TEXT    NOT NULL UNIQUE CHECK (length(code) = 6),
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE push_subscriptions (
    id           INTEGER PRIMARY KEY,
    account_id   INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    device_id    TEXT    REFERENCES devices(id) ON DELETE SET NULL,
    endpoint     TEXT    NOT NULL UNIQUE,
    p256dh       TEXT    NOT NULL,
    auth         TEXT    NOT NULL,
    user_agent   TEXT,
    device_name  TEXT    CHECK (device_name IS NULL OR length(device_name) <= 80),
    created_at   INTEGER NOT NULL,
    last_used_at INTEGER,
    failures     INTEGER NOT NULL DEFAULT 0 CHECK (failures >= 0)
) STRICT;
CREATE INDEX push_subscriptions_account ON push_subscriptions (account_id);
CREATE INDEX push_subscriptions_device ON push_subscriptions (device_id);

CREATE TABLE deliveries (
    id         INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id INTEGER REFERENCES profiles(id) ON DELETE SET NULL,
    channel    TEXT    NOT NULL CHECK (channel IN ('email', 'push', 'telegram')),
    event      TEXT    NOT NULL CHECK (event IN ('new_content', 'acquisition', 'errors', 'invitations', 'announcements')),
    source     TEXT,    -- what raised the event (job type, library); errors of one source go out once per hour
    subject    TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    link       TEXT,
    state      TEXT    NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'sent', 'failed', 'skipped')),
    attempts   INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT,
    created_at INTEGER NOT NULL,
    sent_at    INTEGER
) STRICT;
CREATE INDEX deliveries_state ON deliveries (state, created_at);
CREATE INDEX deliveries_account ON deliveries (account_id, created_at DESC);
CREATE INDEX deliveries_dedupe ON deliveries (event, source, created_at) WHERE source IS NOT NULL;

CREATE TABLE announcements (
    id           INTEGER PRIMARY KEY,
    title        TEXT    NOT NULL CHECK (length(title) BETWEEN 1 AND 120),
    body         TEXT    NOT NULL CHECK (length(body) BETWEEN 1 AND 4000),
    created_by   INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    published_at INTEGER
) STRICT;
CREATE INDEX announcements_published ON announcements (published_at DESC) WHERE published_at IS NOT NULL;

CREATE TABLE announcement_acks (
    announcement_id INTEGER NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    profile_id      INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    acked_at        INTEGER NOT NULL,
    PRIMARY KEY (announcement_id, profile_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX announcement_acks_profile ON announcement_acks (profile_id);

-- Default channels of the existing accounts (5.2). Guests: e-mail on with announcements. Main
-- account: e-mail off, with new content, errors, invitations and announcements chosen. Push and
-- Telegram start off with the same event lists.
INSERT INTO account_channels (account_id, channel, enabled, events, config, updated_at)
SELECT a.id, c.channel,
       CASE WHEN c.channel = 'email' AND a.role = 'guest' THEN 1 ELSE 0 END,
       CASE WHEN a.role = 'guest' THEN '["announcements"]'
            ELSE '["new_content","errors","invitations","announcements"]' END,
       '{}',
       CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER)
FROM accounts AS a
JOIN (SELECT 'email' AS channel UNION ALL SELECT 'push' UNION ALL SELECT 'telegram') AS c;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
