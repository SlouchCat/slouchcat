# Konzept 28 · Umsetzungsplan

Stand: 28. November 2026 · Fassung 1 · Status: Entwurf, nicht freigegeben

> **Kurz gesagt.** Dieser Plan ist die Entwicklungsroadmap für Live-TV (Auftrag LTV-021, Abschnitt 18 Punkt 17).
> Er ist ein **Vorschlag an dich**; gebaut wird erst auf dein ausdrückliches Wort, und wann Live-TV im Projekt
> drankommt, entscheidest du. Vor dem Bau stehen eine **Stufe V** mit Fragen an dich und kleinen, wegwerfbaren
> Messversuchen und eine **Stufe 0** mit Regeln, Schutz der Zugangsdaten in den Protokollen, Härtung und einem
> Testsimulator für Quellen. Danach folgen sechs Stufen in der Reihenfolge der Fachpapiere: **1** Sender schauen
> (im Web), **2** Programmzeitschrift, **3** Empfang und Umschalten, **4** Timeshift, **5** Aufnahmen,
> **6** Serien und wiederkehrende Aufnahmen. Dazu kommen vier Stufen, die nur von Stufe 1 (oder 3) abhängen und
> an Messungen oder Entscheide gebunden sind: **H** Quellen im Heimnetz mit UDP, RTP und Multicast, **L** RTSP und
> RTMP, **W** DASH und HLS mit fMP4 (zuletzt), **E** Ergänzungen nach Messung (Untertitel im Browser, Erkennen
> eines stehenden Bildes). Wann die TV-App und die Handy-App folgen, ist eine Entscheidungsvorlage (Abschnitt 5).
> Jede Stufe ist für sich nutzbar, hat eine Abnahme und ist ein Release.

Dieses Papier ändert keine Zeile Code und keine Regel. Alles hier ist **spezifiziert** oder **Empfehlung**; nichts
ist gebaut. Massgeblich bleiben Spec, Entscheidungsprotokoll und ADR; bei Widerspruch gilt
Entscheidungsprotokoll > Plan > Spec (`CLAUDE.md`). Migrations- und Vertragsnummern vergibt der Bau (Anker:
Server 1.44.1, Vertrag 1.44.0, letzte Migration 00086, letzte ADR 0045, letzte Frage 36). Dieses Papier nennt
**keine Termine und keinen Aufwand in Tagen** und sagt nichts zu Kapazität oder Leistung zu: Zielwerte sind zu
prüfende Grössen.

## Lesart

| Kennung | Bedeutung | Wo sie steht |
|---|---|---|
| `LTV-001` … `LTV-021` | Grundsatzentscheide des Auftrags | `02-grundsatzentscheide.md` |
| `LTV-nnn.mm`, `LTV-Q-nn.mm`, `LTV-X-nn` | Anforderungen, Querschnitt, Ausschlüsse | `03-fachkonzept.md` |
| `AK-L-nnn` | Abnahmekriterien | `17-test-und-abnahme.md` |
| `T-n`, `M-n` | tragende technische Entscheide, Messpunkte | `04-architektur.md`, `05-streaming-und-fast-zapping.md` Abschnitt 10 |
| «`13` M-n», «`09` M-n», «`10` M-n», «`11` M-n» | Messpunkte anderer Dateien | jeweilige Datei |
| «`03` F-n», «`04` F-n» … | Fragen an dich, je Datei gezählt | jeweilige Datei; Sammlung in `20-offene-entscheidungen.md` |
| `V-n` | Messversuch vor der Baufreigabe (dieses Dokument) | Abschnitt 2.2 |
| `D-n` | Widerspruch in den Stufenangaben (dieses Dokument) | Abschnitt 4.1 |
| `U-n` | Umplanungspunkt (dieses Dokument) | Abschnitt 8.2 |
| `RP-n` | Risiko des Vorhabens (dieses Dokument) | Abschnitt 8.1 |
| `O-n` | offene Punkte dieses Dokuments | Abschnitt 11 |

**Baufreigabe** heisst wie in `05` 10.1: Ohne das genannte Ergebnis wird die Stufe nicht zum Bau freigegeben, und
der davon abhängige Entscheid gilt nicht als tragend bestätigt. **Abnahme** heisst: Die `AK-L` der Stufe sind
grün, die Messungen der Stufe liegen vor, und du hast die Prüfliste auf der Testinstanz durchgesehen (`17` 8).

---

## 1. Grundsätze

### 1.1 Erst Konzept, dann Bau auf dein Wort

- **Jetzt ist Konzeptphase** (LTV-021). Es entstehen nur die Papiere in `docs/planning/28-live-tv/` und die
  Mockups in `docs/planning/mockups/28-live-tv/`. Keine Migration, kein Code, keine Änderung an Regeln, Spec oder
  `docs/dev/CURRENT_STATUS.md` (Muster der Konzepte 16, 25, 26, 27).
- **Freigabe:** Mit deiner Freigabe kommt Frage 37 ins Entscheidungsprotokoll (Entwurf in `02` Abschnitt 5). Für
  jede Frage, die du dann noch nicht beantwortet hast, gilt die Empfehlung des Papiers, es sei denn, sie steht in
  Abschnitt 2.1 als «vor Stufe 0 nötig».
- **Bau:** beginnt erst mit einem eigenen Auftrag von dir. Ein Auftrag kann eine Stufe meinen oder mehrere; sagst
  du «100 % selbständig», heisst das alle genannten Stufen samt Merge und Tag in einem Zug.

### 1.2 Stufen statt [MVP], [1.1], [1.x], [Later]

Die Marken des Auftrags gibt es in diesem Repo nicht (`02` AB-5). Umfang wird Stufen zugeordnet; jede Stufe ist
ein Release. Damit du deine Begriffe wiederfindest:

| Marke im Auftrag | Bedeutung im Auftrag | Hier | Warum so |
|---|---|---|---|
| **[MVP]** | kleinster nutzbarer Umfang | **Stufe 0 und Stufe 1** (Sender schauen im Web, mit Rechten, Verwaltung, zentralem Empfang, Umwandlung je Zuschauer) | Ab Stufe 1 kann ein Haushalt Fernsehen schauen; alles Weitere baut auf ihrem Eingangsstrom, ihren Sitzungen und ihren Rechten auf. |
| **[1.1]** | nächster Ausbau | **Stufe 2** (Programmzeitschrift, Erinnerungen) und **Stufe 3** (Empfangsstrategien, Vorladen, Ersatzquellen, erweiterte Überwachung) | Programm und schnelles, stabiles Umschalten sind deine Schwerpunkte 2, 3 und 7 (Auftrag Abschnitt 21). |
| **[1.x]** | spätere Ausbauten | **Stufen 4, 5, 6** (Timeshift, Aufnahmen, Serien), **H, L, W, E**, die **App-Ausgaben** | Jede ist ein eigenes Release; H, L, W und E hängen an Messungen oder an deinen Entscheiden. |
| **[Later]** | später, nicht jetzt | **nichts aus den 21 Grundsatzentscheiden.** «Später» heisst hier nur: ausdrücklich nicht in Konzept 28 und nicht vorbereitet (Abschnitt 10.3) | Alle 21 Entscheide sind verbindlich und bekommen eine Stufe (Abschnitt 10.1). |

### 1.3 Jede Stufe ist nutzbar, geprüft und ein Release

- **Nutzbar:** Nach jeder Stufe kann ein Zuschauer oder der Administrator etwas Neues tun, das ohne die
  nächste Stufe vollständig ist. Keine Stufe liefert halbe Seiten oder Knöpfe ohne Wirkung.
- **Abnahme:** `AK-L` nach `17` 8.1 (kumulativ: jede Stufe schliesst die früheren ein), Messungen nach `17` 8.2,
  deine Prüfliste nach `17` 8.3 (für H, L, W, E und die Apps in diesem Dokument ergänzt).
- **Release:** Jede Stufe ab 1 bringt eine neue Vertragsfassung mit eigenem Tag (Regel «Vertrag = Tag») und ist
  ein **Minor-Release** des Servers. Die erste Fassung ist frühestens 1.45.0; welche Nummer es wird, hängt davon
  ab, was bis dahin sonst veröffentlicht ist. Bekommt eine App in der Stufe Live-TV, ist das zusätzlich eine
  App-Ausgabe mit eigenem Tag. Stufe 0 ändert den Vertrag nicht (Abschnitt 3.4).
- **Bündeln:** Du kannst Stufen zu einem Release zusammenlegen (wie bei Konzept 21 und 25). Das ändert die
  Reihenfolge des Baus nicht, nur wie oft veröffentlicht wird.

### 1.4 Die Priorität des Projekts bleibt deine

Dieser Plan legt nur die Reihenfolge **innerhalb** von Live-TV fest. Er schiebt keine laufende oder geplante
Arbeit nach hinten (Auftrag Abschnitt 18: «ändere die bisherige Entwicklungspriorität nicht eigenmächtig»). Ob
Live-TV vor, nach oder zwischen anderen Vorhaben gebaut wird, sagst du mit dem Bauauftrag. Ein Hinweis zur
Planung: Die Messversuche der Stufe V brauchen keinen Produktcode und können auch dann laufen, wenn der Bau noch
nicht beauftragt ist, sofern du sie freigibst.

### 1.5 Was dieser Plan am Schnitt der Fachpapiere ändert

Die Fachpapiere (`03` Lesart, `04`, `07` 11.2, `08` 2, `17` 8) ordnen den Umfang den Stufen 0 bis 6 zu und
führen W (Weg W), L (RTSP und RTMP) und «Apps» als späte Stufen. Dieser Plan übernimmt das und ändert drei Dinge
(Begründung in Abschnitt 4.2):

1. **Neue Stufe H «Quellen im Heimnetz»:** UDP, RTP, Multicast, die festen Ziele im Heimnetz und der Netzmodus
   «host» werden aus Stufe 3 herausgelöst. Abweichung von `03` (LTV-003.14 bis .16), `07` 11.2 und `08` 2.8.
2. **Neue Stufe E «Ergänzungen nach Messung»:** Untertitel als WebVTT im Browser (`04` O-5) und, falls nicht schon
   in Stufe 3 gebaut, die Stichprobe mit Dekodieren für stehende Bilder (`05` 9.4). Beides hat heute keine
   Stufe in `03` und `17`.
3. **Stufe 1 in zwei Releases 1a und 1b** als Empfehlung, weil sie fast die Hälfte aller Abnahmekriterien trägt
   (152 von 312 nach `17` 8.1).

Die Folgeänderungen in `03`, `07`, `08` und `17` sind O-1 dieses Dokuments; dieses Papier ändert die anderen
Dateien nicht.

---

## 2. Vor dem Bau: Stufe V

Stufe V baut nichts ins Produkt. Sie klärt, was vor Stufe 0 und vor der Baufreigabe der einzelnen Stufen
feststehen muss: deine Antworten auf einige Fragen und eine Reihe kleiner Messungen.

### 2.1 Fragen an dich vor Stufe 0

Die vollständige Liste mit Varianten, Folgen und Empfehlung steht in `20-offene-entscheidungen.md`. Hier nur, **bis
wann** eine Antwort nötig ist. Was nicht beantwortet ist, folgt der Empfehlung; bei den Fragen der ersten Gruppe
würde eine spätere andere Antwort aber Regeln, ADR-Text oder schon gebauten Code ändern.

| Bis wann | Frage | Worum es geht | Warum so früh |
|---|---|---|---|
| **vor Stufe 0** | `03` F-6 | Welche Clients in welcher Stufe (Abschnitt 5) | bestimmt, ob Regel 8 in Stufe 0 einen TV-Tastentest bekommt und ob `05` M-4 vor Stufe 1 gemessen werden muss |
| vor Stufe 0 | `04` F-12 | Härtung der Standard-Compose-Datei für alle | wird in Stufe 0 gebaut (Abschnitt 3) |
| vor Stufe 0 | `12` F-1 | Darf FFmpeg Live-TV umwandeln, wenn der Kernel keine Sperre erlaubt? | steht in ADR 0046 und entscheidet, ob Stufe 1 ohne Sperre überhaupt Bild in den Browser bringt |
| vor Stufe 0 | `12` F-2, `12` F-3 | `https` nach `http` immer abbrechen; Zertifikatsprüfung nie abschaltbar | Regel für jeden Abruf ab Stufe 1, gehört in ADR 0046 |
| vor Stufe 0 | `04` F-3 | Bleibt es bei «kein Verbindungslimit je Anbieter» (LTV-017)? | Eine Änderung wäre eine Änderung eines Grundsatzentscheids und gehört ins Protokoll, bevor Regeln geschrieben werden |
| vor Stufe 0 | `04` F-10 | Sieht das Hauptprofil Aufnahmen aus der Zeit vor einer Altersgrenze? | Wortlaut der Ausnahme A-6 in ADR 0046 |
| vor Stufe 1 | `03` F-1, `07` F-2 | Werkseinstellung der drei Rechte; wer bei der Einführung «Administrator» ist | Migration der Stufe 1 setzt die Werte für den Bestand |
| vor Stufe 1 | `04` F-2 | Startpuffer nur im Arbeitsspeicher (Abweichung von LTV-019.01, .04) oder wählbar | entscheidet D-3 und ob es zwei Bauformen des Rings gibt |
| vor Stufe 1 | `12` F-4 | Zugangsdaten der Quellen in einer «Sicherung ohne Geheimnisse» leeren | Sicherung ist ab Stufe 1 betroffen |
| vor Stufe 1 | `15` F-13, `15` F-14, `14` F-12 | eigener Verwaltungsbereich; neue Sender ab Werk aktiv (`07` W-6); Eintrag in der Navigation | Seiten der Stufe 1 |
| vor Stufe 1 | `17` O-8, O-11, O-3 | Schwellen des Benchmarks; gehört das iPhone zu Stufe 1 (es gibt keines, Entscheid 2); Grenze «Filmstart nicht unzulässig verzögert» | Abnahme der Stufe 1 |
| vor Stufe 1 | `13` P-7, P-8 | Anteil des Umwandlungsbudgets für Live; Vorrang des Empfangs gegen Filmumwandlungen auf dem Software-Backend | Zulassung in Stufe 1 |
| vor Stufe 2 | `03` F-3, F-5; `09` F-1 bis F-4; `14` F-2, F-3, F-6, F-8, F-11 | Meldewege der Erinnerungen, Programmsuche in der allgemeinen Suche, eigene Datei für Programmdaten, Namensgleiche zuordnen, EPG im Heimnetz, Bilder im Raster | |
| vor Stufe 3 | `04` F-4, F-9; `15` F-19; `13` P-1, P-3, P-5 | Vorrang des Zuschauers vor ruhendem Empfang; was ab Werk vorsorglich empfangen wird; Ausfallmeldungen | |
| vor Stufe H | `04` F-11 | Multicast über Netzmodus «host» oder eigenes Netz des Containers | Zusatzdateien und Ausnahme A-9 |
| vor Stufe 4 | `04` F-7, F-8; `10` F-1; `14` F-9; `13` P-4 | Rückspulgrenze, Speicherform des Puffers, Sofortaufnahme im Zeitversatz | |
| vor Stufe 5 | `03` F-2, F-4, F-7, F-9, F-10; `04` F-5, F-6; `07` F-1; `08` F-1; `11` F-1, F-2; `14` F-4, F-5, F-10; `15` F-17, F-18, F-20, F-21; `13` P-2, P-6, P-9 | Kontingent, Vorrang der Aufnahmen, Entzug, Update, Spuren ausserhalb des TS, Notfall-Löschen, Höchstzahl gleichzeitiger Aufnahmen | |
| vor Stufe 6 | `03` F-8 | Serienaufnahme ohne zuverlässige Folgenkennung | |
| vor Stufe L | `04` F-1 | RTSP und RTMP über neue Go-Bibliotheken oder FFmpeg | |
| vor Stufe W | `04` F-13 | DASH und HLS mit fMP4, falls Weg W nicht dicht zu belegen ist | |
| vor den App-Ausgaben | `14` F-7, `14` F-1 | Bedienung des TV-Players mit dem Steuerkreuz; Vorgabe der Schalter | |

