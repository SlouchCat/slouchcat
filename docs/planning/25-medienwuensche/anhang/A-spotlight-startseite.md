# Bestandsanalyse A: Spotlight, Trakt, Startseite, Detailseiten, Suche, externe IDs

Stand: Haupt-Checkout `main` (`ba6efe31`, Vertrag `api/openapi.yaml` Version 1.40.0, letzte Migration
00065), gelesen am 16. November 2026. Nur gelesen, nichts geändert, nichts gebaut oder getestet.

Kennzeichnung: **[gelesen]** = im Code oder Papier nachgesehen, mit Beleg. **[vermutet]** = Schluss
aus dem Gelesenen, nicht ausgeführt oder nicht bis zum Ende verfolgt. Pfade relativ zum Repo-Wurzel.

---

## 0. Vorweg: Was die Papiere heute zum Wünschen sagen [gelesen]

Das ist für das Konzept die wichtigste Randbedingung, darum vorne:

- Spec, Statuszeile Integrationen: «Wünschen über Seerr … zurückgestellt (Runden 12 und 20)»
  (`docs/spec/01-anforderungen-v1.md:35`).
- Spec: «Wünschen von Inhalten erfolgt ausserhalb von Slouchcat direkt in Seerr …; Slouchcat baut
  weder eine eigene Wunschverwaltung noch einen Seerr-Bereich» (`docs/spec/01-anforderungen-v1.md:46`).
- Non-Goals: «Eigene Wunschverwaltung und ein Bereich „Wünschen“ über die Seerr-API»
  (`docs/spec/01-anforderungen-v1.md:66`), «Steuerung von Radarr, Sonarr, SABnzbd»
  (`docs/spec/01-anforderungen-v1.md:69`); I-1 bis I-4, I-6 entfallen (`:159`), Seerr-Einbettung
  entfallen (`:185`).
- Entscheid 5 (19.10.2026) hatte «Wünschen über eigene Seite via Seerr-API, keine Einbettung»
  beschlossen (`docs/planning/04-entscheidungen.md:14`); **Entscheid 10** (22.10.2026) hat I-4 und
  5.1 ersatzlos gestrichen, «Statusanzeige und Wünschen als Kandidaten für die Ideensammlung»
  (`docs/planning/04-entscheidungen.md:19`). Auch die ursprüngliche Regel «Profile des Hauptkontos ab
  FSK 12 dürfen wünschen, Gäste nicht; jedem Konto ein Seerr-Benutzer» steht dort als Vorschlag.
- **S-4** (Frage 11): «Titel ohne Bibliothekstreffer erscheinen **nie** auf der Startseite
  (Non-Goal «eigene Wunschverwaltung») …, in der Verwaltung aber schon»; der Schalter «Nur vorhandene
  Titel zeigen» entfiel (`docs/planning/04-entscheidungen.md:20`, `docs/adr/0011-spotlight.md:99`,
  `docs/planning/05-plan-spotlight.md:84-85,291,1622`).
- CLAUDE.md Regel 11 nennt «eigene Wunschverwaltung, Steuerung von Arr/Seerr» als Nicht-Ziel.

**Folge:** Das Wunsch-Konzept braucht eine neue Frage im Entscheidungsprotokoll, die Spec-Zeilen 46,
66, 69 (teilweise) und S-4 ablöst, und eine ADR. Der Code verhindert es nicht, die Papiere schon.

---

## 1. Spotlight heute

### 1.1 Herkunft der Einträge [gelesen]

- **Eine** öffentliche Trakt-Liste, in der Verwaltung gewählt (Singleton `spotlight_config`). Sieben
  Listenarten: `trending`, `popular`, `watched_weekly`, `collected_monthly`, `anticipated`,
  `boxoffice` (nur Filme), `url` (öffentliche Benutzerliste) (`server/internal/trakt/trakt.go:31-40,93-101`;
  CHECK in `server/migrations/00021_spotlight.sql:76-78`).
- **Kuratierung** des Administrators je Eintrag: ausblenden, an Platz N anheften, Notiz
  (`spotlight_overrides`, `server/migrations/00021_spotlight.sql:111-126`).
- Keine Empfehlungsberechnung, keine Picks im Spotlight (Picks sind eine eigene Reihe, nie Bühne:
  `docs/adr/0011-spotlight.md:56`). Watchlist-Adressen werden abgelehnt
  (`server/internal/trakt/trakt.go:174-176`, `server/internal/admin/spotlight.go:102-106`).

### 1.2 Datenmodell [gelesen]

| Tabelle | Spalten | Schlüssel / Index | Beleg |
|---|---|---|---|
| `spotlight_config` | `id`(=1), `enabled`, `list_key`, `list_url`, `media` (movie/show/both), `item_count` (5–20), `min_hits` (0–10), `interval_hours` (6–48), `last_manual_refresh`, `updated_at`, **`rotation_seconds`** (3–15, Vorgabe 5, seit 00026) | Singleton | `server/migrations/00021_spotlight.sql:73-91`, `server/migrations/00026_library_views.sql:22` |
| `spotlight_entries` | `list_key`, `media` (movie/show), `position`, `tmdb_id`, `title`, `year`, `fetched_at` | PK `(list_key, media, position)`, `WITHOUT ROWID`; Index `spotlight_entries_tmdb (media, tmdb_id)` | `server/migrations/00021_spotlight.sql:95-105` |
| `spotlight_overrides` | `list_key`, `media`, `tmdb_id`, `hidden`, `pinned_rank`, `note` (≤280), `updated_at` | PK `(list_key, media, tmdb_id)`; Unique `spotlight_overrides_rank (list_key, pinned_rank) WHERE pinned_rank IS NOT NULL`; **kein FK auf `items`** | `server/migrations/00021_spotlight.sql:111-126` |
| Einstellungen | `settings`-Schlüssel `trakt_last_success`, `trakt_last_attempt`, `trakt_last_error` | – | `server/internal/metadata/trakt.go:41-44` |

Migrationen: 00015 (alte `trakt_lists`, `server/migrations/00015_discover.sql:312-324`), 00021
(Spotlight, löscht `trakt_lists`, `:134`), 00026 (`rotation_seconds`). Danach keine Änderung mehr an
den Spotlight-Tabellen (grep über `server/migrations/`).

### 1.3 Abgleich mit der Bibliothek [gelesen]

- **Nur über TMDB-ID.** Der Trakt-Client liest aus `ids` ausschliesslich `tmdb`
  (`server/internal/trakt/trakt.go:270-273`); Einträge ohne TMDB-ID werden verworfen, Positionen haben
  dann Lücken (`server/internal/trakt/trakt.go:184-185,392`). IMDb-, TVDB-, Trakt-ID und Slug werden
  **nicht** gelesen und **nicht** gespeichert.
