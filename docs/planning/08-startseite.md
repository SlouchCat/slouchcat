# Konzept: Startseite, Seitenleiste und Bibliotheksansichten

- **Status: freigegeben, in Umsetzung (seit 29. Oktober 2026, Zweig `claude/startseite`).** Marco hat
  zwölf Fragen in drei Runden beantwortet (Abschnitt 11, F-1 bis F-12), vier Wünsche nachgereicht
  (N-1 bis N-4) und die offenen Punkte O-1, O-3, O-4, O-5 wie vorgeschlagen entschieden. Der
  OMC-Kritiker hat die erste Fassung geprüft (1 Blocker, 18 Major, 14 Minor; Abschnitt 13);
  Fassung 2 arbeitet alle ein. Ein klickbarer Prototyp liegt unter
  `docs/planning/mockups/08-startseite-prototyp.html` (Abschnitt 7.5). Marco hat die Umsetzung am
  29. Oktober 2026 nach dem Merge von Konzept 07 (PR #45) beauftragt («Beginne mit der
  Entwicklung»); Schritt 0 trägt Frage 14 und ADR 0016 nach.
- **Datum: 29. Oktober 2026, Fassung 2 vom selben Tag; Fassung 3 (Nummern nach dem Merge von 07)
  vom selben Tag**
- **Wogegen dieses Konzept verankert ist:** `main`, Stand **`4049724`** («Merge pull request #45»,
  Bibliotheken), Vertragsfassung **1.3.0**, letzte Migration **00025** (`items.filled_providers`,
  aus dem Review von 07), letzte ADR **0015**. Es setzt das Konzept **07 Bibliotheken**
  (`docs/planning/07-bibliotheken.md`, gemergt am 29. Oktober 2026) **voraus**: Familie je
  Bibliothek (`LibraryKind` = `video` / `audiobooks` / `podcasts`), Inhaltsart je Ordner
  (`RootContent`), Vertrag 1.3.0, Migrationen 00023 bis 00025, ADR 0015. Die Nummern dieses
  Papiers sind darum **Migration 00026, ADR 0016, Vertrag 1.4.0** (Fassung 2 nannte 00025; 07 hat
  diese Nummer im Review belegt). `:Zeile`-Angaben beziehen sich auf `9923807` (Fassung 1 und 2);
  nach Namen suchen, nicht nach Zeile.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
- **Jeder Schritt ist ein eigener Commit, und vor dem Abschluss jedes Schritts ist `task ci` grün**
  (`CLAUDE.md`, «Arbeitsweise»). DoD-Belege sind Befehl **und** Ausgabe.

> **Dieses Papier selbst ändert keine Zeile Code.** Die Arbeit beginnt mit Schritt 0 aus
> Abschnitt 10, einem reinen Doku-Commit.

> **Abgrenzung.** Konzept 07 schliesst in Abschnitt 2.1 ausdrücklich aus: «Eine
> Bibliotheksnavigation für Zuschauer (Kacheln Filme, Dokus, Kids wie in Emby) ist nicht Teil dieses
> Konzepts … eigenes Vorhaben mit eigenen Reihen, eigener Startseiten-Regel und eigenem
> Tastaturtest.» **Dieses Papier ist dieses Vorhaben.** Es baut auf dem, was 07 im Server anlegt
> (Familie, Inhaltsart je Ordner), und fügt dem Zuschauer die Navigation und die Ansichten hinzu.

---

## 1. Ausgangslage und Problem

### 1.1 Was die Zuschauerseite heute kann

- **Navigation.** Eine feste Kopfzeile (`web/src/ui/AppHeader.tsx`, 4 rem) mit horizontalen
  Pillen, `aria-label` «Hauptnavigation»: Start, Suche, Merkliste, Downloads (nur mit
  Offline-Unterstützung), Verwaltung (nur Administrator), Profile (nur Hauptprofil); rechts Glocke,
  Update-Hinweis (Administrator), Profilwechsel, Abmelden (`web/src/app/AppLayout.tsx:64` bis
  `:132`). **Keine Seitenleiste, keine Bibliotheken in der Navigation.** Die Taste `/` öffnet die
  Suche von jeder Seite. 20 e2e-Specs sprechen die Navigation über den Namen «Hauptnavigation» an.
- **Startseite** (`web/src/features/catalog/HomePage.tsx`): genau die acht Reihen aus ADR 0011 in
  der vom Hauptbenutzer gewählten Reihenfolge, leere Reihen entfallen (E-9): `spotlight`, `picks`,
  `continue`, `recent`, `released`, `watchlist`, `collections`, `genres`. Der Server nennt in
  `HomePage.heroRow`, welche Reihe die Bühne ist (Regel 12); der Client entscheidet nichts.
  `homeQuery` hat `staleTime` 30 s (`web/src/features/catalog/queries.ts:10`),
  `refetchOnWindowFocus` ist an (`web/src/app/queryClient.ts:29`), und Player, Verwaltung und
  Merkliste invalidieren `['home']`. **Es gibt keinen Startseiten-Cache im Server**:
  `discover.Home` rechnet bei jedem Aufruf (`server/internal/discover/discover.go:86` bis `:130`),
  `FirstHero` für das Vormalen rechnet getrennt davon (`:218`).
- **Spotlight** (`web/src/ui/HeroCarousel.tsx`, ADR 0011): gerahmtes Band mit Backdrop, Titel,
  «Abspielen · Details · Trailer» (`SpatialButtonLink` mit `href`), darunter eine **Posterleiste in
  zwei Dritteln** der Reihen-Postergrösse (`HeroCarousel.module.css:195`,
  `calc(var(--sc-size-poster-sm) * 2 / 3)`). Das Band beginnt **immer mit Platz 1** der Liste,
  wechselt alle **8 s** (`intervalMs`, `HeroCarousel.tsx:171`) und **hält an**, sobald der
  Mauszeiger darüber steht, ein Element darin Fokus hat (`focusWithin`, `:177`, auch nach einem
  Mausklick), ein Trailer offen ist oder das Betriebssystem «Bewegung reduzieren» meldet (`:180`).
  Pfeil links auf «Abspielen» blättert das Band zurück (`:211` bis `:215`); die Enden der Leiste
  halten den Fokus (`:230` bis `:232`); die Pfeile sind mit `excludeFromTabOrder` aus der
  Tab-Reihenfolge (`:356`). Leistenposter laden erst nach dem Bild des Bands (`stripImages`,
  `:190`). Für den LCP-Wert (P-10) malt der Server das erste Bild als `#hero-prepaint` vor, wenn
  die Bühne die erste Reihe ist (`server/internal/api/preload.go:180`), sonst die ersten sechs
  Poster als `#poster-prepaint` (`:205`, Entscheide 69 und 70); beide mit `sizes="100vw"`
  (`:186`, `:251`). Die Geometrie steht ein zweites Mal in `web/src/ui/global.css:73` und `:104`.
- **Bibliotheken durchsehen.** Es gibt **keine Seite**, die alle Titel einer Bibliothek oder einer
  Inhaltsart zeigt. Am nächsten kommt die Suche im Blättermodus (`/search` ohne Text, mit Filtern:
  Genre, Jahr, Freigabe, Medienart, Verfügbarkeit; `web/src/features/discover/searchParams.ts`),
  sortiert **nur nach Titel**, ohne Bibliotheksfilter, 50 je Seite mit «Mehr laden» (`ItemGrid`,
  ohne Haken und ohne Freigabe-Chip, `web/src/features/discover/ItemGrid.tsx:33`).
- **Sortiertitel.** `items.sort_title` (00003 `:53`) ist der kleingeschriebene Titel **aus dem
  Dateinamen** mit abgestreiftem Artikel (`server/internal/library/names.go:369`); der
  Metadatenabgleich schreibt ihn nie um. Der gezeigte Titel kommt aus `item_texts` in der
  Profilsprache. Eine TMDB-Wertung führt `items` **nicht**; `rating` heisst überall die
  Altersfreigabe.
- **Zuschauerseitige Bibliotheksdaten.** `GET /libraries` (`listAccountLibraries`,
  `x-visibility: account`) liefert Name und Art für die Profileinstellungen, **nicht** für ein
  Profil gefiltert und ohne Titel. `listGenres` ist global und auf 20 Einträge begrenzt
  (`api/openapi.yaml:3432`). `/api` ist im Service Worker `NetworkOnly` (`web/vite.config.ts:155`);
  Navigationen beantwortet der Precache (`:138`).

