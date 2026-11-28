# Konzept 28 · Review 1 Architektur

Stand: 28. November 2026 · Reviewer: oh-my-claudecode:architect · geprüft: 04-architektur.md, 05-streaming-und-fast-zapping.md (Commit des Entwurfs edd301b5)

Unabhängiges Review, nur lesend. Massstab: Leitplanken, Auftrag (LTV-002, 003, 004, 007, 008, 010, 011, 019, 020;
Abschnitte 8, 11, 16, 21), `03-fachkonzept.md`, der Code auf dem Stand des Arbeitsbaums, die Rohberichte und
`24-recherche-und-quellen.md`. Zeilenangaben `04:n` und `05:n` beziehen sich auf die geprüften Dateien. Rechnungen
sind nachvollziehbar angegeben; alles, was nicht gemessen ist, ist hier ebenfalls **Annahme**.

## 1. Kurzfassung

Das Grundgerüst (ein Dienst im Prozess, ein Ring je Sender, benannte Haltegründe, persönlicher Puffer vor der
Umwandlung, Aufnahme am Verteiler, FFmpeg nur über Pipes) ist die einfachste tragfähige Lösung und deckt sich mit
dem Code (Radio-Relay, `transcode.Manager`, `netguard`). Es gibt aber einen kritischen Fehler im Lebenszyklus
(Karenz plus Verbindungslimit plus Sperre bei 401/403), neun hohe Befunde, davon drei zum Wortlaut des Auftrags
(LTV-007 «nicht vor dem Einschalten», Timeshift ohne Recht, Abwägung des Sitzungspuffers), und mehrere
Widersprüche zwischen 04 und 05 bei den Grenzen des Rings. So ist der Entwurf nicht freigabereif; mit den
Korrekturen unten trägt er.

| Schwere | Anzahl |
|---|---|
| kritisch | 1 |
| hoch | 9 |
| mittel | 10 |
| niedrig | 6 |

## 2. Befunde

### RA-01 · kritisch · Karenz, Verbindungslimit und Sperre bei 401/403 legen Sender still

**Stelle:** T-17 (04:60), 04 Abschnitt 5.3 «Freigabe» (04:350-353) und «Wiederverbindung» (04:331-333), T-18 (04:61), F-3.

**Problem:** Beim Umschalten verlässt die Sitzung Sender A; A läuft noch 30 Sekunden Karenz und hält seine
Verbindung zum Anbieter. Gleichzeitig öffnet B eine zweite Verbindung. Bei Zugängen mit einer einzigen erlaubten
Verbindung (bei Xtream-Anbietern verbreitet; Annahme, M-2) gibt es zwei Ausgänge:

1. Der Anbieter wirft A hinaus: harmlos.
2. Der Anbieter lehnt B ab, je nach Panel mit 401, 403 oder einem eigenen Code. Nach 04:331-333 gilt 401/403 als
   «Fehler, die Wiederholen nicht heilt»: Die Quelle bekommt «Zugang prüfen» und wird übersprungen, **bis der
   Administrator sie ändert**. Ein einziger Senderwechsel sperrt damit die Hauptquelle von B, im Fall eines
   Zugangs pro Liste womöglich alle Sender dieses Zugangs.

Die Karenz ist zudem genau die «weitere Verbindung zum Anbieter, ungefragt geöffnet», mit der T-18 das Abschalten
von Vorladen begründet (04:61). Der Entwurf ist hier in sich widersprüchlich: Vorladen ist ab Werk aus, die Karenz
ab Werk an.

**Beleg:** 04:350-351 («30 Sekunden nach einem Zuschauer»), 04:331-333, 04:61; LTV-010 (Auftrag 698: «sehr hohe
Priorität»); F-3 (04:760-777) benennt das Limit, aber nicht diese Wirkung.

**Korrektur:**
1. Umschalten innerhalb einer Live-Sitzung löst den Haltegrund «Zuschauer» **ohne Karenz**, wenn der verlassene
   Sender keinen anderen Haltegrund hat. Die Karenz von 30 Sekunden bleibt nur für das Ende einer Sitzung (App
   geschlossen, Neustart des Players), wo sie das Wiedereinsteigen beschleunigt.
2. Bevor ein neuer Empfang auf derselben Quelle (gleicher Zugang) verbindet, beendet die Stromverwaltung Ströme
   derselben Quelle, die nur noch Karenz, Vorladen oder Prüfung halten.
3. 401, 403 und 429 bei einem Verbindungsversuch, während eine andere Verbindung derselben Quelle offen ist oder
   in den letzten Sekunden war, gelten als «vermutlich Verbindungslimit»: haltegrundlose Ströme dieser Quelle
   freigeben, **einmal** sofort neu versuchen, keine Sperre. «Zugang prüfen» erst bei 401/403 ohne andere offene
   Verbindung desselben Zugangs. Das ist Fehlerbehandlung, keine Limitverwaltung; LTV-017 bleibt unberührt.
4. Ablauftest im Testplan: zehn Umschaltungen in zehn Sekunden auf einem Testserver, der nur eine Verbindung
   annimmt und die zweite mit 403 ablehnt; erwartet: jede Umschaltung gelingt, keine Quelle wird gesperrt.

### RA-02 · hoch · Karenz-Ströme fehlen in der Rangfolge bei Mangel

**Stelle:** 04 Abschnitt 8.2 (04:509-524), Obergrenze «Eingangsströme insgesamt 8» (04:412).

**Problem:** Ströme in Karenz und Ströme im Zustand «Verbindet» ohne Haltegrund sind in keiner Stufe der Rangfolge
genannt. Wer neun noch nicht empfangene Sender innert 30 Sekunden durchschaltet, belegt mit acht Karenz-Strömen die
Obergrenze; der neunte Sender bekommt «Der Server ist ausgelastet» (Stufe 6), obwohl niemand die acht schaut.
Das widerspricht Auftrag Abschnitt 21 Punkt 8 («keine Ressourcenlecks bei häufigen Senderwechseln») im Ergebnis,
auch ohne Leck im engen Sinn.

**Korrektur:** Neue Stufe 0 vor «Vorgeladene Sender»: Ströme ohne Haltegrund (Karenz, abgebrochenes Verbinden),
ältester zuerst, sofort beendbar. Die Zulassung zählt sie nicht als belegt. Zusammen mit RA-01 Punkt 1 entsteht
die Lage beim Durchschalten gar nicht erst.

### RA-03 · hoch · LTV-007 «nicht vor dem Einschalten» ist umdefiniert statt als Frage gestellt

**Stelle:** «Klarstellung zu LTV-007» (04:71-75), T-7 (04:50), 05:324-326, 05 Abschnitt 5.1 Zeile 5 (05:433).

**Problem:**
- Auftrag 621: «Ein Benutzer kann nicht auf Inhalte zurückspulen, die vor seinem Einschalten ausgestrahlt wurden.»
  `03-fachkonzept.md` LTV-007.11 schärft auf «nie». Der Entwurf erklärt den Anfang des ersten Segments zum
  «Einschaltpunkt». Damit ist alles im Startpuffer rückspulbar, obwohl es vor dem Tastendruck gesendet wurde. Das
  ist eine Auslegung einer verbindlichen Anforderung und gehört nach Leitplanken 3.1 und Auftrag Abschnitt 21 als
  Frage an Marco, nicht als «keine Abschwächung» in den Text.
