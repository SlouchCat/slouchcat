# ADR 0018: Sprachenliste, Bildwahl mit Sprachkette und lokalen Dateien, Freigabeland

- Status: angenommen (31. Oktober 2026); **ersetzt teilweise ADR 0015** (Abschnitt «Bilder» der
  Auffüllreihenfolge: die Felder `poster` und `backdrop` werden nicht mehr allein nach dem Rang der
  Ordnerliste entschieden, Abschnitt «Was von ADR 0015 stehen bleibt») und **ergänzt ADR 0006** um
  einen Satz (die Trailer-Kette sucht YouTube in den Metadaten-Sprachen statt fest in `de` und `en`;
  Abschnitt «Was von ADR 0006 stehen bleibt»)
- Quelle: Konzept `docs/planning/10-metadaten-einstellungen.md` (Fassung 2, 31. Oktober 2026,
  Abschnitte 2, 4, 5, 6 und 9); **Frage 16** in `docs/planning/04-entscheidungen.md` mit Marcos
  Antworten F-1 bis F-10 und O-1 bis O-4; Idee 01 (`docs/ideas/01-metadaten-einstellungen.md`,
  Teil B); ADR 0006 (Anbieter-Schnittstelle, Trailer-Kette), ADR 0015 (Quellenliste je Ordner,
  `metadata.fill`, `item_field_sources`), ADR 0016 (Spotlight-Bühne), ADR 0017 (`jobs.Registry`)
- Betroffene Spec-IDs: B-3, B-6, K-2, K-3, AN-8 (Entscheid 6), C-5
- Harte Regeln: 1 (generierter Code nur durch Codegen), 2 (Sichtbarkeit nur über `visibility`; keine
  Scoped-Abfrage liest Einstellungen), 3 (`os/exec` nur in `transcode`), 4 (Dateizugriff nur über
  `paths`), 5 (Migrationen nur vorwärts, nach Vorlage), 6 und 7 (Streaming als `http.Handler`,
  `x-visibility` je Operation), 8 (Tastaturtest je neue Seite), 9 (keine harten Farben und Texte),
  10 (keine Secrets in Logs), 11 (keine Nicht-Ziele), 12 (Entscheid im Server)

## Kontext

Seit M1b holt der Server für jeden Titel die Texte in **genau zwei Sprachen**, fest im Code
(`textLangs = {"de", "en"}`), und die zwei Sprachen stecken an elf Stellen: in den `CHECK`-Klauseln
von `item_texts.lang` und `item_genres.lang`, in den Genre-Triggern, im Go-Modell `genres{DE, EN}`,
in `profiles.language`, in `visibility.Profile`, in `pickTexts`/`pickGenres`, in den Joins der
Bibliotheksseite und der Suche, in den System-Abfragen, im Sortierschlüssel und in der Trailer-Suche.
Ein Profil kann keine dritte Sprache wählen, und der Server holt keine. Bilder kommen aus dem
englischen Datensatz des Anbieters (`PosterPath`, `BackdropPath` aus `recordLang`), ohne Sprache,
Mindestbreite oder Bewertung; `images.lang` ist seit M1b leer; es gibt kein Logo, und die Emby-Exporte
auf dem NAS (`poster.jpg`, `fanart.jpg`, `logo.png` neben dem Film, von Marco selbst gewählt) liest
niemand. Die Länderkette der Altersfreigabe ist fest (`ratingCountries`, Entscheid 6).

Marco hat am 29. Oktober 2026 an einem Jellyfin-Bild («Bilder abrufen») gefragt, welche Einstellungen
je Metadatenquelle Sinn ergeben, und am 31. Oktober 2026 das Konzept 10 beauftragt und alle Fragen
wie vorgeschlagen beantwortet (Frage 16). Konzept 07 (ADR 0015) hat die Quellenliste je Ordner, das
feldweise Auffüllen und die Herkunft je Feld gebaut; dieses Papier ändert daran nichts, sondern legt
fest, **welche Sprachen** diese Quellen liefern, **welche Bilder** in **welcher Sprache und Güte**,
**welches Land** die Freigabe zuerst bestimmt, und gibt dem Administrator die **Bildwahl je Titel**.

