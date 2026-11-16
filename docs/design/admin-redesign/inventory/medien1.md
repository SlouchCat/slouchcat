# Bestandsaufnahme Teil «medien1»: Bibliotheken, Metadaten, Hörbuch-Quellen, Pflege, Titel

Stand: Arbeitsbaum `claude/admin-redesign` auf `main` `55b72c9a` (Release 1.38.0), gelesen am
15. November 2026. Ich habe nur gelesen und nichts ausgeführt. Die Zeilen stehen in `medien1.csv` (266 Zeilen).

**Pfade.** In der CSV und hier sind Pfade ohne Vorsatz relativ zu `web/src/features/admin/`.
`ui/` steht für `web/src/ui/`, `shared/` für `web/src/features/shared/`, `catalog/` für
`web/src/features/catalog/`, `app/` für `web/src/app/` und `e2e/` für `web/e2e/`. Testdateien ohne Pfad
liegen neben ihrer Komponente. Die Texte in «…» stammen wörtlich aus `web/src/locales/de.json`.

**Starthilfe.** Als Starthilfe diente `konzept-verwaltung-ux/…/02-inventar-a-bibliothek.md` (Stand
1.31.0). Jede Zeile ist am heutigen Code geprüft. Zeilennummern und Namen sind neu. Seit 1.31.0 kam dazu:

- **Konzept 22, Freigabeland:** Stufennamen («FSK 12», «PG-13», sonst «ab n») kommen jetzt überall aus
  dem Freigabeland der Sitzung (`useRatingSystem`, `shared/choices.ts`). Das betrifft Bibliotheken,
  Pflege, Titelseite und Editor. «Land» ist eine Auswahl über alle Länder der Server-Tabelle, darunter
  stehen die Stufen. Speichern fragt bei vorhandenen Titeln zurück.
- **Konzept 22, Sprachen:** Keine Sprachliste braucht mehr Deutsch oder Englisch (Vertrag 1.34).
  `ProviderList` sagt jetzt «liefert kein {{languages}}».
- **Konzept 21:** Die Trailer-Tabelle hat neu die Spalte «Auflösung».
- Überall gilt `intlLocale` für Zahlen und Daten, sichtbar etwa bei `de-CH` mit Apostroph.

**Gemeinsame Bausteine** (gelten für alle fünf Seiten, nicht je Zeile wiederholt):

- `ui/AdminPage.tsx`: h1, Hilfeknopf «Hilfe zu dieser Seite», optional «Live, alle N Sekunden»,
  Kopfaktionen, Statusleiste.
- `ui/SaveBar.tsx`: «Ungespeicherte Änderungen» oder «Gespeichert», dazu «Verwerfen» und «Speichern».
- `ui/useUnsavedChanges.ts`: Rückfrage «Seite mit ungespeicherten Änderungen verlassen?» mit «Verwerfen
  und verlassen» oder «Auf der Seite bleiben», dazu `pausePolling`.
- `ui/useSearchSelection.ts`: `?tab=` und `?item=`, immer mit `replace: true`. Mit `normalise`
  ersetzt die Seite einen unbekannten Wert.
- `ui/Dialog.tsx` `ConfirmInputDialog`: Der Knopf wird erst aktiv bei `typed.trim() === expected`
  (Gross- und Kleinschreibung zählt).
- `ui/DataTable.tsx`: Keine Sortierung und keine Suche. Fixe Tabellen haben erst ab 768 px eine
  Mindestbreite von 48rem. Darunter scrollt die Tabelle in ihrem Rahmen.
- `ui/MasterDetail.tsx`: Unter 768 px steht das Detail unter der Liste.
- Der Router (`app/router.tsx:51-80`, `adminSearch`) behält `tab` und `item` nur, wenn sie wie eine
  Id aussehen (Slug, Zahl, ULID, gepunkteter Jobtyp).

**Nicht verwendet.** `CurationParts.tsx` und `PosterTitle.tsx` verwendet keine Seite dieses Teils
(per grep geprüft). Sie gehören zu Sammlungen, Picks und Spotlight.

---

## 1. Bibliotheken `/admin/libraries` (LIB, 87 Zeilen)

### Aufbau heute

- **Kopf** (`libraries/LibrariesPage.tsx:416-428`): h1 «Bibliotheken», Hilfe, «Live, alle 5 Sekunden»
  (nur bei Scan und ohne offenen Dialog), Primärknopf «Bibliothek hinzufügen».
- **Unter dem Kopf:** Skeleton, Ladefehler und Aktionsfehler (rot), das gelbe Entwurfsband mit
  «Weiter bearbeiten» und «Entwurf verwerfen», dazu eine Live-Region (`:429-459`).
- **Reiter** «Alle», «Video», «Hörbücher», «Podcasts» mit Zahlen (`:462-492`). In jedem Reiter steht
  eine Tabelle mit den Spalten Bibliothek, Art, Zustand, Titel · Dateien, Ordner und Metadaten
  (`libraries/libraryColumns.tsx:155-213`).
- **Klappbereich je Zeile** (`libraries/LibraryDetail.tsx:96-166`):
  - Knopf «Erneut scannen», der während des Scans «Scan abbrechen» heisst.
  - Knöpfe «Ordner» und «Vorschau beim Spulen» (nur Video).
  - Menü mit «Umbenennen …», «Standardstufe ändern» (nur Audio), «Verlust akzeptieren» (nur nach
    einem gestoppten Scan) und «Bibliothek löschen …».
  - Links «Metadaten der Bibliothek» (Video) und «Quellen für Hörbücher» (Hörbücher).
  - Fortschritt, Fehlerband, Keyframe-Abdeckung, Vorschau-Zusammenfassung und die Ordnerliste.
