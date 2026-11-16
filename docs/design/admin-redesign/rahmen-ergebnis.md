# Ergebnis Schritt 1: Rahmen (für die Seitenschritte)

Commits `dcba5318` bis `d01338a6` auf `claude/admin-redesign`. Belege: `task check:fast` grün
(2122 Vitest-Tests), `e2e/admin-shell.spec.ts` 8 bestanden, alle angepassten Verwaltungs-Specs
grün, `task storybook:test` 547 bestanden.

## Was steht

- **Navigationsmodell** `web/src/features/admin/sections.ts`: `adminAreas`
  (`media`, `people`, `home`, `automation`, `system`), jede Seite mit `area`; aktuelle Seite nach
  dem längsten passenden Pfad. Neuer Eintrag `audiobookSources`. Der Eintrag `server` fehlt noch
  (Schritt 2).
- **Seitenleiste** (ab 768 px): «Zurück zur App», Überschrift «Verwaltung», «Übersicht» und die
  fünf Bereiche mit Pille; Fuss mit Profil und Rolle. `AdminShellNav.tsx` baut die Einträge.
- **Handy**: Knopf «Verwaltung › Bereich / Seite» über jeder Seite (`PageNav`), Blatt mit allen
  Seiten nach Bereichen.
- **Kopfzeile**: Zusatz «Control Center», Brotkrumen, Knopf «Bereich suchen» (`Strg K`).
- **Suchpalette**: `CommandPalette` aus `@/ui/aria`, gefüllt aus `sections.ts`.
- **Bereichsreiter**: `AreaTabs` aus `@/ui/aria` über jeder Seite, die `AdminPage` nutzt; ein
  Tab-Stopp, ←/→/Pos1/Ende. Zahlen und Pillen der Seiten stehen hier.
- **Seitenkopf**: `AdminPage` liest `useAdminFrame()` (`web/src/ui/adminFrame.ts`): Zeile über dem
  Titel, Titel 30 px, Einleitung (`help`) offen darunter, Aktionen rechts. Ohne Rahmen
  (`/settings/*`) wie bisher mit Hilfe-Knopf.
- **Aussehen** nur unter `data-shell="admin"`: `web/src/ui/admin-look.css` stellt
  Komponenten-Tokens um; die Bauteile lesen sie mit dem bisherigen Wert als Rückfall.

## Für deine Seiten

- **`AdminPage`** mit `title`, `help` (Einleitung, jetzt sichtbar), `actions` (Hauptaktionen rechts
  im Kopf), `status`, `liveSeconds`, `titleRef` wie bisher. Die Hauptaktion des Mockups gehört in
  `actions`.
- **Token-Stellen** (in `admin-look.css` gesetzt, in den Bauteilen mit Rückfall gelesen):
  `--sc-panel-border`, `--sc-table-background`, `--sc-stat-background`, `--sc-stat-border`,
  `--sc-stat-radius`, `--sc-chip-{warning,danger,success}-{background,color}`,
  `--sc-notice-border`, `--sc-notice-{warning,danger}-{background,border}`, `--sc-button-radius`,
  `--sc-button-primary-{background,color}`, `--sc-field-focus-border`, `--sc-select-radius`.
  Design-Tokens: `admin.title-size`, `admin.title-size-narrow`, `admin.title-tracking`,
  `admin.eyebrow-size`, `admin.intro-size`.
- **Bauteile mit dem neuen Aussehen**: `Panel` (Karte), `DataTable` (Tabelle), `StatTile` und
  `StatusStrip` (Kennzahlen), `Chip` (Abzeichen), `Notice` (Hinweisband), `Button`, `TextField`,
  `SelectField`. Ohne eigene Token-Stelle: `Tabs` (seiteneigene Reiter), `DashCard`.
- **e2e-Helfer** `web/e2e/admin.ts`: `adminNav(page)`, `areaTabs(page)`, `areaTab(page, name)`,
  `adminEntry(page, name)`. Innerhalb eines Bereichs wechselt man über die Bereichsreiter, zwischen
  Bereichen über den Bereichseintrag der Seitenleiste oder die Palette.
- **Tests mit Navigation**: Seitenverweise und `aria-current` stehen in den Bereichsreitern
  (`navigation` «Seiten im Bereich …»), nicht mehr in der Seitenleiste.

## Bekannt offen (gehört in die Seitenschritte)

- `/admin/metadata/audiobooks` läuft auf dem Handy über (langer Ordnerpfad, 542 px breit, bestand
  schon vorher): in Schritt 3a beheben und die Seite in `e2e/admin-phone.spec.ts` aufnehmen.
- Die Übersicht hat noch ihren alten Kopf (Schritt 2).
- Das Abzeichen «Update» steht noch nur an «Übersicht»; mit der Seite «Server» kommt es auch an
  «System» und den Reiter «Server» (Schritt 2).
- In 20 Sprachen fehlen die neuen Texte (Rückfall Englisch); der Koordinator zieht sie am Schluss
  nach. Neue Texte nur in `de`, `en` und, wo nötig, `de-DE` anlegen.
- Die Höhe des Seitenkopfs: Bereichsreiter, Zeile über dem Titel, Titel und Einleitung brauchen
  Platz. Halte den Kopf deiner Seiten knapp (kurze Einleitung, Aktionen in der Titelzeile), damit
  die erste Arbeitszeile nicht tiefer rutscht als nötig.