Zu entscheiden war:

1. Wo die Sprachliste lebt (Server, Bibliothek, Ordner, Profil) und wie die Laufzeitkette ohne
   Einstellungen in den Scoped-Abfragen auskommt (Regel 2 und 12).
2. Wie die Bildquelle, die Bildsprache und die Wahl je Titel gespeichert werden, wenn
   `item_field_sources` den Schlüssel `(item_id, field, lang)` hat und `lang = ''` dort «Feld ohne
   Sprache» heisst.
3. In welcher Reihenfolge Quellenrang, Sprachkette, Mindestbreite und Bewertung ein Bild wählen.
4. Woher lokale Bilddateien kommen, ohne eine zweite Stelle mit Dateizugriff (Regel 4).
5. Wie «anwenden» läuft, wenn `metadata.fill` bei einem Ordner mit einer Quelle keine Anfrage macht
   und die Texte je Sprache der Match schreibt.
6. Was ein einstellbares Freigabeland für gespeicherte und neue Titel bedeutet (K-3).

Verworfen: (a) **Zwei Anbieterlisten je Server und Bibliothek** (`text_providers`,
`image_providers`, Idee 01 §6): eine zweite Liste je Ordner wäre die Unübersichtlichkeit, die Marco
am 29. Oktober nicht wollte; die Ordnerliste aus 07 bleibt die eine Liste, für Bildfelder fest um
`local` davor und `fanart` dahinter ergänzt (E-4). (b) **Sprachen je Ordner** statt je Bibliothek:
Sprache und Bildsprache sind Geschmack der Sammlung («Anime original, Kinder deutsch»), die
Bibliothek ist die Sammlung (E-8). (c) **Banner, Disc, Clearart, Thumb holen** («nur cachen, per API
liefern», Interview vom 20. Oktober): Clients, die sie bräuchten, sind Non-Goal (Regel 11); ohne
Client ist das Cachefüllung (E-2, F-3). (d) **Neue Oberflächensprachen je Metadatensprache:** rund
2'300 Texte je Sprachpaket mit Pflege bei jedem Release; die Oberfläche bleibt de/en (F-1).
(e) **Bildsprache in `item_field_sources.lang`:** der Schlüssel bedeutet dort «Feld ohne Sprache»;
eine Tabelle `item_images` trägt Quelle, Sprache und Wahl (4.4.1 des Konzepts). (f) **Eine
Kandidatentabelle** (Idee 01 `image_candidates`): 60'000 Titel mal dreissig Kandidaten wären 1,8
Millionen Zeilen ohne Nutzen; Kandidaten werden live geholt und 10 Minuten gecacht. (g) **Quellenrang
vor Sprachkette** (Idee 01 §6): die Sprache ist die sichtbare Güte eines Bildes, die Ordnerliste
regelt Identität und Texte; darum «Kette aussen, Quelle innen». (h) **Automatisch anwenden nach dem
Speichern** (Interview vom 20. Oktober): ein Lauf lädt Bilder über das NAS nach und wechselt sichtbar
Poster; das soll keine Nebenwirkung eines Speicherns sein (F-8). (i) **`apply` reiht `fill` ein:**
`fill` macht bei einem Ordner mit einer Quelle keine Anfrage, und die Texte je Sprache schreibt der
Match; `apply` reiht darum `metadata.match` je Titel ein (Blocker des Kritikers). (j) **ISO 639-1
ohne Region:** verliert `pt-BR`/`pt-PT` und `zh-CN`/`zh-TW`, und die Codes landen in
`item_texts.lang`, wo sie sich später kaum ändern lassen; Sprachcodes sind BCP-47 (E-10). (k) **Die
TMDB-Sprachliste live holen:** macht das Speichern vom TMDB-Schlüssel abhängig; die Liste ist
statisch im Binary. (l) **Mehrere Hintergründe je Titel** (Jellyfins «maximale Anzahl»): Spotlight und
Detailseite zeigen genau einen (E-5). (m) **Option «strengste Stufe aus gewähltem Land und DE»:**
eine Einstellung mehr, deren Wirkung niemand ohne Tabelle versteht (F-7). (n) **Eine Hand-Jobart für
`metadata.apply`:** deren `Run` hat keinen Payload, «Jetzt ausführen» liefe über alles mit Priorität 7
(ADR 0017); darum eine Ereignis-Jobart mit Payload.

