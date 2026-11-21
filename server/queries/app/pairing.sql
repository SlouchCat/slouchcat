-- Pairing vouchers of the TV set-up (package store/appq, concept 27 section 10, ADR 0040). Owner:
-- auth. Times are Unix milliseconds; voucher_hash is the hex SHA-256 of the voucher.

-- name: UpsertPairingVoucher :exec
-- The one open voucher of an account: a new one replaces the older.
INSERT INTO pairing_vouchers (account_id, voucher_hash, tv_key, device_kind, created_at, expires_at)
VALUES (@account_id, @voucher_hash, @tv_key, @device_kind, @now, @expires_at)
ON CONFLICT (account_id) DO UPDATE
SET voucher_hash = excluded.voucher_hash, tv_key = excluded.tv_key, device_kind = excluded.device_kind,
    created_at = excluded.created_at, expires_at = excluded.expires_at;

-- name: TakePairingVoucher :one
-- Uses the voucher up: the row is read and deleted in one statement, so of two concurrent redeems
-- only one gets it. Expiry and key are checked by the caller on the returned row (a voucher shown
-- with another key is used up too).
DELETE FROM pairing_vouchers WHERE voucher_hash = @voucher_hash
RETURNING account_id, tv_key, device_kind, expires_at;

-- name: DeleteExpiredPairingVouchers :execrows
DELETE FROM pairing_vouchers WHERE expires_at <= @now;
