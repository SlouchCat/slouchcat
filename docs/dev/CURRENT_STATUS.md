# Aktueller Stand

Stand: 19. Oktober 2026. M0 ist in `main` gemerged und nach `https://github.com/SlouchCat/slouchcat`
gepusht. M1a liegt auf dem Branch `claude/m1a-scan-direct-play-home` (nach origin gepusht, nicht gemerged;
Merge in `main` und Pull Request nur auf Marcos Wort).

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
| B-4 ext4: neu/umbenannt/gelöscht sichtbar < 60 s; Netzfreigaben nach AN-12 mit Messwert | inotify meldet in 2 s (inkl. Entprellung); **NFS in Referenzgrösse** (60'800 Dateien auf dem NAS): Stat-Durchlauf 3.0–10.9 s, Intervall 45 s, neuer Titel nach 48.4 s sichtbar; Voll-Scan 2'500 Filme über NFS 4.75 s | `slouchcat watchbench`, `docs/perf/M1a.md` Abschnitte 1 und 1a |
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
task ci                      # alles bauen und prüfen (inkl. Playwright)
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

**Noch ausstehend:** Android-Lauf des Spikes (Protokoll in `docs/perf/M1-ios-offline.md`); Lastmessung
über SMB (NFS erledigt).

## Offene Punkte für M1b

1. Lastmessung in Referenzgrösse **über NFS/SMB** (Platzhalterbaum auf der Freigabe oder Proxmox-CT,
   Entscheid 2/3); lokal erfüllt.
2. Android-Lauf des Spikes durch Marco (Download mit Sperre, Spulen, Bestand nach 1/3/8 Tagen); Auswertung
   und Go/No-Go in M1b.
3. Stream-Token (5.4), Bildpipeline (WebP, Grössen, Akzentfarbe), HLS/Remux, Design-Feinschliff, arm64.
4. `inotify_poll`-Abweichungen und Scan-Fehler erscheinen erst mit dem Dashboard (M8) in der Oberfläche;
   bis dahin Log und `/admin/libraries`.
5. Proxmox-Testcontainer als schwaches NAS anlegen (Entscheid 2, vor M1b).
6. GitHub Actions: Workflow `.github/workflows/ci.yml` läuft seit dem Push dieses Branches; Ergebnis auf
   GitHub prüfen (der Lauf braucht ffmpeg, Playwright-Browser und `task testdata:media`, alles im Workflow).
