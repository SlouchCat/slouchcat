# Messwerte: Verwaltungsseiten (Konzept 12, ADR 0020)

Konzept `docs/planning/12-verwaltungsseiten.md`.

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
- Aufruf nach `task build` und `task testdata:media`, auf einem eigenen Port (8096 bleibt frei):

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