## Entscheid

| Gegenstand | Entscheid |
|---|---|
| **Sprachen als geordnete Liste** (F-1, F-9, E-9, E-10) | `settings.metadata.text_langs` ist eine geordnete JSON-Liste aus BCP-47-Codes mit optionaler Region (`de`, `en`, `pt-BR`), Vorgabe `["de","en"]`, geprüft gegen die statische TMDB-Sprachliste im Binary. Die **Serverliste muss `de` enthalten**; eine Bibliothek darf abweichen (`library_metadata.text_langs`, `NULL` = wie Server), solange `de` **oder** `en` drin ist, damit der Rückfall lesbar bleibt. Je Quelle gilt eine Abbildung (TMDB `de` → `de-DE`, TVDB ISO 639-2 aus `/languages`, AniList bleibt `en` und Original); eine Sprache, die eine Quelle nicht kennt, wird übersprungen und auf der Seite genannt. **Keine Übersteuerung je Ordner** (E-8). Die Oberfläche bleibt bei `de`/`en` |
| **`item_texts` ohne Sprachschranke, mit Rang** | Migration 00028 baut `item_texts` und `item_genres` nach Vorlage neu: der `CHECK (lang IN ('de','en'))` fällt, `item_texts.rank` (0 = erste Sprache der wirksamen Liste zum Zeitpunkt des Füllens) kommt dazu, `sort_key` und Index aus 00026 bleiben, die Trigger `search_texts_*` und die Genre-Trigger werden neu angelegt (über die Objektschlüssel von `items.genres` statt `'de'`/`'en'`). `match` und `fill` schreiben `rank` je Zeile; `fill` löscht Zeilen von Sprachen, die nicht mehr in der wirksamen Liste stehen, ausser das Feld ist gesperrt oder die Quelle `manual`. `genres{DE, EN}` wird `map[string][]string` |
| **Laufzeitkette im Go-Code, feldweise** (Regel 2, 12) | Für jedes Feld (Titel, Beschreibung, Tagline, Genres) die erste nicht-leere Zeile in der Kette **Metadatensprache des Profils → Oberflächensprache des Profils (`de`/`en`) → Liste nach `rank`** → `items.title`. Die Scoped-Abfragen lesen **keine** Einstellungen: der Entscheid, welche Sprachen es gibt, liegt im Job, der den Rang schreibt; die Sortierung joint die Profilsprache, sonst `rank = 0` statt `'en'`; Genrefilter, Suche, System-Abfragen (`de` als Profilsprache des Administrators) und `admin.go` nehmen dieselbe Kette. `SortKey` streicht Artikel nur für de/en, andere Sprachen bleiben unverändert |
| **Profil: Metadatensprache getrennt** (F-1) | `profiles.metadata_language TEXT` (`NULL` = wie `language`); `language` bleibt `de`/`en` und steuert Oberfläche und, solange `metadata_language` leer ist, die Metadaten. Wählbar ist die Vereinigung aller wirksamen Listen; `updateProfile` prüft dagegen, `getSession` liefert sie als `availableMetadataLanguages` (die Profilseite ist `account`, ADR 0005). Eine Sprache, die aus allen Listen fällt, bleibt am Profil und fällt in der Kette weiter. Offline (M5) ändert sich nichts |
| **Bildarten** (F-2, F-3, E-1, E-2, E-5) | `images.kind` wird `poster`, `backdrop`, `logo`; `format` um `png` (nur Logo); **Poster und Hintergrund bleiben Pflicht**, Logo, Staffelposter und Folgenbild sind schaltbar (`metadata.image_kinds`, Vorgabe alle drei; aus heisst «nicht holen, vorhandene Dateien beim nächsten `apply` löschen»). Art, Banner, Disc, Clearart und Thumb werden **nicht** geholt; das Folgen-Standbild bleibt `kind = 'backdrop'`; ausgelieferte Grössen und Formate bleiben Code (E-3), Formate neu **je Art** (Logo WebP und PNG, kein JPEG); `transcode.ImagePNG` kommt dazu, `scaleLocked` behält den Medientyp des Originals. Ein Hintergrund je Titel |
| **`item_images` statt zwei Spalten** (4.4.1) | Neue Tabelle `item_images (item_id, kind, provider, lang, source, chosen, updated_at)` ist die **einzige** Quelle für `image.fetch`; Migration 00029 füllt sie aus `tmdb_poster_path`/`tmdb_backdrop_path` (Quelle nach Host, `lang = ''`) und baut `images` neu mit `provider` (Rückfüllung nach Host). Die zwei Spalten an `items` bleiben ungenutzt stehen (Abbau ist ein Nachtrag). `item_field_sources` behält für Bildfelder `lang = ''`; die Bildsprache steht in `item_images.lang` und wird auf jede `images`-Zeile kopiert |
| **Bildwahl im `fill`, feste Reihenfolge** (6.2, E-4, O-1) | Für Bildfelder übergibt `fill` der Entscheidung die Reihenfolge `["local", …Ordnerliste, "fanart"]`; `local` und `fanart` stehen nie in der Ordnerliste. Jede Quelle liefert Kandidaten `{kind, lang, width, height, votes, voteCount, source}` (TMDB über `append_to_response=images` mit `include_image_language=<Vereinigung der Ketten>,null` am Detail-Abruf der `recordLang`-Sprache und an `TVSeason`; TVDB `artworks` mit `language`, `score`; lokal aus `item_local_images`). Wahl je Art: (1) eine **gewählte** Zeile (`chosen = 1`) gewinnt, `fill` rührt das Feld nicht an; (2) ein **lokaler** Kandidat mit `width >= min_width` gewinnt (ein zu kleiner bleibt Kandidat, gewinnt aber nicht von allein, O-1); (3) **Kette aussen, Quelle innen**: je Eintrag der Bildsprache-Kette (`metadata.image_langs.<kind>`, Poster `["de","en",""]`, Hintergrund `["","de","en"]`, Logo `["de","en"]`) alle Anbieter-Kandidaten dieser Sprache mit `width >= min_width`; davon bei `image_by_vote` der mit den meisten Stimmen unter denen mit mindestens drei, Gleichstand nach `voteCount`, dann Quellenrang; sonst der erste in Quellenreihenfolge; (4) erfüllt keiner Kette und Breite: derselbe Durchlauf ohne Mindestbreite; (5) sonst der Kandidat ohne Sprache, den die Quelle als Vorgabe nennt; (6) sonst leer. `fill` bleibt idempotent |
| **Lokale Bilddateien als erste Quelle** (F-5, 4.6, Regel 4) | Der **Scanner** erkennt beim Scan Bilddateien je Titel (Film allein im Ordner: `poster.jpg` > `folder.jpg`, `fanart.jpg` > `backdrop.jpg`, `logo.png` > `clearlogo.png`; mehrere Filme im Ordner nur `<stem>-poster.jpg` usw.; Serie am Ordner der `tvshow.nfo`; Staffel `seasonNN-poster.jpg`; Folge `<stem>-thumb.jpg`; Endungen `jpg`, `jpeg`, `png`, `webp`; `landscape.jpg` nicht) und schreibt sie mit Breite und Höhe aus dem Header in `item_local_images (item_id, kind, rel_path, width, height, size, mtime)`, nur über `paths`, nur Header. `fill` liest die Tabelle, kein Verzeichniszugriff über NFS je Titel. Adresse `file:<rel_path>?<size>-<mtime>` wie `coverSource`. Schalter `metadata.local_images` (Vorgabe an) |
| **Bildwahl je Titel** (B-6, 4.4.2) | Ein Klick auf einen Kandidaten schreibt `item_images` mit `chosen = 1` und trägt `kind` in `items.locked_fields` ein. `image.fetch` unterscheidet **Sperre mit Wahl** (genau diese Quelle holen, dann nie mehr) von **Sperre ohne Wahl** (Altbestand, hochgeladenes Bild: nur nachskalieren). «Sperre aufheben» setzt `chosen = 0` und löscht den Sperreintrag. Kandidaten werden beim Öffnen live geholt (TMDB `/images` ohne Sprachfilter, TVDB `artworks`, `item_local_images`, Fanart.tv falls an), 10 Minuten je Titel und Art im Prozess gecacht; `setItemImage` nimmt `source` nur aus den Kandidaten (abgelaufener Cache: neu holen, dann 422 `source_unknown`). Vorschauen der Anbieter lädt der Browser des Administrators direkt mit `referrerpolicy="no-referrer"`; lokale Vorschauen über `paths` als `http.Handler` |
| **Freigabeland** (F-7, Entscheid 6) | `metadata.rating_country` (Vorgabe `DE`, aus `fskTables`) ist das Land, das zuerst gelesen wird; `ratingCountries` wird `ratingOrder(country)`: gewähltes Land zuerst, dann die heutige Kette ohne dieses Land; `FSKFromCertifications` nimmt weiter das erste Land mit Wert. **Wirkung, ausdrücklich:** ein neuer Titel mit CH zuerst bekommt die CH-Stufe (oft tiefer als DE), ein Kinderprofil mit Grenze 12 sieht ihn; eine gespeicherte Stufe wird nie gesenkt (K-3). Die Oberfläche zeigt «12 (CH)» aus `ratingSource`. Keine Option «strengste aus beiden» |
| **Einstellungen und Abweichung** (F-6, 4.1, 4.5) | Zwölf Schlüssel `metadata.*` in `settings` (Liste in Konzept 4.1), gelesen über `metadata.Settings` mit Vorgaben; `library_metadata (library_id, text_langs, images, updated_at)` mit zwei Spalten für «Sprachen abweichend» und «Bilder abweichend» (Migration 00030). Mindestbreite, Bewertung, lokale Bilder, Freigabeland, Erwachseneninhalte (`metadata.tmdb_include_adult`) und Fanart.tv sind **nur global**. `metadata.Effective(libraryID)` löst die Abweichung auf und wird von `match`, `fill`, `image.fetch`, `trailer.find` und `updateProfile` gelesen, nie von einer Scoped-Abfrage |
| **Job `metadata.apply`** (F-8, 6.5, E-7) | Eine **Ereignis**-Jobart im Register aus ADR 0017 (`Group metadata`, `Pool general`, `Trigger event`, Payload `{libraryIds, what}`), eingereiht nur von `applyMetadata` (`POST /admin/metadata/apply`, 202 mit `jobId` und Titelzahl). Der Handler reiht **je Titel `metadata.match`** ein (Neuabruf wie `refreshItem`; nur so holt ein Ordner mit einer Quelle eine neue Sprache), bei `what = ["images"]` nur `fill` und `image.fetch`; Idempotenzschlüssel je Kind. **Kein Zeitplan**; `updateMetadataSettings` reiht nicht selbst ein, die Antwort nennt die Zahl der betroffenen Titel, die Oberfläche bietet «Jetzt anwenden». Der Elternlauf meldet «n Kinder eingereiht», die Seite zeigt die wartenden Kinder aus `listJobTypes` |
| **Trailer-Sprachen** (E-6) | `trailer.find` sucht YouTube in den primären Untersprachen der wirksamen Liste (`pt-BR` → `pt`, weil `trailers.lang` 2 bis 3 Zeichen erlaubt) statt fest `de`, `en`; die Kette und das Tageskontingent aus ADR 0006 bleiben. Mehr Sprachen heisst weniger Titel am Tag, was die Antwort von `updateMetadataSettings` nennt |
| **Fanart.tv** (F-4, 9.3) | Optional, Schalter `metadata.fanart_enabled` (Vorgabe aus); Schlüssel `fanart_api_key` und `fanart_client_key` verschlüsselt in `settings`, in der Redaction-Liste und im Test (Regel 10); Hosts `webservice.fanart.tv` und `assets.fanart.tv` in `imageSource`. Ohne Schlüssel «nicht konfiguriert», wird nie gefragt. Serien nur mit `tvdb_id`, Filme mit `tmdb_id`; fehlt die Id, wird die Quelle still übersprungen. O-4 (Schlüsselübergabe, Rate, Kontingent) wird in Schritt 6 gegen die Doku geprüft, bevor der Schlüssel auf «Integrationen» erscheint |
| **Vertrag** | **1.6.0, additiv** (C-5): neun Operationen (`getMetadataSettings`, `updateMetadataSettings`, `getLibraryMetadata`, `updateLibraryMetadata`, `applyMetadata`, `listItemImageCandidates`, `getItemImageCandidatePreview`, `setItemImage`, `clearItemImage`), alle `x-visibility: admin`; `ImageKind` um `logo`, `ItemDetail.logo` und `HomePage.heroRow[].logo` optional, `Profile.metadataLanguage`, `Session.availableMetadataLanguages`, `AdminItem.images` mit Quelle, Sprache, Sperre, Wahl; `AdminSettingsKeys` um `fanart` und `fanartClient`. Kein `scoped`-Schema verliert ein Feld. Release-Notes: «Metadaten-Einstellungen: Sprachen als Liste, Bildsprache und Logo, lokale Bilddateien, Freigabeland, Bildwahl je Titel; Vertrag 1.6.0 additiv» |
| **Oberfläche** (7.1 bis 7.5) | Neue Verwaltungsseite `/admin/metadata` («Metadaten», Gruppe Bibliothek, Icon `image`) mit Geltung Standard/Bibliothek, Sprachen, Bilder, Güte, Quellen, Altersfreigabe, Anwenden; Zeile «Metadaten» auf der Bibliothekskarte; Feld «Sprache der Beschreibungen» im Profil, nur wenn mehr als de/en wählbar ist oder ein Wert gesetzt ist; Abschnitt «Bilder» in der Titelpflege; Logo statt Text-Titel auf Detailseite und Spotlight, mit `alt` und Rückfall auf den Text bei Ladefehler (O-2: kein Pseudo-Logo). Regel 8: `admin-metadata.spec.ts`, Erweiterung von `admin-item.spec.ts` |

