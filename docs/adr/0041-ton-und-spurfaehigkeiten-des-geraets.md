# ADR 0041: Ton- und Spurfähigkeiten des Geräts in der Wiedergabe-Entscheidung

- Status: gebaut auf dem Zweig `claude/k26-stufe1-a` (21. November 2026, Stufe 1 von Konzept 26),
  **angenommen mit dem Merge**. Die Zahl ist in Konzept 26 reserviert.
- Quelle: Konzept `docs/planning/26-android-tv.md`, Abschnitte 2.3, 5.2 bis 5.4, 7 und 10; Messung
  der Stufe 0 (`docs/perf/K26-android-tv.md`, Punkte 1 bis 4)
- Vertrag: zusätzlich, drei Felder in `PlaybackCapabilities` (Fassung 1.43.0, setzt der Koordinator)
- Keine Migration. Harte Regeln: 1 (Codegen), 3 (`os/exec` nur in `transcode`), 6 und 7 (Vertrag),
  12 (der Server entscheidet)

## Kontext

Bisher meldete ein Client nur Container, Codecs, HLS-Art, Bildschirm und HDR. Für einen Fernseher
mit Tonanlage reicht das nicht: Der Server machte aus jedem Ton, den er neu kodieren musste, Stereo-
AAC, kopierte neben neu kodiertem Video nur AAC, remuxte jede andere als die Standard-Tonspur und
brannte jede Bildspur ein, auch wenn der Player sie selbst zeichnen kann. Die Messung der Stufe 0
fand dazu einen Fehler, der heute jeden Client trifft: Kopiertes oder neu kodiertes AC-3 und E-AC-3
schreibt die Segment-Pipeline nicht («Cannot write moov atom before AC3 packets», ffmpeg endet mit
234). Ausserdem nannte die HLS-Liste für kopiertes DTS, TrueHD, FLAC und Opus AAC als Codec; das
bringt ExoPlayer zum Abbruch.

## Entscheid

1. **Drei zusätzliche Felder in `PlaybackCapabilities`** (Namen der Messung):
   - `maxAudioChannels` (2, 6 oder 8): höchste Kanalzahl des Tonausgangs. Andere Werte gelten als
     nicht gemeldet.
   - `selectsTracks` (Ja/Nein): Der Player wählt Ton- und Untertitelspur in der Originaldatei.
   - `subtitles` (Liste, ffprobe-Namen): Untertitelformate, die der Player selbst zeichnet. Die
     Kurznamen `srt`, `vtt`, `pgs`, `vobsub`, `dvdsub` liest der Server als `subrip`, `webvtt`,
     `hdmv_pgs_subtitle`, `dvd_subtitle`.
   Fehlt ein Feld, gilt das Verhalten von vorher. `hls: mse` ist auch der Wert eines nativen Players
   mit eigenem HLS-Abspieler (ExoPlayer); die Beschreibung im Vertrag sagt das jetzt.
2. **Direktwiedergabe** (`playback/decide.go`):
   - Mit `selectsTracks` erzwingt eine andere als die Standard-Tonspur keinen Remux mehr. Die
     Antwort nennt die gewählte Spur in `selection.audioTrack`.
   - Eine eingebettete Untertitelspur, deren Format in `subtitles` steht, wird bei
     Direktwiedergabe weder eingebrannt noch als WebVTT geliefert. `selection` nennt sie mit
     `subtitleTrack` und **ohne** `subtitleDelivery`, `subtitleUrl` fehlt. Eine Sidecar-Datei, eine
     ausdrücklich verlangte Lieferung (`text`, `burn`) und jede HLS-Ausgabe bleiben wie vorher.
