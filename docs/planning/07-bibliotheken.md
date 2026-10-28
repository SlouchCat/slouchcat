# Konzept: Bibliotheken (Ordner mit eigener Inhaltsart und Metadatenquelle)

- **Status: Entwurf, Interview geführt (28. Oktober 2026).** Marcos acht Antworten aus zwei
  Fragerunden stehen in Abschnitt 11 (F-1 bis F-8); zwei Fragen sind noch offen (F-9, F-10). Die
  Entscheide gehen als **Frage 13** ins Entscheidungsprotokoll (`docs/planning/04-entscheidungen.md`)
  und als **ADR 0015** nach `docs/adr/`, sobald Marco das Papier freigibt. Umgesetzt wird nichts,
  bevor das geschehen ist.
- **Datum: 28. Oktober 2026**
- **Wogegen dieses Konzept verankert ist:** `main`, Stand `0dd4f19` («Merge pull request #43»), im
  Worktree `claude/konzept-bibliotheken`. Jede Zeilenangabe bezieht sich auf diesen Stand. **Parallel
  läuft die Benutzerverwaltung** auf `claude/benutzerverwaltung` (Stand `5ece34d`, Konzept
  `06-benutzerverwaltung.md`): sie bringt Migration **00022**, ADR **0014** und Vertragsfassung
  **1.2.0**. Dieses Papier rechnet damit, dass sie **zuerst** in `main` geht (F-10), und nennt darum
  als nächste freie Nummern **Migration 00023, ADR 0015, Vertrag 1.3.0**. Geht sie später, rücken die
  drei Nummern um eins zurück; inhaltlich ändert sich nichts, denn die Benutzerverwaltung fasst
  weder `libraries` noch `library_roots` noch den Scanner an (`git diff --stat main 5ece34d --
  server/internal/library api/openapi.yaml` nennt nur den Vertrag, und dort nur Konten und Rollen).
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
> Bildarten, Anbieterreihenfolge, Übersteuerung je Titel) bleibt Sache von Idee 01
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
Folgen-Hook `QueuePendingSegments` nur für `series` (`:301`), und `walkRoot` bekommt
`lib.Kind` für **alle** Wurzeln gleich (`:446`). Im Paket `admin` prüft `validateLibrary`
(`server/internal/admin/admin.go:240`) Pfadpflicht und Altersstufe je Art, `ScanLibrary` (`:352`)
leitet Podcast-Bibliotheken auf `RefreshLibrary` um (`:357`), und `AddRoot` verweigert ihnen jeden
Ordner (`server/internal/library/roots.go:34`).

Die Metadaten wissen von der Bibliothek nichts. `match` (`server/internal/metadata/match.go:78`)
verzweigt nach der **Titelart**: Filme nur TMDB (`matchMovie`, `:122`), Serien TMDB und bei Leerlauf
TVDB (`matchSeries`, `:187`). Es gibt genau zwei Anbieter (`provider.go:12`, `ProviderTMDB`,
`ProviderTVDB`), beide mit serverweiten Schlüsseln. Hörbücher haben keinen Anbieter: Autor, Sprecher,
Reihe und Cover kommen aus Audiobookshelfs `metadata.json`, den Tags der ersten Datei und den
Ordnernamen (`server/internal/library/audiobook.go:24`, ADR 0007). Podcasts sind Feed-Container in
einer Bibliothek **ohne Wurzel** (ADR 0007, `docs/adr/0007-audio-modell.md:41`); lokale Podcast-Ordner
stehen dort ausdrücklich als offen (`:58`).

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
   Beschreibung, **`feedURL`**, `itunesId`, `language`, `explicit` und ein `cover.jpg`. Heute nimmt
   eine Podcast-Bibliothek keinen Ordner an.

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
Suche filtert nach `mediaTypes` auf Titelebene (`SearchRequest`, `api/openapi.yaml:8519` ff. auf dem
Zweig), die Startseite nach Reihen. Der einzige Zuschauer-nahe Ort, der eine Bibliotheksart sieht,
ist `listAccountLibraries` (`:857`, Stufe `account`, für die Bibliothekswahl in den Profilen) — und
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
| **ADR 0006** (`docs/adr/0006-metadaten-anbieter-trailer.md:24` bis `:33`): TMDB ist Hauptquelle und einzige Quelle für Filme; weitere Anbieter sind weitere Implementierungen der einen Schnittstelle | Ein `Provider` stellt die Identität fest | AniList wird **keine** Identitätsquelle (F-5): Staffeln, Folgen und IDs bleiben bei TMDB/TVDB. AniList ist eine **Anreicherung** nach dem Match (Abschnitt 4.4). Das ergänzt ADR 0006, es widerspricht ihm nicht; ADR 0015 benennt den Unterschied zwischen Identität und Anreicherung ausdrücklich, damit Idee 01 ihn übernimmt |
| **Idee 01** (`docs/ideas/01-metadaten-einstellungen.md:105` bis `:110`): Einstellungen je **Bibliothek** in `library_metadata (library_id PK, …)` | Schlüssel ist die Bibliothek | Mit diesem Konzept ist die natürliche Ebene der **Ordner**. Idee 01 bleibt «Ausgearbeitet», bekommt aber im Kopf einen Vermerk: Schlüssel wird `root_id`, und die Anbieterliste enthält `anilist`. Mehr ändert dieses Papier dort nicht |
| **Idee 02** (`docs/ideas/02-ordner-browser.md:3` bis `:6`, Kopf sagt «Schritte 12 bis 14 offen») | veraltet | Nur der Kopf wird in Schritt 0 nachgeführt; E8 bis E18 gelten unverändert |
| **Entscheid 9 / AN-2** (`04-entscheidungen.md:18`, `:43`): Standard-Altersstufe je Hörbuch- und Podcast-Bibliothek | bleibt | unverändert; die Stufe hängt weiter an der Bibliothek, nie am Ordner |
| **Harte Regel 11** (nichts vorziehen, keine Nicht-Ziele) | — | Anime und lokale Podcast-Ordner stehen in keiner Nicht-Ziel-Liste der Spec (`:55` bis `:68` nennt Musik, Fotos, Live-TV, DVR, Apps, Plugins, …). Beide gelten als Erweiterung von B-3 und H-2 und werden mit Frage 13 bewusst übernommen |

**Was weiterhin nicht gilt:** keine Bibliotheksnavigation für Zuschauer (2.1); keine Einstellungen je
Quelle (Idee 01); kein AniDB, kein MyAnimeList; kein externer Hörbuch-Anbieter (F-7); keine
Podcast-Suche (ADR 0007, offen); **kein Löschen einer Bibliothek** — es gibt heute keine
`DELETE FROM libraries`-Abfrage und keine Operation dafür, und dieses Papier führt sie nicht ein, weil
Marcos Zielstruktur mit Ordner-Umhängen erreichbar ist (Abschnitt 8.1).

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
`library.Service` geprüft (bei `Create`, `AddRoot`, `UpdateRoot`) und in der Migration als Gegenprobe
ausgeführt.

### 4.3 `items` lernt seine Quelle

`metadata.match` arbeitet je Titel und kennt die Bibliothek nicht (`match.go:78`). Ein Serien-Item hat
keine `media_files` (nur seine Folgen), kann seine Wurzel also nicht über `media_files.root_id`
finden, und ein Titel darf theoretisch Dateien in zwei Wurzeln derselben Bibliothek haben. Darum
bekommt jeder Titel seine Quelle **kopiert**, nicht abgeleitet:

```sql
ALTER TABLE items ADD COLUMN metadata_source TEXT NOT NULL DEFAULT 'tmdb'
    CHECK (metadata_source IN ('tmdb', 'anime', 'audiobook', 'podcast'));
ALTER TABLE items ADD COLUMN anilist_id INTEGER;
```

- Der Scanner schreibt `metadata_source` beim Anlegen aus der Wurzel (`titleItem`, `episodeItem`,
  `extraItem`, `audiobookItem` in `write.go:412` ff.; Podcasts in 4.5). Kinder (Staffel, Folge, Extra,
  Episode) erben den Wert des Elternteils.
- Ändert ein Administrator die Quelle einer Wurzel (F-4), setzt `UpdateRoot` den Wert auf allen
  Titeln, die eine Datei unter dieser Wurzel haben, samt Eltern und Kindern, und reiht
  `metadata.match` neu ein (Abschnitt 6.3).
- `anilist_id` steht neben `tmdb_id` und `imdb_id` (`00003_libraries_items.sql:59`, `:60`); die
  TVDB-Id liegt heute in `locked_fields` bzw. `external_ids` und bleibt dort.
- Feed-Podcasts ohne Ordner tragen `podcast`; Hörbücher `audiobook`. Beides wird in der Migration
  aus der Titelart gesetzt.

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

**Anreicherung danach.** Hat ein Titel mit Quelle `anime` nach dem Match eine `tmdb_id` oder
TVDB-Id, reiht `match` einen neuen Job `anime.enrich` ein (Priorität wie `image.fetch`, Schlüssel
`anime:<item>`). Der Job:

1. **löst die AniList-Id auf** über die Zuordnungsliste von Fribb (`anime-list-full.json`, Felder
   u. a. `anilist_id`, `tvdb_id`, `themoviedb_id`, `imdb_id`, `mal_id`, `anidb_id`, `season`,
   `episode_offset`, `type`; das Repository ist `github.com/Fribb/anime-lists`). Die Liste wird einmal
   geladen, mit ETag unter `/cache/anime-lists/` gehalten, wöchentlich erneuert und beim Parsen auf
   64 MB begrenzt; sie wird **gestreamt** gelesen und nur in ein `map[tmdb]→anilist` /
   `map[tvdb]→[]anilist` übersetzt, weil sie alle Anime kennt, nicht nur Marcos. Bei Serien kann eine
   TVDB-Id auf mehrere AniList-Einträge zeigen (eine AniList-«Season» je TVDB-Staffel, `season` und
   `episode_offset` sagen welche); der Job nimmt den Eintrag mit `season = 1`, bzw. je Staffel den
   passenden, wenn Staffeltexte angereichert werden.
2. **fragt AniList** (`https://graphql.anilist.co`, GraphQL, **ohne Schlüssel**) nach
   `Media(id)`: `title { romaji english native }`, `description`, `genres`, `coverImage.extraLarge`,
   `bannerImage`, `averageScore`, `format`, `isAdult`. Rate-Limit laut AniList-Dokumentation **90
   Anfragen je Minute** mit `X-RateLimit-Remaining` und `Retry-After`; AniList behält sich vor, das
   Limit bei Störungen zu senken, und Drittbibliotheken berichten zeitweise von 30. Der Client läuft
   darum über `ratelimit.Quota` (`server/internal/ratelimit/quota.go:36`) mit **60 je Minute** und
   gehorcht `Retry-After` und 429 ohne eigenen Zähler.
3. **schreibt**, was TMDB nicht hat, und überschreibt nichts, was TMDB hat (Regel «feldweise
   auffüllen» aus Idee 01, hier mit fester Reihenfolge TMDB → AniList):
   `anilist_id`; `original_title` = `title.native`, wenn leer; `item_texts` (`en`) Titel aus
   `title.english` (sonst `romaji`) und `overview` aus `description`, jeweils nur wenn der TMDB-Wert
   leer ist; `genres` als Vereinigung; Poster und Backdrop aus `coverImage`/`bannerImage` nur, wenn
   `image.fetch` von TMDB **kein** Bild bekommen hat. `isAdult = true` setzt `own_rating` auf 18 mit
   `rating_source 'anilist'`, sofern keine strengere Quelle vorliegt (K-3: nie senken). Was AniList
   darüber hinaus übersteuern darf, ist F-9.
4. **markiert den Zustand** je Titel wie `trailer_state`: eine neue Spalte braucht es nicht,
   `metadata_state` bleibt der TMDB-Zustand; der Job schreibt `anilist_id = 0` für «nachgeschlagen,
   kein Treffer», damit er nicht bei jedem Scan neu fragt, und `NULL` für «noch nie».

Was fehlt und bewusst fehlt: eine **Titelsuche bei AniList** für Ordner, die TMDB nicht kennt. Ohne
TMDB-Identität gäbe es heute weder Credits noch Trailer noch die Bildpipeline; ein solcher Titel bleibt
`unmatched` und erscheint in den Pflegewerkzeugen (B-9) wie jeder andere. Das gehört in Idee 01, wenn
AniList dort ein vollwertiger Textanbieter wird.

Zwei Dinge, die vor Schritt 5 **gegen die Quelle zu prüfen** sind, weil dieses Papier sie nicht
verifizieren konnte: die Rohadresse der Zuordnungsliste (erwartet
`https://raw.githubusercontent.com/Fribb/anime-lists/master/anime-list-full.json`) und ihre Lizenz
für die Weiterverwendung in einem AGPL-Projekt; sowie ob Filme in der Liste zuverlässig
`themoviedb_id` tragen (das Repository verschmilzt über `anidb_id`, Filme sind dort seltener als
Serien). Fehlt die TMDB-Zuordnung eines Films, bleibt er ohne AniList-Daten — kein Fehler, ein
Pflegefall.

### 4.5 Die Quelle `podcast`: Ordner lesen, Feed mitnehmen

Ein Ordner mit `content = 'podcast'` wird so gelesen (alles im Paket `library`, über `paths`, Regel 4):

- **Jeder Unterordner der ersten Ebene ist ein Podcast** (`items.kind = 'podcast'`), so wie bei
  Hörbüchern jeder Buchordner ein Buch ist. Dateien direkt in der Wurzel werden ignoriert und wie
  heute unter «ignorierte Dateien» geführt.
- **`metadata.json`** (Audiobookshelf-Schema, Beispiel `NZZ Megahertz`: `title`, `author`,
  `description`, `releaseDate`, `genres`, `feedURL`, `imageURL`, `itunesId`, `language`, `explicit`,
  `podcastType`) wird mit derselben Grössenbremse gelesen wie bei Büchern (`audiobook.go:188`,
  `maxMetadataJSON`). Sie füllt Titel, `author`, Beschreibung (`item_texts` in `language`, sonst
  `en`), `genres`, `explicit` (→ FSK 18, `rating_source 'tags'`, wie ADR 0007 es für Bücher tut) und
  **`feed_url`**. Fehlt sie, ist der Ordnername der Titel.
- **`cover.jpg`/`folder.jpg`/`poster.jpg`** wird wie bei Büchern `cover_path`.
- **Jede Audiodatei im Podcast-Ordner ist eine Episode** (`podcast_episode`) mit einer Zeile in
  `media_files` (`root_id`, `rel_path`, `part_index NULL`). Titel aus dem ID3-/MP4-Tag `title`, sonst
  Dateiname; `published_at` aus dem Tag `date`/`TDRC`, sonst `mtime`; `duration_ms` aus der Sonde.
  Die Identität einer Ordner-Episode ist `root_id + rel_path` (das ist der `UNIQUE` von
  `media_files`), **nicht** die GUID — die hat eine lokale Datei nicht.
