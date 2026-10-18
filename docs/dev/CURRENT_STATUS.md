# Aktueller Stand

Stand: 18. Oktober 2026. Branch `claude/m0-project-foundation`, nicht gepusht (Remote legt Marco an).

## Meilenstein M0 – Projektfundament: abgeschlossen

Alle sechs Schritte aus `docs/planning/03-plan-v1.md` (M0) sind umgesetzt, je ein Commit:

| Schritt | Commit | Inhalt |
|---|---|---|
| 1 Repo-Struktur, Spec, ADR | `chore(repo)` | Ordner nach Plan 4, `LICENSE` AGPL-3.0, Spec in `docs/spec/`, ADR 0001 Stack, `.tool-versions`, Setup-Doku, Installationsskript |
| 2 Agentenregeln | `docs(agents)` | `CLAUDE.md` mit den zwölf Regeln aus Plan M0 Schritt 2 |
| 3 Go-Server | `feat(server)` | `/healthz`, Konfiguration, slog mit Redaction, SQLite + goose mit `VACUUM INTO`, sqlc, statfs-Prüfung, Job-Queue-Gerüst, goqite-Prüfung (ADR 0002) |
| 4 Vertrag | `feat(api)` | `api/openapi.yaml` (Health, Problem), Codegen Go und TypeScript, Schema-Test `x-visibility`, Analyzer `itemsaccess`, Root-Taskfile |
| 5 Web | `feat(web)` | Vite 8, React 19, TanStack Router/Query, i18next de/en, PWA-Manifest, Token-Paket mit 8 Akzentfarben, ESLint/stylelint/Prettier |
| 6 Deployment | `feat(deploy)` | Dockerfile amd64, `compose.yaml` + nvidia/intel/amd, Test-Medien-Generator, Platzhalter-Baum-Generator |

### Definition of Done (Plan M0 plus 6.2) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| Container `healthy` in < 5 s | 1.17–1.30 s | `task deploy:healthy-time`, `docs/perf/M0.md` |
| Leerlauf-RSS < 60 MB | 13.5–14.0 MB | gleiche Messung, `VmRSS` aus `/proc/1/status` |
| Redaction-Test grün | PASS | `cd server && go test -v -run Redact ./internal/logging/`: 14 Fake-Secrets in Attributen, Gruppen, URLs (`api_key`, `st`), Fehlern und Nachrichtentext erscheinen nicht in der Ausgabe |
| Lint: Hex-Wert in CSS | schlägt an | `pnpm exec stylelint`: `color-no-hex`, `declaration-strict-value` |
| Lint: Literaltext in JSX | schlägt an | `pnpm exec eslint`: `i18next/no-literal-string` (JSX-Text, `aria-label`, `title`, `placeholder`) |
| Lint: `os/exec` ausserhalb `transcode` | schlägt an | `go tool golangci-lint run`: `depguard` (Import) und `forbidigo` (`exec.Command`) |
| Lint: Operation ohne `x-visibility` | schlägt an | `go test -run Contract ./internal/api/`: Fixture ohne Marker, mit Tippfehler und mit `ItemRef` ohne `scoped` schlagen fehl |
| Analyzer `itemsaccess` | schlägt an | `SQL "FROM items" accesses a title table; query through the visibility package instead` |
| Migration erzeugt vorher Sicherung | PASS | `go test -v ./internal/store/`: Sicherung `backups/slouchcat-<version>-<zeit>.db` vor anstehender Migration, Inhalt geprüft, 6 → 5 Sicherungen |
| `task ci` grün (6.2 Pkt. 2) | exit 0 | gen, gen:check (keine Codegen-Drift), lint (0 issues), Go-Tests mit `-race`, Vitest (9 Tests), Build |
| Kontrast Akzentfarben ≥ 4.5:1 | 6.76–11.32 | Vitest in `packages/design-tokens` druckt die Tabelle |
| trixie-Basis verifiziert (ADR) | ja | `jellyfin-ffmpeg7` 7.1.4-3-trixie für amd64 und arm64 im Jellyfin-Repository |

Nicht anwendbar in M0: Abnahmematrix-Zeilen (erst ab M1a), Playwright-Tastaturtests (keine Seite mit
Bedienung), eingefrorener DB-Stand (ab M1a), Gerätecheckliste.

