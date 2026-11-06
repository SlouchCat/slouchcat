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
