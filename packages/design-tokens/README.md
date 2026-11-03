# @slouchcat/design-tokens

Einzige Quelle für Farben, Typografie, Abstände, Radien, Bewegung und Ebenen der Web-App (Spec C-2).

- Quelle: `tokens/*.json` im DTCG-Format (`$type`, `$value`, `$description`).
- Build: `node build.mjs` (Style Dictionary 5), aufgerufen von `task gen` und `task web:build`.
- Ausgabe in `dist/` (nicht eingecheckt, nie von Hand ändern):
  - `tokens.css` – CSS Custom Properties auf `:root` mit Präfix `--sc-`, z. B. `--sc-color-bg-base`.
  - `index.js` und `index.d.ts` – `tokens` (Werte als verschachteltes Objekt, Pfade in camelCase,
    z. B. `tokens.color.text.onAccent`), `accentNames` (Tupel der 8 Akzentfarben) und Typ `AccentName`.

## Inhalt

| Gruppe                                                   | Zweck                                                                                       |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `color.bg`, `color.surface`                              | dunkle, kinoartige Flächen (base, elevated, overlay; Stufen 1–3)                            |
| `color.text`                                             | primary, secondary, muted, on-accent                                                        |
| `color.border.subtle`                                    | Haarlinie, sparsam eingesetzt                                                               |
| `color.state`                                            | focus-ring, danger, success, warning                                                        |
| `color.accent.<name>.base/.on`                           | 8 Akzentfarben je Profil (ember, coral, amber, lime, teal, sky, violet, rose) mit Textfarbe |
| `font.*`                                                 | Schriftfamilie, Grössen, Gewichte, Zeilenhöhen, Laufweite                                   |
| `space.*`                                                | Abstände auf 4-px-Basis                                                                     |
| `radius.*`, `focus.ring.*`                               | Eckenradien, Breite und Abstand des Fokusrings                                              |
| `motion.*`                                               | Dauer (fast 120 ms, focus 180 ms ≤ 200 ms, enter, exit) und Kurven                          |
| `z.*`                                                    | Ebenen (base bis toast)                                                                     |
| `control.*`, `panel.*`, `table.*`, `page.*`, `section.*` | Komponententokens (Konzept 12, ADR 0020), Standard = heutiger Wert der allgemeinen Skala    |
| `density.compact.*`                                      | kompakte Werte der Verwaltung (Konzept 12, 5.1); eingeschaltet unter `/admin` (K12-8)       |

## Komponententokens und Dichte

Die Bausteine in `web/src/ui` lesen für Steuerelemente, Panels, Tabellen und Seitentitel eigene Tokens
statt der allgemeinen Skala; ihr Standard ist genau der heutige Wert, darum ändert sich ohne Dichte
nichts. `size.control.height` (44 px) bleibt als Mindestmass für Berührung und ist der Standard von
`control.height`.

| Token (CSS)                   | Standard                                 | `density.compact`      | gelesen von                                            |
| ----------------------------- | ---------------------------------------- | ---------------------- | ------------------------------------------------------ |
| `--sc-control-height`         | `size.control.height` (44 px)            | 32 px                  | Steuerelemente (Liste unten)                           |
| `--sc-control-font-size`      | `font.size.md` (16 px)                   | 14 px                  | `Button`, `TextField` (`Field`)                        |
| `--sc-panel-padding`          | `space.6` (24 px)                        | 16 px                  | `Panel`, `Dialog`, `BandPanel`                         |
| `--sc-panel-gap`              | `space.5` (20 px)                        | 12 px                  | `Panel`, `BandPanel`, `MasterDetail`                   |
| `--sc-panel-radius`           | `radius.xl` (20 px)                      | 12 px (`radius.lg`)    | `Panel`, `Dialog`, `BandPanel`, `MasterDetail` (Liste) |
| `--sc-table-cell-padding-y`   | `space.3` (12 px)                        | 9 px (Zeile 40 px)     | `DataTable` (Kopf und Zellen)                          |
| `--sc-page-title-size`        | `font.size.display` (60 px)              | 24 px                  | `Text` Variante `pageTitle` ab 768 px (`AdminPage`)    |
| `--sc-page-title-weight`      | `font.weight.bold` (700)                 | 600                    | `Text` Variante `pageTitle`                            |
| `--sc-page-title-tracking`    | `font.letter-spacing.display` (−0.02 em) | normal (0)             | `Text` Variante `pageTitle`                            |
| `--sc-page-title-line-height` | `font.line-height.normal` (1.5)          | 0                      | `AdminPage` (eigene Zeile des h1)                      |
| `--sc-section-gap`            | `space.10` (40 px)                       | 24 px                  | `AdminPage` (Abstand der Abschnitte)                   |
| `--sc-section-title-size`     | `font.size.xl` (24 px)                   | 16 px (`font.size.md`) | `SectionHeader` (h2), `BandPanel` (Titel im Band)      |
| `--sc-dialog-gap`             | `space.4` (16 px)                        | 12 px                  | `Dialog` (Titel, Text, Aktionen)                       |

