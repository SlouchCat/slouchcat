# Messungen Konzept 18 «Vorschau beim Spulen (Trickplay)»

Gemessen am 8. November 2026 für `docs/planning/18-trickplay.md` (Abschnitt 3). Skripte in
[`18-trickplay/`](18-trickplay/). Bilder aus Filmen kommen nie ins Repository; die Skripte schreiben
sie nur in temporäre Ordner.

## 1. Aufbau

- **CT 190:** Intel Core i5-10500T (UHD 630), 6 Threads, 4 GB RAM, Slouchcat 1.19.2 im Betrieb,
  jellyfin-ffmpeg 7.1.4 im Container. Medien per NFS 4.2 (`rsize` 1 MB) vom NAS 192.168.1.50 unter
  `/media/nas`. Gemessen im laufenden Container (`docker exec`), Ausgaben in `/tmp/tp`, danach
  gelöscht. Gelesene NAS-Bytes aus `/proc/self/mountstats` des CT (Zähler `bytes:`, normale Lesungen)
  vor und nach jedem Lauf; der CT hatte keine andere NFS-Last.
- **Entwicklungsrechner:** 28 Threads, RTX 4070, ffmpeg 6.1.1, Dateien auf lokale NVMe kopiert
  (NAS → lokal: 5,9 GB in 21 s). Gelesene Bytes aus ffmpegs eigener Statistik
  (`-loglevel verbose`, «Statistics: N bytes read»).
- Bildpunkt alle 10 s, 320 px breit, JPEG `-q:v 4` (nur für die Zeitmessung; das Format der
  Blätter steht in Abschnitt 4).

## 2. CT 190 (`ct-run.sh`, `ct-extra.sh`)

Dateien: «3 Body Problem» S01E01 (WEB-DL 1080p HEVC, 61 min, 1,1 GB), «28 Days Later» (Bluray
1080p HEVC, 7,0 Mbit/s, 118 min, 5,5 GB). Beide vor der Messung nicht gelesen (kalt); der kleine
Seitencache des CT hielt auch bei Wiederholungen nichts (gleiche NAS-Bytes).

| Datei | Weg | Parallel | Bilder | Dauer | CPU | NAS |
|---|---|---|---|---|---|---|
| Folge | Springen, erste 60 Punkte | 1 | 60 | 6,5 s | 3,4 s | 62 MB |
| Folge | Springen | 2 | 363 | 18,7 s | 19,7 s | 352 MB |
| Folge | Springen | 4 | 363 | 8,4 s | 20,6 s | 352 MB |
| Folge | Nur Keyframes, ganze Datei | 2 Threads | 362 | 31,2 s | 9,2 s | 1 206 MB |
| Folge | VAAPI/QSV, ganze Datei | GPU | 363 | 86,6 s | 18,5 s | 1 248 MB |
| Folge | Ganz dekodieren (CPU) | 6 Threads | 363 | 196,9 s | 590,9 s | 1 292 MB |
| Film | Springen | 4 | 705 | 29,7 s | 87,2 s | 536 MB |
| Film | Springen | 2 | 705 | 42,3 s | 77,0 s | 589 MB |
| Film | Nur Keyframes, ganze Datei | 2 Threads | 704 | 159,9 s | 29,7 s | 6 243 MB |
| Film | VAAPI/QSV, ganze Datei | GPU | 705 | 179,4 s | 46,5 s | 6 193 MB |

**HDR** (`/media/calib/dutton-4k-hdr10plus-120s.mkv`, 4K HEVC 10 bit, lokal auf dem CT), Springen mit
der CPU-Tonemapping-Kette aus `GrabFrameArgs`, 11 Punkte nacheinander: **0,24 s CPU, 0,29 s Dauer je
Bild**.

**Neben einer Wiedergabe:** QSV-Transcode HEVC 1080p → H.264 (6 Mbit/s, Deckel 8 Mbit/s), 120 s
ab 20:00 von «21 Jump Street»: allein **11,1×**, neben dem Springen mit zwei Prozessen an «2012»
(158 min) **11,4×**. Das Springen dazu: 946 Bilder in 58,2 s, 81,5 s CPU.

## 3. Entwicklungsrechner (`bench.py`)