3. **Ton einer HLS-Ausgabe** (`hlsAudio` in `decide.go`):
   - **Remux kopiert**, was der Client meldet und fMP4-HLS trägt: AAC, AC-3, E-AC-3, DTS, FLAC,
     Opus (`transcode.CopyableAudio`). **TrueHD nie** (ffmpeg schreibt es nur experimentell, Media3
     kennt es nicht), ebenso PCM, MP3 und Unbekanntes: Sie werden neu kodiert.
   - **Transcode kopiert** gemeldetes AAC wie vorher und, **wenn der Client `maxAudioChannels`
     meldet**, auch AC-3 und E-AC-3.
   - **Neu kodiert** wird bei `maxAudioChannels` 6 oder 8 und einer Quelle über Stereo in
     Mehrkanal: AC-3 mit 640 kbit/s, wenn der Client `ac3` meldet, sonst AAC 5.1 mit 384 kbit/s;
     bei 8 und einer Quelle über 6 Kanälen AAC 7.1 mit 512 kbit/s. Nie mehr Kanäle als die Quelle
     (`-ac`, ffmpeg wählt die Standard-Anordnung der Kanalzahl und mischt herunter). Sonst
     Stereo-AAC mit 160 kbit/s wie vorher.
4. **Bandbreite** (`playback/quality.go`, `transcode/plan.go`): Stufenwahl, Videobitrate unter einer
   Grenze und `BANDWIDTH` der Master-Liste rechnen mit der Bitrate des Tons, der wirklich
   herausgeht: kopiert die Bitrate der Spur (ffprobe `bit_rate`, sonst das Matroska-Tag `BPS`),
   ohne gemessenen Wert die Obergrenze des Formats (AC-3 640, E-AC-3 1024, DTS 1536, Opus 510,
   FLAC 750 je Kanal, AAC wie vorher 160 kbit/s); neu kodiert die Bitrate des Encoders.
5. **Codec-Angabe** der Master-Liste je Format: `mp4a.40.2` (HE-AAC wie vorher), `ac-3`, `ec-3`,
   `mp4a.a9` (DTS), `fLaC`, `Opus`. Kein Rückfall auf `mp4a.40.2`: Ein Codec ohne Angabe wird nicht
   kopiert (`NewPlan` setzt `AudioCopy` zurück).
6. **AC-3 und E-AC-3 in fMP4** (`transcode/pipeline.go`, `split.go`): Nur wenn der Ton AC-3 oder
   E-AC-3 ist, läuft der mp4-Muxer mit `+delay_moov+frag_keyframe`. Er schreibt das `moov` dann
   erst nach dem ersten Tonpaket und je Keyframe ein Fragment. Die Dateien eines Laufs entsprechen
   darum nicht mehr eins zu eins den Segmenten: Die erste Datei ist `ftyp+moov` allein, wenn das
   erste Segment nur einen Keyframe hat, und dessen Fragment eröffnet die zweite Datei; ein Segment
   mit mehreren Keyframes (Remux) hat mehrere Fragmente. Die Sitzung liest die Dateien eines solchen
   Laufs als einen Strom von Boxen und ordnet jedes Fragment dem Segment zu, in dem sein erstes
   Videobild liegt (Zeit aus `tfdt` und dem Versatz des ersten Bildes, mit der Schnitt-Toleranz des
   Segment-Muxers). Ein Segment ist fertig, wenn seine Datei geschlossen ist (die nächste Datei
   existiert oder ffmpeg ist beendet), sofort, wenn sein Fragment in einer späteren Datei liegt
   (das zurückgehaltene, das der Muxer in einem Zug schreibt), oder wenn ein Fragment eines späteren
   Segments folgt. Neustarts (`-ss`), `-copytb` und `setts` bleiben unverändert; alle anderen
   Formate laufen über den bisherigen Weg.

## Alternativen

- **`delay_moov` für alle Formate:** geht laut Messung, ändert aber den Weg jeder Wiedergabe ohne
  Not. Verworfen; nur AC-3 und E-AC-3 brauchen ihn.
- **Nur `+delay_moov`:** Segment 0 und 1 landen in einem Fragment (Messung). Unbrauchbar.
- **Zuordnung nach Dateinummer mit Sonderfall «erste Datei leer»:** kürzer, aber Zwischen-Keyframes
  im Remux und die Reihenfolge der Leerungen hängen an ffmpeg-Interna. Die Zuordnung nach Zeit
  verlangt nur, was der Vertrag des Segmentierers (ADR 0003) schon zusichert: gleiche Zeitachse in
  jedem Lauf.