- Join: `items.tmdb_id = e.tmdb_id AND items.kind = CASE e.media WHEN 'movie' THEN 'movie' ELSE
  'series' END AND deleted_at IS NULL`, dann `visible_items` des Profils
  (`server/queries/scoped/spotlight.sql:9-16`). Gleiche Regel ohne Profil für die Verwaltung
  (`server/queries/system/spotlight.sql:22-34`).
- Ordnung in Go: Dedup, Angeheftete einfügen, `min_hits` vor dem Schnitt prüfen, dann auf
  `item_count` schneiden (`server/internal/visibility/discover.go:383-447`,
  `server/internal/discover/discover.go:380-393`).

### 1.4 Einträge ohne Bibliothekstreffer [gelesen]

- **Gespeichert, nicht verworfen**: Der Abruf schreibt alle Einträge mit TMDB-ID in
  `spotlight_entries`, unabhängig von der Bibliothek (`server/internal/metadata/trakt.go:291-298`).
- **Startseite**: nie (innerer Join, `server/queries/scoped/spotlight.sql:10`).
- **Verwaltung**: ja, `getSpotlightEntries` listet sie mit Status «Nicht in der Bibliothek», ohne
  `itemId` und ohne Poster (`server/queries/system/spotlight.sql:22-34`,
  `api/openapi.yaml:6636-6643,17152-17154`). In der Oberfläche haben sie **keine** Knöpfe
  (Anheften, Ausblenden, Notiz), nur den Text «Nicht in der Bibliothek, erscheint nicht auf der
  Startseite» (`web/src/features/admin/spotlight/columns.tsx:106,126-129`), obwohl der Server ein
  Override auch ohne Treffer annimmt (`server/internal/admin/spotlight.go:178-189`).

### 1.5 Was je Eintrag gespeichert ist [gelesen]

Nur `media`, `position`, `tmdb_id`, `title` (so wie Trakt ihn nennt), `year`, `fetched_at`
(`server/internal/trakt/trakt.go:186-192`, `server/queries/system/spotlight.sql:8-10`).
**Kein** Poster, Backdrop, keine Beschreibung, keine Altersfreigabe, keine Laufzeit, keine Genres,
keine Übersetzung. Bilder zeigt die Verwaltung nur für Treffer über `getAdminItemImage`
(`server/internal/admin/spotlight.go:298-310`, `api/openapi.yaml:17169-17171`).
[vermutet] Trakt liefert den Titel englisch; geprüft habe ich die Trakt-Sprache nicht.

### 1.6 Job, Takt, Rate-Limit [gelesen]

- Job `trakt.refresh`, Idempotenzschlüssel `trakt`, Priorität 2 (Zeitplan) bzw. 7 (manuell und bei
  Schlüsselwechsel, hebt einen wartenden an) (`server/internal/metadata/trakt.go:28-35,195-205`;
  Entscheid 26.10.2026 `docs/planning/04-entscheidungen.md:71`).
- Zeitplan alle `interval_hours` (6–48, Vorgabe 24); dieselbe Zahl ist auf der Seite «Jobs» und
  «Spotlight» einstellbar («ein Wert, zwei Türen», `server/queries/app/spotlight.sql:16-19`,
  `server/internal/admin/spotlight.go:136-139`, `server/internal/admin/jobtypes.go:252-258,677`).
- Abruf: je Medienart ein Request `?limit=100` (`MaxLimit`), bei `both` zwei Requests; Rate-Limit
  **1 Anfrage/s, Burst 2** (`server/internal/trakt/trakt.go:49-59,243`,
  `server/internal/metadata/trakt.go:105-107`), Timeout 15 s.
- Ein Lauf ersetzt die Tabelle in **einer** Transaktion; ein Fehlschlag **leert** sie und speichert den
  Fehler (`server/internal/metadata/trakt.go:278-313`). 5xx/429 → Wiederholung mit Backoff der Queue;
  401/403 → kein Retry (`server/internal/metadata/trakt.go:243-254`).
- Ohne Client-ID: kein Job, Einträge werden geleert (`server/internal/metadata/trakt.go:127-140,176-190`).
- `enabled = 0` blendet nur die Reihe aus, der Abruf läuft weiter (`server/internal/metadata/trakt.go:143-145`,
  `api/openapi.yaml:17043`).
- Veraltung: Einträge älter als 2 × `interval_hours` werden nicht gezeigt
  (`server/internal/visibility/discover.go:395`).
- «Jetzt abrufen» höchstens einmal je Minute, Zeitstempel in der DB (`server/internal/admin/spotlight.go:35-37,254-273`).

---

## 2. Spotlight-API und -Client

### 2.1 OpenAPI-Operationen [gelesen]

| operationId | Methode, Pfad | x-visibility | Antwort | Beleg |
|---|---|---|---|---|
| `getHome` | GET `/home?start=K` | scoped | `HomePage {rows[≤15], heroRow?, heroStart?, heroIntervalSeconds?}` | `api/openapi.yaml:1234-1300,12256-12286` |
| `listHomeRow` | GET `/home/rows/{key}` (`spotlight`, `continue`, `recent`, `released`) | scoped | `ItemPage`; 404 wenn Spotlight aus / unter `min_hits` | `api/openapi.yaml:1302-1368,12144-12149` |
| `getTraktStatus` | GET `/admin/trakt` | admin | `TraktStatus {configured, lastSuccess?, lastAttempt?, lastError?, listKey, entries}` | `api/openapi.yaml:6548-6569,16981-17006` |
| `getSpotlightConfig` | GET `/admin/spotlight` | admin | `SpotlightConfig {enabled, rotationSeconds, listKey, listUrl?, media, itemCount, minHits, intervalHours, listKinds[]}` | `api/openapi.yaml:6571-6595,17037-17075` |
| `setSpotlightConfig` | PUT `/admin/spotlight` | admin | `SpotlightConfig`; Body `SpotlightConfigUpdate` (`rotationSeconds` optional) | `api/openapi.yaml:6596-6630,17077-17110` |
| `getSpotlightEntries` | GET `/admin/spotlight/entries` | admin | `SpotlightEntries {listKey, entries[≤200], orphans[]}` | `api/openapi.yaml:6632-6656,17112-17128` |
| `setSpotlightEntryOverride` | PATCH `/admin/spotlight/entries/{media}/{tmdbId}` | admin | `SpotlightEntry`; 409 `rank_taken` | `api/openapi.yaml:6658-6719` |
| `refreshSpotlight` | POST `/admin/spotlight/refresh` | admin | 202; 422 ohne Schlüssel; 429 unter 1 min | `api/openapi.yaml:6721-6749` |
| `getAdminItemImage` | GET `/admin/items/{id}/images/{kind}/{size}` | admin | Bild (http.Handler) | `api/openapi.yaml:4370` |

