# ADR 0016: Seitenleiste, Bibliotheksseite und Spotlight-Takt

- Status: angenommen (29. Oktober 2026); **ersetzt teilweise ADR 0011** (Abschnitt «Was von ADR 0011
  stehen bleibt»)
- Quelle: Konzept `docs/planning/08-startseite.md` (Fassung 2, 29. Oktober 2026, Abschnitte 3 bis 8);
  **Frage 14** in `docs/planning/04-entscheidungen.md` mit Marcos Antworten F-1 bis F-12, N-1 bis N-4
  und O-1, O-3, O-4, O-5; ADR 0011 (Spotlight), ADR 0015 (Familie je Bibliothek, `contents` je
  Ordner, Auffüllreihenfolge)
- Betroffene Spec-IDs: E-1, E-2, E-9, C-4, C-5, K-3, P-10, P-11
- Harte Regeln: 1 (generierter Code nur durch Codegen), 2 (Sichtbarkeit nur über `visibility`,
  404 nie 403), 5 (Migrationen nur vorwärts), 7 (`x-visibility` je Operation und Eintrag im
  Vertragstest), 8 (Tastaturtest je neue Seite), 9 (keine harten Farben und Texte), 10 (keine
  Secrets in Logs), 11 (keine Nicht-Ziele), 12 (Entscheid im Server)

## Kontext

Die Zuschauerseite hat bis Vertrag 1.3.0 keine Seite, die alle Titel einer Bibliothek zeigt, und keine
Navigation zu den Bibliotheken: die Kopfzeile trägt horizontale Pillen (Start, Suche, Merkliste,
Downloads, Verwaltung, Profile), die Suche im Blättermodus sortiert nur nach Titel und kennt keinen
Bibliotheksfilter. Das Spotlight-Band (ADR 0011) beginnt immer mit Platz 1 der Liste, wechselt alle
8 s, hält beim Mauszeiger und bei jedem Fokus an und zeigt darunter eine Posterleiste in zwei
Dritteln der Reihen-Postergrösse. Marco wünscht (29. Oktober 2026, mit Emby-Bildschirmfoto) eine Liste
aller Filme, Serien, Dokus je Bibliothek, die linke Navigation von Emby mit «Meine Medien» und
«Cat's Empfehlungen», ohne Symbole und ohne die Kachelreihe auf der Startseite, und ein Band, das
bei jedem Öffnen mit einem anderen Titel beginnt, alle paar Sekunden weiterläuft und darunter Poster
in Normalgrösse zeigt.

Vier Dinge waren zu entscheiden, die weder Spec noch Plan noch eine ADR festlegen:

1. **Woher weiss der Server, womit das Band beginnt**, wenn die App-Hülle `GET /` (Vormalen und
   Vorladung des Bühnenbilds, P-10) und das vorgeladene `GET /home` zwei Anfragen sind?
2. **Wonach sortiert «Titel»**, wenn `items.sort_title` aus dem Dateinamen stammt und nicht zum
   gezeigten Titel in der Profilsprache passt («Spirited Away» im Dateinamen, «Chihiros Reise» in
   `item_texts`)?
3. **Was ist eine «sichtbare Bibliothek»** für die Seitenleiste und die Bibliotheksseite, ohne die
   Freigaberegel aus `visible_items` ein zweites Mal zu schreiben?
4. **Woher kommt eine Wertung** für die Sortierung «Bewertung», wenn `items` keine TMDB-Wertung
   führt und `rating` überall die Altersfreigabe heisst?

Verworfen: (a) **Würfeln im Client** – der Client entschiede etwas über den Inhalt der Startseite
(Regel 12), und das Vormalen der Hülle zeigte einen anderen Titel als das Band. (b) **Ein eigener
Startseiten-Cache im Server**, damit Hülle und `/home` denselben Wurf lesen – es gibt keinen solchen
Cache, und einer nur für den Index wäre ein zweiter Zustand je Profil, der bei jedem Refetch
mitläuft. (c) **`items.sort_title` umschreiben** beim Metadatenabgleich – der Schlüssel ist je
Sprache verschieden, eine Spalte am Titel kann nur eine Sprache tragen. (d) **«Sichtbare Bibliothek»
aus den Freigabetabellen** (`all_libraries`, `account_libraries`, `profile_libraries`) ableiten – eine
zweite Kopie der Regel, die bei jeder Änderung an `visible_items` nachgezogen werden müsste, und eine
freigegebene, aber leere Bibliothek zeigte eine leere Seite. (e) **Eine Kachelreihe «Meine Medien»
auf der Startseite** wie in Emby – Marco will sie nicht (F-7); die acht Reihen bleiben (E-9).
(f) **Eine Seitenleiste mit Symbolzustand** – ohne Symbole (N-2) gibt es keinen schmalen Zustand;
«Einklappen» heisst ausblenden.

