# ADR 0015: Bibliotheken als Familie, Inhaltsart und geordnete Metadatenquellen je Ordner

- Status: angenommen (29. Oktober 2026); **ersetzt teilweise ADR 0006** (siehe Abschnitt «Was von
  ADR 0006 stehen bleibt») und schreibt eine Zeile von ADR 0007 fort (Podcast-Bibliothek «ohne Wurzel»)
- Quelle: Konzept `docs/planning/07-bibliotheken.md` (Fassung 4, 28./29. Oktober 2026, Abschnitte 2,
  4, 5, 6, 9); **Frage 13** in `docs/planning/04-entscheidungen.md` mit Marcos Antworten F-1 bis F-13;
  Idee 01 (`docs/ideas/01-metadaten-einstellungen.md`, Teil A); ADR 0006 (Anbieter-Schnittstelle),
  ADR 0007 (Audio-Modell), ADR 0012 (browsbare Wurzeln)
- Betroffene Spec-IDs: B-1, B-3, H-1, H-2, AN-2, AN-15, C-5, K-3
- Harte Regeln: 1 (generierter Code nur durch Codegen), 2 (Sichtbarkeit nur über `visibility`),
  4 (Dateizugriff nur über `paths`), 5 (Migrationen nur vorwärts), 7 (`x-visibility` je Operation und
  Eintrag im Vertragstest), 8 (Tastaturtest), 9 (keine harten Farben und Texte), 10 (keine Secrets in
  Logs), 11 (keine Nicht-Ziele), 12 (Entscheid im Server)

## Kontext

Eine Bibliothek trägt bis Vertrag 1.2.0 **eine** Art für alle ihre Ordner (`libraries.kind` in
`movies`, `series`, `audiobooks`, `podcasts`), und die Reihenfolge der Metadaten-Anbieter ist fest im
Code: Filme nur TMDB, Serien TMDB und TVDB nur als Rückfall (ADR 0006). Marcos Zielstruktur vom
28. Oktober 2026 braucht drei Dinge, die dieses Modell nicht kann:

1. **Eine Bibliothek mit Film- und Serienordnern** («Dokus», «Kids», «Anime»), damit ein Profil je
   Bibliothek ein Recht bekommt und nicht zwei.
2. **Eine Anbieterliste je Ordner mit Reihenfolge**, wie Emby sie zeigt: Anime-Ordner sind nach Sonarr
   mit TVDB-Staffelzählung benannt, dort muss TVDB zuerst erkennen; ein Anime-Anbieter (AniList) soll
   Originaltitel, Beschreibungen und Bilder beisteuern.
3. **Einen Podcast-Ordner** (Audiobookshelf-Layout mit `metadata.json` und `feedURL`), den ADR 0007
   ausdrücklich als offen gelassen hat.

Vier Fragen waren zu entscheiden:

1. Wo lebt die Art: an der Bibliothek, am Ordner, oder an beiden?
2. Was bedeutet eine Anbieterliste mit Reihenfolge, wenn zwei Anbieter dieselbe Folge verschieden
   zählen oder dieselbe Beschreibung verschieden liefern?
3. Wie wird ein Podcast-Ordner gelesen, und wie geht der Server mit einer Feed-Adresse um, die aus
   einer Datei auf dem NAS kommt statt vom Administrator?
4. Was geschieht mit den Titeln eines Ordners, wenn der Administrator Inhaltsart oder Liste nachträglich
   ändert (F-4)?