- **Feed mitabonnieren (F-6):** Steht eine `feedURL` in der `metadata.json`, setzt der Scanner
  `feed_url` am Podcast und reiht `podcast.fetch` ein. Der Feed läuft durch **denselben** Weg wie ein
  von Hand abonnierter: `netguard`, bedingtes GET, 2000-Episoden-Grenze, 55-Minuten-Ticker (ADR 0007).
  Es gibt also eine Podcast-Zeile mit Feed **und** Dateien.
- **Zusammenführen von Feed- und Ordner-Episoden:** Der Feed-Abruf (`podcast.go:472` ff.) legt heute
  je GUID eine Episode an. Neu prüft er vor dem Anlegen, ob der Podcast eine **dateigestützte**
  Episode ohne GUID hat, deren Dateiname dem Dateinamen der Enclosure-URL entspricht oder deren
  normalisierter Titel (Kleinschreibung, ohne Satzzeichen) gleich ist; trifft eines zu, bekommt diese
  Episode `guid`, `enclosure_url`, `published_at` und den Feed-Text, statt dass eine zweite entsteht.
  Umgekehrt hängt der Scanner eine neue Datei an eine bestehende Feed-Episode, wenn dieselbe Regel
  trifft. Was nicht zusammenfindet, bleibt zwei Episoden — sichtbar, nicht still verschluckt; die
  Pflegewerkzeuge zeigen «Episode doppelt (Datei und Feed)» als neue Zeile in `listDuplicates`.
- **Wiedergabe und Download bevorzugen die Datei.** `GetVisiblePodcastEnclosure`
  (`server/queries/scoped/podcasts.sql:28`) ist nur noch der Rückfall; hat die Episode eine
  `media_file`, läuft sie wie ein Hörbuchteil (direkt oder Audio-HLS, `transcode`), und der Download
  encodiert aus der Datei (`downloads.media_file_id`) statt aus `source_url`. Die Trigger für
  `items.available` decken `podcast_episode` mit Datei seit 00014.

**Grenzen, die jetzt gelten müssen und nicht erst auffallen dürfen:** Marcos Podcast-Ordner hat
**26 031 Audiodateien**. Jede neue Datei reiht heute `probe.file` ein (ffprobe über NFS), und bei
Video zusätzlich `probe.keyframes`, das die Datei **ganz** liest. Der Keyframe-Index ist für Audio
sinnlos und wird für `podcast_episode` und `audiobook` **nicht** eingereiht (für Hörbücher ist das zu
prüfen — `probe.go:173` ruft `finishAudiobook`; die Keyframe-Sonde darf dort nicht laufen). Die
Dauer-Sonde bleibt, aber mit `-show_format` ohne Paketlesen, damit sie je Datei Kilobytes liest statt
Megabytes. Der M3-Lasttest kannte 45 000 Dateien; 26 000 kleine MP3 auf NFS bei 45 MB/s
(`docs/dev/CURRENT_STATUS.md`, NAS-Stau vom 27. Oktober 2026) sind kein Grössen-, sondern ein
Latenzproblem. Die Messung gehört nach `docs/perf/bibliotheken.md` (Schritt 7).

### 4.6 Migrationsplan 00023 (Schritte, kein fertiges SQL)

Nach `CLAUDE.md` Regel 5 und `server/migrations/TEMPLATE.sql.txt`: nur vorwärts, `-- +goose NO
TRANSACTION`, `PRAGMA foreign_keys=OFF`, Änderungen zwischen `BEGIN` und `COMMIT`, am Ende
`PRAGMA foreign_key_check` und `PRAGMA foreign_keys=ON`. Der Server sichert vor dem Lauf mit
`VACUUM INTO`.

1. **`library_roots` erweitern** (4.2) mit `ADD COLUMN` und Vorgaben. Dann befüllen aus der
   Bibliothek: `UPDATE library_roots SET content = CASE l.kind WHEN 'movies' THEN 'movie' WHEN
   'series' THEN 'series' WHEN 'audiobooks' THEN 'audiobook' END, metadata_source = CASE l.kind WHEN
   'audiobooks' THEN 'audiobook' ELSE 'tmdb' END FROM libraries l WHERE l.id = library_id`. Eine
   Podcast-Bibliothek hat heute keine Wurzel (`roots.go:34`), also keine Zeile zu befüllen.
2. **`items` erweitern** (4.3): `metadata_source` nach Titelart (`audiobook` → `audiobook`;
   `podcast`, `podcast_episode` → `podcast`; alles andere `tmdb`), `anilist_id NULL`.
3. **`libraries` neu bauen** mit `kind IN ('video', 'audiobooks', 'podcasts')`; Zeilen kopieren mit
   `CASE kind WHEN 'movies' THEN 'video' WHEN 'series' THEN 'video' ELSE kind END`, `id` erhalten;
   tauschen; Indizes neu anlegen. **Vor dem Schreiben die Spaltenliste gegen `PRAGMA
   table_info(libraries)` auf dem eingefrorenen M11-Stand abgleichen** (dieselbe Vorsicht wie in
   `06-benutzerverwaltung.md` 4.6).
4. **Sicht `visible_items`:** sie liest `items`, `profiles`, `accounts`, `profile_libraries` und
   `account_libraries`, aber **nicht** `libraries` (`00010_container_visibility.sql:148` bis `:167`,
   auf dem Zweig wortgleich in `00022:75` bis `:94`). Sie muss **nicht** fallen. Das ist in Schritt 1
   mit `SELECT sql FROM sqlite_master WHERE name = 'visible_items'` zu belegen, bevor darauf gebaut
   wird; liest sie doch `libraries`, gilt der Weg aus 00022 (ablegen, wortgleich neu anlegen).
5. **Gegenprobe im selben Lauf:** kein `library_roots`-Eintrag, dessen `content` nicht zur Familie
   seiner Bibliothek passt (Matrix aus Abschnitt 2); kein `libraries.kind` ausserhalb der drei Werte;
   `PRAGMA foreign_key_check` leer. Schlägt sie fehl, bricht die Migration vor `COMMIT` ab.
6. **Kein Datenverlust an Kindtabellen:** `items`, `library_roots`, `profile_libraries` (ohne FK,
   `00002:43`), `account_libraries`, `invitation_libraries` zeigen über `id` auf `libraries`; die `id`
   bleibt, die Fremdschlüssel sind während des Laufs aus.

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
| `LibraryRequest` (`:7781`) | `name, kind, path?, defaultRating?` | `name, kind, defaultRating?, root?: LibraryRootRequest` — der erste Ordner bringt seine Inhaltsart und Quelle mit; Pflicht für `video` und `audiobooks`, verboten für `podcasts` **nur dann nicht mehr**, wenn er `content: podcast` trägt |
| `LibraryRootRequest` (`:7805`) | `path` | `path, content, metadataSource` (alle Pflicht) |
| `LibraryRootUpdate` | — | neu: `content?, metadataSource?, confirmPath?` (5.2) |
| `LibraryRoot` (`:7878`) | `id, path, fsType, watch, itemCount, fileCount, warning?` | zusätzlich `content`, `metadataSource` |
| `LibraryUpdate` (`:7831`) | `name?, defaultRating?` | unverändert — die Familie ist nach dem Anlegen fest (wie heute `kind`) |
| `RootContent` | — | neu: `[movie, series, audiobook, podcast]` |
| `MetadataSource` | — | neu: `[tmdb, anime, audiobook, podcast]` |
| `MetadataProvider` (`:7494`) | `[tmdb, tvdb]` | unverändert — das ist die Identitätsquelle für `searchMetadata` und die manuelle Zuordnung; AniList gehört nicht hinein (4.4) |
| `ItemRef`, `ItemDetail` | — | **unverändert.** `anilistId` wird nicht ausgeliefert; ein Zuschauer braucht es nicht, und Fremd-Clients bekommen es, wenn Idee 01 die Herkunft je Feld ausliefert |
| `AdminItem` | `libraryId, …` | zusätzlich `metadataSource` und `anilistId?`, damit die Titelpflege zeigt, woher Texte kommen |
| `AdminPodcast` (`:7292`) | Feed-Daten | zusätzlich `rootId?` und `localEpisodeCount`, damit die Podcast-Seite «aus Ordner» zeigen kann |
| `SetupRequest.library` (`:127`, `runSetup`) | `LibraryRequest` | folgt automatisch; der Assistent wählt Inhalt und Quelle des ersten Ordners |

