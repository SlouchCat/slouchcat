# Konzept: Metadaten pro Ordner – ein Formular je Bibliothek, Klappbares zu, mehr Kontrast

- **Status: Entwurf, Fassung 3 (16. November 2026). Alle Fragen sind beantwortet** (Abschnitt 10):
  Marco hat das Mockup freigegeben, F-1 bis F-3 im Gespräch und F-4 bis F-6 über AskUserQuestion
  beantwortet, alle wie empfohlen. Fassung 2 hatte das Kritiker-Review eingearbeitet (REVISE,
  0 Blocker, 12 Major, 13 Minor; Abschnitt 12) und die Nachprüfung der Einarbeitung (acht kleine
  Korrekturen). Die Freigabe des Papiers steht aus. **Gebaut wird erst auf Marcos ausdrückliches
  Wort.**
- **Auslöser:** Marco am 16. November 2026: «Zur Zeit gibt es Bibliothek Metadaten Einstellungen
  sowie Ordner Metadaten Einstellungen. Ich möchte, dass es nur noch Metadaten Einstellungen pro
  Ordner gibt. Deshalb möchte ich auch gerne die Ansicht in der Verwaltung/Medien angepasst haben.
  Es soll einfach und übersichtlich konzipiert sein.» Am selben Tag nachgereicht: «Zusätzlich
  möchte ich, dass alles was klappbar ist immer zugeklappt ist (als Standard).» «Allgemein gilt,
  dass der Kontrast der Farben zu klein ist. Bitte überall anpassen.» «Ich möchte, wenn ich eine
  Bibliothek ändere, eigentlich die gleiche Ansicht wie ich eine Bibliothek hinzufüge.»
