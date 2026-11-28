# Messwerte CI – von einem seriellen Job zu parallelen Jobs

Stand: 25. Oktober 2026, Branch `claude/ci-parallel-72a724`, Pull Request #36. Grundlage: Plan
`docs/planning/03-plan-v1.md` Abschnitt 4 («Ein Befehl baut und prüft alles. Lokal und CI
identisch»), ADR `docs/adr/0013-ci-parallele-jobs.md`. Alle CI-Zahlen stammen aus
`gh api repos/:owner/:repo/actions/runs/<id>/jobs`, also aus den Zeitstempeln der Jobs selbst, nicht
aus der Anzeige. Läufer: `ubuntu-24.04`, vier Kerne. Die lokalen Vergleichsmessungen liefen auf dem
Entwicklungsrechner (Intel Core i7-14700K, 28 Threads), die Go-Messungen darauf mit
`taskset -c 0-3`, um die vier Kerne des Läufers nachzustellen.

Diese Aufgabe gehört zu keinem Meilenstein. Sie ändert nichts am Server und nichts an der PWA —
mit einer Ausnahme, die unten in Abschnitt 6 steht, weil erst das Sharden sie sichtbar gemacht hat.

## 1. Ausgangslage

Letzter grüner Lauf auf `main` vor dem Umbau: **Run 40207488836**, Commit `b551a63`.

| Job | Dauer |
|---|---|
| `ci` (führte `task ci` am Stück aus) | **56 min 52 s** |
| `image` (lief schon daneben) | 3 min 8 s |
| **Lauf gesamt (Wanduhr)** | **56 min 54 s** |

Aufschlüsselung des Jobs `ci` aus den Zeitstempeln seines Protokolls:

| Abschnitt | Zeit |
|---|---|
| Einrichtung (checkout, Go, pnpm, Node, `task`, `pnpm install`) | 1 min |
| Systempakete (ffmpeg, PulseAudio, Playwright-Bibliotheken) | 8 min |
| `server:test` (`go test -race ./...`) | 19 min |
| `web:e2e` (Playwright, `workers: 1`) | 23 min |
| Storybook (Bau und Bildvergleich) | 2 min |
| lint (Server und Web) | 1 min |
| `gen`, `gen:check`, `build` | < 1 min |

Drei dieser Posten hängen nicht voneinander ab: Die Go-Tests brauchen den Webbau nicht, der
ESLint-Lauf braucht die Go-Tests nicht, und die Playwright-Suite braucht von beiden nur das fertige
Binary. Nacheinander liefen sie, weil ein Job ein Skript ist.

Dazu zwei vermeidbare Posten: Der Schritt «Install the system packages» brauchte **479 s**, weil der
apt-Cache nicht griff («Cache not found for input keys: apt-debs-noble-…»), und `task` wurde in
jedem Lauf aus dem Quelltext übersetzt.

## 2. Nachher

**Run 40225244202**, Commit `a1b1d7f`, der Endstand dieses Zweigs, grün.

| Job | Dauer |
|---|---|
| `server test groups` | 0 min 6 s |
| `build` (Binary mit den Web-Assets) | 1 min 15 s |
| `server (generators, linters)` | 1 min 19 s |
| `image (amd64, arm64)` | 2 min 46 s |
| `web (linters, unit tests, Storybook)` | 4 min 38 s |
| `e2e (shard 1)` | 4 min 40 s |
| `e2e (shard 4)` | 4 min 52 s |
| `e2e (shard 3)` | 5 min 5 s |
| `e2e (shard 6)` | 5 min 30 s |
| `server tests (1)` | 5 min 59 s |
| `server tests (3)` | 6 min 16 s |
| `e2e (shard 2)` | 6 min 38 s |
| `e2e (shard 5)` | 7 min 13 s |
| `server tests (rest)` | 7 min 4 s |
| `server tests (2)` | 7 min 24 s |
| `server tests (4)` | **7 min 33 s** |
| `ci` (Gate, führt die Shard-Berichte zusammen) | 0 min 28 s |
| **Lauf gesamt (Wanduhr)** | **10 min 46 s** |

