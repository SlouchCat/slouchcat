# Konzept 28 · Recherche und Quellen

Stand: 29. November 2026 · Fassung 2 · Status: Entwurf nach Gesamtreview, nicht freigegeben

Diese Übersicht verdichtet fünf Rechercheberichte (R1 bis R5) und eine Messung (M1). Sie enthält keine neuen Befunde und ruft nichts im Netz ab. Die Rohberichte liegen ausserhalb des Repos (`.omc/research/live-tv/`). Alles in Konzept 28 ist «spezifiziert», nichts ist «implementiert»; die tragenden Rechercheaussagen hat ein unabhängiger Agent nachgeprüft (`anhang/nachpruefung-quellen.md`); Empfehlungen aus den Berichten sind Hypothesen, keine Entscheide.

## Änderungen gegenüber der vorigen Fassung

Fassung 2 zieht das Dokument auf die unabhängige Nachprüfung der tragenden Rechercheaussagen (`anhang/nachpruefung-quellen.md`, 45 Zeilen: 26 bestätigt, 13 teilweise, 2 widerlegt, 3 nicht prüfbar, 1 nicht geprüft; Befund RG-07 des Gesamtreviews). Zitiert wird als «Nachprüfung Nr. n».

- **Einstufung:** Jede Aussage in den Tabellen von Abschnitt 2, die in der Nachprüfung vorkommt, trägt deren Befund und die Nummer. Neu: Abschnitt 2.6 mit Aussagen, die nur die Nachprüfung prüft (Landlock, `go-landlock`, `http_proxy`, MPEG-TS-Muxer, HLS-Pflichten).
- **Widerlegt:** Media3 `DefaultLoadControl` (richtig 1000 / 2000 ms statt 2500 / 5000); FFmpeg-HLS-Demuxer `max_reload` (richtig 100 statt 1000).
- **Abweichend oder nur teilweise:** Apple verlangt mindestens 6 Segmente je Live-Liste und IDR am Segmentanfang, HEVC nur in fMP4; der Light-Bau von hls.js liest in TS kein HEVC und AC-3 (CEA-608 «vermutlich weg» ist nicht belegt); Landlock regelt UDP ab ABI 10, `BestEffort()` ist nicht zu verwenden; `dd_progid` steht nicht in der XMLTV-DTD; Docker-Multicast stützt sich nur auf Foren; Sprung zur Live-Kante in hls.js ist ab Werk aus; Tiefengrenze in `encoding/xml` gilt nur für `Unmarshal` und `DecodeElement`, nicht für `Decoder.Token` (Nachprüfung Nr. 32).
- **Abschnitt 6:** je Widerspruch neu der Stand (entschieden durch Nachprüfung, entschieden in 04 und 05, entschieden in einem anderen Dokument, offen). W-5 und W-13 sind durch die Nachprüfung entschieden, W-11 teilweise.
- **Abschnitt 7:** Was die Nachprüfung erledigt hat, ist gestrichen; neu aufgenommen sind die Punkte, die sie nicht prüfen konnte.
- **Spalte «verwendet in»:** gegen die vorhandenen Dokumente 04 bis 18 per Stichwortsuche geprüft (nicht Zeile für Zeile gegen den Inhalt); 18 neu aufgenommen, wo Stufen betroffen sind.
- **Quellenverzeichnis:** neuer Abschnitt 8.7 mit den Quellen der Nachprüfung; in 8.1, 8.2, 8.3 und 8.5 steht, wo die Nachprüfung eine Quelle nachträglich abgerufen hat.
- **Stand der Angaben in Abschnitt 6:** aus 04, 16 und 20 übernommen (Stichprobe der Entscheidstellen); Dokumente 04 und 05 stehen in Fassung 4, 10 in Fassung 2.
- Nach der Schlussprüfung (Review 7): Nachprüfung Nr. 32 (Tiefengrenze `encoding/xml`) ist «teilweise», nicht «widerlegt».

## 1. Vorgehen und Grenzen

### 1.1 Vorgehen

Fünf Recherche-Agents arbeiteten am 28. November 2026, je ein Thema:

| Bericht | Thema | Hauptdokumente |
|---|---|---|
| R1 | Zentraler Empfang, interne Verteilung, Fast Zapping (schnelles Umschalten), FFmpeg-Prozessführung | 04, 05, 13 |
| R2 | Auslieferung an die Clients (Web, Handy, TV) | 16, 05, 14 |
| R3 | Eingangsquellen und Protokolle (M3U, Xtream, HLS, DASH, RTSP, UDP, DRM-Erkennung) | 04, 12, 15 |
| R4 | EPG (elektronische Programmzeitschrift) und XMLTV-Format | 09, 07 |
| R5 | Timeshift (zeitversetztes Sehen) und DVR (Aufnahmen) | 10, 11, 13 |

Dazu M1: eine einzige echte Messung der FFmpeg-Fähigkeiten im Slouchcat-Image (Abschnitt 5).

### 1.2 Grenzen, ehrlich benannt

- **Gemessen wurde nichts ausser M1.** Alle Zielwerte (Umschaltzeit, CPU, RAM, Datenmengen) sind zu prüfende Grössen, keine Ergebnisse. Aus dem Repo übernommene ältere Messwerte (Webradio-Relay, VoD-Start, TV-App-Start) stammen aus früheren Konzepten und sind keine Messung für Live-TV.
- **Quellenlage je Bericht** (Zählung nach den Quellenlisten der Berichte; Überschneidungen zwischen Berichten sind nicht bereinigt):

