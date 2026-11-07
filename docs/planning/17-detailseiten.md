# Konzept: Serien- und Filmseite, Bibliotheksraster und Seitenleiste

- **Status: Fassung 2 (7. November 2026), mit klickbarem Mockup.** Marco hat am 7. November 2026
  die Fragen **F-1, F-2, F-4 bis F-10, F-12 bis F-14** beantwortet (Abschnitt 11): **Serie A**,
  **Knöpfe A mit Trailer in der Reihe der Rundknöpfe**, **Sammlung nur als Reihe** (2),
  **Seitenleiste B mit Profilkopf, «Meine Medien» immer offen**. Abweichend von der Empfehlung:
  **FSK überall weg** (F-10), **Merken-Knopf nur beim Zeigen** (F-12), **Schauspielerbilder alle im
  Voraus** (F-8). Offen sind nur **F-3, F-11, F-15, F-16** (Empfehlung angenommen, Bestätigung
  ausstehend). Marco hat angekündigt, dass noch mehr dazukommen kann.
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
  Konzept 15 Stufe 4 und Konzept 16. Vorläufig gelten **Vertrag 1.19.0 (additiv)** und **eine
  Migration** (Personenbilder, Abschnitt 8).
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

## 2. D-1, D-3 Aufbau der Serien- und Filmseite

### 2.1 Heute

- Film, Serie, Staffel und Folge zeigt **eine** Komponente `ItemView` (`features/catalog/ItemPage.tsx`,
  Route `/items/$itemId`, `app/router.tsx:283`). Reihenfolge: Hintergrund, links das Poster mit
  `PosterPen` (ab 900 px klebend, darunter ausgeblendet, `ItemPage.module.css:321-332`), rechts Logo,
  `ItemMeta` (FSK-Chip, Jahr, Laufzeit, Genres), Tagline, `Actions`, `RelatedLinks`, Fortschritt,
  Inhalt; bei Serien `Seasons`; dann «Datei» (nur Film und Folge), Extras, Sammlungen, Besetzung.
- **Staffeln** sind `ChoiceChips` mit dem Text «Staffel 1» (`Seasons`, `ItemPage.tsx:432-456`).
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
bei der Serie Staffeln und Folgen, beim Film Sammlung und Datei; bei beiden zuletzt die Besetzung.

**Gewählt: A** (F-1). Alles auf einer Seite, ein Klick weniger als B, und auf dem Handy einfacher als C.
Die Staffelseite aus B ist trotzdem nützlich, wenn jemand von einer Folge «hoch» navigiert; die
Detailseite einer Staffel gibt es schon (`ItemView` mit `kind = season`) und bekommt denselben Kopf.

### 2.3 Was sich ändert (bei A)

1. Die Chips werden zu Staffelcovern (`posterSmall`), mit «Staffel N», Jahr aus `releaseDate`,
   `childCount` Folgen und dem Fortschritt der Staffel (neu: gesehene Folgen je Staffel im
   `ItemRef` der Staffel, additiv).
2. Die Folgen werden eine Liste mit Inhalt (zwei Zeilen). Dafür braucht `ItemDetail.episodes`
   den Inhalt je Folge (`overview`, heute nur auf der Detailseite der Folge); additiv.
3. Der Staffeltext erscheint über der Folgenliste, wenn es einen gibt.
4. «Zurück» verlässt die Knopfzeile und wird ein runder Pfeil oben links über dem Kopf.

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
  (nur bei Fortschritt), Merken, Favorit, Cat's Empfehlungen. (Fassung 1 hatte Trailer neben dem
  grossen Knopf; Marco will ihn ausgeschrieben mit Symbol in der Reihe von Merken.)
- **B:** derselbe grosse Knopf, dann nur Symbole mit Hinweis beim Zeigen; «Von vorne», «Zu Cat's
  Empfehlungen» und «Metadaten bearbeiten» im Menü «⋯».
- **C:** eine Karte «Als Nächstes» mit Folgenbild statt eines Knopfs, darunter Kapseln.

**Gewählt: A** (F-2), mit Trailer in der Reihe. Beschriftung hilft auf dem Handy und mit der Fernbedienung (Tastatur-Navigation,
Regel 8); das Menü «⋯» aus B versteckt Dinge.

Download und Verwalten bleiben, wo sie gebraucht werden: Download in der Zeile der runden Knöpfe,
Verwalten als Eintrag dort nur für den Admin (wie heute nur bei nicht bearbeitbaren Arten).

