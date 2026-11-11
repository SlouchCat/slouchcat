# ADR 0030: Radio-Angebot statt ganzer Katalog

- Status: angenommen (11. November 2026); die Entscheide folgen Marcos Antworten RF-1 bis RF-11 (Mockups
  vom 10. November 2026) und OB-1 bis OB-11, die Zahlen der Messung B0 (`docs/perf/19-radio.md`)
- Quelle: Konzept `docs/planning/19-podcasts-und-radio.md` Teil B (Abschnitte B1 bis B15, Anhang B);
  Entscheidungsprotokoll Frage 26 (Zeilen vom 10. und 11. November 2026)
- Ergänzt ADR 0025 (Webradio: Katalog, Weiterleitung mit Zeitversatz). Weiterleitung, Zeitversatz,
  Hör-Sitzungen, Hörzeit und die stabile Kennung der Sender bleiben, wie ADR 0025 sie festlegt.
- Vertrag 1.28.0 (Stufe B1, brechend für die Verwaltung, additiv für die App), Migration 00048 (beim
  Baubeginn vergeben; das Papier nannte vorläufig 1.29.0 und 00048)
- Harte Regeln: 2 und 12 (der Server entscheidet, was im Angebot ist; der Client fragt nur ab), 4
  (Logodateien nur über `paths`), 5 (Migration nur vorwärts), 7 (`x-visibility` je Operation)

## Kontext

Seit 1.17.0 zeigt die App den ganzen Katalog von Radio Browser, rund 56'000 Sender nach dem
Zusammenführen (CT 190, 11. November 2026: 55'953 Sender in 228 Ländern, 1'099 ohne Land). Die Verwaltung
konnte Länder einschränken (`radio_config.countries`), einzelne Sender ausblenden und auf dem Reiter
«Einordnung» Schlagworte von Hand Filterwerten zuordnen. Marco findet das «eine Flut an Kanälen»: auch das
Einschränken bleibe unübersichtlich, die Zuordnung von Hand wolle er gar nicht pflegen (R-1 bis R-7).

Logos holte der Server für jeden Sender, den eine Liste zeigte. Mit dem ganzen Katalog als Liste heisst
das: jeder Sender, durch den jemand scrollt, löst Abrufe bei fremden Webseiten aus. Marco will, dass der
Server Logos und weitere Angaben erst holt, wenn der Admin einen Sender ins Angebot nimmt (RF-8).

## Entscheid

1. **Die App zeigt nur das Angebot.** Das Angebot besteht aus gewählten Ländern mit Modus «Alle» oder «Nur
   markierte» und aus Ausnahmen je Sender («markiert», «demarkiert»). Ein Sender ist im Angebot nach der
   ersten zutreffenden Regel: demarkiert nein, markiert ja, eigener Sender ja, Land mit «Alle» ja, sonst
   nein. Die Ausnahme hängt am Sender, nicht am Land; wechselt ein Sender das Land, nimmt er sie mit. Ein
   Sender ausserhalb des Angebots ist für Profile unsichtbar: Liste, Zähler, Filter, Startseite fehlen,
   Station, Hören, Titelangaben, Bild und «Favorit setzen» antworten mit 404. Favoriten bleiben in der
   Datenbank.
2. **Der Katalog bleibt ganz,** täglich abgeglichen wie bisher, aber nur als Auswahlliste der Verwaltung.
3. **Das Angebot liegt als unveränderliche Momentaufnahme im Speicher** (Länder, Ausnahmen, Schalter
   «Nicht erreichbare weglassen», Versionsnummer). Jede Anfrage schlägt je Sender in zwei Tabellen nach;
   kein Lesen von `radio_config` je Anfrage, kein Neubau des Index. «Übernehmen» ersetzt die
   Momentaufnahme ganz und beendet Hör-Sitzungen auf Sendern, die das Angebot verlassen.
4. **Änderungen sind relative Schritte** («Land wählen», «diese Sender demarkieren», «Gezeigte markieren»
   mit Suchtext). Die Verwaltung schickt sie zur Vorschau und zum Übernehmen; der Server rechnet die Folgen
   und löst Suchtexte selbst auf. Übernehmen trägt die Version des Stands, auf dem der Entwurf beruht; eine
   abweichende Version gibt 409, und der Entwurf lässt sich auf dem neuen Stand neu rechnen, ohne dass eine
   der beiden Änderungen verloren geht.
5. **Nach dem Update und auf einer neuen Installation ist das Angebot leer** (OB-2). Die frühere
   Ländereinstellung wird nicht übernommen. Ausgeblendete Sender, auch eigene, werden demarkiert, damit sie
   draussen bleiben, wenn der Admin ihr Land wählt.
