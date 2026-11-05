# ADR 0022: Keyframe-Index aus dem Container und Wiedergabestart ohne Wartezeit

- Status: angenommen (5. November 2026); **ersetzt teilweise ADR 0003** (Abschnitt «Konsequenzen
  und Grenzen», Satz zur Herkunft des Keyframe-Index) und **ergänzt ADR 0017** (Pause-Ausnahme für
  Wiedergabe-Aufträge, Pool `index`, einmaliger Termin)
- Quelle: Konzept `docs/planning/14-wiedergabestart.md` (Fassung 2, freigegeben am 2. November
  2026, Abschnitte 2 bis 6); **Frage 20** in `docs/planning/04-entscheidungen.md` mit Marcos
  Antworten F-1 bis F-8 und O-1 bis O-19
- Betroffene Spec-IDs: W-3 (Start in unter fünf Sekunden), A-3 (Hintergrundjobs), Plan 3.5
  (Spulen, Remux an Quell-Keyframes)
- Harte Regeln: 2 (SQL auf `media_files` nur in `library`, `admin`, `visibility`, `offline`),
  3 (`os/exec` nur in `transcode`), 4 (Dateizugriff über `paths`), 5 (Migration 00033 nach Vorlage),
  7 (`x-visibility`, Vertragstest), 12 (Entscheid im Server)

## Kontext

Auf CT 190 dauerte am 1. November 2026 der Start einer MKV ohne gespeicherten Keyframe-Index 20 s:
`Start` las den Index synchron mit ffprobe über die ganze Datei (`KeyframeTimeout`), brach ab und
transcodierte dann. Die Jobart `probe.keyframes`, die den Index im Hintergrund nachholt, war
pausiert. Der Transcode auf QSV war zudem unscharf, weil `-global_quality` zusammen mit `-maxrate`
ohne `-b:v` in ffmpeg CQP mit QP 26 ergibt. Die Messung im Papier (Anhang A) zeigt: Matroska-Cues
und die MP4-Tabellen `stss`/`stts`/`ctts`/`elst` sind in diesem Bestand ein exakter Keyframe-Index
und kosten im Median 58 bis 159 ms statt Minuten.

## Entscheid

1. **Drei Quellen in fester Reihenfolge**, festgehalten in `media_files.keyframes_source`:
   Matroska-Cues (`cues`), MP4-Tabellen (`mp4`), Volllesung (`scan`); `none` heisst «für diese
   Dateiversion unmöglich». Cues und MP4-Tabellen werden nur nach Plausibilitätsregeln und einer
   Stichprobe per ffprobe gespeichert (F-8). Der Parser ist das reine Paket `internal/mediaindex`
   ohne Dateizugriff und ohne Prozess; die Stichprobe ist `transcode.Runner.VerifyKeyframes`.
2. **Der Index entsteht beim Einlesen** (`probe.file`), alles Lesende vor der Schreibtransaktion.
   Indexwürdig sind vorhandene Dateien mit H.264, HEVC, AV1 oder VP9 ausserhalb von Hörbüchern und
   Podcast-Folgen (`transcode.CopyableVideo`).
3. **Ein Index ist je Dateiversion endgültig** (Invariante I-1): geschrieben nur, wenn noch keiner
   da ist und Grösse und mtime noch stimmen; geleert nur, wenn sich die Datei ändert. Einzige
   Ausnahme: die Titel-Aktualisierung in der Pflege setzt `none` zurück.
4. **`Start` wartet nie auf einen Index.** Fehlt er, wird sofort transcodiert (`keyframes_pending`),
   bei `none` ebenso (`keyframes_unavailable`). Damit ersetzt dieser Entscheid in ADR 0003 den Satz
   «der Aufrufer berechnet den Index zuerst (`Runner.Keyframes`) oder kodiert um» durch «der
   Aufrufer kodiert um»; Schnittregeln, Toleranz 2 ms und Spulmodell aus ADR 0003 bleiben.
   Ändert sich die Datei unter einer Remux-Sitzung, antworten die HLS-Endpunkte mit 409
   `playback_source_changed`, und der Player startet einmal an derselben Stelle neu.
5. **Die Volllesung ist der Job `probe.keyframes` im neuen Pool `index` mit genau einem Arbeiter**
   (F-2). Wiedergabe-Aufträge laufen **trotz Pause** der Jobart (F-3), höchstens drei gleichzeitig
   (O-7) und nicht neu nach einem Fehlschlag in den letzten 24 h (O-17). Gesetzt wird das Merkmal
   `jobs.override_pause` nur über `Queue.EnqueueOverridingPause`, die nur `library` rufen darf.
   Das ergänzt ADR 0017 («Pause ist ein Merkmal der Jobart») um genau diese Ausnahme.
6. **Nachholen** mit der Jobart `keyframes.backfill` per Knopf und Zeitplan (F-4); **einmaliger
   Termin** `job_schedules.once_at` für alle Zeitplan-Jobs (F-6): unabhängig von Takt, Ein/Aus und
   Fenster, verfällt bei Pause, wird höchstens 24 h nachgeholt. Das ergänzt ADR 0017 («Intervalle
   statt cron»), cron bleibt draussen.
7. **QSV und VAAPI mit VBR und Deckel** (F-5, O-2): Ziel 0,75 × R, Deckel R, Puffer 2 × R; nie
   `-global_quality` mit `-maxrate` ohne `-b:v`. Der CBR-Zweig der Downloads bleibt.

## Konsequenzen

- Migration 00033 (`keyframes_source`, `jobs.override_pause`, `job_schedules.once_at`), Vertrag
  1.13.0 additiv (Abdeckung je Bibliothek, Termin, Pool `index`, `overridesPause`, 409 an den
  HLS-Endpunkten).
- `KeyframeTimeout` und `playback.WithKeyframeIndex` entfallen; Offline-Downloads lesen nie selbst,
  sie warten bis 24 h auf den Index und kodieren dann.
- Erweitertes Enum `JobPool.pool` kann fremde Clients mit erschöpfender Fallunterscheidung stören;
  die eigene Oberfläche kommt aus demselben Vertrag (Release-Notes-Zeile).
- Mehr Bitrate auf QSV (im Mittel 0,75 × R statt rund 1,7 Mbit/s bei 1080p); Profile mit
  Bandbreitengrenze bleiben geschützt.
- Restrisiko: ein Muxer, der selten falsche Cues schreibt und die Stichprobe besteht; MakeMKV ist
  nicht gemessen.
