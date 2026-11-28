# Konzept 28 · Review 3 Mockups

Stand: 28. November 2026 · Prüfer: oh-my-claudecode:verifier

Gegenstand: `docs/planning/mockups/28-live-tv/` (Web, Verwaltung, TV, Handy) und `docs/planning/28-live-tv/19-mockups.md`.
Massstab: Auftrag Abschnitt 13, LTV-014, Abschnitt 9 und 16, `14-oberflaeche.md`, `15-administration.md`, `LEITPLANKEN.md`.
Nur geprüft, nichts an den Mockups geändert. Eigener Dateiserver auf Port 8738 (am Ende beendet). Für den Test «ohne
`shared/img/`» lief kurz eine Kopie ohne Bilder auf Port 8739 (Kopie und Server sind entfernt).

## 1. Gesamturteil

**bereit mit Korrekturen.** Alle 121 Registereinträge sind gebaut, jede Adresse öffnet eine echte Seite, die wesentlichen
Abläufe lassen sich bedienen, es gibt keine Platzhalterseite und kein «undefined». Vor der Vorlage beim Auftraggeber sind zwei
Dinge zu beheben (RM-01, RM-02) und eine falsche Aussage in der Dokumentation zu berichtigen (RM-03).

Befunde: 2 hoch, 3 mittel, 11 niedrig (zusammen 16).

## 2. Befunde

### Hoch

**RM-01 · Verwaltung · LA-09 Empfang (`#/admin/live/reception?tab=strategy`) · Verbindungssteuerung je Anbieter**
- Schritte: Verwaltung › Live-TV › Empfang, Abschnitt «Vorsorglicher Empfang je Quelle» (auch in `web/js/admin-b.js` Zeile 140 bis 155).
- Erwartet: keine anbieterbezogene Verbindungsverwaltung (LTV-017; Abschnitt 16 «Separate IPTV-Anbieterlimits»; LEITPLANKEN:
  «auch kein vom Administrator eingetragenes Limit»; wer es für nötig hält, schreibt es als offene Entscheidung auf, nicht ins Konzept).
- Gesehen: Tabelle «Quelle · Art · Darf vorsorglich zusätzliche Verbindungen öffnen» mit Schalter je Quelle (Kabelanbieter «erlaubt»,
  Xtream «aus», Heimnetz «wird nie vorsorglich geöffnet»); dazu die Rechnung «höchstens 5 Verbindungen über das Nötige hinaus»
  und mehrfach «belegt dauerhaft eine Verbindung zur Quelle». Der Text sagt «keine Zahl, keine Verwaltung von Anbieterlimits»,
  funktional ist es eine Verbindungsfreigabe je Anbieter. `19-mockups.md` Abschnitt 6 nennt die Abweichung und hängt sie an Frage F-9.
- Bewertung: Grenzfall. Entweder ins Konzept (14/15, 04) als offene Entscheidung zurückstufen und im Mockup entfernen, oder dem
  Auftraggeber ausdrücklich als Entscheidungsfrage vorlegen. Nicht stillschweigend zeigen.

**RM-02 · Web · Profil ohne Recht «Live-TV» sieht Hinweise zum Bereich**
- Schritte: Zustände › Wer schaut = Luca › Rechte von Luca: «Live-TV» ausschalten. Dann `#/` (Start), `#/search?q=tatort`,
  `#/notifications`, `#/settings/profiles` öffnen.
- Erwartet: nichts vom Bereich sichtbar (14, 0 Grundsatz 1: kein Eintrag, keine Route, keinen Hinweis). Die Adressen `#/live…` sind
  korrekt «Nicht gefunden»; die Seitenleiste zeigt keinen Eintrag.
