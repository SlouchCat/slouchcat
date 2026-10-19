# @slouchcat/web

React-PWA von Slouchcat: Vite, React 19, TypeScript (strict), `@tanstack/react-router` (Routen im Code),
`@tanstack/react-query`, `i18next`/`react-i18next`, `react-aria-components` (Formulare, Dialoge, Player-Knöpfe),
`@noriginmedia/norigin-spatial-navigation` (Pfeiltasten wie mit einer Fernbedienung), `vite-plugin-pwa`,
Playwright für die Tastatursuite.

## Befehle

| Befehl           | Wirkung                                                                                               |
| ---------------- | ----------------------------------------------------------------------------------------------------- |
| `task web:dev`   | Vite-Entwicklungsserver; `/api` wird an den Go-Server auf `http://localhost:8096` weitergeleitet      |
| `task web:gen`   | Design-Tokens bauen, API-Client aus `api/openapi.yaml` erzeugen                                       |
| `task web:lint`  | ESLint, stylelint, `tsc --noEmit`, `prettier --check`                                                 |
| `task web:test`  | Vitest im ganzen Workspace                                                                            |
| `task web:build` | Build nach `web/dist`, danach Kopie nach `server/internal/webassets/dist` (eingebettet ins Go-Binary) |
| `task web:ci`    | Installation mit `--frozen-lockfile`, gen, lint, test, build                                          |
| `task web:e2e`   | Playwright-Tastatursuite (P-15) gegen einen frischen Server auf `:8097`; setzt `task build` voraus    |
| `task web:icons` | PNG-Icons in `public/icons/` aus `public/icon.svg` neu erzeugen (ffmpeg, Farbe aus den Tokens)        |

## Aufbau

```
src/main.tsx          Einstieg: Tokens-CSS, globale Basisstile, i18n, Raumnavigation, Provider
src/app/              Router mit Wächter, Layouts (Kopfzeile), Sitzung, QueryClient, Pfade
src/api/              API-Client (@slouchcat/api-client), apiUrl(), ApiError, Fehlercodes → Texte
src/lib/              Formatierung (Laufzeit, Grösse, Zeit), Zurück-Tasten, Dokumenttitel
src/i18n/             i18next-Setup, Spracherkennung, Typen der Schlüssel
src/locales/          de.json (Standard), en.json (Rückfall)
src/ui/               gemeinsame Bausteine für Zuschauer und Admin (C-2), Export nur über index.ts
src/features/setup/   Einrichtungsassistent /setup (A-1)
src/features/auth/    Anmeldung /login (K-1)
src/features/catalog/ Startseite / (E-1) und Detailseite /items/$itemId
src/features/player/  Player /play/$itemId (W-1 Direct Play, W-5 Fortschritt)
src/features/admin/   Bibliotheken /admin/libraries (A-1, B-1)
src/features/system/  Serverstatus (Anmelde- und Einrichtungsseite)
e2e/                  Playwright: eine Tastatursuite je Seite, globale Einrichtung, Serverstart
scripts/icons.mjs     PWA-Icons aus icon.svg
```

## Entscheide

- **Wächter im Router.** Die Root-Route fragt zuerst `GET /auth/session` ab. Eine Sitzung gibt es erst nach
  dem Assistenten; nur ohne Sitzung (401) fragt sie `GET /setup` und schickt alles nach `/setup`, solange der
  Assistent nicht gelaufen ist. Auf der Startseite fordert sie `GET /home` gleichzeitig mit der Sitzung an
  (Prefetch, nicht abgewartet), statt erst nach dem Wächter (P-10). Die Layout-Route `_authed` führt ohne
  Sitzung nach `/login`. Danach gilt die Sprache des Profils (`i18n.changeLanguage`). Ein 401 auf irgendeiner
  späteren Abfrage löscht die Sitzung im Cache und führt ebenfalls zur Anmeldung.
- **Keine Sichtbarkeitsregel im Client** (Regel 12). Die Seiten zeigen, was der Server liefert; leere
  Reihen werden nur defensiv ausgeblendet (E-9). Die Prüfung `canPlayType` im Player ist eine
  Gerätefähigkeit, keine Freigaberegel: Kann der Browser das Format nicht, erscheint der Hinweis auf M1b.
- **URLs aus der API** (`images.*.url`, `streamUrl`) sind relativ zur API-Basis; `apiUrl()` setzt
  `/api/v1` davor, sofern die URL ihn nicht schon trägt. Sonst werden sie nicht verändert.
- **Raumnavigation nur auf den Stöberseiten.** norigin bewegt den echten DOM-Fokus
  (`shouldFocusDOMNode`), also gelten `:focus-visible`, Screenreader und Enter wie gewohnt. Native
  Ereignisse bleiben unangetastet; `SpatialPage` (Start, Detail) verhindert nur das Scrollen durch
  Pfeiltasten. Auf Formularseiten und im Player ist norigin pausiert; dort gelten Tab und die eigenen
  Tasten. Pfeil links auf „Abspielen“ bzw. rechts auf „Details“ blättert das Karussell. norigin (mit
  lodash-es) ist eine eigene Datei: `main.tsx` fordert sie beim Start an (`loadSpatialNavigation()`),
  die Routen mit Kopfzeile (`_chrome`) warten darauf, danach nutzen die Bausteine sie synchron (P-10).
