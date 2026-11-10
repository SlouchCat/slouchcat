# Konzept: Bedienung, Löschen und die neue Seite «Gesehen»

- **Status: Fassung 1 (11. November 2026).** Marco hat am 10. und 11. November 2026 im Interview
  elf Fragen beantwortet (**F-1 bis F-11**, Abschnitt 17), drei davon an klickbaren Mockups
  (Anhang A). Offen sind die Punkte **O-1 bis O-12** (Abschnitt 18), jeder mit Empfehlung; bis zu
  Marcos Antwort gilt die Empfehlung. Ein Kritiker-Review steht noch aus. **Die Umsetzung beginnt
  erst auf Marcos ausdrückliches Wort.**
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
  (`SearchPage.tsx:304`).
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

- **Migration** (nur vorwärts, nach Vorlage): bestehende Favoriten in die Merkliste übernehmen
  (`INSERT OR IGNORE INTO watchlist SELECT profile_id, item_id, created_at FROM favorites`), dann
  `DROP TABLE favorites`. Niemand verliert eine Benachrichtigung.
- **Server:** `NotifyHasFavorites` und `NotifyFavoriteTargets` (`queries/system/notify.sql`) lesen
  neu `watchlist`, eingeschränkt auf Einträge der Art Serie und Podcast; ebenso die Abfrage der
  Offline-Regel (`queries/system/offline.sql:180-194`). Es entfallen `discover.Favorites`,
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
- **Doku:** Spec E-8 und ADR 0008 bekommen einen Nachtrag («Favorit durch Merkliste abgelöst»).
  Ältere Konzepte, Reviews, Mockups und die eingefrorenen DB-Stände in `testdata/db/` bleiben, wie
  sie sind: Sie beschreiben den damaligen Stand.
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
- «Abspielen» im Spotlight (`ui/HeroCarousel.tsx:515-526`).

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

- **Setzen** geht ohne Rückfrage, auch für eine ganze Serie.
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
- Gesetzt wird sie, **wenn `completed` von 0 auf 1 wechselt** (derselbe Übergang, der heute das
  Aktivitätsereignis `playback_completed` auslöst, `playback.go`), und beim Haken von Hand. Jedes
  weitere Fertigsehen schreibt das neue Datum.
- Sie **bleibt stehen**, wenn ein späteres Ereignis den Titel wieder öffnet. `completed` und
  `position_ms` verhalten sich wie heute (AN-10 bleibt für «Weiterschauen»).
- **Der Haken in der Oberfläche zeigt `watched_at`**, nicht mehr `completed`. Ein gesehener Film,
  den man nochmals anfängt, trägt also Haken **und** Fortschrittsbalken und steht in
  «Weiterschauen» **und** unter «Gesehen». Der Filter «Gesehen» der Bibliothek
  (`status=watched`) und der abgeleitete Zustand von Serie, Staffel und Podcast rechnen ebenfalls
  mit `watched_at`.
- **Migration:** `watched_at = updated_at` für alle Zeilen mit `completed = 1`. Titel, die vor dem
  Update fertig gesehen und danach wieder angefangen wurden, lassen sich nicht nachtragen; das
  Wissen fehlt in der Datenbank.
- Offline gesehene Titel bekommen `watched_at` beim Hochladen der Ereignisse, mit der Zeit des
  Ereignisses (`internal/playback/offline.go:65-67` wendet die 90-%-Regel schon an).

### 6.4 Vertrag und Server

- **Neue Operation `setItemWatched`**: `PUT /items/{id}/watched`, Körper `{ "watched": true|false }`,
  `x-visibility: scoped`; ein nicht sichtbarer Titel gibt 404.
  - Einzelner Titel (Film, Folge, Extra, Hörbuch, Podcast-Folge): `true` setzt `completed = 1`,
    `position_ms = duration_ms`, `watched_at = jetzt`, `updated_at = jetzt`; `false` setzt
    `completed = 0`, `position_ms = 0`, `watched_at = NULL` (die Zeile bleibt wegen der gemerkten
    Hörgeschwindigkeit).
  - Serie, Staffel, Podcast: dasselbe für **alle sichtbaren Folgen** darunter, in einer
    Transaktion. Folgen, die das Profil nicht sehen darf, bleiben unberührt.
  - `updated_at = jetzt` ist Absicht: Setzt man Staffel 1 von Hand auf gesehen, erscheint die
    Serie in «Weiterschauen» mit der ersten Folge von Staffel 2 (Regel aus Konzept 15, P-1).