- Gesehen:
  - Start: Kasten «Unverändert durch Konzept 28 (Frage F-2) … Dieses Profil hat kein Live-TV, es gibt keinen Eintrag.»
    (Klasse `va-homenote`, ein Mockup-Vermerk, aber im App-Bild).
  - Suche: «Aufnahmen sind privat und erscheinen in der Suche nie.» (Klasse `va-fine`).
  - Benachrichtigungen: «Erinnerungen und Aufnahmen betreffen nur dich. Sie erscheinen nirgends sonst.»
  - Profile (als Luca): «Wechsle zum Hauptprofil, um Live-TV für Profile ein- oder auszuschalten.»
- Zu tun: Texte nur zeigen, wenn das Profil das Recht hat; Mockup-Vermerke aus dem App-Bild in die Leiste oder die Doku verlegen.

### Mittel

**RM-03 · alle Clients · Schalter «Leer» gilt nur für LS-01 bis LS-05**
- Schritte: Zustände › Zustand der Seiten = Leer, dann LS-08 Meine Aufnahmen, LS-11 Geplant, LS-12, LS-13 Erinnerungen, LS-14, LS-15.
- Erwartet: `19-mockups.md` 5.1 und README: «jede Seite (Schalter «Leer»)»; 14 Abschnitt 2 nennt eigene Leerzustände für LS-08 bis 12, 14
  und LS-13 «Keine Erinnerungen».
- Gesehen: diese Seiten bleiben gefüllt (18 Aufnahmen, 9 Termine, 3 Erinnerungen). Der Leerzustand ist im Code vorhanden
  (`viewer-b-2.js` Zeile 108, 161, 479), aber nur erreichbar, wenn man alle Einträge von Hand löscht. Verwaltung: LA-09 und LA-16
  reagieren nicht auf «Leer» (bei Einstellungsseiten vertretbar).
- Zu tun: Schalter «Leer» auch auf die Aufnahme-, Planungs- und Erinnerungsseiten wirken lassen oder die Doku auf «LS-01 bis LS-05 und
  Verwaltung» einschränken (Auftrag 13.1 «Leere Zustände» ist sonst nur teilweise gezeigt).

**RM-04 · TV · Programmraster (`tv/index.html#/live/guide`) · gedämpfter Text unter 8:1**
- Gemessen (berechnete Farben, Skript): Text `rgb(188,193,205)` auf Zellfläche `rgb(47,53,70)` = 6.78:1 (Beispiel «20:10» in der Zelle).
  Marcos Regel: gedämpfter Text mindestens 8:1 gegen seinen Grund. Web Sender-Seite: kleinster Wert 9.23:1; Rahmen 3.33:1 und 4.52:1
  (Regel 3:1 erfüllt).
- Zu tun: Zeit- und Nebentext in Zellen auf `--tx2`/`--tx` heben oder Zellfläche abdunkeln.

**RM-05 · Verwaltung · LA-03 Schritt 3 · Weiter nach fehlgeschlagenem Test ohne Rückmeldung**
- Schritte: Quelle hinzufügen › M3U-URL › Name und Adresse › Weiter › «Ergebnis des Tests» = Zugangsdaten falsch (oder Nicht erreichbar,
  Keine Senderliste, Kopierschutz, Heimnetz) › «Noch einmal testen» › «Weiter».
- Erwartet: 15 3.3: bei Fehler zurück zu Schritt 2 (Pfeil «Fehler → S2»), oder «Weiter» sichtbar gesperrt.
- Gesehen: «Weiter» sieht bedienbar aus, tut nichts und sagt nichts. Ausserdem ändert die Auswahl «Ergebnis des Tests» das Bild erst
  nach «Noch einmal testen» (nicht beim Wählen).

### Niedrig

- **RM-06 · Web · LS-01:** Reihe «Zuletzt gesehen» hat kein Reihenmenü «Liste leeren» (14 3.1 Punkt 5). Im Quelltext kein Treffer.
- **RM-07 · Verwaltung · LA-01 bei «Speicher voll»:** «Braucht Aufmerksamkeit» zeigt keinen Eintrag zum vollen Speicher; nur der Balken
  «Aufnahmen 4 TB von 4 TB» ändert sich. Die Seitenleisten-Pille bleibt «Speicher knapp».
