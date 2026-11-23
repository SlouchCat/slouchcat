# K26 Stufe 0: Messung für die Android-TV-App

Stand 21. November 2026. Grundlage: Konzept 26 (`docs/planning/26-android-tv.md`), Abschnitte 2.3,
2.6, 3.1, 5, 7, 10 und 13 (F-8, F-13). Gemessen mit einer Wegwerf-App (`com.slouchcat.spike.tv`,
nicht im Repo) und Proben aus synthetischen Testsignalen (lavfi, keine Filme). Kein Code im Produkt
geändert. Was K27 Stufe 0 schon gemessen hat (Media3 für Direkt und HLS, Spulen, WebVTT,
Versionen, Robolectric), ist nicht wiederholt (`docs/perf/K27-android-handy.md`).

**Lage:** Die drei Fernseher (Chromecast oder Google TV Streamer, Nvidia Shield, Fernseher mit
Google TV) waren nicht erreichbar. Gemessen ist darum alles, was der Emulator und der Rechner
belegen; die Entscheide reichen so weit wie die Belege. Was auf den Geräten nachzumessen ist, steht
am Ende als Schritt-für-Schritt-Liste.

**Umgebung:** Linux x86_64, JDK 21, Gradle 9.9.2, AGP 9.5.2, Kotlin 2.4.21, Compose BOM 2026.11.01,
`androidx.tv:tv-material` 1.1.0, Media3 1.11.1 (neueste stabile; 1.12.0 ist Beta), Coil 3.6.3,
Robolectric 4.18.1, alle aus den Maven-Metadaten vom 21.11.2026. Emulator `sc_tv`: Google TV,
Android 14 (API 34), x86, 1920 × 1080 bei 320 dpi (960 × 540 dp), Grafik `swiftshader_indirect`
(Software), ohne Tonausgabe. ffmpeg 7.1.4-Jellyfin (aus dem Server-Image) für die Proben, ffmpeg
6.1.1 des Rechners zur Gegenprobe. Probendateien über einen eigenen HTTP-Server auf Port 8697, im
Emulator `http://10.0.2.2:8697`.

## Übersicht der Entscheide

| Nr. | Punkt | Entscheid | Für Stufe |
|---|---|---|---|
| 1 | Ton in fMP4-HLS | HLS trägt AAC, AC-3, E-AC-3 (kopiert und neu), DTS, FLAC, Opus (nur kopiert); **TrueHD nie**. Codec-Angaben `mp4a.40.2`, `ac-3`, `ec-3`, `mp4a.a9`, `fLaC`, `Opus`. Ziel beim Neu-Kodieren AC-3 640 kbit/s, wenn gemeldet, sonst AAC 5.1 384 kbit/s (7.1: 512). **Heutiger Fehler:** AC-3/E-AC-3 in HLS scheitert, Pipeline braucht `+delay_moov+frag_keyframe` | 1 |
| 2 | Gerätemeldung | `audio` = dekodiert ∪ durchgereicht; Kanalzahl aus Durchreichen und HDMI-Ausgang; drei neue Felder `maxAudioChannels`, `selectsTracks`, `subtitles`; Schalter «Ton» als `Auto`/`Stereo`/`Manual(formats, channels)` | 1, 3 |
| 3 | Untertitel | App meldet `subrip`, `ass`, `webvtt`, `hdmv_pgs_subtitle`, `dvd_subtitle` als selbst gezeichnet: **PGS ja, ASS ja** | 1, 3 |
| 4 | Spuren selbst wählen | `selectsTracks: true` bei Direktwiedergabe; Wechsel in 20 ms ohne Neustart; Zuordnung n-te Spur je Art, Prüfung über Sprache und Titel | 1, 3 |
| 5 | Direkt und HDR | Emulator: H.264 und HEVC Main direkt, kein HEVC Main 10, kein HDR, kein Dolby/DTS. `hdr: true` nur bei HDR10/HLG-Anzeige **und** Main-10-Decoder; F-8 bleibt bei der Empfehlung | 3 |
| 6 | Compose for TV | trägt; Fokusmuster `focusRestorer(fallback)` je Reihe und Leiste (mit `focusGroup`), Startfokus nach dem ersten Bild, Zurück → Leiste → Startbildschirm. Bildrate im Emulator nur Anhaltspunkt | 2b |
| 7 | Oberflächentests | **JVM-Tests (Robolectric) tragen** für Fokus und sichtbaren Fokus; Zurück über den Dispatcher; Emulator nur für Wiedergabe | 2a, 2b |
| 8 | `minSdk` | **28 bestätigt**, `targetSdk` 34; Manifest-Checkliste unten | 2a |

## 1. Tonformate in fMP4-HLS

**Gemessen.** `proben.sh` (Notizordner) baut je Probe eine Quelle (H.264 720p24, Keyframe alle
4 s, 12 s, 5.1-Signal mit einem Sinus je Kanal) und schreibt daraus HLS mit genau den Schaltern
der Pipeline (`-f segment -segment_format mp4`, dieselben `movflags`, `-output_ts_offset 10`,
`-individual_header_trailer 0`); `init.mp4` wird wie in `segmenter.go` am ersten `moof`
abgetrennt. Video im Transcode neu mit `libx264 -preset veryfast … -profile:v high`, im Remux
kopiert. Danach `pruefen.sh` (ffprobe, ganzes Dekodieren) und im Emulator `MeasureTest#audioHls`
(ExoPlayer, 3 s Wiedergabe, Spuren, Decoder, `AudioTrack`-Kanalmaske).

**Heutiger Fehler.** Mit den `movflags` der Pipeline schreibt ffmpeg (7.1.4 und 6.1.1) **kein
AC-3 und kein E-AC-3**: «Cannot write moov atom before AC3 packets. Set the delay_moov flag to fix
this.» Gegenprobe mit einem Wegwerf-Test gegen `NewPlan` und `Manager.Start` (Remux,
`AudioCopy: true`, nicht eingecheckt): `ac3` und `eac3` enden mit «ffmpeg from segment 0: exit
status 234», `aac` läuft. Das trifft heute jeden Client, der `ac3` oder `eac3` meldet und eine
Datei mit Dolby-Ton remuxt (Safari und Edge melden beides über `capabilities.ts`). TrueHD scheitert
zusätzlich an «truehd in MP4 support is experimental»; `-strict -2` als Ausgabeoption wirkt beim
Segment-Muxer nicht, nur `strict=-2` in `-segment_format_options`.

**Lösung, gemessen:**

| `movflags` zusätzlich | Ergebnis |
|---|---|
| `+delay_moov` | Datei 0 = `ftyp+moov` ohne Daten; Segment 0 und 1 landen in **einem** Fragment (Datei 1 = 0–8 s). Unbrauchbar |
| `+delay_moov+frag_keyframe` | Datei 0 = `ftyp+moov`; Datei 1 = zwei Fragmente (Segment 0 und 1); danach ein Fragment je Datei. Segmentstarts 10,00 / 14,00 / 18,00 s (Versatz 10 s) |

Mit `frag_keyframe` schreibt ffmpeg nach dem `moov` sofort das erste Fragment (der Muxer leert bei
`delay_moov` zweimal). Der Segmentierer muss dafür eine erste Datei ohne `moof` als Init annehmen
und eine Datei mit mehreren Fragmenten nach `tfdt` auf die Segmente verteilen; Zwischen-Keyframes
im Remux erzeugen weitere Fragmente in derselben Datei.

**Ergebnis je Probe** (alle mit `+delay_moov+frag_keyframe`, Spalte «Emulator» = ExoPlayer nach 3 s):

