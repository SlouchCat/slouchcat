# CI-Optimierungsplan (Vorschlag zur Freigabe)

Stand: 28. November 2026, main `1220769b`. Zweig `claude/ci-optimierung`. **Nichts davon ist umgesetzt;**
kein Workflow wurde geändert. Jede Massnahme braucht Marcos Freigabe und wird danach einzeln, messbar
und mit einem Revert umkehrbar umgesetzt (Abschnitt 6).

Grundlage sind Messungen aus der GitHub-API und den Job-Logs, keine Schätzungen. Geschätzte Werte sind als
**geschätzt** gekennzeichnet, mit Rechenweg. Quellen (lokal, nicht im Repo):
`.omc/research/ci-optimierung/` im Hauptordner mit `MESSWERTE.md` (505 Läufe, 9424 Jobs, 29.10. bis
28.11.2026), `FEHLSCHLAEGE.md` (alle 101 roten Job-Logs), `SICHERHEIT.md`, den Berichten von
`ci-speedup` und `ci-score` und den Skripten (`skripte/holen.sh`, `skripte/auswertung.py`,
`skripte/prtypes.py`).

Ären: **E3** ist die aktuelle Konfiguration (seit 26.11. 11:12 UTC: Android in zwei Hälften,
`store` in drei Gruppen). Sie hat erst 6 volle grüne PR-Läufe; ihre Mediane sind vorläufig. E2 ist
22. bis 26.11., E0 das ganze Fenster.

## 1. Ausgangslage (gemessen)

| Kennzahl | E3 (n) | E2 (n) | E0 (n) |
|---|---|---|---|
| Wartezeit voller PR-Lauf bis Gate `ci`, Median / P95 | **15,2 / 16,5 min** (6) | 24,2 / 28,7 (27) | 14,3 / 26,4 (161) |
| main voll, Median / P95 | 13,9 / 16,2 (5) | 23,7 / 26,5 (20) | 13,6 / 24,3 (127) |
| main übernimmt PR-Ergebnis, Median | 5,5 min (2; Android läuft dort weiter) | 0,6 (10) | 0,5 (26) |
| PR nur Doku, Median | 0,4 min (3) | 0,4 (7) | 0,3 (65) |
| Runner-Minuten je vollem PR-Lauf | **188,0** (174,2 ohne Emulator-Job) | | 130,8 |
| Längster Pol | **e2e-Pfad in 11 von 11** | server-test 55 % | e2e 54 %, server-test 43 % |
| Abstand längster zu zweitlängstem Pol, Median / P95 | **5,0 / 6,7 min** | 5,1 / 9,7 | 3,7 / 8,5 |
| Wartezeit bis Job-Start, Median / P95 | 2 s / 3 s | | 2 s / 3 s |

Weiter gemessen (E0, wo nicht anders genannt):

- Ende je Pol in E3 (Median ab Laufbeginn): e2e-Pfad 14,2 (build 1,2 + längster Shard 12,7), server-test
  9,3, web 8,1 (P95 11,9), android 8,0 (P95 10,7), storybook 6,3, android-contract 5,4, image 3,8.
- Einrichtung ist nicht der Engpass: 11 % der Jobzeit bei e2e, 15 % bei server-test, 17–19 % bei
  android. Die Nutzlast (der eigentliche `task`-Aufruf) dominiert.
- 505 Läufe: 400 grün, 61 rot (8426 Runner-Min), 44 abgebrochen (4173 Runner-Min, 7,4 %). Überholte
  Läufe werden sauber abgebrochen: 0 Läufe liefen trotz neuerem Push zu Ende.
- 22 Läufe mit Neustart (4,4 %), davon 17 ohne Codeänderung grün.
- PR nur Doku: 23 % der PR-Läufe (65 von 279), main-Übernahme: 33 % der main-Pushes seit 16.11.
  (20 von 60).
- Woche ab 23.11.: 16 157 Runner-Min (Woche davor 6969; Android-Jobs kamen dazu).
- Actions-Cache: **10,41 von 10 GiB**, 114 Einträge, alle auf `main`. Am 28.11. fehlten setup-go,
  `apt-debs-noble-server` und `apt-debs-noble-android-e2e`, die Läufe bis 08:56 UTC noch trafen.
- `ci-score`: 67/100 (6 von 9 Prüfungen). `ci-secure`: 0 von 10 Angriffswegen getroffen, alle 10
  Fremd-Pins verifiziert. `actionlint` 1.7.12: 0 Meldungen (ohne shellcheck). `zizmor` 1.33.1
  offline: 5 hoch, 17 mittel, 28 niedrig; bewertet als Fehlalarme oder niedrig, echte Punkte in H2.
