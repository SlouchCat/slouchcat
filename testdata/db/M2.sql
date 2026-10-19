CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	);
INSERT INTO "goose_db_version" VALUES (1, 0, 1, '2026-10-19 16:04:21 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (2, 1, 1, '2026-10-19 16:04:21 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (3, 2, 1, '2026-10-19 16:04:21 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (4, 3, 1, '2026-10-19 16:04:21 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (5, 4, 1, '2026-10-19 16:04:21 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (6, 5, 1, '2026-10-19 16:04:21 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (7, 6, 1, '2026-10-19 16:04:21 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (8, 7, 1, '2026-10-19 16:04:21 +0000 UTC');
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
INSERT INTO "jobs" VALUES (1, 'library.sweep', 2, 'sweep', '{}', 'done', 1792425861953, NULL, NULL, 1, 5, NULL, 1792425861953, 1792425861953);
INSERT INTO "jobs" VALUES (2, 'scan.library', 5, 'scan:1', '{"libraryId":1}', 'done', 1792425873446, NULL, NULL, 1, 5, NULL, 1792425873446, 1792425873967);
INSERT INTO "jobs" VALUES (3, 'metadata.match', 4, 'match:01M5AEGMHC71HJ486FAS74JTFB', '{"itemId":"01M5AEGMHC71HJ486FAS74JTFB"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425873968);
INSERT INTO "jobs" VALUES (4, 'probe.file', 3, 'probe:01M5AEGMHD6P05YDYD5R4066Y7', '{"mediaFileId":"01M5AEGMHD6P05YDYD5R4066Y7"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425874000);
INSERT INTO "jobs" VALUES (5, 'metadata.match', 4, 'match:01M5AEGMHD6P05YDYD5TM2S08V', '{"itemId":"01M5AEGMHD6P05YDYD5TM2S08V"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425873968);
INSERT INTO "jobs" VALUES (6, 'probe.file', 3, 'probe:01M5AEGMHD6P05YDYD5XVZC03V', '{"mediaFileId":"01M5AEGMHD6P05YDYD5XVZC03V"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425874030);
INSERT INTO "jobs" VALUES (7, 'metadata.match', 4, 'match:01M5AEGMHD6P05YDYD5Y81V684', '{"itemId":"01M5AEGMHD6P05YDYD5Y81V684"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425873968);
INSERT INTO "jobs" VALUES (8, 'probe.file', 3, 'probe:01M5AEGMHD6P05YDYD5ZW7N54V', '{"mediaFileId":"01M5AEGMHD6P05YDYD5ZW7N54V"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425874058);
INSERT INTO "jobs" VALUES (9, 'metadata.match', 4, 'match:01M5AEGMHEY3P51VEJST0Q4W1H', '{"itemId":"01M5AEGMHEY3P51VEJST0Q4W1H"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425873968);
INSERT INTO "jobs" VALUES (10, 'probe.file', 3, 'probe:01M5AEGMHEY3P51VEJSY0221HT', '{"mediaFileId":"01M5AEGMHEY3P51VEJSY0221HT"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425874087);
INSERT INTO "jobs" VALUES (11, 'metadata.match', 4, 'match:01M5AEGMHEY3P51VEJSZ6JPSGC', '{"itemId":"01M5AEGMHEY3P51VEJSZ6JPSGC"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425873968);
INSERT INTO "jobs" VALUES (12, 'probe.file', 3, 'probe:01M5AEGMHEY3P51VEJT0F2T3FD', '{"mediaFileId":"01M5AEGMHEY3P51VEJT0F2T3FD"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425874114);
INSERT INTO "jobs" VALUES (13, 'metadata.match', 4, 'match:01M5AEGMHEY3P51VEJT3ST667B', '{"itemId":"01M5AEGMHEY3P51VEJT3ST667B"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425873969);
INSERT INTO "jobs" VALUES (14, 'probe.file', 3, 'probe:01M5AEGMHEY3P51VEJT4XP06A5', '{"mediaFileId":"01M5AEGMHEY3P51VEJT4XP06A5"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425874145);
INSERT INTO "jobs" VALUES (15, 'metadata.match', 4, 'match:01M5AEGMHFXX1852WHWF3BJNBW', '{"itemId":"01M5AEGMHFXX1852WHWF3BJNBW"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425873969);
INSERT INTO "jobs" VALUES (16, 'probe.file', 3, 'probe:01M5AEGMHFXX1852WHWHM6GT8N', '{"mediaFileId":"01M5AEGMHFXX1852WHWHM6GT8N"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425874175);
INSERT INTO "jobs" VALUES (17, 'metadata.match', 4, 'match:01M5AEGMHFXX1852WHWJWHW8Q6', '{"itemId":"01M5AEGMHFXX1852WHWJWHW8Q6"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425873969);
INSERT INTO "jobs" VALUES (18, 'probe.file', 3, 'probe:01M5AEGMHFXX1852WHWK8Y1YPD', '{"mediaFileId":"01M5AEGMHFXX1852WHWK8Y1YPD"}', 'done', 1792425873964, NULL, NULL, 1, 5, NULL, 1792425873964, 1792425874205);
INSERT INTO "jobs" VALUES (19, 'probe.keyframes', 1, 'keyframes:01M5AEGMHD6P05YDYD5R4066Y7', '{"mediaFileId":"01M5AEGMHD6P05YDYD5R4066Y7"}', 'done', 1792425874000, NULL, NULL, 1, 5, NULL, 1792425874000, 1792425874245);
INSERT INTO "jobs" VALUES (20, 'probe.keyframes', 1, 'keyframes:01M5AEGMHD6P05YDYD5XVZC03V', '{"mediaFileId":"01M5AEGMHD6P05YDYD5XVZC03V"}', 'done', 1792425874030, NULL, NULL, 1, 5, NULL, 1792425874030, 1792425874275);
INSERT INTO "jobs" VALUES (21, 'probe.keyframes', 1, 'keyframes:01M5AEGMHD6P05YDYD5ZW7N54V', '{"mediaFileId":"01M5AEGMHD6P05YDYD5ZW7N54V"}', 'done', 1792425874058, NULL, NULL, 1, 5, NULL, 1792425874058, 1792425874304);
INSERT INTO "jobs" VALUES (22, 'probe.keyframes', 1, 'keyframes:01M5AEGMHEY3P51VEJSY0221HT', '{"mediaFileId":"01M5AEGMHEY3P51VEJSY0221HT"}', 'done', 1792425874086, NULL, NULL, 1, 5, NULL, 1792425874086, 1792425874335);
INSERT INTO "jobs" VALUES (23, 'probe.keyframes', 1, 'keyframes:01M5AEGMHEY3P51VEJT0F2T3FD', '{"mediaFileId":"01M5AEGMHEY3P51VEJT0F2T3FD"}', 'done', 1792425874114, NULL, NULL, 1, 5, NULL, 1792425874114, 1792425874366);
INSERT INTO "jobs" VALUES (24, 'probe.keyframes', 1, 'keyframes:01M5AEGMHEY3P51VEJT4XP06A5', '{"mediaFileId":"01M5AEGMHEY3P51VEJT4XP06A5"}', 'done', 1792425874145, NULL, NULL, 1, 5, NULL, 1792425874145, 1792425874395);
INSERT INTO "jobs" VALUES (25, 'probe.keyframes', 1, 'keyframes:01M5AEGMHFXX1852WHWHM6GT8N', '{"mediaFileId":"01M5AEGMHFXX1852WHWHM6GT8N"}', 'done', 1792425874175, NULL, NULL, 1, 5, NULL, 1792425874175, 1792425874425);
INSERT INTO "jobs" VALUES (26, 'probe.keyframes', 1, 'keyframes:01M5AEGMHFXX1852WHWK8Y1YPD', '{"mediaFileId":"01M5AEGMHFXX1852WHWK8Y1YPD"}', 'done', 1792425874204, NULL, NULL, 1, 5, NULL, 1792425874204, 1792425874464);
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
, max_rating INTEGER NOT NULL DEFAULT 18 CHECK (max_rating IN (0, 6, 12, 16, 18)), all_libraries INTEGER NOT NULL DEFAULT 1 CHECK (all_libraries IN (0, 1)), invited_by INTEGER REFERENCES accounts(id) ON DELETE SET NULL, last_seen_at INTEGER) STRICT;
INSERT INTO "accounts" VALUES (1, 'frozen@example.test', '$argon2id$v=19$m=65536,t=3,p=4$rRQeq3zy/9m9HOokQtFsEQ$Je/uJzHx6lFoqBQlsCpUbceOQINNTb9xX+GhmTbCRDA', 'owner', 1792425873446, 1792425873446, 18, 1, NULL, NULL);
INSERT INTO "accounts" VALUES (2, 'frozen-guest@example.test', '$argon2id$v=19$m=65536,t=3,p=4$Bo+7YPBI6RFWMz43cODZZQ$pkLs9ycIq/RaF7Wuz1VMsrtn3p3EimWGDBO+8K467R4', 'guest', 1792425875583, 1792425875583, 12, 0, 1, NULL);
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
, pin_hash TEXT) STRICT;
INSERT INTO "profiles" VALUES (1, 1, 'frozen', NULL, 18, 'de', NULL, 1, 1, 1792425873446, 1792425873446, NULL);
INSERT INTO "profiles" VALUES (2, 1, 'Kind', NULL, 6, 'de', 'teal', 1, 0, 1792425875513, 1792425875513, '$argon2id$v=19$m=65536,t=3,p=4$QlCVKHgyKBTNDRmgZVO5hQ$fkZTExJM65zZfZA2zXhaCm5aEfTMdieiwWQDujRMLQU');
INSERT INTO "profiles" VALUES (3, 1, 'Teenager', NULL, 12, 'en', 'violet', 1, 0, 1792425875519, 1792425875519, NULL);
INSERT INTO "profiles" VALUES (4, 2, 'Oma', NULL, 12, 'de', NULL, 1, 1, 1792425875583, 1792425875583, NULL);
INSERT INTO "profiles" VALUES (5, 2, 'Enkel', NULL, 6, 'de', NULL, 0, 0, 1792425875588, 1792425875588, NULL);
CREATE TABLE profile_libraries (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    library_id INTEGER NOT NULL,
    PRIMARY KEY (profile_id, library_id)
) STRICT, WITHOUT ROWID;
INSERT INTO "profile_libraries" VALUES (5, 1);
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
INSERT INTO "libraries" VALUES (1, 'Filme', 'movies', 1792425873446, 1792425873967, 'done', 1792425873963, 1792425873967, NULL, 8, 8, NULL);
CREATE TABLE library_roots (
    id         INTEGER PRIMARY KEY,
    library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    path       TEXT    NOT NULL UNIQUE,
    fs_type    TEXT    NOT NULL DEFAULT 'unknown',
    watch      TEXT    NOT NULL DEFAULT 'poll' CHECK (watch IN ('inotify', 'poll', 'inotify_poll')),
    created_at INTEGER NOT NULL
) STRICT;
INSERT INTO "library_roots" VALUES (1, 1, '/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/testdata/media/Movies', 'ext4', 'inotify', 1792425873446);
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
INSERT INTO "items" VALUES ('01M5AEGMHC71HJ486FAS74JTFB', 'movie', NULL, 1, 'Slouchcat Test AC3 51', 'slouchcat test ac3 51', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792425873964, 1792425873964, 1792425873968, NULL);
INSERT INTO "items" VALUES ('01M5AEGMHD6P05YDYD5TM2S08V', 'movie', NULL, 1, 'Slouchcat Test Chapters', 'slouchcat test chapters', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792425873964, 1792425873964, 1792425873968, NULL);
INSERT INTO "items" VALUES ('01M5AEGMHD6P05YDYD5Y81V684', 'movie', NULL, 1, 'Slouchcat Test DTS', 'slouchcat test dts', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792425873964, 1792425873964, 1792425873968, NULL);
INSERT INTO "items" VALUES ('01M5AEGMHEY3P51VEJST0Q4W1H', 'movie', NULL, 1, 'Slouchcat Test H264', 'slouchcat test h264', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792425873964, 1792425873964, 1792425873968, NULL);
INSERT INTO "items" VALUES ('01M5AEGMHEY3P51VEJSZ6JPSGC', 'movie', NULL, 1, 'Slouchcat Test HDR10 4K', 'slouchcat test hdr10 4k', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792425873964, 1792425873964, 1792425873968, NULL);
INSERT INTO "items" VALUES ('01M5AEGMHEY3P51VEJT3ST667B', 'movie', NULL, 1, 'Slouchcat Test HEVC 10bit', 'slouchcat test hevc 10bit', NULL, 2024, NULL, NULL, '[]', NULL, NULL, 16, 16, NULL, 1, 'manual', NULL, '["own_rating"]', 'skipped', 'no_tmdb_key', NULL, 1792425873964, 1792425873964, 1792425885668, NULL);
INSERT INTO "items" VALUES ('01M5AEGMHFXX1852WHWF3BJNBW', 'movie', NULL, 1, 'Slouchcat Test Sidecar Subtitles', 'slouchcat test sidecar subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, 12, 12, NULL, 1, 'manual', NULL, '["own_rating"]', 'skipped', 'no_tmdb_key', NULL, 1792425873964, 1792425873964, 1792425885662, NULL);
INSERT INTO "items" VALUES ('01M5AEGMHFXX1852WHWJWHW8Q6', 'movie', NULL, 1, 'Slouchcat Test Subtitles', 'slouchcat test subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, 6, 6, NULL, 1, 'manual', NULL, '["own_rating"]', 'skipped', 'no_tmdb_key', NULL, 1792425873964, 1792425873964, 1792425885658, NULL);
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
INSERT INTO "media_files" VALUES ('01M5AEGMHD6P05YDYD5R4066Y7', '01M5AEGMHC71HJ486FAS74JTFB', 1, 'Slouchcat Test AC3 51 (2024)/Slouchcat Test AC3 51 (2024).mkv', 66310, 2121821, 2943036, 1792345527307, 'b2e891bc9da05c622e0a18ac5bdf0e040d400465fb1b84c586f905794c33a735', NULL, 'mkv', 'h264', 'ac3', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":2943036,"bitRate":2354428},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"ac3","channels":6,"channelLayout":"5.1(side)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792425874000, 1792425874245, NULL, 1792425873964, 1792425874245);
INSERT INTO "media_files" VALUES ('01M5AEGMHD6P05YDYD5XVZC03V', '01M5AEGMHD6P05YDYD5TM2S08V', 1, 'Slouchcat Test Chapters (2024)/Slouchcat Test Chapters (2024).mkv', 66310, 2121833, 2623898, 1792345529388, 'c2bb87ccf56f8bb95c24c9b0b9cbd63020f2aef72cb50c08fee9773851895722', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2623898,"bitRate":2094719},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[{"start":0,"end":3000000000,"title":"Kapitel 1: Anfang"},{"start":3000000000,"end":6500000000,"title":"Kapitel 2: Mitte"},{"start":6500000000,"end":10000000000,"title":"Kapitel 3: Ende"}]}', X'0100d00fd00fd00fd00f', 1792425874030, 1792425874275, NULL, 1792425873964, 1792425874275);
INSERT INTO "media_files" VALUES ('01M5AEGMHD6P05YDYD5ZW7N54V', '01M5AEGMHD6P05YDYD5Y81V684', 1, 'Slouchcat Test DTS (2024)/Slouchcat Test DTS (2024).mkv', 66310, 2121823, 3427132, 1792345527962, 'a0630ec8242c388a6440b8dd84cdc8221d56f16bfe33809f0c0dfc1b7e805d54', NULL, 'mkv', 'h264', 'dts', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":3427132,"bitRate":2741705},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"dts","profile":"DTS","channels":6,"channelLayout":"5.1(side)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792425874058, 1792425874304, NULL, 1792425873964, 1792425874304);
INSERT INTO "media_files" VALUES ('01M5AEGMHEY3P51VEJSY0221HT', '01M5AEGMHEY3P51VEJST0Q4W1H', 1, 'Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024).mp4', 66310, 2121811, 2624977, 1792345514047, '1a3559a74acfe685061086a9246f1ad08d3f220ba78aad39e503e56f4400693b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624977,"bitRate":2099981},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792425874086, 1792425874335, NULL, 1792425873964, 1792425874335);
INSERT INTO "media_files" VALUES ('01M5AEGMHEY3P51VEJT0F2T3FD', '01M5AEGMHEY3P51VEJSZ6JPSGC', 1, 'Slouchcat Test HDR10 4K (2024)/Slouchcat Test HDR10 4K (2024).mkv', 66310, 2121816, 761065, 1792345526738, '4f5e90e46dfab8b657b36c94ac139833c1bef0954ccc3750969aedb3d4547f86', '2160p', 'mkv', 'hevc', 'aac', 3840, 2160, 30021, '{"format":{"name":"matroska,webm","duration":30021000000,"size":761065,"bitRate":202808},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":150,"width":3840,"height":2160,"pixFmt":"yuv420p10le","colorTransfer":"smpte2084","colorPrimaries":"bt2020"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1792425874114, 1792425874366, NULL, 1792425873964, 1792425874366);
INSERT INTO "media_files" VALUES ('01M5AEGMHEY3P51VEJT4XP06A5', '01M5AEGMHEY3P51VEJT3ST667B', 1, 'Slouchcat Test HEVC 10bit (2024)/Slouchcat Test HEVC 10bit (2024).mkv', 66310, 2121814, 2102531, 1792345515157, '2469bfd53f3d50de5dd084a6dff5c3263c0d36fc93643768ce16469bfd62197a', NULL, 'mkv', 'hevc', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2102531,"bitRate":1678499},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":120,"width":1920,"height":1080,"pixFmt":"yuv420p10le","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792425874145, 1792425874395, NULL, 1792425873964, 1792425874395);
INSERT INTO "media_files" VALUES ('01M5AEGMHFXX1852WHWHM6GT8N', '01M5AEGMHFXX1852WHWF3BJNBW', 1, 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).mkv', 66310, 2121827, 2623761, 1792345528684, 'ddf64e1615182551e4bb7121f07c190c691f6b99117d3146e49e27d845311ec8', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10047, '{"format":{"name":"matroska,webm","duration":10047000000,"size":2623761,"bitRate":2089189},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0115d00fd00fd00fd00f', 1792425874175, 1792425874425, NULL, 1792425873964, 1792425874425);
INSERT INTO "media_files" VALUES ('01M5AEGMHFXX1852WHWK8Y1YPD', '01M5AEGMHFXX1852WHWJWHW8Q6', 1, 'Slouchcat Test Subtitles (2024)/Slouchcat Test Subtitles (2024).mkv', 66310, 2121825, 2626565, 1792345528629, '369b617c7d7ad81e45b193ddbad680f77b77593fc0a095272d6199ab77521204', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2626565,"bitRate":2096848},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"},{"index":2,"type":"subtitle","codec":"subrip","language":"ger","title":"Deutsch (SRT)","default":true},{"index":3,"type":"subtitle","codec":"subrip","language":"eng","title":"English (SRT)"},{"index":4,"type":"subtitle","codec":"ass","language":"ger","title":"Deutsch (ASS)"},{"index":5,"type":"subtitle","codec":"ass","language":"eng","title":"English (ASS)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792425874204, 1792425874464, NULL, 1792425873964, 1792425874464);
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
INSERT INTO "tracks" VALUES (1, '01M5AEGMHD6P05YDYD5R4066Y7', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (2, '01M5AEGMHD6P05YDYD5R4066Y7', 1, 'audio', 'ac3', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (3, '01M5AEGMHD6P05YDYD5XVZC03V', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (4, '01M5AEGMHD6P05YDYD5XVZC03V', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (5, '01M5AEGMHD6P05YDYD5ZW7N54V', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (6, '01M5AEGMHD6P05YDYD5ZW7N54V', 1, 'audio', 'dts', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (7, '01M5AEGMHEY3P51VEJSY0221HT', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (8, '01M5AEGMHEY3P51VEJSY0221HT', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (9, '01M5AEGMHEY3P51VEJT0F2T3FD', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (10, '01M5AEGMHEY3P51VEJT0F2T3FD', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (11, '01M5AEGMHEY3P51VEJT4XP06A5', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (12, '01M5AEGMHEY3P51VEJT4XP06A5', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (13, '01M5AEGMHFXX1852WHWHM6GT8N', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (14, '01M5AEGMHFXX1852WHWHM6GT8N', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (15, '01M5AEGMHFXX1852WHWHM6GT8N', 0, 'subtitle', 'ass', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.ass', 0, 0);
INSERT INTO "tracks" VALUES (16, '01M5AEGMHFXX1852WHWHM6GT8N', 1, 'subtitle', 'subrip', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.srt', 0, 0);
INSERT INTO "tracks" VALUES (17, '01M5AEGMHFXX1852WHWHM6GT8N', 2, 'subtitle', 'ass', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.ass', 0, 0);
INSERT INTO "tracks" VALUES (18, '01M5AEGMHFXX1852WHWHM6GT8N', 3, 'subtitle', 'subrip', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.srt', 0, 0);
INSERT INTO "tracks" VALUES (19, '01M5AEGMHFXX1852WHWK8Y1YPD', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (20, '01M5AEGMHFXX1852WHWK8Y1YPD', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (21, '01M5AEGMHFXX1852WHWK8Y1YPD', 2, 'subtitle', 'subrip', 'ger', 'Deutsch (SRT)', 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (22, '01M5AEGMHFXX1852WHWK8Y1YPD', 3, 'subtitle', 'subrip', 'eng', 'English (SRT)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (23, '01M5AEGMHFXX1852WHWK8Y1YPD', 4, 'subtitle', 'ass', 'ger', 'Deutsch (ASS)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (24, '01M5AEGMHFXX1852WHWK8Y1YPD', 5, 'subtitle', 'ass', 'eng', 'English (ASS)', 'embedded', NULL, 0, 0);
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
CREATE TABLE account_libraries (
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    PRIMARY KEY (account_id, library_id)
) STRICT, WITHOUT ROWID;
INSERT INTO "account_libraries" VALUES (2, 1);
CREATE TABLE devices (
    id           TEXT    PRIMARY KEY CHECK (length(id) = 32),
    user_agent   TEXT,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL
) STRICT;
INSERT INTO "devices" VALUES ('7899b3a01277538461f1a605f518ca81', 'curl/8.5.0', 1792425873446, 1792425873446);
INSERT INTO "devices" VALUES ('51aa9a722a86c8c38776f8424802e100', 'curl/8.5.0', 1792425875583, 1792425875583);
CREATE TABLE pin_attempts (
    profile_id   INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    device_id    TEXT    NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    failures     INTEGER NOT NULL DEFAULT 0,
    locked_until INTEGER NOT NULL DEFAULT 0,
    updated_at   INTEGER NOT NULL,
    PRIMARY KEY (profile_id, device_id)
) STRICT, WITHOUT ROWID;
CREATE TABLE "sessions" (
    id           INTEGER PRIMARY KEY,
    token_hash   TEXT    NOT NULL UNIQUE,
    account_id   INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id   INTEGER REFERENCES profiles(id) ON DELETE SET NULL,
    device_id    TEXT    REFERENCES devices(id) ON DELETE SET NULL,
    user_agent   TEXT,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,
    revoked_at   INTEGER
) STRICT;
INSERT INTO "sessions" VALUES (1, '21ebc4b7b9732374ac98e78bcdb5de7276647c447d8ac9e22d8cfa283afb8490', 1, 1, '7899b3a01277538461f1a605f518ca81', 'curl/8.5.0', 1792425873446, 1792425873446, 1795017873446, NULL);
INSERT INTO "sessions" VALUES (2, '7b6ed3221e7482250a46b7946f596e94316db8b31136687b7d45f30eb61d009d', 2, 4, '51aa9a722a86c8c38776f8424802e100', 'curl/8.5.0', 1792425875583, 1792425875583, 1795017875583, NULL);
CREATE TABLE invitations (
    id            INTEGER PRIMARY KEY,
    token_hash    TEXT    NOT NULL UNIQUE,             -- SHA-256 hex of the plain token
    note          TEXT,
    max_rating    INTEGER NOT NULL CHECK (max_rating IN (0, 6, 12, 16, 18)),
    all_libraries INTEGER NOT NULL CHECK (all_libraries IN (0, 1)),
    created_by    INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    created_at    INTEGER NOT NULL,
    expires_at    INTEGER NOT NULL,
    used_at       INTEGER,
    used_by       INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    revoked_at    INTEGER
) STRICT;
INSERT INTO "invitations" VALUES (1, '50793c4e76971f794149aba452704e321299d610d6a84462b16d0b92ad4d839d', 'Oma', 12, 0, 1, 1792425875539, 1793030675539, 1792425875583, 2, NULL);
INSERT INTO "invitations" VALUES (2, '1d9f92393abb9358c4ac4b65829e5406cdad2d9ddab2f667ee0fa14b5aaefeb8', 'offen', 16, 1, 1, 1792425875593, 1793030675593, NULL, NULL, NULL);
CREATE TABLE invitation_libraries (
    invitation_id INTEGER NOT NULL REFERENCES invitations(id) ON DELETE CASCADE,
    library_id    INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    PRIMARY KEY (invitation_id, library_id)
) STRICT, WITHOUT ROWID;
INSERT INTO "invitation_libraries" VALUES (1, 1);
CREATE UNIQUE INDEX jobs_active_idempotency_key ON jobs (idempotency_key)
    WHERE state IN ('queued', 'running');
CREATE INDEX jobs_claim ON jobs (state, type, priority DESC, created_at, id);
CREATE INDEX profiles_account ON profiles (account_id);
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
CREATE UNIQUE INDEX profiles_primary ON profiles (account_id) WHERE is_default = 1;
CREATE INDEX sessions_account ON sessions (account_id);
CREATE INDEX sessions_profile ON sessions (profile_id);
CREATE VIEW visible_items AS
SELECT p.id AS profile_id, i.id AS item_id
FROM profiles AS p
JOIN accounts AS a ON a.id = p.account_id
JOIN items AS i
  ON i.deleted_at IS NULL
 AND i.available = 1
 AND COALESCE(i.effective_rating, 18) <= min(p.max_rating, a.max_rating)
 AND (i.kind NOT IN ('series', 'season', 'podcast') OR COALESCE(i.min_child_rating, 18) <= min(p.max_rating, a.max_rating))
WHERE (p.all_libraries = 1
       OR EXISTS (SELECT 1 FROM profile_libraries AS pl
                  WHERE pl.profile_id = p.id AND pl.library_id = i.library_id))
  AND (a.all_libraries = 1
       OR EXISTS (SELECT 1 FROM account_libraries AS al
                  WHERE al.account_id = a.id AND al.library_id = i.library_id));