## Entscheid

| Gegenstand | Entscheid |
|---|---|
| **Ein Spotlight-Wurf je Seitenaufruf** (F-2) | Der Server wählt den Starttitel des Bands: `HomePage.heroStart` ist der Index in `items` der Bühnenreihe **nach** Dedup, Anheftung und Schnitt (ADR 0011, Ordnungsregel), damit die Ordnung der Angehefteten unberührt bleibt. **Die App-Hülle würfelt**: `GET /` ruft `FirstHero`, das Index und Bild liefert, schreibt den Index als `data-start` an `#hero-prepaint` und in die `Link`-Vorladung `</home?start=K>`. Der Client liest `data-start` beim ersten Abruf und schickt `start=K` mit; `getHome` wiederholt mit `start` den Wurf statt neu zu würfeln (geklemmt auf `[0, len)`, ausserhalb = neu gewürfelt). Ohne Hülle (Navigation in der PWA, Refetch) fehlt `start`, der Server würfelt, und der Client **rastet** `heroStart` beim Einhängen des Bands ein und ignoriert spätere Werte aus Refetches. Die Zufallsquelle ist einsetzbar (`discover.WithRand`), die e2e-Umgebung setzt `SLOUCHCAT_HERO_START=0` |
| **Takt und Pause** (F-3, F-9, F-10) | `spotlight_config.rotation_seconds` (3 bis 15, Vorgabe 5), in der Verwaltung auf der Seite «Spotlight» einstellbar; `HomePage.heroIntervalSeconds` trägt ihn zum Client. `SpotlightConfigUpdate.rotationSeconds` ist optional, **fehlt = unverändert**: ein älterer Admin-Client darf den Wert nicht stumm zurücksetzen. Das Band hält an bei **Tastaturfokus im Band** (`:focus-visible`, react-aria `isFocusVisible()`), offenem Trailer, einer einzigen Folie, dem Knopf «Pause / Weiter» (WCAG 2.2.2, `aria-pressed`) und **bei verborgenem Tab** (`visibilitychange`; nicht Teil von F-3, hier benannt). Es hält **nicht** an beim Mauszeiger und nicht nach einem Mausklick. Bei «Bewegung reduzieren» keine Automatik, Zeitbalken ausgeblendet |
| **Posterreihe unter dem Band** (F-5, N-3, N-4) | Eine normale `PosterCard`-Reihe (`Row`) in der neuen **Variante ohne `href`** (`onPress`, `active`, `aria-current`, Zeitbalken als Kind): Enter, Leertaste oder Klick stellt den Titel ins Band und öffnet **nicht** die Detailseite. Der Trailer-Knopf bleibt allein an der Bühne (Entscheid vom 25. Oktober 2026). Alle Reihen rasten mit `scroll-snap-type: x mandatory` an der linken Kante ein; die Spotlight-Reihe rückt bei jedem Wechsel das aktive Poster an die linke Kante und trägt am Ende einen Leerraum (`aria-hidden`, Breite = Ausschnitt minus eine Karte). Unter 768 px zeigt jede Reihe **genau drei ganze Cover** (`calc((100% - 2 × gap) / 3)`); `CARD_SIZES`, `posterSizes` und `#poster-prepaint` bekommen `calc(33vw - 1.5rem)` unter 768 px |
| **Sortierschlüssel je Sprache** | Neue Spalte **`item_texts.sort_key`**: gezeigter Titel, Artikel der jeweiligen Sprache abgestreift («Der», «Die», «Das», «Ein», «Eine»; «The», «A», «An»), Kleinschreibung, Diakritika gefaltet (Ä→a, É→e, ß→ss). **Buchstabe** = erstes Zeichen des `sort_key`, grossgeschrieben, wenn in `[A-Z]`, sonst `#`. Einmaliger Backfill in der Migration über die vorhandenen `item_texts`; danach pflegt der Scanner und der Metadatenabgleich. `items.sort_title` bleibt für Staffeln, Folgen und den Scanner |
| **Wertung** | Neue Spalte **`items.vote_average REAL`** (0 bis 10, NULL = unbekannt), gefüllt wie ein Feld nach ADR 0015 in der Anbieterreihenfolge des Ordners (`item_field_sources` bekommt `vote_average`; TMDB liefert es, AniList `averageScore` ÷ 10; **TVDB nicht**: die v4-API kennt nur einen Beliebtheits-`score` ohne Obergrenze, keine 0–10-Wertung, darum bleibt eine Serie mit der Liste `[tvdb]` ohne Wertung; `local` nie). Backfill durch den Job `metadata.vote_backfill` über den Bestand, gedrosselt wie jeder Abgleich. Unbekannte stehen am Ende der Sortierung. Der Vertrag nennt die Sortierung `vote`, nie `rating` |
| **Sichtbare Bibliothek = mindestens ein sichtbarer Titel** | Abgeleitet aus `visible_items`, nicht aus einer zweiten Kopie der Freigaberegel. Eine freigegebene, aber leere Bibliothek erscheint **nicht** in der Seitenleiste und ist auf `/libraries/{id}` ein **404** wie eine nicht freigegebene (Regel 2, nie 403). `itemCount` zählt nur Titel der obersten Ebene (`movie`, `series`, `audiobook`, `podcast`). Die Seitenleiste und die Bibliotheksseite lesen ausschliesslich durch `visibility`; die Allowlists in `server/.golangci.yml` bleiben |
| **Serienstatus je Profil** | *angefangen* = mindestens eine sichtbare Folge hat Fortschritt und nicht alle sind abgeschlossen; *gesehen* = alle sichtbaren Folgen abgeschlossen; sonst *ungesehen*. Exklusiv. Podcasts gleich über ihre Folgen; Hörbücher und Filme über ihren eigenen Fortschritt. Dieselbe Regel füllt `ItemRef.progress.completed` für Serien in `listLibraryItems` |
| **Bibliotheksseite** (F-4, F-6, F-8, F-12) | `GET /libraries/{id}/items` (`listLibraryItems`, `scoped`): ein Raster, Chips «Alle · Filme · Serien» nur bei mehr als einer Inhaltsart (`content` bildet auf `items.kind` ab); Sortierung `title`, `added`, `released`, `vote`, `runtime`, `age`, `played`, `author` (nur Hörbücher, sonst 422); Filter `genre`, `yearFrom`, `yearTo`, `maxRating` (Server nimmt `min(Profilgrenze, Parameter)`), `status`; `letter` (`[A-Z#]`, nur mit `sort=title`, sonst 422 `letter_requires_title_sort`); Antwort mit **Facetten** (`letters`, `genres`, `ratings` unter der Profilgrenze) in einer Transaktion. Der Client zeigt nur, was in `facets` steht (Regel 12); eine Gruppe ohne Werte entfällt (so fehlen Genre und Freigabe bei Audio ohne Regel im Client) |
| **Seitenleiste** (F-1, F-11, N-1, N-2, O-1, O-4, O-5) | `GET /libraries/visible` (`listVisibleLibraries`, `scoped`) für die Gruppe «Meine Medien» mit Titelzahl und als letztem Eintrag «Cat's Empfehlungen» (`GET /picks`, `listPicks`, `scoped`, dieselbe Abfrage wie die Reihe ohne die 20er-Klemme). Ab 768 px feste Spalte links (15 rem), darunter und bei ausgeblendeter Leiste eine Schublade aus `Dialog`; Abmelden und Update-Hinweis ziehen in den Fuss der Leiste; die Kopfzeile behält Menü-Symbol, Suche, Glocke, Profil. Keine Zeile «Favoriten». Die Abfrage blockiert die Hülle nicht; offline zeigt die Gruppe einen Hinweis mit Verweis auf Downloads |
| **Geometrie des Vormalens** (P-10) | Eine CSS-Variable `--sidenav-width` (0 unter 768 px, 15 rem darüber, 0 bei ausgeblendeter Leiste), die `#hero-prepaint`, `#poster-prepaint`, das Band und das `sizes`-Attribut der vorgeladenen Bilder benutzen; ein Inline-Skript in `index.html` liest den Ausblendzustand aus `localStorage` und setzt `data-sidenav` vor dem ersten Malen |

