-- Concept 25 stage 4 (06-benachrichtigungen.md section 6, F-14): every existing channel row of an
-- administrator account chooses the event wish_review, which has its producer from now on (plan
-- step 4.3). Like errors it is the administrators' event (ADR 0009), so no other account gets it.
-- It is appended to the end of the list, the place the settings sort it to; a row that has it
-- already stays. Only the lists change, never enabled. wish_status (off by default) is added
-- nowhere. New administrator accounts get it from ChannelEnsureDefaults.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

UPDATE account_channels
SET events = json_insert(events, '$[#]', 'wish_review')
WHERE account_id IN (SELECT id FROM accounts WHERE role = 'admin')
  AND NOT EXISTS (SELECT 1 FROM json_each(account_channels.events) AS e WHERE e.value = 'wish_review');

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