### 2.2 Messversuche vor der Baufreigabe

Jeder Versuch ist **klein, wegwerfbar und ausserhalb des Produkts**: ein kurzes Programm oder Skript auf einem
eigenen Zweig, der nie gemergt wird, oder im Arbeitsordner der Session. Er liest keine Datenbank des Servers,
ändert keine Installation und schreibt nur Kennzahlen nach `docs/perf/28-live-tv.md` (keine Senderliste, keine
Adresse, keine Zugangsdaten, keine Bilder). Versuche an deinen echten Quellen laufen nur mit deiner Zustimmung und
auf deinen Rechnern. **Laufzeit** ist die Messdauer des Versuchs, keine Schätzung des Aufwands.

| Nr. | Frage | Methode | Laufzeit | Abbruchkriterium | Was das Ergebnis am Entwurf ändern kann | Vor |
|---|---|---|---|---|---|---|
| **V-1** (`05` M-2) | Wie sehen deine wirklichen Quellen aus: Codec, Zeilensprung, Ton (AAC, MP2, AC-3), Länge der Bildgruppe, Bitrate, Keyframe-Kennzeichen, Schub beim Verbinden, Verhalten bei einer zweiten Verbindung? | Wegwerf-Werkzeug in Go, das einen Strom 2 Minuten liest und nur die TS-Tabellen und Zeitstempel auswertet; Ausgabe als Zahlentabelle je Quelle ohne Namen | je Quelle wenige Minuten; dazu eine Stichprobe von bis zu 50 Sendern | Die Quellen sind nicht erreichbar oder das Lesen verlangt etwas, das der Auftrag ausschliesst (DRM) | Anteil des Direktwegs im Browser (T-12) und damit die Kapazität (`04` 7.6); Vorgabe der Zeilensprung-Entfernung (T-15); ob 15 s Startpuffer reichen (T-7); ob Stufe H früher kommen muss (lokale Quellen); ob `12` F-3 (b) nötig ist | Baufreigabe Stufe 1 |
| **V-2** (`05` M-3) | Spielt hls.js im schlanken Bau TS-Segmente mit H.264, HEVC, AAC, MP2, AC-3, Diskontinuitäten und einem gesetzten Startpunkt sauber ab? | statische Testseite mit hls.js und mit FFmpeg selbst erzeugten Testströmen (Testbild, Ton), Chrome und Firefox | ein Messlauf je Browser und Format | hls.js spielt TS mit Diskontinuität oder Startpunkt nicht verlässlich | T-11: bleibt TS, oder der Segmentierer liefert fMP4 (die eine vorgesehene Stelle, T-11); Tonregel (MP2/AC-3 immer umwandeln) | Baufreigabe Stufe 1 |
| **V-3** (`05` M-5) | Safari und iPhone mit HLS-TS: Zeit bis zum Bild bei Listen mit 1, 2, 3 Segmenten | nur mit Gerät; ohne Gerät WebKit in Playwright als Näherung | ein Messlauf | kein Gerät vorhanden (Entscheid 2): dann gilt das iPhone für Stufe 1 als **ungeprüft** (`17` O-11) | kurze Segmente in der Startphase, hls.js auf dem iPhone als Ausweg (`05` 4.3) | Baufreigabe Stufe 1, nur wenn das iPhone dazugehört |
| **V-4** (`05` M-25) | Lässt CT 190 (Kernel und Docker-Profil) die Sperre für FFmpeg zu (Landlock für Dateien und Netz)? Läuft die ganze Prüfreihe mit `no-new-privileges`, entzogenen Fähigkeiten und Prozessgrenze? Verträgt QSV eine Grenze des Adressraums? | Wegwerf-Programm im Container, das die Sperre setzt und FFmpeg auf einer Pipe startet; ein Testlauf der bestehenden Prüfreihe mit gehärteter Compose-Datei | ein Durchlauf je Einstellung | Sperre nicht verfügbar | `12` F-1 wird zur echten Weiche; Weg W kann ohne Sperre nicht gebaut werden (`04` F-13 (a)); Stufe 0 härtet nur, was den Testlauf besteht | Baufreigabe Stufe 1 (und Stufe 0 für die Härtung) |
| **V-5** (`05` M-4) | Media3 mit HLS-TS auf Pixel und Android-TV-Gerät: Ladepuffer, Start mit und ohne übergeordnete Liste, Decoder beim Umschalten | Wegwerf-App mit eigenem Paketnamen (Muster K27 Stufe 0), dieselben Testströme wie V-2 | ein Messlauf je Gerät | Media3 startet TS-Listen nicht verlässlich | Einstellungen des Players in der App; ob die Apps die übergeordnete Liste brauchen | Baufreigabe der Stufe, in der die Apps Live-TV bekommen (Abschnitt 5) |
| **V-6** (`05` M-16) | UDP und Multicast im Container auf CT 190: Bridge mit veröffentlichtem Port, Netzmodus «host», eigenes Netz (`macvlan`, `ipvlan`) | ein FFmpeg sendet Testbild an Port oder Gruppe, ein Wegwerf-Empfänger im Container zählt Pakete | je Netzart ein Lauf unter Last | Multicast kommt in keiner Netzart an | `04` F-11; ob Multicast nur über «host» geht (Ausnahme A-9); im schlimmsten Fall ein Abstrich an LTV-003.16, den nur du entscheiden kannst (U-8) | Baufreigabe Stufe H |
| **V-7** (`05` M-14, `10` M-1, `11` M-1, M-2) | Schreiblast je Zuschauer und Aufnahme auf SSD, HDD, RAM-Disk und NAS; was passiert, wenn das NAS 30 s weg ist oder abgehängt wird; `fdatasync` und `fallocate` im Container | Wegwerf-Skript schreibt Stückdateien mit 1 MB/s je Strom für 5 und 20 Ströme; trennt das NAS; prüft, ob je ins leere Verzeichnis geschrieben wird | je Speicherart ein Lauf von rund einer Stunde | Schreiben landet bei getrenntem NAS auf der lokalen Platte, oder ein hängender Aufruf hält das Herunterfahren auf | `04` F-8 (c) als Bremse; ob ein NAS als Speicherort «Aufnahmen» nur mit Warnung erlaubt ist; Grösse des Rückstaus (T-8); `13` O-2 | Baufreigabe Stufen 4 und 5 |
| **V-8** (`13` M-1) | Läuft der Go-Empfänger 72 Stunden ohne wachsenden Speicher, Goroutinen und offene Dateien? | **kein Wegwerf-Code**, sondern der Empfänger aus Stufe 1, gehalten durch einen Haltegrund nur im Testbau, 20 Testsender des Simulators, keine Zuschauer, auf CT 190 | 72 Stunden; kann laufen, während Stufe 2 gebaut wird | Leck nach `17` 7 (Speicher > 10 %, Goroutinen oder Deskriptoren > 5 nach einer Stunde) | Stufe 3 wird nicht freigegeben, bis der Fehler in einem Patch von Stufe 1 behoben ist (U-4) | Baufreigabe Stufe 3 |
| **V-9** (`09` M-10 bis M-12) | Wie gut sind deine echten Programmdaten (fehlende Endzeiten, Folgenkennung, Zeitversatz) und wie stark schwankt die Menge von Tag zu Tag? | Wegwerf-Skript liest deine XMLTV-Dateien einmal täglich und schreibt nur Zählwerte | zwei Wochen | keine verwendbare Datei | Schwelle «Liste geschrumpft» (50 %); Vorgaben der Zuordnung; Grundlage für `03` F-8 (Serien ohne Folgenkennung) | Bau Stufe 2 (früh beginnen, läuft nebenher) |
| **V-10** (`05` M-10, M-11, M-12 FFmpeg) | Ist Weg W dicht: Öffnet FFmpeg mit umgeschriebenen Listen nur die Verbindung zum Weiterleiter? Hält ein FFmpeg-Leser für DASH 30 Stunden? | feindliche Testmenge (`05` 2.6) im Testaufbau, Mitschrift aller Verbindungsversuche des Prozesses | 30 Stunden für den Langlauf | eine einzige Verbindung an einem anderen Ziel | ohne Beleg kein Weg W (T-4); dann `04` F-13 | Baufreigabe Stufe W |
| **V-11** (`05` M-23) | Verbinden die Go-Bibliotheken für RTSP und RTMP nur über den übergebenen Dialer? | Test mit einem Dialer, der alles ablehnt, gegen einen Testserver | ein Lauf | eine Verbindung am Dialer vorbei | `04` F-1 (b) oder (c) | Baufreigabe Stufe L |
| **V-12** (`11` M-7) | Hat das Image Zonendaten, und rechnen wiederkehrende Aufnahmen unter `TZ` gleich UTC (CT 190) richtig? | Abfrage im Image mit gestellter Uhr über eine Zeitumstellung | ein Lauf | Zonendaten fehlen | Zonendaten ins Image oder in den Server einbetten | Bau Stufe 6 |

Was **nicht** in Stufe V gehört, sondern während des Baus vor dem betroffenen Bauteil gemessen wird (`17` 8.2):
`05` M-6, M-7, M-8, M-22, `13` M-5 (Stufe 1); `09` M-1 bis M-7, M-9 (Stufe 2); `05` M-13, M-15, M-17, M-20
(Stufe 3); `05` M-9, M-19, `10` M-2, M-3 (Stufe 4); `11` M-5 (Stufe 5); `05` M-24 (Apps, Stufe E).

---

## 3. Stufe 0: Regeln und Grundlagen

**Ziel in einem Satz für dich:** Bevor eine Zeile Live-TV entsteht, stehen die Ausnahmen mit ihren Grenzen in
Regeln, Spec und ADR, die Protokolle schwärzen Zugangsdaten von Fernsehquellen, und es gibt einen Testsimulator,
der Quellen und ihre Störungen nachstellt.

### 3.1 Planschritte (je ein Commit)

| Schritt | Commit-Titel (Vorschlag) | Inhalt | Dateien |
|---|---|---|---|
| 0.1 | `docs(planning): record decision 37, concept 28 approved` (falls nicht schon mit der Freigabe) und `docs(planning): question 37 addendum, answers to the open questions` | Frage 37 nach dem Entwurf in `02` Abschnitt 5; Nachtrag mit deinen Antworten aus Abschnitt 2.1 | `docs/planning/04-entscheidungen.md`, Konzept-README |
| 0.2 | `docs(rules): name live TV as an exception to the non-goals` | Spec «Non-Goals» (`docs/spec/01-anforderungen-v1.md:29`, `:44`, `:56-57`) und neuer Kriterienblock **L-1 ff.** (Empfehlung aus A1 3.2: Block in der Spec); Plan `docs/planning/03-plan-v1.md:30`; `docs/dev/HANDOVER.md:155`; `CLAUDE.md` **Regel 11** (Ausnahme A-1 mit Grenze: Fernsehsender, kein DRM, keine Plattform-Streams, Webradio ohne Aufnahmen), **Regel 10** (neue Schlüssel und Muster aus `12` 5.4), **Regel 3** (Hülle für die Sperre: `syscall.Exec` nur im Paket `transcode`, `12` O-1), **Regel 8** (TV-Tastentest mit Steuerkreuz, falls `03` F-6 die TV-App in Stufen 1 bis 6 nimmt; `01` RI-7), Regel 2 um `store/liveq` (nur im Paket `live`) | `docs/spec/`, `docs/planning/03-plan-v1.md`, `CLAUDE.md`, `docs/dev/HANDOVER.md` |
| 0.3 | `docs(adr): 0046 live TV exceptions, rights and limits` | ADR 0046 «angenommen» mit allen benannten Ausnahmen **A-1 bis A-9** und ihren Grenzen (`02` Abschnitt 4, `12` 1.1; Beleg `AK-L-272`), dem Rechte-Modell (drei Schalter, Deckel, keine Altersprüfung) und den Antworten auf `12` F-1 bis F-3, `04` F-3, `04` F-10. Nachträge: ADR 0025 (Regel «FFmpeg sieht keine Adresse» gilt für Live-TV; Weg W als einzige, noch nicht gebaute Ausnahme), ADR 0032 (A-4), ADR 0045 (A-5), ADR 0029 (eigene schreibbare Wurzeln, A-7) | `docs/adr/0046-…md`, Nachträge in `docs/adr/` |
| 0.4 | `feat(logging): redact live TV source secrets` | neue Abfrageparameter, Endungsregel, Pfadmuster (Benutzerteil, Xtream-Pfade, Ersatznamen), Schlüsselnamen und zusammengesetzte Werte nach `12` 5.4, mit Pflicht-Testfall und Gegentest | `server/internal/logging/` |
| 0.5 | `chore(lint): guard the live packages` | Paketgerüst `server/internal/live` (nur Paketkommentar, Zuschnitt nach `04` T-24); `depguard`-Riegel für `store/liveq` (Muster `wishq`), Riegel gegen `items`, `media_files`, `tracks` im Paket `live` (Analyzer `itemsaccess`), `forbidigo` für `syscall.Exec`/`unix.Exec` ausserhalb `transcode`; CI-Schritt, der das Release-Binary auf Test-Build-Tags prüft (`12` SM-26) | `server/.golangci.yml`, `server/internal/live/`, CI |
| 0.6 | `chore(deploy): harden the default compose file` | nach `04` F-12 (a) und dem Lauf aus V-4: `no-new-privileges`, `cap_drop: ALL`, `pids_limit`, höhere Grenze offener Dateien (`13` O-6) | `deploy/compose.yaml`, `docs/INSTALLATION.md`, `deploy/README.md` |
| 0.7 | `test(live): source simulator and generated test streams` | Simulator `internal/live/livetest` mit Build-Tag `livetest`, Programm `livesim`, `task testdata:live`, `task live:sim` (Namen Vorschlag, `17` O-1); Fähigkeiten nach `17` 2.1 für Stufe 1 (HTTP-TS, HLS-TS, Xtream, M3U-Korpora, Störungen, Zähler, «nur eine Verbindung», Weiterleitungen, Ziel-Sensor), je Fähigkeit ein Selbsttest; Teststrom-Dateien nur selbst erzeugt, nicht eingecheckt | `server/internal/live/livetest/`, `server/cmd/livesim/`, `testdata/Taskfile.yml`, `Taskfile.yml` |

