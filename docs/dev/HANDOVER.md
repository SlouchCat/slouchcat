# Übergabe an eine neue Hauptsession

Stand: 21. Oktober 2026. Diese Datei ersetzt das Gedächtnis der bisherigen Hauptsession (Claude Code,
Account 1). Wer sie liest, übernimmt die Rolle der Hauptsession. Alles andere steht im Repository:
`docs/dev/CURRENT_STATUS.md` (Stand mit Belegen), `docs/planning/03-plan-v1.md` (freigegebener Plan),
`docs/planning/04-entscheidungen.md` (verbindliche Antworten von Marco), `CLAUDE.md` (Regeln für Agenten).

## 1. Wer Marco ist und wie er arbeitet

- Marco ist kein Entwickler. Er entscheidet, bestätigt und testet am Handy; er schreibt keinen Code.
- Deutsch, Schweizer Schreibweise (ss statt ß), du. Antworten in Klartext, Fachbegriffe erklären.
- Code, Bezeichner, Commits und PR-Beschreibungen auf Englisch, Commit-Titel mit Bereich:
  `feat(web): …`, `docs(status): …`. Dokumentation auf Deutsch.
- Befehle mit sudo führt nur Marco aus, im Terminal-Panel der App (`run_in_terminal`, dann `read_terminal`).
  Schlüssel und Passwörter nie im Chat; Marco trägt sie selbst in `deploy/.env` ein (gitignored).
- Marco hat nur Android-Testgeräte. iOS, iPadOS und Safari gelten als ungeprüft (Entscheid 2).

## 2. Rolle der Hauptsession

Die Hauptsession baut nichts selbst. Sie

1. startet pro Meilenstein eine **frische Session** mit vollständigem Briefing (Vorlage in Abschnitt 7),
2. meldet Marco Zwischenstände in ein, zwei Sätzen, wenn eine Session einen Turn beendet,
3. prüft am Ende den Pull Request (Checks, „Not covered"), holt Marcos Wort zum Merge ein,
4. merged, zieht `main` nach, **baut das Docker-Image neu** und startet den Container (Abschnitt 5),
5. trägt Marcos Antworten und Testergebnisse ins Entscheidungsprotokoll bzw. in `docs/perf/` ein,
6. schlägt den nächsten Meilenstein als Karte vor (`start_session`, own_initiative).

Regeln, die sich bewährt haben:

- **Nie direkt nach `main` pushen.** Der Sicherheitsfilter lehnt das ab (CI-Bypass). Immer Branch plus
  Pull Request, auch für Einzeiler. Merge nur mit Marcos Wort, ausser er hat es für einen konkreten PR
  vorab erlaubt.
- **Der Arbeitsbaum ist geteilt.** Andere Sessions (Meilenstein-Sessions, Marcos Ideensammlung) wechseln
  den Branch im selben Ordner. Vor jeder Git-Aktion `git branch --show-current` und `git status` prüfen,
  nichts verwerfen. `main` ohne Branchwechsel aktualisieren: `git fetch origin main:main`.
- **Image nie aus dem Arbeitsbaum bauen**, sondern aus einem sauberen Worktree von `origin/main`
  (Abschnitt 5), sonst landet halbfertiger Code im laufenden Container.
- CI-Läufe nicht pollen. PR an die Session binden (`bind_pr`) und Auto-fix einschalten (`set_monitor`),
  dann weckt die App bei Fehlern.
- Jede Meilenstein-Session beginnt mit den Fragen aus `04-entscheidungen.md`, die „vor Mx" fällig sind,
  und trägt die Antworten mit Datum ein.

## 3. Stand der Meilensteine (Plan: 12 Meilensteine, M0 bis M10)

| Meilenstein | Stand | Pull Request |
|---|---|---|
| M0 Fundament | gemerged | #0 (direkt), Merge 98c17b8 |
| M1a Scan, Direct Play, Startseite | gemerged | #1, Nachtrag #2 (Proxmox-Testcontainer 190, SMB-Messung) |
| M1b HLS, Seeking, Stream-Tokens, Bildpipeline, Design, arm64 | gemerged | #3 |
| M2 Konten, Profile, Zugriff, HTTPS | gemerged | #4 |
| M3 Bibliothek komplett (Serien, Versionen, Extras, Trailer, Pflege, 60k-Lasttest) | gemerged | #5 |
| M4 Wiedergabe komplett (Geräteprofile, HDR je Backend, Spuren, Limits, Intro-Skip) | gemerged | #6 |
| M5 Offline (fMP4-Downloads, Staffel/Auto, Offline-PIN und -Schauzeit, Merge, Entzug) | gemerged | #10 |
| M6 Hörbücher und Podcasts | gemerged 21.10. | #11 |
| CI: Go-Testlimit 30 min | gemerged 21.10. | #12 |
| **M7 Entdecken und Kuratierung** | **fertig**, wartet auf Marcos Merge-Wort; lokal `task ci` grün, Verifier mit Ausnahmen (Android-Checks, O-6 bis 19. November, Messung auf CT 190 offen). Branch `claude/m7-discover`. | #14 (offen, Checks laufen) |
| M8 Administration und Benachrichtigungen | offen | |
| M9 Integrationen (Radarr/Sonarr lesend, Seerr-Bereich, Bazarr, SABnzbd) | offen; Frage 10 vorab | |
| M10 Härtung und Release | offen | |

Weitere offene Pull Requests: #9 `claude/ideas-collection` ist Marcos Ideensammlung für nach 1.0 (nicht
Teil der Meilensteine; mergen nur auf sein Wort).

