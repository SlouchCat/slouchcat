# Messwerte M1a – Durchstich Teil 1: Scan, Direct Play, Startseite

Stand: 18. Oktober 2026, Branch `claude/m1a-scan-direct-play-home`.

Hardware: Entwicklungsrechner Linux Mint 22.3 (Kernel 7.0), Intel i7-14700K (28 Threads), 31 GB RAM,
NVMe (ext4). NAS: 192.168.1.50 über NFS v4.2 (Gigabit-LAN). Go 1.27.2, ffprobe 6.1 (Host).

## 1. Dateiüberwachung auf dem Platzhalterbaum (Plan 3.4, Entscheid 3)

Baum aus `task testdata:tree` (Standardwerte): 60'800 leere Dateien in 8'213 Ordnern (2'500 Filme,
40'000 Folgen, 334 Specials, 1'808 Trailer, 3'361 Extras, 12'797 Untertitel) auf der lokalen NVMe.
Messung mit `server/bin/slouchcat watchbench --root <baum> --passes 5 --inotify`:

| Messung | Wert | Grenzwert |
|---|---|---|
| Stat-Durchlauf des Pollers (alle Ordner lesen, Listen vergleichen), 5 Läufe | 193 / 128 / 141 / 128 / 136 ms | < 15 s je 10'000 Dateien (Plan 3.4) |
| Adaptives Intervall daraus (`max(45 s, 3 × Dauer)`) | 45 s | |
| inotify: 8'213 Ordner registrieren | 401 ms | |
| `fs.inotify.max_user_watches` auf dem Rechner | 256'774 (Belegung 3.2 %, Warnung ab 80 %) | |
| Neue Datei per inotify gemeldet (inkl. 2 s Entprellung) | 2.0 s | < 60 s (B-4) |

Auf Marcos NFS-Freigabe `/mnt/nerdnas/medien/Testfilme` (13 Ordner, 45 Dateien): erster Durchlauf
5.9 ms, danach 2.6–2.8 ms (Messung des Scanner-Agenten). Der Durchlauf in Referenzgrösse **über NFS/SMB**
steht noch aus: dafür muss der Platzhalterbaum auf die Freigabe geschrieben werden (60'800 Dateien auf
dem NAS, nach Rücksprache), oder er läuft auf dem Proxmox-Testcontainer (Entscheid 2, vor M1b).

## 2. Scanner

