# Konzept: Seitenmuster der Verwaltungsseiten (alle Seiten unter `/admin/*`)

- **Status: Fassung 2 (1. November 2026), Phase 1 «Grundmuster».** Fassung 1 hat der
  OMC-Kritiker geprüft (1 Blocker, 9 Major, 8 Minor; Abschnitt 11); Fassung 2 arbeitet alle ein. Marco hat am 1. November 2026
  gesagt: «Im Prinzip möchte ich alle Seiten in der Verwaltung überarbeiten. Es ist mir alles
  irgendwie zu gross und zu unübersichtlich.» Dieses Papier legt ein **Seitenmuster** fest, das
  alle bestehenden Verwaltungsseiten umbaut und für **jede künftige Verwaltungsseite** gilt.
  Zuerst wählt Marco das Grundmuster (drei Varianten, Abschnitt 6); danach folgen je Seite drei
  Vorschläge, gruppenweise (Abschnitt 8).
- **Datum:** 1. November 2026.
- **Verankert gegen:** `main` **`5e0e63d`** («Merge pull request #61», Konzept 11 als Papier),
  Vertragsfassung **1.6.0**. Konzept 11 reserviert Frage 17, ADR 0019, Migration 00031 und
  Vertrag 1.7.0. Dieses Konzept nimmt darum **Frage 18** und, falls nötig, **ADR 0020**.
- **Was nicht dazugehört:** die Übersicht `/admin` (bleibt nach Konzept 11, Abschnitt 11) und
  alle Zuschauerseiten mit der Startseite (unberührt).
- **Umsetzung:** erst nach Marcos ausdrücklichem Wort, **zusammen mit Konzept 11 und den
  weiteren Konzepten, die noch folgen** (Marco, 1. November 2026). Bis dahin ändert dieses Papier
  keine Zeile Code.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).

## 1. Ausgangslage

### 1.1 Warum die Seiten gross wirken

Die Verwaltung benutzt heute dieselben Masse wie die Zuschauerseiten, die für Sofa-Abstand und
Fernbedienung gemacht sind. Gemessen im Code (`main` `5e0e63d`):

| Nr. | Befund | Ursache im Code |
|---|---|---|
| P-1 | Seitentitel 60 px (ab 768 px), fett. | Jede Seite nutzt `Text variant="display"` (`ui/Text.module.css:44`, `--sc-font-size-display` 3,75 rem). |
| P-2 | Unter jedem Titel ein Erklärsatz in 20 px, darunter 32 px Abstand. | Kopf `Text` Standard `body` 1,25 rem; `.header { margin-bottom: space-8 }` (`AdminTablePage.module.css:6`, `LibrariesPage.module.css:5` u. a.). |
| P-3 | Knöpfe und Felder 44 px hoch, Schrift 16 px. | `--sc-size-control-height` 2,75 rem (`ui/Button.module.css:6`, `ui/Field.module.css:16`). |
| P-4 | Panels mit 24 px Innenabstand, 20 px Lücke, 20 px Radius. | `ui/Panel.module.css:7`. |
| P-5 | Abschnitte 40 bis 48 px auseinander, Abschnittstitel 24 px. | `AdminHomePage.module.css`, `JobsPage.module.css`, `AdminData.module.css` (`.blocks` gap space-10). |
| P-6 | Seitenrand bis 64 px ab 1440 px. | `--page-gutter` in `ui/global.css`. |
| P-7 | Lange Seiten, die alles untereinander zeigen: Hintergrundjobs (929 Zeilen Code), Metadaten (852), Spotlight (779), Integrationen (694 + 449 Webhooks). | Jede Seite baut ihren Aufbau selbst; es gibt kein gemeinsames Muster für Kopf, Abschnitte, Einstellungen und Zeilenaktionen. |
| P-8 | Jede Seite hat eigene CSS-Module für denselben Kopf und dieselben Abstände (16 Module). | `AdminTablePage`, `AdminData`, `Curation`, `JobsPage`, `MetadataPage` usw. |

### 1.2 Bestand

18 Seiten der Verwaltungsleiste (`sections.ts:39-61`, davon zwei unter `/settings/*`) und die
Unterseite Titel `/admin/items/$itemId`, alle in Abschnitt 7 mit ihrem vollen Inhalt aufgeführt:

| Gruppe | Seiten |
|---|---|
| Bibliothek | Bibliotheken, Podcasts, Pflege (mit Unterseite Titel `/admin/items/$itemId`), Metadaten |
| Konten | Konten (mit Einladungen), Profile (`/settings/profiles`), Geräte, Streams |
| Kuratierung | Spotlight, Cat's Empfehlungen, Sammlungen, Startseitenreihen, Ankündigungen |
| System | Hintergrundjobs, Integrationen (mit Webhooks), Benachrichtigungen (`/settings/notifications`), Statistik, Sicherung |

Dazu die Weiterleitung `/admin/access` → `/admin/accounts` (ADR 0014), die bleibt.

## 2. Was gilt

| Quelle | Inhalt | Folge |
|---|---|---|
| Spec A-2 | Verwaltung «an einem Ort», mit derselben Sorgfalt wie die Zuschauerseite. | Gleiche Tokens, gleiche Komponenten, nur dichtere Masse. |
| Konzept 11, 11.1 | Verwaltungsleiste links auf allen `/admin/*`-Seiten (Band-Titel, Zahlen, Pillen); am Handy ein Blatt. | Jede Seite dieses Konzepts steht neben der Verwaltungsleiste; die horizontale `AdminNav` ist weg. |
| Regel 8 (`CLAUDE.md`) | Jede neue Seite hat einen Playwright-Tastaturtest. | Alle umgebauten Seiten haben einen; die bestehenden Specs werden angepasst, keiner wird gelöscht. 11 Specs prüfen den Fokus nach dem Entfernen (`toBeFocused`); dieses Verhalten bleibt (ALL-2). |
| Regel 9 (`CLAUDE.md`) | Farben nur aus Tokens, Texte nur aus `locales/{de,en}.json`, Admin und Zuschauer nutzen dieselben Komponenten aus `web/src/ui`. | Die kompakte Dichte entsteht über Token-Werte im Verwaltungsgerüst, nicht über eigene Admin-Kopien der Komponenten (Abschnitt 5). |
| Regel 11 (`CLAUDE.md`) | Keine Nicht-Ziele, nichts vorziehen. | Das Seitenmuster ordnet um; es baut **keine neue Funktion**. |
| Marco, 1. November 2026 | «Es darf keine Seite oder Funktion oder Übersicht verloren gehen. Inhaltlich muss alles übernommen werden.» | Abschnitt 7 ist die **Erhaltungsliste**; sie ist Abnahme-Checkliste für jeden Mockup und für den Bau (Regel E-1 bis E-3). |

