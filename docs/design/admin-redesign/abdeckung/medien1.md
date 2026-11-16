# Funktionsprüfung Teil `medien1` (LIB, MET, ABS, MNT, ITM)

Geprüfter Stand: `4251cf7a` (Zweig `claude/admin-redesign`), verglichen mit `55b72c9a`.
Zeilen: 266 in `medien1.csv`. Geprüft wurde am Code, nicht an den Berichten der Executor.

## Ergebnis in Kürze

- Auf den Seiten dieses Teils hat der Umbau wenig geändert. `MetadataPage.tsx`, `MaintenancePage.tsx`,
  `AdminItemPage.tsx`, `item/`, `editor/`, `MatchDialog.tsx` und `deletion/` sind seit `55b72c9a`
  unverändert. Geändert sind `libraries/libraryColumns.tsx` (Zustand als Chip statt Statuspunkt),
  `Libraries.module.css` (unbenutzte Klasse entfernt), `Maintenance.module.css` (Rahmen um die
  Werkzeugleiste) und `AudiobookSourcesPage.tsx` mit `AudiobookSources.module.css` (Abschnitte als
  `BandPanel`-Karten, Speicherleiste nach der Karte, Pfade brechen um). Dazu kommt der Rahmen:
  `AdminPage` (Einleitung offen statt Hilfe-Knopf, Bereichsreiter), `AdminLayout` und `sections.ts`.
- Es fehlt kein Bedienelement, kein Dialog und keine Berechtigungsprüfung. Drei Zeilen sind
  `LÜCKE`: zwei Verweise führen seit dem Umbau der Integrationen nur noch über einen Umweg ans Ziel,
  und ein Text nennt einen veralteten Ort.
- `AudiobookSourcesPage.test.tsx` lief gezielt: 10 von 10 Tests bestanden
  (`pnpm exec vitest run src/features/admin/AudiobookSourcesPage.test.tsx`).

## LÜCKE, LÜCKE Test, unklar

| ID | Status | Befund |
|---|---|---|
| MET-027 | LÜCKE | Der Link «Schlüssel unter Integrationen» (`MetadataPage.tsx:908`) zeigt weiter auf `/admin/integrations`. In `55b72c9a` standen dort alle Bänder mit den Anbieterschlüsseln auf einer Seite. Heute zeigt die Adresse ohne `?item=` nur die Kacheln der Dienste (`integrations/services.tsx:8-16`); die Schlüssel kommen erst nach einem weiteren Schritt (`?item=keys`). Das ist ein Umweg. Zu tun: `serviceHref('keys')` verwenden, wie es `4251cf7a` schon für Übersicht und Server macht, und in `MetadataPage.test.tsx:282` das `href` prüfen. |
| ABS-009 | LÜCKE | Gleicher Befund für «Unter «Integrationen» einrichten» (Schlüssel für Google Books, `AudiobookSourcesPage.tsx:429`). Der Test `AudiobookSourcesPage.test.tsx:246` sichert das alte Ziel `/admin/integrations` ab und bleibt deshalb grün. Zu tun: Link auf `/admin/integrations?item=keys` setzen und den Test anpassen. |
| LIB-047 | LÜCKE | Der Dialog «Bibliothek löschen» sagt in `admin.libraries.delete.backup`: «Vorher legt Slouchcat eine Sicherung der Datenbank an (Verwaltung › Sicherung).» (`en`: «Administration › Backup»). Die Seite «Sicherung» steht jetzt unter «System» (Brotkrumen «Verwaltung › System / Sicherung»). Der Text war vor dem Umbau richtig und ist jetzt veraltet. Zu tun: wie in Bauplan 7.2 («Übersicht › Neu starten» wurde «System › Server») auf «System › Sicherung» ändern, in `de`, `de-DE`, `en` und den übrigen Sprachen. |

`LÜCKE Test` und `unklar`: keine.

Schwächere Stellen ohne eigenen Status (die Zeile ist trotzdem `ok`):

- **MNT-006**: Die Zahl der Pflege-Fälle steht nach Bauplan 1 am Bereichsreiter «Pflege» und als
  Warnpille an «Medien». Die Vitest-Fälle prüfen die Pille an «Medien» (`AdminHomePage.test.tsx:620`)
  und die Zahl im Handy-Blatt (`:799`). Die Zahl am Bereichsreiter «Pflege» auf dem Desktop prüft
  kein Test direkt. Sie kommt aus derselben Funktion `figure()` wie im Blatt.
- **MET-046**: `e2e/invitations.spec.ts` (heute `:248`, Ende des Tests) erreichte früher «Metadaten»
  per Pfeiltaste in der Seitenleiste. Heute geht der Weg per Bereichsreiter zu «Geräte». Damit belegt
  die Stelle nichts mehr für «Metadaten». Der Tastaturweg zu jeder Seite steht jetzt in
  `e2e/admin-shell.spec.ts:111`.
- **LIB-002**: Der genannte Test `admin-home.spec.ts:508` heisst jetzt «on a phone: «Verwaltung ·
  Übersicht» opens every page as a sheet, Escape returns» (`:294`) und prüft die h1 «Bibliotheken»
  weiter. Den Dokumenttitel prüft wie vorher kein Test.
- **MET-002, MNT-002, ABS-002, LIB-003**: Der Hilfe-Knopf entfällt nach Bauplan 2.6, die Einleitung
  steht offen. Der Mechanismus ist in `web/src/ui/AdminPage.test.tsx:69` und
  `e2e/admin-shell.spec.ts:145` geprüft. Den Einleitungssatz der einzelnen Seite prüft kein Test
  (vorher auch nicht).

