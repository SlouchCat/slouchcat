# Entwicklungs-Setup

Entwicklungsrechner: Linux Mint 22.3 (Ubuntu noble), NVIDIA RTX 4070. Alles ausser Docker und ffmpeg
wird ohne Root unter `~/.local` installiert; die gepinnten Versionen stehen in `.tool-versions`.

## Ohne sudo (von Claude eingerichtet)

| Werkzeug | Ort | Einrichtung |
|---|---|---|
| Go | `~/.local/share/go`, Link `~/.local/bin/go` | offizielles Tarball von go.dev, SHA-256 geprüft |
| Node.js 24 LTS | `~/.local/share/node`, Link `/usr/local/bin/node` | vorhanden |
| pnpm | `npm install -g pnpm`, Link `~/.local/bin/pnpm` | |
| task | `go install github.com/go-task/task/v3/cmd/task@latest`, Link `~/.local/bin/task` | |
| sqlc, goose, oapi-codegen, golangci-lint | `go tool` aus `server/go.mod` | keine Installation nötig |

Go-Upgrade: neues Tarball nach `~/.local/share/go` entpacken, `go.mod` und `.tool-versions` anpassen.

## Mit sudo (macht Marco im Terminal-Panel)

```bash
sudo bash scripts/install-system-tools.sh
```

Installiert Docker Engine mit Compose- und Buildx-Plugin aus Dockers apt-Repository sowie ffmpeg/ffprobe
aus Ubuntu und nimmt den Benutzer in die Gruppe `docker` auf. Bis zur nächsten Anmeldung:
`sg docker -c 'docker ps'`.

Optional für GPU-Tests im Container (ab M1b): NVIDIA Container Toolkit.

## Audioausgabe für die Playwright-Tests

Firefox spielt einen reinen Audiotitel über seine Audioausgabe ab. Fehlt sie (Rechner ohne Soundkarte,
Container, CI-Runner), scheitert die Ausgabe, das Medienelement meldet `MediaError 3`
(«OnMediaSinkAudioError»), hls.js hängt es ab und neu an — und die Hörbuch-Specs kommen nie vom Fleck.
Auf Marcos Rechner ist nichts zu tun. Auf einem Rechner ohne Soundkarte braucht es einmal eine
Dummy-Ausgabe, wie sie auch die CI einrichtet:

```bash
sudo apt-get install -y pulseaudio pulseaudio-utils
pulseaudio --start --exit-idle-time=-1
pactl load-module module-null-sink sink_name=slouchcat_null
```

Chromium und WebKit bringen ihre eigene Ausgabe mit und brauchen das nicht.

## Täglich

```bash
task ci          # alles bauen und prüfen (wie CI)
task gen         # Codegen: OpenAPI → Go und TypeScript, sqlc, Tokens
task dev         # Server und Vite-Dev-Server
task --list      # alle Ziele
```

## Schnelle Läufe beim Arbeiten

```bash
task check:fast                          # Web und Server nebeneinander, nur was sich geändert hat
task web:check:fast                      # ESLint (Cache) und Prettier auf geänderten Dateien,
                                         # tsc, `vitest related` für die geänderten Quellen
task web:check:fast BASE=main            # dasselbe gegen main statt gegen den letzten Commit
task e2e:quick -- e2e/login.spec.ts      # Chromium, keine Wiederholung, Abbruch beim ersten Fehler;
                                         # baut Web und Server nur, wenn sie sich geändert haben
task e2e:quick E2E_PORT=8197 -- e2e/home.spec.ts:134
task ci:e2e:parallel                     # die ganze e2e-Suite als 3 Shards nebeneinander
task ci:e2e:parallel E2E_SHARDS=2 E2E_PORT=8197
task web:e2e:shards:check                # Selbsttest: jeder Test in genau einem CI-Shard
task web:e2e:durations RUNS="<id> <id>"  # Dauertabelle der Shards aus CI-Läufen auffrischen
```

- `task check:fast` ersetzt `task ci` nicht. Wann welcher Lauf fällig ist, steht im Abschnitt
  «Teststufen» weiter unten. Der Server-Teil heisst `task server:check:fast` (`server/Taskfile.yml`):
  `go vet` und `go test` ohne `-race` für die geänderten Pakete und ihre direkten Abnehmer.