- Die Menge ist falsch angegeben. 04:72 sagt «höchstens zwei Bildgruppen». T-7 hält aber «die letzten zwei
  vollständigen Bildgruppen und die laufende», und 05:433 sagt selbst «zwei bis drei Bildgruppen». Rechnung: Start
  am älteren der zwei vollständigen Keyframes heisst bis zu 2 Bildgruppen plus der angefangenen, also knapp 3.
  Bei der Testquelle Q-D (10 s) wären das 30 s; die harte Grenze von 15 s kappt auf 15 s vor dem Tastendruck.

**Korrektur:** Neue Frage an Marco mit drei Varianten:
- (a) Start an der neuesten Bildgruppe: höchstens eine Bildgruppe vor dem Tastendruck, dafür wartet der Player auf
  den Rest der ersten Bildgruppe (langsamer).
- (b) Start an der letzten **vollständigen** Bildgruppe: höchstens zwei Bildgruppen vor dem Tastendruck, sofortiges
  Bild.
- (c) Entwurf: bis zu drei Bildgruppen, gekappt bei 15 s.

Empfehlung: (b). In jeder Variante gilt die Rückspulgrenze am ersten Segment, davor nichts. 04:72 korrigieren.
Zusammenhang mit RA-10: Wer Safari einen längeren Anlauf in der Liste geben will, verschiebt die Grenze weiter vor
den Tastendruck. Diese Spannung gehört in dieselbe Frage.

### RA-04 · hoch · Die Grenzen des Rings widersprechen sich

**Stelle:** T-7 (04:50), 04:531/533, 05 Abschnitt 3.1 Zeile «Ring» (05:228), 05 Abschnitt 3.6 (05:296-298).

**Problem:**
1. «Zwei vollständige Bildgruppen und die laufende» passen in 15 Sekunden nur bis zu einer Bildgruppenlänge von
   rund 5 Sekunden. Ab 7,5 Sekunden passt keine zweite, ab 15 Sekunden keine einzige vollständige. Für Q-C (4 s) geht
   es knapp, für Q-D (10 s) nicht. Was dann gilt (Start an welchem Keyframe? Gilt der Strom als «läuft» nach 04:308?),
   steht nirgends.
2. 05:228 beschreibt den Ring als «einmal angelegt» und zugleich als «Startwert 8 MiB, wächst mit der gemessenen
   Bitrate bis höchstens 64 MiB». 04:533 sagt «vorab reserviert und wachsen nicht über ihr Budget». Wie viel
   reserviert wird, ist offen. Werden 64 MiB je Ring reserviert, reicht das Budget von 512 MB für genau acht Ringe,
   und 100 dauerhafte Sender bräuchten 6,4 GB. Werden 8 MiB reserviert, hält ein Ring bei 20 Mbit/s nur 3,4 s
   (8 MiB × 8 ÷ 20 Mbit/s), also weniger als zwei Bildgruppen von 2 s, und das Wachsen kann zur Laufzeit scheitern.
3. Die 64 MiB werden nie erreicht: 15 s bei 20 Mbit/s sind 37,5 MB. Zwei harte Konstanten für dieselbe Grenze sind
   eine zu viel.

**Korrektur:** Eine einzige harte Grenze: 15 s Inhalt. Reservierung beim Start nach dem Muster des Radios
(`server/internal/radio/buffer.go:67-70`): gespeicherte Bitrate × 15 s × 1,03, ohne gespeicherte Bitrate ein fester
Vorgabewert. Bei geänderten Parametern (neuer Zeitabschnitt) neu reservieren, ein Ring wächst nie an Ort. Regel für
lange Bildgruppen: Passt keine vollständige Bildgruppe in 15 s, beginnt der Abnehmer am neuesten Keyframe (Fall «kein
schneller Start», sichtbar in der Verwaltung). Sender mit Bildgruppen über 15 s gelten trotzdem als «läuft».

### RA-05 · hoch · Der Direktweg ist für Browser und iPhone die Ausnahme, die Begründung von T-12 überzeichnet

**Stelle:** T-12 (04:55), T-15 (04:58), 04 Abschnitt 7.1 bis 7.3, 05:449-451, 04 Abschnitt 8.1.

**Problem:** Der Direktweg setzt im Web und auf dem iPhone voraus: H.264 ohne Zeilensprung, AAC (ADTS), keine
DVB-Untertitel. Sobald der Sender
- Zeilensprung hat (576i und 1080i sind bei Sendern aus Rundfunkquellen verbreitet; Annahme, M-2), gilt nach T-15
  «der Server entfernt ihn, ausser das Gerät meldet» und ein Browser hat keinen Weg, das zu melden. Jeder
  Web-Zuschauer eines solchen Senders braucht also eine **Bild-Umwandlung**;
- MP2, AC-3 oder AAC-LATM als Ton hat, braucht jeder Web-Zuschauer eine Ton-Umwandlung (05:449-451 nennt das
  selbst «den Normalfall» für Browser).

Der Satz «20 Zuschauer desselben Senders kosten 20 kleine Schreiber statt 20 Prozesse» (04:55) gilt damit nur für
Media3-Geräte mit passenden Decodern und für progressives H.264 mit AAC.

Rechnung für die Referenz CT 190: Das QSV-Budget beträgt 8 (`server/internal/transcode/manager.go:104-108`), Live-TV
erhält ab Werk die Hälfte (04:504-505), also 4 Einheiten. Eine Bild-Umwandlung in HD kostet 2
(`server/internal/transcode/plan.go:120`, `costHD`), eine Ton-Umwandlung nach 04:430 0,5. Ergebnis: höchstens
**zwei** Web-Zuschauer gleichzeitig auf Sendern mit Zeilensprung, oder acht auf Sendern mit MP2-Ton. Der dritte
Web-Zuschauer eines 1080i-Senders bekommt «ausgelastet». Das steht nirgends in 04.

Nebenbei: Für reine Tonarbeit gibt es schon den Kostensatz `costAudioOnly = 0.25` (`plan.go:123`). 0,5 für «Bild
kopieren, Ton neu» ist eine Annahme, die M-8 klären muss.

**Korrektur:** Begründung von T-12 auf die Geräte und Quellen beschränken, für die sie gilt. M-2 (Anteil
progressiver H.264/AAC-Quellen bei Marco) als **Vorbedingung der Freigabe** von T-11 und T-12 führen, nicht nur
des Baus. In 04 Abschnitt 8.1 eine Zeile «Kapazität der Referenz»: Zahl gleichzeitiger Bild- und Ton-Umwandlungen für
Live-TV auf CT 190. In T-15 festhalten, dass Browser die Angabe nie liefern, die Vorgabe dort also immer greift.

### RA-06 · hoch · Der `CONNECT`-Tunnel im Weg W hebelt die Prüfungen des Weiterleiters aus

**Stelle:** T-4 (04:47), 05 Abschnitt 2.6 (05:170-178), O-18 (05:770).

**Problem:** Für absolute `https`-Adressen in Listen und Manifesten baut FFmpeg nach 05:171-172 einen Tunnel
(`CONNECT`) durch den Weiterleiter. TLS läuft dann durchgehend zwischen FFmpeg und dem Anbieter. Der Weiterleiter
sieht nur verschlüsselte Bytes. Damit fallen für jede solche Unterliste und jedes Segment aus:
- die DRM-Regel 2.5, die der Weiterleiter «bei jeder durchgereichten Liste» anwenden soll (05:178; Widerspruch im
  selben Abschnitt);
- die Grössengrenzen;
- das Setzen von Kopfzeilen (darum O-18);
- die Zertifikatsprüfung von Go. FFmpeg prüft Zertifikate in 7.1 nach unserem Wissen ab Werk nicht (`tls_verify`
  ist 0; **Annahme**, im Image zu prüfen).

