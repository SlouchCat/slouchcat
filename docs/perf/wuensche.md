# Messwerte: Konzept 25 «Wünsche über Seerr» (Schritte 2.11, 3.6 und 5.7)

Konzept `docs/planning/25-medienwuensche/`, Umsetzungsplan Schritte 2.11, 3.6 und 5.7; Zielwerte aus
`10-abnahmekriterien.md` Abschnitt 7 (AK-96 bis AK-101, AK-137, AK-189, AK-190, alle als **Vorschlag**) und
P-11 (Startseiten-API < 300 ms p95, `docs/perf/startseite.md`).

**Ergebnis:** Alle gemessenen Zielwerte halten, bis auf einen: `createWish` liegt bei Wünschen in schneller
Folge mit **594 ms p95** über den 500 ms von AK-98. Die Zeit geht nicht in Slouchcats Arbeit, sondern in das
eigene Ratenlimit des Seerr-Clients (5 Aufrufe je Sekunde, AK-103): Ist der Eimer leer, wartet jeder
Seerr-Aufruf 200 ms, und ein Wunsch braucht zwei bis drei davon. Mit 2 s Abstand zwischen zwei Wünschen
(eine Person am Bildschirm) sind es 43 ms p95. Dasselbe Limit macht eine kalte Entdecken-Seite **0,62 s**
langsam, auch mit vollem Eimer: Sie braucht sieben Seerr-Aufrufe, zwei mehr als der Eimer fasst. Der
Abgleich über drei IDs macht die Startseite nicht messbar langsamer (+0,3 ms p95 im ruhigen Lauf).

## Aufbau

Entwicklungsrechner (Linux Mint 22.3, i7-14700K, 28 Threads, NVMe, wie `docs/perf/startseite.md`),
18. November 2026, 01:26 bis 02:58 Uhr. Baustand `claude/k25-bau` auf `a8941cab` (gebaut mit `task build`
als `v1.40.1-118-g8cf2b22a`; der Commit dazwischen ändert nur den eingefrorenen Stand, keinen Servercode),
zum Vergleich `main` auf `5734a4e3` (Vertrag 1.40, Migrationen bis 00065, `go build` aus `git archive` mit
derselben Web-Hülle).

**Methode.** Eigener Server `serve --listen 127.0.0.1:8297 --dev` mit leerem Konfigurations- und
Cache-Ordner, Seerr, TMDB, Radarr und Sonarr ersetzt durch die e2e-Mocks `web/e2e/seerr-server.mjs`,
`tmdb-server.mjs` und `arr-server.mjs` auf den Ports von `start-server.mjs` (8286, 8285, 8284, 8283;
`SLOUCHCAT_TEST_TMDB_BASE_URL` mit `--dev`). Verbunden wie `web/e2e/wishes.ts` `openWishes`: der TMDB-Schlüssel
und der Seerr-Schlüssel der Mocks über die Verwaltung, Wünsche eingeschaltet, das Konto mit dem Seerr-Benutzer 2
verknüpft, Limit «unbegrenzt». Gemessen wird wie mit `docs/perf/measure-home.sh`: `curl` je Anfrage mit neuer
Verbindung vom selben Rechner (`time_total`), Sitzung mit gewähltem Hauptprofil (Stufe 18), p50, p95, max und
die Zahl der Antworten ausser 200/201. Jede Reihe lief dreimal; die Tabellen nennen den **mittleren Lauf**
(Median nach p95), die p95 der beiden anderen stehen in Klammern. Die Messskripte sind Wegwerfskripte und
liegen nicht im Repository.

**Was die Zahlen sind.** Die Mocks antworten in unter einer Millisekunde. Die Zahlen sind darum **Slouchcats
eigener Anteil**: Datenbank, Sichtbarkeit, Bildverarbeitung und die eigenen Ratenlimits der Clients (Seerr
5/s, TMDB 20/s). Im Betrieb kommen die Antwortzeiten von Seerr und TMDB dazu, je Aufruf, der nicht aus dem
Zwischenspeicher kommt; sie sind hier nicht gemessen und nicht geschätzt.

