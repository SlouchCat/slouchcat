# Messwerte: Konzept 19 Teil D «Hörbücher», Stufe D0

Konzept `docs/planning/19-podcasts-und-radio.md` Teil D (D6 Quellen, D9 Zeile D0, D12 Befund 22), ADR 0031.
Gemessen am 10. November 2026 an Marcos 107 Hörbüchern. Die Messung ist reine Dokumentation: Sie ändert
keinen Code des Servers und schreibt nichts auf das NAS, auf CT 190 und in Marcos Instanz.

| Frage der Zeile D0 | Antwort |
|---|---|
| Treffer je Quelle | Audible 92 %, Apple Books 87 %, DNB 85 %, Open Library 43 %; Google Books ohne Schlüssel 0 % (Abschnitt 3) |
| Eindeutig und mit passender Länge | Audible allein liefert Längen: **64 %** streng (ein Treffer, Länge ±3 %), **73 %** mild (ein Treffer in der Toleranz). DNB, Apple, Open Library: **0 %**, weil keine Länge da ist |
| Abfragen je Buch | Vorgabe-Quellen zusammen **4,65**; mit «weiter nur, wenn nicht zugeordnet» **2,9** (Abschnitt 4) |
| Laufzeit eines ersten Laufs über 107 Bücher | **13 Minuten** nacheinander, **7 Minuten** wenn die Hosts getrennt laufen (Apple bestimmt die Zeit) |
| Audible von CT 190 aus | **Ja**: 20 von 20 Abfragen mit 200 und denselben Treffern wie vom Entwicklungsrechner |
| Zeile «Reihen und Sammlungen» bei 5'000 Büchern | **p95 8 ms** (Ziel unter 100 ms), bei 20'000 Büchern 29 ms (Abschnitt 6) |

## 1. Die 107 Bücher

Gelesen read-only unter `/mnt/nerdnas/medien/Audiobookshelf/Hörbücher`: 107 Ordner mit `metadata.json`.
Die Länge kommt aus den Audiodateien (`ffprobe`), bei den **18 Ordnern ohne Audiodatei** aus den Kapiteln
der `metadata.json`; bei den 89 Büchern mit Datei stimmen beide in jedem Fall überein (Abweichung unter
1 %). Die Bibliothek von Slouchcat zeigt nur die 89 mit Datei; **die Messung nimmt alle 107 als Hauptzahl
und nennt die Werte der 89 in Abschnitt 3**, sie weichen höchstens um wenige Punkte ab.

Was die Dateien über die Bücher sagen, bestimmt die Treffer mit:

- Autor: bei allen vorhanden, aber in mehreren Schreibweisen («J K Rowling», «J.K. Rowling»,
  «J. R. R. Tolkien»); bei zwei Büchern kein Autor, sondern ein Stück Titel («Perry Rhodan NEO 075»,
  «Sebastian Fitzek-Der Nachbar»).
- ASIN bei **1** Buch, ISBN bei 2 (wie in D3). Alle anderen Bücher sind nur über Titel und Autor zu finden.
- Titel mit Anhängseln («(Hörbuch mit voller Besetzung)», «Live und ungekürzt», «(Ungekurezt)») und
  Schreibfehlern («Gefangende von Azkaban», «Silmarillon»).

## 2. Methode

- **Skripte** in `docs/perf/19-hoerbuecher/` (Python 3, nur Standardbibliothek). `books.py` liest die
  Bücher, `fetch.py <Quelle>` holt die Antworten, `best.py` wählt je Buch die ASIN für Audnexus,
  `analyze.py` und `analyze2.py` rechnen. `books.json` und die Rohantworten liegen **nicht** im Repository.
- **Anfrage je Buch und Quelle** wie D6.3 sie braucht: 1. Versuch mit Titel und erstem Autor wie in den
  Dateien; **nur wenn er keinen Treffer bringt**, ein 2. Versuch mit dem bereinigten Titel (Klammern und
  «- …» weg), bei Audible und DNB ohne Autor. Audible: Katalog `api.audible.de` mit 10 Ergebnissen;
  Zweitversuch auf `api.audible.com` für die 10 Bücher ohne Treffer. Audnexus: ein Abruf je ASIN des besten
  Audible-Treffers (`region=de`); es gibt dort keine Suche. DNB: SRU `tit=…` und `per=<Nachname>`, MARC21.
  Apple: iTunes Search, `country=ch`, `media=audiobook`. Open Library: `search.json`.
