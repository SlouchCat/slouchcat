-- Progress events of a playback session (K-5, M4 review). Times are Unix milliseconds (UTC).
--
--   progress_events  playback_session_id names the playback session an event belongs to (the
--                    contract's ProgressEvent.playbackSessionId; NULL for clients that send none).
--                    No foreign key: playback sessions are deleted a day after they ended, the
--                    events stay for the watch-time report. The index serves the duplicate check
--                    (same playback session and seq) and the previous event of a playback, which
--                    bounds the played time of the next one.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE progress_events ADD COLUMN playback_session_id TEXT;
CREATE INDEX progress_events_playback ON progress_events (playback_session_id, seq);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
