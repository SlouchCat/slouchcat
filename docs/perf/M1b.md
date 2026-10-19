# Messwerte M1b – Durchstich Teil 2: HLS, Seeking, Design

Stand: 19. Oktober 2026, Branch `claude/vigorous-lumiere-17beb0`. Grundlage: Plan `docs/planning/03-plan-v1.md`
Abschnitte 3.1, 3.5, 5.4, M1b; Entscheid 4 (AN-17) in `docs/planning/04-entscheidungen.md`.

Hardware: Entwicklungsrechner Linux Mint 22.3, 28 Kerne, 31 GB RAM, NVIDIA RTX 4070 (Treiber 595.99,
NVENC/NVDEC), ffmpeg 6.1.1 (Ubuntu) für den Host-Binary; Docker-Image mit jellyfin-ffmpeg 7.1.
Testserver: Proxmox-CT 190 `slouchcat-nas` auf `pve2` (2 Kerne, 4 GB, Intel UHD 630 durchgereicht), siehe
`docs/dev/TESTSERVER.md`.

## 1. HLS-Start und Spulen (Playwright, Schritt 1)

Messung 19. Oktober 2026 auf dem Entwicklungsrechner mit `web/e2e/hls.spec.ts` (Teil von `task e2e`,
Chromium 156 und Firefox 157 von Playwright, beide über hls.js 1.7.3, siehe ADR 0004) gegen
`server/bin/slouchcat` auf localhost. Testclips aus `testdata/generate-media.sh`, je 90 s, 1280×720:
«Slouchcat Test HEVC Long» (HEVC Main 10, x265 ultrafast, GOP 48, AAC) und «Slouchcat Test H264 MKV Long»
(H.264/AAC in MKV, Keyframes nur bei 0, 2, 7, 8, 15, 21, 30, 31, 45, 60, 61, 75 s, damit die
Remux-Segmentgrenzen dem Keyframe-Index folgen).

- **Start** = `data-start-ms`: vom Öffnen der Player-Route bis zum ersten `playing` (enthält
  `POST /items/{id}/play`, Start von ffmpeg, Master-, Media-Playlist, `init.mp4`, erstes Segment).
- **Spulen** = `data-seek-ms`: vom `seeking` des Elements bis es wieder spielt (`playing`, oder das erste
  `timeupdate` nach `seeked`). Gesprungen wird per `video.currentTime` auf 20, 50 und 80 % der Dauer.
  20 % (18 s) liegt meist noch im Puffer von hls.js (30 s voraus); 50 % und 80 % liegen ausserhalb und
  erzwingen beim Transcode einen Neustart von ffmpeg am Zielsegment.
- Der Transcode-Fall meldet nur H.264 als Videocodec: Firefox 157 meldet auf diesem Rechner HEVC Main 10
  (System-ffmpeg) und bekäme sonst Remux (ADR 0004). Ohne diese Einschränkung spielt Firefox die
  HEVC-Datei als Remux: Start 87 ms, Spulen 34 / 19 / 31 ms.
- Budget (DoD M1b): Start < 5 s, Spulen < 3 s. Die Specs schlagen bei Überschreitung fehl.

`transcode.Detect` wählt auf dem Entwicklungsrechner **NVENC** (`SLOUCHCAT_HW=auto`); der Software-Weg
(libx264 veryfast) wurde mit `SLOUCHCAT_HW=software` gemessen. Zwei Läufe je Backend:

| Browser | Fall | Backend | Start (ms) | Spulen 20 % / 50 % / 80 % (ms) |
|---|---|---|---|---|
| Chromium | HEVC → Transcode | NVENC | 373 / 376 | 75 / 304 / 157 · 76 / 299 / 151 |
| Chromium | HEVC → Transcode | Software | 371 / 331 | 91 / 295 / 249 · 74 / 308 / 243 |
| Chromium | H.264-MKV → Remux | – | 83 / 84 (151 / 90 im Software-Lauf) | 58 / 254 / 198 · 55 / 243 / 194 |
| Firefox | HEVC → Transcode | NVENC | 284 / 338 | 226 / 336 / 78 · 43 / 231 / 397 |
| Firefox | HEVC → Transcode | Software | 337 / 288 | 406 / 286 / 319 · 438 / 305 / 143 |
| Firefox | H.264-MKV → Remux | – | 97 / 87 (85 / 93 im Software-Lauf) | 33 / 98 / 73 · 30 / 118 / 85 |

Alle Werte liegen weit unter dem Budget; der langsamste Sprung war 438 ms (Firefox, Software, 20 %).

**1080p, echter Film (serverseitig).** Die DoD nennt 1080p; die Testclips sind 720p, damit sie in CI schnell
entstehen. Deshalb zusätzlich «Dangerous Animals (2025)» (HEVC Main 10, 1920×1080, 5941 s, 8.8 Mbit/s, über
NFS vom NAS) mit einem Skript, das wie hls.js abruft: `POST /items/{id}/play` (Fähigkeiten nur H.264/AAC),
Master- und Media-Playlist, `init.mp4`, Segment 0; danach je zwei Segmente ab 20, 50 und 80 % (1486
Segmente zu 4 s). Gemessen mit `curl` auf localhost, also ohne Puffern und Dekodieren im Browser:

| Backend | Erstes Segment ab `POST /play` (ms) | Zwei Segmente ab 20 % / 50 % / 80 % (ms) |
|---|---|---|
| NVENC | 362 | 659 / 608 / 557 |
| Software (libx264 veryfast, 28 Kerne) | 514 | 808 / 911 / 960 |

Damit ist auch bei 1080p genug Abstand zu 5 s (Start) und 3 s (Spulen); ein Firefox-Lauf mit dem 1080p-Film im Browser selbst fehlt noch (nur serverseitig und im eingebetteten Chromium gemessen). Auf dem NAS (QSV, 2 Kerne) folgt die
Messung mit der Kalibrierung in Abschnitt 3; Safari ist ungeprüft (keine Hardware).

## 2. Stream-Token (Schritt 2)

Umsetzung nach Plan 5.4: `st = <Wiedergabesitzungs-ID>.<base64url(HMAC-SHA256(Schlüssel, ID + "\n" +
Item-ID))>`, Schlüssel per HKDF-SHA256 aus `/config/secret.key` abgeleitet
(`secrets.Keyring.Derive("stream-token")`), nur im Speicher. Kein Ablauf im Token; Ablauf und Widerruf
stehen in `playback_sessions` (Migration 00005): `expires_at` = letzte Aktivität + 12 h (höchstens einmal
pro Minute fortgeschrieben), `revoked_at` bei `DELETE /playback/sessions/{id}` und beim Logout. Jede
Playlist-, Init-, Segment- und Range-Anfrage liest die Zeile per Primärschlüssel über den Lese-Pool (mit
der Browser-Sitzung verknüpft: ein abgemeldetes Konto widerruft alle seine Tokens), prüft das HMAC in
konstanter Zeit und danach den Titel über `visibility` (nicht mehr sichtbar → 404). Alle Antworten tragen
`Referrer-Policy: no-referrer`; `st` wird im Log redigiert (auch in Playlists, die in Fehlermeldungen
stehen, `internal/logging`-Test).

Messung mit `web/e2e/stream-token.spec.ts` (Chromium und Firefox, je ein Lauf in `task e2e`): Ein
Request-Kontext **ohne** Sitzungs-Cookie ruft Master-, Media-Playlist, `init.mp4` und `seg/0` mit dem Token ab
(200); ohne `st` antworten dieselben URLs 401 `unauthenticated`, auch mit Cookie. Nach `DELETE
/playback/sessions/{id}` (204) wird dieselbe Media-Playlist-URL sofort erneut abgerufen; gemessen wird
`Date.now()` vom Ende des `DELETE` bis zur Antwort:

| Browser-Projekt | Antwort nach Widerruf | Zeit bis zur 401 (ms) |
|---|---|---|
| Chromium | 401 | 0 / 2 |
| Firefox | 401 | 1 / 1 |

Budget (DoD M1b): 401 innert 1 s. Der Widerruf wirkt ab der nächsten Anfrage, weil es keinen Cache der
Token-Prüfung gibt. Der Direct-Play-Stream (`/items/{id}/stream?st=…`) antwortet mit Token 206, ohne Token
401 und nach dem Widerruf 401. Die Go-Tests (`internal/playback`, `internal/api`) prüfen zusätzlich
gefälschte Tokens, Tokens eines anderen Titels oder einer anderen Wiedergabesitzung (404), Ablauf nach 12 h
ohne Aktivität, den Logout und dass nur die eigene Browser-Sitzung eine Wiedergabe beenden kann.

## 3. Admission Control: Kalibrierung (Schritt 3)

Messung 19. Oktober 2026 mit `slouchcat hlsbench` (`server/cmd/slouchcat/hlsbench.go`): N Sitzungen
starten gleichzeitig auf derselben Datei und lesen die Segmente wie ein Player (Segment i muss i × 4 s
nach dem Start bereit sein). Je Lauf 45 s Inhalt, `--budget 1000` (Admission Control ausgeschaltet, damit
die Hardware und nicht das Budget die Grenze setzt). Gemeldet werden die Zeit bis zum ersten Segment
(Min–Max über die Streams), der grösste Verzug eines Segments gegenüber seiner Abspielzeit und die
CPU-Sekunden von ffmpeg je Stream. Das gleichzeitige Starten aller N Streams ist der schlechteste Fall;
im Betrieb starten Streams nacheinander. Quelle 1080p: «Dangerous Animals (2025)» (HEVC Main 10,
1920×800, EAC3 5.1, 8.8 Mbit/s → H.264 1080p, AAC); Remux-Quelle: «13 Minutes (2021)» (HEVC 8-bit, AC3).
Auf dem Entwicklungsrechner liegen beide Filme auf dem NAS (NFS), auf dem Testcontainer auf der lokalen
Disk unter `/srv/media/calib`.

