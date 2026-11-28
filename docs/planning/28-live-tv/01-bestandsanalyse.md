# Konzept 28 · Bestandsanalyse: was es für Live-TV heute gibt

Stand: 28. November 2026 · Fassung 1 · Status: Entwurf, nicht freigegeben

> **Kurz gesagt.** Live-TV, Programmzeitschrift (EPG), Video-Zeitversatz und Aufnahmen gibt es nicht; Live-TV ist
> Nicht-Ziel. Tragfähig ist viel: Webradio-Relay (ein Empfang, viele Zuhörer, Zeitversatz, Ersatzquellen),
> Hardware-Transcoding, Konten und Profile mit Rechteschaltern, Jobs, Benachrichtigungen und in allen Clients eine
> Vorlage für «Sender». Es fehlen: Sitzungsart für Kanäle, Deinterlacing, Live-Modus im Video-Player, Zeitraster, Aufnahmeort.
> Die fünf grössten Zwänge:
> 1. **Regelwerk:** Live-TV und DVR sind Nicht-Ziel (`CLAUDE.md:75`, Spec `:56`); die Ausnahme steht vor dem Bau.
> 2. **Netz:** ffmpeg bekommt nie eine Netzadresse; Heimnetz-Quellen und Multicast kollidieren mit `netguard`, ADR 0025 F-15 und Docker-Bridge.
> 3. **Video-Kette:** auf Dateien und VOD gebaut (Plan, Seek, Pause, `ENDLIST`); ein Live-Zweig gefährdet die Nähte; Zeilensprung fehlt.
> 4. **Datenmodell:** Sitzungen, Grenzen, Merkliste, Fortschritt, Suche hängen an `items`; Sender und Aufnahmen brauchen eigene Tabellen.
> 5. **Dauerläufer:** Jobs (Takt 60 s, Pause) taugen nicht für Aufnahmen; Updater, Shutdown (10 s), Platzprüfung kennen sie nicht; `/media` ist `:ro`.
>
> Dies ist der Ist-Stand ohne Lösung; die steht in `04-architektur.md` und `05-streaming-und-fast-zapping.md`.
> `05-streaming-und-fast-zapping.md`.

## 1. Ausgangslage

### 1.1 Stand des Repos

| Grösse | Wert | Beleg |
|---|---|---|
| Commit | `1220769b` (= `origin/main`) | Arbeitsbaum `claude/konzept-live-tv` |
| Server | 1.44.1 | `deploy/compose.yaml:23` |
| Vertrag (`api/openapi.yaml`) | 1.44.0 (der Patch 1.44.1 hat den Vertrag nicht geändert) | `api/openapi.yaml:4`; `docs/dev/CURRENT_STATUS.md:17-19` |
| Letzte Migration | 00086 | `server/migrations/00086_wishes_rights_follow_rating.sql` |
| Letzte ADR / letzte Frage | 0045 / 36 | `docs/adr/`; `docs/planning/04-entscheidungen.md:142` |
| Nächste freie Nummern | Frage 37, ADR 0046, Migration 00087, Vertrag 1.45.0 (die letzten beiden vergibt erst der Bau) | Prüfung V1 |

Die Konzept-Nummer 28 ist durch den Auftrag selbst belegt (`docs/planning/28-live-tv/anhang/auftrag-2026-11-28.md`);
Frage 37, ADR 0046, Migration 00087 und Vertrag 1.45.0 sind frei (V1, Korrektur 2).

### 1.2 Live-TV ist bisher Nicht-Ziel

Ein **Nicht-Ziel** ist etwas, das die Spec ausdrücklich ausschliesst; ein Agent darf es nicht bauen, bevor Marco es im
Entscheidungsprotokoll aufhebt. In der Doku kommen `IPTV`, `EPG` und `XMLTV` nirgends vor; im Code heisst «DVR» nur
der Radarr/Sonarr-Server der Seerr-Testattrappe (`server/internal/seerr/seerrtest/seerrtest.go:180-181`), das ist
kein Bezug (A1 §1).

**Verbindliche Stellen, die sich mit der Freigabe ändern müssen** (Änderung erst in Stufe 0 des Baus, nicht mit
diesem Papier):

| Nr. | Stelle | Inhalt |
|---|---|---|
| S-1 | `CLAUDE.md:75` (Regel 11) | «… Musik (ausser Live-Webradio, Frage 23), Fotos, Live-TV, TV-Apps …» |
| S-2 | `docs/spec/01-anforderungen-v1.md:56-57` | «Musik, Fotos, Live-TV und DVR (spätere Versionen)»; der Halbsatz «DVR bleiben Nicht-Ziele» widerspräche einer zweiten Ausnahme |
| S-3 | `docs/spec/01-anforderungen-v1.md:44` | «Architektur hält Musik, Fotos und Live-TV offen, baut sie aber nicht» |
| S-4 | `docs/spec/01-anforderungen-v1.md:29` | «Musik, Fotos, Live-TV zurückgestellt (Runde 4)» |
| S-5 | `docs/planning/03-plan-v1.md:30` | «Nicht eingeplant: Musik, Fotos, Live-TV/DVR …» (nennt die Webradio-Ausnahme gar nicht: Altlast) |
| S-6 | `docs/dev/HANDOVER.md:155` | «Nicht-Ziele: Musik, Fotos, Live-TV, native Apps, Plugins …» («native Apps» ist seit Frage 34/35 falsch) |

**Geschichtliche Stellen** (werden nicht umgeschrieben, nur abgelöst): Spec `:188` und `:286` (Interview Runde 4);
`docs/planning/01-vision.md:70`; `docs/planning/04-entscheidungen.md:90` (Frage 23: «Musikbibliothek, Fotos, Live-TV
und DVR bleiben Nicht-Ziele»); `docs/planning/16-webradio.md:52`, `:65`, `:693` (keine Radio-Aufnahmen; Zeitversatz
ist «kein DVR»); `docs/planning/13-abspielseite.md:62-63`, `:81`, `:205` («Läuft gerade» und «TV Guide» gehören
zu Live-TV); `docs/planning/25-medienwuensche/11-entscheide-und-adr.md:960`; ADR 0025 (`:3-4`, `:146`).

**Nicht-Ziele, die bleiben:** Fire TV und andere TV-Apps ausser Android TV (`01-anforderungen-v1.md:58-60`; K26 E-1),
natives Installationspaket `.deb` (`:41`, `:86`), YouTube-Downloads (`:90`, Regel 11), Mandantenfähigkeit (`:85`),
Freigabe-Anfragen von Kinderprofilen an den Hauptbenutzer (`:67`).

### 1.3 Der Auftrag wurde gegen ein altes Repo geschrieben

Der Auftrag setzt NestJS, PostgreSQL, Drizzle, Redis, BullMQ, ein `.deb`-Paket, `DEC-001`, ein Glossar
`docs/spec/00_Glossar_und_Entscheidungen.md` und die Marken [MVP]/[1.1]/[1.x]/[Later] voraus. Davon gibt es hier
nichts: `docs/spec/` enthält nur `01-anforderungen-v1.md` und `README.md`; eine Suche nach `DEC-0`, `NestJS`,
`BullMQ`, `[MVP]` in `docs/` und `CLAUDE.md` findet nichts (A1 §0). Massgeblich ist der Go-Stack: Go-Server, SQLite
(`docs/adr/0001-stack.md:32`), eigene Job-Queue in SQLite (`:35`, ADR 0002, ADR 0017), ein Container
(`docs/planning/03-plan-v1.md:39`). Marco hat am 28. November 2026 entschieden, dass das Konzept für das aktuelle
Go-Produkt gilt; die 21 Grundsatzentscheide LTV-001 bis LTV-021 tragen als fachliche Vorgaben hinüber.

**Lesart dieses Papiers:** «gebaut» = im Code und im Vertrag vorhanden; «teilweise gebaut» = ein Teil vorhanden;
«nur spezifiziert» = in Spec oder Konzept beschrieben, nicht im Code; «fehlt» = weder noch. Zeilenangaben beziehen
sich auf `1220769b`. Bei Widerspruch zwischen den Berichten A1–A4 und der unabhängigen Prüfung V1 gilt V1.

## 2. Übersicht

«Für Live-TV» heisst: **wiederverwenden** (ohne Umbau nutzbar), **erweitern** (vorhanden, braucht Änderung) oder
**neu** (gibt es nicht).

| # | Bereich | Zustand | Beleg (Auswahl) | Für Live-TV |
|---|---|---|---|---|
| 1 | Wiedergabe-Entscheid (`Decide`) | gebaut (Dateien) | `server/internal/playback/decide.go:318-326`, `:386` | erweitern |
| 2 | Wiedergabesitzungen | teilweise gebaut | `server/migrations/00032_playback_prefs.sql:63` (`item_id NOT NULL`) | neu (Sitzungsart für Kanäle) |
| 3 | Grenzen je Konto/Profil | teilweise gebaut (gebaut für Titel, nicht für Sender) | `playback/limits.go:140-200`; `00063_…:39-41` | erweitern |
| 4 | Transcoding und Hardware | gebaut (VOD); Live-Video fehlt | `transcode/plan.go:32-43`; `transcode/backend.go:140-223` | erweitern; Live-Session neu |
| 5 | Deinterlacing | fehlt im Code (Filter im Image vorhanden) | Suche `yadif\|bwdif` in `server/internal/` ohne Treffer; M1 | neu |
| 6 | FFmpeg im Image (gemessen, M1) | gebaut | `deploy/Dockerfile:69-102`; M1 | wiederverwenden |
| 7 | Webradio-Relay | gebaut (nur Ton) | `radio/relay.go:19-61`; `radio/buffer.go:15-70` | wiederverwenden als Muster; Video neu |
| 8 | Ausgehende Verbindungen, `netguard` | gebaut (Go-HTTP); für ffmpeg-Eingänge fehlt er | `netguard/netguard.go:29-128` | erweitern |
| 9 | Geheimnisse und Redaction | teilweise gebaut | `secrets/secrets.go:115-141`; `logging/logging.go:63-75` | erweitern |
| 10 | Pfade und Speicher | gebaut (lesend, Cache, Podcasts); Aufnahmeort fehlt | `paths/paths.go:15-48`; `deploy/compose.yaml:36` | erweitern |
| 11 | Compose, Betrieb, Updater, Shutdown | gebaut; Netz und Dauerläufer fehlen | `deploy/compose.yaml:16-40`; `cmd/slouchcat/main.go:656-664` | erweitern |
| 12 | Konten, Profile, Rechte | gebaut; Live-Schalter fehlen | `00067_wishes_rights.sql:22-24`; `auth/profiles.go:40-45` | erweitern (Muster `can_request`) |
| 13 | Sichtbarkeit | gebaut (nur `items`) | `visibility/visibility.go:1-14`; `sqlc.yaml:40-73` | neu (eigenes Paket nach Muster `wishq`) |
| 14 | Datenbank | gebaut | `store/store.go:94-118` | erweitern (EPG-Grösse) |
| 15 | Jobs | gebaut; für Aufnahmen ungeeignet | `jobs/scheduler.go:14-64`; `00027_jobs_scheduler.sql:52-54` | wiederverwenden für Import, Abgleich, Aufräumen |
| 16 | Benachrichtigungen | gebaut; neue Arten brauchen Tabellenneubau | `model/notify.go:42-57`; `00070_wishes_notifications.sql:32-43` | erweitern |
| 17 | API-Konventionen, Segment-Token | gebaut | `api/api.go:25-60`, `:200-212`; `api/stream_token.go:24-43` | wiederverwenden; Kanal-Token neu |
| 18 | Verwaltung und Systemwerte | teilweise gebaut | `admin/dashboard.go:18-21`; `admin/streams.go:21-32` | erweitern; CPU/RAM/GPU-Last neu |
| 19 | Suche, Merkliste, Fortschritt, Tageslimit | gebaut (an `items`) | `00054_watchlist_follow.sql:23-29`; `playback/limits.go:251-273` | nur als Muster; eigene Tabellen neu |
| 20 | Einrichtung | gebaut | `admin/setup_wizard.go:93-160` | erweitern (zusätzlicher Schritt) |
| 21 | Web-App: Navigation, Bauteile, Player | gebaut; Live-Modus Video und Zeitraster fehlen | `web/src/ui/` (125 Bauteile); `VideoPlayer.tsx:735-792` | wiederverwenden, erweitern, neu |
| 22 | Web-App: Service Worker | gebaut | `web/vite.config.ts:123-160` | erweitern (Logo-Cache) |
| 23 | Android-Handy-App | gebaut; Live-Video-Steuerung fehlt | `android/app-mobile/…/shell/Registry.kt:36-84` | erweitern |
| 24 | Android-TV-App (Tasten) | teilweise gebaut | `…/app/tv/ui/TvRemoteKeys.kt:39-46` | erweitern |
| 25 | Mockup-Baukästen | gebaut; Daten und Bilder für Live-TV fehlen | `docs/planning/mockups/*/BAUKASTEN.md` | wiederverwenden |
| 26 | Tests | gebaut; Lücke bei TV-Seiten | `CLAUDE.md` Regel 8; `web/e2e/keyboard.ts` | erweitern |
| 27 | Live-TV, EPG, Aufnahmen selbst | fehlt (Spec und Konzept: nur dieses Konzept) | keine Spur in `web/src`, Locales oder Vertrag | neu |

