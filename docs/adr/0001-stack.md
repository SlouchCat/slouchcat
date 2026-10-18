# ADR 0001: Stack für Slouchcat V1

- Status: angenommen (18. Oktober 2026)
- Quelle: `docs/planning/03-plan-v1.md`, Abschnitte 2 bis 4 und 12; `docs/planning/04-entscheidungen.md`
- Betroffene Spec-IDs: alle; direkt C-2, C-5, A-4, A-9

## Kontext

Slouchcat ist ein selbst gehosteter Media-Server für einen Haushalt, der als ein Docker-Container auf
Linux läuft, auch auf schwacher Hardware (NAS). Die Spec lässt Sprache, Framework und Datenbank offen und
nennt drei Kriterien: sehr stabil, läuft überall gut, performant. Die Entwicklung erfolgt solo mit
Claude-Code-Agenten.

Gewichtete Entscheidungstreiber (Plan 2.2):

1. Betriebsstabilität auf schwacher Hardware (40 %): Leerlauf-RSS < 150 MB, Start < 5 s, amd64 und arm64,
   keine Zusatzdienste, Langzeit-Kompatibilität.
2. Zuverlässige Prozess- und Streamingsteuerung (35 %): viele ffmpeg-Kindprozesse, Range, HLS,
   Admission Control.
3. Solo-Wartbarkeit mit Agenten (25 %): eine Regel an einer Stelle, wenig Abhängigkeits-Churn.

## Entscheid

### Server

| Bereich | Entscheid |
|---|---|
| Sprache | Go, immer eine der zwei neuesten unterstützten Versionen (Stand Oktober 2026: 1.27 oder 1.26), gepinnt in `server/go.mod` und `.tool-versions` |
| HTTP | `net/http` mit `chi` als Router |
| API-Vertrag | OpenAPI 3.1 in `api/openapi.yaml`; `oapi-codegen` (strict server) für JSON-Endpunkte; Binär- und Streaming-Endpunkte (Range, HLS, Bilder) sind im Vertrag beschrieben, aber als `http.Handler` umgesetzt; jede Operation trägt `x-visibility` (`public`, `scoped`, `system`, `admin`) |
| Fehlerformat | RFC 9457 Problem Details (`application/problem+json`) mit zusätzlichem Feld `code` |
| Datenbank | SQLite im WAL-Modus über `modernc.org/sqlite` (ohne CGO); eine Schreibverbindung, N Leseverbindungen; Datei in `/config`, das nie auf NFS/SMB liegen darf (`statfs`-Prüfung beim Start) |
| Abfragen | `sqlc` für statische Abfragen, getrennt in `queries/scoped/` (müssen `visible_items` enthalten) und `queries/system/` (Paket `store/systemq`, Import nur in erlaubten Paketen); Query-Builder in `visibility` für Listen und Suche (ab M1a) |
| Migrationen | `goose`, nur vorwärts, eingebettet; Vorlage mit `-- +goose NO TRANSACTION`, `PRAGMA foreign_keys=OFF`, `PRAGMA foreign_key_check`, `PRAGMA foreign_keys=ON`; vor jedem Lauf `VACUUM INTO` nach `/config/backups/`, die letzten 5 Sicherungen bleiben |
| Jobs | eigene persistente Queue in SQLite (Lease, Idempotenz-Schlüssel, Debounce, Retry mit Backoff, Priorität 0–9, Abbruch, Zeitpläne); `goqite` wurde in M0 mit Timebox geprüft, Ergebnis in ADR 0002 |
| Logging | `log/slog` mit Redaction-Handler für `password`, `pin`, `token`, `apiKey`, `Authorization`, `X-Api-Key`, Query-Parameter `api_key` und `st` |
| Transcoding, Bilder | ffmpeg/ffprobe aus dem jellyfin-ffmpeg-Build, ausschliesslich hinter dem Paket `transcode`; `os/exec` ist sonst verboten |
| Lint | `golangci-lint` mit `forbidigo`, `depguard`, `gosec`; eigener Analyzer `itemsaccess`; `govulncheck` |

### Web

