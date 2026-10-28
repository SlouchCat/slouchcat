# Konzept: Bibliotheken (Ordner mit eigener Inhaltsart und Metadatenquelle)

- **Status: Entwurf, Interview geführt, Kritiker-Review eingearbeitet (28. Oktober 2026).** Marcos
  acht Antworten aus zwei Fragerunden stehen in Abschnitt 11 (F-1 bis F-8); zwei Fragen sind noch
  offen (F-9, F-10). Der OMC-Kritiker hat die erste Fassung am selben Tag geprüft (22 Befunde, davon
  10 MAJOR, kein BLOCKER); alle sind in dieser Fassung eingearbeitet und in Abschnitt 13 mit Herkunft
  genannt. Die Entscheide gehen als **Frage 13** ins Entscheidungsprotokoll
  (`docs/planning/04-entscheidungen.md`) und als **ADR 0015** nach `docs/adr/`, sobald Marco das Papier
  freigibt. Umgesetzt wird nichts, bevor das geschehen ist.
- **Datum: 28. Oktober 2026**
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

> **Abgrenzung zum Scraper-Konzept.** Was eine Metadatenquelle im Einzelnen holt (Sprachen,
> Bildarten, Anbieterreihenfolge, Herkunft je Feld, Übersteuerung je Titel) bleibt Sache von Idee 01
> (`docs/ideas/01-metadaten-einstellungen.md`). Dieses Papier regelt nur, **welcher Ordner mit
> welcher Inhaltsart und welcher Metadatenquelle gelesen wird** — und bringt die zwei Quellen mit, die
> Marco dafür verlangt hat und die es heute nicht gibt: Anime (AniList) und lokale Podcast-Ordner.

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

Die Metadaten wissen von der Bibliothek nichts. `match` (`server/internal/metadata/match.go:78`)
verzweigt nach der **Titelart**: Filme nur TMDB (`matchMovie`, `:122`), Serien TMDB und bei Leerlauf
TVDB (`matchSeries`, `:187`). Es gibt genau zwei Anbieter (`provider.go:12` die Schnittstelle, `:25`
bis `:26` die Konstanten `ProviderTMDB`, `ProviderTVDB`), beide mit serverweiten Schlüsseln.
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

Drei Dinge daran kann das heutige Modell nicht:

1. **Eine Bibliothek mit Film- und Serienordnern.** «Dokus», «Kids» und «Anime» bestehen je aus
   einem Filmordner und einem Serienordner. Heute müsste Marco sechs Bibliotheken anlegen («Dokus
   Filme», «Dokus Serien», …), und jedes Profil bekäme in den Bibliotheksrechten sechs Häkchen statt
   drei (`profile_libraries`, `account_libraries`). Die Art muss vom Ordner kommen, nicht von der
   Bibliothek.
2. **Eine Metadatenquelle je Ordner.** «Anime» soll anders gelesen werden als «Filme». Heute gibt es
   keine Wahl, und es gibt keine Anime-Quelle. Auf dem NAS liegen 29 Anime-Serien und 82 Anime-Filme
   (`/mnt/nerdnas/medien/MangaSerien`, `MangaFilme`), benannt nach Sonarr und Radarr, mit
   `[tvdb-…]`- und `[tmdb-…]`-Marken im Ordnernamen (`names.go:112` liest sie heute schon).
3. **Ein Podcast-Ordner.** `/Podcast` ist auf dem NAS `Audiobookshelf/Podcast` mit 102 Podcasts,
   25 896 MP3- und 135 M4A-Dateien, je Podcast eine `metadata.json` (95 Stück) mit Titel, Autor,
   Beschreibung, **`feedURL`**, `itunesId`, `language`, `explicit` und ein `cover.jpg`. Die Dateien
   tragen ID3-Tags, die Audiobookshelf beim Herunterladen schreibt (`title`, `date` in RFC-2822-Form,
   `comment`/`subtitle` mit der Beschreibung, `album` = Podcastname, `itunes-id`; **keine GUID**).
   Heute nimmt eine Podcast-Bibliothek keinen Ordner an.

Dazu ein kleinerer Punkt: **die Hörbuchbibliothek** liegt unter `Audiobookshelf/Hörbücher` (62
Autorenordner, M4B mit `metadata.json`, `cover.jpg`, teils `.opf`). Das kann der heutige Erkenner
bereits; sie braucht nur einen Ordner mit der Inhaltsart «Hörbuch».

### 1.3 Marcos Entscheide vom 28. Oktober 2026

Zwei Fragerunden, acht Antworten (Abschnitt 11, F-1 bis F-8), in Kurzform:

| Nr. | Entscheid |
|---|---|
| F-1 | **Lokale Podcast-Ordner werden gescannt.** Das offene Ende aus ADR 0007 wird gebaut |
| F-2 | **Anime bekommt jetzt einen eigenen Anbieter**, nicht nur ein Etikett |
| F-3 | **Nur Video ist mischbar.** Filme und Serien dürfen in eine Bibliothek; Hörbücher und Podcasts bleiben eigene Bibliotheken |
| F-4 | **Inhaltsart und Quelle eines Ordners sind nachträglich änderbar**, mit Neuerkennung der Titel |
| F-5 | **Der Anime-Anbieter ist AniList.** Staffeln und Folgen bleiben bei TMDB/TVDB; AniList liefert Titel, Beschreibungen, Bilder, Genres dazu |
| F-6 | **Podcast-Ordner und Feeds zusammen:** steht eine `feedURL` in der `metadata.json`, wird der Feed mitabonniert; Abos ohne Ordner bleiben möglich |
| F-7 | **Der «Hörbuch Scraper» ist der heutige lokale Erkenner.** Ein externer Hörbuch-Anbieter kommt im Scraper-Konzept |
| F-8 | **Die Wahl am Ordner heisst «Metadatenquelle»**, englisch «metadata source» |

---

## 2. Zielbild

