# Konzept 28 · Review 5 Architektur, Zweitprüfung

Stand: 29. November 2026 · Reviewer: oh-my-claudecode:architect (anderer Lauf als Review 1)

Geprüft, nur lesend: `04-architektur.md` (Fassung 3), `05-streaming-und-fast-zapping.md` (Fassung 3), `06-ablaeufe.md`,
`10-timeshift.md`, `11-dvr.md`, `13-ressourcen-und-performance.md`, `16-client-kompatibilitaet.md` (je Fassung 2), dazu
die Stellen in `07-datenmodell.md`, `08-api.md` und `20-offene-entscheidungen.md`, die Laufzeitverhalten festlegen.
Massstab: `anhang/reviews/review-1-architektur.md` (RA-01 bis RA-26), `anhang/nachpruefung-quellen.md`, Auftrag
(LTV-002, 003, 004, 007, 008, 010, 011, 019, 020; Abschnitte 8, 11, 16), `03-fachkonzept.md`, Leitplanken, Code im
Arbeitsbaum. Zeilenangaben `04:n`, `05:n` usw. beziehen sich auf den Stand vom 28. November 2026, 15:03. Keine Builds,
keine Tests. Alles, was nicht gemessen ist, ist auch hier **Annahme**; Rechnungen sind angegeben.

## 1. Kurzfassung

Die zehn kritischen und hohen Befunde von Review 1 sind im Kern bearbeitet, nicht nur im Protokoll: Karenz beim
Umschalten ist weg, die Ablehnungen werden nach Lage gedeutet, der Ring hat eine Grenze, ohne Recht gibt es kein
rückspulbares Fenster, Kapazität und Direktweg sind ehrlich gerechnet, Wortlaut-Abweichungen stehen als Fragen.
Die Korrekturen haben aber an vier Stellen neue Fehler erzeugt, die aus dem Zusammenspiel der Fassung-3-Regeln
entstehen und in keinem Dokument durchgespielt sind:

1. Der 15-Sekunden-Ring ist seit T-10 in den Stufen 1 bis 3 die **einzige** Quelle der Segmente aller Zuschauer.
   Mit Segmenten an Quell-Keyframes (2 bis 10 s) und einem Fenster von drei Segmenten passt das bei langen
   Bildgruppen, bei Bitratenspitzen und bei einem verkürzten Ring nicht (RB-01).
2. Der Abbruch nach 15 Sekunden ohne Segment widerspricht der Regel «auch Sender mit Bildgruppen über 15 s
   laufen» (RB-02).
3. Bei Anbietern, die bei einer zweiten Verbindung die **ältere** trennen, erzeugen die Regeln eine Endlosschleife
   zwischen zwei gehaltenen Strömen (RB-03), und ein zurückgestellter Dauersender kommt alle 60 Sekunden wieder
   und stört dabei laufende Zuschauer (RB-04).
4. T-29 lässt Ströme einer **gelöschten** Quelle weiterlaufen, entgegen der Regel für entzogene feste Ziele
   (RB-05); und LTV-004 «individuelle Untertitelauswahl» wird für den Browser als erfüllt bezeichnet, obwohl
   `20` E-28-56 das Gegenteil festhält (RB-06).

| Schwere | Anzahl |
|---|---|
| kritisch | 0 |
| hoch | 6 |
| mittel | 9 |
| niedrig | 6 |

Urteil: **als Konzept zur Vorlage beim Auftraggeber geeignet nach Korrekturen** (RB-01 bis RB-06, Textarbeit in
04, 05, 06, 08, 16, 20; kein neuer tragender Entscheid nötig). Vor einer Baufreigabe zusätzlich nötig: Abschnitt 8.

## 2. Stand der Befunde RA-01 bis RA-10

Je Befund wurde das Szenario aus Review 1 gegen den neuen Text gespielt.

| Befund | Stand | Szenario gegen den neuen Text | Stelle |
|---|---|---|---|
| RA-01 kritisch, Karenz + Limit + Sperre | **teilweise** | Zehn Umschaltungen auf einem Zugang mit einer Verbindung: A geht ohne Karenz auf «ruht» (04:444), das Ende wird höchstens 1 s abgewartet (04:379), 401/403 bei offener Nachbarverbindung sperrt nichts (04:414). Das Szenario von Review 1 ist geschlossen. Offen bleiben drei Nachbarfälle desselben Musters: Anbieter, die die ältere Verbindung trennen (RB-03), Anbieter, die eine geschlossene Verbindung verzögert freigeben (RB-10), und «Zugang prüfen» bei Fremdnutzung des Zugangs (RB-09). | 04:93, 04:373-423, 04:439-455, 04:820-823 |
| RA-02 hoch, Karenz in der Rangfolge | **behoben** | Neun Sender in 30 s durchschalten: Ströme ohne Haltegrund zählen nicht (04:364-365), Stufe 0 (04:762). Wegen RA-01 entsteht ohnehin keine Karenz beim Umschalten. | 04:762-763, 04:722 |
| RA-03 hoch, LTV-007 umgedeutet | **behoben** | Vorgabe wortlauttreu (04:64), Frage F-7 mit Varianten (04:1236-1253), Menge korrigiert (05:408-411), Umsetzung in `10` 4 (Stück 0). | 04:83, 05:403-436, 10:142-163 |
| RA-04 hoch, Grenzen des Rings | **teilweise** | Der Widerspruch 15 s gegen 64 MiB ist weg (04:83, 05:260). Die neue Regel für lange Bildgruppen (05:375-384) kollidiert aber mit dem Abbruch nach 15 s (RB-02) und mit dem Fenster von drei Segmenten aus dem Ring (RB-01); die Reservierung ohne gespeicherte Bitrate bleibt bis zum nächsten Zeitabschnitt zu klein (RB-16). | 04:83, 05:366-401 |
| RA-05 hoch, Direktweg überzeichnet | **behoben** | Tabelle je Client und Kapazität der Referenz (04:668-714), übernommen in `13` 3.5 und `16` 8; «zwei Browser-Zuschauer auf Zeilensprung» steht im Klartext (04:699-701). Die Rechnungen stimmen (4 ÷ 2 = 2; 4 ÷ 1,5 = 2; 8 ÷ 1,5 = 5). | 04:688-714; 13:400-426; 16:278-292 |
| RA-06 hoch, `CONNECT`-Tunnel | **behoben** | Kein Tunnel, Ersatzpfad direkt auf den Weiterleiter, TLS nur in Go, Sperre Pflicht, Weg W in eigener Stufe hinter M-10 (04:79-80). Das Restrisiko (Landlock regelt TCP nur nach Port) ist offen benannt. | 04:80, 04:1023 |
| RA-07 hoch, 15 s Toleranz der Schreiber | **teilweise** | Eine einzelne Aufnahme hat jetzt 60 s (04:742, 11:413). Bei mehreren Aufnahmen auf demselben Netzspeicher teilt sich ein gemeinsamer Rest ohne Vorrang, den dauernd belegte Ausgabestücke der Umwandlungen mit verbrauchen (RB-08). | 04:737-751, 13:366-398 |
| RA-08 hoch, Zeitversatz ohne Recht | **behoben** | Ohne Recht nur drei Segmente, ältere 404, kein Plattenpuffer vor Stufe 4 (04:86, 05:535). Rest: Für native Apple-Player bis sechs Segmente (bis 15 s rückspulbar ohne Recht) ist als `05` O-28 offen benannt. Neue Folge: RB-01. | 04:86, 05:535, 10:165-176 |
| RA-09 hoch, Kosten von T-10 | **behoben** | F-8 mit Rechnung für 1, 5 und 20 Zuschauer und drei Varianten (04:1255-1283). | 04:1255-1283 |
| RA-10 hoch, Startregel Safari | **behoben als benanntes Risiko** | Spannung benannt (05:484-506), Varianten vorbereitet, `startPosition` vom Server und `liveSyncDurationCount` 2 festgelegt (05:622-629). Nachweis unmöglich, solange kein Apple-Gerät da ist; diese Clients sind «ungeprüft und nicht zugesagt» (04:1114). Das ist ehrlich, aber kein Beleg. | 04:87, 04:1113-1114, 05:467-506 |

**Mittlere Befunde, die das Protokoll als «nicht übernommen» oder abweichend führt:**

| Befund | Begründung im Protokoll | Urteil |
|---|---|---|
| RA-18, Teil «automatisch ohne eigene Art je Sender» | LTV-002.05 und .06 verlangen eine globale Vorgabe und eine Strategie je Sender, LTV-002.02 nennt «automatisch» als Strategie (04:1445; `03`:217-221). | **stichhaltig.** |
| RA-25 | «Keine eigene Frage, weil nichts gestrichen wird» (04:1404). | **überholt:** F-14 (04:1354-1370) ist inzwischen eine eigene Frage. Protokoll nachführen (RB-19). |
| RA-26, Punkt Weg-W-Empfänger | Protokoll: «ausserhalb des Live-Anteils (8.1)» (04:1405). | **widerspricht dem Text:** 04:731-735 und 13.4 (04:1471) sagen «innerhalb». Protokoll nachführen (RB-19). |