| Datei | Weg | Parallel | Bilder | Dauer | CPU | gelesen |
|---|---|---|---|---|---|---|
| «24» S01E01 (41 min) | Springen | 1 | 245 | 26,3 s | 29,5 s | 180 MB |
| «24» S01E01 | Springen | 4 | 245 | 6,3 s | 28,0 s | 180 MB |
| «24» S01E01 | Springen, 10 Punkte je Prozess | 4 | 245 | 4,3 s | 21,1 s | 218 MB |
| «24» S01E01 | Nur Keyframes, ganze Datei | 1 | 245 | 10,0 s | 11,3 s | 1 239 MB |
| «24» S01E01 | Nur Keyframes, NVDEC | GPU | 245 | 2,6 s | 3,2 s | 1 239 MB |
| «24» S01E01 | Ganz dekodieren, NVDEC | GPU | 245 | 93,3 s | 41,9 s | 1 239 MB |
| «24» S01E01 | Ganz dekodieren (CPU) | 28 Threads | 245 | 63,6 s | 456,8 s | 1 239 MB |
| «12 Years a Slave» (134 min) | Springen | 1 | 804 | 134,6 s | 124,6 s | 406 MB |
| «12 Years a Slave» | Springen | 4 | 804 | 36,5 s | 131,9 s | 406 MB |

Die lokale Reihe wurde nach diesen Zeilen abgebrochen; für das Konzept zählt CT 190.
«Springen, 10 Punkte je Prozess» spart rund ein Viertel CPU (weniger Prozessstarts) bei etwas mehr
Lesen; ein Feinschliff für den Bau.

**Bildgenau** (`-ss` ohne `-noaccurate_seek`, 20 Punkte der Folge): 13,3 s statt 1,9 s, 13,6 s statt
2,2 s CPU, 42 MB statt 13 MB.

**Der Kniff** (Konzept 3.2), ein Punkt bei 1000 s in «12 Years a Slave»:

| Aufruf | Pakete gelesen | Frames dekodiert | Bytes gelesen |
|---|---|---|---|
| `-noaccurate_seek -skip_frame nokey -ss` | 518 | 3 | 12,5 MB |
| `-noaccurate_seek -ss` | 254 | 252 | 6,0 MB |
| `-noaccurate_seek -ss … -fps_mode passthrough` | 6 | 4 | 0,4 MB |

Derselbe Vergleich bei 2000 s und 4000 s (Paketzahlen gleich): `-skip_frame nokey` 23,5 MB (4000 s),
ohne `-fps_mode passthrough` 9,1 MB (2000 s), mit 0,5 MB (2000 s). Daher «6 bis 23 MB je Bild» im
Konzept 3.2.

Keyframe-Abstände (ffprobe über alle Pakete): «12 Years a Slave» fest 10,42 s (773 Keyframes),
«Ad Astra» fest 5,0 s, «24» S01E01 1 bis 10 s (Mittel 4,35 s).

## 4. Blattgrössen (`sheets.sh`)

10 × 10 Kacheln je Blatt aus PNG-Einzelbildern (lanczos), Grösse aller Blätter je Stunde Video:

| Datei | Breite | JPEG q 3 | JPEG q 5 | JPEG q 8 | WebP 60 | WebP 75 | WebP 85 |
|---|---|---|---|---|---|---|---|
| «24» S01E01 (16:9) | 320 | 3,18 MB/h | 2,31 MB/h | 1,72 MB/h | 1,09 MB/h | 1,30 MB/h | 1,95 MB/h |
| «12 Years a Slave» (2,40:1) | 320 | – | 2,71 MB/h | – | – | 1,81 MB/h | – |
| «12 Years a Slave» (2,40:1) | 480 | – | 5,02 MB/h | – | – | 3,33 MB/h | – |

Ein Blatt WebP 75 bei 320 px: 295 KB (16:9) bzw. 448 KB (2,40:1, Blatt 3200 × 1340 px). Sichtprüfung
eines Blatts: Bilder scharf genug für die Vorschau; bei festem Keyframe-Abstand über 10 s erscheinen
benachbarte Kacheln doppelt (Grund für die Wahl des nächsten Keyframes im Konzept 5.4).

## 5. Bestand (für die Hochrechnung)

- CT 190, Datenbank nur lesend: 2 107 Filme mit 3 925 Stunden, 291 Folgen mit 116 Stunden;
  Keyframe-Index: `scan` 2 115, `cues` 902, `mp4` 2, ohne 87.
- NAS: 35 896 Folgen-Dateien unter `Serien`, `KinderSerien`, `MangaSerien` (`find`, `.mkv`/`.mp4`).

## 6. Messung beim Bau (9. November 2026, CT 190, `k18-175443c` = `v1.22.0-54-g175443ce`)