## 3. Die Bereiche im Einzelnen

### 3.1 Wiedergabe-Entscheid

`Decide(file, probe, req)` ist eine reine Funktion (`server/internal/playback/decide.go:318-326`). Sie wählt Direct
Play (Datei unverändert), Remux (neu verpackt, nicht neu kodiert) oder Transcode (neu kodiert) nach Container,
Codec, Ton, Untertiteln, HDR, Höhe und Bitrate (`:383-461`, Gründe als stabile Codes `:22-48`).

- **Remux nur für H.264 und HEVC** (`decide.go:386`; `transcode/probe.go:382-388` kennt zusätzlich AV1 und VP9). **MPEG-2-Video** (DVB-SD,
  viele IPTV-Kanäle) wird deshalb immer transcodiert; die GPU dekodiert MPEG-2 (`transcode/pipeline.go:573-584`).
- **Gerätefähigkeiten** meldet der Client je Start (`model/model.go:2307-2340`: `containers`, `video`, `audio`, `hls`,
  Bildschirm, `hdr`, seit ADR 0041 `maxAudioChannels`, `selectsTracks`, `subtitles`). Es fehlen ein Container
  «MPEG-TS», eine Angabe «kann Zeilensprung» und eine für die Kanalwechselzeit (`model.go:2308`). Jede Ausgabe ist fMP4-HLS.
- `trackpick` (`trackpick/trackpick.go:1-9`) wählt Ton- und Untertitelspur rein und ohne Datenbank: **wiederverwendbar**
  für DVB-Tonspuren. `segments` (Intro/Abspann) und `mediaindex` (Keyframes aus Dateien, `mediaindex/mediaindex.go:1-11`)
  sind für Live ohne Belang; `mediaindex` wird erst für fertige Aufnahmen wieder wichtig, und TS-Aufnahmen hätten keinen Index.

**Wiederverwenden:** `Decide`, `trackpick`, Grenzen, Token-Schema, Ende-Pfade. **Erweitern:** Deinterlacing als Grund und
Filter, Fähigkeit TS. **Neu:** Sitzungsart für Kanäle, Umschalten innerhalb einer Sitzung. **Risiko:** Der Entscheid ist
durch Vergleich ganzer `Decision`-Werte getestet (`decide.go:50-52`); neue Felder ändern diese Vergleiche und treffen
jede Dateiwiedergabe.

### 3.2 Wiedergabesitzungen

| Schritt | Ist | Beleg |
|---|---|---|
| Anlegen | `Start` prüft Sichtbarkeit, Löschsperre, Grenzen, entscheidet, startet HLS | `playback/sessions.go:97-130`; `limits.go:140-201` |
| Token | HMAC über Sitzungs-ID und Titel, ohne Ablauf (Safari behält Playlist-URLs) | `playback/token.go:10-35` |
| Jede Anfrage | Zeile per Primärschlüssel, Touch höchstens 1×/min, Lebensdauer 12 h | `sessions.go:35-41`, `:800-864` |
| Sichtbarkeit | jede Playlist- und Segmentanfrage prüft `vis.Visible` neu | `sessions.go:866-876`; `streams.go:94-97` |
| Ende | `Stop`, `EndSession` (Logout), `EndAccount` (Gast entzogen), `EndProfile` (gelöscht, Tageslimit); Hook `OnProfileEnd` | `sessions.go:1102-1187` |
| Aufräumen | Sweep jede Minute, Zeilen 400 Tage | `sessions.go:1189-1253` |

**Bindung an `items`:** `playback_sessions.item_id TEXT NOT NULL REFERENCES items(id)` (`00032_playback_prefs.sql:63`).
Ein Sender ist kein Titel. Entweder würde ein Kanal ein `items`-Eintrag (dann gelten Regel 2 und `visible_items`), oder
Live bekommt wie das Radio eine eigene Sitzungstabelle: `radio_listen_sessions` mit `ended_by` inklusive `'restart'`
(`00038_radio.sql:139-151`), eigener Token (`radio/token.go:10-35`), Anschluss an die Ende-Pfade über den Hook
`OnProfileEnd`. **Gebaut ist die zweite Möglichkeit nur für Radio.**

### 3.3 Grenzen je Konto und Profil

Gebaut sind Grenzen je Profil (`MaxStreams`, `MaxBitrateKbps`, `MaxHeight`, `WatchLimitMinutes`; `model/model.go:281-301`)
und je Konto (`accounts.max_streams` 1–16, `max_bitrate_kbps`, `max_height`; `00063_minimum_age_accounts.sql:39-41`,
`00061_profiles_open_language.sql:47-49`). Durchsetzung in `playback/limits.go:159-200`; ein Stream zählt als aktiv, wenn
er in den letzten 2 Minuten angefragt wurde (`:19-22`). Je Feld gilt das Strengere (`PlaybackLimits.Cap`,
`auth/auth.go:708-735`).

**Zu schreiben ist: «gebaut für Titel, nicht für Kanäle»** (V1 §3, Punkt 1). Es fehlen:

- Zählung, die Live einschliesst (die Tabelle hängt an `items`);
- Unterscheidung lokal/extern (keine Fundstelle; `trusted-proxies` dient nur der Client-IP, `config/config.go:241`);
- globale Grenzen für Direktströme oder ausgehende Bandbreite (nur das Transcode-Budget, siehe 3.4);
- ein Recht «darf umwandeln» (Suche nach `allow_transcod`, `can_transcode` ohne Treffer, A1 R-4).

Radio zählt nur zur Tageszeit (K-5), nicht zu den Stream-Grenzen (`radio/listen.go:116-118`; Frage 23 F-16, F-19,
`04-entscheidungen.md:90`).

### 3.4 Transcoding und Hardware

**Gebaut für Dateien (VOD).** Eingang ist ein Pfad (`transcode/plan.go:32-43`, `pipeline.go:109-111`); der Plan kennt
Dauer und alle Segmente vorab (`plan.go:92-107`); Remux braucht den Keyframe-Index, sonst `ErrNoKeyframes` (`:274-278`);
die Playlist ist immer `VOD` mit `ENDLIST` (`playlist.go:11-25`); Neustart per `-ss` an Segment *i*
(`pipeline.go:91-101`); ein Encoder, der dem Player vorausläuft, wird angehalten und getötet
(`session.go:502-519`). Für einen Live-Strom wäre Anhalten Datenverlust.

| Teil | Heute | Für Live (Befund) |
|---|---|---|
| `Source` | `Path`, `Parts`, gespeicherter `Probe`, `Keyframes` (`plan.go:32-43`) | Eingang ohne Pfad, Probe zur Laufzeit |
| `Plan` | feste `Segments`, `Duration` | offenes Ende |
| Neustart/Seek | `-ss` auf Segment *i* | Vergangenheit liegt nur im Puffer |
| Leerlauf | nach 60 s ffmpeg weg, Dateien nach 10 min (`manager.go:21-31`) | beim Dauerempfang unerwünscht |
| Zeitachse | `-copyts` mit 10 s Versatz (`pipeline.go:13-18`, `:176-186`) | setzt monotone Quelle voraus; TS-Zeitstempel springen bei Senderwechsel, PCR-Überlauf (33 Bit, rund 26,5 h) und Abbrüchen |

- **Prozesse:** `os/exec` nur in `transcode` (`server/.golangci.yml:27-35`, `:129-133`), immer `exec.CommandContext` mit
  Argumentliste (`session.go:646-658`). Beenden: SIGTERM, `WaitDelay` 2 s, dann SIGKILL (`:26`, `:649-651`).
  **Keine Prozessgruppen** (`Setpgid`, `Pdeathsig`: keine Fundstelle). `nice 10` und `oom_score_adj 500` für **jedes**
  ffmpeg, auch die Radio-Packager (`session.go:702-712`, `:683`): Unter Last verliert ein Empfänger zuerst Pakete, bei
  Speicherdruck stirbt zuerst eine Aufnahme.
- **Wiederanlauf und Logs:** höchstens 2 Starts je Anfrage, nach 3 Fehlschlägen Fehler (`session.go:27-31`, `:236-253`);
  stderr roh in `ffmpeg.log` und als 4-KiB-Tail in `s.err` und ins Warn-Log (`:303-309`, `:540-543`, `:714-734`).
- **Hardware:** `DetectWith` prüft Encoder und Filter, testkodiert NVENC, QSV, VAAPI und wählt NVENC > QSV > VAAPI >
  Software (`backend.go:140-223`, `:228-258`); ohne `libx264` startet nichts (`:180-182`). Ketten je Backend in
  `videoPipeline` (`pipeline.go:398-519`); Gerätepfad fest `/dev/dri/renderD128` (`backend.go:30`). Die Erkennung
  testet zwei Eingänge ohne Pfad (`-f lavfi -i color=…`, `backend.go:285`, `:292`; Testbild, kein Netz) und die Concat-Liste (`pipeline.go:109`)
  (V1, Korrektur 1).
- **Budget:** NVENC 12, QSV/VAAPI 8, Software Kerne/2 (`manager.go:104-115`), überschreibbar mit
  `SLOUCHCAT_TRANSCODE_BUDGET`; Kosten Remux 0,5, nur Ton 0,25, HD 2, 4K-HDR 4, 2160p-Ausgabe 5 (`plan.go:109-124`).
  Warteschlange bis 10 s, sonst 503 `transcode_busy` (`manager.go:170-232`). Download-Leases halten Budget und werden von
  Live-Wiedergabe verdrängt (`download.go:188-246`): **Muster für Dauerempfang**, aber eine Aufnahme darf man nicht
  verdrängen wie einen Download.
- **Segmentformat:** nur fMP4 (`pipeline.go:208-222`, `segmenter.go:16-55`); Segmentlänge 4 s (`plan.go:24-30`).
  Ein TS-HLS für ältere Fernseher gibt es nicht.
- **Ablage:** `<cache>/transcode` wird beim Start gelöscht (`manager.go:117-131`); der Plattenverbrauch wird nur
  gemessen (`:335-366`), nicht begrenzt; kein Umgang mit `ENOSPC` in `transcode/` und `radio/`.
- **Messwerte:** erstes Segment QSV 1080p rund 0,87 s (`docs/perf/wiedergabestart.md:18-26`); Kapazität CT 190
  (2 vCPU, 4 GB RAM, UHD 630): 4 × 1080p oder 2 × 4K-HDR (`docs/perf/M1b.md:125-172`, `M10.md:9`); 2160p60 nur 1,4 ×
  Echtzeit (`docs/perf/13-abspielseite.md:37-61`). **Es gibt keine Messung** zu Kanalwechselzeit, Deinterlacing,
  Plattendurchsatz eines Video-Zeitversatzes und Last gleichzeitiger Aufnahmen.

