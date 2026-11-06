# Messwerte: Konzept 15 «Feinschliff»

Konzept `docs/planning/15-feinschliff.md`, Stufe 2 (Abschnitt 2, P-1 «Weiterschauen» je Serie).
Review-Befund L-1: Aufwand von `ListContinueWatching` bei einem Profil mit sehr vielen
Fortschrittszeilen messen; Schwelle für eine Meldung rund 20 ms.

## 1. Vorher: `ListContinueWatching` mit 10 000 Fortschrittszeilen (6. November 2026)

Entwicklungsrechner (Linux Mint 22.3, i7-14700K, NVMe, wie `docs/perf/M3.md`), Go 1.27.2, Stand
`1cd48e56` (Zweig `claude/fs2-fix`). Benchmark `BenchmarkContinueWatching` in
`server/internal/visibility/continue_bench_test.go`:

```
cd server
go test -run '^$' -bench ContinueWatching -benchtime 200x -v ./internal/visibility/
```

Datenbank aus `vistest.New` (Filmbibliothek der Testumgebung) plus 100 Serien mit je einer Staffel
zu 100 Folgen, je Folge eine abspielbare Datei (10 000 Folgen). Ohne `ANALYZE`, wie im Betrieb.
Zwei Profile mit je 10 000 Zeilen in `progress`:

- **gemischt (P16):** Folgen 1 bis 99 fertig, eine Stunde auseinander; die neueste Zeile der Serie s
  liegt s × 1,5 Tage zurück. Gerade Serien haben Folge 100 angefangen (der Eintrag ist diese Folge),
  ungerade haben Folge 100 auf Position 0, ihr neuester Stand ist also die fertige Folge 99 und der
  Eintrag die nächste Folge, nur innerhalb des 60-Tage-Fensters.
- **alles fertig (P18):** jede Serie endet auf der fertigen Folge 99 innerhalb des Fensters
  (s × 0,5 Tage zurück), Folge 100 auf Position 0. Jede Serie läuft durch `next_episodes`; das ist
  der ungünstigste Fall.

Beide liefern volle 20 Einträge (`MaxRowItems`). n = 200 je Fall.

| Fall                                                                | p50     | p95         |
| ------------------------------------------------------------------- | ------- | ----------- |
| nur die Abfrage, gemischt                                           | 46,7 ms | **47,1 ms** |
| nur die Abfrage, alles fertig                                       | 49,4 ms | **50,1 ms** |
| `Reader.ContinueWatching` (Abfrage und Zusammenfassungen), gemischt | 48,1 ms | 49,1 ms     |
| `Reader.ContinueWatching`, alles fertig                             | 50,7 ms | **51,4 ms** |

**Ergebnis vorher: über der Schwelle von rund 20 ms**, etwa das 2,5-Fache. Die Zusammenfassungen der 20
Einträge kosten nur rund 1,5 ms; der Aufwand liegt in der Abfrage selbst.

### Aufteilung (Einzelmessung, Profil «alles fertig», p50 aus 30 Läufen)

Teilabfragen aus derselben SQL, jeweils mit `count(*)` am Ende:

| bis einschliesslich                                                                            | p50     |
| ---------------------------------------------------------------------------------------------- | ------- |
| `SELECT count(*) FROM visible_items` des Profils (Vergleich: ganze Sicht)                      | 2,4 ms  |
| CTE `watched` (10 000 Zeilen: `progress` × `items` × `visible_items`, Staffel, Serie sichtbar) | 20,6 ms |
| CTE `newest`, `rn = 1` (Fenstersortierung über 10 000 Zeilen)                                  | 31,6 ms |
| CTE `next_episodes`                                                                            | 34,4 ms |
| ganze Abfrage                                                                                  | 49,4 ms |

Ein Versuch, die sichtbaren Titel des Profils vorab als `MATERIALIZED`-CTE zu bilden und alle drei
`visible_items`-Joins darauf zu legen, brachte nichts (51,9 ms gegenüber 49,4 ms). Der Aufwand
wächst mit der Zahl der Fortschrittszeilen, nicht mit der Sichtbarkeitsprüfung allein.

### `EXPLAIN QUERY PLAN` vorher (Kurzfassung)