## Was von ADR 0015 stehen bleibt

Alles ausser der Zeile, dass `poster` und `backdrop` wie die anderen Felder **allein nach dem Rang
der Ordnerliste** entschieden werden: Familie je Bibliothek, Inhaltsart und geordnete Quellenliste je
Ordner, Erkenner und Auffüller, das feldweise Auffüllen der Texte mit `item_field_sources`, die
Freigabe-Ausnahme, AniList, Podcast-Ordner, `feed_origin`, Konvergenz und die Vertragsregeln von
1.3.0. Für die **Bildfelder** `poster`, `backdrop` und neu `logo` gilt ab jetzt die Reihenfolge
`["local", …Ordnerliste, "fanart"]` und die sechsstufige Wahl oben; `item_field_sources` hält die
Herkunft der Bildfelder weiter mit `lang = ''`, und `fill` bleibt idempotent (Regel 2 von ADR 0015).
Die Konvergenzregel bekommt keinen neuen Fall: `metadata.apply` ist der ausdrückliche Weg, bestehende
Titel nachzuziehen.

## Was von ADR 0006 stehen bleibt

Alles. Dieser Entscheid ändert an ADR 0006 genau einen Satz: in der Trailer-Kette heisst Schritt 2
nicht mehr «YouTube Data API v3 für Deutsch und Englisch», sondern «für die primären Untersprachen der
wirksamen Metadaten-Sprachliste». Schnittstelle, Serien-Job, Tageskontingent, `AITrailerProvider`
und die Schlüsselregel gelten weiter; die Redaction-Liste wächst um `fanart_api_key`,
`fanart_client_key` und das Query-Muster `client_key`.

