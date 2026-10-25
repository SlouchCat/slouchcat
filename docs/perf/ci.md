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

**Run 40223187251**, Commit `e75811b`, grün, warme Caches.

| Job | Dauer |
|---|---|
| `server test groups` | 0 min 6 s |
| `build` (Binary mit den Web-Assets) | 1 min 15 s |
| `server (generators, linters)` | 1 min 24 s |
| `image (amd64, arm64)` | 2 min 44 s |
| `e2e (shard 1)` | 3 min 59 s |
| `server tests (3)` | 4 min 32 s |
| `e2e (shard 4)` | 4 min 39 s |
| `server tests (rest)` | 4 min 56 s |
| `e2e (shard 3)` | 5 min 13 s |
| `web (linters, unit tests, Storybook)` | 5 min 30 s |
| `e2e (shard 6)` | 5 min 53 s |
| `e2e (shard 2)` | 5 min 54 s |
| `server tests (2)` | 6 min 25 s |
| `e2e (shard 5)` | 6 min 59 s |
| `server tests (4)` | 7 min 27 s |
| `server tests (1)` | **8 min 49 s** |
| `ci` (Gate, führt die Shard-Berichte zusammen) | 0 min 35 s |
| **Lauf gesamt (Wanduhr)** | **11 min 27 s** |

**56 min 54 s auf 11 min 27 s**, Faktor 5.0. Der Richtwert von 25 Minuten ist mit mehr als der
halben Zeit Abstand eingehalten.

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

Längster Job in Run 40223187251: **`server tests (1)` mit 8 min 49 s**, also `internal/api` samt
Einrichtung des Jobs (checkout, Go, `task`, ffmpeg, Testmedien: rund 2 Minuten). Das Gate wartet auf
ihn und braucht danach 35 Sekunden.

Weiter zu senken ginge nur über das Paket selbst — `internal/api` ist ein einzelnes Paket und lässt
sich mit diesem Werkzeug nicht teilen. Entweder zerlegt man es, oder man beschleunigt seine Tests.
Beides ist eine Aufgabe für sich.

Die zweite Untergrenze liegt beim Einrichten: Jeder Job zahlt checkout, Werkzeuge und Caches erneut.
Bei 17 Jobs summiert sich das zu mehr Läufer-Minuten als vorher, auch wenn die Wanduhrzeit sinkt.
Für ein öffentliches Repository ist das kostenlos.

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

## 7. Lokal

`task ci` bleibt der eine Befehl, der alles baut und prüft. Er ruft dieselben Aufgaben auf, die die
Jobs aufrufen, nur nacheinander:

```
task ci   -> exit 0
            Server: gen, gen:check, lint, test (alle Pakete auf einmal)
            Web:    gen, gen:check, lint, test (585 Tests), Storybook (160 Bilder)
            build, e2e (426 Tests)
```

Auf dem Entwicklungsrechner mit 28 Threads: **1670.7 s** Wanduhr. Das ist langsamer als der
CI-Lauf, und das ist richtig so — CI verteilt auf 17 Maschinen, was hier eine Maschine
nacheinander tut.

Eine Einschränkung, die man kennen muss: **Auf einer Maschine müssen die Shards nacheinander
laufen.** Sie wollen alle denselben Port und dasselbe Artefaktverzeichnis, und der `blob`-Reporter
leert sein Verzeichnis bei jedem Start. In CI stellt sich die Frage nicht, weil jeder Shard einen
eigenen Läufer hat.