**Wiederverwendbar ohne Umbau:** `Runner`, `DetectWith`, `videoPipeline`, Tonkodierung, `delayedInit`,
`startFFmpegPiped`, `tailWriter`, `deprioritize` (mit anderer Priorität), Budget und Leases, Box-Werkzeuge. **Fest auf
Dateien:** `Plan`, `ffmpegArgs`, `Session`. Ein eigener Live-Session-Typ neben `Session` hält die Abstimmung der
Dateikette fern (ADR 0003 Nachtrag vom 7. November 2026).

### 3.5 Deinterlacing

**Zeilensprung (Interlacing)** heisst: Das Bild besteht aus zwei ineinander verschachtelten Halbbildern; ohne
Entfernung entstehen Kammartefakte. 576i und 1080i sind bei Fernsehen der Normalfall. Im Server gibt es **kein**
`yadif`, `bwdif` oder `deinterlace_*` (Suche ohne Treffer in `server/internal/`); ein Remux reicht Halbbilder
unverändert weiter. **Das Image enthält die Filter** (M1): `yadif`, `bwdif`, `deinterlace_qsv`, `deinterlace_vaapi`,
`yadif_cuda`, `bwdif_cuda`, dazu `idet` (Erkennung). Auf CT 190 ist CPU-Deinterlacing teuer (2 vCPU). Je GPU-Backend
ist nicht gemessen.

### 3.6 FFmpeg im Image (gemessen, M1)

Paket `jellyfin-ffmpeg7` 7.1.4-3 für `trixie` (`deploy/Dockerfile:69-102`). Der Server fragt zur Laufzeit nur
`-version`, `-encoders`, `-filters` und Testkodierungen ab, **nie** `-protocols` oder `-demuxers`
(`transcode/backend.go:160-172`). Die Messung M1 (Image 1.41.3) schliesst die Lücke:

| Frage | Befund (M1) | Folge |
|---|---|---|
| Eingangsprotokolle | u. a. `http`, `https`, `tcp`, `tls`, `udp`, `rtp`, `srtp`, `rtmp` (und Varianten), `hls`, `ftp`, `pipe`, `file`, `concat`, `subfile`, `data`, `crypto`, `unix` | UDP, RTP und RTMP sind im Image vorhanden |
| Demuxer | `mpegts`, `hls`, `dash`, `rtsp`, `rtp`, `flv`, `mov,mp4` | HLS, DASH, TS, RTSP lesbar |
| SRT | weder in der Protokollliste noch in der Konfiguration (kein `libsrt`-Eintrag gefunden) | SRT-Quellen gingen mit diesem Build nicht |
| DASH | `--enable-libxml2` gesetzt | DASH-Demuxer vorhanden |
| Ausgabe-Muxer | `hls`, `dash`, `segment`, `mpegts`, `matroska`, `mp4`, `tee`, `fifo` | Aufnahme und Segmentierung möglich |
| Deinterlacer | siehe 3.5 | vorhanden, ungemessen |
| Offene Protokolle | alle eingebauten, solange `-protocol_whitelist` fehlt (`file`, `concat`, `subfile`, `data` …) | Sicherheitsfrage, siehe 3.8 |

M1 listet die Protokolle des Builds; ob eine Quelle in der Praxis läuft, ist damit nicht gemessen. Ein anderes
ffmpeg-Paket oder andere Build-Flags änderten die Grundlage aller Dateiwiedergaben (Kalibrierung `docs/perf/M1b.md`).

### 3.7 Webradio-Relay

Das Vorbild für «einmal empfangen, an viele verteilen» ist **gebaut, aber nur für Ton**.

- **Aufbau:** Ein Relay je Sender, gleich wie viele zuhören (`radio/relay.go:20-24`; `radio/listen.go:228-270`). Go holt
  die Quelle (ICY, HLS), ffmpeg liest nur `pipe:0` und schreibt auf `pipe:1` (`transcode/radio.go:76-80`, Kopfkommentar
  `:14-17`), Go schneidet fMP4 auf Box-Ebene und baut die Live-Playlist mit Diskontinuitäten und Program-Date-Time
  (`radio/playlist.go:11-62`).
- **Generationen:** Codec-, Stream- oder Bruchwechsel starten einen neuen Packager mit neuem Init, `DISCONTINUITY` und
  neuer PDT (`relay.go:19-23`, `:211-213`, `:259-333`): das Muster für den Wechsel auf eine Ersatzquelle.
- **Ersatzquellen und Backoff:** beim Start jede Quelle einmal, danach Rotation bei jedem Wiederaufbau
  (`relay.go:183-205`); Backoff 1, 2, 5, 10, 20 s, danach endet das Relay; «gesund» nach 1 min; Tempo höchstens
  10 s vor Echtzeit (`:47-61`, `:336-348`).
- **Puffer = Zeitversatz:** im `tmpfs` (`/run/slouchcat/radio`) oder nur im RAM für den Live-Rand (`radio/buffer.go:15-28`);
  Reservation = Bitrate × Fenster × 1,03, Zulassung gegen `max_relays` und `budget_mb` (`buffer.go:67-70`;
  `listen.go:245-254`). Standard `budget_mb` 512 (64–4096; `00038_radio.sql:166`), tmpfs fest 564 m
  (`deploy/compose.yaml:39-40`): ein höheres Budget führt zu Schreibfehlern, die nur als Warnung erscheinen
  (`relay.go:388-402`). Grace 120 s (`00038_radio.sql:168`). Radio-Zeitversatz 10/30/60 min (`radio/config.go:61`).
- **Sitzung und Token:** eigene Tabelle `radio_listen_sessions`, Token `rt` = ULID + HMAC über Hörsitzung und Sender
  (`radio/token.go:10-35`; Schlüssel `radio-token`, `cmd/slouchcat/main.go:460`).
- **Katalog und Logos:** Abgleich täglich im Fenster 04:30–06:30, 1 Anfrage/s, verschwundene Sender nach 7 Tagen weg
  (`radio/jobs.go:16-43`, `radio/sync.go:24`); M3U-Parsing nur für `.pls`/`.m3u` mit Einträgen, **ohne**
  `#EXTINF`-Attribute wie `tvg-id` und `group-title`. Logos über den geschützten Client (`radio/logo.go:275-287`),
  skaliert nach WebP und PNG (`transcode/images.go:108`, `:174`), versioniert über `logo_hash`: direkt übertragbar auf `tvg-logo`.

**Grenzen für Video:** Segmente bis 2 MiB, HLS-Varianten bis 320 kbit/s (`radio/hlssrc.go:24-33`), Box bis 8 MiB
(`transcode/radio.go:47-50`), Puffer im RAM. Wer die Typen verallgemeinert, ändert Grenzen, die das Radio gegen feindliche
Quellen schützen (`radio/hostile_test.go`). 8 Mbit/s × 30 min ≈ 1,8 GB je Kanal: tmpfs (564 MB) und RAM von CT 190
(4 GB) tragen das nicht (A2 §7.5). Radio hat **keine** Aufnahmen (Frage 23, K16 F-1).

### 3.8 Ausgehende Verbindungen und `netguard`

- **Heute holt nur Go das Netz.** ffmpeg bekommt nie eine Netzadresse: keine Fundstelle für `-i http…` in `transcode/`;
  Suche nach `"-i"` ohne Treffer mit Netzadresse (V1, Korrektur 1). `-protocol_whitelist` setzen **nur Bilder und
  Radio** (`transcode/images.go:108`, `:174`; `transcode/radio.go:79`); Trickplay (`trickplay.go:145`, `-i pipe:0`),
  Untertitel, Tags, Fingerabdruck, Frames und Hörbuch haben keine. **Jeder `os/exec`-Aufruf ausser Bild und Radio ist ohne Whitelist.**
  Die Abwehr des Sicherheitsreviews stützt sich darauf, dass jeder Pfad mit `/` beginnt (`docs/dev/SICHERHEITSREVIEW-1.0.md:633-640`).
- **Was `netguard` sperrt:** `RefuseLocal` ist eine `net.Dialer.Control`-Funktion und sieht die aufgelöste Adresse jeder
  Verbindung, auch nach Weiterleitungen und bei DNS-Rebinding (`netguard/netguard.go:70-82`). Gesperrt: Loopback,
  privat, Link-Local, **Multicast**, unspezifiziert (`:50`), dazu `100.64/10`, `198.18/15`, `0/8`, NAT64 und weitere (`:29-37`).
  Eine globale IPv6-Adresse im Heimnetz ist nicht erkennbar und bewusst nicht gesperrt (`:44-47`). Kein Proxy aus der
  Umgebung (`radio/reader.go:61`). Weiterleitungen begrenzt der Aufrufer (`fetch/fetch.go:46-53`; `radio/reader.go:34`: 5 Sprünge).
- **Ausnahme für lokale Ziele:** nur bei Podcasts. `Clients.For(ctx, feedURL, origin)` gibt einen **völlig ungeschützten**
  Client zurück, wenn der Host der Feed-URL lokal ist (`netguard.go:118-128`); dieser Client hat keine `Control`-Funktion
  (`:96-100`) und darf danach auch an öffentliche und andere lokale Ziele. `LocalHost` löst den Namen selbst auf (`:58-60`):
  Entscheid und spätere Verbindung sind zwei getrennte Auflösungen. Für «nur dieser Host im LAN» wäre das Muster zu grob (V1, Lücke L-5).
  Radio kennt keine Ausnahme; eigene Sender werden schon beim Anlegen abgewiesen (`radio/own.go:35-36`); Arr und Seerr laufen bewusst ungeschützt.
- **Dialer-Test-Variante:** Der Radio-Dialer hat zwei Build-Varianten (`!radiotest`/`radiotest`, `radio/dialer.go:9-11`);
  nur der e2e-Build erlaubt Loopback. Ein Live-TV-Dialer braucht dasselbe, sonst sind e2e-Tests gegen eine lokale
  Test-Quelle unmöglich (V1, Lücke L-5).
- **Herkunft einer Quelle:** Für Live-TV gibt es keine Angabe, woher eine Quelle stammt (Admin getippt, aus Liste
  importiert, aus Provider-Konto); das Muster `origin` von `Clients.For` kennt nur `file` und Admin (V1, L-5).
- **`ratelimit`:** Token-Bucket je Anbieter mit Backoff nach 429 und Tageskontingent (`ratelimit/ratelimit.go:1-30`);
  brauchbar für Listen- und EPG-Abrufe.

### 3.9 Geheimnisse und Redaction

- **Speicherung gebaut:** AES-256-GCM, Format `v1:` + base64url, Schlüssel `<config>/secret.key` (32 Byte, 0600),
  Zweckschlüssel per HKDF (`secrets/secrets.go:43-153`; `cmd/slouchcat/main.go:445`, `:460`, `:764`). Nutzer:
  Anbieterschlüssel in `settings` und Hörbuchquellen (`admin/settings.go:630-640`, `admin/book_sources.go:246`).
  **Vorbild mit Lücke:** Podcast-Feed-URLs dürfen Zugangsdaten tragen, liegen aber im Klartext in `items.feed_url`
  (`00014_audio.sql:71`) und gehen in jede Sicherung.