## Spec-Wortlaut, der hier abgelöst wird (ohne die Spec umzuschreiben)

- **B-3** «Metadaten und Bilder kommen von TMDB und TVDB in Deutsch und Englisch; Sprache ist je
  Profil wählbar»: erweitert, nicht gebrochen. Sprachen sind eine Liste mit Vorgabe `["de","en"]`,
  das Profil wählt weiterhin; die Oberfläche bleibt bei Deutsch und Englisch.
- **Entscheid 6 / AN-8** «Zertifikate in der Reihenfolge DE → AT → CH → GB → US»: die Vorgabe bleibt,
  das erste Land ist einstellbar; der Vermerk mit der Wirkung auf neue Titel steht in Frage 16.
- **B-6** (`locked_fields` für `poster`, `backdrop`): erweitert um `logo`; die Bildwahl je Titel ist
  Sperre plus gewählte Quelle.
- **K-3**: unverändert für gespeicherte Stufen; der Kinderprofil-Vertragstest bekommt die Fälle
  «Freigabeland senkt keine gespeicherte Stufe» und «Neuzugang mit CH zuerst bekommt die CH-Stufe».

## Restrisiken

- **Die zwei Sprachen stecken an elf Stellen.** Eine vergessene Stelle fällt nicht als Fehler auf,
  sondern als englischer Text in einem spanischen Profil. Gegenmittel: ein Test, der mit einem Profil
  in der dritten Sprache jede Scoped- und System-Operation aufruft und auf die dritte Sprache prüft.