Spätere Fähigkeiten des Simulators (UDP/RTP/Multicast, RTSP/RTMP über MediaMTX im Testaufbau, DASH und fMP4)
kommen mit der Stufe, die sie braucht (H, L, W), nicht hier: Regel 11 «nichts vorziehen».

### 3.2 Abnahme, Messungen, Doku

| Punkt | Inhalt |
|---|---|
| Abnahme | `AK-L-098`, `AK-L-246` (Schwärzung), `AK-L-272` (Ausnahmen in Regeln, Spec und ADR, Skript); `task check:fast` grün; Selbsttests des Simulators grün |
| Prüfliste für dich | (1) Du liest Regel 11 und ADR 0046 und findest jede Ausnahme mit ihrer Grenze. (2) Nach dem Update auf eine Fassung mit gehärteter Compose-Datei läuft dein Server wie vorher (Filme, Radio, Apps). |
| Messungen | V-4 (für die Härtung) |
| Doku | ADR 0046 und Nachträge; `docs/INSTALLATION.md` und `deploy/README.md` für die Compose-Härtung; `CURRENT_STATUS.md` erst, wenn Stufe 0 Code ins Produkt bringt (Schwärzung, Compose) |
| Verträglichkeit | keine Vertragsänderung, keine App betroffen |

### 3.3 Abhängigkeiten und Risiken

Abhängig von deiner Freigabe und den Antworten «vor Stufe 0» (Abschnitt 2.1). Risiko: Die Härtung der
Compose-Datei betrifft jede bestehende Installation; deshalb erst nach dem Testlauf aus V-4 und mit einem Satz im
Release-Text. Die Schwärzung schwärzt eher zu viel (etwa Parameter `u` und `p`); das ist gewollt (`12` 5.4).

### 3.4 Release und Grösse

Stufe 0 bringt Code ins Produkt (Schwärzung, Lint, Compose), aber keine sichtbare Funktion und keinen neuen
Vertrag. **Empfehlung:** kein eigenes Minor-Release; die Änderungen reisen mit dem Release der Stufe 1. Soll die
Härtung früher bei allen ankommen, ist ein Patch-Release möglich (Weg «Versionscommit auf den Fix-Zweig»). Grösse:
**klein bis mittel** (der Simulator ist der grösste Teil; vergleichbar mit dem Testserver des Webradios).

---

## 4. Stufen 1 bis 6 und die späten Stufen H, L, W, E

### 4.1 Widersprüche in den Stufenangaben der Fachpapiere

Beim Abgleich von `03`, `04`, `05`, `07`, `08`, `12`, `16` und `17` fielen diese Unstimmigkeiten auf. Dieser Plan
entscheidet sie als Vorschlag (letzte Spalte); die anderen Papiere werden in ihrer nächsten Fassung nachgeführt
(O-1).

| Nr. | Thema | Stellen | Befund | Dieser Plan |
|---|---|---|---|---|
| **D-1** | RTSP und RTMP | `03` LTV-003.12, .13: Stufe 3 · `04` T-3, F-1, `16` Matrix A, `17` 8.1: «späte Stufe L, vorbehaltlich F-1» | `03` setzt sie in Stufe 3, die Architektur in eine eigene späte Stufe, weil Go die Protokolle nicht spricht und erst ein Entscheid über neue Bibliotheken fallen muss | **Stufe L** |
| **D-2** | DASH (und HLS mit fMP4) | `03` LTV-003.10: Stufe 3 · `04` T-3 («Weg W gehört zu keiner der Stufen 1 bis 3»), `04` O-24, `12` SM-44, `16`, `17` 8.1: Stufe W | `04` O-24 nennt die nötige Änderung in `03` selbst | **Stufe W**, zuletzt |
| **D-3** | Speicherort des Startpuffers | `03` Lesart: Stufe 4 («samt Speicherorten für Startpuffer und Timeshift») · `03` LTV-019.01, .04 und LTV-007.12, .13: Stufe 3 · `07` 11.2: `live_config` Startpuffer in Stufe 3, `live_storage` in Stufe 4 · `08` 2.10: alle Speicherort-Operationen Stufe 4 | Kopftabelle und Zeilen von `03` widersprechen sich; die Wahl des Ordners braucht die Prüfungen und Marker von `live_storage`, die erst in Stufe 4 kommen | Mit `04` F-2 (b) (nur Arbeitsspeicher, Empfehlung) entfällt die Ortswahl; die Grösse ist eine Einstellung der Stufe 3. Mit `04` F-2 (a) kommt die Ortswahl für den Startpuffer in **Stufe 4** zusammen mit `live_storage` |
| **D-4** | Feste Ziele im Heimnetz | `07` 11.2 (`live_net_targets`), `08` 2.8 (`resolveLiveNetTarget`), `12` SM-41, `03` LTV-003.14 bis .16: Stufe 3 · `17` 8.1: `AK-L-230`, `AK-L-231` (Quelle mit festem Ziel) in Stufe 1 | `17` prüft in Stufe 1 etwas, das es erst später gibt | In Stufe 1 prüfen `AK-L-230`, `-231` nur die verbotene Hälfte (keine lokale Adresse ohne Ziel); die erlaubte Hälfte gehört zu **Stufe H** |
| **D-5** | Eingefrorener Strom | `03` LTV-011.04: Stufe 3 · `04` 9, `05` 9.4: Stillstand der Zeit in Stufe 3, **stehendes Bild** in gültigem Strom erst mit einer Stichprobe mit Dekodieren «als eigener, späterer Schritt nach Stufe 3» und nur nach `05` M-20 | `03` und `17` kennen diesen späteren Schritt nicht; LTV-011.04 wäre in Stufe 3 nur zur Hälfte erfüllt | Stichprobe als letzter Planschritt der Stufe 3, wenn M-20 vorliegt und trägt; sonst **Stufe E**. Bis dahin ist LTV-011.04 «teilweise» und so in `CURRENT_STATUS.md` zu nennen |
| **D-6** | Untertitel je Zuschauer | `03` LTV-004.17: Stufe 1 · `04` 7.4, `16` Matrix E: im Browser und auf dem iPhone «in den ersten Stufen» keine Untertitel; Teletext nach WebVTT ist `04` O-5, «spätere Stufe» | LTV-004.17 ist im Browser in Stufe 1 nicht erfüllbar | Stufe 1 liefert die Wahl für Clients, die DVB-Untertitel zeichnen (Media3, nach `05` M-24); WebVTT für den Browser in **Stufe E** |
| **D-7** | Messungen vor Stufe 1 | `04` T-11 und 12.1: «M-2, M-3, M-4 und M-5 vor der Baufreigabe der Stufe 1» ohne Bedingung · `05` 10.1, `16` 3.2, `17` 8.2: M-4 erst vor der Stufe, in der die Apps Live-TV bekommen; M-5 nur, wenn das iPhone zur Stufe gehört · Entscheid 2: kein iPhone vorhanden | `04` verlangt etwas, das mangels Gerät nicht geht | Es gilt `05`/`17`: M-2 und M-3 immer; M-4 nach Abschnitt 5; M-5 nur mit Gerät, sonst iPhone «ungeprüft» (`17` O-11) |
| **D-8** | 72-Stunden-Lauf | `13` M-1, `17` 7: Baufreigabe der Stufe 3 · Strategie «dauerhaft» wird erst in Stufe 3 gebaut | Vor dem Bau kann nur der Empfänger aus Stufe 1 gemessen werden | V-8 misst den Empfänger der Stufe 1 mit einem Haltegrund nur im Testbau; der 7-Tage-Lauf (`05` M-12) ist Abnahme der Stufe 3 |
| **D-9** | Compose-Härtung | `04` 10.6, 10.8: Stufe 1 (vorbehaltlich `04` F-12) | kein Widerspruch im Inhalt, nur im Zeitpunkt | **Stufe 0**, weil sie keine Funktion braucht und vor dem ersten fremden Datenstrom stehen soll |
| **D-10** | Clients in jeder Stufe | `03` Lesart: «jede Stufe gilt zuerst für die Web-App» · `08` 1.2, 7.1: `x-clients: [app, tv]` ab Stufe 1 · `17` 8.1: eigene Zeile «Apps» | verträglich: Der Vertrag ist den Apps voraus, damit eine spätere App keine Vertragsänderung braucht | so übernommen; die Reihenfolge der Apps entscheidet Abschnitt 5 |

Kein Widerspruch, aber erwähnenswert: `07` 11.2 legt in Stufe 2 die neuen Meldungsarten aller Stufen in
`notifications` und `deliveries` an (geschlossene Wertelisten), während `08` 1.3 den Vertrag Werte erst mit ihrem
Erzeuger beschreiben lässt (ADR 0009). Das ist bewusst so und keine Vorbereitung ausgeschlossener Funktionen.

### 4.2 Der Schnitt und seine Begründung

**Geprüfter Vorschlag aus dem Auftrag an dieses Dokument** (1 Quellen bis Player, 2 EPG mit Favoriten, 3
Timeshift, 4 DVR, 5 Serien, 6 Strategien und Überwachung, 7 Apps, 8 weitere Protokolle): Er bringt Aufnahmen
früher, verlangt aber, dass Aufnahmen über Stunden laufen, bevor es Ersatzquellen, erweiterte Überwachung und den
7-Tage-Lauf des Empfängers gibt. Ausserdem müssten `03`, `07`, `08` und `17` umnummeriert werden. Favoriten stehen
in `03`, `07` und `08` schon in Stufe 1 und brauchen keine Programmdaten.

**Empfohlen ist die Reihenfolge der Fachpapiere mit drei Änderungen** (Abschnitt 1.5). Gründe:

1. **Deine Schwerpunkte zuerst** (Auftrag Abschnitt 21): ein Eingang für viele (1), schnelles Umschalten (1 und 3),
   stabiler Dauerempfang (3), Umwandlung je Zuschauer (1), automatische Wiederherstellung (3), keine Lecks
   (1 und 3). Timeshift und Aufnahmen (5, 6 der Liste) bauen darauf auf.
2. **Risiko früh prüfen:** Die teuersten Annahmen (Direktweg, hls.js mit TS, Eindämmung von FFmpeg, Lecks beim
   Umschalten) zeigt Stufe 1; Dauerbetrieb und Ersatzquellen Stufe 3, bevor Daten auf Platte geschrieben werden.
3. **Aufnahmen brauchen Zuverlässigkeit:** Eine Aufnahme ohne Ersatzquelle und ohne geprüften Dauerbetrieb verliert
   bei der ersten Störung die Sendung. Deshalb 3 vor 5.
4. **4 vor 5 aus Technik:** Timeshift bringt Speicherorte (`live_storage`), Marker, Stückdateien mit Verzeichnis,
   Rückstau und das Löschen über `paths`. Aufnahmen nutzen dieselben Bausteine (`04` T-8, T-16, T-25).
5. **2 vor 3 aus Planung:** Der 72-Stunden-Lauf (V-8) auf dem Code der Stufe 1 kann auf CT 190 laufen, während
   Stufe 2 gebaut wird. Stufe 2 bringt sichtbaren Nutzen ohne neue Netzrisiken.
6. **Stufe H herausgelöst:** UDP, RTP, Multicast und Quellen im Heimnetz sind eine eigene Sicherheitsklasse (Ausnahmen
   A-3 und A-9, Netzmodus «host» mit vier benannten Folgen, `04` 10.6), lassen sich in der CI kaum prüfen (`17`
   R-5) und hängen nur an Stufe 1. Getrennt bleibt Stufe 3 kleiner, und du bekommst schnelles Umschalten ohne
   Lockerung des Netzschutzes. **Wenn V-1 zeigt, dass deine Quellen im Heimnetz liegen** (zum Beispiel ein
   eigener Zuspieler), kommt H direkt nach Stufe 1, sonst nach Stufe 3 oder später.