Absolute `https`-Adressen für Unterlisten und Segmente sind bei CDN-Quellen der Regelfall (Annahme).

**Korrektur:** Kein Tunnel. Der Weiterleiter schreibt **jede** Adresse in Listen und Manifesten auf den Ersatznamen
um: bei HLS die URI-Zeilen und die `URI=`-Merkmale von `EXT-X-KEY`, `EXT-X-MEDIA`, `EXT-X-MAP` und
`EXT-X-I-FRAME-STREAM-INF`, bei DASH `BaseURL`, `Location` sowie `media` und `initialization` in
`SegmentTemplate`. Go spricht TLS mit eigener Zertifikatsprüfung. Liste der Protokolle für FFmpeg:
`http,tcp,crypto` (ohne `https`, `tls`, `httpproxy`). Folgen: O-18 entfällt, FFmpeg sieht auch in Manifesten und in
der Fehlerausgabe keine Anbieter-Adresse, und M-10 wird einfacher, weil es nur noch einen Weg gibt. Preis: ein
Umschreiber für Manifeste, den der Weiterleiter für die DRM-Prüfung ohnehin zum Lesen braucht.

### RA-07 · hoch · Aufnahmen und Sitzungspuffer vertragen nur 15 Sekunden Stillstand des Datenträgers

**Stelle:** T-8 (04:51), T-16 (04:59), 05 Abschnitt 3.5 (05:282-285), T-25.

**Problem:** Einzige Reserve eines Schreibers ist der Ring, und der ist mit 15 s hart begrenzt (T-7). Damit ist die
Konstante gegen ein Archiv zugleich die Toleranz gegen Stockungen beim Schreiben. LTV-019 verlangt HDD und
eingebundene Netzspeicher als Speicherorte. Anlaufen einer schlafenden Platte (5 bis 15 s) und ein Neuaufbau von SMB
(15 bis 60 s) liegen in dieser Grössenordnung (Annahme, 24 N-18). Folge: Lücke «Datenträger zu langsam» in der
Aufnahme, obwohl Empfang und Platz in Ordnung sind. Das trifft Schwerpunkt 6 des Auftrags («zuverlässige
DVR-Aufnahmen», Auftrag 2065) und LTV-008.

**Korrektur:** Jeder Aufnahme-Schreiber bekommt einen eigenen, begrenzten Rückstau im Arbeitsspeicher (Vorschlag
64 MiB, oder etwa 60 s bei der gespeicherten Bitrate, das Kleinere), gezählt gegen das Byte-Budget. Er ist nur für
diese Aufnahme lesbar; ein Archivrisiko entsteht nicht. Für Sitzungspuffer dasselbe in kleinerer Form (16 MiB).
M-14 um «SMB 30 s getrennt» und «Platte schläft» erweitern.

### RA-08 · hoch · Sitzungspuffer ohne Timeshift-Recht: Zeitversatz ohne Recht und Schreiblast ohne Nutzen

**Stelle:** T-10 (04:53, «festes Fenster von 60 Sekunden»), 05 Abschnitt 4.4 Zeile «Fenster» (05:386), 03 DR-22,
03:36 (Stufe 1 «Player ohne Timeshift»).

**Problem:**
1. Ein Fenster von 60 s in der Wiedergabeliste ist für Safari (nativ) und Media3 ein gewöhnliches Live-Fenster, in
   dem der Player pausieren und zurückspringen kann, auch wenn die App keinen Knopf zeigt. Nach DR-22 sieht ein
   Zuschauer ohne Recht «nur Live». Durchgesetzt wäre das nur in der Oberfläche; das widerspricht Regel 12 und
   Auftrag 10.6 («serverseitige Durchsetzung sämtlicher Freigaben»).
2. Jeder Zuschauer ohne Recht schreibt trotzdem 1 MB/s (8 Mbit/s) auf die Platte, und Stufe 1 bekäme einen
   Plattenpuffer für eine Funktion, die es in Stufe 1 nicht gibt.

**Korrektur:** Ohne wirksames Recht ist das Fenster so klein, wie der Player es zum Spielen braucht (Vorschlag:
drei Segmente oder `HOLD-BACK`). Ältere Segmente: 404. Der Puffer liegt dann im Arbeitsspeicher der Sitzung (Muster
des Radios ohne tmpfs, `server/internal/radio/buffer.go:15-18`, `memorySegments`) oder bleibt auf Platte, aber ohne
Rückgriff. Der Sitzungspuffer auf Platte kommt mit der Stufe, die Timeshift bringt.

### RA-09 · hoch · Die Kosten von T-10 sind nicht gegen eine wortlauttreue Alternative abgewogen

**Stelle:** T-10 (04:53), 04:676 (Hardlinks verworfen), Risiko «Sitzungspuffer» (04:696), O-6 (04:716).

**Problem:** Rechnung bei 8 Mbit/s: 1 MB/s je Zuschauer, also 3,6 GB/h.

| Lage | Schreiblast | Platz bei 2 h Höchstdauer |
|---|---|---|
| 3 Zuschauer, je 4 h am Tag | 43 GB/Tag | 21,6 GB |
| 20 Zuschauer desselben Senders | 20 MB/s, 72 GB/h | 144 GB |
| 50 Zuschauer | 50 MB/s, 180 GB/h | 360 GB |

Für eine Haushaltsgrösse (ein bis fünf Zuschauer) ist das tragbar: 43 GB am Tag verbraucht eine Verbraucher-SSD mit
600 TBW in rund 38 Jahren (Annahme zur Haltbarkeit). Für 20 Zuschauer desselben Senders ist es die
zwanzigfache Kopie desselben Inhalts. Das ist nicht «ressourcenschonend» (LTV-002, Auftrag 420), sondern der Preis
einer Auslegung.

O-6 bietet nur «Puffer erst ab der ersten Pause» an. Das bricht LTV-007 («Timeshift beginnt beim Einschalten»,
Zurückspulen bis dahin) und ist keine echte Alternative. Die einzige wortlauttreue Alternative steht nicht als
Frage da: **einmal je Sender geschriebene Stücke, persönlicher Puffer als Referenzliste je Sitzung**, ein Stück
gelöscht, wenn keine Sitzung es mehr nennt. Logisch hat jeder Zuschauer seinen eigenen Puffer ab dem Einschalten;
physisch wird einmal geschrieben. Der Entwurf nennt das «verboten» (04:53) und begründet mit dem Fehlerrisiko einer
Filterregel (04:675). Ob das unter «Gemeinsame Timeshift-Puffer» (Auftrag 1763) fällt, ist eine Auslegung; die
rechtliche Seite ist ebenfalls offen (24 N-19).

**Bewertung (ausdrücklich verlangt):** T-10 in der Form «Kopie je Sitzung» ist für Homelab-Grössen vertretbar und
der einfachere, robustere Code; für 20 Zuschauer desselben Senders ist er es nicht. Eine einfachere Lösung, die
LTV-007 wörtlich einhält und die Schreiblast senkt, gibt es nur in zwei Formen: RA-08 (kein Plattenpuffer ohne
Recht; senkt die Last um den Anteil der Zuschauer ohne Recht) und die Referenzliste (senkt die Last auf einmal je
Sender, verlangt aber Marcos Auslegung von Abschnitt 16). «Ab Pause» hält LTV-007 nicht ein.

**Korrektur:** Neue Frage an Marco jetzt, nicht erst nach M-14: (a) Kopie je Sitzung (Empfehlung für die erste
Timeshift-Stufe), (b) Referenzliste mit einmal geschriebenen Stücken, (c) Kopie je Sitzung, dazu eine Obergrenze
gleichzeitiger Timeshift-Sitzungen. Mit Zahlen wie oben. Risiko-Zeile 04:696 auf diese Rechnung stellen.