`SpotlightEntry`: `media`, `tmdbId`, `position`, `title?`, `year?`, `itemId?`, `itemTitle?`,
`hidden`, `pinnedRank?`, `note?`, `orphaned`, `posterUrl?` (`api/openapi.yaml:17130-17171`).
Die Startseite liefert Spotlight-Titel als `ItemRef` (nur Bibliothekstitel, `api/openapi.yaml:11677-11804`)
mit `note` und `hasTrailer` (nur in `spotlight` und `picks`, `:11793-11804`). **`ItemRef` hat
keine `tmdbId`** (nur `ItemDetail.tmdbId`/`tvdbId`, `:11854,11862`).

### 2.2 Web-Komponenten [gelesen]

- Bühne: `web/src/ui/HeroCarousel.tsx` (662 Zeilen), eingebunden in
  `web/src/features/catalog/HomePage.tsx:586-616`. Die Bühne ist die Reihe aus `heroRow`
  (`HomePage.tsx:283`); der Client entscheidet sie nicht (Regel 12).
- Liste hinter dem Titel: `/lists/spotlight` → `web/src/features/catalog/HomeListPage.tsx`
  (`web/src/app/router.tsx:662`, `web/src/app/paths.ts:64`).

**Aktionen der Bühne** (`HeroCarousel.tsx:57-110,190-197`, `HomePage.tsx:589-614`, Texte `de.json`):

| Element | Beschriftung | Bedingung |
|---|---|---|
| Überschrift als Link zur ganzen Liste | «Spotlight» (`home.rows.spotlight`) | immer |
| Abspielen | «Abspielen» (`item.play`); begonnener Film fragt «Weiterschauen ab …/Von vorn starten» | immer (`HomePage.tsx:603-606`) |
| Details | «Details» (`item.details`) | immer |
| Trailer | «Trailer» (`item.trailer`), öffnet `TrailerPanel` | nur `hasTrailer === true` |
| Pause/Weiter | «Pause» / «Weiter» (`home.spotlight.pause/resume`), `aria-pressed` | immer bei ≥2 Folien |
| Pfeile vor/zurück | `ui.row.previous/next` | nur ab 768 px (`HeroCarousel.module.css:237-240`) |
| Posterreihe darunter | «Spotlight-Titel» (`home.spotlight.row`); Druck stellt den Titel ins Band, öffnet **nicht** die Detailseite | immer |
| An Postern: Stift (Metadaten), Merken, Gesehen-Haken | `CoverPen` (nur Admin-Hauptprofil), `CoverWatchlist`, `CoverWatched` | `HomePage.tsx:317-339` |
| Notiz | als Bildunterschrift statt Jahr | wenn `note` (`HomePage.tsx:165-166`) |

Ton/Autoplay von Trailern im Band: **nicht vorhanden** [gelesen: kein Ton-Knopf in den Props
`HeroCarousel.tsx:57-110`]. Automatik: Wechsel alle `heroIntervalSeconds`; Start bei `heroStart`
(Server würfelt, ADR 0016); Pause bei Tastaturfokus, Fokus in der Posterreihe, offenem Trailer,
verborgenem Tab, einer Folie, «Bewegung reduzieren» (`HeroCarousel.tsx:183-189,247-248,293`). Nicht
beim Mauszeiger (`docs/adr/0016-startseite-seitenleiste.md:58`). Endlosband über CSS `order`
(`HeroCarousel.tsx:199-207`, ADR 0016 Nachtrag 6.11.).

**Mobil/Desktop**: unter 768 px keine Pfeile, kleineres Padding (`HeroCarousel.module.css:237-251`),
je Reihe genau drei Cover (`docs/adr/0016-startseite-seitenleiste.md:59`); Bühnenbreite
`(width >= 768px) calc(100vw - 15rem), 100vw` (`HeroCarousel.tsx:139-142`); Wischen scrollt die
Posterreihe frei (`:206`).

**Caching/Preloading/Prepaint**:
- `homeQuery` (`['home']`, `staleTime 30 s`); der erste Abruf sendet `start` aus `data-start` der Hülle
  (`web/src/features/catalog/queries.ts:28-55`).
- Server-Hülle `GET /` malt das erste Bühnenbild vor (`#hero-prepaint`) und lädt `/home?start=K`
  vor, nur wenn Spotlight die **erste** Reihe ist (`server/internal/api/preload.go:24-27,100,194`;
  `server/internal/discover/discover.go:395-470`; `docs/adr/0011-spotlight.md:55`).
- Bilder: nur aktive Folie sofort, die nächsten zwei danach lazy (`HeroCarousel.tsx:210-215`).
- Service Worker: Bilder `/api/v1/items/{id}/images/…` CacheFirst (300 Einträge, 30 Tage), **alles
  übrige `/api` NetworkOnly** (`web/vite.config.ts:141-161`). Folge: Startseite und Spotlight sind
  offline nicht verfügbar [gelesen]; die Offline-Ansicht läuft über `/offline` (`web/src/app/paths.ts:21-22`).

---

## 3. Verwaltung «Spotlight» – vollständige Optionsliste [gelesen]

Seite `web/src/features/admin/SpotlightPage.tsx` (Route `/admin/spotlight`, `web/src/app/router.tsx:516`),
nur für Administrator mit Hauptprofil (`SpotlightPage.tsx:66-77`). Texte aus `web/src/locales/de.json`
(`admin.spotlight.*`). Drei zuklappbare Bänder plus Fussnote.

**Kopf**: Titel «Spotlight», Hilfe «Eine öffentliche Trakt-Liste als Reihe der Startseite. Gezeigt werden
nur Titel, die in der eigenen Bibliothek stehen; jedes Profil sieht nur, was seine Altersstufe erlaubt.»

**Band «Quelle»** (`SourceForm.tsx`, startet zugeklappt, `SpotlightPage.tsx:244-298`; Kopfzeile zeigt
Chip «Ungespeicherte Änderungen», Zusammenfassung «Liste · Medienart» und Chip «Auf der Startseite» /
«Fehlt auf der Startseite»):