## 3. Befunde

### RB-01 · hoch · Der 15-Sekunden-Ring trägt das Fenster von drei Segmenten nicht

**Stelle:** T-7 (04:83), T-10 (04:86), 05:356, 05:375-384, 05:398-400, 05:535-536; 04 14.3 und 14.5 (04:1567, 04:1599);
`07`:279, `08`:1216 («Ring 4 bis 15 s»).

**Problem.** Ohne Sitzungspuffer, also für **alle** Zuschauer der Stufen 1 bis 3 (der Plattenpuffer kommt erst mit
Stufe 4, 04:86), liefert der Server die drei neuesten Segmente «direkt aus dem Ring, nur soweit sie dort noch liegen»
(05:535), blockweise, und bricht eine Antwort ab, wenn der Block inzwischen überschrieben ist (05:356). Segmente im
Direktweg beginnen an Quell-Keyframes und dauern 2 bis 10 s (05:536). Der Ring hält höchstens 15 s **einschliesslich**
der laufenden, noch unfertigen Bildgruppe.

Rechnung (Segmentdauer s, Ring 15 s, laufender Teil 0 bis s):

| Segmentdauer | vollständige Segmente im Ring | Folge ohne Recht «Timeshift» |
|---|---|---|
| 2 s | 6 bis 7 | trägt |
| 4 s | 2 bis 3 | das älteste der drei ist zeitweise angeschnitten |
| 6 s | 1 bis 2 | hls.js mit `liveSyncDurationCount` 2 startet zwei Segmente vor dem Ende, also ausserhalb oder am Rand des Rings |
| 10 s | 0 bis 1 | ein fertiges Segment bleibt nach seiner Fertigstellung nur rund 5 s abrufbar; ein 10-MB-Segment (8 Mbit/s) braucht dafür 16 Mbit/s beim Client; danach 5 s ohne neues Segment: Stockung |

Verschärfend:
- Der Ring ist nach der **mittleren** Bitrate reserviert (04:83, 05:260). In einer Bitratenspitze (Sport, Szenenwechsel)
  hält er weniger als 15 s.
- Der Ring ist «nach unten einstellbar» bis 4 s (04:1567, `08`:1216), und 05:398-400 **empfiehlt** 6 s, um Speicher zu
  sparen. Bei 4 s und Segmenten von mindestens 2 s passt nie mehr als ein vollständiges Segment hinein; bei 6 s sind es
  zwei. Damit bricht die Empfehlung die Wiedergabe ohne Recht.
- Die Testquelle Q-D (Bildgruppe 10 s, `05` 8.2) fällt genau in die letzte Zeile; `13` 1.1 führt für IPTV- und
  Internet-Quellen eine Spanne von 1 bis 10 s.

Die Prüfung des Testplans «kein Abnehmer erhält ein Paket älter als Einstellen minus 15 s» (05:436) erzwingt das
Problem zusätzlich, weil sie die Archivgrenze mit der Auslieferung koppelt.

**Beleg:** 05:356, 05:375-384 (dort selbst: «bei langen Bildgruppen sind es weniger als drei», 05:535), 05:536, 04:1567.

**Korrektur.**
1. Archivgrenze und Auslieferungsfenster trennen, wie RA-07 es für die Schreiber getan hat: Die Grenze gegen ein
   Archiv ist eine Regel des **Fensters** (nichts vor dem Einschalten ausser dem Anlauf), nicht der Byte-Länge des
   Rings. Der Ring hält «mindestens Anlauf plus drei vollständige Segmente, höchstens 30 s» (harte Konstante); die
   15 s bleiben Vorgabe für kurze Bildgruppen. 15 s stammen nicht aus dem Auftrag (Auftrag 623-625 verlangt nur
   «kurzfristig», «kein Archiv»).
2. Die untere Einstellgrenze des Rings aus der gemessenen Segmentdauer ableiten (nie unter Anlauf plus drei Segmente);
   die Empfehlung «6 s» in 05:398-400 streichen.
3. Prüfung 05:436 auf «kein Segment, das vor dem Anlauf beginnt» umstellen.
4. In V-2 (M-3) und V-5 (M-4) ausdrücklich messen: Wiedergabe ohne Recht mit Segmenten von 2, 6 und 10 s aus einem Ring
   der gewählten Länge, mit Client im WLAN von 10 Mbit/s.

### RB-02 · hoch · Kalter Start: Abbruch nach 15 s gegen lange Bildgruppen und Umwandlung

**Stelle:** 04:342 («gilt auch dann als läuft, wenn seine Bildgruppen länger als 15 s sind»), 05:380-381, 04:836,
05:542, 04 14.1 (04:1544), 04:361 (Prüfung höchstens 10 s), 04:727-728 (Reservieren wartet bis 10 s).

**Problem.** Ein noch nicht empfangener Sender braucht bis zum ersten Segment: Verbindung, Warten auf den ersten
Keyframe (bis eine Bildgruppe) und das Ende dieser Bildgruppe (eine Bildgruppe), weil Segmente nur an Keyframes enden
(05:585). Bei einer Bildgruppe G also bis 2 × G plus Verbindung. Der Startversuch endet aber nach 15 s ohne Segment
(04:836, 04:1544; Muster `readyTimeout` des Radios, `server/internal/radio/relay.go:56`, dort für Ton gedacht).

| Bildgruppe | Zeit bis zum ersten Segment (Verbindung 0,6 s) | gegen 15 s |
|---|---|---|
| 2 s | bis 4,6 s | trägt |
| 6 s | bis 12,6 s | knapp |
| 8 s | bis 16,6 s | scheitert zeitweise |
| über 15 s | über 30 s | scheitert immer, obwohl der Sender als «läuft» gilt |

Mit Umwandlung kommen dazu: Probe bis 1 s, Warten auf Einheiten bis 10 s, Start von FFmpeg bis 1,8 s (05:587). Schon bei
G = 2 s sind das im ungünstigen Fall 0,6 + 4 + 1 + 10 + 1,8 = 17,4 s: Der Versuch bricht ab, während er legitim auf
das Budget wartet. Dasselbe Muster trifft die Prüfung ruhender Sender (höchstens 10 s, 04:361): Bei G ≥ 5 s meldet sie
einen gesunden Sender als nicht verfügbar.

**Korrektur.** Abbruchfrist aus dem Zustand rechnen statt fest: «Verbindung plus zweimal die gespeicherte Bildgruppe
plus Wartezeit auf Einheiten plus 5 s, mindestens 15 s, höchstens 45 s»; ohne gespeicherte Bildgruppe 30 s. Gleiche
Regel für die Prüfung (Frist mindestens zweimal die gespeicherte Bildgruppe). Der Zustand «startet» meldet über den
Herzschlag den Grund («wartet auf Keyframe», «wartet auf Umwandlung»). Sender mit G > 7 s in der Verwaltung als
Kandidaten für «dauerhaft» kennzeichnen (das steht schon in 05:677, aber ohne Folge für den Abbruch).

### RB-03 · hoch · Verdrängungs-Pingpong zwischen zwei gehaltenen Strömen derselben Quelle

**Stelle:** 04:395-399 (Leiter, Zurücksetzen bei Daten bis zu einem Keyframe), 04:419-423 («Gegenrichtung» nur nach
vorsorglicher Verbindung), F-3 (a) (04:1196-1199); Code: `server/internal/radio/relay.go:223-225` (`published`
setzt `attempt = 0`).

**Problem.** Review 1 nannte zwei Ausgänge bei einem Zugang mit einer Verbindung: Der Anbieter lehnt die neue ab, oder
er **wirft die ältere hinaus** (RA-01, Punkt 1: «harmlos»). Fassung 3 behandelt nur den ersten Ausgang sauber. Für
den zweiten gilt mit zwei Zuschauern (oder Zuschauer und Aufnahme) auf verschiedenen Sendern derselben Quelle:

1. Zuschauer 2 stellt B ein. Der Anbieter nimmt B an und trennt A.
2. A bricht ab, ist gehalten (Zuschauer 1), wartet 1 s und verbindet neu. Der Anbieter nimmt A an und trennt B.
3. B bricht ab und verbindet nach 1 s neu, und so fort.

