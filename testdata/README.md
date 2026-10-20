# Testdaten

Generatoren für Test-Medien und Lasttest-Bäume (Plan M0, Schritt 6). Alles, was sie erzeugen, landet in
`testdata/media/` und ist per `.gitignore` vom Repository ausgeschlossen. Es werden keine urheberrechtlich
geschützten Medien verwendet: Bild und Ton stammen aus den ffmpeg-Quellen `testsrc2`, `smptebars`,
`smptehdbars` und `sine`, die PGS-Untertitel aus `make-pgs.py` und das HDR-Testbild aus
`hdr-quality.py` (beide nur Python-Standardbibliothek, Aufruf mit `python3 -I`).

## Test-Medien (`task testdata:media`)

`generate-media.sh` erzeugt kurze Dateien für die Wiedergabe-Testmatrix (Plan 10.3) in Plex/Jellyfin-
Benennung. Voraussetzung ist ffmpeg mit `libx264`, `libx265`, `ac3` und `dca`; das Skript bricht mit einer
klaren Meldung ab, wenn ffmpeg oder ein Encoder fehlt. Andere Binaries über Umgebungsvariablen:

```bash
FFMPEG=/usr/lib/jellyfin-ffmpeg/ffmpeg FFPROBE=/usr/lib/jellyfin-ffmpeg/ffprobe task testdata:media
FORCE=1 task testdata:media      # bestehende Dateien neu erzeugen
LONG=1 task testdata:media       # zusätzlich der 120-s-4K-HDR10-Clip (langsam, rund 260 MB)
OUT_DIR=/srv/media task testdata:media
```

| Datei (unter `media/`) | Inhalt | Zweck (Plan 10.3) |
|---|---|---|
| `Movies/Slouchcat Test H264 (2024)/….mp4` | H.264 High 1080p24 + AAC Stereo, 10 s, `faststart` | Direct Play |
| `Movies/Slouchcat Test HEVC 10bit (2024)/….mkv` | HEVC Main 10 (yuv420p10le) SDR BT.709 1080p + AAC, 10 s | Remux/Transcode |
| `Movies/Slouchcat Test HDR10 4K (2024)/….mkv` | HEVC Main 10 3840×2160, BT.2020/PQ, Mastering Display und MaxCLL 1000/400 als SEI, 30 s | Tonemapping |
| `Movies/Slouchcat Test AC3 51 (2024)/….mkv` | H.264 + AC3 5.1 (384 kbit/s) | Audio-Transcode |
| `Movies/Slouchcat Test DTS (2024)/….mkv` | H.264 + DTS 5.1 (768 kbit/s, Encoder `dca` mit `-strict -2`) | Audio-Transcode |
| `Movies/Slouchcat Test Subtitles (2024)/….mkv` | H.264 + AAC mit eingebetteten Spuren SRT de/en und ASS de/en | WebVTT-Umwandlung |
| `Movies/Slouchcat Test Sidecar Subtitles (2024)/` | dieselbe Datei ohne eingebettete Untertitel, daneben `.de.srt`, `.en.srt`, `.de.ass`, `.en.ass` | externe Untertitel |
| `Movies/Slouchcat Test Chapters (2024)/….mkv` | H.264 + AAC mit 3 Kapiteln aus einer ffmetadata-Datei | Kapitel |
| `TV/Slouchcat Test Show (2024)/Season 01/… S01E01.mp4`, `… S01E02.mp4` | H.264/AAC, 10 s | Serien-Erkennung |
| `TV/Slouchcat Test Show (2024)/Specials/… S00E01.mp4` | H.264/AAC, 10 s | Specials (Staffel 0) |
| `TV/Slouchcat Test Show (2024)/Season 01/… S01E03-E04.mp4` | H.264/AAC, 10 s | Mehrfachfolge (ein Item E03 mit `episode_end = 4`) |
| `Movies/Slouchcat Test H264 (2024)/… - 720p.mp4` | dieselbe Quelle wie die H.264-Datei, 1280×720, H.264/AAC, 10 s | zweite Version desselben Titels (B-5) |
| `Movies/Slouchcat Test Extras (2024)/….mp4` | H.264/AAC, 10 s | Film mit Extras (B-7) |
| `Movies/Slouchcat Test Extras (2024)/Trailers/Slouchcat Test Extras Trailer.mp4`, `Featurettes/Making Of.mp4`, `Deleted Scenes/Scene 1.mp4`, `Behind The Scenes/On Set.mp4`, `Slouchcat Test Extras (2024)-trailer.mp4` | H.264/AAC, je 5 s | fünf Extras: zwei Trailer, Featurette, entfernte Szene, Hinter den Kulissen (B-7) |
| `Movies/Slouchcat Test Broken (2024)/….mkv` | Textdatei mit Video-Endung; ffprobe scheitert | nicht abspielbar, `probe_error` (B-9) |
| `Movies/Slouchcat Test H264 Copy (2024)/….mp4` | Byte-Kopie der H.264-Datei (gleicher Teil-Hash) | Duplikat nach Hash (B-9) |
| `Movies/Slouchcat Test Unrated/Slouchcat Test Unrated.mp4` | H.264/AAC, 10 s, ohne Jahr, ohne NFO | Titel ohne Freigabe (B-9 «unrated») |
| `Movies/Slouchcat Test Multi Audio (2024)/….mkv` | H.264 1080p, 20 s; Stream 1 AAC Stereo `ger` (Standard, 440 Hz), Stream 2 AC3 5.1 `eng` (660 Hz), Streams 3/4 SRT `ger`/`eng` mit je fünf Untertiteln («Deutsch 1» … «Deutsch 5» bei 1–3, 5–7, 9–11, 13–15, 17–19 s) | Ton- und Untertitelwechsel (W-4), Spur-Mapping |
| `Movies/Slouchcat Test PGS (2024)/….mkv` | H.264/AAC 1080p, 20 s, Stream 2 `hdmv_pgs_subtitle` (`eng`): «Slouchcat PGS 1» 2–6 s, «Slouchcat PGS 2» 8–12 s | Bild-Untertitel einbrennen (10.3 «H.264 + PGS») |
| `Movies/Slouchcat Test HDR10 Chart (2024)/….mkv` | HEVC Main 10 3840×2160, BT.2020/PQ, Mastering Display und MaxCLL wie die HDR10-Datei, 10 s, Standbild: Graustufenkeil und sechs Hautton-Felder (Geometrie unten) | P-6 (`hdr-quality.sh`) |
| `Movies/Slouchcat Test HDR10 4K Long (2024)/….mkv` | nur mit `LONG=1`: dasselbe Muster wie die HDR10-4K-Datei, 120 s | 5-s-Start (W-3), `hlsbench` |