### 3.3 Welche Folge der Knopf nennt

Vorschlag, eine Regel für Serie und Startseite (Konzept 15, F-1/F-2):

| Lage | Knopf | zweite Zeile |
|---|---|---|
| nichts gesehen | **Abspielen** | S01E01 · Titel |
| eine Folge angefangen (jüngste Aktivität) | **Weiterschauen** | SxxEyy · Titel · noch N min |
| jüngste Folge fertig, es gibt eine nächste | **Nächste Folge** | SxxEyy · Titel |
| alles gesehen | **Nochmals ansehen** | ab S01E01 |

Beim Film: Abspielen (Laufzeit), Fortsetzen (Stelle und Restzeit), Nochmals ansehen.

Server: neues Feld `ItemDetail.nextUp` (Folge als `ItemRef` mit `progress`, dazu `reason`:
`start`, `resume`, `next`, `rewatch`) für Serien, berechnet mit derselben Abfrage wie
`ListContinueWatching`, aber für **eine** Serie und **ohne** die 60-Tage-Grenze der Startseite
(F-4). Additiv, `x-visibility: scoped`.

## 4. D-4 Sammlung auf der Filmseite

### 4.1 Heute

- `ItemDetail.collections` gibt es (`openapi.yaml:8515`); die Seite zeigt sie als Reihe von
  `CollectionCard`s **ganz unten** nach den Extras (`ItemPage.tsx:306-324`). Darum fällt sie nicht auf.
- Eine Sammlungsseite gibt es (`/collections/$collectionId`, `CollectionPage.tsx`): Titel, Text,
  Raster mit «Mehr laden» (50 je Seite).
- `collections.poster_path` wird gespeichert, aber nie geladen; das Cover einer Sammlung ist heute das
  Poster des ersten Films (`CollectionRef.cover`).
- `collection_items` enthält **nur Filme aus der Bibliothek**. Welche Teile fehlen, weiss der Server
  nicht.

### 4.2 Varianten (Mockup, «Sammlung 1/2/3»)

- **1 «Hinweis»:** Kapsel unter den Genres: «Teil der Sammlung Cloverfield Filmreihe · 3 Filme».
- **2 «Reihe»:** eigener Abschnitt direkt nach dem Kopf: alle Teile in Reihenfolge (`position`),
  dieser Film markiert, gesehene mit Haken.
- **3 «Band»:** breites Band mit Sammlungsbild, Teilen als Kapseln, Fortschritt der Reihe, Knopf
  «Sammlung öffnen».

**Gewählt: nur 2 «Reihe»** (F-6; Empfehlung war 1 und 2). Der Abschnitt steht direkt nach dem Kopf,
vor «Datei»; die bisherige Reihe `CollectionCard` ganz unten fällt weg. Ein Film in zwei Sammlungen
bekommt zwei Abschnitte. Die Sammlungsseite bekommt denselben Kopf wie Film und Serie und zeigt
Gesehen-Haken.

**Fehlende Teile** (grau, «Nicht in der Bibliothek») zeigt das Mockup als Möglichkeit. Dafür müsste
der Server die vollständige Teileliste von TMDB speichern (neue Tabelle oder Spalte). **Empfehlung:
nicht in diesem Konzept** (F-7, so entschieden).

## 5. D-5 Bilder der Schauspieler

### 5.1 Heute

- `CastCard` zeigt nur den ersten Buchstaben (`ui/CastCard.tsx:13-26`, «There are no portraits in M3»).
- `people.profile_path` ist gefüllt (`metadata/match.go:1022`), wird aber **nie geladen**. `CastMember`
  hat kein Bildfeld (`openapi.yaml:8605`); einen Bild-Endpunkt für Personen gibt es nicht.

### 5.2 Vorschlag

1. Neuer Job `people.images`: lädt für jede Person mit `profile_path` das TMDB-Bild in Grösse
   `w185` in den Bild-Speicher (wie Titelbilder), gestartet nach jedem Match und einmal als Nachholen
   für den Bestand. Grob 15 KB je Person; bei 2'000 Titeln mit je 15 Personen rund 30'000 Personen,
   also etwa **450 MB** (Messung beim Bau).
2. Migration: `people.image_path` und `people.image_fetched_at`.
3. Streaming-Endpunkt `GET /people/{id}/image` als `http.Handler` (Regel 6), `x-visibility: scoped`:
   nur, wenn die Person in einem sichtbaren Titel vorkommt; sonst 404 (Regel 2).
