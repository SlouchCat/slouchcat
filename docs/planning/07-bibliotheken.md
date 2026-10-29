# Konzept: Bibliotheken (Ordner mit eigener Inhaltsart und eigener Scraper-Liste)

- **Status: Entwurf, drei Fragerunden geführt, Kritiker-Review eingearbeitet (28. Oktober 2026).**
  Marcos zehn Antworten stehen in Abschnitt 11 (F-1 bis F-8, F-11, F-12); eine Frage ist noch offen
  (F-10). Der OMC-Kritiker hat die erste Fassung geprüft (22 Befunde, Abschnitt 13). **Fassung 3**
  (dritte Fragerunde): die Metadatenquelle je Ordner ist keine Einzelwahl mehr, sondern eine
  **geordnete Scraper-Liste wie in Emby** (F-11), und **TVDB ist als Hauptscraper für Serien
  wählbar** (F-12). Beides kehrt zwei verworfene Alternativen aus ADR 0006 um; ADR 0015 wird darum
  dessen Teilnachfolger. Die Entscheide gehen als **Frage 13** ins Entscheidungsprotokoll
  (`docs/planning/04-entscheidungen.md`), sobald Marco das Papier freigibt. Umgesetzt wird nichts,
  bevor das geschehen ist. **Fassung 4** (29. Oktober 2026): ein **klickbarer Prototyp** der
  Verwaltungsseiten liegt unter `docs/planning/mockups/07-bibliotheken-prototyp.html` (7.1), und aus
  Marcos Rückmeldung dazu folgt **F-13**: eine Bibliothek bekommt **mehrere Ordner in einem Zug**
  (`LibraryRequest.roots` als Liste, `addLibraryRoot` nimmt eine Liste, alles oder nichts).
- **Datum: 28. Oktober 2026, Fassung 4 vom 29. Oktober 2026**
- **Wogegen dieses Konzept verankert ist:** `main`, Stand `0dd4f19` («Merge pull request #43»), im
  Worktree `claude/konzept-bibliotheken`. Jede Zeilenangabe bezieht sich auf diesen Stand. **Parallel
  läuft die Benutzerverwaltung** auf `claude/benutzerverwaltung` (Stand `5ece34d` oder neuer, Konzept
  `06-benutzerverwaltung.md`): sie bringt Migration **00022**, ADR **0014** und Vertragsfassung
  **1.2.0**. Dieses Papier rechnet damit, dass sie **zuerst** in `main` geht (F-10), und nennt darum
  als nächste freie Nummern **Migration 00023, ADR 0015, Vertrag 1.3.0**. Geht sie später, rücken die
  drei Nummern um eins zurück; inhaltlich ändert sich nichts, denn die Benutzerverwaltung fasst weder
  `libraries` noch `library_roots` noch den Scanner an (`git diff --stat main 5ece34d --
  server/internal/library api/openapi.yaml` nennt den Vertrag, dort nur Konten und Rollen, und
  `roots_test.go` mit einer Testanpassung).
- **Letzte Migration auf `main` ist `00021_spotlight.sql`**, auf dem Zweig `00022_account_roles.sql`.
  **Letzte ADR auf `main` ist `0013-ci-parallele-jobs.md`**, auf dem Zweig `0014-benutzerverwaltung.md`.
  **Vertragsfassung auf `main` ist `1.1.0`** (`api/openapi.yaml:4`), auf dem Zweig `1.2.0`.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
- **Jeder Schritt ist ein eigener Commit, und vor dem Abschluss jedes Schritts ist `task ci` grün**
  (`CLAUDE.md`, Abschnitt «Arbeitsweise»). DoD-Belege sind Befehl **und** Ausgabe.

> **Dieses Papier selbst ändert keine Zeile Code.** Die eigentliche Arbeit beginnt mit Schritt 0 aus
> Abschnitt 10, und auch der ist ein reiner Doku-Commit.

> **Abgrenzung zum Scraper-Konzept (Idee 01), neu gezogen in Fassung 3.** Hierher gehören jetzt:
> die geordnete Scraper-Liste **je Ordner**, das **feldweise Auffüllen** in dieser Reihenfolge, die
> **Herkunft je Feld** (`item_field_sources`) und TVDB als wählbarer Hauptscraper. Bei Idee 01
> bleiben: Sprachen je Ordner, Bildarten (Logo, Thumb, Banner, …), Bildsprache, die Anbieter
> Fanart.tv, OMDb und lokale NFO, die Bildwahl je Titel, das Land für die Altersfreigabe und die
> Rohzertifikate. Idee 01 wird in Schritt 0 um genau diese Grenze ergänzt.

---

## 1. Ausgangslage und Problem

### 1.1 Was das heutige Modell kann

Eine Bibliothek ist heute eine Zeile in `libraries` mit **einer** Art für alle ihre Ordner. Die
Tabelle (zuletzt in `server/migrations/00014_audio.sql:32` bis `:47` neu gebaut) trägt
`kind TEXT NOT NULL CHECK (kind IN ('movies', 'series', 'audiobooks', 'podcasts'))`, den Namen, die
Standard-Altersstufe `default_rating` für Audio (AN-2) und den Scan-Zustand. Die Ordner stehen in
`library_roots` (`00003_libraries_items.sql:35` bis `:44`): `library_id`, `path` (UNIQUE), `fs_type`,
`watch`, `created_at` — **keine** Inhaltsart, **keine** Quelle.

Mehrere Ordner je Bibliothek gibt es seit PR #35 (Idee 02, Schritte 12 bis 14; Stand
`docs/dev/CURRENT_STATUS.md:250`): `addLibraryRoot` (`api/openapi.yaml:2368`) und `removeLibraryRoot`
(`:2414`) mit abgetippter Bestätigung, Löschweg unter der Scan-Sperre, Bibliothek ohne Ordner
erlaubt. Der Dialog «Ordner von X» (`web/src/features/admin/LibraryFoldersDialog.tsx`) zeigt je Zeile
den Pfad und sonst nichts, denn mehr weiss eine Wurzel nicht.

Was ein Ordner **ist**, entscheidet allein `libraries.kind`, an genau drei Stellen:

```go
// server/internal/library/scanner.go:169
func parsePath(kind, rel string) ParsedName {
	switch kind {
	case "series":
		return ParseSeriesPath(rel)
	case model.LibraryAudiobooks:
		return ParsedName{}
	}
	return ParseMoviePath(rel)
}
// server/internal/library/write.go:364
func (w *batchWriter) item(ctx context.Context, info fileInfo) (string, error) {
	switch {
	case w.lib.Kind == model.LibraryAudiobooks:
		return w.audiobookItem(ctx, info)
	case info.extra:
		return w.extraItem(ctx, info)
	case w.lib.Kind == "series":
		return w.episodeItem(ctx, info)
	default:
		return w.titleItem(ctx, info, "movie")
	}
}
```

Dazu `scannedExt` (`scanner.go:181`, Audio- oder Videoendungen), `wanted` (`:606` bis `:619`, in
Serienbibliotheken fliegt jede Datei ohne Folgenmarke heraus), die Batchgrösse (`:511`), der
Folgen-Hook `QueuePendingSegments` nur für `series` (`:301`), `describe`, `describeBook` und
`touchBook` (`write.go:52`, `:221`, `:299`, `:312`), und `walkRoot` bekommt `lib.Kind` für **alle**
Wurzeln gleich (`:446`). Im Paket `admin` prüft `validateLibrary` (`server/internal/admin/admin.go:240`)
Pfadpflicht und Altersstufe je Art, `ScanLibrary` (`:352`) leitet Podcast-Bibliotheken auf
`RefreshLibrary` um (`:357`), und `AddRoot` verweigert ihnen jeden Ordner
(`server/internal/library/roots.go:34`).

**Die Metadaten wissen von der Bibliothek nichts, und die Reihenfolge der Anbieter ist fest im
Code.** `match` (`server/internal/metadata/match.go:78`) verzweigt nach der **Titelart**: Filme nur
TMDB (`matchMovie`, `:122`); Serien TMDB, und TVDB nur, wenn TMDB nichts findet oder die manuelle
Zuordnung `tvdb_id` und ein leeres `tmdb_id` sperrt (`matchSeries`, `:187`, `manualTVDB` `:198`);
`seriesProvider` (`:315`) wählt danach nach den gespeicherten Ids, TMDB zuerst. Es gibt genau zwei
Anbieter hinter einer Schnittstelle (`provider.go:12`; `ProviderTMDB`, `ProviderTVDB` `:25` bis `:26`;
`tvdbProvider` in `tvdb.go:67` ff. mit `SearchSeries` `:271`, `Series` `:357`, ohne Filme `:248`,
`:253`), beide mit serverweiten Schlüsseln (`admin.go:38`, `tvdb_api_key`, `tvdb_pin`). Texte werden
je Anbieter **ganz** geschrieben: `UpdateItemMatch` (`server/queries/system/metadata.sql:4` bis `:20`)
überschreibt `original_title`, `genres`, `own_rating`, `rating_source`; `UpsertItemText` (`:115`)
überschreibt Titel, Beschreibung und Tagline je Sprache. Eine Herkunft je Feld gibt es nicht; nur
`rating_source` sagt, woher die Freigabe kam. ADR 0006 hat «TVDB als Hauptquelle für Serien» und
«beide Anbieter abfragen und zusammenführen» ausdrücklich verworfen
(`docs/adr/0006-metadaten-anbieter-trailer.md:37` bis `:41`).

Hörbücher haben keinen Anbieter: Autor, Sprecher, Reihe und Cover kommen aus Audiobookshelfs
`metadata.json`, den Tags der ersten Datei und den Ordnernamen (`server/internal/library/audiobook.go:24`,
ADR 0007) — **die Tags allerdings erst nach der Sonde**, nicht beim Scan (`audiobook.go:125` bis
`:145`, aufgerufen aus `probe.go:173`). Podcasts sind Feed-Container in einer Bibliothek **ohne
Wurzel** (ADR 0007, `docs/adr/0007-audio-modell.md:41`); lokale Podcast-Ordner stehen dort ausdrücklich
als offen (`:58`).

### 1.2 Wo es für Marcos Sammlung nicht reicht

Marco hat am 28. Oktober 2026 diese Zielstruktur vorgegeben:

| Bibliothek | Typ | Ordner | Scraper |
|---|---|---|---|
| Filme | Filme | /Filme | Filme |
| Serien | Serien | /Serien | Serien |
| Dokus | Filme | /Doku Filme | Filme |
| Dokus | Serien | /Doku Serien | Serien |
| Kids | Serien | /Kids Serien | Serien |
| Kids | Filme | /Kids Filme | Filme |
| Kids | Filme | /Kids Filme 2 | Filme |
| Anime | Filme | /Anime Filme | Anime |
| Anime | Serien | /Anime Serien | Anime |
| Hörbücher | Hörbücher | /Hörbuch | Hörbuch Scraper |
| Podcast | Podcast | /Podcast | Podcast Scraper |

Und in der dritten Fragerunde präzisiert: **die Spalte «Scraper» ist keine Vorgabe aus vier Werten,
sondern je Ordner eine Liste mit Häkchen und Reihenfolge, wie Emby sie je Bibliothek zeigt.**
«Anime» ist dann kein eigener Scraper, sondern zum Beispiel die Liste «TVDB, TMDB, AniList» an einem
Serienordner.

Vier Dinge daran kann das heutige Modell nicht:

1. **Eine Bibliothek mit Film- und Serienordnern.** «Dokus», «Kids» und «Anime» bestehen je aus
   einem Filmordner und einem Serienordner. Heute müsste Marco sechs Bibliotheken anlegen («Dokus
   Filme», «Dokus Serien», …), und jedes Profil bekäme in den Bibliotheksrechten sechs Häkchen statt
   drei (`profile_libraries`, `account_libraries`). Die Art muss vom Ordner kommen, nicht von der
   Bibliothek.
2. **Eine Scraper-Liste je Ordner.** Heute ist die Reihenfolge TMDB → TVDB fest im Code, TVDB nur
   als Rückfall. Marcos Anime-Ordner sind nach Sonarr benannt, also mit **TVDB-Staffelzählung**
   (`[tvdb-…]`-Marken, `names.go:112` liest sie); bei Anime weichen TMDB und TVDB in der Zählung oft
   voneinander ab, und dann passen die TMDB-Texte nicht zu den Dateien. Dafür braucht es TVDB zuerst
   — genau die Alternative, die ADR 0006 für den Haushalt verworfen hat, als es noch keine
   Anime-Bibliothek gab.
3. **Einen Anime-Anbieter.** Es gibt keinen. Auf dem NAS liegen 29 Anime-Serien und 82 Anime-Filme
   (`/mnt/nerdnas/medien/MangaSerien`, `MangaFilme`).
4. **Ein Podcast-Ordner.** `/Podcast` ist auf dem NAS `Audiobookshelf/Podcast` mit 102 Podcasts,
   25 896 MP3- und 135 M4A-Dateien, je Podcast eine `metadata.json` (95 Stück) mit Titel, Autor,
   Beschreibung, **`feedURL`**, `itunesId`, `language`, `explicit` und ein `cover.jpg`. Die Dateien
   tragen ID3-Tags, die Audiobookshelf beim Herunterladen schreibt (`title`, `date` in RFC-2822-Form,
   `comment`/`subtitle` mit der Beschreibung, `album` = Podcastname, `itunes-id`; **keine GUID**).
   Heute nimmt eine Podcast-Bibliothek keinen Ordner an.

Dazu ein kleinerer Punkt: **die Hörbuchbibliothek** liegt unter `Audiobookshelf/Hörbücher` (62
Autorenordner, M4B mit `metadata.json`, `cover.jpg`, teils `.opf`). Das kann der heutige Erkenner
bereits; sie braucht nur einen Ordner mit der Inhaltsart «Hörbuch».

### 1.3 Marcos Entscheide vom 28. und 29. Oktober 2026

Drei Fragerunden, zehn Antworten, dazu ein Entscheid am Prototyp (Abschnitt 11), in Kurzform:

| Nr. | Entscheid |
|---|---|
| F-1 | **Lokale Podcast-Ordner werden gescannt.** Das offene Ende aus ADR 0007 wird gebaut |
| F-2 | **Anime bekommt jetzt einen eigenen Anbieter**, nicht nur ein Etikett |
| F-3 | **Nur Video ist mischbar.** Filme und Serien dürfen in eine Bibliothek; Hörbücher und Podcasts bleiben eigene Bibliotheken |
| F-4 | **Inhaltsart und Scraper-Liste eines Ordners sind nachträglich änderbar**, mit Neuerkennung der Titel |
| F-5 | **Der Anime-Anbieter ist AniList.** Staffeln und Folgen bleiben bei TMDB/TVDB; AniList liefert Titel, Beschreibungen, Bilder, Genres dazu |
| F-6 | **Podcast-Ordner und Feeds zusammen:** steht eine `feedURL` in der `metadata.json`, wird der Feed mitabonniert; Abos ohne Ordner bleiben möglich |
| F-7 | **Der «Hörbuch Scraper» ist der heutige lokale Erkenner.** Ein externer Hörbuch-Anbieter kommt im Scraper-Konzept |
| F-8 | **Die Wahl am Ordner heisst «Metadatenquelle»**, englisch «metadata source»; in Fassung 3 der Plural «Metadatenquellen» für die Liste |
| F-11 | **Liste mit Reihenfolge wie in Emby:** je Ordner mehrere Häkchen, sortierbar; der erste erkennt den Titel, die weiteren füllen feldweise auf |
| F-12 | **TVDB ist als Hauptscraper für Serienordner wählbar.** Bei TVDB zuerst gilt die TVDB-Staffelzählung; TMDB füllt nur auf |
| F-13 | **Mehrere Ordner in einem Zug.** Beim Anlegen einer Bibliothek und im Dialog «Ordner von X» ist der Ordner eine Liste nummerierter Karten («+ Weiteren Ordner hinzufügen»); ein Absenden legt alle an, der Server alle oder keinen |

---

## 2. Zielbild

```
Bibliothek  (Name, Art: Video | Hörbücher | Podcasts, Standard-Altersstufe bei Audio)
  └─ Ordner  (Pfad, Inhaltsart, Metadatenquellen = geordnete Liste, Dateisystem, Überwachung)
  └─ Ordner
  └─ …
  └─ [nur Podcasts] Feed-Abos ohne Ordner, wie heute
```

Die **Bibliothek** behält ihre Rolle als Einheit für Rechte (`profile_libraries`, `account_libraries`,
`invitation_libraries`), Scan-Zustand, Verlustschutz und Standard-Altersstufe. Neu ist ihre Art nur
noch eine **Familie**: `video`, `audiobooks` oder `podcasts`. Sie sagt, welche Inhaltsarten ihre Ordner
tragen dürfen, sonst nichts.

Der **Ordner** (`library_roots`) trägt neu zwei Werte:

| Wert | Spalte / Vertrag | Erlaubte Werte | Bedeutung |
|---|---|---|---|
| **Inhaltsart** | `content` / `content` | `movie`, `series`, `audiobook`, `podcast` | Welcher Parser und welcher Erkenner den Ordner lesen (heute `libraries.kind`) |
| **Metadatenquellen** | `providers` (JSON-Liste) / `providers: MetadataProvider[]` | geordnete Teilmenge aus `tmdb`, `tvdb`, `anilist`, `local` | **Der erste Eintrag erkennt** den Titel und bestimmt Ids und Staffelzählung; **jeder weitere füllt auf**, was davor leer blieb (4.4) |