**Nächste Schritte der Hauptsession:** Pull Request #14 prüfen (Checks, Abschnitt „Not covered"), Marcos Wort
zum Merge einholen, mergen, Image neu bauen (Abschnitt 5), dann M8 als Karte vorschlagen (Abschnitt 7).

## 4. Offene Punkte bei Marco (nicht blockierend)

- Android-Checklisten: M5 (`docs/perf/M5.md` Abschnitt 7: Download, Staffel, Flugmodus, „nur WLAN") und
  M6 (`docs/perf/M6.md` Abschnitt 5: Hörbuch, Podcast, Flugmodus mit Kapitelsprung). Ergebnisse trägt die
  Hauptsession ein.
- Offline-Spike: Tage 3 und 8 (bis 27. Oktober 2026) auf dem Handy prüfen; danach Spike-Seite und
  `SLOUCHCAT_SPIKE_DIR` entfernen (eigener kleiner PR).
- O-6-Langzeittest (30 Tage ab 20. Oktober, Protokoll `docs/perf/M5-o6-langzeittest.md`): wird in M7
  weitergeführt und erst nach dem 19. November abgehakt.
- HTTPS-Zugang (K-8) mit Marcos echter Domain testen.
- Entscheidungsprotokoll: Frage 9 Trakt-Teil (vor/in M7), Frage 10 Wünschen (vor M9).

## 5. Betrieb auf Marcos Rechner

- Linux Mint 22.3, Host „Gaming", Benutzer `marco`. NVIDIA-GPU mit NVIDIA-Container-Laufzeit.
- Werkzeuge: Go, pnpm, task unter `~/.local/bin` (ggf. `export PATH="$HOME/.local/bin:$PATH"`); Node 24
  unter `~/.local/share/node`, verlinkt nach `/usr/local/bin`; Docker 29 mit Compose; ffmpeg; gh CLI
  angemeldet als MarcoBrechbuhl. Marco ist in der Gruppe `docker`; bis zur Neuanmeldung braucht er
  `sg docker -c "…"`. Für die Hauptsession funktioniert `sg docker -c` ebenfalls.
- Laufender Server: Container `slouchcat` aus `deploy/compose.yaml`, Image-Tag
  `ghcr.io/slouchcat/slouchcat:0.1`, Port 8096, Daten in Volumes, Medien read-only von SMB/NFS.
  `deploy/.env` enthält TMDB- und YouTube-Schlüssel (nie committen, nie anzeigen).
- Testserver: Proxmox-CT 190 im lokalen Netz (Intel QuickSync), Zugang in `docs/dev/TESTSERVER.md`.
- **Image nach jedem Merge neu bauen** (aus sauberem Worktree, nie aus dem Arbeitsbaum):

```bash
cd /mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat && git fetch origin && W=/tmp/slouchcat-build && rm -rf "$W" && git worktree prune && git worktree add --detach "$W" origin/main && (cd "$W" && sg docker -c "docker build -f deploy/Dockerfile -t ghcr.io/slouchcat/slouchcat:0.1 .") && (cd deploy && sg docker -c "docker compose up -d slouchcat") && git worktree remove --force "$W"
```

- Kontrolle: `cd deploy && sg docker -c "docker compose ps"` muss `healthy` zeigen.
- oh-my-claudecode (OMC) 5.7.5 ist als Plugin für den Linux-Benutzer installiert (`~/.claude`), gilt also
  auch für einen anderen Claude-Account auf diesem Rechner. Skills: `execute`, `ralph`, `team`, `verify`,
  `review`, `plan`, `deep-interview`.

## 6. Projekt in einem Absatz

Slouchcat ist Marcos eigenständiger, selbst gehosteter Media-Server (Emby/Jellyfin-Alternative) mit
dunklem, kinoartigem, ordentlichem Design. Go-Server mit SQLite (goose, sqlc, eigene Job-Queue), React-19-PWA
(Vite, TanStack, hls.js, MSE-Offline-Player, Service Worker, i18next de/en, Design-Tokens, Storybook,
Playwright mit stummem Firefox), OpenAPI-Vertrag mit Codegen für Go und TypeScript, ein Docker-Container
mit jellyfin-ffmpeg. Version 1 umfasst Filme, Serien, Hörbücher, Podcasts; Netflix-artige Profile mit
Altersstufen und PIN; Cat's Empfehlungen (Admin-Kuratierung); Transcoding mit GPU; Offline-Wiedergabe;
Integrationen lesend (Radarr/Sonarr, Seerr, Bazarr, Trakt-Listen). Nicht-Ziele: Musik, Fotos, Live-TV,
native Apps, Plugins, algorithmische Empfehlungen. Repository öffentlich unter AGPL-3.0:
https://github.com/SlouchCat/slouchcat. Spezifikation mit 68 Abnahmekriterien (IDs K/B/H/W/E/O/C/A/I) in
`docs/spec/01-anforderungen-v1.md`; Sammlung rund 2'500 Filme und 40'000 Folgen auf SMB/NFS.

## 7. Vorlage für das Briefing einer Meilenstein-Session

`start_session` mit `context: "fresh"`, `use_worktree: false` (die Session arbeitet im geteilten Ordner auf
einem eigenen Branch). `background` fasst Abschnitt 6, den Stand der Meilensteine und die Werkzeuge zusammen.
`prompt` enthält:

1. „Setze Meilenstein Mx ‚…' aus docs/planning/03-plan-v1.md um (Abschnitt ### Mx) und nichts aus Mx+1."
2. Pflichtlektüre: CLAUDE.md, docs/dev/CURRENT_STATUS.md, Plan-Abschnitte (Architektur, Roadmap Mx, 6.2 DoD,
   7 Abnahmematrix für die betroffenen IDs, 10.2 Vertragstest), 04-entscheidungen.md, betroffene Spec-IDs.
3. Vorab zu klären: die Fragen aus 04-entscheidungen.md, die „vor Mx" fällig sind (AskUserQuestion, Klartext,
   Plan-Vorschlag als empfohlene erste Option; Antwort mit Datum eintragen und committen); benötigte
   Schlüssel erklärt Marco in drei Sätzen, er trägt sie selbst in deploy/.env ein.
