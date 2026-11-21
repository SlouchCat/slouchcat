-- Pairing vouchers of the TV set-up (package store/appq, concept 27 section 10, ADR 0040). Owner:
-- auth. Times are Unix milliseconds; voucher_hash is the hex SHA-256 of the voucher.

-- name: UpsertPairingVoucher :exec
-- The one open voucher of an account: a new one replaces the older. session_id is the app session
-- that issued it.
INSERT INTO pairing_vouchers (account_id, session_id, voucher_hash, tv_key, device_kind, created_at, expires_at)
VALUES (@account_id, @session_id, @voucher_hash, @tv_key, @device_kind, @now, @expires_at)
ON CONFLICT (account_id) DO UPDATE
SET session_id = excluded.session_id, voucher_hash = excluded.voucher_hash, tv_key = excluded.tv_key,
    device_kind = excluded.device_kind, created_at = excluded.created_at, expires_at = excluded.expires_at;

-- name: TakePairingVoucher :one
-- Uses the voucher up: the row is read and deleted in one statement, so of two concurrent redeems
-- only one gets it. Expiry, key and the issuing session (PairingIssuerAlive) are checked by the
-- caller on the returned row (a voucher shown with another key is used up too).
DELETE FROM pairing_vouchers WHERE voucher_hash = @voucher_hash
RETURNING account_id, session_id, tv_key, device_kind, expires_at;

-- name: PairingIssuerAlive :one
-- Whether the session that issued a voucher still lives (not revoked, not expired).
SELECT EXISTS (SELECT 1 FROM sessions WHERE id = @id AND revoked_at IS NULL AND expires_at > @now);

-- name: DeleteAccountPairingVouchers :execrows
-- A new password ends the open voucher of the account (code review M-1).
DELETE FROM pairing_vouchers WHERE account_id = @account_id;

-- name: DeleteExpiredPairingVouchers :execrows
DELETE FROM pairing_vouchers WHERE expires_at <= @now;