7. **L und W hinter Messungen:** RTSP/RTMP brauchen deinen Entscheid `04` F-1 und V-11; DASH und fMP4 brauchen
   die Sperre und V-10. Beide sind seltene Quellarten (Annahme, V-1 prüft es) und kommen zuletzt.
8. **Stufe 1 in zwei Releases (Empfehlung):** 1a «Schauen» und 1b «Senderverwaltung erweitert». Beide sind für
   sich nutzbar; der Schnitt halbiert den grössten Pull Request des Vorhabens.

### 4.3 Stufe 1: Sender schauen

**Ziel in einem Satz für dich:** Du legst eine Senderliste an, und jedes freigegebene Profil kann im Browser
fernsehen; jeder Sender wird nur einmal empfangen, und wer ein anderes Format braucht, bekommt seine eigene
Umwandlung.

| Punkt | Inhalt |
|---|---|
| **Umfang 1a «Schauen»** | Quellen: M3U/M3U8 als Datei und Adresse, Einzel-URL, Xtream-Live (LTV-003.01 bis .09, .11, .17 bis .21, .23); Zugangsdaten, Kopfzeilen, User-Agent verschlüsselt und nur an feste Ursprünge (T-28); nur öffentliche Ziele (D-4). Empfang **bei Bedarf** (LTV-002.03, .09, .10), ein Eingang je Sender, Verteiler mit Ring und Startpuffer von höchstens 15 s, Lebenszyklus über Haltegründe, Zulassung mit Serverdeckeln (LTV-004 ausser .06 und .32; T-1 bis T-3, T-6 bis T-9, T-11 bis T-14, T-17, T-19, T-21 bis T-24). Original, Umpacken, Ton- und Bild-Umwandlung je Zuschauer mit bestehender Hardware (LTV-004.08 bis .19), Zeilensprung-Entfernung (T-15), Eindämmung von FFmpeg und Fuzz-Ziele (T-27). Grundüberwachung (LTV-011.02, .03, .11, .12, .16, .17). Rechte: Schalter «Live-TV» auf Konto und Profil, 404 ohne Recht (LTV-012 ausser .04, .05; LTV-015 ausser Aufnahmen), Zählung zu Strömen, Bitrate, Höhe und Schauzeit (T-26). Import mit stabiler Identität und Erhalt der Pflege, Aktualisierung von Hand (LTV-018 ausser .12, .13). Favoriten und «zuletzt gesehen» (LTV-001.12). |
| **Umfang 1b «Senderverwaltung erweitert»** | Zusammenführen und Duplikate (LA-07), eigene Gruppen (LA-08), Massenbearbeitung und Nummern, Logo-Upload, automatische Aktualisierung nach Zeitplan (Jobart), Importbericht mit «zurückgehaltener Liste» (LTV-006, LTV-018 Rest) |
| **Tabellen** (`07` 11.2) | 1a: `live_config` (Spalten Stufe 1), `live_sources` (ohne Spalten der Stufen 3 und H), `live_source_headers`, `live_source_origins`, `live_source_imports`, `live_source_import_items`, `live_channels` (ohne `strategy`, `schedule_id`), `live_channel_locks`, `live_channel_categories`, `live_channel_streams`, `live_favorites`, `live_recent_channels`, `live_sessions` (ohne `recording_id`), `live_session_tunes`, `live_watch_time`, `live_stream_events`, Sicht `live_visible_channels`; Erweiterung `accounts`, `profiles`, `invitations` (Schalter «Live-TV», Werkseinstellung nach `03` F-1); Neubau `my_media` (Schlüssel `livetv`). 1b: `live_groups`, `live_group_channels`, `live_duplicate_dismissals`. Ein eingefrorener DB-Stand je Release in `testdata/db/` (Regel 5) |
| **Operationen** (`08` 2) | 63 neue: Sender und Favoriten (11), Sitzung und Wiedergabe ohne `seekLiveSession` (8, davon 3 Binärrouten), Übersicht, Ströme, Diagnose ohne `switchLiveIngestSource` und `getLiveUpdateImpact` (6), Quellen und Importe ohne `resolveLiveNetTarget` (16), Sender, Gruppen, Duplikate (20; die Gruppen- und Zusammenführ-Operationen in 1b), `getLiveConfig`/`updateLiveConfig` (2). Erweitert: `getSession`, Profile, Konten, Einladungen, `listMyMedia`, `listStreams`, `getDashboard`, `listJobRuns`, `getWatchReport`, `PlaybackCapabilities` |
| **Seiten** (`14`, `15`) | Zuschauer: LS-01 (ohne Programmteile), LS-02, LS-03, LS-07 (ohne Timeshift und Aufnahme), LS-20, LS-21, LS-25 (Ton-Vorrang mit Radio). Verwaltung: LA-01 bis LA-06, LA-12, LA-13 (Grundteil), in 1b LA-07, LA-08; geänderte Seiten LA-20 bis LA-23, LA-24 (Importfehler), LA-25, LA-26. Jede neue Seite mit Playwright-Tastaturtest (Regel 8) |
| **Clients** | Web-App (Desktop, Handy-Breite, Tablet). Apps nach Abschnitt 5 |
| **Abhängigkeiten** | Stufe 0; V-1, V-2, V-4 (und V-3 nur mit Gerät, V-5 nur bei Variante (b) oder (c) aus Abschnitt 5); Antworten «vor Stufe 1» |
| **Risiken** | Direktweg greift selten (RP-1); hls.js mit TS (RP-2); Grösse der Stufe (RP-3); FFmpeg ohne Sperre (RP-4); Wirkung auf Filme (RP-9) |
| **Abnahme** | 152 `AK-L` nach `17` 8.1 (Pflichtfälle, Verteilung, Umwandlung, Rechte, Sicherheit SM-01 bis SM-07, SM-13 bis SM-27, SM-35 bis SM-42 soweit Stufe 1, SM-44 für HLS; Schutz des Bestands `AK-L-282` bis `-291`); bei 1a/1b werden sie nach Inhalt geteilt (O-2) |
| **Prüfliste für dich** (`17` 8.3) | (1) Testliste anlegen, Sender erscheinen. (2) Ein Sender: Bild kommt, Ton passt. (3) Zweites Gerät, derselbe Sender: in der Verwaltung **ein** Eingang, **zwei** Zuschauer. (4) Schnell durch zehn Sender: nichts hängt, danach ein bis zwei Eingänge. (5) «Live-TV» einem Profil entziehen: Wiedergabe endet, Bereich weg, kein «verboten». (6) Ein Profil mit Altersgrenze sieht Live-TV nur nach deiner Freigabe. (7) Ein Film läuft ungestört, während Live-TV läuft. Zusätzlich in 1b: (8) zwei Listen mit demselben Sender zusammenführen; Favoriten bleiben nach einer Aktualisierung. |
| **Messungen** (`docs/perf/28-live-tv.md`) | während des Baus `05` M-6, M-7, M-8, M-22, `13` M-5; zur Abnahme `05` M-1 (Fälle 1, 3, 4 ohne Vorladen), `05` M-18 (hinter deinem Reverse-Proxy), `13` M-2 (1'000 Wechsel), `13` M-3 (20 Zuschauer), `13` M-6 (Wirkung auf Filme), `AK-L-308` |
| **Doku** | ADR 0047 (Empfang, Verteilung, Sitzungen, Auslieferung, Umwandlung: T-1 bis T-3, T-6 bis T-9, T-11 bis T-15, T-17, T-19 bis T-24, T-26), ADR 0048 (Geheimnisse der Quellen, Netzschutz nur öffentlich, Eindämmung von FFmpeg: T-27, T-28, T-5 Teil «öffentlich»); `CURRENT_STATUS.md`; `docs/INSTALLATION.md` (Live-TV einrichten, Sicherheitshinweise V-1 bis V-5 aus `12`); Texte im Namensraum `live` in `de`, `en`, `de-DE`, übrige Sprachen nach dem Übersetzungsweg von Konzept 22 |
| **Verträglichkeit** | neue Vertragsfassung (Minor) je Release 1a und 1b; alle Zuschauer-Operationen `scoped` mit `x-clients: [app, tv]`, Verwaltung `admin` ohne `x-clients`; nur additive Änderungen (`AK-L-286`); `livetv` in `listMyMedia` und Live-Einträge nur an Clients mit Live-TV-Fassung (`08` 7.2, O-1 dort); Stand neu einfrieren (`task server:freeze:x-clients`) |
| **Grösse** | **sehr gross**: grösser als jede bisherige Stufe eines Konzepts, etwa so viel wie ein ganzes früheres Konzept (Annahme der Redaktion); deshalb der Schnitt 1a/1b |

**Planschritte (Vorschlag, je ein Commit; Reihenfolge innerhalb 1a):**

| Schritt | Commit-Titel | Kern |
|---|---|---|
| 1.1 | `feat(store): live tables, rights and the visible channels view` | Migrationen, `server/queries/live/` → `store/liveq`, Werkseinstellung der Rechte |
| 1.2 | `feat(live): MPEG-TS reader, ring and keyframe index` | `live/mpegts`, `live/hub`, Fuzz-Ziele |
| 1.3 | `feat(live): HTTP-TS, HLS-TS and Xtream receivers behind the guarded dialer` | `live/ingest`, Verschärfung des Adressfilters in `netguard` (`04` 10.9), Weitergabe nur an feste Ursprünge |
| 1.4 | `feat(live): stream manager with hold reasons and admission` | T-17, T-19, Kopplung an die Grenzen (T-26), Abfangen von Programmfehlern an der Wurzel |
| 1.5 | `feat(transcode): live budget interface and contained ffmpeg` | reservieren, umbuchen, freigeben; Hülle mit Sperre; Prüfung jeder Argumentliste |
| 1.6 | `feat(live): live sessions, segmenter and the lt token` | Sitzungen, Einstellen, Herzschlag, Binärrouten |
| 1.7 | `feat(live): import with stable identity` | Erkennung Liste oder Manifest, alles oder nichts, Erhalt manueller Angaben |
| 1.8 | `feat(api): contract for live TV stage 1` | `api/openapi.yaml`, `task gen`, Vertragstest, `binaryRoutes` |
| 1.9 | `feat(admin): live TV administration` | LA-01 bis LA-06, LA-12, LA-13, Änderungen an LA-20 bis LA-26 |
| 1.10 | `feat(web): live TV area, channel list, favourites and player` | LS-01 bis LS-03, LS-07, LS-20, LS-21, LS-25, Tastaturtests, Storybook für neue Bauteile |
| 1.11 | `test(live): regression, leak and limit tests` | `AK-L-282` bis `-291`, `AK-L-302` |
| 1.12 | `docs: live TV stage 1 status, measurements and installation` | `CURRENT_STATUS.md`, `docs/perf/28-live-tv.md`, Installationsanleitung, eingefrorener DB-Stand |

### 4.4 Stufe 2: Programmzeitschrift

**Ziel in einem Satz für dich:** Zu jedem Sender siehst du, was läuft und was kommt, kannst im Programm blättern
und suchen und dich an eine Sendung erinnern lassen.

| Punkt | Inhalt |
|---|---|
| **Umfang** | LTV-005 ausser .18 und .26 (XMLTV als Datei und Adresse, komprimiert, mehrere Quellen, eine Zuordnung je Sender `03` DR-19, Zeitzonen, Sommerzeit, fehlerhafte Daten, nichts erfinden); LTV-001.04, .05, .13; Programmteile von LTV-014 (Startseite, Programmzeitschrift, Sendungsdetails, Senderprogramm); LTV-018.12 (Zuordnung bleibt bei Aktualisierung); LTV-015 (Programmdaten ohne Altersfilter) |
| **Tabellen** | `live_epg_sources`, `live_epg_imports`, `live_channel_epg`, `live_reminders`; eigene Datei `epg.db` (vom Server angelegt, nach `09` F-1); `live_config` EPG-Spalten; Neubau `notifications`, `deliveries` (`07` 11.2) |
| **Operationen** | 25 neue: Programm (6, davon 1 Binärroute), Erinnerungen (4), EPG-Verwaltung (15); `uploadLiveFile` für XMLTV; erweitert `listNotifications`, `getMyChannels`, `updateMyChannel`; `getLiveHome` mit Programmteilen |
| **Seiten** | LS-04, LS-05, LS-06, LS-13, LS-15, LS-01 (Programmteile), LS-22, LS-23, LS-24 (nur nach `03` F-5 / `14` F-3); LA-10, LA-11; Banner und Programm im Player (LS-07) |
| **Abhängigkeiten** | Stufe 1; V-9 läuft nebenher; Antworten «vor Stufe 2» |
| **Risiken** | Import blockiert die eine Schreibverbindung (RP-8); Pünktlichkeit der Erinnerung über Push (`09` M-8) |
| **Abnahme** | 31 neue `AK-L`, dazu 4 Teile (`17` 8.1); `AK-L-290` (Benachrichtigungen) |
| **Prüfliste für dich** | (1) Test-Programmdatei laden: Raster zur richtigen Uhrzeit. (2) Sender ohne Daten: «Keine Programmdaten», trotzdem schaubar. (3) Erinnerung kommt rechtzeitig. (4) Suche findet eine Sendung; eine kaputte Datei lässt den alten Stand stehen. |
| **Messungen** | während des Baus `09` M-1 bis M-7, M-9 (M-3 entscheidet `09` F-1 endgültig); zur Abnahme `09` M-3, M-4, M-8, `05` M-1 mit Programm, `AK-L-304` |
| **Doku** | ADR 0049 (Programmdaten in eigener Datei `epg.db`, nicht in der Sicherung, streamender Parser, eine Zuordnung je Sender); `CURRENT_STATUS.md`; Sicherung und Wiederherstellung in der Installationsanleitung (was `epg.db` ist und warum sie fehlt); Servertexte der neuen Meldungen in allen Sprachen der Kanäle |
| **Verträglichkeit** | neue Vertragsfassung; neue `NotificationKind`-Werte nur an Clients mit Live-TV-Fassung (`08` O-7) |
| **Grösse** | **gross** (Parser, eigene Datenbankdatei, Zuordnung, Raster) |

### 4.5 Stufe 3: Empfang und Umschalten

**Ziel in einem Satz für dich:** Wichtige Sender laufen dauernd oder nach Zeitplan, Nachbarsender sind
vorgeladen, und fällt eine Quelle aus, wechselt Slouchcat selbst auf die nächste und sagt dir Bescheid.

| Punkt | Inhalt |
|---|---|
| **Umfang** | LTV-002.01, .02, .04 bis .07, .12 (dauerhaft, automatisch, zeitgesteuert, Vorgabe und Ausnahme je Sender, Vorladen, Wiederanlauf nach Neustart); LTV-003.22, LTV-004.32, LTV-011.09 (Ersatzquellen); LTV-010 (Startpuffer, Vorladen, Benchmark; LTV-001.06); LTV-011 Rest (Takt der Prüfungen, Bitrate, Auflösung, Codecs, kein Ton, Meldungen entprellt über das Benachrichtigungssystem); LTV-007.12, .13 und LTV-013.02 (Startpuffer getrennt, kein Archiv); Stichprobe mit Dekodieren als letzter Schritt, wenn `05` M-20 trägt (D-5) |
| **Tabellen** | `live_reception_schedules`, `live_reception_windows`; `live_channels.strategy`, `.schedule_id`; `live_sources.allow_more_conns`; `live_config` Empfang, Startpuffer (Grösse), Überwachung; Sicht `live_visible_channels` neu anlegen (`07` O-6) |
| **Operationen** | 5 neue: `switchLiveIngestSource`, Zeitpläne (4); `updateLiveChannel` mit Empfang; `getLiveDiagnostics` mit den Teilen der Stufe 3 |
| **Seiten** | LA-09; LA-13 (erweitert); LA-24 (Ausfallmeldungen); LA-01 (Kosten des vorsorglichen Empfangs); LS-07 (Hinweis «Ersatzquelle») |
| **Abhängigkeiten** | Stufe 1 (technisch), Stufe 2 (Reihenfolge); **V-8 als Baufreigabe**; Antworten «vor Stufe 3» |
| **Risiken** | Anbieter mit einer erlaubten Verbindung stören Vorladen und Dauerempfang (RP-5); Lecks über Tage (RP-6); Fehlalarme der Überwachung (`05` M-15) |
| **Abnahme** | 37 neue `AK-L` und 10 Teile nach `17` 8.1, ohne die nach Stufe H umgehängten (`AK-L-043` bis `-045`, `-239`, `-240`, O-2); `AK-L-103` (24-Stunden-Lauf ohne Fehlalarm), `AK-L-305`, `AK-L-306` |
| **Prüfliste für dich** | (1) Ein «dauerhafter» Sender läuft ohne Zuschauer und kommt nach einem Neustart von selbst. (2) Ein zeitgesteuerter Sender startet im Fenster und endet danach. (3) Hauptquelle kappen: Zuschauer sieht kurz «Ersatzquelle» und schaut weiter. (4) Umschaltzeiten im Protokoll mit Median und Streuung: welche Zielwerte gehalten sind und welche nicht, mit Grund. (5) Der 7-Tage-Lauf zeigt keine wachsenden Werte. |
| **Messungen** | vor der Baufreigabe V-8; während des Baus `05` M-13, M-15, M-17, M-20; zur Abnahme `05` M-12 (7 Tage), `13` M-2 mit Vorladen, `13` M-7 (Überlast), `05` M-1 (Fall 2 und alle Quellarten), `13` M-6 erneut |
| **Doku** | ADR 0050 (Empfangsstrategien, Vorladen, Ersatzquellen, Überwachung, Werkszustand nach `04` F-9); `CURRENT_STATUS.md` mit dem Benchmark-Protokoll; Hinweis zu Anbieterzugängen mit einer Verbindung in der Installationsanleitung |
| **Verträglichkeit** | neue Vertragsfassung; additiv |
| **Grösse** | **gross** (vier Strategien, Vorladen, Überwachung, lange Messläufe auf CT 190) |

### 4.6 Stufe H: Quellen im Heimnetz, UDP, RTP und Multicast

**Ziel in einem Satz für dich:** Quellen in deinem Heimnetz und Ströme über UDP, RTP und Multicast lassen sich
nutzen, aber nur für Ziele, die du je Quelle ausdrücklich bestätigst.

| Punkt | Inhalt |
|---|---|
| **Umfang** | LTV-003.14 bis .16; feste Ziele je Quelle (A-3 präzisiert, T-5, `04` 10.9) auch für HTTP-, HLS- und Xtream-Quellen und für EPG-Adressen, falls `09` F-4 es erlaubt; Netzmodus «host» oder eigenes Netz nach `04` F-11 (A-9); Hinweis «vermutlich fehlt das Host-Netz» |
| **Tabellen** | `live_net_targets`; `live_sources.listen_interface` (aus Stufe 3 von `07` 11.2 hierher) |
| **Operationen** | 1 neue: `resolveLiveNetTarget`; `createLiveSource`/`updateLiveSource` mit Zielen |
| **Seiten** | LA-03 (Abschnitt «Ziele im Heimnetz»), LA-02 (Abzeichen), LA-01 (Zahl der Freigaben, Hinweis Netzmodus), Protokoll der Verwaltung |
| **Compose** | freiwillige Zusatzdateien `compose.live-udp.yaml` und `compose.live-hostnet.yaml` (Arbeitsnamen, `04` 10.6); keine Änderung an `compose.yaml` |
| **Abhängigkeiten** | Stufe 1; V-6 als Baufreigabe; `04` F-11, `09` F-4 |
| **Risiken** | Lockerung des Netzschutzes (RP-11); gefälschte UDP-Absender (`12` R-2); im Netzmodus «host» liegt das Loopback des Hosts offen (`12` R-5) |
| **Abnahme** | `AK-L-043`, `-044`, `-045`, `-239`, `-240`, der erlaubte Teil von `-230`, `-231`, `-270` (Ziele), `-281` (UDP-Ports) |
| **Prüfliste für dich** | (1) Eine Quelle im Heimnetz geht erst, nachdem du ihr Ziel bestätigt hast; die Verwaltung zeigt die aufgelöste Adresse. (2) Ein Sender derselben Liste mit einem anderen Ziel im Heimnetz wird nie verbunden. (3) Ein Multicast-Sender läuft mit der gewählten Zusatzdatei; ohne sie erscheint der Hinweis. |
| **Messungen** | V-6; zur Abnahme Paketverlust unter Last (`AK-L-044`) |
| **Doku** | ADR 0051 (Quellen im Heimnetz mit festen Zielen, Netzmodus «host» mit seinen vier Folgen); Installationsanleitung mit den Folgen im Wortlaut; `CURRENT_STATUS.md` |
| **Verträglichkeit** | neue Vertragsfassung; nur Verwaltung betroffen, kein `x-clients` |
| **Grösse** | **mittel** |

### 4.7 Stufe 4: Persönliches Timeshift

**Ziel in einem Satz für dich:** Wer das Recht «Timeshift» hat, kann anhalten, zurückspulen bis zu seinem eigenen
Einschalten und wieder zu «Live» springen; der Puffer gehört nur ihm und verschwindet beim Umschalten.

| Punkt | Inhalt |
|---|---|
| **Umfang** | LTV-007 (eigener Puffer je Sitzung, Rückspulgrenze nach `04` F-7, Speicherform nach `04` F-8, Höchstdauer, Löschen beim Umschalten, Beenden und Serverstart); LTV-012.05 (Schalter «Timeshift»); LTV-019 für den Speicherort «Timeshift» (Deckel, Reserve, kein stilles Ausweichen, Bereinigung); bei `04` F-2 (a) auch die Ortswahl des Startpuffers (D-3); Timeshift im Player (LTV-014) |
| **Tabellen** | `live_storage`; `live_config` Timeshift; Schalter «Timeshift» auf Konto, Profil, Einladung |
| **Operationen** | 7 neue: `seekLiveSession`, Speicher (6); erweitert Profile (`canLiveTimeshift`) |
| **Seiten** | LS-07 (Zeitleiste, Pause, Spulen, Rückfrage beim Umschalten nach `14` F-9), LS-21 (Schalter); LA-15, LA-16 (Speicherort «Timeshift»), LA-22 (Schalter) |
| **Abhängigkeiten** | Stufe 1 (technisch), Stufe 3 (Reihenfolge); V-7 als Baufreigabe; `04` F-7, F-8, `10` F-1 |
| **Risiken** | Schreiblast bei vielen Zuschauern desselben Senders (`13` P-4, RP-7); Platz neben dem Cache der Filme (`AK-L-293`) |
| **Abnahme** | 28 neue `AK-L` und 9 Teile (`17` 8.1); `AK-L-126`, `-129` |
| **Prüfliste für dich** | (1) 30 s pausieren, zurückspulen, zu «Live» springen. (2) Nicht vor das eigene Einschalten zurück. (3) Beim Umschalten ist der alte Puffer weg (Belegung sinkt). (4) Ein zweiter Zuschauer hat seinen eigenen Puffer. (5) Datenträger voll: Live läuft weiter, nur Zurückspulen fehlt, mit Hinweis. |
| **Messungen** | vor der Baufreigabe V-7 (`05` M-14, `10` M-1); während des Baus `05` M-9, M-19, `10` M-2, M-3; zur Abnahme `13` M-3 mit Timeshift, `05` M-3, M-4, M-5 im Fenster |
| **Doku** | ADR 0052 (Sitzungspuffer vor der Umwandlung, eigene Kopie oder Antwort auf `04` F-8, Speicherorte und Marker); `CURRENT_STATUS.md`; Speicherplanung in der Installationsanleitung (`13` 5) |
| **Verträglichkeit** | neue Vertragsfassung; `Profile.canLiveTimeshift` additiv |
| **Grösse** | **mittel** |

### 4.8 Stufe 5: Aufnahmen

**Ziel in einem Satz für dich:** Jedes Profil mit dem Recht «Aufnahmen» kann sofort, einmalig oder von Hand
aufnehmen, auch ohne angemeldet zu sein; die Aufnahmen sind privat, in Originalqualität und zählen zum
Kontingent des Kontos.

| Punkt | Inhalt |
|---|---|
| **Umfang** | LTV-008 ausser Serien und Wiederholungen (sofort, aus dem Programm, manuell, Vor- und Nachlauf, Konflikte, parallele Aufnahmen, Anpassung bei geänderten Zeiten, unvollständige Aufnahmen); LTV-005.18, .26; LTV-002.08, .11; LTV-004.06; LTV-009 (privat je Profil, Einsicht des Hauptprofils nach E-3 und A-6); LTV-012.04; LTV-018.13; LTV-019 für den Speicherort «Aufnahmen», Kontingent je Konto, Reserve, Ablehnung bei vollem Kontingent (E-4), automatisches Löschen nur nach Frist des Profils (A-4); LTV-020 (Originalqualität, alle Spuren); Seiten «Meine Aufnahmen» und «Geplante Aufnahmen» (LTV-014) |
| **Tabellen** | `live_recordings` (ohne Regelspalten), `live_recording_parts`, `live_recording_gaps`; `accounts.live_quota_bytes`, `invitations.live_quota_bytes`; `live_sessions.recording_id`; `live_config` Aufnahmen; Schalter «Aufnahmen» |
| **Operationen** | 14 neue: Aufnahmen (11), `getLiveUpdateImpact`, `deleteLiveOrphanRecordingFiles`, `getProfileLiveUsage`; erweitert `installUpdate` (`confirmLiveRecordings`), Konten (Kontingent), Profile (`canLiveRecord`), `createLiveSession` für Aufnahmen |
| **Seiten** | LS-08 bis LS-12, LS-14, Löschdialog in LS-21; Aufnahme aus LS-04, LS-06, LS-07; LA-14, LA-16 (Speicherort «Aufnahmen», Belegung je Konto ohne Titel), LA-21 (Kachel «Private Aufnahme»), LA-22 (Kontingent), LA-27 (Warnung vor dem Update) |
| **Compose** | `stop_grace_period: 30s` in `compose.yaml`; Zusatzdatei `compose.live.yaml` mit schreibbarem Volume für Aufnahmen (Muster `compose.podcasts.yaml`) |
| **Abhängigkeiten** | Stufen 2 und 4 (technisch), Stufe 3 (Reihenfolge, Ersatzquellen); V-7 (`11` M-1, M-2) und `13` M-4 als Baufreigabe; Antworten «vor Stufe 5» |
| **Risiken** | Lücken bei Netzspeicher (RP-7); Planer nicht pünktlich (`11` M-3); Byte-Budget reicht nicht für zehn HD-Aufnahmen (`13` H-1); Anbieter lehnt die zusätzliche Verbindung einer Aufnahme ab (RP-5) |
| **Abnahme** | 47 neue `AK-L` und 27 Teile (`17` 8.1); `AK-L-274` mit 30 s Stoppfrist, `AK-L-288` (Pause eines Jobs bricht keine Aufnahme ab) |
| **Prüfliste für dich** | (1) Aufnahme planen, abmelden: sie läuft trotzdem. (2) Alle Tonspuren und Untertitel, dieselbe Qualität wie die Quelle. (3) Ein anderes Profil sieht sie nicht; das Hauptprofil sieht nur die Aufnahmen der Profile mit Altersgrenze. (4) Kontingent voll: neue Aufnahme abgelehnt und gemeldet, nichts gelöscht. (5) Aufnahme-Datenträger aushängen: keine Datei am falschen Ort, Aufnahme zeigt eine Lücke. (6) Neustart mitten in einer Aufnahme: sie macht weiter, mit Lücke. |
| **Messungen** | vor der Baufreigabe V-7, `13` M-4; während des Baus `11` M-5; zur Abnahme `11` M-3, M-4, `13` M-6, M-7 |
| **Doku** | ADR 0053 (Aufnahmen: Abzweig am Verteiler, Originalqualität, Speicherort, Kontingent, Ablehnung bei vollem Kontingent, Löschen nur nach Frist des Profils, Einsicht des Hauptprofils); `CURRENT_STATUS.md`; Installationsanleitung (Volume, Stoppfrist, Sicherung: Aufnahmen sind nicht in der Sicherung, nur ihre Angaben); Servertexte «Aufnahme fertig», «fehlgeschlagen», «abgelehnt» in allen Sprachen der Kanäle |
| **Verträglichkeit** | neue Vertragsfassung; `getProfileLiveUsage` ist `account` mit `x-clients: [app, tv]`; `installUpdate` nur additiv |
| **Grösse** | **gross** (Planer, Schreiber, Teile und Lücken, Kontingent, Rechte des Hauptprofils, sechs Zuschauerseiten) |

### 4.9 Stufe 6: Serien und wiederkehrende Aufnahmen

**Ziel in einem Satz für dich:** Ein Profil kann eine ganze Serie oder einen festen Sendeplatz aufnehmen lassen,
ohne Doppel, und auf Wunsch nur die letzten N Folgen behalten.

| Punkt | Inhalt |
|---|---|
| **Umfang** | LTV-008 Serien und Wiederholungen, Folgenerkennung nur bei zuverlässiger Kennung (`03` F-8), «behalte die letzten N» (A-4); LTV-009 für Regeln; Seiten «Serien» und «Wiederkehrend» (LTV-014) |
| **Tabellen** | `live_recording_rules`, `live_rule_episodes`; `live_recordings.rule_id`, `.rule_key` mit Index |
| **Operationen** | 7 neue (Regeln) |
| **Seiten** | LS-11 (Reiter «Serien», «Wiederkehrend»), Dialoge LD-02, LD-03 |
| **Abhängigkeiten** | Stufen 2 und 5; V-12 |
| **Risiken** | Programmdaten ohne Folgenkennung (V-9); Zeitumstellung (`11` M-7) |
| **Abnahme** | `AK-L-133`, `-134`, `-178`, Teil von `-196` |
| **Prüfliste für dich** | (1) Eine Serienaufnahme plant nur Folgen mit sicherer Kennung und nimmt keine doppelt auf. (2) Eine wiederkehrende Aufnahme beginnt auch nach der Zeitumstellung zur gleichen Uhrzeit. |
| **Messungen** | während des Baus V-12 / `11` M-7 |
| **Doku** | Nachtrag zu ADR 0053; `CURRENT_STATUS.md` |
| **Verträglichkeit** | neue Vertragsfassung; additiv |
| **Grösse** | **mittel** |

### 4.10 Stufe L: RTSP und RTMP

**Ziel in einem Satz für dich:** Quellen, die nur RTSP oder RTMP sprechen, lassen sich nutzen, ohne dass FFmpeg
je eine Adresse oder ein Passwort sieht.

| Punkt | Inhalt |
|---|---|
| **Umfang** | LTV-003.12, .13 nach `04` F-1 (Empfehlung (a): Go-Bibliotheken, RTSP nur über TCP, `rtmpe` abgelehnt) |
| **Tabellen, Operationen, Seiten** | keine neuen Tabellen und Operationen (Quellart als neuer Wert); LA-03 mit den neuen Quellarten |
| **Abhängigkeiten** | Stufe 1; `04` F-1; V-11 als Baufreigabe; im Heimnetz zusätzlich Stufe H |
| **Risiken** | drei neue Abhängigkeiten samt ihren eigenen (`04` F-1 (a)); Bibliothek verbindet am Dialer vorbei (V-11) |
| **Abnahme** | `AK-L-042`, `-241` |
| **Prüfliste für dich** | (1) Ein RTSP-Testsender und ein RTMP-Testsender laufen wie jeder andere Sender. (2) Ein RTSP-Server, der UDP verlangt, wird mit verständlicher Meldung abgelehnt. |
| **Doku** | ADR 0054 (RTSP und RTMP, Entscheid zu `04` F-1); `CURRENT_STATUS.md` |
| **Verträglichkeit** | neue Vertragsfassung (neuer Aufzählungswert der Quellart, nur Verwaltung) |
| **Grösse** | **mittel** |

### 4.11 Stufe W: DASH und HLS mit fMP4 (zuletzt)

**Ziel in einem Satz für dich:** Auch Quellen im Format DASH oder HLS mit fMP4-Segmenten laufen, über einen
eng eingesperrten FFmpeg, der nur mit dem Weiterleiter des Servers sprechen kann.

| Punkt | Inhalt |
|---|---|
| **Umfang** | LTV-003.10 (DASH ohne DRM, DASH mit Kopierschutz abgelehnt); HLS mit fMP4 oder getrennter Tonspur; Weg W nach T-4 und `05` 2.6 (Weiterleiter, Umschreiben aller Adressen, TLS nur in Go, kein Tunnel, Sperre Pflicht, gesperrt im Netzmodus «host»); Prozessklasse «Empfang» (T-20) |
| **Abhängigkeiten** | Stufe 1 (technisch), zuletzt in der Reihenfolge; V-4 (Sperre verfügbar), V-10 als Baufreigabe; `04` F-13 |
| **Risiken** | Weg W nicht dicht (RP-12); FFmpeg-Empfänger nimmt Einheiten aus dem Teil der Filme (`13` H-5) |
| **Abnahme** | `AK-L-041`, `-237`, `-238` und 5 Teile (`17` 8.1); `AK-L-306` Teil W (30 Stunden) |
| **Prüfliste für dich** | (1) Ein DASH-Testsender läuft. (2) Die Verwaltung zeigt «FFmpeg-Sperre aktiv»; ohne Sperre steht «DASH nicht verfügbar». (3) Ein DASH-Strom mit Kopierschutz wird abgelehnt. |
| **Doku** | Nachtrag zu ADR 0025 (die benannte Ausnahme Weg W wird gebaut); ADR 0055 nur, wenn `04` F-13 (b) gewählt wird; `CURRENT_STATUS.md` |
| **Verträglichkeit** | neue Vertragsfassung (nur Verwaltung) |
| **Grösse** | **mittel bis gross** (Weiterleiter, Umschreiben, Sperre, feindliche Testmenge) |

### 4.12 Stufe E: Ergänzungen nach Messung

**Ziel in einem Satz für dich:** Untertitel erscheinen auch im Browser, und Slouchcat erkennt ein stehendes oder
schwarzes Bild in einem sonst laufenden Strom.

| Punkt | Inhalt |
|---|---|
| **Umfang** | Teletext- und DVB-Untertitel als WebVTT für Browser und iPhone (`04` O-5, D-6; rundet LTV-004.17 ab); Stichprobe mit Dekodieren aus dem Ring (`05` 9.4, D-5; rundet LTV-011.04 ab), falls nicht schon in Stufe 3 |
| **Abhängigkeiten** | Stufe 1 (Untertitel), Stufe 3 (Stichprobe); `05` M-20, M-24 |
| **Abnahme** | neue `AK-L` in `17` nachzutragen (O-2); Prüfliste: (1) Ein Testsender mit Teletext zeigt im Browser wählbare Untertitel. (2) Ein Testsender mit eingefrorenem Bild erscheint in der Diagnose als gestört. |
| **Doku** | Nachtrag zu ADR 0047 und ADR 0050; `CURRENT_STATUS.md` |
| **Verträglichkeit** | additive Felder (Untertitelspuren) |
| **Grösse** | **klein bis mittel** |

### 4.13 Stufenübersicht

| Stufe | LTV-Entscheide (Schwerpunkt) | Dokumente | Clients | Risiko | Grösse |
|---|---|---|---|---|---|
| V | LTV-021 | `05` 10, `13` 10, `17` 8.2, `20` | – | niedrig (ausserhalb des Produkts) | – |
| 0 | LTV-015 (Ausnahme dokumentiert), LTV-Q-10 | `02` 4, `12` 1.1, 5.4, `04` 10.6 | – | niedrig | klein bis mittel |
| 1 (1a, 1b) | 001, 002, 003, 004, 006, 010, 011, 012, 015, 016, 017, 018 | `04`, `05`, `07`, `08`, `12`, `13`, `14`, `15`, `16` | Web (Apps nach Abschnitt 5) | **hoch** (Grundannahmen) | sehr gross |
| 2 | 005, 014, 018 | `09`, `14`, `15` | Web | mittel | gross |
| 3 | 002, 003, 004, 007, 010, 011, 013, 019 | `04` 5, 6, 9, `05` 6, 8, 9, `06` | Web | mittel bis hoch (Langlauf) | gross |
| H | 003 | `04` 10.6, 10.9, `12` 1.1, `16` | Web (Verwaltung) | **hoch** (Netzschutz) | mittel |
| 4 | 007, 012, 014, 019 | `10`, `13` 5 | Web | mittel | mittel |
| 5 | 002, 004, 005, 008, 009, 012, 014, 015, 018, 019, 020 | `11`, `13` 5, `14`, `15` | Web | hoch (Datenverlust) | gross |
| 6 | 008, 009, 014 | `09` 7, `11` 8 | Web | mittel | mittel |
| L | 003 | `04` F-1, `05` 2.2 | Web (Verwaltung) | mittel | mittel |
| W | 003 | `04` T-4, `05` 2.6, `12` | Web (Verwaltung) | **hoch** (Ausnahme ADR 0025) | mittel bis gross |
| E | 004, 011 | `04` 7.4, `05` 9.4, `16` 6 | Web, iPhone | niedrig | klein bis mittel |
| Apps | 014 | `14`, `16`, `08` 7 | TV-App, Handy-App | mittel | je App gross |

---

## 5. Reihenfolge der Clients (Entscheidungsvorlage zu `03` F-6)

Der Vertrag nennt `x-clients: [app, tv]` für alle Zuschauer-Operationen ab Stufe 1 (`08` 7.1); die Frage ist nur,
wann die Apps die Seiten bekommen. Die Verwaltung bleibt in jeder Variante nur in der Web-App.

| Variante | Ablauf | Folgen | Was es vor Stufe 1 braucht |
|---|---|---|---|
| **(a) Web zuerst** | Stufen 1 bis 6 nur in der Web-App; danach eine gesammelte App-Stufe je App (TV, Handy) | jede Stufe am kleinsten und am schnellsten abnehmbar; der Fernseher bleibt lange ohne Live-TV (auf dem Fernseher nur über den Browser, falls vorhanden); der Vertrag wird ohne Rückmeldung aus den Apps entworfen, und ein späterer Bruch kostet nach `docs/dev/VERTRAEGLICHKEIT.md` mindestens 90 Tage Übergang, sobald eine App öffentlich ist | nichts zusätzlich |
| **(b) Web und TV zusammen** | jede Stufe in Web-App und TV-App; die Handy-App gesammelt danach | Fernsehen kommt früh dorthin, wo es meist geschaut wird; jede Stufe wird um die TV-Seiten grösser (Steuerkreuz, Fokus, Tastentest); der Vertrag wird in jeder Stufe von zwei Clients geprüft; M-4 und der TV-Tastentest in Regel 8 werden Pflicht | V-5 (Media3) als Baufreigabe der Stufe 1; Regel 8 in Stufe 0; `14` F-7 |
| **(c) alle drei zusammen** | jede Stufe in Web, TV und Handy | volle Abdeckung, aber jede Stufe am grössten; drei Oberflächen je Stufe abzunehmen | wie (b), dazu V-5 auf dem Pixel |

**Empfehlung: (b)**, wie in `03` F-6, mit einer Lockerung: Die TV-App-Ausgabe einer Stufe darf nach dem
Server-Release derselben Stufe folgen, mit eigenem Tag und **ohne** neuen Vertrag, weil der Vertrag die TV-Seiten
schon trägt. So hält die TV-App den Server nicht auf. Die Handy-App folgt gesammelt nach Stufe 5 oder 6; bis dahin
geht Live-TV auf dem Handy über die Web-App im Browser. Begründung: Live-TV gehört auf den Fernseher; die TV-App
ist gebaut und hat ein Steuerkreuz-Muster; die Verträglichkeitsregel ist bis zur ersten öffentlichen App-Version
nur Übung (`docs/dev/VERTRAEGLICHKEIT.md` «Ab wann sie gilt»), sodass Fehler im Vertrag jetzt noch billig sind.

App-Ausgaben in jeder Variante: Prüfung `AK-L-292` (bestehende App-Tests unverändert), `AK-L-296`, `-297`
(Geräte), Oberflächentest je Seite (Regel 8), Texte aus `web/src/locales` mit dem Namensraum `live` (in
`android/strings-namespaces.txt` aufnehmen), Protokolle nur über `com.slouchcat.core.api.log.Log` mit den
neuen Schlüsselendungen. Prüfliste je App-Ausgabe: (1) dieselbe Prüfliste wie die Stufe, am Gerät; (2) jede
Seite mit Fernbedienung oder Touch erreichbar, Zurück führt hinaus; (3) eine App ohne Live-TV-Fassung zeigt keinen
leeren Eintrag «Live-TV» (`08` 7.2).

---

## 6. Abhängigkeiten der Stufen

Durchgezogene Pfeile sind technische Abhängigkeiten, gestrichelte die empfohlene Reihenfolge oder eine
Messvorbedingung.

```mermaid
flowchart TD
  K["Konzeptphase (jetzt)"] --> F["Freigabe durch Marco, Frage 37"]
  F --> V["Stufe V: Fragen und Messversuche"]
  V --> S0["Stufe 0: Regeln und Grundlagen"]
  S0 --> S1a["Stufe 1a: Sender schauen"]
  S1a --> S1b["Stufe 1b: Senderverwaltung erweitert"]
  S1a --> S2["Stufe 2: Programmzeitschrift"]
  S1b -.-> S2
  S1a --> S3["Stufe 3: Empfang und Umschalten"]
  S2 -.-> S3
  S1a -.->|"V-8: 72 h auf dem Code der Stufe 1"| S3
  S1a --> S4["Stufe 4: Timeshift"]
  S3 -.-> S4
  S4 --> S5["Stufe 5: Aufnahmen"]
  S2 --> S5
  S3 -.->|"Ersatzquellen und 7-Tage-Lauf"| S5
  S5 --> S6["Stufe 6: Serien und wiederkehrend"]
  S2 --> S6
  S1a --> H["Stufe H: Heimnetz, UDP, RTP, Multicast"]
  S3 -.->|"ausser V-1 zeigt lokale Quellen"| H
  S1a --> L["Stufe L: RTSP und RTMP"]
  S1a --> W["Stufe W: DASH und HLS mit fMP4"]
  S6 -.->|"zuletzt"| W
  S1a --> E["Stufe E: Ergänzungen nach Messung"]
  S3 --> E
  S1a --> A["App-Ausgaben je Stufe (Abschnitt 5)"]
```

Die Übersichtstabelle Stufe × LTV-Entscheide, Dokumente, Clients und Risiko steht in Abschnitt 4.13.

---

## 7. Arbeitsschritte innerhalb einer Stufe

So läuft jede Stufe nach der Arbeitsweise dieses Repos (`CLAUDE.md` «Arbeitsweise», «Agenten und Prüfungen»;
`docs/dev/SETUP.md` «Teststufen»):

| Schritt | Was geschieht | Regel |
|---|---|---|
| 1. Auftrag | Du beauftragst eine oder mehrere Stufen; die Hauptsession prüft Zweig, Arbeitsbaum und Stand (`git status`, `.omc/state/`) | dein Wort |
| 2. Zweig | eigener Zweig nach Thema, etwa `claude/live-tv-sender`, `claude/live-tv-programm`, in einem eigenen Arbeitsbaum, ab dem aktuellen `main` | Zweige nach Thema benennen |
| 3. Planschritte | jeder Planschritt ein Commit, Titel englisch mit Bereich (`feat(live): …`, `feat(store): …`, `feat(web): …`, `docs(adr): …`); generierter Code nur über `task gen` | `CLAUDE.md` Regel 1, «Arbeitsweise» |
| 4. Teststufe 1 je Schritt | `task check:fast` und nur die betroffenen Tests (`go test ./internal/live -run …`, `task e2e:quick -- <spec>`, `task storybook:test` bei geänderten Bauteilen); schwere Läufe nur über die Task-Ziele mit Platz aus `scripts/heavy-slot.sh`; Belege (Befehl, Ausgabe, SHA) je Schritt | Teststufe 1 |
| 5. Agenten | höchstens drei Executor gleichzeitig; Verifier und Reviewer lesen die Belege und wiederholen keinen Lauf auf derselben SHA; höchstens drei Korrekturversuche je roter Prüfung, nie `.skip`, `.only`, `t.Skip` | «Agenten und Prüfungen» |
| 6. Messungen | Messläufe der Stufe auf CT 190 in einem angekündigten Fenster (lange Läufe blockieren die Testinstanz, `17` R-4); Ergebnisse nach `docs/perf/28-live-tv.md` | «Messwerte nach `docs/perf/`» |
| 7. Teststufe 2 vor dem PR | einmal auf dem fertigen Zweig `task ci` grün, Belege in den PR; `docs/dev/CURRENT_STATUS.md` nachgeführt (Abschnitt mit «Stand je Stufe», eigenen Entscheiden des Baus, «nur gegen Simulator geprüft», «offen»); eingefrorener DB-Stand in `testdata/db/` | Teststufe 2, Regel 5 |
| 8. Pull Request | ein PR je Release; Text mit Zusammenfassung, Belegen, Abweichungen vom Konzept («so gebaut»), offenen Punkten | – |
| 9. Teststufe 3 | GitHub CI prüft vollständig; gemergt wird nur mit grünem Gate `ci` | Teststufe 3 |
| 10. Merge | **nur auf dein Wort** (oder wenn dein Bauauftrag Merge und Tag ausdrücklich einschliesst) | Rolle der Hauptsession |
| 11. Release | Versionscommit (Server, Updater, `deploy/`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md`), Tag `v<Version>`, Release-Lauf grün; bei App-Ausgaben eigener Tag nach dem bestehenden Muster; neuer Vertragsstand für die Apps (`task server:freeze:x-clients`) | «Vertrag = Tag» |
| 12. Testinstanz | Du hebst CT 190 selbst per Updater auf die neue Version; danach prüfst du die Prüfliste der Stufe | Abnahme |
| 13. Abschluss | Befunde aus deiner Prüfliste als Patch auf einem Fix-Zweig (Versionscommit dort); Konzeptpapiere auf «so gebaut» nachgeführt; Zweig und Arbeitsbaum aufgeräumt | Muster K24, K25 |

---

## 8. Risiken des Vorhabens und Umplanungspunkte

### 8.1 Risiken

| Nr. | Risiko | Wahrscheinlichkeit / Wirkung | Gegenmassnahme | Zeigt sich in |
|---|---|---|---|---|
| RP-1 | Der Direktweg ohne FFmpeg greift bei deinen Sendern selten (Zeilensprung, MP2, MPEG-2) | eher wahrscheinlich / hoch: wenige gleichzeitige Zuschauer im Browser | V-1 vor der Baufreigabe; Ton-Umwandlung als billiger Weg; Kapazität in `04` 7.6 neu rechnen | V, 1 |
| RP-2 | hls.js oder Safari kommen mit TS-Segmenten, Diskontinuitäten oder der Startregel schlecht zurecht | möglich / hoch: langsames Umschalten, Abbrüche | V-2, V-3; Segmentierer kann auf fMP4 wechseln (T-11) | V, 1 |
| RP-3 | Stufe 1 ist so gross, dass Prüfung und Review leiden | wahrscheinlich / mittel | Schnitt 1a/1b; Planschritte klein; Review je Planschritt | 1 |
| RP-4 | Der Kernel erlaubt keine Sperre für FFmpeg | möglich / hoch für die Sicherheit: Schadcode in FFmpeg läse die Geheimnisse (`12` R-1) | V-4; `12` F-1; Anzeige «FFmpeg läuft ohne Sandbox»; Weg W ohne Sperre nicht gebaut | V, 1, W |
| RP-5 | Anbieter erlaubt nur eine oder wenige Verbindungen (Slouchcat kennt kein Limit, LTV-017) | wahrscheinlich bei IPTV-Zugängen / mittel: Störungen beim Vorladen, gescheiterte Aufnahmen | Freigabe vor jedem Verbindungsaufbau, Ablehnung als «vermutlich Limit» (T-17), Schalter je Quelle (T-18); `04` F-3 | 1, 3, 5 |
| RP-6 | Speicher, Goroutinen oder offene Dateien wachsen über Tage | möglich / hoch: Absturz nach Tagen | V-8 (72 h), `05` M-12 (7 Tage), `13` M-2 (1'000 Wechsel), Leck-Definition in `17` 7 | 1, 3 |
| RP-7 | Netzspeicher stockt oder fällt weg; Schreiblast vieler Puffer | möglich / hoch für Aufnahmen: Lücken, im schlimmsten Fall Schreiben am falschen Ort | V-7; kein stilles Ausweichen (T-25); Rückstau je Leser (T-8); `04` F-8 (c) als Bremse | 4, 5 |
| RP-8 | Der EPG-Import blockiert die eine Schreibverbindung von SQLite | möglich / mittel: stockende Wiedergabe und Verwaltung | eigene Datei `epg.db` (`09` F-1), Stapel, `09` M-2, M-3 | 2 |
| RP-9 | Live-TV verdrängt Filme (Budget, Rechenzeit, Platte) | möglich / mittel | Live-Anteil am Budget, Deckel und Reserve; `13` M-6; `13` P-7, P-8 | 1, 3, 4 |
| RP-10 | Zugangsdaten gelangen in Protokolle, Antworten, Sicherungen oder an fremde Hosts | möglich / hoch | Stufe 0 (Schwärzung), T-28 (ganze Adresse verschlüsselt, feste Ursprünge), `12` F-4; Sicherheitstests SM-14 bis SM-20 | 0, 1 |
| RP-11 | Heimnetz-Quellen und Netzmodus «host» öffnen den Server nach innen | möglich / hoch | eigene Stufe H, feste Ziele je Quelle, nie Loopback und Link-Local, Zusatzdateien statt Standard | H |
| RP-12 | Weg W lässt sich nicht als dicht belegen | möglich / mittel: DASH und fMP4 fehlen | V-10; `04` F-13; Stufe W ganz am Schluss | W |
| RP-13 | Lange Messläufe belegen CT 190, auf dem du selbst testest und Updates einspielst | wahrscheinlich / niedrig bis mittel | angekündigte Fenster, Zwischenstand je 24 h, Wiederaufnahme als neuer Lauf (`17` R-4) | 1, 3, W |
| RP-14 | Kein iPhone, kein Mac (Entscheid 2) | sicher / mittel: Safari ungeprüft | iPhone in Stufe 1 als «ungeprüft» oder ausserhalb der Stufe (`17` O-11); WebKit in Playwright als Näherung | 1 bis 5 |
| RP-15 | Die CI wird spürbar länger | wahrscheinlich / niedrig | eigene Zeilen in `server/testgroups.txt`, Fuzz und Langläufe wöchentlich oder auf CT 190; Messen bei Stufe 1 (`17` O-12) | 1 |
| RP-16 | Ältere Apps vertragen neue Aufzählungswerte nicht | möglich / mittel: leere Einträge | Auslieferung nur an Clients mit Live-TV-Fassung (`08` 7.2); Prüfung der erzeugten Kotlin-Clients (`08` O-1) | 1, 2, Apps |
| RP-17 | Ein Bauteil rutscht in Richtung der Ausschlussliste (Archiv, gemeinsamer Puffer, gemeinsame Umwandlung, vorsorgliche Umwandlung) | möglich / hoch für den Auftrag | Negativtests «gibt es nicht» in jeder Stufe (`17` 5.12); Review prüft Abschnitt 16 des Auftrags | alle |
| RP-18 | Andere Arbeit nimmt Frage 37 oder ADR 0046 zuerst | möglich / niedrig | Nummern erst beim Bau vergeben, Verweise im Konzept nachführen | 0 |
| RP-19 | Die Zeitumstellung verschiebt wiederkehrende Aufnahmen | möglich / mittel | V-12, Tests mit gestellter Uhr | 6 |
| RP-20 | Der Betreiber verletzt die Bedingungen seiner Anbieter (Mehrfachverteilung) | Sache des Betreibers / rechtlich | Hinweis in der Verwaltung (LTV-004.38) und in der Installationsanleitung | 1 |

### 8.2 Abbruch- und Umplanungspunkte

Kein Messergebnis führt dazu, dass eine verbindliche Anforderung still wegfällt. Muss etwas wegfallen oder anders
werden als beschlossen, geht es als Frage an dich, mit Varianten.

| Nr. | Auslöser | Was dann geschieht |
|---|---|---|
| **U-1** | V-1 zeigt, dass die meisten Sender eine Bild-Umwandlung brauchen | Stufe 1 bleibt; die Kapazitätsrechnung (`04` 7.6, `13` 3.3) wird mit den gemessenen Anteilen neu gesetzt, die Serverdeckel ab Werk angepasst; du bekommst die Zahl der gleichzeitigen Browser-Zuschauer auf deiner Hardware als Ergebnis, nicht als Zusage |
| **U-2** | V-2 zeigt, dass hls.js TS nicht verlässlich spielt | Auslieferung auf fMP4 umstellen (der Segmentierer ist die vorgesehene Stelle, T-11); `05` 4, `16` Matrix B und `08` 4 werden vor der Baufreigabe der Stufe 1 überarbeitet; Stufe 1 wird erst danach freigegeben |
| **U-3** | V-4 zeigt, dass es keine Sperre gibt | `12` F-1 entscheidet: Umwandlung mit sichtbarem Hinweis oder nur Direktweg. Weg W fällt dann nach `04` F-13 (a) weg; ob (b) «Go liest selbst» oder (d) «gar nicht» gilt, entscheidest du, weil (d) ein Abstrich an LTV-003 wäre |
| **U-4** | V-8 oder `05` M-12 zeigt ein Leck | Stufe 3 wird nicht freigegeben oder nicht abgenommen; der Fehler wird als Patch der betroffenen Stufe behoben, der Lauf wiederholt; nie die Prüfgrenze lockern |
| **U-5** | V-7 zeigt unhaltbare Schreiblast oder Lücken auf dem NAS | `04` F-8 (c) als Bremse für Timeshift; Netzspeicher als Speicherort «Aufnahmen» nur mit Warnung oder gar nicht (Frage an dich); Rückstau grösser, wenn das Byte-Budget es trägt (`13` O-2) |
| **U-6** | V-10 zeigt, dass Weg W nicht dicht ist | Stufe W entfällt in dieser Form; `04` F-13 an dich: (b) eigener Leser in Go, (c) mit Restrisiko oder (d) Abstrich an LTV-003.10 |
| **U-7** | V-11 zeigt, dass eine Bibliothek am Dialer vorbei verbindet | andere Bibliothek, eigene Umsetzung oder `04` F-1 (b) oder (c); Frage an dich |
| **U-8** | V-6 zeigt, dass Multicast auf deinem Aufbau nur mit Netzmodus «host» oder gar nicht geht | `04` F-11 (a) mit den benannten Folgen; geht es gar nicht, ist LTV-003.16 auf deinem Aufbau nicht erfüllbar, und du entscheidest, ob Stufe H ohne Multicast kommt |
| **U-9** | `13` M-6 zeigt, dass Live-TV Filme spürbar verzögert | Live-Anteil am Budget senken, Vorrang nach `13` P-7, P-8 neu entscheiden; Stufe wird erst nach der Korrektur abgenommen |
| **U-10** | Der Benchmark verfehlt Zielwerte | kein Abbruch: Zielwerte sind zu prüfende Grössen. Das Protokoll nennt jede Abweichung mit Grund; du entscheidest, ob die Stufe so abgenommen wird (`17` O-8) |
| **U-11** | V-5 zeigt Probleme mit Media3 | App-Ausgaben verschieben sich; die Web-Stufen laufen weiter (Variante (a) aus Abschnitt 5 als Rückfall) |
| **U-12** | Stufe 1a zeigt beim Bau, dass der Schnitt 1a/1b nicht trägt | 1a und 1b werden zu einem Release; die Abnahme bleibt dieselbe |
| **U-13** | `09` M-3 zeigt, dass auch die eigene Datei den Server stört | Import drosseln oder in die Nacht legen; Frage an dich, falls das die Aktualität der Programmdaten mindert |

---

## 9. Betroffene bestehende Funktionen

Alle bestehenden Tests bleiben unverändert grün; neue Erwartungen an bestehendes Verhalten gibt es nicht
(`17` 5.16). Die Regressionszeilen gelten ab der ersten Stufe, die einen Bereich berührt, und in jeder späteren.

| Bereich | Was sich ändert (Stufe) | Wie der Schutz geprüft wird |
|---|---|---|
| **Dateiwiedergabe und Umwandlung** | neue Budget-Schnittstelle «reservieren, umbuchen, freigeben» und Live-Anteil am Budget (1); Kopplung der Grenzen in beide Richtungen zwischen `playback` und `live` (1); `PlaybackCapabilities` additiv (1); abschaltbarer Baustein gegen Zeilensprung, nur von Live genutzt (1); FFmpeg-Empfänger des Wegs W ausserhalb des Live-Anteils (W) | `AK-L-282` (Entscheide und Argumentlisten identisch), `AK-L-283` (50 gleichzeitige Starts, genau einer gelingt), `13` M-6 mit Filmstart gegen den gemessenen Wert aus `docs/perf/wiedergabestart.md`, `AK-L-033` |
| **Radio und Podcasts** | Live-TV als dritte Tonquelle mit Ton-Vorrang (1); `netguard` bekommt weitere nie öffentliche Präfixe, wirkt auch für Radio und Podcasts (1, Verschärfung) | `AK-L-284` (Radio ohne Stockung während 20 Live-Sitzungen, bestehende Radio-Tests), bestehende Tests von `netguard`, Podcasts und Radio |
| **Konten, Profile, Einladungen** | Schalter «Live-TV» (1), «Timeshift» (4), «Aufnahmen» und Kontingent (5) auf Konto, Profil, Einladung; keine Kopplung an die Altersgrenze, also kein Auslöser aus ADR 0045 auf diesen Spalten (A-5); Löschdialog eines Profils mit Zahl und Grösse seiner Aufnahmen (5) | Migrationstests mit eingefrorenem Stand; Tests der Deckelregel (Profil nie mehr als Konto); bestehende Tests des Wunschrechts unverändert; `AK-L-285` (Jugendschutz der Bibliothek unverändert) |
| **Sichtbarkeit und Jugendschutz der Bibliothek** | keine Änderung; Live-Antworten tragen keine `ItemRef` (A-8); das Paket `live` fasst `items`, `media_files`, `tracks` nicht an | `itemsaccess`, `depguard`, Kinderprofil-Vertragstest unverändert (`AK-L-285`) |
| **Verwaltung** | Übersicht, Aktive Wiedergaben, Konten, Hintergrundjobs, Meine Medien, Seitenleiste (1); Benachrichtigungen der Verwaltung (1 Import, 3 Ausfälle); Update-Dialog (5); Speicherseite (4, 5) | Tastaturtests der geänderten Seiten; bestehende e2e-Specs und Storybook-Baselines nur mit Hinzufügungen (`AK-L-291`) |
| **Benachrichtigungen** | neue Arten und Ereignisse: Erinnerung (2), Ausfall eines Senders (3), Aufnahme fertig, fehlgeschlagen, abgelehnt (5); Neubau von `notifications` und `deliveries` (2) | `AK-L-290` (`TestServerTexts`, `task i18n:check`); Migration gegen eingefrorene Stände; PIN-Profile ohne Titel (`AK-L-271`) |
| **Sicherung und Wiederherstellung** | Zugangsdaten der Quellen in der Sicherung ohne Geheimnisse geleert (1, `12` F-4); `epg.db` nicht in der Sicherung (2, `09` F-1); Aufnahmedateien nicht in der Sicherung, nur ihre Angaben (5) | bestehende Tests der Sicherung; neue Tests «Sicherung enthält kein Testgeheimnis», Wiederherstellung ohne `epg.db` und ohne Aufnahmedateien |
| **Updater** | Warnung und Bestätigung bei laufenden oder bald beginnenden Aufnahmen über `installUpdate` (5, `04` F-6); der Updater-Begleiter selbst bleibt unverändert (ADR 0024) | `AK-L-226`, `AK-L-275`; bestehende Updater-Tests |
| **Compose und Installation** | Härtung von `compose.yaml` (0); Grenze offener Dateien (0); Zusatzdateien für UDP und Netzmodus «host» (H); `stop_grace_period: 30s` und Zusatzdatei mit Aufnahme-Volume (5) | `AK-L-281`; Testlauf der ganzen Prüfreihe mit gehärteter Datei (V-4); `AK-L-274` (Herunterfahren 10 s bis Stufe 4, 30 s ab Stufe 5) |
| **Hintergrundjobs** | neue Gruppe mit fünf Jobarten `livetv.*` (1, 2, 4, 5); der Aufnahme-Planer ist kein Job | `AK-L-288` (Pause einer Jobart bricht keine Aufnahme ab), bestehende Job-Tests |
| **Geräte und Sitzungen** | Live-Wiedergaben erscheinen in «Meine Geräte» und lassen sich beenden (1) | `AK-L-289` |
| **Downloads und Offline** | keine Aufnahmen offline; Live-Umwandlungen verdrängen Offline-Vorbereitungen nur nach der bestehenden Regel (1) | `AK-L-287` |
| **Protokolle** | Schwärzung erweitert, wirkt auf alle Protokolle; Überschwärzung harmlos (0) | `AK-L-098`, `AK-L-246` mit Gegentest |
| **Suche und Startseite** | Startseite unverändert (LS-26); allgemeine Suche höchstens ein Verweis ins Programm nach `03` F-5 / `14` F-3 (2) | bestehende Tests der Suche und der Startseite unverändert |
| **Statistik und Schauzeit** | Live-TV zählt zur Schauzeit, ohne Sender (1) | Tests von `getWatchReport` mit Hinzufügung |
| **Service Worker der Web-App** | darf Live-Listen und Segmente nicht zwischenspeichern (`08` 4.4) (1) | e2e-Prüfung, dass keine Live-Antwort im Cache landet |
| **TV-App und Handy-App** | neue Seiten je App-Ausgabe (Abschnitt 5); ohne Live-TV-Fassung keine Änderung | `AK-L-292` (bestehende Oberflächentests und Lint unverändert) |

---

## 10. Abdeckung

### 10.1 Grundsatzentscheide → Stufen

| Entscheid | Stufe(n) | Bemerkung |
|---|---|---|
| LTV-001 Funktionsumfang | 1, 2, 3, 4, 5, 6 | Sammelentscheid; erfüllt, wenn alle Detailentscheide erfüllt sind |
| LTV-002 Hybride Empfangsstrategie | 1 (bei Bedarf, Freigabe, Schutz laufender Wiedergaben), 3 (dauerhaft, automatisch, zeitgesteuert, Vorgaben, Vorladen, Wiederanlauf), 5 (Start für Aufnahmen, Schutz laufender Aufnahmen) | |
| LTV-003 Empfangsquellen | 1 (M3U, Einzel-URL, HTTP(S), HLS-TS, MPEG-TS, Xtream-Live, Zugangsdaten, Kopfzeilen, Rangfolge), 3 (automatischer Wechsel), H (UDP, RTP, Multicast, Heimnetz), L (RTSP, RTMP), W (DASH, HLS mit fMP4) | D-1, D-2 |
| LTV-004 Zentraler Empfang, individuelles Transcoding | 1, 3 (Ersatzquelle), 5 (Aufnahme ohne zweite Verbindung), E (Untertitel im Browser) | D-6 |
| LTV-005 EPG | 2, 5 (Aufnahme aus dem Programm, Anpassung geplanter Aufnahmen) | |
| LTV-006 Senderverwaltung | 1 (1a Grundteil, 1b erweitert) | |
| LTV-007 Timeshift | 4; 3 (Startpuffer getrennt) | D-3 |
| LTV-008 DVR | 5, 6 | |
| LTV-009 Aufnahmeeigentum | 5, 6 | E-3, A-6 |
| LTV-010 Fast Zapping | 1 (bestehende Ströme sofort nutzen), 3 (Startpuffer, Vorladen, Benchmark); `05` M-1 bei jeder Stufe mit Wiedergabe | |
| LTV-011 Streamüberwachung | 1 (Neuverbinden, Abbruch, Meldungen, Leiter), 3 (Rest), 3 oder E (stehendes Bild) | D-5 |
| LTV-012 Benutzerrechte | 1 (Live-TV, Grenzen, 404), 4 (Timeshift), 5 (Aufnahmen) | |
| LTV-013 Kein Catch-up | in jeder Stufe als Negativtest (`17` 5.12); 3 (Dauerempfang schreibt nichts auf) | Ausschluss |
| LTV-014 Oberfläche | 1 (Start, Senderübersicht, Player), 2 (Programm, Details), 4 (Timeshift im Player), 5 (Meine und Geplante Aufnahmen), 6 (Serien); App-Ausgaben | |
| LTV-015 Jugendschutz | 0 (Ausnahme dokumentiert), 1 (Freigabe durch das Hauptprofil), 2 (Programmdaten ohne Altersfilter), 5 (Aufnahmen der Profile mit Altersgrenze) | A-2, A-5 |
| LTV-016 Kein Multi-View, kein PiP | 1 (ein Sender je Player) und Negativtests in jeder Stufe | Ausschluss |
| LTV-017 Keine Anbieterlimits | 1 (Zugangsdaten sicher, kein Limit) und Negativtests | `04` F-3 |
| LTV-018 Automatische Senderaktualisierung | 1 (1a von Hand mit Erhalt der Pflege, 1b nach Zeitplan), 2 (EPG-Zuordnung bleibt), 5 (Planungen bleiben) | |
| LTV-019 Speicherverwaltung | 3 (Grösse des Startpuffers), 4 (Speicherort Timeshift, Grenzen, Reserve; Ort des Startpuffers nach `04` F-2), 5 (Speicherort Aufnahmen, Kontingent) | D-3 |
| LTV-020 Originalqualität | 5 | |
| LTV-021 Konzept vor Bau | Konzeptphase, V | dieses Dokument ist die verlangte Roadmap |

Keine der 21 Grundsatzentscheidungen bleibt ohne Stufe.

### 10.2 Ausschlüsse bleiben draussen

Keine Stufe plant oder bereitet etwas aus der Ausschlussliste vor (Auftrag Abschnitt 16; `03` 7.1, LTV-X-01 bis
LTV-X-29): kein Catch-up, kein serverweites Archiv, kein gemeinsamer Timeshift-Puffer (Speicherform nur nach
deinem ausdrücklichen Entscheid `04` F-8), keine gemeinsam sichtbare Aufnahmebibliothek, kein Multi-View, kein
Bild-in-Bild, keine Tuner, kein SAT>IP, DVB, HDHomeRun, TVHeadend, keine DRM-Umgehung, keine Xtream-VOD- oder
-Serienbibliotheken, keine Werbeerkennung, keine KI-Funktionen, keine automatische Altersprüfung, keine
Abonnementverwaltung, keine Anbieterlimits, keine gemeinsam genutzten oder vorsorglich gestarteten Umwandlungen,
keine automatische Komprimierung oder Konvertierung von Aufnahmen, keine Übernahme in die Bibliothek, keine
suchwortbasierten Aufnahmen, keine neuen Benutzergruppen, keine externen Umwandlungsknoten, keine
Live-TV-Microservices, keine verpflichtende IPTV-Middleware. Bestehende Nicht-Ziele bleiben (`03` 7.2): Fire TV,
`.deb`, Aufnahmen von Webradio, Plattform-Streams, Freigabe-Anfragen von Kinderprofilen, Mandanten.

Jede Stufe hat dafür die Negativtests aus `17` 5.12, und der Review jeder Stufe prüft Abschnitt 16 des Auftrags
ausdrücklich (RP-17).

### 10.3 Ausdrücklich nicht in Konzept 28

Nicht vorbereitet und nicht eingeplant, aber kein Ausschluss des Auftrags: Download und Offline-Abspielen von
Aufnahmen (Leitplanken; eine eigene Frage, falls gewünscht), LL-HLS (`16` 3.2), ein Push-Kanal für Zustände (T-23),
eine Deduplizierung gleicher Aufnahmen über Profile hinweg (`11` F-2, nur auf deine Antwort). Sie bekommen keine
Stufe und keinen Code.

---

## 11. Offene Punkte dieses Dokuments

| Nr. | Punkt | Gehört zu |
|---|---|---|
| O-1 | Folgeänderungen in anderen Papieren nach Abschnitt 1.5 und 4.1: Stufe H in `03` (LTV-003.14 bis .16), `07` 11.2 (`live_net_targets`, `listen_interface`), `08` 2.8 (`resolveLiveNetTarget`), `17` 8.1 bis 8.3; D-1 und D-2 in `03`; D-3 in `03` und `08`; D-4 in `17`; D-5 und D-6 (Stufe E) in `03` und `17`; D-7 in `04` | nächste Fassung der Papiere |
| O-2 | Aufteilung der `AK-L` auf 1a und 1b sowie auf 3 und H; neue `AK-L` für Stufe E | `17` |
| O-3 | Zuschnitt der ADR 0047 bis 0055 ist ein Vorschlag; die Nummern vergibt der Bau, Zahl und Grenzen entscheidet die jeweilige Stufe | Bau |
| O-4 | Ob Stufe 0 ein eigenes Patch-Release bekommt (Abschnitt 3.4) | du, beim Bauauftrag |
| O-5 | Wo das Wegwerf-Werkzeug von V-1 liegt, damit es nie ins Produkt gelangt (eigener, nie gemergter Zweig oder Arbeitsordner der Session) | Bau, vor V-1 |
| O-6 | Ob die TV-App-Ausgabe einer Stufe dieselbe Abnahme wie die Web-Stufe braucht oder eine verkürzte Prüfliste reicht | du, mit `03` F-6 |

*Ende von `18-umsetzungsplan.md`. Nichts in diesem Dokument ist gebaut oder gemessen; jede Stufe beginnt erst auf
Marcos ausdrückliches Wort.*