```
MATERIALIZE newest
  CO-ROUTINE watched
    SEARCH p USING PRIMARY KEY (profile_id=?)           -- alle Zeilen des Profils
    SEARCH e USING INDEX sqlite_autoindex_items_1 (id=?)
    SEARCH i USING INDEX sqlite_autoindex_items_1 (id=?) -- visible_items je Zeile, mit Unterabfragen
    SEARCH se USING INDEX sqlite_autoindex_items_1 (id=?) LEFT-JOIN
    CORRELATED SCALAR SUBQUERY (EXISTS: Serie sichtbar, je Zeile)
    USE TEMP B-TREE FOR ORDER BY                         -- row_number() über alle Zeilen
MATERIALIZE next_episodes
  SCAN x USING INDEX sqlite_autoindex_items_1          -- alle Titel, dann Bloom-Filter auf newest
  SEARCH xs USING INDEX sqlite_autoindex_items_1 (id=?)
  SEARCH n USING AUTOMATIC PARTIAL COVERING INDEX (rn=? AND completed=? AND in_window=? AND series_id=?)
  SEARCH xp USING PRIMARY KEY (profile_id=? AND item_id=?) LEFT-JOIN
  USE TEMP B-TREE FOR ORDER BY
SCAN n
SEARCH nx USING AUTOMATIC PARTIAL COVERING INDEX (series_id=? AND rn=?) LEFT-JOIN
SEARCH i … ; visible_items je Ergebniszeile
USE TEMP B-TREE FOR ORDER BY
```

Der volle Plan steht im Log des Benchmarks (`-v`).

## 2. Nachher: drei Umschreibungen ohne Regeländerung (6. November 2026)

Stand `0a7d2b3e`, sonst wie Abschnitt 1 (gleicher Benchmark, gleiche Daten, n = 200). Keine neue
Tabelle, keine Migration; alle bestehenden Tests von «Weiterschauen» laufen unverändert grün, ein
neuer Test hält die Gleichstandsregel fest (`TestContinueWatchingSameTime`).

| Fall                                    | p50 vorher | p50 nachher | p95 vorher | p95 nachher |
| --------------------------------------- | ---------- | ----------- | ---------- | ----------- |
| nur die Abfrage, gemischt               | 46,7 ms    | 17,8 ms     | 47,1 ms    | **18,7 ms** |
| nur die Abfrage, alles fertig           | 49,4 ms    | 19,6 ms     | 50,1 ms    | **20,1 ms** |
| `Reader.ContinueWatching`, gemischt     | 48,1 ms    | 19,2 ms     | 49,1 ms    | 20,3 ms     |
| `Reader.ContinueWatching`, alles fertig | 50,7 ms    | 21,1 ms     | 51,4 ms    | **21,6 ms** |

Rund 60 % weniger; die Abfrage liegt jetzt an der Schwelle von 20 ms, mit den Zusammenfassungen
knapp darüber.

**Warum `next_episodes` vorher alle Titel las.** Die Bedingungen standen als gewöhnliche `JOIN … ON`
da; damit darf SQLite die Reihenfolge der Tabellen frei wählen. Ohne `ANALYZE` schätzte der Planer
die materialisierte CTE `newest` (10 000 Zeilen) teurer ein als einen Durchlauf über `items` und nahm
`x` als äussere Schleife: alle Titel, je Titel die Sichtprüfung, erst danach der Abgleich mit
`newest` über einen automatischen Index. Die Indizes `items_parent` und `items_episode` blieben
ungenutzt.

Die drei Änderungen, einzeln gemessen (Profil «alles fertig», p50 aus 20 Läufen, nur die Abfrage):

| Variante                                                     | p50     |
| ------------------------------------------------------------ | ------- |
| vorher                                                       | 49,3 ms |
| nur `GROUP BY` mit `max()` in `newest`                       | 40,1 ms |
| nur `CROSS JOIN` in `next_episodes`                          | 38,4 ms |
| beides                                                       | 29,2 ms |
| beides und die Serie einmal je Gruppe prüfen (jetzt im Code) | 19,3 ms |

1. **`newest` gruppiert statt Fensterfunktion.** `GROUP BY coalesce(series_id, item_id)` mit dem
   einzigen Aggregat `max(printf('%020d', updated_at) || item_id)`. Bei genau einem `max()` nimmt
   SQLite die übrigen Spalten aus der Zeile mit dem Maximum. Der Schlüssel ordnet wie vorher
   `updated_at DESC, item_id DESC` (Zeit mit führenden Nullen, IDs gleich lang). Danach hat
   `newest` eine Zeile je Gruppe statt 10 000.
