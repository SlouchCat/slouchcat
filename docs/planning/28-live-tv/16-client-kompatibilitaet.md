# Konzept 28 · Client-Kompatibilität

Stand: 29. November 2026 · Fassung 3 · Status: Entwurf nach Gesamtreview, nicht freigegeben

Dieses Dokument beantwortet die Forderung von LTV-003: eine Kompatibilitätsmatrix für Eingangsprotokolle,
Container, Codecs und unterstützte Clients. Es verdichtet `04-architektur.md` und
`05-streaming-und-fast-zapping.md` (beide **Fassung 4**, verbindlich), `12-sicherheit.md` (Fassung 3; wo 12 strenger ist, gilt 12), `18-umsetzungsplan.md` (Stufen),
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
| **vorgesehen, ungeprüft ohne Gerät** | Für Safari, iPhone und iPad: Sie gehören als Web-Clients zum Umfang (LTV-014: Smartphone, Tablet) und sind **vorgesehen wie die übrigen Browser, aber ungeprüft, solange kein Gerät da ist** (`04` 12.1, `05` 4.3). Was dort steht, ist aus Dokumenten belegt oder Annahme, nie gemessen. Ob ein Gerät beschafft wird oder auf die Zusage verzichtet wird, ist eine Frage an Marco (`20`, E-28-12); nirgends gilt es als entschieden. |

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
| **Web: Safari macOS** | `hls.js` über MSE (`05` 4.3); natives HLS als Alternative | wie oben | `05` spezifiziert; vorgesehen, ungeprüft ohne Gerät |
| **Web: Safari iPhone** (Browser und installierte App) | natives HLS. Kein klassisches `MediaSource`; `ManagedMediaSource` ab iOS 17.1 als Ausweg über `hls.js` (hls.js-README seit 1.5.0). | «native» nur, wo es allein `canPlayType('application/vnd.apple.mpegurl')` gibt | belegt (Repo; Nachprüfung Nr. 15); vorgesehen, ungeprüft ohne Gerät |
| **Web: Safari iPadOS** (Browser und installierte App) | folgt der Weiche der App: `hls.js`, wo die klassische Medienschnittstelle vorhanden ist (auf dem iPad **Annahme**), sonst natives HLS (`05` 4.3; Fassung 2 führte das iPad fälschlich als «nativ») | `capabilities.ts` | Annahme; vorgesehen, ungeprüft ohne Gerät |
| **Android-Handy-App** | Media3 1.11.1, `ExoPlayer` mit `SlouchcatRenderersFactory`, OkHttp | `android/core/player/.../Media3Engine.kt`; setzt heute weder `LoadControl` noch `LiveConfiguration` | belegt (Repo) |
| **Android-TV-App** (deckt Google TV ab) | Media3 1.11.1, wie Handy | wie Handy; wertet `DecoderReuseEvaluation` schon aus (`PlayerFeature.kt`) | belegt (Repo) |

**Fire TV: berücksichtigt, nicht zugesagt.** LTV-014 nennt Fire TV unter «Berücksichtige»; das Repo führt Fire
TV als Nicht-Ziel (`CLAUDE.md`, Regel 11). Das Konzept löst das so: Fire TV ist **kein eigener Client und keine
Zusage**; es sagt nur, was die Android-TV-App auf Fire OS bräuchte (`05` 4.3, `04` O-29). Das ist eine
**Auslegung des Koordinators**, kein Entscheid von Marco; sie steht als Frage in `20`. Die Aussage ist
**Annahme**, es gibt kein Gerät: Fire OS ist ein Android-Abkömmling ohne Google-Dienste; die App verlangt
mindestens Android 9 (`Conventions.kt:16`, Repo); die neueren Fire-OS-Fassungen bauen nach Annahme auf
Android 9 und 11 auf; die Baudatei der TV-App bindet keine Google-Dienste ein, ob ihre Module es tun, ist
nicht geprüft. Nötig wären ein Lauf auf einem Gerät, die Verteilung ausserhalb von Google Play und die
Prüfung von Fernbedienung, Einrichtung per Scan und Decodern (M-7).

**Weiterhin Nicht-Ziele** (`CLAUDE.md`, Regel 11): Apple TV, webOS, Tizen.

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
| **HLS mit fMP4-Segmenten oder getrennter Tonspur** | W, **Stufe W**, nur nach M-10 | Weiterleiter holt alles selbst über den geschützten Dialer; FFmpeg läuft in einer Kernel-Sperre (Landlock: auf einen Port eingeengt, nicht auf den Weiterleiter; Sperre Pflicht, M-25). **Ohne wirksame Sperre läuft Weg W nicht** (kein Live-FFmpeg ohne Sperre, T-27). Dass `tcp` in der Protokoll-Liste nötig ist, ist belegt (Nachprüfung Nr. 24). Netzmodus «host»: gesperrt. | Go startet den Prozess neu, wenn er abbricht oder 10 s nichts liefert; ein FFmpeg-Lauf dauert höchstens 24 Stunden und nie über einen Überlauf der Quellzeit (`04` 7.5). `reconnect*` von FFmpeg sind in 7.1.4 ab Werk aus (belegt (Nachprüfung Nr. 26)). | Ohne belegte Dichtheit entfällt der Weg (F-13); Alternativen: nur mit Sperre, Go liest selbst (viel Eigencode) oder gar nicht (Abstrich an LTV-003, müsste von Marco kommen). | Go liest jede Liste, bevor FFmpeg sie sieht; Regel wie HLS mit TS |
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
| **Safari macOS** | `hls.js` (`05` 4.3), schlanker Bau, Grenzen wie links · vorgesehen, ungeprüft ohne Gerät | `hls.js` oder nativ · Annahme; vorgesehen, ungeprüft ohne Gerät | nativ · Annahme (Apple-Seite nicht abgerufen); vorgesehen, ungeprüft ohne Gerät | `mpegts.js` · Annahme; vorgesehen, ungeprüft ohne Gerät | nur mit Bibliothek · Annahme; vorgesehen, ungeprüft ohne Gerät |
| **iPhone** | nativ; H.264 in TS erlaubt (belegt (Nachprüfung Nr. 8)); **Apples Vorgaben: mindestens 6 Segmente in einer Live-Liste, Segment beginnt mit IDR, Uhrzeit in jeder Live-Liste** (belegt (Nachprüfung Nr. 6, 7)); Startverhalten **zu messen (M-5)** · vorgesehen, ungeprüft ohne Gerät | nativ; **HEVC nur in fMP4, nie in TS** (belegt (Nachprüfung Nr. 8)) · vorgesehen, ungeprüft ohne Gerät | nativ · Annahme; vorgesehen, ungeprüft ohne Gerät | nur `mpegts.js` ab 1.8 über `ManagedMediaSource`, iOS ab 17.1 (belegt, Beschreibung von mpegts.js) · vorgesehen, ungeprüft ohne Gerät | kein natives DASH; Bibliothek nur mit `ManagedMediaSource` · Annahme; vorgesehen, ungeprüft ohne Gerät |
| **iPad** | wie die Web-App entscheidet (`hls.js` nach Annahme, sonst nativ wie iPhone) · vorgesehen, ungeprüft ohne Gerät | wie links | wie links | wie links | wie links |
| **Android-Handy-App (Media3)** | ja: `HlsMediaSource` mit TS-Leser (belegt, Media3 `supported-formats`; Nachprüfung Nr. 17, 19) | ja (belegt, Nr. 17) | nur Apples Form, nicht die der Community (belegt (Nachprüfung Nr. 17)) | ja: `ProgressiveMediaSource`, aber **ohne Live-Fenster** und nur an einer Position abspielbar (belegt, Media3 `live-streaming`; Nachprüfung Nr. 18) | ja für fMP4 und WebM, **kein TS** (belegt (Nachprüfung Nr. 17)) |
| **Android-TV-App (Media3)** | wie Handy | wie Handy | wie Handy | wie Handy | wie Handy |

