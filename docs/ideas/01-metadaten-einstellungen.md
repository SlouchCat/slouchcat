# Idee 01: Metadaten- und Scraper-Einstellungen je Bibliothek

- Status: Ausgearbeitet (Interview am 20. Oktober 2026, Mehrdeutigkeit 17%; Review durch den
  OMC-Kritiker am selben Tag eingearbeitet). **Teil A übernommen durch Konzept 07 (29. Oktober 2026):**
  die geordnete Anbieterliste, das feldweise Auffüllen in dieser Reihenfolge, die Herkunft je Feld
  (`item_field_sources`), die Freigabe-Ausnahme «nie senken» und TVDB als frei platzierbarer Erkenner
  sind mit `docs/planning/07-bibliotheken.md` (Frage 13, ADR 0015) beauftragt und gebaut, allerdings
  **auf Ordnerebene statt Bibliotheksebene** und ohne Serverstandard. **Teil B bleibt hier:** Sprachen
  je Bibliothek, Bildarten (Logo, Thumb, Banner, Disc, Clearart), Bildsprache, die Anbieter Fanart.tv,
  OMDb und lokale Dateien/NFO, die Bildwahl je Titel, das Land für die Altersfreigabe und die
  Rohzertifikate (`item_certifications`). Wer Teil B plant, baut auf `library_roots.providers` und
  `item_field_sources` auf und ergänzt dort nur Werte
- Frühestens: nach Version 1.0
- Interview: `.omc/specs/deep-interview-metadaten-einstellungen.md`
- Vorbild: Bibliothekseinstellungen und Bildbeschaffer-Liste in Emby

## 1. Problem und Ziel

Slouchcat holt heute für jeden Titel fest Deutsch und Englisch von TMDB, kennt nur Poster und Backdrop
und fragt TVDB nur, wenn TMDB nichts findet. Der Administrator kann weder Sprachen noch Bildarten noch
Anbieter beeinflussen. Für einen Haushalt mit Standardgeschmack reicht das; sobald eine Bibliothek anders
tickt (Anime mit japanischen Originaltiteln, eine Kinderbibliothek mit textlosen Postern, eine Sammlung mit
sorgfältig gepflegten lokalen Bildern), fehlt jede Stellschraube.

Ziel: Der Administrator legt auf Serverebene fest, welche Sprachen für Texte geholt werden, welche
Bildsprache und welche Bildarten gelten und welche Anbieter in welcher Reihenfolge Texte und Bilder
liefern. Jede Bibliothek kann einzelne Werte übersteuern. Je Titel kann er ein Bild von Hand wählen und
sperren. Das Land für die Altersfreigabe ist eine globale Einstellung.

## 2. Zielgruppe und Nutzen

- **Hauptbenutzer als Administrator:** weniger Nachpflege, weil Sprache und Bilder von Anfang an passen;
  ein Logo statt Text-Titel macht Startseite und Detailseite sichtbar hochwertiger.
- **Profile:** sehen Texte weiterhin in ihrer Sprache; neu auch in Sprachen ausserhalb von de/en.
- **Künftige Clients:** erhalten über die Bild-API Banner, Disc und Clearart, ohne dass die Web-App sie
  zeigen muss. Das setzt Clients voraus, die es in v1 nicht gibt (siehe Non-Goals unten).

## 3. Abgrenzung zur Spec v1

| Quelle | Heute | Konzept |
|---|---|---|
| B-3 «Metadaten und Bilder von TMDB und TVDB in Deutsch und Englisch; Sprache je Profil wählbar» | fest de/en | Sprachliste je Bibliothek, beliebige TMDB-Sprachen; Profil wählt weiterhin |
| ADR 0006 «TMDB zuerst, TVDB nur als Rückfall», verworfene Alternative «beide abfragen und zusammenführen» | Titelebene, Filme nur von TMDB | zwei geordnete Anbieterlisten, feldweise auffüllen. **Das ersetzt Teile von ADR 0006 und braucht einen Nachfolge-ADR** |
| AN-8 und Entscheid 6: Zertifikate in der Reihenfolge DE → AT → CH → GB → US | fest im Code (`ratingCountries`) | gewähltes Land zuerst, Rest in heutiger Reihenfolge; Abbildung auf FSK bleibt. **Ändert Entscheid 6, braucht einen Eintrag im Entscheidungsprotokoll** |
| K-2 Altersstufen FSK 0/6/12/16/18 | unverändert | unverändert |
| B-6 manuelle Zuordnung, `items.locked_fields` | Titel ↔ TMDB/TVDB-ID; Sperren für Felder, Texte, Poster, Backdrop | bleibt; neu zusätzlich Bildwahl je Titel über dieselbe Sperrliste |
| Tabelle `settings` | seit M1a vorhanden, Seite mit verschlüsselten Schlüsseln folgt in M8 | neue Schlüssel `metadata.*` |