```
Bibliothek  (Name, Art: Video | Hörbücher | Podcasts, Standard-Altersstufe bei Audio)
  └─ Ordner  (Pfad, Inhaltsart, Metadatenquelle, Dateisystem, Überwachung)
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
| **Metadatenquelle** | `metadata_source` / `metadataSource` | `tmdb`, `anime`, `audiobook`, `podcast` | Welche Quelle die Titel beschreibt (Abschnitt 4.3) |

Die zulässigen Paare sind eine feste Matrix, die der Server durchsetzt (422 `content_not_allowed`,
`source_not_allowed`):

| Bibliotheksart | Inhaltsart | Metadatenquelle | Entspricht in Marcos Tabelle |
|---|---|---|---|
| `video` | `movie` | `tmdb` | «Filme» |
| `video` | `series` | `tmdb` | «Serien» |
| `video` | `movie` | `anime` | «Anime» (Filme) |
| `video` | `series` | `anime` | «Anime» (Serien) |
| `audiobooks` | `audiobook` | `audiobook` | «Hörbuch Scraper» |
| `podcasts` | `podcast` | `podcast` | «Podcast Scraper» |

Für Hörbücher und Podcasts gibt es heute je genau eine Quelle; die Spalte steht trotzdem von Anfang an
da, damit Idee 01 (Fanart.tv, OMDb, Audnexus, …) später nur Werte ergänzt und keine Tabelle umbaut.

**Benennung** (F-8): In der Oberfläche heissen die zwei Felder **«Inhalt»** (das Wort steht schon
heute über der Artauswahl, `web/src/locales/de.json:194`) und **«Metadatenquelle»**, englisch
«Content» und «Metadata source». Die Quellen heissen «TMDB und TVDB», «Anime (TMDB, TVDB und
AniList)», «Hörbuch (lokale Dateien)», «Podcast (Ordner und Feed)». Die Bibliotheksarten heissen
«Video», «Hörbücher», «Podcasts». Die Vertragswerte bleiben englisch und klein.

**Marcos Tabelle, abgebildet auf das Zielbild** (Pfade so, wie der Stack auf CT 190 das NAS sieht,
`docs/dev/TESTSERVER.md`, Bind-Mount `/media/nas`):

| Bibliothek (Art) | Ordner | Inhalt | Quelle |
|---|---|---|---|
| Filme (video) | `/media/nas/Filme` | movie | tmdb |
| Serien (video) | `/media/nas/Serien` | series | tmdb |
| Dokus (video) | `/media/nas/Doku/Filme` | movie | tmdb |
| Dokus (video) | `/media/nas/Doku/Serien` | series | tmdb |
| Kids (video) | `/media/nas/KinderFilme` | movie | tmdb |
| Kids (video) | `/media/nas/KinderSerien` | series | tmdb |
| Kids (video) | «Kids Filme 2» — auf dem NAS heute ohne Gegenstück | movie | tmdb |
| Anime (video) | `/media/nas/MangaFilme` | movie | anime |
| Anime (video) | `/media/nas/MangaSerien` | series | anime |
| Hörbücher (audiobooks) | `/media/nas/Audiobookshelf/Hörbücher` | audiobook | audiobook |
| Podcast (podcasts) | `/media/nas/Audiobookshelf/Podcast` | podcast | podcast |

Zwei Ordner derselben Inhaltsart in einer Bibliothek («Kids Filme» und «Kids Filme 2») kann das
heutige Modell schon; neu ist nur, dass es dafür keine zweite Bibliothek braucht.

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
| **H-1, H-2** (`:96`, `:97`) | Hörbuch-Bibliotheken auf Ordnern; Podcasts per Feed-URL | Beides bleibt erfüllt. H-2 bekommt einen zweiten Weg (Feed aus der `metadata.json` eines Ordners), kein Ersatz |
| **ADR 0007** (`docs/adr/0007-audio-modell.md:41`): Podcast-Bibliothek «ohne Wurzel»; `:58` nennt lokale Podcast-Ordner als offen | Nicht-Ziel mit Vorbehalt | Wird gebaut (F-1, F-6). ADR 0015 schreibt die Zeile fort; ADR 0007 bekommt einen Ablösungshinweis an genau dieser Stelle. Die Trigger für `podcast_episode` mit Datei sind seit 00014 vorbereitet (`00014_audio.sql:15` bis `:17`; ADR 0007, Abschnitt «Folgen»: «damit eine spätere lokale Episodendatei zählt») |
| **ADR 0006** (`docs/adr/0006-metadaten-anbieter-trailer.md:24` bis `:33`): TMDB ist Hauptquelle und einzige Quelle für Filme; weitere Anbieter sind weitere Implementierungen der einen Schnittstelle | Ein `Provider` stellt die Identität fest | AniList wird **keine** Identitätsquelle (F-5): Staffeln, Folgen und IDs bleiben bei TMDB/TVDB. AniList ist eine **Anreicherung** nach dem Match (Abschnitt 4.4). Das ergänzt ADR 0006, es widerspricht ihm nicht; ADR 0015 benennt den Unterschied zwischen Identität und Anreicherung ausdrücklich und hält fest, dass dieses Papier weder das Herkunftsmodell je Feld aus Idee 01 noch den dort angekündigten Nachfolge-ADR zu 0006 vorwegnimmt |
| **Idee 01** (`docs/ideas/01-metadaten-einstellungen.md:105` bis `:110`): Einstellungen je **Bibliothek** in `library_metadata (library_id PK, …)` | Schlüssel ist die Bibliothek | Mit diesem Konzept ist die natürliche Ebene der **Ordner**. Idee 01 bleibt «Ausgearbeitet», bekommt aber im Kopf einen Vermerk (Schlüssel wird `root_id`, Anbieterliste enthält `anilist`), **und Issue #8 bekommt denselben Vermerk** — nach `docs/ideas/README.md` trägt das Issue Status und Diskussion. Mehr ändert dieses Papier dort nicht |
| **Idee 02** (`docs/ideas/02-ordner-browser.md:3` bis `:6`, Kopf sagt «Schritte 12 bis 14 offen») | veraltet | Nur der Kopf wird in Schritt 0 nachgeführt; E8 bis E18 gelten unverändert |
| **Entscheid 9 / AN-2** (`04-entscheidungen.md:18`, `:43`): Standard-Altersstufe je Hörbuch- und Podcast-Bibliothek | bleibt | unverändert; die Stufe hängt weiter an der Bibliothek, nie am Ordner. Ordner-Podcasts bekommen sie wie Bücher (`rating_source 'library'`) |
| **Harte Regel 11** (nichts vorziehen, keine Nicht-Ziele) | — | Anime und lokale Podcast-Ordner stehen in keiner Nicht-Ziel-Liste der Spec (`:55` bis `:68` nennt Musik, Fotos, Live-TV, DVR, Apps, Plugins, …). Beide gelten als Erweiterung von B-3 und H-2 und werden mit Frage 13 bewusst übernommen |

**Was weiterhin nicht gilt:** keine Bibliotheksnavigation für Zuschauer (2.1); keine Einstellungen je
Quelle und keine Herkunft je Feld (Idee 01); kein AniDB, kein MyAnimeList; kein externer
Hörbuch-Anbieter (F-7); keine Podcast-Suche (ADR 0007, offen); **kein Löschen einer Bibliothek** — es
gibt heute keine `DELETE FROM libraries`-Abfrage und keine Operation dafür, und dieses Papier führt
sie nicht ein, weil Marcos Zielstruktur mit Ordner-Umhängen erreichbar ist (Abschnitt 8.1).

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

### 4.2 `library_roots` bekommt Inhaltsart und Metadatenquelle

```sql
ALTER TABLE library_roots ADD COLUMN content TEXT NOT NULL DEFAULT 'movie'
    CHECK (content IN ('movie', 'series', 'audiobook', 'podcast'));
ALTER TABLE library_roots ADD COLUMN metadata_source TEXT NOT NULL DEFAULT 'tmdb'
    CHECK (metadata_source IN ('tmdb', 'anime', 'audiobook', 'podcast'));
```

`ADD COLUMN` mit `DEFAULT` genügt hier (kein Neubau), weil die Tabelle nur wächst. Die Vorgaben
dienen nur dem Befüllen in der Migration (4.6); der Server setzt beide Werte bei jedem `INSERT`
ausdrücklich (`InsertLibraryRoot`, `server/queries/app/libraries.sql:13`). Die Matrix aus Abschnitt 2
lässt sich nicht als `CHECK` schreiben, weil sie `libraries.kind` braucht; sie wird in
`library.Service` geprüft (bei `Create`, `AddRoot`, `UpdateRoot`) und in der Migration als Wächter
ausgeführt (4.6, Punkt 5).

### 4.3 `items` lernt seine Quelle

`metadata.match` arbeitet je Titel und kennt die Bibliothek nicht (`match.go:78`). Ein Serien-Item hat
keine `media_files` (nur seine Folgen), kann seine Wurzel also nicht über `media_files.root_id`
finden, und ein Titel darf theoretisch Dateien in zwei Wurzeln derselben Bibliothek haben. Darum
bekommt jeder Titel seine Quelle **kopiert**, nicht abgeleitet:

```sql
ALTER TABLE items ADD COLUMN metadata_source TEXT NOT NULL DEFAULT 'tmdb'
    CHECK (metadata_source IN ('tmdb', 'anime', 'audiobook', 'podcast'));
ALTER TABLE items ADD COLUMN anilist_id INTEGER;
ALTER TABLE items ADD COLUMN feed_origin TEXT
    CHECK (feed_origin IS NULL OR feed_origin IN ('admin', 'file'));
