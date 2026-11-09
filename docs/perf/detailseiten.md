# Messwerte: Konzept 17 «Serien- und Filmseite», Stufe 3

Konzept `docs/planning/17-detailseiten.md` (Abschnitte 5.2 Punkt 5 und 6.2), ADR 0026. Gemessen am
9. November 2026 auf CT 190 (192.168.1.43, Image `detailseiten3-c18cb81`, `v1.20.0-33-gc18cb813`)
und auf dem Entwicklungsrechner (Go-Benchmarks).

## 1. Personen- und Sammlungsbilder (D-5)

Erster Lauf von `people.images` nach dem Start mit Migration 00040 (Migration 1,0 s, Sicherung
`pre-migration-v1.20.0-33-gc18cb813-20261109T101118.757Z.db`). Der Lauf startete von selbst kurz nach dem
Serverstart; ein Nachholen von Hand war nicht nötig.

| Grösse | Wert |
|---|---|
| Auftrag | ein Auftrag (`198370`), 18'199 Subjekte, Zustand `done`, 0 Fehler, kein Folgeauftrag |
| Dauer | 1'858 s (31 min), rund 9,8 Subjekte je Sekunde (Drossel 10/s) |
| Personen mit Bild | 17'794 (von 19'971 Personen mit `profile_path`; Kandidaten sind nur Personen mit einer Rolle in einem vorhandenen Titel) |
| Sammlungen mit Bild | 405 (von 408 TMDB-Sammlungen mit `poster_path`) |
| `missing`-Zeilen | 0 |
| `sum(bytes)` Personen | JPEG 241,5 MB, WebP 149,6 MB, zusammen 391 MB (rund 22 KB je Person) |
| `sum(bytes)` Sammlungen | JPEG 22,2 MB, WebP 16,5 MB, zusammen 38,7 MB |
| `du -sh` im `CacheDir` | `people/` 465 MB, `collections/` 40 MB |

Die Schätzung im Konzept (0,9 GB bei 30'000 Personen, 30 KB je Person) war zu hoch: es sind rund
0,5 GB. Der Lauf lief neben dem normalen Hintergrundstau; im Log steht in den zwei Stunden nach dem
Start kein `ERROR`.

## 2. Bibliotheksraster (D-6)

### 2.1 Server

Go-Benchmark `BenchmarkLibraryRows/page/LibraryItems-title` (10'000 Filme, 200 Läufe, alt und neu
abwechselnd auf demselben Rechner):

| Fall | vor der Korrektur aus dem Code-Review | nach der Korrektur |
|---|---|---|
| Seite 0 mit `letterStarts` | p50 53,6 / 54,5 ms | p50 42,4 / 42,5 ms |
| Seite bei `offset=900` (ohne Facetten) | p50 42,4 ms | p50 43,4 / 43,2 ms |

Seit der gruppierten Zählabfrage kostet `letterStarts` nichts mehr messbar.

Auf CT 190 im Browser-Pane der Claude-App (Chromium, LAN, Marcos Sitzung), je 5 Aufwärmrunden und
60 Anfragen, 0 Antworten ungleich 200. Der CT war dabei durch den Lauf aus Abschnitt 1 ausgelastet.

| Anfrage | p50 | p95 | max |
|---|---|---|---|
| `/libraries/visible` | 18,2 ms | 22,6 ms | 30,3 ms |
| `/libraries/1/items` (1'800 Filme, Standard) | 107,7 ms | 115,0 ms | 115,8 ms |
| `/libraries/1/items?sort=added` | 107,7 ms | 116,7 ms | 121,0 ms |
| `/libraries/1/items?sort=title&limit=100` (mit `letterStarts`) | 115,9 ms | 123,1 ms | 134,1 ms |
| `/libraries/1/items?sort=title&order=desc&limit=100` | 119,5 ms | 136,4 ms | 147,2 ms |
| `/libraries/1/items?sort=title&offset=900&limit=100` | 121,6 ms | 134,2 ms | 156,9 ms |
| `/libraries/2/items?sort=title&status=started&limit=100` | 213,3 ms | 225,5 ms | 231,7 ms |
| `/picks` | 10,6 ms | 13,1 ms | 13,8 ms |

`letterStarts` der Filmbibliothek: `#` bei 0 und bei 1'798 (zwei Blöcke wie erwartet), A bei 32,
M bei 863, Z bei 1'775.

### 2.2 App

Browser-Pane mit Handy-Emulation (375 × 812, Android-Chrome-Kennung), Filmbibliothek mit 1'800 Titeln.
**Ohne CPU-Drosselung:** das Browser-Pane bietet keine Drosselung; der Plan sah vierfache Drosselung
vor. Den Beleg auf dem Telefon liefert Marcos Abnahme.

| Grösse | Wert |
|---|---|
| Erste Seite (100 Titel) geladen | 186 ms nach Navigationsstart |
| Erste Bilder im Bild | 237 ms, 18 Karten gezeichnet, 18 Bilder geladen, kein Platzhalter |
| Schnelles Durchscrollen bis ans Ende (400 px je Bild, 150'399 px) | 7,5 s, 374 Bilder, 0 lange Aufgaben; in 162 Bildern kurz Platzhalter (höchstens 15 Karten), am Ende sofort alle gefüllt; 3 Seitenabrufe, weil nur die sichtbaren Seiten geladen werden |
| Ruhiges Scrollen (40 px je Bild, 24'000 px, Seiten schon im Cache) | 12 s, 600 Bilder, 0 lange Aufgaben, nie ein Platzhalter |

Die Tastaturabnahme (↓ über 360 Zeilen, Sprung auf «M» und Zurück auf Karte N, Tab aus dem Raster,
40'000 Titel auf 390 px) läuft in `e2e/library.spec.ts` in Chromium und Firefox.