| Messung | Wert |
|---|---|
| Voll-Scan `Movies/` des Platzhalterbaums (2'500 Filmordner, Trailer/Extras/Untertitel übersprungen) | 1.09 s, 2'500 Titel und 2'500 Dateien angelegt, 2 Worker |
| Voll-Scan der echten Bibliothek (12 Filme auf NFS, Radarr-Benennung mit NFO) | 0.47 s |
| Zeit vom Assistenten bis alle 12 Titel mit TMDB-Texten (de/en), FSK und Poster/Backdrop | rund 4 s (Scan 22:09:28.4, letztes `image.fetch` 22:09:31.2) |
| Namenskorpus | 184 Pfadmuster, 184 exakt (100 %), Schwelle ≥ 98 % |
| NFO-Korpus | 30 Fälle |

Probe (`ffprobe`) läuft als eigener Job je Datei; der Keyframe-Index (`probe.keyframes`) liest die
ganze Datei (nur Demux) und läuft darum mit niedrigster Priorität nach den Metadaten. Bei leeren
Platzhalterdateien schlägt die Probe erwartungsgemäss fehl (`EBML header parsing failed`) und wird nach
Backoff bis zu fünfmal wiederholt; Titel und Poster hängen nicht davon ab.

## 2a. Frischer Stack per Docker Compose (DoD M1a)

Image `slouchcat:dev` (`task deploy:build`), `docker compose -f compose.yaml -f .dev/compose.m1a.yaml up -d`
mit leerem `config/` und `cache/`: 20 generierte Filme (Kopien der H.264-Testdatei unter echten Titeln wie
`The Matrix (1999)`) unter `/media/Generated` und Marcos 12 Testfilme (NFS) unter `/testfilme`.

| Messung | Wert | Ziel |
|---|---|---|
| Container `healthy` | 1.62 s | < 5 s |
| Assistent (`POST /setup` 22:38:45.1) bis alle 32 `image.fetch`-Jobs fertig (22:38:54.7) | **9.5 s** | < 5 min |
| Scan 20 generierte Filme (lokal) / 12 eigene (NFS) | 22 ms / 0.83 s | |
| Warnungen oder Fehler im Container-Log | keine | |

Alle 32 Titel haben TMDB-Texte, FSK-Stufe, Poster und Backdrop.

## 3. Direct Play

| Messung | Wert |
|---|---|
| Range-Anfrage `bytes=0-99` auf eine 10.5-GB-MKV über NFS | HTTP 206, `Content-Range: bytes 0-99/10555016079`, `Cache-Control: private, no-store` |
| ohne Sitzungs-Cookie | HTTP 401 |
| Startzeit Navigation → erstes `playing` (H.264/AAC-MP4, 10 s Testclip, Playwright `data-start-ms`, Server auf localhost) | Chromium 43 ms, Firefox 73 ms (Ziel < 2 s); Safari ungeprüft (keine Hardware, Entscheid 2) |

## 4. Startseite

Lighthouse 12.8.2, Mobile-Voreinstellung, simuliert (1.6 Mbit/s, 150 ms RTT, 4× CPU), Startseite mit
12 Titeln (TMDB-Poster w500 50–190 kB, Backdrops w780 58–120 kB bzw. w1280 140–400 kB), Server auf
127.0.0.1 (HTTP/1.1, gzip), angemeldet (Cookie als Kopfzeile). Median aus drei Läufen.

| Messung | vorher | Runde 1 | Runde 2 | Runde 3 |
|---|---|---|---|---|
| LCP | 13.0 s | 3.4 s (3.38 / 3.38 / 3.38 s) | 2.57 s (2.568 / 2.570 / 2.568 s) | 2.55 s (2.552 / 2.551 / 2.459 s) |
| FCP | 4.4 s | 2.25 s | 1.70 s | 1.65 s (1.652 / 1.651 / 1.709 s) |
| TBT / CLS | 32 ms / 0 | 7 ms / 0 | 4 ms / 0 | 0–1 ms / 0 |
| LCP-Phasen (TTFB / Ladeverzug / Ladezeit / Renderverzug) | 0.45 / 10.6 / 0.5 / 1.4 s | 0.45 / 2.25 / 0.02 / 0.66 s | 0.45 / 0–0.14 / 0–0.24 / 1.74–2.12 s | 0.45 / 0.75 / 0.32 / 1.03 s; 0.45 / 0.54 / 0.35 / 1.22 s; 0.45 / 0.58 / 0.53 / 0.89 s |
| JS beim Start (gzip) | 591 kB ohne Kompression | 190 kB (index 105 kB, ui 85 kB), 3 Dateien | 156 kB, 1 Datei (Übertragung 158 KiB) | 139 kB, 1 Datei (Übertragung 141 KiB); norigin 16.6 kB als eigene Datei nach dem LCP |
| Anfragen vor dem LCP | – | 11 | 7 (Dokument, Sitzung, Startseite, Hero-Bild, JS, CSS, Manifest) | 7 (unverändert) |
| LCP-Element | aktives Hero-`<img>` w1280 | aktives Hero-`<img>`, w780 über `srcset` | unverändert | `#hero-prepaint img` aus der App-Hülle (w780 über `srcset`) |

Runde 2 (Läufe `lighthouse-home-3-run{1,2,3}.json`):

- **Vorladen durch den Server.** `GET /` mit gültiger Sitzung trägt `Link: rel=preload` für
  `/api/v1/auth/session` und `/api/v1/home` (`as=fetch; crossorigin`) und für das erste Karussellbild der
  Empfehlungen des Profils (`as=image`, gleiche `imagesrcset`/`imagesizes` wie das Karussell,
  `fetchpriority=high`; `api/preload.go`, `discover.Service.FirstPick` über die Sichtbarkeitsschicht).
  Lesende API-Aufrufe gehen ohne den CSRF-Kopf, damit der Browser die vorgeladenen Antworten nimmt
  (geprüft: je eine Anfrage für Sitzung und Startseite). Damit beginnen Bild und Daten mit dem JS statt
  danach: der Ladeverzug fällt von 2.25 s auf 0.
- **Kein react-aria auf dem Startpfad.** Stöberseiten nutzen schlichte `<button>`/`<a>` (`@/ui`),
  Formulare, Dialoge und Player `@/ui/aria`; die Anmeldung wird bei Bedarf geladen. −32 kB gzip.
- **Eine JS- und eine CSS-Datei** für alles, was die Startseite statisch lädt (rolldown
  `codeSplitting`-Gruppe `$initial`), SW-Registrierung inline statt als blockierendes `registerSW.js`
  (FCP −0.3 s).
- Das zweite Karussellbild startet erst, wenn das erste gezeichnet ist (es ist vorgeladen und sonst vor
  dem Zeichnen geladen).

Geprüft und verworfen: Zeilen einen Frame nach dem Hero rendern (LCP unverändert, CPU ist nicht der
Engpass), react-dom als eigene parallele Datei (2.70 s), terser statt oxc (+0.9 kB), nur die aktive
Sprache laden (Lighthouse-Browser `en`, Profil `de`: beide nötig, ein Nachladen kostete eine Anfrage),
lodash-es von norigin ersetzen (−7.6 kB gzip, siehe unten: unter der Schwelle).

**Stand nach Runde 2: P-10 (< 2.5 s) knapp verfehlt (2.57 s).** Laut Lantern-Simulation (Trace dieses Stands) endet das
Dokument bei 0.60 s; danach teilen sich JS (158 KiB) und Hero-Bild (114 KiB) die Leitung: Bild fertig bei
2.25 s, JS bei 2.55 s, der LCP folgt dem JS-Ende nach rund 20 ms CPU. Dominant ist also die Ladezeit des
Start-JS im Wettbewerb mit dem Bild (im Lighthouse-Bericht als „Renderverzug“ ausgewiesen). Lantern
rechnet in RTT-Stufen: erst rund 30 kB weniger JS (oder 40 kB weniger Bild) senken den LCP um eine
Stufe auf etwa 2.42 s; 7–27 kB weniger ändern nichts. Nächster Schritt: das erste Hero-Bild serverseitig
in die App-Hülle von `/` schreiben (ein `<img>` mit denselben Attributen, das React übernimmt), damit der
LCP nicht mehr auf das JS wartet; die Simulation erwartet dann ≈ 2.25 s (Bildende). Hinter Caddy mit
HTTPS (K-8) kommen HTTP/2 und Brotli hinzu (Chrome sendet `br` nur über HTTPS), was das JS um rund
20 % verkleinert; das ist hier im HTTP-Labor nicht messbar.

Runde 3 (Läufe `lighthouse-home-4-run{1,2,3}.json`):

- **Hero-Bild in der App-Hülle.** `GET /` mit Sitzung und erster Empfehlung ersetzt den Marker
  `<!--hero-->` in `index.html` (vor `#root`) durch `<div id="hero-prepaint" aria-hidden="true"><img …>`
  mit denselben Attributen wie das aktive Karussellbild (URLs, `srcset`, `sizes`, `fetchpriority=high`,
  `decoding=sync`; Werte HTML-maskiert; `api/preload.go`). Ohne Sitzung oder Bild fällt der Marker weg.
  `#hero-prepaint` in `global.css` hat die Geometrie des Karussellbilds (Höhe
  `clamp(hero-min, 78vh, hero-max)`, `object-fit: cover`, `object-position: center 25%`), liegt über
  der Seite und unter der Kopfzeile (`z-raised`, `pointer-events: none`) und wird ausgeblendet
  (`motion.duration.crossfade`) und entfernt, sobald das Karussell sein eigenes Bild gezeichnet hat, die
  Seite ohne Hero-Bild auskommt oder der Router `/` verlässt. Geprüft mit Playwright (412 und 1440 px
  breit): gleiches Rechteck und gleiche `currentSrc` wie das Karussellbild, danach entfernt; CLS 0.
  Im Browser ist das Bild damit nach 29–67 ms gezeichnet statt nach dem JS (vorher 143 ms).
- **norigin als eigene Datei.** `@noriginmedia/norigin-spatial-navigation` mit lodash-es (16.6 kB gzip)
  lädt per `import()`, sobald das Start-JS läuft; die Routen mit Kopfzeile (`_chrome`) warten darauf
  (`loadSpatialNavigation()` in `ui/spatial.tsx`). Die Anfrage beginnt nach dem LCP und zählt deshalb
  nicht mehr dazu. Start-JS 156 → 139 kB gzip. Kosten: Kopfzeile und Seiteninhalt erscheinen eine
  Anfrage später; das Hero-Bild steht dann schon.
- Geprüft: `@tanstack/react-query-devtools` ist nicht im Bundle. Verworfen: die inaktive Sprache
  nachladen (≈ 2.5 kB, siehe Runde 2), Hero-Bild vor Sitzung und Startseite im `Link`-Kopf (Chrome
  fordert das Bild trotzdem nach der Sitzung an; 2.445 / 2.551 / 2.552 s).

Zwischenstände: nur Hero-Bild in der Hülle 2.61 / 2.55 / 2.60 s (`decoding=async`) bzw. 2.60 / 2.55 /
2.60 s (`decoding=sync`), also ohne Wirkung auf den simulierten Wert; norigin testweise ganz entfernt
2.447 / 2.456 / 2.458 s; drei weitere Läufe des Endstands (`lighthouse-home-4x-run*`) 2.551 / 2.444 /
2.444 s. Über alle sechs Läufe des Endstands: dreimal ≈ 2.45 s, dreimal 2.55 s.

**P-10 (< 2.5 s) im Median weiterhin knapp verfehlt (2.55 s), Werte pendeln zwischen 2.44 und 2.55 s.**
Warum das Hero-Bild in der Hülle den simulierten Wert kaum bewegt: Lantern baut den LCP-Graphen aus
allen Anfragen, die im (ungedrosselten) Beobachtungslauf *vor* dem gezeichneten LCP fertig waren. Auf
127.0.0.1 ist das Start-JS nach 15–47 ms geladen, das Bild nach 29–67 ms gezeichnet; das JS bleibt also im
Graphen, und der simulierte LCP endet weiter mit dem JS-Download (Bild und JS teilen sich die Leitung).
Ein Modul-Skript erzeugt zudem kein `EvaluateScript`-Ereignis, mit dem Lantern es als „nicht
renderblockierend“ aussortieren könnte. In einem echten gedrosselten Browser zeichnet die Hülle das Bild,
sobald es geladen ist (laut Simulation bei ≈ 2.25 s), unabhängig vom JS. Gewirkt hat im simulierten Wert
nur die kleinere Start-Datei (−17 kB): Sie bringt den LCP genau an die Stufe, die je nach CPU-Zeiten des
Beobachtungslaufs (FCP 1.65 s ↔ LCP 2.55 s, FCP 1.70 s ↔ LCP 2.45 s) mal darunter, mal darüber liegt.
Ein sicherer Abstand bräuchte nochmals rund 15–30 kB weniger Start-JS, oder das Start-JS erst nach dem
Hero-Bild anzufordern (nur auf `/` mit Hülle; verschiebt die Interaktivität um etwa eine Anfrage). Beides
ist nicht umgesetzt.

## 5. Leerlauf

Server mit 12 Titeln nach abgeschlossenen Jobs: folgt (Ziel P-2 < 150 MB bei 5'000 Titeln, in M1a nur
Richtwert).