Zwei Regeln an der Liste, die der Server durchsetzt (422 `content_not_allowed`,
`providers_not_allowed`):

| Inhaltsart | Erlaubt als **erster** Eintrag (Erkenner) | Erlaubt als weitere Einträge (Auffüller) | Beispiele aus Marcos Tabelle |
|---|---|---|---|
| `movie` | `tmdb` | `anilist` | «Filme»: `[tmdb]` · «Anime Filme»: `[tmdb, anilist]` |
| `series` | `tmdb` **oder** `tvdb` | `tmdb`, `tvdb`, `anilist` | «Serien»: `[tmdb, tvdb]` · «Anime Serien»: `[tvdb, tmdb, anilist]` |
| `audiobook` | `local` | — | «Hörbuch Scraper»: `[local]` |
| `podcast` | `local` | — | «Podcast Scraper»: `[local]` |

Dazu die Familienregel aus F-3: in einer `video`-Bibliothek nur `movie` und `series`, in
`audiobooks` nur `audiobook`, in `podcasts` nur `podcast`. AniList ist **nie** Erkenner (F-5): es
liefert keine Staffelstruktur und keine Ids, die der Rest des Servers (Credits, Trailer, Bilder)
braucht. `tvdb` an einem Filmordner ist ein 422, weil `tvdbProvider.Movie` nichts liefert
(`tvdb.go:253`). Die Liste darf nicht leer sein. Hörbücher und Podcasts haben heute genau einen
Eintrag; die Liste steht trotzdem da, damit Idee 01 später nur Werte ergänzt (Audnexus, Fanart.tv,
OMDb, NFO).

**Benennung** (F-8, F-11): In der Oberfläche heissen die Felder **«Inhalt»** (das Wort steht schon
heute über der Artauswahl, `web/src/locales/de.json:194`) und **«Metadatenquellen»**, englisch
«Content» und «Metadata sources». Die Einträge heissen «TMDB», «TVDB», «AniList», «Lokale Dateien».
Die Bibliotheksarten heissen «Video», «Hörbücher», «Podcasts». Die Vertragswerte bleiben englisch und
klein.

**Marcos Tabelle, abgebildet auf das Zielbild** (Pfade so, wie der Stack auf CT 190 das NAS sieht,
`docs/dev/TESTSERVER.md`, Bind-Mount `/media/nas`; die Listen sind Vorschläge, Marco setzt sie
selbst):

| Bibliothek (Art) | Ordner | Inhalt | Metadatenquellen |
|---|---|---|---|
| Filme (video) | `/media/nas/Filme` | movie | TMDB |
| Serien (video) | `/media/nas/Serien` | series | TMDB, TVDB |
| Dokus (video) | `/media/nas/Doku/Filme` | movie | TMDB |
| Dokus (video) | `/media/nas/Doku/Serien` | series | TMDB, TVDB |
| Kids (video) | `/media/nas/KinderFilme` | movie | TMDB |
| Kids (video) | `/media/nas/KinderSerien` | series | TMDB, TVDB |
| Kids (video) | «Kids Filme 2» — auf dem NAS heute ohne Gegenstück | movie | TMDB |
| Anime (video) | `/media/nas/MangaFilme` | movie | TMDB, AniList |
| Anime (video) | `/media/nas/MangaSerien` | series | **TVDB**, TMDB, AniList |
| Hörbücher (audiobooks) | `/media/nas/Audiobookshelf/Hörbücher` | audiobook | Lokale Dateien |
| Podcast (podcasts) | `/media/nas/Audiobookshelf/Podcast` | podcast | Lokale Dateien |

Nichts davon ist vorgegeben: Marco legt eine Bibliothek an, fügt Ordner hinzu und setzt je Ordner
Inhalt und Liste. Zwei Ordner derselben Inhaltsart in einer Bibliothek («Kids Filme» und «Kids
Filme 2») kann das heutige Modell schon; neu ist nur, dass es dafür keine zweite Bibliothek braucht.

### 2.1 Was sich für den Zuschauer ändert

**Nichts, und das ist Absicht.** Kein `scoped`-Endpunkt liest `libraries.kind` oder filtert nach
Bibliothek (Prüfung über alle 39 `x-visibility: scoped`-Operationen und `server/queries/scoped/`);
die Sicht `visible_items` prüft Rechte nur über `library_id` und Alter nur über die **Titelart**. Die
Suche filtert nach `mediaTypes` auf Titelebene (`SearchRequest` in `api/openapi.yaml`), die
Startseite nach Reihen. Der einzige Zuschauer-nahe Ort, der eine Bibliotheksart sieht, ist
`listAccountLibraries` (`:857`, Stufe `account`, für die Bibliothekswahl in den Profilen) — und
`web/src/features/shared/useLibraryOptions.ts:35` bis `:38` liest davon nur `id` und `name`.

Eine Bibliotheksnavigation für Zuschauer (Kacheln «Filme», «Dokus», «Kids» wie in Emby) ist **nicht**
Teil dieses Konzepts. Sie wäre ein eigenes Vorhaben mit eigenen Reihen, eigener Startseiten-Regel und
eigenem Tastaturtest; dieses Papier schafft dafür nur die Voraussetzung, dass «Dokus» eine Bibliothek
ist und nicht zwei.

---

## 3. Konflikt mit Spec, Plan und ADRs

| Quelle | Wortlaut heute | Was dieses Konzept daraus macht |
|---|---|---|
| **B-1** (`docs/spec/01-anforderungen-v1.md:85`): «Der Admin legt Bibliotheken auf Ordner an (Typ Filme oder Serien)» | Typ an der Bibliothek | Der Typ wandert an den Ordner; eine Bibliothek darf beide tragen. **Die Spec wird nicht umgeschrieben** (`docs/spec/README.md:6`, derselbe Weg wie F-6 der Benutzerverwaltung): der neue Wortlaut geht als Frage 13 ins Entscheidungsprotokoll und in ADR 0015 |
| **B-3** (`:87`): «Metadaten und Bilder kommen von TMDB und TVDB» | zwei Quellen, Reihenfolge offen | bleibt erfüllt; AniList kommt als dritte dazu, die Reihenfolge wird je Ordner wählbar |
| **H-1, H-2** (`:96`, `:97`) | Hörbuch-Bibliotheken auf Ordnern; Podcasts per Feed-URL | Beides bleibt erfüllt. H-2 bekommt einen zweiten Weg (Feed aus der `metadata.json` eines Ordners), kein Ersatz |
| **ADR 0006** (`docs/adr/0006-metadaten-anbieter-trailer.md`): TMDB Hauptquelle, TVDB nur Rückfall (`:24` bis `:33`); **verworfen** «TVDB als Hauptquelle für Serien» (`:37` bis `:39`) und «beide Anbieter abfragen und zusammenführen» (`:40` bis `:41`) | feste Reihenfolge im Code | **Beide verworfenen Alternativen werden mit F-11 und F-12 angenommen** — mit den Gründen, die 0006 damals selbst nannte (Folgenzählung bei Anime) und die mit einer Anime-Bibliothek jetzt zutreffen. Die Einwände von 0006 («doppelte Anfragen», «Widersprüche bei Freigaben und Folgennummern») werden in 4.4 einzeln beantwortet: Anfragen nur für angekreuzte Anbieter; Folgennummern kommen **allein vom Erkenner**; Freigaben nie gesenkt (K-3). **ADR 0015 ist der Teilnachfolger von ADR 0006**, den Idee 01 angekündigt hat; die Trailer-Kette und die Schlüsselregel aus 0006 bleiben |
| **ADR 0007** (`docs/adr/0007-audio-modell.md:41`): Podcast-Bibliothek «ohne Wurzel»; `:58` nennt lokale Podcast-Ordner als offen | Nicht-Ziel mit Vorbehalt | Wird gebaut (F-1, F-6). ADR 0015 schreibt die Zeile fort; ADR 0007 bekommt einen Ablösungshinweis an genau dieser Stelle. Die Trigger für `podcast_episode` mit Datei sind seit 00014 vorbereitet (`00014_audio.sql:15` bis `:17`; ADR 0007, Abschnitt «Folgen») |
| **Idee 01** (`docs/ideas/01-metadaten-einstellungen.md`): Anbieterlisten je Bibliothek (`:55`, `:105` bis `:110`), feldweise auffüllen mit Herkunft je Feld (`item_field_sources`, `:116` bis `:119`) | «Ausgearbeitet», nicht beauftragt | **Dieser Teil zieht hierher**, auf Ordnerebene statt Bibliotheksebene: Anbieterliste, feldweises Auffüllen, `item_field_sources`. Was bei Idee 01 bleibt, steht im Kopf dieses Papiers; Idee 01 und Issue #8 bekommen in Schritt 0 den Vermerk «Teil A übernommen durch Konzept 07» |
| **Idee 02** (`docs/ideas/02-ordner-browser.md:3` bis `:6`, Kopf sagt «Schritte 12 bis 14 offen») | veraltet | Nur der Kopf wird in Schritt 0 nachgeführt; E8 bis E18 gelten unverändert |
| **Entscheid 7 / AN-15** (`04-entscheidungen.md:16`, `:36`): «TMDB Hauptquelle, TVDB optional hinter einer Anbieter-Schnittstelle» | TMDB zuerst | wird durch Frage 13 präzisiert: TMDB bleibt Vorgabe und Pflicht als Erkenner für Filme; für Serien ist TVDB als Erkenner wählbar |
| **Entscheid 9 / AN-2** (`:18`, `:43`): Standard-Altersstufe je Hörbuch- und Podcast-Bibliothek | bleibt | unverändert; die Stufe hängt weiter an der Bibliothek, nie am Ordner. Ordner-Podcasts bekommen sie wie Bücher (`rating_source 'library'`) |
| **Harte Regel 11** (nichts vorziehen, keine Nicht-Ziele) | — | Anime, Anbieterliste und lokale Podcast-Ordner stehen in keiner Nicht-Ziel-Liste der Spec (`:55` bis `:68`). Sie gelten als Erweiterung von B-3 und H-2 und werden mit Frage 13 bewusst übernommen; der Teil aus Idee 01 wird nach `docs/ideas/README.md` «bewusst in Spec und Plan übernommen», hier über das Entscheidungsprotokoll |

**Was weiterhin nicht gilt:** keine Bibliotheksnavigation für Zuschauer (2.1); keine Sprachen,
Bildarten, Bildsprache, Bildwahl je Titel, kein Fanart.tv, OMDb oder NFO (Idee 01, Teil B); kein AniDB,
kein MyAnimeList; kein externer Hörbuch-Anbieter (F-7); keine Podcast-Suche (ADR 0007, offen);
**kein Löschen einer Bibliothek** — es gibt heute keine `DELETE FROM libraries`-Abfrage und keine
Operation dafür, und dieses Papier führt sie nicht ein, weil Marcos Zielstruktur mit Ordner-Umhängen
erreichbar ist (Abschnitt 8.1).

---

## 4. Datenmodell

### 4.1 `libraries.kind` wird zur Familie

Die `CHECK`-Bedingung wird zu `kind IN ('video', 'audiobooks', 'podcasts')`. SQLite kann eine
`CHECK`-Bedingung nicht ändern; die Tabelle wird nach Vorlage neu gebaut (wie in 00014). Spaltenliste,
Scan-Spalten und `loss_accepted_at` bleiben wörtlich; die `id` bleibt erhalten, damit `items`,
`library_roots`, `profile_libraries`, `account_libraries` und `invitation_libraries` weiter stimmen.

Warum nicht die Spalte streichen und die Familie aus den Ordnern ableiten? Weil eine Bibliothek ohne
Ordner erlaubt ist (E13) und weil `default_rating`, `ScanLibrary` (Feed-Abruf oder Ordner-Scan) und
die Podcast-Abos (`metadata/podcast.go:214`, `Subscribe` prüft die Art) eine Antwort brauchen, bevor
der erste Ordner da ist.

Warum `video` und nicht `movies` behalten? Weil ein entfernter Enum-Wert nie wiederverwendet wird
(`06-benutzerverwaltung.md`, 6.3) und `movies` nach dieser Anhebung etwas anderes bedeuten würde als
vorher. Fremd-Clients, die `kind === 'movies'` prüfen, sollen **brechen**, nicht stillschweigend
falsch liegen.

### 4.2 `library_roots` bekommt Inhaltsart und Scraper-Liste

```sql
ALTER TABLE library_roots ADD COLUMN content TEXT NOT NULL DEFAULT 'movie'
    CHECK (content IN ('movie', 'series', 'audiobook', 'podcast'));
ALTER TABLE library_roots ADD COLUMN providers TEXT NOT NULL DEFAULT '["tmdb"]'
    CHECK (json_valid(providers) AND json_array_length(providers) >= 1);
```

`providers` ist eine JSON-Liste in Reihenfolge, zum Beispiel `["tvdb","tmdb","anilist"]`, nach dem
Muster von `items.genres` und `items.locked_fields` (`00003_libraries_items.sql:58`, `:67`). Eine
eigene Tabelle `library_root_providers (root_id, position, provider)` wäre normalisierter, aber die
Liste wird nur als Ganzes gelesen und geschrieben, ist nie länger als vier Einträge, und `sqlc` liefert
sie als `string`, die `model` einmal parst. `ADD COLUMN` mit `DEFAULT` genügt (kein Neubau); die
Vorgaben dienen nur dem Befüllen in der Migration (4.6), der Server setzt beide Werte bei jedem
`INSERT` ausdrücklich (`InsertLibraryRoot`, `server/queries/app/libraries.sql:13`). Die Regeln aus
Abschnitt 2 (Familie, erlaubter Erkenner, erlaubte Auffüller, keine Doppelten) lassen sich nicht als
`CHECK` schreiben; sie werden in `library.Service` geprüft (bei `Create`, `AddRoot`, `UpdateRoot`;
`model.ValidRootProviders(content, providers)`) und in der Migration als Wächter ausgeführt (4.6,
Punkt 5).

### 4.3 `items` lernt seine Liste und seine Herkunft je Feld

`metadata.match` arbeitet je Titel und kennt die Bibliothek nicht (`match.go:78`). Ein Serien-Item hat
keine `media_files` (nur seine Folgen), kann seine Wurzel also nicht über `media_files.root_id`
finden, und ein Titel darf theoretisch Dateien in zwei Wurzeln derselben Bibliothek haben. Darum
bekommt jeder Titel seine Liste **kopiert**, nicht abgeleitet:

```sql
ALTER TABLE items ADD COLUMN providers TEXT NOT NULL DEFAULT '["tmdb"]'
    CHECK (json_valid(providers));
ALTER TABLE items ADD COLUMN anilist_id INTEGER;
ALTER TABLE items ADD COLUMN feed_origin TEXT
    CHECK (feed_origin IS NULL OR feed_origin IN ('admin', 'file'));

CREATE TABLE item_field_sources (
    item_id  TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    field    TEXT NOT NULL,        -- title, overview, tagline, original_title, genres,
                                   -- runtime_minutes, release_date, own_rating, poster, backdrop
    lang     TEXT NOT NULL DEFAULT '',  -- '' für sprachlose Felder
    provider TEXT NOT NULL,
    PRIMARY KEY (item_id, field, lang)
) STRICT;
```

- Der Scanner schreibt `providers` beim Anlegen aus der Wurzel. Betroffen sind **alle vier
  Einfügewege**, nicht nur `InsertItem` (`server/queries/system/items.sql:4`): `InsertAudiobook`
  (`system/audiobooks.sql:5`), `PodcastInsert` und `PodcastInsertEpisode` (`system/podcasts.sql:12`,
  `:66`). Ohne diese drei stünde an jedem Buch und jedem Podcast stumm `["tmdb"]`. Kinder (Staffel,
  Folge, Extra, Episode) erben die Liste des Elternteils.
- Ändert ein Administrator die Liste einer Wurzel (F-4), setzt `UpdateRoot` sie auf allen Titeln, die
  eine Datei unter dieser Wurzel haben, samt Eltern und Kindern, und reiht je Titel den passenden Job
  ein (Abschnitt 6.3).
- **`item_field_sources`** ist die Tabelle aus Idee 01 (`01-metadaten-einstellungen.md:116` bis
  `:119`), hier mit `lang ''` statt `NULL`, weil `NULL` in einem Primärschlüssel in SQLite nicht
  eindeutig ist. Sie macht das feldweise Auffüllen **wiederholbar**: ohne sie wüsste der zweite Lauf
  nicht, ob eine Beschreibung von TMDB stammt (dann darf TVDB sie nicht ersetzen) oder von TVDB (dann
  darf ein neuer TMDB-Wert sie ersetzen, weil TMDB vorne steht). `items.rating_source` bleibt als
  Kurzform für die Anzeige, wie Idee 01 es vorsieht.
- `anilist_id` steht neben `tmdb_id`, `imdb_id` (`00003_libraries_items.sql:59`, `:60`) und
  `tvdb_id` (Spalte seit `00008_library_complete.sql:23`, Index `items_tvdb` `:32`).
- `feed_origin` sagt bei einem Podcast, **wer den Feed bestimmt hat**: `admin` für ein von Hand
  abonniertes Abo, `file` für eine `feedURL` aus einer `metadata.json`. Davon hängt der Netzwächter
  ab (9.1). `NULL` für Ordner-Podcasts ohne Feed und für alle Nicht-Podcasts.