- **RM-08 · alle · Zahlen widersprechen sich:** Warnungen: LS-01 Web «2 geplante Aufnahmen», LS-11 «3 Termine mit Warnungen», LS-12 «(4)»,
  TV-Start «4 geplante Aufnahmen». LA-01 «3 laufen auf 1 Sender», LA-14 «auf 2 Sendern», LA-12 SRF zwei 2 plus Das Erste 1.
  LA-07 Reiter «Duplikate 7» bei 3 Zeilen. Konto Keller «31 Aufnahmen», Profile zusammen 19 + 11 + 4. LA-03 Vorschau «2 Duplikate, 4
  fehlerhaft», danach im Importergebnis 0 und 0. LA-21 «Live-TV 5» bei 4 Zuschauern. Webradio 2 Sender (Web) gegen 3 (Handy), Filme 1840 gegen 1'846.
- **RM-09 · Handy · LS-06 Sendungsdetails:** «Erinnern» steht auch bei laufender Sendung (14 3.6: nur bei künftigen; Web macht es richtig).
  Die fünfte Aktion «Senderprogramm» ist am Rand halb abgeschnitten, ohne Hinweis auf Wischen.
- **RM-10 · Web · LS-06 Vergangene Sendung:** «Weitere Termine» listet die gezeigte Sendung selbst (Tagesschau Nacht 00:30–00:45) und nennt keinen Tag.
- **RM-11 · Verwaltung · LA-26:** Marke «NEU» hat grauen Text auf Orange (1.60:1, kaum lesbar).
- **RM-12 · Dokumentation:** `19-mockups.md` Abschnitt 8 behauptet «0 Konsolenfehler» auch ohne `shared/img/`. Gemessen: 16 Konsolenmeldungen
  «404» (Logos); Seiten bleiben ganz bedienbar (Initialen auf heller Fläche). Dazu: Echte Logos und Bilder sieht der Auftraggeber nur in diesem
  Arbeitsbaum; aus einem Klon des Repos nicht. In der Vorlage ausdrücklich sagen.
- **RM-13 · Ordner:** Im Mockup-Ordner liegt ein Laufzeit-Ordner `.omc/` (Status-Dateien, git-ignoriert, nicht im Repo). Vor dem Weitergeben löschen.
- **RM-14 · Textfehler:** LA-15 «eingestellt: 2 Std..» (doppelter Punkt); LA-03 Schritt 5 zeigt bei M3U-URL «Einzelströme werden nicht
  importiert.»; LA-12 ZDFinfo «Wiederverbindet (Versuch 3 von 5)» und «alle Quellen fehlgeschlagen» zugleich; LA-13 «nächster Versuch
  20:43» bei Uhr 20:50; Konten-Tabelle: nur die E-Mail ist ein Link, nicht die Zeile; Web LD-04 Auswahl «Aufbewahren» führt
  «Standard (bis du löschst)» und «bis du löschst» nebeneinander.
- **RM-15 · Web/TV · Meldungen:** Beim wiederholten Zurückspulen vor das Einschalten stapeln sich drei gleiche Meldungen über der Bedienleiste
  des Players; am TV überdeckt die Meldung «Aufnahme läuft bis 22:05.» die Hilfezeile im Programm.
- **RM-16 · alle · Schalter «Player»:** Fehlerzustände wirken erst beim nächsten Senderwechsel, nicht auf den gerade laufenden Sender.
  Wer den Schalter betätigt und im Player bleibt, sieht nichts (README: «im Player ansehen»).

## 3. Abdeckung Auftrag Abschnitt 13

Messung: Register (`index.html`) zählt 121 Einträge, alle «gebaut», 0 «offen», keine Platzhalterseite (Web 33, Verwaltung 25, TV 30, Handy 33).
Ich habe jede Seite mindestens geöffnet (Textdurchlauf, Überlauf- und «undefined»-Prüfung) und die Abläufe unten von Hand durchgespielt.
«Web» ist die Zuschauer-App (Adresse `web/index.html#…`), «V» die Verwaltung (`web/index.html#/admin…`).