- **Dialoge:**
  - `AddLibraryDialog.tsx` mit `shared/LibraryFields.tsx`, `ui/FolderList.tsx`, `ui/ProviderList.tsx`,
    `TrickplayFields.tsx` und der Frage «Entwurf behalten oder verwerfen?».
  - `LibraryFoldersDialog.tsx` mit Ordnerzeilen, «Ändern», «Pfad ändern», «Entfernen» und dem
    Abschnitt «Neue Ordner». Darin liegen die Unterdialoge «Ordner ändern», «Inhalt wirklich
    wechseln?», «Pfad ändern?» und «Ordner wirklich entfernen?».
  - `shared/PathBrowserDialog.tsx` («Ordner auswählen»).
  - `RenameLibraryDialog.tsx`, `DefaultRatingDialog.tsx` und `TrickplayDialog.tsx`.
  - Die Rückfrage «Fehlende Dateien entfernen?» und die Bestätigung «Bibliothek «…» löschen?» mit
    Eintippen.

### Suchparameter, Deeplinks, Verweise

- `?tab=all|video|audiobooks|podcasts`: Ohne Parameter gilt «Alle», ein unbekannter Wert wird ersetzt.
  Klappzustand und Dialoge stehen nicht in der Adresse.
- **Ausgehend:**
  - `/admin/metadata?library={id}` (LIB-035) und `/admin/metadata/audiobooks` (LIB-036).
  - `/admin/jobs?item=keyframes.backfill` (LIB-039) und `/admin/jobs?item=library.delete` (LIB-047).
- **Eingehend:**
  - Seitenleiste mit der Zahl «Bibliotheken, n Titel» (`AdminShellNav.tsx:74-75`).
  - Fusslink von Metadaten (MET-044).
  - Hörbuch-Quellen öffnen denselben Ordner-Dialog (ABS-006).
  - `e2e/profiles.spec.ts:91`: Der Deeplink übersteht die Profilwahl.
- Die gespeicherten Folgen einer Podcast-Bibliothek (`podcastDownloads`) stellt die Seite Podcasts
  ein, nicht diese Seite (anderer Teil).

### Abfragen und Mutationen

**Abfragen:**

- `listLibraries`: alle 5 s, solange ein Scan wartet oder läuft. Bei offenem Dialog pausiert es,
  auch beim Fensterfokus (`:121-138`).
- `getLibraryMetadata` je Video-Bibliothek (Spalte Metadaten).
- `listMetadataProviders` nur bei offenem Dialog, mit `staleTime` von 60 s.
- `getLibrary` für die Schätzung im Vorschau-Dialog.
- `listJobRuns` (`library.delete`, queued und running) alle 2 s, solange ein Löschen offen ist.
- `listBrowseRoots` und `browseServerPaths` (100 je Seite) im Ordnerbrowser.

**Mutationen:**

- Ohne Rückfrage: `scanLibrary`, `cancelLibraryScan`, `runJobType trickplay.backfill`.
- Mit einfacher Rückfrage: `acceptLibraryLoss` (reiht laut Vertrag einen Scan ein) und
  `updateLibraryRoot {path}` («Pfad ändern?»).
- Mit Eintippen: `deleteLibrary` (Name exakt), `removeLibraryRoot` (Pfad exakt) und
  `updateLibraryRoot` mit Inhaltswechsel (Pfad exakt).
- Über Dialog mit «Speichern»: `updateLibrary` (Name, Standardstufe, Vorschau), `createLibrary`,
  `addLibraryRoot`, `updateLibraryRoot {content, providers}`.
- Nach Umbenennen liest die Seite zusätzlich `['libraries']`, `['home']`, `['homeList']` und
  `['library']` neu. Seitenleiste und «Neu in …» folgen damit dem neuen Namen.

### Fallen für den Umbau

- **Entwurf:** Er lebt im Speicher der Seite (`useLibraryDraft`). Band, Verlassen-Frage und die Frage
  bei «Abbrechen» hängen daran, ✕ und Escape behalten ihn ohne Frage. Der Mockup-Knopf «Entwurf
  speichern» suggeriert dauerhaftes Speichern. Das gibt es nicht.
- **Scan-Knopf:** Ein Knopf mit zwei Rollen und 1 s Doppeldruck-Sperre. Der Fokus bleibt darauf.
- **Rückfragen und Fokus:** Rückfragen gehören der Seite, damit eine Zeile, die sich beim Polling
  ändert, die Rückfrage nicht mitnimmt. Nach Aktionen geht der Fokus auf die h1, wenn das Element
  weg ist.
- **Ende eines Löschjobs:** Die Seite beobachtet es nur, solange sie offen ist.
- **«Jetzt erzeugen»:** Es wirkt auf alle Bibliotheken mit «Im Voraus», nicht nur auf diese.
- **Hinweistext:** Bei der Art «Hörbücher» zeigt der Dialog den Video-Hinweis «Ordner dieser
  Bibliothek dürfen Filme oder Serien enthalten, auch gemischt.» (`shared/LibraryFields.tsx:107`).
  Vermutlich ein Fehler. Vor dem Umbau klären.