- **Dolby-Kopie beim Transcode für alle Clients:** Safari und Edge melden `ac3` und `eac3` schon
  heute und bekämen sonst bei jedem Transcode den Originalton statt Stereo-AAC. Die Bedingung
  `maxAudioChannels` hält das Verhalten der Browser gleich.
- **Eigener Lieferwert für «zeichnet der Player selbst»** in `subtitleDelivery`: wäre eine Änderung
  am Schema der Antwort. Das fehlende Feld sagt dasselbe, und der Vertrag bleibt eng an
  `PlaybackCapabilities`.
- **MP3 kopieren** (`mp4a.40.34` oder `mp4a.6B`): hls.js und Media3 lesen die beiden Angaben nicht
  gleich; neu kodiert läuft es überall.

## Was gemessen ist und was nicht

- Im Emulator (Stufe 0, Media3 1.11.1): Codec-Angaben, Kanalzahlen und Wiedergabe von AAC 2.0/5.1/7.1,
  das Erkennen von AC-3, E-AC-3, DTS (ohne Decoder: stumm), FLAC, Opus; PGS und ASS gezeichnet;
  Spurwechsel ohne Neustart.
- Im Bau (Stufe 1) mit echtem ffmpeg: Remux mit kopiertem AC-3 und E-AC-3 5.1 und Transcode mit
  kopiertem E-AC-3, neu kodiertem AC-3 5.1, AAC 5.1 und AAC 7.1, je vom Anfang und nach Neustarts;
  `ffprobe` an den Segmenten zeigt Codec, Kanalzahl, einen Keyframe am geplanten Start jedes
  Segments und eine Tonspur ohne Lücke innerhalb eines Laufs (`hls_audio_test.go`). Das Prüfprogramm
  `apicheck` spielt die Testdatei als Fernseher direkt, als Remux und als Transcode mit kopiertem
  AC-3. Messwerte: `docs/perf/K26-android-tv.md`, Abschnitt «Stufe 1».
- **Auf den Geräten nachzumessen** (Chromecast, Shield, Fernseher): ob AC-3 5.1 oder AAC 5.1 als
  Ziel beim Neu-Kodieren sauber über HDMI und ARC läuft, ob das Durchreichen von AC-3 und E-AC-3 aus
  HLS so klappt wie aus MKV, und ob 640 kbit/s AC-3 über ARC ankommen.
- **Bekannt, nicht Teil dieser Stufe:** Beim Neustart eines Transcodes fehlt an der Naht Ton, den
  der vorige Lauf noch nicht geschrieben hatte, bis etwa drei Tonrahmen (Stereo-AAC vorher 53 ms
  gemessen, E-AC-3 kopiert bis 96 ms). Der Remux ist davon nicht betroffen.

## Folgen

- Browser senden die neuen Felder nicht und bekommen dieselben Entscheide wie vorher. Für sie ändert
  sich: Ein Remux mit kopiertem AC-3 oder E-AC-3 (Safari, Edge) bricht nicht mehr ab, die
  Codec-Angabe stimmt für jedes kopierte Format, TrueHD und MP3 werden im Remux neu kodiert statt mit
  falscher Angabe kopiert, und die Bandbreite eines Transcodes rechnet mit der gemessenen Bitrate
  einer kopierten AAC-Spur.
- `transcode.Stream` trägt neu `bitRate` je Tonspur; Proben von früher haben sie nicht, dort gilt
  die Obergrenze des Formats.
- Die Apps leiten aus der Antwort ab, was sie tun (Übergabe an Stufe 3): `mode: direct` mit
  `selection.audioTrack` und einem `subtitleTrack` ohne `subtitleDelivery` heisst «Spur in der
  Datei wählen», `mode: hls` heisst «genau die eine Tonspur der Liste».