## 3. Ziel

Jede Verwaltungsseite beantwortet auf den ersten Blick: **Wo bin ich, wie ist der Zustand, was
kann ich tun?** und zeigt die eigentliche Arbeit ohne Scrollen durch Erklärtexte.

- **Kleiner:** dieselbe Information auf deutlich weniger Höhe (Titel, Abstände,
  Steuerelemente).
- **Übersichtlicher:** jede Seite folgt demselben Muster; wer eine Seite kennt, kennt alle.
- **Vollständig:** alles, was heute da ist, bleibt erreichbar (Abschnitt 7).
- **Gilt weiter:** neue Verwaltungsseiten werden nach diesem Muster gebaut (Abschnitt 4,
  Regeln S-1 bis S-12).

**Gemessen** wird beim Bau an drei Seiten (Hintergrundjobs, Integrationen, Konten) bei 1440 px
Fensterbreite: Höhe bis zur ersten Arbeitszeile (Tabelle, Liste oder Formular) heute und neu.
Ziel: höchstens die Hälfte. Der Ausgangswert wird vor dem Bau gemessen und in
`docs/perf/verwaltungsseiten.md` abgelegt.

## 4. Seitenmuster (gilt für jede Variante und jede künftige Seite)

- **S-1 Gerüst:** `/admin/*`: Verwaltungsleiste links (Konzept 11), App-Kopf oben, Inhalt
  rechts mit höchstens 75 rem Breite (`--sc-size-admin-content-max`, neu), Seitenrand 24 bis
  32 px, kompakte Dichte (Abschnitt 5). `/settings/*` (Profile, Benachrichtigungen): App-Seitenleiste
  wie nach Konzept 11, 11.1 Punkt 4 und O-3, **heutige Dichte**, Muster S-2 bis S-13; diese Seiten
  sehen auch Konten ohne Verwaltungsrecht.
- **S-2 Seitenkopf in einer Zeile:** h1 (24 px, 600), daneben ein kleiner Knopf «Hilfe zu dieser
  Seite» (Symbol `info`), der den heutigen Erklärsatz als Aufklapptext zeigt; rechts kleiner
  Zustand (nur bei Seiten mit dauerndem Abfragen «Live, alle 5 Sekunden»; bei Abfragen unter
  Bedingung, etwa Bibliotheken nur beim Scan, erscheint er nur, solange abgefragt wird) und die
  Hauptaktion(en) der Seite. **Kein Einleitungssatz** im Fliesstext. S-2 gilt nur für den
  Einleitungssatz der Seite; Hinweise mit Zustand oder Warnung (zum Beispiel
  `MaintenancePage.tsx:283`, `IntegrationsPage.tsx:314`, Spotlight «warum die Reihe fehlt») bleiben
  sichtbar. Schalter, die heute nur mit einem Formular gespeichert werden (Spotlight «Aktiv»),
  bleiben in ihrem Formular und wandern nicht als Sofortschalter in den Kopf.
- **S-3 Eine Hierarchie:** h1 → Abschnittstitel h2 (15 px, 600) → Gegenstandstitel h3 (14 px,
  600, zum Beispiel ein Schlüssel, eine Bibliothekskarte, eine Bildart; heute
  `IntegrationsPage.tsx:188`, `LibraryCard.tsx:164`, `MetadataPage.tsx:659`). Tiefer nicht.
- **S-4 Zustand zuerst:** Hat eine Seite Zählwerte (Jobs, Pflege, Streams), stehen sie als
  schmale Zustandsleiste (`StatTile` kompakt, 56 px hoch) direkt unter dem Kopf.
- **S-5 Tabellen dicht:** Zeilen 40 px, Kopf 12 px Grossbuchstaben gedämpft; lange Texte
  (Pfade, Fehler) einzeilig gekürzt mit vollem Text im Tooltip bzw. Detail.
- **S-6 Zeilenaktionen:** höchstens zwei sichtbare Knöpfe je Zeile (die häufigsten); der Rest in
  einem Menü «Weitere Aktionen» (neuer Baustein `ActionMenu`, react-aria `Menu` mit Abschnitt
  «Gefahr», `aria-label` mit Zeilenname; der heutige `MenuButton` ist ein Auswahlmenü mit Häkchen
  und taugt dafür nicht, `ui/Menu.tsx:30-60`). Gefährliche Aktionen stehen im Menü unten, rot, und
  behalten ihre Rückfrage. Dafür werden `ConfirmDialog` und `FormDialog` steuerbar
  (`isOpen`/`onOpenChange`; heute öffnen sie nur über `trigger`, `ui/Dialog.tsx:15-26`). Fokus nach
  der Aktion auf den Menüknopf, beim Wegfall der Zeile auf den Seitenkopf (wie heute). Die Pause
  des Abfragens bei offenem Dialog bleibt (`JobsPage.tsx:122`, `StreamsPage.tsx:57`).
- **S-7 Einstellungen als Zeilen:** Bezeichnung und kurze Hilfe links, Steuerelement rechts,
  eine Einstellung je Zeile. Mehrere Einstellungen eines Bereichs speichern gemeinsam
  (Speicherleiste unten wie heute bei Metadaten), einzelne Geheimnisse (Schlüssel) speichern
  einzeln.
- **S-8 Kartenraster:** gleiche Höhe je Reihe; wo Karten im Raster stehen, keine allein in
  einer Reihe, wenn ihre Zahl das zulässt (G-2 aus Konzept 11). Listen und Tabellen sind davon
  nicht betroffen.
- **S-9 Jede Angabe einmal** (G-8 aus Konzept 11).
- **S-10 Leer, Laden, Fehler einheitlich:** `Skeleton` in der Form des Inhalts, `Notice` mit
  «Erneut versuchen», Leerzustand als eine Zeile mit Hauptaktion.
