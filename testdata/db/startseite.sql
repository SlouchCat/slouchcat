CREATE TABLE goose_db_version (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now'))
	);
INSERT INTO "goose_db_version" VALUES (1, 0, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (2, 1, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (3, 2, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (4, 3, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (5, 4, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (6, 5, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (7, 6, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (8, 7, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (9, 8, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (10, 9, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (11, 10, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (12, 11, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (13, 12, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (14, 13, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (15, 14, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (16, 15, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (17, 16, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (18, 17, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (19, 18, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (20, 19, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (21, 20, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (22, 21, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (23, 22, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (24, 23, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (25, 24, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (26, 25, 1, '2026-10-30 01:53:15 +0000 UTC');
INSERT INTO "goose_db_version" VALUES (27, 26, 1, '2026-10-30 01:53:15 +0000 UTC');
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
INSERT INTO "jobs" VALUES (1, 'library.sweep', 2, 'sweep', '{}', 'done', 1793325197219, NULL, NULL, 1, 5, NULL, 1793325197219, 1793325198223);
INSERT INTO "jobs" VALUES (2, 'scan.library', 5, 'scan:1', '{"libraryId":1}', 'done', 1793325197707, NULL, NULL, 1, 5, NULL, 1793325197707, 1793325198292);
INSERT INTO "jobs" VALUES (3, 'scan.library', 5, 'scan:2', '{"libraryId":2}', 'done', 1793325197764, NULL, NULL, 1, 5, NULL, 1793325197764, 1793325198292);
INSERT INTO "jobs" VALUES (4, 'metadata.match', 4, 'match:01M6585TWQDRBNT5QQW1HXW2JH', '{"itemId":"01M6585TWQDRBNT5QQW1HXW2JH"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325198789);
INSERT INTO "jobs" VALUES (5, 'probe.file', 3, 'probe:01M6585TWT1YNE1CH3TZ3Q9HRF', '{"mediaFileId":"01M6585TWT1YNE1CH3TZ3Q9HRF"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199543);
INSERT INTO "jobs" VALUES (6, 'probe.file', 3, 'probe:01M6585TWXFMDSEPE5ANZ5WTCB', '{"mediaFileId":"01M6585TWXFMDSEPE5ANZ5WTCB"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199562);
INSERT INTO "jobs" VALUES (7, 'probe.file', 3, 'probe:01M6585TX0A6KKFTTK4AM5BRK8', '{"mediaFileId":"01M6585TX0A6KKFTTK4AM5BRK8"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199575);
INSERT INTO "jobs" VALUES (8, 'probe.file', 3, 'probe:01M6585TX2N7TACN188D77429J', '{"mediaFileId":"01M6585TX2N7TACN188D77429J"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199591);
INSERT INTO "jobs" VALUES (9, 'probe.file', 3, 'probe:01M6585TX4J3HDJ459C70A729B', '{"mediaFileId":"01M6585TX4J3HDJ459C70A729B"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199593);
INSERT INTO "jobs" VALUES (10, 'probe.file', 3, 'probe:01M6585TX6FV1CMGBDPNKQFBZ9', '{"mediaFileId":"01M6585TX6FV1CMGBDPNKQFBZ9"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199597);
INSERT INTO "jobs" VALUES (11, 'metadata.match', 4, 'match:01M6585TX6FV1CMGBDPS84K004', '{"itemId":"01M6585TX6FV1CMGBDPS84K004"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325198683);
INSERT INTO "jobs" VALUES (12, 'probe.file', 3, 'probe:01M6585TX9NYJJJ3RSXYYKAVP4', '{"mediaFileId":"01M6585TX9NYJJJ3RSXYYKAVP4"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199605);
INSERT INTO "jobs" VALUES (13, 'probe.file', 3, 'probe:01M6585TXANBCG17M505BH3Q06', '{"mediaFileId":"01M6585TXANBCG17M505BH3Q06"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199618);
INSERT INTO "jobs" VALUES (14, 'probe.file', 3, 'probe:01M6585TXBMVWKFXBHJXPXMAV8', '{"mediaFileId":"01M6585TXBMVWKFXBHJXPXMAV8"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199618);
INSERT INTO "jobs" VALUES (15, 'probe.file', 3, 'probe:01M6585TXCHS51027J7B63SVZ6', '{"mediaFileId":"01M6585TXCHS51027J7B63SVZ6"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199642);
INSERT INTO "jobs" VALUES (16, 'probe.file', 3, 'probe:01M6585TXEKWNC2RXHZ0XN2X6Q', '{"mediaFileId":"01M6585TXEKWNC2RXHZ0XN2X6Q"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199648);
INSERT INTO "jobs" VALUES (17, 'metadata.match', 4, 'match:01M6585TXFV7FMJPYSCG96AQRK', '{"itemId":"01M6585TXFV7FMJPYSCG96AQRK"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325198956);
INSERT INTO "jobs" VALUES (18, 'probe.file', 3, 'probe:01M6585TXGEC9NVX13PAQ17V84', '{"mediaFileId":"01M6585TXGEC9NVX13PAQ17V84"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199650);
INSERT INTO "jobs" VALUES (19, 'probe.file', 3, 'probe:01M6585TXHVBT2ZBQVVXKTTNA3', '{"mediaFileId":"01M6585TXHVBT2ZBQVVXKTTNA3"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199650);
INSERT INTO "jobs" VALUES (20, 'probe.file', 3, 'probe:01M6585TXHVBT2ZBQVW296NXXY', '{"mediaFileId":"01M6585TXHVBT2ZBQVW296NXXY"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199681);
INSERT INTO "jobs" VALUES (21, 'probe.file', 3, 'probe:01M6585TXJHQ022AZ82V1BJ4WN', '{"mediaFileId":"01M6585TXJHQ022AZ82V1BJ4WN"}', 'done', 1793325198231, NULL, NULL, 1, 5, NULL, 1793325198231, 1793325199685);
INSERT INTO "jobs" VALUES (22, 'metadata.match', 4, 'match:01M6585TXK7NNK68XFTPTBESVF', '{"itemId":"01M6585TXK7NNK68XFTPTBESVF"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325198900);
INSERT INTO "jobs" VALUES (23, 'probe.file', 3, 'probe:01M6585TXM70G9GSBN3Z8T42WQ', '{"mediaFileId":"01M6585TXM70G9GSBN3Z8T42WQ"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199680);
INSERT INTO "jobs" VALUES (24, 'metadata.match', 4, 'match:01M6585TXM70G9GSBN41B2G7D0', '{"itemId":"01M6585TXM70G9GSBN41B2G7D0"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199018);
INSERT INTO "jobs" VALUES (25, 'probe.file', 3, 'probe:01M6585TXNTJS6VDVEE232TD2D', '{"mediaFileId":"01M6585TXNTJS6VDVEE232TD2D"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199674);
INSERT INTO "jobs" VALUES (26, 'metadata.match', 4, 'match:01M6585TXNTJS6VDVEE4JJQ3P1', '{"itemId":"01M6585TXNTJS6VDVEE4JJQ3P1"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199069);
INSERT INTO "jobs" VALUES (27, 'probe.file', 3, 'probe:01M6585TXPNMFEW1D01BQK1CKQ', '{"mediaFileId":"01M6585TXPNMFEW1D01BQK1CKQ"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199714);
INSERT INTO "jobs" VALUES (28, 'metadata.match', 4, 'match:01M6585TXPNMFEW1D01E2HFA8X', '{"itemId":"01M6585TXPNMFEW1D01E2HFA8X"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199131);
INSERT INTO "jobs" VALUES (29, 'probe.file', 3, 'probe:01M6585TXQPPWG4PVPX5WH98G9', '{"mediaFileId":"01M6585TXQPPWG4PVPX5WH98G9"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199712);
INSERT INTO "jobs" VALUES (30, 'metadata.match', 4, 'match:01M6585TXRHCKT35BV8XKA9H60', '{"itemId":"01M6585TXRHCKT35BV8XKA9H60"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199185);
INSERT INTO "jobs" VALUES (31, 'probe.file', 3, 'probe:01M6585TXRHCKT35BV8YYN6X1G', '{"mediaFileId":"01M6585TXRHCKT35BV8YYN6X1G"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199711);
INSERT INTO "jobs" VALUES (32, 'metadata.match', 4, 'match:01M6585TXSN46TF04KKM2FK30H', '{"itemId":"01M6585TXSN46TF04KKM2FK30H"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199261);
INSERT INTO "jobs" VALUES (33, 'probe.file', 3, 'probe:01M6585TXT49PGDCXHCA7N0SV4', '{"mediaFileId":"01M6585TXT49PGDCXHCA7N0SV4"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199711);
INSERT INTO "jobs" VALUES (34, 'probe.file', 3, 'probe:01M6585TXT49PGDCXHCBYT3NJJ', '{"mediaFileId":"01M6585TXT49PGDCXHCBYT3NJJ"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199762);
INSERT INTO "jobs" VALUES (35, 'metadata.match', 4, 'match:01M6585TXV85XJJ0DS5Z7Y68VM', '{"itemId":"01M6585TXV85XJJ0DS5Z7Y68VM"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199302);
INSERT INTO "jobs" VALUES (36, 'probe.file', 3, 'probe:01M6585TXWMGFWKDKB0CFC7GFR', '{"mediaFileId":"01M6585TXWMGFWKDKB0CFC7GFR"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199762);
INSERT INTO "jobs" VALUES (37, 'metadata.match', 4, 'match:01M6585TXXSRR48VSW6EVTC46P', '{"itemId":"01M6585TXXSRR48VSW6EVTC46P"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199336);
INSERT INTO "jobs" VALUES (38, 'probe.file', 3, 'probe:01M6585TXYKMDET3BAZ1E56GYJ', '{"mediaFileId":"01M6585TXYKMDET3BAZ1E56GYJ"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199745);
INSERT INTO "jobs" VALUES (39, 'metadata.match', 4, 'match:01M6585TXZ9E4QBE7KXJ408K87', '{"itemId":"01M6585TXZ9E4QBE7KXJ408K87"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199344);
INSERT INTO "jobs" VALUES (40, 'probe.file', 3, 'probe:01M6585TY0V149WSKVTXVXAPXZ', '{"mediaFileId":"01M6585TY0V149WSKVTXVXAPXZ"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199802);
INSERT INTO "jobs" VALUES (41, 'metadata.match', 4, 'match:01M6585TY1F43AEP4WPVG98FXG', '{"itemId":"01M6585TY1F43AEP4WPVG98FXG"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199380);
INSERT INTO "jobs" VALUES (42, 'probe.file', 3, 'probe:01M6585TY2F5QXFA1ZTJ4V4H7A', '{"mediaFileId":"01M6585TY2F5QXFA1ZTJ4V4H7A"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199828);
INSERT INTO "jobs" VALUES (43, 'metadata.match', 4, 'match:01M6585TY3CMXZFGPM4XVA6K63', '{"itemId":"01M6585TY3CMXZFGPM4XVA6K63"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199413);
INSERT INTO "jobs" VALUES (44, 'probe.file', 3, 'probe:01M6585TY5S60SV3DEYXS58QYR', '{"mediaFileId":"01M6585TY5S60SV3DEYXS58QYR"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199792);
INSERT INTO "jobs" VALUES (45, 'metadata.match', 4, 'match:01M6585TY6Z709NFGCE902YJD9', '{"itemId":"01M6585TY6Z709NFGCE902YJD9"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199454);
INSERT INTO "jobs" VALUES (46, 'probe.file', 3, 'probe:01M6585TY7APW6R5BQ43BJ4Q8V', '{"mediaFileId":"01M6585TY7APW6R5BQ43BJ4Q8V"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199792);
INSERT INTO "jobs" VALUES (47, 'metadata.match', 4, 'match:01M6585TY82HPB7DNYVEFJ57ZE', '{"itemId":"01M6585TY82HPB7DNYVEFJ57ZE"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199459);
INSERT INTO "jobs" VALUES (48, 'probe.file', 3, 'probe:01M6585TY97A2XBTT5DMSCH37V', '{"mediaFileId":"01M6585TY97A2XBTT5DMSCH37V"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199832);
INSERT INTO "jobs" VALUES (49, 'metadata.match', 4, 'match:01M6585TYAPAP726NKRT2YB2Q5', '{"itemId":"01M6585TYAPAP726NKRT2YB2Q5"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199496);
INSERT INTO "jobs" VALUES (50, 'probe.file', 3, 'probe:01M6585TYBM1BK4M91ZZRZCBWN', '{"mediaFileId":"01M6585TYBM1BK4M91ZZRZCBWN"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199834);
INSERT INTO "jobs" VALUES (51, 'metadata.match', 4, 'match:01M6585TYBM1BK4M9202GET26Y', '{"itemId":"01M6585TYBM1BK4M9202GET26Y"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199530);
INSERT INTO "jobs" VALUES (52, 'probe.file', 3, 'probe:01M6585TYCYRGFPP41J7FQEEQD', '{"mediaFileId":"01M6585TYCYRGFPP41J7FQEEQD"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199831);
INSERT INTO "jobs" VALUES (53, 'metadata.match', 4, 'match:01M6585TYCYRGFPP41JB1ABAY4', '{"itemId":"01M6585TYCYRGFPP41JB1ABAY4"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199569);
INSERT INTO "jobs" VALUES (54, 'probe.file', 3, 'probe:01M6585TYD2X2X729XG9DEMSKF', '{"mediaFileId":"01M6585TYD2X2X729XG9DEMSKF"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199860);
INSERT INTO "jobs" VALUES (55, 'metadata.match', 4, 'match:01M6585TYE1RWHX6C8CR9YJD94', '{"itemId":"01M6585TYE1RWHX6C8CR9YJD94"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199566);
INSERT INTO "jobs" VALUES (56, 'probe.file', 3, 'probe:01M6585TYE1RWHX6C8CTJQM6YM', '{"mediaFileId":"01M6585TYE1RWHX6C8CTJQM6YM"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199861);
INSERT INTO "jobs" VALUES (57, 'probe.file', 3, 'probe:01M6585TYF4HEW4J1E4N59SE9N', '{"mediaFileId":"01M6585TYF4HEW4J1E4N59SE9N"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199865);
INSERT INTO "jobs" VALUES (58, 'probe.file', 3, 'probe:01M6585TYG50HT3BSC3K6GPE80', '{"mediaFileId":"01M6585TYG50HT3BSC3K6GPE80"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199864);
INSERT INTO "jobs" VALUES (59, 'probe.file', 3, 'probe:01M6585TYHS7QMZSG7AS0VRQAK', '{"mediaFileId":"01M6585TYHS7QMZSG7AS0VRQAK"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199901);
INSERT INTO "jobs" VALUES (60, 'probe.file', 3, 'probe:01M6585TYHS7QMZSG7AVR7ZZ5F', '{"mediaFileId":"01M6585TYHS7QMZSG7AVR7ZZ5F"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199904);
INSERT INTO "jobs" VALUES (61, 'probe.file', 3, 'probe:01M6585TYJ68J0E2CS58PM7T3N', '{"mediaFileId":"01M6585TYJ68J0E2CS58PM7T3N"}', 'done', 1793325198232, NULL, NULL, 1, 5, NULL, 1793325198232, 1793325199894);
INSERT INTO "jobs" VALUES (62, 'metadata.match', 3, 'match:01M6585TX877H28GBN26XWG310', '{"itemId":"01M6585TX877H28GBN26XWG310"}', 'done', 1793325198681, NULL, NULL, 1, 5, NULL, 1793325198681, 1793325199976);
INSERT INTO "jobs" VALUES (63, 'metadata.match', 3, 'match:01M6585TX9NYJJJ3RSY1ARMMC0', '{"itemId":"01M6585TX9NYJJJ3RSY1ARMMC0"}', 'done', 1793325198682, NULL, NULL, 1, 5, NULL, 1793325198682, 1793325200004);
INSERT INTO "jobs" VALUES (64, 'metadata.match', 3, 'match:01M6585TXANBCG17M508AY0707', '{"itemId":"01M6585TXANBCG17M508AY0707"}', 'done', 1793325198682, NULL, NULL, 1, 5, NULL, 1793325198682, 1793325200011);
INSERT INTO "jobs" VALUES (65, 'metadata.match', 3, 'match:01M6585TXCHS51027J79733F3J', '{"itemId":"01M6585TXCHS51027J79733F3J"}', 'done', 1793325198682, NULL, NULL, 1, 5, NULL, 1793325198682, 1793325200455);
INSERT INTO "jobs" VALUES (66, 'metadata.match', 3, 'match:01M6585TXDAXTMQ0X78MGGC0BC', '{"itemId":"01M6585TXDAXTMQ0X78MGGC0BC"}', 'done', 1793325198682, NULL, NULL, 1, 5, NULL, 1793325198682, 1793325200087);
INSERT INTO "jobs" VALUES (67, 'metadata.fill', 2, 'fill:01M6585TX6FV1CMGBDPS84K004', '{"itemId":"01M6585TX6FV1CMGBDPS84K004"}', 'done', 1793325198682, NULL, NULL, 1, 5, NULL, 1793325198682, 1793325200017);
INSERT INTO "jobs" VALUES (68, 'trailer.find', 1, 'trailer:01M6585TX6FV1CMGBDPS84K004', '{"itemId":"01M6585TX6FV1CMGBDPS84K004"}', 'done', 1793325198683, NULL, NULL, 1, 5, NULL, 1793325198683, 1793325200676);
INSERT INTO "jobs" VALUES (69, 'image.fetch', 2, 'image:01M6585TWQDRBNT5QQW1HXW2JH', '{"itemId":"01M6585TWQDRBNT5QQW1HXW2JH"}', 'done', 1793325198789, NULL, NULL, 1, 5, NULL, 1793325198789, 1793325200823);
INSERT INTO "jobs" VALUES (70, 'metadata.fill', 2, 'fill:01M6585TWQDRBNT5QQW1HXW2JH', '{"itemId":"01M6585TWQDRBNT5QQW1HXW2JH"}', 'done', 1793325198789, NULL, NULL, 1, 5, NULL, 1793325198789, 1793325200031);
INSERT INTO "jobs" VALUES (71, 'trailer.find', 1, 'trailer:01M6585TWQDRBNT5QQW1HXW2JH', '{"itemId":"01M6585TWQDRBNT5QQW1HXW2JH"}', 'done', 1793325198789, NULL, NULL, 1, 5, NULL, 1793325198789, 1793325200699);
INSERT INTO "jobs" VALUES (72, 'probe.keyframes', 1, 'keyframes:01M6585TWT1YNE1CH3TZ3Q9HRF', '{"mediaFileId":"01M6585TWT1YNE1CH3TZ3Q9HRF"}', 'done', 1793325199542, NULL, NULL, 1, 5, NULL, 1793325199542, 1793325200707);
INSERT INTO "jobs" VALUES (73, 'probe.keyframes', 1, 'keyframes:01M6585TWXFMDSEPE5ANZ5WTCB', '{"mediaFileId":"01M6585TWXFMDSEPE5ANZ5WTCB"}', 'done', 1793325199561, NULL, NULL, 1, 5, NULL, 1793325199561, 1793325200726);
INSERT INTO "jobs" VALUES (74, 'segments.detect', 2, 'segments:01M6585TWSZF0B4KYY554V0591', '{"seasonId":"01M6585TWSZF0B4KYY554V0591"}', 'done', 1793325199561, NULL, NULL, 1, 5, NULL, 1793325199561, 1793325200289);
INSERT INTO "jobs" VALUES (75, 'probe.keyframes', 1, 'keyframes:01M6585TX0A6KKFTTK4AM5BRK8', '{"mediaFileId":"01M6585TX0A6KKFTTK4AM5BRK8"}', 'done', 1793325199575, NULL, NULL, 1, 5, NULL, 1793325199575, 1793325200736);
INSERT INTO "jobs" VALUES (76, 'probe.keyframes', 1, 'keyframes:01M6585TX2N7TACN188D77429J', '{"mediaFileId":"01M6585TX2N7TACN188D77429J"}', 'done', 1793325199591, NULL, NULL, 1, 5, NULL, 1793325199591, 1793325200764);
INSERT INTO "jobs" VALUES (77, 'probe.keyframes', 1, 'keyframes:01M6585TX4J3HDJ459C70A729B', '{"mediaFileId":"01M6585TX4J3HDJ459C70A729B"}', 'done', 1793325199593, NULL, NULL, 1, 5, NULL, 1793325199593, 1793325200763);
INSERT INTO "jobs" VALUES (78, 'probe.keyframes', 1, 'keyframes:01M6585TX6FV1CMGBDPNKQFBZ9', '{"mediaFileId":"01M6585TX6FV1CMGBDPNKQFBZ9"}', 'done', 1793325199597, NULL, NULL, 1, 5, NULL, 1793325199597, 1793325200771);
INSERT INTO "jobs" VALUES (79, 'probe.keyframes', 1, 'keyframes:01M6585TX9NYJJJ3RSXYYKAVP4', '{"mediaFileId":"01M6585TX9NYJJJ3RSXYYKAVP4"}', 'done', 1793325199604, NULL, NULL, 1, 5, NULL, 1793325199604, 1793325200799);
INSERT INTO "jobs" VALUES (80, 'probe.keyframes', 1, 'keyframes:01M6585TXANBCG17M505BH3Q06', '{"mediaFileId":"01M6585TXANBCG17M505BH3Q06"}', 'done', 1793325199617, NULL, NULL, 1, 5, NULL, 1793325199617, 1793325200796);
INSERT INTO "jobs" VALUES (81, 'segments.detect', 2, 'segments:01M6585TX7TT52TQKRKR9G5KMG', '{"seasonId":"01M6585TX7TT52TQKRKR9G5KMG"}', 'done', 1793325199617, NULL, NULL, 1, 5, NULL, 1793325199617, 1793325200741);
INSERT INTO "jobs" VALUES (82, 'probe.keyframes', 1, 'keyframes:01M6585TXBMVWKFXBHJXPXMAV8', '{"mediaFileId":"01M6585TXBMVWKFXBHJXPXMAV8"}', 'done', 1793325199618, NULL, NULL, 1, 5, NULL, 1793325199618, 1793325200803);
INSERT INTO "jobs" VALUES (83, 'probe.keyframes', 1, 'keyframes:01M6585TXCHS51027J7B63SVZ6', '{"mediaFileId":"01M6585TXCHS51027J7B63SVZ6"}', 'done', 1793325199622, NULL, NULL, 1, 5, NULL, 1793325199622, 1793325200828);
INSERT INTO "jobs" VALUES (84, 'probe.keyframes', 1, 'keyframes:01M6585TXEKWNC2RXHZ0XN2X6Q', '{"mediaFileId":"01M6585TXEKWNC2RXHZ0XN2X6Q"}', 'done', 1793325199631, NULL, NULL, 1, 5, NULL, 1793325199631, 1793325200836);
INSERT INTO "jobs" VALUES (85, 'probe.keyframes', 1, 'keyframes:01M6585TXHVBT2ZBQVVXKTTNA3', '{"mediaFileId":"01M6585TXHVBT2ZBQVVXKTTNA3"}', 'done', 1793325199649, NULL, NULL, 1, 5, NULL, 1793325199649, 1793325200832);
INSERT INTO "jobs" VALUES (86, 'probe.keyframes', 1, 'keyframes:01M6585TXGEC9NVX13PAQ17V84', '{"mediaFileId":"01M6585TXGEC9NVX13PAQ17V84"}', 'done', 1793325199649, NULL, NULL, 1, 5, NULL, 1793325199649, 1793325200854);
INSERT INTO "jobs" VALUES (87, 'segments.detect', 2, 'segments:01M6585TXFV7FMJPYSCH66A7DW', '{"seasonId":"01M6585TXFV7FMJPYSCH66A7DW"}', 'done', 1793325199649, NULL, NULL, 1, 5, NULL, 1793325199649, 1793325200476);
INSERT INTO "jobs" VALUES (88, 'probe.keyframes', 1, 'keyframes:01M6585TXM70G9GSBN3Z8T42WQ', '{"mediaFileId":"01M6585TXM70G9GSBN3Z8T42WQ"}', 'done', 1793325199680, NULL, NULL, 1, 5, NULL, 1793325199680, 1793325200861);
INSERT INTO "jobs" VALUES (89, 'probe.keyframes', 1, 'keyframes:01M6585TXHVBT2ZBQVW296NXXY', '{"mediaFileId":"01M6585TXHVBT2ZBQVW296NXXY"}', 'done', 1793325199681, NULL, NULL, 1, 5, NULL, 1793325199681, 1793325200863);
INSERT INTO "jobs" VALUES (90, 'probe.keyframes', 1, 'keyframes:01M6585TXJHQ022AZ82V1BJ4WN', '{"mediaFileId":"01M6585TXJHQ022AZ82V1BJ4WN"}', 'done', 1793325199685, NULL, NULL, 1, 5, NULL, 1793325199685, 1793325200863);
INSERT INTO "jobs" VALUES (91, 'probe.keyframes', 1, 'keyframes:01M6585TXRHCKT35BV8YYN6X1G', '{"mediaFileId":"01M6585TXRHCKT35BV8YYN6X1G"}', 'done', 1793325199710, NULL, NULL, 1, 5, NULL, 1793325199710, 1793325200900);
INSERT INTO "jobs" VALUES (92, 'probe.keyframes', 1, 'keyframes:01M6585TXT49PGDCXHCA7N0SV4', '{"mediaFileId":"01M6585TXT49PGDCXHCA7N0SV4"}', 'done', 1793325199711, NULL, NULL, 1, 5, NULL, 1793325199711, 1793325200889);
INSERT INTO "jobs" VALUES (93, 'probe.keyframes', 1, 'keyframes:01M6585TXQPPWG4PVPX5WH98G9', '{"mediaFileId":"01M6585TXQPPWG4PVPX5WH98G9"}', 'done', 1793325199712, NULL, NULL, 1, 5, NULL, 1793325199712, 1793325200906);
INSERT INTO "jobs" VALUES (94, 'probe.keyframes', 1, 'keyframes:01M6585TXPNMFEW1D01BQK1CKQ', '{"mediaFileId":"01M6585TXPNMFEW1D01BQK1CKQ"}', 'done', 1793325199713, NULL, NULL, 1, 5, NULL, 1793325199713, 1793325200892);
INSERT INTO "jobs" VALUES (95, 'probe.keyframes', 1, 'keyframes:01M6585TXYKMDET3BAZ1E56GYJ', '{"mediaFileId":"01M6585TXYKMDET3BAZ1E56GYJ"}', 'done', 1793325199745, NULL, NULL, 1, 5, NULL, 1793325199745, 1793325200925);
INSERT INTO "jobs" VALUES (96, 'probe.keyframes', 1, 'keyframes:01M6585TXT49PGDCXHCBYT3NJJ', '{"mediaFileId":"01M6585TXT49PGDCXHCBYT3NJJ"}', 'done', 1793325199761, NULL, NULL, 1, 5, NULL, 1793325199761, 1793325200922);
INSERT INTO "jobs" VALUES (97, 'probe.keyframes', 1, 'keyframes:01M6585TXWMGFWKDKB0CFC7GFR', '{"mediaFileId":"01M6585TXWMGFWKDKB0CFC7GFR"}', 'done', 1793325199761, NULL, NULL, 1, 5, NULL, 1793325199761, 1793325200939);
INSERT INTO "jobs" VALUES (98, 'probe.keyframes', 1, 'keyframes:01M6585TY7APW6R5BQ43BJ4Q8V', '{"mediaFileId":"01M6585TY7APW6R5BQ43BJ4Q8V"}', 'done', 1793325199791, NULL, NULL, 1, 5, NULL, 1793325199791, 1793325200941);
INSERT INTO "jobs" VALUES (99, 'probe.keyframes', 1, 'keyframes:01M6585TY5S60SV3DEYXS58QYR', '{"mediaFileId":"01M6585TY5S60SV3DEYXS58QYR"}', 'done', 1793325199791, NULL, NULL, 1, 5, NULL, 1793325199791, 1793325200961);
INSERT INTO "jobs" VALUES (100, 'probe.keyframes', 1, 'keyframes:01M6585TY0V149WSKVTXVXAPXZ', '{"mediaFileId":"01M6585TY0V149WSKVTXVXAPXZ"}', 'done', 1793325199801, NULL, NULL, 1, 5, NULL, 1793325199801, 1793325200954);
INSERT INTO "jobs" VALUES (101, 'probe.keyframes', 1, 'keyframes:01M6585TY2F5QXFA1ZTJ4V4H7A', '{"mediaFileId":"01M6585TY2F5QXFA1ZTJ4V4H7A"}', 'done', 1793325199827, NULL, NULL, 1, 5, NULL, 1793325199827, 1793325200970);
INSERT INTO "jobs" VALUES (102, 'probe.keyframes', 1, 'keyframes:01M6585TYCYRGFPP41J7FQEEQD', '{"mediaFileId":"01M6585TYCYRGFPP41J7FQEEQD"}', 'done', 1793325199831, NULL, NULL, 1, 5, NULL, 1793325199831, 1793325200971);
INSERT INTO "jobs" VALUES (103, 'probe.keyframes', 1, 'keyframes:01M6585TY97A2XBTT5DMSCH37V', '{"mediaFileId":"01M6585TY97A2XBTT5DMSCH37V"}', 'done', 1793325199831, NULL, NULL, 1, 5, NULL, 1793325199831, 1793325201007);
INSERT INTO "jobs" VALUES (104, 'probe.keyframes', 1, 'keyframes:01M6585TYBM1BK4M91ZZRZCBWN', '{"mediaFileId":"01M6585TYBM1BK4M91ZZRZCBWN"}', 'done', 1793325199833, NULL, NULL, 1, 5, NULL, 1793325199833, 1793325200999);
INSERT INTO "jobs" VALUES (105, 'probe.keyframes', 1, 'keyframes:01M6585TYD2X2X729XG9DEMSKF', '{"mediaFileId":"01M6585TYD2X2X729XG9DEMSKF"}', 'done', 1793325199859, NULL, NULL, 1, 5, NULL, 1793325199859, 1793325200999);
INSERT INTO "jobs" VALUES (106, 'probe.keyframes', 1, 'keyframes:01M6585TYE1RWHX6C8CTJQM6YM', '{"mediaFileId":"01M6585TYE1RWHX6C8CTJQM6YM"}', 'done', 1793325199860, NULL, NULL, 1, 5, NULL, 1793325199860, 1793325200999);
INSERT INTO "jobs" VALUES (107, 'probe.keyframes', 1, 'keyframes:01M6585TYG50HT3BSC3K6GPE80', '{"mediaFileId":"01M6585TYG50HT3BSC3K6GPE80"}', 'done', 1793325199863, NULL, NULL, 1, 5, NULL, 1793325199863, 1793325201029);
INSERT INTO "jobs" VALUES (108, 'probe.keyframes', 1, 'keyframes:01M6585TYF4HEW4J1E4N59SE9N', '{"mediaFileId":"01M6585TYF4HEW4J1E4N59SE9N"}', 'done', 1793325199864, NULL, NULL, 1, 5, NULL, 1793325199864, 1793325201028);
INSERT INTO "jobs" VALUES (109, 'probe.keyframes', 1, 'keyframes:01M6585TYJ68J0E2CS58PM7T3N', '{"mediaFileId":"01M6585TYJ68J0E2CS58PM7T3N"}', 'done', 1793325199893, NULL, NULL, 1, 5, NULL, 1793325199893, 1793325201029);
INSERT INTO "jobs" VALUES (110, 'probe.keyframes', 1, 'keyframes:01M6585TYHS7QMZSG7AS0VRQAK', '{"mediaFileId":"01M6585TYHS7QMZSG7AS0VRQAK"}', 'done', 1793325199900, NULL, NULL, 1, 5, NULL, 1793325199900, 1793325201037);
INSERT INTO "jobs" VALUES (111, 'probe.keyframes', 1, 'keyframes:01M6585TYHS7QMZSG7AVR7ZZ5F', '{"mediaFileId":"01M6585TYHS7QMZSG7AVR7ZZ5F"}', 'done', 1793325199903, NULL, NULL, 1, 5, NULL, 1793325199903, 1793325201060);
INSERT INTO "jobs" VALUES (112, 'update.check', 1, 'update.check', '{}', 'done', 1793325257213, NULL, NULL, 1, 5, NULL, 1793325257213, 1793325258575);
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
INSERT INTO "settings" VALUES ('update_state', '{"latestVersion":"1.1.0","url":"https://github.com/SlouchCat/slouchcat/releases/tag/v1.1.0","notes":"Spotlight, Kuratierung von Hand und der Ordner-Browser.\n\nStartseite\n\n- Neue Reihe «Spotlight»: eine öffentliche Trakt-Liste. Welche, steht in der Verwaltung unter\n  Kuratierung → Spotlight — «Trending bei Trakt», «Beliebt bei Trakt», «Meistgesehen»,\n  «Meistgesammelt», «Am meisten erwartet», «Kinocharts» oder die Adresse einer öffentlichen\n  Liste. Gezeigt werden nur Titel aus der eigenen Bibliothek, und jedes Profil sieht nur, was\n  seine Altersstufe erlaubt. Einzelne Einträge lassen sich ausblenden, an einen Platz heften\n  und mit einer eigenen Notiz versehen; diese Änderungen überleben den nächsten Abruf.\n- Neue Reihe «Kürzlich veröffentlicht»: Titel, die in den letzten zwölf Monaten erschienen\n  sind. Bei Serien zählt die neueste Folge, die das Profil sehen darf.\n- Die Reihenfolge der Reihen und welche sichtbar sind, stellst du unter Kuratierung →\n  Startseite ein. Die oberste Reihe wird zur Bühne: gerahmtes Hintergrundbild mit Posterleiste\n  darunter. Ohne Bühne beginnt die Seite direkt mit den Posterzeilen.\n- Neben «Abspielen» und «Details» steht auf der Bühne ein Knopf «Trailer».\n- «Cat''s Empfehlungen» bleibt von Hand gepflegt und zeigt in der Verwaltung jetzt Poster und\n  Plätze.\n- Die beiden festen Reihen «Trending bei Trakt» und «Beliebt bei Trakt» entfallen. Spotlight\n  deckt sie ab und kann mehr.\n- Die Startseite bleibt in beiden Formen unter der gemessenen Grenze von 2.5 Sekunden bis zum\n  grössten Bild.\n\nBibliotheken\n\n- Eine Bibliothek darf mehrere Ordner haben.\n- Im Bibliotheksformular blättert ein Ordner-Browser durch die Ordner des Servers. Welche er\n  zeigen darf, sagt SLOUCHCAT_BROWSE_ROOTS; ohne Eintrag gilt /media.\n\nBehoben\n\n- Die Kennung des Geräts stand bei einem abgebrochenen Abruf im Klartext im Protokoll. Sie wird\n  jetzt geschwärzt wie die übrigen Geheimnisse.\n\nFür eigene Programme am API\n\nDie Vertragsfassung steigt auf 1.1.0, und sie ist brechend: HomeRowKey verliert die Werte\ntrakt_trending und trakt_popular und bekommt spotlight und released. Ein fremdes Programm, das\ndie alten Werte kennt, muss angepasst werden; entfernte Werte werden nie wiederverwendet. Neu\nist das Feld HomePage.heroRow, das nennt, welche Reihe die Bühne ist.\n\nAktualisieren\n\nSLOUCHCAT_VERSION in .env auf 1.1.0 setzen, dann «docker compose pull \u0026\u0026 docker compose up -d».\nDer Server sichert die Datenbank beim Start selbst und wandelt sie um (Migration 00021). Die\nEinträge der alten Reihe «Trending bei Trakt» wandern dabei nach Spotlight; ein «Jetzt abrufen»\nunter Kuratierung → Spotlight holt die aktuelle Liste.\n\nInstallation: docs/INSTALLATION.md.","checkedAt":1793325258135}', 1793325258135);
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
INSERT INTO "profiles" VALUES (1, 1, 'frozen', NULL, 18, 'de', NULL, 1, 1, 1793325197706, 1793325197706, NULL, NULL, NULL, NULL, NULL, NULL, NULL);
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
, content TEXT NOT NULL DEFAULT 'movie'
    CHECK (content IN ('movie', 'series', 'audiobook', 'podcast')), providers TEXT NOT NULL DEFAULT '["tmdb"]'
    CHECK (json_valid(providers) AND json_array_length(providers) >= 1)) STRICT;
INSERT INTO "library_roots" VALUES (1, 1, '/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/.claude/worktrees/startseite/testdata/media/Movies', 'ext4', 'inotify', 1793325197707, 'movie', '["tmdb"]');
INSERT INTO "library_roots" VALUES (2, 2, '/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/.claude/worktrees/startseite/testdata/media/TV', 'ext4', 'inotify', 1793325197763, 'series', '["tmdb","tvdb"]');
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
    CHECK (trailer_state IN ('pending', 'found', 'none', 'failed')), trailer_checked_at INTEGER, segments_state TEXT NOT NULL DEFAULT 'pending' CHECK (segments_state IN ('pending', 'done', 'none', 'failed')), segments_checked_at INTEGER, author TEXT, narrator TEXT, series_name TEXT, series_index REAL, explicit INTEGER NOT NULL DEFAULT 0 CHECK (explicit IN (0, 1)), duration_ms INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0), cover_url TEXT, cover_path TEXT, feed_url TEXT, feed_etag TEXT, feed_modified TEXT, feed_checked_at INTEGER, feed_error TEXT, guid TEXT, published_at INTEGER, enclosure_type TEXT, enclosure_length INTEGER CHECK (enclosure_length IS NULL OR enclosure_length >= 0), providers TEXT NOT NULL DEFAULT '["tmdb"]' CHECK (json_valid(providers)), anilist_id INTEGER, feed_origin TEXT CHECK (feed_origin IS NULL OR feed_origin IN ('admin', 'file')), filled_providers TEXT
    CHECK (filled_providers IS NULL OR json_valid(filled_providers)), vote_average REAL
    CHECK (vote_average IS NULL OR (vote_average >= 0 AND vote_average <= 10))) STRICT;
INSERT INTO "items" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'series', NULL, 2, 'Slouchcat Test Download Show', 'slouchcat test download show', 'Transamerican Love Story', 2008, '2008-02-11', 60, '{"de":[],"en":[]}', 7101, 'tt1152813', NULL, NULL, 18, 1, NULL, NULL, '[]', 'matched', NULL, 1793325198558, 1793325198231, 1793325198231, 1793325200699, NULL, NULL, NULL, NULL, NULL, NULL, '/qADLlBrxdy9dhf5jKhPd1reQwua.jpg', '/tAgoJsvCxhodWFNH0Cpcmobv7e.jpg', 'none', 1793325200699, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, '["tmdb","tvdb"]', 1);
INSERT INTO "items" VALUES ('01M6585TWSZF0B4KYY554V0591', 'season', '01M6585TWQDRBNT5QQW1HXW2JH', 2, 'Season 1', 'season 0001', NULL, NULL, '2008-02-11', NULL, '[]', NULL, NULL, NULL, NULL, 18, 1, NULL, NULL, '[]', 'matched', NULL, 1793325198781, 1793325198231, 1793325198231, 1793325200289, NULL, 1, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'none', 1793325200289, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TWSZF0B4KYY554YCSGP', 'episode', '01M6585TWSZF0B4KYY554V0591', 2, 'Episode 1', '0001 0001', NULL, NULL, '2008-02-11', 60, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'matched', NULL, 1793325198781, 1793325198231, 1793325198231, 1793325198781, NULL, 1, 1, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TWVGG193PDQ4EA1YP5W', 'episode', '01M6585TWSZF0B4KYY554V0591', 2, 'Episode 2', '0001 0002', NULL, NULL, '2008-02-18', 60, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'matched', NULL, 1793325198781, 1793325198231, 1793325198231, 1793325198781, NULL, 1, 2, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TWY3XENWFZFRW7HS18X', 'episode', '01M6585TWSZF0B4KYY554V0591', 2, 'Episode 3', '0001 0003', NULL, NULL, '2008-02-25', 60, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'matched', NULL, 1793325198781, 1793325198231, 1793325198231, 1793325198781, NULL, 1, 3, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TX1SPDKZ0TMGPY8T2PN', 'episode', '01M6585TWSZF0B4KYY554V0591', 2, 'Episode 4', '0001 0004', NULL, NULL, '2008-03-03', 60, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'matched', NULL, 1793325198781, 1793325198231, 1793325198231, 1793325198781, NULL, 1, 4, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TX3SNZ18WSVMA3XB3DP', 'episode', '01M6585TWSZF0B4KYY554V0591', 2, 'Episode 5', '0001 0005', NULL, NULL, '2008-03-10', 60, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'matched', NULL, 1793325198781, 1793325198231, 1793325198231, 1793325198781, NULL, 1, 5, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TX5TZF3VF0Q88721J6C', 'episode', '01M6585TWSZF0B4KYY554V0591', 2, 'Episode 6', '0001 0006', NULL, NULL, '2008-03-17', 60, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'matched', NULL, 1793325198781, 1793325198231, 1793325198231, 1793325198781, NULL, 1, 6, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TX6FV1CMGBDPS84K004', 'series', NULL, 2, 'Slouchcat Test Intro Show', 'slouchcat test intro show', 'Where Are They Now?', 2024, NULL, NULL, '{"de":[],"en":[]}', 7102, NULL, NULL, NULL, 18, 1, NULL, NULL, '[]', 'matched', NULL, 1793325198567, 1793325198231, 1793325198231, 1793325200676, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'none', 1793325200676, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, '["tmdb","tvdb"]', NULL);
INSERT INTO "items" VALUES ('01M6585TX7TT52TQKRKR9G5KMG', 'season', '01M6585TX6FV1CMGBDPS84K004', 2, 'Season 1', 'season 0001', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, 18, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198231, 1793325198231, 1793325200741, NULL, 1, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'done', 1793325200741, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TX877H28GBN26XWG310', 'episode', '01M6585TX7TT52TQKRKR9G5KMG', 2, 'Episode 1', '0001 0001', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199976, 1793325198231, 1793325198231, 1793325199976, NULL, 1, 1, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TX9NYJJJ3RSY1ARMMC0', 'episode', '01M6585TX7TT52TQKRKR9G5KMG', 2, 'Episode 2', '0001 0002', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325200004, 1793325198231, 1793325198231, 1793325200004, NULL, 1, 2, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXANBCG17M508AY0707', 'episode', '01M6585TX7TT52TQKRKR9G5KMG', 2, 'Episode 3', '0001 0003', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325200010, 1793325198231, 1793325198231, 1793325200010, NULL, 1, 3, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXCHS51027J79733F3J', 'episode', '01M6585TX7TT52TQKRKR9G5KMG', 2, 'Episode 4', '0001 0004', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325200455, 1793325198231, 1793325198231, 1793325200455, NULL, 1, 4, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXDAXTMQ0X78MGGC0BC', 'episode', '01M6585TX7TT52TQKRKR9G5KMG', 2, 'Episode 5', '0001 0005', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325200086, 1793325198231, 1793325198231, 1793325200086, NULL, 1, 5, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXFV7FMJPYSCG96AQRK', 'series', NULL, 2, 'Slouchcat Test Show', 'slouchcat test show', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, 18, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325198956, 1793325198231, 1793325198231, 1793325198956, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXFV7FMJPYSCH66A7DW', 'season', '01M6585TXFV7FMJPYSCG96AQRK', 2, 'Season 1', 'season 0001', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, 18, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198231, 1793325198231, 1793325200475, NULL, 1, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'none', 1793325200475, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXFV7FMJPYSCJTY4ZRP', 'episode', '01M6585TXFV7FMJPYSCH66A7DW', 2, 'Episode 1', '0001 0001', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198231, 1793325198231, 1793325198231, NULL, 1, 1, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXGEC9NVX13PDDVTPJG', 'episode', '01M6585TXFV7FMJPYSCH66A7DW', 2, 'Episode 2', '0001 0002', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198231, 1793325198231, 1793325198231, NULL, 1, 2, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXHVBT2ZBQVW1GFPND5', 'episode', '01M6585TXFV7FMJPYSCH66A7DW', 2, 'Episode 3', '0001 0003', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198231, 1793325198231, 1793325198231, NULL, 1, 3, 4, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXHVBT2ZBQVW2FR1WFX', 'season', '01M6585TXFV7FMJPYSCG96AQRK', 2, 'Specials', 'specials', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, 18, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198231, 1793325198231, 1793325199685, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXJHQ022AZ82RZ7TT93', 'episode', '01M6585TXHVBT2ZBQVW2FR1WFX', 2, 'Episode 1', '0000 0001', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198231, 1793325198231, 1793325198231, NULL, 0, 1, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb","tvdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXK7NNK68XFTPTBESVF', 'movie', NULL, 1, 'Slouchcat Test AC3 51', 'slouchcat test ac3 51', NULL, 2024, NULL, NULL, '[]', 7001, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325198900, 1793325198232, 1793325198232, 1793325198900, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXM70G9GSBN41B2G7D0', 'movie', NULL, 1, 'Slouchcat Test Broken', 'slouchcat test broken', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199017, 1793325198232, 1793325198232, 1793325199017, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXNTJS6VDVEE4JJQ3P1', 'movie', NULL, 1, 'Slouchcat Test Chapters', 'slouchcat test chapters', NULL, 2024, NULL, NULL, '[]', 7004, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199069, 1793325198232, 1793325198232, 1793325199069, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXPNMFEW1D01E2HFA8X', 'movie', NULL, 1, 'Slouchcat Test DTS', 'slouchcat test dts', NULL, 2024, NULL, NULL, '[]', 7002, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199131, 1793325198232, 1793325198232, 1793325199131, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXRHCKT35BV8XKA9H60', 'movie', NULL, 1, 'Slouchcat Test Extras', 'slouchcat test extras', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199184, 1793325198232, 1793325198232, 1793325199184, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXSN46TF04KKM2FK30H', 'movie', NULL, 1, 'Slouchcat Test H264', 'slouchcat test h264', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199261, 1793325198232, 1793325198232, 1793325199261, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXV85XJJ0DS5Z7Y68VM', 'movie', NULL, 1, 'Slouchcat Test H264 Copy', 'slouchcat test h264 copy', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199302, 1793325198232, 1793325198232, 1793325199302, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXXSRR48VSW6EVTC46P', 'movie', NULL, 1, 'Slouchcat Test H264 MKV Long', 'slouchcat test h264 mkv long', NULL, 2024, NULL, NULL, '[]', 7005, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199336, 1793325198232, 1793325198232, 1793325199336, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TXZ9E4QBE7KXJ408K87', 'movie', NULL, 1, 'Slouchcat Test HDR10 4K', 'slouchcat test hdr10 4k', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199344, 1793325198232, 1793325198232, 1793325199344, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TY1F43AEP4WPVG98FXG', 'movie', NULL, 1, 'Slouchcat Test HDR10 Chart', 'slouchcat test hdr10 chart', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199380, 1793325198232, 1793325198232, 1793325199380, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TY3CMXZFGPM4XVA6K63', 'movie', NULL, 1, 'Slouchcat Test HEVC 10bit', 'slouchcat test hevc 10bit', NULL, 2024, NULL, NULL, '[]', 7003, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199412, 1793325198232, 1793325198232, 1793325199412, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TY6Z709NFGCE902YJD9', 'movie', NULL, 1, 'Slouchcat Test HEVC Long', 'slouchcat test hevc long', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199454, 1793325198232, 1793325198232, 1793325199454, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TY82HPB7DNYVEFJ57ZE', 'movie', NULL, 1, 'Slouchcat Test Multi Audio', 'slouchcat test multi audio', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199458, 1793325198232, 1793325198232, 1793325199458, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TYAPAP726NKRT2YB2Q5', 'movie', NULL, 1, 'Slouchcat Test PGS', 'slouchcat test pgs', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199495, 1793325198232, 1793325198232, 1793325199495, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TYBM1BK4M9202GET26Y', 'movie', NULL, 1, 'Slouchcat Test Sidecar Subtitles', 'slouchcat test sidecar subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199530, 1793325198232, 1793325198232, 1793325199530, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TYCYRGFPP41JB1ABAY4', 'movie', NULL, 1, 'Slouchcat Test Subtitles', 'slouchcat test subtitles', NULL, 2024, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199569, 1793325198232, 1793325198232, 1793325199569, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TYE1RWHX6C8CR9YJD94', 'movie', NULL, 1, 'Slouchcat Test Unrated', 'slouchcat test unrated', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'unmatched', NULL, 1793325199566, 1793325198232, 1793325198232, 1793325199566, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TYF4HEW4J1E4JZE5V40', 'extra', '01M6585TXRHCKT35BV8XKA9H60', 1, 'On Set', 'on set', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198232, 1793325198232, 1793325198232, NULL, NULL, NULL, NULL, NULL, 'behind_the_scenes', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TYF4HEW4J1E4S0F89G2', 'extra', '01M6585TXRHCKT35BV8XKA9H60', 1, 'Scene 1', 'scene 1', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198232, 1793325198232, 1793325198232, NULL, NULL, NULL, NULL, NULL, 'deleted_scene', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TYG50HT3BSC3N918J8G', 'extra', '01M6585TXRHCKT35BV8XKA9H60', 1, 'Making Of', 'making of', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198232, 1793325198232, 1793325198232, NULL, NULL, NULL, NULL, NULL, 'featurette', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TYHS7QMZSG7AV4RMH2Q', 'extra', '01M6585TXRHCKT35BV8XKA9H60', 1, 'Slouchcat Test Extras (2024)', 'slouchcat test extras (2024)', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198232, 1793325198232, 1793325198232, NULL, NULL, NULL, NULL, NULL, 'trailer', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
INSERT INTO "items" VALUES ('01M6585TYJ68J0E2CS58DN0GGF', 'extra', '01M6585TXRHCKT35BV8XKA9H60', 1, 'Slouchcat Test Extras Trailer', 'slouchcat test extras trailer', NULL, NULL, NULL, NULL, '[]', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '[]', 'pending', NULL, NULL, 1793325198232, 1793325198232, 1793325198232, NULL, NULL, NULL, NULL, NULL, 'trailer', NULL, NULL, 'pending', NULL, 'pending', NULL, NULL, NULL, NULL, NULL, 0, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '["tmdb"]', NULL, NULL, NULL, NULL);
CREATE TABLE item_texts (
    item_id  TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    lang     TEXT NOT NULL CHECK (lang IN ('de', 'en')),
    title    TEXT NOT NULL,
    overview TEXT,
    tagline  TEXT, sort_key TEXT,
    PRIMARY KEY (item_id, lang)
) STRICT, WITHOUT ROWID;
INSERT INTO "item_texts" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'de', 'Transamerican Love Story', 'Transamerican Love Story is an American reality television program in which suitors wooed transgender woman Calpernia Addams. Addams chose a suitor by process of elimination.

Logo, a digital cable channel with LGBT content, originally broadcast the series in 2008 from February through March. Alec Mapa hosted each episode.

Zach Rosen of the Washington Blade praised the series, describing it as "refreshing for its lack of sensationalism".', NULL, 'transamerican love story');
INSERT INTO "item_texts" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'en', 'Transamerican Love Story', 'Transamerican Love Story is an American reality television program in which suitors wooed transgender woman Calpernia Addams. Addams chose a suitor by process of elimination.

Logo, a digital cable channel with LGBT content, originally broadcast the series in 2008 from February through March. Alec Mapa hosted each episode.

Zach Rosen of the Washington Blade praised the series, describing it as "refreshing for its lack of sensationalism".', NULL, 'transamerican love story');
INSERT INTO "item_texts" VALUES ('01M6585TWSZF0B4KYY554V0591', 'de', 'Staffel 1', NULL, NULL, 'staffel 1');
INSERT INTO "item_texts" VALUES ('01M6585TWSZF0B4KYY554V0591', 'en', 'Season 1', NULL, NULL, 'season 1');
INSERT INTO "item_texts" VALUES ('01M6585TWSZF0B4KYY554YCSGP', 'de', 'Folge 1', 'Calpernia meets the eight bachelors.', NULL, 'folge 1');
INSERT INTO "item_texts" VALUES ('01M6585TWSZF0B4KYY554YCSGP', 'en', 'Getting to Know You', 'Calpernia meets the eight bachelors.', NULL, 'getting to know you');
INSERT INTO "item_texts" VALUES ('01M6585TWVGG193PDQ4EA1YP5W', 'de', 'Folge 2', 'Calpernia''s friends are invited to a Southern dinner which the bachelors must prepare.', NULL, 'folge 2');
INSERT INTO "item_texts" VALUES ('01M6585TWVGG193PDQ4EA1YP5W', 'en', 'Something''s Cookin''', 'Calpernia''s friends are invited to a Southern dinner which the bachelors must prepare.', NULL, 'something''s cookin''');
INSERT INTO "item_texts" VALUES ('01M6585TWY3XENWFZFRW7HS18X', 'de', 'Folge 3', 'The six remaining suitors must paddle a rubber raft to a yacht in order to win time alone with Calpernia.', NULL, 'folge 3');
INSERT INTO "item_texts" VALUES ('01M6585TWY3XENWFZFRW7HS18X', 'en', 'Love''s a Beach', 'The six remaining suitors must paddle a rubber raft to a yacht in order to win time alone with Calpernia.', NULL, 'love''s a beach');
INSERT INTO "item_texts" VALUES ('01M6585TX1SPDKZ0TMGPY8T2PN', 'de', 'Folge 4', 'Calpernia brings the men to a nightclub where several of her friends evaluate them. Calpernia performs her song "Stunning", and the men learn they will perform on stage and in costume for her and a panel of judges.', NULL, 'folge 4');
INSERT INTO "item_texts" VALUES ('01M6585TX1SPDKZ0TMGPY8T2PN', 'en', 'I Love the Nightlife', 'Calpernia brings the men to a nightclub where several of her friends evaluate them. Calpernia performs her song "Stunning", and the men learn they will perform on stage and in costume for her and a panel of judges.', NULL, 'i love the nightlife');
INSERT INTO "item_texts" VALUES ('01M6585TX3SNZ18WSVMA3XB3DP', 'de', 'Folge 5', 'Everyone goes to Oil Can Harry''s to learn to dance the country-western two-step.', NULL, 'folge 5');
INSERT INTO "item_texts" VALUES ('01M6585TX3SNZ18WSVMA3XB3DP', 'en', 'I Like The Way You Move', 'Everyone goes to Oil Can Harry''s to learn to dance the country-western two-step.', NULL, 'i like the way you move');
INSERT INTO "item_texts" VALUES ('01M6585TX5TZF3VF0Q88721J6C', 'de', 'Folge 6', 'With Shawn absent for a funeral, only Jim and Mike are left to meet Calpernia''s ''adopted mother'', Chyna.', NULL, 'folge 6');
INSERT INTO "item_texts" VALUES ('01M6585TX5TZF3VF0Q88721J6C', 'en', 'Meet The Folks', 'With Shawn absent for a funeral, only Jim and Mike are left to meet Calpernia''s ''adopted mother'', Chyna.', NULL, 'meet the folks');
INSERT INTO "item_texts" VALUES ('01M6585TX6FV1CMGBDPS84K004', 'de', 'Where Are They Now?', 'Where Are They Now? is a show hosted by Lydia Shum in 2006, and was shown on TVB in 2006, as well as on Fairchild TV in Canada in 2007 and 2008. Throughout the 32 episodes, Lydia Shum travelled to Taiwan, Japan, USA, and Canada to interview popular celebrities of the 1960s in Hong Kong and Taiwan. Most of these celebrities have now emigrated to foreign countries.', NULL, 'where are they now?');
INSERT INTO "item_texts" VALUES ('01M6585TX6FV1CMGBDPS84K004', 'en', 'Where Are They Now?', 'Where Are They Now? is a show hosted by Lydia Shum in 2006, and was shown on TVB in 2006, as well as on Fairchild TV in Canada in 2007 and 2008. Throughout the 32 episodes, Lydia Shum travelled to Taiwan, Japan, USA, and Canada to interview popular celebrities of the 1960s in Hong Kong and Taiwan. Most of these celebrities have now emigrated to foreign countries.', NULL, 'where are they now?');
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
INSERT INTO "media_files" VALUES ('01M6585TWT1YNE1CH3TZ3Q9HRF', '01M6585TWSZF0B4KYY554YCSGP', 2, 'Slouchcat Test Download Show (2024) {tmdb-7101}/Season 01/Slouchcat Test Download Show (2024) {tmdb-7101} S01E01.mp4', 66310, 11799768, 1243717, 1793316242869, '16dab3d9f17d65e2b7abcad2b590aea6b76455c6760284ed75dcf69969ed57df', NULL, 'mp4', 'h264', 'aac', 1280, 720, 8000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":8000000000,"size":1243717,"bitRate":1243717,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","episode_sort":"1","major_brand":"isom","minor_version":"512","season_number":"1","show":"Slouchcat Test Download Show","title":"Episode 1"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00f', 1793325199542, 1793325200707, NULL, 1793325198231, 1793325200707, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TWXFMDSEPE5ANZ5WTCB', '01M6585TWVGG193PDQ4EA1YP5W', 2, 'Slouchcat Test Download Show (2024) {tmdb-7101}/Season 01/Slouchcat Test Download Show (2024) {tmdb-7101} S01E02.mp4', 66310, 11799770, 1243698, 1793316243137, '22ca0c79783e95083a82d8fd40f9947d4909b462c3dd02f64f74dced623b795e', NULL, 'mp4', 'h264', 'aac', 1280, 720, 8000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":8000000000,"size":1243698,"bitRate":1243698,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","episode_sort":"2","major_brand":"isom","minor_version":"512","season_number":"1","show":"Slouchcat Test Download Show","title":"Episode 2"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00f', 1793325199561, 1793325200726, NULL, 1793325198231, 1793325200726, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TX0A6KKFTTK4AM5BRK8', '01M6585TWY3XENWFZFRW7HS18X', 2, 'Slouchcat Test Download Show (2024) {tmdb-7101}/Season 01/Slouchcat Test Download Show (2024) {tmdb-7101} S01E03.mp4', 66310, 11799771, 1243693, 1793316243414, '4eb823d78ad45cbd0db384f6d41ba3eb07e74517b6ea084200224767fe8bbe5f', NULL, 'mp4', 'h264', 'aac', 1280, 720, 8000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":8000000000,"size":1243693,"bitRate":1243693,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","episode_sort":"3","major_brand":"isom","minor_version":"512","season_number":"1","show":"Slouchcat Test Download Show","title":"Episode 3"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00f', 1793325199575, 1793325200736, NULL, 1793325198231, 1793325200736, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TX2N7TACN188D77429J', '01M6585TX1SPDKZ0TMGPY8T2PN', 2, 'Slouchcat Test Download Show (2024) {tmdb-7101}/Season 01/Slouchcat Test Download Show (2024) {tmdb-7101} S01E04.mp4', 66310, 11799772, 1243729, 1793316243664, '24a1188fbbe427aadd3e4b80a57e379902989bb409a576a3b37a4b7dafe26b56', NULL, 'mp4', 'h264', 'aac', 1280, 720, 8000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":8000000000,"size":1243729,"bitRate":1243729,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","episode_sort":"4","major_brand":"isom","minor_version":"512","season_number":"1","show":"Slouchcat Test Download Show","title":"Episode 4"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00f', 1793325199591, 1793325200764, NULL, 1793325198231, 1793325200764, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TX4J3HDJ459C70A729B', '01M6585TX3SNZ18WSVMA3XB3DP', 2, 'Slouchcat Test Download Show (2024) {tmdb-7101}/Season 01/Slouchcat Test Download Show (2024) {tmdb-7101} S01E05.mp4', 66310, 11799773, 1243726, 1793316243935, '60c35fd090f2f2c3727b4136f69ecb06e4a24214b2bdd488b94c17575fd53da6', NULL, 'mp4', 'h264', 'aac', 1280, 720, 8000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":8000000000,"size":1243726,"bitRate":1243726,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","episode_sort":"5","major_brand":"isom","minor_version":"512","season_number":"1","show":"Slouchcat Test Download Show","title":"Episode 5"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00f', 1793325199593, 1793325200763, NULL, 1793325198231, 1793325200763, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TX6FV1CMGBDPNKQFBZ9', '01M6585TX5TZF3VF0Q88721J6C', 2, 'Slouchcat Test Download Show (2024) {tmdb-7101}/Season 01/Slouchcat Test Download Show (2024) {tmdb-7101} S01E06.mp4', 66310, 11799774, 1243702, 1793316244204, 'a9eaef655561db50088bc8cf046e86447ceebef4b55fb4edbbe54ba7969fc108', NULL, 'mp4', 'h264', 'aac', 1280, 720, 8000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":8000000000,"size":1243702,"bitRate":1243702,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","episode_sort":"6","major_brand":"isom","minor_version":"512","season_number":"1","show":"Slouchcat Test Download Show","title":"Episode 6"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00f', 1793325199597, 1793325200771, NULL, 1793325198231, 1793325200771, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TX9NYJJJ3RSXYYKAVP4', '01M6585TX877H28GBN26XWG310', 2, 'Slouchcat Test Intro Show (2024) {tmdb-7102}/Season 01/Slouchcat Test Intro Show (2024) {tmdb-7102} - S01E01.mkv', 66310, 11799761, 6012073, 1793316236588, '2527a341e35e0693086d3d7d86dd2e3e95783bfdbeb87672b6071c846841a91e', NULL, 'mkv', 'h264', 'aac', 640, 360, 98021, '{"format":{"name":"matroska,webm","duration":98021000000,"size":6012073,"bitRate":490676,"tags":{"encoder":"Lavf60.16.100","title":"Episode 1"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":30,"width":640,"height":360,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793325199604, 1793325200799, NULL, 1793325198231, 1793325200799, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXANBCG17M505BH3Q06', '01M6585TX9NYJJJ3RSY1ARMMC0', 2, 'Slouchcat Test Intro Show (2024) {tmdb-7102}/Season 01/Slouchcat Test Intro Show (2024) {tmdb-7102} - S01E02.mkv', 66310, 11799762, 6144553, 1793316238058, '27a7f3033f61f475ccb91f1eea90730e2d58d3cf2ae0bdce319ee9ee8fc80dcc', NULL, 'mkv', 'h264', 'aac', 640, 360, 100021, '{"format":{"name":"matroska,webm","duration":100021000000,"size":6144553,"bitRate":491461,"tags":{"encoder":"Lavf60.16.100","title":"Episode 2"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":30,"width":640,"height":360,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793325199617, 1793325200796, NULL, 1793325198231, 1793325200796, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXBMVWKFXBHJXPXMAV8', '01M6585TXANBCG17M508AY0707', 2, 'Slouchcat Test Intro Show (2024) {tmdb-7102}/Season 01/Slouchcat Test Intro Show (2024) {tmdb-7102} - S01E03.mkv', 66310, 11799763, 6262595, 1793316239509, 'a12f5504bb77fdf82776111b0a0cf62dc6e909c6deaa50bc7869db0ea48a0c4a', NULL, 'mkv', 'h264', 'aac', 640, 360, 102021, '{"format":{"name":"matroska,webm","duration":102021000000,"size":6262595,"bitRate":491082,"tags":{"encoder":"Lavf60.16.100","title":"Episode 3"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":30,"width":640,"height":360,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[{"start":0,"end":8000000000,"title":"Prolog"},{"start":8000000000,"end":28000000000,"title":"Intro"},{"start":28000000000,"end":77000000000,"title":"Handlung"},{"start":77000000000,"end":102000000000,"title":"Credits"}]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793325199618, 1793325200803, NULL, 1793325198231, 1793325200803, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXCHS51027J7B63SVZ6', '01M6585TXCHS51027J79733F3J', 2, 'Slouchcat Test Intro Show (2024) {tmdb-7102}/Season 01/Slouchcat Test Intro Show (2024) {tmdb-7102} - S01E04.mkv', 66310, 11799764, 6395238, 1793316241036, '94108121ed20d7e699b2f89c7174b8cc13856575e40ef5814b42a16afb6cf465', NULL, 'mkv', 'h264', 'aac', 640, 360, 104021, '{"format":{"name":"matroska,webm","duration":104021000000,"size":6395238,"bitRate":491842,"tags":{"encoder":"Lavf60.16.100","title":"Episode 4"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":30,"width":640,"height":360,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793325199622, 1793325200828, NULL, 1793325198231, 1793325200828, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXEKWNC2RXHZ0XN2X6Q', '01M6585TXDAXTMQ0X78MGGC0BC', 2, 'Slouchcat Test Intro Show (2024) {tmdb-7102}/Season 01/Slouchcat Test Intro Show (2024) {tmdb-7102} - S01E05.mkv', 66310, 11799765, 6511608, 1793316242590, 'c9247e97b0c7c278662218d321a277d69747193fdf8fca10ed8d6f4824908f44', NULL, 'mkv', 'h264', 'aac', 640, 360, 106021, '{"format":{"name":"matroska,webm","duration":106021000000,"size":6511608,"bitRate":491344,"tags":{"encoder":"Lavf60.16.100","title":"Episode 5"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":30,"width":640,"height":360,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793325199631, 1793325200836, NULL, 1793325198231, 1793325200836, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXGEC9NVX13PAQ17V84', '01M6585TXFV7FMJPYSCJTY4ZRP', 2, 'Slouchcat Test Show (2024)/Season 01/Slouchcat Test Show (2024) S01E01.mp4', 66310, 11799730, 2625066, 1793316218887, 'e98d821e296951b1d8b2fa0e3bb6c5adca9c31432b4bdea6070349b6d3010100', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2625066,"bitRate":2100052,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","episode_sort":"1","major_brand":"isom","minor_version":"512","season_number":"1","show":"Slouchcat Test Show","title":"Episode 1"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199649, 1793325200854, NULL, 1793325198231, 1793325200854, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXHVBT2ZBQVVXKTTNA3', '01M6585TXGEC9NVX13PDDVTPJG', 2, 'Slouchcat Test Show (2024)/Season 01/Slouchcat Test Show (2024) S01E02.mp4', 66310, 11799731, 192565, 1793316219577, '85375e27fb7f54ea546ed60861a70e6738a2b630cee46f0f72af1ab43137bf09', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":192565,"bitRate":154052,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","episode_sort":"2","major_brand":"isom","minor_version":"512","season_number":"1","show":"Slouchcat Test Show","title":"Episode 2"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199649, 1793325200832, NULL, 1793325198231, 1793325200832, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXHVBT2ZBQVW296NXXY', '01M6585TXHVBT2ZBQVW1GFPND5', 2, 'Slouchcat Test Show (2024)/Season 01/Slouchcat Test Show (2024) S01E03-E04.mp4', 66310, 11799734, 2625085, 1793316221026, 'f39081cd62fdeb5e16b7a5fe891e8258241afdcfe81682c5c8abe7b9a39bca6c', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2625085,"bitRate":2100068,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Episode 3 and 4"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199681, 1793325200863, NULL, 1793325198231, 1793325200863, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXJHQ022AZ82V1BJ4WN', '01M6585TXJHQ022AZ82RZ7TT93', 2, 'Slouchcat Test Show (2024)/Specials/Slouchcat Test Show (2024) S00E01.mp4', 66310, 11799733, 192551, 1793316220247, '3973cfd71cc227a84f7f2f355d36a86c26e415c5787658afe847b60f7e7297fe', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":192551,"bitRate":154040,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Special 1"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199685, 1793325200863, NULL, 1793325198231, 1793325200863, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXM70G9GSBN3Z8T42WQ', '01M6585TXK7NNK68XFTPTBESVF', 1, 'Slouchcat Test AC3 51 (2024) {tmdb-7001}/Slouchcat Test AC3 51 (2024) {tmdb-7001}.mkv', 66310, 11799710, 2943036, 1793316209125, 'b14cac395794250c4ca43703c6ded91894ca4cd262693d361aac75c22b54d0fe', NULL, 'mkv', 'h264', 'ac3', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":2943036,"bitRate":2354428,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test AC3 51"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"ac3","channels":6,"channelLayout":"5.1(side)","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199680, 1793325200861, NULL, 1793325198232, 1793325200861, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXNTJS6VDVEE232TD2D', '01M6585TXM70G9GSBN41B2G7D0', 1, 'Slouchcat Test Broken (2024)/Slouchcat Test Broken (2024).mkv', 66310, 11799748, 57, 1793316224878, '8ed0f89e2cd4d6f84fa244afd8b1a2d46d26aaf54bf6537fc72508bff522218f', NULL, 'mkv', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 1793325199674, NULL, NULL, 1793325198232, 1793325199674, 'ffprobe: exit status 1: Truncating packet of size 12149 to 53
[matroska,webm @ 0x5626ede5d680] EBML header parsing failed
Slouchcat Test Broken (2024).mkv: Invalid data found when processing input', NULL);
INSERT INTO "media_files" VALUES ('01M6585TXPNMFEW1D01BQK1CKQ', '01M6585TXNTJS6VDVEE4JJQ3P1', 1, 'Slouchcat Test Chapters (2024) {tmdb-7004}/Slouchcat Test Chapters (2024) {tmdb-7004}.mkv', 66310, 11799722, 2623898, 1793316212159, '6c56599898202143db5e5d23e9711968218780c8792ea4be51c2e7b8373fce42', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2623898,"bitRate":2094719,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test Chapters"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[{"start":0,"end":3000000000,"title":"Kapitel 1: Anfang"},{"start":3000000000,"end":6500000000,"title":"Kapitel 2: Mitte"},{"start":6500000000,"end":10000000000,"title":"Kapitel 3: Ende"}]}', X'0100d00fd00fd00fd00f', 1793325199713, 1793325200892, NULL, 1793325198232, 1793325200892, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXQPPWG4PVPX5WH98G9', '01M6585TXPNMFEW1D01E2HFA8X', 1, 'Slouchcat Test DTS (2024) {tmdb-7002}/Slouchcat Test DTS (2024) {tmdb-7002}.mkv', 66310, 11799712, 3427132, 1793316209922, '5ba93ff1251d21ea832afaeb2329a16fd55f4e20fb3366b82a0fea6991965868', NULL, 'mkv', 'h264', 'dts', 1920, 1080, 10000, '{"format":{"name":"matroska,webm","duration":10000000000,"size":3427132,"bitRate":2741705,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test DTS"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"dts","profile":"DTS","channels":6,"channelLayout":"5.1(side)","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199712, 1793325200906, NULL, 1793325198232, 1793325200906, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXRHCKT35BV8YYN6X1G', '01M6585TXRHCKT35BV8XKA9H60', 1, 'Slouchcat Test Extras (2024)/Slouchcat Test Extras (2024).mp4', 66310, 11799737, 2624979, 1793316222268, 'b453a8980a2883e31feeb6f382dc29c9d4b7d8f789b0c1f827ac1bb81da1caf5', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624979,"bitRate":2099983,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test Extras"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199710, 1793325200899, NULL, 1793325198232, 1793325200899, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXT49PGDCXHCA7N0SV4', '01M6585TXSN46TF04KKM2FK30H', 1, 'Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024) - 720p.mp4', 66310, 11799735, 904192, 1793316221428, '0186b76306ab4b1c44501e8fd1fa880c817b1aedff7a4d9ac5de3802a5ed11c6', '720p', 'mp4', 'h264', 'aac', 1280, 720, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":904192,"bitRate":723353,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test H264 720p"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199711, 1793325200889, NULL, 1793325198232, 1793325200889, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXT49PGDCXHCBYT3NJJ', '01M6585TXSN46TF04KKM2FK30H', 1, 'Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024).mp4', 66310, 11799704, 2624977, 1793316196914, '1a3559a74acfe685061086a9246f1ad08d3f220ba78aad39e503e56f4400693b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624977,"bitRate":2099981,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test H264"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199761, 1793325200922, NULL, 1793325198232, 1793325200922, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXWMGFWKDKB0CFC7GFR', '01M6585TXV85XJJ0DS5Z7Y68VM', 1, 'Slouchcat Test H264 Copy (2024)/Slouchcat Test H264 Copy (2024).mp4', 66310, 11799750, 2624977, 1793316224881, '1a3559a74acfe685061086a9246f1ad08d3f220ba78aad39e503e56f4400693b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2624977,"bitRate":2099981,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test H264"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199761, 1793325200939, NULL, 1793325198232, 1793325200939, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TXYKMDET3BAZ1E56GYJ', '01M6585TXXSRR48VSW6EVTC46P', 1, 'Slouchcat Test H264 MKV Long (2024) {tmdb-7005}/Slouchcat Test H264 MKV Long (2024) {tmdb-7005}.mkv', 66310, 11799725, 13446607, 1793316218058, 'a968b756ec9b967922823f53f6cff28cc98eaa47ca3882625de2a8860cd066dd', NULL, 'mkv', 'h264', 'aac', 1280, 720, 90021, '{"format":{"name":"matroska,webm","duration":90021000000,"size":13446607,"bitRate":1194975,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test H264 MKV Long"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":31,"width":1280,"height":720,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00f8827e807d836f02ea846e807b06d9875e807b06d', 1793325199745, 1793325200925, NULL, 1793325198232, 1793325200925, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TY0V149WSKVTXVXAPXZ', '01M6585TXZ9E4QBE7KXJ408K87', 1, 'Slouchcat Test HDR10 4K (2024)/Slouchcat Test HDR10 4K (2024).mkv', 66310, 11799708, 761065, 1793316208317, '4824a158e726a20ddb99b96fba2ba41dec4f15871875901a9a4b1fe8de8f29ae', '2160p', 'mkv', 'hevc', 'aac', 3840, 2160, 30021, '{"format":{"name":"matroska,webm","duration":30021000000,"size":761065,"bitRate":202808,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test HDR10 4K"}},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":150,"width":3840,"height":2160,"pixFmt":"yuv420p10le","colorTransfer":"smpte2084","colorPrimaries":"bt2020"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793325199801, 1793325200954, NULL, 1793325198232, 1793325200954, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TY2F5QXFA1ZTJ4V4H7A', '01M6585TY1F43AEP4WPVG98FXG', 1, 'Slouchcat Test HDR10 Chart (2024)/Slouchcat Test HDR10 Chart (2024).mkv', 66310, 11799758, 262639, 1793316234964, 'ba28a881c7887c203b2a316446dfdaaaf08128e3a7610a20f3cb1f515d6f7465', NULL, 'mkv', 'hevc', 'aac', 3840, 2160, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":262639,"bitRate":209670,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test HDR10 Chart"}},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":150,"width":3840,"height":2160,"pixFmt":"yuv420p10le","colorTransfer":"smpte2084","colorPrimaries":"bt2020"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199827, 1793325200970, NULL, 1793325198232, 1793325200970, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TY5S60SV3DEYXS58QYR', '01M6585TY3CMXZFGPM4XVA6K63', 1, 'Slouchcat Test HEVC 10bit (2024) {tmdb-7003}/Slouchcat Test HEVC 10bit (2024) {tmdb-7003}.mkv', 66310, 11799706, 2102531, 1793316197881, '085bf3cd2d6ef3ee370218dc2f2e903cd1cae5ebf575b24cacd0396f468cfd2e', NULL, 'mkv', 'hevc', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2102531,"bitRate":1678499,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test HEVC 10bit"}},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":120,"width":1920,"height":1080,"pixFmt":"yuv420p10le","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199791, 1793325200961, NULL, 1793325198232, 1793325200961, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TY7APW6R5BQ43BJ4Q8V', '01M6585TY6Z709NFGCE902YJD9', 1, 'Slouchcat Test HEVC Long (2024)/Slouchcat Test HEVC Long (2024).mkv', 66310, 11799724, 9588128, 1793316216021, '1581b172248f938b87ac2869782c31069121326ac29f938cd0a7fe2a299bec38', NULL, 'mkv', 'hevc', 'aac', 1280, 720, 90021, '{"format":{"name":"matroska,webm","duration":90021000000,"size":9588128,"bitRate":852079,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test HEVC Long"}},"streams":[{"index":0,"type":"video","codec":"hevc","profile":"Main 10","level":93,"width":1280,"height":720,"pixFmt":"yuv420p10le","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793325199791, 1793325200941, NULL, 1793325198232, 1793325200941, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TY97A2XBTT5DMSCH37V', '01M6585TY82HPB7DNYVEFJ57ZE', 1, 'Slouchcat Test Multi Audio (2024)/Slouchcat Test Multi Audio (2024).mkv', 66310, 11799754, 6263167, 1793316226821, 'd9502c53792e1b1017a1afb8454aec05b41853319170cfcb68ce0a014e195754', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 20021, '{"format":{"name":"matroska,webm","duration":20021000000,"size":6263167,"bitRate":2502639,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test Multi Audio"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"ger","title":"Deutsch (AAC 2.0)","default":true},{"index":2,"type":"audio","codec":"ac3","channels":6,"channelLayout":"5.1(side)","sampleRate":48000,"language":"eng","title":"English (AC3 5.1)"},{"index":3,"type":"subtitle","codec":"subrip","language":"ger","title":"Deutsch (SRT)"},{"index":4,"type":"subtitle","codec":"subrip","language":"eng","title":"English (SRT)"}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793325199831, 1793325201007, NULL, 1793325198232, 1793325201007, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TYBM1BK4M91ZZRZCBWN', '01M6585TYAPAP726NKRT2YB2Q5', 1, 'Slouchcat Test PGS (2024)/Slouchcat Test PGS (2024).mkv', 66310, 11799756, 5309544, 1793316228053, '43428c206728376f4d0b32ebc36be37e241b5071968743da3710b516700eeecc', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 20021, '{"format":{"name":"matroska,webm","duration":20021000000,"size":5309544,"bitRate":2121589,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test PGS"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000},{"index":2,"type":"subtitle","codec":"hdmv_pgs_subtitle","width":1920,"height":1080,"language":"eng","title":"English (PGS)"}],"chapters":[]}', X'0100d00fd00fd00fd00fd00fd00fd00fd00fd00f', 1793325199833, 1793325200999, NULL, 1793325198232, 1793325200999, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TYCYRGFPP41J7FQEEQD', '01M6585TYBM1BK4M9202GET26Y', 1, 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).mkv', 66310, 11799716, 2623761, 1793316211083, '7dffac449bed6b7d72fbebb210adb4d9c364256a762da52055413f5d1f733df9', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10047, '{"format":{"name":"matroska,webm","duration":10047000000,"size":2623761,"bitRate":2089189,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test Sidecar Subtitles"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000}],"chapters":[]}', X'0115d00fd00fd00fd00f', 1793325199831, 1793325200971, NULL, 1793325198232, 1793325200971, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TYD2X2X729XG9DEMSKF', '01M6585TYCYRGFPP41JB1ABAY4', 1, 'Slouchcat Test Subtitles (2024)/Slouchcat Test Subtitles (2024).mkv', 66310, 11799714, 2626565, 1793316210993, 'ada167ccf1f63ec56c7f3fb22acd3fe3884d296d196db09b6f4e9078f9554377', NULL, 'mkv', 'h264', 'aac', 1920, 1080, 10021, '{"format":{"name":"matroska,webm","duration":10021000000,"size":2626565,"bitRate":2096848,"tags":{"encoder":"Lavf60.16.100","title":"Slouchcat Test Subtitles"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709"},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000},{"index":2,"type":"subtitle","codec":"subrip","language":"ger","title":"Deutsch (SRT)","default":true},{"index":3,"type":"subtitle","codec":"subrip","language":"eng","title":"English (SRT)"},{"index":4,"type":"subtitle","codec":"ass","language":"ger","title":"Deutsch (ASS)"},{"index":5,"type":"subtitle","codec":"ass","language":"eng","title":"English (ASS)"}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199859, 1793325200999, NULL, 1793325198232, 1793325200999, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TYE1RWHX6C8CTJQM6YM', '01M6585TYE1RWHX6C8CR9YJD94', 1, 'Slouchcat Test Unrated/Slouchcat Test Unrated.mp4', 66310, 11799752, 2625046, 1793316225639, '4c3cb756cbf73d3913ed4c77ad5fd261f37d7f3d9e1e5c6eb988fccc17da2577', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 10000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":10000000000,"size":2625046,"bitRate":2100036,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test Unrated"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00fd00fd00f', 1793325199860, 1793325200999, NULL, 1793325198232, 1793325200999, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TYF4HEW4J1E4N59SE9N', '01M6585TYF4HEW4J1E4JZE5V40', 1, 'Slouchcat Test Extras (2024)/Behind The Scenes/On Set.mp4', 66310, 11799745, 98171, 1793316224345, '30049f1b99ded9fca7c51ea76d1282b28f6a444a07e12d477d8401727308ddd8', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 5000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":5000000000,"size":98171,"bitRate":157073,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"On Set"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00f', 1793325199864, 1793325201028, NULL, 1793325198232, 1793325201028, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TYG50HT3BSC3K6GPE80', '01M6585TYF4HEW4J1E4S0F89G2', 1, 'Slouchcat Test Extras (2024)/Deleted Scenes/Scene 1.mp4', 66310, 11799743, 98172, 1793316223794, '20e54a28024662e3d88b81bcb6f92dc549074fc06d0e0351c2f5ccb505de5a73', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 5000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":5000000000,"size":98172,"bitRate":157075,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Scene 1"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00f', 1793325199863, 1793325201029, NULL, 1793325198232, 1793325201029, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TYHS7QMZSG7AS0VRQAK', '01M6585TYG50HT3BSC3N918J8G', 1, 'Slouchcat Test Extras (2024)/Featurettes/Making Of.mp4', 66310, 11799741, 98174, 1793316223282, '4d22df8b4b0150f430bcdcffa84101300a26dec5d108f9714d1bf7d779b6ab71', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 5000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":5000000000,"size":98174,"bitRate":157078,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Making Of"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00f', 1793325199900, 1793325201037, NULL, 1793325198232, 1793325201037, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TYHS7QMZSG7AVR7ZZ5F', '01M6585TYHS7QMZSG7AV4RMH2Q', 1, 'Slouchcat Test Extras (2024)/Slouchcat Test Extras (2024)-trailer.mp4', 66310, 11799746, 98201, 1793316224850, 'b5d9ae88034b726cf9e84ac537f185f76ea8baac390744ac752990784a124e8b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 5000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":5000000000,"size":98201,"bitRate":157121,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test Extras (2024)-trailer"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00f', 1793325199903, 1793325201060, NULL, 1793325198232, 1793325201060, NULL, NULL);
INSERT INTO "media_files" VALUES ('01M6585TYJ68J0E2CS58PM7T3N', '01M6585TYJ68J0E2CS58DN0GGF', 1, 'Slouchcat Test Extras (2024)/Trailers/Slouchcat Test Extras Trailer.mp4', 66310, 11799739, 98194, 1793316222755, '254ced3455cc586e45a5f98fcb8547cfc93da14d64c7a58c7b7f1e911361ef2b', NULL, 'mp4', 'h264', 'aac', 1920, 1080, 5000, '{"format":{"name":"mov,mp4,m4a,3gp,3g2,mj2","duration":5000000000,"size":98194,"bitRate":157110,"tags":{"compatible_brands":"isomiso2avc1mp41","encoder":"Lavf60.16.100","major_brand":"isom","minor_version":"512","title":"Slouchcat Test Extras Trailer"}},"streams":[{"index":0,"type":"video","codec":"h264","profile":"High","level":40,"width":1920,"height":1080,"pixFmt":"yuv420p","colorTransfer":"bt709","colorPrimaries":"bt709","language":"und","default":true},{"index":1,"type":"audio","codec":"aac","profile":"LC","channels":2,"channelLayout":"stereo","sampleRate":48000,"language":"und","default":true}],"chapters":[]}', X'0100d00fd00f', 1793325199893, 1793325201029, NULL, 1793325198232, 1793325201029, NULL, NULL);
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
INSERT INTO "tracks" VALUES (1, '01M6585TWT1YNE1CH3TZ3Q9HRF', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (2, '01M6585TWT1YNE1CH3TZ3Q9HRF', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (3, '01M6585TWT1YNE1CH3TZ3Q9HRF', 0, 'subtitle', 'subrip', 'de', NULL, 'sidecar', 'Slouchcat Test Download Show (2024) {tmdb-7101}/Season 01/Slouchcat Test Download Show (2024) {tmdb-7101} S01E01.de.srt', 0, 0);
INSERT INTO "tracks" VALUES (4, '01M6585TWXFMDSEPE5ANZ5WTCB', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (5, '01M6585TWXFMDSEPE5ANZ5WTCB', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (6, '01M6585TX0A6KKFTTK4AM5BRK8', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (7, '01M6585TX0A6KKFTTK4AM5BRK8', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (8, '01M6585TX2N7TACN188D77429J', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (9, '01M6585TX2N7TACN188D77429J', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (10, '01M6585TX4J3HDJ459C70A729B', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (11, '01M6585TX4J3HDJ459C70A729B', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (12, '01M6585TX6FV1CMGBDPNKQFBZ9', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (13, '01M6585TX6FV1CMGBDPNKQFBZ9', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (14, '01M6585TX9NYJJJ3RSXYYKAVP4', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (15, '01M6585TX9NYJJJ3RSXYYKAVP4', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (16, '01M6585TXANBCG17M505BH3Q06', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (17, '01M6585TXANBCG17M505BH3Q06', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (18, '01M6585TXBMVWKFXBHJXPXMAV8', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (19, '01M6585TXBMVWKFXBHJXPXMAV8', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (20, '01M6585TXCHS51027J7B63SVZ6', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (21, '01M6585TXCHS51027J7B63SVZ6', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (22, '01M6585TXEKWNC2RXHZ0XN2X6Q', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (23, '01M6585TXEKWNC2RXHZ0XN2X6Q', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (24, '01M6585TXHVBT2ZBQVVXKTTNA3', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (25, '01M6585TXHVBT2ZBQVVXKTTNA3', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (26, '01M6585TXGEC9NVX13PAQ17V84', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (27, '01M6585TXGEC9NVX13PAQ17V84', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (28, '01M6585TXM70G9GSBN3Z8T42WQ', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (29, '01M6585TXM70G9GSBN3Z8T42WQ', 1, 'audio', 'ac3', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (30, '01M6585TXHVBT2ZBQVW296NXXY', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (31, '01M6585TXHVBT2ZBQVW296NXXY', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (32, '01M6585TXJHQ022AZ82V1BJ4WN', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (33, '01M6585TXJHQ022AZ82V1BJ4WN', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (34, '01M6585TXRHCKT35BV8YYN6X1G', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (35, '01M6585TXRHCKT35BV8YYN6X1G', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (36, '01M6585TXT49PGDCXHCA7N0SV4', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (37, '01M6585TXT49PGDCXHCA7N0SV4', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (38, '01M6585TXQPPWG4PVPX5WH98G9', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (39, '01M6585TXQPPWG4PVPX5WH98G9', 1, 'audio', 'dts', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (40, '01M6585TXPNMFEW1D01BQK1CKQ', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (41, '01M6585TXPNMFEW1D01BQK1CKQ', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (42, '01M6585TXYKMDET3BAZ1E56GYJ', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (43, '01M6585TXYKMDET3BAZ1E56GYJ', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (44, '01M6585TXT49PGDCXHCBYT3NJJ', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (45, '01M6585TXT49PGDCXHCBYT3NJJ', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (46, '01M6585TXWMGFWKDKB0CFC7GFR', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (47, '01M6585TXWMGFWKDKB0CFC7GFR', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (48, '01M6585TY7APW6R5BQ43BJ4Q8V', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (49, '01M6585TY7APW6R5BQ43BJ4Q8V', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (50, '01M6585TY5S60SV3DEYXS58QYR', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (51, '01M6585TY5S60SV3DEYXS58QYR', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (52, '01M6585TY0V149WSKVTXVXAPXZ', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (53, '01M6585TY0V149WSKVTXVXAPXZ', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (54, '01M6585TY2F5QXFA1ZTJ4V4H7A', 0, 'video', 'hevc', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (55, '01M6585TY2F5QXFA1ZTJ4V4H7A', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (56, '01M6585TYCYRGFPP41J7FQEEQD', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (57, '01M6585TYCYRGFPP41J7FQEEQD', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (58, '01M6585TYCYRGFPP41J7FQEEQD', 0, 'subtitle', 'ass', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.ass', 0, 0);
INSERT INTO "tracks" VALUES (59, '01M6585TYCYRGFPP41J7FQEEQD', 1, 'subtitle', 'subrip', 'de', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).de.srt', 0, 0);
INSERT INTO "tracks" VALUES (60, '01M6585TYCYRGFPP41J7FQEEQD', 2, 'subtitle', 'ass', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.ass', 0, 0);
INSERT INTO "tracks" VALUES (61, '01M6585TYCYRGFPP41J7FQEEQD', 3, 'subtitle', 'subrip', 'en', NULL, 'sidecar', 'Slouchcat Test Sidecar Subtitles (2024)/Slouchcat Test Sidecar Subtitles (2024).en.srt', 0, 0);
INSERT INTO "tracks" VALUES (62, '01M6585TY97A2XBTT5DMSCH37V', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (63, '01M6585TY97A2XBTT5DMSCH37V', 1, 'audio', 'aac', 'ger', 'Deutsch (AAC 2.0)', 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (64, '01M6585TY97A2XBTT5DMSCH37V', 2, 'audio', 'ac3', 'eng', 'English (AC3 5.1)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (65, '01M6585TY97A2XBTT5DMSCH37V', 3, 'subtitle', 'subrip', 'ger', 'Deutsch (SRT)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (66, '01M6585TY97A2XBTT5DMSCH37V', 4, 'subtitle', 'subrip', 'eng', 'English (SRT)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (67, '01M6585TYBM1BK4M91ZZRZCBWN', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (68, '01M6585TYBM1BK4M91ZZRZCBWN', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (69, '01M6585TYBM1BK4M91ZZRZCBWN', 2, 'subtitle', 'hdmv_pgs_subtitle', 'eng', 'English (PGS)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (70, '01M6585TYD2X2X729XG9DEMSKF', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (71, '01M6585TYD2X2X729XG9DEMSKF', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (72, '01M6585TYD2X2X729XG9DEMSKF', 2, 'subtitle', 'subrip', 'ger', 'Deutsch (SRT)', 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (73, '01M6585TYD2X2X729XG9DEMSKF', 3, 'subtitle', 'subrip', 'eng', 'English (SRT)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (74, '01M6585TYD2X2X729XG9DEMSKF', 4, 'subtitle', 'ass', 'ger', 'Deutsch (ASS)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (75, '01M6585TYD2X2X729XG9DEMSKF', 5, 'subtitle', 'ass', 'eng', 'English (ASS)', 'embedded', NULL, 0, 0);
INSERT INTO "tracks" VALUES (76, '01M6585TYE1RWHX6C8CTJQM6YM', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (77, '01M6585TYE1RWHX6C8CTJQM6YM', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (78, '01M6585TYG50HT3BSC3K6GPE80', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (79, '01M6585TYG50HT3BSC3K6GPE80', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (80, '01M6585TYF4HEW4J1E4N59SE9N', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (81, '01M6585TYF4HEW4J1E4N59SE9N', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (82, '01M6585TYJ68J0E2CS58PM7T3N', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (83, '01M6585TYJ68J0E2CS58PM7T3N', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (84, '01M6585TYHS7QMZSG7AS0VRQAK', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (85, '01M6585TYHS7QMZSG7AS0VRQAK', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (86, '01M6585TYHS7QMZSG7AVR7ZZ5F', 0, 'video', 'h264', NULL, NULL, 'embedded', NULL, 1, 0);
INSERT INTO "tracks" VALUES (87, '01M6585TYHS7QMZSG7AVR7ZZ5F', 1, 'audio', 'aac', NULL, NULL, 'embedded', NULL, 1, 0);
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
INSERT INTO "top_picks" VALUES (1, '01M6585TXK7NNK68XFTPTBESVF', NULL, 0, 'Eingefroren: Cats Notiz', NULL, NULL, 1793325262869);
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
INSERT INTO "images" VALUES (1, '01M6585TWQDRBNT5QQW1HXW2JH', 'poster', 'original', 'jpeg', NULL, 'https://image.tmdb.org/t/p/original/qADLlBrxdy9dhf5jKhPd1reQwua.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/poster-original.jpg', '54f0dc25d97d01285a255ad5e24bd28a6d5232019977be28a824469e3a7d41bb', 'image/jpeg', 500, 750, 53490, NULL, 1793325200133);
INSERT INTO "images" VALUES (2, '01M6585TWQDRBNT5QQW1HXW2JH', 'poster', 'w200', 'jpeg', NULL, 'https://image.tmdb.org/t/p/original/qADLlBrxdy9dhf5jKhPd1reQwua.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/poster-w200.jpg', '9a76356e8432a355b3bfffdd37cc2c07d4d153b2914390cb64728e8ccf482b10', 'image/jpeg', 200, 300, 12080, '#7aa1a3', 1793325200376);
INSERT INTO "images" VALUES (3, '01M6585TWQDRBNT5QQW1HXW2JH', 'poster', 'w200', 'webp', NULL, 'https://image.tmdb.org/t/p/original/qADLlBrxdy9dhf5jKhPd1reQwua.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/poster-w200.webp', '0294364a754ae54718d72cf0f4e0fea151c4b4385369fe923e4543924864aae8', 'image/webp', 200, 300, 8500, '#7aa1a3', 1793325200376);
INSERT INTO "images" VALUES (4, '01M6585TWQDRBNT5QQW1HXW2JH', 'poster', 'w400', 'jpeg', NULL, 'https://image.tmdb.org/t/p/original/qADLlBrxdy9dhf5jKhPd1reQwua.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/poster-w400.jpg', '27e14c906c6dc61543897d41516d2ce1de0d3836960373058f051cf37d1e39df', 'image/jpeg', 400, 600, 27717, '#7aa1a3', 1793325200376);
INSERT INTO "images" VALUES (5, '01M6585TWQDRBNT5QQW1HXW2JH', 'poster', 'w400', 'webp', NULL, 'https://image.tmdb.org/t/p/original/qADLlBrxdy9dhf5jKhPd1reQwua.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/poster-w400.webp', 'e57eaeedc4ebf9591c2a7885a5141e44aadc397f6f7d8037ca9f9793c889b471', 'image/webp', 400, 600, 18580, '#7aa1a3', 1793325200376);
INSERT INTO "images" VALUES (6, '01M6585TWQDRBNT5QQW1HXW2JH', 'poster', 'w800', 'jpeg', NULL, 'https://image.tmdb.org/t/p/original/qADLlBrxdy9dhf5jKhPd1reQwua.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/poster-w800.jpg', '83372f61d42f207c549eb3a4657fb4f1b613c16944e3c00a7411926b8b607f8b', 'image/jpeg', 800, 1200, 65848, '#7aa1a3', 1793325200376);
INSERT INTO "images" VALUES (7, '01M6585TWQDRBNT5QQW1HXW2JH', 'poster', 'w800', 'webp', NULL, 'https://image.tmdb.org/t/p/original/qADLlBrxdy9dhf5jKhPd1reQwua.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/poster-w800.webp', 'e414720383d8c362dec3f68a9cde62b882e3b697bce257fa9ecf75465c74354b', 'image/webp', 800, 1200, 41290, '#7aa1a3', 1793325200376);
INSERT INTO "images" VALUES (8, '01M6585TWQDRBNT5QQW1HXW2JH', 'backdrop', 'original', 'jpeg', NULL, 'https://image.tmdb.org/t/p/original/tAgoJsvCxhodWFNH0Cpcmobv7e.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/backdrop-original.jpg', '0014fa2f930d8100189104840d9974507faebcbe8b940a42f79779c317744b82', 'image/jpeg', 1280, 720, 278031, NULL, 1793325200443);
INSERT INTO "images" VALUES (9, '01M6585TWQDRBNT5QQW1HXW2JH', 'backdrop', 'w780', 'jpeg', NULL, 'https://image.tmdb.org/t/p/original/tAgoJsvCxhodWFNH0Cpcmobv7e.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/backdrop-w780.jpg', '81d09656aca4a52cfbe0b65ded73f74b83041a127b1dc9d4f34217da807b08ec', 'image/jpeg', 780, 438, 96296, NULL, 1793325200822);
INSERT INTO "images" VALUES (10, '01M6585TWQDRBNT5QQW1HXW2JH', 'backdrop', 'w780', 'webp', NULL, 'https://image.tmdb.org/t/p/original/tAgoJsvCxhodWFNH0Cpcmobv7e.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/backdrop-w780.webp', '01ae9655a27ce94f08f89ddac50777f297ce7a996b7049f270c351e6816b2da8', 'image/webp', 780, 438, 78840, NULL, 1793325200822);
INSERT INTO "images" VALUES (11, '01M6585TWQDRBNT5QQW1HXW2JH', 'backdrop', 'w1280', 'jpeg', NULL, 'https://image.tmdb.org/t/p/original/tAgoJsvCxhodWFNH0Cpcmobv7e.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/backdrop-w1280.jpg', '0d5c3a302c63a4e96cd155ced34a503abe787ca7ceb9e5cdce581e2f0a9f9154', 'image/jpeg', 1280, 720, 240628, NULL, 1793325200822);
INSERT INTO "images" VALUES (12, '01M6585TWQDRBNT5QQW1HXW2JH', 'backdrop', 'w1280', 'webp', NULL, 'https://image.tmdb.org/t/p/original/tAgoJsvCxhodWFNH0Cpcmobv7e.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/backdrop-w1280.webp', '8c5d49dcc6065052b797c584122d3512c4b1f92ff65070e7d153a955bf7ca987', 'image/webp', 1280, 720, 199744, NULL, 1793325200822);
INSERT INTO "images" VALUES (13, '01M6585TWQDRBNT5QQW1HXW2JH', 'backdrop', 'w1920', 'jpeg', NULL, 'https://image.tmdb.org/t/p/original/tAgoJsvCxhodWFNH0Cpcmobv7e.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/backdrop-w1920.jpg', 'f3ff967f8e61aaad0ad91e7fbab5344e3f97f8dce25a4b6d5d787a4ae92c28fd', 'image/jpeg', 1920, 1080, 387897, NULL, 1793325200823);
INSERT INTO "images" VALUES (14, '01M6585TWQDRBNT5QQW1HXW2JH', 'backdrop', 'w1920', 'webp', NULL, 'https://image.tmdb.org/t/p/original/tAgoJsvCxhodWFNH0Cpcmobv7e.jpg', 'images/01M6585TWQDRBNT5QQW1HXW2JH/backdrop-w1920.webp', '188070ed540e34d1bef440bddc9f3caa3f78edba870072534d25887e19e94648', 'image/webp', 1920, 1080, 304016, NULL, 1793325200823);
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
INSERT INTO "devices" VALUES ('08f24f7f6557012f03c565948c639be4', 'curl/8.5.0', 1793325197707, 1793325197707, NULL, NULL, NULL, NULL, NULL, NULL);
INSERT INTO "devices" VALUES ('d5e65ce5192819f426b1ef43ca2d26a2', 'curl/8.5.0', 1793325197747, 1793325197747, NULL, NULL, NULL, NULL, NULL, NULL);
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
INSERT INTO "sessions" VALUES (1, '7b1eb564430ef5e2f26e44e2263b29d5edb4116a604958455dbe49fb8c7a3838', 1, 1, '08f24f7f6557012f03c565948c639be4', 'curl/8.5.0', 1793325197707, 1793325197707, 1795917197707, NULL);
INSERT INTO "sessions" VALUES (2, '575fdb2f2b4e7b468bf716306bf95bd9a111edc486f1f00c1e8485723cf10f1c', 1, 1, 'd5e65ce5192819f426b1ef43ca2d26a2', 'curl/8.5.0', 1793325197747, 1793325197758, 1795917197747, NULL);
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
INSERT INTO "people" VALUES (1, 12224, 'Alec Mapa', '/sb8qZmCfkwliw2vY2nWDf6cJP1e.jpg', 1793325198558);
INSERT INTO "people" VALUES (2, 1240420, 'Andrea James', '/f637WuIoNjfrenrgWvLZfsg3qCA.jpg', 1793325198558);
INSERT INTO "people" VALUES (3, 1240421, 'Calpernia Addams', NULL, 1793325198558);
CREATE TABLE credits (
    item_id   TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role      TEXT    NOT NULL CHECK (role IN ('cast', 'director', 'writer', 'creator')),
    character TEXT,
    ord       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (item_id, person_id, role)
) STRICT, WITHOUT ROWID;
INSERT INTO "credits" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 1, 'cast', NULL, 0);
INSERT INTO "credits" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 2, 'cast', NULL, 1);
INSERT INTO "credits" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 3, 'cast', NULL, 2);
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
INSERT INTO "media_segments" VALUES ('01M6585TX877H28GBN26XWG310', 'intro', 3404, 24452, 'fingerprint', 1, 1793325200741);
INSERT INTO "media_segments" VALUES ('01M6585TX877H28GBN26XWG310', 'outro', 72676, 98021, 'fingerprint', 1, 1793325200741);
INSERT INTO "media_segments" VALUES ('01M6585TX9NYJJJ3RSY1ARMMC0', 'intro', 10585, 30704, 'fingerprint', 1, 1793325200741);
INSERT INTO "media_segments" VALUES ('01M6585TX9NYJJJ3RSY1ARMMC0', 'outro', 73666, 100021, 'fingerprint', 1, 1793325200741);
INSERT INTO "media_segments" VALUES ('01M6585TXANBCG17M508AY0707', 'intro', 8000, 28000, 'chapter', 1, 1793325200741);
INSERT INTO "media_segments" VALUES ('01M6585TXANBCG17M508AY0707', 'outro', 77000, 102000, 'chapter', 1, 1793325200741);
INSERT INTO "media_segments" VALUES ('01M6585TXCHS51027J79733F3J', 'intro', 13247, 34295, 'fingerprint', 1, 1793325200741);
INSERT INTO "media_segments" VALUES ('01M6585TXCHS51027J79733F3J', 'outro', 77999, 104021, 'fingerprint', 0.75, 1793325200741);
INSERT INTO "media_segments" VALUES ('01M6585TXDAXTMQ0X78MGGC0BC', 'intro', 9719, 29466, 'fingerprint', 1, 1793325200741);
INSERT INTO "media_segments" VALUES ('01M6585TXDAXTMQ0X78MGGC0BC', 'outro', 79857, 106021, 'fingerprint', 1, 1793325200741);
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
INSERT INTO "search_docs" VALUES (1, 'item', '01M6585TWQDRBNT5QQW1HXW2JH', '01M6585TWQDRBNT5QQW1HXW2JH', 'Slouchcat Test Download Show / Transamerican Love Story /  /  /');
INSERT INTO "search_docs" VALUES (2, 'item', '01M6585TWSZF0B4KYY554V0591', '01M6585TWSZF0B4KYY554V0591', 'Season 1 /  /  /  /');
INSERT INTO "search_docs" VALUES (3, 'item', '01M6585TWSZF0B4KYY554YCSGP', '01M6585TWSZF0B4KYY554YCSGP', 'Episode 1 /  /  /  /');
INSERT INTO "search_docs" VALUES (4, 'item', '01M6585TWVGG193PDQ4EA1YP5W', '01M6585TWVGG193PDQ4EA1YP5W', 'Episode 2 /  /  /  /');
INSERT INTO "search_docs" VALUES (5, 'item', '01M6585TWY3XENWFZFRW7HS18X', '01M6585TWY3XENWFZFRW7HS18X', 'Episode 3 /  /  /  /');
INSERT INTO "search_docs" VALUES (6, 'item', '01M6585TX1SPDKZ0TMGPY8T2PN', '01M6585TX1SPDKZ0TMGPY8T2PN', 'Episode 4 /  /  /  /');
INSERT INTO "search_docs" VALUES (7, 'item', '01M6585TX3SNZ18WSVMA3XB3DP', '01M6585TX3SNZ18WSVMA3XB3DP', 'Episode 5 /  /  /  /');
INSERT INTO "search_docs" VALUES (8, 'item', '01M6585TX5TZF3VF0Q88721J6C', '01M6585TX5TZF3VF0Q88721J6C', 'Episode 6 /  /  /  /');
INSERT INTO "search_docs" VALUES (9, 'item', '01M6585TX6FV1CMGBDPS84K004', '01M6585TX6FV1CMGBDPS84K004', 'Slouchcat Test Intro Show / Where Are They Now? /  /  /');
INSERT INTO "search_docs" VALUES (10, 'item', '01M6585TX7TT52TQKRKR9G5KMG', '01M6585TX7TT52TQKRKR9G5KMG', 'Season 1 /  /  /  /');
INSERT INTO "search_docs" VALUES (11, 'item', '01M6585TX877H28GBN26XWG310', '01M6585TX877H28GBN26XWG310', 'Episode 1 /  /  /  /');
INSERT INTO "search_docs" VALUES (12, 'item', '01M6585TX9NYJJJ3RSY1ARMMC0', '01M6585TX9NYJJJ3RSY1ARMMC0', 'Episode 2 /  /  /  /');
INSERT INTO "search_docs" VALUES (13, 'item', '01M6585TXANBCG17M508AY0707', '01M6585TXANBCG17M508AY0707', 'Episode 3 /  /  /  /');
INSERT INTO "search_docs" VALUES (14, 'item', '01M6585TXCHS51027J79733F3J', '01M6585TXCHS51027J79733F3J', 'Episode 4 /  /  /  /');
INSERT INTO "search_docs" VALUES (15, 'item', '01M6585TXDAXTMQ0X78MGGC0BC', '01M6585TXDAXTMQ0X78MGGC0BC', 'Episode 5 /  /  /  /');
INSERT INTO "search_docs" VALUES (16, 'item', '01M6585TXFV7FMJPYSCG96AQRK', '01M6585TXFV7FMJPYSCG96AQRK', 'Slouchcat Test Show /  /  /  /');
INSERT INTO "search_docs" VALUES (17, 'item', '01M6585TXFV7FMJPYSCH66A7DW', '01M6585TXFV7FMJPYSCH66A7DW', 'Season 1 /  /  /  /');
INSERT INTO "search_docs" VALUES (18, 'item', '01M6585TXFV7FMJPYSCJTY4ZRP', '01M6585TXFV7FMJPYSCJTY4ZRP', 'Episode 1 /  /  /  /');
INSERT INTO "search_docs" VALUES (19, 'item', '01M6585TXGEC9NVX13PDDVTPJG', '01M6585TXGEC9NVX13PDDVTPJG', 'Episode 2 /  /  /  /');
INSERT INTO "search_docs" VALUES (20, 'item', '01M6585TXHVBT2ZBQVW1GFPND5', '01M6585TXHVBT2ZBQVW1GFPND5', 'Episode 3 /  /  /  /');
INSERT INTO "search_docs" VALUES (21, 'item', '01M6585TXHVBT2ZBQVW2FR1WFX', '01M6585TXHVBT2ZBQVW2FR1WFX', 'Specials /  /  /  /');
INSERT INTO "search_docs" VALUES (22, 'item', '01M6585TXJHQ022AZ82RZ7TT93', '01M6585TXJHQ022AZ82RZ7TT93', 'Episode 1 /  /  /  /');
INSERT INTO "search_docs" VALUES (23, 'item', '01M6585TXK7NNK68XFTPTBESVF', '01M6585TXK7NNK68XFTPTBESVF', 'Slouchcat Test AC3 51 /  /  /  /');
INSERT INTO "search_docs" VALUES (24, 'item', '01M6585TXM70G9GSBN41B2G7D0', '01M6585TXM70G9GSBN41B2G7D0', 'Slouchcat Test Broken /  /  /  /');
INSERT INTO "search_docs" VALUES (25, 'item', '01M6585TXNTJS6VDVEE4JJQ3P1', '01M6585TXNTJS6VDVEE4JJQ3P1', 'Slouchcat Test Chapters /  /  /  /');
INSERT INTO "search_docs" VALUES (26, 'item', '01M6585TXPNMFEW1D01E2HFA8X', '01M6585TXPNMFEW1D01E2HFA8X', 'Slouchcat Test DTS /  /  /  /');
INSERT INTO "search_docs" VALUES (27, 'item', '01M6585TXRHCKT35BV8XKA9H60', '01M6585TXRHCKT35BV8XKA9H60', 'Slouchcat Test Extras /  /  /  /');
INSERT INTO "search_docs" VALUES (28, 'item', '01M6585TXSN46TF04KKM2FK30H', '01M6585TXSN46TF04KKM2FK30H', 'Slouchcat Test H264 /  /  /  /');
INSERT INTO "search_docs" VALUES (29, 'item', '01M6585TXV85XJJ0DS5Z7Y68VM', '01M6585TXV85XJJ0DS5Z7Y68VM', 'Slouchcat Test H264 Copy /  /  /  /');
INSERT INTO "search_docs" VALUES (30, 'item', '01M6585TXXSRR48VSW6EVTC46P', '01M6585TXXSRR48VSW6EVTC46P', 'Slouchcat Test H264 MKV Long /  /  /  /');
INSERT INTO "search_docs" VALUES (31, 'item', '01M6585TXZ9E4QBE7KXJ408K87', '01M6585TXZ9E4QBE7KXJ408K87', 'Slouchcat Test HDR10 4K /  /  /  /');
INSERT INTO "search_docs" VALUES (32, 'item', '01M6585TY1F43AEP4WPVG98FXG', '01M6585TY1F43AEP4WPVG98FXG', 'Slouchcat Test HDR10 Chart /  /  /  /');
INSERT INTO "search_docs" VALUES (33, 'item', '01M6585TY3CMXZFGPM4XVA6K63', '01M6585TY3CMXZFGPM4XVA6K63', 'Slouchcat Test HEVC 10bit /  /  /  /');
INSERT INTO "search_docs" VALUES (34, 'item', '01M6585TY6Z709NFGCE902YJD9', '01M6585TY6Z709NFGCE902YJD9', 'Slouchcat Test HEVC Long /  /  /  /');
INSERT INTO "search_docs" VALUES (35, 'item', '01M6585TY82HPB7DNYVEFJ57ZE', '01M6585TY82HPB7DNYVEFJ57ZE', 'Slouchcat Test Multi Audio /  /  /  /');
INSERT INTO "search_docs" VALUES (36, 'item', '01M6585TYAPAP726NKRT2YB2Q5', '01M6585TYAPAP726NKRT2YB2Q5', 'Slouchcat Test PGS /  /  /  /');
INSERT INTO "search_docs" VALUES (37, 'item', '01M6585TYBM1BK4M9202GET26Y', '01M6585TYBM1BK4M9202GET26Y', 'Slouchcat Test Sidecar Subtitles /  /  /  /');
INSERT INTO "search_docs" VALUES (38, 'item', '01M6585TYCYRGFPP41JB1ABAY4', '01M6585TYCYRGFPP41JB1ABAY4', 'Slouchcat Test Subtitles /  /  /  /');
INSERT INTO "search_docs" VALUES (39, 'item', '01M6585TYE1RWHX6C8CR9YJD94', '01M6585TYE1RWHX6C8CR9YJD94', 'Slouchcat Test Unrated /  /  /  /');
INSERT INTO "search_docs" VALUES (40, 'item', '01M6585TYF4HEW4J1E4JZE5V40', '01M6585TYF4HEW4J1E4JZE5V40', 'On Set /  /  /  /');
INSERT INTO "search_docs" VALUES (41, 'item', '01M6585TYF4HEW4J1E4S0F89G2', '01M6585TYF4HEW4J1E4S0F89G2', 'Scene 1 /  /  /  /');
INSERT INTO "search_docs" VALUES (42, 'item', '01M6585TYG50HT3BSC3N918J8G', '01M6585TYG50HT3BSC3N918J8G', 'Making Of /  /  /  /');
INSERT INTO "search_docs" VALUES (43, 'item', '01M6585TYHS7QMZSG7AV4RMH2Q', '01M6585TYHS7QMZSG7AV4RMH2Q', 'Slouchcat Test Extras (2024) /  /  /  /');
INSERT INTO "search_docs" VALUES (44, 'item', '01M6585TYJ68J0E2CS58DN0GGF', '01M6585TYJ68J0E2CS58DN0GGF', 'Slouchcat Test Extras Trailer /  /  /  /');
INSERT INTO "search_docs" VALUES (45, 'text', '01M6585TWQDRBNT5QQW1HXW2JH', '01M6585TWQDRBNT5QQW1HXW2JH:de', 'Transamerican Love Story');
INSERT INTO "search_docs" VALUES (46, 'text', '01M6585TWQDRBNT5QQW1HXW2JH', '01M6585TWQDRBNT5QQW1HXW2JH:en', 'Transamerican Love Story');
INSERT INTO "search_docs" VALUES (47, 'person', '01M6585TWQDRBNT5QQW1HXW2JH', '1', 'Alec Mapa');
INSERT INTO "search_docs" VALUES (48, 'person', '01M6585TWQDRBNT5QQW1HXW2JH', '2', 'Andrea James');
INSERT INTO "search_docs" VALUES (49, 'person', '01M6585TWQDRBNT5QQW1HXW2JH', '3', 'Calpernia Addams');
INSERT INTO "search_docs" VALUES (50, 'text', '01M6585TX6FV1CMGBDPS84K004', '01M6585TX6FV1CMGBDPS84K004:de', 'Where Are They Now?');
INSERT INTO "search_docs" VALUES (51, 'text', '01M6585TX6FV1CMGBDPS84K004', '01M6585TX6FV1CMGBDPS84K004:en', 'Where Are They Now?');
INSERT INTO "search_docs" VALUES (52, 'text', '01M6585TWSZF0B4KYY554V0591', '01M6585TWSZF0B4KYY554V0591:de', 'Staffel 1');
INSERT INTO "search_docs" VALUES (53, 'text', '01M6585TWSZF0B4KYY554V0591', '01M6585TWSZF0B4KYY554V0591:en', 'Season 1');
INSERT INTO "search_docs" VALUES (54, 'text', '01M6585TWSZF0B4KYY554YCSGP', '01M6585TWSZF0B4KYY554YCSGP:de', 'Folge 1');
INSERT INTO "search_docs" VALUES (55, 'text', '01M6585TWSZF0B4KYY554YCSGP', '01M6585TWSZF0B4KYY554YCSGP:en', 'Getting to Know You');
INSERT INTO "search_docs" VALUES (56, 'text', '01M6585TWVGG193PDQ4EA1YP5W', '01M6585TWVGG193PDQ4EA1YP5W:de', 'Folge 2');
INSERT INTO "search_docs" VALUES (57, 'text', '01M6585TWVGG193PDQ4EA1YP5W', '01M6585TWVGG193PDQ4EA1YP5W:en', 'Something''s Cookin''');
INSERT INTO "search_docs" VALUES (58, 'text', '01M6585TWY3XENWFZFRW7HS18X', '01M6585TWY3XENWFZFRW7HS18X:de', 'Folge 3');
INSERT INTO "search_docs" VALUES (59, 'text', '01M6585TWY3XENWFZFRW7HS18X', '01M6585TWY3XENWFZFRW7HS18X:en', 'Love''s a Beach');
INSERT INTO "search_docs" VALUES (60, 'text', '01M6585TX1SPDKZ0TMGPY8T2PN', '01M6585TX1SPDKZ0TMGPY8T2PN:de', 'Folge 4');
INSERT INTO "search_docs" VALUES (61, 'text', '01M6585TX1SPDKZ0TMGPY8T2PN', '01M6585TX1SPDKZ0TMGPY8T2PN:en', 'I Love the Nightlife');
INSERT INTO "search_docs" VALUES (62, 'text', '01M6585TX3SNZ18WSVMA3XB3DP', '01M6585TX3SNZ18WSVMA3XB3DP:de', 'Folge 5');
INSERT INTO "search_docs" VALUES (63, 'text', '01M6585TX3SNZ18WSVMA3XB3DP', '01M6585TX3SNZ18WSVMA3XB3DP:en', 'I Like The Way You Move');
INSERT INTO "search_docs" VALUES (64, 'text', '01M6585TX5TZF3VF0Q88721J6C', '01M6585TX5TZF3VF0Q88721J6C:de', 'Folge 6');
INSERT INTO "search_docs" VALUES (65, 'text', '01M6585TX5TZF3VF0Q88721J6C', '01M6585TX5TZF3VF0Q88721J6C:en', 'Meet The Folks');
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
INSERT INTO "account_channels" VALUES (1, 'email', 0, '["new_content","errors","invitations","announcements"]', '{}', 1793325197706);
INSERT INTO "account_channels" VALUES (1, 'push', 0, '["new_content","errors","invitations","announcements"]', '{}', 1793325197706);
INSERT INTO "account_channels" VALUES (1, 'telegram', 0, '["new_content","errors","invitations","announcements"]', '{}', 1793325197706);
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
, rotation_seconds INTEGER NOT NULL DEFAULT 5
    CHECK (rotation_seconds BETWEEN 3 AND 15)) STRICT;
INSERT INTO "spotlight_config" VALUES (1, 1, 'trending', NULL, 'both', 20, 5, 24, NULL, 1793325197768, 7);
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
INSERT INTO "accounts" VALUES (1, 'frozen@example.test', '$argon2id$v=19$m=65536,t=3,p=4$rCCYcBHPK1usz9XgBkxUdQ$374vqbxPk0ySlcACCLRD0OEccoAEzD7QK/6zpWGH4H0', 'admin', 1, 1793325197706, 1793325197706, 18, 1, NULL, NULL, NULL, NULL, NULL);
CREATE TABLE item_field_sources (
    item_id  TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    field    TEXT NOT NULL,              -- title, overview, tagline, original_title, genres,
                                         -- runtime_minutes, release_date, own_rating, poster, backdrop
    lang     TEXT NOT NULL DEFAULT '',   -- '' for fields without a language
    provider TEXT NOT NULL,
    PRIMARY KEY (item_id, field, lang)
) STRICT, WITHOUT ROWID;
INSERT INTO "item_field_sources" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'backdrop', '', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'original_title', '', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'overview', 'en', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'poster', '', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'release_date', '', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'runtime_minutes', '', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'title', 'de', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'title', 'en', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TWQDRBNT5QQW1HXW2JH', 'vote_average', '', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TX6FV1CMGBDPS84K004', 'original_title', '', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TX6FV1CMGBDPS84K004', 'overview', 'en', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TX6FV1CMGBDPS84K004', 'title', 'de', 'tmdb');
INSERT INTO "item_field_sources" VALUES ('01M6585TX6FV1CMGBDPS84K004', 'title', 'en', 'tmdb');
CREATE TABLE "libraries" (
    id                     INTEGER PRIMARY KEY,
    name                   TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    kind                   TEXT    NOT NULL CHECK (kind IN ('video', 'audiobooks', 'podcasts')),
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
INSERT INTO "libraries" VALUES (1, 'Filme', 'video', NULL, 1793325197707, 1793325198291, 'done', 1793325198219, 1793325198291, NULL, 23, 23, NULL);
INSERT INTO "libraries" VALUES (2, 'Serien', 'video', NULL, 1793325197763, 1793325198290, 'done', 1793325198220, 1793325198290, NULL, 15, 15, NULL);
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
CREATE TRIGGER items_enclosure_update AFTER UPDATE OF enclosure_url ON items
WHEN NEW.kind = 'podcast_episode'
BEGIN
    UPDATE items SET available = (NEW.enclosure_url IS NOT NULL
        OR EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = NEW.id AND m.missing_since IS NULL))
    WHERE id = NEW.id;
END;
CREATE INDEX items_library_kind_added    ON items (library_id, kind, added_at DESC);
CREATE INDEX items_library_kind_released ON items (library_id, kind, release_date DESC);
CREATE INDEX items_library_kind_vote     ON items (library_id, kind, vote_average DESC);
CREATE INDEX item_texts_sort_key         ON item_texts (item_id, lang, sort_key);
