# Konzept 28 · Klickbare Mockups

Stand: 28. November 2026 · Fassung 1 · Status: Entwurf, nicht freigegeben

Die Mockups zeigen alle Seiten und Dialoge aus `14-oberflaeche.md` (Zuschauer) und `15-administration.md`
(Verwaltung) in drei Clients zum Anklicken: **Web-App** (Computer, Tablet, Handy-Browser, dazu die
Verwaltung), **Android-TV-App** im gezeichneten Fernseher mit Fernbedienung und **Handy-App** im
Geräterahmen (Handy und Tablet, hoch und quer). Ordner: `docs/planning/mockups/28-live-tv/`.

## 1. Zweck und Grenzen

- **Zweck:** Abläufe und Seiten so zeigen, dass man sie bedienen kann, bevor etwas gebaut wird. Jeder Knopf
  tut etwas Sichtbares: Navigation, ein Dialog, eine Meldung oder eine Änderung des gespielten Zustands.
- **Simuliert:** Es gibt keinen Server und keinen Stream. Das «Fernsehbild» ist eine ruhige Fläche mit
  leichter Bewegung und dem Vermerk «Beispieldaten, simulierte Wiedergabe». Die Uhr des Mockups beginnt
  bei jedem Laden am **Samstag, 28. November 2026, 20:41** und läuft weiter.
- **Beispieldaten:** Konto «Familie Keller» mit Anna (Hauptprofil), Luca (ab 16) und Mia (ab 6), 16 Sender,
  Programm von Sa 28.11. bis Fr 4.12., Aufnahmen, Planungen, Quellen, EPG-Quellen und Speicher wie in
  14 Abschnitt 0.1 und 15 Abschnitt 0.1. Adressen nur auf `example.*`-Domänen und maskiert, keine
  Zugangsdaten.
- **Getrennt von der Anwendung:** Die Mockups teilen keinen Code mit `web/`, `android/` oder `server/`. Sie
  übernehmen nur das Aussehen der bestehenden Mockups 25 (Web), 26 (TV) und 27 (Handy).
- **Bilder nur lokal:** Senderlogos und Sendungsbilder liegen in `shared/img/` und sind nicht im Repo
  (Markenrechte). Ohne sie zeigen die Mockups Initialen auf heller Fläche und ruhige Flächen statt
  Sendungsbildern; bedienbar bleibt alles.
- **Gespielte Zahlen:** Was nicht aus dem Mockup-Zustand kommt (zum Beispiel 584 aktive Sender, 10'354
  importierte Einträge, CPU 31 %), sind feste Beispielwerte. Zeiten beim Umschalten im Player sind für die
  Vorführung verkürzt (Abschnitt 4).

## 2. So öffnest du sie

1. Im Dateimanager den Ordner `docs/planning/mockups/28-live-tv/` des Arbeitsbaums öffnen.
2. `index.html` doppelklicken. Der Browser zeigt vier Kacheln: **Web-App**, **Verwaltung**, **Fernseher**,
   **Handy und Tablet**, darunter die Liste aller 121 Seiten und Dialoge mit Kennung (LS-, LD-, LA-).
3. Eine Kachel anklicken. Oben steht immer eine gelbliche Leiste, die nicht zur App gehört:
   - **Zustände** öffnet die Schalter des Mockups (Abschnitt 4).
   - **Seite** springt direkt zu einer Seite oder einem Dialog.
   - **Alle Einstiege** führt zurück zur Übersicht.
4. Web-App: Mit **Desktop · Tablet · Handy** wechselst du die Breite. **Zuschauer · Verwaltung** wechselt
   zwischen App und Verwaltung.
5. Fernseher: Bedienen mit der gezeichneten Fernbedienung oder der Tastatur: Pfeiltasten, Enter = OK,
   Backspace oder Esc = Zurück, Leertaste = Wiedergabe/Pause, Bild↑/Bild↓ = Kanal +/−, G = Guide,
   I = Info, R = Aufnahme, L = zuletzt gesehen, 0–9 = Ziffern.
6. Handy: Mit **Handy · Tablet** und **Hochformat · Querformat** wechselst du den Rahmen. Das Menü links
   (Medienauswahl) öffnet der Knopf oben links oder Wischen vom linken Rand.