- **Treffer** = ein Ergebnis, dessen Titel (ohne Füllwörter, Akzente, Satzzeichen; Ähnlichkeit ab 0,85 oder
  enthalten) **und** dessen Autor (ein Nachname) zum Buch passt. **Eindeutig** = genau ein solches Ergebnis.
  **Länge passt** = Abweichung höchstens 3 % von der Länge der Dateien.
  **Automatisch, streng** = eindeutig und Länge passt (so steht es in D6.3 Punkt 3).
  **Automatisch, mild** = genau ein Treffer liegt innerhalb der Toleranz, weitere Treffer ausserhalb stören
  nicht. Die Titelregel ist meine, nicht die spätere des Servers; sie ist grosszügiger als eine exakte
  Gleichheit und strenger als eine reine Stichwortsuche.
- **Höflichkeit:** ein Abruf je Sekunde und Host (Apple alle 3,2 s = 18,9 je Minute, Grenze im Papier 20;
  Audnexus 60 je Minute), User-Agent
  `Slouchcat-D0-measurement/1.0 (+https://github.com/SlouchCat/slouchcat; one-off research run …)`, keine
  Wiederholung, Abbruch nach drei Antworten 403 oder 429. Alle Hosts liefen gleichzeitig, jeder in seinem
  eigenen Takt. Insgesamt **676 Abrufe** für die Messung der Treffer, dazu 20 von CT 190 und 6 für Kennungen.
  **Keine einzige Antwort 403 oder 429 und keine HTML-Seite statt JSON**, ausser Google Books ohne Schlüssel
  (Abschnitt 3).
- **Nicht gemessen:** Cover-Abrufe (nur geprüft, dass ein Cover-Host von CT 190 aus antwortet, 200 in
  84 ms), die Qualität der Texte, Quellen, die der Server später anders anfragt als ich.

## 3. Treffer je Quelle

107 Bücher. «Länge geliefert» heisst: der Treffer trägt eine Länge, mit der sich vergleichen lässt.

| Quelle | Treffer | eindeutig | Länge geliefert | Länge ±3 % | automatisch streng | automatisch mild |
|---|---|---|---|---|---|---|
| **Audible** (Katalog, `.de`) | **98 (92 %)** | 73 (68 %) | 97 (91 %) | 87 (81 %) | **68 (64 %)** | **78 (73 %)** |
| **DNB** (SRU) | 91 (85 %) | 3 (3 %) | 19 (18 %) | 13 (12 %) | 0 | 8 (7 %) |
| **Apple Books** (`ch`) | 93 (87 %) | 64 (60 %) | 0 | – | 0 | 0 |
| **Open Library** | 46 (43 %) | 35 (33 %) | 0 | – | 0 | 0 |
| Audnexus (je ASIN von Audible) | 97 von 97 | – | 91 | bei allen 91 gleich wie bei Audible | – | – |
| Google Books, ohne Schlüssel | **0**: Antwort 429, Kontingent «0 je Tag und Projekt» | | | | | |

Die 89 Bücher mit Audiodatei: Audible 93 %, DNB 90 %, Apple 90 %, Open Library 43 %; automatisch streng
66 %, mild 76 %.

**Warum «eindeutig» so selten ist.** Audible führt dasselbe Buch oft mehrfach: Lesung und Hörspiel, zwei
Sprecher, gekürzte Fassung, zwei ASIN für dieselbe Aufnahme («Frühstück mit Kängurus»: zwei Treffer, beide
739 Minuten). Harry Potter hat je Band drei bis vier Ausgaben. DNB nennt jedes Buch dazu als gedruckte
Ausgabe, E-Book und Hörbuch (von 91 Treffern sind 75 Hörbücher, davon 35 eindeutig). Die **Länge löst das
meiste auf**: Wo Audible mehrere Treffer hat, bleibt bei 10 Büchern genau einer in der Toleranz.
Fasst man Treffer mit gleichem Titel und gleicher Länge (±1 Minute) zusammen, steigt «mild» von 78 auf 80.

