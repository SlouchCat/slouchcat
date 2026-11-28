# Konzept 28 · Client-Kompatibilität

Stand: 28. November 2026 · Fassung 1 · Status: Entwurf, nicht freigegeben

Dieses Dokument beantwortet die Forderung von LTV-003: eine Kompatibilitätsmatrix für Eingangsprotokolle,
Container, Codecs und unterstützte Clients. Es verdichtet `04-architektur.md` und
`05-streaming-und-fast-zapping.md` (beide **Fassung 2**, verbindlich), `24-recherche-und-quellen.md` und die
Recherche R1 bis R3. Es ändert keinen Entscheid (T-1 bis T-28) und erfindet keinen Befund. Wo R2 und `05`
sich widersprechen, gilt `05` Fassung 2; die Widersprüche stehen in Abschnitt 10.

## 0. So liest du die Tabellen

**Alles in Konzept 28 ist «spezifiziert», nichts ist gebaut.** Jede Zelle trägt zusätzlich eine
Verlässlichkeit:

| Marke | Bedeutung |
|---|---|
| **belegt** | In der Recherche am 28. November 2026 aus einer abgerufenen Quelle entnommen (Media3-Dokumentation, hls.js-API, mpegts.js-Beschreibung, MDN) oder im Repo gelesen (Datei im Text genannt), oder im Image gemessen (`M1`). |
| **Annahme** | Aus Wissen, aus einer nicht abgerufenen Quelle oder aus einer Überlegung. Kein Befund. |
| **zu messen (M-n)** | Nur ein Gerätetest klärt es; die Nummer verweist auf die Messpunkte in Abschnitt 9. |

Nichts ist stärker gemacht als die Unterlage. Zielwerte sind zu prüfende Grössen, keine Zusagen.

**Die drei Wege des Servers** (`04`, T-14; Vorgänge heissen dort «Original», «Ton-Umwandlung» und
«Bild-Umwandlung»):

| Ergebnis in den Matrizen | Was der Server tut | Kosten |
|---|---|---|
| **direkt** | Bild und Ton bleiben, wie sie sind. Go schneidet den Strom nur in Segmente (kein FFmpeg-Prozess). | keine im Umwandlungs-Budget |
| **nur Ton umwandeln** | Bild wird kopiert, der Ton mit FFmpeg neu kodiert (meist zu AAC). | Satz 0,25 (**Annahme**, M-8) |
| **Bild umwandeln** | FFmpeg kodiert das Bild neu (auch zum Entfernen des Zeilensprungs, zum Einbrennen von Untertiteln, zum Anpassen von HDR). | Satz 1,5 bis 6, HD ist 2 (`04` 7.6) |

**Begriffe in einem Halbsatz.** *Zeilensprung* (interlaced, 1080i, 576i): Das Bild besteht aus zwei
ineinander verschränkten Halbbildern; ohne Aufbereitung zeigen sich Kammartefakte, *Deinterlacing* entfernt
sie. *Segment*: ein kurzes Stück des Stroms, aus dem eine HLS-Liste besteht. *MSE* (Media Source
Extensions): die Schnittstelle, über die ein Browser-Player Stücke zum Abspielen übergibt. *Media3*: der
Android-Player der beiden Apps. *PID*: die Kennung einer Tonspur oder eines Untertitels im MPEG-TS.
*LATM*: eine Verpackung für AAC-Ton, wie sie DVB nutzt. *DRM*: Kopierschutz mit Lizenzserver.

## 1. Die Clients

| Client | Player | Wahl der Wiedergabe | Verlässlichkeit |
|---|---|---|---|
| **Web: Chrome, Edge, Firefox, Chrome Android** | `hls.js` 1.7.3, heute als `hls.js/light` geladen, über MSE | `web/src/features/player/capabilities.ts`: «mse» überall, wo klassisches `MediaSource` existiert | belegt (Repo: `web/package.json`, `VideoPlayer.tsx`, `capabilities.ts`) |
| **Web: Safari macOS** | `hls.js` über MSE (`05` 4.3); natives HLS als Alternative | wie oben | `05` spezifiziert; Messung M-5 |
| **Web: Safari und installierte App (PWA) auf iPhone und iPadOS** | natives HLS (`05` 4.3). Das iPhone hat kein klassisches `MediaSource`; `ManagedMediaSource` gibt es ab iOS 17.1 (Ausweg: `hls.js`). | iPhone: «native» nur, wo es allein `canPlayType('application/vnd.apple.mpegurl')` gibt | iPhone belegt (`capabilities.ts`-Kommentar, mpegts.js-Beschreibung). **iPadOS: Annahme**, siehe Abschnitt 10, Punkt 9 |
| **Android-Handy-App** | Media3 1.11.1, `ExoPlayer` mit `SlouchcatRenderersFactory`, OkHttp | `android/core/player/.../Media3Engine.kt`; setzt heute weder `LoadControl` noch `LiveConfiguration` | belegt (Repo) |
| **Android-TV-App** (deckt Google TV ab) | Media3 1.11.1, wie Handy | wie Handy; wertet `DecoderReuseEvaluation` schon aus (`PlayerFeature.kt`) | belegt (Repo) |

**Nicht dabei** (Nicht-Ziele nach `CLAUDE.md`, Regel 11): Fire TV, Apple TV, webOS, Tizen. Zu Fire OS nur
eine **Annahme** aus R2 (Amazon-Dokumentation nicht abgerufen): Es ist ein Android-Fork, auf dem APKs und
Media3 laufen; ob die Android-TV-App dort unverändert startet, hängt am Leanback-Manifest, an fehlenden
Google-Diensten (FCM, Anmeldung) und am Amazon-Appstore. Kein Ziel, höchstens ein Nebeneffekt.

## 2. Matrix A · Eingangsprotokoll

Wie der Server die Quelle holt, gilt unabhängig vom Client. Sie steht hier, weil sie bestimmt, was der
Client überhaupt zu sehen bekommt: Alle Wege liefern denselben Rohstoff, **einen TS-Bytestrom mit genau
einem Programm** (`05` 2.1). Nichts davon ist gebaut.

**Die drei Empfangswege (T-3):** **G** = Go liest selbst, **L** = Go mit einer Protokoll-Bibliothek,
**W** = FFmpeg liest, aber nur unverschlüsseltes HTTP von einem Go-Weiterleiter auf `127.0.0.1` (T-4).
FFmpeg bekommt nie eine Anbieter-Adresse, nie Zugangsdaten, nie eine Kopfzeile (T-4, `05` 2.6).