- **Stufennamen:** Die Stufennamen hängen am Freigabeland (MET-029). Ein alter Wert ausserhalb der
  Stufen bleibt als «ab n» wählbar.
- **«schreibbar»/«nur lesbar»:** Der Chip im Ordner-Dialog entscheidet, ob beim Titel-Löschen «Auch
  die Dateien löschen» geht (ADR 0032).
- **Browser-Zurück:** Weil Reiter den Verlauf ersetzen, verlässt Zurück die Seite. Das ist mit
  `e2e/admin-libraries.spec.ts:616` so getestet.
- **Handy:** Die Seite ist im Phone-Test (390 px) enthalten. Tabellen scrollen in ihrem Rahmen.

### Abgleich mit dem Mockup (D001–D004)

| D | Einstufung | Begründung |
|---|---|---|
| D001 Neue Bibliothek anlegen | visuelle Umgruppierung | Name, Art («Bibliothekstyp») und Ordner («Medienverzeichnis») gibt es, ebenso das Anlegen (LIB-049–062). Der Schalter «Automatisch nach neuen Dateien suchen» und der Knopf «Entwurf speichern» sind Beispiele ohne Basis: Die Überwachung ergibt sich aus dem Dateisystem, der Entwurf wird automatisch und nur im Speicher gehalten. |
| D002 Bibliothek bearbeiten | offene Frage | Name (LIB-045), Stufe (LIB-046, nur Audio) und Scan starten oder abbrechen (LIB-026/027) gibt es. «Keyframe-Vorschau aktivieren» als ein Schalter widerspricht dem Bestand: «Vorschau beim Spulen» hat je Filme und Serien vier Modi, dazu «Speichern» und «Umfang» (LIB-075–083), und der Keyframe-Index ist etwas anderes (LIB-039). «Automatische Suche» hat keine Basis. Die Freigabestufe gilt nur für Audio. |
| D003 Ordner und Scans | visuelle Umgruppierung | Ordner hinzufügen und entfernen sowie Verlust akzeptieren gibt es (LIB-044, -069, -070). «Scanmodus» als Eingabe hat keine Basis, die Überwachungsart wird nur angezeigt (LIB-041). Im Mockup fehlen «Ändern» (Inhalt und Quellen), «Pfad ändern» und das Eintippen des Pfads. |
| D004 Entwürfe & Löschen | offene Frage | Entwurf fortsetzen oder verwerfen sowie Bibliothek löschen gibt es (LIB-013–015, -047). **Widerspruch:** «Dateien auf Datenträger ebenfalls löschen» gibt es beim Löschen einer Bibliothek nicht. Der Text sagt ausdrücklich «Die Dateien auf der Platte bleiben unberührt.» Nur das Löschen eines Titels kennt «Auch die Dateien löschen» (ITM-064). |

**Im Mockup fehlt** (muss bleiben):

- Filter-Reiter nach Art mit Zahlen.
- Spalten Zustand, Ordner und Metadaten.
- Keyframe-Abdeckung mit Link zu den Jobs.
- Fehlerband des letzten Scans mit seinen vier Texten.
- Scanfortschritt.
- Umbenennen als eigener Dialog.
- Löschen mit Eintippen des Namens, Hinweis auf die Sicherung und Beobachtung des Löschjobs.
- Ordner: Ändern (Inhalt, Quellen mit Platz, drei Gewichte), Pfad ändern mit Ordnerbrowser,
  Folgenliste beim Entfernen, Chip «schreibbar».
- Ordnerbrowser mit Tastatur und allen Hinweisen.
- Metadatenquellen je Ordnerkarte.
- Vorschau beim Spulen mit Schätzung, Lösch-Warnung und «Jetzt erzeugen».
- Links zu Metadaten und Hörbuch-Quellen.
- Verlassen-Frage beim Entwurf.

---

## 2. Metadaten `/admin/metadata` (MET, 46 Zeilen)

### Aufbau heute

- **Kopf** (`MetadataPage.tsx:962-982`): h1 «Metadaten», Hilfe und die Auswahl «Geltung» («Standard»
  oder eine Video-Bibliothek, geänderte mit «(geändert)»).
- **Hinweis «Hörbücher: Quellen»** mit «Quellen öffnen» (`:1009-1021`). Er erscheint nur bei der
  Geltung Standard und wenn es eine Hörbuch-Bibliothek gibt.
- **Bereichsliste** `MasterDetail` (`:1022-1029`): Sprachen, Bilder, Bildqualität, Quellen,
  Altersfreigabe, Anwenden. Eine Bibliothek hat nur Sprachen und Bilder. Jeder Bereich zeigt eine
  Zusammenfassung.
- **Detail je Bereich** (`:705-957`):
  - Sprachen: Plätze, Hinweise, Schalter «Sprachen abweichend».
  - Bilder: feste Arten, Kästchen Bildarten, Bildsprache je Art, Schalter «Bilder abweichend».
  - Bildqualität: zwei Mindestbreiten und «Nach Bewertung wählen».
  - Quellen: drei Schalter.
  - Altersfreigabe: Land, Stufen, Hinweis.
  - Anwenden (`ApplySection`, `:258-356`): Was, Wo, «Jetzt anwenden», Warteschlange.
