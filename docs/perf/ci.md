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
