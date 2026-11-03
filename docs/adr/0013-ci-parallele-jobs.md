# ADR 0013: Dieselben Prüfungen, in CI auf parallele Jobs verteilt

- Status: angenommen (25. Oktober 2026)
- Quelle: Messung des letzten grünen Laufs auf `main` (Run 40207488836, Commit `b551a63`);
  `docs/planning/03-plan-v1.md` Abschnitt 4 («Ein Befehl baut und prüft alles. Lokal und CI
  identisch»); `CLAUDE.md`, Abschnitt «Werkzeuge und Versionen»
- Betroffene Spec-IDs: keine. Das ist Infrastruktur, kein Verhalten des Servers oder der PWA
- Harte Regeln: 1 (generierter Code wird nur durch Codegen geändert), 8 (Tastaturtest je Seite),
  und aus der Arbeitsweise: übersprungene Tests und Platzhalter sind Blocker, nicht Fortschritt
- Messwerte: `docs/perf/ci.md`

## Kontext

Der Workflow hatte einen einzigen Job `ci`, der `task ci` am Stück ausführte. Im letzten grünen
Lauf auf `main` dauerte er **56 Minuten 52 Sekunden**. Der Job `image` lief mit 3 Minuten 8
Sekunden schon daneben, trug zur Gesamtzeit also nichts bei. Ein Pull Request wartete damit knapp
eine Stunde auf sein erstes Urteil, und ein Tippfehler im ESLint-Lauf zeigte sich erst, nachdem die
Go-Tests durch waren.

Die Zeit verteilte sich so (Zeitstempel aus dem Job-Log):

| Abschnitt | Zeit |
|---|---|
| Einrichtung (checkout, Go, pnpm, Node, task, `pnpm install`) | 1 min |
| Systempakete (ffmpeg, PulseAudio, Playwright-Bibliotheken) | 8 min |
| `server:test` (`go test -race ./...`) | 19 min |
| `web:e2e` (Playwright, 416 Tests, `workers: 1`) | 23 min |
| Storybook (Bau und Bildvergleich) | 2 min |
| lint (Server und Web) | 1 min |
| übriges (`gen`, `gen:check`, `build`) | < 1 min |

Drei dieser Posten sind voneinander unabhängig: Die Go-Tests brauchen den Webbau nicht, der
ESLint-Lauf braucht die Go-Tests nicht, und die Playwright-Suite braucht von beiden nur das fertige
Binary. Nacheinander ausgeführt wurden sie, weil ein Job ein Skript ist — nicht, weil eine
Abhängigkeit es verlangte.

Dagegen stand die Zusage aus Plan-Abschnitt 4: **ein** Befehl baut und prüft alles, lokal wie in CI.
Die Frage war also nicht, ob man parallelisieren kann, sondern wie man es tut, ohne diese Zusage zu
verlieren.

## Entscheid

