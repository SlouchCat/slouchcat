# Konzept: Seitenmuster der Verwaltungsseiten (alle Seiten unter `/admin/*`)

- **Status: in Umsetzung seit 2. November 2026 (Zweig `claude/verwaltungsseiten`, Schritte K12-0
  bis K12-9 ohne Zwischenhalt, Abnahme in K12-9 auf CT 190; Vertrag 1.8.0).** Fassung 3
  (1. November 2026), Bauplan (Abschnitt 11). Als Papier abgeschlossen
  (Marco, 1. November 2026: «Ja, abschliessen»). Fassung 1 hat der
  OMC-Kritiker geprüft (1 Blocker, 9 Major, 8 Minor; Abschnitt 12); Fassung 2 arbeitet alle ein.
  Marco hat am 1. November 2026 gesagt: «Im Prinzip möchte ich alle Seiten in der Verwaltung
  überarbeiten. Es ist mir alles irgendwie zu gross und zu unübersichtlich.» Dieses Papier legt
  ein **Seitenmuster** fest, das alle bestehenden Verwaltungsseiten umbaut und für **jede künftige
  Verwaltungsseite** gilt. Marco hat **je Seite anhand eines klickbaren Mockups** gewählt
  (Abschnitt 10.3, alle 19 Seiten gewählt). Fassung 3 legt den Bau fest (Abschnitt 11); **wo
  Abschnitt 11 etwas anderes sagt als die Abschnitte 4 bis 9, gilt Abschnitt 11.**
- **Datum:** 1. November 2026; Fassung 3 vom selben Tag.
- **Umsetzung begonnen (2. November 2026):** Marco: «Bitte beginne mit der Umsetzung und
  Entwicklung von Konzept 12.» Zweig `claude/verwaltungsseiten` ab `main` **`4c860bc`** («Merge
  pull request #70», Konzept 11 umgesetzt und als 1.7.0 veröffentlicht). **Frage 18** und
  **ADR 0020** sind mit Schritt K12-0 eingetragen. **O-1 ist beantwortet** (11.9): 1.7.0 ist schon
  erschienen, die Felder aus 11.4 kommen darum in **Vertrag 1.8.0 als eigenes Release**; Konzept 13
  «Abspielseite» und Konzept 14 «Wiedergabestart» rücken auf 1.9.0 oder später und behalten
  Frage 19 und 20 sowie ADR 0021 und 0022. Marco will **durchbauen**: kein Zwischenhalt zwischen
  den Schritten, Abnahme nur in K12-9 auf CT 190. O-2 bis O-4 gelten wie vorgeschlagen; O-4 ist am
  2. November 2026 auf ganze Wochen verfeinert (11.4, 11.9).
- **Verankert gegen:** `main` **`4c860bc`**, Vertragsfassung **1.7.0** (Konzept 11: Frage 17,
  ADR 0019, Migration 00031, Verwaltungsleiste, Übersicht). Dieses Konzept nimmt **Frage 18**,
  **ADR 0020** und **Vertrag 1.8.0** (additiv), keine Migration. Die Zeilenangaben in Abschnitt 11
  stammen von `main` `5e0e63d` (Fassung 3); Konzept 11 hat seither unter anderem `AppLayout`,
  `AdminLayout`, `sections.ts` und `AdminHomePage` geändert. Jeder Schritt prüft die Zeilenangaben,
  die er braucht, gegen den aktuellen Stand nach.
- **Was nicht dazugehört:** die Übersicht `/admin` (bleibt nach Konzept 11, Abschnitt 11) und
  alle Zuschauerseiten mit der Startseite (unberührt).
- **Umsetzung:** ~~erst nach Marcos ausdrücklichem Wort, zusammen mit Konzept 11 und den
  weiteren Konzepten, die noch folgen (Marco, 1. November 2026).~~ Konzept 11 ist für sich
  erschienen (1.7.0); dieses Konzept wird seit dem 2. November 2026 allein gebaut (siehe oben).
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
  `--sc-table-cell-padding-y`, `--sc-page-title-size`, `--sc-section-gap`,
  `--sc-page-title-line-height` (Token `page.title-line-height`, Standard 1.5, kompakt 0; seit
  K12-8). `Button`, `Field`, `Panel`, `DataTable`, `Dialog` und `Text` lesen sie; ihre
  Referenzbilder werden neu erstellt, sehen aber gleich aus. Eine Dichte-Gruppe `density.compact`
  liefert die Werte aus 5.1; dazu kommt `density.compact.page.gutter` (32 px), das in `#content`
  `--page-gutter` aus `ui/global.css` setzt und kein `--sc-*`-Komponententoken ist (seit K12-8,
  ADR 0020, Nachtrag).
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

**Entscheid je Seite:** Marco hat am 1. November 2026 zuerst C gewählt und den Entscheid gleich
wieder zurückgezogen: «Ich möchte für jede Seite einzeln entscheiden aufgrund eines klickbaren
Mockups.» Die Varianten A, B und C sind darum die drei Vorschläge **je Seite**; passt ein Muster
zu einer Seite nicht, ersetzt der Vorschlag es durch eine begründete Alternative. Die Regeln
S-1 bis S-15 und die Dichte gelten für jede Wahl. Wählt Marco für verschiedene Seiten
verschiedene Muster, entstehen die Bausteine aller gewählten Muster (5.2).

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
- PFL-1 Sieben Reiter mit Zahl: Nicht zugeordnet, Ohne Bilder, Altersfreigabe fehlt (umbenannt bei
  der Abnahme K12-9, 3. November 2026), Verwaist, Nicht abspielbar, Freigabe-Abweichung, Doppelte.
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
- KON-3 Eigene Zeile (geändert 3. November 2026): «Konto ändern» inline im Klappbereich (E-Mail, neues Passwort
  zweimal, aktuelles Passwort) und darunter «Meine Profile» (KP-1). Der frühere Knopf «Profile →
  `/settings/profiles`» entfällt; an seiner Stelle stehen «Meine Profile» und der Leisten-Eintrag
  «Profile» (→ `/admin/accounts?item=me#meine-profile`; `AccountDetail.tsx:26`,
  `sections.ts:58`). `/settings/profiles` bleibt für alle Konten erreichbar (`router.tsx:491`).
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
- PIC-3 (geändert 3. November 2026) Platzwahl (`RankSelect`, S-16) statt Hoch/Runter, Bearbeiten inline im Klappbereich
  statt im Dialog (Notiz, von, bis; `PickDetail.tsx:110`), Entfernen (Rückfrage). Die Reihenfolge
  speichert die Operation `reorderTopPicks` (`PUT /admin/picks/order`, `api/openapi.yaml`); im
  Client heisst die Funktion `reorderPicks` (`web/src/features/admin/queries.ts:282`).
- PIC-4 Hinweis mit Link → Startseitenreihen.

**Sammlungen** (`CollectionsPage.tsx`)
- SAM-1 Sammlung anlegen (Dialog: Name).
- SAM-2 Tabelle: Name (mit deutschem Namen, `CollectionsPage.tsx:312-314`), Quelle (TMDB, von
  Hand), Anzahl.
- SAM-3 (geändert 3. November 2026) Mitglieder inline im Klappbereich statt im Dialog (Titelsuche, geordnete Liste mit
  Platzwahl (`RankSelect`, S-16) statt Hoch/Runter, Entfernen, Speichern;
  `CollectionDetail.tsx:168`), Umbenennen, Entfernen (Rückfrage); bei TMDB-Sammlungen Mitglieder und Umbenennen
  gesperrt (`:341-358`).

**Startseitenreihen** (`HomeRowsPage.tsx`)
- REI-1 Acht Reihen (Spotlight, Empfehlungen, Weiterschauen, Neu, Erschienen, Merkliste,
  Sammlungen, Genres): Ausblenden, Platzwahl (`RankSelect`, S-16; `HomeRowsPage.tsx:160`)
  statt Hoch/Runter (geändert 3. November 2026); Speichern; Link → Spotlight.

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
- BEN-3 Push: Geräteliste mit «dieses Gerät», Datum und Entfernen, «Auf diesem Gerät aktivieren»
  statt «Dieses Gerät abonnieren» (geändert 3. November 2026), iOS-Hinweis, «nicht unterstützt»
  (`PushDevices.tsx:86,102,127,131`).
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
  «Erneut prüfen» statt «Erneut warten» (geändert 3. November 2026; `admin.restart.slow.retry`), Version, Hinweis
  «ohne Geheimnisse», danach Weiterleitung zur Anmeldung; Fokus auf die Warte-Überschrift;
  `RestartPanel.tsx:36,76,95`, `restart.ts:5,8`).

### 7.5 Seitenübergreifend

- ALL-1 Zugriffsprüfung: `/admin/*` Administrator mit Hauptprofil, sonst Hinweis «nicht
  erlaubt»; `/settings/profiles` Hauptprofil jedes Kontos; `/settings/notifications` jedes Profil,
  Steuerung nur mit Hauptprofil.
- ALL-2 Laden, Fehler mit «Erneut versuchen», Leerzustand, Ergebnis-Meldungen als `role="status"`;
  Fokus nach dem Entfernen auf den Seitenkopf (`tabIndex={-1}`; `AdminPage` mit
  `titleRef`, `ui/AdminPage.tsx:58`; Fokus über `focusTitleIfLost`, `ui/focusAfter.ts:50`; geändert
  3. November 2026: `DevicesPage.tsx:212`, `StreamsPage.tsx:177`, `PodcastsPage.tsx:242`,
  `CollectionsPage.tsx:342`, `PicksPage.tsx:464`, `AnnouncementsPage.tsx:485`,
  `SpotlightPage.tsx:251`, `BackupPage.tsx:348`).
- ALL-3 Links in die Verwaltung bleiben gültig: Titelseite → Titel (`/admin/items/$id`) und →
  Empfehlungen `?add=`, Startseite ohne Bibliothek → Bibliotheken, Benachrichtigungen →
  Integrationen, Profilwahl → Profile (`ProfilePickerPage.tsx:145`), Mitteilungen →
  Benachrichtigungen (`NotificationsPage.tsx:93`), App-Leiste → Profile (`AppLayout.tsx:257`;
  geändert 3. November 2026). Das Ziel `/settings/profiles` bleibt, auch für
  Administratoren; die Verwaltungsleiste führt mit «Profile» nach `/admin/accounts?item=me` (KON-3).