- **S-11 Tastatur:** wie heute (Regel 8); Menüs und Klappbereiche mit Tastatur bedienbar,
  sichtbarer Fokus, keine Fokusfalle ausser in offenen Dialogen.
- **S-12 Eine Quelle für das Muster:** neue Bausteine in `web/src/ui` mit Story (Abschnitt 5.2);
  die 16 seiteneigenen CSS-Module für Kopf und Abstände entfallen zugunsten dieser Bausteine.
- **S-13 Nichts Getipptes geht verloren:** Wechselt man Reiter (C) oder Gegenstand (B) oder
  verlässt die Seite mit ungespeicherten Änderungen, bleiben sie erhalten oder es kommt eine
  Rückfrage (Router-`useBlocker`; heute schützt keine Seite davor, und `ui/Tabs.tsx:41` hängt
  inaktive Reiter aus). Abfragen pausiert, solange ein Formular geändert ist.
- **S-14 Adressen:** eine Auswahl, die man verlinken will (Variante B: Gegenstand; Variante C:
  Reiter), steht als Suchparameter in der Adresse, einheitlich benannt (`?tab=`, `?item=`);
  bestehende Parameter (`?library=`, `?add=`, `?days=`) bleiben, wie sie sind.
- **S-15 Fenstertitel** je Seite bleibt (`useDocumentTitle`).

**Erhaltung (gilt für jeden Mockup und den Bau):**

- **E-1** Jede Zeile der Erhaltungsliste (Abschnitt 7) ist im Mockup sichtbar oder mit einem
  gezeichneten Menü, Reiter, Klappbereich oder Dialog erreichbar.
- **E-2** Beim Bau ist die Erhaltungsliste die Abnahme-Checkliste je Seite; der OMC-Verifizierer
  hakt sie gegen die gebaute Seite ab.
- **E-3** Nichts fällt stillschweigend weg; zieht eine Funktion um (zum Beispiel in einen Dialog
  oder auf eine andere Seite), steht das als Abweichung in Frage 18.

## 5. Kompakte Dichte nur unter `/admin`

### 5.1 Masse heute und neu (Vorschlag, im Grundmuster sichtbar)

| Grösse | Heute (alle Seiten) | Neu unter `/admin/*` |
|---|---|---|
| Seitentitel | 60 px, `display` | 24 px, 600 |
| Erklärsatz | 20 px, immer sichtbar | entfällt, im Hilfe-Knopf |
| Steuerelement-Höhe | 44 px | 32 px |
| Schrift in Knöpfen und Feldern | 16 px | 14 px |
| Panel Innenabstand / Lücke / Radius | 24 / 20 / 20 px | 16 / 12 / 12 px (`radius-lg`) |
| Abschnittsabstand | 40 bis 48 px | 24 px |
| Abschnittstitel | 24 px | 16 px (`font-size-md`), 600 |
| Tabellenzeile | rund 48 px | 40 px |
| Seitenrand | bis 64 px | 24 bis 32 px |

**Barrierefreiheit:** 32 px Steuerelemente liegen unter der 44-px-Empfehlung für Berührung,
aber über WCAG 2.2 AA (2.5.8, 24 px). Die kompakten Masse gelten darum nur bei **Breite ≥ 768 px
und `(pointer: fine)`** (Maus; wie schon `global.css:154`); Tablets mit Touch und Handys behalten
44 px. Kontraste bleiben unverändert (dieselben Farben). **Dialoge,** die aus einer
Verwaltungsseite geöffnet werden, sind ebenfalls kompakt (Marco, 1. November 2026).

### 5.2 Wie es gebaut wird (Skizze für den Bau)

- **Komponententokens statt Kopien:** Die Bausteine lesen heute die allgemeine Skala
  (`Panel.module.css:4-9` `space-6`/`space-5`/`radius-xl`, `Button.module.css:11` `font-size-md`,
  `DataTable.module.css:28,52` `space-3`/`space-4`); diese umzustellen, würde Fliesstext, Chips
  und Abstände überall mitschrumpfen. Darum bekommt `packages/design-tokens/tokens/` neue
  **Komponententokens** mit dem heutigen Wert als Standard: `--sc-control-height`,
  `--sc-control-font-size`, `--sc-panel-padding`, `--sc-panel-gap`, `--sc-panel-radius`,
  `--sc-table-cell-padding-y`, `--sc-page-title-size`, `--sc-section-gap`. `Button`, `Field`,
  `Panel`, `DataTable`, `Dialog` und `Text` lesen sie; ihre Referenzbilder werden neu erstellt,
  sehen aber gleich aus. Eine Dichte-Gruppe `density.compact` liefert die Werte aus 5.1.
- **Wo umgestellt wird:** nur der **Inhaltsbereich** (`#content`) und die Dialoge der
  Verwaltungsseiten, unter `[data-shell="admin"]` (Attribut aus Konzept 11, 11.6) mit
  `@media (min-width: 48rem) and (pointer: fine)`. App-Kopf und Verwaltungsleiste behalten ihre
  Masse. Dialoge liegen im Portal ausserhalb von `#content`; sie bekommen die kompakten Werte über
  das Attribut am Dokument, solange `data-shell="admin"` gilt.
- `--sc-size-control-height` bleibt unverändert als «mindestens 44 px» dokumentiert
  (`tokens/space.json:134-138`); die neue Kompaktgrösse ist ein eigener Token.
- stylelint prüft nur Farbeigenschaften; die neuen Tokens sind Masse. Regel 9 bleibt erfüllt.
- Eine zweite Dichte im Tokensystem ist ein Architekturentscheid: **ADR 0020** «Kompakte Dichte
  der Verwaltung» ist nötig.
- **Neue Bausteine** (`web/src/ui`, je Story und Referenzbild): `AdminPage` (Kopf nach S-2,
  Zustandsleiste, Inhalt), `SectionHeader`, `SettingRow` (S-7), `RowActions` (S-6, nutzt
  `MenuButton`), `Disclosure` (Klappbereich, nur Variante C), `MasterDetail` (nur Variante B),
  `SaveBar` (aus `MetadataPage` gelöst). `Text` bekommt die Variante `pageTitle`.
