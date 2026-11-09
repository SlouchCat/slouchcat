# Konzept: Serien- und Filmseite, Bibliotheksraster und Seitenleiste

- **Status: Fassung 4 (7. November 2026), freigegeben und gemergt am 8. November 2026 (Frage 24), Kritiker-Review eingearbeitet (REVISE, 0 Blocker, 8 Major, 13 Minor; Abschnitt 14), mit klickbarem Mockup.** Marco hat am 7. November 2026
  alle Fragen **F-1 bis F-17** beantwortet (Abschnitt 11): **Serie A**,
  **Knöpfe A mit Trailer in der Reihe der Rundknöpfe**, **Sammlung nur als Reihe** (2),
  **Seitenleiste B mit Profilkopf, «Meine Medien» immer offen**. Abweichend von der Empfehlung:
  **FSK überall weg** (F-10), **Merken-Knopf nur beim Zeigen** (F-12), **Schauspielerbilder alle im
  Voraus** (F-8). Keine Frage ist mehr offen. Marco hat angekündigt, dass noch mehr dazukommen kann.
  **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Auslöser:** Marco hat am 7. November 2026 zwei Bildschirmfotos geschickt (Serienseite «Two and
  a Half Men», Filmseite «10 Cloverfield Lane») und ein drittes von der Seitenleiste auf dem Handy.
  Er ist mit der Serienseite nicht zufrieden und will klickbare Mockups nach dem Vorbild von Plex,
  Emby und Jellyfin, dazu eigene Ideen (Abschnitt 1).
