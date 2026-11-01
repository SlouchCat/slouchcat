# Konzept: Schneller Wiedergabestart – Keyframe-Index aus dem Container, Transcode ohne Wartezeit, scharfes QSV

- **Status: Fassung 2 (1. November 2026), Kritiker-Review eingearbeitet (0 Blocker, 6 Major,
  22 Minor), noch nicht freigegeben.** Auslöser
  ist ein Befund auf dem Testserver CT 190 vom 1. November 2026: Der Start von «Silo» S01E03 in
  Firefox dauerte **20 s**, und das Bild danach war auf QSV sichtbar unscharf (1.1). Marco hat am
  1. November 2026 in zwei Interviewrunden alle acht Fragen entschieden (**F-1 bis F-8**,
  Abschnitt 11.1); sieben folgen der Empfehlung, F-4 (Nachholen per Knopf **und** einplanbar,
  einmalig wie wiederkehrend) erweitert sie. Offene Punkte ohne Blockerwirkung stehen als **O-1
  bis O-19** mit Empfehlung in 11.2. In einer dritten Runde hat Marco am selben Tag **O-7
  (höchstens 3), O-17 (24 h Ruhe) und O-18 (Audio ausgenommen) wie empfohlen** und **O-3
  abweichend** entschieden: **alles in einem PR**, kein Patch 1.6.1 vorab. Der OMC-Kritiker hat Fassung 1 geprüft (REVISE, 0/6/22);
  Fassung 2 arbeitet alle Befunde ein, die Abwägungen stehen in Abschnitt 13.
  **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Datum: 1. November 2026 (Fassung 1 und 2).**