### 3.2 Empfehlung laut T-11 und Alternative

**T-11 ist eine Empfehlung unter Vorbehalt:** M-2, M-3 und M-4 müssen **vor der Baufreigabe der Stufe**
vorliegen, in der der jeweilige Client Live-TV bekommt. M-5 gibt es ohne Gerät nicht; **Safari, iPhone und
iPad sind vorgesehen, aber ungeprüft ohne Gerät**, und keine andere Baufreigabe hängt daran (`05` 10.1).

| Client | Empfehlung (T-11, `05` 4.3) | Alternative, falls Messungen widersprechen | Zu messen |
|---|---|---|---|
| **Chrome, Edge, Firefox, Chrome Android** | HLS mit TS-Segmenten über `hls.js`, schlanker Bau | HLS-fMP4 über einen FFmpeg je Sitzung | M-1, M-3 |
| **Safari macOS** | HLS mit TS über `hls.js` (wie die anderen Browser) · vorgesehen, ungeprüft ohne Gerät | natives HLS | M-5 (kein Gerät) |
| **iPhone** | HLS mit TS, nativ, für H.264; HEVC wird umgewandelt · vorgesehen, ungeprüft ohne Gerät | `hls.js` über `ManagedMediaSource` ab iOS 17.1 (belegt (Nachprüfung Nr. 15)); HLS-fMP4 für HEVC (`04` O-3) | M-1, M-5 (kein Gerät) |
| **iPad** | wie die Web-App entscheidet · vorgesehen, ungeprüft ohne Gerät | natives HLS | M-5 (kein Gerät) |
| **Android-Handy-App** | HLS mit TS; Zeitversatz über das Live-Fenster | progressives TS nur als Schnellstart wäre ein zweiter Weg ohne Zeitversatz: nicht verfolgen, ausser M-1 zeigt einen grossen Abstand | M-1, M-4, M-7 |
| **Android-TV-App** | HLS mit TS; Fernseher können oft MPEG-2, MP2 und Zeilensprung selbst (**Annahme**) | wie Handy | M-1, M-4, M-6, M-7 |
| **Fire TV** (berücksichtigt, keine Zusage) | dieselbe TV-App, **falls** sie auf Fire OS läuft (Abschnitt 1; Annahme, kein Gerät) | – | M-7 |

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

### 3.3 Lange Segmente (T-7): was ein Sender mit langen Bildgruppen für den Client heisst

Im Direktweg schneidet Go an den Keyframes der Quelle; eine Bildgruppe wird nie geteilt, und eine Obergrenze
für die Segmentdauer gibt es nicht. Ohne Sitzungspuffer (Stufen 1 bis 3 und ohne Recht «Timeshift») kommen die
Segmente aus dem Ring; er muss drei vollständige Segmente und das laufende fassen. Der Ring hält 15 Sekunden,
bei langen Segmenten das Vierfache der Segmentdauer, nie mehr als 30 Sekunden und nie mehr als 96 MiB; die
Länge ist keine Einstellung (`04` T-7, `05` 3.6). Daraus folgt für jeden Client dasselbe:

| Segmentdauer des Senders | Wiedergabe ohne Sitzungspuffer | wenn die Umwandlung nicht verfügbar ist |
|---|---|---|
| bis 7,5 s | Direktweg aus dem Ring, drei Segmente plus das laufende (Rechnung, Annahme; M-3, M-4) | gleich (kein FFmpeg nötig) |
| über 7,5 bis 10 s | Direktweg mit **zwei** sicheren Segmenten; spielbar nach Rechnung, aber ohne Reserve und **nicht zugesichert**: Ein Player, der zwei Segmente vor dem Listenende beginnt (`hls.js` mit `liveSyncDurationCount` 2), steht am Rand. Die Verwaltung kennzeichnet den Sender «lange Bildgruppen». Zu messen (M-3, M-4). | gleich |
| **über 10 s** | **Kein Direktweg aus dem Ring, für keinen Client.** Die Sitzung bekommt eine **Bild-Umwandlung** mit Segmenten im Raster von 2 Sekunden. Mit Sitzungspuffer (Recht «Timeshift», ab Stufe 4) läuft der Sender im Original, weil die Segmente dann vom Datenträger kommen. | «auf diesem Server nicht abspielbar» |

Die **Zieldauer** der Liste ist die grösste beobachtete Segmentdauer des Senders, aufgerundet, mindestens 2 s;
sie steht fest für die ganze Ausgabe-Generation, ein längeres Segment beginnt eine neue (`05` 4.4). Für native
Apple-Player rechnet die Startregel (drei Zieldauern) mit diesem Wert (M-5, ungeprüft ohne Gerät). Ein Segment
von 10 s bei 8 Mbit/s sind 10 MB; bei 10 Mbit/s im WLAN braucht der Client 8 s dafür und holt es noch vor dem
Überschreiben (nach 20 s). Wie oft ein Sender mit langen Segmenten vorkommt, zeigt erst M-2 (Verteilung der
Bildgruppen-Längen); `13` nennt für Internet-Quellen 1 bis 10 s (Annahme).

## 4. Matrix C · Videocodec × Client

Ergebnis: **direkt** / **Bild umwandeln**. Grundlage: `04` 7.6 und 7.2 (Fassung 4). **Der Server entscheidet
aus den gemeldeten Gerätefähigkeiten**, die für Live-TV eine **eigene Angabe** sind: was der Player in
TS-Segmenten abspielt (T-14). Die Tabelle zeigt die Vorgabe, wenn ein Gerät nichts anderes meldet. Fehlt eine
Angabe, gilt «H.264 ohne Zeilensprung, AAC».

Spalte «Safari macOS, iPad» heisst: spielt über `hls.js` im schlanken Bau, also dieselben Grenzen wie die
Browser; wählt das iPad das native HLS, gilt die Spalte iPhone. **Alle Safari-, iPhone- und iPad-Zellen sind
vorgesehen, ungeprüft ohne Gerät.**