### 5.2 Operationen

| Operation | Änderung | `x-visibility` |
|---|---|---|
| `createLibrary` (`:2157`) | Körper nach 5.1; 422 `content_not_allowed`, `source_not_allowed`, weiterhin `library_path_invalid` | admin |
| `addLibraryRoot` (`:2368`) | Körper nach 5.1; dieselben 422; Podcast-Bibliotheken nehmen Ordner mit `content: podcast` an | admin |
| **`updateLibraryRoot`** — `PATCH /admin/libraries/{id}/roots/{rootId}` | **neu** (F-4). Ändert `content` und/oder `metadataSource`. Ein Wechsel der **Quelle** allein reiht `metadata.match` für die Titel der Wurzel neu ein, löscht nichts und braucht keine Bestätigung. Ein Wechsel der **Inhaltsart** ist in der Wirkung «Ordner entfernen und neu anlegen» (6.3) und verlangt darum `confirmPath` wie `removeLibraryRoot` (422 `confirm_mismatch`), 409 `scan_running` | admin |
| `scanLibrary` (`:2268`) | Für `podcasts`: Ordner-Scan **und** Feed-Abruf; heute nur Letzteres (`admin.go:357`) | admin |
| `listAccountLibraries` (`:857`) | nur neue `kind`-Werte | account |
| `searchMetadata` (`:3142`) | unverändert; `kind: [movie, series]` ist die Titelart | admin |
| `listDuplicates` | neue Gruppenart «Episode: Datei und Feed» (4.5) | admin |

Jede neue oder geänderte Operation bekommt ihren Eintrag im Vertragstest
(`TestContractCoversEveryOperation`, `server/internal/api/contract_access_test.go:354` auf dem Zweig)
und den Schema-Test auf `x-visibility` (Regel 7). Neue Fehlercodes gehen in die `problem`-Liste des
Vertrags und in `web/src/locales/*.json`.

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
(`:446`). Neu bekommt `walkRoot` die **Wurzel** (`root.Content`, `root.MetadataSource`) statt der
Bibliothek. Die Stellen, die heute `lib.Kind` lesen, lesen den Inhalt der Wurzel:

| Stelle | heute | neu |
|---|---|---|
| `parsePath` (`scanner.go:169`) | `kind` der Bibliothek | `content` der Wurzel; `podcast` → eigener Pfad (4.5) |
| `scannedExt` (`:181`) | Audio bei `audiobooks` | Audio bei `audiobook` **und** `podcast` |
| `wanted` (`:606`) | Serienbibliothek verwirft Dateien ohne Folgenmarke | Serien**wurzel** |
| `videoFile` (`:632`), `listDirFiles` (`pathscan.go:277`) | Bibliotheksart | Wurzelinhalt |
| Batchgrösse (`:511`) | kleiner bei `series` | kleiner, sobald **eine** Wurzel `series` ist |
| `QueuePendingSegments` (`:301`) | nur `series`-Bibliotheken | sobald eine Wurzel `series` ist |
| `batchWriter.item` (`write.go:364`) | `w.lib.Kind` | `info.root.Content`; die `fileInfo` trägt die Wurzel schon (`rootID` in `index.go:69` bis `:84`) |
| `describe` (`write.go:52`) | Bibliotheksart | Wurzelinhalt |
| `titleKindOf`, `fileFolderKey` (`index.go:86`, `:99`) | je Bibliothek | je Wurzel; der Titelindex `ListItemsByLibrary` (`system/items.sql:16`) bleibt bibliotheksweit, wird aber **nach Titelart getrennt** gehalten, damit ein Film «Alone (2015)» im Filmordner nicht an die Serie «Alone (2015)» im Serienordner derselben Bibliothek gebunden wird (Doku-Fall, beide existieren auf dem NAS) |
| `admin.validateLibrary` (`admin.go:240`) | Pfad je Art | Matrix aus Abschnitt 2 |
| `admin.ScanLibrary` (`:352`) | Podcasts → nur Feed | Podcasts → Ordner-Scan, dann Feed-Abruf |
| `library.Create` (`service.go:189`), `AddRoot` (`roots.go:26`) | Art prüfen, Podcasts ohne Wurzel | Matrix prüfen; Podcasts mit `content: podcast` |
| `maintenance.go:429` (Zusammenführen nur gleiche Art und gleiche Bibliothek) | unverändert | unverändert — Titelart, nicht Wurzel |
| Webhooks (`integrations/pathmap.go:66`) | Pfad → irgendeine Wurzel | unverändert; Radarr trifft Filmwurzeln, Sonarr Serienwurzeln, weil Marco die Umsetzungstabelle so pflegt. Eine Prüfung «Radarr darf nur auf `movie`-Wurzeln» wäre eine Sicherheitsgewinn-Illusion und bleibt aus |

`scan.path` (`jobs.go:17` ff.) trägt `rootId` bereits je Verzeichnis; der Watcher läuft je Wurzel
(`service.go:807` ff.). Beides ändert sich nicht.

### 6.2 Metadaten

- `match` (`match.go:78`) liest `items.metadata_source`. Für `tmdb` und `anime` läuft derselbe
  Weg; bei `anime` reiht der erfolgreiche Match `anime.enrich` ein (4.4). `audiobook` und `podcast`
  werden wie heute übersprungen (`:101`, `default`).
- Neues Paket? **Nein.** Der AniList-Client wird `server/internal/metadata/anilist.go` neben
  `tmdb.go` und `tvdb.go`; die Zuordnungsliste `anime_lists.go` im selben Paket, Cache unter
  `/cache/anime-lists/`. Beide dürfen nur über `netguard`-freie, feste Hosts sprechen
  (`graphql.anilist.co`, `raw.githubusercontent.com`); ein Schalter `SLOUCHCAT_ANILIST=off` schaltet
  die Anreicherung serverweit ab, ohne dass die Quelle `anime` am Ordner verschwindet (die Titel
  bleiben dann TMDB-Titel). Die Zahl der Anreicherungen steht im Dashboard neben Trailern.
- `podcast.Subscribe` (`podcast.go:214`) prüft weiter die Art `podcasts`; neu akzeptiert
  `PodcastInsert` (`system/podcasts.sql:12`) einen Podcast mit `feed_url NULL` (reiner
  Ordner-Podcast) — die Spalte ist schon `NULL`-fähig (`00014_audio.sql:71`), nur der eindeutige Index
  `items_podcast_feed (library_id, feed_url)` (`00014:83`) ist dann ohne Wirkung, was SQLite für
  `NULL` ohnehin so handhabt.

### 6.3 `UpdateRoot`: Quelle wechseln ist billig, Inhalt wechseln ist ein Umzug