- **Redaction (Regel 10):** nur `slog` über `RedactingHandler` (`logging/logging.go:77-127`). `sensitiveKeys` fangen
  Schlüssel mit Endung `password`, `token`, `secret`, `listurl`, `endpoint` … (`:63-70`). `sensitiveQuery` maskiert
  `api_key|apikey|key|client_key|st|dt|rt|auth|token|access_token|secret|sig|signature` (`:75`). **Es fehlen** `username`,
  `user`, `password`, `pass`. Pfadmuster nur für Einladung, Webhook, Telegram-Bot (`:129-161`); **nicht erfasst** sind
  `user:pass@` in URLs und Xtream-Pfade `/live/<user>/<pass>/<id>.ts`. Schlüssel wie `url`, `streamurl`, `sourceurl`
  sind nicht unter den `sensitiveKeys`: Wer eine URL als Feld loggt, muss sie durch `RedactString` laufen lassen
  (`error`-Werte und `*url.URL` tun das, einfache Strings nicht; V1 §2.5). Gute Praxis: Radio und Podcasts loggen nur `netguard.Site(url)`
  (`radio/relay.go:187-188`).
- **Folgen:** Eine URL in der Argumentliste von ffmpeg steht in `/proc/<pid>/cmdline` und im `ps` des Hosts; ffmpeg
  schreibt bei Fehlern die Eingangs-URL in stderr, das heute roh nach `ffmpeg.log` und ins Warn-Log geht. Der Radio-Packager
  hat das Problem nicht, weil ffmpeg dort keine URL kennt (ADR 0025).
- **Sicherung:** ohne Geheimnisse verliert die Kopie die verschlüsselten Einstellungen; mit Geheimnissen wird das
  Archiv mit `age` verschlüsselt (`admin/backup.go:33-40`).

### 3.10 Pfade und Speicher

- `paths`: Wurzeln absolut, `Resolve` verwirft `..`, NUL und absolute Teile, Zugriff über `os.Root` (`paths/paths.go:15-48`,
  `paths/root.go:11-24`). Bekannte Grenze: ein nach der Prüfung untergeschobener Symlink, wenn der Pfad an ffmpeg geht
  (`SICHERHEITSREVIEW-1.0.md:349-368`).
- **Schreibziele heute:**

| Ziel | Ort | Grenze | Beleg |
|---|---|---|---|
| Transcode | `<cache>/transcode/<id>/` | keine, nur gemessen | `config/config.go:193-194`; `manager.go:335-366` |
| Radio-Puffer | tmpfs `/run/slouchcat/radio/…` | `budget_mb` 512; tmpfs 564 m | `radio/buffer.go:15-19`; `compose.yaml:39-40` |
| Offline-Downloads | `<cache>/downloads` | 100 GB | `config/config.go:66-68`; `offline/job.go:126-135` |
| Bilder, Logos, Trickplay | `<cache>/images/…` | Anzeige freier Platz | `radio/logo.go:41` |
| Podcast-Folgen | schreibbarer Ordner auf dem NAS | Ordnerprüfung, freier Platz | `library/podcast_folder_check.go:17-40` |

- **`/media` ist nur lesend eingehängt** (`deploy/compose.yaml:36`); schreibbar ist nur der Podcast-Ordner über
  `deploy/compose.podcasts.yaml:11-15`, ein **Unterordner**, der im `:ro`-Baum existieren muss (V1 §2.8, ADR 0029). Das ist das
  einzige Schreibmuster.
- **Freier Platz:** nur für Anzeigen und Podcasts abgefragt (`admin/storage_linux.go:15`); `ENOSPC` wird nur beim
  Podcast-Download erkannt (`library/podcast_download.go:643-644`).
- **Aufräumen nach Absturz:** Transcode-Ordner beim Start (`manager.go:126`); Radio-Pufferordner und offene Hörsitzungen
  (`radio/listen.go:59-90`); Offline-Teildateien (`offline/cleanup.go:15-30`).
- **Nicht eingehängte Netzlaufwerke (gebaut, zweifach):** der Scanner bricht bei leerer Wurzel ab (`root_empty`,
  `library/pathscan.go:249-255`); die Podcast-Ordnerprüfung vergleicht den Dateisystemtyp (`library/podcast_folder_check.go:17-29`,
  `paths/fstype.go:6-54`, `paths/writable_linux.go:14-28`). **Direkt übertragbar auf einen Aufnahmeordner**, auch für den
  Fall, dass das NAS mitten in der Aufnahme wegfällt.
- **Sicherung:** Medien und Cache sind nie Teil der Sicherung (ADR 0009, A-4): Aufnahmedateien nicht, ihre Zeilen schon.

### 3.11 Compose, Betrieb, Updater, Shutdown

- **Netz:** kein `network_mode`, Standard-Bridge, nur Port 8096 (`deploy/compose.yaml:16-31`). Multicast kommt über die
  Docker-Bridge nicht in den Container; nötig wären `network_mode: host`, macvlan/ipvlan oder ein Umsetzer ausserhalb.
  `host` änderte zugleich die Annahmen von `SLOUCHCAT_TRUSTED_PROXIES` und der HTTPS-Variante (`compose.yaml:3-7`,
  `compose.https-only.yaml:1-6`). Kein DVB-Gerät (`/dev/dvb`) vorgesehen.
- **Ressourcen:** kein `mem_limit`, `cpus` oder `pids_limit` (Suche über `deploy/*.yaml`). Hardware-Overrides
  `compose.{nvidia,intel,amd}.yaml`.
- **Shutdown-Reihenfolge** (V1, L-4; `cmd/slouchcat/main.go`, `defer`s rückwärts): (1) `srv.Shutdown` mit 10 s
  (`:656-664`; offene Direct-Play-Antworten werden abgeschnitten); (2) `stopSweep()` und `sweeper.Wait()`: beendet
  `playbackSvc.Run`, `offlineSvc.Run` und **`radioSvc.Run` (stoppt alle Relays)**; (3) `closeTranscoder()` (`mgr.Close`
  beendet alle ffmpeg, `:566-575`); (4) `workers.Wait()` (Lease zurück ohne Versuch); (5) `WaitActivity` (5 s); (6) `db.Close()`.
  Docker sendet SIGKILL nach der Vorgabe 10 s; `stop_grace_period` fehlt im Hauptdienst (nur der Updater hat 30 s,
  `compose.updater.yaml:26`). A2 nannte (1) und (3), nicht den Relay-Stopp in (2).
- **Nach dem Neustart** sind Transcode-Ordner leer, Radio-Relays und -Puffer weg und Hörsitzungen beendet; HLS-Sitzungen
  bauen beim nächsten Abruf neu auf (`sessions.go:866-900`). Ein Wiederanlauf aus der Datenbank gäbe es nur nach dem
  Muster `ended_by 'restart'` (`00038_radio.sql:150-151`).
- **Updater:** ersetzt den Container auf einen Klick (ADR 0024); `admin/update_install.go` prüft **keine** laufenden
  Streams oder Aufnahmen (Suche ohne Fund).
- **PID 1** ist der Server selbst, ohne `init` oder `tini` (`deploy/Dockerfile:143-144`); Zombies holt nur `cmd.Wait`. Bei
  vielen kurzlebigen ffmpeg (Kanalwechsel-Probes) und ohne Prozessgruppen bleibt das eine Leckgefahr, falls ffmpeg
  Kinder startet (V1, L-5).

### 3.12 Konten, Profile, Rechte

- **Schema:** `accounts` (`role admin|user`, `is_owner`, `max_rating 0..18`, `all_libraries`, Grenzen) in
  `00063_minimum_age_accounts.sql:27-44`; `profiles` (`account_id`, `max_rating`, `is_default` = Hauptprofil, PIN, Grenzen,
  `watch_limit_minutes`) in `00061_profiles_open_language.sql:34-57`. Drei Ebenen Administrator, Konto, Profil; «Profil
  schaut und verwaltet nichts» (ADR 0014, `0014-benutzerverwaltung.md:66`).
- **Ein «Kinderprofil» gibt es als Begriff nicht:** keine Spalte `is_child`; Kind ist jedes Profil mit `max_rating < 18`
  (`model/ratings.go:26`; ADR 0045 «Ohne Altersgrenze heisst Stufe 18»).
- **Hauptprofil:** Profilverwaltung nur durch das Hauptprofil (`requirePrimary`, `auth/profiles.go:40-45`; `403
  primary_profile_required`, `api/session.go:643`). «Hauptnutzer» des Auftrags kann Administrator, Besitzer oder Hauptprofil heissen.
- **Muster `can_request`:** Spalte auf `accounts`, `profiles` und `invitations` als `ALTER TABLE … ADD COLUMN … DEFAULT 0 CHECK`
  ohne Neubau (`00067_wishes_rights.sql:22-24`); wirksam = Konto UND Profil (`:9`). Seit ADR 0045 setzen zwei Trigger das
  Recht nach der Altersgrenze, sechs weitere weisen mit `RAISE(ABORT,'age_restricted')` ab (8 Trigger in
  `00086_wishes_rights_follow_rating.sql:86-120`). **Die Live-TV-Schalter dürfen nicht an diese Trigger gehängt werden**:
  der Auftrag will für Kinderprofile eine Freigabe ohne Altersprüfung (LTV-015).
- **Durchsetzung:** Middleware `authorize` prüft `x-visibility` je Route aus dem Vertrag (`api/session.go:305-380`):
  unbekannt oder `system` = 401 fail-closed, Bearer ohne `x-clients` = 403 `client_not_allowed`, `admin` = Administrator
  plus Hauptprofil, `scoped` = Profil gewählt. Gültige Stufen nur `public, account, scoped, admin, owner` (`:384`).
  **Fachrechte setzt nicht die Middleware durch, sondern der Dienst** (`wishes/rights.go:14-20`) und meldet sie in
  `SessionInfo.wishes` (`openapi.yaml:13754-13789`). Das Wunschrecht antwortet mit **403** `age_restricted`; «unsichtbar, 404»
  für ein Fachrecht ist neu und gehört in den Dienst. Eine neue `x-visibility`-Stufe hiesse Änderung von `validVisibility`,
  Middleware und Vertragstests.
- **Frische:** Konto und Profil werden **bei jeder Anfrage** neu geladen (`auth/auth.go:505-523`); Rechteänderungen wirken
  sofort.
- **Sitzungsarten (ADR 0040):** `sessions.kind IN ('browser','app','device')` (`00082_session_kind.sql:14`); App und TV =
  Bearer ohne Ablauf, nie `admin`/`owner`. Logout widerruft Wiedergabesitzungen in derselben Transaktion
  (`auth.go:613-647`); Hooks `accountDeleting`, `profileDeleting`, `requestRightChanged` (`:71-93`); Dienste
  `EndAccount`/`EndProfile` für Wiedergabe und Radio (`api/services.go:580-583`, `:725-726`).
- **Geräteverwaltung** ist gebaut (`openapi.yaml:5839`, `:5865`, `:11127`).
- **Neu für Live-TV:** drei Schalterpaare «Live-TV», «Aufnahmen», «Timeshift» auf Konto und Profil (und
  `invitations`); Kontingent je Konto für Aufnahmen gibt es nicht; Sendergruppen je Profil gibt es nicht.

### 3.13 Sichtbarkeit

- `visible_items` (View, `00063_…:92-112`): Profil × Konto × `items`, Freigabe `≤ min(Profil, Konto)`, unbekannt = 18,
  Bibliotheksrechte beider Ebenen. Das Paket `visibility` ist der einzige Lesepfad für Titel (`visibility/visibility.go:1-14`).
- Der Analyzer `itemsaccess` erlaubt SQL auf `items`, `media_files`, `tracks` nur in `visibility, admin, library, metadata,
  jobs, notify, offline, store` (`server/tools/itemsaccess/itemsaccess.go:28`); `depguard` für `systemq`
  (`.golangci.yml:37-43`). Der Analyzer deckt eigene Tabellen **nicht** ab.
- **Muster ausserhalb von `items`:** eigenes sqlc-Paket mit `depguard`-Riegel, nur ein Fachpaket liest (`certq`, `externalq`,
  `wishq`; `sqlc.yaml:40-73`, `.golangci.yml:44-73`). **Webradio:** Sichtbarkeit **global** (Angebot des Administrators),
  Sender ausserhalb des Angebots = 404 (`radio/catalog_profile.go:142-157`); je Profil nur Favoriten und Zustand
  (`00038_radio.sql:123-137`); F-5 «alle Profile hören Radio, ohne Schalter und ohne Altersgrenze». **Wünsche:** privat je
  Profil über `wish_requesters (wish_id, profile_id, account_id, role)` (`00069_wishes.sql:125-134`). **ADR 0038:** Titel ausserhalb der Bibliothek in eigenen Tabellen, Alterspüfung in `visibility`, unsichtbar = 404.