### RA-10 · hoch · Safari und iPhone: Eine Liste, die beim Einschalten beginnt, kollidiert mit der Startregel von HLS

**Stelle:** T-11 (04:54), 05 Abschnitt 4.3 (Zeile iPhone), 4.4 «Startpunkt», 5.2 Fall 1 und 2.

**Problem:** Nach RFC 8216 Abschnitt 6.3.3 soll ein Client bei einer Live-Liste kein Segment wählen, das weniger als
drei Zieldauern vor dem Ende beginnt. Auch `EXT-X-START` soll bei Live-Listen nicht in diesen Bereich zeigen
(**Annahme**, die RFC ist laut 24 W-11 nicht von allen Berichten abgerufen). Eine Sitzungsliste enthält beim Einschalten
nur den Inhalt des Rings, also ein bis zwei Segmente. Der native Player von Safari kann deshalb warten, bis genug
Segmente da sind: bei 2 s Zieldauer bis zu rund 6 s, bei 10 s bis zu 30 s. Fall 1 und 2 (unter 1 s) sind auf dem iPhone
damit nicht nur unbelegt, sondern vom Aufbau her gefährdet. Bei hls.js hängt es am Startpunkt (`liveSyncDurationCount`,
Widerspruch W-6, in 05 nicht aufgelöst).

**Korrektur:** Die Spannung in 05 Abschnitt 4.3 und 7 benennen. M-5 so fassen, dass genau dies gemessen wird: Zeit
bis zum Bild bei einer Liste mit einem, zwei und drei Segmenten. Varianten vorbereiten: kurze Segmente in der
Startphase (eine Bildgruppe je Segment, kleine Zieldauer, verwandt mit O-19); für die installierte Web-App auf dem
iPhone hls.js über ManagedMediaSource ab iOS 17.1 (R2, 24 N-3). Für hls.js `liveSyncDurationCount` und `startPosition`
festlegen statt nur auf `EXT-X-START` zu setzen.

### RA-11 · mittel · Herunterfahren: «innerhalb der 10 Sekunden» geht mit der heutigen Reihenfolge nicht

**Stelle:** 04 Abschnitt 5.3 «Herunterfahren» (04:363-366), 10.6.

**Problem:** `shutdown` wartet zuerst bis 10 s auf offene Anfragen (`server/cmd/slouchcat/main.go:68`, `:656-664`). Erst
danach laufen die `defer`s, die die Dienste stoppen (`main.go:565-575`; Reihenfolge V1 L-4). Ein Live-Dienst, der sich wie
das Radio dort einhängt, beginnt also erst nach bis zu 10 s mit dem Abschluss der Aufnahmeteile. Eine offene
Direktwiedergabe hält die Anfrage minutenlang offen, die 10 s sind also der Normalfall. Docker beendet nach 10 s
hart, weil `compose.yaml` keine `stop_grace_period` hat (nur `compose.updater.yaml:26`). Die längere Frist nur in einer
freiwilligen Zusatzdatei reicht nicht, solange die Reihenfolge bleibt.

**Korrektur:** `live` bekommt beim Signal einen eigenen Abschluss, **parallel** zu `srv.Shutdown`: neue Sitzungen
ablehnen, Aufnahmeteile schliessen und den Stand schreiben (Ziel unter 3 s), dann FFmpeg beenden. Er läuft vor
`closeTranscoder` und vor `db.Close`. Dazu `stop_grace_period: 30s` in der Standard-Compose-Datei, als Änderung der
Baustufe mit Aufnahmen ausgewiesen.

### RA-12 · mittel · Umschalten mit Umwandlung: Budget, sterbende Prozesse und fehlende Schnittstelle

**Stelle:** T-13 (04:56), T-19 (04:62), 04 Abschnitt 8.1, O-7.

**Problem:**
1. Jedes Einstellen mit Ton- oder Bild-Umwandlung beendet einen FFmpeg (SIGTERM, bis 2 s bis SIGKILL,
   `server/internal/transcode/session.go:26`, `:651`) und startet einen neuen unter der Zulassung mit bis zu 10 s Warten
   (`manager.go:27-28`). Ist das Budget bis zum Ende des alten Prozesses belegt, füllen schnelle Umschaltungen das
   Budget mit sterbenden Prozessen. Folge: Wartezeit oder «ausgelastet» mitten im Durchschalten, und das
   ausgerechnet im Fall 4.
2. `Manager.Start` verlangt einen Plan mit Segmenten (`manager.go:180-182`); Live-TV kann diesen Eingang nicht
   nutzen. «Wiederverwendet wird das Budget» (04:56) heisst also: Der Manager bekommt eine neue Schnittstelle. O-7
   nennt das, T-13 klingt nach Wiederverwendung ohne Änderung.

**Korrektur:** Die Reservierung gehört der Live-Sitzung, nicht dem Prozess. Sie wird beim Einstellen auf den neuen
Entscheid umgebucht (nur die Differenz wartet) und erst beim Ende der Sitzung freigegeben, so wie der Platz bei den
Stream-Grenzen (05:430). Der neue FFmpeg startet, ohne auf das Ende des alten zu warten. T-13 um «neue Schnittstelle
`Reserve/Adjust/Release` im Manager, Regeln der Dateiwiedergabe unverändert, belegt durch die bestehenden Tests des
Managers» ergänzen.

### RA-13 · mittel · Nebenläufigkeit im Ring und Speicherverwaltung von Go sind nicht festgelegt

**Stelle:** 05 Abschnitt 3.1 und 3.5, T-6, T-8.

**Problem:**
1. Ein Schreiber, der nie wartet, und Leser, die kopieren: Ohne Regel liest ein Leser Bytes, die während des
   Kopierens überschrieben werden. Das ergibt zerrissene Pakete im Sitzungspuffer oder in der Aufnahme, und das
   gerade bei langsamen Abnehmern, also dem Fall, den T-8 schützen soll.
2. Der Zugriff auf den GOP-Index durch neue Abnehmer, während der Schreiber Einträge verwirft, ist offen.
3. Ringe von zusammen 512 MB liegen als `[]byte` im Heap von Go. Weder Server noch Compose setzen `GOMEMLIMIT`
   (geprüft: keine Fundstelle in `server/cmd/slouchcat/main.go` und `deploy/`). Mit der Vorgabe `GOGC=100` darf der
   Heap bis etwa zum Doppelten des lebenden Speichers wachsen. Das ist auf 4 GB relevant (RA-14).
4. Ein Abnehmer, der während «Ende» beitritt: Das Radio lehnt einen endenden Relay mit Fehler ab
   (`server/internal/radio/listen.go:228-232`). Für Live-TV hiesse das: Fehler beim Zurückschalten in der letzten
   Sekunde der Karenz.

**Korrektur:**
1. Festlegen: Der Leser kopiert den Bereich und prüft danach die Schreibposition. Wurde der Bereich überschrieben,
   verwirft er die Kopie und meldet «überrundet» (Prüfung nach dem Kopieren, ohne Sperre im Schreibpfad). Den
   GOP-Index unter einer kurzen Lesesperre führen. Das Signal «neue Daten» als geschlossener und ersetzter Kanal,
   wie `freed` im Manager (`manager.go:95`).
2. Ringe einmal je Zeitabschnitt anlegen. `GOMEMLIMIT` aus der Speichergrenze des Containers ableiten, oder Ringe
   ausserhalb des Heaps (`mmap`).
