-- Language, subtitle and quality preferences of the player (concept 13 stage 2, sections 11.3 to
-- 11.9; decision 19, F-3, F-4, F-11 to F-14).
--
--   profiles.audio_languages     the audio languages of the profile in order of preference, a JSON
--                                array of ISO 639-1 codes (`["de","en"]`); `[]` = none.
--   profiles.subtitle_mode       the subtitle rule: off, forced (default, F-3), foreign, always.
--   profiles.subtitle_languages  the subtitle languages in order, like audio_languages.
--                                Existing profiles start with their interface language in both
--                                lists (F-12): `["<language>"]`, `[]` when a profile has none.
--                                metadata_language (00028) stays out: it steers titles and texts,
--                                not the sound.
--   profile_series_tracks        the languages a profile last chose by hand for a series (one row
--                                per profile and series; an empty language counts as none,
--                                subtitle_off = the profile switched subtitles off). The series id
--                                always comes from the visibility layer; the table itself never
--                                joins items, so its queries live in the app package.
--   items.original_language      NULL = not looked up yet, '' = looked up, unknown, otherwise an
--                                ISO 639-1 code (11.7).
--   tracks.is_hearing_impaired   an SDH track (11.3, F-11); 0 until the scan stores it.
--   playback_sessions            rebuilt (blocker B-1): the CHECK on quality takes the two new
--                                rungs original and 2160p. SQLite has no ALTER COLUMN for a CHECK,
--                                so the table is created anew with the same columns in the same
--                                order, the rows are copied column by column, the old table is
--                                dropped and the new one renamed; the four indexes come back. No
--                                view, trigger or foreign key points at playback_sessions
--                                (progress_events.playback_session_id is a plain column), and
--                                PRAGMA foreign_key_check at the end proves its own references.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE profiles ADD COLUMN audio_languages TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(audio_languages) AND json_type(audio_languages) = 'array');
ALTER TABLE profiles ADD COLUMN subtitle_mode TEXT NOT NULL DEFAULT 'forced' CHECK (subtitle_mode IN ('off', 'forced', 'foreign', 'always'));
ALTER TABLE profiles ADD COLUMN subtitle_languages TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(subtitle_languages) AND json_type(subtitle_languages) = 'array');

UPDATE profiles
SET audio_languages    = CASE WHEN language IS NULL OR language = '' THEN '[]' ELSE json_array(language) END,
    subtitle_languages = CASE WHEN language IS NULL OR language = '' THEN '[]' ELSE json_array(language) END;

CREATE TABLE profile_series_tracks (
    profile_id        INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    series_id         TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    audio_language    TEXT    NOT NULL DEFAULT '',
    subtitle_language TEXT    NOT NULL DEFAULT '',
    subtitle_off      INTEGER NOT NULL DEFAULT 0 CHECK (subtitle_off IN (0, 1)),
    updated_at        INTEGER NOT NULL,
    PRIMARY KEY (profile_id, series_id)
) STRICT, WITHOUT ROWID;

CREATE INDEX profile_series_tracks_series ON profile_series_tracks (series_id);

ALTER TABLE items ADD COLUMN original_language TEXT CHECK (original_language IS NULL OR original_language = '' OR length(original_language) = 2);

ALTER TABLE tracks ADD COLUMN is_hearing_impaired INTEGER NOT NULL DEFAULT 0 CHECK (is_hearing_impaired IN (0, 1));

CREATE TABLE playback_sessions_new (
    id                TEXT    PRIMARY KEY CHECK (length(id) = 26), -- ULID
    session_id        INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    account_id        INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id        INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id           TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    media_file_id     TEXT    NOT NULL,
    device_id         TEXT,
    mode              TEXT    NOT NULL CHECK (mode IN ('direct', 'remux', 'transcode')),
    backend           TEXT,                                         -- transcode backend, NULL for direct
    reason            TEXT,                                         -- why not direct play
    started_at        INTEGER NOT NULL,
    last_activity_at  INTEGER NOT NULL,
    expires_at        INTEGER NOT NULL,
    revoked_at        INTEGER,
    quality           TEXT    CHECK (quality IS NULL OR quality IN ('auto', 'original', '2160p', '1080p', '720p', '480p', '360p')),
    width             INTEGER,
    height            INTEGER,
    bitrate           INTEGER,
    hardware          INTEGER NOT NULL DEFAULT 0 CHECK (hardware IN (0, 1)),
    audio_track       INTEGER,
    subtitle_track    INTEGER,
    subtitle_delivery TEXT    CHECK (subtitle_delivery IS NULL OR subtitle_delivery IN ('text', 'burn')),
    capabilities      TEXT    CHECK (capabilities IS NULL OR json_valid(capabilities)),
    position_ms       INTEGER,
    stopped_by        TEXT    CHECK (stopped_by IS NULL OR stopped_by IN ('client', 'admin', 'limit', 'logout', 'revoked'))
) STRICT;

INSERT INTO playback_sessions_new (id, session_id, account_id, profile_id, item_id, media_file_id, device_id, mode,
                                   backend, reason, started_at, last_activity_at, expires_at, revoked_at,
                                   quality, width, height, bitrate, hardware, audio_track, subtitle_track,
                                   subtitle_delivery, capabilities, position_ms, stopped_by)
SELECT id, session_id, account_id, profile_id, item_id, media_file_id, device_id, mode,
       backend, reason, started_at, last_activity_at, expires_at, revoked_at,
       quality, width, height, bitrate, hardware, audio_track, subtitle_track,
       subtitle_delivery, capabilities, position_ms, stopped_by
FROM playback_sessions;

DROP TABLE playback_sessions;
ALTER TABLE playback_sessions_new RENAME TO playback_sessions;

CREATE INDEX playback_sessions_session ON playback_sessions (session_id);
CREATE INDEX playback_sessions_expires ON playback_sessions (expires_at);
CREATE INDEX playback_sessions_profile_activity ON playback_sessions (profile_id, revoked_at, last_activity_at);
CREATE INDEX playback_sessions_device ON playback_sessions (device_id);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
