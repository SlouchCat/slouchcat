-- Concept 27 stage 1, second review round (ADR 0040, ADR 0044):
--
--   pairing_vouchers  A voucher is bound to the app session that issued it (session_id): redeeming
--                     checks that this session still lives, so logout, «Meine Geräte» and a new
--                     password end an open voucher with the phone (code review M-1, security
--                     review L-3). The table holds vouchers of two minutes only; it is rebuilt
--                     empty, an open voucher of the moment of the update is simply gone.
--   deliveries        pushed_to keeps the push subscriptions a delivery reached (a JSON array of
--                     ids), so a retry goes to the others only (code review M-2): a subscription of
--                     the push relay that answered 503 is tried again without a second message to
--                     a browser that took it already. NULL = none yet.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

DROP TABLE pairing_vouchers;

CREATE TABLE pairing_vouchers (
    account_id   INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    session_id   INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    voucher_hash TEXT    NOT NULL UNIQUE CHECK (length(voucher_hash) = 64),
    tv_key       TEXT    NOT NULL CHECK (length(tv_key) = 43),
    device_kind  TEXT    NOT NULL CHECK (device_kind IN ('tv')),
    created_at   INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL
) STRICT;

CREATE INDEX pairing_vouchers_expiry ON pairing_vouchers (expires_at);
CREATE INDEX pairing_vouchers_session ON pairing_vouchers (session_id);

ALTER TABLE deliveries ADD COLUMN pushed_to TEXT CHECK (pushed_to IS NULL OR json_valid(pushed_to));

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