Verworfen: (a) **`libraries.kind` streichen und die Familie aus den Ordnern ableiten** – eine
Bibliothek ohne Ordner ist erlaubt (Idee 02, E13), und `default_rating`, der Scan-Weg und die
Podcast-Abos brauchen eine Antwort vor dem ersten Ordner. (b) **`movies` als Wert behalten und
umdeuten** – ein entfernter Enum-Wert wird nie wiederverwendet; Fremd-Clients, die `kind === 'movies'`
prüfen, sollen brechen, nicht stillschweigend falsch liegen. (c) **Eine Einzelwahl «Hauptscraper
plus Ergänzer» statt einer Liste** – Marco hat die Liste mit Reihenfolge gewählt (F-11), weil nur sie
den Fall «AniList vor TMDB für englische Beschreibungen» ausdrücken kann. (d) **Eine eigene Tabelle
`library_root_providers (root_id, position, provider)`** – die Liste wird nur als Ganzes gelesen und
geschrieben und ist nie länger als vier Einträge; eine JSON-Liste wie bei `items.genres` genügt.
(e) **Eine Spalte `items.root_id`** – für Container falsch (eine Serie gehört zur Bibliothek, ihre
Folgen zu Wurzeln); `media_files.root_id` beantwortet die Frage «welche Titel liegen unter dieser
Wurzel» bereits. (f) **Alle Auffüller in der Match-Transaktion abfragen**, damit kein Fenster zwischen
Erkennen und Auffüllen entsteht – Netzaufrufe in einer Schreibtransaktion tut dieses Projekt nirgends;
das Fenster bleibt als Restrisiko benannt. (g) **AniList als Erkenner** – es liefert keine
Staffelstruktur und keine Ids, an denen Credits, Trailer-Kette und Bildpipeline hängen (F-5).
(h) **Eine Titelsuche bei AniList** für Ordner, die weder TMDB noch TVDB kennen – bleibt aus; ein
solcher Titel bleibt `unmatched` und erscheint in den Pflegewerkzeugen (B-9). (i) **Die
LAN-Ausnahme von `netguard` auch für Feed-Adressen aus Dateien** – dann entschiede eine Datei auf
dem NAS, dass der Server Adressen im Heimnetz abruft (9.1 des Konzepts).

## Entscheid

