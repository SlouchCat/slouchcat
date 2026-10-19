# Abfragen mit Scope

Abfragen in diesem Ordner erzeugt sqlc in das Paket `internal/store/scopedq`; sie laufen immer für
ein Zuschauerprofil. Jede Abfrage muss `visible_items` verbinden (Plan 3.3, CLAUDE.md Regel 2); der
Test `TestScopedQueriesJoinVisibleItems` in `internal/store` prüft jede Abfrage dieses Ordners.
Ungefilterte Abfragen gehören nach `../system/`, Benutzerdaten ohne Titeltabellen nach `../app/`.
Nur das Paket `visibility` importiert `scopedq`.