**56 min 54 s auf 10 min 46 s**, Faktor 5.3. Der Richtwert von 25 Minuten ist mit mehr als der
doppelten Zeit Abstand eingehalten.

### Alle Läufe dieses Zweigs

| Run | Commit | Zuschnitt | Caches | Ergebnis | Wanduhr |
|---|---|---|---|---|---|
| 40220711169 | `e75811b`* | 3 Go-Gruppen | **kalt** (apt neu verschlüsselt, Treffer: keiner) | grün | 22 min 58 s |
| 40223187251 | `e75811b` | 5 Go-Gruppen | warm | grün | 11 min 27 s |
| 40224518268 | `e75811b` | 5 Go-Gruppen | warm | grün | 9 min 53 s |
| 40225244202 | `a1b1d7f` | 5 Go-Gruppen | warm | grün | 10 min 46 s |

\* Derselbe Zweig, aber vor dem Commit, der `rest` weiter geteilt hat.

Der kalte Lauf zeigt, wofür der apt-Cache da ist: In Shard 4 brauchte der Schritt
`./.github/actions/system-packages` allein **16 Minuten 9 Sekunden**, während die Playwright-Suite
desselben Shards in 4 Minuten 14 Sekunden durch war. Der Azure-Spiegel drosselt ohne Vorwarnung;
mit warmem Cache fällt der Schritt auf Sekunden, und alle sechs Shards liegen dann eng beieinander.

## 3. Wie die Shards verteilt sind

Playwright teilt nach Testgruppen; bei `fullyParallel: false` ist eine Gruppe **eine Spec-Datei je
Projekt**. Eine Datei wird also nie zerschnitten, und die Reihenfolge innerhalb einer Datei bleibt
die alte. Die Verteilung ist deterministisch — lokal und in CI dieselbe:

| Shard | Tests | Dauer in Run 40223187251 |
|---|---|---|
| 1 / 6 | 79 | 3 min 59 s |
| 2 / 6 | 63 | 5 min 54 s |
| 3 / 6 | 72 | 5 min 13 s |
| 4 / 6 | 70 | 4 min 39 s |
| 5 / 6 | 72 | 6 min 59 s |
| 6 / 6 | 70 | 5 min 53 s |
| **Summe** | **426** | |

Die Zahl der Tests ist dieselbe wie ohne Shards: `pnpm exec playwright test --list` meldet
`Total: 426 tests in 65 files`, und die sechs Shard-Auflistungen ergeben zusammen genau diese 426.
Vor dem Merge mit `main` waren es 416 in 64 Dateien; die zehn zusätzlichen Tests sind
`web/e2e/admin-library-folders.spec.ts`, das mit Pull Request #35 nach `main` kam.

Sechs Shards und nicht mehr: Jeder Shard zahlt den `globalSetup` mit den Bibliotheksscans erneut.
Ab etwa acht Shards wächst dieser feste Anteil schneller, als der geteilte Anteil schrumpft.

## 4. Wie die Go-Testgruppen geschnitten sind

Die Pakete stehen in `server/testgroups.txt`; `server/scripts/testgroups.sh` löst sie auf, und der
Workflow fragt dasselbe Skript nach der Matrix, statt die Gruppen aufzuzählen.

Entschieden wurde anhand einer eigenen Messung, nicht anhand der CI-Zahlen des seriellen Laufs. Die
sind nämlich irreführend:

| Paket | im seriellen CI-Lauf | allein, vier Kerne |
|---|---|---|
| `internal/api` | 918.7 s | **211.7 s** |
| `internal/admin` | 776.4 s | **205.1 s** |

Vier Fünftel der Differenz sind Wartezeit: `go test -p 4` startet vier Paket-Binärdateien
gleichzeitig, jede mit `GOMAXPROCS=4`, und auf vier Kernen konkurrieren dann bis zu sechzehn
lauffähige Goroutinen. Die ganze Suite lokal mit `taskset -c 0-3`: **1196.3 s Wanduhr** gegen 19
Minuten in CI — das Vier-Kern-Modell bildet den Läufer also richtig ab.