| `TV/Slouchcat Test Intro Show (2024)/Season 01/… - S01E01.mkv` bis `… - S01E05.mkv` | H.264 640×360 + AAC, 98 bis 106 s (E01 98 s, je Folge 2 s mehr); Ton: eigene Sinustöne, ab 5/12/8/15/10 s dasselbe 20-s-Intro (8 Töne à 2.5 s), wieder eigene Töne, die letzten 25 s dasselbe Abspann-Thema (10 Töne à 2.5 s); E03 mit den Kapiteln «Prolog», «Intro» (8–28 s), «Handlung», «Credits» (77–102 s) | Intro-/Abspann-Erkennung (W-9) |
Hinweise:

- Die HDR10-Datei zeigt SDR-Testbalken, die als HDR10 gekennzeichnet sind. Für Erkennung und
  Tonemapping-Pipeline genügt das; für die visuelle Beurteilung des Tonemappings dient das HDR10-Testbild.
- H.264 nutzt `-preset veryfast` statt `ultrafast`, weil `ultrafast` nur Constrained Baseline erzeugt;
  echte Dateien sind High Profile. HEVC nutzt `-preset ultrafast`. Keyframes alle 2 s (`-g 48`).
- PGS-Untertitel (Zeile "H.264 + PGS" in 10.3) kann ffmpeg nicht erzeugen (kein PGS-Encoder).
  `make-pgs.py` schreibt deshalb eine `.sup`-Datei selbst: je Untertitel ein Display Set (PCS, WDS,
  PDS, ODS, END) mit einem RLE-Bild aus einer eingebauten 5×7-Pixelschrift (achtfach vergrössert,
  weiss auf halbtransparentem Schwarz, unten mittig) und ein leerendes Display Set am Ende; am Anfang
  steht ein leeres Display Set bei 0 s, damit ffmpeg die Spur beim Muxen nicht um ihre Startzeit
  verschiebt. `python3 -I testdata/make-pgs.py out.sup [--caption START END TEXT ...]`.