| Bericht | Externe Quellen abgerufen | Extern nicht abgerufen oder nur über Suchtreffer | Repo-Dateien gelesen | Anteil Vorwissen |
|---|---|---|---|---|
| R1 | 31 | 4 (Q8 TVHeadend-Doku, Q12b nginx-rtmp-Wiki, Q24 und Q27 FFmpeg-Tickets: nur Suchtreffer oder Sekundärquellen) | 7 | gering; Teilzeiten, Keyframe-Abstände und Vergleichsurteile sind [Annahme] |
| R2 | 5 | 8 (RFC 8216, Apple-Spezifikation, `DefaultLoadControl`, `HlsMediaSource.Factory`, `DefaultTsPayloadReaderFactory`, mpegts.js `api.md`, `getStartDate`, Amazon Fire TV) | 6 | hoch; Geräte- und Safari-Aussagen sind [Annahme] oder [Beleg*] |
| R3 | 6 (FFmpeg-Formats-Seite nur die ersten 100'000 Zeichen) | 18 (plus rund 6 im Text genannte URLs) | 0 | hoch; M3U, Xtream, Docker-Netz, DRM, Verhalten der Vergleichssysteme sind [Beleg*] oder [Annahme] |
| R4 | 4 (FTS5-Seite nur 100'000 von rund 172'000 Zeichen; DTD nur als Zusammenfassung des Abruf-Werkzeugs) | 4 Verweise (SQLite WAL, Go-Release-Notes, RFC 1952, Quellcode der DVR-Tools) | 4 | hoch; Vergleichssysteme und Go-Verhalten sind [Wissen] |
| R5 | 23 (davon 5 Forum- oder README-Angaben) | 2 (Plex-Support: HTTP 403; TVHeadend-Doku: 404, stattdessen Quellcode gelesen) | 2 | mittel; Channels DVR, Plex, Enigma2 sind [Annahme] |
| **Summe** | **69** | **36** | | |

- **Kennzeichen der Berichte:** «[Beleg]» heisst abgerufen (oder im Repo gelesen); «[Beleg*]» heisst: offizielle Quelle, aus dem Gedächtnis genannt, nicht abgerufen; «[Wissen]» heisst Fachkenntnis ohne Abruf. In diesem Dokument gilt: **«belegt» nur, wo der Bericht oder die Nachprüfung eine abgerufene Quelle nennt.** [Beleg*] und [Wissen] stehen hier als «Annahme aus Vorwissen».
- **Nachprüfung (Fassung 2):** Die Spalte «Einstufung» nennt, wo die Nachprüfung eine Aussage an Primärquellen gelesen hat («belegt (Nachprüfung Nr. n)»). Sie las Quelltexte an den Tags der Produktversionen (hls.js v1.7.3, Media3 1.11.1, FFmpeg n7.1.4, Go 1.24.0); Go 1.27 las sie nicht. Lange Seiten las sie nur in den ersten 100'000 Zeichen. Was sie nicht prüfen konnte, steht in Abschnitt 7.
- **Zuordnung «verwendet in»:** Die Dokumente 04 bis 18 liegen vor (04 Architektur · 05 Streaming und Fast Zapping · 06 Abläufe · 07 Datenmodell · 08 API · 09 EPG · 10 Timeshift · 11 DVR · 12 Sicherheit · 13 Ressourcen und Performance · 14 Oberfläche · 15 Administration · 16 Client-Kompatibilität · 17 Test und Abnahme · 18 Umsetzungsplan). Die Zuordnung ist per Stichwortsuche gegen diese Dokumente geprüft, nicht Zeile für Zeile.
- **Versionsstand der Quellen:** Doku-Seiten (FFmpeg, hls.js, Media3) sind teils neuer als die im Repo gepinnten Versionen (hls.js 1.7.3, Media3 1.11.1, FFmpeg 7.1.4). Defaults aus dem Hauptzweig gelten für die gepinnten Versionen als zu prüfen.

### 1.3 Begriffe in einem Satz

| Begriff | Bedeutung |
|---|---|
| Keyframe / GOP | Vollbild, von dem aus ein Decoder starten kann; GOP = Bildgruppe zwischen zwei Keyframes. Ohne Keyframe gibt es kein Bild. |
| MPEG-TS (TS) | Übliches Containerformat für Fernsehstreams (Pakete zu je 188 Byte). |
| HLS / fMP4 | Auslieferung in kurzen Segmenten über HTTP, mit Segmenten als TS oder fMP4. |
| Remux / Transcode | Remux packt den Strom um, ohne ihn neu zu rechnen; Transcode rechnet Bild oder Ton neu. |
| Ringpuffer | Speicher fester Grösse, der die ältesten Daten überschreibt. |
| XMLTV | Dateiformat für Programmdaten (EPG). |
| SSRF | Angriff, bei dem der Server verleitet wird, interne Adressen abzurufen. |
| DRM | Technischer Kopierschutz; Slouchcat umgeht ihn nicht. |

## 2. Recherche je Thema

Einstufung: **belegt** (abgerufene Quelle) · **Annahme** (aus Vorwissen, Forum oder Suchtreffer) · **Empfehlung** (Hypothese des Berichts) · **zu messen**.

### 2.1 Stream-Sharing und Fast Zapping (R1)

| Aussage | Einstufung | Quelle (URL) | verwendet in |
|---|---|---|---|
| Jellyfin kopiert die Quelle in eine Temp-Datei und gibt sie über eine lokale URL an alle Abnehmer; kein Reconnect | belegt (Nachprüfung Nr. 42) | https://raw.githubusercontent.com/jellyfin/jellyfin/master/src/Jellyfin.LiveTv/TunerHosts/SharedHttpStream.cs | 04, 05 |
| Jellyfin-Leser springt nach 10 s auf 20 000 Byte vor das Dateiende (kein Vielfaches von 188, kein Keyframe); keine Grössengrenze der Datei | belegt (Nachprüfung Nr. 43); «keine Grössengrenze» gilt für die zwei gelesenen Dateien | https://raw.githubusercontent.com/jellyfin/jellyfin/master/src/Jellyfin.LiveTv/TunerHosts/LiveStream.cs | 05, 13 |
| Dispatcharr: Redis-Stücke zu 188 × 5644 Byte mit TTL, Cursor je Client, neue Clients 5 s hinter live, Failover-Konstanten (10 Wechsel, 20 s) | belegt (Quellcode; Nachprüfung Nr. 45); HLS-Ausgang mit `random_access_indicator` nicht nachgelesen | https://github.com/Dispatcharr/Dispatcharr/blob/main/apps/proxy/live_proxy/input/buffer.py ; https://github.com/Dispatcharr/Dispatcharr/blob/main/apps/proxy/live_proxy/config_helper.py | 04, 05 |
| Dispatcharr-TS-Ausgang beginnt an Stückgrenzen, nicht am Keyframe; HLS-Ausgang nutzt `random_access_indicator` | Annahme (Code-Suche, nicht vollständig gelesen) | wie oben | 05 |
| TVHeadend: Timeshift je Subscription, RAM zuerst, dann Platte; OOM-Berichte bei «unlimited» | belegt (Nachprüfung Nr. 44, Quellcode `timeshift.c`); R1 selbst nur Suchtreffer | https://tvheadend.readthedocs.io/en/latest/webui/config_timeshift/ ; https://old.tvheadend.org/issues/5109 (R5 belegt dasselbe am Quellcode, siehe 2.5) ; https://github.com/tvheadend/tvheadend/blob/master/src/timeshift.c | 05, 10 |
| SRS: `gop_cache` gibt neuen Zuschauern sofort ein Keyframe; ohne Grenze OOM-Risiko; GOP-Länge bestimmt die Quelle | belegt | https://github.com/ossrs/srs/wiki/migrate_v4_EN_low-latency ; https://github.com/ossrs/srs/pull/3236 | 05, 13 |
| nginx-rtmp: `wait_key`, `wait_video` (Ton nicht vor erstem Bild) | Annahme (Sekundärquellen) | https://github.com/arut/nginx-rtmp-module/wiki/Directives | 05 |
| Channels DVR teilt IPTV-Verbindungen; Emby teilt nur reinen MPEG-TS-Kanal | Annahme (Forum) | https://community.getchannels.com/t/channels-reusing-stream-for-multiple-viewers/46603 ; https://emby.media/community/topic/46113-multiple-streams-watching-the-same-m3u-channel | 05 |
| Plex: keine Primärquelle zum Teilen von IPTV-Streams gefunden | Annahme (kein Befund) | – | – |
| Webradio-Relay (ADR 0025): ein Relay je Sender, Go liest, ffmpeg nur über Pipes, Generationen, Byte-Budget 512 MB, tmpfs | belegt (Repo) | `docs/adr/0025-webradio-katalog-und-weiterleitung.md` | 04, 05, 13 |
| Radio-Messwerte: erster Ton bei laufendem Relay 38 bis 156 ms, bei neuem Relay 0,16 bis 1,48 s (Ton, nicht Video) | belegt (Repo, ältere Messung) | `docs/perf/radio.md` | 13, 17 |
| `netguard.RefuseLocal` verweigert lokale Adressen; LAN- und Multicast-Quellen brauchen eine bewusste Ausnahme | belegt (Repo) | `server/internal/netguard/netguard.go` | 12, 04 |
| VoD-Start QSV 1080p: erstes Segment rund 0,87 s; TV-App erstes Bild 127 bis 288 ms (VoD) | belegt (Repo, ältere Messung) | `docs/perf/wiedergabestart.md` ; `docs/perf/K26-android-tv.md` | 13, 17 |
| hls.js-Defaults: `liveSyncDurationCount` 3, `lowLatencyMode` true, `maxBufferLength` 30 s; unter 3 Segmenten Stockungsrisiko | belegt (Nachprüfung Nr. 9, Tag v1.7.3 gelesen); `lowLatencyMode` ist ab Werk an und wird für Live-TV auf `false` gesetzt | https://raw.githubusercontent.com/video-dev/hls.js/master/docs/API.md | 05, 16 |
| HLS-Entwurf: `HOLD-BACK` mindestens 3 × Target Duration; `PART-HOLD-BACK` mindestens 2 ×, soll 3 × | belegt (Nachprüfung Nr. 2); steht im Entwurf -22, nicht in RFC 8216 | https://datatracker.ietf.org/doc/html/draft-pantos-hls-rfc8216bis | 05, 16 |
| Media3: Apple-LL-HLS ja, Blocking Reload ja, Community-LL-HLS nein; progressive Live-Streams ohne Live-Fenster | belegt (Nachprüfung Nr. 17, 18) | https://developer.android.com/media/media3/exoplayer/supported-formats ; https://developer.android.com/media/media3/exoplayer/live-streaming | 05, 16 |
| `DefaultLoadControl` in Media3 1.11.1: 1000 ms bis Start, 2000 ms nach Rebuffer, Min- und Max-Puffer je 50 000 ms | belegt (Nachprüfung Nr. 16, Tag 1.11.1 gelesen); die Zahlen gelten für 1.11.1 | https://github.com/androidx/media/blob/main/libraries/exoplayer/src/main/java/androidx/media3/exoplayer/DefaultLoadControl.java ; https://raw.githubusercontent.com/androidx/media/1.11.1/libraries/exoplayer/src/main/java/androidx/media3/exoplayer/DefaultLoadControl.java | 04, 05, 16 |
| FFmpeg-HLS-Muxer kennt nur `lhls` (nicht Apples Variante); kein `EXT-X-PART` dokumentiert | belegt (Doku der neuesten Version) | https://ffmpeg.org/ffmpeg-formats.html | 05 |
| FFmpeg 9.1 «Gauss» ist aktuell; im Image läuft 7.1.4 (jellyfin-ffmpeg) | belegt | https://ffmpeg.org/download.html ; Repo `deploy/Dockerfile` | 13 |
| 33-Bit-PTS läuft nach rund 26,5 h über; Fehler mit `-copyts` im mpegts-Muxer | teilweise (Nachprüfung Nr. 29): Rechnung stimmt; FFmpeg behandelt den Überlauf laut Mail 2019 nur einmal (zweiter Bruch bei rund 53 h); für 7.1.4 nicht belegt, Ticket #7876 nicht abgefragt | https://ffmpeg.org/pipermail/ffmpeg-devel/2019-May/243443.html ; https://trac.ffmpeg.org/ticket/7876 | 05, 13 |
| FFmpeg-Speicherwachstum bei Live-Eingang über 24 h (Ticket #11629), 24/7-Transcode (#10500); Status von #11629 nicht abrufbar | belegt laut R1 (Mailingliste); Trac-Tickets in der Nachprüfung nicht abgefragt: nicht prüfbar | https://ffmpeg.org/pipermail/ffmpeg-trac/2025-June/073873.html ; https://ffmpeg.org/pipermail/ffmpeg-trac/2023-August/066729.html | 13, 05 |
| Bell Labs: häufigere I-Frames senken die mittlere Umschaltzeit in Multicast-IPTV auf rund 300 ms | belegt | https://nokia.com/bell-labs/publications-and-media/publications/channel-change-delay-in-iptv-systems | 05 |
| Go-Bibliotheken `bluenviron/*`, `go-astits`: Lizenz und Stand | belegt (GitHub-API); Lizenz und neuester Tag in der Nachprüfung bestätigt (Nr. 36 bis 38), Push-Daten nicht ablesbar | siehe Abschnitt 4 | 04, 18 |
| Typischer Keyframe-Abstand: DVB 0,5 bis 1 s, OTT 1 bis 2 s, Restreams 5 bis 10 s | Annahme aus Vorwissen | – | 05, 13 |
| Teilzeiten des Umschaltens (Verbindung, Probing, Decoder-Start) | Annahme; zu messen | – | 05 |
| Umschaltziele (unter 1 s, 3 s, 5 s): kein veröffentlichter Messwert belegt sie für eine vergleichbare Architektur | zu messen | – | 05, 17 |
| iPhone-Safari: kein klassisches MSE, nur ManagedMediaSource ab iOS 17.1 (R2 belegt dies über die mpegts.js-README) | belegt (Nachprüfung Nr. 15: hls.js-README, WebKit-Blog, MDN) | https://webkit.org/blog/14735/webkit-features-in-safari-17-1/ ; https://developer.mozilla.org/en-US/docs/Web/API/ManagedMediaSource | 16 |
| Intern ein TS-Ringpuffer je Sender in Go mit GOP-Index, Cursor je Abnehmer, Start am letzten Keyframe (Variante D) | Empfehlung (Hypothese) | – | 04, 05 |
| Go empfängt, ffmpeg nur kurzlebig je Zuschauer oder Aufnahme und planmässig neu gestartet; nie Dauerempfänger | Empfehlung | – | 04, 13 |
| Vorladen von Nachbarsendern nur als Empfang in den Ring, mit Höchstzahl und Nachlauf | Empfehlung | – | 05, 13 |
| LL-HLS nicht in der ersten Stufe; später mit gohlslib, falls Messungen die Latenz zum Signal als störend zeigen | Empfehlung | – | 05 |

### 2.2 Client-Kompatibilität (R2)

| Aussage | Einstufung | Quelle (URL) | verwendet in |
|---|---|---|---|
| Ist-Stand: hls.js 1.7.3 als `hls.js/light`; Radio nutzt `liveSyncDurationCount: 2`, `lowLatencyMode: false`; Media3 1.11.1, kein `LoadControl`, keine `LiveConfiguration` | belegt (Repo) | `web/src/features/player/capabilities.ts`, `web/src/app/radio/engine.ts`, `android/core/player/.../Media3Engine.kt` | 16, 14 |
| Media3 HLS: TS und fMP4 ja; DASH ohne TS; progressiv MPEG-TS und fMP4 ja, aber ohne Live-Fenster; Plattform-Decoder | belegt (Nachprüfung Nr. 17) | https://developer.android.com/media/media3/exoplayer/supported-formats ; https://developer.android.com/media/media3/exoplayer/live-streaming | 16, 05 |
| Media3: `LiveConfiguration`, Tempoanpassung, `ERROR_CODE_BEHIND_LIVE_WINDOW`, `Window.windowStartTimeMs` | belegt (Nachprüfung Nr. 18) | https://developer.android.com/media/media3/exoplayer/live-streaming | 16, 10, 14 |
| mpegts.js: H.264, HEVC, AV1; kein MPEG-2-Video; Latenz unter 1 s im besten Fall; kein Seeking; iOS ab 17.1 über ManagedMediaSource | belegt | https://github.com/xqq/mpegts.js | 16 |
| hls.js: Optionen, `liveSyncPosition`, `playingDate`, `startFragPrefetch`, `enableWorker` | teilweise (Nachprüfung Nr. 9, 10): der Sprung zur Live-Kante hängt an `liveMaxLatencyDurationCount`; ab Werk unbegrenzt, also kein automatischer Sprung; Verhalten am Listenanfang nach langer Pause nicht gelesen (M-3) | https://github.com/video-dev/hls.js/blob/master/docs/API.md | 16, 05 |
| ManagedMediaSource: Bedingungen auf Safari, Puffer kann jederzeit geräumt werden | belegt (Nachprüfung Nr. 15) | https://developer.mozilla.org/en-US/docs/Web/API/ManagedMediaSource | 16 |
| Safari und iOS spielen HLS nativ (TS und fMP4); Apple-Spezifikation nennt H.264 und HEVC | teilweise, abweichend (Nachprüfung Nr. 7, 8): Apple verlangt mindestens 6 Segmente je Live-Liste, IDR am Segmentanfang und `EXT-X-PROGRAM-DATE-TIME`; HEVC nur in fMP4, MPEG-2-Video nicht in der Codecliste; AC-3 und E-AC-3 erlaubt | https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices ; https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices | 05, 16 |
| Natives LL-HLS in Safari; `getStartDate()` nur Safari | nicht geprüft (Nachprüfung, Abschnitt 5) | https://developer.apple.com/documentation/webkitjs/htmlmediaelement/1630011-getstartdate | 16 |
| Codec-Matrix (Eingang × Client): HEVC, AC-3, MP2, MPEG-2, AV1 je Gerät | Annahme; zu messen (Gerätewerte). HEVC im Browser: teilweise (Nachprüfung Nr. 21); AC-3, E-AC-3, MP2, LATM in MSE-Browsern: nicht prüfbar (Nr. 23) | – | 16, 17 |
| MPEG-2-Video, MP2 und AAC-LATM lassen sich nicht sinnvoll in fMP4 für MSE packen | Annahme; MPEG-2-Video im Browser belegt (Nachprüfung Nr. 22: nur Safari laut MDN, Apple-Spezifikation kennt es nicht); `hls.js` liest LATM und E-AC-3 in TS nie (Nachprüfung Nr. 13) | – | 16, 05 |
| Interlaced (1080i, 576i): Verhalten von Browser- und Android-Decodern uneinheitlich | Annahme; zu messen | – | 16, 13, 17 |
| Interlaced-Quellen serverseitig deinterlacen als Standard; Ausnahme nur nach Messung | Empfehlung | – | 04, 05, 13, 16 |
| Ton immer nach AAC-LC Stereo; AC-3 und E-AC-3 als Passthrough nur für Safari und Android TV nach Messung | Empfehlung | – | 05, 16 |
| Untertitel: Teletext und DVB-Text serverseitig nach WebVTT; DVB-Bitmap nur mit OCR | Empfehlung | – | 04, 05, 16 |
| `DefaultLoadControl` Standardwerte 50 000 / 50 000 / 2 500 / 5 000 ms «nach Erinnerung» | widerlegt (Nachprüfung Nr. 16): richtig sind 50 000 / 50 000 / 1000 / 2000 ms (Media3 1.11.1); damit ist W-5 entschieden | https://developer.android.com/reference/androidx/media3/exoplayer/DefaultLoadControl ; https://raw.githubusercontent.com/androidx/media/1.11.1/libraries/exoplayer/src/main/java/androidx/media3/exoplayer/DefaultLoadControl.java | 04, 05, 16 |
| `hls.js/light` lässt vermutlich Alt-Audio, Untertitel und CEA-608 weg | teilweise (Nachprüfung Nr. 11 bis 13): belegt, dass der Light-Bau Alt-Audio, Untertitel, CMCD, EME, Interstitials, I-Frame-Trickplay sowie HEVC und AC-3 in TS weglässt; E-AC-3 und LATM in TS liest auch der Voll-Bau nicht. CEA-608 «vermutlich weg» ist nicht belegt, wahrscheinlich falsch (zu messen, M-3) | https://raw.githubusercontent.com/video-dev/hls.js/v1.7.3/README.md ; https://raw.githubusercontent.com/video-dev/hls.js/v1.7.3/build-config.js | 04, 05, 14, 16 |
| RFC 8216: `EVENT`-Playlist, Live-Start bei 3 Zieldauern, `EXT-X-PROGRAM-DATE-TIME` | belegt (Nachprüfung Nr. 1, 3, 4, 5; RFC 8216 gelesen): Startregel 3 Zieldauern, `EVENT`, `EXT-X-DISCONTINUITY`, `EXT-X-PROGRAM-DATE-TIME` | https://www.rfc-editor.org/rfc/rfc8216 ; https://www.rfc-editor.org/rfc/rfc8216.txt | 10, 16 |
| Media3 `setAllowChunklessPreparation` und `CODECS` in der Master-Playlist beschleunigen den Start | nicht geprüft (Nachprüfung Nr. 20); bleibt Annahme (M-4) | https://developer.android.com/reference/androidx/media3/exoplayer/hls/HlsMediaSource.Factory | 05, 16 |
| Media3-TS-Extractor liest H.262, H.264, H.265, AC-3, E-AC-3, MPEG-Audio, AAC-ADTS und -LATM, DVB-Subtitles | belegt (Nachprüfung Nr. 19, Tag 1.11.1): dazu DTS, SCTE-35 durchgereicht; kein Teletext-Leser | https://github.com/androidx/media/blob/release/libraries/extractor/src/main/java/androidx/media3/extractor/ts/DefaultTsPayloadReaderFactory.java | 04, 16 |
| WebRTC verwerfen (Codec-Grenzen, kein Timeshift, kein Media3-Client); DASH und progressives fMP4 verwerfen; LL-HLS nicht in Stufe 1 | Empfehlung | – | 05, 16 |
| Ein HLS-fMP4-Ring je (Sender, Profil), Segmente 1 bis 2 s, GOP gleich Segmentdauer, Programmzeit auf jedem Segment | Empfehlung | – | 05, 10 |
| Timeshift: je Zuschauer eine Playlist ab dem Einschaltpunkt (`EVENT`-artig) oder langes Gleitfenster; Pause bleibt auf dem Server sicher. Die `EVENT`-artige Liste je Zuschauer ist regelkonform, das gleitende Fenster der Live-Kante darf kein `PLAYLIST-TYPE` tragen (belegt, Nachprüfung Nr. 3) | Empfehlung | – | 10, 16 |
| hls.js: `loadSource` auf bestehender Instanz beim Senderwechsel; ExoPlayer `setMediaItem` und `prepare` | nicht prüfbar (Nachprüfung Nr. 14); zu messen (M-3) | – | 05, 16 |
| Fire OS: Start der TV-App unklar; kein Ziel | Annahme (nicht abgerufen) | https://developer.amazon.com/docs/fire-tv/ | – |

### 2.3 Eingangsquellen und Protokolle (R3)

| Aussage | Einstufung | Quelle (URL) | verwendet in |
|---|---|---|---|
| FFmpeg-Optionsnamen und Defaults: `rw_timeout`, `reconnect*`, UDP `fifo_size` 7 × 4096 Pakete, `overrun_nonfatal` 0, `buffer_size` 384 KB, `rtsp_transport`, `rtmp_buffer` 3000 ms | teilweise (Nachprüfung Nr. 26, Tag n7.1.4): Namen belegt; Standardwerte in 7.1.4: `reconnect*` = 0 (aus), `reconnect_delay_max` 120 s, `reconnect_max_retries` -1, `rw_timeout` 0 (aus); die Doku nennt sie nicht | https://ffmpeg.org/ffmpeg-protocols.html ; `libavformat/http.c` (n7.1.4) | 04, 13 |
| FFmpeg-HLS-Demuxer: `max_reload` 100 und `m3u8_hold_counters` 1000 (nicht beide 1000), `seg_max_retry` 0, `extension_picky` an; DASH-Option `cenc_decryption_key(s)` existiert | widerlegt (Nachprüfung Nr. 27): `max_reload` ist 100, nicht 1000; `m3u8_hold_counters` 1000, `seg_max_retry` 0, `extension_picky` an belegt; der Demuxer lässt nur `http`, `https`, `file`, `data` zu | https://ffmpeg.org/ffmpeg-formats.html ; `libavformat/hls.c` (n7.1.4) | 04, 12 |
| HTTP-`timeout`, `-protocol_whitelist` und der `hls`-Protokoll-Abschnitt waren im abgerufenen Teil nicht enthalten | teilweise (Nachprüfung Nr. 24, 26): `tcp` muss auf der Whitelist bleiben, sonst scheitert jede HTTP-Verbindung (belegt); eine Option `timeout` steht nicht in `http.c`, `tcp.c` nicht gelesen (zu messen) | `libavformat/http.c`, `avio.c` (n7.1.4) | 04, 12, 17 |
| Go-Bibliotheken: Lizenz und Stand von gohlslib, gortsplib, go-astits, grafov/m3u8 | belegt (Nachprüfung Nr. 36 bis 38) | siehe Abschnitt 4 | 04 |
| M3U-Format: `tvg-id`, `tvg-logo`, `group-title`, `#EXTVLCOPT`, `#KODIPROP`, URL-Anhang mit `\|User-Agent=` | Annahme aus Vorwissen | https://github.com/kodi-pvr/pvr.iptvsimple | 07, 15 |
| M3U-Senderliste und HLS-Manifest am Inhalt unterscheiden, nie an Endung oder Content-Type | Empfehlung (RFC 8216 aus dem Gedächtnis) | https://datatracker.ietf.org/doc/html/rfc8216 | 06, 15 |
| Xtream-API (`player_api.php`, `get_live_streams`, `get_short_epg` mit Base64-Titeln, `xmltv.php`); keine offizielle Spezifikation | Annahme aus Vorwissen | – | 04, 07, 15, 17 |
| Xtream-Anbieter weichen ab (Zahlen als Strings, HTTP 200 bei falschem Login, HTML-Fehlerseiten) | Annahme; zu messen (nur gegen eigenen Mock) | – | 04, 15, 17 |
| Stabile Sender-Identität: Stufenfolge `stream_id`, `tvg-id`, normalisierte URL, Name und Gruppe; interne ID nie aus einem Schlüssel abgeleitet; Verschwundene markieren statt löschen | Empfehlung | – | 07, 15 |
| Verhalten von Threadfin, Dispatcharr, Jellyfin, TVHeadend, Channels DVR beim Re-Import und bei Ersatzquellen | Annahme aus Vorwissen (Quellcode nicht geprüft) | https://github.com/Dispatcharr/Dispatcharr ; https://github.com/Threadfin/Threadfin ; https://github.com/jellyfin/jellyfin | 07, 15 |
| Docker-Bridge: IGMP-Join kommt nicht beim Host-Switch an; Multicast braucht `network_mode: host` oder macvlan | teilweise (Nachprüfung Nr. 41): die offiziellen Docker-Seiten erwähnen Multicast nicht; die Bridge-Aussage stützt sich nur auf Foren und Blogs (Annahme, M-10). Bei `host` wird `ports:` ignoriert; bei macvlan erreicht der Host den Container nicht direkt und der Switch muss mehrere MAC-Adressen dulden | https://docs.docker.com/engine/network/drivers/host/ ; https://docs.docker.com/engine/network/drivers/macvlan/ | 04, 12, 13, 15, 16, 17 |
| Multicast: dokumentiertes Opt-in mit Host-Netz und Selbstdiagnose in der Oberfläche | Empfehlung | – | 15, 14 |
| RTSP-UDP hinter Docker-NAT unzuverlässig; `-rtsp_transport tcp` erzwingen | Annahme; zu messen | – | 04, 13 |
| FFmpeg erhält kein eingebautes Reconnect für RTSP, RTMP, UDP | Annahme (R1 nennt für HTTP-Reconnect-Optionen einen Beleg) | – | 04, 13 |
| DRM-Erkennung: `SAMPLE-AES*` oder `KEYFORMAT` ungleich `identity`, DASH `ContentProtection`, `#KODIPROP`-Lizenzdaten abweisen; AES-128 mit `identity` bleibt erlaubt | Empfehlung (RFC 8216 und DASH-IF in R3 nicht abgerufen; DASH-IF auch in der Nachprüfung nicht geprüft); Media3 spielt Sample-AES nicht (Nachprüfung Nr. 17) | https://dashif.org/identifiers/content_protection/ | 05, 12, 16, 17 |
| `SAMPLE-AES` mit `identity` ist ein Grenzfall; bis zur Messung ablehnen | Empfehlung; zu messen | – | 12 |
| Art. 39a URG verbietet das Umgehen wirksamer technischer Massnahmen | Annahme aus Vorwissen; juristisch zu klären | – | 12 |
| `-protocol_whitelist` setzen, `file`, `concat`, `subfile` für Nutzerquellen verbieten | Empfehlung; Nachprüfung Nr. 24 belegt, dass `tcp` auf der Liste bleiben muss; Nr. 25: `http_proxy` und `no_proxy` aus der Umgebung liest `http.c`, die Umgebung des Prozesses ist zu bereinigen | https://ffmpeg.org/ffmpeg-protocols.html | 04, 12 |
| FFmpeg als einziger Eingang in der ersten Stufe; Go nur für Parsing, Vorprüfung, Wächter | Empfehlung; entschieden gegen R3 (siehe Abschnitt 6, W-3) | – | 04 |
| Ersatzquellen: Auslöser Start-Fehler und Stille, Hysterese, Quarantäne mit Backoff, Hauptquelle beim nächsten Start zuerst | Empfehlung | – | 06, 15 |
| Eigener streamender M3U-Parser statt Bibliothek (Zeichensatz, Anführungszeichen, 100 000 Zeilen) | Empfehlung; Speicher zu messen | – | 04, 17 |
| Testquellen nur selbst erzeugt (`lavfi`-Testbild, Mock-Server, MediaMTX als Testserver); keine fremden IPTV-Listen | Empfehlung (Befehle nicht getestet) | – | 17 |
| Externe Teststreams (Apple, Bitmovin, DASH-IF, Blender-Filme) | Annahme (nicht abgerufen) | https://developer.apple.com/streaming/examples/ ; https://bitmovin.com/demos/stream-test ; https://dash.akamaized.net/ ; https://peach.blender.org/ | 17 |

### 2.4 EPG und XMLTV (R4)

| Aussage | Einstufung | Quelle (URL) | verwendet in |
|---|---|---|---|
| XMLTV-DTD: `start` und `channel` Pflicht, `stop` optional, Zeit ohne Offset gilt als UTC, `clumpidx`, `episode-num` mit `xmltv_ns` (nullbasiert), `previously-shown` und `new` (Fehlen beweist nichts), keine Programm-ID | belegt (Nachprüfung Nr. 39); `episode-num`-Systeme und Nullbasierung von `xmltv_ns` teilweise (Nachprüfung Nr. 40) | https://raw.githubusercontent.com/XMLTV/xmltv/master/xmltv.dtd ; https://raw.githubusercontent.com/XMLTV/xmltv/master/xmltv.dtd | 09, 07 |
| XMLTV-Wiki: Beispielzeit mit Offset, `dd_progid`, freie Kanal-IDs, Encoding ISO-8859-1 | belegt; `dd_progid` steht nicht in der DTD (Nachprüfung Nr. 40): Konvention von Schedules Direct, nicht DTD-Teil | https://wiki.xmltv.org/index.php/XMLTVFormat | 09 |
| Go `encoding/xml`: `Decoder.Token`, `DecodeElement`, `Entity` (Standard leer), `CharsetReader`, `Strict` | belegt (R4; Nachprüfung Nr. 31 für Go 1.24) | https://pkg.go.dev/encoding/xml | 09, 12 |
| FTS5: External-Content-Tabellen, `contentless_delete` ab SQLite 3.43.0, Trigram, `remove_diacritics` | belegt (100'000 von rund 172'000 Zeichen gelesen) | https://sqlite.org/fts5.html | 09, 12 |
| Das Repo nutzt FTS5 schon (`00015_discover.sql`); Migrationen nur vorwärts | belegt (Repo) | `server/migrations/00015_discover.sql`, `server/internal/store/dump.go` | 09, 12 |
| Go lädt keine externen Entities oder DTDs; `<!DOCTYPE`/`<!ENTITY` kommen als `Directive`-Token; XXE und Billion-Laughs praktisch ausgeschlossen | belegt (Nachprüfung Nr. 31; Go 1.24.0 gelesen, Go 1.27 nicht) | https://raw.githubusercontent.com/golang/go/go1.24.0/src/encoding/xml/xml.go | 12, 17 |
| `maxUnmarshalDepth` (10 000) gilt nur für `Unmarshal` und `DecodeElement`, nicht für `Decoder.Token`; der EPG-Import muss Tiefe und Byte-Menge in der `Token`-Schleife selbst begrenzen | teilweise (Nachprüfung Nr. 32, abweichend): R4 nahm eine allgemeine Tiefengrenze seit Go 1.19 an | https://raw.githubusercontent.com/golang/go/go1.24.0/src/encoding/xml/read.go | 09, 12, 17 |
| Quellenqualität (Xtream-`xmltv.php`, WebGrab+Plus, iptv-org/epg, `tv_grab_*`) | Annahme aus Vorwissen; zu messen | – | 09, 15 |
| Zuordnung Sender zu EPG-Kanal in Jellyfin, TVHeadend, Threadfin, Dispatcharr, Channels DVR | Annahme aus Vorwissen (Quellcode nicht gelesen) | – | 09 |
| Serien-Duplikaterkennung und verschobene Sendungen bei TVHeadend, MythTV, Jellyfin, Channels DVR | Annahme aus Vorwissen | – | 09, 11 |
| Datenmengen (30 Sendungen je Sender und Tag, 800 Byte je Sendung) und Parse-Geschwindigkeit 20 bis 50 MB/s | Annahme; zu messen | – | 13, 17 |
| Zeitumstellung: Go löst Lücke und Doppelstunde bei `ParseInLocation` ohne Fehler auf | Annahme aus Vorwissen (R5 belegt `time.Date`, siehe 2.5) | – | 09 |
| SQLite WAL: Leser blockieren den Schreiber nicht; Checkpoint hängt an langen Lesern | Annahme aus Vorwissen | https://sqlite.org/wal.html | 13 |
| Magic Bytes für gzip, zip, xz, zstd; Go entpackt `gzip` transparent bei eigenem `Accept-Encoding` | Annahme aus Vorwissen | – | 09, 12 |
| Zeitzone je Quelle einstellbar (Standard UTC nach DTD), nie raten | Empfehlung (auf belegter DTD) | – | 09, 15 |
| Ersetzen je Kanal und Zeitfenster in einer Transaktion statt Upsert; Aufnahmen speichern Rohdaten statt Verweis auf Programmzeilen | Empfehlung | – | 07, 09, 11 |
| Zuordnungsstufen: `tvg-id`, dann exakter normalisierter Name (nur bei genau einem Treffer), Fuzzy nur als Vorschlag; manuelle Zuordnung am XMLTV-ID-Text | Empfehlung | – | 07, 09, 15 |
| Titel-Dictionary-FTS statt FTS über alle Sendungen | Empfehlung; zu messen | – | 07, 13 |
| Folgen-Dedup nur bei `xmltv_ns` oder `dd_progid`; sonst ehrlich «nicht identifizierbar» | Empfehlung | – | 09, 11 |
| Verschobene Sendung nur bei eindeutigem Kandidaten automatisch nachziehen | Empfehlung | – | 09, 11 |
| Bilder aus dem EPG nie direkt vom Client laden, nur über Server-Proxy; Sendungsbilder standardmässig aus | Empfehlung | – | 12, 14 |
| Importlimits (entpackt, Verhältnis, Feldgrössen), v1 mit XML, gzip, zip | Empfehlung (Zahlen sind Annahmen) | – | 12, 13 |

### 2.5 Timeshift und DVR (R5)

| Aussage | Einstufung | Quelle (URL) | verwendet in |
|---|---|---|---|
| TVHeadend-Timeshift: Puffer je Subscription, «Maximum size» über alle Puffer, RAM bevorzugt, Dateien zu 60 s, Pufferordner wird bei Start und Ende gelöscht | belegt (Quellcode; Nachprüfung Nr. 44) | https://github.com/tvheadend/tvheadend/blob/master/src/timeshift.c ; https://github.com/tvheadend/tvheadend/blob/master/src/timeshift/timeshift_filemgr.c | 10, 13 |
| TVHeadend-Muxer: Matroska deaktiviert Teletext und Daten; Pass-Profil schreibt Original-TS mit neu geschriebener PAT/PMT | belegt (Quellcode) | https://github.com/tvheadend/tvheadend/blob/master/src/muxer/muxer_mkv.c ; https://github.com/tvheadend/tvheadend/blob/master/src/muxer/muxer_pass.c | 11 |
| TVHeadend-DVR-Einstellungen: Vor- und Nachlauf, EPG-Änderungsfenster, Aufbewahrung, freier Platz (1000 MiB), Fehlerschwelle, Duplikate | belegt (Quellcode) | https://github.com/tvheadend/tvheadend/blob/master/src/dvr/dvr_config.c ; https://github.com/tvheadend/tvheadend/blob/master/src/dvr/dvr_autorec.c | 11, 15 |
| FFmpeg: Matroska-Mapping ohne Teletext; MP4-Tags ohne DVB-Subtitle und Teletext | belegt (Quellcode) | https://github.com/FFmpeg/FFmpeg/blob/master/libavformat/matroska.c ; https://github.com/FFmpeg/FFmpeg/blob/master/libavformat/movenc.c | 11 |
| Kodi puffert clientseitig ab Wiedergabestart | belegt (README) | https://github.com/kodi-pvr/pvr.iptvsimple/blob/Omega/README.md | 10 |
| Jellyfin kann bei Direct Play nicht zurückspulen, der Server puffert nicht | belegt (nur eine Forumaussage, schwache Quelle) | https://forum.jellyfin.org/t-can-t-live-tv-timeshift-using-the-roku-client?pid=15082 | 10 |
| Channels DVR nimmt als MPEG-TS (`.mpg`) auf; Jellyfin als `.ts` ohne Containerwahl | belegt (Forum und Doku) | https://community.getchannels.com/t/dvr-recording-filename/23908 ; https://jellyfin.org/docs/general/server/live-tv/post-process | 11 |
| Plex: `.ts`-Aufnahmen und Timeshift über Transcode-Puffer | Annahme (Support-Seite HTTP 403) | – | – |
| MPEG-TS: 188-Byte-Pakete, Continuity Counter, `discontinuity_indicator`, PTS 33 Bit bei 90 kHz (26,5 h) | belegt (laut Bericht, Norm ITU-T H.222.0); 33-Bit-Überlauf siehe Nachprüfung Nr. 29 (teilweise) | https://www.itu.int/rec/T-REC-H.222.0 | 10, 11 |
| `fsync` und `fdatasync`; Schreibfehler `EIO` und `ENOSPC` können erst bei `fsync` oder `close` sichtbar werden | belegt | https://man7.org/linux/man-pages/man2/fsync.2.html ; https://man7.org/linux/man-pages/man2/write.2.html | 11, 13 |
| Speicherort-Erkennung über `st_dev`, `statfs`-Typ; tmpfs-Grenze (Standard halber RAM); Reflink (`FICLONE`) nur auf unterstützenden Dateisystemen | belegt (Linux-Handbuchseiten) | https://man7.org/linux/man-pages/man2/stat.2.html ; https://man7.org/linux/man-pages/man2/statfs.2.html ; https://man7.org/linux/man-pages/man5/tmpfs.5.html ; https://man7.org/linux/man-pages/man2/ioctl_ficlone.2.html | 11, 13, 15 |
| Go `time.Date` mit `Location` normalisiert bei Zeitumstellung | belegt | https://pkg.go.dev/time#Date | 11 |
| Slouchcat hat `paths.DetectFS` (`statfs`) und einen eigenen `segmenter.go` | belegt (Repo) | `server/internal/paths/fstype_linux.go`, `server/internal/transcode/segmenter.go` | 04, 10, 11 |
| MythTV kennt harte und weiche Vor- und Nachläufe bei Konflikten (Stand 2006) | belegt (alt) | https://lists.mythtv.org/pipermail/mythtv-users/2006-March/125977.html | 11 |
| Speicherbedarf: bei 8 Mbit/s 3,6 GB je Stunde; 7,2 GB je Puffer von 2 h; 5 Zuschauer 36 GB (alle voll) oder 18,4 GB (gestaffelt) | Annahme (Rechnung; Bitraten zu messen) | – | 13, 10 |
| Gemeinsamer Segmentvorrat mit Zeigern ist technisch sparsam, aber fachlich ein gemeinsamer Puffer; verwerfen | Empfehlung (rechtliche Einschätzung ist Annahme, keine Rechtsauskunft) | – | 10, 12 |
| Puffer vor dem Transcode; Transcode über wachsender Quelle; Pause drosselt über Lesedruck, nach Frist beenden | Empfehlung | – | 10, 05 |
| Startpuffer gehört dem Verteiler (wenige Sekunden), nie dem Timeshift | Empfehlung | – | 10, 04 |
| Aufnahme als MPEG-TS der Programm-PIDs in Teilstücken, dazu Keyframe-Index; Lücken mit Grund in SQLite | Empfehlung | – | 11, 07 |
| Matroska und fMP4 verlieren Teletext, SCTE-35 und Daten-PIDs; fMP4 zusätzlich DVB-Subtitles | belegt (aus FFmpeg- und TVHeadend-Code) | siehe oben | 11 |
| `fdatasync` alle 5 bis 10 s; langsame Platte verwirft Pakete mit Lückenvermerk, statt den Verteiler zu blockieren | Empfehlung | – | 11, 13 |
| Hardlinks funktionieren auf SMB oft nicht; Zahl identischer Doppelaufnahmen im Haushalt unbekannt | Annahme; zu messen | – | 11 |
| Keine physische Deduplizierung in der ersten Stufe (D0 getrennte Dateien) | Empfehlung | – | 11, 13 |
| Planer: Regeln werden Termine im EPG-Horizont, UTC plus IANA-Zeitzone für lokale Regeln; Neuberechnung bei EPG-Import | Empfehlung | – | 06, 11 |
| Nur ein vom Administrator eingetragenes Verbindungslimit wäre ein harter Konflikt; Vorrang der Aufnahme (E-5) | Empfehlung (steht gegen LTV-017, siehe Abschnitt 6) | – | 11, 15 |
| Speicherorte: Markerdatei, Gerätenummer und Dateisystem-Typ vor jedem Teil; `openat` relativ zu einem Verzeichnis-Deskriptor | Empfehlung; Verhalten bei SMB-Abbruch zu messen | – | 11, 12, 15 |
| Neustartzeit des Transcodes an einer Pufferposition (QSV auf CT 190) gegen VoD-Seek | zu messen | – | 13, 17 |

### 2.6 Zusätzlich geprüft (nur in der Nachprüfung, nicht in R1 bis R5)

| Aussage | Einstufung | Quelle (URL) | verwendet in |
|---|---|---|---|
| Landlock: TCP `bind` und `connect` nur nach Port (ab ABI 4), keine Adressen; UDP regelt Landlock ab ABI 10; die Kernel-Versionsnummern (5.13, 6.7) stehen auf der Seite nicht | belegt (Nachprüfung Nr. 34); Versionsnummern nicht gelesen | https://docs.kernel.org/userspace-api/landlock.html | 04, 05, 12, 13, 17 |
| `go-landlock`: MIT, v0.11.2 vom 10.11.2026; `BestEffort()` fällt ohne Landlock still auf «keine Beschränkung» zurück und darf in der Hülle nicht verwendet werden (hart abbrechen, fail-closed); Multipath-TCP wird nicht eingeschränkt | belegt (Nachprüfung Nr. 35) | https://github.com/landlock-lsm/go-landlock ; https://pkg.go.dev/github.com/landlock-lsm/go-landlock/landlock | 04, 05, 12, 18 |
| FFmpeg `http.c` liest `http_proxy` und `no_proxy` aus der Umgebung; die Option `http_proxy` hat Vorrang; bei HTTPS wird der Proxy an die TLS-Schicht weitergereicht | belegt (Nachprüfung Nr. 25, Tag n7.1.4) | https://raw.githubusercontent.com/FFmpeg/FFmpeg/n7.1.4/libavformat/http.c | 04, 12 |
| FFmpeg MPEG-TS-Muxer: `mpegts_flags` (`resend_headers`, `pat_pmt_at_frames`, `initial_discontinuity`, `latm`, `omit_rai`), `pat_period` 100 ms, `sdt_period` 500 ms, `pcr_period` 20 ms | belegt (Nachprüfung Nr. 28, Tag n7.1.4) | https://raw.githubusercontent.com/FFmpeg/FFmpeg/n7.1.4/libavformat/mpegtsenc.c | 05, 06 |
| HLS: TS-Segment beginnt mit PAT und PMT (MUST); IDR am Anfang ist im RFC nur SHOULD, bei Apple MUST | teilweise (Nachprüfung Nr. 6) | https://www.rfc-editor.org/rfc/rfc8216.txt | 05, 16 |
| HLS: `EXT-X-DISCONTINUITY` ist bei Wechsel von Format, Spuren oder Zeitstempelfolge Pflicht; Apple verlangt `EXT-X-DISCONTINUITY-SEQUENCE` in Live-Listen, die einen Bruch enthalten können | belegt (Nachprüfung Nr. 4); ob `06`/`08` es für Safari setzen, ist zu prüfen | https://www.rfc-editor.org/rfc/rfc8216.txt | 05, 06, 08 |
| Go `compress/bzip2` bietet nur `NewReader` (nur Entpacken); gzip und zip nicht einzeln abgerufen | teilweise (Nachprüfung Nr. 33) | https://pkg.go.dev/compress/bzip2 | 09, 12 |
| FFmpeg-Filterdoku: `bwdif`, `deinterlace_qsv`, `vpp_qsv` in der Zusammenfassung der Seite nicht gefunden (Lücke der Zusammenfassung, kein Gegenbefund) | nicht prüfbar (Nachprüfung Nr. 30); Beleg bleibt M1 | https://ffmpeg.org/ffmpeg-filters.html | 04, 13, 16 |

## 3. Vergleichssysteme

Verlässlichkeit: **hoch** = Quellcode oder Projektseite abgerufen; **mittel** = Forum, README, Suchtreffer; **niedrig** = nur Vorwissen oder Annahme. Keines der Systeme wird eingebunden; sie liefern Muster. **Nachprüfung:** Jellyfin `SharedHttpStream` und `LiveStream` (Nr. 42, 43), der TVHeadend-Timeshift (Nr. 44) und Dispatcharr `buffer.py` und `config_helper.py` (Nr. 45) sind am Quellcode bestätigt. Nicht abgefragt wurden SRS, nginx-rtmp, Emby, Channels DVR, Plex, Enigma2, Kodi, MythTV, MediaMTX, go2rtc, ErsatzTV und Threadfin; deren Zeilen bleiben bei der Einstufung der Fassung 1.

| System | Mechanismus | Was Slouchcat daraus lernt | Was bewusst nicht übernommen wird | Verlässlichkeit |
|---|---|---|---|---|
| **Jellyfin** (Live TV) | `SharedHttpStream` kopiert die Quelle in eine unbegrenzte Temp-Datei, Abnehmer lesen eine lokale URL; Einstieg 20 000 Byte vor dem Dateiende; kein Reconnect; bei Direct Play kein Timeshift (Forum); Aufnahmen als `.ts` | Idee «Quelle einmal lesen, intern verteilen» ist richtig | Unbegrenzte Datei, Einstieg ohne Paket- und Keyframe-Ausrichtung, fehlender Reconnect | hoch (Quellcode) für Streaming; mittel (Forum) für Timeshift; niedrig (Vorwissen) für Re-Import und EPG-Zuordnung |
| **TVHeadend** | Timeshift je Subscription mit RAM- und Plattengrenze, Dateien zu 60 s, Ordner beim Start leeren; DVR mit Vor-/Nachlauf, EPG-Änderungsfenster, Aufbewahrung, Reserve; TS-Pass-Profil | Persönlicher Puffer mit harter Grenze je Zuschauer und global; Teilstücke; Aufräumen beim Start; Planerparameter | «Unbegrenzt»-Option (OOM-Berichte); Tuner-Logik; kein Einbinden als Dienst | hoch (Quellcode, R5); mittel (R1, Suchtreffer); niedrig für EPG-Zuordnung und Dedup (R4, R3) |
| **Threadfin / xTeVe** | Eigener Puffer entfernt, ffmpeg oder VLC als Kindprozess; Mapping über `tvg-id` oder Name; Backup-URLs | Nur als Warnung: ein reiner Prozessstarter verschiebt die Probleme zu ffmpeg | Codevorlage (Projekt wirkt seit über einem Jahr ruhig; letzter Release 11.09.2025) | mittel (README, GitHub-API); niedrig für Mapping und Ersatzquellen |
| **Dispatcharr** | Ein Upstream je Kanal, viele Clients; Redis-Ringpuffer mit Cursor je Client; neue Clients 5 s hinter live; Failover mit Zählern und Gnadenfristen; mehrere Streams je Kanal | Cursor je Abnehmer in einem begrenzten Ring; langsame Clients springen, statt den Ring aufzuhalten; Modell «Quellzeile n:1 Kanal»; Zähler und Gnadenfristen | Redis als zweiter Dienst; Einstieg an 1-MB-Grenzen und 5 s Rückstand; Python-Code (nur Referenz, obwohl AGPL-3.0 kompatibel wäre) | hoch (Quellcode, R1); niedrig für Re-Import, EPG-Zuordnung, Ersatzquellen (R3, R4) |
| **Channels DVR** | Tuner-Sharing; IPTV-Teilung laut Forum; Aufnahmen als MPEG-TS (`.mpg`); eigene Guide-Quelle | Aufnahme als unveränderter TS ist verbreitet | Nichts übernehmbar, Mechanismus nicht öffentlich dokumentiert | mittel (Forum); niedrig sonst |
| **Plex** | Serverseitiger Transcode-Puffer für Pause und Zurückspulen (nicht belegt) | Nichts belegt | Nicht als Vorbild verwenden | niedrig (Support-Seite HTTP 403) |
| **Emby** | Teilt nur, wenn der Kanal ein einziger MPEG-TS ist; sonst je Client über ffmpeg | Teilen auf TS-Byte-Ebene ist der typische Weg | – | mittel (Forum) |
| **MediaMTX** | Go-Server mit Protokollumsetzung (SRT, WebRTC, RTSP, RTMP, HLS, TS); Aufnahme als fMP4 oder TS; `sourceOnDemand`; MIT, v1.21.4 | Seine Go-Bibliotheken (gohlslib, gortsplib, mediacommon) sind Kandidaten im eigenen Prozess; als Testserver für RTSP und RTMP | Als Produktbaustein (eigener Server, widerspricht «ein Prozess») | hoch (README, GitHub-API) |
| **go2rtc** | Quellen- und Ausgangsvielfalt; `preload` für langsame Quellen; fMP4 über MSE statt HLS; MIT, v1.9.15 | Vorwärmen entspricht dem Vorladen von Nachbarsendern; MSE-Strom als Alternative zu HLS im Browser | Eigener Server; HLS-Kritik ist Meinung des Projekts | mittel (README) |
| **SRS** | `gop_cache` hält den letzten GOP für schnellen ersten Frame; ohne Grenze OOM-Risiko; MIT, v6.0-r3 | Burst ab letztem Keyframe, mit Grenze in Byte und GOPs | Eigener Server | hoch (Wiki, PR) |
| **nginx-rtmp** | `wait_key`, `wait_video`; Repo seit 12/2024 ohne Push | Ton nicht vor dem ersten Videobild senden | Eigener Server | niedrig (Sekundärquellen) |
| **ErsatzTV** | Erzeugt Kanäle aus einer Bibliothek; Zlib, v26.14.0 | Nichts belegt | Für IPTV-Empfang nicht relevant | niedrig (Segmenter nicht belegt) |
| **Kodi PVR** | Puffer clientseitig ab Wiedergabestart; bei HLS und DASH nur im Fenster des Anbieters | Bestätigt, dass Timeshift beim Zuschauer beginnt | Clientseitiger Puffer passt nicht zum Server-Modell | mittel (README) |
| **MythTV** | Harte und weiche Vor-/Nachläufe; Duplikate über `programid` | Dedup braucht anbieterstabile IDs | – | mittel (alte Mailingliste 2006); niedrig für EPG-Teil |
| **Enigma2** | Timeshift auf dem Empfänger, ein Zuschauer | – | Nicht übertragbar | niedrig |
| **Slouchcat Webradio-Relay** (ADR 0025) | Go liest, ffmpeg nur `pipe:0` nach `pipe:1`, Generationen bei Brüchen, Byte-Budget, tmpfs | Passendes Muster im eigenen Haus; Basis für den Kanal-Hub | Ton-Messwerte nicht auf Video übertragen | hoch (Repo) |

## 4. Geprüfte Bibliotheken und Abhängigkeiten

Die Berichte führen die Lizenzen für MIT, BSD-3-Clause, Zlib und Apache-2.0 als mit AGPL-3.0 verträglich; das ist **Annahme** (FSF-Liste nicht abgerufen). Gebündelte Bibliotheken behalten ihre Lizenztexte und Urheberhinweise. FFmpeg läuft als Unterprozess und ist keine Bindung.

| Name | Zweck | Lizenz | Letzter Stand laut Bericht | AGPL-3.0-Verträglichkeit | Wartung / Risiko | Empfehlung |
|---|---|---|---|---|---|---|
| `bluenviron/mediacommon` | Codec- und Containerhilfen (MPEG-TS, MP4, H.264/H.265) | MIT | Tag v2.10.0, Go ≥ 1.26 (Nachprüfung Nr. 37: Lizenz und Tag bestätigt; Push-Datum nicht ablesbar) | verträglich (Annahme) | aktiv; R3 hat sie nicht geprüft | offen (Kandidat für TS-Parsing im Prozess) |
| `bluenviron/gohlslib` | HLS-Client und -Muxer (TS, fMP4, Low-Latency) | MIT | Tag v2.4.8, Go ≥ 1.26 (Nachprüfung Nr. 36: bestätigt; Push-Datum nicht ablesbar) | verträglich (Annahme) | aktiv; Client liest nur einen Videotrack | offen (für LL-HLS später, nur wenn Messung es verlangt) |
| `bluenviron/gortsplib` | RTSP-Client und -Server | MIT | Go 1.26 und Modulpfad v5 bestätigt, Tag v5.7.0 nicht abgefragt (Nachprüfung Nr. 37) | verträglich (Annahme) | aktiv; Basis von MediaMTX | offen (nur wenn FFmpeg-Eingang nicht genügt) |
| `bluenviron/gortmplib` | RTMP | MIT | Tag v1.0.6, Push 26.11.2026 | verträglich (Annahme) | aktiv; von R3 nicht untersucht | offen |
| `asticode/go-astits` | MPEG-TS-Demux und -Mux (PAT, PMT, PES, SDT, EIT) | MIT | Tag v1.17.0 (Nachprüfung Nr. 38: bestätigt; Datum nicht ablesbar) | verträglich (Annahme) | README: «not yet production ready» | offen (Vorbehalt; eigener TS-Zähler als Alternative) |
| `landlock-lsm/go-landlock` | Sperre des FFmpeg-Prozesses (Weg W) | MIT | v0.11.2 vom 10.11.2026 (Nachprüfung Nr. 35) | verträglich (Annahme) | `BestEffort()` fällt ohne Landlock still auf «keine Beschränkung» zurück; Multipath-TCP wird nicht eingeschränkt | verwenden, nur fail-closed (ohne `BestEffort()`) |
| `yapingcat/gomedia` | TS, FLV, MP4 | MIT | Push 06.09.2024 | verträglich (Annahme) | ruhig | nicht verwenden |
| `grafov/m3u8` | HLS-Playlists (keine IPTV-Senderlisten) | BSD-3-Clause | archiviert seit 11.09.2025 | verträglich (Annahme) | archiviert | nicht verwenden |
| `Eyevinn/hls-m3u8` | Nachfolger von grafov | nicht geprüft | nicht geprüft | offen | unbekannt | offen |
| Eigener M3U-Parser (Senderlisten) | Streamendes Parsen mit Zeichensatz-Erkennung | – (eigener Code) | – | – | Aufwand rund 300 Zeilen (Schätzung R3); kleine Go-Parser existieren, Pflege unklar | verwenden (Empfehlung R3, Speicher zu messen) |
| `xqq/mpegts.js` (Web) | HTTP-TS und FLV im Browser über MSE | Apache-2.0 | v1.8.2 vom 14.08.2026 | verträglich (Annahme) | aktiv; kein MPEG-2, kein Seeking | offen (Alternative zu HLS; R2 bewertet Timeshift schwach) |
| `golang.org/x/net/html/charset` oder `golang.org/x/text/encoding/htmlindex` | Zeichensatz für XMLTV (`CharsetReader`) | im Bericht nicht genannt | nicht geprüft | Lizenz zu prüfen | – | offen (von R4 vorgeschlagen) |
| `tamerh/xml-stream-parser` | Schnellerer XML-Parser, falls `encoding/xml` zu langsam | im Bericht nicht genannt | nicht geprüft | zu prüfen | zusätzliche Abhängigkeit | nicht verwenden, bis Messung `encoding/xml` verwirft |
| xz-Bibliothek (z. B. `ulikunitz/xz`) | xz-komprimierte EPG-Dateien | im Bericht nicht genannt | nicht geprüft | zu prüfen | neue Abhängigkeit | nicht verwenden in der ersten Stufe (R4: gzip und zip) |
| `tellytv/go.xtream-codes` | Go-Client für Xtream | nicht geprüft | Alter nicht geprüft | zu prüfen | unklar | offen (nur als Lesevorlage genannt) |
| hls.js (1.7.3 gepinnt) | HLS im Browser | im Bericht nicht genannt (bereits im Repo) | v1.7.4 laut GitHub-API | im Repo bereits im Einsatz | aktiv; `light`-Build lässt wohl Funktionen weg | verwenden (vorhanden; Voll-Build zu prüfen) |
| Media3 / ExoPlayer (1.11.1 gepinnt) | Player in Android-Apps | im Bericht nicht genannt (bereits im Repo) | 1.12.0-rc01 laut GitHub-API | im Repo bereits im Einsatz | aktiv | verwenden (vorhanden) |
| FFmpeg 7.1.4-Jellyfin (Unterprozess) | Transcode, Remux, Deinterlace | GPL-Build (`--enable-gpl --enable-version3`, siehe Abschnitt 5) | im Image 1.41.3 gemessen | Unterprozess ist keine Bindung; Lizenzfolge des Builds laut R3 noch offen | Langläufer-Probleme (R1) | verwenden (vorhanden) |
| MediaMTX | Testserver für RTSP, RTMP, HLS | MIT | v1.21.4 vom 15.11.2026 | verträglich (Annahme) | sehr aktiv | nicht im Produkt; als Testwerkzeug offen |
| Dispatcharr, go2rtc, Threadfin, SRS, ErsatzTV | Nur Referenz | AGPL-3.0, MIT, MIT, MIT, Zlib | v0.34.1, v1.9.15, 1.2.37, v6.0-r3, v26.14.0 | – | – | nicht verwenden (kein Code übernommen) |
| `udpxy` | Multicast-zu-HTTP-Umsetzer, vom Nutzer selbst betrieben | nicht geprüft | nicht geprüft | – | Betriebsrisiko beim Nutzer | offen (nur als Hinweis in der Doku) |

## 5. Gemessen: M1 (FFmpeg im Image)

M1 stammt aus dem Image `ghcr.io/slouchcat/slouchcat:1.41.3` (laut Auftrag der Koordination). Das Dockerfile im Repo nennt `jellyfin-ffmpeg7 7.1.4-3` (R1 Q33); mit dem Image 1.44.1 wurde nicht nachgemessen.

**Version und Bau:** `ffmpeg version 7.1.4-Jellyfin`, gebaut mit gcc 14 (Debian 14.2.0-19). Auszug aus der Konfiguration:

| Bereich | Aktiviert |
|---|---|
| Lizenz | `--enable-gpl --enable-version3` (kein `--enable-nonfree` in der Konfiguration) |
| Hardware | `--enable-libvpl` (QSV), `--enable-vaapi`, `--enable-ffnvcodec --enable-cuda --enable-cuvid --enable-nvdec --enable-nvenc`, `--enable-opencl`, `--enable-vulkan --enable-libplacebo --enable-libshaderc`, `--enable-amf`, `--enable-libdrm` |
| Video | `libx264`, `libx265`, `libdav1d`, `libsvtav1`, `libvpx`, `libwebp`, `libtheora` |
| Audio | `libfdk-aac`, `libmp3lame`, `libopus`, `libvorbis`, `libopenmpt` |
| Untertitel und Text | `libzvbi` (Teletext), `libass`, `libfreetype`, `libfribidi`, `libfontconfig`, `libharfbuzz` |
| Netz | `--enable-gnutls`; **kein `libsrt`** |

**Protokolle (Eingang):** async, bluray, cache, concat, concatf, crypto, data, fd, ffrtmpcrypt, ffrtmphttp, file, ftp, gopher, gophers, hls, http, httpproxy, https, mmsh, mmst, pipe, rtmp, rtmpe, rtmps, rtmpt, rtmpte, rtmpts, rtp, srtp, subfile, tcp, tls, udp, udplite, unix, ipfs, ipns. **Kein `srt`.**

**Protokolle (Ausgang):** crypto, fd, ffrtmpcrypt, ffrtmphttp, file, ftp, gopher, gophers, http, httpproxy, https, icecast, md5, pipe, prompeg, rtmp, rtmpe, rtmps, rtmpt, rtmpte, rtmpts, rtp, srtp, tee, tcp, tls, udp, udplite, unix.

**Demuxer** (die Datei enthält nur eine Auswahl): dash, flv, hls, mov/mp4, mpegts, rtp, rtsp. **Muxer** (ebenfalls Auswahl): dash, fifo, hls, matroska, mp4, mpegts, segment, stream_segment, tee.

**Deinterlacer (Filter):** `bwdif`, `bwdif_cuda`, `bwdif_opencl`, `bwdif_vulkan`, `yadif`, `yadif_cuda`, `yadif_opencl`, `deinterlace_qsv`, `deinterlace_vaapi`, `estdif`, `w3fdif`, `nnedi`, `kerndeint`, `mcdeint`; dazu `idet` (Erkennung) und `il`.

**Bitstream-Filter** (Auszug): `h264_mp4toannexb`, `hevc_mp4toannexb`, `extract_extradata`, `setts`, `dts2pts`, `mpeg2_metadata`, `aac_adtstoasc`.

Was M1 für die Berichte klärt, und was nicht:

| Frage aus den Berichten | Befund aus M1 |
|---|---|
| R1: Gibt es SRT? | Nein. Weder in der Konfiguration noch in der Protokollliste. |
| R2: Hat der Build `libzvbi` für Teletext? | Ja, `--enable-libzvbi` ist aktiviert. Ob die Umwandlung nach WebVTT funktioniert, ist nicht getestet. |
| R2 und R1: Deinterlace auf CPU, CUDA und QSV? | `yadif`, `bwdif`, `yadif_cuda`, `bwdif_cuda`, `deinterlace_qsv`, `deinterlace_vaapi` sind vorhanden. Der QSV-Filter `vpp_qsv` (von R2 genannt) taucht in M1 nicht auf; die Auswahl ist darauf nicht ausgelegt, M1 klärt das nicht. |
| R3: Protokolle HTTP, HLS, DASH, RTSP, RTMP, UDP, RTP | Eingang vorhanden: `http(s)`, `hls`, `rtmp*`, `udp`, `rtp`, `crypto`, `tcp`, `tls`; Demuxer `dash`, `hls`, `rtsp`, `rtp`, `flv`, `mpegts`. |
| R3: Testbefehle mit `-f flv`, `-f rtsp`, `-f rtp_mpegts` | Die Muxer-Auswahl in M1 enthält diese nicht. Da die Liste eine Auswahl ist, ist nicht belegt, ob sie im Image fehlen. R3-Testbefehle laufen dort möglicherweise nicht; zu prüfen. |
| R3: `-protocol_whitelist` nötig? | M1 zeigt, dass `file`, `concat`, `concatf`, `subfile`, `ftp`, `gopher` und weitere im Image als Eingangsprotokolle vorhanden sind. |
| R3 Punkt 7: Lizenz des Builds | `--enable-gpl --enable-version3`; `libfdk-aac` aktiviert; die rechtliche Folge für die Auslieferung bleibt offen. |
| R1: FFmpeg 9.1 gegen 7.1.4 | Im Image läuft 7.1.4. Die Nachprüfung hat einzelne Werte am Tag n7.1.4 gelesen (Nr. 24 bis 28, z. B. `max_reload` 100); alles andere aus der aktuellen Doku bleibt Annahme. |
| Nachprüfung Nr. 30: Filterdoku | Die Zusammenfassung der FFmpeg-Filterseite zeigte `bwdif`, `deinterlace_qsv` und `vpp_qsv` nicht; das ist eine Lücke der Zusammenfassung. Beleg bleibt M1; Optionen vor dem Bau mit `ffmpeg -h filter=...` im Image lesen. |

## 6. Widersprüche zwischen den Berichten

Die Widersprüche stammen aus den Berichten R1 bis R5. Die Spalte «Stand» sagt, wie sie inzwischen behandelt sind: **entschieden durch Nachprüfung**, **entschieden in 04/05** (Fassung 4) oder in einem anderen Dokument, **offen**. Der Stand ist aus den Entscheidstellen von 04, 16 und 20 übernommen (Stichprobe).

| Nr | Thema | Aussage A | Aussage B | Stand |
|---|---|---|---|---|
| W-1 | Auslieferung an Android | R1: progressiv (HTTP-TS oder fMP4) ab Keyframe-Burst, am schnellsten | R2: HLS-fMP4 für alle; progressives TS ohne Live-Fenster, kein Timeshift im Player (Bewertung c1) | **Entschieden in 05** (Handy und TV: HLS mit TS-Segmenten; progressiv nicht verfolgt, ausser M-1 zeigt einen grossen Abstand; `16` Abschnitt «Entscheide», Nr. 1). «Progressiv ohne Live-Fenster» ist durch die Nachprüfung belegt (Nr. 18). |
| W-2 | Gemeinsame Stufe im Server | R1: gemeinsam nur der TS-Ring mit Cursor; Packager und Transcode je Zuschauer | R2: «ein HLS-fMP4-Ring pro (Sender, Profil)», «gemeinsamer Transcode je (Sender, Profil)» | **Entschieden in 04/05:** Direktweg aus dem Ring, FFmpeg je Zuschauer nur wenn nötig, kein gemeinsamer Transcode; Basisformat HLS-TS (`16` Nr. 2, T-11). |
| W-3 | Wer empfängt | R1: Go empfängt; ffmpeg kurzlebig | R3: ffmpeg als einziger Eingang | **Entschieden in 04** (T-3): Go liest (Weg G, L); FFmpeg nur in Weg W, und nur wenn M-10 ihn belegt. |
| W-4 | FFmpeg-Versionen | Doku der Berichte gilt für 9.1 («Gauss») | Im Image läuft 7.1.4 (M1, Dockerfile) | **Teils durch Nachprüfung, teils offen:** Einzelwerte sind am Tag n7.1.4 belegt (Nr. 24 bis 28, z. B. `max_reload` 100). Sonst gilt der Vorbehalt «für 7.1.4 nicht geprüft» an jeder FFmpeg-Aussage (04 RA-21); Rest über M-12. |
| W-5 | `DefaultLoadControl` | R1: 1000 / 2000 ms | R2: 50 000 / 50 000 / 2500 / 5000 ms | **Entschieden durch Nachprüfung** (Nr. 16): R2 widerlegt, Media3 1.11.1 hat Start 1000 ms, nach Rebuffer 2000 ms, Min- und Max-Puffer 50 000 ms. |
| W-6 | Startpunkt in hls.js | R1: `liveSyncDurationCount` 1 | R2: 2 (wie Radio); Doku warnt unter 3 | **Entschieden in 05** (Abschnitt 5.3): Startpunkt vom Server, Wert 2. Standard 3 und `lowLatencyMode` ab Werk an sind belegt (Nachprüfung Nr. 9). |
| W-7 | Segmentlänge und Quell-GOP | R1: ohne Neucodierung bestimmt der Quell-GOP den Segmentanfang | R2: Segmente 1 bis 2 s, «GOP gleich Segmentdauer», dabei Remux | **Offen:** Messung M-5. Dazu zeigt die Nachprüfung (Nr. 7): Apple verlangt mindestens 6 Segmente und IDR am Anfang; das Konzept weicht bewusst ab (`05` 4.3, `16` O-28). |
| W-8 | Lage und Format des Timeshift | R1: Spool je Zuschauer ab Cursor | R5: TS-Segmentfolge je Zuschauer vor dem Transcode; R2: Liste je Zuschauer mit fMP4 | **Entschieden in 10** (Fassung 2): Sitzungspuffer als Stückdateien aus dem Quell-TS (rund 60 s), vor der Umwandlung. Das TVHeadend-Vorbild ist belegt (Nachprüfung Nr. 44). |
| W-9 | Lokale und Multicast-Quellen | R3: «private Ziele erlauben», Standard an | Leitplanken: Freigabe nur ausdrücklich je Quelle | **Entschieden in 04/12:** lokale und Multicast-Ziele nur als ausdrücklich freigegebene feste Ziele je Quelle; Loopback, Link-Local und die eigenen Netze nie freigebbar. Der Standard «an» aus R3 gilt nicht. |
| W-10 | Verbindungslimit des Anbieters | R5: eingetragenes Limit als harter Konflikt; R1: Vorladen durch Limit begrenzt | R3: nicht Sache von Slouchcat; Auftrag LTV-017: kein Limit | **Offen:** `04` F-3, `20` E-28-02. Der Entwurf arbeitet ohne Limit. Entscheid bei Marco. |
| W-11 | Abruf von RFC 8216 | R5: abgerufen | R2, R3: nicht abgerufen | **Teils durch Nachprüfung entschieden:** Startregel (6.3.3), `EVENT`, `EXT-X-DISCONTINUITY`, `EXT-X-PROGRAM-DATE-TIME` sind belegt (Nr. 1, 3 bis 5). Die DRM-Teile (RFC, DASH-IF, Art. 39a URG) bleiben Annahme (N-13). |
| W-12 | mpegts.js im Web | R1: Alternative für rohes HTTP-TS | R2: Variante D, kein Timeshift, kein MPEG-2 | **Offen für mpegts.js:** keine ausdrückliche Entscheidung gefunden. Der progressive Weg ist mit W-1 nicht verfolgt. |
| W-13 | TVHeadend-Timeshift | R1: nur Suchtreffer, Annahme | R5: Quellcode gelesen | **Entschieden durch Nachprüfung** (Nr. 44): am Quellcode belegt. |

## 7. Noch nachzuprüfen

Tragende Aussagen, die nur aus Vorwissen, Forum, Suchtreffern oder Rechnung stammen und die auch die Nachprüfung nicht klären konnte. Die Nummern N-1 bis N-26 stammen aus Fassung 1; erledigte sind gestrichen, N-27 bis N-31 sind neu.

**Durch die Nachprüfung erledigt (gestrichen):** Media3-Ladepuffer-Standardwerte (N-6, Nr. 16); hls.js-Standardwerte (N-6, Nr. 9); Funktionsumfang des hls.js-Light-Baus (N-7, Nr. 11, 13); ManagedMediaSource auf dem iPhone ab 17.1 (Teil von N-3, Nr. 15); Whitelist `tcp`, `http_proxy`, Standardwerte von `reconnect*` und `max_reload` in FFmpeg 7.1.4 (Teil von N-8, Nr. 24 bis 27); Entity- und DTD-Verhalten von `encoding/xml` in Go 1.24 (Teil von N-14, Nr. 31); Lizenzen und neueste Tags von gohlslib, mediacommon, gortsplib, go-astits (Teil von N-21, Nr. 36 bis 38); RFC 8216 gelesen (W-11, Teil von N-13).

| Nr | Aussage | Bericht | Prüfweg |
|---|---|---|---|
| N-1 | Typischer Keyframe-Abstand realer Quellen (0,5 bis 10 s) | R1 | Messen aus dem GOP-Index an Marcos Quellen |
| N-2 | Teilzeiten und Zielwerte der Umschaltzeit | R1, R2 | Messreihen in hls.js (Chromium, Firefox), Safari, iPhone, ExoPlayer Handy und TV |
| N-3 | Safari und iPhone: natives LL-HLS, `getStartDate()`, Startverhalten, Autoplay und Senderwechsel in der PWA (Nachprüfung Abschnitt 5: nicht abgerufen) | R1, R2 | Apple-Doku lesen, auf iPhone messen (M-5) |
| N-4 | Decoder je Gerät: HEVC (8 und 10 Bit, HDR), MPEG-2, MP2, AC-3, E-AC-3, AAC-LATM; im Browser nur teilweise belegt (Nr. 21), MSE-Audio nicht prüfbar (Nr. 23) | R2 | `MediaCodecList` und `isTypeSupported` je Gerät |
| N-5 | Verhalten bei Interlaced (1080i50, 576i) je Browser und Gerät | R2 | Test mit Bewegungsfolge, Kammartefakte |
| N-6 | Media3: `allowChunklessPreparation` (Nr. 20), Standard-Live-Versatz bei HLS (Nr. 18), TS-Flags; hls.js: Wiederverwendung mit `loadSource` (Nr. 14) | R1, R2 | `HlsMediaSource`-Quellcode und Doku lesen; Messung auf Pixel und Android TV (M-3, M-4) |
| N-7 | CEA-608/708 am schlanken hls.js-Bau: nicht belegt, wahrscheinlich anders als in R2 angenommen (Nr. 12) | R2 | Am gebauten Light-Bau messen (M-3) |
| N-8 | FFmpeg 7.1.4: Option `timeout` am TCP-Handler (`tcp.c`), TLS-Zertifikatsprüfung (`tls*.c`), SAMPLE-AES mit `identity`, Signalbehandlung bei SIGTERM | R1, R3 | Quellcode und Test im Image |
| N-9 | Langläufer: Speicherwachstum und 33-Bit-Überlauf (zweiter Bruch bei rund 53 h) mit 7.1.4; Tickets #7876, #11629, #10500 nicht abgefragt (Trac hinter Anubis) | R1 | 24-bis-54-h-Lauf (M-22); Tickets nach Aufhebung der Sperre lesen |
| N-10 | Multicast im Docker-Bridge-Netz, Host-Netz, macvlan; IGMP-Snooping des Switches: nur Foren, offizielle Docker-Seiten schweigen (Nr. 41) | R3 | Messung auf Zielhardware (M-10) |
| N-11 | Xtream-Felder und Anbieterabweichungen; M3U-Dialekte | R3 | Gegen eigenen Mock und offene Client-Quellen (`pvr.iptvsimple`, Dispatcharr) |
| N-12 | Verhalten von Threadfin, Dispatcharr, Jellyfin, TVHeadend, Channels DVR, MythTV bei Re-Import, EPG-Zuordnung, Ersatzquellen, Serien-Dedup | R3, R4 | Quellcode lesen |
| N-13 | DRM-Regeln: DASH-IF-UUIDs, `KEYFORMAT` ungleich `identity`, Art. 39a URG | R3 | Primärquellen lesen; juristisch klären (keine Rechtsauskunft) |
| N-14 | `encoding/xml` mit Go 1.27; `time.ParseInLocation` in der Zeitumstellung; die fehlende Tiefengrenze in `Decoder.Token` (Nr. 32) im Bau absichern und testen | R4 | Testdatei, Test mit Go 1.27 |
| N-15 | SQLite im Treiber: Version (`contentless_delete` ab 3.43.0, Trigram), WAL-Verhalten, Importlast | R4 | Version prüfen, Messung mit 1 Mio Zeilen |
| N-16 | EPG-Datenmengen, Parse-Geschwindigkeit, Magic-Byte- und Gzip-Verhalten; XMLTV-Wiki (`dd_progid`, Encoding) und FTS5/WAL nicht abgefragt | R4 | Benchmark mit synthetischer Datei |
| N-17 | Speicherbedarf bei realen Bitraten; Platz-Prognose | R5 | Bitraten der Sender messen |
| N-18 | Container-Verhalten: tmpfs im cgroup-Limit, SMB-Abbruch, `fsync`, `fallocate`, Hardlinks und Reflink auf dem NAS | R5 | Test auf CT 190 und NAS |
| N-19 | Rechtliche Einschätzung gemeinsamer Vorrat und Deduplizierung (Eigengebrauch, Schweiz) | R5 | Juristische Klärung |
| N-20 | Plex, Channels DVR, Enigma2: interner Timeshift-Aufbau | R5 | Nicht öffentlich dokumentiert; nur Annahmen |
| N-21 | AGPL-3.0-Verträglichkeit nach FSF-Liste; Lizenzen von `golang.org/x`, `xml-stream-parser`, `Eyevinn/hls-m3u8`, xz-Bibliothek; Lizenzfolge des GPL-Builds von FFmpeg (MIT von gohlslib, mediacommon, gortsplib, go-astits, go-landlock ist bestätigt) | R1, R3, R4 | FSF-Liste und Lizenztexte lesen |
| N-22 | Pufferung und Timeouts hinter Nginx Proxy Manager (progressiver Strom, lange Anfragen, HTTP/2) | R1 | Messung hinter dem Proxy |
| N-23 | Teletext und DVB-Untertitel nach WebVTT (`libzvbi` ist im Build vorhanden); OCR-Laufzeit | R2 | Test mit Quelle, die Teletext liefert |
| N-24 | Wiederverwendung des Players beim Senderwechsel in ExoPlayer (`setMediaItem`, Decoder-Reuse) | R1, R2 | Messung auf Pixel und Android TV |
| N-25 | Neustartzeit des Transcodes an einer Pufferposition (QSV auf CT 190) | R5 | Messung gegen VoD-Seek aus Konzept 14 |
| N-26 | R3-Testbefehle (`-f flv`, `-f rtsp`, `-f rtp_mpegts`) im Image lauffähig | R3, M1 | Probelauf im Image |
| N-27 | Landlock-ABI des Zielkerns (CT 190) und die Kernel-Versionsnummern für ABI 4 und ABI 10 (auf der Kerneldoku-Seite nicht genannt) | Nachprüfung Nr. 34 | ABI auf CT 190 messen (M-25); Kernel-Änderungsprotokoll lesen |
| N-28 | Nicht abgefragt: `gortsplib` v5.7.0, `gortmplib` v1.0.6, `Eyevinn/hls-m3u8`, MediaMTX, go2rtc, ErsatzTV, Threadfin-Releases; Release-Daten der Bibliotheken (GitHub-Seiten zeigen keine Daten) | Nachprüfung Abschnitt 5 | Tags und Lizenzen abfragen |
| N-29 | Nicht abgefragt: SRS `gop_cache`, nginx-rtmp `wait_key`, Emby, Channels DVR (Forum), Plex, Enigma2, Kodi-README, MythTV-Liste | Nachprüfung Abschnitt 5 | Nur nötig, wenn das Konzept sich auf sie stützt |
| N-30 | FFmpeg-Filteroptionen `bwdif`, `deinterlace_qsv`, `vpp_qsv` (Doku nicht prüfbar, Nr. 30) | Nachprüfung Nr. 30 | `ffmpeg -h filter=...` im Image; Beleg bleibt M1 |
| N-31 | Apple-Abweichung: Live-Listen mit weniger als 6 Segmenten und kurze Segmente (Nr. 7) | Nachprüfung Nr. 7 | M-5 mit 3 und 6 Segmenten; Zusage für Safari und iPhone nicht möglich |

## 8. Quellenverzeichnis

«abgerufen: ja» heisst: laut Bericht am 28.11.2026 abgerufen. «nein» heisst: nicht abgerufen oder nur über Suchtreffer. Repo-Dateien sind gelesen und mit «ja (Repo)» geführt.

### 8.1 Stream-Sharing und Fast Zapping (R1)

| Kürzel | URL oder Pfad | abgerufen |
|---|---|---|
| Q1 | https://raw.githubusercontent.com/jellyfin/jellyfin/master/src/Jellyfin.LiveTv/TunerHosts/SharedHttpStream.cs | ja, 28.11.2026 |
| Q2 | https://raw.githubusercontent.com/jellyfin/jellyfin/master/src/Jellyfin.LiveTv/TunerHosts/LiveStream.cs | ja, 28.11.2026 |
| Q3 | https://emby.media/community/topic/145674-endless-transcode-temp-usage-while-watching-live-tv | ja, 28.11.2026 |
| Q4 | https://dispatcharr.github.io/Dispatcharr-Docs/ | ja, 28.11.2026 |
| Q5 | https://github.com/Dispatcharr/Dispatcharr/blob/main/apps/proxy/live_proxy/input/buffer.py | ja, 28.11.2026 |
| Q6 | https://github.com/Dispatcharr/Dispatcharr/blob/main/apps/proxy/live_proxy/config_helper.py | ja, 28.11.2026 |
| Q7 | https://dispatcharr.github.io/Dispatcharr-Docs/user-guide | ja, 28.11.2026 |
| Q8 | https://tvheadend.readthedocs.io/en/latest/webui/config_timeshift/ ; https://old.tvheadend.org/issues/5109 | nein (nur Suchtreffer) |
| Q9 | https://github.com/Threadfin/Threadfin | ja, 28.11.2026 |
| Q10 | https://community.getchannels.com/t/channels-reusing-stream-for-multiple-viewers/46603 ; https://getchannels.com/docs/apps/usage/streaming/ ; https://emby.media/community/topic/46113-multiple-streams-watching-the-same-m3u-channel | ja, 28.11.2026 |
| Q11 | https://github.com/bluenviron/mediamtx ; https://mediamtx.org/docs/features/proxy | ja, 28.11.2026 |
| Q12 | https://github.com/ossrs/srs/wiki/migrate_v4_EN_low-latency ; https://github.com/ossrs/srs/pull/3236 | ja, 28.11.2026 |
| Q12b | https://github.com/arut/nginx-rtmp-module/wiki/Directives | nein (Sekundärquellen) |
| Q13 | https://github.com/AlexxIT/go2rtc | ja, 28.11.2026 |
| Q14 | https://raw.githubusercontent.com/video-dev/hls.js/master/docs/API.md | ja, 28.11.2026 (Nachprüfung am Tag v1.7.3, Nr. 9, 10) |
| Q15 | https://github.com/androidx/media/blob/main/libraries/exoplayer/src/main/java/androidx/media3/exoplayer/DefaultLoadControl.java | ja, 28.11.2026 (Hauptzweig; in der Nachprüfung am Tag 1.11.1 gelesen, Nr. 16) |
| Q15a | https://developer.android.com/media/media3/exoplayer/live-streaming | ja, 28.11.2026 |
| Q16 | https://developer.android.com/media/media3/exoplayer/supported-formats | ja, 28.11.2026 |
| Q17 | https://datatracker.ietf.org/doc/html/draft-pantos-hls-rfc8216bis | ja, 28.11.2026 |
| Q18 | https://ffmpeg.org/ffmpeg-formats.html (Teil 1) | ja, 28.11.2026 |
| Q19 | https://ffmpeg.org/ffmpeg-formats.html (Teil 2) | ja, 28.11.2026 |
| Q20 | https://ffmpeg.org/download.html | ja, 28.11.2026 |
| Q21 | https://ffmpeg.org/ffmpeg.html | ja, 28.11.2026 |
| Q22 | https://github.com/bluenviron/gohlslib | ja, 28.11.2026 |
| Q23 | https://ffmpeg.org/ffmpeg-protocols.html | ja, 28.11.2026 |
| Q24 | https://trac.ffmpeg.org/ticket/7876 | nein (Suchtreffer; Trac hinter Anubis) |
| Q24b | https://ffmpeg.org/pipermail/ffmpeg-trac/2021-November/060313.html | ja, 28.11.2026 |
| Q25 | https://ffmpeg.org/pipermail/ffmpeg-devel/2019-May/243443.html | ja, 28.11.2026 |
| Q26 | https://ffmpeg.org/pipermail/ffmpeg-trac/2025-June/073873.html ; https://ffmpeg.org/pipermail/ffmpeg-trac/2023-August/066729.html | ja, 28.11.2026 |
| Q27 | https://trac.ffmpeg.org/ticket/7610 ; https://trac.ffmpeg.org/ticket/6369 | nein (Suchtreffer) |
| Q28 | https://nokia.com/bell-labs/publications-and-media/publications/channel-change-delay-in-iptv-systems | ja, 28.11.2026 |
| Q29 | https://en.wikipedia.org/wiki/Zap_time | ja, 28.11.2026 |
| Q30 | `docs/adr/0025-webradio-katalog-und-weiterleitung.md` | ja (Repo) |
| Q31 | `docs/perf/radio.md` | ja (Repo) |
| Q32 | `server/internal/netguard/netguard.go` | ja (Repo) |
| Q33 | `deploy/Dockerfile` ; `server/go.mod` | ja (Repo) |
| Q34 | GitHub-API `repos/<owner>/<repo>` und `releases/latest` bzw. `tags` (MediaMTX, go2rtc, Dispatcharr, Threadfin, ErsatzTV, hls.js, Media3, SRS, gohlslib, mediacommon, go-astits, gortsplib, gortmplib) | ja, 28.11.2026 |
| Q35 | `docs/perf/wiedergabestart.md` | ja (Repo) |
| Q36 | `docs/perf/K26-android-tv.md` | ja (Repo) |
| Q37 | https://github.com/xqq/mpegts.js | ja, 28.11.2026 |
| Q38 | `server/internal/transcode/session.go` | ja (Repo) |
| Q39 | https://github.com/bluenviron/mediacommon ; https://github.com/asticode/go-astits ; https://github.com/bluenviron/gortsplib | ja, 28.11.2026 |
| – | TVHeadend-Doku (Weiterleitung ohne Inhalt), FFmpeg-Trac (Anubis), Apple-LL-HLS-Seite (nur Titel) | nein (nicht abrufbar) |

### 8.2 Client-Kompatibilität (R2)

| URL oder Pfad | abgerufen |
|---|---|
| https://developer.android.com/media/media3/exoplayer/live-streaming | ja, 28.11.2026 |
| https://developer.android.com/media/media3/exoplayer/supported-formats | ja, 28.11.2026 |
| https://github.com/video-dev/hls.js/blob/master/docs/API.md (Rohtext von raw.githubusercontent.com) | ja, 28.11.2026 |
| https://developer.mozilla.org/en-US/docs/Web/API/ManagedMediaSource | ja, 28.11.2026 |
| https://github.com/xqq/mpegts.js | ja, 28.11.2026 |
| `web/src/features/player/capabilities.ts`, `web/src/app/radio/engine.ts`, `web/src/features/player/VideoPlayer.tsx`, `web/package.json`, `android/gradle/libs.versions.toml`, `android/core/player/.../Media3Engine.kt` | ja (Repo) |
| https://www.rfc-editor.org/rfc/rfc8216 | nein (R2); ja in der Nachprüfung 28.11.2026 (Nr. 1, 3 bis 5) |
| https://developer.apple.com/documentation/http-live-streaming/hls-authoring-specification-for-apple-devices | nein (R2); ja in der Nachprüfung 28.11.2026 (Nr. 7, 8) |
| https://developer.android.com/reference/androidx/media3/exoplayer/DefaultLoadControl | nein |
| https://developer.android.com/reference/androidx/media3/exoplayer/hls/HlsMediaSource.Factory | nein |
| https://github.com/androidx/media/blob/release/libraries/extractor/src/main/java/androidx/media3/extractor/ts/DefaultTsPayloadReaderFactory.java | nein (R2); ja in der Nachprüfung 28.11.2026, Tag 1.11.1 (Nr. 19) |
| https://github.com/xqq/mpegts.js/blob/master/docs/api.md | nein |
| https://developer.apple.com/documentation/webkitjs/htmlmediaelement/1630011-getstartdate | nein |
| https://developer.amazon.com/docs/fire-tv/ | nein |

Im Text genannt, nicht in der Quellenliste: RFC 7742 und RFC 7874 (WebRTC-Codecs, [Beleg*]), caniuse (nicht abgerufen).

### 8.3 Eingangsquellen und Protokolle (R3)

| URL | abgerufen |
|---|---|
| https://ffmpeg.org/ffmpeg-protocols.html | ja, 28.11.2026 |
| https://ffmpeg.org/ffmpeg-formats.html (erste 100'000 Zeichen) | ja, 28.11.2026 |
| https://github.com/bluenviron/gohlslib | ja, 28.11.2026 |
| https://github.com/bluenviron/gortsplib | ja, 28.11.2026 |
| https://github.com/asticode/go-astits | ja, 28.11.2026 |
| https://github.com/grafov/m3u8 | ja, 28.11.2026 |
| https://datatracker.ietf.org/doc/html/rfc8216 | nein |
| https://dashif.org/identifiers/content_protection/ | nein |
| https://docs.docker.com/engine/network/drivers/host/ | nein (R3); ja in der Nachprüfung 28.11.2026 (Nr. 41) |
| https://docs.docker.com/engine/network/drivers/macvlan/ | nein (R3); ja in der Nachprüfung 28.11.2026 (Nr. 41) |
| https://github.com/bluenviron/mediamtx | nein |
| https://github.com/bluenviron/mediacommon | nein (R3); ja in der Nachprüfung 28.11.2026 (Nr. 37) |
| https://github.com/Eyevinn/hls-m3u8 | nein |
| https://github.com/Dispatcharr/Dispatcharr | nein |
| https://github.com/Threadfin/Threadfin | nein |
| https://github.com/xteve-project/xTeVe | nein |
| https://github.com/jellyfin/jellyfin | nein |
| https://getchannels.com/ | nein |
| https://github.com/kodi-pvr/pvr.iptvsimple | nein |
| https://www.gnu.org/licenses/license-list.html | nein |
| https://developer.apple.com/streaming/examples/ | nein |
| https://bitmovin.com/demos/stream-test | nein |
| https://dash.akamaized.net/ | nein |
| https://peach.blender.org/ | nein |

Im Text genannt, nicht in der Quellenliste (nein): https://github.com/iptv-org/iptv, https://github.com/tellytv/go.xtream-codes, https://github.com/pcherenkov/udpxy, https://reference.dashif.org/dash.js/, https://durian.blender.org/, https://mango.blender.org/.

### 8.4 EPG und XMLTV (R4)

| URL oder Pfad | abgerufen |
|---|---|
| https://raw.githubusercontent.com/XMLTV/xmltv/master/xmltv.dtd | ja, 28.11.2026 (Zusammenfassung des Abruf-Werkzeugs) |
| https://wiki.xmltv.org/index.php/XMLTVFormat | ja, 28.11.2026 |
| https://pkg.go.dev/encoding/xml | ja, 28.11.2026 |
| https://sqlite.org/fts5.html | ja, 28.11.2026 (100'000 von rund 172'000 Zeichen) |
| `server/migrations/00015_discover.sql`, `server/internal/store/dump.go`, `docs/planning/09-hintergrundjobs.md`, `CLAUDE.md` | ja (Repo) |
| https://sqlite.org/wal.html | nein |
| Go-Release-Notes (CVE-2022-30633) | nein |
| RFC 1952 (gzip) | nein |
| Quellcode von Threadfin, Dispatcharr, Jellyfin, TVHeadend, MythTV, Channels DVR | nein |

### 8.5 Timeshift und DVR (R5)

| Nr | URL oder Pfad | abgerufen |
|---|---|---|
| 1 | https://github.com/tvheadend/tvheadend/blob/master/src/timeshift.c | ja, 28.11.2026 |
| 2 | https://github.com/tvheadend/tvheadend/blob/master/src/timeshift/timeshift_filemgr.c ; https://github.com/tvheadend/tvheadend/blob/master/src/timeshift/private.h | ja, 28.11.2026 |
| 3 | https://github.com/tvheadend/tvheadend/blob/master/src/muxer/muxer_mkv.c ; https://github.com/tvheadend/tvheadend/blob/master/src/muxer/muxer_pass.c ; https://github.com/tvheadend/tvheadend/blob/master/src/profile.c | ja, 28.11.2026 |
| 4 | https://github.com/FFmpeg/FFmpeg/blob/master/libavformat/matroska.c | ja, 28.11.2026 |
| 5 | https://github.com/FFmpeg/FFmpeg/blob/master/libavformat/movenc.c | ja, 28.11.2026 |
| 6 | https://www.rfc-editor.org/rfc/rfc8216 | ja laut R5, 28.11.2026; in der Nachprüfung gelesen (Nr. 1, 3 bis 5; siehe W-11) |
| 7 | https://github.com/kodi-pvr/pvr.iptvsimple/blob/Omega/README.md | ja, 28.11.2026 |
| 8 | https://jellyfin.org/docs/general/server/live-tv/post-process | ja, 28.11.2026 |
| 9 | https://community.getchannels.com/t/dvr-recording-filename/23908 | ja, 28.11.2026 |
| 10 | https://forum.jellyfin.org/t-can-t-live-tv-timeshift-using-the-roku-client?pid=15082 | ja, 28.11.2026 |
| 11 | https://github.com/tvheadend/tvheadend/blob/master/src/dvr/dvr_config.c | ja, 28.11.2026 |
| 12 | https://github.com/tvheadend/tvheadend/blob/master/src/dvr/dvr_autorec.c | ja, 28.11.2026 |
| 13 | https://www.itu.int/rec/T-REC-H.222.0 | ja laut R5, 28.11.2026 |
| 14 | https://forum.jellyfin.org/t-any-way-to-remux-livetv-encoding-from-ts-to-mkv-in-realtime-for-dvr | ja, 28.11.2026 |
| 15 | https://man7.org/linux/man-pages/man2/fsync.2.html | ja, 28.11.2026 |
| 16 | https://ffmpeg.org/ffmpeg-formats.html#concat-1 | ja, 28.11.2026 |
| 17 | https://pkg.go.dev/time#Date | ja, 28.11.2026 |
| 18 | https://lists.mythtv.org/pipermail/mythtv-users/2006-March/125977.html | ja, 28.11.2026 |
| 19 | https://man7.org/linux/man-pages/man2/ioctl_ficlone.2.html | ja, 28.11.2026 |
| 20 | https://man7.org/linux/man-pages/man2/stat.2.html | ja, 28.11.2026 |
| 21 | https://man7.org/linux/man-pages/man2/statfs.2.html | ja, 28.11.2026 |
| 22 | https://man7.org/linux/man-pages/man2/write.2.html | ja, 28.11.2026 |
| 23 | https://man7.org/linux/man-pages/man5/tmpfs.5.html | ja, 28.11.2026 |
| Repo | `server/internal/paths/fstype_linux.go`, `server/internal/transcode/segmenter.go` | ja (Repo) |
| – | Plex-Support «DVR Settings» (HTTP 403); docs.tvheadend.org (404) | nein (nicht abrufbar) |

### 8.6 Messung (M1)

| Datei | Inhalt |
|---|---|
| `.omc/research/live-tv/M1-ffmpeg-faehigkeiten.txt` | FFmpeg 7.1.4-Jellyfin im Image 1.41.3: Konfiguration, Protokolle, Demuxer, Muxer, Deinterlacer, Bitstream-Filter (Abschnitt 5) |

### 8.7 Quellen der Nachprüfung (28.11.2026, `anhang/nachpruefung-quellen.md`)

Alle laut Nachprüfung am 28.11.2026 abgerufen; lange Seiten nur in den ersten 100'000 Zeichen.

| Thema | URL | Nr. |
|---|---|---|
| HLS | https://www.rfc-editor.org/rfc/rfc8216.txt ; https://datatracker.ietf.org/doc/html/draft-pantos-hls-rfc8216bis ; Apple-Spezifikation (siehe 8.2) | 1 bis 8 |
| hls.js v1.7.3 | https://raw.githubusercontent.com/video-dev/hls.js/v1.7.3/docs/API.md ; .../v1.7.3/README.md ; .../v1.7.3/build-config.js ; `src/demux/tsdemuxer.ts` | 9 bis 14 |
| iPhone und Browser | https://webkit.org/blog/14735/webkit-features-in-safari-17-1/ ; https://developer.mozilla.org/en-US/docs/Web/API/ManagedMediaSource ; https://developer.mozilla.org/en-US/docs/Web/Media/Guides/Formats/Video_codecs ; MDN Audio_codecs ; https://caniuse.com/hevc | 15, 21 bis 23 |
| Media3 1.11.1 | https://raw.githubusercontent.com/androidx/media/1.11.1/libraries/exoplayer/src/main/java/androidx/media3/exoplayer/DefaultLoadControl.java ; https://raw.githubusercontent.com/androidx/media/1.11.1/libraries/extractor/src/main/java/androidx/media3/extractor/ts/DefaultTsPayloadReaderFactory.java ; supported-formats und live-streaming (siehe 8.1) | 16 bis 19 |
| FFmpeg n7.1.4 | https://raw.githubusercontent.com/FFmpeg/FFmpeg/n7.1.4/libavformat/http.c ; `avio.c` ; `hls.c` ; `mpegtsenc.c` ; https://ffmpeg.org/ffmpeg-protocols.html ; https://ffmpeg.org/ffmpeg-filters.html (nicht aussagekräftig) | 24 bis 30 |
| Go 1.24.0 | https://raw.githubusercontent.com/golang/go/go1.24.0/src/encoding/xml/xml.go ; `read.go` ; https://pkg.go.dev/encoding/xml ; https://pkg.go.dev/compress/bzip2 | 31 bis 33 |
| Landlock | https://docs.kernel.org/userspace-api/landlock.html ; https://github.com/landlock-lsm/go-landlock ; https://pkg.go.dev/github.com/landlock-lsm/go-landlock/landlock | 34, 35 |
| Go-Bibliotheken | https://github.com/bluenviron/gohlslib ; https://github.com/bluenviron/mediacommon ; https://github.com/bluenviron/gortsplib ; https://github.com/asticode/go-astits (GitHub-API `tags`) | 36 bis 38 |
| XMLTV und Docker | https://raw.githubusercontent.com/XMLTV/xmltv/master/xmltv.dtd ; https://docs.docker.com/engine/network/drivers/host/ ; .../macvlan/ | 39 bis 41 |
| Vergleichssysteme | Jellyfin `SharedHttpStream.cs` und `LiveStream.cs` ; TVHeadend `timeshift.c`, `timeshift_filemgr.c`, `private.h` ; Dispatcharr `buffer.py`, `config_helper.py` (URLs wie in 8.1 und 8.5) | 42 bis 45 |

