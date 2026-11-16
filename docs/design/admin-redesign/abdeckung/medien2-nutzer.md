# Funktionsprüfung Teil `medien2-nutzer`

Geprüfter Stand: `4251cf7a` gegen `55b72c9a`. Kürzel `POD`, `PDP`, `RAD`, `ACC`, `DEV`, `STR`
(224 Zeilen). Einzelheiten je Zeile: `medien2-nutzer.csv`.

## Ergebnis

Ich habe keine `LÜCKE`, keine `LÜCKE Test` und kein `unklar` gefunden. Unten stehen drei Hinweise:
Dort ist die Funktion vorhanden, aber anders als vorher. Der Koordinator soll sie bewusst abnehmen.

### Zählung je Kürzel und Status

| Kürzel | Zeilen | ok | ok, ohne Test | LÜCKE | LÜCKE Test | unklar |
|---|---|---|---|---|---|---|
| POD | 56 | 49 | 7 | 0 | 0 | 0 |
| PDP | 16 | 9 | 7 | 0 | 0 | 0 |
| RAD | 77 | 53 | 24 | 0 | 0 | 0 |
| ACC | 44 | 40 | 4 | 0 | 0 | 0 |
| DEV | 14 | 13 | 1 | 0 | 0 | 0 |
| STR | 17 | 17 | 0 | 0 | 0 | 0 |
| **Summe** | **224** | **181** | **43** | **0** | **0** | **0** |

Die Zeilen mit `ok, ohne Test` hatten schon vor dem Umbau keinen Test. Keine dieser Zeilen hat
durch den Umbau einen Test verloren.

Werte in der Spalte `Aenderung`:

- `keine`: 201
- `Aussehen`: 11 (RAD-015, 041, 047, 063, 068 bis 073, 077)
- `verschoben`: 7 (POD-055, RAD-064, RAD-076, ACC-043, DEV-013, STR-004, STR-016)
- `umgebaut`: 5 (POD-002, RAD-002, ACC-002, DEV-002, STR-002: Hilfe-Knopf entfällt nach Bauplan 2.6,
  die Einleitung steht offen)

### Vorgehen

- **154 Zeilen «unverändert»:** übernommen. Jede zehnte Zeile habe ich im Code nachgesehen
  (16 Stichproben, in der CSV vermerkt): POD-001, 012, 022, 032, 042, 052, PDP-008, RAD-020, 030,
  053, 065, ACC-011, 021, 031, 041, DEV-011. Alle Stellen stimmen.
- **70 Zeilen «prüfen»:** jede Zeile am Diff nachvollzogen.
- **Geänderte Seiten im Code:** nur fünf Dateien unter `radio/`, dazu `RadioAdminPage.tsx`,
  `StreamsPage.tsx` und zwei CSS-Dateien (`Radio.module.css`, `Podcasts.module.css`).
- **Unverändert seit `55b72c9a`:** Podcasts, Podcast-Seite, Konten, Geräte, `accounts/`,
  `devices/`, `streams/`, `podcasts/` (ausser CSS) und `features/profiles`.
- **Zeilennummern in «Neue Stelle»:** auf den heutigen Stand umgerechnet. Mehrere davon habe ich
  von Hand nachgesehen.
- **Vitest-Lauf:** `RadioAdminPage.test.tsx` und `StreamsPage.test.tsx` mit
  `pnpm exec vitest run`, 2 Dateien, 50 Tests grün.

## Hinweise (keine Lücke, bewusst abzunehmen)

### H-1 Streams: «Serverlast» ohne sichtbare Überschrift (STR-004)

- **Vorher:** `BandPanel` mit Überschrift h2 «Serverlast» und Symbol `server` als erster Inhalt
  (`StreamsPage.tsx:468-500` in `55b72c9a`).
- **Jetzt:** dieselbe `StatusStrip` mit allen sechs Kacheln, gleichen Werten und Notizen im
  `status`-Platz von `AdminPage`, direkt unter dem Kopf (`StreamsPage.tsx:459` `serverLoad`, Zeile 497 `status={serverLoad}`).