2. **Sichtbarkeit der Serie einmal je Gruppe.** Vorher prüfte ein `EXISTS` je Fortschrittszeile, ob
   die Serie sichtbar ist (10 000-mal die Sicht `visible_items`). Jetzt prüft `newest` das je
   Gruppe. Das Ergebnis ist gleich: Ist die Serie unsichtbar, fallen alle ihre Zeilen weg, also die
   ganze Gruppe. Die Sichtprüfung der einzelnen Folge bleibt je Zeile (eine unsichtbare Folge ist
   keine Aktivität, `TestContinueWatchingVisibility`).
3. **`CROSS JOIN` legt die Reihenfolge fest.** In SQLite behält `CROSS JOIN` die geschriebene
   Reihenfolge. `next_episodes` geht jetzt von den Serien in `newest` aus über `items_episode`
   (Staffeln der Serie) und `items_parent` (Folgen der Staffel).

### `EXPLAIN QUERY PLAN` nachher (Kurzfassung)

```
MATERIALIZE grouped
  SEARCH p USING PRIMARY KEY (profile_id=?)           -- alle Zeilen des Profils
  SEARCH e USING INDEX sqlite_autoindex_items_1 (id=?)
  SEARCH i USING INDEX sqlite_autoindex_items_1 (id=?) -- visible_items der Folge je Zeile
  SEARCH se USING INDEX sqlite_autoindex_items_1 (id=?) LEFT-JOIN
  USE TEMP B-TREE FOR GROUP BY
MATERIALIZE next_episodes
  SCAN g                                               -- eine Zeile je Gruppe
  CORRELATED SCALAR SUBQUERY (EXISTS: Serie sichtbar, je Gruppe)
  SEARCH xs USING INDEX items_episode (parent_id=?)
  SEARCH x USING INDEX items_parent (parent_id=?)
  SEARCH xp USING PRIMARY KEY (profile_id=? AND item_id=?) LEFT-JOIN
  USE TEMP B-TREE FOR ORDER BY
SCAN g
SEARCH nx USING AUTOMATIC PARTIAL COVERING INDEX (series_id=? AND rn=?) LEFT-JOIN
SEARCH i … ; visible_items je Ergebniszeile
USE TEMP B-TREE FOR ORDER BY
```

Was bleibt: der eine Durchlauf über alle Fortschrittszeilen des Profils mit der Sichtprüfung je
Folge (der grösste Teil der 19 ms; nicht einzeln gemessen). Ihn spart nur eine Tabelle mit dem neuesten Stand je Gruppe
(Vorschlag unten).

### Einordnung

- Die Grenze der Startseite (P-11: `GET /home` < 300 ms p95) hält mit Abstand. Auf CT 190
  (2 vCPU) ist nach `docs/perf/M3.md` 1c mit dem Faktor 2 bis 3 zu rechnen, also 40 bis 60 ms für
  diese Reihe statt 100 bis 150 ms vorher.
- 10 000 Fortschrittszeilen in einem Profil sind viel (etwa 100 ganz gesehene Serien). Der Aufwand
  wächst etwa linear mit der Zahl der Zeilen des Profils, nicht mit der Grösse der Bibliothek.
- Die Regel bleibt, wie sie ist. Alte fertige Zeilen vorab wegzufiltern wäre falsch: Die neueste
  Zeile einer Serie entscheidet, auch wenn sie alt und fertig ist (dann fehlt die Serie). Ohne sie
  käme eine ältere angefangene Folge wieder in die Reihe.

### Vorschlag (offen)

**Neuester Stand je Gruppe als eigene Tabelle.** Eine Tabelle `progress_latest (profile_id,
group_id, item_id, updated_at, completed)` mit `group_id` = Serie oder Titel, gepflegt beim
Schreiben des Fortschritts (Trigger auf `progress` oder im selben Schreibvorgang). Die Reihe liest
dann nur noch eine Zeile je Serie und Film, sortiert über einen Index auf
`(profile_id, updated_at DESC)`, und kann nach 20 sichtbaren Einträgen aufhören. Die Regel bleibt
gleich, die Kosten hängen nur noch an der Zahl der Gruppen. Das braucht eine Migration nach Vorlage
und einen eingefrorenen DB-Stand. Sinnvoll erst, wenn echte Profile auf CT 190 so viele
Fortschrittszeilen haben, dass die Reihe spürbar wird; dann in einer eigenen Stufe mit Entscheid
von Marco.

## 3. Stufe 3, P-7: «Meine Medien» und «Neu in …» (6. November 2026)

