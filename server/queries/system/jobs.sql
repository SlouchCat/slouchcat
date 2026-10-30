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
    last_error = 'lease expired after the last attempt', updated_at = @now, finished_at = @now
WHERE state = 'running' AND lease_until < @now AND attempts >= max_attempts;

-- Expired leases of jobs that may still run go back to the queue (the attempt stays counted), so
-- the claim below reads only queued rows in the order of the index jobs_claim.
-- name: RequeueExpiredLeases :execrows
UPDATE jobs
SET state = 'queued', lease_owner = NULL, lease_until = NULL, updated_at = @now
WHERE state = 'running' AND lease_until < @now AND attempts < max_attempts;

-- name: ClaimJob :one
-- Every attempt starts the run anew: started_at is set, the progress and skipped of the attempt
-- before are cleared (concept 09, 4.1). A paused type (job_types.paused_at) is not claimed; the
-- subquery reads at most a few dozen rows.
UPDATE jobs
SET state = 'running', lease_owner = @worker_id, lease_until = @lease_until,
    attempts = attempts + 1, updated_at = @now, started_at = @now, finished_at = NULL,
    progress_done = NULL, progress_total = NULL, progress_note = NULL, skipped = 0
WHERE id = (
    SELECT j.id FROM jobs AS j
    WHERE j.state = 'queued' AND j.run_after <= @now AND j.type IN (sqlc.slice('types'))
      AND j.type NOT IN (SELECT t.type FROM job_types AS t WHERE t.paused_at IS NOT NULL)
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
SET state = 'done', lease_owner = NULL, lease_until = NULL, updated_at = @now, finished_at = @now
WHERE id = @id AND state = 'running' AND lease_owner = @worker_id;

-- name: FailJob :execrows
-- finished_at is set with the final state only; a retry has not finished (the duration of a run
-- is finished_at - started_at of its last attempt, concept 09 4.1).
UPDATE jobs
SET state = @state, run_after = @run_after, last_error = @last_error,
    lease_owner = NULL, lease_until = NULL, updated_at = @now,
    finished_at = CASE WHEN @state = 'failed' THEN @now ELSE NULL END
WHERE id = @id AND state = 'running' AND lease_owner = @worker_id;

-- name: UpdateJobProgress :execrows
-- The reporter of the worker writes the progress of a running job under its own lease.
UPDATE jobs
SET progress_done = @progress_done, progress_total = @progress_total, progress_note = @progress_note, updated_at = @now
WHERE id = @id AND state = 'running' AND lease_owner = @worker_id;

-- name: SetJobSkipped :execrows
-- jobs.Skip: the run had nothing to do and does not count for the median duration.
UPDATE jobs SET skipped = 1, updated_at = @now
WHERE id = @id AND state = 'running' AND lease_owner = @worker_id;

-- name: ReleaseJob :execrows
UPDATE jobs
SET state = 'queued', attempts = attempts - 1, lease_owner = NULL, lease_until = NULL, updated_at = @now
WHERE id = @id AND state = 'running' AND lease_owner = @worker_id;

-- name: CancelJob :execrows
-- A queued or running job stops; a failed one is dismissed (the dashboard offers both).
UPDATE jobs
SET state = 'cancelled', lease_owner = NULL, lease_until = NULL, updated_at = @now, finished_at = @now
WHERE id = @id AND state IN ('queued', 'running', 'failed');

-- name: ListActiveJobsByType :many
-- The queued and running jobs of a type, oldest first (cancelJobRuns, concept 09 6.3).
SELECT * FROM jobs WHERE type = @type AND state IN ('queued', 'running') ORDER BY id;

-- name: ListJobDurations :many
-- The durations of the newest done runs of a type that did work (the median of concept 09 6.2),
-- over the index jobs_history.
SELECT finished_at - started_at AS duration_ms
FROM jobs
WHERE type = @type AND state = 'done' AND skipped = 0 AND started_at IS NOT NULL AND finished_at IS NOT NULL
ORDER BY finished_at DESC
LIMIT @limit_count;

-- name: ListHistoryJobTypes :many
-- The types that have finished runs to prune.
SELECT DISTINCT type FROM jobs WHERE state IN ('done', 'cancelled') ORDER BY type;

-- name: PruneJobs :execrows
-- One batch of jobs.prune (concept 09 4.4, F-8): done and cancelled runs of a type older than
-- @before, except the newest @keep of the type; failed runs stay until the administrator acts.
-- Rows from before migration 00027 have no finished_at and age by updated_at.
DELETE FROM jobs WHERE id IN (
    SELECT j.id FROM jobs AS j
    WHERE j.type = @type AND j.state IN ('done', 'cancelled') AND COALESCE(j.finished_at, j.updated_at) < @before
      AND j.id NOT IN (
          SELECT k.id FROM jobs AS k WHERE k.type = @type AND k.state IN ('done', 'cancelled')
          ORDER BY COALESCE(k.finished_at, k.updated_at) DESC, k.id DESC
          LIMIT @keep)
    ORDER BY j.id
    LIMIT @batch
);

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

-- name: PauseJobType :execrows
-- The pause of a type (concept 09 6.3, F-1, F-6): no new claims, running runs are stopped by the
-- registry and go back to the queue. Idempotent: a paused type stays paused at its first time.
UPDATE job_types SET updated_at = @now, paused_at = @now WHERE type = @type AND paused_at IS NULL;

-- name: ResumeJobType :execrows
UPDATE job_types SET paused_at = NULL, updated_at = @now WHERE type = @type AND paused_at IS NOT NULL;

-- name: ListPausedJobTypes :many
SELECT type FROM job_types WHERE paused_at IS NOT NULL ORDER BY type;

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

-- ---------------------------------------------------------------------------------------------
-- The page "Hintergrundjobs" (concept 09, step 3): counts per type, running and last runs, the
-- runs of a type by state. Times are Unix milliseconds.
-- ---------------------------------------------------------------------------------------------

-- name: JobCountsByType :many
-- Per type: waiting runs (due), planned runs (start in the future), running and failed runs, and
-- the lowest priority among the waiting ones (the waiting time cumulates over the pool from it).
SELECT type,
       CAST(COALESCE(SUM(state = 'queued' AND run_after <= @now), 0) AS INTEGER) AS queued,
       CAST(COALESCE(SUM(state = 'queued' AND run_after > @now), 0) AS INTEGER)  AS planned,
       CAST(COALESCE(SUM(state = 'running'), 0) AS INTEGER)                      AS running,
       CAST(COALESCE(SUM(state = 'failed'), 0) AS INTEGER)                       AS failed,
       CAST(COALESCE(MIN(CASE WHEN state = 'queued' THEN priority END), 0) AS INTEGER) AS min_queued_priority
FROM jobs
WHERE state IN ('queued', 'running', 'failed')
GROUP BY type;

-- name: ListRunningJobsByType :many
SELECT * FROM jobs WHERE type = @type AND state = 'running' ORDER BY started_at, id LIMIT @limit_count;

-- name: GetLastFinishedJobByType :one
-- The newest run of a type in a final state (done, failed, cancelled), over the index jobs_history.
SELECT * FROM jobs
WHERE type = @type AND state IN ('done', 'failed', 'cancelled')
ORDER BY COALESCE(finished_at, updated_at) DESC, id DESC
LIMIT 1;

-- name: ListQueuedJobRuns :many
-- The waiting runs of a type in claim order (priority, then age); @planned 1 lists those whose
-- start lies in the future, 0 those that are due.
SELECT * FROM jobs
WHERE type = @type AND state = 'queued' AND (run_after > @now) = @planned
ORDER BY priority DESC, created_at, id
LIMIT @limit_count;

-- name: CountQueuedJobRuns :one
SELECT count(*) FROM jobs WHERE type = @type AND state = 'queued' AND (run_after > @now) = @planned;

-- name: ListJobRunsByState :many
-- The runs of a type in one state (running, done, failed, cancelled), newest first.
SELECT * FROM jobs
WHERE type = @type AND state = @state
ORDER BY COALESCE(finished_at, started_at, updated_at) DESC, id DESC
LIMIT @limit_count;

-- name: CountJobRunsByState :one
SELECT count(*) FROM jobs WHERE type = @type AND state = @state;

-- name: CountPausedJobTypes :one
SELECT count(*) FROM job_types WHERE paused_at IS NOT NULL;