- **Was fehlt:** die sichtbare Überschrift und das h2. Der Name «Serverlast» bleibt nur als
  Gruppenname (`role="group"`, `aria-label`).
- **Folgen:** Wer mit dem Screenreader von Überschrift zu Überschrift springt, findet «Serverlast»
  nicht mehr. Wer die Seite sieht, liest über den Kacheln keinen Titel mehr.
- **Tests:** `StreamsPage.test.tsx` (2 Stellen) und `e2e/admin-streams.spec.ts` (1 Stelle) prüfen
  statt `heading` neu `group`. Sonst ist die Aussage gleich.
- **Warum keine Lücke:**
  - Bauplan 2.6 und S-4 sehen die Kennzahlen unter dem Kopf vor.
  - Auf Radio steht die «Zusammenfassung» schon vor dem Umbau ohne Überschrift im selben Platz.
  - Keine Zahl und kein Zustand fehlt.
- **Wenn die Überschrift bleiben soll:** sichtbare Beschriftung über der Leiste oder h2 vor der
  `StatusStrip`.

### H-2 Radio: «Eigenen Sender hinzufügen» im Seitenkopf (RAD-063 bis RAD-065)

- **Vorher:**
  - Mit Einträgen stand der Knopf im Kopf des Abschnitts «Eigene Sender».
  - Bei leerer Liste stand er neben «Noch kein eigener Sender.».
  - Stellen in `55b72c9a`: `radio/OwnStationsTab.tsx:77-81`, `154`, `162`.
  - Dialog und Abschluss: `OwnStationsTab.tsx:189-200`.
- **Jetzt:**
  - Der Knopf ist die Hauptaktion im Seitenkopf: `RadioAdminPage.tsx:173-177`.
  - Er steht auf jedem Reiter, also auch auf «Übersicht», «Angebot», «Sender» und «Einstellungen».
  - Im Reiter und im Leerzustand steht kein Knopf mehr. `RadioAdminPage.test.tsx` prüft, dass es nur
    einen Knopf gibt.

Die einzelnen Punkte des Auftrags:

- **Auffindbarkeit im Leerzustand:**
  - Der Leertext «Noch kein eigener Sender.» steht allein.
  - Der Knopf steht auf demselben Bildschirm im Kopf darüber.
  - In der Tab-Reihenfolge kommt der Knopf vor den Reitern. Der e2e-Test erreicht ihn mit
    `tabTo(…, 80)` unverändert.
  - Der Leertext nennt keinen Ort, also verweist er auch nicht falsch.
  - Ergebnis: schlechter auffindbar ist nur die direkte Nachbarschaft zum Leertext. Das habe ich
    nicht als Lücke gewertet.
- **Fokus nach dem Anlegen:**
  - Wie vorher öffnet sich nach dem Anlegen der Editor in `?item=`.
  - Schliesst sich der Editor, gibt `EditorDialog` den Fokus an das Element zurück, das ihn öffnete.
    Das ist in beiden Ständen der Absendeknopf des Anlegedialogs, und den gibt es dann nicht mehr.
  - Vorher verschwand bei leerer Liste zusätzlich der Knopf im Leerzustand, sobald die Liste einen
    Eintrag hatte. Jetzt bleibt der Kopfknopf stehen.
  - Ergebnis: keine Verschlechterung. Getestet ist dieser Fokus weder vorher noch heute.
- **Ansage:**
  - Die Meldung «{{name}} ist hinzugefügt.» geht neu über eine `StatusRegion` der Seite
    (`RadioAdminPage.tsx`), vorher über die des Reiters. Der Weg ist derselbe (`useStatusMessage`).
  - Der neue Test prüft den Text mit `findByText`.
  - Die Meldung «… ist gelöscht.» bleibt in der `StatusRegion` des Reiters.
