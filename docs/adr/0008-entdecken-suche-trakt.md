# ADR 0008: Entdecken – Suchindex, Startseiten-Reihen, Trakt und Benachrichtigungen

- Status: angenommen (21. Oktober 2026); **zwei Zeilen seit M11 teilweise abgelöst durch
  [ADR 0011](0011-spotlight.md)** — siehe den Hinweis unter der Entscheidstabelle
- Quelle: `docs/planning/03-plan-v1.md`, Abschnitte 3.1 (Module `discover`, `notify`, `ratelimit`), 3.3
  (Query-Builder in `visibility`), 3.4 Schritt 5, 5.5, 5.7 AN-3 und M7; Entscheid 9 (Trakt-Teil) in
  `docs/planning/04-entscheidungen.md`; Spec E-1 bis E-9, I-7; Entwurf in den Sitzungsnotizen `m7-design.md`
- Betroffene Spec-IDs: E-1 bis E-9, I-7, K-3, O-3

## Kontext

M7 macht die Startseite vollständig, bringt die Suche, Merkliste, Favoriten, Sammlungen und die Trakt-Reihen.
Fünf Fragen waren zu entscheiden:

1. Wie wird **gesucht** (Titel, Personen, Sammlungen, Autor, Sprecher ab dem zweiten Zeichen, E-4), ohne dass die
   Sichtbarkeit (K-3) umgangen werden kann und ohne dass der Index veraltet?
2. Wo laufen die neuen **Abfragen**, wenn `discover` nach CLAUDE.md Regel 2 kein Titel-SQL enthalten darf?
3. Wie kommen die **Trakt-Listen** in die Bibliothek (AN-3: Top 100 laden, erste 20 Treffer zeigen, unter 5
   ausblenden), und was passiert bei einem Ausfall (I-7)?
4. Wie entstehen **Benachrichtigungen** für Kinderprofile (5.5), ohne dass eine unsichtbare Folge je einen
   Eintrag erzeugt?
5. Was wird aus dem **Platzhalter-Karussell** aus M1a (neueste Titel mit Backdrop), wenn es echte Picks gibt?

Verworfen: (a) Suche über `LIKE` auf `items.title` – kein Treffer in der Profilsprache, keine Personen, keine
Präfixsuche mit Diakritika, bei 42 000 Titeln zu langsam für P-11. (b) Ein externer Suchdienst – Zusatzcontainer
(Prinzip 2). (c) Den FTS-Index aus Go-Code pflegen (`library`, `metadata` nach jedem Schreiben) – jede Fixture mit
rohen `INSERT`s wäre unsichtbar, und ein vergessener Aufruf liesse den Index still veralten. (d) Ein Paket
`discover` mit `systemq` – Regel 2 müsste geändert werden. (e) Trakt-Treffer beim Abruf in Items «einmischen» – die
Verschneidung je Profil muss durch `visible_items`; die Rohliste gehört darum in eine eigene Tabelle. (f)
Benachrichtigungen per Umfrage (Ticker über `added_at`) – einfacher, aber ein zweites Mal Scan-Wissen nachbauen;
die Hooks der schreibenden Pakete sind präziser. (g) Trakt-Konten je Profil oder Scrobbling – Non-Goal.

## Entscheid

