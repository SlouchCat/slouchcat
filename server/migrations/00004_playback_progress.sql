-- Playback progress (W-5, plan 3.2 and 3.4 step 7, M1a step 5). User data hangs on the item, never on
-- the media file, so renames and version changes lose nothing. Times are Unix milliseconds (UTC).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;

-- Merged position per profile and item.
CREATE TABLE progress (
    profile_id  INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id     TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    position_ms INTEGER NOT NULL CHECK (position_ms >= 0),
    duration_ms INTEGER NOT NULL CHECK (duration_ms >= 0),
    completed   INTEGER NOT NULL DEFAULT 0 CHECK (completed IN (0, 1)),
    updated_at  INTEGER NOT NULL,
    PRIMARY KEY (profile_id, item_id)
) STRICT, WITHOUT ROWID;

CREATE INDEX progress_recent ON progress (profile_id, updated_at DESC);

-- Raw events as sent by clients (basic form in M1a; offline buffering and merging follow in M5;
-- condensed to daily values per profile and item after 90 days, M8).
CREATE TABLE progress_events (
    id            INTEGER PRIMARY KEY,
    profile_id    INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id       TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    media_file_id TEXT,
    device_id     TEXT,
    position_ms   INTEGER NOT NULL CHECK (position_ms >= 0),
    duration_ms   INTEGER NOT NULL CHECK (duration_ms >= 0),
    played_ms     INTEGER NOT NULL CHECK (played_ms >= 0),
    seq           INTEGER NOT NULL,
    state         TEXT    NOT NULL CHECK (state IN ('playing', 'paused', 'ended')),
    client_time   INTEGER NOT NULL,
    server_time   INTEGER NOT NULL
) STRICT;

CREATE INDEX progress_events_profile_item ON progress_events (profile_id, item_id, server_time DESC);

PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