Jede Wiederverbindung liefert Daten bis zu einem Keyframe und setzt die Leiter zurück (04:397-398, gebaut im Radio
so). Die Leiter wächst also nie; die «Gegenrichtung» (04:419) greift nicht, weil keine der beiden Verbindungen
vorsorglich ist. Ergebnis: beide Bilder stocken im Sekundentakt, ohne Ende, ohne Meldung «Quelle lehnt ab». F-3 (a)
beschreibt nur «für die betroffene Person sieht es wie eine Störung aus», nicht die Schleife. Mit einer laufenden
Aufnahme heisst das: Lücken im Abstand weniger Sekunden über die ganze Sendung.

**Korrektur.** Fehlerbehandlung, keine Limitverwaltung (LTV-017 bleibt unberührt):
1. «Ein gehaltener Strom bricht innert 10 s ab, nachdem **irgendeine** andere Verbindung derselben Quelle aufgebaut
   wurde» gilt als «Quelle lehnt eine weitere Verbindung ab». Der jüngere Abnehmer verliert: Er bekommt den Fehlercode,
   der ältere verbindet ohne Wartezeit neu. Für «Aufnahme gegen Zuschauer» gilt F-5.
2. Die Leiter wird erst nach einer gesunden Laufzeit zurückgesetzt (das Radio hat dafür schon `healthyAfter` von
   1 Minute für Senderfehler, `relay.go:47`, `:59`), nicht beim ersten Keyframe.
3. Ablauftest im Testplan: Testserver, der bei einer zweiten Verbindung die ältere trennt; erwartet: nach höchstens
   zwei Wechseln stabil, ein Abnehmer mit Fehlercode, keine Schleife.

### RB-04 · hoch · Zurückgestellter Dauersender: Wiederanlauf undefiniert, Flattern, Störung laufender Zuschauer

**Stelle:** 04:414 (bei Ablehnung für Zuschauer wird Dauerhaftes «zurückgestellt»), 04:457-469 (Wiederanlauf «sobald
die Grösse, die gefehlt hat, seit 60 s frei ist»), 04:336, `06`:184-186, 04 14.1 (04:1535).

**Problem.** «Zurückgestellt» entsteht auf zwei Wegen: bei Mangel einer der **vier** Grössen (8.2 Stufe 5) und bei
einer **Ablehnung durch die Quelle** (04:414). Im zweiten Fall ist die fehlende «Grösse» eine Verbindung beim
Anbieter, die Slouchcat nicht zählt. Das Wiederanlauf-Kriterium ist dann nicht definiert; die naheliegende Umsetzung
(«keine der vier Grössen fehlt») startet den Dauersender nach 60 s neu. Dann:
- Lehnt der Anbieter ab: Der Versuch ist ein «ausdrücklicher Auftrag» (dauerhaft), also gibt der Server nach 04:414
  bei dieser Quelle frei und versucht einmal sofort neu; der Folgezustand nach dem zweiten Fehlschlag ist für einen
  Strom **ohne** Abnehmer nirgends festgelegt (Fehlercode «an den Abnehmer», den es nicht gibt).
- Trennt der Anbieter die ältere Verbindung: Der Dauersender wirft den Zuschauer hinaus, der Zuschauer verbindet neu
  und stellt den Dauersender wieder zurück (RB-03). Das wiederholt sich alle 60 s, solange der Zuschauer schaut.
- Zustandstabelle `06`:184-186 kennt für «zurückgestellt» weder «Zulassung beim Wiederanlauf verweigert» noch
  «Quelle lehnt beim Wiederanlauf ab».

Zum Verhungern: Unter Dauerlast (Obergrenze von Zuschauern belegt) startet ein Dauersender nie; das ist mit F-4 (a)
gewollt. Unter Zurückgestellten gewinnt immer die kleinere Kanalnummer (04:462): Sender mit hoher Nummer verhungern
auch, wenn zwischendurch Platz frei wird. Das ist hinnehmbar, steht aber nicht als Folge in F-4.

**Korrektur.**
1. Zwei Ursachen unterscheiden: «zurückgestellt wegen Mangel» (Kriterium wie bisher) und «zurückgestellt, weil die
   Quelle ablehnte» (Wiederanlauf nur, wenn bei dieser Quelle seit 60 s keine andere Verbindung offen ist; sonst
   warten, kein Versuch).
2. Ein Wiederanlauf aus «zurückgestellt» zählt in der Tabelle der Ablehnungen als **vorsorglich** (04:415): keine
   Freigabe, kein Sofort-Wiederholen, Gegenrichtung gilt.
3. Folgezustand nach gescheitertem Wiederanlauf: zurück nach «zurückgestellt» mit verlängerter Leiter (60 s, 5 min,
   15 min), Zähler in der Verwaltung.
4. Reihenfolge der Wiederanläufe: ältester Zurückgestellter zuerst statt Kanalnummer; als Folge in F-4 nennen.

### RB-05 · hoch · T-29 hält Ströme einer gelöschten Quelle am Leben; Laufzeit ohne Grenze

**Stelle:** T-29 (04:105), 04:481-484, 04:848-849, 04:1058 (Entzug eines festen Ziels), 04:1061 (Transport je
Regelstand); `07`:1908-1922 (Quelle entfernt: Sender `enabled = 0`, Ziele und Ursprünge gehen mit), `07`:1928;
`10`:69; `11`:444; `08`:1411 (Token-Erneuerung über den Herzschlag).

**Problem.**
1. **Quelle gelöscht.** Nach `07` 10.3 werden die Sender einer gelöschten Quelle deaktiviert. Nach T-29 beendet das
   nichts Laufendes: Zuschauer und Aufnahmen laufen weiter, mit den entschlüsselten Zugangsdaten im Speicher. Zugleich
   löscht dieselbe Transaktion die festen Ziele und Ursprünge der Quelle. 04:1058 verlangt für den **Entzug** eines
   festen Ziels, dass laufende Verbindungen innert 5 s enden. Beide Regeln widersprechen sich; welcher Regelstand für
   die nächste Segmentanfrage einer HLS-Quelle gilt (jedes Segment ist eine neue Anfrage), ist offen. Der übliche
   Grund, eine Quelle zu löschen (Zugang kompromittiert, Liste gekapert, Anbieter nicht mehr gewollt), ist genau der
   Fall, in dem Weiterlaufen falsch ist.
2. **Wie lange ein deaktivierter Sender läuft.** Eine Wiedergabe läuft, «bis der Zuschauer geht». Das Token wird über
   den Herzschlag alle 11 Stunden erneuert (`08`:1411); dass die Erneuerung den Zustand des Senders prüft, steht
   nirgends. Ein Fernseher, der über Nacht läuft, hält den Strom also unbegrenzt. Eine Aufnahme läuft «bis zu ihrem
   Ende», bei einer Sofortaufnahme ohne Ende ebenfalls offen.

**Korrektur.**
1. T-29 auf das Deaktivieren eines **Senders** beschränken. Löschen einer Quelle, Entzug eines festen Ziels oder
   Ursprungs und Ändern der Zugangsdaten beenden die Verbindungen dieser Quelle nach Bestätigung mit Zahl der
   Betroffenen (wie 04:1058); Aufnahmen bekommen eine Lücke und gehen, falls vorhanden, auf eine Ersatzquelle.
   `07` 10.3 entsprechend nachziehen.
2. Die Token-Erneuerung prüft, ob der Sender noch aktiv ist; sonst endet die Wiedergabe spätestens mit dem Ablauf des
   Tokens (12 h). Für Aufnahmen auf deaktivierten Sendern gilt das geplante Ende, für Sofortaufnahmen eine Höchstdauer.

### RB-06 · hoch · LTV-004 «individuelle Untertitelauswahl» im Browser: als erfüllt bezeichnet, tatsächlich nicht

**Stelle:** 04:634-636 («Im Browser und auf dem iPhone gibt es Untertitel in den ersten Stufen nur eingebrannt … ist
dort erfüllt, aber teuer»), `16`:215-217 (gleicher Satz), `16`:241; dagegen `20`:812-824 (E-28-56: «Bis Stufe E zeigt
der Browser darum keine Untertitel; LTV-004.17 ist dort bis dahin nicht erfüllt», Vorgabe (a) WebVTT in Stufe E;
Einbrennen ist dort nur Variante (c)); `18`:492.

**Problem.**
1. Zwei Dokumente sagen «erfüllt», das Entscheidungsdokument sagt «nicht erfüllt». Nach `20` wird das Einbrennen gar
   nicht gebaut, solange Marco nicht (c) wählt. Dann gibt es im Browser in den Stufen 1 bis 4 **keine**
   Untertitel.
2. Teletext-Untertitel zeichnet in keiner Stufe vor E irgendein Client (04:631, `16`:222). Bei Sendern, die Untertitel
   nur als Teletext führen (im deutschsprachigen Rundfunk verbreitet; Annahme, M-2), ist LTV-004 auf **allen** Clients
   bis Stufe E nicht erfüllt. Das steht nirgends als Abstrich.