| # | Beschriftung | Werte / Hinweis | Feld / API |
|---|---|---|---|
| 1 | «Auf der Startseite zeigen» (Schalter) | Hinweis «Gilt erst mit «Speichern», zusammen mit der Quelle.» | `enabled` / `setSpotlightConfig` |
| 2 | «Trakt-Liste» (Radiogruppe) | «Trending bei Trakt», «Beliebt bei Trakt», «Meistgesehen (letzte Woche)», «Meistgesammelt (letzter Monat)», «Am meisten erwartet», «Kinocharts», «Öffentliche Liste per Adresse»; Hinweis «Öffentliche Listen, kein Trakt-Login.» + Link «Schlüssel unter Integrationen» | `listKey` (Katalog aus `listKinds`) |
| 3 | «Adresse der Liste» (nur bei «per Adresse») | Hinweis «Eine öffentliche Liste auf trakt.tv, etwa https://trakt.tv/users/name/lists/name-der-liste. Merklisten (Watchlists) gehen nicht.» | `listUrl` (≤500) |
| 4 | «Medienart» (Radiogruppe) | «Nur Filme», «Nur Serien», «Filme und Serien»; gesperrt mit «Diese Liste gibt die Medienart vor.» (Kinocharts); bei Benutzerliste «Die Liste mischt Filme und Serien: Die Medienart filtert ihre Einträge nach dem Abruf.» | `media` |
| 5 | «Höchstens» | «Titel in der Reihe, je Profil (5–20)» | `itemCount` |
| 6 | «Ausblenden unter» | «Treffern in der Bibliothek, je Profil (0–10)» | `minHits` |
| 7 | «Abruf alle … Stunden» | «6 bis 48 Stunden» | `intervalHours` (auch Zeitplan `trakt.refresh`) |
| 8 | «Spotlight Wechsel Takt» | «3 bis 15 Sekunden» | `rotationSeconds` |
| 9 | Abrufzustand (Fuss) | «Zuletzt abgerufen {{date}}» / «Noch nie abgerufen» / «Letzter Abruf fehlgeschlagen: {{reason}}» · «{{hits}} von {{entries}} Titeln in der Bibliothek» · «Schlüssel gesetzt» / «Kein Schlüssel»; Grund-Hinweise «Ohne Trakt-Schlüssel fehlt die Reihe …», «Spotlight ist ausgeschaltet …», «Die Reihe fehlt …, bis ein Abruf gelingt.», «Die Liste ist leer …», «Nur {{hits}} Treffer …, weniger als die verlangten {{min}} …» | `getTraktStatus` + `getSpotlightEntries` (`FetchState.tsx:28-106`) |
| 10 | «Jetzt abrufen» (Knopf) | Meldung «Die Liste wird abgerufen.»; danach 30 s lang Abfrage alle 2 s | `refreshSpotlight` (`SpotlightPage.tsx:42-44,108-126`) |
| 11 | «Speichern» | Meldung «Spotlight ist gespeichert.» | `setSpotlightConfig` |

**Band «Einträge der Liste»** (Einleitung «Reihenfolge von Trakt; eigene Änderungen bleiben beim
nächsten Abruf erhalten.», Zähler «n Einträge», leer: «Noch keine Einträge. Rufe die Liste ab, sobald
Trakt eingerichtet ist.»). Tabelle (`spotlight/columns.tsx:55-165`):

| Spalte | Inhalt | Aktion / API |
|---|---|---|
| «Rang» | Trakt-Position | – |
| «Titel» | Poster (nur Treffer) + Trakt-Titel, «Film/Serie · Jahr», Fallback «TMDB {{id}}» | – |
| «Status» | «Sichtbar» / «Ausgeblendet» / «Nicht in der Bibliothek», darunter «Angeheftet auf Platz {{rank}}» | – |
| «Eigene Notiz» | «{{note}}» oder «–», Knopf «Notiz» (nur Treffer) → Dialog «Notiz zu {{title}}», Feld «Notiz», «Höchstens {{max}} Zeichen. Leer entfernt die Notiz.» | `setSpotlightEntryOverride {note}` |
| «Aktionen» | «Anheften» → Dialog «Platz in der Reihe» (Hinweis zu Platz 1, belegte Plätze «Belegt: …», «Alle Plätze sind belegt; löse zuerst einen.») / «Lösen»; «Ausblenden» / «Einblenden»; ohne Treffer nur Text «Nicht in der Bibliothek, erscheint nicht auf der Startseite» | `setSpotlightEntryOverride {pinnedRank \| hidden}`; 409 `rank_taken` |

**Band «Verwaiste Anpassungen»** (nur wenn vorhanden; «Anpassungen an Titeln, die nicht mehr in der
Liste stehen. Sie wirken erst wieder, wenn der Titel zurückkehrt.»): Spalten Titel, «Anpassung»
(Zusammenfassung), Knopf «Entfernen» → `setSpotlightEntryOverride` mit leerem Body
(`columns.tsx:168-218`).

**Fussnote**: «Wo die Reihe auf der Startseite steht und ob sie erscheint, stellst du unter
«Startseite» ein.» + Link «Zur Seite «Startseite»» (`SpotlightPage.tsx:341-344`).

Nebenverhalten: Abfragen ruhen bei offenem Dialog/ungespeicherter Eingabe; Rückfrage bei ungespeicherten
Änderungen beim Verlassen (`SpotlightPage.tsx:88-106,374`).

Weitere Türen zu denselben Werten: Seite «Jobs» (Zeitplan `trakt.refresh`, `de.json` Schlüssel
`trakt_refresh`, Zeile 4769), Seite «Integrationen» (Trakt-Schlüssel), Übersicht/Dashboard (Trakt-Status,
`server/internal/admin/dashboard.go:407-408`).

---

## 4. Trakt-Integration [gelesen]

- **Endpunkte** (nur GET, Header `trakt-api-key`, `trakt-api-version: 2`): `/movies|shows/trending`,
  `/popular`, `/watched/weekly`, `/collected/monthly`, `/anticipated`, `/movies/boxoffice`,
  `/users/{user}/lists/{slug}/items` (`server/internal/trakt/trakt.go:93-101,334,357-367`).
  Kein Login, kein Senden, kein Scrobbling (I-7; Schema-Test `TestContractNoTraktPerProfile`,
  `server/internal/api/contract_test.go:317`).
- **Gespeichert**: nur `spotlight_entries` (s. 1.5) und drei Settings (s. 1.2).
- **Client-ID**: Umgebung `SLOUCHCAT_TRAKT_CLIENT_ID` gewinnt (`server/internal/config/config.go:79-81,294`),
  sonst verschlüsselte Einstellung `trakt_client_id` von der Seite Integrationen
  (`server/internal/admin/trakt.go:9-12`, `server/internal/admin/settings.go:45-46,126,353-357`,
  `server/cmd/slouchcat/main.go:677-678`). Mock-Basis-URL nur mit `--dev` (`main.go:293-298`).
