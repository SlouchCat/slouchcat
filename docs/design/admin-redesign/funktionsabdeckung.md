# Funktionsabdeckung des Verwaltungs-Redesigns

Stand 15. November 2026, Zweig `claude/admin-redesign`, geprüft gegen den Ausgangsstand `main`
`55b72c9a` (Release 1.38.0). Bedingung des Auftrags: Keine bestehende Funktion, Option oder
Einstellmöglichkeit geht verloren.

## Ergebnis

**Alle 876 aufgenommenen Funktionen sind im neuen Stand vorhanden.** 759 belegt ein Test, 117 sind
am Code nachgewiesen und haben keinen Test; diese 117 hatten schon vor dem Umbau keinen. Keine Zeile
steht auf `LÜCKE`, `LÜCKE Test` oder `unklar`.

| Seite | Zeilen | ok | ok, ohne Test |
|---|---:|---:|---:|
| Bibliotheken (LIB) | 87 | 77 | 10 |
| Metadaten (MET) | 46 | 30 | 16 |
| Hörbuch-Quellen (ABS) | 25 | 19 | 6 |
| Pflege (MNT) | 39 | 37 | 2 |
| Titelseite, Editor, Löschen (ITM) | 69 | 59 | 10 |
| Podcasts (POD) | 56 | 49 | 7 |
| Podcast-Seite (PDP) | 16 | 9 | 7 |
| Radio (RAD) | 77 | 53 | 24 |
| Konten (ACC) | 44 | 40 | 4 |
| Geräte (DEV) | 14 | 13 | 1 |
| Streams (STR) | 17 | 17 | 0 |
| Startseite (HOM) | 21 | 19 | 2 |
| Spotlight (SPT) | 33 | 32 | 1 |
| User-Empfehlungen (PCK) | 13 | 7 | 6 |
| Sammlungen (COL) | 23 | 23 | 0 |
| Meine Medien (MYM) | 20 | 16 | 4 |
| Ankündigungen (ANN) | 24 | 24 | 0 |
| Statistiken (STA) | 14 | 13 | 1 |
| Sicherung (BKP) | 22 | 21 | 1 |
| Navigation und Rahmen (NAV) | 32 | 30 | 2 |
| Übersicht (OVW) | 66 | 62 | 4 |
| Hintergrundjobs (JOB) | 43 | 40 | 3 |
| Integrationen (INT) | 51 | 46 | 5 |
| Benachrichtigungen (NOT) | 24 | 23 | 1 |
| **zusammen** | **876** | **759** | **117** |

Was sich an den Funktionen geändert hat (Spalte «Aenderung» der Matrix):

| Änderung | Zeilen | Bedeutung |
|---|---:|---|
| keine | 744 | gleiche Stelle, gleiche Bedienung |
| verschoben | 57 | andere Stelle oder Seite, gleiche Funktion (vor allem Übersicht → Server, Zahlen der Seitenleiste → Bereichsreiter, Zustand der Integrationen → Kacheln) |
| Aussehen | 39 | gleiche Stelle und Bedienung, anderes Aussehen (Abzeichen statt Punkt, Karte, gerahmte Leiste) |
| umgebaut | 35 | Bedienung geändert (Integrationen als Kacheln mit Ansicht, Startseite mit zwei Karten, Jobs mit Zustandsfilter) |
| entfällt | 1 | der Hilfe-Knopf im Seitenkopf; sein Text steht offen unter dem Titel |

## Die Dateien

| Datei | Inhalt |
|---|---|
| `functional-inventory.csv` | Bestand vor dem Umbau: 876 Zeilen, je Funktion Route, Komponente, Bedingung, API, Zustände, Tests, Entsprechung im Mockup |
| `inventory/<teil>.md`, `.csv` | die vier Teile der Aufnahme mit Aufbau, Suchparametern, Abfragen, Fallen und dem Abgleich jeder Mockup-Zeile D001 bis D069 |
| `funktionsabdeckung.csv` | die Matrix: je Funktion alte Stelle, neue Stelle, Art der Änderung, Testbeleg, Status |
| `abdeckung/<teil>.csv`, `.md` | Befunde der vier Prüfer am Stand `850a31b6` |
| `abdeckung/nachpruefung.md` | Nachprüfung der Befunde und des Deltas am Stand `c7e9b88a` |
| `abdeckung/vorpruefung.py`, `zusammenfuehren.py` | maschinelle Vorprüfung (Dateien und Tests je Zeile seit dem Ausgangsstand) und Zusammenführung zur Matrix |
| `abweichungen.md` | Abweichungen vom Mockup mit Grund |
| `screenshots/` | 55 Bilder: 25 Seiten und die Suchpalette in 1440×900 und 390×844, drei Seiten in 768×1024 |