**Last.** Während der Reihen liefen auf dem Rechner Tests anderer Sitzungen: Lastmittel (1 min) **40 bis 165**
bei 28 Threads in den Läufen von 01:30 bis 02:01 Uhr. Die Läufe «5 s Abstand» (02:50 bis 02:56 Uhr) und der
zweite Vergleich der Startseite (02:57 Uhr) liefen auf ruhigem Rechner (Lastmittel unter 1). Unter Last
schwanken die warmen Werte zwischen den Läufen um den Faktor 2 bis 3 (etwa 3 statt 9 ms); die Aussagen unten
stützen sich nur auf Abstände, die grösser sind als diese Streuung.

## 1. Startseite vor und nach dem Abgleich über drei IDs (Schritt 3.6, AK-96, P-11)

Platzhalterbaum aus `testdata/generate-library-tree.py` (Standardgrössen wie `startseite.md`: 2'500 Filme,
400 Serien mit 2'167 Staffeln und 40'334 Folgen), Bibliotheken «Filme» und «Serien» mit dem Server von `main`
angelegt und gescannt, ohne TMDB-Schlüssel. Danach per SQL (Messaufbau): IDs an allen Titeln (Filme TMDB und
IMDb, Serien TMDB, TVDB und IMDb), `spotlight_entries` mit 100 Filmen und 100 Serien der Liste `trending`
(jeder zweite Eintrag trifft einen Bibliothekstitel über die TMDB-ID), `min_hits 0`, `item_count 20`. Die
Trakt-Abfrage zeigt auf einen geschlossenen Port und ihr Job ist abgeschaltet, damit der Server die Einträge
nicht leert. Dieser Stand wurde kopiert: eine Kopie läuft mit `main`, die andere mit dem Baustand (Migrationen
00066 bis 00081 beim Start). Für «drei IDs» bekamen die Einträge dieser Kopie danach ihre TVDB- und IMDb-IDs:
die ungeraden Plätze (ohne TMDB-Treffer) treffen über die IMDb-ID, jeder zehnte Serienplatz hat keine TMDB-ID
und trifft über die TVDB-ID. `GET /home` liefert in allen Fällen die Reihen `spotlight` (20 Titel) und `recent`;
die neue Reihe «Wünsche» bleibt ohne erfüllte Wünsche leer und erscheint nicht. n = 60 je Lauf.

| Stand | Last | p50 | p95 | max | nicht 200 | Grenze |
|---|---|---|---|---|---|---|
| `main` 5734a4e3 (nur TMDB) | 42 | 14.4 ms | **17.0 ms** (22.9, 16.2) | 17.7 ms | 0 | P-11 < 300 ms p95 |
| Baustand, Einträge nur mit TMDB-ID | 38 | 13.1 ms | **23.4 ms** (20.4, 29.3) | 29.6 ms | 0 | |
| Baustand, drei IDs | 37 | 14.2 ms | **20.5 ms** (16.9, 20.6) | 24.3 ms | 0 | AK-96: ≤ `main` + 10 ms |
| `main` 5734a4e3, ruhig | < 1 | 10.8 ms | **20.0 ms** (25.5, 15.2) | 23.9 ms | 0 | |
| Baustand, drei IDs, ruhig | < 1 | 13.6 ms | **20.3 ms** (19.5, 22.6) | 25.7 ms | 0 | AK-96: ≤ `main` + 10 ms |

**Lesart.** AK-96 hält: unter Last +3.5 ms p95, auf ruhigem Rechner +0.3 ms, beides innerhalb der Streuung der
drei Läufe. P-11 hält mit grossem Abstand. Die Abfrage `ListSpotlightRow` allein (Python, `mode=ro`, auf der
Kopie mit drei IDs, 200 Einträge, 200 Treffer vor dem Zuschnitt) braucht 0.8 ms; ihr Plan materialisiert
`hits` einmal und sucht die beste Stufe über einen automatischen Index (`BLOOM FILTER`,
`AUTOMATIC COVERING INDEX (media, position)`), also kein N+1 und kein quadratischer Vergleich; jede der drei
Stufen sucht über ihren Index (`items_tvdb`, `items_tmdb`, `items_imdb`).