- **Reihen «Trending/Beliebt bei Trakt»** gibt es **nicht mehr** (seit M11, Vertrag 1.1.0): ersetzt
  durch die eine Spotlight-Reihe (`docs/adr/0011-spotlight.md:39-43`). Die alte Regel «Top 100 laden,
  erste 20 Treffer zeigen, unter 5 ausblenden» (AN-3, `docs/adr/0008-entdecken-suche-trakt.md:43`)
  lebt als Einstellungen weiter: 100 je Medienart geladen, `item_count` 5–20 (Vorgabe 20), `min_hits`
  0–10 (Vorgabe 5) (`server/migrations/00021_spotlight.sql:90-91`). `visibility.MaxRowItems = 20`
  begrenzt jede Reihe (`server/internal/visibility/visibility.go:31-32`).

---

## 5. Startseite [gelesen]

### 5.1 Reihenarten

`HomeRowKey`: `spotlight, picks, continue, recent, released, watchlist, collections, genres, libraries,
library, radio` (`api/openapi.yaml:12134-12142`; CHECK `server/migrations/00039_radio_stage2.sql:65-67`).
Laden je Art: `server/internal/discover/discover.go:330-360`.

| Schlüssel | Titel (`home.rows.*`) | Inhalt |
|---|---|---|
| `spotlight` | «Spotlight» | Trakt-Liste ∩ Bibliothek, einzige Bühne (`heroRow`) |
| `picks` | «User-Empfehlungen» (`home.picks`) | Empfehlungen der Profile des Besitzerkontos (ADR 0028) |
| `continue` | «Weiterschauen» | eine Karte je Serie, Filme/Hörbücher/Podcastfolgen angefangen |
| `recent` | «Neu hinzugefügt» | nach `added_at` |
| `released` | «Kürzlich veröffentlicht» | 12 Monate nach `release_date`, Serie nach neuster sichtbarer Folge |
| `watchlist` | «Merkliste» | Merkliste des Profils |
| `collections` | «Sammlungen» | ≥2 sichtbare Mitglieder |
| `genres` | «Genres» | Links in die Suche |
| `libraries` | «Meine Medien» | Kacheln Bibliotheken, Radio, Sammlungen, User-Empfehlungen |
| `library` (je Bibliothek) | «Neu in {{library}}» | neueste Titel der Bibliothek |
| `radio` | «Meine Sender» | Radio-Favoriten |

Höchstens **15 sichtbare** Reihen (`server/internal/model/model.go:997-999`,
`server/internal/discover/discover.go:72`); leere und ausgeblendete entfallen
(`discover.go:212-231,284-290`). Reihen laden parallel (≤4 Goroutinen, `discover.go:240-270`).

### 5.2 Verwaltung «Startseite» (`web/src/features/admin/HomeRowsPage.tsx`, `/admin/home-rows`)

Titel «Startseite», Hilfe «Reihenfolge der Reihen auf der Startseite, für alle Profile. Ausgeblendete und
leere Reihen erscheinen nicht.»; nur Admin mit Hauptprofil (`HomeRowsPage.tsx:70-82`).

| Element | Beschriftung | Bedeutung |
|---|---|---|
| Band «Reihen» mit Zähler «n Reihen» | Tabelle «Reihen der Startseite» | – |
| Spalte «Platz» | `RankSelect` je Reihe | Reihenfolge; Meldung «{{row}} steht jetzt auf Platz {{position}}. Speichere, damit es gilt.» |
| Spalte «Reihe» | Name, bei Bibliotheksreihe Chip «Bibliothek» | – |
| Spalte «Zustand» | «sichtbar» / «ausgeblendet» | – |
| Spalte «Ausblenden» | Kästchen «Ausblenden» | `hidden`; über 15: «Schon {{count}} Reihen sichtbar. Blende zuerst eine andere aus.» |
| Ausklappzeile je Reihe | Beschreibung `admin.homeRows.rows.<key>` + Link «Zur Seite «…»» (Spotlight, User-Empfehlungen, Sammlungen, Meine Medien) | `web/src/features/admin/homerows/rows.ts:16-25` |
| Band «Vorschau» mit «n sichtbar» | Schema mit Platzhaltern in ungespeicherter Reihenfolge | `homerows/HomeRowsPreview.tsx` |
| Speicherleiste | Speichern / Verwerfen («Änderungen verwerfen?» … «Verwerfen») | `setHomeRows` |
| Fussnote | «Welche Trakt-Liste Spotlight zeigt … steht unter «Spotlight».» + Link | `HomeRowsPage.tsx:322-325` |

Zeilenangaben: Tabelle `HomeRowsPage.tsx:181-237`, Ausklappzeile `:239-252`, Speicherleiste `:314-319`.

### 5.3 API

`getHome`, `listHomeRow`, `listPicks` (`/picks`), `listVisibleLibraries` (`/libraries/visible`,
`api/openapi.yaml:819`), Verwaltung `getHomeRows`/`setHomeRows` (`:6403-6473`, admin) und
`getMyMedia`/`setMyMedia` (`:6475-6546`, admin).

---

## 6. Detailseiten Film und Serie [gelesen]

### 6.1 Dateien und Aufbau

- `web/src/features/catalog/ItemPage.tsx` (Route `/items/$itemId`, `web/src/app/router.tsx:361-365`) für
  **alle** Arten, auch Serie und Staffel. `SeriesPage.tsx` ist die Hörbuch-Reihe
  (`/libraries/$libraryId/series/$seriesId`), nicht die TV-Serie.
- Kopf: `DetailHeader.tsx` (Cover/Still, Titel oder Logo, Metazeile, Stift für Admin), Knopfzeile
  `DetailActions.tsx`, Nachbarlinks `RelatedLinks` (Nächste Folge, Zur Serie, Haupttitel, Podcast,
  `ItemPage.tsx:132-170`).
- Körper (`ItemPage.tsx:610-652`): Sammlungsregale (`CollectionShelf`), Hörbuchreihe, bei Serie
  `SeasonShelf` (Staffelregal), bei Staffel `EpisodeList` («Folgen»), Hörbuchkapitel, Podcastfolgen,
  Versionen (Dateiinfo), «Extras», «Besetzung»; `TrailerPanel` als Dialog.

### 6.2 Knöpfe (`DetailActions.tsx:165-340`, Texte `de.json`)

| Knopf | Beschriftung | Bedingung |
|---|---|---|
| Haupt | «Abspielen» / «Fortsetzen» (+ «bei {{time}} · noch {{left}}») / «Nochmals ansehen»; Serie: «Abspielen»/«Weiterschauen»/«Nächste Folge»/«Nochmals ansehen» mit Folgencode | abspielbar |
| Trailer | «Trailer» | `trailerCount > 0` und online |
| Von vorne | «Von vorne» | angefangen |
| Merken | «Merken» / «Gemerkt» | nicht Staffel/Extra |
| Gesehen | Haken (drei Zustände bei Serie/Staffel) | `WATCHED_KINDS` |
| Empfehlen | «Empfehlen» / «Empfohlen ✓», Hinweis «Für alle Profile, die den Titel sehen dürfen, ohne deinen Namen» | `canRecommend` |
| Herunterladen | `DownloadAction` | Gerät kann offline |
| Verwalten | «Verwalten» → `/admin/items/{id}` | Admin-Hauptprofil, nicht editierbare Arten |
| Löschen | `admin.itemDelete.action` | Admin, löschbar |

