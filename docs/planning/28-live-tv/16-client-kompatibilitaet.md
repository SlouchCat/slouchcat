# Konzept 28 · Client-Kompatibilität

Stand: 28. November 2026 · Fassung 2 · Status: Entwurf nach Konsolidierung 1, nicht freigegeben

Dieses Dokument beantwortet die Forderung von LTV-003: eine Kompatibilitätsmatrix für Eingangsprotokolle,
Container, Codecs und unterstützte Clients. Es verdichtet `04-architektur.md` und
`05-streaming-und-fast-zapping.md` (beide **Fassung 3**, verbindlich), `18-umsetzungsplan.md` (Stufen),
`anhang/nachpruefung-quellen.md` (Nachprüfung an Primärquellen), `24-recherche-und-quellen.md` und die
Recherche R1 bis R3. Es ändert keinen Entscheid (T-1 bis T-29) und erfindet keinen Befund. Wo R2 und `05`
sich widersprechen, gilt `05`; der Stand der Widersprüche steht in Abschnitt 10.

## 0. So liest du die Tabellen

**Alles in Konzept 28 ist «spezifiziert», nichts ist gebaut.** Jede Zelle trägt zusätzlich eine
Verlässlichkeit:

| Marke | Bedeutung |
|---|---|
| **belegt** | In der Recherche am 28. November 2026 aus einer abgerufenen Quelle entnommen, im Repo gelesen (Datei im Text genannt) oder im Image gemessen (`M1`). |
| **belegt (Nachprüfung Nr. n)** | In `anhang/nachpruefung-quellen.md` unabhängig an der Primärquelle bestätigt (Zeile n). Das ist die stärkste Marke dieses Dokuments. |
| **Annahme** | Aus Wissen, aus einer nicht abgerufenen Quelle oder aus einer Überlegung. Kein Befund. |
| **zu messen (M-n)** | Nur ein Gerätetest klärt es; die Nummer verweist auf Abschnitt 9. |
| **ungeprüft, nicht zugesagt** | Für Safari, iPhone und iPad: Es gibt kein Apple-Gerät (`04` 12.1, `05` 4.3). Was dort steht, ist aus Dokumenten belegt oder Annahme, nie gemessen, und Konzept 28 sagt diese Clients nicht zu, bis ein Gerät da ist. |

Nichts ist stärker gemacht als die Unterlage. Zielwerte sind zu prüfende Grössen, keine Zusagen.

**Die drei Wege des Servers** (`04` 7.1, T-14: «Original», «Ton-Umwandlung», «Bild-Umwandlung»):

| Ergebnis in den Matrizen | Was der Server tut | Kosten |
|---|---|---|
| **direkt** | Bild und Ton bleiben, wie sie sind. Go schneidet den Strom nur in Segmente (kein FFmpeg-Prozess). | keine im Umwandlungs-Budget |
| **nur Ton umwandeln** | Bild wird kopiert, der Ton mit FFmpeg neu kodiert (zu AAC). | Satz 0,25 (**Annahme**, M-8) |
| **Bild umwandeln** | FFmpeg kodiert das Bild neu (auch zum Entfernen des Zeilensprungs, zum Einbrennen von Untertiteln, zum Anpassen von HDR). | Satz 1,5 bis 6, HD ist 2 (`04` 7.6) |

**Begriffe in einem Halbsatz.** *Zeilensprung* (interlaced, 1080i, 576i): Das Bild besteht aus zwei
ineinander verschränkten Halbbildern; ohne Aufbereitung zeigen sich Kammartefakte, *Deinterlacing* entfernt
sie. *Segment*: ein kurzes Stück des Stroms, aus dem eine HLS-Liste besteht. *MSE* (Media Source Extensions):
die Schnittstelle, über die ein Browser-Player Stücke zum Abspielen übergibt. *Media3*: der Android-Player
der beiden Apps. *PID*: die Kennung einer Tonspur oder eines Untertitels im MPEG-TS. *LATM*: eine Verpackung
für AAC-Ton, wie sie DVB nutzt. *schlanker Bau* (`hls.js/light`): die verkleinerte Fassung des Web-Players,
die die App heute lädt. *DRM*: Kopierschutz mit Lizenzserver.

## 1. Die Clients

| Client | Player | Wahl der Wiedergabe | Verlässlichkeit |
|---|---|---|---|
| **Web: Chrome, Edge, Firefox, Chrome Android** | `hls.js` 1.7.3, **schlanker Bau** (`hls.js/light`), über MSE. Live-TV lädt den vollen Bau nicht (`05` 4.3, `04` O-22). | `web/src/features/player/capabilities.ts`: «mse» überall, wo klassisches `MediaSource` existiert | belegt (Repo: `web/package.json`, `VideoPlayer.tsx`, `capabilities.ts`) |
| **Web: Safari macOS** | `hls.js` über MSE (`05` 4.3); natives HLS als Alternative | wie oben | `05` spezifiziert; ungeprüft, nicht zugesagt |
| **Web: Safari iPhone** (Browser und installierte App) | natives HLS. Kein klassisches `MediaSource`; `ManagedMediaSource` ab iOS 17.1 als Ausweg über `hls.js` (hls.js-README seit 1.5.0). | «native» nur, wo es allein `canPlayType('application/vnd.apple.mpegurl')` gibt | belegt (Repo; Nachprüfung Nr. 15); ungeprüft, nicht zugesagt |
| **Web: Safari iPadOS** (Browser und installierte App) | folgt der Weiche der App: `hls.js`, wo die klassische Medienschnittstelle vorhanden ist (auf dem iPad **Annahme**), sonst natives HLS (`05` 4.3; Fassung 2 führte das iPad fälschlich als «nativ») | `capabilities.ts` | Annahme; ungeprüft, nicht zugesagt |
| **Android-Handy-App** | Media3 1.11.1, `ExoPlayer` mit `SlouchcatRenderersFactory`, OkHttp | `android/core/player/.../Media3Engine.kt`; setzt heute weder `LoadControl` noch `LiveConfiguration` | belegt (Repo) |
| **Android-TV-App** (deckt Google TV ab) | Media3 1.11.1, wie Handy | wie Handy; wertet `DecoderReuseEvaluation` schon aus (`PlayerFeature.kt`) | belegt (Repo) |

**Nicht dabei** (Nicht-Ziele nach `CLAUDE.md`, Regel 11): Fire TV, Apple TV, webOS, Tizen. Zu Fire OS nur
eine **Annahme** aus R2 (Amazon-Dokumentation nicht abgerufen): Es ist ein Android-Fork, auf dem APKs und
Media3 laufen; ob die Android-TV-App dort unverändert startet, hängt am Leanback-Manifest, an fehlenden
Google-Diensten (FCM, Anmeldung) und am Amazon-Appstore. Kein Ziel, höchstens ein Nebeneffekt.

## 2. Matrix A · Eingangsprotokoll

Wie der Server die Quelle holt, gilt unabhängig vom Client. Sie steht hier, weil sie bestimmt, was der
Client zu sehen bekommt: Alle Wege liefern denselben Rohstoff, **einen TS-Bytestrom mit genau einem
Programm** (`05` 2.1). Nichts davon ist gebaut.

**Die drei Empfangswege (T-3):** **G** = Go liest selbst, **L** = Go mit einer Protokoll-Bibliothek,
**W** = FFmpeg liest, aber nur unverschlüsseltes HTTP von einem Go-Weiterleiter auf `127.0.0.1` über einen
Ersatzpfad (`http://127.0.0.1:<Port>/<Zufall>/…`, T-4, `05` 2.6). FFmpeg bekommt nie eine Anbieter-Adresse,
nie Zugangsdaten, nie eine Kopfzeile.

**Stufen** nach `18-umsetzungsplan.md`: **1** (1a «Schauen», 1b), **3** (Empfang und Umschalten), **H**
(Heimnetz: UDP, RTP, Multicast), **L** (RTSP, RTMP), **W** (DASH, HLS mit fMP4, zuletzt), **E**
(Ergänzungen nach Messung).