- **Fuss:** zwei Links, die klebende Speicherleiste, Fehlermeldung und Speichermeldungen.
- **Rückfragen:** «Änderungen verwerfen?», «Freigabeland wechseln?» und die Verlassen-Frage.

### Suchparameter, Deeplinks, Verweise

- `?library=<id>` steht für eine Video-Bibliothek und muss eine positive ganze Zahl sein. Eine
  unbekannte Id wird aus der Adresse entfernt.
- `?item=` steht für den Bereich: `languages|images|quality|sources|rating|apply`, bei einer
  Bibliothek nur `languages|images`.
- Ein Wechsel von `?library=` behält die Entwürfe und fragt nicht (`keep: ['library']`).
- **Eingehend:** LIB-035 (`?library=`), der Fusslink der Bibliotheken, der Editor-Link bei
  ausgeschalteten Folgenbildern (ITM-048) und die Seitenleiste.
- **Ausgehend:** `/admin/metadata/audiobooks`, `/admin/integrations` (Fanart-Schlüssel),
  `/admin/jobs` (zweimal) und `/admin/libraries`.

### Abfragen und Mutationen

**Abfragen** (ohne Intervall, ausser beim Anwenden):

- `getMetadataSettings`, `listLibraries` und `listMetadataProviders`.
- `getLibraryMetadata` für jede Video-Bibliothek im Voraus.
- `listJobTypes` alle 10 s, nur im Bereich «Anwenden». Bei ungespeicherten Änderungen pausiert es.

**Mutationen:**

- `updateMetadataSettings` und `updateLibraryMetadata` laufen nacheinander über die eine
  Speicherleiste, der Standard zuerst. Beim ersten Fehler hält die Leiste an.
- Danach liest die Seite `['admin','library-metadata']` und die Sitzung neu.
- Bei geändertem Land und vorhandenen Titeln fragt vorher «Freigabeland wechseln?».
- `applyMetadata` läuft ohne Rückfrage.

### Fallen für den Umbau

- **Entwürfe:** Es gibt einen Entwurf je Geltung (`useDrafts`), alle gespeichert in einem Durchlauf.
  Was während des Speicherns getippt wird, bleibt Entwurf (`restores`, Tests `:610`, `:650`).
- **Abweichungen:** «abweichend aus» parkt die eigene Liste, «an» holt sie zurück. Das ist Zustand
  der Seite, kein Serverwert.
- **Anwenden:** Es nutzt die gespeicherten Einstellungen, nicht den Entwurf, und weist nicht darauf
  hin. Die Wahl von Was und Wo lebt nur im Bereich und geht beim Bereichswechsel verloren.
- **Speicherleiste:** Sie muss direktes Kind der Seitenspalte bleiben, sonst klebt sie nicht
  (`e2e/admin-metadata.spec.ts:170`, WCAG 2.4.11).
- **Fehlermeldung:** Der Text der Speicherfehlermeldung setzt den Namen in Anführungszeichen. Die
  `de-DE`-Fassung beachten.

### Abgleich mit dem Mockup (D006–D012)

| D | Einstufung | Begründung |
|---|---|---|
| D006 Metadaten pro Bibliothek | visuelle Umgruppierung | Es gibt die Geltung je Video-Bibliothek mit den Schaltern «Sprachen abweichend» und «Bilder abweichend». «aus» entspricht «Serverstandard übernehmen». Gespeichert wird über die gemeinsame Speicherleiste (MET-003, -013, -020, -040). Nur Video-Bibliotheken haben eine Geltung. |
| D007 Textsprachen | bestehende Funktion | Plätze mit Tausch, «Weitere Sprache» und Entfernen gibt es (MET-010). Der Schalter «Sprachfallback verwenden» ist ein Beispiel ohne Basis: Der Rückfall gilt immer (MET-012). |
| D008 Poster, Hintergründe und Logos | bestehende Funktion | Die Bildsprache je Poster, Hintergrund und Logo gibt es (MET-018), ebenso die Kästchen Logo, Staffelposter und Folgenbild (MET-016). Poster und Hintergrund sind fest «immer». |
| D009 Bildqualität | bestehende Funktion | Mindestbreite Poster und Hintergrund mit festen Stufen sowie «Nach Bewertung wählen» gibt es (MET-021–023). Nur im Standard. |
| D010 Quellen und Inhalte | offene Frage | Die drei Schalter gibt es (MET-025–027). «Anbieter prüfen» gibt es nur auf Integrationen (Schlüssel prüfen, anderer Teil). Offen ist, ob der Knopf hierher gespiegelt wird. |
| D011 Altersfreigaben | bestehende Funktion | Land und Speichern gibt es (MET-029–032). Der Knopf «Freigaben neu lesen» ist ein Beispiel ohne Basis: Das Neulesen folgt dem Landwechsel nach der Rückfrage. |
| D012 Metadaten anwenden | bestehende Funktion | Was («Daten»), Wo («Umfang»), «Jetzt anwenden» und der Link zu den Jobs gibt es (MET-034–038). «Bereits vorhandene Daten aktualisieren» ist ein Beispiel ohne Basis. |

**Im Mockup fehlt** (muss bleiben):

