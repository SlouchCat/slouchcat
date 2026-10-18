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
127.0.0.1, angemeldet. Median aus drei Läufen.

| Messung | vorher | nachher |
|---|---|---|
| LCP | 13.0 s | 3.4 s (3.38 / 3.38 / 3.38 s) |
| FCP | 4.4 s | 2.25 s |
| TBT / CLS | 32 ms / 0 | 7 ms / 0 |
| LCP-Phasen (TTFB / Ladeverzug / Ladezeit / Renderverzug) | 0.45 / 10.6 / 0.5 / 1.4 s | 0.45 / 2.25 / 0.02 / 0.66 s |
| JS beim Start (gzip) | 591 kB ohne Kompression | 190 kB (index 105 kB, ui 85 kB) |
| LCP-Element | aktives Hero-`<img>` w1280 | aktives Hero-`<img>`, w780 über `srcset` |

Massnahmen: gzip für App und JSON, `/auth/session` und `/home` parallel (`/setup` nur ohne Sitzung),
Backdrop w780 für Telefone, nur das aktive Karussellbild sofort, Poster erst nach dem gezeichneten
Hero-Bild. Ziel P-10 (< 2.5 s) noch nicht erreicht: Der Ladeverzug ist die Kette HTML → 190 kB JS →
Ausführung (4× CPU) → `/home` → Bild. Ein Prototyp mit `Link: rel=preload` für das Hero-Bild im Antwortkopf
von `/` (Server kennt Sitzung und Startseite) ergab 2.9 s; den Rest bestimmt die JS-Menge und -Ausführung.

## 5. Leerlauf

Server mit 12 Titeln nach abgeschlossenen Jobs: folgt (Ziel P-2 < 150 MB bei 5'000 Titeln, in M1a nur
Richtwert).