- PostgreSQL und Redis gibt es im Stack nicht (SQLite über `modernc.org/sqlite`, kein `services:`).

## 2. Priorisierte Massnahmen (Übersicht)

| # | Massnahme | Wirkt auf | Einsparung | Basis | Aufwand | Risiko |
|---|---|---|---|---|---|---|
| M1 | main-Übernahme: informativen Emulator-Job nicht mitzählen, Lauf an Basis `main` binden | Runner-Min, main-Ergebnis früher | 1800–4400 Runner-Min/Monat; main-Ergebnis nach 5,5 statt 13,9 min | Zähler gemessen, Minuten abgeleitet | S | gering |
| M2 | apt-Hänger zeitlich begrenzen | Zuverlässigkeit | Hänger rot nach ≤ 10 statt 30–47 min; ~600 Runner-Min/30 Tage | gemessen 17 Hänger, Ersparnis geschätzt | S | gering |
| M3 | Fehlerlog-Leseweg für Agenten, Gate-Zusammenfassung | Tokens | −98 % Tokens je rotem Job (88 800 → 1 700, Median) | gemessen | S–M | gering |
| M4 | e2e: Dauern auffrischen, Shard-Zahl auf dem Runner messen und setzen | **PR-Wartezeit** | −2,5 bis −3,7 min (Ziel ≈ 11,5 min) | geschätzt aus Messwerten | M | mittel |
| M5 | Web-Job in Lint und Unit-Tests teilen; Einzelschritte in web und server-lint | PR-Wartezeit (nach M4), Rückmeldung | web-Pol −2 bis −3,5 min | geschätzt aus Log-Zeiten | S | gering |
| M6 | Cache-Budget unter 9 GiB | Zuverlässigkeit | verhindert Fehlschläge (23.11.: 60 von 287 e2e-Jobs ohne Browser-Cache) | gemessen Vorfälle, Ersparnis geschätzt | M | gering |
| M7 | Workflow-Selbstprüfung (actionlint, Gate-Liste, zizmor), doppelte Android-Blöcke zusammenfassen | Wartbarkeit, Sicherheit | 0 Laufzeit | – | S | gering |
| M8 | Instabile Tests beheben, je Test eine Aufgabe | Zuverlässigkeit | je vermiedenem Neustart ≈ 15 min und 188 Runner-Min | gemessen | M je Test | gering |
| M9 | Change Detection je Job (Pfad → Jobs) | PR-Wartezeit für 21 % der Code-PRs, Runner-Min | nur Android 14,9 → 8,7 min, nur Deploy 14,9 → 4,5 min | Anteile gemessen, Zeiten geschätzt | L | **mittel–hoch, Marcos Entscheid** |
| M10 | `server tests (1)` (`internal/api`) teilen | PR-Wartezeit, erst wenn Pol | −3 bis −4 min für diesen Pol | geschätzt | S | gering |
| H1–H3 | Härtung Release-Pfad und Repo-Einstellungen | Sicherheit | 0 Laufzeit | – | S–M | – |

Reihenfolge der Umsetzung: Welle 1 M1, M2, M3, M7 (klein, unabhängig) · Welle 2 M4, dann M5 · Welle 3 M6 ·
laufend M8 · nach Entscheid M9, M10, H1, H2.

## 3. Massnahmen im Einzelnen

### M1 main-Übernahme: Emulator-Job ausnehmen, an Basis `main` binden