```bash
server/bin/slouchcat hlsbench --file FILM --mode transcode --hw nvenc --streams 4 --seconds 45 --budget 1000 --json
docker exec slouchcat slouchcat hlsbench --file /media/calib/FILM --mode transcode --hw qsv --streams 4 --seconds 45 --budget 1000 --json
```

### 3a. Entwicklungsrechner (RTX 4070, NVENC; 28 Kerne für Software)

| Fall | Streams | Erstes Segment (ms) | Max. Verzug | CPU-s je Stream (45 s) | Wand (s) |
|---|---|---|---|---|---|
| Remux (`-c copy`) | 1 | 201 | 0 | 0.6 | 0.6 |
| Remux | 8 | 252–253 | 0 | 0.9 | 1.0 |
| NVENC 1080p | 1 | 502 | 0 | 1.2 | 3.7 |
| NVENC 1080p | 2 | 603 | 0 | 1.3 | 3.8 |
| NVENC 1080p | 4 | 916–1016 | 0 | 1.4 | 7.3 |
| NVENC 1080p | 6 | 901–1552 | 0 | 1.6 | 11.0 |
| NVENC 1080p | 8 | 1952–2103 | 0 | 1.8 | 14.9 |
| NVENC 1080p | 12 | **Encoder-Fehler** ab dem 9. Stream | – | – | – |
| Software (libx264 veryfast) 1080p | 1 | 851 | 0 | 26.5 | 3.3 |
| Software 1080p | 2 | 901–952 | 0 | 32.9 | 4.4 |
| Software 1080p | 4 | 1255–1652 | 0 | 40.6 | 8.6 |
| Software 1080p | 8 | 2007–2502 | 0 | 45.4 | 15.5 |
| Software 1080p | 14 | 3109–4702 | 0 | 46.5 | 26.6 |

- **NVENC:** Der Treiber der GeForce-Karte erlaubt höchstens 8 gleichzeitige NVENC-Sitzungen; der 9. und
  jeder weitere Encoder scheitern beim Öffnen (`Error while opening encoder`). Bis 8 Streams gab es keinen
  Segmentverzug; die Startzeit wächst linear (etwa 0.25 s je zusätzlichem gleichzeitig startenden Stream).
  Das Budget 12 (Plan-Startwert) lässt 6 Streams zu 1080p (Kosten 2) oder 8 zu 720p (Kosten 1.5) zu und bleibt
  damit innerhalb des Sitzungslimits; der Start bei 6 gleichzeitig startenden Streams liegt mit 1.55 s
  über der 1-s-Marke des Plans, aber weit unter dem W-3-Ziel von 5 s. **Kalibriert: Budget NVENC 12
  (bestätigt), Kosten 1080p 2 (bestätigt).** Ein Encoder-Fehler jenseits des Sitzungslimits wird dem Client
  als Fehler gemeldet; das Budget verhindert ihn.
- **Software:** Mit 28 Kernen trägt der Rechner 14 gleichzeitige 1080p-Transcodes ohne Verzug (Start bis
  4.7 s). Der Plan-Startwert «Kerne / 2» = 14 passt zur Messung, ist aber für den gleichzeitigen Start zu
  grosszügig (7 Streams zu Kosten 2 → Start ≈ 2 s). **Kalibriert: Budget Software = Kerne / 2 (bestätigt).**
- **Remux:** Kosten 0.5 bestätigt: 8 Streams brauchen 0.9 CPU-s je Stream für 45 s Inhalt; die Grenze ist
  die Lesegeschwindigkeit der Quelle (NFS), nicht die CPU.

### 3b. Testcontainer CT 190 (2 Kerne, Intel UHD 630 über QSV, jellyfin-ffmpeg 7.1)

`transcode.Detect` wählt im Container **QSV** (MediaSDK über `/dev/dri/renderD128`, Hardware-Tonemapping
`vpp_qsv`); der Software-Weg wurde mit `--hw software` gemessen.

| Fall | Streams | Erstes Segment (ms) | Max. Verzug | CPU-s je Stream (45 s) | Wand (s) |
|---|---|---|---|---|---|
| Remux | 1 | 253 | 0 | 1.2 | 1.0 |
| Remux | 4 | 504–554 | 0 | 1.3 | 2.6 |
| Remux | 8 | 808–1307 | 0 | 1.3 | 5.1 |
| QSV 1080p | 1 | 302 | 0 | 2.4 | 2.6 |
| QSV 1080p | 2 | 504 | 0 | 2.6 | 5.1 |
| QSV 1080p | 3 | 702–754 | 0 | 2.6 | 7.7 |
| QSV 1080p | 4 | 854–1008 | 0 | 2.6 | 10.3 |
| QSV 1080p | 6 | 1403–1503 | 0 | 2.6 | 15.6 |
| Software 1080p (2 Kerne) | 1 | 1501 | 0 | 31.6 | 16.4 |
| Software 1080p | 2 | 2803–2901 | 0 | 31.5 | 31.9 |

