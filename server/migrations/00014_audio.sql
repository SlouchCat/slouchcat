-- Audiobooks and podcasts (plan M6, 3.1, 3.2, 3.3, AN-2 decision 9; M6 design section 1). Times are
-- Unix milliseconds (UTC).
--
--   libraries      rebuilt: kind gains 'audiobooks' and 'podcasts'; default_rating is the age rating
--                  every title of an audio library starts with (AN-2, rating_source 'library'); NULL for
--                  video libraries. Podcast libraries have no root.
--   items          audio columns: author, narrator, series_name and series_index (volume) of an
--                  audiobook; explicit (itunes:explicit or metadata.json → FSK 18, rating_source 'feed'
--                  or 'tags'); duration_ms (book-wide sum of the parts, or the feed duration of an
--                  episode); cover_url (feed image) and cover_path (folder image relative to the root, or
--                  'embedded:<media_file_id>'); the feed state of a podcast (feed_url, feed_etag,
--                  feed_modified, feed_checked_at, feed_error; one live subscription per library and feed
--                  URL); guid, published_at, enclosure_type and enclosure_length of an episode
--                  (enclosure_url exists since 00003).
--   media_files    part_index: order of the parts of a multi-file audiobook (NULL for versions and
--                  single files). The availability triggers now also cover podcast_episode (an episode
--                  with an enclosure stays available without its file).
--   chapters       book-wide chapter list of an audiobook (from chapter marks or one per part), with the
--                  part the chapter starts in and its start inside that part.
--   progress       playback_rate: speed per profile and title (H-3).
--   downloads      rebuilt: media_file_id becomes nullable and source_url names the enclosure of an
--                  episode without a file. Still no expiry column (O-6).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

-- ---------------------------------------------------------------------------------------------
-- libraries: new kinds and the default rating (AN-2).
-- ---------------------------------------------------------------------------------------------
CREATE TABLE libraries_new (
    id                     INTEGER PRIMARY KEY,
    name                   TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    kind                   TEXT    NOT NULL CHECK (kind IN ('movies', 'series', 'audiobooks', 'podcasts')),
    default_rating         INTEGER CHECK (default_rating IS NULL OR default_rating IN (0, 6, 12, 16, 18)),
    created_at             INTEGER NOT NULL,
    updated_at             INTEGER NOT NULL,
    last_scan_state        TEXT    NOT NULL DEFAULT 'never'
                           CHECK (last_scan_state IN ('never', 'queued', 'running', 'done', 'failed', 'aborted')),
    last_scan_started_at   INTEGER,
    last_scan_finished_at  INTEGER,
    last_scan_error        TEXT,
    scan_files_seen        INTEGER NOT NULL DEFAULT 0,
    scan_files_total       INTEGER NOT NULL DEFAULT 0,
    loss_accepted_at       INTEGER
) STRICT;

INSERT INTO libraries_new (id, name, kind, default_rating, created_at, updated_at, last_scan_state,
                           last_scan_started_at, last_scan_finished_at, last_scan_error, scan_files_seen,
                           scan_files_total, loss_accepted_at)
SELECT id, name, kind, NULL, created_at, updated_at, last_scan_state,
       last_scan_started_at, last_scan_finished_at, last_scan_error, scan_files_seen,
       scan_files_total, loss_accepted_at
FROM libraries;

DROP TABLE libraries;
ALTER TABLE libraries_new RENAME TO libraries;

-- ---------------------------------------------------------------------------------------------
-- items: audiobook, podcast and episode columns.
-- ---------------------------------------------------------------------------------------------
ALTER TABLE items ADD COLUMN author TEXT;
ALTER TABLE items ADD COLUMN narrator TEXT;
ALTER TABLE items ADD COLUMN series_name TEXT;
ALTER TABLE items ADD COLUMN series_index REAL;
ALTER TABLE items ADD COLUMN explicit INTEGER NOT NULL DEFAULT 0 CHECK (explicit IN (0, 1));
ALTER TABLE items ADD COLUMN duration_ms INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0);
ALTER TABLE items ADD COLUMN cover_url TEXT;
ALTER TABLE items ADD COLUMN cover_path TEXT;
ALTER TABLE items ADD COLUMN feed_url TEXT;
ALTER TABLE items ADD COLUMN feed_etag TEXT;
ALTER TABLE items ADD COLUMN feed_modified TEXT;
ALTER TABLE items ADD COLUMN feed_checked_at INTEGER;
ALTER TABLE items ADD COLUMN feed_error TEXT;
ALTER TABLE items ADD COLUMN guid TEXT;
ALTER TABLE items ADD COLUMN published_at INTEGER;
ALTER TABLE items ADD COLUMN enclosure_type TEXT;
ALTER TABLE items ADD COLUMN enclosure_length INTEGER CHECK (enclosure_length IS NULL OR enclosure_length >= 0);

CREATE UNIQUE INDEX items_episode_guid ON items (parent_id, guid) WHERE guid IS NOT NULL;
CREATE INDEX items_feed_due ON items (kind, feed_checked_at) WHERE kind = 'podcast' AND deleted_at IS NULL;
CREATE UNIQUE INDEX items_podcast_feed ON items (library_id, feed_url) WHERE kind = 'podcast' AND deleted_at IS NULL;
CREATE INDEX items_published ON items (parent_id, published_at DESC) WHERE kind = 'podcast_episode';

