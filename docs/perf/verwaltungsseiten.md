# Messwerte: Verwaltungsseiten (Konzept 12, ADR 0020)

Konzept `docs/planning/12-verwaltungsseiten.md`, Abschnitte 3, 11.4 und 11.7.

## Ausgangsmessung (K12-2a)

Abschnitt 3 des Konzepts: Höhe von der Oberkante der Seite bis zur ersten Arbeitszeile (Tabelle,
Liste oder Formular) auf drei Seiten bei 1440 px Fensterbreite, heute und nach dem Bau. **Ziel: höchstens
die Hälfte.** Diese Messung hält den Ausgangswert fest; K12-8 wiederholt sie mit eingeschalteter
kompakter Dichte.

### Methode

- Skript: `web/e2e-perf/admin-height.spec.ts` mit der Konfiguration `web/perf.config.ts`. Sie
  übernimmt Server und globales Setup der e2e-Suite (`web/e2e/start-server.mjs`, frische
  Konfiguration, Hauptkonto mit Hauptprofil, Bibliotheken aus `testdata/media`) und läuft nur in
  Chromium. Nicht Teil von `task ci`.
- Aufruf nach `task build` und `task testdata:media`, auf einem eigenen Port (8096 bleibt frei),
  nicht gleichzeitig mit `task e2e` laufen lassen (beide schreiben dieselbe Sitzung `owner.json`):

  ```bash
  cd web && SLOUCHCAT_E2E_PORT=8197 node_modules/.bin/playwright test -c perf.config.ts
  ```

- Fenster 1440 × 900 px, `deviceScaleFactor` 1, Schrift aus dem System (Linux, Chromium aus
  `@playwright/test` 1.64.1). Gemessen wird nach `document.fonts.ready` und `networkidle` die
  Oberkante des Arbeitselements, `getBoundingClientRect().top + scrollY`, gerundet auf ganze px.
- Erste Arbeitszeile je Seite:

  | Seite           | Pfad                  | Arbeitselement                                                      |
  | --------------- | --------------------- | ------------------------------------------------------------------- |
  | Hintergrundjobs | `/admin/jobs`         | Liste «Läuft gerade» (Panel unter der Überschrift, die erste Liste) |
  | Integrationen   | `/admin/integrations` | erstes Formular (TMDB-Schlüssel unter «Schlüssel der Anbieter»)     |
  | Konten          | `/admin/accounts`     | Tabelle «Konten», ab ihrer Kopfzeile                                |

- Datenstand: frischer e2e-Server, kein laufender und kein pausierter Job, keine Fehlermeldung.
  Hinweise mit Zustand (pausierte Jobs, Fehler) schieben die Arbeitszeile weiter nach unten und
  sind in dieser Messung nicht enthalten.

### Ergebnis

Gemessen am 2. November 2026 auf `main` 4c860bc mit K12-2a (Komponententokens mit heutigem Wert;
alle 212 Storybook-Bilder pixelgleich, die Seiten sehen also aus wie auf `main`), Server
`v1.7.0-1-g621b098`. Zwei Läufe hintereinander, beide mit denselben Werten:

```
Hintergrundjobs (/admin/jobs) at 1440 px: 476 px to the first work row
Integrationen (/admin/integrations) at 1440 px: 432 px to the first work row
Konten (/admin/accounts) at 1440 px: 353 px to the first work row
```

| Seite           | heute  | Ziel nach K12-8 (höchstens die Hälfte) |
| --------------- | ------ | -------------------------------------- |
| Hintergrundjobs | 476 px | ≤ 238 px                               |
| Integrationen   | 432 px | ≤ 216 px                               |
| Konten          | 353 px | ≤ 176 px                               |

Bei 900 px Fensterhöhe liegen heute rund 53 % (Hintergrundjobs), 48 % (Integrationen) und 39 %
(Konten) des sichtbaren Bereichs über der ersten Arbeitszeile.

## Statistik-Abfrage (K12-1)

Vertrag 1.8.0 ergänzt `GET /api/v1/admin/stats` um `watchTimeByQuarterHour` (672 Viertelstunden der
Woche in der Zeitzone des Servers) mit der neuen Abfrage `StatsWatchTimeByQuarter` über
`progress_events` (`SUM(played_ms)` je UTC-Viertelstunde, höchstens 90 Tage). Sie läuft als fünfter
Block neben den vier bisherigen; dazu kommen zwei kleine Abfragen für Poster und Hintergrundbilder
der höchstens 20 meistgesehenen Titel.

Gemessen mit `BenchmarkStatsMillionEvents` (`server/internal/admin/stats_bench_test.go`), gleicher
Bestand wie in `docs/perf/M8.md` (1'000'000 Fortschrittsereignisse über 400 Tage, 3 Profile,
50 Geräte, 5'000 Titel), am Service ohne HTTP, 40 Läufe je Zeitraum, Entwicklungsrechner
(Intel i7-14700K, 28 Threads), 2. November 2026, ohne andere Last:

```bash
cd server && go test -run '^$' -bench StatsMillionEvents -benchtime 40x ./internal/admin/
```

| Zeitraum | p50 vorher (`main` 4c860bc) | p95 vorher | p50 neu | p95 neu |
|---|---|---|---|---|
| 7 Tage | 25.3 ms | 29.3 ms | 36.8 ms | 41.2 ms |
| 30 Tage | 107.2 ms | 110.3 ms | 145.8 ms | 152.1 ms |
| 90 Tage | 296.0 ms | 303.8 ms | 394.2 ms | 404.3 ms |
| 365 Tage | 834.1 ms | 884.5 ms | 870.4 ms | 885.7 ms |

Die neue Abfrage allein (mit der Zuordnung zu Wochentag und Uhrzeit in Go):

| Fenster | p50 | p95 |
|---|---|---|
| 7 Tage | 6.5 ms | 6.7 ms |
| 30 Tage | 33.6 ms | 34.3 ms |
| 90 Tage | 102.0 ms | 105.2 ms |

Bewertung: Bis 90 Tage kostet das Raster etwa so viel wie die Abfrage allein (p95 +12 ms bei 7
Tagen, +100 ms bei 90 Tagen), obwohl die fünf Blöcke nebeneinander laufen; die Blöcke teilen sich
Speicher und Seiten-Cache des reinen Go-SQLite. Bei 365 Tagen bleibt das Raster bei 90 Tagen
Fenster und verschwindet hinter dem längsten Block (p95 unverändert). Mit dem Faktor 2.5 für
CT 190 aus M3 liegen 90 Tage bei rund 1 s, ein Jahr unverändert bei rund 2.2 s. Ein Haushalt mit
einigen tausend Ereignissen im Jahr bleibt weit darunter. Ein abdeckender Index
`(server_time, played_ms)` würde die Gruppierung verkürzen, braucht aber eine Migration und ist bei
diesen Werten nicht nötig.