**Quelle wechseln** (`tmdb` ↔ `anime`): `UPDATE items SET metadata_source = ?` für alle Titel mit
einer Datei unter der Wurzel (Join über `media_files.root_id`), dazu deren Eltern und Kinder; dann
`QueuePendingMetadata` für diese Titel mit zurückgesetztem `anilist_id` (`NULL`). Keine Löschung,
kein Scan, keine Bestätigung. Hörbuch- und Podcast-Wurzeln haben nur eine Quelle; dort ist der Aufruf
ein 422 `source_not_allowed`.

**Inhalt wechseln** (`movie` ↔ `series`; andere Wechsel verbietet die Matrix): unter der Scan-Sperre
der Bibliothek, in **einer** Transaktion, in dieser Reihenfolge — es ist der Löschweg aus Idee 02
(`roots.go:80`, `RemoveRoot`) **ohne** den letzten Schritt:

1. Titel der Wurzel ohne Datei unter einer anderen Wurzel weich löschen
   (`SoftDeleteItemsOfRootWithoutOtherFiles`, `system/library.sql:73`), danach leere Staffeln und
   Serien.
2. Downloads der überlebenden Titel umhängen (`MoveDownloadsOffRoot`, `:86`).
3. `DELETE FROM media_files WHERE root_id = ?` — die Kaskade räumt die Downloads der weichen Titel
   ab, die Trigger setzen `items.available`.
4. `UPDATE library_roots SET content = ?, metadata_source = ?`.
5. Nach `COMMIT`: `scan.library` einreihen. Der Scan legt die Titel neu an; ein Titel mit
   Provider-Id, der binnen 30 Tagen wieder auftaucht, kommt samt Fortschritt zurück
   (`relinkWindow`, `library/scanner.go:40`, `library/write.go:415`) — **aber nur, wenn seine Art
   gleich bleibt**, und genau das ist hier nicht der Fall. Ein Film, der zur Folge wird, ist ein neuer
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
| **Geteiltes Formular**, `web/src/features/shared/LibraryFields.tsx` | `KINDS` `:10`, `RadioGroup` `:76` bis `:82`, Pfad `:89` bis `:105`, `libraryRequest` `:23` bis `:36` | Trägt die neuen Felder und baut `LibraryRequest.root`. Das Formular ist mit dem **Einrichtungsassistenten** geteilt (`web/src/features/setup/SetupPage.tsx:127`, `:210`, `:277`); der Assistent bekommt damit dieselben zwei Felder, ohne eigenen Code. Vorgabe bleibt Video / Filme / TMDB; der Vorgabename `t('library.kind.movies')` (`:277`) wird «Filme» aus einem neuen Schlüssel, weil `library.kind.movies` verschwindet |
| **Bibliothekskarte**, `web/src/features/admin/LibraryCard.tsx` | Chip `:57`, `Folders` `:128` bis `:171` | Der Art-Chip zeigt die Familie; bei mehreren Ordnern steht statt «2 Ordner» eine Zusammenfassung «1 Filme, 1 Serien». Bei genau einem Ordner wie heute Pfad, Dateisystem, Überwachung, neu plus Inhalt und Quelle |
| **Ordner von X**, `web/src/features/admin/LibraryFoldersDialog.tsx` | Hinzufügen `:64`, Formular `:174` bis `:198`, Entfernen `:86` | Jede Zeile zeigt zwei Chips (Inhalt, Quelle) und bekommt neben «Entfernen» ein **«Ändern»**. Das Hinzufügen-Formular bekommt die zwei Felder. «Ändern» öffnet einen Dialog mit denselben zwei Feldern; wechselt der Inhalt, erscheint darunter die abgetippte Bestätigung (die `TypedConfirmation`-Variante von `ConfirmDialog` aus Schritt 13 der Idee 02) mit dem Text aus 6.3 |
| **Podcasts**, `web/src/features/admin/PodcastsPage.tsx` | `:129` filtert `kind === 'podcasts'` | Bleibt die Seite für Feed-Abos. Podcasts aus Ordnern erscheinen in derselben Liste mit dem Chip «aus Ordner» und der Zahl lokaler Episoden (`AdminPodcast.rootId`, `localEpisodeCount`); «Feed entfernen» ist bei ihnen ausgeblendet, denn ihr Ordner bestimmt sie |
| **Titelpflege**, `web/src/features/admin/AdminItemPage.tsx` | Match-Bereich `:357`, `:409` | Zeigt die Quelle des Titels und bei `anime` die AniList-Id als Link; die manuelle Zuordnung bleibt TMDB/TVDB |
| **Bibliothekswahl in Profilen und Zugang**, `useLibraryOptions.ts:35` | liest nur `id`, `name` | **keine Änderung** |
| **Zuschauer** | — | **keine Änderung** (2.1) |

**Regel 8 — Tastaturtests.** Es entsteht keine neue Route, aber zwei Formulare wachsen um je zwei
Felder und ein Dialog kommt dazu; in der Wirkung ist «Ändern» eine neue Seite. Darum:
`web/e2e/admin-libraries.spec.ts` (Tab-Reihenfolge `:27`, Audio-Arten `:48`) wird auf die neue
Artauswahl und die zwei Felder erweitert; `web/e2e/admin-library-folders.spec.ts` bekommt einen
Test für «Ändern» (Pfeiltasten in beiden Radiogruppen, Tab in die Bestätigung, Enter, Escape, Zurück,
sichtbarer Fokus, keine Fokusfalle); `web/e2e/setup.spec.ts:115` («arrow keys switch the library
kind») folgt der neuen Auswahl. Die Fixtures in `web/e2e/global-setup.ts:55`, `:61`, `:125`, `:128`
legen die Bibliotheken mit dem neuen Körper an; dazu eine gemischte Bibliothek «Dokus» mit einem
Film- und einem Serienordner, damit der Mischfall im e2e-Lauf vorkommt.

**Regel 9 — Texte und Farben.** Neue Schlüssel in `web/src/locales/de.json` und `en.json`:
`library.kind.{video,audiobooks,podcasts}` (die vier alten `library.kind.*`-Schlüssel `:242` bis
`:247` verschwinden mit den Werten), `library.content.{movie,series,audiobook,podcast}`,
`library.source.{tmdb,anime,audiobook,podcast}` mit Kurzbeschreibungen,
`admin.libraries.folders.change.*` (Dialog und Bestätigungstext), `admin.podcasts.fromFolder`,
`setup.library.content`, `setup.library.source`, die neuen Fehlercodes. `pnpm -C web test locales`
prüft die Symmetrie. Chips kommen aus `web/src/ui`.

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
bestehenden Löschweg. Für die Hörbücher und Podcasts gilt die Grenze aus 4.5 (26 000 Dateien,
NAS-Latenz); der erste Scan dieser zwei Bibliotheken ist ein Messlauf (Schritt 7), kein
Nebenbei.

### 8.2 Wo alte Clients scheitern

Die Web-App des Standes 1.2.0 sendet `LibraryRequest.path` und erwartet `kind: movies`; gegen einen
1.3.0-Server scheitert nur die Bibliotheksseite der Verwaltung mit 422 (fehlender `root`), nichts auf
der Zuschauerseite. Weil Web-App und Server aus demselben Image kommen, ist das nur für den Moment
zwischen Image-Wechsel und Neuladen relevant; der Service Worker lädt bei Versionswechsel neu.

### 8.3 Eingefrorene Datenbankstände