| Probe | ffprobe: Eintrag, Kanäle | CODECS | Media3 erkennt | Emulator |
|---|---|---|---|---|
| AAC 2.0 neu, 160 k | `mp4a`, 2 | `mp4a.40.2` | `audio/mp4a-latm` 2 ch | spielt, `c2.android.aac.decoder`, Maske 0xc |
| AAC 5.1 neu, 384 k | `mp4a`, 6 | `mp4a.40.2` | 6 ch | spielt, Maske 0xfc (5.1) |
| AAC 7.1 neu, 512 k | `mp4a`, 8 | `mp4a.40.2` | 8 ch | spielt, Maske 0x18fc (7.1) |
| AAC 5.1 kopiert (Remux) | `mp4a`, 6 | `mp4a.40.2` | 6 ch | spielt |
| AC-3 5.1 neu 448 k / 640 k, kopiert 640 k | `ac-3`, 6 | `ac-3` | `audio/ac3` | `UNSUPPORTED_SUBTYPE`: Video läuft, Ton stumm, kein Fehler |
| E-AC-3 5.1 neu und kopiert, 640 k | `ec-3`, 6 | `ec-3` | `audio/eac3` | wie AC-3 |
| DTS 5.1 kopiert, 1536 k | `mp4a` (OTI 0xA9), 6 | `dtsc` oder `mp4a.a9` | `audio/vnd.dts` (beide) | stumm, kein Fehler |
| DTS mit heutigem `mp4a.40.2` | | falsch | hält es für AAC | **Abbruch** `ERROR_CODE_DECODER_INIT_FAILED` |
| TrueHD 5.1 kopiert | `mlpa`, 6 | `mlpa` (Media3 kennt keinen TrueHD-Code) | Tonspur fehlt | stumm; mit `mp4a.40.2` **Abbruch** |
| FLAC 2.0 kopiert | `fLaC`, 2 | `fLaC` | `audio/flac` | spielt |
| FLAC 5.1 kopiert | `fLaC`, 6 | `fLaC` | `audio/flac` 6 ch | «Buffer too small (32768 < 33207)», Ton bricht ab; mit App-Korrektur (unten) sauber |
| Opus 5.1 kopiert | `Opus`, 6 | `Opus` | `audio/opus` 6 ch | spielt, `c2.android.opus.decoder` |

Erstes Bild 127–288 ms, alle Proben bei 3 s ohne Fehler ausser den markierten. ffmpeg dekodiert
jede geschriebene Liste ohne Fehler (`pruefen.sh`). Die Codec-Angaben, die Media3 kennt, stehen in
`MimeTypes` 1.11.1: `ac-3`, `ec-3`, `ec+3`, `ac-4`, `dtsc`/`dtse`/`dtsh`/`dtsl`/`dtsx`, `opus`,
`flac`, `mp4a.*`; kein Eintrag für TrueHD.

**Kein Decoder:** Media3 wählt eine Tonspur ohne Decoder und ohne Durchreichen ab und spielt das
Bild stumm weiter, ohne Fehler und ohne Ausweichen. Die App muss das selbst erkennen (Tonspur da,
keine gewählt) und neu starten, ohne diesen Codec zu melden.

**FLAC über 2 Kanäle:** Media3 1.11.1 lässt den Eingangspuffer des FLAC-Decoders auf 32 KiB; 5.1-
Rahmen sind grösser. In HLS bricht der Ton ab, in MKV endet die Wiedergabe mit
`ERROR_CODE_FAILED_RUNTIME_CHECK`. Ein eigener `MediaCodecAudioRenderer` mit
`getCodecMaxInputSize` ≥ 111 616 Byte für FLAC (`TvRenderersFactory`) behebt beides: 8 s ohne
Fehler, 96 Puffer hinein, 89 heraus (`MeasureTest#flacFix`).

**Entscheid.**

- **Formate der HLS-Ausgabe:** AAC (neu oder kopiert), AC-3 und E-AC-3 (kopiert und neu), DTS,
  FLAC und Opus (nur kopiert, nur wenn gemeldet). **TrueHD nie in HLS**: ffmpeg nennt es
  experimentell, Media3 erkennt es nicht; TrueHD wird neu kodiert.
- **Codec-Angabe** (`audioCodecString`): `aac` → `mp4a.40.2` (HE-AAC wie heute), `ac3` → `ac-3`,
  `eac3` → `ec-3`, `dts` → `mp4a.a9` (RFC 6381 zum Eintrag `mp4a` mit OTI 0xA9, den ffmpeg
  schreibt; Media3 liest `dtsc` gleich), `flac` → `fLaC`, `opus` → `Opus`. Kein stiller Rückfall
  auf `mp4a.40.2`: Ein Codec ohne Angabe wird nicht kopiert, sondern neu kodiert. Eine falsche
  Angabe ist schlimmer als eine fehlende (Abbruch statt stumm).
- **Pipeline:** `+delay_moov+frag_keyframe` in `movflags`, wenn der Ton AC-3 oder E-AC-3 ist;
  Segmentierer wie oben. Behebt den heutigen Fehler auch für den Browser.
- **Ziel beim Neu-Kodieren** mit gemeldeten 6 Kanälen: **AC-3, wenn das Gerät `ac3` meldet, sonst
  AAC 5.1** (Vorschlag des Papiers bestätigt; ob beide auf den drei Geräten sauber laufen, kann der
  Emulator nicht zeigen). Mit 8 gemeldeten Kanälen und einer Quelle über 6 Kanälen: AAC 7.1
  (AC-3 kann höchstens 5.1). Nie mehr Kanäle als die Quelle.
- **Bitraten:** AC-3 5.1 **640 kbit/s** (Höchstwert von AC-3, geht über ARC; die Quelle ist beim
  Neu-Kodieren meist verlustfrei oder DTS, die 190 kbit/s mehr als 448 fallen neben dem Video nicht
  ins Gewicht). AAC 5.1 **384 kbit/s**, AAC 7.1 **512 kbit/s**: 64 kbit/s je Kanal wie Stereo
  heute (160 für 2). Die heutigen 160 kbit/s bleiben für Stereo.
- **Bandbreite:** echte Bitrate der kopierten Spur (ffprobe `bit_rate` der Spur), sonst die
  Obergrenze des Formats (AC-3 640, E-AC-3 1024, DTS 1536, Opus 510, FLAC nach Kanälen).

**Für Stufe 1:** `transcode/plan.go` (`audioCodecString`, Kopierregel), `transcode/pipeline.go`
(`movflags`, Kanäle, Bitraten, `strict` entfällt mit «TrueHD nie»), `transcode/segmenter.go`
(Init ohne Daten, mehrere Fragmente je Datei), `playback/decide.go` (Tonkopie beim Transcode für
gemeldetes AC-3/E-AC-3), `playback/quality.go` (Bandbreite). Ein Test mit AC-3-Quelle gegen
`Manager.Start` deckt den heutigen Fehler ab. **Für Stufe 3:** `TvRenderersFactory`, Erkennen von
«Ton stumm».

## 2. Was das Gerät dem Server meldet

**Code** (`Capabilities.kt` im Notizordner):

- `video`: aus `MediaCodecList(ALL_CODECS)`, Decoder je MIME-Typ; `hevc` nur mit Profil
  `HEVCProfileMain10` (wie K27). `av1`, `vp9`, `mpeg2video` nach Decoder.
- `audio`: Decoder (`aac`, `mp3`, `opus`, `flac`, `vorbis`, `ac3`, `eac3`, `dts`, `truehd`) **oder**
  Durchreichen: Media3 `AudioCapabilities.getCapabilities(context, AudioAttributes.DEFAULT, null)`
  und `supportsEncoding(AudioFormat.ENCODING_AC3 | E_AC3 | DTS | DOLBY_TRUEHD)`.
- `maxAudioChannels`: 8 bei Durchreichen von TrueHD (oder E-AC-3-JOC), 6 bei Durchreichen von AC-3,
  E-AC-3 oder DTS, sonst die PCM-Kanäle des Ausgangs (`AudioDeviceInfo.channelCounts` des
  HDMI-Geräts, `AudioCapabilities.maxChannelCount`), gerundet auf 2, 6 oder 8.
- `hdr`, `maxWidth`/`maxHeight`: siehe Punkt 5. `containers`: `mp4`, `mkv`, `webm`, `ts`, `m2ts`,
  `mov` (Namen aus `library/probe.go`). `hls: mse`, `type: tv`, Name «Modell · Android TV 14».