- **Abgrenzung zu Konzept 11:** Konzept 11 baut Leiste, Gerüst und Übersicht; dieses Konzept
  baut die Seiten. Gemeinsame Teile (`data-shell`, `DashCard`, `StreamRow`, `TaskRow`) entstehen
  einmal, im Bauplan wird das zusammengelegt.
- **Keine Zwischenstände mit gemischter Grösse:** Die kompakte Dichte wird erst mit der letzten
  umgebauten Seite eingeschaltet; bis dahin bauen die Commits die Seiten auf die neuen Bausteine
  um, in heutiger Grösse.

## 6. Drei Grundmuster zur Wahl (Phase 1)

Gezeigt an drei typischen Seiten, je eine für Zustand und Aktionen (**Hintergrundjobs**),
Formulare und Geheimnisse (**Integrationen**) und Tabellen mit Dialogen (**Konten**). Alle drei
Varianten benutzen dieselbe kompakte Dichte (5.1) und dieselben Regeln (Abschnitt 4); sie
unterscheiden sich im **Aufbau**.

### Variante A «Kompakte Liste»

Eine Seite, flach und dicht. Abschnitte mit Titel und feiner Linie statt schwerer Panels;
Daten als dichte Tabellen; Einstellungen als Zeilen (S-7); seltene Zeilenaktionen im Menü;
Bearbeiten in Dialogen. **Stärke:** nah am heutigen Aufbau, kleinster Umbau, alles auf einer
Seite durchsuchbar (Strg+F). **Schwäche:** lange Seiten (Integrationen) bleiben lang, nur kürzer.

### Variante B «Liste und Detail»

Links im Inhalt eine schmale Liste der Gegenstände (Jobarten, Integrationen, Konten) mit
Zustand und Suchfeld; rechts der gewählte Gegenstand mit allen Angaben und Aktionen. Ersetzt
lange Seiten und die meisten Dialoge. **Stärke:** sehr ruhig, immer nur ein Gegenstand im Blick,
wie Emby und Mail-Programme. **Schwäche:** Vergleich über viele Gegenstände schwerer; braucht
eine Adresse je Auswahl (`?select=`), damit Zurück und Links funktionieren.

### Variante C «Reiter und Klappbereiche»

Kopf, Zustandsleiste, dann Reiter für Teilbereiche (zum Beispiel Integrationen: Schlüssel ·
Webhooks · E-Mail · Telegram · Push · System); in einem Reiter Klappbereiche mit einer
Zusammenfassung in einer Zeile, wenn zu. **Stärke:** kurze Seiten, Zustand auf einen Blick in
den Zusammenfassungen. **Schwäche:** Inhalt hinter Reitern nicht mit Strg+F auffindbar; ein
Klick mehr.

**Mischung ist erlaubt:** Marco kann ein Grundmuster wählen und je Seitentyp abweichen
(zum Beispiel C für Einstellungsseiten, A für Tabellenseiten). Das hält Abschnitt 10 fest.

**Mockup:** Design-Canvas «Verwaltungsseiten Grundmuster» (claude.ai, privat, mit Kommentaren);
neun Zeichenflächen, Zeilen = Varianten, Spalten = Seiten. Abweichend von der bisherigen Regel
«eine Datei `NN-…-prototyp.html` je Konzept» liegen die Quellen der Zeichenflächen als Ordner
`docs/planning/mockups/12-verwaltungsseiten/` im Zweig (eine `.dc.html` je Zeichenfläche; sie
laufen nur im Canvas, die Ablage dient der Nachvollziehbarkeit).

## 7. Erhaltungsliste (alle Funktionen je Seite)

Kennungen bleiben über alle Fassungen stabil. «→» heisst: Ziel oder geöffneter Dialog.

### 7.1 Bibliothek

**Bibliotheken** (`LibrariesPage.tsx`, `LibraryCard.tsx`, `LibraryFoldersDialog.tsx`; Abfrage
alle 5 s, solange ein Scan läuft)
- BIB-1 Liste aller Bibliotheken: Name, Art, Standard-Freigabe (Audio), Scan-Zustand,
  Titel- und Dateizahl, Fortschritt beim Scan, Ende des letzten Scans, Fehlermeldung.
- BIB-2 Ordner-Zusammenfassung je Bibliothek: Pfad, Inhalt, Anbieterkette, Dateisystem,
  Warnung Netzwerkfreigabe, Überwachungsart.
- BIB-3 Jede Videobibliothek: Zeile Metadaten (abweichend oder «wie Server») mit Link →
  Metadaten `?library=` (`LibraryCard.tsx:121,206-221`).
- BIB-2a Mehrere Ordner: Karte zeigt Anzahl und Zusammenfassung (`LibraryCard.tsx:183-197`).
- BIB-4 Aktionen: Neu scannen (gesperrt ohne `canScan`), Scan abbrechen (nur beim Scan), Ordner
  (→ Dialog), Standard-Freigabe ändern (nur Audio, Dialog, Meldung «gespeichert»), Verlust
  akzeptieren (nur abgebrochen, Rückfrage).
- BIB-5 Ordner-Dialog: Wurzeln mit Ändern (Inhalt, Anbieter, Rückfrage mit Eingabe) und
  Entfernen (Rückfrage mit Eingabe und Folgenliste); Ordner hinzufügen mit Ordnerbrowser.
- BIB-6 Bibliothek hinzufügen: Name, Art, Freigabe, Ordnerkarten mit Anbietern, Ordnerbrowser,
  «N Ordner hinzufügen».

**Podcasts** (`PodcastsPage.tsx`)
- POD-1 Feed abonnieren (Dialog: Adresse, Bibliothek); ohne Podcast-Bibliothek Hinweis mit Link
  → Bibliotheken.
- POD-2 Tabelle: Titel (Autor, Feed-Adresse, «aus Ordner», «explizit»), Bibliothek, Folgen
  (lokal), letzter Abruf, Fehler.
- POD-3 Aktionen: Jetzt abrufen (nur Feed; danach 30 s Abfrage alle 2 s), Abo entfernen
  (Rückfrage; nur wenn kein Ordner es trägt).

