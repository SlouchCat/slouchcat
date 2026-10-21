-- Accounts as the administration and the account owner see them (M8, A-2; packages auth and
-- admin). Times are Unix milliseconds.

-- name: ListAccountsAdmin :many
-- The main account first, then the guests, newest first.
SELECT id, email, role, max_rating, all_libraries, created_at, last_seen_at
FROM accounts
ORDER BY role = 'guest', created_at DESC, id DESC;

-- name: ListProfilesAdmin :many
-- Every profile of every account, the primary profile of an account first.
SELECT id, account_id, name, max_rating, is_default
FROM profiles
ORDER BY account_id, is_default DESC, id;

-- name: ListAccountLibrariesAll :many
-- Library rights of every account with all_libraries = 0.
SELECT account_id, library_id FROM account_libraries ORDER BY account_id, library_id;

-- name: UpdateAccountEmail :execrows
UPDATE accounts SET email = @email, updated_at = @now WHERE id = @id;

-- name: UpdateAccountPassword :execrows
UPDATE accounts SET password_hash = @password_hash, updated_at = @now WHERE id = @id;

-- name: RevokeOtherAccountSessions :execrows
-- A new password ends every other session of the account; the calling one stays.
UPDATE sessions SET revoked_at = @now WHERE account_id = @account_id AND id <> @keep_id AND revoked_at IS NULL;