| Feld | Inhalt |
|---|---|
| Zustand | `.github/scripts/already-tested.sh` übernimmt das Ergebnis eines grünen PR-Laufs, wenn **jeder** seiner Jobs `success` meldet. Dazu zählt `android-emulator (flow, informational)`, der `continue-on-error` hat und nicht im Gate steht. Der PR-Lauf wird nur über Kopf-SHA und Ereignis zugeordnet. |
| Problem | Ein roter informativer Job verhindert die Übernahme; main läuft dann voll, obwohl das Gate denselben Job nicht verlangt. Ein grüner PR-Lauf desselben SHA gegen einen anderen Zielzweig zählt mit. |
| Nachweis | Seit 16.11. 39 volle main-Läufe. Gründe (Prüfreihenfolge des Skripts): 10× Emulator im PR-Lauf rot, 12× PR-Kopf hinter main, 16× PR nur Doku (Absicht, ADR 0013), 1× unerklärt (Lauf 45844593181). 18 grüne PR-Läufe tragen einen roten Emulator-Job, z. B. 45987897414, 45253038403, 45216383177. In E3 kostete die Übernahme 12,5 statt 186,2 Runner-Min und endete nach 5,5 statt 13,9 min. |
| Lösung | Im Skript die Jobs ausnehmen, die das Gate nicht prüft (eine Liste an einer Stelle, die Gate und Skript lesen), und deren Ergebnis im Gate-Protokoll nennen. Nur PR-Läufe zählen, deren `pull_requests[].base.ref` `main` ist (Fork-PRs haben keine Liste und laufen voll). Test mit gefälschtem `gh` und Fixtures. |
| Laufzeit | Auf main-Läufe wartet niemand. Das main-Ergebnis liegt nach rund 5,5 statt 13,9 min vor (gemessen, E3, n = 2 und 5). |
| Runner-Min | **geschätzt** 1800–4400 je Monat: 10 verhinderte Übernahmen in 12 Tagen (gemessen) × 174 Runner-Min je Lauf (abgeleitet); untere Grenze mit der Rotquote des Emulators in E3 (17 %). |
| Aufwand | S (halber Tag) |
| Risiken | Ein Fehler, den nur der Emulator-Job findet, zeigt sich auf main nicht ein zweites Mal. Das Gate verlangt diesen Job schon heute nicht; das Risiko ist nicht neu. |
| Sicherheit / Abdeckung | Keine Prüfung fällt weg. Die Basisbindung schliesst eine Zuordnungslücke. |
| Akzeptanz | (1) Skripttest: alle grün ausser Emulator rot → `true`; ein Gate-Job rot → `false`; Basis nicht main → `false`; leere `pull_requests` → `false`. (2) Von den nächsten 10 Merges eines Zweigs auf aktuellem main mit grünem PR-Lauf werden 10 übernommen. (3) Das Gate-Protokoll nennt das Emulator-Ergebnis des übernommenen Laufs. |

### M2 apt-Hänger begrenzen

| Feld | Inhalt |
|---|---|
| Zustand | `.github/actions/system-packages` ruft `sudo apt-get update` und `install` ohne eigene Zeitgrenze; es gilt nur die Jobgrenze (30–45 min). Der .deb-Cache greift (0 MB Download in den ausgewerteten Logs). |
| Problem | Hängt ein Spiegel, läuft der Job bis zur Jobgrenze und meldet dann rot, ohne dass ein Test fiel. |
| Nachweis | 17 Jobs hingen bis zum Zeitlimit in `system-packages`, in 6 Läufen und 3 Zeitfenstern (11.11. Lauf 43062819414 mit 8 Jobs, 15.11., 23.11. mit 4 Läufen), dazu 1 Emulator-Job: zusammen 736 Runner-Min. Am 23.11. endete ein Lauf nach 47 min rot. Normalfall: Schritt 26–33 s (E3, P95 bis 48 s, E1 max 289 s). |
| Lösung | `timeout` um `apt-get update` und `apt-get install` (4 min je Aufruf, über dem gemessenen Maximum), dazu `Acquire::Retries "3"` und `Acquire::http::Timeout "30"` in der vorhandenen Drop-in-Datei. Kein Wiederholen von Tests. |
| Laufzeit | Normalfall 0. Hängerfall: rot nach höchstens rund 10 min statt 30–47 min (**geschätzt**). |
| Runner-Min | **geschätzt** −600 im 30-Tage-Fenster (17 Hänger × rund 30 min); hängt vom Spiegel ab. |
| Aufwand | S |
| Risiken | Ein langsamer, aber funktionierender Spiegel kann rot statt langsam enden. Darum liegt die Grenze über dem gemessenen Maximum. |
| Sicherheit / Abdeckung | Unverändert; apt prüft die Pakete weiter gegen den signierten Index. |
| Akzeptanz | In 30 Tagen endet kein Job am Job-Zeitlimit in `system-packages`; Median des Schritts ±5 s gegen heute; `apt-config dump` im Log zeigt die Werte. |

### M3 Fehlerlog-Leseweg für Agenten

