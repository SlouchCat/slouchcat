# ADR 0031: Externe Quellen für Hörbücher

- Status: vorgeschlagen (10. November 2026); die Entscheide 1 und 2 sind Marcos Antworten OD-3 und OD-4,
  die Entscheide 3 bis 8 folgen aus der Messung D0 und warten auf Marcos Wort im Pull Request
- Quelle: Konzept `docs/planning/19-podcasts-und-radio.md` Teil D (Abschnitte D6.1 bis D6.5, D9 Stufe D0,
  D11 OD-3 und OD-4, Kritiker-Review D12 Befunde 1, 11, 12, 20); Messung `docs/perf/19-hoerbuecher.md`
- Vertrag 1.30.0 (Stufe D3, additiv), Migration 00049 (vorläufige Nummern des Papiers)
- Harte Regeln: 10 (keine Secrets in Logs: Schlüssel für Google Books), 12 (Entscheid im Server);
  Entscheid 13 (Quellen je Ordner)

## Kontext

Hörbücher haben bis 1.23.x nur die Quelle `local`: Titel, Autor, Sprecher, Reihe und Beschreibung stammen
aus den Dateien und aus `metadata.json` von Audiobookshelf (`identifiers[RootAudiobook]` und
`fillers[RootAudiobook]` sind `local`). Marcos 107 Bücher tragen zu 1 % eine ASIN und zu 2 % eine ISBN;
Autoren und Reihen stehen in mehreren Schreibweisen. Teil D des Konzepts 19 will Hörbücher aus Anbietern
ergänzen und zuordnen, wie Filme und Serien aus TMDB.

Es gibt keine offizielle, offene Schnittstelle für Hörbücher. Die Recherche (D6.1) kennt: den **Katalog von
Audible** (`api.audible.de/1.0/catalog/products`, ohne Schlüssel, **Nutzungsbedingungen verbieten das
automatische Auslesen**, inoffiziell, kann jederzeit brechen), **Audnexus** (liefert Audibles Daten je ASIN,
ein Betreiber, selbst betreibbar), die **Deutsche Nationalbibliothek** (SRU, CC0), **Apple Books**
(iTunes Search), **Open Library** und **Google Books** (mit Schlüssel).

Marco hat am 10. November 2026 entschieden: **alle Quellen samt Audible ab Werk an** (OD-3, gegen die
Empfehlung des Papiers, Audible wegen der Bedingungen auszulassen) und **automatisch zuordnen für «Alle
ohne Zuordnung»** (OD-4, gegen die Empfehlung «Aus»). Die Messung D0 hat danach die Zahlen geliefert, auf
denen die übrigen Entscheide beruhen.

## Entscheid

1. **Audible ist ab Werk eingeschaltet, im Wissen um das Risiko** (OD-3). Der Katalog von Audible und
   Audnexus stehen als Quelle `audible` in der Vorgabe jedes Hörbuch-Ordners. Marco hat das Risiko
   angenommen: Audibles Bedingungen verbieten, soweit die Recherche (D6.1) es sagt, das automatische
   Auslesen; die Schnittstelle ist inoffiziell und kann ohne Vorwarnung wegfallen oder Slouchcat
   aussperren. Nicht geprüft und keine Rechtsberatung für die Schweiz; den Wortlaut der Bedingungen hat die
   Messung nicht gelesen. Jeder Administrator kann `audible` in der Liste eines Ordners ausschalten; ohne
   Audible ordnen die übrigen Quellen weiter zu, aber ohne Länge nur zur Prüfung (Entscheid 4).

2. **Vorgabe je Hörbuch-Ordner `local`, `audible`, `dnb`, `applebooks`** (OD-3), für neue Ordner und, beim
   ersten Start nach dem Update, für bestehende. **Open Library und Google Books sind umgesetzt, stehen aber
   nicht in der Liste.** Messung: Audible 92 % Treffer und als einzige Quelle mit Länge; DNB 85 % ohne Länge
   (bei 82 % der Treffer), Apple 87 % ohne Länge; Open Library 43 % und 0 zusätzliche Bücher; Google Books
   ohne Schlüssel 429 mit Kontingent 0. `local` bleibt immer zuerst: die Dateien stehen vor jedem Anbieter
   (Entscheid 13).

3. **Automatisch zugeordnet wird nur mit passender Länge**, und «eindeutig» wird **nach** der Länge geprüft:
   Aus den Treffern mit passendem Titel und Autor bleiben die mit Länge innerhalb der Toleranz (ab Werk
   **3 %**); ist es genau einer, gilt das Buch als `matched`. Treffer mit gleichem Titel und gleicher Länge
   (±1 Minute) zählen als einer (zwei ASIN derselben Aufnahme). Messung: 73 % der Bücher so, gegenüber 64 %,
   wenn zuerst Eindeutigkeit über alle Treffer verlangt wird; die Reihennummer stimmte bei 33 von 33
   prüfbaren Zuordnungen. Mehrere Treffer in der Toleranz oder keiner: `review` mit den drei besten.

4. **Quellen ohne Länge ordnen nie automatisch zu.** Apple Books, Open Library und die DNB (bei 82 % der
   Treffer) liefern keine Länge; ihre Treffer sind `review` mit Kandidaten und füllen leere Felder erst
   durch die Zuordnung im Editor, nicht im automatischen Lauf, ausser ein Buch ist schon von Audible
   zugeordnet. Folge: Bei Marcos Bestand landen rund 29 von 107 Büchern in der Liste «Prüfen» (20 mit
   Audible-Treffern, deren Ausgabe sich nicht festlegen lässt, 9 ohne Treffer; 4 davon in keiner Quelle).

