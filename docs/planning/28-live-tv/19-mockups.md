# Konzept 28 · Klickbare Mockups

Stand: 28. November 2026 · Fassung 2 · Status: Entwurf nach Review 3, nicht freigegeben

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
  (Markenrechte). Echte Logos und Bilder siehst du nur in diesem Arbeitsbaum und nur, wenn du den Schalter
  «Echte Logos und Bilder» einschaltest (auf `index.html` oder unter «Zustände»). Ab Werk ist er aus: Dann
  zeigen die Mockups Initialen auf heller Fläche und ruhige Flächen und fragen keine Bilddatei an, auch
  nicht in einem Klon des Repos, wo die Bilder fehlen. Bedienbar bleibt alles.
- **Gespielte Zahlen:** Was nicht aus dem Mockup-Zustand kommt (zum Beispiel 584 aktive Sender, 10'354
  importierte Einträge, CPU 31 %), sind feste Beispielwerte. Zeiten beim Umschalten im Player sind für die
  Vorführung verkürzt (Abschnitt 4).

## 2. So öffnest du sie

1. Im Dateimanager den Ordner `docs/planning/mockups/28-live-tv/` des Arbeitsbaums öffnen.
2. `index.html` doppelklicken. Der Browser zeigt vier Kacheln: **Web-App**, **Verwaltung**, **Fernseher**,
   **Handy und Tablet**, darunter die Liste aller 121 Seiten und Dialoge mit Kennung (LS-, LD-, LA-).
3. Hast du den Ordner `shared/img/`: oben «Echte Logos und Bilder» einschalten (gilt für alle drei Mockups).
4. Eine Kachel anklicken. Oben steht immer eine gelbliche Leiste, die nicht zur App gehört:
   - **Zustände** öffnet die Schalter des Mockups (Abschnitt 4).
   - **Seite** springt direkt zu einer Seite oder einem Dialog.
   - **Alle Einstiege** führt zurück zur Übersicht.
5. Web-App: Mit **Desktop · Tablet · Handy** wechselst du die Breite. **Zuschauer · Verwaltung** wechselt
   zwischen App und Verwaltung.
6. Fernseher: Bedienen mit der gezeichneten Fernbedienung oder der Tastatur: Pfeiltasten, Enter = OK,
   Backspace oder Esc = Zurück, Leertaste = Wiedergabe/Pause, Bild↑/Bild↓ = Kanal +/−, G = Guide,
   I = Info, R = Aufnahme, L = zuletzt gesehen, 0–9 = Ziffern.
7. Handy: Mit **Handy · Tablet** und **Hochformat · Querformat** wechselst du den Rahmen. Das Menü links
   (Medienauswahl) öffnet der Knopf oben links oder Wischen vom linken Rand.
8. Wer lieber einen kleinen Dateiserver nimmt: im Repo `python3 -m http.server 8728 --directory
   docs/planning/mockups/28-live-tv` starten und `http://localhost:8728/` öffnen.