| Punkt | Entscheid |
|---|---|
| Familie an der Bibliothek | `libraries.kind` kennt nur noch die **Familien** `video`, `audiobooks`, `podcasts`. Die Familie sagt, welche Inhaltsarten ihre Ordner tragen dürfen, sonst nichts; `movies` und `series` sind verbrannt und werden nie wiederverwendet. Rechte (`profile_libraries`, `account_libraries`, `invitation_libraries`), Scan-Zustand, Verlustschutz und Standard-Altersstufe (AN-2) bleiben an der Bibliothek |
| Inhaltsart am Ordner | `library_roots.content` in `movie`, `series`, `audiobook`, `podcast`: welcher Parser und welcher Erkenner den Ordner lesen. Der Scanner liest **je Wurzel**, nicht je Bibliothek; der Titelindex ist einer **je Titelart**, weil TMDB-Film- und Serien-Ids getrennte Zahlenräume sind |
| Nur Video ist mischbar (F-3) | In einer `video`-Bibliothek nur `movie` und `series`; in `audiobooks` nur `audiobook`; in `podcasts` nur `podcast`. Verstoss ist 422 `content_not_allowed` |
| Metadatenquellen am Ordner (F-8, F-11) | `library_roots.providers` ist eine **geordnete JSON-Liste** aus `tmdb`, `tvdb`, `anilist`, `local`, nie leer, ohne Doppelte. In der Oberfläche heisst das Feld «Metadatenquellen» (englisch «Metadata sources»); die Vertragswerte bleiben englisch und klein |
| Erkenner und Auffüller | **Der erste Eintrag erkennt**: er allein bestimmt Identität, Ids und bei Serien die Staffel- und Folgenzählung. **Jeder weitere füllt auf**, was davor leer blieb. Erlaubte Erkenner: `movie` nur `tmdb`; `series` `tmdb` **oder** `tvdb` (F-12); `audiobook` und `podcast` nur `local`. Erlaubte Auffüller: `movie` `anilist`; `series` `tmdb`, `tvdb`, `anilist`. AniList ist nie Erkenner (F-5), `tvdb` an einem Filmordner ist 422 `providers_not_allowed` |
| TVDB zuerst (F-12) | Bei `["tvdb", …]` an einem Serienordner erkennt TVDB, Staffeln und Folgen folgen der TVDB-Zählung, und die `tmdb_id` wird **zusätzlich** über TMDBs Fremdschlüssel-Suche (`/find/{tvdb_id}`) geholt, damit Credits, Trailer-Kette und Bildpipeline weiter arbeiten, ohne dass TMDB die Zählung bestimmt. Die manuelle TVDB-Zuordnung je Titel bleibt derselbe Weg, nur je Titel statt je Ordner |
| Feldweises Auffüllen mit Herkunft | Nach jedem erfolgreichen Match reiht der Server in derselben Transaktion den Job `metadata.fill` ein. Er läuft die Liste von vorne nach hinten, Erkenner eingeschlossen, und entscheidet je Feld und Sprache: ein gesperrtes Feld (`items.locked_fields`, B-6) wird nie angefasst; sonst gewinnt der erste Anbieter mit einem nicht-leeren Wert; die Tabelle **`item_field_sources (item_id, field, lang, provider)`** hält fest, wer gewonnen hat, und macht den Job idempotent (ein weiter vorne stehender oder derselbe Anbieter bleibt, ein weiter hinten stehender wird ersetzt). Felder: `title`, `overview`, `tagline` je Sprache, `original_title`, `genres` (Vereinigung), `runtime_minutes`, `release_date`, `poster`, `backdrop`. `items.providers` trägt die Liste **kopiert** aus der Wurzel; Kinder erben sie |
| Freigabe-Ausnahme (K-3) | `own_rating` wird nur **erhöht**, nie gesenkt, gleich wo der Anbieter in der Liste steht; `rating_source` nennt den strengsten. AniList `isAdult = true` hebt auf 18. Der Kinderprofil-Vertragstest hält als Negativtest fest: «Anbieterreihenfolge senkt die Freigabe nicht» |
| AniList (F-2, F-5) | Reiner Auffüller, ohne Schlüssel, über `ratelimit.Limiter` (1 Anfrage je Sekunde, Burst 5), feste Hosts `graphql.anilist.co` und `raw.githubusercontent.com` (Zuordnungsliste von Fribb, mit ETag unter `/cache/anime-lists/`, wöchentlich erneuert, 64 MB Grenze, Karten je Art). `SLOUCHCAT_ANILIST=off` schaltet den Anbieter serverweit ab; die Häkchen bleiben. Liefert `original_title` = `title.native`, englische Texte, Genres, Poster, Backdrop; keine deutschen Texte |
| Konvergenz | `QueuePendingMetadata` reiht zusätzlich jeden gematchten Titel als `metadata.fill` ein, dessen Liste mehr als einen Eintrag hat und dessen `item_field_sources` leer sind oder einen Anbieter nennen, der nicht mehr in der Liste steht. So kommt ein Titel nach Ausfall, Listenänderung oder `SLOUCHCAT_ANILIST=off → on` zu seinen Feldern |
| Podcast-Ordner (F-1, F-6) | Ein Ordner mit `content = 'podcast'` wird gescannt: jeder Unterordner der ersten Ebene ist ein Podcast, `metadata.json` (Audiobookshelf-Schema) füllt Titel, Autor, Beschreibung, Genres, `explicit` (→ 18, `rating_source 'tags'`) und `feed_url`; jede Audiodatei ist eine `podcast_episode` mit `media_files`-Zeile, Identität `root_id + rel_path`. Steht eine `feedURL` in der Datei, wird der Feed **übernommen**: gibt es in der Bibliothek schon einen Podcast mit dieser Adresse, hängen sich die Episoden an ihn; sonst entsteht der Podcast mit `feed_origin = 'file'`. Ordner-Podcasts ohne Feed werden nicht abgefragt. Nach der Sonde führt `finishPodcastEpisode` Tags ein und verheiratet eine Datei mit einer Feed-Episode **nur bei eindeutigem Paar** (Datum auf die Minute, sonst normalisierter Titel, nie der Podcast-Titel; Dauer ±2 %); sonst bleiben zwei Episoden, sichtbar als `file_and_feed` in `listDuplicates`. Wiedergabe und Download bevorzugen die Datei. Der Erst-Scan feuert keinen Neue-Folgen-Hook |
| `feed_origin` und der Netzwächter | `items.feed_origin` in `admin` (Abo von Hand) oder `file` (aus `metadata.json`). Eine Adresse aus einer Datei wird nur mit `http`/`https` und nie mit einem lokalen Host gespeichert (Pflegezeile «Feed-Adresse aus Datei nicht zulässig», nur Schema und Host im Log, Regel 10). Bei **jedem** Abruf eines Podcasts mit `feed_origin = 'file'` (Feed, Enclosure, Cover, Download) gilt der geschützte Client, nie die LAN-Ausnahme von `netguard`. Beim Entfernen eines Podcast-Ordners bleibt ein Podcast mit Feed als Abo (`feed_origin` wird `admin`), einer ohne Feed geht mit seinen Episoden |
| Mehrere Ordner in einem Zug (F-13) | `LibraryRequest.roots[]` und `LibraryRootsRequest.roots[]` (Körper von `addLibraryRoot`): Bibliothek und alle Ordner entstehen in **einer** Transaktion, alle oder keiner. Überschneiden sich zwei Ordner der Anfrage oder einer mit dem Bestand, kommt 422 `root_overlap` mit dem Feld des Verursachers (`library.roots[1].path`). `roots` ist Pflicht bei `video` und `audiobooks` (422 `roots_required`), darf bei `podcasts` leer sein |
| Nachträgliche Änderung (F-4) | `updateLibraryRoot` (`PATCH /admin/libraries/{id}/roots/{rootId}`) mit drei Wirkungen: **Liste ändern bei gleichem Erkenner** setzt `items.providers` der Titel unter der Wurzel und reiht je Titel `metadata.fill` ein, löscht nichts, braucht keine Bestätigung. **Erkenner wechseln** dazu `metadata.match` je Serie; die Folgen-Items bleiben (Identität `(Serie, Staffel, Folge)` aus dem Dateinamen), nur Staffel- und Folgentexte werden nach der neuen Zählung neu beschrieben. **Inhalt wechseln** (`movie` ↔ `series`) ist in der Wirkung «Ordner entfernen und neu anlegen»: Löschweg aus Idee 02 ohne das Löschen der Wurzelzeile, in einer Transaktion unter der Scan-Sperre, verlangt `confirmPath` (422 `confirm_mismatch`, 409 `scan_running`), danach Wurzelzustand des Scanners leeren und `scan.library` einreihen |
| Vertrag | **1.3.0, brechend** (C-5): `LibraryKind` verliert `movies` und `series`; `LibraryRequest.path` weicht `roots[]`; `addLibraryRoot` nimmt eine Liste; `LibraryRootRequest` bekommt die Pflichtfelder `content` und `providers`; `MetadataProvider` wächst um `anilist` und `local` (`searchMetadata` und `assignMatch` nehmen weiter nur `tmdb` und `tvdb`, 422 `provider_not_searchable`); `DuplicateGroup.reason` bekommt `file_and_feed`. Neu `updateLibraryRoot`, `listMetadataProviders` (`GET /admin/metadata/providers`), die Schemata `RootContent`, `LibraryRootsRequest`, `LibraryRootUpdate`, `MetadataProviderInfo`, `FieldSource`. Kein `scoped`-Schema ändert sich; `ItemRef` und `ItemDetail` bleiben unverändert, weder `anilistId` noch Herkunft gehen an Zuschauer |
| Oberfläche (F-8, F-13) | Die Felder heissen «Inhalt» und «Metadatenquellen», die Bibliotheksarten «Video», «Hörbücher», «Podcasts», die Einträge «TMDB», «TVDB», «AniList», «Lokale Dateien». Zwei Bauteile in `web/src/ui`: «Metadatenquellen» (Häkchen, Sortierknöpfe, Hinweise aus `listMetadataProviders`) und «Ordnerliste» (nummerierte Karten, «+ Weiteren Ordner hinzufügen», Absendeknopf zählt mit). Der Client prüft keine Regel (Regel 12); er bietet Unmögliches nur nicht an, der 422 bleibt der Riegel |

