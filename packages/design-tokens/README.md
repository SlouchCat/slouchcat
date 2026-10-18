# @slouchcat/design-tokens

Einzige Quelle für Farben, Typografie, Abstände, Radien, Bewegung und Ebenen der Web-App (Spec C-2).

- Quelle: `tokens/*.json` im DTCG-Format (`$type`, `$value`, `$description`).
- Build: `node build.mjs` (Style Dictionary 5), aufgerufen von `task gen` und `task web:build`.
- Ausgabe in `dist/` (nicht eingecheckt, nie von Hand ändern):
  - `tokens.css` – CSS Custom Properties auf `:root` mit Präfix `--sc-`, z. B. `--sc-color-bg-base`.
  - `index.js` und `index.d.ts` – `tokens` (Werte als verschachteltes Objekt, Pfade in camelCase,
    z. B. `tokens.color.text.onAccent`), `accentNames` (Tupel der 8 Akzentfarben) und Typ `AccentName`.

## Inhalt

| Gruppe                         | Zweck                                                                                       |
| ------------------------------ | ------------------------------------------------------------------------------------------- |
| `color.bg`, `color.surface`    | dunkle, kinoartige Flächen (base, elevated, overlay; Stufen 1–3)                            |
| `color.text`                   | primary, secondary, muted, on-accent                                                        |
| `color.border.subtle`          | Haarlinie, sparsam eingesetzt                                                               |
| `color.state`                  | focus-ring, danger, success, warning                                                        |
| `color.accent.<name>.base/.on` | 8 Akzentfarben je Profil (ember, coral, amber, lime, teal, sky, violet, rose) mit Textfarbe |
| `font.*`                       | Schriftfamilie, Grössen, Gewichte, Zeilenhöhen, Laufweite                                   |
| `space.*`                      | Abstände auf 4-px-Basis                                                                     |
| `radius.*`, `focus.ring.*`     | Eckenradien, Breite und Abstand des Fokusrings                                              |
| `motion.*`                     | Dauer (fast 120 ms, focus 180 ms ≤ 200 ms, enter, exit) und Kurven                          |
| `z.*`                          | Ebenen (base bis toast)                                                                     |

## Schrift

`font.family.sans` ist vorläufig ein System-Schriftstapel. Die selbst gehostete variable Schrift wird im
Design-Brief von M1b gewählt und ersetzt dann diesen Wert.

## Kontrast

`pnpm test` baut die Tokens und prüft nach WCAG 2.x (relative Luminanz), dass jede Akzentfarbe auf
`color.bg.base`, die `on`-Farbe jeder Akzentfarbe auf ihrer Akzentfarbe sowie `text.primary`,
`text.secondary` und `text.muted` auf `color.bg.base` mindestens 4.5:1 erreichen. Die Tabelle der
Verhältnisse erscheint in der Testausgabe.