Gemessen vom Koordinator im laufenden Container auf CT 190 (Aufbau wie Abschnitt 1, Medien per
NFS). Zeiten mit «etwa» stammen aus wiederholtem Abfragen des Zustands und sind so genau wie dessen
Takt.

**Migrationen** (vorher wie immer die Sicherung der Datenbank): 00041 in **1,06 s**, 00042 in
**1,10 s**.

**Film beim Start der Wiedergabe:** «A Beautiful Mind» (2 h 15 min, 1080p, 9,3 GB über NFS),
Einstellung «Im Voraus», ohne Satz gestartet, also beim Start erzeugt:

| Messgrösse | Wert |
|---|---|
| Job | 66,4 s, 9 Blätter |
| Erstes Blatt sichtbar | nach etwa 12 s |
| Container | etwa 170–205 % CPU, 2 ffmpeg-Prozesse, höchstens 262 MiB Speicher |

**Neben einem QSV-Transcode** (720p H.264, 4 Mbit/s, «10DANCE», 2 h 08 min): Der Transcode lieferte
in 60 s **1 040 s** Medien (**17-fache Echtzeit**), während Trickplay lief. Container etwa 290–300 %
CPU, 3 Trickplay- und QSV-ffmpeg-Prozesse, höchstens 434 MiB.

| Datei | Job | Bemerkung |
|---|---|---|
| «10DANCE» (2 h 08 min) | 54,7 s | neben dem eigenen Transcode |
| «23 000 Lives» (1 h 54 min) | 73,0 s | wartete zuerst auf «10DANCE» (ein Worker, O-1 angenommen); erstes Blatt etwa 69 s nach seinem Start |

**Serienvorlauf:** «Two and a Half Men» S09, Folgen von etwa 20 min, Einstellung «Beim Start alle
Folgen», Umfang «Rest der Staffel»:

| Folge | Job | ffmpeg |
|---|---|---|
| gestartete Folge | 6,9 s | 2 |
| die nächsten zwei, gleichzeitig | 7,0 s und 6,8 s | je 2 |
| der Rest, solange die Sitzung offen war (gedrosselt) | 11,5–13,2 s je Folge | 1 |

Container dabei etwa 90–100 % CPU.

**Nicht gemessen:**

- 4K über NFS: keine 4K-Datei in den Bibliotheken von CT 190 (damit auch kein Transcode mit
  CPU-Tonemapping neben Trickplay, Abschnitt 3.4 des Konzepts).
- Remux neben Trickplay.
- Volles Lesen der Keyframes parallel dazu.
- Vorlauf über eine Stunde neben einem 4K-Remux.
- P-10 mit einem Hörbuch in der ersten Reihe und Scrollen auf dem Telefon: Der Browser-Bereich war
  verborgen, ohne CPU-Drosselung; offen für Marcos Abnahme auf dem Telefon.

**Vergleich mit dem Konzept (Abschnitt 3):** Das Konzept rechnet für einen Film von 2 Stunden mit
30 s (vier Prozesse) bis 42 s (zwei Prozesse) und für eine Folge von 61 min mit 19 s, das erste
Blatt nach 5 bis 10 s. Auf die Länge umgerechnet (42,3 s für 118 min, 18,7 s für 61 min, je zwei
Prozesse) wären es für «A Beautiful Mind» etwa 48 s (gemessen 66,4 s), für «10DANCE» etwa 46 s
(54,7 s) und für eine Folge von 20 min etwa 6 s (6,8–7,0 s). Das erste Blatt kam nach etwa 12 s.
Der gedrosselte Vorlauf mit einem Prozess braucht knapp das Doppelte (11,5–13,2 s), wie für
einen statt zwei Prozesse zu erwarten. Der Transcode mit 17-facher Echtzeit ist mit der 11,1-
bzw. 11,4-fachen aus Abschnitt 3.4 des Konzepts nicht direkt vergleichbar (720p mit 4 Mbit/s statt
1080p mit 6 Mbit/s).

**Lesart:** Der echte Job ist bei Filmen rund 20 bis 40 % langsamer als die Messung vom
8. November, bei kurzen Folgen etwa 10 %. Er tut mehr als jene Messung (Blätter als WebP
zusammensetzen, Prüfung nach jedem Blatt, Schreiben mit fsync), und «A Beautiful Mind» hat mit rund
9,2 Mbit/s eine höhere Bitrate als «28 Days Later» (7,0 Mbit/s); welcher Anteil worauf fällt, ist
nicht gemessen. Der QSV-Stream lief neben Trickplay weit schneller als die Wiedergabe braucht.
