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
- Jede Testgruppe hat deshalb einen eigenen Cache auf `~/.cache/go-build`, Schlüssel mit Gruppe,
  `go.sum`-Hash und `run_id`, dazu `restore-keys` als Präfix. **Gespeichert wird nur bei einem Push
  auf `main`** (`actions/cache/save`), gelesen in jedem Lauf (`actions/cache/restore`): Jeder Branch
  darf die Caches von `main` lesen, einen eigenen Cache aber nur er selbst; fünf Gruppen je Push
  eines Pull Requests würden die Caches von `main` und die Playwright-Browser aus den 10 GB des
  Repositorys verdrängen.
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
  Projekts), mit der Zuordnung aller 73 Specs im Kommentar: 31 Dateien ganz, 43 `@keyboard`-Tests in
  39 Dateien, zwei Dateien nur in Chromium (`admin-density`, `admin-phone`), dazu wie bisher
  `offline-cold-start`. Firefox hat damit 122 statt 307 Tests.
- Die bekannten Firefox-Fokusfehler bleiben abgedeckt, auch wo das einen zweiten Test einer Seite
  heisst (Review vom 3. November 2026): `admin-jobs` (Abbruchmenü und Fokus auf das h1;
  Fehlschläge mit der Bestätigung, die Firefox für ihre Ausblend-Animation stehen lässt),
  `admin-library-folders` (Tab-Budget bis zum Ordnerdialog), `admin-announcements` (sofort
  veröffentlichen: das h1 nimmt den Fokus einige Frames später, Firefox springt mit Tab nicht
  zurück an den Anfang), `library` (Filterbereich: `:focus-visible` nach Zeigerklick) und
  `announcements` (Fokus im Dialog der Ankündigung).
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

## Nachtrag vom 15. November 2026: main übernimmt das Ergebnis des PR-Laufs

**Anlass:** Marco sah nach dem Merge von #167 einen zweiten, vollen Lauf auf `main` für genau den
Stand, den der PR-Lauf schon grün geprüft hatte. Das ist bei fast jedem Merge so: Die Zweige
übernehmen `main` vor dem Merge, und der Merge-Commit hat dann den Baum des PR-Kopfs. Marco: «Ja,
einbauen».

**Entscheid:** Ein Push auf `main` überspringt die schweren Jobs, wenn ein grüner PR-Lauf genau
diesen Inhalt schon vollständig geprüft hat. Das entscheidet der Job `changes` mit
`.github/scripts/already-tested.sh`. Alle vier Bedingungen müssen gelten, sonst läuft alles wie bisher:

1. Der neue Commit auf `main` ist ein Merge-Commit mit zwei Eltern: `main` davor (P1) und der Kopf
   des Pull Requests (P2). Squash- und Rebase-Merges laufen voll.
2. P2 enthält P1 (GitHub-Vergleich: P2 liegt nicht hinter P1). Der PR-Lauf hat den Merge von P2 mit
   dem `main` seiner Zeit geprüft; jenes `main` ist in P1 enthalten und damit in P2, also hat er den
   Baum von P2 geprüft.
3. Der Merge-Commit hat genau den Baum von P2.
4. Ein Lauf dieses Workflows mit Ereignis `pull_request` für P2 endete mit Erfolg, und **jeder** seiner
   Jobs war grün, keiner übersprungen. Ein reiner Doku-PR überspringt seine schweren Jobs; sein Merge
   läuft auf `main` deshalb weiter voll (`docs-only.sh` wird nicht zweimal vertraut).

Der Gate-Job `ci` zählt die übersprungenen Jobs dann als grün und nennt den PR-Lauf mit Link. Jeder
Fehler beim Abfragen der GitHub-API ergibt «nicht geprüft», also einen vollen Lauf.

**Wöchentlicher Lauf:** Einmal pro Woche (Montag 02:23 UTC, `schedule`) prüft die CI `main` vollständig,
egal was vorher geschah. Er ersetzt das Sicherheitsnetz der vollen `main`-Läufe und frischt den
Build-Cache für `-race` auf, den nur `main` speichert.