3. Ein Beitritt zu einem endenden Strom wartet auf dessen Ende und startet neu.
4. Abnahmetest: 1000 Umschaltungen, danach gleich viele Goroutinen, offene Dateien und Prozesse wie vorher.

### RA-14 · mittel · Arbeitsspeicher gegen 4 GB der Referenz ist nicht gerechnet

**Stelle:** 05 Abschnitt 3.6 Rechenbeispiel (05:303-305), 04 Abschnitt 6.3 (Vorgaben), T-25 (`compose.live-ram.yaml`).

**Problem:** Das Rechenbeispiel nimmt Bildgruppen von 2 s an, also den günstigsten Fall. Rechnung mit dem Inhalt des
Rings (rund 2,5 Bildgruppen, höchstens 15 s):

| Sender (HD, 8 Mbit/s) | GOP 2 s (≈ 5 MB) | GOP 5 s (≈ 12,5 MB) | Grenze 15 s (15 MB) | Eingang |
|---|---|---|---|---|
| 5 | 25 MB | 63 MB | 75 MB | 40 Mbit/s |
| 20 | 100 MB | 250 MB | 300 MB | 160 Mbit/s |
| 50 | 250 MB | 625 MB | 750 MB | 400 Mbit/s |
| 100 | 500 MB | 1,25 GB | 1,5 GB | 800 Mbit/s |

UHD (20 Mbit/s) ergibt das 2,5-Fache. Auf CT 190 liegen daneben schon bis 564 MB tmpfs für das Radio
(`deploy/compose.yaml:37-39`, zählt zum Speicher des Containers). Hinzu kommen die FFmpeg-Umwandlungen (je einige
hundert MB, Annahme), der Heap-Zuschlag aus RA-13 und der Seitencache der Sitzungspuffer. Das Budget von 512 MB ab
Werk lässt bei 5-s-Bildgruppen rund 40 HD-Sender zu, nicht 100. Bei 100 Sendern begrenzt ohnehin die Bandbreite
(800 Mbit/s Eingang). `compose.live-ram.yaml` für Timeshift ist bei 3,6 GB je Stunde und Zuschauer nur für sehr kurze
Fenster sinnvoll.

**Korrektur:** Tabelle in `13-ressourcen-und-performance.md`; Vorgabe des Budgets aus dem Arbeitsspeicher ableiten
oder ausdrücklich begründen. Die RAM-Disk für Timeshift nur mit Rechnung (Fenster × Bitrate × Zuschauer) anbieten
oder streichen (Abschnitt 5).

### RA-15 · mittel · Zu viele neue Zeitabschnitte: PMT-Version und Diskontinuitäten von HLS-Quellen

**Stelle:** 04 Abschnitt 9 «Wechselnde Streamparameter» (04:559), 05 Abschnitt 3.3 (05:254-255), 3.4 (05:267-271), 2.7 Punkt 3.

**Problem:**
1. Die Versionsnummer der PMT ändert sich auch bei Änderungen an Beschreibern (Sprache, Komponenten), ohne dass
   sich Spuren oder Codecs ändern (Annahme, M-2).
2. HLS-Quellen mit Werbeeinblendung oder FAST-Sender setzen `EXT-X-DISCONTINUITY` oft mehrmals je Stunde. Jeder neue
   Zeitabschnitt bedeutet: neuer Aufnahmeteil, Neustart jedes laufenden FFmpeg («kein FFmpeg-Lauf geht über eine
   Abschnittsgrenze», 05:271), also 1 bis 2 s Unterbruch je Zuschauer mit Umwandlung, und eine Diskontinuität bei
   allen anderen.
3. Eine Aufnahme eines Spielfilms auf einem FAST-Sender hätte Dutzende Teile.

**Korrektur:** Neuer Zeitabschnitt nur bei geänderter Menge der PIDs, geändertem `stream_type` oder geänderten
Bildparametern (Grösse, Codec, Zeilensprung) und bei Quellwechsel. Eine Diskontinuität der Quelle bei gleichen
Parametern wird in der Liste markiert (`EXT-X-DISCONTINUITY`), die Aufnahme bleibt im selben Teil (TS verträgt
Zeitsprünge; die Lücke kommt ins Verzeichnis), FFmpeg läuft weiter. Ob FFmpeg einen Zeitsprung im TS-Eingang ohne
Neustart sauber übernimmt, gehört zu M-9.

### RA-16 · mittel · Die TS-Bibliothek ist unterschätzt; fünf Fälle fehlen

**Stelle:** 05 Abschnitt 2.7, 3.2, 3.3; T-12 (Tonwahl durch PID-Filter); 04 Abschnitt 3.2 `live/mpegts` («nur
Standardbibliothek»).

**Problem:**
1. **PCR auf einer Ton-PID:** Trägt die weggelassene Tonspur die Referenzuhr (`PCR_PID`), fehlt sie nach dem Filter.
   Die Pakete mit Anpassungsfeld müssen bleiben, oder die PMT muss eine andere PCR-PID nennen und die Uhr umziehen.
2. **Keyframe ohne Kennzeichen:** Vor dem IDR-Bild stehen AUD, SEI, SPS und PPS; das erste Bild-NAL liegt oft
   mehrere TS-Pakete hinter dem Anfang des PES, und Startcodes können über Paketgrenzen laufen. «Blick in den Anfang
   des Bildpakets» (05:241) heisst also: PES bis zum ersten Bild-NAL zusammensetzen.
3. **Offene Bildgruppen:** Ein HEVC-CRA mit RASL-Bildern, ein MPEG-2-I-Bild ohne `closed_gop` und ein H.264-Bild mit
   Wiedereinstiegspunkt ergeben am Start nicht dekodierbare Vorlaufbilder. IDR, BLA und geschlossene Bildgruppen
   vorziehen, die anderen kennzeichnen.
4. **Eingefügte PAT und PMT vor jedem Segment:** Die Zähler (Continuity Counter) von PID 0 und der PMT-PID müssen
   zum übrigen Strom passen, sonst zählt die eigene Überwachung Fehler.
5. **Fehlalarme:** Verschlüsselungsbits auf einer Daten-PID sind kein DRM des Senders (nur Bild- und Ton-PIDs werten,
   über eine Mindestdauer). Zählersprünge an Segmentgrenzen von HLS-Quellen sind kein Paketverlust.

Zudem wägt 04 die Eigenentwicklung nicht gegen `bluenviron/mediacommon` ab, das 24 (Zeile 238) als Kandidaten führt
und das mit F-1 (a) ohnehin als Abhängigkeit kommt.

**Korrektur:** Die fünf Punkte als Anforderungen an `live/mpegts` und als feindliche Testfälle (Fuzzing, Vorbild
`radio/hostile_test.go`). Die Entscheidung «eigene Bibliothek oder `mediacommon`» in 04 Abschnitt 3.2 begründen.

### RA-17 · mittel · Untertitel-Aussagen sind für zwei Clients zu optimistisch

**Stelle:** 04 Abschnitt 7.4 (04:463-467), 05:375-377.

**Problem:** «CEA-608/708 bleiben im Bild, der Player zeigt sie» trifft für `hls.js/light` vermutlich nicht zu: Der
schlanke Bau lässt Untertitel und Captions weg (Annahme, 24 N-7). Das Durchreichen von Teletext nützt keinem der drei
Clients; uns ist kein Player bekannt, der es zeichnet (Annahme, M-24). Im Web gibt es DVB-Untertitel damit nur
eingebrannt, also mit Bild-Umwandlung. LTV-004 «individuelle Untertitelauswahl» ist für Web und iPhone in Stufe 1
faktisch nur über die Umwandlung erfüllt. Das gehört offen in die Kompatibilitätsmatrix.