| Videocodec | Browser: Chrome, Edge, Firefox, Chrome Android (`hls.js`, schlank) | Safari macOS, iPad (`hls.js`, schlank) | iPhone (nativ) | Handy-App | TV-App |
|---|---|---|---|---|---|
| **H.264 progressiv** (8 Bit) | **direkt** · belegt (Nachprüfung Nr. 13: schlanker Bau liest H.264 in TS); Gerät zu messen (M-3) | **direkt** · wie Browser; vorgesehen, ungeprüft ohne Gerät | **direkt**; H.264 in TS erlaubt, belegt (Nachprüfung Nr. 8); Verhalten vorgesehen, ungeprüft ohne Gerät | **direkt** · Annahme (M-4) | **direkt** · Annahme (M-4) |
| **H.264 interlaced** (576i, 1080i) | **Bild umwandeln** (T-15: ein Browser meldet «stellt Zeilensprung sauber dar» nie) · zu messen (M-6) | **Bild umwandeln**, bis M-6 · vorgesehen, ungeprüft ohne Gerät | **Bild umwandeln**, bis M-6; Apple entfernt den Zeilensprung nach Annahme selbst · vorgesehen, ungeprüft ohne Gerät | **Bild umwandeln**, ausser das Gerät meldet saubere Darstellung (Handys eher nicht) · Annahme, zu messen (M-6) | **direkt**, wenn das Gerät meldet (Fernseher meist) · Annahme, zu messen (M-6) |
| **HEVC 8 und 10 Bit** | **Bild umwandeln**: der schlanke Bau liest HEVC in TS nicht (belegt (Nachprüfung Nr. 13)); Original nur mit vollem Bau, den Live-TV nicht lädt (O-22). Browser-Decoder ohnehin geräteabhängig (belegt (Nachprüfung Nr. 21)). | **Bild umwandeln** · belegt (Nachprüfung Nr. 13); vorgesehen, ungeprüft ohne Gerät | **Bild umwandeln**: HEVC in TS ist nach Apples Vorgaben nicht erlaubt, nur in fMP4 (belegt (Nachprüfung Nr. 8)); zweites Segmentformat ist O-3 · vorgesehen, ungeprüft ohne Gerät | **direkt** bei Hardware-Decoder · Annahme, zu messen (M-7) | **direkt** bei Hardware-Decoder · Annahme, zu messen (M-7) |
| **HDR** (HDR10, HLG, Dolby Vision; HEVC oder AV1) | **Bild umwandeln** (mit bestehendem Tonemapping auf SDR, `04` 7.2); Folge der HEVC-Zeile | **Bild umwandeln** · vorgesehen, ungeprüft ohne Gerät | **Bild umwandeln** · vorgesehen, ungeprüft ohne Gerät | **direkt** nur bei Hardware-Decoder **und** HDR-Display, sonst **Bild umwandeln** · Annahme (M-7) | wie Handy; Fernseher mit HDR-Display eher direkt · Annahme (M-7) |
| **MPEG-2** | **Bild umwandeln**: Chrome, Edge, Firefox lesen es nicht (belegt (Nachprüfung Nr. 22)); `hls.js` demuxt es nie (belegt, Nr. 13) | **Bild umwandeln**; MDN führt Safari als unterstützend, Apples Codecliste nennt MPEG-2 nicht: nur messen, nicht zusichern (belegt (Nachprüfung Nr. 22)); vorgesehen, ungeprüft ohne Gerät | **Bild umwandeln**; MPEG-2 steht nicht in Apples Codecliste (belegt (Nachprüfung Nr. 8)) · vorgesehen, ungeprüft ohne Gerät | **direkt**, wenn das Gerät MPEG-2 dekodiert (der TS-Leser von Media3 liest H.262, belegt (Nachprüfung Nr. 19); viele Handys ohne MPEG-2-Hardware, Annahme), sonst **Bild umwandeln** · zu messen (M-7) | **direkt**, wenn das Gerät MPEG-2 dekodiert (viele Fernseher können es, Annahme) · zu messen (M-7) |
| **AV1** | nicht eigens entschieden in `04`/`05`. Vorgabe **Bild umwandeln**, bis die Fähigkeitsprobe des Clients (`isTypeSupported`, im Repo vorhanden) es für TS-Segmente bestätigt · Annahme | wie Browser · Annahme; vorgesehen, ungeprüft ohne Gerät | wie Browser; nur neuere Chips · Annahme; vorgesehen, ungeprüft ohne Gerät | Hardware-Decoder abhängig; Media3 nutzt nur den Gerätedecoder (belegt, `supported-formats`) · zu messen (M-7) | wie Handy · zu messen (M-7) |

**Wenn die Umwandlung nicht verfügbar ist.** Live-FFmpeg läuft **nur mit wirksamer Sperre** (Landlock und
Filter für Systemaufrufe, T-27, `12`); das ist die sichere Vorgabe bis zu Marcos Antwort (`12` F-1, `20`
E-28-04). Fehlt die Sperre auf dem Host oder reichen die Einheiten nicht, bleibt der Direktweg, und jede Zelle
mit **Bild umwandeln** oder **Ton umwandeln** heisst für diesen Client: «auf diesem Server nicht abspielbar».
Die Verwaltung zeigt «Eindämmung nicht verfügbar» mit dieser Folge. Zellen mit **direkt** sind unberührt.
«Trotzdem erlauben» wäre nur die ausdrückliche Wahl des Betreibers (Variante der Frage); sie gilt nie ohne
setzbaren Filter für Systemaufrufe.

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

Grundlage: `04` 7.3 und 7.6 (Fassung 4). Regel aus ADR 0041, angewandt auf TS: AAC wird kopiert; AC-3,
E-AC-3, MP2 und AAC-LATM werden kopiert, wenn das Gerät sie **für TS-Segmente** meldet, sonst zu AAC neu
kodiert. Ein Sender ohne Tonspur ist ein gültiger Fall (`05` 9.3).

Gleiche Spaltenbedeutung wie in Matrix C. **Alle Safari-, iPhone- und iPad-Zellen sind vorgesehen, ungeprüft ohne Gerät.**

| Tonformat | Browser (`hls.js`, schlank) | Safari macOS, iPad (`hls.js`, schlank) | iPhone (nativ) | Handy-App | TV-App |
|---|---|---|---|---|---|
| **AAC-LC** | **direkt** · belegt (Nachprüfung Nr. 13: AAC-ADTS in TS); Gerät zu messen (M-3) | **direkt** · wie Browser; vorgesehen, ungeprüft ohne Gerät | **direkt** · Annahme; vorgesehen, ungeprüft ohne Gerät | **direkt** · Annahme (M-4) | **direkt** · Annahme (M-4) |
| **HE-AAC** (SBR) | **direkt** · Annahme, zu messen (M-3, M-7: HE-AAC ist dort aufgenommen) | **direkt** · Annahme; vorgesehen, ungeprüft ohne Gerät | **direkt** · Annahme; vorgesehen, ungeprüft ohne Gerät | **direkt** · Annahme (M-7) | **direkt** · Annahme (M-7) |
| **AAC-LATM** (DVB) | **Ton umwandeln**: `hls.js` liest LATM in TS in keinem Bau (belegt (Nachprüfung Nr. 13)) | **Ton umwandeln** · belegt (Nr. 13); vorgesehen, ungeprüft ohne Gerät | **Ton umwandeln** · Annahme; vorgesehen, ungeprüft ohne Gerät | **direkt**, wenn das Gerät den Ton dekodiert; der TS-Leser von Media3 liest LATM (belegt (Nachprüfung Nr. 19)) · Gerät zu messen (M-7) | wie Handy |
| **MP2** (MPEG-1 Layer II) | **Ton umwandeln**; direkt nur, wenn der Browser MP3-Ton meldet und `hls.js` ihn durchreicht (belegt (Nachprüfung Nr. 13): MPEG-Audio geht als `mp3` an die Medienschnittstelle, nur wenn der Browser es meldet) · zu messen (M-3) | wie Browser · vorgesehen, ungeprüft ohne Gerät | **Ton umwandeln**: Apples Vorgaben nennen MP2 nicht (belegt (Nachprüfung Nr. 8)) · vorgesehen, ungeprüft ohne Gerät | **direkt**, wenn das Gerät MP2 dekodiert (Media3 liest MPEG-Audio, belegt (Nachprüfung Nr. 19)), sonst **Ton umwandeln** · zu messen (M-7) | wie Handy |
| **AC-3** | **Ton umwandeln**: der schlanke Bau liest AC-3 in TS nicht (belegt (Nachprüfung Nr. 13)) | **Ton umwandeln** · belegt (Nr. 13); vorgesehen, ungeprüft ohne Gerät | **direkt**: Apples Vorgaben erlauben AC-3 und E-AC-3 (belegt (Nachprüfung Nr. 8)); das Verhalten in TS ist unbekannt (M-5) · vorgesehen, ungeprüft ohne Gerät | **direkt** nur mit AC-3-Decoder oder Durchreichen, sonst **Ton umwandeln** · zu messen (M-7) | **direkt** mit passender Tonanlage oder Decoder, sonst **Ton umwandeln** · zu messen (M-7) |
| **E-AC-3** | **Ton umwandeln**: E-AC-3 liest `hls.js` in TS auch im vollen Bau nicht (belegt (Nachprüfung Nr. 13)) | **Ton umwandeln** · belegt (Nr. 13); vorgesehen, ungeprüft ohne Gerät | wie AC-3 · vorgesehen, ungeprüft ohne Gerät | wie AC-3 | wie AC-3 |
| **Mehrere Tonspuren** | Der **Server** wählt die Spur: Er lässt die anderen Ton-PIDs samt angepasster Tabelle weg (T-12). Der Wechsel erzeugt eine neue Ausgabe-Generation (`04` 7.5). Alternative Tonspuren in der Liste braucht Live-TV nicht; der schlanke Bau lässt sie ohnehin weg (belegt (Nachprüfung Nr. 11)). | wie Browser | wie Browser; natives HLS könnte `EXT-X-MEDIA` selbst anbieten (Annahme, nicht vorgesehen) | **Server**, ausser das Gerät wählt Spuren selbst (`selectsTracks`, ADR 0041) | wie Handy |