### 1.2 Was Marco vermisst (29. Oktober 2026, mit Emby-Bildschirmfoto)

1. **Eine Liste aller Filme, Serien, Dokus usw.** wie in Emby, je Bibliothek, auch für gemischte
   Bibliotheken (nach 07: Dokus mit Film- und Serienordner).
2. **Die linke Seite von Emby**: Start, dann «Meine Medien» mit einem Eintrag je Bibliothek,
   **ergänzt um Cat's Empfehlungen**, ohne Symbole. Die Kachelreihe «Meine Medien» **auf** der
   Startseite will er dagegen nicht; die Startseite soll übersichtlich bleiben.
3. **Spotlight bleibt**, aber: die Poster unter dem Band in **derselben Grösse** wie in den anderen
   Reihen, **ordentlich**, bei jedem Öffnen ein **anderer** Titel zuerst, und **alle paar
   Sekunden** der nächste.

### 1.3 Marcos Entscheide vom 29. Oktober 2026

Zwölf Fragen in drei Runden plus zwei Nachträge, Wortlaut in Abschnitt 11:

| Nr. | Entscheid |
|---|---|
| F-1 | **Seitenleiste ab Tablet-Breite, Schublade auf dem Handy.** Die Kopfzeile behält nur Suche, Glocke, Profil |
| F-2 | **Zufälliger Starttitel je Aufruf**, vom Server gewählt; danach der Reihe nach weiter; Angeheftete bleiben in ihrer Ordnung |
| F-3 | **5 Sekunden Takt; Pause nur bei Tastaturfokus oder offenem Trailer**, nicht mehr beim blossen Mauszeiger; keine Automatik bei «Bewegung reduzieren» |
| F-4 | **Gemischte Bibliothek: ein Raster, Chips «Alle · Filme · Serien»**; reine Bibliotheken ohne Chips |
| F-5 | **Band bleibt, die Leiste darunter wird eine normale Posterreihe** mit denselben Karten und derselben Grösse wie «Weiterschauen»; Klick auf ein Poster stellt den Titel ins Band |
| F-6 | **Sortierungen nach dem Vorbild Emby**: Titel, Zuletzt hinzugefügt, Erscheinungsjahr, Bewertung; dazu, was Emby bietet und Slouchcat kennt (Laufzeit, Altersfreigabe, Zuletzt gesehen); mit Buchstabenleiste |
| F-7 | **Keine Reihe je Bibliothek auf der Startseite**; die acht Reihen bleiben (E-9) |
| F-8 | **Filter: Genre, Status (Ungesehen / Angefangen / Gesehen), Jahr und Altersfreigabe.** Verfügbarkeit (Offline) **nicht** |
| F-9 | **Knopf «Pause / Weiter» am Band** (WCAG 2.2.2), oben rechts neben der Positionsanzeige |
| F-10 | **Takt in der Verwaltung einstellbar** (3 bis 15 s, Vorgabe 5) auf der Seite «Spotlight» |
| F-11 | **Seitenleiste ab 768 px** (Tablet, auch hochkant) |
| F-12 | **Hörbücher: Buchstabenleiste nach Titel; «Autor» als zusätzliche Sortierung** nur dort |
| N-1 | Nachtrag: **«Cat's Empfehlungen» als Eintrag unter «Meine Medien»** mit eigener Seite |
| N-2 | Nachtrag: **keine Symbole** in der Seitenleiste. Damit gibt es keinen schmalen Symbolzustand; «Einklappen» heisst **ausblenden**, das Menü-Symbol oben links öffnet dann die Schublade |
| N-3 | Nachtrag: **auf dem Handy drei Cover je Reihe** sichtbar, ganz, nicht angeschnitten |
| N-4 | Nachtrag: **Reihen rasten links an der Kante ein**; ein links angeschnittenes Cover (wie in Marcos Bildschirmfoto) darf nicht vorkommen, auch nicht nach dem automatischen Weiterrücken der Spotlight-Reihe |

---

## 2. Zielbild

### 2.1 Vier Bauteile

1. **Seitenleiste** (ab 768 px fest links, 15 rem, ausblendbar; darunter und bei ausgeblendeter
   Leiste eine Schublade über das Menü-Symbol oben links): Start, Suche, Merkliste, Downloads;
   Gruppe **«Meine Medien»** mit einem Eintrag je Bibliothek, die das aktive Profil sehen darf, mit
   Titelzahl, und als letztem Eintrag **Cat's Empfehlungen**; unten Verwaltung (mit
   Update-Hinweis), Profile, Abmelden nach denselben Regeln wie heute. Die Liste der Bibliotheken
   kommt vom Server (`listVisibleLibraries`, Abschnitt 5), der Client zeigt nur. Keine Symbole.
2. **Startseite ohne Kachelreihe**: Bühne und acht Reihen wie heute, aber das Spotlight-Band nach
   F-2, F-3, F-5, F-9 und F-10: Posterreihe in Normalgrösse mit laufendem Zeitbalken am aktiven
   Poster, 5 s Takt, Einstieg vom Server, Pause bei Tastaturfokus, Trailer oder Knopf.
3. **Bibliotheksseite** `/libraries/{id}`: Überschrift mit Zähler, Werkzeugzeile (Chips für
   gemischte Bibliotheken, Sortierung mit Richtung, Filter), Buchstabenleiste bei Sortierung nach
   Titel (rechts ab 768 px, oben als Lauf auf dem Handy), Posterraster wie in der Suche, «Mehr
   laden». Der Zustand steht in der Adresse (wie `searchParams.ts` für die Suche).
4. **Seite «Cat's Empfehlungen»** `/picks`: alle gepflegten Empfehlungen in Cats Reihenfolge mit
   Notiz, als Raster; die Reihe auf der Startseite zeigt wie heute höchstens 20 (`MaxRowItems`)
   und bekommt «Alle anzeigen ›».

### 2.2 Was sich für den Zuschauer ändert

- Die Kopfzeile verliert die Pillen; die Navigation wandert nach links. Auf dem Handy bleibt oben
  das Menü-Symbol, die Suche, die Glocke und das Profil.
- Jede Bibliothek ist mit einem Klick erreichbar und vollständig durchsehbar; Serien erscheinen als
  Serie (nie als Folge), Hörbücher und Podcasts mit quadratischem Cover (wie heute in den Reihen).
- Der Spotlight-Einstieg wechselt bei jedem Öffnen; das Band läuft, bis jemand es mit der
  Tastatur, dem Knopf oder dem Trailer hält.
- **Keine neue Regel im Client** (Regel 12): Sichtbarkeit, Altersgrenze, Bibliotheksfreigabe,
  die angebotenen Filterwerte und der Starttitel kommen vom Server.

---

## 3. Konflikt mit Spec, Plan und ADRs