| Protokoll | Weg, Stufe (T-3) | Voraussetzungen: Netz und Freigabe lokaler Ziele | Wiederverbindung | Bekannte Grenzen | DRM-Erkennung |
|---|---|---|---|---|---|
| **HTTP-TS** (Dauerstrom, auch Xtream `.ts`) | G, **Stufe 1** | Bridge-Netz genügt (`04` 10.6). Öffentliche Adressen frei; lokale nur als feste Ziele der Quelle mit ausdrücklicher Freigabe (T-5). Weiterleitungen: höchstens 5, jede neu geprüft. | Go, Leiter nach `04` 5.3; Stillstand nach 10 s ohne Daten | Neuer Abnehmer wartet auf einen Keyframe; ohne «Schub» beim Verbinden hängt der Start an der Bildgruppen-Länge der Quelle (`05` 7, M-1, M-2). Eine Neuverbindung erzeugt eine Diskontinuität (Annahme, M-9). | Keine Kennzeichnung im Strom möglich. Verschlüsselte Bild- oder Tonpakete erkennt der Verteiler am Kennzeichen im Paketkopf (nur Bild- und Ton-PIDs, über mindestens 5 s) und meldet «verschlüsselt». |
| **HLS mit TS-Segmenten** | G, **Stufe 1** | wie HTTP-TS; Liste, Unterliste, Segment und Schlüssel sind je eine eigene geprüfte Anfrage | Go lädt die Liste nach; Segmentfehler: nächstes Segment, nach 3 Fehlern in Folge Abbruch | Bei mehreren Qualitäten gilt die höchste (O-14). Zieldauer höchstens 60 s, höchstens 32 Varianten, Unterlisten höchstens 3 Ebenen. Sprung von mehr als 10 Segmenten: Neubeginn am Ende. | Regel `05` 2.5, geprüft bei jedem Nachladen: `AES-128` mit `identity` und `http(s)`-Schlüssel wird gespielt; `SAMPLE-AES*`, fremdes Schlüsselformat, `skd://`, `data:` und `EXT-X-SESSION-KEY` mit diesen Merkmalen werden abgelehnt. Grenzfall `SAMPLE-AES` mit `identity`: vorsichtig abgelehnt (O-16). Die HLS-Spezifikation ist nicht von allen Berichten nachgelesen (W-11): **Annahme**. |
| **Xtream-Live** | G, **Stufe 1** | Abfragen über den geschützten HTTP-Client; Stream-Adresse entsteht erst beim Verbinden, nur aus dem eingetragenen Host | wie HTTP-TS oder HLS | Nur Live. Feldabweichungen zwischen Anbietern sind unbelegt (R3 Abschnitt 3.3, **Annahme**). | wie HTTP-TS oder HLS |
| **UDP** (Unicast) | G, **Stufe 3** | Der Server lauscht, kein Verbindungsaufbau nach aussen, also greift der Dialer nicht. Port veröffentlichen (Zusatzdatei `compose.live-udp.yaml`, Arbeitsname). Feste Ziele der Quelle mit Freigabe; Socket an **eine gewählte** Schnittstelle gebunden, Port ab 1024. | zustandslos; 10 s ohne Paket gilt als Stillstand | Paketverlust bei kleinem Socketpuffer; Absender im selben Netz fälschbar (Verwaltung sagt es wörtlich). Nicht unterstützt in der ersten Stufe mit UDP: TS-Pakete mit 204 Byte, Fehlerkorrektur nach SMPTE 2022 (O-15). | keine Kennzeichnung möglich; Kennzeichen im Paketkopf wie bei HTTP-TS |
| **RTP** | G, **Stufe 3** | wie UDP. Go entfernt den RTP-Kopf und zählt Lücken. **Der Empfänger sendet nichts**, auch kein RTCP. | wie UDP | Keine eigene Sitzungsbeschreibung (SDP), kein Umordnen vertauschter Pakete (O-15; Bedarf nach M-2 und M-16). | wie UDP |
| **Multicast** | G, **Stufe 3** | **Netzmodus «host»** (Zusatzdatei `compose.live-hostnet.yaml`, Arbeitsname) oder eigenes Netz des Containers; im Bridge-Netz kommt der Beitritt nicht beim Heimnetz an (R3 4.3, `04` 10.6). Das ist eine **benannte Lockerung** (F-11, offen). Weg W ist im Netzmodus «host» gesperrt. Beitritt wo möglich nur für eine bestimmte Absenderadresse. | Beitritt wird beim Wiederaufbau erneuert; Verhalten bei IGMP-Snooping ohne Abfrager: **zu messen (M-16)** | Strom ohne IGMP-Abfrager im Switch möglich (Annahme, M-16); WLAN-Multicast schwach (Annahme); mehrere Programme in einem Strom: Go wählt das Programm des Senders (`05` 2.7, O-11). | wie UDP |
| **RTSP** | L, **späte Stufe**, vorbehaltlich F-1 | Go-Dialer; **nur TCP**, kein Rückfall auf UDP; nennt der Server eine andere Zieladresse für den Datenkanal, wird abgelehnt | Go, Leiter | Neue Go-Bibliotheken nötig (F-1, Empfehlung (a)); jede muss nachweislich nur über den übergebenen Dialer verbinden (**zu messen, M-23**). | kein DRM in diesem Protokoll |
| **RTMP** | L, **späte Stufe**, vorbehaltlich F-1 | Go-Dialer (TCP, bei `rtmps` mit TLS) | Go, Leiter | wie RTSP; veraltetes Protokoll (R3, Annahme). | `rtmpe` (verschlüsselt) wird abgelehnt |
| **HLS mit fMP4-Segmenten oder getrennter Tonspur** | W, **eigene späte Stufe, nur nach M-10** | Weiterleiter holt alles selbst über den geschützten Dialer; Ersatznamen `*.src.slouchcat.invalid`; Prozess in einer Kernel-Sperre (Landlock, M-25). Netzmodus «host»: gesperrt. | Go startet den Prozess neu, wenn er abbricht oder 10 s nichts liefert | Ohne belegte Dichtheit entfällt der Weg (F-13); Alternativen: nur mit Sperre, Go liest selbst (viel Eigencode) oder gar nicht (Abstrich an LTV-003, müsste von Marco kommen). | Go liest jede Liste, bevor FFmpeg sie sieht; Regel wie HLS mit TS |
| **DASH ohne DRM** | W, wie oben | wie oben | wie oben | DASH-Leser von FFmpeg für Live-Manifeste: Stabilität **zu messen (M-11)**. | Jedes `ContentProtection` führt zur Ablehnung; die FFmpeg-Optionen zum Entschlüsseln mit vorgegebenem Schlüssel setzt der Server nie |

**Verlässlichkeit der Matrix A:** Die Wege und Stufen sind Entscheide aus `04`/`05` (spezifiziert). Das
Verhalten von FFmpeg stammt aus der Dokumentation der neuesten Version, nicht aus 7.1.4 (W-4); es gilt als
**Annahme**, bis M-10 bis M-12 vorliegen. Belegt (gemessen im Image 1.41.3, `M1`) ist nur, was FFmpeg 7.1.4
kann: Eingangsprotokolle u. a. `http(s)`, `rtmp*`, `rtp`, `udp`, `hls`, `crypto`, Demuxer `dash`, `hls`,
`mpegts`, `rtsp`; **SRT fehlt im Image**, deshalb wird es nicht empfangen (`05` 2.2).