- Feed-Podcasts ohne Ordner und Hörbücher tragen `["local"]`. Beides wird in der Migration aus der
  Titelart gesetzt; bestehende Video-Titel bekommen `["tmdb"]` (Filme) bzw. `["tmdb","tvdb"]`
  (Serien), also genau die heutige Reihenfolge.

Warum keine Spalte `items.root_id`? Weil sie für Container falsch wäre (eine Serie gehört zur
Bibliothek, ihre Folgen zu Wurzeln) und weil `media_files.root_id` mit Index
`media_files_root_item (root_id, item_id)` (`00020_media_files_root_item.sql:14`) die Frage «welche
Titel liegen unter dieser Wurzel» bereits beantwortet — genau die Abfrage, die
`SoftDeleteItemsOfRootWithoutOtherFiles` (`server/queries/system/library.sql:73`) heute benutzt.

### 4.4 Erkennen und Auffüllen in Reihenfolge

Zwei Rollen, strikt getrennt — das ist die Antwort auf den Einwand «Widersprüche bei Folgennummern»
aus ADR 0006:

**Der Erkenner** ist der erste Eintrag der Liste. Er allein bestimmt Identität, Ids und bei Serien die
Staffel- und Folgenzählung:

- `tmdb` zuerst: wie heute (`matchMovie` `:122`, `matchSeries` `:187`, `matchSeasons` `:329` ff.).
- `tvdb` zuerst (nur `series`): `matchSeries` benutzt `s.tvdb` für Suche und Serie
  (`SearchSeries` `tvdb.go:271`, `Series` `:357`), `matchSeasons` schreibt Staffeln und Folgen nach
  TVDB-Zählung. Die `tmdb_id` holt der Erkenner **zusätzlich** über TMDBs Fremdschlüssel-Suche
  (`tmdb.go:267`, `/find/{tvdb_id}` mit `external_source tvdb_id`), damit Credits, Trailer-Kette und
  Bildpipeline, die an `tmdb_id` hängen, weiter arbeiten — aber ohne dass TMDB die Zählung bestimmt.
  `seriesProvider` (`:315`) wählt neu nach dem ersten Listeneintrag, nicht mehr «TMDB, wenn vorhanden».
  Der heutige Sonderfall `manualTVDB` (`:198`: TVDB-Id gesperrt, TMDB-Id leer) bleibt als manuelle
  Zuordnung erhalten; er ist jetzt derselbe Weg wie «`tvdb` zuerst», nur je Titel statt je Ordner.
- `local` zuerst: der heutige Hörbuch- und der neue Podcast-Erkenner (4.5), ohne Netz.

**Die Auffüller** sind alle weiteren Einträge. Nach jedem erfolgreichen Match reiht `match` **in
derselben Transaktion** den Job `metadata.fill` ein (Schlüssel `fill:<item>`, Priorität wie
`image.fetch`). Der Job läuft für die Liste von vorne nach hinten, **Erkenner eingeschlossen**, und
entscheidet je Feld und Sprache:

1. Ein Feld, das in `items.locked_fields` steht (manuelle Sperre, B-6), wird nie angefasst.
2. Sonst gewinnt der **erste Anbieter in der Liste, der einen nicht-leeren Wert hat**; `item_field_sources`
   hält fest, wer gewonnen hat. Steht dort schon ein Anbieter, der in der Liste **weiter vorne**
   liegt oder derselbe ist, bleibt der Wert; steht dort einer, der **weiter hinten** liegt, wird
   überschrieben. So ist der Job idempotent und darf beliebig oft laufen — nach einem erneuten Match,
   nach einer Listenänderung, nach einem Ausfall.
3. Felder: `title`, `overview`, `tagline` je Sprache (`item_texts`), `original_title`, `genres`
   (Vereinigung aller, Herkunft `union`), `runtime_minutes`, `release_date`, `poster`, `backdrop`.
   **Freigaben sind die Ausnahme** (K-3, derselbe Grundsatz wie in Idee 01): `own_rating` wird nur
   **erhöht**, nie gesenkt, gleich wo der Anbieter steht; `rating_source` nennt den strengsten.
4. Antworten der Anbieter werden je Lauf einmal geholt (TMDB und TVDB über die bestehenden Clients
   und `ratelimit.Limiter`; AniList unten). Ein Anbieter ohne Schlüssel (`Configured` falsch) wird
   übersprungen und als Pflegezeile «TVDB angekreuzt, aber kein Schlüssel» gemeldet; die
   Oberfläche zeigt denselben Hinweis schon beim Ankreuzen (`listMetadataProviders`, 5.2).

Antwort auf den zweiten Einwand aus ADR 0006 («doppelte Anfragen bei jedem Titel»): Anfragen gibt
es nur für angekreuzte Anbieter, und nur `metadata.fill` stellt sie, einmal je Lauf. Ein Ordner mit
`[tmdb]` kostet genau so viel wie heute.

**AniList** (F-5) ist ein reiner Auffüller mit zwei Besonderheiten:

1. **Zuordnung** über die Liste von Fribb (`github.com/Fribb/anime-lists`, Datei
   `anime-list-full.json`; Felder laut README unter anderem `anilist_id`, `tvdb_id`, `themoviedb_id`,
   `imdb_id`, `mal_id`, `anidb_id`, `season`, `episode_offset`, `type` — **die genauen Feldnamen und
   ihre Form sind vor Schritt 4 gegen die Datei zu prüfen**). Die Liste wird einmal geladen, mit ETag
   unter `/cache/anime-lists/` gehalten, wöchentlich erneuert, beim Parsen auf 64 MB begrenzt und
   **gestreamt** in zwei Karten übersetzt, **je nach Art getrennt** (TMDB-Film- und Serien-Ids sind
   verschiedene Zahlenräume): `(movie, tmdb) → anilist` und `(tvdb) → [anilist mit season,
   episode_offset]`. Bei Serien zeigt eine TVDB-Id auf mehrere AniList-Einträge (eine je Staffel);
   für die Titelebene gilt `season = 1`. `anilist_id = 0` heisst «nachgeschlagen, kein Treffer»;
   die Liste wird trotzdem beim nächsten Lauf wieder gefragt, denn sie wächst.
2. **Abfrage** bei `https://graphql.anilist.co` (GraphQL, **ohne Schlüssel**): `Media(id)` mit
   `title { romaji english native }`, `description`, `genres`, `coverImage.extraLarge`,
   `bannerImage`, `isAdult`. Rate-Limit laut AniList-Dokumentation **90 Anfragen je Minute** mit
   `X-RateLimit-Remaining` und `Retry-After`; AniList senkt das Limit bei Störungen, Drittbibliotheken
   berichten zeitweise von 30. Der Client läuft über **`ratelimit.Limiter`**
   (`server/internal/ratelimit/ratelimit.go:60`, `Do` `:166`, `Backoff` `:149`, `parseRetryAfter`
   `:219` — derselbe Weg wie TMDB und TVDB) mit 1 Anfrage je Sekunde, Burst 5. Nicht
   `ratelimit.Quota` (`quota.go:36`): das ist das Tagesbudget der YouTube-Kette.
   AniList liefert `original_title` = `title.native`, englische Texte aus `title.english` (sonst
   `romaji`) und `description`, Genres, Poster und Backdrop; `isAdult = true` hebt auf 18 mit
   `rating_source 'anilist'`. Keine deutschen Texte. Wer AniList **vor** TMDB setzt, bekommt darum
   englische Beschreibungen, wo TMDB deutsche hätte — die Oberfläche sagt das beim Sortieren.

**Konvergenz.** `QueuePendingMetadata(libraryID)` (`match.go:1062`) reiht heute nur
`metadata_state = 'pending'` und die Nachrüstung ein. Neu reiht es zusätzlich jeden Titel mit
`metadata_state = 'matched'` als `metadata.fill` ein, dessen Liste mehr als einen Eintrag hat und
dessen `item_field_sources` leer sind oder einen Anbieter nennen, der nicht mehr in der Liste steht.
Damit kommt ein Titel auch nach `SLOUCHCAT_ANILIST=off` → `on`, nach einem Ausfall oder nach einer
Listenänderung (6.3) zu seinen Feldern.

**Das Fenster.** Zwischen `UpdateItemMatch` und dem Lauf von `metadata.fill` trägt ein Titel kurz nur
die Werte des Erkenners. Für eine Freigabe, die nur ein Auffüller kennt (AniList `isAdult`), sieht ein
Kinderprofil den Titel in diesem Fenster. Es ist so lang wie die Warteschlange vor dem Job; ADR 0015
benennt es als Restrisiko (12.6). Null wäre nur mit allen Netzaufrufen **in** der Match-Transaktion
zu haben — das tut dieses Projekt nirgends.

Was fehlt und bewusst fehlt: eine **Titelsuche bei AniList** für Ordner, die weder TMDB noch TVDB
kennen; ein solcher Titel bleibt `unmatched` und erscheint in den Pflegewerkzeugen (B-9). Und die
**Herkunft in der Zuschauer-API**: `item_field_sources` wird nur auf der Admin-Titelseite gezeigt
(5.1), nicht in `ItemDetail`.

Vor Schritt 4 **gegen die Quelle zu prüfen**, weil dieses Papier es nicht verifizieren konnte: die
Rohadresse der Zuordnungsliste (erwartet
`https://raw.githubusercontent.com/Fribb/anime-lists/master/anime-list-full.json`), die Feldnamen,
die Lizenz für die Weiterverwendung in einem AGPL-Projekt, und ob Filme in der Liste zuverlässig
`themoviedb_id` tragen (die Liste verschmilzt über `anidb_id`; Filme sind dort seltener als Serien).

### 4.5 Die Quelle `local` für Podcasts: Ordner lesen, Feed mitnehmen

Ein Ordner mit `content = 'podcast'` wird so gelesen (alles im Paket `library`, über `paths`, Regel 4;
`library` darf `store/systemq` sehen, `.golangci.yml:35` bis `:41`):

**Scan (Dateien und `metadata.json`, keine Tags).**

- **Jeder Unterordner der ersten Ebene ist ein Podcast** (`items.kind = 'podcast'`), so wie bei
  Hörbüchern jeder Buchordner ein Buch ist. Audiodateien **direkt in der Wurzel** werden
  übergangen und nur in `scan_files_seen` gezählt — anders als bei Büchern, wo eine Datei in der
  Wurzel ein Buch ist (`audiobook.go:20` bis `:24`); ein Podcast ist immer ein Ordner.
- **`metadata.json`** (Audiobookshelf-Schema, Beispiel `NZZ Megahertz`: `title`, `author`,
  `description`, `releaseDate`, `genres`, `feedURL`, `imageURL`, `itunesId`, `language`, `explicit`,
  `podcastType`) wird mit derselben Grössenbremse gelesen wie bei Büchern (`audiobook.go:188`,
  `maxMetadataJSON`; `absMeta` `:172` wird um die Podcast-Felder erweitert). Sie füllt Titel,
  `author`, Beschreibung (`item_texts` in `language`, sonst `en`), `genres`, `explicit` (→ FSK 18,
  `rating_source 'tags'`, wie ADR 0007 es für Bücher tut) und **`feed_url`** mit `feed_origin =
  'file'`. Fehlt sie, ist der Ordnername der Titel. Die Standard-Altersstufe der Bibliothek gilt wie
  für Bücher (`rating_source 'library'`, AN-2).
- **`cover.jpg`/`folder.jpg`/`poster.jpg`** wird wie bei Büchern `cover_path`.
- **Jede Audiodatei im Podcast-Ordner ist eine Episode** (`podcast_episode`) mit einer Zeile in
  `media_files` (`root_id`, `rel_path`, `part_index NULL`). Beim Scan heisst sie wie die Datei und
  trägt `mtime` als `published_at`; die Identität einer Ordner-Episode ist `root_id + rel_path` (der
  `UNIQUE` von `media_files`), **nicht** die GUID — die hat eine lokale Datei nicht.
- **Feed übernehmen (F-6), aber nie doppelt:** Steht eine `feedURL` in der `metadata.json`, sucht
  der Scanner zuerst mit `PodcastFindByFeed` (`system/podcasts.sql:5`) nach einem Podcast dieser
  Bibliothek mit derselben Adresse. **Gibt es ihn** (ein Abo von Hand oder ein zweiter Ordner mit
  demselben Feed), wird er **übernommen**: die Episoden des Ordners hängen sich an ihn, der Ordner
  bekommt keinen eigenen Podcast. Ohne das verletzt der zweite `INSERT` den eindeutigen Index
  `items_podcast_feed (library_id, feed_url) WHERE kind = 'podcast' AND deleted_at IS NULL`
  (`00014_audio.sql:83`), und weil ein Batch **eine** Transaktion ist (`write.go:228` bis `:236`),
  scheiterte damit jeder Scan dieser Bibliothek. Ein zweiter Ordner mit demselben Feed bekommt
  zusätzlich eine Pflegezeile «zwei Ordner, ein Feed». **Gibt es ihn nicht**, prüft der Scanner die
  Adresse nach 9.1 (`http`/`https`, kein lokaler Host) und legt den Podcast mit `feed_url` und
  `feed_origin = 'file'` an; der Feed läuft danach durch den Abruf wie jeder andere (bedingtes GET,
  2000-Episoden-Grenze, 55-Minuten-Ticker, ADR 0007).
- **Ordner-Podcasts ohne Feed werden nicht abgefragt.** `PodcastListDue` (`system/podcasts.sql:53`)
  und `PodcastListIDsByLibrary` (`:49`, benutzt von `RefreshLibrary`) filtern heute nicht auf
  `feed_url IS NOT NULL`; ohne die Bedingung liefe alle 55 Minuten `podcast.fetch` mit leerer
  Adresse und schriebe `feed_error`. Beide Abfragen bekommen `AND feed_url IS NOT NULL`.
- **Vorrang zwischen Feed und `metadata.json`:** Hat der Podcast einen Feed, gewinnt der Feed für
  Titel, Autor, Beschreibung, Cover und `explicit` (`PodcastUpdateChannel`, `:21`, schreibt sie bei
  jedem Abruf); die `metadata.json` ist bei Audiobookshelf ohnehin eine Kopie des Feeds. Nur ohne
  Feed bleibt die `metadata.json` die Quelle.

**Nach der Sonde (Tags und Zusammenführen).** Die Tags einer Audiodatei kennt der Server erst, wenn
`probe.file` gelaufen ist (`transcode.Format.Tags`; so arbeitet `finishAudiobook`, `probe.go:173`).
Darum bekommt `probe.go` einen Schritt **`finishPodcastEpisode`**, der nach der Sonde einer
`podcast_episode` mit Datei:

1. Titel (`title`), Veröffentlichung (`date`/`releasedate`, RFC 2822, also `Fri, 13 Feb 2026
   06:54:00 +0100`), Beschreibung (`comment`, sonst `subtitle`) und Dauer schreibt. Ein Tag-Titel,
   der **gleich dem Podcast-Titel** ist, zählt nicht als Titel (auf dem NAS tragen 11 von 239 Dateien
   von «100 Sekunden Wissen» genau das: Dateiname `100 Sekunden Wissen (<uuid>).mp3`, Tag-Titel «100
   Sekunden Wissen»); dann bleibt der Dateiname.
2. **die Datei mit einer Feed-Episode zusammenführt, nur bei eindeutigem Paar.** Kandidaten sind
   die Feed-Episoden desselben Podcasts ohne Datei. Erster Schlüssel: `published_at` **auf die Minute
   gleich** (Audiobookshelf übernimmt das `pubDate` des Feeds in den Tag; beim SRF-Feed stimmen
   beide überein). Zweiter Schlüssel: normalisierter Titel gleich (Kleinschreibung, ohne
   Satzzeichen), nie der Podcast-Titel. Nachprüfung in beiden Fällen: Dauer innerhalb ±2 %, sofern
   der Feed eine nennt. Trifft **genau eine** Episode, bekommt sie die `media_file`, behält `guid`,
   `enclosure_url` und den Feed-Text, und die Datei-Episode geht auf. Trifft keine oder mehr als eine,
   geschieht **nichts** — zwei sichtbare Episoden, keine falsch verheiratete. Die Pflegewerkzeuge
   zeigen den Fall als neue Gruppenart in `listDuplicates` (5.1). Dateinamen taugen nicht als
   Schlüssel: Audiobookshelf benennt die Datei nach dem Episodentitel (`Godzilla.mp3`), die Enclosure
   heisst `100_Sekunden_Wissen_radio_AUDI20261028_RS_0001_….mp3`; die `(uuid)` im Dateinamen ist
   Audiobookshelfs eigene Id, nicht die Feed-GUID (geprüft am 28. Oktober 2026 gegen den SRF-Feed).
   Umgekehrt führt der Feed-Abruf (`podcast.go:472` ff.) eine **neue** Feed-Episode vor dem Anlegen
   nach derselben Regel mit einer dateigestützten Episode ohne GUID zusammen.
3. **keine Benachrichtigung beim Erst-Scan.** Der Hook für neue Folgen (`library/service.go:112`,
   `WithNewEpisodes`) und die Auto-Downloads (O-3) feuern für Ordner-Episoden nur, wenn der Podcast
   **vor** diesem Scan-Batch schon bestand; die Regel des Feed-Wegs («neuer als die neueste bekannte
   Episode», `podcast.go:475`) gilt sinngemäss. Sonst lösten 26 000 Dateien beim ersten Scan 26 000
   Ereignisse aus.