**Durchreichen in Media3:** nichts einzuschalten. `ExoPlayer.Builder(context)` baut eine
`DefaultAudioSink`, die die `AudioCapabilities` des Geräts liest (ab API 33 über
`AudioManager.getDirectProfilesForAttributes`, davor `AudioTrack.isDirectPlaybackSupported` und das
HDMI-Signal) und bei Änderungen des Ausgangs neu liest. Meldet das Gerät eine Kodierung, nimmt der
`MediaCodecAudioRenderer` die Spur ohne Decoder an. **Woran die App es erkennt:**
`AnalyticsListener.onAudioTrackInitialized(AudioTrackConfig)`: `encoding` 5 (AC-3), 6 (E-AC-3),
7 (DTS), 14 (TrueHD) heisst durchgereicht, 2 (PCM 16 bit) heisst dekodiert; dazu die Kanalmaske.
Das zeigt die Info-Anzeige des Players.

**Emulator meldet:** Decoder AAC (8 ch), FLAC (8), Opus (8), Vorbis, MP3, H.264, HEVC nur Main
(Profile 1, 4), VP9, AV1, MPEG-2; kein AC-3, E-AC-3, DTS, TrueHD. Ausgänge: eingebauter
Lautsprecher und Telefonie, keine HDMI-Geräte, `channelCounts` leer. Media3:
`AudioCapabilities[maxChannelCount=2, audioProfiles={2=…PCM, maxChannelCount=2}]`,
`isDirectPlaybackSupported(AC-3 5.1)` = false. `settings put global encoded_surround_output 2`
(«immer») und `3` mit Formaten von Hand ändern daran nichts (Ton-HAL des Emulators).
Ergebnis `auto`: `video=[h264, av1, vp9, mpeg2video]`, `audio=[aac, mp3, opus, flac, vorbis]`,
`maxAudioChannels=2`, `hdr=false`, `1920×1080`.

**Datenmodell Schalter «Ton»** (Einstellungen der Wiedergabe, je Gerät gespeichert):

```kotlin
sealed interface AudioSetting {
    data object Auto : AudioSetting                                    // erkennen wie oben
    data object Stereo : AudioSetting                                  // nur Dekodiertes, ohne Dolby/DTS, 2 Kanäle
    data class Manual(val formats: Set<String>, val channels: Int) : AudioSetting  // ffprobe-Namen, 2/6/8
}
```

«Von Hand» bietet die Formate `aac`, `ac3`, `eac3`, `dts`, `truehd`, `flac`, `opus` zum Ankreuzen
und die Kanalzahl 2/6/8; vorbelegt mit dem, was `Auto` gefunden hat.

**Entscheid.** Code wie oben; drei Vertragsfelder in `PlaybackCapabilities` (alle zusätzlich):
`maxAudioChannels` (Ganzzahl 2, 6 oder 8), `selectsTracks` (Ja/Nein), `subtitles` (Liste von
ffprobe-Namen, Muster wie `audio`). Durchreichen und Dekodieren meldet die App gemeinsam in `audio`;
der Server muss sie nicht unterscheiden. **Für Stufe 1** (Vertrag), **Stufe 3** (App, Schalter).

## 3. Untertitel

**Gemessen** mit `subs.mkv` (H.264, AAC, vier Untertitelspuren: SRT, ASS mit zwei Stilen und
Inline-Befehlen, PGS aus `pgs_sup.py`, DVD aus ffmpegs `dvdsub`), direkt abgespielt,
`MeasureTest#subtitles` (Cues je Spur bei 2 s und 7 s) und Bildschirmfotos mit `PlayerView`.

| Spur | Media3 | Was erscheint |
|---|---|---|
| SRT | `application/x-subrip` | Text, `<i>` und `<b>` als `StyleSpan` |
| ASS | `text/x-ssa` | Stilfarbe (gelb), Stil-Fett, Stilgrösse (48/720, 40/720), Ausrichtung (`\an7` oben links, Stil oben/unten), `\pos(200,300)` an richtiger Stelle. **Weg:** Inline-Farbe `\c`, `\i1`, `\b1`, `\fs70`; Kontur wird zum schwarzen Kasten des `SubtitleView` |
| PGS | `application/pgs` | Bitmap 415 × 50 an der Stelle aus der Datei (unten, später oben), Kontur erhalten |
| DVD | `application/vobsub` | Bitmap an derselben Stelle |

**Entscheid.** Die App meldet `subtitles: [subrip, ass, webvtt, hdmv_pgs_subtitle, dvd_subtitle]`.
**PGS: ja** (der Server brennt sie bei Direktwiedergabe nicht mehr ein). **ASS: ja**: Media3
behält Position, Ausrichtung und Stile, die WebVTT-Auslieferung verliert alles davon. Bei
Direktwiedergabe liefert der Server für diese Spuren nichts, die App wählt die Spur in der Datei.
Bei HLS bleibt alles wie heute (Text als WebVTT, Bild eingebrannt). Für die Kontur statt Kasten
setzt die App `SubtitleView.setStyle(CaptionStyleCompat(…, EDGE_TYPE_OUTLINE, …))`.
**Für Stufe 1** (`decide.go`: Einbrennen nur, wenn das Format nicht in `subtitles` steht oder HLS)
und **Stufe 3**.

## 4. «Wählt Spuren selbst»

**Gemessen** mit `tracks.mkv` (60 s, Ton `deu` 440 Hz und `eng` 880 Hz mit Standard-Flag,
SRT `deu` und `eng` mit Standard-Flag), `MeasureTest#trackSwitch`: bei laufender Wiedergabe
abwechselnd Ton und Untertitel über `TrackSelectionOverride`, drei Runden.

- Ton: gewählt nach 20–21 ms, ein Puffern von 0–2 ms, neue `AudioTrack`, Position läuft weiter
  (5015 → 6470 ms in 1,5 s, keine Rücksprünge). Untertitel: 20 ms, ohne Puffern. 6 von 6 Wechseln
  sauber, kein Fehler.
- Ohne Vorgabe wählt Media3 die Spuren mit Standard-Flag (hier `eng`).
- Was Media3 je Spur zeigt: `Format.id` = Spurnummer der MKV (`2`, `3` für Ton, `4`, `5` für
  Untertitel; bei ffmpeg- und mkvmerge-Dateien `index + 1`), `language` normalisiert (`deu` → `de`,
  `eng` → `en`), `label` = Titel der Spur, `selectionFlags` 1 = Standard. Gruppen in der Reihenfolge
  der Datei.

**Entscheid.** `selectsTracks: true` bei Direktwiedergabe. **Zuordnung** `getItemTracks` →
Media3: je Art (Ton, Untertitel) die n-te eingebettete Spur des Servers nach `index` = die n-te
Gruppe dieser Art in Media3. Prüfung, bevor die App wählt: Sprache (ISO 639-2 auf -1 abgebildet)
und Titel (`title` = `label`); passt etwas nicht, nimmt die App den Weg über den Server (neuer
Start mit `audioStream`). `Format.id` nur als zusätzlicher Hinweis, weil Spurnummern in MKV frei
sind. Sidecar-Untertitel (`index` −1) gibt die App als `SubtitleConfiguration` dazu. Beim Start
setzt die App die Wahl des Servers als Vorgabe, sonst gewinnt das Standard-Flag der Datei.
**Für Stufe 1** (`decide.go`: andere Tonspur erzwingt bei `selectsTracks` und Direktwiedergabe
keinen Remux) und **Stufe 3**.

## 5. Direktwiedergabe und HDR

**Gemessen** (`MeasureTest#directPlay`, MKV direkt):

| Datei | Ergebnis im Emulator |
|---|---|
| H.264 + AAC 5.1 / Opus 5.1 | spielt, Ton 6 Kanäle |
| H.264 + AC-3 / E-AC-3 / DTS / TrueHD | Bild spielt, Ton `UNSUPPORTED_SUBTYPE`, stumm |
| H.264 + FLAC 5.1 | Abbruch nach 2,8 s (`ERROR_CODE_FAILED_RUNTIME_CHECK`); mit `TvRenderersFactory` sauber |
| HEVC Main 1080p | spielt (`c2.goldfish.hevc.decoder`) |
| HEVC Main 10 | `EXCEEDS_CAPABILITIES`, `ERROR_CODE_DECODING_FAILED` |
| HEVC Main 10 mit HDR10-Kennung (PQ, BT.2020, Mastering-Daten) | Media3 liest `ColorInfo(BT2020, ST2084 PQ, 10bit)`; Decoder scheitert wie oben |

Anzeige: ein Modus 1920 × 1080 bei 60 Hz, `supportedHdrTypes=[]`, `isHdr=false`. Dolby Vision
lässt sich im Emulator nicht prüfen.

