# Nachprüfung der Funktionsabdeckung

Prüfstand: `7dd1562f6ab58717081e5cd1b4239d2a2be87614` (HEAD von `claude/admin-redesign`). Ich habe
die Befunde vom Stand `4251cf7a` nachgeprüft und das Delta `4251cf7a..HEAD` gelesen (12 Commits unter
`web`). Massgeblich war der Code, nicht die Berichte. Nachgeführt sind die Zeilen in
`medien1.csv`, `medien2-nutzer.csv`, `startseite-system.csv` und `automatisierung-shell.csv`.
`funktionsabdeckung.csv` ist mit `zusammenfuehren.py` neu geschrieben.

Selbst laufen lassen (Vitest, einmal, alle Dateien mit neuen Belegen):

```
pnpm exec vitest run src/ui/AdminPage.test.tsx src/ui/StatusStrip.test.tsx
  src/features/admin/integrations/services.test.ts src/features/admin/ServerPage.test.tsx
  src/features/admin/AdminHomePage.test.tsx src/features/admin/BackupPage.test.tsx
  src/features/admin/RadioAdminPage.test.tsx src/features/admin/StreamsPage.test.tsx
  src/features/admin/MetadataPage.test.tsx src/features/admin/AudiobookSourcesPage.test.tsx
  src/features/admin/SpotlightPage.test.tsx src/features/admin/libraries/LibrariesPage.test.tsx
Test Files  12 passed (12)
     Tests  255 passed (255)
```

Kein e2e und kein Storybook (Auftrag). Die e2e-Belege beruhen auf dem Lesen der Specs.

## Aufgabe 1: Befunde

| ID | Befund 4251cf7a | Behoben | Commit | Code heute | Test |
|---|---|---|---|---|---|
| MET-027 | LÜCKE | ja | `b5a4b56b` | `MetadataPage.tsx:909` `serviceHref('keys')` | `MetadataPage.test.tsx:177`, Zeile 284 prüft href `/admin/integrations?item=keys` |
| ABS-009 | LÜCKE | ja | `b5a4b56b` | `AudiobookSourcesPage.tsx:430` `serviceHref('keys')` | `AudiobookSourcesPage.test.tsx:246` (href genau); `e2e/audiobook-sources.spec.ts:120-123` |
| INT-051 | LÜCKE | ja | `b5a4b56b` | `spotlight/FetchState.tsx:100`, `spotlight/SourceForm.tsx:195` `serviceHref('keys')`; Fehlerhinweis `server/parts.tsx:336` `integrationHref` | `SpotlightPage.test.tsx:190`, `:693`, `:699`; `e2e/admin-spotlight.spec.ts:136`; `integrations/services.test.ts` |
| LIB-047 | LÜCKE | ja | `1fe6eb79` | `de.json` `admin.libraries.delete.backup` «(Verwaltung › System › Sicherung)», `en` «(Administration › System › Backup)», `de-DE` erbt | `LibrariesPage.test.tsx:900` im Test `:839` |
| OVW-063 | LÜCKE Test | ja | `7dd1562f` | `server/parts.tsx:1061-1123`, leer `:1090` | `ServerPage.test.tsx:399` prüft «Server-Protokoll · keine Fehler», den zugeklappten Satz «Keine Fehler seit dem Start.» und die leere Live-Region |
| HOM-021 | LÜCKE Test | ja, mit Einschränkung | `7dd1562f` | `homerows/HomeRows.module.css:160-188` (unverändert) | `e2e/admin-home-rows.spec.ts:297`: 1440 px nebeneinander, Tabelle ≥ 36rem; 900 px untereinander, keine waagrechte Seitenrolle |

Zu HOM-021: Der Test prüft «≥ 36rem». Das gilt auch bei der alten Grenze von 48rem. Dass die
Mindestbreite neben der Vorschau genau auf 36rem sinkt, belegt er also nicht. Die Lage nebeneinander
und gestapelt ist belegt. Darum habe ich die Zeile auf `ok` gesetzt und die Einschränkung in der
Bemerkung vermerkt.

