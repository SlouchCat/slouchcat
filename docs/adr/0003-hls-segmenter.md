# ADR 0003: HLS-Segmentierung – Segment-Muxer mit absoluter Zeitachse

- Status: angenommen (19. Oktober 2026)
- Quelle: `docs/planning/03-plan-v1.md`, Abschnitt 3.5 (Seeking, HDR, Admission Control) und M1b
  Schritte 1 und 3
- Betroffene Spec-IDs: W-2 (SDR-Fall), W-3 (Spulen), W-8 (Admission Control)

## Kontext

Plan 3.5 verlangt verlässliches Spulen für Transcode und Remux: Die Playlist steht beim Start vollständig
fest (Transcode: 4-s-Raster; Remux: Schnitte auf Quell-Keyframes, Segmente mindestens 4 s), und ein
Sprung ausserhalb des erzeugten Bereichs beendet ffmpeg und startet es an der Segmentgrenze neu. Bereits
erzeugte Segmente müssen gültig bleiben. Das geht nur, wenn jeder Lauf für dasselbe Segment dieselben
Grenzen und dieselben Zeitstempel (fMP4 `tfdt`) schreibt, sonst braucht der Player Diskontinuitäten
(`EXT-X-DISCONTINUITY`), die eine vollständig vorab erzeugte VOD-Playlist nicht kennen kann.

Ausgabeformat ist fMP4-HLS (CMAF-artig): ein Init-Segment (`ftyp`+`moov`) und Mediensegmente aus
`moof`+`mdat`. hls.js (Chromium, Firefox) und Safari spielen das; HEVC-Remux braucht fMP4 mit Tag `hvc1`.

## Prüfung mit ffmpeg 6.1.1 (Entwicklungsrechner)

Geprüft am 19. Oktober 2026 mit dem Ubuntu-ffmpeg 6.1.1, einem 20-s-Clip mit B-Frames, unregelmässigen
Keyframes (0, 2, 5, 7, 9, 13, 14, 18 s) und AAC-Priming (Audio beginnt bei -21 ms), dem generierten
HDR10-4K-Clip und dem HEVC-10-Bit-Film "Dangerous Animals (2025)" (5942 s, NFS). Die Boxen wurden mit einem
eigenen Parser gelesen (`hls_ffmpeg_test.go`), dekodiert wurde mit ffmpeg (Init + Segmente).

1. **hls-Muxer:** schneidet auf einem Raster, das am ersten Paket nach dem Seek verankert ist
   (`pkt->pts - start_pts >= hls_time * n`). Nach einem Neustart mit `-ss` liegen Remux-Grenzen anders als
   in der vorab berechneten Playlist. Verworfen.
2. **Segment-Muxer** (`-f segment -segment_format mp4 -individual_header_trailer 0`): eine einzige
   mp4-Muxer-Instanz über alle Dateien eines Laufs. Die erste Datei eines Laufs beginnt mit
   `ftyp`+`moov`+`moof`+`mdat`, alle weiteren sind reine `moof`+`mdat`. Bestätigt.
3. **Fallstricke, die der Entwurf nicht vorhergesehen hatte** (aus dem Quelltext von 6.1.1 bestätigt):
   - `-segment_times` zählt ab dem pts des **ersten Videopakets des Laufs**, nicht absolut
     (`end_pts += reference_stream_first_pts`). Die Schnittliste wird deshalb je Lauf relativ zu dessen
     Startgrenze berechnet.
   - Der fragmentierte mp4-Muxer setzt bei `avoid_negative_ts=auto` intern `make_zero` und verschiebt
     jeden Lauf auf 0; `tfdt` ist dann pro Lauf relativ. Abhilfe: `avoid_negative_ts=disabled` für den
     inneren Muxer (über `-segment_format_options`) und den äusseren.
   - Ohne Edit-Liste (`empty_moov`) startet die Videopräsentation um die B-Frame-Verzögerung zu spät
     (83 ms bei 24 fps), wenn der Muxer `use_editlist=1` wählt. Abhilfe: `use_editlist=0`; dann ist
     `tfdt` die Dekodierzeit und `tfdt + Kompositionsversatz` genau der Quell-pts.
   - `-ss` vor `-i` landet bei MKV mit negativem Startzeitpunkt (AAC-Priming) und B-Frames einen Keyframe
     zu früh (ffmpeg addiert `start_time` und zieht für B-Frames 0,13 s ab). Beim Kopieren verwirft
     ffmpeg mit Ausgabe-`-ss` zudem den Keyframe selbst, weil es dessen **dts** mit dem Start vergleicht.
   - Ausgabe-`-ss` verschiebt die Zeitstempel trotz `-copyts`; `-output_ts_offset` mit demselben Wert
     stellt die absolute Zeit wieder her.
   - Bei SIGTERM schliesst ffmpeg das angefangene Segment und trägt es in die Segmentliste ein, als wäre
     es vollständig.