**Was nirgends empfangen wird:** SRT, `rtmpe`, `file`, `ftp`, `gopher`, `mms`, Plattform-Streams (YouTube,
Twitch), alles mit DRM.

**Senderliste und HLS-Liste:** Beim Import unterscheidet Slouchcat eine M3U-Senderliste von einem
HLS-Manifest am Inhalt der ersten Bytes, nie an Endung oder Typangabe (`05` 2.3, R3 1.3).

## 3. Matrix B · Container und Auslieferung je Client

Die Auslieferung ist das, was der Client abspielt: Der Server liefert je Sitzung eine HLS-Liste mit
TS-Segmenten (T-11). Die Tabelle zeigt, was jeder Client kann; die Empfehlung folgt darunter.

### 3.1 Was der Client kann und über welche Bibliothek

| Client | HLS mit TS | HLS mit fMP4 | LL-HLS | Progressives TS (eine lange Antwort) | DASH |
|---|---|---|---|---|---|
| **Chrome, Edge, Firefox, Chrome Android** | `hls.js`: demuxt TS in einem Hintergrund-Thread (`enableWorker`) und packt zu fMP4 für MSE um. **belegt** (hls.js-API) | `hls.js`. **belegt** (gebaut, erprobt bei Filmen und Radio, ADR 0003, ADR 0025) | `hls.js` mit `lowLatencyMode` (Vorgabe `true`). **belegt** | nur mit zweiter Bibliothek `mpegts.js` (MSE); kein MPEG-2, kein Seeking. **belegt** (Beschreibung von mpegts.js) | nur mit zusätzlicher Bibliothek (dash.js, Shaka). **Annahme** |
| **Safari macOS** | `hls.js` (`05` 4.3); nativ ebenfalls möglich. **Annahme** | `hls.js` oder nativ. **Annahme** | nativ. **Annahme** | `mpegts.js`. **Annahme** | nur mit Bibliothek. **Annahme** |
| **iPhone, iPad (Safari, PWA)** | nativ. **Annahme** (Apple-Dokumentation nicht abgerufen); Startregel offen, **zu messen (M-5)** | nativ; HEVC nur in fMP4 (**Annahme**) | nativ. **Annahme** | nur `mpegts.js` ab 1.8 über `ManagedMediaSource`, iOS ab 17.1. **belegt** (mpegts.js-Beschreibung) | kein natives DASH; Bibliothek nur mit `ManagedMediaSource`. **Annahme** |
| **Android-Handy-App (Media3)** | ja: `HlsMediaSource` mit TS-Leser. **belegt** (Media3, `supported-formats`) | ja. **belegt** | nur Apples Form, nicht die der Community. **belegt** | ja: `ProgressiveMediaSource`, aber **ohne Live-Fenster** und nur an einer Position abspielbar. **belegt** (Media3, `live-streaming`) | ja für fMP4 und WebM, **kein TS**. **belegt** |
| **Android-TV-App (Media3)** | wie Handy | wie Handy | wie Handy | wie Handy | wie Handy |

### 3.2 Empfehlung laut T-11 und Alternative

**T-11 ist eine Empfehlung unter Vorbehalt:** M-2, M-3, M-4 und M-5 müssen **vor der Baufreigabe der Stufe 1**
vorliegen (Stufe 1 braucht nicht alle Clients: M-4 gilt für die Stufe, in der die Android-Apps Live-TV
bekommen, `03` F-6). Vorher ist T-11 nicht freigegeben.

| Client | Empfehlung (T-11, `05` 4.3) | Alternative, falls Messungen widersprechen | Zu messen vor der Baufreigabe |
|---|---|---|---|
| **Chrome, Edge, Firefox, Chrome Android** | HLS mit TS-Segmenten über `hls.js` | HLS-fMP4 über einen FFmpeg je Sitzung | M-1, M-3 |
| **Safari macOS** | HLS mit TS über `hls.js` (wie die anderen Browser) | natives HLS | M-5 |
| **iPhone, iPad** | HLS mit TS, nativ, für H.264; HEVC wird bis zur Messung umgewandelt | `hls.js` über `ManagedMediaSource` ab iOS 17.1 (R2); HLS-fMP4 für HEVC (`04` O-3) | M-1, M-5 |
| **Android-Handy-App** | HLS mit TS; Zeitversatz über das Live-Fenster | progressives TS nur als Schnellstart wäre ein zweiter Weg ohne Zeitversatz: nicht verfolgen, ausser M-1 zeigt einen grossen Abstand | M-1, M-4, M-7 |
| **Android-TV-App** | HLS mit TS; Fernseher können oft MPEG-2, MP2 und Zeilensprung selbst (**Annahme**) | wie Handy | M-1, M-4, M-6, M-7 |

**Nicht verfolgt in Stufe 1:** LL-HLS (senkt die Verzögerung zur Sendung, nicht die Umschaltzeit; FFmpeg
kann Apples LL-HLS nicht schreiben, R1), DASH als Ausgabe (kein Gewinn, auf dem iPhone nicht nativ),
fortlaufender TS-Strom (kein Zeitversatz im Player), WebRTC (Codecs passen nicht, kein Weg in Media3).
Der Segmentierer in Go ist die **eine Stelle**, an der ein zweites Segmentformat (fMP4) anschliesst; Liste,
Token, Sitzung und Puffer bleiben gleich.

**Benanntes Risiko Safari und iPhone:** Nach der HLS-Spezifikation soll ein Player bei einer Live-Liste
nicht näher als drei Zieldauern am Ende beginnen. Eine Sitzungsliste enthält beim Einschalten nur ein bis
zwei Segmente; der native Player könnte auf drei warten (bei 2 s Zieldauer rund 6 s). Das ist eine
**Annahme** (RFC 8216 nicht nachgelesen, W-11). Damit sind die Zielwerte der Fälle 1 und 2 auf dem iPhone
**vom Aufbau her gefährdet**. M-5 misst es; Auswege: kurze Segmente in der Startphase (O-19), längerer
Anlauf für diesen Client, `hls.js` statt nativ ab iOS 17.1.

## 4. Matrix C · Videocodec × Client

Ergebnis: **direkt** / **nur Ton umwandeln** (trifft beim Bild nicht zu, steht in Matrix D) / **Bild
umwandeln**. Grundlage: `04` 7.6 und 7.2. **Der Server entscheidet aus den gemeldeten Gerätefähigkeiten**
(ADR 0041, additiv erweitert, `04` O-1), nicht aus dieser Tabelle; sie zeigt die Vorgabe, wenn ein Gerät
nichts anderes meldet. Fehlt eine Angabe, gilt die vorsichtige Annahme «H.264 ohne Zeilensprung, AAC».