**Korrektur:** Matrix je Client in `16`. Entscheid «schlanker oder voller Bau von hls.js» als offenen Punkt führen.
Teletext-Zeile auf «kein Client, O-5» setzen.

### RA-18 · mittel · Vorladen und «automatisch» ab Werk aus sind als Entscheid festgelegt, nicht als Frage

**Stelle:** T-18 (04:61), 04 Abschnitt 6.1 bis 6.3, 05 Abschnitt 6.

**Problem:** LTV-010 hat «sehr hohe Priorität» und nennt «Vorladen benachbarter Sender» und «Vorladen häufig genutzter
Sender». LTV-002 nennt «Automatischer Empfang». Der Entwurf baut beides, macht es aber dreifach ausgeschaltet:
1. die Art «automatisch» je Sender;
2. N = 0 und «Vorladen je Sitzung 0»;
3. zwei Schalter je Quelle, ab Werk aus (04:382).

Das ist eine Empfehlung mit Folgen für den Kern des Auftrags. Sie steht als tragender Entscheid mit «Verworfen:
Vorladen ab Werk an» da, ohne Frage an Marco (F-3 betrifft das Limit, nicht die Vorgabe).

**Korrektur:** Neue Frage: (a) aus wie im Entwurf, (b) Vorladen an, sobald der Administrator bei einer Quelle «mehrere
gleichzeitige Verbindungen erlaubt» setzt. Dazu Vereinfachung: ein einziger Schalter je Quelle statt zwei, und
«automatisch» ohne eigene Art je Sender (Abschnitt 5).

### RA-19 · mittel · Die Ausnahme für lokale Quellen ist breiter als nötig

**Stelle:** T-5 (04:48), 05 Abschnitt 2.3.

**Problem:** Der bestehende Weg für Ausnahmen ist `netguard.Clients` mit einem ungeschützten Client
(`server/internal/netguard/netguard.go:93-111`, Auswahl über `For`, `:121-126`). Der ungeschützte Client darf
**jede** lokale Adresse anwählen. Eine Quelle «lokal erlaubt» mit HLS oder DASH (etwa ein Gerät im Heimnetz, oder eine
Liste, deren Inhalt der Administrator nicht kontrolliert) kann dann über Unterlisten, Segmente, Schlüssel und
Weiterleitungen beliebige Geräte im Heimnetz erreichen, etwa die Verwaltung des Routers. Der Administrator vertraut
dem Gerät, nicht jeder Adresse in dessen Listen. 04 und 05 sprechen von einer «neuen Prüffunktion mit Regel» und
erwähnen das bestehende `Clients.For` nicht (V1-Lücke).

**Korrektur:** Die Freigabe gilt für die Adresse oder das Präfix des Quellhosts, aufgelöst beim Freigeben und beim
Verbinden, und für öffentliche Adressen. Andere lokale Ziele werden auch bei freigegebener Quelle abgelehnt.
Umsetzung als `Control`-Funktion mit erlaubter Adressmenge, aufbauend auf `Clients`.

### RA-20 · mittel · Die geschätzten Bereiche der Umschaltkette sind in sich nicht stimmig

**Stelle:** 05 Abschnitt 5.1 und 5.2 (05:427-447).

**Problem:** Fall 1 ist mit «0,3 bis 1,0 s» angegeben. Die Summe der genannten Abschnitte 1, 2, 6, 8, 9, 10 im Heimnetz
nach der eigenen Tabelle ergibt:
- untere Grenze: 5 + 1 + 5 + 160 + 0 + 50 ms ≈ 0,22 s;
- obere Grenze bei WLAN mit 30 Mbit/s: 50 + 5 + 30 + 500 + 2500 + 300 ms ≈ 3,4 s.

Die obere Grenze von 1,0 s fällt genau auf den Zielwert und setzt stillschweigend einen Startpuffer des Players von
höchstens etwa 0,3 s voraus. Fall 3 «1 bis 2 s bei HLS-Quellen» übergeht das Laden eines Quellsegments: 6 s bei
8 Mbit/s sind 6 MB, bei 50 Mbit/s Anbindung an den Anbieter rund 1 s. Als «Annahme» gekennzeichnet ist alles, aber die
Summen wirken wie Zusagen. Eine ausdrückliche Behauptung, ein Zielwert werde erreicht, gibt es nicht (05:12, 05:505-516).

**Korrektur:** Bereiche als Summe der Zeilen ausweisen und die Annahmen nennen, unter denen die untere Hälfte gilt
(Startpuffer des Players unter 0,5 s, Segment unter 2 MB, kabelgebundenes Netz).

### RA-21 · niedrig · «Belegte» Schwächen von FFmpeg sind nicht belegt

**Stelle:** T-3 (04:46), 04 Abschnitt 11 (04:674).

**Problem:** «Belegte Schwächen (Speicherwachstum nach 24 h, 33-Bit-Überlauf)» stützt sich auf R1. 24 führt das als
N-9 «nachzuprüfen» (Ticket nicht gelesen) und W-4 (Dokumentation der neuesten Version, nicht 7.1.4). Nach den
Leitplanken (Abschnitt 5) ist das eine Annahme.

**Korrektur:** «Berichtete Schwächen (Annahme, M-12)». Die Entscheidung T-3 trägt auch ohne diese Begründung (Netzschutz
und Wiederverbindung in Go).

### RA-22 · niedrig · Der «schmale Haken» für die Stream-Grenzen ist breiter

**Stelle:** T-26 (04:69), 04 Abschnitt 3.2 Zeile `internal/playback`.

**Problem:** `admit` zählt laufende Wiedergaben per SQL über `playback_sessions` und addiert Reservierungen im Speicher
(`server/internal/playback/limits.go:180-200`). Live-Sitzungen stehen in Tabellen von `liveq`, die `playback` wegen des
depguard-Riegels nicht lesen darf (04:113). Nötig ist also ein Zähler-Rückruf von `playback` nach `live` unter der
Startsperre und umgekehrt ein Reservieren von `live` aus. Das ist eine Kopplung in beide Richtungen, lösbar über einen
Setter wie beim Radio (`main.go:471`). Das Radio zählt heute nicht zu `MaxStreams`; «wie das Radio» trifft nur auf die
Schauzeit zu.

**Korrektur:** Schnittstelle in 04 beschreiben (Rückruf «aktive Live-Sitzungen je Profil und Konto»). Test: eine
Dateiwiedergabe und eine Live-Sitzung kämpfen um den letzten Platz.

### RA-23 · niedrig · Die VOD-Nähte werden doch berührt

**Stelle:** T-13 («unverändert bleiben … `ffmpegArgs`», 04:56), T-15, 04 Abschnitt 8.4.

**Problem:** Der Baustein «Zeilensprung» kommt in die gemeinsame Videokette (`pipeline.go:398`), und `deprioritize`
(`session.go:702-712`) bekommt einen Parameter. Beides ist Code der Dateiwiedergabe. Klein, aber «unverändert» stimmt
so nicht.

**Korrektur:** «Unverändert im Verhalten, belegt durch die bestehenden Tests der Pipeline und der Sitzung (Vergleich der
Argumentlisten)».

### RA-24 · niedrig · Die 18 Abläufe aus Auftrag 8.2 fehlen und sind nicht verwiesen

**Stelle:** 04 und 05 insgesamt.