## Entscheid

**Zeitachse:** Alle Läufe schreiben die Quellzeit plus einen festen Versatz von 10 s (`-copyts`,
`-output_ts_offset`). Der Versatz hält negative Dekodierzeiten (B-Frames) und das AAC-Priming von
Segment 0 positiv; 10 s ist auch der übliche Startwert von Apple-Streams. hls.js und Safari bilden den
ersten `tfdt` auf den Playlist-Anfang ab, der Versatz ist für sie unsichtbar. Ergebnis: Das Segment *i*
beginnt in jedem Lauf mit einem Keyframe, dessen Präsentationszeit `Start(i) + 10 s` ist (Test: Abweichung
≤ 2 ms, gemessen 0–1 ms), und die Segmente verschiedener Läufe lassen sich ohne Diskontinuität mischen.

**Argumentliste** (einzige Stelle: `server/internal/transcode/pipeline.go`, Golden-Tests je Backend):

| Teil | Segment 0 | Neustart an Grenze *B* (Segment *i*) |
|---|---|---|
| Eingang | `-copyts -i Datei` | Transcode: `-copyts -noaccurate_seek -ss B -i Datei`; Remux: `-copyts -ss B -i Datei` |
| Ausgabe-Seek | – | Transcode: `-ss B -output_ts_offset B+10`; Remux: `-ss B-m -output_ts_offset B-m+10`, *m* = min(250 ms, halber Abstand zum vorherigen Keyframe) |
| Keyframes (Transcode) | `-force_key_frames` mit den Grenzen | dieselben Grenzen minus *B* (Encoderzeit beginnt bei 0) |
| Schnitte | `-segment_times` = Grenzen minus erster Keyframe (Remux) bzw. 0 | Grenzen minus *B*; plus ein unerreichbarer Wächterwert, damit nie das Standardraster von 2 s greift |
| Toleranz | `-segment_time_delta` 0,002 s (Remux, Index in ms) bzw. 0,25 s (Transcode: Keyframes nur an erzwungenen Stellen, `-g 9999`, Szenenwechsel aus) | gleich |
| Muxer | `-avoid_negative_ts disabled -f segment -segment_format mp4 -segment_format_options movflags=+empty_moov+frag_custom+default_base_moof+frag_discont+skip_trailer:avoid_negative_ts=disabled:use_editlist=0 -segment_start_number i -individual_header_trailer 0 -reset_timestamps 0 -segment_list list.txt -segment_list_type flat seg%05d.m4s` | gleich |

Video: H.264 High, Level 4.1, 8 Mbit/s Spitze bis 1080p (4 Mbit/s bis 720p), `yuv420p`; Audio AAC 160 kbit/s
Stereo (5.1 wird heruntergemischt) oder Kopie. Kapitel und Metadaten werden nicht übernommen
(`-map_chapters -1 -map_metadata -1`), sonst entsteht eine zusätzliche Textspur.

**Backends** (`Detect`: Encoder- und Filterliste, je Hardware-Backend ein Testbild; Reihenfolge NVENC,
QSV, VAAPI, Software):

