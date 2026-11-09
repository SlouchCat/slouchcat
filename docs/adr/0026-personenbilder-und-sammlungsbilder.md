# ADR 0026: Personenbilder und Sammlungsbilder

- Status: angenommen (9. November 2026)
- Quelle: Konzept `docs/planning/17-detailseiten.md` (Abschnitte 5.1 und 5.2, Tabelle Abschnitt 12
  Zeile 3), **Frage 24** in `docs/planning/04-entscheidungen.md` mit den Nachträgen vom 8. November
  2026; Bauplan Stufe 3 (Annahmen E-1 bis E-7, E-17, E-19)
- Vertrag 1.21.0 (additiv), Migration 00040
- Harte Regeln: 2 (Sichtbarkeit nur über `visibility`, nicht sichtbar heisst 404), 3 (`os/exec` nur in
  `transcode`), 4 (Dateien nur über `paths`), 6 (Bilder als `http.Handler`), 7 (`x-visibility`,
  Vertragstest), 12 (Entscheid im Server)

## Kontext

Die Besetzung, die Personenseite und die Personentreffer der Suche zeigen bis 1.20.0 nur einen
Buchstaben; Sammlungen zeigen das Poster ihres ersten sichtbaren Mitglieds. Die Pfade der Bilder kennt
der Server längst: `people.profile_path` (seit 00008) und `collections.poster_path` (seit 00015) werden
bei jedem Abgleich gespeichert, aber nie geladen. Gesucht war ein Weg, rund 30'000 Porträts und einige
hundert Sammlungsposter zu laden, ohne die Abgleiche zu bremsen, ohne einen Auftrag je Person in der
Historie und ohne dass ein Sammlungsposter einem Kinderprofil einen verborgenen Film zeigt.

## Entscheid

1. **Zwei Tabellen, eine Zeile je Format, eine Zeile je Fehlschlag** (Migration 00040).
   `person_images` (Schlüssel `person_id, format`) und `collection_images` (`collection_id, format`),
   `STRICT, WITHOUT ROWID`, nach dem Muster von `images` (00029). `source` hält den Pfad des Anbieters
   (`profile_path` bzw. `poster_path`), so fällt ein geänderter Pfad auf. `status = 'ok'` verlangt
   Datei, Hash, Medientyp, Breite, Höhe und Grösse (alle positiv); `status = 'missing'` ist genau eine
   JPEG-Zeile ohne diese Werte. Die Tabellen `people` und `collections` bleiben, wie sie sind.

2. **Grössen und Ablage.** Person: TMDB `w185`, die JPEG-Datei so, wie sie kommt (Breite und Höhe aus
   dem Kopf), WebP daraus über `transcode` in derselben Breite. Sammlung: TMDB `w500`, daraus `w400`
   als JPEG und WebP. Dateien unter dem `CacheDir`: `people/<id>/w185.{jpg,webp}` und
   `collections/<ULID>/w400.{jpg,webp}`, über `paths`, mit temporärem Namen und Umbenennen. Die
   Dateien sind regenerierbar und gehören nicht in die Sicherung (die Sicherung kennt den `CacheDir`
   nicht).

3. **Eigene Drossel für das Bild-CDN.** `Client.DownloadCDNImage` lädt von `image.tmdb.org` mit einem
   eigenen Limiter (10 je Sekunde, Spitze 10), ohne Schlüssel. Die API-Drossel (20 je Sekunde) bleibt
   den Abgleichen; `image.fetch` behält seinen bisherigen Weg.

4. **Ein Auftrag `people.images` für alles.** Ereignistyp der Gruppe `metadata`, Pool `general`,
   Priorität 1 (unter `image.fetch` = 2), Schlüssel `people.images`, mit eigenem Zeitfenster
   (`Windowable`, Massen-Download). Ein Lauf geht erst die Personen, dann die Sammlungen in Paketen zu
   200 nach `id` durch.
   - **Offen** ist eine Person mit `profile_path` und einer Rolle auf einem nicht gelöschten Titel
     bzw. eine TMDB-Sammlung mit `poster_path` und einem nicht gelöschten Mitglied, wenn keine Zeile
     zum aktuellen Pfad besteht, die `missing`-Zeile älter als 30 Tage ist oder die Datei einer
     `ok`-Zeile fehlt (geprüft in jedem Lauf). Ohne Pfad gehen Zeilen und Dateien.
   - **Fehler:** Nur 404 oder eine leere bzw. nicht lesbare Antwort ergibt `missing`. 5xx, 429 nach
     den Wiederholungen, Zeitüberschreitung und ein ffmpeg-Fehler schreiben nichts; drei in Folge
     beenden den Lauf mit Fehler, die Warteschlange wiederholt ihn nach ihrem Backoff.
   - **Durchgänge:** Nach dem letzten Paket folgt ein weiterer Durchgang, solange offene Arbeit da
     ist, die in diesem Lauf nicht schon fehlschlug; höchstens drei Durchgänge je Lauf.
   - **Eingereiht** nach einem Abgleich, der Rollen oder eine Sammlung geschrieben hat (Film und
     Serie), und am Ende von `QueuePendingMetadata`, wenn etwas offen ist. Der erste volle Scan nach
     dem Update holt so den Bestand. Der Schlüssel macht aus vielen Abgleichen einen Auftrag; was
     während eines Laufs dazukommt, nimmt der nächste Durchgang mit.

