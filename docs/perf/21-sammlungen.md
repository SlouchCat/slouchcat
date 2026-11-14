# Messwerte: Konzept 21 P-10 «Sammlungen»

Gemessen am 14. November 2026 lokal (28 Kerne, Go 1.27.2, SQLite über `modernc.org/sqlite`, Datenbank in einem
temporären Ordner) auf dem Zweig `claude/k21-stufen4-6` (Vertrag 1.33.0, Migration 00060). Während der Messung
liefen auf derselben Maschine andere Agenten; die Werte streuen deshalb nach oben.

**Ergebnis:** `listVisibleCollections` liefert mit **333 Sammlungen** (Stand CT 190) im Leser jede Sammlung mit
Jahren, Gesamtdauer und Fortschritt in **7,4 ms** statt 5,7 ms ohne diese Angaben; die Antwort wächst um rund 5 %.
Darum sucht, filtert und sortiert die Übersicht `/collections` im Client über **eine** Antwort, wie es Abschnitt
12.3 des Konzepts zulässt; Filter und Sortierung stehen in der Adresse (`min`, `sort`, `q`).

## 1. `Reader.AllCollections` mit 333 Sammlungen

`BenchmarkAllCollections` in `server/internal/visibility/collections_bench_test.go`:

```
go test -run '^$' -bench AllCollections -benchtime 200x ./internal/visibility/
```

Aufbau: die Testbibliothek von `vistest` mit Serie und Sammlungen, dazu 333 TMDB-Sammlungen zu je drei Filmen. Das
Profil P18 hat den ersten Film gesehen, den zweiten angefangen, den dritten nicht begonnen. Der Vergleich «vorher»
lief mit demselben Benchmark auf dem Stand vor diesem Schritt (Abfrage ohne Jahre, Dauer und Fortschritt).

| Lauf (200 Aufrufe)                           | Zeit je Aufruf | Antwort (JSON des Modells) |
| -------------------------------------------- | -------------: | -------------------------: |
| vorher (Vertrag 1.32.0, ohne Angaben)        |         5,7 ms |                   558 KB   |
| mit Jahren, Dauer, `seenCount`, `started`    |     **7,4 ms** |                   585 KB   |

Die Grösse ist die des Modells als JSON (mit leeren Feldern) und liegt über der Antwort des Vertrags; der
Unterschied zeigt den Zuwachs. Auf CT 190 ist noch nicht gemessen; der Wert dort folgt bei der Abnahme.