**Pflege** (`MaintenancePage.tsx`, `MatchDialog.tsx`)
- PFL-1 Sieben Reiter mit Zahl: Nicht zugeordnet, Ohne Bilder, Ohne Freigabe, Verwaist,
  Nicht abspielbar, Freigabe-Abweichung, Doppelte.
- PFL-2 Je Reiter: Hinweis, Ergebnis-Meldung, Auswahl (alle / keine, «N gewählt»), Liste mit
  Auswahlkästchen und Link → Titel, «X von Y», Mehr laden (50 je Seite).
- PFL-3 Aktionen: Zuordnen (Dialog: Suche, Jahr, Anbieter TMDB/TVDB, Art; Kandidaten), Metadaten
  auffrischen, Bilder auffrischen, Freigabe setzen (Dialog), Aufräumen (Rückfrage), Neu prüfen,
  Dateien entfernen (Rückfrage); Freigabe-Abweichung nur Liste.
- PFL-4 Doppelte: je Gruppe Dateien mit Pfad und Grösse, Ziel wählen, Zusammenführen
  (Rückfrage).

**Titel** (`/admin/items/$itemId`, `AdminItemPage.tsx`, `ImagesPanel.tsx`)
- TIT-1 Kopf: Art, Jahr, Titel; Zuordnen (Film, Serie), Metadaten auffrischen, Ansehen → Titelseite.
- TIT-2 Metadaten-Fakten (Art, Id, Bibliothek, Staffel/Folge, Zustand, Fehler, zugeordnet am,
  TMDB/TVDB/IMDb-Ids, AniList als externer Link, Anbieter, gesperrt, Kinder); unbekannter Titel →
  Zustand «nicht gefunden» (`AdminItemPage.tsx:419`).
- TIT-3 Herkunft der Felder.
- TIT-4 Freigabe: eigene, wirksame, Quelle; Wahl inkl. «aus Metadaten»; Speichern.
- TIT-5 Bilder je Art (Poster, Hintergrund, Logo; Staffel: Poster): Anbieter, gewählt,
  gesperrt; Wählen (Dialog mit Kandidaten, Mehr zeigen, Kandidaten neu laden), Entsperren.
- TIT-6 Dateien: Pfad, Version, «nicht verfügbar», Grösse, Warnungen, geprüft am, Prüffehler
  (`AdminItemPage.tsx:359-367`).
- TIT-7 Trailer: Liste mit Sprache und «offiziell», Löschen (Rückfrage), Trailer hinzufügen
  (Dialog: Adresse, Sprache, andere Sprache).

**Metadaten** (`MetadataPage.tsx`)
- MET-1 Geltung: Server oder Bibliothek (Auswahl); bei Bibliothek Schalter «Texte weichen ab»,
  «Bilder weichen ab»; Links → Bibliotheken, Hintergrundjobs.
- MET-2 Sprachen: geordnete Liste mit Hoch, Runter, Entfernen; Sprache hinzufügen;
  YouTube-Hinweis; Warnung «Sprache bei Anbieter X unbekannt» (`MetadataPage.tsx:270-281`);
  Rückfall-Hinweis (`:601`).
- MET-3 Bilder: Poster und Hintergrund immer; weitere Bildarten wählbar (mit Vermerk «noch nicht
  gebaut», `:657`); Sprachkette je Bildart.
- MET-4 Nur Server: Qualität (Mindestbreite Poster, Mindestbreite Hintergrund, Wahl nach
  Bewertung), Quellen (TMDB Erwachsene, lokal mit Zahl lokaler Bilder `:771`, Fanart mit Link →
  Integrationen), Freigabeland.
- MET-5 Jetzt anwenden: was (beides, Texte, Bilder), wo (alle oder eine Bibliothek, mit
  Titelzahl), Ausführen (Job), Link → Hintergrundjobs, letzter Lauf / wartend (Abfrage alle 10 s).
- MET-6 Speicherleiste: geändert / unverändert, Verwerfen, Speichern, Meldungen inklusive der
  Zahl betroffener Titel nach dem Speichern (`:435`, `:841-845`).

### 7.2 Konten

**Konten** (`AccountsPage.tsx`, `Invitations.tsx`)
- KON-1 Einladen (Dialog: Notiz, Freigabe, Bibliotheken → einmaliger Link mit Kopieren, Fertig).
- KON-2 Tabelle: E-Mail, Rolle (Besitzer), Freigabe, Bibliotheken, Profile, zuletzt gesehen,
  Kanäle.
- KON-3 Eigene Zeile: Bearbeiten (E-Mail, neues Passwort zweimal, aktuelles Passwort), Profile
  → `/settings/profiles`.
- KON-4 Andere Zeilen: Zugriff (nur Benutzer: Freigabe, Bibliotheken, Limits),
  Befördern/Zurückstufen (nur Besitzer, Rückfrage), Entfernen (Rückfrage).
- KON-5 Einladungen: Notiz, Zustand, Freigabe, benutzt von / läuft ab; Widerrufen (Rückfrage,
  nur bei Zustand «offen», `Invitations.tsx:211`).

**Profile** (`/settings/profiles`, `ProfileSettingsPage.tsx`)
- PRO-1 Profil hinzufügen.
- PRO-0 Zugriff: Hauptprofil jedes Kontos; sonst Hinweis mit Link → Profilwahl
  (`ProfileSettingsPage.tsx:28,59-75`).
- PRO-2 Je Profil: Bild, Name, Merkmale (Hauptprofil, Freigabe oder «Freigabe über Konto»,
  Oberflächen- und Metadatensprache getrennt, PIN, Limits, Bibliotheken;
  `ProfileSettingsPage.tsx:146-177`).
- PRO-3 Bearbeiten (Name, Auswahlfelder, Profilfarbe, Bibliotheken, Limits, PIN), Bericht
  zeigen/verbergen (Balkendiagramm mit Heute, Gesamt, Limit und Limitlinie, `WatchReport.tsx:58-81`;
  mehrere Berichte gleichzeitig offen), PIN setzen/ändern, PIN entfernen (Rückfrage), Löschen
  (Rückfrage).

**Geräte** (`DevicesPage.tsx`)
- GER-1 Tabelle: Name, Art, Besitzer, zuletzt gesehen, Container, Video, Audio, Auflösung, HDR,
  Streams; Vergessen (Rückfrage).