4. `CastMember.image` (URL, additiv). Ohne Bild bleibt der Buchstabe.
5. Dasselbe Muster für `collections.poster_path` (Sammlungsbild statt Poster des ersten Films).

**Entschieden (F-8): alle im Voraus**, auch für den Bestand (Empfehlung war «beim ersten Anzeigen»).
Der Job läuft im Pool der Bild-Jobs, ist pausierbar und hat einen Knopf «Nachholen» auf der Seite
Hintergrundjobs. ADR 0026 hält Speicherort, Grösse und Sichtbarkeitsregel fest.

## 6. D-6 Filmbibliothek: alle Filme, ohne FSK, mit Buchstaben

### 6.1 Heute

- `LibraryPage.tsx` lädt mit `useInfiniteQuery` je **50** Titel und einen Knopf «Mehr laden»
  (`features/catalog/queries.ts:91-118`); kein Nachladen beim Scrollen, keine Virtualisierung.
  `listLibraryItems` erlaubt höchstens `limit=100` (`openapi.yaml:1011-1025`).
- Die Buchstabenleiste ist ein **Filter** auf dem Server (`letter`): wer «M» drückt, sieht nur M.
  Leere Buchstaben sind gesperrt (`LetterBar.tsx:47-64`).
- Auf jedem Poster steht die FSK (`showRating`, `ItemGrid.tsx:82-86`), dazu Haken und Fortschritt.

### 6.2 Vorschlag

1. **Nachladen beim Scrollen** statt «Mehr laden»: die App lädt die nächste Seite, sobald das Ende
   näher als zwei Bildschirmhöhen ist. Seitengrösse **100**.
2. **Virtualisiertes Raster:** gezeichnet werden nur die Zeilen im Bild und je eine Bildschirmhöhe
   davor und danach. Bilder laden erst kurz vor dem Sichtbarwerden (`posterSmall`). So bleiben auch
   1'795 Filme flüssig, auf dem Handy wie am Fernseher.
3. **Buchstaben springen statt filtern** (F-9, durchgehend ohne Trenner): Der Server liefert in `facets` neu die Zahl der Titel je
   Buchstabe (additiv). Daraus rechnet die App den Versatz, lädt die Seite dort und springt hin. Die
   Leiste markiert beim Scrollen den aktuellen Buchstaben.
4. **Keine FSK mehr, nirgends** (F-10): nicht auf den Postern der Bibliothek, der Startseite und der
   Sammlungen, und nicht im Kopf der Detailseite. Haken und Fortschritt bleiben. Die Altersregel im
   Server bleibt unverändert; nur die Anzeige fällt weg. Der Filter «bis FSK» in der Bibliothek
   (`maxRating`) bleibt.
5. Gilt für Film- und Serienbibliotheken, nicht für Podcasts und Hörbücher (die haben schon heute
   keine Buchstabenleiste).

Messung beim Bau nach `docs/perf/`: erste Zeile und Scrollen über 1'795 Titel auf dem Handy.

## 7. D-7 Der Stift

### 7.1 Was Marco beobachtet

Die Karte im Raster wird beim Zeigen grösser (`.card:hover`, Faktor 1,06, `PosterCard.module.css:154`),
mit ihr die FSK-Plakette. Der Stift liegt **neben** der Karte über ihr (`ItemGrid tool`,
`Discover.module.css:80-101`). Fährt die Maus auf den Stift, endet `:hover` der Karte und sie
schrumpft; der Stift selbst ändert nur die Farbe (`ui/Button.module.css:103-105`).

### 7.2 Vorschlag

- Die Vergrösserung hängt am **ganzen Feld** (Karte samt Stift und Merken-Knopf): die Karte bleibt
  gross, solange die Maus irgendwo auf ihr ist, auch auf dem Stift.
- Der Stift selbst wird beim Zeigen und mit Tastaturfokus grösser (Faktor 1,18) und heller.
- Überall, wo er steht: Detailseite (Cover), Bibliotheksraster, Reihen der Startseite, Spotlight-Reihe.
  Weiterhin nur für den Server-Admin (Frage 22, Nachtrag vom 7. November 2026).

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
- Orte wie der Stift: Detailseite, Bibliotheksraster, Reihen der Startseite; dazu Sammlungsseite und
  Suche. Nicht bei Staffeln, Folgen und Extras (die kann man heute nicht merken).