7. Wer lieber einen kleinen Dateiserver nimmt: im Repo `python3 -m http.server 8728 --directory
   docs/planning/mockups/28-live-tv` starten und `http://localhost:8728/` öffnen.
8. Zurücksetzen: Neu laden setzt alles auf 20:41 zurück; in der Leiste gibt es auch «Alles zurücksetzen».

## 3. Rundgänge

### 3.1 Web-App (Zuschauer)

1. Einstieg «Web-App»: In der Seitenleiste unter «Meine Medien» steht neu **Live-TV · 16 Sender** (LS-20).
2. **Sender** (LS-02): Gruppe «Schweiz» wählen, nach «zdf» suchen, auf **Kacheln** umschalten, beim Herz
   von RTS 1 einen Lieblingssender setzen und mit «Rückgängig» zurücknehmen. Menü ⋯ einer Zeile öffnen.
3. **Start** (LS-01): Reihen «Jetzt im Fernsehen», «Gleich», «Zuletzt gesehen», «Meine Sender», «Nach
   Genre»; Hinweis auf Warnungen der geplanten Aufnahmen.
4. **Zappen:** Eine Zeile einschalten (LS-07). Bild↑/Bild↓ wechselt den Sender, `C` öffnet die
   Senderliste, Ziffern `1` `1` springen zu ZDF.
5. **Timeshift:** Leertaste hält an; nach einigen Sekunden zeigt die Leiste den Zeitversatz und «Zurück zu
   Live». Mit ← spulst du zurück, aber nie vor dein Einschalten. Bei Zeitversatz fragt ein Senderwechsel
   nach (LD-10).
6. **Sofortaufnahme:** Im Player «Aufnehmen» (LD-04): «Bis Sendungsende» oder eine feste Dauer.
7. **Programm** (LS-04): Raster mit Jetzt-Linie, «Hauptabend 20:15», Pfeiltasten von Sendung zu Sendung, `R`
   plant eine Aufnahme (LD-01), `E` eine Erinnerung (LD-05). TeleZüri und phoenix zeigen «Keine
   Programmdaten», KiKA endet um 21:00.
8. **Sendungsdetails** (LS-06): «Ein starkes Team» öffnen, Aufnehmen, Erinnern, Senderprogramm (LS-15).
9. **Serie:** Programmsuche (LS-05) nach «tatort»: SRF 1 So 20:05 ist «Serie geplant», Das Erste 20:15
   «Schon geplant». Serienaufnahme ändern (LD-02).
10. **Konflikt:** Aufnahmen › Geplant (LS-11) zeigt Warnungen; «Konflikte und Warnungen» (LS-12) erklärt
    Kontingent, Überschneidung, «Sender weg» (TV24) und geänderte Zeit (LD-07).
11. **Meine Aufnahmen** (LS-08): laufende Aufnahme (wächst mit der Uhr), Tatort als Stapel, unvollständige
    Aufnahme mit Lücken (LS-09), fehlgeschlagene unter dem Aufklapper; eine Aufnahme abspielen (LS-10).
12. **Kinderprofil:** Unter «Zustände» Luca oder Mia wählen; als Anna «Aufnahmen meiner Profile» (LS-14)
    öffnen. Unter «Profile» den Block «Live-TV» mit den drei Schaltern ansehen (LS-21).
13. **Kontingent voll:** Unter «Zustände» → Aufnahmen «Kontingent voll» einschalten; LD-04 lehnt ab, unter
    «Aktionen» startet «Start einer geplanten Aufnahme» einen Termin, der abgelehnt und gemeldet wird.
14. **Recht entzogen:** Im Player unter «Zustände» «Recht entzogen während der Wiedergabe» einschalten.

### 3.2 Verwaltung (Web-App)

1. Einstieg «Verwaltung»: Seitenleiste mit dem neuen Bereich **Live-TV** und Pillen «3 gestört», «Fehler»,
   «Speicher knapp» (LA-25); `Strg K` sucht Seiten, Sender und Quellen.