## Was von ADR 0011 stehen bleibt

Cache (`spotlight_entries`), Kuratierung (`spotlight_overrides`), Ordnungsregel (Dedup, Anheftung,
Schnitt), `min_hits` vor dem Schnitt, Singleton `spotlight_config`, Veraltung 2 × `interval_hours`,
`HomePage.heroRow`, die Stelle der Bühne in der Reihenfolge, der Trailer-Knopf nur an der Bühne und
alle Admin-Operationen. **Abgelöst** sind allein die Zeilen zur Darstellung des Bands: Leiste in
zwei Dritteln, Beginn bei Platz 1, 8 s Takt, Pause bei Mauszeiger und bei jedem Fokus.

## Restrisiken

- **Das Band läuft unter dem Mauszeiger weiter** (F-3). Alle drei Bandknöpfe merken sich den Titel
  beim `pointerdown`; der `click` navigiert zum gemerkten Ziel. Der Pause-Knopf hält für jeden an.
- **Aggregationen über `visible_items`**: `total`, `letters`, `genres`, `ratings`, `itemCount` und
  der Serienstatus über Folgen. Ausstiegsklauseln (Konzept Abschnitt 8): Zähltabelle je Bibliothek,
  die der Scanner pflegt, wenn `itemCount` über 20 ms p95 bleibt; Hilfstabelle für den Serienstatus,
  wenn die Bibliotheksseite über 300 ms p95 liegt.
