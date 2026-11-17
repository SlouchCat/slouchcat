-- Concept 25 stage 4, step 4.6b (08-datenmodell-api.md section 2.14): the profile tag in Radarr
-- and Sonarr. Only package wishes reads and writes the tables (store/wishq). Times are Unix
-- milliseconds.
--
--   wish_profile_tags  the tag name of a profile, written once and never changed by Slouchcat
--                      afterwards (N-21, K-18): the name the first tag of the profile used
--                      (custom 0: prefix and profile name after N-113, with the account's name
--                      added on a collision) or a name the administrator set (custom 1,
--                      setProfileTagName). Only a-z, 0-9 and "-", 1 to 64 characters, unique.
--   wish_tags          the state of the tag per wish and profile (owner and co-wishers, N-24):
--                      pending (not yet set, for example the title is not yet in Radarr),
--                      set, failed (Radarr or Sonarr refused or did not answer; the next run tries
--                      again, N-25). tag_name is the name at the time of setting; a tag is never
--                      removed (N-26). last_error is an error class, never an address or a key.
--                      Wishes placed directly in Seerr (origin seerr) get no row (N-27).
--
-- New tables only, nothing is rebuilt; the frame of the template stays.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE wish_profile_tags (
    profile_id INTEGER NOT NULL PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL UNIQUE CHECK (length(name) BETWEEN 1 AND 64 AND name NOT GLOB '*[^a-z0-9-]*'),
    custom     INTEGER NOT NULL DEFAULT 0 CHECK (custom IN (0, 1)),
    set_at     INTEGER NOT NULL
) STRICT;

CREATE TABLE wish_tags (
    wish_id      INTEGER NOT NULL REFERENCES wishes(id) ON DELETE CASCADE,
    profile_id   INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    service      TEXT    NOT NULL CHECK (service IN ('radarr', 'sonarr')),
    tag_name     TEXT    NOT NULL CHECK (length(tag_name) BETWEEN 1 AND 64 AND tag_name NOT GLOB '*[^a-z0-9-]*'),
    arr_tag_id   INTEGER CHECK (arr_tag_id IS NULL OR arr_tag_id > 0),
    arr_title_id INTEGER CHECK (arr_title_id IS NULL OR arr_title_id > 0),
    state        TEXT    NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'set', 'failed')),
    attempts     INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error   TEXT    CHECK (last_error IS NULL OR (last_error <> '' AND last_error NOT GLOB '*[^a-z_]*')),
    set_at       INTEGER,
    PRIMARY KEY (wish_id, profile_id),
    CHECK ((state = 'set') = (set_at IS NOT NULL))
) STRICT, WITHOUT ROWID;

-- The rows wishes.tag still has to set.
CREATE INDEX wish_tags_open ON wish_tags (state) WHERE state <> 'set';
CREATE INDEX wish_tags_profile ON wish_tags (profile_id);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
