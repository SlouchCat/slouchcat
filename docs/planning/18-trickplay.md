# Konzept: Vorschau beim Spulen (Trickplay) für Filme und Serien

- **Status: Fassung 2 (8. November 2026), Messungen und Marcos Antworten F-1 bis F-11
  eingearbeitet (Abschnitt 11); Kritiker-Review steht aus.** Auslöser ist Marcos Auftrag vom 8. November 2026: Vorschaubilder für Filme und
  Folgen, erzeugt **sobald eine Folge oder ein Film gestartet wird**; dazu die Fragen, wie lange
  das dauert und ob die Bilder gespeichert werden. Grundlage ist Idee 05
  (`docs/ideas/05-trickplay.md`, aus Konzept 13 V-21, Frage 19 F-7 «später»).
  **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Verankert gegen:** `main` **`6c05eda7`** (Merge PR #125, Release 1.19.2), Vertragsfassung
  **1.19.0** (`api/openapi.yaml:4`), letzte Migration **`00039_radio_stage2.sql`**, letzte ADR in
  `main` **`0025`**, letzte Frage im Entscheidungsprotokoll **24**. Reserviert und nicht gebaut:
  Konzept 17 Stufe 2 (Vertrag 1.20.0) und Stufe 3 (Vertrag 1.21.0, Migration 00040, ADR 0026).
- **Nummern:** Dieses Papier ist **Konzept 18** und reserviert **Frage 25** und **ADR 0027**
  («Trickplay: Erzeugen beim Start, Springen an Keyframes, Modus je Bibliothek»). Vertrag:
  vorläufig **1.22.0 additiv**, **eine Migration**, vorläufig **00041** (Spalte
  `libraries.trickplay_mode`, Abschnitt 8.3); beide endgültig beim Baubeginn.
- `:Zeile`-Angaben beziehen sich auf `6c05eda7`; **nach Namen suchen, nicht nach Zeile.** Pfade
  ohne Präfix liegen unter `server/` (Go) oder `web/src/` (Web-App).
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).

> **Dieses Papier ändert keine Zeile Code.** Messskripte und Rohwerte liegen in
> `docs/perf/18-trickplay.md` und `docs/perf/18-trickplay/`.

## 0. Kurz gesagt

- **Es geht schnell, wenn man nicht die ganze Datei liest.** Der Server springt für jeden
  Zeitpunkt (alle 10 s) direkt an das nächste Keyframe und dekodiert nur dieses eine Bild. Auf
  CT 190 dauert das für eine **Folge von 61 Minuten 19 s**, für einen **Film von 2 Stunden 30 s
  (vier Prozesse) bis 42 s (zwei Prozesse)**; ein 4K-HDR-Film braucht mit Tonemapping etwa das
  Zweieinhalbfache. Das **erste Kachelblatt** (100 Bilder, gut
  16 Minuten Film rund um die Startposition) ist nach **5 bis 10 s** fertig.
- **Das NAS merkt kaum etwas davon:** rund **1 MB je Bild**, also 350 MB für die Folge und 540 MB
  für den Film, statt 1,2 GB bzw. 6,2 GB beim Lesen der ganzen Datei (dauert auf CT 190 31 s bzw. 160 s). Ein laufender Stream wird
  nicht gebremst (Abschnitt 3.4).
- **Speichern lohnt sich:** WebP mit 320 px Breite braucht **1,3 bis 1,8 MB je Stunde Video**.
  Ein Film belegt rund 4 MB, eine Folge 1 bis 2 MB. Selbst der ganze Bestand (rund 31 000 Stunden)
  käme auf etwa 55 GB, nur die geschauten Titel auf wenige GB. Jedes Mal neu erzeugen hiesse
  dagegen bei jedem Start wieder 1 bis 2 CPU-Minuten und einige hundert MB NAS-Lesen.
- **Marcos Entscheid (F-1 bis F-11):** Jede Film- und Serienbibliothek wählt
  **«Speichern»** (ab Werk), **«Bei jedem Start erzeugen»** oder **«Aus»**. Beim Speichern entstehen
  die Bilder beim Start, für neue Dateien und per Nachholen (Knopf und Zeitplan, Zeitplan ab Werk
  aus), wie bei Emby. Beim Erzeugen je Start entstehen sie nur für die laufende Wiedergabe und
  werden danach gelöscht. Bildpunkt **am nächsten Keyframe**, alle **10 s**, **320 px**, zwei
  ffmpeg-Prozesse, alle Profile; Downloads nehmen die Vorschau mit.