**Wiedergabe und Download bevorzugen die Datei.** Offline tut es schon
(`server/internal/offline/audio.go:113` bis `:123`: «its file, else its enclosure»). Die Wiedergabe
noch nicht: `playback/sessions.go:116` schickt jede `podcast_episode` zu `startEnclosure`; neu geht
eine Episode **mit** Datei den Hörbuchweg (direkt oder Audio-HLS über `transcode`), und
`GetVisibleEnclosure` (`server/queries/scoped/podcasts.sql:27` bis `:31`) ist nur noch der Rückfall.
Eine Fixture mit Datei **und** Enclosure gibt es bereits (`internal/visibility/vistest/audio.go:34`).
Die Trigger für `items.available` decken `podcast_episode` mit Datei seit 00014.

**Verschwinden und Aufräumen.** Audiobookshelf löscht alte Episoden routinemässig. Heute schliessen
`SoftDeleteItemsMissingSince` (`system/items.sql:52`) und `SoftDeleteItemsOfRootWithoutOtherFiles`
(`system/library.sql:73`) die Art `podcast_episode` aus; eine reine Datei-Episode bliebe nach dem
Löschen der Datei für immer mit `available = 0` stehen, und das Entfernen eines Podcast-Ordners
liesse Podcasts und Episoden zurück. Darum: beide Abfragen nehmen `podcast_episode` mit
`enclosure_url IS NULL` auf (eine Episode mit Enclosure bleibt als Feed-Episode stehen, nur ohne
Datei); ein neuer Sweep räumt Podcasts mit `feed_url IS NULL` und ohne lebende Episode weich ab
(Vorbild `SoftDeleteEmptySeries`, `library.sql:32`); und beim Entfernen eines Podcast-Ordners
(`removeLibraryRoot`) bleibt ein Podcast **mit** Feed als gewöhnliches Abo bestehen (`feed_origin`
wird `admin`, damit der Administrator ihn sieht und löschen kann), ein Podcast ohne Feed geht mit
seinen Episoden.

**Last, nüchtern gerechnet.** Marcos Podcast-Ordner hat **26 031 Audiodateien**. Jede neue Datei
reiht `probe.file` ein, und `probe.keyframes` folgt (`probe.go:164`). Die Sonde liest schon heute nur
`-show_format -show_streams -show_chapters` ohne Pakete (`transcode/probe.go:181`), und
`handleKeyframes` überspringt Audiodateien bereits (`probe.go:218` bis `:221`). Der Preis ist also
**nicht** das Lesen der Dateien, sondern rund 26 000 ffprobe-Starts über NFS mit hoher Latenz plus
26 000 Keyframe-Jobs, die sofort nichts tun. Beides ist auf CT 190 Stunden, nicht Wochen. Schritt 3
reiht `probe.keyframes` für `audiobook` und `podcast_episode` gar nicht erst ein und misst den
Erst-Scan (`docs/perf/bibliotheken.md`).

### 4.6 Migrationsplan 00023 (Schritte, kein fertiges SQL)

Nach `CLAUDE.md` Regel 5 und `server/migrations/TEMPLATE.sql.txt`: nur vorwärts, `-- +goose NO
TRANSACTION`, `PRAGMA foreign_keys=OFF`, Änderungen zwischen `BEGIN` und `COMMIT`, am Ende
`PRAGMA foreign_key_check` und `PRAGMA foreign_keys=ON`. Der Server sichert vor dem Lauf mit
`VACUUM INTO`.

1. **`library_roots` erweitern** (4.2) mit `ADD COLUMN` und Vorgaben. Dann befüllen aus der
   Bibliothek: `UPDATE library_roots SET content = CASE l.kind WHEN 'movies' THEN 'movie' WHEN
   'series' THEN 'series' WHEN 'audiobooks' THEN 'audiobook' END, providers = CASE l.kind WHEN
   'movies' THEN '["tmdb"]' WHEN 'series' THEN '["tmdb","tvdb"]' WHEN 'audiobooks' THEN '["local"]'
   END FROM libraries l WHERE l.id = library_id`. Beide `CASE` haben **kein `ELSE`**: eine Wurzel an
   einer Podcast-Bibliothek (die es heute nicht geben kann, `roots.go:34`) ergäbe `NULL` und
   scheiterte am `NOT NULL` — der erste Wächter. `["tmdb","tvdb"]` für Serien ist genau das heutige
   Verhalten (TMDB zuerst, TVDB als Rückfall und Auffüller).
2. **`items` erweitern** (4.3): `providers` nach Titelart (`movie`, `extra` → `["tmdb"]`; `series`,
   `season`, `episode` → `["tmdb","tvdb"]`; `audiobook`, `podcast`, `podcast_episode` → `["local"]`),
   `anilist_id NULL`, `feed_origin = 'admin'` für jeden Podcast mit `feed_url`, sonst `NULL`.
   `item_field_sources` anlegen und **leer lassen**: die Herkunft bestehender Titel ist unbekannt,
   der erste `metadata.fill` je Titel schreibt sie nach (Konvergenz, 4.4) — für bestehende Titel
   mit `["tmdb"]` ist das ein Lauf ohne Netzaufruf, weil nur der Erkenner in der Liste steht.
3. **`libraries` neu bauen** mit `kind IN ('video', 'audiobooks', 'podcasts')`; Zeilen kopieren mit
   `CASE kind WHEN 'movies' THEN 'video' WHEN 'series' THEN 'video' ELSE kind END`, `id` erhalten;
   tauschen; Indizes neu anlegen. **Vor dem Schreiben die Spaltenliste gegen `PRAGMA
   table_info(libraries)` auf dem eingefrorenen M11-Stand abgleichen** (dieselbe Vorsicht wie in
   `06-benutzerverwaltung.md` 4.6).
4. **Sicht `visible_items` bleibt stehen.** Sie liest `items`, `profiles`, `accounts`,
   `profile_libraries` und `account_libraries`, aber **nicht** `libraries`
   (`00010_container_visibility.sql:148` bis `:167`, auf dem Zweig wortgleich in `00022:75` bis
   `:94`); auch kein Trigger liest `libraries`. Der Neubau folgt dem Muster von 00014, das die
   Sicht ebenfalls nicht angefasst hat. Schritt 1 belegt das mit `SELECT sql FROM sqlite_master WHERE
   sql LIKE '%libraries%'` auf dem M11-Stand.
5. **Wächter im selben Lauf.** Ein blosses `SELECT` bricht eine Migration nicht ab, und
   `PRAGMA foreign_key_check` läuft erst in Go nach `COMMIT` (`server/internal/store/migrate.go:166`).
   Darum ein Wächter, der **scheitert**: `CREATE TEMP TABLE guard (n INTEGER CHECK (n = 0))`, dann
   `INSERT INTO guard SELECT count(*) FROM library_roots r JOIN libraries l ON l.id = r.library_id
   WHERE NOT ((l.kind = 'video' AND r.content IN ('movie', 'series')) OR (l.kind = 'audiobooks' AND
   r.content = 'audiobook') OR (l.kind = 'podcasts' AND r.content = 'podcast'))` — eine Zeile mit
   `n > 0` verletzt die `CHECK`-Bedingung und beendet den Lauf vor `COMMIT`. Dasselbe Muster für
   «der erste Eintrag von `providers` passt zur Inhaltsart» (`json_extract(providers, '$[0]')`).
6. **Kein Datenverlust an Kindtabellen:** `items`, `library_roots`, `account_libraries`,
   `invitation_libraries` zeigen über Fremdschlüssel auf `libraries`; `profile_libraries` hat keinen
   (`00002_accounts_sessions.sql:43`). Die `id` bleibt, die Fremdschlüssel sind während des Laufs
   aus; `foreign_key_check` nach dem Lauf ist der Beleg.

**Was die Migration ausdrücklich nicht tut:** Sie legt keine Ordner an, abonniert keinen Feed, reiht
keinen Scan ein, stellt keinen Match neu an und fasst keine Downloads an. Marcos Zielstruktur entsteht
danach in der Oberfläche (8.1).

---

## 5. Auswirkung auf den Vertrag

### 5.1 Schemata

| Schema | Heute (`api/openapi.yaml`) | Neu |
|---|---|---|
| `LibraryKind` (`:7759`) | `[movies, series, audiobooks, podcasts]` | `[video, audiobooks, podcasts]` |
| `LibrarySummary` (`:7766`) | `id, name, kind, defaultRating` | unverändert; `kind` mit neuen Werten |
| `LibraryRequest` (`:7781`) | `name, kind, path?, defaultRating?` | `name, kind, defaultRating?, roots: LibraryRootRequest[]` (F-13) — jeder Ordner bringt Inhaltsart und Liste mit; `minItems: 1` für `video` und `audiobooks` (422 `roots_required`), bei `podcasts` darf die Liste leer sein. Der Server legt **alle Ordner oder keinen** an (eine Transaktion); überschneiden sich zwei Ordner der Anfrage oder einer mit dem Bestand, kommt 422 `root_overlap` mit dem Feld des Verursachers. **Die Validierungsfelder heissen damit `library.roots[1].path` statt `library.path`**; die Fehlerzuordnung des Einrichtungsassistenten hängt an diesen Namen |
| `LibraryRootRequest` (`:7805`) | `path` | `path, content, providers` (alle Pflicht; `providers` geordnet, `minItems: 1`, `uniqueItems: true`) |
| `LibraryRootsRequest` | — | neu (F-13): `roots: LibraryRootRequest[]`, `minItems: 1`; Körper von `addLibraryRoot`, damit der Dialog «Ordner von X» mehrere Ordner mit einem Klick anlegt |
| `LibraryRootUpdate` | — | neu: `content?, providers?, confirmPath?` (5.2) |
| `LibraryRoot` (`:7878`) | `id, path, fsType, watch, itemCount, fileCount, warning?` | zusätzlich `content`, `providers` |
| `LibraryUpdate` (`:7831`) | `name?, defaultRating?` | unverändert — die Familie ist nach dem Anlegen fest (wie heute `kind`) |
| `RootContent` | — | neu: `[movie, series, audiobook, podcast]` |
| `MetadataProvider` (`:7494`) | `[tmdb, tvdb]` — Parameter von `searchMetadata` und der manuellen Zuordnung | **erweitert** um `anilist` und `local`; `searchMetadata` und `assignMatch` nehmen weiter nur `tmdb` und `tvdb` an (422 `provider_not_searchable`), weil nur Erkenner suchbar sind |
| `MetadataProviderInfo` | — | neu, für `listMetadataProviders`: `id`, `configured` (Schlüssel vorhanden), `roles: [identify, fill]`, `contents: RootContent[]`, `languages` (für den Hinweis «liefert kein Deutsch») |
| `FieldSource` | — | neu: `field, lang, provider` |
| `DuplicateGroup.reason` (`:7706`) | `[provider_id, title_year, file_hash]` | zusätzlich `file_and_feed` (4.5) |
| `ItemRef`, `ItemDetail` | — | **unverändert.** Weder `anilistId` noch Herkunft werden an Zuschauer ausgeliefert |
| `AdminItem` | `libraryId, …` | zusätzlich `providers`, `anilistId?`, `fieldSources: FieldSource[]` |
| `AdminPodcast` (`:7292`) | Feed-Daten | zusätzlich `feedOrigin?`, `rootId?`, `localEpisodeCount` |
| `SetupRequest.library` (`:127`, `runSetup`) | `LibraryRequest` | folgt automatisch; der Assistent zeigt die Liste mit der Vorgabe `[tmdb]` bzw. `[tmdb, tvdb]` |

### 5.2 Operationen

| Operation | Änderung | `x-visibility` |
|---|---|---|
| `createLibrary` (`:2157`) | Körper nach 5.1 mit `roots[]`; 422 `content_not_allowed`, `providers_not_allowed`, `roots_required`, `root_overlap`, weiterhin `library_path_invalid`; jeder 422 nennt das Feld mit Index (`library.roots[1].providers`). Bibliothek und alle Ordner entstehen in **einer** Transaktion; ein Fehler an Ordner 2 lässt keine Bibliothek zurück | admin |
| `addLibraryRoot` (`:2368`) | Körper wird `LibraryRootsRequest` (Liste, F-13); dieselben 422 mit Index; alle Ordner der Anfrage oder keiner (eine Transaktion, danach **ein** Scan für die Bibliothek); Podcast-Bibliotheken nehmen Ordner mit `content: podcast` an. Der Operationsname bleibt, der Pfad auch | admin |
| **`updateLibraryRoot`** — `PATCH /admin/libraries/{id}/roots/{rootId}` | **neu** (F-4). Eine Änderung der **Liste** löscht nichts und braucht keine Bestätigung (6.3). Ein Wechsel der **Inhaltsart** ist in der Wirkung «Ordner entfernen und neu anlegen» und verlangt `confirmPath` wie `removeLibraryRoot` (422 `confirm_mismatch`), 409 `scan_running` | admin |
| **`listMetadataProviders`** — `GET /admin/metadata/providers` | **neu**: die vier Anbieter mit `configured`, Rollen, erlaubten Inhaltsarten, Sprachen. Die Oberfläche baut daraus die Häkchenliste und die Hinweise («kein TVDB-Schlüssel», «AniList liefert kein Deutsch»). Idee 01 nannte dieselbe Operation (`GET /admin/providers`, `:208`) | admin |
| `scanLibrary` (`:2268`) | Für `podcasts`: Ordner-Scan **und** Feed-Abruf; heute nur Letzteres (`admin.go:357`) | admin |
| `removeLibraryRoot` (`:2414`) | Podcast-Ordner: Podcasts mit Feed bleiben als Abo (4.5) | admin |
| `listAccountLibraries` (`:857`) | nur neue `kind`-Werte | account |
| `searchMetadata` (`:3142`), `assignMatch` | `provider` weiter nur `tmdb`/`tvdb`; eine manuelle TVDB-Zuordnung an einem Titel mit `[tmdb, …]` wirkt wie heute (`manualTVDB`) | admin |
| `listDuplicates` | neue Gruppenart `file_and_feed` | admin |

Jede neue oder geänderte Operation bekommt ihren Eintrag im Vertragstest
(`TestContractCoversEveryOperation`, `server/internal/api/contract_access_test.go`) und den
Schema-Test auf `x-visibility` (Regel 7). Neue Fehlercodes gehen in die `problem`-Liste des Vertrags
und in `web/src/locales/*.json`.

### 5.3 Ist das eine brechende Anhebung?

**Ja.** `LibraryKind` verliert `movies` und `series`, `LibraryRequest.path` verschwindet zugunsten
von `roots[]`, `addLibraryRoot` nimmt eine Liste statt eines Ordners, `LibraryRootRequest` bekommt
Pflichtfelder, `MetadataProvider` wächst (für Clients, die
das Enum erschöpfend behandeln, ebenfalls brechend). Für Fremd-Clients (C-5) ist das brechend, wie es
1.1.0 (`HomeRowKey`) und 1.2.0 (`AccountRole`) waren. **Die Fassung ist 1.3.0**, dem Präzedenzfall
folgend, und der Bruch steht wörtlich in den Release-Notes. Entfernte Enum-Werte (`movies`, `series`
in `LibraryKind`) werden nie wiederverwendet.

Was **nicht** bricht: kein `scoped`-Schema ändert sich; die Web-App auf einem alten Stand gegen
einen neuen Server scheitert nur auf der Bibliotheksseite der Verwaltung.

---

## 6. Scanner, Metadaten und Jobs

### 6.1 Der Scanner liest je Wurzel

Heute ruft `scanLibrary` (`scanner.go:429`) für jede Wurzel `walkRoot(ctx, rs.proot, lib.Kind, …)`
(`:446`). Neu bekommt `walkRoot` die **Wurzel** (`op.rs.root` mit `Content` und `Providers`) statt
der Bibliothek; `fileInfo` trägt selbst keine Wurzel, der Wurzelzustand `rs` tut es. Die Stellen, die
heute `lib.Kind` lesen, lesen den Inhalt der Wurzel:

| Stelle | heute | neu |
|---|---|---|
| `parsePath` (`scanner.go:169`) | `kind` der Bibliothek | `content` der Wurzel; `podcast` → eigener Pfad (4.5) |
| `scannedExt` (`:181`) | Audio bei `audiobooks` | Audio bei `audiobook` **und** `podcast` |
| `wanted` (`:606`) | Serienbibliothek verwirft Dateien ohne Folgenmarke | Serien**wurzel** |
| `videoFile` (`:632`), `listDirFiles` (`pathscan.go:277`) | Bibliotheksart | Wurzelinhalt |
| Batchgrösse (`:511`) | kleiner bei `series` | kleiner, sobald **eine** Wurzel `series` ist |
| `QueuePendingSegments` (`:301`) | nur `series`-Bibliotheken | sobald eine Wurzel `series` ist |
| `batchWriter.item` (`write.go:364`), `describe` (`:52`), `describeBook` (`:221`), `touchBook` (`:299`, `:312`) | `w.lib.Kind` | Wurzelinhalt |
| **Titelindex** `itemIndex` (`index.go:23` bis `:38`, gefüllt in `loadItemIndex` `:112`, Art in `:125` bis `:131`) | **eine** Karte je Schlüssel (`byTMDB`, `byTVDB`, `byIMDB`, `byTitle`) für die eine Titelart der Bibliothek | **ein `itemIndex` je Titelart**, gewählt nach dem Inhalt der Wurzel. Eine gefilterte Gesamtkarte genügt nicht: TMDB-Film-Ids und TMDB-Serien-Ids sind getrennte Zahlenräume, und ein Film mit `tmdb_id 1399` und eine Serie mit `tmdb_id 1399` in derselben Bibliothek («Dokus») würden in einer gemeinsamen Karte aneinander gebunden. `titleKindOf` (`:86`), `fileFolderKey` (`:99`) und die Wiederanbindung weich gelöschter Titel (`relinkWindow`, `scanner.go:40`, `write.go:415`) arbeiten je Wurzel |
| `admin.validateLibrary` (`admin.go:240`) | Pfad je Art | Regeln aus Abschnitt 2 (`ValidRootProviders`) |
| `admin.ScanLibrary` (`:352`) | Podcasts → nur Feed | Podcasts → Ordner-Scan, dann Feed-Abruf |
| `library.Create` (`service.go:189`), `AddRoot` (`roots.go:26`) | Art prüfen, Podcasts ohne Wurzel | Regeln prüfen; Podcasts mit `content: podcast` |
| `maintenance.go:429` (Zusammenführen nur gleiche Art und gleiche Bibliothek) | unverändert | unverändert — Titelart, nicht Wurzel |
| Webhooks (`integrations/pathmap.go:66`) | Pfad → irgendeine Wurzel | unverändert; Radarr trifft Filmwurzeln, Sonarr Serienwurzeln, weil Marco die Umsetzungstabelle so pflegt. Eine Prüfung «Radarr darf nur auf `movie`-Wurzeln» wäre eine Sicherheitsgewinn-Illusion und bleibt aus |
| Go-Fixtures mit `Kind: "movies"` | 13 `*_test.go` im Server, `vistest/vistest.go:347`, `:389`, `vistest/audio.go:75`, `:79`, `cmd/apicheck/fixtures.go:228`, `:270`, `cmd/slouchcat/scanbench.go:73` bis `:92` | gehen **im selben Commit** wie das Schema um (10, Schritt 1) |

`scan.path` (`jobs.go:17` ff.) trägt `rootId` bereits je Verzeichnis; der Watcher läuft je Wurzel
(`service.go:807` ff.). Beides ändert sich nicht.

### 6.2 Metadaten

- `match` (`match.go:78`) liest `items.providers` und wählt den Erkenner nach dem ersten Eintrag
  (4.4); `seriesProvider` (`:315`) ebenso. Nach jedem erfolgreichen Match: `metadata.fill`.
  `audiobook` und `podcast` (`["local"]`) werden wie heute übersprungen (`:101`, `default`).
- Neuer Job **`metadata.fill`** in `server/internal/metadata/fill.go`: holt je Anbieter der Liste
  einmal den Datensatz (TMDB/TVDB über `Provider.Movie`/`Series`, AniList über `anilist.go`),
  wendet die Regel aus 4.4 an, schreibt `item_field_sources`, reiht bei geändertem Poster oder
  Backdrop `image.fetch` ein. Für Staffeln und Folgen läuft derselbe Job je Staffel (eine Anfrage je
  Anbieter und Staffel, wie `matchSeasons` es heute für den Erkenner tut), **nur** wenn mehr als
  ein Anbieter Staffeltexte liefern kann (TMDB, TVDB); AniList füllt nur die Titelebene.
- Neues Paket? **Nein.** `anilist.go` und `anime_lists.go` liegen neben `tmdb.go` und `tvdb.go`,
  Cache unter `/cache/anime-lists/`. Feste Hosts (`graphql.anilist.co`,
  `raw.githubusercontent.com`); `SLOUCHCAT_ANILIST=off` schaltet den Anbieter serverweit ab
  (`configured: false` in `listMetadataProviders`, die Häkchen bleiben erhalten). Die Zahl der
  `fill`-Läufe steht im Dashboard neben Trailern.
- `podcast.Subscribe` (`podcast.go:214`) prüft weiter die Art `podcasts` und setzt `feed_origin =
  'admin'`; `PodcastInsert` (`system/podcasts.sql:12`) nimmt neu `feed_url NULL` an (reiner
  Ordner-Podcast; die Spalte ist schon `NULL`-fähig, `00014_audio.sql:71`). Die zwei Abfragen des
  Abrufs bekommen `feed_url IS NOT NULL` (4.5).

### 6.3 `UpdateRoot`: Liste ändern ist billig, Erkenner wechseln ist ein Re-Match, Inhalt wechseln ist ein Umzug

**Liste ändern, Erkenner gleich** (zum Beispiel `[tmdb]` → `[tmdb, anilist]` oder Reihenfolge der
Auffüller): `UPDATE items SET providers = ?` für alle Titel mit einer Datei unter der Wurzel (Join
über `media_files.root_id`), dazu deren Eltern und Kinder; dann **je Titel** `metadata.fill`.
`QueuePendingMetadata(libraryID)` taugt dafür nicht, es nimmt keine Titel-Ids und reiht nur
`pending`. Keine Löschung, kein Scan, keine Bestätigung. Felder eines abgewählten Anbieters werden
beim nächsten Lauf vom nächsten in der Liste ersetzt; gibt es keinen, bleibt der Wert stehen und
`item_field_sources` nennt weiter den alten Anbieter (sichtbar auf der Admin-Titelseite, kein stilles
Löschen).

**Erkenner wechseln** (`[tmdb, tvdb]` → `[tvdb, tmdb]` an einem Serienordner): wie oben, plus
`metadata.match` je Serie. Die Serie behält `tmdb_id` **und** `tvdb_id`; Staffeln und Folgen werden
nach der Zählung des neuen Erkenners neu beschrieben. Die **Folgen-Items selbst bleiben**: ihre
Identität ist `(Serie, Staffelnummer, Folgennummer)` aus dem Dateinamen, nicht die Anbieter-Id —
Fortschritt und Merkliste hängen also weiter an derselben Datei. Was sich ändert, sind Titel,
Beschreibung und Bild einer Folge, wenn die beiden Anbieter dieselbe Nummer verschieden besetzen.
Genau das ist der Grund für den Wechsel bei Anime, und genau das steht im Hinweis des Dialogs
(«Staffel- und Folgentexte werden nach TVDB neu zugeordnet»). Keine Bestätigung nötig, weil nichts
gelöscht wird.

**Inhalt wechseln** (`movie` ↔ `series`; andere Wechsel verbieten die Regeln): unter der Scan-Sperre
der Bibliothek, in **einer** Transaktion, in dieser Reihenfolge — der Löschweg aus Idee 02
(`roots.go:80`, `RemoveRoot`) **ohne** das Löschen der Wurzelzeile und ohne das Beenden des Watchers:

1. Titel der Wurzel ohne Datei unter einer anderen Wurzel weich löschen
   (`SoftDeleteItemsOfRootWithoutOtherFiles`, `system/library.sql:73`), danach leere Staffeln und
   Serien.
2. Downloads der überlebenden Titel umhängen (`MoveDownloadsOffRoot`, `:86`).
3. `DELETE FROM media_files WHERE root_id = ?` — die Kaskade räumt die Downloads der weichen Titel
   ab, die Trigger setzen `items.available`.
4. `DELETE FROM ignored_files WHERE root_id = ?` (`00008_library_complete.sql:36` bis `:42`): die
   Einträge beziehen sich auf die alte Lesart des Ordners; heute fallen sie nur mit der Wurzelzeile.
5. `UPDATE library_roots SET content = ?, providers = ?`.
6. Nach `COMMIT`: den Wurzelzustand des Scanners leeren, wie `forgetRoot` (`roots.go:172` bis
   `:193`) es tut — `pathPending` und `orphans` dieser Wurzel halten `ParsedName`-Werte des **alten**
   Parsers — und wartende Aufträge wie `settleScans` (`:207`) abgleichen, **ohne** den Watcher zu
   stoppen; dann `scan.library` einreihen. Der Scan legt die Titel neu an; die Wiederanbindung
   binnen 30 Tagen greift nur bei **gleicher** Art — ein Film, der zur Folge wird, ist ein neuer
   Titel. Das steht im Bestätigungstext.

Darum verlangt der Inhaltswechsel `confirmPath` (5.2): Er löscht Downloads und trennt Fortschritt und
Merkliste vom Titel, genau wie `removeLibraryRoot`, und soll nicht leichter zu erreichen sein als
dieser. Der Dialog nennt die Zahl der betroffenen Titel (Obergrenze wie bei E17).

---

## 7. Oberfläche

Grundsatz wie in jedem Konzept dieses Projekts: **der Client prüft keine Regel** (`CLAUDE.md`
Regel 12). Die Regeln aus Abschnitt 2 stehen auch im Client, aber nur, um Unmögliches gar nicht erst
anzubieten (Entscheid vom 25. Oktober 2026 zu Spotlight: «Unmögliche Kombinationen werden gar nicht
erst angeboten», `04-entscheidungen.md`); der 422 bleibt der Riegel. Was angeboten wird, kommt aus
`listMetadataProviders`, nicht aus einer Liste im Client.

**Das Bauteil «Metadatenquellen»** (neu in `web/src/ui`, wiederverwendet in drei Formularen): eine
Liste der für die Inhaltsart erlaubten Anbieter, je Zeile ein Häkchen, der Name, ein Hinweis
(«kein Schlüssel hinterlegt», «liefert kein Deutsch», «erkennt Titel» beim ersten angekreuzten) und
zwei Knöpfe «nach oben» / «nach unten». **Keine Drag-and-drop-Pflicht** — Regel 8 verlangt
Tastaturbedienung, und Sortieren per Pfeiltasten auf fokussiertem Knopf ist der Weg, den
`HomeRowsPage` für die Reihenfolge der Startseiten-Reihen schon geht. Der erste angekreuzte Eintrag
ist der Erkenner; ein Eintrag, der nicht Erkenner sein darf (AniList), lässt sich nicht an die erste
Stelle schieben, und ist er der einzige angekreuzte, zeigt die Liste den Fehler, bevor der Server ihn
mit 422 bestätigt.

**Das Bauteil «Ordnerliste»** (neu in `web/src/ui`, F-13, wiederverwendet im Formular «Bibliothek
hinzufügen», im Einrichtungsassistenten und im Dialog «Ordner von X»): eine Liste **nummerierter
Karten**, je Karte ein Ordner mit Pfad und «Durchsuchen …», «Inhalt» und dem Bauteil
«Metadatenquellen». Jede Karte hat eine Kopfleiste mit Nummer («1», «2» in einer farbigen Kugel),
einer Kurzfassung («Serien · TMDB › TVDB») und «Entfernen»; die Karten sind durch einen Akzentbalken
links, einen kräftigeren Rand und grösseren Abstand klar getrennt (Marcos Rückmeldung an der ersten
Prototyp-Fassung: «Ordner 1 und Ordner 2 nur schwer erkennbar»). Unter der Liste steht
«+ Weiteren Ordner hinzufügen»; die neue Karte bekommt den Fokus im Pfadfeld. In einer
Video-Bibliothek schlägt die zweite Karte die **andere** Inhaltsart vor (erster Ordner Filme, also
Serien vorgewählt), damit «Dokus» mit Film- und Serienordner zwei Klicks sind. Die Liste hält ein
Minimum: eine Karte bei `video` und `audiobooks`, keine bei `podcasts` und im Dialog (dort beginnt sie
leer mit «+ Ordner hinzufügen»). Der Absendeknopf zählt mit («Bibliothek mit 2 Ordnern hinzufügen»,
«3 Ordner hinzufügen», «Bibliothek ohne Ordner hinzufügen» bei Podcasts). Überschneiden sich zwei
Pfade der Liste, zeigt das Bauteil den Fehler vor dem Absenden; der Server prüft dasselbe noch einmal
und antwortet mit `root_overlap` und dem Index des Verursachers (5.2). Entfernen einer Karte löscht
nur im Formular; angelegt wird erst beim Absenden, alles oder nichts.

| Seite / Datei | Stelle | Was geschieht |
|---|---|---|
| **Bibliothek anlegen**, `web/src/features/admin/LibrariesPage.tsx` | `AddLibraryForm` `:24` bis `:86`, `kind` `:27` | Die Artauswahl wird zu **«Video / Hörbücher / Podcasts»**. Darunter die **«Ordnerliste»** (F-13), die mit einer Karte beginnt: Pfad mit «Durchsuchen …» (`PathBrowserButton`, `:74`), **«Inhalt»** (bei Video: Filme / Serien; bei Hörbüchern und Podcasts fest und ausgeblendet) und **«Metadatenquellen»** (das Bauteil; bei Hörbüchern und Podcasts nur «Lokale Dateien», fest). Vorgabe bei Filmen `[TMDB]`, bei Serien `[TMDB, TVDB]`. Ein Absenden legt Bibliothek und alle Karten an. Standard-Altersstufe wie heute nur bei Audio. Der Podcast-Hinweis `library.podcastsHint` («braucht keinen Ordner») wird zu «Ordner optional; Feeds lassen sich auch ohne Ordner abonnieren» |
| **Drei Stellen, die «Podcast hat keinen Ordner» annehmen** | `LibrariesPage.tsx:283` (versteckt `FoldersAction`), `LibraryCard.tsx:138` (Text «keine Ordner» für Podcasts), `features/admin/queries.ts:52` (`canScan`) | alle drei fallen: Podcast-Bibliotheken haben Ordner **oder** Feeds, der Scan-Knopf gilt, sobald eines davon da ist |
| **Geteiltes Formular**, `web/src/features/shared/LibraryFields.tsx` | `KINDS` `:10`, `RadioGroup` `:76` bis `:82`, Pfad `:89` bis `:105`, `libraryRequest` `:23` bis `:36` | Trägt die Ordnerliste und baut `LibraryRequest.roots`. Das Formular ist mit dem **Einrichtungsassistenten** geteilt (`web/src/features/setup/SetupPage.tsx:127`, `:210`, `:277`); der Assistent bekommt damit dieselben Felder samt «+ Weiteren Ordner hinzufügen», ohne eigenen Code, und seine Fehlerzuordnung folgt den neuen Feldnamen `library.roots[0].path`. Der Vorgabename `t('library.kind.movies')` (`:277`) wird «Filme» aus einem neuen Schlüssel, weil `library.kind.movies` verschwindet. Im Assistenten ist `listMetadataProviders` nicht erreichbar (keine Sitzung); er zeigt die Vorgabeliste fest und ohne Hinweise |
| **Bibliothekskarte**, `web/src/features/admin/LibraryCard.tsx` | Chip `:57`, `Folders` `:128` bis `:171` | Der Art-Chip zeigt die Familie; bei mehreren Ordnern steht statt «2 Ordner» eine Zusammenfassung «1 Filme, 1 Serien». Bei genau einem Ordner wie heute Pfad, Dateisystem, Überwachung, neu plus Inhalt und die Liste als Kurzform «TVDB › TMDB › AniList» |
| **Ordner von X**, `web/src/features/admin/LibraryFoldersDialog.tsx` | Hinzufügen `:64`, Formular `:174` bis `:198`, Entfernen `:86` | Jede Zeile zeigt den Inhalt-Chip und die Liste als Kurzform und bekommt neben «Entfernen» ein **«Ändern»**. Das Hinzufügen-Formular heisst «Neue Ordner» und ist die **Ordnerliste** (F-13): leer bis zum ersten «+ Ordner hinzufügen», dann beliebig viele Karten, ein Knopf «3 Ordner hinzufügen» sendet `LibraryRootsRequest`. «Ändern» öffnet einen Dialog mit denselben Feldern; wechselt der Erkenner, erscheint der Hinweis aus 6.3; wechselt der Inhalt, erscheint die abgetippte Bestätigung (die `TypedConfirmation`-Variante von `ConfirmDialog` aus Schritt 13 der Idee 02) |
| **Podcasts**, `web/src/features/admin/PodcastsPage.tsx` | `:129` filtert `kind === 'podcasts'` | Bleibt die Seite für Feed-Abos. Podcasts aus Ordnern erscheinen in derselben Liste mit dem Chip «aus Ordner» und der Zahl lokaler Episoden; «Feed entfernen» ist bei `feedOrigin: file` ausgeblendet, denn der Ordner bestimmt den Feed |
| **Titelpflege**, `web/src/features/admin/AdminItemPage.tsx` | Match-Bereich `:357`, `:409` | Zeigt die Liste des Titels, je Textfeld die Herkunft («Beschreibung (de): TMDB», aus `fieldSources`, so wie Idee 01 es beschreibt, `:85`) und bei AniList die Id als Link; die manuelle Zuordnung bleibt TMDB/TVDB |
| **Bibliothekswahl in Profilen und Zugang**, `useLibraryOptions.ts:35` | liest nur `id`, `name` | **keine Änderung** |
| **Zuschauer** | — | **keine Änderung** (2.1) |

