-- Offline playback (M5, plan 3.3): which profiles of an account may see a title. The server stores
-- the answer in downloads.allowed_profile_ids; the client only looks it up (plan principle 1).

-- Profiles of the account that see the titles, by title and profile id. Read by
-- visibility.AllowedProfiles, which accepts the System scope only.
-- name: ListAllowedProfiles :many
SELECT v.item_id, v.profile_id
FROM visible_items AS v
JOIN profiles AS p ON p.id = v.profile_id
WHERE p.account_id = @account_id AND v.item_id IN (sqlc.slice('item_ids'))
ORDER BY v.item_id, v.profile_id;