| Protokoll | Weg, Stufe | Voraussetzungen: Netz und Freigabe lokaler Ziele | Wiederverbindung | Bekannte Grenzen | DRM-Erkennung |
|---|---|---|---|---|---|
| **HTTP-TS** (Dauerstrom, auch Xtream `.ts`) | G, **Stufe 1a** | Bridge-Netz genügt (`04` 10.6). Öffentliche Adressen frei; lokale nur als feste Ziele der Quelle mit ausdrücklicher Freigabe (T-5, Stufe H). Weiterleitungen: höchstens 5, jede neu geprüft. | Go, Leiter nach `04` 5.3; Stillstand nach 10 s ohne Daten | Neuer Abnehmer wartet auf einen Keyframe; ohne «Schub» beim Verbinden hängt der Start an der Bildgruppen-Länge der Quelle (`05` 7, M-1, M-2). Eine Neuverbindung erzeugt eine Diskontinuität (Annahme, M-9). | Keine Kennzeichnung im Strom möglich. Verschlüsselte Bild- oder Tonpakete erkennt der Verteiler am Kennzeichen im Paketkopf (nur Bild- und Ton-PIDs, über mindestens 5 s) und meldet «verschlüsselt». |
| **HLS mit TS-Segmenten** | G, **Stufe 1a** | wie HTTP-TS; Liste, Unterliste, Segment und Schlüssel sind je eine eigene geprüfte Anfrage | Go lädt die Liste nach; Segmentfehler: nächstes Segment, nach 3 Fehlern in Folge Abbruch | Bei mehreren Qualitäten gilt die höchste (O-14). Zieldauer höchstens 60 s, höchstens 32 Varianten, Unterlisten höchstens 3 Ebenen. Sprung von mehr als 10 Segmenten: Neubeginn am Ende. | Regel `05` 2.5, geprüft bei jedem Nachladen: `AES-128` mit `identity` und `http(s)`-Schlüssel wird gespielt; `SAMPLE-AES*`, fremdes Schlüsselformat, `skd://`, `data:` und `EXT-X-SESSION-KEY` mit diesen Merkmalen werden abgelehnt. Dass Media3 `SAMPLE-AES` nicht kann, stützt die Regel (belegt (Nachprüfung Nr. 17)). Grenzfall `SAMPLE-AES` mit `identity`: vorsichtig abgelehnt (O-16); rechtliche Einordnung nicht abgerufen (Annahme). |
| **Xtream-Live** | G, **Stufe 1a** | Abfragen über den geschützten HTTP-Client; Stream-Adresse entsteht erst beim Verbinden, nur aus dem eingetragenen Host | wie HTTP-TS oder HLS | Nur Live. Feldabweichungen zwischen Anbietern unbelegt (R3 3.3, **Annahme**). | wie HTTP-TS oder HLS |
| **UDP** (Unicast) | G, **Stufe H** | Der Server lauscht, kein Verbindungsaufbau nach aussen, also greift der Dialer nicht. Port veröffentlichen (Zusatzdatei `compose.live-udp.yaml`, Arbeitsname). Feste Ziele der Quelle mit Freigabe; Socket an **eine gewählte** Schnittstelle gebunden, Port ab 1024. | zustandslos; 10 s ohne Paket gilt als Stillstand | Paketverlust bei kleinem Socketpuffer; Absender im selben Netz fälschbar (die Verwaltung sagt es wörtlich). Nicht unterstützt in der ersten Stufe mit UDP: TS-Pakete mit 204 Byte, Fehlerkorrektur nach SMPTE 2022 (O-15). | keine Kennzeichnung möglich; Kennzeichen im Paketkopf wie bei HTTP-TS |
| **RTP** | G, **Stufe H** | wie UDP. Go entfernt den RTP-Kopf und zählt Lücken. **Der Empfänger sendet nichts**, auch kein RTCP. | wie UDP | Keine eigene Sitzungsbeschreibung (SDP), kein Umordnen vertauschter Pakete (O-15; Bedarf nach M-2 und M-16). | wie UDP |
| **Multicast** | G, **Stufe H** | **Netzmodus «host»** (`compose.live-hostnet.yaml`, Arbeitsname) oder eigenes Netz des Containers; dass der Beitritt im Bridge-Netz nicht ankommt, stützt sich nur auf Foren und Blogs, die offiziellen Docker-Seiten erwähnen Multicast nicht (Annahme; Nachprüfung Nr. 41; **zu messen, M-16**). Bei `network_mode: host` ist `ports:` wirkungslos; `macvlan` braucht einen Switch, der mehrere MAC-Adressen je Anschluss duldet, und der Host erreicht seine `macvlan`-Container nicht direkt (belegt (Nachprüfung Nr. 41)). Benannte Lockerung (F-11, offen). Weg W ist im Netzmodus «host» gesperrt. | Beitritt wird beim Wiederaufbau erneuert; Verhalten bei IGMP-Snooping ohne Abfrager: **zu messen (M-16)** | Mehrere Programme in einem Strom: Go wählt das Programm des Senders (`05` 2.7, O-11). WLAN-Multicast schwach (Annahme). | wie UDP |
| **RTSP** | L, **Stufe L**, vorbehaltlich F-1 | Go-Dialer; **nur TCP**, kein Rückfall auf UDP; nennt der Server eine andere Zieladresse für den Datenkanal, wird abgelehnt | Go, Leiter | Neue Go-Bibliotheken nötig (F-1, Empfehlung (a)); jede muss nachweislich nur über den übergebenen Dialer verbinden (**zu messen, M-23**). `18` D-1: `03` setzte RTSP und RTMP in Stufe 3, gilt Stufe L (RTSP, RTMP). | kein DRM in diesem Protokoll |
| **RTMP** | L, **Stufe L**, vorbehaltlich F-1 | Go-Dialer (TCP, bei `rtmps` mit TLS) | Go, Leiter | wie RTSP; veraltetes Protokoll (R3, Annahme). | `rtmpe` (verschlüsselt) wird abgelehnt |
| **HLS mit fMP4-Segmenten oder getrennter Tonspur** | W, **Stufe W**, nur nach M-10 | Weiterleiter holt alles selbst über den geschützten Dialer; FFmpeg läuft in einer Kernel-Sperre (Landlock: auf einen Port eingeengt, nicht auf den Weiterleiter; Sperre Pflicht, M-25). Dass `tcp` in der Protokoll-Liste nötig ist, ist belegt (Nachprüfung Nr. 24). Netzmodus «host»: gesperrt. | Go startet den Prozess neu, wenn er abbricht oder 10 s nichts liefert; ein FFmpeg-Lauf dauert höchstens 24 Stunden und nie über einen Überlauf der Quellzeit (`04` 7.5). `reconnect*` von FFmpeg sind in 7.1.4 ab Werk aus (belegt (Nachprüfung Nr. 26)). | Ohne belegte Dichtheit entfällt der Weg (F-13); Alternativen: nur mit Sperre, Go liest selbst (viel Eigencode) oder gar nicht (Abstrich an LTV-003, müsste von Marco kommen). | Go liest jede Liste, bevor FFmpeg sie sieht; Regel wie HLS mit TS |
| **DASH ohne DRM** | W, **Stufe W**, wie oben | wie oben | wie oben | FFmpeg-Leser für Live-Manifeste: Stabilität **zu messen (M-11)**; `18` D-2: `03` setzte DASH in Stufe 3, gilt Stufe W. | Jedes `ContentProtection` führt zur Ablehnung; die FFmpeg-Optionen zum Entschlüsseln mit vorgegebenem Schlüssel setzt der Server nie |

**Verlässlichkeit der Matrix A:** Wege und Stufen sind Entscheide aus `04`, `05` und `18` (spezifiziert).
Das Verhalten von FFmpeg stammt aus der Dokumentation der neuesten Version; die Nachprüfung hat einzelne
Werte am Tag n7.1.4 gelesen (Nr. 24 bis 27), alles andere bleibt **Annahme** bis M-10 bis M-12 (W-4). Belegt
(gemessen im Image 1.41.3, `M1`) ist, was FFmpeg 7.1.4 kann: Eingangsprotokolle u. a. `http(s)`, `rtmp*`,
`rtp`, `udp`, `hls`, `crypto`, Demuxer `dash`, `hls`, `mpegts`, `rtsp`; **SRT fehlt im Image**, deshalb wird
es nicht empfangen (`05` 2.2).

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
| **Chrome, Edge, Firefox, Chrome Android** | `hls.js`: demuxt TS im Hintergrund-Thread und packt zu fMP4 für MSE um (belegt, hls.js-API). **Der schlanke Bau liest in TS nur H.264, AAC und MPEG-Audio (MP2/MP3, wenn der Browser es meldet); HEVC und AC-3 erst der volle Bau, E-AC-3, AAC-LATM und MPEG-2 keiner** (belegt (Nachprüfung Nr. 13)). | `hls.js`; gebaut und erprobt bei Filmen und Radio (belegt, ADR 0003, ADR 0025) | `hls.js` mit `lowLatencyMode`, **ab Werk an** (belegt (Nachprüfung Nr. 9)); für Live-TV ausdrücklich `false` | nur mit zweiter Bibliothek `mpegts.js` (MSE); kein MPEG-2, kein Seeking (belegt, Beschreibung von mpegts.js) | nur mit zusätzlicher Bibliothek (dash.js, Shaka); Annahme |
| **Safari macOS** | `hls.js` (`05` 4.3), schlanker Bau, Grenzen wie links · ungeprüft, nicht zugesagt | `hls.js` oder nativ · Annahme; ungeprüft, nicht zugesagt | nativ · Annahme (Apple-Seite nicht abgerufen); ungeprüft, nicht zugesagt | `mpegts.js` · Annahme; ungeprüft, nicht zugesagt | nur mit Bibliothek · Annahme; ungeprüft, nicht zugesagt |
| **iPhone** | nativ; H.264 in TS erlaubt (belegt (Nachprüfung Nr. 8)); **Apples Vorgaben: mindestens 6 Segmente in einer Live-Liste, Segment beginnt mit IDR, Uhrzeit in jeder Live-Liste** (belegt (Nachprüfung Nr. 6, 7)); Startverhalten **zu messen (M-5)** · ungeprüft, nicht zugesagt | nativ; **HEVC nur in fMP4, nie in TS** (belegt (Nachprüfung Nr. 8)) · ungeprüft, nicht zugesagt | nativ · Annahme; ungeprüft, nicht zugesagt | nur `mpegts.js` ab 1.8 über `ManagedMediaSource`, iOS ab 17.1 (belegt, Beschreibung von mpegts.js) · ungeprüft, nicht zugesagt | kein natives DASH; Bibliothek nur mit `ManagedMediaSource` · Annahme; ungeprüft, nicht zugesagt |
| **iPad** | wie die Web-App entscheidet (`hls.js` nach Annahme, sonst nativ wie iPhone) · ungeprüft, nicht zugesagt | wie links | wie links | wie links | wie links |
| **Android-Handy-App (Media3)** | ja: `HlsMediaSource` mit TS-Leser (belegt, Media3 `supported-formats`; Nachprüfung Nr. 17, 19) | ja (belegt, Nr. 17) | nur Apples Form, nicht die der Community (belegt (Nachprüfung Nr. 17)) | ja: `ProgressiveMediaSource`, aber **ohne Live-Fenster** und nur an einer Position abspielbar (belegt, Media3 `live-streaming`; Nachprüfung Nr. 18) | ja für fMP4 und WebM, **kein TS** (belegt (Nachprüfung Nr. 17)) |
| **Android-TV-App (Media3)** | wie Handy | wie Handy | wie Handy | wie Handy | wie Handy |

