# Testdaten

Generatoren für Test-Medien und Lasttest-Bäume (Plan M0, Schritt 6). Alles, was sie erzeugen, landet in
`testdata/media/` und ist per `.gitignore` vom Repository ausgeschlossen. Es werden keine urheberrechtlich
geschützten Medien verwendet: Bild und Ton stammen aus den ffmpeg-Quellen `testsrc2`, `smptebars`,
`smptehdbars` und `sine`.

## Test-Medien (`task testdata:media`)

`generate-media.sh` erzeugt kurze Dateien für die Wiedergabe-Testmatrix (Plan 10.3) in Plex/Jellyfin-
Benennung. Voraussetzung ist ffmpeg mit `libx264`, `libx265`, `ac3` und `dca`; das Skript bricht mit einer
klaren Meldung ab, wenn ffmpeg oder ein Encoder fehlt. Andere Binaries über Umgebungsvariablen:

```bash
FFMPEG=/usr/lib/jellyfin-ffmpeg/ffmpeg FFPROBE=/usr/lib/jellyfin-ffmpeg/ffprobe task testdata:media
FORCE=1 task testdata:media      # bestehende Dateien neu erzeugen
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

Hinweise:

- Die HDR10-Datei zeigt SDR-Testbalken, die als HDR10 gekennzeichnet sind. Für Erkennung und
  Tonemapping-Pipeline genügt das; für die visuelle Beurteilung des Tonemappings ist sie nicht geeignet.
- H.264 nutzt `-preset veryfast` statt `ultrafast`, weil `ultrafast` nur Constrained Baseline erzeugt;
  echte Dateien sind High Profile. HEVC nutzt `-preset ultrafast`. Keyframes alle 2 s (`-g 48`).
- PGS-Untertitel (Zeile "H.264 + PGS" in 10.3) kann ffmpeg nicht erzeugen (kein PGS-Encoder); eine
  freie PGS-Testdatei wird bei Bedarf in M4 ergänzt.
- Am Ende druckt das Skript eine ffprobe-Zusammenfassung je Datei und die HDR-Metadaten des ersten
  Frames der 4K-Datei. Die absichtlich kaputte Datei «Slouchcat Test Broken» fehlt in der
  Zusammenfassung, weil ffprobe sie nicht lesen kann.
- Die Fälle aus M3 (Versionen, Extras, Specials, Mehrfachfolge, kaputte Datei, Duplikat, Titel ohne
  Jahr) prüft `TestScanGeneratedMedia` im Paket `library`; die Web-E2E-Tests verwenden dieselben Namen.
  Eine Bibliothek vom Typ «Serien» gehört auf `media/TV`, eine vom Typ «Filme» auf `media/Movies`.

Laufzeit auf dem Entwicklungsrechner (ffmpeg 6.1.1 aus Ubuntu): rund 18 s für die Dateien aus M0,
zusammen 22 MB; mit den langen Clips aus M1b und den Fällen aus M3 belegt `media/Movies` und
`media/TV` zusammen rund 55 MB.

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
