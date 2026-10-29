# Messwerte: Bibliotheken mit Inhaltsart und Metadatenquellen je Ordner (ADR 0015)

Konzept `docs/planning/07-bibliotheken.md`, Schritte 1 bis 6. Die Messungen zu Hörbuch- und
Podcast-Ordnern auf CT 190 und zum Anime-Lauf mit drei Anbietern folgen mit den Schritten 3, 4 und 6.

## 1. Scanner ohne Regression (Schritt 1, `slouchcat scanbench`)

Derselbe Platzhalterbaum wie in `docs/perf/M3.md` (60'800 leere Dateien, 2'500 Filme mit 5'169
Extras, 400 Serien mit 40'334 Folgen), erzeugt mit `testdata/generate-library-tree.py` unter
`/tmp`, 4 Worker, auf dem Entwicklungsrechner am 29. Oktober 2026. Die neue Fassung liest je
Wurzel und hält einen Titelindex je Titelart; zum Vergleich lief die Binärdatei von `main`
(`9923807`) **abwechselnd** auf demselben Baum, zweimal je Fassung:

```bash
server/bin/slouchcat scanbench --root /tmp/…/placeholder-tree --workers 4 --json
```

| Messung | `main` Lauf 1 | neu Lauf 1 | `main` Lauf 2 | neu Lauf 2 | M3 (19. Okt., Median) |
|---|---|---|---|---|---|
| Erst-Scan «Filme» (7'669 Dateien) | 7.9 s | 9.6 s | 9.3 s | 9.0 s | 5.8 s |
| Erst-Scan «Serien» (40'334 Dateien) | 40.7 s | 42.7 s | 41.7 s | 42.2 s | 26.9 s |
| Erst-Lauf bis die Queue leer ist (50'906 Jobs) | 64.1 s | 66.3 s | 65.1 s | 65.2 s | 47.0 s |
| Schreibtransaktionen | 713 | 713 | 713 | 713 | 713 |
| Längste Schreibtransaktion | 324 ms | 408 ms | 342 ms | 319 ms | 189 ms (< 2 s) |
| `GET /home` p95 während des Scans | 197 ms | 216 ms | 186 ms | 235 ms | 99.5 ms (< 300 ms) |
| Rescan ohne Änderung | 1.4 s, 0 Op. | 1.4 s, 0 | 1.6 s, 0 | 1.3 s, 0 | 1.9 s |
| Wiederaufnahme nach Abbruch bei 10 s | 31.1 s | 31.3 s | 31.8 s | 32.9 s | 16.3 s |
| Jobs mit `attempts > 1`, abgelaufene Leases, doppelte Schlüssel | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 |

**Lesart:** `main` und die neue Fassung liegen am selben Tag innerhalb der Streuung (Serien 40.7
bis 42.7 s, Erst-Lauf 64 bis 66 s, gleiche Transaktionszahl, alle Grenzwerte eingehalten). Der
Abstand zu den M3-Zahlen vom 19. Oktober betrifft beide Fassungen gleichermassen und kommt vom
Rechner an diesem Tag (laufende Instanz auf Port 8096, Docker, die e2e-Suite kurz davor), nicht
vom Scanner: die Titel-, Datei- und Transaktionszahlen sind identisch. Keine Regression durch
den Index je Titelart und das Lesen je Wurzel.