| Feld | Inhalt |
|---|---|
| Zustand | `ci.yml` schreibt keine Job-Zusammenfassungen. Annotationen tragen ausser beim Lint nur den Exit-Code. Agenten lesen ganze Logs. `gh run view --log-failed` liefert mit gh 2.45.0 0 Byte (10 von 10 Läufen). |
| Problem | Ein roter Job kostet beim Lesen den grössten Teil des Kontexts, obwohl wenige Zeilen die Ursache tragen. |
| Nachweis | 101 rote Job-Logs: Median je Job 88 800 Tokens ganzes Log, 4 300 Tokens letzte 150 Zeilen, 1 700 Tokens gezielter Auszug. In 10 von 10 Referenzläufen enthielt der Auszug die Ursache; die Testnamen standen in 52 von 52 Fällen in den letzten 150 Zeilen. Bei e2e ist der rote Schritt 96–98 % des Logs. |
| Lösung | (a) `scripts/ci-failures.sh <run>` mit Task-Ziel: rote Jobs und Schritte über `gh api …/jobs` (rund 200 Byte), belegten Flake melden (derselbe SHA später grün), Log nur auf Platte, Auszug je Werkzeug (Go `--- FAIL`/`panic:`, Playwright-Fehlerblöcke, Vitest, Gradle `FAILED`/`What went wrong`, golangci-lint, Storybook `snapshot doesn't exist`), höchstens 200 Zeilen; ohne Treffer die letzten 150 Zeilen. (b) Das Gate schreibt seine Ergebnistabelle nach `$GITHUB_STEP_SUMMARY`. (c) Der Skill `slouchcat-ci` schreibt den Weg vor. |
| Laufzeit | 0 |
| Runner-Min | 0 (Zusammenfassung unter 1 s) |
| Tokens | **gemessen** −98 % je rotem Job (Median 88 800 → 1 700) |
| Aufwand | S–M |
| Risiken | Ein Filter verpasst eine neue Fehlerform; dann greift der Rückfall auf die letzten 150 Zeilen. |
| Sicherheit / Abdeckung | Keine Änderung an Prüfungen. Logs bleiben nicht vertrauenswürdige Daten; das Skript gibt keine token-artigen Zeichenketten aus. |
| Akzeptanz | Für die 10 Referenzläufe aus `FEHLSCHLAEGE.md` Abschnitt 4: Ausgabe je Lauf ≤ 3000 Tokens und nennt den fehlgeschlagenen Test oder Befund (10 von 10). Skripttests mit gespeicherten, bereinigten Auszügen. |

### M4 e2e: Dauern auffrischen und Shard-Zahl messen

| Feld | Inhalt |
|---|---|
| Zustand | 6 Shards, geplant nach `web/e2e/durations.json` (zuletzt 12.11., `080f5eb4`). 19 von 106 Spec-Dateien fehlen darin und werden mit dem Median ihres Projekts geplant. ADR 0013 begründete «sechs» am 3.11. mit rund 6 min je Shard und server-test als kritischem Pfad. |
| Problem | Heute laufen die Shards 10,6–12,4 min, und der e2e-Pfad ist der einzige lange Pol. |
| Nachweis | e2e-Pfad in 11 von 11 vollen Läufen der längste Pol, Ende Median 14,2 / P95 15,9 min. Der zweitlängste Pol (server-test) endet 5,0 min früher (P95 6,7). Shards: Median 10,6–12,4 min, Verhältnis 1,17. Shard 4 in Lauf 45977212910: Plan 406 s, Testzeit rund 640 s. Einrichtung je Shard 1,2 min. |
| Lösung | Stufe a: `task web:e2e:durations RUNS="…"` mit drei grünen E3-Läufen. Stufe b: 6, 7, 8 und 9 Shards auf dem Runner messen, mit einem Wegwerf-Workflow auf eigenem Zweig (je Variante zwei Wiederholungen). Danach Wahl, ADR-0013-Nachtrag, `docs/perf/ci.md`. |
| Laufzeit | **geschätzt** Stufe a ≤ 1 min; Stufe b −2,5 min (7 Shards) bis −3,7 min (8 Shards), Wartezeit ≈ 11,5 min. Rechnung: Nutzlast aller Shards zusammen 58,9 min (Summe der gemessenen Mediane); je Shard Nutzlast/N × 1,1 Unwucht + 1,5 min fest; dazu build 1,2 und Gate 0,7. Untergrenze: server-test 9,3, web 8,1 (P95 11,9). Mehr als 8 Shards bringt deshalb wenig. |
| Runner-Min | **geschätzt** +1,5 je zusätzlichem Shard (8 Shards: +3, +1,6 %). |
| Aufwand | M (eine Messrunde von rund einer Stunde Runnerzeit, die Änderung selbst ist klein) |
| Risiken | Der feste Anteil je Shard (Einrichtung, `globalSetup` mit Bibliotheksscans) wächst mit der Zahl. Eine neue Verteilung kann versteckte Abhängigkeiten zwischen Specs zeigen; das ist dann ein Befund zum Beheben, kein Grund zum Überspringen. |
| Sicherheit / Abdeckung | Gleiche Testmenge. `task web:e2e:shards:check` prüft, dass die Shards zusammen genau alle Tests ergeben. |
| Akzeptanz | (1) `shards:check` grün, Testzahl unverändert. (2) Über mindestens 10 volle PR-Läufe: Median Wartezeit ≤ 12,0 min, P95 ≤ 13,5 min. (3) Runner-Min je vollem PR-Lauf ≤ 193. (4) Keine Zunahme der «flaky»-Meldungen. |

