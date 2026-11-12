# Messwerte: Konzept 20 «Bedienung und Gesehen»

Gemessen am 12. November 2026 lokal (28 Kerne, Go 1.27.2, SQLite über `modernc.org/sqlite`, Datenbank in einem
temporären Ordner) auf dem Zweig `claude/k20-s2srv` (Vertrag 1.31.0, Migration 00053). Während der Messung liefen
auf derselben Maschine andere Agenten mit Tests; die Werte streuen deshalb nach oben (siehe 2.2).

**Ergebnis M-2:** `listWatched` mit **5 000 gesehenen Folgen** (50 Serien × 100 Folgen) braucht im Leser
(Abfrage, Folgenzahlen und Karten mit `watchedCount`) **33 ms im Median und 51 ms im 95. Perzentil**, die
Abfrage allein 16 / 30 ms. Das Ziel von 150 ms ist eingehalten. Auf CT 190 ist noch nicht gemessen; der Wert
dort folgt bei der Abnahme der Stufe.

## 1. M-2: `listWatched` mit 5 000 gesehenen Folgen

`BenchmarkWatchedList` in `server/internal/visibility/watched_bench_test.go`:

```
go test -run '^$' -bench WatchedList -benchtime 200x -v ./internal/visibility/
```

Aufbau: die Testbibliothek von `vistest` mit ihren Filmen, dazu 50 Serien mit je einer Staffel zu 100 Folgen und
einer abspielbaren Datei je Folge. Das Profil P16 hat jede Folge und jeden Film fertig gesehen (`completed = 1`,
`watched_at` eine Minute auseinander), also 5 000 gesehene Folgen. Jede Serie ist damit ein Eintrag der Liste,
jede Seite zählt die sichtbaren und die gesehenen Folgen ihrer Serien.

| Lauf (200 Aufrufe) | p50 | p95 |
|---|---:|---:|
| `ListWatched` allein, neueste zuerst, 50 je Seite | 15,7 ms | 30,4 ms |
| `Reader.Watched`, neueste zuerst, 50 je Seite | **33,0 ms** | **51,1 ms** |
| `Reader.Watched`, nach Titel, 50 je Seite | 31,6 ms | 41,9 ms |
| `Reader.Watched`, nur Serien, 100 je Seite | 33,3 ms | 45,0 ms |

Ein zweiter Lauf mit 50 Aufrufen kurz danach: Leser 39,2 / 60,0 ms, Abfrage 17,3 / 24,2 ms.

Die Abfrage geht von den gesehenen Zeilen des Profils aus (Teilindex `progress_watched` aus Migration 00053) und
prüft die Sichtbarkeit erst danach, je gesehene Zeile einmal. Ihre Kosten folgen also dem, was das Profil gesehen
hat, nicht der Grösse der Bibliothek. Die Gesamtzahl rechnet dieselbe Abfrage mit (`count(*) OVER ()`); nur eine
Seite hinter dem Ende fragt sie mit `CountWatched` eigens nach.

## 2. Nebenwirkung: `watchedCount` an Serien und Podcasts

Damit jede Karte den halben Haken zeigen kann (F-12), trägt `ItemRef.watchedCount` seit 1.31.0 auch an Serien und
Podcasts die Zahl der gesehenen sichtbaren Folgen. Jede Liste mit Serien oder Podcasts rechnet sie mit.

### 2.1 Verworfene erste Fassung

Die erste Fassung zählte in `ListVisibleChildCounts` je Titel über alle seine Folgen. Gemessen mit
`BenchmarkPodcastLibrary` (100 Podcasts × 1 000 Folgen, Abfrage der Kinderzahlen für 100 Podcasts):

| Fall | in `docs/dev/CURRENT_STATUS.md` (1.22.0) | erste Fassung |
|---|---:|---:|
| P18, nichts gehört | 0,9 ms | **103,4 ms** (p95 147 ms) |
| P16, alle 100 000 Folgen gehört | 72,8 ms | 255,3 ms |

Das Zählen liest jede Folge jedes Podcasts, auch wenn das Profil nichts gehört hat; das ist derselbe Fehler, der
schon beim Bau von `ItemRef.watched` verworfen wurde (212 ms für 100 Serien).

### 2.2 Gebaute Fassung

`ListVisibleWatchedCounts` (`server/queries/scoped/items.sql`) geht wie `listWatched` von den gesehenen Zeilen
des Profils aus, ordnet jede ihrer Serie oder ihrem Podcast zu und prüft die Sichtbarkeit nur für Zeilen der
angefragten Titel. Wer nichts gesehen hat, zahlt nichts.

| Lauf (`BenchmarkPodcastLibrary`, 50 Aufrufe) | p50 | p95 |
|---|---:|---:|
| Kinderzahlen 100 Podcasts, P18 nichts gehört | **0,9 ms** | 2,2 ms |
| Kinderzahlen 100 Podcasts, P16 alles gehört | 140,4 ms | 191,0 ms |
| Bibliotheksseite nach Titel, P18 | 365,8 ms | 562,7 ms |
| Bibliotheksseite nach Titel, P16 alles gehört | 809,6 ms | 1 636 ms |

Diese Läufe liefen unter der Last der anderen Agenten: Die Bibliotheksseite für P18 berührt die neue Abfrage gar
nicht (keine gesehene Zeile) und brauchte trotzdem 366 statt der 189 ms aus `CURRENT_STATUS.md`. Die Maschine war
also etwa doppelt so langsam. Bereinigt darum kostet die neue Zahl im ungünstigsten Fall (100 000 gehörte Folgen,
alle in den angefragten Titeln) grob 250 ms mehr, das sind etwa 2,5 µs je gesehene Folge. Ein Profil mit 5 000
gesehenen Folgen zahlt je Liste mit Serien grob 12 ms. Eine Messung ohne fremde Last steht noch aus, ebenso eine
auf CT 190.

## 3. Was nicht gemessen ist

- `setItemWatched` für eine ganze Serie: schreibt je sichtbare Folge eine Zeile in einer Transaktion; bei
  Serien mit einigen hundert Folgen unkritisch, nicht eigens gemessen.
- CT 190: beide Messungen folgen bei der Abnahme der Stufe auf dem Server mit seiner echten Datenbank.