- **Additive Felder:** `watchedAt` am Fortschritt eines Titels; die Karten (`ItemSummary`) tragen
  schon, was sie für Balken und Abzeichen brauchen, und bekommen `watched` (für abgeleitete Arten:
  alle sichtbaren Folgen gesehen).
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
- «Löschen» heisst wie heute: Der Lauf wird `cancelled`, verschwindet aus «Fehlgeschlagen» und
  wird nach der 30-Tage-Regel ganz entfernt. Unter «Läufe» ist er bis dahin als abgebrochen zu
  sehen.

### 7.3 Vertrag und Server

- `retryFailedJobs`: `POST /admin/jobs/failed/retry`, optional `type`; antwortet mit
  `{ retried, skipped }`. Ein Lauf wird übersprungen, wenn schon ein wartender oder laufender
  Zwilling mit demselben Idempotenzschlüssel besteht (dieselbe Regel wie `RetryFailedJob`,
  `queries/system/admin_dashboard.sql:20-28`).
- `dismissFailedJobs`: `DELETE /admin/jobs/failed`, optional `type`; antwortet mit `{ dismissed }`.
- `listJobRuns`: `type` wird freiwillig, wenn `state=failed` gesetzt ist (heute Pflicht,
  `api/openapi.yaml:6095-6150`).
- Alle drei `x-visibility: admin`, additiv. Beide Sammelaktionen sind je eine SQL-Anweisung, keine
  Schleife im Browser.

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
- Server: Die vier Leser in `visibility` (`ContinueWatching`, `Recent`, `Released`,
  `SpotlightRow`) bekommen Versatz und Grenze; die Abfragen bleiben in `queries/scoped/` und
  enthalten `visible_items` (Regel 2).
- **Neue Operation `listCollections`** für `/collections` (O-3), falls sie gebaut wird.
- Additiv. Tastaturtest für die neue Seite und für den Reihentitel als Fokus-Halt (Pfeil nach oben
  aus der Reihe führt auf den Titel, Enter öffnet die Liste) in `e2e/home.spec.ts`.

## 9. P-8 Bibliothek löschen

### 9.1 Heute

Eine Bibliothek lässt sich nicht löschen: Der Vertrag hat kein `deleteLibrary`
(`/admin/libraries/{id}` kennt nur `get` und `patch`), die Oberfläche keinen Knopf. Konzept 15
hält das fest (`15-feinschliff.md:538-540`), das Protokoll führt es unter «Nicht gebaut»
(`04-entscheidungen.md:22`). Man kann nur einzelne Ordner entfernen (`removeLibraryRoot`). Die
Fremdschlüssel sind vorbereitet: Mit der Bibliothekszeile fallen Ordner, Titel und alles, was an
Titeln hängt (`migrations/00003_libraries_items.sql:37`, `:51`).

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
- **Sicherheitsnetz:** Vor dem Löschen sichert der Server die Datenbank mit `VACUUM INTO`, wie vor
  jeder Migration (`internal/store/migrate.go`); die Sicherung lässt sich mit der vorhandenen
  Wiederherstellung zurückspielen.

### 9.3 Vertrag und Server

- `deleteLibrary`: `DELETE /admin/libraries/{id}?confirmName=…`, `x-visibility: admin`; 409, wenn
  der Name nicht passt.
- Ablauf im Paket `library` unter der Scan-Sperre (`lockScan`): laufenden Scan und wartende Jobs
  der Bibliothek abbrechen, Datenbank sichern, Bibliothekszeile löschen (die Fremdschlüssel räumen
  den Rest), danach die Dateien im eigenen Cache aufräumen (Bilder, Trickplay, Transkodierungen,
  Downloads) über die vorhandenen Aufräumläufe.
- **Messauftrag M-1 vor dem Bau:** Wie lange dauert das Löschen einer Bibliothek mit 10 000 Titeln
  auf CT 190 (die Suchtabellen hängen an Triggern)? Unter fünf Sekunden läuft es direkt in der
  Anfrage; darüber als Hintergrundjob `library.delete`, die Bibliothek verschwindet sofort aus den
  Ansichten.
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
  Fortschritt und Merkliste bleiben an ihm hängen und kommen zurück, wenn derselbe Titel wieder
  auftaucht (Umhängen über die Anbieter-Nummer, ohne Frist).
- Eine Ansicht der ignorierten Dateien gibt es nicht; was einmal ignoriert ist, bleibt es.

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
  eingetippt werden.