Als Gewichte dienen die Paketzeiten des seriellen Laufs. Sie sind keine CPU-Zeit, kommen ihr aber
nahe, solange alle vier Kerne beschäftigt sind:

| Gruppe | Pakete | Gewicht |
|---|---|---|
| 1 | `internal/api` | 775.6 |
| 2 | `internal/admin` | 762.9 |
| 3 | `internal/store`, `internal/library` | 776.5 |
| 4 | `internal/metadata`, `internal/playback`, `internal/offline` | 843.6 |
| rest | die übrigen 34 Pakete | 844.2 |

Der erste Zuschnitt hatte nur `api`, `admin` und ein grosses `rest`. **Run 40220711169** war damit
grün in 22 min 58 s — aber `server tests (rest)` brauchte davon 18 min 45 s und war damit die
Untergrenze des ganzen Workflows. Darum die weitere Teilung entlang derselben Gewichte.

Gruppe 1 und 2 sind je ein Paket, weil sie die zwei schweren sind. Die anderen sind Bündel, was die
vier Kerne auch besser ausnutzt: `go test -p 4` lässt mehrere Paket-Binärdateien gleichzeitig
laufen, ein einzelnes serielles Paket lässt drei Kerne leer.

**`-race` bleibt in jeder Gruppe.** `t.Parallel()` nachzuziehen wäre der andere Weg gewesen und
wurde verworfen: In `internal/api` steht es bereits in allen 26 Testdateien, und `internal/admin`
läuft heute seriell (Summe der Einzeltests 217 s gegen 205.1 s Paketlaufzeit). Dort nachzuziehen
hiesse, 69 Tests auf geteilten Zustand zu prüfen — eine eigene Aufgabe mit eigenem Risiko, und die
Gruppen erreichen dasselbe Ziel, ohne eine Zeile Testcode anzufassen.

## 5. Was die Untergrenze setzt

Längster Job in Run 40225244202: **`server tests (4)` mit 7 min 33 s**, dicht gefolgt von
`server tests (2)` (7 min 24 s), `e2e (shard 5)` (7 min 13 s) und `server tests (rest)` (7 min 4 s).
Das Gate wartet auf den letzten und braucht danach 28 Sekunden.

Die Gruppen sind damit ausgeglichen: Es gibt keinen einzelnen Pol mehr, den man wegnehmen könnte,
sondern vier ungefähr gleich lange. Wer weiter senken will, muss entweder die Gruppen noch feiner
schneiden — dann wächst aber der Anteil, den jeder Job für die Einrichtung zahlt — oder die Tests
selbst beschleunigen.

Die Einrichtung ist nämlich die zweite Untergrenze: checkout, Werkzeuge, Caches und die Testmedien
kosten je Go-Job rund zwei Minuten, je e2e-Shard rund zweieinhalb. Bei 17 Jobs summiert sich das zu
mehr Läufer-Minuten als vorher, auch wenn die Wanduhrzeit sinkt. Für ein öffentliches Repository ist
das kostenlos.

Welcher Job der längste ist, schwankt von Lauf zu Lauf um ein bis zwei Minuten — in Run 40223187251
war es `server tests (1)` mit 8 min 49 s, in Run 40224518268 `server tests (4)` mit 9 min 13 s. Das
ist die Streuung der Läufer selbst, nicht der Zuschnitt.

## 6. Was die Umstellung gefunden hat

Das Sharden liess `web/e2e/maintenance.spec.ts` zum ersten Mal in **Firefox** durch seinen
Entfernen-Zweig laufen. Der Zweig war dort nie gelaufen: Die Spec nimmt einen kurzen Pfad, wenn der
andere Browser die kaputte Datei schon entfernt hat, und Chromium kam immer zuerst. In Firefox fiel
er sofort — am sichtbaren Fokus (harte Regel 8).

Die Ursache liegt in der App, nicht im Test. Der Ring kommt aus `:focus-visible` in
`web/src/ui/global.css`, und Firefox vergibt das nicht an einen Knopf, den die Seite aus dem Code
heraus fokussiert — genau das tut react-aria mit dem Vorgabeknopf einer Bestätigung. Der Trace zeigt,
dass react-aria es besser weiss und `data-focus-visible="true"` setzt; acht andere Komponenten
hörten schon darauf, der Knopf war der einzige, der es nicht tat. **«Entfernen» in einem
Bestätigungsdialog hatte in Firefox keinen sichtbaren Fokus, in der ausgelieferten Fassung.**

