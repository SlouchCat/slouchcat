# Konzept: Bedienung, Löschen und die neue Seite «Gesehen»

- **Status: Fassung 3 (12. November 2026): Nachtrag P-15 «Kontinent-Kacheln» (Abschnitt 22).
  Am 12. November 2026 hat Marco O-1 bis O-14 beantwortet (Abschnitt 18) und den Bau freigegeben.**
  Fassung 2 vom 11. November 2026 hat das Kritiker-Review eingearbeitet (REVISE, 1 Blocker,
  13 Major, 13 Minor; Abschnitt 21). Marco hat am 10. und 11. November 2026 im Interview
  dreizehn Fragen beantwortet (**F-1 bis F-13**, Abschnitt 17), drei davon an klickbaren Mockups
  (Anhang A), zwei nach dem Review. Offen sind die Punkte **O-1 bis O-14** (Abschnitt 18), jeder
  mit Empfehlung; bis zu Marcos Antwort gilt die Empfehlung. **Die Umsetzung beginnt erst auf
  Marcos ausdrückliches Wort.**
- **Auslöser:** Marco hat am 10. November 2026 nach der Abnahme von Konzept 19 Stufe D3 auf CT 190
  vierzehn Wünsche geschickt (Abschnitt 1) und dazu ein Konzept mit Interview verlangt, für die
  Staffeln ausdrücklich einen Mockup-Vorschlag.
