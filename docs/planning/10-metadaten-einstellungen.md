# Konzept: Metadaten-Einstellungen (Sprachen, Bilder, Freigabeland, Bildwahl je Titel)

- **Status: Fassung 2 zur Freigabe (31. Oktober 2026), Umsetzung nicht gestartet.** Dieses Papier
  ist **Teil B der Idee 01** (`docs/ideas/01-metadaten-einstellungen.md`): alles, was Konzept 07
  dort zurückgelassen hat. Marco hat am 29. Oktober 2026 nach einem Jellyfin-Bild («Bilder
  abrufen») gefragt, welche Einstellungen je Metadatenquelle Sinn ergeben, und am 31. Oktober das
  Konzept beauftragt. Der OMC-Kritiker hat Fassung 1 geprüft (1 Blocker, 19 Major, 31 Minor;
  Abschnitt 13); Fassung 2 arbeitet alle ein. Ein klickbarer Prototyp liegt unter
  `docs/planning/mockups/10-metadaten-prototyp.html` (Abschnitt 7.6). **Schritt 0 (Frage 16 im
  Entscheidungsprotokoll, ADR 0018) und alles Weitere beginnen erst auf Marcos ausdrückliches
  Wort** nach den Antworten in Abschnitt 11.
- **Datum: 31. Oktober 2026, Fassung 2 vom selben Tag**
- **Wogegen dieses Konzept verankert ist:** `main`, Stand **`525c125`** («Merge pull request
  #56», Release 1.5.0), Vertragsfassung **1.5.0**, letzte Migration **00027**, letzte ADR
  **0017**, letzte Frage **15**. Konzept 07 (Bibliotheken, ADR 0015, `metadata.fill`,
  `item_field_sources`), 08 (Startseite, ADR 0016) und 09 (Hintergrundjobs, ADR 0017,
  `jobs.Registry`) sind gemergt und werden **vorausgesetzt**. Die nächsten freien Nummern sind
  darum **Migrationen 00028 bis 00030** (eine je Schritt, 4.7), **ADR 0018, Vertrag 1.6.0,
  Frage 16**. `:Zeile`-Angaben beziehen sich auf `525c125`; wo eine Stelle wandern kann, steht
  der Name.
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
> Quellen bleibt am Ordner; eine zweite Anbieterliste «für Bilder», wie Idee 01 sie vorsah,
> gibt es **nicht** (E-4, eine bewusste Abweichung vom Interview).

> **Abgrenzung zu Konzept 09.** Wie oft etwas läuft, steht auf der Seite «Hintergrundjobs»
> (`job_schedules`). Dieses Papier legt **keine** Intervalle an; der neue Job `metadata.apply`
> ist eine **Ereignis**-Jobart im Register aus 09 (6.5) und erscheint dort mit Verlauf.

---

## 1. Ausgangslage und Problem

### 1.1 Was der Server heute kann

**Sprachen.** Jeder Titel bekommt die Texte in **genau zwei Sprachen**, fest im Code:
`textLangs = []string{"de", "en"}` (`metadata/provider.go:30`). `metadata.match` fragt TMDB je
Sprache einmal (`tmdb_provider.go:133` bis `:139`; `tmdbLang` in `tmdb_provider.go:20` bildet
`de` auf `de-DE` ab und reicht Unbekanntes durch), TVDB über `tvdbLangs` (`tvdb.go:42`,
`deu`/`eng`). Die zwei Sprachen stecken an **vielen** Stellen, nicht nur in einer Konstante:

| Stelle | Was dort fest ist |
|---|---|
| `item_texts.lang CHECK (lang IN ('de','en'))` (`00003_libraries_items.sql:87`; seit `00026_library_views.sql:28` mit `sort_key`) | die Zeilen je Titel |
| `item_genres.lang CHECK ('de','en')` und die Trigger `SELECT 'de' … UNION ALL SELECT 'en'` (`00015_discover.sql:93` bis `:125`) | Genres und Genrefilter |
| `genres{DE, EN}` (`match.go:63`, `match.go:957`, `fill.go:555`) | das Go-Modell der Genres |
| `profiles.language CHECK` (`00002_accounts_sessions.sql:32`), `Profile.language enum: [de, en]` (`openapi.yaml:7163`), Prüfung in `auth/profiles.go:269` | das Profil |
| `visibility.Profile` setzt jede andere Sprache auf `de` (`visibility.go:51`) | der Scope |
| `pickTexts`, `pickGenres` mit `fallbackLang = "en"` (`visibility/mapping.go:25` bis `:85`), **feldweise**: Titel, Beschreibung und Tagline einzeln | die Auswahl zur Laufzeit |
| Sortierung und Genrefilter joinen `'en'` und `s.language` (`visibility/library.go:295` ff., `search.go:112`, `:325` bis `:332`) | Bibliotheksseite, Suche |
| `queries/system/{admin,admin_stats,spotlight,collections}.sql`, `admin/admin.go:241`, `:490` | Verwaltung, Spotlight, Sammlungen |
| `SortKey` kennt Artikel nur für de/en (`library/sortkey.go:14` bis `:17`) | Sortierschlüssel |
| `trailers.lang` erlaubt 2 bis 3 Zeichen (`00008_library_complete.sql:48`), YouTube sucht fest `de`, `en` (`trailers.go:24`) | Trailer |

**Ein Profil kann keine dritte Sprache wählen, und der Server holt keine.** Marcos Vermutung
vom 29. Oktober («wird alles gemäss Profilsprache heruntergeladen») trifft nicht zu: es sind
immer dieselben zwei Sprachen, das Profil wählt nur die Anzeige.

**Bilder.** Zwei Bildarten, `CHECK (kind IN ('poster', 'backdrop'))`, zwei Formate, `CHECK
(format IN ('jpeg', 'webp'))` (`00006_images_formats.sql:27`), je Titelart aus `itemImageKinds`
(`images.go:83`): Film und Serie Poster und Hintergrund, Staffel Poster, Folge das Standbild als
`backdrop` (`images.go:87`), Hörbuch und Podcast das Cover als `poster` (Quelle `coverSource`,
`images.go:749` ff., Präfix `file:` mit Grösse und Änderungszeit). Die Pfade kommen aus dem
Detail-Abruf des Anbieters (`PosterPath`, `BackdropPath`), und zwar aus dem Datensatz der
**englischen** Sprache (`recordLang`, `tmdb_provider.go:111` bevorzugt `en`); die Spalte
`images.lang` existiert seit M1b und bleibt **leer**. `metadata.fill` entscheidet seit 07 die
Felder `poster` und `backdrop` je Quelle (`fill.go:539`, `:542`) und schreibt
`items.tmdb_poster_path` und `tmdb_backdrop_path` (`00008_library_complete.sql:26`), die seit 07
auch TVDB-Adressen tragen; `imageSource` (`images.go:157` ff.) kennt nur die Hosts von TMDB und
TVDB und verwirft andere («image path of an unknown host ignored», `:172`). TVDB wählt sein Bild
nach `score` (`bestArtwork`, `tvdb.go:430`), ohne Sprache; das Artwork-Struct (`tvdb.go:329`
ff.) kennt weder `language` noch Breite. `image.fetch` lädt das Original einmal und leitet die
Grössen ab (`imageWidths`, `images.go:50`: Poster 200/400/800, Hintergrund 780/1280/1920, je
WebP und JPEG, `imageFormats` `:56`; `kindComplete` erwartet jede Breite in beiden Formaten).
**Weder Sprache noch Mindestbreite noch Bewertung des Bildes sind wählbar, und ein Logo gibt
es nicht.** Gesperrte Bildarten (`locked_fields` `poster`, `backdrop`) werden nie geholt, nur
nachskaliert (`scaleLocked`, `images.go:271`).

**`fill` bei einer Quelle.** Hat die Ordnerliste **einen** Eintrag (Marcos «Filme»: `[tmdb]`),
macht `fill` **keine** Anfrage, sondern schreibt nur die Herkunft (`fill.go:252`,
`recordIdentifier`); Staffeln und Folgen füllt `fill` nur bei zwei Quellen (`fill.go:40`). Die
Texte je Sprache schreibt der **Match** (`match.go:173`, `:296`, `:320`, `:424`, `:691`,
`:1019`). Wer also «alle Titel mit einer neuen Sprache nachziehen» will, muss den Match neu
laufen lassen, so wie `refreshItem` es für einen Titel tut, nicht den `fill` (6.5).

**Freigabe.** Die Länderkette ist fest: `ratingCountries = {"DE", "AT", "CH", "GB", "US"}`
(`ratings.go:8`, Entscheid 6, AN-8); `FSKFromCertifications` nimmt das **erste Land mit einem
Wert** (`ratings.go:58` ff.). `metadata.fill` hebt `own_rating` nur an, senkt nie (K-3,
`fill.go:31`).

**Trailer.** YouTube wird fest für `de` und `en` gesucht, Kontingent 10'000 Einheiten am Tag,
also höchstens 100 Suchen (`youtube.go:25`).

**Einstellungen.** Die Tabelle `settings` trägt seit M8 `public_url`, `update_check_enabled`
und die verschlüsselten Schlüssel (`admin/settings.go:29` ff.); die Seite «Integrationen» zeigt
TMDB-Token, TVDB-Schlüssel und PIN, YouTube und Trakt (`de.json`, `admin.integrations.keys`).
Je Bibliothek gibt es Name, Familie, Ordner (Inhaltsart, Metadatenquellen) und die Altersvorgabe;
**nichts zu Sprachen oder Bildern.** `listMetadataProviders` liefert je Quelle schon
`languages` (`MetadataProviderInfo`, `openapi.yaml:8195`), heute `de`, `en`.

**Lokale Dateien.** Der Scanner liest NFO (`library/nfo.go:54`, `ParseNFO`: `movie.nfo`,
`tvshow.nfo`, `<stem>.nfo`) für Ids, Plot und Freigabe; Bilddateien neben den Filmen liest
**niemand**. Auf dem NAS liegen sie aber: im Ordner «Filme» (1'789 Filmordner, 1'791 Dateien
laut dem Scan in `CURRENT_STATUS.md`; gezählt am 31. Oktober 2026 unter
`/mnt/nerdnas/medien/Filme`) 122 `poster.jpg`, 122 `fanart.jpg`, 119 `logo.png`,
119 `landscape.jpg`, 88 `folder.jpg`, 85 `backdrop.jpg`, dazu `movie.nfo` in den meisten
Ordnern. Das sind Embys Exporte. **Ob Marco diese Bilder von Hand gewählt hat oder Emby sie
automatisch abgelegt hat, weiss dieses Papier nicht** (F-5 fragt).

### 1.2 Was Marco will (29. und 31. Oktober 2026)

Marcos Bild zeigt den Jellyfin-Abschnitt «Bilder abrufen»: Schalter je Bildart (Primär, Art,
Banner, Disk, Logo, Miniaturansicht), «Maximale Anzahl an Hintergründen pro Element» und
«Minimale Breite für herunterzuladende Hintergründe». Seine Fragen: Welche Bilder, welche
Qualität, welche Cover; ob die Sprachen wirklich je Profil geholt werden; welche Einstellungen je
Quelle Sinn ergeben. Seine Vorgabe: «Einfach das, was Sinn macht.»

Die Antwort vom 29. Oktober, die Marco mit «Mach doch mal ein Mockup» und «Ja, Konzept»
angenommen hat, in drei Ebenen: **global** Sprachenliste, Bildsprache je Bildart, Bildarten
(Logo neu; Art, Banner, Disc bewusst nicht), Mindestbreite, Auswahl nach Bewertung,
Freigabeland; **je Quelle** nur, was die Quelle wirklich anders kann (TMDB: Erwachseneninhalte
bei der Suche); **je Bibliothek** überschreibbar nur Sprachen und Bilder. Diese drei Punkte
gelten als **bestätigt** und stehen in 11.1 nur noch zur Gegenzeichnung (F-2, F-3, F-6).

### 1.3 Was Idee 01 schon geklärt hat (Interview vom 20. Oktober 2026)

Idee 01 hat die Grundsatzfragen mit Marco im Interview geklärt und Teil A an Konzept 07
abgegeben. **Teil B**, das Thema dieses Papiers, trägt aus dem Interview folgende Entscheide,
die hier **übernommen** werden: Profil gewinnt bei der Textsprache, die Bibliothek bestimmt, was
geholt wird und die Rückfallreihenfolge; ein Bild je Bibliothek, Art und Titel, für alle Profile
gleich, mit Rückfallkette wie «de → ohne Text → en»; Server-Standard, Bibliothek übersteuert
(«wie Server» ist Standard); Bildwahl je Titel aus allen Kandidaten, gesperrt; Freigabeland
global.

**Vier Punkte des Interviews stellt dieses Papier neu**, weil sich die Lage seit dem
20. Oktober geändert hat (Konzept 07 ist gebaut, Regel 11 gilt):

| Interview-Entscheid | Hier | Warum, und wo Marco entscheidet |
|---|---|---|
| Banner, Disc, Clearart «nur holen, cachen, per API liefern» | **nicht geholt** | Clients, die sie bräuchten, sind Non-Goal (Regel 11); ohne Client ist das Cachefüllung. F-3 |
| «Logo und Thumb mit Platz im Web-Design» | Logo ja; **Thumb nicht** | Thumb für Filme und Serien gibt es nur von Fanart.tv; «Weiterschauen» nutzt heute den beschnittenen Hintergrund, das Folgen-Standbild liegt schon als `backdrop` da. F-3 |
| Zwei Anbieterlisten (`text_providers`, `image_providers`) je Server und Bibliothek | **eine** Liste je Ordner (07), für Bilder fest ergänzt um `local` davor und `fanart` danach | Zwei Listen je Ordner wären die Unübersichtlichkeit, die Marco am 29. Oktober nicht wollte. E-4; F-5 fragt die lokale Stelle |
| Änderung «startet sofort einen Hintergrundjob» | Knopf «Jetzt anwenden» mit Zahl der Titel | Ein Lauf lädt Bilder über das NAS nach und wechselt sichtbar Poster; das soll keine Nebenwirkung eines Speicherns sein. F-8 |

Was Idee 01 sonst noch für Teil B vorsah und dieses Papier **nicht** baut, bleibt dort als
Rest vermerkt (Schritt 0): OMDb (Texte, Bewertungen), Banner/Disc/Clearart, Thumb, NFO als
**Text**quelle (heute liest der Scanner NFO nur für Ids, Plot und Freigabe), die Rohzertifikate
`item_certifications` (Landwechsel ohne Netz), `item_provider_state` (Zustand je Quelle und
Titel) und die zweite Anbieterliste.

---

## 2. Zielbild

Vier Bauteile, in dieser Reihenfolge gebaut:

| Bauteil | Was es tut | Wo es sichtbar ist |
|---|---|---|
| **Sprachen** | Geordnete Liste der Metadaten-Sprachen (BCP-47, 4.1), Server-Standard mit Abweichung je Bibliothek; Profil wählt die **Anzeigesprache der Metadaten** getrennt von der Oberflächensprache | Verwaltung «Metadaten», Bibliothek, Profil |
| **Bilder** | Bildarten (Poster, Hintergrund fest; Logo, Staffelposter, Folgenbild schaltbar), Bildsprache-Kette je Bildart, Mindestbreite, Wahl nach Bewertung; **lokale Bilddateien** als Quelle vor den Ordnerquellen; Fanart.tv als Bildquelle danach (optional, F-4) | Verwaltung «Metadaten», Bibliothek; Zuschauer sieht Logos und passende Poster |
| **Freigabeland** | Welches Land die Freigabe zuerst bestimmt; Rest der Kette wie Entscheid 6 | Verwaltung «Metadaten», Anzeige «12 (CH)» |
| **Bildwahl je Titel** | Kandidaten aller Quellen je Bildart, Klick wählt und sperrt | Titelpflege (`/admin/items/{id}`) |

Dazu ein Job **`metadata.apply`** (6.5), der nach einer Änderung die betroffenen Titel neu
zuordnen lässt (`metadata.match` wie `refreshItem`, dann `fill` und `image.fetch` wie heute),
mit Verlauf auf der Jobseite.

**Entscheide dieses Papiers, die keine Frage an Marco sind** (sie folgen aus Regeln und aus 07):

| Nr. | Entscheid | Grund |
|---|---|---|
| E-1 | **Poster und Hintergrund bleiben Pflicht**, kein Schalter | Ohne sie gibt es keine Kacheln (`PosterCard`), keine Bühne, keine Akzentfarbe |
| E-2 | **Art, Banner, Disc, Clearart werden nicht geholt** | Regel 11, siehe 1.3; Marco zeichnet in F-3 gegen |
| E-3 | **Ausgelieferte Grössen und Formate bleiben Code** (`imageWidths`, `imageFormats`) | Sie hängen an den Breakpoints der Oberfläche und am Vertrag (`ImageRef`), nicht am Geschmack |
| E-4 | **Keine zweite Anbieterliste «für Bilder».** Bilder kommen aus **`local` → Ordnerliste (TMDB, TVDB, AniList) → `fanart`**; `local` und `fanart` stehen nie in der Ordnerliste, sondern sind für Bildfelder fest davor und dahinter gesetzt (6.2) | Abweichung von Idee 01 §4/§6, Grund in 1.3. `local` zuerst, weil eine Datei neben dem Film eine bewusste Wahl ist (F-5 fragt, ob das für Marcos Emby-Dateien gilt); Fanart.tv zuletzt, weil es nur Bilder liefert und keine Ids kennt |
| E-5 | **Mehrere Hintergründe je Titel gibt es nicht** (Jellyfins «maximale Anzahl») | Spotlight und Detailseite zeigen genau einen; mehrere wären Cache ohne Nutzen. Als Idee vermerkt, falls die Bühne je wechseln soll |
| E-6 | **Trailer-Sprachen folgen den Metadaten-Sprachen** (primäre Untersprache, `pt-BR` → `pt`, weil `trailers.lang` 2 bis 3 Zeichen erlaubt) | Eine Liste; die Trailer-Kette aus ADR 0006 bleibt |
| E-7 | **Intervalle gehören nach 09**, nicht hierher | `job_schedules` ist die eine Stelle |
| E-8 | **Keine Übersteuerung je Ordner**, nur je Bibliothek | 07 hat die Quellenliste an den Ordner gehängt, weil die Inhaltsart dort liegt; Sprache und Bildsprache sind Geschmack der Sammlung («Anime original, Kinder deutsch»), und die Bibliothek ist die Sammlung |
| E-9 | **Jede wirksame Sprachliste enthält mindestens `de` oder `en`** | Sonst kann der Rückfall (4.2) eine Sprache zeigen, die kein Profil lesen kann. Ob darüber hinaus `de` Pflicht ist, fragt F-9 |
| E-10 | **Sprachcodes sind BCP-47 mit optionaler Region** (`de`, `en`, `pt-BR`, `zh-TW`) | TMDB-Übersetzungen sind `xx-YY`; ein nacktes ISO-639-1 verliert `pt-BR`/`pt-PT` und `zh-CN`/`zh-TW`, und die Codes landen in `item_texts.lang`, wo sie sich später kaum ändern lassen (Idee 01 §6 sah das schon vor) |

### 2.1 Was sich für den Zuschauer ändert

- Ein Profil mit Metadatensprache Spanisch sieht spanische Titel und Beschreibungen, sobald der
  Administrator Spanisch in die Liste nimmt; die **Oberfläche** (Knöpfe, Menüs) bleibt Deutsch
  oder Englisch, weil nur diese zwei Sprachpakete existieren (`web/src/locales`). Fehlt ein Text
  in Spanisch, kommt **feldweise** (Titel, Beschreibung, Tagline einzeln, wie heute `pickTexts`)
  die Oberflächensprache des Profils, dann die Liste in ihrer Reihenfolge, dann der Dateiname.
- Poster passen zur Sprache der Bibliothek statt zur englischen Vorgabe; die Kinderbibliothek
  kann textlose Hintergründe und deutsche Poster haben, Anime Originalposter. **Bestehende Titel
  behalten ihre Bilder, bis der Administrator «Jetzt anwenden» drückt** (8.1); neue Titel
  bekommen sofort die neue Wahl (F-10 fragt nach der Vorgabekette).
- Detailseite und Spotlight zeigen das **Logo** über dem Hintergrund statt des Text-Titels, wenn
  eines da ist; sonst bleibt der Text-Titel (O-2 zur Optik).
- Freigaben zeigen das Land, aus dem die Stufe stammt («12 (CH)»). Eine **gespeicherte** Stufe
  wird nie gesenkt (K-3). Ein **neuer** Titel bekommt die Stufe des zuerst gelesenen Landes:
  wer CH wählt, bekommt CH-Stufen, die bei manchen Filmen tiefer sind als die deutsche (F-7
  sagt das ausdrücklich).
- Sonst nichts: keine neue Zuschauerseite, keine Sichtbarkeitsregel ändert sich (Regel 2, 12).

---

## 3. Konflikt mit Spec, Plan und ADRs

| Quelle | Heute | Dieses Papier |
|---|---|---|
| **B-3** (`01-anforderungen-v1.md:87`): «Metadaten und Bilder kommen von TMDB und TVDB in Deutsch und Englisch; Sprache ist je Profil wählbar» | fest de/en | **Erweitert:** Sprachen als Liste, Vorgabe bleibt `["de", "en"]`; Profil wählt weiterhin. Spanisch ist keine Nicht-Ziel-Sprache; die Oberfläche bleibt bei de/en (F-1) |
| **Entscheid 6** (AN-8): Zertifikate in der Reihenfolge DE → AT → CH → GB → US | fest in `ratingCountries` | **Ändert Entscheid 6**: ein gewähltes Land kommt zuerst, der Rest in heutiger Reihenfolge; die Abbildung auf FSK bleibt. Frage 16 hält das fest, mit der Wirkung auf neue Titel (2.1) |
| **ADR 0006** (Trailer-Kette, Schlüsselregel) | YouTube fest de/en | Trailer-Sprachen folgen der Liste (E-6); sonst unverändert. ADR 0018 ergänzt 0006 nur um diesen Satz |
| **ADR 0015** (Quellenliste je Ordner, `metadata.fill`, Herkunft je Feld) | Felder `poster`, `backdrop` je Quelle, Bild ohne Sprache; Fremdquellen stehen in `decideFields` am Ende (`fill.go:163` ff.) | **Verfeinert, nicht ersetzt:** `fill` entscheidet die Bildfelder mit der festen Bildreihenfolge aus E-4, Sprachkette, Mindestbreite und Bewertung (6.2). ADR 0018 ist Teilnachfolger von 0015 im Abschnitt «Bilder» |
| **ADR 0016** (Spotlight, Bühne) | Titel als Text über dem Hintergrund | Logo statt Text, wenn vorhanden; Takt, Pausenregel, Vormalen unverändert |
| **ADR 0017** (Register, Scheduler) | Jobarten mit `Trigger` | Neue Ereignis-Jobart `metadata.apply`; **kein** neuer Zeitplan |
| **K-2 / K-3** (Altersstufen, nie senken) | unverändert | unverändert für gespeicherte Stufen; Negativtest «Land senkt keine gespeicherte Stufe» und Test «Neuzugang mit CH zuerst» im Kinderprofil-Vertragstest |
| **B-6** (manuelle Zuordnung, `locked_fields`) | Sperren `poster`, `backdrop` | **Erweitert** um `logo`; die Bildwahl je Titel ist Sperre plus gewählte Quelle (4.4) |
| **Regel 4** (Dateizugriff nur über `paths`) | NFO über `paths` | Lokale Bilddateien liest der **Scanner** beim Scan (6.2), keine zweite Stelle mit Dateizugriff |
| **Regel 10** (Secrets) | `tvdb_api_key`, `tvdb_pin`, `youtube_api_key` in der Redaction-Liste; Query-Muster `key`, `apikey`, `api_key` | `fanart_api_key` und `fanart_client_key` kommen dazu, und das Query-Muster `client_key` (nur mit F-4) |
| **Regel 11** (Nicht-Ziele) | — | Banner, Disc, Clearart, Thumb **bewusst nicht** (E-2, 1.3); Logo ist Teil der Web-App (Detailseite, Spotlight), kein Client-Futter |
| **Idee 01** | Teil B «ausgearbeitet» | wird mit Frage 16 «Übernommen (Teil B)»; der Rest aus 1.3 bleibt dort vermerkt, Issue #8 bekommt den Vermerk |

---

## 4. Datenmodell

### 4.1 Einstellungen in `settings` (Schlüssel `metadata.*`)

Die Tabelle `settings` ist ein Schlüssel-Wert-Speicher (`00002_accounts_sessions.sql:10`); neue
Schlüssel brauchen keine DDL (wie `00016_admin.sql:17` festhält). Der Server liest sie über
`admin/settings.go` wie `public_url`.

| Schlüssel | Typ | Vorgabe | Bedeutung |
|---|---|---|---|
| `metadata.text_langs` | JSON-Liste BCP-47 (E-10), geordnet, nicht leer, enthält `de` oder `en` (E-9) | `["de","en"]` | Sprachen, die jede Quelle für jeden Titel liefert; Reihenfolge = Rang (4.2) |
| `metadata.image_langs.poster` | JSON-Liste aus Sprachcodes und `""` (ohne Text), geordnet, ohne Duplikate | `["de","en",""]` (F-10) | Rückfallkette je Bildart |
| `metadata.image_langs.backdrop` | ebenso | `["","de","en"]` | Hintergründe meist textlos |
| `metadata.image_langs.logo` | ebenso | `["de","en"]` | Logos ohne Sprache sind selten |
| `metadata.image_kinds` | JSON-Liste aus `logo`, `season_poster`, `episode_still` | alle drei | Schaltbare Bildarten; `poster` und `backdrop` stehen nie drin (E-1) |
| `metadata.image_min_width.poster` | Zahl, 0 = egal | `500` | Kandidaten darunter nur, wenn keiner darüber (6.2) |
| `metadata.image_min_width.backdrop` | Zahl, 0 = egal | `1280` | ebenso |
| `metadata.image_by_vote` | `"1"`/`"0"` | `"1"` | Innerhalb der Sprache das bestbewertete Bild statt des ersten |
| `metadata.local_images` | `"1"`/`"0"` | `"1"` | Lokale Bilddateien als Quelle (6.2); aus = nur Anbieter |
| `metadata.rating_country` | Ländercode aus `fskTables` | `"DE"` | Das Land, das zuerst gelesen wird; Vorgabe ist das heutige Verhalten |
| `metadata.tmdb_include_adult` | `"1"`/`"0"` | `"0"` | Suche in `searchMetadata` und `matchMovie` mit `include_adult` |
| `metadata.fanart_enabled` | `"1"`/`"0"` | `"0"` | Fanart.tv als Bildquelle nach der Ordnerliste (nur mit F-4) |

Keine neue Tabelle für den Server-Standard: zwölf Schlüssel, alle über einen Typ
`metadata.Settings` gelesen und mit Vorgaben aufgefüllt, damit ein fehlender Schlüssel nie ein
Fehler ist. **Sprachcodes** prüft der Server gegen eine **statische** Liste im Binary (die
TMDB-Sprachliste `/configuration/languages`, einmal eingebettet; Idee 01 §6 wollte sie live
holen, das macht das Speichern vom TMDB-Schlüssel abhängig). Je Quelle gilt eine Abbildung:
TMDB `de` → `de-DE`, `pt-BR` → `pt-BR`, sonst Durchreichen (`tmdbLang`; **zu prüfen in
Schritt 1:** was TMDB bei `language=es` ohne Region liefert); TVDB ISO 639-2 aus `/languages`
(einmal geholt, im Prozess gecacht, Rückfall auf die heutige Tabelle); AniList bleibt bei
`en` und Original.

### 4.2 `item_texts` ohne Sprachschranke, mit Rang; Genres je Sprache

```
item_texts:  CHECK (lang IN ('de','en')) fällt; neue Spalte rank INTEGER NOT NULL DEFAULT 0;
             sort_key und Index item_texts_sort_key (00026) bleiben; Trigger search_texts_* neu angelegt
item_genres: CHECK (lang IN ('de','en')) fällt; Trigger über json_each(json_keys(items.genres)) statt 'de'/'en'
```

SQLite kennt kein `ALTER COLUMN`, beide Tabellen werden nach Vorlage neu gebaut (Regel 5). Ein
`DROP TABLE item_texts` löscht die Trigger `search_texts_insert/update/delete`
(`00015_discover.sql:235` bis `:254`); die Migration legt sie neu an, und der Migrationstest
prüft `sqlite_master` auf Trigger und Index und führt danach eine Suche aus. `rank` ist die
Stelle der Sprache in der **wirksamen** Liste des Titels zum Zeitpunkt des Füllens (0 = erste).
`SortKey` streicht Artikel nur für de/en (`sortkey.go:14`); für andere Sprachen bleibt der
Titel unverändert, ein späterer Nachtrag kann Tabellen ergänzen.

**Die Wahl zur Laufzeit** bleibt im Go-Code (`visibility/mapping.go`), nicht im SQL, und bleibt
**feldweise** (Befund 3): für jedes Feld (Titel, Beschreibung, Tagline, Genres) die erste
nicht-leere Zeile in der Kette **Metadatensprache des Profils → Oberflächensprache des Profils
(`de`/`en`) → Liste nach `rank`** → `items.title` (Dateiname). Der Scope (`visibility.Scope`)
bekommt die Metadatensprache; die Sortierung (`library.go`) joint die Zeile mit der
Profilsprache, sonst `rank = 0` statt `'en'`; der Genrefilter und die Suche ebenso. So lesen
die Scoped-Abfragen **keine Einstellungen** (Regel 2 bleibt eine Abfrage mit `visible_items`),
und der Entscheid, welche Sprachen es gibt, liegt im Job, der den Rang schreibt (Regel 12).
`genres{DE, EN}` wird `map[string][]string`. Die System-Abfragen (`queries/system/admin.sql`,
`admin_stats.sql`, `spotlight.sql`, `collections.sql`) und `admin/admin.go` nehmen dieselbe
Kette, mit `de` als Profilsprache des Administrators. Schritt 1 zählt jede Stelle aus 1.1 auf.

### 4.3 Profil: Metadatensprache getrennt von der Oberflächensprache

```
profiles: ADD COLUMN metadata_language TEXT   -- NULL = wie language
```

`language` bleibt `de`/`en` und steuert die Oberfläche (`web/src/locales`) **und** die
Metadaten, solange `metadata_language` leer ist (so bleibt jedes heutige Profil, wie es ist).
Nimmt der Administrator eine dritte Sprache in eine Liste, kann das Profil sie als
`metadata_language` wählen (7.4). Wählbar ist die **Vereinigung aller wirksamen Listen**
(Server-Standard und jede Bibliothek); `updateProfile` prüft dagegen (`auth/profiles.go:269`
prüft heute fest de/en), und `getSession` liefert sie als `availableMetadataLanguages`, damit
auch ein Gastkonto (die Profilseite ist `account`) die Auswahl kennt, ohne `admin`-Operationen.
Eine Sprache, die später aus allen Listen fällt, bleibt am Profil (das Feld bleibt sichtbar,
solange ein Wert gesetzt ist, 7.4) und fällt in der Kette weiter. Regel 9: Sprachnamen über
`Intl.DisplayNames`. Offline (M5) ändert sich nichts: der Client bekommt die Texte wie heute
fertig gewählt.

### 4.4 Bilder: Quelle und Sprache je Art, Logo, Wahl je Titel

```
images:       CHECK (kind IN ('poster','backdrop','logo'))              -- Tabellenneubau nach Vorlage
              CHECK (format IN ('jpeg','webp','png'))                   -- png nur für logo (6.3)
              lang gefüllt ('' = ohne Text, NULL = unbekannt/Altbestand)
              provider TEXT NOT NULL DEFAULT 'tmdb'                     -- tmdb, tvdb, anilist, local, fanart, cover, manual
item_images (                                                            -- die entschiedene Quelle je Art
  item_id   TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  kind      TEXT NOT NULL CHECK (kind IN ('poster','backdrop','logo')),
  provider  TEXT NOT NULL,
  lang      TEXT NOT NULL DEFAULT '',                                    -- '' = ohne Text oder unbekannt
  source    TEXT NOT NULL,                                               -- Adresse beim Anbieter oder file:<rel>?<size>-<mtime> (6.2)
  chosen    INTEGER NOT NULL DEFAULT 0 CHECK (chosen IN (0,1)),          -- 1 = vom Administrator gewählt (und in locked_fields)
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (item_id, kind)
) STRICT, WITHOUT ROWID;
item_local_images (                                                      -- vom Scanner gefundene Dateien (6.2)
  item_id   TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  kind      TEXT NOT NULL,
  rel_path  TEXT NOT NULL,                                               -- relativ zur Wurzel, nur über paths lesbar
  width     INTEGER NOT NULL, height INTEGER NOT NULL,
  size      INTEGER NOT NULL, mtime INTEGER NOT NULL,
  PRIMARY KEY (item_id, kind, rel_path)
) STRICT, WITHOUT ROWID;
```

**4.4.1 Eine Tabelle statt zwei Spalten.** `items.tmdb_poster_path` und `tmdb_backdrop_path`
können Quelle und Sprache nicht tragen. `item_images` ersetzt sie als **einzige** Quelle für
`image.fetch`: Migration 00029 füllt die Tabelle aus den zwei Spalten (Quelle nach Host:
`image.tmdb.org` → `tmdb`, `artworks.thetvdb.com` → `tvdb`, AniList-Host → `anilist`, Präfix
`file:` → `cover`; `lang = ''`), `fill` schreibt ab Schritt 2 nur noch die Tabelle, die zwei
Spalten bleiben **ungenutzt** stehen (ein Neubau von `items` mit seinen Fremdschlüsseln lohnt
dafür nicht; der Abbau ist ein Nachtrag). `item_field_sources` behält für `poster`, `backdrop`,
`logo` den Schlüssel `lang = ''` (Befund 9: der Primärschlüssel ist `(item_id, field, lang)`,
und `''` heisst dort «Feld ohne Sprache»); die Bildsprache steht in `item_images.lang` und wird
auf jede `images`-Zeile kopiert, damit die Titelpflege sie ohne zweite Abfrage zeigt.

**4.4.2 Bildwahl je Titel.** Ein Klick des Administrators auf einen Kandidaten schreibt die
Zeile in `item_images` mit `chosen = 1` und trägt `kind` in `items.locked_fields` ein (B-6, wie
heute `poster`/`backdrop`). `fill` lässt gesperrte Felder aus (`fill.go:22`); `image.fetch`
unterscheidet **Sperre mit Wahl** (`chosen = 1`: genau diese Quelle holen, dann nie mehr) von
**Sperre ohne Wahl** (Altbestand, hochgeladenes Bild: nie holen, nur nachskalieren, `scaleLocked`
wie heute). «Sperre aufheben» setzt `chosen = 0` und löscht den Sperreintrag; der nächste `fill`
wählt nach Regel. **Keine Kandidatentabelle**: Kandidaten der Anbieter werden beim Öffnen der
Bildwahl live geholt (6.4); lokale Kandidaten stehen in `item_local_images`. 60'000 Titel mal
dreissig Anbieter-Kandidaten wären 1,8 Millionen Zeilen ohne Nutzen.

**4.4.3 Folgen-Standbild.** Es bleibt `kind = 'backdrop'` an der Folge (`images.go:87`); der
Schalter `episode_still` entscheidet nur, ob es geholt wird. Keine Umbenennung zu «thumb»
(Idee 01 §6): nichts in der Oberfläche unterscheidet die beiden, und `ImageKind` bliebe sonst
nicht additiv. Staffelposter aus: die Oberfläche fällt auf das Serienposter zurück, wie heute
bei einer Staffel ohne eigenes Bild (`fill` übernimmt dann kein Staffelposter, die API liefert
das Serienposter in `Season.poster`).

### 4.5 Abweichung je Bibliothek

```
library_metadata (
  library_id      TEXT PRIMARY KEY REFERENCES libraries(id) ON DELETE CASCADE,
  text_langs      TEXT CHECK (text_langs IS NULL OR json_valid(text_langs)),
  images          TEXT CHECK (images IS NULL OR json_valid(images)),   -- {"langs":{"poster":[…],…},"kinds":[…]}
  updated_at      INTEGER NOT NULL
) STRICT, WITHOUT ROWID;
```

Zwei Spalten, zwei Schalter in der Oberfläche (7.1): «Sprachen abweichend» und «Bilder
abweichend». `NULL` heisst «wie Server». Keine Zeile heisst «alles wie Server». Mindestbreite,
Bewertung, lokale Bilder, Freigabeland, Erwachseneninhalte und Fanart.tv sind **nur** global
(1.2; Marco kann mit F-6 mehr verlangen). `metadata.Effective(libraryID)` löst die Abweichung
auf und wird von **`match`** (Texte beim Zuordnen), `fill`, `image.fetch`, `trailer.find` und
von der Prüfung in `updateProfile` (4.3) gelesen, nie von einer Scoped-Abfrage.

### 4.6 Was `item_local_images` füllt

Der **Scanner** (`library`, er geht jeden Ordner ohnehin ab und liest NFO) erkennt Bilddateien
und schreibt sie je Titel, mit Breite und Höhe aus dem Header (`image.DecodeConfig`, nur Header)
und Grösse/Änderungszeit für die Erkennung von Ersetzungen (wie `coverSource`). **Zuordnung:**

| Titel | Ordner | Dateien (Reihenfolge = Vorrang) |
|---|---|---|
| Film, **einziger** Film im Ordner | Ordner der Datei | `poster.jpg` > `folder.jpg`; `fanart.jpg` > `backdrop.jpg`; `logo.png` > `clearlogo.png` |
| Film, mehrere Filme im Ordner | Ordner der Datei | nur `<stem>-poster.jpg`, `<stem>-fanart.jpg`, `<stem>-logo.png` |
| Serie | Ordner der `tvshow.nfo`, sonst der gemeinsame Elternordner der Folgen | `poster.jpg` > `folder.jpg`; `fanart.jpg` > `backdrop.jpg`; `logo.png` > `clearlogo.png` |
| Staffel | Serienordner oder Staffelordner | `seasonNN-poster.jpg` im Serienordner > `poster.jpg`/`folder.jpg` im Staffelordner |
| Folge | Ordner der Datei | `<stem>-thumb.jpg` |

Dateiendungen `jpg`, `jpeg`, `png`, `webp` (`imageExt`, `images.go:67`). `landscape.jpg`
(Embys Thumb) wird **nicht** gelesen (kein Thumb, 1.3). Der Scanner liest nur über `paths`
(Regel 4) und nur Header; `fill` liest dann die Tabelle, **kein** Verzeichniszugriff über NFS je
Titel. Für Hörbücher und Podcasts bleibt `coverSource`, wie es ist.

### 4.7 Migrationsplan: drei Migrationen, eine je Schritt

`TEMPLATE.sql.txt` sagt «Never edit a migration that has been committed» (Regel 5). Darum eine
Migration je Schritt, jede nach Vorlage (`NO TRANSACTION`, `foreign_keys=OFF`,
`foreign_key_check`, `foreign_keys=ON`), nur vorwärts, mit eingefrorenem Stand in
`testdata/db/` danach:

**00028 (Schritt 1, Sprachen):** `item_texts` neu bauen ohne Sprach-CHECK, mit `rank` (`de` 0,
`en` 1), mit `sort_key` und Index aus 00026, Trigger `search_texts_*` neu anlegen; `item_genres`
neu bauen ohne CHECK, Trigger über die Objektschlüssel von `items.genres`; `profiles ADD COLUMN
metadata_language TEXT`.

**00029 (Schritt 2, Bilder):** `images` neu bauen mit `kind` um `logo`, `format` um `png`,
`provider` (Rückfüllung nach Host, 4.4.1); `item_images` anlegen und aus `tmdb_poster_path`/
`tmdb_backdrop_path` füllen; `item_local_images` anlegen (leer, der nächste Scan füllt sie).

**00030 (Schritt 3, Einstellungen):** `library_metadata` anlegen. `settings`-Schlüssel brauchen
keine DDL.

Keine Migration füllt Bilder oder Texte nach: bestehende Titel bleiben, bis `metadata.apply`
läuft (8.1). Laufzeit: Tabellenneubau von `item_texts` (120'000 Zeilen beim M3-Lasttest) und
`images` (rund 400'000 Zeilen) in Sekunden; Messung je Schritt. Die Sicherung vor jeder Migration
macht der Server wie immer mit `VACUUM INTO`.

---

## 5. Auswirkung auf den Vertrag (1.6.0, nicht brechend)

Die Fassung steigt **in Schritt 1** auf 1.6.0 (erste additive Änderung, `Profile.metadataLanguage`
und `Session.availableMetadataLanguages`); die weiteren Schritte ergänzen 1.6.0, kein
Zwischenstand mit eigener Nummer.

### 5.1 Operationen

| Operation | Pfad | x-visibility | Zweck |
|---|---|---|---|
| `getMetadataSettings` | `GET /admin/settings/metadata` | admin | Die zwölf Schlüssel mit Vorgaben aufgelöst; die Sprachliste und je Quelle die bekannten Sprachen kommen aus `listMetadataProviders` (`MetadataProviderInfo.languages`, vorhanden) |
| `updateMetadataSettings` | `PUT /admin/settings/metadata` | admin | Alles auf einmal; 422 mit Feldfehlern (`langs_empty`, `lang_unknown`, `ui_lang_required`, `chain_duplicate`, `country_unknown`); Antwort nennt, wie viele Titel die Änderung betrifft |
| `getLibraryMetadata` | `GET /admin/libraries/{id}/metadata` | admin | Die Abweichung der Bibliothek (jede Spalte `null` = wie Server) **und** die wirksamen Werte |
| `updateLibraryMetadata` | `PUT /admin/libraries/{id}/metadata` | admin | ebenso |
| `applyMetadata` | `POST /admin/metadata/apply` | admin | Körper `{libraryIds?: [], what: ["texts","images"]}`; reiht `metadata.apply` ein; Antwort 202 `ApplyMetadataResponse {jobId, titles}` (ein `JobRef`-Schema gibt es nicht; `refreshItem` antwortet mit `AdminItem`) |
| `listItemImageCandidates` | `GET /admin/items/{id}/image-candidates?kind=` | admin | Live beim Anbieter plus `item_local_images` (6.4) |
| `getItemImageCandidatePreview` | `GET /admin/items/{id}/image-candidates/preview?source=` | admin | Nur für lokale Kandidaten (Datei über `paths`), als `http.Handler` (Regel 6) |
| `setItemImage` | `PUT /admin/items/{id}/images/{kind}` | admin | `{provider, source}` wählt und sperrt; reiht `image.fetch` ein |
| `clearItemImage` | `DELETE /admin/items/{id}/images/{kind}` | admin | Sperre und Wahl aufheben; reiht `fill` und `image.fetch` ein |

Neun Operationen, jede mit `x-visibility` und Eintrag in `TestContractCoversEveryOperation`
(`contract_access_test.go`, Regel 7). `getAdminSettings` und «Integrationen» bleiben, wie sie
sind; mit F-4 kommen `fanart` und `fanartClient` in `AdminSettingsKeys` (zwei Felder, additiv).

### 5.2 Schemata

- `ImageKind`: `enum: [poster, backdrop, logo]` (additiv). `getAdminItemImage` hat ein eigenes
  Enum (`openapi.yaml:3294`) und nur jpeg/webp: beide um `logo` und `png` erweitern.
  `getItemImage` fällt heute auf JPEG zurück, wenn WebP fehlt; für `logo` ist der Rückfall PNG.
- `ItemDetail`: neues optionales Feld `logo: ImageRef` neben `poster` und `backdrop`.
  **Nicht** an jedem `ItemSummary` (das vergrössert jede Liste und `ListItemImages`), sondern nur
  an den Einträgen der Spotlight-Reihe (`HomePage.heroRow`, `ItemSummary` dort mit optionalem
  `logo`).
- `Profile` und `ProfileUpdate`: `metadataLanguage?: string` (BCP-47); `language` bleibt
  `enum: [de, en]`. `Session`: `availableMetadataLanguages: string[]` (4.3).
- `AdminItem`: `images` je Art mit `provider`, `lang`, `locked`, `chosen` (für die Titelpflege).
- `Rating`-Anzeige: `ratingSource` nennt schon heute Anbieter und Land (`tvdb:DE`); die
  Oberfläche zeigt neu «12 (CH)» daraus, kein neues Feld.
- Neue Schemata `MetadataSettings`, `LibraryMetadata`, `ImageCandidate` (`provider`, `lang`,
  `width`, `height`, `score`, `source`, `previewUrl`), `ApplyMetadataRequest`,
  `ApplyMetadataResponse`.

### 5.3 Ist das eine brechende Anhebung?

Nein: nur neue Operationen, neue optionale Felder, erweiterte Enums. Vertrag 1.6.0, Tag
`v1.6.0` nach der Regel «Vertrag = Tag». Eintrag für die Release-Notes: «Metadaten-Einstellungen:
Sprachen als Liste, Bildsprache und Logo, lokale Bilddateien, Freigabeland, Bildwahl je Titel;
Vertrag 1.6.0 additiv.»

---

## 6. Server

### 6.1 Sprachen durch die Kette

`textLangs` (`provider.go:30`) wird zu `metadata.Effective(libraryID).TextLangs`; **`match`**
(alle sechs Stellen aus 1.1) und `fill` fragen jede Quelle je Sprache wie heute, nur mit der
Liste statt der Konstante. TVDB: `tvdbLangs` wird zur Tabelle aus `/languages` (4.1); eine
Sprache, die eine Quelle nicht kennt, wird übersprungen, und `getMetadataSettings` zeigt das
über `MetadataProviderInfo.languages` («TVDB: kein Rätoromanisch»). AniList bleibt bei `en` und
Originaltitel. `match` und `fill` schreiben `rank` je Zeile; `fill` **löscht** Zeilen von
Sprachen, die nicht mehr in der wirksamen Liste stehen, ausser das Feld ist gesperrt oder die
Quelle `manual`. Trailer: `trailer.find` nimmt die primären Untersprachen der Liste statt `de,
en` (E-6); mehr Sprachen heisst mehr YouTube-Suchen am Tag, was die Antwort von
`updateMetadataSettings` nennt («3 Sprachen: bis zu 33 Titel am Tag statt 50»).

### 6.2 Bildwahl im `fill`: Kandidaten, Reihenfolge, Kette, Breite, Bewertung

`fill` entscheidet die Felder `poster`, `backdrop`, `logo` (`fill.go:539` ff.) neu. Für
**Bildfelder** übergibt es `decideFields` die Reihenfolge `["local", …Ordnerliste, "fanart"]`
(E-4; heute bekäme eine Quelle ausserhalb der Ordnerliste den letzten Rang, `fill.go:163`, und
eine lokale Wahl verlöre bei jedem Lauf gegen TMDB; Test «local bleibt nach dem zweiten fill»).
Jede Quelle liefert **Kandidaten** `{kind, lang, width, height, votes, voteCount, source}`:

- **TMDB:** der Detail-Abruf der **`recordLang`-Sprache** (nur dieser eine, nicht jeder
  Sprach-Abruf, sonst käme der Bildblock zwei- bis dreimal) bekommt `images` in
  `append_to_response` mit `include_image_language=<Vereinigung aller Ketten ohne "">,null`;
  TMDB liefert `posters`, `backdrops`, `logos` mit `iso_639_1` (`null` = ohne Text → `""`),
  `width`, `height`, `vote_average`, `vote_count`, `file_path`. Dasselbe an `TVSeason` für das
  Staffelposter. Heute nimmt der Server `poster_path` aus dem englischen Datensatz; das ist
  künftig nur noch der Kandidat ohne Treffer in der Kette (Stufe 5 unten). **SVG-Logos:** TMDB
  liefert zu einem `.svg` nach meinem Wissen unter derselben Id mit Endung `.png` ein PNG
  (Jellyfin tauscht die Endung so); O-3 prüft das in Schritt 2, und dann wird das SVG **als
  PNG geholt** statt übersprungen. Bis zur Prüfung: überspringen (die Pipeline kennt JPEG, PNG
  und WebP als Eingabe, `imageExt`; ffmpeg ohne librsvg rastert nicht). **Zu prüfen:** ob
  TMDB-Bilder inzwischen `iso_3166_1` tragen (dann Region für `pt-BR` nutzbar).
- **TVDB:** `artworks` tragen `type`, `language` (ISO 639-2, Abbildung über die Tabelle aus
  4.1), `score`, `image`; das Struct bekommt `language`, `width`, `height`. `bestArtwork` wird
  zu «alle Artworks der Art als Kandidaten». Typen: Serienposter 2, Hintergrund 3, Staffelposter
  7, ClearLogo 23 (**zu prüfen** gegen die v4-Doku in Schritt 2; Filmtypen sind hier ohne
  Bedeutung, weil `tvdbProvider.Movie` `ErrNotFound` liefert, `tvdb.go:256`).
- **`local`** (nur mit `metadata.local_images`): die Zeilen aus `item_local_images` (4.6);
  `lang = ''` (unbekannt), Breite aus der Tabelle, keine Bewertung. `source` ist
  `file:<rel_path>?<size>-<mtime>` nach dem Muster von `coverSource` (`images.go:729` ff.), damit
  eine ersetzte `poster.jpg` als Änderung erkannt wird; `imageSource` lernt das Präfix für
  Videos (heute nur Cover) und löst es über `paths` auf.
- **Fanart.tv** (nur mit F-4): `GET /v3/movies/{tmdb_id}`, `GET /v3/tv/{tvdb_id}`; Arten
  `movieposter`/`tvposter`, `moviebackground`/`showbackground`, `hdmovielogo`/`hdtvlogo`,
  `seasonposter`; Felder `url`, `lang` (`00` = ohne Text → `""`), `likes` (als String, zu Zahl).
  Serien nur mit `tvdb_id`, Filme mit `tmdb_id`; fehlt die Id, wird die Quelle still
  übersprungen (wie `fill.go:34` für TVDB). **Zu prüfen in Schritt 6 (O-4):** Schlüssel als
  Header `api-key`/`client-key` oder als Query `api_key`/`client_key` (dann Redaction-Muster,
  9.3), Rate und Kontingent (unbelegt), Bedeutung des persönlichen Schlüssels. Host
  `webservice.fanart.tv` und `assets.fanart.tv` in `imageSource` zulassen.

**Die Wahl je Art, in dieser festen Reihenfolge** (Befund 11; zwei Entwickler sollen dasselbe
bauen):

1. Eine **gewählte** Zeile (`item_images.chosen = 1`) gewinnt; `fill` rührt das Feld nicht an.
2. Ein **lokaler** Kandidat gewinnt, wenn `width >= min_width` (O-1: ein zu kleiner bleibt
   Kandidat in der Bildwahl, gewinnt aber nicht von allein).
3. **Kette aussen, Quelle innen:** für jeden Eintrag der Kette (Sprache oder `""`) werden die
   Kandidaten **aller** Anbieterquellen dieser Sprache mit `width >= min_width` gesammelt; davon
   gewinnt bei `image_by_vote` der mit dem höchsten `votes` **unter denen mit mindestens drei
   Stimmen** (sonst zählt die Stimme nicht; Gleichstand nach `voteCount`, dann Quellenrang),
   ohne `image_by_vote` der erste in Quellenreihenfolge. Das heisst: ein deutsches TVDB-Poster
   schlägt ein englisches TMDB-Poster, weil die Sprache die sichtbare Güte ist und die
   Ordnerliste die Identität und die Texte regelt. Das weicht von Idee 01 §6 («Anbieter-Rang →
   Sprachkette») ab und ist der Fixture-Fall für Schritt 2.
4. Erfüllt kein Kandidat Kette **und** Breite: derselbe Durchlauf ohne Mindestbreite (die Art
   bleibt nie leer, nur weil die Vorgabe zu hoch ist).
5. Sonst der Kandidat ohne Sprache, den die Quelle als Vorgabe nennt (`poster_path`).
6. Sonst leer, Art bleibt leer.

Quelle, Sprache und Adresse landen in `item_images`; die Herkunft in `item_field_sources`
(`lang = ''`). `fill` bleibt idempotent (ADR 0015 Regel 2). Ändert sich nur die **Kette**, ist
die neue Wahl ein anderes Bild derselben Quelle; `fill` schreibt die neue Adresse, und
`image.fetch` erkennt sie an `source_url` (`images.go:40`).

### 6.3 Pipeline: Logo mit Transparenz

`image.fetch` (`images.go:102` ff.) bekommt `logo` in `itemImageKinds` für Film und Serie,
Breiten `w400`, `w800`. Die Formate werden **je Art** definiert (`map[kind][]format` statt der
globalen Liste `imageFormats`): Poster und Hintergrund wie heute JPEG und WebP; Logo **WebP und
PNG** (Transparenz; JPEG bewusst nicht). `kindComplete` prüft gegen die Formate der Art.
`transcode.ImagePNG` kommt dazu (ffmpeg `-c:v png`); `scaleLocked` behält den Medientyp des
Originals statt JPEG anzunehmen (Idee 01 §6 nennt den Fehler). Akzentfarbe weiter nur vom
Poster. Staffelposter und Folgen-Standbild hängen am Schalter `image_kinds`: aus heisst «nicht
holen, vorhandene Dateien beim nächsten `apply` löschen» (8.1), nicht «verstecken».

### 6.4 Kandidaten für die Titelpflege

`listItemImageCandidates` fragt **live**: TMDB `/{movie|tv}/{id}/images` ohne Sprachfilter (alle
Sprachen, damit der Administrator wählen kann), TVDB `artworks` aus dem Serien-Datensatz,
`item_local_images`, Fanart.tv falls an. Antwort im Prozess gecacht, 10 Minuten je Titel und
Art (eine kleine Map mit Ablauf im `metadata.Service`, wie der Trakt-Cache; es gibt keinen
allgemeinen Cache-Mechanismus). Jeder Kandidat trägt `previewUrl`: beim Anbieter die
`w300`-Adresse (TMDB `t/p/w300`, TVDB und Fanart.tv ihre Vorschau), lokal
`getItemImageCandidatePreview?source=` über `paths` (Regel 4, `http.Handler`, Regel 6).
**Bewusst entschieden:** die Vorschauen der Anbieter lädt der Browser des Administrators
direkt (wie heute `getAdminItem` den `w200`-Pfad beim Anbieter liefert, `openapi.yaml:7595`),
mit `referrerpolicy="no-referrer"` am `<img>`; ein Umweg über den Server hiesse, hundert
Vorschauen je Öffnen durch den Server zu ziehen.

### 6.5 Der Job `metadata.apply`

Eine **Ereignis**-Jobart im Register aus 09 (`jobs.Type`, `registry.go:61`): `Group =
metadata`, `Pool = general`, `Trigger = event`, mit `Handler` und Payload `{libraryIds, what}`,
eingereiht nur von `applyMetadata`. **Nicht** als Hand-Jobart: deren `Run` hat keinen Payload,
«Jetzt ausführen» auf der Jobseite liefe dann über **alles** und mit `ManualPriority = 7`
(`registry.go:58`) statt tief, genau die Nebenwirkung, die F-8 vermeidet. Der Handler zählt die
Titel (`ListFillItems` mit Filter) und reiht **je Titel** `metadata.match` ein (Neuabruf wie
`refreshItem`, ohne Probes; nur so holt ein Ordner mit einer Quelle eine neue Sprache, 1.1),
mit der Priorität des Matches; der Match reiht wie heute `fill` und `image.fetch` nach. Bei
`what = ["images"]` ohne `texts` reiht der Handler nur `fill` (Bildfelder) und `image.fetch`
ein, das reicht, weil die Bildkandidaten aus dem Detail-Abruf kommen, den `fill` ohnehin macht,
sobald zwei Quellen oder (neu) Bildfelder zu entscheiden sind. Idempotenzschlüssel je Kind,
damit ein zweites `apply` wartende Jobs nicht verdoppelt (`RaiseQueuedPriority`, Entscheid vom
26. Oktober 2026). **Fortschritt, ehrlich (`19ee05c`):** der `apply`-Lauf ist nach Sekunden
fertig und meldet «n Kinder eingereiht»; die Seite «Metadaten» zeigt darunter die Zahl der
noch wartenden `metadata.match`/`image.fetch` aus `listJobTypes` (09, vorhanden), und die
Jobseite zeigt die Kinder je Art mit eigenem Fortschritt. Pausiert der Administrator
`metadata.match` auf der Jobseite, warten die Kinder; der Elternlauf ist dann längst fertig.
**Kein Zeitplan** (E-7). `updateMetadataSettings` reiht **nicht** selbst ein (F-8); die
Antwort nennt die Zahl der betroffenen Titel, die Oberfläche bietet «Jetzt anwenden» an (7.1).

### 6.6 Freigabeland

`ratingCountries` wird zur Funktion `ratingOrder(country)`: gewähltes Land zuerst, dann die
heutige Kette ohne dieses Land. `FSKFromCertifications` nimmt die Reihenfolge als Parameter und
weiter das **erste Land mit Wert**. Wirkung, ausdrücklich: ein **neuer** Titel mit CH zuerst
bekommt die CH-Stufe (oft tiefer als DE: «12 (CH)» statt «16 (DE)»), und ein Kinderprofil mit
Grenze 12 sieht ihn; eine **gespeicherte** Stufe bleibt (K-3, `fill` hebt nur an). Wer die
strengere von zwei Ländern will, kann das nicht einstellen (eine Option «strengste aus
gewähltem Land und DE» ist in F-7 angeboten, aber nicht empfohlen). `rating_source` zeigt,
woher die Stufe kam, und die Titelpflege setzt sie wie heute von Hand (`setItemRating`). Die
Oberfläche sagt beides unter dem Feld (7.1).

---

## 7. Oberfläche

### 7.1 Verwaltungsseite «Metadaten»

Neuer Eintrag `metadata` in `adminSections` (`sections.ts`), Gruppe **Bibliothek** (neben
Bibliotheken, Podcasts, Pflege), Pfad `paths.adminMetadata = '/admin/metadata'`, Icon `image`
(**neu** in `Icon.tsx`, Teil von Schritt 4; `image` und `tag` gibt es heute nicht). Titel
«Metadaten», Karte auf der Übersicht wie jede Sektion. Aufbau wie «Integrationen» (`Panel`,
`Section`, `Field`, `Switch`, `segments`), alles aus `web/src/ui` (Regel 9):

1. **Geltung** oben: eine **Auswahlliste** «Standard | <Bibliothek> …» (Segmente wie im
   Prototyp nur bis fünf Bibliotheken, darüber ein `Select`; abweichende Bibliotheken tragen eine
   Markierung). Bei einer Bibliothek stehen zwei Schalter, «Sprachen abweichend» und «Bilder
   abweichend» (4.5); nur die eingeschalteten Abschnitte sind bearbeitbar, die anderen zeigen
   die wirksamen Werte gedimmt mit «wie Server». Die globalen Abschnitte (Güte, Quellen,
   Freigabe, Anwenden) sind bei einer Bibliothek ausgeblendet.
2. **Sprachen:** geordnete Chips (Pfeile links/rechts, Entfernen; der letzte von `de`/`en` ist
   nicht entfernbar, E-9), «+ Sprache» öffnet eine Auswahl aus der statischen Liste mit
   `Intl.DisplayNames`-Namen; unter der Liste, welche Quelle welche Sprache nicht kennt.
3. **Bilder:** Schalter Logo, Staffelposter, Folgenbild (Poster und Hintergrund als feste
   Zeilen «immer», E-1); Bildsprache-Kette je Art als geordnete Chips mit «Ohne Text».
4. **Güte** (global): Mindestbreite Poster und Hintergrund (Auswahl egal/500/1000/1500/2000 bzw.
   egal/1280/1920/3840); Schalter «Nach Bewertung wählen».
5. **Quellen** (global): TMDB «Erwachseneninhalte bei der Suche»; «Lokale Dateien» mit Schalter
   und der Zahl der Titel mit Dateien; Fanart.tv Schalter mit Link zum Schlüssel auf
   «Integrationen» (nur mit F-4). Hinweis: «Reihenfolge der Quellen je Ordner unter
   Bibliotheken» mit Link. Keine TVDB-Rolle hier (die steht am Ordner, 07).
6. **Altersfreigabe** (global): Land (DE, AT, CH, GB, US) mit dem Hinweis aus 6.6.
7. **Anwenden** (global): «Auf bestehende Titel anwenden» mit Auswahl Texte/Bilder und Bibliothek,
   Zahl der Titel, Knopf «Jetzt anwenden» (reiht `metadata.apply` ein, Link auf die Jobseite);
   darunter «noch wartend: n Zuordnungen, m Bilder» aus `listJobTypes` und der letzte Lauf aus
   dem Verlauf von 09.
8. **Speicherleiste** unten, klebend: «Ungespeicherte Änderungen» / «Gespeichert»; nach dem
   Speichern der Hinweis «n Titel betroffen, jetzt anwenden?».

Regel 8: `web/e2e/admin-metadata.spec.ts` (Pfeiltasten durch Auswahl und Chips, Tab, Enter,
Zurück, sichtbarer Fokus, keine Fokusfalle in der Sprachauswahl).

### 7.2 Bibliothek

Die Bibliothekskarte (`LibraryCard.tsx`) bekommt eine Zeile «Metadaten: wie Server» oder
«Metadaten: Sprachen abweichend» / «Bilder abweichend» mit Link auf
`/admin/metadata?library=<id>`; keine zweite Oberfläche. Der Ordner-Dialog aus 07 bleibt
unverändert.

### 7.3 Titelpflege: Bilder

`AdminItemPage` (`/admin/items/{id}`) bekommt einen Abschnitt **Bilder**: je Art das gewählte
Bild mit Quelle, Sprache, Breite und Schloss; Knopf «Andere wählen» öffnet die Kandidaten
(`listItemImageCandidates`) als Kacheln mit Sprache, Breite, Quelle, Bewertung, 40 auf einmal
und «mehr» auf Knopf; Klick wählt und sperrt (`setItemImage`), «Sperre aufheben»
(`clearItemImage`). Bestehende Sperren für Poster und Hintergrund erscheinen dort als «Sperre
ohne Wahl». Regel 8: Erweiterung von `admin-item.spec.ts` um den Kandidatenraster (Pfeiltasten
im Raster, Enter wählt, Escape schliesst, Fokus kehrt auf «Andere wählen» zurück).

### 7.4 Profil

In den Profil-Einstellungen erscheint unter «Sprache» ein zweites Feld «Sprache der
Beschreibungen», wenn `availableMetadataLanguages` mehr als `de`/`en` enthält **oder** das
Profil einen Wert gesetzt hat (sonst liesse sich ein verwaister Wert nie zurücksetzen); Werte
aus `Intl.DisplayNames`, erster Eintrag «wie Oberfläche». Der bestehende Tastaturtest der
Profilseite deckt das Feld mit einem Fall «mit dritter Sprache» ab.

### 7.5 Zuschauer: Logo

Detailseite (`ItemMeta.tsx`) und Spotlight (`HomePage.tsx`): ist `logo` gesetzt, bleibt das
`<h1>` und enthält ein `<img alt="<Titel>">` statt des Textes, Breite begrenzt auf die Breite
des Textes (max. 40 % der Bühne, 60 % auf dem Handy), `object-fit: contain`; bei `onError`
(Ladefehler, **offline**: das Logo ist nicht im Offline-Paket von M5) fällt der Knoten auf den
Text zurück. Screen-Reader lesen den Titel über `alt`. Keine neue Seite, also kein neuer
Tastaturtest; die Storybook-Baselines von 08 bekommen je einen Fall mit Logo.

### 7.6 Klickbarer Prototyp

`docs/planning/mockups/10-metadaten-prototyp.html` ist eine einzelne HTML-Datei ohne
Abhängigkeiten, dunkel, mit den Tokens aus `packages/design-tokens/tokens/` (Farben,
Abstände, Radien, Schrift) und den Mustern der heutigen Verwaltungsseiten (AdminNav, Panel,
Switch, Chips). Sie zeigt die Seite «Metadaten» nach 7.1 mit **fünf** Bibliotheken als
Segmente (die Auswahlliste ab sechs zeigt er nicht) und einem Schalter «Von Standard
abweichen» für beide Abschnitte zusammen (die gebaute Seite hat zwei, 4.5); Anime weicht ab,
Filme nicht. Der Prototyp ist **kein** Code für `web/src` und wird nicht gebaut oder getestet.
Dieselbe Fassung ist als privates Artefakt unter
`https://claude.ai/artifact/TGSPUTWogTsfeBZqDzF1oU` erreichbar; Änderungen immer in beide
Kopien.

---

## 8. Leistung

| Frage | Antwort |
|---|---|
| Mehr TMDB-Anfragen? | **Nicht je Sprache mehr als heute**: Bilder kommen über `append_to_response=images` im Detail-Abruf der `recordLang`-Sprache und an `TVSeason`. Je **zusätzlicher** Sprache eine Anfrage je Titel und je Staffel (wie heute für `en`). TMDB-Rate bleibt 20 Anfragen je Sekunde (`tmdbRate`, `tmdb.go:23`). |
| Was kostet ein `apply`? | Marcos NAS (1'791 Filme; 3'749 Folgen in «KinderSerien» allein, Serien gesamt zählt Schritt 7): Texte mit drei Sprachen rund 5'400 Film-Anfragen, von der Rate her **4,5 Minuten**; wirklich begrenzen Arbeiter, Latenz (rund 150 ms je Anfrage, ein Arbeiter: 13 min) und das NAS. Bilder: ein Download je **geändertem** Bild, meist Poster (deutsche statt englische), rund 1'800 Originale à 1 MB plus Ableitungen über das NAS-Cache-Volume; das ist der eigentliche Lauf. Mit 40'000 Folgen (M3-Lasttest) und Folgenbild an: bis zu 40'000 Standbilder; darum der Schalter `episode_still` und die Zahl vor dem Knopf. Messung in Schritt 7. |
| Cache-Grösse? | Logo: zwei Breiten mal zwei Formate, PNG-Logos 50 bis 200 KB, also rund 0,5 MB je Film/Serie; bei 2'500 Titeln 1,2 GB. Folgen-Standbilder sind heute schon da (drei Breiten mal zwei Formate, rund 300 KB je Folge: 40'000 Folgen = 12 GB) und werden mit dem Schalter erstmals abschaltbar. Messung in `docs/perf/metadaten.md` (Schritt 7): Cache vor/nach `apply` auf CT 190. |
| Scoped-Abfragen langsamer? | `item_texts` bekommt `rank` und den Index `(item_id, rank)`; die Kette aus 4.2 liest je Titel alle Zeilen (heute zwei, mit drei Sprachen drei). Latte: `listLibraryItems` p95 auf CT 190 unverändert ±10 % (Messung Schritt 1 mit `docs/perf/measure-library.sh`). |
| Scanner langsamer? | Je Titelordner ein `Readdir` (macht er schon) und je Bilddatei ein Header-Lesen (einige KB). Bei 1'789 Ordnern mit 122 mal vier Dateien vernachlässigbar; Messung des NAS-Scans «Filme» (heute 186 s) in Schritt 2. |
| Startseite? | Ein Feld `logo` mehr je Spotlight-Titel; die Hülle `GET /` aus 08 (Vormalen, P-10) bekommt das Logo als weiteres `<link rel=preload>`, oder nicht, wenn die Messung aus 08 (2,33 s) kippt (O-2). |
| Kandidaten-Dialog? | Eine Anbieteranfrage je Öffnen und Art, gecacht 10 min; TMDB liefert für populäre Filme über 100 Poster, die Oberfläche zeigt 40 und lädt mehr auf Knopf. |

---

## 9. Sicherheit

### 9.1 Nur Administratoren, nichts Scoped

Alle neuen Operationen sind `admin` (5.1) ausser dem Feld `availableMetadataLanguages` an
`getSession` (`account`, nur Sprachcodes, kein Geheimnis). Die Bildauslieferung bleibt `scoped`
über `visible_items`, `logo` läuft über denselben Handler wie `poster` (`getItemImage`). Keine
Scoped-Abfrage liest `settings` oder `library_metadata` (4.2, Regel 2 und 12). Der
Kinderprofil-Vertragstest bekommt zwei Fälle: «Freigabeland senkt keine gespeicherte Stufe»
und «Neuzugang mit CH zuerst bekommt die CH-Stufe» (6.6, dokumentiertes Verhalten).

### 9.2 Dateien nur über `paths`, nur vom Scanner und der Vorschau

Lokale Bilddateien liest der **Scanner** (4.6) über `paths.Open(root)` relativ zur
registrierten Wurzel (Regel 4): kein `..`, keine Symlinks nach aussen, nur die Namen aus 4.6,
nur Header. Die Vorschau (`getItemImageCandidatePreview`) liefert nur Pfade, die in
`item_local_images` des Titels stehen (`source` wird dagegen geprüft, sonst 404); `image.fetch`
löst `file:`-Quellen ebenso nur über `paths` auf. Die vorhandene Grössenschranke der Pipeline
(Original höchstens 20 MB) gilt auch für lokale Dateien.

### 9.3 Ein neuer Aussenhost, ein neues Geheimnis (nur mit F-4)

Fanart.tv: `webservice.fanart.tv` und `assets.fanart.tv` in der Hostliste von `imageSource`
(`netguard` ist der SSRF-Schutz der Feeds, kein Host-Register; Befund 13); `fanart_api_key`,
`fanart_client_key` verschlüsselt in `settings` wie die anderen Schlüssel, in der
Redaction-Liste und im Test (Regel 10), dazu das Query-Muster `client_key` in `logging.go`,
falls Fanart die Schlüssel als Query erwartet (O-4). Ohne Schlüssel: Quelle «nicht
konfiguriert», wird nie gefragt (wie TVDB, ADR 0006).

### 9.4 Keine Shell, kein SVG-Rastern

PNG-Ableitung über `transcode` mit `exec.CommandContext` und Argumentliste (Regel 3). SVG wird
nie gerastert; ein SVG-Logo wird entweder als PNG beim Anbieter geholt (O-3) oder übersprungen.

### 9.5 Eingaben prüft der Server

Sprachcodes gegen die statische Liste, `de` oder `en` Pflicht (E-9), Kette ohne Duplikate,
Breiten aus der Auswahl, Land aus `fskTables`, Bibliotheks-Id vorhanden; alles 422 mit
Feldfehlern (`model.ValidationError`). `setItemImage` nimmt `source` nur, wenn es in den
Kandidaten des Titels vorkommt; ist der 10-Minuten-Cache abgelaufen, holt der Server die
Kandidaten **neu** statt abzulehnen, und erst ein wirklich unbekanntes `source` ist 422
`source_unknown`: kein Administrator lädt eine fremde Adresse in den Cache.

### 9.6 Vorschauen beim Anbieter

Der Browser des Administrators lädt die Vorschaubilder der Kandidaten direkt bei TMDB, TVDB
und Fanart.tv (6.4): IP und Zeitpunkt gehen an den Anbieter, kein Referrer
(`referrerpolicy="no-referrer"`). Das ist dieselbe Lage wie heute bei der TMDB-Suche in der
Titelpflege (`w200`-Pfad beim Anbieter) und bei den Trailern (`youtube-nocookie.com`).

---

## 10. Schnitt in Schritte

Jeder Schritt ein Commit auf einem Zweig ab `main`, `task ci` grün, DoD-Belege in
`docs/dev/CURRENT_STATUS.md`. Reihenfolge und Abhängigkeiten:

```
0 Papiere → 1 Sprachen (00028, match/fill, Laufzeitkette, Profil, Vertrag 1.6.0)
  → 2 Bilder (00029, Scanner, Kandidaten, Wahl, Logo-Pipeline)
  → 3 Einstellungen (00030, Operationen, apply, Freigabeland)
  → 4 Seite «Metadaten», Bibliothek, Profil, Logo beim Zuschauer
  → 5 Bildwahl je Titel → [6 Fanart.tv, nur mit F-4] → 7 Abnahme, Messung, Doku
```

### Schritt 0 — Entscheide und Papiere

Frage 16 (F-1 bis F-10, O-1 bis O-4) ins Entscheidungsprotokoll mit dem Vermerk zu Entscheid 6
(und seiner Wirkung auf neue Titel); ADR 0018 «Sprachenliste, Bildwahl mit Sprachkette und
lokalen Dateien, Freigabeland» als Teilnachfolger von ADR 0015 (Abschnitt Bilder) und
Ergänzung von ADR 0006 (Trailer-Sprachen); Hinweise in 0006 und 0015; Idee 01 auf «Übernommen
(Teil B)» mit der Restliste aus 1.3, Issue #8 kommentieren, `docs/ideas/README.md` Index;
dieses Papier auf den dann aktuellen `main` verankern. DoD: Doku-Commit, `task ci` grün.

### Schritt 1 — Sprachen

Migration 00028 (4.7) mit Migrationstest (Trigger, Index, Suche danach); `metadata.Settings`,
`Effective` (nur Server-Standard, Bibliothek kommt in 3); `textLangs` ersetzt in `match` (sechs
Stellen) und `fill`; `rank` und Löschregel (6.1); TVDB-Sprachtabelle; die Laufzeitkette aus
4.2 an **jeder** Stelle aus 1.1 (`visibility.Profile`, `pickTexts`, `pickGenres`,
`library.go`, `search.go`, System-Abfragen, `admin.go`, `genres`-Struct); `SortKey` für
unbekannte Sprachen; `profiles.metadata_language` im Modell, `updateProfile` prüft gegen die
Vereinigung, `getSession.availableMetadataLanguages`; Vertrag **1.6.0** mit
`Profile.metadataLanguage`. Tests: Migrationstest gegen den eingefrorenen Stand, `match`-Test
mit drei Sprachen über einen TMDB-Ordner (**der Blocker-Fall**: nach dem Neuabruf sind die Texte
der dritten Sprache da), `fill`-Tests, Rückfallkette feldweise (de-Profil mit leerer
de-Beschreibung bekommt en), Genrefilter in der dritten Sprache, Vertragstest. Messung
`listLibraryItems` (8). **Aufwand 5 Tage** (Fassung 1 sagte 3; Befund 2). DoD: Tests und
Messung grün.

### Schritt 2 — Bilder im Scanner, im `fill` und in der Pipeline

Migration 00029 (4.7); Scanner füllt `item_local_images` (4.6); Kandidaten je Quelle (TMDB
`append_to_response=images` an `recordLang` und `TVSeason`, TVDB Artworks mit Sprache und
Typen, `local`); Wahl nach 6.2 mit der Reihenfolge `["local", …, "fanart"]`; `item_images`
statt der zwei Spalten; `logo` in `itemImageKinds`, Formate je Art, `transcode.ImagePNG`,
`scaleLocked` mit Medientyp; Schalter `image_kinds` (noch mit Vorgaben, die Seite kommt in 4);
`imageSource` mit `file:` für Videos. Tests mit Fixture-Antworten (ein Film mit deutschem und
englischem Poster, textlosem Hintergrund, SVG- und PNG-Logo; ein Ordner mit `poster.jpg`; der
Fall «TVDB-de schlägt TMDB-en»; «local bleibt nach dem zweiten fill»; «zu kleine lokale Datei
verliert»); Negativtest «Datei ausserhalb des Ordners wird nie gelesen»; O-3 prüfen. Messung des
NAS-Scans. DoD: Tests grün, ein Titel mit Logo im Cache (Beleg: `ls`).

### Schritt 3 — Einstellungen, Vertrag, `metadata.apply`, Freigabeland

Migration 00030 (`library_metadata`); OpenAPI (5.1, 5.2 ohne die Bildwahl-Operationen) mit
`x-visibility`, `task gen`; Handler für Einstellungen und Bibliothek; Validierung (9.5);
`Effective` mit Bibliothek; Jobart `metadata.apply` im Register (6.5); `ratingOrder` (6.6);
`tmdb_include_adult`. Tests: Vertragstest, Schematest, Register-Test, die zwei Freigabe-Fälle
(9.1), `apply` reiht ohne Doppel ein und meldet die Kinderzahl. DoD: `task ci` grün.

### Schritt 4 — Oberfläche: Seite «Metadaten», Bibliothek, Profil, Logo beim Zuschauer

7.1, 7.2, 7.4, 7.5; Icon `image`; Locales de/en; Storybook-Baselines; `admin-metadata.spec.ts`;
Profiltest-Fall; Prototyp gegen die gebaute Seite abgleichen. DoD: Tastaturtest grün in
Chromium und Firefox, Regel 9 (Lint) grün.

### Schritt 5 — Bildwahl je Titel

`listItemImageCandidates` mit Cache und Neuholen, Vorschau-Handler, `setItemImage`,
`clearItemImage`, Abschnitt «Bilder» in der Titelpflege (7.3), Tastaturtest-Erweiterung. DoD:
ein Titel mit gewähltem und gesperrtem Poster überlebt einen `apply` (Beleg).

### Schritt 6 — Fanart.tv (nur mit F-4)

O-4 gegen die Doku prüfen (Schlüsselübergabe, Rate, Kontingent); Schlüssel auf
«Integrationen», Hostliste in `imageSource`, Redaction (auch `client_key`),
`ImageProvider`-Implementierung, Kandidaten in `fill` und in der Bildwahl. DoD: ein Titel mit
Fanart-Logo im Cache; ohne Schlüssel keine Anfrage (Test).

### Schritt 7 — Abnahme, Messung, Doku

`apply` auf CT 190 für «Filme» und eine Serienbibliothek, **nach** Schritt 5, damit jeder
ungewollte Bildwechsel per Bildwahl zurückgeht; Cache vorher/nachher; `listLibraryItems` p95;
`docs/perf/metadaten.md`; Abnahme am Handy (Logo in Spotlight und Detail, deutsche Poster,
Kinderbibliothek textlos); Release-Notes; `CURRENT_STATUS.md`; Memory-Notiz. Marco prüft an
seinen 122 Emby-Ordnern, ob die lokalen Bilder gewonnen haben.

**Aufwand (Arbeitstage, Schätzung):** 0: 0,5 · 1: 5 · 2: 5 · 3: 2,5 · 4: 3 · 5: 2,5 · 6: 2 ·
7: 1,5 · **Summe 20 bis 22** (ohne 6: 18 bis 20). Idee 01 schätzte für Teil B rund 20 bis 28;
die Differenz sind OMDb, Banner/Disc/Clearart, Thumb, Rohzertifikate und die Kandidatentabelle,
die dieses Papier weglässt.

---

## 11. Fragen an Marco

### 11.1 Entscheide vor Schritt 0

| Nr. | Frage | Vorschlag |
|---|---|---|
| F-1 | **Sprachen:** Dritte und weitere Sprachen nur für Metadaten (Titel, Beschreibungen, Poster), Oberfläche bleibt Deutsch/Englisch? Oder soll eine neue Sprache auch ein Sprachpaket der Oberfläche bekommen (je Sprache rund 2'300 Texte in `de.json`, Übersetzung und Pflege bei jedem Release)? | **Nur Metadaten.** Ein Profil hat dann zwei Felder: Oberfläche (de/en) und Beschreibungen (Liste). Oberflächensprachen sind ein eigenes Vorhaben |
| F-2 | **Logo** als neue Bildart, gezeigt auf Detailseite und Spotlight statt des Text-Titels (am 29. Oktober so angenommen, hier zur Gegenzeichnung)? | **Ja.** Grösster sichtbarer Gewinn; TMDB liefert Logos ohne zweite Anfrage, lokale `logo.png` sind schon da |
| F-3 | **Banner, Disc, Clearart und Thumb streichen?** Im Interview vom 20. Oktober hiess es «Logo und Thumb mit Platz im Web-Design; Banner, Disc, Clearart nur holen». Thumb (Querformat für «Weiterschauen» und Folgenlisten) gäbe es für Filme und Serien nur von Fanart.tv; die Folgen haben ihr Standbild schon | **Streichen, alle vier** (E-2, Regel 11). «Weiterschauen» nutzt weiter den beschnittenen Hintergrund. Sie bleiben als Rest bei Idee 01 |
| F-4 | **Fanart.tv** als zusätzliche Bildquelle (Logos und Poster in hoher Güte, braucht einen eigenen Schlüssel, ein weiterer Aussendienst, Schritt 6)? Jetzt, später, nie? | **Jetzt, aber optional** (Schalter aus, ohne Schlüssel nie gefragt): TMDB-Logos sind bei älteren und kleineren Titeln lückenhaft, Fanart.tv ist dafür die Standardquelle in Emby und Jellyfin. Wenn du keinen Schlüssel willst: später, Schritt 6 entfällt, nichts anderes ändert sich |
| F-5 | **Lokale Bilddateien** (`poster.jpg`, `fanart.jpg`, `logo.png` neben dem Film, wie Emby sie schreibt) als Quelle **vor** allen Anbietern? Wichtig: **Hast du diese Bilder in Emby selbst gewählt, oder hat Emby sie automatisch abgelegt?** Emby schreibt auch die automatische Wahl in den Medienordner. Lokal gewinnt immer (ausser zu klein); bei automatischen Emby-Bildern wären das oft englische Poster | **Ja, wenn du sie gewählt hast.** Sonst: Schalter «Lokale Dateien» (4.1) auf aus lassen, bis du die Ordner bereinigt hast; die Dateien bleiben als Kandidaten in der Bildwahl sichtbar |
| F-6 | **Abweichung je Bibliothek** nur für Sprachen und Bilder (Kette, Bildarten), alles andere global (am 29. Oktober so angenommen)? | **Ja.** Mindestbreite, Bewertung, Land und Quellen-Schalter sind Servergeschmack; mehr Felder je Bibliothek machen die Seite zu dem, was du nicht willst |
| F-7 | **Freigabeland:** Vorgabe bleibt DE (heutiges Verhalten) oder wird CH, weil der Haushalt in der Schweiz steht? Wirkung (6.6): ein **neuer** Titel bekommt die Stufe des zuerst gelesenen Landes, mit CH also «12 (CH)», wo DE 16 sagt, und ein Kinderprofil mit Grenze 12 sieht ihn; **gespeicherte** Stufen werden nie gesenkt. Alternativ eine Option «strengste Stufe aus gewähltem Land und DE» | **DE bleibt Vorgabe**, du stellst auf CH um, wenn du die CH-Stufen willst; die Option «strengste aus beiden» **nicht** bauen (eine Einstellung mehr, deren Wirkung niemand ohne Tabelle versteht) |
| F-8 | **Anwenden:** Nach dem Speichern selbst «Jetzt anwenden» drücken (mit Zahl der Titel) oder sofort automatisch (Interview vom 20. Oktober)? | **Selbst drücken.** Nicht wegen der Dauer (Texte: Minuten), sondern weil der Lauf Bilder über das NAS nachlädt und sichtbar Poster wechselt; das soll keine Nebenwirkung eines Speicherns sein |
| F-9 | **Sprachliste einer Bibliothek ohne Deutsch** erlaubt (zum Beispiel Anime `["ja","en"]`)? Jede Liste muss `de` oder `en` enthalten (E-9), damit der Rückfall lesbar bleibt; ein de-Profil sähe bei Anime dann Englisch, wo es kein Deutsch gibt | **Erlaubt.** Die Serverliste muss `de` enthalten; eine Bibliothek darf davon abweichen, solange `de` oder `en` drin ist |
| F-10 | **Vorgabekette für Poster** `["de","en",""]` (deutsche Poster zuerst) oder `["en","de",""]` (wie heute faktisch)? Mit der Vorgabe «de» wechseln nach dem ersten «Jetzt anwenden» viele Poster sichtbar von Englisch auf Deutsch (12.2); neue Titel bekommen sofort deutsche Poster | **`["de","en",""]`.** Das ist der Zweck; die Bildwahl (Schritt 5) kommt vor dem grossen Lauf, damit jeder Einzelfall zurückgeht |

### 11.2 Offene Punkte, die der Bau klärt (Vorschlag steht, Widerspruch willkommen)

| Nr. | Punkt | Vorschlag |
|---|---|---|
| O-1 | Lokale Datei kleiner als die Mindestbreite: trotzdem nehmen? | Nein, dann zählt die Kette; die Datei bleibt als Kandidat in der Bildwahl (Prototyp und Papier sagen dasselbe) |
| O-2 | Logo ohne Logo-Datei: typografischer Titel in Logo-Optik oder normaler Text-Titel? Und: Logo in der vorgemalten Hülle von 08 (P-10)? | Normaler Text-Titel (kein Pseudo-Logo); Hülle nur, wenn die 08-Messung hält |
| O-3 | TMDB-SVG-Logos: liefert TMDB zu jedem `.svg` unter `.png` ein PNG? | In Schritt 2 gegen die Doku und mit einer Anfrage prüfen; ja → als PNG holen, nein → überspringen |
| O-4 | Fanart.tv: Schlüssel als Header oder Query, Rate, Kontingent, Bedeutung des `client-key` | In Schritt 6 gegen die Doku prüfen, bevor der Schlüssel auf «Integrationen» erscheint |

---

## 12. Risiken

### 12.1 Ein `apply` über das ganze NAS

Marcos Sammlung hängt per NFS am CT (M11: NAS-Stau bei Keyframes und Segmenten, 45 MB/s). Ein
`apply` mit Bildern schreibt rund 1'800 Originale plus Ableitungen in den Cache. Gegenmittel: der
Knopf statt Automatik (F-8), die Zahl vor dem Knopf, tiefe Priorität, Pause auf der Jobseite (09).

### 12.2 Die Kette ändert Bilder, die Marco gewohnt ist

Nach dem ersten `apply` mit `["de","en",""]` wechseln viele Poster von Englisch auf Deutsch.
Das ist das Ziel (F-10), aber sichtbar; gesperrte Poster bleiben, und die Bildwahl (Schritt 5)
macht jeden Einzelfall rückgängig. Darum kommt Schritt 5 **vor** dem grossen `apply` auf CT 190
(Schritt 7). Embys automatische Bilder im Ordner (F-5) könnten den Effekt umkehren; darum der
Schalter «Lokale Dateien».

### 12.3 Sprachen, die eine Quelle nicht kennt

TVDB kennt nach `/languages` einige Dutzend Sprachen (zu prüfen), TMDB über 180, AniList
zwei. Ein Titel in Rätoromanisch bleibt bei den meisten Quellen leer und fällt in der Kette
weiter. Die Seite sagt das vorher (7.1, Punkt 2).

### 12.4 `item_texts` und `item_genres` wachsen

Je Sprache eine Zeile je Titel, Staffel und Folge; mit fünf Sprachen und 40'000 Folgen
200'000 Zeilen statt 80'000. Der Suchindex (`00015_discover.sql`) wächst mit. Messung in
Schritt 1; die Latte ist `listLibraryItems` ±10 %.

### 12.5 Die zwei Sprachen stecken an mehr Stellen, als Fassung 1 sah

Der Kritiker hat elf Stellen gezählt (1.1). Schritt 1 ist darum der grösste Schritt, und eine
vergessene Stelle fällt nicht als Fehler auf, sondern als englischer Text in einem spanischen
Profil. Gegenmittel: ein Test, der mit einem Profil in der dritten Sprache **jede** Scoped-
und System-Operation aufruft und auf die dritte Sprache prüft (Vertragstest-Erweiterung).

### 12.6 Transparente PNGs in der Pipeline

Die Pipeline kennt nur JPEG und WebP als Ausgabe; PNG-Ausgabe und Alphakanal sind neu
(`transcode`). Ein Fixture-Logo mit Alpha im Test; `scaleLocked` muss den Medientyp des
Originals behalten.

### 12.7 Was dieses Papier nicht zählen konnte

Die Zahl der Serien und Staffeln auf dem NAS (nur «KinderSerien» ist aus 09 bekannt), die
Trefferquote von TMDB-Logos für Marcos Titel, die Grösse der Logos in Byte, die Latenz einer
TMDB-Anfrage vom CT aus, ob Marcos Emby-Bilder gewählt oder automatisch sind. Schritt 7 misst,
F-5 fragt.

---

## 13. Review-Protokoll (31. Oktober 2026)

Fassung 1 ging an den OMC-Kritiker (Prüfung gegen `main` `525c125` im Worktree). Ergebnis:
**1 Blocker, 19 Major, 31 Minor**; alle in Fassung 2 eingearbeitet. Die tragenden Befunde:

| Befund | Was Fassung 1 sagte | Fassung 2 |
|---|---|---|
| **Blocker:** `apply` reiht `fill` ein, aber `fill` macht bei einer Quelle keine Anfrage (`fill.go:252`), und Texte schreibt der Match | `metadata.apply` → `fill` | `apply` reiht **`metadata.match`** je Titel ein, wie `refreshItem` (6.5); Blocker-Test in Schritt 1 |
| Major 2, 3, 4, 5, 6: die Sprachwahl liegt an elf Stellen im Go-Code und in Genres, Trigger und Sortierschlüssel; Rückfall muss feldweise sein; Rang kann unlesbare Sprache zeigen | «Scoped-Abfragen mit min(rank)» | Tabelle der Stellen in 1.1; feldweise Kette `Metadatensprache → Oberflächensprache → Rang` (4.2); E-9 und F-9; `item_genres` neu gebaut; Trigger und Index neu angelegt; Schritt 1 auf 5 Tage |
| Major 7, 8: `format`-CHECK ohne `png`; Migration über drei Commits verteilt | eine Migration 00028 | `png` im CHECK, Formate je Art (6.3); **drei** Migrationen 00028 bis 00030, eine je Schritt (4.7) |
| Major 9, 10, 11, 12, 13: Bildsprache passt nicht in `item_field_sources`; Fremdquellen landen in `decideFields` am Ende; Schleifenreihenfolge mehrdeutig; lokale Quelle ohne Adressschema; «Titelordner» undefiniert | Sprache in `item_field_sources.lang`; `local` ohne Schema; Ordner vage | Tabelle `item_images` (4.4.1); Reihenfolge `["local", …, "fanart"]` für Bildfelder; Wahl in sechs Stufen mit «Kette aussen, Quelle innen» und Fixture (6.2); `file:`-Schema wie `coverSource`; Scanner füllt `item_local_images` mit klarer Ordnerregel (4.6) |
| Major 14, 15: Hand-Jobart ohne Payload liefe über alles mit Priorität 7; TMDB-Rate ist 20/s, nicht 40/10 s | Hand-Jobart; «23 Minuten» | Ereignis-Jobart mit Payload (6.5); Rechnung und F-8 neu begründet (8) |
| Major 16, 17, 18: Freigabeland wirkt auf Neuzugänge; Thumb ohne Frage gestrichen; Rest von Idee 01 unvollständig | «keine Stufe wird gesenkt» | Wirkung in 2.1, 6.6, F-7 mit Option; Thumb in F-3; Restliste in 1.3 |
| Major 19, 20: Profilseite ist `account`; ISO 639-1 verliert Regionen | `getMetadataSettings` (admin) | `getSession.availableMetadataLanguages` (4.3); BCP-47 (E-10) |
| Minor 1 bis 31 | Zeilenangaben, Schemadetails, Vorschau-Datenschutz, Cache-Ablauf, statische Sprachliste, `netguard`, SVG als PNG, TVDB-Struct, Fanart-Details, Stimmenzahl, lokale Vorrangregeln, 122-Titel-Annahme, Staffelposter aus, ehrlicher Fortschritt, Icon, Segmentleiste, Profilfeld, Offline-Logo, `listLibraryItems`, Vertragsstand, Fragen als Bestätigung, `Effective`-Leser, Prototyp-Satz zu O-1 | je an der Stelle korrigiert; neu `metadata.local_images`, F-9, F-10, 9.6, 12.5 |

Der Kritiker hat bestätigt: B-3, `ratingCountries`, TVDB-Typen 2/3, `bestArtwork`, `fill.go:539`
und `:542`, YouTube-Kontingent, Bildschranke 20 MB, `jobs.Type`, die Gruppe in `sections.ts`, die
TMDB-Annahmen (`append_to_response=images` mit `include_image_language=…,null`, `logos` mit
`iso_639_1` und `vote_average`, keine Banner/Disc/Clearart) und die Fanart-Artnamen nach seinem
Wissen. Fassung 2 ist **nicht** erneut geprüft.