### 3.2 Empfehlung laut T-11 und Alternative

**T-11 ist eine Empfehlung unter Vorbehalt:** M-2, M-3 und M-4 müssen **vor der Baufreigabe der Stufe**
vorliegen, in der der jeweilige Client Live-TV bekommt. M-5 gibt es ohne Gerät nicht; **Safari, iPhone und
iPad sind ungeprüft und nicht zugesagt**, und keine andere Baufreigabe hängt daran (`05` 10.1).

| Client | Empfehlung (T-11, `05` 4.3) | Alternative, falls Messungen widersprechen | Zu messen |
|---|---|---|---|
| **Chrome, Edge, Firefox, Chrome Android** | HLS mit TS-Segmenten über `hls.js`, schlanker Bau | HLS-fMP4 über einen FFmpeg je Sitzung | M-1, M-3 |
| **Safari macOS** | HLS mit TS über `hls.js` (wie die anderen Browser) · ungeprüft, nicht zugesagt | natives HLS | M-5 (kein Gerät) |
| **iPhone** | HLS mit TS, nativ, für H.264; HEVC wird umgewandelt · ungeprüft, nicht zugesagt | `hls.js` über `ManagedMediaSource` ab iOS 17.1 (belegt (Nachprüfung Nr. 15)); HLS-fMP4 für HEVC (`04` O-3) | M-1, M-5 (kein Gerät) |
| **iPad** | wie die Web-App entscheidet · ungeprüft, nicht zugesagt | natives HLS | M-5 (kein Gerät) |
| **Android-Handy-App** | HLS mit TS; Zeitversatz über das Live-Fenster | progressives TS nur als Schnellstart wäre ein zweiter Weg ohne Zeitversatz: nicht verfolgen, ausser M-1 zeigt einen grossen Abstand | M-1, M-4, M-7 |
| **Android-TV-App** | HLS mit TS; Fernseher können oft MPEG-2, MP2 und Zeilensprung selbst (**Annahme**) | wie Handy | M-1, M-4, M-6, M-7 |

**Nicht verfolgt in Stufe 1:** LL-HLS (senkt die Verzögerung zur Sendung, nicht die Umschaltzeit; FFmpeg
kann Apples LL-HLS nicht schreiben, R1), DASH als Ausgabe (kein Gewinn, auf dem iPhone nicht nativ),
fortlaufender TS-Strom (kein Zeitversatz im Player), WebRTC (Codecs passen nicht, kein Weg in Media3). Der
Segmentierer in Go ist die **eine Stelle**, an der ein zweites Segmentformat (fMP4) anschliesst.

**Startregel und Apples Vorgaben auf Safari und iPhone: bewusste Abweichung.** Belegt sind zwei Dinge:

- Nach RFC 8216 (6.3.3) soll ein Player bei einer Live-Liste kein Segment wählen, das weniger als drei
  Zieldauern vor dem Listenende beginnt (belegt (Nachprüfung Nr. 1); die frühere Unsicherheit W-11 ist damit
  aufgelöst).
- Apples Vorgaben verlangen in einer Live-Liste **mindestens sechs Segmente** (MUST), Zieldauer 6 s
  (SHOULD), ein IDR-Bild am Anfang jedes Segments (MUST) und die Uhrzeit in jeder Live-Liste (belegt
  (Nachprüfung Nr. 6, 7)).

Eine Sitzungsliste enthält beim Einschalten nur den Anlauf, also ein bis zwei Segmente, und ohne Recht
«Timeshift» höchstens drei. **Das weicht auf Apple-Geräten von Apples Vorgaben ab, bewusst** (`05` 4.3).
Eine Vorgabe ist keine Wiedergabegarantie in beide Richtungen: Der native Player kann kurze Listen spielen
oder warten, bis genug Segmente da sind (bei 2 s Zieldauer und sechs Segmenten rund 12 s). Die Zielwerte der
Fälle 1 und 2 sind auf dem iPhone damit **vom Aufbau her gefährdet**. M-5 würde Listen mit einem, drei und
sechs Segmenten messen; ohne Gerät bleibt es ungeprüft. Auswege (`05` 4.3): kurze Segmente in der Startphase
(O-19), längerer Anlauf, `hls.js` statt nativ ab iOS 17.1, bis zu sechs Segmente für native Apple-Player
auch ohne Recht «Timeshift» (O-28, berührt LTV-007, nur als Frage an Marco).

## 4. Matrix C · Videocodec × Client

Ergebnis: **direkt** / **Bild umwandeln**. Grundlage: `04` 7.6 und 7.2 (Fassung 3). **Der Server entscheidet
aus den gemeldeten Gerätefähigkeiten**, die für Live-TV eine **eigene Angabe** sind: was der Player in
TS-Segmenten abspielt (T-14). Die Tabelle zeigt die Vorgabe, wenn ein Gerät nichts anderes meldet. Fehlt eine
Angabe, gilt «H.264 ohne Zeilensprung, AAC».

Spalte «Safari macOS, iPad» heisst: spielt über `hls.js` im schlanken Bau, also dieselben Grenzen wie die
Browser; wählt das iPad das native HLS, gilt die Spalte iPhone. **Alle Safari-, iPhone- und iPad-Zellen sind
ungeprüft, nicht zugesagt.**