`--sc-control-height` lesen `Button`, `TextField` (`Field`), `SelectField`, `Switch`, `CheckboxGroup`,
die Segmente von `RadioGroup` (`Field`), die Reiter von `Tabs` (Innenabstand), `ProviderList` (Zeilen),
die Einträge von `MenuButton`, `ActionMenu` und `MasterDetail`, `ChoiceChips`, `FilterChips`,
`RankSelect`, `SettingRow`, die Klappknöpfe von `BandPanel` und `DisclosureGroup` sowie Kopfzeile und
Hilfe-Knopf von `AdminPage`. `--sc-control-font-size` lesen zusätzlich die Segmente von `RadioGroup`.
`SideNav` und `AdminSideNav` lesen weiter `--sc-size-control-height` und behalten 44 px (Konzept 12,
5.2: Leisten behalten ihre Masse).

Die Variante `display` von `Text` liest wieder `font.size.display`: Seiten ohne `AdminPage` behalten
ihren Titel, auch wenn die Dichte eingeschaltet ist.

Die kompakten Werte stehen als `--sc-density-compact-*` bereit; die Zuordnung auf die
Komponententokens steht in `web/src/ui/density.css`. Sie gilt nur unter `:root[data-shell="admin"]`
(das Gerüst der Verwaltung, Konzept 11) und dort nur im Inhaltsbereich `#content` und auf Portalen mit
`data-density="compact"`, die `AdminDensityScope` setzt (Dialoge aus `Dialog.tsx`, `MessageDialog`,
die Popover von `MenuButton` und `ActionMenu`), nie auf `:root` selbst: App-Kopf und
Verwaltungsleiste behalten ihre Masse, `/settings/*` (ohne Verwaltungsgerüst) samt Dialogen die
heutige Dichte. Dazu `@media (min-width: 48rem) and (pointer: fine)`; Touch und Handy behalten 44 px.
Zusätzlich setzt sie den Seitenrand `--page-gutter` (aus `web/src/ui/global.css`, 40 px ab 768 px,
64 px ab 1440 px) auf `density.compact.page.gutter` (32 px). Die App lädt die Datei in
`web/src/main.tsx` (seit K12-8), Storybook in `.storybook/preview.tsx`
(Global `density`).

## Akzentfarben zur Laufzeit

Zwei Werte kommen nicht aus dem Build, sondern zur Laufzeit als CSS Custom Properties (Beschreibung in
`web/src/ui/accent.ts`):

- `--sc-accent` / `--sc-on-accent`: Akzent des aktiven Profils. `web/src/app/session.ts` setzt sie auf
  `var(--sc-color-accent-<name>-base)` bzw. `-on` (nur Namen aus `accentNames`); ohne Sitzung gilt
  `ember` aus `web/src/ui/global.css`.
- `--sc-item-accent`: Akzentfarbe eines Titels (`ItemRef.accent`, `#rrggbb` aus der Bildpipeline) für
  das Glühen der Posterkarte und den Tint des Detail-Backdrops; jeder Gebrauch hat einen Token-Rückfall.

## Schrift

`font.family.sans` ist vorläufig ein System-Schriftstapel. Die selbst gehostete variable Schrift wird im
Design-Brief von M1b gewählt und ersetzt dann diesen Wert.

## Kontrast

`pnpm test` baut die Tokens und prüft nach WCAG 2.x (relative Luminanz), dass jede Akzentfarbe auf
`color.bg.base`, die `on`-Farbe jeder Akzentfarbe auf ihrer Akzentfarbe sowie `text.primary`,
`text.secondary` und `text.muted` auf `color.bg.base` mindestens 4.5:1 erreichen. Die Tabelle der
Verhältnisse erscheint in der Testausgabe.