Belege: `task ci:e2e -- maintenance.spec.ts --project=firefox` vorher 1 Fehler und 4 grün, nachher 5
grün; `task storybook:test` 160 grün, kein Grundbild bewegt.

### Ein Flattern, das nicht vom Sharden kommt

In Run 40223187251 meldete Shard 4 **ein** `flaky`:
`admin-library-folders.spec.ts › the folders dialog opens from the keyboard and lists the folder`
(Firefox), mit `Error: Tab did not reach the element within 80 presses` im ersten Versuch und grün
im zweiten.

Nachgeprüft, weil ein flatternder Shard den ganzen Umbau in Frage stellte — er tut es nicht:

| Prüfung | Ergebnis |
|---|---|
| Die Spec **allein**, ohne jede andere davor | 1 flaky, 2 grün — flattert auch so |
| Dieselbe Spec mit `main`s Fassung von `Button.module.css` | 1 flaky, 2 grün — nicht die neue CSS-Regel |

Die Datei kam am 25. Oktober 2026 mit Pull Request #35 nach `main`. Das Flattern ist also
vorbestehend und von der Aufteilung unabhängig; es gehört in eine eigene Aufgabe. Bleiben wird es
auffallen, weil `retries: 1` es als `flaky` ausweist statt es zu verstecken.

## 7. Lokal

`task ci` bleibt der eine Befehl, der alles baut und prüft. Er ruft dieselben Aufgaben auf, die die
Jobs aufrufen, nur nacheinander:

```
task ci   -> exit 0
            Server: gen, gen:check, lint, test (alle Pakete auf einmal)
            Web:    gen, gen:check, lint, test (599 Tests in 77 Dateien), Storybook (160 Bilder)
            build, e2e (426 Tests)
```

Auf dem Entwicklungsrechner mit 28 Threads: **1606.0 s** Wanduhr (Stand `a1b1d7f`, also derselbe
Prüfumfang wie Run 40225244202). Das ist langsamer als der CI-Lauf, und das ist richtig so — CI
verteilt auf 17 Maschinen, was hier eine Maschine nacheinander tut.

Eine Einschränkung, die man kennen muss: **Auf einer Maschine läuft die Suite nur einmal
gleichzeitig.** Die Shards wollen alle denselben Port und dasselbe Artefaktverzeichnis, und der
`blob`-Reporter leert sein Verzeichnis bei jedem Start. In CI stellt sich die Frage nicht, weil
jeder Shard einen eigenen Läufer hat.

Das gilt **maschinenweit, nicht je Arbeitsverzeichnis**: `web/e2e/start-server.mjs` belegt 8097 für
den Server und 8089 bis 8095 für die Mocks, und zwei Slouchcat-Prüfungen auf demselben Rechner
greifen sich diese Ports gegenseitig weg — auch aus zwei verschiedenen Klonen oder Worktrees. Beim
Umbau am 25. Oktober 2026 sah das so aus: ein Shard-Lauf fiel mit 68 von 70 Tests, weil ein
`start-server.mjs` einer anderen Sitzung noch auf 8097 lauschte; derselbe Shard auf
`SLOUCHCAT_E2E_PORT=8197` war sofort wieder grün. Wer parallel prüfen will, setzt die Variable, und
zwar mit mindestens zehn Schritten Abstand, weil die Mock-Ports von ihr abgezählt werden.

## 8. Nachtrag vom 26. November 2026: `server tests (2)` und `android`

Auftrag von Marco: die zwei längsten Jobs kürzen, ohne dass eine Prüfung wegfällt. ADR 0013,
Nachtrag vom 26. November 2026, hält die Entscheide fest; hier stehen die Messungen. Lokal heisst
wie oben der Entwicklungsrechner, gemessen im Käfig (`taskset`, Speicherdeckel); «Runner» heisst
`ubuntu-24.04` mit vier Kernen.