-- ---------------------------------------------------------------------------------------------
-- media_files: parts of a multi-file audiobook; availability triggers also cover episodes with a file.
-- ---------------------------------------------------------------------------------------------
ALTER TABLE media_files ADD COLUMN part_index INTEGER CHECK (part_index IS NULL OR part_index >= 1);

DROP TRIGGER media_files_insert;
DROP TRIGGER media_files_update;
DROP TRIGGER media_files_delete;

-- +goose StatementBegin
CREATE TRIGGER media_files_insert AFTER INSERT ON media_files
BEGIN
    UPDATE items SET available = CASE
            WHEN kind = 'podcast_episode' AND enclosure_url IS NOT NULL THEN 1
            ELSE EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since IS NULL) END
    WHERE id = NEW.item_id AND kind IN ('movie', 'episode', 'extra', 'audiobook', 'podcast_episode');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER media_files_update AFTER UPDATE OF missing_since, item_id ON media_files
BEGIN
    UPDATE items SET available = CASE
            WHEN kind = 'podcast_episode' AND enclosure_url IS NOT NULL THEN 1
            ELSE EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since IS NULL) END
    WHERE id IN (NEW.item_id, OLD.item_id) AND kind IN ('movie', 'episode', 'extra', 'audiobook', 'podcast_episode');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER media_files_delete AFTER DELETE ON media_files
BEGIN
    UPDATE items SET available = CASE
            WHEN kind = 'podcast_episode' AND enclosure_url IS NOT NULL THEN 1
            ELSE EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since IS NULL) END
    WHERE id = OLD.item_id AND kind IN ('movie', 'episode', 'extra', 'audiobook', 'podcast_episode');
END;
-- +goose StatementEnd

-- ---------------------------------------------------------------------------------------------
-- chapters: book-wide chapter list of an audiobook.
-- ---------------------------------------------------------------------------------------------
CREATE TABLE chapters (
    item_id       TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    idx           INTEGER NOT NULL CHECK (idx >= 0),
    title         TEXT    NOT NULL,
    start_ms      INTEGER NOT NULL CHECK (start_ms >= 0),
    end_ms        INTEGER NOT NULL CHECK (end_ms >= start_ms),
    media_file_id TEXT    REFERENCES media_files(id) ON DELETE CASCADE,
    file_start_ms INTEGER NOT NULL DEFAULT 0 CHECK (file_start_ms >= 0),
    PRIMARY KEY (item_id, idx)
) STRICT, WITHOUT ROWID;
CREATE INDEX chapters_media_file ON chapters (media_file_id);

-- ---------------------------------------------------------------------------------------------
-- progress: playback speed per profile and title (H-3).
-- ---------------------------------------------------------------------------------------------
ALTER TABLE progress ADD COLUMN playback_rate REAL
    CHECK (playback_rate IS NULL OR (playback_rate >= 0.5 AND playback_rate <= 3.0));

-- ---------------------------------------------------------------------------------------------
-- downloads: a nullable media file and the enclosure source of an episode without a file.
-- ---------------------------------------------------------------------------------------------
CREATE TABLE downloads_new (
    id                  TEXT    PRIMARY KEY CHECK (length(id) = 26),
    account_id          INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id          INTEGER REFERENCES profiles(id) ON DELETE SET NULL,
    device_id           TEXT    NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    item_id             TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    media_file_id       TEXT    REFERENCES media_files(id) ON DELETE CASCADE,
    source_url          TEXT,
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
    CHECK (media_file_id IS NOT NULL OR source_url IS NOT NULL),
    UNIQUE (device_id, item_id)
) STRICT;

INSERT INTO downloads_new (id, account_id, profile_id, device_id, item_id, media_file_id, source_url, quality,
                           audio_stream, auto, state, mode, width, height, video_bitrate, audio_bitrate,
                           duration_ms, estimated_bytes, size_bytes, segments_done, segments_total, error,
                           allowed_profile_ids, allowed_at, created_at, updated_at, ready_at, fetched_at,
                           files_removed_at)
SELECT id, account_id, profile_id, device_id, item_id, media_file_id, NULL, quality,
       audio_stream, auto, state, mode, width, height, video_bitrate, audio_bitrate,
       duration_ms, estimated_bytes, size_bytes, segments_done, segments_total, error,
       allowed_profile_ids, allowed_at, created_at, updated_at, ready_at, fetched_at,
       files_removed_at
FROM downloads;

-- The tombstone trigger must not fire for the rows that move to the new table.
DROP TRIGGER downloads_tombstone;
DROP TABLE downloads;
ALTER TABLE downloads_new RENAME TO downloads;

CREATE INDEX downloads_account ON downloads (account_id);
CREATE INDEX downloads_state ON downloads (state, updated_at);
CREATE INDEX downloads_item ON downloads (item_id);
CREATE INDEX downloads_profile_device ON downloads (profile_id, device_id);

-- +goose StatementBegin
CREATE TRIGGER downloads_tombstone AFTER DELETE ON downloads
BEGIN
    INSERT OR REPLACE INTO download_tombstones (id, device_id, removed_at)
    VALUES (OLD.id, OLD.device_id, CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER));
END;
-- +goose StatementEnd

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