3. Für Media3 sagt `20` «DVB-Untertitel schon ab Stufe 1a», 04:630 und `16`:221 sagen «Annahme, M-24», und M-24 liegt
   in Stufe E (`16`:321).
4. Wird doch eingebrannt, kostet jeder Browser-Zuschauer mit Untertiteln eine Bild-Umwandlung (2 Einheiten). Auf
   CT 190 heisst das: höchstens zwei Zuschauer mit Untertiteln gleichzeitig. Diese Folge fehlt in 7.6.

**Korrektur.** 04 7.4 und `16` 6 auf den Stand von `20` bringen: «bis Stufe E im Browser nicht erfüllt; Teletext auf
keinem Client vor Stufe E». In `20` E-28-56 den Teletext-Fall und die Kapazitätsfolge von (c) ergänzen und Marco
fragen, ob dieser Abstrich bis Stufe E hinnehmbar ist oder WebVTT vorgezogen wird. Die Aussage zu Media3 auf eine
Lesart bringen.

### RB-07 · mittel · Neuer Zuschauer auf einem gestörten oder wartenden Strom; Zustand nach dem Abbruch widersprüchlich

**Stelle:** 04:403-406 (verlängerte Leiter 60 s, 5 min, 15 min), 04:836, `06`:161, `06`:177; `08`:631-632 («der nächste
Herzschlag meldet `reconnecting` mit `retryInSeconds` oder die Sitzung 410»), `08`:646-647 («Ein gestörter Sender
liefert beim Einstellen 201 mit `state: reconnecting`»).

**Problem.** Ein dauerhaft empfangener Sender ist «gestört»; der nächste langsame Versuch ist bis zu 15 Minuten
entfernt. Ein Zuschauer stellt ihn ein: Kein Dokument sagt, dass ein neuer Haltegrund auf Abruf einen sofortigen
Versuch auslöst. Nach `08` bekommt er `reconnecting` mit `retryInSeconds` bis 900. Dasselbe im Zustand «wartet» (bis
20 s), wobei der Versuch nach 15 s schon als gescheitert gilt. Ausserdem: 04 und 06 sagen, nach 15 s ende der
Startversuch «mit einem Fehlercode»; 08 sagt, die Sitzung bleibe in `reconnecting`. Was dabei mit Haltegrund,
Platz bei den Stream-Grenzen und Reservierung der Umwandlung geschieht, ist offen.

**Korrektur.** Neue Zeile im Automaten: «Gestört oder Wartet, neuer Haltegrund Zuschauer oder Aufnahme: ein Versuch
sofort, höchstens einer je Minute und Sender, danach weiter auf der Leiter». Eine Lesart für das Ende des
Startversuchs: Haltegrund lösen, Platz und Reservierung zurückgeben, Herzschlag meldet Fehlercode; `08` anpassen.

### RB-08 · mittel · Rückstau: ein gemeinsamer Rest ohne Vorrang, teilweise dauernd belegt

**Stelle:** T-19 (04:95), 04:737-751, 04:818, 04 14.3 (04:1571-1574), `13`:116-120, `13`:366-389, `11`:413.

**Problem.**
1. Zum «Rückstau bei Gebrauch» zählen auch Zuführer und Ausgabestücke einer Umwandlung ohne Zeitversatz (je bis 8 MiB,
   04:742). Die sind **nicht** «im Normalbetrieb leer»: Die Ausgabestücke sind die Segmente, die der Client gerade
   holt. 16 Ton-Umwandlungen (Budget von CT 190) belegen bis 16 × 16,8 MB = 269 MB dauernd.
2. Ringe dürfen drei Viertel belegen; der Rest kann also auf ein Viertel (128 MB bei 512 MB) sinken. Teilen sich
   dann stockende Aufnahmen, Sitzungspuffer und Umwandlungen diesen Rest «zu gleichen Teilen», kann eine Aufnahme nahe
   null Toleranz haben. «Eine Aufnahme wird nie wegen des Rückstaus abgelehnt» ist dann formal wahr, aber jede
   Stockung von einer Sekunde wird zur Lücke.
3. Liegen alle Aufnahmen auf demselben Netzspeicher (der Normalfall bei `compose.live.yaml`), stocken sie **zugleich**.
   `13`:379 rechnet bei zehn HD-Aufnahmen 36 s gegen eine Neuverbindung von SMB von 15 bis 60 s (Annahme RA-07).
   «Laufende Aufnahmen geschützt» (8.2) schützt vor Verdrängung, nicht vor dieser Lücke; das sagt der Text nicht.

**Korrektur.** Ausgabestücke und Zuführer bei der Zulassung der Umwandlung reservieren (sie sind immer in Gebrauch),
nicht aus dem Rest. Im Rest Vorrang für Aufnahmen: je Aufnahme ein Mindestteil (Vorschlag 16 MiB, rund 16 s HD), dann
Sitzungspuffer. In 8.2 den Satz ergänzen, dass gleichzeitige Stockungen eines Netzspeichers bei vielen Aufnahmen
Lücken geben, mit der Rechnung aus `13` 3.4.

### RB-09 · mittel · «Zugang prüfen» sperrt die Quelle bei Fremdnutzung des Zugangs

**Stelle:** 04:416 (zweimal 401 oder 403 im Abstand von mindestens 10 s ohne andere Verbindung → «Zugang prüfen», «bis
der Administrator sie ändert, testet oder die Liste sie neu liefert»), `06`:158.

**Problem.** Wird derselbe Zugang ausserhalb von Slouchcat genutzt (eine IPTV-App auf dem Handy, ein zweiter Rechner),
sieht Slouchcat keine eigene offene Verbindung. Ein Zuschauer stellt ein: 403, Leiter 1, 2, 5, 10 s, zweites 403 nach
mindestens 10 s. Nach rund 18 s ist die Quelle für alle Sender, alle Zuschauer und alle geplanten Aufnahmen gesperrt,
bis jemand in der Verwaltung handelt. Das ist der Schaden von RA-01 auf einem anderen Weg. Viele Panels antworten auf
«zu viele Verbindungen» mit 403 (Annahme, M-17).

**Korrektur.** «Zugang prüfen» als Quarantäne mit langsamem, selbsttätigem Wiederversuch (Vorschlag 5 min, 30 min, dann
stündlich) statt als Sperre bis zum Eingriff. Bei Xtream zusätzlich die Anmeldung über die Schnittstelle der Quelle
prüfen: Gelingt sie, ist der Zugang gültig und die Ablehnung ein Verbindungsproblem (Fehlerdeutung, kein Limit).

### RB-10 · mittel · «Einmal sofort neu» trifft Anbieter, die eine Verbindung verzögert freigeben

**Stelle:** 04:379 (Ende abwarten höchstens 1 s), 04:414 («einmal sofort neu»), 04:820-823 (Abnahmeprüfung),
`06`:542, `06`:571-572.

**Problem.** Die Regel wartet auf das Ende der **eigenen** TCP-Verbindung. Ob der Anbieter den Platz dann schon
freigibt, weiss niemand; manche Panels zählen eine Verbindung noch einige Sekunden nach dem Schliessen (Annahme,
M-17). Dann scheitert bei einem Zugang mit einer Verbindung **jedes** Umschalten auf einen nicht empfangenen Sender:
403, sofortiger zweiter Versuch, 403, Fehlercode. Der Testserver der Abnahmeprüfung gibt den Platz sofort frei und
deckt den Fall nicht ab.

**Korrektur.** Zweiter Versuch nach einer kurzen Pause innerhalb des Startfensters (Vorschlag: 1 s, dann 2 s, zusammen
höchstens 3 Versuche in 5 s), dann Fehlercode. Abnahmeprüfung um einen Testserver ergänzen, der den Platz erst 3 s
nach dem Schliessen freigibt. M-17 (Verhalten von Marcos Anbietern bei einer zweiten Verbindung: ablehnen, ältere
trennen, verzögert freigeben) in die Stufe V ziehen, weil es Stufe 1 betrifft.

### RB-11 · mittel · Herzschlag und Hintergrund: zwei Regeln, die sich ausschliessen; Folgen der 60 s

**Stelle:** 04:904-912, 05:640-642, `16`:244, 04:445, 04:1585.

**Problem.**
1. «Geht die App in den Hintergrund …, beendet der Client die Sitzung» und im selben Absatz «eine kurze Unterbrechung
   (Anruf) innerhalb von 60 s übersteht die Sitzung, weil der Herzschlag so lange ausbleiben darf» (04:905-909).
   Beendet der Client aktiv, übersteht nichts; übersteht die Sitzung, beendet der Client nicht. Das muss eine Regel
   sein (Vorschlag: Client beendet erst nach 30 s im Hintergrund).