Hinweise ohne Status:

| Punkt | Behoben | Commit | Code heute | Test |
|---|---|---|---|---|
| STR-004 Überschrift «Serverlast» | ja | `c99afbec` | `StreamsPage.tsx:463` `heading`; `ui/StatusStrip.tsx:24-38`: verborgene h2, die Gruppe ist per `aria-labelledby` nach ihr benannt | `StreamsPage.test.tsx:217`; `StatusStrip.test.tsx` (beide Fälle); `e2e/admin-streams.spec.ts:152` |
| RAD-064 Sender aus dem Kopf | ja | `de8919dc` | `RadioAdminPage.tsx:246-252`: eine Navigation mit `tab: 'eigene'` und `item` (replace wie bei `openEditor`) | `RadioAdminPage.test.tsx:407`: von «Übersicht» aus angelegt, danach Escape, dann ist «Eigene Sender» gewählt und die Zeile steht in der Tabelle |
| MNT-006 Zahl am Reiter «Pflege» | ja | `7dd1562f` | `AdminShellNav.tsx:92-95` | `AdminHomePage.test.tsx:756`: Name «Pflege, 4 Titel brauchen Pflege», href, sichtbare Zahl |
| Kopfknopf «Neue Sicherung» (BKP-004) | ja | `7dd1562f` | `BackupPage.tsx:357-374`, `:361` `!restart` | `BackupPage.test.tsx:612` (öffnet `?tab=create`); `:578` (gesperrt während der Wiederherstellung, fehlt auf der Neustart-Tafel) |
| Integrationsfehler führt zum Dienst (OVW-043) | ja | `b5a4b56b` | `server/parts.tsx:336` `integrationHref(integration.key)` | `AdminHomePage.test.tsx:179` (Trakt → `?item=keys`); `services.test.ts` (alle acht Schlüssel) |
| Linktext Grafikchip (OVW-056, Fund 5) | ja | `27dd9c63` | `server/parts.tsx:413` «Zur Grafikchip-Wahl» → `?item=server` | `AdminHomePage.test.tsx:476`; die Zeile stand vorher auf `ok, ohne Test` |

Die Hinweiszeilen sind mit Beleg und Bemerkung nachgeführt: STR-004, RAD-064, MNT-006, BKP-004,
OVW-043, OVW-056, dazu OVW-032 und OVW-036 wegen «Gerade aktiv».

## Aufgabe 2: Delta `4251cf7a..HEAD`

Ich habe keinen neuen Verlust gefunden. Keine Inventarzeile ist auf `LÜCKE` gesetzt.

1. **Einleitung auf zwei Zeilen** (`ui/AdminPage.tsx:153-206`, `AdminPage.module.css` `.introClamped`).
   - Gekürzt wird nur mit `line-clamp` und `overflow: hidden`. Der ganze Text bleibt im DOM, nicht
     `hidden` und nicht `display: none`. Ein Screenreader liest ihn also ganz. Der Bauteiltest
     prüft das mit `getByText(long)`.
   - «Mehr»/«Weniger» ist ein echter `button`. Er trägt `aria-expanded` und `aria-controls` mit der
     id des Absatzes. Der Fokus bleibt nach dem Umschalten auf dem Knopf.
   - Belege: `AdminPage.test.tsx` (Enter, Leertaste, Fokus; kein Knopf, wenn der Text passt) und
     `e2e/admin-shell.spec.ts:158` (Tab, sichtbarer Fokus, Höhe, keine Fokusfalle).
   - Der Knopf erscheint nur, wenn der Text wirklich gekürzt ist. Gemessen wird erneut, wenn sich
     die Grösse ändert (ResizeObserver).
   - `/settings/*` bleibt unverändert. `Intro` steht nur in `FramedPage`. Den Rahmen setzt nur
     `AdminLayout.tsx` (`AdminFrame`). Der Zweig ohne Rahmen (Hilfe-Knopf, `AdminPage.tsx:72-117`)
     ist im Diff nicht berührt. `ProfileSettingsPage`, `PlaybackSettingsPage` und
     `ChannelSettingsPage` unter `/settings` laufen weiter über diesen Zweig.
   - Grenzfall ohne Verlust: Wird das Fenster bei offener Einleitung so breit, dass der Text in
     zwei Zeilen passt, verschwindet «Weniger». Der Text bleibt dann ganz sichtbar.