### 13.1 Benutzeroberfläche (19 Zeilen)

| Auftragszeile | Seite (Kennung · Clients · Adresse) | gefunden und bedienbar |
|---|---|---|
| Live-TV-Startseite | LS-01 · Web, TV, Handy · `#/live` | ja |
| Senderübersicht | LS-02 · Web, TV, Handy · `#/live/channels` | ja (Suche, Filter, Liste/Kacheln, Herz, Menü ⋯) |
| Sendergruppen | LS-02 Filter «Gruppe» · `#/live/channels`; V LA-08 | ja |
| Favoriten | LS-03 · `#/live/channels/favorites` (Ordnen mit Pfeilen geprüft) | ja |
| Programmzeitschrift | LS-04 · `#/live/guide` (Tage, Uhrzeit, Gruppe, Jetzt-Linie) | ja |
| Sendungsdetails | LS-06 · `#/live/programs/p-zdf-1215` | ja |
| Live-TV-Player | LS-07 · `#/live/watch/srfzwei` | ja |
| Senderwechsel im Player | LS-07 (Bild↑/↓, C, Ziffern 1 1, Pfeiltasten TV) und LD-10 | ja |
| Timeshift-Bedienung | LS-07 (Anhalten, 10 s, «Zurück zu Live», Grenze «Vor 20:41 nicht verfügbar») | ja |
| Sofortaufnahme | LD-04 · `#/live/dialog/LD-04` (Web, TV, Handy) | ja |
| Geplante Aufnahme | LD-01, LD-03 | ja |
| Serienaufnahme | LD-02 | ja |
| Meine Aufnahmen | LS-08, LS-09, LS-10 (abspielen, Auswahl, Löschen LD-06 geprüft) | ja |
| Geplante Aufnahmen | LS-11 (Ändern, Abbrechen geprüft) | ja |
| Aufnahmekonflikte | LS-12, LD-07 | ja |
| Sendungserinnerungen | LS-13, LD-05 (anlegen, entfernen, rückgängig geprüft) | ja |
| Leere Zustände | Schalter «Leer» | **teilweise** (RM-03) |
| Ladezustände | Schalter «Lädt», Player-Phasen | ja |
| Fehlerzustände | Schalter «Fehler», Player-Fehler | ja (RM-16) |

### 13.2 Administration (19 Zeilen)

| Auftragszeile | Seite · Adresse | gefunden und bedienbar |
|---|---|---|
| Live-TV-Dashboard | LA-01 · `#/admin/live` | ja |
| Quellenverwaltung | LA-02 · `…/sources` (Aktualisieren mit Fortschritt, Testen, Aufklapper) | ja |
| M3U-Import | LA-03 Art M3U-URL, LA-04 (sechs Schritte bis «übernehmen» durchgespielt) | ja (RM-05) |
| Xtream-Quellenkonfiguration | LA-03 Art Xtream · `…/sources/src-xtream` (Ändern-Modus, Testen) | ja; Neuanlage nicht durchgespielt |
| Manuelle Streamkonfiguration | LA-03 Art Einzelstrom · `…/sources/src-mc` | ja; Neuanlage nicht durchgespielt |
| Senderverwaltung | LA-05 · `…/channels?tab=channels` (Filter, Suche, Auswahl, Massenbearbeitung) | ja |
| Sendergruppen | LA-08 · `…?tab=groups` (Dialog Neu/Bearbeiten) | ja |
| Senderbearbeitung | LA-06 · `…&edit=srf1` (alle Aufklapper zu, Speichern) | ja |
| Ersatzquellen | LA-06 «Quellen» (Rangfolge), LA-07 Zusammenführen (durchgespielt) | ja |
| Empfangsstrategien | LA-09 · `…/reception` | ja (RM-01) |
| Empfangszeitpläne | LA-09 Reiter Zeitpläne (Dialog Zeitplan) | ja |
| EPG-Verwaltung | LA-10 · `…/epg?tab=sources` (Jetzt laden geprüft) | ja |
| EPG-Zuordnungen | LA-11 · `…?tab=mapping` | ja |
| Aktive Streams | LA-12 · `…/streams` (Strom «Beenden» mit Rückfrage geprüft) | ja |
| Streamüberwachung | LA-12, LA-13 | ja |
| DVR-Einstellungen | LA-14 · `…/storage?tab=dvr` | ja |
| Timeshift-Einstellungen | LA-15 · `…?tab=timeshift` | ja |
| Speicherverwaltung | LA-16 · `…?tab=storage` (Dialog «Speicherort wechseln» in vier Schritten) | ja |
| Fehlerdiagnose | LA-13 · `…/diagnostics?channel=zdfinfo` (Jetzt prüfen) | ja |