9. Zurücksetzen: Neu laden setzt alles auf 20:41 zurück; in der Leiste gibt es auch «Alles zurücksetzen».

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
| Echte Logos und Bilder | an: Logos und Bilder aus `shared/img/`; aus (ab Werk): Initialen und Flächen, keine Anfrage nach Bilddateien |
| Uhr: Anhalten, +5 Min., +30 Min., +2 Std. | Fortschrittsbalken, laufende Aufnahme, Programm rücken vor |
| Wer schaut? Anna, Luca, Mia | andere Favoriten, Aufnahmen, Rechte; LS-14 nur für Anna |
| Rechte des Profils / Deckel des Kontos: Live-TV, Aufnahmen, Timeshift | ohne Live-TV fehlt der Bereich ganz: kein Eintrag, kein Hinweis auf Start, Suche, Benachrichtigungen, Profilseite, Radio, im Menü oder in der Leiste; Adressen «Nicht gefunden»; die neuen Fernbedienungstasten tun nichts. Ohne Aufnahmen fehlen alle Aufnahme-Knöpfe und -Texte; ohne Timeshift fehlen Anhalten, Spulen und ihre Hinweise. Das Hauptprofil sieht in «Profile verwalten» weiter den Block «Live-TV», weil es dort die Rechte setzt |
| Recht entzogen während der Wiedergabe | der Player zeigt «Live-TV ist für dieses Profil nicht mehr freigegeben.», alle anderen Live-TV-Adressen «Nicht gefunden» |
| Zustand der Seiten: Normal, Leer, Lädt, Fehler | jede Seite zeigt ihren Leer-, Lade- (Skeleton) und Fehlerzustand mit «Erneut versuchen». «Leer» heisst: keine Sender, keine Aufnahmen, keine Planungen, keine Erinnerungen, keine Favoriten, nichts zuletzt gesehen; so zeigen auch LS-08 bis LS-15 und die Tabellen der Verwaltung ihren eigenen Leerzustand. Einstellungsseiten der Verwaltung (LA-09, LA-14, LA-15) behalten ihre Felder |
| Ohne EPG | kein Sender hat Programmdaten; Fernsehen geht trotzdem |
| Sender gestört: zusätzlich SRF 1 | SRF 1 «Gerade nicht erreichbar» (ZDFinfo ist immer gestört) |
| Viele Sender | 2'000 Testsender im Programmraster (Web und TV) |
| Kontingent voll | Speicher des Kontos 200 von 200 GB; neue Aufnahmen werden abgelehnt und gemeldet |
| Player: Sender nicht erreichbar, Ersatzquelle aktiv, Grenze erreicht, Strom stockt, Langsamer Start, Nicht abspielbar, Server ausgelastet | die Fehler- und Ladebilder aus 14 Abschnitt 4.11 und 4.12 |
| Verwaltung (nur Web): Speicher knapp, Speicher voll, Netzlaufwerk nicht eingehängt, EPG-Import fehlgeschlagen | Warnungen und Zustände in LA-01, LA-10, LA-12, LA-14 bis LA-16 |
| Aktionen (je Client) | einmalige Vorführungen, zum Beispiel «Erinnerung jetzt zeigen» in der Web-App |

**Ladezeiten im Player:** Das Konzept schlägt 0,5 s (Ladeanzeige), 3 s («Dauert länger als üblich …») und
15 s (Fehlerbild) vor (14 Fassung 2, 4.11). Damit man alles in wenigen Sekunden sieht, sind die Werte im Mockup
**verkürzt**: Ladeanzeige ab 0,5 s, «Dauert länger» ab 2,5 s, Fehlerbild nach 6 s. Ein schon empfangener
Sender ist nach 0,6 s da, ein neuer nach 1,8 s, «Langsamer Start» nach 4,5 s. Die Werte gelten in allen
drei Clients gleich (`LIVE_UI.LOAD`). Ein Fehlerschalter wirkt sofort auf den laufenden Sender.

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

Stand nach 14 Fassung 2 (Abschnitt 11) und 15 Fassung 2 (Abschnitt 8). «nachgezogen» heisst: 14/15 folgen
jetzt dem Mockup; «Mockup nachgezogen» heisst: das Mockup folgt jetzt 14/15.