- **Verankert gegen:** `main` **`88d03c11`** («Merge pull request #106», Status zu Release 1.15.0),
  Vertragsfassung **1.15.0** (`api/openapi.yaml:4`), letzte Migration **`00037_items_top_added.sql`**,
  letzte ADR in `main` **`0023`** (0024 ist für Konzept 15, 0025 für Konzept 16 reserviert), letzte
  Frage im Entscheidungsprotokoll **22** (23 ist für Konzept 16 reserviert).
- **Nummern:** Dieses Papier ist **Konzept 17** und reserviert **Frage 24** und **ADR 0026**
  («Personenbilder und Sammlungsbilder»). Vertrag und Migration werden beim Baubeginn vergeben, nach
  Konzept 15 Stufe 4 und Konzept 16 (dort vorläufig 1.17.0 und 1.18.0). Vorläufig gelten:
  **Stufe 1 ein Patch ohne Vertrag** (1.18.1), **Stufe 2 Vertrag 1.19.0** (additiv), **Stufe 3
  Vertrag 1.20.0** (additiv) mit **einer Migration** (Tabellen `person_images` und
  `collection_images`, Abschnitt 5).
- `:Zeile`-Angaben beziehen sich auf `88d03c11`; **nach Namen suchen, nicht nach Zeile.** Pfade ohne
  Präfix liegen unter `server/` (Go) oder `web/src/` (Web-App), wo es eindeutig ist.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`). Wo dieses
  Papier einen früheren Entscheid ändert, steht das in Abschnitt 10 und als Frage in Abschnitt 11.

> **Dieses Papier ändert keine Zeile Code.** Es sagt je Wunsch, was der Code heute tut, zeigt
> Varianten im Mockup (Anhang A) und schlägt eine Lösung vor.

## 1. Was Marco will

| Nr. | Marcos Wunsch (7. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| D-1 | Die Serienseite schöner: Staffelcover oder Ähnliches, andere Anordnung, Vorschläge als klickbare Mockups nach Plex, Emby, Jellyfin und eigenen Ideen. | 2 |
| D-2 | Ein Knopf «Weiterschauen» oder «Start»; bei «Weiterschauen» steht, welche Folge. Die Knöpfe oben allgemein neu ordnen. | 3 |
| D-3 | Serien- und Filmseite grundsätzlich gleich oder ähnlich aufgebaut. | 2, 3 |
| D-4 | Auf der Filmseite sehen, ob und zu welcher Sammlung der Film gehört, mit Mockups. | 4 |
| D-5 | Bilder der Schauspieler anzeigen. | 5 |
| D-6 | In der Filmbibliothek alle Filme anzeigen, mit Blick auf die Leistung; die FSK-Anzeige kann weg; die Navigation nach Buchstaben bleibt. | 6 |
| D-7 | Der Stift verhält sich wie die FSK (wird beim Darüberfahren grösser und kleiner), überall wo er steht. | 7 |
| D-8 | Die Seitenleiste schöner, nach dem Vorbild der Leiste in der Verwaltung. Normale Profile sehen nur Medien, der Hauptaccount auch die Profile. | 9 |
| D-9 | Analog zum Stift einen Knopf, der einen Film oder eine Serie auf die Merkliste nimmt, je Profil, unten rechts auf dem Cover. | 8 |
| D-10 | «Cat's Empfehlungen» umbenennen in «User-Empfehlungen» (nachgereicht am 7. November 2026). | 9.3 |

## 2. D-1, D-3 Aufbau der Serien- und Filmseite

### 2.1 Heute

- Film, Serie, Staffel und Folge zeigt **eine** Komponente `ItemView` (`features/catalog/ItemPage.tsx`,
  Route `/items/$itemId`, `app/router.tsx:283`). Reihenfolge: Hintergrund, links das Poster mit
  `PosterPen` (ab 900 px klebend, darunter ausgeblendet, `ItemPage.module.css:321-332`), rechts Logo,
  `ItemMeta` (FSK-Chip, Jahr, Laufzeit, Genres), Tagline, `Actions`, `RelatedLinks`, Fortschritt,
  Inhalt; bei Serien `Seasons`; dann «Datei» (nur Film und Folge), Extras, Sammlungen, Besetzung.
- **Staffeln** sind `ChoiceChips` mit dem Text «Staffel 1» (`Seasons`, `ItemPage.tsx:458`; die Folgen lädt `SeasonEpisodes`, :432-456).
  Darunter ein Raster `EpisodeGrid` mit `EpisodeCard` (Bild, Code, Dauer, Fortschritt), **ohne
  Inhaltstext**.
- **Staffelcover gibt es schon:** eine Staffel hat ein eigenes Poster (`itemImageKinds["season"]`,
  `metadata/images.go:149`, von TMDB über `tmdb_provider.go:313-315`). `ItemDetail.seasons` liefert je
  Staffel ein `ItemRef` mit `images.poster` und `childCount`. Die Chips nutzen es nicht.
- **Staffeltext** steht in der Datenbank, kommt aber nur über `getItem(staffel).overview`. **Jahr der
  Staffel:** `UpdateChildMatch` schreibt `release_date`, nicht `year`
  (`queries/system/metadata.sql:28-36`); `ItemRef.year` ist bei Staffeln leer.

### 2.2 Varianten (Mockup, Anhang A)

| Variante | Vorbild | Staffeln | Folgen |
|---|---|---|---|
| **A «Staffelregal»** | Emby, Jellyfin | Reihe von Staffelcovern mit Fortschritt (Balken, Haken bei «alles gesehen») | Liste unter der Reihe: Bild, Code, Titel, Dauer, zwei Zeilen Inhalt, Haken |
| **B «Staffelseite»** | Plex | Raster von Staffelcovern auf der Serienseite | eigene Staffelseite: Staffelcover, Jahr, Folgenzahl, Staffeltext, eigener Knopf, Folgen als Kacheln, «Weitere Staffeln» |
| **C «Staffelleiste»** | eigene Idee | feste Leiste links mit Mini-Cover und Fortschritt; das grosse Cover oben wechselt auf das Cover der gewählten Staffel | kompakte Liste rechts |

Für **alle drei** gilt (D-3): Film und Serie haben denselben Kopf (Pfeil zurück oben links, Cover,
Logo, Kennzahlen, Tagline, Knopfzeile, Inhalt) und darunter Abschnitte in derselben Reihenfolge:
bei der Serie Staffeln und Folgen, beim Film Datei; dann bei beiden Sammlungen (4.2), Extras und
zuletzt die Besetzung. Der Startfokus der Seite liegt auf dem grossen Knopf (heute `BACK_KEY`,
`ItemPage.tsx:799`), bei einer Serie also auf `nextUp`.

**Gewählt: A** (F-1). Alles auf einer Seite, ein Klick weniger als B, und auf dem Handy einfacher als C.
Die Staffelseite aus B ist trotzdem nützlich, wenn jemand von einer Folge «hoch» navigiert; die
Detailseite einer Staffel gibt es schon (`ItemView` mit `kind = season`) und bekommt denselben Kopf.

### 2.3 Was sich ändert (bei A)

1. Die Chips werden zu Staffelcovern (`posterSmall`), mit «Staffel N», Jahr, `childCount` Folgen
   und dem Fortschritt der Staffel. Vertrag (additiv, Stufe 2):
   - **`ItemRef.year` für Staffeln** setzt der Server aus `release_date` (heute leer; `ItemRef` hat
     kein `releaseDate`, `openapi.yaml:8395-8498`). Ohne Datum kein Jahr.
   - **`ItemRef.watchedCount`** (Ganzzahl, nur bei Staffeln gesetzt): gesehene sichtbare Folgen,
     gerechnet wie `episodeStats`.
2. Die Folgen werden eine Liste mit Inhalt (zwei Zeilen). Dafür **`ItemRef.overview`**, beschrieben
   als «nur in `ItemDetail.episodes` gesetzt», nach dem Muster von `hasTrailer`; andere Listen lassen
   es weg.
3. Der Staffeltext erscheint über der Folgenliste, wenn es einen gibt.
4. «Zurück» verlässt die Knopfzeile und wird ein runder Pfeil oben links über dem Kopf.
5. **Staffelseite** (Detailseite einer Staffel, `kind = season`): derselbe Kopf mit Staffelcover,
   Serientitel als Link, «Staffel N», Jahr, Folgenzahl, Staffeltext; grosser Knopf nach `nextUp`,
   eingeschränkt auf die Staffel; Rundknöpfe ohne Merken und Favorit (Staffeln kann man nicht
   merken); darunter die Folgenliste.

## 3. D-2 Knopfzeile und «Weiterschauen»

### 3.1 Heute

- Zeile 1: Abspielen bzw. «Weiterschauen ab {Zeit}», Zurück, Trailer, Download, Verwalten. Zeile 2:
  Merken, Favorit (nur Serie und Podcast, `ItemPage.tsx:173`), «Zu Cat's Empfehlungen» (nur
  Hauptprofil des Server-Admins, :262).
- **Eine Serie hat keinen Abspielknopf:** `playable` ist für Serie, Staffel und Podcast falsch
  (:153). Eine Serie hat keinen `progress` (`visibility/mapping.go:202`); `nextEpisode` gibt es nur
  auf einer Folge (`visibility/tree.go:166-228`). Die Gruppierung je Serie aus Konzept 15 P-1 steckt
  nur in `ListContinueWatching` für die Startseite (`queries/scoped/items.sql:46-94`).

### 3.2 Varianten der Knopfzeile (gehören im Mockup zu A, B, C)

- **A:** grosser heller Knopf mit zwei Zeilen («Weiterschauen» / «S01E02 · Der Sockengolf-Champion ·
  noch 12 min», Fortschrittsstrich); darunter runde Knöpfe **mit Beschriftung**: Trailer, Von vorne
  (nur bei Fortschritt), Merken, Favorit, User-Empfehlungen (D-10). (Fassung 1 hatte Trailer neben dem
  grossen Knopf; Marco will ihn ausgeschrieben mit Symbol in der Reihe von Merken.)
- **B:** derselbe grosse Knopf, dann nur Symbole mit Hinweis beim Zeigen; «Von vorne», «Zu Cat's
  Empfehlungen» und «Metadaten bearbeiten» im Menü «⋯».
- **C:** eine Karte «Als Nächstes» mit Folgenbild statt eines Knopfs, darunter Kapseln.

**Gewählt: A** (F-2), mit Trailer in der Reihe. Beschriftung hilft auf dem Handy und mit der Fernbedienung (Tastatur-Navigation,
Regel 8); das Menü «⋯» aus B versteckt Dinge.

Download und Verwalten bleiben, wo sie gebraucht werden: Download in der Zeile der runden Knöpfe,
Verwalten als Eintrag dort nur für den Admin (wie heute nur bei nicht bearbeitbaren Arten).

### 3.3 Welche Folge der Knopf nennt (F-4, F-5)

Grundlage ist die jüngste Aktivität der Serie wie bei «Weiterschauen» (Konzept 15, F-1/F-2), aber
**ohne die 60-Tage-Grenze**. «Erste Folge» heisst: die erste **sichtbare Folge ohne Specials**
(Staffel ≥ 1, kleinste Staffel, kleinste Nummer); gibt es nur Specials, die erste davon.
«Nächste Folge» heisst: die nächste sichtbare Folge **nach Staffel und Nummer**, ohne Specials,
**unabhängig davon, ob sie schon gesehen ist** (sonst kommt man beim Nochmalsehen nie über S01E01
hinaus; die Startseite überspringt gesehene Folgen, `items.sql:36-39`).

| Lage | `reason` | Knopf | zweite Zeile |
|---|---|---|---|
| kein Fortschritt an einer Folge | `start` | **Abspielen** | S01E01 · Titel |
| jüngste Folge angefangen | `resume` | **Weiterschauen** | SxxEyy · Titel · noch N min |
| jüngste Folge fertig, es gibt eine nächste | `next` | **Nächste Folge** | SxxEyy · Titel |
| jüngste Folge fertig, keine nächste, aber frühere ungesehene | `next` | **Nächste Folge** | erste ungesehene Folge nach Staffel und Nummer |
| alles gesehen | `rewatch` | **Nochmals ansehen** | ab erster Folge |

Beim Film: Abspielen (Laufzeit), Fortsetzen (Stelle und Restzeit), Nochmals ansehen.

Server (Stufe 2): neues Schema **`NextUp { reason: start|resume|next|rewatch, episode: ItemRef }`**
und Feld **`ItemDetail.nextUp`** (nur bei Serien; bei Staffeln auf die Staffel eingeschränkt).
Neue Abfrage `GetSeriesNextUp` in `queries/scoped/` (mit `visible_items`, Regel 2), die früh auf die
eine Serie filtert, mit Rückfall auf die erste Folge; `ListContinueWatching` bleibt unverändert.
`x-visibility: scoped`, Eintrag im Vertragstest.

## 4. D-4 Sammlung auf der Filmseite

### 4.1 Heute

- `ItemDetail.collections` gibt es (`openapi.yaml:8515`); die Seite zeigt sie als Reihe von
  `CollectionCard`s **ganz unten** nach den Extras (`ItemPage.tsx:306-324`). Darum fällt sie nicht auf.
- Eine Sammlungsseite gibt es (`/collections/$collectionId`, `CollectionPage.tsx`): Titel, Text,
  Raster mit «Mehr laden» (50 je Seite).
- `collections.poster_path` wird gespeichert, aber nie geladen; das Cover einer Sammlung ist heute das
  Poster des ersten Films (`CollectionRef.cover`).
- `collection_items` enthält **nur Titel aus der Bibliothek**: TMDB-Sammlungen Filme, manuelle
  Sammlungen alles ausser Staffeln, Folgen und Extras (`openapi.yaml:5197-5199`). Die alte Reihe
  zeigt Sammlungen bei jeder Art (`ItemPage.tsx:917`). Welche TMDB-Teile fehlen, weiss der Server
  nicht.
- `CollectionRef` hat nur `id, name, count, cover` (`openapi.yaml:11290-11305`); Teile, «dieser
  Titel» und Haken fehlen.

### 4.2 Varianten (Mockup, «Sammlung 1/2/3»)

- **1 «Hinweis»:** Kapsel unter den Genres: «Teil der Sammlung Cloverfield Filmreihe · 3 Filme».
- **2 «Reihe»:** eigener Abschnitt direkt nach dem Kopf: alle Teile in Reihenfolge (`position`),
  dieser Film markiert, gesehene mit Haken.
- **3 «Band»:** breites Band mit Sammlungsbild, Teilen als Kapseln, Fortschritt der Reihe, Knopf
  «Sammlung öffnen».

**Gewählt: nur 2 «Reihe»** (F-6; Empfehlung war 1 und 2). Der Abschnitt ersetzt die bisherige
Reihe `CollectionCard` und gilt **für jede Art**, die in einer Sammlung sein kann (Film, Serie,
Hörbuch …), nicht nur für Filme; sonst verlören Serien ihre Sammlung. Ein Titel in zwei Sammlungen
bekommt zwei Abschnitte.

Vertrag (additiv, Stufe 2): **`CollectionRef.members`**: die sichtbaren Mitglieder als `ItemRef`
(mit `progress`) in `position`-Ordnung, höchstens 20, danach «Alle zeigen» zur Sammlungsseite. Nur in
`ItemDetail.collections` gesetzt. Die Sammlungsseite bekommt denselben Kopf wie Film und Serie
(Sammlungsbild, Name, Anzahl, Text; kein Abspielknopf) und zeigt Gesehen-Haken.

**Fehlende Teile** (grau, «Nicht in der Bibliothek») zeigt das Mockup als Möglichkeit. Dafür müsste
der Server die vollständige Teileliste von TMDB speichern (neue Tabelle oder Spalte). **Empfehlung:
nicht in diesem Konzept** (F-7, so entschieden).

## 5. D-5 Bilder der Schauspieler

### 5.1 Heute

- `CastCard` zeigt nur den ersten Buchstaben (`ui/CastCard.tsx:13-26`, «There are no portraits in M3»).
- `people.profile_path` ist gefüllt (`metadata/match.go:1022`), wird aber **nie geladen**. `CastMember`
  hat kein Bildfeld (`openapi.yaml:8605`); einen Bild-Endpunkt für Personen gibt es nicht.

### 5.2 Vorschlag

- Es gibt eine **Personenseite** (`/people/$personId`, `PersonPage.tsx`, `getPerson`); das Bild gehört
  auch in deren Kopf und in die Personentreffer der Suche.
- Die Bildpipeline der Titel (`images`, `00029_item_images.sql`) hängt an `items` (`item_id NOT NULL
  REFERENCES items`) und liefert WebP und JPEG mit `Vary: Accept` und `v`-Hash
  (`openapi.yaml:1292-1302`). Personen und Sammlungen bekommen **eigene Tabellen nach diesem Muster**.

1. **Migration (Stufe 3):** Tabellen `person_images` und `collection_images` mit Quelle
   (`profile_path` bzw. `poster_path`, um Änderungen zu erkennen), Format, Hash, Breite, Höhe,
   `fetched_at`. Nach Vorlage `TEMPLATE.sql.txt`, nur vorwärts.
2. **Job `people.images`** in der Gruppe `metadata`, Pool `general` (einen eigenen Bildpool gibt es
   nicht, `jobs/registry.go:38-44`). **Ein Auftrag je Lauf**, der in Paketen (z. B. 200) die Personen
   ohne aktuelles Bild abarbeitet; nach einem Match wird höchstens ein Lauf eingereiht, nicht ein
   Auftrag je Person (sonst Tausende Einträge in der 30-Tage-Historie von Konzept 09). Pausierbar,
   mit Zeitfenster wie in Konzept 15 (`00034_job_windows.sql`). **Nachholen** für den Bestand als
   Knopf auf der Seite Hintergrundjobs, nach dem Muster von `keyframes.backfill`. Sammlungsbilder
   im selben Job.
3. **Endpunkte** `GET /people/{id}/image` und `GET /collections/{id}/image` als `http.Handler`
   (Regel 6) mit Beschreibung in `api/openapi.yaml`, `x-visibility: scoped`, Cache-Header wie bei
   Titelbildern. Sichtbarkeit (Regel 2): eine Person nur, wenn sie in mindestens einem sichtbaren
   Titel vorkommt; eine Sammlung nur mit mindestens einem sichtbaren Mitglied; sonst 404.
4. **Felder** (`ImageRef`): `CastMember.image`, `PersonRef.image`, `Person.image`,
   `CollectionRef.image`. Ohne Bild bleibt der Buchstabe bzw. das Poster des ersten sichtbaren
   Mitglieds; lädt ein Bild nicht (`onError`, offline), fällt die App ebenfalls darauf zurück.
5. **Speicher:** `w185` als WebP und JPEG, grob 30 KB je Person; bei rund 30'000 Personen etwa
   **0,9 GB** im `CacheDir`. Regenerierbar, darum nicht in der Sicherung. Messung beim Bau nach
   `docs/perf/`.
6. **Entschieden (F-8): alle im Voraus**, auch für den Bestand (Empfehlung war «beim ersten Anzeigen»).

ADR 0026 hält Tabellen, Job, Sichtbarkeitsregel und Speicherort fest, dazu die offene Frage, ob ein
TMDB-Sammlungsposter ein für das Profil verborgenes Mitglied zeigen darf (bis dahin: Sammlungsbild nur,
wenn **alle** Mitglieder sichtbar sind, sonst Poster des ersten sichtbaren Mitglieds).

## 6. D-6 Filmbibliothek: alle Filme, ohne FSK, mit Buchstaben

### 6.1 Heute

- `LibraryPage.tsx` lädt mit `useInfiniteQuery` je **50** Titel und einen Knopf «Mehr laden»
  (`features/catalog/queries.ts:91-118`); kein Nachladen beim Scrollen, keine Virtualisierung.
  `listLibraryItems` erlaubt höchstens `limit=100` (`openapi.yaml:1011-1025`).
- Die Buchstabenleiste ist ein **Filter** auf dem Server (`letter`): wer «M» drückt, sieht nur M.
  Leere Buchstaben sind gesperrt (`LetterBar.tsx:47-64`).
- Auf jedem Poster steht die FSK (`showRating`, `ItemGrid.tsx:82-86`), dazu Haken und Fortschritt.

### 6.2 Vorschlag

1. **Nachladen beim Scrollen** statt «Mehr laden»: Seitengrösse **100**. Statt `useInfiniteQuery`
   (lädt nur fortlaufend, `queries.ts:107-121`) ein **Seitenmodell mit Lücken**: die App kennt die
   Gesamtzahl, hält leere Plätze und lädt die Seite, in die gescrollt oder gesprungen wird, auch
   nach oben. `PicksPage` teilt sich heute `LIBRARY_PAGE` (`queries.ts:91`) und bleibt beim alten
   Weg.
2. **Virtualisiertes Raster** mit **`@tanstack/react-virtual`** (Version beim Bau gepinnt, neue
   Abhängigkeit): gezeichnet werden die Zeilen im Bild und je eine Bildschirmhöhe davor und danach;
   Bilder (`posterSmall`) erst kurz vor dem Sichtbarwerden.
   **Fokusregeln** (Raumnavigation mit norigin kennt nur eingehängte Elemente): am Rand des
   gezeichneten Fensters scrollt ↓/↑ zuerst die nächste Zeile ins Bild und fokussiert dann; Tab
   folgt derselben Regel, nach der letzten Karte verlässt er das Raster. Beim Öffnen eines Titels
   merkt sich die App **Index und Scrollstand**; «Zurück» stellt beides her und fokussiert dann die
   Karte (`useBackFocus` wartet heute auf eine eingehängte Karte, `useBackFocus.ts:20-26`).
3. **Buchstaben springen statt filtern** (F-9, durchgehend ohne Trenner). Der Client kann die Stelle
   nicht selbst rechnen: sortiert wird nach `sort_key` im Bytevergleich mit Artikelregeln
   (`visibility/library.go:458`), Ziffern stehen vor A und andere Anfänge hinter Z, `#` ist also
   nicht zusammenhängend; bei `order=desc` dreht alles. Darum liefert der Server bei `sort=title`
   die Facette **`letterStarts: [{letter, offset}]`** in der aktiven Richtung und auf der
   gefilterten Menge, mit demselben `ORDER BY`; `#` darf zweimal vorkommen. Facetten rechnet er nur
   bei `offset=0`. Der Sprung lädt die Seite am Versatz; die Leiste markiert beim Scrollen den
   Buchstaben, in dessen Bereich der oberste sichtbare Index liegt. Ohne `sort=title` keine Leiste
   (wie heute).
4. **Keine FSK-Anzeige mehr für Zuschauer** (F-10): weg aus dem Bibliotheksraster (heute der einzige
   Ort auf Postern, `showRating` nur `LibraryPage.tsx:426`) und aus dem Kopf der Detailseite
   (`ItemMeta`). Die Verwaltung behält sie (`AdminItemPage`, `MetadataEditor`, `RatingDialog`).
   Die Altersregel im Server bleibt unverändert; der Filter «bis FSK» (`maxRating`), der FSK-Filter
   der Suche und die Sortierung `age` bleiben.
5. Gilt für Film- und Serienbibliotheken, nicht für Podcasts und Hörbücher (die haben schon heute
   keine Buchstabenleiste).
   **Nachtrag (9. November 2026, Konzept 18 Teil C, 16.4):** Nachladen beim Scrollen und das
   virtualisierte Raster gelten auch für Hörbuch- und Podcast-Bibliotheken; nur die Buchstabenleiste
   bleibt bei Filmen und Serien. So gebaut in Stufe 3 (1.21.0, PR #128).

Messung beim Bau nach `docs/perf/`: erste Zeile und Scrollen über 1'795 Titel auf dem Handy.
Abnahme mit Playwright: ↓ über 300 Zeilen, Sprung auf «M» und Zurück auf Karte N, Tab aus dem Raster.

## 7. D-7 Der Stift

### 7.1 Was Marco beobachtet

Die Karte im Raster wird beim Zeigen grösser (`.card:hover`, Faktor 1,06, `PosterCard.module.css:159`),
mit ihr die FSK-Plakette. Der Stift liegt **neben** der Karte über ihr (`ItemGrid tool`,
`Discover.module.css:80-101`). Fährt die Maus auf den Stift, endet `:hover` der Karte und sie
schrumpft; der Stift selbst ändert nur die Farbe (`ui/Button.module.css:103-105`).

### 7.2 Vorschlag

- Die Vergrösserung hängt am **ganzen Feld** `.cell` (Karte samt Stift und Merken-Knopf,
  `Discover.module.css:91-95`): `.cell:hover` und `.cell:focus-within` skalieren die Karte; die
  Werkzeuge sitzen im selben skalierten Feld. Die Karte bleibt gross, auch auf dem Stift.
- Der Stift selbst wird beim Zeigen und mit Tastaturfokus grösser und heller; der Faktor 1,18 wird
  ein neues Token (`--sc-tool-hover-scale`, Regel 9).
- Überall, wo er steht: Detailseite (Cover), Bibliotheksraster, Reihen der Startseite, Spotlight-Reihe.
  Weiterhin nur für den Server-Admin (Frage 22, Nachtrag vom 7. November 2026). F-11: auch in der
  Spotlight-Reihe.

## 8. D-9 Merken direkt auf dem Cover

### 8.1 Heute

- Die Merkliste ist **schon je Profil** (`watchlist`, «one list per profile», `00015_discover.sql:4`).
  Merken geht heute nur auf der Detailseite (`PUT/DELETE /items/{id}/watchlist`, `openapi.yaml:4647`).
- `ItemRef` in Listen kennt den Merk-Zustand nicht; nur `ItemDetail.inWatchlist`.
- Im Raster sitzt der Gesehen-Haken unten rechts, wenn der Stift da ist.

### 8.2 Vorschlag

- Runder Knopf **unten rechts** auf dem Cover, gleiche Grösse und dasselbe Zeigen-Verhalten wie der
  Stift; leer = nicht gemerkt, gefüllt in der Akzentfarbe = gemerkt. Für **jedes Profil**.
- **Nur beim Zeigen** (F-12): erscheint, wenn die Maus auf dem Cover ist oder es den Tastaturfokus
  hat. Ein gemerkter Titel zeigt den gefüllten Knopf immer. Auf Geräten ohne Maus (`hover: none`,
  Handy und Tablet) ist er immer sichtbar, sonst wäre er dort nicht erreichbar.
- Orte: Detailseite, Bibliotheksraster, Reihen der Startseite (mit Spotlight-Reihe), Sammlungsseite,
  Suche, Merkliste, User-Empfehlungen, Personenseite. Nicht bei Staffeln, Folgen und Extras (die kann
  man heute nicht merken).
- **Platz auf dem Cover:** Der Gesehen-Haken wandert nach **oben links**, die Ecke, in der heute die
  Plakette `.badge` sitzt (`PosterCard.module.css:106-109`); sie wird mit der FSK frei. Der
  Fortschrittsbalken (`:124-128`) endet links vom Merken-Knopf.
- **Bedienung ohne Maus:** Der Knopf ist wie `CoverPen` per Tab erreichbar und erscheint mit dem
  Fokus; in der Spotlight-Reihe mit `excludeFromTabOrder` (`HomePage.tsx:295`), damit die
  Pfeiltasten-Navigation der Reihe gleich bleibt. Mit Fernbedienung bleibt «Merken» auf der
  Detailseite der Weg.
- **Offline** ist der Knopf gesperrt, wie der Trailer (`ItemPage.tsx:741-747`).
- **Vertrag (Stufe 2):** `ItemRef.inWatchlist` wird in allen Listen **immer** gesetzt (eine Abfrage
  auf den Primärschlüssel von `watchlist` je Liste, `00015_discover.sql:32-37`). Nach dem Klick
  ändert die App den Zustand sofort in **allen** Caches per `setQueriesData` (heute erneuert
  `useWatchlistMutation` nur `item`, `watchlist`, `home`, `discover/queries.ts:109-114`) und nimmt
  ihn bei einem Fehler zurück; die Bibliothek wird nicht neu geladen.
- Auf der Detailseite bleibt «Merken» zusätzlich in der Knopfzeile, weil das Cover unter 900 px
  ausgeblendet ist (F-12).

## 9. D-8 Seitenleiste

### 9.1 Heute

- `ui/SideNav.tsx`: oben Start, Suche, Merkliste, Downloads; Gruppe «Meine Medien» mit den
  Bibliotheken und «Cat's Empfehlungen»; unten (`footer`, `app/AppLayout.tsx:240-266`) Verwaltung (nur
  Server-Admin), **Profile (nur Hauptprofil eines Kontos)**, Wiedergabe (alle), Abmelden.
- Bewusst **ohne Symbole** (Konzept 08, N-2). Die Verwaltungsleiste (`ui/AdminSideNav.tsx`) hat
  Symbole, farbige Bänder je Gruppe und Kapseln.

### 9.2 Varianten (Mockup, Seite «Seitenleiste»)

- **A «wie die Verwaltung»:** Symbol vor jedem Eintrag; «Meine Medien» als Band in der Akzentfarbe
  mit Pfeil; Bibliotheken mit Symbol nach Art (Film, Serie, Podcast, Ordner) und Zahl als Kapsel;
  aktive Seite mit Akzentstrich. Unten, durch eine Linie getrennt: Wiedergabe, Profile, Verwaltung
  mit Update-Kapsel, Abmelden.
- **B «mit Profilkopf»:** wie A, oben eine Karte mit dem aktiven Profil und «Wechseln»; unten ein
  zweites Band «Konto».

Wer was sieht (in beiden): **weiteres Profil:** Medien, Wiedergabe, Abmelden; **Hauptprofil eines
Kontos:** dazu Profile; **Server-Admin:** dazu Verwaltung. Das entspricht der heutigen Regel; neu ist
nur das Aussehen.

**Gewählt: B** (F-13; Empfehlung war A). Dazu: **«Meine Medien» ist immer offen**, ohne Zuklappen;
der Schalter und der Gerätespeicher `slouchcat.sidenav.libraries` fallen weg. «Wechseln» führt zur
bestehenden Profilwahl. «Profile» sieht weiterhin das Hauptprofil **jedes** Kontos (F-14).

Symbole je Bibliotheksart (`LibraryKind`): Film und gemischtes Video Filmstreifen, Serien Fernseher,
Podcasts Mikrofon, Hörbücher Buch, User-Empfehlungen Stern. Kommt Konzept 16 vorher, steht
«Radio» mit Radio-Symbol in der oberen Gruppe (`16-webradio.md:443`). Einen schmalen Zustand nur mit
Symbolen gibt es weiterhin nicht (die Leiste wird ausgeblendet, nicht eingeklappt).

### 9.3 D-10 «User-Empfehlungen»

- Heute heisst die vom Server-Admin kuratierte Liste fest «Cat's Empfehlungen» (englisch «Cat's
  picks»): 7 Stellen in `web/src/locales/de.json` (u. a. `nav`/`home.picks` :419, `addToPicks`
  :599, Seitentitel :1745, Meldungen :1822 und :1836, Verwaltung :2164 und :4048), ebenso in
  `en.json`.