- **QSV:** 4 gleichzeitig startende Streams erreichen das erste Segment in ≤ 1.0 s, 6 Streams in 1.5 s;
  Verzug gab es nie (die UHD 630 wandelt 1080p-HEVC mit rund 17× Echtzeit je Stream um; sechs Streams
  teilen sich das). **Kalibriert: Budget QSV 8 (bestätigt), also 4 × 1080p oder 2 × 4K-HDR.**
- **Software (2 Kerne):** ein Stream läuft mit 2.7× Echtzeit (erstes Segment 1.5 s), zwei Streams mit je
  1.4× Echtzeit (erstes Segment 2.9 s). Budget «Kerne / 2» = 1 lässt genau einen Stream zu (eine Sitzung,
  die allein das Budget übersteigt, wird zugelassen, wenn sonst nichts läuft). **Bestätigt**, mit der
  Einschränkung aus AN-17 (Abschnitt 4).

### 3c. Kalibrierte Werte (ersetzen die Startwerte aus Plan 3.5)

| Backend | Budget | Streams 1080p (Kosten 2) | Streams 4K-HDR (Kosten 4) | Bemerkung |
|---|---|---|---|---|
| NVENC (RTX 4070) | 12 | 6 | 3 | Treiber-Limit 8 Sitzungen; Start bei 6 Streams 1.55 s |
| QSV (UHD 630) | 8 | 4 | 2 | Start bei 4 Streams 1.0 s |
| Software | Kerne / 2 | 28 Kerne: 7; 2 Kerne: 1 | 28 Kerne: 3; 2 Kerne: 1 (nicht echtzeitfähig, AN-17) | |
| Remux | – | Kosten 0.5, Grenze ist die Quelle | | |

Die Werte sind in `transcode.DefaultBudget` hinterlegt und über `SLOUCHCAT_TRANSCODE_BUDGET` überschreibbar;
die Admin-Oberfläche folgt in M8. Nicht umgesetzt (M4): die nächstniedrigere Qualität bei SDR-Quellen; M1b
wartet bis 10 s auf Budget und antwortet sonst 503 `transcode_busy`.

## 4. HDR10-4K → 1080p-SDR (Schritt 4)

Quelle: generierter Clip `Slouchcat Test HDR10 4K Long` (3840×2160, HEVC Main 10, PQ/BT.2020 mit
Mastering-Display- und MaxCLL-Metadaten, 120 s, x265 ultrafast, 258 MB; `.dev/hdr`, nicht im Repository).
Kein echter HDR-Film liegt vor (`/mnt/nerdnas/medien/Testfilme` sind 1080p-SDR). Ausgabe 1920×1080 H.264
SDR (BT.709), Audio AAC. Messung mit `hlsbench`, 45 s je Stream, wie in Abschnitt 3.

| Umgebung | Pipeline | Streams | Erstes Segment (ms) | Max. Verzug | Tempo | CPU-s je Stream |
|---|---|---|---|---|---|---|
| Entwicklungsrechner, NVENC | NVDEC → `scale_cuda` → `hwdownload` → `zscale`/`tonemap` (hable, CPU) → `h264_nvenc` | 1 | 1107 | 0 | 4.7× Echtzeit | 68 |
| Entwicklungsrechner, NVENC | dito | 2 | 1657 | 0 | 6.3× (Summe) | 90 |
| Entwicklungsrechner, NVENC | dito | 4 | 2714–2907 | 0 | 7.1× (Summe) | 114 |
| Entwicklungsrechner, Software | `zscale` → `tonemap` → `libx264 veryfast` (28 Kerne) | 1 | 1602 | 0 | 3.8× | 154 |
| Entwicklungsrechner, Software | dito | 2 | 2754–2755 | 0 | 4.4× (Summe) | 190 |
| Testcontainer, QSV | `vpp_qsv` (Skalierung + Tonemapping) → `h264_qsv` | 1 | 802 | 0 | 5.5× | 4.9 |
| Testcontainer, QSV | dito | 2 | 1502–1556 | 0 | 5.6× (Summe) | 5.0 |
| Testcontainer, QSV | dito | 3 | 2203–2405 | 0 | 5.6× (Summe) | 4.9 |
| Testcontainer, Software (2 Kerne) | `zscale` → `tonemap` → `libx264 veryfast` | 1 | 5052 | **2848 ms** | 0.85× | 103 |

- **NVENC auf dem Entwicklungsrechner** läuft ohne `tonemap_cuda`, weil das Ubuntu-ffmpeg 6.1.1 den
  Jellyfin-Filter nicht hat; das Tonemapping rechnet die CPU (68–114 CPU-s je 45 s). Das Docker-Image
  (jellyfin-ffmpeg 7.1) nutzt `tonemap_cuda`, sobald die NVIDIA-Container-Laufzeit installiert ist
  (`nvidia-container-toolkit`, nicht auf dem Entwicklungsrechner; Eintrag für
  `scripts/install-system-tools.sh`). Die Pipeline ist in `pipeline_test.go` als Golden-Liste hinterlegt,
  auf Hardware ungeprüft.