## Wie geprüft wurde

1. **Aufnahme vor dem Umbau.** Vier Agenten haben jede Verwaltungsseite samt Unterseiten, Dialogen,
   Zuständen, Tastaturbedienung und Verhalten auf dem Handy am Code aufgenommen (`BRIEF-INVENTAR.md`).
   492 der 876 Zeilen zeigt das Mockup nicht; sie bleiben trotzdem.
2. **Maschinelle Vorprüfung nach dem Umbau.** Je Zeile: Sind die genannten Dateien seit `55b72c9a`
   unverändert, stehen die genannten Tests noch da? 494 Zeilen waren unverändert, 382 gingen zur
   Einzelprüfung.
3. **Unabhängige Prüfung.** Vier Prüfer, die am Umbau nicht beteiligt waren, haben die 382 Zeilen
   einzeln am Code nachvollzogen, bei den übrigen jede zehnte nachgesehen und den ganzen Unterschied
   jeder Seite auf Verluste ohne Inventarzeile durchsucht (gestrichene oder gelockerte Prüfungen,
   entfernte Texte, entfernte `aria`-Angaben). Ergebnis: keine Funktion fehlt; vier Stellen waren
   schlechter als vorher, zwei Tests fehlten.
4. **Korrektur.** Behoben in den Commits `e608af6b` bis `c7e9b88a`:
   - Verweise auf die Schlüssel der Anbieter (Metadaten, Hörbuch-Quellen, Spotlight) landeten auf der
     Kachelliste der Integrationen statt bei den Schlüsseln; sie führen jetzt in die Ansicht
     `?item=keys`. Der Hinweis eines Integrationsfehlers führt zum betroffenen Dienst.
   - Der Löschdialog einer Bibliothek nannte «Verwaltung › Sicherung»; neu «Verwaltung › System ›
     Sicherung». Ebenso zwei Texte des Einrichtungsassistenten.
   - Streams: «Serverlast» hat wieder eine Überschrift für die Überschriften-Navigation.
   - Radio: Ein Sender, der über den Seitenkopf angelegt wird, öffnet den Reiter «Eigene Sender».
   - Neue Tests: leeres Server-Protokoll, Kopfknopf «Neue Sicherung», Editor und Vorschau der
     Startseite neben- und untereinander, Zahl am Bereichsreiter «Pflege».
5. **Nachprüfung.** Ein fünfter Prüfer hat jeden Befund am Stand `c7e9b88a` nachvollzogen und den
   Unterschied seit `850a31b6` (Feinschliff und Korrektur) auf neue Verluste durchsucht: keiner.

## Offen und Grenzen

- **Was ein Mensch ansehen sollte:** Die Prüfung ist eine Prüfung am Code und an Tests. Wie sich die
  Seiten anfühlen (Wege, Höhe des Seitenkopfs, Lesbarkeit auf dem Handy), entscheidet die Abnahme
  auf CT 190.
- **117 Funktionen ohne Test.** Sie sind vorhanden, aber wie vor dem Umbau nur am Code belegt. Der
  Umbau hat ihre Dateien bis auf 30 nicht berührt (Spalte «Aenderung»).
- **HOM-021:** Der Test belegt, dass Editor und Vorschau der Startseite neben- und untereinander
  stehen, nicht die genaue Mindestbreite der Tabelle (36 rem).
- **A-3 auf der Übersicht:** Ein neuer Fehler steht wie bisher binnen 10 s im Protokoll (jetzt auf
  «Server»). Den Hinweis auf der Übersicht prüft der Test erst nach dem Wechsel dorthin.
- **Schon vor dem Umbau offen** und nicht Teil des Auftrags (aus dem Inventar): Der Anlegen-Dialog
  zeigt bei der Art «Hörbücher» den Video-Hinweis (LIB-052); die Toleranz der Hörbuch-Zuordnung
  bietet 1, 3 und 5 %, der Server nimmt 1 bis 10 % (ABS-011); «Verwaiste Benutzerdaten» und
  «Entfernte Titel» zeigen höchstens 200 Einträge; der Text `admin.home.update.body` sagt «Slouchcat
  installiert nichts selbst», obwohl es den Knopf «Update installieren» gibt.