## Was von ADR 0006 stehen bleibt

ADR 0006 hat zwei Alternativen verworfen, die dieser Entscheid **annimmt**, mit den Gründen, die 0006
damals selbst nannte und die mit einer Anime-Bibliothek jetzt zutreffen:

- **«TVDB als Hauptquelle für Serien»** (wie Sonarr): bessere Folgenzählung bei Anime und Tages-Shows.
  Jetzt je Serienordner wählbar (F-12); die Einwände von 0006 (zweiter Schlüssel, deutsche Texte nur
  aus der Gemeinschaft) bleiben wahr und sind der Grund, warum TMDB die Vorgabe bleibt und
  `[TVDB, TMDB]` die deutschen Lücken aus TMDB füllt.
- **«Beide Anbieter abfragen und zusammenführen»**: die Einwände «doppelte Anfragen bei jedem Titel»
  und «Widersprüche bei Freigaben und Folgennummern» werden einzeln beantwortet: Anfragen gibt es nur
  für angekreuzte Anbieter und nur aus `metadata.fill`, einmal je Lauf (ein Ordner mit `[tmdb]` kostet
  genau so viel wie heute); Folgennummern kommen **allein vom Erkenner**; Freigaben werden nie gesenkt.

**Unverändert aus ADR 0006** bleiben: die Anbieter-Schnittstelle `metadata.Provider`, der Serien-Job
mit einer Transaktion je Staffel, die Trailer-Kette mit Tageskontingent, `AITrailerProvider` mit
`SLOUCHCAT_TRAILER_AI=off`, die Schlüsselregel (je Installation eigene Schlüssel, nur in `deploy/.env`
oder verschlüsselt in den Einstellungen, Redaktionsliste) und die anbieterneutrale Freigabe-Abbildung
(AN-8). TMDB bleibt Vorgabe und Pflicht als Erkenner für Filme (Entscheid 7 / AN-15 präzisiert).