**Regel 8 — Tastaturtests.** Es entsteht keine neue Route, aber ein neues Bauteil mit Häkchen und
Sortierknöpfen, zwei Formulare wachsen darum, und ein Dialog kommt dazu; in der Wirkung ist «Ändern»
eine neue Seite. Darum: `web/e2e/admin-libraries.spec.ts` (Tab-Reihenfolge `:27`, Audio-Arten `:48`)
wird auf die neue Artauswahl, den Inhalt und beide Bauteile erweitert (Häkchen per Leertaste,
Reihenfolge per Pfeiltaste auf dem Sortierknopf, «+ Weiteren Ordner hinzufügen» per Enter mit Fokus
im neuen Pfadfeld, «Entfernen» einer Karte mit Fokus zurück auf dem Hinzufügen-Knopf, sichtbarer
Fokus); `web/e2e/admin-library-folders.spec.ts` bekommt einen Test für «Ändern» (Tab in die
Bestätigung, Enter, Escape, Zurück, keine Fokusfalle) und einen für zwei Ordner in einem Absenden;
`web/e2e/setup.spec.ts:115` («arrow keys switch the library kind») folgt der neuen Auswahl.

**Fixtures, vollständig gezählt**, weil sie den Schrittschnitt bestimmen (10): `web/e2e/global-setup.ts:55`,
`:61`, `:68`, `:125`, `:128`, `:139`, `:176` bis `:177`; `web/e2e/accounts.ts:197`;
`web/e2e/audio.ts:178`; `web/src/test/fixtures.ts:218`, `:268` bis `:269`, `:286`. Dazu eine gemischte
Bibliothek «Dokus» mit einem Film- und einem Serienordner, damit der Mischfall im e2e-Lauf vorkommt —
sie braucht **eigene** Testordner, weil Wurzeln sich nicht überlappen dürfen (`MOVIES_DIR`, `TV_DIR`
sind vergeben), und sie verschiebt Zählungen in anderen Specs.

**Regel 9 — Texte und Farben.** Neue Schlüssel in `web/src/locales/de.json` und `en.json`:
`library.kind.{video,audiobooks,podcasts}` (die vier alten `library.kind.*`-Schlüssel `:242` bis
`:247` verschwinden mit den Werten), `library.content.{movie,series,audiobook,podcast}`,
`library.providers.{title,identifies,moveUp,moveDown,noKey,noGerman,fillOnly}`,
`library.provider.{tmdb,tvdb,anilist,local}`,
`library.roots.{title,number,addFirst,addMore,remove,empty,emptyPodcasts,overlap,submit_one,submit_other,submitNone}`
(Ordnerliste, mit Pluralformen für den zählenden Knopf), `admin.libraries.folders.change.*` (Dialog,
Erkenner-Hinweis, Bestätigungstext), `admin.libraries.folders.new` («Neue Ordner»),
`admin.podcasts.fromFolder`, `admin.item.fieldSource`, `setup.library.content`,
`setup.library.providers`, die neuen Fehlercodes (`roots_required`, `root_overlap` mit Index) und die
Gruppenart `file_and_feed`. `pnpm -C web test locales` prüft die Symmetrie. Chips kommen aus
`web/src/ui`.

### 7.1 Klickbarer Prototyp

`docs/planning/mockups/07-bibliotheken-prototyp.html` ist eine einzelne HTML-Datei ohne Abhängigkeiten
(Doppelklick genügt): die Seite «Bibliotheken» mit Marcos sieben Bibliotheken aus Abschnitt 2, das
Formular «Bibliothek hinzufügen» mit Ordnerliste, der Dialog «Ordner von X» mit «Neue Ordner», der
Dialog «Ordner ändern» mit den drei Wirkungen aus 6.3 (nur Auffüller, Erkennerwechsel,
Inhaltswechsel mit abgetippter Bestätigung), das Bauteil «Metadatenquellen» mit Tastaturbedienung,
die Seite «Podcasts» mit dem Chip «aus Ordner» und eine «Titelpflege» mit Herkunft je Feld. Farben
und Abstände sind die Tokens aus `packages/design-tokens/tokens/`. Der Prototyp prüft die Regeln aus
Abschnitt 2 nur zur Anzeige, wie es der Client später auch tut; er ist **kein** Code für
`web/src` und wird nicht gebaut oder getestet. Stand: zweite Fassung vom 29. Oktober 2026 (F-13
eingearbeitet, Ordnerkarten getrennt). Dieselbe Fassung ist als privates Artefakt unter
`https://claude.ai/artifact/5ESS4fvgNMyoUqC35kyS4P` erreichbar.

---

## 8. Migration und Rückwärtsverträglichkeit

### 8.1 Was mit dem Bestand geschieht

Nach Migration 00023 ist jede bestehende Bibliothek, was sie vorher war: eine Filmbibliothek ist eine
Video-Bibliothek mit einem Filmordner (`[tmdb]`), eine Serienbibliothek eine Video-Bibliothek mit
einem Serienordner (`[tmdb, tvdb]`, die heutige Reihenfolge). Kein Scan wird nötig, kein Match wird
neu angestossen; der erste `metadata.fill` je Titel schreibt nur die Herkunft nach.

Auf **CT 190** (Stand 28. Oktober 2026, gelesen aus `/opt/slouchcat/deploy/config/slouchcat.db`):
Bibliothek 1 «Filme» (`movies`, Wurzel `/media/nas/Filme`, 1786 Dateien) und Bibliothek 2 «Serien»
(`series`, Wurzel `/media/nas/TestSerien`). Auf dem **Entwicklungsrechner** (`deploy/config/`):
Bibliothek 1 «Filme» (`movies`, `/test/Filme`). Beide Stände migrieren zu `video` mit je einem
Ordner.

Marcos Zielstruktur baut er danach **von Hand in der Oberfläche**, ohne Datenverlust an den
bestehenden Titeln: «Dokus» neu anlegen mit `/media/nas/Doku/Filme` (Filme) und `/media/nas/Doku/Serien`
(Serien) hinzufügen; «Kids» ebenso; «Anime» mit `[TMDB, AniList]` für Filme und `[TVDB, TMDB, AniList]`
für Serien; «Hörbücher» und «Podcast» als neue Bibliotheken. «Serien» bekommt `/media/nas/Serien`
dazu und verliert `/media/nas/TestSerien` über den bestehenden Löschweg. Für die Hörbücher und
Podcasts gilt die Lastrechnung aus 4.5; der erste Scan dieser zwei Bibliotheken ist ein Messlauf
(Schritt 6), kein Nebenbei.

### 8.2 Wo alte Clients scheitern

Die Web-App des Standes 1.2.0 sendet `LibraryRequest.path` und erwartet `kind: movies`; gegen einen
1.3.0-Server scheitert nur die Bibliotheksseite der Verwaltung mit 422 (fehlendes `roots`), nichts auf
der Zuschauerseite. Weil Web-App und Server aus demselben Image kommen, ist das nur für den Moment
zwischen Image-Wechsel und Neuladen relevant; der Service Worker lädt bei Versionswechsel neu.

### 8.3 Eingefrorene Datenbankstände

`testdata/db/` hält M1a bis M11 (`TestFrozenStatesMigrate`, `server/internal/store/frozen_test.go:23`).
Migration 00023 muss über **jeden** laufen; M6 und M7 sind die wichtigen, weil sie alle vier alten
Arten enthalten, darunter eine Podcast-Bibliothek ohne Wurzel (die vier Bibliotheken in
`testdata/db/M6.sql:737` bis `:740`, ihre Wurzeln `:215` bis `:217`). `PRAGMA foreign_key_check` muss
nach jedem Stand leer sein. Nach der Umsetzung kommt ein neuer Stand dazu (`test(store): freeze the …
database state`, Vorbild `5bc6ce7`) mit einer gemischten Video-Bibliothek, einem Serienordner mit
TVDB als Erkenner, einem Titel mit `anilist_id` und gefüllten `item_field_sources`, und einem
Ordner-Podcast mit Feed und Datei. Liegt die Benutzerverwaltung bis dahin in `main`, ist ihr
eingefrorener Stand (mit 00022) Teil der Reihe und 00023 läuft auch über ihn.

---

## 9. Sicherheit

### 9.1 Eine Feed-Adresse aus einer Datei bekommt nie den ungeschützten Client

Die `feedURL` in einer `metadata.json` ist Inhalt einer Datei auf dem NAS — nicht die Eingabe eines
Administrators. Der Server ruft sie trotzdem ab (F-6). **Der Netzwächter allein reicht dafür nicht**:
`netguard` lässt einen Feed auf einem lokalen Host absichtlich durch — «ein Feed, den der Haushalt
mit Absicht im LAN betreibt; seine Enclosures und Cover dürfen dann auch lokal sein»
(`server/internal/netguard/netguard.go:1` bis `:6`), und `Clients.For` gibt für einen solchen Feed den
**ungeschützten** Client heraus (`:96`, `LocalHost` `:38`). Diese Ausnahme ist für den Administrator
gedacht, der die Adresse selbst tippt. Für eine Adresse aus einer Datei darf sie nicht gelten, sonst
entschiede eine Datei auf dem NAS, dass der Server Adressen im Heimnetz abruft, durchreicht und
herunterlädt.

Darum `items.feed_origin` (4.3), und drei Regeln daran:

1. **Beim Übernehmen** (Scanner, 4.5): nur `http` und `https`; ein Host, für den `LocalHost` wahr
   ist, wird **nicht** gespeichert und erscheint als Pflegezeile «Feed-Adresse aus Datei nicht
   zulässig» mit Schema und Host (Regel 10: kein Pfad, keine Zugangsdaten im Log).
2. **Bei jedem Abruf** eines Podcasts mit `feed_origin = 'file'` — Feed (`podcast.go:413`),
   Enclosure-Proxy der Wiedergabe, Download (`offline/audio.go:66`), Cover — wird **immer** der
   geschützte Client benutzt, nie die LAN-Ausnahme. Der Feed könnte nach dem Übernehmen seine
   Enclosures ins Heimnetz umleiten; der Wächter prüft nach der DNS-Auflösung, also auch das.
3. **Ein Abo von Hand** (`feed_origin = 'admin'`) verhält sich wie heute. Der E2E-Mock-Feed auf
   `127.0.0.1` ist ein solches Abo und bleibt unberührt; der Ordner-Podcast der E2E-Suite trägt
   darum **keine** `feedURL` (er prüft den Ordnerweg), und das Übernehmen eines Feeds aus der Datei
   wird in Go mit `httptest` und der vorhandenen Testnaht von `netguard` geprüft, nicht im Browser.

### 9.2 Lesen nur über `paths`, mit Grenzen

`metadata.json` und Cover werden wie bei Büchern über das `*os.Root` der Wurzel gelesen, mit
`maxMetadataJSON` als Bremse; Symlinks nach aussen scheitern an `paths` (Regel 4). Ein Podcast-Ordner
mit 100 000 Dateien ist kein Sicherheits-, sondern ein Lastfall (4.5).

### 9.3 Zwei neue Aussenverbindungen, beide ohne Geheimnis

AniList braucht keinen Schlüssel, die Zuordnungsliste auch nicht; es gibt nichts zu redigieren. Beide
Hosts sind fest im Code, nicht konfigurierbar, damit kein Setting eine Adresse im Heimnetz nennen
kann. Antworten werden mit Grössenbremse gelesen (GraphQL 1 MB, Liste 64 MB); 429 und `Retry-After`
behandelt `ratelimit.Limiter` wie bei TMDB. Fällt ein Auffüller aus, bleiben die Werte des Erkenners;
der Job meldet den Fehler in den Fehlerring, nicht an den Zuschauer. Der TVDB-Schlüssel bleibt, wo er
ist (verschlüsselte `settings`, Redaktionsliste, Regel 10).

### 9.4 Keine Sichtbarkeitsregel ändert sich

`visible_items` liest weder `libraries.kind` noch `library_roots` noch `item_field_sources`; die
Rechte hängen an `library_id`, das Alter an der Titelart und an `effective_rating`, das nur Trigger
schreiben. Die Freigabe-Regel aus 4.4 (nie senken, strengster Anbieter gewinnt) wird im
Kinderprofil-Vertragstest als Negativtest festgehalten: «Anbieterreihenfolge senkt die Freigabe
nicht» (die Pflicht, die Idee 01 vorsah). Ein Profil, das «Dokus» nicht sehen darf, sieht weder die
Filme noch die Serien darin — eine Bibliothek, ein Recht (Regel 2; Nicht sichtbar heisst 404).

### 9.5 Der Inhaltswechsel ist so schwer wie das Entfernen

`updateLibraryRoot` mit Inhaltswechsel verlangt `confirmPath`, läuft unter der Scan-Sperre, in einer
Transaktion, und schreibt die `Warn`-Zeile und die `notify`-Meldung des Löschwegs (Idee 02, 6.7).
Nichts daran ist leichter zu erreichen als `removeLibraryRoot`.

---

## 10. Schnitt in Schritte

Reihenfolge nach Baubarkeit, und **so geschnitten, dass `task ci` nach jedem Commit grün sein kann**:
die alten Werte `movies` und `series` stecken in `ValidLibraryKind` (`model.go:982`), im Scanner, in
13 Server-Testdateien, in den `vistest`-Fixtures und in den e2e-Fixtures, und `task ci` fährt die
e2e-Suite mit (`Taskfile.yml:68` bis `:74`). Ein Schema ohne Scanner liesse eine migrierte
Serienbibliothek als Filmbibliothek lesen; ein neuer Client ohne angepasste Fixtures liesse
`LibraryFields.tsx:8` bis `:10` (`satisfies LibraryKind[]`) nicht mehr kompilieren. Darum sieben
Schritte, Schema und Scanner zusammen. **Beginn erst, wenn die Benutzerverwaltung in `main` ist**
(F-10).

### Schritt 0 — Entscheide und Papiere
`docs(planning,adr): record the library decisions and ADR 0015`

**Ziel:** Marcos Antworten F-1 bis F-12 stehen verbindlich im Entscheidungsprotokoll, der neue
Wortlaut zu B-1, die Umkehr der zwei verworfenen Alternativen aus ADR 0006 und die neue Grenze zu
Idee 01 ebenso.

**Berührte Dateien:** `docs/planning/04-entscheidungen.md` (neue **Frage 13** nach dem Muster von
Frage 12, dazu eine Zeile in «Weitere Entscheide», und ein Nachtrag zu Entscheid 7 / AN-15),
`docs/adr/0015-bibliotheken-ordner-scraper.md` (neu: Familie, Ordnerwerte, Regeln an der Liste,
Erkenner versus Auffüller, feldweises Auffüllen mit `item_field_sources`, Freigabe-Ausnahme, Fenster,
Podcast-Ordner und `feed_origin`, Inhaltswechsel; **Status «ersetzt teilweise ADR 0006»**, mit dem
Satz, was dort stehen bleibt), Ablösungshinweise in `docs/adr/0006-metadaten-anbieter-trailer.md`
(Kopf und die zwei Zeilen unter «Verworfene Alternativen») und `docs/adr/0007-audio-modell.md`
(Zeile «ohne Wurzel», offener Punkt), Kopfvermerk in `docs/ideas/01-metadaten-einstellungen.md`
(«Teil A — Anbieterliste, feldweises Auffüllen, Herkunft je Feld — übernommen durch Konzept 07 auf
Ordnerebene; Teil B bleibt») **und derselbe Vermerk als Kommentar in Issue #8**, Statuszeile in
`docs/ideas/README.md`, Kopf von `docs/ideas/02-ordner-browser.md` (Schritte 12 bis 14 sind gebaut),
dieses Papier.

**DoD:** `git diff --stat docs/spec/` ist leer. Frage 13 trägt alle Antworten mit Datum. ADR 0015
nennt B-1, B-3, H-1, H-2, AN-2, AN-15, C-5, K-3 und die zwei umgekehrten Alternativen wörtlich. Der
Kommentar in Issue #8 ist gesetzt (`gh issue comment 8`). `task ci` grün.

### Schritt 1 — Schema, Modell, Scanner und Fixtures
`feat(store,library): content and provider list per root, scanned per root`

**Ziel:** Die Datenbank kennt Familie, Inhaltsart, Liste und Herkunftstabelle, **und der Server liest
sie schon**: eine Video-Bibliothek mit Film- und Serienordner scannt richtig. Der Vertrag ändert sich
noch nicht; die Handler übersetzen die alten Vertragswerte (`movies` → `video` + Wurzel
`movie`/`[tmdb]`, `series` → `video` + Wurzel `series`/`[tmdb, tvdb]`) für genau einen Schritt, damit
die Web-App und die e2e-Suite unverändert laufen. `metadata.match` liest den Erkenner schon aus der
Liste, verhält sich mit den Vorgabelisten aber wie heute; `metadata.fill` kommt in Schritt 4.

**Berührte Dateien:** `server/migrations/00023_library_roots_providers.sql` (neu, Plan 4.6),
`server/queries/app/libraries.sql` (`InsertLibrary`, `InsertLibraryRoot`, `ListLibraryRoots`, neue
`UpdateLibraryRoot`, `UpdateItemsProvidersByRoot`), `server/queries/system/items.sql`
(`InsertItem`), `system/audiobooks.sql` (`InsertAudiobook`), `system/podcasts.sql` (`PodcastInsert`,
`PodcastInsertEpisode`, `PodcastListDue`, `PodcastListIDsByLibrary`), `system/library.sql`
(`DeleteMediaFilesOfRoot`, `DeleteIgnoredFilesOfRoot`), `system/metadata.sql` (`item_field_sources`
lesen und schreiben), `server/internal/model/model.go` (`:975` bis `:991`: `LibraryVideo`,
`RootContent*`, `Provider*`, `ValidRootProviders`; `LibraryRequest` `:994` bekommt `Roots []LibraryRootRequest`;
`LibraryRoot` `:1017` die zwei Felder), der Scanner nach 6.1 (`scanner.go`, `write.go`, `index.go`
mit einem Index je Titelart, `pathscan.go`), `library/service.go` (`Create` legt alle Wurzeln in
einer Transaktion an, F-13), `roots.go` (`AddRoot` wird `AddRoots` mit Liste, Überlappungsprüfung
auch innerhalb der Liste, ein Scan am Ende; neu `UpdateRoot` mit den drei Wegen aus 6.3),
`admin/admin.go` (`validateLibrary`, `ScanLibrary`),
`metadata/match.go` (`:78`, `:187`, `:315`: Erkenner aus der Liste, TVDB zuerst mit
`/find/{tvdb_id}` für die `tmdb_id`), die Go-Fixtures aus 6.1 (13 Testdateien, `vistest`,
`cmd/apicheck/fixtures.go`, `scanbench.go`), danach `task gen`.

