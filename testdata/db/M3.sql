CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	);
INSERT INTO "goose_db_version" VALUES (1, 0, 1, '2026-10-19 21:44:08 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (2, 1, 1, '2026-10-19 21:44:08 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (3, 2, 1, '2026-10-19 21:44:08 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (4, 3, 1, '2026-10-19 21:44:08 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (5, 4, 1, '2026-10-19 21:44:08 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (6, 5, 1, '2026-10-19 21:44:08 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (7, 6, 1, '2026-10-19 21:44:08 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (8, 7, 1, '2026-10-19 21:44:08 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (9, 8, 1, '2026-10-19 21:44:08 +0000 UTC');
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
INSERT INTO "jobs" VALUES (1, 'library.sweep', 2, 'sweep', '{}', 'done', 1792446248807, NULL, NULL, 1, 5, NULL, 1792446248807, 1792446248808);
INSERT INTO "jobs" VALUES (2, 'scan.library', 5, 'scan:1', '{"libraryId":1}', 'done', 1792446266670, NULL, NULL, 1, 5, NULL, 1792446266670, 1792446266834);
INSERT INTO "jobs" VALUES (3, 'scan.library', 5, 'scan:2', '{"libraryId":2}', 'done', 1792446266673, NULL, NULL, 1, 5, NULL, 1792446266673, 1792446266825);
INSERT INTO "jobs" VALUES (4, 'metadata.match', 4, 'match:01M5B1YZEBVR6T6X9S0HTDE3SQ', '{"itemId":"01M5B1YZEBVR6T6X9S0HTDE3SQ"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266834);
INSERT INTO "jobs" VALUES (5, 'probe.file', 3, 'probe:01M5B1YZEBVR6T6X9S0JBNGSSK', '{"mediaFileId":"01M5B1YZEBVR6T6X9S0JBNGSSK"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266868);
INSERT INTO "jobs" VALUES (6, 'metadata.match', 4, 'match:01M5B1YZEC2SR0VD1RYQ7WGP7R', '{"itemId":"01M5B1YZEC2SR0VD1RYQ7WGP7R"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266834);
INSERT INTO "jobs" VALUES (7, 'probe.file', 3, 'probe:01M5B1YZEC2SR0VD1RYR5GDYZ2', '{"mediaFileId":"01M5B1YZEC2SR0VD1RYR5GDYZ2"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266900);
INSERT INTO "jobs" VALUES (8, 'metadata.match', 4, 'match:01M5B1YZED8098W4HE83CZ21XF', '{"itemId":"01M5B1YZED8098W4HE83CZ21XF"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266835);
INSERT INTO "jobs" VALUES (9, 'probe.file', 3, 'probe:01M5B1YZED8098W4HE854TD7YH', '{"mediaFileId":"01M5B1YZED8098W4HE854TD7YH"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266939);
INSERT INTO "jobs" VALUES (10, 'metadata.match', 4, 'match:01M5B1YZED8098W4HE86F16SY1', '{"itemId":"01M5B1YZED8098W4HE86F16SY1"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266835);
INSERT INTO "jobs" VALUES (11, 'probe.file', 3, 'probe:01M5B1YZEE2XQKGPXWE8TVVDY2', '{"mediaFileId":"01M5B1YZEE2XQKGPXWE8TVVDY2"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266979);
INSERT INTO "jobs" VALUES (12, 'metadata.match', 4, 'match:01M5B1YZEE2XQKGPXWECGBXBS3', '{"itemId":"01M5B1YZEE2XQKGPXWECGBXBS3"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266835);
INSERT INTO "jobs" VALUES (13, 'probe.file', 3, 'probe:01M5B1YZEFZFHX1Q6J8TAG3XYR', '{"mediaFileId":"01M5B1YZEFZFHX1Q6J8TAG3XYR"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446267009);
INSERT INTO "jobs" VALUES (14, 'metadata.match', 4, 'match:01M5B1YZEFZFHX1Q6J8V08BQKW', '{"itemId":"01M5B1YZEFZFHX1Q6J8V08BQKW"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266836);
INSERT INTO "jobs" VALUES (15, 'probe.file', 3, 'probe:01M5B1YZEFZFHX1Q6J8XS0PBSW', '{"mediaFileId":"01M5B1YZEFZFHX1Q6J8XS0PBSW"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446267060);
INSERT INTO "jobs" VALUES (16, 'metadata.match', 4, 'match:01M5B1YZEGJYVGV09ENYB0CAF8', '{"itemId":"01M5B1YZEGJYVGV09ENYB0CAF8"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266836);
INSERT INTO "jobs" VALUES (17, 'probe.file', 3, 'probe:01M5B1YZEGJYVGV09ENYP8B3XB', '{"mediaFileId":"01M5B1YZEGJYVGV09ENYP8B3XB"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446267090);
INSERT INTO "jobs" VALUES (18, 'metadata.match', 4, 'match:01M5B1YZEGJYVGV09ENYZ9XXX5', '{"itemId":"01M5B1YZEGJYVGV09ENYZ9XXX5"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266837);
INSERT INTO "jobs" VALUES (19, 'probe.file', 3, 'probe:01M5B1YZEH47P4DWJBAZWKYGVK', '{"mediaFileId":"01M5B1YZEH47P4DWJBAZWKYGVK"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446267121);
INSERT INTO "jobs" VALUES (20, 'metadata.match', 4, 'match:01M5B1YZEH47P4DWJBB094VPWH', '{"itemId":"01M5B1YZEH47P4DWJBB094VPWH"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266837);
INSERT INTO "jobs" VALUES (21, 'probe.file', 3, 'probe:01M5B1YZEH47P4DWJBB3TN3D9S', '{"mediaFileId":"01M5B1YZEH47P4DWJBB3TN3D9S"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446267160);
INSERT INTO "jobs" VALUES (22, 'metadata.match', 4, 'match:01M5B1YZEH47P4DWJBB45NW5DG', '{"itemId":"01M5B1YZEH47P4DWJBB45NW5DG"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446266837);
INSERT INTO "jobs" VALUES (23, 'probe.file', 3, 'probe:01M5B1YZEJGF86TB3T7GS83GGH', '{"mediaFileId":"01M5B1YZEJGF86TB3T7GS83GGH"}', 'done', 1792446266827, NULL, NULL, 1, 5, NULL, 1792446266827, 1792446267198);
INSERT INTO "jobs" VALUES (24, 'probe.keyframes', 1, 'keyframes:01M5B1YZEBVR6T6X9S0JBNGSSK', '{"mediaFileId":"01M5B1YZEBVR6T6X9S0JBNGSSK"}', 'done', 1792446266867, NULL, NULL, 1, 5, NULL, 1792446266867, 1792446267231);
INSERT INTO "jobs" VALUES (25, 'probe.keyframes', 1, 'keyframes:01M5B1YZEC2SR0VD1RYR5GDYZ2', '{"mediaFileId":"01M5B1YZEC2SR0VD1RYR5GDYZ2"}', 'done', 1792446266900, NULL, NULL, 1, 5, NULL, 1792446266900, 1792446267265);
INSERT INTO "jobs" VALUES (26, 'probe.keyframes', 1, 'keyframes:01M5B1YZED8098W4HE854TD7YH', '{"mediaFileId":"01M5B1YZED8098W4HE854TD7YH"}', 'done', 1792446266939, NULL, NULL, 1, 5, NULL, 1792446266939, 1792446267296);
INSERT INTO "jobs" VALUES (27, 'probe.keyframes', 1, 'keyframes:01M5B1YZEE2XQKGPXWE8TVVDY2', '{"mediaFileId":"01M5B1YZEE2XQKGPXWE8TVVDY2"}', 'done', 1792446266978, NULL, NULL, 1, 5, NULL, 1792446266978, 1792446267345);
INSERT INTO "jobs" VALUES (28, 'probe.keyframes', 1, 'keyframes:01M5B1YZEFZFHX1Q6J8TAG3XYR', '{"mediaFileId":"01M5B1YZEFZFHX1Q6J8TAG3XYR"}', 'done', 1792446267009, NULL, NULL, 1, 5, NULL, 1792446267009, 1792446267392);
INSERT INTO "jobs" VALUES (29, 'probe.keyframes', 1, 'keyframes:01M5B1YZEFZFHX1Q6J8XS0PBSW', '{"mediaFileId":"01M5B1YZEFZFHX1Q6J8XS0PBSW"}', 'done', 1792446267060, NULL, NULL, 1, 5, NULL, 1792446267060, 1792446267429);
INSERT INTO "jobs" VALUES (30, 'probe.keyframes', 1, 'keyframes:01M5B1YZEGJYVGV09ENYP8B3XB', '{"mediaFileId":"01M5B1YZEGJYVGV09ENYP8B3XB"}', 'done', 1792446267089, NULL, NULL, 1, 5, NULL, 1792446267089, 1792446267463);
INSERT INTO "jobs" VALUES (31, 'probe.keyframes', 1, 'keyframes:01M5B1YZEH47P4DWJBAZWKYGVK', '{"mediaFileId":"01M5B1YZEH47P4DWJBAZWKYGVK"}', 'done', 1792446267121, NULL, NULL, 1, 5, NULL, 1792446267121, 1792446267498);
INSERT INTO "jobs" VALUES (32, 'probe.keyframes', 1, 'keyframes:01M5B1YZEH47P4DWJBB3TN3D9S', '{"mediaFileId":"01M5B1YZEH47P4DWJBB3TN3D9S"}', 'done', 1792446267160, NULL, NULL, 1, 5, NULL, 1792446267160, 1792446267533);
INSERT INTO "jobs" VALUES (33, 'probe.keyframes', 1, 'keyframes:01M5B1YZEJGF86TB3T7GS83GGH', '{"mediaFileId":"01M5B1YZEJGF86TB3T7GS83GGH"}', 'done', 1792446267197, NULL, NULL, 1, 5, NULL, 1792446267197, 1792446267574);
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
INSERT INTO "accounts" VALUES (1, 'frozen@example.test', '$argon2id$v=19$m=65536,t=3,p=4$D+dvEdU/czSrOLs4n5Z5BQ$0l3azBcf0Uk2D7TdOynFBlJBBgcIk2lFg2qYvMv7KtI', 'owner', 1792446266669, 1792446266669, 18, 1, NULL, NULL);
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
INSERT INTO "profiles" VALUES (1, 1, 'frozen', NULL, 18, 'de', NULL, 1, 1, 1792446266669, 1792446266669, NULL);
INSERT INTO "profiles" VALUES (2, 1, 'Teenager', NULL, 12, 'de', NULL, 1, 0, 1792446266976, 1792446266976, NULL);
INSERT INTO "profiles" VALUES (3, 1, 'Kind', NULL, 6, 'en', NULL, 1, 0, 1792446266977, 1792446266977, NULL);
CREATE TABLE profile_libraries (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    library_id INTEGER NOT NULL,
    PRIMARY KEY (profile_id, library_id)
) STRICT, WITHOUT ROWID;
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
INSERT INTO "libraries" VALUES (1, 'Filme', 'movies', 1792446266670, 1792446266834, 'done', 1792446266824, 1792446266834, NULL, 10, 10, NULL);
INSERT INTO "libraries" VALUES (2, 'Serien', 'series', 1792446266672, 1792446266825, 'done', 1792446266825, 1792446266825, NULL, 0, 0, NULL);
CREATE TABLE library_roots (
    id         INTEGER PRIMARY KEY,
    library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    path       TEXT    NOT NULL UNIQUE,
    fs_type    TEXT    NOT NULL DEFAULT 'unknown',
    watch      TEXT    NOT NULL DEFAULT 'poll' CHECK (watch IN ('inotify', 'poll', 'inotify_poll')),
    created_at INTEGER NOT NULL
) STRICT;
INSERT INTO "library_roots" VALUES (1, 1, '/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/testdata/media/Movies', 'ext4', 'inotify', 1792446266670);
INSERT INTO "library_roots" VALUES (2, 2, '/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/testdata/media/TV', 'ext4', 'inotify', 1792446266672);
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
, season_number INTEGER CHECK (season_number IS NULL OR season_number >= 0), episode_number INTEGER CHECK (episode_number IS NULL OR episode_number >= 0), episode_end INTEGER CHECK (episode_end IS NULL OR episode_end >= 0), tvdb_id INTEGER, extra_kind TEXT CHECK (extra_kind IS NULL OR extra_kind IN
    ('trailer', 'featurette', 'deleted_scene', 'behind_the_scenes', 'interview', 'scene', 'short', 'other')), tmdb_poster_path TEXT, tmdb_backdrop_path TEXT, trailer_state TEXT NOT NULL DEFAULT 'pending'
    CHECK (trailer_state IN ('pending', 'found', 'none', 'failed')), trailer_checked_at INTEGER) STRICT;
INSERT INTO "items" VALUES ('01M5B1YZEBVR6T6X9S0HTDE3SQ', 'movie', NULL, 1, 'Slouchcat Test AC3 51', 'slouchcat test ac3 51', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792446266827, 1792446266827, 1792446266834, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5B1YZEC2SR0VD1RYQ7WGP7R', 'movie', NULL, 1, 'Slouchcat Test Chapters', 'slouchcat test chapters', NULL, 2024, NULL, NULL, '[]', NULL, NULL, 0, 0, NULL, 1, 'manual', NULL, '["own_rating"]', 'skipped', 'no_tmdb_key', NULL, 1792446266827, 1792446266827, 1792446304021, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5B1YZED8098W4HE83CZ21XF', 'movie', NULL, 1, 'Slouchcat Test DTS', 'slouchcat test dts', NULL, 2024, NULL, NULL, '[]', 603, NULL, NULL, NULL, NULL, 1, NULL, NULL, '["tmdb_id","tvdb_id","imdb_id"]', 'pending', 'no_tmdb_key', NULL, 1792446266827, 1792446266827, 1792446266835, NULL, NULL, NULL, NULL, NULL, NULL, '/matrix.jpg', NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5B1YZED8098W4HE86F16SY1', 'movie', NULL, 1, 'Slouchcat Test H264', 'slouchcat test h264', NULL, 2024, NULL, NULL, '[]', NULL, NULL, 12, 12, NULL, 1, 'manual', NULL, '["own_rating"]', 'skipped', 'no_tmdb_key', NULL, 1792446266827, 1792446266827, 1792446304023, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'found', 1792446345037);
INSERT INTO "items" VALUES ('01M5B1YZEE2XQKGPXWECGBXBS3', 'movie', NULL, 1, 'Slouchcat Test H264 MKV Long', 'slouchcat test h264 mkv long', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792446266827, 1792446266827, 1792446266835, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5B1YZEFZFHX1Q6J8V08BQKW', 'movie', NULL, 1, 'Slouchcat Test HDR10 4K', 'slouchcat test hdr10 4k', NULL, 2024, NULL, NULL, '[]', NULL, NULL, 16, 16, NULL, 1, 'manual', NULL, '["own_rating"]', 'skipped', 'no_tmdb_key', NULL, 1792446266827, 1792446266827, 1792446304025, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5B1YZEGJYVGV09ENYB0CAF8', 'movie', NULL, 1, 'Slouchcat Test HEVC 10bit', 'slouchcat test hevc 10bit', NULL, 2024, NULL, NULL, '[]', NULL, NULL, 6, 6, NULL, 1, 'manual', NULL, '["own_rating"]', 'skipped', 'no_tmdb_key', NULL, 1792446266827, 1792446266827, 1792446304022, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'none', 1792446345037);
INSERT INTO "items" VALUES ('01M5B1YZEGJYVGV09ENYZ9XXX5', 'movie', NULL, 1, 'Slouchcat Test HEVC Long', 'slouchcat test hevc long', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792446266827, 1792446266827, 1792446266837, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5B1YZEH47P4DWJBB094VPWH', 'movie', NULL, 1, 'Slouchcat Test Sidecar Subtitles', 'slouchcat test sidecar subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1792446266827, 1792446266827, 1792446266837, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5B1YZEH47P4DWJBB45NW5DG', 'movie', NULL, 1, 'Slouchcat Test Subtitles', 'slouchcat test subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, 12, 12, NULL, 1, 'manual', NULL, '["own_rating"]', 'skipped', 'no_tmdb_key', NULL, 1792446266827, 1792446266827, 1792446304024, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5C0000000000000000SER01', 'series', NULL, 2, 'Slouchcat Test Show', 'slouchcat test show', NULL, 2024, NULL, NULL, '[]', 900001, NULL, 12, 12, 12, 1, 'tmdb:DE', NULL, '[]', 'matched', NULL, 1792446345037, 1792446345037, 1792446345037, 1792446345037, NULL, NULL, NULL, NULL, 800001, NULL, '/poster-show.jpg', NULL, 'found', NULL);
INSERT INTO "items" VALUES ('01M5C0000000000000000SEA01', 'season', '01M5C0000000000000000SER01', 2, 'Season 1', 'season 0001', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, 12, 12, 1, NULL, NULL, '[]', 'matched', NULL, NULL, 1792446345037, 1792446345037, 1792446345037, NULL, 1, NULL, NULL, NULL, NULL, '/poster-s1.jpg', NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5C0000000000000000SEA00', 'season', '01M5C0000000000000000SER01', 2, 'Specials', 'specials', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, 12, 12, 1, NULL, NULL, '[]', 'matched', NULL, NULL, 1792446345037, 1792446345037, 1792446345037, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5C0000000000000000EPI01', 'episode', '01M5C0000000000000000SEA01', 2, 'Pilot', '0001 0001', NULL, NULL, '2024-01-05', NULL, '[]', NULL, NULL, NULL, 12, NULL, 1, NULL, NULL, '[]', 'matched', NULL, NULL, 1792446345037, 1792446345037, 1792446345037, NULL, 1, 1, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5C0000000000000000EPI02', 'episode', '01M5C0000000000000000SEA01', 2, 'Der zweite Tag', '0001 0002', NULL, NULL, NULL, NULL, '[]', NULL, NULL, 16, 16, NULL, 1, 'manual', NULL, '["own_rating"]', 'matched', NULL, NULL, 1792446345037, 1792446345037, 1792446345037, NULL, 1, 2, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5C0000000000000000EPI03', 'episode', '01M5C0000000000000000SEA01', 2, 'Doppelfolge', '0001 0003', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, 12, NULL, 1, NULL, NULL, '[]', 'matched', NULL, NULL, 1792446345037, 1792446345037, 1792446345037, NULL, 1, 3, 4, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5C0000000000000000SPE01', 'episode', '01M5C0000000000000000SEA00', 2, 'Hinter der Kamera', '0000 0001', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, 12, NULL, 1, NULL, NULL, '[]', 'matched', NULL, NULL, 1792446345037, 1792446345037, 1792446345037, NULL, 0, 1, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5C0000000000000000EXT01', 'extra', '01M5C0000000000000000SER01', 2, 'Making of', 'making of', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, 12, NULL, 1, NULL, NULL, '[]', 'skipped', NULL, NULL, 1792446345037, 1792446345037, 1792446345037, NULL, NULL, NULL, NULL, NULL, 'behind_the_scenes', NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5C0000000000000000EXT02', 'extra', '01M5B1YZED8098W4HE86F16SY1', 1, 'Teaser', 'teaser', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, 12, NULL, 1, NULL, NULL, '[]', 'skipped', NULL, NULL, 1792446345037, 1792446345037, 1792446345037, NULL, NULL, NULL, NULL, NULL, 'trailer', NULL, NULL, 'pending', NULL);
INSERT INTO "items" VALUES ('01M5C0000000000000000BRK01', 'movie', NULL, 1, 'Slouchcat Test Broken', 'slouchcat test broken', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, NULL, 1792446345037, 1792446345037, 1792446345037, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL);
CREATE TABLE item_texts (
    item_id  TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    lang     TEXT NOT NULL CHECK (lang IN ('de', 'en')),
    title    TEXT NOT NULL,
    overview TEXT,
    tagline  TEXT,
    PRIMARY KEY (item_id, lang)
) STRICT, WITHOUT ROWID;
INSERT INTO "item_texts" VALUES ('01M5C0000000000000000EPI01', 'de', 'Pilot', 'Handlung: Pilot', NULL);
INSERT INTO "item_texts" VALUES ('01M5C0000000000000000EPI01', 'en', 'Pilot', 'Plot: Pilot', NULL);
INSERT INTO "item_texts" VALUES ('01M5C0000000000000000SEA01', 'de', 'Staffel 1', 'Handlung: Staffel 1', NULL);
INSERT INTO "item_texts" VALUES ('01M5C0000000000000000SEA01', 'en', 'Season 1', 'Plot: Season 1', NULL);
INSERT INTO "item_texts" VALUES ('01M5C0000000000000000SER01', 'de', 'Slouchcat Testserie', 'Handlung: Slouchcat Testserie', NULL);
INSERT INTO "item_texts" VALUES ('01M5C0000000000000000SER01', 'en', 'Slouchcat Test Show', 'Plot: Slouchcat Test Show', NULL);
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
    updated_at    INTEGER NOT NULL, probe_error TEXT,
    UNIQUE (root_id, rel_path)
) STRICT;
INSERT INTO "media_files" VALUES ('01M5B1YZEBVR6T6X9S0JBNGSSK', '01M5B1YZEBVR6T6X9S0HTDE3SQ', 1, 'Slouchcat Test AC3 51 (2024)/Slouchcat Test AC3 51 (2024).mkv', 66310, 2121821, 2943036, 1792345527307, 'b2e891bc9da05c622e0a18ac5bdf0e040d400465fb1b84c586f905794c33a735', NULL, 'mkv', 'h264', 'ac3', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":2943036,"bitRate":2354428},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"ac3","channels":6,"channelLayout":"5.1(side)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792446266867, 1792446267231, NULL, 1792446266827, 1792446267231, NULL);
INSERT INTO "media_files" VALUES ('01M5B1YZEC2SR0VD1RYR5GDYZ2', '01M5B1YZEC2SR0VD1RYQ7WGP7R', 1, 'Slouchcat Test Chapters (2024)/Slouchcat Test Chapters (2024).mkv', 66310, 2121833, 2623898, 1792345529388, 'c2bb87ccf56f8bb95c24c9b0b9cbd63020f2aef72cb50c08fee9773851895722', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2623898,"bitRate":2094719},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[{"start":0,"end":3000000000,"title":"Kapitel 1: Anfang"},{"start":3000000000,"end":6500000000,"title":"Kapitel 2: Mitte"},{"start":6500000000,"end":10000000000,"title":"Kapitel 3: Ende"}]}', X'0100d00fd00fd00fd00f', 1792446266900, 1792446267265, NULL, 1792446266827, 1792446267265, NULL);
INSERT INTO "media_files" VALUES ('01M5B1YZED8098W4HE854TD7YH', '01M5B1YZED8098W4HE83CZ21XF', 1, 'Slouchcat Test DTS (2024)/Slouchcat Test DTS (2024).mkv', 66310, 2121823, 3427132, 1792345527962, 'a0630ec8242c388a6440b8dd84cdc8221d56f16bfe33809f0c0dfc1b7e805d54', NULL, 'mkv', 'h264', 'dts', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":3427132,"bitRate":2741705},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"dts","profile":"DTS","channels":6,"channelLayout":"5.1(side)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792446266939, 1792446267296, NULL, 1792446266827, 1792446267296, NULL);
INSERT INTO "media_files" VALUES ('01M5B1YZEE2XQKGPXWE8TVVDY2', '01M5B1YZED8098W4HE86F16SY1', 1, 'Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024).mp4', 66310, 2121811, 2624977, 1792345514047, '1a3559a74acfe685061086a9246f1ad08d3f220ba78aad39e503e56f4400693b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624977,"bitRate":2099981},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792446266978, 1792446267345, NULL, 1792446266827, 1792446267345, NULL);
INSERT INTO "media_files" VALUES ('01M5B1YZEFZFHX1Q6J8TAG3XYR', '01M5B1YZEE2XQKGPXWECGBXBS3', 1, 'Slouchcat Test H264 MKV Long (2024)/Slouchcat Test H264 MKV Long (2024).mkv', 66310, 3286276, 13446607, 1792427238596, '201c421ed5521ef87bf11cd453de79aec1def64bc511895e32e66d49175a6ac8', NULL, 'mkv', 'h264', 'aac', 1280, 720, 90021, '{"format":{"name":"matroska,webm","duration":90021000000,"size":13446607,"bitRate":1194975},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00f8827e807d836f02ea846e807b06d9875e807b06d', 1792446267009, 1792446267392, NULL, 1792446266827, 1792446267392, NULL);
INSERT INTO "media_files" VALUES ('01M5B1YZEFZFHX1Q6J8XS0PBSW', '01M5B1YZEFZFHX1Q6J8V08BQKW', 1, 'Slouchcat Test HDR10 4K (2024)/Slouchcat Test HDR10 4K (2024).mkv', 66310, 2121816, 761065, 1792345526738, '4f5e90e46dfab8b657b36c94ac139833c1bef0954ccc3750969aedb3d4547f86', '2160p', 'mkv', 'hevc', 'aac', 3840, 2160, 30021, '{"format":{"name":"matroska,webm","duration":30021000000,"size":761065,"bitRate":202808},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":150,"width":3840,"height":2160,"pixFmt":"yuv420p10le","colorTransfer":"smpte2084","colorPrimaries":"bt2020"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1792446267060, 1792446267429, NULL, 1792446266827, 1792446267429, NULL);
INSERT INTO "media_files" VALUES ('01M5B1YZEGJYVGV09ENYP8B3XB', '01M5B1YZEGJYVGV09ENYB0CAF8', 1, 'Slouchcat Test HEVC 10bit (2024)/Slouchcat Test HEVC 10bit (2024).mkv', 66310, 2121814, 2102531, 1792345515157, '2469bfd53f3d50de5dd084a6dff5c3263c0d36fc93643768ce16469bfd62197a', NULL, 'mkv', 'hevc', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2102531,"bitRate":1678499},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":120,"width":1920,"height":1080,"pixFmt":"yuv420p10le","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792446267089, 1792446267463, NULL, 1792446266827, 1792446267463, NULL);
INSERT INTO "media_files" VALUES ('01M5B1YZEH47P4DWJBAZWKYGVK', '01M5B1YZEGJYVGV09ENYZ9XXX5', 1, 'Slouchcat Test HEVC Long (2024)/Slouchcat Test HEVC Long (2024).mkv', 66310, 3286272, 9588128, 1792427236518, '0c0cf1bda8fb7ba37b5b1edb86736944dc48d81450b7f2e05326e9a211cb65e2', NULL, 'mkv', 'hevc', 'aac', 1280, 720, 90021, '{"format":{"name":"matroska,webm","duration":90021000000,"size":9588128,"bitRate":852079},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":93,"width":1280,"height":720,"pixFmt":"yuv420p10le","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1792446267121, 1792446267498, NULL, 1792446266827, 1792446267498, NULL);
INSERT INTO "media_files" VALUES ('01M5B1YZEH47P4DWJBB3TN3D9S', '01M5B1YZEH47P4DWJBB094VPWH', 1, 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).mkv', 66310, 2121827, 2623761, 1792345528684, 'ddf64e1615182551e4bb7121f07c190c691f6b99117d3146e49e27d845311ec8', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10047, '{"format":{"name":"matroska,webm","duration":10047000000,"size":2623761,"bitRate":2089189},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"}],"chapters":[]}', X'0115d00fd00fd00fd00f', 1792446267160, 1792446267532, NULL, 1792446266827, 1792446267532, NULL);
INSERT INTO "media_files" VALUES ('01M5B1YZEJGF86TB3T7GS83GGH', '01M5B1YZEH47P4DWJBB45NW5DG', 1, 'Slouchcat Test Subtitles (2024)/Slouchcat Test Subtitles (2024).mkv', 66310, 2121825, 2626565, 1792345528629, '369b617c7d7ad81e45b193ddbad680f77b77593fc0a095272d6199ab77521204', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2626565,"bitRate":2096848},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo"},{"index":2,"type":"subtitle","codec":"subrip","language":"ger","title":"Deutsch (SRT)","default":true},{"index":3,"type":"subtitle","codec":"subrip","language":"eng","title":"English (SRT)"},{"index":4,"type":"subtitle","codec":"ass","language":"ger","title":"Deutsch (ASS)"},{"index":5,"type":"subtitle","codec":"ass","language":"eng","title":"English (ASS)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1792446267197, 1792446267574, NULL, 1792446266827, 1792446267574, NULL);
INSERT INTO "media_files" VALUES ('01M5C0000000000000000MF001', '01M5C0000000000000000EPI01', 2, 'Slouchcat Test Show (2024)/Season 01/Slouchcat Test Show (2024) S01E01.mp4', 0, 0, 1048576, 1792446345037, 'm3-e1', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 60000, NULL, NULL, 1792446345037, NULL, NULL, 1792446345037, 1792446345037, NULL);
INSERT INTO "media_files" VALUES ('01M5C0000000000000000MF002', '01M5C0000000000000000EPI02', 2, 'Slouchcat Test Show (2024)/Season 01/Slouchcat Test Show (2024) S01E02.mp4', 0, 0, 1048576, 1792446345037, 'm3-e2', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 60000, NULL, NULL, 1792446345037, NULL, NULL, 1792446345037, 1792446345037, NULL);
INSERT INTO "media_files" VALUES ('01M5C0000000000000000MF003', '01M5C0000000000000000EPI03', 2, 'Slouchcat Test Show (2024)/Season 01/Slouchcat Test Show (2024) S01E03-E04.mp4', 0, 0, 1048576, 1792446345037, NULL, NULL, 'mp4', 'h264', 'aac', 1280, 720, 60000, NULL, NULL, 1792446345037, NULL, NULL, 1792446345037, 1792446345037, NULL);
INSERT INTO "media_files" VALUES ('01M5C0000000000000000MF004', '01M5C0000000000000000SPE01', 2, 'Slouchcat Test Show (2024)/Specials/Slouchcat Test Show (2024) S00E01.mp4', 0, 0, 1048576, 1792446345037, NULL, NULL, 'mp4', 'h264', 'aac', 1920, 1080, 60000, NULL, NULL, 1792446345037, NULL, NULL, 1792446345037, 1792446345037, NULL);
INSERT INTO "media_files" VALUES ('01M5C0000000000000000MF005', '01M5C0000000000000000EXT01', 2, 'Slouchcat Test Show (2024)/Behind The Scenes/Making of.mp4', 0, 0, 1048576, 1792446345037, NULL, NULL, 'mp4', 'h264', 'aac', 1280, 720, 60000, NULL, NULL, 1792446345037, NULL, NULL, 1792446345037, 1792446345037, NULL);
INSERT INTO "media_files" VALUES ('01M5C0000000000000000MF006', '01M5C0000000000000000EXT02', 1, 'Slouchcat Test H264 (2024)/Trailers/Teaser.mp4', 0, 0, 1048576, 1792446345037, NULL, NULL, 'mp4', 'h264', 'aac', 1280, 720, 60000, NULL, NULL, 1792446345037, NULL, NULL, 1792446345037, 1792446345037, NULL);
INSERT INTO "media_files" VALUES ('01M5C0000000000000000MF007', '01M5B1YZED8098W4HE86F16SY1', 1, 'Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024) - 720p.mp4', 0, 0, 1048576, 1792446345037, NULL, '720p', 'mp4', 'h264', 'aac', 1280, 720, 60000, NULL, NULL, 1792446345037, NULL, NULL, 1792446345037, 1792446345037, NULL);
INSERT INTO "media_files" VALUES ('01M5C0000000000000000MF008', '01M5C0000000000000000BRK01', 1, 'Slouchcat Test Broken (2024)/Slouchcat Test Broken (2024).mkv', 0, 0, 1048576, 1792446345037, NULL, NULL, 'mp4', 'h264', 'aac', 0, 0, 60000, NULL, NULL, 1792446345037, NULL, NULL, 1792446345037, 1792446345037, 'Invalid data found when processing input');
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
INSERT INTO "tracks" VALUES (1, '01M5B1YZEBVR6T6X9S0JBNGSSK', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (2, '01M5B1YZEBVR6T6X9S0JBNGSSK', 1, 'audio', 'ac3', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (3, '01M5B1YZEC2SR0VD1RYR5GDYZ2', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (4, '01M5B1YZEC2SR0VD1RYR5GDYZ2', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (5, '01M5B1YZED8098W4HE854TD7YH', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (6, '01M5B1YZED8098W4HE854TD7YH', 1, 'audio', 'dts', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (7, '01M5B1YZEE2XQKGPXWE8TVVDY2', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (8, '01M5B1YZEE2XQKGPXWE8TVVDY2', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (9, '01M5B1YZEFZFHX1Q6J8TAG3XYR', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (10, '01M5B1YZEFZFHX1Q6J8TAG3XYR', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (11, '01M5B1YZEFZFHX1Q6J8XS0PBSW', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (12, '01M5B1YZEFZFHX1Q6J8XS0PBSW', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (13, '01M5B1YZEGJYVGV09ENYP8B3XB', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (14, '01M5B1YZEGJYVGV09ENYP8B3XB', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (15, '01M5B1YZEH47P4DWJBAZWKYGVK', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (16, '01M5B1YZEH47P4DWJBAZWKYGVK', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (17, '01M5B1YZEH47P4DWJBB3TN3D9S', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (18, '01M5B1YZEH47P4DWJBB3TN3D9S', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (19, '01M5B1YZEH47P4DWJBB3TN3D9S', 0, 'subtitle', 'ass', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.ass', 0, 0);
INSERT INTO "tracks" VALUES (20, '01M5B1YZEH47P4DWJBB3TN3D9S', 1, 'subtitle', 'subrip', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.srt', 0, 0);
INSERT INTO "tracks" VALUES (21, '01M5B1YZEH47P4DWJBB3TN3D9S', 2, 'subtitle', 'ass', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.ass', 0, 0);
INSERT INTO "tracks" VALUES (22, '01M5B1YZEH47P4DWJBB3TN3D9S', 3, 'subtitle', 'subrip', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.srt', 0, 0);
INSERT INTO "tracks" VALUES (23, '01M5B1YZEJGF86TB3T7GS83GGH', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (24, '01M5B1YZEJGF86TB3T7GS83GGH', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (25, '01M5B1YZEJGF86TB3T7GS83GGH', 2, 'subtitle', 'subrip', 'ger', 'Deutsch (SRT)', 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (26, '01M5B1YZEJGF86TB3T7GS83GGH', 3, 'subtitle', 'subrip', 'eng', 'English (SRT)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (27, '01M5B1YZEJGF86TB3T7GS83GGH', 4, 'subtitle', 'ass', 'ger', 'Deutsch (ASS)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (28, '01M5B1YZEJGF86TB3T7GS83GGH', 5, 'subtitle', 'ass', 'eng', 'English (ASS)', 'embedded', NULL, 0, 0);
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
CREATE TABLE devices (
    id           TEXT    PRIMARY KEY CHECK (length(id) = 32),
    user_agent   TEXT,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL
) STRICT;
INSERT INTO "devices" VALUES ('55df89b3d6f4b0cb544744d688fbd2ac', 'Python-urllib/3.12', 1792446266671, 1792446266671);
INSERT INTO "devices" VALUES ('e08902277f98fd62d7603c4b867cda4d', 'Python-urllib/3.12', 1792446298428, 1792446298428);
INSERT INTO "devices" VALUES ('d0dd6a218bc0de1c7f5c616b93a9347f', 'Python-urllib/3.12', 1792446304018, 1792446304018);
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
INSERT INTO "sessions" VALUES (1, '181051d62ecc4afdaa833c3346667d408d344035e0bbce6b5f59c79dc7e01aa5', 1, 1, '55df89b3d6f4b0cb544744d688fbd2ac', 'Python-urllib/3.12', 1792446266671, 1792446266671, 1795038266671, NULL);
INSERT INTO "sessions" VALUES (2, 'dfc96659694956a47c4885398e8e86a1ce5e65738d1c37132588d022de68f09f', 1, NULL, 'e08902277f98fd62d7603c4b867cda4d', 'Python-urllib/3.12', 1792446298428, 1792446298428, 1795038298428, NULL);
INSERT INTO "sessions" VALUES (3, 'afb53f8d1163617a7e84203d5490c846c150d4f9d7e35bb1660d216d39769aad', 1, 1, 'd0dd6a218bc0de1c7f5c616b93a9347f', 'Python-urllib/3.12', 1792446304018, 1792446304020, 1795038304018, NULL);
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
CREATE TABLE invitation_libraries (
    invitation_id INTEGER NOT NULL REFERENCES invitations(id) ON DELETE CASCADE,
    library_id    INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    PRIMARY KEY (invitation_id, library_id)
) STRICT, WITHOUT ROWID;
CREATE TABLE ignored_files (
    root_id    INTEGER NOT NULL REFERENCES library_roots(id) ON DELETE CASCADE,
    rel_path   TEXT    NOT NULL,
    reason     TEXT    NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (root_id, rel_path)
) STRICT, WITHOUT ROWID;
INSERT INTO "ignored_files" VALUES (1, 'Slouchcat Test Removed (2024)/Removed.mkv', 'unplayable', 1792446345037);
CREATE TABLE trailers (
    id         INTEGER PRIMARY KEY,
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    lang       TEXT    NOT NULL CHECK (length(lang) BETWEEN 2 AND 3),
    youtube_id TEXT    NOT NULL CHECK (length(youtube_id) BETWEEN 8 AND 16),
    title      TEXT,
    source     TEXT    NOT NULL CHECK (source IN ('tmdb', 'youtube', 'ai', 'manual')),
    official   INTEGER NOT NULL DEFAULT 0 CHECK (official IN (0, 1)),
    created_at INTEGER NOT NULL,
    UNIQUE (item_id, lang, youtube_id)
) STRICT;
INSERT INTO "trailers" VALUES (1, '01M5B1YZED8098W4HE86F16SY1', 'de', 'DeTrailerH264', 'Trailer de', 'youtube', 0, 1792446345037);
INSERT INTO "trailers" VALUES (2, '01M5B1YZED8098W4HE86F16SY1', 'en', 'EnTrailerH264', 'Trailer en', 'tmdb', 1, 1792446345037);
INSERT INTO "trailers" VALUES (3, '01M5C0000000000000000SER01', 'de', 'DeTrailerShow', 'Trailer de', 'manual', 0, 1792446345037);
INSERT INTO "trailers" VALUES (4, '01M5C0000000000000000SER01', 'en', 'EnTrailerShow', 'Trailer en', 'tmdb', 1, 1792446345037);
CREATE TABLE people (
    id           INTEGER PRIMARY KEY,
    tmdb_id      INTEGER UNIQUE,
    name         TEXT    NOT NULL,
    profile_path TEXT,
    created_at   INTEGER NOT NULL
) STRICT;
INSERT INTO "people" VALUES (1, 70001, 'Pat Example', '/pat.jpg', 1792446345037);
INSERT INTO "people" VALUES (2, 70002, 'Kim Beispiel', NULL, 1792446345037);
CREATE TABLE credits (
    item_id   TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role      TEXT    NOT NULL CHECK (role IN ('cast', 'director', 'writer', 'creator')),
    character TEXT,
    ord       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (item_id, person_id, role)
) STRICT, WITHOUT ROWID;
INSERT INTO "credits" VALUES ('01M5B1YZED8098W4HE86F16SY1', 1, 'cast', 'Hauptfigur', 0);
INSERT INTO "credits" VALUES ('01M5B1YZED8098W4HE86F16SY1', 2, 'director', NULL, 0);
INSERT INTO "credits" VALUES ('01M5B1YZEFZFHX1Q6J8V08BQKW', 1, 'director', NULL, 0);
INSERT INTO "credits" VALUES ('01M5C0000000000000000SER01', 1, 'cast', 'Gast', 3);
INSERT INTO "credits" VALUES ('01M5C0000000000000000SER01', 2, 'creator', NULL, 0);
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
CREATE INDEX items_tvdb ON items (tvdb_id);
CREATE INDEX items_episode ON items (parent_id, season_number, episode_number);
CREATE INDEX trailers_item ON trailers (item_id, lang);
CREATE INDEX credits_person ON credits (person_id);
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
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = OLD.parent_id AND OLD.parent_id IS NOT NEW.parent_id;
END;
CREATE TRIGGER items_container_update AFTER UPDATE OF available, effective_rating, deleted_at ON items
WHEN NEW.parent_id IS NOT NULL
BEGIN
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = NEW.parent_id;
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = (SELECT p.parent_id FROM items AS p WHERE p.id = NEW.parent_id);
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = (SELECT g.parent_id FROM items AS g
                WHERE g.id = (SELECT p.parent_id FROM items AS p WHERE p.id = NEW.parent_id));
END;
CREATE TRIGGER items_container_delete AFTER DELETE ON items
WHEN OLD.parent_id IS NOT NULL
BEGIN
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(COALESCE(c.effective_rating, 18)) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = OLD.parent_id;
END;