- **Verankert gegen:** `main` **`be75fb37`** («Merge pull request #173», Release 1.39.1),
  Vertragsfassung **1.38.0** (`api/openapi.yaml:4`; 1.39.0 und 1.39.1 haben den Vertrag nicht
  geändert), letzte Migration **`00064_item_certifications.sql`**, letzte ADR **`0035`**, letzte
  Frage im Entscheidungsprotokoll **31**.
- **Nummern:** Dieses Papier ist **Konzept 24** und reserviert **Frage 32** und **ADR 0036**
  («Metadaten je Ordner, Heimatordner eines Titels»; ersetzt in ADR 0018 den Entscheid E-8). Die
  ADR entsteht mit dem Bau von Stufe 2. Migrations- und Versionsnummern vergibt der Bau.
- Pfade ohne Präfix liegen unter `web/src/` oder `server/`. **Nach Namen suchen, nicht nach
  Zeile**; Zeilen beziehen sich auf den Anker.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
  Dieses Papier ändert frühere Entscheide (Abschnitt 9).

> **Dieses Papier ändert keine Zeile Code.** Es beschreibt vier Wünsche, sagt je Wunsch, was der
> Code heute tut, und schlägt eine Lösung vor. Die Messwerte stammen von der Testinstanz CT 190,
> alles andere aus dem Code am Anker.

## 1. Was Marco will

| Nr. | Wunsch (16. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| P-1 | Metadaten-Einstellungen gibt es nur noch pro Ordner, nicht mehr pro Bibliothek. Die Ansicht Verwaltung › Medien passt sich an, einfach und übersichtlich. | 3 |
| P-2 | Eine Bibliothek ändern sieht aus wie eine Bibliothek hinzufügen. | 4 |
| P-3 | Alles Klappbare ist als Standard zugeklappt. | 5 |
| P-4 | Der Kontrast der Farben ist zu klein; überall anpassen. | 6 |

Nicht Teil dieses Papiers: die zwei Befunde der Seite «Pflege» vom selben Tag (gesperrtes
«Zuordnen» bei Folgen, 5'023 Podcast-Duplikate «Datei und Feed»). Sie laufen als eigener Patch.

## 2. Heute

### 2.1 Drei Ebenen für eine Frage

Wie Slouchcat die Titel eines Ordners beschreibt, steht heute an drei Orten:

| Ebene | Wo in der Verwaltung | Was | Speicher |
|---|---|---|---|
| Server («Standard») | Medien › Metadaten, Geltung «Standard» | Sprachen, Bilder (Arten und Sprache je Bildart), Bildqualität, Quellen-Schalter, Altersfreigabe-Land, Anwenden | Schlüssel `metadata.*` in `settings` (`internal/metadata/settings.go`) |
| Bibliothek | Medien › Metadaten, Geltung = eine Videobibliothek | Sprachen abweichend, Bilder abweichend | Tabelle `library_metadata` (`migrations/00030_library_metadata.sql`), `NULL` = wie Server |
| Ordner | Medien › Bibliotheken › aufklappen › «Ordner» › «Ändern» | Inhalt (Filme, Serien), Metadatenquellen in Reihenfolge | `library_roots.content`, `library_roots.providers` (`migrations/00023_library_roots_providers.sql`) |

ADR 0018 hat Sprachen je Ordner ausdrücklich abgelehnt (E-8: «Keine Übersteuerung je Ordner»; die
Bibliothek sei die Sammlung). Marcos Wunsch P-1 dreht das um.

Auf CT 190 gibt es sechs Bibliotheken mit acht Ordnern, davon sechs Video-Ordner in vier
Videobibliotheken (Filme, Serien, Anime, Dokus). **Keine Bibliothek weicht ab**: Die Spalte
«Metadaten» zeigt überall «wie Server».

### 2.2 Wie der Server auflöst

- **Sprachen und Bilder:** `metadata.Effective(libraryID)` nimmt den Standard und legt die
  Abweichung der Bibliothek darüber (`settings.go`). Jeder Verbraucher fragt nach der Bibliothek
  des Titels: Zuordnen (`match.go`), Auffüllen (`fill.go`), Bilder holen (`images.go`,
  `image_candidates.go`), Trailer (`trailers.go`), Podcast-Texte (`podcast.go`). `pruneTexts`
  löscht Textzeilen von Sprachen, die nicht mehr in der Liste stehen.
- **Nur den Standard** lesen schon heute: der Hinweis, welche Sprache eine Quelle nicht kennt
  (`ProviderLanguages`, `settings.go`), und die Bildsprachen der TMDB-Anfrage
  (`include_image_language` in `tmdb.go`; die Liste liefert `Settings.ImageLanguages` aus dem
  Standard, verdrahtet in `metadata.go`). Eine Sprache, die nur eine Bibliothek in ihrer Bildkette
  hat, wird darum womöglich gar nicht angefragt.
- **Quellen:** Ein Titel kennt seinen Ordner nicht. Es gibt kein `items.root_id`, nur
  `media_files.root_id`. Die Quellenliste des Ordners, den der Scan gerade liest, wird beim Anlegen
  auf den Titel kopiert (`items.providers`, `library/write.go`); das gilt auch für eine neue
  Staffel oder Folge einer Serie, die in einem anderen Ordner begonnen hat. Beim Ändern eines
  Ordners schreibt `UpdateItemsProvidersByRoot` (`queries/system/library.sql`) die Liste neu: auf
  jeden Titel mit einer Datei im Ordner, auf dessen Serie und alle ihre Staffeln und Folgen, auf
  einen Film mit seinen Extras. Liegt eine Serie in zwei Ordnern, trägt sie heute gemischte Listen,
  bis einer der Ordner geändert wird; dann gilt dessen Liste für die ganze Serie.
- **Hörbücher** lösen anders auf: `bookSourcesOf` (`book_match.go`) geht über den Ordner der
  ersten vorhandenen Datei.
- **Profile** wählen ihre Beschreibungssprache aus `AvailableMetadataLanguages`: der Standardliste
  plus allen Abweichungen der Bibliotheken (`settings.go`, geprüft in `auth/profiles.go`).
- **Anwenden** (`metadata.apply`, `apply.go`) nimmt `libraryIds`; `affectedTitles` zählt je
  Bibliothek.
- **Kosten:** TMDB braucht eine Anfrage je Sprache für Film, Serie und jede Staffel.

### 2.3 Eine Bibliothek ändern: sieben Wege

| Was | Bauteil | Aufruf | Speichert |
|---|---|---|---|
| Hinzufügen | `libraries/AddLibraryDialog.tsx` mit `shared/LibraryFields.tsx`, `ui/FolderList.tsx`, `ui/ProviderList.tsx` | `createLibrary` | mit dem Formular; Entwurf bleibt auf der Seite (`useLibraryDraft`) |
| Ordner hinzufügen | `libraries/LibraryFoldersDialog.tsx` | `addLibraryRoot` | mit dem Formular |
| Inhalt, Quellen eines Ordners | `ChangeRootDialog` (in derselben Datei) | `updateLibraryRoot` | sofort; anderer Inhalt verlangt den getippten Pfad |
| Pfad eines Ordners | Ordnerwahl, dann Rückfrage | `updateLibraryRoot` | sofort |
| Ordner entfernen | Rückfrage mit getipptem Pfad | `removeLibraryRoot` | sofort |
| Umbenennen, Standardstufe, Vorschau beim Spulen | `RenameLibraryDialog`, `DefaultRatingDialog`, `TrickplayDialog` | `updateLibrary` | je Dialog |
| Sprachen, Bilder abweichend | `MetadataPage.tsx`, Auswahl «Geltung» (`?library=`) | `updateLibraryMetadata` | Speicherleiste der Seite |

Um die Sprachen eines Ordners zu ändern, geht man heute auf eine andere Seite als für seine
Quellen, und dort gilt die Änderung für die ganze Bibliothek. Keine Operation ändert Name,
Standardstufe, Vorschau und Ordner zusammen: `UpdateLibrary` liegt im Paket `admin`, die
Ordner-Operationen im Paket `library`, jede mit eigener Transaktion.

### 2.4 Klappbares

Zwei gemeinsame Bauteile sind ab Werk offen: `ui/DisclosureGroup.tsx` (`defaultExpanded`) und
`ui/BandPanel.tsx` (`useState(true)`). Aufklappbare Tabellenzeilen (`ui/DataTable.tsx`) sind ab
Werk zu. Kein Klappzustand wird gespeichert; zwei Seiten öffnen eine Zeile einmalig aus der
Adresse (`?item=` auf «Jobs» und «Konten»). Heute ab Werk offen sind zehn Stellen (Abschnitt 5).

### 2.5 Kontrast

Gemessen am 16. November 2026 auf CT 190 (dunkles Thema, das einzige):

| Was | Verwaltung | Zuschauer-App (Startseite) |
|---|---|---|
| Gedämpfter Text (`color.text.muted`, oft 12 px) | 6,1 : 1 auf der Fläche, 5,4 : 1 in einer aufgeklappten Zeile | 5,9 bis 6,5 : 1 |
| Zweiter Text (`color.text.secondary`) | 9,3 : 1 | 9,1 bis 10 : 1 |
| Rahmen einer Fläche (`color.border.subtle`) | 1,2 : 1 | – |
| Zweitknopf gegen seine Umgebung (keine Kontur) | 1,1 : 1 | 1,36 : 1 («Details», «Trailer») |
| Aktiver Eintrag der Seitenleiste | – | 1,27 : 1 |

Rechnerisch der schwächste Text ist gedämpfter Text auf `surface.3` mit 4,8 : 1. Der Text hält
damit die Schwelle des bestehenden Tests (4,5 : 1, `packages/design-tokens/test/contrast.test.ts`).
Schwach ist, was kein Text ist:

- **Ein einziger Rahmen-Token**, `color.border.subtle` = `#ffffff14` (8 % Weiss), beschrieben als
  «Hairline separators, used sparingly», 83 Verwendungen in 53 CSS-Dateien. Alle Rahmen der
  Verwaltung hängen daran (`ui/admin-look.css`).
- **Knöpfe ohne Kontur:** `ui/Button.module.css` setzt `border: 0` für alle Varianten. Der
  Zweitknopf ist nur eine Füllung (`surface.3`) und wird beim Überfahren dunkler.
- **Gesperrtes** ist Blässe: `opacity` zwischen 0,3 und 0,6 in dreizehn Bauteilen (`Button`, `Switch`,
  `Field`, `RoundAction`, `PlayerStep` 0,45; `SelectCheck`, `CandidateCard`, `LockToggle`,
  `RankedChoiceList` 0,6; `VolumeControl` 0,5; `AudioPlayer` 0,55; `WatchedToggle` 0,4;
  `LetterBar` 0,3).
- **Kein Wächter** für Nicht-Text-Kontrast (WCAG 2.2, 1.4.11); kein axe in Playwright oder
  Storybook.
- **Ein Fehler nebenbei:** `features/admin/integrations/Integrations.module.css` verwendet an
  zwei Stellen `var(--sc-color-border)`. Diesen Token gibt es nicht; beide Rahmen werden
  vermutlich nicht gezeichnet (im Lauf nicht geprüft). stylelint merkt es nicht, weil die
  Kurzschreibweise `border` nicht in seiner Liste steht.

## 3. P-1: Metadaten nur noch je Ordner

### 3.1 Zwei Ebenen statt drei

Es gibt den **Standard** und den **Ordner**. Die Ebene Bibliothek entfällt. Ein Ordner folgt dem
Standard, bis er Eigenes hat (F-1, Marco: «Ja, Standard behalten»).

| Einstellung | Standard | Ordner |
|---|---|---|
| Inhalt (Filme, Serien) | – | ja (wie heute) |
| Metadatenquellen in Reihenfolge | – | ja (wie heute) |
| Sprachen der Texte | ja | «Wie Standard» oder «Eigene» |
| Bilder: Arten (Logo, Staffelposter, Folgenbild) und Sprache je Bildart | ja | «Wie Standard» oder «Eigene» |
| Bildqualität (Mindestbreite, nach Bewertung) | ja | nein |
| Quellen-Schalter (lokale Bilddateien, Fanart.tv, TMDB-Erwachseneninhalte) | ja | nein |
| Altersfreigabe-Land | ja | nein |

Die unteren drei bleiben beim Standard: Das Freigabeland entscheidet über die Altersgrenzen aller
Profile, Fanart und TMDB sind ein Schlüssel für den ganzen Server. So steht es schon heute; eine
Bibliothek konnte nie davon abweichen.

**Hörbuch- und Podcast-Ordner haben keine eigenen Sprachen und Bilder.** Die Oberfläche bot die
Abweichung schon bisher nur für Videobibliotheken an. Podcasts nutzen für ihre Texte weiter die
Sprachen des Standards; die Quellen für Hörbücher stehen auf der Seite «Hörbuch-Quellen».

### 3.2 Welcher Ordner gilt für einen Titel (ADR 0036)

Heute weiss ein Titel nicht, zu welchem Ordner er gehört (2.2). Für Einstellungen je Ordner muss
das eindeutig sein.

**Jeder Titel hat einen Heimatordner: den Ordner seiner ältesten vorhandenen Datei.** Neue Spalte
`items.root_id` (leer erlaubt).

- **Eine Regel für alle Wege:** Massgeblich ist die vorhandene Datei mit dem ältesten
  `media_files.created_at`, bei Gleichstand der Pfad. Für eine Serie zählen die Dateien aller
  ihrer Folgen. **Staffeln, Folgen und Extras erben den Heimatordner ihrer Serie oder ihres
  Films**; sie nehmen nicht den Ordner, den der Scan gerade liest.
- **Gesetzt** wird er überall, wo ein Titel entsteht oder zurückkommt: beim Anlegen
  (`library/write.go`), bei der Rückkehr eines weich gelöschten Titels (`restore` in derselben
  Datei, `library/relink.go`; ADR 0023), für Hörbücher (`library/audiobook_write.go`) und für
  Podcasts aus einem Ordner (`library/podcast_folder.go`). Feed-Folgen erben den Ordner ihres
  Podcasts; ein Podcast nur aus Feeds hat keinen (`metadata/podcast.go`).
- **Neu bestimmt** wird er, sobald ein Titel in seinem Heimatordner keine vorhandene Datei mehr
  hat, er aber anderswo eine hat: wenn der Ordner entfernt wird, wenn er einen anderen Inhalt
  bekommt (die Titel, die das überleben, weil sie noch eine Datei in einem anderen Ordner haben)
  und wenn ein Scan feststellt, dass die Dateien umgezogen sind. Mit dem Umzug schreibt der Server
  in derselben Transaktion die Quellenliste des neuen Heimatordners auf den Titel und reiht
  Zuordnen und Auffüllen ein.
- **Pfad ändern** lässt den Heimatordner stehen (die Kennung des Ordners bleibt, ADR 0023).
- **Titel ohne Heimatordner** nehmen den Standard.
- `metadata.Effective` fragt nicht mehr nach der Bibliothek, sondern nach dem Titel: eigene
  Sprachen und Bilder des Heimatordners, sonst der Standard. Die Auflösung ist live; ändert sich
  ein Ordner, muss kein Titel umgeschrieben werden.
- **Quellen folgen derselben Regel:** `UpdateItemsProvidersByRoot` schreibt die Liste auf die
  Titel, deren Heimatordner der geänderte Ordner ist, und beim Anlegen einer Staffel oder Folge
  kopiert der Scan die Liste der Serie. Das ändert das Verhalten für Titel, die in zwei Ordnern
  liegen: Heute tragen sie gemischte Listen oder die des zuletzt geänderten Ordners, künftig die
  des Heimatordners.
- **Sichtbar:** Die Titelseite der Verwaltung nennt den Heimatordner bei den Angaben
  (`admin/item/facts.tsx`). Sonst könnte niemand nachsehen, warum ein Titel bestimmte Sprachen
  hat.
- **Fremdschlüssel:** `items.root_id` verweist auf `library_roots(id)` mit `ON DELETE SET NULL`
  und hat einen Index. Weich gelöschte Titel eines entfernten Ordners verlieren so ihren Verweis,
  statt ins Leere zu zeigen.

Verworfen: (a) Sprachen und Bilder wie die Quellenliste auf jeden Titel kopieren: zwei
JSON-Spalten mehr je Titel, bei jeder Änderung tausende Zeilen neu schreiben. (b) Bei jedem
Auftrag über die erste Datei nachschlagen wie bei Hörbüchern: eine Abfrage mehr je Auftrag, und
eine Serie ohne eigene Datei hat keine Antwort. (c) Die Vereinigung aller Ordner eines Titels: am
teuersten, und die Sprache je Bildart lässt sich nicht vereinigen.

Für Marco in einem Satz (F-4): Liegt eine Serie in zwei Ordnern, gelten die Einstellungen des
Ordners mit ihrer ältesten Datei.

### 3.3 Folgen im Server

- **Profile:** `AvailableMetadataLanguages` bildet die Auswahl aus dem Standard und den eigenen
  Listen aller Ordner (bisher: aller Bibliotheken).
- **Katalog:** unverändert. Die Laufzeitkette liest `item_texts` nach Profilsprache,
  Oberflächensprache und `rank` und kennt keine Einstellungen (ADR 0018). Haben zwei Ordner einer
  Bibliothek verschiedene Listen, fällt ein Profil bei Titeln ohne seine Sprache auf die nächste
  zurück; in einer gemischten Bibliothek sortiert die Liste dann nach Schlüsseln in verschiedenen
  Sprachen. Das ist heute zwischen zwei Bibliotheken genauso.
- **Anwenden:** `ApplyMetadataRequest.rootIds` ersetzt `libraryIds`; leer heisst alle Ordner. Ein
  Ordner meint die Titel, deren Heimatordner er ist; `affectedTitles` zählt so. Der Schlüssel gegen
  doppelte Läufe nennt die Ordner. Ein Lauf, der beim Update noch wartet und `libraryIds` trägt,
  wird auf die Ordner dieser Bibliotheken umgerechnet; er darf nicht als «alle» gelesen werden.
- **Hinweis «Quelle kennt die Sprache nicht»:** `ProviderLanguages` rechnet mit der Liste, die
  gerade bearbeitet wird (Standard oder Ordner), nicht mehr nur mit dem Standard.
- **TMDB-Bildsprachen:** `include_image_language` nimmt die Vereinigung der Bildketten des
  Standards und aller Ordner. Das schliesst die Lücke aus 2.2.
- **Hörbücher:** `bookSourcesOf` liest künftig den Heimatordner statt den Ordner der ersten
  vorhandenen Datei. So gilt eine Regel für alle Arten.
- **Kosten:** Eine weitere Sprache an einem Ordner mit N Serien und S Staffeln je Serie kostet beim
  Anwenden rund N · (1 + S) TMDB-Anfragen. Die Ordnerkarte sagt das wie heute die Seite «Metadaten»
  («Jede weitere Sprache kostet eine Anfrage je Titel und Staffel»), samt dem Hinweis auf das
  YouTube-Kontingent der Trailersuche.

### 3.4 Seite «Metadaten»

Die Auswahl «Geltung» entfällt. Die Seite zeigt nur noch den Standard. Reiter und Seitentitel
bleiben «Metadaten» (Frage 31: die Seitentitel bleiben); die Einleitung sagt: «Gilt für jeden
Ordner ohne eigene Einstellung. Eigene Sprachen und Bilder eines Ordners stellst du in seiner
Bibliothek unter ‹Bearbeiten› ein.»

Statt «Liste und Detail» (Konzept 12, Met-B) stehen die Bereiche als **Klappabschnitte**
untereinander, alle zu, jeder mit seiner Zusammenfassung in der Kopfzeile (Mockup Bild 4):

| Abschnitt | Zusammenfassung in der Kopfzeile (Stand CT 190) |
|---|---|
| Sprachen | Deutsch › Englisch |
| Bilder | Poster, Hintergrund, Logo, Staffelposter, Folgenbild |
| Bildqualität | Poster ab 1500 px, Hintergrund ab 1920 px |
| Quellen | Lokale Dateien, Fanart.tv, TMDB mit Erwachseneninhalten |
| Altersfreigabe | Deutschland (FSK) |
| Anwenden | Was (Texte, Bilder, beides) und Wo (alle Ordner oder ein Ordner) |
| Ordner | «6 Video-Ordner folgen dem Standard, 0 haben Eigenes»; aufgeklappt eine Zeile je Ordner mit «Bearbeiten» |

Der Inhalt jedes Abschnitts ist der heutige Bereich, unverändert: die Felder (`LanguageRanks`,
`CheckboxGroup`, `RadioGroup`, `Switch`), die Hinweise (welche Quelle eine Sprache nicht kennt,
YouTube-Kontingent, Zahl der Titel mit lokalen Bildern), der Link zu den Schlüsseln bei Fanart, in
«Anwenden» der Stand des letzten Laufs und die wartenden Aufträge. Die Speicherleiste, die
Rückfrage beim Wechsel des Freigabelands, die Rückfrage «Verwerfen», der Hinweis «Hörbücher:
Quellen» und die Links zu Bibliotheken und Hintergrundjobs am Seitenende bleiben.

Ein Abschnitt mit ungespeicherten Änderungen zeigt in seiner Kopfzeile **«geändert»**, auch
zugeklappt (heute der Zusatz in der Liste links). `?item=<bereich>` öffnet weiter den genannten
Abschnitt (Abschnitt 5, Ausnahme 1). `?library=` entfällt und wird aus der Adresse entfernt, wie
heute eine unbekannte Kennung.

### 3.5 Migration

Eine Migration nach Vorlage (`migrations/TEMPLATE.sql.txt`, Regel 5):

1. `library_roots` bekommt `text_langs` und `images` (JSON mit `json_valid`-Prüfung; **`NULL` =
   wie Standard**). `items` bekommt `root_id INTEGER REFERENCES library_roots(id) ON DELETE SET
   NULL` und einen Index. Beides geht mit `ALTER TABLE … ADD COLUMN`; auf `root_id` hört kein
   Trigger.
2. Jede Abweichung in `library_metadata` wird auf **jeden Video-Ordner** dieser Bibliothek kopiert.
   Was heute gilt, gilt danach unverändert.
3. `items.root_id` wird nach der Regel aus 3.2 gefüllt: zuerst Filme, Hörbücher und Podcasts aus
   ihrer ältesten vorhandenen Datei und Serien aus der ältesten Datei ihrer Folgen; danach erben
   Staffeln, Folgen und Extras vom Elternteil. Das sind auf CT 190 rund 70'000 Zeilen.
4. `library_metadata` wird gelöscht.

Auf CT 190 weicht keine Bibliothek ab; Schritt 2 kopiert dort nichts. Die Sicherung vor dem Lauf
(`VACUUM INTO`) gibt es schon. Der Test braucht einen **neuen eingefrorenen Stand** in
`testdata/db/` auf dem Stand von `00064` mit einer abweichenden Bibliothek, zwei Ordnern und einer
Serie, die in beiden liegt.

### 3.6 Vertrag

| Stelle | Änderung |
|---|---|
| `LibraryRoot` | neu `textLangs` und `images` (`null` = wie Standard), dazu die Zahl der Titel, deren Heimatordner er ist |
| `LibraryRootUpdate`, `LibraryRequest.roots[]`, `LibraryRootsRequest` | nehmen `textLangs` und `images` |
| `ApplyMetadataRequest` | `rootIds` ersetzt `libraryIds` |
| `getLibraryMetadata`, `updateLibraryMetadata`, `LibraryMetadata`, `LibraryMetadataUpdate` | entfallen |
| Titel der Verwaltung (`AdminItem`) | neu der Heimatordner |

Beim Ändern eines Ordners gibt es je Feld drei Zustände: Feld fehlt = bleibt, `null` = wie
Standard, Wert = eigene Einstellung. Eine eigene Bildeinstellung mit leerer Liste der Bildarten
ist gültig (kein Logo, kein Staffelposter, kein Folgenbild).

Das bricht den Vertrag (zwei Operationen entfallen, ein Feld wird ersetzt). Der einzige Client ist
die eigene Web-App im selben Release, wie bei der Benutzerverwaltung (Vertrag 1.2.0). Jede neue
oder geänderte Operation behält `x-visibility: admin` und ihren Eintrag im Vertragstest (Regel 7).

## 4. P-2: Ein Formular je Bibliothek

### 4.1 Dieselbe Ansicht

«Bibliothek bearbeiten» ist der Dialog «Bibliothek hinzufügen» (`FormDialog`, breit), mit den
Werten der Bibliothek gefüllt (Mockup Bild 2, oben umschaltbar zwischen beiden). Ein Bauteil
`LibraryForm` trägt beide und die Ersteinrichtung (`setup/LibrariesStep.tsx` nutzt den Dialog
schon heute).

| Feld | Hinzufügen | Bearbeiten |
|---|---|---|
| Name | leer | Name der Bibliothek |
| Inhalt (Video, Hörbücher, Podcasts) | wählbar | gesetzt und gesperrt, mit dem Satz «Die Art steht fest, sobald die Bibliothek angelegt ist.» |
| Ordner | eine offene Karte (Podcasts: keine) | eine Karte je Ordner, alle zu |
| «Weiteren Ordner hinzufügen» | ja | ja; die neue Karte ist offen |
| Standardstufe (nur Hörbücher, Podcasts) | ja | ja |
| Vorschau beim Spulen (nur Video) | Abschnitt | derselbe Abschnitt, zu, mit «Filme: … · Serien: …» in der Kopfzeile |
| Knöpfe | «Abbrechen», «Bibliothek mit N Ordnern hinzufügen» | «Abbrechen», «Speichern» |

**Mindestzahl der Ordner:** Hinzufügen verlangt wie heute einen Ordner (Podcasts keinen).
Bearbeiten darf wie heute auch den letzten Ordner entfernen; die Bestätigung nennt dann die Folge
«letzter Ordner dieser Bibliothek».

**Vorschau beim Spulen** behält alles aus dem heutigen Dialog: die Speicherschätzung samt der
Zeile «Schätzung nicht verfügbar», die Warnung bei grossem Bedarf und «Generieren». «Generieren»
startet wie heute sofort den Lauf für alle Bibliotheken mit «Im Voraus»; es wirkt auf die
gespeicherte Einstellung und ist gesperrt, solange der Abschnitt ungespeicherte Änderungen hat.
Der Grund steht daneben (Z-5).

### 4.2 Die Ordnerkarte

Zugeklappt eine Zeile (Mockup Bild 2): Pfad, Inhalt, Quellenkette, «Sprachen: Standard» oder die
eigene Kette, «Bilder: Standard» oder «Bilder: eigene». Hat die Karte ungespeicherte Änderungen,
steht «geändert» dabei.

Aufgeklappt (Mockup Bild 3), von oben nach unten:

| Zeile | Inhalt | Bauteil (Bestand) |
|---|---|---|
| Ordner | Pfad mit «Durchsuchen …»; darunter Titel, Dateien, Dateisystem, die Warnung «Netzlaufwerk», «nur lesbar» oder «schreibbar» (ADR 0032), die Art der Überwachung | `PathBrowserButton`, die Angaben aus `LibraryFoldersDialog` |
| Inhalt | Filme oder Serien; ein anderer Inhalt liest den Ordner neu ein | `RadioGroup` |
| Metadatenquellen | ankreuzen, Platz wählen; der erste Eintrag erkennt den Titel | `ProviderList` mit `ordering="rank"` |
| Sprachen | «Wie Standard» oder «Eigene»; bei «Eigene» die Plätze 1., 2., … | `RadioGroup`, `LanguageRanks` |
| Bilder | «Wie Standard» oder «Eigene»; bei «Eigene» die drei schaltbaren Bildarten und die Sprache je Bildart | `RadioGroup`, `CheckboxGroup`, `LanguageRanks` mit «ohne Text» |
| – | «Ordner entfernen …» | Knopf, gefährlich und leise |

Bei «Wie Standard» steht der Standardwert als Text daneben. Wer «Eigene» wählt, beginnt mit dem
Standard; wer zurückschaltet, verliert die eigene Liste erst beim Speichern (wie heute der
Schalter «abweichend», S-13). Die drei Hinweise beim Ändern bleiben in der Karte: nur Auffüller
geändert, der Erkenner wechselt, der Inhalt wechselt. Hörbuch- und Podcast-Ordner zeigen Pfad und
Quellen, keine Sprachen und Bilder.

**Pfad und Inhalt desselben Ordners** lassen sich wie heute nicht in einem Schritt ändern; die
Karte sagt das, bevor gespeichert wird.

Das Mockup zeichnet die Quellenliste vereinfacht (Platz als Text). Gebaut wird die bestehende
`ProviderList`.

### 4.3 Speichern in einem Zug

Marco: «Ja, in einem Zug» (F-3). Nichts wirkt, bevor «Speichern» gedrückt ist.

- **Neue Operation `saveLibrary`** (`PUT /admin/libraries/{id}`, `x-visibility: admin`): Name,
  Standardstufe, Vorschau beim Spulen, die Ordner (bestehende mit Kennung, neue ohne) und eine
  **ausdrückliche Liste der zu entfernenden Ordner** mit getipptem Pfad. Ein bestehender Ordner,
  der in der Anfrage weder steht noch entfernt wird, ist ein Fehler (409) und wird **nicht**
  entfernt.
- **Schutz vor veraltetem Stand:** Die Anfrage nennt den Stand der Bibliothek, den das Formular
  geladen hat (eine Marke, die jede Änderung an Bibliothek oder Ordnern weiterzählt). Passt er
  nicht mehr, weil ein zweiter Administrator oder ein anderes Fenster inzwischen gespeichert hat,
  antwortet der Server mit 409 `stale` und schreibt nichts; der Dialog sagt es und lädt die
  Bibliothek neu.
- **Eine Transaktion, eine Reihenfolge:** Die Ordner-Funktionen des Pakets `library` (`AddRoots`,
  `UpdateRoot`, `RemoveRoot`) und `UpdateLibrary` des Pakets `admin` bleiben als Bausteine und
  nehmen künftig eine gemeinsame Transaktion entgegen. Reihenfolge: entfernen, anderer Inhalt,
  Pfadwechsel, neue Ordner; so kann ein neuer Ordner den Pfad eines eben entfernten bekommen. Was
  nicht in der Datenbank liegt (Überwachung, Scan-Zustand, Benachrichtigung), läuft nach dem
  Abschluss. Schlägt etwas fehl, ist nichts geschrieben; Feldfehler heissen wie beim Anlegen
  `roots[i].…`. Ändert eine Anfrage Pfad und Inhalt desselben Ordners, lehnt der Server sie wie
  heute mit einem Feldfehler ab, bevor etwas geschrieben wird.
- **Höchstens ein Scan:** eingereiht nur, wenn ein Ordner neu ist oder einen anderen Pfad oder
  Inhalt hat. Geänderte Quellen reihen wie heute Zuordnen und Auffüllen ein, keinen Scan; ein
  neuer Name scannt nichts.
- **Schwere Änderungen bestätigen:** «Ordner entfernen» und «anderer Inhalt» verlangen wie heute
  den getippten Pfad (`confirmPath`), jetzt gesammelt beim Speichern. Der Dialog «Änderungen
  bestätigen» nennt je betroffenem Ordner die Folgen (der heutige Text `RemovalConsequences`) und
  hat je Ordner ein Feld. Der Server prüft die Pfade selbst.
- **Pfad ändern** verlangt keinen getippten Pfad, wie heute; der Bestätigungsdialog nennt alten
  und neuen Pfad.
- **Entfernen in der Karte** markiert nur: Die Karte klappt zu, zeigt «Wird beim Speichern
  entfernt» und «Rückgängig».
- **Laufender Scan:** Entfernen, anderer Inhalt und Pfadwechsel nehmen wie heute die Scan-Sperre
  der Bibliothek. Läuft ein Scan, lehnt der Server das ganze Speichern ab (409 `scan_running`);
  der Dialog bleibt mit allen Eingaben offen und bietet «Scan abbrechen» an. Name, Stufe, Vorschau,
  Quellen, Sprachen, Bilder und ein neuer Ordner gehen wie heute auch während eines Scans.
- **Ungespeichertes:** «Abbrechen», ✕ und Escape mit Änderungen fragen «Änderungen verwerfen?».
  Einen aufbewahrten Entwurf gibt es nur beim Hinzufügen (Konzept 20, 13.2), nicht beim
  Bearbeiten.
- **Eine Meldung statt fünf:** Nach dem Speichern nennt ein Band auf der Seite, was sich geändert
  hat (heute je Aktion eine eigene Meldung im Ordner-Dialog).
- **Danach anwenden:** Haben sich Sprachen oder Bilder eines Ordners geändert, nennt die Antwort
  je Ordner die Zahl der Titel, deren Heimatordner er ist. Das Band bietet «Jetzt anwenden» an.
  Slouchcat wendet nicht von selbst an, weil jede Sprache Anfragen kostet (3.3).

`createLibrary` bleibt und nimmt die neuen Felder je Ordner. `updateLibrary` bleibt für die
Podcast-Einstellung «Neue Folgen» auf der Seite «Podcasts». `addLibraryRoot`, `updateLibraryRoot`
und `removeLibraryRoot` entfallen mit Stufe 3 aus dem Vertrag; ihre Funktionen im Server bleiben
als Bausteine von `saveLibrary`. Drei e2e-Dateien rufen die alten Operationen als Vorbereitung auf
und werden umgestellt (`admin-podcasts.spec.ts`, `admin-podcast-page.spec.ts`,
`admin-library-folders.spec.ts`).

### 4.4 Seite «Bibliotheken»

Mockup Bild 1.

- **Tabelle:** Bibliothek, Art, Zustand, Titel · Dateien, Ordner. Die Spalte «Metadaten» entfällt.
- **Aufgeklappte Zeile:** «Erneut scannen» (während eines Scans «Scan abbrechen»),
  **«Bearbeiten»**, «Weitere Aktionen» («Verlust akzeptieren», «Bibliothek löschen …»), bei
  Hörbüchern der Link «Quellen für Hörbücher». Darunter wie heute Fortschritt, Fehlerband,
  Keyframe-Index mit dem Link zum Nachtrag, die Zeile «Vorschau beim Spulen» und die Ordner, je
  Ordner mit Inhalt, Quellenkette, «Sprachen: …», «Bilder: …» und den technischen Angaben. Die
  Ordnerliste bleibt hier nur zum Lesen. Der Text einer Bibliothek ohne Ordner verweist auf
  «Bearbeiten» statt auf «Ordner»; «Titel aus Feeds» bei Podcasts bleibt.

### 4.5 Nichts geht verloren

| Heute | Künftig |
|---|---|
| Knopf «Ordner» → Dialog mit Ordnerliste | «Bearbeiten»; die Liste zum Lesen steht in der aufgeklappten Zeile |
| «Neue Ordner» im Ordner-Dialog | «Weiteren Ordner hinzufügen» im Formular |
| «Ändern» (Inhalt, Quellen) je Ordner | Ordnerkarte: Inhalt, Metadatenquellen |
| «Pfad ändern» je Ordner | Ordnerkarte: Pfad mit «Durchsuchen …» |
| «Entfernen» je Ordner mit getipptem Pfad, samt Folge «letzter Ordner» | Ordnerkarte: «Ordner entfernen …», Bestätigung beim Speichern mit denselben Folgen |
| Angaben je Ordner: Dateisystem, «Netzlaufwerk», «nur lesbar» oder «schreibbar», Überwachung, Titel und Dateien | in der aufgeklappten Zeile und in der Ordnerkarte |
| Hinweise der drei Gewichte beim Ändern | dieselben Hinweise in der Ordnerkarte |
| Meldungen «gespeichert» je Aktion | ein Band nach dem Speichern |
| «Umbenennen …» | Feld «Name» |
| «Standardstufe ändern» | Feld «Standardstufe» |
| «Vorschau beim Spulen» mit Schätzung, Warnung und «Generieren» | Abschnitt im Formular |
| Link «Metadaten der Bibliothek», Spalte «Metadaten» | Ordnerkarte: Sprachen, Bilder; Zusammenfassung je Ordner in der aufgeklappten Zeile |
| Seite «Metadaten», Geltung = Bibliothek, mit den Hinweisen zu Quellen und YouTube | Ordnerkarte: Sprachen, Bilder, dieselben Hinweise |
| «geändert» je Bereich und je Geltung | «geändert» in der Kopfzeile des Abschnitts und der Ordnerkarte |
| «Anwenden» auf eine Bibliothek | «Anwenden» auf einen Ordner, dazu das Band nach dem Speichern |
| «Hörbuch-Quellen»: Stift an einem Ordner öffnet den Ordner-Dialog | öffnet «Bearbeiten» dieser Bibliothek mit offener Karte dieses Ordners |
| Link «Quellen für Hörbücher», Hinweis «Hörbücher: Quellen» auf «Metadaten», Links zu Schlüsseln, Bibliotheken, Hintergrundjobs | unverändert |
| «Erneut scannen», «Scan abbrechen», «Verlust akzeptieren», «Bibliothek löschen …», Entwurf beim Hinzufügen | unverändert |

Es entfallen als Bauteile: `LibraryFoldersDialog` samt `ChangeRootDialog`, `RenameLibraryDialog`,
`DefaultRatingDialog`, `TrickplayDialog` (die Felder `TrickplayFields` bleiben), `MetadataCell`.

## 5. P-3: Alles Klappbare ist zu

**Regel:** Jeder Aufklapper startet geschlossen, in der Verwaltung und in der Zuschauer-App. Seine
Kopfzeile sagt, was drin ist (Zusammenfassung oder Anzahl) und ob darin etwas ungespeichert ist
(«geändert»). Innerhalb einer Seite bleibt er, wie man ihn gelassen hat; nach dem Neuladen ist er
wieder zu. Gespeichert wird nichts, wie heute.

**Ausnahmen** (sonst bliebe etwas unsichtbar):

1. **Sprunglink:** Eine Adresse, die ein Ziel nennt (`?item=` auf «Jobs», «Konten», «Metadaten»;
   der Stift auf «Hörbuch-Quellen»), öffnet dieses eine Ziel.
2. **Eben angelegt:** Was man gerade hinzugefügt hat, ist offen (neue Ordnerkarte).
3. **Fehler oder aktuelle Wahl darin:** Meldet der Server einen Fehler zu einem Feld in einem
   geschlossenen Aufklapper, öffnet er sich. «Weitere Sprachen» im Ton- und Untertitelmenü des
   Players bleibt offen, wenn die gewählte Spur darin liegt oder es keine Spur in der Sprache des
   Profils gibt (`player/panels/TrackPanels.tsx`).
4. **Das Aktuelle** (F-5, Marco: «Beide offen»): Die Gruppe «Laufend» auf «Jobs» und die Zeile der
   nächsten Folge auf der Serienseite bleiben offen. Man öffnet die Seite ihretwegen.

Keine Aufklapper im Sinn der Regel: Menüs, Dialoge, Reiter, «Liste und Detail» und die Panels des
Players selbst (was in einem Panel aufklappt, schon).

**Was sich ändert** (acht der zehn heute offenen Stellen):

| Nr. | Stelle | Datei | Heute |
|---|---|---|---|
| 1 | Vorgabe von `DisclosureGroup` | `ui/DisclosureGroup.tsx` | `defaultExpanded` |
| 2 | Vorgabe von `BandPanel` (klappbar) | `ui/BandPanel.tsx` | `useState(true)` |
| 3 | Gruppen je Gerätetyp | `admin/DevicesPage.tsx` | alle offen |
| 4 | Gruppen je Sicherungsart | `admin/BackupPage.tsx` | alle offen |
| 5 | Abschnitte je Bibliothek in der Pflege | `admin/maintenance/IssueTab.tsx` | alle offen |
| 6 | Erste Duplikat-Gruppe | `admin/maintenance/DuplicatesTab.tsx` | erste offen |
| 7 | Bänder «Quelle», «Einträge», «Verwaiste» | `admin/SpotlightPage.tsx` | offen |
| 8 | Blöcke je Kanal | `notifications/ChannelSettingsPage.tsx` | alle offen |

Offen bleiben die Gruppe «Laufend» (`admin/JobsPage.tsx`) und die Zeile der nächsten Folge
(`catalog/SeasonShelf.tsx`), Ausnahme 4. Die Messung «erste Arbeitszeile» an der Gruppe «Laufend»
(`e2e-perf/admin-height.spec.ts`) bleibt damit, wie sie ist. Beide Bauteile setzen ihren Zustand
künftig ausdrücklich, weil die Vorgabe von `DisclosureGroup` auf «zu» wechselt.

Der Bau prüft je Stelle, ob die Kopfzeile schon eine Anzahl oder Zusammenfassung trägt, und
ergänzt sie, wo sie fehlt (sicher bei den drei Spotlight-Bändern und den Kanalblöcken). Schon
heute zu und unverändert: alle aufklappbaren Tabellenzeilen (Bibliotheken, Konten, Sammlungen,
Startseiten-Reihen, Podcasts, Jobs, Dateien, Webhooks), die Aufklapper der Titelseite der
Verwaltung, «Neue Folgen», die Kontinent-Kacheln, das Fehlerprotokoll, der Filter der
Bibliotheksseite, «Mehr» an Hilfetexten und Biografien.

Die neuen Flächen dieses Papiers folgen der Regel: Ordnerkarten, «Vorschau beim Spulen», die
Abschnitte der Seite «Metadaten».

## 6. P-4: Mehr Kontrast, überall

Marco: «Ja, überall» (F-2): Verwaltung und Zuschauer-App.

### 6.1 Zielwerte

Verbindlich sind die Schwellen, nicht die Farbwerte.

| Nr. | Ziel | Heute | Geprüft durch |
|---|---|---|---|
| Z-1 | Die Textfarben `color.text.*` mindestens **7 : 1** auf jeder Fläche, auf der sie stehen dürfen. Akzent- und Zustandsfarben als Text bleiben bei mindestens 4,5 : 1 | Test verlangt 4,5 : 1; rechnerisch ab 4,8 : 1 | Token-Test |
| Z-2 | Rahmen einer Fläche (Panel, Tabelle, Band, Hinweis, Speicherleiste) mindestens **3 : 1** gegen die Fläche | 1,2 : 1 | Token-Test |
| Z-3 | Jedes Bedienelement (Knopf, Feld, Auswahl, Ankreuzfeld, Schalter, Reiter) hebt sich mit Kontur oder Füllung mindestens **3 : 1** von seiner Umgebung ab (WCAG 2.2, 1.4.11) | 1,1 bis 1,4 : 1 beim Zweitknopf | Token-Test für die Kontur; je Bauteil von Hand |
| Z-4 | Trennlinien in Listen und Tabellen sind klar sichtbar (Richtwert 2 : 1) | 1,2 : 1 | Token-Test |
| Z-5 | Gesperrtes bleibt lesbar (Beschriftung mindestens 4,5 : 1) und ist an der Form erkennbar (gestrichelte Kontur, keine Füllung), nicht an Blässe. Ist es die Hauptaktion einer Leiste, steht der Grund daneben | `opacity` 0,3 bis 0,6 | von Hand |
| Z-6 | Text über Bildern mindestens 4,5 : 1 auch über einem hellen Bild | nur die Bibliothekskachel ist geprüft | Test je Stelle |

«Von Hand» heisst: Eine Storybook-Seite «Kontrast» zeigt jedes Bedienelement in jedem Zustand auf
jeder Fläche. Sie wird bei der Abnahme angesehen, und ihr Bild gehört zu den Bildvergleichen.
Ein automatischer Prüfer (axe) wäre eine neue Abhängigkeit und ist nicht Teil dieses Papiers.

### 6.2 Richtwerte aus dem Mockup

Mockup Bild 5 zeigt denselben Ausschnitt bisher und neu. Die Bilder 1 bis 4 nutzen die neuen
Farben.

| Rolle | Token heute | Heute | Richtwert | Gemessen neu |
|---|---|---|---|---|
| Gedämpfter Text | `color.text.muted` | `#9095a3` | `#bcc1cd` | 9,7 : 1 auf der Fläche, 8,6 : 1 in der aufgeklappten Zeile |
| Zweiter Text | `color.text.secondary` | `#b6b9c4` | `#d9dce4` | 12,7 : 1 |
| Fläche | `color.bg.elevated` | `#13151b` | `#171a22` | – |
| Aufgeklappte Zeile, Tabellenkopf | `color.surface.2` | `#1d2029` | `#20242f` | Spaltenkopf 11,3 : 1 |
| Trennlinie | `color.border.subtle` | `#ffffff14` | `#ffffff38` | rund 2 : 1 |
| Rahmen einer Fläche | neu `color.border.frame` | – | `#ffffff57` | 3,1 : 1 |
| Kontur eines Bedienelements | neu `color.border.control` | – | `#ffffff73` | 4,3 : 1 gegen die aufgeklappte Zeile; Feld auf dunklem Grund mit `#ffffff8c` 6,2 : 1 |

Aus einem Rahmen-Token werden drei, weil eine Trennlinie in einer Liste leiser sein soll als der
Rand eines Knopfs. Die Akzentfarben der Profile und die Zustandsfarben bleiben. Die endgültigen
Werte legt der Bau gegen den Kontrasttest fest.

### 6.3 Bauteile

| Bauteil | Änderung |
|---|---|
| `Button` Zweitknopf | Kontur `border.control`; beim Überfahren heller statt dunkler |
| Alles Gesperrte (die dreizehn Bauteile aus 2.5) | Z-5 statt `opacity` |
| `Chip` neutral und Info | Kontur; die Zustands-Chips der Verwaltung bekommen eine Kontur in ihrer Farbe |
| `TextField`, `SelectField`, `SearchField` | Kontur `border.control` |
| `CheckboxGroup`, `SelectCheck`, `Switch` | Kästchen und Bahn mit Kontur |
| `Tabs`, `AreaTabs`, `MasterDetail` | gewählter Eintrag hebt sich nach Z-3 ab |
| `DataTable` | Kopfzeile mit zweitem Text statt gedämpftem, eigener Grund |
| `Panel`, `BandPanel`, `Notice`, `SaveBar`, Kennzahl-Kacheln | Rahmen `border.frame` (in der Verwaltung über `ui/admin-look.css`) |

### 6.4 Zuschauer-App

Die Tokens gelten für beide Oberflächen; Text, Trennlinien, Knöpfe, Felder und die Seitenleiste
der App ändern sich mit. **Bildflächen bekommen keinen Rahmen** (Poster, Kacheln, Spotlight,
Kopf der Detailseite): Die Spec verlangt «grosse Bildflächen, … wenig Rahmen und Linien», und ein
Bild hebt sich selbst ab (F-6). Knöpfe über Bildern stehen schon auf einem dunklen Grund
(`scrim.strong`).

Text über Bildern (Spotlight, Kopf der Detailseite, Player) liegt auf Verläufen aus `scrim.*`.
Ob zweiter und gedämpfter Text dort über einem hellen Bild Z-6 hält, ist ungeprüft; der Bau misst
es mit einem weissen Testbild, wie es der Test der Bibliothekskachel schon tut, und verstärkt den
Verlauf, wo es nicht reicht.

### 6.5 Wächter

- `packages/design-tokens/test/contrast.test.ts` prüft künftig Z-1 für die Textfarben (7 : 1; die
  Akzent- und Zustandsfarben bleiben in ihrer bestehenden Prüfung bei 4,5 : 1) und neu Z-2, Z-3
  und Z-4 für die Rahmen-Tokens gegen jede Fläche.
- Ein Test für Z-6 je Stelle mit Text über Bild.
- Ein Test, der jeden in CSS verwendeten Token `--sc-color-*` gegen die Token-Quelle prüft. Er
  hätte `var(--sc-color-border)` gefunden; dieser Fehler wird mit Stufe 1 behoben.
- Die bestehenden Tests der Seitenleiste und des Bands (`AdminSideNav.test.tsx`,
  `BandPanel.test.tsx`) bleiben.

### 6.6 Bildvergleiche

Eine Token-Änderung trifft fast alle **532 Storybook-Bilder** (`web/e2e-storybook/__screenshots__/`).
Sie werden einmal neu aufgenommen (`task web:storybook:update`; auf der Entwicklungsmaschine
ohne Docker über CT 190, `docs/dev/CURRENT_STATUS.md`). Die Toleranz (`maxDiffPixelRatio: 0.001`)
hat einen dunklen Knopf auf dunklem Grund schon einmal übersehen; nach der Neuaufnahme wird eine
Stichprobe von Hand angesehen. Die e2e-Tests vergleichen keine Bilder.

## 7. Was gleich bleibt

- Inhalt und Quellen je Ordner mit ihren Regeln (ADR 0015), Pfadwechsel ohne Datenverlust
  (ADR 0023), was «Ordner entfernen» tut, die Scan-Sperre.
- Der Standard mit allen sechs Bereichen; «Hörbuch-Quellen»; «Neue Folgen» bei Podcasts.
- Die Laufzeitkette der Texte im Katalog; kein Client entscheidet eine Regel (Regel 12).
- Die sechs Bereiche der Verwaltung, die Bereichsreiter, alle Adressen `/admin/...` (Frage 31).
  Es entfällt nur der Suchparameter `?library=` der Seite «Metadaten» (Abschnitt 9).
- Die kompakte Dichte der Verwaltung (ADR 0020). Im Mockup sind die Knöpfe 44 px hoch, weil das
  Werkzeug es so vorgibt; gebaut wird die bestehende Dichte.
- Nur das dunkle Thema. Ein helles Thema oder `prefers-contrast` sind nicht Teil dieses Papiers.

## 8. Stufen

| Stufe | Inhalt | Server, Vertrag, Migration |
|---|---|---|
| **1** | P-4 Kontrast (Tokens, Bauteile, Wächter, Storybook-Seite «Kontrast», alle Storybook-Bilder neu) und P-3 Klappbares zu | nur Tokens und Web; kein Vertrag, keine Migration |
| **2** | P-1 Metadaten je Ordner: Heimatordner, Auflösung je Titel, Sprachauswahl der Profile, Anwenden je Ordner; Seite «Metadaten» ohne «Geltung»; Sprachen und Bilder in der Ordnerkarte der bestehenden Dialoge («Hinzufügen», «Ordner ändern»); auf «Bibliotheken» entfallen die Spalte «Metadaten», `MetadataCell` und der Link «Metadaten der Bibliothek», die aufgeklappte Zeile zeigt Sprachen und Bilder je Ordner; Heimatordner auf der Titelseite der Verwaltung | Migration (3.5), Vertrag brechend (3.6), ADR 0036 |
| **3** | P-2 Ein Formular: `saveLibrary`, «Bearbeiten» wie «Hinzufügen», «Bearbeiten» in der aufgeklappten Zeile, alte Dialoge weg | Vertrag: eine Operation neu, drei entfallen; keine Migration |

Stufe 1 hängt an nichts und ist sofort sichtbar. Jede Stufe hinterlässt ein stimmiges Produkt: Nach
Stufe 2 gibt es keine Bibliotheks-Ebene mehr, weder im Server noch in der Oberfläche. Stufe 2 baut
das Stück «Sprachen und Bilder eines Ordners» als eigenes Bauteil, das Stufe 3 unverändert in das
Formular setzt. Bis Stufe 3 heisst der Einstieg zu den Ordnern einer Bibliothek weiter «Ordner»;
die Texte der Seite «Metadaten» (3.4) nennen ihn in Stufe 2 so und ab Stufe 3 «Bearbeiten». Marco
kann die Stufen 2 und 3 auch in einem Release bestellen.

## 9. Änderungen an früheren Entscheiden

| Früher | Neu |
|---|---|
| ADR 0018, E-8 (Konzept 10): «Keine Übersteuerung je Ordner», Sprachen und Bilder je Bibliothek (`library_metadata`) | Sprachen und Bilder je Ordner; die Ebene Bibliothek entfällt (ADR 0036) |
| ADR 0033, «Was von ADR 0018 bleibt»: die Abweichung je Bibliothek | die Abweichung je Ordner; die Sprachregeln von ADR 0033 gelten für die Liste eines Ordners |
| ADR 0015: die Quellenliste eines Ordners gilt für jeden Titel mit einer Datei darin, samt Serie | gilt für die Titel, deren Heimatordner er ist (3.2) |
| Konzept 12, Met-B und MET-1: Seite «Metadaten» als «Liste und Detail» mit Auswahl «Geltung» | Klappabschnitte, nur der Standard (3.4) |
| Konzept 12, BIB-3 und BIB-5: Spalte «Metadaten», Ordner-Dialog | entfallen; ein Formular (4) |
| Konzept 12, S-6 und Konzept 18, 9: «Umbenennen», «Standardstufe», «Vorschau beim Spulen» als eigene Dialoge | Felder desselben Formulars |
| Frage 31, ADR 0035: «Alle Adressen `/admin/...` samt Suchparametern bleiben» | es entfällt `?library=` auf `/admin/metadata`; alte Links landen auf dem Standard |
| Mehrere Seiten: Gruppen ab Werk offen (Geräte, Sicherung, Pflege, Spotlight, Kanäle) | ab Werk zu (5); «Laufend» und die nächste Folge bleiben offen (F-5) |
| ADR 0020: «Farben und Kontraste bleiben unverändert» | Farben und Konturen ändern sich für beide Oberflächen (6); die Dichte bleibt |
| Spec, Design-Richtung: «wenig Rahmen und Linien» | gilt weiter für Bildflächen; Bedienelemente und Flächen der Verwaltung bekommen sichtbare Konturen (6.4) |
| `packages/design-tokens/tokens/color.json`: ein Rahmen-Token, «used sparingly» | drei Rahmen-Tokens (6.2) |

Das kommt mit der Freigabe als Zeile unter **Frage 32** ins Entscheidungsprotokoll.

## 10. Fragen

| Nr. | Frage | Empfehlung | Antwort |
|---|---|---|---|
| F-1 | Folgt ein Ordner einem gemeinsamen Standard, bis er Eigenes hat, oder trägt jeder Ordner alles selbst? | Standard behalten (heute weicht nichts ab; eine Änderung statt sechs) | **«Ja, Standard behalten»** (Marco, 16. November 2026) |
| F-2 | Höherer Kontrast nur in der Verwaltung oder auch in der Zuschauer-App? | überall (dieselben Tokens) | **«Ja, überall»** (Marco, 16. November 2026) |
| F-3 | Speichert «Bearbeiten» alles in einem Zug, mit getipptem Pfad für «Entfernen» und «anderer Inhalt» beim Speichern? | ja | **«Ja, in einem Zug»** (Marco, 16. November 2026) |
| F-4 | Eine Serie liegt in zwei Ordnern: Welcher Ordner gilt? | der Ordner mit ihrer ältesten Datei (3.2); die Titelseite der Verwaltung nennt ihn | **«Ordner der ältesten Datei»** (wie empfohlen) |
| F-5 | Zwei Stellen zeigen heute beim Öffnen das Aktuelle: die Gruppe «Laufend» auf «Jobs» und die nächste Folge auf der Serienseite. Bleiben sie offen? | beide bleiben offen (sie sind der Grund, die Seite zu öffnen); alles andere ist zu | **«Beide offen»** (wie empfohlen) |
| F-6 | Zuschauer-App: Konturen für Knöpfe, Felder und Trennlinien, aber keine Rahmen um Bilder? | ja (6.4) | **«Keine Rahmen um Bilder»** (wie empfohlen) |

Antworten zu F-4 bis F-6 von Marco am 16. November 2026 über AskUserQuestion. Die Klappabschnitte
der Seite «Metadaten» und die Ordnerkarte hat Marco mit dem Mockup am 16. November 2026
freigegeben («Ja, Konzept schreiben»).

## 11. Prüfung beim Bau

- **Stufe 1, Tokens:** `contrast.test.ts` mit den neuen Schwellen; Test der verwendeten Tokens;
  Tests für Text über Bild. **Bauteile:** Storybook-Seite «Kontrast», Storybook-Bilder aller
  Bauteile neu (`task storybook:test`), Stichprobe von Hand. **Klappbares:**
  `DisclosureGroup.test.tsx`, `BandPanel.test.tsx` (Vorgabe zu), je geänderter Seite der
  Seitentest; e2e, die heute auf offene Gruppen bauen, klappen erst auf (`maintenance.spec.ts`,
  `admin-jobs.spec.ts` und die Specs der Seiten Geräte, Sicherung, Spotlight, Kanäle);
  `e2e-perf/admin-height.spec.ts` unverändert (Ausnahme 4), `JobsPage` und `SeasonShelf` mit
  ausdrücklich offenem Zustand. Tastatur (Regel 8): Aufklapper mit Enter und
  Leertaste, Fokus bleibt auf der Kopfzeile.
- **Stufe 2, Server:** Migration gegen den neuen eingefrorenen Stand (3.5); `Effective` je Titel
  (eigener Ordner, Standard, Titel ohne Heimatordner, Serie in zwei Ordnern); Heimatordner an
  jedem Weg aus 3.2 (anlegen, Staffel und Folge erben, Rückkehr, Hörbuch, Podcast aus Ordner und
  Feed, Ordner entfernt, anderer Inhalt, Dateien umgezogen), jeweils mit der Quellenliste nach dem
  Umzug; `UpdateItemsProvidersByRoot` nach Heimatordner; `AvailableMetadataLanguages` über Ordner;
  `ProviderLanguages` je Liste; Bildsprachen als Vereinigung; `apply` je Ordner mit Zählung,
  Schlüssel und einem wartenden Lauf mit `libraryIds`; Vertragstest und Schema-Test (Regel 7);
  `itemsaccess` bleibt grün (Regel 2: die neuen Abfragen liegen in `metadata` und `library`).
  **Web:** `MetadataPage.test.tsx` ohne Geltung, `admin-metadata.spec.ts` (die Fälle «library
  deviation» und «scope switch» entfallen, dafür Abschnitte, «geändert» und `?item=`),
  **Tastaturtest der Klappabschnitte der Seite «Metadaten»** (Regel 8), Ordnerkarte mit Sprachen
  und Bildern, `LibrariesPage.test.tsx` ohne Spalte «Metadaten», Test der Titelseite der
  Verwaltung (Heimatordner in den Angaben, `admin/item/facts.tsx`).
- **Stufe 3:** `saveLibrary` (alles in einer Transaktion; Reihenfolge; höchstens ein Scan und
  keiner bei reinem Umbenennen; fehlende und falsche Bestätigung; ein fehlender Ordner wird nicht
  entfernt; Pfad und Inhalt eines Ordners zugleich wird abgelehnt; 409 `stale`; 409 bei laufendem Scan; Feldfehler je Ordner; nichts geschrieben bei
  einem Fehler); `LibrariesPage.test.tsx`, `LibraryFoldersDialog.test.tsx` geht im Test des
  Formulars auf, `AudiobookSourcesPage.test.tsx`, `Setup.test.tsx`; e2e `admin-libraries.spec.ts`,
  `admin-library-folders.spec.ts`, `admin-library-draft.spec.ts`, `admin-folder-browser.spec.ts`,
  `audiobook-sources.spec.ts`, `setup.spec.ts`, dazu die Vorbereitung in `admin-podcasts.spec.ts`
  und `admin-podcast-page.spec.ts`; Tastaturtest des Formulars in Chromium und Firefox (Regel 8).
- **Texte:** neue Schlüssel in `de.json` und `en.json`, die übrigen Sprachen über den
  Übersetzungsweg von Konzept 22; entfallende Schlüssel (`admin.metadata.scope.*`, Teile von
  `admin.libraries.folders.*`) werden entfernt (`locales.test.ts`).
- **Funktionsabdeckung:** Die Tabelle 4.5 wird beim Bau von Stufe 3 Zeile für Zeile gegen die
  Oberfläche geprüft, wie beim Verwaltungs-Redesign.

## Anhang A. Mockup

Design-Leinwand **«Slouchcat – Metadaten pro Ordner»**:
<https://claude.ai/artifact/9YzJbtHiQWrMygqp7zzbPd> (privat, nur für Marco). Kopie im Repository
unter `docs/planning/mockups/24-metadaten-pro-ordner/`. Namen, Pfade und Zahlen sind der Stand von
CT 190 am 16. November 2026.

| Bild | Zeigt |
|---|---|
| 1 Bibliotheken | Tabelle ohne Spalte «Metadaten», Zeilen zu, aufgeklappt «Erneut scannen», «Bearbeiten», «Weitere Aktionen» und die Ordner mit Zusammenfassung |
| 2 Bearbeiten | das Formular, oben umschaltbar «Bearbeiten» und «Hinzufügen»; Ordnerkarten zu, «Vorschau beim Spulen» zu |
| 3 Ordner offen | eine Ordnerkarte mit eigenen Sprachen als Beispiel |
| 4 Metadaten | der Standard als Klappabschnitte, dazu «Ordner» |
| 5 Kontrast | derselbe Ausschnitt bisher und neu, mit Messwerten |

Die Bilder 1 bis 4 sind über «Play» klickbar. Wo der Text dieses Papiers genauer ist als das
Mockup, gilt der Text:

- «Bearbeiten» ist ein Dialog wie «Hinzufügen» heute; das Mockup zeichnet ihn als Fläche auf der
  Seite. Seitenleiste und Kopfzeile der Verwaltung fehlen, die Knopfhöhe ist die des Werkzeugs.
- Die Quellenliste und die Sprache je Bildart sind vereinfacht. Die Ordnerkarte zeigt die
  Überwachung nicht, «geändert» in den Kopfzeilen fehlt.
- Die Spalte heisst im Mockup «Titel», im Produkt «Titel · Dateien».
- Die Zahl «3001 Titel» ist im Mockup die Zahl der Titel mit einer Datei im Ordner. Das Band nach
  dem Speichern zählt die Titel, deren Heimatordner er ist.
- Bild 5 zeigt als Beispiel für Z-5 einen gesperrten Knopf «Zuordnen» mit Grund. Die Änderung an
  der Seite «Pflege» selbst gehört nicht zu diesem Papier (Abschnitt 1).

## 12. Kritiker-Review (Fassung 1)

Review vom 16. November 2026 gegen den Code am Anker: **REVISE**, 0 Blocker, 12 Major, 13 Minor.
Die «Heute»-Angaben stimmten bis auf die Punkte 13, 19 und 21. Nicht nachgeprüft hat der Kritiker
die Messwerte von CT 190.

| Nr. | Stufe | Befund | Eingearbeitet in |
|---|---|---|---|
| 1 | Major | Ein anderer Inhalt des Heimatordners liess überlebende Titel am falschen Ordner hängen | 3.2: Heimatordner wird neu bestimmt; 11 |
| 2 | Major | Fremdschlüssel von `items.root_id` ohne Löschregel | 3.2, 3.5: `ON DELETE SET NULL`, Index |
| 3 | Major | «Zuerst gefunden» hiess an drei Stellen Verschiedenes | 3.2, 3.5, F-4: eine Regel (älteste vorhandene Datei), Kinder erben |
| 4 | Major | Rückkehr, Hörbuch und Podcast setzten keinen Heimatordner | 3.2: alle Wege genannt |
| 5 | Major | Beim Umzug blieb die Quellenliste des alten Ordners | 3.2: Liste und Aufträge in derselben Transaktion |
| 6 | Major | Stufe 2 allein liess Spalte und Link ohne Ziel | 8: nach Stufe 2 verschoben |
| 7 | Major | «Einziger Aufrufer» stimmte nicht (e2e, Go-Tests) | 4.3: Funktionen bleiben Bausteine, e2e umstellen; 11 |
| 8 | Major | «Fehlender Ordner = entfernen» lädt zu Verlust ein | 4.3: ausdrückliche Liste, Schutz vor veraltetem Stand |
| 9 | Major | «Eine Transaktion» gab es so nicht; Reihenfolge offen | 4.3: gemeinsame Transaktion, Reihenfolge, Nebenwirkungen danach; 4.2: Pfad und Inhalt |
| 10 | Major | Z-1 widersprach «Akzentfarben bleiben» | 6.1, 6.5: Z-1 nur für `color.text.*` |
| 11 | Major | «Alles zu» verbirgt Laufendes und Ungespeichertes | 5: «geändert» in der Kopfzeile, F-5, Höhentest; 3.4, 4.2 |
| 12 | Major | Hinweis der Quellen und TMDB-Bildsprachen lesen nur den Standard | 2.2, 3.3: entschieden statt «Bau prüft» |
| 13 | Minor | Podcasts nutzen die Sprachen des Standards | 3.1; Mockup Bild 4 |
| 14 | Minor | «Leer = wie Standard» war mehrdeutig | 3.5, 3.6: `NULL`, drei Zustände |
| 15 | Minor | Wartender Lauf mit `libraryIds` hiesse nach dem Update «alle» | 3.3 |
| 16 | Minor | Tabelle 4.5 unvollständig | 4.1, 4.2, 4.4, 4.5, 3.4 |
| 17 | Minor | Ausnahme 3 unvollständig und widersprüchlich | 5 |
| 18 | Minor | ADR 0033 und Frage 31 fehlten in Abschnitt 9 | 9 |
| 19 | Minor | 2.2 zu eng (gemischte Listen, Extras) | 2.2, 3.2 |
| 20 | Minor | Z-3 und Z-5 sind mit dem Token-Test nicht prüfbar | 6.1: Spalte «Geprüft durch», Storybook-Seite |
| 21 | Minor | Liste des Gesperrten unvollständig; schwächster Textwert 4,8 | 2.5, 6.1, 6.3 |
| 22 | Minor | Papier und Mockup wichen ab | Anhang A |
| 23 | Minor | «Reiht einen Scan ein» klang nach immer | 4.3 |
| 24 | Minor | Tastaturtest und neuer DB-Stand für Stufe 2 fehlten | 3.5, 11 |
| 25 | Minor | Der Heimatordner war nirgends sichtbar | 3.2, 3.6, 8 |