- Am Ende druckt das Skript eine ffprobe-Zusammenfassung je Datei und die HDR-Metadaten des ersten
  Frames der 4K-Datei. Die absichtlich kaputte Datei «Slouchcat Test Broken» fehlt in der
  Zusammenfassung, weil ffprobe sie nicht lesen kann.
- Die Fälle aus M3 (Versionen, Extras, Specials, Mehrfachfolge, kaputte Datei, Duplikat, Titel ohne
  Jahr) prüft `TestScanGeneratedMedia` im Paket `library`; die Web-E2E-Tests verwenden dieselben Namen.
  Eine Bibliothek vom Typ «Serien» gehört auf `media/TV`, eine vom Typ «Filme» auf `media/Movies`.
- Die Intro-Staffel prüft `TestSegmentsDetectGeneratedSeason` im Paket `library` (Intro und Abspann auf
  ±2 s bei mindestens 4 von 5 Folgen, E03 aus den Kapiteln). Die Längen folgen den Regeln der Erkennung
  (Paket `segments`): das Intro liegt in den ersten 40 % der Folge, ein Abspann dauert mindestens 20 s.
  Die Töne zwischen Intro und Abspann sind je Folge pseudozufällig, aber für einen festen Startwert
  deterministisch (linearer Kongruenzgenerator im Skript, kein `$RANDOM`).

Laufzeit auf dem Entwicklungsrechner (ffmpeg 6.1.1 aus Ubuntu): rund 18 s für die Dateien aus M0,
zusammen 22 MB; mit den langen Clips aus M1b und den Fällen aus M3 belegt `media/Movies` und
`media/TV` zusammen rund 55 MB. Die Dateien aus M4 (Mehrspur, PGS, HDR-Testbild) brauchen rund 11 s.

### HDR10-Testbild (P-6)

`python3 -I hdr-quality.py chart OUT.ppm` schreibt ein 16-Bit-PPM (3840×2160), dessen R'G'B'-Werte
bereits PQ-codiert in BT.2020 sind; `generate-media.sh` wandelt es mit der BT.2020-Matrix in
`yuv420p10le` (begrenzter Bereich) und codiert es mit x265 (`-crf 16`, HDR10-Signalisierung und
Metadaten wie die HDR10-Datei). Gemessen weichen die decodierten Codewerte um höchstens 3 von den
Sollwerten ab. Koordinaten in Pixeln des 4K-Bilds (x nach rechts, y nach unten, Ende exklusiv):

| Bereich | Lage | Wert |
|---|---|---|
| Graustufenkeil, Feld i = 0 … 10 | x = round(i × 3840 / 11) bis round((i + 1) × 3840 / 11), y = 0 bis 1080 | R' = G' = B' = i / 10 des PQ-Signals (Luma-Code 64 + 87.6 × i; 50 % ≈ 92 cd/m², 70 % ≈ 620 cd/m², 100 % = 10'000 cd/m²) |
| Hautton-Feld k = 0 … 5 | Spalte c = k mod 3, Zeile r = k div 3: x = 240 + 1200 c bis + 960, y = 1200 + 480 r bis + 360 | 8-Bit-sRGB-Vorlage, linear nach BT.2020, sRGB-Weiss = 203 cd/m² (BT.2408), PQ-codiert |
| Hintergrund | übrige Fläche der unteren Hälfte | Schwarz (PQ 0) |

Hautton-Vorlagen (k = 0 … 5): ColorChecker «light skin» (194, 150, 130), «dark skin» (115, 82, 68),
Monk Skin Tone 2 (243, 231, 219), 4 (234, 218, 186), 6 (160, 126, 86), 8 (96, 65, 52).
Die ΔE-Auswertung mittelt je Feld nur die inneren 60 % (20 % Rand auf jeder Seite) und rechnet die
Koordinaten auf die Ausgabegrösse um.

### P-6: HDR → SDR je Backend (`hdr-quality.sh`)

```bash
testdata/hdr-quality.sh software            # zuerst: Software-Referenz für ΔE
testdata/hdr-quality.sh nvenc               # Entwicklungsrechner: libplacebo über Vulkan
SLOUCHCAT_BIN=server/bin/slouchcat testdata/hdr-quality.sh qsv /usr/lib/jellyfin-ffmpeg/ffmpeg
```