- **Player-Tasten:** Leertaste/Enter Wiedergabe/Pause, ←/→ ±10 s, ↑/↓ Lautstärke, F Vollbild,
  Escape/Backspace zurück zur Detailseite. Die Bedienelemente verschwinden nach 3 s ohne Eingabe.
  `data-start-ms` am Player misst die Zeit vom Öffnen der Route bis zum ersten `playing`.
- **Fortschritt:** alle 10 s während der Wiedergabe, bei Pause, am Ende, beim Verstecken der Seite und
  beim Verlassen (die letzten beiden mit `keepalive`). `seq` steigt je Wiedergabe, `playedMs` zählt nur
  tatsächlich abgespielte Zeit seit dem letzten Ereignis.
- **Bundles:** Start und Detail liegen im Hauptbundle (LCP, P-10); Anmeldung, Assistent, Player und
  Admin werden bei Bedarf geladen und mit ihnen react-aria. Die Stöberseiten (Start, Detail, Kopfzeile,
  Fehlerseiten) nutzen schlichte `<button>`/`<a>` aus `@/ui` (`Button`, `ButtonLink`, `Spatial*`,
  `PosterCard`, `TextLink`, `ProgressBar`); `AppLink` übergibt interne Pfade dem Router. Formulare,
  Dialoge und der Player nehmen `AriaButton`, `TextField`, `RadioGroup` und `ConfirmDialog` aus
  `@/ui/aria` (ESLint lässt in Features nur `@/ui` und `@/ui/aria` zu). Hover, Fokus und Druck kommen
  aus `:hover` (nur bei `hover: hover`), `:focus-visible` und `:active`. Der Server liefert App und
  JSON mit gzip aus.
- **Vorladen durch den Server (P-10):** Fragt ein angemeldeter Browser `/` an, trägt die App-Hülle
  `Link: rel=preload` für `/auth/session`, `/home` (`as=fetch; crossorigin`) und das erste
  Karussellbild (gleiche URLs, `imagesrcset` und `imagesizes` wie im Karussell). Lesende API-Aufrufe
  gehen deshalb ohne den CSRF-Kopf `X-Requested-With` (nur schreibende brauchen ihn), sonst nähme der
  Browser die vorgeladenen Antworten nicht.
- **Start-JS nach dem Hero-Bild (P-10):** Im Build ersetzt das Vite-Plugin `deferStartScript`
  (`vite.config.ts`) den Skript-Tag der App-Hülle durch einen Inline-Lader am Ende von `<body>`. Er hängt
  das Modul-Skript ein, sobald das Hero-Bild aus der Hülle gezeichnet ist (zwei Frames nach `load`), und
  sofort ohne Hülle-Bild, bei einem Bildfehler oder verborgenem Dokument; nach 2.5 s in jedem Fall. Bild
  und JS teilen sich so keine langsame Leitung mehr. Der Service Worker wird deshalb nicht mehr inline,
  sondern aus der App registriert (`app/serviceWorker.ts`: nach `load` im Leerlauf), damit die
  Vorab-Cache-Füllung nicht mit dem Start-JS konkurriert. In der Entwicklung (`vite`) bleibt der normale
  Skript-Tag.
- **Hero-Bild in der App-Hülle (P-10):** Am Marker `<!--hero-->` in `index.html` schreibt der Server für
  dieselbe Anfrage `<div id="hero-prepaint">` mit dem ersten Karussellbild (gleiche Attribute) vor
  `#root`; ohne Sitzung oder Bild entfällt der Marker. `#hero-prepaint` in `global.css` hat die Geometrie
  des Karussellbilds und liegt über der Seite, unter der Kopfzeile. `dismissHeroPrepaint()` blendet es aus,
  sobald das Karussell sein Bild gezeichnet hat (oder die Startseite kein Hero-Bild hat), und `main.tsx`
  beim Verlassen von `/`. Ändert sich Höhe oder Bildausschnitt des Karussells, muss `global.css` mit.
- **Bilder auf der Startseite (P-10):** Im Karussell steht zuerst nur das Bild der aktiven Folie im
  Dokument, `loading="eager"` und `fetchpriority="high"`, als `srcset` aus `backdropSmall` (w780) und
  `backdrop` (w1280) mit `sizes="100vw"`. Ist es gezeichnet, kommt das Bild der nächsten Folie dazu (lazy,
  niedrige Priorität); gezeigte Folien bleiben. Die Poster kommen erst ins Dokument, wenn das erste
  Karussellbild geladen und gezeichnet ist (oder fehlschlug): Über HTTP/1.1 teilen sich sonst bis zu sechs
  Verbindungen die Bandbreite, und das Hero-Bild kommt um Sekunden später. Danach laden die ersten sechs
  Poster der ersten Reihe sofort, alle anderen `loading="lazy"` (horizontale Reihen liegen für Chrome
  meist innerhalb der Lazy-Distanz), alle `decoding="async"`. Alle Bildflächen haben feste
  Seitenverhältnisse (kein Layout-Sprung).
