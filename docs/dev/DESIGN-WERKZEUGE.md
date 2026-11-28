# Design-Werkzeuge

Arbeitsumgebung für den UI-/UX-Relaunch und das Rebranding, eingerichtet am 28. November 2026
(Auftrag «Installation und Einrichtung der Design- und Entwicklungswerkzeuge»). Hier steht, was
installiert ist, wo es liegt, wer es benutzt, wie es geprüft wurde und wie es wieder weggeht.

Massgeblich bleiben `CLAUDE.md`, das Entscheidungsprotokoll und die ADRs. Die Skills von aussen sind
Ratschläge: Wo sie Tailwind, shadcn, Next.js, Hex-Farben im Code oder eigene Dateien wie
`DESIGN.md` voraussetzen, gilt das Repository.

## Auf einen Blick

| Was | Wo | Einrichtung |
|---|---|---|
| Design-Agents (7) | `.claude/agents/*.md` (im Repo) | Agent-Dateien, Abschnitt «Agents» |
| Skills von aussen (25) | `~/.claude/skills/<name>/` | `scripts/install-design-skills.sh` |
| Skills ein- und ausblenden | `.claude/settings.json` (Repo: an), `~/.claude/settings.json` (sonst: aus) | dasselbe Skript |
| Werkzeugkiste (Grafik, Prüfung, Browser-MCP) | `~/.local/share/slouchcat-design/` | `scripts/install-design-tools.sh` |
| Versionen der npm-Werkzeuge | `scripts/design-tools/package.json`, `package-lock.json` | gepinnt, `npm ci` |
| Eigene Prüfwerkzeuge `sc-*` | `scripts/design-tools/tools/` | vom Installer kopiert |
| Funktionsprüfung | `scripts/design-tools-check.sh` | 45 Prüfungen, eine Zeile je Prüfung |
| Echte Medienbilder für Mockups | `~/.local/share/slouchcat-design/media/<satz>/` | `scripts/design-media.sh` |
| MCP-Server für eine CLI-Sitzung | `scripts/design-tools/mcp.json` | `claude --mcp-config scripts/design-tools/mcp.json` |
| Sicherungen vor der Einrichtung | `~/.claude/backups/*.vor-design-werkzeuge-2026-11-28` | einmalig |

Einrichten auf einem frischen Rechner (ohne sudo, rund 5 Minuten, rund 0,9 GB):

```bash
scripts/install-design-tools.sh
scripts/install-design-skills.sh
scripts/design-tools-check.sh
```

## Status der Bausteine

Prüfdatum für alles: 28. November 2026. Die Begriffe sind die aus dem Auftrag.

| Baustein | Version | Status |
|---|---|---|
| Claude Code (CLI im Terminal) | 2.1.302 | Bereits vorhanden und erfolgreich getestet |
| Claude Code (in der Desktop-App) | 2.1.330 | Bereits vorhanden und erfolgreich getestet |
| oh-my-claudecode (Plugin und CLI) | 5.8.4 | Bereits vorhanden und erfolgreich getestet |
| oh-my-claudecode 5.8.9 (Update) | 5.8.9 | Nicht erforderlich (Begründung unter «oh-my-claudecode») |
| Plugin `frontend-design` (Anthropic) | 1.1.1 | Bereits vorhanden und erfolgreich getestet (identisch mit upstream) |
| Plugins `ui-consistency`, `caveman` | 1.4.0, 4.2.0 | Bereits vorhanden, unverändert |
| 25 Skills von aussen (Tabelle «Skills») | gepinnte Commits | Installiert und erfolgreich getestet (erkannt; Aufruf durch Agents siehe «Offen») |
| `impeccable` ohne Engine | 4.11.1 | Installiert und erfolgreich getestet |
| `impeccable`-Engine (Binary, Hooks) | engine 0.1.31 | Benötigt meine Zustimmung |
| `taste-skill` | 2.5.1 | Nicht kompatibel |
| 7 Design-Agents | im Repo | Installiert, aber noch nicht vollständig getestet (neue Sitzung nötig) |
| Playwright MCP | 0.0.86 | Installiert und erfolgreich getestet |
| Chrome DevTools MCP | 1.14.0 | Installiert und erfolgreich getestet |
| Context7 MCP (entfernt, ohne Schlüssel) | Dienst | Installiert und erfolgreich getestet |
| GitHub über `gh` | 2.45.0 | Bereits vorhanden und erfolgreich getestet |
| GitHub-MCP-Server | – | Nicht erforderlich (`gh` deckt alles ab) |
| Chrome for Testing | 158.0.8127.88 | Installiert und erfolgreich getestet |
| Inkscape (AppImage) | 1.4.4 | Installiert und erfolgreich getestet |
| ImageMagick (AppImage) | 7.1.2-36 | Installiert und erfolgreich getestet |
| svgo, sharp, resvg-js, png-to-ico, svg2vectordrawable, pwa-assets-generator | Tabelle «Werkzeugkiste» | Installiert und erfolgreich getestet |
| fonttools mit Brotli (WOFF2) | 4.69.1 | Installiert und erfolgreich getestet |
| Lighthouse, axe-core, pixelmatch, playwright-core | Tabelle «Werkzeugkiste» | Installiert und erfolgreich getestet |
| Playwright-Testsuite, Storybook mit 548 Baselines, Vitest, jsx-a11y | wie im Repo | Bereits vorhanden (nicht neu geprüft, CI prüft sie) |
| Android-SDK, adb, Emulatoren `sc_phone`, `sc_tablet`, `sc_tv`, `sc_phone28`, KVM | wie im Repo | Bereits vorhanden und erfolgreich getestet (`sc_tv`) |
| UI Automator (`uiautomator dump` im Emulator) | im System-Image | Bereits vorhanden und erfolgreich getestet |
| Design-CT auf pve2 | – | Nicht erforderlich (jetzt), Plan unten |