### Ausgangslage

Zehn grüne PR-Läufe vom 23. bis 26. November 2026 (45216383177 bis 45779414360):

| Job | kürzester | längster |
|---|---|---|
| `server tests (2)` (`internal/store`) | 1168 s | 1520 s |
| `android (builds, lint, generators, tests)` | 806 s | 1398 s |
| e2e, längster Shard | 719 s | 816 s |
| `server tests (1)` | 499 s | 612 s |
| Gate fertig nach | 1313 s | 1596 s |

### `internal/store`

| Messung | Ergebnis |
|---|---|
| CPU-Profil, 10 Migrationstests | 46 % in der Race-Laufzeit (`__tsan_read`, `racecall`), der Rest in SQLite |
| Mutex-Profil | 99 % der Wartezeit auf `allocatorMu` in `modernc.org/libc` (`Xmalloc` 55 %, `Xfree` 41 %) |
| `strace -c`, ein Migrationstest | 6539 `fsync`, 1636 `unlink` von `slouchcat.db-journal`: der Helfer `replay` schreibt jede Zeile des Dumps als eigene Transaktion |
| 10 Tests, `-parallel 1`, vor / nach `synchronous(OFF)` im Replay | 260 s / 43 s (26 % / 104 % eines Kerns) |
| alle 139 Tests einzeln je Prozess, nacheinander summiert | 1460 s Wand, 1357 s CPU |
| ganzes Paket in einem Prozess, 24 Kerne | 1007 s Wand, 2635 s CPU (268 %) |
| ganzes Paket in einem Prozess, 4 Kerne | 960 s |
| `modernc.org/...` ohne Race-Instrumentierung, 10 Tests | 11 s statt 43 s (nicht gebaut, Frage an Marco) |

Daraus: Ein Prozess nutzt nie mehr als rund 2,7 Kerne, weil jede SQLite-Verbindung ihren
Speicher über dieselbe Sperre holt, und mehr Kerne kosten nur CPU-Zeit für das Warten an ihr. Der
fsync-Fix hilft lokal, auf dem Runner nicht messbar.

Runner, Messlauf 45791696144 (Testschritt, ohne Einrichtung):

| Variante | Sekunden |
|---|---|
| vorher, ganzes Paket (zweimal) | 995, 1289 |
| fsync-Fix, ganzes Paket | 1263 |
| fsync-Fix, zwei Hälften | 470 und 568 (Migrationen, Wiederherstellung), 585 (Rest) |

Gewählt sind drei Gruppen (`2`, `2b`, `2c`; 68, 7 und 64 Tests), lokal unter `-race` mit
`task server:test GROUP=…` 364, 352 und 368 s, zusammen 139 Tests, keiner doppelt.

### `android`

Lokal, leerer Build-Cache, `clean` dazwischen: `ciCheck ciTest` 6 min 3 s (709 Aufgaben
ausgeführt), dann 8 s (449 aus dem Cache; die 415 ausgeführten sind Kopier- und Metadaten-Aufgaben).
Der Build-Cache dafür ist 269 MB gross, auf dem Runner 450 bis 660 MB. Die Aufgaben lassen sich
zwischenspeichern; der Cache der CI war nur drei Tage alt.

Runner, Messlauf 45791696144: zuerst der Job wie heute (alter Cache), dann mit dessen frischem
Build-Cache nach einer kleinen Änderung (eine neue Funktion in `app-mobile` bzw. eine neue
öffentliche Funktion in `core:api`, von der jedes Modul abhängt). Job-Dauer:

| Variante | Job |
|---|---|
| wie heute (alter Cache) | 1033 s |
| frischer Cache, keine Android-Änderung | 141 s |
| frischer Cache, Änderung in `app-mobile` | 668 s |
| frischer Cache, Änderung in `core:api` | 957 s (mit vier Workern 889 s) |
| dito, nur Lint / nur Builds / nur Tests | 364 / 712 / 397 s (in den Builds 327 s R8) |

Messlauf 45798911327, die zwei Hälften so, wie `ci.yml` sie jetzt hat (Job-Dauer `debug` /
`release`):

