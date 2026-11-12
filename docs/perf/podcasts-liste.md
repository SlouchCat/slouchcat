# Messwerte: Verwaltungsseite «Podcasts», Feedliste

Gemessen am 12. November 2026 auf CT 190 (Release 1.30.0) und auf einer lokalen Kopie seiner Datenbank. Auf
CT 190 wurde nichts geschrieben: Die Datenbank ist dort nur lesend geöffnet (`?mode=ro`, Python, weil `sqlite3`
im CT fehlt), die Abfragen kommen über die Standardeingabe. Die Zeiten des Endpunkts stammen aus dem Log des
Servers (`docker logs slouchcat`).

**Ergebnis:** Die Feedliste der Seite «Podcasts» wartete auf `GET /api/v1/admin/podcasts`, und der brauchte auf
CT 190 **201 bis 223 Sekunden**. Fast die ganze Zeit ging in drei Abfragen, die über den Index
`media_files_missing` jede vorhandene Datei des Servers einmal je Podcast lasen. Migration 00052 macht den Index
partiell; dieselben Abfragen brauchen danach **0,4 statt 47 bis 93 Sekunden**, alle Abfragen der Liste zusammen
**2,6 statt 188 Sekunden** auf CT 190 und **0,47 statt 22 Sekunden** auf der lokalen Kopie.

## 1. Befund

| Was | Wert |
|---|---|
| Bild auf der Seite | Der Bereich «Downloads» steht sofort, die Feedliste bleibt ein leerer Platzhalter (`Skeleton`, solange `podcastsQuery` lädt) |
| `GET /api/v1/admin/podcasts/downloads` | 11 ms |
| `GET /api/v1/admin/libraries` | 454 ms |
| `GET /api/v1/admin/podcasts` | **201 s, 218 s, 223 s** (drei Aufrufe, Log von CT 190, 07:49 bis 08:01 Uhr) |
| Datenbestand | 61'749 `media_files`, davon 713 fehlend; 106 Podcasts, 65'133 Podcastfolgen; 327'257 Jobs |
| Statistiken des Planers | keine (`sqlite_stat1` fehlt; der Server führt kein `ANALYZE` aus) |

`GET /admin/podcasts` liest acht Abfragen (`metadata.ListPodcasts`, `withDownloads`, `library.PodcastStates`) und
prüft je Podcast-Wurzel einmal den Ordner, je Podcast ein `Lstat`. Drei Abfragen tragen die ganze Zeit:

| Abfrage (CT 190, `mode=ro`) | Zeilen | vorher | Plan vorher |
|---|---:|---:|---|
| `PodcastList` | 100 | **46,8 s** | `SEARCH m USING INDEX media_files_missing (missing_since=?)` |
| `PodcastDownloadStats` | 100 | **46,9 s** | dito |
| `PodcastNewestRoots` | 94 | **93,4 s** | dito, zweimal je Podcast |
| `PodcastEffectiveSettings` | 94 | 0,000 s | |
| `PodcastAllEpisodesWithoutFile` | 6'458 | 0,197 s | |
| `PodcastAllFolders` | 101 | 0,374 s | |
| `PodcastAllFolderFiles` | 26'405 | 0,554 s | |
| `PodcastAllFileEpisodesWithoutGUID` | 9'229 | 0,181 s | |
| **Summe** | | **188,4 s** | |

## 2. Ursache

