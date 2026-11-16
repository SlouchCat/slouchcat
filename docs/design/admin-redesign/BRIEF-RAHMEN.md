# Auftrag Schritt 1: Rahmen der Verwaltung

Du baust den neuen Rahmen der Slouchcat-Verwaltung: Navigationsmodell, Seitenleiste, Kopfzeile mit
Brotkrumen, Suchpalette, Bereichsreiter, Seitenkopf und das Aussehen unter `data-shell="admin"`.
Die Inhalte der einzelnen Seiten, die Übersicht und die neue Seite «Server» sind **nicht** dein
Auftrag (Schritte 2 und 3).

## Lies zuerst

1. `CLAUDE.md` im Wurzelverzeichnis (harte Regeln, Teststufen). Gilt ohne Ausnahme.
2. `docs/design/admin-redesign/bauplan.md`, Abschnitte 1, 2, 6 und 7. Das ist die Vorgabe.
3. Das Mockup: `docs/design/admin-redesign/handoff/mockup/Slouchcat_Verwaltung_V2.html` (Quelltext
   lesen: CSS am Anfang, Aufbau im Skript) und `handoff/screenshots/Desktop.png`, `Mobil.png`.
   Es ist die **Gestaltungsvorlage**: Masse, Abstände, Gewichte, Zustände daraus ablesen und mit den
   vorhandenen Tokens nachbauen. Kein HTML, kein CSS und keine Farbe daraus kopieren.
4. Den Bestand, den du änderst: `web/src/features/admin/sections.ts`, `AdminShellNav.tsx`,
   `AdminLayout.tsx`, `web/src/app/AppLayout.tsx`, `web/src/ui/AppHeader.tsx`, `AdminSideNav.tsx`,
   `AdminSideNavMenu.tsx`, `AdminPage.tsx`, `PageNav`, `density.css`, `global.css`, `Logo.tsx`,
   jeweils mit CSS-Modul, Test und Story; ADR 0019 und 0020 in `docs/adr/`.

## Arbeitsbaum

`/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat/.claude/worktrees/admin-redesign-s1`, Zweig
`claude/admin-redesign-s1`. Nur dort arbeiten und committen. Nicht pushen. Den Hauptcheckout und die
anderen Worktrees nicht anfassen (dort laufen andere Sitzungen).

## Was zu bauen ist

### a) Navigationsmodell (`sections.ts`)

- `adminAreas = ['media', 'people', 'home', 'automation', 'system']`, jede Seite mit `area` statt
  `group`, Reihenfolge nach Bauplan 1. Neuer Eintrag `audiobookSources`
  (`paths.adminAudiobookSources`). Der Eintrag `server` kommt erst in Schritt 2; `system` hat bei dir
  `stats` und `backup`.
- Aktuelle Seite nach dem längsten passenden Pfad (`/admin/metadata/audiobooks` gehört zu
  `audiobookSources`, nicht zu `metadata`). Hilfsfunktionen: Bereich einer Seite, erste Seite eines
  Bereichs, aktuelle Seite zu einem Pfad. `sections.test.ts` nachziehen.
- Texte: `admin.nav.areas.*` (de: «Medien», «Nutzer & Geräte», «Startseite», «Automatisierung»,
  «System»), `admin.sections.audiobookSources.title` (de: «Hörbuch-Quellen»). Die bestehenden Titel
  der Seiten bleiben, wie sie sind.

### b) Seitenleiste (`AdminSideNav`, ab 768 px)

- Zuoberst «Zurück zur App», dann die Überschrift «Verwaltung» (klein, Grossbuchstaben, gedämpft),
  dann sechs Einträge: Übersicht und die fünf Bereiche. Ein Bereichseintrag führt auf die erste Seite
  des Bereichs und ist aktuell, solange eine Seite des Bereichs offen ist.
- Aussehen nach Mockup (`.nav-btn`, `.nav-btn.active`): aktueller Eintrag mit Akzentfläche
  (Profilakzent `--sc-accent`, getönt), Akzentstrich links, Pfeil rechts, Symbol in Akzentfarbe.
- Pille oder Abzeichen am Bereichseintrag nach der Tabelle in Bauplan 1 (Medien: offene Pflegefälle;
  Nutzer & Geräte: laufende Streams; Automatisierung: fehlgeschlagene Jobs, sonst stille Webhooks).
  Das Abzeichen «Update» bleibt in diesem Schritt an «Übersicht».
- Fuss der Leiste: aktives Profil (Initiale, Name) mit Rolle als reine Anzeige.
- Bedienung bleibt: ein Tab-Stopp, Pfeiltasten, Pos1/Ende, `focusCarry`, `ADMIN_OVERVIEW_INDEX`,
  `shellFocus`. Der Kontrasttest der acht Akzente in `AdminSideNav.test.tsx` muss weiter gelten
  (für die neue Akzentfläche anpassen, nicht streichen).

### c) Handy (unter 768 px)