Konzept 15, Abschnitt 8.3. Die zwei neuen Reihen sind ab Werk ausgeblendet; die Startseite wird
nur schwerer, wenn der Administrator sie einblendet. Gemessen, was jede eingeblendete Reihe je
Aufruf von `GET /home` kostet, neben dem, was die Startseite und die Bibliotheksseite heute schon
lesen.

Entwicklungsrechner wie oben, Go 1.27.2, Stand `988f2710` (Zweig `claude/fs3-homerows`).
Benchmark `BenchmarkLibraryRows` in `server/internal/visibility/library_bench_test.go`:

```
cd server
go test -run '^$' -bench LibraryRows -benchtime 200x -v ./internal/visibility/
```

Datenbank aus `vistest.New` plus 10 000 Filme in der Filmbibliothek, je einer mit abspielbarer
Datei und eigenem `added_at`; Profil P18 (sieht alles). Ohne `ANALYZE`. n = 200 je Fall.

| Fall                                                                    | p50     | p95     |
| ----------------------------------------------------------------------- | ------- | ------- |
| `VisibleLibraries` (Seitenleiste, Vergleich)                            | 7,2 ms  | 8,2 ms  |
| `Recent` (Reihe «Neu hinzugefügt», Vergleich)                           | 16,3 ms | 17,0 ms |
| `LibraryTiles` («Meine Medien», eine Kachel)                            | 20,3 ms | 23,5 ms |
| `LibraryNewest` («Neu in Filme», 20 Titel)                              | 13,8 ms | 16,5 ms |
| `LibraryItems` mit `sort=added`, 20 Titel (Bibliotheksseite, Vergleich) | 43,8 ms | 84,2 ms |

Einordnung:

- «Neu in …» liest mit derselben Anweisung wie die Bibliotheksseite (`libraryStmt`, sortiert nach
  `added_at`), aber ohne Zählung, Buchstaben, Genres und Altersstufen; sie kostet etwa so viel wie
  die Reihe «Neu hinzugefügt». Je eingeblendeter Bibliotheksreihe kommt das einmal dazu.
- «Meine Medien» zählt die Titel je Bibliothek wie die Seitenleiste und liest je Bibliothek die vier
  neuesten Titel; der Aufwand wächst mit der Zahl der Bibliotheken und ihrer Grösse.
- Mit allen fünfzehn Plätzen voll (die acht Reihen, «Meine Medien» und sechs Bibliotheksreihen) kämen
  bei Bibliotheken dieser Grösse rund 20 ms + 6 × 14 ms ≈ 105 ms dazu, auf CT 190 nach
  `docs/perf/M3.md` 1c mit dem Faktor 2 bis 3 also 210 bis 315 ms. Dann wäre die Grenze P-11
  (`GET /home` < 300 ms p95) nicht mehr sicher. Mit einer oder zwei eingeblendeten Reihen bleibt
  sie mit Abstand. Nachmessen bei der Abnahme der Stufe auf CT 190 mit den echten Bibliotheken:
  siehe unten; die Schätzung war zu günstig, weil die Testdatenbank keine Folgen enthielt.

### Nachmessung auf CT 190 (7. November 2026)

CT 190 (2 vCPU), Image des Stands `3429ef3b`, alle 13 Reihen eingeblendet: spotlight, picks,
continue, recent, released, watchlist, collections, genres, libraries und die Bibliotheksreihen
`library:1` Filme, `library:2` Serien, `library:3` test, `library:4` Podcast. Serverseitige Dauer
von `GET /home`, 60 Läufe warm, Profil 1 (Hauptprofil des Administrators):

| Stand                                  | p50         | p95        | max    |
| -------------------------------------- | ----------- | ---------- | ------ |
| `3429ef3b`, 13 Reihen                  | 586 ms      | **617 ms** | 683 ms |
| acht bisherige Reihen (nach dem Start) | ~300–320 ms |            |        |

Die Grenze P-11 (< 300 ms p95) war damit deutlich überschritten, und schon die acht bisherigen
Reihen lagen an ihr.

### Ursache

Gemessen auf einer Kopie der Datenbank von CT 190 (rund 70 000 Einträge in `items`: 2 099 Filme, 498 Serien mit
2 010 Staffeln und 29 106 Folgen, 100 Podcasts mit 38 605 Folgen; im Abzug sind 496 Serien
gelöscht, «Neu in Serien» zeigt darum zwei Serien), im Prozess über `discover.Service` je Reihe und
für `Home` gesamt, ohne HTTP und JSON. Entwicklungsrechner wie oben, ein Kern (`GOMAXPROCS=1`,
`taskset`), 40 Läufe warm, Profil 1. Der Stand `3429ef3b` braucht dort 126 ms p50; CT 190 ist also
rund 4,7-mal langsamer als ein Kern des Entwicklungsrechners.

