# Aktueller Stand

## Konzept 17 Stufe 1 «Stift, Seitenleiste, FSK, Name» (8. November 2026): Pull Request, Release 1.19.2

Patch ohne Vertragsänderung (Vertrag bleibt 1.19.0), keine Migration. Zweig `claude/detailseiten-stufe1` ab `main`
`75e49d71` (Release 1.19.1); Nummern und zwei Antworten von Marco im Entscheidungsprotokoll (Nachtrag zu Frage 24).
Stufe 2 wird Vertrag 1.20.0, Stufe 3 Vertrag 1.21.0 mit Migration 00040.

- **D-10 Umbenennung:** «Cat's Empfehlungen» heisst überall «User-Empfehlungen» (en «User picks»); nur Texte, Route
  `/picks` und Reihenschlüssel `picks` bleiben.
- **D-8 Seitenleiste B:** Profilkarte oben (Buchstabe in der Akzentfarbe, Name, Rolle, «Wechseln» zur Profilwahl),
  Symbole vor jedem Eintrag, Band «Meine Medien» immer offen (Schalter und Gerätespeicher
  `slouchcat.sidenav.libraries` entfallen), Bibliotheken mit Symbol nach Art und Zahl als Kapsel, Band «Konto» mit
  Wiedergabe, Profile, Verwaltung; wer was sieht, ist unverändert. Das Profil im Kopf bleibt, die Merkliste hat
  keine Zahl (Marcos Antworten). Nachtrag in Konzept 08 (N-2, 7.1).
- **D-7 Stift:** Die Vergrösserung hängt an der ganzen Zelle (Karte samt Stift, `PosterCard` `tool`), die Karte
  bleibt gross, wenn die Maus auf dem Stift ist; der Stift selbst wächst um das neue Token
  `--sc-tool-hover-scale` (1.18) und wird heller. Raster, Startseitenreihen, Spotlight-Reihe und Detailseite.
- **D-6 Punkt 4 / F-10 FSK weg:** keine FSK mehr auf Postern der Bibliothek und im Kopf von Detailseite und
  Spotlight (`ItemMeta`). Verwaltung, Altersregel, Filter «bis FSK», FSK-Filter der Suche und Sortierung `age`
  bleiben.
- Prüfung Stufe 1: `task check:fast` grün (158 Dateien, 1607 Tests), e2e `sidenav`, `home`, `admin-home`,
  `profiles`, `announcements`, `invite`, `admin-density`, `metadata-editor`, `library`, `detail` grün,
  `task web:storybook:update` 412 grün (Baselines neu). Code-Review: kein Blocker, Befunde behoben.
- Versionsstellen auf 1.19.2 (`deploy/.env.example`, `deploy/compose.yaml`, `deploy/compose.updater.yaml`,
  `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`).

## Release 1.19.1 (8. November 2026): Radiotitel mit weiteren Trennstrichen

Patch ohne Vertragsänderung (Vertrag bleibt 1.19.0), keine Migration. Zweig `claude/radio-title-dash` ab `main`
`85223f7a`. Fix und Versionsstellen in einem Pull Request.

- Befund aus der Abnahme von Konzept 16 Stufe 2 auf CT 190 (`docs/perf/radio.md` 4.1): Energy Zürich meldet Titel
  als «Rihanna ˗ Diamonds» mit U+02D7. `cleanTitle` trennte nur an « - » und « – », darum fehlten Interpret und
  Titel und damit jedes Cover.
- `cleanTitle` trennt jetzt auch an Geviertstrich (U+2014), Minuszeichen (U+2212) und U+02D7, jeweils zwischen
  Leerzeichen, am ersten im Text (`titleSeparators`). Test in `TestMetaCleanup`.
- Prüfung: `go test -race ./internal/radio/...` ok, `task server:lint` 0 issues, `task check:fast` grün.
- Versionsstellen auf 1.19.1 (`deploy/.env.example`, `deploy/compose.yaml`, `deploy/compose.updater.yaml`,
  `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`).

## Konzept 16 Stufe 2 «Webradio: Zurück und Cover» (8. November 2026): Pull Request

Zweig `claude/webradio-stufe2-09f306` ab `main` `344ebe33` (Release 1.18.2). **Vertrag 1.19.0, brechend für `getHome`
und `setHomeRows`** (Reihen-Schlüssel `radio`), **Migration 00039**. Bau auf Marcos Wort vom 8. November 2026 nach Plan
in drei Spuren (Relay und Cover, Regeln und eigene Sender, Web) mit Kritiker-Review des Plans (REVISE, alle Befunde
eingearbeitet), Code-Review und Security-Review (Befunde behoben). Entscheide im Nachtrag zu Frage 23 und in ADR 0025.

- **Zeitversatz:** Regler bis zur Pufferlänge (30 Minuten ab Werk) in «Jetzt läuft», Zustand «LIVE» oder «−12:30» im
  Mini-Player, Hinweis nach einer Pause über die Pufferlänge; ohne `tmpfs` kein Regler (`RadioListen.timeShiftSeconds`).
  Die Hörzeit (K-5) zählt jedes ausgelieferte Stück einmal, auch hinter Live.
- **Zeitleiste nach Ton-Position:** MP3 und AAC werden nach gezählten Rahmen gestempelt (Abweichung im Test unter 3 µs,
  Ziel 4 s); «Zuvor gespielt» aus derselben Zeitleiste.
- **Cover:** `StreamUrl`-Bild, MusicBrainz mit Cover Art Archive, iTunes-Suche (zwei Schalter), Cache `radio_tracks`
  (30 Tage Treffer, 24 Stunden Fehlschlag), Senderlogo als Rückfall; eigene Bildroute `getRadioCover`.
- **Startseite:** Reihe «Meine Sender» (ab Werk ausgeblendet am Ende), Kachel mit aktuellem Titel nur, solange der
  Sender läuft; Vorabmalerei, wenn sie zuoberst steht.
- **Verwaltung:** Reiter «Einordnung» (Regeln, Arbeitsliste ab 20 Sendern, «Ignorieren», Abdeckung), «Eigene Sender»
  (ohne Adressen im lokalen Netz, auch nicht über `.pls`/`.m3u`), Cover-Schalter; Radio-Hörer auf «Streams» aus
  `listRadioRelays` (`listStreams` unverändert).
- **Tests:** Migrations- und Stands-Test (`testdata/db/webradio2.sql`), Abnahmetest der Zeitleiste, Cover-Kette gegen
  Attrappen, Regeln über 40 000 Sender, Playwright-Tastaturtests (`radio-player`, `home-radio`, `admin-radio`,
  `admin-streams`), 18 neue Storybook-Baselines. Messwerte in `docs/perf/radio.md` Abschnitt 4.
- **Prüfung:** `task ci` auf `f675dab7`: Server (39 Pakete) grün, Web 1603 Vitest-Tests, Storybook 410 Geschichten,
  e2e in drei Shards 546 von 547 grün; der eine rote Test (`admin-folder-browser.spec.ts:151`, Fokus nach Esc, ohne
  Bezug zum Radio) lief danach mit `--repeat-each 3` 24 von 24 grün: ein Flake unter Last. Danach Versionsstellen auf
  1.19.0 (`deploy/.env.example`, `deploy/compose.yaml`, `deploy/compose.updater.yaml`, `deploy/README.md`,
  `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`) und `task check:fast` grün.
- **Offen für die Abnahme auf CT 190:** siehe `docs/perf/radio.md` Abschnitt 4; iPhone weiterhin ungetestet.

## Release 1.18.2 (8. November 2026): Radio-Verwaltung ohne Wartezeit

Patch ohne Vertragsänderung (Vertrag bleibt 1.18.0), keine Migration. Zweig `claude/radio-station-reload`
ab `main` `e2e36082` (Release 1.18.1). Fix und Versionsstellen in einem Pull Request.

- Befund aus der Abnahme von Konzept 16 Stufe 1 auf CT 190 (1.18.1, 55 646 Sender): die Seite «Sender» der
  Radio-Verwaltung brauchte 15 s, das Ändern eines Senders 0,6 s, eine Senderliste mit neuen Logos bis 20 s.
- Ursache: das Nachladen eines einzelnen Senders in den Index nutzte die Abfragen des ganzen Katalogs
  (`station_id = ? OR ? IS NULL`, Rangfolge über alle Streams), also rund 300 ms je Sender. Die
  Verwaltungsliste lädt 50 Sender einzeln, die Logo-Warteschlange jeden gelisteten Sender.
- Fix: eigene Abfragen für einen Sender über seine Schlüssel (`GetRadioStationForIndex`,
  `ListRadioStationFacetsByStation`, `ListRadioStationTags`, `ListRadioStationLocks`). Ein Test vergleicht
  jeden einzelnen Eintrag mit dem vollen Laden und prüft die Abfragepläne auf Tabellenscans.
- Messung (Entwicklungsrechner, 40 000 Sender, eine Verwaltungsseite mit 50 Sendern): vorher 4,8 s,
  nachher 13 ms (`BenchmarkAdminStationsLarge`).
- Versionsstellen auf 1.18.2 (`deploy/.env.example`, `deploy/compose.yaml`, `deploy/compose.updater.yaml`,
  `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`).

## Release 1.18.1 (8. November 2026): ganzer Radiokatalog

Patch ohne Vertragsänderung (Vertrag bleibt 1.18.0), keine Migration. Release ab `main` nach PR #116
(Merge `ddfc61ec`), Zweig `claude/release-1.18.1`. Release auf Marcos Wort vom 8. November 2026.

- Radio Browser antwortet auf `/json/stations` ohne `limit` nur mit den ersten 1000 Einträgen. CT 190 hatte
  mit 1.17.0 deshalb nur 986 Sender (`entries=998`), kaum Schweizer. `catalogStationsPath` fragt jetzt mit
  `limit=1000000`: rund 63 500 Sender, davon 661 mit `countrycode=CH`, etwa 75 MB. Der Testkatalog in
  `sync_test.go` lehnt eine Anfrage ohne `limit` ab.
- Bestehende Installationen bekommen die fehlenden Sender beim nächsten Abgleich (täglich ab 04:30 oder
  «Jetzt abgleichen» auf der Seite Radio).
- Versionsstellen auf 1.18.1 (`deploy/.env.example`, `deploy/compose.yaml`, `deploy/compose.updater.yaml`,
  `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`).

## Update, das immer durchgeht (Vertrag 1.18.0, 8. November 2026)

Zweig `claude/manifest-fallback` ab `main` `216ea50a` (Release 1.17.0). Vertrag **1.18.0, additiv**, keine
Migration. Entscheide im Nachtrag vom 8. November 2026 in ADR 0024.

Anlass: Am 8. November 2026 meldete die Update-Prüfung auf CT 190 «Letzte Prüfung fehlgeschlagen», weil
GitHub die Download-Adresse der Releases stundenlang mit 504 beantwortete (auch für fremde Projekte), während
API, Ablage der Anhänge und `ghcr.io` liefen. Marco verteilt Slouchcat weltweit und kann auf keinen Server
zugreifen; er hat entschieden, alle Lücken des Update-Wegs in einem Zug zu schliessen. Ein Architektur-Audit
fand dazu: Der Updater aktualisierte sich nie, Compose-Dateien reisten nie mit, nirgends gab es eine
Wiederholung, alte Images blieben liegen, Vorabversionen konnten «latest» werden.

- **Release-Dateien lesen** (`updater.FetchReleaseFile`): zwei Versuche, danach GitHub-API (latest und Tag),
  nur Anhänge desselben Repositorys; Fehler nennt beide Wege.
- **Selbstupdate des Updaters** über einen Hilfscontainer aus dem neuen Image; Rückkehr zur alten Version,
  wenn der neue nicht stabil läuft; höchstens ein Versuch je Version und 6 h.
- **Compose-Dateien reisen mit** (`deploy`, `deployKnown` im Manifest): ersetzt wird nur eine Datei, die seit
  einem Release unverändert ist, mit Sicherung und Rücknahme; eine selbst geänderte bleibt liegen.
- **Pull** bis dreimal, Grund in der Meldung, `disk_full`; **Aufräumen** alter Images ausser der Vorversion.
- **Release-Lauf**: nur die höchste `vX.Y.Z` wird «latest» und `:latest`; Vorabversionen bleiben Vorabversion.
- **Karte**: letzte erfolgreiche Prüfung, Version des Updaters, Warnung mit Befehl für Updater vor 1.18.0.
- **Neustartschleife**: Docker meldet einen Container zwischen zwei Neustarts als `running` und
  `restarting`; der Updater wartete deshalb die vollen 10 min. Jetzt gibt er nach drei Neustarts auf.

**Prüfung:** Zwei Reviews (Architektur-Audit, Code-Review mit Nachprüfung, APPROVE). `task ci` auf
`9c303e64`: Server grün; Web, Storybook (392), Build und e2e (537 in 3 Shards) grün; ein Radio-Test
(`useRadio.test.tsx` «shows the code of a start the server refused») fiel einmal unter Last und lief danach
fünfmal grün; er stammt aus dem Webradio und ist ein Flake. Danach `task check:fast` und gezielte Tests.

**Abnahme auf CT 190** (8. November 2026, mit Test-Images 1.17.90–1.17.93 aus einer lokalen Registry und
einem Test-Manifest über https mit eigener CA):
- Lauf 1: Update 1.17.90 → 1.17.91 in 3 s `done`; `compose.yaml` und `compose.updater.yaml` ersetzt (Sicherung
  `.bak-vor-1.17.91`), `compose.intel.yaml` gleich und unberührt; der Updater hob sich selbst auf 1.17.91, der
  Hilfscontainer entfernte sich; Aufräumen behielt Server 1.17.91 und 1.17.90.
- Lauf 2: eigene Änderung in `compose.yaml` blieb liegen, Meldung «has own changes»; Selbstupdate auf einen
  absichtlich kaputten Updater 1.17.92: nach 60 s zurück auf 1.17.91, keine Schleife.
- Lauf 3: Server 1.17.93 in der Neustartschleife → Rücknahme auf 1.17.92, `compose.yaml` wiederhergestellt;
  zuerst nach 10 min, mit dem Fix `93d00976` nach 11 s.
- Danach CT 190 zurück auf Release 1.17.0 mit Updater 1.16.0, Testteile entfernt; Sicherungen in
  `/root/abnahme118` auf dem CT.

**Für bestehende Installationen:** Ein Updater vor 1.18.0 hebt sich nicht selbst; einmal von Hand heben
(Befehl auf der Karte und in `docs/INSTALLATION.md`). Danach folgt er dem Server.

## Release 1.17.0 (8. November 2026): Webradio, Stufe 1

Release ab `main` nach PR #113 (Merge `a2598bea`, Konzept 16 Stufe 1). Vertrag **1.17.0, additiv**,
**Migration 00038**. Merge und Tag auf Marcos Wort vom 8. November 2026.

- Versionsstellen auf 1.17.0 im Zweig `claude/release-1.17.0` (`deploy/.env.example`,
  `deploy/compose.yaml`, `deploy/compose.updater.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`,
  `docs/dev/HANDOVER.md`).
- Die Compose-Datei hat neu eine `tmpfs` `/run/slouchcat/radio` (564 MB). Wer nur das Image hebt
  (Updater), behält die alte Compose-Datei: Radio spielt dann nur live, die Verwaltung warnt. CT 190
  bekommt die `tmpfs` von Hand.

## Konzept 16 Stufe 1 «Webradio: Hören» (7. November 2026): gemergt (PR #113)

Zweig `claude/webradio-stufe1` ab `main` `2bfa9979` (Release 1.16.1). **Vertrag 1.17.0 additiv,
Migration 00038**, Frage 23, ADR 0025. Bau auf Marcos Wort vom 7. November 2026, nach Plan in drei Spuren
(Relay, Katalog, Web) mit Kritiker-Review des Plans, Code-Review und Security-Review; alle Befunde behoben.

- **Katalog:** ganzer Radio-Browser-Katalog (Daten gemeinfrei), täglicher Abgleich `radio.catalog.sync`
  im Fenster ab 04:30, erster Lauf 10 Minuten nach dem Start bei leerem Katalog; stabile Kennung über
  `radio_station_aliases`; Einordnung in Land, Region, Sprache, Genre, Sendeart, Stimmung und Qualität
  über eingebettete Regeln (`server/internal/radio/rules/default.json`); Schutz gegen eine zu kurze Liste
  eines Spiegels.
- **Weiterleitung:** eigener Go-Leser (ICY, Shoutcast v1, HLS, Ogg-Titel) über `netguard`, ffmpeg nur
  zwischen `pipe:0` und `pipe:1` (AAC kopiert mit `+delay_moov`, sonst AAC 96–192 kbit/s), Stücke und
  Delta-Listen aus Go in die `tmpfs` `/run/slouchcat/radio`; ohne `tmpfs` nur live. Budget 512 MB,
  höchstens 10 Sender, 2 Minuten Nachlauf. Hör-Sitzungen mit Token je Sender (`rt`, geschwärzt), Hörzeit
  je Viertelstunde im Tageslimit (K-5), nicht in den Stream-Grenzen (W-8).
- **App:** Seite `/radio` (Variante A) mit Suche, Schnellfiltern, Filtern mit Zählern, Sortierung und
  Favoriten; Standardland je Profil aus der Zeitzone des Servers; Radio-Player auf App-Ebene mit
  Mini-Player und «Jetzt läuft». **Verwaltung** `/admin/radio` mit Reitern Übersicht, Sender,
  Einstellungen.
- **Container:** `deploy/compose.yaml` bekommt die `tmpfs` für die Puffer; ältere Compose-Dateien spielen
  nur live.
- **Tests:** Test-Sender und Testkatalog nur im Build `bin/slouchcat-e2e` (Tag `radiotest`); drei
  Playwright-Specs mit Tastaturprüfung; eingefrorener Stand `testdata/db/webradio.sql`.
- **Offen für die Abnahme auf CT 190:** Messungen nach `docs/perf/radio.md` (erster Ton, Liste und
  Suche, Abgleich, CPU und RAM je Relay), `+delay_moov` mit dem ffmpeg im Container, iPhone mit
  gesperrtem Bildschirm, Schweizer Sender trotz «gestört» bei Radio Browser (Prüfung aus Deutschland,
  ADR 0025 «Folgen»). Stufe 2 (Zeitversatz-Regler, Cover, Startseiten-Reihe, Regeln, eigene Sender) folgt
  danach.

## Release 1.16.1 (7. November 2026): Ruckler beim Remux

Patch ohne Vertragsänderung (Vertrag bleibt 1.16.0), keine Migration. Zweig `claude/remux-seams`.

Marco sah beim Schauen von Silo S03E09 (HEVC aus MKV, EAC3 zu AAC, Remux) gelegentliche Mikroruckler.
Diagnose an CT 190, unabhängige Zweitmeinung und eine Messung im Browser (Firefox,
`requestVideoFrameCallback`): Alle drei markierten Ruckler lagen 0,6–0,9 s nach einem ffmpeg-Neustart.

- **Ton-Lücke an jedem Neustart** (etwa 134 ms, beim normalen Schauen rund jede Minute wegen
  `MaxAhead`): Der Eingangs-Seek landete auf dem Keyframe der Grenze, der neue Lauf las keinen Ton davor.
  Jetzt sucht der Eingang den letzten Keyframe vor dem Ausgabe-Seek; der Ton überlappt die Naht.
- **Keyframe drei Frames zu früh** an fast jeder zweiten Grenze (mp4-Muxer, Paketdauer vor dem Keyframe
  zwei Frames): `setts` setzt die Paketdauern des kopierten Videos auf den Abstand zur nächsten
  Dekodierzeit; `-copytb 1` verhindert, dass ffmpeg 6.1 die Zeitstempel dabei rundet.
- Tests `TestHLSRemuxSeams`, `TestRemuxInputSeek`; Review APPROVE (Auflagen umgesetzt); Einzelheiten im Nachtrag von ADR 0003. Nachmessung mit den neuen Argumenten
  an der Originaldatei im Container (ffmpeg 7.1.4-Jellyfin): keine Muxer-Warnung, 40 von 40 Grenzen exakt,
  Naht 116 ms Überlappung statt 134 ms Lücke.
- Nebenbefund, nicht behoben: Das Herauslösen eingebetteter Text-Untertitel (`ffmpeg webvtt`) bricht bei
  grossen Dateien auf dem langsamen NAS nach 60 s ab (`signal: killed`, 500); bei Silo S03E09 fehlten so
  die deutschen Forced-Untertitel.
- Versionsstellen auf 1.16.1 (`deploy/.env.example`, `deploy/compose.yaml`, `deploy/compose.updater.yaml`,
  `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`).

## Release 1.16.0 (7. November 2026): veröffentlicht

Release ab `main` nach PR #108 (Konzept 15, Stufe 4). Vertrag **1.16.0, additiv**, keine Migration.
Neu ist das zweite Image `ghcr.io/slouchcat/slouchcat-updater` (Updater-Begleiter, ADR 0024).

- Versionsstellen auf 1.16.0 im Zweig `claude/release-1.16.0` (`deploy/.env.example`,
  `deploy/compose.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`),
  gemergt als **PR #109** (Merge `266415ba`).
- Tag `v1.16.0` von Marco auf `266415ba` gesetzt (7. November 2026, Release-Notes
  `release-notes-1.16.0.txt`). Release-Lauf `42385465442` grün, GitHub-Release um 16:18
  veröffentlicht. Das Manifest trägt erstmals das Feld `image`
  (`ghcr.io/slouchcat/slouchcat@sha256:84361004…`). Beide Images lassen sich ohne Anmeldung ziehen;
  das neue Paket `slouchcat-updater` war ohne Handgriff öffentlich.
- **CT 190 hat sich selbst auf 1.16.0 gehoben:** Updater auf sein Release-Image 1.16.0, Testdatei
  `compose.abnahme.yaml` entfernt, dann der Auftrag an den Updater (1.15.0 hat noch keinen Knopf).
  Echtes Manifest von GitHub, Digest geprüft, `done` nach 13 s; 1.16.0 schrieb `server.json` selbst
  (Schema 37, keine Migration). Sicherung `.env.bak-vor-1.16.0` (→ 1.15.0). `COMPOSE_FILE` ist jetzt
  `compose.yaml:compose.ct.yaml:compose.intel.yaml:compose.updater.yaml`; Port 8096 und GPU wie
  vorher, 0 ERROR. Testimages, Bau- und Storybook-Ordner der Abnahme, Build-Cache (2,1 GB), Zweige
  und Schritt-Klone der Stufe 4 sind aufgeräumt.

## Konzept 15 «Feinschliff», Stufe 4: gemergt und auf CT 190 abgenommen (7. November 2026)

Zweig `claude/feinschliff-stufe4` ab `main` `88d03c11` (Release 1.15.0 veröffentlicht). Geplant als
Release **1.16.0**: Vertrag **1.16.0, additiv**, **keine Migration**. Neue **ADR 0024
«Updater-Begleiter»**; Spec A-5 erweitert; `CLAUDE.md` Regel 3 mit der benannten Ausnahme
`server/cmd/updater` (Lint erlaubt dort nur `exec.CommandContext`).

- **P-8 Updater-Begleiter** `server/cmd/updater`, eigenes Image `ghcr.io/slouchcat/slouchcat-updater`
  (`deploy/Dockerfile.updater` auf `docker:29.9.3-cli`, per Digest gepinnt), freiwillig über die
  Override-Datei `deploy/compose.updater.yaml` (statt nur eines Profils: der Server braucht eine
  Einhängung und eine Variable). Nur der Updater hat `/var/run/docker.sock`; `deploy/` ist unter
  demselben Pfad eingehängt (`SLOUCHCAT_DEPLOY_DIR`), `./config` und `./cache` sind darin mit
  schreibgeschütztem tmpfs überdeckt. Kein offener Port, Bearer-Token in `deploy/updater/token`.
  Ablauf: Manifest (nur https) prüfen, neuere Version und Digest Pflicht, `.env` sichern
  (`.env.bak-vor-<X>`, `0600`, der Updater löscht nie eine Sicherung), nur den Wert `SLOUCHCAT_VERSION` ändern, Compose mit
  genau den Dateien aus den Labels des laufenden Containers (`config --format json`, `pull`,
  `up -d --no-deps --no-build`), Digest prüfen, bis 10 min selbst auf «healthy» warten (kein
  `--wait`: der Healthcheck meldet eine lange Migration nach 16 s als krank). Ohne Migration nimmt er
  ein gescheitertes Update zurück, mit Migration meldet er nur die Sicherung (F-14). Nach einem
  Stopp oder Neustart des Updaters macht er beim nächsten Start weiter, ohne zurückzunehmen.
- **Server und Vertrag:** `SLOUCHCAT_UPDATER_URL`; `server.json` vor und nach den Migrationen
  (Hooks in `store.Options.Schema`; ein zweiter Start derselben Version behält den ersten Stand und
  die erste Sicherung, die nie weggeräumt wird); `UpdateState.updaterConfigured` und
  `updaterAvailable`, `UpdateState.install` (`UpdateInstallStatus`); `POST /admin/update/install`
  (202, 409 `update_running`/`backup_busy`/`restart_pending`, 422 mit dem Grund des Updaters, 503
  `updater_unavailable`) und `GET /admin/update/status` (liest `state.json` direkt). Neustart,
  Sicherung und Wiederherstellung antworten während eines Updates 409 `update_running`. Ein
  gescheitertes oder zurückgenommenes Update meldet der Server einmal als Ereignis «Fehler»
  (Einstellung `update_install_reported`).
- **Karte «Server»:** «Update auf X installieren» mit Release-Notizen im Bestätigungsdialog, danach
  die Phasen (Herunterladen, Neustart, Gesundheitsprüfung), Neuladen, sobald die neue Version
  antwortet; Ergebnis bleibt sichtbar. Ohne Updater (oder bei `no_compose_labels`,
  `outside_deploy_dir`) die Schritte von Hand zum Kopieren (Rückfall C).
- **Release-Lauf:** baut, veröffentlicht und attestiert auch das Updater-Image; das Manifest bekommt
  das Feld `image` (`ghcr.io/slouchcat/slouchcat@sha256:…`). `ci.yml` baut das Updater-Image mit.
  Das neue Paket `slouchcat-updater` muss nach dem ersten Release einmal von Hand öffentlich
  gemacht werden (Kopf von `release.yml`).
- **Reviews** (OMC-Code-Reviewer und Verifier): Updater REQUEST CHANGES (2 High: `config --images`
  listete wegen `depends_on` zwei Images, jedes Update wäre gescheitert; `chown` über einen Pfad im
  vom Server beschreibbaren Ordner hätte einem übernommenen Server Root auf dem Gastgeber geben
  können. 4 Medium, 9 Low); Server und Karte REQUEST CHANGES (1 High: `server.json` verlor beim
  zweiten Start den Beweis der Migration. 5 Medium, 9 Low); Verifier: alle behoben, 6 weitere Low
  ebenfalls behoben. Ein versehentlich mitgeschriebenes Programm `server/updater` (11 MB) wurde vor
  dem Push aus allen Commits entfernt, `.gitignore` ergänzt.
- **Bildspeicher geprüft:** Nach `docker compose pull` zeigt `RepoDigests` den Index-Digest des
  Release-Laufs, mit dem containerd-Speicher und mit dem klassischen. Ein echtes Update 1.1.0 →
  1.15.0 lief lokal mit einem eigenen https-Manifest durch (`done` nach 12 s).

**Belege auf `b10945c1`** (Inhalt gleich `d1bb86ed`, vor den letzten sieben Low-Fixes): `task ci`
grün bis auf Storybook: 371 gleich, **1 neue Story ohne Vergleichsbild**
(`ui-confirmdialog--with-details`); Go alle Pakete ok, Vitest 1504, api-client 4, design-tokens 2.
e2e `task build` + `task ci:e2e:parallel`: **522/523**, `admin-picks.spec.ts:98` (Firefox) einzeln
grün. Nach den Low-Fixes (`aa3abad3`): gezielte Go-Tests, `-race -count=3 ./cmd/updater/`, Lint,
`task check:fast`, AdminHomePage-Vitest 56/56 und `task e2e:quick -- e2e/admin-home.spec.ts` 9/9 grün.

**Abnahme auf CT 190 (7. November 2026):** Vergleichsbild `ui-confirmdialog--with-details` auf CT 190
geschossen (`66c3703f`, 367 passed). Testimage aus `66c3703f` mit der Version **1.14.99** (damit 1.15.0
als neuer gilt) und Updater-Image `abnahme-66c3703`, eingerichtet mit `compose.updater.yaml` und einer
Testdatei `compose.abnahme.yaml` (Test-Manifest über https mit eigener CA, Version 1.15.0 und dem echten
Index-Digest von 1.15.0). Port 8096, `/dev/dri` und Gruppe 993 standen danach wie vorher; im Updater ist
`config` leer und schreibgeschützt. Marco hat zweimal auf «Update auf 1.15.0 installieren» geklickt:

- **Lauf 1, Rücknahme:** Herunterladen, Digest, Neustart in 2 s; 1.15.0 lief gesund, schrieb aber
  (als Release vor 1.16) kein `server.json`. Nach 10 min nahm der Updater das Update zurück
  (`rolled_back`), 1.14.99 lief wieder, der Server meldete einmal das Ereignis «Fehler».
- **Lauf 2, Erfolg:** dasselbe, `server.json` für 1.15.0 von Hand geschrieben, eine Sekunde später
  `done`; CT 190 läuft seither auf Release 1.15.0.
- **Befunde und Fixes:** Der Updater löschte alle `.env.bak-vor-*` ausser den fünf neuesten (auf CT 190
  rund 20 von Hand) und überschrieb eine gleichnamige Sicherung: er löscht und ersetzt jetzt keine
  Sicherung mehr, sondern wählt `-2`, `-3`, … (`242ca4de`, `2a3a68de`). Die Meldung nach der Wartezeit
  unterscheidet jetzt «läuft gesund, meldet sich aber nicht» von «wird nicht gesund» (`af3a6607`).

Marco hat Stufe 4 am 7. November 2026 freigegeben.

Die GitHub-CI fand auf dem Zweig einen Testfehler, der nur dort auftrat: In Firefox übernahm der
Service Worker die Seite mitten in `metadata-editor.spec.ts:539`, und die Test-Attrappe für
`/api/v1/home` griff nicht mehr. Die Testdatei blockiert jetzt den Service Worker wie zehn andere
(`1f9fe1fb`; CI-Teillauf 1/6 lokal 99/99). Gemergt als **PR #108** (Merge `a96e196f`, GitHub-CI 19/19
grün auf `1f9fe1fb`).

Veröffentlicht als Release 1.16.0 (siehe oben).

## Release 1.15.0 (7. November 2026): veröffentlicht

Release ab `main` nach PR #104 (Konzept 15, Stufe 3). Vertrag **1.15.0, brechend für `getHome` und
`setHomeRows`** (Web-App und Server kommen im selben Image), Migrationen
**`00035_home_rows_libraries.sql`**, **`00036_item_text_sort_title.sql`** und
**`00037_items_top_added.sql`**: der Weg zurück auf 1.14.0 geht nur über die Datenbanksicherung, die
der Server vor der Migration schreibt (`/config/backups/pre-migration-…`).

- Versionsstellen auf 1.15.0 im Zweig `claude/release-1.15.0` (`deploy/.env.example`,
  `deploy/compose.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`),
  gemergt als **PR #105** (Merge `d70be9b1`).
- Tag `v1.15.0` von Marco auf `d70be9b1` gesetzt (7. November 2026, Release-Notes
  `release-notes-1.15.0.txt`). Release-Lauf `42358123485` grün, GitHub-Release um 10:06
  veröffentlicht.
- CT 190 läuft auf Image `1.15.0` (Sicherung `.env.bak-vor-1.15.0`; die Migrationen 00035 bis 00037
  liefen dort schon bei den Testimages, DB-Sicherungen
  `pre-migration-v1.14.0-42-g3429ef3b-20261106T225800.002Z.db` und
  `pre-migration-v1.14.0-47-g8251b6cf-20261106T233749.740Z.db`). Zweige, Schritt-Klone und
  Zwischen-Images der Stufe 3 sind aufgeräumt.

## Konzept 15 «Feinschliff», Stufe 3: veröffentlicht als 1.15.0 (7. November 2026)

Zweig `claude/feinschliff-stufe3` ab `main` `d2361e54` (Release 1.14.0 veröffentlicht). Geplant als
Release **1.15.0**: Vertrag **1.15.0, brechend für `getHome` und `setHomeRows`** (Web-App und Server
kommen im selben Image), Migrationen **`00035_home_rows_libraries.sql`** und
**`00036_item_text_sort_title.sql`** (Nachtrag in Konzept 15, 7.5) und **`00037_items_top_added.sql`**
(Teilindex für die Startseite). Der Weg zurück auf 1.14.0 geht nur über die Datenbanksicherung vor der
Migration.

- **P-6 Metadaten bearbeiten:** Stift «Metadaten bearbeiten» auf der Detailseite von Film, Serie,
  Staffel und Folge (ersetzt «Verwalten»; Podcasts, Hörbücher und Extras behalten «Verwalten») und
  auf den Covern im Bibliotheksraster (zweiter Tab-Halt neben dem Link, nur mit Zeiger, nicht auf
  Geräten ohne Hover), nur für den Server-Administrator. Dialog mit vier Reitern: **Bilder**
  (Vorschläge mit Filter nach Anbieter und Sprache, eigenes Bild hochladen, «Zurück auf
  automatisch» entfernt auch ein hochgeladenes Bild; Folgenbild nur mit der Einstellung
  «Folgenbilder»), **Metadaten** (Titel, Originaltitel, Sortiertitel, Jahr, Datum, Slogan,
  Beschreibung, Genres, Freigabe, Originalsprache, je Feld Herkunft und Schloss), **Zuordnen**
  (bei Staffeln und Folgen ein Hinweis) und **Aktualisieren** (mit «Vorher alle Sperren aufheben»).
  Ungespeicherte Änderungen fragen vor dem Schliessen nach.
- **Server und Vertrag P-6:** Sperren je Sprache (`title:de`, `overview:de`, `tagline:de`,
  `sort_title:de`; alte Sperren ohne Sprache gelten weiter für alle Sprachen, beim Aufheben bleiben
  die anderen Sprachen gesperrt), `PATCH /admin/items/{id}/metadata`, `GET /admin/genres`,
  `POST /admin/items/{id}/refresh` mit `{unlockAll}`, `PUT /admin/items/{id}/images/{kind}/upload`
  (`http.Handler`, höchstens 20 MB, JPEG/PNG/WebP nach Inhalt, höchstens 50 Megapixel, ffmpeg mit
  `-max_pixels`; erst skalieren, dann sperren), `AdminItem.metadata` und `AdminItem.imageKinds`,
  Folgenstills von TMDB. Ein Upload wartet auf ein laufendes `image.fetch` desselben Titels.
- **P-7 Bibliotheken auf der Startseite:** Kachelreihe «Meine Medien» und je Bibliothek «Neu in …»,
  Kennung `id` je Reihe (`library:<id>`), höchstens **15 sichtbare** Reihen (422
  `too_many_visible`), fehlende Zeilen behalten ihren Platz. Ab Werk ausgeblendet; neue
  Bibliotheken bekommen ihre Reihe per Trigger. Eingefrorener Stand `testdata/db/startseitenreihen.sql`.
- **Reviews** (OMC-Code-Reviewer, zwei Durchgänge): Server und P-7 REQUEST CHANGES (1 High: kaputtes
  Bild nach gültigem Kopf, 1 Medium: WebP-VP8X umging die Pixelgrenze, 5 Low, Rennen mit
  `image.fetch`), Dialog REQUEST CHANGES (1 High: Vorschau des hochgeladenen Bilds fehlte, 4 Medium,
  11 Low). Alle behoben.
- **Wackeltests aus `main` behoben:** `TestQueueKeyframesForDownloadWindow` (unsortierte
  Erwartung), `TestScanbenchSmallTree` (Abbruch nach 1 ms kam unter Last zu spät),
  `AdminItemPage.test.tsx` Originalsprache (zweiter Klick während des ersten Speicherns).
- **Messung** `docs/perf/feinschliff.md` Abschnitt 3: Auf CT 190 brauchte `GET /home` mit 13
  sichtbaren Reihen zuerst **p95 617 ms** (P-11: 300 ms). Ursache: «Neu hinzugefügt», «Meine Medien»
  und die Genres durchsuchten alle rund 70 000 Einträge von `items` samt Folgen und Podcastfolgen.
  Umbau: Teilindex `items_top_added` (Migration 00037), Genres ab den sichtbaren Titeln, «Neu in …»
  sortiert nur die neuesten Titel, Kacheln gebündelt, Reihen nebeneinander (höchstens vier). Nachher
  auf CT 190 (`feinschliff3-8251b6c`): **p50 63,5 ms, p95 66,4 ms**. Die Startseite liefert für alle
  vier Profile dieselben Daten wie vorher.

**Belege auf `04a6e8a8`:** `task ci` grün bis auf Storybook: 360 gleich, **9 neue Stories ohne
Vergleichsbild** (`ui-editordialog--with-tabs`, `--landscape`, `ui-librarytile--default`,
`--focused`, `--few-titles`, `--long-name`, `ui-locktoggle--states`, `--toggling`,
`ui-textfield--with-suggestions`); Go alle Pakete ok, Vitest 1470, api-client 4, design-tokens 2.
e2e `task build` + `task ci:e2e:parallel`: **505/507**; `admin-libraries.spec.ts:417` (Chromium)
einzeln grün, `metadata-editor.spec.ts:86` (Firefox) war ein Testfehler (headless Firefox springt
am Seitenende nicht zum Anfang) und ist mit `e9fd70f8` behoben (`task e2e:quick --
e2e/metadata-editor.spec.ts --project=firefox` dreimal 12 passed, Chromium 8 passed).

**Abnahme auf CT 190 (7. November 2026):** Marco hat drei Runden auf CT 190 geprüft und am Ende
freigegeben. Aus der Abnahme kamen drei Änderungen (Entscheidungsprotokoll vom 7. November 2026,
Konzept 15, 7.5):

- **Stift direkt auf dem Cover:** auf der Detailseite oben rechts auf dem Poster statt in der
  Knopfzeile, am Handy über dem Hintergrundbild. Im Bibliotheksraster ist er immer sichtbar, auch
  auf Touch-Geräten.
- **Stift auf den Covern der Startseite:** auf allen Titelkarten, nicht auf Kacheln «Meine Medien»,
  Sammlungen und Genres. Auf «Weiterschauen» bearbeitet er die Serie. Alle Stifte erscheinen nur für
  den Server-Administrator.
- **Dialog fest 90 % der Fensterhöhe:** Die Reiter «Zuordnen» und «Aktualisieren» verschieben das
  Fenster nicht mehr.

CT 190 lief zur Abnahme auf `feinschliff3-23b2f0a`. Die GitHub-CI auf `23b2f0a4` fand einen Testfehler
in `episode.spec.ts:72`: Der Stift ist auf der Folgenseite ein neuer Tab-Halt vor «Abspielen». Der Test
ist angepasst wie `detail.spec.ts` (`task e2e:quick -- e2e/episode.spec.ts` 5 passed, Firefox 1
passed).

Gemergt als **PR #104** (Merge `69308bd7`, GitHub-CI 19/19 grün auf `c681ac6c`).

## Release 1.14.0 (6. November 2026): veröffentlicht

Release ab `main` nach PR #100 (Konzept 15, Stufe 2). Vertrag **1.14.0** (additiv), Migration
**`00034_job_windows.sql`**: der Weg zurück auf 1.13.2 geht nur über die Datenbanksicherung, die der
Server vor der Migration schreibt (`/config/backups/pre-migration-…`).

- Versionsstellen auf 1.14.0 im Zweig `claude/release-1.14.0` (`deploy/.env.example`,
  `deploy/compose.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`),
  gemergt als **PR #101** (Merge `6a3d9eb5`).
- Tag `v1.14.0` von Marco auf `6a3d9eb5` gesetzt (6. November 2026, Release-Notes
  `release-notes-1.14.0.txt`). Release-Lauf `42287836155` grün, GitHub-Release um 21:24
  veröffentlicht.
- CT 190 läuft seit 21:27 auf Image `1.14.0` (Sicherung `.env.bak-vor-1.14.0`; Migration 00034 lief
  dort schon beim Testimage, DB-Sicherung
  `pre-migration-v1.13.2-21-g353f552a-20261106T185611.630Z.db`). Worktrees, Zweige und
  Zwischen-Images der Stufe 2 sind aufgeräumt.

## Konzept 15 «Feinschliff», Stufe 2: veröffentlicht als 1.14.0 (6. November 2026)

Zweig `claude/feinschliff-stufe2-26fc61` ab `main` `8ca83892` (Release 1.13.2). Geplant als Release
**1.14.0**: Vertrag **1.14.0 (additiv)**, Migration **`00034_job_windows.sql`**.

- **P-1 «Weiterschauen» je Serie:** eine Karte je Serie mit Serienposter, Serientitel und «S4 · F7 ·
  Noch 20 min» oder «S4 · F8 · Neu»; ein Klick führt zur Folge (F-1). Je Serie zählt der jüngste
  Fortschritt; ist die Folge fertig, zeigt die Karte die nächste nicht fertige sichtbare Folge,
  solange die letzte Aktivität höchstens 60 Tage zurückliegt (F-2, `visibility.NextEpisodeWindow`,
  geteilt mit den automatischen Downloads). Eine ganz fertige Serie fällt weg; Folgen ohne
  Staffel- oder Folgennummer haben keine nächste Folge (wie `episodeChain`). Vertrag: neues Feld
  **`ItemRef.seriesRef`** (Schema `SeriesRef`), abweichend vom Konzept nicht `series`, weil
  `ItemDetail.series` schon existiert. Die Vorabmalerei malt das Serienposter nur für die Reihe
  «Weiterschauen».
- **P-3 Zeitfenster (4.3):** `probe.keyframes`, `segments.detect` und `chapters.images` laufen nur
  im eingestellten Fenster (Verwaltung › Hintergrundjobs, aufgeklapptes Feld «Nur in einem
  Zeitfenster», ab Werk aus). Neu: `job_types.window_start/window_end`, `jobs.override_window`,
  `JobTypeInfo.windowable/window/windowTimeZone/windowOpensAt`, `JobRun.overridesWindow`,
  `PUT /admin/job-types/{type}/window` (409 `job_type_not_windowable`). Pause geht vor Fenster;
  Wiedergaben **und Offline-Downloads** (Marcos Entscheid «Download geht vor», Nachtrag zu
  Frage 22) starten trotzdem sofort; `override_pause` und die Grenze von drei bleiben. Die Spalte
  zeigt «wartet bis 01:00» nur, solange ein Auftrag ohne Ausnahme auf das Fenster wartet.
- **Review** (OMC-Code-Reviewer): APPROVE mit 1 Medium (M-1 Downloads, entschieden und umgesetzt)
  und 8 Low; L-1 bis L-5, L-7, L-8 behoben, L-6 (`seriesRef` an allen Folgen) bewusst belassen.
- **Messung** `docs/perf/feinschliff.md`: `ListContinueWatching` mit 10 000 Fortschrittszeilen
  p95 50,1 → 20,1 ms (Index-Weg über die Staffeln, `GROUP BY` statt Fensterfunktion).
- **Zeitzone:** Der Planer meldete die Zeitzone als «Local», auch mit gesetztem `TZ` (vorbestehend
  beim Zeitplan, bei der Abnahme auf CT 190 bemerkt). Er nimmt jetzt die Zone der Statistik
  (`admin.ResolveZone`), also z. B. «Europe/Zurich»; die Web-App fällt bei «Local» weiter auf die
  Zeitzone des Browsers zurück.
- **Storybook-Baseline** `ui-postercard--continue-episode` auf CT 190 erzeugt (355 gleich, 1 neu).

**Belege auf `269f2d23`:** `task ci` grün bis auf Storybook: 359 gleich, **1 neue Story ohne
Vergleichsbild** (`ui-postercard--continue-episode`); Go 36 Pakete ok, Vitest 1441, api-client 4,
design-tokens 2. e2e separat `task ci:e2e:parallel` (nach `task build`): **490/492**, die zwei
Firefox-Fehler (`admin-picks.spec.ts:98` Tab-Lauf, `downloads.spec.ts:44` Vorbereitung unter Last)
einzeln wiederholt grün (`task e2e:quick -- e2e/downloads.spec.ts e2e/admin-picks.spec.ts
--project=firefox`: 11 passed).

Abgenommen von Marco auf CT 190 (`feinschliff2-a6ea4c2`; Migration 00034 lief dort in 1,6 s),
gemergt als **PR #100** (Merge `cb759b49`, GitHub-CI 19/19 grün auf `a6ea4c2e`).

## Release 1.13.2 (6. November 2026): veröffentlicht

Release ab `main` nach PR #98 (Konzept 15, Stufe 1). Der Vertrag bleibt **1.13.0** (keine Änderung an
`api/openapi.yaml` seit `v1.13.0`), keine Migration.

- Versionsstellen auf 1.13.2 im Zweig `claude/release-1.13.2` (`deploy/.env.example`,
  `deploy/compose.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`),
  gemergt als **PR #99** (Merge `8ca83892`).
- Tag `v1.13.2` von Marco auf `8ca83892` gesetzt (6. November 2026, Release-Notes
  `release-notes-1.13.2.txt`). Release-Lauf `42213787617` grün, GitHub-Release um 11:57
  veröffentlicht.
- CT 190 läuft seit 13:01 auf Image `1.13.2` (Sicherung `.env.bak-vor-1.13.2`). Worktrees, Zweige
  und Zwischen-Images der Stufe 1 sind aufgeräumt.

## Konzept 15 «Feinschliff», Stufe 1: veröffentlicht als 1.13.2 (6. November 2026)

Zweig `claude/feinschliff-stufe1` ab `main` `311a5117` (Merge von PR #97, Konzept 15). Geplant als
Release **1.13.2**: Vertrag bleibt **1.13.0**, keine Migration. Frage 22 im Entscheidungsprotokoll
(F-1 bis F-16); Hinweise in Konzept 08, ADR 0016, ADR 0017 und Spec E-9/A-5.

- **P-2 Beschreibung je Job:** nach Marcos Abnahme auf CT 190 (6. November 2026) **eigene Spalte
  «Beschreibung»** statt zweiter Zeile unter dem Namen: höchstens zwei Zeilen, danach «…», voller
  Satz als `title` und oben in der aufgeklappten Zeile; Texte `admin.jobs.types.<key>.description`
  für alle 24 Jobs. Bei 1280 px passen 23 von 24 ganz, ab 1440 px alle. S-5 bleibt streng 40 px,
  gemessen bei 1920 px, wo die Zeile einzeilig ist. Nebenbei behoben: «in 5 h 60 min» → «in 6 h».
- **P-3:** Ereignis-Jobs zeigen «automatisch» statt «bei Bedarf» und im aufgeklappten Feld einen
  Satz, warum sie keinen Zeitplan haben. «Kapitelbilder nachholen» und «Originalsprachen
  nachholen» sind Zeitplan-Jobs (ab Werk aus, 7 Tage, `jobs.SeedOff`). Zeitfenster (4.3) folgen in
  Stufe 2.
- **P-4:** neue Route `/admin/notifications` mit derselben `ChannelSettingsPage` und einem Verweis
  auf Integrationen; Gäste und Zweitprofile werden nach `/settings/notifications` umgeleitet, das
  für alle Konten bleibt. Der Link im E-Mail-Formular zeigt auf die neue Seite.
- **P-5 Endlosband:** Spotlight-Reihe dreht über CSS `order`, DOM-Reihenfolge fest,
  `aria-posinset`/`aria-setsize`, `holdEnds` nach sichtbarer Reihenfolge, Takt hält bei Fokus in der
  Reihe, kein Leerraum und kein Einrasten mehr in dieser Reihe.
- **Update-Prüfung:** Zeitlimit 20 s; nach einem Fehlschlag eines geplanten Laufs erneut nach
  15 min, 1 h und 6 h über den einmaligen Termin (`retryAt` im Zustand `update_state`, keine
  Migration). Ein früherer Termin des Administrators bleibt; ein Erfolg löscht nur den eigenen
  Termin. «Jetzt prüfen» auf der Karte zählt nicht mit; «Jetzt ausführen» auf der Jobs-Seite zählt
  mit (der Job kennt seine Herkunft nicht).
- **Review** (OMC-Code-Reviewer): REVISE mit 1 Medium (Richtungsmerker des Bands nach Umkehr
  mitten im Gleiten) und 9 Low; 7 behoben, nicht umgesetzt: Sommerzeit-Verschiebung des
  Termins, Animation beim Umkehren mitten im Gleiten, toter Zweig `triggerManual`.

**Belege auf `5ab72ed8`:** `task ci` grün bis auf Storybook: 357 gleich, **2 erwartete Abweichungen**
(`ui-herocarousel--active-poster`, `ui-herocarousel--paused`: gedrehte Reihe); Vitest 1430,
api-client 4, design-tokens 2, 36 Go-Pakete ok. e2e separat `task ci:e2e:parallel`: **488/488**
in 3 Shards (501 s).

Storybook-Baselines der zwei Bilder auf CT 190 neu geschossen (Marcos Wort, `/root/storybook-ct-k15`,
`--update-snapshots=changed`): 359 passed, nur diese zwei PNG geändert.

Abgenommen von Marco auf CT 190 (`feinschliff1-3410053`), gemergt als **PR #98** (Merge `84b30a98`). Entwurf `release-notes-1.13.2.txt` im
Wurzelordner (unversioniert).

## Release 1.13.1 (5. November 2026)

Patch-Release ab `main` nach PR #93. Der Vertrag bleibt **1.13.0** (keine Änderung an
`api/openapi.yaml` seit `v1.13.0`), keine Migration.

- **PR #79** (Merge `f2f12b3`): Der Scanner hält eine neue Datei mit der Inode einer eben gelöschten
  Datei gleicher Grösse nicht mehr für die verschobene alte (Abschnitt unten).
- **PR #92** (Merge `ad76bbc`): e2e-Test des Offline-Players wartet vor den Pfeiltasten der
  Werkzeugleiste (Firefox-Wackler).
- **PR #93** (Merge `66940d2`): CI-Actions auf `setup-go` 7.0.0, `cache` 6.1.0, `upload-artifact`
  7.0.2, `download-artifact` 8.0.2, `attest-build-provenance` 4.2.2 (mit
  `create-storage-record: false`); Dependabot prüft auch `.github/actions/*`.
- Versionsstellen auf 1.13.1 im Zweig `claude/release-1.13.1` (`deploy/.env.example`,
  `deploy/compose.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`), Tag
  `v1.13.1` setzt Marco auf den Merge-Commit. Release-Notes: `release-notes-1.13.1.txt` im
  Wurzelordner (unversioniert).
- Beim Tag `v1.13.1` läuft der Attest-Schritt erstmals mit `attest-build-provenance` 4.2.2; den
  Release-Lauf darauf prüfen.

## Scanner: wiederverwendete Inode ist keine Verschiebung (gemergt 5. November 2026, in 1.13.1)

Zweig `claude/scanner-inode-reuse` ab `main` `cbf17fb`, `main` `ae0502b` (mit Idee 04) und
`58245ed` (Release 1.13.0) eingemergt; als **PR #79** gemergt (Merge `f2f12b3`, 5. November 2026),
veröffentlicht in 1.13.1. Der Scanner ordnete eine neue Datei einer bekannten zu, sobald (`dev`, `inode`, `size`)
übereinstimmten. ext4 vergibt die Inode einer eben gelöschten Datei neu; bei gleicher Grösse hielt
der Scanner die neue Datei für die verschobene alte und holte deren Titel zurück, statt einen neuen
anzulegen. Aufgefallen als wackliger
`TestRelinkFoldsOlderTwins` in PR #76 (Idee 04), dort nur in der Testhilfe `addFilm` entschärft
(jede Datei eigene Grösse); die Entschärfung bleibt.

- Fix in `planRoot`: Ein Treffer über die Inode gilt direkt nur bei gleicher `mtime` (eine
  Umbenennung behält sie, ein Umbenennen kostet also weiterhin kein Lesen). Bei anderer `mtime`
  entscheidet der `partial_hash`: gleich, dann dieselbe Datei; sonst geht es weiter wie bei einer
  neuen Datei (Abgleich über `partial_hash`, sonst neu). Die alte Datei fehlt dann, im Pfad-Scan
  erst, wenn ihr Ordner zum Scan gehört (sonst beim nächsten vollen Scan).
- Mehrere bekannte Dateien mit derselben Inode (die fehlende alte und die neue) sind alle
  Kandidaten; die mit gleicher `mtime` gewinnt.
- Ausnahmen, in denen die Inode wie bisher allein entscheidet: Die bekannte Datei hat keinen
  `partial_hash` (Hashen schlug damals fehl), oder die neue Datei lässt sich gerade nicht lesen
  (abgerissene Freigabe). So geht keine Datei verloren, und der Schwellwert für verlorene Dateien
  springt nicht an.
- Kosten: gehasht (128 KiB) wird zusätzlich nur eine Datei mit fremdem Pfad, bekannter Inode und
  neuer `mtime`; solche Dateien wurden vorher ohnehin neu geprüft (`ffprobe`).
- Grenzfall: Eine verschobene Datei, die am Ort im Kopf oder Ende umgeschrieben wurde (gleiche
  Grösse), gilt jetzt als neue Datei. Vorher galt sie als dieselbe.
- Test `TestScanReusedInodeIsNoMove`: gleiche Inode und Grösse, neuer Inhalt und neue `mtime`
  ergibt eine neue Datei mit neuem Titel (ohne Fix rot); verschoben und nur berührt bleibt
  dieselbe Datei; ohne alten `partial_hash` entscheidet die Inode. Paketbeschreibung in
  `server/internal/library/doc.go` angepasst.

## Konzept 14 «Schneller Wiedergabestart»: gemergt und auf CT 190 abgenommen (5. November 2026)

**Zweig `claude/wiedergabestart-89645e` ab `main` `c514ab1` (Release 1.12.0).** Marcos Freigabe des
Baus vom 5. November 2026. Vertrag **1.13.0** (additiv), Migration **00033**, Frage 20, ADR 0022.
Alle Entscheide des Baus in `docs/planning/14-wiedergabestart.md` 11.3.

- **Schritt 0:** Frage 20, ADR 0022 (Teilnachfolger von ADR 0003, Nachtrag zu ADR 0017); O-9 auf
  CT 190 nur lesend ausgelesen: alle 263 fehlgeschlagenen `probe.keyframes` sind `No such file or
  directory` aus einem NFS-Aussetzer am 31. Oktober 2026.
- **Schritt 1:** QSV und VAAPI im HLS-Zweig mit VBR `-b:v 0,75×R -maxrate R -bufsize 2×R`, QSV
  `-preset faster`, kein `-global_quality` mehr; Regel für kleine Raten; CBR der Downloads unverändert.
- **Schritt 2:** Paket `internal/mediaindex` (Matroska-Cues, MP4-Tabellen, Plausibilität), gehärtet
  gegen fremde Bytes, Fuzz-Ziele, Kreuztest gegen ffprobe in `transcode`.
- **Schritt 3:** Migration 00033 (`keyframes_source`, `override_pause`, `once_at`, Teilindizes),
  eingefrorener Stand `testdata/db/wiedergabestart.sql`; `probe.file` liest den Index vor der
  Schreibtransaktion aus dem Container und prüft ihn mit `Runner.VerifyKeyframes`; jedes Schreiben
  an die Dateiversion gebunden, Index je Version endgültig.
- **Schritt 4:** Pool `index` mit einem Arbeiter; Pause-Ausnahme für Wiedergabe-Aufträge (höchstens
  3, 24 h Ruhe nach Fehlschlag); einmaliger Termin für alle Zeitplan-Jobs; Volllese-Job mit
  Fehlerklassen; Jobart `keyframes.backfill` («Keyframe-Index nachholen»).
- **Schritt 5:** `Start` wartet nie auf einen Index (`KeyframeTimeout`, `WithKeyframeIndex`
  entfernt), Grund `keyframes_unavailable`; Rebuild nach Dateiwechsel antwortet 409
  `playback_source_changed`; Offline-Downloads lesen nie selbst und warten höchstens 24 h.
- **Schritt 6:** Vertrag 1.13.0 (`Library.keyframeIndex`, `JobSchedule.once`, `onceLocal`/`clearOnce`,
  Pool `index`, `JobRun.overridesPause`, `JobTypeInfo.overridesPauseForPlayback`, 409 an HLS und
  Untertiteln).
- **Schritt 7:** Grund auf der Stream-Kachel, Index-Abdeckung je Bibliothek, Karte «Keyframe-Index
  nachholen», «Einmalig am», Pausentexte; Player (hls.js und Safari) startet bei 409 einmal neu.
- **Reviews:** Welle A (1 M), B (1 M), C (0 M), D (1 M), Schluss-Review (APPROVE, 0 M); alle M- und
  m-Befunde behoben bis auf m-10 aus C (nur Kommentar) und den optionalen Aktivitätseintrag aus D.

**Belege (`task ci` auf `ea74b979`, 14 min):**

| Prüfung | Ergebnis |
|---|---|
| `ci:server` | Codegen, Lint, `go test` unter `-race`: 36 Pakete `ok` |
| `ci:web` | Lint, Vitest `1427 passed` |
| `ci:storybook` | `359 passed`, keine fehlende Baseline (kein Bauteil in `web/src/ui` geändert) |
| `build`, `ci:e2e:parallel` (3 Shards) | 484 von 484 (179, 154, 151) |
| Schluss-Fixes `1c03f7d5`, `18ade097` | `task check:fast` grün, gezielte Tests unter `-race` `ok`, Lint 0, `gen:check` sauber, e2e `hls` 2 passed |

Weitere Messwerte aus dem Bau: `probe.file` auf den lokalen Testmedien im Mittel 72 ms je Datei
(29–117 ms) inklusive Stichprobe; e2e-Remux von «H264 MKV Long» beim ersten Abspielen in 224 ms;
Prüfung der Obergrenze beim Einreihen 9,1 ms → 38 µs (50 000 wartende Jobs); Claim 5–10 % langsamer.

**Offen:**
- **Abnahme auf CT 190 am 5. November 2026 bestanden** (Image `wiedergabestart-68cefaf`, Migration
  00033 in 1,8 s; Sicherung `.env.bak-vor-wiedergabestart`): Silo S01E03 ohne Index startet mit
  `POST /play` in 16,6 ms statt 20 016 ms, der Index kommt trotz Pause in 193 ms aus den Cues, der
  zweite Start remuxt; Nachhol-Lauf 257 Dateien in 88 s, «ausstehend = 0» in allen Bibliotheken;
  Termin feuert pünktlich; QSV-Bild auf dem Handy scharf; QSV und VAAPI mit VBR statt CQP. Werte in
  `docs/perf/wiedergabestart.md`.
- **PR #90** auf Marcos Wort gemergt (`619d53cb`), GitHub-CI 19 von 19 grün.
- **Release 1.13.0:** Versionsstellen auf 1.13.0 im Zweig `claude/release-1.13.0`
  (`deploy/.env.example`, `deploy/compose.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`,
  `docs/dev/HANDOVER.md`), Tag `v1.13.0` setzt Marco auf den Merge-Commit. Release-Notes:
  `release-notes-1.13.0.txt` im Wurzelordner (unversioniert wie 1.5.0 bis 1.12.0). Danach CT 190
  vom Zweig-Image `wiedergabestart-68cefaf` auf das Release-Image 1.13.0 (keine Migration mehr).
- Bekannte Grenzen: 4K-HDR auf QSV startet weiter in 2–3 s (CPU-Tonemapping); NVENC hält die Leiter
  nur im Mittel (O-19); Safari mangels Apple-Gerät nur mit Vitest geprüft.

## Konzept 13 «Abspielseite», Stufe 3 «Bilder»: gemergt und auf CT 190 abgenommen (5. November 2026)

**Zweig `claude/abspielseite-stufe3-992847` ab `main` `4022ba8` (Release 1.11.0).** Marcos Wort
«Wenn Stufe 2 abgeschlossen ist …, starte direkt Stufe 3». Vertrag **1.12.0** (additiv). Keine
Migration. Alle Entscheide des Baus in `docs/planning/13-abspielseite.md` 11.15 «Stufe 3».

- **Schritt 13:** `transcode.Chapters` für Film und Folge (Hörbücher, Podcasts, Extras
  unverändert): Titel, die nur eine Zeit oder «Chapter 12» sind, fallen weg (der Client zeigt
  «Kapitel 3»); unter vier Kapiteln und ab 30 Minuten Ersatzkapitel auf jeder vollen 10. Minute
  (`Chapter.auto`); ein echter Abspann (höchstens 15 Minuten, zweite Hälfte) bleibt. Wiedergabe und
  Download-Manifest nutzen dieselbe Funktion, ein gemeinsamer Mapper `toChapters`.
- **Schritt 14:** `Runner.GrabFrame` (Eingabe-Sprung, ein Bild, WebP 480 px, HDR/DV auf der CPU
  tonemappt, anamorph entzerrt, Schwarztest im selben Lauf, zweiter Versuch 10 s später); Jobarten
  `chapters.images` (Ereignis, Priorität 5) und `chapters.images_backfill` (nur von Hand, reiht mit
  Priorität 1 ein); Ablage `cache/chapters/<mediaFileId>-<probedAt>-<hash8>/`, Aufräumen im
  stündlichen `library.sweep`; Auslöser neue Datei nach fehlerfreiem Scan, Ende einer Wiedergabe
  (minütliche Aufräumrunde, 10 Minuten verzögert, wartet solange die Datei läuft) und neuer Probe;
  `getChapterImage` als `http.Handler` (`scoped`, 404 für alles Unsichtbare, versionierte URL mit
  `immutable`, sonst `no-cache` mit ETag), `Chapter.imageUrl` in `PlaybackInfo`.
- **Schritt 15:** Bildreihe im Reiter «Kapitel» (`ChapterCard` mit Bild, Nummer als Rückfall,
  «automatisch»), Kapitelbild im Feld über der Zeitleiste (`SeekBar`, feste 16:9-Box);
  Tastaturtest in `chapters.spec.ts`; Stories `UI/ChapterCard` «Images» und `UI/SeekBar`
  «PreviewImage», Baselines auf CT 190 geschossen (Marcos Wort).
- **Reviews:** Schritt 13 (0 M), Schritt 15 (2 M), Schritt 14 (4 M), Schluss-Review der Fixes
  (1 M), Kurz-Review (APPROVE); alle M- und die meisten m-Befunde behoben.
- **Marcos Entscheid beim Bau:** Backfill-Jobs mit Priorität 1, Ereignis-Jobs 5.

**Belege (`task ci` auf `01a5f04`, Storybook auf `8371246`):**

| Prüfung | Ergebnis |
|---|---|
| `ci:server` | Codegen, Lint, `go test` unter `-race`: 35 Pakete `ok` |
| `ci:web` | Lint, Vitest `1414 passed` |
| `ci:storybook` | erst `357 passed`, 2 fehlende Baselines; nach den Baselines (`8371246`) `359 passed` |
| `build`, `ci:e2e:parallel` (3 Shards) | 481 von 481 (157, 162, 162) |
| Nachzug `ed50794` (nur `library`, `jobs`) | `task check:fast` und gezielte Tests `ok`, golangci-lint 0, `gen:check` sauber |

**Abnahme und Merge:**
- **CT 190** auf `stufe3-b846c5a` (`v1.11.0-22-gb846c5a2`, auf dem CT gebaut nach
  `/root/build-b846c5a`), Sicherung `.env.bak-vor-stufe3`, davor Release 1.11.0. Keine Migration,
  Start `healthy`. Kapitelbilder für den Bestand entstehen erst nach dem Abspielen oder mit
  «Kapitelbilder nachholen» von Hand. Abgenommen von Marco am 5. November 2026.
- **PR #88** auf Marcos Wort gemergt (`d70ac4b`), GitHub-CI 19 von 19 grün.
- **Release 1.12.0:** Versionsstellen auf 1.12.0 im Zweig `claude/release-1.12.0`
  (`deploy/.env.example`, `deploy/compose.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`,
  `docs/dev/HANDOVER.md`), Tag `v1.12.0` setzt Marco auf den Merge-Commit. Release-Notes:
  `release-notes-1.12.0.txt` im Wurzelordner (unversioniert wie 1.5.0 bis 1.11.0).

**Offen:**
- Bekannte Grenzen: keine Kapitelbilder offline und im Kapitelblatt des Handy-Layouts; Farben bei
  Dolby Vision Profil 5 können abweichen.

## Konzept 13 «Abspielseite», Stufe 2 «Sprache und Qualität»: gemergt und auf CT 190 abgenommen (4. November 2026)

**Zweig `claude/abspielseite-stufe2` ab `main` `d74b665` (Release 1.10.0).** Marcos Wort «Stelle
die CT190 bitte um und beginne danach Stufe2». Vertrag **1.11.0** (additiv, eine
Bedeutungsänderung: fehlendes oder leeres `subtitleTrack` heisst «Server wählt»). Migration
**00032**. Alle Entscheide des Baus in `docs/planning/13-abspielseite.md` 11.15.

- **Schritt 7:** Migration 00032 (Profilfelder Ton/Untertitel, `profile_series_tracks`,
  `items.original_language`, `tracks.is_hearing_impaired`, `playback_sessions` neu aufgebaut mit
  `original` und `2160p`), eingefrorener Stand `testdata/db/abspielseite.sql`, Vertrag 1.11.0,
  Operationen `getMyPlaybackPrefs`, `putMyPlaybackPrefs`, `resetMySeriesTracks`.
- **Schritt 8:** Originalsprache aus TMDB, TVDB und AniList in `metadata.fill`, sperrbar,
  `setItemOriginalLanguage` in der Verwaltung, Jobart `metadata.original_language_backfill`.
- **Schritt 9:** Paket `internal/trackpick` (Ton- und Untertitelregel nach 11.3, F-11, Gründe),
  Sprachtabelle (470 Einträge, Paritätstest mit dem Web-Client), Serienmerker (nur der geänderte
  Teil), `visibility.TrackContext`, Downloads nach derselben Regel, SDH aus Titel und
  Disposition; im Player «Aus» = `subtitleOff`, jeder Wechsel nennt beide Spuren, Schalter «Für
  diese Serie merken».
- **Schritt 10:** Vorbelegung neuer Profile an allen drei Stellen, Seite «Meine Wiedergabe»
  (`/settings/playback`), Felder im Profilformular, «Als Standard für dieses Profil» im
  Ton-Panel, leere Sprachlisten möglich («Originalton immer»).
- **Schritt 11:** eine Leiter (`transcode.Rungs`), `original` nach 11.5 (a)–(d), 2160p
  freigeschaltet (Backend und Tonemapping aus dem Manager), `QualityCap` (1080p–360p),
  Qualitätsliste im Zahnrad mit Merker je Gerät.
- **Schritt 12:** 2160p in `transcode` (Grenze je Backend, Kosten 5, 4K ab 2160 Zeilen oder 3840
  Breite, H.264-Level 4.1/4.2/5.1/5.2 aus Bildgrösse und Bildrate, Ausgabe in einer 16:9-Box).
  **Messung auf CT 190** in `docs/perf/13-abspielseite.md` 1: QSV, ein 2160p24-Strom 3,8-fache,
  zwei je 1,9-fache, 2160p60 1,4-fache Echtzeit, Kosten 5 bleiben.
- **Reviews:** je Welle ein `code-reviewer` (7; 8, 9, 12; Schluss-Review 9-Fixes, 10, 11); alle
  M-Befunde behoben.
- **Marcos Entscheide beim Bau:** 4K im Breitbild ab 3840 Breite; volle Bildspur vor SDH-Text bei
  «Immer» und «Fremdsprache».

**Belege (`task ci` auf `a29d5c7`, 12 min 44 s):**

| Prüfung | Ergebnis |
|---|---|
| `ci:server` | Codegen, Lint, `go test` unter `-race`: 35 Pakete `ok` |
| `ci:web` | Lint, Vitest `1406 passed` |
| `ci:storybook` | `357 passed` |
| `ci:e2e:parallel` (3 Shards) | 477 von 479; rot `profiles.spec.ts:380` in Chromium und Firefox (der neue «×» als Tab-Halt), behoben in `2fad41c`, danach `profiles` und `playback-settings` 18/18 in beiden Browsern |

**Abnahme und Merge:**
- **CT 190** auf `stufe2-8bcc359` (`v1.10.0-51-g8bcc359a`, auf dem CT gebaut nach
  `/root/build-8bcc359`), Sicherung `.env.bak-vor-stufe2`, davor Release 1.10.0. Migration 00032
  lief in 2,2 s, DB-Sicherung vorher
  `/config/backups/pre-migration-v1.10.0-51-g8bcc359a-20261104T213007.599Z.db`; Start `healthy`,
  Backend `qsv`, Jobart «Originalsprache nachholen» lief an (200 Titel je rund 100 s). Abgenommen
  von Marco am 4. November 2026.
- **PR #86** auf Marcos Wort gemergt (`632938b`), GitHub-CI 19 von 19 grün.
- **Release 1.11.0:** Versionsstellen auf 1.11.0 im Zweig `claude/release-1.11.0`
  (`deploy/.env.example`, `deploy/compose.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`,
  `docs/dev/HANDOVER.md`), Tag `v1.11.0` auf `4022ba8` (PR #87), gesetzt am 4. November 2026. Release-Notes:
  `release-notes-1.11.0.txt` im Wurzelordner (unversioniert wie 1.5.0 bis 1.10.0). Danach Stufe 3
  (Schritte 13 bis 15, Kapitelbilder, Vertrag 1.12.0) in einer neuen Session (Marcos Wort).

**Offen:**
- Bekannte Grenzen und Hinweise für die Release-Notiz: ein gemerktes 1080p wandelt 4K-Titel um;
  Breitbild-Ausgaben werden schmaler (16:9-Box), auch bei Downloads; Level 4.2/5.2 nicht auf
  echten Geräten geprüft; Spulen in 2160p nicht gemessen (keine echte 4K-Datei auf CT 190).
- `SetItemRating`/`SetItemsRating`: ein leerer Body hebt die Freigabe auf (bestehend, nicht aus
  diesem Zweig).
- Auf CT 190 liegen noch `/root/uhd-test` (Testclips, rund 3 GB), das Image `stufe2-8428d7f` und
  `/root/build-8428d7f`.

## Konzept 13 «Abspielseite», Stufe 1 «Bedienung»: gemergt und auf CT 190 abgenommen (4. November 2026)

**Zweig `claude/abspielseite-ae8b82` ab `main` `c023448`, `main` (`f7e2308`, PR #83) nachgezogen.**
Neustart ab `main` auf Marcos Wort (11.15). Vertrag **1.10.0** (additiv). Release 1.10.0 nach der
Abnahme auf CT 190, nur auf Marcos Wort. Bauplan `docs/planning/13-abspielseite.md` 11.12, Nachtrag
mit allen Entscheiden des Baus in 11.15.

- **Schritt 0** (`a7fb55b`): Frage 19, ADR 0021 (2160p), Idee 05 (Trickplay).
- **Schritt 1:** Player-Tests über Aktionen statt Menürollen (`b1920d5`, `5bf481d`).
- **Schritt 2:** Bauteile `VolumeControl`, `PlayerPanel`, `OptionRow`, `ChapterCard`, `SeekBar` in
  `web/src/ui` (`a62a33f` bis `6b844ee`).
- **Schritt 5b:** Stopp durch die Verwaltung. `GET /playback/sessions/{id}/state` antwortet aus dem
  Speicher (`playback/state.go`), der Player fragt alle 4 s (`usePlaybackStopWatch`), Video und
  Audio (`e74335d` bis `96694b4`).
- **Schritt 3:** `PlayerChrome`, Fokusmodell, Tastenkarte, Hilfe mit `?`, `focusOnMount`
  (`0c0287e` bis `7f7cf3e`).
- **Schritt 4:** Panels Ton, Untertitel und Zahnrad; eigene Untertitel-Ebene mit Stil und Versatz;
  Nachtmodus und 150 % über Web Audio, in Safari ausgeblendet (`b69f74d` bis `157cba2`).
- **Schritt 5:** Reiter Kapitel, Als Nächstes, Besetzung, Info; Pause-Ansicht; Nächste-Folge-Karte;
  Handy-Layout mit Doppeltipp; V-17 (nur auf dem Gerät), V-18 (`0a6fdd8`).
- **Schritt 6:** Offline-Player auf `PlayerChrome`, `focusOnMount` in den Offline-Dialogen
  (`486592e`).
- **Nachträge:**
  - Kapitel für Video-Downloads, Marcos Entscheid zu M-1 aus dem Review von Schritt 6
    (`6b8bcee`, `4d49e54`).
  - Jeder geöffnete Player startet eine eigene Sitzung (`d7ace21`).
  - Zurück während einer Navigation aus dem Player geht nicht verloren (`e2a33e0`).
  - Handy-Layout mit eigener Titelzeile, Bild-in-Bild und Vollbild neben der Zeit (`d90a1b6`).
  - Antworten der Statusabfrage in den Stopp-Tests im Testprozess gelesen (`b42de23`).
- **Reviews:** je Schritt ein `code-reviewer` (3, 4, 5, 6) und ein Schluss-Review der Nachträge;
  alle M-Befunde behoben. Bewusst offen: m-6 des Schluss-Reviews (eine doppelte Probe-Abfrage je
  Manifest, Umbau von `visibility` nötig).
- **Marcos Entscheide beim Bau** (11.15): Englisch als zweite Wahl in Ton und Untertitel; Kapitel
  für Video-Downloads vom Server; Handy-Layout richten.
- **Storybook:** 34 neue Baselines auf CT 190 im Playwright-Image `v1.64.1-noble` geschossen
  (Bündel `/root/storybook-ct-k13`, `--update-snapshots=missing`), alle bestehenden unverändert
  (`efbb086`, `6853c56`).

**Belege (`task ci` auf `6853c56`, 12 min 38 s):**

| Prüfung | Ergebnis |
|---|---|
| `ci:server` | Codegen, Lint, `go test` unter `-race`: 34 Pakete `ok` |
| `ci:web` | Codegen, Lint, Vitest `1303 passed`, Shard-Plan und Platz-Selbsttest grün |
| `ci:storybook` | `357 passed` |
| `ci:e2e:parallel` (3 Shards) | 461 von 463; Firefox `audio-download.spec.ts:111` (O-1, Zeitgrenze 120 s beim Vorbereiten) und Chromium `admin-folder-browser.spec.ts:151` (Fokus nach Escape); beide Dateien und der Code dahinter sind auf diesem Zweig unverändert; danach je 4 Wiederholungen in Chromium und Firefox `20 passed` |

Lauf davor auf `972381f`: e2e ebenfalls 461 von 463, andere zwei Tests (`admin-picks.spec.ts:98`
in 8 Wiederholungen grün; Stopp-Test des Hörbuchs, behoben in `b42de23`, danach 24 von 24 grün).

**Abnahme und Merge:**
- **CT 190** auf `abspielseite-9f6d42b` (`v1.9.0-116-g9f6d42b9`, auf dem CT gebaut nach
  `/root/build-9f6d42b`), Sicherung `.env.bak-vor-abspielseite`, davor `wurzelwechsel-34822aa`.
  Keine Migration, Start `healthy`. Abgenommen von Marco am 4. November 2026.
- **PR #84** auf Marcos Wort gemergt (`14964c3`), GitHub-CI 19 von 19 grün.
- Fertige Downloads zeigen Kapitel erst nach einem neuen Download (Release-Notiz).
- Bekannte Grenzen: Bild-in-Bild endet bei jedem Wechsel von Ton, Untertitel, Qualität oder
  Version; das eigene Bild-in-Bild von Firefox zeigt keine Untertitel.
- **Release 1.10.0** (Marcos Wort «Ja, Release 1.10.0»): Versionsstellen auf 1.10.0 im Zweig
  `claude/release-1.10.0` (`deploy/.env.example`, `deploy/compose.yaml`, `deploy/README.md`,
  `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`), Tag `v1.10.0` setzt Marco auf den Merge-Commit.
  Release-Notes: `release-notes-1.10.0.txt` im Wurzelordner (unversioniert wie 1.5.0 bis 1.9.0).
  Danach Stufe 2 (Schritte 7 bis 12, Vertrag 1.11.0) auf Marcos Wort.

## Teststufen, Agentenregeln und Plätze für schwere Läufe (4. November 2026)

**Performance-Audit von Claude Code, OMC und CI; Marcos Antwort «A bis D umsetzen».** In der Woche
vom 28. Oktober bis 3. November lief `task ci` lokal 48-mal (je rund 50 min), derselbe Stand wurde
bis zu sechsmal voll geprüft (Executor, Verifier, git-master, Koordinator, CI auf PR und main), bis
zu fünf Agenten liefen gleichzeitig mit eigenen e2e-Servern. Die OMC-Hooks starten keine Prüfungen
(je Aufruf ≤ 0,3 s) und sind kein Engpass.

- **A:** PR #81 (Konzept 13 Schritt C, schnellere CI und Tests) gemergt, `d86720d`. CI auf
  `3c56e5f` 9,0 min statt Median 16,0 min.
- **B–D:** PR #82 auf `claude/teststufen`: drei Teststufen (`CLAUDE.md`, `docs/dev/SETUP.md`
  «Teststufen»), Regeln für Agenten (`CLAUDE.md` «Agenten und Prüfungen»), zwei Plätze der Maschine
  für die ganze Go-Suite unter `-race`, e2e und Storybook (`scripts/heavy-slot.sh`, Selbsttest
  `task heavy-slot:check`). Entscheid in `docs/planning/04-entscheidungen.md`.
- **Beleg:** `task ci E2E_PORT=8497` auf dem fertigen Zweig grün in 9 min 32 s (Vitest 1082,
  Storybook 323, e2e 431 in drei Shards); Go-Tests dabei aus dem Testcache des Laufs davor, dort
  34/34 Pakete ohne Cache unter `-race`, längstes 220 s.
- **Konzept 13** wird auf Marcos Wort ganz neu ab `main` gestartet. `claude/abspielseite-stufe1`
  (`7accff6`, Schritte 0 bis 4) und der Worktree `agent-af47cb4bbb853b37a` (Schritt 5) bleiben
  unangetastet liegen. Offen für die neue Session: die nicht committete checkptr-Änderung aus
  `claude/ci-schneller` (`-gcflags='modernc.org/...=-d=checkptr=0'` in `server:test`).

## Idee 04 «Benutzerdaten beim Wechsel eines Bibliotheksordners»: gemergt und auf CT 190 abgenommen (3. November 2026)

**PR #76 am 3. November 2026 in `main` gemergt (`34822aa`).** Marcos Wort «Jetzt mergen» kam,
während der neu gestartete e2e-Shard 2 noch lief. Zuvor war er nur wegen des Zeitbudgets von
`hdr.spec.ts` rot (5122 ms statt < 5000 ms, CPU-Tonemapping auf dem Runner).

**Abnahme auf CT 190 (Marcos Wort «Schreibe auf CT 190»):**
- **Image:** `wurzelwechsel-34822aa` (`v1.8.0-28-g34822aa`), auf dem CT gebaut nach
  `/root/build-34822aa`. Container sauber gestoppt, Datenbank gesichert nach
  `config/backups/pre-wurzelwechsel-20261103/`, alte `.env` als `.env.bak-vor-wurzelwechsel`.
  Keine Migration.
- **Automatisch** wurde nichts zusammengeführt. Das ist erwartet: Beide Kopien waren schon vor dem
  Update zugeordnet, und ausgelöst wird nur nach einem Match oder einem Löschen.
- **Pflegeaktion:** Sie zeigte die zwei Paare erst nach einem Neuladen des Browsers; vorher sah
  Marco noch den alten Stand der App. Marco hat beide umgehängt.
  - «Silo»: `01M5TG0K…` bleibt, `merged=34`.
  - «Two and a Half Men»: `01M5TG28…` bleibt, `merged=274`.
- **Prüfung, lesend in der Datenbank:**
  - Beide alten Serien leben (`available=1`), die Kopien sind weg.
  - 15 Staffeln und 291 Folgen hängen unter den alten Serien.
  - 6 Fortschritte (3 alte, 3 aus der Kopie), 168 Wiedergabe-Ereignisse und 11 Wiedergaben hängen
    an den alten Folgen.
  - Kein Fortschritt hängt mehr an einem gelöschten Titel.
  - Je ein `metadata.match` für die beiden Serien ist gelaufen.
  - Die übrigen 496 gelöschten Serien bleiben gelöscht.
- **Scanzeit «Serien» vor dem Update:** 213 bis 608 ms (8 Läufe nach Zeitplan, Median etwa 345 ms).
  Der Wert danach folgt mit dem nächsten Scan nach Zeitplan.
- **Nachgezogen:** Der Hinweis im Dialog «Ordner entfernen» sprach noch von «binnen 30 Tagen». Er
  sagt jetzt, dass Titel über ihre Anbieter-ID zurückfinden, und verweist für einen Umzug auf
  «Pfad ändern».
- **Release 1.9.0 wird veröffentlicht** (Marcos Wort «Erst 1.9.0 taggen», 3. November 2026):
  Versionsstellen auf 1.9.0 im Zweig `claude/release-1.9.0` (`deploy/.env.example`,
  `deploy/compose.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`), Tag
  `v1.9.0` setzt Marco auf den Merge-Commit. PR #79 (Scanner, wiederverwendete Inode) kommt nicht
  mit (Marco), sondern in ein späteres Release. Release-Notes: `release-notes-1.9.0.txt` im
  Wurzelordner (unversioniert wie 1.5.0 bis 1.8.0). Danach beginnt Konzept 13 als Vertrag 1.10.0.


Marcos Auftrag vom 3. November 2026: «Mergen und bauen» (Idee 04, PR #74 gemergt `cbf17fb`). Zweig
`claude/wurzelwechsel` ab `main` `cbf17fb`, Frage 21, ADR 0023, Vertrag **1.9.0** additiv als eigenes
Release vor Konzept 13 und 14, keine Migration. Schritte:

- **W-0** (`258abce`) Papiere: Frage 21, ADR 0023, Plan 3.2, Ideenstatus «Übernommen».
- **W-1** (`167abe0`) Zusammenführung über den Baum im Paket `library` (die alte ID bleibt).
- **W-2** (`4d96de0`) Scanner: Wiederverknüpfung über Anbieter-ID ohne Frist, über (`sort_title`, Jahr)
  30 Tage.
- **W-3** (`c4eb17d`) Haken nach `metadata.match` (`metadata.WithRelink`).
- **Review W-1 bis W-3** (4 Major, 7 Minor) behoben in `1673ec8`, `85f2734` (`jobs.RetryLater`,
  `Queue.Defer`) und `256b954`. Wichtigste Änderungen: dauerhafter Job `library.relink`,
  Mitfalten älterer Zwillinge, gepflegte Werte der Kopie bleiben, nur frische Kopien.
- **W-4** (`3ca1243`, `469f7ba`) Pflegeaktion «Verwaiste Benutzerdaten» (Vertrag 1.9.0, Server,
  Reiter unter «Pflege», Tastaturtest).
- **Review W-4** (4 Major, 10 Minor) behoben in `5c74811`, `db606bc` und `e480d78`. Marcos Antworten:
  **7 Tage Spielraum** für die automatische Wiederverknüpfung, keine Frische-Regel in der
  Pflegeaktion; **alle Paare vorausgewählt**, abgewählte bleiben abgewählt. Ein wackliger Test
  (`TestRelinkFoldsOlderTwins`) hatte seine Ursache in der Testhilfe: gleich grosse Dateien und ein
  wiederverwendeter inode. Belegt mit `-count=200` und `-race -count=50`.
- **W-5** (`4e6e9b1`, `f9696b3`) «Pfad ändern» für einen Ordner (Vertrag, Server, Oberfläche,
  Tastaturtest). Der e2e-Test zieht einen echten Ordner um, und der Titel behält seine ID.
- **Schluss-Review** (1 Blocker, 1 Major, 11 Minor) behoben in `b891821`, `fa59c21` und `1dafa0e`.
  Blocker: Eine gesperrte mildere Freigabe des Zwillings konnte die strengere der Kopie verdrängen.
  Major: Auch das Löschen reiht jetzt die Wiederverknüpfung ein («hinzufügen, scannen, entfernen»).
- **Kontroll-Review** der Schluss-Fixes: alle 11 Befunde behoben. Drei kleine Punkte nachgezogen in
  `55b1bf5`: Die Zwillingsabfragen nutzen fest die Indizes der Anbieter-IDs. Nach `ANALYZE` brauchte
  `ListRelinkPairs` vorher 56 s, jetzt 33 ms. Ein neuer Plan-Test sichert das ab.
- **W-6** Messung `docs/perf/wurzelwechsel.md` (`ec2546f`). **Voller `task ci` auf `55b1bf5` grün**
  (Exit 0, 616 e2e-Tests, Log-Scan ohne Befund). Ein Lauf davor scheiterte am bekannten Ausreisser
  `LoginPage.test.tsx` («document is not defined», siehe unten), den dieser Zweig nicht berührt.
- **Offen:** Pull Request, Merge auf Marcos Wort, Release 1.9.0. Abnahme auf CT 190: Image bauen,
  Pflegeaktion für die zwei Serien vom 28. Oktober ausführen, Scanzeit der Bibliothek «Serien»
  messen. Schreibzugriffe auf CT 190 nur auf Marcos Wort.
- **W-6** Messung (`docs/perf/wurzelwechsel.md`), `task ci`, Pull Request. Abnahme und Reparatur auf
  CT 190 erst auf Marcos Wort.

## Konzept 12 «Seitenmuster der Verwaltungsseiten»: umgesetzt und abgenommen, Schritte K12-0 bis K12-9 (3. November 2026)

**Abgeschlossen (3. November 2026).**
- Von Marco auf CT 190 abgenommen (K12-9, unten).
- PR #72 in `main` gemergt (`af87164`).
- **Release 1.8.0 veröffentlicht:** Versionsstellen in PR #75 (`a9b6fe6`), Tag `v1.8.0`,
  Release-Lauf grün.
- Telegram-Hinweistexte bleiben, wie sie sind (Marco, 3. November 2026).
- Die Worktrees und Zweige des Baus sind aufgeräumt.

Was aus der Abnahme übrig blieb, läuft anderswo weiter:
- Verlauf bei ersetztem Ordner: Idee 04, gemergt mit PR #76, Abschnitt oben.
- Admin-Stopp bei voll gepuffertem Titel und `autoFocus`-Dialoge: Konzept 13.
- Weiterer Anbieter für Altersfreigaben: Idee 03, Issue #71.

Zweig `claude/verwaltungsseiten` ab `main` `4c860bc` (nach Konzept 11 und Release 1.7.0), Bau nach
`docs/planning/12-verwaltungsseiten.md` Abschnitt 11.8 (Schritte K12-0 bis K12-9). Marcos Auftrag
vom 2. November 2026: «Bitte beginne mit der Umsetzung und Entwicklung von Konzept 12.» Seine
Antworten: **Durchbauen** (kein Zwischenhalt, abgenommen wird nur in K12-9 auf CT 190); der Vertrag
**1.8.0** erscheint als **eigenes Release**; «Wann geschaut wird» rechnet über **ganze Wochen**
(Entscheid unten). Gebaut mit OMC (`executor` je Schritt, `code-reviewer` je Schritt, `git-master`
für die Merges); Befunde sind vor dem nächsten Schritt behoben.

- **K12-0** (`040f157`): Frage 18 im Entscheidungsprotokoll, ADR 0020, Papier auf `main` `4c860bc`
  verankert.
- **K12-2a** (`6cbbf0f`, `773d0a0`, `4878564`): Komponententokens mit den heutigen Werten,
  `density.compact` definiert, aber ausgeschaltet. Alle 212 Storybook-Referenzbilder pixelgleich,
  auch in einem strengen Lauf ohne Toleranz (0 Differenz). Ausgangshöhe bis zur ersten Arbeitszeile
  bei 1440 px: **Hintergrundjobs 476 px, Integrationen 432 px, Konten 353 px** (Skript
  `web/e2e-perf/admin-height.spec.ts` mit `web/perf.config.ts`, nicht Teil von `task ci`;
  `docs/perf/verwaltungsseiten.md`). Review: freigegeben, kein Major, Befunde behoben.
- **K12-2b** (`edb40c4`, `6a605e2`, `e0c065c`, `fb0fc14`, `f60452c`): alle Bausteine aus 11.2:
  `AdminPage`, `SectionHeader`, `StatusStrip`, `BandPanel`, `Tabs` mit Zahlen, `DataTable` mit
  Klappzeilen, `DisclosureGroup`, `MasterDetail`, `SettingRow`, `SaveBar`, `RankSelect` mit
  `ProviderList` und `FolderList` im Platzwahl-Modus, `ActionMenu`, steuerbarer `ConfirmDialog`,
  `AdminDensityScope` und `density.css` (gilt nur unter
  `:root[data-shell='admin'] :is(#content, [data-density='compact'])` und ab 768 px mit Maus),
  `useDrafts`, `useUnsavedChanges`, `useSearchSelection`, `adminSearch`, `BarChart` horizontal,
  `HeatGrid` mit Legende, `ImageTile`. Stories in beiden Dichten; kein Referenzbild von `main`
  geändert. Review: Änderungen verlangt, drei Major (Pfeiltasten in aufgeklappten Zeilen,
  Fokusverlust in `RankSelect`, Dialoge unter `/settings` kompakt), alle in `f60452c` behoben.
- **K12-1** (`643bdf4`, `7eecf83`, `5c3df2d`, `b5aa092`): Vertrag **1.8.0** additiv mit den Feldern
  aus 11.4, `AdminListImages` statt `AdminListPosters`, Serienbilder für Folgen,
  `StatsWatchTimeByQuarter`, `admin.ResolveZone` beim Start, ganze Wochen
  (`timeWindowDays` 7, 28 oder 84). Review: Änderungen verlangt (falsche Mengenangabe im
  Messpapier, Zonenabweichung), behoben.
- **K12-3** (Hintergrundjobs, Metadaten, Integrationen; letzter Commit `6e46c6d`):
  - *Integrationen* (`5b9212c`, `e96ac0c`, `78b2fc8`): sechs auf- und zuklappbare Felder, alle
    offen; Schlüssel als Zeilen, Webhooks als Tabelle mit Klappzeile (Pfad-Umsetzung und
    Ereignisprotokoll), Entwürfe gehören der Seite; Seite in `features/admin/integrations/` geteilt.
  - *Hintergrundjobs* (`3129a82`, `ab2cbc8`): Zustandsleiste mit fünf Zahlen, Klappbereich «Läuft
    gerade», Reiter je Jobgruppe und «Fehlschläge»; je Jobart eine Klappzeile mit Aktionen,
    Zeitplan und Läufen (statt zweier Dialoge).
  - *Metadaten* (`cbc3251`, `be800a1`, `6e46c6d`): Geltung als Auswahl im Kopf, Bereiche links
    (`MasterDetail`), eine `SaveBar` für alle Geltungen; Sprachen als feste Plätze mit dem neuen
    Baustein `RankedChoiceList`.
  - *Gemeinsame Korrekturen* (`988d1a6`, `470a49b`, `c61fe4b`): Anfangsfokus der Dialoge, siehe
    unten.
  - *Review:* je Seite ein `code-reviewer`-Lauf, Befunde behoben (Metadaten zwei Läufe, einer mit
    einem High-Befund zu den Pfeiltasten in `RankedChoiceList`).
- **K12-4** (Bibliotheken, Pflege, Titel, Podcasts; Stand `ce34f53`):
  - *Pflege* (`0473896`, `55da553`): Reiter je Problemart mit Zahl, je Reiter ein Klappbereich je
    Bibliothek mit Tabelle; Duplikate als Gruppen; Auswahl und Ergebnis gehören der Seite.
  - *Titel* (`fb74426`, `34cef47`): Reiter Metadaten, Bilder, Dateien und Trailer; Bilder und
    Dateien als Tabellen, «Herkunft je Feld» und «Freigabe» als Klappbereiche; die Freigabe ist ein
    Entwurf der Seite.
  - *Podcasts* (`7960b73`, `6a0fe90`): Filterreiter «Alle» und je Podcast-Bibliothek in
    `?tab=library-<id>`; je Podcast eine Klappzeile mit Autor, Feed-Adresse, Bibliothek und Fehler.
  - *Bibliotheken* (`e069714`, `ce34f53`): Filterreiter nach Art mit Zahl, je Bibliothek eine
    Klappzeile (Scan, Ordner, «Weitere Aktionen» mit «Gefahr», Fehlerband); «Bibliothek
    hinzufügen» als Dialog mit Entwurf.
  - *Gemeinsame Korrektur* (`b593486`): Fokus in verschachtelten Formulardialogen.
  - *Review:* je Seite ein `code-reviewer`-Lauf, Befunde behoben.
- **Aufräumrunde 1** (`653cf80`, `a0b7ba9`, `f6c6f6c`, `b1a158a`, `2b128e7`): wackliger Unit-Test von
  `IntegrationsPage` (Text per Einfügen statt Taste für Taste, Warten auf das Entfernen aus dem
  Cache); wackliger Firefox-e2e der Podcasts (`expectNoFocusTrap` wartet, bis kein Dialog mehr
  `[data-exiting]` trägt); Live-Regionen von Hintergrundjobs und Metadaten bleiben im
  Zugänglichkeitsbaum; Testlücken von K12-3 (Zurück-Taste, Fokus der Rückfrage, JOB-4a, JOB-6,
  JOB-7, MET-1 bis MET-3) geschlossen.
- **K12-5** (Geräte, Streams, Konten mit Profilen; Stand `b802319`): je Seite ein
  `code-reviewer`-Lauf, Befunde behoben.
  - *Geräte* (`5d452c5`, `a9b993e`): Reiter «Geräte» und «Fähigkeiten» im `BandPanel`, je Gerätetyp
    ein Klappbereich (ohne Typ unter «Unbekannt»); eine Rückfrage der Seite für «Vergessen».
  - *Streams* (`8bfaa04`, `fdc1be7`): laufende Streams als Kacheln mit Hintergrundbild, darüber
    die Leiste «Serverlast»; Pfeiltasten zwischen den Kacheln, Abfrage pausiert bei offener
    Rückfrage. Neue Bausteine `StreamTile` und `StreamProgress` (von `StreamRow` mitgenutzt,
    dessen Bilder unverändert bleiben).
  - *Konten* (`cac31f2`, `b802319`): Zustandsleiste mit vier Zahlen, Reiter «Konten» und
    «Einladungen»; je Konto eine Klappzeile (eigenes Konto: E-Mail, Passwort, «Meine Profile»;
    fremdes: Zugriff). `ProfileList` aus `ProfileSettingsPage` herausgelöst und von
    `/settings/profiles` und «Konten» geteilt; Einladungen als Tabelle.
- **K12-6** (Startseite, Sammlungen, Spotlight, Cat's Empfehlungen, Ankündigungen; Stand `0490a03`):
  je Seite ein `code-reviewer`-Lauf, Befunde behoben.
  - *Startseite* (`afb2fb3`, `d4aac2c`): Reiter «Reihenfolge» und «Vorschau», Platzwahl statt
    «Nach oben»/«Nach unten», Klappzeile mit Beschreibung, eine `SaveBar` mit «Verwerfen».
  - *Sammlungen* (`0786112`, `7952385`): Filterreiter, Mitglieder inline in der Klappzeile statt im
    Dialog; Entwurf je Sammlung, TMDB-Sammlungen nur lesbar.
  - *Spotlight* (`f71ab06`, `e288d51`, `7e76c51`): drei auf- und zuklappbare `BandPanel`
    («Quelle», «Einträge der Liste», «Verwaiste Anpassungen»); Quelle als `SettingRow`-Zeilen
    mit einem Entwurf der Seite.
  - *Cat's Empfehlungen* (`6186790`, `a5c803b`): Filterreiter nach Zeitfenster, Notiz und Zeitfenster
    in der Klappzeile, Platzwahl, Entwurf je Eintrag.
  - *Ankündigungen* (`8b8b363`, `f1d0c19`, `0490a03`): Liste und Detail (`MasterDetail`) statt
    Dialoge. Gemeinsame Korrektur `f1d0c19`: `MasterDetail` setzt den Fokus beim Tabben in die Liste
    auf den gewählten Eintrag. `0490a03` behebt die Befunde des Reviews.
- **K12-7** (Statistik, Sicherung, Benachrichtigungen; Stand `c2a969d`): je Seite ein
  `code-reviewer`-Lauf, Befunde behoben.
  - *Statistik* (`c37e3d4`, `c5c190b`, `52fdada`): Mockup «Raster»; neu «Wann geschaut wird»
    (Stunde, Wochentag, Raster «Woche und Stunde»), Plätze 1 bis 3 als Kacheln, Rest als Tabelle.
  - *Sicherung* (`6503cfc`, `e1cab40`): Reiter «Sicherungen», «Neue Sicherung» und «Aus Datei
    wiederherstellen», je Art ein Klappbereich; Wiederherstellen und Löschen im Menü unter
    «Gefahr»; Passwörter nur in Reiter oder Dialog.
  - *Benachrichtigungen* (`80a9432`, `c2a969d`): E-Mail, Telegram und Web Push als drei
    auf- und zuklappbare Felder in heutiger Dichte (S-1); Entwurf je Kanal; neue Rückfrage vor
    «Verknüpfung lösen».
- **Aufräumrunde 2** (`46e5fc1` bis `fa81401`, 19 Commits, nach dem Merge `26e5d48`): gemeinsame
  Bausteine, die Speicherregel von Frage 18 auf jeder Seite, Befunde des Reviews.
  - *Fokus-Helfer* (`46e5fc1`, `fa81401`): `afterRender`, `afterDialogsClosed` und `focusTitleIfLost`
    stehen einmal in `ui/focusAfter.ts` (über `@/ui/aria`) und ersetzen 17 Kopien und drei Arten, auf
    ein schliessendes Dialogfenster zu warten. Verlorener Fokus heisst: `body`, ein abgehängtes
    Element oder ein fokussiertes Steuerelement, das gesperrt wurde. `afterDialogsClosed` wartet,
    bis kein Overlay mehr `data-exiting` trägt. `afterRender` läuft in einem verborgenen Reiter
    weiter: es wartet auf den Frame höchstens 100 ms (kein Frame, `requestAnimationFrame` feuerte nie).
  - *Auswahl in der Adresse* (`98ee610`, `0b0893c`): `useSearchSelection` ersetzt zwölf eigene
    Effekte für unbekannte `?tab=` und `?item=` durch die Option `normalise` (ersetzt, kein Eintrag im
    Verlauf). Seiten mit Ids aus einer Liste übergeben `normalise` erst nach dem Laden (Podcasts,
    Titel, Ankündigungen). Konten nutzt es für den Reiter, `?item=` (me, Hash) bleibt bei der Seite.
  - *StatusRegion* (`755034d`, `bb111df`): `ui/StatusRegion` und `useStatusMessage` ersetzen die
    Kopien der Seiten, Integrationsblöcke, Pflegereiter und Benachrichtigungskanäle samt `.live`-CSS
    und Zählern. Die Region bleibt im Zugänglichkeitsbaum, auch leer, verlässt aber den Fluss; es gibt
    eine `alert`-Variante, und jede Meldung hat einen eigenen Schlüssel (derselbe Text zweimal wird
    zweimal vorgelesen). Der Schlüssel zählt über einen Ref, damit er nach `say(null)` nicht von vorn
    beginnt. Story in beiden Dichten mit Referenzbildern.
  - *Speichern-Leiste* (`6ab38a3`, `6e7196a`): die `SaveBar` klebt auf Metadaten und Startseite am
    unteren Rand (sie ist jetzt direkt Kind der Spalte, danach Fehlerhinweis und `StatusRegion`).
    Sie trägt `data-save-bar`; solange eine auf der Seite steht, hat das Dokument ein
    `scroll-padding-block-end`, damit die Leiste das fokussierte Feld nie verdeckt (WCAG 2.4.11).
    Ein e2e auf Metadaten tabbt zum letzten Link über der Leiste; ohne Padding scheiterte er in
    beiden Browsern.
  - *Speicherregel auf jeder Seite* (`4586daa`): «Speichern» ist immer bedienbar und schickt ohne
    Änderung nichts (Fokus bleibt), «Verwerfen» steht nur bei einer Änderung da, und ein Entwurf fällt
    nur weg, wenn er noch dem Gesendeten gleicht. Angewendet auf `SaveBar` (Referenzbild
    `ui-savebar--clean` neu), Hintergrundjobs, Integrationen (Schlüssel, Bot-Token, E-Mail-Server,
    öffentliche Adresse), Konten (Zugriff, eigenes Konto), Spotlight (Quelle), Benachrichtigungen
    (Kanäle), Sammlungen (Mitglieder) und Titel (Freigabe); je Fix ein Test.
  - *Geheimnisfelder* (`0f56bea`): Schlüssel der Anbieter und der Telegram-Bot-Token folgen der
    Regel. Leeres Feld heisst «gespeicherten Wert behalten»: keine Anfrage, kein «Pflichtfeld», der
    Fokus bleibt auf «Speichern». Nur Leerzeichen allein weist das Feld ab.
  - *Ordnerdialog* (`c7831bb`): «Speichern» in «Ordner ändern» ist ohne Änderung bedienbar; gedrückt
    schickt es nichts, Dialog und Fokus bleiben. Eine ungültige Quellenreihenfolge sperrt es weiter.
  - *Passwörter* (`fed0971`): Ein Entwurf des eigenen Kontos, der während des Speicherns weiter
    getippt wurde, bleibt (Frage 18); aktuelles und neues Passwort werden trotzdem geleert.
  - *Öffentliche Adresse* (`b872bcf`): die Zeile hat eine eigene Mutation mit dem gesendeten Entwurf
    in den Variablen (`null` bei «Löschen»); vorher lief die Freigabe in einem `onSuccess` von
    `mutate()`, das nicht läuft, wenn die Zeile vor der Antwort aushängt.
  - *Seitenstile (S-12)* (`d309560`, `e54a12f`): die seiteneigenen CSS-Module entfallen (`AdminData`,
    `AdminTablePage`, `Curation`; reine Seitenteile ziehen in den Ordner der Seite um); `moveBy`,
    `useMoveButtons` und der Schlüssel `admin.backup.restore.confirmBody` hatten keinen Aufrufer mehr.
    `e54a12f` entfernt Verweise auf nicht vorhandene Klassen (`styles.label` in `MasterDetail`,
    `styles.addTitle` im Ordnerdialog); ein Scan aller CSS-Modul-Importe findet keine weiteren.
  - *Tests* (`b870fb1`, `7e179ec`, `c66944f`, `3a6ab4f`): `fill()` (leeren, klicken, einfügen) liegt
    in `web/src/test/fill.ts`; die Integrations- und Kontentests füllen damit (der langsamste Fall
    fällt von etwa 760 ms auf 300 ms, unter 2.6 s mit zwei weiteren vollen Läufen parallel); beide
    Kontenblöcke haben 15 s Zeitgrenze, weil die Passwortfälle unter Last 5 s erreichten und Tasten
    in den nächsten Test liefen. Neue Stories `Tabs` `Disabled` und `DisabledCompact` mit Referenzbildern
    (`isDisabled` kam in K12-7 ohne Story). Der e2e-Helfer `switchOn` kehrt früh zurück, wenn der Kanal
    schon an ist (Telegram verknüpfen schaltet ihn ein; «Speichern» schickt dann nichts).
  - *Sichtbare Verhaltensänderungen:* nach «Verwerfen» verschwindet der Knopf, statt gesperrt zu
    bleiben; nach dem Speichern auf der Startseite bleibt der Fokus auf «Speichern».
- **Testhärtung** (2. November 2026): Zurück-Schritte und Unit-Lücken geschlossen (bis `2b9bf4d`;
  Unit-Lücken PRO-3, BIB-3, TIT-7, PIC-1, BEN-2, PFL-2, PFL-4). Stopp-Lücke des Players: der Fix
  «let the video player notice a stop by the server within 5 s» ist zurückgenommen (`d754f10`). Der
  e2e zum Stopp durch die Verwaltung ist deterministisch, weil die Segmente des Zuschauers im Test
  1 s gehalten werden (`5df15d2`). Die Lücke liegt in Konzept 13, Abschnitt 11.13 (`0e27e0e`).
- **K12-8 Dichte** (2. November 2026, `2078fa9`): `density.css` in `main.tsx` eingeschaltet;
  Tokens `page.title-line-height` (kompakt 0) und `density.compact.page.gutter` (32 px); `/admin` in
  `AdminDensityScope` (`035e068`); e2e `admin-density.spec.ts`; Review-Befunde behoben (`8f4c512`,
  `5936483`).
  - Messung in `docs/perf/verwaltungsseiten.md` (1440 px, Maus, Chromium, Ausgang K12-2a):

    | Seite           | Ausgang | K12-8  | nach Designer-Abgleich | Anteil |
    | --------------- | ------- | ------ | ---------------------- | ------ |
    | Hintergrundjobs | 476 px  | 272 px | 272 px                 | 57 %   |
    | Integrationen   | 432 px  | 259 px | 250 px                 | 58 %   |
    | Konten          | 353 px  | 315 px | 315 px                 | 89 %   |

    **Ziel «höchstens die Hälfte» verfehlt.** Marcos Entscheid vom 3. November: Entscheid bei der
    Abnahme in K12-9. Optionen: Zähler unter der Liste (Hintergrundjobs), Erklärsatz in den
    Hilfe-Knopf (Integrationen), Zähler oder Reiter über der Liste (Konten).
- **Designer-Abgleich** (3. November 2026, alle 21 Seiten gegen die Mockups, `a04c56b` bis `cd1dc75`):
  - Fliesstext 14 px kompakt (`text.body-size`); Tabellenzeilen 40 px, Zellen mittig (`05043f9`,
    `938720a`).
  - Handy: Überlauf durch versteckte Texte in Scroll-Boxen behoben (`27b111c`); Seitentitel bricht nur
    am Handy um (`236e097`, `4ab8331`). Neuer e2e `admin-phone.spec.ts`: 21 Seiten bei 390 px.
  - Einstellungen als Zeilen (`SettingRow` mit `labelAddon`, Höhe nach Mockup: `6906932`, `4ba905d`).
    Mail-Server und Telegram als Zeilen mit «Verwerfen» (`22ef698`; 607 → 461 px, 205 → 159 px).
  - Integrationen: Schlüssel auf einer Zeile (`ab5d9dd`, 122 → 58 bis 78 px); Quelle neben der
    Bezeichnung (`5fd4174`); Felder nach der sichtbaren Bezeichnung benannt (WCAG 2.5.3, `9182d9c`,
    `cd1dc75`).
  - Weiteres: Spotlight-Auswahl untereinander als `RadioGroup` (`952732f`, `8de93cd`); `RankSelect`
    fluchtet mit der Zeile (`e783aed`); Abstand der Worker-Zeile bei Hintergrundjobs (`f78c5a7`);
    Bandtitel entfällt, wenn er den ersten Reiter wiederholt (`BandPanel hideTitle`, S-9, `8f63a49`);
    Sprung zu «Meine Profile» unter dem App-Kopf behoben (`f9eff4e`).
- **Entscheide Marco vom 3. November** (Frage 18, Nachtrag 5): Seitentitel `/admin` 24 px wie auf allen
  Verwaltungsseiten (`035e068`). Auslöser in der Seite als rote Schrift ohne Füllung (`Button`
  Variante `dangerQuiet`, `7d2393b`, `b8d9373`, `e1eeb3f`); Bestätigungsknöpfe in Dialogen bleiben
  gefüllt rot. Höhe: Entscheid bei K12-9.
- **Fokus-Wettlauf** (3. November 2026, `263347e`): `afterDialogsClosed()` auf zwölf Verwaltungsseiten
  und in `ProfileList`; behebt den Firefox-Wackler in `admin-jobs.spec.ts:239`.
- **K12-9 Abnahme** (3. November 2026): CT 190 auf Image `verwaltungsseiten-bf456e1`
  (`v1.7.0-158-gbf456e1`). **Von Marco abgenommen.**
  - *Desktop mit Maus:* alle 18 Seiten mit Marco im Browser durchgegangen. *Handy (44 px):* Marco.
  - *Korrekturen aus der Abnahme* (`583fd8b` bis `bf456e1`):
    - Idee 03 «Weiterer Metadatenanbieter» für Titel ohne Altersfreigabe, Issue #71 (`583fd8b`;
      17 Filme auf CT 190 ohne Freigabe in DE, AT, CH, GB und US).
    - Pflege-Reiter «Altersfreigabe fehlt» statt «Ohne Freigabe» (`3a9b876`).
    - Metadaten: Bereich «Bildqualität» statt «Güte» (`5b2dfc1`).
    - Integrationen: Link je Anbieter-Schlüssel zur Seite, die ihn ausgibt (`4d77104`); leere Felder
      gespeicherter Schlüssel zeigen «API Key gesetzt», der Telegram-Bot-Token «Token gesetzt»
      (`81bca41`).
    - Feste Spalten über die Tabellen einer Gruppe (Geräte, Sicherung, Pflege) und in Tabellen mit
      wechselndem Inhalt, ab 768 px (`DataTable` `layout="fixed"`; `7265dc4`, `0d09bca`, `e8921cb`).
    - Leisten-Eintrag «Profile» entfernt (`8fd9e00`); «Meine Profile» über die eigene Kontozeile.
    - Statistik: Bilder für gelöschte und neu eingelesene Titel über die lebende Kopie mit gleicher
      TMDB-, TVDB- oder IMDb-Id (`e4e323a`, `7d12315`).
    - Spotlight: «Spotlight Wechsel Takt» statt «Takt der Bühne … Sekunden» (`8b712a0`).
    - Statistik: «Woche und Stunde» immer sichtbar, nicht mehr zugeklappt (`1204562`).
    - Review-Nachträge und Doku (`037f760`, `b51a658`, `f4d97eb`, `3f21258`, `bf456e1`).
  - *Messung:* `GET /api/v1/admin/stats?days=90`, 40 Aufrufe auf CT 190 aus dem Browser:
    p50 6.2 ms, p95 10.1 ms, max 15 ms. Schwelle 200 ms, **kein Zwischenspeicher**
    (`docs/perf/verwaltungsseiten.md`, «Messung K12-9 (CT 190)»).
  - *Entscheide Marco* (Frage 18, Nachtrag 6): Höhen-Ziel verfehlt, die Seiten bleiben, wie sie sind
    (Konten, Hintergrundjobs, Integrationen unverändert, keine Verschiebung von Zählern oder
    Erklärsatz); Telegram-Satz in der Token-Hilfe bleibt; Rückfallkette DE/AT/CH/GB/US und
    «Freigabe zuweisen» reichen («Reicht so»); Beschriftungen nach Marco.
  - Release-Notes: `release-notes-1.8.0.txt` im Wurzelordner (unversioniert wie 1.5.0 bis 1.7.0).
- **Offen (Konzept 12):**
  - Push und Pull Request auf Marcos Wort; Merge danach; Release 1.8.0 mit Tag `v1.8.0` (setzt Marco).
  - Versionsstellen auf 1.8.0 gesetzt (`deploy/.env.example`, `deploy/compose.yaml`,
    `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`), Zweig `claude/release-1.8.0`.
  - Designer, nicht umgesetzt: Metadaten-Bereichsliste ohne Symbole; Titel mit Art und Jahr auf eigener
    Zeile (Tit-C); «Letzter Scan» bei Bibliotheken umbricht; Pflege zeigt roh `no_tmdb_key`
    (vorbestehend). Primäre Knöpfe bleiben weisse Pillen statt Orange wie im Mockup (beibehalten).
    Mögliche spätere Bausteine: `OneLine` und `Muted`.
- **Offen / ausserhalb Konzept 12:**
  1. **Bibliothekswurzel ersetzt:** der Scanner liest den Baum mit neuen Ids neu ein; Verlauf und
     Fortschritt bleiben an den gelöschten Ids hängen (CT 190: 29'106 Folgen). Entscheid durch Marco
     offen.
  2. **`autoFocus` in Dialogen von Offline und Player** (`OfflinePinDialog`, `DownloadDialog`,
     `PlayerPage`, `OfflinePlayer`): für Konzept 13.
- **Nächster Schritt:** Pull Request, Merge auf Marcos Wort, Release 1.8.0.

**Belege K12-9:**

- **`task ci` auf `bf456e1` am 3. November 2026: EXIT 0** — Server-Lint `0 issues.`, Go-Tests
  33 Pakete ok, Web 120 Dateien / 1064 Tests, api-client 4, design-tokens 2, Storybook 323 passed
  (38.1 s), e2e 610 passed (33.6 min, Chromium und Firefox), 0 failed, 0 flaky; A-9-Logscan:
  101'895 Zeilen, 17 Secrets des Laufs, 0 Funde. `7c3e29e` und der Status-Commit danach ändern
  nur Doku.
- `GET /api/v1/admin/stats?days=90` auf CT 190, 40 Aufrufe: p50 6.2 ms, p95 10.1 ms, max 15 ms.

**Belege K12-8 und Designer-Abgleich:**

- `task ci` auf `cd1dc75`: **rot (EXIT 201)**.
  - Web-Unit `OfflinePlayer.test.tsx` unter Last: Testrace, behoben `7469eb3`.
  - e2e `admin-home.spec.ts:79` Firefox: echter Fokusfehler. norigin holt 300 ms nach dem Wechsel in die Verwaltung den Fokus in den App-Kopf zurück. Behoben `f3c2f7f`: `autoRestoreFocus=false` auf der Gruppe `shell`, neuer Test.
- `task ci` auf `f3c2f7f`: 605/606. e2e `admin-announcements.spec.ts:222` Firefox: Testrace, der Tab begann vor der Fokussetzung der Seite auf die h1. Behoben `ec5c123`: `tabTo` meldet besuchte Elemente.
- Review-Fixes Integrationen:
  - `a862b43` Server: Botname nur bei gespeichertem Token.
  - `50d4c74` `afterDialogsClosed()` nach dem Entfernen.
  - `6f7efed` Verwerfen räumt Fehler, Fokus nach Löschen, Zugangsart in Beschreibungen, Portprüfung, Speichern-Namen.
- **Voller `task ci` auf `ec5c123`: EXIT 0** (3. November 2026, Last 8–27): Server-Lint `0 issues.`, Server-Tests 33 Pakete ok (aus Go-Cache), Web 120 Dateien / 1063 Tests, api-client 4, design-tokens 2, Storybook 323, e2e 606 bestanden (33.4 min, 0 flaky), A-9 0 Funde.
- Commits seit `cd1dc75`: `a862b43`, `50d4c74`, `6f7efed`, `7469eb3`, `e4ff097` (Doku), `f3c2f7f`, `ec5c123`.

Belege der Teilläufe vor `ec5c123` (Designer-Runde):

```
web:test                    1058 Tests, 119 Dateien
storybook                   323
```

**Belege K12-3** (Prüfer auf `6e46c6d`):

```
task ci                     EXIT=0
Go                          33 Pakete ok
web:test                    112 Dateien, 844 Tests bestanden
storybook                   290 bestanden
e2e                         510 bestanden (29.0 min), Chromium und Firefox
```

Erhaltungsliste (Abschnitt 7) JOB-1 bis JOB-8, MET-1 bis MET-6 und INT-1 bis INT-8 nachgeprüft.
Dichte aus. Kein Referenzbild von `main` geändert, 63 neue Bilder.

**Belege K12-4** (nach dem Merge auf `ce34f53`):

```
task web:test               874 bestanden
storybook                   295 bestanden
e2e (libraries, folders, folder-browser, podcasts, podcast, metadata, setup)   92 bestanden
e2e Pflege, Titel, Podcasts                                                    52 bestanden
```

**Belege K12-5 bis K12-7** (Agentenläufe `git-master`, Merges auf dem Zweig):

```
K12-5 Stand b802319                    task web:test 906 bestanden, e2e 74 bestanden
Streams (fdc1be7)                      task web:test 895 bestanden, storybook 300, e2e 26
Startseite und Sammlungen (7952385)    task web:test 931 bestanden, e2e 68 bestanden
Spotlight, Empfehlungen, Statistik,
  Sicherung (e1cab40)                  task web:test 976 bestanden, storybook 307,
                                       e2e 44 bestanden (Chromium und Firefox)
Benachrichtigungen (c2a969d)           task web:test 982 bestanden, e2e 28 bestanden
```

Die Zahlen stammen aus den Läufen je Merge, nicht aus einem vollen `task ci`-Lauf. Der
Zugänglichkeitsbaum ist weiter nur über CDP geprüft, nicht mit einem Bildschirmleser.

**Belege Ankündigungen und Aufräumrunde 2:**

```
nach dem Merge 26e5d48 (mit Ankündigungen)   task web:test 996 bestanden, storybook 308, e2e 24
Aufräumrunde 2, task web:test                3 Läufe mit je 117 Dateien, 1034 Tests bestanden
Aufräumrunde 2, storybook                    315 bestanden
e2e vor den Review-Korrekturen (voll)        532 bestanden, 2 gescheitert (Telegram-Fall,
                                             behoben in 3a6ab4f)
e2e nach den Korrekturen                     sechs Specs 72/72, Chromium und Firefox
```

Das volle `task ci` auf `fa81401` steht aus (Prüfer läuft). Der volle e2e-Lauf stammt von vor den
Review-Korrekturen; nach ihnen liefen nur die sechs betroffenen Specs.

**Gemeinsame Korrekturen unterwegs:**

- Fokus in Dialogen bei «Bewegung reduzieren»: react-aria `runAfterTransition` zählt nur das erste
  `transitioncancel`, der Fokus blieb aus (`988d1a6`, `470a49b`). `focusOnMount` fokussiert nun in
  einem Microtask nach dem Commit (`c61fe4b`); der Fokus im Commit selbst warf in den Unit-Tests
  «Should not already be working.».
- Fokus in verschachtelten `FormDialog` (`b593486`): ebenfalls im Microtask, sonst zog der
  äussere Fokusbereich den Fokus zurück.
- e2e-Helfer `markLastTabbable` überspringt `[hidden]` und `[inert]` und fragt `checkVisibility`.
- Firefox malte die Beschriftung der versteckten Diagrammtabellen über den Diagrammtitel; die
  Versteckklasse steht nun auf einem umhüllenden `div` (`c5c190b`, `BarChart`, `HeatGrid`,
  `WatchReport`).
- Neue und erweiterte Bausteine unterwegs: `ui/Tabs` mit `isDisabled`, `RadioGroup` mit `hideLabel`
  und `inline`, `TextField` mit `hideLabel`, `BarChart` mit `tickEvery`, `VisuallyHidden` mit
  `as` und `role`, `SelectCheck`, `StreamTile`, `StreamProgress`, `ProfileList` (geteilt).

**Offen (nach K12-7):**

- Ankündigungen (K12-6): Fokus in `MasterDetail` noch zu beheben.
- Aufräumrunde 2: gemeinsame Helfer `afterRender`, `focusTitleIfLost` und `afterDialogClosed`;
  `useSearchSelection` mit unbekannten Reitern; `SaveBar` klebend; Speicher-Wettlauf in
  Hintergrundjobs («Speichern» immer aktiv, keine Anfrage ohne Änderung, «Verwerfen» nur bei
  Änderung, Eingabe während des Speicherns bleibt, wie auf den anderen Seiten); Test von
  `IntegrationsPage` unter hoher Last; Block der `StatusRegion`; Story für `Tabs isDisabled`.
- K12-8 und K12-9.
- Dialoge mit `autoFocus` in Offline und Player (ausserhalb von Konzept 12).

**Entscheid «Wann geschaut wird»** (Marco, 2. November 2026, Frage 18 Nachtrag): das Fenster
umfasst ganze Wochen, `timeWindowDays` = `min(days, 90)` abgerundet auf ein Vielfaches von 7, also
7, 28 oder 84 (365 Tage rechnen über 84). Sonst wirkte ein Wochentag, der 5-mal statt 4-mal im
Fenster liegt, 25 % stärker. Die Schwelle von 200 ms (11.10) gilt für die Rasterabfrage allein;
über einen Zwischenspeicher entscheidet die Messung in K12-9.

**Abweichungen vom Bauplan** (im Review von K12-2b angenommen, in Frage 18 nachgetragen):
`?tab=` und `?item=` über den Hook `useSearchSelection` (`Tabs` und `MasterDetail` bleiben
darstellend); `useUnsavedChanges` gibt die Props für `ConfirmDialog` zurück und kennt die Option
`keep` für Parameter derselben Seite (`library`, `add`, `days`); zusätzliche Tokens
`section.title-size`, `page.title-weight`, `page.title-tracking`, `dialog.gap`; `SaveBar` mit Rand
und Fläche (die alten Tokens gab es nicht); Hilfe-Knopf in Steuerelement-Grösse (44 px, kompakt
32 px statt 24 px im Mockup).

**Belege K12-2b** (Agentenläufe auf dem Zweig):

```
task web:test              111 Dateien, 805 Tests bestanden
task web:storybook:test    269 bestanden
task gen:check             Exit 0
task web:lint              Exit 0
```

Bekannt: `LoginPage.test.tsx` schlägt vereinzelt mit «document is not defined» fehl (schon vor
Konzept 12 vorhanden, ein Wackler, keine Folge dieser Änderungen). Die Bausteine mit Klappzeilen
sind über den Zugänglichkeitsbaum von Chromium (CDP) geprüft, **nicht mit einem echten
Bildschirmleser**; das ist ein Stellvertreter.

**Belege K12-1** (Agentenläufe auf dem Zweig):

```
task server:lint            0 issues.
task gen:check              ohne Abweichung
go test -race               admin, api, cmd/slouchcat ok
task web:test               grün
```

Messung (`docs/perf/verwaltungsseiten.md`, Entwicklungsrechner i7, 1 Mio. Ereignisse):
`GET /admin/stats` bei 90 Tagen p95 **303.8 → 402.2 ms**; die Rasterabfrage allein bei 84 Tagen
p95 **96.4 ms**. Schätzung für CT 190 (Faktor 2.5): rund 105 ms beim realen Bestand, rund 240 ms
beim Bestand der Messung. Ob ein Zwischenspeicher nötig ist, fällt mit der Messung auf CT 190 in
K12-9.

**Offen (Stand K12-1, überholt, aktueller Stand oben):** Ankündigungen, Aufräumrunde 2, K12-8 und K12-9; Abnahme durch Marco nur in K12-9 auf CT 190 (Desktop mit Maus und Handy
mit 44 px); Release 1.8.0 danach als eigenes Release.

## Konzept 11 «Übersicht der Verwaltung»: umgesetzt und abgenommen, Schritte 0 bis 6 (2. November 2026)

Zweig `claude/verwaltung-uebersicht` ab `main` `43311b0`, Umsetzung nach
`docs/planning/11-verwaltung-uebersicht.md` Abschnitt 11.8 mit OMC (`executor` je Schritt,
`code-reviewer` je Schritt, Befunde vor dem nächsten Schritt behoben). Marcos Auftrag vom
1. November 2026: «Bitte starte mit der Entwicklung von Konzept 11.»

- **Schritt 0** (`cb7cf17`): Frage 17 im Entscheidungsprotokoll, ADR 0019, Papier verankert.
- **Schritt 1** (`4c25656`, `4ea3f0c`, `a0b360a`): Fix `JobCounts.paused` im Handler mit Test;
  Vertrag **1.7.0** additiv mit `JobCounts.active`, `nextRun`, `failedByType`,
  `AdminStream.posterUrl` (Serienposter bei Folgen), `Dashboard.attention.abortedScans` und
  `Dashboard.counts` (60-s-Speicher mit Generationszähler und einer Berechnung je Ablauf, geleert
  bei Änderungen über Pflege, Bibliotheken, Konten, Einrichtung und angenommene Einladungen).
  Review: freigegeben, sechs kleinere Befunde behoben.
- **Schritt 4** (`68d8abd`, `00e6e78`): `ui/AdminSideNav` mit Band, Zahlen und Pillen (ein
  Tab-Halt, Pfeil hoch/runter, Pos1/Ende), `AdminSideNavMenu` (Blatt am Handy), `DashCard`,
  `TaskRow`, `StreamRow`, `ActivityFeed`, sechs Symbole, Stories und 15 neue Referenzbilder;
  Kontrasttest für alle acht Profilfarben (Band und Pillen ≥ 4,5:1, keine Ausnahme nötig). Noch
  nicht im Gerüst; die alte `AdminNav` läuft bis Schritt 5. Review: freigegeben, zwölf kleinere
  Befunde behoben. Entscheid für Schritt 5: die Leiste wird im Gerüst lazy geladen (Startbündel,
  P-10).

- **Schritt 2** (`a54405a`, `75ffc3d`): Aktivitätsprotokoll, Migration **00031** (`activity_events`,
  `STRICT`, `kind` ohne CHECK; `AFTER DELETE`-Trigger auf `accounts` und `profiles` löschen die
  Einträge im selben Statement, ADR 0019 Datenschutz), Regeln gegen Flut je Art (Start 30 min je
  Profil und Titel, `job_failed` je Jobart und Stunde, Scan-Fehler je Bibliothek, Grund und Stunde),
  Haken in `playback`, `library`, `jobs`, `admin` und `main.go`, `backup_restored` beim Start nach
  der Wiederherstellung (`DB.Restored`), Aufräumen über einen Haken in `jobs.prune` (30 Tage, höchstens
  2'000), `Dashboard.activity` im Vertrag, eingefrorener Stand `testdata/db/verwaltung.sql`.
  Abweichungen vom Papier 11.4 (begründet, Review bestätigt): `library_id` INTEGER; Löschen per
  Trigger statt im Go-Code. Review: freigegeben, Befunde behoben.
- **Schritt 3** (`4380fa1`, `c55d6a8`, `e45c002`): `POST /admin/server/restart` (`restartServer`,
  `x-visibility: admin`, 202 `RestartAccepted`, 409 `restart_pending` und `backup_busy`, die
  Wiederherstellung ebenso), `server_restart` mit Konto; `RestartPanel` aus `BackupPage` gelöst und
  allgemein (`useId`, Tests für Aufgeben nach 90 s und «Erneut prüfen»). Noch kein Knopf in der
  Oberfläche (Schritt 5). Review: freigegeben, Befunde behoben.

- **Schritt 5** (`a67fdf9`, `2d52b1f`, `c265df3`, `65cf024`, `00b6880`): Verwaltungsleiste im
  Gerüst statt der App-Leiste auf `/admin*` (lazy geladen mit Rückfall, `data-shell="admin"` aus
  `index.html` vor dem ersten Bild, Breite ab 768 px reserviert, Fokus wandert beim Wechsel der
  Leisten mit), Blatt-Knopf unter 768 px; Übersicht neu mit acht Karten in zwei Spalten, Hinweisen
  in der Reihenfolge von 11.1, «Letzte Aktivität», zugeklapptem Server-Protokoll, «Nach Updates
  suchen» und «Neu starten» mit Rückfrage und `RestartPanel` (409 `restart_pending` zeigt direkt
  die Warteanzeige); Verlust-Bedingung auf «Bibliotheken» wie im Server (`!lossAcceptedAt`);
  `ui/AdminNav`, Bereichskarten und Fehlschlag-Tabelle entfernt, Sprachschlüssel bereinigt.
  Abweichungen (Review bestätigt): Abzeichen A-5 ausserhalb von `/admin` zusätzlich an
  «Verwaltung» der App-Leiste; Karte Webhooks behält ihren Satz statt drei Kennzahlen; Karte
  Server nur mit Fakten, Warnungen zu Grafikchip und inotify in den Hinweisen; Symbol `download`
  für Webhooks; Laufzeit aus `Health.uptimeSeconds`. Review: Änderungen verlangt (Fokusverlust
  beim Leistenwechsel, Major), alle Befunde ausser Dateiaufteilung von `AdminHomePage.tsx` behoben.

**Belege Schritt 5** (`verifier`, HEAD `00b6880`, frischer Lauf):

```
$ task ci                                   EXIT=0
web test:  Test Files  93 passed (93)
web test:       Tests  741 passed (741)
web:storybook:test  212 passed (16.7s)
ci:e2e  498 passed (25.4m)                  Chromium und Firefox, 0 rot
A-9 log scan: 83154 lines of …/server.log, 17 secrets of the run, 0 findings
```

Server: `go test -race` 33 Pakete ok, kein `FAIL`. Fokus-Nachweis in
`web/e2e/admin-home.spec.ts` (Enter auf «Verwaltung» fokussiert «Übersicht», «Zurück zur App»
fokussiert «Start»); Fokusfalle im Blatt mit Tab und Umschalt+Tab geprüft.

**Belege Schritt 2 und 3** (Agentenläufe, frischer lokaler Server):

```
$ curl -X POST …/admin/server/restart
{"restartAt":"2026-11-02T00:08:01.445668948Z"}   HTTP 202
$ curl -X POST …/admin/server/restart
{"code":"restart_pending",…,"status":409}        HTTP 409
Log: restart requested by the administration account_id=1 … restarting … listening startup_ms=1554
GET /admin/dashboard (gleiches Cookie, nach dem Neustart):
  {"kind":"server_started","detail":{"version":"dev"}},
  {"kind":"server_restart","accountEmail":"dod@example.test"}
Nach Scan und Wiedergabe: playback_started, scan_done {"newTitles":22}, scan_started {"trigger":"manual"}
```

`go test -count=1` grün für `admin`, `api`, `auth`, `jobs`, `library`, `playback`, `store`
(eingefrorene Stände, Migration 00031, Wiederherstellung) und `cmd/slouchcat`; `task server:lint`
`0 issues.`; `task server:gen:check` und `task web:gen:check` ohne Abweichung; `task web:test`
93 Dateien, 730 Tests grün.

**Belege Schritt 1 und 4.** Auf dem Zweig mit Schritt 1 und 4 zusammen:

```
$ task web:lint
All matched files use Prettier code style!
$ task web:test
web test:  Test Files  92 passed (92)
web test:       Tests  729 passed (729)
```

Schritt 1 (Agentenlauf): `task ci:web` grün; `task ci:server` grün bis auf
`internal/library` `TestPodcastFolderPairsFileWithFeedEpisodeAfterProbe`. Ursache: veraltete lokale
Test-Medien (`testdata/media/Podcasts/episode-*.mp3` vom 20. Oktober ohne `date`-Tag; der Generator
überspringt vorhandene Dateien); neu erzeugt, seither grün. Kein Codefehler, die CI erzeugt die
Medien bei jeder Änderung des Generators neu;
nach den Review-Befunden `go test` für `admin`, `api`, `auth`, `store` ok,
`golangci-lint` `0 issues.`, `task server:gen:check` ohne Abweichung. Schritt 4 (Agentenlauf):
`task web:storybook:test` in Docker `215 passed`. Beispielantwort von `GET /admin/dashboard` aus
dem Handlertest: `jobs.active`, `jobs.failedByType` mit geschwärztem Fehler, `jobs.nextRun`,
`attention.abortedScans`, `counts {accounts, maintenanceOpen, titles}`, `streams[].posterUrl` mit
dem Hash des Serienposters. e2e lief noch nicht (keine Änderung der Oberfläche bis Schritt 5).

- **Schritt 6** (Abnahme und Doku): CT 190 läuft seit 2. November 2026, 08:01 Uhr, auf
  `ghcr.io/slouchcat/slouchcat:verwaltung-955709b` (`v1.6.0-58-g955709b`), gebaut auf dem CT aus
  `git archive 955709b`, davor `1.6.0`, Sicherung `.env.bak-vor-verwaltung`. Migration 00031 in
  235 ms nach der Datenbanksicherung `pre-migration-v1.6.0-58-g955709b-…`, Start 2'372 ms, `healthy`
  nach rund 4 s, QSV erkannt (`docs/perf/verwaltung.md` 1). **Von Marco abgenommen am
  2. November 2026** («Abgenommen»). Versionsstellen auf 1.7.0 (`deploy/.env.example`,
  `deploy/compose.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`);
  Release-Notes als `release-notes-1.7.0.txt` im Wurzelordner (unversioniert wie 1.5.0 und 1.6.0,
  Beschriftung des Tags). `docs/perf/measure-home.sh` kennt neu `SLOUCHCAT_PAUSE` für den kalten
  Fall. Papier als umgesetzt markiert.

**Messung** (`docs/perf/verwaltung.md` 2, CT 190, 2. November 2026): `GET /api/v1/admin/dashboard`
warm p95 6.0 ms (n=60), kalt p95 6.1 ms (n=20, 61 s Pause, `counts` jedes Mal neu gerechnet);
Latte 200 ms, der Rückfall aus 11.9 entfällt.

**Prototyp gegen gebaute Seite** (`docs/planning/mockups/11-verwaltung-prototyp.html`, Variante B,
Desktop, gegen die Aufnahme des e2e-Servers bei 1440 px): gleich sind Leiste mit Band-Gruppentiteln,
Zahlen und Pillen, Reihenfolge und Raster der acht Karten, Kopf mit «Live, alle 5 Sekunden»,
Aufgaben mit drei Kennzahlen, zugeklapptes Server-Protokoll. Abweichungen in der Gestaltung der
Karteninhalte (abgenommen, keine Funktionslücke):

- Server: Fakten ohne getönte Kästchen; Update-Stand als Zeile «Updates: Aktuell» statt Pille
  «aktuell» mit «Geprüft heute, …».
- Jetzt läuft, Webhooks: Leerzustand als Satz ohne getönten Kasten mit Symbol; Webhooks ohne
  Erklärsatz zu Radarr und Sonarr.
- Integrationen: Liste «Name: Zustand» in einer Spalte statt Chips in zwei Spalten.
- Speicher: Legende «Belegt» und Zeile «Frei: … von …» statt Kopfzeile «x GB von y GB» und
  «Frei · % belegt».
- Kartenlinks unterstrichen (`TextLink` wie in der übrigen App) statt Akzentfarbe mit Pfeil.

**Offen:**

- `activity_events` nach einem Tag (3. November 2026) auslesen; Ausgangsstand 7 Zeilen
  (`docs/perf/verwaltung.md` 3).
- Push und Pull Request auf Marcos Wort; beim Merge Tag `v1.7.0` (setzt Marco).

## Nachtrag Konzept 10: «Nur Bilder» bei Ordnern mit einer Quelle (1. November 2026)

Zweig `claude/fix-apply-images-single-source` ab `main` `da5d854` (Merge von PR #58). Befund vom
`apply`-Lauf auf CT 190 nach dem Merge: die Hintergrund-Kette auf `["", "de", "en"]` gestellt und
«Nur Bilder» gestartet (Lauf 124231, 2'095 Titel) – alle `metadata.fill`-Kinder endeten in einer
Sekunde mit `item provenance recorded … fields=0`, keine TMDB-Anfrage, kein Bild neu entschieden.
Ursache: `fill` kehrt bei einem Ordner mit einer Quelle vor jedem Abruf zurück (`recordIdentifier`,
Papier 1.1); Bilder entschied bis dahin nur der Match (`matchImages`). Die Bibliothek «Filme» auf
CT 190 hat nur TMDB. «Texte und Bilder» (Lauf 128425, Match je Titel, rund 6 min) setzte die Kette
dann um; das ist die Umgehung auf dem Stand 1.6.0.

**Gebaut.** `itemPayload.Images` (Paket `metadata`), `handleFill` ruft damit `fillImages`, das die
Ein-Quellen-Abkürzung überspringt und die Datensätze holt; `metadata.apply` reiht das Kind mit dem
Flag und dem Schlüssel `fill:images:<id>` ein (neben einem wartenden `fill:<id>` des Matches, dessen
Payload das Flag sonst nicht bekäme). Der gewöhnliche `fill` und alle anderen Aufrufer sind
unverändert; Vertrag unverändert (1.6.0). Papier Abschnitt 14 nachgeführt.

**Belege.** Ohne Fix schlägt `TestApplyImagesRedecidesWithOneProvider` fehl (Hintergrund bleibt
der ohne Text, obwohl die Kette `en` zuerst nennt); mit Fix:

```
$ go test ./internal/metadata/
ok  	github.com/slouchcat/slouchcat/server/internal/metadata
$ go tool golangci-lint run ./internal/metadata/... ./internal/admin/... ./internal/api/...
0 issues.
```

`task ci` lief in dieser Sitzung nicht lokal (der Entwicklungsrechner hatte am 1. November 2026 keinen
Zugriff auf das Arbeitsverzeichnis unter `/mnt/Nerdcloud`, Klon unter `~/src/slouchcat`); der
CI-Lauf des Pull Requests war der Beleg (PR #59, 17 Checks grün, gemergt als `6d562b8`).

**Beleg auf CT 190 (1. November 2026, 16:25 bis 16:32).** CT 190 auf `metadaten-6d562b8`
(`v1.5.0-21-g6d562b8`, auf dem CT gebaut aus `git archive 6d562b8`, Sicherung `.env.bak-vor-fix-6d562b8`,
keine Migration). «Nur Bilder» auf der Bibliothek «test» (304 Titel, nur TMDB) mit der Logo-Kette
testweise auf `en, de` (Lauf 137118), dann zurück auf `de, en` (Lauf 137853):

```
item filled … providers=["tmdb"] fetched=1      329 bzw. 279 Zeilen, 0 «provenance only»
Toy Story 4, Over the Hedge: Logo de → en → de
job failed 0, level=ERROR 0
```

Vor dem Fix endeten dieselben Kinder mit `item provenance recorded … fields=0` und ohne Anfrage.
Die Einstellungen stehen danach wieder wie vor dem Beleg.

## Konzept 10 «Metadaten-Einstellungen»: umgesetzt, Pull Request offen (31. Oktober 2026)

Zweig `claude/festive-swanson-7af516` ab `main` `d3d73bc` (Merge von PR #57, Konzept 10). Umsetzung
nach `docs/planning/10-metadaten-einstellungen.md` Abschnitt 10, ein Commit je Schritt, `task ci`
grün vor jedem Abschluss (Storybook-Vergleich ab Schritt 5 lokal mit Docker, siehe je Schritt). Marco hat
am 31. Oktober 2026 alle Fragen wie vorgeschlagen beantwortet (Frage 16) und die Umsetzung
beauftragt. Alle acht Schritte (0 bis 7) sind committet (PR #58); CT 190 läuft auf dem Kopf des Zweigs,
der `apply`-Lauf dort ist gemessen. Offen für Marco: Merge und Tag `v1.6.0` (Vertrag 1.6.0,
Release-Notes-Entwurf unten). Nicht mergen; Marco entscheidet.

### Schritt 0 — Entscheide und Papiere (`b0649e4`, 31. Oktober 2026)

Reiner Doku-Commit: Frage 16 in `docs/planning/04-entscheidungen.md` (Tabellen 11.1 und 11.2 des
Papiers, Vermerk zu Entscheid 6 mit der Wirkung auf neue Titel), ADR 0018
`docs/adr/0018-metadaten-sprachen-bilder-freigabeland.md` als Teilnachfolger von ADR 0015 (Abschnitt
«Bilder») und Ergänzung von ADR 0006 (Trailer-Sprachen) mit Hinweisen in beiden Status-Zeilen, Idee 01
auf «Übernommen (Teil B)» mit der Restliste aus Abschnitt 1.3 des Papiers, `docs/ideas/README.md`,
Kommentar an Issue #8 (`issuecomment-6541846035`), Papier auf `d3d73bc` verankert mit neuem Abschnitt
14 «Abweichungen während der Umsetzung». Dieser Statuseintrag kam erst mit Schritt 1 in die Datei
(der Schreibvorgang in Schritt 0 landete nicht im Commit).

DoD-Beleg `task ci` auf dem unveränderten Code von `main`:

```
$ task ci
… ci:server grün, ci:web: web test FAIL src/features/offline/OfflinePlayer.test.tsx
  > stops while playing once the rest of the day is played (1159 ms, Timer-Flake)
$ cd web && npx vitest run src/features/offline/OfflinePlayer.test.tsx
 Test Files  1 passed (1)   Tests  15 passed (15)
```

Der eine rote Test war ein Zeitflattern des Offline-Players unter Last (`ci:server` lief parallel);
allein läuft er grün. Im CI-Lauf von Schritt 1 war er grün (679 passed).

### Schritt 1 — Sprachen (31. Oktober 2026)

**Was gebaut ist.** Migration `00028_metadata_languages.sql` nach Vorlage: `item_texts` und
`item_genres` neu ohne `CHECK (lang IN ('de','en'))`, `item_texts.rank` (Bestand `de` 0, `en` 1),
Indizes `item_texts_sort_key` und neu `item_texts_rank`, die Trigger `search_texts_*` und
`items_genres_*` neu (die Genre-Trigger lesen die Schlüssel des `genres`-Objekts statt fest
`'de'`/`'en'`), `profiles.metadata_language`. Paket `metadata`: `settings.go` (die zwölf Schlüssel
`metadata.*` als `Settings` mit Vorgaben, `Validate`, `ValidateTextLangs`, `Effective` noch ohne
Bibliothek, `AvailableMetadataLanguages`, `ProviderLanguages`), `languages.go` (statische BCP-47-Liste:
ISO 639-1 plus TMDB-Regionsvarianten, `KnownLanguage`), `textLangs` an allen sechs Stellen des Match
und im `fill` durch die wirksame Liste ersetzt, `rank` je Zeile, `pruneTexts` (Löschregel 6.1),
`genresJSON` statt `genres{DE, EN}`, TVDB-Sprachtabelle aus `/languages` mit statischem Rückfall,
Trailer-Kette in den primären Untersprachen der Liste, Feed-Texte in jeder Sprache der Liste.
Paket `visibility`: `Scope` trägt Metadaten- und Oberflächensprache, `pickTexts`/`pickGenres` mit
der Kette «Metadatensprache → Oberflächensprache → Zeilen nach Rang», Sortierschlüssel der
Bibliotheksseite und Genrefilter in SQL mit beiden Sprachen, Verwaltungssuche und System-Abfragen
(`admin.sql`, `admin_stats.sql`, `spotlight.sql`, `collections.sql`) mit der Rang-Zeile als dritter
Stufe. Profil: `metadata_language` im Modell, `updateProfile` prüft gegen die verfügbaren Sprachen
(422 `metadataLanguage: invalid_value`, `""` setzt zurück), `getSession` liefert
`availableMetadataLanguages`. Vertrag **1.6.0** (additiv): `Profile.metadataLanguage`,
`ProfileUpdate.metadataLanguage`, `SessionInfo.availableMetadataLanguages`, Schema
`MetadataLanguage`; `MetadataProviderInfo.languages` nennt je Quelle die Sprachen der Serverliste,
die sie liefert. Abweichungen vom Papier mit Begründung in dessen Abschnitt 14.

**Tests** (neu): `TestMigration00028KeepsTriggersAndOpensTheLanguages` (Trigger, Indizes, Ränge,
Suche nach einem `es`-Text, Genre-Trigger mit drittem Schlüssel, Profilspalte),
`TestKnownLanguages`, `TestSettingsDefaultsParseAndValidate`,
`TestMatchFetchesEveryLanguageOfTheListAndPrunesTheRest` (der Blocker-Fall: drei Sprachen über
einen `[tmdb]`-Ordner, drei TMDB-Anfragen, Ränge 0/1/2, Genres mit drittem Schlüssel; Liste
schrumpft → `es`-Zeile weg; gesperrtes Feld hält jede Zeile), `TestAvailableAndProviderLanguages`,
`TestPickTextsChainWithThirdLanguage`, `TestThirdLanguageThroughTheReader` (Detail, Rückfall
feldweise, Genrefilter und Facette in der dritten Sprache, Buchstabe, Entdecken-Genres),
`TestContractMetadataLanguage` (Session, 422, Wechsel, Rücksetzen). Angepasst:
`TestLibraryItemsLetters` (P6 ohne englischen Schlüssel nimmt jetzt den deutschen am Rang 0 statt
des Dateinamens, das ist die Kette), `TestSetupFlowEndToEnd` (Session trägt die Sprachen).

DoD-Belege:

```
$ task ci                                   (Zweig b0649e4 + Schritt 1, alles gestaged)
server lint:   0 issues.   itemsaccess, go vet, testgroups --check grün
server test:   ok  … 33 Pakete (admin 483.8 s, api 440.7 s, store 306 s auf dem NAS-Mount), kein FAIL
web gen:check, lint grün; web test:  Test Files  85 passed (85)   Tests  679 passed (679)
storybook:test: permission denied … docker.sock  (kein Docker in dieser Umgebung, wie bei Konzept 07 bis 09;
                Schritt 1 ändert keine Komponente aus web/src/ui, Baselines unberührt; CI auf GitHub prüft sie)
$ task build   go build … -X main.version=v1.5.0-8-gb0649e4-dirty -o bin/slouchcat   exit=0
$ task ci:e2e  479 passed (25.4m), 1 failed: [firefox] admin-streams.spec.ts:39 «its player stops within 5 s»
$ task e2e -- e2e/admin-streams.spec.ts   1 passed (chromium), 1 failed (firefox, derselbe 5-s-Fall)
$ go test ./internal/metadata/ -run 'TestKnownLanguages|TestSettingsDefaults|TestMatchFetchesEvery|TestAvailableAndProvider'  ok
$ go test ./internal/visibility/ -run 'TestPickTexts|TestThirdLanguage|TestLibraryItems'   ok
$ go test ./internal/api/ -run 'TestContractMetadataLanguage|TestContractProfileLanguage|TestSetupFlowEndToEnd|TestAuthorize'  ok
$ go test ./internal/store/ -run 'TestMigration00028'   ok
```

Der eine rote e2e-Fall ist das bekannte 5-Sekunden-Zeitbudget von `admin-streams.spec.ts` in Firefox
(schon bei Konzept 08 und 09 als zeitabhängig und ohne Bezug notiert; `data-ended-reason` kommt in
Firefox nach dem Beenden durch den Administrator zu spät). Er berührt keine Zeile dieses Schritts
(Wiedergabe, nicht Metadaten) und war im Chromium-Lauf grün.

**Messung `listLibraryItems`** (Papier 8, Latte ±10 %): `docs/perf/metadaten.md` Abschnitt 1.
Fünf Läufe (main zweimal, Zweig mit de+en zweimal, einmal mit `es` als dritter Sprache, je 50'570 Titel
und 90'802 bzw. 136'203 Textzeilen): die schweren Serienfälle (rund 190 ms p95) liegen in allen Läufen
innerhalb von 2 %, die leichten Filmfälle schwanken zwischen Läufen desselben Binaries stärker (p50 20 bis
48 ms) als zwischen den Binaries. Lauf 1 des Zweigs zeigte den doppelten `item_texts`-Join bei gleicher
Metadaten- und Oberflächensprache; der zweite Join entfällt seither in diesem Fall.

### Schritt 2 — Bilder im Scanner, im `fill` und in der Pipeline (31. Oktober 2026)

**Was gebaut ist.** Migration `00029_item_images.sql` nach Vorlage: `images` neu mit `kind` um
`logo`, `format` um `png`, `provider` (Rückfüllung nach dem Host der `source_url`); `item_images`
(entschiedene Quelle je Titel und Art, aus `tmdb_poster_path`/`tmdb_backdrop_path` gefüllt,
`lang = ''`, Quelle nach Host); `item_local_images` (leer, der nächste Scan füllt sie; mit
`root_id`, Abschnitt 14 des Papiers). Kandidaten je Quelle: TMDB über `append_to_response=images`
mit `include_image_language=<Vereinigung der Ketten>,null` am Detail-Abruf der `recordLang`-Sprache
und an der Staffel (`tmdb.go` `Images`, `imageQuery`; `tmdbProvider.imageLangs` liest die
Einstellungen je Aufruf), TVDB-Artworks mit `language`, `width`, `height` (Typen 2, 3, 23;
Sprache über die Tabelle aus `/languages`), AniList Cover und Banner als Vorgabe ohne Sprache,
lokale Dateien aus `item_local_images` als `file:<rel>?<size>-<mtime>`. Die Wahl in sechs Stufen
(`images_choice.go`: `chooseImage`, `decideImages`, `imageSources` mit der festen Reihenfolge
`["local", …Ordnerliste, "fanart"]`, `writeImageDecisions`), eingehängt in den **Match**
(`matchImages`: der Erkenner als einzige Quelle, auch für Staffelposter und Folgen-Standbild) und
in den **Fill** (`writeFill`); Match und Fill schreiben die zwei Pfadspalten nicht mehr. Pipeline:
`image.fetch` liest `item_images` (`imageSource`: TMDB-Pfad, `.svg` → `.png`, TVDB/AniList/Fanart-
Hosts, `file:` über die Wurzel der Scanner-Zeile), `logo` in `itemImageKinds` für Film und Serie
(Breiten 400/800), Formate **je Art** (`formatsOf`: Logo PNG und WebP, kein JPEG;
`transcode.ImagePNG` mit `-c:v png -f image2 -update 1`, Alphakanal bleibt), `scaleLocked` behält den
Medientyp des Originals, Sperre **mit** Wahl (`chosen = 1`) holt genau diese Quelle, Sperre **ohne**
Wahl skaliert nur, eine ausgeschaltete Art (`metadata.image_kinds`) verliert Zeilen und Dateien
beim nächsten Lauf (`dropKind`), eine unbrauchbare Quelle (`errImageSkipped`) überspringt die Art
mit Warnung statt den Job zu brechen. Scanner: `walkRoot` und die Pfad-Scans sammeln Bilddateien je
Ordner, `syncLocalImages` (`library/local_images.go`) ordnet sie nach 4.6 zu (Film im eigenen
Ordner: `poster` > `folder`, `fanart` > `backdrop`, `logo` > `clearlogo`; Film direkt in der Wurzel
nur `<stem>-…`; Serie über den Serienordner; Staffel `seasonNN-poster`; Folge `<stem>-thumb`;
`landscape` nie), liest nur den Header über die Wurzel (JPEG, PNG, eigener WebP-Header-Leser) und
pflegt die Tabelle (Ersetzung über Grösse/Änderungszeit, Entfernung). Vertrag (weiter 1.6.0,
additiv): `ImageKind` um `logo`, `ItemImages.logo` (vom Server nur in `getItem` gesetzt),
`getAdminItemImage` nimmt `ImageKind`; `getItemImage` liefert Logos in `w400`/`w800` als WebP oder
PNG (Rückfall je Art in `GetItemImage`/`AdminGetItemImage`). `ListItemsMissingImages` liest
`item_images`. Abweichungen vom Papier mit Begründung in dessen Abschnitt 14.

**Tests** (neu): `TestMigration00029CarriesTheImagePathsIntoItemImages`, `TestChooseImageSteps`
(die sechs Stufen, Stimmenregel, Mindestbreite, «TVDB-de schlägt TMDB-en», zu kleine lokale Datei
verliert), `TestDecideImagesHonoursLocksChoicesAndSwitches` (Sperre, Wahl, Schalter, «local bleibt
nach dem zweiten fill», lokale Dateien aus), `TestMatchAndFetchWithLogoAndLanguageChain` (deutsches
Poster vor englischem, Hintergrund ohne Text, SVG-Logo als PNG geholt, PNG mit Alphakanal und WebP
abgeleitet, kein JPEG, Logo-Schalter aus löscht Zeilen und Dateien),
`TestLocalImageBeatsTheProviders` (lokale `poster.jpg` über die Wurzel, zweiter Fill behält sie,
Schalter aus gibt TMDB), `TestScanListsLocalImagesOfMovies`, `TestScanListsLocalImagesOfSeries`,
`TestLocalImageOutsideTheRootIsNeverRead` (Symlink aus der Wurzel). Angepasst: `fill_test`
(Poster-Herkunft `tvdb`, der Fixture-Fall des Papiers), `series_test` und `fill_test` lesen
`item_images` statt der Spalten.

DoD-Belege:

```
$ git add -A && task ci                     (Zweig e8290b4 + Schritt 2, alles gestaged)
server lint:   0 issues.   itemsaccess, go vet, testgroups --check grün
server test:   ok  … 33 Pakete (store 720 s, admin 603 s, api 573 s auf dem NAS-Mount), kein FAIL
web gen:check, lint grün; web test:  Test Files  85 passed (85)   Tests  679 passed (679)
storybook:test: permission denied … docker.sock  (kein Docker, wie bei Schritt 1; Schritt 2 ändert keine
                Komponente aus web/src/ui, Baselines unberührt; CI auf GitHub prüft sie)
$ task build   go build … -X main.version=v1.5.0-9-ge8290b4-dirty -o bin/slouchcat   exit=0
$ task ci:e2e  480 passed (25.3m)   A-9 log scan: 77338 lines, 17 secrets of the run, 0 findings
$ go test ./internal/metadata/ -run 'TestChooseImageSteps|TestDecideImages|TestMatchAndFetchWithLogo|TestLocalImageBeats'  ok
$ go test ./internal/library/ -run 'TestScanListsLocalImages|TestLocalImageOutsideTheRoot'   ok
$ go test ./internal/store/ -run 'TestMigration00029'   ok
$ bash nas-scan.sh bin/slouchcat step2     == step2 scan: 1:done:1789 in 220.4 s (kalt) / 6.2 s (warm)
                                            local_images ('backdrop', 207) ('logo', 119) ('poster', 210); total 536, titles 211
$ bash nas-scan.sh bin-main main           == main scan: 1:done:1789 in 8.2 s / 4.2 s (warm)
```

Das Logo im Cache (DoD «ein Titel mit Logo im Cache», Papier 10 Schritt 2) belegt
`TestMatchAndFetchWithLogoAndLanguageChain` im Paket `metadata`: der Erkenner liefert ein SVG-Logo,
`image.fetch` holt es als PNG (`.svg` → `.png`), legt `logo/w400.png`, `logo/w400.webp`,
`logo/w800.png`, `logo/w800.webp` ab (kein JPEG), und der Alphakanal überlebt die Ableitung
(`transcode.ImagePNG`). Ein Logo aus der echten TMDB (ohne Schlüssel lokal nicht möglich) folgt in
Schritt 7 auf CT 190.

**Messung NAS-Scan** (Papier 8, Latte «vernachlässigbar»): `docs/perf/metadaten.md` Abschnitt 2.
Vier Läufe über `/mnt/nerdnas/medien/Filme` (1'789 Filmordner): der erste, kalte Lauf (220 s) wärmt den
Verzeichnis-Cache des NAS, unabhängig vom Binary; mit warmem Cache braucht der Zweig 3.6 s statt 3.2 s
(`main`), also 0.4 s für die Header von 536 Bilddateien in 211 Ordnern (`folder.jpg`, `backdrop.jpg`,
`logo.png`).

### Schritt 3 — Einstellungen, Vertrag, `metadata.apply`, Freigabeland (31. Oktober 2026)

**Was gebaut ist.** Migration `00030_library_metadata.sql` (Abweichung je Bibliothek: `text_langs`,
`images` als JSON, `NULL` = wie Server). Paket `metadata`: `LibraryDeviation` (Lesen, Prüfen,
Speichern, Entfernen bei «alles wie Server»), `Effective(libraryID)` legt die Abweichung über den
Server-Standard, `AvailableMetadataLanguages` ist die Vereinigung aller Listen (Server zuerst),
`UpdateSettings` schreibt die zwölf Schlüssel auf einmal nach `Validate`; die Sicht für die
Verwaltung in `settings_api.go` (`MetadataSettings`, `UpdateMetadataSettings`, `LibraryMetadata`,
`UpdateLibraryMetadata`, `Apply`; mit `affectedTitles`, `localImageTitles`, `knownLanguages`). Job
`metadata.apply` (`apply.go`): Ereignis-Jobart im Register (Gruppe `metadata`, Pool `general`,
Priorität 1, kein `Run`), Payload `{libraryIds, what}`, Idempotenzschlüssel je Umfang; der Handler
reiht je gematchtem Film und je Serie `metadata.match` ein (`what` mit `texts`) oder `metadata.fill`
und `image.fetch` (`what = ["images"]`), hebt wartende Kinder an (`RaiseQueuedPriority`) und meldet
«n Titel eingereiht» als Fortschritt. Freigabeland: `ratingOrder(country)` stellt das gewählte Land
vor die Kette DE → AT → CH → GB → US, `fskFromCertificationsFirst` liest es zuerst; Match und Fill
lesen das Land aus den wirksamen Einstellungen; ein Länderwechsel senkt eine gespeicherte Stufe
derselben Quelle nicht (K-3, Papier 14). `metadata.tmdb_include_adult`: `Client.Search` sendet
`include_adult=true`, wenn gesetzt (Titelsuche des Matches und Verwaltungssuche). Vertrag 1.6.0
(additiv) um fünf Operationen mit `x-visibility: admin`: `getMetadataSettings`,
`updateMetadataSettings` (`PUT /admin/settings/metadata`), `getLibraryMetadata`,
`updateLibraryMetadata` (`/admin/libraries/{id}/metadata`), `applyMetadata`
(`POST /admin/metadata/apply`, 202 `{jobId, titles}`); Schemata `MetadataSettings`,
`MetadataSettingsUpdate`, `ImageLangs`, `LibraryImages`, `LibraryMetadata`,
`LibraryMetadataUpdate`, `ApplyMetadataRequest`, `ApplyMetadataResponse`. Verwaltung (`admin`)
reicht die fünf Operationen an `metadata` durch; Handler in
`api/metadata_settings_handlers.go`; Einträge im Vertragstest (`adminRegistry`).

**Tests** (neu): `TestMigration00030CreatesLibraryMetadata`, `TestEffectiveSettingsWithLibraryDeviation`
(Abweichung wirkt nur auf die Bibliothek, Vereinigung der Sprachen, 422 `ui_lang_required` und
`invalid_value`, Rücksetzen, 404, Speichern der Serverliste mit Pflicht `de`),
`TestApplyQueuesChildrenWithoutDoubles` (Zahl der Titel je Umfang, ein Lauf je Umfang, ein Kind je
Titel, `images` ohne Match, kein Doppel, Register-Eintrag als Ereignis-Jobart),
`TestRatingCountryReadFirst` (Reihenfolge, Neuzugang bekommt die Stufe des zuerst gelesenen
Landes, Länderwechsel senkt nicht; die zwei Fälle aus Papier 9.1 als Metadaten-Tests, Papier 14).
`TestContractCoversEveryOperation` und `TestContractAdminOperations` decken die fünf Operationen.
Eingefrorener Stand `testdata/db/metadaten.sql` (Migrationen 00028 bis 00030 in einem Stand, Papier 14)
mit `TestFrozenMetadatenStateCoversTheConcept`; der Eintrag in `testdata/db/README.md` nennt Herkunft und
Daten. Zwei Nachbesserungen aus dem CI-Lauf: `TestRegisterTypesSchedules` zählt jetzt neun Handler
(`metadata.apply` kam dazu), und die Regel «ein Länderwechsel senkt nicht» greift nur noch bei einer
**echt tieferen** Stufe; bei gleicher Stufe nimmt die Quelle das neu gelesene Land an
(`TestAssignMatchAndRefresh`: ein neu zugeordneter Titel trug sonst die Quelle `tmdb:DE` seines
Vorgängers; Papier 14 nachgeführt).

DoD-Belege:

```
$ git add <Schritt 3> && task ci                (Zweig 50a74f0 + Schritt 3 gestaged, Schritt 4 im Arbeitsbaum;
                                                Schritt 4 ändert keine Server-Datei)
server lint:   0 issues.   itemsaccess, go vet, testgroups --check grün; server:gen:check grün
server test:   ok  … 33 Pakete (api 419.7 s, playback 317.8 s, metadata 311.3 s, library 297.3 s;
               admin, store aus dem Cache des abgebrochenen Laufs mit demselben Code), kein FAIL
$ go build ./... && go vet ./internal/{metadata,admin,api,store}/   auf dem gestagten Baum allein (git archive)
                                                → STEP3-INDEX-BUILDS
$ bash freeze-metadaten.sh   migration applied 28/29/30; rows: 3 item_images, 1 library_metadata, 1 es texts
                             → testdata/db/metadaten.sql (1607 Zeilen)
$ go test ./internal/store/ -run 'TestFrozenMetadaten|TestFrozenStatesMigrate' -count=1   ok (127.0 s)
$ go test ./internal/metadata/ -run 'TestAssignMatchAndRefresh|TestRegisterTypesSchedules|TestRatingCountryReadFirst' -count=1   ok
```

Der Web-Teil von `task ci` (Lint, Komponenten- und Storybook-Tests) folgt mit Schritt 4: das Web
dieses Commits ist nur der generierte Client (`api-client`-Lint lief grün im selben Lauf).

### Schritt 4 — Oberfläche: Seite «Metadaten», Bibliothek, Profil, Logo beim Zuschauer (31. Oktober 2026)

**Was gebaut ist.** Neue Verwaltungsseite `/admin/metadata` (`web/src/features/admin/MetadataPage.tsx`,
Sektion «Metadaten» in der Gruppe Bibliothek mit dem neuen Icon `image`): **Geltung** als Segmente
«Standard | <Videobibliothek> …» (über fünf Bibliotheken das `SelectField`), bei einer Bibliothek die
Schalter «Sprachen abweichend» und «Bilder abweichend», nicht abweichende Abschnitte gedimmt mit «wie
Server», die globalen Abschnitte ausgeblendet; **Sprachen** als geordnete Chips (Position, nach vorne,
nach hinten, entfernen; die letzte Oberflächensprache ist nicht entfernbar, E-9), «Sprache wählen»
über `Intl.DisplayNames`, darunter welche Quelle welche Sprache nicht kennt und die YouTube-Rechnung
(100 Suchen am Tag geteilt durch die Zahl der Grundsprachen); **Bilder**: Poster und Hintergrund
«immer», Logo, Staffelposter, Folgenbild als Kontrollkästchen, je Art die Bildsprache-Kette mit «Ohne
Text»; **Güte** (Mindestbreiten, «Nach Bewertung wählen»), **Quellen** (TMDB-Erwachseneninhalte,
lokale Dateien mit Zahl der Titel, Fanart.tv mit Link auf Integrationen), **Altersfreigabe** (Land),
**Anwenden** (Was, Wo mit Titelzahl, «Jetzt anwenden», «Noch wartend» und letzter Lauf aus
`listJobTypes`); klebende **Speicherleiste** mit «Ungespeicherte Änderungen» / «Gespeichert»,
«Verwerfen», nach dem Speichern «n Titel betroffen – jetzt anwenden?». Feldfehler des Servers
(`langs_empty`, `lang_unknown`, `ui_lang_required`, `chain_duplicate`, `country_unknown`) über
`formErrors` am Abschnitt. Route mit `?library=<id>` (`validateSearch`). Bibliothekskarte: Zeile
«Metadaten: wie Server» / «Sprachen abweichend» / «Bilder abweichend» mit Link auf die Seite.
Profil: Feld «Sprache der Beschreibungen» (nur mit dritter Serversprache oder gesetztem Wert, erster
Eintrag «Wie Oberfläche», die Oberflächensprache selbst fehlt in der Liste), Chip auf der Profilkarte.
Zuschauer: `TitleOrLogo` (`web/src/ui`) setzt das Logo ins `<h1>` der Detailseite und in die
Spotlight-Folie, `alt` = Titel, Rückfall auf den Text bei Ladefehler (offline). Texte de/en, keine
harte Farbe. Stories `UI/TitleOrLogo` (Text, Logo, kaputtes Logo) und eine Spotlight-Folie mit Logo.
Abweichungen vom Papier mit Begründung in dessen Abschnitt 14 (sechs Zeilen «4»).

**Tests** (neu): `MetadataPage.test.tsx` (sieben Fälle: Standard, Sprache hinzufügen/verschieben/
speichern, Feldfehler, Bibliotheksabweichung mit `null` für «wie Server», Wechsel der Geltung,
«Jetzt anwenden», kein Administrator), zwei Fälle in `ProfileSettingsPage.test.tsx` (Feld nur mit
dritter Sprache), `web/e2e/admin-metadata.spec.ts` (Regel 8: Standard, dritte Sprache per Tastatur
hinzufügen, verschieben, speichern, entfernen; Land mit Pfeiltasten und «Verwerfen»; Bibliothek
weicht ab, Karte zeigt es, Link führt zurück; «Jetzt anwenden»; keine Fokusfalle), ein Fall in
`profiles.spec.ts` (dritte Serversprache, Pfeiltasten, Speichern, Karte, Rücksetzen), `NAV_PAGES` in
`admin-home.spec.ts` um «Metadaten».

DoD-Belege:

```
$ task ci:web                (Zweig 546af07 + Schritt 4)
web:gen:check, tokens grün; web lint: eslint, stylelint (nach Korrektur der Medienabfrage in
               HeroCarousel.module.css), tsc grün; prettier --check: All matched files use Prettier code style!
web test:      Test Files  1 failed | 85 passed (86)  (AdminHomePage: die neue Karte «Metadaten» fehlte
               in den zwei erwarteten Listen des Tests; nachgeführt)
$ cd web && pnpm exec vitest run          Test Files  86 passed (86)   Tests  688 passed (688)
$ pnpm -w run lint                        web lint: Done; All matched files use Prettier code style!
$ pnpm run storybook:build                Storybook build completed successfully
storybook:test: braucht Docker (nicht auf dem Entwicklungsrechner); die neuen Stories UI/TitleOrLogo und
               die Spotlight-Folie mit Logo bekommen ihre Baselines auf CT 190 oder in CI (Papier 14)
$ task build && task e2e -- e2e/admin-metadata.spec.ts e2e/profiles.spec.ts
               32 passed, 2 failed (Fall «dritte Serversprache»: Pfeil traf zuerst «Englisch»; Test nachgeführt)
$ task e2e -- e2e/profiles.spec.ts -g "third server language"   2 passed (chromium, firefox)
```

Der erste e2e-Lauf meldete «kein sichtbarer Fokus» an der Sprachauswahl, weil `task e2e` das zuvor
gebaute Bündel ausliefert (das alte native `<select>` mit einer nicht definierten Farbvariable im
Fokusring); nach `task build` mit dem `SelectField` ist der Ring da. `web:gen:check` lief im zweiten
Durchgang nicht mehr, weil der Arbeitsbaum schon die Vertragsänderung von Schritt 5 trug; deshalb die
Komponententests und der Lint einzeln (gleicher Stand des Web-Codes).

### Schritt 5 — Bildwahl je Titel (31. Oktober 2026)

**Was gebaut ist.** Paket `metadata`, `image_candidates.go`: `ImageCandidates` fragt live die
Bilddateien neben dem Medium (`item_local_images`, zuerst), TMDB ohne Sprachfilter
(`Client.AllImages`: `/movie|tv/{id}/images`, `/tv/{id}/season/{n}/images`) und die TVDB-Artworks
einer Serie (`tvdbProvider.Artworks`), sortiert je Quelle nach der Sprachkette der Art, dann
Bewertung, Stimmen und Breite; Vorschau beim Anbieter in `w300` (ein SVG-Logo als PNG-Zwilling),
lokal über den neuen Vorschau-Handler. Cache im Prozess 10 Minuten je Titel und Art (höchstens 256
Einträge), `refresh` fragt neu; eine Quelle, die nicht antwortet, steht in `unavailable`.
`SetItemImage` nimmt nur einen Kandidaten des Titels (abgelaufener Cache wird neu gefüllt, sonst
422 `source_unknown`), schreibt `item_images` mit `chosen = 1`, sperrt die Art in `locked_fields`,
setzt die Herkunft und reiht `image.fetch` mit der manuellen Priorität ein. `ClearItemImage` löscht
Wahl und Sperre und reiht einen manuellen Match ein (Papier 14). `ItemImages` liefert
`AdminItem.images` (Quelle, Sprache, Vorschau, gewählt, gesperrt; «Sperre ohne Wahl»).
`LocalImagePreview` liefert nur Dateien, die der Scanner für den Titel gelistet hat, über die
Wurzel (Regel 4), höchstens 20 MB. Neue Abfrage `SetItemLockedFields`. Vertrag 1.6.0 (additiv) um
vier Operationen mit `x-visibility: admin`: `listItemImageCandidates`,
`getItemImageCandidatePreview` (`http.Handler`, Regel 6, vom strict server ausgenommen),
`setItemImage`, `clearItemImage`; Schemata `ImageCandidate`, `ImageCandidateList`,
`ItemImageChoice`, `AdminItemImage`, `AdminItem.images`. Oberfläche: Abschnitt «Bilder» in der
Titelpflege (`ImagesPanel.tsx`: je Art Vorschau, Quelle, Sprache, «von Hand gewählt», «Sperre ohne
Wahl», «Andere wählen», «Sperre aufheben») mit dem Dialog «<Art> wählen» (Raster
`ImageCandidateGrid` aus `web/src/ui`, 40 Kacheln, «weitere zeigen», «Neu abfragen», Vorschauen ohne
Referrer). Texte de/en, Feldfehler `source_unknown`.

**Tests** (neu): `TestImageChoiceSurvivesApply` (Kandidaten in Kettenreihenfolge, Cache und
`refresh`, SVG-Vorschau als PNG, 422 `source_unknown` und `kind`, Wahl mit Sperre und Job p7,
`ItemImages`, **DoD: ein `apply` mit Texten und Bildern behält das gewählte, gesperrte Poster**,
Sperre aufheben gibt die automatische Wahl zurück), `TestLocalImagePreviewServesOnlyListedFiles`
(gelistete Datei ja; ungelistete, `..`, falsche Version, Anbieterpfad: 404); Vertragstest um die
vier Operationen; drei Komponententests in `AdminItemPage.test.tsx`; e2e-Fall in
`admin-item.spec.ts` (Regel 8: Raster mit Pfeiltasten, Enter wählt, Escape schliesst, Fokus zurück
auf «Andere wählen», keine Fokusfalle); Story `UI/ImageCandidateGrid`.

Storybook-Baselines: Docker ist auf dem Entwicklungsrechner jetzt verfügbar, `storybook:test` lief im
CI-Lauf. Neu bzw. geändert und von Hand geprüft: `ui-titleorlogo--{text,logo,broken-logo}`,
`ui-imagecandidategrid--{posters,landscape,empty}`, `ui-herocarousel--{poster-only,active-poster}`
(die Folie «Glut» trägt seit Schritt 4 ein Logo). Die Logo-Fixture der Stories war zuerst ein in
breiter Box zentriertes SVG und stand dadurch eingerückt; sie ist jetzt eng beschnitten wie ein
echtes Logo. Damit sind auch die Baselines der Stories aus Schritt 4 nachgeholt.

DoD-Belege:

```
$ git add -A && task ci                    (Zweig a161762 + Schritt 5, alles gestaged)
server lint:   0 issues.   itemsaccess, go vet, testgroups --check grün; gen:check grün
server test:   ok  … 33 Pakete, kein FAIL
web lint grün; web test:  Test Files  86 passed (86)   Tests  691 passed (691)
storybook:test: 192 passed, 8 failed (2 geänderte HeroCarousel-Bilder, 6 neue Stories ohne Baseline)
$ task web:storybook:update -- -g "herocarousel--poster-only|herocarousel--active-poster|imagecandidategrid|titleorlogo"
                                           8 passed (Bilder geprüft, Logo-Fixture gestrafft, erneut geschrieben)
$ task storybook:test                      200 passed (15.2s)
$ go test ./internal/metadata/ -run 'TestImageChoiceSurvivesApply|TestLocalImagePreviewServesOnlyListedFiles' -count=1   ok
$ go test ./internal/api/ -run 'TestContract' -count=1                                                                 ok
$ task build && task e2e -- e2e/admin-item.spec.ts --repeat-each=2        28 passed (chromium, firefox)
```

**DoD «ein Titel mit gewähltem und gesperrtem Poster überlebt einen `apply`»:**
`TestImageChoiceSurvivesApply` wählt das englische Poster (`setItemImage`), startet `applyMetadata`
mit `texts` und `images`, lässt den Elternlauf, den Match und den Fill laufen und findet danach
weiter `item_images` = `/en-poster.jpg`, `chosen = 1`, `poster` in `locked_fields`; erst
`clearItemImage` gibt die automatische Wahl (deutsches Poster) zurück.

### Schritt 6 — Fanart.tv (31. Oktober 2026)

**O-4 geprüft** (Papier 14): gegen den offiziellen Client `@fanart-tv/api` 1.2.0 (README in der
npm-Registry; `api.fanart.tv` steht hinter einer Browser-Prüfung). API `webservice.fanart.tv/v3.2`
mit Breite und Höhe je Bild; Projektschlüssel `api_key` und persönlicher Schlüssel `client_key` als
Query oder Header; kein Kontingent, 429 mit `Retry-After` bei Übernutzung; neue Bilder nach 7
(Projekt) bzw. 2 Tagen (persönlich); Vorschau über `/preview/` statt `/fanart/`.

**Was gebaut ist.** `metadata/fanart.go`: Anbieter mit eigenem Begrenzer (2 Anfragen/s), Schlüssel
**als Header** `api-key`/`client-key` (nie in der Adresse), Filme über die TMDB-, Serien über die
TVDB-Id; Felder `hdmovielogo`/`movielogo`, `movieposter`, `moviebackground`/`movie4kbackground`
bzw. `hdtvlogo`/`clearlogo`, `tvposter`, `showbackground`/`show4kbackground`; Sprache `00` = ohne
Text, `likes` als Bewertung; nur Adressen des Asset-Hosts werden Kandidat. Gefragt wird nur mit
Schalter (`metadata.fanart_enabled`) **und** Projektschlüssel; ein Fehler von Fanart.tv lässt Match
und Fill weiterlaufen (Warnung im Log). Eingebunden im Match (Erkenner, `TitleRecord.Fanart`), im
Fill (letzter Datensatz) und in der Bildwahl je Titel (letzte Quelle, CDN-Vorschau); die Wahl in
sechs Stufen nimmt Fanart.tv wie geplant zuletzt. Schlüssel: `AdminSettingsKeys.fanart` und
`fanartClient` (Vertrag 1.6.0, additiv), `fanart_api_key`/`fanart_client_key` verschlüsselt in
`settings`, `SLOUCHCAT_FANART_API_KEY`/`SLOUCHCAT_FANART_CLIENT_KEY` (Umgebung gewinnt, beide aus
derselben Quelle), zwei Felder auf «Integrationen», `deploy/.env.example`, `compose.yaml`,
`deploy/README.md`. Redaction: `clientkey` in den Attributnamen, `client_key` in der Query-Regel.

**Tests** (neu): `TestFanartNeverAskedWithoutKeyOrSwitch` (Schalter ohne Schlüssel, Schlüssel ohne
Schalter: Match, Fill und Bildwahl fragen nie), `TestFanartLogoInCache` (DoD: TMDB ohne Logo, das
deutsche Fanart-Logo gewinnt, `image.fetch` legt `logo/w400|w800` als PNG und WebP ab; das deutsche
TMDB-Poster schlägt das textlose Fanart-Poster; Schlüssel nur als Header, nie in Adresse oder Log;
Kandidat mit `/preview/`; fremder Host nie Kandidat); `TestIsSensitiveKey` um die Fanart-Namen;
Integrationen-Komponententests mit den zwei Schlüsseln.

Nachzug aus Schritt 4, den erst der volle e2e-Lauf zeigte: `admin-home.spec.ts` zählt 18 Karten (die
Karte «Metadaten» kam dazu), `admin-libraries.spec.ts` gibt dem Tab bis zum Namensfeld 80 statt 40
Schritte (jede Videobibliothek trägt jetzt den Link «Metadaten der Bibliothek», Firefox hält dort).

DoD-Belege:

```
$ git add -A && task ci                    (Zweig 49c74a4 + Schritt 6, alles gestaged)
server lint:   0 issues.   itemsaccess, go vet, testgroups --check grün; gen:check grün
server test:   ok  … 33 Pakete, kein FAIL
web lint:      prettier --check: web/src/features/admin/IntegrationsPage.tsx (Zeilenumbruch der KEYS-Liste;
               korrigiert)
$ pnpm -w run lint                         web lint: Done; All matched files use Prettier code style!
$ cd web && pnpm exec vitest run           Test Files  86 passed (86)   Tests  691 passed (691)
$ task storybook:test                      200 passed (15.1s)
$ task build && task ci:e2e                489 passed, 5 failed (26.2m): admin-home Tab-Fall (chromium, firefox:
               17 statt 18 Karten), admin-libraries zwei Fälle (firefox: Tab-Budget 40), admin-streams 5-s-Fall (firefox)
$ task e2e -- e2e/admin-home.spec.ts e2e/admin-libraries.spec.ts e2e/admin-streams.spec.ts
                                           29 passed, 1 failed: [firefox] admin-streams.spec.ts:39 «its player stops within 5 s»
$ go test ./internal/metadata/ -run 'TestFanart' -count=1                  ok
$ go test ./internal/logging/ -count=1                                    ok
```

Der eine rote Fall ist das bekannte 5-Sekunden-Zeitbudget von `admin-streams.spec.ts` in Firefox (Schritt 1,
Konzepte 08 und 09); er berührt keine Zeile dieses Konzepts.

### Schritt 7 — Abnahme, Messung, Doku (31. Oktober 2026)

**CT 190** läuft seit 16:01 Uhr auf `ghcr.io/slouchcat/slouchcat:metadaten-19251ac` (`v1.5.0-14-g19251ac`,
Kopf nach Schritt 6), gebaut auf dem CT aus `git archive 19251ac` (Skripte im ignorierten Ordner `.omc/ct190/`,
Umstellung über das Terminal-Panel), Sicherung `.env.bak-vor-metadaten`, davor `1.5.0`. Die Migrationen
00028 bis 00030 liefen in 0.9 / 0.7 / 0.2 s nach der Datenbanksicherung `pre-migration-v1.5.0-14-g19251ac-…`;
Start 3.9 s, gesund nach 7 s. Messwerte und Startzustand in `docs/perf/metadaten.md` Abschnitt 3
(2'093 Filme, 2 Serien; 4'480 Bildquellen aus den TMDB-Spalten; 208 Bilddateien in 91 Titelordnern
kurz nach dem Start; Cache 9.7 GB).

**`apply` auf CT 190** (Marco startete ihn um 16:25 Uhr, `docs/perf/metadaten.md` 3.1): 2'095 Titel,
Zuordnungen in 16.3 min, letztes Bild nach 20.1 min, 0 fehlgeschlagene Jobs; Cache 9.7 → 12 GB; 2'046
Titel haben jetzt ein Logo, 1'337 ein deutsches Poster (vorher 60). Live belegt: TMDB liefert zu jedem
der 23 gewählten SVG-Logos die PNG-Fassung (O-3). Offen bleibt nur der Live-Beleg der Sprachcodes ohne
Region (Schritt 1): Marcos Liste hat keine dritte Sprache.

**Nachtrag Bildschärfe im Spotlight** (Marcos Rückmeldung nach dem `apply`, 31. Oktober 2026): das
Spotlight lud höchstens `w1280`, obwohl `w1920` im Cache lag; auf CT 190 sind 1'107 Hintergrund-Originale
3840 px breit oder mehr, weitere 156 mindestens 2560 px. Jetzt bieten Spotlight, Detailseite, Vorab-Bild
und `Link`-Header `w1920` und die neue Stufe `w3840` an (nur aus Originalen ab 2560 px, nie hochskaliert,
mit echter Breite im `srcset`; `ItemImages.backdropUhd`, `getItemImage` `w3840`). Bestehende Titel
bekommen sie mit dem nächsten Scan. Auf CT 190 (`metadaten-e86f520`, 17:10 Uhr) hatten nach dem Startscan 956 Titel `w3840`,
nach rund 6 min, Cache 12 → 14 GB (`docs/perf/metadaten.md` 3.2). Tests: `TestSizesFor`, `TestBackdropUhdFromWideOriginal` (4000 px →
3840, 2600 px bleibt, 2000 px ohne; der Scan-Check reiht fehlende Titel ein), `TestHeroPrepaint`,
`TestStartPagePreloadHeaders`, ein Fall in `HomePage.test.tsx`; Papier 14 «Nachtrag».

```
$ git add -A && task ci                    (Zweig a5abade + Nachtrag)
server: 0 issues, gen:check grün, 33 Pakete ok; web lint grün; web test: 86 Dateien, 692 Tests passed
storybook:test: 200 passed (16.7s); task ci:e2e: 493 passed, 1 failed (25.5m):
               [firefox] admin-streams.spec.ts:39 «its player stops within 5 s» (bekannt, ohne Bezug)
```

**Versionsstellen** auf 1.6.0: `deploy/.env.example`, `deploy/compose.yaml`, `deploy/README.md`,
`docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`. Den Tag `v1.6.0` setzt Marco nach dem Merge
(HANDOVER, Beschriftung = Release-Notes).

**Release-Notes-Entwurf 1.6.0** (C-5, nicht brechend; erste Zeile nach 5.4):

- Metadaten-Einstellungen: Sprachen als Liste, Bildsprache und Logo, lokale Bilddateien,
  Freigabeland, Bildwahl je Titel; Vertrag 1.6.0 fügt hinzu, nichts entfällt.
- Verwaltung, neue Seite «Metadaten» (Bibliothek → Metadaten): die Sprachen, in denen die
  Metadatenquellen Titel, Beschreibungen und Genres liefern, als geordnete Liste (Deutsch Pflicht,
  jede weitere Sprache wird für jeden Titel geholt; die Reihenfolge ist der Rückfall); je Bildart
  (Poster, Hintergrund, Logo) eine Sprachkette mit «Ohne Text»; Schalter für Logo, Staffelposter
  und Folgenbild; Mindestbreiten und «Nach Bewertung wählen»; TMDB-Erwachseneninhalte bei der
  Suche; lokale Bilddateien als Quelle; Fanart.tv; das Land, dessen Altersfreigabe zuerst gelesen
  wird (DE, AT, CH, GB, US). Jede Videobibliothek kann bei Sprachen und Bildern abweichen. Neue
  Einstellungen gelten für neue Titel sofort; «Jetzt anwenden» bringt bestehende Titel nach (Lauf
  unter Hintergrundjobs).
- Profile: «Sprache der Beschreibungen», sobald der Server eine dritte Sprache führt; die
  Oberfläche bleibt Deutsch oder Englisch.
- Bilder: Spotlight und Detailseite laden Hintergründe bis 1920 px und, wo das Original mindestens
  2560 px breit ist, in einer neuen Stufe bis 3840 px für 4K- und Retina-Bildschirme (nie hochskaliert);
  Logos (PNG und WebP, nie JPEG) auf der Detailseite und im Spotlight statt des Text-Titels; Bilddateien neben dem Film (`poster.jpg`/`folder.jpg`, `fanart.jpg`/`backdrop.jpg`,
  `logo.png`/`clearlogo.png`, `seasonNN-poster.jpg`, `<Datei>-thumb.jpg`) gewinnen, wenn sie breit
  genug sind; Fanart.tv als letzte Quelle mit Schlüssel unter «Integrationen» oder
  `SLOUCHCAT_FANART_API_KEY`/`SLOUCHCAT_FANART_CLIENT_KEY`.
- Titelpflege: Abschnitt «Bilder» mit den Kandidaten aller Quellen je Art; eine eigene Wahl sperrt
  die Art und überlebt «Anwenden» und «Aktualisieren», «Sperre aufheben» gibt die automatische Wahl
  zurück.
- Altersfreigabe: eine gespeicherte Stufe wird durch einen Länderwechsel nie gesenkt; ein neuer
  Titel bekommt die Stufe des zuerst gelesenen Landes.
- Vertrag 1.6.0, additiv: `getMetadataSettings`, `updateMetadataSettings`, `getLibraryMetadata`,
  `updateLibraryMetadata`, `applyMetadata`, `listItemImageCandidates`,
  `getItemImageCandidatePreview`, `setItemImage`, `clearItemImage` (alle `admin`);
  `Profile.metadataLanguage`, `SessionInfo.availableMetadataLanguages`, `ImageKind` `logo`,
  `ItemImages.logo`, `ItemImages.backdropUhd` mit der Grösse `w3840` in `getItemImage`, `AdminItem.images`,
  `AdminSettingsKeys.fanart`/`fanartClient`.
- Datenbank: Migrationen 00028 (Rang der Textsprachen, Metadatensprache des Profils), 00029
  (`item_images`, `item_local_images`, Bildart `logo`, Format `png`), 00030
  (`library_metadata`); vor dem Lauf sichert der Server die Datenbank wie immer.
- Hinweis: Bestehende Titel behalten ihre Bilder und Texte, bis «Jetzt anwenden» läuft. Der erste
  Scan nach dem Update liest die Bilddateien neben den Medien (nur den Kopf jeder Datei).

## Konzept 09 abgeschlossen, Release 1.5.0, CT 190 (31. Oktober 2026)

**Veröffentlicht am 31. Oktober 2026:** PR #55 (`claude/vigorous-raman-651439`, 11 Commits bis
`19ee05c`) von Marco gemergt (Merge `9e1e44a`), Tag `v1.5.0` auf `9e1e44a` (Beschriftung =
Release-Notes, nachzulesen mit `git cat-file tag v1.5.0`), Release-Lauf 41184322886 grün: Image
`ghcr.io/slouchcat/slouchcat:1.5.0` (amd64, arm64, anonym ziehbar), `manifest.json` mit
`version 1.5.0` und `FFMPEG-NOTICE.md` am Release. Vertrag 1.5.0 ist damit getaggt (Regel
Vertrag = Tag).

**CT 190** lief zuerst, von 09:18 bis 09:27, als Zwischenschritt auf dem selbst auf dem CT gebauten
Image `hintergrundjobs-19ee05c` (`v1.4.0-31-g19ee05c`, der Zweigstand vor dem Merge). Dabei lief
Migration 00027 in 2.1 s; Sicherung vorher als
`pre-migration-v1.4.0-31-g19ee05c-20261031T081751.721Z.db`; `job_schedules` hat sechs Zeilen.
Bauweg auf dem CT (ohne lokales Docker) in HANDOVER 5. Dafür musste auf dem CT
`docker-buildx-plugin` nachinstalliert werden: der alte Builder kennt `$BUILDPLATFORM` nicht und
brach mit `failed to parse platform : ""` ab.

Seit 09:27 läuft CT 190 auf dem Release-Image `ghcr.io/slouchcat/slouchcat:1.5.0`
(`docker compose pull && docker compose up -d --wait`; Sicherung der Einstellungen
`/opt/slouchcat/deploy/.env.bak-vor-1.5.0`; `healthz` meldet `version 1.5.0`). Keine Migration
mehr, 00027 lief schon mit dem Zwischenstand. Beim Start reihte der Scheduler Scan, Sweep und
Podcast-Abruf sofort und die Update-Prüfung nach 60 s ein: das ist die Startregel (`StartDelay`),
kein Befund.

**Noch offen** (Einzelheiten im Abschnitt «Schritt 5» unten): Abnahme am Handy, ein Tag
Beobachtung der Zeitpläne auf CT 190, Messung `listJobTypes` auf CT 190 gegen die Latte 50 ms p95.

## Hintergrundjobs (Konzept 09): Schritt 5, Teil der Session (30. Oktober 2026)

Gleicher Zweig und PR (`claude/vigorous-raman-651439`, PR #55). Konzept Abschnitt 10, Schritt 5:
Messung, Release-Notes, Versionsstellen; Befunde in **13.8**. Image auf CT 190, Merge und Tag sind
erledigt (siehe oben); Abnahme am Handy, Beobachtung und Messung auf CT 190 stehen aus.

**Messung** `docs/perf/hintergrundjobs.md` (Entwicklungsrechner, eigener Server auf 8099, NAS per
NFS, schwere Folgejobs pausiert; `docs/perf/seed-jobs.py` für 50'000 erledigte Läufe):

```
$ bash docs/perf/measure-home.sh /api/v1/admin/job-types     (55'423 Zeilen, vor dem ersten prune)
/api/v1/admin/job-types: n=60 p50=14.9 ms p95=23.6 ms max=33.8 ms not-200=0
$ POST /admin/job-types/jobs.prune/run                        32'760 Zeilen in 525 ms, 22'663 bleiben
$ bash docs/perf/measure-home.sh /api/v1/admin/job-types     (nach dem prune)
/api/v1/admin/job-types: n=60 p50=9.0 ms p95=23.6 ms max=26.2 ms not-200=0
```

Latte < 50 ms p95 gilt für CT 190; mit Faktor 2 bis 3 (M3 1c) sind dort 47 bis 71 ms zu
erwarten, Messung auf CT 190 offen. NAS-Scan «Filme» (1'791 Dateien): 186 s; Schätzung
«gemessen» 2'036 s bei wirklich 34 s, Median nach sechs leeren Rescans 0 s. **Vier Befunde mit
Hebeln** (B-1 Reporter lässt Meldungen bis zum Heartbeat liegen, B-2 Hash-Phase ohne Fortschritt,
B-3 Rate ab `started_at` statt ab der ersten Meldung, B-4 leere Rescans zählen für den Median) in
`docs/perf/hintergrundjobs.md` 2. **Auf Marcos Wort am 31. Oktober behoben** (`jobs/progress.go`
Timer und Fortschrittsreihe, `Registry.ProgressSince`, `admin/jobtypes.go` Rate ab der Reihe mit
`measuredMinSeries` 10 s, `library/scanner.go` Fortschritt in `planRoot` und `Skip` bei leerem
Schreibplan; Testhilfe `jobs.Recorder`). Nachmessung in 2a: Scan «KinderSerien» (3'749 Folgen,
126 s) mit Schätzfehler unter Faktor 2 statt 60; leere Rescans `skipped`.

```
$ go test ./internal/jobs ./internal/admin ./internal/library     ok (TestProgressTimerAndSeries,
                                                                  TestJobRunMeasuredEstimate, TestScanReportsHashPhaseAndSkipsUnchangedRescan)
$ task ci:server                                                  grün (lint, gen-check, go test -race, 31.10. 00:30)
```

**Versionsstellen auf 1.5.0:** `deploy/.env.example`, `deploy/compose.yaml`, `deploy/README.md`,
`docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`. Tag `v1.5.0` setzt Marco auf den Merge-Commit
(HANDOVER, Beschriftung = Release-Notes).

**Release-Notes-Entwurf 1.5.0** (C-5, nicht brechend; erste Zeile nach 5.4):

- Neue Verwaltungsseite Hintergrundjobs; Vertrag 1.5.0 fügt `/admin/job-types` hinzu, nichts
  entfällt.
- Verwaltung: Seite «Hintergrundjobs» (System → Hintergrundjobs, Kachel auf dem Dashboard) mit
  den Zahlen Laufend, Wartend, Geplant, Pausiert, Fehlgeschlagen, den Arbeitern und dem Stau je
  Pool, «Läuft gerade» mit Fortschritt und Restzeit («noch ca.», «wird berechnet», «länger als
  üblich») und einer Karte je Jobart: Jetzt ausführen, Pausieren und Fortsetzen (hält laufende
  Läufe sofort an, überlebt einen Neustart), Wartende abbrechen, Alle Läufe abbrechen, Zeitplan
  (Takt 5 Minuten bis 7 Tage, Zeitfenster in der Serverzeit), Läufe (Verlauf nach Zustand, Erneut,
  Abbrechen).
- Zeitpläne: Bibliotheksscan, Aufräumen fehlender Titel, Podcast-Abruf (`podcast.poll`),
  Spotlight-Liste (`trakt.refresh`), Update-Prüfung (`update.check`) und Verlauf aufräumen
  (`jobs.prune`) laufen über einen Scheduler statt über fünf eigene Schleifen; Takt und Fenster
  sind in der Verwaltung einstellbar. Der Spotlight-Takt (ganze Stunden, 6 bis 48) und der
  Schalter der Update-Prüfung bleiben mit ihren Einstellungen verbunden; die Update-Prüfung läuft
  60 Sekunden nach jedem Start.
- Verlauf: erledigte und abgebrochene Läufe älter als 30 Tage werden täglich gelöscht, die
  neuesten 50 je Jobart bleiben; fehlgeschlagene Läufe bleiben, bis die Verwaltung sie erneut
  startet oder abbricht.
- Vertrag 1.5.0, additiv: `listJobTypes`, `runJobType`, `pauseJobType`, `resumeJobType`,
  `updateJobSchedule`, `cancelJobRuns`, `listJobRuns` (alle `admin`), `JobCounts.paused`,
  Schemata `JobRun`, `JobProgress`, `JobWindow`, `JobSchedule`, `JobScheduleUpdate` (mit
  `clearWindow`), `JobTypeInfo`, `JobPool`, `JobOverview`, `JobRunPage`, `JobCancelResult`,
  `JobTypeName`; Fehlercodes `job_type_not_runnable`, `job_type_paused`,
  `job_type_not_scheduled`; `DownloadState.error` kennt `cancelled_by_admin`.
- Migration 00027: `jobs` mit Start, Ende, Fortschritt und `skipped`, Index `jobs_history`,
  Tabelle `job_types`, `job_schedules` neu gebaut und aus den bisherigen Einstellungen gefüllt;
  der Server sichert die Datenbank vorher nach `/config/backups/`.

**Erledigt:** Image des Zweigs auf CT 190 gebaut und umgestellt (`hintergrundjobs-19ee05c`,
31. Oktober 09:18, ab 09:27 das Release-Image 1.5.0), Merge von PR #55 (`9e1e44a`), Tag `v1.5.0`, Release-Lauf grün.

**Offen (Marco):**

- Abnahme am Handy nach Schritt 5.
- Ein Tag Beobachtung der sechs Zeitpläne auf CT 190 (laufen sie im Takt und im Fenster, bleibt
  der Verlauf nach `jobs.prune` bei 30 Tagen und 50 je Jobart).
- Messung `listJobTypes` (`/api/v1/admin/job-types`) auf CT 190 gegen die Latte 50 ms p95
  (Abschnitt 3 der Messung, `docs/perf/hintergrundjobs.md`).

## Hintergrundjobs (Konzept 09): Schritt 4 (30. Oktober 2026)

Gleicher Zweig und PR (`claude/vigorous-raman-651439`, PR #55). Konzept Abschnitt 10, Schritt 4:
die Seite. Kein Vertragsbruch (nur die Beschreibung von `DownloadState.error` nennt
`cancelled_by_admin`).

**Gebaut.** `web/src/features/admin/JobsPage.tsx`: Kopf mit Titel und fünf Zahlen (Laufend,
Wartend, Geplant, Pausiert, Fehlgeschlagen), die Pools mit Arbeitern und Stau, ein Hinweis in
Warnfarbe mit «Fortsetzen» je pausierter Jobart, «Läuft gerade» (Name, Gegenstand, Notiz, Dauer
sekündlich hochgezählt, Balken bei bekanntem Umfang, Restzeit «noch ca.» / «wird berechnet» /
«länger als üblich», Knöpfe «Lauf pausieren» und «Abbrechen» mit Bestätigung), je Jobart eine Karte
in vier Gruppen (Name und Code, Zustandschip, Wartend mit Schätzung und «üblich», Letzter Lauf,
Nächster Lauf mit Takt und Fenster, Ausgelöst durch; Knöpfe Jetzt ausführen, Pausieren/Fortsetzen,
Wartende abbrechen (n), Alle Läufe abbrechen, Zeitplan …, Läufe …), Dialog «Läufe von X» mit sechs
Reitern und Tabelle (Erneut, Abbrechen), `JobScheduleDialog.tsx` (Ein/Aus, Takt aus den Grenzen
des Servers, Zeitfenster mit zwei Zeitfeldern, Hinweise für Spotlight und Update, 422 am Feld).
Abfrage alle 5 s, pausiert bei offenem Dialog. Route `/admin/jobs`, Navigation in «System» vor
«Integrationen» (Icon `refresh`), Dashboard-Kachel mit «Alle Jobs» und «Pausiert». Texte in
`de.json`/`en.json` (`admin.jobs.*`, achtzehn Jobarten mit Name und Auslöser, drei Problemcodes).
Abweichungen in **13.7**.

**Tests.** `JobsPage.test.tsx` (Zahlen, Pools, laufende Läufe mit Balken und Restzeit, Karten;
Pausieren in der Zeile ruft die Mutation einmal mit der Jobart und beide Zeilen zeigen «wird
angehalten»; Jetzt ausführen mit Meldung und 409; Wartende abbrechen mit Bestätigung, Escape und
Fokus zurück; Nur-Administratoren-Hinweis), `AdminHomePage.test.tsx` (Karte und Navigation),
`locales.test.ts`; `web/e2e/admin-jobs.spec.ts` (Navigation, Jetzt ausführen bis «Erledigt»,
Pausieren/Fortsetzen mit 409, Dialog Läufe mit Pfeiltasten und Escape, Zeitplan auf 2 h, Zurück,
sichtbarer Fokus, keine Fokusfalle; `afterEach` setzt pausierte Jobarten fort und stellt den
Sweep-Takt wieder her), `NAV_PAGES` mit «Hintergrundjobs».

**DoD-Belege.**

```
$ pnpm exec vitest run src/features/admin/JobsPage.test.tsx src/features/admin/AdminHomePage.test.tsx src/locales
Tests  22 passed (22)
$ task web:lint                                 grün (eslint, stylelint, tsc, prettier)
$ task ci:server · web:gen:check · web:test · build   grün für ced5636
$ task ci:e2e (ced5636)                        477 passed, 3 failed: Kartenzahl 16 → 17 in admin-home.spec.ts
                                                und zwei Firefox-Fallen in admin-jobs.spec.ts (Fokus-Startpunkt
                                                nach ersetztem Knopf, Fokusrückgabe nach dem Speichern); behoben
                                                im Folge-Commit, beide Specs danach in Chromium und Firefox grün
$ task ci:e2e (nach dem Fix, 9ee3b72)           480 passed (25.2m); GitHub-CI für 9ee3b72 grün (Lauf 41088318618,
                                                inkl. Job «web» mit storybook:test, der hier nicht laufen kann)
```

Dev-Stack: `localhost:8096` läuft seit 16:58 auf dem Image `hintergrundjobs` mit dieser Seite
(`v1.4.0-26-gced5636`), sechs Zeitpläne (`jobs.prune` übersprungen beim ersten Lauf), 18 Jobarten.

```
```

**Offen (Schritt 5):** Image bauen, CT 190 umstellen, Abnahme am Handy, Messung
`docs/perf/hintergrundjobs.md` (`listJobTypes` vor und nach dem ersten `prune`, Schätzung gegen
Wirklichkeit), ein Tag Beobachtung der Zeitpläne, Release-Notes-Zeile (5.4), Storybook unverändert.

## Hintergrundjobs (Konzept 09): Schritt 3 (30. Oktober 2026)

Gleicher Zweig und PR (`claude/vigorous-raman-651439`, PR #55). Konzept Abschnitt 10, Schritt 3:
**Vertrag 1.5.0, additiv**, Server-Handler, keine Seite (Schritt 4).

**Vertrag.** `api/openapi.yaml` 1.5.0: `listJobTypes` (`GET /admin/job-types`), `listJobRuns`
(`GET /admin/jobs?type=&state=&limit=`, `state` mit `planned`), `runJobType`, `pauseJobType`,
`resumeJobType`, `updateJobSchedule` (PATCH, `JobScheduleUpdate` mit `everySeconds`, `window`,
`clearWindow`, `enabled`), `cancelJobRuns` (`?scope=queued|all`), alle `x-visibility: admin`, kein
`ItemRef`; Schemata `JobRun`, `JobProgress`, `JobWindow`, `JobSchedule`, `JobTypeInfo`, `JobPool`,
`JobOverview`, `JobRunPage`, `JobCancelResult`, `JobTypeName`; `JobCounts.paused` optional;
Problemcodes `job_type_not_runnable`, `job_type_paused`, `job_type_not_scheduled`. `task gen` für
Go (`internal/api/gen`) und TypeScript (`packages/api-client`); die Web-App kompiliert unverändert
(`tsc --noEmit`).

**Server.** `admin/jobtypes.go`: `WithJobControl(registry, scheduler, Arbeiter je Pool)`;
`ListJobTypes` baut je Jobart Gruppe, Auslöser, Pool, Pause (`stopping` = pausiert und noch
laufend), `runnable`, Zeitplan mit `limits` und `timeZone`, Zähler (wartend, geplant, laufend,
fehlgeschlagen), bis zehn laufende Läufe mit Fortschritt und Restzeit (gemessen ab 5 % und 30 s,
sonst Median, «länger als üblich» ab 2 × Median), letzter Lauf, Median (ab 5 Läufen), Wartezeit
kumuliert nach Priorität über den Pool, Stau je Pool, unbekannte `job_types`-Zeilen mit
`known = false`; Gegenstand je Lauf aus dem Payload (Bibliothek, Titel mit Staffel-/Folgenmarke,
Kanal, Download), redigiert. `RunJobType` mit Priorität 7 (Trakt über `RefreshSpotlight`, Update
mit Schalterprüfung, `podcast.poll` über neues `metadata.QueueAllPodcasts` = alle Abos), der
Zeitplan rückt nicht (F-5). `UpdateJobSchedule` prüft Grenzen (Trakt 6 bis 48 ganze Stunden) und
schreibt «ein Wert, zwei Türen»: `spotlight_config.interval_hours` (`SetSpotlightInterval`) und
`update_check_enabled`. `CancelJob` des Dashboards läuft neu über den Abbruch-Haken.
Abweichungen in **13.6** (u. a. `clearWindow` statt `null`, `JobRun` als eigenes Schema).

**Tests.** `admin/jobtypes_test.go` (Übersicht mit Median, Wartezeit, Stau, laufender Lauf,
unbekannte Zeile, Dashboard `paused`; gemessene Schätzung; `runJobType` mit 409/404/422 und
unverändertem `next_run_at`; Pause/Fortsetzen; Zeitplan mit Grenzen, Fenster, Trakt- und
Update-Durchschreiben; Läufe listen und abbrechen). Vertragstest mit Registry und Scheduler in der
Fixture (`TestContractCoversEveryOperation`, Schema-Test `x-visibility`).

**DoD-Belege.**

```
$ go test ./internal/admin/ ./internal/api/ ./internal/metadata/ ./internal/jobs/
ok  …/admin 0.852s (Jobtypen) · ok …/api 17.933s (Vertrag, Schema) · ok …/metadata 26.090s · ok …/jobs 5.690s
$ go tool golangci-lint run ./...               0 issues.
$ pnpm -C web exec tsc --noEmit                 keine Fehler (Web-App kompiliert gegen den Client 1.5.0)
$ task ci:server                               grün (gen, gen:check, lint, test)
$ task web:gen web:gen:check web:lint web:test   grün (api-client 1.5.0 ohne Drift)
$ task build && task ci:e2e                     478 passed (24.8m)
```

**Offen (Schritt 4 und 5):** Seite `/admin/jobs` mit Dialogen, Navigation, Dashboard-Link und
-Kachel, Sprachdateien, `admin-jobs.spec.ts`, Fixtures; `DownloadState.error` um
`cancelled_by_admin` in der Beschreibung ergänzen; CT 190, Messung, Release-Notes.

## Hintergrundjobs (Konzept 09): Schritt 2 (30. Oktober 2026)

Gleicher Zweig und PR wie Schritt 0 und 1 (`claude/vigorous-raman-651439`, PR #55), auf Marcos
Wort («mach weiter») nach dem Umstellen von `localhost:8096` auf das Branch-Image. Konzept
Abschnitt 10, Schritt 2; Vertrag bleibt **1.4.0**, keine Oberfläche.

**Gebaut.** `jobs.Progress(ctx, done, total, note)` und `jobs.Skip(ctx)` über einen Reporter, den
der Arbeiter in den Kontext legt (schreibt höchstens alle 2 s, mit jedem Heartbeat und einmal nach
der Rückkehr des Handlers, unter eigener Lease, Notiz durch `logging.RedactString` und auf
200 Zeichen gekürzt). Vier Handler melden: `scan.library` in der Durchsuchphase (gezählte Ordner,
`total = 0`, Notiz «Bibliothek · Wurzel/Ordner») und in der Schreibphase (`seen`/`total`),
`segments.detect` je Folge, `download.prepare` je Segment, `probe.keyframes` die Datei relativ zur
Wurzel; `Skip` rufen `download.prepare` beim Warten, `scan.library` ohne Ordner, `update.check` bei
ausgeschalteter Prüfung und `jobs.prune` ohne Arbeit. `ClaimJob` setzt `started_at` und löscht den
Fortschritt des vorigen Versuchs; `CompleteJob`, `FailJob` (nur im Endzustand), `CancelJob` und
`FailExhaustedLeases` setzen `finished_at`; `RetryFailedJob` setzt alles zurück. **Pause** (F-1,
F-6): `ClaimJob` nimmt keine Läufe pausierter Jobarten; `Registry.Pause` setzt `job_types.paused_at`
und bricht laufende Läufe über die Laufliste mit `jobs.ErrPaused` ab, der Arbeiter prüft die Ursache
**vor** dem `switch` und gibt den Lauf ohne verbrauchten Versuch zurück, auch wenn der Handler `nil`
zurückgibt; der Heartbeat ist der Rückfall. **Abbruch** mit Haken: `Registry.CancelJob` und
`CancelRuns(type, running)` rufen `Type.Cancel` (`library.cancelScanJob` → `CancelScan`, Bibliothek
endet `aborted/cancelled`; `offline.cancelPrepareJob` → `CancelPreparation`, Download `failed` mit
`cancelled_by_admin`, `Reconcile` lässt ihn liegen, der unterbrochene Handler setzt nur noch
`queued`, wenn der Download noch `preparing`/`paused` ist), dann `Cancel` in der Queue und sofortiger
Abbruch des Kontexts mit `jobs.ErrCancelled`. **Verlauf**: `jobs.prune` (von `NewRegistry` selbst
registriert, täglich, Priorität 2) löscht `done`/`cancelled` älter als 30 Tage, behält je Jobart die
neuesten 50, lässt `failed` liegen, Batches à 500; `Queue.MedianDuration` über die neuesten 50
nicht übersprungenen Läufe (bekannt ab 5). Eingefrorener Stand `testdata/db/hintergrundjobs.sql` mit
Test. Abweichungen vom Papier in **13.5** (u. a. Pause/Abbruch auf der Registry statt der Queue,
sofortiger Abbruch statt Heartbeat, Download-Handler schützt den Fachzustand des Hakens).

**Tests.** `control_test.go`: pausiert wird nicht beansprucht; laufender Lauf hält fünfmal an und ist
wieder `queued` mit `attempts = 0`, kein `JobFailed`, Handler gibt `nil` zurück; Pause per Heartbeat;
Abbruch durch Haken (wartend, laufend, Reihenfolge der Haken, fremde Jobart unberührt); Fortschritt
gedrosselt, redigiert, gekürzt, Flush am Ende, Retry setzt zurück; Median (Ausreisser, übersprungene);
Prune behält 50 und `failed`. `library/cancel_hook_test.go`: Bibliothek endet `aborted/cancelled`.
`offline/cancel_hook_test.go`: abgebrochener Download wird von `Reconcile` nicht wieder eingereiht,
fertiger bleibt unberührt. `store/frozen_hintergrundjobs_test.go`.

**DoD-Belege.**

```
$ go test ./internal/jobs/                      ok  4.950s
$ go test ./internal/library/ ./internal/offline/ ./internal/admin/ ./internal/metadata/ ./internal/api/ ./internal/notify/ ./cmd/...
ok  …/library 40.259s  ok  …/offline 15.522s  ok  …/admin 21.792s  ok  …/metadata 28.322s
ok  …/api 38.419s      ok  …/notify 7.351s    ok  …/cmd/slouchcat 4.071s
$ go test ./internal/store/ -run 'TestFrozenHintergrundjobs|TestFrozenStatesMigrate/hintergrundjobs'   ok  22.154s
$ go tool golangci-lint run ./...               0 issues.
$ task ci:server (nach dem Commit)           grün (gen, gen:check, lint, test)
$ task build && task ci:e2e                     478 passed (25.1m); A-9 log scan: 76016 lines, 17 secrets of the run, 0 findings
```

Eingefrorener Stand erzeugt aus `startseite.sql` plus 80 s Serverlauf (Port 18099, zwei Arbeiter):
18 `job_types`, 6 `job_schedules` (`update.check` aus), 196 erledigte Läufe mit Start und Ende,
`jobs.prune` als `skipped = 1`, Scan-Notizen «Filme · Movies/…», ein laufender `segments.detect`
mit `3/10` per SQL; dann `slouchcat dump` (`testdata/db/README.md`).

**Offen (Schritt 3 bis 5):** Vertrag 1.5.0 mit den sieben Operationen, `JobCounts.paused`,
`stopping`, die Schätzungen (Restzeit, Wartezeit, Stau), `admin.Schedules`-Rückschreiben,
`podcast.poll` von Hand = alle Abos; Seite (Schritt 4); CT 190 und Messung (Schritt 5).

## Hintergrundjobs (Konzept 09): Schritt 0 und 1 (30. Oktober 2026)

Auf dem Zweig `claude/vigorous-raman-651439` (Worktree, ab `main 05df9c5`, der Freigabe des Papiers).
Konzept `docs/planning/09-hintergrundjobs.md`, Abschnitt 10; Vertrag bleibt **1.4.0** (1.5.0 kommt
mit Schritt 3), keine Oberfläche.

**Schritt 0 (`065889e`, nur Doku).** Entscheid 15 (F-1 bis F-10, O-1, O-2) in
`04-entscheidungen.md` in beiden Tabellen; ADR 0017 «Scheduler, Fortschritt, Pause und
Abbruch-Haken in der Job-Queue» als Teilnachfolger von ADR 0002, dort der Hinweis; das Papier auf
`05df9c5` neu verankert (Kopf: Vertrag 1.4.0, Migration 00026, ADR 0016; alle Zeilenangaben
geprüft). Wo Papier und Code sich widersprachen, gilt der Code, festgehalten in **13.3**:
`update.check` läuft mit Priorität 8 (PR #52), `metadata.vote_backfill` ist eine Ereignis-Jobart
(08 stösst ihn nach jedem Scan und beim Start selbst an), der Trakt-Zeitplan hängt nicht an
`spotlight_config.enabled` (der Poller hat es nie gelesen; `enabled = 0` blendet nur das Band aus).
`git diff --stat docs/spec/` leer.

**Schritt 1 (`8e95e3e`).** Migration `00027_jobs_scheduler.sql` nach Vorlage: sechs Spalten und
Index `jobs_history` an `jobs`, Tabelle `job_types`, `job_schedules` mit Wächter wie 00023
gelöscht und neu gebaut (`every_ms` 5 min bis 7 d, `start_delay_ms`, `enabled`, `window_start`,
`window_end`, `next_run_at`, `last_fired_at`). Paket `jobs`: `Type` (Gruppe, Pool, Auslöser,
Priorität, Handler, `Run`, `Cancel`, `Every`, `StartDelay`, `Seed`), `Registry`
(`Register`, `Lookup`, `Types`, `Attach` je Pool), `Scheduler` (`Start` legt `job_types` und
fehlende Zeitplanzeilen an, aus Einstellung und Historie der alten Poller, und wendet die
Startregel an; `Run` tickt sofort und dann alle 60 s; pausierte Jobart feuert nicht, rückt aber
vor; unbekannte Zeile wird übersprungen; verpasste Läufe holen nicht nach; `UpdateSchedule`
richtet `next_run_at` an `last_fired_at + every`, Untergrenze jetzt, und am Fenster aus, über
Mitternacht erlaubt). `RegisterJobs(w)` → `RegisterTypes(r)` in `library`, `metadata`, `admin`,
`offline`, `notify`; Handler unverändert. Die fünf Ticker-Schleifen sind weg (`library.Start`
nur noch Wächter; `RunPodcasts`, `RunTrakt`, `RunUpdateCheck` gelöscht); neue Zeitplan-Jobart
`podcast.poll` mit `QueueDuePodcasts` als `Run`; der Trakt-Sonderfall «kein Client-Schlüssel
leert die Einträge» liegt im `Run`. `enqueueScan`, `enqueuePodcast`, `runUpdateCheck` nehmen die
Priorität als Parameter und heben über `RaiseQueuedPriority`. Spotlight-Seite schreibt den Takt,
Update-Seite den Schalter über `admin.Schedules` in die Zeile. Abweichungen vom Papier in
**13.4** (u. a. `update.check` prüft 60 s nach jedem Start; `Type` hat zusätzlich `Priority` und
`Seed`; Run-Fehler lässt `next_run_at` stehen; `podcast.poll` von Hand bleibt für Schritt 3 offen).

**Tests.** `scheduler_test.go` mit injizierter Uhr und fester Zone UTC+2: Startverzögerung
(0, 90 s, keine; Neustart), Takt, `enabled = 0`, Fenster über Mitternacht (22:00–02:00, vier
Läufe), Fensteränderung verschiebt `next_run_at`, Takt kürzer als Fenster (1 h in 02:00–05:00:
dreimal je Nacht, 6 h einmal), pausiert feuert nicht, unbekannte Zeile, fehlgeschlagener `Run`,
Seed und bestehende Zeile, Prüfung der Grenzen, Registry. Je Jobart mit `Run` ein Test, dass ein
wartender Lauf angehoben wird: `TestRunScansRaisesWaitingScan`, `TestRunSweepRaisesWaitingSweep`
(`library`), `TestQueueDuePodcasts`, `TestTraktSchedule` mit Sonderfall in
`TestTraktRowsGoWithoutClientID` (`metadata`), `TestQueueDueUpdateCheck` (`admin`, mit
`fakeSchedules`), `TestSpotlightConfigSyncsSchedule`.

**DoD-Belege.**

```
$ go test ./internal/jobs/ -run Scheduler
ok  	github.com/slouchcat/slouchcat/server/internal/jobs	1.641s

$ go test ./internal/jobs/ ./internal/library/ ./internal/metadata/ ./internal/admin/ ./internal/offline/ ./internal/notify/ ./internal/api/ ./cmd/...
ok  	…/internal/jobs	5.072s   ok  …/internal/library	35.126s   ok  …/internal/metadata	26.635s
ok  	…/internal/admin	24.101s  ok  …/internal/offline	15.205s  ok  …/internal/notify	7.609s
ok  	…/internal/api	32.729s  ok  …/cmd/slouchcat	4.197s

$ go test ./internal/store/ -run 'TestFrozenStatesMigrate|Migration'   (alle 15 Stände durch 00027)
ok  	github.com/slouchcat/slouchcat/server/internal/store	195.335s

$ go tool golangci-lint run ./...
0 issues.

$ task ci
ci:server grün (gen, gen:check, lint, test); ci:web grün bis storybook:test, das in der Sandbox
nicht läuft (`sg docker`: setgroups: Vorgang nicht zulässig; kein web/src-Anteil in diesem PR,
die GitHub-CI schiesst die Baselines); danach von Hand:
$ task build        → server/bin/slouchcat v1.4.0-22-g8e95e3e-dirty
$ task ci:e2e       → 478 passed (25.1m); A-9 log scan: 75548 lines, 17 secrets of the run, 0 findings
```

Dev-Server (`sqlite3` ist auf dem Rechner nicht installiert, darum `python3 -I` mit dem
eingebauten `sqlite3`-Modul; Port 8096 ist vom laufenden Stack belegt, darum `--listen
127.0.0.1:18097` gegen `.dev/config/slouchcat.db`, frische Datenbank):

```
level=INFO msg="migration applied" version=27 file=00027_jobs_scheduler.sql duration_ms=17
level=INFO msg="schedule created" component=scheduler job_type=scan.library every=6h0m0s enabled=true next_run_at=2026-10-30T13:06:19.160Z
level=INFO msg="schedule created" component=scheduler job_type=library.sweep every=1h0m0s enabled=true …
level=INFO msg="schedule created" component=scheduler job_type=podcast.poll every=5m0s enabled=true …
level=INFO msg="schedule created" component=scheduler job_type=trakt.refresh every=24h0m0s enabled=true …
level=INFO msg="schedule created" component=scheduler job_type=update.check every=24h0m0s enabled=true …
level=INFO msg="scheduled run queued" component=scheduler job_type=library.sweep jobs=1
level=INFO msg="scheduled run queued" component=scheduler job_type=podcast.poll jobs=0
level=INFO msg="scheduled run queued" component=scheduler job_type=scan.library jobs=0
level=INFO msg="scheduled run queued" component=scheduler job_type=trakt.refresh jobs=0
level=INFO msg="job done" worker=worker-1 job_id=1 job_type=library.sweep attempt=1 duration=1.546532ms

SELECT type, next_run_at FROM job_schedules            (fünf Zeilen; jobs.prune kommt mit Schritt 2)
('library.sweep', 1793369179163)    = 15:06:19 lokal, in 1 h
('podcast.poll',  1793365879163)    = 14:11:19, in 5 min
('scan.library',  1793387179163)    = 20:06:19, in 6 h
('trakt.refresh', 1793451979163)    = 31.10. 14:06:19, in 24 h
('update.check',  1793365639160)    = 14:07:19, 60 s nach dem Start (start_delay_ms)
SELECT count(*) FROM job_types                          17 (die 16 Jobarten von heute plus podcast.poll), paused_at überall NULL
```

**Offen (Schritt 2 bis 5):** `Type.Cancel` setzen (`CancelScan`, `offline.CancelPreparation`),
Pause in `ClaimJob` und im Arbeiter, `started_at`/`finished_at`/Fortschritt schreiben (`jobs.Job`
trägt die Felder noch nicht), `jobs.prune`, eingefrorener Stand `testdata/db/` nach 00027 (8.3);
Vertrag 1.5.0 und `admin.Schedules`-Rückschreiben (Schritt 3); Seite (Schritt 4); CT 190 und
Messung (Schritt 5). Ein Tag Beobachtung der fünf Zeitpläne auf CT 190 vor dem Merge (12.1).

## Update-Prüfung: Warteschlangenplatz des täglichen Jobs (30. Oktober 2026)

Auf dem Zweig `claude/interesting-kalam-9fb6ca` (ab `main c6741b4`, Release 1.4.0). Befund: der
Job `update.check` stand mit Priorität 1 in der Warteschlange, auf derselben Stufe wie
`probe.keyframes` eines Scans, die Episodenbilder von `image.fetch` und `trailer.find`; eine Stufe
darunter liegen `segments.detect`, `metadata.fill` und die Poster (2). Der Worker nimmt Jobs nach
`priority DESC, created_at, id` (`server/queries/system/jobs.sql`), also läuft bei gleicher
Priorität der älteste zuerst. Nach einem Bibliotheksimport wie auf CT 190 (Zehntausende Jobs auf
den Stufen 1 und 2, tagelang) wartete die zehnsekündige Manifest-Abfrage hinter dem ganzen Rückstau,
und die Verwaltung zeigte tagelang ein veraltetes Release; derselbe Befund wie bei «Jetzt abrufen»
der Trakt-Liste in M11.

**Fix, ein Commit `fix(admin): …`:** `updatePriority` in `server/internal/admin/update.go` ist neu
**8**, über jedem Massen-Job (Scans 5/6, Matches 3/4, Proben 3, Bilder/Segmente/Fill 2, Keyframes 1)
und über den manuellen Aktionen (7); nur der Keyframe-Index einer Wiedergabe (9) geht vor. Die
Leiter aller Job-Typen steht neu an einer Stelle, in der Paketdokumentation von
`server/internal/jobs/queue.go`; die Konstanten bleiben beim jeweiligen Paket. Test:
`TestQueueDueUpdateCheck` reiht zuerst einen älteren `probe.keyframes` (1) ein und prüft, dass
`Claim` den `update.check` mit Priorität `updatePriority` zuerst nimmt (bei Priorität 1 rot,
geprüft). `task ci:server` grün.

**Nicht gebaut:** der nächtliche `trakt.refresh` (Priorität 2, `metadata/trakt.go`) hat denselben
Platz hinter dem Rückstau; nur der manuelle Abruf läuft auf 7. Offen als Folgepunkt, nicht Teil
dieses Fixes.

## CI-Fehler e2e (shard 4) in Firefox nach Konzept 08 (30. Oktober 2026)

Auf dem Zweig `claude/heuristic-moore-3a82ad` (ab `main 1bb1c7e`). Seit PR #47/#48 war der Job
«e2e (shard 4)» auf `main` rot: `audiobook.spec.ts:110` («the book's detail page by keyboard»)
fiel in Firefox zweimal je Lauf (Versuch und Wiederholung), Chromium grün. Zwei Ursachen, zwei
Commits, Test und Seite unverändert in ihrer Erwartung.

**Ursache 1 – Umbruch der Aktionszeile (`3911556`).** Der CI-Runner hat keine Noto/Inter, Firefox
fällt auf DejaVu Sans zurück (breiter). Mit der Seitenleiste ist die Inhaltsspalte schmaler, die
Aktionszeile der Detailseite bricht um, und «Verwalten» (Konto `admin`) steht unter «Abspielen».
norigin sucht das Ziel von ↓ zuerst geometrisch unter den Geschwistern derselben Gruppe und fand
den umgebrochenen Knopf statt «Merken» in der Zeile darunter. Der Screenshot im Blob-Report des
Laufs 41026001430 zeigt den Fokusring auf «Verwalten». Lokal (Noto Sans) passte alles in eine Zeile;
reproduziert mit `letter-spacing: 0.12em` per `addStyleTag`. **Fix:** `SpatialGroup` kennt
`ordered`: ←/→ laufen die Kinder in DOM-Reihenfolge ab (`nextInOrder`), ↑/↓ und die Zeilenenden
gehen an die Elterngruppe (Geometrie). Beide Aktionszeilen der Detailseite (`.actions`,
`.listActions`) nutzen das; es ist die Reihenfolge, die der Kommentar dort schon versprach.
Unit-Test `spatial.test.ts`.

**Ursache 2 – `admin-library-folders.spec.ts:71` («Tab did not reach the element within 80
presses», `5d41049`).** Nicht dieselbe Ursache. `resetFocus` blurte nur; der Startpunkt der
sequenziellen Fokusnavigation blieb beim zuletzt fokussierten Element. Nach dem Klick auf
«Bibliothek mit 1 Ordner hinzufügen» (die Spec legt ihre Bibliothek im `beforeEach` über das
Formular an) lief Tab also vom Formular am Seitenende weiter: Chromium springt am Ende an den
Anfang und erreichte die Karte oberhalb noch, Headless-Firefox bleibt auf dem letzten Element
stehen. In CI blieb das verborgen, solange der Chromium-Lauf derselben Datei im selben Shard die
Bibliothek vorher anlegte; im roten Lauf 41026001430 liegt Chromium in Shard 1 und Firefox in
Shard 4 (die drei neuen Specs von PR #47 verschoben die Shard-Grenze), und die Wiederholung fand
die Karte vor, darum «flaky». Die Logs der letzten grünen Läufe liefert `gh run view --log` nicht
mehr, der Split ist nur im roten Lauf belegt. Lokal mit
`--project=firefox` allein deterministisch rot. **Fix im Helfer:** `resetFocus` fokussiert und
entfernt eine Sonde am Anfang von `<body>`; damit beginnt Tab in beiden Browsern oben, wie der
Helfer es verspricht.

**Belege (DoD).**

- `pnpm exec playwright test e2e/audiobook.spec.ts e2e/admin-library-folders.spec.ts
  --project=firefox --project=chromium --retries=0`: `28 passed (1.1m)`.
- `pnpm exec vitest run src/ui/spatial.test.ts`: `3 passed`; `tsc --noEmit`: 0 Fehler.
- `task build`, `SLOUCHCAT_E2E_PORT=8197 task web:e2e` (ganze Suite, Chromium und Firefox):
  `478 passed (25.5m)`, kein Fehler, nichts flaky.
- `task ci:web` ohne `storybook:test` (gen, gen:check, lint, Vitest): `84 passed (84)` Dateien,
  `673 passed (673)` Tests; Storybook-Baselines wie bei Konzept 08 auf CT 190 (kein Docker hier).
- `task ci:server`: alle Pakete `ok` bis auf `internal/library`
  (`TestPodcastFolderPairsFileWithFeedEpisodeAfterProbe`): die lokalen Podcast-Dateien stammten vom
  22. Oktober, `generate-media.sh` schreibt den `date`-Tag erst seit `193fb22` (29. Oktober) und
  überspringt vorhandene Dateien. Nach `rm testdata/media/Podcasts/episode-*.mp3` und
  `task testdata:media` läuft der Test grün (`ok 4.559s`); in CI (frische Medien) war der Job
  «server tests» auch vorher grün. Wer alte Testmedien hat: `FORCE=1 task testdata:media`.

**Nachtrag CI des PR (`web`-Job rot, Lauf 41034366273).** Alle sechs e2e-Shards grün, aber Vitest
meldete einen unbehandelten Fehler: `ReferenceError: document is not defined` aus react-arias
`LiveAnnouncer`, 100 ms nach der ersten Ansage, als jsdom nach `LoginPage.test.tsx` schon abgebaut
war. Ursache: `@testing-library/react` setzt `IS_REACT_ACT_ENVIRONMENT` nur, wo Jest-Globals
(`beforeAll`/`afterAll`) existieren; unser Vitest läuft ohne `globals`, und ohne das Flag
verzögert react-aria die erste Ansage um 100 ms (Safari-Umweg). Zeitabhängig, lokal nicht
aufgetreten; deterministisch nachgestellt mit `announce()` und anschliessend gelöschtem
`globalThis.document`. **Fix:** `web/src/test/setup.ts` setzt das Flag wie unter Jest; die Ansage
geschieht sofort. Danach volle Vitest-Suite `674 passed (674)`, keine act-Warnungen, `tsc` 0 Fehler.

**Offen.** Die CI entscheidet über die Firefox-Schrift (DejaVu Sans); lokal (Noto Sans) bricht die
Aktionszeile nicht um, der e2e-Test deckt den Umbruch also nur in CI ab, der Unit-Test die
Reihenfolge in jedem Fall.

## Update-Prüfung: Entwicklungsstand statt Update-Hinweis, Release 1.4.0 (30. Oktober 2026)

Auf dem Zweig `claude/update-check` (ab `main 1bb1c7e`, Startseite und Posterreihe gemergt). Auslöser:
CT 190 lief auf dem Image `spotlight-marker-f92c09f` und zeigte unter «Zustand des Servers» «Version
1.1.0 ist verfügbar», obwohl der Stand drei Vertragsfassungen neuer war. Zwei Ursachen: der
Image-Tag ist keine Semver-Version, und `newerVersion` in `admin/update.go` wertete jede
unparsbare Version als älter als jedes Release; dazu war nur 1.1.0 veröffentlicht, während
`api/openapi.yaml` längst auf 1.4.0 stand (`docs/INSTALLATION.md` zeigte auf Adressen unter
`v1.4.0`, die es nicht gab). Drei Schritte, je ein Commit:

1. **`build(ci,deploy): stamp git describe into every image`.** Der Image-Job der CI holt die ganze
   Historie (`fetch-depth: 0`) und stempelt `git describe --tags --always --dirty` statt
   `github.sha`; `deploy/Taskfile.yml` beschreibt auch leichte Tags (`--tags`); der Baubefehl in
   `docs/dev/HANDOVER.md` gibt `VERSION` mit. Ein Commit nach einem Release heisst damit
   `v1.4.0-12-gabc1234`, was `version.Parse` als dieses Release liest (der `describe`-Suffix war
   im Parser schon vorgesehen, nur kam nie einer an).
2. **`feat(admin,web): report a development build instead of an update`.** `UpdateState` bekommt
   das Pflichtfeld `development` (Vertrag 1.4.0, additiv, noch nicht veröffentlicht): wahr, wenn
   `currentVersion` keine Semver-Version ist (Commit-Hash, Image-Tag, `dev`). Dann bleibt
   `available` falsch, der Header trägt kein Abzeichen (`UpdateAvailable` folgt `Available`), und
   die Verwaltung zeigt «Du verwendest einen Entwicklungsstand» mit dem laufenden Stand und dem
   neuesten Release daneben (`admin.home.update.development.*` in beiden Sprachen). `newerVersion`
   meldet bei unparsbarem `current` nicht mehr «neuer»; `isRelease` entscheidet. Tests:
   `TestUpdateCheckDevBuild` (kein Hinweis, kein Abzeichen), `TestIsRelease`, die Tabelle von
   `TestNewerVersion` mit `spotlight-marker-f92c09f`, im Web «names a development build instead of
   offering an update for it». Die Vertragsfixture lief bisher als Version `test` und wäre jetzt
   ein Entwicklungsstand; `SystemSource.Version` ist neu `0.0.0-test` (Semver-Vorabfassung, also
   älter als `0.0.1`), die Backup-Fixture behält `test` (Dateiname `slouchcat-backup-test-…`).
3. **Release 1.4.0 vorbereitet.** Notes-Entwurf als Tag-Beschriftung (Konten 1.2.0, Bibliotheken
   1.3.0, Startseite 1.4.0, die drei Vertragsstufen mit beiden Brüchen wörtlich, Migrationen
   00022 bis 00026, `metadata.vote_backfill`), Tag-Befehl und die Regel **Vertragsanhebung =
   Tag beim Merge** in `docs/dev/HANDOVER.md`; `deploy/README.md` erklärt den Entwicklungsstand.
   Den Tag `v1.4.0` setzt Marco nach dem Merge dieses Zweigs selbst auf den Merge-Commit; vorher
   bleibt der Hinweis auf CT 190 stehen, bis dort ein Release-Image läuft.

**Befund der CI (PR #49, erster Lauf):** der e2e-Test «a newer release in the manifest shows the
update badge» (A-5) fiel in beiden Browsern, weil `task build` den Server mit `git describe` ohne
`--tags` auf einem flachen Checkout stempelte, also mit dem blossen Commit-Hash; seit Schritt 2 ist
das ein Entwicklungsstand ohne Hinweis. Vierter Commit `build(ci,server)`: `server/Taskfile.yml`
beschreibt gegen Tags, der Job `build` holt die ganze Historie. Die roten Storybook-Screenshots
(AppHeader, HeroCarousel, LetterBar, PosterCard, SelectField, SideNav, SideNavDrawer) sind der
offene Punkt aus Konzept 08: `main` war auf `fa642f1` und `1bb1c7e` im Job «web» schon rot,
Baselines holt Marco auf CT 190 nach.

**Veröffentlicht am 30. Oktober 2026:** PR #49 von Marco gemergt (Merge `c6741b4`), Tag `v1.4.0`
auf `c6741b4` (Beschriftung = Release-Notes), Release-Lauf 41034208236 grün: Image
`ghcr.io/slouchcat/slouchcat:1.4.0` (amd64, arm64, anonym ziehbar), `manifest.json` unter
`releases/latest/download` mit `version 1.4.0`. Erste Fassung, die 1.2.0 und 1.3.0 nachträgt. Offen:
Storybook-Baselines von Konzept 08 (eigener Zweig ab `main`), der Firefox-Test
`audiobook.spec.ts:110` (eigene Session), CT 190 auf `1.4.0` umstellen.

**Nicht gebaut:** keine Änderung an der Prüfung selbst (Manifest, Takt, Job), kein Abgleich mit
Vorabfassungen (`1.5.0-rc.1` zählt wie bisher als älter als `1.5.0`).

## Startseite, Seitenleiste und Bibliotheksansichten (Konzept 08, 30. Oktober 2026, in Umsetzung)

Auf dem Zweig `claude/startseite` (ab `main 4049724`, Konzept 07 gemergt) nach dem Konzept
`docs/planning/08-startseite.md` (Fassung 3), den Entscheiden in **Frage 14** des
Entscheidungsprotokolls und **ADR 0016** (Teilnachfolger von ADR 0011). Ein Commit je Planschritt;
vor jedem Abschluss war `task ci` grün (ohne `storybook:test`: auf dem Entwicklungsrechner ist
kein Docker erreichbar, Baselines holt Marco auf CT 190 nach, wie bei Konzept 07).

**Was sich ändert.** Die Zuschauerseite bekommt eine Seitenleiste («Meine Medien» mit einem
Eintrag je sichtbarer Bibliothek und «Cat's Empfehlungen»), eine Bibliotheksseite je Bibliothek
(ein Raster mit Chips, acht Sortierungen, Filtern, Buchstabenleiste) und eine Seite «Cat's
Empfehlungen»; das Spotlight-Band beginnt je Seitenaufruf mit einem zufälligen Titel, läuft im
einstellbaren Takt und zeigt darunter eine Posterreihe in Normalgrösse.

### Schritt 0 — Entscheide und Papiere (`dca8efc`)

Frage 14 mit F-1 bis F-12, N-1 bis N-4 und O-1 bis O-5 im Entscheidungsprotokoll, ADR 0016,
Ablösungshinweis in ADR 0011, Papier auf «freigegeben, in Umsetzung», Nummern nach dem Merge von
07: **Migration 00026** (00025 hat `items.filled_providers` aus dem Review von 07 belegt), ADR 0016,
Vertrag 1.4.0. Reiner Doku-Commit; Code identisch mit `main`.

### Schritt 1 — Vertrag 1.4.0, Migration 00026 und Server

**Vertrag 1.4.0, additiv:** `listVisibleLibraries` (`GET /libraries/visible`), `listLibraryItems`
(`GET /libraries/{id}/items` mit `content`, `sort`, `order`, `genre`, `yearFrom`, `yearTo`,
`maxRating`, `status`, `letter`, `offset`, `limit`), `listPicks` (`GET /picks`), alle `scoped`;
`getHome` mit `start`; `HomePage.heroStart` und `heroIntervalSeconds`;
`SpotlightConfig.rotationSeconds` (Pflicht) und `SpotlightConfigUpdate.rotationSeconds`
(optional, fehlt = unverändert); Schemata `LibraryNav`, `LibrarySort`, `LibraryItemStatus`,
`LibraryFacets`, `LibraryItemsPage`, `PicksPage`. Alle drei Operationen stehen im
`profileRegistry` (`TestContractCoversEveryOperation`).

**Migration 00026** (`server/migrations/00026_library_views.sql`): `spotlight_config.rotation_seconds`
(3–15, Vorgabe 5), `items.vote_average REAL` (0–10), `item_texts.sort_key` mit Backfill in zwei
`UPDATE`s (sqlc kennt kein `UPDATE … FROM`; `lower()` faltet nur ASCII, darum die üblichen
Latin-1-Buchstaben per `replace`), Indizes `items_library_kind_added`, `_released`, `_vote` und
`item_texts_sort_key`. Der genaue Schlüssel ist `model.SortKey` (NFD, Markierungen weg, ß→ss,
Artikel je Sprache); jeder Schreiber einer `item_texts`-Zeile setzt ihn (`UpsertItemText` mit
`@sort_key`: Scanner, Hörbuch- und Podcast-Ordner, Feeds, Match, Fill).

**Server.** `visibility.VisibleLibraries` / `LibraryItems` / `PicksPage` (`server/internal/visibility/library.go`,
feste Abfragen in `server/queries/scoped/library.sql`, die dynamische Seite als Statement-Builder
wie die Suche; Seite, `total`, Buchstaben, Genres, Freigaben in **einer** Lesetransaktion;
Serienstatus über die sichtbaren Folgen, exklusiv). `discover.Home(ctx, profile, start)` würfelt
`heroStart` über die Bühnenreihe (`WithRand`, `WithHeroStart`; `SLOUCHCAT_HERO_START` /
`--hero-start`, -1 = Zufall; die e2e-Umgebung setzt 0) und trägt `rotation_seconds` als
`HeroIntervalSeconds`. Das Vormalen (`preload.go`) bleibt bis Schritt 4 bei Platz 1.
`metadata`: `vote_average` als Feld in der Auffüllreihenfolge (TMDB `vote_average`, AniList
`averageScore` ÷ 10; **TVDB v4 liefert keine 0–10-Wertung**, nur einen Beliebtheits-`score` –
Abweichung vom Papier 4.2, dort und in ADR 0016 nachgetragen), auch beim Match des Erkenners; Job
`metadata.vote_backfill` in Batches zu 200 mit Cursor, Markierung «gefragt, keine Wertung» als
`item_field_sources`-Zeile ohne Wert, einmal beim Serverstart und nach jedem vollständigen Scan
(`QueueVoteBackfill`, idempotent). `admin`: `rotationSeconds` 3–15, nil = unverändert.

**Belege (DoD).**

- `task ci:server` (gen, gen:check, lint, Tests mit `-race`): `0 issues.`, alle Pakete `ok`,
  darunter `internal/visibility 174.7s`, `internal/api 402.0s`, `internal/store 525.2s`
  (`TestFrozenStatesMigrate` über alle eingefrorenen Stände bis `bibliotheken.sql`,
  `TestScopedQueriesJoinVisibleItems` mit `library.sql`), `exit=0`.
- `task web:gen:check && task web:lint && task web:test`: grün (`exit=0`; die Vertragsänderung
  brauchte nur `rotationSeconds` in `SpotlightPage.test.tsx`).
- Negativtest K-3 (`server/internal/api/contract_library_test.go`): P0/P6/P12 sehen in Bibliothek 1
  keinen Titel über ihrer Stufe, `maxRating=18` ergibt dieselbe Seite wie ohne, `facets.ratings`
  bleibt unter der Grenze; `px` bekommt für Bibliothek X 404, eine leere und eine unbekannte
  Bibliothek 404; `letter` ohne `sort=title` 422 `letter_requires_title_sort`, `sort=author`
  ausserhalb von Hörbüchern 422; `getHome?start=K` wiederholt K, ausserhalb wird gewürfelt;
  `listPicks` entspricht der Reihe ohne Klemme. Das `curl`-Gegenstück mit einem Kinderprofil am
  laufenden Server folgt in Schritt 5 auf dem M3-Baum (dort steht ohnehin ein gefüllter Server).
- `EXPLAIN QUERY PLAN` der Sortierungen (Fixture `vistest`, temporärer Test): `title`, `added`,
  `released`, `vote` nehmen `SEARCH i USING INDEX items_library_kind_vote (library_id=? AND kind=?)`
  (der Planer wählt den ersten passenden `(library_id, kind, …)`-Index; die DESC-Spalte nutzt er
  wegen des `visible_items`-Joins nicht, die Sortierung läuft über `USE TEMP B-TREE FOR ORDER BY`),
  `played` mit Status materialisiert `ep` einmal (`MATERIALIZE ep`, `GROUP BY` über die sichtbaren
  Folgen). Ob das auf dem M3-Baum unter 300 ms p95 bleibt, misst Schritt 5 (Ausstiegsklausel 8).
- `TestSortKey`/`TestSortLetter` (`server/internal/model/sortkey_test.go`): «Der Herr der Ringe» →
  `herr der ringe`, «Die Ärzte» → `arzte`, «Straße der Besten» → `strasse der besten`, «The Matrix»
  → `matrix`, «2001» → `#`.
- Entscheide beim Bauen: eine Podcast-Bibliothek führt `podcast` immer in `contents` (Feed-Abos haben
  keinen Ordner, H-2); eine angefangene Serie trägt `progress.completed = false` bei Position 0
  (die Karte zeigt dann keinen Balken); Gleichstand bricht der Sortierschlüssel, dann die Id;
  Unbekanntes (`NULL`, leerer Autor) steht in beiden Richtungen am Ende; `WithHeroStart(k)` jenseits
  der Reihe nimmt den letzten Titel, ein gültiges `start` aus der Anfrage gewinnt immer.

### Schritt 2 — Bibliotheksseite und Seite «Cat's Empfehlungen»

Routen `/libraries/$libraryId` (`web/src/features/catalog/LibraryPage.tsx`, Adresszustand in
`libraryParams.ts`: `content`, `sort`, `order`, `letter`, `genre`, `status`, `from`, `to`,
`rating`; ungültige Werte fallen weg, `letter` nur mit Sortierung nach Titel) und `/picks`
(`PicksPage.tsx`), beide lazy unter `_chrome`. Aufbau nach Papier 7.3: Kopf mit Zähler
(«312 Titel», «41 Titel mit B», «… · gefiltert»; Hörbücher und Podcasts mit eigenem Nomen),
Werkzeugzeile mit Chips «Alle · Filme · Serien» nur bei mehr als einer Inhaltsart, Sortierung als
`<select>` (neues `web/src/ui/SelectField.tsx`; «Autor» nur bei Hörbüchern, «Altersfreigabe» nur
bei Video), Richtungsknopf mit Text, «Filter» mit Zähler und ausklappbarem Feld (Genre und Freigabe
nur, wenn `facets` Werte nennen – Regel 12; Status mit «Ungehört / Gehört» bei Audio; Escape
schliesst und gibt den Fokus an «Filter» zurück, gefangen vor der Zurück-Taste der Seite),
Buchstabenleiste (neues `web/src/ui/LetterBar.tsx`, nur bei Sortierung nach Titel und nicht bei
Podcasts; ab 768 px als klebende Spalte rechts, darunter als Lauf über dem Raster; Buchstaben ohne
Treffer `disabled` und für die Pfeiltasten nicht registriert), Raster mit drei Spalten unter 768 px,
«Mehr laden (N weitere)». `PosterCard` bekommt den Haken (`watchedLabel`, «Gesehen» / «Gehört»)
und nutzt das vorhandene `badge` für die Freigabe («FSK 12», nur bei Video); `ItemGrid` bekommt
`showWatched`, `showRating`, `className`. `Row` bekommt `moreHref`/`moreLabel` («Alle anzeigen ›»
nur an Cat's Empfehlungen und Merkliste, O-3). Neu ausserdem `useBackFocus`: die Zurück-Taste des
Browsers landet wieder auf der verlassenen Karte (Regel 8), je Verlaufseintrag gemerkt.
`YearField` aus der Suche nach `web/src/features/discover/YearField.tsx` ausgelagert.

**Belege (DoD).**

- `task web:lint && task web:test`: grün (82 Dateien, 661 Tests im Web-Paket, dazu api-client und
  design-tokens).
- `task build`, dann `SLOUCHCAT_E2E_PORT=8197 task web:e2e` (ganze Suite, Chromium und Firefox):
  `456 passed, 2 failed (24.1m)` – die zwei Fehler waren Firefox-Eigenheiten in den neuen Specs
  (kein `:focus-visible`-Ring nach einem Skript-Fokus, wenn die letzte Eingabe ein Mausklick war);
  nach der Korrektur der zwei Specs: `library.spec.ts` und `picks-page.spec.ts` in beiden Browsern
  `9 passed` / `2 passed`. Tastatur-Belege: Tab-Reihenfolge Kopf → Chips → Sortieren → Richtung →
  Filter → Leiste → Raster, Pfeiltasten ins Raster und hinüber zur Buchstabenleiste, Enter öffnet
  die Detailseite, Zurück landet auf derselben Karte mit derselben Adresse, Escape im Filterfeld,
  kein Fokusfalle (`expectNoFocusTrap`); auf 390 px drei Spalten und die Leiste als Lauf.
- «Mehr laden» mit Fokus auf der ersten neuen Karte: die Suite hat unter 50 Titel je Bibliothek,
  darum belegt es der Vitest-Test der Seite «Cat's Empfehlungen» (zweite Seite) und die
  bestehende `MoreButton`-Logik; am laufenden Server prüft es Schritt 5 auf dem M3-Baum.
- Drei bestehende Specs brauchten nur neue Locatoren, weil «Alle anzeigen ›» der erste Link der
  Reihe ist (`admin-picks.spec.ts`, `home.spec.ts`).
- Storybook-Vergleichsbilder (`PosterCard` Watched/StartedWithRating/SquareHeard, `Row`
  WithMoreLink, `SelectField`, `LetterBar`) fehlen: kein Docker auf dem Entwicklungsrechner;
  Marco holt sie wie bei Konzept 07 auf CT 190 nach (`task storybook:update`).

### Schritt 3 — Seitenleiste, Schublade, Vormalen, Offline

`web/src/ui/SideNav.tsx` (ein Bauteil, zwei Darstellungen) mit dem `<nav>`-Namen «Hauptnavigation»:
ab 768 px eine feste Spalte links (Token `size.sidenav.width` = 15 rem, als `--sidenav-width` in
`global.css`: 0 unter 768 px und bei `html[data-sidenav="hidden"]`), darunter oder bei
ausgeblendeter Leiste eine Schublade (`SideNavDrawer.tsx`, react-aria `Modal`/`Dialog`, lazy
geladen wie der Trailer-Dialog, damit die Startseite ohne react-aria bleibt, P-10). Inhalt ohne
Symbole (N-2): Start, Suche, Merkliste, Downloads; Gruppe «Meine Medien» (ein- und ausklappbar,
Zustand je Gerät in `localStorage`, Lesen und Schreiben in `try/catch`) mit einem Eintrag je
sichtbarer Bibliothek samt Titelzahl (`listVisibleLibraries`, mit der Sitzung vorgeladen) und
«Cat's Empfehlungen» mit Zahl (`listPicks?limit=1`); unten Verwaltung (Update-Hinweis am Eintrag,
O-5), Profile, Abmelden, «Seitenleiste ausblenden». Offline oder bei Fehler sagt die Gruppe «Offline:
Bibliotheken nicht verfügbar» mit Verweis auf Downloads; die Abfrage blockiert die Hülle nicht.
Die Kopfzeile behält Menü-Symbol (nur ohne Spalte), Logo, Suche, Glocke, Profil; Pillen,
Update-Hinweis und Abmelden ziehen in die Leiste. Ein Inline-Skript in `web/index.html` setzt
`data-sidenav` vor dem ersten Malen; `#hero-prepaint`, `#poster-prepaint` und der Inhalt rücken um
`--sidenav-width`; `sizes` der Bühnenbilder ist `(width >= 768px) calc(100vw - 15rem), 100vw`
(`heroSizes` in `preload.go`, `HERO_SIZES` in `HeroCarousel.tsx`). Räumliche Navigation:
`AppLayout` führt Kopfzeile, Spalte und Inhalt in einer Gruppe mit Richtungstabelle (Inhalt links →
Spalte auf dem Eintrag der aktuellen Seite, Spalte rechts → zuletzt fokussiertes Element im
Inhalt, hoch → Kopfzeile); `SpatialGroup` bekommt `nextFocusKey`, `focusBoundary` und `tabIndex`.
Tab-Reihenfolge: Sprunglink → Kopfzeile → Spalte → Inhalt. Im Admin-Bereich bleibt `AdminNav`.

**Belege (DoD).**

- `task web:gen:check && task web:lint && task web:test`: grün (83 Dateien, 666 Tests, darunter
  `SideNav.test.tsx`); `task ci:server`: `0 issues.`, alle Pakete `ok` (`preload_test.go` mit
  `data-start`-losem Stand dieses Schritts, drei Vorladungen).
- `task build`, `SLOUCHCAT_E2E_PORT=8197 task web:e2e` (ganze Suite, Chromium und Firefox):
  `467 passed, 3 failed (24.9m)`. Zwei Fehler waren Firefox: der scrollende `<nav>` der Spalte
  war dort eine eigene Tab-Station (Tab-Reihenfolge in `home.spec.ts` und `sidenav.spec.ts`);
  behoben mit `tabIndex={-1}` auf Spalte und Schubladenkörper. Der dritte
  (`admin-streams.spec.ts`, Firefox, «stops within 5 s») ist zeitabhängig und ohne Bezug zu diesem
  Schritt (die Suite lief, während der nächste Agent baute); der Nachweis für beide folgt im
  Gesamtlauf von Schritt 4.
- `sidenav.spec.ts` (Chromium, 6 Tests): Spalte bei 1280 px mit allen Einträgen, Pfeile hoch/runter,
  Enter öffnet die Bibliotheksseite, Pfeil rechts in den Inhalt und links zurück; Ausblenden mit
  Zustand über `reload`, Schublade zeigt «Seitenleiste einblenden»; Schublade bei 390 px mit Fokus im
  Dialog, kein Scrollen dahinter, Escape mit Fokusrückgabe; Offline-Zustand der Gruppe
  (`/libraries/visible` abgebrochen, Service Worker blockiert); Geometrie-Test: `#hero-prepaint`
  deckt sich mit der Bühne und `#poster-prepaint` mit der ersten Karte, Spalte ein- und
  ausgeblendet, innerhalb 1 px.
- Lighthouse mobile und desktop (DoD dieses Schritts) laufen in Schritt 5 zusammen mit der
  Messung nach Papier-Abschnitt 8, weil der Messaufbau (Lighthouse-CLI, Testfilme vom NAS mit
  Bildern) dort ohnehin steht.
- Bekannte Lücke: in einer Bibliothek mit nur einer Inhaltsart beginnt die Werkzeugzeile mit dem
  `<select>`; Pfeil links von dort erreicht die Spalte nicht, weil `SpatialPage` Pfeiltasten in
  Auswahlfeldern dem Feld lässt. Raster und Chips erreichen sie.
- Storybook-Vergleichsbilder fehlen (kein Docker): `AppHeader` SearchFocused/WithMenu, `SideNav`,
  `SideNavDrawer`, die Admin-Seiten (15 rem schmaler); die alten `ui-appheader--nav-focused.png`
  und `ui-appheader--with-update.png` sind entfernt.

### Schritt 4 — Spotlight nach F-2, F-3, F-5, F-9, F-10 und die Reihen nach N-3, N-4

**Reihen.** `Row` bekommt `RowList` (Scroller und Liste mit der Reihengeometrie, wahlweise mit
Leerraum am Ende); alle Reihen rasten mit `scroll-snap-type: x mandatory` an der linken Kante
ein, der Scroller ist ein Grössen-Container: unter 768 px ist eine Karte
`(100cqi − 2·Gutter − 2·Abstand) / 3` breit (genau drei ganze Cover, N-3), ab 768 px gelten die
Poster-Tokens. `CARD_SIZES` (`PosterCard`, `CollectionCard`), `posterSizes` (`preload.go`) und
`#poster-prepaint > img` tragen den Eintrag `calc(33vw - 1.5rem)` unter 768 px im selben Commit.
Der Scroller hat `tabIndex={-1}` (Firefox macht Scrollboxen sonst zu Tab-Stationen).

**Band.** `HeroCarousel` neu: unter der Bühne eine `PosterCard`-Reihe in Normalgrösse in der neuen
Variante ohne `href` (`onPress`, `active` mit Ring in `--sc-item-accent`, `dimmed` 70 %,
`aria-current`, Zeitbalken als Kind im Rahmen); Enter oder Klick stellt den Titel ins Band und
navigiert nicht (F-5). Die Reihe steht, solange das aktive Poster ganz sichtbar ist (die Markierung
wandert, nicht die Poster; Marcos Nachtrag vom 30. Oktober 2026, zuvor rückte sie bei jedem Wechsel
nach); liegt es ausserhalb, rückt es nur horizontal an die linke Kante, der Leerraum am Ende trägt
auch das letzte Poster dorthin (N-4). Start bei `heroStart`, einmal je
Einhängen eingerastet (ein Refetch mit anderem Wert bewegt das Band nicht); Takt aus
`heroIntervalSeconds`. Pausenregel: Trailer, Pause-Knopf (`aria-pressed`, Symbol wechselt, Label
bleibt «Pause» nach der WAI-ARIA-Regel für Umschalter, «Weiter» als `title`), **Tastaturfokus im
Band** (`focusin` prüft `:focus-visible`, eine Taste setzt es, `focusout` löscht es), verborgener
Tab (`visibilitychange`), eine einzige Folie; **kein** Halt beim Mauszeiger und nicht nach einem
Mausklick (F-3); bei reduzierter Bewegung keine Automatik und kein Zeitbalken. Die drei Bandknöpfe
merken sich den Titel beim `pointerdown` und navigieren beim `click` dorthin (12.1). Nach dem
ersten Bild werden die nächsten zwei vorgeladen. Ein Mausdruck auf ein Spotlight-Poster vergibt
keinen Fokus (sonst scrollte die Pfeilnavigation die Reihe auf die Mitte und der Klick ging
verloren – Befund der e2e-Tests). Positionsanzeige «3 von 7» sichtbar neben dem Pause-Knopf.

**Ein Wurf je Seitenaufruf** (Papier 6, der Blocker des Reviews). `discover.FirstHero` liefert
Titel und Index und würfelt mit demselben `drawHeroStart` wie `Home`; `preload.go` schreibt
`data-start="K"` an `#hero-prepaint` und die Vorladung `</api/v1/home?start=K>`; steht die Bühne
weiter unten oder fehlt das Backdrop, trägt die Hülle `<meta name="slouchcat-hero-start"
content="K">`. Der Client (`shellHeroStart()` in `queries.ts`) schickt `start` nur beim ersten
Abruf der Sitzung (`dataUpdatedAt === 0`), danach würfelt der Server und das Band bleibt
eingerastet. Admin-Seite «Spotlight» bekommt das Feld «Takt der Bühne … Sekunden» (3–15).

**Belege (DoD).**

- `task build`, `SLOUCHCAT_E2E_PORT=8197 task web:e2e` (ganze Suite, Chromium und Firefox):
  `477 passed, 1 failed (25.2m)`; `task ci:server`: `0 issues.`, alle Pakete `ok`;
  `task web:gen:check && task web:lint && task web:test`: grün (83 Dateien, 670 Tests).
  Der eine Fehler ist `admin-streams.spec.ts` in Firefox («its player stops within 5 s»): ein
  Zeitbudget unter der Last der ganzen Suite; allein läuft der Test in Firefox zweimal grün
  (`--repeat-each=2`, 5.5 s und 6.1 s). Der Test hat mit diesem Konzept nichts zu tun; in der
  CI läuft die Suite in sechs Shards.
- `home.spec.ts` (neu oder umgeschrieben): Band beginnt bei `SLOUCHCAT_HERO_START` (`data-start`
  bzw. Meta = 0, `Link </api/v1/home?start=0>`), `GET /home?start=3` liefert `heroStart 3`, ein
  Refetch nach `visibilitychange` ändert den Bandtitel nicht; Enter auf einem Poster wechselt das
  Band ohne Navigation; Tab ins Band hält den Takt (`data-paused`), ein Mausklick nicht, der
  Pause-Knopf hält und löst; aktives Poster nach jedem Wechsel an der linken Kante, `scrollY`
  bleibt; bei 390 px drei ganze Cover je Reihe, nach `scrollBy` rastet ein Cover am Gutter ein;
  Tab-Reihenfolge Abspielen → Details → Pause → aktives Poster. `admin-spotlight.spec.ts`: Takt 7
  speichern, nach Reload 7, `GET /home` liefert `heroIntervalSeconds 7`.
- `preload_test.go`: mit `WithHeroStart(2)` trägt die Hülle `data-start="2"` und den `Link`
  `/home?start=2`; Bühne weiter unten nur das Meta-Element; ohne Bühne keines von beiden; ein
  Zufallswurf nennt in Hülle, Vorladung und `GET /home?start=K` denselben Titel.
  `discover_test.go`: `FirstHero` mit `WithHeroStart(3)` und `(9)` (auf den letzten Titel geklemmt),
  `Home(start=k)` wiederholt den Wurf.
- `HomePage.test.tsx`: Takt 7 s aus der Antwort, eingerasteter `heroStart`, `?start=2` nur beim
  ersten Abruf, Klick hält nicht, Taste hält, Pause-Knopf, `pointerdown`-Merker, Vorladen der
  nächsten zwei Bilder.
- Storybook-Vergleichsbilder (`ui-herocarousel--*`, `ui-row--*`, `ui-postercard--button-*`) fehlen,
  kein Docker; `task storybook:update` auf CT 190.

### Schritt 5 — Abnahme, Messung, Doku

**Messung** nach Papier-Abschnitt 8 in `docs/perf/startseite.md` (Entwicklungsrechner, 30. Oktober
2026): auf dem M3-Baum (2'500 Filme, 400 Serien mit 40'334 Folgen) `GET /home` **58.5 ms p95**
(während des Scans 34.9 ms; P-11 < 300 ms), die Bibliotheksseite in jeder Sortierung unter 300 ms
p95 (schwerster Fall «Serien, Status angefangen, Titel, Buchstabe S» **198.4 ms**, `sort=played`
196.4 ms), `GET /libraries/visible` 50.0 ms p95, `GET /picks` 2.9 ms. Neues Schwesterskript
`docs/perf/measure-library.sh`. Lighthouse (12.8.2, Testfilme vom NAS mit TMDB-Bildern, angemeldet):
mit Bühne **1.502 s** mobil (M11: 1.652) und 0.441 s Desktop, ohne Bühne **1.577 s** mobil (M11:
2.328) und 0.362 s Desktop, Note 100; LCP-Element `#hero-prepaint > img` bzw.
`#poster-prepaint > img`. K-3 am laufenden Server mit `curl` (Kinderprofil Stufe 6 nach Massen-
Freigabe): nur «Filme» mit `itemCount 3` in der Seitenleiste, `maxRating=18` ändert nichts,
`facets.ratings [0]`, unbewertete Serienbibliothek und unbekannte Id 404, `letter` ohne
Titel-Sortierung 422 `letter_requires_title_sort`, `sort=author` bei Video 422.

**Zwei Befunde der Messung.**

1. **LCP ohne Bühne auf dem Handy (behoben).** Der erste Mobile-Lauf ohne Bühne lag bei 3.5 s: das
   Vormal-Poster und das Kartenbild waren gleich gross, Chrome nahm durch Subpixel-Rundung die um
   7 px² grössere Karte als LCP-Kandidaten, der LCP sprang auf das React-Malen. Das Vormal-Poster
   ist jetzt 0.5 px breiter als die Karte (`global.css`, Kommentar dort; der Geometrie-Test erlaubt
   unter 1 px). Danach 1.577 s.
2. **`itemCount` über der Ausstiegsklausel (offen, Entscheid für Marco).** `GET /libraries/visible`
   braucht 50 ms p95 statt der 20 ms aus Papier-Abschnitt 8. Die vorgeschlagene «Zähltabelle je
   Bibliothek, die der Scanner pflegt» kann die Zahl nicht liefern: `itemCount` ist je Profil
   verschieden (Freigabestufe, Bibliotheksrechte), der Scanner kennt kein Profil; eine Zähltabelle je
   Profil wäre ein zweiter Zustand neben `visible_items` (Regel 2). Der Aufruf wird mit der Sitzung
   vorgeladen, läuft parallel zu `/home`, hält die Seitenleiste nicht auf und wird 60 s gehalten.
   Vorschlag: so lassen; Alternative: die Zahl aus der Seitenleiste nehmen (O-1 zurücknehmen).
   Details in `docs/perf/startseite.md` 1b.

**Belege (DoD) des Abschlusses:** `task web:gen:check && task web:lint && task web:test` grün;
`task ci:server` `0 issues.`, alle Pakete `ok` (mit `startseite.sql`); ganze Playwright-Suite
(Chromium und Firefox) `475 passed, 3 failed (25.6m)`: zwei davon waren die Toleranz von
`home.spec.ts` «no stage» (< 0.5 px) gegen den gewollten halben Pixel des Vormal-Posters – jetzt
< 1 px wie im Geometrie-Test, danach `home.spec.ts` und `sidenav.spec.ts` in beiden Browsern
`42 passed`; der dritte ist das bekannte 5-s-Zeitbudget von `admin-streams.spec.ts` in Firefox
unter Suite-Last.

**Eingefrorener Stand** `testdata/db/startseite.sql` (Erzeugung in `testdata/db/README.md`) mit
`TestFrozenStartseiteStateCoversTheConcept` (Takt 7, Sortierschlüssel ohne Artikel, Wertung mit
Herkunft, Indizes, Notiz); `TestFrozenStatesMigrate` nimmt ihn über das Glob mit.

**Versionsstellen auf 1.4.0:** `deploy/.env.example`, `deploy/compose.yaml`, `deploy/README.md`,
`docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`.

**Release-Notes-Entwurf 1.4.0** (C-5, nicht brechend):

- Zuschauer: Seitenleiste links ab 768 px («Meine Medien» mit jeder sichtbaren Bibliothek und
  «Cat's Empfehlungen», ausblendbar), Schublade auf dem Handy; Bibliotheksseite je Bibliothek mit
  Chips «Alle · Filme · Serien», acht Sortierungen, Filtern (Genre, Status, Jahr, Freigabe) und
  Buchstabenleiste; Seite «Cat's Empfehlungen»; Spotlight-Band mit zufälligem Einstieg je Aufruf,
  Takt aus der Verwaltung, Pause-Knopf und Posterreihe in Normalgrösse; alle Reihen rasten links
  ein, auf dem Handy drei Cover je Reihe.
- Vertrag 1.4.0, additiv: `listVisibleLibraries`, `listLibraryItems`, `listPicks` (alle `scoped`),
  `getHome?start`, `HomePage.heroStart` und `heroIntervalSeconds`, `SpotlightConfig.rotationSeconds`
  (im Update optional, fehlt = unverändert), Schemata `LibraryNav`, `LibrarySort`,
  `LibraryItemStatus`, `LibraryFacets`, `LibraryItemsPage`, `PicksPage`; Fehlercode
  `letter_requires_title_sort`.
- Migration 00026 (`rotation_seconds`, `item_texts.sort_key` mit Backfill, `items.vote_average`,
  Indizes); der Server sichert die Datenbank vorher nach `/config/backups/`. **Nach dem Update läuft
  einmal der Job `metadata.vote_backfill`** (Wertungen der vorhandenen Titel von TMDB und AniList, in
  Batches zu 200, gedrosselt; bei 2'500 Filmen und 1'000 Serien rund zwei Stunden, zwei TMDB-Anfragen je
  Titel für beide Sprachen); bis dahin sortiert «Bewertung» unvollständig. TVDB liefert keine
  Wertung.
- `SLOUCHCAT_HERO_START` (nur für Tests) fixiert den Einstieg des Bands.

**Testanleitung für Marco am Handy und am Tablet** (CT 190 oder der Stack auf 8096 nach dem Merge):

1. Startseite öffnen: Menü-Symbol oben links öffnet die Schublade mit Start, Suche, Merkliste,
   Downloads, «Meine Medien» (jede Bibliothek mit Zahl, Cat's Empfehlungen), Verwaltung, Profile,
   Abmelden; Tippen neben die Schublade schliesst sie. Am Tablet (quer und hochkant) steht die
   Leiste fest links; «Seitenleiste ausblenden» unten, danach Menü-Symbol und in der Schublade
   «Seitenleiste einblenden»; der Zustand bleibt nach dem Neuladen.
2. Band: bei jedem Öffnen ein anderer Titel zuerst, alle 5 s der nächste (Takt in Verwaltung →
   Spotlight → «Takt der Bühne» auf 3 bis 15 s stellen); Tippen auf ein Poster unter dem Band
   stellt den Titel ins Band, öffnet nichts; Pause-Knopf oben rechts hält an; die Poster bleiben
   stehen, die Markierung wandert, die Reihe rückt erst nach, wenn das aktive Poster aus dem Bild
   wäre; Band bleibt stehen, wenn du mit Pfeiltasten (Tastatur am Tablet) hineingehst.
3. Reihen: auf dem Handy drei ganze Cover je Reihe, nach dem Wischen steht links immer ein ganzes
   Cover; «Alle anzeigen ›» an Cat's Empfehlungen und Merkliste.
4. Bibliothek («Filme», «Dokus», …): Zähler oben, bei gemischten Bibliotheken Chips «Alle · Filme ·
   Serien», Sortieren (Titel, Zuletzt hinzugefügt, Erscheinungsjahr, Bewertung, Laufzeit,
   Altersfreigabe, Zuletzt gesehen; bei Hörbüchern Autor), Richtung, Filter (Genre, Status, Jahr,
   Freigabe – nur was es in der Bibliothek gibt), Buchstabenleiste als Lauf über dem Raster
   (nur bei Sortierung nach Titel); Haken auf gesehenen Titeln, FSK-Chip bei Video; «Mehr laden»
   ab 50; Zurück landet auf der Karte, von der du kamst. Ein Kinderprofil sieht in jeder
   Bibliothek nur seine Stufe, eine leere Bibliothek fehlt in der Leiste.
5. Cat's Empfehlungen: Seite mit allen Empfehlungen und Notizen in Cats Reihenfolge.
6. Verwaltung → Spotlight: «Takt der Bühne» speichern; Startseite neu laden.

**Umstellung auf CT 190:** am 30. Oktober 2026, 07:18 Uhr, Image
`ghcr.io/slouchcat/slouchcat:startseite` (212 MB, Version `startseite-3d37305`), auf dem CT aus der
Laufzeitstufe des Dockerfiles gebaut (Binary hier mit `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`, Web
eingebettet; auf dem CT fehlt BuildKit, darum klassischer Builder mit `--build-arg TARGETARCH=amd64`,
sonst findet apt `jellyfin-ffmpeg7` nicht). `.env` gesichert nach `.env.bak-vor-startseite` (vorher
`bibliotheken`); Sicherung der Datenbank `pre-migration-startseite-3d37305-20261030T061811.543Z.db`;
Migration 00026 in 1 927 ms; `/healthz` 200; `metadata.vote_backfill` lief beim Start einmal durch
(47 s). Die Abnahme nach der Testanleitung oben macht Marco.

**Offen nach diesem Zweig:** Storybook-Vergleichsbilder auf CT 190 nachholen (`task
storybook:update`: `AppHeader`, `SideNav`, `SideNavDrawer`, `PosterCard`, `Row`, `HeroCarousel`,
`SelectField`, `LetterBar`, Admin-Seiten); Messung der Bibliotheksseite auf CT 190 (2 vCPU: die
Folgen-Aggregation könnte dort über 300 ms liegen, `docs/perf/startseite.md` 1b); Entscheid zu
`itemCount` (oben); Pfeil links aus einem `<select>` in der Werkzeugzeile erreicht die Leiste
nicht.

## Bibliotheken: Familie, Inhaltsart und Metadatenquellen je Ordner (29. Oktober 2026, abgenommen)

Auf dem Zweig `claude/bibliotheken` (Pull Request #45, bereit) nach dem Konzept
`docs/planning/07-bibliotheken.md` (Fassung 4), den Entscheiden in **Frage 13** des
Entscheidungsprotokolls und **ADR 0015** (Teilnachfolger von ADR 0006). Ein Commit je Planschritt;
vor jedem Abschluss war `task ci` grün.

**Was sich ändert.** Eine Bibliothek ist nur noch eine **Familie** (`video`, `audiobooks`,
`podcasts`); die alten Arten `movies` und `series` sind verbrannt. Jeder Ordner (`library_roots`)
trägt eine **Inhaltsart** (`movie`, `series`, `audiobook`, `podcast`) und eine **geordnete Liste
von Metadatenquellen** (`tmdb`, `tvdb`, `anilist`, `local`): der erste Eintrag erkennt den Titel
und bestimmt bei Serien die Staffelzählung, jeder weitere füllt auf. Eine Video-Bibliothek darf
Film- und Serienordner zugleich tragen («Dokus», «Kids», «Anime»); eine Bibliothek nimmt mehrere
Ordner in einem Zug an, der Server legt alle oder keinen an (F-13).

### Schritt 0 — Entscheide und Papiere (`01b9b70`)

Frage 13 mit F-1 bis F-13 im Entscheidungsprotokoll (F-10 durch den Merge von PR #44 geschlossen),
ADR 0015, Ablösungshinweise in ADR 0006 und 0007, Idee 01 in Teil A (hierher) und Teil B (bleibt)
geteilt, `docs/ideas/README.md` und der Kopf von Idee 02 nachgeführt, das Papier auf `main 9923807`
verankert. `git diff --stat docs/spec/` ist leer. Der Kommentar zu Issue #8 liegt in den
Sitzungsnotizen zum Posten durch Marco bereit.

### Schritte 1 und 2 — Schema, Scanner je Wurzel, Vertrag 1.3.0 (ein Commit)

Die zwei Schritte sind **ein Commit**, wie Abschnitt 10 des Papiers es für den Fall vorsieht, dass
`task ci` dazwischen nicht grün sein kann: der Vertrag ohne das Schema liesse die Web-App nicht
kompilieren, das Schema ohne den Vertrag bräuchte eine Übersetzungsschicht für genau einen Commit.

**Migration 00023** (`server/migrations/00023_library_roots_providers.sql`): `library_roots` bekommt
`content` und `providers` (JSON-Liste), `items` bekommt `providers` (kopiert aus der Wurzel),
`anilist_id` und `feed_origin`, die Tabelle `item_field_sources` entsteht leer, `libraries` wird mit
`kind IN ('video', 'audiobooks', 'podcasts')` neu gebaut (`id` bleibt). Bestehende Wurzeln und Titel
bekommen die heutige Reihenfolge: Filme `["tmdb"]`, Serien `["tmdb","tvdb"]`, Hörbücher und
Podcasts `["local"]`; ein Feed-Abo trägt `feed_origin = 'admin'`. Zwei Wächter mit `CHECK`-Tabelle
brechen den Lauf vor `COMMIT` ab, wenn ein Paar nicht passt; der `CASE` ohne `ELSE` ist der erste
Wächter davor.

**Scanner je Wurzel.** `walkRoot`, `parsePath`, `scannedExt`, `wanted`, `videoFile`, `listDirFiles`,
`describe` und der Batch-Writer lesen den Inhalt der Wurzel statt der Bibliotheksart; der Titelindex
ist **einer je Titelart** (`libraryIndex`), damit ein Film und eine Serie mit derselben `tmdb_id` in
einer Bibliothek zwei Titel bleiben (Konzept 12.1). Die Batchgrösse schrumpft, sobald eine Wurzel
Serien hält; `QueuePendingSegments` läuft, sobald eine Wurzel Serien hält. Podcast-Ordner werden
angenommen, aber erst in Schritt 3 gelesen.

**`library.Service`:** `Create` legt Bibliothek und alle Ordner in einer Transaktion an;
`AddRoots` nimmt eine Liste (Überlappung auch innerhalb der Liste, ein Scan am Ende);
`UpdateRoot` mit den drei Wirkungen aus 6.3 (Liste ändern: `UpdateItemsProvidersByRoot` und ein
`metadata.match` je Titel; Erkenner wechseln: dasselbe; Inhalt wechseln: Löschweg aus Idea 02 ohne
Wurzelzeile, `confirmPath`, Scan-Sperre, Wurzelzustand des Scanners leeren, Watcher bleibt, Scan
neu). Die Regeln aus Abschnitt 2 stehen in `model.ValidRootProviders`, `ContentAllowed`; jede
Verletzung ist ein Feldfehler mit Index (`roots[1].path root_overlap`, `roots[0].content
content_not_allowed`, `roots[0].providers providers_not_allowed`, `roots roots_required`).

**Metadaten:** `match` wählt den Erkenner nach dem ersten Listeneintrag; bei `tvdb` zuerst erkennt
TVDB, die Staffeln folgen der TVDB-Zählung und die `tmdb_id` wird über TMDBs `/find` zusätzlich
geholt; `seriesProvider` folgt der Liste. `searchMetadata` und `assignMatch` nehmen weiter nur
`tmdb` und `tvdb` (`provider_not_searchable`). Das Auffüllen (`metadata.fill`) kommt in Schritt 4.

**Vertrag 1.3.0, brechend** (gehört wörtlich in die Release-Notes, C-5):

- `LibraryKind` verliert `movies` und `series` und bekommt `video`; entfernte Enum-Werte werden nie
  wiederverwendet.
- `LibraryRequest.path` entfällt; neu `LibraryRequest.roots[]` (`LibraryRootRequest` mit den
  Pflichtfeldern `path`, `content`, `providers`), Pflicht bei `video` und `audiobooks`.
- `addLibraryRoot` nimmt `LibraryRootsRequest` (`roots[]`) statt eines Ordners.
- `LibraryRoot` trägt `content` und `providers`; `MetadataProvider` wächst um `anilist` und `local`;
  `DuplicateGroup.reason` bekommt `file_and_feed`; `AdminItem` bekommt `providers`, `fieldSources`,
  `anilistId?`; `AdminPodcast` bekommt `feedOrigin?`, `rootId?`, `localEpisodeCount`, `feedUrl` wird
  optional.
- Neu: `updateLibraryRoot` (`PATCH /admin/libraries/{id}/roots/{rootId}`), `listMetadataProviders`
  (`GET /admin/metadata/providers`), die Schemata `RootContent`, `LibraryRootsRequest`,
  `LibraryRootUpdate`, `MetadataProviderInfo`, `FieldSource`.
- Ein ungültiger Ordner ist bei `createLibrary`, `runSetup` und `addLibraryRoot` kein Problemcode
  `library_path_invalid` mehr, sondern ein Feldfehler `roots[i].path` (`library_path_invalid`,
  `root_overlap`) in `validation_failed`.

**Web-Minimum** (die Oberfläche kommt in Schritt 5): drei Arten «Video», «Hörbücher», «Podcasts»;
die Formulare senden feste Vorgabelisten (`movie/["tmdb"]`, `audiobook/["local"]`,
`podcast/["local"]`); Fehler an `roots[0].path` landen am Ordnerfeld; Fixtures und e2e-Specs
folgen.

**Belege (DoD Schritt 1 und 2):**

| Kriterium | Beleg |
|---|---|
| `task ci` grün | `task ci` (29. Oktober 2026): `ci:server` alle Pakete `ok` (Go mit `-race`, `golangci-lint` 0 Befunde, `gen:check` ohne Drift), `ci:web` Vitest 632 Tests in 77 Dateien, api-client 4, design-tokens 2, Storybook-Vergleichsbilder über `sg docker`, `task build`, `task ci:e2e` **442 passed (23.7m)**, A-9-Logscan 0 Befunde |
| Eingefrorene Stände migrieren, `foreign_key_check` leer | `go test ./internal/store/ -run 'TestFrozenStatesMigrate\|TestMigration00023' -count=1`: `ok` (13 Stände, M1a bis `benutzerverwaltung`) |
| M6-Stand nach 00023: `video, video, audiobooks, podcasts`; Wurzeln `movie/["tmdb"]`, `series/["tmdb","tvdb"]`, `audiobook/["local"]`; Hörbuchtitel `["local"]`; Feed-Podcast `["local"]` und `feed_origin = 'admin'`; `item_field_sources` leer | `TestMigration00023OverM6` (`server/internal/store/frozen_libraries_test.go`) |
| Kein View, kein Trigger liest `libraries` (4.6 Punkt 4) | dieselbe Prüfung auf dem M6-Stand vor der Migration: `SELECT count(*) FROM sqlite_master WHERE type IN ('view','trigger') AND (sql LIKE '%FROM libraries%' …)` = 0 |
| Wächter bricht einen präparierten Stand ab | `TestMigration00023GuardAbortsAWrongPair`: Wurzel in die Podcast-Bibliothek verschoben → `partial migration error (type:sql,version:23): … NOT NULL constraint failed: library_roots.content`, die alten Arten stehen danach noch |
| Gemischte Bibliothek: Film und Serie mit derselben `tmdb_id` ergeben zwei Titel | `TestMixedLibraryKeepsFilmAndSeriesWithTheSameTMDBIDApart` (`server/internal/library/mixed_test.go`) |
| Serienordner verwirft Dateien ohne Folgenmarke, Filmordner derselben Bibliothek nicht | `TestSeriesRootRejectsFilesWithoutMarkerTheMovieRootDoesNot` |
| Inhaltswechsel `movie → series`: kein Titel ohne Datei, Tombstone des Downloads, `ignored_files` leer, `pathPending` und `orphans` leer, Watcher läuft, Scan legt Folgen neu an; `confirm_mismatch`, `scan_running` | `TestUpdateRootContentChangeRereadsTheFolder` |
| Listenänderung: ein Job je Titel, nichts gelöscht, kein Scan | `TestUpdateRootProvidersQueuesOneJobPerTitleAndDeletesNothing` |
| Regeln: `content_not_allowed`, `providers_not_allowed` (AniList allein, TVDB am Filmordner, Doppelte, leere Liste), `roots_required`, `root_overlap` innerhalb der Anfrage; `["tvdb","tmdb","anilist"]` am Serienordner 200; Podcasts ohne Ordner 201; alles oder nichts | `TestCreateAppliesTheFolderRules`, `TestAddRootsIsAllOrNothing`, `TestPodcastLibraryTakesPodcastFolders` |
| Serie mit `["tvdb","tmdb"]` über TVDB erkannt, `tvdb_id` und `tmdb_id` (aus `/find`), Staffeln nach TVDB; `["tmdb","tvdb"]` wie heute; TVDB zuerst ohne Schlüssel `skipped` | `TestMatchSeriesTVDBFirst`, `TestMatchSeriesDefaultListIsTMDBFirst`, `TestMatchSeriesTVDBFirstWithoutKeyIsSkipped` (`server/internal/metadata/providers_test.go`) |
| Vertrag über HTTP: zwei Ordner in einem `createLibrary`, `roots: []` bei `video` 422 `roots_required`, bei `podcasts` 201; `updateLibraryRoot` 200 beim Listenwechsel, `confirm_mismatch` und 409 `scan_running` beim Inhaltswechsel; `listMetadataProviders` mit TVDB `configured: false` | `TestCreateLibraryWithSeveralFolders`, `TestUpdateLibraryRootEndpoint`, `TestListMetadataProvidersEndpoint` (`server/internal/api/library_roots_handlers_test.go`) |
| Schema-Test findet `x-visibility`, Vertragstest deckt jede Operation | `TestContractCoversEveryOperation` mit den Einträgen `updateLibraryRoot`, `listMetadataProviders` |
| `searchMetadata` mit `anilist` → 422 `provider_not_searchable` | `TestSearch`, `TestAssignMatchAndRefresh` (`manage_test.go`) |
| `scanbench` ohne Regression gegen `docs/perf/M3.md` | `docs/perf/bibliotheken.md` Abschnitt 1: `main` und neue Fassung abwechselnd auf demselben Baum, Serien 40.7/41.7 s gegen 42.7/42.2 s, Erst-Lauf 64.1/65.1 s gegen 66.3/65.2 s, 713 Transaktionen in allen Läufen, 0 Wiederholungen; innerhalb der Streuung |

### Schritt 3 — Podcast-Ordner (`feat(library,metadata,playback): scan podcast folders and adopt their feeds`)

**Was der Scanner tut** (`server/internal/library/podcast_folder.go`, Konzept 4.5): in einem Ordner
mit `content: podcast` ist jeder Unterordner der ersten Ebene ein Podcast, jede Audiodatei darin
eine Episode mit `media_files`-Zeile (Identität `root_id + rel_path`, keine GUID); Audiodateien
direkt in der Wurzel werden übergangen. `metadata.json` (Audiobookshelf-Schema) füllt Titel,
Autor, Beschreibung, Genres, `explicit` (→ 18, `rating_source 'tags'`) und `feed_url`; der Ordner
heisst sonst wie der Titel; `cover.jpg` wird `cover_path` mit `image.fetch`. **Feed übernehmen
(F-6):** steht in der Bibliothek schon ein Podcast mit derselben Adresse (Abo von Hand oder zweiter
Ordner), hängen sich die Episoden an ihn, der Ordner bekommt keinen eigenen Podcast; ein zweiter
Ordner mit demselben Feed schreibt `feed_error = two_folders_one_feed` (sichtbar auf der Seite
Podcasts, bis der nächste Abruf es löscht) und eine Warn-Zeile. Gibt es ihn nicht, entsteht der
Podcast mit `feed_origin = 'file'`. Eine `feedURL` mit lokalem Host oder ohne `http`/`https` wird
**nicht** gespeichert; der Podcast trägt `feed_error = feed_url_not_allowed: <Schema://Host>`
(Regel 10: kein Pfad). Der Erst-Scan eines Ordners feuert keinen Neue-Folgen-Hook; ein späterer
Scan mit neuer Datei an einem bestehenden Podcast schon.

**Nach der Sonde** (`finishPodcastEpisode` in `probe.go`): Titel aus dem Tag, ausser er ist gleich
dem Podcast-Titel (dann bleibt der Dateiname); Datum aus `date`/`releasedate` (RFC 2822); Beschreibung
aus `comment`, sonst `subtitle`; Dauer aus der Sonde. Dann das **Zusammenführen** mit einer
Feed-Episode desselben Podcasts ohne Datei: gleiche Minute, sonst gleicher normalisierter Titel,
Dauer ±2 %, **nur bei genau einem Treffer** — die Feed-Episode bekommt die Datei und behält GUID,
Enclosure und Feed-Text, die Datei-Episode wird weich gelöscht. Umgekehrt nimmt der Feed-Abruf
(`metadata/podcast.go`, `episodeWrite.adopt`) eine neue Feed-Episode auf einer passenden
Datei-Episode auf statt sie doppelt anzulegen. Audio-Dateien reihen **kein** `probe.keyframes`
mehr ein (auch Hörbuchteile nicht).

**Netzwächter (9.1):** `netguard.Clients.For(ctx, feedURL, origin)` gibt für `feed_origin = 'file'`
immer den geschützten Client, in allen vier Abrufen (Feed, Enclosure-Proxy, Download, Cover);
`GetVisibleEnclosure` und `GetDownloadItem` tragen `feed_origin`. **Wiedergabe:** eine Episode mit
Datei spielt als Datei (`direct` oder Audio-HLS), der Enclosure-Proxy ist nur noch der Rückfall;
Offline bevorzugte die Datei schon. **Aufräumen:** `SoftDeleteItemsMissingSince` und
`SoftDeleteItemsOfRootWithoutOtherFiles` nehmen `podcast_episode` ohne Enclosure mit; der Sweep und
`removeLibraryRoot` löschen Ordner-Podcasts ohne Feed und ohne lebende Episode weich; ein Podcast
**mit** Feed bleibt beim Entfernen des Ordners als Abo (seit dem Review mit `feed_origin = 'file'`,
siehe unten). `PodcastListDue`
und `PodcastListIDsByLibrary` fragen nur Podcasts mit `feed_url`. `scanLibrary` einer
Podcast-Bibliothek ruft die Feeds ab **und** scannt die Ordner. `listDuplicates` zeigt nicht
zusammengeführte Paare als `file_and_feed`.

**Migration 00024** (`00024_podcast_episode_availability.sql`): der Trigger
`items_enclosure_update` aus 00003 setzte die Verfügbarkeit einer Episode allein aus der Enclosure;
eine Datei-Episode ohne Enclosure wäre damit unsichtbar geworden. Der Trigger wird ersetzt
(Enclosure **oder** vorhandene Datei), Trigger werden nie bearbeitet (Regel 5).

**Testdaten:** `testdata/generate-media.sh` legt `testdata/media/PodcastFolder/` an (ein Podcast
mit `metadata.json`, `cover.jpg`, zwei getaggten Episoden — eine davon mit dem Podcast-Titel als
Tag-Titel — und einer losen Datei in der Wurzel) und taggt die drei Episoden des Mock-Feeds mit
`date` (RFC 2822, gleich dem `pubDate` der Vorlage), `album` und `comment`, wie Audiobookshelf es
beim Herunterladen tut.

**Belege (DoD Schritt 3):**

| Kriterium | Beleg |
|---|---|
| Ordner → Podcast mit `feed_url` und `feed_origin = 'file'`; zweiter Ordner mit demselben Feed legt keinen zweiten Podcast an und scheitert nicht; `feedURL` auf lokalem Host wird nicht gespeichert und ergibt die Pflegezeile; `explicit: true` ergibt 18 mit Quelle `tags`; Ordner-Podcast ohne Feed nicht in der Abruf-Liste | `TestPodcastFolderAdoptsAndRefusesFeeds` (`server/internal/library/podcast_folder_test.go`) |
| Ordner-Scan: Podcast und Episoden aus `metadata.json` und Dateien, lose Datei übergangen, Titel/Datum/Beschreibung aus den Tags nach der Sonde, Tag-Titel gleich Podcast-Titel wird nicht Episodentitel, kein `probe.keyframes` für Audio, zweiter Scan schreibt nichts | `TestPodcastFolderScanReadsPodcastsAndEpisodes`; `TestScanAudiobookFixtures` (keine Keyframe-Jobs für Hörbuchteile) |
| Sonde hängt die bekannte Feed-Episode an die Datei (Minute gleich, eindeutig); zwei Dateien mit derselben Minute werden nicht zusammengeführt | `TestPodcastFolderPairsFileWithFeedEpisodeAfterProbe` |
| Feed-Abruf nimmt eine neue Feed-Episode auf der passenden Datei-Episode auf; die unbekannte Feed-Episode entsteht als Enclosure-Episode; zwei Kandidaten bleiben getrennt | `TestFetchAdoptsAFileEpisode` (`server/internal/metadata/podcast_folder_test.go`) |
| Feed aus Datei nur über den geschützten Client (Loopback-Feed von Hand abrufbar, aus Datei verweigert) | `TestFetchOfAFeedFromAFileIsGuarded`; `TestClientsFor` in `netguard_test.go` (`For` mit `FeedOriginFile`) |
| Datei-Episode spielt `direct` aus der Datei; explizite Datei-Episode für P12 nicht gefunden | `TestPodcastEpisodeWithFilePlaysFromTheFile` (`server/internal/playback/file_episode_test.go`) |
| Gelöschte Datei-Episode ohne Enclosure wird vom Sweep weich gelöscht, eine mit Enclosure bleibt; Ordner-Podcast ohne Feed geht mit der letzten Episode | `TestSweepTakesFileEpisodesWithoutEnclosure` |
| `removeLibraryRoot` lässt den Feed-Podcast als Abo stehen (seit dem Review mit `feed_origin = 'file'`), der ohne Feed geht mit seinen Episoden | `TestPodcastFolderRemovalKeepsFeedPodcastsAsSubscriptions` |
| Kinderprofil-Vertragstest: explizite Datei-Episode (`QF`, Datei ohne Enclosure, `rating_source tags`) fehlt in `listPodcastEpisodes` unter 18 und spielt für P18 aus der Datei | `vistest.Audio.FileOnlyExplicit` in `TestContractAudio`, `TestContractChildProfiles` |
| `listDuplicates` zeigt nicht zusammengeführte Paare als `file_and_feed` | `TestAdminMaintenanceFileAndFeedDuplicates` |
| Migration 00024 über alle eingefrorenen Stände | `TestFrozenStatesMigrate` |
| `task ci` grün | `task ci` (29. Oktober 2026): `ci:server` alle Pakete `ok` (Go mit `-race`, `golangci-lint` 0 Befunde, `itemsaccess`, `gen:check` ohne Drift), `ci:web` Vitest 632 Tests in 77 Dateien, api-client 4, design-tokens 2, Storybook-Build ok, `task build`, `task ci:e2e` **442 passed (23.4m)** auf `SLOUCHCAT_E2E_PORT=8197` (8097 hielt ein verwaister e2e-Server der abgebrochenen Session), A-9-Logscan 66 902 Zeilen, 0 Befunde. **Nicht gelaufen:** der Storybook-Screenshot-Vergleich (`storybook:test`), weil Docker in dieser Session nicht erreichbar war (`sg docker` scheitert an `NoNewPrivs`); Schritt 3 ändert keine Datei unter `web/src`, die Vergleichsbilder sind davon nicht berührt |

**Korrektur nach dem Review** (`af313cc fix(metadata,library)`, gefunden beim Einfrieren des Standes): der Scanner
prüfte die `feedURL` aus der Datei **vor** dem Nachschlagen und verweigerte darum auch die
Übernahme eines **von Hand** abonnierten Feeds auf einem lokalen Host (der Mock-Feed der Tests, ein
Feed im LAN). Konzept 4.5 sagt: zuerst `PodcastFindByFeed`, erst für einen **neuen** Podcast die
Prüfung nach 9.1. Jetzt wird ein bestehendes Abo auch mit lokaler Adresse übernommen (es bleibt
`feed_origin 'admin'`, der Netzwächter gilt wie für jedes Abo von Hand); nur ein neuer Podcast aus
einer lokalen Adresse wird weiter verweigert. `TestPodcastFolderAdoptsAndRefusesFeeds` prüft beide
Fälle («Lokal Abo» übernommen, «Lokal» verweigert).

**Nicht in Schritt 3:** die Messung des Erst-Scans von `/media/nas/Audiobookshelf/Podcast` auf CT 190
(26 031 Dateien) und die Quote der nicht zusammengeführten Episoden am echten Ordner kommen mit der
Abnahme in Schritt 6 nach `docs/perf/bibliotheken.md`; die Seite Podcasts zeigt den Chip «aus Ordner»
erst in Schritt 5.

### Schritt 4 — Auffüllen in Reihenfolge und AniList (`feat(metadata): fill fields provider by provider, with provenance and AniList`)

**Vorher die Prüfpunkte aus 4.4** (Beleg im Papier, Abschnitt 4.4 «Geprüft am 29. Oktober 2026»,
und in Frage 13): Rohadresse stimmt (7.5 MB, `ETag`), Feldnamen erhoben (`themoviedb_id` als
Objekt, `imdb_id` als Liste, `season` je Anbieter), **keine Lizenzdatei** im Repository, nur 15 %
der Filme mit TMDB-Film-Id. Marcos Antworten **F-14** (nur zur Laufzeit abrufen, nie ins
Repository) und **F-15** (Filme auch über die IMDb-Id) stehen in Abschnitt 11, in Frage 13 und in
ADR 0015 (`d9fff9d`).

**Der Job `metadata.fill`** (`server/internal/metadata/fill.go`, Schlüssel `fill:<item>`, Priorität
wie `image.fetch`) läuft die Liste eines gematchten Films oder einer Serie von vorne nach hinten,
Erkenner eingeschlossen, holt je Anbieter einmal den Datensatz (TMDB, TVDB; AniList über die
Zuordnungsliste) und entscheidet je Feld und Sprache nach 4.4: gesperrte Felder nie; sonst
gewinnt der erste Anbieter mit Wert, `item_field_sources` hält fest, wer; steht dort ein Anbieter
weiter vorne, bleibt der Wert, einer weiter hinten (oder nicht mehr in der Liste) wird ersetzt.
Felder: `title`, `overview`, `tagline` je Sprache, `original_title`, `release_date` (mit Jahr),
`runtime_minutes`, `poster`, `backdrop`; `genres` ist die Vereinigung aller Anbieter mit Herkunft
`union`; `own_rating` wird **nur erhöht** (K-3), `rating_source` nennt den strengsten; bei gleicher
Stufe nennt sie den vordersten Anbieter, damit ein Lauf nach einem erneuten Match dieselbe Zeile
ergibt. Eine Liste mit **einem** Eintrag macht keine Anfrage und schreibt nur die Herkunft des
Erkenners für alle Felder, die der Titel trägt. Für Staffeln und Folgen läuft derselbe Entscheid
je Staffel, nur wenn zwei Anbieter sie beschreiben (TMDB und TVDB), eine Transaktion je Staffel;
Mehrfolgen-Dateien behalten den zusammengesetzten Titel des Matches; AniList füllt nur die
Titelebene. Ein Anbieter ohne Schlüssel wird übersprungen und als `WARN` «metadata provider in the
list of a title but not configured, skipped» gemeldet. Geändertes Poster oder Backdrop reiht
`image.fetch` ein; eine erhöhte Freigabe löst den Sichtbarkeits-Recheck der offenen Streams aus.

**Einreihen und Konvergenz:** `applyTitle` reiht den Job **in der Transaktion des Matches** ein
(`enqueueFillTx`). Findet der Job das Match seiner Serie noch aktiv (`match:<id>` in der Queue),
endet er mit `errFillWaitsForMatch`, die Queue wiederholt ihn mit Backoff — sonst überschrieben
die Staffel-Transaktionen des Matches, was der Job entschied. `QueuePendingMetadata` reiht
zusätzlich jeden gematchten Titel mit mehr als einem Eintrag ein, dessen Herkunft fehlt oder
einen Anbieter nennt, der nicht mehr in der Liste steht (`ListFillItems`; «mehr als ein Eintrag»
ist `instr(providers, ',') > 0`, weil sqlc `json_array_length` nicht kennt). `updateLibraryRoot`
reiht bei einer Listenänderung mit gleichem Erkenner je Titel `metadata.fill` ein, bei einem
Erkennerwechsel wie bisher `metadata.match` (dessen Fill folgt).

**AniList** (`anilist.go`): GraphQL `Media(id, type: ANIME)` mit `title {romaji english native}`,
`description`, `genres`, `coverImage.extraLarge`, `bannerImage`, `isAdult`, ohne Schlüssel,
`ratelimit.Limiter` 1/s Burst 5 (429 und `Retry-After` wie bei TMDB), 1 MB Lesegrenze, HTML der
Beschreibung entfernt. Liefert englischen Titel (sonst Romaji), `original_title` = `title.native`,
englische Beschreibung und Genres, Cover und Banner (nur vom Host `s4.anilist.co`; `image.fetch`
lädt sie über den Anbieter-Client wie TVDB-Artworks), `isAdult` → 18 mit `rating_source
'anilist'`. `SLOUCHCAT_ANILIST=off` (`config.AniList`, `metadata.WithAniList`,
`admin.WithAniList`) schaltet den Anbieter ab; `listMetadataProviders` meldet dann `configured:
false`, die Häkchen bleiben. **Zuordnungsliste** (`anime_lists.go`): fester Pfad bei
`raw.githubusercontent.com`, Cache unter `<cache>/anime-lists/` mit ETag, wöchentlich erneuert
(304 setzt die Dateizeit), 64-MB-Bremse beim Laden und beim Lesen, gestreamt in drei Karten:
TMDB-Film-Id → AniList, IMDb-Id → AniList (nur `MOVIE`, F-15), TVDB-Id → Einträge je Staffel
(Titelebene nimmt `season.tvdb = 1`, sonst die kleinste Staffel). `anilist_id = 0` heisst
«nachgeschlagen, kein Eintrag» und wird beim nächsten Lauf wieder gefragt. Bricht der Abruf bei
vorhandenem Cache ab, bleibt die alte Liste mit einer Warnung; Einträge anderer Form werden mit
einer Warnung übersprungen (die Datei führt `themoviedb_id.movie` als Liste); Worker, die nach
einem Scan zugleich fragen, teilen sich einen Ladevorgang (`loadMu`).

**Messbarkeit:** jede Anbieter-Anfrage steht bei `--log-level debug` als `metadata request` mit
`provider`, `host` und `path` im Log (ohne Query und Header, Regel 10); die Messung in
`docs/perf/bibliotheken.md` Abschnitt 2 zählt damit. `deploy/.env.example` und `deploy/README.md`
nennen `SLOUCHCAT_ANILIST`.

**Belege (DoD Schritt 4):**

| Kriterium | Beleg |
|---|---|
| Prüfpunkte aus 4.4 mit Beleg in der Rückmeldung | Papier 4.4 «Geprüft am 29. Oktober 2026», F-14 und F-15 (`d9fff9d`) |
| `["tmdb","tvdb"]` füllt eine leere TMDB-Beschreibung aus TVDB und lässt eine vorhandene stehen, Herkunft je Feld stimmt; Idempotenz (zweimal laufen, gleiches Ergebnis); Staffeln und Folgen folgen, Mehrfolgen-Titel bleibt; der Match reiht den Fill in seiner Transaktion ein | `TestFillSeriesTMDBThenTVDB` (`server/internal/metadata/fill_test.go`) |
| `["tvdb","tmdb"]` ersetzt eine TVDB-Beschreibung nicht durch TMDB; Wechsel auf `["tmdb","tvdb"]` ersetzt sie beim nächsten Lauf (zweimal gleich); nach einem erneuten Match steht das Auffüllen wieder (Match, Fill, Match, Fill → gleiche Felder) | `TestFillTVDBFirstKeepsTVDBTextsUntilTheListChanges` |
| Gesperrtes Feld bleibt; `isAdult` hebt auf 18 mit Quelle `anilist`; `own_rating` wird nie gesenkt (AniList später nicht mehr adult, TMDB 12 → 18 bleibt; `manual` unberührt); Genres als Vereinigung | `TestFillKeepsLockedFieldsAndNeverLowersTheRating` |
| Kinderprofil-Vertragstest: ein Film, dessen 18 vom Auffüller AniList hinter TMDB stammt, fehlt für P16 in jeder Antwort und steht für P18 in «Zuletzt» («Anbieterreihenfolge senkt die Freigabe nicht») | `vistest.Movie.FillerRated` in `AddEdgeCases`, Prüfung in `TestContractChildProfiles` (`home`) plus der Negativtest der Registry |
| TVDB-Id → AniList-Id über die Liste (Staffel 1 vor Staffel 2), TMDB-Film-Id ebenso, Film und Serie mit derselben Zahl 550 bei verschiedenen Einträgen, Film über IMDb-Id (F-15); kein Treffer schreibt `anilist_id = 0`; AniList-Cover wird Poster, `image.fetch` folgt und lädt vom AniList-Host; Liste einmal geladen, nach der Woche mit ETag erneuert (304) | `TestFillAniListThroughTheMappingList`, `TestParseAnimeListAndStripHTML` |
| Liste mit einem Eintrag: keine Anfrage, Herkunft des Erkenners für jedes Feld; `QueuePendingMetadata` lässt sie in Ruhe | `TestFillSingleProviderRecordsTheIdentifier` |
| Konvergenz: fehlende oder veraltete Herkunft wird nach dem Scan eingereiht, passende nicht, aktive Jobs behalten den Schlüssel | `TestQueuePendingMetadataQueuesStaleFills` |
| Anbieter ohne Schlüssel übersprungen und gemeldet, keine TVDB-Anfrage; Fill wartet auf das aktive Match | `TestFillSkipsAProviderWithoutKeyAndWaitsForTheMatch` |
| 429 mit `Retry-After` pausiert den Limiter (zweite Anfrage nach der Sekunde); Liste grösser als 64 MB bricht ab, keine AniList-Anfrage | `TestFillAniListRateLimitAndListSize` |
| Listenänderung am Ordner reiht `metadata.fill` je Titel ein, kein Match, kein Scan | `TestUpdateRootProvidersQueuesOneJobPerTitleAndDeletesNothing` (`library/mixed_test.go`) |
| `SLOUCHCAT_ANILIST` gelesen (`on` Vorgabe, `off`), Vorgabe im Config-Test | `TestDefaults`, `TestPrecedenceFlagsOverEnv` (`config_test.go`) |
| Review-Befunde (a) bis (h): Fill wartet in der Transaktion auf das Match; Konvergenz über `filled_providers` (nie gelaufen, andere Liste, AniList aus → an genau einmal, AniList aus → nichts); Auffüller nur bei gleichem Ausstrahlungstag | `TestFillSkipsAProviderWithoutKeyAndWaitsForTheMatch`, `TestQueuePendingMetadataQueuesStaleFills`, `TestSameAirDate`, `TestFillTVDBFirstKeepsTVDBTextsUntilTheListChanges` (Folge 2 bleibt leer, anderer Tag), `TestFrozenStatesMigrate` mit 00025 |
| Messung: 111 Anime-Titel vom NAS mit `[tmdb, anilist]` (82 Filme) und `[tmdb, tvdb, anilist]` (29 Serien, ohne TVDB-Schlüssel), Dauer und Anfragen je Host | `docs/perf/bibliotheken.md` Abschnitt 2: 330.6 s vom ersten Abruf bis zum letzten Fill (Matches nach 33 s, die Fills warteten hinter 1 961 Folgen-Matches), TMDB 2 477 Anfragen, AniList 91 (Spitze 34/min, kein 429), Liste 1× (7.5 MB, ETag), AniList-Treffer 62/82 Filme und 29/29 Serien, 0 fehlgeschlagene Jobs; der erste Lauf deckte `themoviedb_id.movie` als Liste auf (Parser korrigiert) |
| `task ci` grün | `task ci` (29. Oktober 2026, Stand vor Schritt 5): `task ci:server` **EXIT 0** (Go `-race` alle 32 Pakete `ok`, `golangci-lint` 0 Befunde, `itemsaccess`, `vet`, `gen:check` ohne Drift), der Web-Teil ist seit Schritt 3 unverändert (Vitest 632, api-client 4, design-tokens 2, Storybook-Build), `task build` und `task ci:e2e` auf dem Binary dieses Standes **442 passed (23.5m)** auf `SLOUCHCAT_E2E_PORT=8197`, A-9-Logscan 64 715 Zeilen, 0 Befunde; der Storybook-Screenshot-Vergleich lief auch hier nicht (kein Docker in dieser Session, keine Web-Änderung) |

**Bewusste Abweichungen vom Papier:** (1) Ein Auffüller beschreibt eine Staffel oder Folge nur,
wenn sie beim Erkenner unter derselben Nummer am selben Tag ausgestrahlt wurde (`sameAirDate`);
wo TMDB und TVDB verschieden zählen, bleibt die Lücke lieber leer, als dass die Folge eines
anderen Tages sie füllt (Review-Befund 5; 4.4: «Folgennummern kommen allein vom Erkenner»).
(2) Es gibt keinen eigenen Dashboard-Zähler für Fill-Läufe: das Dashboard zählt Jobs nur nach
Zustand (`JobCounts`), auch Trailer haben keinen; Fill-Jobs erscheinen dort wie jeder andere Job
und im Log als `item filled`. (3) Bei gleicher Freigabe aus zwei Anbietern nennt `rating_source`
den vordersten (nicht «den strengsten», die sind gleich), damit Match und Fill idempotent sind.
(4) Der Fill einer **Serie** wird nach den Staffel-Transaktionen des Matches eingereiht, nicht in
der Titel-Transaktion (Filme schon): sonst liefe er neben den Staffeln und verbrauchte wartend
seine Versuche; ein Absturz dazwischen wird von der Konvergenz aufgefangen.

**Nach dem Review (29. Oktober 2026, `fix(metadata,library)`):** (a) ein laufendes Match sperrt
den Fill auch **in** jeder Schreibtransaktion (`waitForMatch`), nicht nur beim Start; so kann der
Fill, dessen Schlüssel das Match-eigene Einreihen verschluckt hat, nach dem Match wiederholt
laufen. (b) Ein erneuter Match desselben Titels senkt eine Freigabe nicht mehr, die ein anderer
Anbieter gab (nur der gebende Anbieter revidiert sie; eine andere Zuordnung beginnt neu). (c) Eine
andere Zuordnung (neue `tmdb_id`/`tvdb_id` im Match, manuelle Zuordnung) löscht `item_field_sources`
und `anilist_id`, sonst holte der Fill den alten Anime. (d) `sameIdentity` vergleicht auch
`providers`: ein Erkennerwechsel während eines laufenden Matches lässt ihn mit `errItemChanged`
neu anlaufen. (e) **Migration 00025:** `items.filled_providers` trägt die Liste, mit der der
letzte Fill lief (nur die konfigurierten Anbieter); die Konvergenz (`ListFillItems` +
`QueuePendingFills`) reiht nur noch Titel ein, deren Liste sich gegenüber dem letzten Lauf
geändert hat — statt jeden Titel bei jedem Scan, dessen Herkunft einen abgewählten Anbieter nennt
(das Feld behält seine alte Herkunft absichtlich). `SLOUCHCAT_ANILIST=off → on` reiht so jeden
Titel genau einmal ein. (f) Gibt der Erkenner keinen Datensatz (kein Schlüssel, Id unbekannt),
schreibt der Fill nichts, statt die Auffüller über die Werte des Matches zu lassen. (g) Die
Zuordnungsliste: eine unlesbare Cache-Datei wird gelöscht und neu geholt; nach einem gescheiterten
Abruf ohne Cache gilt fünf Minuten Pause; ein neuer Körper ohne ETag entfernt den alten. (h) Die
AniList-Antwort über 1 MB ist ein klarer Fehler, kein «unreachable». (i) Beim Entfernen eines
Podcast-Ordners behält das zurückbleibende Abo `feed_origin = 'file'` (9.1: die Adresse kam aus
einer Datei; die LAN-Ausnahme gilt ihr nie), die Podcast-Seite erlaubt das Löschen, sobald kein
Ordner mehr dazu gehört (`rootId` fehlt). **Nicht umgesetzt** (als
Restpunkte für später notiert): der Server verweigert das Löschen eines Podcasts, den ein Ordner
noch besitzt, nicht selbst (die Oberfläche blendet den Knopf aus; ein gelöschter Ordner-Podcast
käme beim nächsten Scan wieder, Befund 13); eine Listenänderung am Ordner läuft nicht unter der
Scan-Sperre (ein gleichzeitig laufender Scan könnte Titel mit der alten Liste anlegen, Befund 15,
geringe Wahrscheinlichkeit); der Rückfall einer Datei-Episode auf ihre Enclosure greift nur bei
«Datei nicht gefunden», nicht bei anderen Dateifehlern (Befund 16).

### Schritt 5 — Oberfläche (`feat(web): library families, content and ordered metadata sources per folder`)

**Zwei Bauteile in `web/src/ui`** (je mit Story, Unit-Test, Tastaturtest; Texte nur aus den
Locales, Farben nur aus Tokens):

- **`ProviderList`** («Metadatenquellen», Konzept 7): je Anbieter, den `listMetadataProviders`
  für die Inhaltsart erlaubt, eine Zeile mit Häkchen, Name, Chip («erkennt Titel» am ersten
  angekreuzten, «füllt auf» an den weiteren, «kann nicht erkennen» wenn ein reiner Auffüller
  vorne steht), Hinweisen («kein Schlüssel hinterlegt», «liefert kein Deutsch», «nur Auffüller»)
  und «Nach oben» / «Nach unten»; Pfeiltasten auf einem fokussierten Knopf sortieren ebenfalls,
  der Fokus bleibt nach dem Tausch auf dem gedrückten Knopf (wie `HomeRowsPage`). Ankreuzen behält
  die gezeigte Reihenfolge, Abwählen nimmt den Eintrag aus der Liste. Der Client prüft keine
  Regel (Regel 12): `providerListError` sagt nur vorher, was der Server mit 422 beantworten würde
  (nichts angekreuzt, Auffüller vorne). Eine Inhaltsart mit einer Quelle (Hörbücher, Podcasts) und
  eine Liste ohne Serverantwort (Einrichtungsassistent, keine Sitzung) stehen als Text.
- **`FolderList`** («Ordnerliste», F-13): nummerierte Karten mit Akzentbalken, Kopfzeile
  (Nummer, Inhalt-Chip, Kurzform «TMDB › TVDB», «Entfernen»), Pfadfeld mit `pathAddon`
  (Durchsuchen-Knopf oder die Prüfung des Assistenten), «Inhalt» (nur bei Video) und
  `ProviderList`. «+ Weiteren Ordner hinzufügen» setzt den Fokus ins Pfadfeld der neuen Karte, die
  in einer Video-Bibliothek die andere Inhaltsart vorschlägt; «Entfernen» gibt den Fokus an den
  Hinzufügen-Knopf; die Liste hält ein Minimum (`min`). `folderListError` nennt Überschneidungen
  innerhalb der Liste und Listenfehler vor dem Absenden und sperrt den Knopf solange (ein leerer
  Pfad ist Sache des Pflichtfelds, nicht der Liste); der Server prüft dasselbe noch einmal
  (`root_overlap` mit Index). Eingaben heissen `roots[i].path` wie die
  Vertragsfelder, so landen die Feldfehler des Servers direkt am Feld; Fehler an `.providers` oder
  `.content` zeigt die Karte unter der Liste.

**Seiten:** `LibraryFields` (geteilt mit dem Assistenten) trägt die drei Familien «Video /
Hörbücher / Podcasts» und die Ordnerliste; `libraryRequest` baut `roots[]` aus den Karten; der
Absendeknopf zählt («Bibliothek mit 2 Ordnern hinzufügen», «Bibliothek ohne Ordner hinzufügen»).
`LibrariesPage` lädt `listMetadataProviders` (`metadataProvidersQuery`). `LibraryCard` zeigt bei
einem Ordner die Kurzform und bei Video den Inhalt-Chip (bei Hörbüchern und Podcasts nennt ihn
schon der Art-Chip), bei mehreren «2 Ordner · 1 Filme, 1 Serien» (`rootSummary`).
`LibraryFoldersDialog`: jede Zeile mit Inhalt-Chip und Kurzform, «Ändern» öffnet
`ChangeRootDialog` (Inhalt, `ProviderList`; Hinweis je Wirkung aus 6.3: nur Auffüller,
Erkennerwechsel, Inhaltswechsel mit abgetippter Bestätigung über `ConfirmInputDialog`,
`confirmPath` reist wie beim Entfernen); «Neue Ordner» ist die Ordnerliste mit Minimum 0 und dem
Knopf «3 Ordner hinzufügen» (`LibraryRootsRequest`). `SetupPage`: dieselbe Ordnerliste mit
Präfix `library.roots`, die Pfadprüfung je Karte (`pathCheck` mit Kartenschlüssel), feste
Vorgabeliste ohne Hinweise. `PodcastsPage`: Chip «aus Ordner» und «N lokal» bei `feedOrigin:
file`, «Abo löschen» dort ausgeblendet, `feedUrl` optional. `AdminItemPage`: Zeile
«Metadatenquellen» (Kurzform), Zeile «AniList» mit der Id als Link auf `anilist.co/anime/<id>`
(`FactList` nimmt dafür `ReactNode`-Werte), Panel «Herkunft je Feld» aus `fieldSources`
(«Beschreibung (de) → TMDB», Genres → «Vereinigung aller Quellen»). `packages/api-client/src/index.ts`
exportiert `LibraryKind`, `RootContent`, `MetadataProviderInfo`, `FieldSource`, `LibraryRootUpdate`.

**Locales:** `library.content.*`, `library.provider.*`, `library.providers.*`, `library.roots.*`
(mit Pluralformen `submit`, `addSubmit`), `library.videoHint`, `admin.libraries.rootSummary`,
`admin.libraries.folders.new`, `admin.libraries.folders.change.*`, `admin.podcasts.fromFolder`,
`admin.podcasts.localEpisodes`, `admin.item.facts.providers`, `admin.item.sources.*`,
`format.or`, `format.listSeparator`; `pnpm -C web test locales` grün.

**Tastaturtests (Regel 8):** `admin-libraries.spec.ts` (Tab-Reihenfolge mit Ordnerliste und
zählendem Knopf; neuer Test: Inhalt per Pfeiltaste, Häkchen per Leertaste, Sortieren per
Pfeiltaste auf dem Knopf mit bleibendem Fokus, Auffüller vorne wird gemeldet und sperrt den
Knopf, «+ Weiteren Ordner hinzufügen» per Enter mit Fokus im neuen Pfadfeld, «Entfernen» mit
Fokus zurück auf dem Knopf); `admin-library-folders.spec.ts` (Karte statt Feld im Dialog; neu:
zwei Ordner in einem Absenden, «Ändern» mit Auffüller-Wechsel ohne Bestätigung, Inhaltswechsel mit
Escape und abgetipptem Pfad, zurück); `setup.spec.ts` (Tab-Reihenfolge mit Karte, Inhalt und
Hinzufügen-Knopf; Podcasts ohne Karte); `global-setup.ts` legt die gemischte Bibliothek «Dokus»
mit zwei versteckten Ordnern in **einem** `createLibrary` an (F-13), ohne Medien, damit die
Zählungen der anderen Specs stehen bleiben. Zwei Fallen aus dem ersten Lauf: Playwrights
`getByRole(…, { name })` passt als Teilzeichenkette, «Ordner von Dokus bearbeiten» traf darum
`name: 'OK'` des Ordner-Browsers, und «Weiteren Ordner hinzufügen» / «1 Ordner hinzufügen» treffen
«Ordner hinzufügen»; die Specs nennen diese Knöpfe mit `exact: true`, der zählende Knopf des
Dialogs heisst «1 Ordner hinzufügen».

**Nachbesserung des Layouts (29. Oktober 2026, abends):** Marco fand das Formular «Bibliothek
hinzufügen» mit der ersten Fassung nicht übersichtlich (Zeilen der Quellenliste brachen um, die
Textknöpfe «Nach oben» / «Nach unten» frassen die Breite, drei Hilfstexte je Karte). Geändert:
die Quellenliste zeigt je Zeile Häkchen, Name mit Chip, eine Zeile Hinweis mit Auslassung und
zwei Pfeilknöpfe (nur Symbol, `aria-label` «TMDB nach oben»); «Durchsuchen …» steht neben dem
Pfadfeld (`FolderList` trennt `pathButton` rechts vom Feld und `pathAddon` darunter, das der
Einrichtungsassistent für seine Prüfung braucht); der Pfad bekommt den Platzhalter `/media/Filme`
statt eines Hilfstexts; die Hinweise zu Inhalt und Quellen sind je ein Satz; der Satz unter
«Weiteren Ordner hinzufügen» entfällt (der zählende Knopf sagt es); die Spalte des Formulars ab
1024 px breiter (`minmax(24rem, 2fr)`); der Dialog «Ordner von …» ist `size="wide"`, damit Pfad
und Knöpfe «Ändern» / «Entfernen» in einer Zeile stehen. Belege: `pnpm -C web run lint` ohne
Befund, Vitest **642 Tests in 79 Dateien**, `task ci:e2e -- admin-libraries admin-library-folders
setup admin-folder-browser podcasts` auf `SLOUCHCAT_E2E_PORT=8297` **66 passed (1.7m)**, A-9-Logscan
0 Befunde; die Storybook-Baselines der beiden Bauteile fehlen weiterhin (kein Docker, siehe unten).
Zweiter Befund von Marco im Ordner-Browser: über der obersten Zeile stand eine weisse Linie. Das war
der Fokusring der Zeile darüber: er liegt mit `outline-offset: 2px` ausserhalb der Zeile, die Liste
scrollt (`overflow-y: auto`) und schneidet ihn ab, sichtbar blieb nur das Stück in der Lücke
zwischen zwei Zeilen. `PathList` zeichnet den Ring jetzt nach innen (`outline-offset: -3px`); der
e2e-Helfer für P-15 liest weiterhin die berechnete `outline`. Die Storybook-Baseline von `PathList`
(fokussierte Zeile) ändert sich damit und braucht denselben `task storybook:update`.

**Belege (DoD Schritt 5):**

| Kriterium | Beleg |
|---|---|
| `task web:lint`, `task web:test` grün | `task ci` (29. Oktober 2026, Stand nach dem Review): `ci:web` ESLint, stylelint, `tsc` (beide Projekte), Prettier ohne Befund; Vitest **642 Tests in 79 Dateien** (api-client 4, design-tokens 2), `pnpm -C web test locales` grün |
| `task web:storybook:test` | Baselines der Stories `ProviderList.stories.tsx` (6) und `FolderList.stories.tsx` (4) nachgeholt, im Playwright-Image auf CT 190 (siehe «Nachbesserung des Layouts» oben): 171 passed, 159 bestehende Baselines byteidentisch |
| `task web:e2e` grün, Tastaturtests für beide Bauteile und «Ändern», «Dokus» mit zwei Ordnern in einem Absenden | `task build` und `task ci:e2e` auf `SLOUCHCAT_E2E_PORT=8197`: **448 passed (23.8m)**, A-9-Logscan 65 336 Zeilen, 0 Befunde; die Teilläufe der fünf betroffenen Specs auf 8297 (Chromium und Firefox) gingen voraus |
| Unit-Tests der Bauteile | `src/ui/ProviderList.test.tsx` (4), `src/ui/FolderList.test.tsx` (4), `LibrariesPage.test.tsx`, `LibraryFoldersDialog.test.tsx`, `SetupPage.test.tsx`, `AdminItemPage.test.tsx`, `PodcastsPage.test.tsx` angepasst oder neu |
| `pnpm -C web test locales` grün | 2 Tests grün |
| `grep -rn "'movies'" web/src --include=*.tsx` leer; `'series'` nur als Titelart und Suchfilter | leer; Treffer in `FolderList.tsx` (Inhaltsart), `MaintenancePage.tsx`, `MatchDialog.tsx`, `ItemPage.tsx` (Titelart), `FilterChips.stories.tsx` |

### Schritt 6 — Abnahme, Messung, Doku (`docs(dev,perf): library folders with content and provider lists, acceptance on CT 190`)

**Eingefrorener Stand** `testdata/db/bibliotheken.sql` (Beschreibung in `testdata/db/README.md`):
«Dokus» mit Film- und Serienordner in einem Zug, «Anime» mit TVDB als Erkenner, «Podcasts» mit
einem Abo von Hand, das ein Ordner übernommen hat (eine Episode mit Datei **und** Enclosure), zwei
gematchte Serien mit Herkunftszeilen, ein Titel mit `anilist_id` (per SQL ergänzt, die Testmedien
sind keine Anime). `TestFrozenStatesMigrate` nimmt ihn über das Glob mit,
`TestFrozenBibliothekenStateCoversTheConcept` prüft seinen Inhalt. Der Freeze-Lauf deckte die
Adoptionsreihenfolge in Schritt 3 auf (Korrektur oben).

**Versionsstellen auf 1.3.0:** `deploy/.env.example`, `deploy/compose.yaml`, `deploy/README.md`,
`docs/INSTALLATION.md` (Assistent und Ordner-Abschnitt beschreiben Art, Inhalt, Metadatenquellen
und Podcast-Ordner), `docs/dev/HANDOVER.md`; `docs/ideas/README.md` nennt Idee 01 Teil A als
gebaut.

**Release-Notes-Entwurf 1.3.0** (C-5, der Bruch wörtlich):

- Bibliotheken sind Familien: `LibraryKind` verliert `movies` und `series` und bekommt `video`;
  entfernte Enum-Werte werden nie wiederverwendet. Eine Video-Bibliothek darf Film- und
  Serienordner zugleich tragen.
- Jeder Ordner trägt `content` (`movie`, `series`, `audiobook`, `podcast`) und `providers`, eine
  geordnete Liste aus `tmdb`, `tvdb`, `anilist`, `local`: der erste erkennt, die weiteren füllen
  auf. `LibraryRequest.path` entfällt zugunsten von `roots[]` (Pflicht bei `video` und
  `audiobooks`), `addLibraryRoot` nimmt `LibraryRootsRequest` (eine Liste, alle oder keiner),
  `LibraryRootRequest` hat die Pflichtfelder `content` und `providers`.
- Neu: `updateLibraryRoot` (`PATCH /admin/libraries/{id}/roots/{rootId}`; ein Inhaltswechsel
  verlangt `confirmPath`, 409 `scan_running`), `listMetadataProviders`
  (`GET /admin/metadata/providers`), die Schemata `RootContent`, `LibraryRootsRequest`,
  `LibraryRootUpdate`, `MetadataProviderInfo`, `FieldSource`; `AdminItem` bekommt `providers`,
  `fieldSources`, `anilistId`; `AdminPodcast` bekommt `feedOrigin`, `rootId`,
  `localEpisodeCount`, `feedUrl` wird optional.
- `MetadataProvider` wächst um `anilist` und `local` (`searchMetadata` und `assignMatch` nehmen
  weiter nur `tmdb` und `tvdb`, 422 `provider_not_searchable`); `DuplicateGroup.reason` bekommt
  `file_and_feed`; neue Fehlercodes `content_not_allowed`, `providers_not_allowed`,
  `roots_required`, `root_overlap` (Feldfehler mit Index, auch bei `createLibrary`, `runSetup`
  und `addLibraryRoot` statt `library_path_invalid` als Problemcode).
- Podcast-Bibliotheken nehmen Ordner im Layout von Audiobookshelf an; eine `feedURL` in der
  `metadata.json` wird mitabonniert. AniList ist der dritte Anbieter (ohne Schlüssel,
  `SLOUCHCAT_ANILIST=off` schaltet ihn ab); die Zuordnungsliste von Fribb wird nur zur Laufzeit
  geholt. Migrationen 00023 und 00024; der Server sichert die Datenbank vorher nach
  `/config/backups/`.
- Kein `scoped`-Schema ändert sich: `ItemRef` und `ItemDetail` bleiben, Zuschauer sehen weder
  `anilistId` noch die Herkunft.

**Umstellung auf CT 190 (10.8):** am 29. Oktober 2026, 18:56 Uhr, Image
`ghcr.io/slouchcat/slouchcat:bibliotheken` (212 MB, Version `bibliotheken-af313cc`), auf dem CT aus
der Laufzeitstufe des Dockerfiles gebaut, weil Docker hier nicht erreichbar war; `.env` gesichert
nach `.env.bak-vor-bibliotheken` (vorher `main-9923807`); Sicherung der Datenbank
`pre-migration-bibliotheken-af313cc-20261029T175647.352Z.db`; Migrationen 00023 (1 773 ms), 00024,
00025; `/healthz` 200; Bestand wie in 8.1 vorausgesagt (`docs/perf/bibliotheken.md` 3). **Die
Abnahme selbst** (Zielstruktur anlegen, Mischbibliothek, Scraper-Liste sortieren, Anime-Serien
mit TVDB zuerst, Podcast-Ordner, Inhaltswechsel mit Bestätigung, Bibliotheksrechte eines
Kinderprofils) **macht Marco**; der Erst-Scan der Audio-Ordner ist der Messlauf aus 12.5 und
dauert Stunden.

**Abgenommen von Marco am 29. Oktober 2026** am laufenden CT 190 (Image `bibliotheken-4c6e89e`),
nach zwei Nachbesserungen am Layout (`8c913bf`: Formular und Dialog aufgeräumt; `4c6e89e`:
Fokusring im Ordner-Browser nach innen). **Storybook-Baselines nachgeholt** (Commit
`test(web): add the storybook baselines of the folder and provider lists`): weil Docker auf dem
Entwicklungsrechner in keiner Sitzung dieses Tages erreichbar war (die Gruppe `docker` ist seit dem
18. Oktober eingetragen, die Desktop-Sitzung aber älter), lief der Vergleich auf CT 190 im selben
Image `mcr.microsoft.com/playwright:v1.64.1-noble`: statischer Storybook-Build, Spec, Baselines und
`src/ui/*.tsx` hinüberkopiert, `webServer` auf `http-server` statt `vite preview` (die statischen
Dateien sind dieselben), `playwright test --update-snapshots`: **171 passed (45.8 s)**. Ergebnis:
10 neue PNG (`ui-providerlist--*` 6, `ui-folderlist--*` 4), `ui-pathlist--focused.png` geändert
(Ring innen), die übrigen 159 Baselines **byteidentisch**; das Image auf dem CT rendert also wie
die CI. Offen bleiben die Werte des Erst-Scans der Audio-Ordner für `docs/perf/bibliotheken.md` 3.
Merge auf Marcos Wort durch die Hauptsession.

**Belege (DoD Schritt 6):**

| Kriterium | Beleg |
|---|---|
| `task ci` grün | derselbe Lauf wie in Schritt 5 (Server 32 Pakete `ok` mit `-race`, `golangci-lint` 0, `gen:check` ohne Drift, Web 642 Tests, e2e 448 passed); der Storybook-Screenshot-Vergleich fehlt wie dort |
| Eingefrorener Stand migriert, `foreign_key_check` leer, Inhalt geprüft | `go test ./internal/store/ -run Frozen`: `ok` (14 Stände) |
| Messwerte in `docs/perf/` | `docs/perf/bibliotheken.md` Abschnitt 1 (Scanner), 2 (Anime-Lauf), 3 (CT 190: Umstellung, Migrationen, Konvergenz; der Erst-Scan der Audio-Ordner folgt mit Marcos Abnahme) |

## Benutzerverwaltung: Administrator, Konto, Profil (28. Oktober 2026)

Auf dem Zweig `claude/benutzerverwaltung` umgesetzt, nach dem Konzept
`docs/planning/06-benutzerverwaltung.md`, den Entscheiden in Frage 12 des Entscheidungsprotokolls
und **ADR 0014**. Sieben Commits, einer je Planschritt; vor jedem Abschluss war `task ci` grün.

**Was sich ändert.** Es gibt drei Ebenen statt zweier Kontoarten: **Administrator** (verwaltet
Server, Bibliotheken, Einstellungen, Jobs, Integrationen), **Konto** (verwaltet nur die eigenen
Profile) und **Profil** (schaut). Der Besitzer ist kein eigener Rang, sondern ein Abzeichen am einen
Konto: unlöschbar, nicht degradierbar, und als einziger darf er ernennen und entziehen. Das Wort
«Gast» ist weg — es versprach etwas Kleineres, als es bedeutete.

**Migration 00022** baut `accounts` um: `role` kennt nur noch `'admin'` und `'user'` (Standard
`'user'`, damit ein vergessenes `role` ein harmloses Konto erzeugt), der Besitzer steht in der neuen
Spalte `is_owner` mit Teilindex. Zwei CHECKs halten das Modell: ein Administrator trägt immer
`max_rating = 18` und `all_libraries = 1`, und ein Besitzer ist immer ein Administrator. Die View
`visible_items` wird **wortgleich aus 00010** wiederhergestellt, nicht aus 00007 wie das Konzept
schrieb — 00010 hat sie zuletzt neu geschrieben und die Regel für Extras ergänzt.
`TestFrozenStatesMigrate` fährt die Migration über alle eingefrorenen Stände, `PRAGMA
foreign_key_check` bleibt leer.

**Die Vertragsfassung steigt auf 1.2.0, und die Anhebung ist brechend.** Das gehört wörtlich in die
Release-Notes:

- `Account.role` und `AdminAccount.role` verlieren die Werte `owner` und `guest` und tragen
  `admin` und `user`; entfernte Enum-Werte werden nie wiederverwendet.
- Beide Schemata bekommen das Pflichtfeld `isOwner`.
- `GET /admin/guests` **entfällt**; `listAccounts` ist die einzige Kontoliste und trägt jetzt auch
  die W-8-Grenzen.
- `updateGuest` heisst **`updateAccountAccess`** (`PATCH /admin/accounts/{id}`, Antwort 204 statt
  200), `revokeGuest` heisst **`deleteAccount`** (`DELETE /admin/accounts/{id}`).
- Neu: `setAccountRole` (`PUT /admin/accounts/{id}/role`) mit der **sechsten Sichtbarkeitsstufe
  `owner`**, und die Fehlercodes `owner_required` (403), `owner_protected` (409), `last_admin`
  (409) und `admin_limits_fixed` (409).

**Versionsstellen stehen auf 1.2.0** (`deploy/.env.example`, `deploy/compose.yaml`,
`deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`). **Marco hat am 28. Oktober 2026
entschieden: 1.2.0 ist richtig.** 1.1.0 war vorbereitet, ist aber nie getaggt worden und wird
übersprungen. Daraus folgt: **die Release-Notes von 1.2.0 tragen beide Brüche**, den von 1.1.0
(`HomeRowKey` verliert `trakt_trending` und `trakt_popular`, bekommt `spotlight` und `released`) und
den dieser Änderung (Rollen-Enum, `GET /admin/guests`, die zwei Umbenennungen). Wird der Bruch von
1.1.0 vergessen, steht er in keinen Notes, weil es die Fassung 1.1.0 nie gegeben hat.

**Was Marco am Bildschirm sieht.** Die Seite «Verwaltung → Konten» ist die eine Kontoliste: Rolle
mit Besitzer-Abzeichen, Altersstufe, Bibliotheken, Profile, letzter Kontakt, Kanäle. Je Zeile
«Zugriff» (nur bei Konten ohne Verwaltung), «Zum Administrator ernennen» bzw. «Rolle entziehen»
(nur beim Besitzer sichtbar) und «Entfernen». Oben der Knopf **«Konto einladen»**: der Dialog
fragt Notiz, Altersfreigabe und Bibliotheken, zeigt danach den Einmal-Link ein einziges Mal und
schliesst mit «Fertig». Unter der Tabelle stehen die Einladungen mit ihrem Zustand und dem
Widerruf. **Die Seite «Zugriff» gibt es nicht mehr** (Nachtrag in ADR 0014, 28. Oktober 2026);
`/admin/access` leitet auf `/admin/accounts` um. Der Client setzt nichts durch: er blendet Knöpfe
aus, der Server entscheidet.

**Eingefrorener Datenbankstand.** `testdata/db/benutzerverwaltung.sql` trägt alle drei Ebenen:
den Besitzer mit zwei Profilen, ein Konto ohne Verwaltung mit eigener Altersstufe und eigenen
W-8-Grenzen, und einen zweiten Administrator, der durch Ernennung auf 18 und alle Bibliotheken
gestiegen ist. `TestFrozenStatesMigrate` nimmt ihn über das Glob mit,
`TestFrozenUserManagementStateCoversTheThreeLevels` prüft seinen Inhalt. Der Name trägt bewusst
keine Meilensteinnummer — die Benutzerverwaltung ist keiner.

**Zwei Befunde, die das Konzept korrigieren.**

1. Die View kommt aus 00010, nicht aus 00007 (oben).
2. Der Riegel `last_admin` ist bei vorhandenem Besitzer **strukturell unerreichbar**: der Besitzer
   ist Administrator und weder löschbar noch degradierbar, also bleibt immer einer übrig. Der
   Riegel bleibt trotzdem, als Wächter für eine Datenbank ohne Besitzerabzeichen, und
   `TestLastAdminGuardWithoutAnOwner` stellt genau diesen Zustand her, damit der Wächter geprüft
   ist statt bloss behauptet.

Stand: 27. Oktober 2026. **M0 bis M11 sind in `main`** (`b91a1d9`), dazu nach 1.0 der Ordner-Browser
(#34, #35), der Explorer-Entscheid (#37, #38) und die parallele CI (#36). **M11 «Spotlight und
Kuratierung» ist als #39 gemergt** (Merge-Commit `7c7f472`, 18 Commits), danach zwei Nachbesserungen am
Protokoll (#40, #41) und fünf Dependabot-Anhebungen der GitHub-Actions (#25, #26, #27, #29, #30). Der
Abschnitt zu M11 steht gleich unten.

**Version 1.1.0 wurde vorbereitet, nie getaggt und am 28. Oktober 2026 übersprungen** (Abschnitt
«Benutzerverwaltung» oben): die Versionsstellen zeigen seither auf 1.2.0. Die vier Betreibertexte zum
Trakt-Schlüssel nennen nicht mehr die zwei entfallenen Reihen, sondern Spotlight. Die Release-Notes
sind der Text der Tag-Beschriftung; den Tag setzt die Hauptsession auf Marcos Wort auf den Stand von
`main`, erst danach baut und veröffentlicht `release.yml` das Image. **Die Anhebung ist brechend**,
und genau das gehört in die Notes von 1.2.0: `HomeRowKey` verliert `trakt_trending` und
`trakt_popular` und bekommt `spotlight` und `released`; entfernte Enum-Werte werden nie
wiederverwendet.

**Falle beim Mergen.** `gh pr merge --auto --merge` merged sofort, wenn auf `main` kein Branch-Schutz
mit Pflicht-Checks liegt — PR #39 ging so vor dem Ende des CI-Laufs hinein (der Lauf war danach grün).
«Allow auto-merge» ist eingeschaltet, ein Ruleset mit Required Checks fehlt. Bis es steht, ist
`--auto` kein Tor, sondern nur ein Knopf.

Älterer Kopf (23. Oktober 2026): **M0 bis M9 sind in `main`** (`https://github.com/SlouchCat/slouchcat`,
Pull Requests #1 bis #6, #9, #10, #11, #14 bis #23; zuletzt der Merge-Commit `f17351b`). Der CI-Lauf auf
`f17351b` ist **grün** (Job `ci` 59.8 min, Job `image` 2.8 min) — der erste Lauf, der alle Korrekturen
aus #18, #20 und #21 zusammen geprüft hat. Marco hat M8 und M9 am 23. Oktober am laufenden Container
abgenommen.

**Version 1.0.0 ist veröffentlicht** (23. Oktober 2026, auf Marcos Wort). M10 ist als #24 gemerged,
dazu die zwei Nachbesserungen am Release-Weg (#31, #32). `main` steht auf `6f78a5c`, der Tag
`v1.0.0` darauf. Das eigentliche Ergebnis des Meilensteins ist der **Abnahmebericht
`docs/dev/ABNAHME-1.0.md`**: je Zeile der Abnahmematrix und je Plan-Kriterium steht dort, womit sie
belegt ist oder welche Abweichung gilt.

Nachgeprüft am veröffentlichten Stand, nicht an der Meldung des Workflows: `ghcr.io/slouchcat/slouchcat`
ist öffentlich und liefert `:1.0.0` und `:latest` für **linux/amd64 und linux/arm64**; ein anonymes
`docker pull` und ein Start daraus ergeben `healthy` nach 1.142 s bei 48.8 MB Leerlaufspeicher; der
Lizenzordner im Image trägt `LICENSE`, `FFMPEG-NOTICE.md`, `FFMPEG-COPYRIGHT` und `FFMPEG-VERSION`.
Das Manifest der Update-Prüfung antwortet unter
`https://github.com/SlouchCat/slouchcat/releases/latest/download/manifest.json` mit Fassung, Adresse
und dem Text aus der Tag-Beschriftung.

**Zwei Fehler im Release-Weg, beide erst beim Veröffentlichen aufgefallen** — er lässt sich nicht
anders prüfen als durch ein Release:

1. `ghcr.io/${{ github.repository }}` behielt das grosse S von `SlouchCat`; Docker-Paketnamen müssen
   klein sein, der Build brach vor dem Hochladen ab. Der erste Tag `v1.0.0` wurde darum gelöscht und
   nach der Korrektur neu gesetzt — veröffentlicht war zu dem Zeitpunkt nichts (#31).
2. Die Release-Notes kamen aus dem Commit statt aus der Tag-Beschriftung: `actions/checkout` schreibt
   `refs/tags/<tag>` direkt auf den Commit, das Beschriftungs-Objekt fehlt im Klon. Die schon
   veröffentlichten Texte sind an Ort und Stelle korrigiert, die Ursache in #32 (dort steht auch,
   dass eine **Organisationsregel** öffentliche Pakete verbieten kann — bis die umgestellt ist, bleibt
   «Public» am Paket ausgegraut).

Die CI läuft seit dem 25. Oktober 2026 in parallelen Jobs statt in einem seriellen (Abschnitt gleich
unten). Die Übergabe an eine neue Hauptsession steht in `docs/dev/HANDOVER.md`.

**Nach 1.0 in Arbeit:** der Ordner-Browser im Bibliotheksformular (Idee 02, beauftragt am
25. Oktober 2026). Die Schritte 1 bis 11 sind als PR #34 (Merge-Commit `b551a63`) in `main`; die
Schritte 12 bis 14 stehen auf dem Zweig `claude/bibliothek-wurzeln`. Der Abschnitt dazu steht gleich
unten.

## Meilenstein M11 – Spotlight und Kuratierung (26. Oktober 2026, Zweig `claude/m11-spotlight-7f52c5`)

Plan: `docs/planning/05-plan-spotlight.md` (Schritte 0 bis 9), Entscheide als Frage 11 in
`docs/planning/04-entscheidungen.md`, ADR `docs/adr/0011-spotlight.md`, Messwerte `docs/perf/M11.md`.
Marco hat die Schritte 0 bis 4 am 26. Oktober 2026 abgenommen und die Schritte 5 bis 9 freigegeben. Nicht
gepusht, kein PR. Die Umsetzung lief mit oh-my-claudecode: Fundament, Vertrag, Reviews und Commits in
der Hauptsitzung, die Hälften je Schritt über `executor`-Agenten auf getrennten Dateien.

### Die Commits

| Schritt | Commit | Inhalt | `task ci` |
|---|---|---|---|
| 0 | `d27bad6` | Frage 11 mit elf Antworten und fünf Folgeentscheiden, ADR 0011, Hinweis in ADR 0008; Anker des Plans von `2ca2282` auf `b87ac59` nachgezogen (69 Zeilenangaben geprüft), Migrationsnummer 00021 | grün (428) |
| 1 | `327b7cd` | Migration 00021 (`home_rows` neu, `spotlight_config/entries/overrides`, Übergang der Trending-Zeilen, `trakt_lists` weg, Index `items_released`), Vertrag 1.1.0 (`HomeRowKey`, `heroRow`, `hasTrailer`, `note` 280, `TraktStatus`), 47 Dateien | grün (428) |
| 2 | `1ee7493` | sieben Listenarten mit Pfadtabelle, Benutzerliste per Adresse mit Nachfilter, Watchlist abgelehnt; Overrides mit der Ordnungsregel (dedup, einfügen, schneiden), `min_hits` vor dem Schnitt, kein Abfragefenster | grün (428) |
| 3 | `f42670a` | Reihe «Kürzlich veröffentlicht» (12 Monate, Serien über die neueste sichtbare Folge) | grün (428) |
| 4 | `1143e0c` | fünf Admin-Operationen, `HomePage.heroRow`, `FirstHero`, Prepaint nur zuoberst, `hasTrailer` in zwei Reihen | 427 + Flake-Rerun |
| 5 | `1f684f3` | Admin-Seite `/admin/spotlight` nach Mockup H, Symbol, Tastatur-Spec | grün (430) |
| 5b | `fb6b5a1` | Admin-Bildroute `getAdminItemImage`, `posterUrl`, Picks-Seite nach Mockup I | grün (430) |
| 6 | `c84d242` | Seite «Startseite» ohne Trakt-Block, Mock mit den S-2-Listen | grün (430) |
| Fix | `4638000` | manueller Spotlight-Abruf mit Priorität 7, wartender Auftrag wird angehoben (eigener Commit, Entscheid vom 26.10.) | grün (430) |
| 7a | `04e6ed6` | Startseite liest `heroRow`, Band an seiner Stelle | grün (434) |
| 7b | `c03d050` | gerahmtes Band mit Posterleiste, fünf Storybook-Baselines; Revert-Probe grün | grün (438) |
| 8 | `176cd1b` | Trailer-Knopf an der Bühne | grün (440) |
| 9 | `926101f`, `ba236d4` | Doku, Belege, Messwerte | grün (440) |
| Folge | `3a4b4c9` | ohne Bühne: `Link`-Preload des ersten Posters, `fetchpriority="high"` (Entscheid vom 26.10.) | grün (440) |
| Folge | `db9d364` | ohne Bühne: Prepaint der ersten sechs Poster im Shell (`#poster-prepaint`), Ablösung nach dem ersten gezeichneten Poster, Deckungstest in e2e (Entscheid vom 26.10.) | grün (440) |

### Abnahmezeilen (Spec-IDs und Plan-Kriterien)

| Zeile | Stand | Beleg |
|---|---|---|
| E-1 Karussell zuoberst | **Abweichung, entschieden (Frage 11, S-1):** die Bühne ist `spotlight` und steht an ihrer gespeicherten Stelle; Cat's Empfehlungen ist Posterzeile | `go test ./internal/discover -run 'HeroRow|FirstHero'`, `web/e2e/home.spec.ts` («spotlight on a later position», «no stage») |
| E-2 Cat's Empfehlungen handgepflegt | PASS, inhaltlich unverändert; Admin-Seite nach Mockup I mit Nr., Poster, Hinweiszeile | `web/e2e/admin-picks.spec.ts`, `PicksPage.test.tsx` |
| E-7 Trakt-Reihen | **Abweichung, entschieden (S-2, S-3):** eine wählbare Liste statt zwei feste; Anzahl 5–20, Mindesttreffer 0–10, Intervall 6–48 h | `go test ./internal/trakt` (Pfad je Listenart), `go test ./internal/admin -run Spotlight`, `web/e2e/admin-spotlight.spec.ts` |
| E-9 höchstens acht Reihen, Reihenfolge vom Hauptbenutzer | PASS wörtlich; Startwert `spotlight, picks, continue, recent, released, watchlist, collections, genres` | `TestHomeRowKeysAgree`, `TestFrozenStatesMigrate` (M1a–M10), `TestContractHomeRows` |
| I-7 nur Server-Schlüssel, nichts gesendet | PASS; Watchlist-Adressen 422; `list_url` nie im Log (`listurl` in der Redaction-Liste) | `TestContractNoTraktPerProfile`, `TestSpotlightUserListSecrets`, `TestFetchUserListErrorsHideAddress` |
| W-10 Trailer von der Detailseite | **Erweiterung (Frage 11):** zusätzlich an der Bühne, nur bei `hasTrailer`; offline nicht ausgeblendet (offener Punkt) | `web/e2e/home.spec.ts` («the trailer button on the stage») |
| A-2 Verwaltung an einem Ort | PASS: Karte und Navigationseintrag «Spotlight» in der Gruppe Kuratierung | `web/e2e/admin-home.spec.ts` (17 Karten) |
| C-5 Vertrag ehrlich | PASS: Fassung 1.1.0, Enum getauscht, sechs neue Operationen, jede mit `x-visibility` und Registry-Eintrag | `TestContractCoversEveryOperation`, `TestEveryRouteHasVisibility`, `TestBinaryRoutesMatchContract` |
| K-3 Sichtbarkeit je Profil | PASS: Spotlight und «Kürzlich veröffentlicht» über `visible_items`, Admin-Zahlen systemweit | `go test ./internal/store -run Scoped`, `TestSpotlightRowIntersection`, `TestReleasedSeriesWithInvisibleNewestEpisode` |
| P-10 LCP < 2.5 s | **Mit Bühne PASS: 1.652 s** (M10: 1.802 s). **Ohne Bühne PASS: 2.328 s** nach den zwei Folge-Commits (`3a4b4c9` Preload: 4.031 s, `db9d364` Prepaint: 2.328 s; vorher 4.101 s). Abstand zur Grenze 172 ms, CLS 0 | `docs/perf/M11.md` 3a–3d |
| P-11 Startseiten-API < 300 ms p95 | PASS unter Last: 232.7 ms auf `m11-s6` mit gefüllter Liste (CT 190 während des `segments.detect`-Nachlaufs, Last 17 auf 2 vCPU; M10 auf ruhigem Server 105.4 ms); Lauf auf ruhigem Server offen | `docs/perf/M11.md` 1 und 2 |

### Planabweichungen und Befunde

- `POST /admin/trakt/refresh` blieb bis Schritt 4 im Vertrag (sonst zwischen 1 und 4 kein Endpunkt für
  Knopf und drei Specs); die fünf Operationen kamen in Schritt 4 (strict server braucht Handler).
- sqlc parst keine Window-Funktion: die Migration zählt den Rang korreliert.
- Die Minutensperre von `refreshSpotlight` trifft die e2e-Suite (ein Server für alle Worker): der Helfer
  wartet bei 429, der Knopftest akzeptiert 202 oder 429.
- `itemsaccess` verbietet Titel-SQL auch in `discover_test.go`: Fixture-Helfer liegen in
  `visibility/vistest`. `task server:gen:check` diffs gegen den Index: vor `task ci` alles stagen.
- **Produktfehler, behoben (`4638000`):** der manuelle Spotlight-Abruf reihte mit Priorität 2 ein und wartete
  auf CT 190 hinter 1683 `segments.detect`-Aufträgen (Tage). Jetzt Priorität 7 und Anheben des wartenden
  Auftrags; nächtlicher Lauf unverändert; ADR 0002 ergänzt.
- Der Storybook-Gate hat den dritten Knopf am Band nicht erkannt (dunkler Knopf auf dunklem Backdrop
  unter der Pixelschwelle); die Baselines mussten mit `--update-snapshots=all` neu geschrieben werden.
- Lighthouse-Methode: seit dem Gerätecookie braucht die `Cookie`-Kopfzeile beide Cookies, und
  `--browse-roots` muss die Bibliothek umfassen (`docs/perf/M11.md` 3). Beides steht seit dem
  27. Oktober 2026 auch als datierter Nachtrag in `docs/perf/M10.md` Abschnitt 8, wo die Messanleitung
  steht, die man beim nächsten Mal zuerst aufschlägt.
- Kein Select-Bauteil in `web/src/ui`: die Auswahlfelder der Spotlight-Seite sind `RadioGroup`s.

### Offen

- P-10 ohne Bühne: erfüllt (2.328 s), aber nur 172 ms unter der Grenze; `uses-responsive-images`
  (w400-Poster für 136 px breite Karten, 280 KiB) ist der nächste Hebel, falls der Abstand nicht reicht.
- P-11 auf ruhigem Server (CT 190 arbeitet den `segments.detect`-Nachlauf ab) und der Vergleichslauf auf
  `m11-s2`; `GET /` (LCP-Pfad) ist über die Lighthouse-Läufe mit abgedeckt.
- Mockup-I-Abnahme am laufenden Container (Image `m11-s8` liegt auf CT 190).
- Trailer-Knopf an der Bühne offline ausblenden wie auf der Detailseite (Folge-Commit).
- Ausstiegsklausel D3 → D1 nach der Messung des `EXISTS` auf ruhigem Server.
- Release-Notes für die brechende Vertragsänderung 1.1.0 beim nächsten Tag.

CT 190 stand vor M11 auf `ci-parallel` (`v1.0.0-46-gb87ac59`); Sicherungen `.env.bak-vor-m11-s2` und
`/config/backups/pre-migration-v1.0.0-49-g1ee7493-20261025T232327.523Z.db`. Migration 00021 ist dort
gelaufen; ein Rückweg führt über die Datenbanksicherung (nur vorwärts, Regel 5).

### Formell abgeschlossen (27. Oktober 2026)

Die zwei Punkte, die nach dem Merge von #39 und der Freigabe von 1.1.0 offen geblieben sind, sind
erledigt:

- **`testdata/db/M11.sql` liegt vor** (Regel 5 und `testdata/db/README.md`). Erzeugt wie die früheren
  Stände mit `slouchcat dump` gegen einen frischen Server: Assistent mit `frozen@example.test` und der
  Filmbibliothek auf `testdata/media/Movies` ohne TMDB-Schlüssel, dazu die Tabellen der Migration
  00021 — `spotlight_config` abseits der Vorgaben, fünf `spotlight_entries` aus einem echten Abruf
  gegen einen lokalen Trakt-Mock (drei davon mit Bibliothekstreffer über die `{tmdb-NNNN}`-Ordner der
  Testmedien), drei `spotlight_overrides` in allen drei Formen und die acht `home_rows` in geänderter
  Reihenfolge.
  `TestFrozenStatesMigrate` nimmt den Stand über seinen Glob auf; `TestFrozenM11StateCoversM11Data`
  hält die neuen Tabellen abgedeckt.
- **Die zwei Methodenbefunde zur LCP-Messung stehen jetzt dort, wo man sie sucht**: als datierter
  Nachtrag in `docs/perf/M10.md` Abschnitt 8 (beide Cookies, `--browse-roots` über der Bibliothek).
  Die Messwerte von M10 sind unverändert; nachgetragen ist nur die Anleitung.

## Nach 1.0: die CI läuft in parallelen Jobs (25. Oktober 2026)

Der Workflow hatte einen einzigen Job, der `task ci` am Stück ausführte, und brauchte dafür **56
Minuten 52 Sekunden** (Run 40207488836 auf `b551a63`). Jeder Pull Request wartete also knapp eine
Stunde auf sein erstes Urteil, und ein Tippfehler im ESLint-Lauf zeigte sich erst, wenn die
Go-Tests durch waren.

Geprüft wird unverändert dasselbe. Es läuft nur nebeneinander: ein Job für die Server-Codegeneratoren
und -Linter, eine Matrix für die Go-Testgruppen, ein Job für die Web-Seite samt Storybook, ein Job,
der das Binary einmal baut, sechs Shards der Playwright-Suite, der unveränderte `image`-Job, und
zuletzt ein Gate-Job, der auf alle wartet. **Der Lauf dauert damit 11 Minuten 27 Sekunden** (Run
40223187251), also ein Fünftel von vorher.

**Für den Alltag ändert sich nichts.** `task ci` bleibt der eine Befehl, der lokal alles baut und
prüft (Plan-Abschnitt 4). Er ruft dieselben Aufgaben auf, die die Jobs aufrufen — `ci:server`,
`ci:web`, `ci:e2e` und `build` —, nur nacheinander. Eine neue Prüfung kommt in eine dieser drei
Aufgaben, nicht in den Workflow; CI ruft nie etwas auf, das es als Aufgabe nicht gibt.

**Der Statusprüfname bleibt `ci / ci`**, weil der Gate-Job weiterhin `ci` heisst. Auf `main` ist
derzeit weder ein Branch-Schutz noch ein Ruleset gesetzt, der Name ist also Konvention; wird der
Gate-Job je umbenannt, muss die Merge-Bedingung mit umbenannt werden.

Begründung und Gegenleistung stehen im ADR `docs/adr/0013-ci-parallele-jobs.md`, die Zahlen je Job
in `docs/perf/ci.md`.

**Nebenbefund, der die App betrifft.** Das Sharden liess `web/e2e/maintenance.spec.ts` zum ersten
Mal in Firefox durch seinen Entfernen-Zweig laufen — bis dahin hatte Chromium die kaputte Datei
immer schon entfernt, und Firefox nahm den kurzen Pfad. In Firefox fiel der Test am sichtbaren
Fokus: «Entfernen» in einem Bestätigungsdialog hatte dort **keinen Fokusring**, auch in der
ausgelieferten Fassung (harte Regel 8). Ursache ist `:focus-visible`, das Firefox einem aus dem Code
heraus fokussierten Knopf nicht gibt; react-aria setzt für denselben Fall `data-focus-visible`, und
acht andere Komponenten hörten längst darauf. Der Knopf tut es jetzt auch.

## Nach 1.0: Ordner-Browser im Bibliotheksformular (Idee 02, 25. Oktober 2026)

Marco hat die Umsetzung am 25. Oktober 2026 beauftragt (Eintrag im Entscheidungsprotokoll,
Issue #33). Die **Schritte 1 bis 11** aus Abschnitt 10 des Konzepts `docs/ideas/02-ordner-browser.md`
sind als PR #34 in `main`. Die **Schritte 12 bis 14** (mehrere Ordner je Bibliothek, Löschweg,
Bibliothek ohne Ordner) stehen auf `claude/bibliothek-wurzeln`; ihr eigener Abschnitt folgt weiter
unten.

**Was es jetzt gibt.** Neben dem Ordnerfeld der Seite «Bibliotheken» steht «Durchsuchen …». Der
Dialog zeigt die freigegebenen Wurzeln, steigt mit Enter ab, geht mit der Rücktaste hinauf und
schreibt den Pfad mit «Übernehmen» ins Feld. Jede Zeile trägt bei Bedarf «Gehört zu Bibliothek X»
oder «Verknüpfung, wird nicht durchsucht»; die Kopfzeile nennt Dateisystem, Überwachungsart und die
Warnung für Netzfreigaben des aktuellen Verzeichnisses. Der Einrichtungsassistent bekommt kein
Blättern, sondern die konfigurierten Wurzeln als anklickbare Vorschläge.

**Was sich für den Betrieb ändert.** `SLOUCHCAT_BROWSE_ROOTS` nennt die Ordner, die der Browser
auflisten darf **und** unter denen eine neue Bibliothek liegen muss. Die Vorgabe ist `/media`, die
Regel greift also vom ersten Start an; wer seine Medien anderswo hat, muss die Variable setzen,
bevor er eine Bibliothek dort anlegen kann — auch im Einrichtungsassistenten, der dann
`not_browsable` mit dem Namen der Variablen antwortet. Nur eine **ausdrücklich leere** Variable
schaltet die Regel ab, und dann warnt der Start. Bestehende Bibliothekswurzeln behalten
Bestandsschutz (Pfadgleichheit, nie ein Präfix). Begründung und verworfene Alternativen stehen in
ADR `docs/adr/0012-browsbare-wurzeln.md`.

**Belege je Schritt** (alle lokal gelaufen):

| Schritt | Beleg |
|---|---|
| 1 `paths.ReadDirNames`, `paths.LstatAll`, `paths.FSTypeAt` | `go test ./internal/paths/`, `GOOS=darwin go build ./...` |
| 2 `SLOUCHCAT_BROWSE_ROOTS` | `go test ./internal/config/` |
| 3 `Service.ValidateNewRoot` (E14) | `go test ./internal/library/ ./internal/admin/`; Pfad ausserhalb und `<wurzel>/ablage -> /config` ergeben 422, `checkSetupPath` antwortet `not_browsable` ohne `os.Stat` |
| 4 geteilte Pfadregeln | `go tool golangci-lint run ./...`; die `depguard`-Regel `browse` weist einen Import von `store/systemq` und `transcode` zurück |
| 5 Paket `browse` | `go test -race ./internal/browse/`: Blättern über 300 Ordner, `@eaDir` gefiltert, Symlink sichtbar und nicht betretbar (auch einer, der innerhalb der Wurzel bleibt), eine Wurzel, die über einen Symlink hinausführt, wird abgelehnt, blockierende Dateisystem-Schnittstelle ergibt 503 und danach `browse_busy`, ein zurückgekehrter Aufruf zählt nicht als verlorener Platz, eine Seite kostet eine Namensliste (`paths.ReadDirNames`) plus ein `lstat` je angezeigter Zeile |
| 6 Vertrag | `task gen:check`, `task server:ci` |
| 7 Handler und Drosseln | `go test ./internal/api/ ./internal/admin/`: gleicher Körper für unbekannte Wurzel und fehlenden Pfad, 403 nur für Sitzung und CSRF, `Cache-Control: no-store`, zehn Wurzellisten lassen `POST /setup` durch |
| 8 `PathList`, `FormDialog` | `task web:lint`, `task web:test`, `task web:storybook:test` |
| 9 Dialog und Vorschläge | `task web:lint`, `task web:test` |
| 10 Tastaturtest | `task web:e2e` (`web/e2e/admin-folder-browser.spec.ts`) |
| 11 Doku und ADR | `task ci` |

**Review vor dem Pull Request** (zwei getrennte Durchgänge, Sicherheit und Code): 0 kritische, 0
hohe Sicherheitsfunde; behoben wurden ein Zähler, der jede abgebrochene Anfrage als verlorenen
Semaphorplatz zählte (das Dashboard hätte den Browser nach 16 Klicks dauerhaft als tot gemeldet),
ein `stat` je Verzeichniseintrag statt je angezeigter Zeile, Enter, das nach der ersten markierten
Zeile nicht mehr abstieg (`selectionBehavior`), veraltete Hinweise und Pfade beim Wechsel des
Ordners, der Wurzelpfad im Fehlerring, die fehlende Startwarnung bei leerer Wurzelliste und der
Bestandsschutz einer über einen Symlink erreichten Bibliothekswurzel.

**Nicht abgedeckt:** die Schritte 12 bis 14; ein echtes Netzlaufwerk, das mitten in einer
Auflistung hängen bleibt (im Test steht dafür eine blockierende Dateisystem-Schnittstelle); ein
Ordner mit 50 000 Einträgen am echten NAS; und die Sicht am Handy, die Marco selbst prüft. Die
Zusage «vor der Ablehnung eines Pfads ausserhalb der Wurzeln kein `os.Stat`» ist in
`internal/library` nur indirekt belegt (eine Datei ausserhalb ergibt `not_browsable` statt
`not_directory`), nicht mit einem Zähler in einer eingeschleusten Schnittstelle.
`BrowseRoots` liest je Seite die Bibliotheken aus der Datenbank — bewusst in Kauf genommen, weil
die Abfrage auf dem Leser-Pool läuft und das Dashboard häufiger abfragt.

## Nach 1.0: mehrere Ordner je Bibliothek (Idee 02, Schritte 12 bis 14, 25. Oktober 2026)

Auf `claude/bibliothek-wurzeln`, aufbauend auf PR #34. Marco hat am 25. Oktober 2026 die Bilder
seiner Emby-Installation gezeigt und drei Entscheide gefällt, die im Konzept als **E16 bis E18**
stehen: die Ordnerliste zieht von der Bibliothekskarte in einen eigenen Dialog «Ordner von X»
(E16), die abgetippte Bestätigung beim Entfernen **bleibt** (E17, E12 unverändert), und der
Ordner-Dialog bekommt die Emby-Form mit dem Pfadfeld oben und «OK» unten (E18).

**Was es jetzt gibt.** Die Bibliothekskarte nennt den Pfad, solange es einen Ordner gibt, und sonst
nur die Zahl («2 Ordner»). Der Knopf «Ordner» öffnet die Liste; je Zeile steht «Entfernen», darunter
«Ordner hinzufügen» mit demselben Browser-Dialog. Das Entfernen verlangt den abgetippten Pfad und
sagt im Text, was wirklich geschieht: Die Titel werden **weich** gelöscht und behalten Fortschritt,
Merkliste und Picks; ein Titel mit einer Datei in einem anderen Ordner derselben Bibliothek bleibt
sichtbar; wirklich weg sind die Offline-Downloads; eine laufende Wiedergabe bricht ab; die
Umsetzungstabelle von Radarr und Sonarr zeigt danach ins Leere; derselbe Ordner binnen 30 Tagen
wieder angelegt holt Titel mit Provider-Id samt Benutzerdaten zurück; und ein Ordner ausserhalb von
`SLOUCHCAT_BROWSE_ROOTS` lässt sich ohne Konfigurationsänderung nicht wieder hinzufügen.

**Der Löschweg** läuft unter der Scan-Sperre der Bibliothek und in **einer** Transaktion, in dieser
Reihenfolge: Titel der Wurzel ohne Datei unter einer anderen weich löschen (mit `audiobook`, das
`SoftDeleteItemWithoutFiles` nicht führt), dann leere Staffeln und Serien, dann die Downloads der
**überlebenden** Titel auf eine verbleibende Datei umhängen, dann die Wurzelzeile löschen; danach
Watcher beenden, `pathPending`, `orphans` und den Anteil an einem eingereihten `scan.path`-Auftrag
verwerfen. Die Reihenfolge ist Pflicht: `media_files.root_id` und `downloads.media_file_id` tragen
`ON DELETE CASCADE`, und `SoftDeleteItemsMissingSince` sieht nur Titel, die noch eine Datei haben.
Ein laufender Scan ergibt 409 `scan_running`, ein falscher `confirmPath` 422 `confirm_mismatch`.

**Bibliothek ohne Ordner** (E13): Sie bleibt mit ihren Einstellungen bestehen, queued aber keinen
Scan mehr und meldet deshalb auch kein `done` mit null Dateien; der Knopf «Erneut scannen» ist dort
abgeschaltet, und die Karte bekommt einen eigenen Satz statt `admin.libraries.noRoots`, der Titel aus
abonnierten Feeds verspricht.

Eine Audit-Tabelle gibt es nicht, darum schreibt das Entfernen eine Zeile auf `Warn` (Bibliothek,
Ordner, Zahl der gelöschten Titel und Dateien) und meldet über `notify`, wie es der Verlustschutz für
`root_missing` tut.

**Belege je Schritt** (alle lokal gelaufen):

| Schritt | Beleg |
|---|---|
| 12 `addLibraryRoot`, `removeLibraryRoot` | `task server:ci`; `go test ./internal/library/`: nach `removeLibraryRoot` kein Titel mit `deleted_at IS NULL` und null Dateien, ein Titel mit Datei unter einer zweiten Wurzel bleibt verfügbar **und behält seinen Download** (ohne Tombstone), ein Titel, der geht, nimmt seinen Download samt Tombstone mit, Hörbücher gehen mit, falscher `confirmPath` ergibt `ErrConfirmMismatch`, laufender Scan `ErrScanRunning`; `go test ./internal/api/`: 201, 200, 404, 409 `scan_running`, 422 `confirm_mismatch` |
| 13 Oberfläche | `task web:lint`, `task web:test` (`LibraryFoldersDialog.test.tsx`), `task storybook:test` (neue Story `TypedConfirmation`) |
| 13a Emby-Form des Dialogs (E18) | `task web:lint`, `task web:test` (`PathBrowserDialog.test.tsx`), `task web:e2e` (`admin-folder-browser.spec.ts`: Rücktaste im Pfadfeld blättert nicht hinauf) |
| 14 Bibliothek ohne Ordner | `go test ./internal/library/` (kein Scan-Auftrag, Zustand unverändert), `task web:e2e` |
| Tastaturtest (harte Regel 8) | `task web:e2e` (`web/e2e/admin-library-folders.spec.ts`) |

**Nicht abgedeckt:** `task server:vuln` meldet auf dieser Maschine GO-2026-6809 (`crypto/x509`,
behoben in go1.27.3) — lokal ist go1.27.2 installiert, `.tool-versions` pinnt 1.27.3; in CI ist der
Befund damit weg, auf dieser Maschine braucht es den Werkzeugwechsel, den Marco selbst macht. Der
Löschweg ist im e2e-Lauf nur an leeren Ordnern geprüft (Titel, Downloads und Tombstones decken die
Go-Tests ab, nicht der Browser). Die Zahl im Bestätigungstext ist die **Obergrenze** (Titel mit einer
Datei in diesem Ordner); wie viele davon eine Datei anderswo haben, zählt der Server erst beim
Entfernen. Und die Sicht am Handy prüft Marco selbst.

## Meilenstein M10 – Härtung und Release 1.0

Vorab geklärt mit Marco (23. Oktober 2026, Entscheidungsprotokoll): Er führt die offenen Prüfungen an
echten Geräten **nicht** durch («Wird schon funktionieren. Werde bei der nächsten Version noch
Änderungen vornehmen.»); die HDR-Referenzbilder aller vier Ketten hat er angesehen und freigegeben
(P-6 a); arm64 bleibt mangels Gerät nur unter Emulation geprüft; Messungen auf dem Referenz-NAS
CT 190 sind freigegeben.

**Das Ergebnis dieses Meilensteins ist der Abnahmebericht `docs/dev/ABNAHME-1.0.md`.** Er führt jede
der 64 Abnahmezeilen und jedes der 16 Plan-Kriterien einzeln auf — mit dem Beleg oder der geltenden
Abweichung. Die acht Abweichungen stehen dort gesammelt in einer eigenen Tabelle, damit keine in
einer Zeile untergeht; zwei davon (keine weiteren Gerätetests, arm64 nur emuliert) sind am
23. Oktober neu dazugekommen und seither ausdrücklich in der DoD von M10 genannt.

### Die acht offenen Punkte, mit denen M10 angetreten ist

1. **Formularfehler blieben nach der Korrektur stehen** — der sichtbarste Mangel, gefunden bei Marcos
   Abnahme. Zwei Ursachen: react-aria zeigt einen Server-Fehler wieder, sobald sich die Identität von
   `validationErrors` ändert, und jede Seite baut dieses Objekt bei jedem Rendern neu aus dem Fehler
   der Mutation — die Meldung kam also einen Rendervorgang nach dem Verschwinden zurück. Und
   react-aria lässt sie nur beim nativen `change` los, das ein Textfeld erst beim Verlassen auslöst.
   Neu ist `web/src/ui/Form` das Formular der App: Es reicht react-aria dasselbe Objekt, solange der
   Inhalt gleich ist, und nimmt ein Feld heraus, sobald dieses Feld eine Änderung meldet. Die Marken
   gehören zu der Antwort, unter der sie entstanden sind (Runden), damit die nächste Antwort immer
   wieder für jedes Feld spricht — auch wenn der Server sich wörtlich wiederholt. Alle 24 Formulare
   holen `Form` jetzt aus `@/ui/aria`; `no-restricted-imports` hält das schlichte react-aria-`Form`
   aus den Features heraus.
2. **Go-Testsuite parallelisiert.** `internal/api` war der lange Pol: 579 s von 9 min 41 s der ganzen
   Suite auf dem Entwicklungsrechner. Die Fixtures waren schon unabhängig (eigene SQLite-Datei je
   Test), nur lief nichts nebeneinander. Details und Messwerte unten.
3. **Leerlaufspeicher auf CT 190 nachgemessen** — die 80-MB-Schranke war nie auf der Hardware
   geprüft, für die sie gedacht ist (`docs/perf/M10.md` 1).
4. **arm64-Schwelle nachgeführt** von 120 auf 180 MB, mit fünf eigenen Messungen dahinter
   (`docs/perf/M10.md` 6).
5. **Das Pfad-Umsetzungsformular zieht nach**, wenn ein zweiter Admin dieselbe Integration ändert:
   ohne eigene Eingaben folgt es stillschweigend, mit eigenen Zeilen behält es sie, sagt Bescheid und
   bietet an, die fremde Fassung zu übernehmen.
6. **Verdichtung alter `progress_events`** (Plan 3.2, 90 Tage) gebaut — siehe unten.
7. **Release-Manifest wird veröffentlicht.** `.github/workflows/release.yml` läuft nur auf einen Tag
   `vX.Y.Z`: Image für amd64 und arm64 nach ghcr.io, das Manifest, das die Update-Prüfung liest, und
   das Release mit dem ffmpeg-Quellcode-Angebot. Bis jemand taggt, wird nichts veröffentlicht.
   `compose.yaml` nagelt die Fassung über `SLOUCHCAT_VERSION` fest.
8. **O-6 als deklarierte Abweichung geführt** — namentlich in der Abweichungstabelle des
   Abnahmeberichts (B-1) und in den Zeilen O-6 und P-9.

### Die fünf Plan-Schritte

1. **Fremd-Client-Test (P-16, zweite Hälfte von C-5).** `server/cmd/apicheck` ist ein Client, der den
   Server **nur** über `api/openapi.yaml` kennt: Eine depguard-Regel verbietet ihm jeden Import aus
   `internal/`, vor jedem Aufruf schlägt er Pfad und Methode im Vertrag nach, und ein Aufruf, den der
   Vertrag nicht beschreibt, lässt den Lauf scheitern (gegengeprobt mit `/wishes`). Er richtet seine
   Installation über `/setup` selbst ein und spielt die achtzehn Zuschauerfunktionen durch.
   `task apicheck`: **18 von 18 ok**, 85 Aufrufe in 3 s. Nicht Teil von `task ci` — die CI läuft
   ohnehin eine Stunde, und das hier ist ein Abnahmelauf, keine Regressionsschranke.
2. **Leistung auf dem Referenz-NAS und arm64** — `docs/perf/M10.md` Abschnitte 1 bis 6.
3. **Migrationstest aller DB-Stände M1a → 1.0.** `testdata/db/M10.sql` ist der eingefrorene Stand von
   1.0; er trägt die Wiedergabe-Geschichte in beiden Formen (eine rohe Zeile, ein verdichteter Tag,
   vom Server selbst verdichtet). `TestFrozenStatesMigrate` spielt elf Stände ein und migriert sie
   auf das 1.0-Schema.
4. **Sicherheitsreview und Installationsanleitung** — unten.
5. **Tag `v1.0.0`** — nicht in diesem Meilenstein. Den setzt die Hauptsession auf Marcos
   ausdrückliches Wort.

### Sicherheitsreview (Schritt 4)

Vollständig in `docs/dev/SICHERHEITSREVIEW-1.0.md`, geprüft wurde der ganze Baum gegen das
Bedrohungsmodell aus Plan 3.8. **Kritisch 0, hoch 0, mittel 2, niedrig 8, Hinweis 11**; kein
Geheimnis im Repository oder in der Historie. Alle neun geprüften Bereiche halten — darunter
maschinell: alle 152 Operationen tragen `x-visibility`, alle 13 mit `ItemRef` in der Antwort sind
`scoped`, und die Importeure von `store/systemq` sind genau die depguard-Liste.

Vor dem Tag behoben: **M-1** (`SLOUCHCAT_TRUSTED_PROXIES=private` vertraut jedem Gerät im Heimnetz
und macht mit offenem Port 8096 die Bremse der Anmeldeversuche wirkungslos — der Server warnt jetzt
beim Start, und `.env.example`, die Installationsanleitung und der Kopfkommentar der `compose.yaml`
empfehlen das Docker-Netz von Caddy als CIDR), **M-2** und **N-8** (alle fremden Actions auf
Commit-SHA gepinnt, `.github/dependabot.yml`, Provenance und SBOM am Release-Image), **N-3**
(`golang 1.27.3`) und **H-7**: Der temporäre Spike-Endpunkt `/spike/{variant}` aus M1a — mit
`Access-Control-Allow-Origin: *` — war immer noch da und wäre mit dem öffentlichen 1.0 ausgeliefert
worden. Er ist samt `SLOUCHCAT_SPIKE_DIR` und dem Generatorskript entfernt; das war ohnehin die seit
M1b offene Nacharbeit.

### Installationsanleitung

`docs/INSTALLATION.md`: für jemanden, der Slouchcat zu Hause betreiben will und kein Entwickler ist.
Von den Ordnern über `.env`, den ersten Start und den Assistenten bis HTTPS, Sicherung und
Aktualisieren — mit einem eigenen Abschnitt «Was nicht geprüft ist», damit niemand die Lücken erst im
Betrieb findet.

### Ein Befund, den niemand gesucht hat

Die Startseite war langsamer geworden, ohne dass es auffiel: gemessener LCP **2.702 s** statt unter
2.5 s (P-10). Die Ursache war nicht die gewachsene Seite. Die Hülle verzögert das Startprogramm
absichtlich, bis das Hero-Bild gezeichnet ist; der Bündler erzeugt seit einer Weile aber **zwei**
Modul-Skripte, und die Regex in `deferStartScript()` traf nur das erste. Verzögert wurde das
1-KiB-Laufzeit-Stück, der Einstiegspunkt mit 152 KiB stand wieder eifrig im Kopf. Gemeldet hat es
nichts — der Plugin warf nur, wenn er gar kein Tag fand. Nach der Korrektur (alle Modul-Skripte in
ihrer Reihenfolge, plus eine Schranke, die genau diesen Fall zum Build-Fehler macht): **1.802 s**,
derselbe Wert wie in M1a, obwohl die Seite inzwischen ein Karussell und sieben Reihen trägt.
Messung in `docs/perf/M10.md` Abschnitt 8.

### Belege (DoD)

| Prüfung | Befund |
|---|---|
| Abnahmebericht | `docs/dev/ABNAHME-1.0.md`: 64 Matrixzeilen und 16 Plan-Kriterien einzeln mit Beleg oder benannter Abweichung |
| Fremd-Client (P-16, C-5) | `task apicheck` — 18 von 18 Zuschauerfunktionen ok, Exit-Code 0 |
| Migrationstest (P-13) | `go test ./internal/store -run TestFrozenStatesMigrate` über elf Stände M1a → M10, dazu `TestFrozenM10StateCoversM10Data` |
| Leerlaufspeicher (P-2) | 83.0 MB bei 50'570 Titeln auf CT 190 (Grenze 150 MB bei 5'000), `docs/perf/M10.md` 1b |
| Antwortzeiten (P-11) | CT 190: Startseite p95 105.4 ms, während des Erst-Scans 109.8 ms, Suche p95 8.1–54.2 ms |
| Startseite (P-10) | LCP 1.802 s (Grenze 2.5 s), acht Reihen, keine leere; `web/e2e/home.spec.ts` grün |
| Grafikchip auf dem NAS (A-1) | `backend=qsv tonemapper=cpu budget=8`, `docs/perf/M10.md` 4 |
| HDR-Referenzbilder (P-6 a) | am 23. Oktober von Marco angesehen und freigegeben, alle vier Ketten |
| Sicherheitsreview | `docs/dev/SICHERHEITSREVIEW-1.0.md`, 0 kritisch / 0 hoch / 2 mittel, alle fünf dringenden Funde behoben |
| Testdauer | `internal/api` 579 s → 288 s; sieben weitere Pakete je um 35 bis 45 % kürzer, `docs/perf/M10.md` 7 |
| Verdichtung | `TestStatsSurviveCondensing`: dieselben Statistikzahlen vor und nach dem Verdichten |
| `task ci` | **exit 0** (23. Oktober 2026, 21 min 41 s): Codegen ohne Drift, Lint beidseitig, `go test -race ./...`, Vitest 585 + 4 + 2, Bau, Playwright **404 von 404** in Chromium und Firefox (19.9 min), Storybook **156 von 156** |
| Verifikation (DoD 6.2 Punkt 6) | `docs/dev/VERIFIKATION-1.0.md` — eigener Durchgang, sieben Befunde, alle behoben |

### Zwei Fehler, die erst der vollständige Lauf gefunden hat

Beide sassen in der Formular-Korrektur aus Punkt 1 — also in dem, was als Erstes fertig gemeldet war:

1. **Das erste Zeichen jedes Feldes ging verloren.** Der Horcher, der ein bearbeitetes Feld
   markiert, lief während des Tastendrucks; react-aria hatte den neuen Wert da noch nicht in den
   Zustand der Seite geschrieben, und die Neuzeichnung setzte den alten zurück. Die Anmeldeseite
   schickte `2e-owner@…` statt `e2e-owner@…`. **30 Playwright-Tests** haben es gemeldet, die
   Unit-Tests nicht und können es auch nicht: jsdom ordnet die Ereignisse anders als ein Browser.
   Genau davor warnte der Kopfkommentar von `web/src/lib/useEdited.ts`, der im Commit davor gelöscht
   wurde. Die Markierung wartet jetzt einen Takt.
2. **Vier Formulare hingen gar nicht an der neuen Komponente** (`AccountsPage`, `AnnouncementsPage`
   und beide Formulare der `BackupPage`): Sie sperrten die Server-Meldungen mit einem
   **formularweiten** Schalter, sodass ein Tastendruck in irgendeinem Feld alle Meldungen und den
   Hinweis über dem Formular verschwinden liess — auch zu Feldern, die niemand angefasst hatte.
   Gefunden hat das der unabhängige Verifikationsdurchgang.

Die Lehre für den Bericht: Die Zusage «sitzt unter allen 24 Formularen» war zweimal falsch, und
beide Male hat sie erst etwas gefunden, das **nicht** der schreibende Durchgang war.

### Offen

1. **Sieben Prüfungen an echten Geräten** führt Marco nicht durch (Entscheid 23. Oktober). Sie stehen
   im Abnahmebericht namentlich als Abweichung C-1, nicht als grün.
2. **arm64** ist nur unter Emulation geprüft (Abweichung C-2).
3. Die niedrigen Funde und Hinweise des Sicherheitsreviews bleiben für 1.x; die Reihenfolge steht am
   Ende des Berichts.
4. Der Tag `v1.0.0` und das Veröffentlichen gehören der Hauptsession und Marcos Wort. Nach dem ersten
   Release muss das Paket auf ghcr.io **einmal von Hand auf öffentlich gestellt** werden — GitHub legt
   ein neues Paket privat an.

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

## Meilenstein M9 – Schnelles Einlesen über Webhooks

Vorab geklärt mit Marco (22. Oktober 2026, Entscheidungsprotokoll Frage 10): von den Integrationen bleibt nur der
eingehende Webhook. I-1 bis I-4 und I-6 (Arr-Clients mit Polling, Beschaffungsstatus am Titel und als Suchfilter,
Benachrichtigung bei Verfügbarkeit, Bereich «Wünschen» über Seerr samt AN-5 und AN-16, SABnzbd) sind ersatzlos
gestrichen und stehen in den Non-Goals der Spec; Entscheid 5.1 ist damit gegenstandslos. Neu ist I-8, und
Entscheid 5.6 ist auf Adresse, Token, Ereignisse, Pfad-Umsetzung, Protokoll und Dashboard-Kachel ausgebaut.
Der Zuschnitt lief als eigener Pull Request voraus (#17); diese Umsetzung baut darauf auf.

**Das Prinzip.** Slouchcat ruft Radarr und Sonarr nie ab — Steuerung bleibt ein Non-Goal, und mit I-1 ist auch
das Polling entfallen. Die Verbindung ist rein eingehend: Der Admin legt eine Integration an, Slouchcat zeigt die
vollständige Adresse `POST {BasePath}/webhooks/{token}` **einmal**, der Admin trägt sie in Radarr oder Sonarr
unter «Connect → Webhook» ein. Jede Meldung endet in genau einem `scan.path`-Auftrag — demselben, den der
inotify-Watcher erzeugt (Plan 3.4), mit derselben Zusammenfassung über die Debounce-Regel der Queue (3.6). Es gibt
keinen zweiten Scan-Weg, nur einen zweiten Auslöser. Ohne eingerichteten Webhook bleibt der reguläre Abgleich die
einzige Quelle; es geht nichts verloren, es dauert nur länger (AN-12).

### Was gebaut wurde

1. **Datenhaltung und Dienst** (Migration 00017, Paket `integrations`). `webhook_integrations` hält Name, Typ,
   das Token als SHA-256 (3.8 Punkt 2), dessen letzte vier Zeichen für die Liste, die Pfad-Umsetzung als
   JSON-Präfixpaare und den letzten Kontakt; `webhook_events` das Protokoll aus Zeit, Ereignis, gemeldetem Pfad,
   umgesetztem Pfad und Ergebnis. Ein Trigger hält die letzten 20 Einträge je Integration — das Protokoll ist ein
   Fenster, keine Geschichte. Das Paket erzeugt und prüft Tokens, setzt Pfade mit dem längsten passenden Präfix um
   und löst das Ergebnis über `paths` gegen die registrierten Bibliothekswurzeln auf. Es greift **nie** auf
   `items`, `media_files` oder `tracks` zu (CLAUDE.md Regel 2, `depguard` setzt es durch); die Wirkung läuft über
   die neue Methode `library.RequestPathScan`.
2. **Endpunkt** `POST /webhooks/{token}`, `x-visibility: public`, über den generierten strict server, mit Eintrag
   im Vertragstest. `Download`, `Rename`, `MovieFileDelete`, `EpisodeFileDelete`, `MovieDelete` und `SeriesDelete`
   nehmen den Pfad aus `movie.folderPath` (Radarr) bzw. `series.path` (Sonarr) und lösen den Scan aus; `Test` wird
   mit 200 quittiert und protokolliert; jedes andere Ereignis wird mit 200 angenommen und ignoriert, damit eine
   grosszügige Auswahl in Radarr nichts kaputt macht. Falsches Token → 401 ohne Protokolleintrag (es gibt keine
   Integration, der man ihn zuschreiben könnte), Pfad ausserhalb jeder Wurzel → 422 mit beiden Pfaden im Protokoll,
   mehr als 10 Anfragen je Sekunde und Integration → 429 (`ratelimit.Allow`, das nicht blockierende Gegenstück zu
   `Wait`).
3. **Admin-Oberfläche** auf der bestehenden Integrationsseite aus M8: anlegen, Adresse einmal mit Kopier-Knopf und
   dem Hinweis «jetzt kopieren, danach nicht mehr sichtbar», danach nur die letzten vier Zeichen, Pfad-Umsetzung
   pflegen, Protokoll der letzten 20 Meldungen, «Token erneuern», löschen, letzter Kontakt. Dazu die Kachel
   «Webhooks» im Dashboard mit der Zahl der letzten 24 Stunden und einer Warnung, wenn eine Integration seit sieben
   Tagen nichts gemeldet hat — damit bleibt A-3 «Integrationsstatus» nach dem Wegfall von I-1 gefüllt. Eine heute
   angelegte Integration wird nicht gewarnt: sie ist schlicht noch nicht in Radarr eingetragen.
4. **Kanal «Beschaffung» entfernt** (Migration 00018). Sein Erzeuger war I-1; ohne ihn wäre er eine Schaltfläche
   ohne Wirkung. Weg sind `EventAcquisition`, `AcquisitionChanged`, die Texte, der Eintrag in `eventNames`, der
   Wert im Vertrag und das Kästchen in der Oberfläche. Die Migration baut `deliveries` ohne den Namen in der
   CHECK-Regel neu auf (SQLite kann keine CHECK-Regel löschen) und putzt ihn aus den gespeicherten Ereignislisten.
   Die Einstellungsseite zeigt seither vier Ereignistypen statt fünf.

Die Architekturentscheide stehen in ADR 0010 (`docs/adr/0010-webhook-empfang.md`), ADR 0009 hat einen Nachtrag
zum entfernten Kanal.

### Entscheide beim Bauen

- **Kein CSRF-Header auf der Webhook-Route.** Radarr und Sonarr sind keine Browser und senden ihn nicht. Der
  Header schützt davor, dass ein Browser eine Anfrage mit dem mitlaufenden Sitzungs-Cookie stellt; die
  Webhook-Route liest kein Cookie, ihre Autorisierung ist das Geheimnis im Pfad. Die Ausnahme steht explizit in
  `api/webhook_handlers.go` (`csrfExemptRoutes`) und im Vertrag.
- **Strikt nach Typ.** Eine Sonarr-Nutzlast an einer Radarr-Adresse hat kein `movie.folderPath` und wird mit 422
  und Protokolleintrag `invalid_payload` beantwortet, statt still auf `series.path` auszuweichen — eine falsch
  eingetragene Adresse soll auffallen.
- **Die CHECK-Regel sass woanders als erwartet.** Der Auftrag nannte `account_channels`; `acquisition` stand
  tatsächlich in der CHECK-Regel von `deliveries` (Migration 00016), während `account_channels.events` ein
  JSON-Array ohne CHECK ist. Die Migration behandelt beides.

### Belege (DoD)

| Prüfung | Befund |
|---|---|
| `task ci` | **exit 0** (22. Oktober 2026, Stand `6c6c9b4`, Protokoll `ci4.log` im Scratchpad): gen, `gen:check` ohne Drift, Lint auf beiden Seiten (golangci-lint 0 issues, itemsaccess, vet, eslint, stylelint, tsc, prettier), `go test -race ./...` über 31 Pakete, Vitest 577 in 74 Dateien (+ 4 + 2), Build, Playwright **404 von 404** in Chromium und Firefox (18.7 min), Storybook 155 Screenshots |
| Aufgezeichnete Nutzlasten (I-8) | `testdata/webhooks/`: **Radarr 6.4.4.10685** und **Sonarr 4.0.20.3014**, je sechs echte Meldungen aus Marcos Installation. `TestWebhookHandlesTheRecordedRadarrAndSonarrPayloads` schickt jede über die echte Route und prüft Ergebnis, gescannten Ordner, Protokolleintrag und dass der Dateiname zum `eventType` passt |
| Vertragstest 10.2 | `go test ./internal/api/ -run TestContract` grün; `receiveWebhook` im `publicRegistry`, die sechs Admin-Operationen im `adminRegistry` |
| Paket `integrations` | `go test ./internal/integrations/` grün (Token, Pfad-Umsetzung, Auflösung, Empfänger, Ratenbegrenzung, letzter Kontakt) |
| Kein Token im Log (A-9) | `go test ./internal/logging/` grün; die Redaction maskiert `/webhooks/<token>` in jedem String, der gemeldete Dateipfad bleibt lesbar |
| Migration 00018 | `go test ./internal/store/ -run TestMigration00018` grün: Ereignis aus CHECK-Regel und JSON-Listen weg, andere Zustellungen und die Indexe erhalten |
| Eingefrorener Stand | `testdata/db/M9.sql`, `TestFrozenM9StateCoversM9Data` und `TestFrozenStatesMigrate` grün |
| Vier statt fünf Ereignistypen | `ChannelSettingsPage.test.tsx` und `notification-channels.spec.ts` zählen vier Kästchen und finden «Beschaffungsstatus» nicht mehr |
| Tastaturtest | `web/e2e/admin-webhooks.spec.ts`: anlegen, Adresse einmal, Pfad-Umsetzung, Erneuern und Löschen mit Rückfrage, Dashboard-Kachel — alles über die Tastatur, mit sichtbarem Fokus und ohne Fokusfalle |
| Import → sichtbar ohne Voll-Scan | `docs/perf/M9.md` 1: Platzhalterbaum (60'800 Dateien), neuer Ordner gemeldet → Titel nach **3.1 s** auffindbar, **0** `scan.library`-Jobs; der Voll-Scan derselben Bibliothek dauert 28.4 s |
| 11 Anfragen in einer Sekunde → 429 | `docs/perf/M9.md` 3: 10× 200, die elfte 429 mit `Retry-After: 1` |
| Antwortzeit des Endpunkts | `docs/perf/M9.md` 2: p50 2.3 ms, p95 25.2 ms über 200 Anfragen |

### Review und Verifikation (eigene Durchgänge, nicht im Schreibkontext)

Beide Durchgänge liefen als eigene Agenten gegen den fertigen Branch. Die Review meldete keinen kritischen und
keinen hohen Befund; behoben wurden daraus:

1. **`under()` mit der Quelle `/`** gab für einen *relativen* gemeldeten Pfad «liegt darunter» zurück, ohne ihn
   anzusehen. Mit einer `/`-Umsetzung konnte ein gebastelter relativer `folderPath` den Scan auf eine andere
   Bibliothek lenken — aus den Wurzeln ausbrechen ging nie, `Resolve` verbietet das. Beide Seiten des Vergleichs
   müssen jetzt absolut sein; `TestMapNeverMatchesARelativeReport` hält es fest.
2. **`scan_queued` ohne eingereihten Auftrag.** `RequestPathScan` gab nichts zurück und `flushPathScan` schluckt
   jeden Fehler, also behauptete das Protokoll bei abgebrochener Verbindung einen Scan, der nie lief.
   `RequestPathScan` meldet jetzt, ob die Ordner einen eingereihten Auftrag oder einen geplanten Wiederholversuch
   erreicht haben; nur dann entsteht der Eintrag, sonst antwortet der Server 500 und der Arr-Dienst wiederholt.
   Der Eintrag selbst überlebt den Abbruch des Anrufers (`context.WithoutCancel`).
3. Kleineres: die Redaction-Schranken waren gross-/kleinschreibungsabhängig, während jedes Muster dahinter es
   nicht ist (ein `/WEBHOOKS/<token>` auf einer 404 wäre unredigiert ins Log gewandert); ein `eventType` über
   80 Zeichen verlor seinen Protokolleintrag an die CHECK-Regel, statt gekürzt zu werden; die Dashboard-Zahl
   zählt Protokollzeilen, von denen je Integration 20 bleiben, versprach im Text aber eine vollständige Zahl.

Aus der Verifikation kamen dazu: der Tastaturtest **behauptete** Pfeiltasten über dem Ereignisprotokoll, drückte
aber keine (das Protokoll ist eine reine Lesetabelle ohne Bedienelemente — jetzt prüft der Test die Pfeiltasten
an der Auswahl Radarr/Sonarr, belegt die fehlenden Bedienelemente und geht mit dem Browser zurück); «422 mit
beiden Pfaden» war nur mit identischen Pfaden getestet; `EpisodeFileDelete` und `SeriesDelete` waren nie
ausgeführt; `ratelimit.Allow` hatte keinen Test im eigenen Paket.

### Offen

1. **Bewusst nicht gebaut: Windows-Pfade in der Umsetzungstabelle.** Umsetzung und Auflösung rechnen mit
   Schrägstrichen; Slouchcat läuft laut Spec nur auf Linux, und Marcos Dienste melden `/mnt/medien/…`. Die
   Annahme steht in ADR 0010, und die 422-Meldung nennt sie, statt auf etwas Unmögliches zu verweisen.
2. **Das Formular der Pfad-Umsetzung zieht nicht nach**, wenn ein zweiter Admin dieselbe Integration gleichzeitig
   ändert; der eigene Speicherweg ist abgedeckt. Kleinigkeit für M10.
3. **Einmaliger Vitest-Aussetzer unter Last:** `react-aria` meldete in `LoginPage.test.tsx` einen Timer nach dem
   Abbau der JSDOM-Umgebung («document is not defined»). Die Datei gehört nicht zu M9; sechs weitere Vollläufe
   blieben sauber. Dieselbe Ursachenfamilie wie die flackernden Web-Tests aus #20 (eine lazy geladene Seite
   bezahlt die Übersetzung im `findBy`-Fenster, zentral in `web/src/test/setup.ts` behoben) — ob die Korrektur
   auch diesen Fall erwischt, zeigt der nächste Lauf.
4. **Android und O-6** aus M5 bis M8 bleiben offen; der Langzeittest läuft (Tag 30 am 19. November).
5. **M10** bringt Härtung und Release 1.0.

## Abnahme durch Marco am 23. Oktober 2026 (M8 und M9 am laufenden Container)

Geprüft am Stand `a314b4d` mit dem neu gebauten Image.

| Prüfung | Ergebnis |
|---|---|
| **E-Mail (A-6)** | läuft; Testnachricht über sein SMTP-Konto zugestellt |
| **Grafikchip im Assistenten (A-1)** | meldete zunächst «Kein Grafikchip erkannt» — **richtig**, denn `deploy/compose.yaml` reicht ohne Override keine GPU durch. Seit dem 23. Oktober lädt `COMPOSE_FILE=compose.yaml:compose.nvidia.yaml` in `deploy/.env` die NVIDIA-Datei immer mit; der Container sieht nun die RTX 4070 (Treiber 595.99.02), `transcode backend detected backend=nvenc tonemapper=cuda`, Budget 12 statt 4 |
| **Webhook-Integration (I-8)** | Verbindung von Radarr zu Slouchcat steht; ob eine Meldung bis zum Pfad-Scan durchläuft, zeigt das Ereignisprotokoll und ist noch nicht bestätigt |

**Noch offen bei Marco:** Web-Push am Android-Handy, QSV-Erkennung auf CT 190, der zweite Teil des
Webhook-Tests (Ereignisprotokoll nach einem echten Import, inklusive der Pfad-Umsetzung von `/mnt/medien/…`
auf die Bibliothekswurzel), HTTPS mit der echten Domain (K-8) und die Android-Checklisten aus M5 bis M8.

**Mangel, gefunden bei dieser Abnahme (für M10):** Die Feldfehler des Servers bleiben im Formular stehen, bis
erneut abgesendet wird — sie verschwinden nicht, wenn der Wert korrigiert wird. Beim Einrichten des
E-Mail-Servers sah es dadurch so aus, als seien gültige Eingaben ungültig. Betrifft alle Formulare, die
`validationErrors` von react-aria aus einer Server-Antwort füttern (`IntegrationsPage`, und dieselbe Stelle in
`ChannelSettingsPage` und `WebhooksSection`).

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

1. **Echte Zugangsdaten: am 23. Oktober 2026 von Marco geprüft.** E-Mail läuft mit seinem SMTP-Konto
   (`mail.nerdboxmail.com:587`, STARTTLS, Zugangsdaten in `deploy/.env`). Web-Push am Android-Handy und die
   QSV-Erkennung auf CT 190 stehen noch aus.
2. **Beschaffungsstatus** (`acquisition`) hatte Modell, Kanalwahl und Hook, aber nie einen Erzeuger; mit dem
   Zuschnitt von M9 ist der Kanal am 22. Oktober 2026 ersatzlos entfernt worden (siehe M9, Schritt 4).
3. **Release-Manifest:** die Update-Prüfung liest `https://github.com/SlouchCat/slouchcat/releases/latest/download/manifest.json`;
   das Veröffentlichen gehört zu M10.
4. **Verdichtung alter `progress_events`** (Plan 3.2, 90 Tage) ist nicht gebaut; die Statistik liest Rohdaten.
5. **Android:** Gerätecheckliste `docs/perf/M8.md` 6 und die offenen Punkte aus M5 bis M7. Der O-6-Langzeittest
   ist am 23. Oktober 2026 bei Tag 3 abgeschlossen worden (`docs/perf/M5-o6-langzeittest.md`).
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
