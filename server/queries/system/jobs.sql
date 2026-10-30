-- Unfiltered queries of the job queue (package store/systemq). Times are Unix milliseconds.

-- name: InsertJob :one
INSERT INTO jobs (type, priority, idempotency_key, payload, run_after, max_attempts, created_at, updated_at)
VALUES (@type, @priority, @idempotency_key, @payload, @run_after, @max_attempts, @now, @now)
ON CONFLICT (idempotency_key) WHERE state IN ('queued', 'running') DO NOTHING
RETURNING id;

-- name: GetActiveJobIDByKey :one
SELECT id FROM jobs
WHERE idempotency_key = @idempotency_key AND state IN ('queued', 'running');

-- name: GetActiveJobIDByType :one
-- Any queued or running job of a type (a job that continues itself under changing keys, such as
-- metadata.vote_backfill).
SELECT id FROM jobs
WHERE type = @type AND state IN ('queued', 'running')
ORDER BY id
LIMIT 1;

-- name: SetQueuedJobPayload :execrows
-- Replaces the payload of a job that no worker has claimed yet (coalescing of scan.path).
UPDATE jobs SET payload = @payload, updated_at = @now WHERE id = @id AND state = 'queued';

-- name: RaiseQueuedJobPriority :execrows
-- Raises the priority of a job that no worker has claimed yet and never lowers it: a manual
-- spotlight refresh (M11) overtakes the nightly one that already waits behind a scan backlog.
UPDATE jobs SET priority = max(priority, @priority), updated_at = @now WHERE id = @id AND state = 'queued';

-- name: GetJob :one
SELECT * FROM jobs WHERE id = @id;

-- name: FailExhaustedLeases :execrows
UPDATE jobs
SET state = 'failed', lease_owner = NULL, lease_until = NULL,
    last_error = 'lease expired after the last attempt', updated_at = @now
WHERE state = 'running' AND lease_until < @now AND attempts >= max_attempts;

-- Expired leases of jobs that may still run go back to the queue (the attempt stays counted), so
-- the claim below reads only queued rows in the order of the index jobs_claim.
-- name: RequeueExpiredLeases :execrows
UPDATE jobs
SET state = 'queued', lease_owner = NULL, lease_until = NULL, updated_at = @now
WHERE state = 'running' AND lease_until < @now AND attempts < max_attempts;

-- name: ClaimJob :one
UPDATE jobs
SET state = 'running', lease_owner = @worker_id, lease_until = @lease_until,
    attempts = attempts + 1, updated_at = @now
WHERE id = (
    SELECT j.id FROM jobs AS j
    WHERE j.state = 'queued' AND j.run_after <= @now AND j.type IN (sqlc.slice('types'))
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
-- A queued or running job stops; a failed one is dismissed (the dashboard offers both).
UPDATE jobs
SET state = 'cancelled', lease_owner = NULL, lease_until = NULL, updated_at = @now
WHERE id = @id AND state IN ('queued', 'running', 'failed');

-- ---------------------------------------------------------------------------------------------
-- Job types and schedules (concept 09, ADR 0017). Times are Unix milliseconds; the window is in
-- minutes of the server's day.
-- ---------------------------------------------------------------------------------------------

-- name: EnsureJobType :exec
-- The registry writes one row per registered type at startup; an existing row (and its pause)
-- stays as it is.
INSERT INTO job_types (type, updated_at) VALUES (@type, @now)
ON CONFLICT (type) DO NOTHING;

-- name: GetJobType :one
SELECT * FROM job_types WHERE type = @type;

-- name: ListJobTypes :many
SELECT * FROM job_types ORDER BY type;

-- name: InsertJobSchedule :execrows
-- EnsureSchedule creates a missing row only; a row that exists keeps its interval, switch, window
-- and next run (the table is the source once it has a row, the code only the default).
INSERT INTO job_schedules (type, every_ms, start_delay_ms, enabled, window_start, window_end, next_run_at, updated_at)
VALUES (@type, @every_ms, @start_delay_ms, @enabled, @window_start, @window_end, @next_run_at, @now)
ON CONFLICT (type) DO NOTHING;

-- name: GetJobSchedule :one
SELECT * FROM job_schedules WHERE type = @type;

-- name: ListJobSchedules :many
SELECT * FROM job_schedules ORDER BY type;

-- name: ListDueJobSchedules :many
-- The tick of the scheduler: every enabled schedule whose next run has come, with the pause of
-- its type (a paused type is not run, but its next run moves on).
SELECT s.*, t.paused_at
FROM job_schedules AS s
JOIN job_types AS t ON t.type = s.type
WHERE s.enabled = 1 AND s.next_run_at <= @now
ORDER BY s.type;

-- name: SetJobScheduleNextRun :execrows
-- The start rule (start_delay_ms) and the alignment to the window move the next run only.
UPDATE job_schedules SET next_run_at = @next_run_at, updated_at = @now WHERE type = @type;

-- name: MarkJobScheduleFired :execrows
UPDATE job_schedules SET next_run_at = @next_run_at, updated_at = @now, last_fired_at = @now WHERE type = @type;

-- name: UpdateJobSchedule :execrows
-- UpdateSchedule writes interval, switch, window and the realigned next run together.
UPDATE job_schedules
SET every_ms = @every_ms, enabled = @enabled, window_start = @window_start, window_end = @window_end,
    next_run_at = @next_run_at, updated_at = @now
WHERE type = @type;