| Stelle | Was dort steht | Was dieses Papier daraus macht |
|---|---|---|
| Spec E-9 (`01-anforderungen-v1.md:122`), E-1 (`:114`) | Höchstens acht Reihen, keine leeren; Reihenfolge des Hauptbenutzers | **Unverändert** (F-7). Die Kachelreihe «Meine Medien» aus Emby wird nicht gebaut; die Bibliotheken stehen in der Seitenleiste |
| Spec Gestaltung (`:50`) | Dunkel, kinohaft, grosse Bildflächen, wenig Rahmen, «Übersichtlichkeit hat Vorrang vor Effekten» | Eine Seitenleiste ist ein zweiter Rahmen auf der Seite. Sie ist ausblendbar und unter 768 px unsichtbar; die Bühne behält die Breite des Inhaltsbereichs. Marcos Wahl (F-1, F-11) |
| Spec Non-Goal «Zufallsauswahl» (`:60`) | Keine Entdecken-Hilfe «Zufallsauswahl» | **Nicht berührt**: der zufällige Einstieg (F-2) wählt keinen Titel *für* den Zuschauer, er wählt nur, welcher der ohnehin gezeigten Spotlight-Titel zuerst im Band steht. Keine neue Reihe, keine Empfehlung, keine Sortierung «Zufällig» |
| Spec E-2 (`:115`) | Cat's Empfehlungen von Hand gepflegt, mit Notiz, Reihenfolge, Empfänger, Zeitraum | **Unverändert**; die Seite `/picks` zeigt dieselbe Menge in derselben Reihenfolge ohne die 20er-Klemme der Reihe |
| ADR 0011, «Stelle der Bühne», Plan 05 Abschnitt 7b (Mockup J) | Band mit Posterleiste «als Auswahl», Leiste in zwei Dritteln; 8 s; Pause bei Mauszeiger und bei jedem Fokus | **Geändert** (F-3, F-5, F-9, F-10): Leiste in Normalgrösse, Takt einstellbar mit Vorgabe 5 s, Pause bei Tastaturfokus (`:focus-visible`), Trailer, Knopf und Tab im Hintergrund, **nicht** bei Mauszeiger oder Mausklick. ADR 0016 ist Teilnachfolger von 0011 in diesen Zeilen; Cache, Overrides, `heroRow`, `min_hits`, Trailer-Knopf nur an der Bühne bleiben |
| ADR 0011, `heroRow` | Der Server entscheidet die Bühne | **Erweitert**: der Server entscheidet auch den Einstieg, **einmal je Seitenaufruf** (`heroStart`, Abschnitt 5.2 und 6) |
| Entscheid vom 25. Oktober 2026 (`04-entscheidungen.md:71`) | Trailer-Knopf nur an der Bühne; `PosterCard` kennt keine Knöpfe | **Unverändert in der Sache, präzisiert im Bauteil**: `PosterCard` bekommt eine **Variante ohne `href`** mit `onPress` (ein `<button>`, kein Link, der nicht navigiert), ohne Trailer-Knopf. Die Detailseite erreicht man über «Details» im Band oder über das Poster in jeder anderen Reihe |
| WCAG 2.2.2 (Pause, Stop, Hide) | Bewegtes, das länger als 5 s läuft, braucht einen Weg, es anzuhalten | **Erfüllt durch F-9**: Knopf «Pause / Weiter» am Band |
| Konzept 07, Abschnitt 2.1 | Zuschauer unverändert; Bibliotheksnavigation ist ein eigenes Vorhaben | **Dieses Vorhaben.** 07 muss zuerst in `main` sein |
| Plan P-10, P-11 (`03-plan-v1.md:613`, `:614`) | LCP < 2.5 s, Startseiten-API < 300 ms p95, auch während eines Scans | **Gelten weiter** und werden in Schritt 5 nachgemessen (Abschnitt 8) |
| Regel 8 | Jede neue Seite hat einen Playwright-Tastaturtest | Drei neue Seiten in der Wirkung: Bibliotheksseite, Cat's Empfehlungen, Seitenleiste/Schublade (Abschnitt 7.4) |

---

## 4. Datenmodell

**Migration 00026** (nach Vorlage, nur vorwärts) enthält alles aus 4.1 bis 4.3.

### 4.1 `spotlight_config` bekommt den Takt (F-10)

`rotation_seconds INTEGER NOT NULL DEFAULT 5 CHECK (rotation_seconds BETWEEN 3 AND 15)`. Wer ihn
einstellt, tut es dort, wo Anzahl und Mindesttreffer stehen: Admin-Seite «Spotlight».

### 4.2 Sortierschlüssel je Sprache, Wertung, Indizes

| Gegenstand | Entscheid |
|---|---|
| **Sortierschlüssel** | `items.sort_title` stammt aus dem Dateinamen und passt nicht zum gezeigten Titel («Spirited Away» im Dateinamen, «Chihiros Reise» in `item_texts`). Darum neue Spalte **`item_texts.sort_key`** (je Sprache), gesetzt beim Metadatenabgleich und beim Scan: gezeigter Titel, Artikel der jeweiligen Sprache abgestreift («Der», «Die», «Das», «Ein», «Eine»; «The», «A», «An»), Kleinschreibung, Diakritika gefaltet (Ä→a, É→e, ß→ss). **Buchstabe** = erstes Zeichen des `sort_key`, grossgeschrieben, wenn in `[A-Z]`, sonst `#` (Ziffern, Zeichen, andere Schriften). Einmaliger Backfill in der Migration über die vorhandenen `item_texts`; danach pflegt der Scanner |
| **Wertung** | Neue Spalte **`items.vote_average REAL`** (0 bis 10, NULL = unbekannt). Füllung nach der Regel von 07: ein **Feld** wie Beschreibung oder Backdrop, das in der Anbieterreihenfolge des Ordners aufgefüllt wird (`item_field_sources` bekommt `vote_average`; TMDB liefert es, AniList liefert `averageScore` ÷ 10, `local` nie; **Befund aus Schritt 1:** TVDB v4 liefert keine 0–10-Wertung, nur einen Beliebtheits-`score`, darum bekommt eine Serie mit der Liste `[tvdb]` keine Wertung). Ein Anime-Ordner `[tvdb, tmdb, anilist]` nimmt die erste Quelle, die eine Wertung hat. **Backfill**: ein eigener Job `metadata.vote_backfill` über den Bestand, durch `ratelimit` gedrosselt wie jeder Abgleich; bei 2 500 Filmen und etwa 1 000 Serien (M3-Baum) rund eine Stunde gegen TMDB. Bis dahin stehen Unbekannte am Ende der Sortierung |
| **Serienstatus** | Je Profil: *angefangen* = mindestens eine sichtbare Folge hat Fortschritt und nicht alle sind abgeschlossen; *gesehen* = alle sichtbaren Folgen abgeschlossen; sonst *ungesehen*. **Exklusiv**, eine Serie ist nie zweierlei. Podcasts gleich, über ihre Folgen; Hörbücher und Filme über ihren eigenen Fortschritt. Dieselbe Regel füllt `ItemRef.progress.completed` für Serien in `listLibraryItems` |
| **Indizes** | `(library_id, kind, added_at DESC)`, `(library_id, kind, release_date DESC)`, `(library_id, kind, vote_average DESC)` auf `items`; `(item_id, lang, sort_key)` auf `item_texts`. `items_added` (00003 `:80`) und `items_released` (00021) decken je nur eine Spalte. `runtime_minutes` ohne Index. Welche davon der Planer wirklich nimmt, zeigt `EXPLAIN QUERY PLAN` in Schritt 1 auf dem M3-Baum |

### 4.3 Sichtbare Bibliotheken: eine Regel, eine Stelle

Welche Titel ein Profil sieht, steht in `visible_items` (zuletzt in 00022 `:75` bis `:94`:
`all_libraries`, `account_libraries`, `profile_libraries`, Altersgrenze `min(profil, konto)`).
**Dieses Papier leitet «sichtbare Bibliothek» aus `visible_items` ab, nicht aus einer zweiten
Kopie der Freigaberegel:** eine Bibliothek ist für ein Profil sichtbar, wenn sie **mindestens einen
sichtbaren Titel** hat. Folgen:

- Eine freigegebene, aber leere Bibliothek erscheint **nicht** in der Seitenleiste und ist auf
  `/libraries/{id}` ein **404** wie eine nicht freigegebene (nie 403, Regel 2). Das ist gewollt:
  eine leere Seite hat dem Zuschauer nichts zu sagen; dem Administrator zeigt die Verwaltung die
  Bibliothek weiterhin.
- `itemCount` zählt **nur Titel der obersten Ebene** (`kind IN ('movie','series','audiobook',
  'podcast')`), sonst zählt «Serien» Zehntausende Folgen.
- Die Seitenleiste und die Bibliotheksseite lesen ausschliesslich durch `visibility`; die
  Allowlists in `server/.golangci.yml` bleiben unverändert.

---

## 5. Auswirkung auf den Vertrag

Alles ist **additiv**: keine Operation verschwindet, kein Enum verliert einen Wert. **Vertrag
1.4.0** (nach 1.3.0 aus 07), nicht brechend.

### 5.1 Neue Operationen