## 2. Anfordern auf dem Server (Schritt 2.11)

Stand des Abschnitts 1 (Baustand, drei IDs), dazu 200 Wünsche des Hauptprofils per SQL (je rund 29 in den
Zuständen `queued`, `pending`, `processing`, `available`, `declined`, `failed`, `withdrawn`, mit Eintrag in
`external_titles` und deutschem Text, angelegt vor zwei Stunden, damit sie nicht in die 20 Wünsche je Stunde
zählen). Die neun Titel des TMDB-Mocks (Filme 9001 bis 9006, Serien 9100 bis 9102) dienen als Titel ausserhalb
der Bibliothek. «Kalt» heisst: die Zeile in `external_titles` vor jedem Aufruf gelöscht, der Server holt die
Details beim Mock.

| Aufruf | n | p50 | p95 | max | Fehler | Grenze |
|---|---|---|---|---|---|---|
| `getExternalTitle` kalt, direkt hintereinander | 9 | 14.1 ms | **180.3 ms** (176.0, 198.6) | 180.3 ms | 0 | – |
| `getExternalTitle` kalt, 5 s Abstand (ruhig) | 9 | 13.1 ms | **17.2 ms** (23.0, 15.8) | 17.2 ms | 0 | – |
| `getExternalTitle` warm (Film 9001 gespeichert) | 25 | 3.0 ms | **10.3 ms** (5.0, 10.8) | 11.3 ms | 0 | AK-101: ≤ 100 ms p95 |
| `createWish` direkt hintereinander (6 Filme, 3 Serien «alle Staffeln») | 9 | 393.8 ms | **594.2 ms** (402.8, 597.0) | 594.2 ms | 0 | AK-98: ≤ 500 ms p95, **verfehlt** |
| `createWish` mit 2 s Abstand | 9 | 4.9 ms | **43.3 ms** (19.6, 48.1) | 43.3 ms | 0 | AK-98: ≤ 500 ms p95 |
| `listWishes` (Seite mit 50 von 200) | 60 | 11.2 ms | **21.2 ms** (27.1, 20.7) | 25.5 ms | 0 | AK-100: ≤ 100 ms p95 (dort mit 500 Wünschen) |
| `listWishes?limit=100` | 60 | 18.4 ms | **34.1 ms** (51.1, 31.5) | 35.7 ms | 0 | |
| `getWishQuota` | 60 | 2.7 ms | **3.2 ms** (3.7, 3.1) | 5.0 ms | 0 | – |

**`createWish`.** Die Antwortzeit enthält den sofortigen Versuch bei Seerr (höchstens 3 s, Architektur 6.1),
nicht die Arbeit der Jobs danach (`wishes.tag` und die Nachläufe). Das Protokoll `seerr_log` zeigt je Wunsch
`POST /request` und `GET /user/{id}/quota`, bei einer Serie zuerst `GET /tv/{id}`; jeder dieser Aufrufe stand
mit **191 bis 199 ms** im Protokoll, obwohl der Mock sofort antwortet. Das ist die Wartezeit des Ratenlimits
im Seerr-Client (`server/internal/seerr`, 5 je Sekunde, Eimer 5): Nach den ersten Aufrufen der Reihe ist der
Eimer leer, und jeder weitere Aufruf wartet seinen Takt ab. Darum kostet ein Film rund 400 ms und eine Serie
rund 600 ms. Mit 2 s Abstand ist der Eimer wieder voll, und dieselben Wünsche brauchen 5 bis 48 ms. AK-98 ist
damit bei schneller Folge verfehlt; ob eine Person das erreicht (drei Wünsche in einer Sekunde, oder ein Wunsch,
während Entdecken-Seiten kalt laden), entscheidet die Abnahme. Im Betrieb kommen Seerrs eigene Antwortzeiten
dazu.