- **QSV auf dem Testcontainer** ist der Referenzfall des Plans (Intel-iGPU, «getestet (Referenz-NAS)»):
  erstes Segment 0.8 s, zwei Streams 1.5 s, Kosten 4 und Budget 8 passen (2 Streams).
- **Software ohne GPU (AN-17, Entscheid 4):** auf 2 Kernen ist 4K-HDR nicht echtzeitfähig (0.85×, Verzug
  wächst auf 2.8 s innert 45 s). Start 5.1 s liegt innerhalb der mit AN-17 bestätigten 10 s; die
  Wiedergabe würde nach etwa einer Minute stocken. Für solche Geräte ist 4K-HDR ohne GPU keine
  Wiedergabe-Option; M4 bietet dort nur Direct Play (W-7-Fähigkeiten) oder eine kleinere Ausgabe an, die
  beim Tonemapping aber kaum spart (Plan 3.5: Decodieren und Tonemapping der Quelle dominieren). Auf 28
  Kernen ist auch Software mit 3.8× Echtzeit flüssig.
- **Startzeit am Handy (Android-Chrome, Marco):** ausstehend, wird nachgetragen (Stack unter
  `http://192.168.1.143:8098`, Anleitung im Chat; HEVC-1080p-Film, Stoppuhr). Kein Gate.
- **VAAPI (AMD)** bleibt «best effort, ungetestet» (keine AMD-Hardware), Safari/iOS ungeprüft.

## 5. Bildpipeline (Schritt 5)

Messung 19. Oktober 2026 auf dem Entwicklungsrechner (i7-14700K, ffmpeg 6.1.1 des Hosts), mit
`BenchmarkImagePipelineTitle` in `server/internal/transcode/images_test.go`. Gemessen wird die Arbeit,
die `image.fetch` je Titel mit ffmpeg erledigt (ohne Download): Poster-Original in w200/w400/w800 und
Backdrop-Original in w780/w1280/w1920, je als WebP (libwebp, Qualität 82) und JPEG (q 3), ein
ffmpeg-Lauf je Art (das Original wird einmal dekodiert, `split` verteilt es auf die Skalierungen), dazu
`palettegen=max_colors=8` auf dem w200-Poster. Aufruf:

```bash
cd server
SLOUCHCAT_BENCH_POSTER=<poster.jpg> SLOUCHCAT_BENCH_BACKDROP=<backdrop.jpg> \
  go test ./internal/transcode/ -run '^$' -bench ImagePipelineTitle -benchtime 10x -count 3
```

| Originale | Zeit je Titel (6 Grössen × 2 Formate + Palette) |
|---|---|
| Echte TMDB-Originale ("Dark Phoenix": Poster 2000×3000, 2.0 MB; Backdrop 3840×2160, 1.4 MB) | 1.05 / 1.09 / 1.09 s |
| Synthetisch (`testsrc2` mit Rauschen, gleiche Auflösungen) | 1.03 / 1.05 s |

Aufteilung (einzelne ffmpeg-Läufe mit denselben Argumenten, echte Originale): Poster 0.53 s, Backdrop
0.66 s, Palette 0.05 s. ffmpeg arbeitet dabei praktisch einkernig (CPU-Zeit ≈ Wanduhrzeit), ein Titel
kostet also rund 1.2 CPU-Sekunden. Die Jobs laufen mit Priorität 2 auf den normalen Workern; mehrere
Titel werden parallel verarbeitet, sobald mehrere Worker frei sind. Für CT 190 (2 Kerne) ist mit etwa dem
Zwei- bis Dreifachen je Titel zu rechnen (nicht gemessen); 1'000 Titel wären damit in rund einer Stunde
neu gerechnet.

Grössen der Ausgaben für dieses Beispiel (JPEG / WebP): Poster w200 22 / 18 KB, w400 75 / 61 KB, w800
310 / 263 KB; Backdrop w780 94 / 74 KB, w1280 214 / 166 KB, w1920 431 / 343 KB. Das TMDB-Original
bleibt je Art einmal im Cache (`<kind>-original.<ext>`), damit eine neue Grösse oder eine gelöschte
Datei ohne erneuten Download nachgerechnet wird.

**Ablage:** `<cache>/images/<item>/<kind>-original.<ext>` und `<kind>-w<breite>.webp` / `.jpg`; eine Zeile
in `images` je (Titel, Art, Grösse, Format). Die Akzentfarbe (`#rrggbb`) steht auf jeder abgeleiteten
Poster-Zeile; gewählt wird aus der 8er-Palette die Farbe mit der höchsten HSL-Sättigung bei einer
Helligkeit von 0.25 bis 0.65, sonst die gesättigtste überhaupt.