| Videocodec | Browser: Chrome, Edge, Firefox, Chrome Android (`hls.js`, schlank) | Safari macOS, iPad (`hls.js`, schlank) | iPhone (nativ) | Handy-App | TV-App |
|---|---|---|---|---|---|
| **H.264 progressiv** (8 Bit) | **direkt** · belegt (Nachprüfung Nr. 13: schlanker Bau liest H.264 in TS); Gerät zu messen (M-3) | **direkt** · wie Browser; ungeprüft, nicht zugesagt | **direkt**; H.264 in TS erlaubt, belegt (Nachprüfung Nr. 8); Verhalten ungeprüft, nicht zugesagt | **direkt** · Annahme (M-4) | **direkt** · Annahme (M-4) |
| **H.264 interlaced** (576i, 1080i) | **Bild umwandeln** (T-15: ein Browser meldet «stellt Zeilensprung sauber dar» nie) · zu messen (M-6) | **Bild umwandeln**, bis M-6 · ungeprüft, nicht zugesagt | **Bild umwandeln**, bis M-6; Apple entfernt den Zeilensprung nach Annahme selbst · ungeprüft, nicht zugesagt | **Bild umwandeln**, ausser das Gerät meldet saubere Darstellung (Handys eher nicht) · Annahme, zu messen (M-6) | **direkt**, wenn das Gerät meldet (Fernseher meist) · Annahme, zu messen (M-6) |
| **HEVC 8 und 10 Bit** | **Bild umwandeln**: der schlanke Bau liest HEVC in TS nicht (belegt (Nachprüfung Nr. 13)); Original nur mit vollem Bau, den Live-TV nicht lädt (O-22). Browser-Decoder ohnehin geräteabhängig (belegt (Nachprüfung Nr. 21)). | **Bild umwandeln** · belegt (Nachprüfung Nr. 13); ungeprüft, nicht zugesagt | **Bild umwandeln**: HEVC in TS ist nach Apples Vorgaben nicht erlaubt, nur in fMP4 (belegt (Nachprüfung Nr. 8)); zweites Segmentformat ist O-3 · ungeprüft, nicht zugesagt | **direkt** bei Hardware-Decoder · Annahme, zu messen (M-7) | **direkt** bei Hardware-Decoder · Annahme, zu messen (M-7) |
| **HDR** (HDR10, HLG, Dolby Vision; HEVC oder AV1) | **Bild umwandeln** (mit bestehendem Tonemapping auf SDR, `04` 7.2); Folge der HEVC-Zeile | **Bild umwandeln** · ungeprüft, nicht zugesagt | **Bild umwandeln** · ungeprüft, nicht zugesagt | **direkt** nur bei Hardware-Decoder **und** HDR-Display, sonst **Bild umwandeln** · Annahme (M-7) | wie Handy; Fernseher mit HDR-Display eher direkt · Annahme (M-7) |
| **MPEG-2** | **Bild umwandeln**: Chrome, Edge, Firefox lesen es nicht (belegt (Nachprüfung Nr. 22)); `hls.js` demuxt es nie (belegt, Nr. 13) | **Bild umwandeln**; MDN führt Safari als unterstützend, Apples Codecliste nennt MPEG-2 nicht: nur messen, nicht zusichern (belegt (Nachprüfung Nr. 22)); ungeprüft, nicht zugesagt | **Bild umwandeln**; MPEG-2 steht nicht in Apples Codecliste (belegt (Nachprüfung Nr. 8)) · ungeprüft, nicht zugesagt | **direkt**, wenn das Gerät MPEG-2 dekodiert (der TS-Leser von Media3 liest H.262, belegt (Nachprüfung Nr. 19); viele Handys ohne MPEG-2-Hardware, Annahme), sonst **Bild umwandeln** · zu messen (M-7) | **direkt**, wenn das Gerät MPEG-2 dekodiert (viele Fernseher können es, Annahme) · zu messen (M-7) |
| **AV1** | nicht eigens entschieden in `04`/`05`. Vorgabe **Bild umwandeln**, bis die Fähigkeitsprobe des Clients (`isTypeSupported`, im Repo vorhanden) es für TS-Segmente bestätigt · Annahme | wie Browser · Annahme; ungeprüft, nicht zugesagt | wie Browser; nur neuere Chips · Annahme; ungeprüft, nicht zugesagt | Hardware-Decoder abhängig; Media3 nutzt nur den Gerätedecoder (belegt, `supported-formats`) · zu messen (M-7) | wie Handy · zu messen (M-7) |

Anmerkungen:

- **Firefox** hängt bei H.264 und AAC an den Codecs des Betriebssystems (R2, offener Punkt 11): auf allen drei
  Systemen **zu messen (M-3)**. HEVC im Browser: Chrome ab 107, Firefox ab 120, beides teils nur mit
  Hardware-Decoder (belegt (Nachprüfung Nr. 21)); auf MSE nicht übertragbar ohne Messung.
- **Zeilensprung kostet am meisten.** Jeder Browser und das iPhone bekommen bei Sendern mit Zeilensprung eine
  **Bild-Umwandlung**, bis M-6 etwas anderes belegt; danach kann eine Einstellung «Zeilensprung im Browser
  durchreichen» kommen (O-23).
- Der Server baut dafür einen neuen Baustein in die Videokette (T-15). Vorgesehen sind `bwdif` (Software),
  `deinterlace_qsv`, `deinterlace_vaapi` und `bwdif_cuda`; alle Filter sind im Image 1.41.3 vorhanden
  (belegt, `M1`; die FFmpeg-Dokumentation zeigte sie in der Nachprüfung nicht, Nr. 30: Beleg bleibt die
  Messung); die Kosten je Backend sind **zu messen (M-8)**.

## 5. Matrix D · Tonformat × Client

Grundlage: `04` 7.3 und 7.6 (Fassung 3). Regel aus ADR 0041, angewandt auf TS: AAC wird kopiert; AC-3,
E-AC-3, MP2 und AAC-LATM werden kopiert, wenn das Gerät sie **für TS-Segmente** meldet, sonst zu AAC neu
kodiert. Ein Sender ohne Tonspur ist ein gültiger Fall (`05` 9.3).

Gleiche Spaltenbedeutung wie in Matrix C. **Alle Safari-, iPhone- und iPad-Zellen sind ungeprüft, nicht
zugesagt.**

| Tonformat | Browser (`hls.js`, schlank) | Safari macOS, iPad (`hls.js`, schlank) | iPhone (nativ) | Handy-App | TV-App |
|---|---|---|---|---|---|
| **AAC-LC** | **direkt** · belegt (Nachprüfung Nr. 13: AAC-ADTS in TS); Gerät zu messen (M-3) | **direkt** · wie Browser; ungeprüft, nicht zugesagt | **direkt** · Annahme; ungeprüft, nicht zugesagt | **direkt** · Annahme (M-4) | **direkt** · Annahme (M-4) |
| **HE-AAC** (SBR) | **direkt** · Annahme, zu messen (M-3, M-7: HE-AAC ist dort aufgenommen) | **direkt** · Annahme; ungeprüft, nicht zugesagt | **direkt** · Annahme; ungeprüft, nicht zugesagt | **direkt** · Annahme (M-7) | **direkt** · Annahme (M-7) |
| **AAC-LATM** (DVB) | **Ton umwandeln**: `hls.js` liest LATM in TS in keinem Bau (belegt (Nachprüfung Nr. 13)) | **Ton umwandeln** · belegt (Nr. 13); ungeprüft, nicht zugesagt | **Ton umwandeln** · Annahme; ungeprüft, nicht zugesagt | **direkt**, wenn das Gerät den Ton dekodiert; der TS-Leser von Media3 liest LATM (belegt (Nachprüfung Nr. 19)) · Gerät zu messen (M-7) | wie Handy |
| **MP2** (MPEG-1 Layer II) | **Ton umwandeln**; direkt nur, wenn der Browser MP3-Ton meldet und `hls.js` ihn durchreicht (belegt (Nachprüfung Nr. 13): MPEG-Audio geht als `mp3` an die Medienschnittstelle, nur wenn der Browser es meldet) · zu messen (M-3) | wie Browser · ungeprüft, nicht zugesagt | **Ton umwandeln**: Apples Vorgaben nennen MP2 nicht (belegt (Nachprüfung Nr. 8)) · ungeprüft, nicht zugesagt | **direkt**, wenn das Gerät MP2 dekodiert (Media3 liest MPEG-Audio, belegt (Nachprüfung Nr. 19)), sonst **Ton umwandeln** · zu messen (M-7) | wie Handy |
| **AC-3** | **Ton umwandeln**: der schlanke Bau liest AC-3 in TS nicht (belegt (Nachprüfung Nr. 13)) | **Ton umwandeln** · belegt (Nr. 13); ungeprüft, nicht zugesagt | **direkt**: Apples Vorgaben erlauben AC-3 und E-AC-3 (belegt (Nachprüfung Nr. 8)); das Verhalten in TS ist unbekannt (M-5) · ungeprüft, nicht zugesagt | **direkt** nur mit AC-3-Decoder oder Durchreichen, sonst **Ton umwandeln** · zu messen (M-7) | **direkt** mit passender Tonanlage oder Decoder, sonst **Ton umwandeln** · zu messen (M-7) |
| **E-AC-3** | **Ton umwandeln**: E-AC-3 liest `hls.js` in TS auch im vollen Bau nicht (belegt (Nachprüfung Nr. 13)) | **Ton umwandeln** · belegt (Nr. 13); ungeprüft, nicht zugesagt | wie AC-3 · ungeprüft, nicht zugesagt | wie AC-3 | wie AC-3 |
| **Mehrere Tonspuren** | Der **Server** wählt die Spur: Er lässt die anderen Ton-PIDs samt angepasster Tabelle weg (T-12). Der Wechsel erzeugt eine neue Ausgabe-Generation (`04` 7.5). Alternative Tonspuren in der Liste braucht Live-TV nicht; der schlanke Bau lässt sie ohnehin weg (belegt (Nachprüfung Nr. 11)). | wie Browser | wie Browser; natives HLS könnte `EXT-X-MEDIA` selbst anbieten (Annahme, nicht vorgesehen) | **Server**, ausser das Gerät wählt Spuren selbst (`selectsTracks`, ADR 0041) | wie Handy |

