# Messwerte: Migration 00065 «Metadaten je Ordner» (Konzept 24, Stufe 2)

Konzept `docs/planning/24-metadaten-pro-ordner.md`, Abschnitt 3.5, ADR 0036. Die Migration 00065
bekommt zwei Spalten an `library_roots`, eine Spalte mit Index an `items`, kopiert die Abweichungen
der Bibliotheken auf ihre Video-Ordner und füllt den Heimatordner jedes Titels (`items.root_id`).
Gemessen wurde, wie lange das auf einer Datenbank in der Grösse von CT 190 (rund 70'000 Titelzeilen)
dauert, weil es beim ersten Start nach dem Update einmal läuft.

## Aufbau

- Entwicklungsrechner am 16. November 2026: i7-14700K, SQLite-Datei auf der lokalen SSD
  (Testverzeichnis).
- Ein Wegwerf-Test im Paket `store` (nicht eingecheckt): der eingefrorene Stand
  `testdata/db/ordner-metadaten.sql` (Stand 00064, zwei Abweichungen, zwei Serienordner), dazu per
  SQL 3'000 Filme mit 6'000 Extras im Filmordner und 600 Serien mit 3'000 Staffeln und 60'000
  Folgen, deren Dateien abwechselnd in den beiden Serienordnern liegen (jede Serie liegt also in zwei
  Ordnern). Zusammen 72'656 Titelzeilen und 69'041 Dateien. Dann `store.Open` wie beim Start des
  Servers: Sicherung mit `VACUUM INTO`, dann die Migration.
- Drei Läufe, je auf einer frischen Kopie.

## Ergebnis

| Messung | Lauf 1 | Lauf 2 | Lauf 3 |
|---|---|---|---|
| Migration 00065 (goose, `duration_ms`) | 845 ms | 844 ms | 844 ms |
| `store.Open` samt Sicherung und Migration | 1,10 s | 1,10 s | 1,09 s |
| Titel mit Heimatordner danach | 72'649 | 72'649 | 72'649 |

Die sieben Titel ohne Heimatordner sind die des eingefrorenen Stands ohne vorhandene Datei (die
fünf Hörbücher ohne Teile, ein Podcast nur aus dem Feed mit seiner Folge).

## Einordnung

Unter einer Sekunde auf dem Entwicklungsrechner; CT 190 (2 vCPU, Datenbank auf der lokalen Platte des
Containers) dürfte ein Mehrfaches brauchen, bleibt aber im Bereich weniger Sekunden. Der Start wartet
so lange; die Sicherung davor gibt es schon. Auf CT 190 selbst ist nicht gemessen worden.