Die Abfragepläne (`EXPLAIN QUERY PLAN`, ohne `ANALYZE` wie im Betrieb) zeigen bei drei Abfragen
`SCAN i`: Der Planer läuft über **alle** Zeilen von `items` und wertet für jede die View
`visible_items` aus, auch für jede Folge und jede Podcastfolge, und sucht erst danach die Art
(`kind IN ('movie', 'series', 'audiobook', 'podcast')`):

- `ListRecentlyAdded` (Reihe «Neu hinzugefügt», bisherige Reihe): rund 30 ms. Der vorhandene
  Index `items_added` hilft nicht: Vor dem 20. neuesten Titel liegen 17 006 Podcastfolgen, die der
  Podcast-Abgleich laufend nachträgt; der Aufwand wüchse mit jeder neuen Folge.
- `ListVisibleLibraries` (Zählung der Kacheln von «Meine Medien» und der Seitenleiste): rund 28 ms.
- `ListVisibleGenres` (Reihe «Genres», bisherige Reihe): beginnt bei `item_genres` und wertet die
  View je Genre eines Titels aus, rund 10 ms.

Dazu sortierte `newestIDs` («Neu in …» und die vier Poster je Kachel) jede Bibliothek ganz, mit dem
Sortierschlüssel jedes sichtbaren Titels (5 ms für die 1 795 Filme, zweimal je Seite), und
`LibraryTiles` fasste die Poster jeder Kachel einzeln zusammen. Die alte Grundlast von rund 300 ms
war zur Hälfte die Reihe «Neu hinzugefügt».

«Neu in Serien» listet Serien, nicht Folgen, und «Neu in Podcast» Podcasts (nach dem Zeitpunkt des
Abonnements), genau wie die Bibliotheksseite mit `sort=added`; die Zahl auf jeder Kachel ist die
Gesamtzahl der Bibliotheksseite. Geprüft für alle vier Profile des Abzugs (auch das Profil mit
Stufe 6): Reihe = erste 20 der Bibliotheksseite, Poster = deren erste vier, Zahl = `total`.

### Behebung

Vier Commits auf `claude/fs3-perf`, ohne Änderung an Sichtbarkeit oder Reihenfolge:

1. **Migration 00037** `items_top_added`: Teilindex `ON items (added_at DESC, id DESC) WHERE kind
IN ('movie', 'series', 'audiobook', 'podcast')`. `ListRecentlyAdded` liest ihn in seiner
   Reihenfolge und hört nach 20 sichtbaren Titeln auf (30 ms → 0,07 ms), `ListVisibleLibraries`
   zählt über ihn nur die rund 2 700 Titel der obersten Ebene (28 ms → 3,3 ms). Der Planer nimmt
   einen Teilindex nur, wenn die Abfrage dieselbe Bedingung trägt; die Kommentare der Abfragen
   halten das fest, `TestMigration00037TopAddedIndex` prüft Index und Pläne auf
   `testdata/db/startseitenreihen.sql`.
2. `ListVisibleGenres` beginnt bei den Titeln (über `items_top_added`) und hängt die Genres mit
   `CROSS JOIN` an: die View einmal je Titel (10 ms → 5 ms).
3. `newestIDs` liest zuerst, wann der `limit`-te neueste Titel hinzukam, und sortiert nur die Titel
   ab diesem Zeitpunkt; `added_at` ist nie NULL und steht vorne in der Ordnung, das Ergebnis bleibt
   gleich, auch bei gleichen Zeitpunkten an der Schnittstelle (`TestLibraryNewestTiesAtTheCut`).
   `LibraryTiles` fasst die Poster aller Kacheln in einem Aufruf zusammen.
4. `page()` lädt die Reihen nebeneinander, in höchstens `min(GOMAXPROCS, 4)` Goroutinen, und setzt
   sie danach wie bisher in der gespeicherten Reihenfolge zusammen (Fehler, Schnitt bei 15 Reihen
   und Bühne unverändert). Jede Reihe hält höchstens eine Verbindung des Lesepools zugleich. Der
   Gewinn ist kleiner als die Zahl der Kerne: Die Lesevorgänge in modernc SQLite behindern sich
   vermutlich gegenseitig.