5. **Nachholen `people.images_backfill`.** Zeitplantyp ohne eigenen Handler, Zeitplan aus (alle 7
   Tage, wenn eingeschaltet). Er löscht die `missing`-Zeilen und reiht `people.images` ein (ein
   wartender Auftrag wird auf die Priorität des Laufs gehoben). «Jetzt ausführen» auf der Seite
   Hintergrundjobs ist der Knopf. Ein gesetztes Fenster gilt auch dann.

6. **Sichtbarkeit im Server.** `GET /people/{id}/image` liefert das Porträt nur, wenn die Person eine
   Rolle auf einem für das Profil sichtbaren Titel hat. `GET /collections/{id}/image` liefert das
   Poster nur für eine **ganz sichtbare** Sammlung: mindestens ein sichtbares und kein nicht
   gelöschtes, nicht sichtbares Mitglied. Sonst 404, nie 403, und nie das Poster eines Mitglieds.
   Die Regel steht einmal in `ListFullyVisibleCollectionImages` (`queries/scoped/people.sql`); der
   Endpunkt und die Felder nutzen dieselbe Abfrage. Weil `visible_items` nur verfügbare Titel führt
   (`available = 1`), gilt eine Sammlung mit einem Mitglied ohne Datei als nicht ganz sichtbar und
   zeigt weiter das Mitgliedsposter.

7. **Felder.** `CastMember.image`, `PersonRef.image`, `Person.image`, `CollectionRef.image` und
   `Collection.image` (`ImageRef`, optional) sind genau dann gesetzt, wenn der Endpunkt das Bild
   ausliefern würde. Die URL trägt `v` (Hash der JPEG-Zeile) und `p` (Profil), wie bei den
   Titelbildern; Breite und Höhe sind die der JPEG-Zeile. Geladen ohne N+1: ein `LEFT JOIN` auf die
   `ok`-JPEG-Zeile in den Rollenabfragen und der Personensuche, eine Abfrage für alle Sammlungen einer
   Liste.

8. **Service Worker.** Die Regel `slouchcat-images` (`CacheFirst`) nimmt die beiden neuen Pfade auf.
   Der Cache wird beim Profilwechsel geleert, `p` trennt die Profile ohnehin. Offline zeigt die App so
   gesehene Bilder; sonst fällt sie auf den Buchstaben bzw. das Mitgliedsposter zurück.

## Folgen

- Rund 30'000 Downloads und ebenso viele WebP-Umwandlungen beim ersten Lauf; bei 10 je Sekunde gut
  eine Stunde, im Fenster und mit Pause steuerbar, ein Eintrag in der Historie. Geschätzt 0,9 GB im
  `CacheDir`; gemessen in `docs/perf/detailseiten.md`.
- Jeder Lauf prüft die Dateien aller `ok`-Zeilen (`Stat`); bei 30'000 Zeilen auf lokalem Cache wenige
  Sekunden.
- Der `LEFT JOIN` kostet bei 60 Rollen rund 0,06 bis 0,1 ms je Detailseite (Benchmark
  `BenchmarkListVisibleCredits`: etwa 0,12 ms vorher, 0,18 bis 0,22 ms nachher).
- **Restgrenze:** «Sammlung löschen» in der Verwaltung entfernt die Zeilen über die Kaskade, die bis zu
  zwei Dateien (rund 60 KB) bleiben im `CacheDir`. Personen werden nie gelöscht, eine TMDB-Sammlung
  ohne Mitglieder bleibt stehen, manuelle Sammlungen haben kein Bild. Kein Aufräumlauf in dieser
  Stufe.
- **Offene Frage** aus Konzept 17 Abschnitt 5.2: Darf ein TMDB-Sammlungsposter ein für das Profil
  verborgenes Mitglied zeigen? Bis zu einem Entscheid nein (Punkt 6).

## Verworfen

- **Laden beim ersten Anzeigen** (F-8): die erste Anzeige wäre leer, und jeder Aufruf müsste einen
  Auftrag anstossen.
- **Ein Auftrag je Person** (wie `image.fetch` je Titel): 30'000 Einträge in der Historie, siehe
  Konzept 09.
- **Spalten an `people` und `collections`**: die Tabellen müssten umgebaut werden, und Fehlschläge und
  zwei Formate passten schlecht in Spalten.
- **Die geteilte API-Drossel**: ein Massen-Download nähme den Abgleichen die Anfragen.