**Ton ist die häufigste Fehlerquelle im Browser:** MP2 und LATM sind in DVB-nahen Quellen üblich (Annahme,
R2). Dass `hls.js` sie nicht liest, ist für LATM, AC-3 und E-AC-3 belegt; ob **MSE-Browser** sie selbst
dekodieren würden, hat die Nachprüfung nicht klären können (Nr. 23: MDN deckt es nicht ab): **Annahme**,
aber für die Vorgabe ohne Folge, weil `hls.js` die Spur gar nicht erst übergibt. Darum ist «nur Ton
umwandeln» der billige Standardweg. Mehrkanal-AC-3 zu Mehrkanal-AAC wäre eine zweite Stufe (R2,
**Empfehlung**, nicht entschieden).

## 6. Matrix E · Untertitel × Client

Grundlage: `04` 7.4 (Fassung 4; eine Aussage je Client und Stufe, `16`, `17` und `20` nennen dieselbe),
`05` M-24, Nachprüfung Nr. 11, 12, 17, 19. **Einbrennen wird nach der Vorgabe nicht gebaut**; es ist nur eine
Variante der Frage F-15.

| Client | Stufe 1a bis vor Stufe E | ab Stufe E |
|---|---|---|
| **Handy- und TV-App (Media3)** | **Auswahl der Untertitelspur**, soweit Media3 das Format in HLS mit TS-Segmenten liest. **Belegt** für Untertitel im Bildstrom (CEA-608/708; Media3 `supported-formats`, Nachprüfung Nr. 17). Für **DVB-Untertitel** (Bilder) hat der TS-Leser von Media3 einen Leser (belegt (Nachprüfung Nr. 19)); dass die App sie zeichnet, ist **Annahme** und wird in Stufe 1a gemessen (M-24). Der Server reicht die PID durch, das Gerät zeichnet. WebVTT in HLS liest Media3 (belegt (Nachprüfung Nr. 17)). | dazu Teletext-Untertitel, die der Server nach WebVTT umsetzt (O-5) |
| **Browser** (`hls.js`, schlanker Bau) | **keine wählbaren Untertitel.** Der schlanke Bau lässt Untertitel-Spuren weg (belegt (Nachprüfung Nr. 11)). Ob er Untertitel im Bildstrom (CEA-608/708) trotzdem zeigt, ist **nicht belegt** (weder README noch Bauschalter nennen sie unter den Auslassungen; Nachprüfung Nr. 12) und wird am gebauten Bau gemessen (M-3). | Teletext-Untertitel als WebVTT. DVB-Untertitel sind Bilder und lassen sich nicht in Text umsetzen; im Browser gäbe es sie nur eingebrannt (Variante von F-15). Wie der Browser WebVTT anzeigt (eigene Anzeige im Client oder voller Bau als eigener Teil), ist offen (Abschnitt 10, Punkt 16). |
| **Safari, iPhone, iPad** (vorgesehen, ungeprüft ohne Gerät) | wie Browser; Untertitel im Bildstrom zeigt das System nach Annahme selbst | wie Browser |
| **Teletext-Untertitel in jedem Client** | **in keinem Client:** Media3 hat keinen Teletext-Leser (belegt (Nachprüfung Nr. 19)); Browser und Safari nach Annahme auch nicht | in allen Clients über WebVTT (`libzvbi` ist im Image, belegt `M1`) |
| **Aufnahmen** | behalten immer alle Spuren (T-16, `05` 2.7: alle PIDs bleiben erhalten); beim Abspielen gilt dieselbe Tabelle | gleich |

**Was das für LTV-004 heisst.** LTV-004 verlangt «individuelle Untertitelauswahl» ohne Stufe. In den Stufen
vor E ist das **im Browser nicht erfüllt**, und für Sender, die Untertitel nur als Teletext führen (im
deutschsprachigen Rundfunk verbreitet; Annahme, M-2), **auf keinem Client**. Die früheren Fassungen nannten
das «erfüllt, aber teuer», weil sie vom Einbrennen ausgingen; das war falsch. Die Lücke ist eine Abweichung in
der Zeit, nicht im Umfang, und sie steht als Frage F-15 in `04` (→ `20`) mit wortlauttreuer Vorgabe:
Untertitel gehören zum Umfang; Varianten sind Stufe E vorziehen, voller Bau von `hls.js`, Einbrennen.
**Einbrennen** wäre eine Bild-Umwandlung mit der bestehenden Überlagerung (`pipeline.go:327`): 2 Einheiten je
Zuschauer mit Untertiteln, auf der Referenz ab Werk also höchstens zwei solche Zuschauer gleichzeitig.

## 7. Matrix F · Funktionen × Client

Spalten wie in Matrix C. **Alle Safari-, iPhone- und iPad-Zellen sind vorgesehen, ungeprüft ohne Gerät.**