| Operation | Pfad | `x-visibility` | Zweck |
|---|---|---|---|
| `listVisibleLibraries` | `GET /libraries/visible` | `scoped` | Die Bibliotheken mit mindestens einem sichtbaren Titel für das aktive Profil, für die Seitenleiste: `id`, `name`, `kind` (`LibraryKind`, die Familie aus 07), `contents` (`RootContent[]`, die Inhaltsarten der Ordner, aus 07), `itemCount`. Reihenfolge wie in der Verwaltung. Kein neues Pfadpräfix: die Ressource hängt an `/libraries`, aber getrennt von `listAccountLibraries` (`account`, für die Profileinstellungen, nennt auch Unsichtbares) |
| `listLibraryItems` | `GET /libraries/{id}/items` | `scoped` | Eine Seite Titel der Bibliothek. Parameter: `content` (`RootContent`, eine Inhaltsart aus `contents`; fehlt = alle; die Chips «Filme / Serien» bilden auf `items.kind` `movie` / `series` ab), `sort` (`title`, `added`, `released`, `vote`, `runtime`, `age`, `played`, `author`; Vorgabe `title`; `author` nur bei `audiobooks`, sonst 422), `order` (`asc`/`desc`; Vorgabe je `sort`: `title`, `age`, `author` aufsteigend, alles andere absteigend), `genre`, `yearFrom`, `yearTo`, `maxRating` (der Server nimmt `min(Profilgrenze, Parameter)`), `status` (`unwatched`, `started`, `watched`, mehrfach), `letter` (ein Zeichen aus `[A-Z#]`; nur mit `sort=title`, sonst 422 `validation_failed`), `offset`, `limit` (1 bis 100, Vorgabe 50). Antwort `LibraryItemsPage`: `items[ItemRef]`, `total`, `offset`, `limit`, **`facets`** (`letters[]`: belegte Anfangsbuchstaben der gefilterten Menge ohne Buchstabenfilter; `genres[]`: Genres mit Treffern in der Bibliothek; `ratings[]`: vorkommende Freigaben **unter** der Profilgrenze). Der Client zeigt nur an, was in `facets` steht (Regel 12). Unsichtbare oder leere Bibliothek: 404 |
| `listPicks` | `GET /picks` | `scoped` | Alle Cat's Empfehlungen für das Profil (E-2: Empfänger, Zeitraum, Sichtbarkeit) in Cats Reihenfolge, `offset`/`limit` (50, höchstens 100), mit `note`; dieselbe Abfrage wie die Reihe, ohne die 20er-Klemme |

### 5.2 Erweiterte Schemata

- **`HomePage.heroStart`** (`integer`, `minimum: 0`, optional): Index in `items` der Bühnenreihe,
  mit dem das Band beginnt; fehlt, wenn `heroRow` fehlt. Gewürfelt wird **einmal je Seitenaufruf**
  (6); der Index liegt über der Reihe **nach** Dedup, Anheftung und Schnitt (ADR 0011,
  «Ordnungsregel»), damit die Ordnung der Angehefteten unberührt bleibt (F-2).
  `getHome` bekommt den optionalen Parameter **`start`** (Index, vom Server auf `[0, len)` geklemmt;
  ausserhalb = neu gewürfelt): mit ihm wiederholt der Server einen früheren Wurf statt neu zu
  würfeln. Der Client **rastet** `heroStart` beim Einhängen des Bands ein und ignoriert spätere
  Werte aus Refetches (`staleTime`, Fensterfokus, Invalidierung), sonst spränge das Band.
- **`HomePage.heroIntervalSeconds`** (optional, nur mit `heroRow`): der Takt aus
  `rotation_seconds`, damit er nicht im Client steht.
- **`SpotlightConfig.rotationSeconds`** (3 bis 15) und **`SpotlightConfigUpdate.rotationSeconds`**
  (optional; **fehlt = unverändert**, denn `SpotlightConfigUpdate` ist sonst ein Vollersatz,
  `api/openapi.yaml:8940`, und ein älterer Admin-Client dürfte den Wert nicht stumm zurücksetzen;
  ein Test schickt ein Update ohne das Feld und prüft, dass 7 s bleiben).
- `LibraryNav`, `LibraryItemsPage`, `LibraryFacets`, `PicksPage` (neu). `ItemRef` bleibt
  unverändert; `progress.completed` wird für Serien in `listLibraryItems` nach 4.2 gefüllt.

### 5.3 Vertragstests

Alle drei Operationen tragen `x-visibility: scoped` und einen Eintrag in
`TestContractCoversEveryOperation` (Regel 7). `listLibraryItems` bekommt den **Negativtest** nach
K-3: ein Kinderprofil sieht in einer freigegebenen Bibliothek keinen Titel über seiner Stufe,
`maxRating: 18` wird auf die Profilgrenze geklemmt, `facets.ratings` enthält nur Stufen darunter,
und eine nicht freigegebene oder leere Bibliothek ist 404. `letter` ohne `sort=title` und `author`
ausserhalb von Hörbüchern sind 422.

---

## 6. Server

- **Ein Wurf je Seitenaufruf (Blocker aus dem Review).** Die App-Hülle `GET /` und das vorgeladene
  `GET /home` sind zwei Anfragen (`preload.go:58`, `:102`); würfelte jede für sich, zeigten
  Vormalen und Band verschiedene Titel, und das Bild des Bands wäre nicht vorgeladen — genau der
  Fall ohne Vorladung (M11: 4.101 s statt 1.652 s). Darum: **die Hülle würfelt** (`FirstHero`
  liefert Index und Bild), schreibt den Index als `data-start` an `#hero-prepaint` **und** in die
  `Link`-Vorladung `</home?start=K>`; der Client liest `data-start` beim ersten Abruf und schickt
  `start=K` mit, womit die Vorladung aus dem Browser-Cache greift. Ohne Hülle (Navigation in der
  PWA, Refetch) fehlt `start`, und der Server würfelt; der Client rastet ohnehin ein (5.2).
- **`discover`** bekommt eine **einsetzbare Zufallsquelle** (`discover.WithRand(*rand.Rand)` als
  Option; Vorgabe `math/rand/v2`, kein Sicherheitsbelang), damit Unit-Tests und die e2e-Fixtures
  einen festen Einstieg erzwingen können (heute nehmen `home.spec.ts:148` und `:253` Folie 0 an).
  Die e2e-Umgebung setzt `SLOUCHCAT_HERO_START=0`.
- **`visibility`** bekommt `VisibleLibraries`, `LibraryItems` (mit Facetten in **einer**
  Transaktion: Seite, `total`, `letters`, `genres`, `ratings`) und `Picks` (ohne Klemme); alle
  lesen `visible_items` (Regel 2, kein neues Paket).
- **`preload.go`**: `#hero-prepaint` malt das Bild des **gewürfelten** Titels; die
  Geometrie-Konstanten bleiben, `sizes` ändert sich (8). **Vormalen und Band laufen bis Schritt 4
  getrennt**: bis dahin bleibt das Vormalen bei Platz 1 und der Client auch (10).
- **`metadata`**: `vote_average` als Feld in der Auffüllreihenfolge (07), Job
  `metadata.vote_backfill`.
- **`admin`**: `rotation_seconds` in `getSpotlightConfig` / `setSpotlightConfig`.
- **Logs** (Regel 10): nichts Neues mit Geheimnissen; `library_id`, `sort`, `start` dürfen ins
  Protokoll.

---

## 7. Oberfläche

### 7.1 Seitenleiste und Schublade

Neues Bauteil `web/src/ui/SideNav.tsx` (ein Bauteil, zwei Darstellungen) mit dem `<nav>`-Namen
**«Hauptnavigation»** (20 Specs sprechen ihn an). Ab **768 px** eine feste Spalte links (15 rem),
darunter eine **Schublade** aus `web/src/ui/Dialog` (react-aria: Fokus beim Öffnen auf dem ersten
Eintrag, Fokus bleibt im Dialog, Seite dahinter scrollt nicht, Escape und Scrim schliessen, Fokus
zurück auf das Menü-Symbol). Inhalt und Reihenfolge, **ohne Symbole** (N-2):

1. Start, Suche, Merkliste, Downloads (Regeln wie heute in `AppLayout.tsx:64` bis `:80`);
2. Gruppe **«Meine Medien»** (ein- und ausklappbar, Zustand je Gerät in `localStorage`, Lesen und
   Schreiben in `try/catch`): je sichtbarer Bibliothek Name und `itemCount`, zuletzt **«Cat's
   Empfehlungen»** mit Zahl (N-1); aktive Seite mit `aria-current="page"`;
3. unten Verwaltung (Administrator, mit dem Update-Hinweis aus `AppLayout.tsx:121`), Profile
   (Hauptprofil), **Abmelden** — die beiden Letzten verlassen die Kopfzeile (F-1 nennt sie nicht;
   O-5 fragt, ob das so bleibt) — und **«Seitenleiste ausblenden»**.

**Ausblenden statt Einklappen (N-2):** ohne Symbole gibt es keinen schmalen Zustand. Der Zuschauer
blendet die Leiste aus (Zustand je Gerät); dann erscheint das Menü-Symbol auch über 768 px und
öffnet die Schublade, in deren Fuss «Seitenleiste einblenden» steht. Das ist die Antwort auf den
Fernseher-Browser (12.5).