**Streams** (`StreamsPage.tsx`, Abfrage alle 5 s, pausiert bei offener Rückfrage)
- STR-1 Tabelle: Profil, Gerät, Titel, Qualität, Modus (direkt, Remux, Transcode HW/SW),
  Position, seit; Stoppen (Rückfrage).

### 7.3 Kuratierung

**Spotlight** (`SpotlightPage.tsx`)
- SPO-1 Schalter Aktiv.
- SPO-1a Satz, warum die Reihe fehlt (kein Schlüssel, aus, Fehler, leer, unter Mindesttreffern;
  `SpotlightPage.tsx:81-90,118`).
- SPO-2 Quelle: Liste (mit Link → Integrationen), Listen-Adresse, Medien, Anzahl, Mindesttreffer,
  Takt in Stunden, Rotation; Abrufzustand (Treffer, letzter Abruf oder Fehler mit Grund,
  Schlüssel-Link; `:93-99`); Jetzt abrufen (danach Abfrage alle 2 s), Speichern. «Aktiv» (SPO-1)
  wird mit diesem Speichern gespeichert (`:187-189,689`).
- SPO-3 Einträge: Rang, Titel mit Poster, Medium, Jahr, Zustand, Notiz; Anheften (Dialog mit
  Rang)/Lösen, Verbergen/Zeigen, Notiz (Dialog); Einträge ausserhalb der Bibliothek ohne Aktionen
  (`:516-535,587`).
- SPO-4 Verwaiste Einträge: Entfernen.
- SPO-5 Hinweis mit Link → Startseitenreihen.

**Cat's Empfehlungen** (`PicksPage.tsx`, `?add=`)
- PIC-1 Empfehlung hinzufügen (Dialog: Titelsuche, Zielprofile, Notiz, von, bis); `?add=` wählt
  den Titel vor.
- PIC-2 Liste mit Zahl (Einträge, geplant): Position, Titel, Zielprofil, Notiz, Zeitfenster
  (abgelaufen, geplant).
- PIC-3 Hoch, Runter, Bearbeiten (Dialog: Notiz, von, bis), Entfernen (Rückfrage).
- PIC-4 Hinweis mit Link → Startseitenreihen.

**Sammlungen** (`CollectionsPage.tsx`)
- SAM-1 Sammlung anlegen (Dialog: Name).
- SAM-2 Tabelle: Name (mit deutschem Namen, `CollectionsPage.tsx:312-314`), Quelle (TMDB, von
  Hand), Anzahl.
- SAM-3 Mitglieder (Dialog: Titelsuche, geordnete Liste mit Hoch, Runter, Entfernen,
  Speichern), Umbenennen, Entfernen (Rückfrage); bei TMDB-Sammlungen Mitglieder und Umbenennen
  gesperrt (`:341-358`).

**Startseitenreihen** (`HomeRowsPage.tsx`)
- REI-1 Acht Reihen (Spotlight, Empfehlungen, Weiterschauen, Neu, Erschienen, Merkliste,
  Sammlungen, Genres): Verbergen, Hoch, Runter; Speichern; Link → Spotlight.

**Ankündigungen** (`AnnouncementsPage.tsx`)
- ANK-1 Erstellen (Dialog: Titel, Text, «sofort veröffentlichen»).
- ANK-2 Tabelle: Titel, erstellt, veröffentlicht/Entwurf, bestätigt (Zahl).
- ANK-3 Bearbeiten und Veröffentlichen (nur Entwurf, Veröffentlichen mit Rückfrage), Löschen
  (Rückfrage).

### 7.4 System

**Hintergrundjobs** (`JobsPage.tsx`, `JobScheduleDialog.tsx`; Abfrage alle 5 s, pausiert bei
offenem Dialog)
- JOB-1 Zählwerte: laufend, wartend, geplant, pausiert, fehlgeschlagen.
- JOB-2 Arbeiter-Pools: Arbeiter, frei / Rückstand.
- JOB-3 Hinweis pausierte Jobarten mit Fortsetzen je Art.
- JOB-4 Läuft gerade: Name, Gegenstand, «wird gestoppt», Dauer, Fortschrittstext und -balken,
  Restzeit; Pausieren (Jobart), Abbrechen (Rückfrage mit Warnung je Art).
- JOB-4a Laufzeit tickt im Sekundentakt (`JobsPage.tsx:70-75,128`).
- JOB-5 Jobarten in vier Gruppen (Bibliothek, Metadaten, Auslieferung, System): Name, Zustand,
  wartend mit Schätzung des Rückstands, Zahl geplanter Läufe, Mediandauer (`:546-562`), letzter
  Lauf mit Ergebnis (Fehlertext, abgebrochen, übersprungen, «vor X»; `:496-511`), nächster Lauf
  als «in X, alle Y, Zeitfenster» (`:515-531`), Auslöser.
- JOB-6 Aktionen je Jobart: Jetzt ausführen (nur `runnable` und nicht pausiert),
  Pausieren/Fortsetzen, Wartende abbrechen (Rückfrage), Alle abbrechen (Rückfrage), Zeitplan (nur
  mit `schedule`; Dialog: aktiv, Intervall, Zeitfenster mit Start/Ende, Hinweise), Läufe;
  unbekannte Jobarten ohne Aktionen (`:586-588,668`).
- JOB-7 Läufe: Reiter Läuft, Wartend, Geplant, Fehlgeschlagen, Erledigt, Abgebrochen; Tabelle
  Gegenstand, Priorität, gestartet bzw. geplant für (`runAfter`, `:826`), Dauer, Versuche, Fehler;
  Erneut / Abbrechen je Lauf; höchstens 200 mit Gesamtzahl.
- JOB-8 **Neu auf dieser Seite** (Marco, 1. November 2026): Abschnitt «Letzte Fehlschläge (alle
  Arten)» mit den zehn neuesten Fehlschlägen über alle Jobarten (Jobart, Versuche, Fehler einzeilig,
  Zeit) und **Erneut** / **Abbrechen** (Rückfrage), wie heute auf der Übersicht
  (`AdminHomePage.tsx:447,561`). Daten aus `GET /admin/dashboard` `jobs.recentFailed`, das nach
  Konzept 11 (11.3) im Vertrag bleibt; **keine Vertragsänderung**. Die Übersicht verlinkt mit
  `?section=failed` direkt dorthin. Damit geht die Liste, die Konzept 11 F-2 von der Übersicht
  nimmt, nicht verloren.