## Was läuft

```bash
task ci                      # alles bauen und prüfen
task server:dev              # Server auf :8096 mit .dev/config und .dev/cache
task web:dev                 # Vite auf :5173, Proxy /api → :8096
task deploy:build            # Image slouchcat:dev (amd64)
task deploy:healthy-time     # Messung healthy-Zeit und RSS
task testdata:media          # Test-Medien nach testdata/media (braucht ffmpeg)
task testdata:tree           # Platzhalter-Baum mit 60'800 Dateien
```

Bis zur nächsten Anmeldung braucht Docker ein `sg docker -c '...'` um den Befehl, weil die
Gruppenmitgliedschaft noch nicht aktiv ist.

## Abweichungen und Entscheide während M0

- **OpenAPI bleibt 3.1.0**: oapi-codegen v2.8 liest 3.1 direkt; kein Downgrade nötig.
- **Idempotenz-Schlüssel** ist eindeutig unter aktiven Jobs (`queued`, `running`), nicht für immer; sonst
  wäre ein zweiter Scan desselben Pfads unmöglich (ADR 0002).
- **`PRAGMA foreign_key_check`** in Migrationen wirkt unter goose nicht (Ergebniszeilen werden verworfen);
  der Store prüft nach `Up` selbst und bricht bei Verletzungen ab. Die Vorlage behält die Zeile.
- **Dateisystem-Erkennung** (`statfs`) liegt im Paket `paths`, nicht in `store`, weil M1a sie für
  Bibliothekswurzeln braucht.
- **`queries/scoped/`** ist leer; der sqlc-Eintrag ist auskommentiert, bis die erste Abfrage mit
  `visible_items` kommt (M1a). Der Test „jede scoped-Abfrage enthält `visible_items`" folgt dann.
- **Versionen Web**: Vite 8 (nicht 7), stylelint 17 (nicht 16), TypeScript 6.0 (typescript-eslint
  verlangt < 6.1), ESLint 9 (eslint-plugin-react und jsx-a11y unterstützen ESLint 10 noch nicht).
  Drei Pakete sind exakt gepinnt, weil pnpm 12 Releases jünger als ein Tag ablehnt.
- **NVIDIA-Override** nutzt `capabilities: [gpu, compute, video, utility]`, weil Compose damit
  `NVIDIA_DRIVER_CAPABILITIES` überschreibt und `compute` für CUDA-Filter nötig ist.
- **Keine Debian-VA-Treiberpakete** im Image: jellyfin-ffmpeg liefert libva, iHD, i965, oneVPL und
  radeonsi selbst (wie das offizielle Jellyfin-Image).
- **PGS-Untertitel** kann der Generator nicht erzeugen (ffmpeg hat keinen PGS-Encoder); Zeile 10.3 „H.264 +
  PGS" braucht in M4 eine andere Quelle.
- **App-Icon** ist ein SVG mit zwei fest eingetragenen Farbwerten (dokumentiert); PNG-Icons folgen mit C-1
  in M1a. Die Schrift ist ein System-Stack; die selbst gehostete variable Schrift kommt mit dem
  Design-Brief in M1b.
- **Go-Modulpfad** `github.com/slouchcat/slouchcat/server` ist ein Platzhalter bis das GitHub-Repository
  existiert; Image-Name `ghcr.io/slouchcat/slouchcat` ebenso.

## Offene Punkte für M1a (neuer Auftrag)

1. GitHub-Remote anlegen (Marco), Modulpfad und Image-Owner umbenennen, Branch pushen.
2. GitHub-Actions-Workflow, der `task ci` ausführt (Plan 4); später Image- und Release-Publikation.
3. Scheduler für `job_schedules` mit dem ersten periodischen Job.
4. Test „jede Abfrage in `queries/scoped/` enthält `visible_items`" mit der ersten scoped-Abfrage.
5. ESLint 10, sobald eslint-plugin-react und jsx-a11y es unterstützen.
6. Proxmox-Testcontainer als schwaches NAS anlegen und die Lastmessung über SMB/NFS mit dem
   Platzhalter-Baum fahren (Entscheid 3).
7. iOS/macOS bleiben ungeprüft (Entscheid 2).