### M5 Web-Job teilen, Einzelschritte

| Feld | Inhalt |
|---|---|
| Zustand | `task ci:web` läuft als ein Schritt: gen, gen:check, lint (ESLint, stylelint, tsc, prettier), test (Vitest), shards:check, heavy-slot:check. Der erste Fehler bricht ab. `server-lint` führt gen, gen:check und lint in einem Schritt aus. |
| Problem | Lint und Vitest warten aufeinander; eine Format-Abweichung verdeckt das Vitest-Ergebnis. |
| Nachweis | In 6 Logs: lint 126–214 s, Vitest 239–412 s, der Rest unter 25 s. Web-Job Median 7,8 / P95 11,6 min. Nach M4 wäre web bei P95 (11,9 min) der längste Pol. |
| Lösung | Job `web` als Matrix in zwei Hälften wie `android`: Lint mit den kleinen Prüfungen, Unit-Tests. `task ci:web` bleibt die Summe (ADR 0013: CI ruft nur `task`-Ziele). In `server-lint` gen, gen:check und lint als drei Schritte mit `if: success() \|\| failure()`. |
| Laufzeit | **geschätzt** web-Pol −2 bis −3,5 min (Ende ≈ 0,5 min Einrichtung + Vitest 4,0–6,9 min). Wirkt nur, wenn web der längste Pol ist. |
| Runner-Min | **geschätzt** +0,5 je Lauf (zweite Einrichtung, gemessen 0,4 min) |
| Aufwand | S |
| Risiken | Gering. Gate und `needs` bleiben gleich, die Matrix hat einen Jobnamen. |
| Sicherheit / Abdeckung | Gleiche Prüfungen; jede meldet einzeln. |
| Akzeptanz | Web-Jobs enden in ≥ 95 % der vollen Läufe vor dem e2e-Pfad. Eine rote Lint-Prüfung zeigt das Vitest-Ergebnis trotzdem. |

### M6 Cache-Budget

| Feld | Inhalt |
|---|---|
| Zustand | 114 Einträge, 10,41 von 10 GiB; GitHub verdrängt die ältesten. `gradle-Linux` (2,30 GiB) enthält ein altes `build-cache-1`, das `gradle-build-*` überdeckt. `gradle-e2e` (0,47 GiB) dient nur `android-contract`. Der apt-Schlüssel enthält `pnpm-lock.yaml` auch für Gruppen ohne Playwright. |
| Problem | Die Verdrängung trifft wahllos, und ein fehlender Cache macht Läufe langsamer und anfälliger. |
| Nachweis | 28.11.: setup-go, apt-server und apt-android-e2e verdrängt (bis 08:56 UTC noch Treffer). 23.11.: Playwright-Browser in 60 von 287 e2e-Jobs nicht im Cache. test-media seit 16.11. in 4,8 % der server-test-Jobs neu erzeugt (bis 184 s). |
| Lösung | (a) `gradle-Linux` ohne `build-cache-1` (`!~/.gradle/caches/build-cache-1`), neuer Schlüssel. (b) `android-contract` liest `gradle-Linux` nur, wie der Emulator-Job; `gradle-e2e` entfällt. (c) apt-Schlüssel nur bei der Gruppe mit Playwright über `pnpm-lock.yaml`. (d) Zielbudget ≤ 9 GiB mit Budgettabelle in `docs/perf/ci.md`; kein neuer Cache ohne Budgetzeile. Grössen vor der Änderung messen. |
| Laufzeit | **geschätzt** im Normalfall 0. Verhindert Rückfälle: Browser-Download je Shard (8,3 min im alten Lauf 39789916437, heutige Dauer messen), test-media bis 3 min. |
| Runner-Min | **geschätzt** klein; der Nutzen ist Stabilität. |
| Aufwand | M |
| Risiken | Der neue Gradle-Schlüssel kostet einen kalten Lauf. `android-contract` mit dem grösseren Cache: Dauer messen (Ende heute 5,4 min, kein Pol). |
| Sicherheit / Abdeckung | Kein neuer Schreiber; nur main speichert. Keine Prüfung betroffen. |
| Akzeptanz | Belegung ≤ 9,0 GiB an 7 Tagen nacheinander; jede Cache-Familie aus `MESSWERTE.md` Abschnitt 11 im Inventar; Browser-Installation in ≥ 98 % der e2e-Jobs übersprungen. |

### M7 Workflow-Selbstprüfung und doppelte Blöcke