**Übernahme der M1a-Bilder:** Migration `00006_images_formats.sql` behält die M1a-Zeilen (w500-Poster,
w780/w1280-Backdrop, unverändert von TMDB) als Format `jpeg`. Die Bildprüfung nach jedem Scan
(`QueueIncompleteImages`) wertet sie als unvollständig (WebP und neue Grössen fehlen, w500 wird nicht
mehr erzeugt) und reiht `image.fetch` ein; da der Server beim Start jede Bibliothek scannt, werden die
Bilder direkt nach dem Update neu gerechnet, die w500-Zeile samt Datei entfernt. Bis dahin hat ein
M1a-Titel kein Poster in der API (w500 ist nicht mehr Teil des Vertrags), die Backdrops w780/w1280
bleiben sichtbar.

## 6. Images amd64 und arm64 (Schritt 5)

DoD M1b: "Images amd64 und arm64 starten" (P-1). Entwicklungsrechner, Docker 29 mit containerd-Store,
QEMU-Handler über `docker run --privileged --rm tonistiigi/binfmt --install arm64` (`task deploy:binfmt`).
Build-Kontext: Stand `115519b` mit dem korrigierten `deploy/Dockerfile`.

**Fehler gefunden und behoben:** Das Dockerfile deklarierte `ARG TARGETARCH=amd64`. Ein Standardwert hat
Vorrang vor dem Wert, den BuildKit aus `--platform` setzt; der erste arm64-Build schrieb deshalb
`Architectures: amd64` in die Jellyfin-Quelle (apt brach mit Code 100 ab) und hätte ein amd64-Binary
eingebaut. Jetzt `ARG TARGETOS` / `ARG TARGETARCH` ohne Standard und `test -n "$TARGETARCH"` vor
`go build`. Zudem steht `ARG VERSION` im Laufzeit-Image erst nach der apt-Stufe: Ein Build-Argument
verwirft den Cache aller folgenden `RUN`, und `VERSION` ändert sich mit jedem Commit.

| Messung | amd64 (nativ) | arm64 (QEMU) |
|---|---|---|
| Build, apt-Stufe ohne Cache (Web, Go aus dem Cache) | 31 s, apt 22.5 s | 117 s, apt 113 s |
| Build mit neuer `VERSION`, sonst unverändert | – | 2 s |
| Beide Plattformen in einem Lauf (`build:multi`, `type=cacheonly`) | 115 s (arm64-apt ohne Cache) | |
| `GET /api/v1/healthz` antwortet ab `docker run` (3 Läufe) | – | 0.62 / 0.74 / 0.82 s |
| Docker-Status `healthy` (3 Läufe) | 1.19 / 1.20 / 1.25 s | 1.32 / 1.41 / 1.44 s |
| Server-Start (`startup_ms` im Log) | – | 259 / 285 / 379 ms |
| Leerlauf-RSS PID 1 | 23.5–24.6 MB | 70–71 MB (inkl. QEMU) |

Im arm64-Container: `uname -m` = `aarch64`, ELF-Maschine des Binarys AArch64 (`e_machine` 0xb7),
`ffmpeg version 7.1.4-Jellyfin`; Konfigurationsprüfung, Migrationen 1–4 und Schlüsselerzeugung laufen wie
unter amd64. **Ergebnis: beide Images starten; arm64 unter Emulation `healthy` nach 1.3–1.4 s (Grenze
5 s).** Das RSS unter QEMU ist kein Messwert für echte arm64-Hardware (der Emulator zählt mit); dafür
bleibt die Messung auf einem arm64-Gerät offen.

Werkzeuge: `task deploy:build:arm64`, `task deploy:build:multi`, `task deploy:healthy-time:arm64`
(`MAX_RSS_MB=120`), Beschreibung in `deploy/README.md`. CI: Job `image` baut beide Plattformen ohne Push
(`docker/setup-qemu-action`, `setup-buildx-action`, `build-push-action`, Cache `type=gha`), lädt jedes
Image einzeln und prüft `healthy` mit `deploy/scripts/measure-healthy.sh`.

**GitHub Actions (Job `image`, 19. Oktober 2026):** auf dem Runner (`ubuntu-24.04`, 4 vCPU) wird das
arm64-Image unter QEMU erst nach **6.96 s** healthy (Leerlauf-RSS 86.7 MB), weil die Backend-Erkennung ihren
Software-Test-Encode emuliert ausführt. Der erste Lauf des Jobs scheiterte deshalb an der 5-s-Grenze; die
Grenze gilt für echte Hardware (P-1), der emulierte Schritt prüft seither nur noch «startet innert 30 s».
amd64 nativ auf dem Runner: healthy innert 5 s, RSS unter 60 MB (Schritt grün).

## 7. Design-Feinschliff (Schritt 5)

### Akzentfarben

| Eigenschaft | Gesetzt von | Wert | Rückfall |
|---|---|---|---|
| `--sc-accent`, `--sc-on-accent` | `app/session.ts` `applyProfileAccent()` auf `<html>`, nach dem Login-Wächter; beim Abmelden und auf `/login` entfernt | `var(--sc-color-accent-<name>-base/-on)` für `profile.accent`, nur Namen aus `accentNames` der Tokens | `global.css`: ember |
| `--sc-item-accent` | `ui/accent.ts` `itemAccentStyle()` als Inline-Stil an Posterkarte, Backdrop und Detail-Poster | `ItemRef.accent`, nur `#rrggbb` (sonst nicht gesetzt) | in jedem Gebrauch: Glühen `var(--sc-item-accent, var(--sc-accent))`, Rahmen-Tint und Backdrop-Tint `var(--sc-item-accent, transparent)` |