2. Fernseher am Netzschalter aus oder App abgestürzt: Der Zuschauer-Haltegrund bleibt 60 s, danach 30 s Karenz. Bei
   einem Zugang mit einer Verbindung kann in den ersten 60 s niemand im Haushalt einen anderen Sender derselben Quelle
   sehen (ein Zuschauer-Haltegrund ist geschützt, 04:414). Das gehört als Folge der 60 s in 04 10.3.
3. Eine Sitzung **ohne** Recht «Timeshift», die «Pause» meldet (Medientaste im Browser), hält Strom, Verbindung und
   Platz unbegrenzt, ohne dass es etwas zu puffern gibt.

**Korrektur.** Eine Regel für den Hintergrund; die 60-s-Folge benennen; ohne Recht endet eine Sitzung nach 60 s
gemeldeter Pause wie bei Sitzungsende durch den Zuschauer (mit Karenz).

### RB-12 · mittel · Zustandsautomat: Lücken und Abweichungen zwischen 04, 06, 08, 10

**Stelle:** 04:310-340, `06`:78-133, `06`:148-186, `08`:631-632, `08`:1250, `10`:391-434, `11`:239-290.

**Problem.** Fehlende oder widersprüchliche Übergänge (Abschnitt 4 hat die vollständige Liste):
- 04 5.1 kennt drei Übergänge nicht, die `06` 2.1 einzeichnet (Verbindet → Verbindet bei Limitverdacht, Läuft →
  Verbindet nach der Gegenrichtung, Herunterfahren). `06` sagt von sich, es füge keine Regel hinzu (`06`:7).
- Mangel trifft in `06`:172 nur «Läuft, Wartet, Gestört»; ein Dauersender im Zustand **Verbindet** und vorgeladene
  Ströme in **Verbindet** oder **Wartet** sind bei Mangel nicht geregelt.
- Ein **endender** Strom (Verbindung wird geschlossen, bis 1 s) ist eine Regel in 04:392 und 04:379, aber kein Zustand.
  Eine Aufnahme, deren Aufwärmzeit in dieses Fenster fällt, ist nur über den Satz «Beitritt wartet» abgedeckt.
- Entzug eines festen Ziels (04:1058) und Löschen der Quelle (RB-05) fehlen als Ereignis.
- Gestört oder Wartet mit neuem Abnehmer (RB-07); Zurückgestellt mit verweigerter Zulassung oder Ablehnung (RB-04).
- Der Sitzungsautomat in `10` 13 kennt keinen Zustand für «Quelle weg» (`08` hat `reconnecting`) und keinen für
  «startet» (`08` hat `starting`); die Zuordnung der fünf Zustände in `10` zu den sieben in `08` fehlt.

Der Aufnahmeautomat in `11` 3.6 und die Zustände in `08` 3.5 stimmen überein; die Verknüpfung mit dem Strom (Haltegrund
in «zurückgestellt» startet ihn, `11`:208) ist schlüssig.

**Korrektur.** 04 5.1 als vollständige Quelle führen (die drei Übergänge aus `06` und die fehlenden Zeilen aus
Abschnitt 4 hier aufnehmen), `06` 2.3 danach ableiten, in `10` 13 eine Zuordnungstabelle zu `08` ergänzen.

### RB-13 · mittel · Zieldauer der Liste bei veränderlichen Segmenten

**Stelle:** 05:536 (2 bis 10 s an Quell-Keyframes), 05:486-490 (Startregel drei Zieldauern), 04 14.5 (04:1599).

**Problem.** Nach RFC 8216 darf die Zieldauer einer Medienliste sich nicht ändern und kein Segment darf sie
(gerundet) überschreiten. Segmente an Quell-Keyframes schwanken; eine Bildgruppe über 10 s erzwingt ein Segment über
10 s, obwohl 05:536 «höchstens 10» sagt. Setzt der Server die Zieldauer auf 10 s, verschiebt die Startregel den Start
nativer Player auf 30 s vor dem Listenende; setzt er sie nach der gemessenen Bildgruppe, verletzt eine spätere längere
Bildgruppe die Regel. Welche Zieldauer gilt, steht nirgends.

**Korrektur.** Festlegen: Zieldauer = gerundete grösste Segmentdauer seit Beginn des Zeitabschnitts, eine Änderung
nur an einer Grenze eines Zeitabschnitts oder einer Ausgabe-Generation (neue Liste). Messpunkt in M-3 und M-5.

### RB-14 · mittel · Stillstand nach 10 s gegen HLS-Quellen mit langen Segmenten

**Stelle:** 05:840-841, 04:833, 04 14.1 (04:1541), 05:109 (Zieldauer der Quelle bis 60 s erlaubt).

**Problem.** Eine HLS-Quelle mit Segmenten von 10 s liefert stossweise: ein Segment in rund 1 s, danach rund 9 s
nichts. «Keine Daten 10 s» und «Liste rückt nicht vor» schlagen damit an der Grenze an; bei erlaubten Zieldauern bis
60 s immer. Ob der Empfänger die Daten zeitgerecht in den Ring gibt (das Radio hat dafür `paceAhead`), steht nicht in
05 2.7.

**Korrektur.** Schwelle für HLS-Quellen: grösserer Wert aus 10 s und 1,5 × Zieldauer der Quelle. Festhalten, ob der
Empfänger einen Schub sofort oder zeitgerecht in den Ring schreibt (Folgen für die Uhr-Überwachung und für RB-01).

### RB-15 · mittel · Überholte Einstellvorgänge und die leere Liste

**Stelle:** 04:836, 05:542, `06`:263-277, `06`:516-553, `08`:627-633, 04:811.

**Problem.**
1. Schaltet der Zuschauer weiter, während die Antwort auf ein früheres «einstellen» noch bis 3 s wartet, ist offen, was
   diese Antwort liefert (veraltetes Token, Fehler, sofortige Rückkehr). Die Drosselung (1 je Sekunde, Stoss 5) lässt
   fünf solcher Vorgänge gleichzeitig zu.
2. `06` 3.1 zeigt, dass der Client nach `200 starting` sofort die Liste holt; `08` sagt, er hole sie erst, wenn der
   Herzschlag `live` meldet. Wie hls.js, Media3 und Safari auf eine Live-Liste **ohne** Segment reagieren (Fehler,
   Wiederholung, Abbruch), ist nicht belegt und nicht gemessen.

**Korrektur.** Eine neue Einstell-Nummer beantwortet die wartende Antwort der alten sofort mit `409 live_superseded`
und gibt deren Reservierungen zurück. Der Client lädt eine Liste nur im Zustand `live` (Regel aus `08` gilt, `06` 3.1
anpassen). Leere Liste trotzdem in M-3 und M-4 prüfen.

### RB-16 · niedrig · Ring ohne gespeicherte Bitrate bleibt zu klein

**Stelle:** 04:83, 04 14.3 (04:1569), 05:328.

**Problem.** Ohne gespeicherte Bitrate wird für 8 Mbit/s reserviert (15,45 MB). Ein UHD-Sender mit 20 Mbit/s hält damit
rund 6 s. Neu reserviert wird nur bei einem neuen Zeitabschnitt; eine gemessene höhere Bitrate ist keiner.

**Korrektur.** Nach der ersten Probe neu reservieren, wenn die gemessene Bitrate die Reservierung um mehr als 20 %
übersteigt (gleiche Regel wie beim neuen Zeitabschnitt, einschliesslich «Startpuffer verkürzt»).

### RB-17 · niedrig · Hülle vor FFmpeg: Regel 3 hält, aber drei Bauregeln fehlen

**Stelle:** T-27 (04:103), 04:1021, 04:1027-1032; `server/.golangci.yml:29-35` (depguard `os/exec`), `:132-134`
(forbidigo nur `exec.Command`, `CommandContext`, `LookPath` aus `os/exec`).

**Befund zu Regel 3.** Die Hülle bricht Regel 3 nicht: `transcode` startet weiter über `exec.CommandContext`, der Code
der Hülle liegt in `transcode`, `cmd/slouchcat` ruft ihn nur auf. Der Zusatz zu Regel 3 ist als O-26 vorgesehen.

**Problem.** Für den Bau fehlen:
1. Der heutige Lint fängt `syscall.Exec`, `unix.Exec`, `syscall.ForkExec` und `os.StartProcess` nicht. Ohne neue Muster
   ist «nur in `internal/transcode`» nicht durchgesetzt.
2. Landlock und der Filter für Systemaufrufe gelten je Betriebssystem-Faden. Die Hülle muss die Go-Routine an ihren
   Faden binden (`runtime.LockOSThread`) und auf **diesem** Faden sperren und `execve` aufrufen, oder die Sperre für
   alle Fäden setzen. Sonst läuft FFmpeg ohne Sperre, obwohl die Hülle «gesetzt» meldet.
