-- Fast playback start (concept 14 step 3a, sections 2.1, 4.1, 4.3 to 4.5). Schema only; nothing
-- reads or writes the new columns yet.
--
--   media_files.keyframes_source  where the keyframe index of this file version came from:
--                                 NULL = no index yet, 'cues' (Matroska cues), 'mp4' (MP4 tables)
--                                 and 'scan' (full read) = index present, 'none' = impossible
--                                 (the full read found no keyframe, or the file is broken).
--   media_files_keyframes         partial index that carries the coverage count and the search of
--                                 the backfill job; it lists only files worth indexing (present,
--                                 a codec that can be remuxed). The condition on items.kind comes
--                                 through the join. The id behind the source orders the cursor of
--                                 the backfill, so a batch needs no sort.
--   jobs.override_pause           1 = the run may start while background work is paused (set only
--                                 for playback requests, never by default).
--   jobs_override_pause           partial index that carries the cap count of the playback
--                                 requests (at most three queued or running, O-7): it lists only the
--                                 few rows with override_pause, not the queued backlog.
--   job_schedules.once_at         one-off date in ms UTC, NULL = none.
--
-- The backfill marks every blob with at least one entry as 'scan': until now only the full read
-- wrote the index. A blob that holds only the version byte stays NULL (audio files and the files
-- that cannot be remuxed are not worth indexing). No blob is deleted or recomputed. ADD COLUMN
-- works on the STRICT tables media_files, jobs and job_schedules, so no table is rebuilt.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE media_files ADD COLUMN keyframes_source TEXT CHECK (keyframes_source IN ('cues', 'mp4', 'scan', 'none'));

UPDATE media_files SET keyframes_source = 'scan' WHERE length(keyframes) > 1;

CREATE INDEX media_files_keyframes ON media_files (keyframes_source, id)
    WHERE missing_since IS NULL AND video_codec IN ('h264', 'hevc', 'av1', 'vp9');

ALTER TABLE jobs ADD COLUMN override_pause INTEGER NOT NULL DEFAULT 0 CHECK (override_pause IN (0, 1));

CREATE INDEX jobs_override_pause ON jobs (state) WHERE override_pause = 1;

ALTER TABLE job_schedules ADD COLUMN once_at INTEGER;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