- **Ein `apply` über das ganze NAS** schreibt rund 1'800 Originale plus Ableitungen in den Cache
  (M11: NAS-Stau bei 45 MB/s). Gegenmittel: der Knopf statt Automatik, die Zahl vor dem Knopf, tiefe
  Priorität, Pause auf der Jobseite.
- **Die Kette ändert Bilder, die Marco gewohnt ist.** Nach dem ersten `apply` mit `["de","en",""]`
  wechseln viele Poster von Englisch auf Deutsch; gesperrte bleiben, und die Bildwahl je Titel kommt
  **vor** dem grossen Lauf auf CT 190. Embys automatische Bilder im Ordner könnten den Effekt
  umkehren; darum der Schalter «Lokale Dateien».
- **Sprachen, die eine Quelle nicht kennt** (TVDB einige Dutzend, TMDB über 180, AniList zwei): ein
  Titel bleibt dort leer und fällt in der Kette weiter; die Seite sagt das vorher.
- **`item_texts` und `item_genres` wachsen** je Sprache um eine Zeile je Titel; Latte
  `listLibraryItems` p95 ±10 % (Messung in Schritt 1).
- **Transparente PNGs in der Pipeline** sind neu (PNG-Ausgabe, Alphakanal, `scaleLocked` mit
  Medientyp); ein Fixture-Logo mit Alpha im Test.