**Integrationen** (`IntegrationsPage.tsx`, `WebhooksSection.tsx`)
- INT-1 Schlüssel TMDB, TVDB, TVDB-PIN, YouTube, Fanart, Fanart-Client, Trakt: Herkunft
  (Umgebung, gespeichert), Feld, Speichern, Löschen (Rückfrage); Trakt mit Zustandslink →
  Spotlight.
- INT-2 Webhooks: neue Integration (Name, Art Radarr/Sonarr) → einmalige Adresse mit Kopieren,
  Schliessen.
- INT-3 Je Webhook-Integration: Art, Token-Ende, zuletzt gesehen; Pfadzuordnung Von/Nach mit
  Entfernen, Zeile hinzufügen, Speichern, Übernehmen; Ereignisprotokoll (Zeit, Ereignis,
  gemeldet, aufgelöst, Ergebnis); Adresse erneuern, Löschen (je Rückfrage).
- INT-4 E-Mail: Host, Port, Benutzer, Passwort (nur schreiben), Absender, TLS; Speichern,
  Löschen, Testmail senden.
- INT-5 Telegram: Bot-Name, Token; Speichern, Löschen.
- INT-6 Web-Push: Zustand, öffentlicher Schlüssel, Schlüssel erneuern (Rückfrage).
- INT-7 Update-Prüfung ein/aus.
- INT-8 Öffentliche Adresse: Feld, Speichern, Löschen.

**Benachrichtigungen** (`/settings/notifications`, `ChannelSettingsPage.tsx`)
- BEN-1 Je Kanal (E-Mail, Telegram, Push): Zustand an / aus / nicht verfügbar; bei nicht
  verfügbarem Kanal Link → Integrationen, nur für Administratoren (`ChannelSettingsPage.tsx:340-350`).
- BEN-2 Telegram: Verknüpfen, Erneuern, Lösen; Code mit Ablauf, «In Telegram öffnen», Fall ohne
  Bot (`:116-139`); Abfrage alle 3 s bei offenem Code.
- BEN-3 Push: Geräteliste mit «dieses Gerät», Datum und Entfernen, «Dieses Gerät abonnieren»,
  iOS-Hinweis, «nicht unterstützt» (`:213-235`).
- BEN-4 Formular: Schalter, E-Mail-Adresse, Ereignisse (Ereignis «Fehler» nur für
  Administratoren, `:327`); Speichern, Test senden.
- BEN-5 Andere Profile des Kontos (nicht Hauptprofil): Zusammenfassung ohne Steuerung (Ereignisse,
  Adresse, verknüpft, Gerätezahl) und Hinweis mit Link → Profilwahl (`:492-501,520-527`).

**Statistik** (`StatsPage.tsx`)
- STA-1 Zeitraum 7, 30, 90, 365 Tage (in der Adresse).
- STA-2 Meistgesehene Titel (Link → Titel), Sehzeit je Profil (Diagramm und Tabelle),
  Wiedergabe (direkt, Remux, Transcode, Hardware), Geräte (Diagramm und Tabelle).

**Sicherung** (`BackupPage.tsx`)
- SIC-1 Sicherung erstellen (Dialog: mit/ohne Geheimnisse, Passphrase zweimal).
- SIC-2 Aus Datei wiederherstellen (Dialog: Warnung, Datei, Passphrase, Rückfrage).
- SIC-3 Tabelle: Name, erstellt, Grösse, Art, Geheimnisse, verschlüsselt; Herunterladen,
  Wiederherstellen (Dialog oder Rückfrage), Löschen (Rückfrage).
- SIC-4 Warteanzeige nach Wiederherstellung (Abfrage `/healthz` alle 2 s, Abbruch nach 90 s,
  «Erneut warten», Version, Hinweis «ohne Geheimnisse», danach Weiterleitung zur Anmeldung;
  Fokus auf die Warte-Überschrift; `BackupPage.tsx:281,306-312`, `restart.ts:22-23`).

### 7.5 Seitenübergreifend

- ALL-1 Zugriffsprüfung: `/admin/*` Administrator mit Hauptprofil, sonst Hinweis «nicht
  erlaubt»; `/settings/profiles` Hauptprofil jedes Kontos; `/settings/notifications` jedes Profil,
  Steuerung nur mit Hauptprofil.
- ALL-2 Laden, Fehler mit «Erneut versuchen», Leerzustand, Ergebnis-Meldungen als `role="status"`;
  Fokus nach dem Entfernen auf den Seitenkopf (`tabIndex={-1}`; `DevicesPage.tsx:42`,
  `StreamsPage.tsx:69`, `PodcastsPage.tsx:155`, `CollectionsPage.tsx:300`, `PicksPage.tsx:371`,
  `AnnouncementsPage.tsx:160`, `SpotlightPage.tsx:506`, `BackupPage.tsx:374`).
- ALL-3 Links in die Verwaltung bleiben gültig: Titelseite → Titel (`/admin/items/$id`) und →
  Empfehlungen `?add=`, Startseite ohne Bibliothek → Bibliotheken, Benachrichtigungen →
  Integrationen, Profilwahl → Profile (`ProfilePickerPage.tsx:145`), Mitteilungen →
  Benachrichtigungen (`NotificationsPage.tsx:93`), App-Leiste → Profile (`AppLayout.tsx:220`).
- ALL-4 Adressen und Suchparameter bleiben (`?library=`, `?add=`, `?days=`).

## 8. Ablauf

```
Phase 1  Grundmuster: 3 Varianten an 3 Seiten  → Marcos Wahl (F-1 bis F-4)
Phase 2  Seiten gruppenweise, je 3 Vorschläge  → Wahl und Änderungswünsche je Seite
         2a  Hintergrundjobs, Metadaten, Integrationen, Bibliotheken, Pflege (+ Titel)
         2b  Konten, Profile, Geräte, Streams, Podcasts
         2c  Spotlight, Empfehlungen, Sammlungen, Startseitenreihen, Ankündigungen
         2d  Benachrichtigungen, Statistik, Sicherung
Phase 3  Fassung «Bauplan»: Schritte, Tests, Frage 18, ggf. ADR 0020; OMC-Kritiker
Phase 4  Bau zusammen mit Konzept 11 und den weiteren Konzepten (auf Marcos Wort)
```