- **Zwei Stellen für dieselbe Geometrie** (`global.css`, `preload.go`) und ein Ausblendzustand im
  Browser: `--sidenav-width`, das Inline-Skript und ein Geometrie-Test bei 1280 px sind das
  Gegenmittel.
- **Sortierschlüssel und Wertung sind neue Daten**: der eine entsteht in Sekunden in der Migration,
  der andere durch einen Job gegen TMDB in etwa einer Stunde. Bis dahin sortiert «Bewertung»
  unvollständig; die Release-Notes nennen es.

## Folgen

- **Vertrag 1.4.0, additiv**: drei neue Operationen (`listVisibleLibraries`, `listLibraryItems`,
  `listPicks`, alle `scoped`), `getHome` mit `start`, `HomePage.heroStart` und
  `heroIntervalSeconds`, `SpotlightConfig.rotationSeconds`, `SpotlightConfigUpdate.rotationSeconds`
  optional, neue Schemata `LibraryNav`, `LibraryItemsPage`, `LibraryFacets`, `PicksPage`; `ItemRef`
  unverändert. Alle drei Operationen stehen in `TestContractCoversEveryOperation`;
  `listLibraryItems` bekommt den Negativtest nach K-3.
- **Migration 00026**: `spotlight_config.rotation_seconds`, `item_texts.sort_key` mit Backfill,
  `items.vote_average`, Indizes `(library_id, kind, added_at DESC)`, `(library_id, kind,
  release_date DESC)`, `(library_id, kind, vote_average DESC)` auf `items` und `(item_id, lang,
  sort_key)` auf `item_texts`. Nur vorwärts.
- **Drei neue Seiten in der Wirkung** (Regel 8): `web/e2e/library.spec.ts`,
  `web/e2e/picks-page.spec.ts`, `web/e2e/sidenav.spec.ts`; `home.spec.ts` und
  `admin-spotlight.spec.ts` wachsen.
- **P-10 und P-11 gelten weiter** und werden nach Konzept Abschnitt 8 nachgemessen
  (`docs/perf/startseite.md`): Lighthouse mobile **und** desktop, `measure-home.sh` und ein
  Schwesterskript für die Bibliotheksseite auf dem M3-Baum während eines Scans.
- Spec E-1, E-2, E-9 bleiben unverändert; die Spec-Zeile «Übersichtlichkeit hat Vorrang vor
  Effekten» wird durch eine ausblendbare Leiste eingelöst, die unter 768 px unsichtbar ist.
- **Kein Geheimnis im Protokoll** (Regel 10): `library_id`, `sort`, `start` dürfen ins Protokoll;
  nichts Neues mit Geheimnissen.