| Client | Abweichung | Grund | Stand |
|---|---|---|---|
| Web | LS-21: Hinweis «zeigt alles» nur bei Profilen mit Altersgrenze | wie 14 1.3 | entspricht 14 |
| Web | LS-15: auch künftige Lücken ohne Programmdaten haben «Zeit aufnehmen» | konsequent zu «Ohne EPG» | in 14 nachgezogen |
| Web | LS-25, LD-10 nicht-modal, LD-09 Grundform, Bedienleiste blendet nach 4 s aus | wie 14 | entspricht 14 |
| Web | Programm-Überlagerung zeigt Details künftiger Sendungen als Seitenleiste | Details ohne den Player zu verlassen | in 14 nachgezogen (4.9) |
| Web | «Recht entzogen» mit eigener Adresse `#/player/revoked`; die Meldung bleibt, bis man den Player verlässt | nur Mockup; im Produkt Fehlerbild im Player, danach 404 | nur Mockup |
| Web | LS-14: geplante Termine eines Kinderprofils nur als Zahl | F-4 offen | in 14 als Zwischenstand; hängt an F-4 |
| Verwaltung | LA-05: Gruppen unter dem Sendernamen | Platz | in 15 nachgezogen |
| Verwaltung | LA-01: Zustand «Eingeschränkt» | aus den gestörten Sendern berechnet | entspricht 15 |
| Verwaltung | LA-05, LA-11: Tabellen nur mit Beispielsendern, Köpfe zählen auf 584 | nur Beispielsender im Einzelnen erfasst | nur Mockup |
| Verwaltung | LA-22, LA-26: nur das Neue gezeigt | der Rest bleibt | entspricht 15 |
| Verwaltung | LA-09 und LA-03: Einstellung je Quelle **«Vorladen für Sender dieser Quelle: an/aus»** mit dem Satz «Vorladen öffnet zusätzliche Verbindungen zu dieser Quelle. Ob dein Anbieter das zulässt, verantwortest du.»; keine Rechnung je Quelle, kein «Limit»; die Obergrenze vorgeladener Sender insgesamt bleibt eine Einstellung des Empfangs | Review 3 RM-01 (LTV-017): nichts, was wie eine Verbindungsverwaltung je Anbieter aussieht. 15 Fassung 2 nennt den Schalter nach `04` T-18 «weitere gleichzeitige Verbindungen erlaubt»; das Mockup zeigt ihn enger als reine Vorlade-Einstellung | **hängt an offener Frage `04` F-3 und F-9**; 15 3.3 und 3.9 an den Wortlaut des Mockups angleichen oder Marco entscheidet |
| Verwaltung | LA-15: «Werksvorgabe 1 Stunde, eingestellt 2 Stunden» | 04 14 | in 15 nachgezogen |
| Verwaltung | LA-03: feste Ziele im Heimnetz statt Schalter, Adressen nur Schema · Host · Port, Kopfzeilen nur Name und «gesetzt», Quellentest ohne Hinweis aus dem Heimnetz; LA-10 ohne Heimnetz-Schalter und ohne Kopfzeilen | 15 Fassung 2 | Mockup nachgezogen |
| Verwaltung | «Sofort beenden …» neben «Deaktivieren» (LA-05, LA-06); «zurückgestellt» (3sat) in LA-01, LA-09, LA-12; laufende Aufnahmen nur als Zahl | 15 Fassung 2, `04` T-29, T-17 | Mockup nachgezogen |
| Verwaltung | «Neue Sender sofort anbieten»: Werksvorgabe an; die Beispielquelle «Kabelanbieter Schweiz» hat es auf aus, deshalb bleiben Blue Zoom, Kanal 9 und CNN inaktiv mit Filter «Neu» | so bleibt die Beispielzahl von 16 Sendern für Zuschauer | entspricht 15 (Einstellung bleibt wählbar) |
| TV | OK zeigt die Bedienleiste, ← die Senderliste | Variante A | hängt an F-7 |
| TV | Senderliste im Player folgt der Herkunftsliste; «2 h»-Knöpfe im Raster | | in 14 nachgezogen |
| TV | kein LD-09; LS-21, LS-23, LS-24, LS-26 minimal | | entspricht 14 |
| Handy | «Programm» im Player als Blatt; «Details» beendet die Wiedergabe | | in 14 nachgezogen bzw. Grundsatz 6 |
| alle | LD-02/LD-03 ohne «ähnliche Titel», «Angesehene zuerst» und Schalter gegen doppelte Folgen; Zeitfenster, Wochentage, «nur Erstausstrahlungen», Ende der Wiederholung als «Vorschlag, hängt an Frage F-23»; LD-04 nur «Bis Sendungsende» und «Bis ich stoppe» (bei Zeitversatz «Ab jetzt aufnehmen (live)»); LD-05 Vorlauf 0, 5, 15, 30 Min.; Kinderaufnahme vom Hauptprofil abgespielt ohne Fortschritt | 14 Fassung 2, `03` DR-08, DR-21 | Mockup nachgezogen |
| alle | «Zuletzt gesehen» bis 20 Sender, mit «Liste leeren» | 14 3.1, DR-15 | Mockup nachgezogen |
| alle | Ladezeiten verkürzt (Abschnitt 4) | Vorführung | nur Mockup |

## 7. Bekannte Grenzen