2. **Kopfaktionen auf dem Handy** (`AdminPage.module.css`, `@media (width < 768px)`).
   - Betroffen sind nur direkte Kinder `a` und `button` von `.framedActions`. Dort gilt
     `white-space: normal` und `max-width: 100%`.
   - Ab 768 px ändert sich nichts. Kein Bedienelement fällt weg.
   - Einen eigenen Test hat nur die Breite: `e2e/admin-phone.spec.ts:57` prüft je Seite, dass sie
     nicht breiter ist als das Telefon.
3. **«Gerade aktiv» mit Zwischentiteln** (`AdminHomePage.tsx:98-190`).
   - Zwei Gruppen mit je einer h3, unter der h2 der Karte (`ui/DashCard.tsx:42`).
   - Listen, `aria-label`, Zeilen, Grenze von drei, Leertexte und der Satz zum nächsten Lauf sind
     zeilengleich übernommen, nur eingerückt.
   - Belege: `AdminHomePage.test.tsx:179` (Reihenfolge der h3) und `:577`.
4. **Entfernte Textschlüssel** (11 in `de.json`, dieselben 11 in `en.json`):
   - `admin.home.hints.none`, `admin.home.hints.title`
   - `admin.home.jobs.failed`, `.paused`, `.queued`, `.running`
   - `admin.home.live`, `admin.home.server.title`
   - `admin.homeRows.tabs.label`, `admin.homeRows.tabs.order`
   - `admin.jobs.counts.planned`

   Geprüft habe ich so:
   - Jeden Schlüssel mit `git grep` in `'…'` und `"…"` in `web/src` (ohne `locales`): kein
     Treffer, weder heute noch in `4251cf7a`. Die Treffer in `4251cf7a` waren nur Präfixe
     (`liveFailed`, `failedByType`, `failedType`, `plannedNote`), und diese Schlüssel bestehen weiter.
   - Zusammengesetzte Aufrufe mit `` `admin.(home|homeRows|jobs).…${` ``: Keiner führt auf einen
     entfernten Schlüssel. `admin.jobs.counts.*` und `admin.homeRows.tabs.*` werden nirgends
     zusammengesetzt.
   - Die Schlüssel sind typisiert (`i18n/i18next.d.ts`, `typeof de`). Ein statischer Aufruf eines
     entfernten Schlüssels fiele also schon in `tsc` auf.
   - «Live, alle 5 Sekunden» in den Tests kommt aus `ui.adminPage.live_other`. Der Text besteht
     weiter.
5. **Verweisziele.**
   - `STATUS_KEYS` (`integrations/services.tsx:28-35`) stimmt mit den Ansichten der Seite überein:
     - Ansicht `keys`: `PROVIDERS` (`ProviderCard.tsx:37-52`, mit tmdb, tvdb, fanart, youtube,
       trakt, googlebooks) und damit auch die Schlüssel von MET-027 und ABS-009
     - Ansichten `email`, `telegram` und `push`: `IntegrationsPage.tsx:270-285`
     - `updateCheck` steht in der Ansicht `server` (`ServerBlock.tsx:159` `UpdateRow`)
   - Der API-Typ kennt genau diese acht Schlüssel. `services.test.ts` prüft alle acht.
   - Ohne `?item=`, also auf den Kacheln, bleiben drei allgemeine Einstiege. Sie meinen mehrere
     Dienste, das ist kein Verlust:
     - der Fuss der Serverkarte (`AdminHomePage.tsx:250`)
     - «Kanäle des Servers einrichten» (`ChannelSettingsPage.tsx:146`)
     - «Alle Integrationen» auf der Seite selbst (`IntegrationsPage.tsx:202`)