| Variante | `debug` | `release` | länger |
|---|---|---|---|
| alter Cache (der erste Lauf nach dem Merge) | 625 s | 487 s | 625 s |
| frischer Cache, keine Android-Änderung | 135 s | 113 s | 135 s |
| frischer Cache, Änderung in `app-mobile` | 407 s | 374 s | 407 s |
| frischer Cache, Änderung in `core:api` | 571 s | 485 s | 571 s |

### Warteschlange

Mehr Jobs kosten keine Wartezeit: In Lauf 45779414360 starteten 22 Jobs innerhalb von 3 s nach
ihrer Erzeugung.

### Pull Request #231

Lauf 45808439520, grün, noch mit dem alten Android-Build-Cache (nur `main` speichert ihn):

| Job | Dauer |
|---|---|
| `server tests (2)` / `(2b)` / `(2c)` | 576 / 487 / 569 s (Testschritt 524 / 397 / 476 s) |
| `android (lint, tests, debug builds, generators)` / `(release builds, R8)` | 647 / 587 s |
| `server tests (1)` | 555 s |
| längster e2e-Shard | 778 s |
| Gate fertig nach | **885 s** (vorher 1313 bis 1596 s) |

Der längste Job ist damit wieder ein e2e-Shard.

## 9. Nachtrag vom 28. November 2026: Messung des Monats und Welle 1

Grundlage: alle 505 Läufe des Workflows `ci` vom 29. Oktober bis 28. November 2026 (9424 Jobs, 101
rote Job-Logs), nur lesend über die GitHub-API. Plan und Herleitung: `docs/perf/ci-optimierungsplan.md`.
Die Auswerteskripte liegen lokal in `.omc/research/ci-optimierung/skripte/` (nicht im Repo):
`SINCE=<Datum> bash holen.sh`, dann `python3 -I auswertung.py`.

### Ausgangswerte (vor Welle 1)

| Kennzahl | Wert | n |
|---|---|---|
| Voller PR-Lauf bis Gate, Median / P95 (seit 26.11.) | 15,2 / 16,5 min | 6 (vorläufig) |
| Längster Pol (seit 26.11.) | e2e-Pfad in 11 von 11, 5,0 min vor server-test | 11 |
| Runner-Minuten je vollem PR-Lauf | 188,0 (174,2 ohne Emulator-Job) | 6 |
| main-Pushes mit Übernahme (seit 16.11.) | 20 von 60 (33 %) | 60 |
| volle main-Läufe nur wegen rotem Emulator-Job | 10 von 39 | 39 |
| Jobs bis zur Jobgrenze in `system-packages` | 17 (736 Runner-Min) | Monat |
| rotes Job-Log, Median ganz / Auszug | 88 800 / 1 700 Tokens | 101 |
| Actions-Cache | 10,41 von 10 GiB | 114 Einträge |

### Was Welle 1 ändert und wie es nachgemessen wird

| Massnahme | Erwartung | Nachmessen nach dem Merge |
|---|---|---|
| M1 Übernahme ohne informative Jobs, an den Baum gebunden | Merges eines Zweigs auf aktuellem main mit grünem PR-Lauf werden übernommen, auch bei rotem Emulator-Job; main-Ergebnis nach rund 5,5 statt 13,9 min | Anteil Übernahmen unter diesen Merges, ohne Merges mit Änderungen unter `.github/` (Bedingung 6; der Merge dieser Änderung selbst läuft voll), Ziel 10 von 10 |
| M2 apt-Zeitgrenzen | Hänger rot nach höchstens rund 10 min statt 30–47; Normalfall unverändert | Median von `system-packages` (heute 26–33 s, ±5 s), Jobs am Jobzeitlimit (Ziel 0 in 30 Tagen) |
| M3 Fehlerauszug | ≤ 3000 Tokens je Lauf, Ursache enthalten | an den zehn Referenzläufen gemessen: 332 bis 2340 Tokens (zusammen 9204), Ursache 10 von 10 |
| M7 Workflow-Selbstprüfung | keine Laufzeitwirkung (Sekunden in `server-lint`) | Dauer von `server-lint` (heute Median 3,5 min) |