**Toleranz** (Audible, ein Treffer in der Toleranz, 107 Bücher):

| Toleranz | 1 % | 2 % | 3 % | 5 % | 10 % |
|---|---|---|---|---|---|
| automatisch mild | 78 | 77 | **78** | 84 | 78 |

Bei 3 % und bei 5 % habe ich die Reihennummer von Audible mit der aus den Dateien verglichen, wo beide
vorhanden sind: **33 von 33** (3 %) und **37 von 37** (5 %) stimmen. 5 % ordnet 6 Bücher mehr zu, ohne einen
beobachteten Fehler; 10 % verliert wieder, weil dann zwei Ausgaben in die Toleranz fallen. Bei den Büchern
ohne Reihe lässt sich die Richtigkeit nicht so prüfen.

**Die 9 Bücher ohne Audible-Treffer** (davon 4 in keiner Quelle):

| Buch | Ursache | andere Quellen |
|---|---|---|
| «Eine neue Erde» (Autor «Perry Rhodan NEO 075») | falscher Autor in den Dateien | keine |
| «Das Känguru-Manifest: Live und ungekürzt» | Titelzusatz, kein Ergebnis | keine |
| «FKKAuDiOBooK» (Autor «Sebastian Fitzek-Der Nachbar») | Titel und Autor unbrauchbar | keine |
| «Harry Potter -03- und der Gefangende von Azkaban» | Schreibfehler | keine |
| «Die Känguru-Offenbarung: Live und ungekürzt» | Titelzusatz | DNB |
| «Mit 50 Euro um die Welt» | nicht bei Audible | DNB (Buch), Open Library |
| «Das Silmarillon», «Die Seele will frei sein 1», «Unterwegs in der Weltgeschichte …-4» | Schreibweise | Apple |

Die Lücken liegen also zu mehr als der Hälfte **bei den Dateien, nicht bei den Quellen**. Der Editor aus D1
(Titel oder Autor korrigieren, dann suchen) ist der Weg für diese Bücher.

**Abdeckung nacheinander** (mindestens ein Treffer, 107 Bücher): Audible 98 (92 %), dazu DNB 100 (93 %), dazu
Apple 103 (96 %), dazu Open Library 103 (unverändert). **Open Library bringt nichts hinzu**, DNB 2 Bücher,
Apple 3. Kein Treffer in irgendeiner Quelle: 4 Bücher.

**Felder des besten Treffers** (wo vorhanden, von den Treffern der Quelle):

| Quelle | Beschreibung | Reihe | Verlag | ISBN | Cover |
|---|---|---|---|---|---|
| Audible (98) | 98 | 50 | 98 | – | 98 |
| Audnexus (97) | 97 | 42 | 97 | **81** | 97 (dazu Genres 97) |
| DNB (91) | 0 | 15 | 91 | **75** | 0 |
| Apple (93) | 93 | 0 | 76 | 0 | 93 |
| Open Library (46) | 0 | 0 | 43 | 44 | 36 |

Audnexus ist der einzige Weg zu Genres und ISBN von Audible; Apple liefert Beschreibung und Cover für die
Bücher ohne Audible-Treffer, DNB ISBN und Verlag.

**Kennung zuerst** (Test mit dem einen Buch mit ASIN und ISBN, «Die Wiederkehr des Königs»):

- Audible-Katalog `…/products/B00TLKT600` auf `.de`: Antwort 200, aber **nur ein leerer Rumpf** (ASIN,
  keine Felder): die ASIN gehört zu einer anderen Region.
- Audnexus mit `region=de`: **404 `REGION_UNAVAILABLE`**; ohne `region` 200 mit allen Feldern.
- DNB nach ISBN (`num=`): je 1 Treffer für beide ISBN der Dateien. Open Library nach ISBN: 0 Treffer.