Die alten Namen `--accent` / `--accent-on` sind ersetzt (auch im Player und im Assistenten). Im CSS stehen
weiterhin nur Token-Verweise (stylelint `color-no-hex`, `declaration-strict-value` grün); die Hex-Werte
kommen als Daten aus der API.

- **Posterkarte:** Beim Fokus (und Hover) leuchtet hinter dem Poster ein weichgezeichnetes Glühen in der
  Akzentfarbe des Titels (`::before`, `blur(space-5) brightness(1.35)`, Deckkraft 0 → 0.7 in 180 ms; nur
  die Deckkraft bewegt sich). Solange das Poster auf das Hero-Bild wartet (P-10), trägt der leere Rahmen
  die Akzentfarbe mit 18 % Deckkraft.
- **Detailseite:** neue Komponente `ui/Backdrop` (Bild, Scrim, Tint). Der Tint liegt im Mischmodus
  `color` über Bild und Scrim: Farbton und Sättigung vom Akzent, Helligkeit von den Ebenen darunter. Der
  Kontrast des Texts auf dem Scrim ändert sich damit nicht; zur Unterkante blendet eine Maske den Tint
  aus, damit keine Kante zur ungetönten Seite entsteht. Hinter dem Poster ein ruhendes Glühen (0.4).
- **Bildgrössen:** Karten `srcset` w200/w400 mit `sizes` nach Breakpoint (w800 bewusst nicht, ein
  3x-Telefon würde es nehmen); Detail-Poster w400/w800; Detail-Backdrop w780/w1280/w1920. Hero
  unverändert w780/w1280 (muss zu `api/preload.go` passen).

### Fokusbewegung ≤ 200 ms (Audit aller `transition`/`animation` in `web/src`)

Token `motion.duration.focus` = 180 ms. Alles, was sich mit dem Fokus bewegt, nutzt es jetzt.

| Komponente | Eigenschaft | Auslöser | Dauer |
|---|---|---|---|
| `Button` | `background-color`, `color` | Fokus (`:focus-visible`), Hover | focus 180 ms (vorher fast 120 ms) |
| `Button` | `transform` (0.97) | Drücken | fast 120 ms |
| `PosterCard` | `transform` (`focus.scale` 1.06) | Fokus, Hover | focus 180 ms |
| `PosterCard` | Glühen `opacity` | Fokus, Hover | focus 180 ms (neu) |
| `AppHeader` Navigationslink | `color` | Fokus, Hover | focus 180 ms (vorher fast) |
| `TextField` (`Field.module.css .input`) | `border-color`, `background-color` | Fokus, Hover | focus 180 ms (vorher fast) |
| `RadioGroup` (`.segment`) | `background-color`, `color` | Pfeiltasten (Fokus und Auswahl) | focus 180 ms (vorher fast) |
| Player Suchleiste (`.seekTrack`, `.seekThumb`) | `height`, `opacity`, `transform` | Fokus, Hover, Ziehen | focus 180 ms (Stream B, vorher fast) |
| Zeilen/Seite scrollen beim Fokuswechsel (`ui/reveal.ts`) | `scrollLeft`, `scrollTop` | Fokus per Pfeiltaste oder Tab | focus 180 ms, Kurve `easing.standard` |
| `:focus-visible`-Ring (`global.css`) | `outline` | Fokus | sofort, keine Transition |
| Sprunglink (`AppLayout`) | `transform` | Fokus | sofort |
| `Row` Blätterknöpfe | `opacity` | nur Hover (nicht im Tab-Ablauf) | fast 120 ms |
| `HeroCarousel` Bild, Folientext | `opacity`, `enter`-Animation | Folienwechsel (auch per Pfeiltaste am Rand), kein Fokuswechsel: der Fokus bleibt auf dem Knopf | crossfade 700 ms |
| `HeroCarousel` Punkte | `width`, `background-color` | Folienwechsel | enter 240 ms / fast |
| `#hero-prepaint` (`global.css`) | `opacity` | Karussellbild gezeichnet | crossfade 700 ms |
| `AppHeader` | `background-color` | Scrollen | enter 240 ms |
| `ProgressBar` | `width` | Daten | enter 240 ms |
| `StatusDot`, `Skeleton` | `background-color`, Puls | Status, Laden | fast / 1.4–1.6 s Puls |
| `ConfirmDialog` | `fade`, `rise` | Öffnen/Schliessen | enter 240 ms / exit 160 ms |
| `Backdrop` Bild | `appear` | Laden | crossfade 700 ms |
| Player Steuerleisten | `opacity`, `transform` | Einblenden nach Inaktivität | chrome 300 ms |