**`getExternalTitle` kalt.** In den Reihen ohne Pause hat jeder Lauf genau einen Ausreisser um 180 bis 200 ms
(der Rest 6 bis 24 ms); mit 5 s Abstand verschwindet er. Die Grösse passt zum Takt eines leeren Client-Eimers;
die Ursache ist nicht weiter untersucht. Ohne Wartezeit kostet das erste Holen und Speichern eines Titels rund
13 ms.

**`listWishes`.** 200 Wünsche kosten 21 ms p95 für die erste Seite. AK-100 nennt 500 Wünsche; gemessen sind
200, wie der Umsetzungsplan sie verlangt. Die Seite mit 100 Einträgen kostet rund das Doppelte der Seite mit
50 (Lesen und Prüfen je Eintrag), kein Zeichen eines N+1 über die Zahl aller Wünsche.

## 3. Entdecken kalt und warm (Schritt 5.7)

Derselbe Server. «Kalt» heisst: vor jedem Aufruf `wish_discover_pages` geleert und die Titel des Mocks aus
`external_titles` gelöscht; Seerr liefert die Listen, TMDB die Details (höchstens 20 je Seite, AK-190). «Warm»
heisst: Listen und Details gespeichert. Die Suche hält ihre Treffer zusätzlich 10 Minuten im Speicher; kalt
heisst dort ein neuer Suchtext je Aufruf. Die Mocks kennen wenige Titel: eine Suche trifft 1 bis 3 Titel, eine
Entdecken-Reihe hat höchstens 6. Die Reihen «direkt hintereinander» rufen ohne Pause, die Reihen «5 s Abstand»
liefen auf ruhigem Rechner mit vollem Eimer.

| Aufruf | n | p50 | p95 | max | Fehler | Grenze |
|---|---|---|---|---|---|---|
| `GET /wishes/discover` kalt, direkt hintereinander | 10 | 1552.7 ms | **1568.9 ms** (1564.4, 1578.0) | 1568.9 ms | 0 | AK-190: ≤ 5 s p95 |
| `GET /wishes/discover` kalt, 5 s Abstand | 5 | 618.9 ms | **623.2 ms** (817.9, 618.9) | 623.2 ms | 0 | AK-190: ≤ 5 s p95 |
| `GET /wishes/discover` warm | 40 | 22.5 ms | **25.5 ms** (7.7, 28.5) | 26.1 ms | 0 | AK-189: ≤ 100 ms p95 |
| `GET /wishes/titles/movie` kalt, direkt hintereinander | 10 | 741.0 ms | **748.0 ms** (750.0, 759.6) | 748.0 ms | 0 | – |
| `GET /wishes/titles/movie` kalt, 5 s Abstand | 5 | 18.8 ms | **20.9 ms** (22.1, 19.8) | 20.9 ms | 0 | – |
| `GET /wishes/titles/movie` warm | 40 | 2.9 ms | **8.5 ms** (9.0, 8.3) | 9.1 ms | 0 | – |
| `GET /wishes/search?q=…` kalt, direkt hintereinander | 6 | 565.8 ms | **573.4 ms** (566.2, 582.6) | 573.4 ms | 0 | AK-137: ≤ 3.5 s p95 |
| `GET /wishes/search?q=…` kalt, 5 s Abstand | 5 | 9.2 ms | **14.5 ms** (15.3, 14.4) | 14.5 ms | 0 | AK-137: ≤ 3.5 s p95 |
| `GET /wishes/search?q=glass` warm | 20 | 4.0 ms | **7.5 ms** (2.3, 7.6) | 7.5 ms | 0 | AK-137: zweite gleiche Suche ≤ 100 ms p95 |
| Bild ausserhalb der Bibliothek kalt (`/titles/{media}/{id}/images/poster/w400`, WebP) | 9 | 203.4 ms | **235.3 ms** (416.8, 203.1) | 235.3 ms | 0 | – |
| dasselbe Bild warm | 60 | 9.2 ms | **9.8 ms** (4.2, 9.9) | 13.1 ms | 0 | – |