4. Umfang: die Schritte aus dem Plan-Abschnitt, ausformuliert, mit den Spec-IDs.
5. Definition of Done: Plan-DoD plus 6.2, Abnahmematrix-Zeilen grün mit Belegen, Kinderprofil-Vertragstest
   10.2 für alle neuen Operationen, Playwright-Tastaturtests, `task ci` grün, Code-Review-Passage und
   Verifier, `docs/dev/CURRENT_STATUS.md` und `docs/perf/Mx.md` mit Belegen (Befehl und Ausgabe).
6. Arbeitsweise: OMC-Ausführungs-Skill (execute oder ralph) mit dem Plan als Kontext; ein englischer Commit
   je Schritt mit Bereich im Titel; Branch pushen und am Ende Pull Request gegen main öffnen (Summary,
   Evidence, Not covered), nicht mergen ohne Marcos Wort; keine Nicht-Ziele; Regeln der CLAUDE.md strikt;
   fehlende Werkzeuge mit genauem Befehl nennen, sudo macht Marco; nach dem Merge baut die Hauptsession das
   Image neu.

Die bisherigen Briefings für M0 bis M7 stehen in den jeweiligen Sessions; der nächste ist M8
(Plan-Abschnitt „### M8 – Administration und Benachrichtigungen": Einrichtungsassistent vollständig,
zentrale Verwaltung, Dashboard, Sicherung und Wiederherstellung, Update-Hinweis, Benachrichtigungskanäle
E-Mail/Web-Push/Telegram, Ankündigungen, Statistiken; Spec-IDs A-1 bis A-9 und E-8 Kanäle, Entscheide 5.2 und 5.5).
