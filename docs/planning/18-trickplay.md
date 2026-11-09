# Konzept 18: Vorschau beim Spulen (Trickplay), User-Empfehlungen und Raster-Feinschliff

- **Status: Fassung 9 (9. November 2026), vier Teile:** A «Trickplay» (Abschnitte 0 bis 13),
  B «User-Empfehlungen von Profil zu Profil» (14, 15), C «Raster-Feinschliff» (16), D
  «Streamübersicht für alles, was läuft» (17). Alle Fragen sind entschieden, O-1 am 9. November 2026 (11.2). Eingetragen als **Frage 25** im Entscheidungsprotokoll.
  **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Auslöser:** Marcos Auftrag vom 8. November 2026: Vorschaubilder für Filme und Folgen, erzeugt
  **sobald eine Folge oder ein Film gestartet wird**; dazu die Fragen, wie lange das dauert und ob
  die Bilder gespeichert werden. Grundlage ist Idee 05 (`docs/ideas/05-trickplay.md`, aus Konzept
  13 V-21, Frage 19 F-7 «später»). Teil B, C und D kamen auf Marcos Wunsch dazu.
- **Verlauf:**
  - Fassung 1 bis 3 (8. November 2026): Messungen, Marcos Antworten F-1 bis F-12, Kritiker-Review
    von Fassung 2 (REVISE, 1 Blocker, 8 Major, 13 Minor; Abschnitt 13).
  - Fassung 4 (8. November 2026): Einstellungen je Bibliothek getrennt für Filme und Serien
    (F-13 bis F-15, Abschnitt 6).
  - Fassung 5 (8. November 2026): Teil B mit Marcos Antworten U-F1 bis U-F8 (Abschnitt 14).
  - Fassung 6 (8. November 2026): zweites Kritiker-Review über Fassung 4 und 5 (REVISE, 1 Blocker,
    8 Major, 12 Minor; Abschnitt 15), mit Marcos Antworten F-16 und U-F9 bis U-F11 (Ausnahme vom
    Nicht-Ziel, Gäste nur lesen, alte Einträge verwerfen, Vorlauf gedrosselt).
  - Fassung 7 (9. November 2026): Teil C (Abschnitt 16): Titel unten bündig, grössere Beschriftung
    bei «Meine Medien», alles laden statt «Mehr laden» auf allen Listenseiten (nicht in den Reihen der
    Startseite). Marco hat am Mockup **Variante B**
    und eine Beschriftung von **26 px** gewählt (zuerst 40 px, dann «doch etwas zu gross»). Teil C
    hat noch kein Kritiker gesehen.
  - Fassung 8 (9. November 2026): Teil D (Abschnitt 17): die Streamübersicht zeigt alles, was
    läuft, in einer Liste mit Filterknöpfen; Marcos Antworten D-1 bis D-3. Marco hat das
    Streams-Mockup am 9. November 2026 ohne Änderung abgenommen («Passt so»). Teil D ist gegen
    `main` `43eb1f8a` (Konzept 17 Stufe 3, Vertrag 1.21.0, Migration 00040) geschrieben.
  - Fassung 9 (9. November 2026): Kritiker-Review von Teil C und D gegen `main` `e40ecf1c`
    (Release 1.22.0; REVISE, kein Blocker, 6 Major, 11 Minor; Abschnitt 18) eingearbeitet; O-1
    hingenommen. Vertrag vorläufig **1.23.0**, weil 1.22.0 vergeben ist.
  - Nachtrag 9. November 2026 (Patch **1.23.1**, nach der Abnahme von 1.23.0): Marco hat
    entschieden, dass **«Aus» zuoberst steht und ab Werk gilt**, in beiden Gruppen; Bibliotheken,
    die noch auf «Im Voraus» stehen, stellt Migration **00043** auf «Aus». Das ersetzt «Im Voraus»
    ab Werk aus F-11 (Abschnitte 6.1, 8.3, 9 und 11).