**Scrollen beim Fokus:** Vorher `scrollIntoView({ behavior: 'smooth' })`; die Dauer bestimmt dann der
Browser. Gemessen (Playwright, Zeile in Storybook, 700 px breit, ein Kartenschritt 228 px): Chromium
**263 ms**. Jetzt misst `reveal.ts` das Ziel mit einem sofortigen `scrollIntoView` (inklusive
Scroll-Snapping), setzt die alten Positionen zurück und gleitet in 180 ms dorthin (Snapping während der
Bewegung aus, danach wieder an). Gemessen: Chromium 180 ms (228 px), Firefox 160–179 ms (76–152 px).
Mit `prefers-reduced-motion` springt die Ansicht ohne Bewegung. `reveal.ts` liegt im nachgeladenen
norigin-Chunk (`spatialRuntime`), nicht im Start-JS.

### Startseite nicht schwerer (P-10)

`vite build`, vorher (Stand `115519b`) und nachher:

| Datei | vorher | nachher |
|---|---|---|
| Start-JS `index-*.js` | 434.67 kB, gzip 139.52 kB | 436.01 kB, gzip 140.08 kB (+0.56 kB) |
| Start-CSS `index-*.css` | 27.58 kB, gzip 5.75 kB | 28.75 kB, gzip 5.94 kB (+0.19 kB) |
| norigin-Chunk (nach dem Start-JS) | 52.88 kB, gzip 16.55 kB | 54.77 kB, gzip 17.37 kB (`reveal.ts`) |

Das Start-JS wird seit M1a Runde 4 erst nach dem gezeichneten Hero-Bild angefordert, die +0.56 kB gzip
liegen also hinter dem LCP. Die Poster werden leichter: Bildschirme mit 1x laden w200 statt w400, alle
anderen höchstens w400 (M1a: w500). Eine neue Lighthouse-Messung steht mit Stream C (neue Grössen) aus.

### Storybook und Screenshot-Prüfung (A-2)

- Pakete: `storybook` 10.6.2, `@storybook/react-vite` 10.6.2 (devDependencies, keine Addons; Storybook
  bringt `esbuild` 0.28.2 mit, dessen Installationsskript in `pnpm-workspace.yaml` per
  `allowBuilds: { esbuild: false }` aus bleibt).
- 54 Stories in 26 Dateien, eine Datei je Komponente in `web/src/ui` (ein Test prüft das), darunter
  Fokuszustände per echter Tastatur (Tag `press:Tab`, `press:ArrowDown+ArrowRight` …).
- `task web:storybook:test`: statischer Build (`storybook build`, rund 3 s), dann
  `e2e-storybook/components.spec.ts` in Chromium im Playwright-Image
  `mcr.microsoft.com/playwright:v1.64.1-noble` (gleiche Version wie `@playwright/test`), damit Schriften
  und Rendering auf jedem Rechner und in CI gleich sind; `toHaveScreenshot` mit
  `maxDiffPixelRatio: 0.001`, Referenzbilder in `web/e2e-storybook/__screenshots__/` (54 PNG, 2.9 MB).
  Drei Läufe hintereinander: 55 Tests grün in 6.5–6.8 s, Gesamtdauer der Aufgabe 9.8–10.2 s.
  Referenzbilder neu schreiben: `task web:storybook:update`.
- `task ci` ruft die Prüfung nach `e2e` auf. Mehrzeit in CI: rund 10 s plus einmal das Playwright-Image
  (986 MB komprimiert, erfahrungsgemäss 30–60 s auf GitHub-Runnern), also unter 2 Minuten.

## 8. Gerätecheckliste (Plan 10.1)

Stand 19. Oktober 2026. «–» = in diesem Meilenstein nicht vorgesehen (Profilwechsel mit PIN ab M2,
Ton- und Untertitelwechsel ab M4, Download ab M5).

| Punkt | Desktop Chrome (Chromium 156, Playwright) | Desktop Firefox (157, Playwright) | Android Chrome | iPhone-PWA / iPad / Safari macOS |
|---|---|---|---|---|
| PWA installieren | teilweise (Manifest, SW; keine Lighthouse-Prüfung mehr) | n/a (keine Installation) | ausstehend (Spike-Lauf) | ungeprüft (keine Hardware) |
| Login | bestanden | bestanden | ausstehend | ungeprüft |
| Profilwechsel mit PIN | – | – | – | – |
| Startseite lädt < 3 s | bestanden (LCP 1.80 s, M1a) | bestanden | ausstehend | ungeprüft |
| Direct Play startet | bestanden (43 ms) | bestanden (73 ms) | bestanden (M1a, Marco) | ungeprüft |
| Transcode startet | bestanden (373 ms; echter 1080p-Film im eingebetteten Chromium 445 ms) | bestanden (284 ms) | ausstehend | ungeprüft |
| Spulen auf 50 % | bestanden (304 ms; echter Film 681 ms) | bestanden (336 ms) | ausstehend | ungeprüft |
| Ton-/Untertitelwechsel | – | – | – | – |
| Tastatur auf der neuen/geänderten Seite (Player) | bestanden (`player.spec.ts`, `hls.spec.ts`) | bestanden | n/a | ungeprüft |
