# ADR 0011: Spotlight als wählbare Trakt-Liste auf der Startseite

- Status: vorgeschlagen (25. Oktober 2026); auf «angenommen» mit Marcos Freigabe
- Quelle: Plan `docs/planning/05-plan-spotlight.md` (Meilenstein M11), Frage 11 vom 25. Oktober 2026
  in `docs/planning/04-entscheidungen.md` (Antworten S-1 bis S-11 und die fünf Folgeentscheide);
  Mockups F/J (Startseite), H (Admin Spotlight), I (Admin Cat's Empfehlungen), K (Admin Startseite)
- Betroffene Spec-IDs: E-1, E-2, E-7, E-9, I-7, W-10, A-2, C-5, K-3
- Löst teilweise ab: ADR 0008, Zeile 40 (Karussell nur mit Picks) und Zeile 42 (vier feste Listen,
  höchstens 20, ausgeblendet unter 5)
- Harte Regeln: 1 (generierter Code), 2 (Sichtbarkeit), 4 (`paths`), 5 (Migrationen nur vorwärts),
  6 (strict server), 7 (`x-visibility`), 8 (Tastaturtest), 9 (Farben und Texte), 10 (Logs),
  11 (keine Non-Goals), 12 (Entscheid im Server)

## Kontext

Die Startseite trug seit M7 acht feste Reihen, darunter **zwei** aus Trakt: «Trending bei Trakt»
und «Beliebt bei Trakt». Beide waren im Code festgeschrieben — welche Listen abgerufen werden
(`traktLists` in `server/internal/metadata/trakt.go`), wie viele Titel erscheinen
(`visibility.MaxRowItems = 20`) und ab wann eine Reihe verschwindet (`discover.TraktMinHits = 5`).
Marco konnte sie ordnen und ausblenden, sonst nichts.

Die Bühne der Seite war das Hero-Karussell mit Cat's Empfehlungen (E-1, Spec Zeile 114): fest
zuoberst, fest an den Schlüssel `picks` gebunden, im Client gesucht
(`web/src/features/catalog/HomePage.tsx`) und im Server vorgemalt
(`server/internal/api/preload.go`). Dass die Bühne `picks` heisst, stand damit an zwei Stellen und
in keinem Vertrag — ein Fremd-Client (C-5) konnte die Startseite nicht nachbauen.

Marco wollte drei Dinge: eine Bühne aus einer **wählbaren** öffentlichen Trakt-Liste, einzelne
Einträge darin **kuratieren** (ausblenden, verschieben, mit einer Notiz versehen), und eine neue
Reihe «Kürzlich veröffentlicht». Die Kuratierung ist dabei der Kern: eine Notiz muss nach dem
nächtlichen Abruf noch am selben Titel stehen, auch wenn die Liste sich vollständig umsortiert hat.

## Entscheid

Die Startseite bekommt den Reihenschlüssel **`spotlight`**: eine in der Verwaltung wählbare
öffentliche Trakt-Liste, auf die eigene Bibliothek gefiltert, gezeigt als Band mit Backdrop und
einer Posterleiste darunter. `trakt_trending` und `trakt_popular` entfallen; **`released`**
(«Kürzlich veröffentlicht» nach `items.release_date`, Fenster 12 Monate, mit eigenem Index) kommt
dazu. Es bleiben **genau acht Schlüssel** (E-9).

| Gegenstand | Entscheid |
|---|---|
| Reihenschlüssel | `spotlight, picks, continue, recent, released, watchlist, collections, genres`. Zwei raus, zwei rein — `MaxRows = 8`, `minItems`/`maxItems` 8 und `HomeRowSetting.position maximum: 7` bleiben unverändert. Der `CHECK` auf `home_rows.key` verlangt in SQLite einen Tabellen-Rebuild; die Migration 00021 macht ihn |
| Cache der Liste | `spotlight_entries (list_key, media, position, tmdb_id, title, year, fetched_at)`, `WITHOUT ROWID`, je Lauf in **einer** Transaktion ersetzt. Reiner Cache; ein fehlgeschlagener Lauf leert ihn, die Reihe verschwindet, sonst passiert nichts (I-7) |
| Kuratierung | **Getrennt** vom Cache in `spotlight_overrides (list_key, media, tmdb_id, hidden, pinned_rank, note, updated_at)`. Der Schlüssel trägt die **Listenidentität**, damit ein Listenwechsel die Notizen nicht auf fremde Titel kippt, und **nie** eine Position, damit der nächtliche Abruf sie nicht wandern lässt. Kein Fremdschlüssel auf `items`: ein Eintrag ohne Bibliothekstreffer muss ausblendbar sein |
| Ordnungsregel | «An Platz N» ist eine **Einfügung**, keine Sortierordnung — ein `ORDER BY` kann es nicht ausdrücken. SQL liefert die natürliche Ordnung (`ORDER BY e.position, e.media, i.id`) und filtert `hidden`; danach in Go, in **dieser** Reihenfolge: dedup, angeheftete einfügen (`pinned_rank`, 1-basiert), zuletzt auf `item_count` schneiden |
| Mindesttreffer | `min_hits` wird auf die Trefferzahl **nach dem Dedup und vor dem Schnitt** geprüft. Das ist AN-3 treu («die ersten 20 Treffer … ausgeblendet bei weniger als 5 Treffern») und macht die Kombination «Anzahl 5, Mindesttreffer 10» sinnvoll statt tot |
| Konfiguration | Singleton-Zeile `spotlight_config` mit `list_key`, `list_url`, `media`, `item_count` (5 bis 20), `min_hits` (0 bis 10), `interval_hours` (6 bis 48), `enabled` und `last_manual_refresh`. Startwerte sind die heutigen Konstanten: `trending`, `both`, 20, 5, 24 |
| Veraltung | Nicht mehr die Konstante `TraktMaxAge = 48h`, sondern **2 × `interval_hours`** |
| Bühne im Vertrag | `HomePage.heroRow` (`HomeRowKey`, optional) nennt `spotlight`, **wenn diese Reihe in der Antwort steht**, und fehlt sonst. Der Server entscheidet, der Client zeigt nur (Regel 12). Ohne dieses Feld müsste jeder Client die Zeichenkette `spotlight` als Protokollgeheimnis kennen |
| Stelle der Bühne | **An ihrer Stelle in der Reihenfolge**, nicht zwingend zuoberst. Das Vormalen (`#hero-prepaint`) entsteht nur, wenn sie die **erste** Reihe ist — sonst sässe ein grosses Bild oben, während das Band weiter unten erscheint |
| Cat's Empfehlungen | **Wird nie zur Bühne**, auch nicht als Ersatz. Immer Posterzeile mit Notiz; E-2 inhaltlich unverändert. Fehlt Spotlight, hat die Seite **keine** Bühne — ein zulässiger Zustand |
| `FirstHero` und Prepaint | **Ableitungen** aus `heroRow`, keine zweite Implementierung der Regel. Fehlt das Feld, liefert `FirstHero` `model.ErrNotFound` und es wird nichts vorgemalt |
| Trailer | `ItemRef.hasTrailer` optional, gefüllt **nur** in den Reihen `spotlight` und `picks` (ein `EXISTS` auf höchstens 40 Zeilen je Startseitenabruf). Der Knopf steht nur an der Bühne; der Weg zum Trailer eines Titels in einer Posterzeile bleibt die Detailseite (W-10) |
| Anzahl | Einstellbar, aber **nur bis 20**: `visibility.MaxRowItems` bleibt unangetastet, damit die gemeinsame Klemme von sechs weiteren Reihen nicht wegen einer einzigen steigt |
| Listenarten | Sieben: Trending, Beliebt, Meistgesehen (Woche), Meistgesammelt (Monat), Am meisten erwartet, Kinocharts (nur Filme) und eine öffentliche Liste per Adresse (Medienart als Nachfilter). Der Katalog steht **einmal**, im Server, und reist als Feld `listKinds` in der Antwort von `getSpotlightConfig` mit |
| Watchlists | Eine Watchlist-Adresse wird **serverseitig** mit `422 validation_failed` (`listUrl: invalid_value`) abgelehnt. Ihr Import ist Non-Goal (Spec Zeile 61, Entscheid 9) |
| Admin-Bildweg | `GET /admin/items/{id}/images/{kind}/{size}` (`getAdminItemImage`), systemweit und ohne Profilprüfung, nach Regel 6 ein `http.Handler`. Sie hängt am **Titel**, nicht am Pick, trägt `?v=<Inhalts-Hash>` und **kein** `p`. `AdminPick.posterUrl` zeigt darauf (additiv) |
| Pakete | **Kein neues Go-Paket.** `metadata` holt und schreibt, `trakt` spricht HTTP ohne Datenbank, `visibility` liest über `visible_items`, `discover` ordnet, `admin` schreibt die Konfiguration. `itemsaccess.go` und die `depguard`-Listen in `server/.golangci.yml` bleiben **unverändert** (Regel 2) |
| Vertragsfassung | **1.1.0.** Das Enum `HomeRowKey` verliert zwei Werte: für einen Fremd-Client brechend (C-5). Entfernte Werte werden **nie** wiederverwendet |

## Drivers

1. **Die Startseite bleibt übersichtlich und unter P-10 und P-11** (LCP < 2.5 s, Startseiten-API
   < 300 ms p95). Gewicht 0.40. Entscheidet gegen eine neunte Reihe und gegen eine Anhebung von
   `MaxRowItems`, und für den Index auf `release_date`. *(Nicht mehr «die Seite behält immer einen
   Kopf»: ohne Spotlight hat sie keine Bühne, und das ist Marcos Entscheid.)*
2. **Kuratierung bleibt verlässlich** — eine Notiz steht nach dem Abruf und nach einem
   Listenwechsel noch am selben Titel. Gewicht 0.35. Entscheidet für die getrennte Override-Tabelle
   mit Listenidentität im Schlüssel.
3. **Der Vertrag bleibt ehrlich und prüfbar** — jede Operation mit `x-visibility`, jede im
   Vertragstest, eine Regel mit genau einer Implementierung. Gewicht 0.25. Entscheidet für die
   neuen Schlüsselnamen, für `HomePage.heroRow` und für die Anhebung auf 1.1.0.

## Alternativen

| Alternative | Warum nicht |
|---|---|
| Beide Trakt-Schlüssel umwidmen (A2): `trakt_trending` = Spotlight, `trakt_popular` = Kürzlich veröffentlicht | Kein Rebuild, Enum unverändert, keine Vertragsanhebung — aber `trakt_popular` für eine Reihe nach `release_date` ist eine dauerhafte Falschaussage in Datenbank, Vertrag, Code, Übersetzungen und, seit `heroRow`, in der öffentlichen Antwort. Spart einmalig vier Anweisungen und kostet dauerhaft |
| Schlüssel unverändert lassen, nur den Inhalt konfigurierbar machen (A3) | Die ernsthafteste Alternative: kein Enum-Bruch, kein Rebuild, keine Bühnenregel, Vertrag bliebe bei 1.0.x. Sie scheitert an einer Kette: «Kürzlich veröffentlicht» braucht einen Schlüssel → ein **neunter** ist durch E-9 versperrt (das ist A4) → der Schlüssel muss aus den zwei Trakt-Zeilen kommen → damit trägt A3 den Kern von A2 in sich. **Sie gewänne**, wenn S-1 «nein» **und** S-5 «keine neue Reihe» lautete; Marco hat beide in die andere Richtung beantwortet |
| A3 plus einen neunten Schlüssel, Enum **erweitert** statt getauscht (A4) | Für Fremd-Clients deutlich milder: ein hinzugefügter Wert bricht niemanden, zwei entfernte schon. Verworfen **nicht** aus technischer Not — dass es genau acht sind, steht in `minItems: 8` und ist eine M7-Entscheidung, die Spec sagt «höchstens acht» — sondern wegen **E-9** (Spec Zeile 122) und **Driver 1**: eine neunte Reihe heisst mehr über der Faltkante und eine grössere Antwort bei jedem Startseitenabruf. Dazu bliebe der Schlüsselname falsch (A3-Erbe) |
| `hidden`, `pinned_rank`, `note` als Spalten im Cache | Der Cache wird je Lauf ersetzt; die Kuratierung müsste jedes Mal hinübergerettet werden — genau die Mechanik, die bei `trakt_lists.PRIMARY KEY (list, media, position)` zum Wandern führt |
| Overrides am Bibliothekstitel `item_id` statt an `tmdb_id` | Ein Eintrag **ohne** Bibliothekstreffer liesse sich nicht ausblenden; genau das verlangt Mockup H |
| `pinned_rank` als Sortierschlüssel statt als Einfügung | Zwei Fassungen dieses Plans sind daran gescheitert, in beide Richtungen: `(position + offset)` lässt einen hochgezogenen Eintrag am Gleichstand hängen, und `(pinned_rank IS NULL) ASC, pinned_rank ASC` zieht einen **heruntergeschobenen** Eintrag nach ganz oben. «An Platz N» ist keine Ordnung |
| Cat's Empfehlungen bleibt Karussell (C2) | **Von Marco ausgeschlossen** (25. Oktober 2026): Cat's Empfehlungen wird nie zur Bühne, auch nicht als Ersatz, wenn Spotlight fehlt. `picks` kann kein Band werden, egal wo es steht |
| `FirstHero` liefert nur das Bild, der Client entscheidet die Bühne | Die Regel stünde nirgends im Vertrag, ein Fremd-Client könnte die Startseite nicht nachbauen (C-5), und zwei Implementierungen derselben Regel sind genau das Risiko, vor dem `heroRow` schützt |
| `visibility.MaxRowItems` anheben, damit Spotlight 30 zeigt | Eine gemeinsame Klemme für sechs weitere Reihen wegen einer einzigen anheben: mehr Poster über der Faltkante (P-10) und grössere Antworten überall (P-11), für einen Nutzen, den niemand verlangt hat |
| `ItemRef.hasTrailer` überall füllen (D1) | Ein `EXISTS` auf `trailers` in jeder Liste und in der gepagten Suche; `ItemRef` ist das meistgelesene Schema. Bleibt als Ausstiegsklausel nach einer Messung |
| Trailer-Knopf ohne Flag, immer gezeigt (D2) | Ein toter Knopf an der sichtbarsten Stelle der App |
| Trailer-Knopf auch an den Posterzeilen | **Von Marco ausgeschlossen** (25. Oktober 2026). `PosterCard` kennt keine Knöpfe; ein Knopf je Karte wäre eine neue Tab-Station je Titel, ein Eingriff in die Pfeiltasten-Navigation (C-4) und eine Änderung an `web/src/ui` mit neuen Vergleichsbildern. Der Weg dorthin bleibt die Detailseite (W-10) |
| Das Abfragefenster `2 × n` behalten | Bei `item_count = 20` holte die Abfrage 40 Zeilen; ein auf Rang 1 angehefteter Treffer von natürlicher Position 55 erreichte Go nie — die Anheftung verpuffte lautlos. Die Obergrenze ist jetzt der Cache selbst: höchstens 100 Zeilen je Medienart, also höchstens 200 |
| `released` schliesst aus, was in `recent` steht | Beim Erst-Scan ist alles neu hinzugefügt; die Reihe wäre ausgerechnet beim ersten Eindruck leer |
| Ein eigenes Paket `spotlight` mit `systemq` | Die Allowlists müssten wachsen; Regel 2 bliebe nicht unberührt. ADR 0008 hat dieselbe Alternative schon verworfen |
| Trakt-Konto je Profil, Scrobbling, Watchlist-Import | Non-Goal (Spec Zeile 61, Entscheid 9); die Watchlist-Adresse wird serverseitig abgelehnt |
| Schalter «auch Titel ohne Bibliothekstreffer zeigen» | Non-Goal «eigene Wunschverwaltung» (Spec Zeile 63). In der **Verwaltung** sind sie trotzdem sichtbar |
| Mockup I in einen eigenen Plan verschieben | War der Vorschlag zu S-11 und ist **von Marco verworfen** (25. Oktober 2026): die Admin-Seite wird im selben Zug umgebaut, als Schritt 5b |

## Warum so gewählt

Die Kombination hält alle drei Driver gleichzeitig. `heroRow` im Vertrag beseitigt die zweite
Implementierung der Bühnenregel und hält den Schlüsselnamen aus dem Protokoll: ein Fremd-Client
zeichnet gross, was benannt ist, statt die Zeichenkette `spotlight` kennen zu müssen, und
unterscheidet «Bühne vorhanden» von «Bühne fehlt», ohne aus `items.length` zu schliessen. Gegen
einen leeren Spotlight macht es die Seite **nicht** immun — das ist seit Marcos Entscheid vom
25. Oktober 2026 auch nicht mehr die Absicht.

Der getrennte Override-Schlüssel mit Listenidentität ist die einzige Variante, bei der die
Kuratierung den nächtlichen Abruf **und** einen Listenwechsel überlebt (Driver 2). Der neue
Reihenschlüssel kostet eine Vertragsanhebung, hält dafür Schema, Vertrag und Code an einer
gemeinsamen Wahrheit, die ein Test prüft (Driver 3). Die Begrenzung auf 20 statt 30 Titel kauft für
einen verzichtbaren Regler keine Wirkung auf sechs fremde Reihen ein (Driver 1). Der Zuschnitt auf
bestehende Pakete hält Regel 2 unberührt.

## Folgen

- **Vertrag 1.1.0** mit **sechs** neuen Admin-Operationen: `getSpotlightConfig`,
  `setSpotlightConfig`, `getSpotlightEntries`, `setSpotlightEntryOverride`, `refreshSpotlight` und
  die binäre Route `getAdminItemImage`. `POST /admin/trakt/refresh` entfällt. `TraktStatus` bekommt
  `listKey` und `entries` statt `trending` und `popular`. `HomePage` bekommt `heroRow`, `ItemRef`
  bekommt `hasTrailer` und ein `maxLength: 280` auf `note`, `AdminPick` bekommt `posterUrl`
  (additiv). `HomeRowKey` tauscht zwei Werte. **Für Fremd-Clients brechend**; gehört in die
  Release-Notes.
- **Migration 00021** baut `home_rows` neu, legt `spotlight_config`, `spotlight_entries` und
  `spotlight_overrides` an, übernimmt den eingefrorenen Trending-Stand aus `trakt_lists`, löscht
  `trakt_lists` und legt den Index `items_released` an. Nur vorwärts, keine Down-Migration; die
  Rückkehr läuft über die automatische `pre-migration`-Kopie und A-4.
- **Zwei benannte Verluste.** War `trakt_trending` oder `trakt_popular` ausgeblendet, geht dieses
  `hidden` verloren; die neuen Reihen starten sichtbar (zwei Klicks). Und die Einträge der Liste
  «Beliebt» werden nicht übernommen — ein Cache-Inhalt, kein Datenverlust: der nächste Lauf füllt
  die Tabelle.
- Admin-Oberflächen können Bibliotheksposter zeigen, **ohne** `scoped` zu lesen. Die Spotlight-Seite,
  die Pflegewerkzeuge (`no_images`) und die Titelseite können `getAdminItemImage` ohne Umbau
  mitbenutzen. Die Route wird **nicht** im Service Worker zwischengespeichert: die Verwaltung ist
  kein Offline-Fall, und ein Poster im Cache, das nur der Admin sieht, wäre verschenkter Platz im
  Quota aus O-7.
- **Spec E-1 und E-7 sind überholt**; die Spec bleibt nach `docs/spec/README.md` unverändert, die
  Abweichung steht als Frage 11 im Entscheidungsprotokoll. **AN-3** aus Entscheid 9 ist geändert.
  **P-10** verliert das Wort «Karussell». **E-9 gilt wörtlich** — acht Reihen, keine leeren,
  Reihenfolge ganz die des Hauptbenutzers.
- `discover.TraktMinHits` entfällt; `visibility.TraktMaxAge` wird aus `interval_hours` berechnet;
  `visibility.MaxRowItems` bleibt.
- Der Schema-Test `TestContractNoTraktPerProfile` bekommt `spotlight` in seine Allowlist; seine
  I-7-Absicht — kein Trakt in Profil-, Gast-, Einladungs-, Sitzungs- und Kontoschemas — bleibt
  unverändert.
- Die Verwaltung bekommt eine Seite «Spotlight» in der Gruppe `curation`, ein neues Symbol und
  Texte in beiden Sprachen. Der Trakt-Status zieht von der Seite «Startseite» dorthin; der Schlüssel
  bleibt bei den Integrationen.
- **Kein Geheimnis im Protokoll** (Regel 10): `list_url` wird nicht protokolliert — sie kann einen
  Benutzernamen enthalten. In die Meldung geht nur `list_key`, bei `list_key = 'url'` zusätzlich die
  ersten acht Zeichen eines SHA-256 der Adresse als Unterscheidungsmerkmal. `listurl` kommt in die
  Redaction-Liste und in deren Test.

## Offene Punkte

1. **Ausstiegsklausel D3 → D1**: zeigt die Messung, dass der `EXISTS` auf `trailers` auch in der
   Suche unter 5 ms bleibt, wird `hasTrailer` in einem eigenen Folge-Commit überall gefüllt.
2. **Neuer eingefrorener Datenbankstand** `testdata/db/M11.sql` nach der Freigabe.
3. **Mehrere Spotlight-Listen** (zwei Bänder, Wechsel je Wochentag): ausdrücklich **nicht** Teil
   dieses Entscheids. Das Schema kann zwei Listen nebeneinander halten — `list_key` steht im
   Primärschlüssel —, die Singleton-Konfiguration kann es nicht.
4. **Trennung von «Neu hinzugefügt» und «Kürzlich veröffentlicht»** über ein Zeitfenster auf
   `added_at`, falls die Überlappung stört.
5. **Mehr als 20 Titel je Reihe**: eigener Plan mit Messung über alle Reihen, die `clamp` begrenzt.