Welle 1 ändert keinen langen Pol; die Wartezeit eines PR bleibt bei rund 15 Minuten. Die Wartezeit
senken erst M4 (e2e-Shards) und M5 (Web-Job teilen) aus Welle 2.

## 10. Nachtrag vom 28. November 2026: Messung der e2e-Shard-Zahl und des geteilten Web-Jobs

Wegwerf-Workflow `measure.yml` auf dem Zweig `claude/messung-e2e-shards` (ohne PR, danach gelöscht),
Läufe 46150103062 und 46151712472, je zwei Wiederholungen jeder Variante, also n = 4. Grundlage ist
die aufgefrischte Dauertabelle (806 Tests). «Längster Shard» ist die Job-Dauer des langsamsten Shards
eines Durchgangs; sie bestimmt das Ende des e2e-Pfads. Runner-Minuten der e2e-Jobs eines Durchgangs,
je Job aufgerundet.

| Shards | längster Shard je Durchgang (min) | Median | Runner-Min e2e je Durchgang | Median |
|---|---|---|---|---|
| 6 | 10,9 · 12,5 · 12,6 (· 11,6*) | 12,5 | 63 · 69 · 69 (· 58*) | 69 |
| 7 | 9,8 · 10,1 · 10,2 · 11,2 | 10,1 | 68 · 68 · 70 · 75 | 69 |
| 8 | 9,3 · 10,0 · 10,0 · 10,1 | 10,0 | 73 · 74 · 74 · 75 | 74 |
| 9 | 8,4 · 8,8 · 8,9 · 9,9 | 8,8 | 75 · 76 · 78 · 79 | 77 |

\* Ein Shard rot (Flake in `admin-library-delete.spec.ts:73`, Firefox; derselbe Shard in der
Wiederholung grün). Dieser Durchgang ist unvollständig und zählt nicht zum Median. Mediane aus den
ungerundeten Job-Dauern.

**Gewählt: sieben Shards** (Marcos Entscheid vom 28. November 2026; ADR 0013, Nachtrag Welle 2).

Zum Vergleich in der CI seit dem 26. November mit der alten Tabelle: längster Shard Median 12,7 min
(n = 11). Jeder Shard zahlt rund 2,5 min fest (Einrichtung 1,2 min, Server und `globalSetup`); darum
sinkt die Dauer langsamer, als die Testzeit je Shard schrumpft, und die Runner-Minuten steigen.

Web-Job geteilt (je vier Durchgänge, Job-Dauer): Lint-Hälfte 3,3 · 3,3 min vollständig (dazu 2,8 · 2,9
bis zum Prettier-Fehler der Tabelle, ohne die letzten kleinen Prüfungen), Test-Hälfte 4,9 · 5,0 · 5,7 · 7,8 min. Bisher der ganze Job
im Median 7,8 min (P95 11,6).

### Nachmessen nach dem Merge

| Kennzahl | vorher | erwartet (geschätzt aus der Messung) |
|---|---|---|
| Voller PR-Lauf bis Gate, Median | 15,2 min (n = 6) | rund 12,5 min |
| längster e2e-Shard, Median | 12,7 min (n = 11) | rund 10,1 min |
| Web-Pol (Ende des Jobs), Median / P95 | 8,1 / 11,9 min | rund 5 bis 6 / 8 min |
| Runner-Minuten je vollem PR-Lauf | 188,0 | etwa gleich bis +2 (e2e gemessen gleich, zweite Web-Hälfte rund +1) |

Über mindestens 10 volle PR-Läufe nachmessen (`.omc/research/ci-optimierung/skripte/`); weniger als
10 als vorläufig melden.

## 11. Nachtrag vom 28. November 2026: Cache-Haushalt und Release-Build (Welle 3)

### Caches

Inventar am 28. November 2026 gegen 16:45: 126 Einträge, 10,67 GiB (Grenze 10). Die grössten
Familien: `gradle-build-…-release` 2,57 GiB in drei Einträgen, `gradle-Linux` 2,30 GiB, `gradle-build-
…-debug` 1,55 GiB in zwei, `gradle-e2e` 0,46 GiB, `setup-go` 0,43 GiB (dazu 0,27 GiB im Bereich eines
PR), `playwright` 0,38 GiB, `go-race` acht Gruppen je 0,12 bis 0,20 GiB.