| Punkt | Entscheid |
|---|---|
| Was bleibt | **`task ci` bleibt der eine lokale Befehl** und führt weiterhin jede Prüfung aus. Es ruft nur nicht mehr `gen`, `lint`, `test`, `build`, `e2e`, `storybook:test` direkt auf, sondern die drei Teilaufgaben `ci:server`, `ci:web`, `ci:e2e` samt `build`. Was in eine dieser drei kommt, ist damit automatisch in `task ci` — die Aufteilung kann nicht dazu führen, dass CI etwas prüft, das lokal niemand aufruft, und umgekehrt. Der Kopfkommentar von `ci.yml` behauptete «Runs exactly what `task ci` runs locally»; er sagt jetzt, was wirklich gilt: dieselben Aufgaben, auf parallele Jobs verteilt |
| Was CI nicht darf | **CI ruft nie etwas auf, das es als Aufgabe nicht gibt.** Kein `go test …` und kein `pnpm …` direkt im Workflow; jeder Prüfschritt ist ein `task`-Aufruf. Sonst wäre die nächste Abweichung zwischen lokal und CI eine Zeile YAML entfernt, und niemand merkte es, bis ein Lauf grün ist, der lokal rot wäre |
| Gate | Ein abschliessender Job **`ci`** mit `needs:` auf alle anderen, `if: always()` und einem Schritt, der jedes Ergebnis einzeln prüft. Dadurch bleibt genau **ein** Statusprüfname (`ci / ci`) als Merge-Bedingung — derselbe wie vorher, es ist an GitHub nichts umzustellen. `contains(needs.*.result, 'failure')` wäre kürzer, nennt aber den schuldigen Job nicht und liesse `skipped` und `cancelled` durch |
| e2e-Shards | `playwright test --shard=i/n` über eine Matrix von **sechs** Shards. Jeder Shard ist ein vollständiger Lauf auf einem eigenen Runner: eigener Server, eigene Datenbank, eigener `globalSetup`. `workers: 1` und `fullyParallel: false` bleiben **innerhalb** eines Shards bestehen, es rennt also nach wie vor nichts gegeneinander. Sechs und nicht mehr, weil jeder Shard den `globalSetup` mit den Bibliotheksscans erneut zahlt: ab etwa acht Shards wächst dieser feste Anteil schneller, als der geteilte Anteil schrumpft |
| Wie die Shards geschnitten werden | Playwright teilt nach **Testgruppen**, und bei `fullyParallel: false` ist eine Gruppe eine Spec-Datei je Projekt. Eine Datei wird also nie zerschnitten; die Reihenfolge innerhalb einer Datei ist dieselbe wie bisher. Was ein Shard **nicht** hat, sind die Daten der Spec-Dateien, die in anderen Shards gelandet sind — die `owner`-Fixture setzt vor und nach jedem Test ohnehin zurück, die Suite war auf Eigenständigkeit gebaut (`retries: 1` verlangt sie schon heute) |
| Bericht im Fehlerfall | Jeder Shard schreibt einen **`blob`**-Bericht, der Gate-Job fügt sie mit `playwright merge-reports` zu **einem** HTML-Bericht mit allen Traces zusammen. Sechs HTML-Berichte mit je einem Sechstel des Laufs wären formal dasselbe und praktisch unbrauchbar |
| Server-Binary | **Einmal bauen**, in einem eigenen Job `build`, und per `upload-artifact`/`download-artifact` an die Shards reichen. Sechs Shards, die dasselbe Binary bauen, wären sechs Go-Toolchains und sechs Vite-Bauten für eine Datei. Nach dem Download setzt ein Schritt das Ausführungsrecht neu: ein Artefakt ist ein Zip und trägt das Bit nicht |
| Go-Tests | **Paketgruppen über eine Matrix**, definiert in `server/testgroups.txt`, aufgelöst von `server/scripts/testgroups.sh`. Begründung unten |
| Kein Paket ohne Gruppe | Nur die **teuren** Pakete stehen in `testgroups.txt`; die Gruppe `rest` wird als «alles, was `go list ./...` kennt und die Datei nicht nennt» **berechnet**. Ein neues Paket wird darum ab dem ersten Tag getestet, ohne dass jemand an diese Datei denken muss. `scripts/testgroups.sh --check` läuft im `server:lint` und schlägt fehl, wenn die Datei ein Paket nennt, das es nicht mehr gibt — eine Umbenennung ist dann ein roter Lint statt einer Gruppe, die still nichts prüft |
| Keine Gruppe ohne Job | Die Matrix im Workflow **zählt die Gruppen nicht auf**, sondern fragt `scripts/testgroups.sh --matrix` (eigener Job `server-groups`, braucht nur den Checkout). Stünde die Liste auch in `ci.yml`, hiesse derselbe Name an zwei Stellen — und die Hälfte, die man vergisst, wäre genau die, die keine Tests ausführt: `rest` nimmt ein Paket einer neuen Gruppe heraus, und ohne Matrix-Zweig liefe es nirgends, bei grünem Lauf. Im Review vom 25. Oktober 2026 gefunden, bevor es passieren konnte |
| Linter getrennt von Tests | Eigener Job `server-lint` für `server:gen`, `server:gen:check` und `server:lint`. Die Linter brauchen 90 Sekunden, die Tests 19 Minuten; sie in jeder Testgruppe mitlaufen zu lassen hiesse, dieselben 90 Sekunden drei Mal zu zahlen und einen Lint-Fehler erst nach den Tests zu sehen |
| `gen:check` in zwei Teile | `server:gen:check` prüft `internal/api/gen` und `internal/store`, `web:gen:check` prüft `packages/api-client/src/generated`. Der Server-Job führt den TypeScript-Generator nie aus und der Web-Job nie `sqlc`; keiner von beiden könnte also über die Ausgabe des anderen urteilen. Die Wurzelaufgabe `gen:check` ruft beide auf, lokal ändert sich nichts (harte Regel 1) |
| Was jeder Job installiert | Der Server-Job nur **ffmpeg** (Probe- und Transcode-Tests, Testmedien), der Web-Job **gar keine** Systempakete (die Storybook-Bilder entstehen im offiziellen Playwright-Docker-Image, das Chromium und seine Bibliotheken mitbringt), die e2e-Shards ffmpeg, PulseAudio und die Playwright-Bibliotheken |
| apt-Cache | Schlüssel über **`.github/ci-packages.txt`** und `pnpm-lock.yaml` statt über `ci.yml`, dazu `restore-keys` als Präfix. Der alte Schlüssel hashte die Workflow-Datei: jede Änderung daran — ein Kommentar, ein Zeitlimit — warf den Cache weg, und ein auf einem Branch entstandener Cache erreicht `main` ohnehin nie. Im Messlauf war das ein Treffer von null und 479 Sekunden Wartezeit |
| `task` kommt fertig | Statt `go install …/task@v3.54.0` in jedem Job lädt `.github/actions/task` das Release-Archiv in der in `.tool-versions` gepinnten Fassung. Das spart je Job eine Go-Übersetzung, und der Web-, die e2e- und der Gate-Job brauchen damit **gar kein** Go mehr. Die Fassung kommt aus derselben Datei, die lokal gilt — sie kann nicht auseinanderlaufen. Die SHA-256 des Archivs steht in `.github/task-checksums.txt` und wird vor dem Auspacken geprüft: jede andere fremde Aktion in diesem Workflow hängt an einem Commit-Hash, und dieses Binary landet auf dem PATH **vor** jedem weiteren Schritt |
| Wiederholte Schritte | Vier zusammengesetzte Aktionen unter `.github/actions/` (`task`, `node`, `system-packages`, `test-media`). Die apt-Logik ist subtil genug (siehe die Kommentare dort), dass zwei Abschriften davon zwei Abschriften zum Pflegen wären |