**Code-Weg HDR:** ab API 34 `display.mode.supportedHdrTypes`, darunter
`display.hdrCapabilities.supportedHdrTypes` (`HDR_TYPE_HDR10`, `HLG`, `DOLBY_VISION`,
`HDR10_PLUS`). `maxWidth`/`maxHeight` aus dem grössten Modus in `display.supportedModes`, nicht
aus dem aktuellen (Fernseher zeigen die Oberfläche oft in 1080p auf einer 4K-Anzeige); im Bau
zusätzlich begrenzt durch `VideoCapabilities.isSizeSupported` des HEVC- oder H.264-Decoders.

**Entscheid.** `hdr: true` nur, wenn die Anzeige HDR10 oder HLG meldet **und** ein HEVC-Decoder
mit Main 10 da ist. Dolby Vision allein zählt nicht. F-8 bleibt bei der Empfehlung: kein Grund
gefunden, die drei Verfahren schon in der ersten Ausgabe getrennt zu melden. Nach dem Quellcode von
Media3 fällt Dolby Vision Profil 8 ohne DV-Decoder auf den HEVC-Decoder zurück (Grundschicht
HDR10); das ist im Emulator nicht prüfbar und steht in der Nachmessung. **Für Stufe 3.**

## 6. Compose for TV

**Gebaut:** `TvHome` mit `NavigationDrawer` (fünf Einträge), `Carousel` (5 Bilder 1920 × 1080),
`LazyColumn` mit sechs `LazyRow` zu je 20 `Card` (Coil-Bilder 342 × 513 vom Testserver),
`Card`-Skalierung 1,1 im Fokus. Steuerung mit `adb shell input keyevent` (`fokus-lauf.sh`), Fokus
aus dem Log. Bildrate mit `dumpsys gfxinfo`, gemessen am **Release-Bau** (R8): der Debug-Bau ist
etwa doppelt so langsam (Median 46 ms, 33 % ruckelnde Bilder) und taugt nicht als Anhaltspunkt.

| Lauf (Release, Software-Grafik des Emulators) | Bilder | ruckelnd | Median | 90 % | 99 % |
|---|---:|---:|---:|---:|---:|
| 6 Reihen, je 19 × rechts und 19 × links, eine Taste je Aufruf (234 Tasten in 6,7–8,2 s) | 370–488 | 0,4–5,4 % | 22–29 ms | 27–40 ms | 34–73 ms |
| Taste gehalten (19 × rechts in einem Aufruf je Reihe) | 40–189 | 0–6,8 % | 18–30 ms | 25–38 ms | 34–65 ms |

Spanne über drei Läufe. Kaltstart der Aktivität 351–432 ms (Release), 1001–1199 ms (Debug).
Speicher 80 MB PSS. Das sagt nichts über schwache Geräte: der Emulator rechnet die Grafik auf
der CPU des Rechners.

**Was trägt (Vorlage für die Fokus-Regeln der TV-Seiten):**

- **Je Reihe `Modifier.focusRestorer(first)`** auf der `LazyRow`, `first` = `FocusRequester` der
  ersten Karte. Wiederkehr in eine Reihe landet auf der zuletzt fokussierten Karte (r0 c5 → r1 →
  zurück r0 c5), der erste Eintritt auf Karte 0. Ohne `fallback` landet der erste Eintritt auf der
  geometrisch nächsten Karte (aus dem Spotlight auf c2, von c7 in der Reihe darunter auf c1).
- **Leiste:** `NavigationDrawer` öffnet sich, sobald sie den Fokus bekommt, und schliesst sich beim
  Verlassen (`drawer Open`/`Closed`). Die Spalte der Einträge braucht `focusRestorer(gewählt)`
  **und** `focusGroup()`; ohne `focusGroup()` landet der Fokus von der Karte c0 aus auf dem
  geometrisch nächsten Eintrag (dem letzten). Rechts aus der Leiste kommt der Fokus an die alte
  Stelle zurück.
- **Startfokus** per `FocusRequester` erst nach dem ersten Bild:
  `LaunchedEffect(Unit) { withFrameNanos { }; start.requestFocus() }`. Direkt in
  `LaunchedEffect` setzt der JVM-Test gar keinen Fokus. Am Gerät bekommt trotzdem zuerst der erste
  Leisten-Eintrag kurz den Fokus (Fokus des Systems beim Öffnen des Fensters), dann das Spotlight.
- **Zurück:** im Inhalt `BackHandler` → Fokus in die Leiste (öffnet sie); in der offenen Leiste
  `BackHandler` → `finish()`. Zweimal Zurück führt zum Startbildschirm (TV-DB erfüllt).

**Fallen:**

- `androidx.tv.material3.Text` ausserhalb einer `Surface` ist schwarz auf dunklem Grund
  (`LocalContentColor`): jede Seite braucht eine `Surface` oder einen gesetzten `LocalContentColor`.
- `onFocusChanged` sieht Werte aus der Komposition, nicht den aktuellen Zustand der Leiste; Zustand
  über `snapshotFlow { drawerState.currentValue }` lesen.
- `NavigationDrawer` schiebt den Inhalt um die Breite der offenen Leiste; für eine überlagernde
  Leiste `ModalNavigationDrawer`.
- `Carousel` ist ein Fokus-Container: `onFocusChanged` meldet dort `hasFocus`, nicht `isFocused`.
- Auf dem Fernseher gilt 960 × 540 dp (320 dpi); Masse für Karten und Ränder darauf auslegen.

**Entscheid.** Compose for TV mit `tv-material` 1.1.0 trägt; die Muster oben werden die
Fokus-Regeln der TV-Seiten (Stufe 2b). Das Tempo auf dem schwächsten Gerät bleibt offen
(Nachmessung), Baseline-Profile (TV-BP, Stufe 2 der Qualitätsliste) gehören in den Bau.

## 7. Oberflächentests am Fernseher

**Gemessen:** `TvFocusTest` (JVM, Robolectric 4.18.1, `@Config(sdk = [34], qualifiers =
"w960dp-h540dp-land-television-xhdpi")`, `@GraphicsMode(NATIVE)`,
`createAndroidComposeRule<ComponentActivity>()`) über dieselbe `TvHome`:
Startfokus auf dem Spotlight, ↓ → Karte 0-0, Pixel der fokussierten Karte unterscheiden sich von
der unfokussierten (`captureToImage`), → × 3, ↓ → 1-0 (Rückfallwert), ↑ → 0-3 (wiederhergestellt),
↓ → 1-1, ← ← → Leiste auf «Start», → zurück auf 1-0, Zurück → Leiste, Zurück → verlassen.
**5 von 5 Läufen grün, 4,1–4,9 s je Lauf.**

Was dafür nötig war:

- Tasten mit `onRoot().performKeyInput { pressKey(Key.DirectionDown) }`, danach `waitForIdle()`.
- Zurück erreicht `BackHandler` über `pressKey(Key.Back)` **nicht** (geht nur in den
  Fokusbaum); der Test ruft `activity.onBackPressedDispatcher.onBackPressed()` auf dem UI-Faden.
- `setInTouchMode(false)` vor `setContent` (Fernseher kennen keinen Touch-Modus).
- Fokus sichtbar: Pixelvergleich der Karte vor und nach dem Fokus; `assertIsFocused` allein prüft
  nur die Semantik.

**Entscheid.** Steuerkreuz-Tests der TV-Seiten (Regel 8 für die App) laufen als **JVM-Tests mit
Robolectric** im normalen Lauf von `testDebugUnitTest`; der Emulator ist dafür nicht nötig. Der
Emulator bleibt für Wiedergabe (Media3, Decoder) und einen Rauchtest je Ausgabe. **Für Stufe 2a**
(Testgerüst) und **2b** (ein Test je Seite).

## 8. Kleinste Android-Version (F-13) und Manifest

**Vorgaben** (TV-Qualitätsliste, gelesen am 21.11.2026): TV-PS «minimum Android SDK version of 31
or lower»; TV-G6 32 und 64 Bit sowie 16-KB-Speicherseiten ab 1. August 2026; TV-G1 App-Bundle.
Ziel-API für TV mindestens 34 (Festlegung `targetSdk` 34).