- Der Gesehen-Haken wandert nach **oben links**.
- `ItemRef.inWatchlist` (additiv, nur in den Listen, die Covers mit Knopf zeigen); nach dem Klick
  ändert die App den Zustand sofort und nimmt ihn bei einem Fehler zurück.
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

## 10. Was sich gegenüber früheren Entscheiden ändert

| Früher | Neu | Frage |
|---|---|---|
| Konzept 08 N-2: Seitenleiste ohne Symbole | Symbole wie in der Verwaltung, Profilkopf | F-13 |
| Konzept 08: Gruppe «Meine Medien» zuklappbar | immer offen | F-13 |
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
| F-3 | Film gleich aufgebaut wie die Serie (gleicher Kopf, gleiche Knopfzeile)? | ja | offen (Marcos Wunsch D-3 spricht dafür) |
| F-4 | Welche Folge nennt der Knopf? | jüngste Aktivität ohne 60-Tage-Grenze, sonst S01E01 | **wie empfohlen** |
| F-5 | Bei «alles gesehen»? | Nochmals ansehen ab S01E01 | **wie empfohlen** |
| F-6 | Sammlung auf der Filmseite: 1 Hinweis, 2 Reihe, 3 Band? | 1 und 2 | **nur 2 «Reihe»** |
| F-7 | Fehlende Teile einer Sammlung grau zeigen? | nein, später | **nein, später** |
| F-8 | Schauspielerbilder: wann laden? | beim ersten Anzeigen | **alle im Voraus, mit Nachholen** |
| F-9 | Bibliothek: Springen oder Filtern, mit oder ohne Trenner? | Springen, durchgehend | **wie empfohlen** |
| F-10 | FSK nur im Raster weg oder überall? | nur im Raster | **überall weg** |
| F-11 | Stift: Karte bleibt gross, Stift wächst; auch in der Spotlight-Reihe? | ja | offen |
| F-12 | Merken-Knopf auf dem Cover: immer oder beim Zeigen? | immer; «Merken» bleibt in der Knopfzeile | **nur beim Zeigen**; Knopfzeile behält «Merken» (Annahme, weil das Cover auf dem Handy dort fehlt) |
| F-13 | Seitenleiste: A oder B? | A | **B mit Profilkopf, «Meine Medien» immer offen** |
| F-14 | Wer sieht «Profile»? | Hauptprofil jedes Kontos | **wie empfohlen** |
| F-15 | «Wiedergabe» bleibt für alle Profile in der Leiste? | ja | offen |
| F-16 | Bau in Stufen wie in Abschnitt 12? | ja | offen |

## 12. Bauplan in Stufen (vorläufig)

| Stufe | Inhalt | Vertrag | Migration |
|---|---|---|---|
| 1 | D-7 Stift, D-8 Seitenleiste, FSK überall weg (D-6 Punkt 4) | nein | nein |
| 2 | D-1 bis D-4 Detailseite: Kopf, Knopfzeile, `nextUp`, Staffelcover, Folgenliste, Sammlung; D-9 Merken auf dem Cover | additiv | nein |
| 3 | D-5 Personen- und Sammlungsbilder (ADR 0026); D-6 Nachladen, Virtualisierung, Buchstabensprung | additiv | ja |

Jede Seite bekommt ihren Playwright-Tastaturtest (Regel 8); neue Bauteile in `web/src/ui` mit
Storybook-Geschichte. Texte nur aus `locales/{de,en}.json` (Regel 9).

## 13. Was nicht dazugehört

- Personenseiten mit Lebenslauf und Filmografie (Klick auf eine Person sucht weiter wie heute).
- Empfehlungen «Ähnliche Titel» (Nicht-Ziel «algorithmische Empfehlungen»).
- Fehlende Sammlungsteile (F-7).

## Anhang A. Mockup

Privat; Marco kann es teilen. Die Kopie im Repository enthält keine Filmbilder, nur Farbflächen.

| Inhalt | Artefakt | Kopie im Repository |
|---|---|---|
| Fassung 2 mit Marcos Wahl als Voreinstellung. Serie (A/B/C), Film mit Sammlung 1/2/3, Staffel- und Sammlungsseite, Bibliothek mit 1'248 Beispielfilmen, Seitenleiste A/B; Fortschritt, Gerät und Rolle umschaltbar | <https://claude.ai/artifact/Pq9qcLkKG2suAi4kMUvbWZ> | `docs/planning/mockups/17-detailseiten-prototyp.html` |
