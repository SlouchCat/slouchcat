-- Condensed playback history (W-5, plan 3.2 "raw events ... condensed to daily values per profile
-- and item after 90 days", M10). progress_events keeps the raw rows of the last 90 days
-- (playback.ProgressRetention); everything older becomes one row per profile, title, device and UTC
-- day in progress_days, and the raw rows of that day are deleted in the same transaction
-- (server/internal/playback/condense.go). The usage statistics (A-8,
-- queries/system/admin_stats.sql) read the raw events and the condensed days and add them up, so
-- their numbers do not change when a day is condensed. Times are Unix milliseconds (UTC).
--
-- day is the UTC day number, server_time / 86400000 (whole days since the epoch). A day number was
-- chosen over the day's start in milliseconds because it keeps the key column small, is computed
-- straight from server_time in SQL without a date function, and compares as a plain integer range;
-- the day starts at day * 86400000 and ends at (day + 1) * 86400000.
--
-- device_id is NOT NULL with '' for "no device": an event may carry none, but a WITHOUT ROWID table
-- forbids NULL in its primary key, and two NULLs would never merge in a unique key either. The
-- reading queries map the empty string back with NULLIF(device_id, ''). Like progress_events the
-- column has no foreign key: a device forgotten after 400 days (package auth) leaves its id behind
-- and counts as unknown.
--
-- Two known effects of condensing, both accepted (M10):
--   * A playback that spans midnight lands in two day rows and counts as one play in each, so the
--     plays of a condensed period can be slightly higher than the raw ones. The same holds for a
--     playback that spans the 90-day boundary, which is a day boundary too. Played time is never
--     affected; it is summed either way.
--   * A device that stays offline for more than 90 days and uploads its events afterwards can no
--     longer be checked against the condensed rows for a duplicate upload
--     (CountOfflineDuplicateEvents in queries/app/offline.sql reads progress_events only), so such
--     an upload would count twice. Package playback refuses offline events older than
--     offlineEventMaxAge (30 days), which keeps this out of reach in practice; an upload for an
--     already condensed day is added to its day row (ON CONFLICT) instead of being lost.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE progress_days (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    device_id  TEXT    NOT NULL DEFAULT '',
    day        INTEGER NOT NULL,
    played_ms  INTEGER NOT NULL CHECK (played_ms >= 0),
    plays      INTEGER NOT NULL CHECK (plays >= 0),
    PRIMARY KEY (profile_id, item_id, device_id, day)
) STRICT, WITHOUT ROWID;

-- Carries the three statistics queries (StatsTopTitles, StatsWatchTimeByProfile, StatsDevices):
-- they filter a range of days and then group by title, profile or device. Every index entry of a
-- WITHOUT ROWID table carries the primary key columns, so with played_ms and plays the index covers
-- those queries without reading the table itself.
CREATE INDEX progress_days_day ON progress_days (day, played_ms, plays);

-- The condensation looks for the oldest raw event and deletes one whole day of them;
-- progress_events_profile_time leads with the profile, so server_time needs an index of its own.
CREATE INDEX progress_events_server_time ON progress_events (server_time);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