## oh-my-claudecode

Installiert sind Plugin `oh-my-claudecode@omc` 5.8.4 (Marketplace auf den Tag `v5.8.4` festgelegt)
und die CLI `omc` 5.8.4. 19 Agents, 84 Skills, MCP-Server `t` mit 50 Werkzeugen, Hooks für elf
Ereignisse. Claude Code meldet beim Start keine Plugin-Fehler.

**Update auf 5.8.9: nicht gemacht.** Die Neuerungen von 5.8.5 bis 5.8.9 betreffen fast nur die
Routing-Erweiterung «Jev» (braucht einen bezahlten Fremddienst, hier aus), Bedrock/Vertex und die
tmux-Worker; nichts davon ist hier in Gebrauch. Dagegen ändert 5.8.9 die Hook-Datei (neuer
Schlüssel `modules`, Zeitlimit des PreToolUse-Hooks 5 → 60 s) und braucht wieder `npm install` im
Cache-Ordner, während mehrere Claude-Sitzungen mit den 5.8.4-Hooks laufen. Weg für später, wenn
keine Sitzung läuft: `ref` in `~/.claude/settings.json` auf `v5.8.9`, dann
`claude plugin marketplace add "Yeachan-Heo/oh-my-claudecode#v5.8.9"` und
`claude plugin update oh-my-claudecode@omc`, im neuen Cache-Ordner
`npm install --omit=dev --ignore-scripts` und `npm rebuild better-sqlite3`, dann eine Testsitzung
vor den übrigen.

### Was automatisch geht und was nicht

| Entscheid | Wer entscheidet | Mechanismus |
|---|---|---|
| Modell eines Agents | Claude Code, automatisch | `model:` in der Agent-Datei; für OMC-Agents zusätzlich `.claude/omc.jsonc` (executor und designer auf Opus), ein Hook schreibt das Modell in den Aufruf |
| Denkaufwand eines Agents | Claude Code, automatisch | `effort:` in der Agent-Datei (low bis max); sonst der der Sitzung |
| Skills eines Agents | Claude Code, automatisch | `skills:` in der Agent-Datei lädt sie beim Start in den Kontext |
| Weitere Skills | der Agent selbst | Skill-Werkzeug, nach den Hinweisen in seiner Datei |
| Welcher Agent, wie viele parallel, welche Prüfung danach | das steuernde Modell | Anweisungen in `CLAUDE.md` und den OMC-Regeln; kein Code |
| Modell und Aufwand nach Schwierigkeit | niemand | OMC 5.8.4 hat dafür nur «Jev» (Fremddienst, aus); Claude Code wählt nicht selbst |
| Schlüsselwörter («ralph», «autopilot», «tdd», «ultrathink» …) | OMC-Hook, automatisch | Text in den Kontext; «ultrathink» setzt kein Denkbudget |
| Freigabe einfacher Lesebefehle | OMC-Hook, automatisch | nur `git status`, `gh … view/list`, `ls`, `cat`, `head`, `tail`, `sed -n`, `rg` auf Repo-Dateien ohne Geheimnisse |

Modelle: Fable 5.2 (`fable`), Opus 5.5 (`opus`), Sonnet 5.5 (`sonnet`), Haiku 5.5 (`haiku`). Die
Design-Agents laufen auf Opus nach dem Entscheid vom 18. Oktober 2026 («Code und Design schreibt
Opus»); `model: fable` wäre in jeder Agent-Datei möglich. Parallel: mehrere Agent-Aufrufe in einer
Antwort; Grenzen aus `CLAUDE.md` (höchstens drei Executor, schwere Läufe über `heavy-slot`).

## Agents

Sieben Projekt-Agents in `.claude/agents/`, jeder mit festem Modell, Aufwand, vorgeladenen Skills
und, wo nötig, eigenen MCP-Servern. Weitere Skills holt ein Agent bei Bedarf selbst; seine Datei
sagt welche. So lädt kein Agent alles, und die Vorgaben widersprechen sich nicht.