- **«Nur aus Slouchcat entfernen»:** Die Dateien kommen auf die Ignorierliste (`ignored_files`,
  dieselbe Mechanik wie B-9), ihre Zeilen in `media_files` fallen weg, der Titel wird weich
  gelöscht und verschwindet für alle Profile aus allen Listen. Downloads auf den Geräten werden
  entfernt. Fortschritt, Haken und Merkliste bleiben gespeichert.
- **Neu «Wartung › Ignorierte Dateien»:** Liste aller ignorierten Dateien mit Pfad und Datum,
  je Zeile «Wieder aufnehmen». Der nächste Scan bringt die Datei zurück, der Titel lebt mit seinen
  Benutzerdaten wieder auf. Ohne diese Ansicht wäre «nur entfernen» eine Einbahnstrasse.
- Jedes Löschen schreibt ein Aktivitätsereignis (wer, welcher Titel, welche Wahl, wie viele
  Dateien, wie viele Bytes).

### 10.3 Vertrag und Server

- `getItemDeletion`: `GET /admin/items/{id}/deletion` → Vorschau (`files[]` mit `rootPath`, `path`,
  `sizeBytes`; `fileCount`, `totalBytes`, `filesDeletable`, `reason`).
- `deleteItem`: `POST /admin/items/{id}/deletion`, Körper `{ "mode": "library" | "files",
  "confirmTitle": "…" }` → Ergebnis (`removedItems`, `deletedFiles`, `deletedBytes`, `leftover[]`).
- `listIgnoredFiles`, `restoreIgnoredFile` für die neue Wartungsansicht.
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
- Das Paket `paths` bietet nur lesende Helfer (`internal/paths/root.go:16-124`). Alle
  `Remove`-Aufrufe des Servers treffen `/config` oder `/cache`.
- Konzept 19 Teil A (noch nicht gebaut) öffnet erstmals einen Ordner zum Schreiben, den
  Podcast-Ordner, und hält fest: «Slouchcat löscht nie eine Datei im Podcast-Ordner» (F-4).

### 11.2 Vorschlag

- **Vorgabe bleibt «nur lesbar».** Das mitgelieferte `compose.yaml` bindet Medien weiter mit
  `:ro` ein. Wer Dateien aus Slouchcat löschen will, stellt den Ordner **selbst und bewusst** auf
  schreibbar um; die Installationsanleitung beschreibt es. Der `:ro`-Schalter bleibt damit die
  harte Sicherung: Ohne ihn umzulegen, kann kein Fehler in Slouchcat eine Datei löschen.
- **Der Server prüft je Ordner, ob er schreiben darf** (Schreibprobe beim Start und beim Öffnen
  des Dialogs, ohne eine Datei liegen zu lassen). Der Dialog «Ordner von …» zeigt den Stand
  («schreibbar» / «nur lesbar»). Ist der Ordner des Titels nur lesbar, ist die Wahl «Auch die
  Dateien löschen» grau, mit Hinweis.
- **Was gelöscht wird:** nur, was Slouchcat sicher zuordnen kann:
  1. die Mediendateien des Titels aus `media_files` (alle Versionen und Teile, bei Serie und
     Staffel die aller Folgen),
  2. Begleitdateien **mit demselben Dateistamm** daneben (Untertitel, `.nfo`, Bilder wie
     `Film (2024)-poster.jpg`),
  3. danach den Ordner, **wenn er leer ist**, und leere Elternordner bis unter den
     Bibliotheksordner.
- **Kein rekursives Löschen von Ordnern.** Liegt in einem Film- oder Serienordner etwas, das
  Slouchcat nicht kennt, bleibt es liegen; das Ergebnis nennt die Zahl («3 Dateien bleiben im
  Ordner»).
- **Kein Papierkorb** (O-6): Gelöscht ist gelöscht. Der Dialog sagt es.
- **Reihenfolge:** erst die Dateien, dann die Datenbank. Scheitert eine Datei (Rechte, Datei in
  Gebrauch), bleibt der Titel mit den verbliebenen Dateien bestehen und das Ergebnis nennt sie.
  Eine schon fehlende Datei gilt als gelöscht.
- **Technik:** `paths` bekommt `Remove(root, rel)` und `RemoveEmptyDir(root, rel)` auf `os.Root`
  (Go 1.27, `server/go.mod`); der Zugriff bleibt damit innerhalb der registrierten Wurzel, `..`
  und Symlinks nach aussen sind ausgeschlossen (Regel 4). Kein `os/exec` (Regel 3).
- **Nur der Server-Administrator** darf löschen; die Operation ist `admin`, die Web-App zeigt den
  Knopf nur dem Hauptprofil des Administrators (`canManage`).
