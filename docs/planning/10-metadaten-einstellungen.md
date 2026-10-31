# Konzept: Metadaten-Einstellungen (Sprachen, Bilder, Freigabeland, Bildwahl je Titel)

- **Status: Entwurf zur Freigabe (31. Oktober 2026), Umsetzung nicht gestartet.** Dieses Papier
  ist **Teil B der Idee 01** (`docs/ideas/01-metadaten-einstellungen.md`): alles, was Konzept 07
  dort ausdrücklich zurückgelassen hat. Marco hat am 29. Oktober 2026 nach einem Jellyfin-Bild
  («Bilder abrufen») gefragt, welche Einstellungen je Metadatenquelle Sinn ergeben, und am
  31. Oktober das Konzept beauftragt. Ein klickbarer Prototyp liegt unter
  `docs/planning/mockups/10-metadaten-prototyp.html` (Abschnitt 7.5). **Schritt 0 (Frage 16 im
  Entscheidungsprotokoll, ADR 0018) und alles Weitere beginnen erst auf Marcos ausdrückliches
  Wort** nach den Antworten in Abschnitt 11.
- **Datum: 31. Oktober 2026, Fassung 1**
- **Wogegen dieses Konzept verankert ist:** `main`, Stand **`525c125`** («Merge pull request
  #56», Release 1.5.0), Vertragsfassung **1.5.0**, letzte Migration **00027**, letzte ADR
  **0017**, letzte Frage **15**. Konzept 07 (Bibliotheken, ADR 0015, `metadata.fill`,
  `item_field_sources`), 08 (Startseite, ADR 0016) und 09 (Hintergrundjobs, ADR 0017,
  `jobs.Registry`) sind gemergt und werden **vorausgesetzt**. Die nächsten freien Nummern sind
  darum **Migration 00028, ADR 0018, Vertrag 1.6.0, Frage 16**. `:Zeile`-Angaben beziehen sich
  auf `525c125`.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
- **Jeder Schritt ist ein eigener Commit, und vor dem Abschluss jedes Schritts ist `task ci`
  grün** (`CLAUDE.md`, «Arbeitsweise»). DoD-Belege sind Befehl **und** Ausgabe.

> **Dieses Papier selbst ändert keine Zeile Code.** Die Arbeit beginnt mit Schritt 0 aus
> Abschnitt 10, einem reinen Doku-Commit.

> **Abgrenzung zu Konzept 07.** Die geordnete Liste der Metadatenquellen **je Ordner**
> (`library_roots.providers`), das feldweise Auffüllen (`metadata.fill`), die Herkunft je Feld
> (`item_field_sources`), TVDB als Erkenner für Serien und AniList sind **gebaut** (ADR 0015).
> Dieses Papier ändert daran nichts: es legt fest, **welche Sprachen** diese Quellen liefern,
> **welche Bilder** in **welcher Sprache und Güte** sie liefern, **welches Land** die Freigabe
> zuerst bestimmt, und gibt dem Administrator die **Bildwahl je Titel**. Die Reihenfolge der
> Quellen bleibt am Ordner; eine zweite Anbieterliste («für Bilder») gibt es nicht (Abschnitt
> 2, Entscheid E-4).

> **Abgrenzung zu Konzept 09.** Wie oft etwas läuft, steht auf der Seite «Hintergrundjobs»
> (`job_schedules`). Dieses Papier legt **keine** Intervalle an; der neue Job `metadata.apply`
> ist eine **Hand**-Jobart im Register aus 09 (`Trigger = manual`) und bekommt dort Fortschritt,
> Pause und Abbruch geschenkt.

---

## 1. Ausgangslage und Problem

### 1.1 Was der Server heute kann

**Sprachen.** Jeder Titel bekommt die Texte in **genau zwei Sprachen**, fest im Code:
`textLangs = []string{"de", "en"}` (`metadata/provider.go:30`). TMDB wird je Sprache einmal
gefragt (`tmdb_provider.go:133` bis `:139`, `tmdbLang` bildet `de` auf `de-DE` ab, `tmdb.go:32`),
TVDB über `tvdbLangs` (`tvdb.go:41`, `deu`/`eng`). `item_texts.lang` trägt den `CHECK (lang IN
('de', 'en'))` (`00003_libraries_items.sql:87`); `profiles.language` ebenso
(`00002_accounts_sessions.sql:32`), und der Vertrag kennt `enum: [de, en]` (`openapi.yaml:7163`).
Die Scoped-Abfragen wählen die Profilsprache, dann `en`, dann `items.title`
(`00003_libraries_items.sql:84`). Die Genres liegen als JSON je Sprache in `items.genres`
(`metadata.go:56`). **Ein Profil kann keine dritte Sprache wählen, und der Server holt keine.**
Marcos Vermutung vom 29. Oktober («wird alles gemäss Profilsprache heruntergeladen») trifft
also nicht zu: es sind immer dieselben zwei Sprachen, das Profil wählt nur die Anzeige.

**Bilder.** Zwei Bildarten, `CHECK (kind IN ('poster', 'backdrop'))` (`00006_images_formats.sql`),
je Titelart aus `itemImageKinds` (`images.go:76`): Film und Serie Poster und Hintergrund, Staffel
Poster, Folge das Standbild als `backdrop`, Hörbuch und Podcast das Cover als `poster`. Die Pfade
kommen aus dem Detail-Abruf des Anbieters (`PosterPath`, `BackdropPath`), und zwar aus dem
Datensatz der **englischen** Sprache (`recordLang`, `tmdb_provider.go:111` bevorzugt `en`); die
Spalte `images.lang` existiert seit M1b und bleibt **leer**. `metadata.fill` entscheidet seit 07
auch die Felder `poster` und `backdrop` je Quelle (`fill.go:539`, `:542`) und schreibt
`items.tmdb_poster_path` und `tmdb_backdrop_path` (`00008_library_complete.sql:26`); TVDB wählt
sein Bild nach `score` (`bestArtwork`, `tvdb.go:430`), ohne Sprache. `image.fetch` lädt das
Original einmal und leitet die Grössen ab (`imageWidths`, `images.go:47`: Poster 200/400/800,
Hintergrund 780/1280/1920, je WebP und JPEG). **Weder Sprache noch Mindestbreite noch Bewertung
des Bildes sind wählbar, und ein Logo gibt es nicht.** Gesperrte Bildarten (`locked_fields`
`poster`, `backdrop`) werden nie überschrieben, nur nachskaliert (`scaleLocked`, `images.go:265`).

**Freigabe.** Die Länderkette ist fest: `ratingCountries = {"DE", "AT", "CH", "GB", "US"}`
(`ratings.go:8`, Entscheid 6, AN-8). `metadata.fill` hebt `own_rating` nur an, senkt nie (K-3,
`fill.go:31`).

**Trailer.** YouTube wird fest für `de` und `en` gesucht (`trailers.go:24`), Kontingent 10'000
Einheiten am Tag, also höchstens 100 Suchen (`youtube.go:25`).

**Einstellungen.** Die Tabelle `settings` trägt seit M8 `public_url`, `update_check_enabled`
und die verschlüsselten Schlüssel (`admin/settings.go:29` ff.); die Seite «Integrationen» zeigt
TMDB-Token, TVDB-Schlüssel und PIN, YouTube und Trakt (`de.json:1459` ff.). Je Bibliothek gibt
es Name, Familie, Ordner (Inhaltsart, Metadatenquellen) und die Altersvorgabe; **nichts zu
Sprachen oder Bildern.**

**Lokale Dateien.** Der Scanner liest NFO (`library/nfo.go:54`, `ParseNFO`: `movie.nfo`,
`tvshow.nfo`, `<stem>.nfo`) für Ids, Plot und Freigabe; Bilddateien neben den Filmen liest
**niemand**. Auf dem NAS liegen sie aber: im Ordner «Filme» (1'789 Filmordner, gezählt am
31. Oktober 2026 unter `/mnt/nerdnas/medien/Filme`) 122 `poster.jpg`, 122 `fanart.jpg`,
119 `logo.png`, 119 `landscape.jpg`, 88 `folder.jpg`, 85 `backdrop.jpg`, dazu `movie.nfo` in
den meisten Ordnern. Das sind Embys Exporte, und für diese 122 Titel hat Marco die Bilder schon
einmal von Hand gewählt.

### 1.2 Was Marco will (29. und 31. Oktober 2026)

Marcos Bild zeigt den Jellyfin-Abschnitt «Bilder abrufen»: Schalter je Bildart (Primär, Art,
Banner, Disk, Logo, Miniaturansicht), «Maximale Anzahl an Hintergründen pro Element» und
«Minimale Breite für herunterzuladende Hintergründe». Seine Fragen: Welche Bilder, welche
Qualität, welche Cover; ob die Sprachen wirklich je Profil geholt werden; welche Einstellungen je
Quelle Sinn ergeben. Seine Vorgabe: «Einfach das, was Sinn macht.»

Die Antwort vom 29. Oktober, die Marco mit «Mach doch mal ein Mockup» und «Ja, Konzept» angenommen
hat, in drei Ebenen: **global** Sprachenliste, Bildsprache je Bildart, Bildarten (Logo neu;
Art, Banner, Disc bewusst nicht), Mindestbreite, Auswahl nach Bewertung, Freigabeland; **je
Quelle** nur, was die Quelle wirklich anders kann (TMDB: Erwachseneninhalte bei der Suche);
**je Bibliothek** überschreibbar nur Sprachen und Bilder, sonst wird die Seite unübersichtlich.

### 1.3 Was Idee 01 schon geklärt hat (Interview vom 20. Oktober 2026)

Idee 01 hat die Grundsatzfragen mit Marco im Interview geklärt und Teil A an Konzept 07
abgegeben. **Teil B**, das Thema dieses Papiers, trägt aus dem Interview folgende Entscheide,
die hier **übernommen** werden, sofern Abschnitt 11 sie nicht neu stellt: Profil gewinnt bei der
Textsprache, die Bibliothek bestimmt, was geholt wird und die Rückfallreihenfolge; ein Bild je
Bibliothek, Art und Titel, für alle Profile gleich, mit Rückfallkette wie «de → ohne Text → en»;
Server-Standard, Bibliothek übersteuert einzelne Felder («wie Server» ist Standard); Logo und
Thumb mit Platz im Web-Design, Banner, Disc, Clearart nur holen und per API liefern; eine
Änderung startet sofort einen Hintergrundjob; Bildwahl je Titel aus allen Kandidaten, gesperrt;
Freigabeland global.

**Zwei Punkte stellt dieses Papier neu** (11.1, F-3 und F-5), weil sich die Lage seit dem
20. Oktober geändert hat: Banner, Disc und Clearart «nur für künftige Clients» holen, widerspricht
Regel 11 (Clients sind ein Non-Goal, und ohne Client ist das reine Cachefüllung); und «Thumb»
für Filme und Serien gibt es ohne Fanart.tv nicht, das Folgen-Standbild liegt heute schon als
`backdrop` da.

---

## 2. Zielbild

Vier Bauteile, in dieser Reihenfolge gebaut:

| Bauteil | Was es tut | Wo es sichtbar ist |
|---|---|---|
| **Sprachen** | Geordnete Liste der Metadaten-Sprachen, Server-Standard mit Abweichung je Bibliothek; Profil wählt die **Anzeigesprache der Metadaten** getrennt von der Oberflächensprache | Verwaltung «Metadaten», Bibliothek, Profil |
| **Bilder** | Bildarten (Poster, Hintergrund fest; Logo, Staffelposter, Folgenbild schaltbar), Bildsprache-Kette je Bildart, Mindestbreite, Wahl nach Bewertung; **lokale Bilddateien** als Quelle vor dem Anbieter; Fanart.tv als zusätzliche Bildquelle (optional, F-4) | Verwaltung «Metadaten», Bibliothek; Zuschauer sieht Logos und passende Poster |
| **Freigabeland** | Welches Land die Freigabe zuerst bestimmt; Rest der Kette wie Entscheid 6 | Verwaltung «Metadaten», Anzeige «12 (CH)» |
| **Bildwahl je Titel** | Kandidaten aller Quellen je Bildart, Klick wählt und sperrt | Titelpflege (`/admin/items/{id}`) |

Dazu ein Job **`metadata.apply`** (Hand-Jobart nach 09), der nach einer Änderung die betroffenen
Titel über `metadata.fill` und `image.fetch` nachzieht, mit Fortschritt auf der Jobseite.

**Entscheide dieses Papiers, die keine Frage an Marco sind** (sie folgen aus Regeln und aus 07):

| Nr. | Entscheid | Grund |
|---|---|---|
| E-1 | **Poster und Hintergrund bleiben Pflicht**, kein Schalter | Ohne sie gibt es keine Kacheln (`PosterCard`), keine Bühne, keine Akzentfarbe |
| E-2 | **Art, Banner, Disc, Clearart werden nicht geholt** | Kein Bildschirm der Web-App zeigt sie; TMDB liefert sie nicht; Clients, die sie bräuchten, sind Non-Goal (Regel 11). Weicht von Idee 01 ab, darum F-3 |
| E-3 | **Ausgelieferte Grössen und Formate bleiben Code** (`imageWidths`, `imageFormats`) | Sie hängen an den Breakpoints der Oberfläche und am Vertrag (`ImageRef`), nicht am Geschmack |
| E-4 | **Keine zweite Anbieterliste «für Bilder».** Bilder kommen in der Reihenfolge der Ordnerliste aus 07, davor `local` (Dateien neben dem Titel), danach Fanart.tv, falls eingeschaltet | Zwei Listen je Ordner wären die Unübersichtlichkeit, die Marco nicht will; `local` zuerst, weil eine Datei neben dem Film eine bewusste Wahl ist; Fanart.tv zuletzt, weil es nur Bilder liefert und keine Ids kennt |
| E-5 | **Mehrere Hintergründe je Titel gibt es nicht** (Jellyfins «maximale Anzahl») | Spotlight und Detailseite zeigen genau einen; mehrere wären Cache ohne Nutzen. Als Idee vermerkt, falls die Bühne je wechseln soll |
| E-6 | **Trailer-Sprachen folgen den Metadaten-Sprachen** | Eine Liste; die Trailer-Kette aus ADR 0006 bleibt |
| E-7 | **Intervalle gehören nach 09**, nicht hierher | `job_schedules` ist die eine Stelle |
| E-8 | **Keine Übersteuerung je Ordner**, nur je Bibliothek | 07 hat die Quellenliste an den Ordner gehängt, weil die Inhaltsart dort liegt; Sprache und Bildsprache sind Geschmack der Sammlung («Anime original, Kinder deutsch»), und die Bibliothek ist die Sammlung |

### 2.1 Was sich für den Zuschauer ändert

- Ein Profil mit Metadatensprache Spanisch sieht spanische Titel und Beschreibungen, sobald der
  Administrator Spanisch in die Liste nimmt; die **Oberfläche** (Knöpfe, Menüs) bleibt Deutsch
  oder Englisch, weil nur diese zwei Sprachpakete existieren (`web/src/locales`). Fehlt ein Text
  in Spanisch, kommt der nächste der Liste (Rang), dann der Dateiname wie heute.
- Poster passen zur Sprache der Bibliothek statt zur englischen Vorgabe; die Kinderbibliothek
  kann textlose Hintergründe und deutsche Poster haben, Anime Originalposter.
- Detailseite und Spotlight zeigen das **Logo** über dem Hintergrund statt des Text-Titels, wenn
  eines da ist; sonst bleibt der Text-Titel (O-2 zur Optik).
- Freigaben zeigen das Land, aus dem die Stufe stammt («12 (CH)»); keine Stufe wird gesenkt
  (K-3), auch nicht durch das neue Land.
- Sonst nichts: keine neue Zuschauerseite, keine Sichtbarkeitsregel ändert sich (Regel 2, 12).

---

## 3. Konflikt mit Spec, Plan und ADRs

| Quelle | Heute | Dieses Papier |
|---|---|---|
| **B-3** (`01-anforderungen-v1.md:87`): «Metadaten und Bilder kommen von TMDB und TVDB in Deutsch und Englisch; Sprache ist je Profil wählbar» | fest de/en | **Erweitert:** Sprachen als Liste, Vorgabe bleibt `["de", "en"]`; Profil wählt weiterhin. Spanisch ist keine Nicht-Ziel-Sprache; die Oberfläche bleibt bei de/en (F-1) |
| **Entscheid 6** (AN-8): Zertifikate in der Reihenfolge DE → AT → CH → GB → US | fest in `ratingCountries` | **Ändert Entscheid 6**: ein gewähltes Land kommt zuerst, der Rest in heutiger Reihenfolge; die Abbildung auf FSK bleibt. Frage 16 hält das fest |
| **ADR 0006** (Trailer-Kette, Schlüsselregel) | YouTube fest de/en | Trailer-Sprachen folgen der Liste (E-6); sonst unverändert. ADR 0018 ergänzt 0006 nur um diesen Satz |
| **ADR 0015** (Quellenliste je Ordner, `metadata.fill`, Herkunft je Feld) | Felder `poster`, `backdrop` je Quelle, Bild ohne Sprache | **Verfeinert, nicht ersetzt:** `fill` entscheidet die Bildfelder neu mit Sprachkette, Mindestbreite und Bewertung (6.2) und kennt zwei Quellen mehr, die **nie in der Ordnerliste stehen** (`local` davor, `fanart` danach, E-4). ADR 0018 ist Teilnachfolger von 0015 im Abschnitt «Bilder» |
| **ADR 0016** (Spotlight, Bühne) | Titel als Text über dem Hintergrund | Logo statt Text, wenn vorhanden; Takt, Pausenregel, Vormalen unverändert |
| **ADR 0017** (Register, Scheduler) | Jobarten mit `Trigger` | Neue Hand-Jobart `metadata.apply`; **kein** neuer Zeitplan |
| **K-2 / K-3** (Altersstufen, nie senken) | unverändert | unverändert; Negativtest «Land senkt nicht» kommt in den Kinderprofil-Vertragstest |
| **B-6** (manuelle Zuordnung, `locked_fields`) | Sperren `poster`, `backdrop` | **Erweitert** um `logo`; die Bildwahl je Titel ist ein Sperreintrag plus gewählte Quelle (4.4) |
| **Regel 4** (Dateizugriff nur über `paths`) | NFO über `paths` | Lokale Bilddateien ebenso; nur Dateien im Titelordner, keine Symlinks nach aussen |
| **Regel 10** (Secrets) | `tvdb_api_key`, `tvdb_pin`, `youtube_api_key` in der Redaction-Liste | `fanart_api_key` und `fanart_client_key` kommen dazu (nur mit F-4) |
| **Regel 11** (Nicht-Ziele) | — | Banner, Disc, Clearart **bewusst nicht** (E-2); Logo ist Teil der Web-App (Detailseite, Spotlight), kein Client-Futter |
| **Idee 01** | Teil B «ausgearbeitet» | wird mit Frage 16 «Übernommen» (Teil B), Issue #8 bekommt den Vermerk; was dieses Papier auslässt (OMDb, Banner/Disc/Clearart, Thumb für Filme), bleibt bei Idee 01 als Rest vermerkt |

---

## 4. Datenmodell

### 4.1 Einstellungen in `settings` (Schlüssel `metadata.*`)

Die Tabelle `settings` ist ein Schlüssel-Wert-Speicher (`00002_accounts_sessions.sql:10`); neue
Schlüssel brauchen keine DDL (wie `00016_admin.sql:17` festhält). Der Server liest sie über
`admin/settings.go` wie `public_url`.

| Schlüssel | Typ | Vorgabe | Bedeutung |
|---|---|---|---|
| `metadata.text_langs` | JSON-Liste ISO 639-1, geordnet, nicht leer, `de` immer enthalten | `["de","en"]` | Sprachen, die jede Quelle für jeden Titel liefert; Reihenfolge = Rang (4.2) |
| `metadata.image_langs.poster` | JSON-Liste aus Sprachcodes und `""` (ohne Text), geordnet | `["de","en",""]` | Rückfallkette je Bildart |
| `metadata.image_langs.backdrop` | ebenso | `["","de","en"]` | Hintergründe meist textlos |
| `metadata.image_langs.logo` | ebenso | `["de","en"]` | Logos ohne Sprache sind selten |
| `metadata.image_kinds` | JSON-Liste aus `logo`, `season_poster`, `episode_still` | `["logo","season_poster","episode_still"]` | Schaltbare Bildarten; `poster` und `backdrop` stehen nie drin (E-1) |
| `metadata.image_min_width.poster` | Zahl | `500` | Kandidaten darunter nur, wenn keiner darüber (6.2) |
| `metadata.image_min_width.backdrop` | Zahl | `1280` | ebenso |
| `metadata.image_by_vote` | `"1"`/`"0"` | `"1"` | Innerhalb der Sprache das bestbewertete Bild statt des ersten |
| `metadata.rating_country` | Ländercode aus `fskTables` | `"DE"` | Das Land, das zuerst gelesen wird; Vorgabe ist das heutige Verhalten |
| `metadata.tmdb_include_adult` | `"1"`/`"0"` | `"0"` | Suche in `searchMetadata` und `matchMovie` mit `include_adult` |
| `metadata.fanart_enabled` | `"1"`/`"0"` | `"0"` | Fanart.tv als Bildquelle nach der Ordnerliste (nur mit F-4) |

Keine neue Tabelle für den Server-Standard: elf Schlüssel, alle über einen Typ
`metadata.Settings` gelesen und mit Vorgaben aufgefüllt (`settings.Load`), damit ein fehlender
Schlüssel nie ein Fehler ist.

### 4.2 `item_texts` ohne Sprachschranke, mit Rang

```
item_texts: CHECK (lang IN ('de','en')) fällt; neue Spalte rank INTEGER NOT NULL DEFAULT 0
```

SQLite kennt kein `ALTER COLUMN`, die Tabelle wird nach Vorlage neu gebaut (Regel 5). `rank` ist
die Stelle der Sprache in der **wirksamen** Liste des Titels zum Zeitpunkt des Füllens (0 = erste).
Die Scoped-Abfragen wählen: Zeile der Metadatensprache des Profils, sonst die Zeile mit dem
**kleinsten Rang**, sonst `items.title` (heute: Profilsprache, sonst `en`, sonst Titel). So lesen
die Scoped-Abfragen **keine Einstellungen** (Regel 2 bleibt eine Abfrage mit `visible_items`), und
der Entscheid liegt im Job, der den Rang schreibt (Regel 12). `items.genres` bleibt ein JSON je
Sprache; die API-Schicht wählt wie heute, mit demselben Rückfall. Der Suchindex
(`00015_discover.sql:195`, `'text', item_id, item_id || ':' || lang`) nimmt jede Sprache auf, wie
heute jede der zwei.

### 4.3 Profil: Metadatensprache getrennt von der Oberflächensprache

```
profiles: ADD COLUMN metadata_language TEXT   -- NULL = wie language
```

`language` bleibt `de`/`en` und steuert die Oberfläche (`web/src/locales`) **und** die
Metadaten, solange `metadata_language` leer ist (so bleibt jedes heutige Profil, wie es ist).
Nimmt der Administrator eine dritte Sprache in die Liste, kann das Profil sie als
`metadata_language` wählen (7.4). Die Auswahl zeigt nur Sprachen, die in **irgendeiner**
wirksamen Liste stehen (Server-Standard oder eine Bibliothek); eine Sprache, die später aus
allen Listen fällt, bleibt am Profil und fällt in der Abfrage auf den Rang zurück. Regel 9: die
Sprachnamen kommen aus `Intl.DisplayNames`, nicht aus Locale-Dateien. Offline (M5) ändert sich
nichts: der Client bekommt die Texte wie heute fertig gewählt.

### 4.4 Bilder: Sprache je Zeile, Logo, Wahl je Titel

```
images:  CHECK (kind IN ('poster','backdrop','logo'))         -- Tabellenneubau nach Vorlage
         lang wird gefüllt ('' = ohne Text, NULL = unbekannt/alt)
         provider TEXT NOT NULL DEFAULT 'tmdb'                 -- tmdb, tvdb, local, fanart, manual
items:   ADD COLUMN logo_path TEXT                             -- wie tmdb_poster_path, aber quellenneutral (4.4.1)
item_image_choices (item_id, kind, provider, source, PRIMARY KEY (item_id, kind))
```

**4.4.1 Pfade und Quellen.** `items.tmdb_poster_path` und `tmdb_backdrop_path` heissen heute
nach TMDB, tragen aber seit 07 auch TVDB-Adressen (`fill.go:539`, `artURL`). Dieses Papier
benennt sie **nicht um** (jede Umbenennung zieht Queries, Fixtures und den eingefrorenen
DB-Stand nach) und fügt `logo_path` ohne Präfix dazu; `image_fetch` liest alle drei. Die Quelle
und die Sprache des gewählten Bildes stehen in `item_field_sources` (Feld `poster`, `backdrop`,
`logo`, `lang` = Bildsprache) und werden auf jede `images`-Zeile kopiert (`provider`, `lang`),
damit die Titelpflege sie ohne zweite Abfrage zeigt.

**4.4.2 Bildwahl je Titel.** Ein Klick des Administrators auf einen Kandidaten schreibt
`item_image_choices (item_id, kind, provider, source)`, wobei `source` die Adresse beim
Anbieter oder der relative Pfad der lokalen Datei ist, und trägt `kind` in `items.locked_fields`
ein (B-6, wie heute `poster`/`backdrop`). `fill` lässt gesperrte Felder aus (`fill.go:22`),
`image.fetch` holt bei einer Wahl genau diese Quelle. «Sperre aufheben» löscht beide Einträge,
der nächste `fill` wählt nach Regel. **Keine Kandidatentabelle**: Kandidaten werden beim Öffnen
der Bildwahl live beim Anbieter geholt (6.4); 60'000 Titel mal dreissig Kandidaten wären
1,8 Millionen Zeilen ohne Nutzen.

**4.4.3 Folgen-Standbild.** Es bleibt `kind = 'backdrop'` an der Folge (heute so, `images.go:81`);
der Schalter `episode_still` in `image_kinds` entscheidet nur, ob es geholt wird. Keine Umbenennung
zu «thumb» (Idee 01 §6): nichts in der Oberfläche unterscheidet die beiden, und `ImageKind` im
Vertrag bliebe sonst nicht additiv.

### 4.5 Abweichung je Bibliothek

```
library_metadata (
  library_id      TEXT PRIMARY KEY REFERENCES libraries(id) ON DELETE CASCADE,
  text_langs      TEXT CHECK (text_langs IS NULL OR json_valid(text_langs)),
  image_langs     TEXT CHECK (image_langs IS NULL OR json_valid(image_langs)),   -- {"poster":[…],"backdrop":[…],"logo":[…]}
  image_kinds     TEXT CHECK (image_kinds IS NULL OR json_valid(image_kinds)),
  updated_at      INTEGER NOT NULL
) STRICT, WITHOUT ROWID;
```

Jede Spalte `NULL` heisst «wie Server». Keine Zeile heisst «alles wie Server». Mindestbreite,
Bewertung, Freigabeland, Erwachseneninhalte und Fanart.tv sind **nur** global (1.2: «sonst wird
die Seite unübersichtlich»; Marco kann mit F-6 mehr verlangen). `metadata.Effective(libraryID)`
löst die Abweichung auf und wird **nur von Jobs** gelesen (`fill`, `image.fetch`, `trailer.find`),
nie von einer Scoped-Abfrage.

### 4.6 Migrationsplan 00028 (Schritte, kein fertiges SQL)

Nach Vorlage `TEMPLATE.sql.txt` (`NO TRANSACTION`, `foreign_keys=OFF`, `foreign_key_check`,
`foreign_keys=ON`), nur vorwärts:

1. `item_texts` neu bauen ohne Sprach-CHECK, mit `rank`; Bestand kopieren, `rank` = 0 für `de`,
   1 für `en` (die heutige Reihenfolge der Vorgabe). Index auf `(item_id, rank)`.
2. `profiles ADD COLUMN metadata_language TEXT` (nullable, kein CHECK: die gültigen Werte kennt
   der Server aus den Listen).
3. `images` neu bauen mit `kind` um `logo` erweitert und `provider` mit Vorgabe `tmdb`;
   Bestandszeilen mit `source_url` auf `artworks.thetvdb.com` bekommen `provider = 'tvdb'`
   (eine `UPDATE … WHERE source_url LIKE`), alle anderen `tmdb`; `lang` bleibt `NULL` (unbekannt),
   der nächste `fill` schreibt sie.
4. `items ADD COLUMN logo_path TEXT`.
5. `library_metadata` und `item_image_choices` anlegen.
6. Keine Rückwärtsfüllung: bestehende Titel behalten ihre Bilder, bis `metadata.apply` läuft
   (8.1). Die Migration selbst läuft in Sekunden (Tabellenneubau von `item_texts` mit 120'000
   Zeilen und `images` mit rund 400'000 Zeilen beim M3-Lasttest; Messung in Schritt 1).
7. Eingefrorener DB-Stand `testdata/db/` nach 00028 (Regel 5); die Sicherung vor der Migration
   macht der Server wie immer mit `VACUUM INTO`.

---

## 5. Auswirkung auf den Vertrag (1.6.0, nicht brechend)

### 5.1 Operationen

| Operation | Pfad | x-visibility | Zweck |
|---|---|---|---|
| `getMetadataSettings` | `GET /admin/settings/metadata` | admin | Die elf Schlüssel mit Vorgaben aufgelöst, dazu die Liste der bekannten Sprachen (TMDB-Sprachliste, gecacht) und je Quelle, welche Sprachen sie kennt |
| `updateMetadataSettings` | `PUT /admin/settings/metadata` | admin | Alles auf einmal; 422 mit Feldfehlern (`langs_empty`, `lang_unknown`, `de_required`, `country_unknown`); Antwort nennt, wie viele Titel die Änderung betrifft und ob ein `metadata.apply` nötig ist |
| `getLibraryMetadata` | `GET /admin/libraries/{id}/metadata` | admin | Die Abweichung der Bibliothek (jedes Feld `null` = wie Server) **und** die wirksamen Werte |
| `updateLibraryMetadata` | `PUT /admin/libraries/{id}/metadata` | admin | ebenso |
| `applyMetadata` | `POST /admin/metadata/apply` | admin | Körper `{libraryIds?: [], what: ["texts","images"]}`; reiht `metadata.apply` ein; Antwort `JobRef` wie `refreshItem` |
| `listItemImageCandidates` | `GET /admin/items/{id}/image-candidates?kind=` | admin | Live beim Anbieter plus lokale Dateien (6.4) |
| `getItemImageCandidatePreview` | `GET /admin/items/{id}/image-candidates/preview?source=` | admin | Nur für lokale Kandidaten (Datei über `paths`), als `http.Handler` (Regel 6) |
| `setItemImage` | `PUT /admin/items/{id}/images/{kind}` | admin | `{provider, source}` wählt und sperrt; reiht `image.fetch` ein |
| `clearItemImage` | `DELETE /admin/items/{id}/images/{kind}` | admin | Sperre und Wahl aufheben; reiht `fill` und `image.fetch` ein |

Neun Operationen, jede mit `x-visibility` und Eintrag in `TestContractCoversEveryOperation`
(`contract_access_test.go`, Regel 7). `getAdminSettings` und «Integrationen» bleiben, wie sie sind;
mit F-4 kommen `fanart` und `fanartClient` in `AdminSettingsKeys` (zwei Felder, additiv).

### 5.2 Schemata

- `ImageKind`: `enum: [poster, backdrop, logo]` (additiv; ein Client, der nur die zwei kennt,
  fragt nie nach `logo`).
- `ItemDetail` und `ItemSummary`: neues optionales Feld `logo: ImageRef` neben `poster` und
  `backdrop`; `getItemImage` liefert `logo` mit den Grössen `w400`, `w800` (6.3). Spotlight
  (`HomePage.hero`) bekommt dasselbe Feld.
- `Profile` und `ProfileUpdate`: `metadataLanguage?: string` (ISO 639-1); `language` bleibt
  `enum: [de, en]`.
- `AdminItem`: `images` je Art mit `provider`, `lang`, `locked`, `chosen` (für die Titelpflege);
  `fieldSources` trägt für `poster`/`backdrop`/`logo` die Sprache mit.
- `Rating`-Anzeige: `ratingSource` nennt schon heute Anbieter und Land (`tvdb:DE`); die
  Oberfläche zeigt neu «12 (CH)» daraus, kein neues Feld.
- Neue Schemata `MetadataSettings`, `LibraryMetadata`, `ImageCandidate` (`provider`, `lang`,
  `width`, `height`, `score`, `source`, `previewUrl`), `ApplyMetadataRequest`.

### 5.3 Ist das eine brechende Anhebung?

Nein: nur neue Operationen, neue optionale Felder, ein erweiterter Enum. Vertrag 1.6.0, Tag
`v1.6.0` nach der Regel «Vertrag = Tag». Eintrag für die Release-Notes: «Metadaten-Einstellungen:
Sprachen als Liste, Bildsprache und Logo, Freigabeland, Bildwahl je Titel; Vertrag 1.6.0 additiv.»

---

## 6. Server

### 6.1 Sprachen durch die Kette

`textLangs` (`provider.go:30`) wird zu `metadata.Effective(libraryID).TextLangs`; `match` und
`fill` fragen jede Quelle je Sprache wie heute (`tmdb_provider.go:133`), nur mit der Liste statt
der Konstante. TVDB: `tvdbLangs` (`tvdb.go:41`) wird zur Tabelle ISO 639-1 → 639-2 für die
Sprachen, die TVDB kennt (`/languages` liefert sie; einmal geholt und gecacht); eine Sprache, die
eine Quelle nicht kennt, wird übersprungen, und `getMetadataSettings` zeigt das («TVDB: kein
Rätoromanisch»). AniList liefert nur `romaji`/`english`/`native` und bleibt bei seiner heutigen
Abbildung (`en`, Originaltitel). `fill` schreibt `rank` je Zeile und **löscht** Zeilen von
Sprachen, die nicht mehr in der wirksamen Liste stehen, ausser sie sind gesperrt oder manuell.
Trailer: `trailer.find` nimmt die Liste statt `de, en` (E-6); mehr Sprachen heisst mehr
YouTube-Suchen am Tag, was die Vorschau in `updateMetadataSettings` nennt («3 Sprachen: bis zu
33 Titel am Tag statt 50»).

### 6.2 Bildwahl im `fill`: Kandidaten, Kette, Breite, Bewertung

`fill` entscheidet die Felder `poster`, `backdrop`, `logo` (`fill.go:539` ff.) neu, je Quelle in
der Reihenfolge aus E-4: **`local` → Ordnerliste (TMDB, TVDB, AniList) → `fanart`**. Jede Quelle
liefert **Kandidaten** `{kind, lang, width, height, score, source}`:

- **TMDB:** der Detail-Abruf bekommt `images` in `append_to_response` mit
  `include_image_language=<Kette ohne "">,null` (eine Anfrage, keine zusätzliche: TMDB liefert
  `posters`, `backdrops`, `logos` mit `iso_639_1` (`null` = ohne Text), `width`, `height`,
  `vote_average`, `vote_count`, `file_path`). Heute nimmt der Server `poster_path` aus dem
  englischen Datensatz; das ist künftig nur noch der Kandidat ohne Treffer in der Kette.
  SVG-Logos (`file_path` endet auf `.svg`) werden **übersprungen** (die Pipeline kennt JPEG und
  PNG, `imageExt`, `images.go:62`; ffmpeg ohne librsvg rastert nicht); O-3 prüft, ob TMDB jedes
  SVG auch als PNG liefert.
- **TVDB:** `artworks` tragen `language` und `score`; `bestArtwork` (`tvdb.go:430`) wird zu
  «alle Artworks der Art als Kandidaten». Logo ist Typ `clearlogo` (Serie 23, Film 25; Nummern
  in Schritt 2 gegen die v4-Doku prüfen, heute kennt der Code nur 2 und 3, `tvdb.go:37`).
- **`local`:** Dateien im Titelordner (Regel 4, über `paths`, nur direkt im Ordner, keine
  Symlinks): `poster.jpg`/`folder.jpg`/`<stem>-poster.jpg` → poster; `fanart.jpg`/`backdrop.jpg`/
  `<stem>-fanart.jpg` → backdrop; `logo.png`/`clearlogo.png` → logo; `seasonNN-poster.jpg` →
  Staffelposter; `<stem>-thumb.jpg` → Folgen-Standbild. Sprache unbekannt (`lang = NULL`), Breite
  aus dem Header (`image.DecodeConfig`), Bewertung keine. `local` steht **vor** der Kette: eine
  Datei neben dem Film gewinnt immer, ausser sie ist kleiner als die Mindestbreite und ein
  Anbieter hat etwas Grösseres (dann zählt die Kette, O-1). Marcos 122 Emby-Ordner (1.1) wären
  damit nach dem ersten `apply` so, wie er sie in Emby gewählt hat.
- **Fanart.tv** (nur mit F-4): `GET /v3/movies/{tmdb_id}`, `GET /v3/tv/{tvdb_id}`, Header
  `api-key`, optional `client-key`; Arten `movieposter`/`tvposter`, `moviebackground`/
  `showbackground`, `hdmovielogo`/`hdtvlogo`, `seasonposter`; Felder `url`, `lang`, `likes`
  (Score). Serien nur mit `tvdb_id`, Filme mit `tmdb_id`; fehlt die Id, wird die Quelle still
  übersprungen (wie `fill.go:34` für TVDB). Rate 1 Anfrage/s, kein Tageskontingent bekannt
  (O-4 prüft die Doku). Neuer Aussenhost in `netguard`.

**Die Wahl je Art:** (1) gesperrte Wahl (`item_image_choices`) gewinnt; (2) sonst `local`, wenn
breit genug; (3) sonst in Quellenreihenfolge der **erste** Kandidat, der die Kette erfüllt: für
jeden Eintrag der Kette (Sprache oder `""`) alle Kandidaten dieser Sprache mit
`width >= min_width`, davon bei `image_by_vote` der mit dem höchsten `vote_average` (Fanart:
`likes`; TVDB: `score`), sonst der erste der Quelle; (4) erfüllt kein Kandidat Kette und Breite,
derselbe Durchlauf ohne Mindestbreite; (5) sonst der Kandidat ohne Sprache, den die Quelle als
Vorgabe nennt (`poster_path`); (6) sonst leer, Art bleibt leer. Die Quelle, Sprache und Adresse
landen in `item_field_sources`; `fill` bleibt idempotent (ADR 0015 Regel 2: eine Quelle weiter
vorn behält das Feld). Ändert sich nur die **Kette** (nicht die Quelle), ist die neue Wahl ein
anderes Bild derselben Quelle; `fill` behandelt das wie «Wert geändert» und `image.fetch` erkennt
die neue Adresse (`source_url`, `images.go:38`).

### 6.3 Pipeline: Logo mit Transparenz

`image.fetch` (`images.go:90` ff.) bekommt `logo` in `itemImageKinds` für Film und Serie, Breiten
`w400`, `w800`, Formate **WebP und PNG** (nicht JPEG: Transparenz), also eine dritte Zeile in
`imageFormats` je Art; `transcode.ImagePNG` kommt dazu (ffmpeg `-c:v png`). Akzentfarbe weiter
nur vom Poster. Staffelposter und Folgen-Standbild hängen am Schalter `image_kinds` (4.1): aus
heisst «nicht holen, vorhandene Dateien beim nächsten `apply` löschen» (8.1), nicht «verstecken».

### 6.4 Kandidaten für die Titelpflege

`listItemImageCandidates` fragt **live**: TMDB `/{movie|tv}/{id}/images` ohne Sprachfilter (alle
Sprachen, damit der Administrator wählen kann), TVDB `artworks` aus dem Serien-Datensatz, lokale
Dateien aus dem Ordner, Fanart.tv falls an. Antwort gecacht 10 Minuten je Titel im Prozess
(`cache.Memory`, wie `listMetadataProviders`). Jeder Kandidat trägt `previewUrl`: beim Anbieter
die `w300`-Adresse (TMDB `t/p/w300`, TVDB und Fanart.tv ihre Vorschau), lokal
`getItemImageCandidatePreview?source=` über `paths` (Regel 4, `http.Handler`, Regel 6).

### 6.5 Der Job `metadata.apply`

Hand-Jobart im Register aus 09 (`jobs.Type`, `registry.go:61`): `Group = metadata`, `Pool =
general`, `Trigger = manual`, Priorität tief (wie `fill`). `Run` nimmt `libraryIds` (leer = alle)
und `what` aus dem Payload, zählt die Titel (`ListFillItems` mit Filter) und reiht **je Titel**
`metadata.fill` (für `texts` und `images`, weil `fill` beide Felder entscheidet) und danach
`image.fetch` ein, mit Idempotenzschlüssel, sodass ein zweites `apply` wartende Jobs nicht
verdoppelt (`RaiseQueuedPriority`, Entscheid vom 26. Oktober 2026). Fortschritt: `Progress`
zählt eingereihte Kinder, nicht erledigte (die Kinder sind eigene Läufe mit eigenem Fortschritt
auf der Jobseite); die Jobseite zeigt den Lauf wie jeden anderen (09, 2.2). Pausiert der
Administrator `metadata.fill` auf der Jobseite, steht auch `apply` still (F-10 aus 09). **Kein
Zeitplan** (E-7). `updateMetadataSettings` reiht **nicht** selbst ein (Idee 01 wollte «sofort»);
die Antwort nennt die Zahl der betroffenen Titel, und die Oberfläche bietet den Knopf «Jetzt
anwenden» an (7.1): Marco soll den NAS-Lauf bewusst auslösen (1'791 Filme mal eine
TMDB-Anfrage je Sprache; mit drei Sprachen rund 5'400 Anfragen, bei 40/10 s TMDB-Rate etwa
23 Minuten; 40'000 Folgen aus dem M3-Lasttest je Staffel, Abschnitt 8).

### 6.6 Freigabeland

`ratingCountries` wird zur Funktion `ratingOrder(country)`: gewähltes Land zuerst, dann die
heutige Kette ohne dieses Land. `FSKFromCertifications` nimmt die Reihenfolge als Parameter.
`fill` hebt weiterhin nur an (K-3): wechselt Marco von DE auf CH und CH kennt den Film mit 12,
bleibt eine gespeicherte 16 stehen; `rating_source` zeigt, woher sie kam, und die Titelpflege
kann sie wie heute von Hand setzen (`setItemRating`). Das Papier sagt das in der Oberfläche
(7.1, Hinweis unter dem Feld), damit niemand einen Fehler sucht.

---

## 7. Oberfläche

### 7.1 Verwaltungsseite «Metadaten»

Neuer Eintrag `metadata` in `adminSections` (`sections.ts`), Gruppe **Bibliothek** (neben
Bibliotheken, Podcasts, Pflege), Pfad `paths.adminMetadata = '/admin/metadata'`, Icon `image`
(vorhanden in `Icon.tsx`?, sonst `tag`). Titel «Metadaten», Karte auf der Übersicht wie jede
Sektion. Aufbau wie «Integrationen» (`Panel`, `Section`, `Field`, `Switch`, `segments`), alles aus
`web/src/ui` (Regel 9):

1. **Geltung** oben: Segmente «Standard | <jede Bibliothek>». Standard zeigt alles; eine
   Bibliothek zeigt den Hinweis «übernimmt den Standard» mit dem Schalter «Von Standard
   abweichen» und darunter **nur** Sprachen und Bilder (4.5), die übrigen Abschnitte sind
   ausgeblendet. Abweichende Bibliotheken tragen in der Segmentleiste einen Punkt.
2. **Sprachen:** geordnete Chips (Pfeile links/rechts, Entfernen; Deutsch nicht entfernbar),
   «+ Sprache» öffnet eine Auswahl aus der TMDB-Sprachliste mit `Intl.DisplayNames`-Namen; unter
   der Liste, welche Quelle welche Sprache nicht kennt.
3. **Bilder:** Schalter Logo, Staffelposter, Folgenbild (Poster und Hintergrund als feste
   Zeilen ohne Schalter, E-1); Bildsprache-Kette je Art als geordnete Chips mit «Ohne Text»;
   Mindestbreite Poster und Hintergrund (Auswahl 0/500/1000/1500/2000 bzw. 0/1280/1920/3840);
   Schalter «Nach Bewertung wählen».
4. **Quellen:** TMDB «Erwachseneninhalte bei der Suche», Fanart.tv Schalter mit Link zum
   Schlüssel auf «Integrationen» (nur mit F-4). Hinweis: «Reihenfolge der Quellen je Ordner
   unter Bibliotheken» mit Link. Keine TVDB-Rolle hier (die steht am Ordner, 07).
5. **Freigabe:** Land (DE, AT, CH, GB, US) mit dem Hinweis aus 6.6.
6. **Anwenden:** «Auf bestehende Titel anwenden» mit Zahl der Titel, Auswahl Texte/Bilder, Knopf
   «Jetzt anwenden» (reiht `metadata.apply` ein, Link auf die Jobseite); letzte Läufe aus dem
   Verlauf von 09.
7. **Speicherleiste** unten, klebend: «Ungespeicherte Änderungen» / «Gespeichert»; nach dem
   Speichern der Hinweis «n Titel betroffen, jetzt anwenden?».

Regel 8: `web/e2e/admin-metadata.spec.ts` (Pfeiltasten durch Segmente und Chips, Tab, Enter,
Zurück, sichtbarer Fokus, keine Fokusfalle in der Sprachauswahl).

### 7.2 Bibliothek

Die Bibliothekskarte (`LibraryCard.tsx`) bekommt eine Zeile «Metadaten: wie Server» oder
«Metadaten: abweichend (Sprachen, Bilder)» mit Link auf `/admin/metadata?library=<id>`; keine
zweite Oberfläche. Der Ordner-Dialog aus 07 bleibt unverändert.

### 7.3 Titelpflege: Bilder

`AdminItemPage` (`/admin/items/{id}`) bekommt einen Abschnitt **Bilder**: je Art das gewählte
Bild mit Quelle, Sprache, Breite und Schloss; Knopf «Andere wählen» öffnet die Kandidaten
(`listItemImageCandidates`) als Kacheln mit Sprache, Breite, Quelle, Bewertung; Klick wählt und
sperrt (`setItemImage`), «Sperre aufheben» (`clearItemImage`). Bestehende Sperren für Poster und
Hintergrund erscheinen dort. Regel 8: Erweiterung von `admin-item.spec.ts` um den Kandidatenraster
(Pfeiltasten im Raster, Enter wählt, Escape schliesst, Fokus kehrt auf «Andere wählen» zurück).

### 7.4 Profil

In den Profil-Einstellungen erscheint unter «Sprache» ein zweites Feld «Sprache der
Beschreibungen» **nur, wenn** eine wirksame Liste mehr als `de`/`en` enthält (sonst bleibt die
Seite, wie sie ist); Werte aus `Intl.DisplayNames`. Der bestehende Tastaturtest der Profilseite
deckt das Feld mit einem Fall «mit dritter Sprache» ab.

### 7.5 Zuschauer: Logo

Detailseite (`ItemMeta.tsx`) und Spotlight (`HomePage.tsx`): ist `logo` gesetzt, steht es als
`<img alt="<Titel>">` an der Stelle des Text-Titels, Breite begrenzt auf die Breite des Textes
(max. 40 % der Bühne, 60 % auf dem Handy), `object-fit: contain`; sonst der Text-Titel. Screen-
Reader lesen den Titel über `alt`. Keine neue Seite, also kein neuer Tastaturtest; die Storybook-
Baselines von 08 bekommen je einen Fall mit Logo.

### 7.6 Klickbarer Prototyp

`docs/planning/mockups/10-metadaten-prototyp.html` ist eine einzelne HTML-Datei ohne
Abhängigkeiten, dunkel, mit den Tokens aus `packages/design-tokens/tokens/` (Farben,
Abstände, Radien, Schrift) und den Mustern der heutigen Verwaltungsseiten (AdminNav, Panel,
Switch, Chips). Sie zeigt die Seite «Metadaten» nach 7.1: Geltung Standard/Bibliothek mit
Abweichung (Anime weicht ab, Filme nicht), Sprachen, Bilder mit Kette je Art, Quellen, Freigabe,
Anwenden, Speicherleiste; unten die offenen Entscheide. Der Prototyp ist **kein** Code für
`web/src` und wird nicht gebaut oder getestet. Dieselbe Fassung ist als privates Artefakt unter
`https://claude.ai/artifact/TGSPUTWogTsfeBZqDzF1oU` erreichbar; Änderungen immer in beide
Kopien.

---

## 8. Leistung

| Frage | Antwort |
|---|---|
| Mehr TMDB-Anfragen? | **Nicht je Sprache mehr als heute**: Bilder kommen über `append_to_response=images` im Detail-Abruf. Je **zusätzlicher** Sprache eine Anfrage je Titel und je Staffel (wie heute für `en`). TMDB-Rate bleibt 40 Anfragen je 10 s (`ratelimit`). |
| Was kostet ein `apply`? | Marcos NAS (1'791 Filme, 3'749 Folgen in «KinderSerien» allein; Serien gesamt in Schritt 5 zählen): Texte mit drei Sprachen rund 5'400 Film-Anfragen (23 min) plus je Staffel eine je Sprache. Bilder: ein Download je **geändertem** Bild, meist Poster (deutsche statt englische), rund 1'800 Originale à 1 MB über das NAS-Cache-Volume. Mit 40'000 Folgen (M3-Lasttest) und Folgenbild an: bis zu 40'000 Standbilder; darum der Schalter `episode_still` und die Zahl vor dem Knopf. |
| Cache-Grösse? | Logo: zwei Breiten mal zwei Formate, PNG-Logos 50 bis 200 KB, also rund 0,5 MB je Film/Serie; bei 2'500 Titeln 1,2 GB. Folgen-Standbilder sind heute schon da (drei Breiten mal zwei Formate, rund 300 KB je Folge: 40'000 Folgen = 12 GB) und werden mit dem Schalter erstmals abschaltbar. Messung in `docs/perf/metadaten.md` (Schritt 7): Cache vor/nach `apply` auf CT 190. |
| Scoped-Abfragen langsamer? | `item_texts` bekommt `rank` und den Index `(item_id, rank)`; die Rückfallabfrage wird von «Profilsprache, sonst en» zu «Profilsprache, sonst min(rank)», gleiche Zeilenzahl je Titel plus eine je weiterer Sprache. Latte: `listItems` p95 auf CT 190 unverändert ±10 % (Messung Schritt 1 mit `docs/perf/measure-home.sh`). |
| Startseite? | Ein Feld `logo` mehr je Spotlight-Titel; die Hülle `GET /` aus 08 (Vormalen, P-10) bekommt das Logo als weiteres `<link rel=preload>`, oder nicht, wenn die Messung aus 08 (2,33 s) kippt (O-2). |
| Kandidaten-Dialog? | Eine Anbieteranfrage je Öffnen und Art, gecacht 10 min; TMDB liefert für populäre Filme über 100 Poster, die Oberfläche zeigt 40 und lädt mehr auf Knopf. |

---

## 9. Sicherheit

### 9.1 Nur Administratoren, nichts Scoped

Alle neuen Operationen sind `admin` (5.1); die Bildauslieferung bleibt `scoped` über
`visible_items`, `logo` läuft über denselben Handler wie `poster` (`getItemImage`). Keine
Scoped-Abfrage liest `settings` oder `library_metadata` (4.2, Regel 2 und 12). Der Kinderprofil-
Vertragstest bekommt den Negativtest «Freigabeland senkt keine Stufe».

### 9.2 Dateien nur über `paths`

Lokale Kandidaten und die Vorschau lesen ausschliesslich Dateien **im Titelordner** über
`paths.Open(root)` relativ zur registrierten Wurzel (Regel 4): kein `..`, keine Symlinks nach
aussen, nur die Namen aus 6.2 (keine freie Dateiangabe im Request: `source` ist der relative Pfad,
der Server prüft ihn gegen die erlaubte Namensliste des Titels). `image.DecodeConfig` liest nur
den Header; die vorhandene Grössenschranke der Pipeline (Original höchstens 20 MB) gilt auch für
lokale Dateien.

### 9.3 Ein neuer Aussenhost, ein neues Geheimnis (nur mit F-4)

Fanart.tv: `webservice.fanart.tv` und `assets.fanart.tv` in `netguard`; `fanart_api_key`,
`fanart_client_key` verschlüsselt in `settings` wie die anderen Schlüssel, in der Redaction-Liste
und im Test (Regel 10; das Muster `client_key` kommt zur Liste). Ohne Schlüssel: Quelle «nicht
konfiguriert», wird nie gefragt (wie TVDB, ADR 0006).

### 9.4 Keine Shell, kein SVG

PNG-Ableitung über `transcode` mit `exec.CommandContext` und Argumentliste (Regel 3). SVG wird
nie geladen und nie gerastert (6.2).

### 9.5 Eingaben prüft der Server

Sprachcodes gegen die TMDB-Sprachliste, `de` Pflicht, Kette ohne Duplikate, Breiten aus der
Auswahl, Land aus `fskTables`, Bibliotheks-Id vorhanden; alles 422 mit Feldfehlern
(`model.ValidationError`). `setItemImage` nimmt `source` nur, wenn es in den Kandidaten des
Titels vorkommt (derselbe Cache wie 6.4, sonst 422 `source_unknown`): kein Administrator lädt
eine fremde Adresse in den Cache.

---

## 10. Schnitt in Schritte

Jeder Schritt ein Commit auf einem Zweig ab `main`, `task ci` grün, DoD-Belege in
`docs/dev/CURRENT_STATUS.md`. Reihenfolge und Abhängigkeiten:

```
0 Papiere → 1 Sprachen (Schema, fill, Profil) → 2 Bilder im fill (Kette, Breite, lokal, Logo-Pipeline)
  → 3 Vertrag 1.6.0, Einstellungen, apply → 4 Seite «Metadaten», Bibliothek, Profil
  → 5 Bildwahl je Titel → [6 Fanart.tv, nur mit F-4] → 7 Abnahme, Messung, Doku
```

### Schritt 0 — Entscheide und Papiere

Frage 16 (F-1 bis F-8, O-1 bis O-4) ins Entscheidungsprotokoll mit dem Vermerk zu Entscheid 6;
ADR 0018 «Sprachenliste, Bildwahl mit Sprachkette und lokalen Dateien, Freigabeland» als
Teilnachfolger von ADR 0015 (Abschnitt Bilder) und Ergänzung von ADR 0006 (Trailer-Sprachen);
Hinweise in 0006 und 0015; Idee 01 auf «Übernommen (Teil B)» mit Restliste, Issue #8
kommentieren, `docs/ideas/README.md` Index; dieses Papier auf den dann aktuellen `main` verankern.
DoD: Doku-Commit, `task ci` grün (Markdown-Lint).

### Schritt 1 — Sprachen

Migration 00028 Punkte 1, 2 (4.6); `metadata.Settings` und `Effective`; `textLangs` ersetzt;
`fill` mit `rank` und Löschregel (6.1); TVDB-Sprachtabelle; Scoped-Abfragen mit `min(rank)`;
`profiles.metadata_language` im Modell; Vertrag nur `Profile.metadataLanguage` (Rest in 3).
Tests: Migrationstest gegen den eingefrorenen Stand, `fill`-Tests mit drei Sprachen, Scoped-
Rückfall mit einem Profil ohne Sprache in der Liste, Vertragstest. Messung `listItems` (8).
DoD: Tests und Messung grün.

### Schritt 2 — Bilder im `fill` und Logo in der Pipeline

Migration 00028 Punkte 3, 4; Kandidaten je Quelle (TMDB `append_to_response=images`, TVDB
Artworks mit Sprache, `local`); Wahl nach 6.2; `logo` in `itemImageKinds`, PNG-Format,
`transcode.ImagePNG`; Schalter `image_kinds`; `item_field_sources` mit Bildsprache. Tests mit
Fixture-Antworten (ein Film mit deutschem und englischem Poster, textlosem Hintergrund,
SVG-Logo und PNG-Logo; ein Ordner mit `poster.jpg`); Negativtest «lokale Datei ausserhalb des
Ordners wird nie gelesen». DoD: Tests grün, ein Titel mit Logo im Cache (Beleg: `ls`).

### Schritt 3 — Vertrag 1.6.0, Einstellungen, `metadata.apply`

OpenAPI (5.1, 5.2) mit `x-visibility`, `task gen`; Handler für Einstellungen und Bibliothek;
Validierung (9.5); Jobart `metadata.apply` im Register (6.5); `ratingOrder` (6.6);
`tmdb_include_adult`. Tests: Vertragstest, Schematest, Register-Test, Negativtest Freigabe
(9.1), `apply` reiht ohne Doppel ein. DoD: `task ci` grün, Vertrag 1.6.0 im Kopf.

### Schritt 4 — Oberfläche: Seite «Metadaten», Bibliothek, Profil, Logo beim Zuschauer

7.1, 7.2, 7.4, 7.5; Locales de/en; Storybook-Baselines; `admin-metadata.spec.ts`; Profiltest-
Fall; Prototyp gegen die gebaute Seite abgleichen. DoD: Tastaturtest grün in Chromium und
Firefox, Regel 9 (Lint) grün.

### Schritt 5 — Bildwahl je Titel

`listItemImageCandidates`, Vorschau-Handler, `setItemImage`, `clearItemImage`, Abschnitt
«Bilder» in der Titelpflege (7.3), Tastaturtest-Erweiterung. DoD: ein Titel mit gewähltem und
gesperrtem Poster überlebt einen `apply` (Beleg).

### Schritt 6 — Fanart.tv (nur mit F-4)

Schlüssel auf «Integrationen», `netguard`, Redaction, `ImageProvider`-Implementierung, Kandidaten
in `fill` und in der Bildwahl, Doku-Prüfung aus O-4. DoD: ein Titel mit Fanart-Logo im Cache;
ohne Schlüssel keine Anfrage (Test).

### Schritt 7 — Abnahme, Messung, Doku

`apply` auf CT 190 für «Filme» und eine Serienbibliothek; Cache vorher/nachher; `listItems`
p95; `docs/perf/metadaten.md`; Abnahme am Handy (Logo in Spotlight und Detail, deutsche Poster,
Kinderbibliothek textlos); Release-Notes; `CURRENT_STATUS.md`; Memory-Notiz. Marco testet mit
seinen 122 Emby-Ordnern, ob die lokalen Bilder gewonnen haben.

**Aufwand (Arbeitstage, Schätzung):** 0: 0,5 · 1: 3 · 2: 4 · 3: 2,5 · 4: 3 · 5: 2,5 · 6: 2 ·
7: 1,5 · **Summe 17 bis 19** (ohne 6: 15 bis 17). Idee 01 schätzte für Teil B rund 20 bis 28;
die Differenz sind OMDb, Banner/Disc/Clearart, Thumb und die Kandidatentabelle, die dieses
Papier weglässt.

---

## 11. Fragen an Marco

### 11.1 Entscheide vor Schritt 0

| Nr. | Frage | Vorschlag |
|---|---|---|
| F-1 | **Sprachen:** Dritte und weitere Sprachen nur für Metadaten (Titel, Beschreibungen, Poster), Oberfläche bleibt Deutsch/Englisch? Oder soll eine neue Sprache auch ein Sprachpaket der Oberfläche bekommen (je Sprache rund 2'300 Texte in `de.json`, Übersetzung und Pflege bei jedem Release)? | **Nur Metadaten.** Ein Profil hat dann zwei Felder: Oberfläche (de/en) und Beschreibungen (Liste). Oberflächensprachen sind ein eigenes Vorhaben |
| F-2 | **Logo:** Logo als neue Bildart, gezeigt auf Detailseite und Spotlight statt des Text-Titels? | **Ja.** Grösster sichtbarer Gewinn; TMDB liefert Logos ohne zweite Anfrage, lokale `logo.png` sind schon da |
| F-3 | **Banner, Disc, Clearart:** Idee 01 wollte sie «nur holen, cachen, per API liefern» für Clients, die es nicht gibt. Streichen (E-2)? | **Streichen.** Regel 11; sie bleiben als Rest bei Idee 01 |
| F-4 | **Fanart.tv** als zusätzliche Bildquelle (Logos und Poster in hoher Güte, braucht einen eigenen Schlüssel, ein weiterer Aussendienst, Schritt 6)? Jetzt, später, nie? | **Jetzt, aber optional** (Schalter aus, ohne Schlüssel nie gefragt): TMDB-Logos sind bei älteren und kleineren Titeln lückenhaft, Fanart.tv ist dafür die Standardquelle in Emby und Jellyfin. Wenn du keinen Schlüssel willst: später, Schritt 6 entfällt, nichts anderes ändert sich |
| F-5 | **Lokale Bilddateien** (`poster.jpg`, `fanart.jpg`, `logo.png` neben dem Film, wie Emby sie schreibt) als Quelle **vor** allen Anbietern? | **Ja.** 122 deiner Filmordner tragen sie; eine Datei neben dem Film ist eine bewusste Wahl. Du kannst je Titel trotzdem anders wählen (Bildwahl) |
| F-6 | **Abweichung je Bibliothek** nur für Sprachen und Bilder (Kette, Bildarten), alles andere global? | **Ja.** Mindestbreite, Bewertung, Land und Quellen-Schalter sind Servergeschmack; mehr Felder je Bibliothek machen die Seite zu dem, was du nicht willst |
| F-7 | **Freigabeland:** Vorgabe bleibt DE (heutiges Verhalten) oder wird CH, weil der Haushalt in der Schweiz steht? Beachte: bestehende Stufen werden nie gesenkt (K-3), die Änderung wirkt auf neue Titel und auf Anhebungen | **DE bleibt Vorgabe**, du stellst auf CH um, wenn du es willst; so ändert ein Update nichts von allein |
| F-8 | **Anwenden:** Nach dem Speichern selbst «Jetzt anwenden» drücken (Vorschlag, mit Zahl der Titel) oder sofort automatisch (Idee 01)? | **Selbst drücken.** Ein Lauf über das NAS dauert bei drei Sprachen rund eine halbe Stunde und lädt Bilder nach; das soll keine Nebenwirkung eines Speicherns sein |

### 11.2 Offene Punkte, die der Bau klärt (Vorschlag steht, Widerspruch willkommen)

| Nr. | Punkt | Vorschlag |
|---|---|---|
| O-1 | Lokale Datei kleiner als die Mindestbreite: trotzdem nehmen? | Nein, dann zählt die Kette; die Datei bleibt als Kandidat in der Bildwahl |
| O-2 | Logo ohne Logo-Datei: typografischer Titel in Logo-Optik oder normaler Text-Titel? Und: Logo in der vorgemalten Hülle von 08 (P-10)? | Normaler Text-Titel (kein Pseudo-Logo); Hülle nur, wenn die 08-Messung hält |
| O-3 | TMDB-SVG-Logos: liefert TMDB zu jedem SVG ein PNG? | In Schritt 2 gegen die Doku prüfen; bis dahin SVG überspringen |
| O-4 | Fanart.tv: Kontingent und Bedeutung des `client-key` (persönlicher Schlüssel = schnellere Sichtbarkeit neuer Bilder, kein Kontingent?) | In Schritt 6 gegen die Doku prüfen, bevor der Schlüssel auf «Integrationen» erscheint |

---

## 12. Risiken

### 12.1 Ein `apply` über das ganze NAS

Marcos Sammlung hängt per NFS am CT (M11: NAS-Stau bei Keyframes und Segmenten, 45 MB/s). Ein
`apply` mit Bildern schreibt rund 1'800 Originale plus Ableitungen in den Cache. Gegenmittel: der
Knopf statt Automatik (F-8), die Zahl vor dem Knopf, tiefe Priorität, Pause auf der Jobseite (09).

### 12.2 Die Kette ändert Bilder, die Marco gewohnt ist

Nach dem ersten `apply` mit `["de","en",""]` wechseln viele Poster von Englisch auf Deutsch.
Das ist das Ziel, aber sichtbar; gesperrte Poster bleiben, und die Bildwahl (Schritt 5) macht
jeden Einzelfall rückgängig. Darum kommt Schritt 5 **vor** dem grossen `apply` auf CT 190
(Schritt 7).

### 12.3 Sprachen, die eine Quelle nicht kennt

TVDB kennt rund 25 Sprachen, TMDB über 180, AniList drei. Ein Titel in Rätoromanisch bleibt bei
den meisten Quellen leer und fällt auf den Rang zurück. Die Seite sagt das vorher (7.1, Punkt 2).

### 12.4 `item_texts` wächst

Je Sprache eine Zeile je Titel, Staffel und Folge; mit fünf Sprachen und 40'000 Folgen 200'000
Zeilen statt 80'000. Der Suchindex (`00015_discover.sql`) wächst mit. Messung in Schritt 1; die
Latte ist `listItems` ±10 %.

### 12.5 Transparente PNGs in der Pipeline

Die Pipeline kennt nur JPEG und WebP als Ausgabe; PNG-Ausgabe und Alphakanal sind neu
(`transcode`). Ein Fixture-Logo mit Alpha im Test; `scaleLocked` muss den Medientyp des
Originals behalten (Idee 01 §6 nennt den Fehler).

### 12.6 Was dieses Papier nicht zählen konnte

Die Zahl der Serien und Staffeln auf dem NAS (nur «KinderSerien» ist aus 09 bekannt), die
Trefferquote von TMDB-Logos für Marcos Titel, die Grösse der Logos in Byte. Schritt 7 misst.

---

## 13. Review-Protokoll

Fassung 1 geht nach dem Schreiben an den OMC-Kritiker; die Befunde und ihre Einarbeitung werden
hier nachgetragen, bevor Marco das Papier liest.