### 13.3 Bestehende Seiten

| Auftragszeile | Seite · Adresse | gefunden und bedienbar |
|---|---|---|
| Hauptnavigation | LS-20 (Web, TV, Handy), LA-25 (Seitenleiste, Strg K) | ja |
| Benutzerberechtigungen | LA-22 · `#/admin/accounts/familie.keller%40example.ch` (drei Schalter, Kontingent, Hinweis, SaveBar) | ja |
| Profilverwaltung | LS-21 · `#/settings/profiles` (Schalter ändern, Speichern/Verwerfen) | ja |
| Kinderprofil-Einstellungen | LS-21 mit Hinweis «zeigt alles», LS-14 | ja |
| Serverübersicht | LA-20 · `#/admin` | ja |
| Aktive Wiedergaben | LA-21 · `#/admin/streams` («Private Aufnahme» ohne Titel) | ja |
| Weitere betroffene Seiten | LS-22 bis LS-26, LA-23, LA-24, LA-26, LA-27, LA-28 | ja (RM-11) |

Auftrag 13: 19 + 19 + 7 Zeilen; 44 ja, 1 teilweise (Leere Zustände), 0 nein. Bei den Zeilen Xtream und Manuelle Streamkonfiguration habe
ich nur Ändern-Modus und Test gesehen, die Neuanlage nur für M3U-URL.

### LTV-014 Aufzählungszeilen (72 Zeilen in sieben Seitentypen)

| Seitentyp | Zeilen | ja | teilweise | nein |
|---|---|---|---|---|
| Live-TV-Startseite | 8 | 8 | 0 | 0 |
| Senderübersicht | 10 | 10 | 0 | 0 |
| Programmzeitschrift | 10 | 10 | 0 | 0 |
| Sendungsdetailseite | 11 | 10 | 1 («Verfügbare Bilder»: ohne lokale Bilder nur Senderlogo-Fläche) | 0 |
| Live-TV-Player (16 Zeilen: Wiedergabe, Pause, Timeshift, Zurückspulen, Vorspulen, Live-Anzeige, Zurück zu Live, Senderwechsel, Jetzt/Danach, Aufnahme, Audio, Untertitel, Qualität, Vollbild, Lade- und Fehlerzustände) | 16 | 16 | 0 | 0 |
| Meine Aufnahmen | 8 | 8 | 0 | 0 |
| Geplante Aufnahmen | 9 | 9 | 0 | 0 |
| Summe | 72 | 71 | 1 | 0 |

### Abgleich mit der Abdeckungstabelle in `19-mockups.md`

5.1 bis 5.3 stimmen bis auf drei Stellen:
1. Zeile «Leere Zustände: jede Seite (Schalter «Leer»)» stimmt nicht (RM-03).
2. Abschnitt 8 «0 Konsolenfehler» auch ohne Bilder stimmt nicht (RM-12).
3. Abschnitt 3.2 Punkt 13 «Zustände → Verwaltung «Speicher voll» zeigt Warnungen in LA-01» trifft nur auf die Ressourcenzeile zu (RM-07).
Alle sonstigen Routen der Tabellen habe ich geöffnet; keine führt auf «Nicht gefunden» oder eine Platzhalterseite. «121 Seiten» stimmt.
Das Register zeigt nirgends «offen»; der Filter «nur offene» liefert null Zeilen.