**Problem:** Auftrag Abschnitt 8.2 verlangt Diagramme für 18 Situationen. 04 enthält zwei Flussbilder, ein
Komponentenbild und einen Zustandsautomaten, aber keine Ablauffolgen, und keinen Verweis auf `06-ablaeufe.md`. Die
Fehlerfälle in 04 Abschnitt 9 decken die Situationen 9 bis 18 als Text ab.

**Korrektur:** In 04 Abschnitt 1 auf `06-ablaeufe.md` verweisen. Dort die 18 Abläufe aus T-1 bis T-26 ableiten.

### RA-25 · niedrig · «Eingefrorene Streams» nur teilweise erkannt, ohne Frage

**Stelle:** 05 Abschnitt 9.1 und 9.4 (05:655-656, 05:690-691).

**Problem:** LTV-011 verlangt die Erkennung eingefrorener Streams. Der Entwurf erkennt einen eingefrorenen Strom am
Stillstand der Zeitstempel, ein stehendes Bild aber nur über die optionale Stichprobe (M-20) und erklärt ehrlich,
dass er es bis dahin nicht sicher erkennt. Das ist eine Auslegung des Begriffs; sie ist offen benannt, aber nicht als
Frage gestellt.

**Korrektur:** Als Frage oder als Stufe festhalten: «Stichprobe mit Dekodieren in Stufe n; bis dahin wird nur der
Stillstand der Zeit erkannt».

### RA-26 · niedrig · Kleinere Unstimmigkeiten

- 04:568: Recht neu prüfen «höchstens alle 30 Sekunden» und «wirksam nach spätestens 10 Sekunden (Ziel)»: Die
  10 Sekunden gelten nur über den Haken in `auth`. Das so schreiben.
- 04:502: Ein FFmpeg-Empfänger des Wegs W kostet 0,5 Einheiten und ist «nicht verdrängbar». Ob er in den Anteil für
  Live-TV fällt (04:504), ist offen. Bei acht DASH-Sendern dauerhaft wäre der ganze Anteil weg.
- 04:249 Nutzung je Sender und Tag: Dieselbe Information steckt in den Sitzungszeilen (04:245). Eine zweite Tabelle
  braucht es nicht (Abschnitt 5).
- 05 Abschnitt 4.4 «Lange Listen»: Delta-Listen braucht es erst mit Timeshift.

## 3. Urteil je tragendem Entscheid

| Nr. | Urteil | Begründung, Befunde |
|---|---|---|
| T-1 | trägt | Muster Radio (`main.go:459-483`, `:569`); Jobsystem für Langläufer zu grob. |
| T-2 | trägt | Identität am Sender verhindert zweite Verbindung beim Quellwechsel. |
| T-3 | trägt mit Korrektur | Weg G deckt den Alltag; W und L in späte Stufen (Abschnitt 5); RA-06, RA-21. |
| T-4 | trägt mit Korrektur | Regel richtig, aber Tunnel streichen und Manifeste umschreiben (RA-06). |
| T-5 | trägt mit Korrektur | Freigabe auf den Quellhost begrenzen, auf `Clients` aufbauen (RA-19). |
| T-6 | trägt | Unveränderter TS ist richtig; Zeitabschnitte enger fassen (RA-15), TS-Bibliothek ernst nehmen (RA-16). |
| T-7 | trägt mit Korrektur | Grenzen widersprüchlich (RA-04); Wirkung auf LTV-007 als Frage (RA-03). |
| T-8 | trägt mit Korrektur | Entkopplung richtig; Schreiber brauchen eigenen Rückstau (RA-07); Leseprotokoll festlegen (RA-13). |
| T-9 | trägt | Eigener Sitzungstyp schützt `playback_sessions` und die Regel 2. |
| T-10 | trägt mit Korrektur | Lage vor der Umwandlung richtig; ohne Recht kein rückspulbares Fenster (RA-08); Kosten als Frage (RA-09). |
| T-11 | trägt mit Korrektur | Als Empfehlung haltbar; Startregel von Safari ist ein Strukturrisiko (RA-10). |
| T-12 | trägt mit Korrektur | Begründung überzeichnet (RA-05); PCR beim PID-Filter (RA-16). |
| T-13 | trägt mit Korrektur | Reservierung je Sitzung und neue Schnittstelle im Manager (RA-12); «unverändert» präzisieren (RA-23). |
| T-14 | trägt | Eigene Entscheidfunktion schont `playback.Decide` (`decide.go:318-326`). |
| T-15 | trägt mit Korrektur | Browser melden nie «sauber», die Vorgabe greift dort immer (RA-05). |
| T-16 | trägt mit Korrektur | Abzweig am Verteiler richtig; Rückstau (RA-07), weniger Teile (RA-15). |
| T-17 | trägt nicht (in dieser Fassung) | Haltegründe richtig, aber Karenz beim Umschalten und 401/403-Sperre legen Sender still (RA-01), Karenz fehlt in der Rangfolge (RA-02). Mit diesen Korrekturen tragfähig. |
| T-18 | trägt mit Korrektur | Vorgaben als Frage stellen, Schalter zusammenlegen (RA-18). |
| T-19 | trägt mit Korrektur | Ein Ort richtig; Karenz in die Rangfolge (RA-02), Kapazität der Referenz ausweisen (RA-05). |
| T-20 | trägt | Empfang im Server ausserhalb von `nice`/`oom_score_adj` ist richtig. |
| T-21 | trägt | Muster `rt` (`radio/token.go:11-30`); Bindung an die Einstell-Nummer sinnvoll. |
| T-22 | trägt | Ein Schreiber in SQLite; Zustände nur bei Wechsel. |
| T-23 | trägt | Abfragen genügen in der ersten Stufe. |
| T-24 | trägt | Riegel `liveq` nach Muster `wishq` (`server/.golangci.yml:64`), `os/exec` bleibt in `transcode`. |
| T-25 | trägt mit Korrektur | RAM-Disk für Timeshift nur mit Rechnung (RA-14); Rückstau bei Netzspeicher (RA-07). |
| T-26 | trägt mit Korrektur | Haken ist eine Kopplung in beide Richtungen (RA-22). |

## 4. Abweichungen vom Auftrag

| Nr. | Anforderung | Was der Entwurf tut | Als Frage an Marco ausgewiesen? |
|---|---|---|---|
| A-1 | LTV-007, Auftrag 621 («nicht vor dem Einschalten»); 03 LTV-007.11 | Einschaltpunkt als Anfang des ersten Segments umgedeutet; bis zu drei Bildgruppen, höchstens 15 s, rückspulbar (RA-03) | **nein**, als «keine Abschwächung» erklärt |
| A-2 | LTV-019 Punkt 1 (Speicherort des Startpuffers) | nur Arbeitsspeicher | ja, F-2 |
| A-3 | LTV-017 | Empfehlung, ein Limit je Quelle einzuführen | ja, F-3 (zulässig nach Leitplanken 4); die Folge von RA-01 fehlt darin |
| A-4 | LTV-002 und LTV-010 (Vorladen, häufig genutzte Sender, «automatisch») | gebaut, aber ab Werk dreifach aus | **nein**, als T-18 entschieden (RA-18) |
| A-5 | LTV-003 (RTSP, RTMP, DASH, HLS mit fMP4) | RTSP/RTMP in später Stufe (F-1); DASH und fMP4 nur nach M-10 | ja, F-1 und 05:197-202 |
| A-6 | DR-22 und Auftrag 10.6 (Durchsetzung im Server) | Fenster von 60 s ohne Recht, Pause im Player möglich (RA-08) | **nein**, nicht erkannt |
| A-7 | LTV-002 «ressourcenschonend», Auftrag 16 «Gemeinsame Timeshift-Puffer» | Kopie je Sitzung; die Referenzliste als «verboten» ausgelegt (RA-09) | teilweise, O-6 bietet nur eine nicht wortlauttreue Variante |
| A-8 | LTV-011 «eingefrorene Streams» | stehendes Bild erst mit optionaler Stichprobe (RA-25) | nein, aber offen benannt |
| A-9 | LTV-004 «individuelle Untertitelauswahl» | im Web nur eingebrannt, Teletext später (RA-17, O-5) | als O-5, nicht als Frage |
| A-10 | Auftrag 8.2 (18 Diagramme) | fehlen, kein Verweis (RA-24) | – |
| A-11 | Auftrag 11.1 und 11.2 (Szenarien) | auf `13` verschoben; Vorgaben in 04 Abschnitt 6.3 ohne Rechnung (RA-14) | als «Annahme» gekennzeichnet |

