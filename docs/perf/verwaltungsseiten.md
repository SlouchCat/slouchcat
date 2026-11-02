# Messwerte: Verwaltungsseiten (Konzept 12, ADR 0020)

Konzept `docs/planning/12-verwaltungsseiten.md`, Abschnitte 3, 11.4 und 11.7.

## Ausgangsmessung (K12-2a)

Abschnitt 3 des Konzepts: Höhe von der Oberkante der Seite bis zur ersten Arbeitszeile (Tabelle,
Liste oder Formular) auf drei Seiten bei 1440 px Fensterbreite, heute und nach dem Bau. **Ziel: höchstens
die Hälfte.** Diese Messung hält den Ausgangswert fest; K12-8 wiederholt sie mit eingeschalteter
kompakter Dichte (Abschnitt «Messung nach K12-8»).

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

- Ab K12-8 trägt das Arbeitselement in der Seite das Attribut `data-perf="first-work-row"` (die
  umgebauten Seiten haben keine stabile Struktur mehr für einen eigenen Locator); es markiert
  dasselbe Element wie die Tabelle oben: die Tabelle «Läuft gerade» (oder ihre Leerzeile), die
  erste Zeile der Anbieter-Schlüssel (TMDB) und die Tabelle «Konten».
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

## Messung nach K12-8

Gemessen am 2. November 2026 auf `claude/verwaltungsseiten` mit eingeschalteter kompakter Dichte
(K12-8), gleiche Methode und gleiches Skript wie oben (Chromium, 1440 × 900 px, Maus), zwei Läufe
hintereinander mit denselben Werten:

```
Hintergrundjobs (/admin/jobs) at 1440 px: 272 px to the first work row
Integrationen (/admin/integrations) at 1440 px: 259 px to the first work row
Konten (/admin/accounts) at 1440 px: 315 px to the first work row
```

Zum Vergleich die umgebauten Seiten in heutiger Dichte (dieselbe Seite, `data-shell` im Browser
entfernt): 350, 330 und 387 px.

| Seite           | Ausgang (K12-2a) | Umbau, heutige Dichte | Umbau, kompakt (K12-8) | Verhältnis | Ziel ≤ 50 % |
| --------------- | ---------------- | --------------------- | ---------------------- | ---------- | ----------- |
| Hintergrundjobs | 476 px           | 350 px                | 272 px                 | 57 %       | verfehlt    |
| Integrationen   | 432 px           | 330 px                | 259 px                 | 60 %       | verfehlt    |
| Konten          | 353 px           | 387 px                | 315 px                 | 89 %       | verfehlt    |

**Das Ziel «höchstens die Hälfte» wird auf keiner der drei Seiten erreicht.** Woraus die Höhe
besteht (kompakt, Werte aus dem Layout des Browsers):

| Teil                                         | Hintergrundjobs | Integrationen | Konten |
| -------------------------------------------- | --------------- | ------------- | ------ |
| App-Kopf (64 px) und Rand über dem Titel     | 96              | 96            | 96     |
| Kopfzeile mit h1 (24 px) und Abstand (24 px) | 56              | 56            | 56     |
| Zustandsleiste (S-4) und Abstand             | 80              | –             | 90     |
| Reiter «Konten / Einladungen» und Abstand    | –               | –             | 73     |
| Klappzeile «Läuft gerade» und Abstand        | 40              | –             | –      |
| Band «Schlüssel der Anbieter» mit Erklärsatz | –               | 107           | –      |
| **Summe**                                    | **272**         | **259**       | **315** |

- **Fester Anteil:** App-Kopf und Rand (96 px) ändert die Dichte nicht (Konzept 12, 5.2: der
  App-Kopf behält seine Masse). Er allein ist 20 % (Hintergrundjobs), 22 % (Integrationen) und
  27 % (Konten) der Ausgangshöhe.
- **Hintergrundjobs:** Die Zustandsleiste mit den fünf Zählern (56 px und 24 px Abstand) ist neu
  im Seitenmuster (S-4) und liegt über der Liste; ohne sie wären es 192 px (40 %).
- **Integrationen:** Im Band steht über den Schlüsseln der Erklärsatz «Für Metadaten (TMDB, TVDB),
  Bilder …» in 20 px (`Text` Variante `body`, 30 px hoch mit 12 px Abstand); ohne ihn wären es
  217 px (50.2 %).
- **Konten:** Die Seite hat seit dem Umbau eine Zustandsleiste mit vier Zählern (66 px, zwei mit
  Notiz) und Reiter (49 px) über der Tabelle, die es im Ausgang nicht gab; darum ist sie in
  heutiger Dichte 34 px höher als vorher (387 statt 353 px). Zähler und Reiter brauchen auch
  kompakt zusammen 163 px; die Dichte nimmt 72 px weg.

Die kompakte Dichte selbst wirkt wie geplant: 78, 71 und 72 px weniger gegenüber dem Umbau in
heutiger Dichte (Kopfzeile 32 statt 66 px, Abschnitte 24 statt 40 px, Steuerelemente 32 statt 44 px).
Was fehlt, liegt in den Seiten (Zustandsleiste, Reiter, Erklärsatz), nicht in der Dichte; ob die
Zähler über die Liste gehören oder der Erklärsatz in den Hilfe-Knopf, entscheidet Marco bei der
Abnahme (K12-9).