`AppHeader` behält Menü-Symbol (unter 768 px oder bei ausgeblendeter Leiste), Suche, Glocke und
Profilwechsel; Pillen, Update-Hinweis und Abmelden ziehen in die Leiste. Die Taste `/` bleibt.

**Offline und Fehler.** Die statischen Einträge (1 und 3) kommen aus der Sitzung und stehen
immer. «Meine Medien» hängt an `listVisibleLibraries`; die Abfrage **blockiert die App-Hülle
nicht** (eigene `useQuery`, kein `beforeLoad`), und offline oder bei Fehler zeigt die Gruppe
«Offline: Bibliotheken nicht verfügbar» mit dem Verweis auf Downloads. `offline-cold-start.spec.ts`
wird darum erweitert. Die Antwort wird mit der Sitzung vorgeladen (`preload.go:58`, 8).

**Räumliche Navigation** (`web/src/ui/spatial.tsx`): die Leiste ist eine eigene Spalte. Regeln:
Pfeil links aus der **ersten Karte einer Posterreihe** (auch der Spotlight-Reihe) und aus dem
ersten Element einer Werkzeugzeile landet auf dem Eintrag der aktuellen Seite in der Leiste; die
Knöpfe im **Band** behalten ihr heutiges Blättern mit links/rechts (`HeroCarousel.tsx:211`),
der Ausstieg aus dem Band nach links ist der erste Posterknopf der Spotlight-Reihe (Pfeil runter,
dann links). Pfeil rechts aus der Leiste führt zum zuletzt fokussierten Element im Inhalt, sonst
zum ersten. Im Raster der Bibliotheksseite führt Pfeil rechts aus der letzten Spalte auf die
Buchstabenleiste und Pfeil links von dort zurück in dieselbe Zeile. Im Admin-Bereich bleibt die
eigene `AdminNav`; die Leiste zeigt dort «Verwaltung» als aktiv.

### 7.2 Startseite und Spotlight

`HeroCarousel` behält Bühne, Pfeile (ausserhalb der Tab-Reihenfolge), Positionsanzeige,
Trailer-Knopf und das Vormalen. Es ändert sich:

| Heute | Neu |
|---|---|
| Leiste in zwei Dritteln von `poster-sm` mit eigenem `.poster`-Knopf | **`PosterCard`-Reihe** (`Row`, `--sc-size-poster-*`) in der neuen **Variante ohne `href`**: `onPress`, `active` (Ring in `--sc-item-accent`, volle Deckung; die übrigen auf 70 %), `aria-current`, Zeitbalken als Kind, `deferImage` wie heute (`stripImages`, Poster erst nach dem Bild des Bands). Enter, Leertaste oder Klick stellt den Titel ins Band und **öffnet nicht** die Detailseite (F-5). Neue Storybook-Vergleichsbilder für `PosterCard` |
| `intervalMs = 8000` fest | `heroIntervalSeconds` aus der Antwort (F-10, Vorgabe 5) |
| Beginn bei Index 0 | Beginn bei `heroStart`, eingerastet je Einhängen (5.2) |
| Pause bei Mauszeiger, jedem Fokus, Trailer, Bewegung reduzieren, einer Folie | Pause bei **Tastaturfokus im Band** (`:focus-visible`, react-aria `isFocusVisible()`; ein Mausklick auf ein Poster oder einen Knopf hält **nicht** an), offenem Trailer, einer Folie, **Knopf «Pause / Weiter»** (F-9, `aria-pressed`, oben rechts neben der Positionsanzeige) und Tab im Hintergrund (`visibilitychange`; nicht Teil von F-3, in ADR 0016 benannt). Bei «Bewegung reduzieren» keine Automatik, Zeitbalken ausgeblendet. **Kein** Halt beim Mauszeiger (F-3) |
| Leiste scrollt zur aktiven Folie | Die Spotlight-Reihe scrollt **nur horizontal** und rückt bei **jedem** Wechsel das aktive Poster **an die linke Kante** (N-4). Damit das auch für die letzten Poster geht, trägt die Spotlight-Reihe am Ende einen **Leerraum** (`aria-hidden`, Breite = Ausschnitt minus eine Karte); die Seite springt nie |
| `Row` mit `scroll-snap-type: x proximity` (`Row.module.css`), Karten `poster-sm` 8.5 rem, auf 390 px zweieinhalb Karten sichtbar | **Alle Reihen** (`Row`): `scroll-snap-type: x mandatory`, `scroll-padding-inline-start: var(--page-gutter)`, `scroll-snap-align: start` je Karte — nach jedem Wischen und jedem Blättern steht links ein ganzes Cover (N-4). **Unter 768 px** ist die Kartenbreite nicht `poster-sm`, sondern `calc((100% - 2 × gap) / 3)` des Reihen-Inhalts: **genau drei Cover** je Reihe, ganz sichtbar (N-3); der Token `--sc-size-poster-sm` bleibt für Suche und Raster. `CARD_SIZES` (`PosterCard.tsx:13`) und `posterSizes` (`preload.go:41`) bekommen den Eintrag `calc(33vw - 1.5rem)` unter 768 px, und `#poster-prepaint` folgt in demselben Commit (8) |
| Bild der nächsten Folie wird nach der aktiven geladen | unverändert; bei 5 s Takt wird zusätzlich die **übernächste** vorgeladen (NAS, 12) |
| «Abspielen» nimmt den Titel beim Klick | **Alle drei Knöpfe** (`SpatialButtonLink` mit `href`) merken sich den Titel beim `pointerdown`; der `click` ruft `preventDefault` und navigiert zum gemerkten `href`, damit ein Wechsel zwischen Drücken und Loslassen keinen fremden Titel öffnet (12.1) |

Die Kachelreihe «Meine Medien» aus Emby wird **nicht** gebaut; die acht Reihen bleiben (F-7).
**«Alle anzeigen ›»** steht rechts im Kopf nur bei Reihen mit Ziel: Cat's Empfehlungen → `/picks`
(N-1), Merkliste → `/watchlist`. Für «Neu hinzugefügt», «Kürzlich veröffentlicht» und die
übrigen bleibt es bei O-3.

### 7.3 Bibliotheksseite und Seite «Cat's Empfehlungen»

Route `/libraries/$libraryId` (`web/src/features/catalog/LibraryPage.tsx`, lazy, unter `_chrome`),
Zustand in der Adresse (`content`, `sort`, `order`, `letter`, `genre`, `status`, `from`, `to`,
`rating`) mit `validateSearch` nach dem Muster von `searchParams.ts`. Aufbau von oben nach unten:

1. **Kopf**: Name der Bibliothek, Zähler mit Pluralform («1 Titel», «312 Titel», «41 Titel mit
   B · gefiltert»; bei Audio «Hörbücher» / «Podcasts»).
2. **Werkzeugzeile**: Chips «Alle · Filme · Serien» nur bei `contents.length > 1` (F-4); rechts
   «Sortieren» (`<select>`; die Ordnungen aus 5.1, «Autor» nur bei Hörbüchern, «Altersfreigabe»
   nur bei Video), Richtungsknopf mit Text («↑ aufsteigend» / «↓ absteigend», Vorgabe je
   Ordnung), «Filter» mit Zähler der aktiven Filter.
3. **Filterfeld** (ausklappbar, `aria-expanded`, Escape schliesst): Genre-Chips aus
   `facets.genres` (nur Genres mit Treffern in dieser Bibliothek; `listGenres` mit seiner
   20er-Grenze wird nicht benutzt), Status-Chips (bei Audio «Ungehört / Angefangen / Gehört»),
   Jahr von–bis, Freigabe-Chips «Profilgrenze (16) · bis 0 · bis 6 · …» aus `facets.ratings`.
   Eine Gruppe ohne Facettenwerte wird **nicht gezeigt** (so entfallen Genre und Freigabe bei
   Audio von selbst, ohne Abweichung von F-8 im Client). «Anwenden» schliesst das Feld und setzt
   den Fokus auf «Filter».
