-- Offline playback (plan M5, 3.2, 3.3, AN-4, AN-10, AN-18, AN-19, AN-20; M5 design section 1). Times
-- are Unix milliseconds (UTC).
--
--   downloads            one download per device and title (user data hangs on the item, plan 3.2):
--                        the requesting profile, the source file and audio stream, the download profile
--                        (quality), the preparation state of the fragmented MP4 in /cache/downloads, the
--                        size estimate and the result, and allowed_profile_ids, the profiles of the
--                        account that may see the title, computed by the server at creation and at every
--                        sync (plan 3.3). There is deliberately no expiry column (O-6).
--   download_tombstones  every deleted download, also through cascades (guest revoked, device forgotten,
--                        title deleted): the revocation list a device reads at its next contact (AN-4,
--                        K-7). No foreign keys, because the device and the account may be gone.
--   download_rules       auto-download rule per profile and device (O-3): next episode of started
--                        series, Wi-Fi only (checked by the client), maximum count, quality.
--                        new_podcast_episodes is stored for M6 and has no effect before.
--   profiles             pin_salt: random per PIN, regenerated whenever the PIN changes, delivered only to
--                        devices with downloads of the profile; the client derives its offline verifier
--                        from it at the next successful online PIN entry (AN-19). offline_pin: PBKDF2
--                        verifier of the separate, longer offline PIN.
--   progress_events      offline: uploaded from an offline playback (O-5); server_time is then the
--                        corrected event time, uploaded_at the server time of the upload; download_id
--                        names the download that played. playback_session_id carries the client's id of
--                        the offline playback (duplicate check and played-time bound).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE downloads (
    id                  TEXT    PRIMARY KEY CHECK (length(id) = 26),
    account_id          INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id          INTEGER REFERENCES profiles(id) ON DELETE SET NULL,
    device_id           TEXT    NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    item_id             TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    media_file_id       TEXT    NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
    quality             TEXT    NOT NULL CHECK (quality IN ('high', 'medium', 'low')),
    audio_stream        INTEGER NOT NULL CHECK (audio_stream >= 0),
    auto                INTEGER NOT NULL DEFAULT 0 CHECK (auto IN (0, 1)),
    state               TEXT    NOT NULL DEFAULT 'queued'
                                CHECK (state IN ('queued', 'preparing', 'paused', 'ready', 'failed', 'evicted')),
    mode                TEXT    CHECK (mode IS NULL OR mode IN ('remux', 'transcode')),
    width               INTEGER,
    height              INTEGER,
    video_bitrate       INTEGER,
    audio_bitrate       INTEGER,
    duration_ms         INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0),
    estimated_bytes     INTEGER NOT NULL CHECK (estimated_bytes >= 0),
    size_bytes          INTEGER CHECK (size_bytes IS NULL OR size_bytes >= 0),
    segments_done       INTEGER NOT NULL DEFAULT 0 CHECK (segments_done >= 0),
    segments_total      INTEGER CHECK (segments_total IS NULL OR segments_total >= 0),
    error               TEXT,
    allowed_profile_ids TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(allowed_profile_ids)),
    allowed_at          INTEGER,
    created_at          INTEGER NOT NULL,
    updated_at          INTEGER NOT NULL,
    ready_at            INTEGER,
    fetched_at          INTEGER,
    files_removed_at    INTEGER,
    UNIQUE (device_id, item_id)
) STRICT;
CREATE INDEX downloads_account ON downloads (account_id);
CREATE INDEX downloads_state ON downloads (state, updated_at);
CREATE INDEX downloads_item ON downloads (item_id);
CREATE INDEX downloads_profile_device ON downloads (profile_id, device_id);

CREATE TABLE download_tombstones (
    id         TEXT    PRIMARY KEY CHECK (length(id) = 26),
    device_id  TEXT    NOT NULL,
    removed_at INTEGER NOT NULL
) STRICT, WITHOUT ROWID;
CREATE INDEX download_tombstones_device ON download_tombstones (device_id);

-- Every deleted download leaves a tombstone, including deletions through ON DELETE CASCADE (SQLite
-- fires the triggers of the child table for cascaded deletes).
-- +goose StatementBegin
CREATE TRIGGER downloads_tombstone AFTER DELETE ON downloads
BEGIN
    INSERT OR REPLACE INTO download_tombstones (id, device_id, removed_at)
    VALUES (OLD.id, OLD.device_id, CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER));
END;
-- +goose StatementEnd

CREATE TABLE download_rules (
    profile_id           INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    device_id            TEXT    NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    enabled              INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    next_episodes        INTEGER NOT NULL DEFAULT 1 CHECK (next_episodes IN (0, 1)),
    new_podcast_episodes INTEGER NOT NULL DEFAULT 0 CHECK (new_podcast_episodes IN (0, 1)),
    wifi_only            INTEGER NOT NULL DEFAULT 1 CHECK (wifi_only IN (0, 1)),
    max_count            INTEGER NOT NULL DEFAULT 3 CHECK (max_count BETWEEN 1 AND 20),
    quality              TEXT    NOT NULL DEFAULT 'medium' CHECK (quality IN ('high', 'medium', 'low')),
    updated_at           INTEGER NOT NULL,
    PRIMARY KEY (profile_id, device_id)
) STRICT, WITHOUT ROWID;
CREATE INDEX download_rules_device ON download_rules (device_id);

ALTER TABLE profiles ADD COLUMN pin_salt TEXT CHECK (pin_salt IS NULL OR length(pin_salt) = 32);
ALTER TABLE profiles ADD COLUMN offline_pin TEXT CHECK (offline_pin IS NULL OR json_valid(offline_pin));
UPDATE profiles SET pin_salt = lower(hex(randomblob(16))) WHERE pin_hash IS NOT NULL;

ALTER TABLE progress_events ADD COLUMN offline INTEGER NOT NULL DEFAULT 0 CHECK (offline IN (0, 1));
ALTER TABLE progress_events ADD COLUMN uploaded_at INTEGER;
ALTER TABLE progress_events ADD COLUMN download_id TEXT;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