- **Verankert gegen:** `main` **`6c05eda7`** (Merge PR #125, Release 1.19.2), Vertragsfassung
  **1.19.0** (`api/openapi.yaml:4`), letzte Migration **`00039_radio_stage2.sql`**, letzte ADR in
  `main` **`0025`**, letzte Frage im Entscheidungsprotokoll **24**. Reserviert und nicht gebaut:
  Konzept 17 Stufe 2 (Vertrag 1.20.0) und Stufe 3 (Vertrag 1.21.0, Migration 00040, ADR 0026).
  Nachgeführt am 8. November 2026 auf `main` **`e0299fdd`** (Konzept 17 Stufe 2 gemergt, Vertrag
  **1.20.0**, PR #127); Stufe 2 berührt weder Trickplay noch die Empfehlungen.
  Teil C und D sind am 9. November 2026 gegen `main` **`e40ecf1c`** (Release 1.22.0, PR #129,
  letzte Migration `00040`) geprüft; Teil A und B sind gegen diesen Stand nicht neu geprüft.
- **Nummern (endgültig seit Baubeginn am 9. November 2026):** Dieses Papier ist **Konzept 18** mit
  **Frage 25**, **ADR 0027** («Trickplay: Erzeugen beim Start, Springen an Keyframes, Einstellungen
  je Bibliothek», Teil A) und **ADR 0028** («User-Empfehlungen von Profil zu Profil», Teil B).
  Vertrag **1.23.0** (1.22.0 ist seit 9. November 2026 als Release vergeben), durch Teil B
  **brechend** (14.8), **zwei Migrationen**: **00041**
  (Teil B: `user_picks`, `picks_config`, `top_picks` weg, 14.8) und **00042** (Teil A: Spalten
  `libraries.trickplay_*`, 8.3). Wo der Text unten «vorläufig» sagt, gelten diese Nummern. ADR 0027 ändert
  ADR 0022 nicht: Trickplay nimmt **nicht** an der Pausen-Ausnahme teil (5.3). Teil D braucht
  keine Migration und ändert den Vertrag nur additiv (17.4); kommt er vor Teil A und B, erhält er
  eine eigene Minor-Version.
- `:Zeile`-Angaben beziehen sich auf `6c05eda7`; **nach Namen suchen, nicht nach Zeile.** Pfade
  ohne Präfix liegen unter `server/` (Go) oder `web/src/` (Web-App).
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).

> **Dieses Papier ändert keine Zeile Code.** Messskripte und Rohwerte liegen in
> `docs/perf/18-trickplay.md` und `docs/perf/18-trickplay/`.
>
> **Mockup** (Fassung 5): Design-Leinwand «Konzept 18» mit fünf Artboards. Teil A: Player mit
> Vorschau und gerafft entstehenden Blättern, Bibliotheksdialog mit Film- und Seriengruppe,
> Hintergrundjobs. Teil B: Titelseite mit Stern «Empfehlen» und Startseiten-Reihe,
> Verwaltungsseite «User-Empfehlungen» mit Ablauf in Tagen. Artefakt
> (privat): https://claude.ai/artifact/Thtf5mQnDkaSH273uFPsDk; Quellen der Artboards in
> `docs/planning/mockups/18-trickplay/` (laufen nur in der Leinwand). Die Bilder sind gezeichnete
> Platzhalter, keine Filmbilder.

## 0. Kurz gesagt

- **Es geht schnell, wenn man nicht die ganze Datei liest.** Der Server springt für jeden
  Zeitpunkt (alle 10 s) direkt an das nächste Keyframe und dekodiert nur dieses eine Bild. Auf
  CT 190 dauert das für eine **Folge von 61 Minuten 19 s**, für einen **Film von 2 Stunden 30 s
  (vier Prozesse) bis 42 s (zwei Prozesse)**; ein 4K-HDR-Film braucht mit Tonemapping etwa das
  Zweieinhalbfache. Das **erste Kachelblatt** (100 Bilder, gut
  16 Minuten Film rund um die Startposition) ist nach **5 bis 10 s** fertig.
- **Das NAS merkt bei 1080p wenig davon:** rund **1 MB je Bild**, also 350 MB für die Folge und
  590 MB für den Film, statt 1,2 GB bzw. 6,2 GB beim Lesen der ganzen Datei (dauert auf CT 190 31 s bzw. 160 s). Ein laufender Stream wird
  nicht gebremst (Abschnitt 3.4). Für 4K über das NAS ist die Last
  noch nicht gemessen (Bauschritt 8).
- **Speichern lohnt sich:** WebP mit 320 px Breite braucht **1,3 bis 1,8 MB je Stunde Video**.
  Ein Film belegt rund 4 MB, eine Folge 1 bis 2 MB. Selbst der ganze Bestand (rund 31 000 Stunden)
  käme auf etwa 55 GB, nur die geschauten Titel auf wenige GB. Jedes Mal neu erzeugen hiesse
  dagegen bei jedem Start wieder 20 bis 80 s CPU und einige hundert MB NAS-Lesen.
- **Marcos Entscheid (F-1 bis F-15):** Jede Videobibliothek stellt **Filme und Serien getrennt**
  ein, auch eine gemischte. Filme: **«Aus»** (ab Werk seit 1.23.1), **«Im Voraus»** (beim
  Einlesen und per Job, bis 1.23.0 ab Werk) oder **«Beim Start»** mit Haken **«Speichern»**. Serien dieselben drei und zusätzlich
  **«Beim Start alle Folgen»**: Die erste gestartete Folge löst die Vorschau für die übrigen Folgen
  aus, im Umfang **ganze Serie** (ab Werk), **Rest der Staffel** oder **nächste Folge**. Bildpunkt
  **am nächsten Keyframe**, alle **10 s**, **320 px**, zwei ffmpeg-Prozesse, alle Profile; Downloads
  nehmen die Vorschau mit.
- **Teil B, User-Empfehlungen (Abschnitt 14):** Jedes Profil empfiehlt Titel mit einem Klick auf den
  Stern der Titelseite, **anonym**; jeder Titel steht **einmal** in der Liste, **neueste zuerst**,
  für alle Profile, die ihn sehen dürfen. Nach **60 Tagen** ab der jüngsten Empfehlung fällt er
  heraus (einstellbar). Eigene Empfehlungen lassen sich zurücknehmen. Empfehlen dürfen die Profile
  des Besitzerkontos, alle anderen Konten lesen nur. Die Admin-Pflege mit Notiz, Zielprofil und
  Zeitfenster fällt weg, die heutigen Einträge werden verworfen. Dafür bekommt das Nicht-Ziel
  «Interaktion zwischen Profilen» eine Ausnahme nur für diese Empfehlungen.

## 1. Was Marco will

| Nr. | Marcos Wunsch (8. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| T-1 | Vorschaubilder auf der Zeitleiste (Trickplay) für Serienfolgen und Filme. | 5, 9 |
| T-2 | Die Daten entstehen, **sobald eine Folge oder ein Film gestartet wird**. | 5 |
| T-3 | Wissen, **wie lange** das dauert. | 3 |
| T-4 | Abwägen, ob die Bilder **gespeichert werden oder nicht**. | 6 |
| T-5 | Messen an einem echten FullHD-Film und einer echten Folge, sauber konzipieren. | 3, Anhang |
| T-6 | Profile empfehlen einander Titel, Ablauf nach 60 Tagen, Einstellungsseite (Teil B). | 14 |

## 2. Was sich gegenüber früheren Entscheiden ändert

| Früher | Neu | Warum |
|---|---|---|
| Frage 19, F-7: Trickplay **«später»**; Konzept 13 V-21 entfällt, Idee 05 | Trickplay wird gebaut (dieses Papier) | Marcos Auftrag vom 8. November 2026 |
| Idee 05 Abschnitt 7: «Ein Durchlauf liest die **ganze Datei**» | Ein Durchlauf liest **nur die Keyframes an den Bildpunkten**, rund 1 MB je Bild | Messung 3.2: der Sprung kostet 0,4 bis 1 MB, nicht 6 bis 23 MB, wenn ffmpeg mit `-fps_mode passthrough` läuft |
| Konzept 13, 11.6: Kapitelbilder **nie während der Wiedergabe**, erst 10 Minuten nach dem Ende | Trickplay läuft **während** der Wiedergabe, gleich nach dem Start (T-2) | Messung 3.4: ein QSV-Transcode daneben bleibt schneller als Echtzeit; die NAS-Last ist klein. Die Regel für Kapitelbilder bleibt unverändert |
| Spec «Non-Goals»: «Interaktion zwischen Profilen (Bewertungen, Empfehlungen untereinander)»; Spec-Status «Profil-Interaktion zurückgestellt», Plan 03 «Nicht eingeplant» | **Ausnahme seit 8. November 2026** nur für anonyme Empfehlungen (Teil B); Bewertungen, Kommentare und Nachrichten zwischen Profilen bleiben Nicht-Ziele | Marco, U-F9 |
| Spec E-2: der Hauptbenutzer pflegt Cat's Empfehlungen (Notiz, Zielprofil, Reihenfolge, Zeitraum) | Profile des Besitzerkontos empfehlen selbst, anonym, mit Ablauf; die Admin-Pflege fällt weg | Marco, U-F1 bis U-F11 |
| Konzept 17 D-10/F-17: Knopf «Zu den User-Empfehlungen» nur für den Admin, führt in die Verwaltung | Stern «Empfehlen» / «Empfohlen ✓» für alle empfehlenden Profile | Teil B ersetzt den Knopf; Konzept 17 9.3 trägt einen Hinweis |
| Bibliotheken haben keine Einstellung zu erzeugten Bildern | Neue Einstellungen **«Vorschau beim Spulen»** je Videobibliothek (`LibraryKind` `video`), getrennt für Filme und Serien (F-1, F-10, F-13) | Marco: «Parameter in den Einstellungen», «analog Emby» |

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
Das ist nicht gemessen und gehört in die Messung von Bauschritt 8.

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
Videobibliothek, getrennt für Filme (movie) und Folgen (episode):
  ahead          «Im Voraus»: Start, neue Datei, Nachholen; speichert immer
  on_start       «Beim Start»: nur Start; speichert, wenn der Haken «Speichern» gesetzt ist
  series_start   «Beim Start alle Folgen» (nur Serien): Start der Folge und Vorlauf über die
                 Serie im eingestellten Umfang; speichert immer
  off            «Aus»
Download        fertigen Satz in den Download-Ordner kopieren, fehlenden zuerst erzeugen
Pflege          library.sweep und Wiedergabe-Pflege (6.7)

trickplay.generate (Pool «trickplay», 1 Arbeiter, 2 ffmpeg-Prozesse)
   ├─ Raster und Bildpunkte rechnen (Keyframe-Index, Dauer) → manifest.json zuerst
   ├─ Blattfolge ab startMs: Blatt der Startposition, dann vorwärts, dann rückwärts
   ├─ je Bildpunkt: transcode.Runner (Sprung auf das Keyframe, ein Bild)
   ├─ je Blatt: Kacheln zu WebP, unter Zwischennamen schreiben, umbenennen
   ├─ nach jedem Blatt: Version, Einstellung, wartender Start-Auftrag, Speicherplatz prüfen
   └─ zuletzt: Merker «complete»
App: PlaybackInfo.trickplay kommt bei jedem Start mit (ausser «Aus»), auch leer; der Player
     fragt nach, solange state queued oder running ist, und zeigt bis dahin das Kapitelbild.
```

## 5. Erzeugen (T-2)

### 5.1 Satz, Version und Zustand

- **Ein Satz** ist ein Ordner je Datei und Dateiversion: `<mediaFileId>-<size>-<mtime>` (Version
  wie beim Keyframe-Index, `library.FileVersion`; ein neuer Probe allein, etwa nach kurz fehlender
  Freigabe oder einer Titel-Aktualisierung, verwirft ihn nicht). Er enthält:
  - `manifest.json`, **zuerst** geschrieben: Abstand, Kachelgrösse, Spalten, Zeilen, `pointsMs`,
    Quelle der Bildpunkte (`index` oder `fallback`);
  - `<blatt>.webp`, je Blatt unter einem Zwischennamen geschrieben und dann umbenannt; ein
    vorhandenes Blatt ist fertig;
  - `complete`, ein leerer Merker, **zuletzt** geschrieben.
- **Raster und Bildpunkte** sind eine reine Funktion von Keyframe-Index und Dauer (Paket
  `library`). `playback.Start` rechnet sie selbst, wenn noch kein Manifest da ist, damit die App
  vom ersten Start an Raster und Zustand kennt.
- **Zustand** für die App: `queued`, `running`, `complete`, `failed` (aus Merker, Blättern und dem
  Auftrag in `jobs`).
- Bricht ein Lauf ab (Neustart, Datei offline), bleiben Manifest und fertige Blätter; der nächste
  Lauf macht nur die fehlenden Blätter.
- Hat eine Datei einen Satz und ändert sie sich wirklich (Grösse oder mtime), reiht der Probe in
  einer speichernden Einstellung einen neuen Auftrag ein (Muster `hadChapterImages`).

### 5.2 Auslöser

1. **Start:** `playback.Start` reiht `trickplay.generate` ein (Payload `{"mediaFileId",
   "startMs", "origin"}`, Schlüssel `trickplay:<id>`), wenn die Einstellung der Titelart nicht `off`
   ist, der Titel ein Film oder eine Folge mit Video-Stream ist (`chapterKind`) und kein
   vollständiger Satz vorliegt. Eingereiht wird **immer über `EnqueueOverridingWindow`**: Das hebt
   die Priorität eines wartenden Auftrags desselben Schlüssels und setzt nur `override_window`, nie
   `override_pause` (5.3). Gibt es schon einen Auftrag, schreibt `Start` `startMs` und
   `origin: start` in dessen Payload (wartend über `SetQueuedPayload`, laufend über die Payload, die
   der Job nach jedem Blatt neu liest). Hörbücher, Podcasts und Radio bekommen nie Trickplay.
2. **Neue Datei** (nur `ahead`, F-5, F-8): der Probe einer Datei, die ein Scan nach einem
   vollständigen Scan der Bibliothek hinzufügte oder änderte, reiht den Auftrag mit **Priorität 2**
   ein (Muster `probeJob`/`chapterImages`). Er wartet nicht auf das Ende einer laufenden Wiedergabe.
3. **Nachholen** (nur `ahead`): 6.2.
4. **Serienvorlauf** (nur `series_start`): 6.4.
5. **Download:** 6.8.

**Die Einstellung steht nicht im Auftrag, die Herkunft schon.** `origin` ist `start`, `import`,
`backfill`, `series` oder `download`; ein späterer Auslöser mit höherem Vorrang überschreibt sie
(`start` vor `download` vor `series` vor `import` vor `backfill`). Der Job liest die Einstellung der
Titelart beim Start aus der Bibliothek und nach jedem Blatt neu. Daraus folgt der Ziel-Ordner
(gespeichert bei `ahead`, `series_start` und `on_start` mit Haken, sonst temporär; 6.3, 6.5) und ob
der Auftrag weiterläuft: Er endet ohne weitere Arbeit, wenn die aktuelle Einstellung seine Herkunft
nicht mehr auslöst (`import` und `backfill` nur bei `ahead`, `series` nur bei `series_start`), wenn
sie `off` ist, oder wenn sie nicht speichert und keine Wiedergabe der Datei mehr offen ist. So
endet nach einem Wechsel von «Im Voraus» auf «Beim Start» ein laufendes Nachholen nach dem
nächsten Blatt.

### 5.3 Vorrang, Pause und Last

- **Pool `trickplay` mit einem Arbeiter.** Höchstens ein Satz entsteht gleichzeitig.
- **Start-Aufträge** laufen mit Priorität 9, Serienvorlauf mit 3, Import-Aufträge mit 2,
  Nachholen mit 1. Weil ein laufender Auftrag nie abgebrochen wird, prüft ein Auftrag aus Import,
  Nachholen, Serienvorlauf oder Download **nach jedem Blatt**, ob im Pool ein Start-Auftrag wartet;
  dann gibt er mit `jobs.RetryLater(0, …)` ab, und seine fertigen Blätter bleiben. Ein Start wartet
  so höchstens ein Blatt (5 bis 10 s, bei 4K HDR rund 30 s).
- **Zwei Zuschauer, zwei Titel:** Der zweite Start wartet, bis der erste Satz fertig ist (20 bis
  45 s), weil Start-Aufträge einander nicht abgeben. Siehe O-1.
- **Keine Pausen-Ausnahme.** Trickplay ist eine Annehmlichkeit; pausiert der Admin die Jobart,
  pausiert sie ganz. Die Pausen-Ausnahme und ihr Deckel `MaxPauseOverrides = 3` (ADR 0022, O-7)
  bleiben den Keyframe-Aufträgen vorbehalten. Darum nutzt Trickplay **nie**
  `RaiseQueuedJobForPlayback` oder `EnqueueOverridingPause` (beide setzen `override_pause`, Abfrage
  in `queries/system/jobs.sql`), sondern nur `Enqueue` und `EnqueueOverridingWindow`. Ein Test
  prüft: Eine pausierte Jobart bleibt pausiert, und die Zahl der Pausen-Ausnahmen ändert sich nicht.
- **Zwei ffmpeg-Prozesse** (F-7), nur **einer**, solange ein Transcode auf der CPU läuft (Tonemapping
  auf der CPU, reiner CPU-Transcode oder ein Download-Encode). Dazu bekommt `transcode.Manager`
  eine Abfrage `CPUHeavy()`; `library` sieht sie über ein Interface, geprüft je Blatt.
- **Zeitfenster:** `Windowable` gilt je Jobart (`jobs/registry.go`). `trickplay.generate` ist
  `Windowable`: Import-Aufträge warten auf das Fenster. Start-, Serienvorlauf- und
  Download-Aufträge derselben Jobart gehen über `EnqueueOverridingWindow` am Fenster vorbei, weil
  jemand gerade schaut. Das Fenster des Nachholens steht im Zeitplan von `trickplay.backfill`
  (Schedule-Typ). `trickplay.series` ist nicht `Windowable`.
- **Gedrosselter Serienvorlauf** (F-16): Aufträge mit `origin: series` ausser den nächsten zwei
  Folgen nutzen nur **einen** ffmpeg-Prozess, solange irgendeine Wiedergabe-Sitzung offen ist
  (dieselbe Abfrage wie `CPUHeavy`, um offene Sitzungen erweitert).

### 5.4 Bildpunkte (F-2 «nächstes Keyframe»)

- Für jeden 10-s-Abschnitt das **nächste** Keyframe aus dem Keyframe-Index
  (`media_files.keyframes`, ADR 0022; auf CT 190 haben 3 019 von 3 106 Dateien einen). Gesprungen
  wird auf **Keyframe + 1 ms**, weil der Index auf Millisekunden rundet und ein abgerundeter Wert
  in MP4 ein ganzes GOP zu früh landen liesse; mit `-noaccurate_seek` gibt ffmpeg dann genau dieses
  Keyframe aus.
- Liegen zwei Abschnitte auf demselben Keyframe (Abstand über 10 s), bekommt der zweite das
  folgende Keyframe; gibt es keins im Abschnitt, **kopiert** der Job die vorige Kachel (kein zweiter
  Sprung).
- **Ohne Index** (`none` oder noch ausstehend): Sprung an den Abschnittsanfang mit
  `-noaccurate_seek`; das Bild stammt dann vom Keyframe davor, bis zu einem Keyframe-Abstand früher
  als `pointsMs` sagt. Das ist bewusst hingenommen; das Manifest vermerkt `fallback`. Ein später
  eintreffender Index verbessert einen fertigen Satz nicht; `trickplay.backfill` baut
  `fallback`-Sätze neu, sobald die Datei einen Index hat.
- **Schwarze Bilder** bleiben: Die Vorschau zeigt, was an der Stelle ist.
- **HDR und Dolby Vision:** dieselbe CPU-Tonemapping-Kette wie `GrabFrameArgs` (Messung 3.5).

### 5.5 Fehler

- **Datei nicht lesbar** beim Beginn (`readable`): Start-, Vorlauf- und Download-Aufträge sowie solche ohne Speichern enden
  ohne Wiederholung (der nächste Start versucht es wieder); Import und Nachholen folgen dem Muster
  der Kapitelbilder (`chapterUnreadableRetry` 1 h, Aufgeben nach 7 Tagen).
- **Je Bildpunkt** höchstens **15 s** (ein hängendes NAS hält den einzigen Arbeiter nicht fest). Ein
  einzelner kaputter Bildpunkt wird schwarz gefüllt und protokolliert; nach **5 Fehlern in Folge**
  oder wenn `readable` mitten im Lauf scheitert, endet der Auftrag als `failed`, fertige Blätter
  bleiben.
- **Datei ändert sich während des Laufs:** Nach jedem Blatt prüft der Job die Version; stimmt sie
  nicht mehr, endet er (Muster `chapterChangedRetry`), der Probe reiht neu ein (5.1).

## 6. Einstellungen je Bibliothek (T-4, F-1, F-13 bis F-15)

### 6.1 Die Einstellungen

Jede Videobibliothek hat **zwei** Einstellungen «Vorschau beim Spulen», eine für **Filme** und eine
für **Serien**. Sie gelten auch in einer gemischten Bibliothek: Ein Film folgt der Film-Einstellung,
eine Folge der Serien-Einstellung (Titelart `movie` bzw. `episode`).

| Einstellung | Filme | Serien | Wann entsteht die Vorschau | Gespeichert |
|---|---|---|---|---|
| **Im Voraus** (`ahead`, bis 1.23.0 ab Werk) | ✓ | ✓ | beim Einlesen neuer Dateien, per Nachholen (Knopf, Zeitplan) und beim Start, falls noch nicht da | immer |
| **Beim Start** (`on_start`) mit Haken **«Speichern»** | ✓ | ✓ | beim Start des Titels | mit Haken dauerhaft, ohne Haken nur während der Wiedergabe |
| **Beim Start alle Folgen** (`series_start`) | – | ✓ | beim Start einer Folge für diese Folge und im Hintergrund für die übrigen im Umfang **ganze Serie** (ab Werk), **Rest der Staffel** oder **nächste Folge** | immer |
| **Aus** (`off`, ab Werk seit 1.23.1) | ✓ | ✓ | nie; Zeitleiste wie heute mit Kapitelbild | – |

Marcos Skizze vom 8. November 2026 nannte für Filme zwei und für Serien drei Möglichkeiten; «Aus»
bleibt aus F-1 und F-12 dabei. Der Haken «Speichern» ist ab Werk gesetzt.

**Nachtrag 9. November 2026 (1.23.1):** Marco hat nach der Abnahme von 1.23.0 entschieden, dass
**«Aus» ab Werk gilt** und in der Auswahl **zuoberst** steht (Filme: Aus, Im Voraus, Beim Start;
Serien: Aus, Im Voraus, Beim Start, Beim Start alle Folgen), und dass bestehende Bibliotheken mit
«Im Voraus» auf «Aus» wechseln (Migration 00043, je Art; «Speichern» und Umfang bleiben). Wartende
Aufträge solcher Bibliotheken enden ohne Arbeit, die nächste Pflege entfernt ihre gespeicherten
Vorschauen.

### 6.2 Abwägung

| Einstellung | Dauer beim Start | Speicher | NAS und CPU |
|---|---|---|---|
| Im Voraus | meist **sofort**, weil der Import die Vorschau schon erzeugt hat; sonst beim ersten Start 20 bis 45 s (4K HDR bis 2 min) | ~4 MB je Film, ~1,5 MB je Folge; ganzer Bestand 40 bis 55 GB | einmal je Datei, auch für nie geschaute |
| Beim Start, gespeichert | beim **ersten** Start 20 bis 45 s, danach sofort | nur Geschautes, unter 2 GB im Jahr | einmal je geschaute Datei |
| Beim Start, nicht gespeichert | **bei jedem Start** 20 bis 45 s, bis dahin Teilvorschau | nur während der Wiedergabe | jeder Start 20 bis 80 s CPU und 350 bis 600 MB NAS |
| Beim Start alle Folgen | erste Folge 20 bis 45 s, jede weitere meist sofort | Serie: rund 1,3 MB je Stunde, z. B. 220 Folgen à 42 min = 154 h ≈ 200 MB | Serie von 154 h: rund 50 min Arbeit und 55 GB NAS-Lesen, verteilt im Hintergrund |

### 6.3 Ablage gespeichert

- Ordner `cache/trickplay/<mediaFileId>-<size>-<mtime>/` (5.1).
- **Nachholen** (F-5, nur `ahead`): Jobart `trickplay.backfill` mit Knopf auf «Hintergrundjobs» und
  Zeitplan (alle 7 Tage), **Zeitplan ab Werk aus** (F-11); reiht `trickplay.generate` mit
  Priorität 1 für jede Datei ein, deren Titelart in ihrer Bibliothek auf `ahead` steht und die
  keinen vollständigen oder einen `fallback`-Satz hat (Muster `chapters.images_backfill`). Dauer auf
  CT 190 bei 2 Prozessen rund **20 s je Stunde Video** (gemessen 18,7 s für 61 min, 42,3 s für
  118 min, 58,2 s für 158 min), für den ganzen Bestand von rund 31 000 Stunden also **7 bis 8 Tage**
  Dauerlauf, mit Nachtfenster entsprechend länger.
- **Speicherplatz:** Unter einer Grenze von **10 GB oder 10 %** frei auf dem Cache-Laufwerk (statfs
  wie `admin/storage_linux.go`) stoppen Nachholen, Import und Serienvorlauf (Meldung im Protokoll
  und auf «Hintergrundjobs»); ein Start-Auftrag schreibt dann einen temporären Satz. Die
  Bibliotheksseite zeigt die Grösse der gespeicherten Sätze, getrennt für Filme und Serien.

### 6.4 Serienvorlauf (`series_start`)

- Startet eine Folge einer Serie mit dieser Einstellung, reiht `playback.Start` neben dem
  Start-Auftrag der Folge einen Auftrag `trickplay.series` ein (Pool `general`, Payload
  `{"seriesId", "episodeId"}`, Schlüssel `trickplay_series:<seriesId>`). Wartet schon einer, schreibt
  `Start` die neue `episodeId` hinein (`SetQueuedPayload`, die jüngste Folge gewinnt).
- Der Auftrag reiht für jede Folge im Umfang ohne vollständigen Satz `trickplay.generate` mit
  `origin: series` ein, über `EnqueueOverridingWindow` (hebt auch wartende Import- oder
  Nachhol-Aufträge derselben Datei an), und endet dann:
  - **die nächsten zwei Folgen** mit **Priorität 3** und voller Leistung (F-16);
  - **den Rest** mit Priorität 2, **gedrosselt** auf einen ffmpeg-Prozess, solange jemand schaut
    (5.3).
- **Welche Datei:** je Folge nur die Standardversion, die `playback` ohne Wahl nehmen würde, und nur
  vorhandene Dateien (`missing_since IS NULL`); fehlende und gelöschte Folgen werden übersprungen.
- **Umfang** (F-14, je Bibliothek wählbar): **ganze Serie** (ab Werk), **Rest der Staffel** (ab der
  gestarteten Folge bis Staffelende; beim Start der letzten Folge die erste Staffel danach) oder
  **nächste Folge** (die folgende Folge derselben Staffel, am Staffelende die erste der nächsten).
  Specials (Staffel 0) nur bei «ganze Serie», zuletzt.
- **Reihenfolge:** ab der gestarteten Folge vorwärts in Staffel- und Folgenreihenfolge, danach die
  Folgen davor.
- Der Vorlauf wird bei jedem Start einer Folge erneut eingereiht; er findet dann nur noch fehlende
  Folgen.
- Der Vorlauf ist ein Systemauftrag ohne Profil: Er erzeugt Bilder für alle Folgen der Serie, auch
  solche, die das startende Profil nicht sehen darf. Ausgeliefert werden sie trotzdem nur über die
  Sichtbarkeit (8.2, Regel 2).

### 6.5 Ablage nicht gespeichert

- Ordner `cache/trickplay-session/<mediaFileId>-<size>-<mtime>/`, aufgebaut wie 5.1. Zwei
  Wiedergaben derselben Datei teilen den Satz.
- **Gelöscht** wird er, wenn seit **10 Minuten** keine Wiedergabe-Sitzung der Datei mehr offen ist
  (nicht widerrufen und nicht abgelaufen, also auch keine pausierte). Massgeblich ist die
  Sitzungszeile, nicht die letzte Aktivität: Der Player meldet während einer Pause nichts
  (`useProgressReporter`). So überlebt der Satz Pausen, Spur- und Qualitätswechsel, nicht aber das
  Ende des Abends. Dazu dienen die Liste beendeter Wiedergaben (`ListEndedPlaybackFiles`) und ein
  Blick auf die Sitzungszeilen der Datei.

### 6.6 Umstellen (F-12)

- **Auf eine Einstellung, die nicht speichert** («Beim Start» ohne Haken, «Aus»): Die nächste
  Pflege löscht die gespeicherten Sätze dieser Titelart in der Bibliothek, aber erst, wenn keine
  Wiedergabe-Sitzung der jeweiligen Datei mehr offen ist. Wartende Import-, Nachhol- und
  Vorlauf-Aufträge enden ohne Arbeit (5.2).
- **Zwischen speichernden Einstellungen** («Im Voraus», «Beim Start» mit Haken, «Beim Start alle
  Folgen»): Vorhandene Sätze bleiben. Nur was die neue Einstellung auslöst, entsteht künftig.
- **Auf eine speichernde Einstellung:** Nichts entsteht sofort; bei «Im Voraus» holt das Nachholen
  den Bestand. Ein vorhandener temporärer Satz wird beim nächsten Lauf in den Speicher-Ordner
  **umbenannt** statt neu erzeugt.

### 6.7 Pflege

Gelöscht wird in der stündlichen `library.sweep` und in der minütlichen Wiedergabe-Pflege (Muster
`playback.sweepChapterImages`), ohne eigene Jobart: Sätze anderer Versionen, verschwundener Dateien
und gelöschter Titel; temporäre Sätze nach 6.5; gespeicherte Sätze nach 6.6. **Nie** einen Ordner,
an dem gerade ein Auftrag arbeitet (Abgleich mit dem laufenden Auftrag des Schlüssels
`trickplay:<id>` in `jobs`), und Zwischennamen von Blättern erst nach 6 Stunden.

### 6.8 Downloads (F-6)

- Ein Download eines Films oder einer Folge nimmt die Blätter mit (rund 4 MB je Film), bei jeder
  Einstellung ausser «Aus». Die Blätter werden beim Vorbereiten **in den Download-Ordner kopiert**;
  das Offline-Budget zählt sie mit, und die Pflege des Server-Caches kann sie nicht mehr entziehen.
- Fehlt ein vollständiger Satz, reiht der Download-Job den Auftrag ein (Priorität 5) und wartet wie
  beim Keyframe-Index (`errKeyframesWait`) höchstens **10 Minuten**; danach geht der Download ohne
  Vorschau weiter.
- Die PWA holt die Blätter über eine eigene Operation mit dem Download-Token (8.2) und zeigt sie
  offline aus ihrem Speicher.

### 6.9 Weitere Ideen, nicht aufgenommen

Besprochen am 8. November 2026, ohne Auftrag (Marco: keine Präferenz): Merkliste als Auslöser für
die Vorschau im Voraus, Ablauf oder Budget für gespeicherte Sätze, Knopf «Vorschau erzeugen/löschen»
je Titel in der Verwaltung. Nicht empfohlen: Bilder neben die Mediendateien legen (die Medien sind
schreibgeschützt eingebunden, `/media:ro`).

## 7. Datenformat

- **Bildpunkt alle 10 s** (F-3), Kachel **320 px breit** (F-4), Höhe nach Anzeige-Seitenverhältnis
  (gerade).
- **Blatt 10 × 10 Kacheln**, **WebP Qualität 75** (Messung 3.6). Das letzte Blatt hat nur so viele
  Zeilen wie nötig.
- Die Kachel `k` zeigt den Bildpunkt `k`; seine Zeit steht im Manifest (`pointsMs`), weil sie am
  Keyframe liegt und nicht genau bei `k × 10 s`. Die App wählt für eine Zeigerposition den letzten
  Bildpunkt mit `pointsMs ≤ Position`.
- Warum Blätter statt Einzelbilder: ein Abruf je 16 Minuten statt je 10 s, wenige Dateien im Cache
  (8 statt 720 je Film), Standard bei Jellyfin und Emby.

## 8. Server, Vertrag und Datenbank

### 8.1 Pakete

| Paket | Änderung |
|---|---|
| `transcode` | Bildpunkt-Aufruf (3.2, Sprung auf Keyframe + 1 ms, Tonemapping wie `GrabFrameArgs`), Blatt-Aufruf (`tile`, WebP); `Manager.CPUHeavy()`; `os/exec` nur hier (Regel 3) |
| `library` | `trickplay.go`: Raster und Bildpunkte (rein), Jobarten `trickplay.generate`, `trickplay.series` und `trickplay.backfill`, Ablage je Einstellung, Auslöser im Probe, Abgabe an Start-Aufträge, Speicherplatz-Grenze, Pflege, `QueueTrickplayForPlayback`, `TrickplayState` |
| `visibility` | `MediaFileLocation` bekommt `LibraryID` und die Trickplay-Einstellung der Titelart, damit `playback` sie ohne eigenes SQL auf `media_files` kennt (Regel 2) |
| `playback` | `Start` reiht ein und liefert `PlaybackInfo.trickplay` (5.2); Wiedergabe-Pflege löscht temporäre Sätze (6.5, 6.7) |
| `jobs` | Pool `trickplay` (ein Arbeiter) |
| `admin` | `trickplay` beim Anlegen und Ändern einer Bibliothek, Grösse der Sätze je Bibliothek und Titelart, geschätzter Speicherbedarf (9) |
| `offline` | Blätter kopieren, auf den Satz warten (6.8), Download-Manifest |
| `api` | Handler `getTrickplay`, `getTrickplaySheet`, `getDownloadTrickplaySheet` |
| `cmd/slouchcat` | Arbeiter des neuen Pools in `poolWorkers`/`startWorkers` |

### 8.2 Vertrag (vorläufig 1.23.0; Teil A additiv, Teil B brechend, 14.8)

- **`Trickplay`** (Schema): `{ version, state (queued|running|complete|failed), intervalMs,
  tileWidth, tileHeight, columns, rows, pointsMs[], sheets: [{ index, url }] }`.
- **`PlaybackInfo.trickplay`** (optional): fehlt nur bei der Einstellung «Aus», bei Titeln ohne Video und
  bei Hörbüchern und Podcasts. Beim ersten Start kommt es mit `state: queued` und `sheets: []`.
- **`GET /items/{id}/trickplay?mediaFileId=`** (`getTrickplay`, strict server, `x-visibility:
  scoped`): derselbe Inhalt zum Nachfragen.
- **`GET /items/{id}/trickplay/{sheet}?mediaFileId=&v=`** (`getTrickplaySheet`, `http.Handler`,
  `x-visibility: scoped`, `image/webp`): Caching wie `getChapterImage` (`immutable` mit passendem
  `v`). Unbekannter, unsichtbarer oder altersgesperrter Titel, fremde Datei, fehlendes Blatt → 404,
  nie 403.
- **`GET /downloads/{id}/trickplay/{sheet}`** (`getDownloadTrickplaySheet`, `http.Handler`,
  `x-visibility: scoped`, mit Download-Token wie `getDownloadFile` und `getDownloadSubtitle`).
- **`LibraryTrickplayEstimate`** nur in `getLibrary` (nicht in `listLibraries`), nur Admin: je
  Titelart Anzahl Titel und Laufzeit, Schätzung pro Film bzw. pro Folge (Mittel) und für die ganze
  Bibliothek, und der heute belegte Platz (9). Die Laufzeit zählt je vorhandener Mediendatei (auch
  mehrere Versionen), Dateien ohne `duration_ms` fallen weg. Den belegten Platz rechnet die
  stündliche `library.sweep` beim Durchgang durch die Sätze und legt ihn je Bibliothek und Titelart
  im Speicher ab; `getLibrary` liest nur diesen Wert.
- **`DownloadManifest.trickplay`** (optional): Raster, `pointsMs` und die Blattnummern.
- **`Library.trickplay`** (`LibraryTrickplay`): `{ movies: { mode: ahead|on_start|off, keep },
  series: { mode: ahead|on_start|series_start|off, keep, scope: series|season_rest|next_episode } }`
  in `createLibrary`, `updateLibrary` und den Lese-Operationen. `keep` gilt nur bei `on_start`,
  `scope` nur bei `series_start`; sonst werden sie gespeichert, aber nicht ausgewertet. Nur für
  `video`, bei `audiobooks` und `podcasts` `not_allowed` wie `defaultRating`.
- **Neuer Pool** in den Enums `JobPool` und `JobTypeInfoPool` (`trickplay`), danach `task gen`;
  Locale-Eintrag neben `"index": "Keyframe-Index"`.
- Alle drei neuen Operationen im Vertragstest (`TestContractCoversEveryOperation`).

### 8.3 Datenbank

**Eine Migration für Teil A** (vorläufig 00042, nach Vorlage, nur vorwärts; 00041 ist Teil B, 14.8),
fünf Spalten in `libraries`:

```sql
ALTER TABLE libraries ADD COLUMN trickplay_movies TEXT NOT NULL DEFAULT 'ahead'
    CHECK (trickplay_movies IN ('ahead', 'on_start', 'off'));
ALTER TABLE libraries ADD COLUMN trickplay_movies_keep INTEGER NOT NULL DEFAULT 1
    CHECK (trickplay_movies_keep IN (0, 1));
ALTER TABLE libraries ADD COLUMN trickplay_series TEXT NOT NULL DEFAULT 'ahead'
    CHECK (trickplay_series IN ('ahead', 'on_start', 'series_start', 'off'));
ALTER TABLE libraries ADD COLUMN trickplay_series_keep INTEGER NOT NULL DEFAULT 1
    CHECK (trickplay_series_keep IN (0, 1));
ALTER TABLE libraries ADD COLUMN trickplay_series_scope TEXT NOT NULL DEFAULT 'series'
    CHECK (trickplay_series_scope IN ('series', 'season_rest', 'next_episode'));
```

(Vorbilder für `ADD COLUMN` mit `CHECK` auf STRICT-Tabellen: 00032, 00039.) Bestehende
Videobibliotheken stehen damit auf «Im Voraus» (F-11) mit Zeitplan aus (seit 1.23.1 stellt
Migration 00043 sie auf «Aus»; der Spaltenvorgabewert `'ahead'` bleibt ungenutzt, weil das Anlegen
die Einstellung immer schreibt); bei Hörbuch- und
Podcast-Bibliotheken bleiben die Spalten ohne Wirkung. Der Zustand der Sätze steckt im
Cache-Ordner und in `jobs`. Eingefrorener DB-Stand in `testdata/db/` nach Regel 5.

## 9. Die App

- **Zeitleiste (`web/src/ui/SeekBar.tsx`):** Das Vorschaufeld über dem Zeiger zeigt die Kachel der
  Stelle (Ausschnitt des Blatts als CSS-Hintergrund, `background-position`), darunter wie heute Zeit
  und Kapitelname. Das Feld nimmt das Seitenverhältnis der Kachel an (heute fest 16:9, 10 rem breit).
  Ohne Kachel an der Stelle zeigt es wie heute das Kapitelbild, sonst nur Zeit und Name.
- **Maus:** Vorschau beim Zeigen und Ziehen wie heute.
- **Finger:** Vorschau beim Ziehen; gesprungen wird erst beim Loslassen (heute springt jede
  Bewegung).
- **Tastatur und Fernbedienung (←/→, C-4):** **neu**: Nach einem Sprung erscheint das Vorschaufeld
  1,5 s lang über der neuen Stelle und verschwindet dann; bei Fokus auf der Zeitleiste bleibt es,
  solange die Taste gedrückt wird.
- **Laden:** nur das Blatt unter dem Zeiger und das nächste; der Browser-Cache hält sie
  (`immutable`). Solange `state` `queued` oder `running` ist, fragt der Player `getTrickplay` alle
  10 s nach, ohne Zeitgrenze; bei `complete` oder `failed` hört er auf.
- **Offline:** Die PWA speichert die Blätter mit dem Download und zeigt sie aus dem Speicher (6.8).
- **Verwaltung:** Im Dialog «Bibliothek hinzufügen» (`AddLibraryDialog`) und in einem neuen Dialog
  auf der Bibliotheksseite (`LibraryDetail`, neben `DefaultRatingDialog`) der Abschnitt «Vorschau
  beim Spulen», nur bei Videobibliotheken, mit zwei Gruppen:
  - **Filme:** Auswahl «Aus», «Im Voraus», «Beim Start» (seit 1.23.1 «Aus» zuoberst); bei «Beim
    Start» der Haken «Speichern».
  - **Serien:** Auswahl «Aus», «Im Voraus», «Beim Start», «Beim Start alle Folgen»; bei «Beim
    Start» der Haken «Speichern», bei «Beim Start alle Folgen» die Auswahl «Ganze Serie», «Rest der
    Staffel», «Nächste Folge».
  - Unter jeder Gruppe ein Satz zu Speicherbedarf und Dauer der gewählten Einstellung.
  - **Geschätzter Speicherbedarf** (Marcos Wunsch vom 8. November 2026), nur drei Werte: **pro
    Film**, **pro Folge** und **ganze Bibliothek** (je Titelart). Bei «Beim Start» mit «Speichern»
    und «Beim Start alle Folgen» heisst der Bibliothekswert «höchstens»; ohne «Speichern» steht pro
    Film bzw. Folge «nur während der Wiedergabe» und für die Bibliothek 0 MB; bei «Aus» 0 MB.
    - Daneben der heute belegte Platz je Titelart und der freie Platz im Cache.
    - Die Schätzung rechnet der Server aus der Laufzeit der Titel (`media_files.duration_ms`) mit
      rund 1,5 MB je Stunde Video (Messung 3.6: 1,3 MB/h bei 16:9, 1,8 MB/h bei 2,40:1); die
      Bibliotheksseite liest sie über `LibraryTrickplayEstimate` (8.2).
  Die Bibliotheksseite zeigt die Grösse der gespeicherten Sätze, im Dialog «Vorschau beim Spulen»
  (dort liegt die Schätzung von `getLibrary`); die ausgeklappte Zeile der Liste zeigt sie nicht,
  weil `listLibraries` keine Schätzung trägt. «Hintergrundjobs» zeigt
  `trickplay.generate`, `trickplay.series` und `trickplay.backfill`, das Nachholen mit Knopf und
  Zeitplan.
- **Tests (Regel 8, 9):** Texte nur aus `locales`, Farben nur aus Tokens. Keine neue Seite; neu
  getestet wird das Verhalten der Zeitleiste mit Tastatur (Feld erscheint und verschwindet, Fokus
  bleibt sichtbar) in der Spezifikation der Abspielseite, das neue Feld und der neue Dialog in
  `admin-libraries.spec.ts` (Tab, Pfeiltasten, Enter, Zurück, keine Fokusfalle), Storybook für
  `SeekBar` mit Kachel.

## 10. Harte Regeln

| Regel | Wie |
|---|---|
| 1 Generierter Code | Vertrag ändern, dann `task gen` (Enums, Client, strict server) |
| 2 Titelabfragen über `visibility` | Einstellung über `MediaFileLocation`; Folgenliste des Serienvorlaufs nur in `library`; Blätter und Zustand nur über `vis.MediaFile` des aktiven Profils, sonst 404 |
| 3 `os/exec` nur in `transcode` | alle ffmpeg-Aufrufe als Methoden am `Runner`, Argumentliste, `CommandContext` |
| 4 Dateizugriff über `paths` | Mediendatei, Cache- und Download-Ordner über `paths` |
| 5 Migration nach Vorlage | 00041 nur vorwärts, eingefrorener Stand |
| 6 Streaming als `http.Handler` | `getTrickplaySheet`, `getDownloadTrickplaySheet` mit Beschreibung im Vertrag |
| 7 `x-visibility` | alle drei Operationen `scoped`, Vertragstest |
| 8 Tastaturtest | Zeitleiste und Bibliotheksdialog (9) |
| 9 Farben und Texte | Vorschaufeld und Dialog nur mit Tokens und `locales` |
| 11 Nicht-Ziele | Teil A berührt keins. Teil B berührt «Interaktion zwischen Profilen» und hat dafür eine Ausnahme nur für anonyme Empfehlungen (U-F9, Spec-Änderung in 14.8) |
| 2 (Teil B) | `ListUserPicks` in `queries/scoped` mit `visible_items`; das Schreiben prüft vorher die Sichtbarkeit (unsichtbar heisst 404) und liegt im Muster der Merkliste (`discover`) |
| 7 (Teil B) | sechs neue und geänderte Operationen mit `x-visibility` im Vertragstest (14.8) |
| 8 (Teil B) | Stern in der Knopfzeile und die neue Verwaltungsseite mit Tastaturtest (14.9) |
| 12 Entscheid im Server | Einstellung, Sichtbarkeit und Version entscheidet der Server, die App zeigt nur |

## 11. Fragen an Marco

### 11.1 Antworten (8. November 2026)

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
| F-11 | Ab Werk? | Speichern, Zeitplan aus | **Speichern, Zeitplan aus**; Nachtrag 9. November 2026 (1.23.1): ab Werk **«Aus»** statt «Im Voraus», bestehende «Im Voraus» per Migration 00043 auf «Aus» (6.1) |
| F-12 | Gespeicherte Bilder beim Umstellen weg von «Speichern»? | löschen | **Löschen** |
| F-13 | Feinere Einstellungen (Marcos Skizze): je Bibliothek Filme und Serien getrennt, auch gemischt? | ein Modus mit Vorlauf | **Genau wie skizziert**: Filme «Im Voraus», «Beim Start» (Haken «Speichern»); Serien zusätzlich «Beim Start alle Folgen»; «Aus» bleibt (6.1) |
| F-14 | Umfang von «Beim Start alle Folgen»? | Rest der Staffel | **Einstellbar**: ganze Serie (ab Werk, wie skizziert), Rest der Staffel, nächste Folge (6.4) |
| F-15 | Weitere Ideen: Merkliste als Auslöser, Ablauf/Budget, Knopf je Titel? | – | **Keine Präferenz**: nicht aufgenommen (6.9) |
| F-16 | Wie zurückhaltend ist der Serienvorlauf (ganze Serie: bis eine Stunde mit rund 19 MB/s vom NAS)? | nächste 2 sofort, Rest nachts | **Nächste 2 sofort, Rest gedrosselt** (ein ffmpeg-Prozess, solange jemand schaut; 5.3, 6.4) |

### 11.2 Nachträglich entschieden

| Nr. | Frage | Empfehlung | Marcos Antwort (9. November 2026) |
|---|---|---|---|
| O-1 | Starten zwei Zuschauer gleichzeitig zwei verschiedene Titel ohne Satz, wartet der zweite 20 bis 45 s auf seine Vorschau (ein Arbeiter). Hinnehmen, oder ein zweiter Arbeiter nur für Start-Aufträge (dann bis vier ffmpeg-Prozesse neben zwei Streams auf 6 Threads)? | **Hinnehmen.** Im Haushalt selten; die Vorschau des zweiten erscheint nur später, die Wiedergabe startet sofort | **Hinnehmen** («O-1 ist ok so»): ein Arbeiter, kein zweiter für Start-Aufträge |

## 12. Bauplan

Ein Release für Teil A und B (Vertrag vorläufig 1.23.0 brechend), nach Konzept 17. **Teil B kommt
zuerst** mit Migration 00041, weil er klein ist und den brechenden Vertrag früh festlegt; Teil A folgt
mit Migration 00042. Eine eingecheckte Migration wird nie mehr geändert.

| Schritt | Inhalt | Tage |
|---|---|---|
| 0 | Frage 25, ADR 0027 (Teil A) und ADR 0028 (Teil B), Idee 05 auf «übernommen», Spec (Nicht-Ziel, E-1 bis E-3, Status, Glossar) und Plan 03 nachgeführt, Hinweis in Konzept 17 | 0,5 |
| 1 | Teil B Server: Migration 00041, `ListUserPicks`, Schreiben, Pflege, Einstellung, Verschieben beim Zusammenführen und Wiederverknüpfen, Vertrag, `task gen`, Tests und Altlasten aus 14.9 | 1,5 |
| 2 | Teil B App: Stern «Empfehlen»/«Empfohlen ✓», Reihe und Seite ohne Notiz, Verwaltungsseite «User-Empfehlungen», Storybook, Playwright | 1,5 |
| 3 | Teil A `transcode`: Bildpunkt-Aufruf, Blatt-Aufruf, `CPUHeavy`, Tests mit den Testmedien (SDR, HDR, anamorph, MP4) | 1 |
| 4 | Teil A Migration 00042, Vertrag (Schemas, drei Operationen, Pool-Enum), `task gen`, `admin`: `trickplay`, `visibility.MediaFileLocation` | 1 |
| 5 | Teil A `library`: Raster, Jobs mit `origin`, Ablage je Einstellung, Auslöser Start/Import/Nachholen/Serienvorlauf, Abgabe an Start-Aufträge, Drosselung, Fehler, Platzgrenze, Pflege, Pool `trickplay` | 3 |
| 6 | Teil A `playback.Start`, Handler, Wiedergabe-Pflege, `offline` (Kopie, Warten, Manifest), Vertragstest | 1,5 |
| 7 | Teil A App: `SeekBar` (Kachel, Finger, Tastatur), Nachfragen, Offline-Ablage, Bibliotheksdialoge mit Film- und Seriengruppe, Storybook, Playwright | 2,5 |
| 8 | Messung auf CT 190: Start, Blattzeit, Last neben QSV- und CPU-Transcode, **4K über NFS**, Remux neben Trickplay, gleichzeitige volle Keyframe-Lesung, **Serienvorlauf über eine Stunde neben einem 4K-Remux**; `docs/perf/18-trickplay.md` | 0,5 |
| 9 | Teil C (vor Schritt 1, Patch ohne Vertragsänderung): Karten unten bündig samt Raster- und Vorab-Höhen, «Meine Medien», Nachladen und Virtualisierung in den übrigen Listen, Tests, Baselines, P-10 (16.5) | 4,5 |
| 10 | Teil D (unabhängig, eigene Minor-Version, wenn vor A/B): Streamübersicht für alle Arten (17.6) | 2,5 |
| | **Summe** | **20** |

## 13. Kritiker-Review von Fassung 2 (8. November 2026)

Der OMC-Kritiker hat Fassung 2 gegen Code und Regeln geprüft: **REVISE, 1 Blocker, 8 Major,
13 Minor.** Alle Befunde sind in Fassung 3 eingearbeitet:

| Befund | Kern | Wo jetzt |
|---|---|---|
| B-1 | Manifest zuletzt geschrieben, `PlaybackInfo.trickplay` fehlt beim ersten Start, die App fragt nie nach; Zwischenordner wie bei Kapitelbildern verträgt kein Teilergebnis | 5.1, 8.2, 9 |
| M-1 | Laufender Import- oder Nachhol-Auftrag wird nie unterbrochen, ein Start wartet Minuten | 5.2, 5.3 |
| M-2 | Ein Schlüssel für alle Modi, Modus in der Payload | 5.2, 6.8 |
| M-3 | `probedAt` als Version verwirft Sätze ohne Dateiänderung | 5.1 |
| M-4 | Löschregel `per_start` trifft pausierte Wiedergaben, keine Sperre gegen laufende Aufträge | 6.5, 6.7 |
| M-5 | Pausen-Ausnahme und ihr Deckel 3, Wiederholung bis 7 Tage, 12 h Hänger möglich | 5.3, 5.5 |
| M-6 | `movies`/`series` gibt es nicht mehr, es heisst `video` | 6.1, 8.2, 8.3, 9 (Film und Serie sind seit Fassung 4 die Titelarten `movie`/`episode` innerhalb einer Videobibliothek) |
| M-7 | Kein Endpunkt für Download-Blätter, kein Warte-Deckel, Abhängigkeit vom Server-Cache | 6.8, 8.2 |
| M-8 | Kein Speicherdeckel beim Speichern | 6.3 |
| m-1 | Pool-Enums, `task gen`, `main.go`, Locale, `Windowable` | 5.3, 8.1, 8.2 |
| m-2 | Kein Mechanismus für «nur ein Prozess bei CPU-Transcode» | 5.3, 8.1 |
| m-3 | Gerundete Keyframe-Zeit, Lücke im Ersatzweg, «wiederholt» unklar | 5.4 |
| m-4 | Später eintreffender Index verbessert nichts | 5.4, 6.3 |
| m-5 | Dateiänderung während des Laufs | 5.5 |
| m-6 | `playback` braucht den Modus ohne SQL auf `media_files` | 8.1 |
| m-7 | Zeitleiste bei ←/→ und Finger nicht beschrieben, Feld fest 16:9 | 9 |
| m-8 | Kein einzelnes Bibliotheksformular | 9 |
| m-9 | Nachfragen endet nach 5 Minuten zu früh | 9 |
| m-10 | Gespeicherter Satz verschwindet mitten in der Wiedergabe | 6.6 |
| m-11 | Rechenfehler: CPU je Start, 36 s/h und 13 Tage, 540 MB | 0, 6.2, 6.3 |
| m-12 | 4K über NFS ungemessen | 0, 12 Schritt 8 |
| m-13 | Querverweise F-Nummern, Status | Kopf, 5.3 |

Offene Fragen des Kritikers: «temporären Satz beim Umstellen übernehmen» ist in 6.6 entschieden
(umbenennen); «zwei Zuschauer» ist O-1.

## 14. Teil B: User-Empfehlungen von Profil zu Profil

### 14.1 Was Marco will (8. November 2026)

| Nr. | Wunsch | Abschnitt |
|---|---|---|
| U-1 | Die ehemaligen «Cat's Empfehlungen» (seit Konzept 17 F-17 «User-Empfehlungen») so ändern, dass sich die Profile **gegenseitig Titel empfehlen**. | 14.3 |
| U-2 | Ein Klick auf «User-Empfehlungen» bei einem Titel nimmt ihn in die Liste auf. | 14.4 |
| U-3 | Nach **60 Tagen** fällt er wieder heraus; die Dauer steht auf einer Einstellungsseite (Mockup). | 14.5, 14.7 |

### 14.2 Heute

- Die Liste pflegt **nur der Admin** auf `/admin/picks` (`createTopPicks`, `reorderTopPicks`,
  `listTopPicks`; Tabelle `top_picks` aus 00003 mit `profile_id` für ein Zielprofil, `note`,
  `starts_at`/`ends_at` und `position`). Spec E-2: «Der Hauptbenutzer pflegt Cat's Empfehlungen …».
- Zuschauer sehen sie als Startseiten-Reihe `picks` (`getHome`) und auf der Seite `/picks`
  (`listPicks`); die Abfrage `ListTopPicks` in `queries/scoped/items.sql` filtert über
  `visible_items`.
- Der Knopf «Zu den User-Empfehlungen» auf der Detailseite (`ItemPage.tsx`) erscheint nur dem Admin
  (`canManage`) und führt auf die Verwaltungsseite.

### 14.3 Marcos Antworten

| Nr. | Frage | Antwort |
|---|---|---|
| U-F1 | Was passiert mit der Admin-Liste (Notiz, Zielprofil, Zeitfenster, Reihenfolge)? | **Nur noch Profile.** Die Admin-Pflege fällt weg. (Die Übernahme der heutigen Einträge ist durch U-F11 ersetzt.) |
| U-F2 | Wer sieht eine Empfehlung? | **Alle Profile aller Konten**, die den Titel sehen dürfen (Altersfreigabe gilt immer). |
| U-F3 | Was steht an einer Empfehlung? | **Anonym:** kein Name, keine Notiz. |
| U-F4 | Mehrere Profile empfehlen denselben Titel? | **Nur einmal anzeigen, anonym.** |
| U-F5 | Ab wann zählen die 60 Tage? | **Ab der jüngsten Empfehlung**; jede neue startet sie neu. |
| U-F6 | Wer darf empfehlen? | **Alle Profile**, auch Kinderprofile, eingeschränkt durch U-F10. |
| U-F7 | Eigene Empfehlung zurücknehmen? | **Ja.** Der Titel verschwindet nur, wenn ihn kein anderes Profil empfohlen hat; der Admin kann jeden Eintrag entfernen. |
| U-F8 | Reihenfolge der Liste? | **Neueste zuerst.** |
| U-F9 | Die Spec führt «Interaktion zwischen Profilen (Bewertungen, Empfehlungen untereinander)» als Nicht-Ziel. Ausnahme? | **Ausnahme nur für Empfehlungen**; Bewertungen, Kommentare und Nachrichten zwischen Profilen bleiben Nicht-Ziele. |
| U-F10 | Dürfen eingeladene Konten empfehlen? | **Nur lesen.** Empfehlen dürfen die Profile des **Besitzerkontos** (`accounts.is_owner = 1`, «Haushalt des Besitzers» wie heute in `AdminOwnerProfileIDs`); alle anderen Konten sehen die Liste, aber keinen Stern. |
| U-F11 | Was passiert mit den heutigen Einträgen? | **Verwerfen.** Die Liste startet nach dem Update leer. |

### 14.4 Ablauf für Zuschauer

- **Detailseite** (alle Titelarten, die heute in die Liste dürfen: alles ausser Staffeln und Extras,
  wie `checkCurationItem` in `admin/picks.go`): Rundknopf mit Stern **«Empfehlen»** in der
  Knopfzeile (Konzept 17, Variante A), nur für Profile des Besitzerkontos (U-F10).
  - Klick: Der Titel kommt in die Liste; der Knopf zeigt **«Empfohlen ✓»** und einen kurzen Hinweis
    «Zu den User-Empfehlungen hinzugefügt. Bleibt N Tage.», N aus der Antwort des Servers.
  - Nochmals Klick: Die eigene Empfehlung wird zurückgenommen (U-F7); Hinweis «Empfehlung
    zurückgenommen».
  - Der Stern gehört damit «Empfehlen»; der Rundknopf «Favorit» wechselt auf das Herz (wie im
    Mockup), damit nicht zwei Knöpfe dasselbe Zeichen tragen.
- **Startseiten-Reihe «User-Empfehlungen»** und **Seite `/picks`**: jeder Titel **einmal**, ohne
  Namen und ohne Notiz, **neueste Empfehlung zuerst**, für alle Konten (U-F2, U-F3, U-F4, U-F8). Die
  Reihe bleibt bei 20 Titeln, die Seite blättert wie heute.
- Wer den Titel nicht sehen darf, sieht weder Eintrag noch Knopf (Regel 2: unsichtbar heisst 404).
- Die Seitenleiste zeigt «User-Empfehlungen» weiter mit der Zahl der sichtbaren Einträge.

### 14.5 Ablauf und Pflege

- Eine Empfehlung ist eine Zeile **(Titel, Profil, Zeitpunkt)**. Empfiehlt dasselbe Profil denselben
  Titel nochmals, gibt es nur eine Zeile mit neuem Zeitpunkt.
- Ein Titel steht in der Liste, solange seine **jüngste** Empfehlung jünger als die Ablaufdauer ist
  (U-F5). Ab Werk **60 Tage**, einstellbar von **7 bis 365 Tagen** (14.7). Damit kann ein Profil einen
  Titel durch Zurücknehmen und erneutes Empfehlen beliebig lange oben halten; das ist so gewollt
  (U-F5).
- Die stündliche Pflege löscht abgelaufene Zeilen (Muster `library.sweep`, ohne eigene Jobart). Alle
  Abfragen filtern zusätzlich nach Zeit, damit ein abgelaufener Titel nicht bis zur nächsten Pflege
  stehen bleibt. Eine kürzere Ablaufdauer wirkt sofort.
- Wird ein Profil oder ein Titel gelöscht, gehen seine Empfehlungen mit (Fremdschlüssel mit
  `ON DELETE CASCADE`).

### 14.6 Anonymität

- Die App bekommt pro Titel nur **`recommendedByMe`**: ob das aktive Profil selbst eine **gültige**
  Empfehlung hat (eigene Zeile **und** jünger als die Ablaufdauer). Ein Feld «von wem» oder «wie
  viele» gibt es für Zuschauer nicht.
- Der Server speichert das Profil, weil sonst Zurücknehmen (U-F7) und die Zusammenfassung (U-F4)
  nicht gingen.
- Die Verwaltung zeigt je Titel die **Anzahl** und den **Tag** der jüngsten Empfehlung, **keine
  Namen**.
- **Bewusst hingenommen:** Bei einer Anzahl von 1 könnte ein Admin über das Aktivitätsprotokoll
  erraten, wer empfohlen hat; und wer seine eigene Empfehlung zurücknimmt und den Titel weiter in der
  Liste sieht, weiss, dass ein anderes Profil ihn auch empfohlen hat.

### 14.7 Einstellungsseite in der Verwaltung (Mockup)

`/admin/picks` wird neu zur Seite **«User-Empfehlungen»** (Gruppe «Kuratierung», Symbol Stern):

- **Ablauf:** Zahlenfeld «Empfehlungen bleiben … Tage» (7–365, ab Werk 60), darunter der Satz
  «Jede neue Empfehlung desselben Titels startet die Frist neu.» Gespeichert in der neuen
  Einzeilen-Tabelle `picks_config` (Muster `spotlight_config`, `radio_config`).
- **Aktuelle Empfehlungen:** Liste mit Cover (über die Admin-Bildroute, Entscheid vom 25. Oktober
  2026), Titel, «zuletzt empfohlen am …» (nur der Tag), «läuft ab in N Tagen», Anzahl, Knopf
  «Entfernen» (entfernt alle Empfehlungen des Titels). Neueste zuerst wie bei den Zuschauern.
- **Hinweis** oben: «Profile des Besitzerkontos empfehlen über den Stern auf der Titelseite. Namen
  bleiben verborgen.»
- Die Knöpfe für Reihenfolge, Zielprofil, Notiz und Zeitfenster der alten Seite entfallen.

### 14.8 Server, Vertrag, Datenbank und Spec

- **Migration 00041** (Teil B, nach Vorlage, nur vorwärts):

  ```sql
  CREATE TABLE user_picks (
      item_id    TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
      profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
      created_at INTEGER NOT NULL,
      PRIMARY KEY (item_id, profile_id)
  ) STRICT, WITHOUT ROWID;
  CREATE INDEX user_picks_created ON user_picks (created_at);
  CREATE TABLE picks_config (
      id             INTEGER PRIMARY KEY CHECK (id = 1),
      retention_days INTEGER NOT NULL DEFAULT 60 CHECK (retention_days BETWEEN 7 AND 365),
      updated_at     INTEGER NOT NULL
  ) STRICT;
  INSERT INTO picks_config (id, retention_days, updated_at) VALUES (1, 60, 0);
  DROP TABLE top_picks;
  ```

  Die heutigen Einträge werden **verworfen** (U-F11); `top_picks` geht ganz weg, samt Index.
- **Lesen:** `ListTopPicks` wird `ListUserPicks` in `queries/scoped/items.sql`: `GROUP BY item_id`,
  `MAX(created_at)` als Zeitpunkt, nur jünger als die Ablaufdauer, über `visible_items`, neueste
  zuerst, **mit `LIMIT`/`OFFSET` in SQL und einer Zählabfrage** (die heutige Blätterung in Go in
  `visibility.picks` setzt eine kleine, von Hand gepflegte Tabelle voraus). Der Zähler der
  Seitenleiste nutzt die Zählabfrage.
- **Schreiben** im Muster der Merkliste (`discover`, `queries/app/discover.sql`): erst die
  Sichtbarkeit des Titels für das aktive Profil prüfen (sonst 404), dann prüfen, ob das Profil zum
  Besitzerkonto gehört (sonst 404 wie ein unbekannter Titel, Regel 2), dann `INSERT … ON CONFLICT DO
  UPDATE SET created_at = excluded.created_at`. Zurücknehmen löscht die eigene Zeile.
- **Zusammenführen und Wiederverknüpfen:** `AdminMoveTopPicks` (`queries/system/admin.sql`), gerufen
  von `admin.MergeItems` und von der Wiederverknüpfung (`library/relink.go`), wird
  `AdminMoveUserPicks`: `INSERT INTO user_picks SELECT @target, profile_id, created_at FROM
  user_picks WHERE item_id = @source ON CONFLICT DO UPDATE SET created_at = max(created_at,
  excluded.created_at)`; die Zeilen der Quelle gehen mit der Quelle. Ein einfaches `UPDATE` scheiterte
  am Primärschlüssel, sobald ein Profil beide Kopien empfohlen hat. `CountRelinkUserData`
  (`queries/system/relink.sql`) zählt `user_picks`.
- **Vertrag, brechend** (vorläufig 1.23.0 für beide Teile):
  - neu `PUT /items/{id}/pick` (`recommendItem`, 200 mit `{ expiresAt, retentionDays }`) und
    `DELETE /items/{id}/pick` (`unrecommendItem`, 204), beide `x-visibility: scoped`; unsichtbarer
    Titel, Staffel, Extra und Profil ausserhalb des Besitzerkontos 404;
  - neu `ItemDetail.recommendedByMe` (bool, 14.6) und `ItemDetail.canRecommend` (bool, U-F10), damit
    die App den Stern nur zeigt, wo er wirkt;
  - `listPicks` und die Reihe `picks` von `getHome` ohne `note`; Beschreibungen neu;
  - weg: `listTopPicks`, `createTopPicks`, `reorderTopPicks`, `updateTopPick`, `deleteTopPick`, die
    Schemas `AdminPick`, `PickRequest`, `PickUpdate`, `PickOrder` und der Parameter `PickID`;
  - neu, alle `x-visibility: admin`: `listAdminUserPicks` (`GET /admin/picks`: Titel, `posterUrl`
    über die Admin-Bildroute, Anzahl, Tag der jüngsten Empfehlung, läuft ab),
    `removeAdminUserPick` (`DELETE /admin/picks/{itemId}`), `getPicksSettings` und
    `updatePicksSettings` (`/admin/picks/settings`, `retentionDays` 7–365);
  - `RelinkUserData.topPicks` wird `userPicks`;
  - alle neuen Operationen im Vertragstest; `contract_access_test.go` nachgeführt.
- Nach einer Änderung verwirft die App die Abfragen der Detailseite, der Startseite, von `/picks` und
  den Zähler der Seitenleiste.
- **Spec und Plan** (Schritt 0): Nicht-Ziel «Interaktion zwischen Profilen» mit Ausnahme wie beim
  Webradio («Ausnahme seit 8. November 2026: anonyme Empfehlungen zwischen Profilen, Frage 25»);
  Statuszeile «Entdecken und Kuratierung» (Karussell heisst «User-Empfehlungen») und
  «Profil-Interaktion zurückgestellt»; E-1 (Name der Reihe), **E-2 neu** («Die Profile des
  Besitzerkontos empfehlen Titel anonym; jeder Titel einmal, neueste zuerst; Ablauf N Tage nach der
  jüngsten Empfehlung, einstellbar»), E-3 (Sichtbarkeit gilt auch hier); Glossar «Cat's
  Empfehlungen»; Plan 03 «Nicht eingeplant … Profil-Interaktion».
- **ADR 0028** «User-Empfehlungen von Profil zu Profil» hält Tabelle, Anonymität, Ablauf, die
  Ausnahme vom Nicht-Ziel und den Wegfall der Admin-Pflege fest.
- **Konzept 17:** Der Knopf «Zu den User-Empfehlungen» (D-10, F-17) aus Stufe 2 wird mit Teil B zum
  Stern «Empfehlen»; ein Hinweis in Konzept 17 Abschnitt 9.3 verhindert, dass er zweimal gebaut wird.

### 14.9 Tests und Altlasten

- **Server:** Zusammenfassen und Ablauf ab der jüngsten Empfehlung; `recommendedByMe` falsch für eine
  abgelaufene eigene Zeile, und ein Klick darauf erneuert sie statt sie zu löschen; Zurücknehmen mit
  und ohne weitere Profile; Profil ausserhalb des Besitzerkontos bekommt 404 beim Empfehlen und sieht
  die Liste; Kinderprofil sieht die Empfehlung eines gesperrten Titels nicht (404);
  Zusammenführen und Wiederverknüpfen, wenn ein Profil beide Kopien empfohlen hat;
  `picks_config`-Grenzen; Löschkaskaden; Vertragstest.
- **Migrationstest 00041:** `top_picks` ist weg, `user_picks` leer, `picks_config` hat 60.
- **Eingefrorene Stände** (`internal/store/frozen_m7_test.go`, `frozen_m8_test.go`,
  `frozen_startseite_test.go`) fragen heute `top_picks` ab. Sie prüfen neu, dass die Tabelle nach
  00041 fehlt und `user_picks` leer ist; die Prüfungen werden umgeschrieben, nicht gestrichen.
- **Testhilfen:** `internal/visibility/vistest/vistest.go` und `discover.go` legen Empfehlungen über
  `user_picks` an; die e2e-Hilfe `web/e2e/curation.ts` (genutzt von `home.spec.ts`,
  `admin-home.spec.ts`, `admin-phone.spec.ts`) empfiehlt über `recommendItem` statt
  `createTopPicks`.
- **App:** Stern «Empfehlen»/«Empfohlen ✓» in der Knopfzeile (Tastatur: Enter, sichtbarer Fokus);
  kein Stern für Konten ausserhalb des Besitzerkontos; `picks-page.spec.ts` und `admin-picks.spec.ts`
  neu für die Einstellungsseite (Regel 8); Storybook für die Knopfzeile.

## 15. Kritiker-Review von Fassung 4 und 5 (8. November 2026)

Der OMC-Kritiker hat die Änderungen seit Fassung 3 geprüft: **REVISE, 1 Blocker, 8 Major,
12 Minor.** Eingearbeitet in Fassung 6:

| Befund | Kern | Wo jetzt |
|---|---|---|
| B-1 | Teil B ist ein Nicht-Ziel der Spec, das Papier sagte «berührt keins» | U-F9, 2, 10, 14.8 |
| M-1 | Anheben über `RaiseQueuedJobForPlayback` setzt die Pausen-Ausnahme | 5.2, 5.3 |
| M-2 | `Windowable` gilt je Jobart, nicht je Auftrag | 5.3 |
| M-3 | Auftrag kennt seine Herkunft nicht, Nachholen läuft nach dem Umstellen weiter | 5.2 |
| M-4 | Verschieben beim Zusammenführen und Wiederverknüpfen scheitert am Primärschlüssel | 14.8 |
| M-5 | Wegfall von `top_picks` bricht eingefrorene Tests, Testhilfen und e2e | 14.9 |
| M-6 | Übernahme der alten Einträge unklar (Besitzer, Zeitfenster, Zielprofil) | entfällt: Marco verwirft sie (U-F11) |
| M-7 | Serienvorlauf belastet das NAS stundenlang während einer Wiedergabe | F-16, 5.3, 6.4, 12 Schritt 8 |
| M-8 | Serienauftrag unklar (alte Folge, Pool, Versionen, Anheben, nächste Folge) | 6.4 |
| m-1 | Kopf «brechend», 8.2 «additiv» | 8.2 |
| m-2 | Reihenfolge der Teile und eine gemeinsame Migration | Kopf, 12 (00041 Teil B, 00042 Teil A) |
| m-3 | Wegfallende Operationen und Schemas unvollständig, Cover in der Verwaltung | 14.7, 14.8 |
| m-4 | Erlaubte Titelarten fürs Empfehlen | 14.4, 14.8 |
| m-5 | Schreibweg, Blättern in SQL, Abfragen verwerfen | 14.8 |
| m-6 | Zuschauer kennen die Ablaufdauer nicht, `recommendedByMe` ohne Ablauf | 14.4, 14.6, 14.8 |
| m-7 | Feld «ob empfohlen» fehlt | 14.6 (nur `recommendedByMe`) |
| m-8 | Anonymität: Anzahl 1 und Zurücknehmen verraten etwas | 14.6 (Tag statt Zeit, bewusst hingenommen) |
| m-9 | `settings` ohne Prüfung | 14.7, 14.8 (`picks_config` mit CHECK) |
| m-10 | Knopf aus Konzept 17 würde doppelt gebaut | 2, 14.8 |
| m-11 | `LibraryTrickplayEstimate`: Operation, belegter Platz, Versionen, fehlende Laufzeit | 8.2 |
| m-12 | ADR 0027 deckt nur Trickplay | Kopf, 14.8 (ADR 0028) |

Offene Fragen des Kritikers: Übernahme der alten Einträge (verworfen, U-F11), Empfehlungen über
Konten hinweg (lesen ja, empfehlen nur Besitzerkonto, U-F10), Dauer-Empfehlen durch
Zurücknehmen und erneutes Empfehlen (gewollt, 14.5).

## 16. Teil C: Titel unten bündig, «Meine Medien», alles laden statt «Mehr laden»

### 16.1 Was Marco will (9. November 2026)

| Nr. | Wunsch | Abschnitt |
|---|---|---|
| C-1 | Der Titel unter einer Karte steht **immer auf derselben Linie**, egal ob Film, Serie, Hörbuch oder Podcast, in jeder Reihe («Weiterschauen», «Neu hinzugefügt», …) und in jedem Raster. | 16.2 |
| C-2 | Bei «Meine Medien» ist die Unterscheidung zu schwach: Die Beschriftung «Filme», «Serien» usw. wird grösser (**26 px**). | 16.3 |
| C-3 | **Kein «Mehr laden»** mehr: Die Bibliotheksseite (Menü → Filme oder «Meine Medien» → Filme) lädt alles, mit guter Leistung; ebenso Serien und **überall**, wo heute «Mehr laden» steht, auch bei Podcasts usw. Die Reihen der Startseite bleiben kurz wie heute (Richtigstellung von Marco, 9. November 2026). | 16.4 |

Mockups: Artboards «Startseite: Titel unten bündig, Meine Medien» und «Startseite kurz, Bibliothek lädt
alles» der Leinwand «Konzept 18», mit echten
Postern und Covern aus dem Bild-Cache von CT 190 (nur in der privaten Leinwand; die Repo-Kopie
enthält nur Verweise, keine Bilder).

### 16.2 C-1 Titel unten bündig

- **Heute:** `PosterCard` hat zwei Formen, `poster` (2:3) und `square` (1:1, Hörbücher und Podcasts,
  `PosterCard.module.css` `.square`). In einer gemischten Reihe ist ein quadratisches Cover niedriger,
  der Titel darunter rutscht nach oben.
- **Regel:** Alle Karten einer Reihe oder eines Rasters haben **dieselbe Bildhöhe**; der Titel beginnt
  bei allen auf derselben Linie. Gilt für jede Reihe der Startseite, die Bibliotheks- und
  Suchraster, Merkliste, Sammlung, User-Empfehlungen, Personenseite und die Offline-Bibliothek.
- **Varianten** (Mockup, Umschalter oben):
  - **A:** Jede Karte hat den Posterrahmen 2:3; das quadratische Cover sitzt unten im Rahmen, darüber
    bleibt dunkler Platz.
  - **B (Empfehlung):** wie A, aber das Cover sitzt in der Mitte, und über und unter ihm liegt
    dasselbe Bild unscharf und abgedunkelt (CSS `filter: blur()` auf einem zweiten `<img>` derselben
    Quelle, kein zusätzlicher Download). Ruhiges Raster ohne Lücken.
  - **C:** Alle Bilder gleich hoch; ein quadratisches Cover ist dafür breiter (1,5-fache
    Kartenbreite). Nichts beschnitten, aber ungleich breite Karten; im Raster bricht das die Spalten
    und ist darum nur in Reihen möglich.
- **Entschieden (Marco, 9. November 2026): Variante B**, Cover auf unscharfem Grund (O-2).
- **Umsetzung** an allen Stellen, die heute mit der quadratischen Höhe rechnen (Stand `e40ecf1c`):
  - `web/src/ui/PosterCard`: Form `square` erhält den Posterrahmen 2:3; Bildmasse (`height` heute
    400 statt 600) und Fokus- und Glanzrahmen (`.card::before`) folgen dem Rahmen.
  - `web/src/features/catalog/LibraryGrid.tsx` `measure` (Zeilenhöhe heute `square ? 1 : 1.5`) und
    die Platzhalter (`LibraryGrid.module.css`): Faktor 1,5 für alle, sonst überlappen die Zeilen
    im virtualisierten Hörbuch- und Podcast-Raster.
  - Vorab-Karten der Startseite: `server/internal/api/preload.go` (`squareCover`) und
    `web/src/ui/global.css` (`#poster-prepaint > img.square`) zeichnen dasselbe 2:3-Rahmenbild mit
    dunklem Grund, damit die erste Reihe beim Wechsel zur echten Karte nicht springt.
  - Das unscharfe zweite `<img>` trägt dieselben `src`, `srcset` und `sizes` wie das Cover, sonst
    wählt der Browser womöglich eine andere Grösse und lädt doch zweimal.
  - Storybook-Baselines aller betroffenen Bauteile neu.
- Abnahme: P-10 (erste Reihe der Startseite) mit einem Hörbuch in der ersten Reihe neu messen.

### 16.3 C-2 «Meine Medien» mit grosser Beschriftung

- **Heute:** `LibraryTile` zeigt den Namen in `--sc-font-size-md` (1rem, also 16 px, halbfett) und die Anzahl in
  `--sc-font-size-xs` auf einem schmalen Verlauf am unteren Rand.
- **Neu** (Marco wählte am Mockup **26 px**, nach Vergleich mit 30, 34 und 40 px): Name **26 px**,
  fett, mit Schatten; die Anzahl darunter in kleiner Textgrösse (14 px); die Collage wird über die ganze Kachel abgedunkelt, damit der Name auf
  jedem Bild lesbar bleibt. Auf dem Handy skaliert die Schrift mit der Kachelbreite
  (`clamp()`, mindestens 20 px).
- Neue Tokens in `packages/design-tokens/tokens/` für die Schriftgrösse und den Verlauf (Regel 9);
  Kontrast des Namens mindestens 4.5:1 auf dem dunkelsten Verlaufspunkt.

### 16.4 C-3 Alles laden statt «Mehr laden»

- **Heute (`main` `e40ecf1c`, 1.22.0):** Die **Bibliotheken aller vier Arten** laden schon alles:
  Konzept 17 Stufe 3 (1.21.0, PR #128) hat `LibraryPage` auf das virtualisierte `LibraryGrid`
  (`@tanstack/react-virtual`, Seiten zu 100, Buchstabensprung nur bei Filmen und Serien)
  umgestellt, mit `square` auch für Hörbücher und Podcasts. Marcos Wunsch (9. November 2026),
  das auch für Hörbücher und Podcasts zu haben, ist damit **bestätigt und schon gebaut**.
- «Mehr laden» steht noch in: Suche (`SearchPage`), Merkliste (`WatchlistPage`), Sammlung
  (`CollectionPage`), Mitteilungen (`NotificationsPage`), Podcast-Folgen auf der Titelseite
  (`ItemPage`, `item.podcast.more`), Radio (`RadioPage`), User-Empfehlungen (`PicksPage`), in der
  Verwaltung (Wartung `IssueTab`, Radio-Sender `StationsTab`, `OwnStationsTab`) und im
  Ordnerwähler (`PathBrowserDialog`, `library.browse.loadMore`). Teil C stellt alle um.
  `PicksPage` stellt **Teil B** um, wenn er die Seite ohnehin neu baut (14.6), damit sie nicht
  doppelt gebaut wird.
- **Nicht betroffen:** die Reihen der Startseite («Weiterschauen», «Neu hinzugefügt» usw.). Sie
  zeigen weiterhin nur die ersten Titel und führen mit «Alle anzeigen» auf die Bibliotheks- oder
  Listenseite; erst dort wird alles geladen (Richtigstellung von Marco, 9. November 2026).
- **Bausteine:** Teil C löst das Seitenmodell aus `LibraryGrid` (heute an `listLibraryItems` und
  `letterStarts` gebunden) zu einem allgemeinen Baustein für beliebige Listen-Operationen mit
  `offset`, `limit` und `total`, als Raster (Karten) und als Liste (Zeilen, Tabellen).
- **Leistung:**
  - **Virtualisiert nach Grösse, nicht nach Seitentyp:** sobald `total` über 200 liegt, sind nur
    sichtbare Zeilen plus je eine Bildschirmhöhe davor und danach im DOM. Immer virtualisiert:
    Radio (`RadioPage`, Länder und Genres mit Tausenden Sendern) und die Sendertabellen der
    Verwaltung (`StationsTab`, `OwnStationsTab`, rund 40'000 Sender, `docs/perf/radio.md`).
    Mitteilungen (ungelesene bleiben, `queries/system/notify.sql`) und Wartung wachsen ebenfalls
    und fallen unter dieselbe Regel.
  - Bilder (`posterSmall`) erst kurz vor dem Sichtbarwerden, mit festen Bildmassen gegen Springen.
  - Seiten zu 100 Einträgen, Nachladen ab einer halben Bildschirmhöhe vor dem Ende; nie mehr als zwei
    Seitenabrufe gleichzeitig.
  - **Verschobene Seiten:** Kommen beim Scrollen neue Einträge vorne dazu (Merkliste,
    Mitteilungen), verschieben sich die Plätze. Der Baustein verwirft doppelte Einträge nach ID und
    lädt bei geänderter `total` von vorn neu, wie `LibraryGrid` heute bei einem Wechsel der
    Gesamtzahl.
  - Ziel auf dem Handy (Messung beim Bau nach `docs/perf/`): erste Zeile wie heute, flüssiges Scrollen
    über einen Podcast mit über 1'000 Folgen und über die Sendertabelle der Verwaltung.
- **Mehrfachauswahl in der Wartung:** «Alle auswählen» und «X von Y» in `IssueTab` gelten wie heute
  nur für die **geladenen** Einträge; der Zähler sagt das ausdrücklich («X von Y geladenen»).
- **Server:** keine Änderung. Alle betroffenen Operationen haben schon `offset`/`limit` bis
  mindestens 100 und liefern `total`; Teil C ist ein Patch ohne Vertragsänderung.
- Tastatur (Regel 8): dieselben Fokusregeln wie Konzept 17 6.2 Punkt 2; jede betroffene Seite behält
  ihren Playwright-Tastaturtest, «Mehr laden»-Schritte darin werden durch Scrollen ersetzt.

### 16.5 Bau

- C-3 für Bibliotheken: **schon gebaut** in Konzept 17 Stufe 3 (1.21.0); nur der Nachtrag in
  Konzept 17 6.2 Punkt 5 hält das fest.
- C-1, C-2 und C-3 für die übrigen Listen: eigener Schritt vor Teil B, als Patch ohne
  Vertragsänderung (rund **4,5 Tage**: Karten, Raster- und Vorab-Höhen 1, «Meine Medien» 0,5,
  allgemeiner Listenbaustein und Listen 2,5, Tests, Baselines und P-10 0,5).

## 17. Teil D: Streamübersicht für alles, was läuft

### 17.1 Was Marco will (9. November 2026)

| Nr. | Wunsch | Abschnitt |
|---|---|---|
| D-1 | Die Streamübersicht (`/admin/streams`) zeigt **alles, was gerade läuft**: Film, Serie, Hörbuch, Podcast und Radio. Marco nutzt sie als allgemeine Übersicht. | 17.3 |
| D-2 | **Eine Liste**, neueste zuerst, jede Kachel mit ihrer Art. Radio steht nicht mehr in einem eigenen Abschnitt. | 17.3 |
| D-3 | **Filterknöpfe** oben: «Alle», «Video», «Hörbücher», «Podcasts», «Radio», je mit Anzahl. | 17.3 |

Marcos Angaben zum heutigen Stand: Filme und Serien erscheinen; Hörbücher und Podcasts hat er nicht
geprüft; Radio kam nicht.

### 17.2 Heute (`main` `e40ecf1c`, 1.22.0)

- `listStreams` (`server/internal/playback/devices.go` `ListStreams`, Abfrage
  `ListActivePlaybackSessions`; Titel, Bild und Dauer setzt `server/internal/admin/streams.go` über
  `AdminStreamItems` in `queries/system/admin.sql`, das nur zu Serien verbindet) liefert **jede**
  Wiedergabesitzung ohne Filter nach Art: Film, Folge, Extra (Trailer, Making-of), Hörbuch (eine Datei oder HLS über alle Teile) und Podcast-Folge (mit Bibliotheksdatei oder
  über den Feed, `playback/enclosure.go`, `media_file_id` leer). Hörbücher und Podcasts erscheinen
  also schon, aber schlecht erkennbar:
  - Der Titel ist nur der Titel der Folge, ohne Podcastnamen (`streamTitle` kennt nur Serien).
  - Bild: Die Seite liest nur `backdropUrl` (`StreamsPage.tsx`), nie `posterUrl`; Hörbücher und
    Podcasts haben meist keinen Hintergrund und zeigen darum kein Bild. `posterOf` springt nur bei
    Folgen einer Serie zum Elternteil.
  - Dauer: kommt aus `media_files` der Sitzungsdatei. Ohne Bibliotheksdatei (Feed) fehlt sie; bei
    einem mehrteiligen Hörbuch ist es nur die Dauer von Teil 1 (`playback/audio.go` nimmt den
    ersten Teil als Sitzungsdatei), während die Position buchweit läuft.
  - Die Kachel zeigt Videoangaben (Auflösung, HW/SW-Encoder), die bei Audio leer oder irreführend sind.
- **Radio** steht seit 1.19.0 (Konzept 16 Stufe 2, D-11) in einem eigenen Abschnitt «Radio» unter
  der Liste, gelesen aus `listRadioRelays` (`RadioRelayListener`: Profil, Konto, `playing`, `since`;
  ohne Gerät und ohne Senderlogo).
- **Warum Radio bei Marco fehlte:** Auf CT 190 sind die letzten Hör-Sitzungen
  (`radio_listen_sessions`) vom 8. November 2026 17:54, also vor dem Wechsel auf 1.19.0 um 18:47; seither
  lief dort kein Radio. Vor 1.19.0 gab es den Abschnitt noch nicht. Ein Fehler ist damit nicht
  belegt. Die Abnahme von Teil D prüft Radio ausdrücklich (17.6).

### 17.3 Neu: eine Liste «Läuft gerade»

- **Eine Liste**, neueste zuerst (Startzeit), mit Kacheln für Wiedergaben und Radiohörer. Der
  Abschnitt «Radio» fällt weg; «Serverlast» oben bleibt, mit Radio als eigener Kennzahl wie heute.
- **Sortierung:** nach Startzeit absteigend, bei Gleichstand nach ID, damit die Kacheln beim
  Abruf alle 5 s nicht springen. Die Web-App vereinheitlicht dafür `startedAt` (Wiedergabe, ISO)
  und `since` (Radiohörer).
- **Abzeichen** auf jeder Kachel: «Film», «Serie», «Extra», «Hörbuch», «Podcast», «Radio».
- **Filterknöpfe** über der Liste: «Alle», «Video» (Filme, Folgen und Extras), «Hörbücher»,
  «Podcasts», «Radio», je mit Anzahl; «Alle» ist die Summe der übrigen. Ein Knopf ohne Treffer
  bleibt sichtbar mit 0. Die Wahl gilt nur auf dieser Seite und bleibt beim Neuladen der Liste
  (alle 5 s) stehen, auch wenn der gewählte Filter leer wird; dann steht dort «Gerade läuft nichts
  dieser Art.» (Locale-Schlüssel `admin.streams.emptyKind`), ohne jeden Filter wie heute der
  bestehende Leertext.
- **Tastatur und ARIA (Regel 8):** die Knöpfe sind eine Werkzeugleiste (`role="toolbar"`) aus
  Umschaltknöpfen mit `aria-pressed`, ←/→ zwischen den Knöpfen, Tab führt in die Kacheln.
- **Teilausfall:** Schlägt einer der beiden Abrufe fehl (`listStreams` oder `listRadioRelays`),
  bleibt die andere Hälfte stehen, mit einem Hinweis über der Liste; die betroffenen Zähler zeigen
  «–». Heute blendet ein Fehler von `listStreams` die ganze Liste aus.
- **Kacheln nach Art:**

| Art | Titel | Bild | Angaben | Fortschritt |
|---|---|---|---|---|
| Film | Titel | Hintergrund wie heute | Gerät, Qualität, Modus, Grund | Position / Dauer |
| Serie | «Serie · S01E02 · Titel» wie heute | Hintergrund der Serie | wie Film | Position / Dauer |
| Extra | «Film oder Serie · Extra» | Hintergrund des Films oder der Serie | wie Film | Position / Dauer |
| Hörbuch | «Buch · Kapitel» (Kapitel an der Position, sonst nur Buch) | Cover mittig auf unscharfem Grund (wie Teil C, Variante B, aber in `StreamTile` selbst) | Gerät, Format (Codec, Bitrate, Anzahl Teile), Modus | Position / Dauer des ganzen Buchs |
| Podcast | «Podcast · Folge» | Cover der Folge, sonst des Podcasts, wie Hörbuch | Gerät, Format, Quelle «Datei» oder «Feed» | Position / Dauer (aus dem Feed, wenn die Datei fehlt) |
| Radio | Sendername | Senderlogo wie Hörbuch, sonst Platzhalter | Format (Codec, Bitrate), «umkodiert» oder nicht, Hörer dieses Senders, «Hört» oder «Pausiert» | keiner, nur «seit …» |

- «Beenden» bleibt auf jeder Kachel, mit derselben Bestätigung wie heute (Wiedergabe:
  `stopStream`; Radio: `stopRadioListener`).
- **Bild in `StreamTile`:** neue Bildform für quadratische Cover und Logos mit eigenem unscharfem
  Grund, unabhängig von `PosterCard` (Teil D kann vor Teil C kommen). Abdunkelung und Unschärfe als
  neue Tokens (Regel 9); eine Storybook-Geschichte je Form.

### 17.4 Server und Vertrag

- `AdminStream` erhält additiv:
  - `kind` (`movie`, `episode`, `extra`, `audiobook`, `podcast_episode`);
  - `parentTitle` (Podcastname; bei einem Extra der Film oder die Serie; bei Serien wie heute im
    Titel). Dafür verbindet `AdminStreamItems` auch zum Eltern-`podcast` und zum Besitzer eines
    Extras, nicht nur zur Serie;
  - `chapterTitle` (Hörbuch, Kapitel an `positionMs` aus der Tabelle `chapters`);
  - `partCount` (Hörbuch, Anzahl Teile, wenn mehr als einer);
  - `audioCodec` und `audioBitrate`: direkt abgespielt Codec und Bitrate aus der Sonde der Datei;
    über HLS `aac` und `playback_sessions.bitrate`; über den Feed der Codec aus dem MIME-Typ der
    Enclosure, Bitrate leer (keine Sonde, `playback/enclosure.go`);
  - `source` (`file` oder `feed`).
- `durationMs`: bei `audiobook` **immer** `items.duration_ms` (Summe aller Teile, Migration 00014);
  bei `podcast_episode` ohne Datei `items.duration_ms` aus dem Feed, wenn er eine nennt; sonst wie
  heute `media_files.duration_ms` der Sitzungsdatei.
- Bilder: `posterOf` springt bei einer Podcast-Folge ohne eigenes Bild zum Podcast und bei einem
  Extra zum Film oder zur Serie; Hörbuch und Podcast liefern `posterUrl` (quadratisch) und keinen
  `backdropUrl`.
- `RadioRelayListener` erhält additiv `device` (wie `AdminStream.device`; die Hör-Sitzung kennt
  ihre Browsersitzung `session_id`, darüber das Gerät) und `RadioRelay` das Logo als
  `logo: RadioLogo` wie `RadioStation.logo` (URL von `getAdminRadioImage`, Breite 256, `v` als
  Inhalts-Hash).
- **Beim Bau prüfen:** `radio/listen.go` gibt Hörer nur für laufende Relais aus. Kann eine offene
  Hör-Sitzung ohne laufendes Relais bestehen (Relais gestört oder gestoppt), erscheint sie als
  Radio-Kachel mit Zustand «Relais gestört» statt zu fehlen.
- Die Web-App mischt beide Listen (`listStreams` und `listRadioRelays`) zu einer (Sortierung und
  Teilausfall in 17.3). Keine neue Operation, keine Migration, alles additiv. Kommt Teil D vor
  Teil A und B, erhält er eine **eigene Minor-Version** (vorläufig 1.23.0, Regel Vertrag = Tag), und
  der A/B-Vertrag rückt eine Nummer weiter; sonst steckt er im A/B-Vertrag.
- Die Admin-Ansicht bleibt ungefiltert (`x-visibility: admin`, keine `ItemRef`), wie heute.

### 17.5 Mockup

Artboard «Verwaltung: Streams, alles in einer Liste» der Leinwand «Konzept 18»: Serverlast oben,
Filterknöpfe mit Anzahl, sechs Kacheln (Film, Folge, Hörbuch, Podcast aus Datei, Podcast aus dem
Feed, Radio) mit echten Bildern aus dem Bild-Cache von CT 190 (nur in der privaten Leinwand).
Marco hat das Mockup am 9. November 2026 ohne Änderung abgenommen.

### 17.6 Bau und Abnahme

- Eigener Schritt, unabhängig von Teil A bis C (Bauplan Schritt 10), rund **2,5 Tage**: Server und
  Vertrag 0,75, Seite, Kacheln und Bildform 1,25, Tests 0,5. Er kann vor oder nach Teil C gebaut
  werden.
- Tests:
  - Vertragstest für die neuen Felder.
  - Go-Test `ListStreams` mit je einer Sitzung jeder Art: Film, Folge, Extra, Podcast mit und ohne
    Datei, Hörbuch mehrteilig mit Kapitel; geprüft werden `kind`, `parentTitle`, Bild und dass
    `durationMs` beim mehrteiligen Hörbuch der Buchsumme entspricht.
  - Go-Test im Paket `radio` für `listRadioRelays` mit `device` und `logo`.
  - Playwright-Test der Seite mit allen Arten, Filterknöpfen, leerem Filter, Teilausfall und
    Tastatur (Regel 8); Storybook-Baselines von `StreamTile` je Bildform.
- **Abnahme auf CT 190:** gleichzeitig ein Film, eine Folge, ein Extra, ein Hörbuch, ein Podcast aus
  der Bibliothek, ein Podcast über den Feed und ein Radiosender; alle sieben Kacheln erscheinen innert
  5 s, jeder Filterknopf zeigt die richtige Anzahl und «Alle» deren Summe; das Hörbuch zeigt Kapitel
  und Buchdauer, der Podcast seinen Namen, das Radio Gerät und Logo; «Beenden» stoppt jede Art.

## 18. Kritiker-Review von Teil C und D (9. November 2026)

Der OMC-Kritiker hat Abschnitt 16 und 17, Frage 25 und die drei Mockups gegen `main` `e40ecf1c`
(Release 1.22.0) geprüft: **REVISE, kein Blocker, 6 Major, 11 Minor.** Eingearbeitet in Fassung 9:

| Befund | Kern | Wo jetzt |
|---|---|---|
| M-1 | Bibliotheken aller Arten sind seit 1.21.0 schon virtualisiert, das Papier plante sie neu | 16.4, 16.5, Frage 25, Konzept 17 6.2 |
| M-2 | Variante B betrifft auch Rasterhöhe, Platzhalter, Vorab-Karten und Bildmasse | 16.2 |
| M-3 | Mehrteiliges Hörbuch zeigt die Dauer von Teil 1 | 17.2, 17.4, 17.6 |
| M-4 | Extras laufen über `playback_sessions`, `kind` kannte sie nicht | 17.2, 17.3, 17.4, 17.6 |
| M-5 | Listen über 200 Einträge (Radio, Sendertabellen rund 40'000, Mitteilungen), Schätzung zu knapp | 16.4, 16.5, 12 |
| M-6 | Vertrag 1.22.0 vergeben, Teil D fehlte im Bauplan | Kopf, 8.2, 12, 14.8, 17.4 |
| m-1 | Pfad, Abfrage und Hash in 17.2 | 17.2 |
| m-2 | Seite liest nur `backdropUrl`, Hörbuch und Podcast ohne Bild | 17.2, 17.3 |
| m-3 | Logo als `RadioLogo` statt `logoUrl` | 17.4 |
| m-4 | Format bei Feed-Folgen und HLS offen, «12 Teile» ohne Feld | 17.4 (`partCount`) |
| m-5 | Teilausfall und Sortierung der zusammengeführten Liste | 17.3 |
| m-6 | Leerzustand je Filter und ARIA-Muster | 17.3 |
| m-7 | `PathBrowserDialog` fehlte | 16.4 |
| m-8 | Satz zu `limit` überflüssig | 16.4 (gestrichen) |
| m-9 | Schrift heute 16 px, nicht 15 px | 16.3, Frage 25, Mockup |
| m-10 | Abnahme prüfte die neuen Felder nicht | 17.6 |
| m-11 | `StreamTile` braucht den unscharfen Grund selbst | 17.3 |

Offene Fragen des Kritikers: Hör-Sitzung ohne laufendes Relais (beim Bau prüfen, 17.4),
verschobene Seiten beim Nachladen (16.4), `PicksPage` doppelt gebaut (Teil B stellt sie um, 16.4).

## Anhang: Messskripte

`docs/perf/18-trickplay/`: `bench.py` (lokal, Zeit, CPU und gelesene Bytes je Weg), `ct-bench.sh`
und `ct-run.sh` (CT 190 im Container, NFS-Bytes aus `/proc/self/mountstats`), `ct-extra.sh`
(HDR und Last neben Transcode), `sheets.sh` (Blattgrössen). Bilder aus Filmen kommen nie ins
Repository.