2. **Übersicht** (LA-01): Zustand «Eingeschränkt», Kennzahlen, «Braucht Aufmerksamkeit».
3. **Quellen** (LA-02): «Jetzt aktualisieren» bei «Xtream Beispielanbieter» (Fortschritt, danach in
   Ordnung), «Testen», Zeile aufklappen, Menü ⋯ → «Entfernen» mit Wahl.
4. **Quelle anlegen** (LA-03): «Quelle hinzufügen», Art wählen (M3U-Datei, M3U-URL, Xtream, Einzelstrom),
   Adresse, Test (das Ergebnis des Tests lässt sich im Mockup wählen), Vorschau, übernehmen.
5. **Import** (LA-04): Ergebnis von «Kabelanbieter Schweiz»: neu, geändert, verschwunden, Duplikate,
   fehlerhaft.
6. **Sender** (LA-05): Filter, Suche, Auswahl mehrerer Sender und Massenbearbeitung.
7. **Sender bearbeiten** (LA-06): SRF 1 öffnen, Aufklapper «Quellen» mit Rangfolge der Ersatzquellen.
8. **Sender zusammenführen** (LA-07): Reiter «Duplikate», «SRF1 HD CH (Xtream)» mit SRF 1 zusammenführen.
9. **Gruppen** (LA-08): eigene Gruppe anlegen und ordnen.
10. **Empfang** (LA-09): Standard «Permanent» wählen und speichern; die Übersicht LA-01 und Aktive Streams
    LA-12 zeigen danach dieselben zusätzlichen Eingänge.
11. **EPG** (LA-10, LA-11): Quelle laden; unter «Zuordnungen» RTS 1 («unsicher») bestätigen oder ändern.
12. **Aktive Streams** (LA-12) und **Diagnose** (LA-13): Abnehmer ohne Titel, ZDFinfo «wiederverbindet»,
    Protokoll ohne Zugangsdaten.
13. **Aufnahmen und Speicher** (LA-14 bis LA-16): DVR-Vorgaben, Timeshift (Werksvorgabe 1 Stunde, hier
    2 Stunden eingestellt), drei Speicherorte, Belegung je Konto; unter «Zustände» → Verwaltung «Speicher
    voll» oder «Netzlaufwerk nicht eingehängt».
14. Geänderte bestehende Seiten: Konten mit Live-TV-Rechten und Kontingent (LA-22), Aktive Wiedergaben
    (LA-21), Hintergrundjobs (LA-23) und LA-20, LA-24, LA-26 bis LA-28.

### 3.3 Fernseher

1. Einstieg «Fernseher»: Leiste links mit **Live-TV** vor Webradio (LS-20).
2. **Sender** (LS-02): ↑/↓ durch die Liste, → zum Herz, OK schaltet ein; Kanal +/− blättert sieben Zeilen;
   Ziffern `1` `1` springen zu ZDF; rechts die Vorschau der Sendung.
3. Filter (Seitenmenü von rechts) und Suche mit der Bildschirmtastatur.
4. **Player** (LS-07): ↑/↓ wechselt den Sender, OK zeigt die Bedienleiste, ← die Senderliste, → das
   Info-Banner; Wiedergabe/Pause hält an (Timeshift), Zurück zu Live.
5. **Guide** (Taste G): Programmraster (LS-04), OK auf einer Sendung öffnet die Details als Seitenmenü
   (LS-06); «2 h»-Knöpfe verschieben die Zeit.
