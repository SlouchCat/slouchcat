-- Wishes through Seerr, stage 1 (concept 25, data model 2.1, 2.2, 2.8, 2.13; Marco's answers of
-- 17 November 2026, appendix H 11.3): the mirror of the Seerr users, the Seerr user of each account,
-- the limit per account and the call log of the Seerr view. New tables only.
--
--   * seerr_users: the users of GET /user. is_admin is ADMIN or MANAGE_USERS (no limit, no password
--     sync); created_at is Seerr's createdAt, part of the fingerprint of the instance; gone_at is
--     set when a refresh no longer finds the user. is_shared marks the one shared user
--     «slouchcat-gaeste» Slouchcat creates itself (N-101): at most one row (partial unique index),
--     and it never belongs to an account (triggers below). There is no setting for it: the flag is
--     the single place.
--   * wish_account_map: at most one Seerr user per account and one account per Seerr user (UNIQUE).
--     No row = not opened. state 'waiting' = opening was triggered, Seerr not reached yet, no user.
--     origin created/linked/manual (opened by Slouchcat, linked by the same email, chosen by hand).
--     linked_by is required for origin 'linked' and may already stand on a waiting row (the trigger
--     of a later link, «invite» survives the wait). password_state 'off' = not synced for this
--     account (F-24, N-103: a user an administrator linked keeps its password). Never a password,
--     never a hash.
--   * There is no wish_profile_map: profiles always send as the user of their account (F-27, N-100).
--   * wish_account_limits: the limit of an account when it deviates from the default; no row =
--     «Standard».
--   * seerr_log: the last 20 calls to Seerr (method, path without query, status, duration, error
--     without key or body), kept to 20 rows by a trigger like webhook_events.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE seerr_users (
    seerr_id           INTEGER PRIMARY KEY CHECK (seerr_id >= 1),
    display_name       TEXT    NOT NULL CHECK (length(display_name) BETWEEN 1 AND 200),
    email              TEXT    COLLATE NOCASE,
    user_type          INTEGER NOT NULL CHECK (user_type IN (1, 2, 3, 4)),
    permissions        INTEGER NOT NULL CHECK (permissions >= 0),
    movie_quota_limit  INTEGER CHECK (movie_quota_limit IS NULL OR movie_quota_limit >= 0),
    movie_quota_days   INTEGER CHECK (movie_quota_days IS NULL OR movie_quota_days >= 0),
    tv_quota_limit     INTEGER CHECK (tv_quota_limit IS NULL OR tv_quota_limit >= 0),
    tv_quota_days      INTEGER CHECK (tv_quota_days IS NULL OR tv_quota_days >= 0),
    movie_quota_used   INTEGER CHECK (movie_quota_used IS NULL OR movie_quota_used >= 0),
    tv_quota_used      INTEGER CHECK (tv_quota_used IS NULL OR tv_quota_used >= 0),
    movie_next_free_at INTEGER,
    tv_next_free_at    INTEGER,
    quota_at           INTEGER,
    is_admin           INTEGER NOT NULL DEFAULT 0 CHECK (is_admin IN (0, 1)),
    is_shared          INTEGER NOT NULL DEFAULT 0 CHECK (is_shared IN (0, 1)),
    created_at         INTEGER,
    fetched_at         INTEGER NOT NULL,
    gone_at            INTEGER
) STRICT;
CREATE INDEX seerr_users_email ON seerr_users (email);
CREATE UNIQUE INDEX seerr_users_shared ON seerr_users (is_shared) WHERE is_shared = 1;

