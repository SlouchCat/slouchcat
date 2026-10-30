# Messwerte: Startseite, Seitenleiste und Bibliotheksansichten (Konzept 08, ADR 0016)

Konzept `docs/planning/08-startseite.md`, Abschnitt 8; Grenzen aus Plan P-10 (LCP < 2.5 s) und P-11
(Startseiten-API < 300 ms p95, auch während eines Scans), dazu die Latte des Papiers für die
Bibliotheksseite (< 300 ms p95) und die Ausstiegsklausel für `itemCount` (20 ms p95).

Entwicklungsrechner (Linux Mint 22.3, i7-14700K, NVMe, wie `docs/perf/M3.md`), 30. Oktober 2026,
02:48 bis 02:51 Uhr, Stand `3e25aa4` (Schritt 4), `task build`. Eigener Server
(`serve --listen 127.0.0.1:8099`, leerer Konfigurations- und Cache-Ordner, ohne TMDB-Schlüssel,
4 Worker), Datenbank auf der lokalen Platte. Skripte: `docs/perf/measure-home.sh` (M11) und das
neue Schwesterskript `docs/perf/measure-library.sh` (dieselbe Methode: Aufwärmläufe, dann n
Anfragen vom selben Rechner, p50/p95/max, Zahl der Antworten ungleich 200; Sitzung mit gewähltem
Hauptprofil). Der Messlauf selbst liegt in den Sitzungsnotizen (`m3/run.sh`).

## 1. Startseite und Bibliotheksseite auf dem M3-Baum (P-11, Papier 8)

Platzhalterbaum aus `task testdata:tree` (60'800 leere Dateien: 2'500 Filme mit Extras, 400 Serien
mit 2'167 Staffeln und 40'334 Folgen), Bibliotheken «Filme» (`Movies/`, Inhalt `movie`) und
«Serien» (`TV/`, Inhalt `series`) über die echte API angelegt. Ohne TMDB-Schlüssel bleiben alle
Titel unbewertet; `sort_key` kommt aus dem Dateinamen über `items.sort_title`.

### 1a. Während des Erst-Scans der Serien (n = 30, «Filme» fertig, «Serien» `running`)

| Aufruf | p50 | p95 | max | nicht 200 | Grenze |
|---|---|---|---|---|---|
| `GET /home` (Hauptprofil, nur Reihe `recent`) | 26.4 ms | **34.9 ms** | 40.2 ms | 0 | < 300 ms p95 (P-11) |
| `GET /libraries/visible` | 18.2 ms | **27.5 ms** | 28.7 ms | 0 | 20 ms p95 (Ausstiegsklausel) |
| `GET /libraries/1/items` (Filme, Titel) | 18.0 ms | 34.3 ms | 38.5 ms | 0 | < 300 ms p95 |
| `… ?sort=added` | 17.4 ms | 21.4 ms | 22.3 ms | 0 | |
| `… ?sort=vote` | 16.4 ms | 34.5 ms | 35.4 ms | 0 | |
| `… ?sort=title&letter=S` | 19.1 ms | 26.6 ms | 26.8 ms | 0 | |
| `GET /libraries/2/items` (Serien, im Aufbau) | 26.7 ms | 34.7 ms | 35.1 ms | 0 | |
| `… ?sort=played` | 82.8 ms | 89.4 ms | 95.4 ms | 0 | |
| `… ?sort=title&letter=S&status=started` (schwerster Fall) | 103.1 ms | **111.6 ms** | 112.9 ms | 0 | < 300 ms p95 |
| `GET /picks` (leer) | 0.8 ms | 2.4 ms | 2.6 ms | 0 | |

### 1b. Ruhiger Server nach dem Scan (n = 60; 2'500 Filme, 400 Serien mit 40'334 Folgen)