| Funktion | Browser (`hls.js`) | Safari macOS, iPad (`hls.js`) | iPhone (nativ) | Handy-App (Media3) | TV-App (Media3) |
|---|---|---|---|---|---|
| **Umschalten (Zapping), Player-Wiederverwendung** | Eine `Hls`-Instanz und ein Bildelement über alle Umschaltungen; `loadSource` mit der neuen Liste. Ob das schneller ist als ein Neuaufbau, steht in der Dokumentation nicht (belegt (Nachprüfung Nr. 14)): **zu messen (M-3)**. Einstellungen: `startPosition` vom Server, `liveSyncDurationCount: 2`, **`lowLatencyMode: false`** (ab Werk an, belegt (Nachprüfung Nr. 9)), `startFragPrefetch: true`, `maxBufferLength: 12`. | wie Browser | neue Adresse am Bildelement; die Plattform baut ihre Kette neu auf; der erste Start braucht wohl eine Nutzergeste (Annahme) · zu messen (M-5, kein Gerät) | `setMediaItem` und `prepare` auf derselben `ExoPlayer`-Instanz, Oberfläche bleibt; ob der Decoder weiterläuft, hängt am Gerät · **zu messen (M-4)**. Ladepuffer: Vorgabe sind die **Standardwerte von 1.11.1: Start 1000 ms, nach Stocken 2000 ms, Vorlauf 50 s** (belegt (Nachprüfung Nr. 16)), also kein eigener `LoadControl`; 500 ms nur, wenn M-4 es verlangt. | wie Handy; **kein** zweiter Player für Nachbarsender (Decoder sind knapp, `05` 5.3). Vorgeladen wird nur im Server. |
| **Timeshift: Fenster** (Stufe 4, nur mit Recht) | Mit Recht: von der Rückspulgrenze bis zum neuesten Segment aus dem Sitzungspuffer; Spulen im `seekable`-Bereich (R2; Annahme). **Ohne Sitzungspuffer** (ohne Recht und in den Stufen 1 bis 3): die **drei neuesten vollständigen Segmente** aus dem Ring (15 s, bei langen Segmenten bis 30 s, T-7); ältere Segmente: 404. | wie Browser | wie Browser; für native Apple-Player sind drei Segmente eine bewusste Abweichung von Apples Vorgabe «mindestens sechs» (Abschnitt 3.2; O-28) | `seekTo` im Fenster (belegt (Nachprüfung Nr. 18)); Zeitleiste in Wanduhrzeit über `Window.windowStartTimeMs` (belegt, Media3 `live-streaming`) | wie Handy; lange Listen (60 Minuten und mehr) auf schwachen Fernsehern **zu messen (M-19)** |
| **Timeshift: Verhalten am Fensterende** | **`hls.js` springt ab Werk nicht von selbst an die Live-Kante** (`liveMaxLatencyDurationCount` ist unbegrenzt, belegt (Nachprüfung Nr. 10)). Ohne Zeitversatz setzt Live-TV `liveMaxLatencyDurationCount: 4`, weil ältere Segmente weg sind. Mit Zeitversatz bleibt der Abstand unbegrenzt, und der **Client** setzt am Fensteranfang fort (`04` 7.5, `05` 5.3); das Verhalten von `hls.js` beim Herausfallen am Listenanfang ist **zu messen (M-3)**. | wie Browser | läuft aus dem Fenster, springt zur Kante (Annahme) · zu messen (M-5, kein Gerät) | Fehler `BEHIND_LIVE_WINDOW`; Vorgabe der Doku: `seekToDefaultPosition()` und `prepare()`; die App muss ihn abfangen (belegt (Nachprüfung Nr. 18)) | wie Handy |
| **Aufholen zur Live-Kante durch schnelleres Abspielen** | aus: `maxLiveSyncPlaybackRate: 1` (Vorgabe von `hls.js`, belegt (Nachprüfung Nr. 9)) | aus | nicht möglich (Annahme) | aus: kleinste und grösste Geschwindigkeit 1,0 (belegt (Nachprüfung Nr. 18)) | aus |
| **Aufnahme-Wiedergabe** (Stufe 5) | gleiche Bauweise wie Live; eine fertige Aufnahme bekommt eine **abgeschlossene** Liste, eine laufende eine wachsende (`05` 4.4) · spezifiziert | wie Browser | wie Browser | wie Browser | wie Browser |
| **Tonspurwahl** | Server (T-12); Wechsel = neue Ausgabe-Generation, Liste wird neu geladen | wie Browser | wie Browser | Server, ausser `selectsTracks` (ADR 0041) | wie Handy |
| **Untertitelwahl** | **keine wählbaren Untertitel bis Stufe E** (Matrix E); ob CEA-608/708 am schlanken Bau angezeigt werden, ist nicht belegt (M-3) | wie Browser | wie Browser; Untertitel im Bildstrom zeigt das System nach Annahme selbst | **Spurwahl über Media3**: CEA-608/708 belegt, DVB-Untertitel Annahme (M-24); Teletext: keine | wie Handy |
| **Qualitätswahl** | Server: gewählte Qualität unter der Quelle erzeugt eine **Bild-Umwandlung** und eine neue Ausgabe-Generation (`04` 7.1, 7.5). Bei HLS-Quellen mit mehreren Varianten gilt die höchste (O-14). | wie Browser | wie Browser | wie Browser | wie Browser |
| **Umwandlung über 24 Stunden** | Kein FFmpeg-Lauf geht über einen Überlauf der Quellzeit (26,5 Stunden) und keiner über 24 Stunden; danach neue Ausgabe-Generation, der Zuschauer sieht höchstens einen kurzen Sprung (`04` 7.5). Verhalten der Player im Direktweg am Überlauf **zu messen (M-22)**. | wie Browser | wie Browser · zu messen (M-22, kein Gerät) | wie Browser | wie Browser |
| **Sender mit Segmenten über 10 s** (T-7) | Ohne Sitzungspuffer: **Bild-Umwandlung** mit 2-s-Raster, weil der Ring drei so lange Segmente nicht fasst; mit Sitzungspuffer (Stufe 4) Original. Ohne Umwandlung: «auf diesem Server nicht abspielbar». Zieldauer = grösste beobachtete Segmentdauer, neue Ausgabe-Generation bei einem längeren Segment (`05` 3.6, 4.4; Abschnitt 3.3). | wie Browser | wie Browser; Startregel rechnet mit der Zieldauer (M-5) | wie Browser: auch Media3 bekommt kein Direktsegment über 10 s ohne Sitzungspuffer | wie Handy |
| **Token-Wechsel** | Bei einem aktiven Sender gibt der Herzschlag ab 11 Stunden ein neues Token aus (T-21, T-29). Ein neues Token heisst eine neue Listenadresse: Neuladen im Player und eine kurze, sichtbare Unterbrechung je 11 Stunden. Verhalten **zu messen (M-3)**. | wie Browser | wie Browser · M-5, kein Gerät | **zu messen (M-4)** | wie Handy |
| **Wenn die Umwandlung nicht verfügbar ist** (keine Sperre, kein Budget) | Alles, was Ton- oder Bild-Umwandlung braucht, ist «auf diesem Server nicht abspielbar»; Direktweg bleibt (Abschnitt 4). | wie Browser | wie Browser | wie Browser, soweit das Gerät den Strom nicht selbst dekodiert (M-7) | wie Handy |
| **Hintergrundwiedergabe, Bildschirm aus** | **Offen (O-25):** Der Herzschlag der Sitzung (alle 10 s, Ende nach 60 s ohne, T-21) kann in einem Tab im Hintergrund oder bei Bildübertragung ausfallen; bis M-3 es zeigt, gilt die Regel der 60 Sekunden ohne Ausnahme. | wie Browser | wie Browser; dazu Sperrbildschirm · zu messen (M-5, kein Gerät) | **Der Player hält sofort an, der Herzschlag läuft noch 30 Sekunden weiter:** Kommt der Zuschauer in dieser Zeit zurück (Anruf, Blick in eine andere App), spielt die Sitzung weiter, mit Recht «Timeshift» an der alten Stelle; danach beendet der Client die Sitzung, und wer zurückkommt, stellt neu ein (`04` 10.3). Kein Ton im Hintergrund, kein Bild-in-Bild (LTV-016). | wie Handy |
| **Fernbedienung, Tasten** | Tastatur und Zeigegerät; je neue Seite Tastaturtest (Regel 8) | wie Browser | Touch | Touch | D-Pad. **Gebaut:** nur Medientasten `MEDIA_PLAY_PAUSE`, `PLAY`, `PAUSE`, `STOP`, `NEXT`, `PREVIOUS`, `FAST_FORWARD`, `REWIND` (`TvRemoteKeys.kt:39-46`). **Fehlt:** `CHANNEL_UP/DOWN`, `GUIDE`, `INFO`, `LAST_CHANNEL`, `DVR` und Ziffern im Player; ↑/↓ und ←/→ sind im Player belegt (Bedienung zeigen, spulen). |