## Funde ohne Inventarzeile (Schritt 3)

- **Verweise auf `/admin/integrations` ausserhalb dieses Teils.** Dieselbe Ursache wie bei MET-027
  und ABS-009 betrifft weitere Verweise ohne `?item=`: `spotlight/SourceForm.tsx:195`,
  `spotlight/FetchState.tsx:100` (Trakt-Schlüssel), `notifications/ChannelSettingsPage.tsx:146`,
  `IntegrationsPage.tsx:202` und `AdminHomePage.tsx:243`. Die Teile, denen diese Seiten gehören,
  sollten prüfen, ob dort ein Dienst gemeint ist.
- **Gelockerte oder gestrichene Prüfungen in den Tests dieses Teils:** keine gefunden.
  - `admin-libraries.spec.ts:161` prüft statt des Hilfe-Knopfs den Bereichsreiter. Das ist die
    Abweichung nach Bauplan 2.6.
  - `MetadataPage.test.tsx:181` prüft jetzt, dass der Hilfe-Knopf fehlt.
  - `LibrariesPage.test.tsx` (3 Stellen) und `admin-library-draft.spec.ts` verlassen die Seite über
    «Automatisierung» statt «Integrationen». Die Aussage (Rückfrage beim Verlassen mit Entwurf) bleibt.
  - `admin-metadata.spec.ts` und `admin-item.spec.ts` prüfen `aria-current` am Bereichsreiter statt
    an der Seitenleiste.
  - Kein `.skip` und kein `.only`, keine verlängerten Timeouts in diesen Dateien.
  - `admin-phone.spec.ts` prüft neu auch `/admin/metadata/audiobooks` (ABS-025).
- **Übersetzungen (`de.json`):** In diesem Teil ist kein Text entfallen.
  - `admin.itemDelete.libraryHint` ist berichtigt: «Pflege › Entfernte Titel» statt «Wartung ›
    Entfernte Titel» (ITM-064, Commit `f420914b`).
  - Ausserhalb dieses Teils ist `admin.jobs.livePaused` («Aktualisierung pausiert») entfallen. Das
    gehört zum Teil «Automatisierung» und ist dort zu prüfen.
- **aria-Angaben:** `AudiobookSourcesPage` ersetzt `section aria-labelledby` mit `SectionHeader` durch
  `BandPanel`. Das rendert weiter `section aria-labelledby` mit einer h2 (`web/src/ui/BandPanel.tsx:124-131`),
  die Regionen bleiben also benannt. Der Statuspunkt in der Spalte «Zustand» (LIB-022) ist durch einen
  Chip ersetzt; der Text trägt die Bedeutung wie vorher.
- **Ansagen:** `StatusRegion` der Einstellungen und von «Jetzt alle …» bleibt erhalten, jetzt
  innerhalb der Karte.

## Vorgehen und Stichprobe

- Bei 209 Zeilen meldete die Vorprüfung «unverändert». Sie sind mit `Aenderung` `keine` übernommen.
  Die Testzeilen sind auf den heutigen Stand nachgeführt: Über den Testnamen wurde die neue Zeile
  gesucht. Das betrifft `admin-libraries`, `admin-library-draft`, `admin-metadata`, `admin-item`,
  `admin-phone` und `invitations.spec.ts`, wo eine Import-Zeile hinzukam.
- Stichprobe: Jede zehnte dieser Zeilen (21 Zeilen, LIB-001 bis ITM-060) wurde an der genannten
  Code-Stelle nachgesehen. Alle stimmen.
- Die 57 Zeilen «prüfen» wurden einzeln am Unterschied und am heutigen Code nachvollzogen. Viele davon
  meldete die Vorprüfung nur, weil sie Wörter wie «Einleitung», «Seitenleiste» oder «Hilfe» enthalten
  (Grund «Rahmen»), oder weil ein Testname als Template-String in einer Schleife steht
  (`admin-phone.spec.ts:57`).
- MET-027 und LIB-047 stehen in der Vorprüfung als «unverändert». Ihr Code ist gleich, aber das Ziel
  oder der Ort, den sie nennen, hat sich geändert. Das zeigte erst der Blick auf die Verweise und die
  entfernten oder geänderten Texte.

## Zählung je Kürzel und Status

| Kürzel | ok | ok, ohne Test | LÜCKE | LÜCKE Test | unklar | Summe |
|---|---|---|---|---|---|---|
| LIB | 76 | 10 | 1 | 0 | 0 | 87 |
| MET | 29 | 16 | 1 | 0 | 0 | 46 |
| ABS | 18 | 6 | 1 | 0 | 0 | 25 |
| MNT | 37 | 2 | 0 | 0 | 0 | 39 |
| ITM | 59 | 10 | 0 | 0 | 0 | 69 |
| **Summe** | 219 | 44 | 3 | 0 | 0 | 266 |

Art der Änderung: 240 `keine`, 20 `Aussehen`, 4 `umgebaut` (LIB-084, MET-046, ITM-002, ITM-026),
1 `verschoben` (MNT-006), 1 `entfällt` (LIB-003, Hilfe-Knopf nach Bauplan 2.6).

Bei den «ok, ohne Test» hatte keine Zeile vor dem Umbau einen Test, der danach weggefallen ist.