### 6.3 Staffel- und Folgenliste

- `ItemDetail.seasons`: «Visible seasons of a series»; `episodes`: sichtbare Folgen einer Staffel
  (`api/openapi.yaml:11871-11882`); `ItemRef.childCount` = sichtbare Staffeln/Folgen (`:11774-11777`).
- Staffelregal mit «Alle {{total}} Staffeln anzeigen» (`item.seasonWall.showAll`), Folgenliste kompakt
  (`SeasonShelf.tsx:63-68`).

### 6.4 Kennt die DB die Gesamtzahl der Staffeln/Folgen? **Nein.**

- `items` hat keine Spalte für Gesamtzahl, Status (laufend/beendet) oder nächste Ausstrahlung
  (vollständige Spaltenliste: `server/migrations/00003_libraries_items.sql:46-76` plus alle
  `ALTER TABLE items ADD COLUMN`; grep nach `number_of_seasons`, `episode_count`, `in_production`,
  `next_episode` in `server/migrations/` ohne Treffer).
- TMDB liefert `seasons[].episode_count` und das wird geparst (`server/internal/metadata/tmdb.go:239-245`,
  `tmdb_provider.go:284-285`), aber nur für **Staffelposter** genutzt (`server/internal/metadata/match.go:493-496`);
  abgefragt werden nur die lokal **vorhandenen** Staffelnummern (`match.go:484-503`). TVDB-Staffeln ohne
  Folgenzahl (`tvdb.go:438-441`). Fehlende Folgen werden nie als Zeilen angelegt.
- Damit kann Slouchcat heute **weder «teilweise vorhanden» noch «künftige Folgen»** erkennen.
  Ähnlich bei Sammlungen: fehlende Teile wurden bewusst nicht gebaut (F-7, `docs/planning/17-detailseiten.md:191-193,414`).

---

## 7. Suche, Entdecken, Merkliste, Empfehlungen [gelesen]

- **Globale Suche** `search` (POST `/search`, scoped, `api/openapi.yaml:5505-5542`): FTS5
  `search_index` (`server/migrations/00015_discover.sql:150`; ADR 0008 Zeile 37) über Titel aller
  Sprachen, Originaltitel, Autor, Sprecher, Reihe, Personen, Sammlungen; immer `JOIN items JOIN
  visible_items` (`server/internal/visibility/search.go:176-186,209-229`). **Nur Bibliothek.**
  Filter: Genre, Jahr von/bis, Altersfreigabe höchstens, Medientyp, Verfügbarkeit «Auf dem Server»/
  «Auf diesem Gerät»; ohne Begriff Blättern nach Filtern.
- **Suchseite** `web/src/features/discover/SearchPage.tsx`: Feld «Suchbegriff» («Titel, Personen,
  Sammlungen»), Filterleiste, Abschnitte «Titel» (geblättert), «Personen», «Sammlungen» (je erste 10)
  (`SearchPage.tsx:154-340`).
- **«Entdecken»**: Es gibt **keine** Seite dieses Namens in der Zuschauer-App; der Feature-Ordner
  `features/discover` hält Suche, Merkliste, Gesehen, Sammlungen, Benachrichtigungen. «Entdecken»
  kommt in `de.json` nur als «Sender entdecken» (Radio) vor (Zeile 640).
- **Titel ausserhalb der Bibliothek** gibt es für Zuschauer **nirgends**. Für den Administrator: die
  Spotlight-Einträge ohne Treffer (s. 1.4) und die Anbietersuche `searchMetadata`
  (GET `/admin/metadata/search`, admin, für die manuelle Zuordnung, `api/openapi.yaml:4963-4985`).
- **Merkliste** `watchlist (profile_id, item_id → items ON DELETE CASCADE, created_at, follow_since)`
  (`server/migrations/00054_watchlist_follow.sql:23-29`); Serien/Podcasts auf der Merkliste lösen
  «neue Folge» aus (ADR 0008 Nachtrag 12.11.2026, `docs/adr/0008-entdecken-suche-trakt.md:63-70`).
- **Favoriten** für Titel entfallen seit 1.31.0 (nur Radio-Favoriten bleiben, dieselbe Stelle).
- **User-Empfehlungen** (ADR 0028): `user_picks (item_id, profile_id, created_at)`, FK auf `items`,
  anonym, Ablauf `picks_config.retention_days` (7–365, Vorgabe 60), nur Profile des Besitzerkontos
  empfehlen, alle sehen über `visible_items` (`docs/adr/0028-user-empfehlungen-von-profil-zu-profil.md:27-68`).
- **Benachrichtigungen** `notifications`, `kind IN ('new_episode')`, `item_id` FK auf `items`
  (`server/migrations/00015_discover.sql:329-341`; kein späteres Rebuild gefunden).

---

## 8. Externe IDs [gelesen]

| Spalte | Herkunft | Index | Beleg |
|---|---|---|---|
| `items.tmdb_id INTEGER` | Name/Ordner-Tag `{tmdb-…}`/`tmdbid`, NFO, TMDB-Match | `items_tmdb (tmdb_id)` (nicht eindeutig, ohne `kind`) | `00003_libraries_items.sql:59,81`; `library/names.go:264-279`; `library/write.go:124-134` |
| `items.imdb_id TEXT` | Tag, NFO, TMDB `external_ids`, TVDB `remoteIds` | `items_imdb` | `00003:60,82`; `metadata/tvdb.go:430-434` |
| `items.tvdb_id INTEGER` | Tag, NFO, TMDB `external_ids`, TVDB-Match | `items_tvdb` | `00008_library_complete.sql:23,32` |
| `items.anilist_id INTEGER` | Fill-Job | – | `00023_library_roots_providers.sql:64` |
| Trakt-ID | **nicht vorhanden** | – | grep `trakt_id` ohne Treffer |

- **Ab wann gesetzt**: beim Scan, wenn Datei-/Ordnername oder NFO die ID tragen
  (`server/queries/system/items.sql:4-17`); sonst nach dem Job `metadata.match`
  (`UpdateItemMatch`, `server/queries/system/metadata.sql:4-26`; Setzen nur, wenn der Anbieter einen
  Wert liefert und das Feld nicht gesperrt ist: `server/internal/metadata/match.go:1114-1122`). Der
  Fill schreibt keine IDs (`server/queries/system/metadata.sql:283-301`).