### Warum Paketgruppen und nicht `t.Parallel()`

Beide Wege standen zur Wahl. Entschieden hat eine eigene Messung auf vier Kernen
(`taskset -c 0-3`, derselbe Zuschnitt wie der Runner):

| Paket | im CI-Lauf (alle Pakete gleichzeitig) | allein, vier Kerne |
|---|---|---|
| `internal/api` | 918.7 s | **211.7 s** |
| `internal/admin` | 776.4 s | **205.1 s** |

Die 918 Sekunden von `internal/api` waren also **keine** Untergrenze, sondern zu vier Fünfteln
Wartezeit: `go test -p 4` startet vier Paket-Binärdateien gleichzeitig, jede mit `GOMAXPROCS=4`,
und auf vier Kernen konkurrieren dann bis zu sechzehn lauffähige Goroutinen. Die Pakete bremsen
einander, nicht sich selbst.

Damit ist `t.Parallel()` das falsche Werkzeug für dieses Problem. In `internal/api` steht es bereits
in allen 26 Testdateien — das Paket nutzt seine vier Kerne (Summe der Einzeltests 605 s gegen 211.7 s
Paketlaufzeit, Faktor 2.9). In `internal/admin` steht es in genau einer von 18 Dateien, dort liegt
Summe der Einzeltests und Paketlaufzeit mit 217 s zu 205.1 s praktisch gleich: Das Paket läuft
seriell. `t.Parallel()` nachzuziehen würde dort etwas bringen, aber es ist die riskantere Änderung —
jeder Test, der eine gemeinsame Datenbankdatei, ein gemeinsames temporäres Verzeichnis oder eine
Umgebungsvariable anfasst, wird dadurch flatterhaft statt schneller, und das an 69 Stellen zu
prüfen ist eine eigene Aufgabe mit eigenem Risiko. Die Gruppen erreichen dasselbe Ziel, ohne eine
einzige Zeile Testcode anzufassen.

`-race` bleibt in jeder Gruppe.