| Backend | SDR | HDR10 → SDR | Status |
|---|---|---|---|
| NVENC | `-hwaccel cuda -hwaccel_output_format cuda`, `scale_cuda=…:format=nv12`, `h264_nvenc -preset p4 -tune hq -rc vbr -cq 23` | mit `tonemap_cuda` (jellyfin-ffmpeg): `scale_cuda,tonemap_cuda=…:tonemap=bt2390`; ohne (Ubuntu 6.1): `scale_cuda,hwdownload,format=p010le,zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p` | SDR und CPU-Tonemapping getestet; `tonemap_cuda` ungetestet (der Container hat keinen GPU-Zugriff ohne NVIDIA Container Toolkit) |
| QSV | `-init_hw_device vaapi=va:/dev/dri/renderD128 -init_hw_device qsv=qs@va`, `<codec>_qsv`-Decoder, `vpp_qsv=…:format=nv12`, `h264_qsv` | `vpp_qsv=…:tonemap=1` | ungetestet, Prüfung auf CT 190 (UHD 630) mit Schritt 4 |
| VAAPI | `scale_vaapi=…:format=nv12`, `h264_vaapi` | `tonemap_vaapi` bzw. CPU-Tonemapping zwischen `hwdownload` und `hwupload` | best effort, ungetestet |
| Software | `scale`, `format=yuv420p`, `libx264 -preset veryfast -crf 23` | `zscale` → `tonemap=hable` (der `tonemap`-Filter kennt kein bt.2390) | getestet |

Quellen, die die GPU nicht dekodiert (z. B. H.264 10 Bit, MPEG-4), laufen über Software-Dekodierung und
die CPU-Kette vor dem Hardware-Encoder.

**Ablage und Auslieferung:** ffmpeg schreibt in `<cache>/transcode/<Sitzung>/runNNNN/`. Ein Segment gilt
als fertig, sobald ffmpeg es in die Segmentliste schreibt (alle 50 ms gelesen); die Sitzung verschiebt es
dann nach `<Sitzung>/seg%05d.m4s`. Hat die Datei `ftyp`+`moov` vorne (erste Datei eines Laufs), wird der
Kopf abgeschnitten; der allererste Kopf wird `init.mp4`. Ein laufender Neustart überschreibt so nie eine
ausgelieferte Datei. Nach dem Abbruch eines Laufs werden dessen Listeneinträge verworfen (SIGTERM-Falle).

**Spulmodell:** `OpenSegment(i)` liefert von der Platte, wenn vorhanden; liegt *i* höchstens 3 Segmente vor
dem nächsten Segment des laufenden ffmpeg, wartet die Anfrage; sonst wird ffmpeg beendet (SIGTERM, nach
2 s SIGKILL) und an *i* neu gestartet. Erreicht ein Lauf ein schon vorhandenes Segment, wird er beendet;
die nächste fehlende Anfrage startet dort neu. Pro Anfrage höchstens zwei Starts; scheitert ffmpeg, kommt
dessen Fehlermeldung (letzte 4 KiB von stderr) zurück, das ganze Protokoll steht in `ffmpeg.log`.

**Prozesse und Admission Control:** `exec.CommandContext` mit Argumentliste, nach dem Start `nice 10` und
`oom_score_adj 500` (best effort). Budget je Backend: NVENC 12, QSV und VAAPI 8, Software Kerne / 2,
überschreibbar. Kosten nach Plan 3.5 (4K HDR mit Tonemapping 4, 4K 3, bis 1080p 2, Ausgabe bis 720p 1,5,
Remux 0,5 – auch mit AAC-Kodierung –, nur Audio 0,25). Kosten belegt eine Sitzung, solange ihr ffmpeg
läuft; `Start` wartet bis 10 s auf Budget, sonst `ErrBusy`. Eine Sitzung, die allein das Budget
überschreitet, startet, wenn sonst nichts läuft. Nach 60 s ohne Anfrage wird ffmpeg beendet und das Budget
frei; die Dateien bleiben. Eine fortgesetzte Sitzung erhält ihr Budget ohne Warten zurück (laufende
Wiedergabe darf nicht abbrechen).