## 8. Was das für den Normalfall heisst

**Der Normalfall hängt an deinen Sendern, und die kennt noch niemand.** Wie viele Sender Zeilensprung, MP2,
AC-3 oder MPEG-2 haben, zeigt erst **M-2** auf deinen echten Quellen. Als Arbeitsannahme gilt das Profil
eines typischen IPTV-Senders: H.264 oder MPEG-2, oft mit Zeilensprung, Ton MP2 oder AC-3 (**Annahme**).
Dann fällt der Direktweg nach `04` 7.6 (Fassung 4) so aus; die letzte Spalte gilt, wenn die Umwandlung nicht verfügbar ist (Matrix C: kein Live-FFmpeg ohne Sperre):

| Typischer Sender | Browser, Safari macOS, iPad (`hls.js`) | iPhone (vorgesehen, ungeprüft ohne Gerät) | Handy-App | TV-App | ohne Umwandlung (keine Sperre, kein Budget) |
|---|---|---|---|---|---|
| **H.264 1080i50 (Zeilensprung), Ton MP2 oder AC-3** | Bild umwandeln (inkl. Entfernen des Zeilensprungs) | Bild umwandeln, bis M-6 | Bild umwandeln, ausser Gerät meldet saubere Darstellung; Ton direkt, wenn das Gerät ihn dekodiert | direkt, wenn Gerät Zeilensprung und Ton meldet; sonst Ton oder Bild umwandeln | Browser, iPhone: nicht abspielbar; Handy, TV nur, wenn das Gerät alles selbst kann |
| **MPEG-2 576i50, Ton MP2** | Bild umwandeln | Bild umwandeln | direkt nur, wenn das Gerät MPEG-2 dekodiert und Zeilensprung sauber zeigt, sonst Bild umwandeln | wie Handy; Fernseher können es oft (Annahme) | Browser, iPhone: nicht abspielbar; Handy, TV nur, wenn das Gerät MPEG-2, Ton und Zeilensprung selbst kann |
| **H.264 ohne Zeilensprung, Ton MP2** | nur Ton umwandeln; direkt nur, wenn der Browser MP3-Ton meldet (M-3) | nur Ton umwandeln | direkt, wenn das Gerät den Ton dekodiert, sonst nur Ton umwandeln | wie Handy | Browser, iPhone: nicht abspielbar (ausser MP3-Ton im Browser, M-3); Handy, TV: direkt, wenn das Gerät den Ton dekodiert |
| **H.264 ohne Zeilensprung, Ton AC-3 oder E-AC-3** | **nur Ton umwandeln** (belegt (Nachprüfung Nr. 13)) | direkt: Apples Vorgaben erlauben beide, Verhalten in TS unbekannt | direkt mit passender Tonanlage, sonst nur Ton umwandeln | wie Handy | Browser: nicht abspielbar; iPhone, Handy, TV: direkt (iPhone ungeprüft) |
| **H.264 ohne Zeilensprung, Ton AAC-LATM** | **nur Ton umwandeln** (belegt (Nachprüfung Nr. 13)) | nur Ton umwandeln (Annahme) | direkt, wenn das Gerät den Ton dekodiert | wie Handy | Browser, iPhone: nicht abspielbar; Handy, TV: direkt, wenn das Gerät den Ton dekodiert |
| **HEVC** | **Bild umwandeln** (belegt (Nachprüfung Nr. 13)) | **Bild umwandeln** (belegt (Nachprüfung Nr. 8)) | direkt bei Hardware-Decoder | direkt bei Hardware-Decoder | Browser, iPhone: nicht abspielbar; Handy, TV: direkt bei Hardware-Decoder |
| **Sender mit Segmenten über 10 s**, ohne Sitzungspuffer (T-7) | Bild umwandeln (2-s-Raster) | Bild umwandeln | Bild umwandeln | Bild umwandeln | nicht abspielbar (mit Sitzungspuffer ab Stufe 4: Original) |
| **H.264 ohne Zeilensprung, AAC** | **direkt** | **direkt** | **direkt** | **direkt** | direkt |

**Ehrlich je Client:**

- **Browser, Safari macOS, iPad:** Direkt gibt es nur für H.264 ohne Zeilensprung mit AAC. Alles andere kostet
  einen FFmpeg-Prozess: billig bei «nur Ton» (0,25), teuer bei «Bild» (2 für HD). AC-3, E-AC-3, LATM und
  HEVC gehen im Browser **immer** in die Umwandlung, weil der schlanke Bau sie nicht liest und Live-TV den
  vollen nicht lädt (`05` 4.3). Bei Sendern nach dem Profil oben ist der Direktweg im Browser **die
  Ausnahme, nicht der Normalfall** (Annahme, bis M-2).
- **iPhone:** wie Browser bei Zeilensprung und MPEG-2; offen, ob Apple den Zeilensprung selbst entfernt (M-6)
  und ob AC-3 in TS direkt geht (M-5). HEVC bleibt umgewandelt (O-3). Alles vorgesehen, ungeprüft ohne Gerät.
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

Messpunkte aus `05` 10.1 (Fassung 4), soweit sie diese Matrizen tragen. **Nichts ist gemessen.** Geräte nach
`05` 8.3: Chrome und Firefox (Desktop), Pixel (das Gerät, mit dem K27 geprüft wurde), ein Android-TV-Gerät
(Modell notieren). **Ein Apple-Gerät gibt es nicht:** M-5 kann nicht laufen; WebKit in Playwright wäre nur
eine Näherung (`18` V-3). Referenzserver: CT 190 (`05` 8.1).

**Kriterium für Umschaltzeiten** (`05` 8.5, Vorschlag; verbindlich erst in `17`): Median unter dem Zielwert
(Fall 1 und 2 unter 1 s, Fall 3 unter 3 s, Fall 4 unter 5 s) in den Netzen N-1 und N-2, 95. Perzentil höchstens
das Doppelte, Fehlerquote höchstens 2 %. 50 gewertete Umschaltungen je Reihe.