Verworfen: (a) **Grössere Runner** — für ein öffentliches Repository nicht kostenlos, und die
Aufteilung wäre danach trotzdem fällig. (b) **Ein selbst gehosteter Runner** — an einem
öffentlichen Repository führt er den Code jedes Fork-Pull-Requests auf fremder Hardware aus; am
25. Oktober 2026 von Marco verworfen. (c) **Die e2e-Suite auf `workers: >1` stellen statt zu
sharden** — dann rennen Specs gegen **dieselbe** Datenbank, was die Suite ausdrücklich nicht darf.
(d) **Nur die langsamen Tests in CI laufen lassen** — ein Testumfang, der von der Maschine abhängt,
ist kein Testumfang. (e) **`-race` nur auf `main`** — dann findet `main` die Fehler, die der Pull
Request hätte finden sollen. (f) **Die Gruppen als feste Paketlisten im Workflow** — ein neu
angelegtes Paket fiele durch alle Gruppen und würde von niemandem getestet, ohne dass etwas rot
wird.

## Folgen

- «Lokal und CI identisch» heisst ab jetzt: **dieselben Aufgaben, dieselben Prüfungen, andere
  Reihenfolge.** `task ci` führt sie nacheinander aus, CI gleichzeitig. Die Gegenleistung ist, dass
  die Reihenfolge nicht mehr Teil der Zusage ist: Ein Fehler, der nur auftritt, weil die Go-Tests
  vor der Playwright-Suite liefen, fällt lokal auf und in CI nicht. Ein solcher Fehler wäre
  allerdings schon heute ein Fehler — die Suiten teilen keinen Zustand, und die e2e-Suite startet
  ihren Server mit einem frischen temporären Konfigurationsverzeichnis.
- **Die e2e-Suite muss shardtauglich bleiben.** Eine Spec-Datei, die auf Daten einer anderen baut,
  läuft lokal durch und in CI zufällig: je nachdem, ob beide im selben Shard gelandet sind. Das ist
  die eigentliche neue Pflicht an dieser Stelle, und sie steht als Kommentar in
  `web/playwright.config.ts`, wo `workers: 1` erklärt wird.
- Auf **einer** Maschine müssen die Shards nacheinander laufen: Sie wollen alle denselben Port und
  dasselbe Artefaktverzeichnis. In CI ist das kein Thema, weil jeder Shard einen eigenen Runner hat.
- Es gibt jetzt **mehr Jobs als Prüfungen**: `server-groups` und `build` prüfen nichts, `ci` prüft
  nur Ergebnisse. Wer eine Prüfung hinzufügt, fügt sie einer der drei `ci:*`-Aufgaben hinzu und
  nicht dem Workflow.
- **Stehende Regel: ein neuer Job gehört in `needs:` des Gates und in dessen Liste.** Der Gate-Job
  nennt jeden Job einzeln, damit das Protokoll sagt, welcher es war, und damit `skipped` und
  `cancelled` ebenfalls rot sind. Der Preis dafür ist, dass ein vergessener Job unbeobachtet
  bleibt. `contains(needs.*.result, 'failure')` hätte diesen Preis nicht, nennt dafür den
  schuldigen Job nicht und liesse `skipped` durch — der Tausch ist bewusst so herum.
- Der Statusprüfname bleibt `ci / ci`. **Wird der Gate-Job umbenannt, ist die Merge-Bedingung
  umzustellen** — zum Zeitpunkt dieses Entscheids ist auf `main` weder ein Branch-Schutz noch ein
  Ruleset gesetzt (`gh api repos/SlouchCat/slouchcat/branches/main/protection` antwortet 404), der
  Name ist also heute nur Konvention und morgen eine Einstellung.
- Restrisiko, benannt statt wegerklärt: Mehr Jobs heissen mehr Einrichtungszeit insgesamt (jeder
  Job zahlt checkout, Werkzeuge und Caches erneut) — die Wanduhrzeit sinkt, die Summe der
  Runner-Minuten steigt. Für ein öffentliches Repository ist das kostenlos, für ein privates wäre
  es eine Rechnung. Und ein Shard, der flattert, flattert jetzt in einem von sechs Berichten;
  dagegen hilft der zusammengeführte Bericht des Gate-Jobs, nicht die Aufteilung selbst.

## Nachtrag vom 3. November 2026: schnellere CI ohne weniger Prüfungen

Gemessen in den Läufen 41705191279 und 41707865418: Das Gate brauchte rund 18 Minuten, auch für
einen Pull Request, der nur Dokumentation ändert. Marcos Entscheide vom 3. November 2026 und ihre
Umsetzung:

### Pull Requests mit reiner Dokumentation