| Agent | Modell, Aufwand | Vorgeladen | Auf Abruf | MCP | Rolle |
|---|---|---|---|---|---|
| `design-director` | Opus, high | `impeccable`, `ui-ux-pro-max`; liest `frontend-design` zuerst | `emil-design-eng`, `animate`, `canvas-design`, `theme-generator`, `mobile-native`, `adaptive`, `leanback-to-compose-tv-migration` | – | Richtungen, Art Direction, klickbare Mockups |
| `ux-architect` | Opus, high | `ui-ux-pro-max` | `design-accessibility`, `adaptive`, `leanback-to-compose-tv-migration`, `impeccable` | – | Navigation, Struktur, Abläufe, Funktionsinventar |
| `brand-designer` | Opus, high | `canvas-design`, `icon-generator` | `theme-generator` | – | Logo, Icons, Favicons, App-Icons, Banner, Schriften |
| `motion-designer` | Opus, high | `emil-design-eng` | `animate`, `gestures`, `mobile-native`, `animation-performance`, `break-ui`, `impeccable` | – | Bewegung, Übergänge, Gesten, TV-Fokusbewegung |
| `ui-engineer` | Opus, Sitzung | `vercel-react-best-practices`, `vercel-composition-patterns`, `web-design-guidelines` | `jetpack-compose-performance`, `adaptive`, `edge-to-edge`, `leanback-to-compose-tv-migration`, `emil-design-eng` | `context7` | Freigegebenes Design als Code (Web, Compose) |
| `design-reviewer` | Opus, high | `impeccable`, `web-design-guidelines` | `ui-ux-pro-max`, `design-accessibility`, `emil-design-eng` | `playwright`, `chrome-devtools` | Unabhängige Kritik; Write und Edit gesperrt |
| `design-qa` | Sonnet, medium | `visual-qa`, `a11y-debugging` | `break-ui`, `responsive-audit`, `ui-edge-case-audit`, `design-accessibility`, `debug-optimize-lcp` | `playwright`, `chrome-devtools` | Messen: Screenshots, axe, Lighthouse, Pixelvergleich, Emulator |

Zuordnung zum Auftrag: kreative Teams → `design-director`; UX und Informationsarchitektur →
`ux-architect`; Branding → `brand-designer`; Interaktion → `motion-designer`; Frontend →
`ui-engineer`; unabhängige Reviews → `design-reviewer`; Qualitätssicherung → `design-qa`. Für
Server-Code, Planung und Code-Reviews bleiben die OMC-Agents zuständig (`executor`, `planner`,
`critic`, `code-reviewer`, `security-reviewer`, `verifier`); `oh-my-claudecode:designer` wird für
den Relaunch nicht gebraucht.

`design-reviewer` hat kein Write und Edit, aber wie die OMC-Prüfer Bash; seine Datei verbietet
Änderungen. Die MCP-Server der Agents starten nur, solange der Agent läuft (je rund 120 bis 150 MB,
dazu Chrome bei Gebrauch). Die Hauptsitzung in der Desktop-App hat dafür ihr eingebautes
Browser-Fenster; im Terminal bringt `--mcp-config scripts/design-tools/mcp.json` dieselben drei
Server in eine Sitzung.

## Skills

Kopiert aus dem jeweiligen Repository am geprüften Commit, nichts davon ausgeführt. Keine Hooks,
keine Befehle, keine MCP-Einträge aus diesen Quellen. Jeder Ordner hat eine Datei `.source` mit
Repository, Commit und Lizenz. In `~/.claude/settings.json` stehen alle 25 auf `off` (andere
Projekte sehen sie nicht), in `.claude/settings.json` des Repos auf `name-only` (gelistet ohne
Beschreibung, geladen bei Bedarf). Dass die Projekt-Einstellung gewinnt, ist geprüft (siehe
«Funktionsprüfung»).

