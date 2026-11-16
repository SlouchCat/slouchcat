# Abweichungen vom Mockup

Stand 15. November 2026, Zweig `claude/admin-redesign`. Das Mockup
(`handoff/mockup/Slouchcat_Verwaltung_V2.html`) ist die Gestaltungsvorlage. Wo die Umsetzung davon
abweicht, steht hier der Grund. Zwei Regeln erklären fast alles:

1. **Keine Funktion geht verloren.** Was der Bestand kann und das Mockup nicht zeigt, bleibt.
2. **Nichts wird erfunden.** Was das Mockup zeigt und der Code nicht kann (Beispielfelder,
   Beispielaktionen), wird nicht gebaut; die Einstufung jeder Mockup-Zeile D001 bis D069 steht in
   `inventory/*.md`, Abschnitt «Abgleich mit dem Mockup».

## Rahmen

| Mockup | Umsetzung | Grund |
|---|---|---|
| Eigene Kopfleiste, keine Kopfzeile der App | Kopfzeile der App mit Zusatz «Control Center», Brotkrumen und «Bereich suchen» | Glocke, Profilwechsel und Logo bleiben von jeder Verwaltungsseite erreichbar |
| Marke oben in der Seitenleiste | Marke in der Kopfzeile über der Seitenleiste | gleiche Stelle auf dem Bildschirm; die Kopfzeile bleibt ein Bauteil |
| Seitenleiste ohne Rückweg | «Zurück zur App» zuoberst | bestehende Funktion |
| Demo-Punkt «Demo-Modus», Stern-Knopf | entfallen | keine Entsprechung im Produkt |
| Handy: Menü-Knopf öffnet die sechs Einträge | Knopf «Verwaltung › Bereich / Seite» öffnet ein Blatt mit allen Seiten nach Bereichen | jede Seite bleibt ein Tippen entfernt; der Menü-Knopf der Kopfzeile öffnet weiter das Menü der App |
| Suche «Bereich suchen» mit `⌘ K` | gleich, dazu «Zurück zur App» und «Mediensuche» als Einträge; Kürzel `Strg K` bzw. `⌘ K` | die Mediensuche stand vorher als Verweis in der Kopfzeile |
| Karte «Optionen & Arbeitsabläufe» auf jeder Seite | entfällt | Gerüst des Prototyps: Liste seiner Detailkonzepte. Die Funktionen stehen auf den echten Seiten |
| Einleitung immer ganz sichtbar | höchstens zwei Zeilen, «Mehr» zeigt den Rest | einige Einleitungen sind lang (Radio, Konten); der Seitenkopf bleibt knapp |
| Seitentitel «Cat’s Empfehlungen», «Mitteilungen», «Sicherungen», «Webradio», «Konten & Profile», «Medienpflege», «Aktive Streams», «Startseite gestalten» | «User-Empfehlungen», «Ankündigungen», «Sicherung», «Radio», «Konten», «Pflege», «Streams», «Startseite» | Namen des Produkts, entschieden und auch auf der Zuschauerseite verwendet |
| Zahlen an den Einträgen der Seitenleiste | Pille am Bereichseintrag nur für das, was einen Blick braucht; alle Zahlen der Seiten an den Bereichsreitern | mit sechs Einträgen gibt es keinen Platz für sechs Zahlen; keine Zahl entfällt |

## Übersicht und Server

| Mockup | Umsetzung | Grund |
|---|---|---|
| Kennzahl «Speicherplatz 72 %» als eine Zahl | zwei Werte, Konfiguration und Cache | Entscheid Konzept 11, F-4: immer beide Pfade |
| «Online», «Hardwareprüfung aktiv» im Kurzstatus | Version, Laufzeit, Updates, «x von y eingerichtet», Webhook-Meldungen, Fehlerzahl | für «Online» und «Hardwareprüfung» gibt es keine Daten; der Live-Zustand steht am Titel |
| Drei Schnellzugriffe | fünf, einer je Bereich | die Bereiche sind die Navigation; drei wären willkürlich |
| «Scan erneut ausführen» in der Hinweisliste | Verweis auf die Seite | im Bestand ist der Hinweis ein Verweis, keine Aktion |
| «Neustart» in der Karte «Serverzustand» | in der Karte «Updates & Neustart» | so ordnet es die Mockup-Zeile D068 |
| Hinweis «Daten und Cache nicht doppelt zählen» | entfällt | widerspricht F-4 |
| «Dateiwächter Grenze» als Eingabe, «Diagnose kopieren», «Details anzeigen» (D067) | Anzeige und sysctl-Befehl zum Kopieren | das Limit setzt der Host |
| «Aktivität ansehen» als längere Liste (D069) | die sechs Einträge, die die API liefert | keine neue Datenquelle |
| Einrichtungsband fehlt | eigenes Band über der Übersicht | bestehende Funktion (Konzept 22) |

## Medien

