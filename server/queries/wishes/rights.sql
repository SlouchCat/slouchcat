-- The request right of an account and a profile (concept 25, data model 2.3; package wishes reads
-- it, packages auth and admin write it through store/appq).

-- name: GetProfileRequestRight :one
-- Both switches, both ratings (for the reason age_restricted, ADR 0045) and the account's mapping
-- for one profile of one account; no row when the profile does not belong to the account.
SELECT a.can_request AS account_can_request, p.can_request AS profile_can_request,
       a.max_rating AS account_max_rating, p.max_rating AS profile_max_rating,
       m.seerr_user_id, m.state AS mapping_state
FROM profiles AS p
JOIN accounts AS a ON a.id = p.account_id
LEFT JOIN wish_account_map AS m ON m.account_id = a.id
WHERE p.id = @profile_id AND a.id = @account_id;
