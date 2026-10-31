-- Scheduler, progress and pause of the job queue (concept 09, ADR 0017, decision 15):
--   * jobs learns the start and the end of a run (started_at set by every claim, finished_at by
--     the final states), a progress (progress_done, progress_total, progress_note, written by the
--     handler through jobs.Progress) and skipped (a run that had nothing to do and does not count
--     for the median duration). All NULL or defaulted, no existing row changes. The index
--     jobs_history carries the median query and the pruning of old runs.
--   * job_types: one row per registered job type, written by the registry at startup; paused_at
--     is the pause of the type (NULL = not paused), the only thing about a type that lives in the
--     database. A row of a type no worker knows any more stays (downgrade) and shows as unknown.
--   * job_schedules is rebuilt: the placeholder of 00001 (type, cron_expr, next_run) has never
--     held a row, so it is dropped and created again with intervals instead of cron expressions
--     (every_ms, 5 min to 7 d), a start delay (NULL = do not fire at start, 0 = at once,
--     60000 = after 60 s), an on/off switch, an optional time window in minutes of the server's
--     day (window_end exclusive; end < start spans midnight), the next and the last firing. The
--     guard below ends the migration before the DROP should the table hold a row after all.
--     The rows are written by the registry at startup (EnsureSchedule), never by the migration.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE jobs ADD COLUMN started_at INTEGER;
ALTER TABLE jobs ADD COLUMN finished_at INTEGER;
ALTER TABLE jobs ADD COLUMN progress_done INTEGER;
ALTER TABLE jobs ADD COLUMN progress_total INTEGER;
ALTER TABLE jobs ADD COLUMN progress_note TEXT;
ALTER TABLE jobs ADD COLUMN skipped INTEGER NOT NULL DEFAULT 0 CHECK (skipped IN (0, 1));

CREATE INDEX jobs_history ON jobs (type, state, finished_at DESC);

CREATE TABLE job_types (
    type       TEXT    PRIMARY KEY,
    paused_at  INTEGER,
    updated_at INTEGER NOT NULL
) STRICT;

-- ---------------------------------------------------------------------------------------------
-- Guard in the same run, before the DROP. A row with n > 0 violates the CHECK and ends the run.
-- ---------------------------------------------------------------------------------------------
CREATE TEMP TABLE migration_00027_guard (n INTEGER NOT NULL CHECK (n = 0));

-- The placeholder table of 00001 has never been filled; a row there would be data this
-- migration cannot carry over (cron_expr has no interval).
INSERT INTO migration_00027_guard (n)
SELECT count(*) FROM job_schedules;

DROP TABLE migration_00027_guard;

DROP TABLE job_schedules;

CREATE TABLE job_schedules (
    type           TEXT    PRIMARY KEY REFERENCES job_types(type),
    every_ms       INTEGER NOT NULL CHECK (every_ms BETWEEN 300000 AND 604800000),
    start_delay_ms INTEGER,
    enabled        INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    window_start   INTEGER,
    window_end     INTEGER,
    next_run_at    INTEGER NOT NULL,
    last_fired_at  INTEGER,
    updated_at     INTEGER NOT NULL,
    CHECK ((window_start IS NULL) = (window_end IS NULL)),
    CHECK (window_start IS NULL OR (window_start BETWEEN 0 AND 1439 AND window_end BETWEEN 0 AND 1439
           AND window_start <> window_end))
) STRICT;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
