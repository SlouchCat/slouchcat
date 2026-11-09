-- User picks from profile to profile (concept 18 part B, sections 14.5 and 14.8; ADR 0028).
--
--   user_picks    one row per title and recommending profile. Recommending the same title again
--                 only renews created_at (Unix milliseconds). The profile is stored for taking a
--                 recommendation back and for showing a title once, never shown to viewers.
--   picks_config  the singleton setting (pattern spotlight_config): a title stays in the list while
--                 its newest recommendation is younger than retention_days (7 to 365, default 60).
--
-- top_picks (00003, the curated list of the administration with target profile, note, time
-- window and order) goes with its indexes; its rows are discarded (U-F11), the list starts empty.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE user_picks (
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (item_id, profile_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX user_picks_created ON user_picks (created_at);
CREATE INDEX user_picks_profile ON user_picks (profile_id);

CREATE TABLE picks_config (
    id             INTEGER PRIMARY KEY CHECK (id = 1),
    retention_days INTEGER NOT NULL DEFAULT 60 CHECK (retention_days BETWEEN 7 AND 365),
    updated_at     INTEGER NOT NULL
) STRICT;
INSERT INTO picks_config (id, retention_days, updated_at) VALUES (1, 60, 0);

DROP TABLE top_picks;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