- Der Knopf «Verwaltung · Seite» über jeder Seite bleibt und öffnet das Blatt (`AdminSideNavMenu`).
  Das Blatt zeigt «Zurück zur App», «Übersicht» und die fünf Bereiche mit **allen** ihren Seiten
  (wie heute die vier Gruppen), mit den Zahlen der Seiten. So bleibt jede Seite ein Tippen entfernt.
- Der Knopf sieht aus wie die Brotkrumen-Zeile des Mockups auf dem Handy.

### d) Kopfzeile (`AppHeader`, nur in der Verwaltung anders)

- Zusatz «Control Center» am Logo (Text aus den Sprachdateien, de und en gleich).
- Brotkrumen ab 768 px in der Kopfzeile: «Verwaltung › Medien / Bibliotheken» als
  `<nav aria-label>` mit Verweisen; «Verwaltung» zur Übersicht, der Bereich auf seine erste Seite,
  die Seite `aria-current="page"`. Auf der Übersicht: «Verwaltung › Übersicht».
- Statt des Verweises «Suche» der Knopf «Bereich suchen» mit Kürzel-Hinweis (`Strg K` bzw. `⌘ K`,
  auf schmalen Breiten nur das Symbol). Glocke und Profil bleiben.
- `AppHeader` und `AppLayout` liegen im Startbündel: nichts aus `@/ui/aria` dort einbinden (P-10,
  siehe Kommentar in `AppLayout.tsx`). Die Palette lädt mit dem Verwaltungs-Bündel.
- Die Pfeiltasten-Navigation der Kopfzeile (`SpatialGroup`, `useSpatialItem`) gilt auch für die
  neuen Elemente.

### e) Suchpalette

- Neues Bauteil in `web/src/ui` (allgemein: Einträge mit Titel, Zusatz, Symbol, Ziel), mit Story und
  Test; die Verwaltung füllt es aus `sections.ts`: Übersicht, jeder Bereich, jede Seite (Zusatz:
  Bereich), «Zurück zur App», «Mediensuche» (`paths.search`).
- Dialog mit Fokusfalle (bestehendes `Dialog`/React-ARIA nutzen), Suchfeld hat den Fokus, Tippen
  filtert (Titel und Zusatz, ohne Gross-/Kleinschreibung), Pfeil auf/ab wählt, Enter öffnet, Esc
  schliesst, Fokus kehrt auf den Knopf zurück. Leerzustand «Nichts gefunden».
- `Strg+K` / `⌘K` öffnet sie auf jeder Verwaltungsseite (nicht, wenn schon ein Dialog offen ist).
  Die Taste `/` (Mediensuche, `AppLayout`) bleibt.

### f) Bereichsreiter und Seitenkopf (`AdminPage`)

- `AdminLayout` setzt einen Kontext (Definition in `web/src/ui`), aus dem `AdminPage` liest:
  Bereichsreiter, Zeile über dem Titel («MEDIEN / BIBLIOTHEKEN»), auf dem Handy der Menü-Knopf.
  Ohne Kontext (`/settings/*`, Stories ohne Rahmen) verhält sich `AdminPage` **genau wie heute**
  (Hilfe-Knopf, Titelgrösse).
- Bereichsreiter: neues Bauteil in `web/src/ui`, `<nav aria-label>` mit Verweisen und
  `aria-current="page"`, Aussehen nach Mockup (`.tabs`, `.tab`, `.tab.active`), mit Zahl, Pille oder
  Abzeichen je Reiter (dieselben Angaben und gesprochenen Namen wie heute an den Einträgen der
  Seitenleiste: Titelzahl, offene Pflegefälle, Kontenzahl, Streams, Jobs, Integrationen). Bricht auf
  schmalen Breiten um. Ein Tab-Stopp mit Pfeiltasten wie die Seitenleiste, oder jeder Reiter ein
  Tab-Stopp: wähle, was zu den bestehenden Mustern in `web/src/ui` passt, und begründe es im Commit.
- Die Übersicht hat keine Bereichsreiter. Unterseiten (`/admin/podcasts/:id`, `/admin/items/:id`,
  `/admin/metadata/audiobooks`) zeigen die Reiter ihres Bereichs.
- Seitenkopf mit Kontext: Zeile über dem Titel in Akzentfarbe, Titel gross (Mockup `.heading h1`),
  Einleitung (`help`) **sichtbar** unter dem Titel, Hauptaktionen rechts, «Live …» wie heute. Der
  Hilfe-Knopf entfällt in diesem Modus.
- Seiten, die `AdminPage` nicht nutzen (prüfen: Meldung für Nicht-Administratoren, Lade- und
  Fehlerseiten), dürfen ohne Bereichsreiter bleiben, aber nicht brechen.

### g) Aussehen unter `data-shell="admin"`

