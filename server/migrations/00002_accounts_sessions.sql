-- Settings, accounts, profiles and sessions (plan 3.1 auth/profiles, 3.8 point 2 and 7, M1a step 2).
-- Times are Unix milliseconds (UTC). M1a creates one owner account with one default profile; M2 adds
-- guests, more profiles, PINs and devices.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;

-- Key/value settings. Secrets are stored encrypted by the secrets package ("v1:" prefix).
CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE TABLE accounts (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL COLLATE NOCASE UNIQUE,
    password_hash TEXT    NOT NULL,            -- argon2id PHC string
    role          TEXT    NOT NULL DEFAULT 'owner' CHECK (role IN ('owner', 'guest')),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
) STRICT;

-- Profiles carry no e-mail and no password (K-1). max_rating is the FSK ceiling (K-2, K-3).
CREATE TABLE profiles (
    id            INTEGER PRIMARY KEY,
    account_id    INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name          TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 60),
    image         TEXT,
    max_rating    INTEGER NOT NULL DEFAULT 18 CHECK (max_rating IN (0, 6, 12, 16, 18)),
    language      TEXT    NOT NULL DEFAULT 'de' CHECK (language IN ('de', 'en')),
    accent        TEXT,
    all_libraries INTEGER NOT NULL DEFAULT 1 CHECK (all_libraries IN (0, 1)),
    is_default    INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
) STRICT;

CREATE INDEX profiles_account ON profiles (account_id);

-- Library rights of profiles with all_libraries = 0 (the view visible_items reads it).
CREATE TABLE profile_libraries (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    library_id INTEGER NOT NULL,
    PRIMARY KEY (profile_id, library_id)
) STRICT, WITHOUT ROWID;

-- Sessions store only the SHA-256 of the 256-bit token (plan 3.8 point 2).
CREATE TABLE sessions (
    id           INTEGER PRIMARY KEY,
    token_hash   TEXT    NOT NULL UNIQUE,
    account_id   INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id   INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    user_agent   TEXT,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,
    revoked_at   INTEGER
) STRICT;

CREATE INDEX sessions_account ON sessions (account_id);

PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
