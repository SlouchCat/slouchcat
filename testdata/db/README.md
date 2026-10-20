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