**Zielgeräte** (Stand der Kenntnis, auf den Geräten zu bestätigen): Nvidia Shield mit Android TV 11
(API 30), Chromecast mit Google TV Android 12 bis 14, Google TV Streamer Android 14, Fernseher mit
Google TV ab Android 10 (API 29); ältere Fernseher und Boxen mit Android TV 9 (API 28) sind noch
verbreitet.

**Entscheid: `minSdk` 28 bestätigt.** Liegt unter der Grenze 31, deckt alle drei Gerätearten und
Android-TV-9-Geräte, und ist gleich wie die Handy-App, so dass gemeinsame Module (`core/*`) ohne
zweite Untergrenze auskommen. Tiefer lohnt nicht: Android TV 8 ist kaum noch im Einsatz, und K27
hat API 28 schon geprüft (Tink, Media3, Robolectric). Die Wegwerf-App baut mit `minSdk` 28,
`targetSdk` 34 und `compileSdk` 37.2 (wie K27). Die einzigen nativen Bibliotheken der App
(`libandroidx.graphics.path.so`, vier ABIs) bestehen `zipalign -c -P 16`; ohne FFmpeg (F-12) bleibt
das so.

**Checkliste Manifest und Store für Stufe 2a:**

| Punkt | Vorgabe | in der Wegwerf-App geprüft |
|---|---|---|
| TV-ML | `<intent-filter>` mit `MAIN` und `LEANBACK_LAUNCHER` auf der Start-Aktivität | im Manifest; Eintrag im Startbildschirm nicht angesehen |
| TV-MT | `<uses-feature android:name="android.hardware.touchscreen" android:required="false"/>` und die übrigen Geräte aus «Declare hardware requirements for TV» (Mikrofon, Kamera, GPS …) nur mit `required="false"` | Touchscreen ja |
| Leanback | `<uses-feature android:name="android.software.leanback" android:required="false"/>` (eine App für Handy und TV, E-20) | ja |
| TV-LB, TV-BN | `android:banner` 320 × 180 px (xhdpi) mit App-Namen, dazu Symbol ≥ 160 × 160 px | Banner ja, Symbol nein |
| TV-LO | Querformat ohne Balken | `screenOrientation="landscape"` |
| TV-DB | Zurück führt bis zum Startbildschirm | ja (Punkt 6) |
| TV-DP, TV-DM | alles mit dem Steuerkreuz, nichts hängt an einer Menü-Taste | Startseite ja |
| TV-PC, TV-PP | Mitte = Pause/Weiter, Links/Rechts spulen, Taste Wiedergabe/Pause wirkt (`MediaSession`) | Stufe 3 |
| TV-BU, TV-BA | kein Bildschirmschoner während Video (`FLAG_KEEP_SCREEN_ON`), bei reinem Ton schon | Spieler-Modus setzt das Flag |
| TV-NP | Video pausiert beim Verlassen der App | Stufe 3 |
| TV-G5 | Test-Zugangsdaten für Googles Prüfer | Stufe 6 |
| TV-G6 | 64 Bit, 16 KB | ja (siehe oben) |
| Cleartext | `usesCleartextTraffic` oder Netzwerk-Sicherheitskonfiguration nach F-6 | Stufe 2a |

## Nur im Emulator geprüft

- Media3-Verhalten aller Tonformate: Erkennung, Kanalzahl, stummes Abwählen ohne Decoder,
  Abbruch bei falscher Codec-Angabe. Echte Dolby-, DTS- und TrueHD-Wiedergabe und jedes
  Durchreichen fehlen (keine Decoder, keine HDMI-Ausgabe, Ton-HAL nur PCM Stereo).
- AAC 5.1/7.1 nur als PCM-Kanalmaske an `AudioTrack`; der Emulator hat keine Tonausgabe.
- HEVC Main 10, HDR und Dolby Vision gar nicht (Decoder scheitert, Anzeige ohne HDR).
- Untertitel, Spurwechsel und Fokus mit Software-Grafik; Bildrate nur als Anhaltspunkt.
- ffmpeg-Proben mit ffmpeg 7.1.4 (Image) und 6.1.1 (Rechner), synthetische Signale.

## Auf den drei Geräten nachzumessen

Für jedes Gerät (Chromecast oder Google TV Streamer, Nvidia Shield, Fernseher mit Google TV),
einmal mit Ton direkt am Fernseher und einmal über eine Tonanlage (HDMI oder ARC/eARC). Alles
liegt im Notizordner `k26-stufe0/` (`LIESMICH.md`).

1. **Vorbereiten:** Entwickleroptionen und USB- oder Netzwerk-Debugging am Gerät einschalten,
   `adb connect <Gerät>:5555`, mit `adb devices` prüfen. `adb shell getprop ro.build.version.sdk`
   und `ro.product.model` notieren (bestätigt Punkt 8).
2. **Proben bauen und bereitstellen:** `proben.sh <Ordner>` auf dem Rechner, dann
   `python3 -m http.server 8697 --bind 0.0.0.0 --directory <Ordner>` (nur im Heimnetz, danach
   beenden). Im Test die Adresse mit `-e host http://<Rechner>:8697` übergeben.
3. **Wegwerf-App bauen und installieren:** nach `LIESMICH.md`, `itest.sh` mit
   `SERIAL=<Gerät>`.
4. **Fähigkeiten:** `itest.sh MeasureTest#capabilities`. Ablesen in `out/…log`: Decoder (AC-3,
   E-AC-3, DTS, HEVC Main 10, `hw=true`), Ausgänge (`device type=9` HDMI, `channelCounts`,
   `encodings`), Zeile `media3 AudioCapabilities` (Kodierungen 5/6/7/14 = Durchreichen), Zeile
   `auto` (was die App melden würde), Anzeige (`supportedHdrTypes`, Modi bis 3840 × 2160).
   Danach in den Ton-Einstellungen des Geräts «Surround: immer» und «von Hand» setzen und
   wiederholen.
5. **Ton in HLS:** `itest.sh MeasureTest#audioHls` und hinhören. Je Probe ablesen: Zeile
   `audioTrack encoding=…` (2 = dekodiert, 5/6/7 = durchgereicht), `selectedAudio`, Fehler. Klärt
   **AC-3 gegen AAC 5.1** (Punkt 1): beide Proben müssen ohne Fehler und mit Ton an der Anlage
   laufen; die Anlage muss bei `t-ac3-*` «Dolby Digital» zeigen. Gleiches für `t-eac3-*` und
   `r-dts`.
6. **Direktwiedergabe:** `itest.sh MeasureTest#directPlay` mit einer echten HEVC-Main-10-Datei
   und einer HDR10-Datei ergänzt (eigene Dateien, nicht ins Repo): `first=` (erstes Bild),
   Fehler, Decoder. Prüft `hdr: true` (Punkt 5).
7. **Dolby Vision auf einem Gerät ohne DV** (F-8): eine Datei mit Dolby Vision Profil 8.1 direkt
   abspielen; ablesen, ob `videoDecoder` der HEVC-Decoder ist und das Bild in HDR10 kommt. Zeigt das
   ein Problem, wird F-8 vor Stufe 3 neu gestellt.
8. **Untertitel und Spuren:** `itest.sh MeasureTest#subtitles` und `MeasureTest#trackSwitch`;
   zusätzlich `adb shell am start -n com.slouchcat.spike.tv/.MainActivity --es url <…>/mkv/subs.mkv
   --ei text 2` (PGS) und `--ei text 1` (ASS) und am Bildschirm ansehen.
9. **Tempo:** `fokus-lauf.sh <Gerät> http://<Rechner>:8697/img` mit dem **Release-Bau**
   (`assembleRelease`). Ablesen: «Janky frames», Perzentile; Ziel auf dem schwächsten Gerät unter
   5 % ruckelnd und 90 % unter 16,7 ms (60 Hz). Darüber: Baseline-Profil und Bildgrössen prüfen,
   bevor Stufe 2b weitergeht.
10. **Ergebnis:** je Gerät eine Zeile in diesem Bericht ergänzen (Gerät, Android, Ton direkt und an
    der Anlage, AC-3 oder AAC 5.1, HDR, Bildrate).

## Stufe 1: Ton in der Pipeline (21. November 2026)