Das zweite Argument wählt das ffmpeg (für `tonemap_cuda` und QSV das jellyfin-ffmpeg), `SLOUCHCAT_BIN`
ein gebautes Binary statt `go run ./cmd/slouchcat`; die Software-Bilder für (b) müssen mit demselben
Stand erzeugt sein.

Das Skript holt die Pipeline des Servers über `slouchcat hlsbench --print-chain` (dieselben Funktionen
`transcode.Detect` und `transcode.HDRChain` wie der Server), rendert damit 10 Bilder (eines je Sekunde,
1920×1080) nach `testdata/out/hdr/<backend>/frame-NN.png` (ignoriert) und vergleicht:

- (a) SSIM gegen `testdata/reference/hdr/<backend>/` (`ffmpeg -lavfi ssim`). Fehlt die Referenz, legt
  der erste Lauf sie an und meldet «reference created, approve by inspection»; nach der Sichtprüfung
  wird sie eingecheckt. Schwelle: kleinster Wert ≥ 0.95.
- (b) mittleres ΔE2000 (CIEDE2000, sRGB → Lab D65) des Graustufenkeils und der Hautton-Felder gegen
  die Software-Bilder in `testdata/out/hdr/software/`. Schwelle: je Bereich ≤ 5.

Ausgabe als JSON (`ssim.min/mean`, `deltaE2000.wedge/skin` mit Mittelwert und Werten je Feld,
`pass`). Die Software-Pipeline nutzt `tonemap=hable` (ffmpegs `tonemap` kennt kein BT.2390), NVENC mit
libplacebo und `tonemap_cuda` BT.2390; die Abweichung zeigt sich vor allem in den hellen Keilfeldern.

## Platzhalter-Baum für Lasttests (`task testdata:tree`)

`generate-library-tree.py` erzeugt einen Bibliotheksbaum mit **leeren** Dateien, um den Scanner über
SMB und NFS zu messen (Entscheidungsprotokoll, Frage 3: mindestens 45'000 Dateien in 8'000 Ordnern).
Nur Python-Standardbibliothek, Aufruf mit `python3 -I`. Die Namen sind für einen gegebenen `--seed`
deterministisch.

Standardwerte: 2'500 Filme und 400 Serien × 5 Staffeln × 20 Folgen = 40'000 Folgen. Dazu kommen
realistische Beigaben in Radarr/Sonarr-Benennung: Ordner mit `{tmdb-…}`/`{tvdb-…}`, Auflösung im
Dateinamen, Trailer als `-trailer`-Datei oder im Ordner `Trailers/`, Ordner `Featurettes/` und
`Behind The Scenes/`, Untertitel `.de.srt`, `.en.srt`, `.de.forced.srt`, Staffel `Specials` bei einem
Teil der Serien, Titel mit Umlauten, Akzenten, `&`, Apostroph und Komma.

```bash
task testdata:tree                                  # nach testdata/media/placeholder-tree
task testdata:tree TREE_ROOT=/mnt/nas/slouchcat-load   # direkt auf die SMB/NFS-Freigabe
task testdata:tree -- --movies 20 --series 5        # kleiner Baum
python3 -I testdata/generate-library-tree.py --root /tmp/tree --movies 2500 --series 400 \
  --seasons 5 --episodes 20 --seed 1
```

Der Zielordner muss leer sein oder fehlen. Am Ende stehen Anzahl Dateien und Ordner je Art, die Laufzeit
und ob die Referenzgrösse erreicht ist.

Messung mit Standardwerten auf lokaler NVMe (ext4): 60'800 Dateien (40'000 Folgen, 2'500 Filme,
334 Specials, 1'808 Trailer, 3'361 Extras, 12'797 Untertitel) in 8'212 Ordnern, 0.7 s Laufzeit;
`find -type f | wc -l` = 60'800, `find -mindepth 1 -type d | wc -l` = 8'212.

## Eingefrorene DB-Stände (`testdata/db/`)

Ab M1a legt jeder Meilenstein einen eingefrorenen Datenbankstand `testdata/db/<Mx>.sql` ab (Plan 6.2,
Punkt 4); der Migrationstest spielt alle früheren Stände ein. In M0 ist der Ordner noch leer, siehe
`db/README.md`.

## Aufräumen

```bash
task testdata:clean    # löscht testdata/media komplett
```