**Entdecken kalt über einer Sekunde.** Direkt hintereinander braucht eine kalte Entdecken-Seite **1.55 s**,
die Seite «Filme» und die Suche 0.57 bis 0.75 s. Mit vollem Eimer sind es 0.62 s für Entdecken, aber nur 19 und
15 ms für «Filme» und die Suche: Die Zeit ist fast ganz Wartezeit der Client-Limits. Entdecken ruft sieben
Listen bei Seerr (`trending`, Filme, Serien, beide «Demnächst», beide Genre-Reihen); im Protokoll `seerr_log`
eines Aufrufs mit vollem Eimer gehen die ersten vier sofort, der sechste und siebte warten 190 und 390 ms auf
ihren Takt (Eimer 5, 5 je Sekunde). Eine kalte Entdecken-Seite kostet darum **rund 0.4 s Wartezeit im eigenen
Limit**, auch wenn Seerr sofort antwortet; in schneller Folge kommt die Wartezeit des TMDB-Limits (20 je
Sekunde) für die Details dazu. AK-190 (5 s) hält mit Abstand, die warmen Werte (AK-189, AK-137) ebenso.
Gemessen ist das Profil mit Stufe 18; AK-190 nennt ein Profil mit Stufe 12 (Vorfilter und Nachprüfung), das
hier nicht gemessen ist.

**Bild kalt.** Rund 200 ms für das erste Holen beim Mock und das Skalieren des 800 × 1200 grossen PNG in alle
Grössen und Formate der Zeile (JPEG und WebP); danach kommt das Bild in 10 ms aus dem Cache. Echte TMDB-Bilder
sind grösser und JPEG; ihre Zeit ist hier nicht gemessen.

## 4. Bewertung

| Zielwert | gemessen (p95, mittlerer Lauf) | erfüllt |
|---|---|---|
| P-11 `getHome` < 300 ms | 20.5 ms (Last), 20.3 ms (ruhig) | ja |
| AK-96 `getHome` drei IDs ≤ `main` + 10 ms | +3.5 ms (Last), +0.3 ms (ruhig) | ja |
| AK-98 `createWish` ≤ 500 ms | 594.2 ms direkt hintereinander, 43.3 ms mit 2 s Abstand | **nein** bei schneller Folge (Ratenlimit des Seerr-Clients), ja mit Abstand |
| AK-100 `listWishes` ≤ 100 ms | 21.2 ms (200 Wünsche; 500 nicht gemessen) | ja für 200 |
| AK-101 Vorschauseite warm ≤ 100 ms | 10.3 ms | ja |
| AK-137 Suche kalt ≤ 3.5 s, zweite ≤ 100 ms | 573.4 ms, 7.5 ms (1 bis 3 Treffer statt 20) | ja |
| AK-189 Entdecken warm ≤ 100 ms | 25.5 ms | ja |
| AK-190 Entdecken kalt ≤ 5 s | 1568.9 ms (Stufe 18 statt 12) | ja |

Für diese Messung wurde kein Code geändert. Für die Abnahme zu entscheiden: Das Ratenlimit des Seerr-Clients
bestimmt die kalten Zeiten von `createWish` und Entdecken, nicht die Arbeit des Servers.

Die Messung 6a.1 (Zeit vom Import bis «Verfügbar», AK-104 und AK-212) steht in Abschnitt 5.

## 5. Vom Import bis «Verfügbar» (Schritt 6a.1, 18. November 2026)

**Aufbau.** Test-Container CT 190 mit Release 1.41.1, Bibliotheken auf dem NAS (SMB, Änderungen erkennt die
Ordnerabfrage, Abstand mindestens 45 s). Echte Wünsche über Seerr, Download und Import durch Radarr und Sonarr.
Gelesen wurde nur, aus der Datenbank (`webhook_events`, `jobs`, `wishes`, `wish_events`), ohne Code-Änderung und
ohne eigene Instrumentierung. «Verfügbar» ist `wishes.available_at`.