| Videocodec | Browser: Chrome, Edge, Firefox, Chrome Android (`hls.js`) | Safari macOS (`hls.js`) | iPhone, iPad (nativ) | Handy-App | TV-App |
|---|---|---|---|---|---|
| **H.264 progressiv** (8 Bit) | **direkt** · Annahme (Funktion belegt: `hls.js` packt TS um; Gerät zu messen, M-3) | **direkt** · Annahme (M-5) | **direkt** · Annahme (M-5) | **direkt** · Annahme (M-4) | **direkt** · Annahme (M-4) |
| **H.264 interlaced** (576i, 1080i) | **Bild umwandeln** (T-15: ein Browser meldet «stellt Zeilensprung sauber dar» nie) · zu messen (M-6) | **Bild umwandeln**, bis M-6 etwas anderes belegt · zu messen (M-6) | **Bild umwandeln**, bis M-6 etwas anderes belegt; Apple entfernt den Zeilensprung nach Annahme selbst · zu messen (M-6) | **Bild umwandeln**, ausser das Gerät meldet saubere Darstellung (Handys eher nicht) · Annahme, zu messen (M-6) | **direkt**, wenn das Gerät meldet (Fernseher meist) · Annahme, zu messen (M-6) |
| **HEVC 8 und 10 Bit** | **Bild umwandeln**, bis M-3 HEVC in TS belegt (Hardware-Decoder abhängig) · Annahme, zu messen (M-3, M-7) | **Bild umwandeln**, bis M-3/M-5 | **Bild umwandeln**, bis M-5; offen: zweites Segmentformat fMP4 (`04` O-3) · Annahme | **direkt** bei Hardware-Decoder · Annahme, zu messen (M-7) | **direkt** bei Hardware-Decoder · Annahme, zu messen (M-7) |
| **HDR** (HDR10, HLG, Dolby Vision; immer HEVC oder AV1) | **Bild umwandeln** (mit bestehendem Tonemapping auf SDR, `04` 7.2) · Annahme | **Bild umwandeln**, bis M-5 · Annahme | **Bild umwandeln**, bis M-5 · Annahme | **direkt** nur bei Hardware-Decoder **und** HDR-Display, sonst **Bild umwandeln** · Annahme (M-7) | wie Handy; Fernseher mit HDR-Display eher direkt · Annahme (M-7) |
| **MPEG-2** | **Bild umwandeln**. MSE-Player und `mpegts.js` kennen kein MPEG-2 (`mpegts.js`: belegt; für andere Browser Annahme) | **Bild umwandeln** · Annahme | **Bild umwandeln**; MPEG-2 steht nicht in Apples Authoring-Spezifikation (Annahme, nicht abgerufen) | **direkt**, wenn das Gerät MPEG-2 dekodiert (viele Handys ohne MPEG-2-Hardware, Annahme), sonst **Bild umwandeln** · zu messen (M-7) | **direkt**, wenn das Gerät MPEG-2 dekodiert (viele Fernseher können es, Annahme) · zu messen (M-7) |
| **AV1** | nicht eigens entschieden in `04`/`05`. Vorgabe **Bild umwandeln**, bis die Fähigkeitsprobe des Clients (`isTypeSupported`, im Repo vorhanden) es bestätigt · Annahme | wie Browser · Annahme | wie Browser; nur neuere Chips · Annahme | Hardware-Decoder abhängig; Media3 nutzt nur den Gerätedecoder (belegt, `supported-formats`) · zu messen (M-7) | wie Handy · zu messen (M-7) |

Anmerkungen:

- **Firefox** hängt bei H.264 und AAC an den Codecs des Betriebssystems (R2, offener Punkt 11): auf allen drei
  Systemen **zu messen (M-3)**.
- **Zeilensprung kostet am meisten.** Die Tabelle zeigt, dass jeder Browser und das iPhone bei Sendern mit
  Zeilensprung eine **Bild-Umwandlung** bekommen, bis M-6 etwas anderes belegt; danach kann eine Einstellung
  «Zeilensprung im Browser durchreichen» kommen (O-23).
- Der Server baut dafür einen neuen Baustein in die Videokette (T-15). Vorgesehen sind `bwdif` (Software),
  `deinterlace_qsv`, `deinterlace_vaapi` und `bwdif_cuda`; alle Filter sind im Image 1.41.3 vorhanden
  (**belegt**, `M1`); die Kosten je Backend sind **zu messen (M-8)**.

## 5. Matrix D · Tonformat × Client

Grundlage: `04` 7.3 und 7.6. Regel aus ADR 0041, angewandt auf TS: AAC wird kopiert; AC-3 und E-AC-3 werden
kopiert, wenn das Gerät sie meldet, sonst neu kodiert; MP2 und AAC-LATM werden kopiert, wenn das Gerät sie
meldet, sonst zu AAC. Ein Sender ohne Tonspur ist ein gültiger Fall (`05` 9.3).

| Tonformat | Browser (`hls.js`) | Safari macOS (`hls.js`) | iPhone, iPad (nativ) | Handy-App | TV-App |
|---|---|---|---|---|---|
| **AAC-LC** | **direkt** · Annahme (M-3) | **direkt** · Annahme (M-5) | **direkt** · Annahme (M-5) | **direkt** · Annahme (M-4) | **direkt** · Annahme (M-4) |
| **HE-AAC** (SBR) | **direkt** · Annahme; **nicht Teil von M-7**, Vorschlag: dort mitmessen | **direkt** · Annahme | **direkt** · Annahme | **direkt** · Annahme | **direkt** · Annahme |
| **AAC-LATM** (DVB) | **nur Ton umwandeln** · Annahme | **nur Ton umwandeln** · Annahme | **nur Ton umwandeln** · Annahme | **direkt**, wenn das Gerät den Ton dekodiert; der TS-Leser von Media3 liest LATM (nicht abgerufen: Annahme) · zu messen (M-7) | wie Handy · zu messen (M-7) |
| **MP2** (MPEG-1 Layer II) | **nur Ton umwandeln** · Annahme | **nur Ton umwandeln** · Annahme | **nur Ton umwandeln** · Annahme | **direkt**, wenn das Gerät MP2 dekodiert, sonst **nur Ton umwandeln** · zu messen (M-7) | wie Handy · zu messen (M-7) |
| **AC-3** | **nur Ton umwandeln** · Annahme | **zu messen (M-5)**; Vorgabe bis dahin **nur Ton umwandeln** · Annahme | **direkt** · Annahme, zu messen (M-5) | **direkt** nur mit AC-3-Decoder oder Durchreichen, sonst **nur Ton umwandeln** · zu messen (M-7) | **direkt** mit passender Tonanlage oder Decoder, sonst **nur Ton umwandeln** · zu messen (M-7) |
| **E-AC-3** | wie AC-3 | wie AC-3 | wie AC-3 | wie AC-3 | wie AC-3 |
| **Mehrere Tonspuren** | Der **Server** wählt die Spur: Er lässt die anderen Ton-PIDs samt angepasster Tabelle weg (T-12). Der Wechsel erzeugt eine neue Ausgabe-Generation (`04` 7.5). Alternative Tonspuren in der HLS-Liste braucht der schlanke Bau von `hls.js` damit nicht (`05` 4.3). | wie Browser | wie Browser; natives HLS könnte `EXT-X-MEDIA` selbst anbieten (Annahme, nicht vorgesehen) | **Server**; ausser das Gerät wählt Spuren selbst (`selectsTracks`, ADR 0041) | wie Handy |