5. **Die automatische Zuordnung fragt die Quellen nacheinander und hört bei `matched` auf.** Audnexus
   folgt einem Audible-Treffer sofort (ISBN, Genres, Reihe), DNB und Apple nur für Bücher, die Audible nicht
   zugeordnet hat. Das spart rund 70 % ihrer Abrufe und senkt die Laufzeit des ersten Laufs über 107 Bücher von
   13 auf 7 Minuten (5'000 Bücher: rund 1,8 Stunden statt 5,4). Der Preis: Zugeordnete Bücher ohne ISBN von
   Audnexus (16 von 97) behalten die Lücke, bis jemand im Editor die DNB wählt.

6. **Kennung zuerst, mit Regionswechsel.** Eine ASIN oder ISBN geht vor dem Titel an jede Quelle. Gilt sie
   bei Audible `.de` oder Audnexus mit `region=de` nicht (Messung: leerer Rumpf, 404
   `REGION_UNAVAILABLE`), folgen `.com`, `.co.uk` und Audnexus ohne `region`, bevor sie als unbekannt
   gilt.

7. **Höflichkeit gegenüber den Anbietern** (D6.5, durch die Messung bestätigt): ein eigener Client mit
   Drossel je Host (Audible und DNB 1 je Sekunde, Audnexus 60 je Minute, Apple 20 je Minute, Open Library
   1 je Sekunde), User-Agent `Slouchcat/<Version> (+https://github.com/SlouchCat/slouchcat)`, Antwort
   höchstens 5 MB, Suchergebnisse 7 Tage im Speicher. **Bei diesen Takten gab es in 676 Abrufen keine
   Antwort 403 oder 429.** Schutz bei Störung: 3 Antworten 403, 429 oder HTML statt JSON hintereinander
   pausieren die Quelle 6 Stunden; betroffene Bücher werden `error` und es wird nichts geschrieben.
   Audible antwortet von CT 190 aus (20 von 20, gleiche Treffer wie vom Entwicklungsrechner).

8. **Vereinheitlichte Cover und Texte bleiben Werke der Verlage.** Slouchcat speichert Cover verkleinert im
   eigenen Cache und zeigt sie nur den Konten der Installation (D6.1). Der Schlüssel für Google Books
   (`googlebooks_api_key`) liegt verschlüsselt in den Einstellungen und steht auf der Redaction-Liste
   (Regel 10).

## Folgen

- Stufe D3 baut `metadata.BookProvider` mit den fünf Umsetzungen (`audible` mit Audnexus, `dnb`,
  `applebooks`, `openlibrary`, `googlebooks`) und die Zuordnung nach den Punkten 3 bis 6; die Vorgabe der
  Liste kommt in Migration 00049.
- **Risiko von Audible, das Marco trägt:** Das Projekt liefert unter einer freien Lizenz (AGPL) einen
  Client mit, der Audibles Katalog ohne deren Erlaubnis abfragt und dies ab Werk tut. Fällt die
  Schnittstelle weg oder sperrt Audible die Adresse einer Installation, bleibt Slouchcat voll benutzbar:
  `local`, DNB und Apple ordnen zur Prüfung zu, nur die automatische Zuordnung mit Länge entfällt (73 % der
  Bücher). Wer das Risiko nicht tragen will, nimmt `audible` aus der Liste des Ordners; die Seite «Hörbücher: Quellen»
  (D6.6) zeigt, welche Quellen je Ordner gelten.
- Audibles Bedingungen und etwaige Rechtsfolgen sind nicht geprüft. Vor einem Release, das Audible ab Werk
  anschaltet, sollte Marco den Wortlaut der Bedingungen selbst lesen oder prüfen lassen.
- Die Messung D0 gilt für 107 deutschsprachige Bücher und die Titelregel des Messskripts. Bei einem
  Bestand mit vielen englischen oder seltenen Titeln sinkt der Anteil von Audible; die Entscheide 3 bis 5
  gelten trotzdem, nur die Zahlen ändern sich.
- 5 % Toleranz ordnete in der Messung 6 weitere Bücher ohne beobachteten Fehler zu; die Einstellung
  «Toleranz» auf der Seite «Quellen» erlaubt es später.
- Die Zeile «Reihen und Sammlungen» (D4.2) erreicht bei 5'000 Büchern p95 8 ms (Ziel 100 ms) und braucht
  keinen Zwischenspeicher.

## Verworfen

- **Audible auslassen** (Empfehlung des Papiers): von Marco abgelehnt (OD-3), weil ohne Audible keine
  Quelle eine Länge liefert und die automatische Zuordnung fast ganz wegfällt.
- **Alle Quellen für jedes Buch abfragen:** 4,65 statt 2,9 Abrufe je Buch, ohne dass die Zuordnung
  besser wird; Apple (20 je Minute) bestimmt sonst die Laufzeit.
- **Quellen ohne Länge automatisch zuordnen** (eindeutiger Titel und Autor genügen): Apple wäre bei 60 % der
  Bücher «eindeutig», aber ohne Prüfung der Ausgabe (Lesung oder Hörspiel, Sprecher); die Länge ist das
  einzige Merkmal, das beide trennt.
- **Open Library und Google Books in der Vorgabe:** Open Library bringt in der Messung kein einziges Buch
  zusätzlich, Google Books braucht einen Schlüssel, den ein Haushalt nicht hat.