**Gemessene Imports.** Drei Filme um 14:51 (Heartland, Verity, Allen), ein Film um 15:23 (Resident Evil: Welcome to
Raccoon City), alle vier vor dem Eintrag der Pfad-Umsetzung; eine Staffel mit vier Folgen um 15:56 (Resident Evil:
Infinite Darkness), nach dem Eintrag.

**Erster Befund, eine Einstellung:** Die Webhooks von Radarr und Sonarr hatten keine Pfad-Umsetzung. Radarr meldet
`/mnt/medien/Filme/…`, die Wurzel in Slouchcat heisst `/media/nas/Filme`; jede Meldung endete mit `outside_roots`,
und die Filme fand erst die Ordnerabfrage. Seit 15:31 steht bei beiden `/mnt/medien` → `/media/nas`.

| Abschnitt | Film Heartland (ohne Pfad-Umsetzung) | Staffel, 4 Folgen (mit Pfad-Umsetzung) |
|---|---|---|
| Webhook «Download» empfangen | 14:51:21.006, `outside_roots` | 15:56:00.354, `scan_queued` |
| Scan aus dem Webhook | keiner | 15:56:02.360 bis 02.624 auf dem Serienordner: findet nichts (liest keine Unterordner, die Folgen liegen in `Season 01/`) |
| Warten auf die Ordnerabfrage | 40.6 s | 40.6 s |
| Entprellung und Warteschlange | 2.1 s | 4.6 s |
| Scan (`scan.path`) | 0.34 s | 0.79 s |
| Zuordnung (`metadata.match`) | 0.33 s | 1.22 s |
| Abgleich der Wünsche (`wishes.reconcile`) | 0.24 s | 0.27 s |
| **Webhook bis «Verfügbar»** | **43.6 s** | **47.4 s** |

Die anderen Filme: Verity 39.2 s, Allen 4.3 s, Resident Evil 10.8 s; je nachdem, wann die Ordnerabfrage nach dem
Import lief. Ein Film mit Pfad-Umsetzung ist noch nicht gemessen: Der Webhook nennt bei Radarr den Ordner des Films,
in dem die Datei liegt, also läuft der Scan gleich nach der Entprellung, erwartet sind rund 3 bis 4 s (wie der
Anteil ab dem Scan-Auftrag oben). Die Messung 6a.4 holt das nach.

**Schwelle für einen Gewinn.** Ein Teil von Schritt 6a lohnt sich, wenn er bei einem üblichen Import mindestens
10 s oder mindestens ein Viertel der Zeit vom Webhook bis «Verfügbar» spart. Offen gesagt: Die Schwelle steht
erst nach dem ersten Blick auf die Filmwerte fest, nicht davor, wie es 6a.1 verlangt; an der Bewertung unten
ändert eine andere übliche Schwelle (5 s, 20 %) nichts.

**Bewertung je Teil von Schritt 6a:**

| Teil | spart | Gewinn |
|---|---|---|
| 6a.2 bei Sonarr: den Ordner von `episodeFile.path` scannen statt des Serienordners | rund 40 s von 47 s (das Warten auf die Ordnerabfrage) | **ja** |
| 6a.2 bei Radarr: `movieFile.path` lesen | nichts; der Ordner des Films ist schon der Ordner der Datei, die Pfad-Umsetzung genügt | nein |
| 6a.3: nur die eine Datei mit Vorrang einlesen und über die Kennung zuordnen | höchstens Scan und Zuordnung, 0.7 s (Film) bis 2.0 s (Staffel) | nein |

**Empfehlung der Redaktion war:** Schritt 6a schmal bauen: Bei einer Meldung von Sonarr mit `episodeFile.path`
scannt Slouchcat den Ordner dieser Datei; ohne Dateipfad bleibt es beim Serienordner. 6a.3 entfällt.

**Entscheid von Marco (18. November 2026): Es bleibt, wie es ist.** Schritt 6a wird nicht gebaut, auch nicht schmal.
Neue Folgen kommen weiter über die Ordnerabfrage an (rund 45 s nach dem Import), Filme mit Pfad-Umsetzung über den
Webhook.