**Ton ist die häufigste Fehlerquelle im Browser:** MP2 und LATM sind in DVB-nahen Quellen üblich; Browser
spielen sie nicht (**Annahme**, R2). Darum ist «nur Ton umwandeln» der billige Standardweg für diesen Fall.
Mehrkanal-AC-3 zu Mehrkanal-AAC wäre eine zweite Stufe (R2, **Empfehlung**, nicht in `04`/`05` entschieden).

## 6. Matrix E · Untertitel × Client

Grundlage: `04` 7.4 und `05` M-24. Im Browser und auf dem iPhone gibt es Untertitel in den ersten Stufen
**nur eingebrannt** (Bild-Umwandlung, bestehende Überlagerung `pipeline.go:327`). «Individuelle
Untertitelauswahl» (LTV-004) ist dort erfüllt, aber teuer.

| Untertitelart | Anzeige live: Browser (`hls.js`) | Safari macOS / iPhone, iPad | Handy- und TV-App | Erhalt in der Aufnahme |
|---|---|---|---|---|
| **DVB-Subtitles** (Bild) | nur **eingebrannt** (Bild umwandeln) | nur **eingebrannt** | PID wird durchgereicht, Media3 zeichnet sie · **Annahme**, zu messen (M-24); sonst einbrennen | **ja**: Aufnahmen behalten alle Spuren (T-16, `05` 2.7: alle PIDs bleiben erhalten) · spezifiziert |
| **Teletext-Untertitel** | nicht angeboten | nicht angeboten | nicht angeboten; kein bekannter Client zeichnet sie (**Annahme**, M-24) | **ja** (PID bleibt in der Aufnahme) · spezifiziert |
| **WebVTT** (Textspur in der HLS-Liste) | in den ersten Stufen **nicht vorgesehen**. Umsetzung von Teletext oder DVB-Text nach WebVTT ist O-5 (spätere Stufe; `libzvbi` ist im Image, **belegt** `M1`). Im schlanken Bau von `hls.js` ungewiss (Annahme). | nativ ja (**Annahme**) | Media3: HLS-WebVTT ja (**belegt**, `supported-formats`) | nicht anwendbar (kein Eingangsformat; würde erst erzeugt) |
| **CEA-608/708** (im Bildstrom, SEI) | **im Browser nicht verfügbar**, bis O-22 entschieden ist: Der schlanke Bau von `hls.js` lässt sie vermutlich weg (**Annahme**, `24` N-7) | nativ ja (**Annahme**); über `hls.js` wie Browser | Media3 zeigt 608 und 708 bei HLS (**belegt**, `supported-formats`) | im Bildstrom enthalten, bei Original-Aufnahme mit gespeichert (Annahme); nach einer Bild-Umwandlung **zu messen** |

Weil Aufnahmen immer alle Spuren behalten, hängt der Erhalt nicht vom Client ab; die **Wiedergabe** einer
Aufnahme richtet sich dann nach der Matrix.

## 7. Matrix F · Funktionen × Client

| Funktion | Browser (`hls.js`) | Safari macOS (`hls.js`) | iPhone, iPad (nativ) | Handy-App (Media3) | TV-App (Media3) |
|---|---|---|---|---|---|
| **Umschalten (Zapping), Player-Wiederverwendung** | Eine `Hls`-Instanz und ein Bildelement über alle Umschaltungen; `loadSource` mit der neuen Liste. Die Möglichkeit ist **belegt** (R1, R2), die Geschwindigkeit gegen einen Neuaufbau **zu messen (M-3)**. | wie Browser · zu messen (M-5) | neue Adresse am Bildelement; die Plattform baut ihre Kette neu auf · **zu messen (M-5)**; der erste Start braucht wohl eine Nutzergeste (Annahme) | `setMediaItem` und `prepare` auf derselben `ExoPlayer`-Instanz, Oberfläche bleibt; ob der Decoder weiterläuft, hängt am Gerät · **zu messen (M-4)** | wie Handy; **kein** zweiter Player für Nachbarsender (Decoder sind knapp, `05` 5.3). Vorgeladen wird nur im Server. |
| **Timeshift: Fenster** (Stufe 4, nur mit Recht) | Fenster von der Rückspulgrenze bis zum neuesten Segment; Spulen im `seekable`-Bereich (**belegt** in R2). Ohne Recht: drei neueste Segmente, ältere 404. | wie Browser | `seekable` (**Annahme**) | `seekTo` im Fenster (**belegt**, Media3 `live-streaming`); Zeitleiste in Wanduhrzeit über `Window.windowStartTimeMs` (belegt), Quelle der Zeit in der Liste Annahme | wie Handy; lange Listen (60 Minuten und mehr) auf schwachen Fernsehern **zu messen (M-19)** |
| **Timeshift: Verhalten am Fensterende** | Liegt die Position ausserhalb, setzt `hls.js` an der Live-Kante fort (`liveSyncPosition`, **belegt**). Der Server setzt am Fensteranfang fort, der Client zeigt es an (`04` 7.5). | wie Browser | läuft aus dem Fenster, springt zur Kante (**Annahme**); Pause im PWA-Modus **zu messen (M-5)** | Fehler `BEHIND_LIVE_WINDOW` (**belegt**), die App muss ihn abfangen und am Fensteranfang fortsetzen | wie Handy |
| **Aufholen zur Live-Kante durch schnelleres Abspielen** | aus: `maxLiveSyncPlaybackRate: 1` (`05` 5.3) | aus | nicht möglich (**Annahme**) | aus: kleinste und grösste Geschwindigkeit 1,0 | aus |
| **Aufnahme-Wiedergabe** (Stufe 5) | gleiche Bauweise wie Live; eine fertige Aufnahme bekommt eine **abgeschlossene** Liste, eine laufende eine wachsende (`05` 4.4) · spezifiziert | wie Browser | wie Browser | wie Browser | wie Browser; Fernbedienung als Vorbild das bestehende Spulen im Player (`PlayerScreen.kt`) |
| **Tonspurwahl** | Server (T-12); Wechsel = neue Ausgabe-Generation, Liste wird neu geladen | wie Browser | wie Browser | Server, ausser `selectsTracks` (ADR 0041) | wie Handy |
| **Untertitelwahl** | nur eingebrannt (Matrix E); CEA-608/708 im schlanken Bau vermutlich nicht (O-22) | wie Browser | nur eingebrannt; CEA nativ (Annahme) | Menü der App; DVB und CEA über Media3 (Matrix E; M-24) | wie Handy |
| **Qualitätswahl** | Server: gewählte Qualität unter der Quelle erzeugt eine **Bild-Umwandlung** und eine neue Ausgabe-Generation (`04` 7.1, 7.5). Die Quelle selbst hat keine Qualitätsleiter ausser bei HLS-Quellen mit mehreren Varianten: höchste (O-14). | wie Browser | wie Browser | wie Browser | wie Browser |
| **Hintergrundwiedergabe, Bildschirm aus** | in `04`/`05` **nicht geregelt**. Der Herzschlag der Sitzung (alle 10 s, Ende nach 60 s ohne, T-21) kann im Hintergrund ausfallen: **zu messen (M-5)**. | wie Browser | iOS: Hintergrund und Sperrbildschirm **zu messen (M-5)** (Herzschlag, Bildübertragung); PWA-Modus Annahme | in `04`/`05` nicht geregelt; Bestand der App für Live-TV nicht nachgelesen: **offen** | entfällt vermutlich (Fernseher); nicht geregelt |
| **Fernbedienung, Tasten** | Tastatur und Zeigegerät; je neue Seite Tastaturtest (Regel 8) | wie Browser | Touch | Touch | D-Pad. **Gebaut:** nur Medientasten `MEDIA_PLAY_PAUSE`, `PLAY`, `PAUSE`, `STOP`, `NEXT`, `PREVIOUS`, `FAST_FORWARD`, `REWIND` (`TvRemoteKeys.kt:39-46`). **Fehlt:** `CHANNEL_UP/DOWN`, `GUIDE`, `INFO`, `LAST_CHANNEL`, `DVR` und Ziffern im Player; ↑/↓ und ←/→ sind im Player belegt (Bedienung zeigen, spulen). |

