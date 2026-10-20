-- Playback complete (plan M4, 3.5, W-6, W-7, W-8, W-9, K-5; M4 design section 1). Times are Unix
-- milliseconds (UTC).
--
--   devices            device capability profile (W-7): name and type reported by the client, the
--                      capabilities JSON (PlaybackCapabilities of the contract), the account and
--                      profile of the last playback start (owner). last_seen_at is the last contact.
--   playback_sessions  the decision of a start (quality rung, output size and bit rate, hardware
--                      encoder, chosen audio and subtitle track, how subtitles are delivered), the
--                      capabilities the client reported (a rebuild after a restart repeats the same
--                      decision), the last reported position (stream overview, W-6) and why a session
--                      ended (stopped_by: client, admin, limit, logout, revoked).
--   profiles, accounts W-8 limits (bandwidth, maximum output height, concurrent streams) and the
--                      K-5 daily watch-time limit per profile; NULL = no limit. The account columns
--                      cap every profile of a guest account like max_rating does.
--   media_segments     detected intro and outro per episode (W-9), one row per item and kind.
--   items              segments_state and segments_checked_at track the detection job per season.
--   progress_events    index by profile and server time for the watch-time report (K-5).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

ALTER TABLE devices ADD COLUMN name TEXT CHECK (name IS NULL OR length(name) <= 80);
ALTER TABLE devices ADD COLUMN type TEXT CHECK (type IS NULL OR type IN ('desktop', 'phone', 'tablet', 'tv', 'other'));
ALTER TABLE devices ADD COLUMN account_id INTEGER REFERENCES accounts(id) ON DELETE SET NULL;
ALTER TABLE devices ADD COLUMN profile_id INTEGER REFERENCES profiles(id) ON DELETE SET NULL;
ALTER TABLE devices ADD COLUMN capabilities TEXT CHECK (capabilities IS NULL OR json_valid(capabilities));
ALTER TABLE devices ADD COLUMN capabilities_at INTEGER;
CREATE INDEX devices_account ON devices (account_id);

ALTER TABLE playback_sessions ADD COLUMN quality TEXT CHECK (quality IS NULL OR quality IN ('auto', '1080p', '720p', '480p', '360p'));
ALTER TABLE playback_sessions ADD COLUMN width INTEGER;
ALTER TABLE playback_sessions ADD COLUMN height INTEGER;
ALTER TABLE playback_sessions ADD COLUMN bitrate INTEGER;
ALTER TABLE playback_sessions ADD COLUMN hardware INTEGER NOT NULL DEFAULT 0 CHECK (hardware IN (0, 1));
ALTER TABLE playback_sessions ADD COLUMN audio_track INTEGER;
ALTER TABLE playback_sessions ADD COLUMN subtitle_track INTEGER;
ALTER TABLE playback_sessions ADD COLUMN subtitle_delivery TEXT CHECK (subtitle_delivery IS NULL OR subtitle_delivery IN ('text', 'burn'));
ALTER TABLE playback_sessions ADD COLUMN capabilities TEXT CHECK (capabilities IS NULL OR json_valid(capabilities));
ALTER TABLE playback_sessions ADD COLUMN position_ms INTEGER;
ALTER TABLE playback_sessions ADD COLUMN stopped_by TEXT CHECK (stopped_by IS NULL OR stopped_by IN ('client', 'admin', 'limit', 'logout', 'revoked'));
CREATE INDEX playback_sessions_profile_activity ON playback_sessions (profile_id, revoked_at, last_activity_at);
CREATE INDEX playback_sessions_device ON playback_sessions (device_id);

ALTER TABLE profiles ADD COLUMN max_bitrate_kbps INTEGER CHECK (max_bitrate_kbps IS NULL OR max_bitrate_kbps > 0);
ALTER TABLE profiles ADD COLUMN max_height INTEGER CHECK (max_height IS NULL OR max_height IN (360, 480, 720, 1080, 2160));
ALTER TABLE profiles ADD COLUMN max_streams INTEGER CHECK (max_streams IS NULL OR max_streams BETWEEN 1 AND 16);
ALTER TABLE profiles ADD COLUMN watch_limit_minutes INTEGER CHECK (watch_limit_minutes IS NULL OR watch_limit_minutes BETWEEN 1 AND 1440);

ALTER TABLE accounts ADD COLUMN max_bitrate_kbps INTEGER CHECK (max_bitrate_kbps IS NULL OR max_bitrate_kbps > 0);
ALTER TABLE accounts ADD COLUMN max_height INTEGER CHECK (max_height IS NULL OR max_height IN (360, 480, 720, 1080, 2160));
ALTER TABLE accounts ADD COLUMN max_streams INTEGER CHECK (max_streams IS NULL OR max_streams BETWEEN 1 AND 16);

CREATE TABLE media_segments (
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL CHECK (kind IN ('intro', 'outro')),
    start_ms   INTEGER NOT NULL CHECK (start_ms >= 0),
    end_ms     INTEGER NOT NULL CHECK (end_ms > start_ms),
    source     TEXT    NOT NULL CHECK (source IN ('fingerprint', 'chapter', 'manual')),
    confidence REAL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (item_id, kind)
) STRICT, WITHOUT ROWID;

ALTER TABLE items ADD COLUMN segments_state TEXT NOT NULL DEFAULT 'pending' CHECK (segments_state IN ('pending', 'done', 'none', 'failed'));
ALTER TABLE items ADD COLUMN segments_checked_at INTEGER;

CREATE INDEX progress_events_profile_time ON progress_events (profile_id, server_time);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