**Berührte Non-Goals:** Banner, Disc und Clearart haben in der Web-App keinen Platz und nützen erst
Clients, die ein Non-Goal von v1 sind (TV-Apps, native Apps). Das Konzept baut diese Clients nicht; es
holt die Bildarten nur, wenn der Administrator sie einschaltet. Sonst zieht es nichts aus M4 bis M10 vor.

**Bewusst ausgeschlossen (Interview, Runde 0 und 10):** eingebettete Titel bevorzugen, Bibliothek von
der globalen Suche ausschliessen, Ordner zusammenführen, Wechsel des Stufensystems für Freigaben,
Textübersteuerung je Titel, Zusammenführen mehrerer Anbieter innerhalb eines Feldes.

## 4. Entscheide aus dem Interview

| Frage | Entscheid |
|---|---|
| Vorrang bei der Textsprache | Profil gewinnt. Die Bibliothek bestimmt, welche Sprachen geholt werden, und die Rückfallreihenfolge |
| Bildsprache | Ein Bild je Bibliothek, Art und Titel, für alle Profile gleich; Rückfallkette z. B. de → ohne Text → en |
| Bedeutung der Anbieterreihenfolge | Feldweise auffüllen: der erste Anbieter mit Wert gewinnt das Feld; Herkunft je Feld wird gespeichert. Ausnahme Freigabe, siehe 6 |
| Ebene | Server-Standard, jede Bibliothek übersteuert einzelne Felder («wie Server» ist Standard); kein eigenes Profil-Objekt |
| Bildarten | Logo und Thumb mit Platz im Web-Design; Banner, Disc, Clearart nur holen, cachen, per API liefern |
| Anbieter | TMDB (Pflicht, stellt die Identität fest), TVDB (frei platzierbar), Fanart.tv (nur Bilder), OMDb, lokale Dateien/NFO |
| Änderung einer Einstellung | sofort Hintergrund-Job «Metadaten anwenden» für alle Titel der Bibliothek, Fortschritt sichtbar |
| Sprachen | geordnete Liste beliebiger TMDB-Sprachen je Bibliothek |
| Übersteuerung je Titel | Bildwahl je Art aus allen Kandidaten, gesperrt; keine Textübersteuerung |
| Land für Altersfreigabe | global; wählt nur, welches Zertifikat zuerst gelesen wird; Anzeige «12 (CH)» |

## 5. Nutzerfluss

**Einstellungen → Metadaten (Server):**
1. Sprachen für Texte: geordnete Liste (z. B. Deutsch, Englisch). Hinzufügen aus der TMDB-Sprachliste.
2. Bildsprache: geordnete Kette, Eintrag «ohne Text» erlaubt.
3. Bildarten: Poster und Backdrop fest; Logo, Thumb, Banner, Disc, Clearart je ein Schalter.
   Standard: Logo und Thumb an.
4. Anbieter für Texte: Liste mit Schalter und Griff zum Sortieren (TMDB fest an, TVDB, OMDb, lokale
   NFO). Anbieter für Bilder: dieselbe Art Liste (lokale Dateien, Fanart.tv, TMDB, TVDB). Ein Anbieter
   ohne Schlüssel zeigt «nicht konfiguriert» mit Link zur Schlüsselverwaltung.
5. Land für Altersfreigabe: Auswahl nur aus Ländern mit FSK-Abbildung (DE, AT, CH, GB, US).