| Skill | Quelle (Commit) | Lizenz | Zweck | Einschränkung |
|---|---|---|---|---|
| `ui-ux-pro-max` | nextlevelbuilder/ui-ux-pro-max-skill (`11ca2b3b`) | MIT | UX-Regeln, Paletten, Typografie, Stacks inkl. Compose; Suche `scripts/search.py` (nur Python-Standardbibliothek, offline) | Daten oft Tailwind und Hex; `--persist` schreibt `design-system/` ins Projekt, verboten |
| `impeccable` | pbakaus/impeccable (`2fc5e145`) | Apache-2.0 (`NOTICE.md` beiliegend) | Kritik, Audit, Feinschliff; Referenzen zu Android | ohne `scripts/`: Befehle `shot`, `live`, Detektor und `context` fehlen, der Skill liest den Kontext selbst |
| `emil-design-eng`, `animate`, `animation-performance`, `gestures`, `mobile-native`, `break-ui` | emilkowalski/skills (`64eb3deb`) | MIT | Bewegung, Gesten, Handy-Gefühl, Grenzfälle | empfiehlt teils Motion/Framer; Bibliotheken nur nach Entscheid |
| `design-accessibility`, `token-audit`, `responsive-audit`, `ui-edge-case-audit`, `visual-qa`, `icon-generator`, `theme-generator` | UXLana/design-skills-pack (`298a510c`) | MIT | WCAG 2.2, Token-Disziplin, Responsive, leere Zustände, SVG-Icons, Paletten mit Kontrast | ein Verfasser, 8 Sterne; nur Markdown und ein Kontrastskript |
| `canvas-design` | anthropics/skills (`e4bb5d96`) | Apache-2.0, 54 Schriften OFL-1.1 | Moodboards, Poster als PNG/PDF | kein SVG, kein Logo; fordert zum Laden weiterer Schriften auf (Lizenz vorher prüfen) |
| `web-design-guidelines`, `vercel-react-best-practices`, `vercel-composition-patterns` | vercel-labs/agent-skills (`5a37a632`) | MIT laut README, keine Lizenzdatei | Oberflächenregeln, React-Leistung, Komposition | 16 Regeln nur für Next.js/Server; ältere Fassungen luden Regeln aus dem Netz, diese nicht |
| `jetpack-compose-performance`, `adaptive`, `edge-to-edge`, `leanback-to-compose-tv-migration` | android/skills (`7f7b2173`) | Apache-2.0 | Compose-Leistung, Fenstergrössen, Randlos, TV-Muster | TV-Skill nennt alte Versionen (Coil 2, media3 1.9): nie danach Abhängigkeiten ändern |
| `a11y-debugging`, `debug-optimize-lcp` | ChromeDevTools/chrome-devtools-mcp (`597b029d`) | Apache-2.0 | Barrierefreiheit und LCP mit den DevTools-Werkzeugen | brauchen Chrome DevTools MCP ab 1.14 |

Dazu, schon vorher installiert: Plugin `frontend-design` 1.1.1 (eigenständige Designrichtung, gegen
generisches KI-Design; byte-gleich mit `anthropics/skills`).

**Aktualisieren:** Commit in `scripts/install-design-skills.sh` ändern (vorher den Unterschied lesen),
Skript neu starten. **Entfernen:** `scripts/install-design-skills.sh --uninstall` (löscht nur Ordner
mit `.source` und die eigenen Einträge in `skillOverrides`). Vor jeder Änderung legt das Skript
`~/.claude/settings.json.bak-design-skills` an.

### Bewusst nicht installiert

| Baustein | Grund | Status |
|---|---|---|
| `taste-skill` (Leonxlnx) | Für Firmen-Websites und Landingpages gebaut, nicht für App-Oberflächen; Stop-Hook startet bei jedem Sitzungsende Node und Chrome (bis 600 s); 40 MB Bilder, davon 86 unter CC BY-SA/CC BY mit Nennungspflicht. Kreative Richtung decken `frontend-design` und `impeccable` | Nicht kompatibel |
| `impeccable`-Engine und Plugin-Hooks | lädt beim ersten Gebrauch ein Rust-Binary von GitHub und führt es aus (geprüft nur gegen eine Prüfsumme aus demselben Repo); Hooks bei SessionStart, nach jedem Edit/Write und bei Stop in allen Projekten; Update-Abfrage bei impeccable.style; Konto mit Upload. Freischalten: `IMPECCABLE_ENGINE=1 scripts/install-design-skills.sh` (ohne Hooks) | Benötigt meine Zustimmung |
| übrige Skills von ui-ux-pro-max (`design`, `brand`, `ui-styling`, `banner-design`, `slides`, `design-system`) | `design` liest `~/.claude/.env` und ruft bezahlte Bild-APIs; `ui-styling` startet `npx shadcn`; Tailwind/shadcn | Nicht kompatibel |
| übrige 14 Skills von emilkowalski | React Native/Expo, Swift, Sonner, Bibliothekswahl, Prototyp-Werkzeug | Nicht erforderlich |
| übrige 15 Skills von UXLana | Inhalte für LinkedIn, Jira, Vercel-Hosting, Mermaid über `npx` | Nicht erforderlich |
| übrige Skills von anthropics/skills (`example-skills`) | doppelt (`frontend-design`), Tailwind/shadcn, Anthropic-eigene Marke | Nicht erforderlich |
| `deploy-to-vercel`, `vercel-cli-with-tokens`, `vercel-optimize`, `react-native-skills` | Vercel-Hosting, Token, React Native | Nicht kompatibel |
| `testing-setup` (android/skills) | will Hilt, Mockk, JaCoCo einführen; die Tests der Apps stehen | Nicht erforderlich |
| Offizielles Chrome-DevTools-Plugin | sendet ohne Zusatz Nutzungsstatistik und Seitenadressen (CrUX) an Google; eigene Einrichtung ohne beides | Nicht erforderlich |
| `accessibility-engineering` (addyosmani), `brand-identity-skill`, `dembrandt-skills` | doppelt zu `sc-axe`/`design-accessibility`; zwei Monate alt mit 1 MB Python; Informationsarchitektur doppelt | Nicht erforderlich |

