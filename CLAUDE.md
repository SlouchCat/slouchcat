# Slouchcat – Regeln für Agenten

Slouchcat ist ein selbst gehosteter Media-Server (Go-Server + React-PWA, ein Docker-Container). Massgeblich
sind `docs/spec/01-anforderungen-v1.md` (Spec), `docs/planning/03-plan-v1.md` (Plan),
`docs/planning/04-entscheidungen.md` (verbindliche Entscheide) und `docs/adr/`. Aktueller Stand:
`docs/dev/CURRENT_STATUS.md`. Bei Widerspruch gilt: Entscheidungsprotokoll > Plan > Spec.

## Sprache

- Dokumentation, Kommentare für den Auftraggeber und Commit-Erläuterungen für Marco: Deutsch in
  Schweizer Schreibweise (ss statt ß).
- Code, Bezeichner, Code-Kommentare, Commit-Titel und -Body: Englisch. Commit-Titel mit Bereich,
  z. B. `chore(server): add health endpoint and config loading`.

## Werkzeuge und Versionen

- Go: eine der zwei neuesten unterstützten Versionen, gepinnt in `server/go.mod` und `.tool-versions`.
  Go-Werkzeuge (sqlc, goose, oapi-codegen, golangci-lint) laufen über `go tool` aus `server/go.mod`.
- Node 24 LTS (lokal unter `~/.local/share/node`), pnpm Workspaces, `task` (Taskfile.yml) als einziger
  Einstieg: `task ci` baut und prüft alles, lokal wie in CI.
- Installationen mit sudo macht Marco selbst (`scripts/install-system-tools.sh`).

## Harte Regeln (Lint setzt sie durch, wo möglich)

1. **Generierter Code ist tabu.** `server/internal/api/gen/`, `server/internal/store/*q/`,
   `packages/api-client/src/generated/`, `packages/design-tokens/dist/` werden nur durch Codegen geändert.
   Quelle ändern (`api/openapi.yaml`, `server/queries/`, `server/migrations/`,
   `packages/design-tokens/tokens/`), dann `task gen`.
2. **Titelabfragen nur über `visibility`.** SQL mit `items`, `media_files` oder `tracks` sowie der Import
   von `store/systemq` sind nur in den Paketen `visibility`, `admin`, `library`, `metadata`, `jobs`,
   `notify`, `offline` erlaubt (Analyzer `itemsaccess`, `depguard`). Abfragen in `queries/scoped/` müssen
   `visible_items` enthalten. Nicht sichtbar heisst 404, nie 403.
3. **`os/exec` nur im Paket `transcode`**, immer `exec.CommandContext` mit Argumentliste, nie Shell
   (`forbidigo`).
4. **Dateizugriff mit Benutzerdaten nur über `paths`.** Relativ zu registrierten Wurzeln, `..` wird
   verworfen, Symlinks nur innerhalb der Wurzeln.
5. **Migrationen nur vorwärts, nach Vorlage** (`server/migrations/TEMPLATE.sql.txt`):
   `-- +goose NO TRANSACTION`, `PRAGMA foreign_keys=OFF`, Änderungen, `PRAGMA foreign_key_check`,
   `PRAGMA foreign_keys=ON`. Keine Down-Migrationen. Vor jedem Lauf sichert der Server die Datenbank mit
   `VACUUM INTO` (letzte 5 behalten). Ab M1a: eingefrorene DB-Stände in `testdata/db/`.
6. **Streaming-Endpunkte als `http.Handler`** (Range, HLS, Bilder) mit Beschreibung in `api/openapi.yaml`;
   alle JSON-Endpunkte über den generierten strict server.
7. **Jede OpenAPI-Operation trägt `x-visibility`** (`public`, `scoped`, `system`, `admin`). Antworten mit
   `ItemRef` sind `scoped`. Der Schema-Test schlägt sonst fehl.
8. **Jede neue Seite hat einen Playwright-Tastaturtest** (Pfeiltasten, Tab, Enter, Zurück, sichtbarer
   Fokus, keine Fokusfalle). Gilt ab M1a.
9. **Keine Farbe und kein Text hart im Code.** Farben nur aus Tokens (`stylelint`: `color-no-hex`,
   `declaration-strict-value`), Texte nur aus `web/src/locales/{de,en}.json` (`eslint-plugin-i18next`).
   Admin und Zuschauer nutzen dieselben Komponenten aus `web/src/ui`.
10. **Keine Secrets in Logs.** Nur `slog` über den Redaction-Handler aus `internal/logging`; neue
    Schlüsselnamen mit Secrets in die Redaction-Liste und in den Test.
11. **Keine Nicht-Ziele bauen** (Spec "Non-Goals": Musik, Fotos, Live-TV, TV-Apps, native Apps, Plugins,
    algorithmische Empfehlungen, Trakt-Login, eigene Wunschverwaltung, Steuerung von Arr/Seerr, Mandanten,
    Cloud, YouTube-Downloads). Nichts aus einem späteren Meilenstein vorziehen.
12. **Entscheid im Server, offline nur Nachschlagen.** Der Client implementiert keine Sichtbarkeits- oder
    Altersregel (Plan 2.1, Prinzip 1).

## Arbeitsweise

- Jeder Planschritt ist ein eigener Commit; nicht pushen, bevor Marco das Remote eingerichtet hat.
- Vor dem Abschluss eines Schritts: `task ci` grün, DoD-Belege (Befehl und Ausgabe) sammeln,
  `docs/dev/CURRENT_STATUS.md` aktualisieren.
- Messwerte nach `docs/perf/<Mx>.md`; neue Architekturentscheide als ADR in `docs/adr/`.
- Platzhalter wie TODO ohne Umsetzung, übersprungene Tests oder Stub-Tests gelten als Blocker, nicht als
  Fortschritt.