| Bereich | Entscheid |
|---|---|
| Laufzeit | Node.js aktive LTS (heute 24) nur für Build-Werkzeuge; pnpm Workspaces |
| App | React (aktuell 19) + TypeScript + Vite, reine SPA als PWA (`vite-plugin-pwa`); `@tanstack/react-router`, `@tanstack/react-query` |
| UI-Grundbausteine | `react-aria-components`, `@noriginmedia/norigin-spatial-navigation` (ab M1a) |
| Offline | OPFS für Downloads, IndexedDB (`idb`) für Metadaten (ab M1a/M5) |
| Player | `<video>`/`<audio>` + `hls.js`, natives HLS in Safari (ab M1a) |
| Design-Tokens | DTCG-Format in `packages/design-tokens`, Style Dictionary erzeugt CSS Custom Properties und TypeScript-Typen; CSS Modules; `stylelint` mit `color-no-hex` und `declaration-strict-value`; `eslint-plugin-i18next` gegen Literaltexte |
| i18n | `i18next` + `react-i18next`, Sprachdateien `web/src/locales/{de,en}.json` |
| API-Client | `openapi-typescript` + `openapi-fetch` in `packages/api-client`, generiert aus `api/openapi.yaml` |
| Tests | Go `testing` + `testify`; Vitest + Testing Library; Playwright (ab M1a) |

### Auslieferung

- Ein Container `slouchcat` auf Basis `debian:trixie-slim` mit Go-Binary, eingebetteter Web-App,
  jellyfin-ffmpeg und Intel-/AMD-Mediatreibern; amd64 ab M0, arm64 ab M1b. Fehlt ein jellyfin-ffmpeg-Paket
  für trixie auf einer Architektur, bleibt diese Architektur auf `bookworm-slim` (hier zu dokumentieren).
- Volumes `/config` (Datenbank, Einstellungen, Sicherungen, `secret.key`), `/cache` (regenerierbar),
  `/media/...` read-only.
- Optionaler Container `caddy` (Compose-Profil `https`, ab M2).
- Lizenzordner `/licenses` im Image mit GPL-Hinweis und Quellcode-Angebot für ffmpeg.

### Veröffentlichung und Lizenz

Entscheid von Marco (04-entscheidungen.md, Frage 1): Slouchcat ist Open Source unter **AGPL-3.0**
(`LICENSE`). Quellcode, Docker-Image (ghcr.io) und Release-Manifest sind öffentlich; ein NAS braucht für
die Update-Prüfung (A-5) kein Token. Annahme AN-6 des Plans (privates Repository möglich) ist damit
hinfällig.

## Erwogene Alternativen

- **Option A, TypeScript durchgehend** (Node + Fastify + Kysely): stärkste Alternative dank einer Sprache
  ohne Codegen-Drift; verworfen wegen häufigerer Pflichtwechsel der Node-LTS, npm-Abhängigkeiten im
  Server und 80–120 MB grösserem Image. Bleibt Rückfall nach Plan 2.5 (Prüfung am Ende von M1b).
- **Option C, .NET-Server**: performant und reif, aber grössere Laufzeit, LTS-Zwang und Nähe zu Jellyfin.
- **Rust** (Entwicklungstempo solo) und **Python** (Leistung bei parallelem Streaming) invalidiert.
- **PostgreSQL, Redis**: Zusatzcontainer ohne Nutzen für einen Haushalt.
- **Bildverarbeitung mit `govips` (CGO) oder `x/image`**: verworfen; Bilder laufen über ffmpeg.
- **Separater Transcode-Container**: verworfen zugunsten Admission Control im Prozess.

## Konsequenzen

- Zwei Sprachen und ein Codegen-Schritt je API-Änderung; CI prüft Codegen-Drift (`task ci`).
- Generierter Code (`packages/api-client`, `packages/design-tokens/dist`, `server/internal/api/gen`,
  `server/internal/store/*q`) wird nie von Hand geändert.
- SQLite mit einem Schreiber; Mandantenfähigkeit ist ausgeschlossen (Non-Goal).
- ffmpeg liegt als GPL-Binary im Image; das Release enthält ein Quellcode-Angebot.
- AGPL-3.0 verlangt, dass Änderungen an einem öffentlich betriebenen Server als Quellcode verfügbar sind;
  für den Haushaltsbetrieb entsteht daraus keine Pflicht.

## Offene Folgepunkte

- Verifikation von `debian:trixie-slim` als Basis mit jellyfin-ffmpeg-Paketen (M0, Schritt 6; braucht
  Docker auf dem Entwicklungsrechner).
- Go-Modulpfad: bis das öffentliche GitHub-Repository existiert, lautet er `github.com/slouchcat/slouchcat`;
  er wird beim Anlegen des Remotes auf den echten Besitzer umbenannt (ein `sed` über das Repository).
- Design-Brief mit Schrift und Palette in M1b; TVDB-Lizenz vor M3.