- **Entschieden (F-17): fester Text «User-Empfehlungen»**, für alle Profile gleich, nicht vom
  Profilnamen abgeleitet und nicht einstellbar. Englisch **«User picks»**. Knopf: «Zu den
  User-Empfehlungen», entfernen: «Aus den User-Empfehlungen entfernen».
- Nur Texte: keine Änderung an Vertrag, Datenbank oder Routen (`/picks` und der Reihenschlüssel
  `picks` bleiben). Stufe 1. Tests und Storybook-Baselines, die den Text prüfen, ziehen mit.
- Frühere Einträge im Entscheidungsprotokoll behalten den alten Namen; sie beschreiben ihren Stand.
- **Hinweis (8. November 2026, Konzept 18 Teil B, Frage 25):** Die Admin-Pflege der Liste fällt weg;
  Profile des Besitzerkontos empfehlen selbst über einen Stern «Empfehlen» / «Empfohlen ✓». Der
  Knopf «Zu den User-Empfehlungen» der Knopfzeile (Stufe 2) wird dann zu diesem Stern, nicht
  zusätzlich gebaut.

## 10. Was sich gegenüber früheren Entscheiden ändert

| Früher | Neu | Frage |
|---|---|---|
| Konzept 08 N-2: Seitenleiste ohne Symbole | Symbole wie in der Verwaltung, Profilkopf | F-13 |
| Konzept 08: Gruppe «Meine Medien» zuklappbar | immer offen | F-13 |
| Name «Cat's Empfehlungen» | «User-Empfehlungen» | F-17 |
| FSK im Kopf der Detailseite (`ItemMeta`) | keine FSK-Anzeige mehr | F-10 |
| Buchstabenleiste filtert (Konzept 07) | Buchstabe springt in der ganzen Liste | F-9 |
| FSK auf jedem Poster der Bibliothek | ohne FSK | F-10 |
| Sammlungen als Reihe ganz unten | Abschnitt «Reihe» direkt nach dem Kopf | F-6 |
| Konzept 15 F-1/F-2: Weiterschauen-Regel nur für die Startseite | dieselbe Regel auf der Serienseite, ohne 60-Tage-Grenze | F-4 |
| Merken nur auf der Detailseite | auch auf jedem Cover | F-12 |