Die Antwort von `Home` ist vorher und nachher für alle vier Profile des Abzugs byte-gleich (JSON
verglichen, feste Bühnenwahl).

### Nachher, Abzug von CT 190

Je Reihe, ein Kern, 40 Läufe warm, Profil 1:

| Reihe       | vorher p50 | vorher p95 | nachher p50 | nachher p95 |
| ----------- | ---------: | ---------: | ----------: | ----------: |
| spotlight   |     3,2 ms |     3,5 ms |      3,3 ms |      3,6 ms |
| picks       |     1,3 ms |     1,5 ms |      1,3 ms |      1,5 ms |
| continue    |     3,7 ms |     3,9 ms |      3,6 ms |      3,9 ms |
| recent      |    35,1 ms |    35,6 ms |      2,1 ms |      2,3 ms |
| released    |     3,5 ms |     3,8 ms |      3,5 ms |      3,8 ms |
| watchlist   |     1,1 ms |     1,2 ms |      1,1 ms |      1,2 ms |
| collections |     4,2 ms |     4,6 ms |      4,2 ms |      4,6 ms |
| genres      |    10,3 ms |    10,5 ms |      5,2 ms |      5,5 ms |
| libraries   |    43,3 ms |    43,7 ms |     11,1 ms |     11,5 ms |
| library:1   |     8,0 ms |     8,3 ms |      5,0 ms |      5,3 ms |
| library:2   |     2,4 ms |     2,5 ms |      3,2 ms |      3,3 ms |
| library:3   |     3,6 ms |     3,8 ms |      3,3 ms |      3,6 ms |
| library:4   |     3,1 ms |     3,4 ms |      2,6 ms |      2,9 ms |

`library:2` wird um knapp 1 ms teurer: Bei zwei Serien kostet die zusätzliche Abfrage von
`newestIDs` mehr, als das Sortieren spart.

`Home` gesamt, 40 Läufe warm, Profil 1:

| Kerne                 | vorher p50 | vorher p95 | nachher p50 | nachher p95 |
| --------------------- | ---------: | ---------: | ----------: | ----------: |
| 1 (`GOMAXPROCS=1`)    |   125,9 ms |   126,7 ms |     52,9 ms |     56,8 ms |
| 2 (`taskset`, wie CT) |   122,5 ms |   123,6 ms |     45,4 ms |     46,3 ms |
| alle 28               |   127,6 ms |   133,0 ms |     33,1 ms |     36,2 ms |

Benchmark `BenchmarkLibraryRows` (10 000 Filme, siehe oben), vorher `3429ef3b`, nachher `a4b3e3f6`:

| Fall                            | vorher p50 | vorher p95 | nachher p50 | nachher p95 |
| ------------------------------- | ---------: | ---------: | ----------: | ----------: |
| `VisibleLibraries`              |     7,0 ms |     7,2 ms |      8,7 ms |      8,8 ms |
| `Recent`                        |    16,2 ms |    17,1 ms |      1,3 ms |      2,2 ms |
| `LibraryTiles`                  |    20,1 ms |    20,5 ms |     17,8 ms |     19,0 ms |
| `LibraryNewest`                 |    13,6 ms |    14,7 ms |      9,5 ms |     10,9 ms |
| `LibraryItems` mit `sort=added` |    43,0 ms |    45,6 ms |     42,9 ms |     43,5 ms |

`VisibleLibraries` wird in diesem Benchmark etwas teurer: Dort sind fast alle Zeilen Filme, der
Umweg über den Index bringt also nichts und kostet je Titel einen Zugriff auf die Tabelle. Mit
Folgen und Podcastfolgen wie auf CT 190 ist er achtmal schneller.

### Schätzung für CT 190

Mit dem Faktor 4,7 (CT 190 zu einem Kern des Entwicklungsrechners, aus dem Stand vorher) und den
zwei Kernen von CT 190: nachher rund **210 ms p50 und 220 bis 230 ms p95** für alle 13 Reihen,
statt 586/617 ms; ohne das Nebeneinander (ein Kern) rund 250 bis 270 ms. Die Grenze P-11 hält damit
mit Abstand, und die acht bisherigen Reihen allein liegen deutlich unter den früheren 300 ms. Die
Schätzung trägt die Kosten von HTTP und JSON nur über den Faktor mit; nachmessen auf CT 190 mit dem
Image dieses Stands (offen).