```

- Der Scanner schreibt `metadata_source` beim Anlegen aus der Wurzel. Betroffen sind **alle vier
  Einfügewege**, nicht nur `InsertItem` (`server/queries/system/items.sql:4`): `InsertAudiobook`
  (`system/audiobooks.sql:5`), `PodcastInsert` und `PodcastInsertEpisode` (`system/podcasts.sql:12`,
  `:66`). Ohne diese drei stünde an jedem Buch und jedem Podcast stumm `tmdb`. Kinder (Staffel,
  Folge, Extra, Episode) erben den Wert des Elternteils.
- Ändert ein Administrator die Quelle einer Wurzel (F-4), setzt `UpdateRoot` den Wert auf allen
  Titeln, die eine Datei unter dieser Wurzel haben, samt Eltern und Kindern, und reiht je Titel den
  passenden Job ein (Abschnitt 6.3).
- `anilist_id` steht neben `tmdb_id`, `imdb_id` (`00003_libraries_items.sql:59`, `:60`) und
  `tvdb_id` (Spalte seit `00008_library_complete.sql:23`, Index `items_tvdb` `:32`); die
  Zuordnungsliste (4.4) liest `items.tvdb_id` und `items.tmdb_id`.
- `feed_origin` sagt bei einem Podcast, **wer den Feed bestimmt hat**: `admin` für ein von Hand
  abonniertes Abo, `file` für eine `feedURL` aus einer `metadata.json`. Davon hängt der Netzwächter
  ab (9.1). `NULL` für Ordner-Podcasts ohne Feed und für alle Nicht-Podcasts.
- Feed-Podcasts ohne Ordner tragen `metadata_source = 'podcast'`; Hörbücher `audiobook`. Beides wird
  in der Migration aus der Titelart gesetzt.

Warum keine Spalte `items.root_id`? Weil sie für Container falsch wäre (eine Serie gehört zur
Bibliothek, ihre Folgen zu Wurzeln) und weil `media_files.root_id` mit Index
`media_files_root_item (root_id, item_id)` (`00020_media_files_root_item.sql:14`) die Frage «welche
Titel liegen unter dieser Wurzel» bereits beantwortet — genau die Abfrage, die
`SoftDeleteItemsOfRootWithoutOtherFiles` (`server/queries/system/library.sql:73`) heute benutzt.

### 4.4 Die Quelle `anime`: Identität bei TMDB/TVDB, Anreicherung durch AniList

**Identität wie heute.** Ein Ordner mit `metadata_source = 'anime'` läuft durch denselben Parser
(`ParseMoviePath`, `ParseSeriesPath`) und dasselbe `matchMovie`/`matchSeries`. Marcos Anime-Ordner
tragen `[tvdb-…]`- und `[tmdb-…]`-Marken, die `names.go:112` schon liest; die Staffelzählung bleibt
die von TMDB/TVDB, und die manuelle Zuordnung (B-6, `manage.go:103`) funktioniert unverändert.

**Anreicherung nach jedem Match, nicht nur nach dem ersten.** `UpdateItemMatch`
(`server/queries/system/metadata.sql:4` bis `:20`) überschreibt `original_title`, `genres`,
`own_rating` und `rating_source`, und `UpsertItemText` (`:115`) überschreibt Titel und Beschreibung
ohne Rücksicht. Jeder spätere Match — die Nachrüstung aus der Zeit vor M3, eine manuelle Zuordnung,
ein Quellenwechsel — würde eine einmalige Anreicherung also wieder auslöschen. Darum reiht `match`
für einen Titel mit Quelle `anime` **nach jedem erfolgreichen Match** den Job `anime.enrich` ein,
in derselben Transaktion, mit derselben Priorität wie `image.fetch` und Schlüssel `anime:<item>`.
`anilist_id` ist nur der Zwischenspeicher der Zuordnung, nicht die Marke «schon erledigt». Der Job:

1. **löst die AniList-Id auf** über die Zuordnungsliste von Fribb (`github.com/Fribb/anime-lists`,
   Datei `anime-list-full.json`; Felder laut README unter anderem `anilist_id`, `tvdb_id`,
   `themoviedb_id`, `imdb_id`, `mal_id`, `anidb_id`, `season`, `episode_offset`, `type` — **die
   genauen Feldnamen und ihre Form sind vor Schritt 4 gegen die Datei zu prüfen**, das Papier konnte
   nur das README lesen). Die Liste wird einmal geladen, mit ETag unter `/cache/anime-lists/`
   gehalten, wöchentlich erneuert und beim Parsen auf 64 MB begrenzt; sie wird **gestreamt** gelesen
   und in zwei Karten übersetzt, **je nach Art getrennt**, weil TMDB-Film- und TMDB-Serien-Ids
   verschiedene Zahlenräume sind: `(movie, tmdb) → anilist` und `(tvdb) → [anilist mit season,
   episode_offset]`. Bei Serien zeigt eine TVDB-Id auf mehrere AniList-Einträge (eine je Staffel);
   der Job nimmt für die Titelebene den Eintrag mit `season = 1`.
2. **fragt AniList** (`https://graphql.anilist.co`, GraphQL, **ohne Schlüssel**) nach
   `Media(id)`: `title { romaji english native }`, `description`, `genres`, `coverImage.extraLarge`,
   `bannerImage`, `averageScore`, `format`, `isAdult`. Rate-Limit laut AniList-Dokumentation **90
   Anfragen je Minute** mit `X-RateLimit-Remaining` und `Retry-After`; AniList behält sich vor, das
   Limit bei Störungen zu senken, und Drittbibliotheken berichten zeitweise von 30. Der Client läuft
   über **`ratelimit.Limiter`** (`server/internal/ratelimit/ratelimit.go:60`, `Do` `:166`, `Backoff`
   `:149`, `parseRetryAfter` `:219` — derselbe Weg wie TMDB und TVDB, der 429 und `Retry-After`
   schon kann) mit **1 Anfrage je Sekunde, Burst 5**. Nicht `ratelimit.Quota` (`quota.go:36`): das
   ist das Tagesbudget der YouTube-Kette, `Take` wartet nie.
3. **schreibt**, was TMDB nicht hat, und überschreibt nichts, was TMDB hat (feste Reihenfolge TMDB →
   AniList; die Herkunft je Feld aus Idee 01 wird damit **nicht** eingeführt, der Job ist
   idempotent und darf beliebig oft laufen): `anilist_id`; `original_title` = `title.native`, wenn
   leer; `item_texts` (`en`) Titel aus `title.english` (sonst `romaji`) und `overview` aus
   `description`, jeweils nur wenn der TMDB-Wert leer ist; `genres` als Vereinigung; Poster und
   Backdrop aus `coverImage`/`bannerImage` nur, wenn `image.fetch` von TMDB **kein** Bild bekommen
   hat. `isAdult = true` setzt `own_rating` auf 18 mit `rating_source 'anilist'`, sofern keine
   strengere Quelle vorliegt (K-3: nie senken). Was AniList darüber hinaus übersteuern darf, ist F-9.
4. **markiert den Zustand**: `anilist_id = 0` heisst «nachgeschlagen, kein Treffer in der Liste»,
   `NULL` heisst «noch nie». Weil der Job nach jedem Match ohnehin läuft, braucht es keinen
   weiteren Zustand; ein Titel mit `anilist_id = 0` fragt die Liste beim nächsten Lauf trotzdem
   wieder, denn die Liste wächst.

**Konvergenz.** `QueuePendingMetadata(libraryID)` (`match.go:1062`) reiht heute nur
`metadata_state = 'pending'` und die Nachrüstung ein. Neu reiht es zusätzlich `metadata_source =
'anime' AND metadata_state = 'matched' AND anilist_id IS NULL` als `anime.enrich` ein. Damit kommt
ein Titel auch dann zu seinen AniList-Daten, wenn `SLOUCHCAT_ANILIST=off` wieder eingeschaltet
wurde, ein Job fehlgeschlagen ist oder die Quelle gewechselt hat (6.3).

**Das Fenster.** Zwischen `UpdateItemMatch` und dem Lauf von `anime.enrich` steht ein Titel kurz
ohne die AniList-18 da. Für einen Titel, den nur AniList als `isAdult` kennt, sieht ein Kinderprofil
ihn in diesem Fenster. Das Fenster ist so lang wie die Warteschlange vor dem Job; es wird in ADR 0015
als Restrisiko benannt (12.4) und mit der Konvergenzabfrage klein gehalten, nicht null. Null wäre
nur mit der Anreicherung **in** der Match-Transaktion zu haben, also mit einem Netzaufruf in einer
Schreibtransaktion — das tut dieses Projekt nirgends.

Was fehlt und bewusst fehlt: eine **Titelsuche bei AniList** für Ordner, die TMDB nicht kennt. Ohne
TMDB-Identität gäbe es heute weder Credits noch Trailer noch die Bildpipeline; ein solcher Titel bleibt
`unmatched` und erscheint in den Pflegewerkzeugen (B-9) wie jeder andere. Das gehört in Idee 01, wenn
AniList dort ein vollwertiger Textanbieter wird.

Vor Schritt 4 **gegen die Quelle zu prüfen**, weil dieses Papier es nicht verifizieren konnte: die
Rohadresse der Zuordnungsliste (erwartet
`https://raw.githubusercontent.com/Fribb/anime-lists/master/anime-list-full.json`), die Feldnamen,
die Lizenz für die Weiterverwendung in einem AGPL-Projekt, und ob Filme in der Liste zuverlässig
`themoviedb_id` tragen (die Liste verschmilzt über `anidb_id`; Filme sind dort seltener als Serien).
Fehlt die TMDB-Zuordnung eines Films, bleibt er ohne AniList-Daten — kein Fehler, ein Pflegefall.

