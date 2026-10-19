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