## 4. Ausschlussliste (Auftrag Abschnitt 16 und 14 Abschnitt 8)

Aktiv gesucht, von Hand und per Textsuche im Quelltext (`grep` nach Catch-up, Archiv, Mediathek, verpasst, Bild-im-Bild, PiP, Multiview,
Altersfreigabe, Jugendschutz, FSK, geteilt, Download, Werbung, Suchwort, Benutzergruppe, Abo, VOD, Qualität, Verbindungslimit).

| Verbotenes | Befund |
|---|---|
| Catch-up, «von vorne» bei Live, Archiv, Zurückspulen vor das Einschalten | nicht vorhanden. Vergangene Sendungen haben nur Info («Vorbei. Slouchcat bietet keine verpassten Sendungen an.»). «Von vorne» gibt es nur im Aufnahme-Player (14 3.10 erlaubt es) |
| Bild-im-Bild, Mehrfachbild, laufende Vorschau in Listen | nicht vorhanden (keine `video`/`canvas`; die «Vorschau» im Programm zeigt nur Text) |
| Altersfilter an Sendern oder Sendungen | nicht vorhanden; Luca (ab 16) und Mia sehen alle Sender; Hinweis «prüft keine Altersfreigaben» |
| Geteilte Aufnahmen, «für alle Profile aufnehmen» | nicht vorhanden; LS-14 nur für das Hauptprofil (Luca: «Nicht gefunden») |
| Aufnahmequalität wählbar | nicht vorhanden (LA-14 «Originalqualität»); die Qualitätswahl im Live-Player ist LTV-014 |
| Verbindungslimits, Abo-Verwaltung je Anbieter | keine Zahl, kein Abo; **aber** Schalter «Darf vorsorglich zusätzliche Verbindungen öffnen» je Quelle (RM-01) |
| VOD und Serien aus Xtream | nicht vorhanden; Text «übernimmt nur Live-TV» |
| Werbeerkennung, KI, Empfehlungen, Suchwort-Aufnahme | nicht vorhanden. Serienaufnahme «nach Titel» ist eine Serienregel, kein Suchwort |
| Benutzergruppen | nicht vorhanden; Sendergruppen sind ausdrücklich «keine Rechte» |
| Übernahme in die Bibliothek, Download von Aufnahmen | nicht vorhanden |
| Aufnahmen auf Start, Suche, Sammlungen | nicht vorhanden (Start und Suche zeigen keine Aufnahme; Hinweistext siehe RM-02) |

Leitplanken: Hauptprofil sieht Aufnahmen von Profilen mit Altersgrenze (LS-14 mit Luca und Mia, Löschen und Abspielen angeboten) und
sieht keine anderen; Kontingent voll: LD-04 «Kein Platz frei» abgelehnt, geplante Aufnahme wird gemeldet; kein Altersfilter; der
Administrator sieht keine Titel («Private Aufnahme»). Erfüllt.

## 5. Vorgaben

| Vorgabe | Ergebnis |
|---|---|
| als Beispieldaten gekennzeichnet | ja: Fusszeile «Beispieldaten, simulierte Wiedergabe» auf jeder Seite, auf dem Fernsehbild und in der Leiste |
| keine echten URLs oder Zugangsdaten | ja: `live-data.js` nur `iptv.example.net`, `xtream.example.com`, `region.example.net`, `epg.example.org`; im ganzen Ordner kein weiterer Host ausser dem Platzhalter `http://host:port` |
| keine Bilder im Repo | ja: `git ls-files … \| grep -E '\.(png\|jpe?g\|webp\|svg\|gif\|ico)$'` = 0 Treffer; `shared/img/` per `.gitignore` ausgeschlossen |
| ohne `shared/img/` bedienbar | ja (Kopie ohne Bilder: Web und TV mit Initialen auf Fläche); 16 harmlose 404-Meldungen (RM-12) |
| Aufruf ohne Anpassung | plausibel: nur klassische `<script src>`, kein `fetch`, kein Modul. Das Öffnen per Doppelklick (`file://`) habe ich nicht selbst geprüft, nur über einen Dateiserver |
| Startanleitung | README und `19-mockups.md` Abschnitt 2 stimmen überein: Doppelklick auf `index.html`, Alternative mit `python3 -m http.server 8728`, Tasten der Fernbedienung, Zurücksetzen. Für einen Nicht-Entwickler genügt Abschnitt 2 Punkt 1 bis 3 |

