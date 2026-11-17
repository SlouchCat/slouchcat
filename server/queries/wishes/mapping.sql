-- The Seerr user of each account (concept 25, data model 2.2; package wishes only, store/wishq).
-- No row = not opened. Times are Unix milliseconds. Never a password, never a hash of one.

-- name: GetAccountMapping :one
SELECT * FROM wish_account_map WHERE account_id = @account_id;

-- name: GetMappingBySeerrUser :one
SELECT * FROM wish_account_map WHERE seerr_user_id = @seerr_user_id;

-- name: ListAccountMappings :many
-- Every account with its mapping (columns NULL without one) and the Seerr user, by account id.
SELECT a.id AS account_id, a.email AS account_email, a.role AS account_role, a.is_owner AS account_is_owner,
       a.can_request AS account_can_request,
       m.seerr_user_id, m.origin, m.state, m.linked_by, m.linked_at, m.password_state,
       m.password_synced_at, m.provision_error, m.limit_written_at, m.set_at, m.set_by
FROM accounts AS a
LEFT JOIN wish_account_map AS m ON m.account_id = a.id
ORDER BY a.id;

-- name: SetManualMapping :exec
-- "Anderen Benutzer waehlen" (users and profiles 6, rule 5): the administrator chooses an existing
-- user. password_state is 'off', 'admin' or 'external' (F-24, N-103, N-6), chosen by the service.
INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, linked_by, linked_at,
                              password_state, password_synced_at, provision_error, limit_written_at,
                              set_at, set_by)
VALUES (@account_id, @seerr_user_id, 'manual', 'ready', NULL, NULL, @password_state, NULL, NULL, NULL,
        @now, @set_by)
ON CONFLICT (account_id) DO UPDATE SET
    seerr_user_id = excluded.seerr_user_id, origin = 'manual', state = 'ready', linked_by = NULL,
    linked_at = NULL, password_state = excluded.password_state, password_synced_at = NULL,
    provision_error = NULL, limit_written_at = NULL, set_at = excluded.set_at, set_by = excluded.set_by;

-- name: SetWaitingMapping :exec
-- Opening is triggered and Seerr is not reached yet (state "wartet"); linked_by keeps who
-- triggered it (an invitation stays "invite" when the later run links). An existing ready row is
-- left alone.
INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, linked_by, linked_at,
                              password_state, provision_error, set_at, set_by)
VALUES (@account_id, NULL, NULL, 'waiting', @linked_by, NULL, 'next_login', @provision_error, @now, @set_by)
ON CONFLICT (account_id) DO UPDATE SET
    linked_by = excluded.linked_by, provision_error = excluded.provision_error, set_at = excluded.set_at,
    set_by = excluded.set_by
WHERE wish_account_map.state = 'waiting';

-- name: SetReadyMapping :exec
-- Opened ('created') or linked by the same email or a 409 ('linked', with linked_by and linked_at)
-- by the provisioning (build step 1.5a).
INSERT INTO wish_account_map (account_id, seerr_user_id, origin, state, linked_by, linked_at,
                              password_state, password_synced_at, provision_error, limit_written_at,
                              set_at, set_by)
VALUES (@account_id, @seerr_user_id, @origin, 'ready', @linked_by, @linked_at, @password_state,
        @password_synced_at, NULL, NULL, @now, @set_by)
ON CONFLICT (account_id) DO UPDATE SET
    seerr_user_id = excluded.seerr_user_id, origin = excluded.origin, state = 'ready',
    linked_by = excluded.linked_by, linked_at = excluded.linked_at,
    password_state = excluded.password_state, password_synced_at = excluded.password_synced_at,
    provision_error = NULL, limit_written_at = NULL, set_at = excluded.set_at, set_by = excluded.set_by;

-- name: SetMappingPasswordState :execrows
-- The password state of a ready mapping (build steps 1.5b and setSeerrPasswordMode); synced_at is
-- the time of the last successful sync, kept when NULL is passed.
UPDATE wish_account_map SET
    password_state = @password_state,
    password_synced_at = coalesce(@password_synced_at, password_synced_at)
WHERE account_id = @account_id AND state = 'ready';

-- name: SetMappingProvisionError :exec
UPDATE wish_account_map SET provision_error = @provision_error WHERE account_id = @account_id;

-- name: SetMappingLimitWritten :exec
UPDATE wish_account_map SET limit_written_at = @limit_written_at WHERE account_id = @account_id AND state = 'ready';

-- name: DeleteAccountMapping :execrows
-- "Verknuepfung loesen" (users and profiles 6, rule 7): only the row goes, the user stays in Seerr.
DELETE FROM wish_account_map WHERE account_id = @account_id;

-- name: DeleteAllAccountMappings :exec
-- "Zuordnung loeschen" after an instance change (architecture 7.4).
DELETE FROM wish_account_map;

-- name: CountUnmappedRequesters :one
-- Accounts that may request but have no ready Seerr user (the counter "unmapped" of the status).
SELECT count(*) FROM accounts AS a
WHERE a.can_request = 1
  AND NOT EXISTS (SELECT 1 FROM wish_account_map AS m WHERE m.account_id = a.id AND m.state = 'ready');

-- name: AccountExists :one
SELECT count(*) FROM accounts WHERE id = @account_id;