## Was von ADR 0007 stehen bleibt

Alles ausser der Zeile «in einer Bibliothek der Art `podcasts` **ohne Wurzel**»: eine Podcast-Bibliothek
darf jetzt Ordner mit `content: podcast` tragen, und der offene Punkt «lokale Podcast-Ordner
(Audiobookshelf-Downloads) als Bibliothekswurzel» ist gebaut. Feed-Abos ohne Ordner, der 55-Minuten-Abruf,
die Episodenidentität über GUID für Feed-Episoden, das Streamen und Laden über den Server, die
Standard-Altersstufe (AN-2) und der Netzwächter bleiben wie beschrieben; der Netzwächter bekommt mit
`feed_origin` eine zweite Regel, keine Lockerung.

## Spec-Wortlaut, der hier abgelöst wird (ohne die Spec umzuschreiben)

Nach `docs/spec/README.md` wird die Spec nicht umgeschrieben; der neue Wortlaut gilt über Frage 13
und diesen Entscheid:

- **B-1** «Der Admin legt Bibliotheken auf Ordner an (Typ Filme oder Serien)»: der Typ wandert an den
  Ordner («Inhalt»); eine Bibliothek darf Film- und Serienordner zugleich tragen.
- **B-3** «Metadaten und Bilder kommen von TMDB und TVDB»: bleibt erfüllt; AniList kommt als dritte
  Quelle dazu, und die Reihenfolge ist je Ordner wählbar.
- **H-1, H-2**: Hörbuch-Bibliotheken auf Ordnern bleiben; H-2 bekommt einen zweiten Weg (Feed aus der
  `metadata.json` eines Podcast-Ordners), kein Ersatz.
- **AN-2** (Standard-Altersstufe je Audio-Bibliothek) bleibt; sie hängt an der Bibliothek, nie am
  Ordner; Ordner-Podcasts bekommen sie wie Bücher (`rating_source 'library'`).
- **AN-15** (eigene Schlüssel je Installation; TMDB Hauptquelle, TVDB optional): TMDB bleibt Vorgabe
  und Pflicht als Erkenner für Filme; für Serien ist TVDB als Erkenner wählbar.
- **C-5** (Fremd-Clients): die Anhebung auf 1.3.0 ist brechend und steht wörtlich in den
  Release-Notes.
- **K-3** (Altersfreigaben): nie gesenkt durch eine Anbieterreihenfolge.