- Geltungsauswahl im Kopf mit «(geändert)».
- Hinweis «Standard hat ungespeicherte Änderungen …».
- Hinweise zu Anbietern ohne Sprache und zum YouTube-Kontingent.
- «Art, Banner und Disc gibt es bewusst nicht».
- Anzeige der Stufen des Landes.
- Bestätigung «Freigabeland wechseln?».
- Anzeige der Warteschlange und des letzten Laufs.
- Speichern in mehreren Geltungen mit Teilerfolg.
- Verwerfen mit Rückfrage.
- Verlassen-Frage.
- Fusslinks.
- Einstieg «Hörbücher: Quellen».

---

## 3. Hörbücher: Quellen `/admin/metadata/audiobooks` (ABS, 25 Zeilen)

### Aufbau heute

`AudiobookSourcesPage.tsx` besteht aus vier Abschnitten untereinander (`:182-189`):

1. **«Reihenfolge je Ordner»:** je Hörbuch-Ordner Pfad, Bibliothek und Quellenkette, dazu «Quellen
   ändern».
2. **«Einstellungen»:**
   - Region von Audible und Adresse von Audnexus.
   - Zeile «Schlüssel für Google Books» mit Link.
   - Automatisch zuordnen und Längentoleranz.
   - h3 «Vereinheitlichen» mit zwei Schaltern.
   - Hinweis zu den Nutzungsbedingungen von Audible.
   - Eigene Speicherleiste.
3. **«Stand der Bibliothek»:** sechs Kacheln, «Jetzt alle ohne Zuordnung zuordnen», Link zu den
   Jobs und h3 «Zustand der Quellen».
4. **«Prüfen»:** Tabelle mit Stift zum Metadaten-Editor.

Die Seite öffnet zwei Dialoge: `libraries/LibraryFoldersDialog.tsx` und `editor/MetadataEditor.tsx`.

### Suchparameter, Deeplinks, Verweise

- Die Seite hat keine Suchparameter. Die Route hat kein `validateSearch` (`app/router.tsx:546-553`).
- Die Seitenleiste markiert «Metadaten», weil der Pfad mit `/admin/metadata` beginnt. Einen eigenen
  Eintrag gibt es nicht.
- **Eingehend:** nur MET-007 und LIB-036.
- **Ausgehend:** `/admin/integrations` und `/admin/jobs`.

### Abfragen und Mutationen

- **Abfragen:**
  - `getAudiobookSources` alle 5 s, solange `running` gilt.
  - `listAudiobookReview` ebenso, mit einem Lauf mehr am Ende.
  - `listLibraries` für den Ordner-Dialog.
- **Mutationen:**
  - `updateAudiobookSources` (PATCH, nur geänderte Felder) ohne Rückfrage. «Verwerfen» fragt
    ebenfalls nicht zurück.
  - `matchAllAudiobooks` ohne Rückfrage.
  - Ordner und Editor laufen über deren Operationen.

### Fallen für den Umbau

- **Speicherleiste:** Sie steht mitten auf der Seite (Abschnitt Einstellungen), nicht unten. Enter in
  Audnexus speichert wie die Leiste.
- **«Quellen ändern»:** Es öffnet den Dialog der ganzen Bibliothek, nicht nur des Ordners. Damit
  hängt die Seite direkt an der Bibliotheksseite.
- **Fokus nach dem Editor:** eigener Stift, sonst der Stift der nächsten Zeile, sonst die h1 (`:650-676`).
- **Toleranz:** Die Auswahl bietet 1, 3 und 5 % plus den gespeicherten Wert. Der Server erlaubt 1–10.
- **Handy:** Die Seite fehlt in `e2e/admin-phone.spec.ts`. Das Verhalten auf dem Handy ist nicht belegt.

### Abgleich mit dem Mockup (D005)

| D | Einstufung | Begründung |
|---|---|---|
| D005 Hörbuch-Quellen | visuelle Umgruppierung | Region (ABS-007), Audnexus (ABS-008), Automatisch zuordnen (ABS-010), Toleranz (ABS-011), beide Vereinheitlichen-Schalter (ABS-012/013), Speichern (ABS-015) und «Gesamten Abgleich starten» (ABS-018) gibt es. «Google Books konfigurieren» ist heute nur ein Link zu Integrationen (ABS-009). Das Mockup hängt die Seite unter Bibliotheken, der Code unter Metadaten: Den Ort muss der Koordinator festlegen. |

**Im Mockup fehlt** (muss bleiben):

- «Reihenfolge je Ordner» mit «Quellen ändern».
- Hinweis zu den Nutzungsbedingungen von Audible.
- Kacheln «Stand der Bibliothek».
- «Zustand der Quellen» mit Pause bis.
- Die Liste «Prüfen» mit Gründen, Kandidaten und Stift zum Editor.
- Polling während einer Zuordnung.
- Verlassen-Frage.

---

## 4. Pflege `/admin/maintenance` (MNT, 39 Zeilen)

### Aufbau heute

- **Kopf** (`MaintenancePage.tsx:455-459`): h1 «Pflege» und Hilfe. Es gibt keine Kopfaktion und
  keine Statusleiste.
- **Reiter «Problemarten»** (`:371-465`): neun Reiter mit Zahl, nämlich sechs Problemarten
  (`maintenance/IssueTab.tsx`), «Duplikate» (`maintenance/DuplicatesTab.tsx`), «Verwaiste
  Benutzerdaten» (`maintenance/RelinkTab.tsx`) und «Entfernte Titel» (`maintenance/RemovedTab.tsx`).