| Feld | Inhalt |
|---|---|
| Zustand | Workflow-Dateien werden nicht geprüft. Die Gate-Liste wird von Hand gepflegt (ADR 0013, stehende Regel). Kein Git-Hook. JDK/SDK-Schritt, Gradle-Wrapper-Prüfung mit SHA-256 und Gradle-Cache-Schlüssel stehen je dreimal in `ci.yml` und müssen gleich bleiben. |
| Problem | Ein vergessener Job in der Gate-Liste bleibt unbeobachtet; drei Kopien laufen auseinander. |
| Nachweis | actionlint 0 Meldungen, zizmor offline 5 hoch / 17 mittel / 28 niedrig (bewertet: Fehlalarme oder niedrig). |
| Lösung | `task ci:workflows`: actionlint als `go tool` gepinnt, Selbsttest «jeder Job steht in `needs` des Gates und in dessen Prüfliste», zizmor offline (gepinnte Fassung, lokal; in CI nach Entscheid). Aufruf in `server-lint` und in `task check:fast`, wenn `.github/` geändert ist. Composite `.github/actions/android-sdk` mit Wrapper-SHA in einer Datei. Eingaben der Composite-Actions über `env:` statt direkt in `run:`. |
| Laufzeit | 0 (Sekunden in `server-lint`, kein Pol) |
| Runner-Min | ≈ 0 |
| Aufwand | S |
| Risiken | zizmor meldet nach Updates neue Muster; die bewerteten Funde werden als Basis festgehalten. |
| Sicherheit / Abdeckung | Mehr Prüfung, nicht weniger. |
| Akzeptanz | Der Selbsttest wird rot, wenn ein Job in der Gate-Liste fehlt (Probe mit einer Kopie von `ci.yml`). actionlint 0 Meldungen. |

### M8 Instabile Tests

| Feld | Inhalt |
|---|---|
| Zustand | Playwright `retries: 1` weist Flakes als «flaky» aus. Rote Läufe werden von Hand neu gestartet. |
| Problem | Jeder Neustart kostet eine volle Runde Wartezeit und Runner-Minuten. |
| Nachweis | 22 Läufe mit Neustart, 17 davon ohne Codeänderung grün. Rangliste: `hdr.spec.ts` (Start unter 5 s) 9× rot auf 4 Zweigen, 3× auf demselben SHA später grün; `AudioServiceTest.headphonesPulledOutPauseTheSound` 5×; `admin-library-folders.spec.ts` 5× flaky; `episode.spec.ts` und `audiobook.spec.ts` je 4×; `LoginPage.test.tsx` (react-aria-Timer) 3×; `admin-integrations`, `metadata-editor`, `admin-wishes` je 3×. Emulator-Job im Fenster 53 rot, 33 grün (E3: 3 von 18 rot). |
| Lösung | Je Test eine eigene Aufgabe, Ursache im Test oder in der App beheben. Nie überspringen, Wiederholungen erhöhen oder Zeitlimits verlängern. |
| Laufzeit | je vermiedenem Neustart ≈ 15 min (gemessen, Median voller PR-Lauf E3) |
| Runner-Min | je vermiedenem Neustart ≈ 188 (gemessen) |
| Aufwand | M je Test |
| Risiken | Keine für die CI. |
| Sicherheit / Abdeckung | Abdeckung bleibt; die Tests werden verlässlicher. |
| Akzeptanz | Je behobenem Test 0 Fehlschläge in den nächsten 30 Läufen; Neustarts unter 2 % der Läufe. |

### M9 Change Detection je Job (Entscheid Marco)