**Gewinn:** Bei einem Merge, dessen Zweig `main` enthält, endet der `main`-Lauf nach rund einer
Minute statt nach rund 15. Geprüft an den Merges der Nacht vom 15. November (#158, #162 bis #165,
#167: übernommen; #166 Doku: voll; ein Zweig hinter `main`: voll).

**Korrektur vom 16. November 2026: Abfrage ohne `status=success`.** Nach dem Merge von #178 lief
`main` voll, obwohl alle vier Bedingungen galten: Der PR-Lauf 43963247669 war 28 Sekunden vor dem
Merge fertig, alle 20 Jobs grün. Das Skript fragte die Läufe des PR-Kopfs mit dem Filter
`status=success` ab, und GitHub lieferte diesen Lauf damit nicht, auch 20 Minuten später nicht;
ohne den Filter kam er mit `status=completed` und `conclusion=success`. GitHub beantwortet den
Filter offenbar aus einem Index, der einen beendeten Lauf verpassen kann. Neu fragt das Skript ohne
Statusfilter ab und prüft `status` und `conclusion` des Laufs selbst; die Prüfung jedes einzelnen
Jobs bleibt. Geprüft mit dem geänderten Skript an echten Commits auf `main`: Merge #178 jetzt
übernommen (vorher nicht), #173 und #176 übernommen, #177 (Doku, Jobs übersprungen) voll, ein
Commit ohne zwei Eltern voll, ein roter PR-Lauf wird nicht gewählt.

## Nachtrag vom 26. November 2026: `store` in drei Gruppen, Android in zwei Jobs mit eigenem Build-Cache

**Anlass:** Marco fragte am 26. November 2026, ob sich die zwei längsten Jobs beschleunigen lassen.
In den zehn grünen PR-Läufen vom 23. bis 26. November brauchte `server tests (2)` 1168 bis 1520 s,
`android (builds, lint, generators, tests)` 806 bis 1398 s; das Gate war nach 1313 bis 1596 s
fertig, der nächste Job (die e2e-Shards) nach 12 bis 13 Minuten. Vorgabe: keine Prüfung fällt weg
oder wird gelockert. Messwerte und Belege: `docs/perf/ci.md`, Abschnitt 8.

### `internal/store`: der Engpass ist eine Sperre je Prozess

- **Belegt mit Profilen:** Das Mutex-Profil schreibt 99 % der Wartezeit einer einzigen Sperre zu,
  `allocatorMu` in `modernc.org/libc` (`Xmalloc`, `Xfree`, `Xrealloc`). Alle SQLite-Verbindungen
  eines Prozesses holen ihren Speicher über diese Sperre. Unter `-race` dauert jeder Aufruf länger,
  also hält jeder Test die Sperre länger. Das Paket nutzte deshalb 2,7 von 24 Kernen, und
  parallel brauchte es doppelt so viel CPU-Zeit wie nacheinander (Drehen an der Sperre).
- **Ein zweiter, lokaler Engpass:** Der Testhelfer `replay` spielt die eingefrorenen Stände
  zeilenweise ein, im Rollback-Journal mit vier fsync je Zeile; ein Test machte 6539 fsync. Lokal
  warteten die parallelen Tests aufeinander im Journal des Dateisystems. Behoben mit
  `synchronous(OFF)` für die Wegwerf-Datei (auch in `internal/radio` und `internal/library`). Auf
  dem Runner bringt das allein nichts Messbares: Dort entscheidet die Sperre.
- **Entscheid:** Ein Paket kann in `server/testgroups.txt` nach Testnamen geteilt werden. Jede
  Zeile des Pakets bis auf eine trägt ein Muster über die Namen seiner Tests auf oberster Ebene;
  eine Zeile mit Muster läuft mit `-run=<Muster>` und überspringt, was die Zeilen darüber schon
  nehmen; die Zeile ohne Muster überspringt alle Muster (`-skip`). So landet jeder Test in genau
  einer Gruppe, auch ein neuer, nach demselben Prinzip wie die Gruppe `rest` für Pakete.
  `scripts/testgroups.sh --check` (Teil von `server:lint`) listet die Tests des Pakets, zählt jede
  Gruppe und schlägt fehl, wenn eine Gruppe keinen Test nimmt, die Summe nicht stimmt, eine Gruppe
  neben dem geteilten Paket noch etwas anderes hält oder ein Muster `/` enthält.
- `internal/store` läuft in drei Gruppen: `2` die Migrationstests (68), `2b` die Migration aller
  eingefrorenen Stände und die Wiederherstellung (7), `2c` der Rest (64). Lokal unter `-race` je
  rund 360 s, zusammen 139 von 139 Tests.
- **Verworfen:** (a) Mehr Kerne für ein Paket: Die Sperre gilt je Prozess, mehr Kerne drehen nur
  mehr. (b) Mehrere `go test`-Prozesse in einem Job: Die Runner haben vier Kerne, und eigene Jobs
  kosten keine Wartezeit (22 Jobs starteten in einem Lauf innerhalb von 3 s). (c) Die
  Race-Instrumentierung in `modernc.org/...` abschalten (`-gcflags=modernc.org/...=-race=false`): Das
  macht dieselben Tests 3,9-mal schneller, ist aber eine Lockerung. Marcos Entscheid vom
  26. November 2026: so lassen. Das Gate wartet ohnehin auf den längsten e2e-Shard; schnellere
  `store`-Jobs sparten nur Runner-Minuten.

### Android: der Build-Cache wurde nie erneuert

- **Belegt:** Der Gradle-Cache hat einen Schlüssel nur aus den Build-Dateien. Sein Hauptschlüssel
  traf bei jedem Lauf, also speicherte er nie neu, und der Build-Cache darin blieb der vom
  23. November. Lokal ein Doppellauf mit leerem Build-Cache und `clean` dazwischen: 6 min 3 s, dann
  8 s. Die Aufgaben lassen sich also zwischenspeichern; es fehlte nur ein aktueller Cache.
- **Entscheid:** Der Build-Cache (`~/.gradle/caches/build-cache-1`) bekommt einen Cache für sich:
  Schlüssel mit `run_id`, `restore-keys` als Präfix, gespeichert nur auf `main`, gelesen überall,
  wie der `-race`-Cache der Go-Tests. Er wird über den alten Build-Cache des Abhängigkeits-Caches
  gelegt; ein Lauf vor dem ersten Speichern auf `main` behält also, was er hatte. Der Wochenlauf
  beginnt leer, damit Einträge alter Quellen nicht immer mitreisen.
- **Der Job `android` läuft auf `main` auch dann, wenn ein PR-Lauf den Inhalt schon geprüft hat**
  (Ausnahme zum Nachtrag vom 15. November): Sonst bliebe der Cache hinter den Quellen zurück, denn
  fast jeder Merge übernimmt das Ergebnis seines PR-Laufs. Ein `main`-Lauf nach einem Merge dauert
  dadurch so lange wie die Android-Jobs, nicht mehr eine Minute. Niemand wartet auf ihn.
- **Zwei Jobs statt einem:** Nach einer Änderung an `core:api` brauchte der Job auch mit aktuellem
  Cache 16 Minuten, davon die drei R8-Release-Builds allein 5,5. Der Job ist jetzt eine Matrix mit
  zwei Hälften: `debug` (`ciCheckDebug` und `ciTest`: Lint, Tests, Debug-Builds, Generatoren) und
  `release` (`ciCheckRelease`: die R8-Builds). `ciCheck` hängt genau an diesen zwei Hälften und an
  nichts sonst (`android/build.gradle.kts`); die beiden Jobs zusammen führen also aus, was
  `task ci:android` ausführt. Lokal heissen die Hälften `task ci:android:debug` und
  `task ci:android:release`. Jede Hälfte hat einen eigenen Build-Cache.
- **Verworfen:** (a) Drei Teile (Lint, Builds, Tests): Der längste Teil wäre derselbe, und es wäre
  ein Job mehr. (b) Vier Gradle-Worker statt drei: 889 statt 957 s, zu wenig für den Speicher, den
  es auf dem Runner kostet. (c) Den Build-Cache auch aus PR-Läufen speichern: Einen solchen Cache
  liest nur der eigene Zweig, und er verdrängt die Caches von `main` aus den 10 GB.

### Was gleich bleibt

Kein Test und keine Prüfung fällt weg: `internal/store` hat vor und nach der Teilung 139 Tests
auf oberster Ebene, und alle laufen in genau einer Gruppe (`--check` zählt es bei jedem
`server:lint`). `ciCheck` und `ciTest` hängen an denselben 21 und 13 Aufgaben wie vorher; `ciTest` ist unverändert (lokal 1857 JVM-Testfälle, keiner übersprungen). `-race`
bleibt in jeder Gruppe, `checkptr` bleibt wie am 4. November entschieden. Das Gate verlangt
weiter jeden Job; ein Matrix-Job ist nur grün, wenn jede seiner Hälften grün ist.

## Nachtrag vom 28. November 2026: Übernahme ohne informative Jobs, Selbstprüfung der Workflows

**Anlass:** Messung aller 505 Läufe vom 29. Oktober bis 28. November 2026
(`docs/perf/ci-optimierungsplan.md`, Welle 1, von Marco am 28. November 2026 freigegeben: M1, M2,
M3, M7). Messwerte: `docs/perf/ci.md`, Abschnitt 9.

### main übernimmt auch bei rotem informativem Job, gebunden an den geprüften Baum (M1)

- **Befund:** `already-tested.sh` verlangte `success` von **jedem** Job des PR-Laufs, auch von
  `android-emulator (flow, informational)`, den das Gate nicht verlangt. 10 der 39 vollen
  `main`-Läufe seit dem 16. November liefen nur deshalb voll; 18 grüne PR-Läufe trugen einen roten
  Emulator-Job.
- **Entscheid:** Die Jobs ausserhalb des Gates stehen in **`.github/ci-informational-jobs.txt`**.
  Das Skript nimmt sie aus der Prüfung aus und meldet ihr Ergebnis; das Gate des `main`-Laufs nennt
  es («Informational jobs of that run: …»). Was das Gate nicht verlangt, verlangt auch die Übernahme
  nicht.
- **Neue Bedingung 5:** Der Job `changes` eines PR-Laufs schreibt den Baum, den der Lauf prüft, als
  Annotation `tested-tree`. Übernommen wird nur, wenn genau dieser Baum der Baum auf `main` ist.
  Ein Lauf desselben PR-Kopfs gegen einen anderen Zielzweig prüfte einen anderen Merge und zählt
  nicht mehr (Sicherheitsprüfung vom 28. November 2026). Das Feld `pull_requests` eines Laufs taugt
  dafür nicht: GitHub nennt dort nur **offene** PRs, und in allen 505 Läufen war es leer.
- **Neue Bedingung 6:** Ändert der Merge etwas unter `.github/`, läuft `main` voll. Der PR-Lauf prüfte
  mit seinem eigenen Workflow, seinen Skripten und seiner Jobliste; eine Änderung dort ist eine
  Änderung der Prüfung selbst (Sicherheitsprüfung vom 28. November 2026, solange `main` keinen
  Pflicht-Check und kein CODEOWNERS hat). Auch der Merge, der diese Bedingung einführt, läuft deshalb
  voll.
- PR-Läufe von vor dieser Änderung haben keine Annotation; ihre Merges laufen auf `main` voll.
- **Fremdtext:** Dateinamen, Jobnamen und Annotationen stammen aus dem PR. Der Job `changes` schaltet
  beim Auflisten der Dateien die Workflow-Befehle ab (`::stop-commands::`), und das Skript gibt
  Fremdtext nur einzeilig und bereinigt aus. Sonst könnte eine Datei mit dem Namen eines
  Workflow-Befehls diesen im Job ausführen.
- Mehrdeutiges gilt nicht als Beleg: zwei Jobs namens `changes (docs only?)`, zwei Annotationen
  `tested-tree`, ein Job ohne Ergebnis. Auch die alte Befehlsform `##[`, die überall in einer Zeile
  wirkt, wird in ausgegebenem Fremdtext aufgebrochen.
- Selbsttest: `.github/scripts/already-tested.test.sh` (15 Fälle, gefälschtes `gh`).

### apt mit Zeitgrenze (M2)

Jeder apt-Aufruf in `.github/actions/system-packages` endet nach 300 s (die ganze Aktion brauchte im
Monat höchstens 289 s), dazu `Acquire::Retries 3` und 30 s Zeitgrenze je Verbindung. Ein hängender
Spiegel (17 Jobs im Monat bis zur Jobgrenze von 30 bis 45 Minuten) wird damit nach Minuten rot, mit
der Meldung «apt hang (infrastructure)». Kein Test wird wiederholt.

### Rote Läufe lesen (M3)

`scripts/ci-failures.sh` (`task ci:failures RUN=<id>`) gibt je rotem Job den roten Schritt, eine
Einordnung, einen Hinweis auf denselben Job grün auf demselben Commit und die Fehlerzeilen des
Schritts aus, höchstens 80 Zeilen. Das ganze Log bleibt auf der Platte. An den zehn Referenzläufen
der Messung: Ursache 10 von 10, zusammen rund 9 200 statt 3,05 Millionen Tokens. «Ohne Codeänderung
gescheitert» meldet es nur, wenn derselbe Job in einem anderen Versuch **desselben** Laufs grün war:
Jeder Versuch prüft denselben Commit, ein anderer Lauf desselben Kopfs vielleicht einen anderen Merge.
Steuerzeichen aus Logs erreichen das Terminal nicht, und die Ausgabe sagt, dass ihr Text Daten sind
(auch aus Fork-PRs). Das Gate schreibt seine Ergebnistabelle zusätzlich auf die Übersichtsseite des
Laufs.

### Die Workflows prüfen sich selbst (M7)

- **`task ci:workflows`**, Teil von `task ci` und des Jobs `server (generators, linters)`:
  actionlint (mit shellcheck über jeden `run:`-Block), die **stehende Regel** dieses ADR als Programm
  (`server/tools/ciworkflows`: jeder Job steht in `needs` des Gates und in dessen Prüfliste, oder er
  ist informativ mit `continue-on-error`; das Gate hat `if: always()` und heisst `ci`) und die
  Selbsttests der zwei CI-Skripte. Bisher war die Regel nur ein Kommentar.
- actionlint kommt wie `task` als Release-Archiv mit geprüfter SHA-256 (`.github/actionlint-checksums.txt`).
  `go tool` aus `server/go.mod` geht nicht: actionlint 1.7.12 verlangt `go.yaml.in/yaml/v4`
  v4.0.0-rc.3, gosec (über golangci-lint) rc.6; Go nimmt im gemeinsamen Modulgraph die höhere, und
  gegen deren geänderte Schnittstelle übersetzt actionlint nicht.
- `.github/actions/android-sdk` ersetzt die Wrapper-Prüfung und den JDK-/SDK-Schritt, die drei Jobs
  je einmal trugen. Eingaben und Matrixwerte erreichen die Skripte über `env:`.

## Nachtrag vom 28. November 2026, Welle 2: e2e-Dauern aufgefrischt, Web-Job in zwei Hälften

**Anlass:** `docs/perf/ci-optimierungsplan.md`, Welle 2 (M4, M5), von Marco am 28. November 2026
freigegeben. Messläufe auf einem Wegwerf-Zweig ohne PR: `docs/perf/ci.md`, Abschnitt 10.

### e2e: Dauertabelle (M4 a)

- `web/e2e/durations.json` stammte aus zwei Läufen vom 3. November (vor der Firefox-Auswahl, den
  kürzeren Wartezeiten und 34 der 106 heutigen Spec-Dateien). Neu aus drei grünen PR-Läufen seit dem
  26. November: 806 Tests, sechs Shards planen auf 588 bis 590 s.
- `task web:e2e:durations` formatiert die Tabelle jetzt mit prettier. Die von `durations.mjs`
  geschriebene Form hätte `web:lint` rot gemacht (im Messlauf gesehen).

### Sieben e2e-Shards statt sechs (M4 b, Marcos Entscheid vom 28. November 2026)

- Gemessen auf dem Runner mit der neuen Tabelle, 6 bis 9 Shards je viermal (`docs/perf/ci.md`,
  Abschnitt 10; bei 6 Shards nur drei vollständige Durchgänge). Längster Shard im Median: 6 → 12,5 min,
  7 → 10,1, 8 → 10,0, 9 → 8,8; Runner-Minuten der e2e-Jobs je Lauf 69, 69, 74, 77.
- **Entscheid: sieben.** Fast der ganze Gewinn von acht Shards (−2,4 statt −2,5 min) ohne messbar
  mehr Runner-Minuten (69 wie bei sechs; acht kosten 5 mehr). Neun wären am schnellsten (−3,7 min),
  kosten aber 8 Runner-Minuten mehr je Lauf; Marco wählte am 28. November 2026 sieben. Die Annahme
  vom 3. November, ab etwa acht Shards wachse der feste Anteil schneller als der geteilte schrumpfe,
  bestätigt sich für die Dauer **nicht** (neun sind am schnellsten), wohl aber für die
  Runner-Minuten: Jeder Shard zahlt rund 2,5 min fest.
- `ci.yml` (Matrix 1 bis 7) und `CI_E2E_SHARDS` in `web/Taskfile.yml` nennen dieselbe Zahl;
  `task web:e2e:shards:check` prüft den Plan mit sieben Shards: 806 Tests, jeder in genau einem.

### Web-Job in zwei Hälften (M5)

- Wie `android` seit dem 26. November eine Matrix: `web (generators, linters)` führt
  `task ci:web:lint` aus, `web (unit tests)` `task ci:web:test`. `task ci:web` ist genau beides, also
  bleibt `task ci` gleich (dieser ADR: CI ruft nur Teile von `task ci`).
- Grund: Linter (126 bis 214 s) und Vitest (239 bis 412 s) liefen nacheinander, und ein roter Linter
  verdeckte das Ergebnis von Vitest. Gemessen getrennt: Lint-Hälfte 3,3 min (zwei vollständige
  Durchgänge), Test-Hälfte 4,9 bis 7,8 min, bisher der ganze Job im Median 7,8 min. Die Lint-Hälfte
  ist weiter **ein** Schritt: Ein roter Generator-Abgleich verdeckt dort noch die Linter.
- `server (generators, linters)` führt `server:gen`, `server:gen:check` und `server:lint` als drei
  Schritte aus; die zwei späteren laufen auch nach einem roten Vorgänger, aber nicht nach einem roten
  `server:gen` (halb erzeugter Code gäbe nur Folgefehler).

## Nachtrag vom 28. November 2026, Welle 3: Cache-Haushalt, Release-Härtung

**Anlass:** Marcos Auftrag vom 28. November 2026, den Rest des Plans selbständig umzusetzen
(`docs/perf/ci-optimierungsplan.md`, M6 und H2). Messwerte: `docs/perf/ci.md`, Abschnitt 11.

### Weniger Cache-Einträge je Push auf main (M6)

- **Befund:** Die Caches des Repos stehen dauernd an der Grenze von 10 GB, und GitHub verdrängt dann,
  was am längsten nicht benutzt wurde. Jeder Push auf `main` legte acht neue `go-race`-Einträge an
  (rund 1,6 GB, Schlüssel mit Lauf-ID) und zwei `gradle-build`-Einträge (rund 1,7 GB). Am
  28. November waren die apt-Caches von `server` und `android-e2e`, zuletzt um 08:56 benutzt, um
  10:32 weg; jeder Server-Job lud danach 63 MB und installierte 87 Pakete neu, bis ein main-Lauf um
  15:44 sie wieder speicherte. In PR-Läufen scheitert das Speichern ohnehin am Wettlauf der acht
  Gruppen («Unable to reserve cache … another job may be creating this»).
- **Entscheid:** `go-race` hat einen Eintrag **je Tag** und Gruppe (Datum UTC im Schlüssel). Ein
  Cache desselben Tages übersetzt nur, was seit dem Speichern geändert wurde. `gradle-build` bleibt bei
  einem Eintrag je Push: Ein veralteter Build-Cache machte die Android-Jobs am 26. November drei- bis
  viermal langsamer.
- Der Gradle-Abhängigkeits-Cache (`gradle-deps-…`, vorher `gradle-…`, 2,3 GB) trägt den Build-Cache
  nicht mehr mit, der darin eine veraltete Kopie seines ersten Laufs war; der Build-Cache hat seinen
  eigenen Cache. Er wird nur noch auf `main` gespeichert, nur wenn sein Schlüssel fehlte, und vor dem
  Speichern wird `build-cache-1` gelöscht. Ein Ausschlussmuster (`!…/build-cache-1`) wirkt dafür
  nicht: `actions/cache` gibt `~/.gradle/caches` als ein Verzeichnis an tar (Review vom 28. November
  2026). Die Pfadliste bleibt gleich, damit der alte Eintrag als Rückfallebene gefunden wird (die
  Cache-Version ist ein Hash der Pfadliste). Bisher speicherte jeder PR, der eine Build-Datei änderte,
  eine eigene Kopie von rund 2,5 GB.
- `go-race` speichert nur, wenn der Eintrag des Tages fehlte.
- `android-contract` liest die Caches des Jobs `android` mit (wie der Emulator-Job), statt eine eigene
  Kopie (`gradle-e2e-…`, 0,5 GB) zu halten; beide lesen zusätzlich den Debug-Build-Cache von `main`.

### Release-Workflows (H2)

- `release.yml`: keine Rechte auf Workflow-Ebene, der Job nennt seine eigenen; der Checkout hinterlegt
  kein Token; Tag, Version, Image-Namen und Digests erreichen Manifest und `gh release` über `env:`.
- `release.yml` und `release-relay.yml` prüfen wie `release-android.yml`, dass der getaggte Commit in
  `main` oder einem Zweig `release/*` liegt (Backports), jeweils im ersten Job. Geprüft: `v1.44.1` besteht, ein Commit ausserhalb
  jedes Zweigs scheitert. Gegen einen Tag, der nicht sein soll, schützt das Tag-Ruleset (H1, Marco);
  diese Prüfung fängt das Versehen.
- **Release-Images ohne Build-Cache** (`no-cache: true`): Code, der in einem CI-Lauf auf `main` läuft
  (eine Entwicklungsabhängigkeit etwa), könnte Cache-Einträge schreiben, die der Release ins
  veröffentlichte Image übernähme (Sicherheitsprüfung, B-5). Gemessen auf dem Runner: 3,5 bis 4,0 min
  statt 2,3 bis 2,5 min je Release.

## Nachtrag vom 28. November 2026: Pull Requests prüfen, was ihre Änderung erreichen kann (M9)

**Anlass:** Marcos Auftrag vom 28. November 2026, den Rest des Plans selbständig umzusetzen
(`docs/perf/ci-optimierungsplan.md`, M9). Gemessen an 185 gemergten PRs seit dem 29. Oktober: 29 %
reine Doku (übersprang schon vorher alles), 9 % nur Deploy-Dateien, 6 % nur Android; Server- und
Web-Änderungen brauchen immer den e2e-Pfad.

- **Entscheid:** Der Job `changes` ordnet die Pfade eines PR mit `.github/scripts/affected-jobs.sh`
  sieben Jobgruppen zu (`server`, `relay`, `android`, `android_server`, `web`, `e2e`, `image`),
  transitiv: Generatoren (`api/openapi.yaml`, `web/src/locales`, `packages/design-tokens`), über
  Modulgrenzen gelesene Testdaten und die Binaries, die die e2e-Suite und die Android-Vertragsjobs
  starten. Ein Job läuft, wenn seine Gruppe betroffen ist.
- **Sicherer Ausfall:** Unbekannte Pfade, die Workflows und ihre Skripte, Taskfiles,
  `.tool-versions` und die Lock-Dateien lösen alles aus. Ein Push auf `main`, der Wochenlauf und ein
  Lauf von Hand prüfen immer alles.
- **Gate:** «skipped» ist grün, wenn `changes` erfolgreich war und eine der drei Antworten gab: nur
  Doku, schon geprüft, oder (neu) «Gruppe nicht betroffen». Jedes andere «skipped» bleibt rot.
- **Volle Regression:** Der Merge eines gefilterten PR wird nie übernommen, denn `already-tested.sh`
  nimmt keinen übersprungenen Job als Beleg. `main` prüft den gemergten Inhalt also voll. Ein Bruch
  über eine Kante, die die Karte nicht kennt, zeigt sich dort, nicht vorher; es gibt keinen
  automatischen Revert.
- **Die Karte gegen den Code:** `.github/scripts/affected-jobs.test.sh` hält die Karte in 41 Fällen
  fest und gegen alle 161 Lesezugriffe über Modulgrenzen, die eine Suche heute findet: relative Pfade
  aus einem Modul hinaus in Go, Kotlin, Gradle, TypeScript, Shell und den Taskfiles,
  `filepath.Join("..", …)`, die Muster `$(cd ../x && pwd)/datei` und `go -C ../server tool`,
  Pfadtexte in Dateien, die die Wurzel des Repos berechnen (`logoMark.test.ts`,
  `StringsGeneratorTest`), und die Quellen von `COPY` in den Dockerfiles. Je Fund ordnet der Test
  der lesenden Datei ihre Jobgruppe zu; eine lesende Datei ohne bekannte Gruppe macht ihn rot. Er
  läuft auf jedem PR im Job `changes` selbst (Bash und `git grep`, rund vier Sekunden): Ein PR, der
  einen neuen Lesezugriff einführt, den die Karte nicht abdeckt, macht das Gate rot, bevor er einen
  nötigen Job überspringt. Ein Mutationstest belegt es: Fehlt eine der Kanten (Launcher-Icons → web,
  Mock-Server → android_server, App-Icon → android, LICENSE → relay, relay → android_server,
  Store-Grafiken → web), meldet der Test sie.
- **Gate und Bedingung passen zusammen:** `ciworkflows` (`task ci:workflows`) prüft, dass ein Job,
  der nur für seine Gruppe läuft (`run_<gruppe>` in `if:`), im Gate genau dieselbe Gruppe nennt,
  dass das Gate die Gruppe aus der Ausgabe von `changes` liest und dass ein Job ohne Gruppe kein
  Flag trägt.
- **Doku enger gefasst:** `docs-only.sh` zählt Markdown in `server/`, `relay/`, `android/`,
  `testdata/`, `scripts/` und `api/` nicht mehr als Doku, ebenso `docs/store/grafiken/` (der
  Logo-Test liest sie). Release-Notes gelten nur noch auf der obersten Ebene und unter `docs/` als
  Doku.
- Restrisiko, benannt: Die Suche sieht keinen Pfad, der aus Variablen in anderer Form zusammengesetzt
  wird, und keinen, den ein Werkzeug aus seiner eigenen Konfiguration liest. Das deckt nur der volle
  Lauf auf `main` auf. Wer einen solchen Lesezugriff über Modulgrenzen schreibt, trägt ihn in
  `affected-jobs.sh` und in die Tabelle des Tests ein.