**Ton ist die häufigste Fehlerquelle im Browser:** MP2 und LATM sind in DVB-nahen Quellen üblich (Annahme,
R2). Dass `hls.js` sie nicht liest, ist für LATM, AC-3 und E-AC-3 belegt; ob **MSE-Browser** sie selbst
dekodieren würden, hat die Nachprüfung nicht klären können (Nr. 23: MDN deckt es nicht ab): **Annahme**,
aber für die Vorgabe ohne Folge, weil `hls.js` die Spur gar nicht erst übergibt. Darum ist «nur Ton
umwandeln» der billige Standardweg. Mehrkanal-AC-3 zu Mehrkanal-AAC wäre eine zweite Stufe (R2,
**Empfehlung**, nicht entschieden).

## 6. Matrix E · Untertitel × Client

Grundlage: `04` 7.4 (Fassung 3), `05` M-24, Nachprüfung Nr. 11, 12, 17, 19. Im Browser und auf dem iPhone gibt
es Untertitel in den ersten Stufen **nur eingebrannt** (Bild-Umwandlung, bestehende Überlagerung
`pipeline.go:327`). «Individuelle Untertitelauswahl» (LTV-004) ist dort erfüllt, aber teuer.

| Untertitelart | Anzeige live: Browser (`hls.js`, schlank) | Safari macOS, iPad / iPhone | Handy- und TV-App | Erhalt in der Aufnahme |
|---|---|---|---|---|
| **DVB-Subtitles** (Bild) | nur **eingebrannt** (Bild umwandeln) | nur **eingebrannt** · ungeprüft, nicht zugesagt | Der TS-Leser von Media3 hat einen DVB-Untertitel-Leser (belegt (Nachprüfung Nr. 19)); ob die App sie zeichnet: **zu messen (M-24)**; sonst einbrennen | **ja**: Aufnahmen behalten alle Spuren (T-16, `05` 2.7: alle PIDs bleiben erhalten) · spezifiziert |
| **Teletext-Untertitel** | nicht angeboten | nicht angeboten · ungeprüft, nicht zugesagt | **kein Client zeichnet sie:** Media3 hat keinen Teletext-Leser (belegt (Nachprüfung Nr. 19)) | **ja** (PID bleibt in der Aufnahme) · spezifiziert |
| **WebVTT** (Textspur in der HLS-Liste) | **Der schlanke Bau lässt Untertitel-Spuren weg** (belegt (Nachprüfung Nr. 11)); Live-TV lädt den vollen Bau nicht. Umsetzung von Teletext oder DVB-Text nach WebVTT ist O-5 (**Stufe E**, `libzvbi` ist im Image, belegt `M1`). Wie der Browser sie anzeigt, ist offen (Abschnitt 10, Punkt 16). | nativ ja (Annahme) · ungeprüft, nicht zugesagt | Media3: WebVTT in HLS ja (belegt (Nachprüfung Nr. 17)) | nicht anwendbar (kein Eingangsformat; würde erst erzeugt) |
| **CEA-608/708** (im Bildstrom, SEI) | **nicht belegt, ob der schlanke Bau sie zeigt:** weder README noch Bauschalter nennen CEA-608/708 unter den Auslassungen; ob die Anzeige ohne Untertitel-Controller geht, ist nicht gelesen. Die Fassung 2 sagte «vermutlich nicht»; dafür gibt es keinen Beleg (Nachprüfung Nr. 12). **Zu messen am gebauten schlanken Bau (M-3).** | nativ ja (Annahme); über `hls.js` wie Browser · ungeprüft, nicht zugesagt | Media3 zeigt 608 und 708 bei HLS (belegt (Nachprüfung Nr. 17)) | im Bildstrom enthalten, bei Original-Aufnahme mit gespeichert (Annahme); nach einer Bild-Umwandlung **zu messen** |

Weil Aufnahmen immer alle Spuren behalten, hängt der Erhalt nicht vom Client ab; die **Wiedergabe** einer
Aufnahme richtet sich dann nach der Matrix.

## 7. Matrix F · Funktionen × Client

Spalten wie in Matrix C. **Alle Safari-, iPhone- und iPad-Zellen sind ungeprüft, nicht zugesagt.**

| Funktion | Browser (`hls.js`) | Safari macOS, iPad (`hls.js`) | iPhone (nativ) | Handy-App (Media3) | TV-App (Media3) |
|---|---|---|---|---|---|
| **Umschalten (Zapping), Player-Wiederverwendung** | Eine `Hls`-Instanz und ein Bildelement über alle Umschaltungen; `loadSource` mit der neuen Liste. Ob das schneller ist als ein Neuaufbau, steht in der Dokumentation nicht (belegt (Nachprüfung Nr. 14)): **zu messen (M-3)**. Einstellungen: `startPosition` vom Server, `liveSyncDurationCount: 2`, **`lowLatencyMode: false`** (ab Werk an, belegt (Nachprüfung Nr. 9)), `startFragPrefetch: true`, `maxBufferLength: 12`. | wie Browser | neue Adresse am Bildelement; die Plattform baut ihre Kette neu auf; der erste Start braucht wohl eine Nutzergeste (Annahme) · zu messen (M-5, kein Gerät) | `setMediaItem` und `prepare` auf derselben `ExoPlayer`-Instanz, Oberfläche bleibt; ob der Decoder weiterläuft, hängt am Gerät · **zu messen (M-4)**. Ladepuffer: Vorgabe sind die **Standardwerte von 1.11.1: Start 1000 ms, nach Stocken 2000 ms, Vorlauf 50 s** (belegt (Nachprüfung Nr. 16)), also kein eigener `LoadControl`; 500 ms nur, wenn M-4 es verlangt. | wie Handy; **kein** zweiter Player für Nachbarsender (Decoder sind knapp, `05` 5.3). Vorgeladen wird nur im Server. |
| **Timeshift: Fenster** (Stufe 4, nur mit Recht) | Fenster von der Rückspulgrenze bis zum neuesten Segment; Spulen im `seekable`-Bereich (R2; Annahme). Ohne Recht: höchstens drei neueste Segmente, ältere 404. | wie Browser | `seekable` (Annahme); ohne Recht: Liste unter sechs Segmenten als bewusste Abweichung von Apples Vorgaben (Abschnitt 3.2) | `seekTo` im Fenster (belegt (Nachprüfung Nr. 18)); Zeitleiste in Wanduhrzeit über `Window.windowStartTimeMs` (belegt, Media3 `live-streaming`) | wie Handy; lange Listen (60 Minuten und mehr) auf schwachen Fernsehern **zu messen (M-19)** |
| **Timeshift: Verhalten am Fensterende** | **`hls.js` springt ab Werk nicht von selbst an die Live-Kante** (`liveMaxLatencyDurationCount` ist unbegrenzt, belegt (Nachprüfung Nr. 10)). Ohne Zeitversatz setzt Live-TV `liveMaxLatencyDurationCount: 4`, weil ältere Segmente weg sind. Mit Zeitversatz bleibt der Abstand unbegrenzt, und der **Client** setzt am Fensteranfang fort (`04` 7.5, `05` 5.3); das Verhalten von `hls.js` beim Herausfallen am Listenanfang ist **zu messen (M-3)**. | wie Browser | läuft aus dem Fenster, springt zur Kante (Annahme) · zu messen (M-5, kein Gerät) | Fehler `BEHIND_LIVE_WINDOW`; Vorgabe der Doku: `seekToDefaultPosition()` und `prepare()`; die App muss ihn abfangen (belegt (Nachprüfung Nr. 18)) | wie Handy |
| **Aufholen zur Live-Kante durch schnelleres Abspielen** | aus: `maxLiveSyncPlaybackRate: 1` (Vorgabe von `hls.js`, belegt (Nachprüfung Nr. 9)) | aus | nicht möglich (Annahme) | aus: kleinste und grösste Geschwindigkeit 1,0 (belegt (Nachprüfung Nr. 18)) | aus |
| **Aufnahme-Wiedergabe** (Stufe 5) | gleiche Bauweise wie Live; eine fertige Aufnahme bekommt eine **abgeschlossene** Liste, eine laufende eine wachsende (`05` 4.4) · spezifiziert | wie Browser | wie Browser | wie Browser | wie Browser |
| **Tonspurwahl** | Server (T-12); Wechsel = neue Ausgabe-Generation, Liste wird neu geladen | wie Browser | wie Browser | Server, ausser `selectsTracks` (ADR 0041) | wie Handy |
| **Untertitelwahl** | nur eingebrannt (Matrix E); CEA-608/708 am schlanken Bau zu messen (M-3) | wie Browser | nur eingebrannt; CEA nativ (Annahme) | Menü der App; DVB und CEA über Media3 (Matrix E; M-24) | wie Handy |
| **Qualitätswahl** | Server: gewählte Qualität unter der Quelle erzeugt eine **Bild-Umwandlung** und eine neue Ausgabe-Generation (`04` 7.1, 7.5). Bei HLS-Quellen mit mehreren Varianten gilt die höchste (O-14). | wie Browser | wie Browser | wie Browser | wie Browser |
| **Umwandlung über 24 Stunden** | Kein FFmpeg-Lauf geht über einen Überlauf der Quellzeit (26,5 Stunden) und keiner über 24 Stunden; danach neue Ausgabe-Generation, der Zuschauer sieht höchstens einen kurzen Sprung (`04` 7.5). Verhalten der Player im Direktweg am Überlauf **zu messen (M-22)**. | wie Browser | wie Browser · zu messen (M-22, kein Gerät) | wie Browser | wie Browser |
| **Hintergrundwiedergabe, Bildschirm aus** | **Offen (O-25):** Der Herzschlag der Sitzung (alle 10 s, Ende nach 60 s ohne, T-21) kann in einem Tab im Hintergrund oder bei Bildübertragung ausfallen; bis M-3 es zeigt, gilt die Regel der 60 Sekunden ohne Ausnahme. | wie Browser | wie Browser; dazu Sperrbildschirm · zu messen (M-5, kein Gerät) | **Die Sitzung endet:** Geht die App in den Hintergrund oder der Bildschirm aus, hält der Player an, der Client beendet die Sitzung (spätestens nach 60 s ohne Herzschlag); beim Zurückkommen stellt die App neu ein. Kein Ton im Hintergrund, kein Bild-in-Bild (`04` 10.3, `05` 5.3, LTV-016). | wie Handy |
| **Fernbedienung, Tasten** | Tastatur und Zeigegerät; je neue Seite Tastaturtest (Regel 8) | wie Browser | Touch | Touch | D-Pad. **Gebaut:** nur Medientasten `MEDIA_PLAY_PAUSE`, `PLAY`, `PAUSE`, `STOP`, `NEXT`, `PREVIOUS`, `FAST_FORWARD`, `REWIND` (`TvRemoteKeys.kt:39-46`). **Fehlt:** `CHANNEL_UP/DOWN`, `GUIDE`, `INFO`, `LAST_CHANNEL`, `DVR` und Ziffern im Player; ↑/↓ und ←/→ sind im Player belegt (Bedienung zeigen, spulen). |