- ALL-4 Adressen und Suchparameter bleiben (`?library=`, `?add=`, `?days=`).

## 8. Ablauf

```
Phase 1  Grundmuster: 3 Varianten an 3 Seiten (statisch)  → erledigt, kein globaler Entscheid
Phase 2  Je Seite 3 klickbare Vorschläge (A, B, C)       → Wahl und Änderungswünsche je Seite
         Pilot  Hintergrundjobs (Format angenommen)
         2a  Metadaten, Integrationen, Bibliotheken, Pflege (+ Titel)   → veröffentlicht
         2b  Konten, Profile, Geräte, Streams, Podcasts                   → veröffentlicht
         2c  Spotlight, Empfehlungen, Sammlungen, Startseitenreihen, Ankündigungen  → veröffentlicht
         2d  Benachrichtigungen, Statistik, Sicherung                     → veröffentlicht
Phase 3  Fassung «Bauplan»: Schritte, Tests, Frage 18, ADR 0020; OMC-Kritiker  → abgeschlossen (Abschnitt 11)
Phase 4  Bau: Konzept 11 allein gebaut und als 1.7.0 erschienen; Konzept 12 allein ab 2. November 2026 (1.8.0)
```

Phase 2a nimmt die Seiten, die Marco am meisten stören (Interview, 1. November 2026).

**Klickbar** heisst: Reiter, Klappbereiche, Menüs «Weitere Aktionen», Dialoge und die Auswahl
in Liste und Detail reagieren auf Klicks; Daten bleiben erfunden, nichts wird gespeichert. Jede
Seite ist eine eigene Seite im Design-Canvas mit drei Zeichenflächen.

**Änderungswünsche:** Marco kommentiert direkt im Design-Canvas an der Stelle, die er ändern
will. Jeder Wunsch wird eingearbeitet und hier im Abschnitt 10 mit Datum festgehalten; erst wenn
eine Seite keine offenen Wünsche hat, gilt ihr Vorschlag als gewählt.

**Bau mit den OMC-Agenten (Phase 4, Vorschlag):** `planner` schneidet die Schritte (Konzept 11
ist seit 1.7.0 gebaut, Konzept 12 läuft allein); je Schritt `executor` (Opus für Gerüst und Bausteine, Sonnet für das Umbauen der
Seiten); `designer` gleicht die gebaute Seite mit dem gewählten Mockup ab; `code-reviewer` prüft
je Schritt; `verifier` hakt die Erhaltungsliste (E-2) und die Tastaturtests ab. Jede Seite ein
eigener Commit, `task ci` grün.

## 9. Was dieses Konzept nicht tut

- Keine Migration. ~~Keine neue Funktion, keine Änderung am Vertrag (OpenAPI).~~ Überholt durch
  Marcos Wünsche: «Wann geschaut wird» und additive Bildfelder (11.1, 11.4). JOB-8 zieht eine
  bestehende Anzeige um und nutzt ein bestehendes Feld. Adressen je Auswahl sind Suchparameter im
  Client (S-14).
- Keine Änderung an der Übersicht `/admin` (Konzept 11) und an Zuschauerseiten.
- Keine Handy-Mockups (Marco, 1. November 2026: «Nur Desktop»). Am Handy gelten beim Bau die
  heutigen Masse (44 px) und das Blatt aus Konzept 11; die Seiten müssen dort benutzbar bleiben.

## 10. Fragen an Marco

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-1 | ~~Welches Grundmuster für alle Seiten?~~ Ersetzt durch: welche Variante **je Seite** (Abschnitt 10.3)? | Je Seite am klickbaren Mockup. |
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

| Datum | Seite | Wunsch | Umsetzung |
|---|---|---|---|
| 1. November 2026 | Metadaten (A, B, C) | «Ich möchte das mit den Sprachen bei Metadaten als Dropdown oder anders gelöst. Mir gefällt das mit dem Hin-und-her-Schieben nicht.» Interview: **feste Plätze**, auch für die Bildsprachen. | Je Rang eine Auswahl («1. Sprache», «2. Sprache» …); die Reihenfolge ergibt sich aus dem Platz. Wählt man eine Sprache, die schon auf einem anderen Platz steht, tauschen die beiden die Plätze (Hinweis darunter). «Weitere Sprache» fügt einen Platz an, «×» entfernt ihn. Bildsprachen gleich, mit «Ohne Text» als Eintrag. Die Regeln aus MET-2 (Standard braucht Deutsch, Bibliothek Deutsch oder Englisch) bleiben. **Marco: «Ja, besser so.»** |
| 1. November 2026 | Integrationen (A) | «Jedoch sollen die Titel klarer unterscheidbar sein. Jeder Block sollte klarer sein.» | Jeder Abschnitt (Schlüssel, Webhooks, E-Mail, Telegram, Web-Push, Server) ist ein eigenes Feld (`surface-1`, Rahmen, Radius 10) mit Kopfband in der Profilfarbe zu 12 %, Symbol und Titel 16 px; 20 px Abstand zwischen den Feldern. Für den Bau: dieselbe Band-Technik wie die Gruppentitel der Verwaltungsleiste (Konzept 11, F-9), ohne neue Farb-Tokens. |
| 1. November 2026 | Integrationen (A) | «Schon viel besser. Kannst du in Integrationen Variante A die Titel noch auf- und zuklappbar machen?» | Das Kopfband jedes Blocks ist ein Knopf mit Pfeil (`aria-expanded`); ein Klick klappt den Block zu oder auf, alle sechs starten offen. Beim Bau gilt S-13: Ungespeichertes in einem zugeklappten Block bleibt erhalten. **Marco: «Genau so machen wir’s.»** |
| 1. November 2026 | Konten, Profile | «Macht es nicht Sinn, dass die Profile und Konten im gleichen sind? Immerhin sind ja Profile an Konten gekoppelt. Bitte prüfe dies und mach nochmals Vorschläge.» | Geprüft: Die Profil-Endpunkte (`/profiles`, `/profiles/{id}`, `…/pin`, `…/watch-time`) sind `x-visibility: account`, gelten also nur für das **eigene** Konto; die Verwaltung sieht von fremden Konten nur die Profilnamen (`AccountsPage.tsx:315-318`). `/settings/profiles` bleibt für jedes Konto nötig (Hauptprofil jedes Kontos verwaltet seine Profile, PRO-0). Darum drei Vorschläge auf Basis von Konten C ohne neue Funktion: KP-1 Profile im aufgeklappten Konto, KP-2 eigener Reiter «Meine Profile», KP-3 Konten mit eingerückten Profilzeilen. Profile fremder Konten bleiben zum Lesen; sie zu bearbeiten wäre neue Funktion mit Vertragsänderung (Regel 11) und braucht einen eigenen Entscheid. |
| 1. November 2026 | Streams | «Streams möchte ich andere Vorschläge.» | Drei neue Vorschläge Str-D bis Str-F mit anderem Aufbau als A bis C. |
| 1. November 2026 | Statistik (B) | «Ergänzen um Zeit, wann wird gestreamt, Wochentage ebenfalls.» | Neues Feld «Wann geschaut wird» unter Profilen und Geräten: links «Nach Tageszeit» (24 Balken, Stunde in der Zeitzone des Servers), rechts «Nach Wochentag» (7 Balken Montag bis Sonntag), darunter zugeklappt «Woche und Stunde» als Raster 7 × 24 mit der Spitze im Kopf. Folgt dem Zeitraum (STA-1). **Neue Funktion** über A-8 hinaus: braucht im Vertrag die Felder `Stats.byHour` (24 Werte `watchedMs`) und `Stats.byWeekday` (7 Werte, Montag zuerst), additiv in Vertrag 1.8.0 (Fassung 3 sagte 1.7.0, O-1), gerechnet im Server aus den Schauereignissen in der Zeitzone des Servers (`TZ`). Für den Bau hat jedes Diagramm eine Tabelle als Alternative. Geht in Frage 18. |

### 10.3 Wahl je Seite