- **Verankert gegen:** `main` **`ee06058b`** («Merge pull request #138», Release 1.25.0),
  Vertragsfassung **1.25.0** (`api/openapi.yaml:4`), letzte Migration
  **`00045_audio_series.sql`**, letzte ADR **`0031`** (0029 und 0030 sind für Konzept 19
  reserviert), letzte Frage im Entscheidungsprotokoll **26**. Offen daneben: Pull Request #139
  (Konzept 19 Stufe D3, Vertrag 1.26.0, Migration 00046), auf CT 190 bereits als Image
  `quellen-d430e29` im Einsatz.
- **Nummern:** Dieses Papier ist **Konzept 20**, reserviert **Frage 27** und **ADR 0032**
  (Dateien löschen, Abschnitt 11). Vertragsfassungen und Migrationsnummern nennt das Papier
  **nicht fest**: Konzept 19 hat für die Teile A und B noch Nummern offen, und vergeben wird in
  Bau-Reihenfolge (goose verlangt lückenlose Reihenfolge, die Regel «Vertrag = Tag» eine
  aufsteigende Version). Abschnitt 19 sagt je Stufe nur, **ob** sie Vertrag und Migration braucht.
- **`main` ist seither weitergelaufen:** Am 12. November 2026 steht es auf **`6791dbda`**
  (Release 1.30.1, Vertrag 1.30.0, letzte Migration `00052`), 140 Commits nach dem Anker; darin
  Konzept 19 Stufe D3 und die Teile A (Podcasts) und B (Radio-Angebot). Die Abschnitte 2 bis 15
  beschreiben «Heute» am Anker `ee06058b`; nur Abschnitt 22 ist gegen `6791dbda` gelesen. **Vor
  dem Bau jeder Stufe** werden ihre «Heute»-Angaben am dann aktuellen `main` nachgeprüft
  (Schritt 0). Bekannt überholt: O-7 (PR #139 ist gemergt). Frage 27 und ADR 0032 sind auf `main`
  weiterhin frei.
- `:Zeile`-Angaben beziehen sich auf `ee06058b`; **nach Namen suchen, nicht nach Zeile.** Pfade
  ohne Präfix liegen unter `server/` (Go) oder `web/src/` (Web-App), wo es eindeutig ist.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
  Wo dieses Papier einen früheren Entscheid ändert, steht das in Abschnitt 16.

> **Dieses Papier ändert keine Zeile Code.** Es beschreibt vierzehn Punkte, sagt je Punkt, was der
> Code heute tut, und schlägt eine Lösung vor. Zwölf Punkte sind klein bis mittel; zwei sind
> grösser: das **Löschen von Titeln samt Dateien** (Abschnitt 11, hebt einen alten Entscheid auf)
> und die **Seite «Gesehen»** mit dem Haken zum Anklicken (Abschnitte 6 und 15). Abschnitt 19
> ordnet alles in fünf Stufen, die einzeln erscheinen können.

## 1. Was Marco will

| Nr. | Marcos Wunsch (10. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| P-1 | Bei den Staffeln kann man nicht gut nach rechts scrollen. Mockup-Vorschlag. | 2 |
| P-2 | «Favorit» gibt es nicht mehr. Überall entfernen. | 3 |
| P-3 | Bei den Staffelbildern fehlt der Bleistift. | 4 |
| P-4 | Ein Klick auf eine Folge startet die Folge, statt die Folgenseite zu öffnen. Play-Knopf auf dem Bild wie in Emby. Bei angefangenen Folgen fragen: neu starten oder weiterschauen. | 5 |
| P-5 | Ein Klick auf den leeren Kreis einer ungesehenen Folge markiert sie als gesehen. | 6 |
| P-6 | Bei den Jobs fehlt «alle fehlgeschlagenen löschen» und «alle erneut versuchen». | 7 |
| P-7 | Ein Klick auf «Weiterschauen», «Neu hinzugefügt» usw. auf der Startseite führt zur ganzen Liste. Dazu ein klickbarer Spotlight-Titel. Bleistifte auf den Listen nicht vergessen. | 8 |
| P-8 | Eine Bibliothek lässt sich nicht löschen. | 9 |
| P-9 | Filme, Serien, Staffeln, Folgen, Podcasts und Hörbücher lassen sich nicht löschen. | 10, 11 |
| P-10 | Die Duplikatsuche soll den Pfad der Dateien zeigen. | 12 |
| P-11 | Nach dem Löschen eines Ordners in «test2» fragt die Seite «bleiben oder verwerfen», es gibt aber keinen Knopf zum Speichern. | 13 |
| P-12 | «Schlüssel der Anbieter» sieht chaotisch aus. Aufräumen. | 14 |
| P-13 | «Meine Wiedergabe» soll «Einstellungen» heissen. | 14.4 |
| P-14 | Neue Kategorie auf der Höhe von «Radio»: alles, was einmal gesehen wurde; Serien und Podcasts als ganze Serie. | 15 |
| P-15 | Nachtrag vom 12. November 2026, mit Bildschirmfoto: Die Kontinent-Knöpfe unter «Verwaltung › Radio › Angebot › Länder» sehen schlecht aus. Anpassen. | 22 |

## 2. P-1 Staffelreihe mit Pfeilen

### 2.1 Heute

- Die Serienseite ist `ItemPage`; für `kind === 'series'` zeichnet sie `SeasonShelf`
  (`features/catalog/ItemPage.tsx:620-622`, `features/catalog/SeasonShelf.tsx:146-223`).
- `SeasonShelf` benutzt `RowList` mit einer `PosterCard` je Staffel (`SeasonShelf.tsx:167-204`).
  `RowList` scrollt waagrecht mit versteckter Scrollleiste und `scroll-snap-type: x mandatory`
  (`ui/Row.module.css:67-77`, `:102`).
- **Pfeilknöpfe hat nur `Row`**, die Hülle um `RowList` (`ui/Row.tsx:103-152`): zwei Knöpfe, die um
  80 % der Breite blättern, sichtbar nur mit Maus und nur beim Zeigen auf die Reihe
  (`Row.module.css:45-60`). `Row` nutzen die Startseite und auf der Detailseite Extras und
  Besetzung. **Die Staffeln nutzen `RowList` ohne Pfeile**, ebenso die Bände einer Sammlung
  (`CollectionShelf.tsx:67`), «Meine Sender» (`RadioPage.tsx:511`) und die Personen der Suche
  (`SearchPage.tsx:304`). Die Posterleiste des Spotlights ist ebenfalls eine `RowList`
  (`ui/HeroCarousel.tsx:598`), hat aber eigene Pfeile und bleibt, wie sie ist.
- Mit der Maus bleibt also nur Umschalt+Mausrad oder die Tastatur. Ziehen geht nicht, eine
  Scrollleiste ist nicht zu sehen. Das ist Marcos Befund.

### 2.2 Vorschlag (Variante A, F-9)

Marco hat am Mockup (Anhang A, Bild «Serienseite») zwischen drei Varianten gewählt: **A Reihe mit
Pfeilen**, B Zahlenknöpfe, C Raster. Gebaut wird A:

1. `RowList` bekommt die Eigenschaft `arrows` (`'hover'` wie heute in `Row`, `'always'`, `'none'`).
   Die Blätterlogik wandert aus `Row` in `RowList`; `Row` reicht `'hover'` durch und ändert sich
   für die Startseite nicht.
2. `SeasonShelf` setzt `arrows="always"`: Die zwei Pfeile stehen **fest** links und rechts neben der
   Reihe (44 px, auch auf Touch-Geräten), sind am Anfang und am Ende deaktiviert und fehlen ganz,
   wenn alle Staffeln ohne Scrollen Platz haben.
3. Mit der Maus lässt sich die Reihe **ziehen** (nur `pointer: fine`; ein Zug über 6 px gilt nicht
   als Klick auf die Karte).
4. Die gewählte Staffel **rückt von selbst ins Bild**, beim Laden der Seite (die Staffel der
   nächsten Folge kann Staffel 9 sein) und beim Wechsel mit der Tastatur.
5. `scroll-snap-type` der Staffelreihe wird `x proximity`, damit Ziehen und Trackpad nicht
   einrasten, wo man es nicht will.
6. Dieselben festen Pfeile bekommen die drei anderen Reihen ohne Pfeile (Sammlungsbände, «Meine
   Sender» auf der Radio-Seite, Personen in der Suche), damit sich alle Reihen gleich bedienen.

**Das Mausrad bleibt beim senkrechten Scrollen der Seite.** Würde die Reihe das Rad schlucken,
bliebe man beim Scrollen der Seite in ihr hängen. Umschalt+Mausrad scrollt seitwärts wie überall.
(Der erste Text im Mockup sagte «Mausrad scrollt seitwärts»; er ist korrigiert.)

### 2.3 Was sich ändert

- Nur Web-App: `ui/Row.tsx`, `ui/Row.module.css`, `SeasonShelf.tsx`, `CollectionShelf.tsx`,
  `RadioPage.tsx`, `SearchPage.tsx`. Kein Vertrag, keine Migration.
- Tests: Die Pfeile sind wie heute nicht in der Tab-Reihenfolge (`excludeFromTabOrder`,
  `Row.tsx:128-145`); der Tastaturtest der Serienseite (`e2e/series.spec.ts`) prüft neu, dass die
  gewählte Staffel sichtbar ist. Storybook-Bild für `RowList` mit `arrows="always"`.

## 3. P-2 «Favorit» entfernen, die Merkliste übernimmt

### 3.1 Heute

«Favorit» ist **nicht** verschwunden, sondern lebt an zwei Orten:

- **Serien und Podcasts (E-8):** runder Knopf «Favorit» mit Herz auf der Detailseite
  (`features/catalog/DetailActions.tsx:190-199`, `:252-263`), Tabelle `favorites`
  (`migrations/00015_discover.sql`), Operationen `listFavorites`, `addFavorite`, `removeFavorite`
  (`api/openapi.yaml:5165-5222`), Feld `ItemDetail.favorite`. Der Favorit löst zwei Dinge aus:
  die **Benachrichtigung «neue Folge»** (`internal/notify/notify.go`, `NewEpisodes`) und das
  **automatische Herunterladen neuer Podcast-Folgen** auf das Gerät (O-3,
  `internal/offline/sync.go:252-314`).
- **Radio:** das Herz an einem Sender legt ihn unter «Meine Sender» ab (Tabelle
  `radio_favorites`, eigene Operationen).

Eine Seite «Favoriten» gab es nie; `listFavorites` ruft die Web-App nicht auf. Die Merkliste
(`watchlist`) ist eine eigene Funktion und nimmt jede Art ausser Staffel und Extra, also auch
Serien und Podcasts.

### 3.2 Entscheid (F-1, F-2)

- **Der Favorit bei Serien und Podcasts verschwindet ganz** (Knopf, Tabelle, Schnittstelle).
- **Die Merkliste übernimmt** beide Auslöser: Steht eine Serie oder ein Podcast auf der Merkliste
  eines Profils, bekommt das Profil die Benachrichtigung «neue Folge», und die Regel «neue
  Podcast-Folgen automatisch laden» greift.
- **Das Herz beim Radio bleibt** («Meine Sender», Entscheid OB-6 vom 10. November 2026).

### 3.3 Bau

- **Migration** (nur vorwärts, nach Vorlage): bestehende Favoriten in die Merkliste übernehmen,
  dann `DROP TABLE favorites`. Niemand verliert eine Benachrichtigung.
- **Neue Spalte `watchlist.follow_since`** (Unix-Millisekunden). Die Regel «neue Podcast-Folgen
  automatisch laden» rechnet «neu» ab dem Datum des Eintrags (`ListFavoritePodcasts`,
  `queries/system/offline.sql:180-196`). Übernähme sie einfach `watchlist.created_at`, gälten bei
  einem Podcast, der seit einem Jahr auf der Merkliste steht, alle Folgen eines Jahres als neu,
  und jedes Gerät mit dieser Regel lüde beim ersten Abgleich alte Folgen. Darum:
  - übernommener Favorit: `follow_since` = Datum des Favoriten (auch wenn der Titel schon auf der
    Merkliste stand; `created_at` der Merkliste bleibt, es bestimmt ihre Reihenfolge),
  - bestehender Merklisten-Eintrag ohne Favorit: `follow_since` = Zeitpunkt der Migration,
  - neuer Eintrag: `follow_since` = `created_at`.
- **Server:** `NotifyHasFavorites` und `NotifyFavoriteTargets` (`queries/system/notify.sql`) lesen
  neu `watchlist`, eingeschränkt auf Einträge der Art Serie und Podcast; ebenso die Abfrage der
  Offline-Regel, sie rechnet mit `follow_since`. Es entfallen `discover.Favorites`,
  `AddFavorite`, `RemoveFavorite` (`internal/discover/m7.go:67-94`), `visibility.Favorites` und
  `IsFavorite`, `model.ErrNotFavoritable`, der Favoriten-Zweig beim Zusammenführen und Umhängen
  (`internal/library/relink.go`, `queries/system/relink.sql:107`, `:125`, `admin.sql:264-268`).
- **Vertrag (brechend):** `listFavorites`, `addFavorite`, `removeFavorite`, `ItemDetail.favorite`,
  der Zähler `favorites` in der Umhänge-Vorschau und der Fehlercode `not_favoritable` entfallen.
  Die Texte der Benachrichtigungs-Schemas nennen neu die Merkliste.
- **Web-App:** Knopf und `useFavoriteMutation` weg; Texte `item.favorite.*`,
  `errors.code.not_favoritable`, `admin.maintenance.relink.data.favorites_*` weg; die Texte
  `notifications.empty.body`, `settings.notifications.intro`, `offline.rules.intro` und
  `offline.rules.newPodcastEpisodes` nennen neu die Merkliste. `e2e/favorites.spec.ts` entfällt;
  seine zwei Abläufe (Benachrichtigung, Auto-Download) wandern in die Tests der Merkliste.
- **Tests, die mitziehen müssen:** die Vertragstest-Einträge der drei entfallenen Operationen
  (Regel 7); `frozen_m7_test.go:48-49`, `frozen_m8_test.go:42` und `migration00015_test.go:80`
  (sie fragen `favorites` nach dem Hochziehen auf die neueste Fassung ab); der Prüfer für fremde
  Clients `cmd/apicheck` (`checks.go:34`, `:224-247`, `fixtures.go:355-357`);
  `contract_notify_test.go`; `e2e/discover.ts`, `maintenance.spec.ts`, `notifications.spec.ts`,
  `podcast-auto-download.spec.ts`, `durations.json`; ein Migrationstest für die Übernahme samt
  `follow_since` (kein Nachladen alter Folgen).
- **Doku:** Spec E-8 (`01-anforderungen-v1.md:31`, `:124`, `:180`, `:330`) und ADR 0008 bekommen
  einen Nachtrag («Favorit durch Merkliste abgelöst»). Ältere Konzepte, Reviews, Mockups und die
  eingefrorenen DB-Stände in `testdata/db/` bleiben, wie sie sind: Sie beschreiben den damaligen
  Stand.
- Der freie Platz in der Knopfreihe der Detailseite geht an den Haken «Gesehen» (Abschnitt 6).

### 3.4 Folge, die man kennen muss

Jede Serie auf der Merkliste meldet künftig neue Folgen, auch wenn man sie nur «für später»
gemerkt hat. Die Obergrenze von 50 Meldungen je Serie und Lauf bleibt (`notify.MaxPerParent`).
Wer keine Meldung will, nimmt die Serie von der Merkliste.

## 4. P-3 Bleistift auf den Staffelcovern

### 4.1 Heute

- Der Bleistift-Editor kennt Staffeln längst: `EDITABLE_KINDS` enthält `season`
  (`features/catalog/EditMetadata.tsx:16-22`), der Server bietet für Staffeln das Poster zur Wahl
  und zum Hochladen (`internal/metadata/image_candidates.go:44-47`, `listItemImageCandidates`,
  `setItemImage`, `uploadItemImage`).
- Es fehlt nur der Einstieg: Die Staffelkarten in `SeasonShelf` haben weder Bleistift noch Link
  (`SeasonShelf.tsx:177-201`); die Staffelseite `/items/{seasonId}` hätte den Bleistift, ist von
  der Serienseite aus aber nicht erreichbar.

### 4.2 Vorschlag

- Jede Staffelkarte bekommt für den Server-Administrator (`canManage`, wie auf allen anderen
  Karten) den `CoverPen` im `tool`-Platz der `PosterCard`. Er öffnet `EditMetadata` für die
  Staffel, Reiter «Bilder» zuerst.
- Tab-Reihenfolge: nur die gewählte Staffelkarte ist ein Tab-Halt (wie heute), danach ihr
  Bleistift und ihr Haken; mit den Pfeiltasten wechselt man die Staffel.
- Kein Vertrag, keine Migration. Tastaturtest in `e2e/series.spec.ts` ergänzen (Bleistift der
  Staffel öffnet den Dialog, Escape gibt den Fokus zurück).

## 5. P-4 Ein Klick startet die Folge

### 5.1 Heute

- Die Folgenzeile ist `EpisodeListItem` (`ui/EpisodeListItem.tsx:34-107`). **Die ganze Zeile ist
  ein Link zur Folgenseite** (`SeasonShelf.tsx:65`); abgespielt wird erst dort. Der Test heisst
  entsprechend «Enter opens an episode, play starts it» (`e2e/series.spec.ts:103-124`).
- Auf dem Folgenbild liegt kein Play-Knopf, nur der Fortschrittsbalken.
- Die Wahl «weiter oder von vorn» gibt es nur auf der Detailseite als zwei Knöpfe
  (`DetailActions.tsx:119-131`, `:233-240`): «Weiterschauen» und der runde Knopf «Von vorne»
  (`paths.playAt(id, 0)`). Der Player kennt dafür `?at=` (`app/router.tsx:612-619`); ohne `at`
  gilt die gemerkte Stelle des Servers.

### 5.2 Vorschlag (F-6, F-7)

**Die Folgenzeile bekommt drei Ziele statt einem** (Mockup «Serienseite»):

| Ziel | Was es tut |
|---|---|
| Bild und Text (ein Link, die ganze Fläche) | **startet die Folge.** Auf dem Bild liegt ein runder Play-Knopf, beim Zeigen und im Fokus in der Akzentfarbe. |
| Knopf «Details» rechts | öffnet die Folgenseite (Inhalt, Besetzung, Version wählen, Bleistift). |
| Kreis ganz rechts | setzt oder entfernt den Haken «gesehen» (Abschnitt 6). |

**Nachfrage bei angefangenen Titeln.** Ist die Folge angefangen (Stelle grösser als null, nicht
fertig), öffnet der Klick statt des Players einen kleinen Dialog:

- «Weiterschauen ab 21:36» (Vorgabe, hat den Fokus) → `/play/{id}`
- «Von vorn starten» → `/play/{id}?at=0`
- «Abbrechen» oder Escape → nichts

Der Dialog ist ein gemeinsames Bauteil `ResumeChoice` in `web/src/ui` (auf `Dialog`), dazu ein
Haken `usePlayStart(item)`, der entscheidet, ob er nötig ist. Er gilt **überall, wo ein einziger
Klick ohne Detailseite abspielt** (F-7):

- Folgenliste von Serien und Staffeln (`EpisodeListItem`),
- Folgenliste von Podcasts (`ui/EpisodeRow.tsx`, Knopf `playHref`),
- Bände einer Hörbuch-Reihe (`VolumeRow`, `SeriesPage.tsx:342`; Text «Weiterhören ab …»),
- «Abspielen» im Spotlight (`ui/HeroCarousel.tsx:515-526`), **nur bei Filmen**. Eine Serie im
  Spotlight hat keinen eigenen Fortschritt (`ItemRef.progress` kommt nur aus der Zeile des Titels
  selbst, `internal/visibility/mapping.go:212-222`); ob ihre nächste Folge angefangen ist, weiss
  die Karte nicht. Für Serien bleibt «Abspielen» im Spotlight, wie es heute ist. Ein Feld
  «nächste Folge» am Spotlight-Eintrag wäre eine Vertragsänderung und gehört nicht in Stufe 1.

**Nicht** gefragt wird bei Karten der Reihe «Weiterschauen» (dort ist Weiterschauen gemeint) und
auf der Detailseite (dort stehen die zwei Knöpfe schon).

### 5.3 Was sich ändert

- Nur Web-App; kein Vertrag (die Folgen tragen ihren Fortschritt schon, `SeasonShelf.tsx:74-82`).
- `e2e/series.spec.ts` wird umgeschrieben: Enter auf der Zeile startet die Folge; Tab führt zu
  «Details», dann zum Kreis; eine angefangene Folge öffnet den Dialog, Escape schliesst ihn und
  gibt den Fokus an die Zeile zurück (Regel 8).
- Offline-Bibliothek: unverändert; sie hat ihre eigene Liste.

## 6. P-5 Der Haken «gesehen» zum Anklicken

### 6.1 Heute

- Der Kreis in der Folgenzeile ist **nur eine Anzeige**, kein Knopf: ein `<span>` mit Haken bei
  «gesehen», sonst ein leerer Ring, beide innerhalb des Zeilen-Links
  (`ui/EpisodeListItem.tsx:97-103`). Ein Klick darauf öffnet deshalb die Folgenseite. Das ist
  Marcos «der leere Button tut nichts».
- **Es gibt keine Operation «als gesehen markieren».** Geschrieben wird der Fortschritt nur durch
  `reportProgress` (`api/openapi.yaml:2138-2146`); «gesehen» entsteht allein aus der 90-%-Regel
  (`internal/playback/playback.go`, `Completed`).
- Die Regel wird **bei jedem Ereignis neu gerechnet** (AN-10): Wer einen gesehenen Film nochmals
  startet, macht ihn wieder «ungesehen». Ein Datum «gesehen am» gibt es nicht; `progress` hat nur
  `position_ms`, `duration_ms`, `completed`, `updated_at`
  (`migrations/00004_playback_progress.sql:9-17`).
- Serie, Staffel und Podcast haben keinen eigenen Zustand; er wird aus den Folgen abgeleitet
  (`internal/visibility/library.go:266-269`).

### 6.2 Entscheid (F-8, F-10)

Marco will «wie bei den gesehenen Titeln einfach den Haken setzen», und zwar **überall auf den
Karten**. Der Haken wird ein Knopf:

| Ort | Verhalten |
|---|---|
| Folgenzeile (Serie, Staffel, Podcast) | Kreis rechts: leer = ungesehen, gefüllt mit Haken = gesehen. Ein Klick wechselt. |
| Staffelcover in der Staffelreihe | Haken oben rechts: setzt alle Folgen der Staffel. |
| Posterkarten (Startseite, Listen, Bibliotheksraster, Suche, Merkliste, Sammlung, Person, «Gesehen») | Haken oben rechts, dort, wo heute das Abzeichen «gesehen» steht (`ui/PosterCard.tsx:216-217`). Bei Serie und Podcast setzt er alle Folgen. |
| Detailseite | Runder Knopf «Gesehen» in der Knopfreihe (auf dem Platz des Favoriten), für Film, Folge, Hörbuch, Podcast-Folge, Staffel, Serie, Podcast. |

Regeln:

- **Drei Zustände bei Staffel, Serie und Podcast (F-12):** leer (keine Folge gesehen), **halb
  gefüllt** («teilweise gesehen»), voll (alle sichtbaren Folgen gesehen). Überall gleich, auch
  unter «Gesehen». Einzelne Titel kennen nur leer und voll.
- **Setzen** (Klick auf den leeren Kreis) geht ohne Rückfrage, auch für eine ganze Serie.
- **Klick auf den halben Haken** fragt: «Alle Folgen als gesehen markieren» oder «Alle Haken
  entfernen».
- **Entfernen** geht bei einem einzelnen Titel ohne Rückfrage. Bei Staffel, Serie und Podcast
  fragt ein Dialog nach («Bei 19 Folgen fällt der Haken …»), weil sich das nicht zurückholen lässt.
- Auf Touch-Geräten ist der Haken wie der Bleistift immer sichtbar; mit Maus erscheint der
  **leere** Kreis auf Posterkarten erst beim Zeigen oder im Fokus, der **gesetzte** Haken ist
  immer zu sehen (O-1).
- Offline ist der Haken nur Anzeige (Regel 12: Der Server entscheidet).

### 6.3 «Gesehen» wird dauerhaft

Damit ein Titel unter «Gesehen» bleibt, auch wenn man ihn nochmals anfängt (F-4), braucht es ein
eigenes Merkmal neben `completed`:

- **Neue Spalte `progress.watched_at`** (Unix-Millisekunden, `NULL` = nie fertig gesehen), dazu ein
  Teilindex `(profile_id, watched_at DESC) WHERE watched_at IS NOT NULL`.
- **Gesetzt wird sie von jedem angenommenen Ereignis, das die 90-%-Regel erfüllt:**
  `watched_at = max(watched_at, Zeit des Ereignisses)`. Das gilt ausdrücklich auch für die zwei
  Wege, die `completed` heute **nicht** anfassen: das verspätete Online-Ereignis
  (`playback.go:419-425`, der Zweig `isLate` schreibt vor dem Upsert fest) und das
  Offline-Ereignis, das älter ist als der gespeicherte Fortschritt
  (`internal/playback/offline.go:62-66`, «zählt nur für die Schauzeit»). Sonst erschiene ein
  Film, der offline fertig gesehen und vor dem Hochladen online nochmals gestartet wurde, nie
  unter «Gesehen». Je Weg ein Test.
- Dazu setzt sie der Haken von Hand (6.4).
- Sie **bleibt stehen**, wenn ein späteres Ereignis den Titel wieder öffnet. `completed` und
  `position_ms` verhalten sich wie heute (AN-10 bleibt für «Weiterschauen»).
- **Zusammenführen und Umhängen:** `AdminMergeProgress` (`queries/system/admin.sql:228-239`,
  benutzt von `admin/maintenance.go:503` und `library/relink.go:562`) übernimmt heute die Zeile,
  die «weiter» ist. `watched_at` bekommt dort eine eigene Regel ausserhalb dieses Vergleichs:
  das spätere der beiden Daten gewinnt. Sonst verlöre ein Titel mitten im zweiten Durchgang
  sein Datum. Test.
- **Migration:** `watched_at = updated_at` für alle Zeilen mit `completed = 1`. Titel, die vor dem
  Update fertig gesehen und danach wieder angefangen wurden, lassen sich nicht nachtragen; das
  Wissen fehlt in der Datenbank.

**Wer liest künftig was.** Die Faustregel: Alles, was **weiterführt**, liest `completed` wie
heute; alles, was **anzeigt oder filtert**, liest `watched_at`.

| Leser | Liest | Bemerkung |
|---|---|---|
| «Weiterschauen» (`ListContinueWatching`, `queries/scoped/items.sql:46-99`) | `completed`, `position_ms`, `updated_at` | unverändert |
| Nächste Folge (`ListSeriesNextUpEpisodes`, `GetNextVisibleEpisode`, `visibility/nextup.go`) | `completed` | unverändert |
| Nächster Band einer Hörbuch-Reihe (`visibility/audio_series.go:51-68`) | `completed` | unverändert |
| Offline-Regeln (`ListRecentEpisodeProgress`, `CountOpenAutoDownloads`, `queries/system/offline.sql`) | `completed`, `updated_at` | unverändert |
| Laufende Sitzungen (`internal/playback/sessions.go:237`) | `completed` | unverändert |
| Haken auf Karten und Zeilen, `ItemRef.watched`, `watchedCount` (`items.sql:204-246`, `visibility/tree.go`) | **`watched_at`** | geändert |
| Filter und Zustand der Bibliothek (`visibility/library.go:266-269`, `:412-418`, `:506-540`) | **`watched_at`**, dann `position_ms` | geändert, siehe unten |
| «Gesehen» (`listWatched`) | **`watched_at`** | neu |

- **Zustand in der Bibliothek bleibt eindeutig:** `watched`, sobald `watched_at` gesetzt ist;
  `started` nur ohne `watched_at` und mit Stelle grösser als null; sonst `unwatched`. Ein
  gesehener Film im zweiten Durchgang zählt also als «gesehen», nicht als «angefangen»; in
  «Weiterschauen» steht er trotzdem.
- **Schritt 0 des Baus:** jede Fundstelle von `completed` in `server/` und `web/` einer der zwei
  Spalten zuordnen und die Tabelle oben vervollständigen.

### 6.4 Vertrag und Server

- **Neue Operation `setItemWatched`**: `PUT /items/{id}/watched`, Körper `{ "watched": true|false }`,
  `x-visibility: scoped`; ein nicht sichtbarer Titel gibt 404.
  - **Einzelner Titel, `true`** (Film, Folge, Extra, Hörbuch, Podcast-Folge): `completed = 1`,
    `position_ms = duration_ms`, `watched_at = jetzt`, `updated_at = jetzt`. Gibt es noch keine
    Zeile, kommt `duration_ms` aus der Dauer der Hauptdatei (Probe), sonst aus der Laufzeit der
    Metadaten oder des Feeds; ist keine Dauer bekannt, stehen Dauer und Stelle auf 0.
  - **Einzelner Titel, `false`:** `watched_at = NULL`. War der Titel fertig (`completed = 1`),
    zusätzlich `completed = 0` und `position_ms = 0`. **Eine laufende Stelle bleibt** (zweiter
    Durchgang, `completed = 0`), und **`updated_at` bleibt unverändert**: Das Entfernen eines
    Hakens darf weder «Weiterschauen» umsortieren noch die Offline-Regel «nächste Folge laden»
    auslösen (`ListRecentEpisodeProgress` hat keinen Filter auf die Stelle,
    `offline.sql:170-178`). Die Zeile bleibt wegen der gemerkten Hörgeschwindigkeit.
  - **Serie, Staffel, Podcast:** dasselbe für **alle sichtbaren Folgen** darunter, in einer
    Transaktion. Folgen, die das Profil nicht sehen darf, bleiben unberührt.
  - **`updated_at` beim Setzen vieler Folgen gestaffelt:** in Folgenreihenfolge aufsteigend
    (`jetzt − n` … `jetzt`, eine Millisekunde Abstand), die letzte Folge ist die jüngste. Bekämen
    alle dieselbe Zeit, griffe die Offline-Regel «nächste Folge» eine beliebige Folge heraus
    (`internal/offline/sync.go:281-299` nimmt die erste Zeile je Serie) und lüde womöglich eine
    eben markierte Folge; «Weiterschauen» entschiede den Gleichstand nach der grössten Kennung
    (`items.sql:59-61`). Mit der Staffelung erscheint nach «Staffel 1 gesehen» die Serie in
    «Weiterschauen» mit der ersten Folge von Staffel 2 (Regel aus Konzept 15, P-1). Test für
    beide Leser.
- **`ItemRef.watched` gibt es schon** (seit Vertrag 1.22.0: «alle sichtbaren Folgen fertig»,
  `queries/scoped/items.sql:204-246`, `visibility/tree.go:20-21`). Das Feld bleibt, seine
  **Bedeutung ändert sich** von `completed` auf `watched_at`; die Beschreibung im Vertrag wird
  angepasst. `watchedCount` zählt ebenfalls neu nach `watched_at`. Aus `watchedCount` und der
  Zahl der Folgen ergibt sich der halbe Haken (F-12).
- **Additiv:** `watchedAt` am Fortschritt eines Titels.
- Das Schreiben liegt im Paket `playback` (wie `reportProgress`), das Lesen in `visibility`
  (Regel 2). Ein Aktivitätsereignis schreibt der Haken von Hand nicht (O-2).
- Tests: Vertragstest je neue Operation (Regel 7), Sichtbarkeitstest (Kinderprofil darf über die
  Serie keine gesperrte Folge markieren), eingefrorener DB-Stand für die Migration.

## 7. P-6 Jobs: alle löschen, alle erneut versuchen

### 7.1 Heute

- Der Reiter «Fehlgeschlagen» (`features/admin/JobsPage.tsx:1835-1951`) liest
  `jobs.recentFailed` aus dem Dashboard; **der Server kürzt diese Liste auf zehn**
  (`internal/admin/dashboard.go:22-23`). Je Zeile gibt es «Erneut» (`retryJob`) und einen
  Papierkorb, der «Abbrechen» heisst (`cancelJob`, setzt den Lauf auf `cancelled`).
- Sammelaktionen gibt es nur je Jobart und nur für wartende und laufende Läufe
  (`cancelJobRuns`, `scope=queued|all`); fehlgeschlagene sind ausgenommen
  (`queries/system/jobs.sql:182-184`).
- Fehlgeschlagene Läufe bleiben, bis jemand handelt (F-8 aus Konzept 09); abgebrochene räumt
  `jobs.prune` nach 30 Tagen weg.

### 7.2 Vorschlag

Mockup «Jobs, Löschen, Duplikate», Thema «Jobs: Fehlgeschlagen».

- **Zwei Knöpfe über der Liste:** «Alle erneut versuchen» und «Alle löschen …». Das Löschen fragt
  mit der Anzahl nach. Dieselben zwei Knöpfe stehen je Jobart unter «Läufe», wenn dort der Filter
  «fehlgeschlagen» gewählt ist.
- **Die Liste zeigt alle fehlgeschlagenen Läufe**, seitenweise, nicht nur zehn.
- Der Knopf je Zeile heisst **«Löschen»** statt «Abbrechen»; auch unter «Läufe» bekommt ein
  fehlgeschlagener Lauf ihn (dort fehlt er heute, `JobsPage.tsx:1780-1790`).
- «Löschen» heisst wie heute beim Papierkorb: Der Lauf wird `cancelled` und verschwindet aus
  «Fehlgeschlagen». Unter «Läufe» steht er danach als «abgebrochen», nicht mehr als
  «fehlgeschlagen», bis `jobs.prune` ihn wegräumt (älter als 30 Tage und nicht unter den jüngsten
  50 Läufen seiner Art, `queries/system/jobs.sql:199-212`). Die Rückfrage sagt das.

### 7.3 Vertrag und Server

- `retryFailedJobs`: `POST /admin/jobs/failed/retry`, optional `type`; antwortet mit
  `{ retried, skipped, merged }`.
  - Übersprungen wird ein Lauf, wenn schon ein wartender oder laufender Zwilling mit demselben
    Idempotenzschlüssel besteht (dieselbe Regel wie `RetryFailedJob`,
    `queries/system/admin_dashboard.sql:20-28`).
  - Haben **mehrere fehlgeschlagene Läufe denselben Schlüssel**, wird nur der jüngste neu
    eingereiht; die älteren werden verworfen (`merged`). Eine einzige `UPDATE`-Anweisung über
    alle würde sonst am eindeutigen Teilindex `jobs_active_idempotency_key`
    (`migrations/00001_jobs.sql:27-28`) scheitern und ganz abbrechen. Test mit doppeltem
    Schlüssel.
- `dismissFailedJobs`: `DELETE /admin/jobs/failed`, optional `type`; antwortet mit `{ dismissed }`.
- `listJobRuns`: bekommt `offset` (fehlt heute, `api/openapi.yaml:6095-6125`), und `type` wird
  freiwillig. «Ohne `type` nur mit `state=failed`» lässt sich in OpenAPI nicht ausdrücken; der
  Handler prüft es und antwortet sonst mit 422. Der generierte Parameter wird ein Zeiger.
- Alle drei `x-visibility: admin`, additiv. Die Sammelaktionen laufen im Server in einer
  Transaktion, nicht als Schleife im Browser.

## 8. P-7 Startseite: Reihentitel führen zur Liste

### 8.1 Heute

- Die Startseite kommt aus einem Aufruf `getHome`; jede Reihe hat höchstens 20 Karten
  (`visibility.MaxRowItems`). Der **Reihentitel ist reiner Text** (`ui/Row.tsx:124-126`).
- Rechts vom Titel steht bei vier Reihen ein eigener Link «Alle anzeigen ›»
  (`HomePage.tsx:53-61`): User-Empfehlungen, Merkliste, «Meine Sender», «Neu in …».
- **Für «Weiterschauen», «Neu hinzugefügt», «Kürzlich veröffentlicht», «Spotlight» und
  «Sammlungen» gibt es keine Liste**, weder als Seite noch als Operation. Das war ein Entscheid:
  Konzept 08, O-3 («Nein; nur Cat's Empfehlungen und Merkliste haben eine Seite»).
- Im Spotlight sind nur die Knöpfe klickbar; der Filmtitel und das Etikett «Spotlight» sind Text
  (`ui/HeroCarousel.tsx:496-510`).
- Der Bleistift steht auf den Karten der Startseite und des Bibliotheksrasters, **nicht** auf
  Merkliste, User-Empfehlungen, Sammlung, Suche und Personenseite: `PagedItemGrid` reicht den
  `tool`-Platz nicht durch (`features/discover/PagedItemGrid.tsx:7-19`).

### 8.2 Vorschlag

Mockups «Startseite» und «Listenseite».

1. **Der Reihentitel selbst wird der Link**, mit einem kleinen Pfeil dahinter. Der getrennte Link
   «Alle anzeigen ›» entfällt; es gibt je Reihe ein Ziel statt zwei.
2. **Ziele:**

   | Reihe | Ziel |
   |---|---|
   | Spotlight | `/lists/spotlight` (neu) |
   | Weiterschauen | `/lists/continue` (neu) |
   | Neu hinzugefügt | `/lists/recent` (neu) |
   | Kürzlich veröffentlicht | `/lists/released` (neu) |
   | Sammlungen | `/collections` (neu, O-3) |
   | User-Empfehlungen, Merkliste, Meine Sender, «Neu in …» | wie heute (`/picks`, `/watchlist`, `/radio`, `/libraries/{id}?sort=added`) |
   | Genres, Meine Medien | kein Ziel; jede Kachel ist schon ein Ziel |

3. **Eine Listenseite für alle vier neuen Listen** (`HomeListPage`, Route `/lists/$key`):
   Überschrift, Anzahl, Filter nach Medienart, Raster mit Nachladen (`PagedItemGrid`), Bleistift
   und Haken auf jeder Karte. Reihenfolge und Regeln sind **dieselben wie in der Reihe**, nur ohne
   die Grenze von 20:
   - Weiterschauen: je Serie eine Karte mit der nächsten Folge (Konzept 15, P-1).
   - Neu hinzugefügt: über alle sichtbaren Bibliotheken.
   - Kürzlich veröffentlicht: die letzten zwölf Monate (`visibility.ReleasedMonths`).
   - Spotlight: alle Titel der gewählten Trakt-Liste, die in der sichtbaren Bibliothek liegen,
     ohne den Schnitt auf `item_count`; ist das Spotlight abgeschaltet, gibt die Liste 404.
4. **Spotlight:** Über der Bühne steht neu die Überschrift «Spotlight» als Link wie bei den
   anderen Reihen; das kleine Etikett in der Bühne entfällt. Zusätzlich wird der **Filmtitel in
   der Bühne** ein Link zur Detailseite (Marcos Satz lässt beide Lesarten zu; beides kostet wenig).
5. **Bleistifte auf allen Listen:** `PagedItemGrid` bekommt den `tool`-Platz und reicht ihn an
   `ItemGrid` weiter. Damit tragen auch Merkliste, User-Empfehlungen, Sammlungsseite, Suche und
   Personenseite den Bleistift, immer nur für den Server-Administrator und nur für die Arten, die
   der Editor kann.

### 8.3 Vertrag und Server

- **Neue Operation `listHomeRow`**: `GET /home/rows/{key}` mit `key` aus `continue`, `recent`,
  `released`, `spotlight`; Parameter `content` (Medienart), `offset`, `limit` (höchstens 100).
  `x-visibility: scoped`; Antwort wie die anderen Listen (`items`, `total`), die Karten in
  derselben Form wie in `getHome`.
- Die vier Leser in `visibility` begrenzen heute hart auf 20 (`clamp`, `MaxRowItems`,
  `visibility.go:194-260`) und lassen sich **nicht alle gleich** blättern. Je Reihe:

  | Reihe | Wie die Liste entsteht |
  |---|---|
  | `recent`, `released` | Eigene Abfragen mit `OFFSET`/`LIMIT` und eine Zählabfrage in `queries/scoped/`. Der Filter `content` darf den Teilindex `items_top_added` nicht aushebeln; der Bau prüft den Abfrageplan. |
  | `continue` | Die Abfrage rechnet immer die ganze Liste (ein Eintrag je Serie, `items.sql:46-99`); ein SQL-Versatz spart nichts. Der Server rechnet sie einmal je Aufruf **bis höchstens 200 Einträge** und schneidet die Seite in Go; `total` ist die Länge. Die Reihenfolge kann sich zwischen zwei Seiten verschieben, wenn gerade jemand schaut; das ist hingenommen. |
  | `spotlight` | Die Reihenfolge entsteht in Go (`placeSpotlight`, `visibility/discover.go:298-330`), die Abfrage hat mit Absicht kein `LIMIT` (`queries/scoped/spotlight.sql:1-7`, höchstens 200 Zeilen). Geblättert wird in Go nach dem Einsetzen der angehefteten Titel, ohne den Schnitt auf `item_count`. |

- **Filter `content`:** zählt die Art des **Titels der obersten Ebene**. «Serien» trifft in
  «Weiterschauen» also die Karten mit der nächsten Folge einer Serie.
- **Spotlight ohne Reihe:** Ist das Spotlight abgeschaltet oder liegt die Trefferzahl unter
  `min_hits`, fehlt die Reihe auf der Startseite, und `listHomeRow` gibt für `spotlight` 404
  (dieselbe Bedingung wie die Reihe, `discover.go:387-389`).
- **Messauftrag M-3:** Antwortzeit von `listHomeRow` für `continue` mit 200 angefangenen Titeln
  und für `recent` mit Filter auf CT 190; Ziel unter 150 ms.
- Alle Abfragen bleiben in `queries/scoped/` und enthalten `visible_items` (Regel 2).
- **Neue Operation `listCollections`** für `/collections` (O-3), falls sie gebaut wird.
- Additiv. Tastaturtest für die neue Seite. **Der Reihentitel wird ein Fokus-Halt:** Pfeil nach
  oben aus einer Reihe führt neu auf ihren Titel statt in die Reihe darüber, Enter öffnet die
  Liste. Das ändert die Pfeilwege **aller** Reihen der Startseite; die Tastaturtests der
  Startseite (`e2e/home*.spec.ts`, `home-radio.spec.ts`) werden entsprechend nachgeführt.

## 9. P-8 Bibliothek löschen

### 9.1 Heute

Eine Bibliothek lässt sich nicht löschen: Der Vertrag hat kein `deleteLibrary`
(`/admin/libraries/{id}` kennt nur `get` und `patch`), die Oberfläche keinen Knopf. Konzept 15
hält das fest (`15-feinschliff.md:538-540`). Man kann nur einzelne Ordner entfernen
(`removeLibraryRoot`). Die Fremdschlüssel sind grösstenteils vorbereitet: Mit der Bibliothekszeile
fallen Ordner, Titel und alles, was an Titeln hängt (`migrations/00003_libraries_items.sql:37`,
`:51`). **Eine Lücke gibt es:** `profile_libraries.library_id` hat keinen Fremdschlüssel
(`migrations/00002_accounts_sessions.sql:43-47`), und `libraries.id` wird ohne `AUTOINCREMENT`
vergeben (`00023_library_roots_providers.sql:88-89`), SQLite vergibt die höchste Nummer also neu.

### 9.2 Vorschlag

Mockup «Jobs, Löschen, Duplikate», Thema «Bibliothek löschen».

- Neuer Menüeintrag **«Bibliothek löschen …»** in der Zeile der Bibliothek
  (`features/admin/libraries/LibraryDetail.tsx:73-120`).
- Der Dialog (`ConfirmInputDialog`, `ui/Dialog.tsx:161`) nennt Zahl der Ordner und Titel und
  verlangt, den **Namen der Bibliothek einzutippen**.
- **Eine Bibliothek löschen entfernt nie eine Datei.** Wer auch die Dateien loswerden will,
  löscht sie selbst oder vorher titelweise (Abschnitt 11).
- Gelöscht wird **endgültig** (kein Soft-Delete wie beim Entfernen eines Ordners): Titel,
  Fortschritt, Merkliste, Empfehlungen, eigene Bilder und Texte, die Startseitenreihe «Neu in …»,
  die Freigaben der Profile, Downloads dieser Titel. Ein späterer Scan derselben Ordner in einer
  neuen Bibliothek legt die Titel ohne Benutzerdaten neu an.
- **Sicherheitsnetz:** Vor dem Löschen sichert der Server die Datenbank mit `VACUUM INTO`. Die
  Sicherung bekommt einen **eigenen Namen und eine eigene Aufbewahrung** (die letzten zwei), damit
  sie keine der fünf Sicherungen vor Migrationen verdrängt (`internal/store/migrate.go:19`,
  `:72`). Der Dialog sagt dazu: Wer diese Sicherung zurückspielt, setzt **alles** auf den Stand
  vor dem Löschen zurück, auch den Fortschritt aller Profile seither.
- **Sammlungen:** TMDB-Sammlungen, die danach kein Mitglied mehr haben, werden mitgelöscht;
  von Hand angelegte Sammlungen bleiben leer stehen.

### 9.3 Vertrag und Server

- `deleteLibrary`: `POST /admin/libraries/{id}/deletion`, Körper `{ "confirmName": "…" }`,
  `x-visibility: admin`; 422, wenn der Name nicht passt. Das folgt dem vorhandenen Muster
  (`POST …/roots/{rootId}/removal` mit `confirmPath`) und hält den Namen aus der Adresszeile und
  damit aus den Logs.
- **Die Freigaben der Profile müssen ausdrücklich mit (Blocker aus dem Review).** Bliebe eine
  Zeile in `profile_libraries` stehen und bekäme die nächste neue Bibliothek dieselbe Nummer,
  sähe ein eingeschränktes Profil die neue Bibliothek, ohne dass jemand sie freigegeben hat
  (`visible_items` prüft nicht, ob die Bibliothek noch besteht, `00022_account_roles.sql:89-94`).
  Darum beides:
  1. `deleteLibrary` löscht `profile_libraries WHERE library_id = ?` in derselben Transaktion.
  2. Eine **Migration** baut `profile_libraries` mit Fremdschlüssel auf `libraries` neu
     (`ON DELETE CASCADE`, nach Vorlage; verwaiste Zeilen fallen dabei weg).
  3. Test: die Bibliothek mit der höchsten Nummer löschen, eine neue anlegen; das eingeschränkte
     Profil sieht sie nicht.
  `activity_events.library_id` (`00031:27`) wird für die gelöschte Bibliothek auf `NULL` gesetzt;
  der Name steht im Text des Ereignisses.
- Ablauf im Paket `library` unter der Scan-Sperre (`lockScan`):
  1. Laufenden Scan abbrechen. **Wartende und laufende Jobs zu Titeln der Bibliothek abbrechen**
     (Abgleich, Trickplay, Keyframes, Bilder hängen am Titel, nicht an der Bibliothek; sie
     scheiterten sonst nach dem Löschen und füllten «Fehlgeschlagen»).
  2. **Laufende Wiedergaben und Transkodierungen** auf Dateien der Bibliothek beenden.
  3. Datenbank sichern.
  4. Bibliothekszeile und Freigaben löschen; die Fremdschlüssel räumen den Rest.
  5. Dateien im eigenen Cache aufräumen. Für Trickplay und Kapitelbilder gibt es Aufräumläufe
     (`internal/library/sweep.go:22-37`); **für die Bilddateien endgültig gelöschter Titel wurde
     keiner gefunden**, der Bau ergänzt ihn.
- **Messauftrag M-1 vor dem Bau:** Wie lange dauert das Löschen einer Bibliothek mit 10 000 Titeln
  auf CT 190 (die Suchtabellen hängen an Triggern)? Unter fünf Sekunden läuft es direkt in der
  Anfrage. Darüber läuft es als Hintergrundjob `library.delete`; dann braucht die Bibliothek ein
  Merkmal «wird gelöscht», damit sie sofort aus allen Ansichten verschwindet, und die Jobart
  einen Eintrag in `job_types`. Das wäre eine zweite Migration.
- Podcast-Bibliotheken: Auch Abos ohne Datei gehen mit (O-4).

## 10. P-9 Titel löschen: nur aus Slouchcat

Marco hat gewählt (F-3): Der Dialog fragt jedes Mal, **«Nur aus Slouchcat entfernen»** oder
**«Auch die Dateien löschen»**. Dieser Abschnitt beschreibt das Gemeinsame und die erste Wahl,
Abschnitt 11 die zweite.

### 10.1 Heute

- Es gibt **keine Operation**, die einen einzelnen Film, eine Serie, Staffel, Folge, Podcast-Folge
  oder ein Hörbuch löscht, solange die Datei da ist. Vorhanden sind nur: `cleanupItems` (räumt
  Titel **ohne** Datei weg und merkt entfernte Dateien in `ignored_files`,
  `api/openapi.yaml:4653-4690`), `mergeItems` (Duplikate) und `deletePodcast` (Abo beenden).
- Alle diese Wege löschen nur in der Datenbank, und zwar weich: Der Titel bekommt `deleted_at`,
  Fortschritt und Merkliste bleiben an ihm hängen.
- **Die Ignorierliste taugt nicht für «Titel entfernen».** `IgnoreFiles` merkt sich **genaue
  Pfade** und löscht die Zeile in `media_files` (`internal/library/service.go:503-545`). Daraus
  folgt zweierlei:
  - Der Titel **kommt zurück**, sobald eine Datei unter anderem Pfad auftaucht: eine neue Folge im
    Serienordner, ein Upgrade oder eine Umbenennung durch Radarr oder Sonarr.
  - Nimmt man eine ignorierte Datei wieder auf, entsteht ein **neuer** Titel. Das automatische
    Umhängen der Benutzerdaten gilt nur für Filme und Serien derselben Bibliothek
    (`queries/system/relink.sql:24`, `:36`) und nur innert der Frist `relinkGrace`; bei Folgen,
    Staffeln, Hörbüchern und Podcast-Folgen bliebe der Fortschritt am toten Titel hängen.
- Eine Ansicht der ignorierten Dateien gibt es nicht. Die Liste eines Ordners wird geleert, wenn
  sich sein Inhalt ändert (`library/update_root.go:378`) oder der Ordner entfernt wird.

### 10.2 Vorschlag

Mockup «Jobs, Löschen, Duplikate», Thema «Titel löschen».

- **Wo:** runder Knopf «Löschen» auf der Detailseite und «Löschen …» unten links im
  Bleistift-Editor, beides nur für den Server-Administrator. Der Editor ist der Weg für Staffeln
  (Bleistift auf dem Staffelcover, Abschnitt 4).
- **Für welche Arten:** Film, Serie, Staffel, Folge, Hörbuch, Podcast, Podcast-Folge mit Datei.
  Podcast-Folgen, die nur als Verweis aus dem Feed bestehen, bekommen keinen Knopf (O-5).
- **Der Dialog zeigt vor dem Löschen**, was betroffen ist: bei einem Film jede Datei mit ganzem
  Pfad und Grösse, bei Serie, Staffel und Podcast den Ordner, die Zahl der Dateien und die
  Gesamtgrösse.
- **Bestätigung:** bei einer einzelnen Datei der rote Knopf; bei mehreren Dateien muss der Titel
  eingetippt werden, so wie der Dialog ihn zeigt (Vergleich ohne Gross- und Kleinschreibung, ohne
  Leerzeichen am Rand).
- **Läuft der Titel gerade bei jemandem,** nennt der Dialog das Profil; das Löschen beendet die
  Wiedergabe (O-13).
- **«Nur aus Slouchcat entfernen»** arbeitet am **Titel**, nicht an Pfaden (F-13):
  - Der Titel bekommt neben `deleted_at` das neue Merkmal **`items.removed_at`** («vom
    Administrator entfernt»). Seine Zeilen in `media_files` **bleiben**.
  - Er verschwindet für alle Profile aus allen Listen (`visible_items` verlangt schon
    `deleted_at IS NULL`); Downloads auf den Geräten werden entfernt. Fortschritt, Haken und
    Merkliste bleiben am Titel.
  - **Er bleibt entfernt, was immer im Ordner geschieht.** Der Scanner belebt einen Titel mit
    `removed_at` nicht wieder (`RestoreItem`, `queries/system/items.sql:86-97`, und die Belebung
    der Eltern), und er hängt **nichts Neues darunter**: Eine neue Folge im Ordner einer
    entfernten Serie, eine neue Staffel, eine neue Version eines entfernten Films werden dem
    entfernten Titel zugeordnet und bleiben mit ihm unsichtbar.
  - Bei Serie, Staffel und Podcast tragen alle Titel darunter dasselbe Merkmal.
- **Neu «Wartung › Entfernte Titel»:** Liste der entfernten Titel (Titel, Art, Bibliothek, Ordner,
  Datum), je Zeile **«Wieder aufnehmen»**. Das löscht `removed_at` und `deleted_at`; es ist
  **derselbe Titel** mit allen Folgen, Fortschritt, Haken und Merkliste, für jede Art. Dateien,
  die inzwischen fehlen, behandelt der nächste Scan wie immer.
- **Schritt 0 des Baus:** nachstellen, wie der Scanner heute eine neue Datei unter einer weich
  gelöschten Serie behandelt (im Review nicht geklärt), und die Regel oben mit Tests festnageln:
  neue Folge, umbenannte Datei, Upgrade auf andere Auflösung, je für Film und Serie.
- Jedes Löschen schreibt ein Aktivitätsereignis (wer, welcher Titel, welche Wahl, wie viele
  Dateien, wie viele Bytes).

### 10.3 Vertrag und Server

- `getItemDeletion`: `GET /admin/items/{id}/deletion` → Vorschau: `files[]` mit `rootPath`, `path`,
  `sizeBytes`, `deletable`, `reason` **je Datei**; dazu `fileCount`, `totalBytes`,
  `confirmTitle`, `playingProfiles[]`.
- `deleteItem`: `POST /admin/items/{id}/deletion`, Körper `{ "mode": "library" | "files",
  "confirmTitle": "…" }` → Ergebnis (`removedItems`, `deletedFiles`, `deletedBytes`, `leftover[]`).
- `listRemovedItems`, `restoreRemovedItem` für die neue Wartungsansicht.
- **Migration:** Spalte `items.removed_at`.
- Alle `x-visibility: admin`, additiv. Die Logik liegt im Paket `library` (Regel 2 erlaubt dort den
  Zugriff auf `items` und `media_files`) und läuft unter der Scan-Sperre, damit ein gleichzeitiger
  Scan die Dateien nicht wieder einträgt. Läuft der Titel gerade, bricht der Server die Wiedergabe
  und die Transkodierung zuerst ab.

## 11. P-9 Titel löschen: auch die Dateien (ADR 0032)

### 11.1 Heute

- **Slouchcat kann keine Mediendatei löschen**, und das ist Absicht: Die Medienordner sind im
  Container nur lesbar eingebunden (`deploy/compose.yaml:34`, `…:/media:ro`), die Spec verlangt es
  (B-1, `03-plan-v1.md:19`, `:214`), ein Abnahmetest prüft, dass Schreiben nach `/media` scheitert
  (`:539`), und das Protokoll bekräftigt es («Medienwurzeln bleiben read-only (B-1)»,
  `04-entscheidungen.md:49`).
- Das Paket `paths` bietet keine Funktion zum Löschen (`internal/paths/root.go:16-124`), gibt mit
  `paths.Open` aber ein volles `*os.Root` heraus, das `Remove` könnte. Benutzt wird das nirgends:
  Alle `Remove`-Aufrufe des Servers treffen `/config` oder `/cache`. Geschützt ist das heute nur
  durch den `:ro`-Schalter, nicht durch den Linter.
- Konzept 19 Teil A (noch nicht gebaut) öffnet erstmals einen Ordner zum Schreiben, den
  Podcast-Ordner, und hält fest: «Slouchcat löscht nie eine Datei im Podcast-Ordner» (F-4).

### 11.2 Vorschlag

- **Vorgabe bleibt «nur lesbar».** Das mitgelieferte `compose.yaml` bindet Medien weiter mit
  `:ro` ein. Wer Dateien aus Slouchcat löschen will, stellt den Ordner **selbst und bewusst** auf
  schreibbar um; die Installationsanleitung beschreibt es. Der `:ro`-Schalter bleibt damit die
  harte Sicherung: Ohne ihn umzulegen, kann kein Fehler in Slouchcat eine Datei löschen.
- **Der Server prüft, ohne zu schreiben, ob er löschen darf.** Eine Schreibprobe (Datei anlegen
  und wieder löschen) wäre die falsche Sicherung: Sie prüfte den Bibliotheksordner statt des
  Ordners der Datei, und jede Probe weckte die Ordner-Überwachung von Slouchcat, Radarr und
  Sonarr. Stattdessen:
  - **Je Ordner der Bibliothek:** Ist das Dateisystem nur lesbar eingehängt (`statfs`, Merkmal
    `ST_RDONLY`)? Der Dialog «Ordner von …» zeigt «schreibbar» oder «nur lesbar».
  - **Je Datei in der Vorschau:** Darf der Server im **Elternordner dieser Datei** schreiben
    (`unix.Faccessat` mit `W_OK`)? Ordner von Radarr und Sonarr haben oft eigene Besitzer und
    Rechte. Die Vorschau meldet `deletable` und `reason` je Datei.
  - Ist auch nur eine Datei nicht löschbar, ist die Wahl «Auch die Dateien löschen» grau, mit dem
    Grund.
- **Was gelöscht wird,** genau und in dieser Reihenfolge:
  1. die Mediendateien aus `media_files` des Titels, **aller Titel darunter und seiner Extras**
     (alle Versionen und Teile; bei Serie und Staffel die aller Folgen),
  2. die **Bild- und `.nfo`-Dateien, die Slouchcat für diesen Titel selbst eingelesen und
     verzeichnet hat** (`internal/library/local_images.go`, `nfo.go`: `poster.jpg`, `fanart.jpg`,
     `movie.nfo`, `tvshow.nfo` und so weiter),
  3. Begleitdateien neben einer gelöschten Mediendatei, deren Name mit dem **ganzen Dateistamm**
     beginnt, gefolgt von `.` oder `-` (`Film (2024).de.srt`, `Film (2024)-poster.jpg`),
  4. danach den Ordner, **wenn er leer ist**, und leere Elternordner bis unter den
     Bibliotheksordner.
- **Eine Datei, die in `media_files` einem anderen Titel gehört, wird nie gelöscht,** auch wenn
  ihr Name nach Regel 3 passte (flacher Filmordner: `Alien.mkv` und `Alien-Resurrection.mkv`).
  Hängt eine Mediendatei an **zwei** Titeln, bleibt sie liegen, und das Ergebnis nennt sie.
- **Kein rekursives Löschen von Ordnern.** Liegt in einem Film- oder Serienordner etwas, das
  Slouchcat nicht kennt, bleibt es liegen; das Ergebnis nennt die Zahl («3 Dateien bleiben im
  Ordner»).
- **Kein Papierkorb** (O-6): Gelöscht ist gelöscht. Der Dialog sagt es.
- **Reihenfolge:** Wiedergabe und Transkodierung beenden, dann die Dateien, dann die Datenbank.
  Scheitert eine Datei (Rechte, Datei in Gebrauch), bleibt der Titel mit den verbliebenen Dateien
  bestehen und das Ergebnis nennt sie. Eine schon fehlende Datei gilt als gelöscht.
- **Datenbank danach:** Die Zeilen in `media_files` der gelöschten Dateien fallen **sofort** weg,
  damit kein Player eine gelöschte Version anbietet. Hat der Titel keine Datei mehr, wird er
  **weich** gelöscht, genau so, als wäre die Datei von Hand vom NAS verschwunden, nur ohne die
  Wartefrist von 24 Stunden. Er bekommt **kein** `removed_at`: Lädt Radarr denselben Film später
  wieder, erscheint er normal, und wo das Umhängen heute greift (Film und Serie über die
  Anbieter-Nummer), kommen Fortschritt und Haken zurück (O-14).
- **Technik:** `paths` bekommt `Remove(root, rel)` und `RemoveEmptyDir(root, rel)` auf `os.Root`
  (Go 1.27, `server/go.mod`); der Zugriff bleibt damit innerhalb der registrierten Wurzel, `..`
  und Symlinks nach aussen sind ausgeschlossen (Regel 4). Kein `os/exec` (Regel 3).
- **Neue Lint-Regel:** `Remove` und `RemoveAll` auf `*os.Root` sind ausserhalb von `paths`
  verboten (`forbidigo`); die bestehenden `os.Remove` auf `/config` und `/cache` bleiben erlaubt.
  Damit setzt der Linter durch, was bisher nur der `:ro`-Schalter verhinderte.
- **Nur der Server-Administrator** darf löschen; die Operation ist `admin`, die Web-App zeigt den
  Knopf nur dem Hauptprofil des Administrators (`canManage`).
- **Radarr und Sonarr wissen von nichts.** Löscht man in Slouchcat einen Film, den Radarr noch
  überwacht, lädt Radarr ihn womöglich neu. Der Dialog weist darauf hin; Slouchcat steuert Radarr
  und Sonarr nicht (Nicht-Ziel der Spec, Regel 11).

### 11.3 Warum eine ADR

Der Schritt hebt B-1 in einem Punkt auf: Slouchcat **darf** in Medienordner schreiben, wenn der
Betreiber es erlaubt, und zwar ausschliesslich, um auf ausdrücklichen Befehl des Administrators
Dateien zu löschen. **ADR 0032** hält fest: Vorgabe nur lesbar, Prüfung ohne Schreiben statt
einer Einstellung, die genaue Liste dessen, was gelöscht wird, keine rekursiven Löschungen, kein
automatisches Löschen (kein Aufräumjob, keine Regel «nach dem Sehen löschen»), Lint-Regel für
`Remove`. Der Abnahmetest «Schreiben nach `/media` scheitert» bleibt für die Vorgabe
gültig und bekommt ein Gegenstück für den schreibbaren Fall.

## 12. P-10 Duplikate: ganzer Pfad

### 12.1 Heute

- `listDuplicates` liefert je Titel `files[]` mit `path` **relativ zum Bibliotheksordner** und
  `rootId`; die Seite zeigt den relativen Pfad und die Grösse
  (`features/admin/maintenance/DuplicatesTab.tsx:135-157`). Der Ordner davor fehlt, und liegen
  zwei Titel in verschiedenen Ordnern mit gleichem Aufbau, sehen die Pfade gleich aus.
- Bei **Serien und Podcasts steht «Keine Datei»**, weil die Dateien an den Folgen hängen und der
  Server nur die Dateien des Titels selbst mitgibt (`internal/admin/maintenance.go:349-353`).

### 12.2 Vorschlag

Mockup «Jobs, Löschen, Duplikate», Thema «Duplikate mit Pfad».

- `AdminMediaFile` bekommt `rootPath` (additiv); die Seite zeigt **`rootPath` + `path`** als
  ganzen Pfad, in Festbreitenschrift, umbrechend, dazu den Namen der Bibliothek.
- Für Serien und Podcasts bekommt der Eintrag `folders[]` (`rootPath`, `path`, `fileCount`,
  `sizeBytes`): der gemeinsame Ordner der Folgendateien je Bibliotheksordner. Die Seite zeigt
  «96 Dateien · 212 GB» und den Ordner.
- Derselbe ganze Pfad erscheint im Lösch-Dialog (Abschnitt 10) und auf der Verwaltungsseite eines
  Titels, die `AdminMediaFile` ebenfalls nutzt.
- **Der Pfad ist der im Container** (`/media/…`), nicht der auf dem NAS; Slouchcat kennt nur
  jenen. Darum steht immer der Name der Bibliothek daneben, und ein Satz auf der Seite sagt es.

## 13. P-11 Die Frage «bleiben oder verwerfen» ohne Speichern-Knopf

### 13.1 Befund

- Der Dialog «Ordner von …» speichert **jede Aktion sofort** (Entfernen, Pfad ändern, Hinzufügen);
  er setzt kein «ungespeichert» (`features/admin/libraries/LibraryFoldersDialog.tsx:102-140`).
- Die Rückfrage beim Verlassen kommt auf der Seite «Bibliotheken» aus einer einzigen Quelle:
  `useUnsavedChanges(draft.dirty)` (`features/admin/libraries/LibrariesPage.tsx:89`), und `draft`
  ist der **Entwurf von «Bibliothek hinzufügen»**
  (`features/admin/libraries/AddLibraryDialog.tsx:61-100`).
- Dieser Entwurf **überlebt das Schliessen des Dialogs** mit Absicht (S-13; «Abbrechen» und ✕
  rufen nur `create.reset()`), **die Seite zeigt ihn aber nirgends**. Speichern oder verwerfen
  kann man ihn nur, indem man «Bibliothek hinzufügen» nochmals öffnet.
- `dirty` wird schon wahr, wenn man im Hinzufügen-Dialog einen Namen tippt (etwa «test2») oder
  eine Ordnerkarte hinzufügt und eine andere entfernt (`untouchedFolders`,
  `AddLibraryDialog.tsx:47-58`: die verbleibende Karte hat dann die Art `series` statt `movie`).

**Möglicher Ablauf, nicht gesichert:** Ein erfolgreiches Anlegen setzt den Entwurf zurück
(`AddLibraryDialog.tsx:133-134`); vom Anlegen von «test2» selbst kann er also nicht stammen. Es
braucht einen **zweiten, abgebrochenen** Besuch im Dialog «Bibliothek hinzufügen» in derselben
Sitzung der Seite (etwas getippt oder eine Ordnerkarte verändert, dann geschlossen). Danach
fragt die Seite beim Verlassen nach, ohne dass ein Speichern-Knopf zu sehen ist, gleich was man
dazwischen in «Ordner von test2» getan hat. **Das ist aus dem Code gelesen, nicht nachgestellt**
(Sicherheit mittel). Eine andere Quelle für «ungespeichert» gibt es auf dieser Seite nicht.
**Schritt 0 des Baus** stellt Marcos Klickfolge auf CT 190 und in einem e2e-Test nach. Passt die
Erklärung nicht, wird die tatsächliche Ursache gesucht und behoben und dieses Papier
nachgeführt; der Vorschlag unten ist dann nur noch eine Verbesserung am Rand.

### 13.2 Vorschlag

Mockup «Jobs, Löschen, Duplikate», Thema «Entwurf-Band».

- Liegt ein Entwurf vor und ist der Dialog zu, zeigt die Seite ein **Band**: «Der Entwurf einer
  neuen Bibliothek ist noch nicht gespeichert», mit **«Weiter bearbeiten»** und **«Entwurf
  verwerfen»**.
- «Abbrechen» im Hinzufügen-Dialog fragt neu «Entwurf behalten oder verwerfen?», wenn etwas
  eingetragen ist; ✕ und Escape behalten ihn wie heute.
- Eine einzelne Ordnerkarte **ohne Pfad** gilt als unberührt, gleich welche Art sie trägt.
- Nur Web-App.

## 14. P-12 «Schlüssel der Anbieter» aufräumen, P-13 Umbenennung

### 14.1 Heute

Die Seite «Integrationen» zeigt die Schlüssel als erstes von sechs Bändern
(`features/admin/IntegrationsPage.tsx:112-138`), **sieben Zeilen** in der Reihenfolge TMDB, TVDB,
TVDB-PIN, YouTube, Fanart.tv, Fanart.tv persönlich, Trakt (`features/admin/integrations/shared.tsx:29-37`).
Was unruhig wirkt:

- Zusammengehöriges ist getrennt: TVDB-Schlüssel und TVDB-PIN, die zwei Fanart-Schlüssel, je mit
  eigenem «Speichern» und eigenem Stand.
- Die Reihenfolge der Zeilen weicht vom Einleitungstext ab (YouTube steht zwischen TVDB und Fanart).
- Der Hinweis «In deploy/.env gesetzt» steht doppelt (Abzeichen und Feldtext, `KeyRow.tsx:72`, `:97`).
- Englisch im deutschen Text («API Key gesetzt», «API Read Access Token»), uneinheitliche Namen
  («TMDB-Token» neben «-Schlüssel»).
- Die Erklärtexte sind sehr verschieden lang; nur Trakt hat einen zweiten Link.
- Ob ein Schlüssel **gilt**, sagt die Seite nicht. Eine Prüfung gibt es nur im
  Einrichtungsassistenten und nur für TMDB (`checkSetupTmdbKey`).

### 14.2 Vorschlag (F-11)

Mockup «Schlüssel».

- **Fünf Anbieter statt sieben Zeilen**, je eine Karte, in dieser Reihenfolge: TMDB, TVDB,
  Fanart.tv, YouTube, Trakt.
- **Zugeklappt** zeigt jede Karte: Name, einen Satz zum Zweck, den Stand («Gespeichert», «In
  deploy/.env gesetzt», «Nicht gesetzt») und einen Knopf («Einrichten», «Bearbeiten», «Ansehen»).
- **Aufgeklappt:** die Felder des Anbieters untereinander (TVDB: Schlüssel und Abo-PIN;
  Fanart.tv: Projekt-Schlüssel und persönlicher Schlüssel), ein Satz Hilfe mit **einem** Link
  «Schlüssel bei … holen», darunter **ein** «Speichern» für die Karte, «Prüfen» und «Löschen …».
- **«Prüfen»** fragt den Anbieter mit dem gespeicherten Wert und meldet «gilt» oder den Fehler des
  Anbieters im Klartext.
- Kommt der Wert aus `deploy/.env`, sind die Felder gesperrt und der Hinweis steht einmal;
  «Prüfen» bleibt möglich.
- Alle Texte deutsch und gleich gebaut; das Band zählt «3 von 5 Anbietern eingerichtet».

### 14.3 Vertrag und Server

- `updateAdminSettings` nimmt schon mehrere Schlüssel in einem Aufruf; die Karte nutzt das.
- **Neue Operation `checkProviderKey`**: `POST /admin/settings/keys/{provider}/check` mit
  `provider` aus `tmdb`, `tvdb`, `fanart`, `youtube`, `trakt`; Antwort `{ ok, message }`.
  `x-visibility: admin`, additiv. Je Anbieter ein billiger Aufruf über den vorhandenen Client; die
  Antwort und das Log enthalten nie den Schlüssel (Regel 10).
- Der Schlüssel für Google Books aus Konzept 19 D3 (PR #139) steht auf der Seite «Hörbücher:
  Quellen»; ob er hierher umzieht, ist O-7.

### 14.4 P-13 «Meine Wiedergabe» heisst «Einstellungen»

- `nav.playback` und `settings.playback.title` werden «Einstellungen» (englisch «Settings»),
  `web/src/locales/de.json:21`, `:4461`. Die Route `/settings/playback` und das Symbol bleiben.
- Betroffen sind Texte und Tests (`e2e/playback-settings.spec.ts`, `e2e/sidenav.spec.ts:102`,
  `ui/SideNav.test.tsx`, `PlaybackSettingsPage.test.tsx`, `ProfilePlayback.test.tsx`,
  `SideNav.stories.tsx`).
- Verwechslungsgefahr besteht nicht: Weder Kopfzeile noch Seitenleiste haben heute einen Eintrag
  «Einstellungen»; das Feld im Player heisst weiter «Einstellungen · Qualität» und steht nur dort.
- Die Seite hat danach zwei Felder (Sprachen und Untertitel, Serienmerker). Die Seite
  «Benachrichtigungskanäle» (`/settings/notifications`) hat heute keinen Eintrag in der
  Seitenleiste; ein Verweis von «Einstellungen» dorthin wäre naheliegend (O-8).

## 15. P-14 Neue Seite «Gesehen»

### 15.1 Heute

- Die Seitenleiste hat fünf Einträge auf der obersten Ebene: Start, Suche, Merkliste, Radio,
  Downloads (`app/AppLayout.tsx:234-265`).
- Eine Liste «alles Gesehene» gibt es nicht. Am nächsten kommt die Bibliotheksseite mit
  `status=watched&sort=played`, aber nur je Bibliothek.
- Ein dauerhaftes «gesehen» fehlt in den Daten (Abschnitt 6.1).

### 15.2 Entscheid (F-4, F-5)

- Die Seite heisst **«Gesehen»** und steht in der Seitenleiste **unter «Radio»**, für jedes Profil,
  mit eigener Liste je Profil.
- **Was erscheint:**
  - Film, Hörbuch: sobald einmal fertig gesehen oder gehört (90 %) oder der Haken von Hand gesetzt
    ist.
  - **Serie, Podcast: ein Eintrag für die ganze Serie**, sobald mindestens eine Folge fertig ist.
    Keine Staffeln, keine einzelnen Folgen.
  - Radio und Extras kommen nicht vor. Angefangenes ohne fertigen Titel steht nur in
    «Weiterschauen».
- Ein Titel **bleibt** in der Liste, auch wenn man ihn später nochmals anfängt (`watched_at`,
  Abschnitt 6.3). Er verschwindet, wenn man den Haken entfernt, oder solange der Titel gelöscht
  oder für das Profil nicht sichtbar ist.

### 15.3 Die Seite

Mockup «Gesehen».

- Überschrift «Gesehen», Anzahl der Titel.
- Filter nach Medienart: Alle, Filme, Serien, Hörbücher, Podcasts.
- Sortierung: **zuletzt gesehen** (Vorgabe) oder Titel A–Z.
- Raster aus `PosterCard` wie überall. Unterzeile: bei Film und Hörbuch «gesehen am 8. Nov.»
  beziehungsweise «gehört am …»; bei Serie und Podcast «19 von 96 Folgen · zuletzt 9. Nov.» mit
  Balken.
- Ein Klick öffnet die Detailseite, bei Serien die Serienseite.
- Auf jeder Karte der Bleistift (nur Server-Administrator) und der Haken in seinem Zustand
  (F-12): **voll** bei Film, Hörbuch und ganz gesehener Serie, **halb** bei einer Serie oder einem
  Podcast mit «19 von 96 Folgen». Ein Klick auf den vollen Haken entfernt ihn (bei Serie und
  Podcast mit Rückfrage), der Titel verlässt die Liste. Ein Klick auf den halben Haken fragt
  «Alle Folgen als gesehen markieren» oder «Alle Haken entfernen».
- Leere Liste: «Noch nichts gesehen. Was du zu Ende schaust oder hörst, erscheint hier.»
- Offline zeigt die Seite den üblichen Hinweis; die Liste kommt nur vom Server.

### 15.4 Vertrag und Server

- **Neue Operation `listWatched`**: `GET /watched`, Parameter `content`, `sort` (`watched`,
  `title`), `offset`, `limit` (höchstens 100); `x-visibility: scoped`. Antwort: `items` (Karten wie
  in den anderen Listen, dazu `lastWatchedAt` und für Serie und Podcast `watchedCount` und
  `episodeCount`) und `total`.
- Abfrage in `queries/scoped/` über `visible_items` (Regel 2): Titel mit `watched_at`, vereinigt
  mit Serien und Podcasts, die mindestens eine sichtbare Folge mit `watched_at` haben;
  `lastWatchedAt` ist bei ihnen das jüngste Datum der Folgen. Gezählt werden nur Folgen, die das
  Profil sehen darf.
- **Messauftrag M-2:** Antwortzeit von `listWatched` mit 5 000 gesehenen Folgen auf CT 190; Ziel
  unter 150 ms, Messwert nach `docs/perf/`.
- Web-App: Route `/watched`, Seite `WatchedPage`, Eintrag in `AppLayout` nach «Radio»,
  Tastaturtest (Regel 8), `e2e/sidenav.spec.ts:70` nennt den neuen Eintrag.

## 16. Was sich gegenüber früheren Entscheiden ändert

| Früherer Entscheid | Neu | Abschnitt |
|---|---|---|
| **B-1:** Medienordner sind nur lesbar; Slouchcat schreibt nie hinein (Spec, Plan, Protokoll Zeile 49). | Vorgabe bleibt nur lesbar. Stellt der Betreiber einen Ordner auf schreibbar, darf der Server-Administrator dort Dateien eines Titels löschen. Nichts löscht automatisch. ADR 0032. | 11 |
| **Konzept 19, F-4:** «Slouchcat löscht nie eine Datei im Podcast-Ordner.» | Gilt weiter für alles Automatische. Der Administrator kann einen Podcast oder eine Folge von Hand samt Datei löschen. | 11 |
| **E-8:** Favoriten für Serien und Podcasts mit Benachrichtigung; O-3: Auto-Download für favorisierte Podcasts. | Favorit entfällt; die Merkliste löst beides aus. Radio-Favoriten bleiben (OB-6). | 3 |
| **Konzept 08, O-3:** Nur User-Empfehlungen und Merkliste haben eine eigene Seite. | Jede Reihe mit sinnvoller Liste hat eine; der Reihentitel ist der Link. | 8 |
| **AN-10:** Neuerer Fortschritt gewinnt; ein Ereignis unter 90 % macht einen Titel wieder ungesehen. | Gilt weiter für Stelle und «Weiterschauen». Der **Haken** bleibt neu stehen (`watched_at`), bis man ihn entfernt. | 6 |
| **Konzept 17:** Ein Klick auf eine Folge öffnet die Folgenseite; der Kreis ist eine Anzeige. | Der Klick startet die Folge, «Details» öffnet die Seite, der Kreis ist ein Knopf. | 5, 6 |
| **Vertrag 1.22.0:** `ItemRef.watched` und `watchedCount` rechnen mit `completed`. | Beide rechnen mit `watched_at`; der Zustand der Bibliothek ebenso («gesehen» schlägt «angefangen»). | 6 |
| **B-9:** Entfernte Dateien kommen auf die Ignorierliste. | Bleibt für das Aufräumen. «Titel entfernen» nutzt neu ein Merkmal am Titel (`removed_at`). | 10 |
| **Konzept 15, «Nicht gebaut»:** Bibliothek löschen. | Wird gebaut. | 9 |
| **Konzept 13:** Seite «Meine Wiedergabe». | Heisst «Einstellungen». | 14.4 |

## 17. Fragen an Marco und seine Antworten (10. und 11. November 2026)

| Nr. | Frage | Empfehlung | **Antwort** |
|---|---|---|---|
| F-1 | Was löst «neue Folge» und den Podcast-Auto-Download aus, wenn der Favorit verschwindet? | Merkliste | **Merkliste übernimmt** |
| F-2 | Bleibt das Herz beim Radio («Meine Sender»)? | Ja | **Bleibt** |
| F-3 | Was tut «Löschen» bei Titeln? | Wahl im Dialog | **Wahl im Dialog: nur aus Slouchcat oder auch die Dateien** |
| F-4 | Was zählt als «gesehen»? | Nur fertig Gesehenes | **Nur fertig Gesehenes; Serie ab einer fertigen Folge** |
| F-5 | Name der neuen Kategorie? | Gesehen | **Gesehen** |
| F-6 | Wie erreicht man die Folgenseite, wenn der Klick abspielt? | Ganze Zeile spielt, Knopf «Details» | **Ganze Zeile spielt, Knopf «Details»** |
| F-7 | Wo erscheint die Frage «weiterschauen oder neu starten»? | Überall bei Direktstart | **Überall bei Direktstart** |
| F-8 | Wo markiert man von Hand? | Folgen und Detailseiten | **«Ich möchte wie bei den gesehenen Titeln einfach den Haken setzen»** (eigener Wortlaut; mit F-10 geklärt) |
| F-9 | Staffeln: A, B oder C? | A | **A · Reihe mit Pfeilen** |
| F-10 | Wo steht der Haken? | Überall auf Karten | **Überall auf Karten** |
| F-11 | Schlüssel der Anbieter wie im Mockup? | Ja | **Ja, wie im Mockup (zugeklappt, mit «Prüfen»)** |
| F-12 | Haken bei teilweise gesehenen Serien und Podcasts? (nach dem Review) | Halber Haken, Klick fragt | **Halber Haken, Klick fragt** |
| F-13 | Entfernte Serie, Sonarr legt eine neue Folge ab? (nach dem Review) | Bleibt entfernt | **Bleibt entfernt, bis man sie wieder aufnimmt** |

## 18. Offene Punkte

Marco hat am 12. November 2026 alle vierzehn Punkte einzeln beantwortet (Spalte «Antwort»); vier
weichen von der Empfehlung ab (O-9, O-10, O-13, O-14). Wo der Text oben etwas anderes sagt, gilt
die Antwort.

| Nr. | Punkt | Empfehlung | **Antwort (12. November 2026)** |
|---|---|---|---|
| O-1 | Leerer Kreis auf Posterkarten mit Maus: immer sichtbar oder erst beim Zeigen? | Erst beim Zeigen und im Fokus; der gesetzte Haken immer. Sonst trägt jede Karte dauernd bis zu drei Knöpfe (Bleistift, Merken, Haken). | **Erst beim Zeigen** (Empfehlung) |
| O-2 | Schreibt der Haken von Hand ein Aktivitätsereignis «fertig gesehen»? | Nein. Das Protokoll der Verwaltung soll echte Wiedergaben zeigen. | **Nein** (Empfehlung) |
| O-3 | Bekommt «Sammlungen» eine Übersichtsseite `/collections` als Ziel des Reihentitels? | Ja, einfaches Raster aller sichtbaren Sammlungen. | **Ja, einfaches Raster** (Empfehlung) |
| O-4 | Bibliothek löschen: Gehen bei einer Podcast-Bibliothek auch die Abos ohne Datei mit? | Ja, die ganze Bibliothek ist weg. | **Ja, alles weg** (Empfehlung) |
| O-5 | Podcast-Folgen, die nur als Verweis aus dem Feed bestehen: einzeln löschbar? | Nein. Der Feed brächte sie beim nächsten Abruf zurück; dafür bräuchte es eine eigene Ausblendliste. | **Nein** (Empfehlung) |
| O-6 | Dateien löschen: Papierkorb-Ordner statt endgültig? | Nein, endgültig. Ein Papierkorb bräuchte Platz, eine Frist und eine eigene Ansicht. | **Endgültig** (Empfehlung) |
| O-7 | Zieht der Google-Books-Schlüssel (Konzept 19 D3) als sechste Karte auf die Seite «Schlüssel der Anbieter»? | Ja (PR #139 ist seit dem 10. November 2026 gemergt, Release 1.26.0); die Seite «Hörbücher: Quellen» verweist dann dorthin. | **Ja, sechste Karte** (Empfehlung) |
| O-8 | Bekommt die Seite «Einstellungen» einen Verweis auf «Benachrichtigungskanäle»? | Ja, als drittes Feld mit Link. | **Ja, drittes Feld** (Empfehlung) |
| O-9 | Gilt die Nachfrage «weiter oder von vorn» auch für Bände einer Hörbuch-Reihe? | Ja (Text «Weiterhören ab …»); es ist derselbe Direktstart. | **Nein: Bände spielen immer ab der gemerkten Stelle** (gegen die Empfehlung) |
| O-10 | Fragt «Alle erneut versuchen» nach? | Nein, es ist gefahrlos. Nur «Alle löschen» fragt. | **Mit Rückfrage**: auch «Alle erneut versuchen» fragt mit der Anzahl nach (gegen die Empfehlung) |
| O-11 | Löscht «Auch die Dateien löschen» Begleitdateien mit? | Ja, aber nur nach der genauen Regel aus 11.2: von Slouchcat verzeichnete Bild- und `.nfo`-Dateien und Dateien mit dem ganzen Dateistamm; nie eine Datei eines anderen Titels. | **Ja, nach genauer Regel** (Empfehlung) |
| O-12 | Soll die Serie auf der Merkliste bleiben, wenn man alle Folgen gesehen hat? | Ja. Sie meldet weiter neue Folgen, bis man sie von Hand entfernt. | **Bleibt** (Empfehlung) |
| O-13 | Löschen, während der Titel bei einem anderen Profil läuft: ablehnen oder Wiedergabe beenden? | Beenden. Der Dialog nennt das Profil vorher; der Administrator entscheidet. | **Löschen ablehnen**: solange ein Profil den Titel abspielt, antwortet der Server mit 409, der Dialog nennt das Profil (gegen die Empfehlung) |
| O-14 | «Auch die Dateien löschen»: Benutzerdaten behalten für den Fall, dass der Titel wiederkommt? | Ja, weich löschen wie bei einer verschwundenen Datei. Kostet nichts und rettet Fortschritt, wenn Radarr den Film neu lädt. | **Nein, alles weg**: Sind alle Dateien gelöscht, wird der Titel endgültig gelöscht, samt Fortschritt, Haken und Merkliste (gegen die Empfehlung) |

## 19. Bauplan in Stufen

| Stufe | Inhalt | Vertrag / Migration | Aufwand (grob) |
|---|---|---|---|
| **1 · Serienseite und Kleines** | P-1 Pfeile, P-3 Bleistift der Staffel, P-4 Folgenklick mit Nachfrage, P-11 Entwurf, P-13 Umbenennung | keiner / keine (Patch-Release) | ≈ 4 Tage |
| **2 · Haken und «Gesehen»** | P-5 Haken zum Anklicken in drei Zuständen, `watched_at`, P-14 Seite «Gesehen» | additiv (`setItemWatched`, `listWatched`, `watchedAt`), **geänderte Bedeutung** von `ItemRef.watched` und `watchedCount` / eine (`progress.watched_at`) | ≈ 8 Tage |
| **3 · Startseite und Favorit** | P-7 Reihentitel, Listenseite, Spotlight-Titel, Bleistifte auf allen Listen; P-2 Favorit entfernen | **brechend** (Favoriten-Operationen fallen), additiv (`listHomeRow`, `listCollections`) / eine (Favoriten in die Merkliste, `watchlist.follow_since`, Tabelle weg) | ≈ 7 Tage |
| **4 · Verwaltung** | P-6 Jobs, P-10 Duplikate mit Pfad, P-12 Schlüssel mit «Prüfen», P-15 Kontinent-Kacheln | additiv / keine | ≈ 4,5 Tage |
| **5 · Löschen** | P-8 Bibliothek löschen, P-9 Titel löschen in beiden Arten, «Entfernte Titel», ADR 0032, Lint-Regel, Doku | additiv / eine bis zwei (`profile_libraries` mit Fremdschlüssel, `items.removed_at`; bei M-1 über fünf Sekunden dazu Merkmal «wird gelöscht» und Jobart) | ≈ 10 Tage |

- **Abhängigkeiten:**
  - Stufe 1 baut die Folgenzeile mit dem Kreis **als Anzeige** an seinem neuen Platz ausserhalb
    des Links; zum Knopf wird er in Stufe 2. Die Tab-Reihenfolge von Stufe 1 ist Zeile, «Details».
  - Stufe 3 nutzt den Haken aus Stufe 2 auf der Listenseite. Der Knopf «Gesehen» der Detailseite
    (Stufe 2) rückt auf den Platz des Favoriten, der erst in Stufe 3 fällt; bis dahin stehen
    beide nebeneinander.
  - Stufe 5 braucht `rootPath` aus Stufe 4 (ganzer Pfad im Lösch-Dialog) und den Bleistift der
    Staffel aus Stufe 1 (Weg zum Löschen einer Staffel).
- Jeder Planschritt ist ein eigener Commit mit DoD-Belegen der Teststufe 1; vor jedem Pull Request
  `task ci` grün (`CLAUDE.md`, «Arbeitsweise»). Jede neue Seite hat ihren Tastaturtest (Regel 8):
  Listenseite, «Gesehen», «Sammlungen», «Entfernte Titel»; dazu die umgebaute Folgenzeile und
  die Dialoge. Jede neue Operation hat `x-visibility` und einen Eintrag im Vertragstest (Regel 7);
  die Einträge der entfallenen Favoriten-Operationen verschwinden.
- **Storybook und Bildvergleiche** für jedes geänderte Bauteil in `web/src/ui`: `RowList` mit
  festen Pfeilen, `EpisodeListItem`, `EpisodeRow`, `PosterCard` mit Haken in drei Zuständen,
  `HeroCarousel` mit Überschrift, das neue `ResumeChoice`; dazu `VolumeRow` und die
  Anbieter-Karten, soweit sie Stories haben.
- Alle neuen Texte in `web/src/locales/{de,en}.json` (Regel 9); Migrationen nach Vorlage mit
  eingefrorenem DB-Stand und Migrationstest (Regel 5).
- Schritt 0 je Stufe, wo das Papier ihn nennt: Stufe 1 (Marcos Klickfolge zu P-11), Stufe 2
  (Fundstellen von `completed`), Stufe 5 (Scanner unter einem entfernten Titel).
- Messungen: M-1 vor Stufe 5, M-2 in Stufe 2, M-3 in Stufe 3.
- Abnahme je Stufe auf CT 190; jede Stufe erscheint als eigenes Release. Für die Abnahme von
  Stufe 5 bindet Marco einen **Test**ordner schreibbar ein, nicht die echten Medien.
- Reihenfolge gegenüber Konzept 19 (Teil A Podcasts, Teil B Radio noch offen): Marcos Wahl.
  Empfehlung: Stufe 1 sofort (klein, kein Vertrag), der Rest nach PR #139.

## 20. Was nicht dazugehört

- Automatisches Löschen von Dateien (nach dem Sehen, nach Frist, bei Platzmangel).
- Löschen in Radarr, Sonarr oder Audiobookshelf anstossen (Nicht-Ziel der Spec).
- Ein Papierkorb für gelöschte Dateien.
- Ein lückenloser Verlauf jeder Wiedergabe mit Datum und Gerät; «Gesehen» zeigt je Titel nur das
  letzte Mal.
- «Gesehen» für Radio.
- Haken setzen ohne Verbindung zum Server.
- Die Varianten B und C der Staffeln.
- Favoriten beim Radio abschaffen.
- Alles aus den Non-Goals der Spec (Regel 11).

## 21. Kritiker-Review von Fassung 1 (11. November 2026)

Urteil **REVISE**: 1 Blocker, 13 Major, 13 Minor. Geprüft wurde gegen den Code, nichts wurde
ausgeführt. Alle Befunde sind in Fassung 2 eingearbeitet; keiner widerspricht F-1 bis F-11. Zwei
Befunde brauchten Marcos Entscheid (F-12, F-13).

| Befund | Kern | Eingearbeitet in |
|---|---|---|
| **B1** | Bibliothek löschen liess `profile_libraries` stehen (kein Fremdschlüssel); die nächste Bibliothek mit derselben Nummer wäre für ein eingeschränktes Profil sichtbar geworden. | 9.1, 9.3 |
| M1 | Verspätete und ältere Offline-Ereignisse hätten `watched_at` nie gesetzt. | 6.3 |
| M2 | Zusammenführen und Umhängen hätten `watched_at` verloren. | 6.3 |
| M3 | Viele Folgen auf einmal markiert: gleiche `updated_at`, falsche «nächste Folge» und falscher Auto-Download. | 6.4 |
| M4 | `ItemRef.watched` gibt es schon; der Haken einer teilweise gesehenen Serie hatte zwei Bedeutungen. | 6.2, 6.4, 15.3, F-12 |
| M5 | Liste der Leser von `completed` unvollständig; Zustand der Bibliothek nicht mehr eindeutig. | 6.3 (Tabelle) |
| M6 | «Nur entfernen» über die Ignorierliste: Titel kommt bei neuem Pfad zurück, «Wieder aufnehmen» verliert Benutzerdaten. | 10.1, 10.2, F-13 |
| M7 | Was «auch die Dateien» löscht, war ungenau und konnte Dateien anderer Titel treffen; Extras fehlten. | 11.2 |
| M8 | Schreibprobe am falschen Ort und mit Nebenwirkung; `paths.Open` gibt ein löschfähiges `*os.Root` heraus. | 11.1, 11.2 |
| M9 | Bibliothek löschen: Jobs je Titel, laufende Wiedergaben, Rotation der Sicherung, Kosten der Wiederherstellung. | 9.2, 9.3 |
| M10 | Favoriten in die Merkliste: alte Podcast-Folgen wären als «neu» nachgeladen worden. | 3.3 (`follow_since`) |
| M11 | Die vier Reihen lassen sich nicht gleich blättern. | 8.3 |
| M12 | Sammel-«Erneut» scheitert an doppelten Idempotenzschlüsseln; `listJobRuns` ohne Versatz. | 7.3 |
| M13 | Nachfrage im Spotlight geht für Serien nicht ohne Vertragsänderung. | 5.2 |
| m1–m13 | Falsche Pfade und Zeilen, Hypothese zu P-11 abgeschwächt, `watched:false` und fehlende Dauer, Adresse von `deleteLibrary`, Wortlaut «Löschen» bei Jobs, vergessene Tests und Stories, Abhängigkeiten der Stufen, Pfeilwege der Startseite, Container-Pfad, Vergleich des Titels, leere Sammlungen. | 2.1, 6.4, 7, 8.3, 9, 10, 12, 13, 19 |

**Nicht geklärt, darum als Schritt 0 oder Bauauftrag im Papier:** wie der Scanner eine neue Datei
unter einem weich gelöschten Titel behandelt (10.2); ob ein Aufräumlauf die Bilddateien endgültig
gelöschter Titel entfernt (9.3); was `/play/{seriesId}` im Player tut (5.2, darum keine Nachfrage
für Serien im Spotlight).

## 22. Nachtrag P-15: Kontinent-Kacheln im Radio-Angebot (12. November 2026)

Gelesen gegen `main` **`6791dbda`**. Marco hat ein Bildschirmfoto des Bandes «Länder» geschickt
(«Verwaltung › Radio › Angebot», gebaut mit Konzept 19 Teil B) und will es angepasst haben.

### 22.1 Heute

- Die sechs Kontinente sind je ein `AriaButton` mit `block`
  (`features/admin/radio/OfferTab.tsx`, Schleife über `RADIO_CONTINENTS`), in einem Raster
  (`.continents`, `features/admin/radio/Radio.module.css`). Im Knopf stehen **zwei Zeilen**:
  der Name und darunter «51 Länder · 26.395 Sender · 1 gewählt» (`.continent`, `.continentSub`).
- Der Knopf ist für **eine** Zeile gebaut (`ui/Button.module.css`): Pillenform
  (`border-radius: var(--sc-radius-full)`), `line-height: 1`, `white-space: nowrap`, Inhalt
  **zentriert** (`justify-content: center`), Höhe eine Bedienhöhe.
- Daraus folgt, was das Bildschirmfoto zeigt: Die zwei Zeilen kleben aneinander und füllen die
  Pille bis an den Rand; weil der Block als Ganzes zentriert wird, beginnt der Text in jedem
  Knopf an einer anderen Stelle (bei «Europa» weit links, bei «Asien» in der Mitte); Name und
  Zahlen sind gleich fett und gleich gross; dass der Knopf aufklappt, sieht man ihm nicht an.
- Über den Knöpfen steht ein langer Erklärsatz («Alle Länder nach Kontinent. Ein Kontinent öffnet
  seine Länder; nochmals drücken oder Escape schliesst sie.»).

### 22.2 Vorschlag

Mockup «Radio: Länder» (Anhang A; oben umschaltbar «Neu» und «Heute»).

- **Kacheln statt Pillen:** rechteckig mit der Rundung der Karten, die Höhe wächst mit dem Inhalt
  (mindestens 68 px), Inhalt **linksbündig**.
- **Zwei klar getrennte Zeilen:** der Name gross und fett, darunter «51 Länder · 26.395 Sender» in
  der Farbe für Nebentext.
- **«1 gewählt» als Abzeichen rechts** in der Akzentfarbe, nur wenn die Zahl grösser als null
  ist. So sieht man auf einen Blick, in welchen Kontinenten schon etwas gewählt ist.
- **Pfeil ganz rechts**, der zeigt, dass die Kachel aufklappt; bei der offenen Kachel dreht er.
- **Offene Kachel** mit Akzentrand und getöntem Grund (nicht mehr vollflächig in der
  Akzentfarbe); ihre Länder stehen direkt darunter in einem Rahmen derselben Farbe, mit der
  Überschrift «Länder in Europa» und den zwei Knöpfen «Alle wählen» und «Alle abwählen».
- **Überschrift «Nach Kontinent»** statt des Erklärsatzes. Der Satz bleibt als Beschreibung für
  Screenreader (`aria-describedby`), sichtbar ist er nicht mehr.
- Raster: `auto-fill` mit Mindestbreite 250 px, also drei Kacheln je Zeile am Schreibtisch, eine
  am Handy. Alle Kacheln einer Zeile sind gleich hoch.

### 22.3 Bau

- Neues Bauteil **`DisclosureTile`** in `web/src/ui` (ein `<button>` mit `aria-expanded` und
  `aria-controls`, Plätze für Titel, Unterzeile, Abzeichen), mit Story und Bildvergleich. Der
  `AriaButton` bleibt, wie er ist; zweizeilige Inhalte gehören nicht in ihn.
- `OfferTab.tsx` nutzt die Kachel; Verhalten unverändert (ein Kontinent offen, nochmals drücken
  oder Escape schliesst, Fokus zurück auf die Kachel). Die Tastaturtests des Angebots laufen
  weiter; der Zugriff über den Namen des Knopfs bleibt.
- Farben und Abstände nur aus Tokens (Regel 9). Texte: neuer Schlüssel für die Überschrift,
  «gewählt» wandert aus der Unterzeile ins Abzeichen (`admin.radio.offer.continentChosen` bleibt).
- Nur Web-App; kein Vertrag, keine Migration. Aufwand etwa ein halber Tag, in Stufe 4.
- **Dieselbe Falle anderswo:** Vor dem Bau wird gesucht, wo sonst ein `AriaButton` oder `Button`
  mehrzeiligen Inhalt trägt (`flex-direction: column` innerhalb eines Knopfs); Fundstellen
  bekommen dieselbe Kachel.

---

## Anhang A. Mockups

Klickbare Design-Leinwand **«Konzept 20 Bedienung und Gesehen»**:
<https://claude.ai/artifact/CTuETGaW8MEESdfSMpJM19> (privat, nur für Marco). Kopie der Quellen im
Repository unter `docs/planning/mockups/20-bedienung/`. Graue und farbige Flächen stehen für echte
Bilder; alle Titel sind erfunden.

| Bild | Datei | Zeigt |
|---|---|---|
| Serienseite | `Main.dc.html` | Staffeln in den Varianten A, B, C (oben umschaltbar); Bleistift und Haken auf den Staffelcovern; Folgenzeile mit Play-Knopf, «Details» und Kreis; Nachfrage bei Folge 4 |
| Startseite | `Start.dc.html` | Reihentitel als Link, Überschrift «Spotlight», klickbarer Filmtitel; Seitenleiste mit «Gesehen» und «Einstellungen»; Bleistift und Haken auf den Karten |
| Listenseite | `Liste.dc.html` | Die eine Seite hinter «Weiterschauen», «Neu hinzugefügt», «Kürzlich veröffentlicht», «Spotlight» |
| Gesehen | `Gesehen.dc.html` | Die neue Seite mit Filter, Sortierung, Serien als ein Eintrag, halber Haken mit Auswahl, Rückfrage beim Entfernen |
| Schlüssel | `Schluessel.dc.html` | Fünf Anbieter-Karten, auf- und zuklappbar, «Prüfen» |
| Jobs, Löschen, Duplikate | `Verwaltung.dc.html` | Fünf Themen: Sammelaktionen der Jobs, Titel löschen (Ordner schreibbar oder nur lesbar), Bibliothek löschen, Duplikate mit ganzem Pfad, Band für den Entwurf einer Bibliothek |

| Radio: Länder | `Laender.dc.html` | Kontinente als Kacheln mit Abzeichen und Pfeil, offene Kachel mit Länderliste; umschaltbar auf den heutigen Zustand |

Die Mockups zeigen den Stand der Fragen F-1 bis F-13. Wo der Text dieses Papiers genauer ist als
ein Mockup (Regeln zum Löschen von Dateien, Blättern der Listen), gilt das Papier.
