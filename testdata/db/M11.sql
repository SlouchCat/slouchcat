CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	);
INSERT INTO "goose_db_version" VALUES (1, 0, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (2, 1, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (3, 2, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (4, 3, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (5, 4, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (6, 5, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (7, 6, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (8, 7, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (9, 8, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (10, 9, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (11, 10, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (12, 11, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (13, 12, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (14, 13, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (15, 14, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (16, 15, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (17, 16, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (18, 17, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (19, 18, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (20, 19, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (21, 20, 1, '2026-10-27 10:36:14 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (22, 21, 1, '2026-10-27 10:36:14 +0000 UTC');
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
INSERT INTO "jobs" VALUES (1, 'library.sweep', 2, 'sweep', '{}', 'done', 1793097375798, NULL, NULL, 1, 5, NULL, 1793097375798, 1793097375802);
INSERT INTO "jobs" VALUES (2, 'trakt.refresh', 2, 'trakt', '{}', 'done', 1793097375803, NULL, NULL, 1, 5, NULL, 1793097375803, 1793097376808);
INSERT INTO "jobs" VALUES (3, 'scan.library', 5, 'scan:1', '{"libraryId":1}', 'done', 1793097375881, NULL, NULL, 1, 5, NULL, 1793097375881, 1793097376825);
INSERT INTO "jobs" VALUES (4, 'metadata.match', 4, 'match:01M5YEX91B69XH2ADY2M9EZH4D', '{"itemId":"01M5YEX91B69XH2ADY2M9EZH4D"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376825);
INSERT INTO "jobs" VALUES (5, 'probe.file', 3, 'probe:01M5YEX91CHH6S0ZPJC7XDWD7S', '{"mediaFileId":"01M5YEX91CHH6S0ZPJC7XDWD7S"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376859);
INSERT INTO "jobs" VALUES (6, 'metadata.match', 4, 'match:01M5YEX91CHH6S0ZPJC8XRZVSK', '{"itemId":"01M5YEX91CHH6S0ZPJC8XRZVSK"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376825);
INSERT INTO "jobs" VALUES (7, 'probe.file', 3, 'probe:01M5YEX91CHH6S0ZPJCBTRBP46', '{"mediaFileId":"01M5YEX91CHH6S0ZPJCBTRBP46"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376881);
INSERT INTO "jobs" VALUES (8, 'metadata.match', 4, 'match:01M5YEX91DPT6TRTWSPCJ6EH40', '{"itemId":"01M5YEX91DPT6TRTWSPCJ6EH40"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376826);
INSERT INTO "jobs" VALUES (9, 'probe.file', 3, 'probe:01M5YEX91DPT6TRTWSPENHA7QW', '{"mediaFileId":"01M5YEX91DPT6TRTWSPENHA7QW"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376913);
INSERT INTO "jobs" VALUES (10, 'metadata.match', 4, 'match:01M5YEX91DPT6TRTWSPG4EZQ3M', '{"itemId":"01M5YEX91DPT6TRTWSPG4EZQ3M"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376826);
INSERT INTO "jobs" VALUES (11, 'probe.file', 3, 'probe:01M5YEX91DPT6TRTWSPJM0J9A6', '{"mediaFileId":"01M5YEX91DPT6TRTWSPJM0J9A6"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376945);
INSERT INTO "jobs" VALUES (12, 'metadata.match', 4, 'match:01M5YEX91EAXEXBPAN6A9PT86N', '{"itemId":"01M5YEX91EAXEXBPAN6A9PT86N"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376826);
INSERT INTO "jobs" VALUES (13, 'probe.file', 3, 'probe:01M5YEX91EAXEXBPAN6AYPWFEC', '{"mediaFileId":"01M5YEX91EAXEXBPAN6AYPWFEC"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376987);
INSERT INTO "jobs" VALUES (14, 'metadata.match', 4, 'match:01M5YEX91EAXEXBPAN6DD2HDHE', '{"itemId":"01M5YEX91EAXEXBPAN6DD2HDHE"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376826);
INSERT INTO "jobs" VALUES (15, 'probe.file', 3, 'probe:01M5YEX91F4G0S1DRR5M1RKKTN', '{"mediaFileId":"01M5YEX91F4G0S1DRR5M1RKKTN"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377019);
INSERT INTO "jobs" VALUES (16, 'probe.file', 3, 'probe:01M5YEX91F4G0S1DRR5MT06D47', '{"mediaFileId":"01M5YEX91F4G0S1DRR5MT06D47"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377052);
INSERT INTO "jobs" VALUES (17, 'metadata.match', 4, 'match:01M5YEX91F4G0S1DRR5MXV6SB8', '{"itemId":"01M5YEX91F4G0S1DRR5MXV6SB8"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376826);
INSERT INTO "jobs" VALUES (18, 'probe.file', 3, 'probe:01M5YEX91F4G0S1DRR5NFDV78P', '{"mediaFileId":"01M5YEX91F4G0S1DRR5NFDV78P"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377082);
INSERT INTO "jobs" VALUES (19, 'metadata.match', 4, 'match:01M5YEX91GVAVYSAWPTH60QGAF', '{"itemId":"01M5YEX91GVAVYSAWPTH60QGAF"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376827);
INSERT INTO "jobs" VALUES (20, 'probe.file', 3, 'probe:01M5YEX91GVAVYSAWPTMEX67X7', '{"mediaFileId":"01M5YEX91GVAVYSAWPTMEX67X7"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377111);
INSERT INTO "jobs" VALUES (21, 'metadata.match', 4, 'match:01M5YEX91GVAVYSAWPTPEM5ABR', '{"itemId":"01M5YEX91GVAVYSAWPTPEM5ABR"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376827);
INSERT INTO "jobs" VALUES (22, 'probe.file', 3, 'probe:01M5YEX91GVAVYSAWPTQF8Y7FT', '{"mediaFileId":"01M5YEX91GVAVYSAWPTQF8Y7FT"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377183);
INSERT INTO "jobs" VALUES (23, 'metadata.match', 4, 'match:01M5YEX91H1QAJQB3A1RS977JA', '{"itemId":"01M5YEX91H1QAJQB3A1RS977JA"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376827);
INSERT INTO "jobs" VALUES (24, 'probe.file', 3, 'probe:01M5YEX91H1QAJQB3A1VNJN4NA', '{"mediaFileId":"01M5YEX91H1QAJQB3A1VNJN4NA"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377264);
INSERT INTO "jobs" VALUES (25, 'metadata.match', 4, 'match:01M5YEX91H1QAJQB3A1Y0GQFQQ', '{"itemId":"01M5YEX91H1QAJQB3A1Y0GQFQQ"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376827);
INSERT INTO "jobs" VALUES (26, 'probe.file', 3, 'probe:01M5YEX91J8VC3Z6MWQE1Q6Z14', '{"mediaFileId":"01M5YEX91J8VC3Z6MWQE1Q6Z14"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377307);
INSERT INTO "jobs" VALUES (27, 'metadata.match', 4, 'match:01M5YEX91J8VC3Z6MWQH2AVYF1', '{"itemId":"01M5YEX91J8VC3Z6MWQH2AVYF1"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376827);
INSERT INTO "jobs" VALUES (28, 'probe.file', 3, 'probe:01M5YEX91J8VC3Z6MWQK4Q6HB6', '{"mediaFileId":"01M5YEX91J8VC3Z6MWQK4Q6HB6"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377338);
INSERT INTO "jobs" VALUES (29, 'metadata.match', 4, 'match:01M5YEX91J8VC3Z6MWQNPPJ2TH', '{"itemId":"01M5YEX91J8VC3Z6MWQNPPJ2TH"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376828);
INSERT INTO "jobs" VALUES (30, 'probe.file', 3, 'probe:01M5YEX91KGWA5MGH2BP2FXA8X', '{"mediaFileId":"01M5YEX91KGWA5MGH2BP2FXA8X"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377369);
INSERT INTO "jobs" VALUES (31, 'metadata.match', 4, 'match:01M5YEX91KGWA5MGH2BQ4RCZEB', '{"itemId":"01M5YEX91KGWA5MGH2BQ4RCZEB"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376828);
INSERT INTO "jobs" VALUES (32, 'probe.file', 3, 'probe:01M5YEX91KGWA5MGH2BT2Y01Y0', '{"mediaFileId":"01M5YEX91KGWA5MGH2BT2Y01Y0"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377398);
INSERT INTO "jobs" VALUES (33, 'metadata.match', 4, 'match:01M5YEX91MFMVPSQ7F55GM5XMX', '{"itemId":"01M5YEX91MFMVPSQ7F55GM5XMX"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376828);
INSERT INTO "jobs" VALUES (34, 'probe.file', 3, 'probe:01M5YEX91MFMVPSQ7F56381DN5', '{"mediaFileId":"01M5YEX91MFMVPSQ7F56381DN5"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377426);
INSERT INTO "jobs" VALUES (35, 'metadata.match', 4, 'match:01M5YEX91MFMVPSQ7F57FJQ7VS', '{"itemId":"01M5YEX91MFMVPSQ7F57FJQ7VS"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376828);
INSERT INTO "jobs" VALUES (36, 'probe.file', 3, 'probe:01M5YEX91MFMVPSQ7F5B5SJ4W1', '{"mediaFileId":"01M5YEX91MFMVPSQ7F5B5SJ4W1"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377454);
INSERT INTO "jobs" VALUES (37, 'metadata.match', 4, 'match:01M5YEX91NDKSTWFK6MG3MR0MQ', '{"itemId":"01M5YEX91NDKSTWFK6MG3MR0MQ"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097376828);
INSERT INTO "jobs" VALUES (38, 'probe.file', 3, 'probe:01M5YEX91NDKSTWFK6MJ7S9F72', '{"mediaFileId":"01M5YEX91NDKSTWFK6MJ7S9F72"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377492);
INSERT INTO "jobs" VALUES (39, 'probe.file', 3, 'probe:01M5YEX91PSNK4HSR97N67TZFG', '{"mediaFileId":"01M5YEX91PSNK4HSR97N67TZFG"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377524);
INSERT INTO "jobs" VALUES (40, 'probe.file', 3, 'probe:01M5YEX91PSNK4HSR97S4PZZ9W', '{"mediaFileId":"01M5YEX91PSNK4HSR97S4PZZ9W"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377562);
INSERT INTO "jobs" VALUES (41, 'probe.file', 3, 'probe:01M5YEX91QTS88HPAW1FH51AJR', '{"mediaFileId":"01M5YEX91QTS88HPAW1FH51AJR"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377604);
INSERT INTO "jobs" VALUES (42, 'probe.file', 3, 'probe:01M5YEX91QTS88HPAW1JVPWQ8R', '{"mediaFileId":"01M5YEX91QTS88HPAW1JVPWQ8R"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377637);
INSERT INTO "jobs" VALUES (43, 'probe.file', 3, 'probe:01M5YEX91RP8724TWHMZZTQPYQ', '{"mediaFileId":"01M5YEX91RP8724TWHMZZTQPYQ"}', 'done', 1793097376811, NULL, NULL, 1, 5, NULL, 1793097376811, 1793097377668);
INSERT INTO "jobs" VALUES (44, 'probe.keyframes', 1, 'keyframes:01M5YEX91CHH6S0ZPJC7XDWD7S', '{"mediaFileId":"01M5YEX91CHH6S0ZPJC7XDWD7S"}', 'done', 1793097376859, NULL, NULL, 1, 5, NULL, 1793097376859, 1793097377700);
INSERT INTO "jobs" VALUES (45, 'probe.keyframes', 1, 'keyframes:01M5YEX91DPT6TRTWSPENHA7QW', '{"mediaFileId":"01M5YEX91DPT6TRTWSPENHA7QW"}', 'done', 1793097376912, NULL, NULL, 1, 5, NULL, 1793097376912, 1793097377730);
INSERT INTO "jobs" VALUES (46, 'probe.keyframes', 1, 'keyframes:01M5YEX91DPT6TRTWSPJM0J9A6', '{"mediaFileId":"01M5YEX91DPT6TRTWSPJM0J9A6"}', 'done', 1793097376944, NULL, NULL, 1, 5, NULL, 1793097376944, 1793097377771);
INSERT INTO "jobs" VALUES (47, 'probe.keyframes', 1, 'keyframes:01M5YEX91EAXEXBPAN6AYPWFEC', '{"mediaFileId":"01M5YEX91EAXEXBPAN6AYPWFEC"}', 'done', 1793097376987, NULL, NULL, 1, 5, NULL, 1793097376987, 1793097377802);
INSERT INTO "jobs" VALUES (48, 'probe.keyframes', 1, 'keyframes:01M5YEX91F4G0S1DRR5M1RKKTN', '{"mediaFileId":"01M5YEX91F4G0S1DRR5M1RKKTN"}', 'done', 1793097377019, NULL, NULL, 1, 5, NULL, 1793097377019, 1793097377861);
INSERT INTO "jobs" VALUES (49, 'probe.keyframes', 1, 'keyframes:01M5YEX91F4G0S1DRR5MT06D47', '{"mediaFileId":"01M5YEX91F4G0S1DRR5MT06D47"}', 'done', 1793097377051, NULL, NULL, 1, 5, NULL, 1793097377051, 1793097377862);
INSERT INTO "jobs" VALUES (50, 'probe.keyframes', 1, 'keyframes:01M5YEX91F4G0S1DRR5NFDV78P', '{"mediaFileId":"01M5YEX91F4G0S1DRR5NFDV78P"}', 'done', 1793097377082, NULL, NULL, 1, 5, NULL, 1793097377082, 1793097377856);
INSERT INTO "jobs" VALUES (51, 'probe.keyframes', 1, 'keyframes:01M5YEX91GVAVYSAWPTMEX67X7', '{"mediaFileId":"01M5YEX91GVAVYSAWPTMEX67X7"}', 'done', 1793097377111, NULL, NULL, 1, 5, NULL, 1793097377111, 1793097377861);
INSERT INTO "jobs" VALUES (52, 'probe.keyframes', 1, 'keyframes:01M5YEX91GVAVYSAWPTQF8Y7FT', '{"mediaFileId":"01M5YEX91GVAVYSAWPTQF8Y7FT"}', 'done', 1793097377182, NULL, NULL, 1, 5, NULL, 1793097377182, 1793097377912);
INSERT INTO "jobs" VALUES (53, 'probe.keyframes', 1, 'keyframes:01M5YEX91H1QAJQB3A1VNJN4NA', '{"mediaFileId":"01M5YEX91H1QAJQB3A1VNJN4NA"}', 'done', 1793097377263, NULL, NULL, 1, 5, NULL, 1793097377263, 1793097377911);
INSERT INTO "jobs" VALUES (54, 'probe.keyframes', 1, 'keyframes:01M5YEX91J8VC3Z6MWQE1Q6Z14', '{"mediaFileId":"01M5YEX91J8VC3Z6MWQE1Q6Z14"}', 'done', 1793097377306, NULL, NULL, 1, 5, NULL, 1793097377306, 1793097377906);
INSERT INTO "jobs" VALUES (55, 'probe.keyframes', 1, 'keyframes:01M5YEX91J8VC3Z6MWQK4Q6HB6', '{"mediaFileId":"01M5YEX91J8VC3Z6MWQK4Q6HB6"}', 'done', 1793097377338, NULL, NULL, 1, 5, NULL, 1793097377338, 1793097377896);
INSERT INTO "jobs" VALUES (56, 'probe.keyframes', 1, 'keyframes:01M5YEX91KGWA5MGH2BP2FXA8X', '{"mediaFileId":"01M5YEX91KGWA5MGH2BP2FXA8X"}', 'done', 1793097377369, NULL, NULL, 1, 5, NULL, 1793097377369, 1793097377930);
INSERT INTO "jobs" VALUES (57, 'probe.keyframes', 1, 'keyframes:01M5YEX91KGWA5MGH2BT2Y01Y0', '{"mediaFileId":"01M5YEX91KGWA5MGH2BT2Y01Y0"}', 'done', 1793097377397, NULL, NULL, 1, 5, NULL, 1793097377397, 1793097377937);
INSERT INTO "jobs" VALUES (58, 'probe.keyframes', 1, 'keyframes:01M5YEX91MFMVPSQ7F56381DN5', '{"mediaFileId":"01M5YEX91MFMVPSQ7F56381DN5"}', 'done', 1793097377425, NULL, NULL, 1, 5, NULL, 1793097377425, 1793097377943);
INSERT INTO "jobs" VALUES (59, 'probe.keyframes', 1, 'keyframes:01M5YEX91MFMVPSQ7F5B5SJ4W1', '{"mediaFileId":"01M5YEX91MFMVPSQ7F5B5SJ4W1"}', 'done', 1793097377454, NULL, NULL, 1, 5, NULL, 1793097377454, 1793097377950);
INSERT INTO "jobs" VALUES (60, 'probe.keyframes', 1, 'keyframes:01M5YEX91NDKSTWFK6MJ7S9F72', '{"mediaFileId":"01M5YEX91NDKSTWFK6MJ7S9F72"}', 'done', 1793097377491, NULL, NULL, 1, 5, NULL, 1793097377491, 1793097377970);
INSERT INTO "jobs" VALUES (61, 'probe.keyframes', 1, 'keyframes:01M5YEX91PSNK4HSR97N67TZFG', '{"mediaFileId":"01M5YEX91PSNK4HSR97N67TZFG"}', 'done', 1793097377524, NULL, NULL, 1, 5, NULL, 1793097377524, 1793097377968);
INSERT INTO "jobs" VALUES (62, 'probe.keyframes', 1, 'keyframes:01M5YEX91PSNK4HSR97S4PZZ9W', '{"mediaFileId":"01M5YEX91PSNK4HSR97S4PZZ9W"}', 'done', 1793097377562, NULL, NULL, 1, 5, NULL, 1793097377562, 1793097377978);
INSERT INTO "jobs" VALUES (63, 'probe.keyframes', 1, 'keyframes:01M5YEX91QTS88HPAW1FH51AJR', '{"mediaFileId":"01M5YEX91QTS88HPAW1FH51AJR"}', 'done', 1793097377604, NULL, NULL, 1, 5, NULL, 1793097377604, 1793097377978);
INSERT INTO "jobs" VALUES (64, 'probe.keyframes', 1, 'keyframes:01M5YEX91QTS88HPAW1JVPWQ8R', '{"mediaFileId":"01M5YEX91QTS88HPAW1JVPWQ8R"}', 'done', 1793097377637, NULL, NULL, 1, 5, NULL, 1793097377637, 1793097377999);
INSERT INTO "jobs" VALUES (65, 'probe.keyframes', 1, 'keyframes:01M5YEX91RP8724TWHMZZTQPYQ', '{"mediaFileId":"01M5YEX91RP8724TWHMZZTQPYQ"}', 'done', 1793097377668, NULL, NULL, 1, 5, NULL, 1793097377668, 1793097377999);
INSERT INTO "jobs" VALUES (66, 'trakt.refresh', 7, 'trakt', '{}', 'queued', 1793097406924, NULL, NULL, 0, 5, NULL, 1793097406924, 1793097406924);
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
INSERT INTO "settings" VALUES ('trakt_last_attempt', '1793097376803', 1793097376803);
INSERT INTO "settings" VALUES ('trakt_last_success', '1793097376803', 1793097376803);
CREATE TABLE accounts (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL COLLATE NOCASE UNIQUE,
    password_hash TEXT    NOT NULL,            -- argon2id PHC string
    role          TEXT    NOT NULL DEFAULT 'owner' CHECK (role IN ('owner', 'guest')),
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
, max_rating INTEGER NOT NULL DEFAULT 18 CHECK (max_rating IN (0, 6, 12, 16, 18)), all_libraries INTEGER NOT NULL DEFAULT 1 CHECK (all_libraries IN (0, 1)), invited_by INTEGER REFERENCES accounts(id) ON DELETE SET NULL, last_seen_at INTEGER, max_bitrate_kbps INTEGER CHECK (max_bitrate_kbps IS NULL OR max_bitrate_kbps > 0), max_height INTEGER CHECK (max_height IS NULL OR max_height IN (360, 480, 720, 1080, 2160)), max_streams INTEGER CHECK (max_streams IS NULL OR max_streams BETWEEN 1 AND 16)) STRICT;
INSERT INTO "accounts" VALUES (1, 'frozen@example.test', '$argon2id$v=19$m=65536,t=3,p=4$4ylgI2OMigvuvBS9n1deQg$6NOjU3jgFWnZzF+g1P3J9krXUok5CdAHqkBOMcgPves', 'owner', 1793097375879, 1793097375879, 18, 1, NULL, NULL, NULL, NULL, NULL);
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
INSERT INTO "profiles" VALUES (1, 1, 'frozen', NULL, 18, 'de', NULL, 1, 1, 1793097375879, 1793097375879, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
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
INSERT INTO "library_roots" VALUES (1, 1, '/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/.claude/worktrees/vigorous-lumiere-17beb0/testdata/media/Movies', 'ext4', 'inotify', 1793097375880);
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
INSERT INTO "items" VALUES ('01M5YEX91B69XH2ADY2M9EZH4D', 'movie', NULL, 1, 'Slouchcat Test AC3 51', 'slouchcat test ac3 51', NULL, 2024, NULL, NULL, '[]', 7001, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376825, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91CHH6S0ZPJC8XRZVSK', 'movie', NULL, 1, 'Slouchcat Test Broken', 'slouchcat test broken', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376825, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91DPT6TRTWSPCJ6EH40', 'movie', NULL, 1, 'Slouchcat Test Chapters', 'slouchcat test chapters', NULL, 2024, NULL, NULL, '[]', 7004, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376826, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91DPT6TRTWSPG4EZQ3M', 'movie', NULL, 1, 'Slouchcat Test DTS', 'slouchcat test dts', NULL, 2024, NULL, NULL, '[]', 7002, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376826, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91EAXEXBPAN6A9PT86N', 'movie', NULL, 1, 'Slouchcat Test Extras', 'slouchcat test extras', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376826, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91EAXEXBPAN6DD2HDHE', 'movie', NULL, 1, 'Slouchcat Test H264', 'slouchcat test h264', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376826, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91F4G0S1DRR5MXV6SB8', 'movie', NULL, 1, 'Slouchcat Test H264 Copy', 'slouchcat test h264 copy', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376826, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91GVAVYSAWPTH60QGAF', 'movie', NULL, 1, 'Slouchcat Test H264 MKV Long', 'slouchcat test h264 mkv long', NULL, 2024, NULL, NULL, '[]', 7005, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376827, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91GVAVYSAWPTPEM5ABR', 'movie', NULL, 1, 'Slouchcat Test HDR10 4K', 'slouchcat test hdr10 4k', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376827, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91H1QAJQB3A1RS977JA', 'movie', NULL, 1, 'Slouchcat Test HDR10 Chart', 'slouchcat test hdr10 chart', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376827, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91H1QAJQB3A1Y0GQFQQ', 'movie', NULL, 1, 'Slouchcat Test HEVC 10bit', 'slouchcat test hevc 10bit', NULL, 2024, NULL, NULL, '[]', 7003, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376827, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91J8VC3Z6MWQH2AVYF1', 'movie', NULL, 1, 'Slouchcat Test HEVC Long', 'slouchcat test hevc long', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376827, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91J8VC3Z6MWQNPPJ2TH', 'movie', NULL, 1, 'Slouchcat Test Multi Audio', 'slouchcat test multi audio', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376828, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91KGWA5MGH2BQ4RCZEB', 'movie', NULL, 1, 'Slouchcat Test PGS', 'slouchcat test pgs', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376828, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91MFMVPSQ7F55GM5XMX', 'movie', NULL, 1, 'Slouchcat Test Sidecar Subtitles', 'slouchcat test sidecar subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376828, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91MFMVPSQ7F57FJQ7VS', 'movie', NULL, 1, 'Slouchcat Test Subtitles', 'slouchcat test subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376828, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91NDKSTWFK6MG3MR0MQ', 'movie', NULL, 1, 'Slouchcat Test Unrated', 'slouchcat test unrated', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'skipped', 'no_tmdb_key', NULL, 1793097376811, 1793097376811, 1793097376828, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91NDKSTWFK6MNKBPF3Z', 'extra', '01M5YEX91EAXEXBPAN6A9PT86N', 1, 'On Set', 'on set', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793097376811, 1793097376811, 1793097376811, NULL, NULL, NULL, NULL, NULL, 'behind_the_scenes', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91PSNK4HSR97Q75GJ7J', 'extra', '01M5YEX91EAXEXBPAN6A9PT86N', 1, 'Scene 1', 'scene 1', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793097376811, 1793097376811, 1793097376811, NULL, NULL, NULL, NULL, NULL, 'deleted_scene', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91PSNK4HSR97V3XYNA0', 'extra', '01M5YEX91EAXEXBPAN6A9PT86N', 1, 'Making Of', 'making of', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793097376811, 1793097376811, 1793097376811, NULL, NULL, NULL, NULL, NULL, 'featurette', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91QTS88HPAW1J9T39Y9', 'extra', '01M5YEX91EAXEXBPAN6A9PT86N', 1, 'Slouchcat Test Extras (2024)', 'slouchcat test extras (2024)', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793097376811, 1793097376811, 1793097376811, NULL, NULL, NULL, NULL, NULL, 'trailer', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M5YEX91RP8724TWHMZGPPE0X', 'extra', '01M5YEX91EAXEXBPAN6A9PT86N', 1, 'Slouchcat Test Extras Trailer', 'slouchcat test extras trailer', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793097376811, 1793097376811, 1793097376811, NULL, NULL, NULL, NULL, NULL, 'trailer', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
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
INSERT INTO "media_files" VALUES ('01M5YEX91CHH6S0ZPJC7XDWD7S', '01M5YEX91B69XH2ADY2M9EZH4D', 1, 'Slouchcat Test AC3 51 (2024) {tmdb-7001}/Slouchcat Test AC3 51 (2024) {tmdb-7001}.mkv', 66310, 3410180, 2943036, 1793097293005, 'b980f487c2e54963eb88bc14531115159926906570566b75cbc39edcca2b2315', NULL, 'mkv', 'h264', 'ac3', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":2943036,"bitRate":2354428,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test AC3 51"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"ac3","channels":6,"channelLayout":"5.1(side)","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793097376859, 1793097377700, NULL, 1793097376811, 1793097377700, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91CHH6S0ZPJCBTRBP46', '01M5YEX91CHH6S0ZPJC8XRZVSK', 1, 'Slouchcat Test Broken (2024)/Slouchcat Test Broken (2024).mkv', 66310, 3410218, 57, 1793097304553, '8ed0f89e2cd4d6f84fa244afd8b1a2d46d26aaf54bf6537fc72508bff522218f', NULL, 'mkv', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793097376881, NULL, NULL, 1793097376811, 1793097376881, 'ffprobe: exit status 1: Truncating packet of size 12149 to 53
[matroska,webm @ 0x5f4ea3fb2680] EBML header parsing failed
Slouchcat Test Broken (2024).mkv: Invalid data found when processing input', NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91DPT6TRTWSPENHA7QW', '01M5YEX91DPT6TRTWSPCJ6EH40', 1, 'Slouchcat Test Chapters (2024) {tmdb-7004}/Slouchcat Test Chapters (2024) {tmdb-7004}.mkv', 66310, 3410192, 2623898, 1793097294643, 'ed0acb2c20e8c51f968af9a4deaffc6c5496033a1beacce053619f8adfeeae8d', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2623898,"bitRate":2094719,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test Chapters"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[{"start":0,"end":3000000000,"title":"Kapitel 1: Anfang"},{"start":3000000000,"end":6500000000,"title":"Kapitel 2: Mitte"},{"start":6500000000,"end":10000000000,"title":"Kapitel 3: Ende"}]}', X'0100d00fd00fd00fd00f', 1793097376912, 1793097377730, NULL, 1793097376811, 1793097377730, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91DPT6TRTWSPJM0J9A6', '01M5YEX91DPT6TRTWSPG4EZQ3M', 1, 'Slouchcat Test DTS (2024) {tmdb-7002}/Slouchcat Test DTS (2024) {tmdb-7002}.mkv', 66310, 3410182, 3427132, 1793097293559, '2cbfe34bc743453985d0037f747b01fbca63fa952a0172fb2f83466a5425731d', NULL, 'mkv', 'h264', 'dts', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":3427132,"bitRate":2741705,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test DTS"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"dts","profile":"DTS","channels":6,"channelLayout":"5.1(side)","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793097376944, 1793097377771, NULL, 1793097376811, 1793097377771, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91EAXEXBPAN6AYPWFEC', '01M5YEX91EAXEXBPAN6A9PT86N', 1, 'Slouchcat Test Extras (2024)/Slouchcat Test Extras (2024).mp4', 66310, 3410207, 2624979, 1793097303277, 'b453a8980a2883e31feeb6f382dc29c9d4b7d8f789b0c1f827ac1bb81da1caf5', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624979,"bitRate":2099983,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test Extras"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793097376987, 1793097377802, NULL, 1793097376811, 1793097377802, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91F4G0S1DRR5M1RKKTN', '01M5YEX91EAXEXBPAN6DD2HDHE', 1, 'Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024) - 720p.mp4', 66310, 3410205, 904192, 1793097302777, '0186b76306ab4b1c44501e8fd1fa880c817b1aedff7a4d9ac5de3802a5ed11c6', '720p', 'mp4', 'h264', 'aac', 1280, 720, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":904192,"bitRate":723353,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test H264 720p"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793097377019, 1793097377844, NULL, 1793097376811, 1793097377844, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91F4G0S1DRR5MT06D47', '01M5YEX91EAXEXBPAN6DD2HDHE', 1, 'Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024).mp4', 66310, 3410174, 2624977, 1793097281000, '1a3559a74acfe685061086a9246f1ad08d3f220ba78aad39e503e56f4400693b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624977,"bitRate":2099981,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test H264"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793097377051, 1793097377848, NULL, 1793097376811, 1793097377848, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91F4G0S1DRR5NFDV78P', '01M5YEX91F4G0S1DRR5MXV6SB8', 1, 'Slouchcat Test H264 Copy (2024)/Slouchcat Test H264 Copy (2024).mp4', 66310, 3410220, 2624977, 1793097304557, '1a3559a74acfe685061086a9246f1ad08d3f220ba78aad39e503e56f4400693b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624977,"bitRate":2099981,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test H264"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793097377082, 1793097377836, NULL, 1793097376811, 1793097377836, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91GVAVYSAWPTMEX67X7', '01M5YEX91GVAVYSAWPTH60QGAF', 1, 'Slouchcat Test H264 MKV Long (2024) {tmdb-7005}/Slouchcat Test H264 MKV Long (2024) {tmdb-7005}.mkv', 66310, 3410196, 13446607, 1793097300539, 'b4255bfe38e47b0ca71f55b38e8bf799ec0742c7e39bdfe7ff342707222b37ff', NULL, 'mkv', 'h264', 'aac', 1280, 720, 90021, '{"format":{"name":"matroska,webm","duration":90021000000,"size":13446607,"bitRate":1194975,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test H264 MKV Long"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00f8827e807d836f02ea846e807b06d9875e807b06d', 1793097377111, 1793097377854, NULL, 1793097376811, 1793097377854, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91GVAVYSAWPTQF8Y7FT', '01M5YEX91GVAVYSAWPTPEM5ABR', 1, 'Slouchcat Test HDR10 4K (2024)/Slouchcat Test HDR10 4K (2024).mkv', 66310, 3410178, 761065, 1793097292449, 'ac137c2e2ea2f19f9194868916488388a2cd7047170ccf23d14d2ee5e3579085', '2160p', 'mkv', 'hevc', 'aac', 3840, 2160, 30021, '{"format":{"name":"matroska,webm","duration":30021000000,"size":761065,"bitRate":202808,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test HDR10 4K"}},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":150,"width":3840,"height":2160,"pixFmt":"yuv420p10le","colorTransfer":"smpte2084","colorPrimaries":"bt2020"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793097377182, 1793097377912, NULL, 1793097376811, 1793097377912, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91H1QAJQB3A1VNJN4NA', '01M5YEX91H1QAJQB3A1RS977JA', 1, 'Slouchcat Test HDR10 Chart (2024)/Slouchcat Test HDR10 Chart (2024).mkv', 66310, 3410228, 262639, 1793097313825, 'fbc7fa3e43b481db91c94ce1c6c5f0ca76dc65eca8b4748df6135a69da267dbd', NULL, 'mkv', 'hevc', 'aac', 3840, 2160, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":262639,"bitRate":209670,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test HDR10 Chart"}},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":150,"width":3840,"height":2160,"pixFmt":"yuv420p10le","colorTransfer":"smpte2084","colorPrimaries":"bt2020"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793097377263, 1793097377911, NULL, 1793097376811, 1793097377911, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91J8VC3Z6MWQE1Q6Z14', '01M5YEX91H1QAJQB3A1Y0GQFQQ', 1, 'Slouchcat Test HEVC 10bit (2024) {tmdb-7003}/Slouchcat Test HEVC 10bit (2024) {tmdb-7003}.mkv', 66310, 3410176, 2102531, 1793097281961, 'e456f50e499e459563545d659141aa37cce54ae7f29a4c5d97a23426804dcd11', NULL, 'mkv', 'hevc', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2102531,"bitRate":1678499,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test HEVC 10bit"}},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":120,"width":1920,"height":1080,"pixFmt":"yuv420p10le","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793097377306, 1793097377905, NULL, 1793097376811, 1793097377905, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91J8VC3Z6MWQK4Q6HB6', '01M5YEX91J8VC3Z6MWQH2AVYF1', 1, 'Slouchcat Test HEVC Long (2024)/Slouchcat Test HEVC Long (2024).mkv', 66310, 3410194, 9588128, 1793097298507, '98b25288361fd38b0c4b9ba50c0c8c6e63d2414aedcb4a4814d77ef28c90f1b6', NULL, 'mkv', 'hevc', 'aac', 1280, 720, 90021, '{"format":{"name":"matroska,webm","duration":90021000000,"size":9588128,"bitRate":852079,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test HEVC Long"}},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":93,"width":1280,"height":720,"pixFmt":"yuv420p10le","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793097377338, 1793097377896, NULL, 1793097376811, 1793097377896, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91KGWA5MGH2BP2FXA8X', '01M5YEX91J8VC3Z6MWQNPPJ2TH', 1, 'Slouchcat Test Multi Audio (2024)/Slouchcat Test Multi Audio (2024).mkv', 66310, 3410224, 6263167, 1793097305924, 'ec292c5584598ee07dc3e54fa60fa9083fd1edbca87b653517a97792730b7cab', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 20021, '{"format":{"name":"matroska,webm","duration":20021000000,"size":6263167,"bitRate":2502639,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test Multi Audio"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"ger","title":"Deutsch (AAC 2.0)","default":true},{"index":2,"type":"audio","codec":"ac3","channels":6,"channelLayout":"5.1(side)","sampleRate":48000,"language":"eng","title":"English (AC3 5.1)"},{"index":3,"type":"subtitle","codec":"subrip","language":"ger","title":"Deutsch (SRT)"},{"index":4,"type":"subtitle","codec":"subrip","language":"eng","title":"English (SRT)"}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793097377369, 1793097377930, NULL, 1793097376811, 1793097377930, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91KGWA5MGH2BT2Y01Y0', '01M5YEX91KGWA5MGH2BQ4RCZEB', 1, 'Slouchcat Test PGS (2024)/Slouchcat Test PGS (2024).mkv', 66310, 3410226, 5309544, 1793097306906, 'f264f5d5fd4fc0340b7518509c6db0e73b574c5dd80a2d4d598567ae8ec25416', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 20021, '{"format":{"name":"matroska,webm","duration":20021000000,"size":5309544,"bitRate":2121589,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test PGS"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000},{"index":2,"type":"subtitle","codec":"hdmv_pgs_subtitle","width":1920,"height":1080,"language":"eng","title":"English (PGS)"}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793097377397, 1793097377937, NULL, 1793097376811, 1793097377937, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91MFMVPSQ7F56381DN5', '01M5YEX91MFMVPSQ7F55GM5XMX', 1, 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).mkv', 66310, 3410186, 2623761, 1793097294131, 'f68c4de0465834a8a8c930c296eaf0a05098c47adb3621e30a0ed2efeb300e61', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10047, '{"format":{"name":"matroska,webm","duration":10047000000,"size":2623761,"bitRate":2089189,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test Sidecar Subtitles"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0115d00fd00fd00fd00f', 1793097377425, 1793097377943, NULL, 1793097376811, 1793097377943, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91MFMVPSQ7F5B5SJ4W1', '01M5YEX91MFMVPSQ7F57FJQ7VS', 1, 'Slouchcat Test Subtitles (2024)/Slouchcat Test Subtitles (2024).mkv', 66310, 3410184, 2626565, 1793097294083, 'e54d7f40c259eb8e4cb1c3f371ab0e2419a18e5edfd08b5ff80b6584e4246ed3', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2626565,"bitRate":2096848,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test Subtitles"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000},{"index":2,"type":"subtitle","codec":"subrip","language":"ger","title":"Deutsch (SRT)","default":true},{"index":3,"type":"subtitle","codec":"subrip","language":"eng","title":"English (SRT)"},{"index":4,"type":"subtitle","codec":"ass","language":"ger","title":"Deutsch (ASS)"},{"index":5,"type":"subtitle","codec":"ass","language":"eng","title":"English (ASS)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793097377454, 1793097377950, NULL, 1793097376811, 1793097377950, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91NDKSTWFK6MJ7S9F72', '01M5YEX91NDKSTWFK6MG3MR0MQ', 1, 'Slouchcat Test Unrated/Slouchcat Test Unrated.mp4', 66310, 3410222, 2625046, 1793097305030, '4c3cb756cbf73d3913ed4c77ad5fd261f37d7f3d9e1e5c6eb988fccc17da2577', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2625046,"bitRate":2100036,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test Unrated"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793097377491, 1793097377970, NULL, 1793097376811, 1793097377970, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91PSNK4HSR97N67TZFG', '01M5YEX91NDKSTWFK6MNKBPF3Z', 1, 'Slouchcat Test Extras (2024)/Behind The Scenes/On Set.mp4', 66310, 3410215, 98171, 1793097304296, '30049f1b99ded9fca7c51ea76d1282b28f6a444a07e12d477d8401727308ddd8', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 5000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":5000000000,"size":98171,"bitRate":157073,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"On Set"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00f', 1793097377524, 1793097377968, NULL, 1793097376811, 1793097377968, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91PSNK4HSR97S4PZZ9W', '01M5YEX91PSNK4HSR97Q75GJ7J', 1, 'Slouchcat Test Extras (2024)/Deleted Scenes/Scene 1.mp4', 66310, 3410213, 98172, 1793097304051, '20e54a28024662e3d88b81bcb6f92dc549074fc06d0e0351c2f5ccb505de5a73', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 5000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":5000000000,"size":98172,"bitRate":157075,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Scene 1"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00f', 1793097377562, 1793097377978, NULL, 1793097376811, 1793097377978, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91QTS88HPAW1FH51AJR', '01M5YEX91PSNK4HSR97V3XYNA0', 1, 'Slouchcat Test Extras (2024)/Featurettes/Making Of.mp4', 66310, 3410211, 98174, 1793097303801, '4d22df8b4b0150f430bcdcffa84101300a26dec5d108f9714d1bf7d779b6ab71', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 5000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":5000000000,"size":98174,"bitRate":157078,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Making Of"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00f', 1793097377604, 1793097377978, NULL, 1793097376811, 1793097377978, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91QTS88HPAW1JVPWQ8R', '01M5YEX91QTS88HPAW1J9T39Y9', 1, 'Slouchcat Test Extras (2024)/Slouchcat Test Extras (2024)-trailer.mp4', 66310, 3410216, 98201, 1793097304543, 'b5d9ae88034b726cf9e84ac537f185f76ea8baac390744ac752990784a124e8b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 5000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":5000000000,"size":98201,"bitRate":157121,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test Extras (2024)-trailer"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00f', 1793097377637, 1793097377999, NULL, 1793097376811, 1793097377999, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M5YEX91RP8724TWHMZZTQPYQ', '01M5YEX91RP8724TWHMZGPPE0X', 1, 'Slouchcat Test Extras (2024)/Trailers/Slouchcat Test Extras Trailer.mp4', 66310, 3410209, 98194, 1793097303530, '254ced3455cc586e45a5f98fcb8547cfc93da14d64c7a58c7b7f1e911361ef2b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 5000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":5000000000,"size":98194,"bitRate":157110,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test Extras Trailer"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00f', 1793097377668, 1793097377999, NULL, 1793097376811, 1793097377999, NULL, NULL);
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
INSERT INTO "tracks" VALUES (1, '01M5YEX91CHH6S0ZPJC7XDWD7S', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (2, '01M5YEX91CHH6S0ZPJC7XDWD7S', 1, 'audio', 'ac3', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (3, '01M5YEX91DPT6TRTWSPENHA7QW', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (4, '01M5YEX91DPT6TRTWSPENHA7QW', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (5, '01M5YEX91DPT6TRTWSPJM0J9A6', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (6, '01M5YEX91DPT6TRTWSPJM0J9A6', 1, 'audio', 'dts', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (7, '01M5YEX91EAXEXBPAN6AYPWFEC', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (8, '01M5YEX91EAXEXBPAN6AYPWFEC', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (9, '01M5YEX91F4G0S1DRR5M1RKKTN', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (10, '01M5YEX91F4G0S1DRR5M1RKKTN', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (11, '01M5YEX91F4G0S1DRR5MT06D47', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (12, '01M5YEX91F4G0S1DRR5MT06D47', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (13, '01M5YEX91F4G0S1DRR5NFDV78P', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (14, '01M5YEX91F4G0S1DRR5NFDV78P', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (15, '01M5YEX91GVAVYSAWPTMEX67X7', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (16, '01M5YEX91GVAVYSAWPTMEX67X7', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (17, '01M5YEX91GVAVYSAWPTQF8Y7FT', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (18, '01M5YEX91GVAVYSAWPTQF8Y7FT', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (19, '01M5YEX91H1QAJQB3A1VNJN4NA', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (20, '01M5YEX91H1QAJQB3A1VNJN4NA', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (21, '01M5YEX91J8VC3Z6MWQE1Q6Z14', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (22, '01M5YEX91J8VC3Z6MWQE1Q6Z14', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (23, '01M5YEX91J8VC3Z6MWQK4Q6HB6', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (24, '01M5YEX91J8VC3Z6MWQK4Q6HB6', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (25, '01M5YEX91KGWA5MGH2BP2FXA8X', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (26, '01M5YEX91KGWA5MGH2BP2FXA8X', 1, 'audio', 'aac', 'ger', 'Deutsch (AAC 2.0)', 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (27, '01M5YEX91KGWA5MGH2BP2FXA8X', 2, 'audio', 'ac3', 'eng', 'English (AC3 5.1)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (28, '01M5YEX91KGWA5MGH2BP2FXA8X', 3, 'subtitle', 'subrip', 'ger', 'Deutsch (SRT)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (29, '01M5YEX91KGWA5MGH2BP2FXA8X', 4, 'subtitle', 'subrip', 'eng', 'English (SRT)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (30, '01M5YEX91KGWA5MGH2BT2Y01Y0', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (31, '01M5YEX91KGWA5MGH2BT2Y01Y0', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (32, '01M5YEX91KGWA5MGH2BT2Y01Y0', 2, 'subtitle', 'hdmv_pgs_subtitle', 'eng', 'English (PGS)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (33, '01M5YEX91MFMVPSQ7F56381DN5', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (34, '01M5YEX91MFMVPSQ7F56381DN5', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (35, '01M5YEX91MFMVPSQ7F56381DN5', 0, 'subtitle', 'ass', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.ass', 0, 0);
INSERT INTO "tracks" VALUES (36, '01M5YEX91MFMVPSQ7F56381DN5', 1, 'subtitle', 'subrip', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.srt', 0, 0);
INSERT INTO "tracks" VALUES (37, '01M5YEX91MFMVPSQ7F56381DN5', 2, 'subtitle', 'ass', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.ass', 0, 0);
INSERT INTO "tracks" VALUES (38, '01M5YEX91MFMVPSQ7F56381DN5', 3, 'subtitle', 'subrip', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.srt', 0, 0);
INSERT INTO "tracks" VALUES (39, '01M5YEX91MFMVPSQ7F5B5SJ4W1', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (40, '01M5YEX91MFMVPSQ7F5B5SJ4W1', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (41, '01M5YEX91MFMVPSQ7F5B5SJ4W1', 2, 'subtitle', 'subrip', 'ger', 'Deutsch (SRT)', 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (42, '01M5YEX91MFMVPSQ7F5B5SJ4W1', 3, 'subtitle', 'subrip', 'eng', 'English (SRT)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (43, '01M5YEX91MFMVPSQ7F5B5SJ4W1', 4, 'subtitle', 'ass', 'ger', 'Deutsch (ASS)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (44, '01M5YEX91MFMVPSQ7F5B5SJ4W1', 5, 'subtitle', 'ass', 'eng', 'English (ASS)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (45, '01M5YEX91NDKSTWFK6MJ7S9F72', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (46, '01M5YEX91NDKSTWFK6MJ7S9F72', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (47, '01M5YEX91PSNK4HSR97N67TZFG', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (48, '01M5YEX91PSNK4HSR97N67TZFG', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (49, '01M5YEX91PSNK4HSR97S4PZZ9W', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (50, '01M5YEX91PSNK4HSR97S4PZZ9W', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (51, '01M5YEX91QTS88HPAW1FH51AJR', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (52, '01M5YEX91QTS88HPAW1FH51AJR', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (53, '01M5YEX91QTS88HPAW1JVPWQ8R', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (54, '01M5YEX91QTS88HPAW1JVPWQ8R', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (55, '01M5YEX91RP8724TWHMZZTQPYQ', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (56, '01M5YEX91RP8724TWHMZZTQPYQ', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
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
INSERT INTO "devices" VALUES ('59e49def4b4ba10d0ac32cba40f82fc7', 'curl/8.5.0', 1793097375881, 1793097375881, NULL, NULL, NULL, NULL, NULL, NULL);
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
INSERT INTO "sessions" VALUES (1, 'a7fa509c0af25442fc44fbb04878540e4319a6f117a97ff2fc5fe87b84e41e52', 1, 1, '59e49def4b4ba10d0ac32cba40f82fc7', 'curl/8.5.0', 1793097375881, 1793097375881, 1795689375881, NULL);
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
INSERT INTO "libraries" VALUES (1, 'Filme', 'movies', NULL, 1793097375880, 1793097376824, 'done', 1793097376808, 1793097376824, NULL, 23, 23, NULL);
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
INSERT INTO "search_docs" VALUES (1, 'item', '01M5YEX91B69XH2ADY2M9EZH4D', '01M5YEX91B69XH2ADY2M9EZH4D', 'Slouchcat Test AC3 51 /  /  /  /');
INSERT INTO "search_docs" VALUES (2, 'item', '01M5YEX91CHH6S0ZPJC8XRZVSK', '01M5YEX91CHH6S0ZPJC8XRZVSK', 'Slouchcat Test Broken /  /  /  /');
INSERT INTO "search_docs" VALUES (3, 'item', '01M5YEX91DPT6TRTWSPCJ6EH40', '01M5YEX91DPT6TRTWSPCJ6EH40', 'Slouchcat Test Chapters /  /  /  /');
INSERT INTO "search_docs" VALUES (4, 'item', '01M5YEX91DPT6TRTWSPG4EZQ3M', '01M5YEX91DPT6TRTWSPG4EZQ3M', 'Slouchcat Test DTS /  /  /  /');
INSERT INTO "search_docs" VALUES (5, 'item', '01M5YEX91EAXEXBPAN6A9PT86N', '01M5YEX91EAXEXBPAN6A9PT86N', 'Slouchcat Test Extras /  /  /  /');
INSERT INTO "search_docs" VALUES (6, 'item', '01M5YEX91EAXEXBPAN6DD2HDHE', '01M5YEX91EAXEXBPAN6DD2HDHE', 'Slouchcat Test H264 /  /  /  /');
INSERT INTO "search_docs" VALUES (7, 'item', '01M5YEX91F4G0S1DRR5MXV6SB8', '01M5YEX91F4G0S1DRR5MXV6SB8', 'Slouchcat Test H264 Copy /  /  /  /');
INSERT INTO "search_docs" VALUES (8, 'item', '01M5YEX91GVAVYSAWPTH60QGAF', '01M5YEX91GVAVYSAWPTH60QGAF', 'Slouchcat Test H264 MKV Long /  /  /  /');
INSERT INTO "search_docs" VALUES (9, 'item', '01M5YEX91GVAVYSAWPTPEM5ABR', '01M5YEX91GVAVYSAWPTPEM5ABR', 'Slouchcat Test HDR10 4K /  /  /  /');
INSERT INTO "search_docs" VALUES (10, 'item', '01M5YEX91H1QAJQB3A1RS977JA', '01M5YEX91H1QAJQB3A1RS977JA', 'Slouchcat Test HDR10 Chart /  /  /  /');
INSERT INTO "search_docs" VALUES (11, 'item', '01M5YEX91H1QAJQB3A1Y0GQFQQ', '01M5YEX91H1QAJQB3A1Y0GQFQQ', 'Slouchcat Test HEVC 10bit /  /  /  /');
INSERT INTO "search_docs" VALUES (12, 'item', '01M5YEX91J8VC3Z6MWQH2AVYF1', '01M5YEX91J8VC3Z6MWQH2AVYF1', 'Slouchcat Test HEVC Long /  /  /  /');
INSERT INTO "search_docs" VALUES (13, 'item', '01M5YEX91J8VC3Z6MWQNPPJ2TH', '01M5YEX91J8VC3Z6MWQNPPJ2TH', 'Slouchcat Test Multi Audio /  /  /  /');
INSERT INTO "search_docs" VALUES (14, 'item', '01M5YEX91KGWA5MGH2BQ4RCZEB', '01M5YEX91KGWA5MGH2BQ4RCZEB', 'Slouchcat Test PGS /  /  /  /');
INSERT INTO "search_docs" VALUES (15, 'item', '01M5YEX91MFMVPSQ7F55GM5XMX', '01M5YEX91MFMVPSQ7F55GM5XMX', 'Slouchcat Test Sidecar Subtitles /  /  /  /');
INSERT INTO "search_docs" VALUES (16, 'item', '01M5YEX91MFMVPSQ7F57FJQ7VS', '01M5YEX91MFMVPSQ7F57FJQ7VS', 'Slouchcat Test Subtitles /  /  /  /');
INSERT INTO "search_docs" VALUES (17, 'item', '01M5YEX91NDKSTWFK6MG3MR0MQ', '01M5YEX91NDKSTWFK6MG3MR0MQ', 'Slouchcat Test Unrated /  /  /  /');
INSERT INTO "search_docs" VALUES (18, 'item', '01M5YEX91NDKSTWFK6MNKBPF3Z', '01M5YEX91NDKSTWFK6MNKBPF3Z', 'On Set /  /  /  /');
INSERT INTO "search_docs" VALUES (19, 'item', '01M5YEX91PSNK4HSR97Q75GJ7J', '01M5YEX91PSNK4HSR97Q75GJ7J', 'Scene 1 /  /  /  /');
INSERT INTO "search_docs" VALUES (20, 'item', '01M5YEX91PSNK4HSR97V3XYNA0', '01M5YEX91PSNK4HSR97V3XYNA0', 'Making Of /  /  /  /');
INSERT INTO "search_docs" VALUES (21, 'item', '01M5YEX91QTS88HPAW1J9T39Y9', '01M5YEX91QTS88HPAW1J9T39Y9', 'Slouchcat Test Extras (2024) /  /  /  /');
INSERT INTO "search_docs" VALUES (22, 'item', '01M5YEX91RP8724TWHMZGPPE0X', '01M5YEX91RP8724TWHMZGPPE0X', 'Slouchcat Test Extras Trailer /  /  /  /');
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
INSERT INTO "account_channels" VALUES (1, 'email', 0, '["new_content","errors","invitations","announcements"]', '{}', 1793097375880);
INSERT INTO "account_channels" VALUES (1, 'push', 0, '["new_content","errors","invitations","announcements"]', '{}', 1793097375880);
INSERT INTO "account_channels" VALUES (1, 'telegram', 0, '["new_content","errors","invitations","announcements"]', '{}', 1793097375880);
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
INSERT INTO "home_rows" VALUES ('picks', 2, 0, 1793097406969);
INSERT INTO "home_rows" VALUES ('continue', 1, 0, 1793097406969);
INSERT INTO "home_rows" VALUES ('recent', 3, 0, 1793097406969);
INSERT INTO "home_rows" VALUES ('watchlist', 5, 0, 1793097406969);
INSERT INTO "home_rows" VALUES ('collections', 6, 0, 1793097406969);
INSERT INTO "home_rows" VALUES ('genres', 7, 1, 1793097406969);
INSERT INTO "home_rows" VALUES ('spotlight', 0, 0, 1793097406969);
INSERT INTO "home_rows" VALUES ('released', 4, 0, 1793097406969);
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
INSERT INTO "spotlight_config" VALUES (1, 1, 'trending', NULL, 'both', 12, 2, 12, 1793097406924, 1793097406919);
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
INSERT INTO "spotlight_entries" VALUES ('trending', 'movie', 1, 7001, 'Slouchcat Test AC3 51', 2024, 1793097376803);
INSERT INTO "spotlight_entries" VALUES ('trending', 'movie', 2, 7002, 'Slouchcat Test DTS', 2024, 1793097376803);
INSERT INTO "spotlight_entries" VALUES ('trending', 'movie', 3, 7003, 'Slouchcat Test HEVC 10bit', 2024, 1793097376803);
INSERT INTO "spotlight_entries" VALUES ('trending', 'movie', 4, 7999, 'Slouchcat Not In The Library', 2023, 1793097376803);
INSERT INTO "spotlight_entries" VALUES ('trending', 'show', 1, 7101, 'Slouchcat Test Download Show', 2024, 1793097376803);
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
INSERT INTO "spotlight_overrides" VALUES ('trending', 'movie', 7002, 0, 1, 'Cats Bühnentipp', 1793097406949);
INSERT INTO "spotlight_overrides" VALUES ('trending', 'movie', 7999, 1, NULL, NULL, 1793097406960);
INSERT INTO "spotlight_overrides" VALUES ('trending', 'show', 7101, 0, NULL, 'Die Testserie, jede Woche neu', 1793097406964);
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