## MCP-Server

Alle drei sind nur den Agents zugeordnet, die sie brauchen (Feld `mcpServers` der Agent-Datei), nicht
jeder Sitzung. Die zwei Browser-Server laufen lokal, kopflos, mit frischem Profil je Start und dem
Chrome der Werkzeugkiste.

**Playwright MCP** (`@playwright/mcp` 0.0.86, Apache-2.0, Microsoft). Start über
`sc-mcp-playwright` (= `playwright-mcp --headless --isolated --executable-path <chrome>
--viewport-size 1440x900 --output-dir ~/.local/share/slouchcat-design/out/playwright-mcp`).
25 Werkzeuge: öffnen, klicken, tippen, Formulare, Dialoge, Tasten, Grösse ändern, Screenshot,
Konsole, Netzwerk, `browser_evaluate`. Kaputte Bilder: über `browser_evaluate` oder die
Netzwerkliste. Weitere Argumente gehen durch, z. B. `--device "Pixel 9"`. Netz: nur was die
geöffnete Seite lädt, keine Telemetrie gefunden.

**Chrome DevTools MCP** (`chrome-devtools-mcp` 1.14.0, Apache-2.0, Google). Start über
`sc-mcp-chrome-devtools` mit `--no-usage-statistics --no-performance-crux` und
`CHROME_DEVTOOLS_MCP_NO_UPDATE_CHECKS=1`: keine Nutzungsdaten, keine Seitenadressen an Google, keine
Update-Abfrage. 33 Werkzeuge, dazu, was Playwright nicht kann: Leistungs-Trace mit Auswertung (LCP,
CLS), Lighthouse für Barrierefreiheit, SEO und Best Practices, CPU- und Netzdrosselung, CSS-Werte,
Heap. Dateizugriff nur im Temp-Ordner. Aufteilung: Playwright bedient, DevTools misst; beide mit
eigenem Browser, nie dasselbe Profil.

**Context7** (Dienst `https://mcp.context7.com/mcp`, MIT, Upstash), nur beim `ui-engineer`. Ohne
Schlüssel und ohne Konto, also ohne Kosten. Pro Aufruf gehen Bibliotheksname und Frage an Upstash,
keine Dateien; die Agent-Datei verbietet Projektcode, Namen, Adressen und Schlüssel in Fragen.
Grenzen ohne Schlüssel sind nicht veröffentlicht; mit kostenlosem Konto 1000 Aufrufe im Monat.
Entfernen: den Eintrag `mcpServers` in `.claude/agents/ui-engineer.md` löschen.

**GitHub:** `gh` 2.45.0, angemeldet als `MarcoBrechbuhl` (Schlüsselbund) mit `repo`, `workflow`,
`read:org`, `gist`; Rolle ADMIN auf `SlouchCat/slouchcat`. Lesen, Zweige, Commits, Diffs, PRs,
Reviews und Issues gehen darüber. Bekannte Falle: `gh pr edit` ändert den PR-Text nicht, dafür
`gh api -X PATCH repos/SlouchCat/slouchcat/pulls/<nr> -F body=@datei`. Kein zweiter Zugang.

**Schon vorhanden:** `t` aus oh-my-claudecode (50 Werkzeuge: LSP, ast-grep, Notizblock, Wiki).

## Werkzeugkiste

Alles unter `~/.local/share/slouchcat-design` (rund 870 MB), Links in `~/.local/bin`, ohne sudo.
npm-Pakete exakt nach `package-lock.json` mit `npm ci --ignore-scripts`; keines braucht ein
Installationsskript. AppImages nur mit passender SHA-256 der Projektseite.