Phase 2a nimmt die Seiten, die Marco am meisten stören (Interview, 1. November 2026).

**Änderungswünsche:** Marco kommentiert direkt im Design-Canvas an der Stelle, die er ändern
will. Jeder Wunsch wird eingearbeitet und hier im Abschnitt 10 mit Datum festgehalten; erst wenn
eine Seite keine offenen Wünsche hat, gilt ihr Vorschlag als gewählt.

**Bau mit den OMC-Agenten (Phase 4, Vorschlag):** `planner` schneidet die Schritte zusammen mit
Konzept 11; je Schritt `executor` (Opus für Gerüst und Bausteine, Sonnet für das Umbauen der
Seiten); `designer` gleicht die gebaute Seite mit dem gewählten Mockup ab; `code-reviewer` prüft
je Schritt; `verifier` hakt die Erhaltungsliste (E-2) und die Tastaturtests ab. Jede Seite ein
eigener Commit, `task ci` grün.

## 9. Was dieses Konzept nicht tut

- Keine neue Funktion, keine Änderung am Vertrag (OpenAPI), keine Migration. JOB-8 zieht eine
  bestehende Anzeige um und nutzt ein bestehendes Feld. Adressen je Auswahl sind Suchparameter im
  Client (S-14).
- Keine Änderung an der Übersicht `/admin` (Konzept 11) und an Zuschauerseiten.
- Keine Handy-Mockups (Marco, 1. November 2026: «Nur Desktop»). Am Handy gelten beim Bau die
  heutigen Masse (44 px) und das Blatt aus Konzept 11; die Seiten müssen dort benutzbar bleiben.

## 10. Fragen an Marco

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-1 | Welches Grundmuster: A «Kompakte Liste», B «Liste und Detail», C «Reiter und Klappbereiche», oder eine Mischung je Seitentyp? | Offen bis zum Mockup. |
| F-2 | Passt die kompakte Dichte (5.1), oder kleiner/grösser? | Wie vorgeschlagen. |
| F-3 | Erklärsatz nur noch hinter dem Hilfe-Knopf (S-2)? | **Ja.** |
| F-4 | Höchstens zwei sichtbare Zeilenaktionen, Rest im Menü (S-6)? | **Ja.** |

### 10.1 Marcos Antworten im Interview (1. November 2026)

| Frage | Antwort |
|---|---|
| Reihenfolge zu Konzept 11 | Es kommen noch weitere Konzepte; danach alle Konzepte **zusammen** entwickeln. |
| Mockups ansehen, Änderungen wünschen | Design-Canvas auf claude.ai mit Kommentaren. |
| Vorschläge je Seite | **Drei.** |
| Geltung der kompakten Dichte | **Nur unter `/admin`.** |
| Profile und Benachrichtigungen (`/settings/*`) | **Mit einbeziehen.** |
| Staffelung | Erst Grundmuster, dann je Gruppe. |
| Seiten, die am meisten stören | Hintergrundjobs, Metadaten, Integrationen, Bibliotheken/Pflege. |
| Handy-Ansicht in den Mockups | **Nur Desktop.** |
| Artenübergreifende Fehlschlagliste (Kritiker-Blocker) | **Eigener Abschnitt auf Hintergrundjobs** (JOB-8), keine Vertragsänderung. |
| Dialoge kompakt? | **Ja, auch kompakt.** |
| `/settings/*` kompakt? | Folgt aus «nur unter `/admin`»: Muster ja, heutige Dichte (S-1). |

### 10.2 Änderungswünsche an den Mockups

(Noch keine.)

## 11. Review-Protokoll

### Fassung 1 → 2 (OMC-Kritiker, 1. November 2026)

Urteil «überarbeiten»: 1 Blocker, 9 Major, 8 Minor. Alle eingearbeitet.

| Befund | Schwere | Einarbeitung |
|---|---|---|
| JOB-8: artübergreifende Fehlschlagliste fiel zwischen Konzept 11 und 12 weg | Blocker | Marcos Entscheid: eigener Abschnitt auf Hintergrundjobs aus `jobs.recentFailed` (JOB-8) |
| BEN-5, ALL-1 für `/settings/*` falsch | Major | BEN-1, BEN-4, BEN-5, ALL-1, PRO-0 berichtigt |
| TIT-6 ohne Pfad, Grösse, «nicht verfügbar» | Major | TIT-6 ergänzt |
| SPO-2/3 unvollständig, «Aktiv» kein Sofortschalter | Major | SPO-1a, SPO-2, SPO-3, S-2 |
| JOB-5 unvollständig | Major | JOB-4a, JOB-5, JOB-6, JOB-7 |
| PRO-2/3 unvollständig | Major | PRO-2, PRO-3 |
| Dichte-Mechanik in 5.2 trägt nicht | Major | Komponententokens, nur Inhaltsbereich und Dialoge, ADR 0020 nötig |
| S-1 widerspricht Konzept 11 für `/settings/*` | Major | S-1 ergänzt, F-5 geschlossen |
| S-6 `MenuButton` untauglich | Major | `ActionMenu`, steuerbare Dialoge, Fokus, Abfrage-Pause |
| B/C verwerfen Eingaben | Major | S-13 |
| Kleinere Lücken MET, BIB, SAM, KON, SIC, BEN, TIT, ALL | Minor | ergänzt |
| 32 px nur mit Maus | Minor | 5.1: `(pointer: fine)` |
| Regel 8 falsch zitiert, «21 Seiten» falsch | Minor | Abschnitt 1.2 und 2 berichtigt |
| S-2 Geltungsbereich, S-3 h3, S-8, Ziel, Live-Text, `?select=` | Minor | S-2, S-3, S-8, S-14, Abschnitt 3 |
| Mockup-Ordner statt Datei | Minor | Abschnitt 6 begründet |
| Zwischenstände in Phase 4 | Minor | 5.2: Dichte erst mit der letzten Seite |