## Statistik-Abfrage (K12-1)

Vertrag 1.8.0 ergänzt `GET /api/v1/admin/stats` um `watchTimeByQuarterHour` (672 Viertelstunden der
Woche in der Zeitzone des Servers) mit der neuen Abfrage `StatsWatchTimeByQuarter` über
`progress_events` (`SUM(played_ms)` je UTC-Viertelstunde). Das Fenster `timeWindowDays` umfasst
ganze Wochen, damit jeder Wochentag gleich oft zählt: höchstens 90 Tage, abgerundet auf Vielfache
von 7 (7 → 7, 30 → 28, 90 und 365 → 84; Marcos Entscheid vom 2. November 2026). Die Abfrage läuft als
fünfter Block neben den vier bisherigen; dazu kommen zwei kleine Abfragen für Poster und
Hintergrundbilder der höchstens 20 meistgesehenen Titel.

Gemessen mit `BenchmarkStatsMillionEvents` (`server/internal/admin/stats_bench_test.go`), gleicher
Bestand wie in `docs/perf/M8.md` (1'000'000 Fortschrittsereignisse über 400 Tage, 3 Profile,
50 Geräte, 5'000 Titel), am Service ohne HTTP, 40 Läufe je Zeitraum, Entwicklungsrechner
(Intel i7-14700K, 28 Threads), 2. November 2026, ohne andere Last:

```bash
cd server && go test -run '^$' -bench StatsMillionEvents -benchtime 40x ./internal/admin/
```

| Zeitraum | p50 vorher (`main` 4c860bc) | p95 vorher | p50 neu | p95 neu |
|---|---|---|---|---|
| 7 Tage | 25.3 ms | 29.3 ms | 34.1 ms | 36.4 ms |
| 30 Tage | 107.2 ms | 110.3 ms | 142.6 ms | 146.8 ms |
| 90 Tage | 296.0 ms | 303.8 ms | 392.4 ms | 402.2 ms |
| 365 Tage | 834.1 ms | 884.5 ms | 860.4 ms | 869.7 ms |

Die neue Abfrage allein (mit der Zuordnung zu Wochentag und Uhrzeit in Go):

| Fenster | p50 | p95 |
|---|---|---|
| 7 Tage | 6.4 ms | 6.8 ms |
| 28 Tage | 30.9 ms | 31.8 ms |
| 84 Tage | 95.1 ms | 96.4 ms |

Bewertung der ganzen Antwort: Bis 90 Tage kostet das Raster etwa so viel wie die Abfrage allein
(p95 +7 ms bei 7 Tagen, +98 ms bei 90 Tagen), obwohl die fünf Blöcke nebeneinander laufen; die
Blöcke teilen sich Speicher und Seiten-Cache des reinen Go-SQLite. Bei 365 Tagen bleibt das Raster
bei 84 Tagen Fenster und verschwindet hinter dem längsten Block (p95 unverändert im Rahmen der
Streuung). Mit dem Faktor 2.5 für CT 190 aus M3 liegen 90 Tage bei rund 1 s, ein Jahr unverändert bei
rund 2.2 s.

**Bestand.** Der Player meldet alle 10 s (`web/src/features/player/useProgressReporter.ts`), also
rund 360 Ereignisse je Wiedergabestunde. 3 Stunden am Tag ergeben in 84 Tagen rund 91'000
Ereignisse. Der Bestand der Messung (1 Mio. in 400 Tagen, rund 210'000 in 84 Tagen) entspricht rund
7 Stunden am Tag: realistisch, aber hoch.

**Schwelle aus Konzept 12, 11.10** («über 200 ms rechnet der Server die Stundenwerte höchstens alle
5 min neu»). Sie gilt für die Rasterabfrage allein im längsten Fenster (84 Tage):

| Bestand | Entwicklungsrechner p95 | CT 190 geschätzt (Faktor 2.5) |
|---|---|---|
| Messung, rund 210'000 Ereignisse in 84 Tagen | 96 ms | rund 240 ms |
| Haushalt mit 3 Stunden am Tag, rund 91'000 Ereignisse | rund 42 ms (anteilig) | rund 105 ms |

Beim Bestand der Messung läge CT 190 über der Schwelle, beim realen Bestand darunter. Der Entscheid
fällt mit der Messung von `GET /admin/stats?days=90` auf CT 190 in K12-9; bis dahin gibt es keinen
Zwischenspeicher. Liegt der p95 dort über 200 ms, gilt als Ausweg:

- **Zwischenspeicher nur für das Raster:** Schlüssel ist das Fenster (`timeWindowDays`), Gültigkeit
  5 min, eine Berechnung zur Zeit (singleflight); die übrigen Blöcke bleiben frisch.
- **Abdeckender Index `(server_time, played_ms)`:** würde die Gruppierung verkürzen, braucht aber eine
  Migration und ist nicht geplant.
