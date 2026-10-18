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

## 3. Direct Play

| Messung | Wert |
|---|---|
| Range-Anfrage `bytes=0-99` auf eine 10.5-GB-MKV über NFS | HTTP 206, `Content-Range: bytes 0-99/10555016079`, `Cache-Control: private, no-store` |
| ohne Sitzungs-Cookie | HTTP 401 |
| Startzeit bis `playing` in Chrome und Firefox (H.264/AAC-MP4) | folgt (Playwright-Messung `data-start-ms`) |

## 4. Startseite

| Messung | Wert |
|---|---|
| Lighthouse Mobile LCP | folgt |

## 5. Leerlauf

Server mit 12 Titeln nach abgeschlossenen Jobs: folgt (Ziel P-2 < 150 MB bei 5'000 Titeln, in M1a nur
Richtwert).