3. Der Unterbefehl muss vor jeder Initialisierung des Servers laufen (erste Anweisung in `main`), damit die Hülle weder
   Konfiguration noch `secret.key` lädt.

**Korrektur.** Drei Sätze in 10.8 und ein Test, der in einem Kindprozess prüft, dass FFmpeg nach dem Start der Hülle
`/config` nicht lesen kann.

### RB-18 · niedrig · Langlauf: Neustart nach 24 h und Token-Wechsel nach 11 h

**Stelle:** 04:656-666, 05:315-318, `16`:243, `08`:623, `08`:1411.

**Problem.** «Startet der Prozess an der nächsten Segmentgrenze neu … höchstens ein kurzer Sprung» ist ungemessen und
unvollständig: Der neue FFmpeg muss an einem **Quell**-Keyframe beginnen, nicht an einer Ausgabegrenze; ob der neue
Prozess vor dem Ende des alten startet (wie beim Qualitätswechsel, 04:643), steht nicht da; auf der Klasse «klein»
laufen dann kurz zwei Umwandlungen auf zwei Kernen. Der Token-Wechsel nach 11 h verlangt vom Client eine neue
Listenadresse, also `loadSource` oder `setMediaItem`, also eine sichtbare Unterbrechung.

**Korrektur.** Neustart: neuer Prozess ab dem nächsten Quell-Keyframe vor dem Ende des alten, Überlappung zulassen,
Diskontinuität in der Liste. Token: Erneuerung in denselben Augenblick legen (eine Unterbrechung je 24 h statt zwei)
oder die Listenadresse ohne Token und nur die Segmentadressen mit Token führen. Beides in M-9 und M-22 aufnehmen.

### RB-19 · niedrig · Änderungsprotokoll und Kopf nicht nachgeführt

**Stelle:** 04:8 («T-1 bis T-28»), 04:1404 (RA-25 «keine eigene Frage»), 04:1405 (RA-26 «ausserhalb des Live-Anteils»).

**Problem.** Siehe Abschnitt 2. Der Kopf nennt T-28 statt T-29; das Protokoll widerspricht 04:731-735 und F-14.

**Korrektur.** Drei Stellen nachführen.

### RB-20 · niedrig · Rückstau im Speicher von Go ohne Grenze

**Stelle:** 05:266-269, RA-13 Punkt 2.

**Problem.** Nur die Ringe liegen ausserhalb der Speicherverwaltung von Go. Rückstau, Zuführer und Ausgabestücke
(bis zum Rest des Budgets, bei 512 MB bis 388 MB) liegen im Heap; ohne `GOMEMLIMIT` darf der Heap bis etwa zum Doppelten
wachsen. Auf 4 GB mit Radio-tmpfs (bis 564 MB) ist das relevant.

**Korrektur.** `GOMEMLIMIT` aus Byte-Budget und Grundlast ableiten oder die Rückstaupuffer aus einem festen Vorrat
nehmen; in M-13 messen.

### RB-21 · niedrig · Wortwahl, die Zielwerte als belegt erscheinen lässt

**Stelle:** `06`:300 («Der Benutzer sieht: fast sofort Bild»), `06`:353-354 («Belegt: … Zielwerte Fall 1 und 2»).

**Problem.** «Belegt» meint in `06` «auf diese Entscheide gestützt», liest sich bei Zielwerten aber als Nachweis.
Leitplanken 5: keine Behauptung, ein Zielwert werde erreicht.

**Korrektur.** In `06` «Belegt» durch «Gestützt auf» ersetzen und bei Zielwerten «zu prüfen (M-1)» schreiben.
04 und 05 sind hier sauber (05:15-16, 05:576-577, 05:603).

## 4. Zustandsautomat des Eingangsstroms: Zustand × Ereignis

Geprüft gegen 04 5.1 bis 5.3, `06` 2.1 und 2.3. «–» heisst: in keinem Dokument geregelt. Nur die Zellen mit Lücke
oder Widerspruch stehen hier; alle anderen Kombinationen sind in `06` 2.3 mit Stelle belegt und stimmen mit 04 überein.

| Zustand | Ereignis | Geregelt? | Vorschlag |
|---|---|---|---|
| Verbindet | Mangel, nur dauerhaft oder Zeitplan halten | – (`06`:172 nennt nur Läuft, Wartet, Gestört) | zurückgestellt, Verbindungsversuch abbrechen |
| Verbindet, Wartet | Mangel, nur Vorladen, automatisch oder Prüfung halten | – (`06`:171 nur Läuft) | Ruht (Haltegrund endet), wie Rangfolge 1 bis 3 |
| Wartet, Gestört | neuer Haltegrund Zuschauer oder Aufnahme | – (RB-07) | ein Versuch sofort, höchstens einer je Minute |
| Zurückgestellt | Zulassung beim Wiederanlauf verweigert | – | bleibt zurückgestellt, Frist neu |
| Zurückgestellt | Quelle lehnt beim Wiederanlauf ab, oder ein gehaltener Strom derselben Quelle bricht danach ab | – (RB-04) | zurückgestellt mit verlängerter Leiter; Wiederanlauf als vorsorglich deuten |
| Zurückgestellt | Quelle gelöscht, Rangfolge der Quellen geändert | – | Haltegrund bleibt; Wiederanlauf mit der dann ranghöchsten Quelle; ohne Quelle Ruht |
| Läuft, Wartet | festes Ziel oder Ursprung entzogen, Quelle gelöscht | 04:1058 gegen T-29 widersprüchlich (RB-05) | Verbindung endet nach Bestätigung; Aufnahme Lücke, Ersatzquelle |
| Läuft | ein gehaltener Strom derselben Quelle bricht innert 10 s nach einer **nicht** vorsorglichen Verbindung ab | – (RB-03) | wie Gegenrichtung, der jüngere Abnehmer verliert |
| Läuft | Ersatzquelle aktiv, kein Abnehmer, 10 min um | nur in 04:434, nicht in `06` 2.3 | Zeile ergänzen: Verbindet zur Hauptquelle |
| (endend) | Beitritt eines Abnehmers während des Schliessens | Regel 04:392, kein Zustand | Zustand «endet» (höchstens 1 s) oder Zeile «Ruht mit wartendem Beitritt» |
| Karenz | Sender deaktiviert | – | Ruht sofort (für Neues unsichtbar, es kann niemand mehr beitreten) |
| Karenz | Neustart des Servers | implizit (alles flüchtig) | genügt |
| Verbindet | Startversuch einer Sitzung endet nach 15 s | 04 und `06`: Fehlercode; `08`: `reconnecting` | eine Lesart (RB-07); Abbruchfrist nach RB-02 |

Übereinstimmung der Folgedokumente: Die Sitzungszustände in `10` 13 und in `08` (`starting`, `live`, `shifted`,
`paused`, `reconnecting`) sind nicht aufeinander abgebildet (RB-12). Die Aufnahmezustände in `11` 3.6, `07` 3.9 und
`08` 3.5 stimmen überein (neun Werte, gleiche Namen). Die Zustände des Stroms in der Verwaltung (`08`:1250:
`connecting`, `running`, `reconnecting`, `disturbed`, `grace`, `deferred`) decken 04 5.1 ab.

## 5. Zahlen: Stimmigkeit von 04 Abschnitt 14 und Stichprobe

### 5.1 In sich

