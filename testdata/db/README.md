# Eingefrorene Datenbankstände

Ab M1a legt jeder Meilenstein hier einen SQL-Dump des Datenbankschemas mit Beispieldaten ab:
`testdata/db/<Mx>.sql` (z. B. `M1a.sql`, `M1b.sql`). Grundlage: Plan 6.2, Punkt 4, und Regel 5 in
`CLAUDE.md` (Migrationen nur vorwärts).

- Ein Stand wird am Ende des Meilensteins erzeugt und danach nie mehr geändert.
- Der Migrationstest lädt jeden früheren Stand in eine leere SQLite-Datenbank, führt alle aktuellen
  Migrationen aus und prüft `PRAGMA foreign_key_check` sowie die Sicherung per `VACUUM INTO`.
- `M1a.sql`: Stand nach M1a (Konto `frozen@example.test`, eine Filmbibliothek auf `testdata/media/Movies`
  ohne TMDB-Schlüssel, 8 Titel mit Dateien, Spuren und einem Fortschritt). Erzeugt mit
  `server/bin/slouchcat dump --config-dir <config>` nach einem Lauf des Assistenten gegen einen frischen
  Server. Der Test `TestFrozenStatesMigrate` in `server/internal/store` spielt jeden Stand ein.
- `M1b.sql`: Stand nach M1b (gleiches Vorgehen: Konto `frozen@example.test`, Filmbibliothek auf
  `testdata/media/Movies` ohne TMDB-Schlüssel, 10 Titel mit Dateien, Spuren und Keyframe-Index, ein
  Fortschritt und eine beendete Wiedergabesitzung in `playback_sessions` (Migration 00005); `images` ist
  leer, weil ohne TMDB-Schlüssel keine Bilder entstehen (Migration 00006 wird vom Test
  `TestImagesMigrationKeepsM1aRows` mit M1a-Zeilen geprüft).
- `M2.sql`: Stand nach M2 (Konten, Profile, Zugriff; Migration 00007). Erzeugt mit `server/bin/slouchcat dump`
  nach einem Lauf gegen einen frischen Server: Hauptkonto `frozen@example.test` mit Filmbibliothek auf
  `testdata/media/Movies` ohne TMDB-Schlüssel und drei Profilen (Hauptprofil, «Kind» mit PIN und Stufe 6,
  «Teenager» mit Stufe 12), drei per Administrator übersteuerte Freigaben (6, 12, 16; `rating_source`
  `manual`, `locked_fields` mit `own_rating`), eine eingelöste Einladung und eine offene Einladung, dazu das
  Gastkonto `frozen-guest@example.test` (Obergrenze 12, nur Bibliothek 1 in `account_libraries`) mit zwei
  Profilen. Die Tabellen `devices` (Geräte-Cookie) und `invitation_libraries` sind gefüllt, `pin_attempts`
  bleibt leer. Der Test `TestMigration00007KeepsFrozenData` prüft zusätzlich, dass M1a und M1b durch
  Migration 00007 gültig bleiben (Hauptkonto: Stufe 18, alle Bibliotheken, Sitzungen und Hauptprofil erhalten).
- `M3.sql`: Stand nach M3 (Bibliothek komplett; Migration 00008). Erzeugt mit `server/bin/slouchcat dump` nach
  einem Lauf gegen einen frischen Server: Assistent mit Hauptkonto `frozen@example.test` und Filmbibliothek auf
  `testdata/media/Movies` ohne TMDB-Schlüssel (10 Titel, Scan, Probe und Keyframes durch die echten Dienste), eine
  Serienbibliothek auf `testdata/media/TV`, die Profile «Teenager» (12) und «Kind» (6) und fünf Freigaben über die
  Massenzuweisung `POST /admin/items/rating`. Weil der Scanner und die Metadaten-Jobs dieses Branches noch keine
  Serien kennen, sind die M3-Daten danach per SQL ergänzt (Skript in der Übergabe `m3-api-notes.md`): Serie mit
  Staffel 1 (Folgen E01, E02 mit eigener Stufe 16, Mehrfachfolge S01E03-E04) und Specials (S00E01), Extras an der
  Serie (`behind_the_scenes`) und am H264-Film (`trailer`), eine zweite Version (720p) des H264-Films, Trailer
  automatisch (`tmdb`, `youtube`) und manuell, zwei Personen mit Rollen (Besetzung, Regie, Idee), eine manuelle
  Zuordnung (`tmdb_id` 603, gesperrte Anbieter-IDs, `metadata_state` `pending`), eine ignorierte Datei und eine
  nicht abspielbare Datei (`probe_error`). Die abgeleiteten Spalten `effective_rating`, `available` und
  `min_child_rating` stammen aus den Triggern. `TestFrozenM3StateCoversM3Data` prüft, dass jede dieser Datenarten vorhanden ist;
  `TestMigration00008KeepsFrozenData` prüft M1b und M2 nach Migration 00008 (Zeilen von `items`, `media_files`,
  `playback_sessions` erhalten, Container nach der neuen Regel berechnet).
- `M4.sql`: Stand nach M4 (Wiedergabe komplett; Migration 00012). Erzeugt mit `server/bin/slouchcat dump` nach
  einem Lauf gegen einen frischen Server (Skript in den Sitzungsnotizen `freeze-m4.sh`): Assistent mit
  Hauptkonto `owner@example.test`, Filmbibliothek auf `testdata/media/Movies` und Serienbibliothek auf
  `testdata/media/TV` ohne TMDB-Schlüssel (Scan, Probe, Keyframes, Sidecar-Spuren und die Intro-Erkennung
  `segments.detect` durch die echten Dienste: zehn `media_segments`-Zeilen der Intro-Testsaison), ein Profil
  «Kind» (Stufe 12) mit allen vier Limits (720p, 4000 kbit/s, 1 Stream, 60 min pro Tag), ein Gerät mit
  Fähigkeitsprofil («Chrome 156 · Linux», `capabilities` als JSON) und eine beendete Wiedergabesitzung des
  Multi-Audio-Films mit Qualität 720p, AC3-Tonspur und deutscher SRT-Spur (`subtitle_delivery` `text`,
  `stopped_by` `client`) samt Fortschrittsereignis mit `playback_session_id`. `TestFrozenStatesMigrate` lädt
  ihn; die Migrationen 00011 und 00012 sind in M1a bis M3 ebenfalls geprüft (Spalten mit Standardwerten).
- `M5.sql`: Stand nach M5 (Offline-Wiedergabe; Migration 00013). Erzeugt mit `server/bin/slouchcat dump` nach
  einem Lauf gegen einen frischen Server (Skript in den Sitzungsnotizen `freeze-m5.sh`): Assistent wie M4,
  Profil «Kind» (Stufe 12, 60 min pro Tag) mit PIN (`pin_salt`) und separater Offline-PIN (`offline_pin`,
  PBKDF2-Verifier), Downloads des Hauptprofils auf einem Gerät: der H264-Film in `medium` (vorbereitet, über den
  Sync als vollständig abgerufen gemeldet, `fetched_at`) und die Staffel der «Slouchcat Test Download Show» in
  `low` (fünf vorbereitete Folgen, die sechste gelöscht und damit ein Grabstein in `download_tombstones`),
  `allowed_profile_ids` vom Server berechnet, eine aktive Auto-Download-Regel und ein hochgeladenes
  Offline-Fortschrittsereignis (`offline`, `uploaded_at`, `download_id`). `TestFrozenM5StateCoversM5Data` prüft,
  dass jede dieser Datenarten nach der Migration vorhanden ist.
- `M6.sql`: Stand nach M6 (Hörbücher und Podcasts; Migration 00014). Erzeugt mit `server/bin/slouchcat dump` nach
  einem Lauf gegen einen frischen Server (Skript in den Sitzungsnotizen `freeze-m6.sh`): Assistent wie M5, dazu die
  Bibliotheken «Hörbücher» (Art `audiobooks`, Standardstufe 6, Wurzel `testdata/media/Audiobooks`: das M4B-Buch mit
  zwölf Kapitelmarken und eingebettetem Cover sowie das MP3-Buch mit zehn Teilen, `part_index`, buchweiten Kapiteln
  und Ordnerbild) und «Podcasts» (Art `podcasts`, Standardstufe 0, ohne Wurzel) mit einem Abo auf den lokalen
  Test-Feed (`testdata/podcasts/feed.xml.tmpl` über einen Python-HTTP-Server): drei Episoden nur mit Enclosure,
  Episode 3 `explicit` und damit Stufe 18 (`rating_source` `feed`), Feed-Zustand (`feed_url`, `feed_checked_at`,
  `cover_url`). Profil «Kind» (Stufe 12) mit PIN; Downloads des Hauptprofils: das M4B-Buch in `medium` und
  Episode 1 in `low` (Quelle `source_url`, kein `media_file_id`), beide vorbereitet; Fortschritt im dritten Kapitel
  des Buchs mit Geschwindigkeit 1.5 (`playback_rate`); eine Auto-Download-Regel mit `new_podcast_episodes`.
  `TestFrozenM6StateCoversM6Data` prüft, dass jede dieser Datenarten nach der Migration vorhanden ist.
- `M7.sql`: Stand nach M7 (Entdecken und Kuratierung; Migration 00015). Erzeugt mit `server/bin/slouchcat dump` nach
  einem Lauf gegen einen frischen Server (Skript in den Sitzungsnotizen `freeze-m7.sh`): Bibliotheken, Profil «Kind»,
  Downloads und Regel wie M6; dazu ein globaler Pick mit Notiz und ein Pick nur für «Kind» mit Zeitraum, eine
  manuelle Sammlung «Slouchcat Testreihe» mit zwei Mitgliedern, ein Merklisten-Eintrag, Favoriten auf der Testserie
  und dem Test-Podcast, eine geänderte Reihenfolge der Startseite (Reihe «Genres» ausgeblendet), die Trakt-Listen
  eines Laufs gegen einen lokalen Mock (Trending und Beliebt, fünf Filme und zwei Serien mit den `{tmdb-NNNN}`-IDs der
  Testmedien) und eine Benachrichtigung für eine vierte, nach dem Favorisieren erschienene Episode. Der Suchindex
  (`search_index`) und die Genres (`item_genres`) stammen aus den Triggern der Migration.
  `TestFrozenM7StateCoversM7Data` prüft, dass jede dieser Datenarten nach der Migration vorhanden ist.
- `M8.sql`: Stand nach M8 (Administration und Benachrichtigungen; Migration 00016). Erzeugt mit `server/bin/slouchcat dump`
  nach einem Lauf gegen einen frischen Server (Skript in den Sitzungsnotizen `freeze-m8.sh`): Assistent mit Hauptkonto
  `owner@example.test`, Filmbibliothek auf `testdata/media/Movies` und Serienbibliothek auf `testdata/media/TV` ohne
  TMDB-Schlüssel, Profil «Kind» (Stufe 12, PIN), ein Pick mit Notiz, ein Favorit auf der Testserie und ein
  Fortschrittsereignis mit Gerät (`progress_events.device_id`). Dazu die M8-Daten: Einstellungen `public_url`,
  `update_check_enabled` und der Mailserver (`smtp_*`, Passwort verschlüsselt mit `v1:`), die Kanalzeilen des Hauptkontos
  (`account_channels`: E-Mail an mit Ereignissen, Push und Telegram aus), ein Push-Abo mit P-256-Schlüssel
  (`push_subscriptions`), ein offener Telegram-Link-Code (`telegram_links`, Bot-Token nur aus der Umgebung, nie
  gespeichert), zwei Ankündigungen (eine veröffentlicht und vom Profil «Kind» quittiert in `announcement_acks`, eine als
  Entwurf) und die Zustellung der Ankündigung an den E-Mail-Kanal (`deliveries`, nach dem fehlgeschlagenen Versuch gegen
  den nicht existierenden Mailserver wieder eingereiht). `TestFrozenM8StateCoversM8Data` prüft, dass jede dieser
  Datenarten nach der Migration vorhanden ist und kein Passwort im Klartext im Stand steht.