Gebaut auf `claude/k26-stufe1-a` (ADR 0041). Rechner wie oben (28 Kerne), ffmpeg 6.1.1 des
Rechners, Software-Backend. Quelle: Testdatei «Slouchcat Test Multi Audio» aus
`testdata/generate-media.sh` (H.264 1080p, 20 s, AAC 2.0 als Standard, AC-3 5.1 384 kbit/s).
Gemessen ist die Zeit vom Start der Sitzung (`Manager.Start`) bis init und Segment 0 gelesen sind,
und bis Segment 1, je 7 Läufe, Median (Wegwerf-Test, nicht eingecheckt).

| Ausgabe | Segment 0 Median (min–max) | Segment 1 Median |
|---|---|---|
| Remux, AAC kopiert (bisheriger Weg) | 42 ms (41–52) | 43 ms |
| Remux, AC-3 kopiert (neu; vorher Abbruch mit 234) | 46 ms (43–52) | 47 ms |
| Transcode 1080p, AAC Stereo neu (bisheriger Weg) | 504 ms (404–554) | 703 ms |
| Transcode 1080p, AC-3 kopiert (neu) | 603 ms (552–704) | 604 ms |
| Transcode 1080p, AC-3 5.1 neu kodiert (neu) | 604 ms (603–704) | 606 ms |

Ein «vorher» gibt es für AC-3 nicht: Der Remux brach ab. Mit `+delay_moov+frag_keyframe` kommt
Segment 0 eines Transcodes etwa 100 ms später als beim bisherigen Weg, weil sein Fragment erst mit
dem ersten Bild von Segment 1 geschrieben wird (Abfrage alle 50 ms); Segment 1 ist dafür sofort
danach da. Beim Remux ist der Unterschied unter der Messgenauigkeit.

**Prüfung der Segmente** (`server/internal/transcode/hls_audio_test.go`, echter ffmpeg, Clip mit
Keyframes bei 0, 2, 5, 7, 9, 13, 14, 18 s, also ein Segment mit zwei und eines mit einem Keyframe):
Remux mit kopiertem AC-3 und E-AC-3 5.1 vom Anfang und nach Neustarts (Segment 4, zurück zu 3, 1,
2); Transcode mit kopiertem E-AC-3, AC-3 5.1 aus AAC 5.1, AAC 5.1 aus AC-3, AAC 7.1 aus FLAC 7.1,
je vom Anfang (ein Lauf) und mit vier Läufen. `ffprobe` an init + Segmenten: Codec und Kanalzahl
wie geplant, jedes Segment beginnt mit einem Keyframe am geplanten Start (±2 ms), 480 Bilder
dekodiert, Tonspur ohne Lücke über 1 ms innerhalb eines Laufs und über die Nähte des Remux. An den
Nähten eines Transcode-Neustarts fehlen wie bisher bis drei Tonrahmen (AAC Stereo 53 ms wie vor
dieser Stufe, E-AC-3 kopiert 96 ms); das ist nicht Teil dieser Stufe. Ohne den neuen Schalter
scheitern dieselben Tests an «exit status 234» (Gegenprobe).

**Prüfprogramm** (`task apicheck`, Zeile «Fernseher mit Mehrkanalton (K26)»): Profil `type: tv`,
`hls: mse`, `audio` aac/ac3/eac3, `maxAudioChannels: 6`, `selectsTracks: true`,
`subtitles: [pgs, ass]`. MKV mit der AC-3-Spur → `direct`; nur MP4 → `remux`, CODECS
`avc1.640028,ac-3`, init 1242 Bytes, Segmente 0 und 1 mit 1 170 590 und 1 194 867 Bytes;
Qualität 480p → `transcode`, CODECS `avc1.640029,ac-3`, Segmente 1 056 457 und 1 073 138 Bytes.

## Stufe 3: Videoplayer der TV-App (22. November 2026)

Gebaut auf `claude/k26-stufe3-a`: Player-Seite auf `:core:player`, was der Fernseher meldet, Schalter «Ton»,
Einstellungen der Wiedergabe. Gemessen nur im **Emulator** `sc_tv` (Google TV, API 34, x86, Software-Grafik,
ohne Tonausgabe, ohne Dolby-Decoder, ohne HDR); was nur ein Gerät zeigen kann, steht unten als Abnahmeliste.

**Was der Emulator meldet** (Fähigkeitsprofil des Starts, `type: tv`, `hls: mse`): `video` h264, av1, vp9,
mpeg2video (HEVC nur Main, darum nicht gemeldet), `audio` aac, mp3, opus, flac, vorbis, `maxAudioChannels` 2,
`selectsTracks: true`, `subtitles` subrip, ass, webvtt, hdmv_pgs_subtitle, dvd_subtitle, `hdr: false`,
1920 × 1080, Name aus dem Einrichten («Modell · Android TV 14»).

**Zeit bis zum ersten Bild** (Emulator, `TvFlowTest#c_player`, Testclips aus `scripts/android-test-video.sh`,
Server auf demselben Rechner, gemessen vom Start des Controllers bis `onRenderedFirstFrame`, Zeile
`first picture` im Log):

| Wiedergabe | Abspielart | erstes Bild |
|---|---|---|
| Film «Two Voices» (H.264 720p, AAC, MKV), Start | Direktwiedergabe | 497 ms |
| derselbe Film nach Home und zurück (pausiert) | Direktwiedergabe | 417 ms |
| derselbe Film, Qualität 480p | HLS, Transcode | 646 ms |
| derselbe Film, Fortsetzen von der Detailseite | Direktwiedergabe | 517 ms |
| Folge 1 «Clip Show» | Direktwiedergabe | 247 ms |
| Folge 2 nach dem Countdown | Direktwiedergabe | 290 ms |

Ein Wert je Lauf (frühere Läufe: Start 724 und 753 ms, HLS 587 und 960 ms, Fortsetzen 531 und 583 ms); der Emulator rechnet die
Grafik auf der CPU, das sagt über Geräte wenig. Der Transcode läuft
im Software-Encoder des Rechners.

**Entscheide des Baus:**

- **Spulen:** Links/Rechts in Schritten 10, 10, 20, 30, 30, 60, 60, 120 s (Mockup `brPlSeek`); eine gehaltene
  Taste höchstens alle 150 ms einen Schritt; gesprungen wird einmal, 800 ms nach der letzten Taste und erst
  nach dem Loslassen (Media3 lädt sonst jeden Zwischenschritt). Vorschaubild aus Trickplay, wenn der Server
  Bögen hat.
- **Tonausgang wechselt:** Die App hört auf `AudioDeviceCallback` und `ACTION_HDMI_AUDIO_PLUG`, liest die Fakten
  nach 1 s Ruhe neu und startet nur neu, wenn sich das Gemeldete ändert (bei «Immer Stereo» nie). Ein Kopfhörer
  (kabelgebunden, Bluetooth, USB, BLE) nimmt den Ton vom HDMI-Ausgang: dann Stereo und kein Durchreichen.
- **HDR:** Die App meldet `hdr` nach Stufe 0 Punkt 5; den Anzeigemodus schaltet das System für einen
  HDR-Strom auf der `SurfaceView` selbst. Kein Wechsel der Bildfrequenz (F-8, Empfehlung).
- **Qualität:** Der Fernseher fragt ohne Grenze («Automatisch»), bis man eine wählt; die Wahl gilt für jedes
  Netz. «Original» als Vorgabe fehlt (das Tonmodell `QualityChoice` hat sie nicht), im Player gibt es sie.
- **Kein Tempo am Fernseher:** Papier 5.1 und das Mockup nennen keine Geschwindigkeit; Knopf und Menü sind
  nach dem Review entfallen.
- **«Formate von Hand»** bietet die vier Durchreich-Formate (Dolby Digital, Dolby Digital Plus, DTS, Dolby
  TrueHD) und die Kanalzahl; was der Fernseher selbst dekodiert (AAC, FLAC, Opus), bleibt immer gemeldet.
  Vorbelegt mit dem, was «Automatisch» meldet. Mit Kopfhörer gilt «von Hand» weiter (bewusste Wahl des
  Benutzers; Review L-7).
- **Info-Panel** bleibt stehen, wenn die Bedienebene ausblendet (Mockup: `.br-info` liegt neben der
  Bedienebene); Zurück schliesst es zuerst.
