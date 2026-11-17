-- Concept 25 stage 2 (06-benachrichtigungen.md section 6, F-14): every existing channel row of
-- every account chooses the event wish_available, which has its producer from now on (plan step
-- 2.6). It is appended to the end of the list, the place the settings sort it to; a row that has
-- it already stays. Only the lists change, never enabled, so the event reaches an account only on
-- a channel it has switched on. New accounts get it from ChannelEnsureDefaults.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

UPDATE account_channels
SET events = json_insert(events, '$[#]', 'wish_available')
WHERE NOT EXISTS (SELECT 1 FROM json_each(account_channels.events) AS e WHERE e.value = 'wish_available');

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