## 1. Was Marco will

| Nr. | Marcos Wunsch (8. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| T-1 | Vorschaubilder auf der Zeitleiste (Trickplay) für Serienfolgen und Filme. | 5, 9 |
| T-2 | Die Daten entstehen, **sobald eine Folge oder ein Film gestartet wird**. | 5 |
| T-3 | Wissen, **wie lange** das dauert. | 3 |
| T-4 | Abwägen, ob die Bilder **gespeichert werden oder nicht**. | 6 |
| T-5 | Messen an einem echten FullHD-Film und einer echten Folge, sauber konzipieren. | 3, Anhang |

## 2. Was sich gegenüber früheren Entscheiden ändert

| Früher | Neu | Warum |
|---|---|---|
| Frage 19, F-7: Trickplay **«später»**; Konzept 13 V-21 entfällt, Idee 05 | Trickplay wird gebaut (dieses Papier) | Marcos Auftrag vom 8. November 2026 |
| Idee 05 Abschnitt 7: «Ein Durchlauf liest die **ganze Datei**» | Ein Durchlauf liest **nur die Keyframes an den Bildpunkten**, rund 1 MB je Bild | Messung 3.2: der Sprung kostet 0,4 bis 1 MB, nicht 6 bis 23 MB, wenn ffmpeg mit `-fps_mode passthrough` läuft |
| Konzept 13, 11.6: Kapitelbilder **nie während der Wiedergabe**, erst 10 Minuten nach dem Ende | Trickplay läuft **während** der Wiedergabe, gleich nach dem Start (T-2) | Messung 3.4: ein QSV-Transcode daneben bleibt schneller als Echtzeit; die NAS-Last ist klein. Die Regel für Kapitelbilder bleibt unverändert |
| Bibliotheken haben keine Einstellung zu erzeugten Bildern | Neue Einstellung **«Vorschau beim Spulen»** je Film- und Serienbibliothek (F-1, F-10) | Marco: «Parameter in den Einstellungen», «analog Emby» |

## 3. Messungen

Gemessen am 8. November 2026 auf **CT 190** (Intel Core i5-10500T, 6 Threads, 4 GB RAM, Intel UHD
630 mit QSV, jellyfin-ffmpeg 7.1.4 im Slouchcat-Container 1.19.2, Medien per NFS 4.2 vom NAS
192.168.1.50) und zum Vergleich auf dem Entwicklungsrechner (28 Threads, RTX 4070, ffmpeg 6.1,
Dateien lokal auf NVMe kopiert). Bildpunkt alle 10 s, 320 px breit.

| Datei | Ort | Länge | Grösse | Codec | Keyframe-Abstand |
|---|---|---|---|---|---|
| «3 Body Problem» S01E01 (WEB-DL 1080p) | CT 190, NAS | 61 min | 1,1 GB | HEVC | – |
| «28 Days Later» (Bluray 1080p) | CT 190, NAS | 118 min | 5,5 GB | HEVC, 7,0 Mbit/s | – |
| «24» S01E01 (WEBRip 1080p) | lokal | 41 min | 1,2 GB | HEVC, 4,0 Mbit/s | 1 bis 10 s, Mittel 4,4 s |
| «12 Years a Slave» (Bluray 1080p) | lokal | 134 min | 5,9 GB | HEVC, 5,9 Mbit/s | fest 10,4 s |
| «Ad Astra» (Bluray 1080p) | lokal | 123 min | 4,1 GB | HEVC, 4,5 Mbit/s | fest 5,0 s |

Der Bestand ist fast nur HEVC; auch Dateien mit «x264» im Namen waren HEVC.

### 3.1 Vier Wege, die Bilder zu gewinnen

| Weg | Was ffmpeg tut | Liest |
|---|---|---|
| **Ganz dekodieren** | jede Frame der Datei dekodieren, alle 10 s eines behalten | ganze Datei |
| **Nur Keyframes** (`-skip_frame nokey`) | ganze Datei lesen, nur Keyframes dekodieren | ganze Datei |
| **Hardware** (QSV/VAAPI bzw. NVDEC) | wie «ganz dekodieren», aber auf der GPU | ganze Datei |
| **Springen** (`-ss` vor `-i`, ein Prozess je Bildpunkt) | an das Keyframe vor dem Zeitpunkt springen, ein Bild dekodieren | ~1 MB je Bild |

### 3.2 Der Kniff beim Springen

Der erste Versuch las beim Springen **6 bis 23 MB je Bild** und dekodierte ein ganzes GOP (bis
252 Frames). Grund: ffmpeg verschiebt die Zeitstempel so, dass der Sprungpunkt 0 ist; das Keyframe
davor bekommt einen negativen Zeitstempel und wird vom Bildratenabgleich verworfen, also dekodiert
ffmpeg weiter bis zum Zeitpunkt. Mit `-skip_frame nokey` wartet der HEVC-Decoder zudem auf das
**nächste** Keyframe, bevor er das erste ausgibt. Richtig ist:

```
ffmpeg -threads 1 -noaccurate_seek -ss <t> -i <datei> -map 0:v:0 -frames:v 1 \
       -fps_mode passthrough -vf scale=320:-2 <bild>
```

Damit liest ein Bildpunkt **0,4 bis 0,7 MB** (lokal gemessen über ffmpegs eigene Statistik), auf
dem NFS des CT rund **0,8 bis 1 MB** (Lesegrösse 1 MB), und dekodiert 3 bis 4 Frames. Das Bild
stammt vom **Keyframe vor** dem Zeitpunkt, also bis zu einem Keyframe-Abstand früher (bei «12 Years
a Slave» bis 10,4 s). Bildgenau (ohne `-noaccurate_seek`) kostet es lokal **das Sechsfache an
CPU** (0,67 statt 0,11 s je Bild).

### 3.3 Ergebnisse auf CT 190

| Datei | Weg | Parallel | Bilder | Dauer | CPU | NAS gelesen |
|---|---|---|---|---|---|---|
| Folge 61 min | Springen, nur erste 60 Punkte | 1 | 60 | 6,5 s (0,11 s je Bild) | 3,4 s | 62 MB |
| Folge 61 min | **Springen** | **2** | 363 | **18,7 s** | 19,7 s | **352 MB** |
| Folge 61 min | Springen | 4 | 363 | 8,4 s | 20,6 s | 352 MB |
| Folge 61 min | Nur Keyframes, ganze Datei | 2 Threads | 362 | 31,2 s | 9,2 s | 1 206 MB |
| Folge 61 min | QSV, ganze Datei | GPU | 363 | 86,6 s | 18,5 s | 1 248 MB |
| Film 118 min | **Springen** | **4** | 705 | **29,7 s** | 87,2 s | **536 MB** |
| Film 118 min | **Springen** | **2** | 705 | **42,3 s** | 77,0 s | 589 MB |
| Film 118 min | Nur Keyframes, ganze Datei | 2 Threads | 704 | 159,9 s | 29,7 s | 6 243 MB |
| Film 118 min | QSV, ganze Datei | GPU | 705 | 179,4 s | 46,5 s | 6 193 MB |
| Folge 61 min | Ganz dekodieren (CPU) | 6 Threads | 363 | 196,9 s | 590,9 s | 1 292 MB |

**Erkenntnisse:**

1. **Springen gewinnt** auf CT 190 klar: Es liest ein Drittel bis ein Zehntel der Bytes und ist
   schneller als jede Volllesung (Film: 42 s statt 160 s). Die Volllesung ist durch das NAS
   gebunden (rund 39 MB/s). Ganz dekodieren auf der CPU belegt alle sechs Threads gut drei
   Minuten für eine Folge und scheidet aus.
2. **QSV hilft nicht:** Die GPU dekodiert jede Frame und ist mit 87 s für 61 Minuten langsamer als
   die CPU, die nur Keyframes anfasst. Für Trickplay braucht es keine GPU.
3. **Kosten je Bild auf CT 190:** rund **0,05 s CPU** bei WEB-DL, **0,12 s CPU** bei Bluray (grössere
   Keyframes). Zwei Prozesse belegen knapp zwei der sechs Threads.

### 3.4 Wiedergabe daneben

Ein QSV-Transcode (HEVC 1080p → H.264 6 Mbit/s, 120 s ab Minute 20 von «21 Jump Street»)
lief **allein mit 11,1-facher** und **neben dem Springen mit zwei Prozessen mit 11,4-facher
Echtzeit**. Das Springen erzeugte daneben die 946 Bilder von «2012» (158 min) in **58,2 s**. Ein
laufender Stream wird also nicht gebremst; QSV und die zwei CPU-Prozesse kommen sich nicht in die
Quere.

**Grenze:** Ein Transcode mit **CPU-Tonemapping** (HDR auf QSV, ADR 0021) belegt selbst die CPU.
Läuft so einer, soll der Trickplay-Job auf **einen** Prozess zurückgehen (Abschnitt 5, Punkt 2).
Das ist nicht gemessen und gehört in die Messung von Bauschritt 5.

### 3.5 HDR und Dolby Vision

Gemessen an `dutton-4k-hdr10plus-120s.mkv` (4K HEVC 10 bit, HDR10+, auf CT 190 lokal) mit der
Tonemapping-Kette aus `GrabFrameArgs`: **0,24 s CPU und 0,29 s Dauer je Bild**, also rund das
Zweieinhalbfache eines 1080p-Bildes. Ein 4K-HDR-Film von 2 Stunden (720 Bilder) braucht mit zwei
Prozessen rund **105 s**. Die Bibliothek auf CT 190 hat heute keine 4K-Datei; Marcos NAS schon.

### 3.6 Grösse der Kachelblätter

Blatt mit 10 × 10 Bildern (100 Bildpunkte = 16 min 40 s Video), Höhe nach Seitenverhältnis:

| Datei | Breite | JPEG q 5 | **WebP 75** | WebP 60 | WebP 85 |
|---|---|---|---|---|---|
| Folge 41 min (16:9) | 320 px | 2,3 MB/h | **1,3 MB/h** (295 KB je Blatt) | 1,1 MB/h | 2,0 MB/h |
| Film 134 min (2,40:1) | 320 px | 2,7 MB/h | **1,8 MB/h** (448 KB je Blatt) | – | – |
| Film 134 min (2,40:1) | 480 px | 5,0 MB/h | 3,3 MB/h (825 KB je Blatt) | – | – |

Ein Film von 2 Stunden belegt mit WebP 75 bei 320 px rund **4 MB in 8 Blättern**, eine Folge von
45 Minuten rund **1,4 MB in 3 Blättern**.

**Hochrechnung Bestand:** CT 190 kennt 2 107 Filme mit 3 925 Stunden; auf dem NAS liegen 35 896
Folgen (bei angenommen 45 Minuten im Mittel rund 27 000 Stunden). Alles zusammen ergäbe rund
**31 000 Stunden × 1,8 MB ≈ 55 GB**. Auf CT 190 sind im Cache 35 GB frei (Bild-Cache heute 25 GB).
Nur geschaute Titel: einige hundert im Jahr, also **unter 2 GB im Jahr**.

## 4. Überblick der Lösung

```
Bibliothek: trickplay_mode = store | per_start | off
                 store                         per_start                 off
Start           trickplay.generate  ->  dauerhaft  |  temporär         |  nichts
Neue Datei      trickplay.generate (Prio 2)        |  nichts            |  nichts
Nachholen       trickplay.backfill (Knopf, Plan)   |  nichts            |  nichts
Download        Blätter mitgeben, fehlende zuerst erzeugen (in jedem Modus ausser off)
Pflege          library.sweep: alte Versionen, verschwundene Dateien, temporäre Sätze ohne
                Wiedergabe, gespeicherte Sätze von Bibliotheken, die nicht mehr store sind

trickplay.generate (Pool «trickplay», 1 Arbeiter, 2 ffmpeg-Prozesse)
   ├─ Bildpunkte: alle 10 s das nächste Keyframe aus dem Keyframe-Index
   ├─ Reihenfolge: Blatt der Startposition zuerst, dann vorwärts, dann rückwärts
   ├─ je Bildpunkt: transcode.Runner (Sprung, ein Bild)
   └─ je fertiges Blatt: Kacheln zu WebP fügen, ablegen, freigeben
App: PlaybackInfo.trickplay nennt Raster und fertige Blätter; der Player fragt nach, solange der
     Satz nicht fertig ist, und zeigt bis dahin das Kapitelbild.
```

## 5. Erzeugen (T-2)

1. **Auslöser beim Start:** `playback.Start` reiht nach dem Muster von `queueKeyframes` den Job
   `trickplay.generate` ein (Payload `{"mediaFileId", "startMs", "mode"}`, Schlüssel
   `trickplay:<id>`), wenn die Bibliothek nicht `off` ist, die Datei ein Film oder eine Folge mit
   Video-Stream ist (`chapterKind`) und kein fertiger Satz der aktuellen Version vorliegt (gespeichert
   oder temporär). Ein wartender Job derselben Datei wird nur angehoben. Hörbücher, Podcasts und
   Radio bekommen nie Trickplay.
2. **Auslöser neue Datei** (nur `store`, F-5): der Probe einer Datei, die ein Scan nach einem
   vollständigen Scan der Bibliothek hinzufügte oder änderte, reiht den Job mit **Priorität 2** ein
   (Muster `probeJob`/`chapterImages`). Anders als bei den Kapitelbildern wartet er nicht auf das
   Ende einer laufenden Wiedergabe der Datei.
3. **Priorität beim Start:** über allen Hintergrundjobs ausser den Keyframe-Aufträgen für eine
   Wiedergabe; läuft wie diese **trotz Pause** der Jobart (Muster ADR 0022,
   `EnqueueOverridingPause`), weil der Zuschauer wartet. Höchstens **ein** Trickplay-Job
   gleichzeitig (eigener Pool `trickplay` mit einem Arbeiter); ein Start-Auftrag überholt wartende
   Aufträge aus Import und Nachholen. Der Job nutzt **zwei** ffmpeg-Prozesse (F-11); läuft gerade
   ein Transcode mit CPU-Tonemapping oder ein reiner CPU-Transcode, nur **einen**.
4. **Reihenfolge:** zuerst das Blatt, in dem die Startposition liegt, dann die folgenden Blätter,
   zuletzt die davor. Ein Blatt (100 Bilder) braucht auf CT 190 rund **5 s** (WEB-DL) bis **10 s**
   (Bluray). Import und Nachholen beginnen vorne.
5. **Teilergebnis:** Jedes fertige Blatt ist sofort abrufbar. Bricht der Job ab (Neustart, Datei
   offline), bleiben fertige Blätter erhalten, und der nächste Lauf macht nur den Rest.
6. **Bildpunkte** (F-2 «nächstes Keyframe»): Für jeden 10-s-Abschnitt das **nächste** Keyframe aus
   dem gespeicherten Keyframe-Index (`media_files.keyframes`, ADR 0022; auf CT 190 haben 3 019 von
   3 106 Dateien einen). Der Sprung geht genau auf dieses Keyframe, ffmpeg dekodiert ein Bild.
   Liegen zwei Abschnitte auf demselben Keyframe (Abstand über 10 s), bekommt der zweite das
   folgende Keyframe; gibt es keins im Abschnitt, wiederholt er das Bild. Ohne Index (`none`, noch
   ausstehend) springt der Job mit `-noaccurate_seek` an den Abschnittsanfang (Keyframe davor,
   Messung 3.2).
7. **Schwarze Bilder** werden behalten: Die Vorschau zeigt, was an der Stelle ist; anders als
   beim Kapitelbild gibt es keinen Ersatzpunkt.
8. **HDR und Dolby Vision:** dieselbe CPU-Tonemapping-Kette wie `GrabFrameArgs` (Messung 3.5).
9. **Fehler:** Datei nicht lesbar → Muster der Kapitelbilder (`chapterUnreadableRetry` 1 h,
   Aufgeben nach 7 Tagen); ein einzelner kaputter Bildpunkt wird schwarz gefüllt und protokolliert.

## 6. Speichern oder jedes Mal erzeugen (T-4)

### 6.1 Abwägung

| Modus | Dauer beim Start | Speicher | NAS und CPU |
|---|---|---|---|
| **Speichern** (`store`, ab Werk) | beim **ersten** Start 20 bis 45 s (4K HDR bis 2 min), danach **sofort**; neue Dateien haben die Vorschau meist schon vor dem ersten Start | ~4 MB je Film, ~1,5 MB je Folge; ganzer Bestand ~55 GB | einmal je Datei |
| **Bei jedem Start erzeugen** (`per_start`) | **bei jedem Start** 20 bis 45 s, bis dahin nur Teilvorschau | nur während der Wiedergabe | jeder Start 1 bis 2 CPU-Minuten und 350 bis 550 MB NAS |
| **Aus** (`off`) | keine Vorschau, Zeitleiste wie heute mit Kapitelbild | keiner | keine |

Marco will beides als **Einstellung je Bibliothek** (F-1, F-10), ab Werk **Speichern** (F-11).
Gedacht ist «Bei jedem Start erzeugen» für Bibliotheken, deren Bilder nicht dauerhaft auf dem
Server liegen sollen oder deren Cache knapp ist.

### 6.2 Ablage beim Speichern

- Ordner `cache/trickplay/<mediaFileId>-<probedAt>/` mit `manifest.json` (Abstand, Kachelgrösse,
  Raster, Liste der Bildpunkte in ms) und `<blatt>.webp`. Ein neuer Probe ergibt einen neuen Ordner.
- Ein Ordner, der noch geschrieben wird, heisst `.tmp-…`; fertige Blätter werden einzeln per
  Umbenennen veröffentlicht, das Manifest zuletzt.
- **Nachholen** (F-5): Jobart `trickplay.backfill` mit Knopf auf der Seite «Hintergrundjobs» und
  Zeitplan (alle 7 Tage), **Zeitplan ab Werk aus** (F-11); reiht `trickplay.generate` mit
  Priorität 1 für jede Datei einer `store`-Bibliothek ohne Satz ein (Muster
  `chapters.images_backfill`). Dauer auf CT 190 bei 2 Prozessen rund **36 s je Stunde Video**, für
  den ganzen Bestand gut **13 Tage** Dauerlauf.

### 6.3 Ablage beim Erzeugen je Start

- Ordner `cache/trickplay-session/<mediaFileId>-<probedAt>/`, sonst gleich aufgebaut. Laufen zwei
  Wiedergaben derselben Datei, teilen sie den Satz.
- Die stündliche Pflege und zusätzlich der Wiedergabe-Abschluss (Muster
  `QueueChapterImagesAfterPlayback`) löschen einen Satz, sobald **10 Minuten** lang keine Wiedergabe
  der Datei mehr lief (`chapterPlayingWindow`-Regel). So überlebt er einen Spur- oder
  Qualitätswechsel und eine kurze Pause, nicht aber den Abend.
- Es gibt kein Nachholen und keinen Import-Auftrag.

### 6.4 Umstellen (F-12)

Stellt der Admin eine Bibliothek von «Speichern» auf «Bei jedem Start erzeugen» oder «Aus»,
**löscht die nächste stündliche Pflege die gespeicherten Sätze dieser Bibliothek**; wartende
Import- und Nachhol-Aufträge enden ohne Arbeit. Von «Aus» oder «Bei jedem Start» auf
«Speichern» entsteht nichts sofort; neue Starts und Dateien erzeugen, der Bestand kommt per
Nachholen. Ein gerade laufender Satz einer Wiedergabe bleibt bis zu deren Ende.

### 6.5 Pflege

Die stündliche `library.sweep` löscht (Muster `sweepChapterImages`): Sätze anderer Versionen,
verschwundener Dateien und gelöschter Titel, temporäre Sätze ohne Wiedergabe seit 10 Minuten,
gespeicherte Sätze aus Bibliotheken, die nicht mehr `store` sind, und `.tmp-`-Ordner älter als
6 Stunden.

### 6.6 Downloads (F-6)

Ein Download eines Films oder einer Folge nimmt die Blätter mit (rund 4 MB je Film), in jedem Modus
ausser «Aus». Fehlt der Satz beim Vorbereiten des Downloads, reiht der Download-Job
`trickplay.generate` ein und wartet höchstens auf dessen Ende; scheitert er, lädt der Download ohne
Vorschau. Beim Modus «Bei jedem Start erzeugen» kommen die Blätter in den Download-Ordner, nicht in
den Server-Cache. Die PWA legt sie mit dem Download ab und zeigt sie offline.

## 7. Datenformat

- **Bildpunkt alle 10 s**, Kachel **320 px breit**, Höhe nach Anzeige-Seitenverhältnis (gerade).
- **Blatt 10 × 10 Kacheln**, **WebP Qualität 75** (Messung 3.6). Das letzte Blatt ist kürzer
  (nur so viele Zeilen wie nötig).
- Die Kachel `k` zeigt den Bildpunkt `k`; seine Zeit steht im Manifest (`pointsMs`), weil sie am
  Keyframe liegt und nicht genau bei `k × 10 s`. Die App wählt für eine Zeigerposition den letzten
  Bildpunkt mit `pointsMs ≤ Position`.
- Warum Blätter statt Einzelbilder: ein Abruf je 16 Minuten statt je 10 s, wenige Dateien im Cache
  (8 statt 720 je Film), Standard bei Jellyfin und Emby.

## 8. Server, Vertrag und Datenbank

### 8.1 Pakete

| Paket | Änderung |
|---|---|
| `transcode` | `Runner.GrabTrickplayFrames` (ffmpeg-Aufruf aus 3.2, Tonemapping wie `GrabFrameArgs`), `TrickplaySheetArgs` (Kacheln zu WebP über `tile` aus den Einzelbildern); `os/exec` bleibt hier (Regel 3) |
| `library` | `trickplay.go`: Jobarten `trickplay.generate` und `trickplay.backfill`, Bildpunkte aus dem Keyframe-Index, Ablage je Modus, Pflege in `library.sweep`, Auslöser im Probe, `QueueTrickplayForPlayback` |
| `admin` | Bibliotheksformular: Feld `trickplayMode` lesen und schreiben |
| `offline` | Manifest des Downloads mit den Blättern (6.6) |
| `playback` | `Start` reiht ein (5.1), `PlaybackInfo.trickplay` aus dem Manifest |
| `jobs` | Pool `trickplay` mit einem Arbeiter |
| `api` | Handler für die Blätter (Regel 6) |

### 8.2 Vertrag (vorläufig 1.22.0, additiv)

- `PlaybackInfo.trickplay` (optional): `{ version, intervalMs, tileWidth, tileHeight, columns,
  rows, pointsMs[], sheets: [{ index, url }], complete }`. Fehlt das Feld, gibt es (noch) keine
  Vorschau. `complete: false` heisst: weitere Blätter folgen.
- `GET /items/{id}/trickplay?mediaFileId=&v=` (`getTrickplay`, JSON über den strict server,
  `x-visibility: scoped`): derselbe Inhalt für das Nachfragen, solange `complete` falsch ist.
- `GET /items/{id}/trickplay/{sheet}?mediaFileId=&v=` (`getTrickplaySheet`, `http.Handler`,
  `x-visibility: scoped`, `image/webp`): Caching wie `getChapterImage` (`immutable` mit passendem
  `v`). Unbekannter, unsichtbarer oder altersgesperrter Titel, fremde Datei, fehlendes Blatt → 404.
- `Library.trickplayMode` (`store` | `per_start` | `off`) in den Admin-Operationen der
  Bibliotheken (lesen, anlegen, ändern), nur bei `movies` und `series`.
- Das Download-Manifest (`DownloadManifest`) bekommt optional dasselbe `trickplay` mit relativen
  Pfaden der Blätter.
- Beide neuen Operationen im Vertragstest (`TestContractCoversEveryOperation`).

### 8.3 Datenbank

**Eine Migration** (vorläufig 00041, nach Vorlage, nur vorwärts):
`ALTER TABLE libraries ADD COLUMN trickplay_mode TEXT NOT NULL DEFAULT 'store' CHECK (trickplay_mode
IN ('store', 'per_start', 'off'))`. Bestehende Film- und Serienbibliotheken stehen damit auf
«Speichern» (F-11); bei Hörbuch- und Podcast-Bibliotheken bleibt die Spalte ohne Wirkung. Der
Zustand der Sätze steckt wie bei den Kapitelbildern im Cache-Ordner (fertig, teilweise, fehlt), die
Version ist `probedAt`; die Jobs nutzen die Tabelle `jobs`. Eingefrorener DB-Stand in `testdata/db/`
nach Regel 5.

## 9. Die App

- **Zeitleiste (`web/src/ui/SeekBar.tsx`):** Das Vorschaufeld über dem Zeiger zeigt die Kachel der
  Stelle (Ausschnitt des Blatts als CSS-Hintergrund, `background-position`), darunter wie heute
  Zeit und Kapitelname. Ohne Kachel an der Stelle (Blatt noch nicht fertig) zeigt es wie heute
  das Kapitelbild, sonst nur Zeit und Name. Gilt für Maus, Finger und ←/→ bei Fokus auf der
  Zeitleiste; Fernbedienung (C-4) ebenso.
- **Laden:** nur das Blatt unter dem Zeiger und das nächste; der Browser-Cache hält sie
  (`immutable`). Solange `complete` falsch ist, fragt der Player `getTrickplay` alle 10 s nach,
  höchstens 5 Minuten lang.
- **Offline:** Die PWA speichert die Blätter mit dem Download und zeigt sie aus dem Speicher (6.6).
- **Verwaltung, Bibliotheksformular:** neues Auswahlfeld «Vorschau beim Spulen» mit «Speichern»,
  «Bei jedem Start erzeugen» und «Aus», darunter ein Satz zu Speicherbedarf und Dauer; nur bei Film-
  und Serienbibliotheken. Die Seite «Hintergrundjobs» zeigt die Jobarten `trickplay.generate` und
  `trickplay.backfill` mit Knopf und Zeitplan wie die Kapitelbilder.
- Texte nur aus `locales`, Farben nur aus Tokens (Regel 9); keine neue Seite, also kein neuer
  Tastaturtest; die bestehenden Tests der Abspielseite (Kachel bei ←/→) und des
  Bibliotheksformulars (neues Feld per Tastatur) werden erweitert (Regel 8).

## 10. Harte Regeln

| Regel | Wie |
|---|---|
| 2 Titelabfragen über `visibility` | Blätter und Manifest nur über `vis.MediaFile` des aktiven Profils; sonst 404 |
| 3 `os/exec` nur in `transcode` | alle ffmpeg-Aufrufe als Methoden am `Runner`, Argumentliste, `CommandContext` |
| 4 Dateizugriff über `paths` | Mediendatei und Cache-Ordner über `paths` |
| 6 Streaming als `http.Handler` | `getTrickplaySheet` mit Beschreibung im Vertrag |
| 7 `x-visibility` | beide Operationen `scoped`, Vertragstest |
| 9 Farben und Texte | Vorschaufeld nur mit Tokens und `locales` |
| 11 Nicht-Ziele | berührt keins |
| 12 Entscheid im Server | Sichtbarkeit und Version entscheidet der Server, die App zeigt nur |

## 11. Fragen an Marco und seine Antworten (8. November 2026)

| Nr. | Frage | Empfehlung | Antwort |
|---|---|---|---|
| F-1 | Speichern: dauerhaft, mit Ablauf oder nicht? | dauerhaft | **Einstellung**: speichern oder bei jedem Start erzeugen (6.1) |
| F-2 | Genauigkeit: nächstes Keyframe oder bildgenau? | Keyframe | **Nächstes Keyframe** |
| F-3 | Abstand 10 s? | 10 s | **10 s** |
| F-4 | Bildgrösse 320 px? | 320 px | **320 px** |
| F-5 | Nachholen für den Bestand? | Knopf, Zeitplan aus | **Beim Speichern: Knopf, Zeitplan und neue Dateien, analog Emby** |
| F-6 | Downloads mit Vorschau? | ja | **Ja** |
| F-7 | Zwei ffmpeg-Prozesse während einer Wiedergabe? | zwei | **Zwei** |
| F-8 | Auch beim Einlesen neuer Dateien? | nein | **Ja, beim Speichern** (siehe F-5) |
| F-9 | Alle Profile, auch Kinder? | ja | **Ja** |
| F-10 | Wo liegt die Einstellung? | je Bibliothek | **Je Bibliothek** |
| F-11 | Ab Werk? | Speichern, Zeitplan aus | **Speichern, Zeitplan aus** |
| F-12 | Gespeicherte Bilder beim Umstellen weg «Speichern»? | löschen | **Löschen** |

## 12. Bauplan

Eine Stufe, ein Release (Vertrag vorläufig 1.22.0, Migration vorläufig 00041), nach Konzept 17.

| Schritt | Inhalt | Tage |
|---|---|---|
| 0 | Frage 25 und ADR 0027, Idee 05 auf «übernommen» | 0,5 |
| 1 | `transcode`: Bildpunkt-Aufruf, Blatt-Aufruf, Tests mit den Testmedien (SDR, HDR, anamorph) | 1 |
| 2 | Migration, `admin`: `trickplayMode` im Bibliotheksformular (Server) | 0,5 |
| 3 | `library`: Jobs, Bildpunkte aus dem Keyframe-Index, Ablage je Modus, Auslöser Start/Import/Nachholen, Pflege, Pool `trickplay` | 2 |
| 4 | Vertrag, Handler, `PlaybackInfo.trickplay`, `playback.Start`, Download-Manifest, Vertragstest | 1,5 |
| 5 | App: `SeekBar` mit Kachel, Nachfragen, Offline-Ablage, Feld im Bibliotheksformular, Storybook, Playwright | 1,5 |
| 6 | Messung auf CT 190 (Start, Blattzeit, Last neben CPU- und QSV-Transcode), `docs/perf/18-trickplay.md` | 0,5 |
| | **Summe** | **7,5** |

## Anhang: Messskripte

`docs/perf/18-trickplay/`: `bench.py` (lokal, Zeit, CPU und gelesene Bytes je Weg), `ct-bench.sh`
und `ct-run.sh` (CT 190 im Container, NFS-Bytes aus `/proc/self/mountstats`), `ct-extra.sh`
(HDR und Last neben Transcode), `sheets.sh` (Blattgrössen). Bilder aus Filmen kommen nie ins
Repository.
