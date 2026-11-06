-- Time windows for heavy event jobs (concept 15, section 4.3). Schema only; the claim reads it.
--
--   job_types.window_start  window of an event type that reads whole files (probe.keyframes,
--   job_types.window_end    segments.detect, chapters.images) in minutes of the server's day, like
--                           job_schedules: window_end exclusive, end < start spans midnight, both
--                           NULL = no window (the default). Outside the window no worker claims a
--                           job of the type; a job that runs when the window closes finishes.
--   jobs.override_window    1 = the run may start outside the window of its type (set only for
--                           playback requests, never by default). Unlike override_pause it has no
--                           cap and no index: nothing counts it.
--
-- ADD COLUMN takes no table constraint, so the rule "both or neither" and the range of window_end
-- sit on the column window_end; a column CHECK in SQLite may read the other columns of the row, so
-- the rule is the same as the table CHECK of job_schedules. ADD COLUMN works on the STRICT tables
-- job_types and jobs, so no table is rebuilt; every existing row gets NULL and 0.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE job_types ADD COLUMN window_start INTEGER CHECK (window_start BETWEEN 0 AND 1439);

ALTER TABLE job_types ADD COLUMN window_end INTEGER CHECK ((window_start IS NULL) = (window_end IS NULL)
    AND (window_end IS NULL OR (window_end BETWEEN 0 AND 1439 AND window_end <> window_start)));

ALTER TABLE jobs ADD COLUMN override_window INTEGER NOT NULL DEFAULT 0 CHECK (override_window IN (0, 1));

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
