CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	);
INSERT INTO "goose_db_version" VALUES (1, 0, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (2, 1, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (3, 2, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (4, 3, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (5, 4, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (6, 5, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (7, 6, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (8, 7, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (9, 8, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (10, 9, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (11, 10, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (12, 11, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (13, 12, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (14, 13, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (15, 14, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (16, 15, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (17, 16, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (18, 17, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (19, 18, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (20, 19, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (21, 20, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (22, 21, 1, '2026-10-28 17:06:41 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (23, 22, 1, '2026-10-28 17:06:41 +0000 UTC');
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
INSERT INTO "jobs" VALUES (1, 'library.sweep', 2, 'sweep', '{}', 'done', 1793207202386, NULL, NULL, 1, 5, NULL, 1793207202386, 1793207203384);
INSERT INTO "jobs" VALUES (2, 'scan.library', 5, 'scan:1', '{"libraryId":1}', 'done', 1793207202589, NULL, NULL, 1, 5, NULL, 1793207202589, 1793207203402);
INSERT INTO "jobs" VALUES (3, 'metadata.match', 4, 'match:01M61QMXHV788R0T9NKS9WVPDF', '{"itemId":"01M61QMXHV788R0T9NKS9WVPDF"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203402);
INSERT INTO "jobs" VALUES (4, 'probe.file', 3, 'probe:01M61QMXHW529E06C1TT5F77F8', '{"mediaFileId":"01M61QMXHW529E06C1TT5F77F8"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203435);
INSERT INTO "jobs" VALUES (5, 'metadata.match', 4, 'match:01M61QMXHW529E06C1TTZENJ4W', '{"itemId":"01M61QMXHW529E06C1TTZENJ4W"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203402);
INSERT INTO "jobs" VALUES (6, 'probe.file', 3, 'probe:01M61QMXHXQ7DCK6RFVZV0SFWN', '{"mediaFileId":"01M61QMXHXQ7DCK6RFVZV0SFWN"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203467);
INSERT INTO "jobs" VALUES (7, 'metadata.match', 4, 'match:01M61QMXHXQ7DCK6RFW2Z6A0C8', '{"itemId":"01M61QMXHXQ7DCK6RFW2Z6A0C8"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203402);
INSERT INTO "jobs" VALUES (8, 'probe.file', 3, 'probe:01M61QMXHXQ7DCK6RFW5TWN5TH', '{"mediaFileId":"01M61QMXHXQ7DCK6RFW5TWN5TH"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203498);
INSERT INTO "jobs" VALUES (9, 'metadata.match', 4, 'match:01M61QMXHY1YQNRNJ99SP5Z1KR', '{"itemId":"01M61QMXHY1YQNRNJ99SP5Z1KR"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203402);
INSERT INTO "jobs" VALUES (10, 'probe.file', 3, 'probe:01M61QMXHY1YQNRNJ99X9DAJWN', '{"mediaFileId":"01M61QMXHY1YQNRNJ99X9DAJWN"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203536);
INSERT INTO "jobs" VALUES (11, 'metadata.match', 4, 'match:01M61QMXHY1YQNRNJ99ZTJTJF9', '{"itemId":"01M61QMXHY1YQNRNJ99ZTJTJF9"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203403);
INSERT INTO "jobs" VALUES (12, 'probe.file', 3, 'probe:01M61QMXHZPGMZV70B8KX5PCQ1', '{"mediaFileId":"01M61QMXHZPGMZV70B8KX5PCQ1"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203567);
INSERT INTO "jobs" VALUES (13, 'metadata.match', 4, 'match:01M61QMXHZPGMZV70B8MJH2H5Y', '{"itemId":"01M61QMXHZPGMZV70B8MJH2H5Y"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203403);
INSERT INTO "jobs" VALUES (14, 'probe.file', 3, 'probe:01M61QMXHZPGMZV70B8MKX2N1V', '{"mediaFileId":"01M61QMXHZPGMZV70B8MKX2N1V"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203596);
INSERT INTO "jobs" VALUES (15, 'probe.file', 3, 'probe:01M61QMXHZPGMZV70B8Q9QH6RT', '{"mediaFileId":"01M61QMXHZPGMZV70B8Q9QH6RT"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203626);
INSERT INTO "jobs" VALUES (16, 'metadata.match', 4, 'match:01M61QMXJ00FCWEMS5ETKXE1MH', '{"itemId":"01M61QMXJ00FCWEMS5ETKXE1MH"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203403);
INSERT INTO "jobs" VALUES (17, 'probe.file', 3, 'probe:01M61QMXJ00FCWEMS5EYJC2P65', '{"mediaFileId":"01M61QMXJ00FCWEMS5EYJC2P65"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203673);
INSERT INTO "jobs" VALUES (18, 'metadata.match', 4, 'match:01M61QMXJ00FCWEMS5F19XNGNY', '{"itemId":"01M61QMXJ00FCWEMS5F19XNGNY"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203403);
INSERT INTO "jobs" VALUES (19, 'probe.file', 3, 'probe:01M61QMXJ00FCWEMS5F2P7KS9B', '{"mediaFileId":"01M61QMXJ00FCWEMS5F2P7KS9B"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203723);
INSERT INTO "jobs" VALUES (20, 'metadata.match', 4, 'match:01M61QMXJ1DP340ZJK91AE5JZQ', '{"itemId":"01M61QMXJ1DP340ZJK91AE5JZQ"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203403);
INSERT INTO "jobs" VALUES (21, 'probe.file', 3, 'probe:01M61QMXJ1DP340ZJK92X57HV9', '{"mediaFileId":"01M61QMXJ1DP340ZJK92X57HV9"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203770);
INSERT INTO "jobs" VALUES (22, 'metadata.match', 4, 'match:01M61QMXJ1DP340ZJK96KJWB2B', '{"itemId":"01M61QMXJ1DP340ZJK96KJWB2B"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203404);
INSERT INTO "jobs" VALUES (23, 'probe.file', 3, 'probe:01M61QMXJ2N3Z9RF4YK8906MSK', '{"mediaFileId":"01M61QMXJ2N3Z9RF4YK8906MSK"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (24, 'metadata.match', 4, 'match:01M61QMXJ2N3Z9RF4YKAR84QES', '{"itemId":"01M61QMXJ2N3Z9RF4YKAR84QES"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203404);
INSERT INTO "jobs" VALUES (25, 'probe.file', 3, 'probe:01M61QMXJ2N3Z9RF4YKEPT1QB1', '{"mediaFileId":"01M61QMXJ2N3Z9RF4YKEPT1QB1"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (26, 'metadata.match', 4, 'match:01M61QMXJ2N3Z9RF4YKHPS2SM3', '{"itemId":"01M61QMXJ2N3Z9RF4YKHPS2SM3"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203404);
INSERT INTO "jobs" VALUES (27, 'probe.file', 3, 'probe:01M61QMXJ31E963MFP3Q38R5R4', '{"mediaFileId":"01M61QMXJ31E963MFP3Q38R5R4"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (28, 'metadata.match', 4, 'match:01M61QMXJ31E963MFP3TQAVXGT', '{"itemId":"01M61QMXJ31E963MFP3TQAVXGT"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203404);
INSERT INTO "jobs" VALUES (29, 'probe.file', 3, 'probe:01M61QMXJ31E963MFP3XB885MQ', '{"mediaFileId":"01M61QMXJ31E963MFP3XB885MQ"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (30, 'metadata.match', 4, 'match:01M61QMXJ31E963MFP3Y3007G5', '{"itemId":"01M61QMXJ31E963MFP3Y3007G5"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203404);
INSERT INTO "jobs" VALUES (31, 'probe.file', 3, 'probe:01M61QMXJ4BGYQS37TB9BDS0RW', '{"mediaFileId":"01M61QMXJ4BGYQS37TB9BDS0RW"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (32, 'metadata.match', 4, 'match:01M61QMXJ4BGYQS37TBCYGJYQJ', '{"itemId":"01M61QMXJ4BGYQS37TBCYGJYQJ"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203405);
INSERT INTO "jobs" VALUES (33, 'probe.file', 3, 'probe:01M61QMXJ4BGYQS37TBDK49TJ8', '{"mediaFileId":"01M61QMXJ4BGYQS37TBDK49TJ8"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (34, 'metadata.match', 4, 'match:01M61QMXJ53XFY20BBQYJJCP0S', '{"itemId":"01M61QMXJ53XFY20BBQYJJCP0S"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203405);
INSERT INTO "jobs" VALUES (35, 'probe.file', 3, 'probe:01M61QMXJ53XFY20BBR0NBM0H9', '{"mediaFileId":"01M61QMXJ53XFY20BBR0NBM0H9"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (36, 'metadata.match', 4, 'match:01M61QMXJ53XFY20BBR3M39V8J', '{"itemId":"01M61QMXJ53XFY20BBR3M39V8J"}', 'done', 1793207203387, NULL, NULL, 1, 5, NULL, 1793207203387, 1793207203405);
INSERT INTO "jobs" VALUES (37, 'probe.file', 3, 'probe:01M61QMXJ6X5M0AV1M92JNVFX6', '{"mediaFileId":"01M61QMXJ6X5M0AV1M92JNVFX6"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (38, 'probe.file', 3, 'probe:01M61QMXJ6X5M0AV1M97Q3JPKT', '{"mediaFileId":"01M61QMXJ6X5M0AV1M97Q3JPKT"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (39, 'probe.file', 3, 'probe:01M61QMXJ7VF4V5PMG76ZW25ZG', '{"mediaFileId":"01M61QMXJ7VF4V5PMG76ZW25ZG"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (40, 'probe.file', 3, 'probe:01M61QMXJ8K37KNBWKCEXVAC37', '{"mediaFileId":"01M61QMXJ8K37KNBWKCEXVAC37"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (41, 'probe.file', 3, 'probe:01M61QMXJ8K37KNBWKCK1NYFEF', '{"mediaFileId":"01M61QMXJ8K37KNBWKCK1NYFEF"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (42, 'probe.file', 3, 'probe:01M61QMXJ9Y3RFVFZ838VXRNWW', '{"mediaFileId":"01M61QMXJ9Y3RFVFZ838VXRNWW"}', 'queued', 1793207203387, NULL, NULL, 0, 5, NULL, 1793207203387, 1793207203387);
INSERT INTO "jobs" VALUES (43, 'probe.keyframes', 1, 'keyframes:01M61QMXHW529E06C1TT5F77F8', '{"mediaFileId":"01M61QMXHW529E06C1TT5F77F8"}', 'queued', 1793207203434, NULL, NULL, 0, 5, NULL, 1793207203434, 1793207203434);
INSERT INTO "jobs" VALUES (44, 'probe.keyframes', 1, 'keyframes:01M61QMXHXQ7DCK6RFW5TWN5TH', '{"mediaFileId":"01M61QMXHXQ7DCK6RFW5TWN5TH"}', 'queued', 1793207203497, NULL, NULL, 0, 5, NULL, 1793207203497, 1793207203497);
INSERT INTO "jobs" VALUES (45, 'probe.keyframes', 1, 'keyframes:01M61QMXHY1YQNRNJ99X9DAJWN', '{"mediaFileId":"01M61QMXHY1YQNRNJ99X9DAJWN"}', 'queued', 1793207203536, NULL, NULL, 0, 5, NULL, 1793207203536, 1793207203536);
INSERT INTO "jobs" VALUES (46, 'probe.keyframes', 1, 'keyframes:01M61QMXHZPGMZV70B8KX5PCQ1', '{"mediaFileId":"01M61QMXHZPGMZV70B8KX5PCQ1"}', 'queued', 1793207203567, NULL, NULL, 0, 5, NULL, 1793207203567, 1793207203567);
INSERT INTO "jobs" VALUES (47, 'probe.keyframes', 1, 'keyframes:01M61QMXHZPGMZV70B8MKX2N1V', '{"mediaFileId":"01M61QMXHZPGMZV70B8MKX2N1V"}', 'queued', 1793207203596, NULL, NULL, 0, 5, NULL, 1793207203596, 1793207203596);
INSERT INTO "jobs" VALUES (48, 'probe.keyframes', 1, 'keyframes:01M61QMXHZPGMZV70B8Q9QH6RT', '{"mediaFileId":"01M61QMXHZPGMZV70B8Q9QH6RT"}', 'queued', 1793207203626, NULL, NULL, 0, 5, NULL, 1793207203626, 1793207203626);
INSERT INTO "jobs" VALUES (49, 'probe.keyframes', 1, 'keyframes:01M61QMXJ00FCWEMS5EYJC2P65', '{"mediaFileId":"01M61QMXJ00FCWEMS5EYJC2P65"}', 'queued', 1793207203672, NULL, NULL, 0, 5, NULL, 1793207203672, 1793207203672);
INSERT INTO "jobs" VALUES (50, 'probe.keyframes', 1, 'keyframes:01M61QMXJ00FCWEMS5F2P7KS9B', '{"mediaFileId":"01M61QMXJ00FCWEMS5F2P7KS9B"}', 'queued', 1793207203723, NULL, NULL, 0, 5, NULL, 1793207203723, 1793207203723);
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
, pin_hash TEXT, max_bitrate_kbps INTEGER CHECK (max_bitrate_kbps IS NULL OR max_bitrate_kbps > 0), max_height INTEGER CHECK (max_height IS NULL OR max_height IN (360, 480, 720, 1080, 2160)), max_streams INTEGER CHECK (max_streams IS NULL OR max_streams BETWEEN 1 AND 16), watch_limit_minutes INTEGER CHECK (watch_limit_minutes IS NULL OR watch_limit_minutes BETWEEN 1 AND 1440), pin_salt TEXT CHECK (pin_salt IS NULL OR length(pin_salt) = 32), offline_pin TEXT CHECK (offline_pin IS NULL OR json_valid(offline_pin))) STRICT;
INSERT INTO "profiles" VALUES (1, 1, 'frozen', NULL, 18, 'de', NULL, 1, 1, 1793207202588, 1793207202588, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "profiles" VALUES (2, 1, 'Kind', NULL, 6, 'de', NULL, 1, 0, 1793207203623, 1793207203623, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "profiles" VALUES (3, 2, 'Konto', NULL, 12, 'de', NULL, 1, 1, 1793207203676, 1793207203676, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "profiles" VALUES (4, 3, 'Zweiter', NULL, 18, 'de', NULL, 1, 1, 1793207203731, 1793207203731, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
CREATE TABLE profile_libraries (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    library_id INTEGER NOT NULL,
    PRIMARY KEY (profile_id, library_id)
) STRICT, WITHOUT ROWID;
CREATE TABLE library_roots (
    id         INTEGER PRIMARY KEY,
    library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    path       TEXT    NOT NULL UNIQUE,
    fs_type    TEXT    NOT NULL DEFAULT 'unknown',
    watch      TEXT    NOT NULL DEFAULT 'poll' CHECK (watch IN ('inotify', 'poll', 'inotify_poll')),
    created_at INTEGER NOT NULL
) STRICT;
INSERT INTO "library_roots" VALUES (1, 1, '/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/testdata/media/Movies', 'ext4', 'inotify', 1793207202589);
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
    CHECK (trailer_state IN ('pending', 'found', 'none', 'failed')), trailer_checked_at INTEGER, segments_state TEXT NOT NULL DEFAULT 'pending' CHECK (segments_state IN ('pending', 'done', 'none', 'failed')), segments_checked_at INTEGER, author TEXT, narrator TEXT, series_name TEXT, series_index REAL, explicit INTEGER NOT NULL DEFAULT 0 CHECK (explicit IN (0, 1)), duration_ms INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0), cover_url TEXT, cover_path TEXT, feed_url TEXT, feed_etag TEXT, feed_modified TEXT, feed_checked_at INTEGER, feed_error TEXT, guid TEXT, published_at INTEGER, enclosure_type TEXT, enclosure_length INTEGER CHECK (enclosure_length IS NULL OR enclosure_length >= 0)) STRICT;
INSERT INTO "items" VALUES ('01M61QMXHV788R0T9NKS9WVPDF', 'movie', NULL, 1, 'Slouchcat Test AC3 51', 'slouchcat test ac3 51', NULL, 2024, NULL, NULL, '[]', 7001, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203402, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXHW529E06C1TTZENJ4W', 'movie', NULL, 1, 'Slouchcat Test Broken', 'slouchcat test broken', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203402, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXHXQ7DCK6RFW2Z6A0C8', 'movie', NULL, 1, 'Slouchcat Test Chapters', 'slouchcat test chapters', NULL, 2024, NULL, NULL, '[]', 7004, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203402, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXHY1YQNRNJ99SP5Z1KR', 'movie', NULL, 1, 'Slouchcat Test DTS', 'slouchcat test dts', NULL, 2024, NULL, NULL, '[]', 7002, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203402, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXHY1YQNRNJ99ZTJTJF9', 'movie', NULL, 1, 'Slouchcat Test Extras', 'slouchcat test extras', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203403, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXHZPGMZV70B8MJH2H5Y', 'movie', NULL, 1, 'Slouchcat Test H264', 'slouchcat test h264', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203403, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ00FCWEMS5ETKXE1MH', 'movie', NULL, 1, 'Slouchcat Test H264 Copy', 'slouchcat test h264 copy', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203403, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ00FCWEMS5F19XNGNY', 'movie', NULL, 1, 'Slouchcat Test H264 MKV Long', 'slouchcat test h264 mkv long', NULL, 2024, NULL, NULL, '[]', 7005, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203403, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ1DP340ZJK91AE5JZQ', 'movie', NULL, 1, 'Slouchcat Test HDR10 4K', 'slouchcat test hdr10 4k', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203403, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ1DP340ZJK96KJWB2B', 'movie', NULL, 1, 'Slouchcat Test HDR10 Chart', 'slouchcat test hdr10 chart', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203404, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ2N3Z9RF4YKAR84QES', 'movie', NULL, 1, 'Slouchcat Test HEVC 10bit', 'slouchcat test hevc 10bit', NULL, 2024, NULL, NULL, '[]', 7003, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203404, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ2N3Z9RF4YKHPS2SM3', 'movie', NULL, 1, 'Slouchcat Test HEVC Long', 'slouchcat test hevc long', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203404, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ31E963MFP3TQAVXGT', 'movie', NULL, 1, 'Slouchcat Test Multi Audio', 'slouchcat test multi audio', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203404, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ31E963MFP3Y3007G5', 'movie', NULL, 1, 'Slouchcat Test PGS', 'slouchcat test pgs', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203404, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ4BGYQS37TBCYGJYQJ', 'movie', NULL, 1, 'Slouchcat Test Sidecar Subtitles', 'slouchcat test sidecar subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203405, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ53XFY20BBQYJJCP0S', 'movie', NULL, 1, 'Slouchcat Test Subtitles', 'slouchcat test subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203405, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ53XFY20BBR3M39V8J', 'movie', NULL, 1, 'Slouchcat Test Unrated', 'slouchcat test unrated', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793207203387, 1793207203387, 1793207203405, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ6X5M0AV1M95ABX8GB', 'extra', '01M61QMXHY1YQNRNJ99ZTJTJF9', 1, 'On Set', 'on set', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793207203387, 1793207203387, 1793207203387, NULL, NULL, NULL, NULL, NULL, 'behind_the_scenes', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ7VF4V5PMG74Y2EHKY', 'extra', '01M61QMXHY1YQNRNJ99ZTJTJF9', 1, 'Scene 1', 'scene 1', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793207203387, 1793207203387, 1793207203387, NULL, NULL, NULL, NULL, NULL, 'deleted_scene', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ7VF4V5PMG7ADYYP21', 'extra', '01M61QMXHY1YQNRNJ99ZTJTJF9', 1, 'Making Of', 'making of', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793207203387, 1793207203387, 1793207203387, NULL, NULL, NULL, NULL, NULL, 'featurette', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ8K37KNBWKCHAMK2D5', 'extra', '01M61QMXHY1YQNRNJ99ZTJTJF9', 1, 'Slouchcat Test Extras (2024)', 'slouchcat test extras (2024)', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793207203387, 1793207203387, 1793207203387, NULL, NULL, NULL, NULL, NULL, 'trailer', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M61QMXJ8K37KNBWKCP298YP4', 'extra', '01M61QMXHY1YQNRNJ99ZTJTJF9', 1, 'Slouchcat Test Extras Trailer', 'slouchcat test extras trailer', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793207203387, 1793207203387, 1793207203387, NULL, NULL, NULL, NULL, NULL, 'trailer', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
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
    updated_at    INTEGER NOT NULL, probe_error TEXT, part_index INTEGER CHECK (part_index IS NULL OR part_index >= 1),
    UNIQUE (root_id, rel_path)
) STRICT;
INSERT INTO "media_files" VALUES ('01M61QMXHW529E06C1TT5F77F8', '01M61QMXHV788R0T9NKS9WVPDF', 1, 'Slouchcat Test AC3 51 (2024) {tmdb-7001}/Slouchcat Test AC3 51 (2024) {tmdb-7001}.mkv', 66310, 11170968, 2943036, 1792567467886, 'fcff3d5f3ec794e13b57448aab756b50a51e75ba226625bf03c326ceff99b749', NULL, 'mkv', 'h264', 'ac3', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":2943036,"bitRate":2354428,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test AC3 51"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"ac3","channels":6,"channelLayout":"5.1(side)","sampleRate":48000}],"chapters":[]}', NULL, 1793207203434, NULL, NULL, 1793207203387, 1793207203434, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXHXQ7DCK6RFVZV0SFWN', '01M61QMXHW529E06C1TTZENJ4W', 1, 'Slouchcat Test Broken (2024)/Slouchcat Test Broken (2024).mkv', 66310, 11160613, 57, 1792447202046, '8ed0f89e2cd4d6f84fa244afd8b1a2d46d26aaf54bf6537fc72508bff522218f', NULL, 'mkv', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203467, NULL, NULL, 1793207203387, 1793207203467, 'ffprobe: exit status 1: Truncating packet of size 12149 to 53
[matroska,webm @ 0x5750224ef680] EBML header parsing failed
Slouchcat Test Broken (2024).mkv: Invalid data found when processing input', NULL);
INSERT INTO "media_files" VALUES ('01M61QMXHXQ7DCK6RFW5TWN5TH', '01M61QMXHXQ7DCK6RFW2Z6A0C8', 1, 'Slouchcat Test Chapters (2024) {tmdb-7004}/Slouchcat Test Chapters (2024) {tmdb-7004}.mkv', 66310, 2121833, 2623898, 1792345529388, 'c2bb87ccf56f8bb95c24c9b0b9cbd63020f2aef72cb50c08fee9773851895722', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2623898,"bitRate":2094719,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test Chapters"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[{"start":0,"end":3000000000,"title":"Kapitel 1: Anfang"},{"start":3000000000,"end":6500000000,"title":"Kapitel 2: Mitte"},{"start":6500000000,"end":10000000000,"title":"Kapitel 3: Ende"}]}', NULL, 1793207203497, NULL, NULL, 1793207203387, 1793207203497, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXHY1YQNRNJ99X9DAJWN', '01M61QMXHY1YQNRNJ99SP5Z1KR', 1, 'Slouchcat Test DTS (2024) {tmdb-7002}/Slouchcat Test DTS (2024) {tmdb-7002}.mkv', 66310, 11170970, 3427132, 1792567468635, '050784b234968847b5fb2c3c6581006a7449ee8aa5a261293f918109be077aec', NULL, 'mkv', 'h264', 'dts', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":3427132,"bitRate":2741705,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test DTS"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"dts","profile":"DTS","channels":6,"channelLayout":"5.1(side)","sampleRate":48000}],"chapters":[]}', NULL, 1793207203536, NULL, NULL, 1793207203387, 1793207203536, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXHZPGMZV70B8KX5PCQ1', '01M61QMXHY1YQNRNJ99ZTJTJF9', 1, 'Slouchcat Test Extras (2024)/Slouchcat Test Extras (2024).mp4', 66310, 11160599, 2624979, 1792447200551, 'b453a8980a2883e31feeb6f382dc29c9d4b7d8f789b0c1f827ac1bb81da1caf5', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624979,"bitRate":2099983,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test Extras"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', NULL, 1793207203567, NULL, NULL, 1793207203387, 1793207203567, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXHZPGMZV70B8MKX2N1V', '01M61QMXHZPGMZV70B8MJH2H5Y', 1, 'Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024) - 720p.mp4', 66310, 2127416, 904192, 1792447200023, '0186b76306ab4b1c44501e8fd1fa880c817b1aedff7a4d9ac5de3802a5ed11c6', '720p', 'mp4', 'h264', 'aac', 1280, 720, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":904192,"bitRate":723353,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test H264 720p"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', NULL, 1793207203596, NULL, NULL, 1793207203387, 1793207203596, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXHZPGMZV70B8Q9QH6RT', '01M61QMXHZPGMZV70B8MJH2H5Y', 1, 'Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024).mp4', 66310, 2121811, 2624977, 1792345514047, '1a3559a74acfe685061086a9246f1ad08d3f220ba78aad39e503e56f4400693b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624977,"bitRate":2099981,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test H264"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', NULL, 1793207203626, NULL, NULL, 1793207203387, 1793207203626, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ00FCWEMS5EYJC2P65', '01M61QMXJ00FCWEMS5ETKXE1MH', 1, 'Slouchcat Test H264 Copy (2024)/Slouchcat Test H264 Copy (2024).mp4', 66310, 11160615, 2624977, 1792447202049, '1a3559a74acfe685061086a9246f1ad08d3f220ba78aad39e503e56f4400693b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624977,"bitRate":2099981,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test H264"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', NULL, 1793207203672, NULL, NULL, 1793207203387, 1793207203672, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ00FCWEMS5F2P7KS9B', '01M61QMXJ00FCWEMS5F19XNGNY', 1, 'Slouchcat Test H264 MKV Long (2024) {tmdb-7005}/Slouchcat Test H264 MKV Long (2024) {tmdb-7005}.mkv', 66310, 3286276, 13446607, 1792427238596, '201c421ed5521ef87bf11cd453de79aec1def64bc511895e32e66d49175a6ac8', NULL, 'mkv', 'h264', 'aac', 1280, 720, 90021, '{"format":{"name":"matroska,webm","duration":90021000000,"size":13446607,"bitRate":1194975,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test H264 MKV Long"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', NULL, 1793207203723, NULL, NULL, 1793207203387, 1793207203723, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ1DP340ZJK92X57HV9', '01M61QMXJ1DP340ZJK91AE5JZQ', 1, 'Slouchcat Test HDR10 4K (2024)/Slouchcat Test HDR10 4K (2024).mkv', 66310, 2121816, 761065, 1792345526738, '4f5e90e46dfab8b657b36c94ac139833c1bef0954ccc3750969aedb3d4547f86', '2160p', 'mkv', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ2N3Z9RF4YK8906MSK', '01M61QMXJ1DP340ZJK96KJWB2B', 1, 'Slouchcat Test HDR10 Chart (2024)/Slouchcat Test HDR10 Chart (2024).mkv', 66310, 11170966, 262639, 1792521753392, '2b77b031c72a57284d5b8e0b255214e0207a212dcaf6dc0a2408a1436473ea5f', NULL, 'mkv', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ2N3Z9RF4YKEPT1QB1', '01M61QMXJ2N3Z9RF4YKAR84QES', 1, 'Slouchcat Test HEVC 10bit (2024) {tmdb-7003}/Slouchcat Test HEVC 10bit (2024) {tmdb-7003}.mkv', 66310, 2121814, 2102531, 1792345515157, '2469bfd53f3d50de5dd084a6dff5c3263c0d36fc93643768ce16469bfd62197a', NULL, 'mkv', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ31E963MFP3Q38R5R4', '01M61QMXJ2N3Z9RF4YKHPS2SM3', 1, 'Slouchcat Test HEVC Long (2024)/Slouchcat Test HEVC Long (2024).mkv', 66310, 3286272, 9588128, 1792427236518, '0c0cf1bda8fb7ba37b5b1edb86736944dc48d81450b7f2e05326e9a211cb65e2', NULL, 'mkv', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ31E963MFP3XB885MQ', '01M61QMXJ31E963MFP3TQAVXGT', 1, 'Slouchcat Test Multi Audio (2024)/Slouchcat Test Multi Audio (2024).mkv', 66310, 11170962, 6263167, 1792521741875, '14d2fd61ed9bbc0a3d81e7546c92ac8d8688712fbce5bd9da880edc57306a1de', NULL, 'mkv', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ4BGYQS37TB9BDS0RW', '01M61QMXJ31E963MFP3Y3007G5', 1, 'Slouchcat Test PGS (2024)/Slouchcat Test PGS (2024).mkv', 66310, 11170964, 5309544, 1792521743128, '4c7ffebec482d05dadf74754f1d9274039f21b884a5341a188ae9fd86f9ab397', NULL, 'mkv', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ4BGYQS37TBDK49TJ8', '01M61QMXJ4BGYQS37TBCYGJYQJ', 1, 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).mkv', 66310, 2121827, 2623761, 1792345528684, 'ddf64e1615182551e4bb7121f07c190c691f6b99117d3146e49e27d845311ec8', NULL, 'mkv', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ53XFY20BBR0NBM0H9', '01M61QMXJ53XFY20BBQYJJCP0S', 1, 'Slouchcat Test Subtitles (2024)/Slouchcat Test Subtitles (2024).mkv', 66310, 2121825, 2626565, 1792345528629, '369b617c7d7ad81e45b193ddbad680f77b77593fc0a095272d6199ab77521204', NULL, 'mkv', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ6X5M0AV1M92JNVFX6', '01M61QMXJ53XFY20BBR3M39V8J', 1, 'Slouchcat Test Unrated/Slouchcat Test Unrated.mp4', 66310, 11160617, 2625046, 1792447202569, '4c3cb756cbf73d3913ed4c77ad5fd261f37d7f3d9e1e5c6eb988fccc17da2577', NULL, 'mp4', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ6X5M0AV1M97Q3JPKT', '01M61QMXJ6X5M0AV1M95ABX8GB', 1, 'Slouchcat Test Extras (2024)/Behind The Scenes/On Set.mp4', 66310, 11160609, 98171, 1792447201742, '30049f1b99ded9fca7c51ea76d1282b28f6a444a07e12d477d8401727308ddd8', NULL, 'mp4', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ7VF4V5PMG76ZW25ZG', '01M61QMXJ7VF4V5PMG74Y2EHKY', 1, 'Slouchcat Test Extras (2024)/Deleted Scenes/Scene 1.mp4', 66310, 11160607, 98172, 1792447201432, '20e54a28024662e3d88b81bcb6f92dc549074fc06d0e0351c2f5ccb505de5a73', NULL, 'mp4', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ8K37KNBWKCEXVAC37', '01M61QMXJ7VF4V5PMG7ADYYP21', 1, 'Slouchcat Test Extras (2024)/Featurettes/Making Of.mp4', 66310, 11160603, 98174, 1792447201131, '4d22df8b4b0150f430bcdcffa84101300a26dec5d108f9714d1bf7d779b6ab71', NULL, 'mp4', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ8K37KNBWKCK1NYFEF', '01M61QMXJ8K37KNBWKCHAMK2D5', 1, 'Slouchcat Test Extras (2024)/Slouchcat Test Extras (2024)-trailer.mp4', 66310, 11160610, 98201, 1792447202030, 'b5d9ae88034b726cf9e84ac537f185f76ea8baac390744ac752990784a124e8b', NULL, 'mp4', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M61QMXJ9Y3RFVFZ838VXRNWW', '01M61QMXJ8K37KNBWKCP298YP4', 1, 'Slouchcat Test Extras (2024)/Trailers/Slouchcat Test Extras Trailer.mp4', 66310, 11160601, 98194, 1792447200840, '254ced3455cc586e45a5f98fcb8547cfc93da14d64c7a58c7b7f1e911361ef2b', NULL, 'mp4', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793207203387, 1793207203387, NULL, NULL);
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
INSERT INTO "tracks" VALUES (1, '01M61QMXHW529E06C1TT5F77F8', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (2, '01M61QMXHW529E06C1TT5F77F8', 1, 'audio', 'ac3', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (3, '01M61QMXHXQ7DCK6RFW5TWN5TH', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (4, '01M61QMXHXQ7DCK6RFW5TWN5TH', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (5, '01M61QMXHY1YQNRNJ99X9DAJWN', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (6, '01M61QMXHY1YQNRNJ99X9DAJWN', 1, 'audio', 'dts', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (7, '01M61QMXHZPGMZV70B8KX5PCQ1', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (8, '01M61QMXHZPGMZV70B8KX5PCQ1', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (9, '01M61QMXHZPGMZV70B8MKX2N1V', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (10, '01M61QMXHZPGMZV70B8MKX2N1V', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (11, '01M61QMXHZPGMZV70B8Q9QH6RT', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (12, '01M61QMXHZPGMZV70B8Q9QH6RT', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (13, '01M61QMXJ00FCWEMS5EYJC2P65', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (14, '01M61QMXJ00FCWEMS5EYJC2P65', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (15, '01M61QMXJ00FCWEMS5F2P7KS9B', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (16, '01M61QMXJ00FCWEMS5F2P7KS9B', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
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
    updated_at  INTEGER NOT NULL, playback_rate REAL
    CHECK (playback_rate IS NULL OR (playback_rate >= 0.5 AND playback_rate <= 3.0)),
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
, playback_session_id TEXT, offline INTEGER NOT NULL DEFAULT 0 CHECK (offline IN (0, 1)), uploaded_at INTEGER, download_id TEXT) STRICT;
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
, quality TEXT CHECK (quality IS NULL OR quality IN ('auto', '1080p', '720p', '480p', '360p')), width INTEGER, height INTEGER, bitrate INTEGER, hardware INTEGER NOT NULL DEFAULT 0 CHECK (hardware IN (0, 1)), audio_track INTEGER, subtitle_track INTEGER, subtitle_delivery TEXT CHECK (subtitle_delivery IS NULL OR subtitle_delivery IN ('text', 'burn')), capabilities TEXT CHECK (capabilities IS NULL OR json_valid(capabilities)), position_ms INTEGER, stopped_by TEXT CHECK (stopped_by IS NULL OR stopped_by IN ('client', 'admin', 'limit', 'logout', 'revoked'))) STRICT;
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
, name TEXT CHECK (name IS NULL OR length(name) <= 80), type TEXT CHECK (type IS NULL OR type IN ('desktop', 'phone', 'tablet', 'tv', 'other')), account_id INTEGER REFERENCES accounts(id) ON DELETE SET NULL, profile_id INTEGER REFERENCES profiles(id) ON DELETE SET NULL, capabilities TEXT CHECK (capabilities IS NULL OR json_valid(capabilities)), capabilities_at INTEGER) STRICT;
INSERT INTO "devices" VALUES ('a9e8344c585478a265842d8e65c12cad', 'curl/8.5.0', 1793207202589, 1793207202589, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "devices" VALUES ('e3ce44c199ff44426b87f841106a92a8', 'curl/8.5.0', 1793207203676, 1793207203676, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "devices" VALUES ('48464a9fadc7b8df15dd8433962cda09', 'curl/8.5.0', 1793207203732, 1793207203732, NULL, NULL, NULL, NULL, NULL, NULL);
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
INSERT INTO "sessions" VALUES (1, 'da70829f272936e630843c1770cae9e1363e0342019616583208c1902a972dd6', 1, 1, 'a9e8344c585478a265842d8e65c12cad', 'curl/8.5.0', 1793207202589, 1793207202589, 1795799202589, NULL);
INSERT INTO "sessions" VALUES (2, 'c65fd76c7fd5d54af45ad5825e205465614a3d98ed9eaf985e64fb16035a26e8', 2, 3, 'e3ce44c199ff44426b87f841106a92a8', 'curl/8.5.0', 1793207203676, 1793207203676, 1795799203676, NULL);
INSERT INTO "sessions" VALUES (3, '3ecb002de3722c28a033af62c9cfc44312faffd5e4ac8db66fb1c5e6e35a90f8', 3, 4, '48464a9fadc7b8df15dd8433962cda09', 'curl/8.5.0', 1793207203732, 1793207203732, 1795799203732, NULL);
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
INSERT INTO "invitations" VALUES (1, '1f75277984d3d3fa83170408ff2faf2c501af5fa8b8fcc3d82b0d66b4c36280d', 'Konto', 12, 1, 1, 1793207203627, 1793812003627, 1793207203676, 2, NULL);
INSERT INTO "invitations" VALUES (2, '4679e35a7be63e711b6cbacfb0709ff947bc4f8c678fced664147e1de68d817c', 'Zweiter Admin', 18, 1, 1, 1793207203682, 1793812003682, 1793207203731, 3, NULL);
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
CREATE TABLE people (
    id           INTEGER PRIMARY KEY,
    tmdb_id      INTEGER UNIQUE,
    name         TEXT    NOT NULL,
    profile_path TEXT,
    created_at   INTEGER NOT NULL
) STRICT;
CREATE TABLE credits (
    item_id   TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role      TEXT    NOT NULL CHECK (role IN ('cast', 'director', 'writer', 'creator')),
    character TEXT,
    ord       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (item_id, person_id, role)
) STRICT, WITHOUT ROWID;
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
CREATE TABLE download_tombstones (
    id         TEXT    PRIMARY KEY CHECK (length(id) = 26),
    device_id  TEXT    NOT NULL,
    removed_at INTEGER NOT NULL
) STRICT, WITHOUT ROWID;
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
CREATE TABLE "libraries" (
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
INSERT INTO "libraries" VALUES (1, 'Filme', 'movies', NULL, 1793207202589, 1793207203401, 'done', 1793207203385, 1793207203401, NULL, 23, 23, NULL);
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
CREATE TABLE "downloads" (
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
CREATE TABLE watchlist (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (profile_id, item_id)
) STRICT, WITHOUT ROWID;
CREATE TABLE favorites (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (profile_id, item_id)
) STRICT, WITHOUT ROWID;
CREATE TABLE collections (
    id          TEXT    PRIMARY KEY CHECK (length(id) = 26),
    source      TEXT    NOT NULL CHECK (source IN ('tmdb', 'manual')),
    tmdb_id     INTEGER UNIQUE,
    name        TEXT    NOT NULL,
    name_de     TEXT,
    overview    TEXT,
    overview_de TEXT,
    poster_path TEXT,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
) STRICT;
CREATE TABLE collection_items (
    collection_id TEXT    NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
    item_id       TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    position      INTEGER NOT NULL DEFAULT 0,
    created_at    INTEGER NOT NULL,
    PRIMARY KEY (collection_id, item_id)
) STRICT, WITHOUT ROWID;
CREATE TABLE item_genres (
    item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    lang    TEXT NOT NULL CHECK (lang IN ('de', 'en')),
    name    TEXT NOT NULL,
    PRIMARY KEY (item_id, lang, name)
) STRICT, WITHOUT ROWID;
CREATE TABLE search_docs (
    id      INTEGER PRIMARY KEY,
    kind    TEXT    NOT NULL CHECK (kind IN ('item', 'text', 'person', 'collection')),
    item_id TEXT    NOT NULL,
    ref_id  TEXT    NOT NULL,
    text    TEXT    NOT NULL,
    UNIQUE (kind, ref_id, item_id)
) STRICT;
INSERT INTO "search_docs" VALUES (1, 'item', '01M61QMXHV788R0T9NKS9WVPDF', '01M61QMXHV788R0T9NKS9WVPDF', 'Slouchcat Test AC3 51 /  /  /  /');
INSERT INTO "search_docs" VALUES (2, 'item', '01M61QMXHW529E06C1TTZENJ4W', '01M61QMXHW529E06C1TTZENJ4W', 'Slouchcat Test Broken /  /  /  /');
INSERT INTO "search_docs" VALUES (3, 'item', '01M61QMXHXQ7DCK6RFW2Z6A0C8', '01M61QMXHXQ7DCK6RFW2Z6A0C8', 'Slouchcat Test Chapters /  /  /  /');
INSERT INTO "search_docs" VALUES (4, 'item', '01M61QMXHY1YQNRNJ99SP5Z1KR', '01M61QMXHY1YQNRNJ99SP5Z1KR', 'Slouchcat Test DTS /  /  /  /');
INSERT INTO "search_docs" VALUES (5, 'item', '01M61QMXHY1YQNRNJ99ZTJTJF9', '01M61QMXHY1YQNRNJ99ZTJTJF9', 'Slouchcat Test Extras /  /  /  /');
INSERT INTO "search_docs" VALUES (6, 'item', '01M61QMXHZPGMZV70B8MJH2H5Y', '01M61QMXHZPGMZV70B8MJH2H5Y', 'Slouchcat Test H264 /  /  /  /');
INSERT INTO "search_docs" VALUES (7, 'item', '01M61QMXJ00FCWEMS5ETKXE1MH', '01M61QMXJ00FCWEMS5ETKXE1MH', 'Slouchcat Test H264 Copy /  /  /  /');
INSERT INTO "search_docs" VALUES (8, 'item', '01M61QMXJ00FCWEMS5F19XNGNY', '01M61QMXJ00FCWEMS5F19XNGNY', 'Slouchcat Test H264 MKV Long /  /  /  /');
INSERT INTO "search_docs" VALUES (9, 'item', '01M61QMXJ1DP340ZJK91AE5JZQ', '01M61QMXJ1DP340ZJK91AE5JZQ', 'Slouchcat Test HDR10 4K /  /  /  /');
INSERT INTO "search_docs" VALUES (10, 'item', '01M61QMXJ1DP340ZJK96KJWB2B', '01M61QMXJ1DP340ZJK96KJWB2B', 'Slouchcat Test HDR10 Chart /  /  /  /');
INSERT INTO "search_docs" VALUES (11, 'item', '01M61QMXJ2N3Z9RF4YKAR84QES', '01M61QMXJ2N3Z9RF4YKAR84QES', 'Slouchcat Test HEVC 10bit /  /  /  /');
INSERT INTO "search_docs" VALUES (12, 'item', '01M61QMXJ2N3Z9RF4YKHPS2SM3', '01M61QMXJ2N3Z9RF4YKHPS2SM3', 'Slouchcat Test HEVC Long /  /  /  /');
INSERT INTO "search_docs" VALUES (13, 'item', '01M61QMXJ31E963MFP3TQAVXGT', '01M61QMXJ31E963MFP3TQAVXGT', 'Slouchcat Test Multi Audio /  /  /  /');
INSERT INTO "search_docs" VALUES (14, 'item', '01M61QMXJ31E963MFP3Y3007G5', '01M61QMXJ31E963MFP3Y3007G5', 'Slouchcat Test PGS /  /  /  /');
INSERT INTO "search_docs" VALUES (15, 'item', '01M61QMXJ4BGYQS37TBCYGJYQJ', '01M61QMXJ4BGYQS37TBCYGJYQJ', 'Slouchcat Test Sidecar Subtitles /  /  /  /');
INSERT INTO "search_docs" VALUES (16, 'item', '01M61QMXJ53XFY20BBQYJJCP0S', '01M61QMXJ53XFY20BBQYJJCP0S', 'Slouchcat Test Subtitles /  /  /  /');
INSERT INTO "search_docs" VALUES (17, 'item', '01M61QMXJ53XFY20BBR3M39V8J', '01M61QMXJ53XFY20BBR3M39V8J', 'Slouchcat Test Unrated /  /  /  /');
INSERT INTO "search_docs" VALUES (18, 'item', '01M61QMXJ6X5M0AV1M95ABX8GB', '01M61QMXJ6X5M0AV1M95ABX8GB', 'On Set /  /  /  /');
INSERT INTO "search_docs" VALUES (19, 'item', '01M61QMXJ7VF4V5PMG74Y2EHKY', '01M61QMXJ7VF4V5PMG74Y2EHKY', 'Scene 1 /  /  /  /');
INSERT INTO "search_docs" VALUES (20, 'item', '01M61QMXJ7VF4V5PMG7ADYYP21', '01M61QMXJ7VF4V5PMG7ADYYP21', 'Making Of /  /  /  /');
INSERT INTO "search_docs" VALUES (21, 'item', '01M61QMXJ8K37KNBWKCHAMK2D5', '01M61QMXJ8K37KNBWKCHAMK2D5', 'Slouchcat Test Extras (2024) /  /  /  /');
INSERT INTO "search_docs" VALUES (22, 'item', '01M61QMXJ8K37KNBWKCP298YP4', '01M61QMXJ8K37KNBWKCP298YP4', 'Slouchcat Test Extras Trailer /  /  /  /');
CREATE VIRTUAL TABLE search_index USING fts5(
    text,
    kind UNINDEXED,
    item_id UNINDEXED,
    ref_id UNINDEXED,
    content = 'search_docs',
    content_rowid = 'id',
    tokenize = 'unicode61 remove_diacritics 2',
    prefix = '2 3'
);
CREATE TABLE notifications (
    id         INTEGER PRIMARY KEY,
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL CHECK (kind IN ('new_episode')),
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    parent_id  TEXT    REFERENCES items(id) ON DELETE SET NULL,
    created_at INTEGER NOT NULL,
    read_at    INTEGER,
    UNIQUE (profile_id, kind, item_id)
) STRICT;
CREATE TABLE account_channels (
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    channel    TEXT    NOT NULL CHECK (channel IN ('email', 'push', 'telegram')),
    enabled    INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    events     TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(events) AND json_type(events) = 'array'),
    config     TEXT    NOT NULL DEFAULT '{}' CHECK (json_valid(config) AND json_type(config) = 'object'),
    updated_at INTEGER NOT NULL,
    PRIMARY KEY (account_id, channel)
) STRICT, WITHOUT ROWID;
INSERT INTO "account_channels" VALUES (1, 'email', 0, '["new_content","errors","invitations","announcements"]', '{}', 1793207202589);
INSERT INTO "account_channels" VALUES (1, 'push', 0, '["new_content","errors","invitations","announcements"]', '{}', 1793207202589);
INSERT INTO "account_channels" VALUES (1, 'telegram', 0, '["new_content","errors","invitations","announcements"]', '{}', 1793207202589);
INSERT INTO "account_channels" VALUES (2, 'email', 1, '["announcements"]', '{}', 1793207203676);
INSERT INTO "account_channels" VALUES (2, 'push', 0, '["announcements"]', '{}', 1793207203676);
INSERT INTO "account_channels" VALUES (2, 'telegram', 0, '["announcements"]', '{}', 1793207203676);
INSERT INTO "account_channels" VALUES (3, 'email', 1, '["announcements"]', '{}', 1793207203732);
INSERT INTO "account_channels" VALUES (3, 'push', 0, '["announcements"]', '{}', 1793207203732);
INSERT INTO "account_channels" VALUES (3, 'telegram', 0, '["announcements"]', '{}', 1793207203732);
CREATE TABLE telegram_links (
    account_id INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    code       TEXT    NOT NULL UNIQUE CHECK (length(code) = 6),
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
) STRICT;
CREATE TABLE push_subscriptions (
    id           INTEGER PRIMARY KEY,
    account_id   INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    device_id    TEXT    REFERENCES devices(id) ON DELETE SET NULL,
    endpoint     TEXT    NOT NULL UNIQUE,
    p256dh       TEXT    NOT NULL,
    auth         TEXT    NOT NULL,
    user_agent   TEXT,
    device_name  TEXT    CHECK (device_name IS NULL OR length(device_name) <= 80),
    created_at   INTEGER NOT NULL,
    last_used_at INTEGER,
    failures     INTEGER NOT NULL DEFAULT 0 CHECK (failures >= 0)
) STRICT;
CREATE TABLE announcements (
    id           INTEGER PRIMARY KEY,
    title        TEXT    NOT NULL CHECK (length(title) BETWEEN 1 AND 120),
    body         TEXT    NOT NULL CHECK (length(body) BETWEEN 1 AND 4000),
    created_by   INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    published_at INTEGER
) STRICT;
CREATE TABLE announcement_acks (
    announcement_id INTEGER NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    profile_id      INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    acked_at        INTEGER NOT NULL,
    PRIMARY KEY (announcement_id, profile_id)
) STRICT, WITHOUT ROWID;
CREATE TABLE webhook_integrations (
    id           INTEGER PRIMARY KEY,
    name         TEXT    NOT NULL UNIQUE CHECK (length(name) BETWEEN 1 AND 80),
    kind         TEXT    NOT NULL CHECK (kind IN ('radarr', 'sonarr')),
    token_hash   TEXT    NOT NULL UNIQUE CHECK (length(token_hash) = 64),
    token_tail   TEXT    NOT NULL CHECK (length(token_tail) = 4),
    path_map     TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(path_map) AND json_type(path_map) = 'array'),
    last_seen_at INTEGER,
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
) STRICT;
CREATE TABLE webhook_events (
    id             INTEGER PRIMARY KEY,
    integration_id INTEGER NOT NULL REFERENCES webhook_integrations(id) ON DELETE CASCADE,
    received_at    INTEGER NOT NULL,
    -- The event name as Radarr/Sonarr sent it (Download, Rename, Test, Grab …), kept verbatim so
    -- the log stays readable when a service adds one.
    event          TEXT    NOT NULL CHECK (length(event) <= 80),
    reported_path  TEXT,
    resolved_path  TEXT,
    result         TEXT    NOT NULL CHECK (result IN ('scan_queued', 'acknowledged', 'ignored',
                                                      'outside_roots', 'invalid_payload'))
) STRICT;
CREATE TABLE "deliveries" (
    id         INTEGER PRIMARY KEY,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id INTEGER REFERENCES profiles(id) ON DELETE SET NULL,
    channel    TEXT    NOT NULL CHECK (channel IN ('email', 'push', 'telegram')),
    event      TEXT    NOT NULL CHECK (event IN ('new_content', 'errors', 'invitations', 'announcements')),
    source     TEXT,
    subject    TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    link       TEXT,
    state      TEXT    NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'sent', 'failed', 'skipped')),
    attempts   INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error TEXT,
    created_at INTEGER NOT NULL,
    sent_at    INTEGER
) STRICT;
CREATE TABLE progress_days (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    device_id  TEXT    NOT NULL DEFAULT '',
    day        INTEGER NOT NULL,
    played_ms  INTEGER NOT NULL CHECK (played_ms >= 0),
    plays      INTEGER NOT NULL CHECK (plays >= 0),
    PRIMARY KEY (profile_id, item_id, device_id, day)
) STRICT, WITHOUT ROWID;
CREATE TABLE "home_rows" (
    key        TEXT    PRIMARY KEY CHECK (key IN ('spotlight', 'picks', 'continue', 'recent', 'released',
                                                  'watchlist', 'collections', 'genres')),
    position   INTEGER NOT NULL,
    hidden     INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)),
    updated_at INTEGER NOT NULL
) STRICT;
INSERT INTO "home_rows" VALUES ('picks', 1, 0, 0);
INSERT INTO "home_rows" VALUES ('continue', 2, 0, 0);
INSERT INTO "home_rows" VALUES ('recent', 3, 0, 0);
INSERT INTO "home_rows" VALUES ('watchlist', 5, 0, 0);
INSERT INTO "home_rows" VALUES ('collections', 6, 0, 0);
INSERT INTO "home_rows" VALUES ('genres', 7, 0, 0);
INSERT INTO "home_rows" VALUES ('spotlight', 0, 0, 0);
INSERT INTO "home_rows" VALUES ('released', 4, 0, 0);
CREATE TABLE spotlight_config (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    enabled             INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    list_key            TEXT    NOT NULL CHECK (list_key IN ('trending', 'popular', 'watched_weekly',
                                                             'collected_monthly', 'anticipated',
                                                             'boxoffice', 'url')),
    list_url            TEXT,
    media               TEXT    NOT NULL CHECK (media IN ('movie', 'show', 'both')),
    item_count          INTEGER NOT NULL CHECK (item_count BETWEEN 5 AND 20),
    min_hits            INTEGER NOT NULL CHECK (min_hits BETWEEN 0 AND 10),
    interval_hours      INTEGER NOT NULL CHECK (interval_hours BETWEEN 6 AND 48),
    last_manual_refresh INTEGER,
    updated_at          INTEGER NOT NULL
) STRICT;
INSERT INTO "spotlight_config" VALUES (1, 1, 'trending', NULL, 'both', 20, 5, 24, NULL, 0);
CREATE TABLE spotlight_entries (
    list_key   TEXT    NOT NULL,
    media      TEXT    NOT NULL CHECK (media IN ('movie', 'show')),
    position   INTEGER NOT NULL,
    tmdb_id    INTEGER NOT NULL,
    title      TEXT,
    year       INTEGER,
    fetched_at INTEGER NOT NULL,
    PRIMARY KEY (list_key, media, position)
) STRICT, WITHOUT ROWID;
CREATE TABLE spotlight_overrides (
    list_key    TEXT    NOT NULL,
    media       TEXT    NOT NULL CHECK (media IN ('movie', 'show')),
    tmdb_id     INTEGER NOT NULL,
    hidden      INTEGER NOT NULL DEFAULT 0 CHECK (hidden IN (0, 1)),
    pinned_rank INTEGER CHECK (pinned_rank IS NULL OR pinned_rank >= 1),
    note        TEXT    CHECK (note IS NULL OR length(note) <= 280),
    updated_at  INTEGER NOT NULL,
    PRIMARY KEY (list_key, media, tmdb_id)
) STRICT, WITHOUT ROWID;
CREATE TABLE "accounts" (
    id               INTEGER PRIMARY KEY,
    email            TEXT    NOT NULL COLLATE NOCASE UNIQUE,
    password_hash    TEXT    NOT NULL,            -- argon2id PHC string
    role             TEXT    NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    is_owner         INTEGER NOT NULL DEFAULT 0 CHECK (is_owner IN (0, 1)),
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,
    max_rating       INTEGER NOT NULL DEFAULT 18 CHECK (max_rating IN (0, 6, 12, 16, 18)),
    all_libraries    INTEGER NOT NULL DEFAULT 1 CHECK (all_libraries IN (0, 1)),
    invited_by       INTEGER REFERENCES accounts(id) ON DELETE SET NULL,
    last_seen_at     INTEGER,
    max_bitrate_kbps INTEGER CHECK (max_bitrate_kbps IS NULL OR max_bitrate_kbps > 0),
    max_height       INTEGER CHECK (max_height IS NULL OR max_height IN (360, 480, 720, 1080, 2160)),
    max_streams      INTEGER CHECK (max_streams IS NULL OR max_streams BETWEEN 1 AND 16),
    CHECK (role <> 'admin' OR (max_rating = 18 AND all_libraries = 1)),
    CHECK (is_owner = 0 OR role = 'admin')
) STRICT;
INSERT INTO "accounts" VALUES (1, 'frozen@example.test', '$argon2id$v=19$m=65536,t=3,p=4$x8vGEX2nGnbbj3rbg5xVSQ$0RYD+m0chuP7fKRUUz2rNnGkb0NfQBzFCgY4jE3EPWU', 'admin', 1, 1793207202588, 1793207202588, 18, 1, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "accounts" VALUES (2, 'konto@example.test', '$argon2id$v=19$m=65536,t=3,p=4$5/pL2e0ArFqIHVWk3BjrFA$S/Jc7zmNK00kbxPMGR1TbtUORF1cKJhU9KfDV+sQRhM', 'user', 0, 1793207203676, 1793207203769, 12, 1, 1, NULL, NULL, 720, 2);
INSERT INTO "accounts" VALUES (3, 'admin2@example.test', '$argon2id$v=19$m=65536,t=3,p=4$crPpU21KnWLe6Xn8J8WFvQ$0nhZXCZczjMHV+i7mvYZ3ZFrdObAPBGKNzVJM2h1n4Y', 'admin', 0, 1793207203731, 1793207203765, 18, 1, 1, NULL, NULL, NULL, NULL);
INSERT INTO "search_index"("search_index") VALUES ('rebuild');
CREATE UNIQUE INDEX jobs_active_idempotency_key ON jobs (idempotency_key)
    WHERE state IN ('queued', 'running');
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
CREATE INDEX progress_recent ON progress (profile_id, updated_at DESC);
CREATE INDEX progress_events_profile_item ON progress_events (profile_id, item_id, server_time DESC);
CREATE INDEX playback_sessions_session ON playback_sessions (session_id);
CREATE INDEX playback_sessions_expires ON playback_sessions (expires_at);
CREATE UNIQUE INDEX profiles_primary ON profiles (account_id) WHERE is_default = 1;
CREATE INDEX sessions_account ON sessions (account_id);
CREATE INDEX sessions_profile ON sessions (profile_id);
CREATE INDEX items_tvdb ON items (tvdb_id);
CREATE INDEX items_episode ON items (parent_id, season_number, episode_number);
CREATE INDEX trailers_item ON trailers (item_id, lang);
CREATE INDEX credits_person ON credits (person_id);
CREATE INDEX jobs_claim ON jobs (state, priority DESC, created_at, id);
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
        min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                            THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                            ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
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
        min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                            THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                            ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = NEW.parent_id;
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                            THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                            ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = (SELECT p.parent_id FROM items AS p WHERE p.id = NEW.parent_id);
    UPDATE items SET
        available = CASE WHEN kind IN ('series', 'season', 'podcast')
            THEN EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                         AND c.kind IN ('season', 'episode', 'podcast_episode'))
            ELSE available END,
        min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                            THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                            ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
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
        min_child_rating = (SELECT min(CASE WHEN c.kind IN ('series', 'season', 'podcast')
                                            THEN max(COALESCE(c.effective_rating, 18), COALESCE(c.min_child_rating, 18))
                                            ELSE COALESCE(c.effective_rating, 18) END) FROM items AS c
                            WHERE c.parent_id = items.id AND c.deleted_at IS NULL AND c.available = 1
                              AND c.kind IN ('season', 'episode', 'podcast_episode'))
    WHERE id = OLD.parent_id;
END;
CREATE INDEX devices_account ON devices (account_id);
CREATE INDEX playback_sessions_profile_activity ON playback_sessions (profile_id, revoked_at, last_activity_at);
CREATE INDEX playback_sessions_device ON playback_sessions (device_id);
CREATE INDEX progress_events_profile_time ON progress_events (profile_id, server_time);
CREATE INDEX progress_events_playback ON progress_events (playback_session_id, seq);
CREATE INDEX download_tombstones_device ON download_tombstones (device_id);
CREATE INDEX download_rules_device ON download_rules (device_id);
CREATE UNIQUE INDEX items_episode_guid ON items (parent_id, guid) WHERE guid IS NOT NULL;
CREATE INDEX items_feed_due ON items (kind, feed_checked_at) WHERE kind = 'podcast' AND deleted_at IS NULL;
CREATE UNIQUE INDEX items_podcast_feed ON items (library_id, feed_url) WHERE kind = 'podcast' AND deleted_at IS NULL;
CREATE INDEX items_published ON items (parent_id, published_at DESC) WHERE kind = 'podcast_episode';
CREATE TRIGGER media_files_insert AFTER INSERT ON media_files
BEGIN
    UPDATE items SET available = CASE
            WHEN kind = 'podcast_episode' AND enclosure_url IS NOT NULL THEN 1
            ELSE EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since IS NULL) END
    WHERE id = NEW.item_id AND kind IN ('movie', 'episode', 'extra', 'audiobook', 'podcast_episode');
END;
CREATE TRIGGER media_files_update AFTER UPDATE OF missing_since, item_id ON media_files
BEGIN
    UPDATE items SET available = CASE
            WHEN kind = 'podcast_episode' AND enclosure_url IS NOT NULL THEN 1
            ELSE EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since IS NULL) END
    WHERE id IN (NEW.item_id, OLD.item_id) AND kind IN ('movie', 'episode', 'extra', 'audiobook', 'podcast_episode');
END;
CREATE TRIGGER media_files_delete AFTER DELETE ON media_files
BEGIN
    UPDATE items SET available = CASE
            WHEN kind = 'podcast_episode' AND enclosure_url IS NOT NULL THEN 1
            ELSE EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = items.id AND m.missing_since IS NULL) END
    WHERE id = OLD.item_id AND kind IN ('movie', 'episode', 'extra', 'audiobook', 'podcast_episode');
END;
CREATE INDEX chapters_media_file ON chapters (media_file_id);
CREATE INDEX downloads_account ON downloads (account_id);
CREATE INDEX downloads_state ON downloads (state, updated_at);
CREATE INDEX downloads_item ON downloads (item_id);
CREATE INDEX downloads_profile_device ON downloads (profile_id, device_id);
CREATE TRIGGER downloads_tombstone AFTER DELETE ON downloads
BEGIN
    INSERT OR REPLACE INTO download_tombstones (id, device_id, removed_at)
    VALUES (OLD.id, OLD.device_id, CAST((julianday('now') - 2440587.5) * 86400000 AS INTEGER));
END;
CREATE INDEX watchlist_item ON watchlist (item_id);
CREATE INDEX favorites_item ON favorites (item_id);
CREATE INDEX collection_items_item ON collection_items (item_id);
CREATE INDEX top_picks_profile ON top_picks (profile_id);
CREATE INDEX item_genres_name ON item_genres (lang, name);
CREATE TRIGGER items_genres_insert AFTER INSERT ON items
BEGIN
    INSERT OR IGNORE INTO item_genres (item_id, lang, name)
    SELECT NEW.id, l.lang, g.value
    FROM (SELECT 'de' AS lang UNION ALL SELECT 'en') AS l
    JOIN json_each(CASE WHEN json_type(NEW.genres) = 'object' THEN json_extract(NEW.genres, '$.' || l.lang)
                        ELSE NEW.genres END) AS g
    WHERE json_valid(NEW.genres) AND g.type = 'text' AND g.value <> '';
END;
CREATE TRIGGER items_genres_update AFTER UPDATE OF genres ON items
BEGIN
    DELETE FROM item_genres WHERE item_id = NEW.id;
    INSERT OR IGNORE INTO item_genres (item_id, lang, name)
    SELECT NEW.id, l.lang, g.value
    FROM (SELECT 'de' AS lang UNION ALL SELECT 'en') AS l
    JOIN json_each(CASE WHEN json_type(NEW.genres) = 'object' THEN json_extract(NEW.genres, '$.' || l.lang)
                        ELSE NEW.genres END) AS g
    WHERE json_valid(NEW.genres) AND g.type = 'text' AND g.value <> '';
END;
CREATE INDEX search_docs_item ON search_docs (item_id);
CREATE TRIGGER search_docs_insert AFTER INSERT ON search_docs
BEGIN
    INSERT INTO search_index (rowid, text, kind, item_id, ref_id)
    VALUES (NEW.id, NEW.text, NEW.kind, NEW.item_id, NEW.ref_id);
END;
CREATE TRIGGER search_docs_delete AFTER DELETE ON search_docs
BEGIN
    INSERT INTO search_index (search_index, rowid, text, kind, item_id, ref_id)
    VALUES ('delete', OLD.id, OLD.text, OLD.kind, OLD.item_id, OLD.ref_id);
END;
CREATE TRIGGER search_docs_update AFTER UPDATE OF text ON search_docs
WHEN NEW.text IS NOT OLD.text
BEGIN
    INSERT INTO search_index (search_index, rowid, text, kind, item_id, ref_id)
    VALUES ('delete', OLD.id, OLD.text, OLD.kind, OLD.item_id, OLD.ref_id);
    INSERT INTO search_index (rowid, text, kind, item_id, ref_id)
    VALUES (NEW.id, NEW.text, NEW.kind, NEW.item_id, NEW.ref_id);
END;
CREATE TRIGGER search_items_insert AFTER INSERT ON items
BEGIN
    INSERT INTO search_docs (kind, item_id, ref_id, text)
    VALUES ('item', NEW.id, NEW.id,
            trim(coalesce(NEW.title, '') || ' / ' || coalesce(NEW.original_title, '') || ' / ' || coalesce(NEW.author, '')
                 || ' / ' || coalesce(NEW.narrator, '') || ' / ' || coalesce(NEW.series_name, '')))
    ON CONFLICT (kind, ref_id, item_id) DO UPDATE SET text = excluded.text;
END;
CREATE TRIGGER search_items_update AFTER UPDATE OF title, original_title, author, narrator, series_name ON items
WHEN NEW.title IS NOT OLD.title OR NEW.original_title IS NOT OLD.original_title OR NEW.author IS NOT OLD.author
  OR NEW.narrator IS NOT OLD.narrator OR NEW.series_name IS NOT OLD.series_name
BEGIN
    INSERT INTO search_docs (kind, item_id, ref_id, text)
    VALUES ('item', NEW.id, NEW.id,
            trim(coalesce(NEW.title, '') || ' / ' || coalesce(NEW.original_title, '') || ' / ' || coalesce(NEW.author, '')
                 || ' / ' || coalesce(NEW.narrator, '') || ' / ' || coalesce(NEW.series_name, '')))
    ON CONFLICT (kind, ref_id, item_id) DO UPDATE SET text = excluded.text;
END;
CREATE TRIGGER search_items_delete AFTER DELETE ON items
BEGIN
    DELETE FROM search_docs WHERE item_id = OLD.id;
END;
CREATE TRIGGER search_texts_insert AFTER INSERT ON item_texts
BEGIN
    INSERT INTO search_docs (kind, item_id, ref_id, text)
    VALUES ('text', NEW.item_id, NEW.item_id || ':' || NEW.lang, NEW.title)
    ON CONFLICT (kind, ref_id, item_id) DO UPDATE SET text = excluded.text;
END;
CREATE TRIGGER search_texts_update AFTER UPDATE OF title ON item_texts
WHEN NEW.title IS NOT OLD.title
BEGIN
    INSERT INTO search_docs (kind, item_id, ref_id, text)
    VALUES ('text', NEW.item_id, NEW.item_id || ':' || NEW.lang, NEW.title)
    ON CONFLICT (kind, ref_id, item_id) DO UPDATE SET text = excluded.text;
END;
CREATE TRIGGER search_texts_delete AFTER DELETE ON item_texts
BEGIN
    DELETE FROM search_docs WHERE kind = 'text' AND ref_id = OLD.item_id || ':' || OLD.lang AND item_id = OLD.item_id;
END;
CREATE TRIGGER search_credits_insert AFTER INSERT ON credits
BEGIN
    INSERT INTO search_docs (kind, item_id, ref_id, text)
    SELECT 'person', NEW.item_id, CAST(NEW.person_id AS TEXT), p.name FROM people AS p WHERE p.id = NEW.person_id
    ON CONFLICT (kind, ref_id, item_id) DO NOTHING;
END;
CREATE TRIGGER search_credits_delete AFTER DELETE ON credits
WHEN NOT EXISTS (SELECT 1 FROM credits AS c WHERE c.item_id = OLD.item_id AND c.person_id = OLD.person_id)
BEGIN
    DELETE FROM search_docs WHERE kind = 'person' AND ref_id = CAST(OLD.person_id AS TEXT) AND item_id = OLD.item_id;
END;
CREATE TRIGGER search_people_update AFTER UPDATE OF name ON people
WHEN NEW.name IS NOT OLD.name
BEGIN
    UPDATE search_docs SET text = NEW.name WHERE kind = 'person' AND ref_id = CAST(NEW.id AS TEXT);
END;
CREATE TRIGGER search_collections_update AFTER UPDATE OF name, name_de ON collections
WHEN NEW.name IS NOT OLD.name OR NEW.name_de IS NOT OLD.name_de
BEGIN
    UPDATE search_docs SET text = trim(NEW.name || ' / ' || coalesce(NEW.name_de, ''))
    WHERE kind = 'collection' AND ref_id = NEW.id;
END;
CREATE TRIGGER search_collection_items_insert AFTER INSERT ON collection_items
BEGIN
    INSERT INTO search_docs (kind, item_id, ref_id, text)
    SELECT 'collection', NEW.item_id, NEW.collection_id, trim(c.name || ' / ' || coalesce(c.name_de, ''))
    FROM collections AS c WHERE c.id = NEW.collection_id
    ON CONFLICT (kind, ref_id, item_id) DO UPDATE SET text = excluded.text;
END;
CREATE TRIGGER search_collection_items_delete AFTER DELETE ON collection_items
BEGIN
    DELETE FROM search_docs WHERE kind = 'collection' AND ref_id = OLD.collection_id AND item_id = OLD.item_id;
END;
CREATE INDEX notifications_inbox ON notifications (profile_id, created_at DESC, id DESC);
CREATE INDEX notifications_unread ON notifications (profile_id, read_at);
CREATE INDEX notifications_item ON notifications (item_id);
CREATE INDEX push_subscriptions_account ON push_subscriptions (account_id);
CREATE INDEX push_subscriptions_device ON push_subscriptions (device_id);
CREATE INDEX announcements_published ON announcements (published_at DESC) WHERE published_at IS NOT NULL;
CREATE INDEX announcement_acks_profile ON announcement_acks (profile_id);
CREATE INDEX webhook_events_integration ON webhook_events (integration_id, id DESC);
CREATE TRIGGER webhook_events_trim AFTER INSERT ON webhook_events
BEGIN
    DELETE FROM webhook_events
    WHERE integration_id = NEW.integration_id
      AND id <= (SELECT id FROM webhook_events
                 WHERE integration_id = NEW.integration_id
                 ORDER BY id DESC LIMIT 1 OFFSET 20);
END;
CREATE INDEX deliveries_state ON deliveries (state, created_at);
CREATE INDEX deliveries_account ON deliveries (account_id, created_at DESC);
CREATE INDEX deliveries_dedupe ON deliveries (event, source, created_at) WHERE source IS NOT NULL;
CREATE INDEX progress_days_day ON progress_days (day, played_ms, plays);
CREATE INDEX progress_events_server_time ON progress_events (server_time);
CREATE INDEX media_files_root_item ON media_files (root_id, item_id);
CREATE INDEX spotlight_entries_tmdb ON spotlight_entries (media, tmdb_id);
CREATE UNIQUE INDEX spotlight_overrides_rank ON spotlight_overrides (list_key, pinned_rank)
WHERE pinned_rank IS NOT NULL;
CREATE INDEX items_released ON items (release_date DESC)
WHERE release_date IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX accounts_owner ON accounts (is_owner) WHERE is_owner = 1;
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
                  WHERE al.account_id = a.id AND al.library_id = i.library_id));