- **Neue Tokens:** `color.scrim.*` (Grundfarbe mit abnehmender Deckkraft für Verläufe über Bildern),
  `color.media.backdrop`, `color.state.track`/`buffered`, `motion.duration.crossfade`/`chrome`,
  `focus.scale` (1.06), `size.*` (Posterbreiten, Kopfzeile, Inhalts- und Formularbreite, Hero, Bedienhöhe).
  Die Akzentfarbe des Profils liegt als `--accent`/`--accent-on` in `global.css` (M1a: ember).

## Regeln

- **Farben nur aus Tokens.** CSS Modules mit `var(--sc-...)`; stylelint verbietet Hex-Werte, benannte
  Farben und Farbwerte ohne `var()` (`color-no-hex`, `color-named`, `declaration-strict-value`).
- **Texte nur aus `src/locales/{de,en}.json`.** `eslint-plugin-i18next` meldet Literaltext in JSX und in
  Textattributen (`title`, `aria-label`, `placeholder`, `alt`, `label` …). Beide Dateien müssen dieselben
  Schlüssel haben (Test `src/locales/locales.test.ts`); Deutsch in Schweizer Schreibweise (ss statt ß).
- **Features importieren UI nur aus `@/ui`** (ESLint `no-restricted-imports`).
- **Sprache:** erste unterstützte Sprache aus `navigator.languages`, sonst Deutsch; fehlende Schlüssel
  fallen auf Englisch zurück. Die Umschaltung je Profil (C-3) folgt mit den Profilen.

## PWA

Manifest aus `vite.config.ts`: `name` und `short_name` Slouchcat, `display: standalone`,
`registerType: autoUpdate`. `background_color`, `theme_color` und das `theme-color`-Meta-Tag stammen zur
Build-Zeit aus `tokens.color.bg.base`; im TypeScript-Code steht kein Hex-Wert.

Icons: `public/icons/icon-192.png`, `icon-512.png` (purpose `any`) und `icon-512-maskable.png` (purpose
`maskable`: das SVG auf 80 % verkleinert auf der Token-Hintergrundfarbe) erzeugt `pnpm run icons`
(`scripts/icons.mjs`, ffmpeg mit librsvg). Das Skript bricht ab, wenn `icon.svg` andere Farben als
`color.bg.base` und `color.accent.ember.base` enthält. Die PNGs liegen im Repository.

Service Worker (Workbox, `generateSW`, registriert aus `src/app/serviceWorker.ts` nach `load` im
Leerlauf): vorab gecacht wird nur die App-Hülle (JS, CSS, HTML, Icons, Manifest).
`/api/` und `/spike/` erhalten nie die App-Hülle als Navigations-Rückfall. Laufzeit-Caching:
`/api/v1/items/*/images/*` CacheFirst (höchstens 300 Einträge, 30 Tage); alles andere unter `/api/`
NetworkOnly (JSON, Streams).

## Tastatursuite (Playwright)

`task web:e2e` (oder `pnpm exec playwright test` in `web/`) startet über `e2e/start-server.mjs` das
Binary `server/bin/slouchcat` auf `:8097` mit frischen temporären Konfigurations- und Cache-Ordnern
(`--dev --log-format text`). `e2e/global-setup.ts` führt den Assistenten über die API aus (festes
Testkonto, Bibliothek auf `testdata/media/Movies`, ohne TMDB-Schlüssel), wartet auf den Scan-Zustand
`done`, meldet sich an und speichert die Sitzung. Projekte: Chromium und Firefox; WebKit mit
`SLOUCHCAT_E2E_WEBKIT=1`, sobald die Systembibliotheken installiert sind
(`sudo pnpm exec playwright install-deps`).

Je Seite eine Suite (`e2e/*.spec.ts`): Tab-Reihenfolge, Pfeiltasten, Enter, Zurück per
Escape/Backspace/Browser-Zurück, sichtbarer Fokusring (berechnetes `outline-style` ≠ `none`) und keine
Fokusfalle. Der Player misst `data-start-ms` < 2000 ms mit dem H.264-Testfilm.

Voraussetzungen: `task testdata:media` (Testfilme) und `task build` (Binary mit eingebetteter Web-App).
Gegen einen selbst gestarteten Server: `SLOUCHCAT_E2E_BASE_URL=http://localhost:4173 pnpm exec playwright
test` (z. B. `SLOUCHCAT_API=http://localhost:8097 pnpm exec vite preview`); dann startet Playwright keinen
Server. Spuren und Bildschirmfotos fehlgeschlagener Tests liegen unter `node_modules/.playwright/`.