Eine Kennung braucht darum einen **Wechsel der Region**, bevor «kein Treffer» gilt.

## 4. Abfragen je Buch und Laufzeit

| Quelle | Abrufe für 107 Bücher | je Buch | davon 2. Versuch nötig | Latenz p50 / p95 | Laufzeit |
|---|---|---|---|---|---|
| Audible `.de` | 123 | 1,15 | 16 Bücher | 193 / 266 ms | 128 s |
| Audible `.com` (10 Bücher) | 10 | 0,09 | – | 362 / 415 ms | 10 s |
| Audnexus (je ASIN) | 97 | 0,91 | – | 35 / 230 ms | 101 s |
| DNB | 138 | 1,29 | 31 Bücher | 189 / 297 ms | 144 s |
| Apple Books | 130 | 1,21 | 23 Bücher | 248 / 349 ms | **413 s** |
| Open Library | 178 | 1,66 | 71 Bücher | 554 / 651 ms | 186 s |

**Vorgabe `local, audible, dnb, applebooks` (mit Audnexus):** 498 Abrufe, 4,65 je Buch.
Laufzeit des ersten Laufs: **nacheinander 796 s (13 Minuten)**; mit je einem Takt je Host gleichzeitig
**413 s (7 Minuten)**, denn Apple (20 je Minute) bestimmt die Zeit.

**Weiter nur, wenn nicht zugeordnet.** Der Server muss DNB und Apple nicht für Bücher fragen, die Audible
schon mild automatisch zugeordnet hat (78 von 107). Für die übrigen 29 Bücher fallen DNB 44 statt 138 und
Apple 39 statt 130 Abrufe an (je −68 % und −70 %): **2,9 Abrufe je Buch, nacheinander rund 7 Minuten, getrennt
rund 2,5 Minuten.** Der Preis: Für zugeordnete Bücher fehlt die ISBN, wenn Audnexus keine hat (16 von 97).

**Hochgerechnet auf grosse Bibliotheken** (linear, gleiche Takte; Marcos Bibliothek hat 107):

| Bücher | Vorgabe, getrennte Hosts (Apple bestimmt) | mit «weiter nur, wenn nicht zugeordnet» (Audible bestimmt) |
|---|---|---|
| 107 | 7 Minuten | 2,5 Minuten |
| 1'000 | 1,1 Stunden | 0,4 Stunden |
| 5'000 | 5,4 Stunden | 1,8 Stunden |

Der Lauf geht im Pool `general` im Hintergrund; ein Neustart setzt ihn fort (Zustand je Buch, D6.3).

## 5. Audible von CT 190 aus

Vom CT (`192.168.1.43`, Host des Containers; nur ausgehende Abrufe, keine Datei der App berührt):

| Abruf | Ergebnis |
|---|---|
| Audible `.de`, Katalogsuche, 20 Bücher (die ersten 20 der Liste), 1 je Sekunde | **20 × 200**, im Mittel 195 ms (Höchstwert 260 ms), **bei allen 20 dieselbe Zahl Treffer** wie vom Entwicklungsrechner |
| Audible `.com` | 200, 360 ms |
| Audnexus | 404 `REGION_UNAVAILABLE` für die ASIN aus Abschnitt 3 (wie am Entwicklungsrechner) |
| DNB, Apple Books, Open Library, ein Cover-Host (Amazon) | 200 |
| Google Books ohne Schlüssel | 429 |

**Audible antwortet von CT 190 aus**, ohne Sperre, ohne HTML, mit gleichen Ergebnissen. Nicht geprüft: ob das
auch gilt, wenn Slouchcat Tausende Bücher hintereinander fragt (Abschnitt 4, 6 Stunden Pause bei Sperre).
Die Abrufe liefen vom Host des CT, nicht aus dem Container der App; beide benutzen dieselbe Netzadresse.

## 6. Die Zeile «Reihen und Sammlungen» bei 5'000 Büchern