## 8. Was das für den Normalfall heisst

**Der Normalfall hängt an deinen Sendern, und die kennt noch niemand.** Wie viele Sender Zeilensprung, MP2,
AC-3 oder MPEG-2 haben, zeigt erst **M-2** auf deinen echten Quellen. Als Arbeitsannahme gilt das Profil
eines typischen IPTV-Senders: H.264 oder MPEG-2, oft mit Zeilensprung, Ton MP2 oder AC-3 (**Annahme**).
Dann fällt der Direktweg nach `04` 7.6 so aus:

| Typischer Sender | Browser, Safari macOS | iPhone, iPad | Handy-App | TV-App |
|---|---|---|---|---|
| **H.264 1080i50 (Zeilensprung), Ton MP2 oder AC-3** | Bild umwandeln (inkl. Entfernen des Zeilensprungs) | Bild umwandeln, bis M-6 | Bild umwandeln, ausser Gerät meldet saubere Darstellung; Ton direkt, wenn das Gerät ihn dekodiert | direkt, wenn Gerät Zeilensprung und Ton meldet; sonst Ton oder Bild umwandeln |
| **MPEG-2 576i50, Ton MP2** | Bild umwandeln | Bild umwandeln | direkt nur, wenn das Gerät MPEG-2 dekodiert und Zeilensprung sauber zeigt, sonst Bild umwandeln | wie Handy; Fernseher können es oft (Annahme) |
| **H.264 ohne Zeilensprung, Ton MP2 oder AC-3** | nur Ton umwandeln | MP2: nur Ton umwandeln; AC-3: direkt (Annahme, M-5) | direkt, wenn das Gerät den Ton dekodiert, sonst nur Ton umwandeln | wie Handy |
| **H.264 ohne Zeilensprung, AAC** | **direkt** | **direkt** | **direkt** | **direkt** |

**Ehrlich je Client:**

- **Browser und Safari macOS:** Direkt gibt es nur für H.264 ohne Zeilensprung mit AAC. Alles andere kostet
  einen FFmpeg-Prozess: billig bei «nur Ton» (0,25), teuer bei «Bild» (2 für HD). Bei Sendern, die nach dem
  Profil oben aussehen, ist der Direktweg im Browser **die Ausnahme, nicht der Normalfall** (Annahme, bis M-2).
- **iPhone, iPad:** wie Browser, dazu offen, ob Apple den Zeilensprung selbst entfernt (M-6) und ob AC-3
  direkt geht (M-5). HEVC bleibt umgewandelt (O-3).
- **Handy-App:** Hängt an Gerät und Meldung: MPEG-2 und MP2 sind auf Handys nicht gesichert (M-7); Zeilensprung
  zeigen Handys «eher nicht» sauber (Annahme).
- **TV-App:** der günstigste Client. Fernseher entfernen den Zeilensprung oft im Anzeigepfad und dekodieren
  MPEG-2 und MP2 häufig (**Annahme**, M-6, M-7).

**Serverlast** (`04` 7.6, Rechnung als Annahme; Referenz CT 190, 2 Kerne, Intel UHD 630 mit QSV, Budget 8
Einheiten, Live darf ab Werk die Hälfte, also 4):

| Lage | Kosten je Zuschauer | Gleichzeitig mit halbem Budget |
|---|---|---|
| direkt | 0 | begrenzt durch Netz, Arbeitsspeicher und Serverdeckel |
| nur Ton umwandeln | 0,25 | 16 nach Budget; ob zwei Kerne das tragen, ist offen (M-8) |
| Bild umwandeln, HD | 2 | **2** |

**Klartext:** Auf der Referenz können ab Werk höchstens **zwei** Personen gleichzeitig im Browser einen
Sender mit Zeilensprung oder MPEG-2 schauen; die dritte bekommt «Der Server ist ausgelastet». Zwanzig
Zuschauer desselben Senders sind nur im Direktweg realistisch. Das folgt aus LTV-004 (keine gemeinsame
Umwandlung). Die Rechnung für die drei Installationsgrössen steht in `13-ressourcen-und-performance.md`.

## 9. Am Gerät zu messen

Messpunkte aus `05` 10.1 und `04`, soweit sie diese Matrizen tragen. **Nichts ist gemessen.** Geräte nach
`05` 8.3: Chrome und Firefox (Desktop), Safari auf dem iPhone im Browser **und** als installierte App, Pixel
(das Gerät, mit dem K27 geprüft wurde), ein Android-TV-Gerät (Modell notieren). Safari auf dem Mac und ein
Tablet sind erwünscht, aber nicht Bedingung. Referenzserver: CT 190 (`05` 8.1).