| Paar | Rechnung | Urteil |
|---|---|---|
| Ring 15 s gegen Bildgruppen 1 bis 10 s | bis rund 7 s eine vollständige Bildgruppe plus laufende; darüber nicht (05:375-384 sagt es selbst) | stimmig für den Anlauf, **nicht** für das Fenster von drei Segmenten (RB-01) |
| Segmentdauer 2 bis 10 s gegen «unter 1 s» | erstes Segment 2 s bei 8 Mbit/s = 2 MB: 0,16 s bei 100 Mbit/s, 0,53 s bei 30 Mbit/s; bei Bildgruppe 10 s: 10 MB, 0,8 s bzw. 2,7 s | stimmig, weil «Segment unter 2 MB» als Bedingung von «günstig» genannt ist (05:594-596); bei langen Bildgruppen ist Fall 1 auch auf empfangenen Sendern nicht erreichbar, das fehlt in 05:681-691 |
| Karenz 30 s gegen Herzschlag 10 s und Ende nach 60 s | nach einem Absturz des Clients hält der Strom 60 s als «Zuschauer», dann 30 s Karenz: 90 s | stimmig, Folge für Zugänge mit einer Verbindung fehlt (RB-11) |
| Herzschlag 10 s gegen Drosselung 1 je 2 s, Stoss 3 (`08`:1539) | 404 an der Liste löst sofort einen Herzschlag aus (`08`:1414); wiederholte Listenfehler von hls.js können die Drosselung treffen | knapp; Client muss die Auslöser bündeln |
| Token 12 h, neu ab 11 h, gegen Segmentabruf | Segmente prüfen Token und Einstell-Nummer, Recht nur an der Liste (höchstens 30 s alt); Token-Wechsel braucht neue Listenadresse | stimmig; Unterbrechung je 11 h (RB-18) |
| Aufwärmzeit 30 s gegen Verbindung und Keyframe | Verbindung bis 0,6 s, zwei Bildgruppen bis 20 s bei 10 s; Leiter bis zum fünften Versuch 18 s | stimmig mit 30 s; die Untergrenze 10 s trägt Bildgruppen über 4 s nicht. Bei der Untergrenze warnen |
| Rückstau 64 MiB oder 60 s | 8 Mbit/s: 60 MB; 20 Mbit/s: 67,1 MB = 26,8 s | stimmig (`13`:118) |
| Zehn HD-Aufnahmen | 512 − 10 × 15,45 = 357 MB; 35,7 MB = 36 s | stimmig (04:745-748, `13`:379), ohne die dauernd belegten Ausgabestücke (RB-08) |
| Byte-Budget, Ringe drei Viertel | 384 ÷ 15,45 = 24,9 | stimmig, 24 Ringe |
| Fall 1 ungünstig mit Media3 1000 ms | 50 + 5 + 10 + 500 + 1000 + 300 = 1865 ms; ohne Startpuffer 865 ms | stimmig (05:603) |
| Abbruch 15 s gegen Bildgruppe über 15 s | 2 × G + Verbindung > 30 s | **widersprüchlich** (RB-02) |
| Stillstand 10 s gegen Zieldauer bis 60 s | Lieferung stossweise je Zieldauer | **widersprüchlich** (RB-14) |

### 5.2 Stichprobe in 07, 08, 10, 11, 13, 16 (20 Werte)

| Nr. | Wert (04 Abschnitt 14) | Gefunden | Urteil |
|---|---|---|---|
| 1 | Ring 15 s, einstellbar 4 bis 15 | `07`:279, `08`:1216, `13`:113 | gleich (die Untergrenze selbst ist RB-01) |
| 2 | Ring 15,45 MB bei 8 Mbit/s | `05`:387, `13`:114 | gleich |
| 3 | Byte-Budget 512 MB, Ringe 384 MB, 24 Ringe | `13`:115-116, `13`:461 | gleich |
| 4 | Rückstau Aufnahme 64 MiB oder 60 s | `11`:413, `13`:118 | gleich |
| 5 | Rückstau Sitzungspuffer 16 MiB | `10`:102, `13`:119 | gleich |
| 6 | Zuführer und Ausgabestücke je 8 MiB | `13`:120, 04:286 | gleich |
| 7 | Karenz 30 s (15 bis 300), nur nach Gehen des Zuschauers | `07`:282-283, `08`:1214, `10`:68, `13`:444 | gleich |
| 8 | Herzschlag 10 s, Ende nach 60 s | `08`:563, `08`:615, `10`:62, `16`:244 | gleich |
| 9 | Token 12 h, neu ab 11 h | `08`:1411, `10`:68 | gleich |
| 10 | Aufwärmzeit 30 s (10 bis 120) vor dem Vorlauf | `11`:208, `06`:146 | gleich |
| 11 | Stück des Sitzungspuffers rund 60 s | `10`:96 | gleich |
| 12 | Höchstdauer Zeitversatz 60 min | `10`:132, `13`:469 | gleich |
| 13 | Deckel kleinerer Wert aus 20 % und 50 GB | `10`:453, `13`:349 | gleich (`13`:468 empfiehlt für «gross» 100 bis 200 GB als Einstellung, kein Widerspruch) |
| 14 | Mindestreserve Aufnahmen grösserer Wert aus 20 GB und 5 % | `11`:1097, `07`:353 | gleich |
| 15 | Vorladen 1 je Richtung, insgesamt 4 (0 bis 16) | `07`:274-275, `08`:1210-1211 | gleich |
| 16 | Einstellen 1 je Sekunde, Stoss 5 | `06`:518, `08`:1537 | gleich |
| 17 | Kosten Ton 0,25, Bild 2 und 1,5, Live-Anteil die Hälfte | `13`:153-157, `16`:281-285 | gleich |
| 18 | Sichern einer Aufnahme alle 10 s | `11`:415 | gleich |
| 19 | Antwort auf «einstellen» bis 3 s, Abbruch nach 15 s | `06`:263-271, `08`:629-632 | Zahlen gleich, **Folge nach dem Abbruch verschieden** (RB-07) |
| 20 | Fenster ohne Recht drei Segmente | `10`:171, `16`:236, `05`:535 | gleich, aber mit Ring und Segmentdauer nicht vereinbar (RB-01) |

Ergebnis: 18 von 20 Werten stimmen ohne Einschränkung; zwei stimmen in der Zahl, tragen aber einen Befund.

## 6. Zapping-Kette und Kapazität nach den Korrekturen

- **Kennzeichnung.** 05 sagt an drei Stellen ausdrücklich, dass kein Zielwert erreicht ist (05:15-16, 05:576-577,
  05:603 «Gemessen ist nichts davon»). Die Summen sind aus den Zeilen gerechnet, die Bedingungen der günstigen Summe
  sind genannt (05:594-596). Das erfüllt RA-20.
- **Android-Rechnung.** Mit 1000 ms Startpuffer stimmt die Summe (1,9 s ungünstig, 0,9 s ohne Wartezeit des
  Startpuffers). Die Aussage «der Startpuffer zählt in Sendezeit, kostet mit einem ersten Segment von mindestens 1 s
  nichts» ist schlüssig; sie gilt aber nur, wenn das erste Segment ganz geladen ist, bevor Media3 startet, und
  Media3 den Startpunkt aus `EXT-X-START` übernimmt statt aus seinem eigenen Live-Abstand (Annahme, M-4). Das sollte
  in 05:589 stehen.
- **Kapazität.** Ehrlich: Browser im Direktweg nur bei H.264 ohne Zeilensprung mit AAC (`16`:266-270), zwei
  Bild-Umwandlungen auf CT 190 (04:699-701), Software-Klasse genau eine (04:706-714). Es fehlt die Folge von
  eingebrannten Untertiteln (RB-06) und die Folge von RB-01 für Zuschauer ohne Recht bei langen Bildgruppen.
- **Satz, der einen Zielwert als erreicht darstellt:** in 04 und 05 keiner gefunden. In `06` die Wortwahl «Belegt: …
  Zielwerte Fall 1 und 2» und «fast sofort Bild» (RB-21).

## 7. Regeltreue und Bestand

| Regel | Urteil | Stelle |
|---|---|---|
| `os/exec` nur in `transcode` (Regel 3) | eingehalten; die Hülle liegt in `transcode`, `cmd` ruft nur auf; benannter Zusatz für `execve` vorgesehen (O-26). Lint und Fadenbindung fehlen (RB-17). | 04:1027-1032 |
| `paths` (Regel 4) | eingehalten: Schreiben relativ zu geprüften Wurzeln, Löschen Datei für Datei über `paths.Remove` (04:1083); Widerspruch zu RS-22 begründet und strenger. | 04:101, 04:1083, 04:1444 |
| Nähte der Dateiwiedergabe | «im Verhalten unverändert, belegt durch die bestehenden Tests» (04:89, 04:613, 04:801); neue Schnittstelle des Managers statt Eingriff in `Start` (04:727-730). Ehrlich formuliert. | T-13 |
| Budget-Teilung | Live hat eine **Obergrenze** (Hälfte), keinen Mindestanteil; Filme können alles belegen, Live wartet dann 10 s und meldet «ausgelastet». So beabsichtigt (kein Borgen, `13` P-7), aber in 7.6 nicht als Folge genannt: Bei zwei laufenden Filmumwandlungen auf CT 190 hat Live noch 4 Einheiten, bei vier keine. | 04:713-714, 04:753-754 |
| Herunterfahren in 10 s mit Aufnahmen | eigener Abschluss parallel zum Warten auf Anfragen, Ziel unter 3 s, `stop_grace_period: 30s` mit Stufe 5 (04:505-509). Rest: ein hängender Schreibaufruf (NFS hart) kann den Abschluss aufhalten; das ist offen benannt (`11`:429, M-2). | 04:499-509 |
| `itemsaccess`, `liveq`-Riegel, 404 statt 403 | eingehalten; 403 nur für den fehlenden CSRF-Kopf, begründet (04:899). | T-24, 04:883-902 |

## 8. Urteil je tragendem Entscheid

