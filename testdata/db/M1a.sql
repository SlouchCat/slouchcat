CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	);
INSERT INTO "goose_db_version" VALUES (1, 0, 1, '2026-10-18 20:39:38 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (2, 1, 1, '2026-10-18 20:39:38 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (3, 2, 1, '2026-10-18 20:39:38 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (4, 3, 1, '2026-10-18 20:39:38 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (5, 4, 1, '2026-10-18 20:39:38 +0000 UTC');
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
INSERT INTO "jobs" VALUES (1, 'library.sweep', 2, 'sweep', '{}', 'done', 1792355978585, NULL, NULL, 1, 5, NULL, 1792355978585, 1792355978586);
INSERT INTO "jobs" VALUES (2, 'scan.library', 5, 'scan:1', '{"libraryId":1}', 'done', 1792355980586, NULL, NULL, 1, 5, NULL, 1792355980586, 1792355980591);
INSERT INTO "jobs" VALUES (3, 'metadata.match', 4, 'match:01M58BVN9B869HFHAB6KM0HMR3', '{"itemId":"01M58BVN9B869HFHAB6KM0HMR3"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980591);
INSERT INTO "jobs" VALUES (4, 'probe.file', 3, 'probe:01M58BVN9CBA1KDEC844C3ZF6Y', '{"mediaFileId":"01M58BVN9CBA1KDEC844C3ZF6Y"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980626);
INSERT INTO "jobs" VALUES (5, 'metadata.match', 4, 'match:01M58BVN9CBA1KDEC8473DF3H0', '{"itemId":"01M58BVN9CBA1KDEC8473DF3H0"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980591);
INSERT INTO "jobs" VALUES (6, 'probe.file', 3, 'probe:01M58BVN9CBA1KDEC847PJYFPA', '{"mediaFileId":"01M58BVN9CBA1KDEC847PJYFPA"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980666);
INSERT INTO "jobs" VALUES (7, 'metadata.match', 4, 'match:01M58BVN9CBA1KDEC84BD9CWH3', '{"itemId":"01M58BVN9CBA1KDEC84BD9CWH3"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980591);
INSERT INTO "jobs" VALUES (8, 'probe.file', 3, 'probe:01M58BVN9D40054TDZ5C5N7MZE', '{"mediaFileId":"01M58BVN9D40054TDZ5C5N7MZE"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980699);
INSERT INTO "jobs" VALUES (9, 'metadata.match', 4, 'match:01M58BVN9D40054TDZ5CE4RDVW', '{"itemId":"01M58BVN9D40054TDZ5CE4RDVW"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980592);
INSERT INTO "jobs" VALUES (10, 'probe.file', 3, 'probe:01M58BVN9D40054TDZ5FN5BVG6', '{"mediaFileId":"01M58BVN9D40054TDZ5FN5BVG6"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980736);
INSERT INTO "jobs" VALUES (11, 'metadata.match', 4, 'match:01M58BVN9D40054TDZ5G5GBN7H', '{"itemId":"01M58BVN9D40054TDZ5G5GBN7H"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980592);
INSERT INTO "jobs" VALUES (12, 'probe.file', 3, 'probe:01M58BVN9D40054TDZ5J94NR2R', '{"mediaFileId":"01M58BVN9D40054TDZ5J94NR2R"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980769);
INSERT INTO "jobs" VALUES (13, 'metadata.match', 4, 'match:01M58BVN9EXJ87SWNEVMMR557G', '{"itemId":"01M58BVN9EXJ87SWNEVMMR557G"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980592);
INSERT INTO "jobs" VALUES (14, 'probe.file', 3, 'probe:01M58BVN9EXJ87SWNEVQER59VQ', '{"mediaFileId":"01M58BVN9EXJ87SWNEVQER59VQ"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980810);
INSERT INTO "jobs" VALUES (15, 'metadata.match', 4, 'match:01M58BVN9EXJ87SWNEVRSRMQD2', '{"itemId":"01M58BVN9EXJ87SWNEVRSRMQD2"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980593);
INSERT INTO "jobs" VALUES (16, 'probe.file', 3, 'probe:01M58BVN9EXJ87SWNEVT22FF7Q', '{"mediaFileId":"01M58BVN9EXJ87SWNEVT22FF7Q"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980843);
INSERT INTO "jobs" VALUES (17, 'metadata.match', 4, 'match:01M58BVN9EXJ87SWNEVV5RQDSJ', '{"itemId":"01M58BVN9EXJ87SWNEVV5RQDSJ"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980593);
INSERT INTO "jobs" VALUES (18, 'probe.file', 3, 'probe:01M58BVN9EXJ87SWNEVXE721YZ', '{"mediaFileId":"01M58BVN9EXJ87SWNEVXE721YZ"}', 'done', 1792355980587, NULL, NULL, 1, 5, NULL, 1792355980587, 1792355980879);
INSERT INTO "jobs" VALUES (19, 'probe.keyframes', 1, 'keyframes:01M58BVN9CBA1KDEC844C3ZF6Y', '{"mediaFileId":"01M58BVN9CBA1KDEC844C3ZF6Y"}', 'done', 1792355980625, NULL, NULL, 1, 5, NULL, 1792355980625, 1792355980922);
INSERT INTO "jobs" VALUES (20, 'probe.keyframes', 1, 'keyframes:01M58BVN9CBA1KDEC847PJYFPA', '{"mediaFileId":"01M58BVN9CBA1KDEC847PJYFPA"}', 'done', 1792355980666, NULL, NULL, 1, 5, NULL, 1792355980666, 1792355980959);
INSERT INTO "jobs" VALUES (21, 'probe.keyframes', 1, 'keyframes:01M58BVN9D40054TDZ5C5N7MZE', '{"mediaFileId":"01M58BVN9D40054TDZ5C5N7MZE"}', 'done', 1792355980699, NULL, NULL, 1, 5, NULL, 1792355980699, 1792355980997);
INSERT INTO "jobs" VALUES (22, 'probe.keyframes', 1, 'keyframes:01M58BVN9D40054TDZ5FN5BVG6', '{"mediaFileId":"01M58BVN9D40054TDZ5FN5BVG6"}', 'done', 1792355980735, NULL, NULL, 1, 5, NULL, 1792355980735, 1792355981030);
INSERT INTO "jobs" VALUES (23, 'probe.keyframes', 1, 'keyframes:01M58BVN9D40054TDZ5J94NR2R', '{"mediaFileId":"01M58BVN9D40054TDZ5J94NR2R"}', 'done', 1792355980768, NULL, NULL, 1, 5, NULL, 1792355980768, 1792355981065);
INSERT INTO "jobs" VALUES (24, 'probe.keyframes', 1, 'keyframes:01M58BVN9EXJ87SWNEVQER59VQ', '{"mediaFileId":"01M58BVN9EXJ87SWNEVQER59VQ"}', 'done', 1792355980810, NULL, NULL, 1, 5, NULL, 1792355980810, 1792355981096);
INSERT INTO "jobs" VALUES (25, 'probe.keyframes', 1, 'keyframes:01M58BVN9EXJ87SWNEVT22FF7Q', '{"mediaFileId":"01M58BVN9EXJ87SWNEVT22FF7Q"}', 'done', 1792355980843, NULL, NULL, 1, 5, NULL, 1792355980843, 1792355981136);
INSERT INTO "jobs" VALUES (26, 'probe.keyframes', 1, 'keyframes:01M58BVN9EXJ87SWNEVXE721YZ', '{"mediaFileId":"01M58BVN9EXJ87SWNEVXE721YZ"}', 'done', 1792355980879, NULL, NULL, 1, 5, NULL, 1792355980879, 1792355981172);
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
INSERT INTO "accounts" VALUES (1, 'frozen@example.test', '$argon2id$v=19$m=65536,t=3,p=4$bWhkNtmFSmOq0QKSLdcGJw$WfEKbG2zzfYaHR21axsYImoaf79eAJ6AzY7PD9x5Jtg', 'owner', 1792355980585, 1792355980585);
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
INSERT INTO "profiles" VALUES (1, 1, 'frozen', NULL, 18, 'de', NULL, 1, 1, 1792355980585, 1792355980585);
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
INSERT INTO "sessions" VALUES (1, '4db7ad859e3253f321ebeac0d8a537c34ab690804770e8128655eeb130603b1f', 1, 1, 'curl/8.5.0', 1792355980587, 1792355980587, 1794947980587, NULL);
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
INSERT INTO "libraries" VALUES (1, 'Filme', 'movies', 1792355980586, 1792355980591, 'done', 1792355980586, 1792355980591, NULL, 8, 8, NULL);
CREATE TABLE library_roots (
    id         INTEGER PRIMARY KEY,
    library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    path       TEXT    NOT NULL UNIQUE,
    fs_type    TEXT    NOT NULL DEFAULT 'unknown',
    watch      TEXT    NOT NULL DEFAULT 'poll' CHECK (watch IN ('inotify', 'poll', 'inotify_poll')),
    created_at INTEGER NOT NULL
) STRICT;
INSERT INTO "library_roots" VALUES (1, 1, '/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/testdata/media/Movies', 'ext4', 'inotify', 1792355980586);
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
INSERT INTO "items" VALUES ('01M58BVN9B869HFHAB6KM0HMR3', 'movie', NULL, 1, 'Slouchcat Test AC3 51', 'slouchcat test ac3 51', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792355980587, 1792355980587, 1792355980591, NULL);
INSERT INTO "items" VALUES ('01M58BVN9CBA1KDEC8473DF3H0', 'movie', NULL, 1, 'Slouchcat Test Chapters', 'slouchcat test chapters', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792355980587, 1792355980587, 1792355980591, NULL);
INSERT INTO "items" VALUES ('01M58BVN9CBA1KDEC84BD9CWH3', 'movie', NULL, 1, 'Slouchcat Test DTS', 'slouchcat test dts', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792355980587, 1792355980587, 1792355980591, NULL);
INSERT INTO "items" VALUES ('01M58BVN9D40054TDZ5CE4RDVW', 'movie', NULL, 1, 'Slouchcat Test H264', 'slouchcat test h264', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792355980587, 1792355980587, 1792355980592, NULL);
INSERT INTO "items" VALUES ('01M58BVN9D40054TDZ5G5GBN7H', 'movie', NULL, 1, 'Slouchcat Test HDR10 4K', 'slouchcat test hdr10 4k', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792355980587, 1792355980587, 1792355980592, NULL);
INSERT INTO "items" VALUES ('01M58BVN9EXJ87SWNEVMMR557G', 'movie', NULL, 1, 'Slouchcat Test HEVC 10bit', 'slouchcat test hevc 10bit', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792355980587, 1792355980587, 1792355980592, NULL);
INSERT INTO "items" VALUES ('01M58BVN9EXJ87SWNEVRSRMQD2', 'movie', NULL, 1, 'Slouchcat Test Sidecar Subtitles', 'slouchcat test sidecar subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792355980587, 1792355980587, 1792355980592, NULL);
INSERT INTO "items" VALUES ('01M58BVN9EXJ87SWNEVV5RQDSJ', 'movie', NULL, 1, 'Slouchcat Test Subtitles', 'slouchcat test subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792355980587, 1792355980587, 1792355980593, NULL);
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
INSERT INTO "media_files" VALUES ('01M58BVN9CBA1KDEC844C3ZF6Y', '01M58BVN9B869HFHAB6KM0HMR3', 1, 'Slouchcat Test AC3 51 (2024)/Slouchcat Test AC3 51 (2024).mkv', 66310, 2121821, 2943036, 1792345527307, 'b2e891bc9da05c622e0a18ac5bdf0e040d400465fb1b84c586f905794c33a735', NULL, 'mkv', 'h264', 'ac3', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":2943036,"bitRate":2354428},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"ac3","channels":6,"channelLayout":"5.1(side)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792355980625, 1792355980922, NULL, 1792355980587, 1792355980922);
INSERT INTO "media_files" VALUES ('01M58BVN9CBA1KDEC847PJYFPA', '01M58BVN9CBA1KDEC8473DF3H0', 1, 'Slouchcat Test Chapters (2024)/Slouchcat Test Chapters (2024).mkv', 66310, 2121833, 2623898, 1792345529388, 'c2bb87ccf56f8bb95c24c9b0b9cbd63020f2aef72cb50c08fee9773851895722', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2623898,"bitRate":2094719},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[{"start":0,"end":3000000000,"title":"Kapitel 1: Anfang"},{"start":3000000000,"end":6500000000,"title":"Kapitel 2: Mitte"},{"start":6500000000,"end":10000000000,"title":"Kapitel 3: Ende"}]}', X'0100d00fd00fd00fd00f', 1792355980666, 1792355980958, NULL, 1792355980587, 1792355980958);
INSERT INTO "media_files" VALUES ('01M58BVN9D40054TDZ5C5N7MZE', '01M58BVN9CBA1KDEC84BD9CWH3', 1, 'Slouchcat Test DTS (2024)/Slouchcat Test DTS (2024).mkv', 66310, 2121823, 3427132, 1792345527962, 'a0630ec8242c388a6440b8dd84cdc8221d56f16bfe33809f0c0dfc1b7e805d54', NULL, 'mkv', 'h264', 'dts', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":3427132,"bitRate":2741705},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"dts","profile":"DTS","channels":6,"channelLayout":"5.1(side)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792355980699, 1792355980996, NULL, 1792355980587, 1792355980996);
INSERT INTO "media_files" VALUES ('01M58BVN9D40054TDZ5FN5BVG6', '01M58BVN9D40054TDZ5CE4RDVW', 1, 'Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024).mp4', 66310, 2121811, 2624977, 1792345514047, '1a3559a74acfe685061086a9246f1ad08d3f220ba78aad39e503e56f4400693b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624977,"bitRate":2099981},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792355980735, 1792355981030, NULL, 1792355980587, 1792355981030);
INSERT INTO "media_files" VALUES ('01M58BVN9D40054TDZ5J94NR2R', '01M58BVN9D40054TDZ5G5GBN7H', 1, 'Slouchcat Test HDR10 4K (2024)/Slouchcat Test HDR10 4K (2024).mkv', 66310, 2121816, 761065, 1792345526738, '4f5e90e46dfab8b657b36c94ac139833c1bef0954ccc3750969aedb3d4547f86', '2160p', 'mkv', 'hevc', 'aac', 3840, 2160, 30021, '{"format":{"name":"matroska,webm","duration":30021000000,"size":761065,"bitRate":202808},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":150,"width":3840,"height":2160,"pixFmt":"yuv420p10le","colorTransfer":"smpte2084","colorPrimaries":"bt2020"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1792355980768, 1792355981065, NULL, 1792355980587, 1792355981065);
INSERT INTO "media_files" VALUES ('01M58BVN9EXJ87SWNEVQER59VQ', '01M58BVN9EXJ87SWNEVMMR557G', 1, 'Slouchcat Test HEVC 10bit (2024)/Slouchcat Test HEVC 10bit (2024).mkv', 66310, 2121814, 2102531, 1792345515157, '2469bfd53f3d50de5dd084a6dff5c3263c0d36fc93643768ce16469bfd62197a', NULL, 'mkv', 'hevc', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2102531,"bitRate":1678499},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":120,"width":1920,"height":1080,"pixFmt":"yuv420p10le","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792355980810, 1792355981096, NULL, 1792355980587, 1792355981096);
INSERT INTO "media_files" VALUES ('01M58BVN9EXJ87SWNEVT22FF7Q', '01M58BVN9EXJ87SWNEVRSRMQD2', 1, 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).mkv', 66310, 2121827, 2623761, 1792345528684, 'ddf64e1615182551e4bb7121f07c190c691f6b99117d3146e49e27d845311ec8', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10047, '{"format":{"name":"matroska,webm","duration":10047000000,"size":2623761,"bitRate":2089189},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0115d00fd00fd00fd00f', 1792355980843, 1792355981136, NULL, 1792355980587, 1792355981136);
INSERT INTO "media_files" VALUES ('01M58BVN9EXJ87SWNEVXE721YZ', '01M58BVN9EXJ87SWNEVV5RQDSJ', 1, 'Slouchcat Test Subtitles (2024)/Slouchcat Test Subtitles (2024).mkv', 66310, 2121825, 2626565, 1792345528629, '369b617c7d7ad81e45b193ddbad680f77b77593fc0a095272d6199ab77521204', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2626565,"bitRate":2096848},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"},{"index":2,"type":"subtitle","codec":"subrip","language":"ger","title":"Deutsch (SRT)","default":true},{"index":3,"type":"subtitle","codec":"subrip","language":"eng","title":"English (SRT)"},{"index":4,"type":"subtitle","codec":"ass","language":"ger","title":"Deutsch (ASS)"},{"index":5,"type":"subtitle","codec":"ass","language":"eng","title":"English (ASS)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792355980879, 1792355981171, NULL, 1792355980587, 1792355981171);
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
INSERT INTO "tracks" VALUES (1, '01M58BVN9CBA1KDEC844C3ZF6Y', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (2, '01M58BVN9CBA1KDEC844C3ZF6Y', 1, 'audio', 'ac3', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (3, '01M58BVN9CBA1KDEC847PJYFPA', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (4, '01M58BVN9CBA1KDEC847PJYFPA', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (5, '01M58BVN9D40054TDZ5C5N7MZE', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (6, '01M58BVN9D40054TDZ5C5N7MZE', 1, 'audio', 'dts', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (7, '01M58BVN9D40054TDZ5FN5BVG6', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (8, '01M58BVN9D40054TDZ5FN5BVG6', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (9, '01M58BVN9D40054TDZ5J94NR2R', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (10, '01M58BVN9D40054TDZ5J94NR2R', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (11, '01M58BVN9EXJ87SWNEVQER59VQ', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (12, '01M58BVN9EXJ87SWNEVQER59VQ', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (13, '01M58BVN9EXJ87SWNEVT22FF7Q', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (14, '01M58BVN9EXJ87SWNEVT22FF7Q', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (15, '01M58BVN9EXJ87SWNEVT22FF7Q', 0, 'subtitle', 'ass', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.ass', 0, 0);
INSERT INTO "tracks" VALUES (16, '01M58BVN9EXJ87SWNEVT22FF7Q', 1, 'subtitle', 'subrip', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.srt', 0, 0);
INSERT INTO "tracks" VALUES (17, '01M58BVN9EXJ87SWNEVT22FF7Q', 2, 'subtitle', 'ass', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.ass', 0, 0);
INSERT INTO "tracks" VALUES (18, '01M58BVN9EXJ87SWNEVT22FF7Q', 3, 'subtitle', 'subrip', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.srt', 0, 0);
INSERT INTO "tracks" VALUES (19, '01M58BVN9EXJ87SWNEVXE721YZ', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (20, '01M58BVN9EXJ87SWNEVXE721YZ', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (21, '01M58BVN9EXJ87SWNEVXE721YZ', 2, 'subtitle', 'subrip', 'ger', 'Deutsch (SRT)', 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (22, '01M58BVN9EXJ87SWNEVXE721YZ', 3, 'subtitle', 'subrip', 'eng', 'English (SRT)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (23, '01M58BVN9EXJ87SWNEVXE721YZ', 4, 'subtitle', 'ass', 'ger', 'Deutsch (ASS)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (24, '01M58BVN9EXJ87SWNEVXE721YZ', 5, 'subtitle', 'ass', 'eng', 'English (ASS)', 'embedded', NULL, 0, 0);
CREATE TABLE images (
    id           INTEGER PRIMARY KEY,
    item_id      TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    kind         TEXT    NOT NULL CHECK (kind IN ('poster', 'backdrop')),
    size         TEXT    NOT NULL,                 -- TMDB size the file was fetched in (w500, w1280)
    lang         TEXT,
    source_url   TEXT    NOT NULL,
    rel_path     TEXT    NOT NULL,                 -- relative to the cache root
    content_hash TEXT    NOT NULL,                 -- SHA-256 hex, cache busting only
    media_type   TEXT    NOT NULL,                 -- image/jpeg, image/png, image/webp
    width        INTEGER NOT NULL,
    height       INTEGER NOT NULL,
    bytes        INTEGER NOT NULL,
    created_at   INTEGER NOT NULL,
    UNIQUE (item_id, kind, size)
) STRICT;
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
INSERT INTO "progress" VALUES (1, '01M58BVN9EXJ87SWNEVV5RQDSJ', 4000, 10000, 0, 1792355988618);
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
INSERT INTO "progress_events" VALUES (1, 1, '01M58BVN9EXJ87SWNEVV5RQDSJ', NULL, NULL, 4000, 10000, 4000, 1, 'paused', 1792356000000, 1792355988618);
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