**Kriterium für Umschaltzeiten** (`05` 8.5, Vorschlag; verbindlich erst in `17`): Median unter dem Zielwert
(Fall 1 und 2 unter 1 s, Fall 3 unter 3 s, Fall 4 unter 5 s) in den Netzen N-1 und N-2, 95. Perzentil höchstens
das Doppelte, Fehlerquote höchstens 2 %. 50 gewertete Umschaltungen je Reihe.

| Nr. | Was | Gerät | Kriterium | Muss vorliegen vor |
|---|---|---|---|---|
| **M-1** | Umschaltzeit je Fall, Client, Netz | alle Clients | siehe oben | Abnahme jeder Stufe mit Wiedergabe |
| **M-2** | Deine echten Quellen: Anteil H.264 ohne Zeilensprung mit AAC, Zeilensprung, MPEG-2, MP2, AC-3; Bildgruppen-Länge | kleines Messwerkzeug auf dem Server | Kennzahlen; entscheidet, wie oft der Direktweg greift | **Baufreigabe der Stufe 1** (T-11, T-12) |
| **M-3** | `hls.js` 1.7.3 (schlanker Bau) mit TS: H.264, HEVC, AAC, MP2, AC-3; Diskontinuitäten; Startpunkt; Listen von drei Segmenten mit `liveSyncDurationCount` 2; `loadSource` gegen Neuaufbau; mit und ohne `startFragPrefetch` | Chrome, Firefox | spielt ohne Stocken; Startzeit | **Baufreigabe der Stufe 1** (T-11) |
| **M-4** | Media3 1.11.1 mit HLS-TS: Ladepuffer; Start mit und ohne übergeordnete Liste (`CODECS`); Decoder-Wiederverwendung; Verhalten hinter dem Fenster; lange Listen | Pixel, Android-TV-Gerät | Startzeit, Stallrate, Decoder-Reuse ja/nein | Baufreigabe der Stufe, in der die Android-Apps Live-TV bekommen (`03` F-6) |
| **M-5** | Safari (Mac, iPhone, installierte App) mit HLS-TS: **Zeit bis zum Bild bei einer Liste mit einem, zwei und drei Segmenten**; Startpunkt aus der Liste; HEVC, AC-3; Diskontinuitäten; Dauer des Umschaltens; Herzschlag im Hintergrund und bei Bildübertragung | iPhone (Browser und PWA), Safari Mac | Startzeit je Listenlänge; Herzschlag bricht nicht ab | **Baufreigabe der Stufe 1**, sofern das iPhone zur Stufe gehört |
| **M-6** | Zeilensprung je Client: Kammartefakte bei 1080i50 und 576i50, auch im Browser | alle Clients mit Testquellen Q-E, Q-F | Kämme sichtbar ja oder nein; Entscheid «Server entfernt immer» oder «Gerät schafft es» | Festlegung der Vorgabe in T-15; `04` O-23 |
| **M-7** | Decoder je Gerät: MPEG-2, MP2, AAC-LATM, AC-3, E-AC-3, HEVC (Abfrage der Gerätefähigkeiten, Gegenprobe mit Testquellen) | Pixel, Android-TV-Gerät, Browser (`isTypeSupported`) | Wie viele Geräte bräuchten wirklich eine Umwandlung? | Festlegung der Fähigkeitsangaben (`04` O-1) |
| **M-8** | Live-Umwandlung: Zeit bis zum ersten Segment je Backend; Filter gegen Zeilensprung und ihre Kosten; Kosten der Ton-Umwandlung | CT 190 (QSV), Entwicklungsrechner (NVENC), Software | Zeit, Kosten je Zuschauer; trägt die Referenz die Ton-Umwandlung? | Bau der Stufe mit Umwandlung; Budgetwerte `04` 7.6 |
| **M-9** | Nahtlose Segmente über mehrere FFmpeg-Läufe mit TS; Zeitsprung im Eingang | alle drei Player | ohne sichtbaren Sprung | Bau des Spulens mit Umwandlung (Stufe 4); sonst Rückfall `05` 4.5 |
| **M-11** | FFmpeg-Leser für DASH-Live und HLS-fMP4 mit getrenntem Ton | Testquellen | Stabilität | Baufreigabe von Weg W |
| **M-16** | UDP und Multicast im Container: Bridge, Host-Netz, macvlan | CT 190, Heimnetz | Paketverlust unter Last | Bau der Stufe mit UDP (F-11) |
| **M-18** | Verhalten hinter dem Reverse-Proxy (Nginx Proxy Manager, Caddy): Abruf der Segmente, Zwischenpufferung | dein Aufbau | Umschaltzeit gegenüber direkt | Abnahme der ersten Stufe |
| **M-19** | Lange Wiedergabelisten (60 Minuten und mehr): Zeit zum Einlesen | Android-TV-Gerät | Parsezeit, Ruckler | Bau des Zeitversatzes |
| **M-22** | Zeitüberlauf nach 26,5 Stunden im Direktweg | Testquelle mit vorgestelltem Zeitstempel, alle Player | spielt ohne Störung durch | Bau der ersten Stufe |
| **M-24** | Untertitel: DVB und Teletext in Media3 über HLS-TS; Umsetzung von Teletext nach WebVTT | Pixel, Android-TV, Testquelle mit Untertiteln | wird gezeichnet | Stufe mit Untertiteln |

**Lücken ohne eigenen Messpunkt** (aus R2 Abschnitt 8 und dieser Matrix; **Vorschlag**, sie unter den
genannten Punkten mitzumessen):

| Lücke | Unter | Warum offen |
|---|---|---|
| Was `hls.js/light` 1.7.3 weglässt (zusätzliche Tonspuren, Untertitel, CEA-608) und Grössenunterschied zum vollen Bau | M-3 (`04` O-22, `24` N-7) | im abgerufenen Teil der API-Seite nicht enthalten |
| HE-AAC und AV1 je Client | M-7 | in `04`/`05` nicht eigens geführt |
| HEVC 10 Bit und HDR: Hardware-Decoder und Tonemap-Pfad | M-3, M-5, M-7 | nur als Annahme |
| Firefox: H.264, AAC, AC-3, HEVC über OS-Codecs auf Windows, Linux, macOS | M-3 | R2, offener Punkt 11 |
| iPadOS: nativ oder `hls.js`? | M-5 | siehe Abschnitt 10, Punkt 9 |
| `EXT-X-PROGRAM-DATE-TIME`: Genauigkeit der Zeitleiste in allen Playern, `getStartDate()` auf iOS | M-5, M-4, M-3 | R2 Abschnitt 8, Punkt 9 |
| Hintergrundwiedergabe und Bildschirm aus (Handy-App, Web) | M-5 und eigene Messung | in `04`/`05` nicht geregelt |
| Fire OS: startet die TV-App überhaupt? | nur, wenn jemand es will | Nicht-Ziel |