**DoD:** `task ci` grün. `go test ./internal/store -run TestFrozenStatesMigrate` grün über alle
Stände, `foreign_key_check` leer; der Wächter aus 4.6 Punkt 5 bricht einen präparierten Stand mit
falschem Paar ab (Test). M6-Stand nach Migration: vier Bibliotheken `video`, `video`, `audiobooks`,
`podcasts`, Wurzeln `movie/["tmdb"]`, `series/["tmdb","tvdb"]`, `audiobook/["local"]`, `items` der
Hörbuchbibliothek tragen `["local"]`, der Feed-Podcast `["local"]` und `feed_origin = 'admin'`.
`go test ./internal/library/`: eine gemischte Bibliothek mit einem Film und einer Serie, die
**dieselbe numerische `tmdb_id`** tragen, ergibt zwei Titel verschiedener Art; ein Serienordner
verwirft Dateien ohne Folgenmarke, der Filmordner derselben Bibliothek nicht; nach Inhaltswechsel
`movie → series` gibt es keinen Titel mit `deleted_at IS NULL` und null Dateien, der Download des
gegangenen Films trägt einen Tombstone, `ignored_files` der Wurzel sind leer, `pathPending` und
`orphans` der Wurzel sind leer, der Watcher läuft weiter, der Scan legt Folgen neu an; Listenänderung
reiht je Titel der Wurzel genau einen Job ein und löscht nichts. `go test ./internal/metadata/`: eine
Serie mit `["tvdb","tmdb"]` wird über TVDB erkannt, trägt danach `tvdb_id` **und** `tmdb_id` (aus
`/find`), und ihre Staffeln folgen der TVDB-Zählung; dieselbe Serie mit `["tmdb","tvdb"]` verhält
sich wie heute. `slouchcat scanbench` ohne Regression gegen `docs/perf/M3.md`. `SELECT sql FROM
sqlite_master WHERE sql LIKE '%libraries%'` auf dem M11-Stand nennt keine Sicht und keinen Trigger
(Beleg für 4.6 Punkt 4).

### Schritt 2 — Vertrag 1.3.0 und die Fixtures beider Seiten
`feat(api): library families, root content and provider list, updateLibraryRoot, listMetadataProviders`

**Ziel:** Der Vertrag beschreibt das Zielbild, die Übersetzung aus Schritt 1 fällt, und Client-Typen
wie Fixtures folgen **im selben Commit**, sonst ist `task ci` rot.

**Berührte Dateien:** `api/openapi.yaml` (5.1, 5.2, `info.version: 1.3.0`),
`server/internal/api/admin_handlers.go` (`:146`, die Wurzel-Handler, `listMetadataProviders`),
`contract_access_test.go` (Einträge), `cmd/apicheck/fixtures.go`, `packages/api-client` über
`task gen`; auf der Web-Seite das **Minimum**, damit Typen und e2e-Suite wieder stimmen:
`web/src/features/shared/LibraryFields.tsx` (`KINDS`, `libraryRequest` baut `roots` mit einem
Ordner und der Vorgabeliste), die Fixtures aus Abschnitt 7 (`global-setup.ts`, `accounts.ts`, `audio.ts`,
`web/src/test/fixtures.ts`), `setup.spec.ts:115`. Die Oberfläche selbst (Bauteil, Chips, Dialoge)
kommt in Schritt 5; bis dahin sendet das Formular feste Vorgabelisten.

**DoD:** `task ci` grün (`gen:check` ohne Drift). Vertragstests: 422 `content_not_allowed` für
`audiobook` in einer Video-Bibliothek; `providers_not_allowed` für `["anilist"]` allein, für
`["tvdb"]` an einem Filmordner, für eine Liste mit Doppelten und für eine leere Liste; 200 für
`["tvdb","tmdb","anilist"]` an einem Serienordner; `confirm_mismatch` und 409 `scan_running` beim
Inhaltswechsel; 200 beim Listenwechsel ohne Bestätigung; `searchMetadata` mit `provider: anilist`
ergibt 422 `provider_not_searchable`; `listMetadataProviders` nennt TVDB als `configured: false`
ohne Schlüssel. **F-13:** `createLibrary` mit zwei Ordnern (Film und Serie) legt beide an;
`createLibrary` mit `roots: []` bei `video` ergibt 422 `roots_required`, bei `podcasts` 201;
überschneiden sich `roots[0]` und `roots[1]`, kommt 422 `root_overlap` mit Feld
`library.roots[1].path` und **keine** Zeile in `libraries` oder `library_roots`; `addLibraryRoot`
mit drei Ordnern, deren dritter überlappt, lässt die ersten zwei **nicht** zurück. Der Schema-Test
findet an jeder neuen Operation `x-visibility`. Release-Notes-Entwurf mit dem Bruch (`LibraryKind`,
`LibraryRequest.roots`, `addLibraryRoot` mit Liste, `MetadataProvider`, `DuplicateGroup.reason`).

### Schritt 3 — Podcast-Ordner
`feat(library,metadata,playback): scan podcast folders and adopt their feeds`

**Ziel:** `/media/nas/Audiobookshelf/Podcast` wird eine Podcast-Bibliothek mit Dateien **und** Feeds.

**Berührte Dateien:** `server/internal/library/podcast_folder.go` (neu: Ordner, `metadata.json`,
Cover, Episoden, Übernahme über `PodcastFindByFeed`), `podcast_folder_test.go`, `audiobook.go`
(`absMeta` um Podcast-Felder), `probe.go` (`finishPodcastEpisode`: Tags, RFC-2822-Datum,
Zusammenführen; keine Keyframe-Jobs für Audio), `metadata/podcast.go` (`:214` `feed_origin`,
Zusammenführen bei `:472` ff., `RefreshLibrary` `:327`, geschützter Client für `file`),
`netguard` (Wahl des Clients nach `feed_origin`), `queries/system/podcasts.sql`,
`queries/system/items.sql` und `system/library.sql` (Sweep-Abfragen mit `podcast_episode`, neuer
Podcast-Sweep), `queries/scoped/podcasts.sql`, `internal/playback/sessions.go:116` (Episode mit Datei
spielt als Datei), `internal/offline/audio.go:66` (Client), `admin/maintenance.go` (Gruppe
`file_and_feed`, Pflegezeilen «zwei Ordner, ein Feed», «Feed-Adresse aus Datei nicht zulässig»),
`library/service.go` (Hook nur für bestehende Podcasts), Testdaten: ein Mini-Podcast-Ordner mit
`metadata.json`, zwei MP3 mit Tags und dem Mock-Feed der CI, der eine der zwei Episoden kennt.

**DoD:** `go test ./internal/library/ ./internal/metadata/ ./internal/playback/ ./internal/offline/`:
Ordner → Podcast mit `feed_url` und `feed_origin = 'file'`; ein zweiter Ordner mit demselben Feed
legt keinen zweiten Podcast an und scheitert nicht; der Feed-Abruf hängt die bekannte Episode an die
Datei (Datum auf die Minute gleich, eindeutig) statt sie zu doppeln; zwei Dateien mit gleichem Titel
werden **nicht** zusammengeführt; ein Tag-Titel gleich dem Podcast-Titel wird nicht zum
Episodentitel; die unbekannte Feed-Episode entsteht als Enclosure-Episode; die Datei-Episode spielt
`direct` aus der Datei und lädt aus der Datei; eine `feedURL` auf einem lokalen Host wird nicht
gespeichert und ergibt die Pflegezeile; ein Podcast mit `feed_origin = 'file'` ruft Feed, Enclosure
und Cover nur über den geschützten Client ab (Test mit der `netguard`-Testnaht); ein Ordner-Podcast
ohne Feed steht nicht in `PodcastListDue`; eine gelöschte Datei-Episode ohne Enclosure wird vom
Sweep weich gelöscht, eine mit Enclosure bleibt als Feed-Episode; `removeLibraryRoot` lässt den
Feed-Podcast als Abo mit `feed_origin = 'admin'` stehen; der Erst-Scan feuert keinen
Neue-Folgen-Hook, ein späterer Scan mit neuer Datei schon; `explicit: true` ergibt 18 mit Quelle
`tags`; keine `probe.keyframes`-Zeile für `podcast_episode` und `audiobook`.
Kinderprofil-Vertragstest: eine explizite Datei-Episode fehlt in `listPodcastEpisodes` unter 18.

### Schritt 4 — Auffüllen in Reihenfolge und AniList
`feat(metadata): fill fields provider by provider, with provenance and AniList`

**Ziel:** Ein Titel mit mehr als einem Anbieter in der Liste trägt je Feld den Wert des ersten, der
einen hat, die Herkunft steht in `item_field_sources`, und AniList ist der dritte Anbieter.

**Berührte Dateien:** `server/internal/metadata/fill.go` (Job `metadata.fill`, Regel aus 4.4,
Staffel-Läufe), `anilist.go` (Client über `ratelimit.Limiter`), `anime_lists.go` (Zuordnungsliste,
Cache, Streaming-Parser, 64 MB, Karten je Art), `match.go` (Einreihen nach jedem Match,
Konvergenzabfrage in `QueuePendingMetadata`), `images.go` (Poster und Backdrop aus der Herkunft),
`queries/system/metadata.sql` (`item_field_sources`), Tests mit `httptest`-Servern für alle drei
Hosts, `config.go` (`SLOUCHCAT_ANILIST`), Dashboard-Zähler, `cmd/apicheck`.

**DoD:** **Vorher** die Prüfpunkte aus 4.4 (Rohadresse, Feldnamen, Lizenz der Liste; TMDB-Ids bei
Filmen) mit Beleg in der Rückmeldung. `go test ./internal/metadata/`: `["tmdb","tvdb"]` füllt eine
leere TMDB-Beschreibung aus TVDB und lässt eine vorhandene stehen, Herkunft je Feld stimmt;
`["tvdb","tmdb"]` ersetzt eine TVDB-Beschreibung **nicht** durch TMDB; ein Wechsel der Liste von
`["tvdb","tmdb"]` auf `["tmdb","tvdb"]` ersetzt sie beim nächsten Lauf (Idempotenz-Test: zweimal
laufen lassen, gleiches Ergebnis); ein gesperrtes Feld bleibt; `own_rating` wird nie gesenkt
(Negativtest, auch im Kinderprofil-Vertragstest); TVDB-Id → AniList-Id über die Liste (Staffel 1),
TMDB-Film-Id ebenso, und ein Film und eine Serie mit derselben Zahl landen bei verschiedenen
AniList-Einträgen; `isAdult` hebt auf 18; **nach einem erneuten Match steht das Auffüllen wieder**
(Test: Match, Fill, manuelle Zuordnung, Fill → Felder gleich); 429 mit `Retry-After` pausiert den
Limiter; Liste grösser als 64 MB wird abgebrochen; kein Treffer schreibt `anilist_id = 0`; ein
Anbieter ohne Schlüssel wird übersprungen und gemeldet. Messung: 111 Anime-Titel vom NAS mit
`[TVDB, TMDB, AniList]` bzw. `[TMDB, AniList]`, Dauer und Anfragen je Host.

### Schritt 5 — Oberfläche
`feat(web): library families, content and ordered metadata sources per folder`

**Ziel:** Marco kann seine Tabelle in der Verwaltung anlegen und je Ordner die Scraper-Liste
ankreuzen und sortieren, ohne eine Zeile Konfiguration.

**Berührte Dateien:** Abschnitt 7 vollständig: die Bauteile «Metadatenquellen» und «Ordnerliste»
in `web/src/ui` (je + Story + Unit-Test), `LibrariesPage.tsx` (samt `:283`), `LibraryFields.tsx`, `LibraryCard.tsx` (samt
`:138`), `queries.ts:52` (+ Abfrage `listMetadataProviders`), `LibraryFoldersDialog.tsx` (+ Dialog
«Ordner ändern»), `PodcastsPage.tsx`, `AdminItemPage.tsx` (Herkunft je Feld), `SetupPage.tsx`,
Locales, Storybook-Stories, Unit-Tests, `web/e2e/admin-libraries.spec.ts`,
`admin-library-folders.spec.ts` (neuer Test «Ändern»), die gemischte e2e-Bibliothek «Dokus» mit
eigenen Testordnern.

**DoD:** `task web:lint`, `task web:test`, `task web:storybook:test`, `task web:e2e` grün; die
Tastaturtests für beide Bauteile und für «Ändern» nach Regel 8; der e2e-Lauf legt die gemischte
Bibliothek «Dokus» mit **zwei Ordnern in einem Absenden** an (F-13); `pnpm -C web test locales` grün;
`grep -rn "'movies'" web/src --include=*.tsx` ist leer und `grep -rn "'series'"` nennt nur Titelarten
und Suchfilter (`searchParams.ts:7`).

### Schritt 6 — Abnahme, Messung, Doku
`docs(dev,perf): library folders with content and provider lists, acceptance on CT 190`

**Ziel:** Der Stand ist belegt, gemessen und beschrieben.

**Berührte Dateien:** `docs/dev/CURRENT_STATUS.md` (neuer Abschnitt), `docs/perf/bibliotheken.md`
(Erst-Scan Hörbücher und Podcast-Ordner auf CT 190: Dauer, ffprobe-Starts, Jobs, NFS-Latenz,
Speicher; Anime-Lauf mit drei Anbietern), `docs/INSTALLATION.md:124` und `:65` bis `:73` (Assistent,
Ordner), `deploy/README.md`, `testdata/db/` (neuer eingefrorener Stand, 8.3), `docs/ideas/README.md`.

**DoD:** `task ci` grün. Abnahme durch Marco auf CT 190 nach 10.8: Zielstruktur angelegt, «Dokus»
zeigt Filme und Serien, «Anime Serien» mit TVDB zuerst zeigt die Staffeln nach TVDB und japanische
Originaltitel, Podcast-Ordner zeigt Episoden mit Datei und neue aus dem Feed, ein Inhaltswechsel
verlangt den getippten Pfad, die Admin-Titelseite zeigt je Feld die Herkunft. Messwerte in
`docs/perf/`.

### 10.8 Der Weg jedes Schritts: Zweig, Commit, Test, Push, Pull Request

Derselbe Weg wie in `06-benutzerverwaltung.md` 10.8, mit **zwei Umgebungen**:

| Umgebung | Wo | Was dort läuft | Wofür |
|---|---|---|---|
| **Entwicklung** | dieser Rechner (Linux Mint, Repo `/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat`) | `task dev`, `task ci`, das selbst gebaute Image `slouchcat:dev`, Marcos ständige Instanz auf Port 8096 | Code, Tests, erster Blick in der Oberfläche; **die echten NAS-Ordner liegen hier unter `/mnt/nerdnas/medien/…` lesbar** (Doku, KinderFilme, MangaSerien, Audiobookshelf), gut für Scannertests ohne CT |
| **Test** | Proxmox-CT 190, `192.168.1.43`, Port 8096 (`docs/dev/TESTSERVER.md`) | Stack aus `/opt/slouchcat/deploy`, 2 vCPU, 4 GB, NAS unter `/media/nas` | Marcos Abnahme an der echten Sammlung, am Handy und am Fernseher; der Messlauf aus Schritt 6 |

Stationen, nicht Planschritte:

**1. Zweig.** `git switch -c claude/bibliotheken main`, **nachdem** `claude/benutzerverwaltung`
gemergt ist. Der Arbeitsbaum ist geteilt: vor jeder Git-Aktion `git branch --show-current` und
`git status --short --branch`; nichts verwerfen, das nicht aus dieser Arbeit stammt. (Dieses Papier
selbst liegt im Worktree `.claude/worktrees/konzept-bibliotheken` auf `claude/konzept-bibliotheken`,
weil der Hauptbaum zur Zeit fremde, nicht übernommene Änderungen trägt.)

**2. Entwickeln.** Quelle ändern, nie den generierten Code; nach `api/openapi.yaml`,
`server/queries/`, `server/migrations/` immer `task gen` (Regel 1).

**3. Commit je Planschritt.** Titel Englisch mit Bereich, Body Englisch mit dem Warum; Erläuterung
für Marco Deutsch.

**4. Auf diesem Rechner testen.** `task ci` grün. Die drei Fallen gelten weiter: Marcos Instanz auf
8096 bleibt unberührt; die e2e-Ports 8097 und 8089 bis 8095 sind rechnerweit (`ss -ltn | grep :809`,
sonst `SLOUCHCAT_E2E_PORT=8197`); Docker nur über `sg docker -c '…'`.

**5. Push.** `git push -u origin claude/bibliotheken`. Nie direkt nach `main`.

