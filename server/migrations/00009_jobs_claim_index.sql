-- Job claim index (plan 3.6, M3 load test). With tens of thousands of queued probe jobs the claim
-- sorted every candidate row, because the old index led with the job type and the claim reads
-- every handled type at once. The new index follows the claim's order (queued jobs by priority,
-- then age), so a claim reads the first matching row and stops.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

DROP INDEX jobs_claim;
CREATE INDEX jobs_claim ON jobs (state, priority DESC, created_at, id);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
