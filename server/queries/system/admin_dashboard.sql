-- Job administration of the dashboard (M8, A-3; package jobs, read by admin). Times are Unix
-- milliseconds.

-- name: JobCountsByState :one
-- Queued, running and failed jobs (done and cancelled ones are history, not counted).
SELECT CAST(COALESCE(SUM(state = 'queued'), 0) AS INTEGER)  AS queued,
       CAST(COALESCE(SUM(state = 'running'), 0) AS INTEGER) AS running,
       CAST(COALESCE(SUM(state = 'failed'), 0) AS INTEGER)  AS failed
FROM jobs
WHERE state IN ('queued', 'running', 'failed');

-- name: ListFailedJobs :many
-- The newest failed jobs first.
SELECT id, type, state, attempts, max_attempts, last_error, created_at, updated_at
FROM jobs
WHERE state = 'failed'
ORDER BY updated_at DESC, id DESC
LIMIT @limit_count;

-- name: RetryFailedJob :execrows
-- A failed job goes back to the queue with its attempts reset. Nothing changes when the job is not
-- failed or an active job with the same idempotency key exists (the unique index of active keys).
UPDATE jobs
SET state = 'queued', attempts = 0, run_after = @now, lease_owner = NULL, lease_until = NULL, updated_at = @now
WHERE jobs.id = @id AND jobs.state = 'failed'
  AND (jobs.idempotency_key IS NULL OR NOT EXISTS (
      SELECT 1 FROM jobs AS a WHERE a.idempotency_key = jobs.idempotency_key AND a.state IN ('queued', 'running')));