Vollständig erfüllt sind aus Abschnitt 8: die 13 Verantwortlichkeiten (04 Abschnitt 3.1, alle 13 zugeordnet, das
Zusammenfassen ist ausdrücklich erlaubt), die acht Ebenen (04 Abschnitt 4.1) und die Prüfung der Lage des Timeshift
(T-10). Aus Abschnitt 16 wird nichts gebaut oder vorbereitet. Zweifel gibt es nur bei A-7, und dort in umgekehrter
Richtung: Der Entwurf legt den Ausschluss enger aus als nötig, ohne zu fragen.

## 5. Unnötige Komplexität: streichen oder verschieben

Ohne eine verbindliche Anforderung zu verletzen:

| Was | Vorschlag | Ersparnis |
|---|---|---|
| `CONNECT`-Tunnel im Weiterleiter (05:170-176) | streichen, Manifeste umschreiben (RA-06) | ein Protokollweg, O-18, Teile von M-10 |
| Weg W insgesamt | späte Stufe; DASH und HLS mit fMP4 sind bei IPTV selten (05:201) | Weiterleiter, FFmpeg-Empfänger, Klasse «Empfang», planmässiger Ersatz alle 12 h |
| Weg L | späte Stufe, wie F-1 empfiehlt; drei neue Abhängigkeiten erst dann | Pflege von `gortsplib`, `gortmplib`, `mediacommon` |
| Planmässiger Ersatz des FFmpeg-Empfängers (05:181-184, O-20) | erst, wenn M-12 den Bedarf zeigt | Zeitsteuerung, Wahl des Augenblicks |
| Plattenpuffer in der Stufe ohne Timeshift | streichen (RA-08) | Schreiblast, Speicherort, Bereinigung in Stufe 1 |
| Delta-Listen | erst mit Timeshift | Code in Stufe 1 |
| Zwei Schalter je Quelle plus Art «automatisch» je Sender plus N | ein Schalter je Quelle «zusätzliche Verbindungen erlaubt» plus globale Zahlen (RA-18) | zwei Einstellungen und ihre Erklärung |
| Tabelle «Nutzung je Sender und Tag» (04:249) | aus den Sitzungszeilen rechnen (04:245) | eine Tabelle, ein Schreibpfad |
| Drei Wartezeit-Leitern (Wiederverbindung 1–20 s, «gestört» 60 s/5/15 min, Quarantäne 30 s/5/30 min) | auf zwei zusammenlegen: Leiter je Strom und Quarantäne je Quelle; «gestört» ist die verlängerte Leiter | Zustände, Tests |
| `compose.live-ram.yaml` (Timeshift im RAM) | streichen; wer will, hängt selbst ein tmpfs auf den einstellbaren Ordner (RA-14) | eine Zusatzdatei mit irreführender Grösse |
| Zwei harte Konstanten 15 s und 64 MiB | eine (15 s), Reservierung aus der Bitrate (RA-04) | Widerspruch |
| Stichprobe mit Dekodieren (05 Abschnitt 9.4) | eigene, spätere Stufe (RA-25) | Filter, Planung, Messung M-20 |

Nicht streichen, obwohl es nach Aufwand aussieht: die Haltegründe mit Besitzer (T-17), die 64-Bit-Zeit (05 Abschnitt
3.4), den Sitzungspuffer vor der Umwandlung (T-10). Sie tragen den Kern.

## 6. Widersprüche zwischen den Recherchen (24, Abschnitt 6)

| Nr. | In 04/05 aufgelöst? |
|---|---|
| W-1 Android progressiv gegen HLS | ja: HLS mit TS-Segmenten für alle (05 Abschnitt 4.3), progressiv verworfen. |
| W-2 gemeinsamer Ring gegen gemeinsamer Transcode | ja: nur der TS-Ring ist gemeinsam (T-6, T-13). |
| W-3 Go oder FFmpeg empfängt | ja: Go (T-3), FFmpeg nur in Weg W. |
| W-4 FFmpeg-Versionen | teilweise: 05:186-191 verlangt den Nachweis mit 7.1.4; 04:46 nennt R1-Aussagen dennoch «belegt» (RA-21). |
| W-5 Vorgaben von `DefaultLoadControl` | als offen gekennzeichnet (05:437); Werte werden gesetzt, nicht vorausgesetzt. Ausreichend. |
| W-6 Startpunkt von hls.js | **nein**: 05 setzt nur auf `EXT-X-START`, `liveSyncDurationCount` fehlt (RA-10). |
| W-7 Segmentlänge gegen Quell-GOP | ja: Direktweg an Keyframes der Quelle (2 bis 10 s), Umwandlung im Raster von 2 s (05 Abschnitt 4.4). |
| W-8 Lage und Format des Timeshift | ja: TS-Stücke je Sitzung vor der Umwandlung (T-10). |
| W-9 lokale Quellen | ja: Freigabe je Quelle (T-5); Umfang zu weit (RA-19). |
| W-10 Verbindungslimit | als Frage F-3; die Wirkung der Karenz fehlt darin (RA-01). |
| W-11 RFC 8216 | ja: als Annahme geführt (05:137-139). |
| W-12 `mpegts.js` | ja: verworfen (05 Abschnitt 4.2, 4.3). |
| W-13 TVHeadend-Timeshift | betrifft 04/05 nicht. |

## 7. Gesamturteil

Die Architektur ist im Kern richtig gewählt und einfacher als die naheliegenden Alternativen (FFmpeg als
Dauerempfänger, gemeinsamer Segmentvorrat, zweiter Dienst). Sie hält die Regeln 2, 3, 4, 6, 10 und 12 im Aufbau ein,
greift auf gebaute Muster zurück (Relay, Token, Budget, `netguard`) und lässt die Nähte der Dateiwiedergabe im
Verhalten unberührt.

**Nicht freigabereif in Fassung 1.** Vor der Freigabe zu beheben:
- RA-01 (kritisch): Karenz beim Umschalten und die Sperre bei 401/403. Sonst schaltet ein Haushalt mit einem
  Ein-Verbindungs-Zugang nicht um.
- RA-02 bis RA-10 (hoch). Darunter drei, die Marco entscheiden muss: RA-03 (Rückspulgrenze), RA-09 (Kopie je Sitzung
  oder einmal geschriebene Stücke) und RA-18 (Vorgaben für Vorladen). RA-08 ist eine Lücke in der Durchsetzung eines
  Rechts. RA-05 und RA-10 verlangen, dass M-2 und M-5 **vor** der Freigabe von T-11 und T-12 vorliegen, nicht erst vor
  dem Bau.

Mit diesen Korrekturen trägt der Entwurf. Die mittleren und niedrigen Befunde lassen sich in Fassung 2 und in den
Folgedokumenten (`06`, `13`, `16`, `17`) erledigen.