| Nr. | Was | Gerät | Kriterium | Muss vorliegen vor |
|---|---|---|---|---|
| **M-1** | Umschaltzeit je Fall, Client, Netz | alle Clients | siehe oben | Abnahme jeder Stufe mit Wiedergabe |
| **M-2** | Deine echten Quellen: Anteil H.264 ohne Zeilensprung mit AAC, Zeilensprung, MPEG-2, MP2, AC-3; **Verteilung der Bildgruppen-Längen** (über 3,75, 7,5 und 10 s) und bei HLS-Quellen der Segmentdauern; Art der Untertitel (DVB, Teletext, im Bildstrom) | kleines Messwerkzeug auf dem Server | Kennzahlen; entscheiden, wie oft der Direktweg greift | **Baufreigabe der Stufe 1** (T-11, T-12) |
| **M-3** | `hls.js` 1.7.3 **im schlanken Bau** mit TS: H.264 mit AAC und HE-AAC, MP2 (als MP3-Ton, je Browser); Diskontinuitäten; Startpunkt; Listen von drei Segmenten mit `liveSyncDurationCount` 2 und `liveMaxLatencyDurationCount` 4; Verhalten bei fehlendem Segment (404); `loadSource` gegen Neuaufbau; mit und ohne `startFragPrefetch`; **Untertitel im Bildstrom (CEA-608/708)**; **Wiedergabe aus dem Ring mit Segmenten von 2, 6 und 10 s** bei einem Client mit 10 Mbit/s (T-7); feste Zieldauer bei schwankenden Segmenten; Herzschlag im Tab im Hintergrund. Im vollen Bau zusätzlich HEVC und AC-3 in TS und die Grösse des Zusatzteils (nur für O-22). **Nicht zu messen, weil belegt:** E-AC-3, AAC-LATM, MPEG-2. | Chrome, Firefox (Testquellen `05` 8.2) | spielt ohne Stocken; Startzeit | **Baufreigabe der Stufe 1** |
| **M-4** | Media3 1.11.1 mit HLS-TS: Start mit Standard-Startpuffer (1000 ms) und mit 500 ms, je mit Stockungen in den ersten 10 s; Start mit und ohne übergeordnete Liste (`CODECS`; `setAllowChunklessPreparation` ist nicht geprüft, Nachprüfung Nr. 20: Annahme); Decoder-Wiederverwendung; Verhalten hinter dem Fenster; lange Listen; **Wiedergabe aus dem Ring mit Segmenten von 2, 6 und 10 s**; Liste ohne Segment; Neuladen bei neuem Token | Pixel, Android-TV-Gerät | Startzeit, Stallrate, Decoder-Reuse ja/nein | Baufreigabe der Stufe, in der die Android-Apps Live-TV bekommen (`03` F-6) |
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
| **M-24** | Untertitel: DVB-Untertitel in Media3 über HLS-TS (Teletext hat dort keinen Leser); Umsetzung von Teletext nach WebVTT | Pixel, Android-TV, Testquelle mit Untertiteln | wird gezeichnet | Stufe 1a (DVB-Untertitel in Media3); Stufe E (WebVTT) |
| **M-25** | Eindämmung: Welche Fassung von Landlock bietet der Kernel von CT 190, lässt das Profil von Docker die Aufrufe zu, greift der Filter für Systemaufrufe, bricht die Hülle bei fehlendem Teil ab? Bestimmt, **ob Ton- und Bild-Umwandlung überhaupt verfügbar sind** (Matrix C) | CT 190, Entwicklungsrechner | Hülle bricht ohne Sperre ab | Baufreigabe von Stufe 0 und 1a (T-27); Baufreigabe von Weg W |

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
| 3 | **Schlanker oder voller Bau von `hls.js`** (`24` N-7, `04` O-22) | R2: lässt eventuell Alt-Audio, Untertitel, CEA weg; nicht nachgelesen. | **Entschieden in 05 Fassung 3, bestätigt in Fassung 4:** kein Voll-Bau. Belegt: schlanker Bau liest in TS kein HEVC, kein AC-3, E-AC-3 und LATM keiner (Nachprüfung Nr. 11, 13). Der volle Bau bleibt Option (O-22; Variante von F-15). **Offen:** CEA-608/708 am schlanken Bau (M-3); die Aussage «vermutlich nicht» war ohne Beleg. | Matrix C, D, E, Abschnitt 8. |
| 4 | **Startpunkt in `hls.js`** (W-6) | R1: `liveSyncDurationCount` 1. R2: 2. | **Entschieden in 05 Fassung 3:** Startpunkt vom Server, Wert bleibt 2. | Matrix F «Umschalten». |
| 5 | **Standardwerte des Ladepuffers in Media3** (W-5) | R1: 1000/2000 ms. R2: 50'000/50'000/2500/5000 ms. | **Entschieden (Nachprüfung Nr. 16):** R2 widerlegt, 1.11.1 hat Start 1000 ms, nach Stocken 2000 ms, Vorlauf 50 s. Vorgabe: diese Standardwerte; 500 ms nur nach M-4. | Matrix F. |
| 6 | **Startregel und Apples Vorgaben auf Safari und iPhone** (W-11) | R2/R3: RFC 8216 nicht abgerufen. | RFC-Regel **belegt** (Nachprüfung Nr. 1), W-11 aufgelöst. Apple verlangt mindestens 6 Segmente; Konzept weicht **bewusst** ab (`05` 4.3). Safari, iPhone, iPad sind vorgesehen, ungeprüft ohne Gerät (E-28-12). **Offen:** O-28 (sechs Segmente auch ohne Recht, berührt LTV-007). | Abschnitt 3.2. |
| 7 | **Ton und Zeilensprung: R2-Vorgaben gegen `04`** | R2: Ton immer zu AAC-LC; Interlaced für Browser und Handy immer entfernen. | **Entschieden (T-14, T-15, `04` 7.3):** kopieren, wenn das Gerät für TS meldet; Zeilensprung entfernt der Server, ausser das Gerät meldet saubere Darstellung (Browser nie). | Matrix C und D folgen `04`. |
| 8 | **HEVC auf Apple: direkt (R2) oder umwandeln** | R2: Safari direkt, nur fMP4. | **Entschieden (belegt (Nachprüfung Nr. 8)):** in TS nicht erlaubt, also Bild umwandeln; zweites Segmentformat ist O-3. | Matrix C. |
| 9 | **iPad: nativ oder `hls.js`** | R2: «nativ oder `hls.js`»; Repo-Weiche wählt «mse», wo `MediaSource` existiert. | **Entschieden in 05 Fassung 3:** iPad folgt der Weiche der App; iPhone nativ. Beides vorgesehen, ungeprüft ohne Gerät. | Abschnitt 1, Matrizen. |
| 10 | **Zeitversatz: Pause im Player oder im Server** | R2: Pause serverseitig, EVENT-Liste. | Sitzungspuffer mit eigenem Fenster; Rückspulgrenze nach `04` F-7; ohne Recht kein Zurückspulen. Zu `EVENT`: Liste je Zuschauer EVENT-artig ist regelkonform, das gleitende Fenster darf kein `PLAYLIST-TYPE` tragen (belegt (Nachprüfung Nr. 3)). | Matrix F. |
| 11 | **DASH, HLS-fMP4 als Eingang; RTSP, RTMP: Stufe** | `03`: Stufe 3. | **Entschieden in `18` (D-1, D-2):** Stufe L und Stufe W; UDP, RTP, Multicast in Stufe H. | Matrix A. |
| 12 | **Wer empfängt: FFmpeg oder Go** (W-3) | R3: FFmpeg. R1: Go. | **Entschieden (T-3):** Go (Weg G, L); FFmpeg nur Weg W. | Matrix A. |
| 13 | **FFmpeg-Version** (W-4) | Berichte beziehen sich auf die neueste Fassung. | Vorbehalt je Aussage bleibt; einzelne Werte an n7.1.4 belegt (Nachprüfung Nr. 24 bis 28); `max_reload` ist 100, nicht 1000 (Nr. 27). | Matrix A. |
| 14 | **LL-HLS, WebRTC** | R2 bewertet beides. | **Entschieden:** nicht verfolgt (T-11); `lowLatencyMode` ist in `hls.js` ab Werk an und wird für Live-TV auf `false` gesetzt (belegt (Nachprüfung Nr. 9)). | Matrix B, F. |
| 15 | **Sprung zur Live-Kante in `hls.js`** | R2: «setzt an der Live-Kante fort (belegt)». | **Richtiggestellt (Nachprüfung Nr. 10):** ab Werk kein Sprung; Live-TV setzt `liveMaxLatencyDurationCount: 4` für Sitzungen ohne Zeitversatz; Verhalten am Listenanfang M-3. | Matrix F. |
| 16 | **Untertitel: Stufe E gegen «kein Voll-Bau»; LTV-004** (`04` F-15, → `20`) | `18` Stufe E: Teletext als WebVTT. LTV-004: «individuelle Untertitelauswahl» ohne Stufe. | **Offen, Frage an Marco mit wortlauttreuer Vorgabe:** Im Browser gibt es bis Stufe E keine wählbaren Untertitel, Teletext in keinem Client. Der schlanke Bau lässt Untertitel-Spuren weg (belegt (Nachprüfung Nr. 11)), Live-TV lädt den vollen Bau nicht; wie der Browser WebVTT dann anzeigt, ist nicht festgelegt (Annahme: eigene Anzeige im Client möglich). Varianten: Stufe E vorziehen, voller Bau, Einbrennen. Die frühere Aussage «erfüllt, aber teuer» ist gestrichen (`04` 7.4). | Matrix E. |
| 17 | **Safari und MPEG-2** | MDN: Safari unterstützt MPEG-2. Apples Codecliste nennt es nicht. | Nur messen, nicht zusichern (Nachprüfung Nr. 22); Vorgabe Bild umwandeln. Kein Apple-Gerät. | Matrix C. |
| 18 | **Hintergrund und «Bildschirm aus»** | in `04`/`05` Fassung 2 nicht geregelt. | **Entschieden für die Apps (`04` 10.3, Fassung 4):** Player hält sofort an, Herzschlag läuft 30 Sekunden weiter, danach endet die Sitzung (Fassung 3 nannte zwei Regeln, die sich ausschlossen). **Offen (O-25):** Browser-Tab im Hintergrund und Bildübertragung. | Matrix F. |
| 21 | **Segmente über 10 Sekunden** (T-7, Fassung 4) | Fassung 2 und 3 nannten «höchstens 10 Sekunden» und 15 Sekunden Ring für alle. | **Entschieden in 04/05 Fassung 4:** Ring 15 s, bei langen Segmenten das Vierfache, höchstens 30 s und 96 MiB, keine Einstellung. Über 10 s Segmentdauer kein Direktweg ohne Sitzungspuffer; Bild-Umwandlung im 2-s-Raster, ohne Umwandlung «nicht abspielbar». 7,5 bis 10 s: nicht zugesichert (M-3, M-4). | Abschnitt 3.3, Matrix F. |
| 22 | **Umwandlung ohne Sperre** (`12` F-1, E-28-04) | R1/R3: FFmpeg als Dauerprozess ohne besondere Eindämmung. | **Vorgabe bis zur Antwort (sichere Variante):** kein Live-FFmpeg ohne wirksame Sperre; dann nur Direktweg, Umwandlung «nicht abspielbar», Verwaltung zeigt «Eindämmung nicht verfügbar». | Matrix A, C, Abschnitt 8. |
| 23 | **Fire TV** (LTV-014, O-29) | R2: Fire OS ist ein Android-Fork (Annahme). | **Berücksichtigt, nicht zugesagt;** Auslegung des Koordinators, als Frage in `20` geführt; alles Annahme, kein Gerät. | Abschnitt 1, Abschnitt 3.2. |
| 19 | **`setAllowChunklessPreparation` und `CODECS`** | R2: beschleunigt den Start. | Nicht geprüft (Nachprüfung Nr. 20): Annahme, M-4. | Matrix F. |
| 20 | **Safari-LL-HLS, `getStartDate()`** | R2: Apple-Seiten nicht abgerufen. | Nicht geprüft (Nachprüfung, Abschnitt 5): Annahme; kein Gerät. | Matrix B. |