| Aufruf | p50 | p95 | max | nicht 200 | Grenze |
|---|---|---|---|---|---|
| `GET /home` | 47.4 ms | **58.5 ms** | 60.7 ms | 0 | < 300 ms p95 (P-11); M3: 99.5 ms während des Scans |
| `GET /libraries/visible` | 40.8 ms | **50.0 ms** | 59.7 ms | 0 | 20 ms p95 (Ausstiegsklausel) |
| `GET /libraries/1/items` | 17.7 ms | 33.8 ms | 36.0 ms | 0 | < 300 ms p95 |
| `… ?sort=added` | 12.6 ms | 22.9 ms | 29.7 ms | 0 | |
| `… ?sort=vote` | 17.7 ms | 26.6 ms | 29.4 ms | 0 | |
| `… ?sort=title&letter=S` | 19.7 ms | 24.8 ms | 45.4 ms | 0 | |
| `GET /libraries/2/items` (Serien, Titel) | 55.8 ms | 65.9 ms | 83.9 ms | 0 | |
| `… ?sort=played` | 183.8 ms | **196.4 ms** | 201.0 ms | 0 | |
| `… ?sort=title&letter=S&status=started` (schwerster Fall) | 190.6 ms | **198.4 ms** | 209.6 ms | 0 | < 300 ms p95 |
| `GET /picks` | 2.4 ms | 2.9 ms | 3.0 ms | 0 | |

**Lesart.** P-11 hält: die Startseite bleibt mit 58.5 ms p95 weit unter 300 ms, auch während des
Scans (34.9 ms, dort mit kleinerer Bibliothek). Die Bibliotheksseite bleibt in jeder Sortierung
unter der Latte; die zwei Fälle mit der Folgen-Aggregation («Zuletzt gesehen», Status) kosten auf
der Serienbibliothek mit 40'334 Folgen rund 200 ms p95, also zwei Drittel der Latte — das ist der
`MATERIALIZE ep` aus dem `EXPLAIN QUERY PLAN` in Schritt 1 (alle sichtbaren Folgen der Bibliothek
einmal je Anfrage). Die Ausstiegsklausel «Hilfstabelle für den Serienstatus» greift nicht; wer sie
später braucht, misst zuerst auf CT 190 (2 vCPU: dort ist mit dem Faktor 2 bis 3 aus
`docs/perf/M3.md` 1c zu rechnen, also 400 bis 600 ms — **das wäre über der Latte**; eine Messung
auf CT 190 steht aus, Marco bringt den Stand dorthin).