## 8. Was das für den Normalfall heisst

**Der Normalfall hängt an deinen Sendern, und die kennt noch niemand.** Wie viele Sender Zeilensprung, MP2,
AC-3 oder MPEG-2 haben, zeigt erst **M-2** auf deinen echten Quellen. Als Arbeitsannahme gilt das Profil
eines typischen IPTV-Senders: H.264 oder MPEG-2, oft mit Zeilensprung, Ton MP2 oder AC-3 (**Annahme**).
Dann fällt der Direktweg nach `04` 7.6 (Fassung 3) so aus:

| Typischer Sender | Browser, Safari macOS, iPad (`hls.js`) | iPhone (ungeprüft, nicht zugesagt) | Handy-App | TV-App |
|---|---|---|---|---|
| **H.264 1080i50 (Zeilensprung), Ton MP2 oder AC-3** | Bild umwandeln (inkl. Entfernen des Zeilensprungs) | Bild umwandeln, bis M-6 | Bild umwandeln, ausser Gerät meldet saubere Darstellung; Ton direkt, wenn das Gerät ihn dekodiert | direkt, wenn Gerät Zeilensprung und Ton meldet; sonst Ton oder Bild umwandeln |
| **MPEG-2 576i50, Ton MP2** | Bild umwandeln | Bild umwandeln | direkt nur, wenn das Gerät MPEG-2 dekodiert und Zeilensprung sauber zeigt, sonst Bild umwandeln | wie Handy; Fernseher können es oft (Annahme) |
| **H.264 ohne Zeilensprung, Ton MP2** | nur Ton umwandeln; direkt nur, wenn der Browser MP3-Ton meldet (M-3) | nur Ton umwandeln | direkt, wenn das Gerät den Ton dekodiert, sonst nur Ton umwandeln | wie Handy |
| **H.264 ohne Zeilensprung, Ton AC-3 oder E-AC-3** | **nur Ton umwandeln** (belegt (Nachprüfung Nr. 13)) | direkt: Apples Vorgaben erlauben beide, Verhalten in TS unbekannt | direkt mit passender Tonanlage, sonst nur Ton umwandeln | wie Handy |
| **H.264 ohne Zeilensprung, Ton AAC-LATM** | **nur Ton umwandeln** (belegt (Nachprüfung Nr. 13)) | nur Ton umwandeln (Annahme) | direkt, wenn das Gerät den Ton dekodiert | wie Handy |
| **HEVC** | **Bild umwandeln** (belegt (Nachprüfung Nr. 13)) | **Bild umwandeln** (belegt (Nachprüfung Nr. 8)) | direkt bei Hardware-Decoder | direkt bei Hardware-Decoder |
| **H.264 ohne Zeilensprung, AAC** | **direkt** | **direkt** | **direkt** | **direkt** |

**Ehrlich je Client:**

- **Browser, Safari macOS, iPad:** Direkt gibt es nur für H.264 ohne Zeilensprung mit AAC. Alles andere kostet
  einen FFmpeg-Prozess: billig bei «nur Ton» (0,25), teuer bei «Bild» (2 für HD). AC-3, E-AC-3, LATM und
  HEVC gehen im Browser **immer** in die Umwandlung, weil der schlanke Bau sie nicht liest und Live-TV den
  vollen nicht lädt (`05` 4.3). Bei Sendern nach dem Profil oben ist der Direktweg im Browser **die
  Ausnahme, nicht der Normalfall** (Annahme, bis M-2).
- **iPhone:** wie Browser bei Zeilensprung und MPEG-2; offen, ob Apple den Zeilensprung selbst entfernt (M-6)
  und ob AC-3 in TS direkt geht (M-5). HEVC bleibt umgewandelt (O-3). Alles ungeprüft, nicht zugesagt.
- **Handy-App:** Hängt an Gerät und Meldung: MPEG-2 und MP2 sind auf Handys nicht gesichert (M-7);
  Zeilensprung zeigen Handys «eher nicht» sauber (Annahme).
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
Zuschauer desselben Senders sind nur im Direktweg realistisch. Auf der Klasse «klein» (Software-Backend,
2 Kerne) gibt es genau **eine** Bild-Umwandlung zur selben Zeit, für Live **oder** für einen Film, nie beides,
dazu höchstens zwei Ton-Umwandlungen (`04` 7.6). Das folgt aus LTV-004 (keine gemeinsame Umwandlung). Die
Rechnung für die drei Installationsgrössen steht in `13-ressourcen-und-performance.md`.

## 9. Am Gerät zu messen

Messpunkte aus `05` 10.1 (Fassung 3), soweit sie diese Matrizen tragen. **Nichts ist gemessen.** Geräte nach
`05` 8.3: Chrome und Firefox (Desktop), Pixel (das Gerät, mit dem K27 geprüft wurde), ein Android-TV-Gerät
(Modell notieren). **Ein Apple-Gerät gibt es nicht:** M-5 kann nicht laufen; WebKit in Playwright wäre nur
eine Näherung (`18` V-3). Referenzserver: CT 190 (`05` 8.1).

**Kriterium für Umschaltzeiten** (`05` 8.5, Vorschlag; verbindlich erst in `17`): Median unter dem Zielwert
(Fall 1 und 2 unter 1 s, Fall 3 unter 3 s, Fall 4 unter 5 s) in den Netzen N-1 und N-2, 95. Perzentil höchstens
das Doppelte, Fehlerquote höchstens 2 %. 50 gewertete Umschaltungen je Reihe.