4. **Raster** (`ItemGrid`; `PosterCard` **erweitert** um Haken bei `progress.completed`, Fortschritt
   und Freigabe-Chip aus `rating` — ItemGrid zeigt heute keines davon, `ItemGrid.tsx:33`;
   quadratische Cover bei Audio; **unter 768 px drei Spalten** wie die Reihen, N-3) und daneben die **Buchstabenleiste** (nur bei `sort=title`;
   bei Video und Hörbüchern nach Titel, F-12; nicht bei Podcasts; rechts, `position: sticky`;
   unter 768 px als horizontaler Lauf über dem Raster). Buchstaben ohne Eintrag in `facets.letters`
   sind `disabled`; «•» hebt die Wahl auf, «#» trägt das Label «Ziffer oder Zeichen». Die Leiste
   **filtert** (`letter`), sie springt nicht im Raster.
5. **«Mehr laden (142 weitere)»** wie in der Suche (`useInfiniteQuery`, 50 je Seite). Keine
   Virtualisierung; nach «Mehr laden» liegt der Fokus auf der **ersten neuen Karte**, die
   Scrollposition bleibt. Alle anderen Bedienungen (Chip, Buchstabe, Sortierung, Richtung, Filter)
   ändern nur die Adresse und das Raster; der Fokus bleibt auf dem bedienten Element.

Leere Zustände: Filter ohne Treffer («Nichts passt zu diesen Filtern» mit «Zurücksetzen»). Eine
Bibliothek ohne sichtbare Titel ist 404 (4.3) und zeigt die bestehende Seite «Nicht gefunden».

**Seite «Cat's Empfehlungen»** `/picks` (`PicksPage.tsx`): Kopf mit Zähler, eine Zeile Erklärung,
Raster aus `PosterCard` mit Notiz als zweizeiliger Unterschrift (wie in der Reihe), «Mehr laden» ab
50. Keine Sortierung, keine Filter: die Reihenfolge ist Cats (E-2).

### 7.4 Regel 8 — Tastaturtests

Neu: `web/e2e/library.spec.ts` (Tab-Reihenfolge Kopf → Chips → Sortieren → Richtung → Filter →
Leiste → Raster; Pfeiltasten im Raster und hinüber zur Buchstabenleiste; Enter öffnet die
Detailseite; Zurück landet auf derselben Karte mit derselben Adresse; sichtbarer Fokus; Escape im
Filterfeld; Fokus nach «Mehr laden» auf der ersten neuen Karte), `web/e2e/picks-page.spec.ts`
(Raster, Zurück), `web/e2e/sidenav.spec.ts` (Leiste bei 1280 px: Pfeil hoch/runter, Enter, Pfeil
rechts in den Inhalt und links zurück; ausblenden und einblenden; Schublade bei 390 px: Öffnen,
Fokus im Dialog, kein Scrollen dahinter, Escape, Fokusrückgabe; Offline-Zustand der Gruppe), und
`web/e2e/home.spec.ts` wächst um: Band beginnt bei `SLOUCHCAT_HERO_START`, Enter auf einem Poster
der Spotlight-Reihe wechselt das Band ohne Navigation, Tab ins Band hält den Takt an, ein Mausklick
auf ein Poster nicht, Fensterwechsel ändert den Einstieg nicht, Knopf «Pause» hält und «Weiter»
löst. `admin-spotlight.spec.ts` bekommt das Feld «Takt». Bestehende Specs: die 20 mit
«Hauptnavigation» laufen unverändert, die Tests bei 390 px prüfen die Schublade statt der Pillen,
`storybook:test` bekommt neue Vergleichsbilder für `AppHeader`, `PosterCard` und die Admin-Seiten
(15 rem schmaler ab 768 px) — **schon in Schritt 3**, nicht erst in Schritt 4.

**Regel 9 — Texte und Farben**: neue Schlüssel `nav.libraries` («Meine Medien»), `nav.menu`,
`nav.menuClose`, `nav.hideSidebar`, `nav.showSidebar`, `nav.toggleLibraries`,
`nav.librariesOffline`, `library.page.{count_one, count_other, countLetter, filtered, sort,
direction, ascending, descending, filter, apply, clear, emptyFilters, more, noun.video,
noun.audiobooks, noun.podcasts}`, `library.sort.{title, added, released, vote, runtime, age,
played, author}`, `library.status.{unwatched, started, watched, unheard, heard}`,
`library.content.{all, movie, series}`, `library.letters.{all, other}`, `home.rows.all` («Alle
anzeigen»), `home.spotlight.{pause, resume, row}`, `picks.page.{title, intro}`,
`admin.spotlight.rotation`, Fehlercode `letter_requires_title_sort`; `pnpm -C web test locales`
prüft die Symmetrie. Farben nur aus Tokens; der Zeitbalken nimmt `--sc-color-text-primary`, der
Ring `--sc-item-accent`.

### 7.5 Klickbarer Prototyp

`docs/planning/mockups/08-startseite-prototyp.html` ist eine einzelne HTML-Datei ohne
Abhängigkeiten (Doppelklick genügt). Sie zeigt die Seitenleiste ohne Symbole mit Marcos sieben
Bibliotheken (Filme, Serien, Dokus, Kids, Anime, Hörbücher, Podcasts; Dokus, Kids und Anime
gemischt) und «Cat's Empfehlungen», «Seitenleiste ausblenden», die Startseite mit dem
Spotlight-Band (Pause-Knopf, Positionsanzeige, Pfeiltasten auf den Bandknöpfen ohne Fokusverlust)
und der Posterreihe in Normalgrösse (5 s Takt, zufälliger Einstieg je Aufruf, Zeitbalken, Pause
nur bei `:focus-visible`), die acht Reihen, die Bibliotheksseite (Chips, Sortierung mit Vorgabe
der Richtung, Richtungsknopf mit Text, Filterfeld aus Facetten, Buchstabenleiste nach
Sortierschlüssel, Zähler, «Mehr laden» mit Fokus auf der ersten neuen Karte, keine Sprünge) und
die Seite «Cat's Empfehlungen». Der Knopf **«Handy-Ansicht»** zeigt dieselbe Seite in einem
400-px-Rahmen mit modaler Schublade (`<dialog>`). Poster sind Farbverläufe mit Titel; Farben und
Abstände sind die Tokens aus `packages/design-tokens/tokens/`. Der Prototyp ist **kein** Code für
`web/src` und wird nicht gebaut oder getestet. Dieselbe Fassung ist als privates Artefakt unter
`https://claude.ai/artifact/JdbQx8uJekoVetmrpf4nU2` erreichbar.

---

## 8. Leistung

- **P-10 (LCP < 2.5 s)**: Die Bühne bleibt das grösste Element im ersten Bild; die Seitenleiste
  ist Text. Die Breite des Inhaltsbereichs sinkt ab 768 px um 15 rem, und das betrifft **beide**
  Vormalungen (`#hero-prepaint` und `#poster-prepaint`, `global.css:73`, `:104`) sowie das
  `sizes`-Attribut der vorgeladenen Bilder (`preload.go:186`, `:251`; die `780w`/`1280w`-Deskriptoren
  bleiben). Lösung: eine CSS-Variable `--sidenav-width` (0 unter 768 px, 15 rem darüber, 0 bei
  ausgeblendeter Leiste), die beide Vormalungen und das Band benutzen; ein **Inline-Skript in
  `index.html`** liest den Ausblendzustand aus `localStorage` und setzt `data-sidenav` vor dem
  ersten Malen, damit die Hülle dieselbe Geometrie zeichnet wie die App. Ein Playwright-Test bei
  1280 px vergleicht die Box der Vormalung mit der des Bands und der ersten Karte (eingeblendet und
  ausgeblendet, mit und ohne Bühne). **Lighthouse mobile** (390 px) sieht die Leiste nicht; darum
  zusätzlich **Lighthouse desktop** im DoD von Schritt 3 und 5.
- **P-11 (Startseiten-API < 300 ms p95, auch während eines Scans)**: `heroStart` ist ein Zufallszug
  ohne Abfrage. `listVisibleLibraries` ist ein zweiter Aufruf beim Start (mit der Sitzung vorgeladen),
  eine Abfrage über `visible_items` mit `COUNT` je Bibliothek. Messung mit
  `docs/perf/measure-home.sh` (M11) auf dem **M3-Baum** (etwa 2 500 Filme, 40 334 Folgen,
  `task testdata:tree`, `docs/perf/M3.md` Abschnitt 1), **während `scanbench` läuft**. Bleibt die
  Abfrage über 20 ms p95, kommt `itemCount` aus einer kleinen Zähltabelle je Bibliothek, die der
  Scanner pflegt (Ausstiegsklausel).