## 6. Marcos Gestaltungsregeln (Stichprobe)

- Aufklapper zu: in allen 32 Web- und Verwaltungsadressen ist ausser dem Gruppenkopf «Live-TV» in LA-23 nichts offen; LA-06 und LA-03 (Ändern)
  zeigen jeden Aufklapper mit Zusammenfassung in der Kopfzeile («Xtream, demo, Passwort gespeichert»). Erfüllt.
- Anlegen gleich Ändern im selben Formular: LD-01 «Aufnahme planen»/«Aufnahme ändern» gleicher Dialog; LD-02 «Serienaufnahme ändern»; LA-08 neu und bearbeiten
  gleicher Dialog, ausser bei Gruppen aus Quellen (begründet). LA-03: Neu als Assistent, Ändern als Aufklapper (so in 15 3.3). Erfüllt.
- Medienauswahl links: Web-Seitenleiste, Handy-Menü, TV-Leiste. Erfüllt.
- Kontrast (berechnete Farben): Web Sender-Seite gedämpfter Text 9.23:1, Rahmen 3.33:1 und 4.52:1. Verstösse: RM-04 (6.78:1) und RM-11 (1.60:1).
- Logos und Sendungsbilder ohne Rahmen: im Web, TV und Handy nur helle Fläche ohne Rand. Erfüllt.

## 7. Übereinstimmung mit dem Konzept (25 Stichproben aus 14 und 15)

| # | Aussage im Konzept | Mockup |
|---|---|---|
| 1 | Ohne Recht fehlt der Bereich, Adresse zeigt «Nicht gefunden» (14 0.1) | ja (Adresse); Hinweistexte siehe RM-02 |
| 2 | Kein Altersfilter, Profile mit Altersgrenze sehen alle Sender | ja |
| 3 | LS-14 nur Hauptprofil | ja |
| 4 | LS-01 Hinweisband nur mit Recht Aufnahmen | ja |
| 5 | LS-01 Reihen Jetzt, Gleich, Zuletzt, Meine Sender, Genre | ja |
| 6 | LS-01 «Zuletzt gesehen» mit Menü «Liste leeren» | **nein** (RM-06) |
| 7 | LS-02 Filter Gruppe, Land, Sprache, Kategorie; Menü ⋯ | ja |
| 8 | LS-04 Tageswahl, Uhrzeit-Sprünge «Hauptabend 20:15», Jetzt-Linie, TeleZüri «Keine Programmdaten» | ja |
| 9 | LS-06 «Erinnern» nur bei künftigen Sendungen | Web ja, Handy **nein** (RM-09) |
| 10 | LD-01 laufende Sendung: «Eine Aufnahme ab Sendungsbeginn gibt es nicht.» | ja |
| 11 | LD-01 Vorlauf, Nachlauf, Aufbewahren, Grösse | ja |
| 12 | LD-02 Serie erkennen, Sender, Zeitfenster, Behalten, «gleiche Folge wird übersprungen» | ja |
| 13 | LD-04 «Bis Sendungsende / 30 Minuten / 1 Stunde / bis ich stoppe (höchstens 4 Stunden)» | ja |
| 14 | LD-05 Zeitpunkt zum Beginn bis 15 Min. vorher | ja |
| 15 | LD-06 laufende Aufnahme: «Nur stoppen, behalten» | ja |
| 16 | LD-10 Rückfrage bei Zeitversatz, «Nicht mehr fragen (dieses Gerät)» | ja |
| 17 | Timeshift beginnt beim Einschalten, nie davor | ja |
| 18 | 4.11 Ladestufen (Mockup verkürzt, in 19 §4 genannt) | ja |
| 19 | 4.12 Fehlertexte ohne Adressen und Anbieternamen im Player | ja |
| 20 | LA-22 Schalter Aufnahmen/Timeshift nur mit Live-TV; Hinweis «Laufende Wiedergaben enden …» | ja |
| 21 | LA-21 Aufnahme-Wiedergabe als «Private Aufnahme» ohne Titel | ja |
| 22 | LA-16 Speicherort wechseln in vier Schritten mit Prüfungen | ja |
| 23 | LA-06 Aufklapper zu mit Zusammenfassung; Rangfolge der Ersatzquellen | ja |
| 24 | LA-03 Fehler führt zurück zu Schritt 2 | **nein** (RM-05) |
| 25 | LA-09 keine Anbieterlimits | **teilweise** (RM-01) |