- **Befund:** Sender, Sendungen und Aufnahmen passen nicht in `visible_items`; die View selbst nicht anfassen (jede Änderung
  ist ein Neubau in einer Migration). «Privat je Profil» = Zeilen mit `profile_id … ON DELETE CASCADE`, nur mit der Profil-Id
  der Sitzung gelesen. Radio liefert kein Vorbild für Sichtbarkeit je Profil.

### 3.14 Datenbank

- SQLite über `modernc.org/sqlite` (`go.mod:26`). **Ein Schreiber:** Schreibpool mit einer Verbindung, `_txlock=immediate`;
  Lesepool `query_only(1)` mit `max(4, NumCPU)`; Pragmas `busy_timeout(5000)`, `journal_mode(WAL)`, `foreign_keys(1)`,
  `synchronous(NORMAL)` (`store/store.go:94-118`).
- Migrationen nur vorwärts nach Vorlage; vor jedem Lauf `VACUUM INTO`, 5 Kopien (`store/migrate.go:19`, `:116`).
  CHECK-Listen lassen sich nur mit Tabellenneubau ändern (Beispiele 00070, 00084). sqlc-Pakete `systemq`, `scopedq`, `appq`,
  `certq`, `externalq`, `wishq` (`sqlc.yaml:5-73`).