**6. Abnahme auf CT 190.** Vor dem Merge, weil die Bibliotheksseite und der erste Scan der zwei
Audio-Ordner nur am echten Bestand etwas zeigen:

```bash
sg docker -c 'task deploy:build'
docker tag slouchcat:dev ghcr.io/slouchcat/slouchcat:bibliotheken
docker save ghcr.io/slouchcat/slouchcat:bibliotheken | ssh root@192.168.1.43 'docker load'
ssh root@192.168.1.43 'cp /opt/slouchcat/deploy/.env /opt/slouchcat/deploy/.env.bak-vor-bibliotheken'
ssh root@192.168.1.43 "sed -i 's/^SLOUCHCAT_VERSION=.*/SLOUCHCAT_VERSION=bibliotheken/' /opt/slouchcat/deploy/.env"
ssh root@192.168.1.43 'cd /opt/slouchcat/deploy && docker compose up -d --wait --wait-timeout 240'
ssh root@192.168.1.43 'curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8096/healthz'
```

**Die Datenbank dreht sich nicht zurück:** Migration 00023 baut `libraries` um. Der Server sichert
vorher nach `/config/backups/` (letzte fünf); der Dateiname gehört in die Rückmeldung. Auf CT 190
steht `SLOUCHCAT_BROWSE_ROOTS=/` (Marcos Entscheid vom 25. Oktober 2026); der Ordner-Browser zeigt
dort den ganzen Container, das ist bekannt und gewollt. Prüfen heisst `/healthz`.

Marco nimmt ab: Zielstruktur anlegen, Mischbibliothek, Scraper-Liste sortieren, Anime-Serien mit TVDB
zuerst, Podcast-Ordner, Inhaltswechsel mit Bestätigung, Bibliotheksrechte eines Kinderprofils auf
«Kids».

**7. Pull Request, CI, Merge.** PR gegen `main`, benennt den Bruch auf 1.3.0. `bind_pr`, CI nicht
pollen, `--auto` ist kein Tor. Merge erst mit grüner CI **und** Marcos Wort. Danach `main`
nachziehen, Image aus `main` bauen, CT 190 umstellen, `docs/dev/CURRENT_STATUS.md` nachführen.

---

## 11. Fragen an Marco

Format wie `docs/planning/04-entscheidungen.md`. Beantwortet am 28. Oktober 2026 in drei Runden,
F-13 am 29. Oktober 2026 am Prototyp; offen ist nur F-10. F-9 ist durch F-11 erledigt.

| Nr. | Frist | Thema | Empfehlung | Antwort | Datum |
|-----|-------|-------|------------|---------|-------|
| **F-1** | vor Schritt 0 | **Podcast-Ordner.** Die Tabelle nennt `/Podcast`; heute ist eine Podcast-Bibliothek reine Feed-Sache ohne Ordner (ADR 0007), lokale Ordner sind dort ein Nicht-Ziel | Feed-only belassen, Zeile streichen | **Lokale Podcast-Ordner scannen** | 2026-10-28 |
| **F-2** | vor Schritt 0 | **Was ist der Scraper «Anime»** bei nur TMDB und TVDB? | Nur eine Wahl am Ordner, Verhalten im Scraper-Konzept | **Neuer Anbieter jetzt** | 2026-10-28 |
| **F-3** | vor Schritt 1 | **Darf eine Bibliothek Video und Audio mischen?** | Ja, alles mischbar | **Nur Video mischbar**; Hörbücher und Podcasts bleiben eigene Bibliotheken | 2026-10-28 |
| **F-4** | vor Schritt 2 | **Inhaltsart oder Quelle eines Ordners nachträglich ändern?** | Ja, mit Neuerkennung; Titel der alten Art weich gelöscht, Fortschritt bleibt am alten Titel | **Ja, mit Neuerkennung** | 2026-10-28 |
| **F-5** | vor Schritt 4 | **Welcher Anime-Anbieter?** Ordner tragen TVDB/TMDB-Ids; Staffeln und Folgen bleiben dort | AniList (ohne Schlüssel, GraphQL, Bilder, mehrsprachige Titel, Zuordnung über öffentliche Liste) | **AniList** | 2026-10-28 |
| **F-6** | vor Schritt 3 | **Podcast-Ordner und Feeds:** `metadata.json` trägt `feedURL` | Zusammen: Ordner scannen, Feed mitabonnieren; Abos ohne Ordner bleiben | **Ordner und Feeds zusammen** | 2026-10-28 |
| **F-7** | vor Schritt 0 | **Was ist der «Hörbuch Scraper»?** Es gibt keinen externen Hörbuch-Anbieter | Der heutige lokale Erkenner; Audnexus im Scraper-Konzept | **Der heutige lokale Erkenner** | 2026-10-28 |
| **F-8** | vor Schritt 5 | **Wortwahl** für die Wahl am Ordner | «Scraper» / «scraper» | **«Metadatenquelle» / «metadata source»** | 2026-10-28 |
| **F-9** | — | Was darf AniList übersteuern? | Füllen, nicht ersetzen | **Erledigt durch F-11:** die Reihenfolge je Ordner entscheidet; wer AniList vorne setzt, bekommt AniList-Texte | 2026-10-28 |
| **F-10** | vor Schritt 0 | **Reihenfolge:** erst die Benutzerverwaltung in `main`, dann dieser Zweig? Beide bauen Tabellen um (00022 `accounts`, 00023 `libraries`), beide heben den Vertrag brechend an | **Ja, Benutzerverwaltung zuerst.** Zwei parallele Tabellenumbauten auf derselben Datenbank von CT 190 sind ein Risiko ohne Nutzen; und die Nummern in diesem Papier (00023, 0015, 1.3.0) setzen es voraus | | |
| **F-11** | vor Schritt 1 | **Wie sieht die Scraper-Wahl je Ordner aus?** Hauptscraper plus Ergänzer (ein Erkenner, Häkchen für Auffüller) / genau einer / Liste mit Reihenfolge wie in Emby | Hauptscraper plus Ergänzer — kleiner, Idee 01 bleibt ganz | **Liste mit Reihenfolge wie in Emby.** Der erste erkennt, die weiteren füllen feldweise auf; Herkunft je Feld und Anbieterliste ziehen aus Idee 01 hierher | 2026-10-28 |
| **F-12** | vor Schritt 1 | **TVDB als Hauptscraper für Serienordner wählbar?** Heute nur Rückfall (ADR 0006); TVDB braucht den kostenpflichtigen Schlüssel | Ja, wählbar; bei TVDB zuerst gilt die TVDB-Staffelzählung, TMDB füllt auf; ändert ADR 0006 | **Ja, wählbar** | 2026-10-28 |
| **F-13** | vor Schritt 1 | **Mehrere Ordner in einem Zug?** Der erste Prototyp (7.1) legte beim Anlegen genau einen Ordner an, weitere erst über «Ordner von X»; Marco: «nicht intuitiv gelöst» | Ordnerliste im Formular, ein Absenden für alle; `LibraryRequest.roots[]`, `addLibraryRoot` mit Liste, Server alles oder nichts | **Ja, so aufnehmen** (zweite Prototyp-Fassung mit nummerierten Ordnerkarten) | 2026-10-29 |

---

## 12. Risiken

### 12.1 Der Titelindex einer gemischten Bibliothek

Der Scanner bindet Dateien an bestehende Titel über einen bibliotheksweiten Index mit **einer** Karte
je Schlüssel (`itemIndex`, `index.go:23` bis `:38`). Morgen stehen Filme und Serien in derselben
Bibliothek, und TMDB vergibt Film- und Serien-Ids aus getrennten Zahlenräumen; dieselbe Zahl kann
beides sein. Ein Index je Titelart (6.1) ist darum keine Verfeinerung, sondern die Bedingung dafür,
dass «Dokus» überhaupt funktioniert. Der Test in Schritt 1 nagelt genau den Kollisionsfall fest.

### 12.2 Der Umfang ist mit Fassung 3 gewachsen

Die erste Fassung hatte vier feste Quellen und eine Anreicherung; Fassung 3 hat eine Liste je Ordner,
feldweises Auffüllen, eine Herkunftstabelle, TVDB als Erkenner und AniList. Idee 01 schätzte den Teil
«feldweise Zusammenführung, Herkunft, Freigabe-Ausnahme» allein auf 3 bis 4 Tage und «TVDB frei
platzierbar» auf 3 bis 4 Tage (`01-metadaten-einstellungen.md:250`, `:248`); beides kommt zu diesem
Vorhaben dazu. Was nicht dazukommt und den Umfang begrenzt: keine Sprachen, keine neuen Bildarten,
keine Bildwahl je Titel, keine Rohzertifikate — die Freigabe-Ausnahme ist hier nur «nie senken», nicht
das Ländermodell aus Idee 01. Schritt 4 ist der grösste Schritt; läuft er aus dem Ruder, lässt er
sich an der Grenze «Auffüllen mit TMDB/TVDB» (zuerst) und «AniList» (danach) in zwei Commits teilen,
ohne dass `task ci` dazwischen rot würde.

### 12.3 TVDB zuerst ist eine andere Zählung, kein anderer Titel

Wer an einem bestehenden Serienordner den Erkenner von TMDB auf TVDB stellt, bekommt neue Staffel-
und Folgentexte, aber **dieselben** Folgen-Items (6.3). Bei Serien, in denen beide Anbieter gleich
zählen, ändert sich nichts Sichtbares; bei Anime mit abweichender Zählung ändert sich genau das, was
Marco will. Das Risiko liegt woanders: TVDB liefert deutsche Texte nur, wo die Gemeinschaft sie
pflegt (ADR 0006 nannte das als Grund gegen TVDB zuerst). Mit `[TVDB, TMDB]` füllt TMDB die
deutschen Lücken — aber nur dort, wo TVDB **leer** ist, nicht dort, wo TVDB einen schlechteren Text
hat. Das ist die Regel «der erste gewinnt», und sie ist Marcos Entscheid.

### 12.4 Ordner- und Feed-Episoden finden nicht zusammen

Audiobookshelf benennt Dateien nach dem Episodentitel, der Feed liefert GUIDs und andere Dateinamen,
und 11 von 239 Dateien eines Podcasts tragen als Tag-Titel den Podcast-Namen. Die Regel aus 4.5
(Datum auf die Minute, sonst Titel; nur bei eindeutigem Paar; nie der Podcast-Titel) nimmt dafür in
Kauf, dass ein Teil der Episoden **doppelt** bleibt — sichtbar in `listDuplicates` als
`file_and_feed`, nie falsch verheiratet. Die Quote ist in Schritt 3 am echten Ordner zu messen.

### 12.5 Last durch die Audio-Ordner

26 031 Dateien im Podcast-Ordner und 62 Autorenordner mit M4B über NFS mit hoher Latenz. Der Preis
ist nicht das Lesen der Dateien (die Sonde liest keine Pakete, die Keyframe-Sonde überspringt Audio),
sondern rund 26 000 ffprobe-Starts über das Netz. Das sind auf CT 190 Stunden, und die Warteschlange
hat keinen Pause-Schalter; `SLOUCHCAT_WORKERS` bleibt der einzige Hebel. Der Erst-Scan ist darum ein
Messlauf mit eigener Datei in `docs/perf/`.

### 12.6 AniList, die Zuordnungsliste und das Fenster

Unsicherheiten, in 4.4 benannt: das Rate-Limit (90 laut Dokumentation, zeitweise weniger), die
Rohadresse, die Feldnamen und die Lizenz der Liste, die TMDB-Abdeckung bei Filmen. Dazu: die Liste
verschmilzt über `anidb_id`, und eine TVDB-Serie mit fünf Staffeln hat fünf AniList-Einträge — die
Wahl «Staffel 1» ist für Titel und Originaltitel richtig; Staffeltexte von AniList gibt es nicht. Und
das Fenster zwischen Match und Auffüllen (4.4): eine Freigabe, die nur ein Auffüller kennt, fehlt in
diesem Fenster. ADR 0015 benennt das als Restrisiko.

### 12.7 Was dieses Papier nicht zählen konnte

Die Stellen in `web/src`, die `'movies'` oder `'series'` als **Bibliotheksart** lesen, sind in
Abschnitt 7 vollständig genannt; aber `'series'` ist auch eine **Titelart** und ein **Suchfilter**
(`searchParams.ts:7`); ein mechanisches Ersetzen wäre falsch. Je Stelle ist zu entscheiden, wie in
der Benutzerverwaltung bei `owner`.

### 12.8 Ein alter Vorbehalt bleibt

`SLOUCHCAT_BROWSE_ROOTS=/` auf CT 190 hebt E14 auf: ein Podcast-Ordner liesse sich dort auf
`/config` anlegen, und der Scanner läse `slouchcat.db` als übergangene Datei. Das ist seit dem
25. Oktober 2026 bekannt und entschieden; dieses Konzept verschärft es nicht und entschärft es nicht.

---

## 13. Review-Protokoll (28. Oktober 2026)

Der OMC-Kritiker hat die erste Fassung (`d4eeee2`) am Code geprüft, über 60 Zeilenangaben
nachgeschlagen und 22 Befunde geliefert. Alle sind eingearbeitet (Fassung 2, `6fcc113`) und in
Fassung 3 erhalten geblieben:

| Befund | Schwere | Eingearbeitet in |
|---|---|---|
| Schrittschnitt nicht grün je Schritt (Scanner, Fixtures, Client-Typen hängen am Schema) | MAJOR | Abschnitt 10, Schritte 1 und 2 zusammengelegt |
| `netguard` lässt LAN-Feeds durch; «wird nicht gespeichert» hatte keinen Mechanismus | MAJOR | 4.3 `feed_origin`, 9.1 |
| `items_podcast_feed` kollidiert bei gleicher Feed-Adresse | MAJOR | 4.5 Übernahme über `PodcastFindByFeed` |
| Ordner-Podcasts ohne Feed würden abgefragt; Vorrang Feed/`metadata.json` offen | MAJOR | 4.5 |
| Zusammenführen über Titel allein trifft falsch (Tag-Titel = Podcast-Titel) | MAJOR | 4.5, 12.4 |
| Tags erst nach der Sonde bekannt | MAJOR | 4.5 `finishPodcastEpisode` |
| Kein Sweep für `podcast_episode`, kein Löschweg für Ordner-Podcasts | MAJOR | 4.5 «Verschwinden und Aufräumen» |
| Anreicherung wird vom nächsten Match gelöscht | MAJOR | 4.4 «nach jedem Match», `item_field_sources` |
| Quellenwechsel reiht nichts ein; keine Konvergenz | MAJOR | 4.4 Konvergenz, 6.3 |
| Titelindex unterbestimmt, Testfall erfunden | MAJOR | 6.1, 12.1 |
| TVDB-Id ist eine Spalte (`00008:23`) | MINOR | 4.3 |
| `Quota` statt `Limiter`; Karten je Art; Feldnamen prüfen | MINOR | 4.4 |
| Keyframe-Risiko überzeichnet | MINOR | 4.5, 12.5 |
| Offline bevorzugt die Datei schon; Abfrage heisst `GetVisibleEnclosure` | MINOR | 4.5 |
| Inhaltswechsel ohne `forgetRoot`, `settleScans`, `ignored_files` | MINOR | 6.3 |
| Gegenprobe in der Migration kann nicht abbrechen | MINOR | 4.6 Punkt 5 |
| Vorgabe sickert in drei weitere Einfügewege | MINOR | 4.3 |
| `DuplicateGroup.reason` braucht einen Wert | MINOR | 5.1 |
| Web: drei Podcast-Annahmen, Fixture-Liste, `library.root.path` | MINOR | 7 |
| Verweise (Provider-Konstanten, M6-Zeilen, `SearchRequest`, `fileInfo`, Wurzeldateien, DoD-grep) | MINOR | 1.1, 2.1, 4.5, 6.1, 8.3, 10 |
| Neue-Folgen-Hook und AN-2 für Ordner-Episoden undefiniert | MINOR | 4.5 |
| Idee 01 / Issue #8 / Herkunft je Feld | MINOR | 3, Schritt 0; mit F-11 kommt die Herkunft je Feld jetzt hierher |

Bestätigt und darum ohne Befund: `visible_items` und kein Trigger lesen `libraries`; nur
`library_roots`, `items`, `account_libraries`, `invitation_libraries` tragen Fremdschlüssel auf
`libraries`; Regel 2 (`depguard`) erlaubt `library` und `metadata` den Zugriff auf `systemq`, und
`library` importiert `metadata` nicht; Regel 10 (nur Schema und Host im Log) deckt sich mit
`netguard.Clean`.

**Fassung 3 und 4 sind noch nicht vom Kritiker geprüft.** Neu gegenüber Fassung 2 und darum
prüfwürdig: 4.2 (`providers` als JSON), 4.3 (`item_field_sources`), 4.4 (Erkenner/Auffüller, TVDB
zuerst mit `/find`, `metadata.fill`), 5.1/5.2 (`MetadataProvider` erweitert, `listMetadataProviders`),
6.2, 6.3 (Erkennerwechsel), 7 (Bauteil), Schritt 1 und 4. Neu in Fassung 4 (F-13): 5.1/5.2
(`LibraryRequest.roots[]`, `LibraryRootsRequest`, `roots_required`, Transaktion über alle Ordner),
7 (Bauteil «Ordnerliste», 7.1 Prototyp), Schritt 1 (`AddRoots`), 2 und 5.