**Aufbau.** Test `docs/perf/19-hoerbuecher/zeile_test.go.txt` im Paket `store`: leere Datenbank mit allen
Migrationen bis 00043 (`store.Open`), dazu `audio_series` und `items.audio_series_id` nach D4.4 (die
Migration 00048 von D2 gibt es noch nicht), SQLite wie im Server (`modernc.org/sqlite`, Lesepool). Daten:
3'000 Filme (damit `items` so gross ist wie eine echte), **5'000 Hörbücher**, davon 40 % in 288 Reihen mit 2
bis 12 Bänden (bei Marco 46 von 107), 60 von Hand angelegte Sammlungen (50 nur mit Hörbüchern, 10 gemischt),
Fortschritt für 25 % der Bücher je Profil, drei Profile (ab 18, ab 12, ab 6) bei Altersfreigaben von 0 bis 18.

**Abfrage.** Eine Anweisung für Reihen (nur sichtbare Bände über `visible_items`, ab zwei sichtbaren, Cover und
Autor des ersten sichtbaren Bandes, Zahl der Bände, gehörte Bände, Gesamtdauer) und Sammlungen (nur solche,
deren sichtbare Mitglieder alle Hörbücher sind), nach Name sortiert, alle Zeilen gelesen. Mit Filter
«2 von 6 Bänden» (Genre, Genre und Jahr) zählt sie zusätzlich die passenden Bände. 300 Läufe je Fall nach 20
zum Aufwärmen; Entwicklungsrechner (i7-14700K, NVMe), ohne `-race`.

| Fall | Profil | Zeilen | kalt | p50 | **p95** | p99 | Höchstwert |
|---|---|---|---|---|---|---|---|
| ohne Filter | ab 18 | 337 | 9,2 ms | 7,6 ms | **7,9 ms** | 10,0 ms | 14,5 ms |
| ohne Filter | ab 12 | 326 | 6,7 ms | 6,7 ms | 6,9 ms | 7,5 ms | 7,8 ms |
| ohne Filter | ab 6 | 291 | 5,2 ms | 5,2 ms | 5,6 ms | 9,0 ms | 9,9 ms |
| Genre | ab 18 | 216 | 8,1 ms | 8,1 ms | 8,3 ms | 8,7 ms | 8,9 ms |
| Genre | ab 12 | 196 | 7,0 ms | 7,0 ms | 7,2 ms | 7,9 ms | 8,6 ms |
| Genre | ab 6 | 153 | 5,4 ms | 5,4 ms | 5,5 ms | 5,8 ms | 6,3 ms |
| Genre und Jahr | ab 18 | 139 | 7,7 ms | 7,8 ms | 8,0 ms | 8,3 ms | 9,7 ms |
| Genre und Jahr | ab 12 | 115 | 6,8 ms | 6,7 ms | 6,8 ms | 7,0 ms | 7,0 ms |
| Genre und Jahr | ab 6 | 94 | 5,2 ms | 5,2 ms | 5,4 ms | 5,6 ms | 9,1 ms |