| Seite | Mockup | Umsetzung | Grund |
|---|---|---|---|
| Bibliotheken | «Verwalten →» am Zeilenende | die Zeile klappt wie bisher über den Namen auf | ein zweiter Knopf für dieselbe Klappe wäre ein Tab-Stopp mehr je Zeile |
| Bibliotheken | Suchfeld «Bibliothek suchen …» | entfällt | im Bestand gibt es keine Suche; die Liste hat Reiter nach Art |
| Bibliotheken | «Dateien auf Datenträger ebenfalls löschen» (D004), «Keyframe-Vorschau aktivieren» (D002), «Automatisch nach neuen Dateien suchen» (D001) | Bestand: Bibliothek löschen entfernt nie Dateien; «Vorschau beim Spulen» mit vier Modi; Überwachung nur als Anzeige | Bauplan 7.1 |
| Metadaten | «Änderungen speichern» im Kopf | klebende Speichern-Leiste unten | Bestand, mit Teilerfolg über mehrere Geltungen und Tastaturtest |
| Metadaten | «Sprachfallback verwenden», «Freigaben neu lesen», «Bereits vorhandene Daten aktualisieren» | entfallen | Beispiele ohne Basis |
| Pflege | «Neu prüfen» im Kopf, «Bearbeiten →» je Zeile, Suche, «Auswahl ignorieren», «Endgültig bereinigen» | Bestand: Reiter je Problemart, Titel als Verweis, Massenaktionen in der Filterleiste | es gibt keine seitenweite Aktion; die übrigen sind Beispiele ohne Basis |
| Hörbuch-Quellen | unter «Bibliotheken» (D005) | eigene Seite, eigener Reiter unter Medien, Adresse `/admin/metadata/audiobooks` | Bestand |
| Podcasts | «Feed prüfen», «Maximal gespeicherte Folgen», «Heruntergeladene Dateien löschen» (D023) | entfallen | ohne Basis; Dateien entfernt der nächste Abgleich |
| Radio | «Sender hinzufügen», «Logo-URL», «Pufferdauer» und «Relay-Limit» als Auswahl | «Eigenen Sender hinzufügen» im Kopf; Einstellungen wie im Bestand | Namen und Felder des Produkts |
| Titelseite | fehlt im Mockup | gleiche Muster (Kopf mit Aktionen, Karten, Abzeichen), alle 69 Funktionen | bestehende Seite |

## Nutzer & Geräte

| Seite | Mockup | Umsetzung | Grund |
|---|---|---|---|
| Konten | «Öffnen →» je Zeile | Zeile öffnet über die E-Mail; am Zeilenende das Menü «Weitere Aktionen» | Bestand |
| Konten | Einladung mit E-Mail, Rolle, Ablaufzeit (D029); Rolle als Feld (D030); Profile fremder Konten (D031); Zustellung je Konto (D032) | Bestand: Einladungslink, sieben Tage; Rolle als Menüpunkt des Eigentümers; nur eigene Profile | Bauplan 7.1 |
| Geräte | «Details anzeigen», «Erneut prüfen» | entfallen | ohne Basis |
| Streams | «Aktualisieren», «Details →», Filter «Qualität» | entfallen; die Seite fragt alle 5 s neu | ohne Basis |

## Startseite

| Seite | Mockup | Umsetzung | Grund |
|---|---|---|---|
| Startseite | Pfeile und Schalter je Bereich, «Vorschau öffnen» | Platzwahl, «Ausblenden» und Zustand je Zeile; Vorschau steht immer daneben | Bestand (D037, D038); ein Knopf für eine sichtbare Vorschau wäre leer |
| Spotlight | Quelle «Automatisch/Manuell», «Titel suchen», Vorschau-Fläche (D039, D040) | Bestand: Trakt-Liste mit sieben Arten und allen Einstellungen | Bauplan 7.1; das Mockup zeigt die Seite kaum |
| User-Empfehlungen | «Regel hinzufügen», Filter, «Empfehlung ansehen» | entfallen | ohne Basis |
| Sammlungen | Freitextsuche, «Aktualisieren» | entfallen | ohne Basis |
| Ankündigungen | Tabelle; veröffentlichte Mitteilung bearbeiten (D049) | Liste neben den Einzelheiten; veröffentlichte bleiben unveränderbar | Bestand |

## Automatisierung

| Seite | Mockup | Umsetzung | Grund |
|---|---|---|---|
| Hintergrundjobs | Kopfaktion «Fehler anzeigen» | Reiter «Fehlschläge» mit Zahl | Bestand; ein zweiter Weg zum selben Reiter wäre doppelt |
| Hintergrundjobs | Kachel «Geplant» fehlt | «N geplant» unter «Wartend» | die Zahl bleibt |
| Hintergrundjobs | nur Kennzahlen, Filter, Liste | dazu «Läuft gerade» mit Arbeiter-Pools, Gruppenreiter, Banner pausierter Jobs | Bestand |
| Integrationen | Kacheln «Metadaten-Anbieter» usw.; Einstellungen als Dialog | Namen des Produkts («Schlüssel der Anbieter»); Ansicht in der Seite mit `?item=` | Deeplinks, Entwürfe und Meldungen bleiben über den Wechsel erhalten |
| Integrationen | «Anbieter aktivieren», Telegram «Chat-ID», editierbarer VAPID-Schlüssel | entfallen | ohne Basis |
| Benachrichtigungen | globale «Testnachricht senden», Hinweis «noch nicht gespeichert» | Testnachricht und Speichern je Kanal | Bestand |

## System

| Seite | Mockup | Umsetzung | Grund |
|---|---|---|---|
| Statistiken | Filter nach Wiedergabemodus, «Aktualisieren» | entfallen | ohne Basis |
| Sicherung | «Mit Passwort verschlüsseln» als Schalter (D065), Schritt «Import prüfen» | Bestand: mit Schlüsseln immer verschlüsselt, ohne nie | Bauplan 7.1 |
| Sicherung | «Sicherung erstellen» im Kopf | «Neue Sicherung» im Kopf öffnet den Reiter «Neue Sicherung» | Name des Produkts |