- **Vor-/Rückspultasten** der Fernbedienung: jeder Druck ein Schritt, Halten beschleunigt nicht (die
  Tasten kommen ohne Wiederholung, Review L-9); auf dem Steuerkreuz beschleunigt Halten.
- **Untertitel:** Kontur statt Kasten, ASS-Stile und -Grössen und PGS-Bilder aus der Datei
  (`setApplyEmbeddedStyles`, `setApplyEmbeddedFontSizes`). Eine eigene Grössen- und Stilwahl sieht das Papier
  nicht vor; hat jemand die Untertitel der Bedienungshilfen des Systems eingestellt, gelten deren Grösse und
  Stil.

**Abnahmeliste je Gerät und Tonanlage.** Für jedes Gerät (Chromecast oder Google TV Streamer, Nvidia Shield,
Fernseher mit Google TV) einmal mit den Lautsprechern des Fernsehers, einmal über eine Soundbar an ARC und, wo
vorhanden, über einen Receiver an HDMI/eARC; dazu ein Bluetooth-Kopfhörer. Testdateien: eigene Filme (nicht
ins Repo) mit E-AC-3 5.1, AC-3 5.1, DTS 5.1, TrueHD 7.1, AAC 5.1, PGS- und ASS-Untertiteln, HDR10, HLG, Dolby
Vision Profil 8.1.

| Nr. | Schritte | Erwartet |
|---|---|---|
| 1 | Einstellungen › Wiedergabe, Abschnitt «Ton» | «Erkannt: …» nennt, was die Anlage kann (Lautsprecher: Stereo; Soundbar: Dolby Digital, Dolby Digital Plus, 5.1; Receiver eARC: dazu DTS, Dolby TrueHD, 7.1) |
| 2 | Film mit E-AC-3 5.1 abspielen, «Info» | «Direktwiedergabe», Ton «Dolby Digital Plus · 5.1», «Ton wird an die Tonanlage durchgereicht»; die Anlage zeigt Dolby Digital Plus. An den Lautsprechern: «Ton wird im Fernseher dekodiert», Ton hörbar |
| 3 | Derselbe Film, «Qualität» 720p | Abspielart Transcode, Ton bleibt Dolby (kopiert), die Anlage zeigt weiter Dolby; Bild nach höchstens wenigen Sekunden |
| 4 | Film mit DTS 5.1 über Soundbar ohne DTS | kein stummer Film: der Server wandelt den Ton (AC-3 5.1 oder AAC 5.1), Info zeigt Remux oder Transcode |
| 5 | Film mit TrueHD 7.1 über Receiver an eARC | Direktwiedergabe, durchgereicht, Receiver zeigt TrueHD; über Soundbar: neu kodiert, Ton hörbar |
| 6 | Film mit AAC 5.1 über Receiver ohne Dolby (nur PCM 5.1) | «Ausgang mit 5.1 erkannt», Ton auf allen Kanälen |
| 7 | Während der Wiedergabe: Anlage ausschalten, dann wieder ein; Bluetooth-Kopfhörer verbinden und trennen | nach etwa 1 s ein Neustart an derselben Stelle; Info zeigt Stereo bzw. wieder Mehrkanal; mit Kopfhörer nichts durchgereicht. Media3 pausiert beim Wegfall des Ausgangs selbst («becoming noisy»): dann pausiert an derselben Stelle, Weiter mit OK |
| 8 | Einstellungen: «Immer Stereo», Film aus Nr. 2; im Menü «Ton» eine Dolby- oder DTS-Spur wählen | Stereo, «Ton wird im Fernseher dekodiert», die Anlage zeigt PCM 2.0; die gewählte Spur kommt über einen Neustart als Stereo, nie als Dolby an die Anlage |
| 9 | Einstellungen: «Formate von Hand», nur Dolby Digital, 5.1; DTS-Film | der Server wandelt DTS; mit DTS angekreuzt geht DTS durch (falls die Anlage es kann) |
| 10 | HDR10-Datei auf HDR-Fernseher | der Fernseher wechselt in HDR, Info «HDR10», Direktwiedergabe oder Remux; auf SDR-Gerät: «auf SDR umgewandelt» |
| 11 | Dolby-Vision-Profil-8.1-Datei auf Gerät ohne DV | Bild in HDR10 (Grundschicht), keine Fehlfarben (F-8) |
| 12 | Datei mit PGS- und ASS-Untertiteln, direkt | PGS an der Stelle aus der Datei, ASS mit Farben und Position; Wahl im Menü ohne Neustart |
| 13 | Fernbedienung: OK, Links/Rechts tippen und halten, Play/Pause-Taste, Vor/Zurück in einer Serie, Stopp | wie im Papier 5.1; gehaltene Taste spult in wachsenden Schritten, Vorschaubild, ein Sprung beim Loslassen |
| 14 | Während es spielt 10 min warten; pausieren und warten | kein Bildschirmschoner beim Spielen; beim Pausieren kommt er, danach steht der Film pausiert an derselben Stelle |
| 15 | Home drücken, App wieder öffnen; Standby und wieder an | kein Ton im Hintergrund; zurück pausiert an derselben Stelle |
| 16 | Folge bis zum Abspann | Karte «Nächste Folge in 10», danach die nächste Folge; «Abbrechen» bleibt in der Folge |
| 17 | Film starten, mit Zurück verlassen, sofort Einstellungen › Profil wechseln; ebenso «Diesen Fernseher abmelden» | die Sitzung ist am Server beendet (die Streamübersicht der Verwaltung zeigt keinen Stream mehr), das neue Profil sieht keine Bilder des alten |
| 18 | Tempo (Shield, schwächstes Gerät) | Bedienebene erscheint ohne Ruckeln, Spulen flüssig |
| 19 | Film mit AC-3 5.1: an den Lautsprechern, dann an der Soundbar über ARC | Lautsprecher: dekodiert, Ton hörbar; Soundbar: «durchgereicht», die Soundbar zeigt Dolby Digital |
| 20 | AAC 7.1 oder FLAC 7.1 an einem Receiver an eARC, der kein TrueHD kann | «Ausgang mit 7.1 erkannt», gemeldet 8 Kanäle, alle acht Kanäle hörbar |
| 21 | HLG-Datei auf HDR-Fernseher und auf SDR-Gerät | HDR: der Fernseher zeigt HLG, Info «HLG»; SDR: «auf SDR umgewandelt» |
| 22 | Dolby Vision Profil 5 auf einem HDR10-Gerät ohne Dolby Vision | der Server wandelt um (Info: Transcode, Grund Dolby Vision), Farben richtig, kein Grün/Violett |
| 23 | Datei mit PGS und ASS, «Qualität» unter der Datei (HLS) | PGS eingebrannt, ASS als einfacher Text (WebVTT), beides zur richtigen Zeit |
| 24 | Pausieren, dann Anlage aus und wieder an; ebenso direkt nach dem Start, solange noch «Wird geladen» steht | bleibt pausiert an derselben Stelle (kein Losspielen, kein Sprung an den Anfang); beim Start kommt das neue Profil an |
| 25 | Während der Wiedergabe am Fernseher den HDMI-Eingang wechseln und nach 10 min zurück | je nach Gerät: die Box schläft per CEC (dann pausiert zurück) oder der Film lief weiter; Ergebnis je Gerät notieren (Review O-3) |
| 26 | System-Untertitel (Bedienungshilfen) auf «sehr gross» stellen, Film mit Textuntertiteln aus 3 m | die Grösse des Systems gilt, lesbar aus 3 m; ohne Systemwahl Kontur in der Standardgrösse |
| 27 | Mit «Dynamikbereich/Bildfrequenz anpassen» des Systems: HDR-Datei starten (Shield, Google TV) | der Film läuft weiter; baut das System die App beim Moduswechsel neu, steht er pausiert an derselben Stelle (Review O-2) |
| 28 | HDR in den Systemeinstellungen abschalten, HDR10-Datei | das Bild kommt umgewandelt (SDR) oder vom System richtig abgebildet, nicht flau (Review O-4) |

## Stufe 4: Ton (23. November 2026)