### Abweichungen vom Konzept, die `19-mockups.md` nicht nennt

- Handy LS-06: «Erinnern» auch bei laufender Sendung (RM-09).
- Web LS-01: kein Reihenmenü «Liste leeren» (RM-06).
- Alle Clients: «Leer» wirkt nur auf LS-01 bis LS-05 (RM-03).
- Verwaltung LA-03: «Weiter» nach Fehlertest (RM-05).
- Verwaltung LA-01: kein Eintrag zum vollen Speicher (RM-07).
- Mockup-Anmerkungen im App-Bild für Profile ohne Recht (RM-02).

Scope-Creep: nichts Verbotenes darüber hinaus. Mehr als 14/15 verlangen (harmlos, nicht in 19 genannt): LA-26 «NEU»-Marken, Mockup-Vermerke
«Unverändert durch Konzept 28», LD-Vorschauseiten (`#/live/dialog/LD-xx`, Seite «Dialog öffnen») als Hilfskonstrukt.

## 8. Was ich nicht prüfen konnte

- Doppelklick über `file://` (nur Dateiserver), Firefox, Safari, echtes Android-Gerät, Bildschirmleser.
- TV: nicht jede der 30 Seiten mit der Fernbedienung durchgetastet. Gesehen und bedient: Start, Sender, Player (Zappen, Senderliste über dem Bild,
  Bedienleiste, Pause, Aufnahme R, Guide G), Erinnerungen. Für alle 30 Adressen mit Skript geprüft: Seite hat Text, kein «undefined», genau ein
  markierter Fokus, ausser dem Player (Bedienleiste erscheint erst mit OK). Das Zurück-Verhalten und eine Fokusfalle sind nicht lückenlos
  nachgewiesen: Zurück folgte dem Browserverlauf meines Durchlaufs, ein sauberer Neustart-Test fehlt.
- Handy: Wischgesten (nur Klick und Skript), Tablet quer nur per Überlaufprüfung, nicht je Seite angesehen. Ergebnis der Überlaufprüfung: kein
  Überlauf ausser den Querreihen auf der Start-Seite und der Aktionszeile in LS-06 (RM-09).
- Web Tablet-Breite nicht angesehen; Handy-Breite für alle 32 Web-Adressen per Skript (kein Überlauf, kein «undefined») und LS-04 per Bildschirmfoto.
- Verwaltung: Neuanlage Xtream, Einzelstrom und Datei nicht durchgespielt; Tastatur in der Verwaltung nicht geprüft.
- Kontrast: gemessen nur Web Sender-Seite, Web und Verwaltung über alle Adressen (Textfarben), TV-Programm. Handy-App nicht gemessen. Ein
  Ausreisser «00:00» im Programmraster (1.31:1) ist vermutlich ein Messfehler durch überlagerte Flächen und sah im Bildschirmfoto lesbar aus.
- Quelltext nicht Zeile für Zeile gelesen; die Ausschlussliste ist per Textsuche und Bedienung geprüft.
