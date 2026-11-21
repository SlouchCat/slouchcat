-- Concept 27 stage 1 (ADR 0040, «Kopplung»; K26 4.6 K-3): the voucher a phone app issues to pair a
-- TV. The server keeps only the SHA-256 of the voucher (hex), the TV's one-time X25519 key it is
-- bound to (base64url) and the kind of the device it pairs (only 'tv' so far). One row per
-- account: a new voucher replaces the open one (primary key account_id). Redeeming deletes the row
-- in the same statement that reads it, so a voucher works once also under concurrent requests; an
-- expired row is removed by the session cleanup.
--
-- A new table only, nothing is rebuilt; the frame of the template stays.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE pairing_vouchers (
    account_id   INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    voucher_hash TEXT    NOT NULL UNIQUE CHECK (length(voucher_hash) = 64),
    tv_key       TEXT    NOT NULL CHECK (length(tv_key) = 43),
    device_kind  TEXT    NOT NULL CHECK (device_kind IN ('tv')),
    created_at   INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL
) STRICT;

CREATE INDEX pairing_vouchers_expiry ON pairing_vouchers (expires_at);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