- Ein Job **`changes`** liest die Dateien des Pull Requests (`git diff --name-only HEAD^1 HEAD`;
  der Merge-Commit eines Pull Requests hat die Basis als ersten Elternteil, ein Checkout mit
  `fetch-depth: 2` genügt) und fragt `.github/scripts/docs-only.sh`, ob es **nur Dokumentation**
  ist: `docs/**`, Markdown-Dateien und Release-Notes. **Keine fremde Action**: Ein
  `git diff`-Schritt ist kürzer als die Prüfung einer weiteren gepinnten Abhängigkeit.
- **Ausnahmen, weil eine Prüfung sie liest:** Markdown unter `web/` und `packages/` prüft
  `prettier --check web packages` mit, und `deploy/licenses/FFMPEG-NOTICE.md` kopiert das Image.
  Eine Änderung dort zählt als Code. Ein leerer Diff zählt ebenfalls als Code: Was sich nicht
  beurteilen lässt, läuft ganz.
- Alle schweren Jobs hängen an `changes` und laufen nur, wenn es **nicht** «nur Dokumentation» ist.
- **Das Gate wertet «skipped» nur in genau diesem Fall als grün:** `changes` war erfolgreich und
  hat «nur Dokumentation» gemeldet. Jedes andere «skipped» bleibt rot wie bisher, und ein
  fehlgeschlagener `changes`-Job ist selbst rot. Damit gilt die Regel «`skipped` ist rot» weiter
  für jeden Fall, in dem ein Job übersprungen wurde, weil etwas vor ihm brach.
- **Push auf `main` und Läufe von Hand prüfen immer alles** (`changes` antwortet dort ohne Diff
  «nein»): `main` bleibt der Nachweis dessen, was gemergt ist. Tags lösen diesen Workflow nicht aus
  (`release.yml`); auch dort läuft also nie ein verkürzter Lauf.
- Restrisiko: Eine Prüfung, die künftig eine Datei unter `docs/` oder eine Markdown-Datei liest,
  muss in `docs-only.sh` als Ausnahme eingetragen werden, sonst sähe ein Pull Request, der nur
  diese Datei ändert, die Prüfung nicht. Der Lauf auf `main` nach dem Merge fände den Fehler
  trotzdem.

### Ein Lauf je Commit auf `main`

- Die `concurrency`-Gruppe enthält auf `main` die Commit-ID (`github.sha`). Bisher teilten sich alle
  Läufe auf `main` eine Gruppe: Ein Merge wartete auf den Lauf davor (3,75 Minuten gemessen), und
  ein zweiter wartender Lauf **verdrängte** den ersten, der dann nie lief — der Merge von #77
  (Run 41704343177) bekam keinen Lauf auf `main`. GitHub hält je Gruppe nur einen wartenden Lauf.
- Pull Requests bleiben bei einer Gruppe je Branch mit `cancel-in-progress`: Ein neuer Push ersetzt
  den Lauf des überholten Commits.

### Build-Cache der Go-Tests mit `-race`

- `setup-go` cacht Modul- und Build-Cache unter einem Schlüssel aus `go.sum`. Weil dieser
  Schlüssel bei jedem Lauf trifft, speichert es nie neu («Cache hit occurred on the primary key»
  im Post-Schritt): Der Build-Cache ist der des ersten Jobs nach einer `go.sum`-Änderung, oft einer
  ohne `-race`, und jede Testgruppe übersetzt die mit `-race` instrumentierten Pakete neu.
- Jede Testgruppe hat deshalb einen eigenen **`actions/cache`** auf `~/.cache/go-build`, Schlüssel
  mit Gruppe, `go.sum`-Hash und `run_id`, dazu `restore-keys` als Präfix: Jeder Lauf speichert
  seinen Stand, der nächste beginnt mit dem neuesten.
- **`GOFLAGS=-count=1`** im Testschritt: Der Cache enthält auch Testergebnisse, und `go test`
  meldete schon vorher vier Pakete als «(cached)», statt sie auszuführen (Run 41707865418). Der
  Cache soll das Übersetzen sparen, nie einen Test.

### Firefox nur, wo sich die Engines unterscheiden

- Bisher lief jede Spec in beiden Browsern; Firefox brauchte 1320 der 2428 Sekunden Testzeit
  (Run 41707865418). **Marcos Entscheid:** Firefox läuft nur noch für Wiedergabe (Video, HLS, MSE,
  Spuren, Qualität, Kapitel, Versionen, HDR), Audio (Hörbücher, Podcasts), Offline,
  Anmeldung/Einrichtung und für **je einen Tastaturtest je Seite** (harte Regel 8), im Titel mit
  `@keyboard` markiert. Chromium läuft weiter alles.