## Messwerte (Entwicklungsrechner, RTX 4070, 28 Kerne)

`slouchcat hlsbench` mit "Dangerous Animals (2025)", HEVC Main 10 1920×800, EAC3 5.1, 60 s je Stream:

| Fall | Erstes Segment | Max. Verzug | CPU (ffmpeg) |
|---|---|---|---|
| Remux, 1 Stream | 0,10 s | 0 | 0,9 s |
| Remux, 4 Streams | 0,10–0,15 s | 0 | je 1,0–1,1 s |
| NVENC, 1 Stream | 0,35 s | 0 | 1,2 s |
| NVENC, 4 Streams | 0,75–1,05 s | 0 | je 1,6–1,7 s |
| Software, 1 Stream | 0,60 s | 0 | 30 s |

Spulen im selben Film (Neustart, erstes Segment / drei Segmente): NVENC 0,41–0,50 s / 0,70–0,80 s, Remux
0,15–0,21 s / 0,25–0,41 s. HDR10-4K-Clip über NVENC mit CPU-Tonemapping: erstes Segment 1,05 s, etwa 5-fache
Echtzeit. Die Kalibrierung von Kosten und Budgets (Plan 3.5) folgt in `docs/perf/M1b.md`.

## Konsequenzen und Grenzen

- An der Naht zweier Läufe überlappt beim Remux das Audio um bis zu 250 ms (Ausgabe-Seek vor dem
  Keyframe). Es sind dieselben Quellframes, der Player ersetzt sie beim Anhängen; beim Transcode beträgt
  die Überlappung nur das AAC-Priming (21 ms).
- Ohne Keyframe-Index fällt Remux auf das 4-s-Raster zurück; ffmpeg schneidet dann am nächsten Keyframe
  nach jedem Rasterpunkt. Die Playlist-Dauern stimmen nur ungefähr, und nach einem Sprung kann die
  Wiedergabe bis zu einem GOP früher oder später einsetzen. `probe.file` legt den Index für jede Datei an.
- Endet das Video vor dem Audio, kann das letzte Rastersegment ohne Videoframe bleiben und wird nie
  geschrieben; die Sitzung meldet es als `ErrNoSegment`.
- Ohne Drosselung kodiert ffmpeg bis zum Ende des Films voraus (Remux und NVENC mit einem Vielfachen
  der Echtzeit) und belegt entsprechend Platz im Cache, bis die Sitzung endet oder 60 s ruht.
  Eine Drosselung folgt bei Bedarf mit den W-8-Limits (M4).
- Fehlt `tonemap_cuda` (Ubuntu-ffmpeg), trägt die CPU das Tonemapping; im Container mit jellyfin-ffmpeg
  greift `tonemap_cuda`, sobald der Container eine NVIDIA-GPU sieht.

## Erwogene Alternativen

- **hls-Muxer** (`-f hls -hls_segment_type fmp4`): Rasteranker am ersten Paket nach dem Seek, siehe oben;
  Remux-Grenzen würden nach jedem Neustart wandern. Hätte für Transcode funktioniert, aber zwei
  Segmentierer für zwei Modi sind teurer als einer.
- **Neustart mit Diskontinuität** (Zeitachse je Lauf ab 0, `EXT-X-DISCONTINUITY` an der Naht): passt
  nicht zu einer beim Start vollständig erzeugten Playlist und verlangt Playlist-Neuladen im Player.
- **`-individual_header_trailer 1`** (jede Datei ein vollständiges mp4): jedes Segment trüge ein eigenes
  `moov`, die Fragmentnummern begännen je Datei bei 1, und `tfdt` wäre pro Datei relativ.
- **Zeitachse ohne Versatz** mit `make_non_negative`: verschiebt nur den Lauf ab Segment 0 (Priming) und
  wäre damit gegenüber Neustarts um 21 ms versetzt.
