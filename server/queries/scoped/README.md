# Abfragen mit Scope

Abfragen in diesem Ordner erzeugt sqlc in das Paket `internal/store/scopedq`; sie laufen immer für
ein Zuschauerprofil. Jede Abfrage muss `visible_items` verbinden (Plan 3.3, CLAUDE.md Regel 2).
Ungefilterte Abfragen gehören nach `../system/`.

In M0 ist der Ordner leer. sqlc lehnt einen Abfrageordner ohne Abfragen ab ("no queries contained
in paths"), deshalb ist der passende Eintrag in `server/sqlc.yaml` auskommentiert. Er wird mit der
ersten Abfrage hier aktiviert.
