-- Concept 22 stage 1 «Sprachgerüst» (ADR 0033): profiles loses the CHECK on language and the CHECK
-- on max_rating.
--
--   language   was CHECK (language IN ('de', 'en')). The list of interface languages grows with
--              the language packs of the web app (concept 22 section 4.3); a CHECK with every code
--              would make each new language a table rebuild. The server checks the code against
--              model.UILanguages, the one list that equals the enum Language of the contract.
--   max_rating was CHECK (max_rating IN (0, 6, 12, 16, 18)). Stage 3 of concept 22 stores the
--              minimum age of the rating country (13, 15, 17 ...); dropping the CHECK now spares
--              profiles a second rebuild (section 4.11). The server keeps checking the value.
--
-- SQLite has no ALTER COLUMN for a CHECK, so profiles is rebuilt (the 12-step procedure, as 00022
-- did for accounts). Every other column keeps its type, default and CHECK character for character,
-- in the order the schema has after 00032 (columns added by 00007, 00011, 00013, 00028 and 00032
-- sit behind updated_at). The id is kept, therefore every table pointing at profiles (sessions,
-- progress, progress_days, progress_events, watchlist, downloads, download_rules, devices,
-- playback_sessions, notifications, deliveries, profile_libraries, profile_series_tracks,
-- pin_attempts, announcement_acks, user_picks and the radio tables) still points at the right
-- row; PRAGMA foreign_key_check at the end is the proof.
--
-- The view visible_items has to go first and come back: ALTER TABLE ... RENAME reparses the whole
-- schema and fails while a view names a table that does not exist. It is restored word for word
-- from 00056, the migration that wrote it last. DROP TABLE takes the trigger
-- activity_events_profile_deleted (00031) and the indexes profiles_account and profiles_primary
-- with it; they come back unchanged. No other view or trigger names profiles.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

DROP VIEW visible_items;

CREATE TABLE profiles_new (
    id            INTEGER PRIMARY KEY,
    account_id    INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name          TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 60),
    image         TEXT,
    max_rating    INTEGER NOT NULL DEFAULT 18,
    language      TEXT    NOT NULL DEFAULT 'de',
    accent        TEXT,
    all_libraries INTEGER NOT NULL DEFAULT 1 CHECK (all_libraries IN (0, 1)),
    is_default    INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    pin_hash      TEXT,
    max_bitrate_kbps INTEGER CHECK (max_bitrate_kbps IS NULL OR max_bitrate_kbps > 0),
    max_height    INTEGER CHECK (max_height IS NULL OR max_height IN (360, 480, 720, 1080, 2160)),
    max_streams   INTEGER CHECK (max_streams IS NULL OR max_streams BETWEEN 1 AND 16),
    watch_limit_minutes INTEGER CHECK (watch_limit_minutes IS NULL OR watch_limit_minutes BETWEEN 1 AND 1440),
    pin_salt      TEXT CHECK (pin_salt IS NULL OR length(pin_salt) = 32),
    offline_pin   TEXT CHECK (offline_pin IS NULL OR json_valid(offline_pin)),
    metadata_language TEXT CHECK (metadata_language IS NULL OR length(metadata_language) BETWEEN 2 AND 12),
    audio_languages TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(audio_languages) AND json_type(audio_languages) = 'array'),
    subtitle_mode TEXT NOT NULL DEFAULT 'forced' CHECK (subtitle_mode IN ('off', 'forced', 'foreign', 'always')),
    subtitle_languages TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(subtitle_languages) AND json_type(subtitle_languages) = 'array')
) STRICT;

INSERT INTO profiles_new (id, account_id, name, image, max_rating, language, accent, all_libraries,
                          is_default, created_at, updated_at, pin_hash, max_bitrate_kbps, max_height,
                          max_streams, watch_limit_minutes, pin_salt, offline_pin, metadata_language,
                          audio_languages, subtitle_mode, subtitle_languages)
SELECT id, account_id, name, image, max_rating, language, accent, all_libraries,
       is_default, created_at, updated_at, pin_hash, max_bitrate_kbps, max_height,
       max_streams, watch_limit_minutes, pin_salt, offline_pin, metadata_language,
       audio_languages, subtitle_mode, subtitle_languages
FROM profiles;

DROP TABLE profiles;
ALTER TABLE profiles_new RENAME TO profiles;

CREATE INDEX profiles_account ON profiles (account_id);
CREATE UNIQUE INDEX profiles_primary ON profiles (account_id) WHERE is_default = 1;

-- Word for word from 00031.
-- +goose StatementBegin
CREATE TRIGGER activity_events_profile_deleted AFTER DELETE ON profiles
BEGIN
    DELETE FROM activity_events WHERE profile_id = OLD.id;
END;
-- +goose StatementEnd

-- Visibility (plan 3.3): word for word from 00056.
CREATE VIEW visible_items AS
SELECT p.id AS profile_id, i.id AS item_id
FROM profiles AS p
JOIN accounts AS a ON a.id = p.account_id
JOIN items AS i
  ON i.deleted_at IS NULL
 AND i.available = 1
 AND COALESCE(i.effective_rating, 18) <= min(p.max_rating, a.max_rating)
 AND (i.kind NOT IN ('series', 'season', 'podcast') OR COALESCE(i.min_child_rating, 18) <= min(p.max_rating, a.max_rating))
 AND (i.kind <> 'extra' OR EXISTS (
        SELECT 1 FROM items AS pa
        WHERE pa.id = i.parent_id AND pa.deleted_at IS NULL AND pa.available = 1
          AND COALESCE(pa.effective_rating, 18) <= min(p.max_rating, a.max_rating)
          AND (pa.kind NOT IN ('series', 'season', 'podcast') OR COALESCE(pa.min_child_rating, 18) <= min(p.max_rating, a.max_rating))))
WHERE (p.all_libraries = 1
       OR EXISTS (SELECT 1 FROM profile_libraries AS pl
                  WHERE pl.profile_id = p.id AND pl.library_id = i.library_id))
  AND (a.all_libraries = 1
       OR EXISTS (SELECT 1 FROM account_libraries AS al
                  WHERE al.account_id = a.id AND al.library_id = i.library_id))
  AND NOT EXISTS (SELECT 1 FROM libraries AS dl WHERE dl.id = i.library_id AND dl.deleting_at IS NOT NULL);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