- **Bibliotheksseite**: Ziel **< 300 ms p95** je Seite (dieselbe Latte wie P-11, weil es dieselbe
  Nutzung ist), gemessen mit einem Schwesterskript von `measure-home.sh` auf dem M3-Baum, Fall
  «Serien, Status = angefangen, Sortierung Titel, Buchstabe S» als schwerster (der `EXISTS` über
  Folgen trifft die grosse Bibliothek). Über 300 ms: Serienstatus in einer Hilfstabelle je Profil,
  die der Player mitführt (Ausstiegsklausel). Ergebnisse nach `docs/perf/startseite.md`.

---

## 9. Sicherheit

- **Nichts Neues im Client.** Alle drei neuen Operationen sind `scoped`; die Altersgrenze und die
  Bibliotheksfreigabe sitzen in `visible_items` (Regel 2 und 12). `maxRating` kann die Grenze nur
  **senken**: der Server nimmt `min(Grenze, Parameter)`; die angebotenen Chips kommen aus
  `facets.ratings`, der Client kennt die Kontogrenze nicht und muss sie nicht kennen.
- **404 statt 403** für eine Bibliothek, die das Profil nicht sieht oder die leer ist; die
  Seitenleiste nennt sie nicht, die Adresse verrät sie nicht.
- **Keine neue Aussenverbindung, kein Geheimnis.** `rotation_seconds`, `start` und `letter` sind
  Zahlen oder ein Zeichen; `letter` wird auf `[A-Z#]` geprüft, `start` auf `[0, len)` geklemmt,
  alles über vorbereitete sqlc-Abfragen.

---

## 10. Schnitt in Schritte

Vorbedingung: **Konzept 07 ist in `main`** (Familie, `contents` je Ordner, Auffüllreihenfolge).
Zweig `claude/startseite`, Basis `main` nach dem Merge von PR #45. Jeder Schritt ein Commit,
`task ci` grün, DoD mit Befehl und Ausgabe in `docs/dev/CURRENT_STATUS.md`. Die Reihenfolge folgt
der Baubarkeit: erst Server und Seiten, dann die Leiste, die auf die Seiten verweist, zuletzt das
Band, das Vormalen und Client zugleich umstellt.

### Schritt 0 — Entscheide und Papiere
Frage 14 ins Entscheidungsprotokoll (F-1 bis F-12, N-1 bis N-4 und die Antworten auf O-1 bis O-5),
**ADR 0016** «Seitenleiste, Bibliotheksseite und Spotlight-Takt» (Teilnachfolger von ADR 0011;
benennt die Pause bei `visibilitychange` und die Ableitung «sichtbare Bibliothek» aus
`visible_items`), Status dieses Papiers auf «freigegeben, in Umsetzung», Nummern nach dem Merge von
07 (Migration 00026). Reiner Doku-Commit.

### Schritt 1 — Vertrag 1.4.0, Migration 00026 und Server
`api/openapi.yaml` (drei Operationen, `start`, drei Felder, vier Schemata), `task gen`, Migration
00026 (`rotation_seconds`, `item_texts.sort_key` mit Backfill, `items.vote_average`, Indizes),
Abfragen in `server/queries/scoped/` mit `visible_items`, `visibility.VisibleLibraries`,
`LibraryItems` mit Facetten, `Picks`, `discover.WithRand` und `heroStart` **nur in der
Antwort** (das Vormalen bleibt bei Platz 1 bis Schritt 4), `metadata` mit `vote_average` in der
Auffüllreihenfolge und Job `metadata.vote_backfill`, Admin-Konfiguration mit «fehlt =
unverändert». Vertragstests, Negativtest K-3, `TestFrozenStatesMigrate` über alle eingefrorenen
Stände (zuletzt `testdata/db/benutzerverwaltung.sql`, nach 07 dessen Stand). **DoD:** `task ci`
grün; `EXPLAIN QUERY PLAN` der vier Sortierungen; Messung nach Abschnitt 8 auf dem M3-Baum mit
Belegen; `curl` mit einem Kinderprofil zeigt Klemme, Facetten und 404.

### Schritt 2 — Bibliotheksseite und Seite «Cat's Empfehlungen»
Routen, `LibraryPage`, `PicksPage`, Adresszustand, Filterfeld aus Facetten, Buchstabenleiste,
`PosterCard` mit Haken und Freigabe-Chip (plus Storybook-Bilder), `library.spec.ts`,
`picks-page.spec.ts`, Texte. Die Seiten sind erreichbar über die Adresse und über «Alle anzeigen ›»
an der Reihe «Cat's Empfehlungen». **DoD:** `task ci` grün; Playwright-Belege für Tastatur,
Zurück und Fokus nach «Mehr laden».

### Schritt 3 — Seitenleiste, Schublade, Vormalen, Offline
`SideNav`, `AppLayout`, `AppHeader` ohne Pillen, Abmelden und Update in der Leiste, räumliche
Navigation, `--sidenav-width` mit Inline-Skript in beiden Vormalungen und in `sizes`,
Offline-Zustand der Gruppe, `sidenav.spec.ts`, Geometrie-Test bei 1280 px, Storybook-Bilder für
`AppHeader` und Admin-Seiten, `offline-cold-start.spec.ts` erweitert, Texte. **DoD:** `task ci`
grün; Lighthouse mobile **und** desktop auf der Startseite nicht schlechter als `docs/perf/M11.md`.

### Schritt 4 — Spotlight nach F-2, F-3, F-5, F-9, F-10
`Row` mit hartem Einrasten links und drei Karten unter 768 px (N-3, N-4; `CARD_SIZES`,
`posterSizes` und `#poster-prepaint` in demselben Commit), `HeroCarousel` mit `PosterCard`-Reihe
(Variante ohne `href`), `heroStart` eingerastet,
`start` aus `data-start` beim ersten Abruf, **Vormalen auf den gewürfelten Titel** (`FirstHero`
mit Index, `Link`-Vorladung `/home?start=K`), `heroIntervalSeconds`, neue Pausenregel mit
`isFocusVisible()`, Pause-Knopf, `pointerdown`-Merker an den drei Knöpfen, Vorladen der
übernächsten Folie, horizontales Nachführen nur ausserhalb des Ausschnitts, Admin-Feld «Takt»,
`home.spec.ts` und `admin-spotlight.spec.ts` erweitert, `SLOUCHCAT_HERO_START` in den
e2e-Fixtures, Storybook-Bilder für `HeroCarousel`. **DoD:** `task ci` grün; Belege für
Fokus-Pause, Mausklick ohne Pause, Fensterwechsel ohne Sprung, Vormalen = Starttitel (Playwright
liest `data-start` und den Titel im Band).

### Schritt 5 — Abnahme, Messung, Doku
Lighthouse mobile und desktop (P-10), `measure-home.sh` und das Schwesterskript (P-11 und
Bibliotheksseite, während `scanbench`), Gerätecheck am Handy und am Tablet (Schublade, Leiste
hochkant, Lauf der Buchstabenleiste, Band auf 390 px), `docs/perf/startseite.md`,
`docs/dev/CURRENT_STATUS.md`, Release-Notes-Zeile (nicht brechend, 1.4.0; Backfill-Job läuft nach
dem Update einmal), eingefrorener Datenbankstand `testdata/db/startseite.sql` mit Test. Pull
Request, Merge auf Marcos Wort.

---

## 11. Fragen an Marco

### 11.1 Beantwortet (29. Oktober 2026)

