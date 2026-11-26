-- The request right follows the age limit (question 36 of 26 November 2026, ADR 0045; replaces F-5
-- of concept 25). Every account and profile without an age limit (max_rating 18) may request, those
-- with one may not.
--
--   * Backfill once: accounts by their own rating, profiles by the lower of their rating and the
--     account's, open invitations by theirs; restricted rows lose a right someone gave them by hand
--     under F-5. The stored setting «Neue Konten dürfen anfordern» goes on.
--   * The Seerr users Slouchcat opened for an account whose right changes here must follow it
--     (N-110): limit_written_at goes back to NULL, so the next users job queues their sync.
--   * Two triggers write can_request whenever max_rating crosses 18 (off below, on at 18), whoever
--     writes the rating; a lowered account ceiling lowers its profiles (ClampProfileRatings), and
--     their trigger takes their right too.
--   * Guards on accounts, profiles and invitations abort a row with can_request = 1 and
--     max_rating < 18, a second line behind package auth (the message is the problem code).
--
-- No rebuild: only rows, settings and triggers change, so visible_items and the triggers of both
-- tables stay as they are.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

UPDATE wish_account_map SET limit_written_at = NULL
WHERE state = 'ready' AND origin = 'created'
  AND account_id IN (SELECT id FROM accounts WHERE can_request <> (max_rating >= 18));

UPDATE accounts SET can_request = (max_rating >= 18) WHERE can_request <> (max_rating >= 18);

UPDATE profiles SET can_request = (
    SELECT min(profiles.max_rating, a.max_rating) >= 18 FROM accounts AS a WHERE a.id = profiles.account_id)
WHERE can_request <> (
    SELECT min(profiles.max_rating, a.max_rating) >= 18 FROM accounts AS a WHERE a.id = profiles.account_id);

UPDATE invitations SET can_request = (max_rating >= 18)
WHERE used_at IS NULL AND revoked_at IS NULL AND can_request <> (max_rating >= 18);
-- A used, revoked or expired invitation creates no account any more, but the guard below holds for
-- every row: those keep their value only where it is allowed.
UPDATE invitations SET can_request = 0 WHERE can_request = 1 AND max_rating < 18;

UPDATE settings SET value = '1' WHERE key = 'wishes.new_accounts_can_request';

-- +goose StatementBegin
CREATE TRIGGER accounts_request_follows_rating
AFTER UPDATE OF max_rating ON accounts
WHEN (OLD.max_rating >= 18) <> (NEW.max_rating >= 18)
BEGIN
    UPDATE accounts SET can_request = (NEW.max_rating >= 18) WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER profiles_request_follows_rating
AFTER UPDATE OF max_rating ON profiles
WHEN (OLD.max_rating >= 18) <> (NEW.max_rating >= 18)
BEGIN
    UPDATE profiles SET can_request = (NEW.max_rating >= 18) WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER accounts_request_needs_no_age_limit_insert
BEFORE INSERT ON accounts
WHEN NEW.can_request = 1 AND NEW.max_rating < 18
BEGIN
    SELECT RAISE(ABORT, 'age_restricted');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER accounts_request_needs_no_age_limit_update
BEFORE UPDATE OF can_request ON accounts
WHEN NEW.can_request = 1 AND NEW.max_rating < 18
BEGIN
    SELECT RAISE(ABORT, 'age_restricted');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER profiles_request_needs_no_age_limit_insert
BEFORE INSERT ON profiles
WHEN NEW.can_request = 1 AND NEW.max_rating < 18
BEGIN
    SELECT RAISE(ABORT, 'age_restricted');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER profiles_request_needs_no_age_limit_update
BEFORE UPDATE OF can_request ON profiles
WHEN NEW.can_request = 1 AND NEW.max_rating < 18
BEGIN
    SELECT RAISE(ABORT, 'age_restricted');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER invitations_request_needs_no_age_limit_insert
BEFORE INSERT ON invitations
WHEN NEW.can_request = 1 AND NEW.max_rating < 18
BEGIN
    SELECT RAISE(ABORT, 'age_restricted');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER invitations_request_needs_no_age_limit_update
BEFORE UPDATE OF can_request ON invitations
WHEN NEW.can_request = 1 AND NEW.max_rating < 18
BEGIN
    SELECT RAISE(ABORT, 'age_restricted');
END;
-- +goose StatementEnd

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
