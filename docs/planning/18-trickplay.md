# Konzept: Vorschau beim Spulen (Trickplay) für Filme und Serien

- **Status: Fassung 1 (8. November 2026), Entwurf mit Messungen, Fragen F-1 bis F-9 offen
  (Abschnitt 11).** Auslöser ist Marcos Auftrag vom 8. November 2026: Vorschaubilder für Filme und
  Folgen, erzeugt **sobald eine Folge oder ein Film gestartet wird**; dazu die Fragen, wie lange
  das dauert und ob die Bilder gespeichert werden. Grundlage ist Idee 05
  (`docs/ideas/05-trickplay.md`, aus Konzept 13 V-21, Frage 19 F-7 «später»).
  **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Verankert gegen:** `main` **`6c05eda7`** (Merge PR #125, Release 1.19.2), Vertragsfassung
  **1.19.0** (`api/openapi.yaml:4`), letzte Migration **`00039_radio_stage2.sql`**, letzte ADR in
  `main` **`0025`**, letzte Frage im Entscheidungsprotokoll **24**. Reserviert und nicht gebaut:
  Konzept 17 Stufe 2 (Vertrag 1.20.0) und Stufe 3 (Vertrag 1.21.0, Migration 00040, ADR 0026).
- **Nummern:** Dieses Papier ist **Konzept 18** und reserviert **Frage 25** und **ADR 0027**
  («Trickplay: Erzeugen beim Start, Springen an Keyframes, Ablage im Cache»). Vertrag: vorläufig
  **1.22.0 additiv**, endgültig beim Baubeginn. **Keine Migration** (Abschnitt 8.3).
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
- **Empfehlung:** erzeugen beim Start, **dauerhaft im Cache speichern** (bis sich die Datei ändert
  oder verschwindet), Bildpunkt **am nächsten Keyframe** statt bildgenau, Nachholen für den ganzen
  Bestand als Knopf mit ausgeschaltetem Zeitplan wie bei den Kapitelbildern.

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
Start einer Wiedergabe (playback.Start)
   └─ trickplay.generate für die Mediendatei einreihen (falls noch kein fertiger Satz da ist)
        └─ Worker im Pool «trickplay» (1 Arbeiter, 2 ffmpeg-Prozesse)
             ├─ Bildpunkte bestimmen: alle 10 s das nächste Keyframe aus dem Keyframe-Index
             ├─ Reihenfolge: Blatt der Startposition zuerst, dann vorwärts, dann rückwärts
             ├─ je Bildpunkt: transcode.Runner.GrabTrickplayFrame (Sprung, ein Bild)
             └─ je fertiges Blatt: Kacheln zu WebP fügen, ablegen, Blatt freigeben
App: PlaybackInfo.trickplay sagt Raster und fertige Blätter; der Player fragt nach,
     solange der Satz nicht fertig ist, und zeigt bis dahin das Kapitelbild.
```

## 5. Erzeugen beim Start (T-2)

1. **Auslöser:** `playback.Start` reiht nach dem Muster von `queueKeyframes` den Job
   `trickplay.generate` (Payload `{"mediaFileId", "startMs"}`, Schlüssel `trickplay:<id>`) ein,
   wenn die Datei ein Film oder eine Folge ist (`chapterKind`), einen Video-Stream hat und noch
   kein fertiger Satz der aktuellen Version im Cache liegt. Ein schon wartender Job derselben
   Datei wird nur angehoben. Hörbücher, Podcasts und Radio bekommen nie Trickplay.
2. **Priorität:** über allen Hintergrundjobs ausser den Keyframe-Aufträgen für eine Wiedergabe;
   läuft wie diese **trotz Pause** der Jobart (Muster ADR 0022, `EnqueueOverridingPause`), weil
   der Zuschauer gerade wartet. Höchstens **ein** Trickplay-Job gleichzeitig (eigener Pool
   `trickplay` mit einem Arbeiter), damit zwei gleichzeitige Starts den CT nicht doppelt belasten.
   Der Job nutzt **zwei** ffmpeg-Prozesse; läuft gerade ein Transcode mit CPU-Tonemapping oder
   ein reiner CPU-Transcode, nur **einen**.
3. **Reihenfolge:** zuerst das Blatt, in dem die Startposition liegt, dann die folgenden Blätter,
   zuletzt die davor. So hat ein Zuschauer, der mitten im Film weiterschaut, die Vorschau dort
   zuerst. Ein Blatt (100 Bilder) braucht auf CT 190 rund **5 s** (WEB-DL) bis **10 s** (Bluray).
4. **Teilergebnis:** Jedes fertige Blatt ist sofort abrufbar. Bricht der Job ab (Neustart des
   Servers, Datei offline), bleiben fertige Blätter erhalten, und der nächste Lauf macht nur den
   Rest.
5. **Bildpunkte:** Für jeden 10-s-Abschnitt das **nächste** Keyframe aus dem gespeicherten
   Keyframe-Index (`media_files.keyframes`, ADR 0022; auf CT 190 haben 3 019 von 3 106 Dateien
   einen). Der Sprung geht dann genau auf dieses Keyframe, ffmpeg dekodiert ein Bild. Liegen zwei
   Abschnitte auf demselben Keyframe (Abstand über 10 s), bekommt der zweite das folgende Keyframe;
   gibt es keins im Abschnitt, wiederholt er das Bild. Ohne Index (`none`, noch ausstehend)
   springt der Job mit `-noaccurate_seek` an den Abschnittsanfang (Keyframe davor, Messung 3.2).
6. **Schwarze Bilder** werden behalten: Die Vorschau zeigt, was an der Stelle ist; anders als
   beim Kapitelbild gibt es keinen Ersatzpunkt.
7. **HDR und Dolby Vision:** dieselbe CPU-Tonemapping-Kette wie `GrabFrameArgs` (Messung 3.5).
8. **Fehler:** Datei nicht lesbar → Muster der Kapitelbilder (`chapterUnreadableRetry` 1 h,
   Aufgeben nach 7 Tagen); ein einzelner kaputter Bildpunkt wird schwarz gefüllt und protokolliert.

## 6. Speichern oder nicht (T-4)

| Variante | Was | Dauer beim Start | Speicher | NAS und CPU |
|---|---|---|---|---|
| **A Dauerhaft speichern** (Empfehlung) | Blätter im Cache, bis sich die Datei ändert oder verschwindet | beim **ersten** Start 20 bis 45 s (4K HDR bis 2 min), danach **sofort** | ~4 MB je Film, ~1,5 MB je Folge; nur Geschautes: unter 2 GB im Jahr | einmal je Datei |
| **B Speichern mit Ablauf** | wie A, aber Blätter von Dateien, die N Tage nicht gespielt wurden, löscht die stündliche Pflege | beim ersten Start nach Ablauf wieder 20 bis 45 s | gedeckelt | gelegentlich wieder |
| **C Nicht speichern** | Blätter nur für die laufende Wiedergabe (im Arbeitsspeicher oder `/tmp`), nach dem Ende gelöscht | **bei jedem Start** 20 bis 45 s ohne vollständige Vorschau | keiner | bei jedem Start 1 bis 2 CPU-Minuten und 350 bis 550 MB NAS |

**Empfehlung A.** Die Bilder sind so klein, dass Speicher kein Argument ist; C verschwendet Arbeit
bei jeder Wiederholung und lässt jede zweite Sichtung genau dort ohne Vorschau, wo man oft spult
(Weiterschauen). B lohnt sich erst, wenn der Cache knapp wird; der Bild-Cache von CT 190 hat
heute 35 GB frei. Die Ablage richtet sich nach den Kapitelbildern:

- Ordner `cache/trickplay/<mediaFileId>-<probedAt>/` mit `manifest.json` (Abstand, Kachelgrösse,
  Raster, Liste der Bildpunkte in ms) und `<blatt>.webp`. Ein neuer Probe ergibt einen neuen
  Ordner; die stündliche `library.sweep` löscht Ordner anderer Versionen, verschwundener Dateien und
  gelöschter Titel (Muster `sweepChapterImages`).
- Ein Ordner, der noch geschrieben wird, heisst `.tmp-…`; fertige Blätter werden einzeln per
  Umbenennen veröffentlicht, das Manifest zuletzt.
- **Nachholen** für den ganzen Bestand: Jobart `trickplay.backfill` mit Knopf auf der Seite
  «Hintergrundjobs» und Zeitplan **aus** ab Werk (Muster `chapters.images_backfill`, Priorität 1).
  Dauer auf CT 190 bei 2 Prozessen rund **36 s je Stunde Video**, für den ganzen Bestand also gut
  **13 Tage** Dauerlauf; darum nur auf ausdrücklichen Wunsch.

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
| `library` | `trickplay.go`: Jobarten `trickplay.generate` und `trickplay.backfill`, Bildpunkte aus dem Keyframe-Index, Ablage, Pflege in `library.sweep`, `QueueTrickplayForPlayback` |
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
- Beide Operationen im Vertragstest (`TestContractCoversEveryOperation`).

### 8.3 Datenbank

**Keine Migration.** Der Zustand steckt wie bei den Kapitelbildern im Cache-Ordner (fertig, teilweise,
fehlt); die Version ist `probedAt`. Die Jobs nutzen die vorhandene Tabelle `jobs`.

## 9. Die App

- **Zeitleiste (`web/src/ui/SeekBar.tsx`):** Das Vorschaufeld über dem Zeiger zeigt die Kachel der
  Stelle (Ausschnitt des Blatts als CSS-Hintergrund, `background-position`), darunter wie heute
  Zeit und Kapitelname. Ohne Kachel an der Stelle (Blatt noch nicht fertig) zeigt es wie heute
  das Kapitelbild, sonst nur Zeit und Name. Gilt für Maus, Finger und ←/→ bei Fokus auf der
  Zeitleiste; Fernbedienung (C-4) ebenso.
- **Laden:** nur das Blatt unter dem Zeiger und das nächste; der Browser-Cache hält sie
  (`immutable`). Solange `complete` falsch ist, fragt der Player `getTrickplay` alle 10 s nach,
  höchstens 5 Minuten lang.
- **Offline:** siehe F-6.
- Texte nur aus `locales`, Farben nur aus Tokens (Regel 9); keine neue Seite, also kein neuer
  Tastaturtest, aber der bestehende Test der Abspielseite prüft die Kachel bei ←/→ (Regel 8).

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

## 11. Fragen an Marco

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-1 | **Speichern:** A dauerhaft, B mit Ablauf nach N Tagen ohne Wiedergabe, oder C nicht speichern? | **A** (Abschnitt 6) |
| F-2 | **Genauigkeit:** Bild am **nächsten Keyframe** (bis zu einige Sekunden daneben, schnell) oder **bildgenau** (sechsfache CPU, Film auf CT 190 rund 3 bis 6 Minuten)? | **Keyframe** |
| F-3 | **Abstand:** alle **10 s** (Standard bei Jellyfin und Emby) oder enger, z. B. 5 s (doppelte Zeit und Grösse)? | **10 s** |
| F-4 | **Bildgrösse:** **320 px** (1,3 bis 1,8 MB/h) oder 480 px für grosse Bildschirme (3,3 MB/h)? | **320 px** |
| F-5 | **Nachholen** für den ganzen Bestand: Knopf mit Zeitplan aus (rund 13 Tage Dauerlauf, ~55 GB), oder gar nicht? | **Knopf, Zeitplan aus** |
| F-6 | **Offline:** Nimmt ein Download die Vorschau mit (rund 4 MB je Film)? | **Ja** |
| F-7 | **Zwei ffmpeg-Prozesse** während einer Wiedergabe, oder nur einer (halbe Geschwindigkeit, halbe Last)? | **Zwei** (Messung 3.4) |
| F-8 | **Auch für Titel ohne Wiedergabe** beim Einlesen neuer Dateien (wie Kapitelbilder)? | **Nein**, nur beim Start und per Nachholen |
| F-9 | Sollen **alle Profile** die Vorschau sehen, auch Kinderprofile? | **Ja**, es sind Bilder aus dem Titel, den das Profil ohnehin sieht |

## 12. Bauplan

Eine Stufe, ein Release (Vertrag vorläufig 1.22.0), nach Konzept 17.

| Schritt | Inhalt | Tage |
|---|---|---|
| 0 | Frage 25 und ADR 0027, Idee 05 auf «beauftragt» | 0,5 |
| 1 | `transcode`: Bildpunkt-Aufruf, Blatt-Aufruf, Tests mit den Testmedien (SDR, HDR, anamorph) | 1 |
| 2 | `library`: Jobs, Bildpunkte aus dem Keyframe-Index, Ablage, Pflege, Pool `trickplay` | 1,5 |
| 3 | Vertrag, Handler, `PlaybackInfo.trickplay`, `playback.Start`, Vertragstest | 1 |
| 4 | App: `SeekBar` mit Kachel, Nachfragen, Storybook, Playwright | 1 |
| 5 | Messung auf CT 190 (Start, Blattzeit, Last neben Transcode), `docs/perf/18-trickplay.md` | 0,5 |
| | **Summe** | **5,5** |

## Anhang: Messskripte

`docs/perf/18-trickplay/`: `bench.py` (lokal, Zeit, CPU und gelesene Bytes je Weg), `ct-bench.sh`
und `ct-run.sh` (CT 190 im Container, NFS-Bytes aus `/proc/self/mountstats`), `ct-extra.sh`
(HDR und Last neben Transcode), `sheets.sh` (Blattgrössen). Bilder aus Filmen kommen nie ins
Repository.