- **Lücke**: Eine Serie mit TVDB als erstem Anbieter bekommt von TVDB nur `tvdb_id` und `imdb_id`
  (TVDB-RemoteIDs werden nur für IMDb gelesen, `metadata/tvdb.go:430-434`), **keine `tmdb_id`** →
  sie trifft das Spotlight nie. [vermutet: wie viele Serien das auf CT 190 betrifft, nicht gezählt.]
- **«Finde Bibliothekstitel zu tmdb_id»**: keine allgemeine Abfrage. Vorhanden sind nur der Join in
  `ListSpotlightRow` (scoped) und `ListSpotlightEntriesAdmin` (system), die Dublettensuche
  (`server/queries/system/admin.sql:185-190`) und die Wiederverknüpfung (`relink.sql`, nur gelöschte
  Titel). `ItemDetail` trägt `tmdbId` und `tvdbId`, kein `imdbId` (`api/openapi.yaml:11854,11862`);
  die Admin-Titelansicht alle vier (`:14101-14112`).

---

## 9. Sichtbarkeit und Jugendschutz [gelesen]

- Spotlight-Reihe und alle Reihen lesen über `visible_items` des aktiven Profils
  (`server/queries/scoped/spotlight.sql:13`). Die View verlangt `deleted_at IS NULL`, **`available = 1`**,
  `COALESCE(effective_rating, 18) <= min(profil.max_rating, konto.max_rating)`, bei Serie/Staffel/Podcast
  zusätzlich `min_child_rating`, freigegebene Bibliothek von Profil und Konto, Bibliothek nicht im
  Löschen (`server/migrations/00063_minimum_age_accounts.sql:92-112`).
- Freigaben sind seit Vertrag 1.36.0 **Mindestalter 0–18** je Freigabeland (25 Länder; Lesereihenfolge
  gewähltes Land → DE → AT → CH → GB → US, strengster Wert über Anbieter; unbekannt = 18)
  (`docs/adr/0034-freigabelaender-und-altersstufen-je-land.md:28-38`). Rohdaten je Land in
  `item_certifications (item_id → items, provider, country, certification)`, nur Paket `metadata`
  (`server/migrations/00064_item_certifications.sql:26-33`).
- `min_hits` und `item_count` gelten je Profil: ein Kinderprofil kann die Reihe verlieren
  (`de.json` `admin.spotlight.missing.belowMinHits`, `FetchState.tsx:28`).
- **Für Titel ohne Bibliothekseintrag**: Es gibt keine Freigabe. `effective_rating`, `min_child_rating`
  und `item_certifications` hängen alle an `items` (FK). `spotlight_entries` hat keine Freigabespalte.
  Nach heutiger Logik würde ein Titel ohne Freigabe als 18 zählen (AN-1) und Profilen unter 18
  verborgen bleiben. Ein «nicht vorhandener» Titel kann `visible_items` gar nicht passieren.
- Regel 2: Titel-SQL nur in `visibility`, `admin`, `library`, `metadata`, `jobs`, `notify`, `offline`;
  Abfragen in `queries/scoped/` müssen `visible_items` enthalten; nicht sichtbar = 404 (CLAUDE.md).

---

## 10. Navigation der Zuschauer-App [gelesen]

Datei: `web/src/app/AppLayout.tsx`; Komponenten `web/src/ui/AppHeader.tsx`, `SideNav`, `SideNavMenu`.

**Kopfzeile** (`AppLayout.tsx:452-485`): Menüknopf «Menü» (nur wenn die Spalte nicht steht), App-Name →
Start, Suche («Suche», `/search`), im Verwaltungsbereich der Admin-Kopf, **Glocke** «Benachrichtigungen»
(`/notifications`, Zähler `unread` aus `notificationsQuery`, Kappung `BADGE_MAX`, `:137-141,475-484`),
**Profil** als Link «Profil wechseln, aktuell {{name}}» nach `/profiles` (`:472-474`; ein Profilmenü mit
Untereinträgen gibt es nicht [gelesen: nur `profileHref`]).

**Seitenleiste** ab 768 px als Spalte (15 rem), darunter Schublade; ausblendbar («Seitenleiste
ausblenden/einblenden», Zustand in `localStorage`) (`AppLayout.tsx:82-95,247-271,486,503-517`; ADR 0016).

| Gruppe | Einträge (Beschriftung → Route) | Bedingung | Beleg |
|---|---|---|---|
| Profilkarte | Name, Rolle, «Profil wechseln» → `/profiles` | immer | `:388-403` |
| Haupt | «Start» → `/`; «Suche» → `/search`; «Merkliste» → `/watchlist`; «Gesehen» → `/watched`; «Downloads» → `/downloads` | Downloads nur `offlineSupported()` | `:322-354` |
| «Meine Medien» | je Bibliothek → `/libraries/{id}` (mit Titelzahl); «Radio» → `/radio`; «Sammlungen» → `/collections`; «User-Empfehlungen» → `/picks`; Reihenfolge/Ausblenden aus `getMyMedia` | offline Hinweis + Link zu Downloads | `:273-316,404-409` |
| «Konto» | «Einstellungen» → `/settings/playback`; «Profile» → `/settings/profiles`; «Verwaltung» → `/admin` (Badge «Update») | Profile nur Hauptprofil, Verwaltung nur Admin | `:357-386` |
| Fuss | «Abmelden», «Seitenleiste ausblenden» | – | `:410-415` |

Weitere Zuschauerrouten (`web/src/app/router.tsx:355-370,640-760`): `/items/$itemId`,
`/people/$personId`, `/lists/$key`, `/collections/$collectionId`, `/libraries/$libraryId/series/$seriesId`,
`/notifications`, `/play/$itemId`, `/settings/notifications`, `/offline`, `/offline/play/$downloadId`.

---

## Folgen für das Wunsch-Konzept

### Einschränkungen (technisch und aus den Papieren)

1. **Papierlage**: Wünschen und Seerr-Bereich sind Non-Goal (Spec 46/66/69, Entscheid 10, S-4,
   CLAUDE.md Regel 11). Ohne neue Frage und ADR darf nichts gebaut werden. S-4 sagt ausdrücklich: Titel
   ohne Treffer nie auf der Startseite.
2. **Spotlight speichert für fehlende Titel nur `tmdb_id`, Trakt-Titel, Jahr**. Für Poster, Backdrop,
   Beschreibung, Freigabe muss etwas neu geholt werden (TMDB-Schlüssel ist vorhanden: `ClearTMDB`,
   `server/internal/model/admin.go:586`). Die Bildstrecke der App (`/items/{id}/images/…`, Service-Worker-
   Cache, Prepaint) setzt eine `items.id` voraus (`web/vite.config.ts:145-149`).