- **Reiter einer Problemart:** Hinweis, Live-Region mit dem Ergebnis und eine Werkzeugleiste mit den
  Aktionen der Art, dem Auswahlzähler und «Alle auswählen». Darunter steht je Bibliothek ein
  Klappbereich mit Tabelle. Am Ende stehen «x von y» und das Nachladen.
- **Dialoge:** `MatchDialog.tsx` und `maintenance/RatingDialog.tsx`, dazu eine gemeinsame Rückfrage
  der Seite (`:488-499`).

### Suchparameter, Deeplinks, Verweise

- `?tab=` steht für die Reiter: `unmatched|no_images|unrated|orphaned|unplayable|rating_mismatch|
  duplicates|relink|removed`. Ohne Parameter gilt `unmatched`, ein unbekannter Wert wird ersetzt.
- Auswahl, Ergebnis, Klappzustand, Ziel je Duplikat-Gruppe und Ausschlüsse bei Relink bleiben beim
  Reiterwechsel im Speicher, nicht in der Adresse.
- **Ausgehend:** `/admin/items/{id}` aus der Titelspalte, den Duplikaten und der Kopie.
- **Eingehend:** Seitenleiste mit der Zahl «n Titel brauchen Pflege». Die Zahl ist die Summe aus
  unmatched, no_images, unrated, orphaned und unplayable (openapi `maintenanceOpen`).
- Die Titelseite markiert «Pflege» in der Seitenleiste.

### Abfragen und Mutationen

**Abfragen** (ohne Intervall, kein Neuladen bei Fokus, solange ein Dialog offen ist):

- `listMaintenanceIssues` je Art mit `limit=1` für die Zahl, dazu seitenweise 100 je Reiter.
- `listDuplicates`, `listRelinkPairs` (höchstens 200) und `listRemovedItems` (`limit 200`).
- `listLibraries` für die Namen, ohne Scan-Polling.

**Mutationen** (nach jeder liest die Seite alles unter `['admin','maintenance']` neu):

- Ohne Rückfrage: `refreshItem` je Titel nacheinander («Aktualisieren», «Erneut prüfen»),
  `restoreRemovedItem` («Wieder aufnehmen») und `setItemMatch` (Klick auf einen Treffer).
- Mit Rückfrage: `cleanupItems` («Bereinigen», «Entfernen»), `mergeItems` («Zusammenführen») und
  `relinkItems` («Umhängen»).
- Über Dialog: `setItemsRating` («Zuweisen»).

### Fallen für den Umbau

- **Rückfrage der Seite:** Sie wirkt auch, wenn die Zeilen inzwischen weg sind (`MaintenancePage.test.tsx:324`).
- **Fokus:** Nach Entfernen geht er immer auf die h1, sonst nur, wenn er verloren ist. Bei
  «Umhängen» bleibt er auf dem Knopf.
- **Auswahl:** Sie zählt nur Geladenes. «Alle auswählen» erfasst keine nicht geladenen Seiten.
- **Auswahlschlüssel:** Bei «Nicht abspielbar» je Datei, sonst je Titel.
- **«Erneut prüfen»:** Es aktualisiert den ganzen Titel. Laut Vertrag prüft `refreshItem` dabei jede
  Datei neu.
- **Grenzen:** Relink und Entfernte zeigen höchstens 200 Einträge mit «x von y», ohne Weg zum Rest.
- **Keine Suche:** Es gibt kein Suchfeld und keinen Filter, nur Gruppen je Bibliothek.
- **«Freigabe zuweisen»:** Es gibt sie nur im Reiter «Altersfreigabe fehlt», ohne Vorwahl.

### Abgleich mit dem Mockup (D013–D018)

| D | Einstufung | Begründung |
|---|---|---|
| D013 Fehlende oder falsche Zuordnung | offene Frage | «Titel zuordnen» (MNT-017, -036, -037) und «Metadaten erneuern» (= «Aktualisieren», MNT-018) gibt es. «Suche nach Titel» und der Filter «Bibliothek» haben keine Basis; heute gruppiert die Seite nach Bibliothek (MNT-013). «Auswahl ignorieren» hat keine Basis. Offen ist, ob eine neue Suche gebaut wird (Nicht-Ziel-Regel 11 beachten). |
| D014 Unvollständige Metadaten | visuelle Umgruppierung | Das entspricht dem Reiter «Ohne Bilder» mit «Aktualisieren» (MNT-018). Ein Filter «Problem» über Bilder, Texte und Medieninformationen hat keine Basis, denn es gibt nur die festen Problemarten als Reiter. «Automatisch nachholen» ist ein Beispiel ohne Basis. |
| D015 Duplikate zusammenführen | bestehende Funktion | «Behalten» (MNT-028), die Gegenüberstellung mit Pfaden und Grössen (MNT-027) und «Zusammenführen» mit Rückfrage (MNT-029) gibt es. «Benutzerdaten übernehmen» als Option ist ein Beispiel ohne Basis, denn Fortschritt und Dateien wandern immer mit. |
| D016 Verwaiste Daten neu zuordnen | bestehende Funktion | Die Liste der Paare mit Auswahl und «Umhängen» gibt es (MNT-030–032). Das Feld «Vorgehen» ist ein Beispiel ohne Basis. |
| D017 Entfernte Titel wiederherstellen | bestehende Funktion | «Wieder aufnehmen» gibt es je Zeile (MNT-034), ohne Mehrfachauswahl. «Endgültig bereinigen» und «Löschvorgänge kontrollieren» sind Beispiele ohne Basis. |
| D018 Altersfreigabe in Auswahl ändern | bestehende Funktion | «Freigabe zuweisen» für die Auswahl gibt es (MNT-019, -035), nur im Reiter «Altersfreigabe fehlt». |