6. **Tests.**
   - Gestrichene Erwartungszeilen im Delta: acht. Jede ist durch eine gleich strenge oder strengere
     ersetzt:
     - genaues href `?item=…` statt `/admin/integrations`
     - `{ tab: 'eigene', item: '9' }` statt `{ item: '9' }`
     - Gruppe und h2 statt nur der Gruppe
   - Kein `.skip` und kein `.only`. Kein Timeout geändert oder neu gesetzt.
7. **Geänderte Ortsangaben in Texten** (`27dd9c63`, `1fe6eb79`). Diese Texte nennen jetzt den neuen
   Ort: `setup.…radioHint` («Medien › Radio»), `household.accounts` («Nutzer & Geräte › Konten»)
   und `done.busy` («System › Server»). Das ist eine Berichtigung, kein Verlust. Einen eigenen Test
   haben diese Einrichtungstexte nicht, vorher auch nicht.

## Zählung nach `zusammenfuehren.py`

| Seite | Zeilen | ok | ok, ohne Test |
|---|---:|---:|---:|
| LIB | 87 | 77 | 10 |
| MET | 46 | 30 | 16 |
| ABS | 25 | 19 | 6 |
| MNT | 39 | 37 | 2 |
| ITM | 69 | 59 | 10 |
| POD | 56 | 49 | 7 |
| PDP | 16 | 9 | 7 |
| RAD | 77 | 53 | 24 |
| ACC | 44 | 40 | 4 |
| DEV | 14 | 13 | 1 |
| STR | 17 | 17 | 0 |
| HOM | 21 | 19 | 2 |
| SPT | 33 | 32 | 1 |
| PCK | 13 | 7 | 6 |
| COL | 23 | 23 | 0 |
| MYM | 20 | 16 | 4 |
| ANN | 24 | 24 | 0 |
| STA | 14 | 13 | 1 |
| BKP | 22 | 21 | 1 |
| NAV | 32 | 30 | 2 |
| OVW | 66 | 62 | 4 |
| JOB | 43 | 40 | 3 |
| INT | 51 | 46 | 5 |
| NOT | 24 | 23 | 1 |
| **zusammen** | **876** | **759** | **117** |

`LÜCKE`, `LÜCKE Test` und `unklar`: 0. Vorher, in `4251cf7a`, waren es 4 `LÜCKE` und 2
`LÜCKE Test`. OVW-056 ist von `ok, ohne Test` auf `ok` gestiegen.

## Offen

- Die e2e-Belege habe ich nicht laufen lassen (Auftrag). Das betrifft:
  - `admin-home-rows.spec.ts:297` (HOM-021)
  - `admin-shell.spec.ts:158` (Einleitung)
  - `admin-streams.spec.ts:152`
  - `audiobook-sources.spec.ts:123`
  - `admin-spotlight.spec.ts:136`
  - `admin-server.spec.ts:51`

  Ob sie grün sind, zeigt erst `task ci` oder die CI.
- Die Storybook-Baselines `ui-adminpage--framed-long-intro.png` und
  `ui-statusstrip--with-heading.png` habe ich nicht geprüft.
- HOM-021: Der neue e2e unterscheidet die Mindestbreite 36rem nicht von 48rem (siehe oben). Wer das
  belegen will, prüft neben der Vorschau eine Tabellenbreite unter 48rem oder den Wert von
  `--sc-table-fixed-min-width`.
- Die Berichte der vier Teile (`medien1.md`, `medien2-nutzer.md`, `startseite-system.md`,
  `automatisierung-shell.md`) zeigen weiter den Stand `4251cf7a`. Massgeblich für heute sind diese
  Datei und die CSV-Zeilen.