- **SVG-Logos von TMDB** werden nie gerastert (Regel 3, kein librsvg); O-3 prüft in Schritt 2, ob
  TMDB zu jedem `.svg` ein `.png` liefert, sonst werden sie übersprungen.

## Folgen

- **Drei Migrationen**, eine je Schritt, nur vorwärts, nach Vorlage, mit eingefrorenem Stand in
  `testdata/db/` danach: 00028 (`item_texts` und `item_genres` neu, `rank`, Trigger,
  `profiles.metadata_language`), 00029 (`images` neu mit `logo`, `png`, `provider`; `item_images`
  aus den zwei Spalten gefüllt; `item_local_images` leer), 00030 (`library_metadata`). Keine
  Migration füllt Bilder oder Texte nach.
- **Paket `metadata`** wächst um `Settings`, `Effective`, die Kandidaten je Quelle, die sechsstufige
  Wahl, `ratingOrder`, den Handler `metadata.apply` und optional `fanart.go`; `library` um die
  Bilddateien im Scanner; `transcode` um `ImagePNG`; `visibility` um die feldweise Kette mit
  Metadatensprache.
- **Vertrag 1.6.0, additiv**, Tag `v1.6.0` nach der Regel «Vertrag = Tag».
- **Zwei Seiten in der Wirkung** (Regel 8): `web/e2e/admin-metadata.spec.ts` neu,
  `admin-item.spec.ts` erweitert, `NAV_PAGES` in `admin-home.spec.ts`.
- **Keine Nicht-Ziele** (Regel 11): Banner, Disc, Clearart, Thumb, OMDb, NFO als Textquelle,
  Rohzertifikate und `item_provider_state` bleiben als Rest bei Idee 01 vermerkt.
- Umsetzung in acht Schritten (0 bis 7) nach Konzept Abschnitt 10; Vertrag ab Schritt 1,
  Oberfläche in Schritt 4, Bildwahl in Schritt 5, Fanart.tv in Schritt 6, Messung und Abnahme auf
  CT 190 in Schritt 7.