Die drei Abfragen zählen oder suchen je Podcast in einer korrelierten Unterabfrage die Dateien seiner Folgen, mit
`m.missing_since IS NULL`. Der Index `media_files_missing (missing_since)` aus Migration 00003 enthielt jede
Datei. Ohne Statistiken hält SQLite einen Gleichheitsvergleich auf einem Index für selektiv und nimmt ihn: Er
liefert aber 61'036 vorhandene Dateien, und für jede liest SQLite die Tabellenzeile und die Folge, einmal je
Podcast (106 × 61'036). Der gute Weg geht von den Folgen des Podcasts (`items_published`) über
`media_files_item` zu ihren Dateien.

Dieselbe Falle war schon zweimal umgangen worden, je mit `INDEXED BY`: `CountLibraryMediaFiles` (1,35 s von
`GET /admin/libraries`, 11. November 2026) und `ListKeyframesBackfillFiles`. Die Abfragen selbst sind älter
(`local_episode_count` seit Vertrag 1.3.0, `PodcastNewestRoots` seit 1.28.0). Ihre Zeit wächst mit Podcasts mal
vorhandenen Dateien des ganzen Servers; mit dem Wiedereintragen des Podcast-Ordners am 11. November 2026 stieg
die Zahl der Dateien von rund 35'000 auf 61'749.

Der Server nutzt dieselbe Planung: Mit dem Treiber des Servers (`modernc.org/sqlite` v1.60.1, SQLite 3.53.4)
wählt SQLite auf der Kopie denselben Plan über `media_files_missing` (Abschnitt 4).

Ein einzelner Aufruf um 07:35 Uhr, sieben Minuten nach dem Neustart des Containers, brauchte nur 8 s. Der
Datenbestand hat sich seither nicht verändert (eine neue Datei, keine neu fehlende). Der Grund für diesen
Ausreisser ist nicht geklärt; alle späteren Aufrufe und alle Messungen der Abfragen liegen beim langen Wert.

## 3. Behebung

Migration `00052_media_files_missing_partial.sql`:

```sql
DROP INDEX media_files_missing;
CREATE INDEX media_files_missing ON media_files (missing_since) WHERE missing_since IS NOT NULL;
```

Für `IS NULL` kann SQLite den Index nicht mehr nehmen, für fehlende Dateien (`IS NOT NULL`, `missing_since < ?`,
`INDEXED BY` in `CountLibraryMediaFiles`) weiterhin; der Plan von `CountLibraryMediaFiles` bleibt
`SEARCH m USING INDEX media_files_missing (missing_since>?)`. Der Index wird kleiner (713 statt 61'749 Einträge).
Die Abfragen selbst bleiben unverändert. Der Test `TestPodcastListQueriesReachFilesThroughEpisodes` (Paket
`library`) prüft die Pläne der drei Abfragen vor und nach `ANALYZE`; ohne die Migration schlägt er fehl.
`TestMigration00052MediaFilesMissingPartial` (Paket `store`) prüft den Index nach der Migration vom
eingefrorenen Stand `radio-wikidata.sql`.

## 4. Nachher

Auf CT 190 wurde nichts geändert. Gemessen ist zweierlei:

- **CT 190, gleicher Plan:** die drei Abfragen mit `+m.missing_since IS NULL` (der unäre Operator sperrt den
  Index für diesen Ausdruck, wie es die partielle Fassung tut). Die Pläne sind dieselben wie auf der Kopie mit
  Migration: `items_published (parent_id=?)`, dann `media_files_item (item_id=?)`.
- **Lokale Kopie:** Datenbank und WAL von CT 190 per `scp` kopiert (`PRAGMA quick_check` = ok), dann die
  Migration angewendet. Gemessen mit Python (SQLite 3.45.1) und, für die drei teuren Abfragen, mit dem Treiber
  des Servers (Go, `modernc.org/sqlite` v1.60.1, SQLite 3.53.4, je ein Lauf mit kaltem Prozess). Der lokale
  Rechner ist rund achtmal schneller als CT 190.

| Abfrage | CT 190 vorher | CT 190 nachher | Kopie vorher | Kopie nachher | Kopie Go vorher | Kopie Go nachher |
|---|---:|---:|---:|---:|---:|---:|
| `PodcastList` | 46,8 s | **0,450 s** | 5,524 s | 0,092 s | 8,167 s | 0,179 s |
| `PodcastDownloadStats` | 46,9 s | **0,379 s** | 5,444 s | 0,052 s | 7,438 s | 0,076 s |
| `PodcastNewestRoots` | 93,4 s | **0,435 s** | 10,803 s | 0,103 s | 14,777 s | 0,131 s |
| übrige fünf | 1,306 s | 1,306 s | 0,220 s | 0,223 s | | |
| **Summe** | **188,4 s** | **2,57 s** | **21,99 s** | **0,47 s** | **30,38 s** | **0,39 s** |

Für den Endpunkt auf CT 190 heisst das rund 2,6 Sekunden statt 200 bis 223 Sekunden. Die Ordnerprüfung (je Wurzel
einmal, je Podcast ein `Lstat` über NFS) und die Antwort (100 Podcasts) kommen dazu; den Endpunkt selbst misst
Marco nach dem Update im Browser.

## 5. Wiederholen

```bash
# CT 190, nur lesend
python3 docs/perf/podcasts-liste/measure.py emit | ssh root@192.168.1.43 python3 - /opt/slouchcat/deploy/config/slouchcat.db
# lokale Kopie, vor und nach der Migration
python3 docs/perf/podcasts-liste/measure.py local /pfad/zur/kopie/slouchcat.db
python3 docs/perf/podcasts-liste/measure.py local /pfad/zur/kopie/slouchcat.db --apply-00052
```

## 6. Nicht gemessen

- Der Endpunkt mit dem neuen Image auf CT 190 (dort schreibt Claude nichts). Nach dem Update gibt das Log von
  `GET /api/v1/admin/podcasts` die Zeit (`duration_ms`).
- Die übrigen Abfragen mit `missing_since IS NULL` ausserhalb der Podcast-Liste einzeln. Sie können nur
  schneller werden: Der Index lieferte für `IS NULL` immer fast alle Dateien.