**Im Mockup fehlt** (muss bleiben):

- Reiter «Verwaist» mit «Bereinigen».
- «Nicht abspielbar» mit «Erneut prüfen» und «Entfernen».
- «Freigabe abweichend» als reine Liste.
- Zahlen je Reiter und in der Seitenleiste.
- Gruppen je Bibliothek mit Klappen.
- Auswahlzähler und «Alle auswählen».
- Nachladen beim Scrollen.
- Die Liste «Verwaiste Benutzerdaten» mit Benutzerdaten-Summe und Kopien.
- Links aus jeder Zeile zur Titelseite.
- Rückfragen mit ihren Texten.

---

## 5. Titelseite `/admin/items/$itemId`, Metadaten-Editor und Titel löschen (ITM, 69 Zeilen)

### Aufbau heute

**Titelseite** (`AdminItemPage.tsx`):

- Kopf mit h1 = Titel und Chip Art und Jahr. Aktionen: «Zuordnen» (Film, Serie), «Aktualisieren»,
  «Zur Detailseite».
- Live-Region und die Reiter «Metadaten», «Bilder», «Dateien», «Trailer».
- **Metadaten** (`item/MetadataTab.tsx`, `item/facts.tsx`): Faktenliste, Klappbereiche «Herkunft je
  Feld», «Freigabe» (mit «Speichern») und «Originalsprache» (mit «Speichern»).
- **Bilder** (`item/ImagesTab.tsx`): Tabelle je Art mit «Andere wählen» als Dialog und «Sperre aufheben».
- **Dateien** (`item/FilesTab.tsx`): Tabelle mit Klappzeilen.
- **Trailer** (`item/TrailersTab.tsx`): Suchzustand, Tabelle, «Hinzufügen» als Dialog, «Löschen» mit
  Rückfrage.

**Metadaten-Editor** (`editor/MetadataEditor.tsx`, Dialog `EditorDialog`):

- Kopf mit Bild, Titel, «Art · Bibliothek», «In der Verwaltung öffnen» und «Löschen …».
- **Reiter Bilder oder Cover** (`editor/ImagesEditorTab.tsx`): Bildart-Chips, aktuelles Bild,
  Hochladen, «Zurück auf automatisch», «Neu abfragen», Filter und Raster.
- **Reiter Metadaten** (`editor/MetadataEditorTab.tsx`, `editor/editorModel.ts`): Felder mit
  Herkunft und Schloss, Genres, Freigabe, Originalsprache, «Speichern» und «Verwerfen».
- **Reiter Zuordnen** (`editor/MatchRefreshTabs.tsx`, `editor/BookMatchTab.tsx`): Film und Serie
  über `MatchSearch`, Hörbuch mit Quelle, Suche, Treffern, «Was übernommen wird» und Übernehmen.
- **Reiter Aktualisieren:** «Jetzt aktualisieren» mit «Sperren aufheben». Beim Hörbuch «Neu lesen»
  und «Neu holen».

**Titel löschen** (`deletion/DeleteItemDialog.tsx`): Vorschau, Wahl «Nur aus Slouchcat entfernen»
oder «Auch die Dateien löschen», betroffene Dateien, Eintippen, «Läuft gerade» und Bestätigen.

### Suchparameter, Deeplinks, Verweise

- **Titelseite:** `?tab=metadata|images|files|trailers`. Ohne Parameter gilt `metadata`. Ein
  unbekannter Wert oder «Bilder» bei einer Folge wird ersetzt, aber erst wenn der Titel geladen ist.
  Klappzustand und Entwürfe stehen nicht in der Adresse.
- **Editor und Löschen:** kein Zustand in der Adresse, beides sind Dialoge.
- **Eingehend:**
  - Pflege (Titel, Duplikate, Kopie), Statistiken (`stats/TopTitles.tsx`), Editor-Link und die
    Detailseite «Verwalten» (nur für Arten ohne Editor, `catalog/DetailActions.tsx:327-329`).
  - Die Seitenleiste markiert «Pflege» (`sections.ts:47-53`). Eine Titelsuche in der Verwaltung gibt
    es nicht.
- **Editor erreichbar:**
  - Über den Stift auf Postern und Karten (`catalog/EditMetadata.tsx`, Arten Film, Serie, Staffel,
    Folge, Hörbuch, Admin mit Hauptprofil) und aus ABS-024.
  - **Nicht** von der Titelseite aus.
- **Löschen erreichbar:** aus dem Editor und von der Detailseite (`catalog/DetailActions.tsx:330-345`).

### Abfragen und Mutationen

**Abfragen:**

- `getAdminItem` ohne Intervall. Bei Entwurf oder offener Rückfrage pausiert das Neuladen beim Fokus.
- `getMetadataSettings`: auf der Titelseite erst beim Aufklappen der Originalsprache, im Editor für
  Film, Serie und Hörbuch.
- `listItemImageCandidates`.
- `listAdminGenres`.
- `listMetadataProviders` und `getAudiobookSources` für die Buchsuche.
- `searchMetadata` und `getMatchPreview`.
- `getItemDeletion` bei jedem Öffnen frisch.