## 10. Offene Punkte und Widersprüche

Wo R2 oder R1 und `05` Fassung 2 sich widersprechen, gilt `05`. Stand der Dinge je Punkt:

| Nr. | Punkt | Was die Recherche sagt | Stand laut `04`/`05` Fassung 2 | Auswirkung auf die Matrix |
|---|---|---|---|---|
| 1 | **Android: progressives TS oder HLS** (W-1) | R1: Android bekommt den Strom progressiv, am schnellsten. R2: progressives TS hat bei Media3 kein Live-Fenster, also kein Timeshift im Player. | HLS mit TS für alle Clients. Progressives TS als zweiter Weg ohne Zeitversatz: **nicht verfolgen**, ausser M-1 zeigt einen grossen Abstand (`05` 4.3). | Matrix B: progressiv nur «kann», nicht «empfohlen». |
| 2 | **Basisformat HLS-fMP4 (R2 Variante A) oder HLS-TS** | R2 empfiehlt fMP4 für alle; MPEG-2, MP2, LATM passen nicht in fMP4 (Annahme). | T-11: **HLS-TS** je Sitzung; fMP4 ist die Stelle für ein zweites Segmentformat und der Rückfall, wenn M-3 bis M-5 TS nicht tragen. | Matrix B und C: MPEG-2 und MP2 lassen sich nur in TS durchreichen; HEVC für Apple offen (O-3). |
| 3 | **`hls.js/light`** (`04` O-22, `24` N-7) | R2: lässt eventuell zusätzliche Tonspuren und Untertitel/CEA weg; nicht nachgelesen. | Die Tonspurwahl löst der Server (T-12); Untertitel im Bildstrom vermutlich nicht im schlanken Bau (Annahme). **Offen:** ob Live-TV den vollen Bau als eigenen Teil lädt. | Matrix D unberührt; Matrix E: CEA-608/708 und WebVTT im Browser unsicher. |
| 4 | **Startpunkt in `hls.js`** (W-6) | R1: `liveSyncDurationCount` 1. R2: 2 wie im Radio. | Der Startpunkt kommt vom Server (`startPosition`, `EXT-X-START`); `liveSyncDurationCount` bleibt 2 und wirkt nur nach Störungen und beim Knopf «zurück zu Live». Wert 1 verworfen; ob 2 bei drei Segmenten trägt, misst M-3. | Matrix F «Umschalten». |
| 5 | **Standardwerte des Ladepuffers in Media3** (W-5) | R1 (abgerufen): 1000 ms bis Start, 2000 ms nach Rebuffer. R2 (aus Erinnerung): 2500 und 5000 ms. | Beide nicht gegen 1.11.1 geprüft; `05` empfiehlt 500 bis 1000 ms Start und 1500 bis 2000 ms nach Stocken. | **zu messen (M-4)**. |
| 6 | **Startregel von HLS auf Safari und iPhone** (W-11) | R2 und R3 haben RFC 8216 nicht abgerufen; R5 führt ihn als abgerufen. | benanntes Risiko, M-5 vor Baufreigabe | Matrix B; Fälle 1 und 2 auf dem iPhone gefährdet. |
| 7 | **Ton und Zeilensprung: R2-Vorgaben gegen `04`** | R2: Ton immer zu AAC-LC für alle Browser und Handys; Interlaced für Browser und Handy immer serverseitig entfernen, Ausnahme Safari und Android TV nach Messung. | T-14/T-15 und `04` 7.3: Kopieren, **wenn das Gerät den Ton meldet**; Zeilensprung entfernt der Server, **ausser das Gerät meldet saubere Darstellung** (Browser nie). Handys: «eher nicht» (Annahme). | Matrix C und D folgen `04`. |
| 8 | **HEVC auf Apple: direkt (R2) oder umwandeln** | R2: Safari direkt, aber nur in fMP4 (`hvc1`). | Bis M-5 umwandeln; zweites Segmentformat ist O-3. | Matrix C. |
| 9 | **iPadOS: nativ (`05` 4.3) oder `hls.js`** | R2: «nativ oder `hls.js`». Das Repo wählt «mse» überall, wo klassisches `MediaSource` existiert (belegt, `capabilities.ts`); ob iPadOS es hat, ist hier **Annahme**. | `05` führt «iPhone und iPad» zusammen als nativ. | Matrix B und C für iPad; **zu messen (M-5)**. |
| 10 | **Zeitversatz: Pause im Player oder im Server** | R2: Pause serverseitig (Playlist wächst weiter, Player wird angehalten); EVENT-Liste oder langes Gleitfenster. | Sitzungspuffer auf dem Datenträger mit eigenem Fenster; Rückspulgrenze ist der Einschaltzeitpunkt (F-7, offen); ohne Recht kein Zurückspulen, ältere Segmente 404. | Matrix F «Timeshift». |
| 11 | **DASH, HLS-fMP4 als Eingang und RTSP, RTMP: Stufe** | `03` nennt DASH, RTSP, RTMP in Stufe 3. | DASH und HLS-fMP4 in eigener später Stufe von Weg W, nur nach M-10 (`04` O-24: `03` ist anzupassen); RTSP und RTMP «spät», vorbehaltlich F-1. | Matrix A. |
| 12 | **Wer empfängt: FFmpeg oder Go** (W-3) | R3: FFmpeg als einziger Eingang. R1: Go. | T-3: Go (Weg G und L); FFmpeg nur Weg W. | Matrix A folgt `04`. FFmpeg-Optionen aus R3 4.2 gelten nur für Weg W. |
| 13 | **FFmpeg-Version** (W-4) | Berichte beziehen sich auf die neueste Fassung; im Image läuft 7.1.4-Jellyfin. | Aussagen zu FFmpeg gelten als Annahme bis zu den Messpunkten. | Matrix A. |
| 14 | **LL-HLS, WebRTC** | R2 bewertet beides. | nicht verfolgt (T-11) | Matrix B. |

**Weitere offene Entscheidungen, die diese Matrix berühren:** F-1 (RTSP und RTMP über Go-Bibliotheken oder
FFmpeg), F-11 (Multicast: Netzmodus «host» oder eigenes Netz), F-13 (DASH und HLS-fMP4, falls Weg W nicht
dicht belegbar ist), O-1 (Form der additiven Fähigkeitsangaben der Geräte: Videocodec `mpeg2video`,
Toncodecs `mp2` und `aac_latm`, «stellt Zeilensprung sauber dar», Untertitelformate `dvb_subtitle` und
`dvb_teletext`), O-14, O-15, O-16, O-19, O-23 (siehe `04` 12.2 und `05` 10.2).