| Feld | Inhalt |
|---|---|
| Zustand | Der Job `changes` kennt nur «nur Doku» und «schon geprüft». Jeder Code-PR führt alle Jobs aus. |
| Problem | Ein reiner Android- oder Deploy-PR wartet auf den e2e-Pfad, den er nicht berührt. |
| Nachweis | 185 gemergte PRs seit 29.10. (`skripte/prtypes.py`, Karte aus der Strukturanalyse): 53 nur Doku (29 %), 32 mit mehr als 100 Dateien (zählen als «alles»), 16 nur Deploy-Dateien (Versionsstände in `deploy/compose.yaml`, `.env.example`, README), 12 nur Android, der Rest Server, Web oder gemischt. Server- und Web-Änderungen brauchen immer den e2e-Pfad; schneller wären 28 von 132 Code-PRs (21 %). |
| Lösung | `changes` klassifiziert mit einem Skript (wie `docs-only.sh`, keine Fremdaktion) nach einer Karte Pfad → Jobs und gibt je Job `run_<job>` aus. Das Gate zählt «skipped» nur bei `changes = success` und `run_<job> = false` als grün. Unbekannte Pfade, `.github/**`, `Taskfile*`, Lockfiles, `api/` und `.tool-versions` lösen alles aus. Ein Kartentest hält die Karte gegen die gelesenen Dateien der Go-Tests, die Gradle-Verweise und den pnpm-Graphen. Volle Regression: Jeder main-Push nach einem gefilterten PR läuft voll, weil `already-tested.sh` übersprungene Jobs nie übernimmt; dazu der Wochenlauf. Variante: main führt nur die im PR übersprungenen Jobs aus. |
| Laufzeit | **geschätzt** nur Android 14,9 → 8,7 min, nur Deploy 14,9 → 4,5 min; über alle Code-PRs Mittel 14,9 → 13,1 min, Median unverändert. |
| Runner-Min | **geschätzt** PR-Läufe im Mittel 188 → 138 (−27 %). Dagegen läuft main nach gefilterten PRs voll statt übernommen. Netto offen, nach Einführung messen. |
| Aufwand | L |
| Risiken | Eine fehlende Kante in der Karte zeigt einen Bruch erst auf main nach dem Merge (kein automatischer Revert). Deshalb nur mit Kartentest und Rückfall auf «alles». |
| Sicherheit / Abdeckung | Kein Pfadfilter auf Workflow-Ebene: Das Gate läuft immer. Fork-PRs und Änderungen an der Prüfung selbst laufen voll. |
| Akzeptanz | Kartentest grün; Tabellentest mit 30 echten PRs; nach 4 Wochen 0 Brüche auf main durch eine fehlende Kante, sonst Rückbau. |

### M10 `server tests (1)` teilen (bedingt)

| Feld | Inhalt |
|---|---|
| Zustand | Gruppe 1 ist allein `internal/api`, ein Test-Binary. |
| Problem | Wird zum nächsten Pol, sobald der e2e-Pfad unter 9,3 min fällt. |
| Nachweis | Job Median 9,0 min, Testschritt 7,7 min, davon `internal/api` 455 s in einem Binary (Lauf 46128085568). Längstes zu kürzestem Bein 2,47. |
| Lösung | In `server/testgroups.txt` nach Testnamen teilen, wie `internal/store` am 26.11. (`--check` zählt jeden Test in genau einer Gruppe). Erst nach M4 und M5 messen, ob server-test dann der Pol ist. |
| Laufzeit | **geschätzt** −3 bis −4 min für diesen Pol |
| Runner-Min | **geschätzt** +1 (eine Einrichtung mehr) |
| Aufwand | S |
| Risiken | Gering; dasselbe Verfahren wie bei `store`. |
| Sicherheit / Abdeckung | Gleiche Tests, `-race` bleibt. |
| Akzeptanz | `testgroups.sh --check` grün, Testzahl gleich; server-test-Pol endet ≤ 7 min. |

## 4. Härtung (ohne Laufzeitwirkung)

Die Einzelheiten stehen im lokalen Bericht `SICHERHEIT.md`, nicht im öffentlichen Repo. Der Scan fand
keinen Angriffsweg für Fork-PRs; die Punkte betreffen Konten mit Schreibrecht und den Release-Pfad.

| # | Massnahme | Wer | Aufwand |
|---|---|---|---|
| H1 | Repo-Einstellungen: Ruleset für main (PR-Pflicht, Pflicht-Check `ci`, kein Force-Push und kein Löschen), Ruleset für die Release-Tags, Freigaberegel mit Tag-Bindung für die Umgebung `android-release` **bevor** dort Signier-Secrets liegen, eine Umgebung mit Freigabe für die Release-Jobs mit Schreibrechten, `CODEOWNERS` für `.github/`, Secret Scanning und Push Protection. Der Pflicht-Check `ci` verträgt sich mit allen Abkürzungen, weil das Gate immer läuft. | nur Marco (Einstellungen) | 30–45 min |
| H2 | `release.yml`: Bau und Veröffentlichung in zwei Jobs mit eigenen Rechten (wie `release-android.yml`), Tag-Name über `env:`, Prüfung «Tag liegt auf main», Release-Builds ohne Cache aus der CI. Bauzeit vorher messen (heute Median 3,5 min). | Agent nach Freigabe, Probe-Tag | 1–2 h |
| H3 | Eingaben der Composite-Actions über `env:` (Teil von M7). | Agent | S |

## 5. Geprüft und verworfen