**Mutationen:**

- Ohne Rückfrage:
  - `refreshItem` (auch mit `unlockAll` und `fromProvider`).
  - `setItemMatch`.
  - `setItemImage` und `clearItemImage`.
  - `uploadItemImage`.
  - `createItemTrailer` über Dialog.
  - `setItemRating` und `setItemOriginalLanguage` über «Speichern».
  - `updateItemMetadata`.
- Mit Rückfrage: `deleteItemTrailer`.
- Mit Vorschau und Eintippen: `deleteItem`, das Eintippen nur bei mehr als einer Datei und ohne
  Gross/Klein. Danach liest die App **alle** Abfragen neu.

### Fallen für den Umbau

- **Zwei Orte für dieselben Felder:** Freigabe und Originalsprache gibt es auf der Titelseite und im
  Editor. Bilder wählen gibt es zweimal, Hochladen nur im Editor. Aktualisieren mit «Sperren
  aufheben» gibt es nur im Editor. Hörbuch-Zuordnung, Texte bearbeiten und Löschen gibt es nur im
  Editor.
- **Bildarten:** Die Titelseite hat feste Bildarten (Film und Serie: Poster, Hintergrund, Logo;
  Staffel: Poster). Der Editor nimmt die Liste des Servers, also auch Folgenbild und Cover.
- **Entwürfe der Titelseite:** Sie überleben den Reiterwechsel und lösen die Verlassen-Frage aus.
- **Entwurf des Editors:** Er gilt nur für den Reiter Metadaten. Schliessen fragt «Ungespeicherte
  Änderungen verwerfen?».
- **Speichern im Editor:** Es sind drei Anfragen nacheinander. Scheitert eine, bleibt der Entwurf und
  der Titel wird neu gelesen.
- **Zwei Arten des Eintippens:** Bibliothek löschen verlangt exakt (Gross/Klein zählt), Titel löschen
  ohne Gross/Klein.
- **Falscher Ortsname:** Der Hinweis «Nur aus Slouchcat entfernen» nennt «Wartung › Entfernte Titel».
  Die Seite heisst «Pflege».
- **Ungenutzter Schlüssel:** `admin.itemDelete.leftover` steht in `de.json`, wird aber von keiner
  Komponente genutzt (grep).
- **Editor auf dem Handy:** Er füllt den ganzen Bildschirm, die Felder stehen einspaltig.
- **Tastatur:** `useSpatialHold` hält die Pfeiltasten der Seite dahinter an.

### Abgleich mit dem Mockup

Keine der Zeilen D001–D018 zeigt die Titelseite, den Editor oder das Löschen eines Titels. Nur
D004 nennt «Dateien … löschen», aber für Bibliotheken (Widerspruch, siehe dort).

**Im Mockup fehlt** (muss bleiben): **alle 69 ITM-Zeilen**, insbesondere:

- Fakten und Herkunft je Feld.
- Eigene Freigabe und Originalsprache.
- Bildwahl je Art.
- Dateien mit Prüffehlern.
- Trailer hinzufügen und löschen mit Auflösung.
- Der ganze Editor: Felder mit Schloss, Hochladen, Buch-Zuordnung mit Vergleich, Aktualisieren der
  Bücher.
- Titel löschen mit Vorschau, Dateiwahl, Sperre bei laufender Wiedergabe und 409 «geändert».

---

## Liste «unklar»

1. **ABS-010 «Alle ohne Zuordnung»:** Der Hinweis sagt «Einmal für die ganze Bibliothek, danach wie
   «Neue Hörbücher»». Der Vertrag (`AudiobookAutoMatch`) sagt «additionally every book without match
   (the default)». Ob Speichern einen Lauf auslöst und ob der Wert danach umspringt, zeigt der Client
   nicht. Server nicht gelesen.
2. **ABS-011 Toleranz:** Angeboten werden 1, 3 und 5 %, der Server nimmt 1–10. Ob 2, 4 und 6–10 nur
   per API gewollt sind, steht nirgends.
3. **MET-036 Anwenden mit Ungespeichertem:** Der Lauf nutzt den gespeicherten Stand. Ob ein Hinweis
   oder eine Sperre fehlt, ist nicht dokumentiert.
4. **ITM-011 «Bibliothek»:** Die Titelseite zeigt die Id als Zahl, der Editor den Namen. Ob das
   gewollt ist, ist nicht belegt.
5. **ITM-018 Bildarten:** Die Titelseite nutzt eine feste Tabelle (`item/ImagesTab.tsx:24-29`), der
   Editor die Liste des Servers. Ob die Titelseite absichtlich weniger zeigt, ist offen.
6. **MNT-030 und MNT-033 Mengen:** Höchstens 200 Einträge mit «x von y», ohne Weg zum Rest.
7. **LIB-052 Hinweistext bei Hörbüchern:** Vermutlich ein Fehler (Video-Text). Vor dem Umbau klären.
8. **ITM-064 Ortsname «Wartung › Entfernte Titel»:** Der Text weicht vom Seitennamen «Pflege» ab.
9. **Mockup D002, «Keyframe-Vorschau aktivieren»:** Gemeint ist vermutlich «Vorschau beim Spulen»
   (Trickplay). Der Keyframe-Index ist eine andere Funktion mit eigenem Job.