CREATE TABLE wish_account_map (
    account_id         INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    seerr_user_id      INTEGER UNIQUE REFERENCES seerr_users(seerr_id),
    origin             TEXT    CHECK (origin IS NULL OR origin IN ('created', 'linked', 'manual')),
    state              TEXT    NOT NULL DEFAULT 'ready' CHECK (state IN ('ready', 'waiting')),
    linked_by          TEXT    CHECK (linked_by IS NULL OR linked_by IN ('admin', 'invite', 'email_change')),
    linked_at          INTEGER,
    password_state     TEXT    NOT NULL DEFAULT 'next_login'
                               CHECK (password_state IN ('synced', 'next_login', 'off', 'admin', 'external')),
    password_synced_at INTEGER,
    provision_error    TEXT    CHECK (provision_error IS NULL OR length(provision_error) <= 500),
    limit_written_at   INTEGER,
    set_at             INTEGER NOT NULL,
    set_by             INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    CHECK ((state = 'waiting') = (seerr_user_id IS NULL)),
    CHECK (state = 'waiting' OR origin IS NOT NULL),
    CHECK (origin IS NOT 'linked' OR (linked_by IS NOT NULL AND linked_at IS NOT NULL)),
    CHECK (origin IS NULL OR origin = 'linked' OR (linked_by IS NULL AND linked_at IS NULL))
) STRICT;

-- The shared user never belongs to an account (N-101), checked by the database as a second line
-- behind package wishes (data model 2.2). The message is the problem code the service answers with.
-- +goose StatementBegin
CREATE TRIGGER wish_account_map_shared_insert BEFORE INSERT ON wish_account_map
WHEN NEW.seerr_user_id IN (SELECT seerr_id FROM seerr_users WHERE is_shared = 1)
BEGIN
    SELECT RAISE(ABORT, 'seerr_user_taken');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER wish_account_map_shared_update BEFORE UPDATE OF seerr_user_id ON wish_account_map
WHEN NEW.seerr_user_id IN (SELECT seerr_id FROM seerr_users WHERE is_shared = 1)
BEGIN
    SELECT RAISE(ABORT, 'seerr_user_taken');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER seerr_users_shared_mapped BEFORE UPDATE OF is_shared ON seerr_users
WHEN NEW.is_shared = 1 AND EXISTS (SELECT 1 FROM wish_account_map WHERE seerr_user_id = NEW.seerr_id)
BEGIN
    SELECT RAISE(ABORT, 'seerr_user_taken');
END;
-- +goose StatementEnd

CREATE TABLE wish_account_limits (
    account_id  INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    mode        TEXT    NOT NULL CHECK (mode IN ('custom', 'unlimited')),
    movie_limit INTEGER CHECK (movie_limit IS NULL OR movie_limit BETWEEN 0 AND 100),
    movie_days  INTEGER CHECK (movie_days IS NULL OR movie_days BETWEEN 1 AND 90),
    tv_limit    INTEGER CHECK (tv_limit IS NULL OR tv_limit BETWEEN 0 AND 100),
    tv_days     INTEGER CHECK (tv_days IS NULL OR tv_days BETWEEN 1 AND 90),
    set_at      INTEGER NOT NULL,
    set_by      INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    CHECK (mode <> 'unlimited' OR (movie_limit IS NULL AND movie_days IS NULL AND tv_limit IS NULL AND tv_days IS NULL)),
    CHECK (mode <> 'custom' OR (movie_limit IS NOT NULL AND movie_days IS NOT NULL AND tv_limit IS NOT NULL AND tv_days IS NOT NULL))
) STRICT;

CREATE TABLE seerr_log (
    id          INTEGER PRIMARY KEY,
    at          INTEGER NOT NULL,
    method      TEXT    NOT NULL CHECK (length(method) BETWEEN 1 AND 10),
    path        TEXT    NOT NULL CHECK (length(path) BETWEEN 1 AND 300),
    status      INTEGER,
    duration_ms INTEGER NOT NULL CHECK (duration_ms >= 0),
    error       TEXT    CHECK (error IS NULL OR length(error) <= 500)
) STRICT;

-- Keep the newest 20 rows (Anhang F 4: «letzte 20 Aufrufe und Fehler»).
-- +goose StatementBegin
CREATE TRIGGER seerr_log_trim AFTER INSERT ON seerr_log
BEGIN
    DELETE FROM seerr_log
    WHERE id <= (SELECT id FROM seerr_log ORDER BY id DESC LIMIT 1 OFFSET 20);
END;
-- +goose StatementEnd

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