| Idee | Grund |
|---|---|
| PostgreSQL- und Redis-Testumgebungen | gibt es nicht; SQLite in-process |
| Pfadfilter auf Workflow-Ebene (ci-score-Empfehlung) | würde das Gate auslassen; Change Detection gehört in den Lauf (M9) |
| Turbo/Nx-Remote-Cache, grössere oder selbst gehostete Runner | Dienst oder Kosten; selbst gehostet am 25.10. von Marco verworfen (Fork-Code auf eigener Hardware) |
| Merge Queue | volle Regression vor dem Merge wäre möglich, verlangt aber Branch-Schutz; nur auf Marcos Wort, später neben H1 |
| `pull_request_target`, `workflow_run` | keine Beschleunigung, mehr Angriffsfläche |
| ESLint `--cache` in CI | typbewusste Regeln; der Cache erkennt Typänderungen in anderen Dateien nicht (falsch grün) |
| TypeScript `--incremental` | tsc ist ein Teil des Lint-Schritts (126–214 s gesamt), web ist nach M5 kein Pol; Nutzen klein |
| Storybook-Build- oder Playwright-Image-Cache | Cache-Budget reicht nicht (M6) |
| `fail-fast` in den Matrizen | verdeckt Ergebnisse anderer Beine; volle Regression je Lauf gewollt |
| Draft-PRs ohne schwere Jobs (ci-speedup OPT33) | PRs werden nicht als Entwurf geöffnet; spart nur Runner-Min |
| `-race` für `modernc.org` abschalten | Lockerung; Marco am 26.11. abgelehnt |
| Binary aus `build` an `android-contract` und Emulator | contract ist kein Pol (Ende 5,4 min); zusätzliche `needs`-Kante kostet 1,2 min Start |
| apt-Index cachen, `apt-get update` weglassen | rund 20 s je Job, aber Signaturprüfung und veraltete Indizes; Risiko höher als Nutzen |
| ci-speedup-Funde OPT6, OPT21, OPT24, OPT40, OPT61 | im Code geprüft: Fehlalarme oder gewollt (z. B. hat `ci.yml:362` `restore-keys`, server-test ist schon eine Matrix) |
| Warteschlange (ci-speedup OPT43, P90 bis 3342 s) | eigene Messung: Wartezeit bis Start Median 2 s, P95 3 s, max 143 s; nicht bestätigt |
| `act` lokal | 34 runner-spezifische Stellen (JDK/SDK-Variablen, KVM, sudo apt, QEMU, gha-Cache), 15 GB freier Speicher; nicht installiert |

## 6. Vorgehen bei der Umsetzung

1. Je Massnahme ein Zweig und ein PR; kleine Massnahmen einer Welle dürfen in einen PR, wenn ihre
   Wirkung getrennt messbar bleibt. Umkehrbar durch Revert des PR.
2. Vor jeder Massnahme mit Laufzeitwirkung den Ist-Wert festhalten, danach mit
   `skripte/holen.sh` und `skripte/auswertung.py` dieselben Kennzahlen nachmessen (mindestens 10 volle
   Läufe, sonst als vorläufig melden). Varianten erst auf dem Runner messen (Wegwerf-Workflow auf
   eigenem Zweig), dann `ci.yml` ändern.
3. Neue Entscheide als Nachtrag zu ADR 0013, Messwerte nach `docs/perf/ci.md`.
4. Keine Einstellung am Repo durch Agenten (H1 macht Marco). Keine automatischen Merges oder Reverts.
5. Prüfung je Schritt nach den Teststufen aus `CLAUDE.md`; vor dem PR einmal `task ci`.

## 7. Installierte Werkzeuge (28.11.2026)

| Werkzeug | Ort | Prüfung |
|---|---|---|
| Skills `ci-speedup`, `ci-score`, `ci-secure` (starslingdev/skills `43b9755b`, MIT) | `~/.claude/skills/` | Quellen gelesen: nur `git` und `gh` (GET; `--verify-*` startet Läufe neu, nicht benutzen), kein Netz sonst, kein `eval`/`shell=True`, keine Telemetrie. Ohne Test- und Eval-Bäume kopiert. |
| Skill `ci-cd-and-automation` (addyosmani/agent-skills `e997ac5b`, MIT) | `~/.claude/skills/` | eine Markdown-Datei, keine Skripte |
| GitHub CLI 2.45.0 | `/usr/bin/gh` | vorhanden; `gh run view --log-failed` liefert hier nichts (M3), `gh pr checks` kennt kein `--json` |
| actionlint 1.7.12 | `~/go/bin`, Link in `~/.local/bin` | aus dem Quelltext gebaut (`go install …@v1.7.12`) |
| zizmor 1.33.1 | pipx, `~/.local/bin` | offline benutzt, ohne Token |
| shellcheck | fehlt | braucht sudo (`sudo apt install shellcheck`, Marco); actionlint prüft `run:`-Blöcke erst damit |
| act | nicht installiert | siehe Abschnitt 5 |