- **FTS5** `search_index` über `search_docs`, `kind` mit CHECK `('item','text','person','collection')` (`00015_discover.sql:136-160`;
  die CHECK-Liste selbst wurde von V1 nicht neu geöffnet). Radio sucht in einem Index im Speicher (7,1 ms über 40'000 Sender).
- **Grösste Bestände:** Radio 55'953 Sender (`docs/perf/19-radio.md:13`), Abgleich 21 s, rund 40 MB; Titel rund 70'000
  (`docs/perf/24-ordner-metadaten.md:6`); DB 88 MB (`docs/perf/bibliotheken.md:92`). Stapel zu 1000 (`radio/sync.go:33`),
  Jobregel höchstens 500 Zeilen je Transaktion (`jobs/queue.go:5-7`).
- **EPG-Grössenordnung (Schätzung, nicht gemessen):** 10'000 Sender × 14 Tage × 20–30 Sendungen ≈ 3 bis 4 Millionen Zeilen,
  rund das 60-Fache des heutigen grössten Bestands. Jeder EPG-Import und jede Zustandsänderung läuft über dieselbe
  Schreibverbindung wie Fortschritt, Leases und Sitzungs-Touch; `VACUUM INTO` und jede manuelle Sicherung kopieren das EPG mit.
  Ein Muster für «grosse, flüchtige Daten in der Haupt-DB» gibt es nicht.
- **Letzte Nummern:** Migration 00086; eingefrorene Stände je Konzept in `store/frozen_*_test.go`.

### 3.15 Jobs

- **Gebaut:** persistente Warteschlange in SQLite (`jobs/queue.go:1-7`), Priorität 0–9, `idempotency_key`, Lease 5 min mit
  Heartbeat 60 s, 5 Versuche (`queue.go:82-91`), Fortschritt, `override_pause`, `override_window`. Handler müssen
  idempotent sein. Zeitpläne: `job_schedules` mit **Primärschlüssel `type`**, also genau ein Zeitplan je Jobart
  (`00027_jobs_scheduler.sql:52-54`); Intervall 5 min bis 7 Tage (`scheduler.go:52-54`); Scheduler-Takt 60 s (`:56`);
  verpasste Läufe verfallen, einmaliges Datum höchstens 24 h nachgeholt (`:18-20`, `:46-47`). Worker-Poll 1 s (`worker.go:45`).
  Pools: `general`, je 1 `download`, `notify`, `index`, `trickplay`, 5 `podcast` (`cmd/slouchcat/main.go:612-620`).
  **Pause** je Jobart bricht laufende Jobs ab und gibt sie zurück (`worker.go:149-161`).
- **Eignung für Live-TV:**

| Aufgabe | Eignung | Grund |
|---|---|---|
| Listen-Aktualisierung je Quelle | teilweise | ein Zeitplan je Jobart; Muster `podcast.poll` → `podcast.fetch` (`registry.go:78-84`) passt |
| EPG-Import | ja | Bulk-Job, Stapel ≤ 500, idempotent, eigener Pool sinnvoll |
| Verfügbarkeitsprüfung | teilweise | Minimum 5 min; darunter Selbstverkettung (`wishes/progress.go:30-33`) |
| Aufräumen | ja | `AddPruneTask` (`queue.go:216`) |
| Aufnahme-Planer, sekundengenau | nein | Takt 60 s, Lease/Retry-Semantik, verpasste Läufe verfallen |
| Laufende Aufnahme, Dauerempfang | nein | Pause der Jobart oder Neustart bricht ab und startet später neu; Worker-Platz für Stunden belegt |

Vorbild für Langläufer ausserhalb des Jobsystems: Radio-Relays als Goroutinen des Dienstes `radio` mit Obergrenze
`max_relays` (`00038_radio.sql:163-175`).

### 3.16 Benachrichtigungen

- **Gebaut:** Ereignisarten `new_content, errors, invitations, announcements, wish_available, wish_status, wish_review,
  device_linked` (`model/notify.go:42-57`) als CHECK in `deliveries.event` (`00084_device_linked_deliveries.sql:16-30`);
  Kanäle `email, push, telegram` je **Konto** (`account_channels`, `00016_admin.sql:27-35`); Android-Push über den
  Weiterleitungsdienst (ADR 0044); der Server sendet immer `UrgencyNormal` (`notify/push/push.go:174`). Empfänger je
  Ereignis (`notify/dispatch.go:68-180`); `ProfileID` setzt nur ein Präfix «[Profilname]» in den Betreff. Entprellung nur für `errors`,
  einmal je `Source` und Stunde (`dispatch.go:25`). Zustellung als Job `notify.deliver` mit **einem** Worker.
- **Posteingang je Profil:** `notifications` mit `kind`-CHECK (`new_episode`, `wish_*`) und `CHECK (item_id IS NOT NULL OR
  wish_id IS NOT NULL)`: **mindestens eines von beiden** (`00070_wishes_notifications.sql:32-43`; V1, Korrektur 4). Eine
  dritte Art (Sendung/Aufnahme) braucht den Neubau der Tabelle.
- **Eignung:** Ausfallmeldung an den Administrator passt direkt (`EventErrors` mit `Source` je Quelle, Vorbilder
  `ScanAborted`, `RootRemoved`, `notify/events.go:83-121`). «Aufnahme fertig/fehlgeschlagen» braucht ein neues Ereignis
  (Neubau der CHECK). Eine Sendungserinnerung fehlt und ist zeitkritisch (Takt, `UrgencyNormal`). Kanaleinstellungen sind je
  Konto: ein Profil kann seine Erinnerungen nicht getrennt schalten, und Meldungen an das Konto gehen an alle Kanäle des
  Kontos (Datenschutz «privat je Profil»).

### 3.17 API-Konventionen und Segment-Token

- **Vertrag:** `api/openapi.yaml` (25'581 Zeilen), 1.44.0; Stufen: 179 `admin`, 92 `scoped`, 27 `account`, 11 `public`, 4
  `owner`; `x-clients` 93× `[app, tv]`, 26× `[app]`. Fehlerformat RFC 9457 mit stabilem `code` (`openapi.yaml:13428-13458`).
  Seiten über `offset`/`limit`; `cursor` nur einmal (`:3773`). Vertragstests `TestContractCoversEveryOperation` und
  `TestXClientsCompatibility` (`api/contract_access_test.go:562-600`, `contract_compat_test.go:16-35`).
- **Nur Polling:** keine Fundstelle für `text/event-stream`, `EventSource` oder WebSocket in `server/internal/`,
  `api/openapi.yaml` und `web/src`.
- **Binäre Endpunkte** ausserhalb des strict servers, `x-visibility` und `x-clients` in Konstanten wiederholt
  (`api/api.go:25-60`, `:94`, `:109`).
- **Wie Segmente authentisiert werden (V1, Lücke L-1):** Die Medienrouten laufen **nicht** durch `authorize`, sondern
  durch `streamAuth`/`hlsAuth` (`api/api.go:200-212`, `api/stream_token.go:24-43`). Geprüft wird nur der Query-Parameter
  `st`: Auflösung der `playback_sessions`-Zeile per Primärschlüssel (`sessions.go:803-851`), HMAC über Sitzungs-ID und
  **Item-ID**, Konto und Profil neu laden. Cookie und `Authorization` werden dort nicht gelesen. Folgen: Ein Kanal-Token
  braucht eine **eigene** Zeile und einen eigenen HMAC-Inhalt (Kanal statt Item), wie `rt` (`radio/token.go`); Entzug wirkt
  nur, wenn die neue Sitzungsart in `EndSession/EndProfile/EndAccount` eingehängt wird (Hook `OnProfileEnd`,
  `sessions.go:1180-1187`); `Referrer-Policy: no-referrer` schützt das Token in URLs; ein neuer Parametername muss in
  `sensitiveQuery` und den Test (`logging.go:75`).
- **Web-Medien-URLs (V1, L-2):** `streamUrl` trägt das Token als `?st=…` (`model/model.go:1436`); hls.js und `<video>`
  laden ohne Header direkt vom Server; Bilder laufen über das Cookie. Alle Live-Segment-URLs sind damit Bearer-URLs: sie
  landen im Browserverlauf, in Proxy-Logs (NPM CT 140) und im Netzwerk-Tab.
- **Weitere Tokenarten:** `rt` (Radio), `dt` (Downloads, `api.go:222-225`).
- **Verträglichkeit für Apps:** `docs/dev/VERTRAEGLICHKEIT.md:54-64`: neue Aufzählungswerte gelten als nicht brechend,
  Änderungen an Streaming-Routen als brechend, ohne dass der Test sie sieht; 90 Tage alt und neu nebeneinander,
  verbindlich ab der ersten öffentlichen App-Version, bis dahin Übung. `TestXClientsCompatibility` prüft nicht, ob die
  installierten Apps einen unbekannten Aufzählungswert vertragen (`:96-101`).

### 3.18 Verwaltung und Systemwerte

- **Gebaut:** Dashboard mit laufenden Streams, Jobzählern, freiem Platz von `config` und `cache`, Integrationen,
  Fehlerring und Update-Stand in einer Anfrage (`admin/dashboard.go:18-36`). «Aktive Streams»: Schnittstelle `Streams`
  (`admin/streams.go:21-32`); `AdminStream` zeigt **Konto-E-Mail, Profilname, Titel**, Modus, Backend, Qualität, Position
  (`model/model.go:2437-2462`); Radio hat eine eigene Relay-Liste mit Stopp (`api/radio_handlers.go:846-879`).
  Aktivitätsprotokoll 30 Tage / 2000 Zeilen, nur Ids, keine IP (`admin/activity.go:32-33`; ADR 0019 `:79-87`); Fehlerring
  der letzten 50 Fehler hinter der Redaktion (`logging/errors.go:13-28`).
- **Systemwerte:** Version, GPU-Erkennung, inotify-Grenze, Dateisysteme (`admin/system.go:16-117`), Plattenplatz per `statfs`.
  **Keine CPU-, RAM- oder GPU-Last** (keine Fundstelle für `/proc/stat`, `meminfo`, `loadavg`, `nvidia-smi`).
- **Datenschutz-Praxis:** Es gibt keinen Grundsatz «der Administrator sieht keine privaten Daten der Profile»; er sieht
  heute in «aktive Streams» Profil und Titel, und Statistiken zeigen Sehzeit je Profil.
- **Web-Verwaltung:** Bereiche `media, people, home, automation, system` (`web/src/features/admin/sections.ts:15`,
  Radio als `area: 'media'` `:74`, Streams `:79`, Jobs `:87`); Seitenleiste als Baum mit Alarm-Pillen
  (`ui/AdminSideNav.tsx`); Seitenrahmen `ui/AdminPage.tsx:37-50`; Radio-Verwaltung `RadioAdminPage.tsx` mit Tabs;
  Streams-Seite fragt alle 5 s (`StreamsPage.tsx:158`) mit Gruppen `video/audiobook/podcast/radio`.

### 3.19 Suche, Merkliste, Fortschritt, Tageslimit

- **Alles an `items` gebunden:** `watchlist (profile_id, item_id REFERENCES items, …)` (`00054_watchlist_follow.sql:23-29`;
  die Tabelle `favorites` ist gelöscht); `progress (profile_id, item_id REFERENCES items, …)` (`00004_playback_progress.sql:9-19`)
  mit `progress_days` (00019). FTS nur für Titel, Texte, Personen, Sammlungen (`visibility/search.go:186-346`).
- **Radio-Favoriten** je Profil mit Reihenfolge (`radio_favorites`, `00038_radio.sql:123-129`; `api/radio_handlers.go:387-445`);
  Titel-Favoriten sind durch die Merkliste ersetzt (Frage 27).
- **Tageslimit (V1, L-5):** `playback.watchTime` (`playback/limits.go:251-273`) zählt aus `progress_days`, die an `items`
  hängen; Radio hat eine eigene Zeitrechnung (`radio/listentime.go:17-51`). Live-TV müsste dort anschliessen, sonst zählt
  das Tageslimit eines Kinderprofils Fernsehen nicht mit. Statistik (`admin/stats.go:16-43`) zählt nur Titel.
- **Befund:** nur Muster, keine Wiederverwendung der Tabellen. Senderfavoriten folgen `radio_favorites`;
  Aufnahme-Fortschritt je Profil folgt `progress` als eigene Tabelle. Eine Programmsuche bräuchte eigenen Index (FTS
  oder Speicher); 3–4 Mio. Zeilen sprengen den Speicherindex eher.

### 3.20 Einrichtung

Assistent mit neun Schritten seit Konzept 22, abgelegt in `settings.setup_wizard` (JSON), `finishSetup` schliesst
(`admin/setup_wizard.go:15-19`, `:93-160`; `openapi.yaml:284`, `:297`, `:8281`). Ein Schritt «Live-TV» wäre additiv; eine
fehlende Einstellung zählt als «fertig», also braucht der Bestand keine Datenmigration. Einstellungen: `settings (key,
value, updated_at)`, Einzeilentabellen je Bereich (`radio_config` mit `id = 1`), Geheimnisse über `Keyring.Encrypt`.
Servertexte als i18next-JSON, 23 Sprachen für Benachrichtigungen (`i18n/i18n.go:1-4`, `notify/locales/`).

### 3.21 Web-App: Navigation, Bauteile, Player, Service Worker

- **Routen und Navigation:** TanStack Router, je Seite ein Chunk (`web/src/app/router.tsx`, Zuschauerseiten unter
  `chromeRoute` `:353`, Verwaltung `adminRoute` `:382`); Pfade in `web/src/app/paths.ts:4-110`. Seitenleiste `ui/SideNav.tsx`,
  Einträge `AppLayout.tsx:330-420`. «Meine Medien» wird vom Server geordnet: `MyMediaKey: [library, radio, collections,
  picks]` (`openapi.yaml:19956-19961`; Abbildung `AppLayout.tsx:281-322`): nächstliegender Platz für einen Schlüssel `livetv`.
  Handy und Tablet: Grenze 768 px, Schublade statt Spalte (`lib/useWideScreen.ts:4`). `RadioDock` unter jeder Seite
  (`app/radio/RadioDock.tsx`); `app/radio/priority.ts` regelt «nie zwei Töne zugleich» und müsste Live-TV als dritte Quelle aufnehmen.
- **Bauteile (125, alle mit Stories, `e2e-storybook/components.spec.ts:50`):** wiederverwendbar sind `Row`, `StationTile`,
  `StationLogo`, `MiniPlayer` (Zustände `starting/live/shifted/paused/error`), `TimeShiftSlider`, `DataTable`, `EditorDialog`,
  `Switch`, `SettingRow`, `SaveBar`, `RankedChoiceList`, `BandPanel`, `StorageMeter`, `StreamTile`, `FilterSheet`, `Skeleton`, `Notice`.
  `TimeShiftSlider`: das Fenster ist ein Parameter (`maxSeconds`, `ui/TimeShiftSlider.tsx:17-20`), «−30:00» ist nur der
  Vorgabetext; `startLabel` ist Parameter (V1, Korrektur 5). **Fehlen:** ein **Zeitraster** (Sender × Zeit, zweiachsig,
  «Jetzt»-Linie, Zellen nach Dauer; nahe Verwandte `HeatGrid.tsx`, `SeekBar.tsx`, `VirtualList.tsx`), eine Karte «Sendung
  jetzt» mit Fortschritt und Sender, die Symbole Uhr/Kalender, Aufnahme-Punkt, Wecker und Antenne.
- **Tokens:** `packages/design-tokens/` mit `tokens/color.json`; **nur dunkel** (`web/src/ui/global.css:3`
  `color-scheme: dark`). Für Live-TV fehlen Farben: Aufnahme-Rot, «Jetzt»-Linie, vergangene Sendung, Konflikt (Regel 9).
- **Video-Player:** `PlayerPage.tsx` → `VideoPlayer.tsx` (1504 Zeilen) → `ui/PlayerChrome.tsx` (1020). hls.js als eigener
  Chunk mit `recoverMediaError` und `startLoad` (`VideoPlayer.tsx:735-792`); **kein Live-Modus im Video-Player** (kein
  `live`/`liveEdge`, `liveSyncDuration` nicht konfiguriert). Tastatur `features/player/keymap.ts:21-23`, `:78-103`:
  Ziffern 0–9 = Prozentsprung, N/P = Folge vor/zurück, ↑/↓ Lautstärke, ←/→ ±10 s: **Kollision** für Senderwahl per Ziffer
  und Kanal ±. Der Live-Modus für Audio ist gebaut (`app/radio/engine.ts:1-5`: fMP4-Fenster mit PDT, Zeitversatz bis
  `RadioListen.timeShiftSeconds`): Vorbild für Video-Timeshift.
- **Seitenmuster:** `RadioPage.tsx` (697 Zeilen) mit `SearchField`, `FacetSelect`, `ActiveFilters`, `VirtualList`, Filter im URL ist die
  Vorlage der Senderübersicht; Detailseite `ItemPage.tsx` mit `RoundAction` für Sendungsdetails; Konten-Formular
  `features/admin/accounts/AccessForm.tsx` (Altersstufe, Bibliotheken, Grenzen, «Darf anfordern») und Profilformular
  `features/profiles/ProfileForm.tsx:259` sind das Muster für die Rechteschalter. Suche und Merkliste kennen nur Bibliothekstitel.
- **Service Worker (V1, L-3):** `web/vite.config.ts:123-160`: Precache nur Shell (`js|css|html|svg|png|webmanifest`),
  «never media or API answers». `runtimeCaching`: Bilder `CacheFirst` (nur `/api/v1/items/…/images/`, Personen,
  Sammlungen), **alles andere unter `/api/` ist `NetworkOnly`**; `navigateFallbackDenylist` `/api/`, `/spike/`. Neue Routen
  unter `/api/v1/` (Live-Playlist, Segmente, EPG) sind damit automatisch ungecacht. **Senderlogos werden nicht gecacht**
  (Radio-Logos laufen über `radioImagePath`, nicht über das Muster `items/…/images`); eine Senderliste mit mehreren
  hundert Logos lädt sonst jede Seite neu. Offline spielt nur der OPFS-Speicher der Downloads; Live ist offline ohnehin nicht möglich.

### 3.22 Android-Handy-App

Rahmen `shell/AppShell.kt:119-131` (untere Leiste, Menü über den Menüknopf, ab 700 dp feste Seitenspalte); fünf Ziele in
`shell/Registry.kt:36-43` (Start 10, Suche 20, Downloads 30, Wünsche 40, Mehr 50; zwei gleiche `order` sind ein Fehler),
Medieneinträge als `MediaEntry` (`Registry.kt:52-84`; Webradio `audio/AudioFeature.kt:83-86`, `order = 90`), Features in
`shell/Features.kt:28-44`. Senderliste nach dem Muster `audio/RadioScreen.kt` (`:103`, `:166`, `:237`, `:288`). Player
`player/PlayerScreen.kt` mit Blättern (`:114`); Kern `core/player/` (`Media3Engine.kt` HLS `:153`, `RadioController.kt`
lädt an `PlayerEngine.LIVE_EDGE` `:229`). Bauteile in `core/design/`; Texte werden aus `web/src/locales` erzeugt, neue
Namensräume stehen in `android/strings-namespaces.txt`. **Fehlen:** Zeitraster, Sendungszeile mit Zeit, Aufnahme-Symbol,
Live-Video-Steuerung (Senderwechsel-Blatt, «Live»-Knopf, Zeitversatz-Fenster). Pflichttest je Seite: `XScreenTest`
(Robolectric, zwei Breiten, 200 % Schrift; 25 vorhanden).

### 3.23 Android-TV-App (Tasten)

Rahmen `shell/TvShell.kt:96-102` (Leiste links, Uhr oben rechts); Leiste `shell/Registry.kt:26-43`, Webradio als `RailEntry`
in `RailSection.Media` (`audio/AudioFeature.kt:100-112`, `RADIO_ORDER = 1_000`). Fokus: `ui/TvComponents.kt:145-153`,
Overlays `ui/TvOverlays.kt` (`TvSideMenu` von rechts). Player `player/PlayerScreen.kt:142-152`: OK = Pause/Play, ←/→
spulen, ↑/↓ Bedienung zeigen, Zurück schliesst Info → Bedienung → Seite; Menüs von rechts (`PlayerMenus.kt:63-146`).

**Teilweise gebaut:** `TvRemoteKeys` (voller Pfad `android/app-tv/src/main/kotlin/com/slouchcat/app/tv/ui/TvRemoteKeys.kt:39-46`)
kennt nur `MEDIA_PLAY_PAUSE`, `PLAY`, `PAUSE`, `STOP`, `NEXT`, `PREVIOUS`, `FAST_FORWARD`, `REWIND`; Ziffern gibt es nur in der PIN-Eingabe
(V1 §2.9). **Fehlen:** `CHANNEL_UP/DOWN`, `GUIDE`, `INFO`, `LAST_CHANNEL`, `DVR`, Ziffern 0–9 im Player, eine
Senderleiste über dem laufenden Bild (OK im Bild öffnet heute Pause, nicht Liste) und ein Info-Banner «Jetzt / Danach».
↑/↓ und ←/→ sind im Player belegt, eine Zapping-Belegung kollidiert damit. Tests: `Dpad.kt:31-47` (`assertFocused`,
`assertEveryControlReachable`), `RadioScreenTest.kt`, `PlayerScreenTest.kt`, `PlayerFlowTest.kt`.

### 3.24 Mockup-Baukästen

| Baukasten | Aufbau | Lücke für Live-TV |
|---|---|---|
| Handy (K27), `docs/planning/mockups/27-android-handy/BAUKASTEN.md` | reines HTML/CSS/JS, Router `route('/pfad/:p', …)`, Zustand `S`/`ui`, Aktionen `data-act`, Geräterahmen 412 × 915 und 1280 × 800 | Senderlogos (nur zwei `r-`-Dateien), EPG-Daten |
| TV (K26), `…/26-android-tv/BAUKASTEN.md` | Bildfläche 1920 × 1080, nur Fokus (`data-f`, `data-row`), Tastenkarte `KEYMAP` (`js/core.js:355`), Fernbedienung nur `up down left right ok back home play` | Kanaltasten, Ziffern, Guide, Info; `core.js` und `index.html` sind gesperrt, Nachrüstung nur in einer eigenen Datei |
| Web, `…/25-medienwuensche/` | vollständigste Hülle (Zuschauer und Verwaltung), Verwaltungsbereiche `AREAS` (`js/shell.js:35-42`), `band()` standardmässig zu | Live-Daten |
| Leinwand-Bretter (`12-verwaltungsseiten`, `19-radio` …) | `*.dc.html` mit `canvas.json` | nur Stilreferenz |

Die **Bilder** der Mockups (`img/`) sind nicht im Repo (`.gitignore`), der Ordner ist leer; die echten Dateien liegen nur
lokal (aus CT 190). Senderlogos, Programmdaten und Testlisten echter Anbieter dürfen nicht ins öffentliche Repo
(RI-5).

### 3.25 Tests

- **Playwright-Tastaturtest** je neue Web-Seite (`CLAUDE.md` Regel 8; `web/e2e/keyboard.ts:25-129`, Vorbild
  `web/e2e/radio.spec.ts:69-147`, Tag `@keyboard` für Firefox, `web/playwright.config.ts:44-48`); Wiedergabe-Specs laufen ganz
  in Firefox. Testserver mit Fake-Upstream für Radio (`web/e2e/radio-server.mjs`); eine Fake-IPTV-Quelle (M3U, XMLTV) wäre neu.
- **Storybook:** jedes Bauteil in `web/src/ui` braucht Stories, Bildvergleich je Story (548 Bilder), Lauf über
  `task storybook:test`.
- **Handy:** `XScreenTest` je Seite (Regel 8). **TV: Regel 8 nennt keinen Pflichttest**, nur «Handy-App»; im Code gibt es für
  die TV-App Ablauftests (`TvFlowTest.kt`) und `Dpad`-Tests. K26 hatte «je TV-Seite ein Steuerkreuz-Test» vorgeschlagen
  (`26-android-tv.md:823`), in `CLAUDE.md` steht er nicht (A1 §5.1; Frage Q-16).
- **Verträglichkeit:** `TestXClientsCompatibility` und `TestContractCoversEveryOperation` (siehe 3.17).

## 4. Annahmen des Auftrags gegen die Wirklichkeit

Der Auftrag nimmt viele Dinge als «bestehend» an. Urteil nach Doku und Stichprobe im Code (A1 §4, mit V1-Korrektur).

| Nr. | Annahme | Urteil | Beleg | Bemerkung für Live-TV |
|---|---|---|---|---|
| R-1 | Limits je Konto für gleichzeitige Streams | **gebaut für Titel, nicht für Sender** (V1) | `00063_…:41`; `00061_…:49`; `playback/limits.go:159-197` | zählt nur `playback_sessions` von Titeln; Radio zählt nicht (F-19) |
| R-2 | Limits für Bandbreite | gebaut (für Titel) | `00063_…:39`; `decide.go:392`, `:549`, `:569` | |
| R-3 | Limits für Auflösung | gebaut (für Titel) | `00063_…:40`; `decide.go:391` | eine Grenze unter der Quelle erzwingt einen Transcode je Zuschauer (`decide.go:391-426`) |
| R-4 | Transcoding-Berechtigungen | **fehlt** | keine Spalte; nur gemeinsames Budget (`transcode/download.go:207-223`) | ein Recht «darf umwandeln» wäre neu |
| R-5 | Lokale und externe Zugriffsbeschränkungen | teilweise gebaut | K-8; `config/config.go:57-59`, `:241`; `api/session.go:449-474` | **kein** Recht «nur im Heimnetz»; Erkennung hinter Reverse-Proxy unzuverlässig |
| R-6 | Geräteverwaltung | gebaut | `openapi.yaml:5839`, `:5865`, `:11127`; `00082_session_kind.sql:14` | |
| R-7 | Statistiken | gebaut (nur Titel) | `admin/stats.go:16-43`; `openapi.yaml:8645` | Live-TV kommt nicht von selbst hinein |
| R-8 | Systemüberwachung CPU/GPU/RAM | **teilweise gebaut** | `openapi.yaml:7251`; `admin/system.go:44-50` | keine Lastwerte |
| R-9 | Backup und Wiederherstellung | gebaut | `admin/backup.go:218`, `:538` | Medien nie im Archiv: Aufnahmen nicht, ihre Zeilen schon |
| R-10 | Installer | teilweise gebaut | `docs/INSTALLATION.md`; `deploy/compose.*.yaml` | neue Compose-Overrides sind der hiesige «Installer» |
| R-11 | Native Linux-Installation (`.deb`) | **fehlt, Nicht-Ziel** | Spec `:41`, `:86` | bleibt draussen |
| R-12 | Einrichtungsassistent | gebaut | `admin/setup_wizard.go:93-160` | ein Schritt «Live-TV» änderte K22 |
| R-13 | Favoriten | teilweise gebaut | `openapi.yaml:6187`, `:11685`, `:11708` | für Sender Vorbild Radio-Favoriten; Titel-Favoriten entfallen |
| R-14 | Suche | gebaut (Titel, Personen, Sammlungen) | `openapi.yaml:5951`; `discover/m7.go:20` | keine Sender, keine Sendungen |
| R-15 | Kinderprofile und Hauptprofil | gebaut, **anders benannt** | `00061_…:39`, `:43`, `:73`; `api/session.go:643` | Kind = Altersgrenze unter 18; «Hauptnutzer» ist mehrdeutig |
| R-16 | Benachrichtigungssystem | gebaut | `notify/dispatch.go:39`; `model/notify.go:36-38` | neue Ereignisse brauchen Neubau der CHECK |
| R-17 | Fire TV | **fehlt, Nicht-Ziel** | Spec `:58`; K26 E-1 | Auftrag widerspricht Spec |
| R-18 | Weitere Clients | gebaut: Web-PWA, Android-Handy, Android TV; Nicht-Ziel: iOS, Apple TV, webOS, Tizen | Spec `:33`, `:58-60` | Apps unter `VERTRAEGLICHKEIT.md` |

## 5. Regeln, die den Entwurf binden

### 5.1 Die zwölf harten Regeln (`CLAUDE.md`)

| Regel | Folge für Live-TV |
|---|---|
| 1 Generierter Code tabu | Neue Operationen nur über `api/openapi.yaml` und `task gen`, neue Tabellen über Migrationen und `server/queries/`. |
| 2 Titelabfragen nur über `visibility` | Entscheidend ist, ob Sender, Sendungen und Aufnahmen Titel (`items`) sind. Ausserhalb von `items` deckt der Analyzer sie nicht ab; es braucht einen eigenen `depguard`-Riegel wie `certq`/`wishq`. Nicht sichtbar heisst 404, nie 403. «Aufnahmen privat je Profil» verlangt Sichtbarkeit **je Profil**, die `visible_items` nicht kennt. |
| 3 `os/exec` nur in `transcode` | Jeder ffmpeg-Aufruf für Empfang, Umpacken und Aufnahme liegt in `internal/transcode`, `exec.CommandContext` mit Argumentliste. Öffnet ffmpeg das Netz selbst, umgeht es `netguard`; Radio lässt ffmpeg nie eine URL sehen. |
| 4 Dateizugriff nur über `paths` | Aufnahmen und Timeshift-Dateien liegen unter einer registrierten Wurzel; ein Aufnahmeordner braucht einen schreibbaren Override wie `compose.podcasts.yaml`. Spec B-1 «read-only» würde ein zweites Mal teilweise aufgehoben. |
| 5 Migrationen nur vorwärts | Neue Tabellen nach Vorlage, keine Down-Migration, eingefrorene Stände. Rechte als Spalten nach Vorbild `can_request`. |
| 6 Streaming als `http.Handler` | Live-HLS-Listen, Segmente, Logos, Aufnahme-Wiedergabe als `http.Handler` mit Beschreibung im Vertrag (Vorbild `radioLivePlaylistPath`, `api/api.go:58`). |
| 7 `x-visibility`, `x-clients` | Jede Operation trägt beides, jede mit Eintrag im Vertragstest. Der Kinderprofil-Vertragstest (Plan 10.2) verlangt «keine ID ohne Freigabe»: erscheinen Aufnahmen als `ItemRef`, bräuchte «Kinder ohne Altersprüfung» eine benannte Ausnahme. |
| 8 Tastaturtest, `XScreenTest` | Senderliste, Programmzeitschrift, Aufnahmen und Verwaltung brauchen je einen Playwright-Test; jede Handy-Seite einen `XScreenTest`. Für die TV-App gibt es keine Pflicht: Lücke. |
| 9 Keine Farbe, kein Text im Code | Texte aus `web/src/locales` (Namensraum `live`; `tv` ist belegt), Farben aus Tokens. Sendernamen, Sendungstitel und Beschreibungen aus M3U und XMLTV sind **Daten**; die Ausnahme müsste benannt werden wie bei Sprachnamen und Freigabebezeichnungen (`CLAUDE.md` Regel 9). |
| 10 Keine Secrets in Logs | Neue Schlüsselnamen in `internal/logging` und den Test; heute fehlen `username`, `password`, `user:pass@` und Xtream-Pfade (siehe 3.9). Zugangsdaten von IPTV-Anbietern landeten sonst im Log. |
| 11 Keine Nicht-Ziele | Die Ausnahme muss vor dem Bau stehen. Fire TV, `.deb`, YouTube-Livestreams als Quelle bleiben verboten; ein eigener Dienst im Internet für Live-TV wäre eine dritte Cloud-Ausnahme. |
| 12 Entscheid im Server | Rechte, Kinderfreigabe, Kontingent und Altersfilter entscheidet der Server; Apps zeigen nur an. Offline nur Nachschlagen. |

### 5.2 Einschlägige ADR

| ADR | Bindet | Folge für Live-TV |
|---|---|---|
| 0001, 0002/0017 | Go, SQLite mit einer Schreibverbindung, eigene Queue | Kein PostgreSQL, Redis, BullMQ. Jobs kennen Pause und Abbruch je Jobart (ADR 0017). |
| 0003, 0004, 0021, 0022 | HLS-Segmenter, hls.js, Budget der Umwandlungen, Keyframe-Index | Live-HLS ist ein neuer Fall neben VOD und Radio; das Budget teilen sich Filme, Downloads und Live. |
| 0005, 0014 | Profilwahl, drei Ebenen | Rechte setzt der Administrator (Konten) und das Hauptprofil (Profile); ein Kinderprofil verwaltet nichts. |
| 0009 | Sicherung | Medien nie im Archiv. |
| 0024 | Updater | ersetzt den Container ohne Prüfung auf Aufnahmen. |
| 0025 Webradio | Ausnahme vom Nicht-Ziel, Relay, Zeitversatz nur im RAM, `netguard`, Standort | Nächstes Vorbild. **F-4** «Puffer nur im Arbeitsspeicher», **F-15** «keine Sender im lokalen Netz», **F-5** «alle Profile, ohne Schalter» binden nur Radio, widersprächen aber dem Auftrag, falls übernommen. |
| 0029 | schreibbare Podcast-Ordner | Vorbild für den Aufnahmeordner. |
| 0032 | Dateien löschen nur auf Befehl, nichts löscht automatisch | Aufbewahrungsfrist und «behalte N Folgen» löschen automatisch: benannte Ausnahme (Auslegung vom 28. November 2026). |
| 0037, 0045 | Rechte: Konto ist der Deckel; Recht folgt der Altersgrenze | ADR 0045 verbietet ein Recht mit Altersgrenze; der Auftrag will das Gegenteil (LTV-015). |
| 0038 | Titel ausserhalb der Bibliothek | Muster für eigene Tabellen und 404. |
| 0040 | Sitzungsarten, `x-clients` | Neue Live-Operationen brauchen die Erlaubnisliste `app`/`tv`. |
| 0041 | Ton- und Spurfähigkeiten | Live-Ton (AC-3, MPEG-2) fällt unter denselben Entscheid. |
| 0042, 0044 | zwei benannte Dienste im Internet | Kein dritter Dienst ohne neue Ausnahme. |
| 0043 | Offline in der Android-App | Falls Aufnahmen herunterladbar würden. |

## 6. Widersprüche und Risiken

Ohne Lösungen; die Antworten gehören in die Fachkonzepte und in `20-offene-entscheidungen.md`.

### 6.1 Widersprüche

| Nr. | Widerspruch | Belege |
|---|---|---|
| W-1 | Der Auftrag beschreibt NestJS, PostgreSQL, Redis, BullMQ, `.deb`, DEC-001, [MVP]-Marken. | `docs/adr/0001-stack.md:32-35`; Spec `:41` |
| W-2 | Fire TV und `.deb` als «bestehend», beides Nicht-Ziel. | Spec `:58`, `:41`, `:86`; K26 E-1 |
| W-3 | Lokale Quellen (Tuner, Multicast) gegen «keine Ziele im Heimnetz». | ADR 0025 F-15; `netguard.go:48-50`; `radio/own.go:91` |
| W-4 | «Kinderprofile ohne Altersprüfung» gegen K-3, AN-1 («ohne bekannte Freigabe für alle Profile unter FSK 18 gesperrt»), Plan 10.2 und ADR 0045. | Spec `:97`; `03-plan-v1.md:673-674`; `0045…:23-28` |
| W-5 | «Freigabe durch den Hauptnutzer» gegen das Nicht-Ziel «Freigabe-Anfragen von Kinderprofilen». | Spec `:67`, `:194`, `:316` |
| W-6 | Kontingent und «nur die letzten N» löschen automatisch; das Repo erlaubt Löschen nur auf Befehl. | ADR 0032; Spec B-1 Nachtrag `:105` |
| W-7 | Persönliches Video-Timeshift über Minuten bis Stunden gegen den RAM-Puffer des Radios (kein Regelbruch, aber das Vorbild trägt nicht). | ADR 0025 §2–3; `deploy/compose.yaml:37-40` |
| W-8 | «Aufnahmen privat je Profil» gegen K-5 (Verlaufsbericht für den Hauptbenutzer) und W-6 (Administrator sieht Titel in «aktive Streams»). | Spec `:99`, `:127` |
| W-9 | Zugangsdaten in IPTV-Adressen gegen Regel 10. | `logging/logging.go:75`, `:144-160` |
| W-10 | Altlasten unabhängig von Live-TV: Plan `:30` ohne Webradio-Ausnahme; `HANDOVER.md:155` mit «native Apps»; Vision `:70`; Spec `:43` («Genau ein Client in Version 1»). | jeweilige Zeile |
| W-11 | Drei Etikett-Unterschiede zwischen den Berichten (Grenzen je Konto «gebaut»/«teilweise», Systemüberwachung, Puffer) sind keine Sachwidersprüche; ein Konzept soll sie nicht übernehmen (V1 §3). | V1 §3 |
| W-12 | ADR 0025 `:82` und `:173` («nichts auf die Platte») gilt ausdrücklich nur für Radio; ein Video-Puffer auf Platte kollidiert mit keiner Regel, aber mit dieser Aussage. | V1 §3 Punkt 3 |

### 6.2 Risiken

| Nr. | Risiko | Beleg |
|---|---|---|
| RI-1 | Multicast und UDP kommen in Dockers Bridge-Netz nicht an; eine Netzänderung berührt Caddy, `compose.https-only.yaml` und die Anleitung. | `deploy/compose.yaml:30-31` |
| RI-2 | Eine Schreibverbindung gegen grosse Programmzeitschriften (3–4 Mio. Zeilen geschätzt); Sicherung kopiert das EPG mit. | ADR 0001; A3 §3 |
| RI-3 | Aufnahmen sind zeitkritisch; die Queue kennt Pause je Jobart und Abbruch; ein Administrator pausiert Aufnahmen versehentlich mit. | ADR 0017; `worker.go:149-161` |
| RI-4 | Geteilter Empfang gegen Grenzen je Zuschauer: eine Höhengrenze unter der Quelle erzwingt je Zuschauer einen eigenen Transcode. | `decide.go:391-426` |
| RI-5 | Öffentliches Repo: Testlisten, Logos und Programmdaten echter Anbieter dürfen nicht hinein. | Konzept 25 «Bilder» |
| RI-6 | Neue Aufzählungswerte (`ItemKind`, `HomeRowKey`, `LibraryKind`) gelten als nicht brechend, ohne dass ein Test belegt, dass installierte Apps sie vertragen. | `VERTRAEGLICHKEIT.md:60-64`; `openapi.yaml:14489-14491`, `:15126`, `:17813` |
| RI-7 | Regel 8 deckt die TV-App nicht ab; die Programmzeitschrift am Fernseher bliebe ohne Pflichttest. | `CLAUDE.md` Regel 8 |
| RI-8 | Remux-Eingang ohne Keyframe-Index; TS-Zeitstempel springen bei Senderwechsel, PCR-Überlauf und Abbrüchen; die Dateikette setzt eine monotone Quelle voraus. | `plan.go:274-278`; `pipeline.go:176-186` |
| RI-9 | Ein Live-Zweig in `Session` und `ffmpegArgs` gefährdet die Abstimmung der Dateikette. | ADR 0003 Nachtrag |
| RI-10 | `nice 10` und `oom_score_adj 500` treffen Empfänger und Aufnahme zuerst. | `session.go:702-712` |
| RI-11 | Jeder Updater-Lauf und jeder Shutdown (10 s, SIGKILL danach) unterbricht laufende Aufnahmen; Format muss einen harten Abbruch überleben. | `compose.yaml`; `main.go:656-664`; `update_install.go` |
| RI-12 | Kein Umgang mit `ENOSPC` und keine Platzprüfung für Transcode und Puffer; ein voller Cache beendet die Sitzung mit Fehler. | `transcode/session.go:421-428` |
| RI-13 | Segment-URLs sind Bearer-URLs und landen in Verlauf und Proxy-Logs; Token ohne Ablauf. | V1 L-1, L-2 |
| RI-14 | Benachrichtigungen gehen an alle Kanäle des Kontos; «privat je Profil» ist damit nicht gewahrt. | `notify/dispatch.go:103-110` |

## 7. Prüfvermerk

Die Analyse (Berichte A1 bis A4) wurde von einem zweiten Agent unabhängig am Code geprüft (Bericht V1, Stand `1220769b`, nur
gelesen). **100 Aussagen geprüft: 95 bestätigt, 5 ungenau, 0 falsch.** Je Bericht: A1 18 von 20, A2 32 von 33,
A3 27 von 28, A4 18 von 19. Die ungenauen Stellen ändern keinen Entwurfsentscheid; V1 geht A1–A4 vor.

### 7.1 Die fünf Korrekturen

1. **A2 §0 Punkt 3** («Jede Argumentliste nimmt einen absoluten Pfad oder eine Pipe») ist nicht ganz richtig. Zwei weitere
   Eingänge ohne Pfad: `-f lavfi -i color=…` in `transcode/backend.go:285` und `:292` (Testbild der Hardware-Erkennung, kein
   Netz) und die Concat-Liste (`transcode/pipeline.go:109`). Die Kernaussage bleibt: **keine Netzadresse in irgendeiner
   Argumentliste**. Zusatz: «Nur Bilder und Radio setzen `-protocol_whitelist`» stimmt; für Live-TV zählt, dass **jeder** `os/exec`-Aufruf
   ausser Bild und Radio ohne Whitelist läuft.
2. **A1 §2.2** («`Konzept 28` kommt nirgends vor»): seit dem Auftrag nicht mehr. `docs/planning/28-live-tv/anhang/auftrag-2026-11-28.md`
   nennt «Konzept 28»; die Nummer ist durch dieses Konzept belegt. Frei sind Frage 37, ADR 0046, Migration 00087, Vertrag 1.45.0.
3. **A1 R-1 «Limits je Konto: gebaut» gegen A3 «teilweise»** ist nur ein Etikettunterschied. Zu schreiben ist: **gebaut für
   Titel, nicht für Sender** (Konto- und Profilgrenzen für Streams, Bitrate, Höhe sind gebaut; es fehlen lokal/extern,
   globale Grenzen und eine Zählung, die Live einschliesst).
4. **A3 §5** («`notifications` mit `item_id` bzw. `wish_id` Pflicht») ist falsch gefasst: richtig ist «mindestens eines von
   beiden» (`CHECK (item_id IS NOT NULL OR wish_id IS NOT NULL)`, `00070_wishes_notifications.sql:32-43`). Folge für Live-TV
   unverändert: eine dritte Art braucht den Neubau der Tabelle, weil `kind` und die CHECKs geschlossen sind.
5. **A4 §1.3** («`TimeShiftSlider` −30:00 bis live»): das Fenster ist ein Parameter (`maxSeconds`, `web/src/ui/TimeShiftSlider.tsx:17-20`;
   Radio-Fenster 10/30/60 min, `radio/config.go:61`); «−30:00» ist nur der Vorgabetext. Das Bauteil ist für Video-Timeshift
   besser wiederverwendbar als beschrieben. Dazu: der Pfad von `TvRemoteKeys` lautet `…/com/slouchcat/app/tv/ui/TvRemoteKeys.kt`.

### 7.2 Die fünf nachgetragenen Lücken

| Lücke | Inhalt | Abschnitt |
|---|---|---|
| L-1 | Segment-Endpunkte prüfen **nur das Token `st`**, nicht Cookie oder Bearer (`streamAuth`/`hlsAuth`); ein Kanal-Token braucht eigene Zeile und eigenen HMAC-Inhalt; Entzug nur über Einhängen in `EndSession/EndProfile/EndAccount`; neuer Parametername in `sensitiveQuery`. | 3.17 |
| L-2 | Die Web-App authentisiert Medien-URLs per `?st=…` ohne Header; alle Live-Segment-URLs sind Bearer-URLs (Verlauf, Proxy-Logs, Netzwerk-Tab). | 3.17 |
| L-3 | Der Service Worker lässt `/api/` unberührt (`NetworkOnly`), cached aber Senderlogos nicht; Logos brauchen ein eigenes `CacheFirst`-Muster. | 3.21 |
| L-4 | Die Reihenfolge beim Herunterfahren: Server-Shutdown 10 s, dann Sweeper (inklusive Radio-Relays), dann `closeTranscoder`, Jobs, Aktivität, DB; ein Live-Dienst gehört in den Sweeper-Block vor `closeTranscoder`; ein Docker-Kill kommt nach 10 s. | 3.11 |
| L-5 | Weitere Funde: Test-Variante des Dialers für e2e, `netguard.Clients.For` hängt am Feed-Origin (keine Herkunftsangabe für Live-Quellen), Hauptkette ohne `-protocol_whitelist`, kein `init`/`tini` im Image, Tageslimit (`watchTime`) hängt an `items`/`progress_days`. | 3.8, 3.11, 3.19 |

### 7.3 Was nicht neu geprüft wurde

`docs/perf/*`-Messwerte, `docs/adr/0041`-Zeilen, die FTS-CHECK-Liste in `00015_discover.sql`, Android-Lintregeln und der
Mockup-Baukasten wurden nicht neu geöffnet; die Aussagen sind nach Stichprobe plausibel. Die Rechercheberichte R1 bis R5
waren nicht Gegenstand der Prüfung. Die ffmpeg-Fähigkeiten des Images sind seit der Messung M1 (Image 1.41.3,
FFmpeg 7.1.4-Jellyfin) belegt; sie sind keine Aussage über die Praxis einzelner Quellen. Die Grössenordnung des EPG (3 bis 4
Millionen Zeilen) ist eine Schätzung, keine Messung.