**`itemCount` und die Ausstiegsklausel (O-1).** `GET /libraries/visible` liegt mit 50.0 ms p95 über
den 20 ms des Papiers (während des Scans 27.5 ms mit halb so vielen Titeln). Die Zeit geht in
`visible_items`: die Zählung läuft je Bibliothek über alle Titel der obersten Ebene (2'900) und
wertet die Sicht je Zeile aus (Freigabestufe, Bibliotheksrechte, Verfügbarkeit). Die Klausel
schlägt «eine kleine Zähltabelle je Bibliothek, die der Scanner pflegt» vor — **die kann es nicht
geben, ohne die Bedeutung von `itemCount` zu ändern**: die Zahl ist je Profil verschieden (ein
Kinderprofil mit Stufe 6 sieht in «Filme» 3 von 2'500 Titeln, siehe 2), der Scanner kennt kein
Profil. Eine Zähltabelle müsste je Profil und Stufe geführt werden und bei jeder Freigabe-,
Rechte- und Verfügbarkeitsänderung mitlaufen — ein zweiter Zustand neben `visible_items` (Regel 2).
Der Aufruf steht nicht im Weg der Startseite: die Hülle lädt ihn parallel zu `/home` und zur
Sitzung vor (`Link`-Kopf), die Seitenleiste wartet nicht auf ihn, und der Client hält ihn 60 s.
Darum bleibt es bei der Abfrage; der Befund steht als offener Punkt im Status (Entscheid für Marco:
so lassen, oder die Zahl aus der Seitenleiste nehmen).

## 2. K-3 am laufenden Server (Kinderprofil, `curl`)

Derselbe Server, nach dem Scan: sechs Filme per `POST /admin/items/rating` auf FSK 0 (drei) und
FSK 12 (drei) gesetzt, Profil «Kind» mit Stufe 6 angelegt und gewählt.

| Aufruf als «Kind» | Antwort |
|---|---|
| `GET /libraries/visible` | nur «Filme» mit `itemCount 3`; «Serien» (alle unbewertet) fehlt |
| `GET /libraries/1/items?limit=100` | `total 3`, alle drei `rating 0`, `facets.ratings [0]`, `facets.letters ["A"]` |
| `GET /libraries/1/items?limit=100&maxRating=18` | **dieselbe Antwort** (`total 3`, `ratings [0]`): die Grenze lässt sich nur senken |
| `GET /libraries/2/items` (unbewertete Serien) | **404** |
| `GET /libraries/99/items` (unbekannt) | **404** |
| `GET /libraries/1/items?sort=added&letter=S` (Hauptprofil) | 422 `validation_failed`, `errors[0] = {field: letter, code: letter_requires_title_sort}` |
| `GET /libraries/1/items?sort=author` (Video, Hauptprofil) | 422 `validation_failed`, `{field: sort, code: invalid_value}` |
| `GET /home?start=3` (ohne Spotlight-Liste) | `heroRow`, `heroStart`, `heroIntervalSeconds` fehlen; `rows: [recent]` |

## 3. LCP der Startseite (P-10): Lighthouse mobile und desktop

Methode wie `docs/perf/M11.md` Abschnitt 3: Lighthouse 12.8.2 (CLI, `CHROME_PATH` auf das
Playwright-Chromium 1248), nur die Kategorie Performance, angemeldet (`--extra-headers` mit
**beiden** Cookies `slouchcat_device` und `slouchcat_session`, M10-Nachtrag), eigener Server auf
`127.0.0.1:8098` aus `task build` (Stand `3e25aa4` plus die Vormal-Korrektur unten), Bibliothek
`/mnt/nerdnas/medien/Testfilme` (12 Titel, alle mit Poster und Backdrop nach `image.fetch`, Marcos
TMDB-Schlüssel), `spotlight_entries` wie in M11 3b mit den zwölf TMDB-Ids gesät (Messaufbau;
Marcos Trakt-Client-Id, damit der Server den Cache beim Start nicht leert), `min_hits 0`. Mobile =
Lighthouse-Voreinstellung (412 px, DPR 1.75, simuliert 1.6 Mbit/s, 150 ms RTT, 4× CPU), Desktop =
`--preset=desktop`. Median aus drei Läufen, Läufe als `stage-{mobile,desktop}-run{1,2,3}.json` und
`nostage2-mobile-run{1,2,3}.json` in den Sitzungsnotizen. 30. Oktober 2026, 02:57 bis 03:10 Uhr.

### 3a. Mit Bühne (Spotlight erste Reihe, `#hero-prepaint` mit `data-start`)

| | Mobile Lauf 1 / 2 / 3 | Median | Desktop Lauf 1 / 2 / 3 | Median | Grenze |
|---|---|---|---|---|---|
| LCP | 1.502 / 1.801 / 1.501 s | **1.502 s** | 0.442 / 0.441 / 0.401 s | **0.441 s** | < 2.5 s; M11: 1.652 s |
| FCP | 0.752 / 0.751 / 0.751 s | 0.751 s | 0.202 / 0.201 / 0.201 s | 0.202 s | |
| CLS / TBT | 0 / 6 ms, 0 / 9 ms, 0 / 2 ms | 0 / 6 ms | 0.0005 / 0 ms (3×) | 0.0005 / 0 ms | |
| Lighthouse-Note | 100 / 99 / 100 | 100 | 100 / 100 / 100 | 100 | |
| LCP-Phasen (TTFB / Ladeverzug / Ladezeit / Renderverzug) | 0.45 / 0.26 / 0.29 / 0.50 s; 0.45 / 0.66 / 0.23 / 0.46; 0.45 / 0.53 / 0.20 / 0.32 | | 0.12 / 0.03 / 0.08 / 0.21 s; 0.12 / 0.06 / 0.07 / 0.19; 0.12 / 0.00 / 0.07 / 0.21 | | |

LCP-Element in allen sechs Läufen: `div#hero-prepaint > img` (das Backdrop des **gewürfelten**
Titels; die Hülle trug `data-start="7"`, `GET /home?start=7` dieselbe Zahl). Die Seitenleiste
(Desktop) kostet nichts Messbares: die Bühne beginnt rechts von ihr, `sizes` zieht die 15 rem ab.
Der CLS von 0.0005 auf dem Desktop ist die Spalte, die mit dem Start-JS ihre Einträge erhält.

### 3b. Ohne Bühne (Spotlight ausgeschaltet, `#poster-prepaint`)

| | Mobile Lauf 1 / 2 / 3 | Median | Desktop Lauf 1 / 2 / 3 | Median | Grenze |
|---|---|---|---|---|---|
| LCP | 1.578 / 1.577 / 1.577 s | **1.577 s** | 0.362 / 0.362 / 0.563 s | **0.362 s** | < 2.5 s; M11: 2.328 s |
| FCP | 0.752 / 0.751 / 0.751 s | 0.751 s | 0.222 / 0.201 / 0.306 s | 0.222 s | |
| CLS / TBT | 0 / 3 ms, 0 / 2 ms, 0 / 0 ms | 0 / 2 ms | 0.0005 / 0 ms (3×) | 0.0005 / 0 ms | |
| Lighthouse-Note | 100 / 100 / 100 | 100 | 100 / 100 / 100 | 100 | |
| LCP-Phasen | 0.45 / 0.69 / 0.13 / 0.31 s; 0.45 / 0.66 / 0.10 / 0.36; 0.45 / 0.54 / 0.06 / 0.52 | | 0.12 / 0.01 / 0.04 / 0.20 s; 0.12 / 0.07 / 0.06 / 0.12; 0.12 / 0.12 / 0.06 / 0.26 | | |

LCP-Element: `div#poster-prepaint > img` (das erste Poster von «Neu hinzugefügt»).

**Befund und Korrektur.** Der erste Mobile-Lauf ohne Bühne lag bei **3.35 / 3.50 / 3.58 s**, das
LCP-Element war die **Karte** (`li > a > span > img`, Renderverzug 2.7 bis 2.9 s), nicht das
Vormalen. Die Geometrie stimmte (Playwright bei 412 px: Vormal-Poster und Kartenrahmen beide
`16, 146, 116 × 174`), aber die Trace zeigte zwei LCP-Kandidaten mit 20'177 px² (Vormalen) und
20'184 px² (Karte): Subpixel-Rundung macht das Kartenbild 7 px² grösser, und Chrome nimmt den
grösseren Kandidaten — damit sprang der LCP auf das Malen der Karte durch React. Korrektur: das
Vormal-Poster ist **0.5 px breiter** als die Karte (`global.css`, alle drei Breiten), der
Geometrie-Test in `sidenav.spec.ts` erlaubt unter 1 px Abweichung, sichtbar ist nichts. Die
Desktop-Läufe hatten den Fall nie (dort gewann das Vormalen auch vor der Korrektur).

**Lesart P-10.** Beide Zustände liegen mobil deutlich unter 2.5 s und unter den M11-Werten
(1.502 gegen 1.652 s mit Bühne; 1.577 gegen 2.328 s ohne). Die drei Cover je Reihe auf dem Handy
(N-3) machen das erste Poster grösser als früher, der Preload und das Vormalen tragen es.