| Seite | Wahl | Datum |
|---|---|---|
| Hintergrundjobs | **C «Reiter und Klappbereiche»** (Reiter je Jobgruppe und «Fehlschläge», Jobarten als Klappbereiche) | 1. November 2026 |
| Metadaten | **B «Liste und Detail»** (Liste der Einstellungsbereiche, Geltung im Kopf), mit Sprachen als feste Plätze (10.2); «Sonst soweit eigentlich gut» | 1. November 2026 |
| Integrationen | **A «Kompakte Liste»**, jeder Block als eigenes, auf- und zuklappbares Feld mit getöntem Kopfband (10.2) | 1. November 2026 |
| Bibliotheken | **C «Reiter und Klappbereiche»** (Reiter nach Art, je Bibliothek ein Klappbereich) | 1. November 2026 |
| Pflege | **C «Reiter und Klappbereiche»** (Reiter je Problemart, je Bibliothek ein Klappbereich) | 1. November 2026 |
| Titel (Unterseite von Pflege) | **C «Reiter und Klappbereiche»** (Reiter Metadaten, Bilder, Dateien, Trailer) | 1. November 2026 |
| Konten | **C «Reiter und Klappbereiche»**, zusammengelegt mit Profile nach **KP-1 «Profile im Konto»**: das aufgeklappte eigene Konto zeigt unter «Konto ändern» den Block «Meine Profile» mit allen Profilaktionen (PRO-1 bis PRO-3); fremde Konten zeigen ihre Profilnamen nur zum Lesen («Profile verwaltet jedes Konto selbst.»). Marco: «Profile im Konto ist gut.» | 1. November 2026 |
| Profile | **In Konten aufgegangen (KP-1)** für Administratoren. Der Eintrag «Profile» der Verwaltungsleiste öffnet Konten mit dem eigenen Konto aufgeklappt bei «Meine Profile» (Abweichung von Konzept 11, 11.1 Punkt 4 und O-3, geht in Frage 18). `/settings/profiles` bleibt unverändert für Konten ohne Verwaltung (PRO-0, S-1). Keine Vertragsänderung: die Profil-Endpunkte bleiben `x-visibility: account`. | 1. November 2026 |
| Geräte | **C «Reiter und Klappbereiche»** (Reiter Geräte und Fähigkeiten, je Typ ein Klappbereich) | 1. November 2026 |
| Streams | **D «Kacheln» mit der Leiste «Serverlast» aus E** (Marco korrigierte zuerst gemeldetes «A»). Kachelbild = **Hintergrundbild quer** des Films bzw. der Serie: neues Feld `AdminStream.backdropUrl` (additiv, Vertrag 1.8.0 statt 1.7.0, O-1; Hash wie `posterUrl` aus Konzept 11, bei Folgen das Bild der Serie). Zähler Transcode HW/SW, Remux, Direct und Bandbreite als Summe der Bitraten im Client aus `AdminStream`. **Marco: «genehmigt».** | 1. November 2026 |
| Podcasts | **C «Reiter und Klappbereiche»** (Reiter je Bibliothek, je Podcast ein Klappbereich) | 1. November 2026 |
| Startseitenreihen | **C «Reiter und Klappbereiche»** | 1. November 2026 |
| Spotlight | **A «Kompakte Liste»** | 1. November 2026 |
| Cat's Empfehlungen | **C «Reiter und Klappbereiche»** | 1. November 2026 |
| Sammlungen | **C «Reiter und Klappbereiche»** | 1. November 2026 |
| Ankündigungen | **B «Liste und Detail»** | 1. November 2026 |
| Benachrichtigungen | **A «Kompakte Liste»** | 1. November 2026 |
| Statistik | **B «Raster»**, ergänzt um «Wann geschaut wird» (10.2) | 1. November 2026 |
| Sicherung | **C «Reiter und Klappbereiche»** (Reiter Sicherungen, Neue Sicherung, Aus Datei wiederherstellen; je Art ein Klappbereich) | 1. November 2026 |

**Abweichungen der Vorschläge vom Grundmuster (Phase 2a):**

- Metadaten B: die Liste links sind die **Einstellungsbereiche** (Sprachen, Bilder, Güte,
  Quellen, Altersfreigabe, Anwenden), die Geltung ist eine Auswahl im Kopf; C hat Reiter **je
  Geltung** (Standard und jede Bibliothek). Keine Menüs und Dialoge nötig.
- Bibliotheken C: Reiter **nach Art** (Alle, Video, Hörbücher, Podcasts), je Bibliothek ein
  Klappbereich; Bibliothek hinzufügen und neue Ordner als Dialog. Die Bestätigung durch Eintippen
  verlangt wie heute den **Pfad** (`ConfirmInputDialog`).
- Pflege C: im Reiter je **Bibliothek** ein Klappbereich (heute nicht gruppiert).
- Titel B: die Bildwahl steht im Detail statt in einem Dialog.
- Bezeichnungen folgen `de.json` (zum Beispiel «Erneut scannen», «Bereinigen», «Aktualisieren»).

**Phase 2b** (Marcos Rückmeldungen aus 2a von Anfang an eingebaut: Blöcke als eigene Felder mit
Kopfband und Auf-/Zuklappen, Auswahl statt Verschieben):

- Geräte C: Reiter «Geräte» und «Fähigkeiten» teilen die breite Tabelle; je Gerätetyp ein
  Klappbereich.
- Streams C: Zustandsleiste und Reiter je Modus (Direct, Remux, Transcode), je Stream ein
  Klappbereich. Die Positionen laufen im Mockup alle 5 s weiter, pausiert bei offener Rückfrage.
- Podcasts B: «Feed abonnieren» öffnet im Detail statt als Dialog; C hat Reiter je Bibliothek.
- Profile: Schauzeit über 30 Tage und PIN mit 4 bis 8 Ziffern wie im Code.

**Phase 2c und 2d** (gleiche Bausteine wie 2b: Felder mit Kopfband, Auswahl «Stelle» statt
«Nach oben»/«Nach unten», Bilder statt Farbflächen, Bezeichnungen aus `de.json`):

- Startseitenreihen B heisst «Liste und Vorschau»: rechts steht eine Vorschau der Startseite
  statt eines Detailbereichs. «Verwerfen» (ungespeicherte Reihenfolge zurücksetzen) und eine
  Kurzbeschreibung je Reihe sind neu in der Oberfläche, ohne neue Funktion (geht in Frage 18).
- Statistik B heisst «Raster»: Wiedergabe als Streifen, Platz 1 bis 3 als Kacheln mit
  Hintergrundbild, Profile und Geräte nebeneinander als Diagramm mit Tabelle.
- Spotlight: die Ränge der angehefteten Titel sind eine Auswahl, begrenzt auf «Höchstens».
- Sammlungen: TMDB-Sammlungen bleiben nur zum Lesen; «Löschen» ist für sie aus, wie im Code.
- Benachrichtigungen: «Verknüpfung lösen» (Telegram) bekommt eine Rückfrage; heute löst ein
  Klick sofort (geht in Frage 18).
- Seitentitel: ~~die Verwaltungsleiste sagt «Startseite», «Benachrichtigungen» und
  «Statistik».~~ Berichtigt in 11.1: heute heissen Seite und Leiste «Startseite» und
  «Statistiken»; nur «Benachrichtigungskanäle» (Leiste «Benachrichtigungen») weicht ab.
