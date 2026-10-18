-- Unfiltered queries of the job queue (package store/systemq). Times are Unix milliseconds.

-- name: InsertJob :one
INSERT INTO jobs (type, priority, idempotency_key, payload, run_after, max_attempts, created_at, updated_at)
VALUES (@type, @priority, @idempotency_key, @payload, @run_after, @max_attempts, @now, @now)
ON CONFLICT (idempotency_key) WHERE state IN ('queued', 'running') DO NOTHING
RETURNING id;

-- name: GetActiveJobIDByKey :one
SELECT id FROM jobs
WHERE idempotency_key = @idempotency_key AND state IN ('queued', 'running');

-- name: GetJob :one
SELECT * FROM jobs WHERE id = @id;

-- name: FailExhaustedLeases :execrows
UPDATE jobs
SET state = 'failed', lease_owner = NULL, lease_until = NULL,
    last_error = 'lease expired after the last attempt', updated_at = @now
WHERE state = 'running' AND lease_until < @now AND attempts >= max_attempts;

-- name: ClaimJob :one
UPDATE jobs
SET state = 'running', lease_owner = @worker_id, lease_until = @lease_until,
    attempts = attempts + 1, updated_at = @now
WHERE id = (
    SELECT j.id FROM jobs AS j
    WHERE j.type IN (sqlc.slice('types'))
      AND ((j.state = 'queued' AND j.run_after <= @now)
        OR (j.state = 'running' AND j.lease_until < @now AND j.attempts < j.max_attempts))
    ORDER BY j.priority DESC, j.created_at, j.id
    LIMIT 1
)
RETURNING *;

-- name: ExtendLease :execrows
UPDATE jobs
SET lease_until = @lease_until, updated_at = @now
WHERE id = @id AND state = 'running' AND lease_owner = @worker_id;

-- name: CompleteJob :execrows
UPDATE jobs
SET state = 'done', lease_owner = NULL, lease_until = NULL, updated_at = @now
WHERE id = @id AND state = 'running' AND lease_owner = @worker_id;

-- name: FailJob :execrows
UPDATE jobs
SET state = @state, run_after = @run_after, last_error = @last_error,
    lease_owner = NULL, lease_until = NULL, updated_at = @now
WHERE id = @id AND state = 'running' AND lease_owner = @worker_id;

-- name: ReleaseJob :execrows
UPDATE jobs
SET state = 'queued', attempts = attempts - 1, lease_owner = NULL, lease_until = NULL, updated_at = @now
WHERE id = @id AND state = 'running' AND lease_owner = @worker_id;

-- name: CancelJob :execrows
UPDATE jobs
SET state = 'cancelled', lease_owner = NULL, lease_until = NULL, updated_at = @now
WHERE id = @id AND state IN ('queued', 'running');