## 11. Fragen an Marco und seine Antworten (7. November 2026)

| Nr. | Frage | Empfehlung | Antwort |
|---|---|---|---|
| F-1 | Aufbau der Serienseite: A «Staffelregal», B «Staffelseite» oder C «Staffelleiste»? | A | **A** |
| F-2 | Knopfzeile: A beschriftete Rundknöpfe, B nur Symbole mit «⋯», C Karte «Als Nächstes»? | A | **A, Trailer ausgeschrieben mit Symbol in der Reihe von Merken** |
| F-3 | Film gleich aufgebaut wie die Serie (gleicher Kopf, gleiche Knopfzeile)? | ja | **ja, gleich** |
| F-4 | Welche Folge nennt der Knopf? | jüngste Aktivität ohne 60-Tage-Grenze, sonst S01E01 | **wie empfohlen** |
| F-5 | Bei «alles gesehen»? | Nochmals ansehen ab S01E01 | **wie empfohlen** |
| F-6 | Sammlung auf der Filmseite: 1 Hinweis, 2 Reihe, 3 Band? | 1 und 2 | **nur 2 «Reihe»** |
| F-7 | Fehlende Teile einer Sammlung grau zeigen? | nein, später | **nein, später** |
| F-8 | Schauspielerbilder: wann laden? | beim ersten Anzeigen | **alle im Voraus, mit Nachholen** |
| F-9 | Bibliothek: Springen oder Filtern, mit oder ohne Trenner? | Springen, durchgehend | **wie empfohlen** |
| F-10 | FSK nur im Raster weg oder überall? | nur im Raster | **überall weg** |
| F-11 | Stift: Karte bleibt gross, Stift wächst; auch in der Spotlight-Reihe? | ja | **ja, überall** |
| F-12 | Merken-Knopf auf dem Cover: immer oder beim Zeigen? | immer; «Merken» bleibt in der Knopfzeile | **nur beim Zeigen**; Knopfzeile behält «Merken» (Annahme, weil das Cover auf dem Handy dort fehlt) |
| F-13 | Seitenleiste: A oder B? | A | **B mit Profilkopf, «Meine Medien» immer offen** |
| F-14 | Wer sieht «Profile»? | Hauptprofil jedes Kontos | **wie empfohlen** |
| F-15 | «Wiedergabe» bleibt für alle Profile in der Leiste? | ja | **ja, für alle** |
| F-16 | Bau in Stufen wie in Abschnitt 12? | ja | **drei Stufen, je ein Release** |
| F-17 | Neuer Name für «Cat's Empfehlungen»: Name des Admin-Profils, frei wählbar oder fest? | Name des Admin-Profils | **fest «User-Empfehlungen»** |

