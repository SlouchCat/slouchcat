-- Accounts, profiles and access (plan 3.2, 3.3, 3.8, 5.4; M2 design section 1). Times are Unix
-- milliseconds (UTC).
--
--   accounts            max_rating is the FSK ceiling of every profile of the account (guests),
--                       all_libraries/account_libraries limit the libraries of a guest, invited_by and
--                       last_seen_at serve the guest list. Existing owners keep 18 / all libraries.
--   profiles            pin_hash (argon2id PHC, NULL = no PIN); is_default is the primary profile
--                       (API: isPrimary), exactly one per account.
--   sessions            rebuilt: profile_id becomes nullable (signed in, profile not chosen yet; a
--                       deleted profile falls back to NULL) and device_id names the browser.
--   devices             one row per browser (cookie slouchcat_device); rows appear lazily.
--   pin_attempts        PIN lock per profile and device (AN-11), survives restarts.
--   invitations         one-time guest links (AN-14): only the SHA-256 of the 256-bit token is stored.
--   visible_items       now also applies the account ceiling and the account's library rights.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;

DROP VIEW visible_items;

ALTER TABLE accounts ADD COLUMN max_rating INTEGER NOT NULL DEFAULT 18 CHECK (max_rating IN (0, 6, 12, 16, 18));
ALTER TABLE accounts ADD COLUMN all_libraries INTEGER NOT NULL DEFAULT 1 CHECK (all_libraries IN (0, 1));
ALTER TABLE accounts ADD COLUMN invited_by INTEGER REFERENCES accounts(id) ON DELETE SET NULL;
ALTER TABLE accounts ADD COLUMN last_seen_at INTEGER;

-- Library rights of accounts with all_libraries = 0 (guests).
CREATE TABLE account_libraries (
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    PRIMARY KEY (account_id, library_id)
) STRICT, WITHOUT ROWID;

ALTER TABLE profiles ADD COLUMN pin_hash TEXT;

-- One primary profile per account: the one created with the account.
CREATE UNIQUE INDEX profiles_primary ON profiles (account_id) WHERE is_default = 1;

CREATE TABLE devices (
    id           TEXT    PRIMARY KEY CHECK (length(id) = 32),
    user_agent   TEXT,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL
) STRICT;

CREATE TABLE pin_attempts (
    profile_id   INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    device_id    TEXT    NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    failures     INTEGER NOT NULL DEFAULT 0,
    locked_until INTEGER NOT NULL DEFAULT 0,
    updated_at   INTEGER NOT NULL,
    PRIMARY KEY (profile_id, device_id)
) STRICT, WITHOUT ROWID;

-- Sessions: profile_id nullable, device_id new. playback_sessions keeps referencing sessions(id).
CREATE TABLE sessions_new (
    id           INTEGER PRIMARY KEY,
    token_hash   TEXT    NOT NULL UNIQUE,
    account_id   INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id   INTEGER REFERENCES profiles(id) ON DELETE SET NULL,
    device_id    TEXT    REFERENCES devices(id) ON DELETE SET NULL,
    user_agent   TEXT,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,
    revoked_at   INTEGER
) STRICT;

INSERT INTO sessions_new (id, token_hash, account_id, profile_id, device_id, user_agent, created_at, last_seen_at,
                          expires_at, revoked_at)
SELECT id, token_hash, account_id, profile_id, NULL, user_agent, created_at, last_seen_at, expires_at, revoked_at
FROM sessions;

DROP TABLE sessions;
ALTER TABLE sessions_new RENAME TO sessions;
CREATE INDEX sessions_account ON sessions (account_id);
CREATE INDEX sessions_profile ON sessions (profile_id);

CREATE TABLE invitations (
    id            INTEGER PRIMARY KEY,
    token_hash    TEXT    NOT NULL UNIQUE,             -- SHA-256 hex of the plain token
    note          TEXT,
    max_rating    INTEGER NOT NULL CHECK (max_rating IN (0, 6, 12, 16, 18)),
    all_libraries INTEGER NOT NULL CHECK (all_libraries IN (0, 1)),
    created_by    INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    created_at    INTEGER NOT NULL,
    expires_at    INTEGER NOT NULL,
    used_at       INTEGER,
    used_by       INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    revoked_at    INTEGER
) STRICT;

CREATE TABLE invitation_libraries (
    invitation_id INTEGER NOT NULL REFERENCES invitations(id) ON DELETE CASCADE,
    library_id    INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    PRIMARY KEY (invitation_id, library_id)
) STRICT, WITHOUT ROWID;

-- Visibility (plan 3.3): the profile's own ceiling and library rights, capped by the account's.
CREATE VIEW visible_items AS
SELECT p.id AS profile_id, i.id AS item_id
FROM profiles AS p
JOIN accounts AS a ON a.id = p.account_id
JOIN items AS i
  ON i.deleted_at IS NULL
 AND i.available = 1
 AND COALESCE(i.effective_rating, 18) <= min(p.max_rating, a.max_rating)
 AND (i.kind NOT IN ('series', 'season', 'podcast') OR COALESCE(i.min_child_rating, 18) <= min(p.max_rating, a.max_rating))
WHERE (p.all_libraries = 1
       OR EXISTS (SELECT 1 FROM profile_libraries AS pl
                  WHERE pl.profile_id = p.id AND pl.library_id = i.library_id))
  AND (a.all_libraries = 1
       OR EXISTS (SELECT 1 FROM account_libraries AS al
                  WHERE al.account_id = a.id AND al.library_id = i.library_id));

PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
