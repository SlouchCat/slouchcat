# Messwerte: «Freigaben neu lesen» (Konzept 22, Stufe 3)

Konzept `docs/planning/22-ersteinrichtung-und-sprachen.md`, Abschnitt 4.11 (F-16, F-17), ADR 0034. Der Job
`metadata.ratings_reread` liest die Freigaben der Filme und Serien einmal nach dem Update und bei jedem
Wechsel des Freigabelands neu. Gemessen wurde, wie lange das dauert: lokal, wenn die Freigaben je Land schon
in `item_certifications` stehen, und mit Anfragen, wenn sie fehlen (das erste Mal nach dem Update).

## Aufbau

- Entwicklungsrechner am 15. November 2026: i7-14700K, SQLite-Datei auf der lokalen SSD (Testverzeichnis).
- Ein Wegwerf-Test im Paket `metadata` (nicht eingecheckt) mit der Testumgebung der Metadaten-Tests
  (`newEnv`, falscher TMDB-Server auf `127.0.0.1`):
  - **Lokal:** 2'000 und 10'000 Filme mit `tmdb:US` 16 und vier gespeicherten Freigaben (US R, DE 16, FR -12,
    GB 15); der ganze Job (`ratingsReread`, Stapel von 200) je Freigabeland US, FR und DE. Jeder Lauf ändert
    jeden Titel (17, 12, 16), schreibt also jede Zeile.
  - **Mit Anfragen:** 300 Filme ohne gespeicherte Freigaben; der Job fragt jeden Titel einmal bei TMDB, durch
    den echten Begrenzer des TMDB-Clients (20 Anfragen je Sekunde, `tmdbRate`).

## Ergebnis

| Lauf | Titel | Dauer | je Titel |
|---|---|---|---|
| lokal, US | 2'000 | 0,85 s | 0,43 ms |
| lokal, FR | 2'000 | 0,72 s | 0,36 ms |
| lokal, DE | 2'000 | 0,69 s | 0,35 ms |
| lokal, US | 10'000 | 4,05 s | 0,40 ms |
| lokal, FR | 10'000 | 3,57 s | 0,36 ms |
| lokal, DE | 10'000 | 3,58 s | 0,36 ms |
| mit Anfragen (TMDB) | 300 | 14,0 s | 46,7 ms (300 Anfragen) |

- **Lokal** kostet ein Titel rund 0,4 ms, fast alles ist das Schreiben samt Triggern. Ein Landwechsel mit
  gespeicherten Freigaben ist auch bei einer grossen Bibliothek in Sekunden fertig.
- **Mit Anfragen** bestimmt der Begrenzer die Dauer: rund 21 Titel je Sekunde, also eine Anfrage je Film.

## Schätzung für eine grosse Bibliothek

| Bibliothek | erstes Mal nach dem Update | jeder spätere Landwechsel |
|---|---|---|
| 2'000 Filme, 300 Serien mit TMDB | rund 2 Minuten | unter 1 Sekunde |
| 10'000 Filme, 1'000 Serien mit TMDB | rund 9 Minuten | rund 4 Sekunden |
| dazu 1'000 Serien mit TVDB in der Liste | plus rund 3½ Minuten (TVDB 5 Anfragen je Sekunde) | – |

- Das erste Mal nach dem Update ist langsam, weil vor Stufe 3 keine Freigaben je Land gespeichert waren. Es
  läuft im Hintergrund in Stapeln von 200 Titeln; ein Neustart setzt beim letzten Stapel fort.
- Die Anfragen teilen sich den Begrenzer mit den übrigen Metadaten-Jobs. Läuft gleichzeitig ein Scan mit
  vielen neuen Titeln, dauert es länger.
- Bis der Job fertig ist, gelten die alten Werte; die Übersicht zeigt «Freigaben werden neu gelesen».
- Ohne TMDB-Schlüssel fragt der Job nichts und lässt die Werte, wie sie sind.

## Nicht gemessen

- Die e2e-Testdaten haben keinen TMDB-Schlüssel und darum keine Freigaben eines Anbieters; dort reiht der
  Server beim Start keinen Lauf ein (`TestRatingsRereadOnceAndOnlyWithTitles`).
- Eine echte Bibliothek auf CT 190 mit echten TMDB-Antworten: Die Antwortzeit von TMDB kommt zum Begrenzer
  hinzu, wenn sie über 50 ms liegt.