- **Sichtbarkeit des neuen Senders:**
  - Die Invalidierung `radioAdminKeys.all` ist unverändert.
  - Im Reiter «Eigene Sender» erscheint der Sender wie vorher in der Liste. Das prüft der e2e-Test:
    Nach Escape findet er die Zeile in der Tabelle «Eigene Sender».
  - **Neu:** Legt man den Sender von einem anderen Reiter aus an, wechselt die Seite nicht zu
    «Eigene Sender». Nach dem Schliessen des Editors sieht man den neuen Sender also nicht.
  - Das ist ein neuer Weg, den es vorher nicht gab, und darum kein Verlust.
  - Mögliche Verbesserung: nach dem Anlegen `?tab=eigene` setzen.

### H-3 Bereichsreiter statt Seitenleisten-Eintrag (POD-055, RAD-076, ACC-043, DEV-013, STR-016)

- Die Seiten stehen als Bereichsreiter unter «Medien» bzw. «Nutzer & Geräte».
- Das Seitensymbol (mic, radio, users, monitor, play) erscheint dort nicht mehr. Die Seitenleiste
  zeigt das Bereichssymbol.
- Zahl und Pille stehen am Reiter (Bauplan 1). Damit ist alles erreichbar und getestet
  (`e2e/admin-shell.spec.ts`, `sections.test.ts`, die Seitentests mit `findAdminTabs`).

## Funde ohne Inventarzeile (Schritt 3)

- **`.skip`, `.only`, Zeitlimits:** kein `.skip` und kein `.only` in den Tests meiner Seiten. Kein
  `test.setTimeout` geändert.
- **Gestrichene `expect`:** 13 Zeilen mit `expect(` in den Tests meiner Seiten sind gestrichen.
  Jede ist gleichwertig ersetzt:
  - Seitenleiste wird Bereichsreiter, mit `ArrowRight`/`ArrowLeft` statt `ArrowUp`/`ArrowDown`.
  - h1 der Übersicht heisst jetzt «Alles im Blick».
  - «Serverlast»: `heading` wird `group` (H-1).
  - `e2e/admin-radio.spec.ts`: Zurück führt jetzt auf `/admin/libraries`, weil der Weg über
    «Medien» geht.
  - `e2e/invitations.spec.ts`: Zurück führt jetzt über «Geräte» statt «Metadaten».
- **Neue Tests:**
  - `RadioAdminPage.test.tsx` «adds an own station from the head on any tab (the page's main
    action)».
  - `e2e/admin-phone.spec.ts` prüft neu auch `/admin/radio` und `/admin/podcasts/:id`.
- **`de.json`:** keine Texte meiner Seiten entfernt. Die gestrichenen Schlüssel (`groups.*`,
  `jobs.livePaused`, `restartAdmin`, `libraryHint`) gehören zu anderen Teilen.
- **Neue Texte:** `admin.radio.settings.relaysCard` und `coverCard` stehen in `de` und `en`.
- **`aria-`-Angaben:** keine entfernt. `aria-label` des Anbieter-Knopfs (RAD-041) steht unverändert
  am Knopf, nur eingerückt. Das entfernte `StatusDot` war `aria-hidden`. Der Zustand wird weiter als
  Text gesprochen, jetzt im `Chip`.
- **Radio-Einstellungen:** Das Formular steht jetzt in zwei Karten («Puffer und Weiterleitungen»,
  «Cover und externe Daten»). Felder, Entwurf, `SaveBar` und Rechtshinweis sind unverändert.
- **Podcasts:** Nur CSS hat sich geändert: die Filterleiste ist gerahmt, der Kopf der Podcast-Seite
  steht als Karte. An der Bedienung hat sich nichts geändert.

## Unklar geblieben

Nichts. Nicht selbst laufen lassen habe ich die e2e-Tests (nach dem Auftrag untersagt). Ihre Belege
beruhen auf dem Lesen des Codes.