- **Wogegen dieses Konzept verankert ist:** `main`, Stand **`5e0e63d`** («Merge pull request
  #61», Konzept 11), Vertragsfassung **1.6.0** (`api/openapi.yaml:4`), letzte Migration
  **`00030_library_metadata.sql`**, letzte ADR **`0018-metadaten-sprachen-bilder-freigabeland.md`**.
  Reserviert, aber **nicht gebaut**: Konzept 11 (Frage 17, ADR 0019, Migration 00031, Vertrag
  1.7.0) und Konzept 12 (`docs/planning/12-verwaltungsseiten.md` auf einem anderen Zweig; Frage
  18, ADR 0020) sowie Konzept 13 «Abspielseite» (`docs/planning/13-abspielseite.md` auf dem
  Zweig `claude/konzept-abspielseite`; Frage 19, ggf. ADR 0021). Dieses Papier ist darum
  **Konzept 14** und nimmt **Frage 20 und ADR 0022**; **Migration und Vertragsfassung werden
  beim Baubeginn in Bau-Reihenfolge vergeben** (goose verlangt lückenlose Reihenfolge, die Regel
  «Vertrag = Tag» eine aufsteigende Version). Wo dieses Papier vorläufig **Migration 00032** und
  **Vertrag 1.8.0** schreibt, sind das Platzhalter für «die nächste freie»; wird es vor 11, 12
  und 13 gebaut, sind es **00031 und 1.7.0** (ein gemeinsamer PR, O-3). Frage- und
  ADR-Nummer sind blosse Reservationen und dürfen bleiben. Die Empfehlung lautet **vorziehen**
  (10.1): das Papier behebt einen Fehler, den jeder Start einer Datei ohne Index spürt.
  Nummernlage siehe **O-1**.
- `:Zeile`-Angaben beziehen sich auf `5e0e63d`; **nach Namen suchen, nicht nach Zeile.** Pfade
  ohne Präfix liegen unter `server/`.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
- **Jeder Schritt ist ein eigener Commit, und vor dem Abschluss jedes Schritts ist `task ci`
  grün** (`CLAUDE.md`, «Arbeitsweise»). DoD-Belege sind Befehl **und** Ausgabe.

> **Dieses Papier ändert keine Zeile Code.** Die Arbeit beginnt mit Schritt 0 aus Abschnitt 10,
> einem reinen Doku-Commit. Schritt 1 (QSV) ist ohne Vertrag und ohne Migration sofort lieferbar.

> **Abgrenzung.** Hier geht es um drei Dinge, die zusammen den Start einer Wiedergabe bestimmen:
> **woher der Keyframe-Index kommt** (Matroska-Cues, MP4-`stss`, erst zuletzt die Volllesung)
> und wann er entsteht (beim Einlesen, nicht beim Start); **dass ein Start nie auf den Index
> wartet** (fehlt er, wird sofort transcodiert, der nächste Start remuxt); und **wie scharf das
> Bild beim Transcode auf QSV ist** (Ratensteuerung). Dazu gehören die Jobarten, die den Index
> nachholen, eine eng umrissene Ausnahme von der Pause, der einmalige Termin im Scheduler und
> zwei Anzeigen für den Administrator. **Nicht** hierher gehören: GPU-Tonemapping für 4K-HDR auf
> QSV (der Start bleibt dort durch das CPU-Tonemapping bei 2,4 bis 3,1 s, 8), neue
> Qualitätsstufen («Original», 2160p; Konzept 13 «Abspielseite», V-15), Vorschaubilder beim
> Spulen (ebenda, V-21), QVBR/ICQ-Varianten der Ratensteuerung, ein Index für fragmentiertes
> MP4, TS, M2TS, AVI und WMV, Open-GOP-Punkte aus `sbgp`/`sgpd`, Kalender-Zeitpläne (cron), eine
> einstellbare Arbeiterzahl in der Oberfläche (Konzept 09, F-9) und die Byte-Positionen aus
> `CueClusterPosition`. Ein Zuschauer merkt nur, dass es schneller startet und schärfer aussieht.

---

## 1. Ausgangslage und Problem

### 1.1 Der Befund vom 1. November 2026

Testserver CT 190 (`192.168.1.43`, Image `slouchcat:1.6.0`, jellyfin-ffmpeg 7.1, Intel UHD 630,
Medien über NFS 4.2). Titel: «Silo» S01E03, MKV, HEVC Main10 1080p, EAC3 Atmos, rund 3 GB.
Firefox kann EAC3 nicht, also entscheidet der Server **Remux** mit umgewandeltem Ton.

| Zeit | Was geschah | Wo |
|---|---|---|
| 0 ms | `POST /api/v1/items/{id}/play` | `playback/sessions.go:104` `Start` |
| 0 ms | `Decide` → Remux-Kandidat; `prepare` ruft `ensureKeyframes` | `sessions.go:489-516` |
| 0–20 000 ms | Kein gespeicherter Index; synchron `ffprobe … -show_entries packet=pts_time,flags` über die **ganze Datei**, Zeitgrenze `KeyframeTimeout = 20 s` | `sessions.go:522-548`, `:37-39`; `transcode/probe.go:311-317` |
| 20 000 ms | Abbruch (`ffprobe: signal: killed`), Job `probe.keyframes` mit Priorität 9 eingereiht, Transcode mit `reason = keyframes_pending` | `sessions.go:511-513`, `:551-555` |
| 20 016 ms | Antwort an den Client | |
| + 350 ms | `init.mp4` und `seg/0` ausgeliefert | |

Drei Umstände kamen zusammen:

1. **Der Index fehlte.** 307 von 2419 Dateien auf CT 190 hatten keinen (`keyframes IS NULL`), drei
   weitere (VC-1) einen leeren (nur das Versionsbyte).
2. **Die Jobart `probe.keyframes` war seit dem 31. Oktober pausiert** (`job_types.paused_at`,
   Konzept 09, Entscheid 15 F-1). Damit lief auch der von der Wiedergabe mit Priorität 9
   eingereihte Auftrag nicht; der nächste Start hätte wieder 20 s gewartet.
3. **Der Transcode auf QSV war unscharf.** `-global_quality 23` zusammen mit `-maxrate` ohne
   `-b:v` lässt `h264_qsv` stillschweigend **CQP mit QP 26** wählen; gemessen im Mittel
   **1,7 Mbit/s** bei 1080p (je Clip 0,7 bis 3,9 Mbit/s, Anhang A.3), Hautporen und Gewebe
   weggewaschen (1.4).

Die 20 s liegen weit über W-3 (Spec: «Ein 4K-HEVC-Film startet im Handy-Browser in unter fünf
Sekunden»). Ohne die Wartezeit wäre derselbe Start in rund 0,4 s fertig gewesen.

### 1.2 Lebenszyklus des Index heute

| Schritt | Heute | Stelle |
|---|---|---|
| Einreihen | `probe.file` schreibt Probe, Spuren, Sidecars in einer Transaktion und reiht am Ende `probe.keyframes` ein (ausser Hörbüchern und Podcast-Folgen); Priorität **1**, Schlüssel `keyframes:<mediaFileId>` | `library/probe.go:68`, `:165-176`; `library/jobs.go:87-89`, `:40` |
| Rechnen | `handleKeyframes` → `transcode.Runner.Keyframes`: ffprobe liest **jede** Paketzeile der Videospur, also die ganze Datei | `library/probe.go:225-244`; `transcode/probe.go:311-338` |
| Fehler | Jeder Fehler gilt als vorübergehend: Backoff 10/20/40/80 s, nach 5 Versuchen `failed`. Ein kaputter 3-GB-Film wird bis zu fünfmal ganz über NFS gelesen. `handleProbe` kennt dagegen `transcode.FileError` und speichert ohne Wiederholung | `probe.go:238-240`; `jobs/queue.go:31-33`, `:61`; `probe.go:77-86` |
| Speichern | `keyframes BLOB`, `keyframes_at`; Format Version 1 (Byte 0 = 1, dann Uvarint-Abstände in ms). `UpdateMediaFileKeyframes` überschreibt **immer** | `migrations/00003_libraries_items.sql:114-116`; `queries/system/items.sql:91-92`; `library/probe.go:246-291` |
| Zweiter Schreiber | `StoreKeyframes` für Wiedergabe und Offline | `library/jobs.go:118-127` |
| Lesen | `GetVisibleMediaFileProbe` über `visibility.MediaFileProbe`; Download-Quelle | `queries/scoped/playback.sql:5-6`; `visibility/playback.go:25-41`; `queries/system/offline.sql:220` |
| Wiedergabe | `ensureKeyframes`: Blob da → Remux; sonst synchron ffprobe bis 20 s; Erfolg → speichern und Remux, sonst Job mit 9 und Transcode. Ein gültiger Blob mit 0 Einträgen (Audio, VC-1) gilt als «unbrauchbar» und löst **jedes Mal** wieder die Volllesung aus | `playback/sessions.go:522-548`, `:524-529`; `main.go:521` |
| Vorrang | `QueueKeyframesForPlayback` **bricht** einen wartenden Scan-Job ab und reiht neu ein, statt ihn anzuheben; daher der «cancelled»-Eintrag im Verlauf. `Queue.RaiseQueuedPriority` gibt es seit Konzept 09 | `library/jobs.go:91-116`; `jobs/queue.go:299-308` |
| Offline | `offline.Service.keyframes`: gespeicherter Index, sonst **synchron ohne Zeitgrenze** im Download-Arbeiter | `offline/job.go:263-279` |

### 1.3 Pause und Arbeiter

`ClaimJob` schliesst pausierte Jobarten **ohne Ausnahme** aus (`queries/system/jobs.sql:46-61`,
Zeile `:57`); eine Pause bricht laufende Läufe sofort ab (`jobs/registry.go:216-232`) und gibt sie
ohne verbrauchten Versuch zurück (`jobs/worker.go:144-148`). Es gibt kein Merkmal «trotz Pause».

Drei Pools (`jobs/registry.go:37-42`): `general` mit `cfg.Workers` Arbeitern (Vorgabe
`max(2, min(4, NumCPU))`, `config/config.go:261-262`; auf CT 190 vier), `download` und `notify`
mit je einem (`cmd/slouchcat/main.go:332-351`). **Eine Arbeiterzahl je Jobart gibt es nicht**:
zwei `probe.keyframes` können zwei Volllesungen gleichzeitig über das NAS fahren, während
`segments.detect` je Job zwei weitere ffmpeg-Prozesse startet (`library/segments.go:28-29`).
Eine Volllesung von 3 bis 6 GB belegt einen der allgemeinen Arbeiter für 1,6 bis 6,6 min
(Anhang A.1).

### 1.4 Ratensteuerung der Encoder

| Backend | Argumente heute (HLS, nicht CBR) | Was ffmpeg daraus macht | Bewertung |
|---|---|---|---|
| QSV | `-preset veryfast -global_quality 23 -look_ahead 0 -maxrate R -bufsize 2R` (`transcode/pipeline.go:389-395`) | `select_rc_mode()` in `libavcodec/qsvenc.c`: `global_quality` mit `maxrate` ist nicht ICQ, ohne `-b:v` nicht VBR → **CQP, QP 26**, der Wert 23 wird verworfen; auf CT 190 mit `-loglevel verbose` belegt (`RateControlMethod: CQP`) | **Fehler**: im Mittel 1,7 Mbit/s bei 1080p (je Clip 0,7 bis 3,9), SSIM 0,9816, sichtbar weich |
| VAAPI | `-rc_mode VBR -b:v R -maxrate R -bufsize 2R` (`:420-421`) | VBR mit `target_percentage = 100`: Ziel = Deckel | scharf, aber fast immer am Deckel (8 Mbit/s bei 1080p) |
| NVENC | `-rc vbr -cq 23 -b:v 0 -maxrate R -bufsize 2R` (`:358-364`) | «CQ mode»: Qualität mit Deckel `maxrate`; `bufsize` wird verworfen (VBV = 0) | Modus richtig gewählt; der Deckel wirkt aber ohne Puffer nur ungefähr: Aladdin 11,1 Mbit/s gleitend über 4 s bei R = 8 (Anhang A.3). Nicht Gegenstand dieses Papiers (O-19) |
| libx264 | `-crf 23 -maxrate R -bufsize 2R` (`:424-431`) | CRF mit VBV-Deckel | korrekt |

Der Kommentar zur Ratensteuerung (`pipeline.go:301-305`, «constant quality capped») stimmt für QSV
und VAAPI nicht. Die Backend-Erkennung (`transcode/backend.go:209-222`, `testEncodeArgs`) kodiert
ein Bild ohne Ratensteuerung und hätte den Fehler nie bemerkt. Die Leiter steht in
`transcode/plan.go:123-145` (Spiegel in `playback/quality.go:20-25`): 1080p 8 Mbit/s, 720p 4,
480p 2, 360p 1. `keyframes_pending` transcodiert auf der höchsten erlaubten Stufe der Quelle
(`sessions.go:381-392`); auf QSV kam der Fallback also mit der zu niedrigen Bitrate zusammen.

### 1.5 Zusatzbefunde

| Nr. | Befund | Stelle | Folge |
|---|---|---|---|
| Z-1 | **Veralteter Index nach Dateiänderung.** Ersetzt Radarr eine Datei am selben Pfad, setzt `UpdateMediaFileLocation` Grösse und mtime neu, lässt `keyframes` aber stehen; `UpdateMediaFileProbe` ebenso. Kein Query im Baum setzt `keyframes = NULL` | `library/scanner.go:726-734`; `library/write.go:349-372`; `queries/system/items.sql:61-65`, `:84-89` | Remux mit den Keyframes der **alten** Datei, bis der neue Job (Priorität 1, ggf. pausiert) lief: Segmentgrenzen verschoben, kaputte Segmente |
| Z-2 | **Rebuild plant mit dem aktuellen Index.** Eine als Remux begonnene Sitzung ruft nach Leerlauf oder Neustart wieder `prepare` mit dem **heutigen** Blob; ändert er sich, passen die Segmente nicht mehr zur Playlist des Clients | `sessions.go:644-711`, `:694`; `transcode/transcoder.go:39-51` | Invariante nötig: **ein Index ändert sich je Dateiversion nicht** (2.6) |
| Z-3 | **Kein Vermerk «Index unmöglich».** Nach `failed` bleibt `keyframes` NULL; jede Wiedergabe versucht es neu | `probe.go:238-240` | Endlose Volllesungen derselben Datei |
| Z-4 | **Abbruch statt Anheben** in `QueueKeyframesForPlayback` | `library/jobs.go:111` | Verlaufslärm, Konzept-09-Regel verletzt |
| Z-5 | **Kein `singleflight`.** Zwei gleichzeitige Starts derselben Datei lesen sie zweimal | `sessions.go:522-548` | doppelte NAS-Last |
| Z-6 | **Offline rechnet synchron ohne Zeitgrenze** im Download-Arbeiter | `offline/job.go:263-279` | eine zweite Volllesung neben dem Job, F-2 verletzt |
| Z-7 | **265 fehlgeschlagene `probe.keyframes`** auf CT 190 (Konzept 11, 1.1); Ursache nicht im Code belegt | – | O-9 |
| Z-8 | **11 DB-Zeilen ohne Datei** unter `/srv/media` (Messung, Anhang A.1) | – | O-10, nicht Thema dieses Papiers; sie zählen nicht als «ausstehend», sobald `missing_since` gesetzt ist (4.1) |
| Z-9 | **Wettlauf Probe gegen Dateiänderung.** Ändert sich eine Datei, während `probe.file` oder `probe.keyframes` läuft, kann der Scanner keinen neuen Job gleichen Schlüssels einreihen (`queries/system/jobs.sql:3-7`, `ON CONFLICT … WHERE state IN ('queued','running') DO NOTHING`); der laufende schreibt Probe und Index der **alten** Version (`library/write.go:368-372`) | heute heilt es der nächste Überschreib-Lauf | mit endgültigem Index (I-1) nicht mehr; darum bindet 4.2 jedes Schreiben an die Dateiversion |
| Z-10 | **VC-1 ohne Keyframe-Flags.** Die drei VC-1-Dateien haben vollständige Cues, aber ffprobe meldet keine `K`-Pakete; der Segment-Muxer schneidet nur an `K` | Messung, Anhang A.1 | nicht remuxbar; VC-1 spielt ohnehin nur als Transcode (`playback/decide.go:250-259`, `directVideo`) |

### 1.6 Was die Messung zeigt

Die Messung auf CT 190 vom 1. November 2026 (Anhang A) beantwortet die Kernfrage, ob der
Container den Index schon mitbringt:

- **Matroska-Cues sind in diesem Bestand ein vollständiger und exakter Keyframe-Index.** In 40 MKV
  mit Referenzindex (40 Releasegruppen, 26 HEVC, 14 H.264, Muxer mkvmerge, ffmpeg, HandBrake) ist
  jede der 59 915 Cue-Zeiten der Videospur ein Referenz-Keyframe (±1 ms, kein Versatz), und jeder
  Referenz-Keyframe steht in den Cues. **Alle 298** auf der Platte vorhandenen MKV ohne Index haben
  Cues für die Videospur; grösste Lücke überall ≤ 10,43 s.
- **Kosten:** Median 123 ms kalt und 153 KB gelesen (Stichprobe A), 58 ms und 68 KB über alle
  298; drei Lesezugriffe. Die Volllesung braucht für 4 bis 6 GB bei gemessenen 42 MB/s 1,6 bis
  2,4 min. Alle 298 Dateien: **23,6 s** statt 2,6 h (42 MB/s) bis 7,2 h (15 MB/s).
- **MP4:** `stss` mit `stts`, `ctts`, `elst` und `mdhd` liefert in 11 von 11 Dateien mit Referenz
  exakt die Keyframe-Zeiten; ohne `ctts` nur in 10 von 11 («1492», nur 21 % exakt). `ctts` ist
  Pflicht.
- **31 von 40 MKV enthalten zusätzlich Cues für Audio und Untertitel** (bis 75 807 Punkte bei
  2 744 Videopunkten): ohne Spurfilter wäre der Index falsch.

### 1.7 Was Marco entschieden hat

Kurz (vollständig in 11.1): **sofort transcodieren**, wenn der Index fehlt (F-1); die Volllesung
**jederzeit, höchstens eine gleichzeitig** (F-2); ein von der Wiedergabe ausgelöster Auftrag läuft
**auch bei pausierter Jobart** (F-3); Nachholen **per Knopf und einplanbar, einmalig wie
wiederkehrend** (F-4); QSV mit **festem Zielwert und Obergrenze** (F-5); der **einmalige Termin
für alle Zeitplan-Jobs** (F-6); der Grund in der **Streams-Seite** und die **Index-Abdeckung je
Bibliothek** (F-7); Cues nur nach **Plausibilität und Stichprobe** (F-8).

---

## 2. Zielbild

### 2.1 Drei Quellen in fester Reihenfolge

| Rang | Quelle | Wann | Kosten (CT 190, kalt) | `keyframes_source` |
|---|---|---|---|---|
| 1 | **Matroska-Cues** (MKV, WebM): EBML-Kopf → Segment → SeekHead (auch der zweite) → Info, Tracks, Cues; nur CuePoints, deren `CueTrackPositions` die Videospur nennt | beim Einlesen (`probe.file`) | 36–757 ms, Median 58–123 ms; 68 KB–2,1 MB | `cues` |
| 2 | **MP4-Tabellen** (MP4, M4V, MOV): `moov` (Anfang oder Ende) → Videospur → `stss`, `stts`, `ctts`, `elst`, `mdhd` | beim Einlesen | 48–347 ms, Median rund 159 ms; 0,7–2,7 MB | `mp4` |
| 3 | **Volllesung** wie heute (`Runner.Keyframes`) | nur, wenn 1 und 2 nichts Brauchbares liefern (keine Cues, Prüfung durchgefallen, anderer Container) | 1,6–6,6 min je 3–6 GB | `scan` |
| – | **unmöglich**: die Volllesung fand keinen Keyframe oder die Datei ist kaputt (`transcode.FileError`) | endgültig für diese Dateiversion | – | `none` |

Jede Quelle 1 oder 2 muss vor dem Speichern **zwei Prüfungen** bestehen (F-8, 6.2): die
Plausibilitätsregeln (Schwellen aus der Messung mit Abstand zum Beobachteten: grösste Lücke 30 s
gegen gemessene 10,68 s, Faktor 2,8; die Endlücke ist für den Remux harmlos und darum weit gefasst)
und die Stichprobe gegen echte Keyframes per ffprobe.

**Nur indexwürdige Dateien.** Rang 1 bis 3 laufen nur für eine Datei, die überhaupt remuxt werden
kann und für die `probe.file` heute schon einen Index vorsieht: vorhanden (`missing_since IS
NULL`), Videocodec **H.264, HEVC, AV1 oder VP9** (die Codecs, die `directVideo` je kennt,
`playback/decide.go:250-259`; eine Funktion `transcode.CopyableVideo(codec)` für beide Stellen),
und der Titel ist kein Hörbuch und keine Podcast-Folge (dieselbe Regel wie
`library/probe.go:165-171`). VC-1, MPEG-2, MPEG-4 Part 2 und Ähnliches werden nie remuxt (Grund
`video_codec`); sie bekommen keinen Index, keine Volllesung und erscheinen in keiner Zählung
(Z-10). Diese Definition gilt gleich für Einlesen, Nachhol-Lauf und Abdeckung (4.1). Hörbücher und
Podcast-Folgen sind nur von diesen **automatischen** Wegen ausgenommen (wie heute, um grosse
Podcast-Ordner nicht mit Jobs zu fluten); startet jemand eine Podcast-Folge mit Video, rechnet der
Wiedergabe-Auftrag ihren Index wie heute (2.3).

### 2.2 Einlesen: der Index entsteht mit der Probe

```text
probe.file (Pool general, Priorität 3)
  Dateiversion (size, mtime) aus der Zeile merken
  ffprobe wie heute ─► indexwürdig (2.1) und Container mkv/webm/mp4/m4v/mov?
     nein ─► wie heute (kein Index)
     ja   ─► mediaindex.Read (über paths, Zeitgrenze 10 s)        ┐ alles VOR BeginTx:
              ├─ Index + Plausibilität ok ─► VerifyKeyframes      │ Dateizugriff, Parser,
              │      ok     ─► Ergebnis: Index, source = cues|mp4 │ ffprobe-Stichprobe
              │      falsch ─► Ergebnis: Volllese-Job nötig       │
              └─ kein Index / unplausibel ─► Volllese-Job nötig    ┘
  BeginTx (BEGIN IMMEDIATE, der einzige Schreiber)
     Probe, Spuren, Sidecars wie heute, nur wenn size/mtime noch stimmen (4.2)
     SetMediaFileKeyframes mit dem fertigen Ergebnis, oder probe.keyframes einreihen
  Commit
```

**Die Schreibtransaktion enthält keinen Dateizugriff.** Der Server hat genau eine
Schreibverbindung mit `BEGIN IMMEDIATE` (`store/store.go:72`, `_txlock=immediate`, eine
Verbindung); `handleProbe` hält seine Transaktion heute von `library/probe.go:118` bis `:179`.
Lägen Parser (bis 10 s) und Stichprobe (geschätzt 0,3 bis 1 s) darin, warteten alle Schreiber des Servers,
auch `Start`, Fortschritt und Heartbeats. Darum wird alles, was liest oder einen Prozess startet,
**vor** `BeginTx` gerechnet; die Transaktion schreibt nur fertige Werte. Hat sich die Datei
zwischen Merken und Schreiben geändert (die Bedingung auf size/mtime trifft keine Zeile), gibt der
Handler einen vorübergehenden Fehler zurück und läuft nach dem Backoff mit der neuen Version
(Z-9).

Das gilt **auch beim Reprobe** einer geänderten Datei. Ändert der Scanner Grösse oder mtime einer
Datei, leert er den Index sofort (Z-1, 4.2): zwischen Änderung und Reprobe gibt es keinen Index,
also wird transcodiert, nie mit dem alten Index remuxt.

### 2.3 Start: nie warten

| Zustand der Datei | Start | Hintergrund |
|---|---|---|
| Direct Play möglich | Direct Play wie heute, Index egal | – |
| Remux-Kandidat, Index da (`cues`, `mp4`, `scan`) | **Remux** | – |
| Remux-Kandidat, kein Index (`keyframes_source IS NULL`) | **sofort Transcode**, `reason = keyframes_pending` (F-1) | `probe.keyframes` mit Priorität 9 einreihen oder einen wartenden anheben (`RaiseQueuedPriority`), **auch bei Pause** (F-3, 6.5); der nächste Start remuxt |
| Remux-Kandidat, Index unmöglich (`none`) | **sofort Transcode**, neuer Grund `keyframes_unavailable` | nichts (Z-3) |
| Remux-Kandidat, aber nicht indexwürdig (Hörbuch, Podcast-Folge mit Video) | sofort Transcode, `keyframes_pending` | `probe.keyframes` mit Priorität 9 wie heute: diese Titel bekommen ihren Index nur auf Abruf (die Ausnahme in `probe.file` spart heute einen Job je Datei grosser Podcast-Ordner) |

`Start` liest kein Byte der Datei mehr, das über das hinausgeht, was `Decide` aus der Probe
weiss. Damit entfallen `KeyframeTimeout`, die Option `WithKeyframeIndex` und mit ihnen die
doppelte Volllesung bei gleichzeitigen Starts (Z-5): ohne synchrone Arbeit gibt es nichts mehr zu
bündeln; die Idempotenz des Job-Schlüssels sorgt für einen Auftrag je Datei.

### 2.4 Jobarten

| Jobart | Neu? | Auslöser | Pool | Priorität | Was ein Lauf tut |
|---|---|---|---|---|---|
| `probe.file` | erweitert | Scan, Reprobe, Titel-Aktualisierung in der Pflege | `general` | 3 | wie heute, dazu Rang 1/2 mit Prüfungen (2.2) |
| `probe.keyframes` | **neu geschnitten**: der **Volllese-Job** | `probe.file` ohne brauchbaren Index (1), Wiedergabe (9, trotz Pause), Nachhol-Lauf (1), Offline (5) | **neu `index`, genau 1 Arbeiter** (F-2) | 1 / 5 / 9 | Index schon da → fertig; sonst erst Rang 1/2 versuchen (billig, schützt den Altbestand an wartenden Jobs), dann Volllesung; Fehlerklassen (6.3) |
| `keyframes.backfill` | **neu** | Knopf «Jetzt ausführen» auf der Seite Hintergrundjobs, Zeitplan wiederkehrend und einmalig (F-4, F-6) | `general` | 2, von Hand 7 | ein langer Lauf mit Fortschritt: geht alle Videodateien ohne Index durch, versucht Rang 1/2, reiht nur für den Rest `probe.keyframes` ein (6.6) |

Der Name `probe.keyframes` bleibt, weil Marcos Pause an genau dieser Jobart hängt und die
wartenden Altjobs so ohne Migration weiterlaufen; seine Bedeutung wird enger: **die teure
Volllesung**. Die schnelle Arbeit verschwindet aus ihm in `probe.file` und `keyframes.backfill`.

### 2.5 Was sich ändert

**Für den Zuschauer:** Start einer Datei ohne Index in rund 1 s statt 20 s, der nächste Start
remuxt; schärferes Bild bei jedem QSV-Transcode. In einem seltenen Fall gibt es eine kurze
Unterbrechung: wird eine Datei während einer Remux-Wiedergabe ersetzt und muss die Sitzung neu
aufgebaut werden, startet der Player die Wiedergabe an derselben Stelle selbst neu (6.4, 7.4),
statt wie heute mit falschen Segmenten weiterzulaufen.

**Für den Administrator:** die Streams-Seite zeigt je Stream den **Grund** der Umwandlung; die
Bibliotheksseite zeigt je Bibliothek die **Index-Abdeckung** («2 112 von 2 419 Videodateien,
307 ausstehend, 0 nicht möglich»); die Seite Hintergrundjobs hat die Karte **«Keyframe-Index
nachholen»** mit «Jetzt ausführen» und Zeitplan, im Zeitplan-Dialog jedes Zeitplan-Jobs ein Feld
**«Einmalig am»**, und ein Wiedergabe-Auftrag, der trotz Pause läuft, ist als solcher markiert.

### 2.6 Invarianten

| Nr. | Invariante | Wer sie hält |
|---|---|---|
| I-1 | **Ein gespeicherter Index ändert sich je Dateiversion nicht mehr, und er gehört zu genau der Version, aus der er gerechnet wurde.** Geschrieben wird nur, wenn `keyframes_source IS NULL` **und** Grösse und mtime der Zeile noch die sind, die der Job vor dem Lesen gemerkt hat; geleert nur, wenn sich die Datei ändert (Grösse oder mtime, gleich über welchen Pfad der Scanner sie erkennt). Einzige Ausnahme: `none` darf die Titel-Aktualisierung in der Pflege zurücksetzen (O-16) | eine Schreibfunktion in `library` (4.2), Rebuild-Regel (6.4) |
| I-2 | **Jeder Eintrag ist ein echter Video-Keyframe**, seine Zeit stimmt auf 2 ms mit der pts überein, die ffmpeg beim Demuxen sieht, und **Eintrag 0 ist der erste Video-Keyframe der Datei** (Basis von Segment 0, `pipeline.go:164-170`) | Spurfilter, Plausibilität, Stichprobe (6.2) |
| I-3 | **Höchstens eine Volllesung gleichzeitig** im ganzen Server | Pool `index` mit einem Arbeiter; Offline liest nicht mehr selbst (6.8) |
| I-4 | **`Start` wartet nie auf einen Index** | 6.4 |
| I-5 | **Ziel, keine Garantie: die Bitratengrenzen der Leiter und des Profils halten auf QSV.** `-maxrate R -bufsize 2R` erlaubt über 4 s rechnerisch bis 1,5 R; gemessen lag die gleitende 4-s-Spitze bei höchstens 7,9 Mbit/s (R = 8), 2-s-Spitzen bis 8,8 Mbit/s. Das ist ein Messbefund über fünf Clips, keine Invariante; Schritt 1 und 8 messen nach | QSV-Argumente (6.9), Messung (Anhang A.3) |

---

## 3. Konflikt mit Spec, Plan und ADRs

| Stelle | Heute | Mit diesem Konzept |
|---|---|---|
| Spec W-3 (4K-HEVC im Handy-Browser < 5 s) | 20 s bei fehlendem Index | **Erfüllt**: Remux rund 0,4 s, Transcode rund 1 s, 4K-HDR auf QSV 2,4 bis 3,1 s (8). Keine Spec-Änderung |
| ADR 0003, «Konsequenzen»: «der Aufrufer berechnet den Index zuerst (`Runner.Keyframes`) oder kodiert um» | `Start` rechnet synchron | **Nur noch «kodiert um»**; der Index kommt aus dem Container oder dem Volllese-Job. ADR 0022 ist Teilnachfolger dieses Absatzes; Schnittregeln, Toleranz 2 ms und Spulmodell bleiben wörtlich |
| **Entscheid 15 F-1 (Konzept 09, ADR 0017): «Pause ist ein Merkmal der Jobart», `ClaimJob` nimmt keine Läufe pausierter Jobarten** | ohne Ausnahme | **Abweichung für genau einen Fall (F-3):** ein von der Wiedergabe ausgelöster `probe.keyframes`-Auftrag wird **trotz Pause** beansprucht und von einer Pause **nicht** angehalten. Begründung: die Pause schützt das NAS vor der Masse der Volllesungen; ein einzelner Auftrag, den ein Zuschauer gerade braucht, ist genau die Arbeit, für die der Index existiert, und Marco hat «Wiedergabe geht vor» entschieden. Die Abweichung ist eng: nur dieser Auftrag, nur über ein Merkmal am Lauf (`jobs.override_pause`, 4.3), das nur `QueueKeyframesForPlayback` setzt, und die Seite zeigt ihn als «für Wiedergabe, trotz Pause». Zwei weitere Einschränkungen legen F-3 **enger aus als Marcos Wortlaut** («läuft trotzdem») und sind darum Rückfragen, nicht Umsetzung: höchstens drei solche Aufträge gleichzeitig (O-7) und kein neuer Auftrag nach einem Fehlschlag in den letzten 24 h (O-17). ADR 0022 hält die Abweichung fest, ADR 0017 bekommt einen Hinweis |
| ADR 0017, Pools `general`, `download`, `notify` | drei Pools, keine Grenze je Jobart | **Vierter Pool `index` mit genau einem Arbeiter** (F-2). Kein neuer Claim-Mechanismus; der Pool trennt zudem die minutenlange Volllesung von den allgemeinen Arbeitern |
| Konzept 09 F-4, ADR 0017: «Intervalle statt cron-Ausdrücken», «verpasste Läufe holen nicht nach» | Takt, Fenster, Ein/Aus | **Erweitert um den einmaligen Termin** (F-6): ein Datum mit Uhrzeit je Zeitplan-Jobart, unabhängig vom Takt. Kein cron. Der Termin ist ein Versprechen, kein Takt; ein verpasster Termin wird darum einmal nachgeholt, wenn er höchstens 24 h zurückliegt (O-5) |
| ADR 0002, fünf Zustände, `MaxAttempts` | – | **Unverändert.** «Index unmöglich» ist ein Merkmal der Datei (`keyframes_source = 'none'`), kein Jobzustand |
| Entscheid vom 26. Oktober 2026 (`RaiseQueuedPriority`, kein Abbruch) | `QueueKeyframesForPlayback` bricht ab (Z-4) | **Eingehalten**: Anheben statt Abbrechen |
| Plan 3.5 (Spulen, Remux an Quell-Keyframes) | Index aus der Volllesung | Index meist aus dem Container; ADR 0022 |
| CLAUDE.md Regel 2 (SQL auf `media_files` nur in erlaubten Paketen) | – | Schreibfunktion in `library`, Zählung der Abdeckung in `library` oder `admin`; der Lesepfad der Wiedergabe bleibt `GetVisibleMediaFileProbe` (enthält `visible_items`, `queries/scoped/playback.sql:5-9`, liefert heute `id, probe, keyframes`) und bekommt die Spalten `keyframes_source` und `keyframes_at` dazu (`visibility.MediaFileProbe`, `visibility/playback.go:25-28`, ebenso); die Download-Quelle `GetDownloadSource` (`queries/system/offline.sql:220`) bekommt `keyframes_source` |
| Vertrag der HLS-Endpunkte (`/playback/sessions/{id}/…`, `api/openapi.yaml:1914`) | 401 (Token widerrufen) und 404 | **Neue Antwort 409** mit Problem-Code `playback_source_changed`, wenn eine Remux-Sitzung nach einer Dateiänderung nicht neu aufgebaut werden kann (6.4); `x-visibility: scoped` bleibt |
| CLAUDE.md Regel 3 (`os/exec` nur in `transcode`) | – | Der Parser (`internal/mediaindex`) startet keinen Prozess; die Stichprobe ist eine neue Methode von `transcode.Runner` |
| CLAUDE.md Regel 4 (Dateien nur über `paths`) | – | `probe.file` öffnet die Datei über die Wurzel, die es für die Sidecars schon offen hat (`library/probe.go:87-92`) |
| CLAUDE.md Regel 11 (keine Nicht-Ziele, nichts vorziehen) | – | Kein Nicht-Ziel berührt |
| CLAUDE.md Regel 12 (Entscheid im Server) | – | Grund, Abdeckung und Termine rechnet der Server; der Client zeigt an |
| Konzept 11 (Verwaltung, Vertrag 1.7.0) | – | Keine Überschneidung im Vertrag (11 erweitert `Dashboard`, `JobCounts`, `AdminStream.posterUrl`; dieses Papier `Library`, `JobSchedule`, `JobPool`, `JobRun`). Beide fassen die Streams-Seite an; wer zuerst gebaut wird, setzt die Spalte «Grund» (O-15) |
| Konzept 13 «Abspielseite» (Fassung 1, nicht freigegeben) | – | V-15 (Grund im Player) nutzt dieselben Codes wie 7.1; V-21 (Trickplay) erwägt, sich an den Keyframe-Durchlauf zu hängen. Dieses Papier legt nur den Index fest (O-1, O-15) |

---

## 4. Datenmodell

### 4.1 Herkunft des Index: Spalte statt Blob-Version

| | Spalte `media_files.keyframes_source` | Formatbyte 2 im Blob (Herkunft im Kopf) |
|---|---|---|
| Migration | ja (`ALTER TABLE ADD COLUMN`, kein Umbau) | nein |
| Zählen je Bibliothek | `GROUP BY keyframes_source`, mit Index | `substr(keyframes, 1, 1)` über jeden Blob, kein Index |
| «unmöglich» ohne Index | `'none'`, Blob NULL | Blob mit Sonderwert |
| Eingefrorene DB-Stände | Spalte NULL, Altblobs bleiben gültig | Version 1 muss weiter gelesen werden, Version 2 kommt dazu |
| Endgültigkeit (I-1) | `WHERE keyframes_source IS NULL` im Update | Bedingung auf den Blob-Kopf |

**Entscheid: Spalte.** Die Abdeckung je Bibliothek muss zählbar sein (F-7), und «unmöglich» ist
eine Aussage über die Datei, kein Blob-Inhalt. Das Blob-Format Version 1 bleibt unverändert.

```sql
ALTER TABLE media_files ADD COLUMN keyframes_source TEXT
    CHECK (keyframes_source IN ('cues', 'mp4', 'scan', 'none'));
```

Bedeutung: `NULL` = noch kein Index für diese Dateiversion; `cues`, `mp4`, `scan` = Index da (Blob
mit ≥ 1 Eintrag); `none` = unmöglich (Blob NULL). Gezählt werden nur **indexwürdige Dateien**
nach 2.1: `missing_since IS NULL AND video_codec IN ('h264','hevc','av1','vp9')` und der Titel
(`items.kind`) ist weder `audiobook` noch `podcast_episode`. Dieselbe Bedingung steht als eine
SQL-Vorlage in `library` und wird von Zählung, Nachhol-Lauf und `probe.file` geteilt, damit
«ausstehend» und «wird nachgeholt» nie auseinanderlaufen. Dateien ohne Video, mit einem nicht
remuxbaren Codec oder ohne Datei auf der Platte erscheinen in keiner Zählung.

### 4.2 Schreibregeln

Es gibt **eine** Schreibfunktion in `library` (`StoreKeyframes`, heute `library/jobs.go:118-127`
mit `*sql.DB`; neu mit einem `DBTX`, damit `probe.file` sie in seiner Transaktion nutzt, und mit
Quelle und gemerkter Dateiversion als Parameter), die alle drei Wege nutzen (`probe.file`,
`probe.keyframes`, `keyframes.backfill`); Wiedergabe und Offline schreiben nicht mehr (6.4, 6.8).

| Query (`queries/system/items.sql`) | Neu | Regel |
|---|---|---|
| `SetMediaFileKeyframes` (ersetzt `UpdateMediaFileKeyframes`) | setzt `keyframes`, `keyframes_at`, `keyframes_source` | `WHERE id = @id AND keyframes_source IS NULL AND size = @size AND mtime = @mtime` – **endgültig und an die Version gebunden** (I-1, Z-9). `@size`/`@mtime` liest der Job **vor** dem Lesen der Datei aus seiner Zeile. 0 Zeilen heisst «Index schon da oder Datei inzwischen geändert» und ist kein Fehler: der Scanner hat bei einer Änderung den Index geleert und `probe.file` für die neue Version eingereiht |
| `UpdateMediaFileProbe` | gleiche Versionsbedingung | `AND size = @size AND mtime = @mtime`; 0 Zeilen → `probe.file` meldet einen vorübergehenden Fehler und läuft nach dem Backoff mit der neuen Version (2.2). Behebt den Wettlauf auch für die Probe selbst (Z-9) |
| `UpdateMediaFileLocation` | Parameter `@reset_index` | **in `library/write.go` entschieden, nicht in `scanner.go`**: `reset_index = op.disk.size != op.known.Size \|\| op.disk.mtime != op.known.Mtime`, gleich über welchen Weg der Scanner die Datei zugeordnet hat (Pfad, `scanner.go:726-734`, oder Identität dev/inode/Grösse mit anderer mtime, `:785-786`). Dann zusätzlich `keyframes = NULL, keyframes_at = NULL, keyframes_source = NULL`; ein Umbenennen oder Verschieben ohne Änderung **behält** den Index (Z-1) |
| `ResetUnavailableKeyframes` | neu | `SET keyframes_source = NULL WHERE id = @id AND keyframes_source = 'none'` – **nur `none`**. Gerufen von der Titel-Aktualisierung in der Pflege (`metadata.Service.Refresh`, `metadata/manage.go:163-200`; in der Oberfläche «Aktualisieren», auf der Jobseite «Datei prüfen in der Pflege», `web/src/locales/de.json:2357`), die heute schon `probe.file` für jede Datei des Titels einreiht. Gültige Indizes (`cues`, `mp4`, `scan`) bleiben stehen (I-1): eine Aktualisierung von Metadaten, auch einer ganzen Serie oder vieler Titel auf einmal, löst keine Volllesung und keinen Sitzungsabbruch aus (O-16) |

### 4.3 `jobs.override_pause`

```sql
ALTER TABLE jobs ADD COLUMN override_pause INTEGER NOT NULL DEFAULT 0
    CHECK (override_pause IN (0, 1));
```

Erwogen und verworfen: eine Schwelle über die Priorität («Priorität ≥ 9 läuft trotz Pause»). Sie
braucht keine Migration, koppelt aber zwei Bedeutungen an eine Zahl; die nächste Jobart mit
Priorität 9 erbte die Ausnahme unbemerkt. Das Merkmal am Lauf ist ausdrücklich, testbar und in der
Oberfläche zeigbar. Gesetzt wird es nicht über ein Feld von `jobs.NewJob`, sondern über eine
eigene Methode (6.5), damit die Regel «nur die Wiedergabe» maschinell prüfbar ist.

### 4.4 Einmaliger Termin: `job_schedules.once_at`

```sql
ALTER TABLE job_schedules ADD COLUMN once_at INTEGER;  -- ms UTC; NULL = kein Termin
```

Erwogen und verworfen: eine eigene Tabelle `job_schedule_once` mit beliebig vielen Terminen. Ein
Termin je Jobart reicht für «heute Nacht einmal nachholen», und `job_schedules` hat schon eine
Zeile je Zeitplan-Jobart. Der Takt (`every_ms NOT NULL`, CHECK 5 min bis 7 d) bleibt Pflicht;
wer nur einmalig will, schaltet den Takt aus (`enabled = 0`), der Termin feuert trotzdem (6.7).
`ADD COLUMN` braucht keinen Umbau der STRICT-Tabelle. Gespeichert wird der Zeitpunkt in ms UTC;
der Server rechnet ihn aus der Wandzeit, die der Administrator in der Zeitzone des Servers eingibt
(5.2), in seiner `time.Location` um – nie der Browser.

### 4.5 Migrationsplan 00032 (vorläufig; Schritte, kein fertiges SQL)

Nach `migrations/TEMPLATE.sql.txt`: `-- +goose NO TRANSACTION`, `PRAGMA foreign_keys=OFF`,
`BEGIN;`, die Schritte, `COMMIT;`, `PRAGMA foreign_key_check`, `PRAGMA foreign_keys=ON`:

1. `ALTER TABLE media_files ADD COLUMN keyframes_source` (4.1).
2. Bestand: `UPDATE media_files SET keyframes_source = 'scan' WHERE length(keyframes) > 1` – ein
   Blob mit mindestens einem Eintrag kam bisher immer aus der Volllesung. Blobs mit nur dem
   Versionsbyte bleiben `NULL`: Audio-Dateien und die drei VC-1-Dateien sind nicht indexwürdig
   (2.1, Z-10) und werden nie angefasst; die VC-1-Dateien haben zwar Cues, aber ohne
   `K`-Pakete wären sie nicht remuxbar. Kein Blob wird gelöscht.
3. `CREATE INDEX media_files_keyframes ON media_files (keyframes_source) WHERE missing_since IS
   NULL AND video_codec IN ('h264','hevc','av1','vp9')` – trägt die Zählung (5.2) und die Suche
   des Nachhol-Laufs (die Bedingung auf `items.kind` kommt über den Join).
4. `ALTER TABLE jobs ADD COLUMN override_pause` (4.3).
5. `ALTER TABLE job_schedules ADD COLUMN once_at` (4.4).

Danach: `task gen` (sqlc: `systemq`, `scopedq`, `appq`), ein eingefrorener Stand
`testdata/db/wiedergabestart.sql` mit Test nach dem Muster von
`internal/store/frozen_hintergrundjobs_test.go`. Der Stand trägt die **Daten dieses Konzepts**:
`keyframes_source` in allen vier Werten (`cues`, `mp4`, `scan`, `none`) und NULL, ein leerer
v1-Blob mit Quelle NULL, ein Lauf mit `override_pause = 1`, eine Zeitplanzeile mit gesetztem
`once_at`; der Test prüft Werte, CHECKs und dass der Index `media_files_keyframes` existiert. Der Server sichert vorher wie
immer mit `VACUUM INTO` (Regel 5). Die Migration löscht nichts und erzwingt keine Neuberechnung;
eine Migration, die Indizes leert, wäre für I-1 gefährlich.

---

## 5. Vertrag (vorläufig 1.8.0, nicht brechend)

### 5.1 Operationen

Keine neue Operation. Geändert werden die Antworten bestehender Operationen und die Beschreibung
des Grundes:

| Operation | `x-visibility` | Änderung |
|---|---|---|
| `listLibraries`, `getLibrary` (und alle Antworten mit `Library`) | `admin` | neues optionales Feld `keyframeIndex` (5.2) |
| `listJobTypes`, `updateJobSchedule` | `admin` | `JobSchedule.once`; `JobScheduleUpdate.onceLocal`, `clearOnce`; `JobPool.pool` kennt `index`; neue Jobart `keyframes.backfill` erscheint in der Liste |
| `listJobRuns` (und `JobRun` überall) | `admin` | neues optionales Feld `overridesPause` |
| HLS-Endpunkte der Sitzung (Playlist, `init.mp4`, `seg/{n}`; `api/openapi.yaml:1914` ff.) | `scoped` (unverändert) | neue Antwort **409** `playback_source_changed` (6.4, 7.4) |
| `Transcoding.reason`, `AdminStream.reason` | – | Beschreibung um `keyframes_unavailable` ergänzt (`api/openapi.yaml:8351-8361`, `:8523`); Typ bleibt `string` |

Jede geänderte Operation bekommt ihren Fall im Vertragstest (`TestContractCoversEveryOperation`
deckt sie schon; neu sind Beispiele mit den Feldern und die 409 der HLS-Endpunkte), der
Schema-Test prüft `x-visibility` wie bisher. Offline-Downloads brauchen **keinen** neuen Code:
ein Download, der auf den Index wartet, bleibt `queued` ohne `error` (6.8).

### 5.2 Schemata

```yaml
LibraryKeyframeIndex:
  type: object
  description: Keyframe index of the files of the library that can be remuxed (concept 14, F-7).
  required: [videoFiles, indexed, pending, unavailable]
  properties:
    videoFiles:  { type: integer, description: Present files with H.264, HEVC, AV1 or VP9 video outside audiobooks and podcasts. }
    indexed:     { type: integer, description: Files with an index (Matroska cues, MP4 tables or full read). }
    pending:     { type: integer, description: Files without an index yet; the next playback transcodes. }
    unavailable: { type: integer, description: Files whose index cannot be built; they are always transcoded. }
```

`Library.keyframeIndex` ist **optional**: es fehlt bei Audio-Bibliotheken. Die Zählung ist eine
Abfrage je Liste (`GROUP BY library_id, keyframes_source` über `media_files` und `items` mit der
Bedingung aus 4.1, Index aus 4.5); bei 2 419 Dateien auf CT 190 erwartet unter einer
Millisekunde, gemessen in Schritt 8.

**Termin.** `datetime-local` im Browser liefert eine Wandzeit ohne Zone, und der Container läuft
oft in einer anderen Zone als der Browser. Darum spricht der Vertrag in der **Wandzeit des
Servers**, wie das Zeitfenster schon in Minuten der Server-Zeitzone spricht:

- `JobSchedule.once` (optional; fehlt ohne Termin): `{ local: string, at: date-time }` –
  `local` im Format `YYYY-MM-DDTHH:MM` in `JobSchedule.timeZone`, `at` derselbe Zeitpunkt als
  Instant (für Anzeige und Sortierung).
- `JobScheduleUpdate.onceLocal` (`string`, `pattern: '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$'`) und
  `clearOnce` (`boolean`, nach dem Muster von `clearWindow`, weil ein JSON-`null` für die
  generierten Clients nicht von einem fehlenden Feld zu unterscheiden ist). Der Server rechnet
  `onceLocal` in seiner `time.Location` um; eine Wandzeit, die es wegen der Sommerzeit nicht oder
  doppelt gibt, nimmt Go nach seiner Regel (`time.Date`), das ist hingenommen.
- Prüfung im Server: frühestens jetzt + 1 min, spätestens jetzt + 366 Tage; sonst **422
  `validation_failed`** mit Feld `onceLocal` und Wert `out_of_range` (Formfehler:
  `invalid_value`), nach dem Muster der bestehenden Prüfung (`jobs/scheduler.go:274-282`,
  `api/openapi.yaml:5380`); kein eigener Problem-Code. Für Jobarten ohne Zeitplan 409 wie bei den
  übrigen Feldern.

`JobPool.pool` (`api/openapi.yaml:11245`) und die Pool-Angabe der Jobart (`:11189`): `enum`
erweitert um `index`. `JobRun.overridesPause` (`boolean`, optional; nur bei `true` gesetzt).
HLS-Endpunkte: `409` mit `Problem`, `code: playback_source_changed`.

### 5.3 Ist das eine brechende Anhebung?

Nein, mit einer Anmerkung. Neue optionale Felder, eine neue Antwort 409 und eine neue Jobart
(Jobnamen sind ein String ohne Enum) brechen keinen Client; ein alter Player behandelt die 409 wie
heute jeden fatalen Netzwerkfehler (ein Ladeversuch, dann Fehlermeldung). **Die Erweiterung des
Enums `pool` in einer Antwort** kann einen fremden Client mit erschöpfender Fallunterscheidung
stören; die eigene Oberfläche wird aus demselben Vertrag generiert und im selben Image
ausgeliefert, fremde Clients für die Verwaltungs-API sind nicht bekannt. Darum Nebenversion,
keine Hauptversion. Release-Notes-Zeile nennt den neuen Pool.

---

## 6. Server

### 6.1 Neues Paket `internal/mediaindex`

**Platz.** Ein eigenes, reines Paket ohne Dateizugriff und ohne Prozess, nur Standardbibliothek:

```go
// Package mediaindex reads the keyframe times a container already carries (Matroska cues, the
// MP4 sample tables) from an io.ReaderAt. It opens no file and starts no process: the caller
// hands it a file opened through paths (rule 4); the check against ffprobe lives in transcode
// (rule 3).
package mediaindex

type Source string // "cues", "mp4"

type Index struct {
	Keyframes []time.Duration // sorted, unique, rounded to ms as in the stored blob
	Source    Source
	Track     uint64 // track number (Matroska) or track ID (MP4) of the video stream used
	Stats     Stats  // bytes read, reads, cue points of all tracks; for logs and docs/perf
}

// ErrNoIndex: the container carries no usable index (no cues, no moov, fragmented MP4, other
// container); the caller falls back to the full read.
var ErrNoIndex = errors.New("mediaindex: no index in container")

func Read(r io.ReaderAt, size int64, container string) (Index, error)
func Check(ix Index, duration time.Duration) error // plausibility rules of 6.2
```

**Warum nicht in `transcode`:** dort wohnen schon `ParseKeyframes` und `fingerprint.go`, aber
`transcode` ist das Prozesspaket (Regel 3, `.golangci.yml:28-34`); ein Parser für fremde Bytes
gehört nicht neben `exec.CommandContext`. **Warum nicht in `library`:** der Parser braucht keine
Datenbank und soll für sich fuzzbar sein. depguard schränkt ein neues Paket nicht ein.

**Matroska (eigener Minimal-Parser, rund 200–300 Zeilen).** EBML-VINT lesen (ID bis 4, Grösse
bis 8 Byte), Segment-Datenbeginn merken (Positionen sind relativ dazu, RFC 9559 Abschnitt 16),
64 KiB am Anfang lesen, ersten SeekHead auswerten und **einen zweiten SeekHead verfolgen**
(mkvmerge; Jellyfins Extraktor scheitert genau daran, Issues #8413, #9052, #13571). Fehlen Info
oder Tracks im SeekHead, die Top-Level-Elemente bis zum ersten Cluster abgehen. Info:
`TimestampScale` (Vorgabe 1 000 000), `Duration`. Tracks: die **erste** Spur mit `TrackType = 1`
(entspricht `v:0` von ffmpeg). Cues: ein Lesezugriff über das ganze Element (≤ 8 MiB, sonst
`ErrNoIndex`); je CuePoint **alle** `CueTrackPositions` prüfen, nicht nur die erste (Jellyfin
liest nur die erste); `CueTime × TimestampScale`, auf ms gerundet. Ohne SeekHead-Eintrag für Cues:
die letzten 1 MiB nach der Cues-ID absuchen (in 338 gemessenen Dateien nie nötig). Segment mit
unbekannter Grösse, keine Cues, Lesefehler → `ErrNoIndex`.

**MP4 (Eigenbau, rund 250 Zeilen).** Top-Level-Boxen lesen (Grösse 1 = 64-Bit, Grösse 0 = bis
Dateiende), `mdat` überspringen, `moov` lesen (≤ 64 MiB), darin die erste `trak` mit
`hdlr = vide`; aus `mdia/mdhd` die Timescale, aus `stbl` `stss`, `stts`, `ctts` (Version 0 und 1)
und aus `edts/elst` den Versatz (leere Edits mit `media_time = -1` addieren Vorlauf in der
Movie-Timescale aus `mvhd`, der erste nicht leere Edit verschiebt um `media_time`; mehr als ein
nicht leerer Edit → `ErrNoIndex`). Zeit eines Keyframes:
`t = (DTS(n) + ctts(n) − media_time) / mdhd.timescale + empty / mvhd.timescale`. Fehlt `stss`, sind
alle Samples Sync (Apple QTFF); `moof` vorhanden (fragmentiert) → `ErrNoIndex`. Diese Formel hat
die Messung in 11 von 11 Dateien exakt bestätigt (Anhang A.2).

**Eigenbau statt Bibliothek.** Geprüft: `remko/go-mkvparse` (MIT, seit 2023 ohne Änderung, nur der
erste SeekHead), `at-wat/ebml-go` (Apache-2.0, kein gezieltes Überspringen belegt),
`abema/go-mp4` (MIT, aktiv, liest gezielt Boxen über `io.ReadSeeker`), `Eyevinn/mp4ff` (MIT,
gross, Schwerpunkt fragmentiert). Für Matroska ist der Eigenbau klar (die Bibliotheken können den
zweiten SeekHead nicht oder nur mit Umwegen). Für MP4 wäre `abema/go-mp4` vertretbar; es spricht
aber Treiber 3 aus ADR 0001 dagegen («wenig Abhängigkeits-Churn»), der Bedarf ist eng (sechs
Boxen), die Formel ist durch die Messung belegt, und ein Paket mit einem Stil ist einfacher zu
fuzzen. **Entscheid: Eigenbau für beide**; `abema/go-mp4` bleibt Rückfall, falls die
MP4-Fixtures in Schritt 2 mehr Sonderfälle zeigen als erwartet (O-12).

**Lesen auf `*os.File` ist nicht per Kontext abbrechbar.** Ein hängendes NFS blockiert die
Goroutine. Darum ruft `library` den Parser in einer eigenen Goroutine mit Zeitgrenze **10 s**
(gemessenes Maximum 0,76 s); läuft sie ab, gilt die Datei als «kein Index» und der Volllese-Job
übernimmt. Die Goroutine endet, wenn der Lesezugriff zurückkehrt. Im Nachhol-Lauf, der viele
Dateien nacheinander öffnet, bliebe bei einem hängenden NFS je Datei eine Goroutine samt offener
Datei liegen; darum beendet der Lauf sich nach **drei Zeitüberschreitungen in Folge** mit einem
Fehler (Backoff der Queue), statt weiterzumachen (6.6).

**Tests ohne `os/exec` im Paket.** depguard verbietet `os/exec` in allen Paketen ausser
`internal/transcode`, Tests eingeschlossen (`.golangci.yml:28-34`). `mediaindex` testet darum nur
mit eingecheckten Byte-Fixtures; der Kreuztest gegen ffprobe und die mit ffmpeg erzeugten
Testdateien liegen in `internal/transcode/mediaindex_cross_test.go`, das `mediaindex` importiert
(Schritt 2).

### 6.2 Prüfung vor dem Speichern (F-8)

**Plausibilität** (`mediaindex.Check`, Schwellen aus der Messung, Anhang A.1):

| Prüfung | «brauchbar» | beobachtetes Maximum |
|---|---|---|
| Einträge der Videospur | ≥ 2 **und** ≥ 1 je 60 s Dauer | min. 5,7 je min |
| grösste innere Lücke | ≤ 30 s | 10,68 s (Faktor 2,8) |
| erster Eintrag | ≤ 5 s | 0,083 s |
| Endlücke (Dauer − letzter Eintrag) | ≤ max(60 s, 2 × grösste innere Lücke) | 42,3 s |
| letzter Eintrag | ≤ Dauer + 1 s | erfüllt |
| sortiert, keine Duplikate, keine negativen Zeiten | Pflicht | erfüllt |
| Grösse des Cues-Elements (Leseschutz) | ≤ 8 MiB | 2,03 MB |

Eine Lücke bis 30 s ist für den Remux korrekt (die Segmente werden länger,
`EXT-X-TARGETDURATION` folgt dem längsten Segment, `playlist.go:11-15`), nur das Spulen wird
gröber; darüber lieber die Volllesung. **Die Endlücke ist für den Remux harmlos**:
`keyframeStarts` hängt den Rest nach dem letzten Keyframe an das letzte Segment
(`transcode/plan.go:291-301`); die Regel fängt nur grob abgeschnittene Cues ab. Fassung 1 hatte
zusätzlich «≤ 5 % der Dauer» und hätte damit das eigene e2e-Medium verworfen: «H264 MKV Long»
dauert 90 s, der letzte Keyframe liegt bei 75 s (`testdata/generate-media.sh:224-231`), Endlücke
15 s = 16,7 %; mit der neuen Regel (max(60 s, 2 × 15 s) = 60 s) wird es aus den Cues indiziert
(Test in Schritt 3).

**Stichprobe** (neue Methode `transcode.Runner.VerifyKeyframes(ctx, abs string, at
[]time.Duration) error`): **ein** ffprobe-Aufruf mit mehreren Intervallen,
`-select_streams v:0 -show_entries packet=pts_time,flags -read_intervals "%+#30,T1%+#1,T2%+#1"`
(spart je Datei zwei Prozessstarts und zwei NFS-Öffnungen):

1. **Anfang** (`%+#30`): die ersten 30 Videopakete der Datei; das erste mit `K` muss Eintrag 0
   sein (I-2, Basis von Segment 0).
2. **Zwei zufällige Einträge** T1, T2 (Zufall mit der Dateikennung als Saat, also in Tests
   wiederholbar; `T%+#1`): das erste Paket ab dem Eintrag muss `K` tragen und auf **2 ms** mit
   dem Eintrag übereinstimmen (`remuxCutDelta`, `pipeline.go:25`).

Abweichung → Index verwerfen, Volllese-Job einreihen, Warnung im Log (Muxer aus
`MuxingApp`/`WritingApp`, ohne Pfad ausserhalb der Wurzel). Die Kosten sind nicht gemessen;
geschätzt 0,3 bis 1 s je Datei über NFS (ein Prozess, drei Seeks). Schritt 3 misst sie (DoD),
Schritt 8 legt sie nach `docs/perf/`; die Wirkung auf einen Erstimport steht in 8. Argumente
festgenagelt in einem Golden-Test wie `pipeline_test.go`.

### 6.3 Der Volllese-Job `probe.keyframes`

- **Pool `index`, genau ein Arbeiter** (`cmd/slouchcat/main.go`, neben `download-1` und
  `notify-1`; die Seite zeigt «1 Arbeiter», nicht einstellbar, Konzept 09 F-9). Er läuft
  **zusätzlich** zu den allgemeinen Arbeitern: eine Volllesung von Minuten belegt keinen
  `general`-Arbeiter mehr. Der Arbeiter `index-1` bekommt wie die allgemeinen
  `OnFinalFailure(notifySvc.JobFailed)` (`cmd/slouchcat/main.go:338`); heute meldet sich ein
  endgültig fehlgeschlagener `probe.keyframes` so, und das soll mit dem Umzug nicht still
  verloren gehen.
- **Handler** (`library/probe.go` `handleKeyframes`, umgebaut):
  1. Zeile laden, Dateiversion (size, mtime) merken. `keyframes_source` schon gesetzt oder Codec
     nicht remuxbar (2.1) → fertig, `jobs.Skip` (I-1; deckt Wiedergabe- und Nachhol-Aufträge,
     die ein `probe.file` überholt hat).
  2. Rang 1/2 mit Prüfungen versuchen (gleiche Funktion wie `probe.file`, 6.2). Das macht die
     auf CT 190 noch wartenden Altjobs billig, ohne sie anzufassen.
  3. Volllesung `Runner.Keyframes` mit Zeitgrenze **2 h** (27,5 GB bei 15 MB/s ≈ 31 min, Faktor 4
     Reserve).
- **Fehlerklassen** (Z-3):

| Ergebnis | Wirkung |
|---|---|
| Keyframes gefunden | speichern, `source = scan`, mit der gemerkten Dateiversion (4.2); hat sich die Datei während der Minuten der Volllesung geändert, trifft das Schreiben keine Zeile und der Lauf endet `done` ohne Index (der Scanner hat die neue Version eingereiht) |
| Volllesung ohne Keyframe (Datei ohne Keyframe-Flags) | `source = none`, `done` |
| `transcode.FileError` (ffprobe meldet kaputte Daten) | `source = none`, `done`, **ohne** Wiederholung (wie `handleProbe`, `probe.go:77-86`) |
| Datei fehlt (`missing_since` gesetzt, `ENOENT`) | `done`, nichts speichern (der Sweep kümmert sich). Solange `missing_since` noch nicht gesetzt ist, zählt die Datei als «ausstehend»; das betrifft die 11 Zeilen aus Z-8 bis zum nächsten Scan (O-10) |
| alles andere (NFS, Zeitgrenze, Abbruch) | Fehler → Backoff, nach 5 Versuchen `failed`, `source` bleibt NULL; der nächste Nachhol-Lauf versucht es wieder, nicht jede Wiedergabe (6.4) |

Fortschritt: wie heute nur die Notiz (Datei relativ zur Wurzel).

### 6.4 Wiedergabe

- **`prepare`** (`sessions.go:489-516`) fragt den Index nur noch nach: `(blob, source,
  keyframes_at)` aus `GetVisibleMediaFileProbe` (3, Regel 2). Fälle wie 2.3. `ensureKeyframes`
  wird zu einer reinen Nachschlage-Funktion; `KeyframeTimeout`, `keyframeTimeout`,
  `WithKeyframeIndex` (`main.go:521`) entfallen.
- **`queueKeyframes`** (`sessions.go:551-555`) ruft `library.QueueKeyframesForPlayback` wie heute
  mit `context.WithoutCancel`. Diese Funktion (`library/jobs.go:91-116`) wird neu geschrieben und
  ruft die eigene Queue-Methode `EnqueueOverridingPause` (6.5). Ein Statement, kein Zählen und
  Einreihen in zwei Schritten (sonst entstehen bei gleichzeitigen Starts vier Ausnahmen):

  ```sql
  -- jobs.sql, InsertJobOverridingPause (behind Queue.EnqueueOverridingPause): the exception only below the cap (O-7)
  INSERT INTO jobs (type, priority, idempotency_key, payload, override_pause, run_after, …)
  VALUES (@type, 9, @key, @payload,
          (SELECT count(*) < 3 FROM jobs WHERE override_pause = 1 AND state IN ('queued','running')),
          @now, …)
  ON CONFLICT (idempotency_key) WHERE state IN ('queued','running') DO NOTHING;
  ```

  Hält ein **wartender** Job den Schlüssel, hebt die neue Query `RaiseQueuedJobForPlayback` ihn
  auf `max(priority, 9)`, setzt `override_pause` nach derselben Obergrenze und **`run_after =
  min(run_after, now)`**: ein Job im Backoff (bis 80 s) soll für die Wiedergabe nicht warten (kein
  Abbruch mehr, Z-4). Ein **laufender** bleibt, wie er ist. Beide Statements laufen auf der einen
  Schreibverbindung, SQLite serialisiert sie; die Obergrenze ist damit atomar. Für `source =
  'none'` wird nichts eingereiht.
- **Kein neuer Auftrag nach einem Fehlschlag** (Rückfrage O-17): ist der letzte
  `probe.keyframes` dieser Datei in den letzten 24 h `failed`, reiht die Wiedergabe nicht neu ein;
  das übernimmt der Nachhol-Lauf. So kann ein Zuschauer, der eine kaputte Datei fünfmal startet,
  nicht fünf Volllesungen auslösen.
- **Rebuild** (`sessions.go:644-711`): eine als Transcode begonnene Sitzung bleibt Transcode
  (`recordedDecision`, `ForceTranscode`, `:746`), wie heute. Eine als **Remux** begonnene Sitzung
  prüft vor dem Neuplanen: fehlt der Index oder ist `keyframes_at` **jünger als `started_at` der
  Sitzung** (`migrations/00005_playback_sessions.sql:22`), hat sich die Datei geändert (I-1 lässt
  keinen anderen Weg zu; seit Fassung 2 setzt die Titel-Aktualisierung nur `none` zurück, also
  nie einen Index, mit dem remuxt wurde). Dann beendet der Server die Sitzung und antwortet an
  Playlist, `init.mp4` und `seg/{n}` mit **409 `playback_source_changed`**; der Player startet die
  Wiedergabe an der aktuellen Stelle neu (7.4). Heute wartet dieser Fall bis 20 s und
  transcodiert dann auf dem 4-s-Raster, das zur Playlist des Clients nicht passt (Z-2).

  **Warum ein eigener Code und ein Neustart im Client** (Kritik M-3): Der Player kennt heute nur
  zwei Wege. Ein 401 ruft `onRevoked()` und beendet die Wiedergabe mit «von der Verwaltung
  beendet» oder «Zeitlimit» (`web/src/features/player/VideoPlayer.tsx:214-219`, `:591-594`); ein
  404 ist ein fataler Netzwerkfehler mit einem `startLoad()` und danach `setFailed(true)`
  (`:595-604`). Beides wäre für «die Datei wurde ersetzt» falsch oder unnötig hart. Der Player
  hat aber schon den Weg, eine neue Sitzung an einer Position zu beginnen: den Wechsel von
  Version, Spur oder Qualität (`leaveSession`, `switchPlayback`, `:715-730`). Ein Neustart mit
  unveränderter Wahl ist darum wenige Zeilen Client-Code und einen Vertragseintrag teuer; die
  Alternative «ehrlich beschreiben und mit eigener Meldung abbrechen» brauchte ebenfalls einen
  Code und eine Client-Änderung, liesse den Zuschauer aber selbst neu starten. Der Fall ist
  selten (Dateiwechsel während einer Remux-Wiedergabe **und** ein Rebuild durch Leerlauf oder
  Neustart), darum genügt ein einziger automatischer Versuch: scheitert auch der, zeigt der Player
  die gewöhnliche Fehlermeldung.
- **Sicherheitsnetz** `startHLS` bei `ErrNoKeyframes` (`sessions.go:400-417`) bleibt.

### 6.5 Pause-Ausnahme im Claim (F-3)

```sql
-- ClaimJob (queries/system/jobs.sql:46-61), neue Bedingung statt Zeile :57
AND (j.override_pause = 1
     OR j.type NOT IN (SELECT t.type FROM job_types AS t WHERE t.paused_at IS NOT NULL))
```

Dazu drei Stellen:

1. **Pause hält ihn nicht an.** `runList.cancelType` (`jobs/registry.go:216-232`) überspringt
   Läufe mit `override_pause`; ebenso die Pausenprüfung im Heartbeat (`jobs/worker.go:122-126`,
   Rückfall nach ADR 0017).
2. **Nur ein Setzer.** Kein öffentliches Feld an `jobs.NewJob` (das Paket `jobs` liest und
   schreibt das Merkmal selbst, ein Muster auf einen Feldnamen wäre nicht durchsetzbar), sondern
   eine eigene Methode `Queue.EnqueueOverridingPause(ctx, NewJob)` samt
   `RaiseQueuedJobForPlayback`. Ein `forbidigo`-Muster `EnqueueOverridingPause` gilt in allen
   Paketen ausser `internal/jobs` und `internal/library` (`.golangci.yml`, wie das Muster für
   `os/exec`), und ein Test im Paket `library` prüft, dass `QueueKeyframesForPlayback` sie ruft.
3. **Sichtbar.** `JobRun.overridesPause` (5.2); die Seite zeigt unter «Läuft gerade» den Zusatz
   «für Wiedergabe, trotz Pause», und der Pausen-Banner der Jobart sagt «Wiedergabe-Aufträge
   laufen trotzdem».

Der Index `jobs_claim` trägt die Abfrage weiter; die zusätzliche Spalte ist im `WHERE`, nicht in
der Sortierung. `claim_bench_test.go` misst vorher und nachher.

### 6.6 Nachhol-Lauf `keyframes.backfill` (F-4)

```go
jobs.Type{
	Name: "keyframes.backfill", Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral,
	Trigger: jobs.TriggerSchedule, Priority: 2,
	Every: 7 * 24 * time.Hour,                 // the maximum of job_schedules.every_ms
	Seed:  func(context.Context) (jobs.ScheduleSeed, error) { return jobs.ScheduleSeed{Enabled: false}, nil },
	Run:     s.runKeyframesBackfill,  // one job, key "keyframes.backfill", RaiseQueuedPriority
	Handler: s.handleKeyframesBackfill,
}
```

- **Zeitplan aus, Takt 7 Tage** als Vorgabe (O-6): der Knopf «Jetzt ausführen» ist der
  Normalfall, Marco schaltet den Takt oder einen Termin selbst ein. Ein Zeitfenster («nachts»)
  gibt es seit Konzept 09.
- **Run** reiht genau **einen** Lauf ein (Schlüssel `keyframes.backfill`) und hebt einen wartenden
  an (Vertrag von `Type.Run`, `registry.go:71-74`).
- **Handler**: Cursor über die indexwürdigen Dateien mit `keyframes_source IS NULL` (dieselbe
  Bedingung wie die Zählung, 4.1), nach `id`, Batches à 200; je Datei Dateiversion merken, Rang 1/2
  mit Prüfungen **ausserhalb** jeder Transaktion, dann eine kurze Schreibtransaktion je Datei
  (M-1-Regel aus 2.2); gelingt es, speichern; sonst `probe.keyframes` mit Priorität 1 einreihen
  (Idempotenz verhindert Doppelte). Fortschritt `done/total` mit Notiz (Datei relativ zur Wurzel),
  wie Konzept 09 6.2. Drei Zeitüberschreitungen des Parsers in Folge (hängendes NFS, 6.1) beenden
  den Lauf mit Fehler; die Queue wiederholt ihn nach dem Backoff.
- **Pause und Abbruch** wie jede Jobart: der Lauf ist wieder `queued`, der nächste Claim beginnt
  von vorn und überspringt Erledigtes (die Bedingung `IS NULL` ist der Fortschritt).
- **Erwartung CT 190:** 298 MKV × (Median 58 ms Cues + geschätzt 0,3–1 s Stichprobe) ≈ 2 bis
  5 min, gelesen rund 46 MB statt 390 GB. Weil alle 298 Cues für die Videospur haben, sollte kein
  Volllese-Job folgen; der eine MP4 ohne Index läuft über Rang 2. Die drei VC-1-Dateien sind nicht
  indexwürdig (Z-10) und werden nicht angefasst, die 11 Zeilen ohne Datei (Z-8) enden je ohne
  Index, bis der Scan `missing_since` setzt. Abnahme ist darum «ausstehend = 0» nach einem Scan
  und einem Nachhol-Lauf (Schritt 8), nicht «alles indexiert».

### 6.7 Scheduler: einmaliger Termin (F-6)

Erweiterung von `jobs/scheduler.go` (Regeln dort im Paketkommentar, je mit Test):

- **Fällig** ist eine Zeile, wenn `enabled = 1 AND next_run_at <= now` (wie heute) **oder**
  `once_at IS NOT NULL AND once_at <= now`.
- **Feuern über den Termin**: `Run` mit der normalen Priorität der Jobart, dann
  `UPDATE job_schedules SET once_at = NULL, last_fired_at = @now WHERE type = @type AND once_at =
  @fired_once_at` – die Bedingung auf den gelesenen Wert verhindert, dass ein Termin, den der
  Administrator zwischen Lesen und Feuern neu gesetzt hat (`MarkJobScheduleFired` folgt dem Lesen
  der fälligen Zeilen, `jobs/scheduler.go:208-243`), mitgelöscht wird; **`next_run_at` bleibt** (ein Termin verschiebt den Takt nicht, wie
  «Jetzt ausführen», Konzept 09 F-5). Fallen Termin und Takt in denselben Tick, feuert `Run`
  einmal (die Idempotenz von `Run` deckt es ohnehin).
- **Das Zeitfenster gilt nicht** für den Termin: wer 03:30 sagt, meint 03:30.
- **`enabled = 0`** hält den Termin nicht auf; Ein/Aus gilt dem Takt.
- **Pausierte Jobart**: der Termin wird **ausgelassen** und verfällt (`once_at = NULL`, Log
  «Termin ausgelassen, Jobart pausiert»), wie die Pause Takt-Läufe auslässt (O-4).
- **Verpasst** (Server war aus): beim Start feuert ein Termin, der **höchstens 24 h** zurückliegt,
  einmal nach `StartDelay` der Jobart oder sofort; ältere verfallen mit Log (O-5).
- **`UpdateSchedule`** nimmt `onceLocal`/`clearOnce` entgegen und rechnet die Wandzeit in der
  `time.Location` des Schedulers in ms UTC um (5.2); der Scheduler liest die Zeile bei
  jedem Tick, ein Termin wirkt also spätestens 60 s nach seiner Zeit.
- Zeitzone: gespeichert in ms UTC, ein- und ausgegeben als Wandzeit in der Zeitzone des Servers
  (`JobSchedule.timeZone` gibt es schon), dazu der Instant `once.at`.

Tests mit injizierter Uhr: «feuert am Termin», «Termin verschiebt `next_run_at` nicht», «Termin
ignoriert das Fenster», «Termin feuert bei `enabled = 0`», «Termin bei Pause verfällt», «verpasster
Termin ≤ 24 h feuert beim Start, älterer nicht», «Termin und Takt im selben Tick feuern einmal»,
«ein zwischen Lesen und Feuern neu gesetzter Termin bleibt stehen», «Wandzeit in Europe/Zurich
wird korrekt in UTC gespeichert».

### 6.8 Offline-Downloads

`offline.Service.keyframes` (`offline/job.go:263-279`) rechnet nicht mehr selbst (Z-6, I-3):

| Index | `download.prepare` |
|---|---|
| da | Remux-Plan wie heute |
| `none` | kodieren wie heute ohne Index (`decideWith(…, false)`, `offline/profiles.go:165`) |
| fehlt | `probe.keyframes` mit Priorität 5 einreihen (**ohne** Pause-Ausnahme: F-3 gilt der Wiedergabe), Lauf mit `jobs.Skip` beenden und den Nachfolger mit wachsendem Abstand einreihen (10 min, 30 min, dann stündlich), wie beim Warten auf Budget (`job.go:103`); nach **24 h** seit `downloads.created_at` (`migrations/00013_offline.sql:55`) kodieren (O-8) |

Einzelheiten:

- **Zustand während des Wartens:** der Download bleibt `queued`, `error` bleibt leer. Damit
  braucht es keinen neuen Fehlercode im Vertrag (`DownloadState.error`, `api/openapi.yaml:9579`);
  der Client zeigt wie beim Warten auf Budget «wartet». Ein eigener Code «wartet auf den
  Keyframe-Index» wäre genauer, ist aber eine Vertragsänderung ohne Nutzen für den Zuschauer.
- **Lesepfad:** `GetDownloadSource` (`queries/system/offline.sql:220`) liefert zusätzlich
  `keyframes_source`, damit `none` von «fehlt» zu unterscheiden ist.
- **Verlauf:** der wachsende Abstand ergibt rund 26 übersprungene Läufe je Download und Tag statt
  144 bei festen 10 min; sie zählen als `skipped` und fallen nach 30 Tagen mit `jobs.prune`
  (Konzept 09, 4.4).

`offline.Encoder.Keyframes` (`offline/offline.go:55-62`) entfällt. `planFingerprint`
(`job.go:336`) bleibt: ein Download, der nach dem Warten remuxt statt kodiert, beginnt bei Segment 0,
was richtig ist.

### 6.9 Ratensteuerung (F-5)

**QSV, nicht-CBR-Zweig** (`transcode/pipeline.go:389-395`), R = Leiterwert bzw. das niedrigere
`MaxBitrate` des Ziels:

```text
-c:v h264_qsv -preset faster -b:v <0.75*R> -maxrate <R> -bufsize <2*R> -look_ahead 0
  -profile:v high -level:v 41 -g 9999 -forced_idr 1
```

1080p: `-b:v 6000000 -maxrate 8000000 -bufsize 16000000`; 720p: `-b:v 3000000 -maxrate 4000000
-bufsize 8000000`. **Kein `-global_quality` mehr im HLS-Zweig.** Heute teilen HLS- und CBR-Zweig
`-preset veryfast` (`pipeline.go:392-393`); **`faster` gilt nur im HLS-Zweig**, der CBR-Zweig der
Downloads (`-preset veryfast -b:v R`, `:389-393`) bleibt Byte für Byte, wie er ist, ebenso
`download_test.go:48-60`.

**Kleine Raten.** Profilgrenzen können R weit unter die Leiter drücken
(`playback/quality.go:127`, `max(min(r.Bitrate, limit − audio), 1)`). Regel:
`b = max(0,75 · R, min(R − 1 kbit/s, 1 Mbit/s))`. Über 1,33 Mbit/s ist das 0,75 · R; darunter
rückt das Ziel gegen R, bleibt aber immer 1 kbit/s darunter, denn `b == maxrate` liesse
`select_rc_mode()` CBR wählen. Hintergrund ist Jellyfins Untergrenze «Bit rate under 1000k is not
allowed in h264_qsv»; ob die UHD 630 sie braucht, misst Schritt 1 bei 480p (R = 2 Mbit/s) und 360p
(R = 1 Mbit/s) (O-13). Der Server darf die
Kombination «`global_quality` mit `maxrate` ohne `-b:v`» nie wieder erzeugen; ein Test prüft das
für alle Backends und Leiterstufen.

Gemessen auf CT 190 (Anhang A.3, Mittel über vier Clips): **SSIM 0,9816 → 0,9904, PSNR 44,1 →
48,3 dB**, gleitende 4-s-Spitze **≤ 7,9 Mbit/s** bei R = 8, VBV-Puffer höchstens zu 9 % belegt,
Tempo **11,9 bis 13,0 × Echtzeit** je Stream (heute 14,4 bis 16,3 ×), erstes Segment **0,35 bis
0,40 s** (heute 0,29 bis 0,38 s). Begründung des Entscheids: nur diese Einstellung hält
gleichzeitig Qualität **und**, gemessen, die Bitratengrenzen der Leiter und des Profils (I-5: Messbefund, keine Garantie). ICQ wäre auf
ruhigem Material sparsamer (Silo 1,3 statt 5,9 Mbit/s), kennt aber keinen Deckel (Aladdin bis 11
Mbit/s gleitend); 0,5 × R fällt bei harten Szenen unter den heutigen Stand (SSIM P5 0,923), 1,0 × R
reisst die Leiter (bis 10,5 Mbit/s); `look_ahead 1` bricht ohne `-extra_hw_frames` den
HEVC-QSV-Decoder nach 22 Bildern ab; `-extbrc 1` setzt doppelt so viele IDR-Bilder.

**VAAPI** (`:420-421`): heute VBR mit Ziel = Deckel. Vorschlag analog `-rc_mode VBR -b:v 0.75*R
-maxrate R -bufsize 2*R` (O-2). Auf AMD nicht gemessen; Mesa kennt kein ICQ, VBR mit
`target_percentage` 75 ist dort der übliche Weg. Auf CT 190 ist der Intel-iHD-VAAPI-Pfad messbar
(DoD Schritt 1).

**NVENC** bleibt (`-rc vbr -cq 23 -b:v 0 -maxrate R`; der Modus ist richtig, `bufsize` wird im
CQ-Modus verworfen und darf stehen bleiben). Ehrlich vermerkt: ohne VBV-Puffer hält NVENC die
Leiter nur im Mittel; Aladdin erreichte lokal 11,1 Mbit/s gleitend über 4 s bei R = 8 (Anhang A.3).
Das ist ein eigener Befund (O-19), nicht Gegenstand dieses Papiers. **libx264** bleibt. Der Kommentar `pipeline.go:301-305` wird
je Backend richtiggestellt.

---

## 7. Oberfläche

### 7.1 Streams-Seite: der Grund (F-7)

`web/src/features/admin/StreamsPage.tsx:75-130` bekommt die Spalte **«Grund»** nach «Modus»;
leer bei Direct Play. Der Text kommt aus denselben Schlüsseln wie die Stream-Info des Players
(`StreamInfo.tsx:15`, `de.json`/`en.json` um `:585`), damit ein Grund nur einen Text hat. Zwei
Änderungen an den Texten: neu `keyframes_unavailable` («Kein Keyframe-Index möglich»), und
**geändert** der Text von `keyframes_pending` von heute «Keyframe-Index wird erstellt»
(`web/src/locales/de.json:585`) zu «Keyframe-Index fehlt noch» – bei pausierter Jobart oder nach
einem Fehlschlag wird er nämlich gerade nicht erstellt. `keyframes_unavailable` kommt zudem in die
Liste `reasons` der Stream-Info (`web/src/features/player/StreamInfo.tsx:9-23`), sonst zeigt der
Player ihn nicht. Ein unbekannter Code erscheint roh (Server und Client können für einen Moment
verschieden alt sein). Keine Vertragsänderung ausser der Beschreibung (5.1). Der bestehende
Playwright-Test der Seite bekommt die Spalte; eine neue Seite entsteht nicht (Regel 8 betrifft neue
Seiten, der Tastaturtest wird trotzdem nachgeführt).

### 7.2 Bibliotheksseite: Index-Abdeckung (F-7)

Je Videobibliothek eine Zeile unter der Scan-Angabe: **«Keyframe-Index: 2 112 von 2 419
Videodateien · 307 ausstehend · 0 nicht möglich»**. Sind alle da, nur «Keyframe-Index
vollständig». Bei `pending > 0` ein Link «Nachholen auf der Seite Hintergrundjobs», der zur Karte
`keyframes.backfill` springt (der Knopf selbst steht nach F-4 dort, nicht hier). Kein Polling
zusätzlich zur heutigen Abfrage der Seite.

### 7.3 Seite Hintergrundjobs

- **Neue Karte «Keyframe-Index nachholen»** (`keyframes.backfill`, Gruppe Bibliothek) mit «Jetzt
  ausführen», «Zeitplan …», Fortschritt «Dateien geprüft: 120 von 307» und der Notiz.
- **Karte `probe.keyframes`** heisst neu «Keyframe-Index (ganze Datei lesen)»; der Pool «Index ·
  1 Arbeiter» steht im Kopf neben den anderen Pools.
- **Zeitplan-Dialog** (`JobScheduleDialog.tsx`) aller Zeitplan-Jobs: neuer Abschnitt **«Einmalig
  am»** mit Datum und Uhrzeit (natives `input type="datetime-local"`). Der eingegebene Wert ist
  eine **Wandzeit in der Zeitzone des Servers** und geht unverändert als `onceLocal` an den Server
  (5.2); der Browser rechnet nichts um, der Dialog nennt die Zone («Zeitzone des Servers:
  Europe/Zurich») wie beim Fenster. Dazu «Termin entfernen» (`clearOnce`); die Karte zeigt
  «Termin: Sa, 8. Nov., 03:30» aus `once.local` neben «Nächster Lauf». Der Fehler
  `validation_failed` mit Feld `onceLocal` und Wert `out_of_range` bekommt einen eigenen Text
  («Termin muss zwischen jetzt und einem Jahr liegen»).
- **Pause**: der Banner einer pausierten Jobart mit Wiedergabe-Ausnahme sagt «Wiedergabe-Aufträge
  laufen trotzdem»; Läufe mit `overridesPause` zeigen den Zusatz «für Wiedergabe, trotz Pause».
- Texte nur aus `web/src/locales/{de,en}.json` (Regel 9), Farben nur aus Tokens; Komponenten aus
  `web/src/ui`. Playwright: `admin-jobs.spec.ts` um den Weg Tab → «Einmalig am» → Enter → Escape
  erweitern (sichtbarer Fokus, keine Fokusfalle); Storybook-Baselines des Dialogs neu.

### 7.4 Player: Neustart bei `playback_source_changed`

`web/src/features/player/VideoPlayer.tsx` behandelt im `Hls.Events.ERROR`-Handler (`:589-606`)
vor dem 401-Zweig eine Antwort **409 mit `playback_source_changed`**: einmal je Sitzung ruft er den
bestehenden Wechselweg mit unveränderter Wahl an der aktuellen Position (`leaveSession`,
`switchPlayback`, `:715-730`); der Server legt eine neue Sitzung mit dem neuen Index oder als
Transcode an. Ein zweites 409 in der neuen Sitzung zeigt die gewöhnliche Fehlermeldung. Kein neuer
Text nötig; während des Neustarts zeigt der Player wie bei einem Spurwechsel den Ladezustand. Tests:
Vitest mit einem simulierten 409 im Fehler-Handler (ein Neustart, Position übergeben, kein
`onRevoked`), dazu ein Server-Test in `playback` (Rebuild mit jüngerem Index → 409). Ein
Playwright-Test ist nicht vorgesehen: den Fall müsste der Test durch Ersetzen einer Datei während
einer Remux-Wiedergabe und einen Rebuild erzwingen, was im e2e-Stack nicht verlässlich geht.

---

## 8. Leistung

| Fall | Heute (CT 190) | Nach dem Umbau | Beleg |
|---|---|---|---|
| Start, Remux-Kandidat **ohne** Index | **20 016 ms** + 350 ms bis `seg/0` | **≈ 1 s** (Transcode: erstes Segment 0,35–0,40 s plus Sitzung und Playlist) | 1.1; Anhang A.3 |
| nächster Start derselben Datei | wieder bis 20 s, wenn der Job pausiert war | **≈ 0,4 s** (Remux) | Index nach 0,1–1 s (Rang 1/2) oder nach der Volllesung |
| Start mit Index | ≈ 0,35 s bis `seg/0` | unverändert | 1.1 |
| 4K-HDR, Transcode auf QSV | erstes Segment 2,4–3,1 s (CPU-Tonemapping) | unverändert (nicht Ziel dieses Papiers) | Anhang A.3 |
| Einlesen einer neuen MKV (`probe.file`) | ffprobe wie heute, dann Job mit Priorität 1, ggf. Tage später | + 36–757 ms Cues (Median 58–123) + geschätzt 0,3–1 s Stichprobe (ein ffprobe-Aufruf), alles vor der Schreibtransaktion | Anhang A.1; Schätzung, Schritt 3 misst |
| Erstimport einer ganzen Bibliothek (2 419 Dateien wie CT 190) | ffprobe je Datei, danach Tage an Volllese-Jobs | zusätzlich rund 0,4–1,1 s je Datei im Pool `general`, bei vier Arbeitern **rund 4 bis 11 min** mehr für den ganzen Import; dafür entfallen die Volllesungen (auf CT 190 Stunden bis Tage) | Schätzung aus Anhang A.1 und 6.2, Schritt 8 misst |
| Bestand nachholen (298 Dateien) | 2,6 h (42 MB/s) bis 7,2 h (15 MB/s) Volllesung | **23,6 s** Cues, mit Stichprobe rund 2–5 min | Anhang A.1; Stichprobe geschätzt |
| NAS-Last je Datei | ganze Datei (3–27 GB) | Median 68–153 KB, P95 0,8–1,3 MB; 0,012 % der Bytes | Anhang A.1 |
| gleichzeitige Volllesungen | bis zur Arbeiterzahl (4) plus Offline | **höchstens 1** | I-3 |
| QSV-Tempo je Stream | 14,4–16,3 × Echtzeit | 11,9–13,0 ×; zwei Streams je 6,9–7,5 × | Anhang A.3 |
| QSV-Bitrate 1080p im Mittel | im Mittel 1,7 Mbit/s, je Clip 0,7–3,9 (CQP, schwankt mit dem Material) | ≈ 6 Mbit/s (0,75 × R), gleitende 4-s-Spitze gemessen ≤ 7,9 | Anhang A.3 |

**W-3** ist für SDR-Quellen voraussichtlich mit grossem Abstand erfüllt, für 4K-HDR auf QSV
voraussichtlich mit rund 2 s Abstand. «Voraussichtlich», weil die Zahlen aus einem einzelnen
ffmpeg ohne Sitzung, Playlist und Ton stammen (Anhang A.3); Schritt 8 misst den ganzen Start im
Browser. Zwei gleichzeitige 4K-HDR-Streams auf 1080p laufen auf den 6 Kernen des CT nur mit 1,09 ×
Echtzeit je Stream. Das ist das CPU-Tonemapping, nicht die Ratensteuerung; ein Gewinn dort braucht
GPU-Tonemapping und ist ein eigenes Thema.

**Datenvolumen.** QSV liefert neu fast konstant 0,75 × R: bei 1080p rund 2,7 GB je Stunde statt
im Mittel 0,8 GB (je nach Material 0,3 bis 1,8 GB). Profile mit Bandbreitengrenze bleiben geschützt (R ist dann deren Wert); im LAN ist
es der Preis für das scharfe Bild (F-5).

**Messung** nach `docs/perf/wiedergabestart.md`: Schritt 1 (QSV und VAAPI auf CT 190, inkl. 480p
und 360p), Schritt 3 (Dauer von `probe.file` mit Cues und Stichprobe über 50 Dateien), Schritt 8
(Start von Silo S01E03 in Firefox vorher und nachher, Nachhol-Lauf über den Bestand,
`listLibraries` mit Zählung, `ClaimJob` im Benchmark).

---

## 9. Sicherheit

### 9.1 Ein Parser für fremde Bytes

Mediendateien kommen von aussen (Downloads, Release-Gruppen). `mediaindex` vertraut keiner Grösse:
feste Obergrenzen für jeden Lesezugriff (64 KiB Kopf, Cues ≤ 8 MiB, `moov` ≤ 64 MiB, Tracks ≤ 1
MiB), keine Allokation nach einer Grössenangabe über der Obergrenze, Verschachtelungstiefe ≤ 8,
VINT-Länge geprüft, Positionen innerhalb `size`, Überläufe bei `CueTime × TimestampScale` und bei
`stts`-Summen geprüft, höchstens 1 000 000 Einträge. **Fuzz-Tests** `FuzzReadMatroska` und
`FuzzReadMP4` mit den Golden-Fixtures als Saat; CI läuft den Saatkorpus, lokal `go test -fuzz`
für eine Minute je Ziel als DoD von Schritt 2. Ein Panic im Handler fängt der Arbeiter ab
(`jobs/worker.go:177`); der Parser soll trotzdem keinen auslösen.

### 9.2 Dateien nur über `paths`

`probe.file` und `keyframes.backfill` öffnen die Datei über `paths.Open(root)` und `Root.Open(rel)`
(Regel 4); `..` und Symlinks aus der Wurzel hinaus verwirft `paths`. Der Parser bekommt nur den
`io.ReaderAt`.

### 9.3 `os/exec` nur in `transcode`

`VerifyKeyframes` nutzt `exec.CommandContext` mit Argumentliste (Regel 3); die Zeiten der
`-read_intervals` formatiert der Code aus `time.Duration`, der Pfad ist ein eigenes Argument.

### 9.4 Die Pause-Ausnahme lässt sich nicht missbrauchen

Nur der Server setzt `override_pause`, und nur über `QueueKeyframesForPlayback` (6.5). Den Weg
dahin öffnet nur ein Zuschauer mit gültiger Sitzung, gewähltem Profil und sichtbarem Titel (`Start`
prüft Sichtbarkeit wie heute; nicht sichtbar heisst 404). Ein Zuschauer, der dreihundert Titel
ohne Index anklickt, könnte trotzdem Marcos Pause aushebeln; darum **höchstens drei** Aufträge mit
Ausnahme gleichzeitig, atomar in einem Statement geprüft (6.4; Rückfrage O-7), kein neuer Auftrag
nach einem Fehlschlag in den letzten 24 h (6.4; Rückfrage O-17), und die Volllesung bleibt auf
einen Arbeiter begrenzt (I-3). Ohne die zwei Rückfragen bliebe nur die dritte Schranke: dann liefe
während einer Pause höchstens eine Volllesung nach der anderen, aber beliebig viele.

### 9.5 Protokoll und Sichtbarkeit

Fortschrittsnotizen und Warnungen nennen Pfade relativ zur Wurzel (wie heute, `probe.go:232`) und
laufen über den Redaction-Handler (Regel 10). Abdeckung, Termine und Pools sind `x-visibility:
admin`; kein neuer `scoped`-Pfad, keine neue Aussenverbindung.

---

## 10. Schnitt in Schritte

Jeder Schritt ein Commit, `task ci` grün, DoD-Belege mit Befehl und Ausgabe. Zweig ab dem dann
aktuellen `main`. Schritt 1 kann als eigener PR vorab gehen (10.1).

### Schritt 0 — Entscheide und Papiere

Frage 20 (F-1 bis F-8, die O-Punkte mit Marcos Antworten) ins Entscheidungsprotokoll; **ADR 0022
«Keyframe-Index aus dem Container und Wiedergabestart ohne Wartezeit»** als Teilnachfolger von
ADR 0003 (Absatz «Konsequenzen», Index-Herkunft) mit Nachtrag zu ADR 0017 (Pause-Ausnahme für
Wiedergabe-Aufträge, Pool `index`, einmaliger Termin); Hinweise in ADR 0003 und 0017. Nummern
endgültig vergeben (Kopf, O-1). Auf CT 190 `SELECT last_error, count(*) FROM jobs WHERE type =
'probe.keyframes' AND state = 'failed' GROUP BY 1` auslesen und die Fehlerklassen in 6.3 prüfen
(O-9). `git diff --stat docs/spec/` leer.

### Schritt 1 — Ratensteuerung QSV (und VAAPI nach O-2)

Ohne Vertrag, ohne Migration. `transcode/pipeline.go` (QSV-Zweig, ggf. VAAPI-Zweig, Kommentar
`:301-305`), Golden-Strings `pipeline_test.go:57`, `:59`, `:122-125`, Test «nie `global_quality`
mit `maxrate` ohne `-b:v`». DoD: `task ci`; auf CT 190 im Container mit dem neuen Image ein
30-s-Ausschnitt von Silo mit `-loglevel verbose` zeigt `RateControlMethod: VBR`, `TargetKbps:
6000; MaxKbps: 8000`; Bitrate, gleitende 4-s-Spitze und SSIM gegen Anhang A.3; dieselbe Messung für
480p und 360p mit der Regel für kleine Raten (6.9, O-13) und, falls O-2 «ja», für `h264_vaapi`
auf der UHD 630; der CBR-Zweig der Downloads ist unverändert (`download_test.go` ohne Änderung).
Ergebnis als Teil 1 von `docs/perf/wiedergabestart.md`. Kein eigener Patch (O-3, Marco: «Alles
zusammen»): Schritt 1 ist ein eigener Commit im gemeinsamen PR; `info.version` und die
Versionsstellen nach `docs/dev/HANDOVER.md:122-124` hebt erst Schritt 6 mit dem Vertrag.

### Schritt 2 — Paket `mediaindex`

`internal/mediaindex` (Matroska, MP4, `Check`), Fixtures **ohne echte Medien im Repo** und
**ohne `os/exec` im Paket** (6.1): (a) im Paket `mediaindex` nur handgebaute Byte-Fixtures von
einigen KB unter `internal/mediaindex/testdata/` (zweiter SeekHead, Audio-Cues vor Video-Cues im
selben CuePoint, Cues ohne Video, fehlende Cues, unbekannte Segmentgrösse, Cues > 8 MiB
angekündigt, leere Edit, `ctts` Version 1 negativ, fragmentiertes MP4, Endlücke wie «H264 MKV
Long»); (b) im Paket `transcode` der Kreuztest `internal/transcode/mediaindex_cross_test.go`: er
erzeugt mit ffmpeg kleine Dateien (MKV und MP4 mit unregelmässigen Keyframes nach
`testdata/generate-media.sh:212-231`, MP4 mit B-Frames und Edit-Liste, MP4 mit `faststart` und
ohne) und prüft `mediaindex.Read` == `Runner.Keyframes` (±1 ms) sowie `mediaindex.Check` = ok.
Fuzz-Ziele mit (a) als Saat. DoD: `task ci`; `go test ./internal/mediaindex/ -run . -v`;
`go test ./internal/mediaindex -run '^$' -fuzz '^FuzzReadMatroska$' -fuzztime 60s` und dasselbe
für `'^FuzzReadMP4$'`, je ohne Fund.

### Schritt 3 — Migration, Schreibregeln, Einlesen

Migration 00032 (4.5), `task gen`, eingefrorener Stand `testdata/db/wiedergabestart.sql` mit
Test (4.5); `transcode.CopyableVideo` für `decide.go` und `library`; `SetMediaFileKeyframes` und
`UpdateMediaFileProbe` mit Versionsbedingung, `UpdateMediaFileLocation` mit `@reset_index` aus
`write.go`, `ResetUnavailableKeyframes` in `metadata.Service.Refresh`; `StoreKeyframes` mit `DBTX`,
Quelle und Version; `transcode.Runner.VerifyKeyframes` (ein Aufruf, mehrere Intervalle) mit
Golden-Test; `probe.file` rechnet Rang 1/2 mit Prüfungen **vor** `BeginTx` und schreibt in der
Transaktion nur. Tests: «Cues gespeichert, kein Volllese-Job», «e2e-Medium ‹H264 MKV Long› wird
aus Cues indiziert» (M-4), «Cues unplausibel → Volllese-Job», «Stichprobe findet Nicht-Keyframe →
Volllese-Job», «VC-1 und Hörbuch werden nicht indiziert, nicht gezählt», «Datei geändert → Index
leer, auch über den Identitätspfad mit anderer mtime», «umbenannt → Index bleibt», «zweites
Schreiben ändert nichts» (I-1), «Index der alten Version wird nach einem Reset nicht gespeichert»
und «Probe der alten Version wird nicht geschrieben, Lauf wiederholt sich» (Z-9), «Refresh lässt
`cues`/`scan` stehen und leert `none`» (M-5), «während der Parser blockiert, gelingt ein anderer
Schreibzugriff» (der Test hält einen Fake-Parser an und schreibt parallel eine Sitzungszeile; so
ist «kein Dateizugriff in der Transaktion» prüfbar), Anpassung `library/probe_test.go`. DoD
zusätzlich: Dauer von `probe.file` über 50 Dateien auf CT 190 (Stichprobenkosten, 6.2).

### Schritt 4 — Jobarten, Pool, Pause-Ausnahme, einmaliger Termin

Pool `index` mit einem Arbeiter und `OnFinalFailure` (`main.go`, `WithJobControl`-Zählung);
`handleKeyframes` neu (6.3, Fehlerklassen, `none`, Versionsbedingung); `keyframes.backfill` mit
`Run`, Handler, Fortschritt, Abbruch nach drei Zeitüberschreitungen (6.6); `override_pause` in
`ClaimJob`, `cancelType`, Heartbeat; `Queue.EnqueueOverridingPause` und
`RaiseQueuedJobForPlayback` (Obergrenze atomar, `run_after = min(run_after, now)`);
`QueueKeyframesForPlayback` darauf umgestellt; `forbidigo`-Muster; Scheduler mit `once_at`
(6.7). Tests: «pausiert, aber Wiedergabe-Auftrag wird beansprucht», «Pause hält
Wiedergabe-Auftrag nicht an», «vierter Wiedergabe-Auftrag ohne Ausnahme, auch bei vier
gleichzeitigen Starts», «Anheben eines Jobs im Backoff setzt `run_after` auf jetzt»,
«endgültig fehlgeschlagener Volllese-Job meldet `JobFailed`», «Wiedergabe hebt wartenden Scan-Auftrag an statt abzubrechen»,
«Pool `index` führt nie zwei Läufe gleichzeitig», «FileError → none ohne Wiederholung»,
«Nachhol-Lauf speichert Cues und reiht nur den Rest ein», «Nachhol-Lauf nach Pause überspringt
Erledigtes», die Scheduler-Tests aus 6.7; `claim_bench_test.go` vorher/nachher.

### Schritt 5 — Wiedergabe und Offline

`ensureKeyframes` nur nachschlagen; `KeyframeTimeout`, `WithKeyframeIndex` raus; Grund
`keyframes_unavailable`; Sperre nach Fehlschlag; Rebuild-Regel (6.4); Offline wartet statt zu
lesen (6.8). Tests in `playback/sessions_test.go`: `TestRemuxComputesMissingKeyframes` (`:572`)
entfällt, ersetzt durch «Start ohne Index transcodiert sofort und reiht Priorität 9 mit Ausnahme
ein» (Start ohne Wartezeit: der Test schlägt fehl, wenn `Start` länger als 100 ms auf einen
Fake-Index wartet); `TestRemuxFallsBackWhileKeyframesPending` (`:610`) angepasst; neu «Index
unmöglich → Transcode ohne Auftrag», «Rebuild mit jüngerem Index antwortet 409
`playback_source_changed`»; Offline: «ohne Index bleibt `queued` ohne Fehler und reiht ein»,
«Abstand wächst», «nach 24 h seit `created_at` kodiert». e2e `web/e2e/hls.spec.ts:52` (Remux beim
ersten Abspielen von `H264_MKV_LONG_TITLE`) bleibt grün, weil der Index jetzt beim Einlesen aus den
Cues entsteht und die Endlücken-Regel das Medium annimmt (6.2, Test in Schritt 3); wird er rot,
ist das ein Befund für Schritt 3, kein Grund, den Test zu lockern.

### Schritt 6 — Vertrag 1.8.0 und Server-Handler

`api/openapi.yaml` (5), `task gen`, Abbildung in `admin`/`library`/`playback` (Zählung,
`once`/`onceLocal`/`clearOnce` mit `validation_failed`, Pool `index`, `overridesPause`, 409
`playback_source_changed` an den HLS-Endpunkten), Vertragstest, Fixtures
(`web/src/test/fixtures.ts`, `web/e2e/fixtures.ts`). Web-Minimum: der api-client kompiliert.

### Schritt 7 — Oberfläche

Streams-Spalte und geänderter Text von `keyframes_pending`, `keyframes_unavailable` in
`StreamInfo` (7.1), Abdeckung auf der Bibliotheksseite (7.2), Karte, Pool, Dialog-Feld mit
Wandzeit der Server-Zeitzone, Pausen-Texte (7.3), Neustart des Players bei 409 mit Vitest (7.4);
Sprachdateien; Playwright-Tastaturtests (`admin-jobs.spec.ts`, Streams- und
Bibliotheksseite) nachgeführt; Storybook-Baselines. DoD: `task ci`; `pnpm -C web test locales`;
Playwright-Lauf der betroffenen Specs mit Ausgabe.

### Schritt 8 — Abnahme, Messung, Doku

Image bauen, CT 190 umstellen (Weg wie Konzept 09, 10.1). Marco nimmt ab: Silo S01E03 in Firefox
mit pausiertem `probe.keyframes` starten (Transcode in ≈ 1 s, Auftrag läuft trotz Pause, zweiter
Start remuxt), «Keyframe-Index nachholen» drücken und den Fortschritt sehen, einen Termin «heute
23:00» setzen und am Morgen im Verlauf finden, Bild auf dem Handy scharf. Abdeckung: nach einem
Scan (setzt `missing_since` für die 11 Zeilen ohne Datei, Z-8) und einem Nachhol-Lauf zeigt jede
Videobibliothek **«ausstehend = 0»**; «nicht möglich» darf grösser als 0 sein und wird mit
Dateinamen im Messbericht aufgeführt. Messung nach `docs/perf/wiedergabestart.md` (8). Rohdaten der Recherche
(Anhang A, heute im Scratchpad) als Tabellen dorthin übernehmen. `docs/dev/CURRENT_STATUS.md`,
Release-Notes-Zeile (neuer Pool, 5.3).

### 10.1 Reihenfolge und Vorziehen

**Empfehlung: vor Konzept 11 und 12 bauen.** 11 und 12 verbessern Verwaltungsseiten; dieses
Papier behebt einen Fehler, der jeden Zuschauer trifft, dessen Datei keinen Index hat (auf CT 190
jede siebte bis achte Datei), und ein unscharfes Bild bei **jedem** QSV-Transcode. **Schritt 1
taugt als eigener, sofort lieferbarer Schritt**: er berührt nur den QSV-Zweig in
`pipeline.go` und seine Golden-Tests, braucht weder Vertrag noch Migration, ist auf CI als
Golden-Test und auf CT 190 messbar prüfbar und lässt sich allein zurücknehmen. **Marco hat am
1. November 2026 entschieden: alles zusammen in einem PR** (O-3); Schritt 1 bleibt ein eigener
Commit und wird als erster gebaut, damit die QSV-Messung früh vorliegt. Wird vorgezogen, bekommen
Migration und Vertrag beim Baubeginn die dann nächsten Nummern (00031, 1.7.0), und 11 rückt nach
(Kopf).

---

## 11. Entscheide und offene Punkte

### 11.1 Marcos Entscheide (1. November 2026)

Format wie `docs/planning/04-entscheidungen.md`. Alle Antworten gehen als **Frage 20** ins
Entscheidungsprotokoll, sobald das Papier freigegeben ist (Schritt 0).

| Nr. | Thema | Empfehlung | Antwort | Datum | eingearbeitet in |
|-----|-------|------------|---------|-------|------------------|
| **F-1** | **Start ohne Index.** (a) bis zu einer Grenze warten und rechnen (heute 20 s); (b) sofort transcodieren, Index im Hintergrund, nächster Start remuxt | (b) | **(b) sofort transcodieren**, keine Wartezeit | 1. November 2026 | 2.3, 6.4, 8 |
| **F-2** | **Volllese-Job** (nur für Dateien ohne brauchbare Cues/`stss`): wann und wie viele? | jederzeit, höchstens 1 gleichzeitig | **jederzeit, höchstens 1 gleichzeitig** (NAS-Schutz) | 1. November 2026 | 2.4, 6.3, I-3 |
| **F-3** | **Wiedergabe-Auftrag bei pausierter Jobart** | läuft trotzdem | **ja, Wiedergabe geht vor** – Abweichung von Entscheid 15 F-1 nur für diesen Fall | 1. November 2026 | 3, 4.3, 6.5, 9.4 |
| **F-4** | **Bestand nachholen** | Knopf auf der Seite Hintergrundjobs | **per Knopf auf der Seite Hintergrundjobs, und einplanbar, einmalig wie wiederkehrend** (Erweiterung der Empfehlung) | 1. November 2026 | 2.4, 6.6, 6.7, 7.3 |
| **F-5** | **QSV-Ratensteuerung** (nach Erklärung neu gefragt): (a) fester Zielwert mit Obergrenze (VBR); (b) Qualitätswert ohne Obergrenze (ICQ) | (a) | **(a) VBR `-b:v 0.75×R -maxrate R -bufsize 2×R -preset faster -look_ahead 0`**, kein `-global_quality` im HLS-Zweig, CBR der Downloads unverändert; Grund: die Bitratengrenzen der Profile müssen halten | 1. November 2026 | 6.9, I-5, Schritt 1 |
| **F-6** | **Einmaliger Termin**: nur für den Nachhol-Lauf oder für alle Zeitplan-Jobs? | alle | **für alle Zeitplan-Jobs** (Erweiterung von Konzept 09 F-4, cron bleibt draussen) | 1. November 2026 | 3, 4.4, 5.2, 6.7, 7.3 |
| **F-7** | **Was sieht der Administrator?** | Grund in der Streams-Seite | **Grund in der Streams-Seite und Index-Abdeckung je Bibliothek** (additive Vertragserweiterung) | 1. November 2026 | 5, 7.1, 7.2 |
| **F-8** | **Prüfung der Cues**: (a) blind vertrauen (wie Jellyfin); (b) Plausibilität; (c) Plausibilität und Stichprobe | (c) | **(c) Plausibilitätsregeln und Stichprobe** (2–3 zufällige Einträge per ffprobe gegen echte Keyframes) | 1. November 2026 | 6.2, I-2 |

### 11.2 Offene Punkte (keine Blocker, je mit Empfehlung)

| Nr. | Punkt | Empfehlung |
|---|---|---|
| **O-1** | **Nummernlage.** Das parallel entstandene `docs/planning/13-abspielseite.md` (Zweig `claude/konzept-abspielseite`, `b222dbf`) war zuerst eingecheckt und hält Datei 13, Frage 19 und ggf. ADR 0021. | Dieses Papier ist **Konzept 14** mit **Frage 20 und ADR 0022**; Migration und Vertrag werden beim Baubeginn in Bau-Reihenfolge vergeben (Kopf). Erledigt mit Fassung 1 |
| **O-2** | **VAAPI analog auf 0,75 × R?** Heute VBR mit Ziel = Deckel; auf AMD nicht gemessen | **Ja, in Schritt 1**, gemessen auf dem iHD-VAAPI-Pfad der UHD 630; AMD bleibt als ungemessenes Restrisiko in `docs/perf/` vermerkt. Der Deckel bleibt, nur das Mittel sinkt um 25 % |
| **O-3** | **Schritt 1 als Patch 1.6.1?** Die Regel lautet «jede Anhebung von `info.version` wird getaggt» (`docs/dev/HANDOVER.md:122-124`) | **Marco (1.11.2026): Nein, alles zusammen in einem PR.** Schritt 1 bleibt ein eigener Commit, kein Patch 1.6.1 |
| **O-4** | **Termin während einer Pause** | **verfällt mit Log** (6.7), wie die Pause Takt-Läufe auslässt; Alternative «nach dem Fortsetzen nachholen» überrascht mit einem Lauf zur falschen Zeit |
| **O-5** | **Verpasster Termin** (Server aus) | **nachholen, wenn ≤ 24 h verpasst**, sonst verfallen (6.7); weicht bewusst von «Takt holt nicht nach» ab, weil ein Termin ein Versprechen ist |
| **O-6** | **Vorgabe des Zeitplans `keyframes.backfill`** | **aus, Takt 7 Tage**; Marco schaltet ihn oder einen Termin ein (F-4). Mit Rang 1 im `probe.file` entstehen neue Lücken nur noch bei Dateien ohne Cues |
| **O-7** | **Rückfrage an Marco – Obergrenze der Wiedergabe-Aufträge trotz Pause.** F-3 lautet «läuft trotzdem», ohne Zahl. Eine Obergrenze legt F-3 enger aus | **3** wartend oder laufend, atomar geprüft (6.4); der vierte wird normal eingereiht und wartet auf das Fortsetzen (9.4). Schützt die Pause gegen einen Zuschauer, der viele Titel ohne Index anklickt. Ohne Obergrenze: jeder Wiedergabe-Auftrag läuft, nacheinander im Pool `index` **Marco (1.11.2026): Ja, höchstens 3.** |
| **O-8** | **Offline-Download ohne Index**: wie lange warten? | **24 h** ab `downloads.created_at`, mit wachsendem Abstand (10 min, 30 min, dann stündlich), Zustand bleibt `queued` ohne Fehlercode; dann kodieren (6.8); der Download-Arbeiter liest nie selbst (I-3) |
| **O-9** | **265 fehlgeschlagene `probe.keyframes`** auf CT 190, Ursache unbelegt | in Schritt 0 `jobs.last_error` auslesen; passt eine Klasse nicht in 6.3 (z. B. Zeitgrenze, NFS-Code), die Tabelle dort ergänzen. Nach dem Umbau holt sie der Nachhol-Lauf billig über die Cues |
| **O-10** | **11 DB-Zeilen ohne Datei** unter `/srv/media` | nicht hier; als Befund an die Pflege (`unplayable`/Sweep) bzw. ein eigenes Issue. Für die Abnahme genügt ein Scan vorher, der `missing_since` setzt; dann zählen sie nicht als «ausstehend» (Schritt 8) |
| **O-11** | **Wiedergabe-Auftrag wartet hinter einer laufenden Volllesung** (ein Arbeiter im Pool `index`) bis zu einigen Minuten | **hinnehmen**: der Start transcodiert ohnehin sofort; nur der zweite Start remuxt etwas später. Erst wenn das in der Abnahme stört: Nachhol-Volllesung zugunsten eines Wiedergabe-Auftrags anhalten (Release wie bei der Pause) |
| **O-12** | **MP4-Leser Eigenbau oder `abema/go-mp4`** | **Eigenbau** (6.1); Rückfall auf `abema/go-mp4`, wenn die Fixtures in Schritt 2 mehr Sonderfälle zeigen, als 250 Zeilen tragen |
| **O-13** | **0,75 × R bei 480p (2 Mbit/s), 360p (1 Mbit/s) und kleinen Profilgrenzen nicht gemessen** | Regel aus 6.9: `b = max(0,75 · R, min(R − 1 kbit/s, 1 Mbit/s))` (unter 1,33 Mbit/s nahe an R, nie gleich R, weil das CBR wäre); in Schritt 1 bei 480p und 360p messen. Zeigt die Messung, dass die UHD 630 keine Untergrenze braucht, fällt der zweite Teil weg und es gilt überall 0,75 · R |
| **O-14** | **Datenvolumen** im WAN steigt auf QSV um den Faktor 2 bis 4 (8) | hinnehmen (Folge von F-5); Profile mit Bandbreitengrenze bleiben geschützt; in die Release-Notes |
| **O-15** | **Überschneidung mit Konzept 11 und 13** (Streams-Seite; Gründe im Player V-15; Trickplay V-21 am Keyframe-Durchlauf) | Codes und Texte der Gründe gehören einer Stelle (Locale-Schlüssel des Players); wer zuerst baut, legt sie an. Trickplay hängt sich, falls gewählt, an den Pool `index` (ein Leser je Datei), nicht an `probe.file` |
| **O-16** | **Was setzt die Titel-Aktualisierung in der Pflege zurück?** (`metadata.Service.Refresh`, «Aktualisieren», auf der Jobseite «Datei prüfen in der Pflege»; reiht `probe.file` für jede Datei des Titels ein, bei einer Staffel für die Serie, auch für viele Titel auf einmal) | **Nur `none`** (geändert in Fassung 2, 4.2): so lässt sich eine als unmöglich markierte Datei ohne Dateiänderung neu versuchen, aber eine Metadaten-Aktualisierung leert nie einen gültigen Index, löst keine Volllesung und keinen Sitzungsabbruch aus. Fassung 1 wollte jeden Index leeren |
| **O-17** | **Rückfrage an Marco – kein neuer Wiedergabe-Auftrag nach einem Fehlschlag.** Ist der letzte Volllese-Job einer Datei in den letzten 24 h endgültig fehlgeschlagen, reiht die Wiedergabe nicht neu ein (6.4). Auch das legt F-3 enger aus | **Ja, 24 h**: verhindert, dass wiederholte Starts einer Datei mit NAS-Fehler immer neue Volllesungen trotz Pause auslösen; der Nachhol-Lauf versucht es wieder. Ohne Sperre: jeder Start einer solchen Datei reiht einen Auftrag ein (die Idempotenz hält es bei einem gleichzeitig) **Marco (1.11.2026): Ja, 24 h Ruhe.** |
| **O-18** | **Rückfrage an Marco – Hörbücher und Podcast-Folgen** bleiben von den automatischen Wegen ausgenommen (wie heute in `probe.file`), auch vom Nachhol-Lauf und der Zählung; eine Podcast-Folge mit Video bekommt ihren Index nur beim ersten Abspielen, und der erste Start transcodiert | **Ja, so lassen**: Podcast-Folgen mit Video sind selten, grosse Podcast-Ordner häufig. Alternativ: Podcast-Folgen mit H.264/HEVC wie Filme behandeln (kostet beim Einlesen 0,4–1,1 s je Folge) **Marco (1.11.2026): Ja, ausnehmen.** |
| **O-19** | **NVENC hält die Leiter nur im Mittel** (`-cq` verwirft den VBV-Puffer; Aladdin lokal 11,1 Mbit/s gleitend bei R = 8) | nicht in diesem Papier; als Befund in `docs/perf/wiedergabestart.md` vermerken und bei Bedarf eigenes Thema (z. B. `-rc vbr` mit Puffer statt `-cq`), sobald ein Profil mit Bandbreitengrenze auf NVENC klemmt |

---

## 12. Risiken

### 12.1 Ein falscher Index bricht Segmente

Ein Eintrag, der kein Keyframe ist oder um mehr als 2 ms von der Demux-pts abweicht, lässt
Segmentgrenzen und Playlist auseinanderlaufen (Hänger, Grünbild nach Spulen, hls.js
`bufferAppendError`; ADR 0003, «Fallstricke»). Abwehr in Schichten: Spurfilter, Plausibilität,
Stichprobe mit Eintrag 0, Kreuztest gegen ffprobe in CI, Fuzzing, Endgültigkeit je Dateiversion,
Rebuild-Regel. Restrisiko: ein Muxer, der nur selten falsche Cues schreibt und die Stichprobe
zufällig besteht; die Messung fand in 338 Dateien keinen («Rule of three»: bei 0 Fehlern in 40
Referenzdateien liegt die Fehlerquote mit 95 % unter rund 7,5 %). MakeMKV ist nicht untersucht.

### 12.2 Die Pause-Ausnahme überrascht

Marco pausiert `probe.keyframes` und sieht trotzdem einen laufenden Lauf. Abwehr: Zusatz «für
Wiedergabe, trotz Pause», Banner-Text, Obergrenze 3, Eintrag im Entscheidungsprotokoll.

### 12.3 Mehr Bitrate, mehr Last im WAN

Siehe 8 und O-14. Die Spitze bleibt unter der Leiter; das Mittel steigt.

### 12.4 Hängendes NFS beim Einlesen

`probe.file` wartet höchstens 10 s auf den Parser; die Goroutine bleibt bis zur Rückkehr des
Lesezugriffs bestehen. Der Nachhol-Lauf bricht nach drei Zeitüberschreitungen in Folge ab, damit
sich keine offenen Dateien stapeln (6.6). Bei einem ganz hängenden NFS hängen ohnehin alle Leser; das ist heute nicht
anders.

### 12.5 Drei Konzepte fassen dieselben Dateien an

Konzepte 11, 12 und 13 (Abspielseite) ändern `StreamsPage.tsx`, die Sprachdateien, den
Zeitplan-Dialog oder den Vertrag. Wer zuerst baut, gewinnt; die anderen verankern neu (Kopf,
O-1, O-15). Migration und Vertrag werden beim Baubeginn in Bau-Reihenfolge vergeben.

### 12.6 Was dieses Papier nicht messen konnte

Die Kosten der Stichprobe (geschätzt), QSV bei 480p und 360p, VAAPI auf AMD, MakeMKV-Dateien,
MP4 mit `moov` am Anfang (alle 12 gemessenen hatten es am Ende), Container ausser MKV und MP4.

---

## 13. Review

### 13.1 Kritiker-Review von Fassung 1 (OMC-Critic, 1. November 2026)

Urteil **REVISE, 0 Blocker, 6 Major, 22 Minor**, geprüft gegen den Code im Worktree
`konzept-wiedergabestart-14` (`af61226` auf `5e0e63d`). Jeder Befund wurde vor dem Einarbeiten am
Code nachgeprüft; alle trafen zu. Fassung 2 arbeitet alle ein. **Ganz widersprochen wird keinem
Befund**; wo der Kritiker zwei Wege offenliess oder eine Massnahme vorschlug, die sich anders
besser umsetzen lässt, steht die Abwägung in der Spalte «Einarbeitung» und ist mit **(Abwägung)**
markiert.

| Nr. | Befund (kurz) | Nachgeprüft | Einarbeitung in Fassung 2 |
|---|---|---|---|
| M-1 | Parser und Stichprobe in der Schreibtransaktion von `probe.file`; ein Schreiber mit `BEGIN IMMEDIATE` | `store/store.go:72`, `library/probe.go:118-179` | 2.2 neu (alles Lesende vor `BeginTx`, Transaktion schreibt nur), 6.6 (eine kurze Transaktion je Datei), Schritt 3. **(Abwägung)** Statt eines Tests «Transaktion enthält keinen Dateizugriff», der sich so nicht formulieren lässt, ein Nebenläufigkeitstest: während ein Fake-Parser blockiert, muss ein anderer Schreibzugriff gelingen |
| M-2 | Wettlauf alter Job gegen Reset macht einen veralteten Index endgültig | `queries/system/jobs.sql:3-7`, `library/write.go:368-372` | Z-9 neu; I-1 an die Dateiversion gebunden; `SetMediaFileKeyframes` **und** `UpdateMediaFileProbe` mit `size = @size AND mtime = @mtime` (4.2); Verhalten bei 0 Zeilen je Job (2.2, 6.3); zwei Tests in Schritt 3. **(Abwägung)** size + mtime statt einer neuen Spalte `file_version`: es ist genau das Änderungskriterium des Scanners, braucht keine Spalte und keinen Zähler |
| M-3 | Rebuild-Regel: 401/404 starten den Player nicht neu | `VideoPlayer.tsx:214-219`, `:589-606` | **Entschieden für Variante (a)**: 409 `playback_source_changed` an Playlist, `init.mp4`, `seg/{n}` (3, 5.1, 5.2) und ein automatischer Neustart über den bestehenden Wechselweg (`:715-730`), höchstens einmal (6.4, 7.4). Begründung in 6.4: der Weg existiert schon, (b) bräuchte ebenfalls Code und Client-Änderung und liesse den Zuschauer selbst neu starten. 2.5 ehrlich nachgeführt |
| M-4 | Endlücke «≤ 5 %» verwirft «H264 MKV Long»; «Faktor ≥ 3» falsch | `testdata/generate-media.sh:224-231`, `transcode/plan.go:291-301` | Regel jetzt «≤ max(60 s, 2 × grösste innere Lücke)» (6.2), Begründung «Endlücke für den Remux harmlos»; Faktor-Aussage in 2.1 und 6.2 korrigiert; Test «e2e-Medium wird aus Cues indiziert» in Schritt 3, Byte-Fixture in Schritt 2 |
| M-5 | «Datei prüfen» ist `metadata.Service.Refresh`; Rücksetzen leert gültige Indizes in Serie | `metadata/manage.go:158-200`, `de.json:2357` | `ResetUnavailableKeyframes` setzt **nur `none`** zurück (4.2), Name der Aktion korrigiert (2.4, 4.2, O-16), I-1 nachgeführt, Test in Schritt 3 |
| M-6 | Abnahme «vollständig» unerreichbar; VC-1-Aussage widerspricht I-2; Podcast-/Zählregel uneinheitlich | `research/cues-messung.md:86`, `library/probe.go:165-171`, `playback/decide.go:247-260` | «Indexwürdig» in 2.1 und 4.1 definiert (vorhanden, H.264/HEVC/AV1/VP9, kein Hörbuch, keine Podcast-Folge), gleiche Bedingung für Einlesen, Nachhol-Lauf und Zählung; VC-1 als Z-10, in 4.5, 6.6 und A.1 korrigiert; Abnahme «ausstehend = 0» nach Scan und Nachhol-Lauf (Schritt 8, O-10). **(Abwägung)** Nicht remuxbare Codecs werden aus der Zählung genommen statt als `none` markiert: sie brauchen nie einen Index, und «nicht möglich» soll nur Dateien zeigen, bei denen ein Remux an der Datei scheitert. Hörbücher und Podcast-Folgen als Rückfrage O-18 |
| m-1 | Rebuild braucht `keyframes_at` im Lesepfad | `queries/scoped/playback.sql:5-9` | 3 (Zeile Regel 2), 6.4 |
| m-2 | ffmpeg-Kreuztest in `mediaindex` verletzt depguard | `.golangci.yml:28-34` | Kreuztest nach `internal/transcode/mediaindex_cross_test.go`, `mediaindex` nur mit Byte-Fixtures (6.1, Schritt 2) |
| m-3 | Identitätspfad des Scanners setzt nicht zurück | `library/scanner.go:785-786` | `reset_index` in `write.go` aus size/mtime, unabhängig vom Pfad (4.2); Test in Schritt 3 |
| m-4 | Offline-Warten unvollständig | `offline/job.go:96-125`, `api/openapi.yaml:9579` | 6.8: Zustand `queued` ohne Fehler, 24 h ab `created_at`, `GetDownloadSource` mit Quelle, wachsender Abstand. **(Abwägung)** Kein neuer Fehlercode: der Kritiker nannte ihn als Möglichkeit, er brächte eine Vertragsänderung ohne Nutzen für den Zuschauer |
| m-5 | Zeitzone von `onceAt` im Browser fehleranfällig | – | Vertrag in Wandzeit der Server-Zone (`onceLocal`, `once.local`, `once.at`), Umrechnung im Server (4.4, 5.2, 6.7, 7.3) |
| m-6 | `once_out_of_range` bricht das Muster | `jobs/scheduler.go:274-282` | 422 `validation_failed`, Feld `onceLocal`, Wert `out_of_range` (5.2, 7.3) |
| m-7 | Wettlauf Termin-Feuern gegen `UpdateSchedule` | `jobs/scheduler.go:208-243` | `… WHERE once_at = @fired_once_at` und Test (6.7) |
| m-8 | `forbidigo` auf `OverridePause:` nicht umsetzbar | `.golangci.yml:61-66` | eigene Methode `Queue.EnqueueOverridingPause`, Muster auf die Methode ausserhalb `jobs`/`library` (4.3, 6.5) |
| m-9 | Obergrenze 3 nicht atomar | – | ein Statement mit Unterabfrage, auf der einen Schreibverbindung (6.4); Test mit vier gleichzeitigen Starts (Schritt 4) |
| m-10 | O-7 und 24-h-Sperre engen F-3 ein | – | beide als **Rückfragen an Marco** markiert (O-7, O-17; Kopf, 3, 9.4) |
| m-11 | Stichprobe als ein ffprobe-Aufruf; Erstimport fehlt in 8 | – | ein Aufruf mit drei Intervallen (6.2), Schätzung 0,3–1 s; Zeile «Erstimport» in 8 |
| m-12 | Preset im CBR-Zweig unklar | `transcode/pipeline.go:392-393` | `faster` nur im HLS-Zweig, CBR Byte für Byte unverändert (6.9, Schritt 1) |
| m-13 | I-5 ist ein Messbefund; NVENC reisst die Leiter | `research/qsv-messung.md` | I-5 als «Ziel, keine Garantie» (2.6, 6.9); NVENC-Befund in 1.4, 6.9 und O-19 |
| m-14 | Uneinheitliche Zahlen zu «heute» | Anhang A.3 | eine Zahl aus A.3: im Mittel 1,7 Mbit/s, je Clip 0,7–3,9 (1.1, 1.4, 8) |
| m-15 | O-13 «`-b:v` = R − 1» unverständlich | `playback/quality.go:127` | konkrete Regel `b = max(0,75 · R, min(R − 1 kbit/s, 1 Mbit/s))` mit Begründung (6.9, O-13) |
| m-16 | Versionsstellen für 1.6.1 fehlen | `docs/dev/HANDOVER.md:122-124` | DoD von Schritt 1 ergänzt |
| m-17 | Index-Arbeiter ohne Fehlermeldung | `cmd/slouchcat/main.go:338` | `OnFinalFailure` für `index-1` (6.3, Schritt 4) |
| m-18 | Eingefrorener Stand zu dünn | `store/frozen_hintergrundjobs_test.go` | Stand mit allen Quellwerten, `override_pause = 1`, `once_at`, Index (4.5) |
| m-19 | Hängendes NFS im Nachhol-Lauf | – | Abbruch nach drei Zeitüberschreitungen in Folge (6.1, 6.6, 12.4) |
| m-20 | Falscher Konzeptverweis im Schema | – | «concept 14, F-7» (5.2) |
| m-21 | Text von `keyframes_pending` stillschweigend geändert; `reasons`-Liste | `de.json:585`, `StreamInfo.tsx:9-23` | Änderung ausdrücklich genannt, `keyframes_unavailable` in `reasons` (7.1, Schritt 7) |
| m-22 | Kleinigkeiten: Fuzz-Befehl, «drei Volllese-Jobs», W-3 «voraussichtlich», `StoreKeyframes` mit `DBTX`, `run_after` beim Anheben | – | Fuzz-Befehl mit Paketpfad (Schritt 2); Satz in 6.6 ersetzt; W-3 als «voraussichtlich, Schritt 8 misst» (8); `StoreKeyframes` mit `DBTX`, Quelle und Version (4.2); `run_after = min(run_after, now)` (6.4) |

Vom Kritiker als in Ordnung bestätigt und unverändert: Remux-Schnitt und I-2 (`pipeline.go:164-170`,
`:25`), die drei Stellen der Pause-Ausnahme, der neue Pool, Regeln 2 und 7, die Migration mit `ADD
COLUMN`, die betroffenen QSV-Golden-Strings.

### 13.2 Was Marco aus Fassung 2 neu entscheiden muss

Drei Rückfragen, weil sie F-3 enger auslegen oder den Umfang festlegen: **O-7** (Obergrenze 3),
**O-17** (24-h-Sperre nach Fehlschlag), **O-18** (Hörbücher und Podcast-Folgen bleiben ausgenommen).
Geändert und zur Kenntnis: **O-13** (konkrete Regel für kleine Raten), **O-16** (nur `none` wird
zurückgesetzt), **O-8** (Warten ohne Fehlercode, wachsender Abstand). Neu als Befund ohne
Entscheid in diesem Papier: **O-19** (NVENC-Deckel).

---

## Anhang A — Messbelege (CT 190, 1. November 2026)

Rohdaten und Skripte liegen bis Schritt 8 im Scratchpad der Recherche-Sitzung
(`research/cues-messung.md`, `cues_probe.py`, `a.jsonl`, `b.jsonl`, `ball.jsonl`, `mp4.jsonl`;
`research/qsv-messung.md` mit `qsv/`); Schritt 8 übernimmt die Tabellen nach
`docs/perf/wiedergabestart.md`. Gemessen nur lesend (DB `mode=ro`, NFS ro), seriell, ein Leser.

### A.1 Matroska-Cues

**Stichprobe A** (40 MKV mit Referenzindex, Seed 42, 40 Releasegruppen, 26 HEVC / 14 H.264,
1,6–16,9 GB, Muxer 21 mkvmerge, 15 ffmpeg, 3 HandBrake, 1 Lavf62; nur Filme):

| Kennzahl | Wert |
|---|---|
| Cue = Referenz-Keyframe (±1 ms) | 59 915 / 59 915, 40/40 Dateien |
| Referenz-Keyframes in den Cues | 59 915 / 59 915, 40/40 Dateien |
| Versatz (Modus) | 0 ms in 40/40 |
| grösste Lücke | identisch zur Referenz in 40/40; Median 10,4 s, Maximum 10,68 s |
| Endlücke | Median 3,7 s, P95 10,1 s, Maximum 42,3 s (identisch zur Referenz) |
| erste Cue | 0 bis 0,083 s, wie die Referenz |
| Wandzeit kalt / warm | Median 123 / 44 ms, P95 443 / 515 ms, Maximum 478 ms |
| gelesen | Median 149 KB, P95 1,30 MB, Maximum 1,33 MB |
| Seeks | 3 (36 Dateien) oder 5 (4 Dateien) |
| zusätzliche Audio-/Untertitel-Cues | 31 von 40 Dateien (bis 47 202 CuePoints bei 2 836 Videopunkten) |

**Alle 298 vorhandenen MKV ohne Index** (289 Serienfolgen, 9 Filme):

| Kennzahl | Wert |
|---|---|
| Cues für die Videospur vorhanden | 298 / 298, 0 Fehler |
| Cues je Datei | 105 bis 19 994, Median 124 |
| grösste Lücke | Median 10,0 s, P95 10,4 s, Maximum 10,43 s |
| Endlücke | Median 5,1 s, Maximum 13,8 s |
| Wandzeit kalt | Median 58 ms, P95 336 ms, Maximum 756 ms |
| gelesen | Median 68 KB, P95 794 KB, Maximum 2,09 MB; Summe 46 MB bei 390 GB (0,012 %) |
| Summe Wandzeit | 23,6 s (Volllesung: 2,6 h bei 42 MB/s, 3,6 h bei 30, 7,2 h bei 15) |
| VC-1 (3 Dateien, Blob leer) | 10 271 bis 19 994 Cues, Lücke 0,584 s; plausibel, aber ffprobe meldet keine `K`-Pakete, also nicht remuxbar und nach 2.1 nicht indexwürdig (Z-10) |

**Nach Muxer:** ffmpeg und HandBrake schreiben eine Cue je GOP (Lücke fest 10,0 s bzw. 10,427 s),
mkvmerge eine je Video-Keyframe inkl. Szenenwechsel (Medianlücke 2–3 s) und zusätzlich Cues für
andere Spuren. Kein Muxer ohne Cues, keiner mit Cues, die Keyframes auslassen. **Serien** (nur 2
mit Referenz): Fenstervergleich der ersten 118 s gegen ffprobe für je einen Muxer: 12/12, 12/12,
12/12, 43/43.

**Volllesung:** Einzelstrom 41,3 bis 42,5 MB/s (64 MiB ab Dateimitte, NAS im Leerlauf); 4,1 /
5,1 / 6,0 GB → 1,6 / 2,0 / 2,4 min (bei 15 MB/s 4,5 / 5,7 / 6,6 min).

### A.2 MP4 (12 Dateien, 11 mit Referenz)

| Kennzahl | Wert |
|---|---|
| mit `ctts` und `elst` exakt | 11 / 11 Dateien, Dichte 100 %, Versatz 0 ms |
| nur DTS (ohne `ctts`) exakt | 10 / 11; «1492 Conquest of Paradise» (Timescale 90 000) nur 201 / 943, Modus −125 ms |
| Lage von `moov` | in allen 12 hinter `mdat` (kein faststart) |
| gelesen | 0,70 bis 2,69 MB (dominiert von `ctts`, 0,7–1,8 MB) |
| Wandzeit kalt / warm | 48–347 ms (Median ≈ 159) / 18–85 ms |
| leere Edit | «Knives Out» (5 Ticks), korrekt berücksichtigt |

### A.3 QSV-Ratensteuerung (UHD 630, jellyfin-ffmpeg 7.1, Container mit 6 Kernen)

Vier Clips à 30 s (Silo HEVC 10 bit dunkel; Aliens HEVC Korn; Aladdin flache Flächen; Dutton 4K
HDR10+ → 1080p), Metrik gegen eine verlustfreie Referenz derselben Kette (misst nur den Encoder),
Paarung nach Bildnummer (Kontrolle: QSV `global_quality 1` SSIM ≥ 0,9966, um ein Bild verschoben
0,9703). Kein VMAF verfügbar.

| Variante | mittl. kbit/s | max. 4-s-Spitze gleitend | PSNR dB | SSIM | SSIM P5 | × Echtzeit (SDR) |
|---|---|---|---|---|---|---|
| heute (`global_quality 23` + `maxrate`, = CQP) | 1 707 | 8 299 | 44,1 | 0,9816 | 0,9671 | 15,2 |
| ICQ 23 `faster` (ohne Deckel) | 2 697 | 10 995 | 46,6 | 0,9879 | 0,9822 | 11,5 |
| VBR 0,5 × R | 3 924 | 5 520 | 46,5 | 0,9864 | 0,9686 | 12,9 |
| VBR 0,75 × R `veryfast` | 5 914 | 7 869 | 48,0 | 0,9898 | 0,9801 | 12,8 |
| **VBR 0,75 × R `faster` (F-5)** | **5 947** | **7 888** | **48,3** | **0,9904** | **0,9819** | **12,4** |
| VBR 0,75 × R `medium` | 5 940 | 7 914 | 48,3 | 0,9904 | 0,9822 | 11,4 |
| VBR 1,0 × R | 8 008 | 10 474 | 48,9 | 0,9915 | 0,9844 | 12,7 |
| LA_ICQ (`look_ahead 1`) | 5 209 | 20 209 | 48,6 | 0,9909 | 0,9868 | 13,0 |

Belege der Ratensteuerung (`-loglevel verbose`, Silo): heute `RateControlMethod: CQP`; ICQ 23 →
`ICQ`, `ICQQuality: 23`; VBR 0,75 → `VBR`, `TargetKbps: 6000; MaxKbps: 8000`.

**Erstes 4-s-Segment** (Segment-Muxer-Argumente des Servers, ohne Audio, inkl.
Geräte-Initialisierung): Silo heute 353–377 ms, VBR 0,75 `faster` 395–396 ms; Aladdin heute
290–311 ms, VBR 0,75 352–377 ms; 4K-HDR → 1080p 2 440–3 107 ms in allen Varianten (CPU-Tonemapping).

**Zwei gleichzeitige Streams** (VBR 0,75 `faster`): 2 × Silo je 6,9 ×; Silo + Aladdin je 7,5 ×;
2 × 4K-HDR → 1080p je 1,09 × (63,5 CPU-s je 30 s); 2 × 4K-HDR → 720p je 2,0 ×.

**VBV-Modell** (Puffer 2 × R, anfangs voll): VBR 0,75 höchstens 0,04 bis 0,09 belegt; VBR 1,0 bis
0,53; LA_ICQ Aladdin 4,73 (Unterlauf, `maxrate` wird ignoriert).

**Sichtbefund** (Standbilder 960 × 540): heute Gewebe und Hautporen weggewaschen, Bart verschwommen
(Silo, Aliens), Blockrauschen in Flächen (Aladdin); VBR 0,75 `faster` und NVENC kaum von der Quelle
zu unterscheiden.

**NVENC zum Vergleich** (RTX 4070, ffmpeg 6.1.1, andere Kettenreferenz, nur Richtwert):
2,3 / 4,8 / 6,6 / 5,1 Mbit/s bei PSNR 50,0 / 50,1 / 46,2 / 49,2 dB; bei ähnlicher Bitrate rund
0,8 bis 1 dB über QSV.

---

## Anhang B — Code-Karte (gegen `5e0e63d`, nach Namen suchen)

| Thema | Stelle |
|---|---|
| Einreihen beim Einlesen | `library/probe.go:68` `handleProbe`, `:165-176`; `library/jobs.go:87-89` `keyframesJob`, `:40` `prioKeyframes` |
| Wurzel und Sidecars offen | `library/probe.go:87-92` (`paths.Open`) |
| Container-Normalisierung | `library/probe.go:320-346` `canonicalContainer` |
| Volllese-Handler | `library/probe.go:225-244` `handleKeyframes` |
| Blob-Format v1 | `library/probe.go:246-291` (`EncodeKeyframes`, `DecodeKeyframes`) |
| ffprobe Volllesung | `transcode/probe.go:311-338` (`Runner.Keyframes`, `ParseKeyframes`) |
| Schema | `migrations/00003_libraries_items.sql:114-116` |
| Schreib-Query | `queries/system/items.sql:91-92` `UpdateMediaFileKeyframes`; `:61-65` `UpdateMediaFileLocation`; `:84-89` `UpdateMediaFileProbe` |
| Scanner «geändert» | `library/scanner.go:726-734`; `library/write.go:349-372` |
| Wiedergabe-Auftrag | `library/jobs.go:91-116` `QueueKeyframesForPlayback`, `:118-127` `StoreKeyframes`, `:182-188` Registrierung |
| Start, prepare, ensureKeyframes | `playback/sessions.go:104`, `:161`, `:175`, `:194`, `:489-516`, `:522-548`, `:551-555`; `:37-39` `KeyframeTimeout`; `playback/playback.go:106` |
| Rebuild | `playback/sessions.go:622-638` `HLS`, `:644-711` `rebuild`, `:729-769` `recordedDecision` |
| Grund | `playback/decide.go:29-31` `ReasonKeyframesPending` |
| Remux-Plan | `transcode/plan.go:202-275` `NewPlan`, `:181-185` `ErrNoKeyframes`, `:291-301` `keyframeStarts`; `transcode/pipeline.go:25` `remuxCutDelta`, `:162-196`, `:218-227` |
| Encoder-Argumente | `transcode/pipeline.go:301-305` Kommentar, `:358-364` NVENC, `:365-395` QSV, `:396-421` VAAPI, `:424-431` libx264 |
| Leiter | `transcode/plan.go:123-145`; `playback/quality.go:20-25` |
| Budget | `transcode/plan.go:94-106`, `:307-330`; `transcode/manager.go:104-115` |
| Claim und Pause | `queries/system/jobs.sql:46-61`; `jobs/registry.go:216-232` `cancelType`; `jobs/worker.go:144-148`, `:177` |
| Pools | `jobs/registry.go:37-42`; `cmd/slouchcat/main.go:332-351`; `config/config.go:261-262` |
| `Type.Run`-Vertrag | `jobs/registry.go:71-74`; `jobs/queue.go:299-308` `RaiseQueuedPriority` |
| Scheduler | `jobs/scheduler.go` (Paketkommentar mit den Regeln); `migrations/00027_jobs_scheduler.sql:52-65` |
| Offline | `offline/job.go:191` `planFor`, `:263-279` `keyframes`, `:336` `planFingerprint`; `offline/offline.go:55-62` |
| Admin-Streams | `web/src/features/admin/StreamsPage.tsx:17-22`, `:75-130`; `web/src/features/player/StreamInfo.tsx:15` |
| Vertrag | `api/openapi.yaml:4` Version, `:8351-8361` `Transcoding.reason`, `:8523` `AdminStream.reason`, `:9342` `Library`, `:11129` `JobSchedule`, `:11161` `JobScheduleUpdate`, `:11189`/`:11245` Pool-Enum |
| Einzige Schreibverbindung | `store/store.go:72` (`_txlock=immediate`, eine Verbindung); Probe-Transaktion `library/probe.go:118`–`:179` |
| Job-Idempotenz | `queries/system/jobs.sql:3-7` (`ON CONFLICT … WHERE state IN ('queued','running')`) |
| Scanner-Identitätspfad | `library/scanner.go:785-786`; Reprobe-Einreihung `library/write.go:368-372` |
| Titel-Aktualisierung | `metadata/manage.go:158-200` `Service.Refresh` (Einreihen von `probe.file` `:193`) |
| Remux-Codecs | `playback/decide.go:247-260` `directVideo` |
| Player-Fehlerwege | `web/src/features/player/VideoPlayer.tsx:214-219` `onRevoked`, `:589-606` Fehler-Handler, `:715-730` `leaveSession`/`switchPlayback` |
| Sitzungsbeginn | `migrations/00005_playback_sessions.sql:22` `started_at` |
| Offline-Zustände | `migrations/00013_offline.sql:40-41`, `:55`; `api/openapi.yaml:9579` `error` |
| Index-Arbeiter und Meldung | `cmd/slouchcat/main.go:338` `OnFinalFailure` |
| Tests heute | `transcode/pipeline_test.go:57`, `:59`, `:122-125`; `transcode/download_test.go:48-60`; `playback/sessions_test.go:572`, `:610`; `web/e2e/hls.spec.ts:52`; `testdata/generate-media.sh:212-231` |