- Die gemeinsamen Bauteile, aus denen die Verwaltungsseiten bestehen (`Panel`, `DataTable`,
  `StatTile`, `StatusStrip`, `DashCard`, `Chip`/Abzeichen, `Notice`, `Tabs`, `Button`, Felder),
  bekommen unter `data-shell="admin"` das Aussehen des Mockups (Karte `.card`, Tabelle `.table`,
  Abzeichen `.badge`, Kennzahl `.stat`, Hinweisband `.callout`, Felder `.field`, Knöpfe `.btn`):
  über Komponenten-Tokens, die `density.css` (oder eine Datei daneben) unter der Verwaltung
  umstellt, nicht über Sonderfälle in jedem Bauteil. Wo ein Bauteil keine Token-Stelle hat, eine
  einführen. Ausserhalb der Verwaltung darf sich **kein Pixel** ändern (die Storybook-Bilder ohne
  `density: 'compact'` müssen unverändert bestehen).
- Nur Tokens (`packages/design-tokens/tokens/`, danach `task gen`), keine Farbe im CSS, kein Text im
  Code. Profilakzent `--sc-accent`, nie fest Ember.
- Die kompakte Dichte (ADR 0020) bleibt die Grundlage. Wenn du Masse der Dichte änderst, halte es im
  Commit fest; der Auftraggeber achtet auf die Höhe der Seiten: **die erste Arbeitszeile einer Seite
  soll nicht tiefer rutschen als nötig** (Bereichsreiter, Zeile über dem Titel, Titel und Einleitung
  zusammen so knapp wie im Mockup).

## Tests (CLAUDE.md, Stufe 1)

- `task check:fast` nach jedem grösseren Stück und vor jedem Commit.
- Vitest der geänderten Bauteile (läuft in `check:fast` über `vitest related`).
- e2e gezielt, mit eigenem Port: `E2E_PORT=8297 task e2e:quick -- e2e/<spec>` (auf der Maschine
  laufen andere Sitzungen auf 8097 und 8197). Betroffen sind mindestens `admin-home.spec.ts`,
  `admin-phone.spec.ts`, `admin-density.spec.ts` und jede Spec, die über Einträge der Seitenleiste,
  den Hilfe-Knopf oder die Kopfzeile geht (`grep` in `web/e2e`). Bestehende Specs anpassen, wo sich
  die Bedienung absichtlich ändert; **keine Prüfung streichen**, die eine Funktion belegt.
- Neue Spec `web/e2e/admin-shell.spec.ts` (Tastatur, Regel 8): Seitenleiste mit sechs Einträgen und
  Pfeiltasten; Bereichsreiter erreichen jede der 20 Seiten; Brotkrumen; Palette öffnen mit Knopf und
  Kürzel, filtern, Enter, Esc mit Fokusrückkehr; Handy-Blatt (390 px) erreicht jede Seite;
  Deeplinks der Unterseiten zeigen den richtigen Reiter; sichtbarer Fokus, keine Fokusfalle.
- Storybook: neue Bauteile mit Story; `task storybook:test` für die geänderten Bauteile. Bilder,
  die sich **absichtlich** ändern (nur Stories mit `density: 'compact'` und die der umgebauten
  Bauteile), mit `task storybook:update -- <Filter>` neu schreiben und die PNG-Differenzen ansehen.
  Ändert sich ein Bild ausserhalb der Verwaltung, ist das ein Fehler in deinem CSS.
- Höchstens drei Korrekturversuche je roter Prüfung, dann anhalten und melden. Nichts überspringen
  (`.skip`, `.only`), keine Timeouts verlängern, keine Grenzen lockern.
- Kein `task ci`, keine ganze e2e-Suite: das macht der Koordinator vor dem PR.

## Bildschirmfotos für den Koordinator

Am Ende mit einem Wegwerf-Skript (nicht committen) gegen den e2e-Server Bilder ablegen in
`/tmp/claude-1000/-home-marco--claude-spares-spare-QLalpS/e5de04e4-51ba-44f0-bdf1-b99aeb6b0581/scratchpad/s1/`:
`/admin/libraries`, `/admin/jobs`, `/admin/metadata/audiobooks` je in 1440×900 und 390×844,
`/admin/libraries` in 768×1024, die offene Palette (1440) und das offene Handy-Blatt (390).

## Commits

Sinnvoll geteilte Commits auf `claude/admin-redesign-s1` (z. B. Navigationsmodell; Seitenleiste und
Kopfzeile; Palette; Bereichsreiter und Seitenkopf; Aussehen; Tests). Titel englisch mit Bereich
(`feat(web): …`), Body englisch, letzte Zeile genau:

`Co-Authored-By: Claude Fable 5.2 <noreply@anthropic.com>`

## Rückmeldung

- Commit-SHAs mit je einem Satz.
- Je Prüfung: Befehl und die entscheidenden Zeilen der Ausgabe.
- Jede geänderte oder gestrichene Testzeile mit Grund.
- Was du am Bauplan anders lösen musstest und warum.
- Neue Texte (Schlüssel) in `de`, `en`, `de-DE`.
- Offenes, Unsicheres, Folgen für die Seitenschritte (z. B. neue Token-Stellen, die Seiten nutzen
  sollen; neue Bauteile mit ihren Props).