## 12. Bauplan in Stufen (vorläufig)

| Stufe | Inhalt | Vertrag | Migration |
|---|---|---|---|
| 1 | D-7 Stift, D-8 Seitenleiste, FSK weg (D-6 Punkt 4), D-10 Umbenennung; Patch (vorläufig 1.18.1) | nein | nein |
| 2 | D-1 bis D-4 Detailseite: Kopf, Knopfzeile, `NextUp`, Staffelcover, `watchedCount`, `overview`, Sammlung mit `members`; D-9 Merken auf dem Cover mit `inWatchlist` | 1.19.0 additiv | nein |
| 3 | D-5 Personen- und Sammlungsbilder (ADR 0026); D-6 Seitenmodell, Virtualisierung, `letterStarts` | 1.20.0 additiv | ja (`person_images`, `collection_images`) |

Jede Seite bekommt ihren Playwright-Tastaturtest (Regel 8); neue Bauteile in `web/src/ui` mit
Storybook-Geschichte. Texte nur aus `locales/{de,en}.json` (Regel 9).

Betroffene Tests, die mitziehen: `e2e/library.spec.ts` (Buchstabenfilter :31-69, «Mehr laden»
:52-54), `detail.spec.ts` (Knopf «Zurück»), `collection.spec.ts`, der Test zum Zuklappen der
Seitenleiste, Storybook-Baselines von `PosterCard`, `CastCard`, `LetterBar`, `SideNav`.

