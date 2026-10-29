# Messwerte: Bibliotheken mit Inhaltsart und Metadatenquellen je Ordner (ADR 0015)

Konzept `docs/planning/07-bibliotheken.md`, Schritte 1 bis 6. Die Messungen zu Hörbuch- und
Podcast-Ordnern auf CT 190 und zum Anime-Lauf mit drei Anbietern folgen mit den Schritten 3, 4 und 6.

## 1. Scanner ohne Regression (Schritt 1, `slouchcat scanbench`)

Derselbe Platzhalterbaum wie in `docs/perf/M3.md` (60'800 leere Dateien, 2'500 Filme mit 5'169
Extras, 400 Serien mit 40'334 Folgen), erzeugt mit `testdata/generate-library-tree.py` unter
`/tmp`, 4 Worker, auf dem Entwicklungsrechner am 29. Oktober 2026. Die neue Fassung liest je
Wurzel und hält einen Titelindex je Titelart; zum Vergleich lief die Binärdatei von `main`
(`9923807`) **abwechselnd** auf demselben Baum, zweimal je Fassung:

```bash
server/bin/slouchcat scanbench --root /tmp/…/placeholder-tree --workers 4 --json
```

| Messung | `main` Lauf 1 | neu Lauf 1 | `main` Lauf 2 | neu Lauf 2 | M3 (19. Okt., Median) |
|---|---|---|---|---|---|
| Erst-Scan «Filme» (7'669 Dateien) | 7.9 s | 9.6 s | 9.3 s | 9.0 s | 5.8 s |
| Erst-Scan «Serien» (40'334 Dateien) | 40.7 s | 42.7 s | 41.7 s | 42.2 s | 26.9 s |
| Erst-Lauf bis die Queue leer ist (50'906 Jobs) | 64.1 s | 66.3 s | 65.1 s | 65.2 s | 47.0 s |
| Schreibtransaktionen | 713 | 713 | 713 | 713 | 713 |
| Längste Schreibtransaktion | 324 ms | 408 ms | 342 ms | 319 ms | 189 ms (< 2 s) |
| `GET /home` p95 während des Scans | 197 ms | 216 ms | 186 ms | 235 ms | 99.5 ms (< 300 ms) |
| Rescan ohne Änderung | 1.4 s, 0 Op. | 1.4 s, 0 | 1.6 s, 0 | 1.3 s, 0 | 1.9 s |
| Wiederaufnahme nach Abbruch bei 10 s | 31.1 s | 31.3 s | 31.8 s | 32.9 s | 16.3 s |
| Jobs mit `attempts > 1`, abgelaufene Leases, doppelte Schlüssel | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 / 0 / 0 | 0 |

**Lesart:** `main` und die neue Fassung liegen am selben Tag innerhalb der Streuung (Serien 40.7
bis 42.7 s, Erst-Lauf 64 bis 66 s, gleiche Transaktionszahl, alle Grenzwerte eingehalten). Der
Abstand zu den M3-Zahlen vom 19. Oktober betrifft beide Fassungen gleichermassen und kommt vom
Rechner an diesem Tag (laufende Instanz auf Port 8096, Docker, die e2e-Suite kurz davor), nicht
vom Scanner: die Titel-, Datei- und Transaktionszahlen sind identisch. Keine Regression durch
den Index je Titelart und das Lesen je Wurzel.

## 2. Auffüllen mit TMDB und AniList: 111 Anime-Titel vom NAS (Schritt 4)

Entwicklungsrechner, 29. Oktober 2026, 16:50 Uhr: ein eigener Server aus dem Stand von Schritt 4
(`serve --listen 127.0.0.1:8198 --log-level debug`, leerer Konfigurations- und Cache-Ordner, Worker
automatisch = 4) liest die echten Ordner vom NAS über NFS, `/mnt/nerdnas/medien/MangaFilme`
(**82 Filme**, Liste `[tmdb, anilist]`) und `/mnt/nerdnas/medien/MangaSerien` (**29 Serien**, Liste
`[tmdb, tvdb, anilist]`; auf diesem Rechner gibt es keinen TVDB-Schlüssel, TVDB wird darum als
Auffüller übersprungen und je Titel als Warnung gemeldet). Marcos TMDB-Schlüssel kam aus
`deploy/.env` in die Umgebung. Gezählt wurde aus dem Log: jede Anbieter-Anfrage steht dort als
`metadata request` mit `provider`, `host` und `path`.

| Messung | Wert |
|---|---|
| Erste Anbieter-Anfrage bis letzter `item filled` | **330.6 s** (16:50:32 bis 16:56:03) |
| davon: alle 111 Titel-Matches fertig | nach 33 s (16:51:05); die Fills warteten danach hinter 1 961 Folgen-Matches (Priorität 3) und den Bildern (Priorität 2) derselben Queue |
| Anfragen `api.themoviedb.org` | **2 477**: `/3/movie` 328 (82 Filme × 2 Sprachen × Match und Fill), `/3/tv` 2 088 (Serien, Staffeln, Folgen des Matches und 58 des Fills), `/3/find` 55, `/3/search` 6 |
| Anfragen `image.tmdb.org` (Bilder, nicht Teil des Auffüllens) | 281 in diesem Fenster |
| Anfragen `graphql.anilist.co` | **91** = 62 Filme + 29 Serien mit Eintrag in der Liste; Spitze **34 je Minute** (Limiter 1/s, Burst 5; AniList erlaubt 90), kein 429 |
| Anfragen `raw.githubusercontent.com` | 2 (die Liste; zwei Worker luden sie zugleich — danach auf einen Lauf je Woche begrenzt, `loadMu`) |
| Zuordnungsliste | 7 512 806 Bytes, ETag gespeichert; Karten: 1 324 Filme nach TMDB-Id, 1 563 nach IMDb-Id, 4 308 Serien nach TVDB-Id |
| AniList-Treffer | Filme **62 von 82** (20 mit `anilist_id = 0`), Serien **29 von 29** |
| Herkunftszeilen | `tmdb` 1 181, `union` 111 (Genres), `anilist` 1 |
| Freigaben | 72 Filme `tmdb:DE`, 1 `tmdb:GB`, 9 ohne; 27 Serien `tmdb:DE`, 2 `tmdb:US`; kein `isAdult` |
| Jobs am Ende des Fensters | `metadata.match` 1 961 done, `metadata.fill` 111 done, 0 failed |

**Lesart.** Das Auffüllen kostet je Titel genau einen Datensatz je Anbieter (TMDB zwei Sprachen,
AniList eine Anfrage) und einmal die Liste; die Dauer bestimmt nicht das Netz, sondern die Queue,
in der die Fills hinter den Folgen-Matches der 29 Serien stehen. **AniList gewinnt mit TMDB an
erster Stelle fast kein Feld:** TMDB hat für alle 111 Titel Titel, Beschreibung, Originaltitel und
Bilder, und AniList füllt nur, was davor leer blieb — hier eine englische Beschreibung eines Films;
die Genres kommen als Vereinigung (Herkunft `union`). Das ist die Regel «der erste gewinnt» (F-11).
Wer AniList-Texte sehen will, stellt AniList vor TMDB, was das Papier für Filme nicht erlaubt (F-5:
AniList ist nie Erkenner) — AniList bringt bei Filmen also `anilist_id`, Genres und die Freigabe
`isAdult`, bei Serien dasselbe. Der eigentliche Nutzen für Marcos Anime-Serien liegt bei
`[tvdb, tmdb, anilist]`: **1 783 von 2 948 Folgen blieben hier `unmatched`**, weil die Ordner nach
Sonarr (TVDB-Zählung) benannt sind und TMDB die Nummern anders vergibt; mit TVDB-Schlüssel und TVDB
zuerst erkennt TVDB sie (Schritt 1, `TestMatchSeriesTVDBFirst`). Das wird in der Abnahme auf CT 190
mit Marcos Schlüssel geprüft (Schritt 6).

**Erster Lauf, verworfen:** die Liste führt `themoviedb_id.movie` als **Liste** (`{"movie": [128]}`,
1 393 Einträge), nicht als Zahl; der erste Parser brach beim ersten solchen Eintrag ab, alle 82
Film-Fills endeten nach fünf Versuchen `failed`. Der Parser liest Zahl oder Liste und überspringt
Einträge anderer Form mit einer Warnung, statt die ganze Liste zu verwerfen.