### 4.5 Die Quelle `podcast`: Ordner lesen, Feed mitnehmen

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
   'series' THEN 'series' WHEN 'audiobooks' THEN 'audiobook' END, metadata_source = CASE l.kind WHEN
   'audiobooks' THEN 'audiobook' ELSE 'tmdb' END FROM libraries l WHERE l.id = library_id`. Das
   `CASE` hat **kein `ELSE`** für `content`: eine Wurzel an einer Podcast-Bibliothek (die es heute
   nicht geben kann, `roots.go:34`) ergäbe `NULL` und scheiterte am `NOT NULL` — der erste Wächter.
2. **`items` erweitern** (4.3): `metadata_source` nach Titelart (`audiobook` → `audiobook`;
   `podcast`, `podcast_episode` → `podcast`; alles andere `tmdb`), `anilist_id NULL`, `feed_origin =
   'admin'` für jeden Podcast mit `feed_url`, sonst `NULL`.
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
   «`items.metadata_source` passt zur Titelart».
6. **Kein Datenverlust an Kindtabellen:** `items`, `library_roots`, `account_libraries`,
   `invitation_libraries` zeigen über Fremdschlüssel auf `libraries`; `profile_libraries` hat keinen
   (`00002_accounts_sessions.sql:43`). Die `id` bleibt, die Fremdschlüssel sind während des Laufs
   aus; `foreign_key_check` nach dem Lauf ist der Beleg.

**Was die Migration ausdrücklich nicht tut:** Sie legt keine Ordner an, abonniert keinen Feed, reiht
keinen Scan ein und fasst keine Downloads an. Marcos Zielstruktur entsteht danach in der Oberfläche
(8.1).

---

## 5. Auswirkung auf den Vertrag

### 5.1 Schemata

| Schema | Heute (`api/openapi.yaml`) | Neu |
|---|---|---|
| `LibraryKind` (`:7759`) | `[movies, series, audiobooks, podcasts]` | `[video, audiobooks, podcasts]` |
| `LibrarySummary` (`:7766`) | `id, name, kind, defaultRating` | unverändert; `kind` mit neuen Werten |
| `LibraryRequest` (`:7781`) | `name, kind, path?, defaultRating?` | `name, kind, defaultRating?, root?: LibraryRootRequest` — der erste Ordner bringt Inhaltsart und Quelle mit; Pflicht für `video` und `audiobooks`, bei `podcasts` erlaubt mit `content: podcast`. **Die Validierungsfelder heissen damit `library.root.path` statt `library.path`**; die Fehlerzuordnung des Einrichtungsassistenten hängt an diesen Namen |
| `LibraryRootRequest` (`:7805`) | `path` | `path, content, metadataSource` (alle Pflicht) |
| `LibraryRootUpdate` | — | neu: `content?, metadataSource?, confirmPath?` (5.2) |
| `LibraryRoot` (`:7878`) | `id, path, fsType, watch, itemCount, fileCount, warning?` | zusätzlich `content`, `metadataSource` |
| `LibraryUpdate` (`:7831`) | `name?, defaultRating?` | unverändert — die Familie ist nach dem Anlegen fest (wie heute `kind`) |
| `RootContent` | — | neu: `[movie, series, audiobook, podcast]` |
| `MetadataSource` | — | neu: `[tmdb, anime, audiobook, podcast]` |
| `MetadataProvider` (`:7494`) | `[tmdb, tvdb]` | unverändert — das ist die Identitätsquelle für `searchMetadata` und die manuelle Zuordnung; AniList gehört nicht hinein (4.4) |
| `DuplicateGroup.reason` (`:7706`) | `[provider_id, title_year, file_hash]` | zusätzlich `file_and_feed` (4.5) |
| `ItemRef`, `ItemDetail` | — | **unverändert.** `anilistId` wird nicht ausgeliefert; ein Zuschauer braucht es nicht, Fremd-Clients bekommen es mit der Herkunft je Feld aus Idee 01 |
| `AdminItem` | `libraryId, …` | zusätzlich `metadataSource` und `anilistId?` |
| `AdminPodcast` (`:7292`) | Feed-Daten | zusätzlich `feedOrigin?`, `rootId?`, `localEpisodeCount` |
| `SetupRequest.library` (`:127`, `runSetup`) | `LibraryRequest` | folgt automatisch; der Assistent wählt Inhalt und Quelle des ersten Ordners |

### 5.2 Operationen

| Operation | Änderung | `x-visibility` |
|---|---|---|
| `createLibrary` (`:2157`) | Körper nach 5.1; 422 `content_not_allowed`, `source_not_allowed`, weiterhin `library_path_invalid` | admin |
| `addLibraryRoot` (`:2368`) | Körper nach 5.1; dieselben 422; Podcast-Bibliotheken nehmen Ordner mit `content: podcast` an | admin |
| **`updateLibraryRoot`** — `PATCH /admin/libraries/{id}/roots/{rootId}` | **neu** (F-4). Ein Wechsel der **Quelle** allein löscht nichts und braucht keine Bestätigung (6.3). Ein Wechsel der **Inhaltsart** ist in der Wirkung «Ordner entfernen und neu anlegen» und verlangt `confirmPath` wie `removeLibraryRoot` (422 `confirm_mismatch`), 409 `scan_running` | admin |
| `scanLibrary` (`:2268`) | Für `podcasts`: Ordner-Scan **und** Feed-Abruf; heute nur Letzteres (`admin.go:357`) | admin |
| `removeLibraryRoot` (`:2414`) | Podcast-Ordner: Podcasts mit Feed bleiben als Abo (4.5) | admin |
| `listAccountLibraries` (`:857`) | nur neue `kind`-Werte | account |
| `searchMetadata` (`:3142`) | unverändert; `kind: [movie, series]` ist die Titelart | admin |
| `listDuplicates` | neue Gruppenart `file_and_feed` | admin |

Jede neue oder geänderte Operation bekommt ihren Eintrag im Vertragstest
(`TestContractCoversEveryOperation`, `server/internal/api/contract_access_test.go`) und den
Schema-Test auf `x-visibility` (Regel 7). Neue Fehlercodes gehen in die `problem`-Liste des Vertrags
und in `web/src/locales/*.json`.

### 5.3 Ist das eine brechende Anhebung?

**Ja.** `LibraryKind` verliert `movies` und `series`, `LibraryRequest.path` verschwindet zugunsten
von `root`, `LibraryRootRequest` bekommt Pflichtfelder. Für Fremd-Clients (C-5) ist das brechend, wie
es 1.1.0 (`HomeRowKey`) und 1.2.0 (`AccountRole`) waren. **Die Fassung ist 1.3.0**, dem Präzedenzfall
folgend, und der Bruch steht wörtlich in den Release-Notes. Entfernte Enum-Werte (`movies`, `series`
in `LibraryKind`) werden nie wiederverwendet.

Was **nicht** bricht: kein `scoped`-Schema ändert sich; die Web-App auf einem alten Stand gegen
einen neuen Server scheitert nur auf der Bibliotheksseite der Verwaltung.

---

## 6. Scanner, Metadaten und Jobs

### 6.1 Der Scanner liest je Wurzel

Heute ruft `scanLibrary` (`scanner.go:429`) für jede Wurzel `walkRoot(ctx, rs.proot, lib.Kind, …)`
(`:446`). Neu bekommt `walkRoot` die **Wurzel** (`op.rs.root` mit `Content` und `MetadataSource`)
statt der Bibliothek; `fileInfo` trägt selbst keine Wurzel, der Wurzelzustand `rs` tut es. Die
Stellen, die heute `lib.Kind` lesen, lesen den Inhalt der Wurzel:

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
| `admin.validateLibrary` (`admin.go:240`) | Pfad je Art | Matrix aus Abschnitt 2 |
| `admin.ScanLibrary` (`:352`) | Podcasts → nur Feed | Podcasts → Ordner-Scan, dann Feed-Abruf |
| `library.Create` (`service.go:189`), `AddRoot` (`roots.go:26`) | Art prüfen, Podcasts ohne Wurzel | Matrix prüfen; Podcasts mit `content: podcast` |
| `maintenance.go:429` (Zusammenführen nur gleiche Art und gleiche Bibliothek) | unverändert | unverändert — Titelart, nicht Wurzel |
| Webhooks (`integrations/pathmap.go:66`) | Pfad → irgendeine Wurzel | unverändert; Radarr trifft Filmwurzeln, Sonarr Serienwurzeln, weil Marco die Umsetzungstabelle so pflegt. Eine Prüfung «Radarr darf nur auf `movie`-Wurzeln» wäre eine Sicherheitsgewinn-Illusion und bleibt aus |
| Go-Fixtures mit `Kind: "movies"` | 13 `*_test.go` im Server, `vistest/vistest.go:347`, `:389`, `vistest/audio.go:75`, `:79`, `cmd/apicheck/fixtures.go:228`, `:270`, `cmd/slouchcat/scanbench.go:73` bis `:92` | gehen **im selben Commit** wie das Schema um (10, Schritt 1) |

`scan.path` (`jobs.go:17` ff.) trägt `rootId` bereits je Verzeichnis; der Watcher läuft je Wurzel
(`service.go:807` ff.). Beides ändert sich nicht.

### 6.2 Metadaten

- `match` (`match.go:78`) liest `items.metadata_source`. Für `tmdb` und `anime` läuft derselbe
  Weg; bei `anime` reiht jeder erfolgreiche Match `anime.enrich` ein (4.4). `audiobook` und `podcast`
  werden wie heute übersprungen (`:101`, `default`).
- Neues Paket? **Nein.** Der AniList-Client wird `server/internal/metadata/anilist.go` neben
  `tmdb.go` und `tvdb.go`; die Zuordnungsliste `anime_lists.go` im selben Paket, Cache unter
  `/cache/anime-lists/`. Beide sprechen nur mit festen Hosts (`graphql.anilist.co`,
  `raw.githubusercontent.com`); ein Schalter `SLOUCHCAT_ANILIST=off` schaltet die Anreicherung
  serverweit ab, ohne dass die Quelle `anime` am Ordner verschwindet (die Titel bleiben dann
  TMDB-Titel). Die Zahl der Anreicherungen steht im Dashboard neben Trailern.
- `podcast.Subscribe` (`podcast.go:214`) prüft weiter die Art `podcasts` und setzt `feed_origin =
  'admin'`; `PodcastInsert` (`system/podcasts.sql:12`) nimmt neu `feed_url NULL` an (reiner
  Ordner-Podcast; die Spalte ist schon `NULL`-fähig, `00014_audio.sql:71`). Die zwei Abfragen des
  Abrufs bekommen `feed_url IS NOT NULL` (4.5).

### 6.3 `UpdateRoot`: Quelle wechseln ist billig, Inhalt wechseln ist ein Umzug

**Quelle wechseln** (`tmdb` ↔ `anime`): `UPDATE items SET metadata_source = ?` für alle Titel mit
einer Datei unter der Wurzel (Join über `media_files.root_id`), dazu deren Eltern und Kinder; dann
**je Titel** den passenden Job: bei `tmdb → anime` `anime.enrich` (der Match steht ja schon), bei
`anime → tmdb` `metadata.match` mit `anilist_id = NULL`, damit TMDB die Felder zurückschreibt.
`QueuePendingMetadata(libraryID)` taugt dafür nicht, es nimmt keine Titel-Ids und reiht nur
`pending`. Keine Löschung, kein Scan, keine Bestätigung. Hörbuch- und Podcast-Wurzeln haben nur eine
Quelle; dort ist der Aufruf ein 422 `source_not_allowed`.

**Inhalt wechseln** (`movie` ↔ `series`; andere Wechsel verbietet die Matrix): unter der Scan-Sperre
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
5. `UPDATE library_roots SET content = ?, metadata_source = ?`.
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
Regel 12). Die Matrix aus Abschnitt 2 steht auch im Client, aber nur, um Unmögliches gar nicht erst
anzubieten (Entscheid vom 25. Oktober 2026 zu Spotlight: «Unmögliche Kombinationen werden gar nicht
erst angeboten», `04-entscheidungen.md`); der 422 bleibt der Riegel.

| Seite / Datei | Stelle | Was geschieht |
|---|---|---|
| **Bibliothek anlegen**, `web/src/features/admin/LibrariesPage.tsx` | `AddLibraryForm` `:24` bis `:86`, `kind` `:27` | Die Artauswahl wird zu **«Video / Hörbücher / Podcasts»**. Darunter der erste Ordner: Pfad mit «Durchsuchen …» (`PathBrowserButton`, `:74`), **«Inhalt»** (bei Video: Filme / Serien; bei Hörbüchern und Podcasts fest und ausgeblendet) und **«Metadatenquelle»** (bei Video: «TMDB und TVDB» / «Anime»; sonst fest). Standard-Altersstufe wie heute nur bei Audio. Der Podcast-Hinweis `library.podcastsHint` («braucht keinen Ordner») wird zu «Ordner optional; Feeds lassen sich auch ohne Ordner abonnieren» |
| **Drei Stellen, die «Podcast hat keinen Ordner» annehmen** | `LibrariesPage.tsx:283` (versteckt `FoldersAction`), `LibraryCard.tsx:138` (Text «keine Ordner» für Podcasts), `features/admin/queries.ts:52` (`canScan`) | alle drei fallen: Podcast-Bibliotheken haben Ordner **oder** Feeds, der Scan-Knopf gilt, sobald eines davon da ist |
| **Geteiltes Formular**, `web/src/features/shared/LibraryFields.tsx` | `KINDS` `:10`, `RadioGroup` `:76` bis `:82`, Pfad `:89` bis `:105`, `libraryRequest` `:23` bis `:36` | Trägt die neuen Felder und baut `LibraryRequest.root`. Das Formular ist mit dem **Einrichtungsassistenten** geteilt (`web/src/features/setup/SetupPage.tsx:127`, `:210`, `:277`); der Assistent bekommt damit dieselben zwei Felder, ohne eigenen Code, und seine Fehlerzuordnung folgt den neuen Feldnamen `library.root.path`. Vorgabe bleibt Video / Filme / TMDB; der Vorgabename `t('library.kind.movies')` (`:277`) wird «Filme» aus einem neuen Schlüssel, weil `library.kind.movies` verschwindet |
| **Bibliothekskarte**, `web/src/features/admin/LibraryCard.tsx` | Chip `:57`, `Folders` `:128` bis `:171` | Der Art-Chip zeigt die Familie; bei mehreren Ordnern steht statt «2 Ordner» eine Zusammenfassung «1 Filme, 1 Serien». Bei genau einem Ordner wie heute Pfad, Dateisystem, Überwachung, neu plus Inhalt und Quelle |
| **Ordner von X**, `web/src/features/admin/LibraryFoldersDialog.tsx` | Hinzufügen `:64`, Formular `:174` bis `:198`, Entfernen `:86` | Jede Zeile zeigt zwei Chips (Inhalt, Quelle) und bekommt neben «Entfernen» ein **«Ändern»**. Das Hinzufügen-Formular bekommt die zwei Felder. «Ändern» öffnet einen Dialog mit denselben zwei Feldern; wechselt der Inhalt, erscheint darunter die abgetippte Bestätigung (die `TypedConfirmation`-Variante von `ConfirmDialog` aus Schritt 13 der Idee 02) mit dem Text aus 6.3 |
| **Podcasts**, `web/src/features/admin/PodcastsPage.tsx` | `:129` filtert `kind === 'podcasts'` | Bleibt die Seite für Feed-Abos. Podcasts aus Ordnern erscheinen in derselben Liste mit dem Chip «aus Ordner» und der Zahl lokaler Episoden; «Feed entfernen» ist bei `feedOrigin: file` ausgeblendet, denn der Ordner bestimmt den Feed |
| **Titelpflege**, `web/src/features/admin/AdminItemPage.tsx` | Match-Bereich `:357`, `:409` | Zeigt die Quelle des Titels und bei `anime` die AniList-Id als Link; die manuelle Zuordnung bleibt TMDB/TVDB |
| **Bibliothekswahl in Profilen und Zugang**, `useLibraryOptions.ts:35` | liest nur `id`, `name` | **keine Änderung** |
| **Zuschauer** | — | **keine Änderung** (2.1) |

**Regel 8 — Tastaturtests.** Es entsteht keine neue Route, aber zwei Formulare wachsen um je zwei
Felder und ein Dialog kommt dazu; in der Wirkung ist «Ändern» eine neue Seite. Darum:
`web/e2e/admin-libraries.spec.ts` (Tab-Reihenfolge `:27`, Audio-Arten `:48`) wird auf die neue
Artauswahl und die zwei Felder erweitert; `web/e2e/admin-library-folders.spec.ts` bekommt einen
Test für «Ändern» (Pfeiltasten in beiden Radiogruppen, Tab in die Bestätigung, Enter, Escape, Zurück,
sichtbarer Fokus, keine Fokusfalle); `web/e2e/setup.spec.ts:115` («arrow keys switch the library
kind») folgt der neuen Auswahl.

**Fixtures, vollständig gezählt**, weil sie den Schrittschnitt bestimmen (10): `web/e2e/global-setup.ts:55`,
`:61`, `:68`, `:125`, `:128`, `:139`, `:176` bis `:177`; `web/e2e/accounts.ts:197`;
`web/e2e/audio.ts:178`; `web/src/test/fixtures.ts:218`, `:268` bis `:269`, `:286`. Dazu eine gemischte
Bibliothek «Dokus» mit einem Film- und einem Serienordner, damit der Mischfall im e2e-Lauf vorkommt —
sie braucht **eigene** Testordner, weil Wurzeln sich nicht überlappen dürfen (`MOVIES_DIR`, `TV_DIR`
sind vergeben), und sie verschiebt Zählungen in anderen Specs.

**Regel 9 — Texte und Farben.** Neue Schlüssel in `web/src/locales/de.json` und `en.json`:
`library.kind.{video,audiobooks,podcasts}` (die vier alten `library.kind.*`-Schlüssel `:242` bis
`:247` verschwinden mit den Werten), `library.content.{movie,series,audiobook,podcast}`,
`library.source.{tmdb,anime,audiobook,podcast}` mit Kurzbeschreibungen,
`admin.libraries.folders.change.*` (Dialog und Bestätigungstext), `admin.podcasts.fromFolder`,
`setup.library.content`, `setup.library.source`, die neuen Fehlercodes und die Gruppenart
`file_and_feed`. `pnpm -C web test locales` prüft die Symmetrie. Chips kommen aus `web/src/ui`.

---

## 8. Migration und Rückwärtsverträglichkeit

### 8.1 Was mit dem Bestand geschieht

Nach Migration 00023 ist jede bestehende Bibliothek, was sie vorher war: eine Filmbibliothek ist eine
Video-Bibliothek mit einem Filmordner (Quelle TMDB), eine Serienbibliothek eine Video-Bibliothek mit
einem Serienordner. Kein Scan wird nötig, kein Titel ändert sich.

Auf **CT 190** (Stand 28. Oktober 2026, gelesen aus `/opt/slouchcat/deploy/config/slouchcat.db`):
Bibliothek 1 «Filme» (`movies`, Wurzel `/media/nas/Filme`, 1786 Dateien) und Bibliothek 2 «Serien»
(`series`, Wurzel `/media/nas/TestSerien`). Auf dem **Entwicklungsrechner** (`deploy/config/`):
Bibliothek 1 «Filme» (`movies`, `/test/Filme`). Beide Stände migrieren zu `video` mit je einem
Ordner.

Marcos Zielstruktur baut er danach **von Hand in der Oberfläche**, ohne Datenverlust an den
bestehenden Titeln: «Dokus» neu anlegen mit `/media/nas/Doku/Filme` (Filme) und `/media/nas/Doku/Serien`
(Serien) hinzufügen; «Kids» ebenso; «Anime» mit Quelle Anime; «Hörbücher» und «Podcast» als neue
Bibliotheken. «Serien» bekommt `/media/nas/Serien` dazu und verliert `/media/nas/TestSerien` über den
bestehenden Löschweg. Für die Hörbücher und Podcasts gilt die Lastrechnung aus 4.5; der erste Scan
dieser zwei Bibliotheken ist ein Messlauf (Schritt 6), kein Nebenbei.

### 8.2 Wo alte Clients scheitern

Die Web-App des Standes 1.2.0 sendet `LibraryRequest.path` und erwartet `kind: movies`; gegen einen
1.3.0-Server scheitert nur die Bibliotheksseite der Verwaltung mit 422 (fehlender `root`), nichts auf
der Zuschauerseite. Weil Web-App und Server aus demselben Image kommen, ist das nur für den Moment
zwischen Image-Wechsel und Neuladen relevant; der Service Worker lädt bei Versionswechsel neu.

### 8.3 Eingefrorene Datenbankstände

`testdata/db/` hält M1a bis M11 (`TestFrozenStatesMigrate`, `server/internal/store/frozen_test.go:23`).
Migration 00023 muss über **jeden** laufen; M6 und M7 sind die wichtigen, weil sie alle vier alten
Arten enthalten, darunter eine Podcast-Bibliothek ohne Wurzel (die vier Bibliotheken in
`testdata/db/M6.sql:737` bis `:740`, ihre Wurzeln `:215` bis `:217`). `PRAGMA foreign_key_check` muss
nach jedem Stand leer sein. Nach der Umsetzung kommt ein neuer Stand dazu (`test(store): freeze the …
database state`, Vorbild `5bc6ce7`) mit einer gemischten Video-Bibliothek, einem Anime-Titel mit
`anilist_id` und einem Ordner-Podcast mit Feed und Datei. Liegt die Benutzerverwaltung bis dahin in
`main`, ist ihr eingefrorener Stand (mit 00022) Teil der Reihe und 00023 läuft auch über ihn.

---

## 9. Sicherheit

### 9.1 Eine Feed-Adresse aus einer Datei bekommt nie den ungeschützten Client

Die `feedURL` in einer `metadata.json` ist Inhalt einer Datei auf dem NAS — nicht die Eingabe eines
Administrators. Der Server ruft sie trotzdem ab (F-6). **Der Netzwächter allein reicht dafür nicht**,
und das ist der Punkt, den die erste Fassung dieses Papiers falsch hatte: `netguard` lässt einen Feed
auf einem lokalen Host absichtlich durch — «ein Feed, den der Haushalt mit Absicht im LAN betreibt;
seine Enclosures und Cover dürfen dann auch lokal sein» (`server/internal/netguard/netguard.go:1` bis
`:6`), und `Clients.For` gibt für einen solchen Feed den **ungeschützten** Client heraus (`:96`,
`LocalHost` `:38`). Diese Ausnahme ist für den Administrator gedacht, der die Adresse selbst tippt.
Für eine Adresse aus einer Datei darf sie nicht gelten, sonst entschiede eine Datei auf dem NAS, dass
der Server Adressen im Heimnetz abruft, durchreicht und herunterlädt.

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
behandelt `ratelimit.Limiter` wie bei TMDB. Fällt AniList aus, bleiben die Titel TMDB-Titel; der Job
meldet den Fehler in den Fehlerring, nicht an den Zuschauer.

### 9.4 Keine Sichtbarkeitsregel ändert sich

`visible_items` liest weder `libraries.kind` noch `library_roots`; die Rechte hängen an
`library_id`, das Alter an der Titelart. Ein Profil, das «Dokus» nicht sehen darf, sieht weder die
Filme noch die Serien darin — eine Bibliothek, ein Recht (Regel 2; Nicht sichtbar heisst 404).

### 9.5 Der Inhaltswechsel ist so schwer wie das Entfernen

`updateLibraryRoot` mit Inhaltswechsel verlangt `confirmPath`, läuft unter der Scan-Sperre, in einer
Transaktion, und schreibt die `Warn`-Zeile und die `notify`-Meldung des Löschwegs (Idee 02, 6.7).
Nichts daran ist leichter zu erreichen als `removeLibraryRoot`.

---

## 10. Schnitt in Schritte

Reihenfolge nach Baubarkeit, und **so geschnitten, dass `task ci` nach jedem Commit grün sein kann**:
die erste Fassung dieses Papiers trennte Schema, Vertrag und Scanner, aber die alten Werte `movies`
und `series` stecken in `ValidLibraryKind` (`model.go:982`), im Scanner, in 13 Server-Testdateien,
in den `vistest`-Fixtures und in den e2e-Fixtures, und `task ci` fährt die e2e-Suite mit
(`Taskfile.yml:68` bis `:74`). Ein Schema ohne Scanner liesse eine migrierte Serienbibliothek als
Filmbibliothek lesen; ein neuer Client ohne angepasste Fixtures liesse `LibraryFields.tsx:8` bis
`:10` (`satisfies LibraryKind[]`) nicht mehr kompilieren. Darum sieben Schritte statt acht. **Beginn
erst, wenn die Benutzerverwaltung in `main` ist** (F-10).

### Schritt 0 — Entscheide und Papiere
`docs(planning,adr): record the library decisions and ADR 0015`

**Ziel:** Marcos Antworten F-1 bis F-10 stehen verbindlich im Entscheidungsprotokoll, der neue
Wortlaut zu B-1 und ADR 0007 ebenso.

**Berührte Dateien:** `docs/planning/04-entscheidungen.md` (neue **Frage 13** nach dem Muster von
Frage 12, dazu eine Zeile in «Weitere Entscheide»), `docs/adr/0015-bibliotheken-ordner.md` (neu:
Familie, Ordnerwerte, Matrix, Identität versus Anreicherung samt Fenster, Podcast-Ordner und
`feed_origin`, Inhaltswechsel; mit dem Satz, dass Herkunft je Feld und der Nachfolger von ADR 0006
Idee 01 bleiben), Ablösungshinweise in `docs/adr/0007-audio-modell.md` (Zeile «ohne Wurzel», offener
Punkt) und `docs/adr/0006-metadaten-anbieter-trailer.md` (Anreicherung), Kopfvermerk in
`docs/ideas/01-metadaten-einstellungen.md` (Schlüssel `root_id`, Anbieter `anilist`) **und derselbe
Vermerk als Kommentar in Issue #8**, Kopf von `docs/ideas/02-ordner-browser.md` (Schritte 12 bis 14
sind gebaut), dieses Papier.

**DoD:** `git diff --stat docs/spec/` ist leer. Frage 13 trägt alle zehn Antworten mit Datum. ADR 0015
nennt B-1, B-3, H-1, H-2, AN-2, C-5, K-3. Der Kommentar in Issue #8 ist gesetzt (`gh issue comment
8`). `task ci` grün.

### Schritt 1 — Schema, Modell, Scanner und Fixtures
`feat(store,library): content and metadata source per root, scanned per root`

**Ziel:** Die Datenbank kennt Familie, Inhaltsart und Quelle, **und der Server liest sie schon**:
eine Video-Bibliothek mit Film- und Serienordner scannt richtig. Der Vertrag ändert sich noch nicht;
die Handler übersetzen die alten Vertragswerte (`movies` → `video` + Wurzel `movie`, `series` →
`video` + Wurzel `series`) für genau einen Schritt, damit die Web-App und die e2e-Suite unverändert
laufen.

**Berührte Dateien:** `server/migrations/00023_library_roots_content.sql` (neu, Plan 4.6),
`server/queries/app/libraries.sql` (`InsertLibrary`, `InsertLibraryRoot`, `ListLibraryRoots`, neue
`UpdateLibraryRoot`, `UpdateItemsMetadataSourceByRoot`), `server/queries/system/items.sql`
(`InsertItem`), `system/audiobooks.sql` (`InsertAudiobook`), `system/podcasts.sql` (`PodcastInsert`,
`PodcastInsertEpisode`, `PodcastListDue`, `PodcastListIDsByLibrary`), `system/library.sql`
(`DeleteMediaFilesOfRoot`, `DeleteIgnoredFilesOfRoot`), `server/internal/model/model.go` (`:975` bis
`:991`: `LibraryVideo`, `RootContent*`, `MetadataSource*`, `ValidRootPair`; `LibraryRequest` `:994`
bekommt `Root`; `LibraryRoot` `:1017` die zwei Felder), der Scanner nach 6.1 (`scanner.go`,
`write.go`, `index.go` mit einem Index je Titelart, `pathscan.go`), `library/service.go` (`Create`),
`roots.go` (`AddRoot`, neu `UpdateRoot` mit beiden Wegen aus 6.3), `admin/admin.go`
(`validateLibrary`, `ScanLibrary`), `metadata/match.go` (`:78` liest `metadata_source`), die
Go-Fixtures aus 6.1 (13 Testdateien, `vistest`, `cmd/apicheck/fixtures.go`, `scanbench.go`), danach
`task gen`.

**DoD:** `task ci` grün. `go test ./internal/store -run TestFrozenStatesMigrate` grün über alle
Stände, `foreign_key_check` leer; der Wächter aus 4.6 Punkt 5 bricht einen präparierten Stand mit
falschem Paar ab (Test). M6-Stand nach Migration: vier Bibliotheken `video`, `video`, `audiobooks`,
`podcasts`, Wurzeln `movie/tmdb`, `series/tmdb`, `audiobook/audiobook`, `items` der Hörbuchbibliothek
tragen `audiobook`, der Feed-Podcast `podcast` und `feed_origin = 'admin'`. `go test
./internal/library/`: eine gemischte Bibliothek mit einem Film und einer Serie, die **dieselbe
numerische `tmdb_id`** tragen, ergibt zwei Titel verschiedener Art; ein Serienordner verwirft Dateien
ohne Folgenmarke, der Filmordner derselben Bibliothek nicht; nach Inhaltswechsel `movie → series` gibt
es keinen Titel mit `deleted_at IS NULL` und null Dateien, der Download des gegangenen Films trägt
einen Tombstone, `ignored_files` der Wurzel sind leer, `pathPending` und `orphans` der Wurzel sind
leer, der Watcher läuft weiter, der Scan legt Folgen neu an; Quellenwechsel reiht je Titel der Wurzel
genau einen Job ein und löscht nichts. `slouchcat scanbench` ohne Regression gegen
`docs/perf/M3.md`. `SELECT sql FROM sqlite_master WHERE sql LIKE '%libraries%'` auf dem M11-Stand
nennt keine Sicht und keinen Trigger (Beleg für 4.6 Punkt 4).

### Schritt 2 — Vertrag 1.3.0 und die Fixtures beider Seiten
`feat(api): library families, root content and metadata source, updateLibraryRoot`

**Ziel:** Der Vertrag beschreibt das Zielbild, die Übersetzung aus Schritt 1 fällt, und Client-Typen
wie Fixtures folgen **im selben Commit**, sonst ist `task ci` rot.

**Berührte Dateien:** `api/openapi.yaml` (5.1, 5.2, `info.version: 1.3.0`),
`server/internal/api/admin_handlers.go` (`:146` und die Wurzel-Handler), `contract_access_test.go`
(Einträge), `cmd/apicheck/fixtures.go`, `packages/api-client` über `task gen`; auf der Web-Seite das
**Minimum**, damit Typen und e2e-Suite wieder stimmen: `web/src/features/shared/LibraryFields.tsx`
(`KINDS`, `libraryRequest` baut `root`), die Fixtures aus Abschnitt 7 (`global-setup.ts`,
`accounts.ts`, `audio.ts`, `web/src/test/fixtures.ts`), `setup.spec.ts:115`. Die Oberfläche selbst
(Felder, Chips, Dialoge) kommt in Schritt 5; bis dahin sendet das Formular feste Werte (Video / Filme /
TMDB bzw. die Audio-Paare).

**DoD:** `task ci` grün (`gen:check` ohne Drift). Vertragstests: 422 `content_not_allowed` für
`audiobook` in einer Video-Bibliothek, `source_not_allowed` für `anime` an einem Hörbuchordner,
`confirm_mismatch` und 409 `scan_running` beim Inhaltswechsel, 200 beim Quellenwechsel ohne
Bestätigung; eine Podcast-Bibliothek nimmt einen Ordner mit `content: podcast` an und lehnt `movie`
ab. Der Schema-Test findet an jeder neuen Operation `x-visibility`. Release-Notes-Entwurf mit dem
Bruch (`LibraryKind`, `LibraryRequest.root`, `DuplicateGroup.reason`).

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

### Schritt 4 — AniList
`feat(metadata): enrich anime titles from AniList after every TMDB match`

**Ziel:** Ein Titel aus einem Anime-Ordner trägt `anilist_id`, den japanischen Originaltitel und
Texte, wo TMDB leer war — und behält sie über jeden späteren Match.

**Berührte Dateien:** `server/internal/metadata/anilist.go` (Client über `ratelimit.Limiter`),
`anime_lists.go` (Zuordnungsliste, Cache, Streaming-Parser, 64 MB, Karten je Art), `anime_enrich.go`
(Job `anime.enrich`), `match.go` (Einreihen nach jedem Match, Konvergenzabfrage in
`QueuePendingMetadata`), Tests mit `httptest`-Servern für beide Hosts, `config.go`
(`SLOUCHCAT_ANILIST`), Dashboard-Zähler, `cmd/apicheck`.

**DoD:** **Vorher** die Prüfpunkte aus 4.4 (Rohadresse, Feldnamen, Lizenz der Liste; TMDB-Ids bei
Filmen) mit Beleg in der Rückmeldung. `go test ./internal/metadata/`: TVDB-Id → AniList-Id über die
Liste (Staffel 1), TMDB-Film-Id ebenso, und ein Film und eine Serie mit derselben Zahl landen bei
verschiedenen AniList-Einträgen; leere TMDB-Beschreibung wird gefüllt, vorhandene nicht angefasst;
`isAdult` hebt auf 18 und senkt nie; **nach einem erneuten Match steht die Anreicherung wieder**
(Test: Match, Enrich, manuelle Zuordnung, Enrich → Felder gleich); Quellenwechsel `tmdb → anime`
reiht `anime.enrich`, `anime → tmdb` reiht `metadata.match` und löscht `anilist_id`; 429 mit
`Retry-After` pausiert den Limiter; Liste grösser als 64 MB wird abgebrochen; kein Treffer schreibt
`anilist_id = 0`. Messung: 111 Anime-Titel vom NAS angereichert, Dauer und Anfragen je Host.

### Schritt 5 — Oberfläche
`feat(web): library families, content and metadata source per folder`

**Ziel:** Marco kann seine Tabelle in der Verwaltung anlegen, ohne eine Zeile Konfiguration.

**Berührte Dateien:** Abschnitt 7 vollständig: `LibrariesPage.tsx` (samt `:283`),
`LibraryFields.tsx`, `LibraryCard.tsx` (samt `:138`), `queries.ts:52`, `LibraryFoldersDialog.tsx`
(+ Dialog «Ordner ändern»), `PodcastsPage.tsx`, `AdminItemPage.tsx`, `SetupPage.tsx`, Locales,
Storybook-Stories, Unit-Tests, `web/e2e/admin-libraries.spec.ts`, `admin-library-folders.spec.ts`
(neuer Test «Ändern»), die gemischte e2e-Bibliothek «Dokus» mit eigenen Testordnern.

**DoD:** `task web:lint`, `task web:test`, `task web:storybook:test`, `task web:e2e` grün; der neue
Tastaturtest für «Ändern» nach Regel 8; `pnpm -C web test locales` grün; `grep -rn "'movies'"
web/src --include=*.tsx` ist leer und `grep -rn "'series'"` nennt nur Titelarten und Suchfilter
(`searchParams.ts:7`).

### Schritt 6 — Abnahme, Messung, Doku
`docs(dev,perf): library folders with content and source, acceptance on CT 190`

**Ziel:** Der Stand ist belegt, gemessen und beschrieben.

**Berührte Dateien:** `docs/dev/CURRENT_STATUS.md` (neuer Abschnitt), `docs/perf/bibliotheken.md`
(Erst-Scan Hörbücher und Podcast-Ordner auf CT 190: Dauer, ffprobe-Starts, Jobs, NFS-Latenz,
Speicher; AniList-Lauf), `docs/INSTALLATION.md:124` und `:65` bis `:73` (Assistent, Ordner),
`deploy/README.md`, `testdata/db/` (neuer eingefrorener Stand, 8.3), `docs/ideas/README.md` (Idee 01
Vermerk).

**DoD:** `task ci` grün. Abnahme durch Marco auf CT 190 nach 10.8: Zielstruktur angelegt, «Dokus»
zeigt Filme und Serien, Anime-Titel zeigen Originaltitel, Podcast-Ordner zeigt Episoden mit Datei und
neue aus dem Feed, ein Inhaltswechsel verlangt den getippten Pfad. Messwerte in `docs/perf/`.

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

Marco nimmt ab: Zielstruktur anlegen, Mischbibliothek, Anime-Titel, Podcast-Ordner, Inhaltswechsel
mit Bestätigung, Bibliotheksrechte eines Kinderprofils auf «Kids».

**7. Pull Request, CI, Merge.** PR gegen `main`, benennt den Bruch auf 1.3.0. `bind_pr`, CI nicht
pollen, `--auto` ist kein Tor. Merge erst mit grüner CI **und** Marcos Wort. Danach `main`
nachziehen, Image aus `main` bauen, CT 190 umstellen, `docs/dev/CURRENT_STATUS.md` nachführen.

---

## 11. Fragen an Marco

Format wie `docs/planning/04-entscheidungen.md`. F-1 bis F-8 sind im Interview vom 28. Oktober 2026
beantwortet; F-9 und F-10 sind offen.

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
| **F-9** | vor Schritt 4 | **Was darf AniList übersteuern?** Abschnitt 4.4 lässt AniList nur **füllen**, was TMDB leer lässt, plus Originaltitel und `anilist_id`. Für Anime wären die AniList-Beschreibungen oft die besseren, TMDB-Deutsch ist bei Anime häufig maschinell oder leer | **Füllen, nicht ersetzen** — bis Idee 01 die Anbieterreihenfolge je Ordner einstellbar macht. Dann wird «AniList vor TMDB» eine Einstellung statt einer Ausnahme im Code | | |
| **F-10** | vor Schritt 0 | **Reihenfolge:** erst die Benutzerverwaltung in `main`, dann dieser Zweig? Beide bauen Tabellen um (00022 `accounts`, 00023 `libraries`), beide heben den Vertrag brechend an | **Ja, Benutzerverwaltung zuerst.** Zwei parallele Tabellenumbauten auf derselben Datenbank von CT 190 sind ein Risiko ohne Nutzen; und die Nummern in diesem Papier (00023, 0015, 1.3.0) setzen es voraus | | |

---

## 12. Risiken

### 12.1 Der Titelindex einer gemischten Bibliothek

Der Scanner bindet Dateien an bestehende Titel über einen bibliotheksweiten Index mit **einer** Karte
je Schlüssel (`itemIndex`, `index.go:23` bis `:38`). Morgen stehen Filme und Serien in derselben
Bibliothek, und TMDB vergibt Film- und Serien-Ids aus getrennten Zahlenräumen; dieselbe Zahl kann
beides sein. Ein Index je Titelart (6.1) ist darum keine Verfeinerung, sondern die Bedingung dafür,
dass «Dokus» überhaupt funktioniert. Der Test in Schritt 1 nagelt genau den Kollisionsfall fest
(Film und Serie mit derselben `tmdb_id`). Eine echte Titel-Kollision zwischen `Doku/Filme` und
`Doku/Serien` gibt es auf dem NAS heute übrigens nicht; die erste Fassung dieses Papiers hatte eine
erfunden.

### 12.2 Ordner- und Feed-Episoden finden nicht zusammen

Audiobookshelf benennt Dateien nach dem Episodentitel, der Feed liefert GUIDs und andere Dateinamen,
und 11 von 239 Dateien eines Podcasts tragen als Tag-Titel den Podcast-Namen. Die Regel aus 4.5
(Datum auf die Minute, sonst Titel; nur bei eindeutigem Paar; nie der Podcast-Titel) nimmt dafür in
Kauf, dass ein Teil der Episoden **doppelt** bleibt — sichtbar in `listDuplicates` als
`file_and_feed`, nie falsch verheiratet. Besser würde es mit Episoden-Metadaten je Datei, die
Audiobookshelf in dieser Sammlung nicht schreibt (keine GUID im Tag, keine Episodenliste in der
`metadata.json`). Die Quote ist in Schritt 3 am echten Ordner zu messen und in `docs/perf/` zu
nennen.

### 12.3 Last durch die Audio-Ordner

26 031 Dateien im Podcast-Ordner und 62 Autorenordner mit M4B über NFS mit hoher Latenz. Der Preis
ist nicht das Lesen der Dateien (die Sonde liest keine Pakete, die Keyframe-Sonde überspringt Audio),
sondern rund 26 000 ffprobe-Starts über das Netz. Das sind auf CT 190 Stunden, und die Warteschlange
hat keinen Pause-Schalter; `SLOUCHCAT_WORKERS` bleibt der einzige Hebel. Der Erst-Scan ist darum ein
Messlauf mit eigener Datei in `docs/perf/`.

### 12.4 AniList, die Zuordnungsliste und das Fenster

Unsicherheiten, in 4.4 benannt: das Rate-Limit (90 laut Dokumentation, zeitweise weniger), die
Rohadresse, die Feldnamen und die Lizenz der Liste, die TMDB-Abdeckung bei Filmen. Dazu: die Liste
verschmilzt über `anidb_id`, und eine TVDB-Serie mit fünf Staffeln hat fünf AniList-Einträge — die
Wahl «Staffel 1» ist für Titel und Originaltitel richtig und für die Beschreibung der fünften Staffel
falsch; Staffeltexte sind Idee 01. Und das Fenster zwischen Match und Anreicherung (4.4): ein Titel,
den nur AniList als `isAdult` kennt, ist in diesem Fenster für ein Kinderprofil sichtbar. ADR 0015
benennt das als Restrisiko.

### 12.5 Was dieses Papier nicht zählen konnte

Die Stellen in `web/src`, die `'movies'` oder `'series'` als **Bibliotheksart** lesen, sind in
Abschnitt 7 vollständig genannt; aber `'series'` ist auch eine **Titelart** und ein **Suchfilter**
(`searchParams.ts:7`); ein mechanisches Ersetzen wäre falsch. Je Stelle ist zu entscheiden, wie in
der Benutzerverwaltung bei `owner`.

### 12.6 Ein alter Vorbehalt bleibt

`SLOUCHCAT_BROWSE_ROOTS=/` auf CT 190 hebt E14 auf: ein Podcast-Ordner liesse sich dort auf
`/config` anlegen, und der Scanner läse `slouchcat.db` als übergangene Datei. Das ist seit dem
25. Oktober 2026 bekannt und entschieden; dieses Konzept verschärft es nicht und entschärft es nicht.

---

## 13. Review-Protokoll (28. Oktober 2026)

Der OMC-Kritiker hat die erste Fassung (`d4eeee2`) am Code geprüft, über 60 Zeilenangaben
nachgeschlagen und 22 Befunde geliefert. Alle sind eingearbeitet:

| Befund | Schwere | Eingearbeitet in |
|---|---|---|
| Schrittschnitt nicht grün je Schritt (Scanner, Fixtures, Client-Typen hängen am Schema) | MAJOR | Abschnitt 10, Schritte 1 und 2 zusammengelegt |
| `netguard` lässt LAN-Feeds durch; «wird nicht gespeichert» hatte keinen Mechanismus | MAJOR | 4.3 `feed_origin`, 9.1 |
| `items_podcast_feed` kollidiert bei gleicher Feed-Adresse | MAJOR | 4.5 Übernahme über `PodcastFindByFeed` |
| Ordner-Podcasts ohne Feed würden abgefragt; Vorrang Feed/`metadata.json` offen | MAJOR | 4.5 |
| Zusammenführen über Titel allein trifft falsch (Tag-Titel = Podcast-Titel) | MAJOR | 4.5, 12.2 |
| Tags erst nach der Sonde bekannt | MAJOR | 4.5 `finishPodcastEpisode` |
| Kein Sweep für `podcast_episode`, kein Löschweg für Ordner-Podcasts | MAJOR | 4.5 «Verschwinden und Aufräumen» |
| Anreicherung wird vom nächsten Match gelöscht | MAJOR | 4.4 «nach jedem Match» |
| Quellenwechsel reiht nichts ein; keine Konvergenz | MAJOR | 4.4 Konvergenz, 6.3 |
| Titelindex unterbestimmt, Testfall erfunden | MAJOR | 6.1, 12.1 |
| TVDB-Id ist eine Spalte (`00008:23`) | MINOR | 4.3 |
| `Quota` statt `Limiter`; Karten je Art; Feldnamen prüfen | MINOR | 4.4 |
| Keyframe-Risiko überzeichnet | MINOR | 4.5, 12.3 |
| Offline bevorzugt die Datei schon; Abfrage heisst `GetVisibleEnclosure` | MINOR | 4.5 |
| Inhaltswechsel ohne `forgetRoot`, `settleScans`, `ignored_files` | MINOR | 6.3 |
| Gegenprobe in der Migration kann nicht abbrechen | MINOR | 4.6 Punkt 5 |
| Vorgabe `'tmdb'` sickert in drei weitere Einfügewege | MINOR | 4.3 |
| `DuplicateGroup.reason` braucht einen Wert | MINOR | 5.1 |
| Web: drei Podcast-Annahmen, Fixture-Liste, `library.root.path` | MINOR | 7 |
| Verweise (Provider-Konstanten, M6-Zeilen, `SearchRequest`, `fileInfo`, Wurzeldateien, DoD-grep) | MINOR | 1.1, 2.1, 4.5, 6.1, 8.3, 10 |
| Neue-Folgen-Hook und AN-2 für Ordner-Episoden undefiniert | MINOR | 4.5 |
| Idee 01 / Issue #8 / Herkunft je Feld | MINOR | 3, Schritt 0 |

Bestätigt und darum ohne Befund: `visible_items` und kein Trigger lesen `libraries`; nur
`library_roots`, `items`, `account_libraries`, `invitation_libraries` tragen Fremdschlüssel auf
`libraries`; Regel 2 (`depguard`) erlaubt `library` und `metadata` den Zugriff auf `systemq`, und
`library` importiert `metadata` nicht; Regel 10 (nur Schema und Host im Log) deckt sich mit
`netguard.Clean`.