6. **Keine Zuordnung von Hand** (R-1). Regeln des Admins, Filterwerte von Hand und ihre Schlösser
   entfallen; die Filter entstehen nur noch aus der mitgelieferten Regeldatei `radio/rules/default.json`.
   Der erste Start nach dem Update leitet die Filterwerte aller Sender neu ab.
7. **Logos und weitere Angaben nur für Sender im Angebot** (RF-8, OB-3). Die Grunddaten des Katalogs lädt
   der Abgleich für alle Sender; alles, was der Server zusätzlich abruft (Logo, ab Stufe B2 Wikidata), holt
   er nach dem Hinzufügen. Die Verwaltung zeigt ein Logo nur, wenn es schon im Cache liegt, und löst nie
   einen Abruf aus.
8. **Logo-Abrufe gedrosselt:** 4 je Sekunde für alle Logo-Aufträge zusammen (OB-11, vorher 2) und je Host
   höchstens ein Auftrag gleichzeitig. Ist der Host belegt, gibt der Auftrag mit «später wieder» zurück,
   ohne einen Versuch zu verbrauchen und ohne einen Worker zu blockieren.
9. **Anbieter eines Senders** ist in Stufe B1 die registrierbare Domain seines besten Streams (Public
   Suffix List), gezeigt in der Verwaltung und in der App (OB-8).
10. **Kontinente** kommen aus einer eingebetteten Tabelle nach den Unterregionen von UN M49, zusammengefasst
    zu sechs Gruppen (Europa; Nord- und Mittelamerika mit Karibik; Südamerika; Asien; Afrika; Ozeanien mit
    der Antarktis). Kosovo (`XK`) zählt zu Europa.
11. **Wikidata (Stufe B2)** liefert nur Eigentümer und Gründung für die Verwaltung, keine Logos (OB-5). Die
    Daten von Wikidata stehen unter CC0; Logos auf Wikimedia Commons haben je eigene Lizenzen und werden
    darum nicht übernommen.

## Folgen

- Die App wird übersichtlicher, aber nach dem Update leer, bis der Admin Länder wählt. Die Versionshinweise
  und die Zustandsleiste der Verwaltung sagen es.
- Die Verwaltung bekommt den Reiter «Angebot»; der Reiter «Einordnung» und die Filterfelder im Editor
  fallen weg. Wer eigene Regeln gepflegt hatte, verliert sie mit dem Update (OB-4).
- Sender ohne gültiges Land lassen sich nicht über ein Land wählen; wer einen will, legt ihn als eigenen
  Sender an (OB-7).
- Logos fremder Sender werden nur noch für das Angebot geholt; die Last auf fremde Server sinkt und ist
  planbar. Ein Land mit 500 Sendern braucht bei 4 Abrufen je Sekunde einige Minuten, bis die Logos da sind.
- Bilder gelöschter Sender räumt der Abgleich weg. Ein Sender hat ein geholtes Logo nur, solange er im Angebot
  ist (Marcos Entscheid vom 11. November 2026): verlässt er es, gehen Dateien und Logo-Zustand, ein eigenes
  Hochladen bleibt; kommt er zurück, wird das Logo neu geholt. Gespeichert werden WebP in 96, 256 und 512 Pixeln
  und PNG nur in 96 Pixeln; grössere PNG entstehen bei Bedarf aus dem WebP (Messung B0).
- Der Vertrag der Verwaltung bricht (1.28.0): entfallen sind Regeln, Filterfelder, `hidden`, `countries` und
  `hideBroken` in den Einstellungen. Die App bleibt verträglich.

## Verworfen

- **Angebot A «Pakete»** (nach Land, Sprache, Genre, Beliebtheit) und **Angebot C «Regeln»** («Nimm alle
  Sender mit …», «Lass weg …»): Marco hat B «Länder zuerst» gewählt (RF-2).
- **«Meine Sender statt Katalog»** (das Angebot entsteht aus dem, was Profile über die Suche hinzufügen):
  verworfen, weil die App dann trotzdem den ganzen Katalog durchsuchen müsste.
- **Vorgaben nach Sprache oder Nachbarländern** (RF-1, R-4): Slouchcat wird weltweit angeboten.
- **Ein Suchfeld über alle Länder** in der Verwaltung (RF-9): einzelne Sender gehen über ihr Land.
- **Den Entwurf im Client rechnen:** Der Client kennt die Regel nicht (Regel 12), und «Gezeigte markieren»
  über Hunderte Treffer ginge mit Seiten zu 100 nicht.
- **Logos aus Wikidata/Commons:** Lizenz je Datei, kleiner Gewinn (Schweiz: 53 Logos bei 81 Treffern,
  fast alle schon über Katalog oder Webseite abgedeckt).
