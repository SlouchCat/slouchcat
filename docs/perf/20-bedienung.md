# Messwerte: Konzept 20 «Bedienung und Gesehen»

Konzept `docs/planning/20-bedienung-und-gesehen.md`. Jede Stufe trägt ihre Messung in einem eigenen
Abschnitt ein.

## M-3 listHomeRow

Stufe 3 (Abschnitt 8.3): Antwortzeit von `listHomeRow` für `continue` mit 200 angefangenen Titeln und
für `recent` mit Filter; Ziel unter 150 ms. Gemessen am 12. November 2026 auf dem Entwicklungsrechner
(Linux Mint 22.3, i7-14700K, NVMe, wie `docs/perf/M3.md`), Go 1.27.2, Zweig `claude/k20-s3`. Die
Maschine war während der Messung durch parallele Läufe anderer Sitzungen belastet (Load um 40 bei
28 Threads); die Werte sind darum eher zu hoch. Auf CT 190 misst Marco bei der Abnahme nach.

Benchmark `BenchmarkHomeList` in `server/internal/visibility/homelist_bench_test.go`:

```
cd server
go test -run '^$' -bench HomeList -benchtime 200x -v ./internal/visibility/
```

Datenbank aus `vistest.New` (Filmbibliothek der Testumgebung) plus 5000 Filme und 250 Serien mit je
einer Staffel zu vier Folgen, je Film und Folge eine abspielbare Datei. Ohne `ANALYZE`, wie im
Betrieb. Profil P18 hat von jeder Serie Folge 2 angefangen: Die Abfrage von «Weiterschauen» liefert
250 Einträge und wird bei `MaxContinueList` (200) abgeschnitten. Die Serien sind vor den Filmen
hinzugefügt, der Filter «Serien» bei `recent` läuft also über alle 5000 Filme im Teilindex
`items_top_added` hinweg (ungünstigster Fall). Gemessen ist `Reader.ContinuePage` bzw.
`Reader.RecentPage` (Abfragen und Zusammenfassungen der Karten), Seite zu 50 Karten, n = 200 je Fall;
die JSON-Ausgabe des Handlers ist nicht mitgemessen.

| Fall                                                           | p50     | p95     |
| -------------------------------------------------------------- | ------- | ------- |
| `continue`, erste Seite (200 Einträge gerechnet)               | 22,7 ms | 40,2 ms |
| `continue`, Seite ab Versatz 150                               | 19,8 ms | 43,9 ms |
| `recent` mit `content=series`, erste Seite (Zählung über alle) | 10,3 ms | 20,9 ms |
| `recent` mit `content=series`, letzte Seite                    | 5,5 ms  | 8,6 ms  |
| `recent` ohne Filter, erste Seite                              | 6,8 ms  | 10,9 ms |

**Ergebnis: Ziel erreicht.** Alle Fälle bleiben deutlich unter 150 ms, auch im p95 unter Last.
`continue` kostet am meisten, weil die Abfrage je Aufruf die ganze Liste rechnet (bis 200 Einträge)
und Go danach die Seite schneidet; die Seite selbst (Versatz 0 oder 150) ändert daran fast nichts.

Abfrageplan: Der Filter `content` steht als zweiter Term neben dem Term des Teilindex; der Planer geht
mit und ohne Filter über `items_top_added` in dessen Reihenfolge, ohne temporären Sortierbaum. Das
prüft `TestHomeListRecentUsesTheIndex` für `ListRecentlyAddedPage` und `CountRecentlyAdded`.
