CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	);
INSERT INTO "goose_db_version" VALUES (1, 0, 1, '2026-10-19 12:52:47 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (2, 1, 1, '2026-10-19 12:52:47 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (3, 2, 1, '2026-10-19 12:52:47 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (4, 3, 1, '2026-10-19 12:52:47 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (5, 4, 1, '2026-10-19 12:52:47 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (6, 5, 1, '2026-10-19 12:52:47 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (7, 6, 1, '2026-10-19 12:52:47 +0000 UTC');
CREATE TABLE jobs (
    id              INTEGER PRIMARY KEY,
    type            TEXT    NOT NULL,
    priority        INTEGER NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 9),
    idempotency_key TEXT,
    payload         TEXT    NOT NULL DEFAULT '{}' CHECK (json_valid(payload)),
    state           TEXT    NOT NULL DEFAULT 'queued'
                    CHECK (state IN ('queued', 'running', 'done', 'failed', 'cancelled')),
    run_after       INTEGER NOT NULL,
    lease_owner     TEXT,
    lease_until     INTEGER,
    attempts        INTEGER NOT NULL DEFAULT 0,
    max_attempts    INTEGER NOT NULL DEFAULT 5 CHECK (max_attempts >= 1),
    last_error      TEXT,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
) STRICT;
INSERT INTO "jobs" VALUES (1, 'library.sweep', 2, 'sweep', '{}', 'done', 1792414367955, NULL, NULL, 1, 5, NULL, 1792414367955, 1792414367955);
INSERT INTO "jobs" VALUES (2, 'scan.library', 5, 'scan:1', '{"libraryId":1}', 'done', 1792414369456, NULL, NULL, 1, 5, NULL, 1792414369456, 1792414369962);
INSERT INTO "jobs" VALUES (3, 'metadata.match', 4, 'match:01M5A3HJ56J3YCNC53QZ4GWRHA', '{"itemId":"01M5A3HJ56J3YCNC53QZ4GWRHA"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414369962);
INSERT INTO "jobs" VALUES (4, 'probe.file', 3, 'probe:01M5A3HJ56J3YCNC53R0D695FQ', '{"mediaFileId":"01M5A3HJ56J3YCNC53R0D695FQ"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414370000);
INSERT INTO "jobs" VALUES (5, 'metadata.match', 4, 'match:01M5A3HJ56J3YCNC53R0K433HF', '{"itemId":"01M5A3HJ56J3YCNC53R0K433HF"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414369962);
INSERT INTO "jobs" VALUES (6, 'probe.file', 3, 'probe:01M5A3HJ5761XX1JK34SM1V6P4', '{"mediaFileId":"01M5A3HJ5761XX1JK34SM1V6P4"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414370035);
INSERT INTO "jobs" VALUES (7, 'metadata.match', 4, 'match:01M5A3HJ5761XX1JK34XBHBVYN', '{"itemId":"01M5A3HJ5761XX1JK34XBHBVYN"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414369962);
INSERT INTO "jobs" VALUES (8, 'probe.file', 3, 'probe:01M5A3HJ5761XX1JK34Y093ANY', '{"mediaFileId":"01M5A3HJ5761XX1JK34Y093ANY"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414370065);
INSERT INTO "jobs" VALUES (9, 'metadata.match', 4, 'match:01M5A3HJ5761XX1JK34YAHC05B', '{"itemId":"01M5A3HJ5761XX1JK34YAHC05B"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414369962);
INSERT INTO "jobs" VALUES (10, 'probe.file', 3, 'probe:01M5A3HJ5761XX1JK34YTGKFBM', '{"mediaFileId":"01M5A3HJ5761XX1JK34YTGKFBM"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414370095);
INSERT INTO "jobs" VALUES (11, 'metadata.match', 4, 'match:01M5A3HJ5761XX1JK34ZTABERS', '{"itemId":"01M5A3HJ5761XX1JK34ZTABERS"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414369963);
INSERT INTO "jobs" VALUES (12, 'probe.file', 3, 'probe:01M5A3HJ58CS687HPN6PXTS43R', '{"mediaFileId":"01M5A3HJ58CS687HPN6PXTS43R"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414370123);
INSERT INTO "jobs" VALUES (13, 'metadata.match', 4, 'match:01M5A3HJ58CS687HPN6SHXZMER', '{"itemId":"01M5A3HJ58CS687HPN6SHXZMER"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414369963);
INSERT INTO "jobs" VALUES (14, 'probe.file', 3, 'probe:01M5A3HJ58CS687HPN6V7HA8MK', '{"mediaFileId":"01M5A3HJ58CS687HPN6V7HA8MK"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414370151);
INSERT INTO "jobs" VALUES (15, 'metadata.match', 4, 'match:01M5A3HJ58CS687HPN6YNN1GCX', '{"itemId":"01M5A3HJ58CS687HPN6YNN1GCX"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414369963);
INSERT INTO "jobs" VALUES (16, 'probe.file', 3, 'probe:01M5A3HJ58CS687HPN6YPQHQAA', '{"mediaFileId":"01M5A3HJ58CS687HPN6YPQHQAA"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414370180);
INSERT INTO "jobs" VALUES (17, 'metadata.match', 4, 'match:01M5A3HJ58CS687HPN71S8YMGR', '{"itemId":"01M5A3HJ58CS687HPN71S8YMGR"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414369963);
INSERT INTO "jobs" VALUES (18, 'probe.file', 3, 'probe:01M5A3HJ597DZ0WRSYH7QND1N0', '{"mediaFileId":"01M5A3HJ597DZ0WRSYH7QND1N0"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414370209);
INSERT INTO "jobs" VALUES (19, 'metadata.match', 4, 'match:01M5A3HJ597DZ0WRSYHANXNYWJ', '{"itemId":"01M5A3HJ597DZ0WRSYHANXNYWJ"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414369963);
INSERT INTO "jobs" VALUES (20, 'probe.file', 3, 'probe:01M5A3HJ597DZ0WRSYHC3SYEEM', '{"mediaFileId":"01M5A3HJ597DZ0WRSYHC3SYEEM"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414370241);
INSERT INTO "jobs" VALUES (21, 'metadata.match', 4, 'match:01M5A3HJ597DZ0WRSYHE0DG9QY', '{"itemId":"01M5A3HJ597DZ0WRSYHE0DG9QY"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414369963);
INSERT INTO "jobs" VALUES (22, 'probe.file', 3, 'probe:01M5A3HJ597DZ0WRSYHHNR1170', '{"mediaFileId":"01M5A3HJ597DZ0WRSYHHNR1170"}', 'done', 1792414369958, NULL, NULL, 1, 5, NULL, 1792414369958, 1792414370271);
INSERT INTO "jobs" VALUES (23, 'probe.keyframes', 1, 'keyframes:01M5A3HJ56J3YCNC53R0D695FQ', '{"mediaFileId":"01M5A3HJ56J3YCNC53R0D695FQ"}', 'done', 1792414369999, NULL, NULL, 1, 5, NULL, 1792414369999, 1792414370304);
INSERT INTO "jobs" VALUES (24, 'probe.keyframes', 1, 'keyframes:01M5A3HJ5761XX1JK34SM1V6P4', '{"mediaFileId":"01M5A3HJ5761XX1JK34SM1V6P4"}', 'done', 1792414370035, NULL, NULL, 1, 5, NULL, 1792414370035, 1792414370345);
INSERT INTO "jobs" VALUES (25, 'probe.keyframes', 1, 'keyframes:01M5A3HJ5761XX1JK34Y093ANY', '{"mediaFileId":"01M5A3HJ5761XX1JK34Y093ANY"}', 'done', 1792414370065, NULL, NULL, 1, 5, NULL, 1792414370065, 1792414370379);
INSERT INTO "jobs" VALUES (26, 'probe.keyframes', 1, 'keyframes:01M5A3HJ5761XX1JK34YTGKFBM', '{"mediaFileId":"01M5A3HJ5761XX1JK34YTGKFBM"}', 'done', 1792414370095, NULL, NULL, 1, 5, NULL, 1792414370095, 1792414370410);
INSERT INTO "jobs" VALUES (27, 'probe.keyframes', 1, 'keyframes:01M5A3HJ58CS687HPN6PXTS43R', '{"mediaFileId":"01M5A3HJ58CS687HPN6PXTS43R"}', 'done', 1792414370122, NULL, NULL, 1, 5, NULL, 1792414370122, 1792414370444);
INSERT INTO "jobs" VALUES (28, 'probe.keyframes', 1, 'keyframes:01M5A3HJ58CS687HPN6V7HA8MK', '{"mediaFileId":"01M5A3HJ58CS687HPN6V7HA8MK"}', 'done', 1792414370151, NULL, NULL, 1, 5, NULL, 1792414370151, 1792414370472);
INSERT INTO "jobs" VALUES (29, 'probe.keyframes', 1, 'keyframes:01M5A3HJ58CS687HPN6YPQHQAA', '{"mediaFileId":"01M5A3HJ58CS687HPN6YPQHQAA"}', 'done', 1792414370180, NULL, NULL, 1, 5, NULL, 1792414370180, 1792414370502);
INSERT INTO "jobs" VALUES (30, 'probe.keyframes', 1, 'keyframes:01M5A3HJ597DZ0WRSYH7QND1N0', '{"mediaFileId":"01M5A3HJ597DZ0WRSYH7QND1N0"}', 'done', 1792414370209, NULL, NULL, 1, 5, NULL, 1792414370209, 1792414370534);
INSERT INTO "jobs" VALUES (31, 'probe.keyframes', 1, 'keyframes:01M5A3HJ597DZ0WRSYHC3SYEEM', '{"mediaFileId":"01M5A3HJ597DZ0WRSYHC3SYEEM"}', 'done', 1792414370241, NULL, NULL, 1, 5, NULL, 1792414370241, 1792414370567);
INSERT INTO "jobs" VALUES (32, 'probe.keyframes', 1, 'keyframes:01M5A3HJ597DZ0WRSYHHNR1170', '{"mediaFileId":"01M5A3HJ597DZ0WRSYHHNR1170"}', 'done', 1792414370271, NULL, NULL, 1, 5, NULL, 1792414370271, 1792414370596);
CREATE TABLE job_schedules (
    type      TEXT    PRIMARY KEY,
    cron_expr TEXT    NOT NULL,
    next_run  INTEGER NOT NULL
) STRICT;
CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;
CREATE TABLE accounts (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL COLLATE NOCASE UNIQUE,
    password_hash TEXT    NOT NULL,            -- argon2id PHC string
    role          TEXT    NOT NULL DEFAULT 'owner' CHECK (role IN ('owner', 'guest')),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
) STRICT;
INSERT INTO "accounts" VALUES (1, 'frozen@example.test', '$argon2id$v=19$m=65536,t=3,p=4$o8AKOFKuOyCCmTVUtqq8gA$OtgLkNoawuLrOfKoqF+u4/x0LA7WJGr7RU7itJK2KBQ', 'owner', 1792414369456, 1792414369456);
CREATE TABLE profiles (
    id            INTEGER PRIMARY KEY,
    account_id    INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name          TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 60),
    image         TEXT,
    max_rating    INTEGER NOT NULL DEFAULT 18 CHECK (max_rating IN (0, 6, 12, 16, 18)),
    language      TEXT    NOT NULL DEFAULT 'de' CHECK (language IN ('de', 'en')),
    accent        TEXT,
    all_libraries INTEGER NOT NULL DEFAULT 1 CHECK (all_libraries IN (0, 1)),
    is_default    INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
) STRICT;
INSERT INTO "profiles" VALUES (1, 1, 'frozen', NULL, 18, 'de', NULL, 1, 1, 1792414369456, 1792414369456);
CREATE TABLE profile_libraries (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    library_id INTEGER NOT NULL,
    PRIMARY KEY (profile_id, library_id)
) STRICT, WITHOUT ROWID;
CREATE TABLE sessions (
    id           INTEGER PRIMARY KEY,
    token_hash   TEXT    NOT NULL UNIQUE,
    account_id   INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id   INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    user_agent   TEXT,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,
    revoked_at   INTEGER
) STRICT;
INSERT INTO "sessions" VALUES (1, '21a7bb4a8555559a9e4cb2e17469e8fc8db2abbac667b247378de25fe049e99e', 1, 1, 'curl/8.5.0', 1792414369456, 1792414369456, 1795006369456, NULL);
CREATE TABLE libraries (
    id                     INTEGER PRIMARY KEY,
    name                   TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    kind                   TEXT    NOT NULL CHECK (kind IN ('movies', 'series')),
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
INSERT INTO "libraries" VALUES (1, 'Filme', 'movies', 1792414369456, 1792414369961, 'done', 1792414369957, 1792414369961, NULL, 10, 10, NULL);
CREATE TABLE library_roots (
    id         INTEGER PRIMARY KEY,
    library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    path       TEXT    NOT NULL UNIQUE,
    fs_type    TEXT    NOT NULL DEFAULT 'unknown',
    watch      TEXT    NOT NULL DEFAULT 'poll' CHECK (watch IN ('inotify', 'poll', 'inotify_poll')),
    created_at INTEGER NOT NULL
) STRICT;
INSERT INTO "library_roots" VALUES (1, 1, '/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/.claude/worktrees/vigorous-lumiere-17beb0/testdata/media/Movies', 'ext4', 'inotify', 1792414369456);
CREATE TABLE items (
    id               TEXT    PRIMARY KEY CHECK (length(id) = 26),
    kind             TEXT    NOT NULL CHECK (kind IN ('movie', 'series', 'season', 'episode', 'extra',
                                                      'audiobook', 'podcast', 'podcast_episode')),
    parent_id        TEXT    REFERENCES items(id) ON DELETE CASCADE,
    library_id       INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    title            TEXT    NOT NULL,            -- parsed title, fallback when item_texts is empty
    sort_title       TEXT    NOT NULL,
    original_title   TEXT,
    year             INTEGER,
    release_date     TEXT,                        -- YYYY-MM-DD
    runtime_minutes  INTEGER,
    genres           TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(genres)),
    tmdb_id          INTEGER,
    imdb_id          TEXT,
    own_rating       INTEGER CHECK (own_rating IN (0, 6, 12, 16, 18)),
    effective_rating INTEGER CHECK (effective_rating IN (0, 6, 12, 16, 18)),
    min_child_rating INTEGER CHECK (min_child_rating IN (0, 6, 12, 16, 18)),
    available        INTEGER NOT NULL DEFAULT 0 CHECK (available IN (0, 1)),
    rating_source    TEXT,                        -- for example 'tmdb:DE', 'nfo', 'manual'
    enclosure_url    TEXT,                        -- podcast episodes (M6)
    locked_fields    TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(locked_fields)),
    metadata_state   TEXT    NOT NULL DEFAULT 'pending'
                     CHECK (metadata_state IN ('pending', 'matched', 'unmatched', 'failed', 'skipped')),
    metadata_error   TEXT,
    matched_at       INTEGER,
    added_at         INTEGER NOT NULL,            -- first time the scanner saw the title
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,
    deleted_at       INTEGER
) STRICT;
INSERT INTO "items" VALUES ('01M5A3HJ56J3YCNC53QZ4GWRHA', 'movie', NULL, 1, 'Slouchcat Test AC3 51', 'slouchcat test ac3 51', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792414369958, 1792414369958, 1792414369962, NULL);
INSERT INTO "items" VALUES ('01M5A3HJ56J3YCNC53R0K433HF', 'movie', NULL, 1, 'Slouchcat Test Chapters', 'slouchcat test chapters', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792414369958, 1792414369958, 1792414369962, NULL);
INSERT INTO "items" VALUES ('01M5A3HJ5761XX1JK34XBHBVYN', 'movie', NULL, 1, 'Slouchcat Test DTS', 'slouchcat test dts', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792414369958, 1792414369958, 1792414369962, NULL);
INSERT INTO "items" VALUES ('01M5A3HJ5761XX1JK34YAHC05B', 'movie', NULL, 1, 'Slouchcat Test H264', 'slouchcat test h264', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792414369958, 1792414369958, 1792414369962, NULL);
INSERT INTO "items" VALUES ('01M5A3HJ5761XX1JK34ZTABERS', 'movie', NULL, 1, 'Slouchcat Test H264 MKV Long', 'slouchcat test h264 mkv long', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792414369958, 1792414369958, 1792414369963, NULL);
INSERT INTO "items" VALUES ('01M5A3HJ58CS687HPN6SHXZMER', 'movie', NULL, 1, 'Slouchcat Test HDR10 4K', 'slouchcat test hdr10 4k', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792414369958, 1792414369958, 1792414369963, NULL);
INSERT INTO "items" VALUES ('01M5A3HJ58CS687HPN6YNN1GCX', 'movie', NULL, 1, 'Slouchcat Test HEVC 10bit', 'slouchcat test hevc 10bit', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792414369958, 1792414369958, 1792414369963, NULL);
INSERT INTO "items" VALUES ('01M5A3HJ58CS687HPN71S8YMGR', 'movie', NULL, 1, 'Slouchcat Test HEVC Long', 'slouchcat test hevc long', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792414369958, 1792414369958, 1792414369963, NULL);
INSERT INTO "items" VALUES ('01M5A3HJ597DZ0WRSYHANXNYWJ', 'movie', NULL, 1, 'Slouchcat Test Sidecar Subtitles', 'slouchcat test sidecar subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792414369958, 1792414369958, 1792414369963, NULL);
INSERT INTO "items" VALUES ('01M5A3HJ597DZ0WRSYHE0DG9QY', 'movie', NULL, 1, 'Slouchcat Test Subtitles', 'slouchcat test subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792414369958, 1792414369958, 1792414369963, NULL);
CREATE TABLE item_texts (
    item_id  TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    lang     TEXT NOT NULL CHECK (lang IN ('de', 'en')),
    title    TEXT NOT NULL,
    overview TEXT,
    tagline  TEXT,
    PRIMARY KEY (item_id, lang)
) STRICT, WITHOUT ROWID;
CREATE TABLE media_files (
    id            TEXT    PRIMARY KEY CHECK (length(id) = 26),
    item_id       TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    root_id       INTEGER NOT NULL REFERENCES library_roots(id) ON DELETE CASCADE,
    rel_path      TEXT    NOT NULL,
    dev           INTEGER NOT NULL DEFAULT 0,
    inode         INTEGER NOT NULL DEFAULT 0,
    size          INTEGER NOT NULL,
    mtime         INTEGER NOT NULL,
    partial_hash  TEXT,                            -- SHA-256 over first and last 64 KiB plus size
    version_label TEXT,
    container     TEXT    NOT NULL,                -- lower-case extension without dot
    video_codec   TEXT,
    audio_codec   TEXT,
    width         INTEGER,
    height        INTEGER,
    duration_ms   INTEGER,
    probe         TEXT    CHECK (probe IS NULL OR json_valid(probe)),
    keyframes     BLOB,                            -- keyframe index for remux seeking (M1b)
    probed_at     INTEGER,
    keyframes_at  INTEGER,
    missing_since INTEGER,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    UNIQUE (root_id, rel_path)
) STRICT;
INSERT INTO "media_files" VALUES ('01M5A3HJ56J3YCNC53R0D695FQ', '01M5A3HJ56J3YCNC53QZ4GWRHA', 1, 'Slouchcat Test AC3 51 (2024)/Slouchcat Test AC3 51 (2024).mkv', 66310, 3283890, 2943036, 1792411246321, 'c69e2e9494f6419924770e226128e3602bfae71d6ad4f72abe53861bd33d62e2', NULL, 'mkv', 'h264', 'ac3', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":2943036,"bitRate":2354428},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"ac3","channels":6,"channelLayout":"5.1(side)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792414369999, 1792414370304, NULL, 1792414369958, 1792414370304);
INSERT INTO "media_files" VALUES ('01M5A3HJ5761XX1JK34SM1V6P4', '01M5A3HJ56J3YCNC53R0K433HF', 1, 'Slouchcat Test Chapters (2024)/Slouchcat Test Chapters (2024).mkv', 66310, 3283902, 2623898, 1792411247944, '0f48550ea976bec79e5dd9074dba21d593b7c4c4f470066e90226b148732c362', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2623898,"bitRate":2094719},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[{"start":0,"end":3000000000,"title":"Kapitel 1: Anfang"},{"start":3000000000,"end":6500000000,"title":"Kapitel 2: Mitte"},{"start":6500000000,"end":10000000000,"title":"Kapitel 3: Ende"}]}', X'0100d00fd00fd00fd00f', 1792414370035, 1792414370345, NULL, 1792414369958, 1792414370345);
INSERT INTO "media_files" VALUES ('01M5A3HJ5761XX1JK34Y093ANY', '01M5A3HJ5761XX1JK34XBHBVYN', 1, 'Slouchcat Test DTS (2024)/Slouchcat Test DTS (2024).mkv', 66310, 3283892, 3427132, 1792411246825, '9e231c3fd69e3cdd46d98cde6b24afb7c48297ff9d43e1d5c12c06cd0f64cf7e', NULL, 'mkv', 'h264', 'dts', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":3427132,"bitRate":2741705},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"dts","profile":"DTS","channels":6,"channelLayout":"5.1(side)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792414370065, 1792414370379, NULL, 1792414369958, 1792414370379);
INSERT INTO "media_files" VALUES ('01M5A3HJ5761XX1JK34YTGKFBM', '01M5A3HJ5761XX1JK34YAHC05B', 1, 'Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024).mp4', 66310, 3283884, 2624977, 1792411234036, '1a3559a74acfe685061086a9246f1ad08d3f220ba78aad39e503e56f4400693b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624977,"bitRate":2099981},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792414370095, 1792414370410, NULL, 1792414369958, 1792414370410);
INSERT INTO "media_files" VALUES ('01M5A3HJ58CS687HPN6PXTS43R', '01M5A3HJ5761XX1JK34ZTABERS', 1, 'Slouchcat Test H264 MKV Long (2024)/Slouchcat Test H264 MKV Long (2024).mkv', 66310, 3285251, 13446607, 1792412317632, 'ca855ab6852bc7b417ff44fe557f1dc00d54919dc4c8f3c845a1312f90628b04', NULL, 'mkv', 'h264', 'aac', 1280, 720, 90021, '{"format":{"name":"matroska,webm","duration":90021000000,"size":13446607,"bitRate":1194975},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00f8827e807d836f02ea846e807b06d9875e807b06d', 1792414370122, 1792414370444, NULL, 1792414369958, 1792414370444);
INSERT INTO "media_files" VALUES ('01M5A3HJ58CS687HPN6V7HA8MK', '01M5A3HJ58CS687HPN6SHXZMER', 1, 'Slouchcat Test HDR10 4K (2024)/Slouchcat Test HDR10 4K (2024).mkv', 66310, 3283888, 761065, 1792411245770, '02ad0bbe638eba63a5d97891e5ea32f62dcf75b79eba35a22bed1db709776e77', '2160p', 'mkv', 'hevc', 'aac', 3840, 2160, 30021, '{"format":{"name":"matroska,webm","duration":30021000000,"size":761065,"bitRate":202808},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":150,"width":3840,"height":2160,"pixFmt":"yuv420p10le","colorTransfer":"smpte2084","colorPrimaries":"bt2020"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1792414370151, 1792414370472, NULL, 1792414369958, 1792414370472);
INSERT INTO "media_files" VALUES ('01M5A3HJ58CS687HPN6YPQHQAA', '01M5A3HJ58CS687HPN6YNN1GCX', 1, 'Slouchcat Test HEVC 10bit (2024)/Slouchcat Test HEVC 10bit (2024).mkv', 66310, 3283886, 2102531, 1792411234989, '42bc78f0ccd76914fef5a8db4229ebc16d12afedb1457462c89464488dfef9ab', NULL, 'mkv', 'hevc', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2102531,"bitRate":1678499},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":120,"width":1920,"height":1080,"pixFmt":"yuv420p10le","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792414370180, 1792414370502, NULL, 1792414369958, 1792414370502);
INSERT INTO "media_files" VALUES ('01M5A3HJ597DZ0WRSYH7QND1N0', '01M5A3HJ58CS687HPN71S8YMGR', 1, 'Slouchcat Test HEVC Long (2024)/Slouchcat Test HEVC Long (2024).mkv', 66310, 3285148, 9588128, 1792412314680, '74fd94f94bcae88f0a3e82ca35c808342a8395d3a536b4b097689d330384aa3b', NULL, 'mkv', 'hevc', 'aac', 1280, 720, 90021, '{"format":{"name":"matroska,webm","duration":90021000000,"size":9588128,"bitRate":852079},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":93,"width":1280,"height":720,"pixFmt":"yuv420p10le","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1792414370209, 1792414370534, NULL, 1792414369958, 1792414370534);
INSERT INTO "media_files" VALUES ('01M5A3HJ597DZ0WRSYHC3SYEEM', '01M5A3HJ597DZ0WRSYHANXNYWJ', 1, 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).mkv', 66310, 3283896, 2623761, 1792411247394, '93cbc6ba6715ccac44124abefe37c44be02669320647098058c3cddfb62a36e0', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10047, '{"format":{"name":"matroska,webm","duration":10047000000,"size":2623761,"bitRate":2089189},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0115d00fd00fd00fd00f', 1792414370241, 1792414370567, NULL, 1792414369958, 1792414370567);
INSERT INTO "media_files" VALUES ('01M5A3HJ597DZ0WRSYHHNR1170', '01M5A3HJ597DZ0WRSYHE0DG9QY', 1, 'Slouchcat Test Subtitles (2024)/Slouchcat Test Subtitles (2024).mkv', 66310, 3283894, 2626565, 1792411247349, '6187f53fc232946990df7f45b92419fa967b627a083a906d46e1c72459839fd2', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2626565,"bitRate":2096848},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"},{"index":2,"type":"subtitle","codec":"subrip","language":"ger","title":"Deutsch (SRT)","default":true},{"index":3,"type":"subtitle","codec":"subrip","language":"eng","title":"English (SRT)"},{"index":4,"type":"subtitle","codec":"ass","language":"ger","title":"Deutsch (ASS)"},{"index":5,"type":"subtitle","codec":"ass","language":"eng","title":"English (ASS)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792414370271, 1792414370596, NULL, 1792414369958, 1792414370596);
CREATE TABLE tracks (
    id            INTEGER PRIMARY KEY,
    media_file_id TEXT    NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
    stream_index  INTEGER NOT NULL,
    kind          TEXT    NOT NULL CHECK (kind IN ('video', 'audio', 'subtitle')),
    codec         TEXT,
    language      TEXT,
    title         TEXT,
    source        TEXT    NOT NULL DEFAULT 'embedded' CHECK (source IN ('embedded', 'sidecar')),
    rel_path      TEXT,                            -- sidecar file, relative to the root of the media file
    is_default    INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    is_forced     INTEGER NOT NULL DEFAULT 0 CHECK (is_forced IN (0, 1)),
    UNIQUE (media_file_id, source, stream_index, rel_path)
) STRICT;
INSERT INTO "tracks" VALUES (1, '01M5A3HJ56J3YCNC53R0D695FQ', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (2, '01M5A3HJ56J3YCNC53R0D695FQ', 1, 'audio', 'ac3', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (3, '01M5A3HJ5761XX1JK34SM1V6P4', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (4, '01M5A3HJ5761XX1JK34SM1V6P4', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (5, '01M5A3HJ5761XX1JK34Y093ANY', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (6, '01M5A3HJ5761XX1JK34Y093ANY', 1, 'audio', 'dts', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (7, '01M5A3HJ5761XX1JK34YTGKFBM', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (8, '01M5A3HJ5761XX1JK34YTGKFBM', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (9, '01M5A3HJ58CS687HPN6PXTS43R', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (10, '01M5A3HJ58CS687HPN6PXTS43R', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (11, '01M5A3HJ58CS687HPN6V7HA8MK', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (12, '01M5A3HJ58CS687HPN6V7HA8MK', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (13, '01M5A3HJ58CS687HPN6YPQHQAA', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (14, '01M5A3HJ58CS687HPN6YPQHQAA', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (15, '01M5A3HJ597DZ0WRSYH7QND1N0', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (16, '01M5A3HJ597DZ0WRSYH7QND1N0', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (17, '01M5A3HJ597DZ0WRSYHC3SYEEM', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (18, '01M5A3HJ597DZ0WRSYHC3SYEEM', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (19, '01M5A3HJ597DZ0WRSYHC3SYEEM', 0, 'subtitle', 'ass', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.ass', 0, 0);
INSERT INTO "tracks" VALUES (20, '01M5A3HJ597DZ0WRSYHC3SYEEM', 1, 'subtitle', 'subrip', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.srt', 0, 0);
INSERT INTO "tracks" VALUES (21, '01M5A3HJ597DZ0WRSYHC3SYEEM', 2, 'subtitle', 'ass', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.ass', 0, 0);
INSERT INTO "tracks" VALUES (22, '01M5A3HJ597DZ0WRSYHC3SYEEM', 3, 'subtitle', 'subrip', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.srt', 0, 0);
INSERT INTO "tracks" VALUES (23, '01M5A3HJ597DZ0WRSYHHNR1170', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (24, '01M5A3HJ597DZ0WRSYHHNR1170', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (25, '01M5A3HJ597DZ0WRSYHHNR1170', 2, 'subtitle', 'subrip', 'ger', 'Deutsch (SRT)', 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (26, '01M5A3HJ597DZ0WRSYHHNR1170', 3, 'subtitle', 'subrip', 'eng', 'English (SRT)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (27, '01M5A3HJ597DZ0WRSYHHNR1170', 4, 'subtitle', 'ass', 'ger', 'Deutsch (ASS)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (28, '01M5A3HJ597DZ0WRSYHHNR1170', 5, 'subtitle', 'ass', 'eng', 'English (ASS)', 'embedded', NULL, 0, 0);
CREATE TABLE top_picks (
    id         INTEGER PRIMARY KEY,
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    profile_id INTEGER REFERENCES profiles(id) ON DELETE CASCADE, -- NULL = all profiles
    position   INTEGER NOT NULL DEFAULT 0,
    note       TEXT,
    starts_at  INTEGER,
    ends_at    INTEGER,
    created_at INTEGER NOT NULL
) STRICT;
CREATE TABLE progress (
    profile_id  INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id     TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    position_ms INTEGER NOT NULL CHECK (position_ms >= 0),
    duration_ms INTEGER NOT NULL CHECK (duration_ms >= 0),
    completed   INTEGER NOT NULL DEFAULT 0 CHECK (completed IN (0, 1)),
    updated_at  INTEGER NOT NULL,
    PRIMARY KEY (profile_id, item_id)
) STRICT, WITHOUT ROWID;
INSERT INTO "progress" VALUES (1, '01M5A3HJ58CS687HPN71S8YMGR', 20000, 90000, 0, 1792414382561);
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
INSERT INTO "progress_events" VALUES (1, 1, '01M5A3HJ58CS687HPN71S8YMGR', NULL, NULL, 20000, 90000, 20000, 1, 'paused', 1792422000000, 1792414382561);
CREATE TABLE playback_sessions (
    id               TEXT    PRIMARY KEY CHECK (length(id) = 26), -- ULID
    session_id       INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    account_id       INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id       INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id          TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    media_file_id    TEXT    NOT NULL,
    device_id        TEXT,
    mode             TEXT    NOT NULL CHECK (mode IN ('direct', 'remux', 'transcode')),
    backend          TEXT,                                         -- transcode backend, NULL for direct
    reason           TEXT,                                         -- why not direct play
    started_at       INTEGER NOT NULL,
    last_activity_at INTEGER NOT NULL,
    expires_at       INTEGER NOT NULL,
    revoked_at       INTEGER
) STRICT;
INSERT INTO "playback_sessions" VALUES ('01M5A3HYDF9HVPZ0994MSF47JJ', 1, 1, 1, '01M5A3HJ58CS687HPN71S8YMGR', '01M5A3HJ597DZ0WRSYH7QND1N0', 'frozen', 'transcode', 'nvenc', 'video_codec', 1792414382511, 1792414382511, 1792457582511, 1792414382566);
CREATE TABLE "images" (
    id           INTEGER PRIMARY KEY,
    item_id      TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    kind         TEXT    NOT NULL CHECK (kind IN ('poster', 'backdrop')),
    size         TEXT    NOT NULL,                 -- w200 … w1920 or 'original'
    format       TEXT    NOT NULL DEFAULT 'jpeg' CHECK (format IN ('jpeg', 'webp')),
    lang         TEXT,
    source_url   TEXT    NOT NULL,                 -- URL of the TMDB original the row derives from
    rel_path     TEXT    NOT NULL,                 -- relative to the cache root
    content_hash TEXT    NOT NULL,                 -- SHA-256 hex, cache busting only
    media_type   TEXT    NOT NULL,                 -- image/jpeg, image/png, image/webp
    width        INTEGER NOT NULL,
    height       INTEGER NOT NULL,
    bytes        INTEGER NOT NULL,
    accent       TEXT    CHECK (accent IS NULL OR (length(accent) = 7 AND accent GLOB '#[0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f]')),
    created_at   INTEGER NOT NULL,
    UNIQUE (item_id, kind, size, format)
) STRICT;
CREATE UNIQUE INDEX jobs_active_idempotency_key ON jobs (idempotency_key)
    WHERE state IN ('queued', 'running');
CREATE INDEX jobs_claim ON jobs (state, type, priority DESC, created_at, id);
CREATE INDEX profiles_account ON profiles (account_id);
CREATE INDEX sessions_account ON sessions (account_id);
CREATE INDEX library_roots_library ON library_roots (library_id);
CREATE INDEX items_library_kind ON items (library_id, kind, deleted_at);
CREATE INDEX items_parent ON items (parent_id);
CREATE INDEX items_added ON items (added_at DESC);
CREATE INDEX items_tmdb ON items (tmdb_id);
CREATE INDEX items_imdb ON items (imdb_id);
CREATE INDEX media_files_item ON media_files (item_id);
CREATE INDEX media_files_identity ON media_files (root_id, dev, inode, size);
CREATE INDEX media_files_hash ON media_files (partial_hash);
CREATE INDEX media_files_missing ON media_files (missing_since);
CREATE INDEX tracks_media_file ON tracks (media_file_id);
CREATE INDEX top_picks_order ON top_picks (position, id);
CREATE VIEW visible_items AS
SELECT p.id AS profile_id, i.id AS item_id
FROM profiles AS p
JOIN items AS i
  ON i.deleted_at IS NULL
 AND i.available = 1
 AND COALESCE(i.effective_rating, 18) <= p.max_rating
 AND (i.kind NOT IN ('series', 'season', 'podcast') OR COALESCE(i.min_child_rating, 18) <= p.max_rating)
WHERE p.all_libraries = 1
   OR EXISTS (SELECT 1 FROM profile_libraries AS pl
              WHERE pl.profile_id = p.id AND pl.library_id = i.library_id);
CREATE TRIGGER items_rating_insert AFTER INSERT ON items
BEGIN
    UPDATE items SET effective_rating = CASE
        WHEN NEW.parent_id IS NULL THEN NEW.own_rating
        WHEN (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id) IS NULL THEN NULL
        WHEN NEW.own_rating IS NULL THEN (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id)
        ELSE max(NEW.own_rating, (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id)) END,
        available = CASE WHEN NEW.kind = 'podcast_episode' THEN (NEW.enclosure_url IS NOT NULL) ELSE available END
    WHERE id = NEW.id;
END;
CREATE TRIGGER items_rating_update AFTER UPDATE OF own_rating, parent_id ON items
BEGIN
    -- level 0: the row itself
    UPDATE items SET effective_rating = CASE
        WHEN NEW.parent_id IS NULL THEN NEW.own_rating
        WHEN (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id) IS NULL THEN NULL
        WHEN NEW.own_rating IS NULL THEN (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id)
        ELSE max(NEW.own_rating, (SELECT p.effective_rating FROM items AS p WHERE p.id = NEW.parent_id)) END
    WHERE id = NEW.id;
    -- level 1: children
    UPDATE items SET effective_rating = CASE
        WHEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id) IS NULL THEN NULL
        WHEN own_rating IS NULL THEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)
        ELSE max(own_rating, (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)) END
    WHERE parent_id = NEW.id;
    -- level 2: grandchildren
    UPDATE items SET effective_rating = CASE
        WHEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id) IS NULL THEN NULL
        WHEN own_rating IS NULL THEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)
        ELSE max(own_rating, (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)) END
    WHERE parent_id IN (SELECT c.id FROM items AS c WHERE c.parent_id = NEW.id);
    -- level 3: great-grandchildren
    UPDATE items SET effective_rating = CASE
        WHEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id) IS NULL THEN NULL
        WHEN own_rating IS NULL THEN (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)
        ELSE max(own_rating, (SELECT p.effective_rating FROM items AS p WHERE p.id = items.parent_id)) END
    WHERE parent_id IN (SELECT g.id FROM items AS g
                        WHERE g.parent_id IN (SELECT c.id FROM items AS c WHERE c.parent_id = NEW.id));
    -- the previous parent (re-parenting) recomputes its container state
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1)
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1)
    WHERE id = OLD.parent_id AND OLD.parent_id IS NOT NEW.parent_id;
END;
CREATE TRIGGER items_enclosure_update AFTER UPDATE OF enclosure_url ON items
WHEN NEW.kind = 'podcast_episode'
BEGIN
    UPDATE items SET available = (NEW.enclosure_url IS NOT NULL) WHERE id = NEW.id;
END;
CREATE TRIGGER media_files_insert AFTER INSERT ON media_files
BEGIN
    UPDATE items SET available = EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since IS NULL)
    WHERE id = NEW.item_id AND kind IN ('movie', 'episode', 'extra', 'audiobook');
END;
CREATE TRIGGER media_files_update AFTER UPDATE OF missing_since, item_id ON media_files
BEGIN
    UPDATE items SET available = EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since IS NULL)
    WHERE id IN (NEW.item_id, OLD.item_id) AND kind IN ('movie', 'episode', 'extra', 'audiobook');
END;
CREATE TRIGGER media_files_delete AFTER DELETE ON media_files
BEGIN
    UPDATE items SET available = EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since IS NULL)
    WHERE id = OLD.item_id AND kind IN ('movie', 'episode', 'extra', 'audiobook');
END;
CREATE TRIGGER items_container_update AFTER UPDATE OF available, effective_rating, deleted_at ON items
WHEN NEW.parent_id IS NOT NULL
BEGIN
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1)
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1)
    WHERE id = NEW.parent_id;
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1)
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1)
    WHERE id = (SELECT p.parent_id FROM items AS p WHERE p.id = NEW.parent_id);
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1)
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1)
    WHERE id = (SELECT g.parent_id FROM items AS g
                WHERE g.id = (SELECT p.parent_id FROM items AS p WHERE p.id = NEW.parent_id));
END;
CREATE TRIGGER items_container_delete AFTER DELETE ON items
WHEN OLD.parent_id IS NOT NULL
BEGIN
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1)
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1)
    WHERE id = OLD.parent_id;
END;
CREATE INDEX progress_recent ON progress (profile_id, updated_at DESC);
CREATE INDEX progress_events_profile_item ON progress_events (profile_id, item_id, server_time DESC);
CREATE INDEX playback_sessions_session ON playback_sessions (session_id);
CREATE INDEX playback_sessions_expires ON playback_sessions (expires_at);
