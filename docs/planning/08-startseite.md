# Konzept: Startseite, Seitenleiste und Bibliotheksansichten

- **Status: Entwurf zur Freigabe (29. Oktober 2026).** Marco hat acht Fragen in zwei Runden
  beantwortet (Abschnitt 11, F-1 bis F-8); ein klickbarer Prototyp liegt unter
  `docs/planning/mockups/08-startseite-prototyp.html` (Abschnitt 7.4). Umgesetzt wird nichts, bevor
  Marco das Papier freigibt; die Entscheide gehen dann als **Frage 14** ins Entscheidungsprotokoll
  (`docs/planning/04-entscheidungen.md`).
- **Datum: 29. Oktober 2026**
- **Wogegen dieses Konzept verankert ist:** `main`, Stand **`9923807`** («Merge pull request #44»,
  Benutzerverwaltung), Vertragsfassung **1.2.0**, letzte Migration **00022**, letzte ADR **0014**.
  Es setzt das Konzept **07 Bibliotheken** (`docs/planning/07-bibliotheken.md`, Zweig
  `claude/bibliotheken`, PR #45, in Umsetzung) **voraus**: Familie je Bibliothek, Inhaltsart je
  Ordner, Vertrag 1.3.0, Migrationen 00023/00024, ADR 0015. Die nächsten freien Nummern sind darum
  **Migration 00025, ADR 0016, Vertrag 1.4.0**; sie verschieben sich, falls 07 weitere Nummern
  belegt. `:Zeile`-Angaben beziehen sich auf `9923807`; nach dem Merge von 07 nach Namen suchen,
  nicht nach Zeile.
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
  Pillen: Start, Suche, Merkliste, Downloads (nur mit Offline-Unterstützung), Verwaltung (nur
  Administrator), Profile (nur Hauptprofil); rechts Glocke, Profilwechsel, Abmelden
  (`web/src/app/AppLayout.tsx:64` bis `:132`). **Keine Seitenleiste, keine Bibliotheken in der
  Navigation.** Die Taste `/` öffnet die Suche von jeder Seite.
- **Startseite** (`web/src/features/catalog/HomePage.tsx`): genau die acht Reihen aus ADR 0011 in
  der vom Hauptbenutzer gewählten Reihenfolge, leere Reihen entfallen (E-9): `spotlight`, `picks`,
  `continue`, `recent`, `released`, `watchlist`, `collections`, `genres`. Der Server nennt in
  `HomePage.heroRow`, welche Reihe die Bühne ist (Regel 12); der Client entscheidet nichts.
- **Spotlight** (`web/src/ui/HeroCarousel.tsx`, ADR 0011): gerahmtes Band mit Backdrop, Titel,
  «Abspielen · Details · Trailer», darunter eine **Posterleiste in zwei Dritteln** der
  Reihen-Postergrösse (`HeroCarousel.module.css:195`, `calc(var(--sc-size-poster-sm) * 2 / 3)`).
  Das Band beginnt **immer mit Platz 1** der Liste, wechselt alle **8 s** (`intervalMs`,
  `HeroCarousel.tsx:171`) und **hält an**, sobald der Mauszeiger darüber steht, ein Element darin
  Fokus hat, ein Trailer offen ist oder das Betriebssystem «Bewegung reduzieren» meldet (`:180`).
  Für den LCP-Wert (P-10) malt der Server das erste Bild als `#hero-prepaint` vor, wenn die Bühne
  die erste Reihe ist (`server/internal/api/preload.go:180`).
- **Bibliotheken durchsehen.** Es gibt **keine Seite**, die alle Titel einer Bibliothek oder einer
  Inhaltsart zeigt. Am nächsten kommt die Suche im Blättermodus (`/search` ohne Text, mit Filtern:
  Genre, Jahr, Freigabe, Medienart, Verfügbarkeit; `web/src/features/discover/searchParams.ts`),
  sortiert **nur nach Titel**, ohne Bibliotheksfilter, 50 je Seite mit «Mehr laden».
- **Zuschauerseitige Bibliotheksdaten.** `GET /libraries` (`listAccountLibraries`,
  `x-visibility: account`) liefert Name und Art für die Profileinstellungen, **nicht** für ein
  Profil gefiltert und ohne Titel.

### 1.2 Was Marco vermisst (29. Oktober 2026, mit Emby-Bildschirmfoto)

1. **Eine Liste aller Filme, Serien, Dokus usw.** wie in Emby, je Bibliothek, auch für gemischte
   Bibliotheken (nach 07: Dokus mit Film- und Serienordner).
2. **Die linke Seite von Emby**: Start, dann «Meine Medien» mit einem Eintrag je Bibliothek.
   Die Kachelreihe «Meine Medien» **auf** der Startseite will er dagegen nicht; die Startseite soll
   übersichtlich bleiben.
3. **Spotlight bleibt**, aber: die Poster unter dem Band in **derselben Grösse** wie in den anderen
   Reihen, **ordentlich**, bei jedem Öffnen ein **anderer** Titel zuerst, und **alle paar
   Sekunden** der nächste.

### 1.3 Marcos Entscheide vom 29. Oktober 2026

Acht Fragen, zwei Runden, Wortlaut in Abschnitt 11:

| Nr. | Entscheid |
|---|---|
| F-1 | **Seitenleiste ab Tablet-Breite, Schublade auf dem Handy.** Die Kopfzeile behält nur Suche, Glocke, Profil |
| F-2 | **Zufälliger Starttitel je Aufruf**, vom Server gewählt; danach der Reihe nach weiter; Angeheftete bleiben in ihrer Ordnung |
| F-3 | **5 Sekunden Takt; Pause nur bei Tastaturfokus oder offenem Trailer**, nicht mehr beim blossen Mauszeiger; keine Automatik bei «Bewegung reduzieren» |
| F-4 | **Gemischte Bibliothek: ein Raster, Chips «Alle · Filme · Serien»**; reine Bibliotheken ohne Chips |
| F-5 | **Band bleibt, die Leiste darunter wird eine normale Posterreihe** mit denselben Karten und derselben Grösse wie «Weiterschauen»; Klick auf ein Poster stellt den Titel ins Band |
| F-6 | **Sortierungen nach dem Vorbild Emby**: Titel, Zuletzt hinzugefügt, Erscheinungsjahr, Bewertung — und was Emby sonst noch bietet, soweit es Slouchcat kennt (Laufzeit, Altersfreigabe, Zuletzt gesehen); mit Buchstabenleiste |
| F-7 | **Keine Reihe je Bibliothek auf der Startseite**; die acht Reihen bleiben (E-9). Neues je Bibliothek steht auf deren Seite unter «Zuletzt hinzugefügt» |
| F-8 | **Filter: Genre, Status (Ungesehen / Angefangen / Gesehen), Jahr und Altersfreigabe.** Verfügbarkeit (Offline) **nicht** |

---

## 2. Zielbild

### 2.1 Drei Bauteile

1. **Seitenleiste** (ab 1024 px Breite fest links, 15 rem, einklappbar auf Symbole; darunter eine
   Schublade über das Menü-Symbol oben links): Start, Suche, Merkliste, Downloads; Gruppe **«Meine
   Medien»** mit einem Eintrag je Bibliothek, die das aktive Profil sehen darf, mit Symbol der
   Familie und Titelzahl; unten Verwaltung und Profile nach denselben Regeln wie heute. Die Liste
   kommt vom Server (neue Operation `listProfileLibraries`, Abschnitt 5), der Client zeigt nur.
2. **Startseite ohne Kachelreihe**: Bühne und acht Reihen wie heute, **nichts Neues**, aber das
   Spotlight-Band nach F-2, F-3 und F-5: Posterreihe in Normalgrösse mit laufendem Zeitbalken am
   aktiven Poster, 5 s Takt, zufälliger Einstieg je Aufruf, Pause nur bei Fokus oder Trailer.
3. **Bibliotheksseite** `/libraries/{id}`: Überschrift mit Zähler, Werkzeugzeile (Chips für
   gemischte Bibliotheken, Sortierung mit Richtung, Filter), Buchstabenleiste bei Sortierung nach
   Titel (rechts ab Tablet, oben als Lauf auf dem Handy), Posterraster wie in der Suche, «Mehr
   laden». Der Zustand steht in der Adresse, damit Zurück, Teilen und Neuladen funktionieren
   (wie `searchParams.ts` für die Suche).

### 2.2 Was sich für den Zuschauer ändert

- Die Kopfzeile verliert die Pillen; die Navigation wandert nach links. Auf dem Handy bleibt oben
  das Menü-Symbol, die Suche und das Profil.
- Jede Bibliothek ist mit einem Klick erreichbar und vollständig durchsehbar; Serien erscheinen als
  Serie (nie als Folge), Hörbücher und Podcasts mit quadratischem Cover (wie heute in den Reihen).
- Der Spotlight-Einstieg wechselt bei jedem Öffnen; das Band läuft, solange niemand es mit der
  Tastatur hält.
- **Keine neue Regel im Client** (Regel 12): Sichtbarkeit, Altersgrenze, Bibliotheksfreigabe und
  der zufällige Starttitel kommen vom Server.

---

## 3. Konflikt mit Spec, Plan und ADRs

| Stelle | Was dort steht | Was dieses Papier daraus macht |
|---|---|---|
| Spec E-9 (`01-anforderungen-v1.md:122`), E-1 (`:114`) | Höchstens acht Reihen, keine leeren; Reihenfolge des Hauptbenutzers | **Unverändert** (F-7). Die Kachelreihe «Meine Medien» aus Emby wird nicht gebaut; die Bibliotheken stehen in der Seitenleiste |
| Spec Gestaltung (`:50`) | Dunkel, kinohaft, grosse Bildflächen, wenig Rahmen, «Übersichtlichkeit hat Vorrang vor Effekten» | Eine Seitenleiste ist ein zweiter Rahmen auf der Seite. Sie ist einklappbar und auf dem Handy unsichtbar; die Bühne behält die Breite des Inhaltsbereichs. Marcos Wahl (F-1) |
| Spec Non-Goal «Zufallsauswahl» (`:59`) | Keine Entdecken-Hilfe «Zufallsauswahl» | **Nicht berührt**: der zufällige Einstieg (F-2) wählt keinen Titel *für* den Zuschauer, er wählt nur, welcher der ohnehin gezeigten Spotlight-Titel zuerst im Band steht. Es entsteht keine neue Reihe und keine Empfehlung |
| ADR 0011, Zeile «Stelle der Bühne» und Plan 05 Abschnitt 7b (Mockup J) | Band mit Posterleiste «als Auswahl», Leiste in zwei Dritteln; 8 s; Pause bei Mauszeiger | **Geändert** (F-3, F-5): Leiste in Normalgrösse, 5 s, keine Pause bei Mauszeiger. ADR 0016 ist Teilnachfolger von 0011 in diesen drei Punkten; alles andere an 0011 (Cache, Overrides, `heroRow`, `min_hits`, Trailer-Knopf nur an der Bühne) bleibt |
| ADR 0011, `heroRow` | Der Server entscheidet die Bühne | **Erweitert**: der Server entscheidet auch den Einstieg (`HomePage.heroStart`, Abschnitt 5) |
| Entscheid vom 25. Oktober 2026 (`04-entscheidungen.md:71`) | Trailer-Knopf nur an der Bühne, `PosterCard` kennt keine Knöpfe | **Unverändert.** Die Posterreihe unter dem Band besteht aus `PosterCard`; ihr Klick stellt den Titel ins Band (F-5), die Detailseite erreicht man über «Details» im Band oder über das Poster in jeder anderen Reihe |
| Konzept 07, Abschnitt 2.1 | Zuschauer unverändert; Bibliotheksnavigation ist ein eigenes Vorhaben | **Dieses Vorhaben.** 07 muss zuerst in `main` sein (Familie und Inhaltsart je Ordner, sonst gibt es keine gemischte Bibliothek, die F-4 braucht) |
| Plan P-10, P-11 (`03-plan-v1.md:613`, `:614`) | LCP < 2.5 s, Startseiten-API < 300 ms p95 | **Gelten weiter** und werden in Schritt 5 nachgemessen; die Seitenleiste darf die Bühne nicht aus dem ersten Bild drängen (Abschnitt 8) |
| Regel 8 | Jede neue Seite hat einen Playwright-Tastaturtest | Zwei neue Seiten in der Wirkung: Bibliotheksseite und Seitenleiste/Schublade (Abschnitt 7.3) |

---

## 4. Datenmodell

Dieses Konzept braucht **keine neue Tabelle**. Zwei Dinge ändern sich:

### 4.1 `spotlight_config` bekommt den Takt

Spalte `rotation_seconds INTEGER NOT NULL DEFAULT 5 CHECK (rotation_seconds BETWEEN 3 AND 15)`.
Heute steckt der Takt als Konstante im Client (`intervalMs = 8000`); nach F-3 wird er 5 s, und wer
ihn einstellt, tut es dort, wo auch Anzahl und Mindesttreffer stehen: auf der Admin-Seite
«Spotlight». **Migration 00025** (nach Vorlage, `ALTER TABLE … ADD COLUMN`, nur vorwärts).

### 4.2 Indizes für die Bibliotheksseite

Die Seite liest `visible_items` je Bibliothek, Inhaltsart, Sortierung und Filter mit `LIMIT 50`.
In Schritt 1 wird **gemessen**, welche Indizes der Planer braucht; Kandidaten (alle über `items`,
nur falls noch nicht vorhanden; der Stand nach 07 ist massgeblich):

| Sortierung | Kandidat |
|---|---|
| Titel A–Z, Buchstabenleiste | `(library_id, kind, sort_title)` — `items.sort_title` gibt es seit 00003 (`:53`); zu prüfen ist nur, ob der Scanner Artikel («Der», «Die», «The») abstreift |
| Zuletzt hinzugefügt | `(library_id, kind, added_at DESC)` — `items_added` aus 00003 (`:80`) deckt nur `added_at` allein |
| Erscheinungsjahr | `(library_id, kind, release_date DESC)` — `items_released` aus 00021 deckt nur `release_date` allein |
| Bewertung | **`items` führt heute keine TMDB-Wertung**, nur die Altersfreigabe (`own_rating`, `effective_rating`). Migration 00025 ergänzt `vote_average REAL` (0 bis 10, NULL = unbekannt); der Metadatenabgleich (`metadata`, nach 07 der Job `metadata.fill`) schreibt `vote_average` aus TMDB, ein einmaliger Lauf nach der Migration füllt den Bestand. Bis dahin sortiert «Bewertung» Unbekannte ans Ende. Index `(library_id, kind, vote_average DESC)` |
| Laufzeit, Zuletzt gesehen | `runtime_minutes` (00003 `:57`) ohne Index; «Zuletzt gesehen» kommt aus dem Fortschritt je Profil (`progress.updated_at`) und sortiert Ungesehene ans Ende |

Status (Ungesehen / Angefangen / Gesehen) kommt aus dem Fortschritt **je Profil**; bei Serien
gilt: angefangen, sobald eine sichtbare Folge Fortschritt hat; gesehen, wenn alle sichtbaren
Folgen abgeschlossen sind. Das ist eine `EXISTS`-Prüfung je Zeile nach dem Sichtbarkeitsfilter und
wird in Schritt 1 auf der Testdatenbank (`testdata/db/M11.sql`, nach 07 der neue Stand) gemessen;
bleibt sie über 50 ms bei 1 000 Titeln, wird der Serienstatus in einer kleinen Hilfstabelle je
Profil mitgeführt (Ausstiegsklausel, nicht Plan).

Die **Buchstabenleiste** braucht je Anfrage die Menge der belegten Anfangsbuchstaben
(`SELECT DISTINCT substr(sort_title, 1, 1)` über dieselbe gefilterte Menge, ohne `LIMIT`). Das ist
eine zweite Abfrage je Seitenabruf; sie wird mit `total` zusammen in **einer** Transaktion gelesen.

### 4.3 Keine neue Sichtbarkeitsregel

Welche Bibliotheken ein Profil sieht, steht seit der Benutzerverwaltung an einer Stelle
(`accounts.all_libraries`, `account_libraries`, Altersgrenze des Profils, `visible_items`). Die
Seitenleiste und die Bibliotheksseite lesen **nur** durch `visibility` (Regel 2); eine Bibliothek,
die das Profil nicht sehen darf, ist auf `/libraries/{id}` ein **404**, nie ein 403.

---

## 5. Auswirkung auf den Vertrag

Alles ist **additiv**: keine Operation verschwindet, kein Enum verliert einen Wert. **Vertrag
1.4.0** (nach 1.3.0 aus 07), nicht brechend.

### 5.1 Neue Operationen

| Operation | Pfad | `x-visibility` | Zweck |
|---|---|---|---|
| `listProfileLibraries` | `GET /profile/libraries` | `scoped` | Die Bibliotheken, die das aktive Profil sehen darf, für die Seitenleiste: `id`, `name`, `family` (`video` / `audiobooks` / `podcasts`, aus 07), `contents` (Menge der Inhaltsarten der Ordner, aus 07: `movie`, `series`, `audiobook`, `podcast`), `itemCount` (sichtbare Titel). Reihenfolge wie in der Verwaltung |
| `listLibraryItems` | `GET /libraries/{id}/items` | `scoped` | Eine Seite Titel der Bibliothek. Parameter: `kind` (eine Inhaltsart aus `contents`; fehlt = alle), `sort` (`title`, `added`, `released`, `rating`, `runtime`, `age`, `played`; Vorgabe `title`), `order` (`asc`/`desc`; Vorgabe je `sort`: Titel und Freigabe aufsteigend, alles andere absteigend), `genre`, `yearFrom`, `yearTo`, `maxRating` (nur **strenger** als die Profilgrenze; der Server klemmt), `status` (`unwatched`, `started`, `watched`, mehrfach), `letter` (ein Grossbuchstabe oder `#`; nur bei `sort=title`, sonst 422), `offset`, `limit` (1 bis 100, Vorgabe 50). Antwort `LibraryItemsPage`: `items[ItemRef]`, `total`, `offset`, `limit`, `letters[]` (belegte Anfangsbuchstaben der gefilterten Menge, für die Leiste). Unsichtbare Bibliothek: 404 |

`listAccountLibraries` (`GET /libraries`, `account`) bleibt für die Profileinstellungen; es nennt
Bibliotheken, die ein Profil womöglich **nicht** sieht, und taugt darum nicht für die Seitenleiste.

### 5.2 Erweiterte Schemata

- `HomePage.heroStart` (`integer`, `minimum: 0`, optional): Index in `items` der Bühnenreihe, mit
  dem das Band beginnt; fehlt, wenn `heroRow` fehlt. **Der Server würfelt** je Antwort gleichverteilt
  über die Reihe **nach** Dedup, Anheftung und Schnitt (ADR 0011, «Ordnungsregel»), damit die
  Ordnung der Angehefteten unberührt bleibt (F-2). Der Client startet dort und läuft der Reihe
  nach; ein Fremd-Client (C-5) kann den Einstieg nachbauen.
- `SpotlightConfig.rotationSeconds` und `SpotlightConfigUpdate.rotationSeconds` (3 bis 15,
  Vorgabe 5); der Zuschauer bekommt den Wert als `HomePage.heroIntervalSeconds` (optional, nur mit
  `heroRow`), damit der Takt nicht im Client steht.
- `LibraryNav` (neu, für `listProfileLibraries`) und `LibraryItemsPage` (neu). `ItemRef` bleibt
  unverändert; die Karte auf der Bibliotheksseite braucht nichts, was die Reihen nicht schon
  zeigen (Fortschritt, Jahr, Autor, Freigabe-Chip aus `rating`).

### 5.3 Vertragstests

Beide Operationen tragen `x-visibility: scoped` und einen Eintrag in `TestContractCoversEveryOperation`
(Regel 7); `listLibraryItems` bekommt den **Negativtest** nach K-3: ein Kinderprofil sieht in einer
freigegebenen Bibliothek keinen Titel über seiner Stufe, und eine nicht freigegebene Bibliothek ist
404. `letter` ohne `sort=title` ist 422 `validation_failed`.

---

## 6. Server

- **`visibility`** bekommt `LibraryItems(ctx, profile, libraryID, query)` und
  `ProfileLibraries(ctx, profile)`; beide lesen `visible_items` (Regel 2, Allowlists in
  `server/.golangci.yml` **unverändert**, kein neues Paket).
- **`discover`** wählt den Einstieg (`heroStart`) nach dem Schnitt der Spotlight-Reihe; die
  Zufallsquelle ist `math/rand/v2` ohne Seed-Pflicht (kein Sicherheitsbelang). Der Startseiten-Cache
  (Plan `:804`) cached die Reihen, **nicht** den Einstieg: der Index wird nach dem Cache-Treffer
  gewürfelt, sonst sähe jeder Aufruf innerhalb der Cache-Dauer denselben Titel.
- **`preload.go`**: `#hero-prepaint` malt das Bild des **Starttitels** (`heroStart`), nicht mehr
  Platz 1; `FirstHero` nimmt den Index entgegen. Sonst ändert sich am Vormalen nichts; die Bühne
  behält Höhe und Rahmen.
- **`admin`**: `rotation_seconds` in `getSpotlightConfig` / `setSpotlightConfig`.
- **Logs** (Regel 10): nichts Neues mit Geheimnissen; `library_id` und `sort` dürfen ins Protokoll.

---

## 7. Oberfläche

### 7.1 Seitenleiste und Schublade

Neues Bauteil `web/src/ui/SideNav.tsx` (ein Bauteil, zwei Darstellungen): ab **1024 px** eine
feste Spalte links (15 rem; eingeklappt 4 rem mit Symbolen und Tooltip, Zustand je Gerät in
`localStorage`), darunter eine **Schublade** (`<dialog>`, Fokus beim Öffnen auf dem ersten Eintrag,
Escape und Scrim schliessen, Fokus zurück auf das Menü-Symbol). Inhalt und Reihenfolge:

1. Start, Suche, Merkliste, Downloads (Regeln wie heute in `AppLayout.tsx:64` bis `:80`);
2. Gruppe **«Meine Medien»** (ein- und ausklappbar, Zustand je Gerät): je sichtbarer Bibliothek
   ein Eintrag mit Symbol der Familie, Name und `itemCount`; aktive Bibliothek mit
   `aria-current="page"`;
3. unten Verwaltung (Administrator) und Profile (Hauptprofil) wie heute (`:82` bis `:95`).

`AppHeader` behält Menü-Symbol (nur unter 1024 px), Suche, Glocke, Profilwechsel, Abmelden; die
Pillen entfallen. Die Taste `/` bleibt. Die **räumliche Tastaturnavigation** (`web/src/ui/spatial.tsx`)
bekommt die Seitenleiste als eigene Spalte: Pfeil links aus der ersten Karte einer Reihe landet
auf dem passenden Eintrag, Pfeil rechts aus der Leiste auf dem zuletzt fokussierten Element im
Inhalt. Im Admin-Bereich bleibt die eigene `AdminNav`; die Seitenleiste zeigt dort nur den Eintrag
«Verwaltung» als aktiv.

### 7.2 Startseite und Spotlight

`HeroCarousel` behält Bühne, Pfeile, Positionsanzeige, Trailer-Knopf und das Vormalen. Es ändert sich:

| Heute | Neu |
|---|---|
| Leiste in zwei Dritteln von `poster-sm` mit eigenem `.poster`-Knopf | **`PosterCard`-Reihe** wie jede andere Reihe (`Row`, `--sc-size-poster-*`); das aktive Poster trägt Ring (`--sc-item-accent`) und den Zeitbalken; die übrigen sind auf 70 % abgedunkelt. Klick oder Enter auf ein Poster stellt den Titel ins Band und **öffnet nicht** die Detailseite (F-5) |
| `intervalMs = 8000` fest | `heroIntervalSeconds` aus der Antwort (Vorgabe 5) |
| Beginn bei Index 0 | Beginn bei `heroStart` |
| Pause bei Mauszeiger, Fokus, Trailer, Bewegung reduzieren, eine Folie | Pause bei **Fokus im Band**, offenem Trailer, einer Folie, Tab im Hintergrund (`visibilitychange`); bei «Bewegung reduzieren» keine Automatik, Zeitbalken ausgeblendet (F-3). **Kein** Halt beim Mauszeiger |
| Bild der nächsten Folie wird nach der aktiven geladen | unverändert; bei 5 s Takt wird zusätzlich die **übernächste** vorgeladen, damit ein langsames Netz (NAS, Abschnitt 12) keinen leeren Übergang zeigt |

Die Kachelreihe «Meine Medien» aus Emby wird **nicht** gebaut; die acht Reihen bleiben (F-7).
Jede Reihe mit einem Ziel bekommt rechts im Kopf «Alle anzeigen ›» (Merkliste → `/watchlist`,
Sammlungen → bleibt wie heute, Genres → Suche); «Neu hinzugefügt» und «Kürzlich veröffentlicht»
verweisen auf **keine** Seite, weil sie bibliotheksübergreifend sind und die Suche keine solche
Ordnung kennt (Frage O-3).

### 7.3 Bibliotheksseite

Route `/libraries/$libraryId` (`web/src/features/catalog/LibraryPage.tsx`, lazy, unter `_chrome`),
Zustand in der Adresse (`kind`, `sort`, `order`, `letter`, `genre`, `status`, `from`, `to`, `rating`)
mit einem `validateSearch` nach dem Muster von `searchParams.ts`. Aufbau von oben nach unten:

1. **Kopf**: Name der Bibliothek, Zähler «312 Titel» (bei Buchstabe oder Filter: «41 Titel mit B ·
   gefiltert»).
2. **Werkzeugzeile**: Chips «Alle · Filme · Serien» nur bei `contents.length > 1` (F-4); rechts
   «Sortieren» (`<select>`, die sieben Ordnungen; bei Audio ohne «Altersfreigabe»), Richtungsknopf,
   «Filter» mit Zähler der aktiven Filter.
3. **Filterfeld** (ausklappbar, `aria-expanded`): Genre-Chips (aus `listGenres`), Status-Chips,
   Jahr von–bis, Altersfreigabe-Chips «Profilgrenze · bis 0 · 6 · 12 · 16 · 18» (nur Stufen unter
   der Profilgrenze werden angeboten, der Server klemmt trotzdem). «Anwenden» schliesst das Feld und
   setzt den Fokus auf «Filter». Bei Audio keine Genre- und keine Freigabe-Filter; Status heisst
   «Ungehört / Angefangen / Gehört».
4. **Raster** (`ItemGrid` aus der Suche, `PosterCard` mit Fortschritt, Haken bei gesehen,
   Freigabe-Chip; quadratische Cover bei Audio) und daneben die **Buchstabenleiste** (nur bei
   `sort=title` und Video; rechts, `position: sticky`; unter 1024 px als horizontaler Lauf über dem
   Raster). Buchstaben ohne Titel sind `disabled`; «•» hebt die Wahl auf. Die Leiste springt nicht
   im Raster, sie **filtert** (`letter`), damit die Seite auch bei 2 000 Titeln klein bleibt.
5. **«Mehr laden (142 weitere)»** wie in der Suche (`useInfiniteQuery`, 50 je Seite). Keine
   Virtualisierung; bei «Mehr laden» bleibt der Fokus auf der ersten neuen Karte.

Leere Zustände: Bibliothek ohne sichtbare Titel («Hier ist es noch leer»), Filter ohne Treffer
(«Nichts passt zu diesen Filtern» mit «Zurücksetzen»).

**Regel 8 — Tastaturtests**, neu: `web/e2e/library.spec.ts` (Seite: Tab-Reihenfolge Kopf →
Chips → Sortieren → Richtung → Filter → Leiste → Raster; Pfeiltasten im Raster; Enter öffnet die
Detailseite; Zurück landet wieder auf derselben Karte und derselben Adresse; sichtbarer Fokus;
keine Fokusfalle im Filterfeld), `web/e2e/sidenav.spec.ts` (Seitenleiste: Pfeil hoch/runter,
Enter, eingeklappt mit Tooltip; Schublade auf 390 px: Öffnen, Escape, Fokusrückgabe, kein
Scrollen der Seite hinter dem Scrim), und `web/e2e/home.spec.ts` wächst um: Band beginnt bei
`heroStart`, Enter auf einem Poster der Spotlight-Reihe wechselt das Band ohne Navigation, Fokus im
Band hält den Takt an, Mauszeiger nicht.

**Regel 9 — Texte und Farben**: neue Schlüssel `nav.libraries` («Meine Medien»), `nav.menu`,
`nav.collapse`, `library.page.{count, countLetter, filtered, sort, direction, filter, apply,
clear, emptyFilters, more}`, `library.sort.{title, added, released, rating, runtime, age, played}`,
`library.status.{unwatched, started, watched, unheard, heard}`, `library.content.{all, movie,
series}`, `library.letters.all`, `home.rows.all` («Alle anzeigen»), `admin.spotlight.rotation`;
`pnpm -C web test locales` prüft die Symmetrie. Farben nur aus Tokens; der Zeitbalken nimmt
`--sc-color-text-primary`, der Ring `--sc-item-accent`.

### 7.4 Klickbarer Prototyp

`docs/planning/mockups/08-startseite-prototyp.html` ist eine einzelne HTML-Datei ohne
Abhängigkeiten (Doppelklick genügt). Sie zeigt die Seitenleiste mit Marcos sieben Bibliotheken
(Filme, Serien, Dokus, Kids, Anime, Hörbücher, Podcasts; Dokus, Kids und Anime gemischt), die
Startseite mit dem Spotlight-Band und der Posterreihe in Normalgrösse (5 s Takt, zufälliger
Einstieg je Aufruf, Zeitbalken, Pause bei Tastaturfokus, Pfeiltasten im Band), die acht Reihen,
und die Bibliotheksseite mit Chips, Sortierung, Richtung, Filterfeld, Buchstabenleiste,
Zähler und «Mehr laden». Der Knopf **«Handy-Ansicht»** in der Prototyp-Leiste zeigt dieselbe Seite
in einem 400-px-Rahmen mit Schublade. Poster sind Farbverläufe mit Titel; Farben und Abstände sind
die Tokens aus `packages/design-tokens/tokens/`. Der Prototyp ist **kein** Code für `web/src` und
wird nicht gebaut oder getestet.

---

## 8. Leistung

- **P-10 (LCP < 2.5 s)**: Die Bühne bleibt das grösste Element im ersten Bild. Die Seitenleiste
  ist Text und Symbole, kein Bild; sie wird mit der App-Hülle gezeichnet. Das Vormalen bleibt an
  seiner Stelle, die Breite des Inhaltsbereichs sinkt ab 1024 px um 15 rem; `#hero-prepaint` in
  `web/src/ui/global.css:73` und die Breiten in `preload.go:30` folgen **in demselben Commit**
  (Schritt 2), sonst zeigt das Vormalen einen anderen Ausschnitt als das Band.
- **P-11 (Startseiten-API < 300 ms p95)**: `heroStart` ist ein Zufallszug nach dem Cache; das kostet
  nichts Messbares. `listProfileLibraries` ist ein zweiter Aufruf beim Start der App (mit der
  Sitzung vorgeladen wie `/home`, `preload.go:58`), eine Abfrage über `libraries` und `visible_items`
  mit `COUNT` je Bibliothek — gemessen in Schritt 1; bleibt sie über 20 ms, entfällt `itemCount` aus
  der Vorladung und kommt per `staleTime` nach.
- **Bibliotheksseite**: Ziel < 150 ms p95 je Seite auf der Testdatenbank bei 1 000 Titeln,
  inklusive `total` und `letters`; Messung mit `k6` wie in `docs/perf/M3.md`. Ergebnisse nach
  `docs/perf/startseite.md`.

---

## 9. Sicherheit

- **Nichts Neues im Client.** Beide neuen Operationen sind `scoped`; die Altersgrenze und die
  Bibliotheksfreigabe sitzen in `visible_items` (Regel 2 und 12). `maxRating` kann die Grenze nur
  **senken**, nie heben: der Server nimmt `min(profil, parameter)`.
- **404 statt 403** für eine Bibliothek, die das Profil nicht sieht; die Seitenleiste nennt sie
  nicht, die Adresse verrät sie nicht.
- **Keine neue Aussenverbindung, kein Geheimnis.** `rotation_seconds` ist eine Zahl.
- **`letter`** wird auf genau ein Zeichen aus `[A-Z#]` geprüft, bevor es in die Abfrage geht
  (vorbereitetes Statement über sqlc, kein String-Bau).

---

## 10. Schnitt in Schritte

Vorbedingung: **Konzept 07 ist in `main`** (Familie, `contents` je Ordner). Zweig
`claude/startseite`, Basis `main` nach dem Merge von PR #45. Jeder Schritt ein Commit, `task ci`
grün, DoD mit Befehl und Ausgabe in `docs/dev/CURRENT_STATUS.md`.

### Schritt 0 — Entscheide und Papiere
Frage 14 ins Entscheidungsprotokoll (F-1 bis F-8 und die Antworten auf O-1 bis O-4), **ADR 0016**
«Seitenleiste, Bibliotheksseite und Spotlight-Takt» (Teilnachfolger von ADR 0011 in drei Zeilen),
Status dieses Papiers auf «freigegeben, in Umsetzung». Reiner Doku-Commit.

### Schritt 1 — Vertrag 1.4.0 und Server
`api/openapi.yaml` (zwei Operationen, drei Felder, zwei Schemata), `task gen`, Migration 00025
(`rotation_seconds`, ggf. `sort_title` und Indizes), Abfragen in `server/queries/scoped/` mit
`visible_items`, `visibility.LibraryItems` und `ProfileLibraries`, `discover.heroStart`,
`preload` mit Starttitel, Admin-Konfiguration. Vertragstests, Negativtest K-3, Messung nach
Abschnitt 8 mit Belegen. **DoD:** `task ci` grün; `curl` gegen beide Operationen mit einem
Kinderprofil zeigt die Klemme.

### Schritt 2 — Seitenleiste und Schublade
`SideNav`, `AppLayout`, `AppHeader` ohne Pillen, räumliche Navigation, Vormalen nachgezogen
(`global.css`, `preload.go`), `sidenav.spec.ts`, Texte. **DoD:** `task ci` grün; Lighthouse
mobile auf der Startseite (LCP) nicht schlechter als `docs/perf/M11.md`.

### Schritt 3 — Bibliotheksseite
Route, `LibraryPage`, Adresszustand, Filterfeld, Buchstabenleiste, `library.spec.ts`, Texte,
Verweise aus der Seitenleiste. **DoD:** `task ci` grün; Playwright-Test für Tastatur und Zurück.

### Schritt 4 — Spotlight nach F-2, F-3, F-5
`HeroCarousel` mit `PosterCard`-Reihe, `heroStart`, `heroIntervalSeconds`, neue Pausenregel,
Vorladen der übernächsten Folie, Admin-Feld «Takt», `home.spec.ts` und `admin-spotlight.spec.ts`
erweitert, Vergleichsbilder erneuert. **DoD:** `task ci` grün; Belege für Fokus-Pause und
Mauszeiger-Weiterlauf.

### Schritt 5 — Abnahme, Messung, Doku
Lighthouse mobile (P-10), `k6` (P-11 und Bibliotheksseite), Gerätecheck am Handy (Schublade,
Lauf der Buchstabenleiste, Band auf 390 px), `docs/perf/startseite.md`,
`docs/dev/CURRENT_STATUS.md`, Release-Notes-Zeile (nicht brechend, 1.4.0), eingefrorener
Datenbankstand `testdata/db/startseite.sql` mit Test. Pull Request, Merge auf Marcos Wort.

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

### 11.2 Offen (vor Schritt 0)

| Nr. | Frage | Vorschlag |
|---|---|---|
| O-1 | Soll die Seitenleiste je Bibliothek die **Titelzahl** zeigen (wie im Prototyp)? Sie kostet eine `COUNT`-Abfrage je Bibliothek beim Start. | Ja, mit der Ausstiegsklausel aus Abschnitt 8 |
| O-2 | Hörbücher: Buchstabenleiste nach **Titel** oder nach **Autor**? Emby sortiert Hörbücher nicht; Audiobookshelf bietet beides. | Nach Titel; «Autor» als achte Sortierung nur für Hörbücher, ohne Leiste |
| O-3 | «Alle anzeigen ›» an den Reihen: nur dort, wo eine Seite existiert (Merkliste, Genres), oder auch an «Neu hinzugefügt» mit einer bibliotheksübergreifenden Seite? | Nur dort, wo eine Seite existiert; keine neue Seite «Alles neu» |
| O-4 | Soll die Seitenleiste eine Zeile **«Favoriten»** bekommen (es gibt `listFavorites`, aber keinen Navigationseintrag)? | Nein in diesem Papier; Favoriten bleiben am Titel und in der Merkliste erreichbar wie heute |

---

## 12. Risiken

### 12.1 Das Band läuft unter dem Mauszeiger weiter
Mit F-3 wechselt der Titel, während jemand auf «Abspielen» zielt; der Knopf bleibt an seiner
Stelle, der Inhalt dahinter nicht. Milderung: der Klick auf «Abspielen» nimmt den Titel, der beim
`pointerdown` im Band stand, nicht den beim `click`. Der Prototyp läuft bereits ohne
Mauszeiger-Pause; Marco prüft dort, ob es stört. Fällt es durch, ist «Pause bei Mauszeiger» ein
Einzeiler.

### 12.2 Das Vormalen und die Seitenleiste
Zwei Stellen beschreiben dieselbe Geometrie (`global.css`, `preload.go`). Die Seitenleiste ändert
die Breite des Inhaltsbereichs; vergisst Schritt 2 eine Seite, springt das Bild beim Laden (CLS)
und P-10 kippt. Gegenmittel: beide Stellen in demselben Commit, Lighthouse im DoD.

### 12.3 Zähler und Buchstaben sind zwei Abfragen mehr
`total`, `letters` und `itemCount` sind Aggregationen über `visible_items`. Bei 1 000 Titeln
unkritisch, bei 20 000 unbekannt. Die Messung in Schritt 1 entscheidet; die Ausstiegsklauseln
stehen in 4.2 und 8.

### 12.4 Abhängigkeit von 07
Ohne Familie und `contents` gibt es keine gemischte Bibliothek, und die Nummern (00025, 0016,
1.4.0) stehen nicht fest. Dieses Papier wartet auf den Merge von PR #45; bis dahin verschiebt sich
jede Zeilenangabe.

### 12.5 Fernseher-Browser
Eine Seitenleiste nimmt auf einem 1080p-Fernseher aus drei Metern Entfernung Platz, den die
Bühne gut brauchte. Sie ist einklappbar, und der Zustand hält je Gerät. Mehr ist nicht
vorgesehen; TV-Apps sind Non-Goal.

### 12.6 Was dieses Papier nicht zählen konnte
Ob `items` einen Sortiertitel führt, welche Indizes 07 anlegt und wie `spatial.tsx` eine zweite
Spalte verträgt, wurde gegen `9923807` nachgeschlagen, nicht gegen den Stand nach 07. Schritt 1 und
2 prüfen es; die Angaben in 4.2 sind Kandidaten, keine Zusagen.