- Die Glocke (LS-23) zeigt feste Beispielmeldungen. Meldungen, die im Mockup entstehen (abgelehnte
  Aufnahme bei vollem Kontingent), stehen im Zustand (`LIVE_SIM.notices()`), erscheinen aber nur als
  Meldung auf der Seite, nicht als neuer Eintrag der Glocke.
- «Viele Sender» wirkt im Raster der Web-App und des Fernsehers, nicht im Raster der Handy-App am Tablet.
- Die Verwaltung ändert die Beispieldaten direkt; «Alles zurücksetzen» stellt den Zustand der Zuschauer
  wieder her, Änderungen der Verwaltung erst ein Neuladen.
- Zahlen ohne Bezug zum Mockup-Zustand (Kennzahlen des Servers, Belegung anderer Konten) sind fest.
- Spracheingabe am TV, Push-Mitteilungen und echte Tastenkürzel des Betriebssystems sind nicht gespielt.
- Im Mockup-Ordner liegt ein Laufzeit-Ordner `.omc/` (Statusdateien der Agenten, git-ignoriert, nicht im Repo).
  Vor dem Weitergeben des Ordners löschen; ich habe ihn nicht gelöscht, weil das Löschen Marco überlassen ist.
- Echte Logos und Bilder sieht nur, wer den Ordner `shared/img/` hat (dieser Arbeitsbaum), nicht aus einem Klon.

## 8. Was geprüft wurde

Alle Läufe mit Chromium ohne Kopf (Playwright aus dem Haupt-Checkout), **ohne Dateiserver über `file://`**.

- **Syntax:** `node --check` für alle JavaScript-Dateien: ohne Fehler.
- **Daten:** Node lädt `shared/live-data.js` und `live-sim.js` und prüft Programmpunkte aus 0.1, Lücken,
  Aufnahme- und Planungsarten, die Serie «Tatort», Rechte, Zustände und Register. Belegung: Anna 82 GB,
  Luca 56.7 GB, Mia 3.3 GB, zusammen 142 GB in 31 gespeicherten Aufnahmen.
- **Alle Adressen:** jede Register-Adresse aller vier Einstiege, die Web-App in Desktop- und Handy-Breite, die
  Handy-App als Handy hoch und Tablet quer (212 Adressen). Mit Bildern (Schalter an): 0 Konsolenfehler,
  0 fehlgeschlagene Anfragen, 0 Seiten mit waagrechtem Überlauf, 0 Mal «undefined», «NaN» oder
  «[object Object]». Ohne `shared/img/` (umbenannt, Schalter aus): dieselben Werte, auch 0 Konsolenfehler.
- **Nichts vom Bereich ohne Recht:** Mit Luca und mit Anna, Recht «Live-TV» aus, prüft ein Skript Start,
  Suche, Benachrichtigungen, Einstellungen, Profile, Radio, eine Bibliothek, das Menü der Handy-App und die
  Leiste der TV-App in allen drei Clients auf die Wörter «Live-TV», «Aufnahme», «Sender», «Programm»,
  «Fernseh…», «Erinnerung», «Timeshift» (Text und Namen für Bildschirmleser, ohne Mockup-Leiste): 0 Treffer.
  Dasselbe ohne «Aufnahmen» (Wörter «Aufnahme», «aufnehmen») und ohne «Timeshift»: je 0 Treffer.
  Ausgenommen ist nur «Profile verwalten» des Hauptprofils (dort setzt es die Rechte).
- **Kontrast** (berechnete Farben auf dem tatsächlichen Grund, jede Register-Adresse): gedämpfter Text
  (`--tx2`, `--tx3`) Web mindestens 8:1 ausser einem Messfehler an der Zeitachse von LS-04 (die Beschriftung
  sitzt auf einem 1 px breiten Strich, gemessen wird der Strich), TV mindestens 8.91:1, Handy mindestens
  8.60:1; Rahmen ringsum mindestens 3.28:1 in allen drei Clients. Angehoben: Zeiten im TV-Raster, Genre und
  Restzeit der laufenden Zeile (Web), Tastenhinweis «Strg K», Rahmen von Karten, Menüs, Dialogen und
  Schalterzeilen am TV und am Handy.