| Werkzeug | Version | Lizenz | Befehl | Wofür |
|---|---|---|---|---|
| Chrome for Testing | 158.0.8127.88 | BSD/Chromium | `~/.local/share/slouchcat-design/bin/chrome` | ein Browser für alle Prüfungen |
| Inkscape | 1.4.4 (AppImage) | GPL | `inkscape` | Text in Pfade, Vektorarbeit, PDF |
| ImageMagick | 7.1.2-36 (AppImage) | ImageMagick | `magick` | Raster, Einfarbig, ICO, Splash |
| svgo | 4.1.0 | MIT | `svgo` | SVG optimieren |
| resvg-js | 2.6.2 | MPL-2.0 | `sc-svg2png` | SVG genau nach PNG, transparent |
| sharp (libvips) | 0.35.5 | Apache-2.0, libvips LGPL-3.0 | über `pwa-assets-generator` | Rasterbilder in Node |
| @vite-pwa/assets-generator | 2.0.0 | MIT | `pwa-assets-generator --preset minimal-2023 logo.svg` | Favicon, PWA, maskable, Apple-Icon |
| png-to-ico | 3.0.2 | MIT | `png-to-ico` | ICO |
| svg2vectordrawable | 2.9.1 | MIT | `s2v` | SVG nach Android-Vektor (adaptive Icons) |
| fonttools mit Brotli | 4.69.1 | MIT | `pyftsubset`, `fonttools`, `ttx` | Schriften schneiden, WOFF2 |
| Lighthouse | 13.6.0 | Apache-2.0 | `lighthouse` (ohne Fehlerberichte) | Leistung, Barrierefreiheit |
| axe-core, @axe-core/playwright | 4.14.1 | MPL-2.0 | `sc-axe` | WCAG-Prüfung gerenderter Seiten |
| pixelmatch, pngjs | 8.0.0, 7.0.0 | ISC, MIT | `sc-pixeldiff` | Bildvergleich |
| playwright-core | 1.65.1 | Apache-2.0 | `sc-shot` | Screenshots in vier Grössen mit Befund |
| Playwright MCP, Chrome DevTools MCP | 0.0.86, 1.14.0 | Apache-2.0 | `sc-mcp-playwright`, `sc-mcp-chrome-devtools` | siehe oben |

Die eigenen Werkzeuge (`scripts/design-tools/tools/`):

- `sc-shot <url> --out <ordner> [--viewports 390x844,768x1024,1440x900,1920x1080]` macht Screenshots
  und meldet Konsolenfehler, fehlgeschlagene Anfragen, kaputte Bilder und waagrechten Überlauf
  (Exit 1 bei Befund). Unter 600 px Breite als Handy mit Touch.
- `sc-axe <url> [--fail-on serious]` prüft WCAG 2.0 bis 2.2 A/AA mit axe-core.
- `sc-pixeldiff a.png b.png [--out diff.png] [--max-ratio 0.001]` vergleicht zwei Screenshots.
- `sc-svg2png in.svg out.png --width 512` rendert mit resvg.
- `sc-mcp-check <url>` bedient beide Browser-MCP-Server über das MCP-Protokoll wie Claude Code.

Assets und ihre Wege: SVG-Master (nur Pfade) → `svgo` → `sc-svg2png`/`magick` für PNG jeder Grösse,
einfarbig über `magick … -alpha extract`; Favicons, PWA- und Apple-Icons über
`pwa-assets-generator`; Android-Vordergrund, -Hintergrund und Monochrom als Vektor über `s2v`,
als Raster 432 px (xxxhdpi); TV-Banner 320×180; Splash über `magick`; WOFF2 über `pyftsubset`.
Bestehende Wege im Repo bleiben: `web/scripts/icons.mjs` (`task web:icons`),
`scripts/store-graphics.sh`, `android/app-tv/banner/build_banner.py`; die Kopien des Zeichens prüft
`web/src/ui/logoMark.test.ts`.

**Aktualisieren:** Versionen in `scripts/design-tools/package.json` setzen, `npm install
--package-lock-only --ignore-scripts` in `scripts/design-tools/`, Chrome-, Inkscape- und
ImageMagick-Version samt SHA-256 oben in `scripts/install-design-tools.sh`; Installer und Prüfung
neu laufen lassen. **Entfernen:** `scripts/install-design-tools.sh --uninstall` (Bilder bleiben),
`--uninstall --purge` (alles).

## Android und Android TV

Vorhanden aus `scripts/install-android-tools.sh`: JDK 21, SDK, Build-Tools 36/37, `adb`, Emulator,
System-Images API 28, 34 (Google TV), 36 (Handy, Tablet), AVDs `sc_phone`, `sc_tablet`, `sc_tv`,
`sc_phone28`, KVM nutzbar. Fotos ganzer Seiten: `task android:e2e SHOTS=<ordner>` (Tablet mit
`AVD=sc_tablet`) und `task android:e2e:tv SHOTS=<ordner>`. Fokus und D-Pad prüfen die
Robolectric-Tests der TV-App (`Dpad.kt`) und `TvFlowTest`. Neu installiert wurde hier nichts.

Für Stichproben von Hand: `adb -s emulator-5584 shell input keyevent KEYCODE_DPAD_LEFT` bewegt den
Fokus, `adb -s emulator-5584 shell uiautomator dump` liefert den Baum mit `focused="true"`,
`adb -s emulator-5584 exec-out screencap -p > bild.png` das Bild. Immer mit `-s`, denn per WLAN
hängt oft Marcos Pixel an `adb`. Vorher die Sperre der Maschine nehmen (`flock` auf
`${XDG_RUNTIME_DIR}/slouchcat-emulator.lock`, wie `scripts/android-helpers.sh`).