- **Radarr und Sonarr wissen von nichts.** Löscht man in Slouchcat einen Film, den Radarr noch
  überwacht, lädt Radarr ihn womöglich neu. Der Dialog weist darauf hin; Slouchcat steuert Radarr
  und Sonarr nicht (Nicht-Ziel der Spec, Regel 11).

### 11.3 Warum eine ADR

Der Schritt hebt B-1 in einem Punkt auf: Slouchcat **darf** in Medienordner schreiben, wenn der
Betreiber es erlaubt, und zwar ausschliesslich, um auf ausdrücklichen Befehl des Administrators
Dateien zu löschen. **ADR 0032** hält fest: Vorgabe nur lesbar, Schreibprobe statt Einstellung,
keine rekursiven Löschungen, kein automatisches Löschen (kein Aufräumjob, keine Regel «nach dem
Sehen löschen»). Der Abnahmetest «Schreiben nach `/media` scheitert» bleibt für die Vorgabe
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

## 13. P-11 Die Frage «bleiben oder verwerfen» ohne Speichern-Knopf

### 13.1 Befund

- Der Dialog «Ordner von …» speichert **jede Aktion sofort** (Entfernen, Pfad ändern, Hinzufügen);
  er setzt kein «ungespeichert» (`features/admin/libraries/LibraryFoldersDialog.tsx:102-140`).
- Die Rückfrage beim Verlassen kommt auf der Seite «Bibliotheken» aus einer einzigen Quelle:
  `useUnsavedChanges(draft.dirty)` (`features/admin/LibrariesPage.tsx:88-89`), und `draft` ist der
  **Entwurf von «Bibliothek hinzufügen»** (`AddLibraryDialog.tsx:61-100`).
- Dieser Entwurf **überlebt das Schliessen des Dialogs** mit Absicht (S-13; «Abbrechen» und ✕
  rufen nur `create.reset()`), **die Seite zeigt ihn aber nirgends**. Speichern oder verwerfen
  kann man ihn nur, indem man «Bibliothek hinzufügen» nochmals öffnet.
- `dirty` wird schon wahr, wenn man im Hinzufügen-Dialog einen Namen tippt (etwa «test2») oder
  eine Ordnerkarte hinzufügt und eine andere entfernt (`untouchedFolders`,
  `AddLibraryDialog.tsx:47-58`: die verbleibende Karte hat dann die Art `series` statt `movie`).

**Wahrscheinlicher Ablauf:** Beim Anlegen von «test2» blieb ein angefangener Entwurf zurück;
später, nach dem Entfernen eines Ordners, fragte die Seite beim Verlassen nach, ohne dass ein
Speichern-Knopf zu sehen war. **Das ist aus dem Code gelesen, nicht nachgestellt** (Sicherheit
mittel bis hoch). Schritt 0 des Baus stellt Marcos Klickfolge in einem e2e-Test nach; passt die
Erklärung nicht, wird die tatsächliche Ursache behoben und dieses Papier nachgeführt.

### 13.2 Vorschlag

Mockup «Jobs, Löschen, Duplikate», Thema «Entwurf "test2"».

- Liegt ein Entwurf vor und ist der Dialog zu, zeigt die Seite ein **Band**: «Entwurf "test2" ist
  noch nicht gespeichert», mit **«Weiter bearbeiten»** und **«Entwurf verwerfen»**.
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
- Auf jeder Karte der Bleistift (nur Server-Administrator) und der **gesetzte Haken**; ein Klick
  entfernt ihn (bei Serie und Podcast mit Rückfrage), der Titel verlässt die Liste.
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

## 18. Offene Punkte

Bis zu Marcos Antwort gilt die Empfehlung.

