# Messwerte: Konzept 15 «Feinschliff»

Konzept `docs/planning/15-feinschliff.md`, Stufe 2 (Abschnitt 2, P-1 «Weiterschauen» je Serie).
Review-Befund L-1: Aufwand von `ListContinueWatching` bei einem Profil mit sehr vielen
Fortschrittszeilen messen; Schwelle für eine Meldung rund 20 ms.

## 1. `ListContinueWatching` mit 10 000 Fortschrittszeilen (6. November 2026)

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

**Ergebnis: über der Schwelle von rund 20 ms**, etwa das 2,5-Fache. Die Zusammenfassungen der 20
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

### `EXPLAIN QUERY PLAN` (Kurzfassung)

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

### Einordnung

- Die Grenze der Startseite (P-11: `GET /home` < 300 ms p95) hält auf dem Entwicklungsrechner auch
  in diesem Extremfall. Auf CT 190 (2 vCPU) ist nach `docs/perf/M3.md` 1c mit dem Faktor 2 bis 3 zu
  rechnen, also 100 bis 150 ms nur für diese Reihe. Das ist ein grosser Teil der Latte.
- 10 000 Fortschrittszeilen in einem Profil sind viel (etwa 100 ganz gesehene Serien). Der Aufwand
  wächst etwa linear mit der Zahl der Zeilen des Profils, nicht mit der Grösse der Bibliothek.
- Die Regel bleibt, wie sie ist. Alte fertige Zeilen vorab wegzufiltern wäre falsch: Die neueste
  Zeile einer Serie entscheidet, auch wenn sie alt und fertig ist (dann fehlt die Serie). Ohne sie
  käme eine ältere angefangene Folge wieder in die Reihe.

### Vorschlag (nicht umgesetzt)

1. **Neuester Stand je Gruppe als eigene Tabelle.** Eine Tabelle `progress_latest (profile_id,
group_id, item_id, updated_at, completed)` mit `group_id` = Serie oder Titel, gepflegt beim
   Schreiben des Fortschritts (Trigger auf `progress` oder im selben Schreibvorgang). Die Reihe liest
   dann nur noch eine Zeile je Serie und Film, sortiert über einen Index auf `(profile_id,
updated_at DESC)` und kann nach 20 sichtbaren Einträgen aufhören. Die Regel bleibt gleich, die
   Kosten hängen nur noch an der Zahl der Gruppen. Braucht eine Migration (nach Vorlage) und einen
   eingefrorenen DB-Stand; sinnvoll in einer eigenen Stufe mit Entscheid von Marco.
2. **Kleinere Massnahme ohne Schema.** `newest` mit `GROUP BY` und `max(updated_at)` (SQLite nimmt
   die übrigen Spalten aus der Zeile mit dem Maximum) statt `row_number()` sparte die
   Fenstersortierung (rund 11 ms von 49 ms), lässt aber den Durchlauf über alle Zeilen. Vorher
   messen; die Gleichstandsregel (`updated_at`, dann `item_id`) müsste erhalten bleiben.

Erst messen, wie viele Fortschrittszeilen echte Profile auf CT 190 haben, bevor umgebaut wird.