| Nr. | Frage | Antwort |
|---|---|---|
| F-1 | Navigation auf Handy und Fernseher-Browser | **Seitenleiste ab Tablet, Schublade auf dem Handy**; Kopfzeile nur Suche, Glocke, Profil |
| F-2 | Spotlight: womit beginnt das Band? | **Zufälliger Start je Aufruf**, vom Server; Angeheftete bleiben in Ordnung |
| F-3 | Takt und Mauszeiger | **5 Sekunden, Pause nur bei Fokus** (und Trailer); keine Automatik bei «Bewegung reduzieren» |
| F-4 | Gemischte Bibliothek | **Ein Raster mit Chips «Alle · Filme · Serien»** |
| F-5 | Form des Spotlight-Bands | **Band bleibt, Posterreihe in Normalgrösse** darunter; Klick stellt den Titel ins Band |
| F-6 | Sortierungen | «Nimm dir ein Beispiel an Emby»; gewählt: **Titel A–Z, Zuletzt hinzugefügt, Erscheinungsjahr, Bewertung**. Dieses Papier nimmt aus Emby zusätzlich Laufzeit, Altersfreigabe und Zuletzt gesehen, lässt Kritikerbewertung, Abspielzähler und Zufall weg (Slouchcat kennt sie nicht, Zufall ist Non-Goal) |
| F-7 | Reihen je Bibliothek auf der Startseite | **Nein, acht Reihen bleiben** |
| F-8 | Filter | **Genre, Status, Jahr und Altersfreigabe**; Verfügbarkeit nicht |
| F-9 | Knopf «Pause / Weiter» am Band (WCAG 2.2.2) | **Ja, kleiner Knopf** |
| F-10 | Takt fest oder einstellbar | **Einstellbar in der Verwaltung** (3 bis 15 s) |
| F-11 | Breite für die feste Seitenleiste | **Ab 768 px** (Tablet, auch hochkant) |
| F-12 | Hörbücher: Buchstabenleiste | **Nach Titel; «Autor» als zusätzliche Sortierung** |
| N-1 | Nachtrag | **«Meine Medien» um Cat's Empfehlungen ergänzen** |
| N-2 | Nachtrag | **Keine Symbole** in der Navigation |
| N-3 | Nachtrag | **Drei Cover je Reihe auf dem Handy** |
| N-4 | Nachtrag | **Reihen rasten links ein**, kein angeschnittenes Cover am linken Rand |

### 11.2 Ehemals offen, mit der Freigabe entschieden (29. Oktober 2026)

| Nr. | Frage | Entscheid («wie vorgeschlagen») |
|---|---|---|
| O-1 | Soll die Seitenleiste je Bibliothek die **Titelzahl** zeigen (wie im Prototyp)? | Ja, mit der Ausstiegsklausel aus Abschnitt 8 |
| O-3 | «Alle anzeigen ›» an weiteren Reihen («Neu hinzugefügt», «Kürzlich veröffentlicht») mit je einer bibliotheksübergreifenden Seite? | Nein; nur Cat's Empfehlungen und Merkliste haben eine Seite |
| O-4 | Soll die Seitenleiste eine Zeile **«Favoriten»** bekommen (`listFavorites` existiert, aber kein Navigationseintrag)? | Nein in diesem Papier |
| O-5 | **Abmelden** und der **Update-Hinweis** ziehen in den Fuss der Seitenleiste (F-1 nennt sie nicht). Einverstanden? | Ja; die Kopfzeile bleibt dreiteilig |

O-2 ist durch F-12 beantwortet.

---

## 12. Risiken

### 12.1 Das Band läuft unter dem Mauszeiger weiter
Mit F-3 wechselt der Titel, während jemand auf «Abspielen» zielt. Milderung: alle drei Knöpfe
merken sich den Titel beim `pointerdown` (7.2), und der Pause-Knopf (F-9) hält das Band für
jeden an. Der Prototyp läuft bereits ohne Mauszeiger-Pause; Marco prüft dort, ob es stört.

### 12.2 Das Vormalen und die Seitenleiste
Zwei Stellen beschreiben dieselbe Geometrie (`global.css`, `preload.go`), und der Ausblendzustand
liegt im Browser. `--sidenav-width` plus Inline-Skript und der Geometrie-Test bei 1280 px (8) sind
das Gegenmittel; Lighthouse desktop im DoD. Der Service Worker beantwortet Navigationen aus dem
Precache, darum läuft die Vormalung ohnehin fast nur beim ersten Besuch.

### 12.3 Zähler und Facetten sind Aggregationen
`total`, `letters`, `genres`, `ratings` und `itemCount` laufen über `visible_items`; der
Serienstatus über Folgen. Auf dem M3-Baum mit 40 334 Folgen entscheidet die Messung in Schritt 1;
die Ausstiegsklauseln stehen in 8.

### 12.4 Abhängigkeit von 07
Ohne Familie, `contents` und Auffüllreihenfolge gibt es weder die gemischte Bibliothek noch die
Wertung. Die Nummern stehen seit dem Merge von PR #45 fest: 00026, 0016, 1.4.0.

### 12.5 Fernseher-Browser
Eine Seitenleiste nimmt auf einem 1080p-Fernseher Platz, den die Bühne gut brauchte. Sie ist
ausblendbar, der Zustand hält je Gerät, und die Schublade bleibt erreichbar. TV-Apps sind Non-Goal.

### 12.6 Sortierschlüssel und Wertung sind neue Daten
`item_texts.sort_key` und `items.vote_average` entstehen durch Backfill: der eine in der Migration
(nur aus vorhandenen Texten, Sekunden), der andere durch einen Job gegen TMDB (Stunde). Bis der Job
durch ist, sortiert «Bewertung» unvollständig; die Seite sagt das nicht, der Zähler auch nicht.
Release-Notes nennen es.

### 12.7 Was dieses Papier nicht zählen konnte
Wie `spatial.tsx` eine zweite Spalte verträgt und ob die react-aria-Schublade mit der räumlichen
Navigation zusammenspielt, wurde nicht geprüft; Schritt 3 trägt das Risiko. Alle Zeilenangaben
gelten gegen `9923807`, nicht gegen den Stand nach 07.

---

## 13. Review-Protokoll (29. Oktober 2026)

Der OMC-Kritiker hat die erste Fassung geprüft: Urteil ITERATE, 1 Blocker, 18 Major, 14 Minor.
Fassung 2 hat alle eingearbeitet:

| Befund | Wo behoben |
|---|---|
| Blocker: Hülle und `/home` würfeln getrennt, Vormalen ≠ Band, LCP fällt; kein Startseiten-Cache | 1.1, 5.2 (`start`), 6 (ein Wurf je Seitenaufruf), 10 (Vormalen erst in Schritt 4) |
| `heroStart` springt bei jedem Refetch | 5.2 (Einrasten je Einhängen), 7.4 (Test Fensterwechsel) |
| Pause bei jedem Fokus, auch nach Mausklick | 7.2 (`:focus-visible`), 7.4 |
| WCAG 2.2.2 ohne Halt für Maus und Finger | F-9, 3, 7.2 |
| `PosterCard` ist ein Link mit Pflicht-`href`; `ItemGrid` zeigt weder Haken noch Freigabe; Serienstatus ohne Daten | 3, 7.2 (Variante ohne `href`), 7.3, 4.2 (Status füllt `progress.completed`) |
| `#poster-prepaint` vergessen; falsches Ziel `preload.go:30`; Ausblendzustand im Browser; Lighthouse mobile sieht die Leiste nicht | 8, 12.2 |
| `sort_title` aus dem Dateinamen; Umlaute, Ziffern | 4.2 (`item_texts.sort_key`) |
| «Sichtbare Bibliothek» ohne gewählte Regel; `itemCount` zählt Folgen | 4.3 |
| Messung an 1 000 Titeln; Zahlen ohne Quelle; `k6` falsch | 8 (M3-Baum, `measure-home.sh`, Scan) |
| `vote_average` ohne Füllregel, Backfill, Namenskollision mit `rating` | 4.2, 5.1 (`vote`), 10 |
| Takt 3–15 s nicht gefragt; Vollersatz in `SpotlightConfigUpdate` | F-10, 5.2 («fehlt = unverändert») |
| 1024 px statt Tablet-Grenze | F-11 (768 px) |
| Schrittschnitt: Vormalen vor dem Client, Folie 0 in Tests, Leiste verweist auf fehlende Route | 6 (`WithRand`), 10 (neue Reihenfolge) |
| Offline und Service Worker | 7.1 |
| Räumliche Navigation widerspricht dem Band; Raster ↔ Leiste; Autoscroll | 7.1, 7.2 |
| Genres global und auf 20 begrenzt; Freigabe-Chips als Regel im Client | 5.1 (Facetten), 7.3 |
| Prototyp: Fokusverlust im Band, Seitensprung und Fokusverlust in der Bibliothek, Schublade ohne Fokusfang | Prototyp Fassung 2 (7.5) |
| Minor: Spec-Zeile, eingefrorener Stand, Pfade und Namen im Vertrag, «Alle anzeigen» vor O-3, Audio-Filter, `visibilitychange`, Abmelden und Update, i18n, Statusdefinition, `pointerdown` für drei Knöpfe, `itemCount`-Klausel, Test-Folgen, `deferImage`, Prototyp-Kleinigkeiten | 3, 10, 5.1, 7.2, 7.3, 7.4, 4.2, O-5, Prototyp |