| Nr. | Was | Gerät | Kriterium | Muss vorliegen vor |
|---|---|---|---|---|
| **M-1** | Umschaltzeit je Fall, Client, Netz | alle Clients | siehe oben | Abnahme jeder Stufe mit Wiedergabe |
| **M-2** | Deine echten Quellen: Anteil H.264 ohne Zeilensprung mit AAC, Zeilensprung, MPEG-2, MP2, AC-3; Bildgruppen-Länge | kleines Messwerkzeug auf dem Server | Kennzahlen; entscheiden, wie oft der Direktweg greift | **Baufreigabe der Stufe 1** (T-11, T-12) |
| **M-3** | `hls.js` 1.7.3 **im schlanken Bau** mit TS: H.264 mit AAC und HE-AAC, MP2 (als MP3-Ton, je Browser); Diskontinuitäten; Startpunkt; Listen von drei Segmenten mit `liveSyncDurationCount` 2 und `liveMaxLatencyDurationCount` 4; Verhalten bei fehlendem Segment (404); `loadSource` gegen Neuaufbau; mit und ohne `startFragPrefetch`; **Untertitel im Bildstrom (CEA-608/708)**; Herzschlag im Tab im Hintergrund. Im vollen Bau zusätzlich HEVC und AC-3 in TS und die Grösse des Zusatzteils (nur für O-22). **Nicht zu messen, weil belegt:** E-AC-3, AAC-LATM, MPEG-2. | Chrome, Firefox (Testquellen `05` 8.2) | spielt ohne Stocken; Startzeit | **Baufreigabe der Stufe 1** |
| **M-4** | Media3 1.11.1 mit HLS-TS: Start mit Standard-Startpuffer (1000 ms) und mit 500 ms, je mit Stockungen in den ersten 10 s; Start mit und ohne übergeordnete Liste (`CODECS`; `setAllowChunklessPreparation` ist nicht geprüft, Nachprüfung Nr. 20: Annahme); Decoder-Wiederverwendung; Verhalten hinter dem Fenster; lange Listen | Pixel, Android-TV-Gerät | Startzeit, Stallrate, Decoder-Reuse ja/nein | Baufreigabe der Stufe, in der die Android-Apps Live-TV bekommen (`03` F-6) |
| **M-5** | Safari (Mac, iPhone, iPad, installierte App) mit HLS-TS: Zeit bis zum Bild bei einer Liste mit einem, drei und sechs Segmenten; Startpunkt; AC-3 in TS; welchen Weg die App auf dem iPad wählt; Herzschlag im Hintergrund und bei Bildübertragung. Safari-LL-HLS und `getStartDate()` sind nicht geprüft (Nachprüfung, Abschnitt 5). HEVC in TS ist nicht mehr zu messen (belegt: nicht erlaubt). | **kein Gerät vorhanden** | Startzeit je Listenlänge; Herzschlag bricht nicht ab | Zusage für Safari, iPhone, iPad; bis dahin ungeprüft. Keine andere Baufreigabe hängt daran. |
| **M-6** | Zeilensprung je Client: Kammartefakte bei 1080i50 und 576i50, auch im Browser | alle Clients mit Q-E, Q-F (Apple: kein Gerät) | Kämme sichtbar ja oder nein; Entscheid «Server entfernt immer» oder «Gerät schafft es» | Festlegung der Vorgabe in T-15; `04` O-23 |
| **M-7** | Decoder je Gerät: MPEG-2, MP2, AAC-LATM, HE-AAC, AC-3, E-AC-3, HEVC, AV1 | Pixel, Android-TV-Gerät, Browser (`isTypeSupported`) | Wie viele Geräte bräuchten wirklich eine Umwandlung? | Festlegung der Fähigkeitsangaben (`04` O-1) |
| **M-8** | Live-Umwandlung: Zeit bis zum ersten Segment je Backend; Filter gegen Zeilensprung und ihre Kosten; Kosten der Ton-Umwandlung | CT 190 (QSV), Entwicklungsrechner (NVENC), Software | Zeit, Kosten je Zuschauer; trägt die Referenz die Ton-Umwandlung? | Bau der Stufe mit Umwandlung; Budgetwerte `04` 7.6 |
| **M-9** | Nahtlose Segmente über mehrere FFmpeg-Läufe mit TS; Zeitsprung im Eingang | Pixel, Android-TV, Chrome, Firefox (Apple: kein Gerät) | ohne sichtbaren Sprung | Bau des Spulens mit Umwandlung (Stufe 4); sonst Rückfall `05` 4.5 |
| **M-10, M-11** | Weg W dicht; FFmpeg-Leser für DASH-Live und HLS-fMP4 mit getrenntem Ton | Image, Testaufbau | Mitschrift zeigt nur den Port des Weiterleiters; Stabilität | Baufreigabe von Weg W (Stufe W) |
| **M-16** | UDP und Multicast im Container: kommt der Beitritt im Bridge-Netz wirklich nicht an? Bridge, Host-Netz, `macvlan` | CT 190, Heimnetz | Paketverlust unter Last | Bau der Stufe H (F-11) |
| **M-18** | Verhalten hinter dem Reverse-Proxy (Nginx Proxy Manager, Caddy): Abruf der Segmente, Zwischenpufferung | dein Aufbau | Umschaltzeit gegenüber direkt | Abnahme der ersten Stufe |
| **M-19** | Lange Wiedergabelisten (60 Minuten und mehr): Zeit zum Einlesen | Android-TV-Gerät | Parsezeit, Ruckler | Bau des Zeitversatzes |
| **M-22** | Zeitüberlauf nach 26,5 Stunden: Verhalten der Player im Direktweg; FFmpeg 7.1.4 beim ersten und zweiten Überlauf (rund 53 Stunden) | Testquelle mit vorgestelltem Zeitstempel, alle Player | spielt ohne Störung durch | Bau der ersten Stufe |
| **M-24** | Untertitel: DVB-Untertitel in Media3 über HLS-TS (Teletext hat dort keinen Leser); Umsetzung nach WebVTT | Pixel, Android-TV, Testquelle mit Untertiteln | wird gezeichnet | Stufe mit Untertiteln (Stufe E) |

**Lücken ohne eigenen Messpunkt** (Vorschlag, sie unter den genannten Punkten mitzumessen). HE-AAC, AV1 und
der schlanke Bau von `hls.js` sind seit `05` Fassung 3 in M-3 und M-7 aufgenommen und stehen hier nicht mehr.

| Lücke | Unter | Warum offen |
|---|---|---|
| HEVC 10 Bit und HDR: Hardware-Decoder und Tonemap-Pfad | M-7 | nur als Annahme |
| Firefox: H.264, AAC über OS-Codecs auf Windows, Linux, macOS | M-3 | R2, offener Punkt 11 |
| `EXT-X-PROGRAM-DATE-TIME`: Genauigkeit der Zeitleiste in allen Playern | M-3, M-4 | R2 Abschnitt 8, Punkt 9 |
| AC-3, E-AC-3, MP2, AAC-LATM in MSE-Browsern ohne `hls.js` | – | Nachprüfung Nr. 23: nicht prüfbar; für die Vorgabe ohne Folge |
| Fire OS: startet die TV-App überhaupt? | nur, wenn jemand es will | Nicht-Ziel |

## 10. Offene Punkte und Widersprüche

Wo R2 oder R1 und `05` sich widersprechen, gilt `05`. «Entschieden in 05 Fassung 3» heisst: erledigt, steht
dort mit Beleg; die Zeile bleibt als Spur.