## Restrisiken

- **Das Fenster zwischen Match und Auffüllen.** Zwischen `UpdateItemMatch` und dem Lauf von
  `metadata.fill` trägt ein Titel nur die Werte des Erkenners. Für eine Freigabe, die nur ein
  Auffüller kennt (AniList `isAdult`), sieht ein Kinderprofil den Titel in diesem Fenster; es ist so
  lang wie die Warteschlange vor dem Job. Null wäre nur mit Netzaufrufen in der Match-Transaktion zu
  haben.
- **AniList und die Zuordnungsliste:** Rate-Limit (90 je Minute laut Dokumentation, zeitweise
  weniger), Rohadresse, Feldnamen, Lizenz und die TMDB-Abdeckung bei Filmen sind vor Schritt 4 gegen
  die Quelle zu prüfen; die Liste verschmilzt über `anidb_id`, eine TVDB-Serie mit fünf Staffeln hat
  fünf AniList-Einträge, Titelebene nimmt Staffel 1.
- **Ordner- und Feed-Episoden finden nicht immer zusammen:** die Regel «nur bei eindeutigem Paar»
  nimmt in Kauf, dass ein Teil der Episoden doppelt bleibt, sichtbar in `listDuplicates`, nie falsch
  verheiratet.
- **Last durch die Audio-Ordner:** rund 26 000 ffprobe-Starts über NFS beim Erst-Scan des
  Podcast-Ordners; Stunden auf CT 190, gemessen in `docs/perf/bibliotheken.md`.

## Folgen

- **Datenmodell:** Migration `00023_library_roots_providers.sql` erweitert `library_roots` (`content`,
  `providers`) und `items` (`providers`, `anilist_id`, `feed_origin`), legt `item_field_sources` an
  (leer; der erste `metadata.fill` je Titel schreibt die Herkunft nach) und baut `libraries` mit
  `kind IN ('video', 'audiobooks', 'podcasts')` neu, `id` erhalten. Zwei Wächter mit `CHECK`-Tabelle
  brechen den Lauf ab, wenn ein Paar Familie/Inhalt oder Inhalt/Erkenner nicht passt. Die Sicht
  `visible_items` liest `libraries` nicht und bleibt stehen. `TestFrozenStatesMigrate` fährt 00023
  über alle eingefrorenen Stände.
- **Bestand:** eine Filmbibliothek wird eine Video-Bibliothek mit einem Filmordner `["tmdb"]`, eine
  Serienbibliothek eine mit einem Serienordner `["tmdb","tvdb"]` (die heutige Reihenfolge). Kein Scan,
  kein neuer Match. Marcos Zielstruktur entsteht danach in der Oberfläche.
- **Scanner:** liest je Wurzel (`walkRoot` bekommt die Wurzel statt der Bibliothek), ein Titelindex
  je Titelart, `wanted`, `scannedExt`, Batchgrösse und `QueuePendingSegments` folgen dem Inhalt der
  Wurzel. Die Webhook-Pfadumsetzung bleibt unverändert.
- **Metadaten:** `match` wählt den Erkenner nach dem ersten Eintrag von `items.providers`; neuer Job
  `metadata.fill` in `server/internal/metadata/fill.go`; `anilist.go` und `anime_lists.go` neben
  `tmdb.go` und `tvdb.go`, kein neues Paket.
- **Zuschauer:** nichts ändert sich. Kein `scoped`-Endpunkt liest `libraries.kind` oder
  `library_roots`; `visible_items` prüft Rechte über `library_id` und Alter über die Titelart. Eine
  Bibliotheksnavigation für Zuschauer ist nicht Teil dieses Entscheids.
- **Nicht gebaut und bewusst nicht gebaut:** Sprachen je Ordner, Bildarten, Bildsprache, Bildwahl je
  Titel, Fanart.tv, OMDb, NFO, Rohzertifikate und das Land für die Altersfreigabe (Idee 01, Teil B);
  AniDB, MyAnimeList; ein externer Hörbuch-Anbieter (F-7); eine Podcast-Suche; das Löschen einer
  Bibliothek.