3. **Nur TMDB-Abgleich**: Trakt-IDs, IMDb, TVDB werden verworfen; TVDB-zuerst-Serien ohne `tmdb_id`
   erscheinen fälschlich als «fehlt». Ein Wunsch-Abgleich müsste mindestens `tvdb_id`/`imdb_id` aus
   Trakt (`ids`) mitnehmen, sonst würden vorhandene Serien angefordert.
4. **Cache ist flüchtig**: `spotlight_entries` wird bei jedem Lauf ersetzt und bei Fehler **geleert**;
   nur die aktuelle Liste (`list_key`) zählt; höchstens 100 je Medienart. Ein «Wunsch-Status» kann
   nicht an diesen Zeilen hängen; Overrides zeigen das Muster (eigene Tabelle mit `list_key, media,
   tmdb_id`).
5. **Jugendschutz**: Freigaben gibt es nur an `items`. Ein nicht vorhandener Titel hat keine
   Freigabe und kann `visible_items` nicht passieren. Ein Wunsch-Angebot an Profile braucht eine eigene
   Freigabequelle (TMDB-Certifications je Freigabeland, ADR 0034-Reihenfolge) und eine neue
   Sichtbarkeitsregel in `visibility` (Regel 2, Regel 12: Entscheid im Server). Unbekannt = 18.
6. **Vertrag**: `ItemRef` ist an Bibliothekstitel gebunden (`id` = `ItemID`, `images` aus dem
   Bildspeicher); `SpotlightEntry` ist admin-only und «never ItemRefs». Ein fehlender Titel auf der
   Startseite braucht ein neues Schema (z. B. eigene Karte) und `x-visibility`
   (`scoped` oder `account`), Eintrag im Vertragstest; `TestContractNoTraktPerProfile` beachten.
7. **Bühne**: `heroRow`, `heroStart`, `FirstHero`, Prepaint und `HeroSlide.playHref/detailsHref`
   setzen abspielbare Bibliothekstitel voraus (`HeroCarousel.tsx:19-51`). Ein fehlender Titel hat weder
   «Abspielen» noch eine Detailseite `/items/{id}`; Merken, Gesehen, Empfehlen, Benachrichtigung sind
   alle FK auf `items` (Merkliste, `user_picks`, `notifications`).
8. **Serien «teilweise vorhanden»** ist heute nicht erkennbar (keine Gesamtzahlen). Dafür bräuchte es
   neue Daten je Serie (TMDB `number_of_seasons`/`seasons[].episode_count`, Status, nächste Folge).
9. **Netzwerk**: `netguard` sperrt Ziele im LAN für Feed-Abrufe; ein Seerr-Client im Heimnetz darf nicht
   darüber laufen (`server/internal/netguard/netguard.go:1-10`) [vermutet: braucht eigenen HTTP-Client
   wie `trakt`].
10. **Geheimnisse**: Seerr-API-Schlüssel gehört wie die Trakt-ID in die verschlüsselten Einstellungen
    und in die Redaction-Liste (Regel 10; Muster `admin/settings.go:45-46,126`).
11. **Offline**: `/api` JSON ist NetworkOnly; Wunschfunktion offline nicht verfügbar (und muss es nicht).

### Anknüpfungspunkte

- `spotlight_entries` enthält schon heute genau die Kandidaten «in Trakt, nicht in der Bibliothek»
  (`ListSpotlightEntriesAdmin` mit `LEFT JOIN items`, `itemId` leer). Eine scoped Abfrage «Einträge ohne
  sichtbaren Treffer» liesse sich neben `ListSpotlightRow` in `queries/scoped/spotlight.sql` legen.
- Override-Muster (`spotlight_overrides`): Schlüssel `(list_key, media, tmdb_id)`, kein FK auf `items`,
  überlebt Abrufe; Vorlage für eine Tabelle «Wünsche/Anforderungen».
- Paket `trakt` ist reiner HTTP-Client ohne DB mit Rate-Limiter und Redirect-Schutz: Vorlage für ein
  Paket `seerr` (Regel 2 bleibt unberührt, wenn die DB-Seite in `metadata`/`admin` bleibt).
- Job-Infrastruktur mit Zeitplan, Prioritäten, `RaiseQueuedPriority`, Verwaltung «Jobs».
- Admin-Seite Spotlight hat die Spalte «Status» mit «Nicht in der Bibliothek» und keine Aktion: dort
  könnte «Anfordern»/Wunschstatus für den Administrator zuerst entstehen (kleinster Schritt).
- `searchMetadata` (admin) zeigt, dass TMDB-Suche serverseitig schon existiert; der TMDB-Client
  (`metadata/tmdb.go`) holt Details inkl. `external_ids`, `content_ratings` und `seasons[].episode_count`.
- Konten-/Profilrollen für eine Wunschberechtigung: `accounts.is_owner`, `role`, `profiles.isPrimary`,
  `canRecommend`-Muster aus ADR 0028 (Server liefert `can…`-Flag, Client zeigt nur).
- Webhooks von Radarr/Sonarr (I-8) melden Imports bereits: ein erfüllter Wunsch erscheint nach dem
  gezielten Scan als Bibliothekstitel und trifft das Spotlight automatisch.

---

## Unsicher / nicht geprüft

- Sprache der Trakt-Titel in `spotlight_entries.title` (vermutet Englisch).
- Anzahl Serien ohne `tmdb_id` (TVDB-zuerst) auf CT 190; nicht gezählt (keine DB-Abfrage gemacht).
- Ob eine Content-Security-Policy externe Bilder (image.tmdb.org) in der App verbieten würde: im Server
  nur `Referrer-Policy` gefunden (`server/internal/api/stream_token.go:54-60`); `index.html`/Proxy nicht
  geprüft.
- `preload.go` nur an den genannten Stellen gelesen, nicht vollständig.
- `HomeListPage.tsx`, `SeasonShelf.tsx`, `DetailHeader.tsx` nur auszugsweise gelesen.
- Inhalt von `docs/planning/05-plan-spotlight.md` (1675 Zeilen), `08-startseite.md`, `15-feinschliff.md`,
  `17-detailseiten.md`, `21-sammlung.md` nur gezielt per Suche gelesen (Wunsch, fehlende Teile,
  Spotlight-Regeln), nicht vollständig.
- Seite «Jobs», «Integrationen» und Dashboard nur über die genannten Codezeilen, nicht ihre
  Oberflächentexte im Einzelnen.
- Keine Tests, Builds oder Laufzeitbeobachtungen; alle Aussagen zum Verhalten stammen aus Code und
  Kommentaren.