| Nr. | Punkt | Empfehlung |
|---|---|---|
| O-1 | Leerer Kreis auf Posterkarten mit Maus: immer sichtbar oder erst beim Zeigen? | Erst beim Zeigen und im Fokus; der gesetzte Haken immer. Sonst trägt jede Karte dauernd bis zu drei Knöpfe (Bleistift, Merken, Haken). |
| O-2 | Schreibt der Haken von Hand ein Aktivitätsereignis «fertig gesehen»? | Nein. Das Protokoll der Verwaltung soll echte Wiedergaben zeigen. |
| O-3 | Bekommt «Sammlungen» eine Übersichtsseite `/collections` als Ziel des Reihentitels? | Ja, einfaches Raster aller sichtbaren Sammlungen. |
| O-4 | Bibliothek löschen: Gehen bei einer Podcast-Bibliothek auch die Abos ohne Datei mit? | Ja, die ganze Bibliothek ist weg. |
| O-5 | Podcast-Folgen, die nur als Verweis aus dem Feed bestehen: einzeln löschbar? | Nein. Der Feed brächte sie beim nächsten Abruf zurück; dafür bräuchte es eine eigene Ausblendliste. |
| O-6 | Dateien löschen: Papierkorb-Ordner statt endgültig? | Nein, endgültig. Ein Papierkorb bräuchte Platz, eine Frist und eine eigene Ansicht. |
| O-7 | Zieht der Google-Books-Schlüssel (Konzept 19 D3) als sechste Karte auf die Seite «Schlüssel der Anbieter»? | Ja, nach dem Merge von PR #139; die Seite «Hörbücher: Quellen» verweist dann dorthin. |
| O-8 | Bekommt die Seite «Einstellungen» einen Verweis auf «Benachrichtigungskanäle»? | Ja, als drittes Feld mit Link. |
| O-9 | Gilt die Nachfrage «weiter oder von vorn» auch für Bände einer Hörbuch-Reihe? | Ja (Text «Weiterhören ab …»); es ist derselbe Direktstart. |
| O-10 | Fragt «Alle erneut versuchen» nach? | Nein, es ist gefahrlos. Nur «Alle löschen» fragt. |
| O-11 | Löscht «Auch die Dateien löschen» Begleitdateien mit gleichem Dateistamm mit? | Ja, sonst bleiben Untertitel und Bilder verwaist und der Ordner wird nie leer. |
| O-12 | Soll die Serie auf der Merkliste bleiben, wenn man alle Folgen gesehen hat? | Ja. Sie meldet weiter neue Folgen, bis man sie von Hand entfernt. |

## 19. Bauplan in Stufen

| Stufe | Inhalt | Vertrag / Migration | Aufwand (grob) |
|---|---|---|---|
| **1 · Serienseite und Kleines** | P-1 Pfeile, P-3 Bleistift der Staffel, P-4 Folgenklick mit Nachfrage, P-11 Entwurf, P-13 Umbenennung | keiner / keine (Patch-Release) | ≈ 4 Tage |
| **2 · Haken und «Gesehen»** | P-5 Haken zum Anklicken, `watched_at`, P-14 Seite «Gesehen» | additiv (`setItemWatched`, `listWatched`, Felder) / eine (`progress.watched_at`) | ≈ 6 Tage |
| **3 · Startseite und Favorit** | P-7 Reihentitel, Listenseite, Spotlight-Titel, Bleistifte auf allen Listen; P-2 Favorit entfernen | **brechend** (Favoriten-Operationen fallen), additiv (`listHomeRow`, `listCollections`) / eine (Favoriten in die Merkliste, Tabelle weg) | ≈ 6 Tage |
| **4 · Verwaltung** | P-6 Jobs, P-10 Duplikate mit Pfad, P-12 Schlüssel mit «Prüfen» | additiv / keine | ≈ 4 Tage |
| **5 · Löschen** | P-8 Bibliothek löschen, P-9 Titel löschen in beiden Arten, Ignorierte Dateien, ADR 0032, Doku | additiv / keine erwartet | ≈ 8 Tage |

- Die Stufen sind voneinander unabhängig, ausser: Stufe 3 nutzt den Haken aus Stufe 2 auf der
  Listenseite, und der Knopf «Gesehen» der Detailseite (Stufe 2) rückt auf den Platz des Favoriten,
  der erst in Stufe 3 fällt; bis dahin stehen beide nebeneinander.
- Jeder Planschritt ist ein eigener Commit mit DoD-Belegen der Teststufe 1; vor jedem Pull Request
  `task ci` grün (`CLAUDE.md`, «Arbeitsweise»). Jede neue Seite hat ihren Tastaturtest (Regel 8):
  Listenseite, «Gesehen», «Sammlungen», «Ignorierte Dateien»; dazu die umgebaute Folgenzeile und
  die Dialoge.
- Vor Stufe 5 die Messung M-1, in Stufe 2 die Messung M-2.
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
| Gesehen | `Gesehen.dc.html` | Die neue Seite mit Filter, Sortierung, Serien als ein Eintrag, Rückfrage beim Entfernen |
| Schlüssel | `Schluessel.dc.html` | Fünf Anbieter-Karten, auf- und zuklappbar, «Prüfen» |
| Jobs, Löschen, Duplikate | `Verwaltung.dc.html` | Fünf Themen: Sammelaktionen der Jobs, Titel löschen (Ordner schreibbar oder nur lesbar), Bibliothek löschen, Duplikate mit ganzem Pfad, Band für den Entwurf «test2» |
