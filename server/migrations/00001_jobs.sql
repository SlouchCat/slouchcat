-- Persistent job queue (plan section 3.6). Times are Unix milliseconds (UTC).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;

CREATE TABLE jobs (
    id              INTEGER PRIMARY KEY,
    type            TEXT    NOT NULL,
    priority        INTEGER NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 9),
    idempotency_key TEXT,
    payload         TEXT    NOT NULL DEFAULT '{}' CHECK (json_valid(payload)),
    state           TEXT    NOT NULL DEFAULT 'queued'
                    CHECK (state IN ('queued', 'running', 'done', 'failed', 'cancelled')),
    run_after       INTEGER NOT NULL,
    lease_owner     TEXT,
    lease_until     INTEGER,
    attempts        INTEGER NOT NULL DEFAULT 0,
    max_attempts    INTEGER NOT NULL DEFAULT 5 CHECK (max_attempts >= 1),
    last_error      TEXT,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
) STRICT;

-- An idempotency key is unique among active jobs only, so the same key can be enqueued again
-- once the previous job has finished (debounced rescans, periodic jobs).
CREATE UNIQUE INDEX jobs_active_idempotency_key ON jobs (idempotency_key)
    WHERE state IN ('queued', 'running');

CREATE INDEX jobs_claim ON jobs (state, type, priority DESC, created_at, id);

-- Placeholder for cron-like schedules; the scheduler that fills jobs from it follows later.
CREATE TABLE job_schedules (
    type      TEXT    PRIMARY KEY,
    cron_expr TEXT    NOT NULL,
    next_run  INTEGER NOT NULL
) STRICT;

PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