- **Zustände:** 20 Seiten mit Player, Rastern, Aufnahmen und Verwaltung (Web, TV, Handy), je Seite jeder
  Schalter der Leiste «Zustände» und die vier Seitenzustände ein und aus, zusammen 480 Wechsel: 0 Fehler in
  der Konsole, 0 Mal «undefined» oder «NaN».
- **Von Hand bzw. mit Skript durchgespielt** (die Wege, die Review 3 nicht geprüft hatte): Neuanlage
  M3U-URL, Xtream und Einzelstrom (Multicast mit festen Zielen) in LA-03 bis «übernehmen» und Import; am TV
  jede Seite und jeder Dialog nur mit Pfeilen, OK und Zurück (jedes Element erreichbar, Zurück führt hinaus
  oder schliesst den Dialog; dabei behoben: Player-Leiste brach um, «Ton» und «Info» waren unerreichbar,
  ←/→ in LD-03 warf einen Fehler); am Handy Wischen nach oben und unten im Player und Ziehen der Zeitleiste
  (behoben: ein Wisch auf einem grossen Knopf wechselte den Sender nicht).
- **Tastatur in der Verwaltung:** LA-03, LA-05/LA-06, LA-11, LA-16 (Fokus bleibt im Dialog, Esc schliesst,
  Fokus zurück; Tab-Reihenfolge folgt der Seite).

**Nicht geprüft:** Firefox und Safari; echte Geräte (Android-Handy, Android TV); Bildschirmleser.

## 9. Änderungen gegenüber der vorigen Fassung

Fassung 1 → 2 (Review 3 und Nachzug nach 14/15 Fassung 2, 28. November 2026):

- **RM-01:** LA-09/LA-03 nur noch «Vorladen für Sender dieser Quelle» (Abschnitt 6).
- **RM-02:** Ohne Recht nichts vom Bereich: Mockup-Vermerke aus dem App-Bild entfernt (Start, Suche,
  Benachrichtigungen, Profile, Radio, unveränderte Seiten), Block «Live-TV» in LS-21 nur für das Hauptprofil,
  Fernbedienungstasten ohne Recht stumm.
- **RM-03:** «Leer» leert im Kern auch Aufnahmen, Planungen, Erinnerungen, Favoriten und «Zuletzt gesehen».
- **RM-04:** Kontrast gemessen und angehoben (Abschnitt 8).
- **RM-05:** LA-03 sagt nach fehlgeschlagenem Test, warum es nicht weitergeht, und führt zurück zu Schritt 2.
- **RM-06:** «Liste leeren» in allen drei Clients. **RM-07:** LA-01 meldet «Speicher voll», «Netzlaufwerk
  nicht eingehängt», «Speicher knapp» und «EPG-Import fehlgeschlagen»; die Pille der Seitenleiste folgt.
- **RM-08:** Zahlen stimmig: Warnungen überall aus `SIM.warningCount()` mit demselben Wortlaut; laufende
  Aufnahmen in LA-01, LA-12, LA-14 aus einer Funktion; Duplikate 3; Vorschau und Import gleich; LA-21
  «Live-TV-Wiedergaben»; «31 gespeicherte Aufnahmen»; Bibliothekszahlen in allen Clients gleich (Filme 1'846,
  Webradio 3).
- **RM-09** bis **RM-11**, **RM-14** bis **RM-16:** behoben (Handy «Erinnern», Weitere Termine, Marke «NEU»,
  Textfehler, gleiche Meldungen nur einmal, Meldungen am TV oben, Fehlerschalter wirken sofort).
- **RM-12:** Bilder nur nach Schalter; ohne Bilder keine Anfrage und kein Konsolenfehler. **RM-13:** `.omc/`
  nicht gelöscht (Abschnitt 7).
- **Nachzug** nach `MOCKUP-NACHZUG.md` und 14/15 Fassung 2 (Abschnitt 6, Zeilen «Mockup nachgezogen»); neue
  Beispieldaten: Startpuffer im Arbeitsspeicher, Timeshift-Deckel 50 GB, 3sat zurückgestellt, Obergrenze
  8 Eingänge und 4 vorgeladene, EPG alle 24 Stunden, Konto des Administrators mit Live-TV.