Grenzen: Ein Browser in Handygrösse ist nicht die App. Echte Fernbedienung, echte TV-Panels
(Overscan, Farbraum), TalkBack und echte Touch-Gesten prüft kein Emulator vollständig.
Screenshot-Vergleiche gegen Baselines (Roborazzi oder Compose Preview Screenshot Testing) gibt es
für die Apps noch nicht; Vorschlag für den Relaunch, wenn das neue Aussehen steht.

## Echte Medienbilder

Mockups zeigen echte Cover, Hintergründe, Porträts und Logos, wie bisher (`BAUKASTEN.md` der
Mockups: Bilder nur über die Helfer `poster(i)`, `backdrop(i)`, `pic()`).

- **Ablage:** Bildsätze unter `~/.local/share/slouchcat-design/media/<satz>/`, ausserhalb jedes
  Arbeitsbaums, damit sie das Löschen eines Worktrees überleben. Vorhanden: `ct190-2026-11-19`
  (255 WebP, 9,4 MB: 110 Poster, 71 Hintergründe, 33 Porträts, 24 Folgenbilder, 9 Titel-Logos,
  6 Sammlungen, 2 Senderlogos) aus dem Bildcache der Testinstanz CT 190.
- **Werkzeug:** `scripts/design-media.sh status`, `seed <ordner> <satz>` (Satz aus vorhandenen
  Bildern), `link <mockup-ordner> [<satz>]` (verlinkt als `img`).
- **Nie in Git:** Neue Regel `docs/planning/mockups/*/img` in `.gitignore`. Die bisherigen Regeln
  `img/` in den Mockup-Ordnern fassen nur Ordner, keinen Link mit diesem Namen, und
  `26-android-tv/` hatte gar keine.
- **Neue Bilder:** aus dem Cache des eigenen Servers (`/opt/slouchcat/deploy/cache/images/<id>/…` auf
  CT 190, nur lesend, gröbste Grössen `poster-w400.webp`, `backdrop-w780.webp`); Weg im Memory
  «Slouchcat Mockups Android». Die Quellen dahinter sind TMDB, Fanart.tv, TVDB, Buchanbieter,
  Podcast-Feeds und Radio-Browser.
- **Rechte:** Die Bilder gehören den Rechteinhabern. Ob Bilder von TMDB oder Fanart.tv öffentlich
  gezeigt werden dürfen, ist nicht belegt (`docs/store/grafiken.md`). Darum nur lokal, nie
  committen, nie in ein öffentliches Artefakt oder einen PR-Text. Für alles, was öffentlich wird
  (Store-Fotos, Doku), gelten die vier offenen Blender-Filme aus `deploy/demo/` (CC BY, Nennung in
  `deploy/demo/README.md`).
- **Keine Personendaten:** keine Profilbilder, Konto- oder Profilnamen der Testinstanz übernehmen;
  im Mockup erfundene Namen und Initialen.
- **Grössen:** Die Cache-Bilder sind schon WebP in Zielbreite; für andere Grössen
  `magick in.webp -resize 400x out.webp`.

## Design-CT auf pve2 (Plan, nicht angelegt)

Jetzt nicht nötig: Mockups laufen lokal (`python3 -m http.server` oder `.claude/launch.json`).
Sobald eine Galerie für mehrere Geräte im Heimnetz gebraucht wird:

- **pve2** (Proxmox 9.3.1, 12 Kerne, 62 GB RAM, davon 47 GB frei, `local-zfs` 620 GB frei,
  Sicherung auf PBS `Backup`). Freie ID **191** (Cluster geprüft), Name `slouchcat-design`.
- Vorlage `debian-13-standard_13.6-1`, unprivilegiert, ohne `nesting`, 2 Kerne, 2 GB RAM,
  16 GB auf `local-zfs`, `onboot=0`.
- Netz `vmbr0` mit DHCP; Proxmox-Firewall: eingehend nur `192.168.1.0/24` auf 22 und 8080; kein
  Eintrag in Nginx Proxy Manager, kein Cloudflare, keine Portfreigabe.
- Inhalt: ein statischer Webserver (Caddy aus Debian) für `docs/planning/mockups/` und die Bilder,
  per `rsync` vom Entwicklungsrechner. Keine Medien, keine Konten, kein Zugriff auf CT 190.
  Prüfwerkzeuge bleiben auf dem Entwicklungsrechner.
- Rückweg: `vzdump` auf PBS vor Änderungen; Entfernen mit `pct destroy 191`.

## Funktionsprüfung

`scripts/design-tools-check.sh` baut ein Probezeichen und eine Probeseite mit eingebauten Fehlern
(zu schwacher Kontrast, Bild ohne Alternativtext, fehlendes Bild, Dialog, Textfeld), stellt sie auf
`127.0.0.1` bereit und prüft jedes Werkzeug daran. Lauf am 28. November 2026: **45 von 45 grün.**