- Die Liste steht ausdrücklich in `web/playwright.config.ts` (`FIREFOX_WHOLE_SPECS`, als `grep` des
  Projekts), mit der Zuordnung aller 73 Specs im Kommentar: 31 Dateien ganz, 39 `@keyboard`-Tests in
  38 Dateien, drei Dateien nur in Chromium (`admin-density`, `admin-phone`, `announcements`), dazu
  wie bisher `offline-cold-start`. Firefox hat damit 118 statt 307 Tests.
- Die bekannten Firefox-Fokusfehler bleiben abgedeckt: `admin-jobs` (Abbruchmenü, Fokus auf das h1)
  und `admin-library-folders` (Tab-Budget bis zum Ordnerdialog) sind die `@keyboard`-Tests ihrer
  Seiten.
- Restrisiko: Ein Fehler, der nur in Firefox und nur in einem Test ohne `@keyboard` auftritt,
  fällt erst bei Marco auf. Für eine neue Seite gilt: Ihr Tastaturtest bekommt `@keyboard`, sonst
  läuft sie nur in Chromium.

### e2e-Shards nach gemessener Dauer

- Playwright 1.64 teilt mit `--shard=i/n` nach **Anzahl** Tests, nicht nach Zeit; die einzige
  Gewichtung (`PWTEST_SHARD_WEIGHTS`, nicht dokumentiert) verschiebt nur die Anteile, weiter in
  Tests gezählt. Eine Spec mit einem 67-Sekunden-Test wiegt dort so viel wie eine mit 21 kurzen;
  gemessen lief der längste Shard 507 s, der kürzeste 353 s, und die Shards 1–3 waren nur
  Chromium, 4–6 nur Firefox.
- **`web/e2e/shard-plan.mjs`** plant deshalb selbst: Es fragt `playwright test --list` nach allen
  Tests, die die Config laufen liesse (samt Firefox-Auswahl), fasst sie wie Playwright zu Gruppen
  zusammen (eine Spec-Datei je Projekt, nie zerschnitten), gewichtet jede mit den gemessenen
  Sekunden aus **`web/e2e/durations.json`** und verteilt die längste Gruppe zuerst auf den Shard mit
  der wenigsten Zeit. Jeder Shard rechnet denselben Plan und führt seinen Teil mit
  `--test-list` aus (`task ci:e2e SHARD=i/n`). Ein Test ohne Messwert (neu, umbenannt) zählt mit
  dem Median seines Projekts: Er wird verplant, nie vergessen.
- **Selbsttest** `task web:e2e:shards:check`: Die Vereinigung aller Shards ist genau die Menge
  aller Tests, geprüft am Plan und an dem, was Playwright selbst aus den geschriebenen Listen
  macht (`--list --test-list` je Shard). Ein Listenformat, das Playwright nicht erkennt, fällt so
  auf, statt still Tests auszulassen.
- **Auffrischen:** `task web:e2e:durations RUNS="<id> <id>"` lädt die Blob-Berichte der Läufe, mittelt
  je Test (`web/e2e/durations.mjs`) und zeigt den neuen Plan. Die eingecheckte Tabelle stammt aus
  den Läufen 41705191279 und 41707865418, also von **vor** den kürzeren Wartezeiten und der
  Firefox-Auswahl; sie überschätzt die drei Specs mit der Spotlight-Sperre. Nach den ersten
  grünen Läufen auf `main` auffrischen.
- **Sechs Shards** bleiben: Nach Firefox-Auswahl und kürzeren Wartezeiten sind es rund 1600 s
  Testzeit (die alte Tabelle sagt 1830 s), also etwa 270 bis 305 s je Shard, dazu rund 90 s fester
  Anteil (Einrichtung 70 s, Server und `globalSetup` 20 s): rund 6 Minuten je Shard statt bis zu
  10. Sieben Shards brächten 40 s, kosten aber einen Runner mehr, und der kritische Pfad ist
  ohnehin `server-test`.

### Web-Job geteilt

- Der Web-Job (8 bis 10 Minuten: Lint 1,7, Vitest 3,5, Storybook 3,6) ist zwei Jobs: **`web`**
  (`task ci:web`: Generatoren, Drift, Linter, Vitest) und **`storybook`** (`task ci:storybook`:
  Bildvergleich im Playwright-Image). `task ci` ruft beide auf; der Gate-Job nennt beide.