Gebaut auf `claude/k26-stufe4-a` auf der gemeinsamen Tonlogik aus K27 Stufe 7 (`AudioPlayback`,
`RadioController`, die ViewModels von Reihe, Folgen und Webradio aus `:core:browse`). Geprüft mit JVM-Tests und
im **Emulator** `sc_tv` (ohne Tonausgabe); was nur ein Gerät mit Lautsprechern zeigen kann, steht unten als
Abnahmeliste.

**Entscheide des Baus:**

- **Ton über die Seiten:** Hörbuch, Folge oder Sender spielen weiter, während man stöbert, solange die App im
  Vordergrund ist (Papier 5.6, Mockup `mdSync`). Oben rechts zeigt eine Plakette, was läuft; in der Leiste
  führt «Läuft gerade» (bzw. «Radio läuft») zurück zum Tonplayer oder zum Sender, ohne neu zu starten.
- **Home und Standby:** Der Titel meldet seinen letzten Stand und beendet die Sitzung am Server; zurück in der
  App wartet er pausiert an derselben Stelle. Ein Sender endet (`endRadioListen`). Die App wartet nach dem
  Stopp der Activity 1,5 s, ob der Bildschirmschoner sich meldet (`ACTION_DREAMING_STARTED`): dann spielt der
  Ton weiter. Geht der Bildschirm aus (`ACTION_SCREEN_OFF`), endet die Sitzung sofort. Endet der Schoner,
  ohne dass die App wieder vorn ist (Home, eine andere App), gilt dieselbe Frist noch einmal. Verschwindet
  der HDMI-Ausgang (anderer Eingang am Fernseher, `ACTION_HDMI_AUDIO_PLUG`) länger als 1,5 s, endet die
  Sitzung ebenso; kommt er zurück, wartet der Titel pausiert. Entschieden wird im Scope des Graphen, nicht
  der Seite.
- **App schliessen:** Das zweite Zurück auf der Startseite (`finish()`) beendet den Ton sofort, ohne Frist.
- **Bildschirmschoner und Einbrennschutz:** Der Tonplayer hält den Bildschirm nicht wach. Nach 12 s ohne Taste
  wird er ruhig (Mockup): kleines Bild, Titel und Fortschritt, das langsam wandert; die erste Taste weckt ihn
  nur. Danach darf der Bildschirmschoner des Fernsehers kommen.
- **Video:** Wird ein Film geöffnet, beendet das Hörbuch seine Sitzung am Server, bevor der Film seine
  beginnt (keine zwei Sitzungen, Stream-Grenze); «Läuft gerade» behält es, nach dem Film wartet es pausiert.
  Ein Sender endet.
- **Tasten:** Links/Rechts auf der Zeitleiste und die Spultasten springen 15 s zurück und 30 s vor. Das
  TV-Mockup zeigt 30/30, das Papier nennt keinen Wert; es gilt der gemeinsame Wert `AudioRules` aus
  `:core:player`, den auch der Tonplayer der Web-App (`player.audio.back15`/`forward30`) und das Handy
  nutzen. Vor/Zurück wechseln das Kapitel bzw. die Folge. Play und Pause getrennter Tasten schalten nicht
  um. Auf den anderen Seiten nimmt die App die Medientasten selbst ab, solange Ton läuft: Play/Pause,
  Stopp (beendet die Sitzung), Spulen ±15/30 s, Vor/Zurück wie im Tonplayer.
- **Prozess-Tod:** Keine Wiederaufnahme und keine letzte Meldung; die Sitzung bleibt bis zu ihrem Ablauf
  am Server in der Streamübersicht (bekannte Grenze).
- **App schliessen, letzte Meldung:** `TvAudio.leave()` beendet den Ton sofort und wartet nicht auf die
  letzte Fortschrittsmeldung und `stopPlayback` bzw. `endRadioListen`. Sie gehen im Scope der App hinaus
  und kommen an, solange der Prozess noch einen Moment lebt (der Regelfall). Beendet das System den Prozess
  gleich nach `finish()`, fehlen sie wie beim Prozess-Tod: Der Fortschritt steht auf der letzten regelmässigen
  Meldung, die Sitzung läuft am Server ab (bekannte Grenze). Ein blockierendes
  Warten in `onDestroy` würde den Hauptfaden anhalten (ANR-Gefahr) und ist darum nicht gebaut; Profilwechsel
  und Abmelden warten weiterhin bis 3 s (`TvAudio.end()`).
- **Tempo** mit 0,75× bis 2× wie im Mockup; kein Schlaftimer am Fernseher (Mockup und Papier nennen keinen).
- **Webradio:** OK startet einen Sender, nochmals OK hält ihn an (Mockup). Die Länder wählt ein Seitenmenü
  wie das Blatt am Handy; «Meine Sender» und die Sender sind Reihen von Kacheln mit dem Herz darunter.
  Die Reihe «Meine Sender» der Startseite fehlt wie am Handy (`HomeViewModel` lässt sie aus).

**Abnahmeliste je Gerät.** Chromecast oder Google TV Streamer, Nvidia Shield, Fernseher mit Google TV; Ton
über die Lautsprecher des Fernsehers und über eine Anlage an ARC/eARC.

| Nr. | Schritte | Erwartet |
|---|---|---|
| 1 | Hörbuch aus mehreren Dateien starten, eine Stunde laufen lassen, zwischendurch Kapitel mit Vor/Zurück wechseln | Ton ohne Lücke an den Dateigrenzen, Kapitelanzeige und Zeitleiste stimmen, kein Abbruch |
| 2 | Ton über die Anlage (ARC/eARC) und über die Lautsprecher des Fernsehers | hörbar, Lautstärke der Fernbedienung wirkt, kein Knacken beim Pausieren |
| 3 | Hörbuch abspielen, nichts drücken | nach 12 s ruhige Anzeige, später der Bildschirmschoner des Geräts, der Ton spielt weiter; eine Taste weckt ohne Aktion |
| 4 | Während es spielt Home drücken; App wieder öffnen | Ton aus; zurück pausiert an derselben Stelle, «Läuft gerade» zeigt «Pausiert»; am Server stimmt die Position (Web-App: Fortsetzen) |
| 5 | Während es spielt den Fernseher in Standby schalten und wieder einschalten (auch über CEC der Fernbedienung) | Ton aus, Sitzung beendet; danach pausiert an derselben Stelle |
| 6 | Webradio: Sender starten, über Nacht laufen lassen (Bildschirmschoner an) | kein Abbruch, der Herzschlag hält die Sitzung, der laufende Titel wechselt; am Morgen spielt er noch |
| 7 | Webradio spielt, Film starten; ebenso mit einem Hörbuch | der Sender endet, der Film spielt; ein Hörbuch beendet seine Sitzung (Streamübersicht zeigt nur den Film) und wartet nach dem Film pausiert unter «Läuft gerade» |
| 8 | Podcast-Folge bis zum Ende hören | die Folge gilt als gehört (Haken in der Liste der Podcast-Seite) |
| 9 | Play/Pause, getrennte Play- und Pause-Tasten (CEC, Shield), Stopp und Spultasten auf einer anderen Seite, während ein Hörbuch spielt | pausiert und spielt weiter, Play pausiert nie; Stopp beendet die Sitzung, «Läuft gerade» verschwindet; Spulen 15/30 s |
| 10 | Profil wechseln und «Diesen Fernseher abmelden», während etwas spielt | der Ton endet sofort, die Streamübersicht der Verwaltung zeigt nichts mehr |
| 11 | HDMI-Eingang am Fernseher wechseln, während ein Sender läuft, nach 10 min zurück | meldet das Gerät den Wegfall des HDMI-Tons: der Sender endet nach 1,5 s; sonst läuft er weiter oder die Box schläft per CEC. Ergebnis je Gerät notieren |
| 12 | Hörbuch spielt, warten bis der Bildschirmschoner kommt, dann Home drücken | der Ton endet nach etwa 1,5 s, zurück in der App wartet der Titel pausiert |
| 13 | Hörbuch spielt, auf der Startseite zweimal Zurück (App verlassen) | der Ton endet sofort |
| 14 | Webradio über Nacht mit Bildschirmschoner auf einem Gerät mit 2 GB | läuft am Morgen noch; beendet das System die App, notieren (Review O-1: dann braucht 5.6 einen Entscheid) |
