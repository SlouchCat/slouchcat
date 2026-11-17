-- The take-over of requests made in Seerr (concept 25 stage 4, build step 4.6, F-21; architecture
-- 7.3, users and profiles 8.2). Package wishes only (store/wishq). Times are Unix milliseconds.

-- name: GetPrimaryProfileOfAccount :one
-- The main profile of an account: a taken-over request belongs to it (F-27, no user per profile).
SELECT id, account_id, language, metadata_language FROM profiles
WHERE account_id = @account_id AND is_default = 1
ORDER BY id LIMIT 1;

-- name: ListReadyMappingsForImport :many
-- Every account with a ready Seerr user and the time the user was set: a new time asks for a new
-- take-over of the user's requests.
SELECT account_id, seerr_user_id, set_at FROM wish_account_map WHERE state = 'ready' ORDER BY account_id;