`testdata/db/` hält M1a bis M11 (`TestFrozenStatesMigrate`, `server/internal/store/frozen_test.go:23`).
Migration 00023 muss über **jeden** laufen; M6 und M7 sind die wichtigen, weil sie alle vier alten
Arten enthalten, darunter eine Podcast-Bibliothek ohne Wurzel (`testdata/db/M6.sql:215` bis `:217`).
`PRAGMA foreign_key_check` muss nach jedem Stand leer sein. Nach der Umsetzung kommt ein neuer Stand
dazu (`test(store): freeze the … database state`, Vorbild `5bc6ce7`) mit einer gemischten
Video-Bibliothek, einem Anime-Titel mit `anilist_id` und einem Ordner-Podcast mit Feed und Datei.
Liegt die Benutzerverwaltung bis dahin in `main`, ist ihr eingefrorener Stand (mit 00022) Teil der
Reihe und 00023 läuft auch über ihn.

---

## 9. Sicherheit

### 9.1 Dateien aus dem Ordner bestimmen keine Adresse ohne Wächter

Die `feedURL` in einer `metadata.json` ist Inhalt einer Datei auf dem NAS — nicht die Eingabe eines
Administrators. Der Server ruft sie trotzdem ab (F-6). Das ist derselbe Fall wie ein Feed, der auf
Enclosures in fremden Netzen zeigt, und wird gleich behandelt: `netguard` verweigert Loopback,
private, Link-Local- und unspezifizierte Adressen **nach** der DNS-Auflösung (ADR 0007); nur `http`
und `https`; dieselben Grössen- und Weiterleitungsgrenzen wie `Subscribe`. Zusätzlich schreibt der
Scanner beim ersten Übernehmen einer `feedURL` eine `Info`-Zeile mit Bibliothek, Podcast und Host
(nicht dem vollen Pfad, Regel 10). Eine `feedURL`, die `netguard` ablehnt, wird **nicht** gespeichert
und erscheint als Pflegehinweis «Feed-Adresse aus Datei nicht zulässig».

### 9.2 Lesen nur über `paths`, mit Grenzen

`metadata.json` und Cover werden wie bei Büchern über das `*os.Root` der Wurzel gelesen, mit
`maxMetadataJSON` als Bremse; Symlinks nach aussen scheitern an `paths` (Regel 4). Ein Podcast-Ordner
mit 100 000 Dateien ist kein Sicherheits-, sondern ein Lastfall (4.5).

### 9.3 Zwei neue Aussenverbindungen, beide ohne Geheimnis

AniList braucht keinen Schlüssel, die Zuordnungsliste auch nicht; es gibt nichts zu redigieren. Beide
Hosts sind fest im Code, nicht konfigurierbar, damit kein Setting eine Adresse im Heimnetz nennen
kann. Antworten werden mit Grössenbremse gelesen (GraphQL 1 MB, Liste 64 MB); ein `Retry-After` wird
gehorcht, 429 setzt die Quote auf null bis zum nächsten Fenster. Fällt AniList aus, bleiben die Titel
TMDB-Titel; der Job meldet den Fehler in den Fehlerring, nicht an den Zuschauer.

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

Reihenfolge nach Baubarkeit: Papiere, Schema, Vertrag, Scanner, dann die zwei neuen Quellen, dann die
Oberfläche, dann Abnahme. Jeder Schritt ein Commit, `task ci` grün. **Beginn erst, wenn die
Benutzerverwaltung in `main` ist** (F-10), damit die Nummern stimmen.

### Schritt 0 — Entscheide und Papiere
`docs(planning,adr): record the library decisions and ADR 0015`

**Ziel:** Marcos Antworten F-1 bis F-10 stehen verbindlich im Entscheidungsprotokoll, der neue
Wortlaut zu B-1 und ADR 0007 ebenso.

**Berührte Dateien:** `docs/planning/04-entscheidungen.md` (neue **Frage 13** nach dem Muster von
Frage 12, dazu eine Zeile in «Weitere Entscheide»), `docs/adr/0015-bibliotheken-ordner.md` (neu:
Familie, Ordnerwerte, Matrix, Identität versus Anreicherung, Podcast-Ordner, Inhaltswechsel),
Ablösungshinweise in `docs/adr/0007-audio-modell.md` (Zeile «ohne Wurzel», offener Punkt) und
`docs/adr/0006-metadaten-anbieter-trailer.md` (Anreicherung), Kopfvermerk in
`docs/ideas/01-metadaten-einstellungen.md` (Schlüssel `root_id`, Anbieter `anilist`), Kopf von
`docs/ideas/02-ordner-browser.md` (Schritte 12 bis 14 sind gebaut), dieses Papier.

**DoD:** `git diff --stat docs/spec/` ist leer. Frage 13 trägt alle zehn Antworten mit Datum. ADR 0015
nennt B-1, B-3, H-1, H-2, AN-2, C-5. `task ci` grün.

### Schritt 1 — Schema und Datenmodell
`feat(store): put content and metadata source on the library root`

**Ziel:** Die Datenbank kennt Familie, Inhaltsart und Quelle; nichts im Vertrag ändert sich noch.

**Berührte Dateien:** `server/migrations/00023_library_roots_content.sql` (neu, Plan 4.6),
`server/queries/app/libraries.sql` (`InsertLibrary`, `InsertLibraryRoot`, `ListLibraryRoots`, neue
`UpdateLibraryRoot`, `UpdateItemsMetadataSourceByRoot`), `server/queries/system/items.sql`
(`InsertItem` mit `metadata_source`), `server/queries/system/library.sql`
(`DeleteMediaFilesOfRoot`), `server/internal/model/model.go` (`:975` bis `:991`: neue Konstanten
`LibraryVideo`, `RootContent*`, `MetadataSource*`, `ValidRootPair`; `LibraryRequest` `:994` bekommt
`Root`; `LibraryRoot` `:1017` die zwei Felder), danach `task gen`.

**DoD:** `task ci` grün. `go test ./internal/store -run TestFrozenStatesMigrate` grün über alle
Stände, `foreign_key_check` leer. Neue Tests: eine Wurzel mit `content` ausserhalb der Matrix wird von
`ValidRootPair` abgelehnt; M6-Stand nach Migration hat vier Bibliotheken mit `video`, `video`,
`audiobooks`, `podcasts` und Wurzeln `movie/tmdb`, `series/tmdb`, `audiobook/audiobook`; `items` einer
Hörbuchbibliothek tragen `audiobook`. `git grep -n "'movies'\|'series'" -- server/queries` nennt nur
noch Titelarten. Beleg aus Schritt 4.6 Punkt 4 (`visible_items` liest `libraries` nicht).

### Schritt 2 — Vertrag 1.3.0
`feat(api): library families, root content and metadata source, updateLibraryRoot`

**Ziel:** Der Vertrag beschreibt das Zielbild; die Handler setzen die Matrix durch.

**Berührte Dateien:** `api/openapi.yaml` (5.1, 5.2, `info.version: 1.3.0`),
`server/internal/api/admin_handlers.go` (`:146` und die Wurzel-Handler), `contract_access_test.go`
(Einträge), `server/internal/admin/admin.go` (`validateLibrary` `:240`, `CreateLibrary` `:343`,
`ScanLibrary` `:352`), `server/internal/library/service.go` (`Create` `:189`), `roots.go`
(`AddRoot` `:26`, neu `UpdateRoot` mit beiden Wegen aus 6.3), `cmd/apicheck/fixtures.go`,
`packages/api-client` über `task gen`.

**DoD:** `task ci` grün (`gen:check` ohne Drift). Vertragstests: 422 `content_not_allowed` für
`audiobook` in einer Video-Bibliothek, `source_not_allowed` für `anime` an einem Hörbuchordner,
`confirm_mismatch` und 409 `scan_running` beim Inhaltswechsel, 200 beim Quellenwechsel ohne
Bestätigung; eine Podcast-Bibliothek nimmt einen Ordner mit `content: podcast` an und lehnt `movie`
ab. Der Schema-Test findet an jeder neuen Operation `x-visibility`. Release-Notes-Entwurf mit dem
Bruch (`LibraryKind`, `LibraryRequest.root`).