| Punkt | Entscheid |
|---|---|
| Suchindex | Eine FTS5-Tabelle `search_index(text, kind, item_id, ref_id)` mit `unicode61 remove_diacritics 2` und `prefix='2 3'`: eine Zeile je `items`-Zeile (Titel, Originaltitel, Autor, Sprecher, Reihe), je `item_texts`-Zeile (Titel in de/en), je `credits`-Zeile (Name der Person) und je Sammlungsmitglied (Name der Sammlung). **Trigger** in derselben Transaktion halten den Index nach; die Migration füllt ihn rück. Der Index kennt keine Sichtbarkeit: jede Lesung joint `visible_items`, eine Person erscheint nur über eine sichtbare Rolle, eine Sammlung nur mit sichtbarem Mitglied |
| Query-Builder | Ein kleiner Builder in `visibility` (`Reader.Search`) setzt aus festen SQL-Fragmenten die Suche zusammen; der Rumpf `FROM search_index JOIN items JOIN visible_items` ist nicht weglassbar, Filter (Genre über `item_genres`, Jahr, Altersstufe, Medientyp, Verfügbarkeit lokal/Download) sind `EXISTS`- bzw. `IN`-Fragmente mit Parametern. Alle übrigen Listen (Merkliste, Favoriten, Sammlungen, Genres, Trakt-Reihen) sind statische sqlc-Abfragen in `queries/scoped/` mit `visible_items` |
| Genres | `item_genres(item_id, lang, name)` per Trigger aus `items.genres` (JSON je Sprache oder reine Liste), damit Genre-Filter und die Reihe «Genres» ohne JSON-Funktionen je Anfrage auskommen |
| Pakete | `discover` ordnet die Reihen, prüft Arten und schreibt Merkliste/Favoriten über `appq` (die Tabellen nennen `items` nicht); `admin` schreibt Picks, manuelle Sammlungen und die Reihenfolge; `metadata` schreibt TMDB-Sammlungen (`belongs_to_collection` im Match) und die Trakt-Listen; `notify` (vorab in der Allowlist) schreibt und liest Benachrichtigungen; das reine Paket `trakt` hat keine Datenbank. Regel 2 und die Lint-Listen bleiben unverändert |
| Startseite | Acht Reihenschlüssel in `home_rows` mit Position und `hidden` (E-9: höchstens acht, Reihenfolge durch den Hauptbenutzer); leere und versteckte Reihen entfallen. Reihen `collections` und `genres` tragen statt `items` eigene Listen (`CollectionRef`, `GenreRef`). Das Karussell zeigt nur noch Picks; **ohne Picks gibt es kein Karussell** (der Platzhalter aus M1a entfällt). Die Notiz eines Picks reist als `ItemRef.note` |
| Picks (E-2, E-3) | `top_picks` mit `profile_id` (NULL = alle), Position, Notiz, Zeitraum; ein Profil sieht einen Pick nur über `visible_items` und nur im Zeitraum (Serveruhr, im Test injizierbar). Treffer für das Profil und global werden dedupliziert |
| Trakt (AN-3, I-7) | Täglicher Job `trakt.refresh` über `ratelimit` (1/s) mit dem Server-Schlüssel (`SLOUCHCAT_TRAKT_CLIENT_ID`, sonst Setting) holt die vier öffentlichen Listen (trending/popular × movies/shows, je 100) in `trakt_lists(list, media, position, tmdb_id)`; die Reihe eines Profils ist der Join auf `items.tmdb_id` und `visible_items`, Reihenfolge nach Position, höchstens 20, **ausgeblendet unter 5 Treffern**. Ein fehlgeschlagener Lauf **leert** die Tabelle: die Reihen verschwinden, sonst passiert nichts. Kein Trakt-Konto je Profil, nichts wird gesendet; ein Schema-Test hält das fest |
| Benachrichtigungen (E-8, 5.5) | `library` (neue Folgen nach dem Commit eines Scan-Batches) und `metadata` (neue Episoden nach dem Feed-Abruf) rufen einen Hook `notify.NewEpisodes(ids)`. `notify` ermittelt Serie bzw. Podcast, die Profile mit Favorit darauf und prüft je Profil `visibility.Visible`; nur dann entsteht ein Eintrag in `notifications` (eindeutig je Profil und Folge). Beim Lesen joint der Eingang erneut `visible_items`. Kanäle (M8) hängen an einer `Dispatcher`-Schnittstelle mit No-op-Standard |
| Favoriten und Auto-Downloads (O-3) | Die Regel «neue Episoden» gilt für **favorisierte** Podcasts des Profils; der M6-Ersatz «in den letzten 60 Tagen gehört» entfällt |

> **Teilweise abgelöst durch ADR 0011 (25. Oktober 2026).** Zwei Zeilen dieser Tabelle gelten ab
> Meilenstein M11 nicht mehr; sie bleiben hier stehen, damit nachvollziehbar ist, was M7 entschieden
> hat.
>
> - **«Startseite», Satz «ohne Picks gibt es kein Karussell»:** die Bühne ist nicht mehr an `picks`
>   gebunden. Welche Reihe als Band gezeichnet wird, sagt der Server im Vertrag
>   (`HomePage.heroRow`), und das ist allein `spotlight`. Cat's Empfehlungen ist immer Posterzeile;
>   fehlt Spotlight, hat die Seite **keine** Bühne.
> - **«Trakt (AN-3, I-7)», vier feste Listen mit «höchstens 20, ausgeblendet unter 5»:** an ihre
>   Stelle tritt **eine in der Verwaltung wählbare** Liste; Anzahl (5 bis 20), Mindesttreffer
>   (0 bis 10) und Abrufintervall (6 bis 48 Stunden) stehen in `spotlight_config`. Der Cache heisst
>   `spotlight_entries`, `trakt_lists` entfällt.
>
> Unverändert gültig bleibt alles Übrige, insbesondere I-7: nur Server-Schlüssel, kein Trakt-Konto
> je Profil, nichts wird gesendet, und ein Ausfall kostet nur die Reihe.

## Folgen

- Vertrag 0.8.0 (109 Operationen): `search` (POST), `listGenres`, `getCollection`, Merkliste und Favoriten je
  Profil, der In-App-Eingang, 16 Admin-Operationen für Picks, Sammlungen, Reihenfolge und Trakt.
- Migration 00015 legt die Tabellen und Trigger an und füllt Index, Genres und Reihenfolge rück; jede Fixture ist
  damit sofort suchbar (`TestMigration00015IndexesFrozenData`).
- Ein gelöschter oder unsichtbarer Titel bleibt im Index; nur der Join filtert. Massenlöschungen kosten je Titel
  einen Index-Scan über die UNINDEXED-Spalten (gemessen in `docs/perf/M7.md`).
- Der Client enthält weiterhin keine Sichtbarkeitsregel (Prinzip 1): Suche, Reihen, Sammlungen und Eingang
  kommen gefiltert vom Server.
- Offen: Filter «in Beschaffung» (M9), Kanäle für Benachrichtigungen (M8), Einstellungsseite für den
  Trakt-Schlüssel (M8).
