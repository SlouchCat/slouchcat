-- Three levels: administrators, accounts, profiles (ADR 0014, decision 12 in
-- docs/planning/04-entscheidungen.md).
--
--   accounts.role loses 'owner' and 'guest' and keeps only 'admin' and 'user'; the default moves
--                 from 'owner' to 'user', so a forgotten role creates a harmless account instead of
--                 a silent administrator.
--   accounts.is_owner is the new home of the owner: one row at most (partial unique index), always
--                 an administrator, and it disappears with the account row instead of dangling in
--                 settings.
--   The two CHECKs: an administrator reads the administration views system-wide and without a
--                 profile check, so a ceiling below 18 would be a false promise, not a protection;
--                 and the owner is an administrator with extra rights, never a third role value.
--
-- SQLite has no ALTER COLUMN for a CHECK, so accounts is rebuilt (the 12-step procedure). The id is
-- kept, therefore profiles, sessions, account_libraries, invitations (created_by, used_by),
-- account_channels, telegram_links, push_subscriptions, deliveries, announcements and the
-- self-reference accounts.invited_by still point at the right row; PRAGMA foreign_key_check at the
-- end is the proof.
--
-- The view visible_items has to go first and come back: ALTER TABLE ... RENAME reparses the whole
-- schema and fails while a view names a table that does not exist. It is restored word for word
-- from 00010 (the migration that wrote it last, adding the rule for extras), NOT from 00007, which
-- 00010 superseded. It reads accounts, but never role.

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
    max_rating       INTEGER NOT NULL DEFAULT 18 CHECK (max_rating IN (0, 6, 12, 16, 18)),
    all_libraries    INTEGER NOT NULL DEFAULT 1 CHECK (all_libraries IN (0, 1)),
    invited_by       INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    last_seen_at     INTEGER,
    max_bitrate_kbps INTEGER CHECK (max_bitrate_kbps IS NULL OR max_bitrate_kbps > 0),
    max_height       INTEGER CHECK (max_height IS NULL OR max_height IN (360, 480, 720, 1080, 2160)),
    max_streams      INTEGER CHECK (max_streams IS NULL OR max_streams BETWEEN 1 AND 16),
    CHECK (role <> 'admin' OR (max_rating = 18 AND all_libraries = 1)),
    CHECK (is_owner = 0 OR role = 'admin')
) STRICT;

-- The former owner becomes the administrator and carries the owner badge; every guest becomes a
-- plain account and keeps its ceiling, its library switch and its W-8 caps character for character.
-- The two CASE expressions on max_rating and all_libraries are belt and braces: an owner has 18 and
-- all libraries since 00007, but a hand-edited database must not make the new CHECK abort the run.
INSERT INTO accounts_new (id, email, password_hash, role, is_owner, created_at, updated_at,
                          max_rating, all_libraries, invited_by, last_seen_at,
                          max_bitrate_kbps, max_height, max_streams)
SELECT id, email, password_hash,
       CASE role WHEN 'owner' THEN 'admin' ELSE 'user' END,
       CASE role WHEN 'owner' THEN 1 ELSE 0 END,
       created_at, updated_at,
       CASE role WHEN 'owner' THEN 18 ELSE max_rating END,
       CASE role WHEN 'owner' THEN 1 ELSE all_libraries END,
       invited_by, last_seen_at, max_bitrate_kbps, max_height, max_streams
FROM accounts;

DROP TABLE accounts;
ALTER TABLE accounts_new RENAME TO accounts;

-- At most one owner, enforced by the database and not only by the code.
CREATE UNIQUE INDEX accounts_owner ON accounts (is_owner) WHERE is_owner = 1;

-- Visibility (plan 3.3): the profile's own ceiling and library rights, capped by the account's; an
-- extra only below a visible parent. Word for word from 00010.
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
                  WHERE al.account_id = a.id AND al.library_id = i.library_id));

-- Counter-check in the same run, before COMMIT: a database that holds accounts holds exactly one
-- owner. An empty database (first start, before the setup wizard) holds none, and that is correct.
-- The CHECK on the table already guarantees that an owner is an administrator.
CREATE TEMP TABLE migration_00022_check (ok INTEGER NOT NULL CHECK (ok = 1));
INSERT INTO migration_00022_check (ok)
SELECT CASE WHEN (SELECT count(*) FROM accounts WHERE is_owner = 1)
                 = (SELECT min(count(*), 1) FROM accounts)
            THEN 1 ELSE 0 END;
DROP TABLE migration_00022_check;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