- Bildfelder, die der Vertrag noch nicht hat (additiv, Hash wie `posterUrl`, Vertrag 1.8.0 statt 1.7.0, O-1):
  `StatTitle.posterUrl` (Statistik A, C) und `StatTitle.backdropUrl` (Statistik B),
  `AdminCollectionItem.posterUrl` (Sammlungen), wahlweise `AdminPick.backdropUrl`
  (Cat's Empfehlungen). Ohne diese Felder zeigt der Bau die Farbfläche mit Titel.
- Sicherung: automatische Sicherungen legt der Server nur vor einer Migration
  (`pre-migration-…`) und vor einer Wiederherstellung (`pre-restore-…`) an.

**Zusammenlegen (Hinweise aus 2c und 2d):** Startseitenreihen, Spotlight und Cat's Empfehlungen
könnten eine Seite «Startseite» mit drei Reitern werden. **Marco (1. November 2026): «Getrennt
lassen».** Die drei Seiten bleiben eigene Einträge der Verwaltungsleiste wie in Konzept 11.
Sammlungen bleibt eigenständig. Benachrichtigungen bleibt getrennt von Integrationen, beide verlinken einander.
Ankündigungen steht inhaltlich näher bei Benachrichtigungen als bei der Kuratierung.

**Korrektur Hintergrundjobs (alle Varianten, auch das gewählte C), eingearbeitet am 1. November
2026:** Die erste Fassung zeigte 14 teils erfundene Jobarten («Sicherung» mit täglichem Zeitplan,
«Vorschaubilder», «Untertitel»). Jetzt zeigen A, B und C die **19 Jobarten, die der Server
registriert**, in ihren Gruppen: Bibliothek (Bibliotheksscan, Ordnerscan, Datei prüfen,
Keyframe-Index, Aufräumen fehlender Titel, Intro und Abspann erkennen), Metadaten (Titel
zuordnen, Metadaten auffüllen, Wertungen nachholen, Bilder holen, Metadaten anwenden, Trailer
suchen, Podcast-Feed abrufen, Podcast-Feeds prüfen, Spotlight-Liste abrufen), Zustellung
(Download vorbereiten, Mitteilung zustellen), System (Update-Prüfung, Verlauf aufräumen). Einen
Zeitplan haben sechs davon (Scan alle 6 h, Aufräumen stündlich, Podcast-Feeds alle 5 min,
Spotlight nach Takt, Update-Prüfung und Verlauf täglich); «Geplant» zeigt darum 6. Für den Bau:
`metadata.apply` hat heute keinen Eintrag unter `admin.jobs.types` in `de.json`/`en.json` und
erscheint roh als «metadata.apply»; der Bau ergänzt «Metadaten anwenden» (Text, keine neue
Funktion). JOB-1 bis JOB-8 bleiben unverändert.

## 11. Bauplan (Fassung 3)

Grundlage: Marcos Wahl je Seite (10.3), seine Änderungswünsche (10.2), die Erhaltungsliste
(Abschnitt 7) und die gewählten Mockups im Ordner `docs/planning/mockups/12-verwaltungsseiten/`
(Dateien in 11.3). Zeilenangaben beziehen sich auf `main` `5e0e63d`, Vertrag 1.6.0. **Wo dieser
Abschnitt etwas anderes sagt als die Abschnitte 4 bis 9, gilt Abschnitt 11.** Der Bau beginnt erst
auf Marcos ausdrückliches Wort und läuft zusammen mit Konzept 11 (11.8). Der OMC-Kritiker hat
diesen Abschnitt geprüft (Abschnitt 12, Fassung 2 → 3); die Befunde sind eingearbeitet.

### 11.1 Was sich gegenüber Fassung 2 ändert

- **Abschnitt 9 gilt nicht mehr ganz:** Marcos Wünsche bringen eine neue Funktion (Statistik
  «Wann geschaut wird», 10.2) und Bildfelder (Streams, Statistik, Sammlungen). Der Vertrag
  bekommt darum **additive** Felder (11.4). Eine Migration braucht es nicht.
- **Neue Regel S-16 «Reihenfolge durch Platzwahl»** (aus Marcos Wunsch an Metadaten, 10.2):
  Wo eine Liste geordnet wird, wählt man den Platz in einer Auswahl («1. Sprache», «Stelle»);
  die anderen rücken nach oder tauschen. Kein «Nach oben»/«Nach unten» und kein Ziehen. Gilt für
  Metadaten (Sprachen, Bildsprachen), Bibliotheken (Anbieterkette im Ordnerdialog der Verwaltung),
  Startseite, Cat's Empfehlungen und Sammlungen. Die Texte `moveUp`/`moveDown` dieser Seiten fallen
  weg.
- **Neue Regel S-17 «Entwürfe gehören der Seite»:** Ungespeicherte Eingaben liegen im Zustand
  der Seite, geschlüsselt nach der Id des Gegenstands (Jobart, Konto, Empfehlung, Sammlung,
  Geltung), nicht in einem Reiter oder einer Klappzeile. So bleiben sie beim Reiterwechsel, beim
  Zuklappen und beim Wechsel der Auswahl erhalten (S-13), und derselbe Gegenstand hat in zwei
  Reitern denselben Entwurf. Reiter dürfen darum aushängen wie heute (`ui/Tabs.tsx:8`); ein
  `keepMounted` braucht es nicht. Zwei Arten von Reitern: **Filterreiter** zeigen eine Liste mit
  Filter (Bibliotheken nach Art, Podcasts je Bibliothek, Empfehlungen, Sammlungen); **Bereichsreiter**
  zeigen verschiedene Inhalte (Hintergrundjobs, Pflege, Titel, Konten, Geräte, Startseite,
  Sicherung). Die Vorschau der Startseite zeigt die ungespeicherte Reihenfolge.
- **Ausserhalb der Verwaltung** ändern sich nur Teile ohne sichtbare Folge: der Inhalt des
  Ankündigungsdialogs der Zuschauer wird als eigener Baustein herausgelöst (für die Vorschau
  auf «Ankündigungen»), und `ui/ProviderList` bekommt eine Platzwahl als Option, die nur der
  Ordnerdialog der Verwaltung einschaltet; der Einrichtungsassistent bleibt wie er ist.
- **Seitentitel bleiben, wie sie heute in `de.json` stehen:** «Startseite» (`de.json:1335`,
  Leiste ebenso `:1528`), «Statistiken» (`:2063`, Leiste `:1548`) und «Benachrichtigungskanäle»
  (`:2985`, Leiste «Benachrichtigungen» `:1544`). Die Mockups zeigen «Startseitenreihen»; gebaut
  wird «Startseite». Die Notiz zu den Seitentiteln in 10.3 (Phase 2c und 2d) ist damit erledigt:
  nur Benachrichtigungen weicht ab, und das ist gewollt (Seite gilt für jedes Konto).
- **Nummern:** Frage 18 und ADR 0020 (wie im Kopf). Die Vertragsfelder kommen in **1.8.0**, als
  eigenes Release (O-1, beantwortet am 2. November 2026): Konzept 11 ist allein als 1.7.0
  erschienen. Die Abschnitte 10.2 und 10.3 sind darauf nachgeführt.

### 11.2 Bausteine in `web/src/ui`

Je Baustein eine Story mit Referenzbild. Bausteine sind darstellend; Daten lesen nur die Seiten
in `features/`.

| Baustein | Zweck | Gebraucht von | Heute |
|---|---|---|---|
| Komponententokens, Gruppe `density.compact` | Masse aus 5.1, Standard = heutiger Wert (ADR 0020, 11.6) | alle | keine Dichte im Tokensystem; Panel-Abstand fest in `Panel.module.css:7` |
| `Text` Variante `pageTitle` | h1 nach dem Token `--sc-page-title-size`: Standard heutige Grösse (`display`), kompakt 24 px, 600 (S-2). `/settings/*` und die Zwischenstände K12-3 bis K12-7 zeigen darum die heutige Grösse | alle | Varianten `display`, `title`, `body`, `caption` (`ui/Text.tsx:4-11`) |
| `AdminPage` | Kopf in einer Zeile (h1, Hilfe-Knopf mit dem heutigen Einleitungssatz, Live-Anzeige, Hauptaktionen), Zustandsleiste, Inhalt; Kopf mit `tabIndex={-1}` für ALL-2 | alle Verwaltungsseiten und die beiden `/settings/*`-Seiten | jede Seite baut ihren Kopf selbst (15 CSS-Module) |
| `StatusStrip` | Reihe kompakter `StatTile` (56 px, S-4) | Hintergrundjobs, Konten, Streams (Serverlast), Statistik (Wiedergabe) | `ui/StatTile.tsx` |
| `BandPanel` | Feld mit getöntem Kopfband (Profilfarbe zu 12 %, dieselbe `--sc-band-tint`-Technik wie das Band der Leiste aus K11-4, 11.6 dort), wahlweise auf- und zuklappbar (`aria-expanded`), wahlweise mit Reiterleiste im Band. Der Kontrasttest aus Konzept 11 prüft zusätzlich 12 % | Integrationen, Spotlight, Benachrichtigungen, Statistik, Streams, Ankündigungen, Konten; Reiter im Band: Geräte, Podcasts, Startseite, Empfehlungen, Sammlungen, Sicherung | `ui/Panel.tsx` ohne Kopf |
| `Tabs` erweitert | Zahl und Ton je Reiter, Anbindung an `?tab=`; inaktive Reiter hängen weiter aus (S-17) | Hintergrundjobs, Bibliotheken, Pflege, Titel, Konten, Geräte, Podcasts, Startseite, Empfehlungen, Sammlungen, Sicherung | hängt inaktive Reiter aus (`ui/Tabs.tsx:8,40-44`) |
| `DataTable` mit Klappzeilen, `DisclosureGroup` | Klappzeile **in der Tabelle**: die erste Zelle ist ein Knopf mit `aria-expanded` und `aria-controls`, der Inhalt steht in einer eigenen Zeile `<tr><td colSpan>` darunter; Tabelle, Spaltenköpfe und die Rollen `row`/`cell` der heutigen Specs bleiben (`ui/DataTable.tsx:62-83`). Kein react-aria `Disclosure` in Tabellen (ein `div` gehört nicht in `<tr>`). `DisclosureGroup` (Klappbereich je Bibliothek, Typ, Art) ist ein Abschnitt mit Überschrift als Knopf und eigener Tabelle; dort darf react-aria `Disclosure` (`react-aria-components` 1.22.1) benutzt werden. Prüfung mit Bildschirmleser in K12-2 | Hintergrundjobs, Bibliotheken, Pflege, Titel, Konten, Geräte, Podcasts, Startseite, Empfehlungen, Sammlungen, Sicherung, Statistik | keiner |
| `MasterDetail` | Liste links (ein Tab-Halt, Pfeil hoch/runter, Enter), Detail rechts, Auswahl in `?item=` (S-14) | Metadaten (Bereiche), Ankündigungen | keiner |
| `SettingRow` | Bezeichnung und Hilfe links, Steuerelement rechts (S-7) | Metadaten, Integrationen, Spotlight, Benachrichtigungen | keiner |
| `SaveBar` | «Ungespeicherte Änderungen» / «Gespeichert», Verwerfen, Speichern; aus `MetadataPage.tsx` gelöst | Metadaten, Startseite | nur in `MetadataPage` |
| `RankSelect` | Platzwahl nach S-16 (verschieben oder tauschen, Hinweis darunter, `aria-describedby`) | Metadaten, Startseite, Empfehlungen, Sammlungen; in `ui/ProviderList` als Option nur für den Ordnerdialog der Verwaltung | keiner |
| `RankedChoiceList` (neu, 3. November 2026, K12-3) | Reihenfolge durch feste Plätze (S-16): je Platz eine Auswahl, eine schon belegte Wahl tauscht die Plätze, «Weitere Sprache» und «×», Prüfung `isAllowed(list)` sperrt Wahlen. Setzt die Änderungswünsche aus 10.2 um; `RankSelect` bleibt für die anderen Seiten | Metadaten (Sprachen, Bildsprachen) | keiner |
| `SelectCheck` (neu, 3. November 2026, K12-4) | Auswahl-Kästchen für eine Tabellenzeile (Tab, ↓, Space). Ersetzt `SelectableList`, der Baustein ist entfallen | Pflege | `ui/SelectableList` |
| `ActionMenu` | Menü «Weitere Aktionen» (S-6), Abschnitt «Gefahr», Fokus zurück auf den Knopf | Sicherung, Profile im Konto, Zeilen mit mehr als zwei Aktionen | `MenuButton` ist ein Auswahlmenü (`ui/Menu.tsx:40-42,132-146`) |
| `ConfirmDialog` steuerbar | `isOpen`/`onOpenChange`, damit Menü und Klappzeilen eine Rückfrage öffnen | alle Seiten mit `ActionMenu` | nur über `trigger` (`ui/Dialog.tsx:15-26,43`); `FormDialog` ist schon steuerbar (`:228`) |
| `AdminDensityScope` | Kontext, den `AdminPage` setzt; Dialoge aus einer Verwaltungsseite tragen damit `data-density="compact"` an ihrem eigenen Portal. Andere Portale auf `/admin` (Schublade der App, Ankündigungsdialog der Zuschauer) bleiben in heutiger Grösse | alle Dialoge der Verwaltungsseiten | keiner |
| `useDrafts` | Entwürfe je Gegenstand-Id im Zustand der Seite (S-17), meldet «geändert» je Id und gesamt, Verwerfen je Id | alle Seiten mit Formularen | keiner |
| `useUnsavedChanges` | S-13: TanStack `useBlocker` (`@tanstack/react-router` 1.170.45) fragt beim Verlassen der Seite; ein Wechsel nur von `?tab=` oder `?item=` auf demselben Pfad blockiert nicht (S-17 hält die Entwürfe); pausiert das Abfragen | Metadaten, Integrationen, Spotlight, Benachrichtigungen, Konten, Empfehlungen, Sammlungen, Startseite, Titel (Freigabe) | `useBlocker` wird nirgends benutzt |
| `StreamTile`, `StreamProgress` (neu, 3. November 2026, K12-5) | Kachel eines laufenden Streams mit Hintergrundbild quer, Modus-Pille, Titel, Profil, Gerät, Qualität und «Beenden»; `StreamProgress` ist der Fortschritt allein und wird von `StreamRow` der Übersicht mitgenutzt (nur der Fortschritt, nicht die Kachel) | Streams; `StreamProgress` auch Übersicht | `ui/StreamRow` mit eigenem Fortschritt |
| `ProfileList` (neu, 3. November 2026, K12-5) | Profilliste mit PIN und Löschen im Menü, aus `ProfileSettingsPage` herausgelöst | Konten (`?item=me`), `/settings/profiles` | in `ProfileSettingsPage` |
| `RadioGroup` und `TextField` mit `hideLabel`, `RadioGroup` mit `inline` (neu, 3. November 2026, K12-6 und K12-7) | `hideLabel`: die Beschriftung bleibt Name des Steuerelements, steht aber nur einmal links in der `SettingRow`; `inline`: Beschriftung und Segmente in einer Zeile | Spotlight; `inline` Statistik | keine |
| `BarChart` mit `tickEvery` (neu, 3. November 2026, K12-7) | beschriftet nur jeden n-ten Balken (Stunden alle 3 h) | Statistik | alle Balken beschriftet |
| `Tabs` mit `isDisabled` (neu, 3. November 2026, K12-7) | sperrt die Reiterleiste, solange eine Wiederherstellung läuft | Sicherung | keine |
| `ImageTile` | Bild quer oder hoch mit Rückfall `PosterFallback` (Farbfläche mit Titel) | Streams, Statistik, Sammlungen | `PosterFallback` vorhanden |
| `adminSearch` (`web/src/app/router.tsx`) | gemeinsame `validateSearch`-Hilfe für `?tab=` und `?item=` nach dem Muster von `:385` und `:412`; bestehende Parameter bleiben | alle Seiten mit Reitern oder Auswahl | je Route eigene Prüfung |
| `BarChart` erweitert, `HeatGrid` neu | waagrechte Balken (Wochentage), Raster 7 × 24; beide mit Tabelle für Bildschirmleser wie heute `BarChart` | Statistik | `ui/BarChart.tsx` senkrecht mit Tabelle |

**Texte (Regel 9):** neue Schlüssel je Deutsch und Englisch für Hilfe-Knopf, Live-Anzeige,
Platzwahl-Hinweis, «Weitere Aktionen» und «Gefahr», ungespeicherte Änderungen, Reiternamen,
Beschreibungen der Startseitenreihen, «Wann geschaut wird» mit Wochentagen und «Aus den letzten 12
Wochen» und «Aus den letzten 4 Wochen» (geändert 2. November 2026, O-4), Rückfrage «Verknüpfung lösen», Gerätetyp «Unbekannt», `admin.jobs.types.metadata_apply`
mit `.name` und `.trigger` (`JobsPage.tsx:539`). Nicht mehr gebrauchte Schlüssel werden entfernt.

**Abgrenzung zu Konzept 11:** `data-shell="admin"`, die Band-Technik und `RestartPanel` kommen
aus Konzept 11 (11.6, 11.5) und werden hier benutzt, nicht neu gebaut. Die Stream-Kachel dieser
Seite und `StreamRow` der Übersicht teilen Bild, Modus-Pille und Fortschritt als gemeinsame
Teile.

### 11.3 Seiten

Je Seite ein Commit. «Daten» nennt nur, was sich am Vertrag ändert; sonst bleiben alle Aufrufe
wie heute. Die Erhaltungsliste (Abschnitt 7) gilt vollständig (E-2).

| Seite | Mockup | Aufbau beim Bau | Daten und Hinweise |
|---|---|---|---|
| Hintergrundjobs | `Jobs-C` | Hinweis pausierter Jobarten (JOB-3); Zustandsleiste mit fünf Zahlen (JOB-1); Klappbereich «Läuft gerade» mit Arbeiter-Pools (JOB-2, JOB-4, JOB-4a); Reiter Bibliothek, Metadaten, Zustellung, System mit Zahl und Reiter «Fehlschläge» (JOB-8); je Jobart eine Klappzeile (JOB-5) mit Aktionen (JOB-6), Zeitplan als Block in der Zeile statt `JobScheduleDialog`, Läufe als Unterreiter statt `RunsDialog` (JOB-7) | Keine Vertragsänderung. Zeitplan **nur bei Jobarten mit `schedule`** (JOB-6); das Mockup zeigt den Block bei allen, das gilt nicht. Text `admin.jobs.types.metadata_apply` ergänzen (10.3). Der Reiter «Fehlschläge» trägt die Gesamtzahl `JobCounts.failed` und listet die zehn aus `recentFailed`. Der Link der Übersicht heisst `?tab=failed` (S-14); die Karte «Aufgaben» aus Konzept 11 bekommt ihn in K11-5. |
| Metadaten | `Met-B` | Geltung als Auswahl im Kopf (`?library=`), Bereiche links (Sprachen, Bilder, Güte, Quellen, Altersfreigabe, Anwenden), eine `SaveBar` für alle Geltungen; Änderungen je Geltung bleiben beim Wechsel erhalten (S-13, S-17) | Keine Vertragsänderung. **Speichern** schickt die geänderten Geltungen nacheinander: zuerst den Standard (`updateMetadataSettings`), dann die Bibliotheken in Listenreihenfolge (`updateLibraryMetadata`, `openapi.yaml:3573,3641`). Die Meldungen mit der Zahl betroffener Titel (MET-6) erscheinen je Geltung untereinander. Schlägt ein Aufruf fehl, hält das Speichern an; die fehlgeschlagene und alle folgenden Geltungen bleiben geändert, der Fehler nennt die Geltung. **Verwerfen** setzt alle geänderten Geltungen zurück und nennt sie in der Rückfrage. Sprachen und Bildsprachen mit `RankSelect` (10.2). |
| Integrationen | `Int-A` | sechs auf- und zuklappbare `BandPanel`, alle offen; Speichern je Zeile oder Block wie heute; Pfad-Umsetzung je Webhook als Klappbereich | Keine Vertragsänderung. |
| Bibliotheken | `Bib-C` | Filterreiter nach Art mit Zahl; je Bibliothek eine Klappzeile mit Aktionen, Fortschritt, Fehlerband und Ordnern; «Bibliothek hinzufügen» als Dialog (heute ein Bereich in der Seite, `LibrariesPage.tsx:76,360`); Ordnerdialog wie heute (`LibraryFoldersDialog`, Bestätigung durch Eintippen des Pfads) | Keine Vertragsänderung. Anbieterkette mit `RankSelect` (S-16) nur im Ordnerdialog der Verwaltung. Live-Anzeige nur während eines Scans. |
| Pflege | `Pfl-C` | Reiter je Problemart mit Zahl (PFL-1); je Reiter Werkzeugzeile und ein Klappbereich je Bibliothek; Duplikate als Gruppen | Keine Vertragsänderung: `Issue.libraryId` ist Pflicht (`openapi.yaml:9094-9119`), Namen aus der Bibliotheksliste. Gruppiert werden die **geladenen** Einträge; «Mehr laden» und «X von Y» bleiben je Reiter (50 je Seite, O-3). **Duplikate** werden nicht nach Bibliothek gruppiert (eine Gruppe kann mehrere Bibliotheken umfassen, `libraryId` ist dort optional, `openapi.yaml:3862-3863,9156`); jede Gruppe ist ein Klappbereich wie im Mockup. |
| Titel | `Tit-C` | Reiter Metadaten, Bilder, Dateien, Trailer; Freigabe und Herkunft als Klappbereiche; Bildwahl im Dialog wie heute | Keine Vertragsänderung. |
| Konten mit Profilen | `KP-1` | Zustandsleiste mit vier Zahlen; Reiter Konten und Einladungen; je Konto eine Klappzeile; Zugriff eines Benutzerkontos inline mit eigenem Speichern (heute Dialog); im eigenen Konto «Konto ändern» inline (heute `AccountDialog`, `AccountsPage.tsx:59-154`) und «Meine Profile»; Profil anlegen und bearbeiten im Dialog wie heute (`ProfileSettingsPage.tsx:235-248`) | Keine Vertragsänderung (`/profiles` bleibt `x-visibility: account`). `ProfileForm` und `WatchReport` sind schon eigene Dateien; nur die Profilliste zieht aus `ProfileSettingsPage.tsx` in eine Komponente, die `/settings/profiles` und Konten gemeinsam benutzen. **Leisten-Eintrag «Profile»** (in K12-5): Ziel `/admin/accounts?item=me#meine-profile`; `me` steht für das eigene Konto, die Seite löst es auf, die Leiste braucht keine Konto-Id. `isSectionPath` (`sections.ts:70-71`) bekommt für diesen Eintrag die Bedingung `item=me`, damit «Profile» und nicht «Konten» aktuell ist. Angepasst werden `sections.ts:51`, `AdminShellNav` aus Konzept 11 und dessen Tests (`admin-home.spec.ts:213-215`, `AdminHomePage.test.tsx:384`). **`/settings/profiles` bleibt für alle Konten erreichbar, auch für Administratoren; keine Weiterleitung** (`profiles.spec.ts:23`, `watch-limit.spec.ts:81` laufen als Besitzer). |
| Geräte | `Ger-C` | Reiter Geräte und Fähigkeiten im Band; je Gerätetyp ein Klappbereich | Keine Vertragsänderung; Geräte ohne `type` (`openapi.yaml:8454`, optional) unter «Unbekannt». |
| Streams | `Str-D2` | `BandPanel` «Serverlast» mit vier Zählern und Bandbreite (Summe von `AdminStream.bitrate`); Kacheln mit Hintergrundbild, Profil, Gerät, Qualität, Fortschritt, «Beenden» mit Rückfrage | **`AdminStream.backdropUrl`** (11.4). Live alle 5 s, pausiert bei offener Rückfrage (`StreamsPage.tsx:57`). |
| Podcasts | `Pod-C` | Filterreiter «Alle» und je Podcast-Bibliothek; je Podcast eine Klappzeile | Keine Vertragsänderung (`AdminPodcast.libraryId`, `openapi.yaml:8369`). |
| Startseite | `Rei-C` | Reiter Reihenfolge und Vorschau; je Reihe Klappzeile mit Beschreibung und Link; Platz mit `RankSelect`, Ausblenden; `SaveBar` mit Verwerfen | Keine Vertragsänderung (`setHomeRows`, `openapi.yaml:4790`). h1 bleibt «Startseite». Die Vorschau ist schematisch und braucht keine Daten. Beschreibungen je Reihe sind neue Texte in `de.json`/`en.json`. |
| Spotlight | `Spo-A` | drei `BandPanel` (Quelle, Einträge, verwaiste Anpassungen); ein Speichern für das Quellformular mit «Aktiv» (SPO-1, SPO-2) | Keine Vertragsänderung. Anheften im Dialog mit Platzraster bis «Höchstens». |
| Cat's Empfehlungen | `Pic-C` | Filterreiter Alle, Aktiv, Geplant, Abgelaufen (im Client aus `startsAt`/`endsAt`; «Aktiv» statt «Sichtbar» im Mockup, weil es nur die Zeit meint, Regel 12); je Empfehlung Klappzeile mit Notiz, Zeitraum und Stelle, inline gespeichert (heute Dialog); Hinzufügen im Dialog, `?add=` wie heute | Keine Vertragsänderung: die Stelle ruft `reorderTopPicks` mit der neuen Reihenfolge auf (`openapi.yaml:4460-4466`); `AdminPick.posterUrl` gibt es schon. |
| Sammlungen | `Sam-C` | Filterreiter Alle, Manuell, TMDB; je Sammlung Klappzeile mit Mitgliedern inline (heute Dialog), Suche, `RankSelect`, Speichern, Verwerfen; Umbenennen im Dialog; TMDB gesperrt | **`AdminCollectionItem.posterUrl`** (11.4). |
| Ankündigungen | `Ank-B` | `MasterDetail`: links Entwürfe und Veröffentlicht, rechts Angaben, gelesen «n von m Profilen» und die Vorschau «So sehen es die Profile»; Erstellen und Bearbeiten inline (heute Dialog) | Keine Vertragsänderung (`acknowledgedCount`, `profileCount`, `openapi.yaml:11960`). Die Vorschau benutzt den Inhalt von `AnnouncementDialog` (`features/notifications/AnnouncementDialog.tsx`), der dafür als Baustein herausgelöst wird, ohne Knöpfe; die Zuschauerseite sieht gleich aus (`Announcements.test.tsx` läuft weiter). |
| Benachrichtigungen | `Ben-A` | `/settings/notifications`, heutige Dichte (S-1); drei `BandPanel` (E-Mail, Telegram, Web-Push); Speichern und Test je Kanal | Keine Vertragsänderung: gespeichert wird schon je Kanal (`updateMyChannel`, `openapi.yaml:6182-6184`). «Verknüpfung lösen» bekommt eine Rückfrage (`deleteTelegramLink`, `:6310`). |
| Statistik | `Sta-B` | Zeitraum im Kopf (`?days=`); Streifen «Wiedergabe»; Meistgesehen mit Platz 1 bis 3 als Bildkacheln und Tabelle ab Platz 4; Schauzeit je Profil und Geräte nebeneinander; «Wann geschaut wird» (10.2) | **`StatTitle.posterUrl`, `StatTitle.backdropUrl`, `Stats.watchTimeByQuarterHour`, `Stats.timeWindowDays`, `Stats.timeZone`** (11.4). Bei 90 und 365 Tagen steht unter «Wann geschaut wird» «Aus den letzten 12 Wochen», bei 30 Tagen «Aus den letzten 4 Wochen» (O-4, geändert 2. November 2026; vorher «Aus den letzten 90 Tagen»). |
| Sicherung | `Sic-C` | Reiter Sicherungen, Neue Sicherung, Aus Datei wiederherstellen; Klappbereiche Manuell und Automatisch; Zeile mit «Herunterladen» und `ActionMenu` (Wiederherstellen, Löschen); Passphrase einer verschlüsselten Sicherung im Dialog; Warteanzeige `RestartPanel` (Konzept 11, 11.5) | Keine Vertragsänderung. Der Satz zu automatischen Sicherungen heisst «vor einer Migration der Datenbank (bei einem Update) und vor einer Wiederherstellung» (`store/migrate.go`, `store/restore.go`). |

### 11.4 Vertrag (additiv)

Nichts wird entfernt oder im Typ geändert.

| Ort | Neu | Herkunft im Server |
|---|---|---|
| `AdminStream.backdropUrl` | optional, Hintergrundbild `w780` mit Hash | Wie `AdminStream.posterUrl` aus Konzept 11 (11.3), bei Folgen das Bild der Serie (die Serien-Id liefert `AdminStreamItems` nach Konzept 11). `AdminListPosters` (`server/queries/system/admin.sql:374-381`) wird zu `AdminListImages` mit Art und Grösse verallgemeinert; `posterHashes` (`server/internal/admin/images.go:74`) bekommt eine Schwester für `backdrop`. URL über `adminImageURL` (`server/internal/api/item_handlers.go:441-443`). |
| `StatTitle.posterUrl`, `StatTitle.backdropUrl` | optional | Für die höchstens 20 `topTitles`. `StatsTopTitles` gruppiert nach dem gespielten Titel (`server/queries/system/admin_stats.sql:15-35`), also auch nach Folgen, die kein Poster haben. Die Abfrage liefert darum zusätzlich die Id der Serie (Folge → Staffel → Serie über `parent_id`), und für Folgen gelten Poster und Hintergrundbild der Serie, wie bei `AdminStream`. |
| `AdminCollectionItem.posterUrl` | optional, `w200` | `withPosters` (`images.go:90`) ist auf `[]model.AdminPick` getippt und wird für beliebige Listen mit Item-Id verallgemeinert; dann für die Mitglieder (`server/queries/system/collections.sql:47-50`, `collections.go:219`). |
| `Stats.watchTimeByQuarterHour` | Pflicht, genau 672 Werte `watchedMs` (7 Wochentage × 96 Viertelstunden, Montag 00:00 zuerst), gerechnet über **ganze Wochen** (`timeWindowDays`, geändert 2. November 2026) | Neue Abfrage `StatsWatchTimeByQuarter` über `progress_events`: `SUM(played_ms)` gruppiert nach `server_time / 900000` (UTC-Viertelstunde, Unix-ms, `migrations/00004_playback_progress.sql:35`). Viertelstunden statt Stunden, damit auch Zonen mit halben oder Viertelstunden-Versatz stimmen. Offline gespielte Ereignisse tragen schon die richtige Spielzeit (`server/internal/playback/offline.go:61`). Go ordnet jede Viertelstunde über die Zeitzone des Servers einem Wochentag und einer Uhrzeit zu (`TZ`, `deploy/compose.yaml:31`; `tzdata` im Image, `deploy/Dockerfile:96`). Die Zeitzone kommt als Option in `admin.Service`, damit Tests sie festlegen; `admin.ResolveZone` löst sie beim Start einmal über den IANA-Namen auf (`TZ`, sonst das Ziel von `/etc/localtime`, sonst UTC), damit `Stats.timeZone` und das Raster dieselbe Zone nennen (geändert 2. November 2026; `time.Local` heisst nur «Local»). Die Oberfläche fasst zu Stunden zusammen und summiert Zeilen und Spalten für «Nach Tageszeit» und «Nach Wochentag». |
| `Stats.timeWindowDays` | Pflicht, Zahl der Tage, über die `watchTimeByQuarterHour` rechnet: **ganze Wochen, höchstens 84** (7, 28 oder 84) | `min(days, 90)` abgerundet auf ein Vielfaches von 7 (geändert 2. November 2026, Marcos Entscheid): 7 → 7, 30 → 28, 90 und 365 → 84. Nach 90 Tagen fasst der Server Ereignisse zu Tagen zusammen (`progress_days`, `migrations/00019_progress_days.sql:37-45`), die Uhrzeit ist dann weg. Ganze Wochen, damit jeder Wochentag gleich oft zählt; sonst wirkte ein Wochentag, der 5-mal statt 4-mal im Fenster liegt, 25 % stärker. |
| `Stats.timeZone` | Pflicht, IANA-Name (zum Beispiel `Europe/Zurich`) | `time.Local.String()` bzw. die Option; die Oberfläche nennt die Zone unter dem Diagramm. |

### 11.5 Abweichungen (gehen in Frage 18)

1. **Profile in Konten (KP-1):** Für Administratoren verwaltet «Konten» die eigenen Profile; der
   Leisten-Eintrag «Profile» zeigt dorthin (Abweichung von Konzept 11, 11.1 Punkt 4 und O-3).
   `/settings/profiles` bleibt für Konten ohne Verwaltung (PRO-0).
2. **Fehlschläge aller Jobarten (JOB-8)** stehen als Reiter auf «Hintergrundjobs»; das schliesst
   die Lücke, die Konzept 11 F-2 auf der Übersicht lässt.
3. **Neue Funktion «Wann geschaut wird»** auf «Statistik» (Marco, 1. November 2026) mit den
   Feldern aus 11.4; über Spec A-8 hinaus, kein Nicht-Ziel (Regel 11).
4. **Bildfelder** für Streams, Statistik und Sammlungen (11.4), damit die Kacheln Bilder statt
   Farbflächen zeigen (Marcos Wahl «Hintergrundbild quer»).
5. **Neue Rückfrage** vor «Verknüpfung lösen» (Telegram); heute löst ein Klick sofort.
6. **Startseite:** «Verwerfen», Beschreibung je Reihe und Reiter «Vorschau» sind neu in der
   Oberfläche, ohne neue Funktion.
7. **Ankündigungen:** Vorschau «So sehen es die Profile» und Lesefortschritt als Balken, aus
   vorhandenen Daten.
8. **Vom Dialog in die Seite** (E-3): Zeitplan einer Jobart (`JobScheduleDialog`), Läufe einer
   Jobart (`RunsDialog`, `JobsPage.tsx:717-775`), Zugriff eines Kontos, «Konto ändern»
   (`AccountDialog`, `AccountsPage.tsx:59-154`), Bearbeiten einer Empfehlung, Mitglieder einer
   Sammlung, Erstellen und Bearbeiten einer Ankündigung, Neue Sicherung und Wiederherstellen aus
   Datei (Reiter). **Von der Seite in einen Dialog:** «Bibliothek hinzufügen» (BIB-6, heute ein
   Bereich in der Seite, `LibrariesPage.tsx:76,360`). Alle Felder und Rückfragen bleiben.
9. **Reihenfolge durch Platzwahl (S-16)** statt «Nach oben»/«Nach unten» auf fünf Seiten.
10. **Kompakte Dichte** unter `/admin/*` mit Maus ab 768 px (Abschnitt 5, ADR 0020); sie gilt
    auch für die Übersicht `/admin` aus Konzept 11, deren Aufbau gleich bleibt (Abweichung von
    Abschnitt 9, zweiter Punkt).
11. **Link zu den Fehlschlägen** `?tab=failed` (JOB-8 nannte `?section=failed`); die Karte
    «Aufgaben» der Übersicht bekommt ihn in K11-5. Der Satz in Konzept 11, 11.3, die eigene
    Oberfläche lese `recentFailed` nicht mehr, wird in K12-0 berichtigt: «Hintergrundjobs» liest es
    (JOB-8).
12. **Profile:** der Leisten-Eintrag führt nach Konten (`?item=me`), `/settings/profiles` bleibt für
    alle Konten.
13. **Cat's Empfehlungen:** Reiter «Aktiv» statt «Sichtbar» (Mockup).
14. **K12-3 und K12-4 (3. November 2026):** die angenommenen Abweichungen der Seiten
    Metadaten, Hintergrundjobs, Integrationen, Bibliotheken, Pflege, Titel und Podcasts stehen in
    Frage 18, «Nachtrag 2 vom 3. November 2026».
15. **K12-5 bis K12-7 (3. November 2026):** die Entscheide der Koordination und die angenommenen
    Abweichungen der Seiten Geräte, Streams, Konten, Startseite, Sammlungen, Spotlight, Cat's
    Empfehlungen, Statistik, Sicherung und Benachrichtigungen (und der Pflege-Ton aus K12-4)
    stehen in Frage 18, «Nachtrag 3 vom 3. November 2026».

### 11.6 ADR 0020 «Kompakte Dichte der Verwaltung»

Inhalt: Komponententokens mit dem heutigen Wert als Standard; Gruppe `density.compact`; wirksam
im Inhaltsbereich und in Dialogen unter `[data-shell="admin"]` mit `(min-width: 48rem) and
(pointer: fine)`; Touch und Handy behalten 44 px (WCAG 2.5.8 bleibt erfüllt). **Geprüfte
Alternativen:** eigene Admin-Kopien der Bausteine (verstösst gegen Regel 9), die allgemeine Skala
umstellen (schrumpft Fliesstext und Chips überall), CSS-`zoom` (bricht Fokusringe und
Trefferflächen). **Folgen:** der Commit mit den Komponententokens (K12-2a) ändert keine Story, und alle
Referenzbilder bleiben pixelgleich (Beweis, dass ausserhalb von `/admin` nichts kleiner wird);
erst die Erweiterungen (K12-2b) ändern Bilder. Jeder Baustein bekommt eine Story in beiden Dichten
(Dekorator `density`), damit die kompakte Dichte eine Bildprüfung hat; Seiten haben keine eigenen
Referenzbilder. Neue Verwaltungsseiten bekommen die Dichte ohne eigenes CSS.

### 11.7 Tests

- **Server:** Vertragstest (`TestContractOpenAPI`) mit den Feldern aus 11.4; Handlertests für
  `backdropUrl`, `posterUrl` und die Statistik; Abfragetest `StatsWatchTimeByQuarter` (geändert 2. November 2026; nicht `ByHour`); Test der
  Zuordnung mit fester Zone `Europe/Zurich` über beide Zeitumstellungen (29. März und 25. Oktober
  2026), mit `Asia/Kolkata` (halbe Stunde) und mit 365 Tagen (`timeWindowDays` = 84); Test von
  `admin.ResolveZone` (`TZ`, `/etc/localtime`, ungültiger Name, kopierte Zonendatei) und der
  Fenster 7, 28 und 84 Tage; Handlertest
  einer Folge in `topTitles` mit den Bildern der Serie; `server/internal/admin/stats_bench_test.go`
  um die neue Abfrage erweitert; Statistik-Attrappen der Web-Tests um die Pflichtfelder ergänzt.
- **Unit (web):** jede Seitentest-Datei wird auf den neuen Aufbau umgeschrieben und behält jede
  geprüfte Funktion (die Erhaltungsliste ist die Prüfliste): `LibrariesPage`,
  `LibraryFoldersDialog`, `PodcastsPage`, `MaintenancePage`, `AdminItemPage`, `MetadataPage`,
  `AccountsPage`, `ProfileSettingsPage`, `ProfileLimits`, `DevicesStreamsPages`, `SpotlightPage`,
  `PicksPage`, `CollectionsPage`, `HomeRowsPage`, `AnnouncementsPage`, `JobsPage`,
  `IntegrationsPage`, `WebhooksSection`, `ChannelSettingsPage`, `StatsPage`, `BackupPage`. Neu:
  Tests der Bausteine (`RankSelect` verschieben und tauschen, Entwurf bleibt beim Reiterwechsel
  (S-17), `useUnsavedChanges` fragt beim Verlassen und nicht beim Wechsel von `?tab=`,
  `ActionMenu` gibt den Fokus zurück, Klappzeile mit Enter und Leertaste, `AdminDensityScope`
  an Dialogen) und der Text für `metadata.apply`. Ausserdem: `ProviderList.test.tsx`,
  `FolderList.test.tsx` (Platzwahl als Option, Standard unverändert), `Announcements.test.tsx`
  (herausgelöster Inhalt), `AdminHomePage.test.tsx` (Leisten-Eintrag «Profile», Link
  `?tab=failed`).
- **e2e (Regel 8):** alle Verwaltungs-Specs werden angepasst, keiner gelöscht: `admin-libraries`,
  `admin-library-folders`, `admin-folder-browser`, `admin-podcasts`, `podcast`, `maintenance`,
  `admin-item`, `admin-metadata`, `admin-accounts`, `invitations`, `profiles`, `watch-limit`,
  `admin-devices`, `admin-streams`, `admin-spotlight`, `admin-picks`, `admin-collections`,
  `admin-home-rows`, `admin-announcements`, `admin-jobs`, `admin-integrations`,
  `admin-webhooks`, `notification-channels`, `admin-stats`, `admin-backup`. Je Seite geprüft:
  Reiter mit Pfeil links/rechts, Klappzeilen mit Enter, Menü mit Pfeiltasten und Esc, Liste von
  `MasterDetail` mit Pfeil hoch/runter, `RankSelect` mit Tastatur, die Tabelle hinter `HeatGrid`
  für Bildschirmleser, Rückfrage
  beim Verlassen mit Ungespeichertem, Fokus nach dem Entfernen auf den Seitenkopf (ALL-2,
  `expectFocusOn` in `web/e2e/keyboard.ts:44-45`), sichtbarer Fokus, keine Fokusfalle.
  `profiles.spec` bleibt für `/settings/profiles`; die Profilfälle für Administratoren kommen
  neu in `admin-accounts`. `admin-home.spec.ts` (Leiste, Konzept 11) und `setup.spec.ts`
  (Einrichtungsassistent unverändert) laufen mit.
- **Storybook-Referenzbilder:** nach K12-2a alle bestehenden Bilder unverändert (11.6); in
  K12-2b neu für jeden Baustein aus 11.2 in beiden Dichten, geändert `ui-tabs--*`,
  `ui-barchart--*`, `ui-confirmdialog--*`, `ui-datatable--*` und neu `ui-providerlist--rank`.
- **Abnahme je Seite:** `verifier` hakt die Erhaltungsliste ab (E-2), `designer` vergleicht mit
  dem gewählten Mockup.

### 11.8 Schnitt in Schritte (zusammen mit Konzept 11)

Konzept 11 hat die Schritte 0 bis 6 (11.8 dort); dieses Konzept hängt seine Schritte K12-0 bis
K12-9 so ein, dass die Seiten erst nach dem Gerüst umgebaut werden. Jeder Schritt ein Commit
(Seiten: je Seite ein Commit), `task ci` grün inklusive e2e, DoD-Belege in
`docs/dev/CURRENT_STATUS.md`.

```
K11-0 + K12-0  Papiere (Frage 17 und 18, ADR 0019 und 0020)
K11-1          Vertrag 1.7.0: Dashboard (Konzept 11)
K12-1          Vertrag 1.8.0: Bilder und Statistik
K11-2, K11-3   Aktivitätsprotokoll, Neustart
K11-4 → K12-2  ui-Bausteine: zuerst Konzept 11 (Band), dann dieses (Dichte noch aus)
K11-5          Gerüst, Leiste, Übersicht
K12-3 … K12-7  Seiten je Gruppe, heutige Dichte
K12-8          Dichte einschalten
K11-6          Abnahme auf CT 190, Messung, Doku, Release 1.7.0 (Konzept 11)
K12-9          Abnahme auf CT 190, Messung, Doku, Release 1.8.0
```

**Stand 2. November 2026:** Konzept 11 ist mit den Schritten K11-0 bis K11-6 allein gebaut und
als 1.7.0 erschienen (`main` `4c860bc`). Dieses Konzept baut K12-0 bis K12-9 danach auf dem Zweig
`claude/verwaltungsseiten` ohne Zwischenhalt («Durchbauen»): K12-1 bringt **Vertrag 1.8.0**, K12-9
ist die einzige Abnahme auf CT 190 und endet mit **Release 1.8.0**.

| Schritt | Inhalt | DoD | Tage |
|---|---|---|---|
| K12-0 Papiere | Frage 18 (11.5, die Wahl je Seite aus 10.3) ins Entscheidungsprotokoll, ADR 0020 (11.6), Papier auf den dann aktuellen `main` verankern | Doku-Commit, `task ci` grün | 0,5 |
| K12-1 Vertrag und Server | Felder aus 11.4, `task gen`, Abfragen, Serien-Id in `StatsTopTitles`, `withPosters` verallgemeinert, Zeitzone als Option, Bench, Tests | Antwort von `GET /admin/stats?days=30` und `GET /admin/streams` als Beleg; Bench-Werte in `docs/perf/verwaltungsseiten.md` | 2,5 |
| K12-2a Komponententokens | Tokens mit Standardwerten, Bausteine lesen sie; **Ausgangsmessung** der Höhe bis zur ersten Arbeitszeile auf Hintergrundjobs, Integrationen und Konten nach `docs/perf/verwaltungsseiten.md` (Abschnitt 3) | alle Referenzbilder pixelgleich | 1,5 |
| K12-2b Bausteine | alle Bausteine aus 11.2 mit Story in beiden Dichten und Test, `adminSearch`, Prüfung der Klappzeilen mit Bildschirmleser | neue Referenzbilder auf CT 190 erzeugt | 6 |
| K12-3 Seiten, die am meisten stören | Hintergrundjobs, Metadaten, Integrationen (rund 2'900 Zeilen) | Tests aus 11.7 grün, Erhaltungsliste abgehakt | 6 |
| K12-4 Bibliothek | Bibliotheken, Pflege, Titel, Podcasts | wie oben | 5 |
| K12-5 Konten | Konten mit Profilen und Leisten-Eintrag «Profile», Geräte, Streams | wie oben | 4,5 |
| K12-6 Kuratierung | Startseite, Spotlight, Cat's Empfehlungen, Sammlungen, Ankündigungen | wie oben | 5 |
| K12-7 System | Benachrichtigungen, Statistik, Sicherung. Danach entfallen die seiteneigenen CSS-Module für Kopf, Abstände und Seitenaufbau (S-12): `AdminData`, `AdminTablePage`, `AdminItemPage`, `Curation`, `IntegrationsPage`, `Invitations`, `JobsPage`, `LibrariesPage`, `MaintenancePage`, `MetadataPage`, `PicksPage`, `SpotlightPage`. Es bleiben `AdminHomePage` (Konzept 11), `LibraryFoldersDialog` und `PosterTitle` (Bausteine, kein Seitenaufbau) | wie oben | 4 |
| K12-8 Dichte | `density.compact` unter `[data-shell="admin"]` und `AdminDensityScope` einschalten, Messung wiederholen (Ziel: höchstens die Hälfte), `designer` vergleicht jede Seite mit dem Mockup | Messwerte in `docs/perf/verwaltungsseiten.md`. **Stand 3. November 2026:** Dichte an (`2078fa9`), Designer-Abgleich aller 21 Seiten erledigt (`a04c56b` bis `cd1dc75`); Messung 272, 250 und 315 px (57, 58 und 89 % des Ausgangs), Ziel verfehlt, Entscheid bei K12-9 (Frage 18, Nachtrag 5) | 1,5 |
| K12-9 Abnahme | auf CT 190 mit Marco am Desktop (Maus) und am Handy (44 px); p95 von `GET /admin/stats?days=90` in `docs/perf/verwaltungsseiten.md`; `CURRENT_STATUS.md`, Release-Notes | Marcos Abnahme | 1 |

**Aufwand:** Summe **37,5 Tage** (Spanne 33 bis 42), dazu die 15 Tage von Konzept 11. Grundlage:
rund 10'500 Zeilen Seitencode, 6'000 Zeilen Unit-Tests und 10'000 Zeilen e2e, die angepasst
werden.

**Rollen (Phase 4, wie Abschnitt 8):** `planner` legt die Reihenfolge beider Konzepte fest;
`executor` mit Opus für K12-1, K12-2 und K12-8, mit Sonnet für die Seiten; `designer` und
`verifier` je Seite; `code-reviewer` je Schritt.

### 11.9 Offene Punkte

| Nr. | Punkt | Vorschlag |
|---|---|---|
| O-1 | Vertrag 1.7.0 oder 1.8.0, falls Konzept 11 vorher allein erscheint. | In K12-0 festhalten; die Felder bleiben dieselben. **Beantwortet (2. November 2026): 1.8.0 als eigenes Release**, Konzept 11 ist allein als 1.7.0 erschienen (Frage 18). |
| O-2 | Mockup-Fehler, die nicht gebaut werden: Zeitplan-Block bei Jobarten ohne Zeitplan (`Jobs-C`), veralteter Einleitungssatz im Dialog «Sammlung anlegen» (`Sam-C`), Hilfetext «stündlich» bei Podcasts (`Pod-C`; das Abo hat seinen Takt, geprüft wird alle 5 min). | Gebaut wird nach Code und `de.json`. **Wie vorgeschlagen** (2. November 2026). |
| O-3 | Pflege gruppiert nur die geladenen Einträge je Bibliothek; eine Bibliothek kann darum auf Seite 2 nochmals erscheinen. | So lassen; Alternative wäre eine Abfrage je Bibliothek (`listMaintenanceIssues` kennt `libraryId`, `openapi.yaml:3801-3830`), mehr Aufrufe. **Wie vorgeschlagen** (2. November 2026). |
| O-4 | «Wann geschaut wird» bei 365 Tagen nur aus den letzten 90 Tagen. | Hinweis unter der Überschrift; volle 365 Tage bräuchten eine Stundenspalte in `progress_days` und eine Migration (nicht empfohlen). **Wie vorgeschlagen** (2. November 2026). **Verfeinert am 2. November 2026 (Marcos Entscheid):** das Fenster umfasst ganze Wochen, `timeWindowDays` ist 7, 28 oder 84. Der Hinweis lautet bei 90 und 365 Tagen «Aus den letzten 12 Wochen» und bei 30 Tagen «Aus den letzten 4 Wochen». |

### 11.10 Risiken

- **Umfang:** 19 Seiten, 21 Testdateien und 25 e2e-Specs. Je Seite ein Commit mit grüner CI;
  K12-3 nimmt die schwierigsten Seiten zuerst, damit die Bausteine früh an echten Seiten reifen.
- **Entwürfe im Seitenzustand (S-17):** mehr Zustand je Seite. Jede Seite bekommt einen
  kleinen Hook (`useDrafts`), der Entwürfe nach Id hält, «geändert» meldet und das Abfragen
  pausiert; Unit-Tests prüfen den Wechsel von Reiter und Auswahl.
- **Dichte und Referenzbilder:** viele Bilder ändern sich in K12-8 auf einmal. Die
  Komponententokens kommen darum in K12-2 mit pixelgleichen Bildern, die Umstellung allein in
  K12-8.
- **Zusammenspiel mit Konzept 11:** beide ändern `AppLayout`, `AdminLayout` und die Specs der
  Leiste. Die Seiten beginnen erst nach K11-5.
- **Statistik-Abfrage:** 90 Tage `progress_events` je Aufruf. K12-9 misst p95; über 200 ms
  rechnet der Server die Stundenwerte höchstens alle 5 min neu (Zwischenspeicher).
  **Klarstellung (2. November 2026):** die Schwelle von 200 ms gilt für die Rasterabfrage allein
  (längstes Fenster, 84 Tage), nicht für die ganze Antwort von `GET /admin/stats`. Der Entscheid
  fällt mit der Messung auf CT 190 in K12-9 (`docs/perf/verwaltungsseiten.md`).

## 12. Review-Protokoll

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

### Fassung 2 → 3 (OMC-Kritiker, 1. November 2026)

Geprüft wurde Abschnitt 11. Urteil «überarbeiten»: kein Blocker, 7 Major, 20 Minor. Die meisten
Zeilenangaben stimmten; `react-aria-components` 1.22.1 hat `Disclosure`, `@tanstack/react-router`
1.170.45 hat `useBlocker`. Alle Befunde sind eingearbeitet.

| Befund | Einarbeitung |
|---|---|
| M-1 Filterreiter brechen S-13, ob eingehängt oder nicht | S-17: Entwürfe nach Id im Zustand der Seite, Filter- und Bereichsreiter, kein `keepMounted`, `useDrafts` (11.1, 11.2) |
| M-2 Leisten-Eintrag «Profile» ohne Schritt, bricht Konzept 11 | Ziel `?item=me#meine-profile`, `isSectionPath`, Tests von Konzept 11, Schritt K12-5; `/settings/profiles` ohne Weiterleitung (11.3, 11.5) |
| M-3 E-3-Liste unvollständig | `RunsDialog`, `AccountDialog`, «Bibliothek hinzufügen» in 11.5 Punkt 8 |
| M-4 Platzwahl ändert den Einrichtungsassistenten | Option in `ui/ProviderList`, nur der Ordnerdialog der Verwaltung; Tests und Bild genannt |
| M-5 Statistik-Kacheln ohne Bild bei Folgen | Serien-Id in `StatsTopTitles`, Bilder der Serie, Handlertest |
| M-6 react-aria `Disclosure` gehört nicht in `<tr>` | Klappzeile als Knopf mit `aria-expanded` und Detailzeile in `DataTable`; `Disclosure` nur für Gruppen |
| M-7 Speichern über mehrere Geltungen undefiniert | Reihenfolge, Halt bei Fehler, Meldungen je Geltung, Verwerfen (11.3 Metadaten) |
| Minor 1–20 | Link `?tab=failed` in K11-5 und Satz zu `recentFailed`; `adminSearch`; `useBlocker` nicht bei `?tab=`; K12-2a pixelgleich, K12-2b mit Bildern in beiden Dichten; Seitentitel als Token; `AdminDensityScope` statt Attribut am Dokument; Übersicht kompakt als Abweichung; CSS-Module namentlich; nur Profilliste herauslösen; `withPosters` verallgemeinert; Duplikate nicht nach Bibliothek; Zeile `:6310`; Kontrast 12 %; Inhalt von `AnnouncementDialog` herauslösen; `timeZone`, Viertelstunden, Bench; Reiter «Aktiv»; Textliste; Tastaturtests für `MasterDetail`, `RankSelect`, `HeatGrid`; Aufwand 37,5 Tage; K12-2 nach K11-4 |