### Schritt 3 — Der Scanner liest je Wurzel
`feat(library): scan each root by its own content and keep mixed libraries apart`

**Ziel:** Eine Video-Bibliothek mit Film- und Serienordner scannt richtig; der Inhaltswechsel
funktioniert.

**Berührte Dateien:** `server/internal/library/scanner.go` (`:169`, `:181`, `:301`, `:446`, `:511`,
`:606`, `:632`), `write.go` (`:52`, `:364`, Quelle in `titleItem`/`episodeItem`/`extraItem`),
`index.go` (`:86`, `:99`, Titelindex je Art), `pathscan.go` (`:277`), `roots.go` (`UpdateRoot`,
Löschweg ohne letzten Schritt), `metadata/match.go` (`:78`: liest `metadata_source`).

**DoD:** `go test ./internal/library/`: gemischte Bibliothek mit «Alone (2015)» als Film und als
Serie ergibt zwei Titel verschiedener Art; ein Serienordner verwirft Dateien ohne Folgenmarke, der
Filmordner derselben Bibliothek nicht; nach Inhaltswechsel `movie → series` gibt es keinen Titel mit
`deleted_at IS NULL` und null Dateien, der Download des gegangenen Films trägt einen Tombstone, der
Scan legt Folgen neu an; Quellenwechsel reiht `metadata.match` für genau die Titel der Wurzel ein und
löscht nichts. `slouchcat scanbench` (`cmd/slouchcat/scanbench.go:73` bis `:92`, folgt dem neuen
Körper) auf dem Platzhalterbaum ohne Regression gegen `docs/perf/M3.md`.

### Schritt 4 — Podcast-Ordner
`feat(library,metadata): scan podcast folders and adopt their feeds`

**Ziel:** `/media/nas/Audiobookshelf/Podcast` wird eine Podcast-Bibliothek mit Dateien **und** Feeds.

**Berührte Dateien:** `server/internal/library/podcast_folder.go` (neu: Ordner, `metadata.json`,
Cover, Episoden), `podcast_folder_test.go`, `audiobook.go` (`absMeta` um Podcast-Felder),
`metadata/podcast.go` (`:214` `Subscribe` aus Datei, Zusammenführen bei `:472` ff., `RefreshLibrary`
`:327`), `queries/system/podcasts.sql`, `queries/scoped/podcasts.sql` (Datei vor Enclosure),
`internal/playback` (Episode mit Datei spielt als Datei), `internal/offline` (Download aus Datei),
`admin/maintenance.go` (Gruppe «Datei und Feed»), `probe.go` (keine Keyframe-Sonde für Audio),
Testdaten: ein Mini-Podcast-Ordner mit `metadata.json`, zwei MP3 und dem Mock-Feed der CI, der eine
der zwei Episoden kennt.

**DoD:** `go test ./internal/library/ ./internal/metadata/ ./internal/playback/ ./internal/offline/`:
Ordner → Podcast mit `feed_url`; Feed-Abruf hängt die bekannte Episode an die Datei statt sie zu
doppeln; die unbekannte Feed-Episode entsteht als Enclosure-Episode; die Datei-Episode spielt
`direct` aus der Datei und lädt aus der Datei; eine `feedURL` auf `127.0.0.1` wird (ausserhalb des
E2E-Mock-Falls) abgelehnt und nicht gespeichert; `explicit: true` ergibt 18 mit Quelle `tags`; keine
`probe.keyframes`-Zeile für `podcast_episode` und `audiobook`. Kinderprofil-Vertragstest: eine
explizite Datei-Episode fehlt in `listPodcastEpisodes` unter 18.

### Schritt 5 — AniList
`feat(metadata): enrich anime titles from AniList after the TMDB match`

**Ziel:** Ein Titel aus einem Anime-Ordner trägt `anilist_id`, den japanischen Originaltitel und
Texte, wo TMDB leer war.

**Berührte Dateien:** `server/internal/metadata/anilist.go` (Client, Quota 60/min,
`Retry-After`), `anime_lists.go` (Zuordnungsliste, Cache, Streaming-Parser, 64 MB),
`anime_enrich.go` (Job `anime.enrich`), Tests mit `httptest`-Servern für beide Hosts,
`match.go` (Einreihen), `config.go` (`SLOUCHCAT_ANILIST`), Dashboard-Zähler, `cmd/apicheck`.

**DoD:** `go test ./internal/metadata/`: TVDB-Id → AniList-Id über die Liste (Staffel 1), TMDB-Id
eines Films ebenso; leere TMDB-Beschreibung wird gefüllt, vorhandene nicht angefasst; `isAdult` hebt
auf 18 und senkt nie; 429 mit `Retry-After` pausiert die Quote; Liste grösser als 64 MB wird
abgebrochen; kein Treffer schreibt `anilist_id = 0` und fragt beim nächsten Scan nicht erneut.
**Vorher** die zwei Prüfpunkte aus 4.4 (Rohadresse und Lizenz der Liste; TMDB-Ids bei Filmen) mit
Beleg in der Rückmeldung. Messung: 111 Anime-Titel vom NAS angereichert, Dauer und Anfragen je Host.

### Schritt 6 — Oberfläche
`feat(web): library families, content and metadata source per folder`

**Ziel:** Marco kann seine Tabelle in der Verwaltung anlegen, ohne eine Zeile Konfiguration.

**Berührte Dateien:** Abschnitt 7 vollständig: `LibrariesPage.tsx`, `LibraryFields.tsx`,
`LibraryCard.tsx`, `LibraryFoldersDialog.tsx` (+ Dialog «Ordner ändern»), `PodcastsPage.tsx`,
`AdminItemPage.tsx`, `SetupPage.tsx`, `queries.ts`, Locales, Storybook-Stories, Unit-Tests,
`web/e2e/admin-libraries.spec.ts`, `admin-library-folders.spec.ts`, `setup.spec.ts`,
`global-setup.ts`.

**DoD:** `task web:lint`, `task web:test`, `task web:storybook:test`, `task web:e2e` grün; der neue
Tastaturtest für «Ändern» nach Regel 8; `pnpm -C web test locales` grün; `grep -rn "'movies'\|
'series'" web/src --include=*.tsx` nennt nur noch Titelarten und Suchfilter.

### Schritt 7 — Abnahme, Messung, Doku
`docs(dev,perf): library folders with content and source, acceptance on CT 190`

**Ziel:** Der Stand ist belegt, gemessen und beschrieben.

**Berührte Dateien:** `docs/dev/CURRENT_STATUS.md` (neuer Abschnitt), `docs/perf/bibliotheken.md`
(Erst-Scan Hörbücher und Podcast-Ordner auf CT 190: Dauer, Jobs, NFS-Durchsatz, Speicher),
`docs/INSTALLATION.md:124` und `:65` bis `:73` (Assistent, Ordner), `deploy/README.md`,
`testdata/db/` (neuer eingefrorener Stand, 8.3), `docs/ideas/README.md` (Idee 01 Vermerk).

**DoD:** `task ci` grün. Abnahme durch Marco auf CT 190 nach 10.8: Zielstruktur angelegt, «Dokus»
zeigt Filme und Serien, Anime-Titel zeigen Originaltitel, Podcast-Ordner zeigt Episoden mit Datei und
neue aus dem Feed, ein Inhaltswechsel verlangt den getippten Pfad. Messwerte in `docs/perf/`.