6. **Aufnehmen:** Taste R im Player (LD-04) oder im Raster (LD-01); Serie aufnehmen (LD-02).
7. **Aufnahmen** (LS-08 bis LS-12): Reihen «Läuft», «Neu», «Serien», «Fehlgeschlagen»; Geplant; Warnungen.
8. **Erinnerungen** (LS-13) und **Aufnahmen meiner Profile** (LS-14, nur als Anna).
9. Unter «Zustände» → Player «Sender nicht erreichbar» oder «Strom stockt» und im Player ansehen.
10. «Viele Sender» einschalten und im Guide nach unten blättern (2'000 Testsender).

### 3.4 Handy und Tablet

1. Einstieg «Handy und Tablet»: Menü links (Knopf oder Wischen) mit **Live-TV** nach Webradio; die untere
   Leiste bleibt (LS-20).
2. **Sender** (LS-02): Suche, «Filter» als Blatt (LD-09), Herz, Menü ⋯ oder langes Drücken.
3. **Player** (LS-07): Im Hochformat Bild oben, darunter Info und Senderliste; quer Vollbild mit Bedienung.
   Wischen nach oben/unten wechselt den Sender, die Leiste spult (Timeshift).
4. **Programm** (LS-04): am Handy als Liste «Jetzt und danach», am Tablet als Raster.
5. Sendungsdetails (LS-06), Aufnahme planen (LD-01), Serie (LD-02), Erinnerung (LD-05).
6. **Meine Aufnahmen** (LS-08) mit Speicherleiste, Auswahl mehrerer Aufnahmen und Löschen (LD-06).
7. **Geplant** (LS-11) und **Konflikte und Warnungen** (LS-12).
8. Rahmen auf **Tablet · Querformat**: feste Seitenspalte statt Menü.
9. Einstellungen › Profile (LS-21) und Benachrichtigungen (LS-22, LS-23).
10. Unter «Zustände» → Aufnahmen «Kontingent voll»; die Sofortaufnahme (LD-04) lehnt ab.

## 4. Die Schalter «Zustände»

Je Zustand gibt es genau einen Schalter, in allen drei Clients gleich (Kern `shared/live-mock.js`,
Zustand in `shared/live-sim.js`). Seitenlokale Mockup-Schalter gibt es nicht mehr. Einzige Ausnahme ist die
Auswahl «Ergebnis des Tests» in LA-03, weil sie die Antwort einer gedachten Quelle auf genau diesen Test
darstellt und kein Zustand ist.

| Schalter | Was du siehst |
|---|---|
| Uhr: Anhalten, +5 Min., +30 Min., +2 Std. | Fortschrittsbalken, laufende Aufnahme, Programm rücken vor |
| Wer schaut? Anna, Luca, Mia | andere Favoriten, Aufnahmen, Rechte; LS-14 nur für Anna |
| Rechte des Profils / Deckel des Kontos: Live-TV, Aufnahmen, Timeshift | ohne Live-TV fehlt der Bereich ganz (Adressen «Nicht gefunden»); ohne Aufnahmen fehlen alle Aufnahme-Knöpfe; ohne Timeshift fehlen Anhalten und Spulen |
| Recht entzogen während der Wiedergabe | der Player zeigt «Live-TV ist für dieses Profil nicht mehr freigegeben.», alle anderen Live-TV-Adressen «Nicht gefunden» |
| Zustand der Seiten: Normal, Leer, Lädt, Fehler | jede Seite zeigt ihren Leer-, Lade- (Skeleton) und Fehlerzustand mit «Erneut versuchen» |
| Ohne EPG | kein Sender hat Programmdaten; Fernsehen geht trotzdem |
| Sender gestört: zusätzlich SRF 1 | SRF 1 «Gerade nicht erreichbar» (ZDFinfo ist immer gestört) |
| Viele Sender | 2'000 Testsender im Programmraster (Web und TV) |
| Kontingent voll | Speicher des Kontos 200 von 200 GB; neue Aufnahmen werden abgelehnt und gemeldet |
| Player: Sender nicht erreichbar, Ersatzquelle aktiv, Grenze erreicht, Strom stockt, Langsamer Start, Nicht abspielbar, Server ausgelastet | die Fehler- und Ladebilder aus 14 Abschnitt 4.11 und 4.12 |
| Verwaltung (nur Web): Speicher knapp, Speicher voll, Netzlaufwerk nicht eingehängt, EPG-Import fehlgeschlagen | Warnungen und Zustände in LA-01, LA-10, LA-12, LA-14 bis LA-16 |
| Aktionen (je Client) | einmalige Vorführungen, zum Beispiel «Erinnerung jetzt zeigen» in der Web-App |

**Ladezeiten im Player:** Das Konzept schlägt 0,5 s (Ladeanzeige), 5 s («Dauert länger als üblich …») und
15 s (Fehlerbild) vor (14, 4.11). Damit man alles in wenigen Sekunden sieht, sind die Werte im Mockup
**verkürzt**: Ladeanzeige ab 0,5 s, «Dauert länger» ab 2,5 s, Fehlerbild nach 6 s. Ein schon empfangener
Sender ist nach 0,6 s da, ein neuer nach 1,8 s, «Langsamer Start» nach 4,5 s. Die Werte gelten in allen
drei Clients gleich (`LIVE_UI.LOAD`).

## 5. Abdeckung des Auftrags (Abschnitt 13)

Routen gelten in allen Clients gleich als Hash-Adresse (`index.html#/…`), sofern der Client die Seite hat.
«W» = Web-App, «V» = Verwaltung, «T» = TV, «H» = Handy-App.

### 5.1 Benutzeroberfläche (13.1)

| Auftrag | Seite oder Dialog | Clients | Route |
|---|---|---|---|
| Live-TV-Startseite | LS-01 | W, T, H | `#/live` |
| Senderübersicht | LS-02 | W, T, H | `#/live/channels` |
| Sendergruppen | LS-02 (Filter «Gruppe»), Verwaltung LA-08 | W, T, H, V | `#/live/channels?group=ch`, `#/admin/live/channels?tab=groups` |
| Favoriten | LS-03 Meine Sender, Herz in LS-02 | W, T, H | `#/live/channels/favorites` |
| Programmzeitschrift | LS-04 | W, T, H | `#/live/guide` |
| Sendungsdetails | LS-06 | W, T, H | `#/live/programs/p-zdf-1215` |
| Live-TV-Player | LS-07 | W, T, H | `#/live/watch/srfzwei` |
| Senderwechsel im Player | LS-07, LD-10 | W, T, H | `#/live/watch/srfzwei`, `#/live/dialog/LD-10` |
| Timeshift-Bedienung | LS-07 (Timeshift-Leiste) | W, T, H | `#/live/watch/srfzwei` |
| Sofortaufnahme | LD-04 | W, T, H | `#/live/dialog/LD-04` |
| Geplante Aufnahme | LD-01, LD-03 | W, T, H | `#/live/dialog/LD-01`, `#/live/dialog/LD-03` |
| Serienaufnahme | LD-02 | W, T, H | `#/live/dialog/LD-02` |
| Meine Aufnahmen | LS-08, LS-09, LS-10 | W, T, H | `#/live/recordings`, `#/live/recordings/rec-terra`, `#/live/recordings/rec-arena/play` |
| Geplante Aufnahmen | LS-11 | W, T, H | `#/live/scheduled` |
| Aufnahmekonflikte | LS-12, LD-07 | W, T, H | `#/live/scheduled/warnings`, `#/live/dialog/LD-07` |
| Sendungserinnerungen | LS-13, LD-05 | W, T, H | `#/live/reminders`, `#/live/dialog/LD-05` |
| Leere Zustände | jede Seite (Schalter «Leer») | W, V, T, H | – |
| Ladezustände | jede Seite (Schalter «Lädt»), Player-Phasen | W, V, T, H | – |
| Fehlerzustände | jede Seite (Schalter «Fehler»), Player-Fehler (Schalter «Player») | W, V, T, H | – |

Dazu: LS-05 Programmsuche, LS-14 Aufnahmen meiner Profile, LS-15 Senderprogramm, LD-06 Löschen und
Abbrechen, LD-08 Tag und Uhrzeit, LD-09 Filterblatt (W, H), LD-11 Aufbewahrung.

### 5.2 Administration (13.2), nur Web-App

| Auftrag | Seite | Route |
|---|---|---|
| Live-TV-Dashboard | LA-01 | `#/admin/live` |
| Quellenverwaltung | LA-02 | `#/admin/live/sources` |
| M3U-Import | LA-03 (Art M3U-Datei oder M3U-URL), LA-04 | `#/admin/live/sources/new`, `#/admin/live/sources/src-kabel/import/run-k5` |
| Xtream-Quellenkonfiguration | LA-03 (Art Xtream) | `#/admin/live/sources/src-xtream` |
| Manuelle Streamkonfiguration | LA-03 (Art Einzelstrom) | `#/admin/live/sources/src-mc` |
| Senderverwaltung | LA-05 | `#/admin/live/channels?tab=channels` |
| Sendergruppen | LA-08 | `#/admin/live/channels?tab=groups` |
| Senderbearbeitung | LA-06 | `#/admin/live/channels?tab=channels&edit=srf1` |
| Ersatzquellen | LA-06 (Aufklapper «Quellen»), LA-07 | `#/admin/live/channels?tab=duplicates` |
| Empfangsstrategien | LA-09 | `#/admin/live/reception` |
| Empfangszeitpläne | LA-09 (Reiter «Zeitpläne») | `#/admin/live/reception?tab=schedules` |
| EPG-Verwaltung | LA-10 | `#/admin/live/epg?tab=sources` |
| EPG-Zuordnungen | LA-11 | `#/admin/live/epg?tab=mapping` |
| Aktive Streams | LA-12 | `#/admin/live/streams` |
| Streamüberwachung | LA-12, LA-13 | `#/admin/live/diagnostics` |
| DVR-Einstellungen | LA-14 | `#/admin/live/storage?tab=dvr` |
| Timeshift-Einstellungen | LA-15 | `#/admin/live/storage?tab=timeshift` |
| Speicherverwaltung | LA-16 | `#/admin/live/storage?tab=storage` |
| Fehlerdiagnose | LA-13 (mit «Jetzt prüfen» je Sender) | `#/admin/live/diagnostics?channel=zdfinfo` |

### 5.3 Bestehende anzupassende Seiten (13.3)

| Auftrag | Seite | Clients | Route |
|---|---|---|---|
| Hauptnavigation | LS-20 (Zuschauer), LA-25 (Seitenleiste und Suchpalette der Verwaltung) | W, T, H, V | jede Seite |
| Benutzerberechtigungen | LA-22 Konten (Rechte und Kontingent je Konto) | V | `#/admin/accounts` |
| Profilverwaltung | LS-21 Profilformular, Block «Live-TV» | W, T, H | `#/settings/profiles` |
| Kinderprofil-Einstellungen | LS-21 (Profile mit Altersgrenze, Hinweis «zeigt alles»), LS-14 | W, T, H | `#/settings/profiles`, `#/live/recordings/profiles` |
| Serverübersicht | LA-20 Verwaltungsübersicht | V | `#/admin` |
| Aktive Wiedergaben | LA-21 | V | `#/admin/streams` |
| Weitere betroffene Seiten | LS-22 Benachrichtigungen einstellen, LS-23 Glocke, LS-24 Suche, LS-25 Ton-Vorrang, LS-26 Startseite (unverändert), LA-23 Hintergrundjobs, LA-24 Benachrichtigungen der Verwaltung, LA-26 Meine Medien, LA-27 Server › Update, LA-28 Sicherung | W, T, H, V | siehe Sprungmenü |

### 5.4 Qualitätsanforderungen (13.4)

| Anforderung | Umsetzung |
|---|---|
| realistisch, vollständig navigierbar, bedienbar | alle 121 Registereinträge gebaut; jede Seite mit fester Adresse und im Sprungmenü |
| Buttons, Dialoge, Menüs, Submenüs, Kontextmenüs | Kontextmenüs (Web ⋯, Rechtsklick und Shift+F10 im Raster), Seitenmenüs am TV, Blätter am Handy |
| Filter, Tabs, Suchfelder, Eingabe- und Auswahlfelder | Filter und Suche in LS-02, LS-05, LA-05; Reiterleisten; Formulare LA-03, LD-01 bis LD-03 |
| wesentliche Bedienabläufe | Rundgänge in Abschnitt 3 |
| echte Senderlogos und Beispielbilder | lokal in `shared/img/` (nicht im Repo); ohne sie Initialen und Flächen |
| keine statischen Screenshots, keine Wireframes, zusammenhängende Navigation | HTML, CSS und JavaScript im Stil der bestehenden Mockups, gemeinsame Daten und Zustände über alle Seiten |
| keine produktiven Zugangsdaten oder Stream-URLs | nur `example.*`-Adressen, maskiert |
| keine produktive Streaming-Engine nötig | Wiedergabe simuliert |

## 6. Abweichungen vom Oberflächenkonzept

| Client | Abweichung | Grund | 14/15 nachziehen? |
|---|---|---|---|
| Web | LS-21: Name und Altersgrenze im Formular änderbar; der Hinweis «Live-TV zeigt alles …» erscheint nur bei Profilen mit Altersgrenze | Das bestehende Profilformular hat diese Felder; der Hinweis ist nur bei begrenzten Profilen nötig (14, 1.3 sagt «bei jedem Profil mit Altersgrenze») | nein, entspricht 14 |
| Web | LS-15: auch künftige Lücken ohne Programmdaten haben «Zeit aufnehmen» | konsequent zu «Ohne EPG» (LS-15), 14 nennt es nur für Sender ganz ohne Daten | ja, ein Satz in 3.15 |
| Web | LS-25: Einschalten von Live-TV beendet Radio und Hörbuch vor dem Umschalten | so beschreibt es 14 (1.2, LS-25) | nein |
| Web | LD-10 im Web-Player nicht-modal über der Leiste | wie 14 (5.10) | nein |
| Web | Programm-Überlagerung im Player zeigt Details künftiger Sendungen als Seitenleiste | Einschalten geht nur für laufende Sendungen; Details ohne den Player zu verlassen | ja, 4.9 ergänzen |
| Web | Bedienleiste des Players blendet nach 4 s aus | Verhalten des bestehenden Videoplayers | nein |
| Web | «Recht entzogen» zeigt die eigene Adresse `#/player/revoked` | damit die Meldung bleibt, wenn der Bereich schon fehlt | nein (nur Mockup) |
| Web | LS-14: geplante Termine eines Kinderprofils nur als Zahl | F-4 ist offen | hängt an F-4 |
| Web | LD-09 bleibt die Grundform des Kerns | genügt für 5.9 | nein |
| Verwaltung | LA-05: Gruppen unter dem Sendernamen statt als eigene Spalte | Platz in der kompakten Tabelle | ja, Spalte «Gruppen» in 3.5 als «unter dem Namen» |
| Verwaltung | LA-01: Zustand «Eingeschränkt» statt «Bereit» | aus den gestörten Sendern berechnet, wie 15 (3.1) es vorsieht | nein |
| Verwaltung | LA-05: Tabelle zeigt 16 Beispielsender plus sechs Einträge der Verwaltung, die Köpfe zählen auf 584 | nur die Beispielsender sind im Einzelnen erfasst | nein (nur Mockup) |
| Verwaltung | LA-22 und LA-26: Konto-Detail und Meine Medien zeigen nur das Neue | der Rest bleibt unverändert | nein |
| Verwaltung | LA-11: Zähler aus den Beispielzahlen (584), Tabelle nur mit Beispielsendern | wie LA-05 | nein (nur Mockup) |
| Verwaltung | LA-09: Schalter je Quelle «vorsorglich zusätzliche Verbindungen öffnen» | entspricht T-18 in `04-architektur.md`; im Mockup ausdrücklich «kein Anbieterlimit» beschriftet | **hängt an offener Frage** (LTV-017, Werkszustand F-9) |
| Verwaltung | LA-15: «Werksvorgabe 1 Stunde, eingestellt 2 Stunden» | Fachkonzept: Vorgabe 60 Min.; die Beispielinstanz hat 2 Std. | ja, Vorgabe in 15 (3.15) von 2 Std. auf 60 Min. angleichen |
| TV | OK zeigt die Bedienleiste, ← die Senderliste | 14 F-7 Variante A | hängt an F-7 |
| TV | Senderliste im Player folgt der Herkunftsliste, Favoriten nur mit Herz markiert | einfacher am TV | nein |
| TV | Zeit im Raster zusätzlich über «2 h»-Knöpfe | viele Fernbedienungen haben keine Spultasten | ja, 4.3 ergänzen |
| TV | kein LD-09 | Filter sind am TV ein Seitenmenü (14, 3.2) | nein |
| TV | LS-21, LS-23, LS-24, LS-26 minimal | nur die Änderung wird gezeigt | nein |
| Handy | «Programm» im Player als Blatt statt Überlagerung | Bildschirmgrösse | ja, 4.9 für die Handy-App |
| Handy | «Details» im Player verlässt den Player und beendet die Wiedergabe | Grundsatz 6 in 14 (wer den Player verlässt, beendet die Wiedergabe) | nein |
| alle | Ladezeiten im Player verkürzt (Abschnitt 4) | Vorführung | nein (nur Mockup) |

## 7. Bekannte Grenzen

- Die Glocke (LS-23) zeigt feste Beispielmeldungen. Meldungen, die im Mockup entstehen (abgelehnte
  Aufnahme bei vollem Kontingent), stehen im Zustand (`LIVE_SIM.notices()`), erscheinen aber nur als
  Meldung auf der Seite, nicht als neuer Eintrag der Glocke.
- «Viele Sender» wirkt im Raster der Web-App und des Fernsehers, nicht im Raster der Handy-App am Tablet.
- Die Verwaltung ändert die Beispieldaten direkt; «Alles zurücksetzen» stellt den Zustand der Zuschauer
  wieder her, Änderungen der Verwaltung erst ein Neuladen.
- Zahlen ohne Bezug zum Mockup-Zustand (Kennzahlen des Servers, Belegung anderer Konten) sind fest.
- Spracheingabe am TV, Push-Mitteilungen und echte Tastenkürzel des Betriebssystems sind nicht gespielt.

## 8. Was geprüft wurde

- **Syntax:** `node --check` für alle JavaScript-Dateien: ohne Fehler.
- **Daten:** ein Skript lädt `shared/live-data.js` und `live-sim.js` mit Node und prüft Sender,
  Programmpunkte aus 0.1, Lücken, Aufnahme- und Planungsarten, die Serie «Tatort», Rechte, Zustände und
  Register: vollständig. Belegung: Anna 82 GB, Luca 56.7 GB, Mia 3.3 GB, zusammen 142 GB in 31 Aufnahmen.
- **Doppelklick (`file://`, ohne Dateiserver):** Chromium ohne Kopf (Playwright aus dem Haupt-Checkout) öffnet
  jede Register-Adresse aller vier Einstiege, die Web-App in Desktop- und Handy-Breite, die Handy-App als
  Handy hoch und Tablet quer: 212 Adressen, 0 Konsolenfehler, 0 fehlgeschlagene Anfragen, 0 Seiten mit
  waagrechtem Überlauf, 0 Mal «undefined», «NaN» oder «[object Object]». Derselbe Lauf mit umbenanntem
  `shared/img/`: dieselben Werte; nur die Bilddateien fehlen (Rückfall auf Initialen und Flächen).
- **Zustände:** ein zweites Skript schaltet auf 20 Player-, Aufnahme-, Programm- und Verwaltungsseiten aller
  Clients 20 Schalter aus Abschnitt 4 nacheinander um (400 Wechsel): 0 Fehler; die Fehlerbilder «Grenze
  erreicht», «Server ausgelastet», «Strom stockt» und «Recht entzogen» erscheinen in allen drei Playern. In der
  Web-App bleibt nach dem Ausschalten von «Recht entzogen» die Meldung stehen, bis man den Player verlässt.
- **Angesehen** (Bildschirmfoto, über einen Dateiserver): Web LS-02, LS-12 bis LS-14, LD-01, LD-03 bis
  LD-11, LA-02, LA-05 in Handy-Breite; TV LS-02, LS-12 bis LS-14, LD-04, LD-06, LD-08, LD-11; Handy LS-02,
  LS-08, LS-12, LS-21 bis LS-26 als Tablet quer.
- **Tastatur in der Verwaltung:** LA-05/LA-06 (Dialog öffnet mit Enter, Fokus bleibt im Dialog, Esc schliesst,
  Fokus zurück auf die Zeile), LA-11 und LA-16 (Dialog «Ändern …», Esc, Fokus zurück), LA-03 (Reihenfolge
  der Tab-Halte folgt der Seite, alle Elemente beschriftet).

**Nicht geprüft:** Firefox und Safari; ein echtes Android-Gerät; Bildschirmleser; Kontrastmessung (die Farben
stammen aus den bestehenden Mockups); jede Seite in jeder Breite einzeln angesehen (nur die Liste oben und die
Musterseiten); die Bedienung mit der Fernbedienung auf jeder TV-Seite von Hand.
