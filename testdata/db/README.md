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