- `task ci:e2e:parallel` (auch in `task ci`) startet je Shard einen eigenen Server auf
  `E2E_PORT`, `+10`, `+20` … (Standard 8097) samt den Mocks auf den neun Ports darunter, mit eigenem
  Artefaktordner (`web/node_modules/.playwright-parallel/shard-<i>/`, Protokoll `run.log`) und einer
  Kopie von `testdata/media` aus harten Links, weil Specs dort Dateien anlegen. Belegte Ports
  brechen den Lauf vor dem Start ab. Am Ende steht ein zusammengeführter HTML-Bericht in
  `web/playwright-report/` (`pnpm -C web exec playwright show-report`). Drei Shards sind der
  Standard: Jeder Shard ist ein Server, ein Browser und dessen Transcodes. `E2E_SHARDS=1` ist ein
  einziger Lauf. Gemessen am 3. November 2026 auf Marcos Rechner: 427 Tests in 7 min 56 s (die
  Shards 6,4, 7,0 und 7,9 min), vorher rund 25 min am Stück.
- Port 8096 gehört dem eigenen Entwicklungsserver; keine der Aufgaben benutzt ihn.
- Die e2e-Suite startet nicht `server/bin/slouchcat`, sondern `server/bin/slouchcat-e2e`, gebaut mit
  dem Build-Tag `radiotest` (`task server:build:e2e`; `task build` und `task e2e:quick` bauen beide
  Binaries, die CI lädt beide als Artefakt hoch). Nur dieser Build liest die Katalog-Adresse des
  Webradios aus `SLOUCHCAT_TEST_RADIO_CATALOG_URL` und lässt das Radio die Loopback-Adresse anwählen;
  eine Umgebungsvariable schaltet das im normalen Build nie frei (Konzept 16, Plan 3.8). Katalog und
  Teststation liefert `web/e2e/radio-server.mjs` neun Ports unter dem App-Port (Standard 8088) aus
  `testdata/radio/catalog.json`. Das Docker-Image und `bin/slouchcat` tragen das Tag nie.
- `SLOUCHCAT_E2E_ARTEFACTS` und `SLOUCHCAT_E2E_MEDIA` verlegen Artefakte und Testmedien eines
  einzelnen Laufs (`web/e2e/env.ts`); `ci:e2e:parallel` setzt beide selbst.

## Teststufen

Entscheid vom 4. November 2026 (`docs/planning/04-entscheidungen.md`). Vorher war `task ci` vor
jedem Planschritt fällig. In der Woche vom 28. Oktober bis 3. November lief es deshalb 48-mal lokal
(rund 50 min je Lauf), oft von mehreren Agenten gleichzeitig, und derselbe Stand wurde bis zu
sechsmal voll geprüft. Jede Stufe prüft jetzt das, was sie wissen muss. Keine Prüfung fällt weg: Die
CI prüft weiterhin alles.

| Stufe | Wann | Was |
|---|---|---|
| 1. Arbeiten | nach Änderungen, vor jedem Commit eines Planschritts | `task check:fast`, dazu die Tests, die der Schritt anfasst: `go test ./internal/<paket> -run <Test>`, `task e2e:quick -- <spec>`, `task storybook:test` bei geänderten Bauteilen in `web/src/ui`. Belege (Befehl und Ausgabe) je Schritt |
| 2. Vor dem Pull Request | einmal, auf dem fertigen Zweig, durch den Koordinator | `task ci` grün, Belege in den PR, `docs/dev/CURRENT_STATUS.md` aktualisieren |
| 3. GitHub CI | jeder PR, jeder Commit auf main | alles (`.github/workflows/ci.yml`), Merge nur mit grünem Gate `ci` |

- Nach `task ci` folgende Korrekturen brauchen kein zweites `task ci`. `task check:fast` und die
  gezielten Tests genügen, weil die CI den neuen Stand vollständig prüft.
- Ein Verifier oder Reviewer wiederholt keinen Lauf auf derselben Commit-SHA. Er liest die Belege und
  prüft gezielt nach, wo er eine Lücke sieht.
- Nach drei erfolglosen Korrekturversuchen an derselben roten Prüfung wird angehalten und gemeldet.
  Übersprungene Tests, mehr Wiederholungen oder gelockerte Grenzen machen eine Prüfung nicht grün.

### Plätze für schwere Läufe

Die ganze Go-Suite unter `-race` (`task server:test`), die e2e-Läufe (`task e2e`,
`task ci:e2e:parallel`) und die Storybook-Screenshots (`task storybook:test`, `task
storybook:update`) laufen lokal über `scripts/heavy-slot.sh`. Auf der Maschine gibt es zwei Plätze
für alle Worktrees zusammen. Ein dritter Lauf wartet und meldet, wer die Plätze belegt:

```text
heavy-slot: all 2 slots are taken, waiting (SLOUCHCAT_HEAVY_SLOTS, docs/dev/SETUP.md):
  slot 1: 00:41:12 pid 81234 in /…/worktrees/a/web: pnpm exec playwright test
```

- `SLOUCHCAT_HEAVY_SLOTS=3` erlaubt drei Plätze, `0` schaltet das Warten ab. In der CI (`CI`
  gesetzt) wird nie gewartet.
- `task ci` wartet für jeden schweren Teil neu. Was darin schon einen Platz hält, wartet nicht ein
  zweites Mal.
- `task check:fast`, `task e2e:quick` und einzelne `go test`-Pakete warten nicht. Sie sind die
  schnellen Läufe beim Arbeiten.
- Die Sperrdateien liegen in `$XDG_RUNTIME_DIR/slouchcat-heavy/`. Sie verschwinden mit dem Prozess,
  der sie hält, auch wenn er abgebrochen wird. Nach einem Absturz bleibt also kein Platz belegt.
- Selbsttest: `task heavy-slot:check` (Teil von `task ci:web`).

## Schnelle und verlässliche Tests

Regeln für neue und geänderte Tests (Vitest und Playwright). Sie halten die Suite schnell und machen
einen roten Lauf zu einem Befund statt zu einem Würfelwurf.

- **Kein `waitForTimeout`, kein `sleep`, kein `setTimeout` als Warten.** Ein festes Warten ist auf
  einem schnellen Rechner zu lang und auf einem ausgelasteten Runner zu kurz. Stattdessen auf den
  Zustand warten: `await expect(locator).toBeVisible()`, `expect.poll(() => …)` für Server- und
  API-Zustand, `expect(async () => …).toPass()` für mehrere Schritte zusammen.
- **Zeit vorspulen statt abwarten.** In Playwright `page.clock` (installieren vor dem Laden, dann
  `runFor`/`fastForward`), in Vitest `vi.useFakeTimers()`. Beispiel: `admin-metadata.spec.ts` prüft
  den 10-Sekunden-Abruf in rund einer Sekunde. «Es passiert nichts» lässt sich nur so prüfen; mit
  der Wanduhr ist es ein Rennen.
- **Server-Sperren im Testserver verkürzen, nicht in jedem Test aushalten.** Die Spotlight-Sperre
  (ein Abruf pro Minute) läuft im e2e-Server mit 2 s (`SLOUCHCAT_TEST_SPOTLIGHT_REFRESH_GAP`, nur mit
  `--dev` wirksam); die echte Minute prüft genau ein Go-Test (`TestSpotlightRefresh`). Eine neue
  Sperre bekommt dasselbe Muster: Test-Einstellung nur im Entwicklungsmodus, Go-Test, dass sie ohne
  ihn nicht greift.
- **Medienverhalten in beiden Engines, Bedienung einmal je Seite.** Wiedergabe, Audio, Offline,
  Anmeldung und Einrichtung laufen in Chromium und Firefox (`FIREFOX_WHOLE_SPECS` in
  `web/playwright.config.ts`). Von jeder anderen Seite läuft genau ein Tastaturtest auch in Firefox:
  Er trägt `@keyboard` im Titel. Eine neue Seite braucht ihren Tastaturtest ohnehin (harte Regel 8);
  er bekommt das Tag.
- **Zeitbudgets nicht als harte Wanduhr-Grenze unter Shard-Last.** «Startet in unter 2 s» misst
  auf einem geteilten Runner den Runner. Budgets, die eine Spec-ID verlangt, gehören in eine eigene
  Messung (`docs/perf/`) oder bekommen eine Marge, die die Messung begründet; die Shards laufen
  parallel und teilen Rechner.
- **Deterministische Testmedien.** Nur Medien aus `testdata/generate-media.sh` (lavfi-Quellen, feste
  Dauer, feste Kapitel); was ein Test zusätzlich braucht, legt er selbst an und räumt es danach weg.
  Ein Test baut nie auf Daten eines anderen Spec-Files: Jeder Shard hat nur seine eigenen.
- **Keine Zustände über Testgrenzen.** Ein Timer, der nach dem Test noch läuft, trifft den nächsten
  (so die norigin-Fokuswiederherstellung in `JobsPage.test.tsx`, siehe `web/src/test/setup.ts`).
  `setup.ts` räumt die bekannten globalen Dienste nach jedem Test ab; ein neuer globaler Dienst mit
  Timern gehört dort dazu.