Gemessen am PR-Lauf 46158298349 und am main-Lauf 46161151377:
- `apt-debs-noble-server` und `-android-e2e`: «Cache not found»; jeder Server-Job lud 63 MB und
  installierte 87 Pakete. Der main-Lauf speicherte sie um 15:44 neu (60 MB je Eintrag), nachdem sie
  zwischen 08:56 und 10:32 verdrängt worden waren. Im PR-Lauf scheiterten die anderen Gruppen am
  Speichern: «Unable to reserve cache … another job may be creating this».
- Treffer bei `setup-go`, `testmedia`, `playwright`, `apt-debs-noble-e2e` und `gradle-e2e`.

Neu je Push auf `main`: bisher acht `go-race`-Einträge und zwei `gradle-build`-Einträge (rund 3,3 GiB),
jetzt höchstens zwei `gradle-build`-Einträge, `go-race` einmal am Tag (rund 1,6 GiB). Entfallen:
`gradle-e2e` (0,46 GiB) und der Build-Cache im Abhängigkeits-Cache (geschätzt rund 0,5 GiB; der
Build-Cache selbst ist je Hälfte 0,8 GiB gross). **Nachmessen:** nach einer Woche Belegung und
Einträge je Familie; Ziel: die apt- und setup-go-Caches bleiben im Inventar, Belegung unter 9 GiB.

### Release-Images ohne Cache

Wegwerf-Lauf 46161828658, beide Release-Images (Server und Updater, je amd64 und arm64), ohne Push,
je zweimal:

| Variante | Job | Server-Image | Updater-Image |
|---|---|---|---|
| mit dem gha-Cache der CI | 2,5 · 2,3 min | 1,7 · 1,5 min | 0,1 · 0,1 min |
| ohne Cache (`no-cache`) | 4,0 · 3,5 min | 2,8 · 2,8 min | 0,3 · 0,3 min |

Ein Release dauert damit rund 1,3 min länger (bisher Median 3,5 min).

### Instabile Tests (M8)

Untersucht die fünf häufigsten und jüngsten Kandidaten aus der Auswertung aller roten Logs:

| Test | Befund | Änderung |
|---|---|---|
| `admin-library-delete.spec.ts:73` (Firefox) | Der Lösch-Job war 31 ms nach der 202 fertig, die Seite ersetzte den Wartezustand samt «Zum Job» nach rund 65 ms, bevor der Test ihn las. Mit der Reihenfolge des Runners nachgestellt: 9 von 10 rot. | Der Test hält die Abfragen der Seite an, bis er den Wartezustand gelesen hat; Service Worker gesperrt wie in `admin-libraries`. Danach 10 von 10 (Firefox) und 20 von 20 (Chromium) grün. |
| `RadioScreenTest > theFocusedFavoriteTileCarriesTheStar` (TV) | Der Test lud die Startseite über das Netz; kam deren «Try again» spät, nahm sie den Fokus aus der Seitenleiste. Nachgestellt: derselbe Fehler. | Eigene leere Startseite (`FakeHome`). 11 von 11 grün. |
| `hdr.spec.ts` (Start in 5 s) | Messung korrekt, ffmpeg ohne GPU braucht die Zeit; alle 9 roten Läufe lagen vor dem Entscheid vom 26.11. (5,5 s ohne GPU, `d90a9547`), seither 24 grüne Läufe. | keine |
| `AudioServiceTest > headphonesPulledOutPauseTheSound` | am 23.11. behoben (`815a7afb`), seither 69 grüne Android-Jobs. | keine |
| `admin-wishes`, `admin-metadata` (18.11.) | keine Flakes: Fehler der App bzw. Zeitlimit, beide am 18.11. behoben. | keine |

Offen als Frage an Marco: Lädt die Startseite der TV-App spät, nimmt sie auch auf einem echten
Fernseher den Fokus aus der Seitenleiste.