**Ziel p95 unter 100 ms: erfüllt** (grösster Wert 8,3 ms, Faktor 12 Luft). Bei **20'000 Büchern**
(1'137 Reihen) liegt p95 bei 29 ms (ab 18), 26 ms (ab 12), 20 ms (ab 6): wächst etwa linear. Die Abfrage
braucht keinen Zwischenspeicher. Auf CT 190 habe ich nicht gemessen, weil dafür eine Datenbank dorthin
kopiert werden müsste; bei einem Rechner, der zehnmal langsamer ist, wären es rund 80 ms bei 5'000 Büchern.
Das Ergebnis gilt für die Form der Abfrage aus D4; D2 misst sie mit der echten Migration nach.

## 7. Empfehlung

1. **Vorgabe `local, audible, dnb, applebooks` bleibt.** Audible ist die einzige Quelle mit Länge und damit
   die einzige, die automatisch zuordnen kann (64 % streng, 73 % mild). Sie liefert auch mit Audnexus die
   meisten Felder (Beschreibung, Reihe, Verlag, Genres, Cover, bei 84 % die ISBN). DNB ergänzt ISBN und
   Verlag und trägt als einzige keine Nutzungsbedingungen gegen das Auslesen (CC0). Apple ergänzt Beschreibung
   und Cover für Bücher, die Audible nicht kennt; es ist mit 3,2 Sekunden je Abruf die langsamste Quelle.
   Die Reihenfolge DNB vor Apple ändert das Ergebnis kaum (Audible, dann Apple: 101 statt 100 Bücher mit
   Treffer nach zwei Quellen); sie ist also Geschmackssache, die das Papier schon entschieden hat.
2. **Open Library und Google Books bleiben aus.** Open Library bringt 0 Bücher hinzu und zu 17 % einen Treffer
   mit falschem Autor; Google Books antwortet ohne Schlüssel mit 429.
3. **«Eindeutig» nach der Länge entscheiden, nicht vor ihr** (D6.3 Punkt 3): zuerst die Treffer mit Länge
   innerhalb der Toleranz bilden, dann prüfen, ob genau einer bleibt (mild: 73 % statt streng 64 %; gleiche
   Titel mit gleicher Länge zählen als einer). Die Reihennummer stimmte bei allen 33 prüfbaren Büchern.
4. **Quellen ohne Länge ordnen nie automatisch zu** (Apple, Open Library, DNB zu 82 %): ihre Treffer sind
   `review` mit Kandidaten, auch wenn sie eindeutig sind. Sie füllen leere Felder nur über die Zuordnung im
   Editor oder wenn Audible das Buch schon zugeordnet hat. So bleibt «automatisch nur mit passender Länge»
   wahr, und es landen von 107 Büchern rund 29 in «Prüfen» (20 mit Audible-Treffern, deren Ausgabe sich nicht
   entscheiden lässt, 9 ohne Treffer).
5. **Toleranz 3 % ab Werk bleibt**; 5 % ordnet 6 weitere Bücher zu und hatte keinen beobachteten Fehler. Marco
   kann die Einstellung später heben.
6. **Quellen nacheinander bis zur Zuordnung** abfragen statt alle für jedes Buch: spart rund 70 % der Abrufe
   von DNB und Apple und senkt die Laufzeit nacheinander von 13 auf 7 Minuten (Abschnitt 4). Audnexus folgt direkt auf einen Audible-Treffer.
7. **Kennung mit Regionswechsel:** Eine ASIN, die Audible `.de` oder Audnexus `region=de` nicht kennt, mit
   `.com`, `.co.uk` und ohne `region` weiter versuchen, bevor die Kennung als unbekannt gilt (Abschnitt 3).
8. **Der Titel der Dateien ist die grösste Fehlerquelle** (4 von 9 Büchern ohne Treffer, 20 mit mehreren
   Ausgaben): ein bereinigter Titel im Zweitversuch (Klammern, «- …», «Live und ungekürzt» weg) ist schon in D6.3
   nötig; das Messskript tut es nur für Klammern und «- …».
9. **Die Schutzgrenzen aus D6.3 Punkt 7 (3 × 403/429, 6 Stunden Pause) bleiben, wurden aber nicht ausgelöst:**
   bei 1 Abruf je Sekunde und Host kam in 676 Abrufen keine Sperre.

Die Zahlen gelten für **diesen Bestand**: deutsche Bücher, viele mit Reihe, wenige englische. Bei einem anderen
Bestand (viele englische oder seltene Titel) fällt der Anteil von Audible ab; Open Library und Google Books
würden dann mehr bringen, bleiben aber hinter einem Schlüssel oder ohne Länge.

## 8. Messweg wiederholen

```bash
cd docs/perf/19-hoerbuecher
python3 books.py                         # books.json aus dem NAS (nur lesen)
for s in audible dnb applebooks openlibrary; do python3 fetch.py $s & done; wait
python3 best.py && python3 fetch.py audnexus && python3 fetch.py audible_com
python3 analyze.py && python3 analyze2.py
```

Die Zeile aus Abschnitt 6: `zeile_test.go.txt` nach `server/internal/store/zz_d0_test.go` kopieren und
`D0=1 go test ./internal/store -run TestZZD0Row -v -count=1` ausführen (danach die Datei wieder löschen).