**Weitere offene Entscheidungen, die diese Matrix berühren:** F-1 (RTSP und RTMP über Go-Bibliotheken oder
FFmpeg), F-11 (Multicast: Netzmodus «host» oder eigenes Netz), F-13 (DASH und HLS-fMP4, falls Weg W nicht
dicht belegbar ist), O-1 (Form der additiven Fähigkeitsangaben der Geräte: Videocodec `mpeg2video`,
Toncodecs `mp2` und `aac_latm`, «stellt Zeilensprung sauber dar», Untertitelformate `dvb_subtitle` und
`dvb_teletext`, dazu die Angabe «in TS-Segmenten abspielbar»), O-3, O-14, O-15, O-16, O-19, O-22, O-23,
O-25, O-28, O-29 (Fire TV) und `04` F-15 (Untertitel, → `20`) (siehe `04` 12.2 und `05` 10.2). Dazu `12` F-1 (Umwandlung ohne Sperre) und `20` E-28-12 (Apple-Gerät). Frühere Fragen dieser Datei: keine eigenen Fragen an Marco
(`F-n`); alle Fragen stehen in `04`.

## 11. Änderungen gegenüber der vorigen Fassung

### Fassung 3 (29. November 2026, Schlussbereinigung nach Gesamtreview)

- **Untertitel (RG-05, RB-06):** Matrix E neu nach `04` 7.4 Fassung 4: «eingebrannt» und «erfüllt, aber
  teuer» gestrichen. Browser: bis Stufe E keine wählbaren Untertitel; Teletext-Untertitel in keinem Client
  bis Stufe E; Apps: Spurwahl über Media3, CEA-608/708 belegt, DVB-Untertitel Annahme (M-24). LTV-004 ist im
  Browser eine Abweichung in der Zeit, Frage F-15. Matrix F «Untertitelwahl» angeglichen.
- **Safari, iPhone, iPad (RG-08):** überall «vorgesehen, ungeprüft ohne Gerät» statt «nicht zugesagt»; Frage
  E-28-12 genannt. **Fire TV:** «berücksichtigt, keine Zusage», Annahmen aus `05` 4.3, O-29; Auslegung des
  Koordinators.
- **T-7 (neu in Fassung 4):** Abschnitt 3.3 und Zeile in Matrix F und Abschnitt 8: Segmente über 10 s nur über
  Umwandlung ohne Sitzungspuffer; Ring 15 bis 30 s; Zieldauer = grösste beobachtete Segmentdauer; M-2, M-3,
  M-4 erweitert.
- **Kein Live-FFmpeg ohne Sperre (RT-01):** Absatz in Matrix C, neue Spalte «ohne Umwandlung» in Abschnitt 8,
  Zeile in Matrix F, M-25 in Abschnitt 9, Weg W ohne Sperre aus.
- **Hintergrund 30 s, Token-Wechsel je 11 Stunden** (`04` 10.3, T-21, T-29) in Matrix F.
- Kopf auf `29. November 2026`, Fassung 3; Bezug auf `04`/`05` Fassung 4 und `12` Fassung 3.

### Fassung 2 (28. November 2026, Konsolidierung 1)

Fassung 1 (28. November 2026) war der erste Entwurf; Fassung 2 führte die Konsolidierung 1 nach.

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
  «vorgesehen, ungeprüft ohne Gerät». iPad folgt der Weiche der App (04/05 Fassung 3).
- **Hintergrund:** In den Apps beendet er die Sitzung (`04` 10.3); Browser offen (O-25). FFmpeg-Lauf höchstens
  24 Stunden (`04` 7.5); neue Zeile in Matrix F.
- **Stufen** nach `18-umsetzungsplan.md` (1a, 3, H, L, W, E); Abschnitt 9 an `05` Fassung 3 angeglichen
  (HE-AAC, AV1, schlanker Bau in M-3 und M-7); Lücken erledigt gestrichen.
- **Abschnitt 10:** erledigte Punkte als «entschieden in 05 Fassung 3» gekennzeichnet; neu: Stufe E gegen
  «kein Voll-Bau» (Punkt 16), Safari und MPEG-2, nicht geprüfte Quellen.
- Quelle der Rückmeldungen: `KONSOLIDIERUNG.md`, `RUECKMELDUNGEN-AN-ARCH.md` (Abschnitt «Aus 16»), `04` 15.
- Restebereinigung nach Konsolidierung 1: O-28 berührt LTV-007, nicht LTV-005 (`20` U-13).
