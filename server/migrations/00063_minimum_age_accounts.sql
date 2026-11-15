-- Concept 22 stage 3 «Freigabeländer» (ADR 0034): accounts.max_rating and invitations.max_rating
-- lose their CHECK on the FSK steps; they hold an age step of the rating country now (13, 15,
-- 17 ...). The new column CHECK keeps the range 0 to 18; the server checks the steps.
--
-- accounts has a table CHECK on max_rating (an administrator has 18 and every library, 00022),
-- which DROP COLUMN refuses, so accounts is rebuilt with the 12-step procedure, as 00022 did.
-- invitations is rebuilt the same way, so both keep their column order. Every other column keeps
-- its type, default and CHECK character for character, the two table CHECKs of accounts stay as
-- they are (admin = 18 and every library; the owner is an administrator). The ids are kept,
-- therefore every table pointing at accounts (profiles, sessions, account_libraries, invitations,
-- account_channels, telegram_links, push_subscriptions, deliveries, announcements, devices and the
-- self-reference invited_by) or at invitations (invitation_libraries) still points at the right
-- row; PRAGMA foreign_key_check at the end is the proof.
--
-- The view visible_items has to go first and come back (ALTER TABLE ... RENAME reparses the whole
-- schema), word for word from 00062, the migration that wrote it last. DROP TABLE accounts takes
-- the trigger activity_events_account_deleted (00031) and the index accounts_owner (00022) with
-- it; both come back unchanged. invitations has neither index nor trigger.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

DROP VIEW visible_items;

CREATE TABLE accounts_new (
    id               INTEGER PRIMARY KEY,
    email            TEXT    NOT NULL COLLATE NOCASE UNIQUE,
    password_hash    TEXT    NOT NULL,            -- argon2id PHC string
    role             TEXT    NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    is_owner         INTEGER NOT NULL DEFAULT 0 CHECK (is_owner IN (0, 1)),
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,
    max_rating       INTEGER NOT NULL DEFAULT 18 CHECK (max_rating BETWEEN 0 AND 18),
    all_libraries    INTEGER NOT NULL DEFAULT 1 CHECK (all_libraries IN (0, 1)),
    invited_by       INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    last_seen_at     INTEGER,
    max_bitrate_kbps INTEGER CHECK (max_bitrate_kbps IS NULL OR max_bitrate_kbps > 0),
    max_height       INTEGER CHECK (max_height IS NULL OR max_height IN (360, 480, 720, 1080, 2160)),
    max_streams      INTEGER CHECK (max_streams IS NULL OR max_streams BETWEEN 1 AND 16),
    CHECK (role <> 'admin' OR (max_rating = 18 AND all_libraries = 1)),
    CHECK (is_owner = 0 OR role = 'admin')
) STRICT;

INSERT INTO accounts_new (id, email, password_hash, role, is_owner, created_at, updated_at,
                          max_rating, all_libraries, invited_by, last_seen_at,
                          max_bitrate_kbps, max_height, max_streams)
SELECT id, email, password_hash, role, is_owner, created_at, updated_at,
       max_rating, all_libraries, invited_by, last_seen_at,
       max_bitrate_kbps, max_height, max_streams
FROM accounts;

DROP TABLE accounts;
ALTER TABLE accounts_new RENAME TO accounts;

-- Word for word from 00022.
CREATE UNIQUE INDEX accounts_owner ON accounts (is_owner) WHERE is_owner = 1;

-- Word for word from 00031.
-- +goose StatementBegin
CREATE TRIGGER activity_events_account_deleted AFTER DELETE ON accounts
BEGIN
    DELETE FROM activity_events WHERE account_id = OLD.id;
END;
-- +goose StatementEnd

CREATE TABLE invitations_new (
    id            INTEGER PRIMARY KEY,
    token_hash    TEXT    NOT NULL UNIQUE,             -- SHA-256 hex of the plain token
    note          TEXT,
    max_rating    INTEGER NOT NULL CHECK (max_rating BETWEEN 0 AND 18),
    all_libraries INTEGER NOT NULL CHECK (all_libraries IN (0, 1)),
    created_by    INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    created_at    INTEGER NOT NULL,
    expires_at    INTEGER NOT NULL,
    used_at       INTEGER,
    used_by       INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    revoked_at    INTEGER
) STRICT;

INSERT INTO invitations_new (id, token_hash, note, max_rating, all_libraries, created_by, created_at,
                             expires_at, used_at, used_by, revoked_at)
SELECT id, token_hash, note, max_rating, all_libraries, created_by, created_at,
       expires_at, used_at, used_by, revoked_at
FROM invitations;

DROP TABLE invitations;
ALTER TABLE invitations_new RENAME TO invitations;

-- Visibility (plan 3.3): word for word from 00062.
CREATE VIEW visible_items AS
SELECT p.id AS profile_id, i.id AS item_id
FROM profiles AS p
JOIN accounts AS a ON a.id = p.account_id
JOIN items AS i
  ON i.deleted_at IS NULL
 AND i.available = 1
 AND COALESCE(i.effective_rating, 18) <= min(p.max_rating, a.max_rating)
 AND (i.kind NOT IN ('series', 'season', 'podcast') OR COALESCE(i.min_child_rating, 18) <= min(p.max_rating, a.max_rating))
 AND (i.kind <> 'extra' OR EXISTS (
        SELECT 1 FROM items AS pa
        WHERE pa.id = i.parent_id AND pa.deleted_at IS NULL AND pa.available = 1
          AND COALESCE(pa.effective_rating, 18) <= min(p.max_rating, a.max_rating)
          AND (pa.kind NOT IN ('series', 'season', 'podcast') OR COALESCE(pa.min_child_rating, 18) <= min(p.max_rating, a.max_rating))))
WHERE (p.all_libraries = 1
       OR EXISTS (SELECT 1 FROM profile_libraries AS pl
                  WHERE pl.profile_id = p.id AND pl.library_id = i.library_id))
  AND (a.all_libraries = 1
       OR EXISTS (SELECT 1 FROM account_libraries AS al
                  WHERE al.account_id = a.id AND al.library_id = i.library_id))
  AND NOT EXISTS (SELECT 1 FROM libraries AS dl WHERE dl.id = i.library_id AND dl.deleting_at IS NOT NULL);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