### 10.8 Der Weg jedes Schritts: Zweig, Commit, Test, Push, Pull Request

Derselbe Weg wie in `06-benutzerverwaltung.md` 10.8, mit **zwei Umgebungen**:

| Umgebung | Wo | Was dort läuft | Wofür |
|---|---|---|---|
| **Entwicklung** | dieser Rechner (Linux Mint, Repo `/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat`) | `task dev`, `task ci`, das selbst gebaute Image `slouchcat:dev`, Marcos ständige Instanz auf Port 8096 | Code, Tests, erster Blick in der Oberfläche; **die echten NAS-Ordner liegen hier unter `/mnt/nerdnas/medien/…` lesbar** (Doku, KinderFilme, MangaSerien, Audiobookshelf), gut für Scannertests ohne CT |
| **Test** | Proxmox-CT 190, `192.168.1.43`, Port 8096 (`docs/dev/TESTSERVER.md`) | Stack aus `/opt/slouchcat/deploy`, 2 vCPU, 4 GB, NAS unter `/media/nas` | Marcos Abnahme an der echten Sammlung, am Handy und am Fernseher; der Messlauf aus Schritt 7 |

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
| **F-5** | vor Schritt 5 | **Welcher Anime-Anbieter?** Ordner tragen TVDB/TMDB-Ids; Staffeln und Folgen bleiben dort | AniList (ohne Schlüssel, GraphQL, Bilder, mehrsprachige Titel, Zuordnung über öffentliche Liste) | **AniList** | 2026-10-28 |
| **F-6** | vor Schritt 4 | **Podcast-Ordner und Feeds:** `metadata.json` trägt `feedURL` | Zusammen: Ordner scannen, Feed mitabonnieren; Abos ohne Ordner bleiben | **Ordner und Feeds zusammen** | 2026-10-28 |
| **F-7** | vor Schritt 0 | **Was ist der «Hörbuch Scraper»?** Es gibt keinen externen Hörbuch-Anbieter | Der heutige lokale Erkenner; Audnexus im Scraper-Konzept | **Der heutige lokale Erkenner** | 2026-10-28 |
| **F-8** | vor Schritt 6 | **Wortwahl** für die Wahl am Ordner | «Scraper» / «scraper» | **«Metadatenquelle» / «metadata source»** | 2026-10-28 |
| **F-9** | vor Schritt 5 | **Was darf AniList übersteuern?** Abschnitt 4.4 lässt AniList nur **füllen**, was TMDB leer lässt, plus Originaltitel und `anilist_id`. Für Anime wären die AniList-Beschreibungen oft die besseren, TMDB-Deutsch ist bei Anime häufig maschinell oder leer | **Füllen, nicht ersetzen** — bis Idee 01 die Anbieterreihenfolge je Ordner einstellbar macht. Dann wird «AniList vor TMDB» eine Einstellung statt einer Ausnahme im Code | | |
| **F-10** | vor Schritt 0 | **Reihenfolge:** erst die Benutzerverwaltung in `main`, dann dieser Zweig? Beide bauen Tabellen um (00022 `accounts`, 00023 `libraries`), beide heben den Vertrag brechend an | **Ja, Benutzerverwaltung zuerst.** Zwei parallele Tabellenumbauten auf derselben Datenbank von CT 190 sind ein Risiko ohne Nutzen; und die Nummern in diesem Papier (00023, 0015, 1.3.0) setzen es voraus | | |

---

## 12. Risiken

### 12.1 Der Titelindex einer gemischten Bibliothek

Der Scanner bindet Dateien an bestehende Titel über einen bibliotheksweiten Index
(`ListItemsByLibrary`, `index.go:112` ff.). Heute kann eine Bibliothek nur eine Titelart haben;
morgen stehen «Alone (2015)» als Film und als Doku-Serie **in derselben Bibliothek** (beide liegen
unter `/mnt/nerdnas/medien/Doku`). Der Index muss nach Art getrennt sein, sonst hängt der Scanner
Folgen an den Film. Schritt 3 trägt genau diesen Test. Das Zusammenführen in der Pflege
(`maintenance.go:429`) prüft die Art bereits.

### 12.2 Ordner- und Feed-Episoden finden nicht zusammen

Audiobookshelf benennt Dateien nach dem Episodentitel (`Godzilla.mp3`, `Besserer Sex - Wie wir
Bedürfnisse klar kommunizieren.mp3`), der Feed liefert GUIDs und Enclosure-Dateinamen, die davon
abweichen können. Die Regel aus 4.5 (Dateiname der Enclosure oder normalisierter Titel) wird bei
einem Teil der 26 000 Episoden danebenliegen; das Ergebnis sind Doppel, sichtbar in den
Pflegewerkzeugen, nicht verlorene Episoden. Besser wird das erst mit Audiobookshelfs
Episoden-Metadaten je Datei (`.abs`-Sidecars oder die ältere `metadata.json` mit Episodenliste) — zu
prüfen in Schritt 4 am echten Ordner, bevor die Regel festgezurrt wird.

### 12.3 Last durch die Audio-Ordner

26 031 Dateien im Podcast-Ordner und 62 Autorenordner mit M4B, alles über NFS mit 45 MB/s und
hoher Latenz (NAS-Stau seit dem 27. Oktober 2026, `probe.keyframes` liest jede Videodatei ganz). Ohne
die Ausnahme in 4.5 (keine Keyframe-Sonde für Audio, Format-Sonde ohne Paketlesen) würde dieser Ordner
die Warteschlange auf CT 190 für Wochen füllen. Der Erst-Scan ist darum ein Messlauf mit eigener
Datei in `docs/perf/`, und `SLOUCHCAT_WORKERS` bleibt der einzige Hebel, bis die Warteschlange einen
Pause-Schalter hat (nicht Teil dieses Papiers).

### 12.4 AniList und die Zuordnungsliste

Drei Unsicherheiten, in 4.4 benannt: das Rate-Limit (90 laut Dokumentation, zeitweise weniger),
die Rohadresse und Lizenz der Liste, die TMDB-Abdeckung bei Filmen. Dazu: die Liste verschmilzt über
`anidb_id`, nicht über TVDB, und eine TVDB-Serie mit fünf Staffeln hat fünf AniList-Einträge — die
Wahl «Staffel 1» ist für Titel und Originaltitel richtig und für die Beschreibung der fünften
Staffel falsch. Für dieses Konzept reicht das (Titelebene); Staffeltexte sind Idee 01.

### 12.5 Was dieses Papier nicht zählen konnte

Die Zahl der Stellen in `web/src`, die `'movies'` oder `'series'` als **Bibliotheksart** lesen, ist
klein (Abschnitt 7 nennt sie alle: `LibrariesPage.tsx:283`, `:289`, `LibraryCard.tsx:57`, `:138`,
`queries.ts:52`, `PodcastsPage.tsx:129`, `LibraryFields.tsx:10` bis `:17`, `:33`, `:83`,
`SetupPage.tsx:127`, `:277`), aber `'series'` ist auch eine **Titelart** und ein **Suchfilter**
(`searchParams.ts:7`); ein mechanisches Ersetzen wäre falsch. Je Stelle ist zu entscheiden, wie in
der Benutzerverwaltung bei `owner`.

### 12.6 Ein alter Vorbehalt bleibt

`SLOUCHCAT_BROWSE_ROOTS=/` auf CT 190 hebt E14 auf: ein Podcast-Ordner liesse sich dort auf
`/config` anlegen und der Scanner läse `slouchcat.db` als «ignorierte Datei». Das ist seit dem
25. Oktober 2026 bekannt und entschieden; dieses Konzept verschärft es nicht und entschärft es nicht.
