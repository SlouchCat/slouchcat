# @slouchcat/web

React-PWA von Slouchcat: Vite, React 19, TypeScript (strict), `@tanstack/react-router` (Routen im Code),
`@tanstack/react-query`, `i18next`/`react-i18next`, `vite-plugin-pwa`.

## Befehle

| Befehl           | Wirkung                                                                                               |
| ---------------- | ----------------------------------------------------------------------------------------------------- |
| `task web:dev`   | Vite-Entwicklungsserver; `/api` wird an den Go-Server auf `http://localhost:8096` weitergeleitet      |
| `task web:gen`   | Design-Tokens bauen, API-Client aus `api/openapi.yaml` erzeugen                                       |
| `task web:lint`  | ESLint, stylelint, `tsc --noEmit`, `prettier --check`                                                 |
| `task web:test`  | Vitest im ganzen Workspace                                                                            |
| `task web:build` | Build nach `web/dist`, danach Kopie nach `server/internal/webassets/dist` (eingebettet ins Go-Binary) |
| `task web:ci`    | Installation mit `--frozen-lockfile`, gen, lint, test, build                                          |

## Aufbau

```
src/main.tsx          Einstieg: Tokens-CSS, globale Basisstile, i18n, Provider
src/app/              Router (Root-Layout und Route "/"), QueryClient, App-Provider
src/api/client.ts     gemeinsamer API-Client aus @slouchcat/api-client
src/i18n/             i18next-Setup, Spracherkennung, Typen der Schlüssel
src/locales/          de.json (Standard), en.json (Rückfall)
src/ui/               gemeinsame Bausteine (Page, Stack, Text, StatusDot) für Zuschauer und Admin
src/features/home/    Startseite mit Serverstatus
```

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
Build-Zeit aus `tokens.color.bg.base` von `@slouchcat/design-tokens`; im TypeScript-Code steht kein
Hex-Wert.

Für M0 gibt es nur das SVG-Icon `public/icon.svg`; seine zwei Farben spiegeln `color.bg.base` und
`color.accent.ember.base` von Hand. PNG-Icons (192, 512, maskable, Apple-Touch) folgen mit C-1 in M1a und
werden dann aus den Tokens erzeugt.