**Bibliotheken → Bibliothek bearbeiten → Metadaten:** dieselben Felder 1 bis 4, jedes mit Schalter
«wie Server». Speichern zeigt eine Vorschau je Anbieter: «2'143 Filme: TMDB rund 6'500 Anfragen (etwa
6 Minuten), Fanart.tv 2'143, OMDb 2'143 (Kontingent 1'000 pro Tag, etwa 3 Tage).» Danach läuft der
Job, sichtbar auf der Bibliotheksseite und im Dashboard. Eine Änderung auf Serverebene startet je
Bibliothek einen Job, die das betroffene Feld auf «wie Server» hat.

**Titel bearbeiten (Admin) → Bilder:** je Bildart die Kandidaten aller aktiven Anbieter als Kacheln mit
Sprache, Auflösung, Anbieter und Bewertung; lokale Kandidaten über einen Admin-Vorschau-Endpunkt. Ein
Klick wählt und sperrt (Schloss-Symbol). «Sperre aufheben» gibt den Titel wieder der Regel zurück. Je
Textfeld steht die Herkunft («Beschreibung (de): TMDB»).

**Zuschauer:** Detailseite und Startseiten-Karussell zeigen das Logo über dem Backdrop statt des
Text-Titels; ohne Logo bleibt der Text-Titel. Querformat-Reihen («Weiterschauen») und Folgenlisten
nutzen das Thumb; bei Folgen ist das Thumb das Standbild (Still), das heute als `backdrop` gespeichert
ist. Alles andere bleibt gleich.

## 6. Datenmodell und API-Skizze

**Einstellungen** (Tabelle `settings`, Schlüssel `metadata.*`):

| Schlüssel | Typ | Beispiel |
|---|---|---|
| `metadata.text_langs` | JSON-Liste BCP-47 | `["de","en"]` |
| `metadata.image_lang_chain` | JSON-Liste, `""` = ohne Text | `["de","","en"]` |
| `metadata.image_kinds` | JSON-Liste | `["poster","backdrop","logo","thumb"]` |
| `metadata.text_providers` | JSON-Liste, geordnet | `["tmdb","tvdb","omdb","local"]` |
| `metadata.image_providers` | JSON-Liste, geordnet | `["local","fanart","tmdb","tvdb"]` |
| `metadata.rating_country` | Ländercode aus `fskTables` | `"CH"` |

**Bibliothek:** Tabelle `library_metadata (library_id PK, text_langs, image_lang_chain, image_kinds,
text_providers, image_providers)`, jede Spalte `NULL` = «wie Server». `metadata.EffectiveSettings
(libraryID)` löst die Übersteuerung auf und wird nur von Jobs gelesen. Damit die Leseseite keine
Einstellungen braucht, schreibt der Job den Rang jeder Sprache in die Zeile: `item_texts.rank`. Die
Scoped-Abfrage wählt Profilsprache, sonst den kleinsten Rang, sonst `items.title`, ohne `settings` oder
`library_metadata` zu lesen (Regel 2 bleibt einfach).

**Texte:** `item_texts.lang` verliert den CHECK auf de/en (Tabellenneubau nach Vorlage); neue Spalte
`rank`. `profiles` erhält `metadata_language` getrennt von `language` (Oberfläche bleibt de/en, weil die
Oberflächentexte aus `web/src/locales` kommen).

**Herkunft:** eine Quelle der Wahrheit, `item_field_sources (item_id, field, lang NULL, provider,
PRIMARY KEY (item_id, field, lang))`, für title, original_title, overview, tagline, genres,
runtime_minutes, release_date, own_rating, cast. `items.rating_source` bleibt als Kurzform für die
Anzeige.

**Freigabe (Ausnahme von der feldweisen Regel):** Die Zertifikate aller aktiven Anbieter werden roh
gespeichert: `item_certifications (item_id, provider, country, value)`. Die Abbildung läuft über die
Länderkette (gewähltes Land, dann DE → AT → CH → GB → US); der Anbieterrang entscheidet nur innerhalb
eines Landes. So kann ein höher gereihter OMDb-Wert «PG-13» nie eine deutsche 16 auf 12 senken (K-3).
Quellen `nfo` und `manual` bleiben wie heute unantastbar. Ein Wechsel des Landes ist damit eine lokale
Neuberechnung ohne Netzanfragen. Negativtest «Anbieterreihenfolge senkt die Freigabe nicht» wird
Pflicht im Kinderprofil-Vertragstest.

**Identität und IDs:** TMDB stellt weiterhin die Identität fest (Suche, manuelle Zuordnung B-6); die
`external_ids` liefern `tvdb_id` und `imdb_id`. Fanart.tv sucht Serien nur über die TVDB-ID, OMDb nur
über die IMDb-ID; fehlt die ID, überspringt der Job den Anbieter für diesen Titel. TVDB bleibt auf Serien
beschränkt (im Code liefert `SearchMovie` nichts, `Movie` gibt `ErrNotFound`).

**Folgen und TVDB:** TVDB-Texte werden in Folgen nur eingemischt, wenn die Folge über die TMDB-
`external_ids` dieselbe TVDB-Folgen-ID trägt; sonst nur auf Serien- und Staffelebene. Das verhindert den
Fall, den ADR 0006 fürchtet (andere Staffelzählung bei Anime und Tages-Shows).

**Sprachabbildung je Anbieter:** TMDB `xx-YY`, TVDB ISO 639-2 (heute fest `deu`/`eng`, wird zur
Tabelle), Fanart.tv `xx` und `00` für «ohne Text», OMDb nur Englisch. Die Anbieter-Schnittstelle bekommt
`Languages() []string`, damit die Einstellungsseite zeigen kann, welche Sprache ein Anbieter nicht kennt.

**Bildarten je Anbieter und Titelart:**

| Art | Film | Serie | Staffel | Folge | Quellen |
|---|---|---|---|---|---|
| poster | ja | ja | ja | – | TMDB, TVDB, Fanart.tv, lokal |
| backdrop | ja | ja | – | – | TMDB, TVDB, Fanart.tv, lokal |
| logo | ja | ja | – | – | TMDB (PNG und SVG), TVDB, Fanart.tv (`hdmovielogo`, `hdtvlogo`), lokal |
| thumb | ja | ja | – | ja (= Still) | Fanart.tv (`moviethumb`, `tvthumb`), lokal; Folgen: TMDB-Still |
| banner | – | ja | ja | – | TVDB, Fanart.tv, lokal |
| disc | ja | – | – | – | Fanart.tv, lokal |
| clearart | ja | ja | – | – | Fanart.tv, lokal |

Folgen-Stills, die heute als `backdrop` liegen, werden per Migration zu `thumb`. Ohne Fanart.tv gibt es
für Filme und Serien kein Thumb; die Web-App fällt auf das beschnittene Backdrop zurück.

**Bilder:** `images.kind` um `logo`, `thumb`, `banner`, `disc`, `clearart` erweitert; neue Spalte
`provider`. Transparente Arten (logo, clearart, disc) werden als WebP ausgeliefert, mit PNG statt JPEG
als Rückfall für Clients ohne WebP; die Grössenstufen (`size`-Enum in `openapi.yaml`) werden um
w300/w500/w800 ergänzt. SVG-Kandidaten von TMDB werden ausgefiltert oder gerastert (ffmpeg nur mit
librsvg); die Pipeline akzeptiert heute nur JPEG und PNG. `scaleLocked` muss den Medientyp des Originals
behalten statt JPEG anzunehmen.

**Kandidaten und Sperre:** `image_candidates (id PK, item_id, kind, provider, lang, source_url NULL,
rel_path NULL, width, height, score)` nur als Verzeichnis, ohne Dateien; lokale Kandidaten tragen
`rel_path`. Die Wahl steht in `item_image_choice (item_id, kind, candidate_id)`, und die Sperre ist ein
Eintrag in `items.locked_fields` (heute schon `poster`, `backdrop`; neu alle Arten). Keine zweite
Sperrspalte an `images`, weil dort je Grösse und Format eine Zeile liegt.

**Anbieter:** `metadata.Provider` bleibt für Texte; neu `metadata.ImageProvider` mit
`Images(ctx, ids, kinds) ([]Candidate, error)`. Implementierungen: `tmdb` (beides), `tvdb` (beides, nur
Serien), `fanart` (Bilder; Schlüssel als Header `api-key` und optional `client-key`), `omdb` (Texte en,
Bewertungen), `local` (beides; liest nur über `paths`; nutzt den vorhandenen Parser `library.ParseNFO`
für Plot, Tagline, Freigabe und IDs). Dateinamen in beiden Konventionen: `poster.jpg`, `fanart.jpg`,
`logo.png` oder `clearlogo.png`, `landscape.jpg` oder `thumb.jpg`, `banner.jpg`, `disc.png` oder
`discart.png`, `clearart.png`, `<datei>-poster.jpg`, `<datei>-thumb.jpg`, `seasonNN-poster.jpg`,
`movie.nfo`, `tvshow.nfo`.

**Zusammenführung:** Für jeden Titel läuft die Textliste in Reihenfolge; je Feld und Sprache gewinnt der
erste nicht-leere Wert (Ausnahme Freigabe, oben). Für Bilder sammelt der Job je Art die Kandidaten aller
aktiven Anbieter und wählt: gesperrte Wahl → Anbieter-Rang → Sprachkette → höchste Auflösung/Bewertung.
Nur das gewählte Bild wird geholt; die Pipeline (Grössen, Akzentfarbe) bleibt.

**Jobs:** Heute gibt es `metadata.match`, `image.fetch` und `trailer.find`. Neu `metadata.apply` je
Bibliothek (niedrige Priorität, ein Job je Bibliothek gleichzeitig), der je Titel `metadata.match` und
`image.fetch` einreiht und Fortschritt zählt. Jeder Anbieter bekommt einen eigenen Zustand je Titel wie
heute `trailer_state` (`item_provider_state (item_id, provider, state, attempted_at)`), damit ein leeres
OMDb-Kontingent nicht den ganzen Titel auf `pending` setzt und `image.fetch` ihn überspringt.
Kontingente wie bei der Trailer-Kette (`ratelimit.Quota`): OMDb 1'000 pro Tag; TMDB ohne Tageslimit,
nur Rate. Die Vorschau rechnet Anfragen je Anbieter und Titelart (Serien: je Staffel eine Anfrage, bei
40'000 Folgen aus dem M3-Lasttest also deutlich mehr als bei Filmen) und nennt Kontingent-Tage.

**API (alle `admin`, ausser Auslieferung):**

| Operation | x-visibility |
|---|---|
| `GET/PUT /admin/settings/metadata` | admin |
| `GET/PUT /admin/libraries/{id}/metadata` | admin |
| `POST /admin/libraries/{id}/metadata/apply` (Vorschau je Anbieter, dann Start) | admin |
| `GET /admin/items/{id}/image-candidates?kind=` | admin |
| `GET /admin/items/{id}/image-candidates/{cid}/preview` (lokale Kandidaten über `paths`) | admin |
| `PUT /admin/items/{id}/images/{kind}` `{candidateId}` / `DELETE` (Sperre aufheben) | admin |
| `GET /admin/items/{id}/field-sources` | admin |
| `GET /admin/providers` (Name, konfiguriert, Sprachen, Kontingent) | admin |
| `GET /items/{id}/images/{kind}/{size}` mit neuen `kind`-Werten | scoped (unverändert) |

## 7. Berührungspunkte mit den harten Regeln

| Regel | Berührung |
|---|---|
| 1 Generierter Code | OpenAPI, Queries und Migrationen als Quelle; `task gen` |
| 2 Sichtbarkeit | Admin-Operationen sind `admin`; Bildauslieferung bleibt `scoped` über `visible_items`; Scoped-Abfragen lesen keine Einstellungen (Rang in der Zeile); Kinderprofil-Vertragstest um den Freigabe-Negativtest erweitert |
| 4 Dateizugriff | lokale Bilder und NFO nur über `paths`, relativ zur Wurzel; keine Symlinks nach aussen |
| 5 Migrationen | `item_texts` und `images` Neubau nach Vorlage (CHECK ändern), Still-Zeilen umhängen, eingefrorener DB-Stand |
| 7 x-visibility | acht neue Operationen mit Eintrag im Vertragstest |
| 8 Tastaturtest | neue Admin-Seiten (Metadaten-Einstellungen, Bildwahl) mit Playwright-Tastaturtest |
| 9 Keine Texte hart | Sprachnamen über `Intl.DisplayNames`, Anbieternamen aus Locales |
| 10 Secrets | `fanart_api_key`, `fanart_client_key`, `omdb_api_key` in die Redaction-Liste (Regex um `client_key` ergänzen) und in den Test; beide in die Liste der verschlüsselten Secrets (Plan 3.8) |
| 12 Entscheid im Server | Sprach- und Bildwahl passiert im Job; der Client fragt nur `kind` ab |

## 8. Offene Fragen

1. **Profilsprache:** Wie wählt ein Profil seine Metadatensprache, wenn Bibliotheken unterschiedliche
   Listen führen? Vorschlag: `metadata_language` je Profil aus der Vereinigung aller Bibliothekslisten,
   Rückfall je Bibliothek über den Rang.
2. **TVDB vor TMDB bei Serien:** Soll eine Bibliothek eine Nummerierungsregel («TVDB-Reihenfolge» vs.
   «DVD-Reihenfolge») bekommen, damit Dateien bei abweichender Staffelzählung richtig zugeordnet werden?
3. **Bewertungen von OMDb:** Slouchcat zeigt heute keine Bewertungen. Eigene Idee, oder gehört ein
   Bewertungsfeld auf die Detailseite zu diesem Konzept?
4. **Cache-Grösse:** fünf zusätzliche Bildarten je Titel bei 60'000 Titeln (Lasttest M3). Grenzwert,
   Messung und Aufräumregel für ausgeschaltete Arten?
5. **Fanart.tv-Schlüssel (nicht gegen die Doku geprüft):** Der persönliche Schlüssel steuert vermutlich,
   wie schnell neue Bilder erscheinen (etwa 7 Tage nur mit Projektschlüssel, 48 Stunden mit persönlichem),
   nicht ein Anfragekontingent. Vor der Umsetzung prüfen.
6. **SVG-Logos von TMDB:** Hat TMDB zu jedem SVG eine PNG-Variante, oder muss gerastert werden?
7. **Ohne Logo:** Soll die Web-App bei fehlendem Logo einen typografischen Titel in Logo-Optik rendern
   (einheitliches Bild) oder den normalen Text-Titel?

## 9. Aufwand und Abhängigkeiten

| Teil | Arbeitstage |
|---|---|
| Beliebige Sprachen (item_texts mit Rang, Profil, API, Oberfläche) | 3–4 |
| Bildarten, Transparenz in der Pipeline, Still-Migration, Logo und Thumb im Design | 5–7 |
| Anbieter Fanart.tv | 2–3 |
| Anbieter OMDb | 1–2 |
| Anbieter lokale Dateien und NFO (Parser vorhanden), Vorschau-Endpunkt | 2–3 |
| TVDB frei platzierbar, Sprachtabelle, Folgenabgleich über IDs | 3–4 |
| Server- und Bibliothekseinstellungen mit Oberfläche und Tastaturtests | 3–4 |
| Feldweise Zusammenführung, Herkunft, Freigabe-Ausnahme mit Rohzertifikaten | 3–4 |
| Job «Metadaten anwenden» mit Anbieterzustand, Vorschau und Kontingenten | 2–3 |
| Bildwahl je Titel (Kandidaten, Sperre, Oberfläche) | 2–3 |
| Migrationen mit eingefrorenen DB-Ständen, Vertrags- und Schematests, ADR, Cache-Messung | 2–3 |
| **Summe** | **28–40** |

Abhängigkeiten: Einstellungsseite mit verschlüsselten Schlüsseln (M8), Anbieter-Schnittstelle (ADR 0006,
M3), Bildpipeline (M1b), Job-Queue mit Kontingenten (ADR 0002, M3). Bei Übernahme fällig: ein ADR für
`ImageProvider`, die feldweise Zusammenführung und die Freigabe-Ausnahme (Nachfolger von ADR 0006) sowie
ein Eintrag im Entscheidungsprotokoll zu Entscheid 6 (Länderreihenfolge).