| Nr. | Punkt | Was die Recherche sagt | Stand | Auswirkung auf die Matrix |
|---|---|---|---|---|
| 1 | **Android: progressives TS oder HLS** (W-1) | R1: Android bekommt den Strom progressiv. R2: progressives TS hat bei Media3 kein Live-Fenster, also kein Timeshift im Player (belegt (Nachprüfung Nr. 18)). | **Entschieden in 05 Fassung 3:** HLS mit TS für alle; progressiv nicht verfolgen, ausser M-1 zeigt einen grossen Abstand. | Matrix B: progressiv nur «kann». |
| 2 | **Basisformat fMP4 (R2 Variante A) oder TS** | R2 empfiehlt fMP4; MPEG-2, MP2, LATM passen nicht hinein (Annahme). | **Entschieden (T-11):** HLS-TS; fMP4 ist die Stelle für ein zweites Segmentformat, der Rückfall und der Weg für HEVC auf Apple (O-3). | Matrix B und C. |
| 3 | **Schlanker oder voller Bau von `hls.js`** (`24` N-7, `04` O-22) | R2: lässt eventuell Alt-Audio, Untertitel, CEA weg; nicht nachgelesen. | **Entschieden in 05 Fassung 3:** kein Voll-Bau. Belegt: schlanker Bau liest in TS kein HEVC, kein AC-3, E-AC-3 und LATM keiner (Nachprüfung Nr. 11, 13). Der volle Bau bleibt Option, wenn M-2 viele AC-3-/HEVC-Sender zeigt. | Matrix C, D, Abschnitt 8 nachgeführt. **Offen:** CEA-608/708 am schlanken Bau (M-3); die Fassung-2-Aussage «vermutlich nicht» ist ohne Beleg. |
| 4 | **Startpunkt in `hls.js`** (W-6) | R1: `liveSyncDurationCount` 1. R2: 2. | **Entschieden in 05 Fassung 3:** Startpunkt vom Server, Wert bleibt 2. | Matrix F «Umschalten». |
| 5 | **Standardwerte des Ladepuffers in Media3** (W-5) | R1: 1000/2000 ms. R2: 50'000/50'000/2500/5000 ms. | **Entschieden (Nachprüfung Nr. 16):** R2 widerlegt, 1.11.1 hat Start 1000 ms, nach Stocken 2000 ms, Vorlauf 50 s. Vorgabe: diese Standardwerte; 500 ms nur nach M-4. | Matrix F. |
| 6 | **Startregel und Apples Vorgaben auf Safari und iPhone** (W-11) | R2/R3: RFC 8216 nicht abgerufen. | RFC-Regel **belegt** (Nachprüfung Nr. 1), W-11 aufgelöst. Apple verlangt mindestens 6 Segmente; Konzept weicht **bewusst** ab (`05` 4.3). Zusage wegen fehlenden Geräts nicht möglich. **Offen:** O-28 (sechs Segmente auch ohne Recht, berührt LTV-007). | Abschnitt 3.2. |
| 7 | **Ton und Zeilensprung: R2-Vorgaben gegen `04`** | R2: Ton immer zu AAC-LC; Interlaced für Browser und Handy immer entfernen. | **Entschieden (T-14, T-15, `04` 7.3):** kopieren, wenn das Gerät für TS meldet; Zeilensprung entfernt der Server, ausser das Gerät meldet saubere Darstellung (Browser nie). | Matrix C und D folgen `04`. |
| 8 | **HEVC auf Apple: direkt (R2) oder umwandeln** | R2: Safari direkt, nur fMP4. | **Entschieden (belegt (Nachprüfung Nr. 8)):** in TS nicht erlaubt, also Bild umwandeln; zweites Segmentformat ist O-3. | Matrix C. |
| 9 | **iPad: nativ oder `hls.js`** | R2: «nativ oder `hls.js`»; Repo-Weiche wählt «mse», wo `MediaSource` existiert. | **Entschieden in 05 Fassung 3:** iPad folgt der Weiche der App; iPhone nativ. Beides ungeprüft, nicht zugesagt. | Abschnitt 1, Matrizen. |
| 10 | **Zeitversatz: Pause im Player oder im Server** | R2: Pause serverseitig, EVENT-Liste. | Sitzungspuffer mit eigenem Fenster; Rückspulgrenze nach `04` F-7; ohne Recht kein Zurückspulen. Zu `EVENT`: Liste je Zuschauer EVENT-artig ist regelkonform, das gleitende Fenster darf kein `PLAYLIST-TYPE` tragen (belegt (Nachprüfung Nr. 3)). | Matrix F. |
| 11 | **DASH, HLS-fMP4 als Eingang; RTSP, RTMP: Stufe** | `03`: Stufe 3. | **Entschieden in `18` (D-1, D-2):** Stufe L und Stufe W; UDP, RTP, Multicast in Stufe H. | Matrix A. |
| 12 | **Wer empfängt: FFmpeg oder Go** (W-3) | R3: FFmpeg. R1: Go. | **Entschieden (T-3):** Go (Weg G, L); FFmpeg nur Weg W. | Matrix A. |
| 13 | **FFmpeg-Version** (W-4) | Berichte beziehen sich auf die neueste Fassung. | Vorbehalt je Aussage bleibt; einzelne Werte an n7.1.4 belegt (Nachprüfung Nr. 24 bis 28); `max_reload` ist 100, nicht 1000 (Nr. 27). | Matrix A. |
| 14 | **LL-HLS, WebRTC** | R2 bewertet beides. | **Entschieden:** nicht verfolgt (T-11); `lowLatencyMode` ist in `hls.js` ab Werk an und wird für Live-TV auf `false` gesetzt (belegt (Nachprüfung Nr. 9)). | Matrix B, F. |
| 15 | **Sprung zur Live-Kante in `hls.js`** | R2: «setzt an der Live-Kante fort (belegt)». | **Richtiggestellt (Nachprüfung Nr. 10):** ab Werk kein Sprung; Live-TV setzt `liveMaxLatencyDurationCount: 4` für Sitzungen ohne Zeitversatz; Verhalten am Listenanfang M-3. | Matrix F. |
| 16 | **Stufe E (WebVTT im Browser) gegen «kein Voll-Bau»** | `18` Stufe E: Untertitel als WebVTT für Browser und iPhone. | **Offen, gemeldet:** Der schlanke Bau lässt Untertitel-Spuren weg (belegt (Nachprüfung Nr. 11)), Live-TV lädt den vollen Bau nicht (`05` 4.3). Wie der Browser WebVTT anzeigt (eigene Anzeige im Client oder voller Bau als eigener Teil, O-22), ist in `04`/`05`/`18` nicht festgelegt (Annahme: eigene Anzeige möglich). | Matrix E. |
| 17 | **Safari und MPEG-2** | MDN: Safari unterstützt MPEG-2. Apples Codecliste nennt es nicht. | Nur messen, nicht zusichern (Nachprüfung Nr. 22); Vorgabe Bild umwandeln. Kein Apple-Gerät. | Matrix C. |
| 18 | **Hintergrund und «Bildschirm aus»** | in `04`/`05` Fassung 2 nicht geregelt. | **Entschieden für die Apps (`04` 10.3):** Sitzung endet. **Offen (O-25):** Browser-Tab im Hintergrund und Bildübertragung. | Matrix F. |
| 19 | **`setAllowChunklessPreparation` und `CODECS`** | R2: beschleunigt den Start. | Nicht geprüft (Nachprüfung Nr. 20): Annahme, M-4. | Matrix F. |
| 20 | **Safari-LL-HLS, `getStartDate()`** | R2: Apple-Seiten nicht abgerufen. | Nicht geprüft (Nachprüfung, Abschnitt 5): Annahme; kein Gerät. | Matrix B. |

**Weitere offene Entscheidungen, die diese Matrix berühren:** F-1 (RTSP und RTMP über Go-Bibliotheken oder
FFmpeg), F-11 (Multicast: Netzmodus «host» oder eigenes Netz), F-13 (DASH und HLS-fMP4, falls Weg W nicht
dicht belegbar ist), O-1 (Form der additiven Fähigkeitsangaben der Geräte: Videocodec `mpeg2video`,
Toncodecs `mp2` und `aac_latm`, «stellt Zeilensprung sauber dar», Untertitelformate `dvb_subtitle` und
`dvb_teletext`, dazu die Angabe «in TS-Segmenten abspielbar»), O-3, O-14, O-15, O-16, O-19, O-22, O-23,
O-25, O-28 (siehe `04` 12.2 und `05` 10.2). Frühere Fragen dieser Datei: keine eigenen Fragen an Marco
(`F-n`); alle Fragen stehen in `04`.

## 11. Änderungen gegenüber der vorigen Fassung

Fassung 1 (28. November 2026) war der erste Entwurf; Fassung 2 führt die Konsolidierung 1 nach.

- **Nachprüfung eingearbeitet** (`anhang/nachpruefung-quellen.md`): Zellen, die dort bestätigt sind, tragen
  «belegt (Nachprüfung Nr. n)» (Media3-Leser, Media3 LL-HLS, `SAMPLE-AES` nein, HEVC nur in fMP4 auf Apple,
  MPEG-2 nicht in Apples Liste, `ManagedMediaSource` ab iOS 17.1, RFC-Startregel, FFmpeg-`tcp`-Whitelist).
- **Richtiggestellt:** Media3 1.11.1 Startpuffer 1000 ms, nach Stocken 2000 ms (Nr. 16, Fassung 1 nannte
  2500/5000 aus R2); `lowLatencyMode` ab Werk an, für Live-TV aus (Nr. 9); kein Sprung zur Live-Kante ab Werk
  (Nr. 10); CEA-608 im schlanken Bau «vermutlich nicht» war unbelegt (Nr. 12); `max_reload` ist 100 (Nr. 27).
- **Schlanker Bau von `hls.js` (04/05 Fassung 3):** kein HEVC, kein AC-3 in TS, E-AC-3 und LATM in keinem Bau
  (Nr. 13); kein Voll-Bau; HEVC, AC-3, E-AC-3, LATM gehen im Browser in die Umwandlung. Matrizen B, C, D,
  Abschnitt 8 nachgeführt.
- **Apple:** mindestens 6 Segmente, IDR am Segmentanfang, HEVC nur in fMP4 (Nr. 6 bis 8); Liste unter sechs
  Segmenten als bewusste Abweichung (O-28). **Kein Apple-Gerät:** alle Safari-, iPhone- und iPad-Zellen
  «ungeprüft, nicht zugesagt». iPad folgt der Weiche der App (04/05 Fassung 3).
- **Hintergrund:** In den Apps beendet er die Sitzung (`04` 10.3); Browser offen (O-25). FFmpeg-Lauf höchstens
  24 Stunden (`04` 7.5); neue Zeile in Matrix F.
- **Stufen** nach `18-umsetzungsplan.md` (1a, 3, H, L, W, E); Abschnitt 9 an `05` Fassung 3 angeglichen
  (HE-AAC, AV1, schlanker Bau in M-3 und M-7); Lücken erledigt gestrichen.
- **Abschnitt 10:** erledigte Punkte als «entschieden in 05 Fassung 3» gekennzeichnet; neu: Stufe E gegen
  «kein Voll-Bau» (Punkt 16), Safari und MPEG-2, nicht geprüfte Quellen.
- Quelle der Rückmeldungen: `KONSOLIDIERUNG.md`, `RUECKMELDUNGEN-AN-ARCH.md` (Abschnitt «Aus 16»), `04` 15.
- Restebereinigung nach Konsolidierung 1: O-28 berührt LTV-007, nicht LTV-005 (`20` U-13).
