-- Fill bookkeeping (ADR 0015, review of 29 October 2026): items.filled_providers records the
-- provider list the last metadata.fill of a title ran with (the configured providers of its list,
-- as JSON), so the convergence after a scan queues a fill only for titles whose list changed or
-- whose fill never ran, instead of every title whose provenance still names a provider that left
-- the list (a field nobody replaced keeps its old source on purpose). NULL = never filled.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE items ADD COLUMN filled_providers TEXT
    CHECK (filled_providers IS NULL OR json_valid(filled_providers));

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