Nachzuführen beim Bau: ADR 0016 Punkt (f) und die Zeilen zum Buchstabenfilter (`0016:50,64`),
Konzept 08 N-2 (Symbole) und die zuklappbare Gruppe, Nummern von Vertrag und Migration in Frage 24.

## 13. Was nicht dazugehört

- Lebenslauf auf der Personenseite (die Seite gibt es; sie bekommt nur das Bild, 5.2).
- Empfehlungen «Ähnliche Titel» (Nicht-Ziel «algorithmische Empfehlungen»).
- Fehlende Sammlungsteile (F-7).

## 14. Kritiker-Review von Fassung 3 (7. November 2026)

Urteil **REVISE**, 0 Blocker, 8 Major, 13 Minor. Die Entscheide F-1 bis F-16 wurden nicht in Frage
gestellt; alle Befunde betreffen die Machbarkeit und sind eingearbeitet.

| Befund | Inhalt | Eingearbeitet in |
|---|---|---|
| M-1 | Buchstabensprung: `sort_key` mit Artikelregeln, `#` nicht zusammenhängend, `desc`, `useInfiniteQuery` nur fortlaufend | 6.2 Punkt 1 und 3 (`letterStarts`, Seitenmodell mit Lücken) |
| M-2 | Virtualisierung gegen Raumnavigation, Zurück-Fokus, Tab; keine Bibliothek genannt | 6.2 Punkt 2 (`@tanstack/react-virtual`, Fokusregeln, Playwright-Abnahme) |
| M-3 | Vertrag für die Reihe fehlt; Sammlungen nicht nur Filme | 4.1, 4.2 (`CollectionRef.members`, Reihe für jede Art) |
| M-4 | `nextUp`-Regel unvollständig (Nochmalsehen, frühere ungesehene, Specials), Form offen | 3.3 (`NextUp`, Regeltabelle, `GetSeriesNextUp`) |
| M-5 | Staffeljahr, gesehene Folgen je Staffel, Folgentext ohne Vertragsform | 2.3 (`year`, `watchedCount`, `overview`) |
| M-6 | Personen- und Sammlungsbilder passen nicht zur Bildpipeline | 5.2 (`person_images`, `collection_images`, Endpunkte, `ImageRef`) |
| M-7 | Job falsch verankert (kein Bildpool, Auftrag je Person, Fenster) | 5.2 Punkt 2 |
| M-8 | `inWatchlist`: Bedeutung und Caches offen | 8.2 (immer gesetzt, `setQueriesData`) |
| m-1 | Personenseite gibt es | 5.2, 13 |
| m-2 | Zeilenangaben | 2.1, 7.1 |
| m-3 | FSK heute nur im Raster; Verwaltung, Suche, Sortierung ausnehmen | 6.2 Punkt 4 |
| m-4 | Überlappung Merken/Balken und Haken/Plakette | 8.2 |
| m-5 | Vergrösserung am ganzen Feld, Token | 7.2 |
| m-6 | Bedienung ohne Maus, Spotlight, weitere Orte | 8.2 |
| m-7 | Nummern: Patch, 1.19.0, 1.20.0 | Kopf, 12 |
| m-8 | ADR 0016, N-2, Radio, Symbole je Art | 9.2, 12 |
| m-9 | betroffene Tests | 12 |
| m-10 | Offline: Merken sperren, Bildrückfall | 5.2, 8.2 |
| m-11 | Speicher rund 0,9 GB, `CacheDir` | 5.2 Punkt 5 |
| m-12 | Staffel- und Sammlungsseite, Startfokus, Extras in der Reihenfolge | 2.2, 2.3 Punkt 5, 4.2 |
| m-13 | Sammlungsposter mit verborgenem Mitglied | 5.2 (ADR 0026) |

## Anhang A. Mockup

Privat; Marco kann es teilen. Die Kopie im Repository enthält keine Filmbilder, nur Farbflächen.

| Inhalt | Artefakt | Kopie im Repository |
|---|---|---|
| Fassung 2 mit Marcos Wahl als Voreinstellung. Serie (A/B/C), Film mit Sammlung 1/2/3, Staffel- und Sammlungsseite, Bibliothek mit 1'248 Beispielfilmen, Seitenleiste A/B; Fortschritt, Gerät und Rolle umschaltbar | <https://claude.ai/artifact/Pq9qcLkKG2suAi4kMUvbWZ> | `docs/planning/mockups/17-detailseiten-prototyp.html` |
