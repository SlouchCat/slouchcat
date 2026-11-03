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
7. **Jede OpenAPI-Operation trägt `x-visibility`** (`public`, `account`, `scoped`, `system`, `admin`; `account`
   = gültige Sitzung ohne gewähltes Profil, ADR 0005). Antworten mit `ItemRef` sind `scoped`. Der Schema-Test
   schlägt sonst fehl; jede Operation braucht einen Eintrag im Vertragstest (`TestContractCoversEveryOperation`).
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
- Geprüft wird in drei Stufen (Entscheid vom 4. November 2026, Einzelheiten in `docs/dev/SETUP.md`
  «Teststufen»):
  1. **Beim Arbeiten und je Planschritt:** nur das Betroffene: `task check:fast` und die Tests, die
     der Schritt anfasst (`go test ./internal/<paket> -run <Test>`, `task e2e:quick -- <spec>`,
     `task storybook:test` bei geänderten Bauteilen in `web/src/ui`). Kein `task ci`, keine ganze
     e2e-Suite und kein voller Build nach jeder Änderung. DoD-Belege (Befehl und Ausgabe) je Schritt.
  2. **Vor dem Pull Request, einmal auf dem fertigen Zweig:** `task ci` grün, Belege in den PR,
     `docs/dev/CURRENT_STATUS.md` aktualisieren. Spätere Korrekturen: `task check:fast` plus
     gezielte Tests; die CI prüft den neuen Stand vollständig.
  3. **GitHub CI:** prüft jeden PR und jeden Commit auf main vollständig; gemergt wird nur mit grünem
     Gate `ci`.
- Messwerte nach `docs/perf/<Mx>.md`; neue Architekturentscheide als ADR in `docs/adr/`.
- Platzhalter wie TODO ohne Umsetzung, übersprungene Tests oder Stub-Tests gelten als Blocker, nicht als
  Fortschritt.

## Agenten und Prüfungen

Diese Regeln gehen den allgemeinen OMC-Vorgaben in `~/.claude/CLAUDE.md` vor («keep iterating»,
Verifier für jede Abnahme).

- **Höchstens drei Executor-Agenten gleichzeitig.** Ganze Go-Suite unter `-race`, e2e-Läufe und
  Storybook nur über die Task-Ziele starten (`task server:test`, `task e2e`, `task ci:e2e:parallel`,
  `task storybook:test`), nicht direkt mit `go test -race ./...` oder `pnpm exec playwright test`:
  Die Ziele warten auf einen von zwei Plätzen der Maschine (`scripts/heavy-slot.sh`). Parallele
  e2e-Läufe brauchen eigene Ports (`E2E_PORT=8197`, `8297` …).
- **Executor:** prüft nach Stufe 1 und meldet Befehl, Ausgabe und Commit-SHA.
- **Verifier und Reviewer:** lesen diese Belege und wiederholen keinen Lauf auf derselben SHA. Eigene
  Läufe nur gezielt, für eine Lücke oder einen Zweifel. `task ci` startet nur der Koordinator vor
  dem PR.
- **git-master:** nach Cherry-pick oder Merge ohne Konflikt keine Tests; nach einem gelösten Konflikt
  `task check:fast`.
- **Höchstens drei Korrekturversuche** je roter Prüfung, dann anhalten und mit Befund melden. Nie
  grün machen durch Überspringen (`.skip`, `.only`, `t.Skip`), mehr Wiederholungen, längere Timeouts
  ohne Begründung oder gelockerte Grenzen.
- Flake-Suche mit `-count=N` oder `--repeat-each` nur für ein Paket oder einen Test, nicht für die
  ganze Suite.
