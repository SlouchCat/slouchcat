# Aktueller Stand

Stand: 22. Oktober 2026. M0 bis M8 sind in `main` gemerged (`https://github.com/SlouchCat/slouchcat`,
Pull Requests #1 bis #6, #10, #11, #14 und #15; M8 am 22. Oktober 2026 auf Marcos Wort, Merge-Commit `f7c57e6`).
Das Docker-Image auf Marcos Rechner ist auf dem M8-Stand gebaut, der Container ist `healthy`. Der CI-Job hat
seit #14 ein Limit von 75 Minuten. Die Übergabe an eine neue Hauptsession steht in `docs/dev/HANDOVER.md`.
Nächster Meilenstein: M9 (Integrationen); Entscheidungsfrage 10 (Wünschen) ist vorab zu klären.

## Befund: die CI braucht eine Audioausgabe für Firefox (22. Oktober 2026)

Nach dem M8-Merge war `main` rot: `web/e2e/audio-offline.spec.ts:121` (Leertaste pausiert nicht) und
`web/e2e/audiobook.spec.ts:192` (Kapitel 9 kommt nicht von selbst) scheiterten auf dem GitHub-Runner,
beide nur in Firefox. **Es war keine M8-Regression:** der Merge-Commit `f7c57e6` trägt denselben Baum
wie die Branchspitze `f9cdb46` (`git diff` leer), und schon deren als grün geltender Lauf
(`gh run view 39733786941`) hatte `audiobook.spec.ts:169` im ersten Versuch rot und nur im Retry grün.
Die beiden Tests waren seit M6 grenzwertig.

**Ursache.** Firefox treibt einen reinen Audiotitel über seine Audioausgabe. Ein Runner hat keine, die
Ausgabe scheitert, das Medienelement meldet `MediaError 3` mit der Meldung «OnMediaSinkAudioError»,
hls.js hängt das Element daraufhin ab und wieder an — und der Player startet die Wiedergabe neu. Das
wiederholt sich etwa achtmal je Sekunde; die Wiedergabe kommt nie vom Fleck. Chromium bringt seine
eigene Ausgabe mit und ist nicht betroffen. Belegt mit einer Messsonde im Playwright-Container
(Ubuntu 24.04, ohne `/dev/snd`), die den Zyklus `suspend → error → emptied → loadedmetadata → play`
samt Fehlercode aufzeichnet; die Serverlogs der CI zeigen dasselbe Bild als Endlosschleife über
dieselben HLS-Segmente, während Chromium jedes Segment genau einmal holt.

**Behebung.** Die CI richtet vor dem Lauf eine Dummy-Ausgabe ein (`pulseaudio` mit
`module-null-sink`, neuer Schritt in `.github/workflows/ci.yml`, der laut scheitert, wenn die Ausgabe
fehlt). In derselben Container-Umgebung laufen damit beide Specs und die ganzen Audio-Specs
dreimal hintereinander grün (30 von 30, Laufzeiten wie in Chromium statt am Zeitlimit).
Für Rechner ohne Soundkarte steht derselbe Befehl in `docs/dev/SETUP.md`.

**Dabei gefundener Produktfehler.** Das Wiederanhängen des Medienelements läuft über den
HTML-Load-Algorithmus, der `paused` **ohne** `pause`-Ereignis setzt. Die Player leiteten ihren Zustand
nur aus Ereignissen ab und entschieden in `togglePlay` nach `element.paused`: In diesem Fenster zeigte
der Knopf «Pause», startete aber die Wiedergabe, und `loadedmetadata` spielte anschliessend auch gegen
einen Pausenwunsch weiter. `AudioPlayerPage`, `OfflinePlayer` und `VideoPlayer` merken sich jetzt den
Wunsch des Hörers (`wantPlaying`) statt des Elementzustands; ein Test in
`AudioPlayerPage.test.tsx` deckt beide Fälle ab (er scheitert ohne die Korrektur).

**Zweiter Fund.** Die CI lud die Playwright-Artefakte bei einem Fehlschlag nicht hoch, obwohl der
Workflow einen `if: failure()`-Schritt hat: beide Ordner beginnen mit einem Punkt, und
`actions/upload-artifact` überspringt versteckte Dateien ohne `include-hidden-files: true`. Bei den
roten Läufen lagen deshalb nur die `dockerbuild`-Artefakte vor und die Traces fehlten. Das Flag ist
jetzt gesetzt.

## Befund: zwei zeitabhängige Tests und eine doppelt laufende CI (22. Oktober 2026)

Die CI war seit Tagen unzuverlässig und zu langsam; sie blockierte M9. Drei Ursachen, keine davon im
M9-Code. Pull Request #20.

### Flackernder Backup-Test – **kein Produktfehler**

`TestRestoreRejectsInvalidArchives` meldete auf dem M9-Branch
(Lauf 39790013960, Job 119863260398) `corrupt payload: err = <nil>, want ErrBackupInvalid` und
anschliessend `slouchcat.db.restore exists after a refused restore`. Das liest sich wie der
schlimmste Fall: eine beschädigte Sicherung wird nicht abgewiesen. **Das ist es nicht.**

**Ursache.** Der Testhelfer `corrupt()` kippte das Byte in der *Mitte* der Archivdatei und erwartete,
dass die Wiederherstellung das Archiv abweist. Nicht jedes gekippte Byte ist aber ein Schaden: rund
2,6 Prozent der Positionen eines gzip-Stroms sind Füllbits der Deflate-Kodierung (Einträge in den
Huffman-Tabellen für Zeichen, die gar nicht vorkommen). Ein Kipp dort entpackt sich zu **exakt
denselben Bytes**. Die gzip-Prüfsumme läuft über die *entpackten* Daten, stimmt also zu Recht
weiterhin, die Datenbank ist bytegleich — das Archiv ist unbeschädigt, und es abzuweisen wäre
falsch. Welche Position die Mitte ist, hängt von der genauen Länge des Archivs ab, und die wandert:
der Schlüsselbund eines frischen Stapels ist zufällig, sein verschlüsseltes Setting ändert die
komprimierte Grösse (23 540 bis 23 555 Bytes über 2000 Läufe). Die zweite Meldung ist nur die Folge
der angenommenen Wiederherstellung, kein zweiter Fehler.

**Beleg, dass die Sicherung dicht ist.** Jedes einzelne Byte von fünf Archiven auf `main` und von
drei auf einem M9-Schema gekippt — 190 000 Kipps — ergab **keinen einzigen** Fall, in dem ein Archiv
angenommen wurde, dessen Nutzdaten sich unterschieden. Jeder Kipp wurde entweder erkannt
(gzip-Prüfsumme oder Flate-Fehler, rund 22 940 je Archiv) oder war ein Null-Effekt mit bytegleicher
Datenbank (rund 603 je Archiv). Die Integritätskette aus ADR 0009 — gzip-Prüfsumme,
`PRAGMA integrity_check`, Schemaprüfung — hat kein Loch. Marco kann einer eingespielten Sicherung
vertrauen.

**Behebung.** `corrupt()` geht von der Mitte aus weiter, bis der Kipp die Nutzdaten wirklich
beschädigt. Keine Zusicherung wurde aufgeweicht: der Test prüft weiterhin, dass beschädigte
Nutzdaten abgewiesen werden, nur jetzt zuverlässig.
`go test -run TestRestoreRejectsInvalidArchives -count=20 -race ./internal/admin/` ist grün.

**Offen.** Der genaue CI-Fall liess sich lokal nicht nachstellen (0 Treffer in 2900 Archiven auf
`main`); der Mechanismus ist bewiesen, und die Behebung nimmt dem Test die Abhängigkeit davon ganz.

### Flackernde Web-Tests: die erste Darstellung bezahlt die Übersetzung

`AdminHomePage.test.tsx` scheiterte auf `main` am reinen Dokumentations-Commit `09733c7`
(Lauf 39767863551) mit `Unable to find role="heading" and name "Verwaltung"`, und zwar mit leerem
`<body><div /></body>`: die App hatte noch gar nichts gezeichnet. Derselbe Fehler traf danach
`ProfileLimits.test.tsx` (Lauf 39837140392, `role="article"` mit Namen «Lena» nicht gefunden, Körper
mit Skelett und leerer Profilliste) — ein Test von 571, der den ganzen Lauf rot machte.

**Ursache.** Jede Seite der App wird über `lazyRouteComponent(() => import(...))` geladen. Die
*erste* Darstellung in einem Worker wartet deshalb darauf, dass Vite diesen Modulbaum übersetzt, und
diese Wartezeit fällt *innerhalb* des Ein-Sekunden-Fensters von `findBy` an — sie hat mit dem, was
der Test prüft, nichts zu tun. Lokal brauchte der erste Test von `AdminHomePage` 417 ms, auf einem
ausgelasteten Runner 1046 ms; jeder andere Test derselben Datei 8 bis 633 ms.

**Das ist eine ganze Klasse, kein Einzelfall:** 23 der 43 Testdateien, die `renderApp` benutzen,
stellen als Erstes eine lazy geladene Route dar. Jede von ihnen kann so rot werden.

**Behebung.** Nicht Datei für Datei, sondern einmal zentral in `web/src/test/setup.ts`: die
Seitenmodule werden in `beforeAll` aufgelöst, bevor ein Test etwas behauptet. Die Modulregistrierung
gilt je Worker, die erste Testdatei bezahlt sie, jede weitere bekommt sie geschenkt. Kein Zeitlimit
wurde erhöht und keine Zusicherung aufgeweicht. Kosten: die ganze Web-Suite braucht 9,17 s statt
8,31 s (+0,86 s bei 571 Tests). Beide vormals flackernden Dateien laufen zehnmal hintereinander grün
(97 bis 203 ms statt 417 ms).

### Jeder Commit startete zwei vollständige Läufe

`.github/workflows/ci.yml` lauschte auf `push` für *jeden* Branch **und** auf `pull_request`. Jeder
Commit auf einem Branch mit offenem Pull Request startete damit zwei vollständige Läufe, die sich
auf demselben Kontingent und demselben 75-Minuten-Limit gegenseitig die Rechenzeit wegnahmen: Lauf
39789916437 wurde bei 1 h 15 abgebrochen, während sein Zwilling 39790013960 weiterlief. `push` gilt
jetzt nur noch für `main`, alles andere wird über seinen Pull Request geprüft, `workflow_dispatch`
deckt einen Branch ohne Pull Request ab, und eine `concurrency`-Gruppe je Branch löst den Lauf des
überholten Commits ab. Läufe auf `main` werden nie abgebrochen.

### Ein Viertel der Zeit ging für Installationen drauf

Im Lauf 39789916437 vergingen **25 der 75 Minuten**, bevor `task ci` überhaupt begann: ffmpeg
11,5 min, Playwright-Browser 8,3 min, Testmedien 2,9 min, pulseaudio 1,7 min. Drei `apt-get update`
sind jetzt eines; die Playwright-Browser (~400 MB, ändern sich nur mit der Version aus der
Lock-Datei) und die erzeugten Testmedien werden zwischengespeichert, mit Schlüsseln, die sich
ändern, wenn ihre Eingaben sich ändern. `task ci` selbst ist unberührt und führt weiterhin genau
das aus, was es lokal ausführt (Plan Abschnitt 4).

**Ergebnis.** Zwei aufeinanderfolgende grüne Läufe auf dem Branch (39845660078, Versuch 1 und 2):
**30,3 min** und **37,2 min** gegen 47,5 / 53,6 / 55,9 min bei den grünen Läufen vorher und den
Abbruch bei 75 min. Das Setup braucht noch 1,5 bis 1,7 min statt 3 bis 25. Der Abstand zum
75-Minuten-Limit liegt damit bei 38 bis 45 Minuten statt bei 0 bis 20. Der grösste Einzelgewinn kam
nicht vom Caching, sondern vom Ende der Doppelläufe: `task ci` selbst braucht 28,4 bis 35,6 min
statt 43,5 bis 49,0 — vorher nahmen sich zwei gleichzeitige Läufe die Rechenzeit weg.

**Noch offen: die Go-Testsuite.** Sie bleibt der lange Pol. `internal/api` braucht allein rund
24 Minuten in der CI (382 s lokal), angeführt von `TestContractChildProfiles` mit 112 s in elf
nacheinander laufenden Unterfällen, die sich eine Fixture teilen und sie verändern. Kein Test in
`internal/api` ruft `t.Parallel()`. Das ist der nächste wirkliche Hebel und gehört in eine eigene
Runde. Nicht die Ursache waren argon2id (`internal/api` legt genau zweimal ein Konto über
`auth.New` an; das Paket `auth` nutzt längst billige Parameter und deckt die Produktionsparameter
mit `TestPHCDefaultParams` ab) und echte Wartezeiten (10 bis 50 ms in Warteschleifen).

## Meilenstein M8 – Administration und Benachrichtigungen

Vorab geklärt mit Marco (21. Oktober 2026, Entscheidungsprotokoll): Web-Push und E-Mail werden mit echten
Zugangsdaten geprüft, Telegram ist vollständig gebaut und nur gegen den Mock getestet; die Sicherung bleibt in
`/config/backups`, es gibt kein NAS-Ziel und keinen zweiten Mount. Die Architekturentscheide stehen in ADR 0009
(`docs/adr/0009-administration-benachrichtigungen.md`): Kanäle gehören dem Konto und werden nur im Hauptprofil
geändert, Zustellung über die Job-Queue mit Zeilen in `deliveries`, Fehlerring hinter der Redaction, Sicherung als
tar.gz (mit Schlüssel age-verschlüsselt), Wiederherstellung per bereitgelegten Dateien und Neustart durch `exec`
im selben Prozess, Update-Hinweis aus dem Release-Manifest, Statistiken aus `progress_events` (mit Gerät) und 400
Tage lang aufbewahrten Wiedergabesitzungen. Der Entwurf liegt in den Sitzungsnotizen (`m8-design.md`). Die Umsetzung
lief wie in M7 auf einer gemeinsamen Basis (Migration 00016, Vertrag 0.9.0 mit 145 Operationen, Modelltypen,
501-Stubs) mit sechs Executor-Agenten (Opus) in Git-Worktrees (S1 Assistent/Dashboard/Einstellungen/Update/Konten,
S2 Sicherung/Statistiken, S3 Kanäle/Zustellung/Ankündigungen, W1 Assistent/Übersicht/Navigation/Integrationen,
W2 Konten/Sicherung/Statistiken/Ankündigungen, W3 Kanal-Einstellungen/Push/Ankündigungsdialog/E2E-Mocks/Log-Scan),
danach ein Integrations-Fix, zwei Review-Agenten und zwei Korrektur-Agenten. Commits mit Bereich im Titel:

| Schritt | Commit | Inhalt |
|---|---|---|
| Vorab | `docs(decisions)` `ca4f29d` | Antworten zu Kanälen und Sicherungsziel |
| Basis | `feat(api,store,model,config,logging)` `cc1b4dc`, `fix(deploy)` `81fcf30` | Vertrag 0.9.0 (36 neue Operationen), Migration 00016 (`account_channels`, `telegram_links`, `push_subscriptions`, `deliveries`, `announcements`, `announcement_acks`, Standardkanäle), Konfiguration und Redaction für SMTP, Telegram, VAPID und Manifest; die Trakt Client ID wird jetzt im Compose durchgereicht (Lücke aus M7) |
| 1 Assistent | `feat(admin,api)` `24c06fa`, `feat(web)` `e8d4b34` | Pfadprüfung mit Dateisystem und Warnung, TMDB-Prüfung mit klarer Meldung, GPU-Erkennung mit Compose-Override zum Kopieren, inotify-Hinweis (A-1) |
| 2 Verwaltung, Dashboard | `feat(logging,jobs,library)` `5ae1c06`, `feat(auth)` `d90e366`, `24c06fa`, `feat(web)` `2316a7e`, `e8d4b34`, `feat(web)` `32a8334` | Fehlerring, Job-Zähler mit Erneut/Abbrechen, Speicher, Integrationsstatus, Konten mit eigener E-Mail/Passwort-Änderung, Übersicht mit Kartenraster und `AdminNav`, Integrationen (Schlüssel, SMTP, Telegram, Push, Update, öffentliche URL) (A-2, A-3) |
| 3 Sicherung, Update | `feat(admin,store,api)` `4bbf6fb`, `24c06fa`, `32a8334` | Sicherung mit/ohne Schlüssel, Liste, Download, Wiederherstellung mit Neustart, Update-Prüfung mit Mock-Manifest und Badge (A-4, A-5) |
| 4 Kanäle, Ankündigungen, Statistiken | `feat(notify)` `73d5616`, `feat(auth,jobs,library)` `fec54d8`, `feat(notify,admin)` `df35e4b`, `feat(web)` `93778e6`, `test(e2e)` `8cfae1a`, `9014e1d`, `32a8334` | Sender E-Mail/Web-Push/Telegram, Kanäle je Konto, Push-Abos, Telegram-Link, Zustellung über die Queue, Hooks (Einladung, Job-Fehler, Scan-Abbruch), Ankündigungen mit Dialog beim Start, Statistiken, Mocks und Log-Scan (A-6, A-7, A-8, A-9, E-8) |
| Integration | `fix(metadata,api)` `c81d763`, `fix(web)` `438841b` | Trakt-Fehler im Fehlerring, Geräte-ID in Fortschrittsereignissen, Fokus der AdminNav über Seitenwechsel, Spec-Anpassungen |
| Review-Korrekturen | `fix(web)` `74e1001`, `fix(server)` `9e7b973` | siehe unten |
| DB-Stand, Doku | `test(store,testdata)`, `docs(adr,perf,status,decisions)`, `test(web)` | `testdata/db/M8.sql`, `TestFrozenM8StateCoversM8Data`, Storybook-Baselines, ADR 0009, `docs/perf/M8.md` |

### Definition of Done (Plan M8 plus 6.2) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| A-1 frischer Stack: Konto, Bibliothek, GPU erkannt, TMDB-Schlüssel geprüft | PASS: NVENC mit `compose.nvidia.yaml` auf dem Entwicklungsrechner, inotify-Limit 256'774 ohne Warnung; ungültiger Schlüssel → «Ungültig», nicht erreichbar → eigene Meldung; QSV auf CT 190 nach dem Merge durch die Hauptsession | `docs/perf/M8.md` 3, `go test ./internal/admin -run 'TestSetupSystemInfo|TestCheckSetupPath|TestCheckSetupTMDBKey'`, `go test ./internal/api -run TestSetupChecksOverHTTP`, `web/e2e/setup.spec.ts` |
| A-2 jede Admin-Seite über eine Navigation erreichbar, gleiche Lint- und Tastaturchecks, Storybook-Screenshots ≤ 0.1 % | PASS: `/admin` mit Kartenraster (16 Karten) und `AdminNav` (15 Einträge), Kopfzeile mit einem Eintrag «Verwaltung»; 22 neue Baselines (AdminNav, CodeBlock, StatTile, LinkCard, Switch, MessageDialog, Page/WithSectionNav, AppHeader/WithUpdate), TextField-Baseline erneuert | `web/e2e/admin-home.spec.ts` («every admin page is reached through the admin navigation»), `task web:storybook:test` (155 Screenshots) |
| A-3 Dashboard mit Streams, Jobs, Speicher, Integrationen, 50 Fehlern; provozierter Fehler < 10 s | PASS: Trakt-Mock auf 503 → «Trakt: Fehler» in der Kachel und Eintrag im Fehlerring in < 10 s; p95 225 ms während des Erst-Scans (die Job-Tabelle wächst dabei auf 50'907 Zeilen), 1.3 ms im Leerlauf mit allen Jobs | `web/e2e/admin-home.spec.ts` (A-3), `go test ./internal/admin -run 'TestDashboard'`, `docs/perf/M8.md` 1 |
| A-4 Restore auf frischem Stack identisch; Archiv ohne Mediendatei; ohne Schlüssel Secrets neu | PASS: Tabellen-Hashes gleich (Go-Test und echter Lauf mit Neustart in derselben PID, 1.5–1.9 s), Archiv genau Manifest + DB (+ Schlüssel), ohne Schlüssel keine `v1:`-Werte und eigener `secret.key` | `go test ./internal/admin -run 'TestRestoreOnFreshStack|TestBackupArchive|TestBackupWithout'`, `go test ./internal/store -run 'TestApplyRestore'`, `m8-backup-evidence.sh` («A-4 Beleg: alle Vergleiche gleich»), `docs/perf/M8.md` 2, `web/e2e/admin-backup.spec.ts` |
| A-5 Mock-Manifest: neuer → Hinweis, gleich → keiner, abschaltbar | PASS, Badge nur für das Hauptkonto; Mock-Manifest 99.0.0 → Badge «Update auf Version» in der Kopfzeile, Prüfung aus → weg | `go test ./internal/admin -run 'TestUpdateCheck'`, `go test ./internal/api -run TestAdminSystemShapesAndUpdateNotice`, `web/e2e/admin-integrations.spec.ts` (Update-Prüfung und Badge) |
| A-6 Kanal-Mocks je Ereignistyp; je 1 echtes Gerät je Kanal | PASS gegen Mocks (SMTP mit AUTH, Push mit VAPID-JWT und aes128gcm entschlüsselt, Telegram-Bot-API mit Long-Polling) für neue Inhalte, Beschaffung (nur Hook), Fehler, Einladung, Ankündigung; echte Geräte: E-Mail und Web-Push prüft Marco nach dem Merge, Telegram ohne Bot (Entscheid) | `go test ./internal/notify/...`, `web/e2e/notification-channels.spec.ts`, Checkliste `m8-marco-tests.md` |
| A-7 Ankündigung beim nächsten Start, nach Quittierung weg; Gast mit E-Mail-Opt-in erhält Mail | PASS (zweites Profil sieht den Dialog, nach «Verstanden» nicht mehr, Hauptprofil weiterhin; Gastkonto bekommt die Mail im SMTP-Mock, Hauptkonto ohne E-Mail nicht) | `web/e2e/announcements.spec.ts`, `go test ./internal/notify -run 'TestAnnounce'`, `go test ./internal/api -run TestContractAnnouncements` |
| A-8 meistgesehene Titel, Schauzeit je Profil, Transcoding-Anteil, Geräteverteilung gegen Rohdaten | PASS: Fixture mit eigener Aggregation im Test verglichen; p95 1'333 ms bei 1 Mio. Ereignissen über 365 Tage; beendete Sitzungen bleiben 400 Tage | `go test ./internal/admin -run 'TestStats'`, `web/e2e/admin-stats.spec.ts`, `docs/perf/M8.md` 1 |
| A-9 Log-Scan über einen kompletten E2E-Lauf: 0 Treffer | PASS: Scan im Playwright-Teardown über das Server-Log (Stufe debug) auf 13 Fake-Secrets und Muster, 0 Treffer; Gegenprobe mit eingeschleustem Passwort schlägt an | `web/e2e/global-teardown.ts`, `go test ./internal/logging` |
| E-8 Kanal-Mocks erhalten die Nachricht mit Profilname (5.5) | PASS: Betreff `[e2e-owner] Neue Folge …` im SMTP-Mock; P6 erhält für eine unsichtbare Folge weder Eintrag noch Zustellung | `web/e2e/notification-channels.spec.ts` (E-8), `go test ./internal/notify -run 'TestNewEpisodes'` |
| Kinderprofil-Vertragstest 10.2 über alle neuen Operationen | PASS: 145 Operationen; Admin-Operationen 403 für Gäste und Profile, Kanaländerungen 403 für Nicht-Hauptprofile, Ankündigungen je Profil, keine Push-Secrets in Antworten | `go test ./internal/api -run 'TestContract'` |
| Tastaturtest je neue oder geänderte Seite (6.2 Pkt. 3) | PASS: `/setup`, `/admin`, `/admin/integrations`, `/admin/accounts`, `/admin/backup`, `/admin/stats`, `/admin/announcements`, `/settings/notifications`, Ankündigungsdialog, Benachrichtigungen (Link) | die genannten Specs |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M8.sql`; alle Stände M1a bis M8 migrieren bis 00016; kein Passwort im Klartext | `go test ./internal/store -run 'TestFrozen|TestMigration00016|TestDumpRoundTrip'` |
| Messwerte (6.2 Pkt. 5) | `docs/perf/M8.md`; Gerätecheckliste Desktop Chromium und Firefox bestanden, Android ausstehend (Marco, nach dem Merge), Apple ungeprüft (Entscheid 2) | `docs/perf/M8.md` 6 |
| `task ci` grün (6.2 Pkt. 2) | exit 0 (21. Oktober 2026, Stand `f945baa`): gen, gen:check, lint 0 issues (golangci-lint, itemsaccess, vet, eslint, stylelint, tsc, prettier), Go `-race` alle 30 Pakete inkl. `notify/{email,push,telegram}` und `version`, Vitest 570 (+ 4 + 2), Build, Playwright 392 von 392 in Chromium und Firefox (19.1 min) mit A-9-Scan über 56'706 Logzeilen (17 Secrets des Laufs, 0 Treffer), Storybook 155 Screenshots; danach nur Doku | `sg docker -c 'task ci'`, Protokoll `task-ci-1.log` im Scratchpad der Session |
| Code-Review | Server 0 kritisch, 5 wichtig, 14 klein, 10 Nits; Web 0 kritisch, 3 wichtig, 8 klein, 12 Nits; alle wichtigen und die meisten kleinen behoben (siehe unten) | Sitzungsnotizen `m8-review-server.md`, `m8-review-web.md`, `m8-fix-server-notes.md`, `m8-fix-web-notes.md` |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen» (keine Blocker): A-1 und A-6 PARTIAL, weil QSV auf CT 190, echte SMTP- und Web-Push-Geräte und die Android-Checkliste erst nach dem Merge möglich sind; eigene Läufe: Vertragstest `-race` ok (145 Operationen), notify/admin/store/logging/version ok, alle Stände M1a bis M8 migrieren, Lint und Codegen sauber, 35 M8-Specs in Chromium und 28 in beiden Browsern grün, Storybook 155, Sicherungsbeleg und GPU-Erkennung reproduziert, Statistik-Benchmark bestätigt (p95 36/140/426/1351 ms), Log-Scanner mit eingeschleusten Secrets geprüft (8 Treffer); Doku-Mängel behoben, offene Review-Punkte M-8, M-12, M-14 nachgetragen | Sitzungsnotizen `m8-verifier.md` |

### Befunde der Review-Passage und ihre Behebung

- **Server (wichtig):** (I-1) ein Neustart nach der Wiederherstellung ging verloren, wenn der geordnete Stopp in
  die 10-s-Grenze lief (laufender Stream) – jetzt werden hängende Verbindungen geschlossen und der Prozess startet
  trotzdem neu; (I-2) jedes Profil konnte die Kanäle des Kontos ändern und so die Benachrichtigungen mit Titeln anderer
  Profile auf sein Gerät umleiten – Kanaländerungen verlangen jetzt das Hauptprofil (403 `primary_profile_required`,
  Vertragstest); (I-3) der Transcoding-Anteil sah nur einen Tag, weil Wiedergabesitzungen nach 24 h gelöscht wurden –
  beendete Sitzungen bleiben 400 Tage; (I-4) ein Fehler beim Sichern der alten Datenbank vor der Wiederherstellung
  blockierte jeden Start – jetzt werden die Dateien ersatzweise verschoben; (I-5) «Abbrechen» für fehlgeschlagene
  Jobs antwortete 404 – erlaubt. Klein: Wiederholung einer fehlgeschlagenen Zustellung, scrypt-Arbeitsfaktor
  begrenzt, `git describe`-Versionen, E-Mail-Adresse folgt dem Konto, Versuchslimit und generische Meldung bei
  Testnachrichten, `QUIT`-Fehler nach angenommener Mail, begrenzte Namenssuche der Sicherung, Feed-URLs im Manifest,
  gemeinsame Helfer (`netguard`, Bot-Token-Form, TLS-Modi), drei ungenutzte Abfragen und der Push-Fehlerzähler
  entfernt.
- **Web (wichtig):** Schlüssel, SMTP-Passwort, Bot-Token und TMDB-Prüfung laufen über `useTransientMutation` (kein
  Secret im Query-Cache); die Fehlerliste des Dashboards hat stabile Schlüssel und meldet nur «n neue Fehler»; der
  Log-Scan kennt alle Fake-Secrets des Laufs (YouTube-Schlüssel, Gastpasswörter, Sitzungs-Cookie) und der E2E-Server
  loggt auf `debug`. Klein: alte Fehlermeldungen verschwinden beim nächsten Versuch, Neustart-Seite gibt nach 90 s
  auf, Abmelden beendet das Push-Abo des Geräts, `pushsubscriptionchange` im Service Worker, fehlgeschlagener Chunk
  des Ankündigungsdialogs reisst die Seite nicht mit, Hinweis ohne Bot-Namen, `StatTile` statt lokaler Kacheln,
  doppelte E2E-Helfer zusammengelegt.
- **Offen gelassen (M10 Härtung):** Zustellungen, deren Einreihung nach dem Einfügen scheitert, bleiben `queued`;
  der Assistent trägt Passwort und TMDB-Schlüssel noch als Mutationsvariablen (Vor-M8-Code); Update-Badge-Daten gehen
  auch an Kinderprofile des Hauptkontos (die Oberfläche zeigt es nur dem Hauptprofil); Feldnamen bei `env_wins`
  weichen von den Formularfeldern ab (Werte aus `.env` sind in der Oberfläche ohnehin gesperrt); ein neues Profil sieht
  die ganze Ankündigungs-Historie (M-8); Push-Abos mit 401/403 werden nicht gelöscht und `CreatePushSubscription` hält
  `vapidMu` nicht (M-12); ein endgültig fehlgeschlagener Job steht in «fehlgeschlagen», aber nicht im Fehlerring (M-14);
  Nits N-5 bis N-10 (Sitzungsnotizen).

### Abweichungen und Entscheide während M8

- **Hauptprofil-Regel für Kanäle** (Review I-2), **400 Tage Wiedergabesitzungen** (I-3), **Neustart per `exec`**,
  **Feed-URLs bleiben in Sicherungen ohne Schlüssel**: im Entscheidungsprotokoll vom 21. Oktober 2026.
- **Multipart-Upload im strict server:** oapi-codegen reicht `multipart/form-data` als `*multipart.Reader`; die
  Passphrase muss vor der Datei kommen; der Download ist ein Binär-Handler (Regel 6).
- **`AdminSettingsUpdate.clear`** statt JSON-null (weder Go- noch TypeScript-Codegen unterscheiden null von fehlend).
- **Statistik-Abfragen** aggregieren vorab und laufen parallel (Faktor 3–4 gegenüber dem Entwurf).
- **Fortschrittsereignisse tragen das Gerät** (`progress_events.device_id` aus dem Geräte-Cookie); vorher blieb die
  Spalte für Browser leer, die Geräteverteilung (A-8) wäre leer geblieben.
- **Fehlgeschlagener Trakt-Abruf** wird als Error protokolliert (Fehlerring, A-3).
- **Update-Prüfung ohne Manifest-URL** liest das echte GitHub-Manifest 60 s nach dem Start; im E2E-Lauf zeigt der
  Mock standardmässig 404, damit kein Badge auf jeder Seite erscheint.
- **E-Mail-Versand mit `tls: none`** sendet Zugangsdaten nur an localhost (Go-SMTP-Client); echte Server brauchen
  STARTTLS oder TLS.
- **Commit-Trailer:** die Lanes nennen das Modell, das die Arbeit gemacht hat (Opus 5.5 bzw. Fable 5.1).

### Nicht in M8 (Plan) und offene Punkte für M9

1. **Echte Zugangsdaten:** SMTP in `deploy/.env` und Web-Push am Android-Handy prüft Marco nach dem Merge und dem
   neuen Image (Checkliste `m8-marco-tests.md`); QSV-Erkennung im Assistenten auf CT 190 mit dem neuen Image.
2. **Beschaffungsstatus** (`acquisition`) hat Modell, Kanalwahl und Hook, aber keinen Erzeuger – M9 (Radarr/Sonarr).
3. **Release-Manifest:** die Update-Prüfung liest `https://github.com/SlouchCat/slouchcat/releases/latest/download/manifest.json`;
   das Veröffentlichen gehört zu M10.
4. **Verdichtung alter `progress_events`** (Plan 3.2, 90 Tage) ist nicht gebaut; die Statistik liest Rohdaten.
5. **Android:** Gerätecheckliste `docs/perf/M8.md` 6 und die offenen Punkte aus M5 bis M7; O-6-Langzeittest läuft
   (Tag 30 am 19. November).
6. Offen gelassene Review-Punkte (oben) für M10.


## Meilenstein M7 – Entdecken und Kuratierung

Vorab geklärt mit Marco (21. Oktober 2026, Entscheidungsprotokoll Frage 9, Teil AN-3): die Reihen «Trending bei
Trakt» und «Beliebt bei Trakt» zeigen die ersten 20 Treffer aus der eigenen Bibliothek und werden unter 5 Treffern
ausgeblendet; Trakt wird nur über einen Server-Schlüssel angebunden (`SLOUCHCAT_TRAKT_CLIENT_ID` in `deploy/.env`,
Anleitung in `deploy/README.md` und `.env.example`), kein Login je Profil, kein Scrobbling. Die Architekturentscheide
stehen in ADR 0008 (`docs/adr/0008-entdecken-suche-trakt.md`): Suchindex mit FTS5 über eine Inhaltstabelle, kleiner
Query-Builder in `visibility`, acht Startseiten-Reihen mit gespeicherter Reihenfolge, Karussell nur aus echten Picks
(der Platzhalter aus M1a entfällt), Trakt-Listen als eigene Tabelle mit Verschneidung je Profil über `visible_items`,
Benachrichtigungen über Hooks von Scanner und Feed-Abruf in einem neuen Paket `notify` (vorab in der Allowlist von
Regel 2). Der Entwurf liegt in den Sitzungsnotizen (`m7-design.md`). Die Umsetzung lief wie in M5/M6 auf einer
gemeinsamen Basis (Migration 00015, Vertrag 0.8.0 mit 109 Operationen, Modelltypen, 501-Stubs) mit fünf
Executor-Agenten (Opus) in Scratchpad-Kopien (A Suche und Listen, B1 Admin und Sammlungen, B2 Trakt,
Benachrichtigungen, Offline, C1 Web Zuschauer, C2 Web Admin), danach zwei Review-Agenten und zwei Korrektur-Agenten.
Commits mit Bereich im Titel:

| Schritt | Commit | Inhalt |
|---|---|---|
| Vorab | `docs(decisions)` `671b5b8` | Antwort auf Frage 9, Teil AN-3 |
| Basis | `feat(api,store,model)` `1a48db5`, `docs(adr)` `fbef9ab`, `fix(store)` `b39c09b`, `feat(api)` `0850a75`, `test(testdata)` `a7e9dc0`, `test(library,transcode,web)` `43a016a` | Vertrag 0.8.0 (28 neue Operationen), Migration 00015 (`watchlist`, `favorites`, `collections`, `collection_items`, `home_rows`, `item_genres`, Suchindex, `trakt_lists`, `notifications`), Konfiguration und Redaction für die Trakt Client ID, Schema-Test I-7, Suche ohne Text (Genre-Link), `{tmdb-NNNN}`-IDs an sieben Testmedien für den Trakt-Mock |
| 1 Cat's Empfehlungen | `feat(admin,metadata,api)` `15f1753`, `feat(web)` `d63f580`, `c0e4337` | Picks für alle oder einzelne Profile, Reihenfolge, Notiz, Zeitraum; Karussell mit Notiz; Altersstufe über `visible_items` |
| 2 Reihen | `feat(visibility,discover,api)` `e8d06c0`, `15f1753`, `c0e4337`, `d63f580` | Weiterschauen, Neu hinzugefügt, Merkliste je Profil (E-5), Sammlungen aus TMDB-Reihen (`belongs_to_collection`) und manuell (E-6), Genres, Trakt; Reihenfolge und Ausblenden durch den Hauptbenutzer, höchstens acht, keine leeren (E-9) |
| 3 Suche | `e8d06c0`, `c0e4337`, `perf(store)` `360036f` | FTS5 über Titel (de/en, Original), Autor, Sprecher, Reihe, Personen, Sammlungen ab dem zweiten Zeichen; Filter Genre, Jahr, Altersfreigabe, Medientyp, Verfügbarkeit lokal/Download (E-4) |
| 4 Trakt | `feat(trakt,metadata,notify,library,offline,admin,api)` `060ee3f` | Reines Paket `trakt`, täglicher Job über `ratelimit`, Ausfall leert die Listen (I-7), Admin-Status und «Jetzt aktualisieren» |
| 5 Favoriten, Benachrichtigungen | `060ee3f`, `c0e4337` | Favoriten für Serien und Podcasts, In-App-Eingang mit Zähler, Hooks nach Scan-Batch und Feed-Abruf, Prüfung je Profil über `visibility` (5.5), `Dispatcher` für M8; die Auto-Download-Regel folgt den Favoriten statt dem Hörverlauf (O-3) |
| Storybook | `test(web)` `31bbfdf` | Baselines der neuen Komponenten |
| DB-Stand | `test(store,testdata)` `d7237c5`, `0acab29` | `testdata/db/M7.sql`, `TestFrozenM7StateCoversM7Data` |
| Review-Korrekturen | `perf(store)` `360036f`, `fix(web)` `fb67b70`, `fix(visibility,discover,notify,metadata,trakt,admin,offline,api)` `421bf84` | siehe unten |
| Messwerte | `docs(perf)` `0acab29`, `17d28d9`, `9d92b5e` | `docs/perf/M7.md` (inkl. erster Lauf gegen das echte Trakt); O-6-Langzeittest Tag 1 |
| Verifier-Korrekturen | `test(visibility)` `bd5de19`, `docs(status,perf)` | Regressionstest «höchstens 20» für die Trakt-Reihen; Messung mit allen acht Reihen; Doku |

### Definition of Done (Plan M7 plus 6.2) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| E-1 Reihenfolge Karussell, Weiterschauen, Neu hinzugefügt, Merkliste, Sammlungen, Genres | PASS: Standardreihenfolge in `home_rows` (Migration), `/home` liefert sie, leere entfallen | `go test ./internal/discover -run TestHome`, `go test ./internal/api -run TestContractChildProfiles` (Subtest home), `web/e2e/home.spec.ts` |
| E-2 Pick für alle und für ein Profil; Reihenfolge per Maus und Tastatur; Notiz sichtbar; vor Start und nach Ende unsichtbar | PASS: Admin-Seite `/admin/picks` per Tastatur und Maus, Notiz im Karussell, Zeitraum mit Fake-Uhr im Server und mit echten Zeitfenstern im E2E | `go test ./internal/visibility -run TestPicks`, `go test ./internal/api -run TestContractAdminPicks`, `web/e2e/admin-picks.spec.ts`, `home.spec.ts` |
| E-3 FSK-18-Pick «für alle» fehlt bei P0–P16, erscheint bei P18 | PASS, Negativtest: P12 sieht den FSK-18-Pick nicht (Vertragstest und E2E mit Kinderprofil) | `go test ./internal/api -run TestContractChildProfiles`, `web/e2e/admin-picks.spec.ts` (E-2/E-3) |
| E-4 2 Zeichen finden Titel, Person, Sammlung; jeder Filter einzeln und kombiniert (ausser «in Beschaffung», M9); < 200 ms p95 bei 10 000 Titeln | PASS: Titel, Person (nur über sichtbare Rolle), Sammlung (nur mit sichtbarem Mitglied); Filter einzeln und kombiniert; 50'570 Titel: p95 102.0 ms nach dem Scan mit allen acht Reihen (Median aus 3, Verifier), 55.6 ms im `scanbench` | `go test ./internal/visibility -run TestSearch`, `go test ./internal/api -run TestContractChildProfiles` (search), `web/e2e/search.spec.ts`, `docs/perf/M7.md` 1 |
| E-5 Merkliste je Profil getrennt; hinzufügen/entfernen; Reihe erscheint und verschwindet | PASS | `go test ./internal/discover -run TestWatchlist`, `web/e2e/watchlist.spec.ts` |
| E-6 TMDB-Reihe (3 Filme) → Sammlung; manuelle Sammlung; Mitglieder und Zähler nur sichtbar | PASS | `go test ./internal/metadata -run TestMatchCollection`, `go test ./internal/api -run 'TestContractAdminCollections|TestContractChildProfiles'`, `web/e2e/admin-collections.spec.ts`, `collection.spec.ts` |
| E-7 Trakt-Mock: nur Bibliothekstitel, max. 20, täglicher Lauf (Zeitplan-Test), AN-3 | PASS gegen den Mock (nur Bibliothekstitel, höchstens 20 in Listenreihenfolge, Zeitplan mit Fake-Uhr); echter Lauf mit Marcos Client ID: 200 Einträge je Liste, kein Fehler, ID nicht im Log; Verschneidung mit der echten Bibliothek folgt auf CT 190 | `go test ./internal/metadata -run 'TestTraktSchedule|TestTraktRefresh'`, `go test ./internal/visibility -run 'TestTraktRowIntersection|TestTraktRowAtMostTwenty'`, `web/e2e/home.spec.ts`, `docs/perf/M7.md` 4 |
| E-8 Favorit, neue Folge → In-App-Meldung < 2 min (Kanäle M8) | PASS: 30.6 s (Chromium) / 31.1 s (Firefox) von der Dateikopie bis zum Zähler; P6 erhält für eine Folge über seiner Stufe weder Eintrag noch Dispatcher-Aufruf | `web/e2e/notifications.spec.ts`, `go test ./internal/notify`, `go test ./internal/api -run TestContractNotifications` |
| E-9 ≤ 8 Reihen, keine leere; Umordnung durch den Hauptbenutzer wirkt | PASS | `go test ./internal/api -run TestContractHomeRows`, `web/e2e/admin-home-rows.spec.ts`, `home.spec.ts` |
| I-7 Trakt 503 → nur die zwei Reihen fehlen; kein Trakt-Konto je Profil | PASS: Schema-Test, 503 leert die Liste, ohne Schlüssel oder nach 48 h ohne Erfolg ebenfalls keine Reihen | `go test ./internal/api -run TestContractNoTraktPerProfile`, `go test ./internal/metadata -run 'TestTraktRefresh|TestTraktRowsGoWithoutClientID'`, `web/e2e/home.spec.ts` |
| Startseiten-API < 300 ms p95 mit allen Reihen und Referenzgrösse | PASS auf dem Entwicklungsrechner: p95 160.7 ms nach dem Scan mit allen acht belegten Reihen (Median aus 3, Messung des Verifiers; ohne Genres 85.5 ms), 229.9 ms während des Erst-Scans; Median aus 3 statt 5 Läufen; CT 190 nicht gemessen (mit dem Faktor 2.5 aus M3 läge die Startseite bei rund 400 ms) | `docs/perf/M7.md` 1a und 1b |
| Kinderprofil-Vertragstest 10.2 über alle neuen Operationen | PASS: 109 Operationen (Picks nur Admin, Merkliste und Favoriten je Profil, Suche per POST, Trakt-Reihen, Benachrichtigungen mit Dispatcher-Mock) | `go test ./internal/api -run 'TestContract'` |
| Tastaturtest je neue Seite (6.2 Pkt. 3) | PASS: Startseite, Suche, Merkliste, Sammlung, Benachrichtigungen, Detail-Knöpfe, `/admin/picks`, `/admin/collections`, `/admin/home-rows` | die genannten Specs |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M7.sql`; alle Stände M1a bis M7 migrieren bis 00015; 00015 gegen M6.sql geprüft (Rückfüllen, Trigger, Integritätsprüfung des Index) | `go test ./internal/store -run 'TestFrozen|TestMigration00015|TestDumpRoundTrip'` |
| Messwerte (6.2 Pkt. 5) | `docs/perf/M7.md`; Gerätecheckliste Desktop Chromium und Firefox bestanden, Android ausstehend (Marco, nach dem Merge), Apple ungeprüft (Entscheid 2); LCP (P-10) in M7 nicht neu gemessen | `docs/perf/M7.md` 5 |
| O-6-Langzeittest aus M5 | **läuft, Abschluss ausstehend**: Desktop Chromium und Firefox Tag 0 und Tag 1 bestanden; Tag 30 ist der 19. November 2026, Android startet mit dem Image nach dem Merge | `docs/perf/M5-o6-langzeittest.md` |
| `task ci` grün (6.2 Pkt. 2) | exit 0 (21. Oktober 2026, Stand `0acab29`): gen, gen:check, lint 0 issues (golangci-lint, itemsaccess, vet, eslint, stylelint, tsc, prettier), Go `-race` alle 26 Pakete inkl. `trakt` und `notify`, Vitest 464 (+ 4 + 2), Build, Playwright 330 von 330 in Chromium und Firefox, Storybook 133 Screenshots; danach nur Doku, der Test «höchstens 20» und der Merge von `main` (Go-Testlimit 30 min) | `sg docker -c 'task ci'`, Protokoll `task-ci-1.log` im Scratchpad der Session |
| Code-Review | Server 22 Befunde (1 kritisch, 1 wichtig, 9 klein, 11 Nits; 4 Go-spezifisch, 2 teilweise), Web 31 Befunde (0 kritisch, 2 wichtig, 11 klein, 18 Nits); kritisch, wichtig und klein behoben ausser Web K4, Nits teils (siehe unten) | Sitzungsnotizen `m7-review-server.md`, `m7-review-web.md`, `m7-fix-server-notes.md`, `m7-fix-web-notes.md` |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen»: O-6 Tag 30 erst am 19. November; Gerätecheckliste Android ausstehend, Apple ungeprüft; E-7 «höchstens 20» ohne eigenen Test (danach ergänzt, `bd5de19`); Abschnitt 1a der Messwerte zählte nur sieben Reihen (danach mit allen acht nachgemessen). Eigene Läufe: Vertragstest `-race` ok (109 Operationen), Pakete visibility, discover, notify, trakt, metadata ok, Store-Tests mit M1a bis M7.sql ok, Lint grün, neun M7-Specs in Chromium und Firefox 46 von 46, Messung mit acht Reihen; Korrekturen am Bericht übernommen | Sitzungsnotizen `m7-verifier.md` |

### Befunde der Review-Passage und ihre Behebung

- **Server (kritisch):** die Trigger des Suchindex suchten FTS5-Zeilen über nicht indizierte Spalten, ein Vollscan
  je Ereignis (34 ms bei 300'000 Zeilen; ein Podcast-Abruf mit 2'000 bekannten Episoden hätte den einzigen Schreiber
  minutenlang belegt). Die Texte liegen jetzt in `search_docs` mit Indizes, `search_index` ist eine FTS5-Tabelle mit
  externem Inhalt (per Rowid), UPDATE-Trigger feuern nur bei geändertem Text: 0.19 ms je geändertem Titel bei
  350'000 Indexzeilen (`docs/perf/M7.md` 2). Migration 00015 ist auf keiner laufenden Instanz angewendet und wurde
  darum direkt angepasst.
- **Server (wichtig):** ohne Client ID blieben die Trakt-Reihen mit alten Listen stehen; jetzt werden die Listen
  geleert und Reihen älter als 48 h nie gezeigt.
- **Server (klein):** Picks werden vor dem Limit entdoppelt (Hero-Vorladen stimmt mit dem Karussell überein); keine
  Zählung für die Merkliste-Reihe; eine Abfrage je Scan-Batch für den Folgen-Hook; Podcasts melden nur Episoden, die
  neuer sind als die neueste bekannte, höchstens 50 je Abruf, gelesene Einträge verfallen nach 90 Tagen;
  `null` im PATCH der Picks unabhängig von der Schreibweise; Suchwörter mit einem Zeichen neben längeren werden
  ignoriert, höchstens acht; `maxRating`, Jahre und Genre werden geprüft (422); der Trakt-Client folgt keiner
  Weiterleitung auf einen anderen Host; der Vertragstest duldet kein 501 mehr. Nits: der Filter «lokal» prüft die
  Kinder über `visible_items` (bei einem Podcast war das beobachtbar), «alle gelesen» nur sichtbare Einträge,
  Prüfungen in der Transaktion, eine Abfrage für Sammlungsmitglieder, Zählung und Seite in einer Lesetransaktion,
  30-s-Grenze um die Hooks.
- **Web (wichtig):** das Suchfeld folgt der URL (der Link «Suche» leert eine Suche); der Mitglieder-Dialog lässt erst
  nach dem Laden der gespeicherten Mitglieder wählen (kein Verlust von Mitgliedern).
- **Web (klein):** 404 gewinnt über gecachte Sammlungen; Umordnen der Picks serialisiert mit Rücksetzen bei Fehler;
  Sammlungsänderungen invalidieren Detail, Sammlung und Suche; `?add=` liest den Titel über die Admin-Abfrage;
  E2E-Resets schlagen laut fehl, der Trakt-Mock wird je Test zurückgesetzt, Reste des E-8-Tests werden vor dem Scan
  entfernt, Umordnen per Maus und kombinierte Suchfilter getestet. Nits: Escape im leeren Suchfeld geht zurück,
  gesprochene Beschriftungen aus den Locales, Randfälle der Such-URL, `aria-busy`, dauerhafte Statusbereiche, das
  Kürzel `/` beachtet `defaultPrevented`, der Eingang folgt dem Zähler.
- **Nicht umgesetzt (Server):** n9 (`setHomeRows` nimmt die Reihenfolge aus dem Array, das Feld `position` im Body
  wird nicht gelesen), n10 (Durchsuchen ohne Medientyp liefert auch Folgen und Episoden), n11 (Sprache der Kommentare
  in `deploy/.env.example`); Begründungen in `m7-fix-server-notes.md`.
- **Nicht umgesetzt (Web):** K4 (links/rechts in den Jahresfeldern, «Jahr bis» per Pfeil schwer erreichbar), N2
  (Zähler-Abfrage beim Start), N4, N11 (Sekunden und Sommerzeit bei `datetime-local`), N12 (404-Zustand ausserhalb
  der räumlichen Navigation, wie auf der Detailseite), N14, N18; Begründungen in `m7-fix-web-notes.md`.

### Abweichungen und Entscheide während M7

- **Kein Platzhalter-Karussell mehr:** ohne Picks gibt es kein Karussell (E-9 «keine leeren Reihen»); das E2E-Setup
  legt einen Basis-Pick an. Nach dem Update sieht Marco das Karussell erst, wenn er unter «Cat's Empfehlungen» Titel
  wählt.
- **Suche ohne Text:** der Vertrag erlaubt eine Suche nur mit Filtern (Genre-Link der Startseite); ohne Text und
  ohne Filter antwortet der Server 422.
- **Suche:** Titeltreffer kommen nur aus Titelzeilen; Personen und Sammlungen füllen eigene Abschnitte. Eine Suche
  «Toy Story 3» sucht nach «Toy Story» (einstellige Wörter neben längeren entfallen). `maxRating` lässt unbekannte
  Freigaben nie durch.
- **Podcast-Regel:** «neue Episoden» gelten ab der neuesten Episode, die beim Favorisieren schon bekannt war (oder
  der neuesten gestarteten); mit dem Zeitpunkt des Favorisierens allein hätte ein Feed mit alten Datumsangaben nie
  etwas geladen.
- **Detailseite:** «Merken», «Favorit» und «Zu Cat's Empfehlungen» stehen in einer zweiten Knopfreihe; die
  Kopfnavigation scrollt bei vielen Einträgen seitwärts; der Eingang ist ein Text mit Zähler (kein Glockensymbol).
- **E-8 im E2E:** die Spec kopiert eine Folge der «Intro Show» unter neuer Nummer in `testdata/media/TV` und
  entfernt sie danach; Reste räumt das globale Setup vor dem Scan weg.
- **Testmedien:** sieben Titel tragen `{tmdb-NNNN}` im Ordnernamen (Trakt-Mock); bestehende `testdata/media` müssen
  mit `task testdata:media` neu erzeugt werden (die alten Ordner bleiben sonst liegen).
- **`pickUpdateNulls`:** der generierte Code unterscheidet bei `updateTopPick` «null» nicht von «fehlt»; eine kleine
  Middleware liest den Body vorher (Go-spezifisch nach 2.5: Grenze von oapi-codegen).

### Nicht in M7 (Plan) und offene Punkte für M8

1. Trakt: die Client ID steht in `deploy/.env` (Marco, 21. Oktober 2026), der echte Abruf ist geprüft
   (`docs/perf/M7.md` 4); die Verschneidung mit der echten Bibliothek auf CT 190 nach dem neuen Image ansehen.
   Einstellungsseite für Schlüssel: M8.
2. Benachrichtigungskanäle (E-Mail, Web-Push, Telegram) hängen am `notify.Dispatcher` (M8); dort den Versand über
   die Job-Queue entkoppeln.
3. O-6-Langzeittest: Tag 3 (23.10.), Tag 8 (28.10.), Tag 30 (19.11.) auf dem Desktop; Android nach dem Merge.
4. Android: Gerätecheckliste `docs/perf/M7.md` 5 und die offenen Punkte aus M5 und M6.
5. **Startseite und Suche auf CT 190 messen** (`measure-m7-home-genres.sh`): mit dem Faktor 2.5 aus M3 lägen sie
   über den Grenzen (rund 400 ms bzw. 260 ms p95); falls ja, Zwischenspeicher für «Genres» und «Sammlungen» und eine
   begrenzte Trefferzählung. Filter «in Beschaffung» folgt in M9.
6. Web-Nits aus dem Review (siehe oben); Weiter offen aus M6 und M5 unverändert.

## Meilenstein M6 – Hörbücher und Podcasts

Vorab geklärt mit Marco (20. Oktober 2026, Entscheidungsprotokoll Frage 9, Teil AN-2): jede Hörbuch- und
Podcast-Bibliothek trägt eine **Standard-Altersstufe**, einzelne Titel dürfen nur strenger gestellt werden, und
Podcasts oder Episoden mit `itunes:explicit=true` gelten als **FSK 18**. Der Trakt-Teil (AN-3) wurde vor M7 beantwortet.
Echte Testdaten liegen schreibgeschützt auf dem NAS (`/mnt/nerdnas/medien/Audiobookshelf/Hörbücher`, 88 M4B in
Audiobookshelf-Struktur, und `…/Podcast` mit den Feed-URLs von Marcos Abos); die CI nutzt nur einen lokalen Mock-Feed.
Die Architekturentscheide stehen in ADR 0007 (`docs/adr/0007-audio-modell.md`): ein Hörbuch ist **ein** Titel mit
geordneten Teilen auf einer buchweiten Zeitachse, Kapitel liegen in einer eigenen Tabelle, mehrteilige Bücher streamen
immer als HLS über eine Concat-Quelle, Podcasts sind Feed-Container, deren Episoden über den Server gestreamt und
geladen werden, und es gibt kein Paket `audio` mit Datenbankzugriff (Regel 2 unverändert; reines Paket `feed`).
Der Entwurf liegt in den Sitzungsnotizen (`m6-design.md`, Abschnitt 6 mit den Abweichungen). Die Umsetzung lief wie
in M5 mit drei Executor-Agenten (Opus) in Scratchpad-Kopien (A Hörbücher, B Podcasts und Offline-Audio, C Web) auf
einer gemeinsamen Basis (Migration 00014, Vertrag 0.7.0 mit 81 Operationen, Modelltypen, Stubs); danach zwei
Review-Agenten und zwei Korrektur-Agenten. Commits mit Bereich im Titel:

| Schritt | Commit | Inhalt |
|---|---|---|
| Vorab | `docs(decisions)` `81b6f5e`, `test(testdata)` `644e563` | Antwort auf Frage 9 (AN-2); Testmedien: M4B mit 12 Kapitelmarken und eingebettetem Cover, MP3-Ordner mit 10 Teilen und `cover.jpg`, drei Podcast-Episoden und `testdata/podcasts/feed.xml.tmpl` (Episode 3 explicit) |
| Basis | `feat(api,store,model)` `6c485e3`, `docs(adr)` `181abe9` | Vertrag 0.7.0 (Bibliotheksarten `audiobooks`/`podcasts` mit `defaultRating`, `updateLibrary`, `listPodcasts`, `createPodcast`, `deletePodcast`, `refreshPodcast`, `listPodcastEpisodes`, Audio-Felder an `ItemRef`/`ItemDetail`, `PlaybackInfo.mediaKind`/`parts`, `playbackRate`, Kapitel im Manifest); Migration 00014 (`libraries` und `downloads` neu aufgebaut, Audio-Spalten an `items`, `media_files.part_index`, `chapters`, `progress.playback_rate`, Trigger für Episoden); ADR 0007 |
| 1 Hörbücher | `feat(library,transcode,playback,metadata)` `2f277e6` | Scanner für Audio (M4B je Buch, Ordner als Buch mit Teilen), Metadaten aus `metadata.json` > Tags > Ordnernamen, buchweite Kapitel nach der Probe aller Teile, Cover aus Ordnerbild oder eingebettetem Bild, Tags und Cover-Extraktion in `transcode`, Concat-Quellen, Audio-Pipeline mit echtem ffmpeg geprüft, Decide für Audio (Direct Play für AAC/MP3-Einzeldateien, sonst Audio-HLS; mehrteilig immer HLS), Audio-MIME-Typen, Geschwindigkeit je Profil und Titel |
| 2 Podcasts, Offline-Audio | `feat(metadata,offline,admin,playback,api)` `c585e0b` | Paket `feed` (RSS 2.0 mit iTunes-Namensraum), Abonnieren mit synchronem Erstabruf, Job `podcast.fetch` mit bedingtem GET, Poller, Episoden nach GUID, explicit → 18, Enclosure-Proxy mit `Range`, `updateLibrary` (Standardstufe zieht Titel mit Quelle `library` nach), Stufe unter dem Standard → 422, Admin-Abos, `listPodcastEpisodes`, Audio-Downloads (Buch über alle Teile, Episode aus der Enclosure in den Cache), Manifest mit Kapiteln und `audio/mp4`, Auto-Download neuer Episoden gehörter Podcasts, Vertragsfixture mit Hörbüchern, Mehrteiler, Podcast mit explicit- und Enclosure-only-Episode |
| 3 Web | `feat(web)` `866a452`, `test(web)` `7f80e72` | Audio-Player im Design-System (`ui/AudioPlayer`, Kapitel, Geschwindigkeit 0.5–3.0, Sleep-Timer, Tastatur), Detailseiten Hörbuch/Podcast/Episode, quadratische Cover, Bibliotheksformulare mit vier Arten und Standardstufe, `/admin/podcasts`, Offline-Audio (Manifest mit Kapiteln, Rate lokal), Mock-Feed-Server und sechs neue Playwright-Specs; Screenshot-Baselines |
| DB-Stand | `test(store,testdata)` `a4f423b` | `testdata/db/M6.sql`, `TestFrozenM6StateCoversM6Data` |
| Review-Korrekturen | `fix(web)` `ad3f892`, `fix(web)` `c2d890a`, `fix(metadata,offline,playback,transcode,library,admin)` `1362e1a` | siehe unten |

### Definition of Done (Plan M6 plus 6.2) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| H-1 M4B mit 12 Kapiteln und Ordner mit 10 MP3: Kapitelzahl exakt, Start ±1 s; Autor, Sprecher, Cover | PASS: Fixture-Scan liefert 12 bzw. 10 Kapitel mit Start ±1 s, Autor/Sprecher/Reihe/Band, Cover eingebettet bzw. aus dem Ordner; Detailseite zeigt Autor, Sprecher und Cover; echte Sammlung: 88 Bücher, 10 025 Kapitel, Kapitelzahl bei allen 88 identisch mit Audiobookshelf (zweimal gemessen, zuletzt auf `0c1a0f1`; das längste Buch mit 544 Kapiteln bleibt vollständig) | `go test ./internal/library -run TestScanAudiobookFixtures`, `web/e2e/audiobook.spec.ts`, `docs/perf/M6.md` 1 |
| H-2 lokaler Feed-Server: neue Episode erscheint < 65 min | PASS: Abruf alle 55 min (Poller mit Fake-Uhr), bedingtes GET, Mock-Feed in Playwright (Episoden 1–2, Kontrollendpunkt auf 3, «Jetzt abrufen» → Episode 3 erscheint); echte Feeds SRF «Echo der Zeit» und «Bad Friends» in 0.21 bzw. 0.25 s abonniert | `go test ./internal/metadata -run 'TestQueueDuePodcasts|TestFetchPodcast'`, `web/e2e/podcast.spec.ts`, `docs/perf/M6.md` 2 |
| H-3 Fortschritt mitten im Kapitel geräteübergreifend ±2 s; Geschwindigkeit 0.5–3.0 je Titel gespeichert | PASS: Position aus dem zweiten Browserkontext ±2 s, Geschwindigkeit 2.0 nach Neuladen und im zweiten Kontext; offline je Profil gespeichert und beim Abgleich mit dem Serverwert zusammengeführt | `web/e2e/audiobook.spec.ts`, `go test ./internal/playback -run TestReportPlaybackRate`, `OfflinePlayer.test.tsx` |
| H-4 Vertragstest 10.2 mit Hörbuch- und Podcast-Operationen; Audio-Download trägt `allowed_profile_ids` | PASS: 81 Operationen (admin 33, scoped 25, account 15, public 8); explicit-Episode und explicit-Podcast (Kanal `itunes:explicit`, Episode erbt 18) für P12/P16 unsichtbar und per ID 404 (Detail, Play, Stream, Episodenliste, Download-Optionen, Downloads, Bilder), Abos nur für das Hauptprofil des Hauptkontos; `allowed_profile_ids` auf Audio-Downloads | `go test ./internal/api -run 'TestContractCoversEveryOperation|TestContractChildProfiles|TestContractAudio'` |
| O-1 (Audio) Hörbuch und Episode in 3 Stufen, Grösse ±10 % der Schätzung | PASS: Hörbuch (AAC-Kopie in allen Stufen) −4.0 %, Episode high/medium/low −0.6/+1.1/+2.6 % in Chromium und Firefox | `web/e2e/audio-download.spec.ts`, `docs/perf/M6.md` 3 |
| O-3 (Podcast) neue Episode lädt automatisch; Max.-Anzahl | PASS Desktop: Regel «Neue Episoden gehörter Podcasts», Fortschritt auf Episode 2, Feed auf 3 → Sync legt Episode 3 an, Episode 1 nicht; «nur WLAN» und Android wie in M5 ausstehend | `web/e2e/podcast-auto-download.spec.ts`, `go test ./internal/offline -run TestPodcastAutoDownloads` |
| Flugmodus-Test (DoD): geladenes Hörbuch mit Kapitelsprung, Chromium und Firefox | PASS: Download, Flugmodus, Wiedergabe, Shift+→ und Kapitelmenü (Ziel −0.5/+2 s, die Spec prüft die Position nach dem Sprung im laufenden Element), Fortschritt nach Wiederverbindung im zweiten Kontext ±10 s | `web/e2e/audio-offline.spec.ts` (beide Projekte) |
| Feed-Abruf gegen lokalen Test-Feed, kein Netz in CI | PASS: Go über `httptest`, Playwright über `web/e2e/feed-server.mjs` (Port App-Port − 2); private und Loopback-Ziele nur für Feeds, deren Host selbst lokal ist (`netguard`) | `go test ./internal/feed ./internal/netguard ./internal/metadata` |
| Tastaturtest je neue Seite (6.2 Pkt. 3) | PASS: Hörbuch-Detail, Podcast-Detail, Episoden-Detail, Audio-Player (online und offline), `/admin/podcasts` (Abo-Dialog, Pfeiltasten zwischen den Zeilen, Zurück), Bibliotheksformular mit Audio-Arten und Standardstufe (`/admin/libraries` und Assistent `/setup`) | `audiobook.spec.ts`, `podcast.spec.ts`, `admin-podcasts.spec.ts`, `admin-libraries.spec.ts`, `setup.spec.ts`, `audio-offline.spec.ts` |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M6.sql`; alle Stände M1a bis M6 migrieren bis 00014; 00014 gegen M5.sql geprüft (Bibliotheken, Downloads, Grabsteine, Trigger) | `go test ./internal/store -run 'TestFrozen|TestMigration00014'` |
| Messwerte (6.2 Pkt. 5) | `docs/perf/M6.md` (Scan der echten Sammlung, echte Feeds und Proxy, O-1-Grössen, Gerätecheckliste) | |
| `task ci` grün (6.2 Pkt. 2) | exit 0 (21. Oktober 2026, Stand `9abd7f3`): gen, gen:check, lint 0 issues (golangci-lint, itemsaccess, vet, eslint, stylelint, tsc, prettier), Go `-race` alle 24 Pakete inkl. `feed` und `netguard`, Vitest 414 (+ 4 + 2), Build, Playwright 292 von 292 in Chromium und Firefox, Storybook 115 Screenshots plus ein Abdeckungstest unverändert. Vorher zwei Läufe mit exit 1: `watch-limit` Firefox einmal rot (M4-Test, sofortige Wiederholung grün) und Startseiten-Specs, die einen Titel aus der vollen 20er-Reihe «Neu hinzugefügt» erwarteten (behoben in `9abd7f3`: das E2E-Setup scannt die Audio-Bibliotheken vor den Video-Bibliotheken) | `task ci`-Protokolle `task-ci-3.log` bis `task-ci-6.log` im Scratchpad der Session |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen» (Android/Apple ausstehend, `task ci` damals ohne exit 0, Tastaturtests Bibliotheksformular und Podcast-Tabelle, Kapitelgrenze 500 kappte ein echtes Buch, kein HLS-Test des MP3-Buchs); alle Code-Ausnahmen danach behoben (`0c1a0f1`, `9abd7f3`: Grenze 5000 mit Test für 544 Kapitel, explicit-Podcast in der Vertragsfixture, Tastaturtests `/admin/libraries`, `/setup`, `/admin/podcasts`, MP3-Buch über HLS über eine Dateigrenze), Messung der echten Sammlung und der Feeds auf `0c1a0f1` wiederholt, Doku korrigiert | Sitzungsnotizen `m6-verifier.md` |
| Code-Review | Server 27 Befunde (0 kritisch, 4 wichtig, 12 klein, 11 Nits, 1 offene Frage), Web 25 Befunde (0 kritisch, 2 wichtig, 11 klein, 12 Nits); alle wichtigen und kleinen behoben, Nits teils (siehe unten) | Sitzungsnotizen `m6-review-server.md`, `m6-review-web.md`, `m6-fix-server-notes.md`, `m6-fix-web-notes.md` |

### Befunde der Review-Passage und ihre Behebung

- **Server (wichtig):** der Enclosure-Proxy reichte den Content-Type des Podcast-Hosts ungeprüft durch (jetzt nur
  `audio/*` und `video/mp4`, sonst Feed-Typ oder `application/octet-stream`, mit `nosniff` und CSP-Sandbox);
  Tokens privater Feeds konnten über `*url.Error` ins Log (Redaction um `auth`, `token`, `access_token`, `secret`,
  `sig`, `signature` erweitert, Abruffehler nennen nur Schema und Host); ein Teil mit Probe-Fehler machte ein
  mehrteiliges Buch unspielbar und liess den Download ewig warten (Teile mit `probe_error` fehlen jetzt in Kapiteln,
  Wiedergabe und Download gleichermassen, ein Download mit kaputter Datei scheitert sofort); der Concat-Weg setzte
  gleiche Stream-Struktur aller Teile voraus (heterogene Teile laufen über den `concat`-Filter mit fester Abtastrate,
  nie als Kopie; Test mit zwei Teilen verschiedener Stream-Reihenfolge und Abtastrate). Offene Frage SSRF entschieden:
  Feed-, Enclosure- und Cover-Abrufe verweigern private, Loopback- und Link-Local-Ziele nach der Namensauflösung,
  ausser der Host der Feed-URL ist selbst eine solche Adresse (neues reines Paket `netguard`, ADR 0007); diese Clients
  ignorieren `HTTP(S)_PROXY`.
- **Server (klein):** gesperrte Felder überleben Feed-Updates; partieller Unique-Index gegen doppelte Abos; der
  Download liest die aktuelle Enclosure-URL; Fortsetzen prüft `Content-Range` und `If-Range` (`Accept-Encoding:
  identity`); Audio-Schätzung aus Buchdauer und Enclosure-Codec; eine Episode, die explicit wird, beendet offene
  Streams; der `media_files_update`-Trigger behält die Enclosure-Verfügbarkeit; eine höhere Standardstufe hebt
  manuelle Stufen darunter auf den Standard; Feed-Batches 150 Episoden, höchstens 500 Kapitel je Buch; Feed-Felder
  begrenzt (500 bzw. 2048 Zeichen); Episoden-Cover nur, wenn es vom Kanalbild abweicht (neue Episoden, höchstens 50 je
  Abruf); Tests für Redirects, Body-Limit, fremde Header, explicit-Episode auf laufendem Token, Enclosure-Download.
  Nits: US-Zeitzonen im `pubDate`, Feed-Intervall 55 min, Audio-Encode unter dem Bandbreitenlimit, Kapitel auf die
  Teildauer geklemmt (gemeinsame Funktion für Concat und Scanner), englische Textzeile für Hörbücher, Ordnerbild
  mit `?` im Namen; fünf statt vier Redirects. Nicht umgesetzt: lokalisierte Kapiteltitel («Kapitel n» steht in der
  Datenbank), gemeinsame `abortOnRevoke`-Hilfe.
- **Web (wichtig):** `[` und `]` funktionieren mit AltGr (Windows/Linux) und Option (macOS); die
  Offline-Geschwindigkeit liegt je Profil und wird beim Abgleich mit dem Serverwert zusammengeführt (der neuere
  gewinnt).
- **Web (klein):** Sleep-Timer «Ende des Kapitels» stoppt genau am Kapitelende (eigener Timer, folgt Sprüngen);
  eine spät eintreffende Server-Rate wird still übernommen; `?at=` verschwindet nach dem Start aus der URL und umgeht
  die 5-s-Schlussregel; Audio-Fähigkeiten nach dem MIME-Typ des Containers; «Mehr laden» bleibt fokussierbar und
  bietet «Erneut versuchen»; Feed-Fehlercodes in der Admin-Tabelle übersetzt; Seekbar mit Slider-Tasten; neue
  Tastaturtests für Hörbuch-Detail und Episoden-Detail; Escape wartet auf das Menü; Hörbuch-IDs aus dem Setup
  statt aus der Startseitenreihe; Feed-Port aus dem App-Port. Nits: unbenutzte Schlüssel entfernt, Band als Zahl der
  Sprache, «AAC kopiert» im Download-Dialog, «Ende des Titels» ohne Kapitel.
- **Beim Zusammenführen gefunden:** ein Menü, das innerhalb seiner 240-ms-Einblendung geschlossen wird (schnelle
  Tastaturwahl), blieb in Chromium montiert und fokussierbar, weil react-aria auf die abgebrochene Animation des
  Popover-Elements wartet; die Einblendung liegt jetzt auf dem inneren Menü (`ad3f892`). Firefox setzt die
  Tab-Reihenfolge nach `blur()` beim zuletzt fokussierten Element fort (Tab-Läufe starten von einer frischen Seite);
  liegengebliebene Test-Podcasts verdrängten ältere Titel aus der 20er-Reihe «Neu hinzugefügt» (das `owner`-Fixture
  räumt sie vor und nach jeder Spec weg); der Abo-Dialog blockierte in Firefox nach einem Serverfehler (Fehler wird
  bei Änderung der Adresse zurückgesetzt).

### Abweichungen und Entscheide während M6

- **Qualitätsstufe `auto` statt `audio`:** `Quality` und der CHECK von `playback_sessions.quality` kennen kein
  `audio`; der Audio-Player zeigt kein Qualitätsmenü.
- **AAC-Kopie in jeder Stufe:** eine AAC-Quelle innerhalb 1.1× der Stufe wird kopiert (`copy`, Dialog «AAC kopiert»);
  das 64-kbit/s-Fixture-M4B ergibt deshalb drei gleiche Dateien, die MP3-Episode wird in jeder Stufe encodiert; die
  Vertragsfixture trägt 256 kbit/s, damit alle Stufen encodieren.
- **Gruppierung:** jedes `.m4b` ist ein Buch, auch im gemischten Ordner; lose Dateien in der Wurzel je ein Buch;
  `metadata.json` und Ordnerbild zählen nur, wenn der Ordner dem Buch gehört; mehrteiliges Buch ohne Transcoder → 404.
- **«Gehörter Podcast» für O-3:** Fortschritt des Profils in den letzten 60 Tagen (Favoriten kommen in M7).
- **Episodentexte in `de` und `en`;** Aufheben einer manuellen Stufe stellt den Bibliotheksstandard her.
- **Web:** Standardstufe und Podcast-Bibliothek als RadioGroup (kein Select im Design-System); `/play/$itemId?at=`
  für Kapitel-Links; Feed-URL in der Admin-Tabelle; Mock-Feed-Server mit Präfix `/f/<key>/` je Spec; die
  Downloads-Tabelle zeigt keine Cover.
- **`testdata/db/M6.sql`** wurde nach der Review-Korrektur der Migration 00014 (Unique-Index, Trigger) von Hand
  nachgeführt; `TestFrozenStatesMigrate` und `TestFrozenM6StateCoversM6Data` sind grün.

### Nicht in M6 (Plan) und offene Punkte für M7

1. Android (Marco, nach dem Merge und dem neuen Image): Gerätecheckliste in `docs/perf/M6.md` 5 (Hörbuch direkt
   und über HLS, Episode über den Proxy, Flugmodus mit Kapitelsprung, Auto-Download mit «nur WLAN»); dazu die
   offenen Android-Punkte aus M5.
2. Trakt-Teil von Frage 9 (AN-3) vor M7; Favoriten (E-5) lösen dann den Fortschritts-Ersatz für «abonnierte
   Podcasts» (O-3) ab; Suche über Autor und Sprecher (FTS, E-4).
3. Kapiteltitel «Kapitel n»/«Teil n» liegen deutsch in der Datenbank (Lokalisierung bräuchte ein Vertragsfeld);
   OPF-Dateien und Atom-Feeds werden nicht gelesen; Podcast-Suche (Apple, Podcast Index) und lokale Podcast-Ordner
   (Audiobookshelf-Downloads) sind Ideen für später.
4. Dialog-Overlay: dieselbe react-aria-Konstellation wie beim Menü (Einblendung auf dem Overlay-Element) ist latent
   vorhanden; bisher ohne Befund in den Tests.
5. Guarded HTTP-Clients ignorieren `HTTP(S)_PROXY`; ein Proxy für Feeds wäre eine spätere Einstellung (M8).
6. Weiter offen aus M5: Grabsteine ohne Konto-ID, `checkOfflineRevocations` ohne Ratenbegrenzung, CBR auf
   Hardware-Encodern nur über Argumente geprüft, Tab-Neuladen nach Datenbank-Upgrade, 5-min-Wartezeit nach
   «Speicher voll»; aus M4: `AdminDevice.current`, Abspann-Erkennung, Einbrennen eingebetteter Textspuren, libass-
   Schriften, Audio-Bitrate in der Probe, CSP.


## Meilenstein M5 – Offline-Wiedergabe

Vorab geklärt mit Marco (20. Oktober 2026, Entscheidungsprotokoll Frage 8): AN-4, AN-10, AN-13, AN-18 und AN-19 wie
vorgeschlagen; AN-20 geändert: Budget für vorbereitete Downloads **100 GB** (Standard, `SLOUCHCAT_DOWNLOAD_BUDGET_GB`;
die Einstellungsseite folgt in M8), Löschung 24 h nach dem Abruf. Marcos Android-Lauf des Spikes (19./20. Oktober) ist
in `docs/perf/M1-ios-offline.md` eingetragen; damit ist der Spike-Entscheid endgültig: **fMP4 bleibt, die Wiedergabe
läuft über Media Source Extensions mit einem Fragmentindex** (hls.js mit eigenem OPFS-Loader), weil Chrome auf Android
per Blob-URL und Service-Worker-Range rund 1.7 s, einmal 12.6 s je Sprung brauchte; Schreibweg
`createSyncAccessHandle` im Worker, `createWritable` als Fallback. Gesamturteil «Go unter Vorbehalt Tag 8» (27. Oktober);
Spike-Seite und `SLOUCHCAT_SPIKE_DIR` bleiben bis dahin. Der Entwurf liegt in den Sitzungsnotizen (`m5-design.md`).
Die Umsetzung lief mit vier Executor-Agenten (Opus) in Scratchpad-Kopien (Transcode, Offline-Server, PIN/Ereignisse,
Web) auf einer gemeinsamen Basis (Migration 00013, Vertrag 0.6.0, Modelltypen); der Koordinator spielte die Patches ein.
Commits mit Bereich im Titel:

| Schritt | Commit | Inhalt |
|---|---|---|
| Vorab | `docs(decisions)` `de5fdc7`, `docs(perf)` `eec942c` | Antwort auf Frage 8; Android-Werte des Spikes, endgültiger Formatentscheid, Protokoll des O-6-Langzeittests |
| Basis | `feat(api,store,model)` `aace6b4` | Migration 00013 (`downloads` ohne Ablaufspalte, `download_tombstones` per Trigger aus jeder Löschung inklusive Kaskaden, `download_rules`, `profiles.pin_salt` und `offline_pin`, Offline-Spalten an `progress_events`), Vertrag 0.6.0 mit 13 neuen Operationen (75 gesamt), Modelltypen, Konfiguration `DownloadBudgetGB`/`DownloadsDir()`, deklarierte Download-API im Transcode-Paket |
| 0 Langzeittest | `docs(perf)` `eec942c`, `test(web,perf)` `d76824d` | Protokoll `docs/perf/M5-o6-langzeittest.md`, Skript `web/scripts/o6-longterm.mjs`; Desktop Chromium und Firefox am 20. Oktober gestartet, Tag 0 bestanden |
| 1 Download-Profile, Job, OPFS | `feat(transcode)` `cfd830a`, `feat(offline,visibility,api,server)` `57a104e`, `feat(web,testdata)` `29be940` | Drei Stufen (H.264/AAC, CBR), gewählte Tonspur, WebVTT-Sidecars aller Textspuren; Job `download.prepare` auf eigenem Worker, segmentweise nach `/cache/downloads`, Budget, Pause bei Live-Wiedergabe, 24-h-Aufräumen; Manifest mit Fragmentindex, Datei und Untertitel hinter dem Download-Token `dt`; im Browser Ablage in OPFS in 8-MiB-Bereichen mit Wiederaufnahme, Metadaten in IndexedDB (`idb`), Offline-Player mit hls.js und Byte-Range-Playlist |
| 2 Staffel, Auto-Downloads | in `57a104e`, `29be940` | Staffel mit einer Aktion, Regel je Profil und Gerät (nächste Folge, max. Anzahl, nur WLAN im Client, Podcasts für M6 gespeichert), Testserie mit sechs Folgen |
| 3 Sichtbarkeit, PIN, Schauzeit, Speicher | `feat(auth,playback,api)` `3cc42ef`, `57a104e`, `29be940` | `allowed_profile_ids` über `visibility.AllowedProfiles` bei jedem Sync; Offline-PIN (PBKDF2, separate Offline-PIN 6–12 Stellen, Verifier der normalen PIN entsteht im Client bei der nächsten Online-Eingabe); Tagesrest und Rücksetzzeitpunkt im Sync; Seite `/downloads` mit Speicheranzeige und Apple-Hinweis |
| 4 Ereignisse, Entzug | `3cc42ef`, `57a104e`, `29be940` | `uploadOfflineEvents` mit Uhrkorrektur, Zusammenführung nach AN-10, Nachbuchung der Schauzeit; Grabsteine als Entzugsliste im Sync und über `checkOfflineRevocations` mit dem Geräte-Cookie |
| Entscheid geteiltes Gerät | `docs(decisions,testdata)` `b088da4` | siehe «Abweichungen» |
| DB-Stand | `test(store,testdata)` `0ad5950` | `testdata/db/M5.sql`, `TestFrozenM5StateCoversM5Data` |
| Review- und E2E-Korrekturen | `fix(offline,api,admin,playback,transcode)` `a623e64`, `fix(web)` `8d2d51f`, `fix(offline)` `295645d` | siehe unten |
| Verifier-Korrekturen | `test(offline,web)` `20f909c`, `docs(status,perf)` `d2ac1fb`, `test(web)` `5a4e2cb`, `fix(web)` `6c96cef` | Folge in drei Stufen, O-5 mit Startposition und 10-s-Grenze, 10 min offline mit simulierter Zeit, Fortsetzung ab Server-Position, kein stiller Skip |
| Firefox stumm | `test(web)` `4c00a13` | `media.volume_scale` 0.0 in den Specs und im Langzeittest (Auftrag von Marco) |

### Definition of Done (Plan M5 plus 6.2) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| O-1 (Video) Film und Folge in 3 Stufen, Grösse ±10 % der Schätzung | PASS: Film und Folge je in allen drei Stufen per E2E (Film «MKV Long» −1.7/−1.7/−4.2 %, Folge der Download-Testserie −2.0/−2.0/−0.9 %, Datei in OPFS = Manifest), Integrationstest −1.0 %, echte Quellen −1.8 % (S01E01 «Mittel») und −1.5 % (4K-HDR-Ausschnitt «Hoch»), siehe `docs/perf/M5.md` 2 | `web/e2e/downloads.spec.ts`, `offline-measure.spec.ts`, `go test ./internal/offline -run 'TestPrepareWithRealEncoder|TestDecidePerProfileAndSource|TestEstimateMatchesPlan'` |
| O-2 Staffel mit 6 Folgen per 1 Aktion → 6 Downloads | PASS | `web/e2e/season-download.spec.ts`, `go test ./internal/offline -run TestCreateSeasonDownloadsVisibleEpisodes` |
| O-3 Folge n beendet → n+1 lädt; Max.-Anzahl; «nur WLAN» | PASS Desktop, Android ausstehend: Regel per Tastatur, n+1 automatisch, Max.-Anzahl, fehlgeschlagene Auto-Downloads nach 6 h erneut; «nur WLAN» per Vitest (`navigator.connection`), auf Android-Chrome ausstehend (Marco); iOS nach AN-13 ausgeblendet, ungeprüft (Entscheid 2); Podcasts M6 | `web/e2e/auto-download.spec.ts`, `go test ./internal/offline -run 'TestAutoDownloads|TestAutoDownloadRetriesAfterAFailure'`, Vitest `sync.test.ts` (`waitsForWifi`) |
| O-4 Wiedergabe im Flugmodus mit Untertitel und gewählter Tonspur | PASS Desktop, Android ausstehend: Chromium und Firefox mit Playwright `setOffline` (MSE, englische Tonspur im Manifest, zwei WebVTT-Spuren, deutsche Spur angezeigt, Spulen per Pfeiltaste). In Firefox erfasst `setOffline` die Anfragen des Service Workers nicht; das Server-Log zeigt im Offline-Fenster aber keine Anfrage an Datei, Manifest oder Untertitel. Ganz ohne Server läuft Firefox nur im Langzeittest (Wiedergabe und Sprung, ohne Untertitel und Tonspur). Apple ungeprüft | `web/e2e/offline-playback.spec.ts` |
| O-5 10 min offline gesehen → zweites Gerät zeigt Position ±10 s; Konfliktfälle AN-10 | PASS: E2E ab 30 s Server-Position, 15 s Wiedergabe im Flugmodus, das zweite Gerät zeigt die neue Position ±10 s innert 10 s nach der Wiederverbindung (gemessen 258–263 ms in Chromium und Firefox, trennscharf gegen die Startposition); 10 min Offline-Wiedergabe mit simulierter Zeit durch Player, Warteschlange und Schauzeit (61 Ereignisse, ein Upload nach der Wiederverbindung); Uhrversatz, ältere und neuere Ereignisse, «fertig» bis zum neueren Ereignis unter 90 % in Go-Tests | `web/e2e/offline-playback.spec.ts`, Vitest `offlineFlow.test.tsx`, `go test ./internal/playback -run 'TestReportOffline'` |
| O-6 kein Ablauffeld; Download nach 30 Tagen abspielbar | Schema-Test PASS; Langzeittest läuft (Desktop seit 20.10., Tag 0 und Tag 1 bestanden; Android nach dem Merge); Abschluss am 19. November 2026 ausstehend (Stand M7: «läuft, Abschluss ausstehend») | `go test ./internal/offline -run TestDownloadsHaveNoExpiryColumn`, `docs/perf/M5-o6-langzeittest.md` |
| O-7 P6 sieht den FSK-18-Download von P18 nicht; Speicheranzeige = `estimate()` ±5 %; iOS-Hinweis | PASS: online und offline auf demselben Gerät mit einem Titel ohne Freigabe (für P6 gesperrt, AN-1), FSK 18 im Go-Test; Speicheranzeige ±5 %; Hinweis mit iPhone-User-Agent, WLAN-Option dort ausgeblendet | `web/e2e/offline-profiles.spec.ts`, `go test ./internal/offline -run 'TestSyncHidesTitlesOfOtherProfiles|TestAllowedProfilesAfterRatingChange'` |
| C-1 PWA startet offline mit den geladenen Inhalten (< 3 s) | PASS: echter Kaltstart ohne Server im Langzeittest (dauerhaftes Profil, Tag 0) Chromium 1334 ms und 1357 ms, Firefox 1446 ms und 1427 ms; Neuladen im Playwright-Flugmodus bei laufendem Service Worker 76 ms (neue Tabs 88–141 ms, nur Chromium); Start mit stummem Server 2375 ms (Chromium) und 2404 ms (Firefox), diese Spec sperrt den Service Worker, die App-Hülle kommt vom Server; Playwright-Firefox kann den Flugmodus mit Service Worker nicht nachbilden (Kaltstart-Specs nur Chromium, Grund in `playwright.config.ts`) | `web/e2e/offline-cold-start.spec.ts`, `offline-start.spec.ts`, `docs/perf/M5.md` 5 |
| K-4 offline: Verifier lehnt falsche PIN ab | PASS: Verifier der normalen PIN nach Online-Eingabe, separate Offline-PIN, Bremse nach drei Fehlversuchen, Profil ohne Verifier gesperrt | `web/e2e/offline-pin.spec.ts`, `go test ./internal/auth -run 'TestOfflinePIN|TestPinSaltFollowsPIN'` |
| K-5 offline: Tagesrest → Stopp ±30 s im Flugmodus, Nachbuchung | PASS mit 1-min-Limit im E2E (Stopp nach 30–90 s mit Hinweis, Bericht zeigt die nachgebuchte Zeit); Abzug und Tageswechsel als Rechnung in `watchTime.test.ts`, Stopp im Player mit simulierter Zeit (`OfflinePlayer.test.tsx`), 10-min-Ablauf siehe Zeile O-5; Nachbuchung langer Läufe in Go | `web/e2e/offline-watch-limit.spec.ts`, Vitest `watchTime.test.ts`, `go test ./internal/playback -run 'TestReportOfflineBooksWatchTime|TestOfflineWatchTime'` |
| K-7 Entzug eines Gastes → Downloads beim nächsten Kontakt weg | PASS: Grabstein aus der Kaskade, Sync 401 → Entzugsliste über das Geräte-Cookie, Download und OPFS-Ordner weg; auch wenn inzwischen ein anderes Konto angemeldet ist | `web/e2e/offline-revocation.spec.ts`, `go test ./internal/api -run TestOfflineGuestRevocation`, `go test ./internal/offline -run TestTombstonesThroughCascades`; «anderes Konto angemeldet»: Vitest `engine.test.ts` und `OfflineLibraryPage.test.tsx` |
| Kinderprofil-Vertragstest 10.2 über alle neuen Operationen | PASS: 75 Operationen; unsichtbarer Titel → 404 bei Optionen, Anlegen, Manifest, Datei und Untertitel (Token eines Profils, das ihn nicht sieht); Sync-Antwort ohne fremde Item-IDs; Serie 12 → 16: P12 fällt beim nächsten Sync aus `allowedProfileIds`; `deleteDownload` beim unsichtbaren Titel: Hauptprofil 204 (Review K1), sonst 404 | `go test ./internal/api -run 'TestContract'`, `go test ./internal/offline -run TestAllowedProfilesAfterRatingChange` |
| Flugmodus-Test (DoD): Film mit Untertitel und Tonspur ohne Server, Fortschritt beim nächsten Kontakt zusammengeführt | PASS Desktop (Chromium und Firefox, Firefox-Vorbehalt wie bei O-4), Android-Gerät ausstehend (Marco) | `web/e2e/offline-playback.spec.ts` |
| iOS-8-Tage-Test (AN-7) | ungeprüft, keine Hardware (Entscheid 2) | `docs/perf/M5-o6-langzeittest.md` |
| `task ci` grün (6.2 Pkt. 2) | exit 0 (20. Oktober 2026, Stand `4c00a13` mit den danach committeten Doku-Dateien, Binary `4c00a13-dirty`): gen, gen:check, lint 0 issues, Go `-race` alle 22 Pakete, Vitest 370 (web, 50 Dateien) + 4 + 2, Build, Playwright 250 (Chromium und Firefox, 0 fehlgeschlagen, 0 übersprungen, 12.8 min), Storybook 104; erster Lauf auf `295645d` ebenfalls exit 0 (Playwright 246, Vitest 359) | `sg docker -c 'task ci'` |
| Tastaturtest je neue Seite (6.2 Pkt. 3) | PASS: `/downloads`, Download-Dialog, `/offline` (Pfeiltasten zwischen Profilen und Karten in beiden Browsern), `/offline/play` | `downloads.spec.ts`, `offline-pin.spec.ts`, `offline-playback.spec.ts`, `offline-cold-start.spec.ts` |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M5.sql`; alle Stände M1a bis M5 migrieren bis 00013 | `go test ./internal/store -run 'TestFrozen|TestMigration00013'` |
| Messwerte (6.2 Pkt. 5) | `docs/perf/M5.md` (Spulen über MSE, Grösse gegen Schätzung, Vorbereitungszeit, Kaltstart, Abrufrate, Gerätecheckliste) | |
| Code-Review | Server 20 Befunde (0 kritisch, 6 wichtig, 14 klein), Web 26 Befunde (0 kritisch, 11 wichtig, 15 klein); alle behoben oder begründet (siehe unten) | Sitzungsnotizen `m5-review-server.md`, `m5-review-web.md`, `m5-server-fixes-notes.md`, `m5-web-e2e-notes.md` |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen» (O-3, O-4 und O-6 auf Android ausstehend, iOS ungeprüft, Entscheid «geteiltes Gerät» offen); eigene Läufe: Go-Tests der M5-Pakete mit `-race`, Vitest 359, 26 Playwright-Tests in beiden Browsern, `task gen`/`gen:check`, `task lint`; 15 Berichtskorrekturen übernommen (Werte nach `295645d`, O-1 Folge, O-5, K-5, O-4/Firefox, C-1, Langzeittest-Bewertung, O-7, K-7, Vertrag `deleteDownload`, Gerätecheckliste, Beobachtungspunkte, toter Skip-Zweig) | Sitzungsnotizen `m5-verifier.md` |

### Befunde der Review-Passage und ihre Behebung

- **Server (wichtig):** Fortsetzen prüft einen Fingerabdruck des Plans, die Segmentzeiten und das Init (sonst
  gemischte Encodes nach einem Planwechsel); die Admission wird vor den Untertiteln freigegeben (sonst blockierte die
  Untertitel-Extraktion einen Live-Start); `Reconcile` reiht hängende Downloads ohne Job neu ein; fehlende Probe
  wartet statt endgültig zu scheitern, ein neuer Auftrag setzt `failed` zurück; Zusammenführen im Admin behält die
  Downloads; das Geräte-Cookie folgt dem Gerät der Sitzung und wird erneuert (sonst erreicht die Entzugsliste nach
  400 Tagen das Gerät nicht mehr).
- **Server (klein):** Löschen nur durch Anfrager, erlaubtes oder Hauptprofil; `paused` beim Warten auf Admission;
  Offline-Ereignisse gegen ihren Download geprüft und auf 30 Tage begrenzt; alle Textuntertitel in einem ffmpeg-Lauf;
  PIN-Material nur für gehaltene Downloads; Regeln im Sync «best effort»; Staffel ohne anlegbare Folge 422; Token für
  unbekannte Downloads 401; ETag an der Datei; Räumen nur bei fehlender Kopie; Test für das `pin_salt` der Migration.
- **Web (wichtig):** Offline-Profilwahl schlug die Sitzung (jetzt spielt der Player aus `/downloads` für das
  Sitzungsprofil, die Wahl verfällt ausserhalb von `/offline`); Offline-Bibliothek nur für das zuletzt angemeldete
  Konto, Entzugsprüfung für Downloads anderer Konten; PIN-Material nur bei Downloads; Schauzeit-Stopp gegen
  Medientasten und Bild-in-Bild abgesichert; letztes Ereignis beim Verlassen des Players; Quota- und Worker-Fehler
  stoppen die Abrufschleife mit Hinweis; Löschen wartet auf den Schreiber; Web Locks für mehrere Tabs; Kaltstart mit
  stummem Server nach 2 s offline; kein Laufzeit-Skip mehr.
- **E2E-Befunde:** Die Länge eines Blocks wurde nach der Übergabe des Puffers an den Worker gelesen (jeder Block
  zählte 0 Byte, der Abruf begann endlos neu); der Service Worker übernahm seine Seite nicht (`clientsClaim`); «Niedrig»
  wurde bei Quellen mit kleiner Bitrate grösser als die Kopie in «Hoch» (Encode jetzt höchstens mit der Quellbitrate).
- **Befunde des Verifiers:** fehlende Belege für die Folge in drei Stufen, für O-5 mit Startposition und
  10-s-Grenze und für einen 10-min-Ablauf ergänzt; dabei fiel auf, dass die Offline-Wiedergabe nur die eigene
  Position des Geräts kannte (ein online bis 30 s gesehener Film begann offline bei 0). Jeder Sync speichert jetzt den
  Serverfortschritt des aktiven Profils am Download, der Player nimmt die neuere Position (`6c96cef`). Ein toter
  Skip-Zweig im Integrationstest ist entfernt, die Bewertung des Langzeittests prüft alle drei Kriterien.
- **Firefox stumm in Playwright** (Auftrag von Marco): die Sinustöne der Testmedien piepten in jedem Lauf aus den
  Lautsprechern; Firefox startet in den Specs und im Langzeittest mit `media.volume_scale` 0.0 (`4c00a13`).

### Abweichungen und Entscheide während M5

- **Geteiltes Gerät mit mehreren Konten** (Entscheidungsprotokoll, Vorschlag des Koordinators nach dem Web-Review,
  von Marco zu bestätigen): die Offline-Bibliothek zeigt nur das zuletzt angemeldete Konto; nach dem Abmelden bleiben
  die Downloads gesperrt auf dem Gerät; Downloads anderer Konten werden nach jedem Sync gegen die Entzugsliste geprüft;
  Offline-PIN und lokale Schauzeit gelten im Offline-Player auch online.
- **Wiedergabe über MSE statt Blob-URL:** siehe Spike-Entscheid; Blob-URL nur noch als Rückfall ohne Media Source
  (verschiebt die +10-s-Zeitachse der HLS-Segmente).
- **Eine Zeile je Gerät und Titel:** hält ein anderes Konto denselben Titel auf dem Gerät, antwortet ein Einzeltitel
  422 `not_downloadable`, eine Staffel überspringt die Folge.
- **Budget:** angefangene Vorbereitungen zählen mit ihrer Schätzung, wartende nicht; ein Download grösser als das ganze
  Budget läuft, wenn sonst nichts liegt.
- **Bild-Untertitel** (PGS, DVD) gibt es offline nicht, nur Textspuren als WebVTT.
- **Langzeittest O-6:** Start erst mit dem fertigen M5-Server (Desktop am Ende des M5-Tages, Android mit dem Image nach
  dem Merge), nicht am ersten Tag; `persist()` in den Playwright-Profilen nicht gewährt.
- **Playwright-Firefox und Flugmodus:** `setOffline` erfasst die Anfragen des Service Workers nicht; die
  Kaltstart-Specs laufen nur in Chromium, Firefox ist im Langzeittest ohne Server echt offline geprüft.
- **Messskript gegen das NAS:** ein erster Messlauf legte eine Serienbibliothek irrtümlich auf den ganzen NAS-Ordner
  `Serien`; der Server wurde nach rund vier Minuten gestoppt, er hatte nur lesend Verzeichnisse durchlaufen (Medien sind
  read-only eingebunden, geschrieben wurde nur in den eigenen Cache). Gemessen wurde danach an einer lokalen Kopie von
  S01E01 und am 120-s-HDR-Ausschnitt aus M4.

### Nicht in M5 (Plan) und offene Punkte für M6

1. Android (Marco, nach dem Merge und dem neuen Image): Gerätecheckliste 10.1 für Download und Flugmodus, «nur WLAN»
   für Auto-Downloads (O-3), Start des O-6-Langzeittests; Spike-Tag 3 und 8 (bis 27. Oktober) und eine gültige
   30-min-Dauerwiedergabe, danach Spike-Seite und `SLOUCHCAT_SPIKE_DIR` entfernen.
2. Bestätigung des Entscheids «geteiltes Gerät» durch Marco (Pull Request).
3. O-6-Langzeittest Tag 1, 3, 8 und 30 (`node web/scripts/o6-longterm.mjs check`, ohne laufenden Server auf 8098);
   Abschluss in M7.
4. Hörbücher und Podcast-Episoden als Downloads, Auto-Download neuer Podcast-Episoden (M6; das Regelfeld ist
   gespeichert).
5. Admin: Einstellung des Download-Budgets und Belegung im Dashboard (M8); Hinweis zu AN-4 ergänzen, dass auch
   «Datei ignorieren» (B-9) und das Aufräumen von Geräten nach 400 Tagen Downloads entziehen.
6. Server: Grabsteine tragen keine Konto-ID (Gerät vergessen, neue Geräte-ID: Downloads bleiben auf dem alten Gerät,
   O2 des Web-Reviews); `checkOfflineRevocations` ohne Ratenbegrenzung; CBR bei QSV, NVENC und VAAPI nur über die
   Argumente geprüft, Grösse gegen Schätzung auf CT 190 (QSV) messen; Byte-Gleichheit des Init beim Fortsetzen für
   Hardware-Encoder (der Fingerabdruck fängt Abweichungen ab).
7. Web: nach einem Datenbank-Upgrade in einem anderen Tab braucht ein alter Tab ein Neuladen; ob das asynchrone
   Schreiben beim Schliessen eines Tabs (`pagehide`) in jedem Browser festgeschrieben wird; nach «Speicher voll» wartet
   der nächste Abrufversuch 5 min, auch wenn inzwischen Platz frei wurde.
8. Weiter offen aus M4: `AdminDevice.current` und `stoppedBy` im 401, Abspann-Erkennung bei Vorschau nach dem Abspann,
   Einbrennen eingebetteter Textspuren über Sidecar-Cache, Schriften für libass, Audio-Bitrate in der Probe, CSP.

Testserver: seit dem 20. Oktober 2026 sind die echten NAS-Bibliotheken schreibgeschützt in den CT 190 eingebunden
(`/media/nas/Filme`, `/media/nas/Serien`, …), damit Marco den Stack dort mit seiner Sammlung nutzen kann; die
Bibliotheken sind in `/admin/libraries` noch anzulegen (`docs/dev/TESTSERVER.md`).

## Meilenstein M4 – Wiedergabe komplett

Vorab geklärt mit Marco (20. Oktober 2026): die NVIDIA-Container-Laufzeit installiert er mit
`scripts/install-nvidia-container-toolkit.sh` (nachträglich am 20. Oktober 2026 installiert, `tonemap_cuda`
im Container gemessen, siehe W-2 und W-3); echte
Dolby-Vision- und HDR-Dateien vom NAS dürfen lesend für Messungen genutzt werden (nichts davon im
Repository). Alle fünf Schritte aus `docs/planning/03-plan-v1.md` (M4) sind umgesetzt; der Entwurf liegt in
den Sitzungsnotizen (`m4-design.md`). Die Umsetzung lief mit vier Executor-Agenten (Opus) parallel
(Transcode, Playback/API, Intro-Erkennung, Web) auf einer gemeinsamen Basis (Migration 00011, Vertrag 0.5.0,
Modelltypen); die Patches der Ströme A, C und D wurden vom Koordinator eingespielt und committet. Commits
mit Bereich im Titel:

| Schritt | Commit | Inhalt |
|---|---|---|
| Vorab | `chore(scripts)` `52d9515` | Installationsskript für das NVIDIA Container Toolkit |
| Basis | `feat(api,store,model)` `14b04fa` | Migration 00011 (Geräteprofile, Entscheidungsspalten der Wiedergabesitzung, Limits an Profil und Konto, `media_segments`, Index für den Schauzeit-Bericht), Vertrag 0.5.0 mit 7 neuen Operationen (62 gesamt), Modelltypen, `transcode.Target`-Felder, DV-Erkennung in der Probe |
| 1 Geräteprofile, Entscheidungstabelle | `feat(auth)` `778c568`, `feat(playback,visibility)` `dfa264f` | Fähigkeitsprofil je Gerät (Container, Codecs, Auflösung, HDR, Name, Typ) wird beim Start gespeichert; Entscheidungstabelle Direct/Remux/Transcode aus Probe, Profil, Wunsch und Limits; Dolby Vision 8 über die Basisschicht, 5/7 Transcode; Entscheidung in `playback_sessions` gespeichert, Wiederaufbau nach Neustart aus den gespeicherten Spalten (M1b-Punkt «nur AAC» erledigt) |
| 2 HDR-Pipelines | `feat(transcode)` `413cbff`, `feat(transcode,config)` `10a2a81`, `feat(testdata)` `16d77b1`, `test(testdata)` `af180fd`, `cf5f05a` | NVENC: `tonemap_cuda`, sonst `libplacebo` über Vulkan (bt.2390), sonst CPU; QSV: Hybrid (GPU-Skalierung, CPU-Tonemapping) als Standard, `vpp_qsv` über `SLOUCHCAT_HDR_TONEMAP=gpu`; VAAPI best effort; Skalierung vor dem Tonemapping; `Detect` mit HDR-Testencode je Tonemapper; HDR10+-Erkennung aus dem ersten Frame; P-6-Skript `testdata/hdr-quality.sh` mit Testbild und Referenzbildern je Backend |
| 3 Spuren, Untertitel, Sidecars | in `dfa264f`, `413cbff`, `feat(library)` `6a6f75d`, `chore(server)` `48fdbf1`, `97d2b7a` | Tonspur-Mapping, Einbrennen (Text über libass, PGS/DVD über `overlay`, auf allen Backends, Zeitstempel über `-copyts` auch nach Neustart), WebVTT-Endpunkt `getPlaybackSubtitle` (Cache je Wiedergabesitzung), `getItemTracks`, Sidecar-Abgleich bei Pfad- und Voll-Scan ohne erneute Probe (I-5), Testmedien Multi Audio, PGS (`make-pgs.py`) |
| 4 W-8, K-5, Admission | in `778c568`, `dfa264f`, `1e70c9e` | Qualitätsleiter 1080p/720p/480p/360p, manuelle Wahl, Limits je Profil und Gast (`limits`-Objekt), `maxStreams` → 409, Schauzeit-Limit → 403 und Widerruf laufender Sitzungen, Bericht `getWatchReport`; `budget_lowered` bei SDR-Quellen, Kosten 6 für HDR mit CPU-Tonemapping |
| 5 Intro, Abspann, Kapitel, Stream-Übersicht | `feat(segments)` `4423738`, `feat(library)` `7d0c477`, `test(testdata)` `7ee9914`, `feat(admin,api)` `1e70c9e`, `test(api)` `4c175da` | Chromaprint über ffmpeg, Paarvergleich je Staffel, Kapitelnamen mit Vorrang, Job `segments.detect` nach Probe und Serien-Scan, Testsaison «Slouchcat Test Intro Show»; `listStreams`/`stopStream`, `listDevices`/`deleteDevice`; Vertragstest über alle 62 Operationen |
| Web | `feat(web)` `9e0d8a2`, `c9cedf6`, `d61a76c`, `test(web)` `52cb289` | Player mit Menüs Spuren (T), Qualität (Q), Kapitel (C), `<track>`-Untertitel, «Intro überspringen» (S), Auto-Next-Countdown (N), Hinweise für Schauzeit, Stream-Limit und beendete Streams, Geräteprofil aus `canPlayType`/MSE/`dynamic-range`; `/settings/profiles` mit Limits und Schauzeit-Bericht (`ui/BarChart`), Gast-Limits; `/admin/devices`, `/admin/streams` (`ui/DataTable`); neun neue Playwright-Specs (`admin-devices`, `admin-streams`, `audio-transcode`, `chapters`, `hdr`, `quality`, `sidecar`, `tracks`, `watch-limit`) |
| Messungen | `test(segments)` `3f76fd1` | `TestRealSeason` (hinter `SLOUCHCAT_REAL_SEASON`) für die W-9-Messung an einer echten Staffel |
| E2E-Korrekturen | `fix(web)` `e89721c`, `fix(web,api)` `b55e326`, `11c94fb` | siehe «Abweichungen» |
| Review-Korrekturen | `fix(playback)` `21771b6`, `384bce2`, `fix(playback,api)` `c662fa8`, `fix(transcode,playback)` `a2f96d1`, `fix(segments,library)` `e5607e5`, `fix(admin,api)` `9f6d592`, `fix(web)` `4144744`, `0ad7987`, `cf36bed`, `test(web)` `f893d7a` | siehe unten; Migration 00012 (`progress_events.playback_session_id`) |

### Definition of Done (Plan M4 plus 6.2) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| W-1 H.264/AAC-MP4 → Direct Play, 0 ffmpeg | PASS (seit M1a); neu: Direct Play folgt dem gemeldeten Geräteprofil (Container, Codecs, HDR, Auflösung); Chromium und Firefox melden MKV und spielen H.264-MKV direkt | `go test ./internal/playback -run TestDecide`, `web/e2e/player.spec.ts` |
| W-2 Testmatrix 10.3, HDR → SDR nach P-6 auf NVENC und QSV | PASS mit einer Ausnahme: Software-Referenz, NVENC (`libplacebo`, SSIM 0.9915, ΔE Keil 2.91 / Hautton 3.71) und QSV-Hybrid (ΔE 0.00) bestanden; `vpp_qsv` (ΔE Hautton 10.25) nicht bestanden und deshalb nicht mehr Standard; `tonemap_cuda` im Container (Standard im Image): SSIM 1.000, ΔE Keil 3.22 / Hautton 5.87, knapp über der Grenze gegen die `hable`-Referenz, gegen die Ausgangsfarben aber am genauesten (3.78); Abweichung von Marco akzeptiert, Referenz `nvenc-cuda`; Sichtfreigabe der Referenzbilder (P-6 a) durch Marco ausstehend | `docs/perf/M4.md` 1, `testdata/hdr-quality.sh`, Referenzbilder `testdata/reference/hdr/` |
| W-3 4K-HEVC-HDR startet im Handy-Profil < 5 s (NVENC, QSV); Spulen < 3 s | PASS: echte 4K-HDR10+-Quelle → 720p: NVENC `tonemap_cuda` im Container 0.50 s (zwei Streams 0.75 s, 1080p 0.50 s), NVENC `libplacebo` auf dem Host 1.55 s (CPU-Tonemapping 0.80 s), QSV-Hybrid 1.85 s (`vpp_qsv` 0.80 s), zwei gleichzeitige Streams im CT 3.0 s (gemessen mit `--budget 1000`; im Betrieb lässt das QSV-Budget 8 bei Kosten 6 nur einen zu); Spulen unverändert aus M1b; im Browser `hdr.spec.ts` (generierter 4K-Clip); Start auf dem Android-Handy (Stoppuhr) ausstehend, iPhone ungeprüft (Entscheid 2) | `docs/perf/M4.md` 2, `web/e2e/hdr.spec.ts`, `hls.spec.ts` |
| W-4 Tonspurwechsel ≤ 3 s; SRT/ASS als WebVTT; PGS eingebrannt | PASS: AC3-Spur gewählt → neue Sitzung an derselben Position, `data-switch-ms` ≤ 3000 in Chromium und Firefox; SRT als `<track>` (`text/vtt`); PGS → `reason subtitle_burn`, spielt | `web/e2e/tracks.spec.ts`, `go test ./internal/transcode -run 'BurnIn|Subtitles'`, `go test ./internal/playback -run 'Subtitle|Track'` |
| W-5 Fortschritt | PASS (seit M1a) | `web/e2e/resume.spec.ts` |
| W-6 Stream-Übersicht mit Beenden, Client stoppt < 5 s, ffmpeg endet | PASS: zwei Streams (Hauptprofil Direct Play, zweites Profil Transcode) mit Profil, Gerät, Titel, Qualität, Modus HW/SW; Beenden per Tastatur → Player zeigt «Die Wiedergabe wurde beendet.» innert 5 s, Zeile verschwindet, Transcoder gestoppt (Go-Test mit Fake-Transcoder) | `web/e2e/admin-streams.spec.ts`, `go test ./internal/playback -run 'StreamOverview|Streams'`, `go test ./internal/api -run 'Streams'` |
| W-7 Gerät mit Eigentümer, letztem Kontakt ±1 min, Fähigkeitsprofil | PASS: nach einer Wiedergabe zeigt `/admin/devices` Name, Typ, «E-Mail · Profil», letzten Kontakt, Container, Codecs, Auflösung, HDR; Vergessen beendet die Sitzungen des Geräts | `web/e2e/admin-devices.spec.ts`, `go test ./internal/playback -run Device`, `go test ./internal/api -run Devices` |
| W-8 max. 720p → ≤ 720p; max. 1 Stream → 409 mit Hinweis; 4 Mbit/s → ≤ 4 Mbit/s; manuelle Wahl | PASS: Limit 720p → `reason max_height`, 1080p nicht verfügbar; Bandbreite 4000 kbit/s → 480p (Ton zählt mit); 1 Stream → zweiter Start 409 `stream_limit_reached` mit Hinweis (je Profil und je Gastkonto); 480p gewählt → Transcode 480p; `budget_lowered` nur, wenn die Kosten sinken | `web/e2e/quality.spec.ts`, `go test ./internal/playback -run 'Limit|Quality|Budget|GuestStreamCap'`, `go test ./internal/api -run 'TestHTTPPlaybackLimits|TestHTTPGuestStreamCapAndSwitch'` |
| W-9 Intro ±2 s bei ≥ 80 % einer Test-Staffel; Auto-Next nach 10 s; Kapitelsprung ±1 s | PASS: generierte Staffel 5/5 Intros und 5/5 Abspänne ±2 s; echte Staffel (9 Folgen) 8/9 Intros, Stichprobe per Standbild ±1 s Start; Countdown und Navigation, Kapitelsprung | `docs/perf/M4.md` 3, `go test ./internal/segments`, `go test ./internal/library -run TestSegmentsDetectGeneratedSeason`, `web/e2e/chapters.spec.ts` |
| K-5 Limit 30 min → Stopp innert 30 s mit Hinweis; Bericht ≤ 1 min Abweichung | PASS: Uhr-Mock (Ereignisse à 10 s, Widerruf beim Erreichen, Start danach 403, nächster Tag frei, Bericht), E2E mit 1-min-Limit (403 mit Hinweis, laufender Player stoppt, Bericht zeigt die Minute); `playedMs` serverseitig gekappt, Duplikate ignoriert, Sweep als Rückhalt | `go test ./internal/playback -run 'Watch|TestReportDedupesAndBoundsPlayedTime'`, `go test ./internal/api -run 'WatchLimit|WatchReport'`, `web/e2e/watch-limit.spec.ts` |
| I-5 Sidecar `.de.srt` → im Player < 60 s | PASS: Datei während der Wiedergabe abgelegt → Spurmenü zeigt sie (im CI-Lauf 4 nach 30.5 s in Chromium und 30.8 s in Firefox) | `web/e2e/sidecar.spec.ts`, `go test ./internal/library -run Sidecar` |
| Testmatrix 10.3 auf Chromium und Firefox, Safari ungeprüft | PASS: Direct Play (`player.spec.ts`), HEVC SDR Transcode/Remux (`hls.spec.ts`, mit eingeschränktem Profil), HDR10 4K Tonemap (`hdr.spec.ts`), DTS/AC3 Audio-Transcode (`audio-transcode.spec.ts`), PGS Einbrennen und SRT/ASS WebVTT (`tracks.spec.ts`); Android Chrome (manuell) ausstehend (Marco), Safari/WebKit ungeprüft (Entscheid 2) | `task e2e` |
| Kinderprofil-Vertragstest 10.2 über alle neuen Operationen | PASS: 62 Operationen; `getItemTracks` und `getPlaybackSubtitle` bei unsichtbarem Titel 404, fremdes Token 401; `getWatchReport` nur Hauptprofil; Geräte- und Stream-Übersicht nur Admin (Gast 403) | `go test ./internal/api -run 'TestContract'` |
| `task ci` grün (6.2 Pkt. 2) | exit 0 (Lauf 4, 20. Oktober 2026, Stand `c124ab8` mit den noch nicht committeten Dateien von `80b23e5`, Binary `c124ab8-dirty`): gen, gen:check, lint (0 issues), Go-Tests mit `-race` (alle Pakete ok, aus dem Testcache; frischer Lauf mit `-count=1` durch den Verifier grün), Vitest (web 216 in 31 Dateien, api-client 4, design-tokens 2), Build, Playwright 220 (Chromium 110, Firefox 110, 29 Specs), Storybook-Screenshots 101; Protokoll `.dev/ci-m4-run4.log` | `task ci` (Storybook-Schritt über `sg docker -c`) |
| Tastaturtest je neue Seite (6.2 Pkt. 3) | PASS: Player-Menüs Spuren/Qualität/Kapitel, Intro-/Countdown-Knöpfe, `/admin/devices`, `/admin/streams`, Profil-Limits und Schauzeit-Bericht, Gast-Limits | `tracks.spec.ts`, `quality.spec.ts`, `chapters.spec.ts`, `admin-devices.spec.ts`, `admin-streams.spec.ts`, `profiles.spec.ts`, `access.spec.ts` |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M4.sql` (Profil mit Limits, Gerät mit Fähigkeitsprofil, Wiedergabesitzung mit Qualität und Spuren, Segmente der Intro-Staffel); M1a bis M3 migrieren bis 00012 | `go test ./internal/store -run TestFrozen` |
| Messwerte (6.2 Pkt. 5) | `docs/perf/M4.md` (P-6, W-3 je Backend, W-9 generiert und echt, Gerätecheckliste) | |
| Code-Review (separate Passage, Opus) | Server 20 Befunde (0 kritisch, 4 wichtig, 16 klein), Web 23 Befunde (0 kritisch, 2 wichtig, 21 klein); alle behoben bis auf zwei Vertragsergänzungen (`AdminDevice.current`, Grund im 401) und zwei akzeptierte Grenzen (kopierter Mehrkanalton unter einem Bandbreitenlimit; ein Client ohne Fortschrittsereignisse wird nur über den Sweep gestoppt), siehe unten | Review-Berichte `m4-review-server.md`, `m4-review-web.md`, Fix-Notizen in den Sitzungsnotizen |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen» (P-6/W-2: `tonemap_cuda` im Container ungemessen, QSV nur im CT 190, Sichtfreigabe der Referenzbilder ausstehend; W-3 auf Endgeräten nicht gemessen; Android-Spalte der Testmatrix und Gerätecheckliste ausstehend, Apple ungeprüft; zwei Vertragsergänzungen und zwei akzeptierte Review-Grenzen offen); eigene Läufe: `go test -race -count=1 ./...` grün, alle zitierten Go-Tests grün, Vertragstests 23 PASS (62 Operationen), lint 0 issues, `pnpm -w run lint` grün, Vitest 216/216, Playwright-Auswahl (tracks, quality, watch-limit, admin-streams; Chromium) 11/11, P-6 software und nvenc pass (SSIM 0.9917, ΔE 2.91/3.71), W-3 NVENC 1403 ms, W-9 echte Staffel 8/9 identisch, kein «ß»; dreizehn Berichtskorrekturen übernommen (Spec-Zahl, I-5-Zeit, drei Testbefehle, CI-Stand, W-2, W-3, Testmatrix, Review, `M4.md` Kette und Keil) | Verifier-Bericht `m4-verifier.md` |

### Befunde der Review-Passage und ihre Behebung

- **Wichtig (Server):** `maxStreams` war über «gleiche Sitzung, gleicher Titel» umgehbar (ein Start ersetzt
  jetzt die alte Wiedergabe desselben Titels serverseitig, `stopped_by client`); der Gast-Deckel zählte je
  Profil statt je Konto (jetzt beides); die WebVTT-Konvertierung lief je Anfrage (jetzt gemeinsam je Sitzung
  und Spur, höchstens 4 parallel, fehlende Quelle 404); K-5 hing allein an `playedMs` des Clients (Duplikate
  werden ignoriert, `playedMs` auf die Serverzeit seit dem letzten Ereignis plus 10 % gekappt, der Sweep
  beendet überzogene Profile auch ohne Ereignisse; Migration 00012 speichert die Wiedergabesitzung am
  Ereignis).
- **Klein (Server):** `budget_lowered` nur bei sinkenden Kosten; Dolby Vision 8.2/9.2 (SDR-Basisschicht)
  gilt als SDR; reserviertes PIN-Gerät nicht in der Geräteliste; `deleteDevice`-Text korrigiert;
  `deviceId` aus dem Request nur im Cookie-Muster und nur für eigene oder neue Geräte; Bandbreitenlimit
  zählt den Ton mit; `.sub`-Sidecars und DVB → 422 `not_burnable`; Kapitel-Segmente längenbegrenzt;
  Segment-Job rollt bei veralteter Staffel zurück; `Problem.extensions` im Schema; Startsperre mit
  Kontext; K-5-Nachprüfung nach dem Einfügen; Wiederaufbau mit aufgezeichneter Stufe; Bericht in einer
  Abfrage. Akzeptiert: kopierte AAC-5.1-Spuren über 160 kbit/s unter einem Bandbreitenlimit (Audio-Bitrate
  nicht in der Probe); ein Client ohne Fortschrittsereignisse wird nur über den Sweep gestoppt.
- **Wichtig (Web):** ein eigener Spur-, Qualitäts- oder Versionswechsel meldete «Verwaltung hat beendet»
  (die alte Sitzung gilt jetzt als verlassen, hls.js stoppt das Laden); Vollbild endete bei jedem Wechsel
  und beim Auto-Next (jetzt auf `document.documentElement`).
- **Klein (Web):** Menüs und N nach einem Ende gesperrt; Qualitätsknopf zeigt die vom Server gelieferte Stufe
  (Regel 12); neutraler Text «Die Wiedergabe wurde beendet.» bei 401 (der Vertrag kennt noch keinen Grund);
  Fokus nach Überspringen und Countdown; Enter/Space bricht den Countdown ab; Live-Region nur beim Start und
  bei 3 s; Backspace schliesst Menüs; `data-start-ms` nur für die erste Sitzung; Admin-Seiten: Fokus nach
  Beenden/Vergessen, kein Polling bei offenem Dialog, Warnung beim eigenen Gerät; Gerätegrösse unabhängig
  von der Ausrichtung; 2160p nicht mehr als Limit; Texte und Trenner aus den Locales; DataTable mit
  180-ms-Fokusübergang; Specs ohne Zusicherungen, die nicht fehlschlagen können. Offen: `AdminDevice.current`
  und ein Grund im 401 (`stoppedBy`) als Vertragsergänzung.

### Abweichungen und Entscheide während M4

- **Agenten im selben Worktree:** die Sitzungsisolierung verweigerte den vier Executor-Agenten Git-Befehle und
  teils Schreibzugriffe in eigenen Worktrees; drei Ströme arbeiteten in Kopien und lieferten Patches, die der
  Koordinator einspielte und committete; der Playback-Strom und die Fix-Agenten committeten direkt im
  M4-Worktree. Zwischenstände einzelner Commits bauen nicht für sich (api erst mit `1e70c9e`).
- **QSV tonemappt auf der CPU** (`SLOUCHCAT_HDR_TONEMAP=auto`): `vpp_qsv` reisst die ΔE-Grenze von P-6
  (Hautton 10.25); die Hybrid-Kette trifft die Referenz exakt, kostet aber 12 CPU-s je 20 s auf dem CT
  (2.4× Echtzeit bei 720p, 1.2× bei 1080p). Kosten eines HDR-Transcodes mit CPU-Tonemapping: 6 statt 4. Die
  GPU-Variante bleibt per `SLOUCHCAT_HDR_TONEMAP=gpu` wählbar. OpenCL-Tonemapping (Weg von Jellyfin) scheitert
  am fehlenden Intel-OpenCL-Treiber im Image (nicht in Debian `main`).
- **NVENC auf dem Host über `libplacebo`** (ffmpeg 6.1.1 ohne `tonemap_cuda`): Start 1.55 s statt 0.80 s mit
  CPU-Tonemapping, dafür ein Drittel der CPU-Zeit; das Image nutzt `tonemap_cuda`.
- **`tonemap_cuda` mit Abweichung akzeptiert** (Nachtrag 20. Oktober 2026, Entscheid Marco): im Container
  ΔE Hautton 5.87 gegen die `hable`-Referenz (Grenze 5), weil BT.2390 die Mitten heller lässt; gegen die
  Ausgangsfarben des Testbilds ist es die genaueste Kette (3.78 gegen 7.02 für `hable`). Start 0.50 s, 1.2 CPU-s
  je 20 s. Referenzbilder als eigenes Verzeichnis `testdata/reference/hdr/nvenc-cuda/`, `hdr-quality.sh` misst
  mit `CONTAINER=` im laufenden Container.
- **Direct Play von MKV:** Chromium und Firefox melden den Matroska-Container; H.264/AAC-MKV spielt dort
  direkt (W-1), die Remux-Messung schränkt das gemeldete Profil deshalb ein (`limitCapabilities`).
- **HDR-Durchleitung nur mit `hdr: true`** des Geräts (M1b remuxte HDR für Safari immer).
- **Handy-Deckel 720p nur für Transcodes**, nicht für Direct Play und Remux (10.3 Zeile 1).
- **Limits als ein Objekt** (`limits`) in Profil- und Gast-Anfragen: ein weggelassenes Feld löscht das
  Limit; `oneOf`-Nullwerte liessen sich im Codegen nicht von «fehlt» unterscheiden.
- **Testsaison länger als geplant** (98–106 s statt 60 s, Abspann 25 s): die Erkennungsregeln (Kopf 40 %,
  Abspann ≥ 20 s) hätten bei 60 s nichts finden dürfen.
- **Abspann-Erkennung an echten Folgen** scheitert an der Vorschau nach dem Abspann (endet > 5 s vor dem
  Dateiende); offener Punkt.
- **GitHub-Runner und Transcode-Budget:** auf dem Runner (4 vCPU, Software-Backend, Budget 2) hielt ein von einer
  vorigen Spec hinterlassener Transcode (Seite geschlossen, bevor der Stop ankam; eine ruhende Sitzung behält ihr
  Budget bis zum Idle-Stopp nach 60 s) das Budget, und `hls.spec.ts`/`quality.spec.ts` liefen in `transcode_busy`
  (Push-Lauf zu PR #6 rot, Pull-Request-Lauf desselben Commits grün). Die `owner`-Fixture beendet jetzt vor und
  nach jedem Test alle laufenden Streams über `DELETE /admin/streams/{id}`.
- **`TestScanbench`** (cmd) ist bei Einzelläufen zeitabhängig (Beobachtung des Playback-Stroms, im vollen
  Lauf grün).
- **Spike-Seite und `SLOUCHCAT_SPIKE_DIR` bleiben** bis zum Ende von Marcos Android-Lauf (27. Oktober 2026).

### Nicht in M4 (Plan) und offene Punkte für M5

1. Erledigt (20. Oktober 2026): NVIDIA-Container-Laufzeit installiert, P-6 und W-3 mit `tonemap_cuda` im
   Container gemessen (`docs/perf/M4.md` 1 und 2), Referenzbilder `testdata/reference/hdr/nvenc-cuda/`. Offen
   bleibt nur die Frage, ob die P-6-Referenz für ΔE langfristig `hable` bleibt (sie dunkelt Hauttöne ab, siehe
   `docs/perf/M4.md` 1).
2. Sichtprüfung der P-6-Referenzbilder durch Marco (`testdata/reference/hdr/`), Android-Startzeit (Stoppuhr),
   Gerätecheckliste Android; Safari/iOS ungeprüft (Entscheid 2).
3. Vertrag: `AdminDevice.current` (eigenes Gerät markieren) und ein Grund (`stoppedBy`) im 401 der
   Stream-Endpunkte, damit der Player «Verwaltung» von «Schauzeit» und «Abmeldung» unterscheidet.
4. Abspann-Erkennung bei Vorschau nach dem Abspann (Stille-/Schwarzbild-Erkennung); Paarvergleich O(n²)
   bei grossen Staffeln (24 Folgen ≈ 10 s CPU).
5. Einbrennen eingebetteter Textspuren liest die Quelle bei jedem ffmpeg-Start (kalt rund 12 s bei 3 GB über
   NFS); Vorschlag: Spur einmal extrahieren und als Sidecar einbrennen. Schriften für libass im Image prüfen.
6. Audio-Bitrate in der Probe (Bandbreitenlimit mit kopiertem Mehrkanalton), Admin-Dashboard und
   Budget-Einstellungen (M8), CSP-Header (M8/M10), YouTube-Stufe und TVDB gegen echte Dienste (seit M3),
   Spike-Entscheid nach dem 27. Oktober.

## Meilenstein M3 – Bibliothek komplett

Vorab beantwortet (Entscheidungsprotokoll, 19. Oktober 2026): Frage 7 (AN-9 Vererbung, AN-15 eigene
Schlüssel) wie vom Plan vorgeschlagen; dazu Marcos Wunsch nach weiteren Metadaten-Anbietern als
Anbieter-Schnittstelle mit TMDB als Hauptquelle und TVDB als optionalem zweiten Anbieter (ADR 0006), Podcast-
Quellen in M6, Radio nicht in Spec v1. Alle fünf Schritte aus `docs/planning/03-plan-v1.md` (M3) sind
umgesetzt; der Entwurf liegt in `.omc/plans/m3-design.md` (Kopie in den Sitzungsnotizen). Die Umsetzung
lief mit vier Executor-Agenten (Opus) parallel in eigenen Worktrees (Scanner, Metadaten, API/Admin, Web)
auf einer gemeinsamen Basis (Migration, Vertrag, Modelltypen); Commits mit Bereich im Titel:

| Schritt | Commit | Inhalt |
|---|---|---|
| Vorab | `docs(decisions)` `8a43da2` | Antwort auf Frage 7 |
| Basis | `feat(api,store)` `4404f80`, `feat(model)` `c46e5dd` | Migration 00008 (Serienbaum-Spalten, `tvdb_id`, `extra_kind`, TMDB-Bildpfade, Trailer-Zustand, `probe_error`, `ignored_files`, `trailers`, `people`, `credits`; Container-Trigger zählen keine Extras), Vertrag 0.4.0 mit 14 neuen Operationen, gemeinsame Modelltypen |
| 1 Serien, Staffeln, Folgen | `feat(library)` `adfaf1d`, `ceb58ba` | Serien-Parser (93 Pfade, 100 %, 12 NFO-Fälle: Sonarr, Plex, Scene, Tages-Shows, Specials, Mehrfachfolgen), Serien-Scan in Batches (Voll- und Pfad-Scan), Extras als Items, Scan-Abbruch, ignorierte Dateien, `probe_error`, Sweep leerer Staffeln und Serien |
| 1b Metadaten | `feat(metadata)` `6a1893c` | `Provider`-Schnittstelle, TMDB für Serien (Staffel-Abfragen, Credits, Bildpfade, `content_ratings` nach AN-8), TVDB v4 optional mit eigenem Schlüssel, `image.fetch` ohne zweiten Titelabruf |
| 2 Versionen, Extras | `feat(visibility,api)` `153dbc7`, `test(testdata)` `c5c88da` | Versionen mit Label aus Name oder Auflösung (Hauptversion zuerst), Extras am Film, an der Serie oder Staffel, Serienbaum und Personen in der scoped API, Testmedien (720p-Version, Extras-Ordner, Specials, Doppelfolge, kaputte Datei, Duplikat, Titel ohne Jahr) |
| 3 Zuordnung, Trailer | `feat(metadata)` `900aeaf`, `0c30d57`, `feat(admin,api)` `fd3b8d5` | Trailer-Kette TMDB → YouTube (Tageskontingent `ratelimit.Quota`) → KI (Schnittstelle, aus), manuelle Zuordnung mit gesperrten Provider-IDs, Aktualisieren, manuelle Trailer, Redaction für `key`/`apikey`/TVDB/YouTube, Konfiguration `SLOUCHCAT_TVDB_API_KEY`, `SLOUCHCAT_TVDB_PIN`, `SLOUCHCAT_YOUTUBE_API_KEY`, `SLOUCHCAT_TRAILER_AI` |
| 4 Pflegewerkzeuge | in `fd3b8d5`, `test(api,store)` `bee4e14` | Listen `unmatched`, `no_images`, `unrated`, `orphaned`, `unplayable`, `rating_mismatch`, Duplikate (Provider-ID, Titel+Jahr, Dateihash), Zusammenführen, Bereinigen, Massenfreigabe, Scan-Abbruch; Vertragstest über alle 55 Operationen, eingefrorener Stand `testdata/db/M3.sql` |
| Web | `feat(web)` `62cf6cf`, `43a253a` | Detailseite je Art (Serie mit Staffelwahl und Folgenliste, Staffel, Folge mit «Nächste Folge», Extra), Versionen, Extras, Besetzung, Trailer-Dialog (`youtube-nocookie.com`, offline ausgeblendet), Personen-Seite, Player mit Versionsmenü und Stream-Info, `/admin/maintenance`, `/admin/items/$id`, Scan abbrechen; neue UI-Primitive mit Stories |
| Verdrahtung | `feat(server)` `db380ea` | `main.go`: Schlüsselquelle, Pflege- und Metadaten-Setter, Prüfung ausstehender Metadaten nach dem Scan |
| 5 Lasttest | `perf(library)` `e1ba3f2`, `perf(jobs)` `563e8e7`, `docs(perf)` `7d8ce53` | `slouchcat scanbench`, Claim-Index (Migration 00009) und Zurückstellen abgelaufener Leases (Claim 14.9 ms → 0.65 ms bei 50'000 wartenden Jobs), `docs/perf/M3.md` |
| Review-Korrekturen | `fix(web)` `928a78a`, `fix(store,visibility,admin)` `618b315`, `fix(server)` `5100421` | siehe unten; Migration 00010 (Container-Mindeststufe über Staffeln, Extras nur mit sichtbarem Elternteil) |

### Definition of Done (Plan M3 plus 6.2) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| B-1 Bibliothek Film und Serie, `/media` read-only | PASS (seit M1a); Serienbibliotheken werden jetzt gescannt, `/admin/libraries` mit Fortschritt und Abbruch | `web/e2e/admin-libraries.spec.ts`, `go test ./internal/library -run 'TestSeriesScanBuildsTree|TestCancelScan'` |
| B-2 Namenskorpus ≥ 150 Pfade + 20 NFO-Fälle, ≥ 98 % | PASS: 184 Filmpfade (100 %) + 93 Serienpfade (100 %), 30 Film-NFO + 12 Serien-NFO-Fälle | `go test ./internal/library -run 'TestParseMoviePathCorpus|TestParseSeriesPathCorpus' -v` druckt die Quote |
| B-3 Texte in Profilsprache, Fallback en; Poster für 25 Referenzfilme | PASS: 25 von 25 Referenzfilmen gegen das echte TMDB erkannt, mit Poster, Hintergrund, Freigabe und Besetzung; Serien, Staffeln und Folgen erhalten de/en-Texte aus den Staffelabfragen | `docs/perf/M3.md` Abschnitt 2, `go test ./internal/metadata -run TestMatchSeriesTMDB`, `go test ./internal/api -run TestContractProfileLanguage` (Filme) |
| B-4 Änderungen sichtbar < 60 s | PASS (seit M1a); Pfad-Scan deckt Serienbibliotheken ab | `go test ./internal/library -run TestSeriesPathScanNewSeason` |
| B-5 4K und 1080p als ein Titel; Versionswahl im Player, Stream-Info zeigt `media_file_id` | PASS: zwei Dateien desselben Films → ein Titel mit zwei Versionen, Wahl «720p» → Player streamt die gewählte Datei, Stream-Info zeigt die Datei-ID | `web/e2e/versions.spec.ts` (Chromium, Firefox), `go test ./internal/visibility -run TestReaderVersionsPrimaryFirst`, `go test ./internal/library -run TestSecondVersionAttachesToSameItem` |
| B-6 manuelle Zuordnung übersteht 3 Voll-Scans und einen Refresh | PASS: Provider-IDs gesperrt, Dateien über Pfad/Inode/Hash gebunden; laufender Match-Job mit altem Kandidaten schreibt nichts (Review 5) | `go test ./internal/admin -run TestSetItemMatchLocksFields`, `go test ./internal/metadata -run 'TestAssignMatchAndRefresh|TestMatchWritesNothingAfterAManualMatchMeanwhile|TestSeasonWriteChecksTheSeriesSnapshot'`, `go test ./internal/library -run TestManualMatchSurvivesScansAndRefresh` |
| B-7 Extras aus `Trailers/`, `Featurettes/`, `Deleted Scenes/`, `Behind The Scenes/`, Suffix `-trailer` | PASS: fünf Extras am Titel, kein eigener Titel auf der Startseite | `web/e2e/extras.spec.ts`, `go test ./internal/library -run TestExtrasBecomeExtraItemsAndSamplesAreSkipped` |
| B-8 ≥ 90 % von 25 Referenzfilmen mit automatischem Trailer; de und en verschieden; manueller Ersatz übersteht Refresh | PASS: 25 von 25 mit je einem deutschen und einem englischen Trailer (alle Video-IDs je Sprache verschieden), Profilsprache zuerst; manuelle Zeilen bleiben beim Refresh | `docs/perf/M3.md` Abschnitt 2, `go test ./internal/metadata -run TestTrailersRefreshKeepsManualRows`, `web/e2e/trailer.spec.ts` |
| B-9 je präpariertem Fehlerfall ein Eintrag und Bereinigung nach Bestätigung | PASS: Duplikat (Dateihash), ohne Metadaten, ohne Bild, ohne Freigabe, verwaist, nicht abspielbar (ffprobe-Fehler) je in der Liste; Bereinigung mit Bestätigung, Massenfreigabe, Zusammenführen | `web/e2e/maintenance.spec.ts` (über die Oberfläche: nicht abspielbar, Duplikat, ohne Freigabe mit Massenzuweisung; Entfernen mit Bestätigung), `go test ./internal/admin -run 'TestAdminMaintenance'` (alle sechs Arten, Zusammenführen, Bereinigen) |
| W-10 Trailer-Knopf startet YouTube-IFrame (`youtube-nocookie.com`), offline ausgeblendet | PASS | `web/e2e/trailer.spec.ts` (iframe-URL, Escape, `context.setOffline`) |
| Lasttest ≥ 45'000 Dateien: keine doppelt ausgeführten Jobs, Startseite < 300 ms p95 | PASS: 60'800 Dateien / 8'212 Ordner; lokal Erst-Scan 33 s, Queue leer nach 47 s, 0 doppelte Jobs, Home p95 99.5 ms (Median aus 3); über SMB (CT 190) 10.4 min, p95 63 ms; im CT 190 (2 vCPU) 73 s, p95 249 ms; Abbruch nach 10 s mit Fortschritt, Wiederaufnahme konvergiert | `docs/perf/M3.md` Abschnitt 1, `server/bin/slouchcat scanbench` |
| Kinderprofil-Vertragstest 10.2 über alle neuen Operationen | PASS: 55 Operationen (`scoped` 13, `admin` 24, `account` 11, `public` 7), Fixture mit Serie, Specials, Doppelfolge, Extras, Versionen, Trailern de/en, Person mit Rollen in FSK-6- und FSK-18-Film, ignorierter Datei; Personen-Seite 404 ohne sichtbare Rolle | `go test ./internal/api -run 'TestContract'` |
| Negativtest: Serie 12 → 16 macht Folgen sofort unsichtbar | PASS: Staffeln, Folgen, Special, Doppelfolge und Extras der Serie für P12 aus Listen und per ID 404, Trailer 404; zurück auf 12 wieder sichtbar; Serie, deren Folgen alle über der Stufe liegen, unsichtbar (Review-Befund 1, Migration 00010) | `go test ./internal/api -run 'TestContractRatingChange|TestContractSeriesHiddenWhenEpisodesAbove|TestContractExtrasNeverContainers'` |
| `task ci` grün (6.2 Pkt. 2) | exit 0 (eigener Lauf, 20. Oktober 2026): gen, gen:check, lint (0 issues), Go-Tests mit `-race`, Vitest (web 168 in 27 Dateien, api-client 4, design-tokens 2), Build, Playwright 178 (Chromium 89, Firefox 89, 20 Specs), Storybook-Screenshots 95 | `task ci` (Storybook-Schritt in Claude-Shells über `sg docker -c`) |
| Tastaturtest je neue Seite (6.2 Pkt. 3) | PASS: Serie, Staffel, Folge, Extra, Trailer-Dialog, Versionen im Player, `/admin/maintenance`, `/admin/items/$id`; Personen-Seite nur im Not-Found-Zustand (ohne TMDB-Schlüssel gibt es im E2E keine Besetzung; die Seite mit Rollen ist in Vitest geprüft) | `web/e2e/series.spec.ts`, `episode.spec.ts`, `extras.spec.ts`, `person.spec.ts`, `trailer.spec.ts`, `versions.spec.ts`, `maintenance.spec.ts`, `admin-item.spec.ts` |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M3.sql` (Serienbaum mit Specials und Doppelfolge, Extras, zwei Versionen, Trailer, Personen, manuelle Zuordnung, ignorierte Datei); M1a, M1b, M2 und M3 migrieren bis 00010; `M3.sql` hält den Stand bei 00008 fest, 00010 rechnet die Container beim Laden neu (`TestMigration00010RecomputesContainers`) | `go test ./internal/store -run 'TestFrozen|TestMigration0000[8]|TestMigration00010'` |
| Messwerte (6.2 Pkt. 5) | `docs/perf/M3.md` (Scan lokal Median aus 3, SMB und Container je ein Lauf, Referenzfilme); Gerätecheckliste Abschnitt 3: Desktop Chromium und Firefox bestanden, Android ausstehend (Marco), Apple ungeprüft | `docs/perf/M3.md` |
| Code-Review (separate Passage, Opus) | Server 22 Befunde (1 kritisch, 5 wichtig, 16 klein), Web 23 Befunde (0 kritisch, 4 wichtig, 19 klein); alle behoben oder begründet akzeptiert (siehe unten) | Commits `928a78a`, `618b315`, `5100421` |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen» (Gerätecheckliste Android/Apple; Läufe gegen TMDB, CT 190, YouTube und TVDB nicht automatisch reproduzierbar, TMDB-Lauf an der Rohdatenbank bestätigt; Personen-Seite mit Rollen nur in Vitest; B-9 im E2E teilweise, Rest in Go-Tests); eigene Läufe: `task ci` exit 0 (ci5), alle zitierten Go-Tests grün, Vertragstests 23 PASS, Namenskorpus 184/184 und 93/93, `scanbench` 60'800 Dateien (Queue leer nach 48.1 s, 0 doppelte Jobs, Home p95 115.6 ms), kein Schlüsselwert im Baum oder in der Historie, kein «ß»; acht Berichtskorrekturen übernommen (Testpakete, B-3-Beleg, Anfragenzahl in M3.md, Gerätecheckliste, DB-Stand, offene Punkte) | Verifier-Bericht (Sitzungsnotizen `m3-verifier.md`) |

### Befunde der Review-Passage und ihre Behebung

- **Kritisch (Server) – Serie blieb sichtbar, wenn alle Folgen über der Profilstufe lagen:** die
  Mindeststufe der Serie las nur die Stufe der Staffeln (immer die eigene), nie deren Mindeststufe.
  Migration 00010 rechnet Container-Kinder mit `max(effective, min_child)`; `VerifyRatings` und das
  Orakel der Fixture folgen derselben Formel; Vertragstest ergänzt.
- **Wichtig (Server):** Extras eines unsichtbaren, nicht verfügbaren oder gelöschten Elterntitels blieben
  per ID erreichbar (View `visible_items` verlangt jetzt einen sichtbaren Elternteil; der Scanner hängt
  Extras nicht mehr an soft-gelöschte Filme); `QueuePendingTrailers` wurde nie aufgerufen (jetzt nach jedem
  Voll-Scan, dazu ein einmaliger Match für Titel aus M1/M2 ohne Bildpfade); Umgebungsfehler (ffprobe
  fehlt, I/O-Fehler) wurden als dauerhaftes `probe_error` gespeichert (jetzt nur Datei-Fehler, sonst
  Retry; Datei nach `missing_since` wird neu geprobt); eine manuelle Zuordnung ging verloren, wenn ein
  Match-Job gerade lief (Job vergleicht Provider-IDs und Sperren mit seinem Schnappschuss und schreibt sonst
  nichts); Serien zerfielen in Duplikate, wenn das TMDB-Jahr vom Ordnerjahr abwich (Index bevorzugt den
  Besitzer des Ordners).
- **Klein (Server):** Bereinigen und Zusammenführen in Batches ≤ 500, Stream-Recheck nach Massenfreigabe,
  Merge und Cleanup (auch nach Teilfehler), Offset-Überlauf, gemeinsames `internal/youtubeid`, höhere
  Priorität für wartende Match-Jobs bei Zuordnung/Refresh, Folgen in «unmatched»/«unrated» nur bei
  gematchter Serie, Hash-Duplikate ohne Kleindateien und mit 200-Gruppen-Deckel, elternlose Extras nur
  einmal gehasht, Warnung je Ordner statt je Datei, `RestoreItem` nur mit vorhandener Datei, Stream-Recheck
  nach Freigabe-Änderungen durch Metadaten-Jobs (`WithVisibilityRecheck`), TVDB-Schlüssel und PIN aus
  einer Quelle. Akzeptiert: Versionslabel leer, solange die Datei nicht geprobt ist; Jobs im Backoff
  werden je Claim mitgelesen (Beobachtung).
- **Wichtig (Web):** Massenfreigabe hatte FSK 0 vorgewählt (jetzt keine Vorauswahl, Knopf gesperrt, Text
  nennt Anzahl und Stufe); Versionsmenü im Vollbild unsichtbar (Popover im Player-Container); Trailer-Dialog
  öffnete sich nach Rückkehr der Verbindung erneut (offline schliesst ihn und setzt den Fokus zurück);
  Backspace im Dialog verliess die Seite (Zurück-Tasten ignorieren Dialoge, Dialog schliesst auf Backspace).
- **Klein (Web):** Versionswechsel wartet auf das Ende der alten Sitzung, sperrt den alten Player und
  schreibt `?file=` in die URL; Speichermeldungen bleiben stehen; wiederholte Suche lädt neu; externe
  Kandidatenposter ohne Referrer; Sammel-Refresh mit Teilfehler-Hinweis; Bestätigungen nennen die Anzahl;
  Pluralformen; tote Schlüssel entfernt; Trenner und Transcode-Gründe aus den Locales; `ui/FactList` und
  `ui/CandidateCard` statt dreifacher Implementierung; E2E ohne `waitForTimeout`; Steuerleiste bleibt bei
  offenem Menü sichtbar. Akzeptiert: keine CSP (Server, später), «Alle auswählen» wählt nur die geladene Seite.

### Abweichungen und Entscheide während M3

- **Parallele Umsetzung in Worktrees:** die vier Agenten-Branches wurden per Cherry-Pick in den M3-Branch
  übernommen; Zwischenstände einzelner Commits bauen nicht für sich allein (z. B. Vertragstest ohne
  Registry-Einträge zwischen `4404f80` und `bee4e14`); Historie nicht umgeschrieben.
- **Admin-Setter statt erweitertem `Libraries`-Interface:** `admin.WithLibraryMaintenance`, `admin.WithMetadata`
  wie `WithQueue`, damit jeder Branch für sich baute.
- **Serien-Match über Staffelabfragen:** ein Job je Serie füllt vorhandene Staffeln und Folgen (2 Aufrufe je
  Staffel statt 2 je Folge); Folgen-Jobs nur für bereits gematchte Serien; Staffeln und Extras bekommen
  keinen Job (`skipped`).
- **Suche ohne TMDB-Schlüssel** antwortet `422 tmdb_key_invalid`, TVDB ohne Schlüssel `provider_not_configured`.
- **Schlüssel nur aus der Umgebung** (keine Flags, damit sie nicht in Prozesslisten stehen); die verschlüsselten
  Einstellungen `tvdb_api_key`, `tvdb_pin`, `youtube_api_key` werden gelesen, die Einstellungsseite folgt in M8.
- **YouTube-Suche mit `videoDuration=short`** (eine Suche je Sprache), Filter 60–300 s danach.
- **Job-Claim:** Index `jobs (state, priority DESC, created_at, id)` und `RequeueExpiredLeases` vor dem Claim
  (Migration 00009); die Lease-Semantik aus ADR 0002 bleibt.
- **Vertragsbeobachtungen:** `ItemDetail.trailerCount` und `DuplicateGroup.items[].libraryId` sind im Schema
  optional, der Server setzt sie immer; der strict server validiert `minLength`/`pattern`/`maxItems` nicht,
  der Admin-Dienst prüft selbst.
- **Firefox in Playwright:** ein Tab nach dem letzten Element wickelt nicht zum Seitenanfang; Tastaturtests
  starten daher wie `detail.spec.ts` vom Seitenanfang; der Fokusring eines react-aria-Checkbox/Radio liegt am
  beschrifteten Wrapper (`data-focus-visible`), der Helfer `expectVisibleFocus` erkennt ihn.
- **Entscheid 3 («Erst-Scan darf Stunden dauern»):** die Referenzgrösse braucht lokal unter einer Minute,
  über SMB rund zehn Minuten, auf zwei Kernen gut anderthalb Minuten.
- **Spike-Seite und `SLOUCHCAT_SPIKE_DIR` bleiben** bis zum Ende von Marcos Android-Lauf (27. Oktober 2026).

### Nicht in M3 (Plan) und offene Punkte für M4

1. Lauf der YouTube-Stufe gegen die echte Data API, sobald `SLOUCHCAT_YOUTUBE_API_KEY` in `deploy/.env`
   steht (Marco; der im Chat genannte Schlüssel sollte neu erzeugt werden). Einstellungsseite für Schlüssel: M8.
2. TVDB nur gegen Mocks geprüft (kein Schlüssel); TVDB-Personen ohne TMDB-ID werden nicht gespeichert,
   Tages-Shows werden nicht nach Sendedatum gematcht.
3. Filme können bei abweichendem TMDB-Jahr beim Nachliefern einer zweiten Version noch zerfallen (Ordnerregel
   gilt für Serien); Merge-Werkzeug fängt es auf.
4. Content-Security-Policy für die Web-App (Server-Header), Autoplay der nächsten Folge, Qualitätsleiter,
   Spurwechsel: M4.
5. Nächtlicher `rating.verify`-Job und Dashboard: M8 (`admin.VerifyRatings` und die Pflegeliste
   `rating_mismatch` existieren).
6. Spike-Entscheid, K-8 mit echter Domain, Gerätecheckliste Android, NVIDIA-Laufzeit: unverändert aus M2.
7. Ungemessen bzw. ohne UI-Test: Dauer einer Massenfreigabe auf eine Serie mit tausenden Folgen (Trigger-Kaskade in
   einer Transaktion), Bestätigungsdialog «Zusammenführen» (nur Vitest), Pflegefälle «ohne Bild» und «verwaist» nur
   als Go-Tests; WebKit nicht gelaufen.

## Meilenstein M2 – Konten, Profile, Zugriff

Vorab beantwortet (Entscheidungsprotokoll, 19. Oktober 2026): Frage 5 (Grundsatzentscheide 5.1–5.3) und
Frage 6 (AN-1, AN-8, AN-11, AN-14), beide wie vom Plan vorgeschlagen. Alle fünf Schritte aus
`docs/planning/03-plan-v1.md` (M2) sind umgesetzt; der Entwurf steht in ADR 0005
(`docs/adr/0005-profilwahl-sitzung.md`). Commits mit Bereich im Titel:

| Schritt | Commit | Inhalt |
|---|---|---|
| Vorab | `docs(decisions)` `87a2758` | Antworten auf Frage 5 und 6 |
| 1 Datenmodell, Profilwahl, PIN | `feat(auth,api)` `68ce178` | Migration 00007 (Konto-Obergrenze und -Bibliotheken, `profiles.pin_hash`, `devices`, `pin_attempts`, `invitations`, `sessions` neu aufgebaut mit nullable `profile_id` und `device_id`, View `visible_items` mit `min(profile, account)`), Anmeldung ohne Profil, `selectProfile` mit PIN (argon2id) und Sperrkurve AN-11 je Profil und Gerät, Sichtbarkeitsstufe `account`, Profilverwaltung `/profiles` nur mit Hauptprofil, `admin` nur Hauptkonto plus Hauptprofil, Geräte-Cookie, `SLOUCHCAT_TRUSTED_PROXIES`, `Clear-Site-Data` und Bild-URLs mit `p=` (offener M1a-Punkt Bildcache) |
| 2 Sichtbarkeit, Vertragstest | `feat(visibility,admin)` `966291f` | System-Scope im `visibility`-Reader für die Freigabe-Übersteuerung `PUT /admin/items/{id}/rating` (Trigger vererben Serie → Staffel → Folge), Fixture mit Serienbaum und Gastkonto, Vertragstest über **alle 41 Operationen** (Registries für `scoped`, `account`, `admin`, `public`), Negativtests Freigabe-Anhebung, Gast-Entzug, Bibliotheksrecht, Profilsprache |
| 3 Einladungen, Gäste, Entzug | `feat(auth,api)` `65d9ba2` | Einladungslinks einmalig, 7 Tage, widerrufbar (nur SHA-256 gespeichert), Gastkonten mit Obergrenze und Bibliotheksliste, `GET/PATCH/DELETE /admin/guests`, Entzug beendet Streams, widerruft Sitzungen und Stream-Tokens und löscht das Konto |
| 4 Profilsprache, Akzent, Web | `feat(web)` `afd0dff` | Seiten `/profiles` («Wer schaut?» mit PIN-Dialog und Sperr-Countdown), `/settings/profiles` (Name, Stufe, Sprache, Akzent-Farbfelder, Bibliotheken, PIN), `/admin/access` (Einladungen, Gäste), `/invite/$token`; Profilwechsel wendet Sprache und Akzent ohne Neuladen an, leert Query- und Service-Worker-Bildcache; nur die erkannte Sprache im Start-Bundle (430 kB, unter M1b) |
| 5 HTTPS | `feat(deploy)` `8ad3d63` | Compose-Profil `https` mit Caddy 2.10 (`deploy/Caddyfile`, Let's Encrypt bzw. interne CA für `localhost`), `compose.https-only.yaml` (8096 nicht veröffentlicht), README-Abschnitte «HTTPS mit Caddy» und «Eigener Reverse-Proxy» (nginx, Traefik, Caddy) |
| Review-Korrekturen | `fix(server,deploy)` `a7261d1`, in `afd0dff` enthalten (Web) | Server 18 Befunde (1 kritisch, 4 wichtig, 13 klein), Web 20 Befunde (5 wichtig, 15 klein), siehe unten |

### Definition of Done (Plan M2 plus 6.2) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| K-1 Login E-Mail/Passwort, 401 bei falschem Passwort; Profil nur `name`, `image` optional | PASS; Schema-Test: `Profile`, `ProfileSummary`, `ProfileRequest`, `ProfileUpdate` ohne E-Mail-/Passwortfeld, `ProfileRequest` verlangt nur `name` | `go test ./internal/api -run 'TestLogin|TestContractProfileHasNoCredentials'` (401 in `auth_handlers_test.go`), `web/e2e/login.spec.ts` (Fehlermeldung) |
| K-2 Profile anlegen, ändern, löschen; Stufen 0/6/12/16/18; PIN setzen/entfernen; Bibliotheksrecht entziehen → unsichtbar | PASS | `web/e2e/profiles.spec.ts` (jede Stufe wählbar, PIN setzen und entfernen, Löschen mit Bestätigung, 11. Profil abgewiesen), `go test ./internal/api -run TestContractLibraryRightRevoked` |
| K-3 Vertragstest 10.2: 0 Lecks über alle Operationen, P0–P18 positiv und negativ | PASS: 41 Operationen (`account` 11, `admin` 12, `public` 7, `scoped` 11), Profile P0, P6, P12, P16, P18, PX (ohne Bibliothek X), Gastprofile PG und PG18 (Konto auf 12 gedeckelt), Sitzung ohne Profil; unsichtbar → 404; Operation ohne Registry-Eintrag lässt den Test fehlschlagen. Fixture: Filme und eine Serie; die weiteren Fixture-Teile aus 10.2 (Hörbuch, Podcast, Sammlung, Person, Extras, Untertitel, Seerr, SSE, Offline) folgen mit ihren Meilensteinen, die Operationen existieren noch nicht | `go test ./internal/api -run 'TestContractCoversEveryOperation|TestContractChildProfiles|TestContractAccountOperations|TestContractAdminOperations|TestContractPublicOperations'` |
| Negativtest: Serie von 12 auf 16 → Folgen für P12 sofort unsichtbar | PASS: Serie, Staffel, Folgen aus `/home` und per ID 404, laufender HLS-Stream einer Folge → 404 beim nächsten Segment, offene Range-Antwort wird beendet; zurück auf 12 → wieder sichtbar; Folge mit eigener Stufe 16 bleibt für P12 unsichtbar | `go test ./internal/api -run 'TestContractRatingChange'` |
| K-4 PIN beim Wechsel verlangt; Sperrkurve AN-11 mit Uhr-Mock | PASS: 3. Fehlversuch 30 s, dann 60 s, 120 s … bis 15 min, je Profil und Gerät der Sitzung, überlebt Neustarts; Obergrenze 20 Fehlversuche/h je Profil; Zähler verfällt nach 24 h | `go test ./internal/auth -run 'TestPIN|TestSelectProfile'`, `web/e2e/profiles.spec.ts` (Dialog, falsche PIN 403, Sperre 429 mit Countdown) |
| K-6 Einladung → Gastkonto → eigenes Profil; Gast 403 auf `admin`; nur freigegebene Bibliotheken; benutzter/abgelaufener Link → 410 | PASS | `web/e2e/invite.spec.ts`, `web/e2e/access.spec.ts`, `go test ./internal/auth -run TestInvitation`, `go test ./internal/api -run 'TestContractAdminOperations|TestInvitationFlow'` |
| K-7 Entzug → 401 auf allen Sitzungen und Stream-Tokens innert 1 s | PASS: Median 1.2 ms bis Cookie und Token 401 antworten, 1.2 ms bis die offene Range-Antwort geschlossen ist (Test bricht ab > 1 s) | `go test ./internal/api -run 'TestContractGuestRevocation' -count=5 -v`, `docs/perf/M2.md` 2 |
| K-8 HTTPS-Domain mit gültigem Zertifikat (Caddy-Profil); 10.2 ohne UI direkt gegen die API | TEILWEISE: lokal mit interner CA geprüft (HTTP/2 200 über Caddy, Login setzt Cookie mit `Secure`, Range 206, HTTP → 308); echte Domain mit Let's Encrypt nur durch Marco prüfbar; 10.2 läuft als Go-Test rein gegen die API | `task deploy:caddy:validate`, Übergabenotiz des Deploy-Agenten, `deploy/README.md` |
| C-3 Schlüsselvergleich de/en = 0 fehlende Schlüssel; Profilwechsel wechselt Sprache ohne Neuladen | PASS | Vitest `locales.test.ts`, `web/e2e/profiles.spec.ts` (de → en: Überschriften, `html lang` und Akzent ohne `reload`) |
| B-3 Titel/Beschreibung in Profilsprache, Fallback en | PASS: P6 (en) «Movie 06», P12 (de) «Film 12», Sprachwechsel wirkt ab der nächsten Anfrage; Fallback auf en im Unit-Test (der Vertragstest hat keinen Fallback-Fall) | `go test ./internal/api -run TestContractProfileLanguage`, `go test ./internal/visibility -run TestPickTextsFallback` |
| AN-8 Freigaben-Abbildung DE → AT → CH → GB → US inkl. US-TV | PASS (Tabelle aus Plan 5.7 vollständig getestet) | `go test ./internal/metadata -run TestFSK` |
| `task ci` grün (6.2 Pkt. 2) | exit 0 (eigener Lauf, 19. Oktober 2026) | gen, gen:check, lint (0 issues), Go-Tests mit `-race` (alle Pakete ok), Vitest (web 124 in 22 Dateien, api-client 4, design-tokens 2), Build, Playwright 102 (Chromium 51, Firefox 51, 12 Specs), Storybook-Screenshots 70 |
| Tastaturtest je neue Seite (6.2 Pkt. 3) | PASS: `/profiles`, `/settings/profiles`, `/admin/access`, `/invite/$token` (Pfeile, Tab, Enter, Escape, Zurück, sichtbarer Fokus, keine Fokusfalle) | `web/e2e/profiles.spec.ts`, `access.spec.ts`, `invite.spec.ts` |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M2.sql` (Hauptkonto mit drei Profilen, Gastkonto, Einladungen, Übersteuerungen; 92 Zeilen); M1a und M1b migrieren weiterhin, 00007 behält Sitzungen und Wiedergabesitzungen. Für M2.sql prüft `TestFrozenStatesMigrate` vorerst nur das Laden (00007 ist die jüngste Migration; aussagekräftig ab M3) | `go test ./internal/store -run 'TestFrozen|TestMigration00007'` |
| Messwerte und Gerätecheckliste (6.2 Pkt. 5) | `docs/perf/M2.md`: PIN-Prüfung 26.0 ms, Profilwahl mit PIN 26.8 ms, ohne 0.04 ms, Login 27.3 ms, Entzug → 401 1.2 ms; Gerätecheckliste Abschnitt 4: «Profilwechsel mit PIN» Desktop Chromium und Firefox bestanden, Android ausstehend (Marco), Apple ungeprüft | `go test ./internal/auth -bench .`, `docs/perf/M2.md` 4 |
| Code-Review (separate Passage, Opus) | Server 18 Befunde, Web 20 Befunde, alle behoben oder begründet akzeptiert (siehe unten) | Commits `a7261d1`, `afd0dff` |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen» (K-8 mit echter Domain und Let's Encrypt nur durch Marco prüfbar; Gerätecheckliste Android/iPhone/iPad/Safari ausstehend bzw. ungeprüft; WebKit ungeprüft); eigene Läufe: Vertragstests mit `-race` 17 PASS, Playwright `profiles/access/invite` (Chromium) 20 von 20, Vitest 124, `task gen` ohne Diff, Entzug → 401 in 0.7–1.8 ms, Benchmarks reproduziert; sechs Berichtskorrekturen übernommen (K-1-Schema-Test ergänzt, B-3-Fallback-Beleg, Gerätecheckliste in `docs/perf/M2.md`, DB-Hinweis, Fixture-Umfang, `account` in CLAUDE.md Regel 7) | Verifier-Bericht |

### Befunde der Review-Passage und ihre Behebung

- **Kritisch (Server) – PIN-Sperre per Cookie umgehbar:** der Zähler hing am Geräte-Cookie, das jede Anfrage
  neu setzen konnte. Jetzt zählt das Gerät der Sitzung (`sessions.device_id`, beim Anmelden festgelegt, nie
  geändert) plus eine Obergrenze je Profil über alle Geräte (20 Fehlversuche/h → 15 min).
- **Wichtig (Server):** Einladungs-Tokens standen im Request-Log und im Problem-`instance` (Pfadsegment wird
  jetzt redigiert, Test); Entzug lief in der Reihenfolge Streams → Löschen (jetzt Sitzungen widerrufen →
  Streams beenden → löschen → nochmals beenden; Registry kennt Konto und Profil); gesenkte Freigaben
  (Obergrenze, Bibliotheksrecht, Übersteuerung) beendeten offene Direktstreams nicht (`RecheckVisibility`);
  `private` als vertrauenswürdige Proxys ist mit veröffentlichtem 8096 unsicher (README, Limiter gedeckelt).
- **Klein (Server):** Profilzahl in der Transaktion, verschwundenes Profil → 404, Zählerverfall 24 h,
  Migration 00007 in einer Transaktion (Vorlage ergänzt), Übersteuerung löschen reiht Metadaten-Job ein,
  leere Bibliotheksauswahl → 422, Gastgrenze kappt Profile, Geräte/Zähler nach 400 Tagen aufgeräumt,
  Einladungsantwort `no-store`, HSTS im Caddyfile, Testlücken (Fremdprofile, Orakel der Fixture).
  Akzeptiert: `email already_exists` lässt Inhaber eines gültigen Links Adressen testen.
- **Wichtig (Web):** leere oder noch ladende Bibliotheksliste weitete Rechte beim Speichern auf «alle» aus
  (jetzt kein Formular ohne bekannte Liste, gespeicherte Rechte werden nie verbreitert); Service-Worker-Bildcache
  überlebte Logout und Wechsel (wird geleert); Bearbeiten des aktiven Profils leerte den Katalog-Cache nicht;
  PIN blieb im MutationCache (gcTime 0, reset); der Client rechnete `min(profile, account)` selbst (Regel 12;
  jetzt beide Werte angezeigt, nichts umgeschrieben).
- **Klein (Web):** Countdown ab dem ersten Render gesperrt, jedes 429 mit Pause, `auth`-Query überlebt den
  Wechsel, `autoComplete="one-time-code"`, Rückkehrziel `next` nochmals geprüft (eine echte offene
  Weiterleitung), MutationCache behandelt 401/`profile_required`, Sperrdauer für Screenreader, gemeinsame
  Helfer, tote Schlüssel, Kachel-Skeletons, Locale-Split (nur erkannte Sprache im Start-Bundle), Tests
  für Cache-Leerung und Bibliothekseinschränkung.

### Abweichungen und Entscheide während M2

- **Sichtbarkeitsstufe `account`** (ADR 0005): Sitzung ohne gewähltes Profil für Wähler und Profilverwaltung;
  `scoped` ohne Profil → `403 profile_required`; `admin` nur Hauptkonto **und** Hauptprofil.
- **Profilverwaltung nur mit aktivem Hauptprofil:** Alle Profile eines Haushalts teilen das Konto-Cookie; der
  Elternteil schützt sein Hauptprofil mit einer PIN (wie die Profilsperre bei Netflix). Gäste verwalten so ihre
  eigenen Profile.
- **Profilwechsel in place** (kein neues Cookie); die Wiedergabesitzungen der Sitzung werden widerrufen.
- **`GET /libraries`** (account) kam während der Umsetzung dazu: Gäste brauchen Bibliotheksnamen.
- **Freigabe-Übersteuerung** (`PUT /admin/items/{id}/rating`) ist in M2, weil der Negativtest 10.2 sie
  verlangt; sie sperrt `own_rating` in `locked_fields`. Löschen reiht `metadata.match` ein (füllt nur Filme;
  Serien bleiben bis M3 unbewertet).
- **`X-Forwarded-For` wird ohne `SLOUCHCAT_TRUSTED_PROXIES` ignoriert** (vorher geglaubt): Verhaltensänderung,
  im README beschrieben; `X-Forwarded-Proto` entscheidet weiter über `Secure`.
- **Profilbild:** `image` bleibt im Schema optional, ein Upload ist nicht gebaut (Avatar = Initiale auf Akzent).
- **Migrations-Vorlage:** DDL zwischen `BEGIN;` und `COMMIT;`, PRAGMAs ausserhalb (00007 ist die erste).
- **Zwischenstände der Server-Commits:** Vertrag und generierter Code liegen vollständig im Commit zu Schritt 1;
  das Go-Paket `api` baut erst mit dem Commit zu Schritt 3 wieder (strict-server-Interface). Historie nicht
  umgeschrieben.
- **E2E-Infrastruktur:** der Testserver läuft mit `SLOUCHCAT_TRUSTED_PROXIES=private`, jeder Test erhält per
  `X-Forwarded-For` eine eigene Adresse (10 Versuche/min je Adresse); die 30-s-Wiederfreigabe des
  PIN-Knopfs ist nur in Vitest mit Fake-Timern geprüft.
- **Spike-Seite und `SLOUCHCAT_SPIKE_DIR` bleiben** bis zum Ende von Marcos Android-Lauf (27. Oktober 2026).

### Nicht in M2 (Plan) und offene Punkte für M3

1. K-8 mit echter Domain und Let's Encrypt durch Marco prüfen (`task deploy:up:https`, README); HTTP/3 ungeprüft.
2. Spike-Entscheid endgültig machen, Spike-Seite und `SLOUCHCAT_SPIKE_DIR` entfernen (nach dem 27. Oktober).
3. Serien im Scanner und Metadaten (M3); bis dahin bleibt eine Serie mit gelöschter Übersteuerung unbewertet.
4. Profilbild-Upload (K-1 «optional ein Bild»): Schema vorhanden, Upload offen.
5. WebKit/Safari weiterhin ungeprüft (Entscheid 2); Gerätecheckliste «Profilwechsel mit PIN» am Android-Handy
   durch Marco.
6. NVIDIA-Container-Laufzeit, Qualitätsleiter, Admin-Dashboard: unverändert aus M1b.


## Meilenstein M1b – Durchstich Teil 2: HLS, Seeking, Design

Alle sechs Schritte aus `docs/planning/03-plan-v1.md` (M1b) sind umgesetzt; Entscheid 4 (AN-17) und die
GPU-Durchreichung in den Testcontainer stehen in `docs/planning/04-entscheidungen.md`. Commits mit Bereich im
Titel (ein Commit je Schritt, die Review-Korrekturen in einem eigenen):

| Schritt | Commit | Inhalt |
|---|---|---|
| 1 HLS, Seeking | `feat(transcode)` `72e5eef` | Paket `transcode`: `Detect` (NVENC, QSV, VAAPI, Software mit Test-Encode), `NewPlan` (Transcode 4-s-Raster, Remux auf Keyframe-Grenzen ≥ 4 s aus dem Index), `Manager`/`Session` mit Spulmodell (von der Platte, warten, oder ffmpeg am Zielsegment neu starten), fMP4 über den Segment-Muxer mit durchgehender Zeitachse (ADR 0003), `hvc1` für HEVC-Remux, Pipelines je Backend als Golden-Tests, `hlsbench` |
| 2 Sitzungen, Token | `feat(playback)` `9f45bea` | Migration 00005 `playback_sessions`, Stream-Token nach 5.4 (HMAC-SHA256, HKDF-Schlüssel aus `secret.key`, Ablauf und Widerruf in der Zeile), Entscheidung Direct/Remux/Transcode nach 10.3 aus Probe und Client-Fähigkeiten, HLS-Endpunkte als `http.Handler`, `DELETE /playback/sessions/{id}`, Logout widerruft, `Referrer-Policy: no-referrer`, Player mit hls.js (ADR 0004), Playwright `hls.spec.ts` und `stream-token.spec.ts`, zwei 90-s-Testclips |
| 3 Admission Control | `72e5eef`, `9f45bea`, Messung `docs/perf/M1b.md` 3 | Kosten nach Quelle und Pipeline (4/3/2/1.5/0.5/0.25), Budget je Backend, Warten bis 10 s dann 503 `transcode_busy`, FIFO, `nice 10`, `oom_score_adj 500`; Kalibrierung auf Entwicklungsrechner (NVENC, Software) und Testcontainer (QSV, Software) |
| 4 HDR | Messung `docs/perf/M1b.md` 4 | HDR10-4K → 1080p-SDR auf NVENC (CPU-Tonemapping, Ubuntu-ffmpeg ohne `tonemap_cuda`), QSV im Testcontainer (`vpp_qsv`) und Software auf 2 und 28 Kernen; Handy-Startzeit ausstehend (Marco) |
| 5 Bilder, Design | `feat(metadata)` `c685fdc`, `feat(web)` `b328782` | Bildpipeline über ffmpeg (Original behalten, Poster w200/400/800, Backdrop w780/1280/1920, WebP + JPEG, `Vary: Accept`, Akzentfarbe per `palettegen`), Migration 00006; Profil- und Titel-Akzent als CSS-Variablen, Poster-Glow, Backdrop-Tint, Fokusbewegung überall 180 ms (auch der Fokus-Scroll), Storybook 10 mit 54 Screenshot-Baselines in `task ci`, arm64-Image (Dockerfile-Fehler `TARGETARCH` behoben), CI-Job für beide Plattformen |
| 6 Spike, Rückfall | Doku | `docs/perf/M1-ios-offline.md` (Entscheid, Android-Zeilen ausstehend), Rückfallkriterium 2.5 unten |
| Review-Korrekturen | `fix(server,web)` | 23 Befunde der Review-Passage (1 kritisch, 6 wichtig, 16 klein), siehe unten |

### Definition of Done (Plan M1b plus 6.2) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| HEVC-Transcode startet in Firefox in < 5 s (1080p) | PASS: Firefox 284–338 ms, Chromium 373–376 ms (720p-Clip, NVENC); echter 1080p-HEVC-Film serverseitig 362 ms (NVENC) / 514 ms (Software), im eingebetteten Chromium 445 ms; ein Firefox-Wert mit 1080p-Film fehlt (Abstand zum Budget > 10×) | `web/e2e/hls.spec.ts`, `docs/perf/M1b.md` 1 |
| Spulen auf 20/50/80 % spielt in < 3 s weiter (Transcode und Remux) | PASS: langsamster Sprung 438 ms (Firefox, Software); Remux 30–254 ms; echter Film 557–960 ms | `docs/perf/M1b.md` 1 |
| Remux-HEVC spielt in Safari | **ungeprüft** (keine Hardware, Entscheid 2); `hvc1`-Tag und CODECS-String gesetzt, HEVC-Remux in Firefox 157 geprüft (Start 87 ms) | `docs/perf/M1b.md` 1 |
| HDR-Messwerte für NAS und Entwicklungsrechner | PASS: QSV 0.8 s erstes Segment (5.5× Echtzeit), NVENC 1.1 s, Software 28 Kerne 1.6 s, Software 2 Kerne 5.1 s und nicht echtzeitfähig (AN-17) | `docs/perf/M1b.md` 4 |
| Kalibrierte Kosten und Budgets | PASS: NVENC 12 (Treiberlimit 8 Sitzungen), QSV 8, Software Kerne/2, Remux 0.5 bestätigt | `docs/perf/M1b.md` 3 |
| Stream-Token nach Widerruf → 401 innert 1 s | PASS: 0–2 ms (Chromium, Firefox); laufende Range-Antworten werden beim Widerruf abgebrochen (Review I2) | `web/e2e/stream-token.spec.ts`, `go test ./internal/playback/ -run TestRevokeEndsOpenStream`, `go test ./internal/api/ -run TestStopEndsRunningRangeResponse` |
| Spike-Bericht mit Entscheid | TEILWEISE: Desktop-Ergebnisse und AN-20-Befund ausgewertet, Entscheid vorläufig; Android-Lauf durch Marco läuft (Tag 0 bis Tag 8) | `docs/perf/M1-ios-offline.md` |
| Images amd64 und arm64 starten | PASS: amd64 `healthy` 1.19–1.25 s; arm64 unter QEMU `healthz` nach 0.6–0.8 s, `healthy` 1.3–1.4 s; echte arm64-Hardware fehlt | `docs/perf/M1b.md` 6, `task deploy:healthy-time:arm64` |
| `task ci` grün (6.2 Pkt. 2) | exit 0 | gen, gen:check, lint (0 issues), Go-Tests mit `-race`, Vitest (web 68, api-client 4, design-tokens 2), Build, Playwright 62 (Chromium und Firefox), Storybook-Screenshots 55 |
| Kinderprofil-Vertragstest 10.2 | PASS für alle scoped-Operationen inkl. der vier HLS-Binärendpunkte und `stopPlayback` mit Token-Fixtures je Profil; unsichtbar → 404, fremdes/widerrufenes Token → 401 | `go test ./internal/api/ -run TestContractChildProfiles` |
| Tastaturtest je neue/geänderte Seite (6.2 Pkt. 3) | PASS: Player (`player.spec.ts`, `hls.spec.ts`), Startseite und Detail (unverändert grün) | `task e2e` |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M1b.sql` (10 Titel, Keyframe-Index, Fortschritt, beendete Wiedergabesitzung); M1a.sql migriert weiterhin | `go test ./internal/store/ -run TestFrozen` |
| Messwerte und Gerätecheckliste (6.2 Pkt. 5) | `docs/perf/M1b.md` Abschnitte 1–8 | |
| Code-Review (separate Passage, Opus) | 23 Befunde, alle behoben (siehe unten) | Commit `fix(server,web)` |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen» (Safari/iOS ungeprüft, Android-Spike und Handy-Startzeit ausstehend, arm64 nur unter QEMU, `tonemap_cuda` und VAAPI ungeprüft); eigene Läufe: `task ci` exit 0, Play bis erstes Segment 255 ms, Widerruf → 401 in 0 ms; zwei Berichtskorrekturen übernommen (Testbefehl, Firefox-1080p-Hinweis) | Verifier-Bericht, CI-Log |

### Befunde der Review-Passage und ihre Behebung

- **Kritisch – Segment-Vollständigkeit:** der Segment-Muxer von ffmpeg schreibt den Listeneintrag vor den
  Segmentdaten; die Sitzung hielt ein gelistetes Segment für fertig (beobachtet als `read … EOF` in der
  Kalibrierung, im schlimmsten Fall ein verkürztes Segment mit passender Content-Length). Jetzt gilt ein
  Segment erst als fertig, wenn seine Boxen genau am Dateiende schliessen (`moof` vorhanden, letzte Box
  `mdat`); unvollständige Dateien werden bis 2 s erneut geprüft.
- **Wichtig:** Remux ohne Keyframe-Index driftete bei langen GOPs (Index wird jetzt vor dem Start berechnet,
  20 s Limit, sonst Transcode mit `keyframes_pending` und Job mit höchster Priorität); Widerruf beendet
  auch laufende Range-Antworten (Kontext-Registry je Wiedergabe, Prüfung alle 5 s, Schreibdeadline);
  Wettlauf Stop/Start liess ffmpeg verwaist weiterlaufen (`ErrClosed` in `startLocked`, Manager-Kontext);
  ffmpeg lief bis zum Filmende voraus und ruhende Sitzungen behielten ihre Dateien (Pause nach 8 Segmenten
  Vorlauf, kalte Sitzungen nach 10 min, Sweep alle 60 s im Hintergrund); die Web-App stellte die
  Play-Anfrage bei Netzwechsel erneut (einmal je Mount, Stop je Sitzungs-ID); Start begann immer bei Segment
  0 statt an der Fortsetzungsposition.
- **Klein:** gemeinsame HDR-Regel (nur Transferkurve), I/O ausserhalb des Mutex, neueste Anfrage gewinnt
  (`ErrRestarted` → 503 `segment_restarted`), Fehlerbegrenzung je Startsegment, ENOENT → 404, Re-Check der
  Zeile nach der Zulassung, Wiederaufbau der Transcode-Sitzung nach Server-Neustart, Akzentfarbe bleibt bei
  Palettenfehler erhalten, Poster-Fallback auf M1a-w500 bis die Pipeline lief, gesperrte Poster werden aus
  dem Original skaliert, High-10/4:2:2 gehen in den Transcode, `Detect` mit 10-s-Deadline, FIFO-Zulassung,
  hls.js versucht eine Wiederherstellung, `--sc-item-accent` vererbt nicht mehr.
- **Offen gelassen (dokumentiert):** die Qualitätsleiter (nächstniedrigere Qualität bei SDR, M4); Remux
  nach einem Server-Neustart kopiert nur AAC-Audio, weil die Client-Fähigkeiten nicht gespeichert sind
  (M4 speichert Geräteprofile); `hvc1` bei HEVC mit nur In-Band-Parametersätzen ungeprüft (Safari fehlt);
  ein verspäteter Keyframe-Job rechnet den Index ein zweites Mal (harmlos); der Akzent-Retry-Zähler lebt im
  Speicher.

### Rückfallkriterium 2.5 (Ende M1b)

- **Signal 1 (Aufwand):** Ein getrenntes Aufgabenprotokoll mit Stunden gibt es nicht; die Commit-Zeiten und
  die Session-Notizen dienen als Protokoll. Mit `server` getaggter Aufwand (Wanduhr der Agenten-Sessions):
  M0 rund 1 h (18.10. 19:33–20:18), M1a rund 5 h (18.10. 21:33 – 19.10. 00:46 plus Nachträge 11:05–12:52),
  M1b rund 4 h (19.10. 13:55–17:30, vier parallele Ströme plus Review und Korrekturen). Zusammen unter einem
  Arbeitstag; die Schwelle von 26 Arbeitstagen ist um mehr als eine Grössenordnung unterschritten.
  **Signal 1: nein.**
- **Signal 2 (Ursache):** Der Review-Agent hat die 23 M1b-Befunde nach der Checkliste eingestuft: 5
  Go-spezifisch (alle Nebenläufigkeit: Stop/Start-Wettlauf, Mutex während I/O, konkurrierende Leser, ENOENT
  nach Schliessen, Zulassungsfenster), 18 nicht Go-spezifisch (ffmpeg-Verhalten, Fachlogik, Browser, CSS,
  Entwurf). Kein Befund aus Typsystem, Generics, fehlender Bibliothek, sqlc oder oapi-codegen. Alle Befunde
  wurden in **einer** Korrekturrunde behoben; keine Aufgabe brauchte mehr als zwei Runden. M1a: 12 Befunde,
  keiner Go-spezifisch, eine Runde. **Signal 2: nein.**
- **Folge:** kein Prototyp in Option A; Go bleibt (Eintrag im Entscheidungsprotokoll).

### Abweichungen und Entscheide während M1b

- **Segment-Muxer statt hls-Muxer** (ADR 0003): der hls-Muxer schneidet auf einem am ersten Paket nach dem
  Seek verankerten Raster; Remux-Grenzen wären nach einem Neustart anders als in der Playlist. Der
  Segment-Muxer mit `-copyts`, `frag_discont`, `use_editlist=0` und einer Zeitachse «Quelle + 10 s» liefert
  identische `tfdt` über Neustarts (Test ±2 ms).
- **hls.js auch für Desktop-Safari** (ADR 0004): Playwright-Chromium meldet `canPlayType(mpegurl)` «maybe»;
  der Player nutzt hls.js, wo das klassische `MediaSource` existiert, natives HLS nur auf dem iPhone
  (`ManagedMediaSource`). Folge: Desktop-Safari bekommt Tonemapping statt HDR-Durchleitung.
- **Firefox 157 dekodiert hier HEVC Main 10** (System-ffmpeg) und bekäme Remux; der Transcode-Test meldet
  deshalb nur H.264. Plan 10.3 nennt für Firefox «Transcode»; die Entscheidung folgt den gemeldeten
  Fähigkeiten, nicht dem Browsernamen.
- **NVENC auf der RTX 4070 ohne `tonemap_cuda`** (Ubuntu-ffmpeg 6.1.1): Tonemapping auf der CPU, NVDEC und
  NVENC auf der GPU. Das Image hat jellyfin-ffmpeg mit `tonemap_cuda`, aber die NVIDIA-Container-Laufzeit
  fehlt auf dem Entwicklungsrechner (sudo, `scripts/install-system-tools.sh` ergänzen).
- **GeForce-Sitzungslimit:** mehr als 8 gleichzeitige NVENC-Encoder scheitern; das Budget 12 hält das ein.
- **QSV im Testcontainer:** jellyfin-ffmpeg erkennt die UHD 630 über MediaSDK, `vpp_qsv` übernimmt
  Skalierung und Tonemapping; `Detect` protokolliert den gescheiterten NVENC-Versuch auf INFO.
- **Remux mit AAC-Encode** für DTS/AC3-Ton (Plan 10.3 Zeile 4, eigentlich M4) ist drin, weil die
  Alternative ein voller Video-Transcode wäre.
- **Storybook-Screenshots laufen im Playwright-Container** (`mcr.microsoft.com/playwright`), damit die
  Schriften auf Mint und dem CI-Runner gleich sind; einmaliger Pull 986 MB.
- **Dockerfile:** `ARG TARGETARCH=amd64` gewann über `--platform`; ein arm64-Build nahm amd64-Pakete. Behoben.
- **Zwei Agenten-Streams in gemeinsamen Dateien:** die Commits wurden mit `git hash-object`/`update-index`
  so geschnitten, dass jeder Commit baut (Prüfung in temporären Worktrees).
- **Spike-Seite und `SLOUCHCAT_SPIKE_DIR` bleiben**, bis Marcos Android-Lauf (Bestand nach 8 Tagen)
  abgeschlossen ist; Entfernen ist dann ein eigener kleiner Commit.
- **`SLOUCHCAT_HW`** (`auto|nvenc|qsv|vaapi|software`), **`SLOUCHCAT_TRANSCODE_BUDGET`** (0 = kalibrierter
  Standard), Transcode-Arbeitsbereich `<cache>/transcode` (beim Start geleert).

**Noch ausstehend (nicht gated):** Android-Lauf des Spikes (Tag 0, 1, 3, 8) und HLS-Startzeit am Handy
durch Marco; Nachtrag in `docs/perf/M1-ios-offline.md` und `docs/perf/M1b.md` 4 und 8.

## Offene Punkte für M2

1. Spike-Entscheid endgültig machen, Spike-Seite und `SLOUCHCAT_SPIKE_DIR` entfernen.
2. NVIDIA-Container-Laufzeit auf dem Entwicklungsrechner (sudo, Marco), damit `tonemap_cuda` im Image
   gemessen werden kann; VAAPI/AMD bleibt «best effort».
3. Qualitätsleiter und Geräteprofile (W-7) in M4; Safari/iOS bleiben ungeprüft bis Hardware da ist.
4. Admin-Oberfläche für Budgets und laufende Streams (W-6, M8); bis dahin Log und `SLOUCHCAT_TRANSCODE_BUDGET`.
5. Bildcache im Browser nach Logout/Profilwechsel (seit M1a offen, relevant ab M2).


## Meilenstein M1a – Durchstich Teil 1: Scan, Direct Play, Startseite

Alle fünf Schritte aus `docs/planning/03-plan-v1.md` (M1a) sind umgesetzt, Commits mit Bereich im Titel:

| Schritt | Commits | Inhalt |
|---|---|---|
| 1 Offline-Spike | `feat(spike)` | `testdata/generate-spike-media.sh` (zwei 1.54-GB-Dateien: fMP4 mit globalem `sidx`, `faststart`), Endpunkt `/api/v1/spike/{variant}` mit Range (nur mit `SLOUCHCAT_SPIKE_DIR`), Test-PWA unter `/spike/` mit beiden Schreibwegen (`createSyncAccessHandle` im Worker, `createWritable`), Wiederaufnahme, Wiedergabe per Blob-URL und per Service-Worker-Range, Spulmessung, Dauerlauf, Bestandsprüfung, Protokoll als JSON; Bericht `docs/perf/M1-ios-offline.md` |
| 2 Fundament | `feat(server)` ×2 | Vertrag mit allen Operationen und `x-visibility`; Migrationen 00002–00004 (Konten, Profile, Sitzungen, Bibliotheken, `items`/`media_files`/`tracks`/`images`, `progress`, `progress_events`), View `visible_items`, Trigger für `effective_rating`, `available`, `min_child_rating`; `secrets` (AES-256-GCM, `/config/secret.key`), `auth` (argon2id, Sitzungs-Cookie, SHA-256-Token), Assistent, Login, Session-Middleware (erzwingt `x-visibility` je Route aus dem eingebetteten Vertrag), CSRF-Header |
| 3 Scanner, Metadaten | `feat(server)` | Namensparser (184 Pfadmuster, 100 %), NFO, Datei-Identität (dev/inode/size, Teil-Hash), Scan in Batches ≤ 500 mit Löschschutz, Soft-Delete nach 24 h, Watcher (inotify lokal, adaptives Stat-Polling für NFS/SMB), Probe mit Keyframe-Index, TMDB de/en über `ratelimit` (20/s, 429-Backoff), AN-8-Freigaben, Bilder unverändert unter `/cache/images`, Auslieferung nur über `/items/{id}/images/...` mit Scope |
| 4 Startseite, Detail | `feat(server)`, `feat(web)` | `visibility` als einziger Lesepfad, Startseite (Karussell «Cat's Empfehlungen» als Platzhalter, «Weiterschauen», «Neu hinzugefügt», leere Reihen entfallen), Detailseite, Design-System-Komponenten in `web/src/ui`, Pfeiltasten-Navigation mit echtem DOM-Fokus, PNG-Icons, Service Worker |
| 5 Direct Play | `feat(server)`, `feat(web)` | Range-Stream über `paths` mit Sitzungs-Cookie, `POST /items/{id}/play`, Fortschritt alle 10 s/Pause/Ende/Verlassen, Fortsetzen, Player mit Tastatur; Assistent, Login, Admin-Bibliotheken als Seiten |

### Definition of Done (Plan M1a plus 6.2 und Abnahmematrix) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| Frischer Stack per Compose: Assistent → 20 generierte + 5 eigene Filme in < 5 min mit Postern | **9.5 s** (zweiter Lauf mit dem finalen Image: 10.5 s) für 20 generierte + 12 eigene (NFS) Titel, alle mit Poster und Backdrop; `healthy` nach 1.6–1.7 s | `docs/perf/M1a.md` Abschnitt 2a, Container-Log ohne Warnungen |
| Umbenennung einer Datei behält den Fortschritt | PASS | `go test ./internal/library/ -run TestScanRenameKeepsItemAndProgress`: Datei und Ordner umbenannt → gleiche `media_files.id`, gleiches Item, `progress`-Zeile bleibt |
| Abgehängte Bibliothekswurzel löscht nichts | PASS | `go test ./internal/library/ -run TestLossProtection`: fehlende Wurzel → `aborted/root_missing`, leere Wurzel → `root_empty`, 3 von 10 Dateien weg → `loss_over_threshold` ohne Änderung; nach `accept-loss` übernommen; 1 von 10 direkt |
| Direct Play startet in Chrome und Firefox in < 2 s | Chromium 43 ms, Firefox 73 ms (localhost, 10-s-Testclip); Safari **ungeprüft** (keine Hardware, Entscheid 2) | Playwright `web/e2e/player.spec.ts` liest `data-start-ms` |
| W-1: H.264/AAC-MP4 → 0 ffmpeg-Prozesse, HTTP 206 | PASS | Vertragstest prüft 206 mit `Content-Range`; im Compose-Container vor und nach `curl -r 0-1048575 …/stream` (206 in 1.7 ms) zeigt `/proc` nur den Prozess `slouchcat`, kein ffmpeg; `api` und `playback` importieren `transcode` nicht |
| Startseite LCP < 2.5 s (Lighthouse Mobile) | PASS: Median 1.80 s (1.802 / 1.802 / 1.802 s); Start 13.0 s → 3.38 s → 2.57 s → 2.55 s → 1.80 s; FCP 0.75 s, CLS 0 | `docs/perf/M1a.md` Abschnitt 4 (vier Runden: gzip, parallele Aufrufe, w780-Backdrop, Link-Preload, Hero-Bild in der Shell, JS 139 kB, Start-JS erst nach dem gezeichneten Hero-Bild) |
| Spike läuft | Testseite auf Desktop-Chrome und -Firefox durchgeprüft (Download beider Varianten und Schreibwege, Wiederaufnahme, Abbruch, Spulen < 100 ms per Blob-URL); Android-Lauf durch Marco steht aus; iOS ungeprüft | `docs/perf/M1-ios-offline.md` |
| Kinderprofil-Vertragstest 10.2 | PASS für alle 8 scoped-Operationen mit P0, P6, P12, P16, P18 und PX (ohne Bibliotheksrecht), positiv und negativ, ID-Endpunkte 404, fehlender und gelöschter Titel unsichtbar | `go test ./internal/api/ -run TestContractChildProfiles`; neue scoped-Operation ohne Registry-Eintrag lässt den Test fehlschlagen |
| K-1 Login E-Mail/Passwort, falsches Passwort → 401; Profil ohne E-Mail/Passwort | PASS | `go test ./internal/api/ -run 'TestSetupFlowEndToEnd|TestLoginFailures|TestSetupConflictsAndValidation'`; Schema `Profile` hat nur `name`, optional `image` |
| B-1 Bibliothek Typ Film und Serie anlegen; `/media` read-only | PASS | Assistent und `POST /admin/libraries` (beide Typen, `admin-libraries.spec.ts`); Schreibversuch im Container: `docker exec slouchcat touch /media/Generated/x` → `Read-only file system` (ebenso `/testfilme`) |
| B-2 Namenskorpus ≥ 150 Muster, ≥ 98 % | 184 Pfade, 100 %; 30 NFO-Fälle | `go test ./internal/library/ -run TestParseMoviePathCorpus -v` druckt die Quote |
| B-3 Titel/Beschreibung in Profilsprache, Fallback en; Poster für die Referenzfilme | PASS: 32 von 32 Titeln mit de/en-Texten und Postern | Compose-Lauf; `go test ./internal/visibility/` (Sprach-Fallback) |
| B-4 ext4: neu/umbenannt/gelöscht sichtbar < 60 s; Netzfreigaben nach AN-12 mit Messwert | inotify meldet in 2 s (inkl. Entprellung); **NFS in Referenzgrösse** (60'800 Dateien auf dem NAS): neuer Titel nach 48.4 s; **SMB in Referenzgrösse** (Proxmox-Testcontainer, Samba): nach parallelem Poller (16 Worker, 3 s je Durchlauf) und Pfad-Scan `scan.path` neuer Titel nach 33–37 s, sofern der CIFS-Client `dir_cache_timeout=0` setzt (Directory Leases, siehe `deploy/README.md`) | `slouchcat watchbench`, `docs/perf/M1a.md` Abschnitte 1, 1a, 1b |
| W-5 Fortsetzung ±10 s über zwei Kontexte | Server: `progress` je Profil und Item, `resumePositionMs` im `PlaybackInfo`; Playwright-Test mit zwei Browser-Kontexten (`web/e2e/resume.spec.ts`) | `task e2e`, `go test ./internal/playback/`, Vitest `PlayerPage.test.tsx` |
| E-1 Reihenfolge Karussell, Weiterschauen, Neu hinzugefügt | PASS | Vertragstest prüft Reihenfolge und entfallende leere Reihen |
| C-1 installierbar | TEILWEISE: Manifest mit PNG-Icons (192, 512, maskable) und Service Worker werden ausgeliefert; Lighthouse 12 hat keine Installierbarkeits-Prüfung mehr, die Installation wird mit dem Android-Lauf des Spikes manuell geprüft | `web/vite.config.ts`, `curl /manifest.webmanifest`, `/sw.js` |
| C-2 Lint 0 Verstösse; Admin und Zuschauer aus `web/src/ui` | PASS | `task lint` |
| C-4 Playwright-Tastatursuite je Seite | 54 Tests (6 Seiten plus W-5-Fortsetzung, Chromium und Firefox) grün; WebKit auf dem Rechner nicht lauffähig (fehlende Systembibliothek `libavif16`) | `task e2e` |
| A-1 frischer Stack: Konto, Bibliothek, TMDB-Schlüssel geprüft (ungültig → Meldung) | PASS; GPU-Erkennung folgt in M8 | Handler-Tests (`tmdb_key_invalid`, `tmdb_unreachable`, 409, 422 mit Feldcodes) |
| `task ci` grün (6.2 Pkt. 2) | exit 0 | gen, gen:check, lint (0 issues), Go-Tests mit `-race`, Vitest (48: web 42, api-client 4, design-tokens 2), Build, Playwright (54) |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M1a.sql`, Migrationstest grün | `go test ./internal/store/ -run TestFrozen` |
| Messwerte (6.2 Pkt. 5) | `docs/perf/M1a.md`, `docs/perf/M1-ios-offline.md` | |
| Lastmessung Dateiüberwachung ≥ 45'000 Dateien, 8'000 Ordner (lokale Platte) | 60'800 Dateien in 8'213 Ordnern: Stat-Durchlauf 128–193 ms, inotify 8'213 Watches in 0.4 s, Voll-Scan 2'500 Filme in 1.09 s; zusätzlich über NFS: 3.0–10.9 s je Durchlauf, Änderung nach 48 s sichtbar | `docs/perf/M1a.md` Abschnitte 1 und 1a |
| Code-Review (separate Passage, Opus) | 0 Sichtbarkeitslecks, 2 wichtige und 10 kleinere Befunde, alle behoben (siehe unten) | Commit `fix(server): harden login, scanner and progress after review` |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen» (damals LCP, Safari/iOS, Android-Spike, NFS-Lastmessung, Abspieltest); danach LCP mit Runde 4 erfüllt und NFS gemessen, Korrekturen am Bericht übernommen, W-5-Test ergänzt | `task ci` durch den Verifier erneut: exit 0 |

Nicht in M1a (Plan): Stream-Token (M1b), Bildpipeline mit Skalierung (M1b), HLS (M1b), Serien (M3),
Profile/PIN (M2), GPU-Erkennung im Assistenten (M8).

## Was läuft

```bash
task ci                      # alles bauen und prüfen (inkl. Playwright; Storybook braucht Docker)
server/bin/slouchcat scanbench --root DIR --workers 4 --json   # Lasttest des Scanners (M3)
testdata/hdr-quality.sh software|nvenc|qsv                     # P-6: HDR-Testbild durch die Server-Kette, SSIM und ΔE (M4)
CONTAINER=slouchcat-nvbench CONTAINER_MEDIA_DIR=/media/testdata VARIANT=cuda testdata/hdr-quality.sh nvenc   # P-6 im Container (tonemap_cuda)
server/bin/slouchcat hlsbench --file F --mode transcode --max-height 720 --hdr-tonemap auto|gpu|cpu --json   # W-3-Startzeit (M4)
SLOUCHCAT_REAL_SEASON=DIR go test ./internal/segments -run TestRealSeason -v   # Intro-Erkennung an einer echten Staffel (M4)
./testdata/generate-reference-movies.sh                        # 25 Referenzfilme als Platzhalter (B-3, B-8)
task server:dev              # Server auf :8096 mit .dev/config und .dev/cache
task web:dev                 # Vite auf :5173, Proxy /api → :8096
task e2e                     # Playwright gegen server/bin/slouchcat (nach task build und testdata:media)
task deploy:build            # Image slouchcat:dev
server/bin/slouchcat watchbench --root DIR --inotify   # Lastmessung der Dateiüberwachung
server/bin/slouchcat dump --config-dir DIR             # SQL-Stand für testdata/db
./testdata/generate-spike-media.sh                     # 2 × 1.5 GB für den Spike
```

Compose-Stack für den DoD-Lauf: `deploy/` mit `.env` (TMDB-Schlüssel optional, nie committen) und der
lokalen Override-Datei `.dev/compose.m1a.yaml` (Testfilme unter `/testfilme`, Spike unter `/spike`).

### Befunde der Review-Passage und ihre Behebung

- **Login konnte den Speicher erschöpfen** (argon2id 64 MiB je Versuch ohne Limit): jetzt höchstens 4
  gleichzeitige Hash-Läufe, 10 Versuche pro Minute je Client (429 `too_many_attempts`, `Retry-After`),
  überlange Eingaben werden vor dem Hashen abgewiesen, Request-Bodys sind auf 64 KiB begrenzt.
- **Ein Symlink aus der Wurzel heraus liess jeden Scan scheitern**: wird jetzt mit Warnung übersprungen.
- Sweep löscht erst, wenn alle Dateien eines Titels 24 h fehlen; Soft-Delete-Titel werden innert 30 Tagen
  per TMDB/IMDb-ID wieder verknüpft (Plan 3.2); Titel ohne Jahr und ID werden nur im selben Ordner
  zusammengeführt; verspätete Fortschrittsereignisse überschreiben die Position nicht mehr; das
  Sitzungs-Cookie gleitet mit dem Server-Ablauf; die 10.2-Fixture hat P12, P16, ein Profil ohne
  Bibliotheksrecht sowie einen fehlenden und einen gelöschten Titel.
- Offen gelassen (dokumentiert): ffprobe folgt Symlinks, die nach dem Scan ersetzt werden (nur Metadaten);
  Bild-Cache im Browser hält Bilder nach Logout/Profilwechsel (relevant ab M2); `X-Forwarded-For` wird
  ohne Proxy dem Client geglaubt (Semaphore begrenzt den Speicher trotzdem).

## Abweichungen und Entscheide während M1a

- **iOS/Safari ungeprüft** (Entscheid 2): Spike und Safari-Zeilen sind als «ungeprüft, keine Hardware»
  dokumentiert; WebKit in Playwright braucht `sudo pnpm exec playwright install-deps` (nicht ausgeführt).
- **Spike-Befund (Firefox):** fragmentiertes MP4 über Service-Worker-Range meldet eine instabile Dauer;
  per Blob-URL spielt es sauber, `faststart` über beide Wege. Geht in den M1b-Entscheid zu AN-20 ein.
- **Android braucht einen sicheren Kontext** für OPFS, Service Worker und `persist()`: über `http://IP:8096`
  nur mit Chrome-Flag `unsafely-treat-insecure-origin-as-secure` oder `adb reverse`; beschrieben im
  Spike-Protokoll. HTTPS kommt mit Caddy in M2.
- **Trigger ohne CTE:** SQLite erlaubt kein `WITH` in Triggern und ruft Trigger nicht rekursiv; die
  Vererbung ist auf drei Ebenen ausgeschrieben (Serie → Staffel → Folge → Extra), getestet.
- **Binär-Endpunkte** (`getItemImage`, `getItemStream`, `getSpikeFile`) sind in `oapi-codegen.yaml`
  ausgeschlossen und als `http.Handler` montiert; ihre `x-visibility` steht in `api.go` und wird gegen den
  Vertrag getestet.
- **`x-visibility` zur Laufzeit:** die Middleware liest den Marker je Route aus dem eingebetteten Vertrag
  (`gen.GetSpec`); Routen ohne Marker schliessen (401).
- **Karussell-Platzhalter:** ohne `top_picks` zeigt «Cat's Empfehlungen» die neuesten Titel mit Backdrop
  (max. 6); Kuratierung in M7.
- **`genres`** ist ein JSON-Objekt `{"de": [...], "en": [...]}`.
- **Ratenbegrenzer** ist ein eigener Token-Bucket (gleiches API wie `x/time/rate`).
- **Keyframe-Index** liest die ganze Datei (nur Demux) und läuft mit niedrigster Priorität nach den
  Metadaten; bei leeren Platzhalterdateien schlägt die Probe erwartungsgemäss fehl.
- **Commit `604f6f7`** enthält versehentlich das Entfernen der alten Startseite (Web baut in diesem
  Zwischenstand nicht); der Web-Commit stellt den Zustand her. Historie nicht umgeschrieben.
- **`SLOUCHCAT_WORKERS`** (Standard 2–4 nach CPU), `SLOUCHCAT_FFMPEG`/`FFPROBE`, `SLOUCHCAT_SPIKE_DIR`
  (temporär bis M1b).

**Abspieltest durch Marco** im Browser gegen den Compose-Stack (`http://192.168.1.143:8096`): bestanden,
19. Oktober 2026 («Scheint alles zu funktionieren»).

**Noch ausstehend:** Android-Lauf des Spikes (Protokoll in `docs/perf/M1-ios-offline.md`).

## Offene Punkte für M1b (Stand M1a, erledigt bzw. verschoben)

1. Lastmessung in Referenzgrösse **über NFS/SMB** (Platzhalterbaum auf der Freigabe oder Proxmox-CT,
   Entscheid 2/3); lokal erfüllt.
2. Android-Lauf des Spikes durch Marco (Download mit Sperre, Spulen, Bestand nach 1/3/8 Tagen); Auswertung
   und Go/No-Go in M1b.
3. Stream-Token (5.4), Bildpipeline (WebP, Grössen, Akzentfarbe), HLS/Remux, Design-Feinschliff, arm64.
4. `inotify_poll`-Abweichungen und Scan-Fehler erscheinen erst mit dem Dashboard (M8) in der Oberfläche;
   bis dahin Log und `/admin/libraries`.
5. Proxmox-Testcontainer als schwaches NAS (Entscheid 2, vor M1b): CT 190 `slouchcat-nas` läuft seit
   2026-10-19 auf `pve2` (Debian 13 amd64, 2 Kerne, 4 GB, Docker 29 mit Compose, Samba-Freigabe `media`,
   Platzhalterbaum mit 60'800 Dateien unter `/srv/media/placeholder-tree`), siehe `docs/dev/TESTSERVER.md`.
   Offen: `smbpasswd -a media` im Container (Marco), SMB-Lastmessung mit `watchbench` über CIFS
   (`docs/perf/M1a.md` 1b).
6. GitHub Actions: Workflow `.github/workflows/ci.yml` läuft seit dem Push dieses Branches; Ergebnis auf
   GitHub prüfen (der Lauf braucht ffmpeg, Playwright-Browser und `task testdata:media`, alles im Workflow).
