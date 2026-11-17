-- Wishes through Seerr, stage 1 (concept 25, data model 2.3): the right to request titles on the
-- account (the cap an administrator sets), on the profile (set by the primary profile) and on the
-- invitation (goes to the new account), and the limit an invitation hands to its account.
--
-- ALTER TABLE ... ADD COLUMN with a constant default and a CHECK is enough (data model 2.3, the
-- proposal to check it first): no rebuild of accounts and profiles, so visible_items and the
-- triggers on both tables stay as they are.
--
--   * Effective is accounts.can_request = 1 AND profiles.can_request = 1.
--   * Backfill (F-5, users and profiles 9): the owner account and every primary profile may
--     request; every other account and profile and every open invitation may not. Without a Seerr
--     user the right has no effect.
--   * invitations.wish_limit_mode: 'default' (the default limit of the settings), 'custom' with the
--     four numbers (counts 0 to 100, days 1 to 90, as wish_account_limits) or 'unlimited'. On
--     acceptance it becomes a row of wish_account_limits unless it is 'default'.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE accounts ADD COLUMN can_request INTEGER NOT NULL DEFAULT 0 CHECK (can_request IN (0, 1));
ALTER TABLE profiles ADD COLUMN can_request INTEGER NOT NULL DEFAULT 0 CHECK (can_request IN (0, 1));
ALTER TABLE invitations ADD COLUMN can_request INTEGER NOT NULL DEFAULT 0 CHECK (can_request IN (0, 1));
ALTER TABLE invitations ADD COLUMN wish_limit_mode TEXT NOT NULL DEFAULT 'default'
    CHECK (wish_limit_mode IN ('default', 'custom', 'unlimited'));
ALTER TABLE invitations ADD COLUMN wish_movie_limit INTEGER CHECK (wish_movie_limit IS NULL OR wish_movie_limit BETWEEN 0 AND 100);
ALTER TABLE invitations ADD COLUMN wish_movie_days INTEGER CHECK (wish_movie_days IS NULL OR wish_movie_days BETWEEN 1 AND 90);
ALTER TABLE invitations ADD COLUMN wish_tv_limit INTEGER CHECK (wish_tv_limit IS NULL OR wish_tv_limit BETWEEN 0 AND 100);
-- The last column carries the rule of the four numbers: all of them with 'custom', none otherwise.
ALTER TABLE invitations ADD COLUMN wish_tv_days INTEGER CHECK (
    (wish_tv_days IS NULL OR wish_tv_days BETWEEN 1 AND 90)
    AND (wish_limit_mode <> 'custom' OR (wish_movie_limit IS NOT NULL AND wish_movie_days IS NOT NULL
                                         AND wish_tv_limit IS NOT NULL AND wish_tv_days IS NOT NULL))
    AND (wish_limit_mode = 'custom' OR (wish_movie_limit IS NULL AND wish_movie_days IS NULL
                                        AND wish_tv_limit IS NULL AND wish_tv_days IS NULL)));

UPDATE accounts SET can_request = 1 WHERE is_owner = 1;
UPDATE profiles SET can_request = 1 WHERE is_default = 1;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