| Nr. | Urteil | Begründung, Befunde |
|---|---|---|
| T-1 | trägt | Muster Radio; Planer im Prozess (`11` 3.1) schlüssig. |
| T-2 | trägt | Ersatzquelle ohne zweite Verbindung. |
| T-3 | trägt | Weg W und L in eigenen späten Stufen, hinter Messpunkten. |
| T-4 | trägt | Kein Tunnel, Ersatzpfad, Sperre Pflicht; Restrisiko «jede Adresse auf dem Port» benannt. |
| T-5 | trägt mit Korrektur | Entzug bei gelöschter Quelle (RB-05). |
| T-6 | trägt | Zeitabschnitt eng gefasst; Diskontinuität ohne neuen Teil. |
| T-7 | trägt mit Korrektur | Ring als Auslieferungsfenster zu kurz, Einstellgrenze und Reservierung (RB-01, RB-16). |
| T-8 | trägt mit Korrektur | Rückstau ohne Vorrang, dauernd belegte Teile im gemeinsamen Rest (RB-08). |
| T-9 | trägt | Umschalten in der Sitzung; überholte Einstellvorgänge regeln (RB-15). |
| T-10 | trägt mit Korrektur | Ohne Recht im Server durchgesetzt; Fenster aus dem Ring trägt lange Segmente nicht (RB-01). |
| T-11 | trägt mit Korrektur | Zieldauer festlegen (RB-13); Apple ungeprüft, ehrlich benannt. |
| T-12 | trägt | Direktweg im Browser eng und ehrlich beschrieben. |
| T-13 | trägt | Reservierung je Sitzung; Langlauf-Neustart präzisieren (RB-18). |
| T-14 | trägt | eigene Fähigkeitsangabe für TS. |
| T-15 | trägt | Browser meldet nie, Vorgabe greift immer; Kapazitätsfolge ausgewiesen. |
| T-16 | trägt mit Korrektur | Schutz laufender Aufnahmen gegen gleichzeitige Stockung (RB-08). |
| T-17 | trägt mit Korrektur | Kern von RA-01 behoben; Pingpong, Wiederanlauf aus Ablehnung, verzögerte Freigabe, Fremdnutzung (RB-03, RB-04, RB-09, RB-10). |
| T-18 | trägt | ein Schalter je Quelle, Werkszustand als Frage. |
| T-19 | trägt mit Korrektur | wie T-8 (RB-08). |
| T-20 | trägt | |
| T-21 | trägt mit Korrektur | Hintergrundregel eindeutig machen, Pause ohne Recht (RB-11); Token-Wechsel (RB-18). |
| T-22 | trägt | |
| T-23 | trägt | |
| T-24 | trägt | |
| T-25 | trägt | |
| T-26 | trägt | |
| T-27 | trägt mit Korrektur | Lint für `execve`, Fadenbindung, Reihenfolge in `main` (RB-17). |
| T-28 | trägt mit Korrektur | Geheimnisse einer gelöschten Quelle bleiben in Gebrauch (RB-05). |
| T-29 | trägt mit Korrektur | auf Sender beschränken, Quelle und Ziele ausnehmen, Laufzeit begrenzen (RB-05). |

Kein Entscheid «trägt nicht».

## 9. Was wirklich offen bleibt, und ob der Entwurf bis Stufe V trägt

Offen sind (ohne die Fragen F-1 bis F-14, die sauber gestellt sind): die Apple-Clients insgesamt (kein Gerät, M-5),
das nahtlose Spulen mit Umwandlung (M-9, Rückfall benannt), die Wirksamkeit der Sperre auf CT 190 (M-25),
Untertitel im Browser (E-28-56), der Werkszustand (F-9). Dazu kommen die Befunde RB-01 bis RB-04, die nicht offen
gelassen, sondern übersehen sind.

**Messungen, die den Kern kippen könnten, und ihr Rückfall:**

| Messung | Was sie kippen kann | Benannter Rückfall | Urteil |
|---|---|---|---|
| V-2 / M-3: hls.js im schlanken Bau mit TS-Segmenten, kurzen Listen, Diskontinuitäten | T-11 und T-12 im Browser | HLS-fMP4 über einen FFmpeg je Sitzung (04:1099, 05:477) | Rückfall da, aber ohne Kostenrechnung: Dann braucht **jeder** Browser-Zuschauer einen Prozess, auch im Direktweg; Kosten je Prozess und Umschaltzeit fehlen in 7.6 und `13` 3.5. Nachtragen. |
| V-1 / M-2: Codecs, Zeilensprung, Keyframe-Kennzeichen, Bildgruppen deiner Quellen | Kapazität im Browser; schneller Start; mit RB-01 die Wiedergabe ohne Recht | «Strom normalisieren» (O-2), Ton-Umwandlung, Durchreichen des Zeilensprungs (O-23) | trägt, wenn M-2 zusätzlich die **Verteilung der Bildgruppenlängen** und Segmentdauern der HLS-Quellen ausweist (für RB-01, RB-02, RB-13). |
| Verhalten der Anbieter bei einer zweiten Verbindung (heute M-17, Stufe 3) | T-17 bei Zugängen mit einer Verbindung schon in Stufe 1 | F-3 (b) | **in Stufe V ziehen** (RB-03, RB-10): ablehnen, ältere trennen, verzögert freigeben. Ohne diese Messung ist die Regel «erst freigeben, dann verbinden» für Stufe 1 nicht belegt. |
| V-4 / M-25: Landlock und Seccomp auf CT 190 | T-27 (Pflicht für Weg W, empfohlen sonst) | Anzeige «Eindämmung nicht verfügbar», Weg W entfällt (F-13) | trägt; das Restrisiko ist eine Abwägung für Marco, kein technischer Rückfall. |
| M-9: Neustarts von FFmpeg mit TS-Zeitstempeln | Spulen mit Umwandlung (Stufe 4), Langlauf-Neustart | neue Ausgabe-Generation mit Diskontinuität (05:565-570) | trägt. |
| V-7 / M-14: Datenträger, Netzspeicher | Aufnahmen und Timeshift | Rückstau, F-8 (c) | trägt mit RB-08. |
| V-3 / M-5: Apple | T-11 für Apple | hls.js über die neuere Medienschnittstelle | nicht messbar; Clients nicht zugesagt. Ehrlich, aber Marco muss wissen, dass «Web-App auf dem iPhone» damit offen ist. |

Der Entwurf trägt bis zu den Messungen der Stufe V, wenn RB-01 bis RB-04 vorher im Text geregelt sind: Keine der
Messungen muss dann einen tragenden Entscheid umwerfen, jede hat einen benannten Rückfall, ausser dem Verhalten der
Anbieter, für das F-3 (b) der einzige Rückfall ist.

## 10. Gesamturteil

**Zur Vorlage beim Auftraggeber: geeignet nach Korrekturen.** Nötig vorher, alles Textarbeit ohne neuen tragenden
Entscheid:
- RB-01 (Ring und Auslieferungsfenster trennen), RB-02 (Abbruchfrist aus dem Zustand), RB-03 und RB-04 (Regeln für
  Anbieter, die die ältere Verbindung trennen, und für den Wiederanlauf), RB-05 (T-29 auf Sender beschränken),
  RB-06 (Untertitel-Aussage in 04 und `16` auf den Stand von `20`, Teletext als Abstrich mit Frage).
- Die mittleren Befunde RB-07 bis RB-15 können in derselben Runde in 04, 05, 06, 08, 10 erledigt werden; RB-12 ist
  die Klammer.

**Vor einer Baufreigabe zusätzlich nötig:**
1. Antworten auf F-3, F-4, F-7, F-9 und E-28-56 (für Stufe 1); F-8 vor Stufe 4, F-5 und F-6 vor Stufe 5.
2. Stufe V: V-1 (M-2 mit Verteilung der Bildgruppen und Quellsegmente), V-2 (M-3 mit den Prüfungen aus RB-01, RB-13,
   RB-15: drei Segmente aus dem Ring bei 2, 6 und 10 s, Zieldauer, leere Liste), V-4 (M-25 einschliesslich der
   Fadenbindung aus RB-17) und neu das Verhalten der Anbieter bei einer zweiten Verbindung (RB-03, RB-10).
3. Abnahmeprüfungen im Testplan ergänzen: Anbieter trennt die ältere Verbindung; Anbieter gibt verzögert frei;
   Zugang wird ausserhalb von Slouchcat genutzt; Quelle wird während einer Aufnahme gelöscht; Wiedergabe ohne Recht
   bei Bildgruppen von 10 s; kalter Start bei Bildgruppen von 8 s mit Umwandlung.
4. Kostenrechnung für den Rückfall «fMP4 je Sitzung» in `13` (Prozess je Browser-Zuschauer).
