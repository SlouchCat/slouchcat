# Slouchcat

Selbst gehosteter Media-Server mit kompromisslosem Design. Eigenständige, von Emby und Jellyfin
losgelöste Lösung für einen Haushalt und seine Gäste: Filme, Serien, Hörbücher und Podcasts, Profile mit
Altersfreigabe, kuratierte Startseite, Transcoding mit GPU, echte Offline-Wiedergabe in der PWA.

Status: Version 1.0. Die Meilensteine M0 bis M10 sind umgesetzt; der Nachweis je Abnahmezeile steht in
`docs/dev/ABNAHME-1.0.md`, der laufende Stand in `docs/dev/CURRENT_STATUS.md`.

**Installieren:** `docs/INSTALLATION.md`.

## Aufbau

```
api/openapi.yaml          Vertrag (Quelle für Server und Web-Client)
server/                   Go-Server (cmd/slouchcat, internal/<modul>, migrations/, queries/, tools/)
web/                      React-PWA (Vite)
packages/design-tokens/   Design-Tokens (DTCG) → CSS Custom Properties und TypeScript
packages/api-client/      generierter TypeScript-Client
deploy/                   Dockerfile, compose.yaml und GPU-Overrides
testdata/                 Generator für Test-Medien, DB-Stände
docs/spec/                Anforderungen (Deep-Interview-Spec, 64 Abnahmekriterien)
docs/planning/            Vision, Plan V1, Entscheidungsprotokoll, Reviews
docs/adr/                 Architekturentscheide
docs/dev/                 Entwicklungs-Setup, aktueller Stand, Abnahmebericht
docs/INSTALLATION.md      Installationsanleitung für den Betrieb zu Hause
```

## Entwicklung

Voraussetzungen und Einrichtung: `docs/dev/SETUP.md`. Einstieg ist `task`:

```bash
task ci
```

baut und prüft Server, Web-App und Vertrag so, wie es auch die CI tut. Weitere Ziele: `task --list`.

## Arbeitsweise

Planung und Umsetzung laufen in Claude Code mit oh-my-claudecode; die Regeln für Agenten stehen in
`CLAUDE.md`. Dokumentation auf Deutsch (Schweizer Schreibweise, ss statt ß), Code und Bezeichner auf
Englisch.

## Lizenz

Slouchcat ist freie Software unter der GNU Affero General Public License, Version 3 (`LICENSE`).
Das Docker-Image enthält ffmpeg aus dem jellyfin-ffmpeg-Build (GPL); der Lizenzordner `/licenses` im Image
und jedes Release enthalten den Quellcode-Hinweis.