| Bereich | Belegt |
|---|---|
| Grafik | SVG optimiert und gültig; Text in Pfade (Inkscape); PNG 512 aus Inkscape und resvg mit Alpha; einfarbige Fassung; ICO mit 16/32/48; ICO aus PNG; PWA-Icons 192/512, maskable 512, Apple 180, Favicon; Android-Vektor; Adaptive-Ebene 432; TV-Banner 320×180; Splash 1170×2532; WOFF2-Teilmenge |
| Browser | Screenshots 390×844 und 1440×900; Fehlerseite meldet kaputtes Bild und Anfrage; axe: saubere Seite 0 Verstösse, Fehlerseite `color-contrast` und `image-alt`; Pixelvergleich gleich und ungleich; Lighthouse Leistung 1,0 und Barrierefreiheit 1,0 |
| Playwright MCP | über MCP: 25 Werkzeuge, Seite öffnen, Knopf klicken, Dialog sichtbar, Feld füllen, 390×844, Screenshot, Konsolenfehler, fehlgeschlagene Anfrage, kaputtes Bild |
| Chrome DevTools MCP | über MCP: 33 Werkzeuge, Seite öffnen, Snapshot, Konsole und Netz, 390×844, Screenshot, Leistungs-Trace mit LCP, Lighthouse |
| Context7 | über MCP: `resolve-library-id` für react-aria-components, `query-docs` liefert 5909 Zeichen Doku |
| GitHub | PRs, Issues, Zweige, Dateien eines PR, `git ls-remote` |
| Claude Code | frische Sitzung im Repo: 7 Projekt-Agents und 25 von 25 Skills geladen, keine Plugin-Fehler; ausserhalb des Repos 0 von 25 |
| Android TV | `sc_tv` startet in 12 s; Slouchcat-TV-App gestartet; Screenshot 1920×1080; `uiautomator dump` zeigt den Fokus; DPAD_LEFT wechselt in die Seitenleiste, DPAD_DOWN zum nächsten Eintrag |
| Skills | `ui-ux-pro-max` Suche läuft offline; Sperre ausserhalb des Projekts greift im Skill-Werkzeug |

## Offen

- **Neue Sitzung nach dem Merge.** Die Desktop-App liest Agents, Skills und Einstellungen auch für
  Worktree-Sitzungen aus dem Haupt-Checkout. Bis der Zweig in `main` ist und der Haupt-Checkout
  nachgezogen ist (`git -C <haupt-checkout> merge --ff-only origin/main`), sind die Design-Agents
  nicht da und die Skills dort aus. Danach in einer neuen Sitzung einmal prüfen: «Starte design-qa
  gegen eine Probeseite und lass ihn je ein Playwright- und ein DevTools-Werkzeug aufrufen.» Erst
  das belegt die MCP-Server und vorgeladenen Skills innerhalb der Agents.
- **Terminal-CLI nicht angemeldet.** `claude -p` im Terminal meldet «Not logged in»; die Desktop-App
  hat eine eigene Anmeldung. Für Kopflos-Läufe einmal `claude` im Terminal starten und `/login`.

## Sicherheit

- Aus fremden Repositories wurde nichts ausgeführt; alle ausgewählten Skills sind gelesen und nach
  Mustern durchsucht (Befehle aus dem Netz, versteckte Anweisungen, Geheimnisse). Fund: keine
  verdeckten Anweisungen; `canvas-design` enthält einen erfundenen Nutzersatz zur Qualität (harmlos).
- Keine Hooks von aussen. Keine neuen Rechte in `permissions`. Keine Geheimnisse im Repo; Context7
  ohne Schlüssel.
- Ausgehender Verkehr: Chrome DevTools MCP ohne Statistik, CrUX und Update-Abfrage; Lighthouse ohne
  Fehlerberichte; Context7 nur Bibliotheksfragen; Browser nur zu den geöffneten Seiten.
- Restrisiken: Skills sind Text, den ein Agent als Anweisung liest; ein Update bringt neuen Text,
  darum nur gepinnte Commits nach dem Lesen. `@playwright/mcp` hängt an einer Alpha von Playwright.
  `evaluate_script` und `browser_evaluate` führen beliebiges JavaScript in der Seite aus; die Agents
  öffnen nur lokale oder Test-Seiten.

## Rückweg

1. `scripts/install-design-skills.sh --uninstall` und `scripts/install-design-tools.sh --uninstall`.
2. Sicherungen zurück: `~/.claude/backups/settings.json.vor-design-werkzeuge-2026-11-28` nach
   `~/.claude/settings.json` (ebenso `claude.json`, `CLAUDE.md`, `installed_plugins.json`,
   `known_marketplaces.json`, `.omc-config.json`).
3. Im Repo den PR zurücknehmen (`git revert` des Merge-Commits); das entfernt Agents, Einstellungen,
   Skripte und diese Seite.
